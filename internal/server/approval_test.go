package server

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/mison/antigravity-go/internal/pkg/i18n"
)

func TestBuildApprovalRequestPayloadForWriteFileIncludesDiff(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "notes.txt")
	if err := os.WriteFile(target, []byte("before\n"), 0644); err != nil {
		t.Fatalf("failed to seed file: %v", err)
	}

	rawArgs, err := json.Marshal(writeFileApprovalArgs{
		Path:    target,
		Content: "after\n",
	})
	if err != nil {
		t.Fatalf("failed to marshal args: %v", err)
	}

	payload, plan := buildApprovalRequestPayload("zh-CN", "write_file", string(rawArgs), root)

	if payload.Tool != "write_file" {
		t.Fatalf("unexpected tool: %q", payload.Tool)
	}
	if plan == nil {
		t.Fatal("expected execution plan")
	}
	if payload.Metadata["path"] != "notes.txt" {
		t.Fatalf("unexpected path metadata: %#v", payload.Metadata["path"])
	}
	if !strings.Contains(payload.Preview, "-before") {
		t.Fatalf("expected removed line in preview, got %q", payload.Preview)
	}
	if !strings.Contains(payload.Preview, "+after") {
		t.Fatalf("expected added line in preview, got %q", payload.Preview)
	}
	if len(payload.Chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(payload.Chunks))
	}
}

func TestBuildApprovalRequestPayloadForApplyCoreEditIncludesDiff(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "main.go")
	if err := os.WriteFile(target, []byte("package main\n"), 0644); err != nil {
		t.Fatalf("failed to seed file: %v", err)
	}

	rawArgs, err := json.Marshal(coreEditApprovalArgs{
		FilePath: target,
		Edits: []coreEditTextEdit{
			{
				Range: coreEditRange{
					Start: coreEditPosition{Line: 0, Character: 8},
					End:   coreEditPosition{Line: 0, Character: 12},
				},
				NewText: "demo",
			},
		},
	})
	if err != nil {
		t.Fatalf("failed to marshal args: %v", err)
	}

	payload, plan := buildApprovalRequestPayload("zh-CN", "apply_core_edit", string(rawArgs), root)

	if plan == nil {
		t.Fatal("expected execution plan")
	}
	if payload.Metadata["file_path"] != "main.go" {
		t.Fatalf("unexpected file_path metadata: %#v", payload.Metadata["file_path"])
	}
	if !strings.Contains(payload.Preview, "-package main") {
		t.Fatalf("expected removed line in preview, got %q", payload.Preview)
	}
	if !strings.Contains(payload.Preview, "+package demo") {
		t.Fatalf("expected added line in preview, got %q", payload.Preview)
	}
}

func TestCancelPendingApprovalsUnblocksWaiters(t *testing.T) {
	ch := make(chan approvalDecision, 1)
	sess := &webSession{
		pending: map[string]*pendingApproval{
			"req-1": {
				ch: ch,
			},
		},
	}

	done := make(chan approvalDecision, 1)
	go func() {
		decision, ok := <-ch
		if !ok {
			done <- approvalDecision{Allow: false, Reason: "closed"}
			return
		}
		done <- decision
	}()

	sess.cancelPendingApprovals("connection_closed")

	select {
	case decision := <-done:
		if decision.Allow {
			t.Fatal("expected pending approval to be rejected")
		}
		if decision.Reason != "connection_closed" && decision.Reason != "closed" {
			t.Fatalf("unexpected decision reason: %q", decision.Reason)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("pending approval was not released")
	}
}

func TestApprovalWithMultibyteChars(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "greet.txt")
	// "你好" is 6 bytes in UTF-8, 2 runes.
	if err := os.WriteFile(target, []byte("你好 World\n"), 0644); err != nil {
		t.Fatalf("failed to seed file: %v", err)
	}

	// Change "World" to "世界"
	rawArgs, _ := json.Marshal(coreEditApprovalArgs{
		FilePath: target,
		Edits: []coreEditTextEdit{
			{
				Range: coreEditRange{
					Start: coreEditPosition{Line: 0, Character: 3}, // After "你好 "
					End:   coreEditPosition{Line: 0, Character: 8}, // After "World"
				},
				NewText: "世界",
			},
		},
	})

	payload, _ := buildApprovalRequestPayload("zh-CN", "apply_core_edit", string(rawArgs), root)
	if !strings.Contains(payload.Preview, "+你好 世界") {
		t.Fatalf("expected correct rune-based diff, got %q", payload.Preview)
	}
}

func TestConcurrentApprovals(t *testing.T) {
	sess := &webSession{
		pending: make(map[string]*pendingApproval),
	}

	var wg sync.WaitGroup
	count := 10
	results := make([]bool, count)

	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			id := fmt.Sprintf("req-%d", idx)
			ch := make(chan approvalDecision, 1)

			sess.pendingMu.Lock()
			sess.pending[id] = &pendingApproval{ch: ch}
			sess.pendingMu.Unlock()

			decision := <-ch
			results[idx] = decision.Allow
		}(i)
	}

	// Give them a moment to start
	time.Sleep(100 * time.Millisecond)

	// Resolve all
	for i := 0; i < count; i++ {
		id := fmt.Sprintf("req-%d", i)
		sess.pendingMu.Lock()
		ch := sess.pending[id].ch
		sess.pendingMu.Unlock()
		ch <- approvalDecision{Allow: i%2 == 0}
	}

	wg.Wait()

	for i := 0; i < count; i++ {
		expected := i%2 == 0
		if results[i] != expected {
			t.Errorf("request %d: expected %v, got %v", i, expected, results[i])
		}
	}
}

func TestApplyApprovedChunksWritesOnlySelectedHunks(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "sample.txt")
	before := strings.Join([]string{
		"alpha",
		"line-1",
		"line-2",
		"line-3",
		"line-4",
		"line-5",
		"line-6",
		"line-7",
		"line-8",
		"line-9",
		"line-10",
		"gamma",
	}, "\n") + "\n"
	after := strings.Join([]string{
		"ALPHA",
		"line-1",
		"line-2",
		"line-3",
		"line-4",
		"line-5",
		"line-6",
		"line-7",
		"line-8",
		"line-9",
		"line-10",
		"GAMMA",
	}, "\n") + "\n"
	if err := os.WriteFile(target, []byte(before), 0644); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	plan, err := buildApprovalExecutionPlan(i18n.MustLocalizer("zh-CN"), "write_file", target, before, after)
	if err != nil {
		t.Fatalf("build plan: %v", err)
	}
	if len(plan.hunks) != 2 {
		t.Fatalf("expected 2 hunks, got %d", len(plan.hunks))
	}

	result, err := applyApprovedChunks(plan, []string{plan.hunks[0].ID}, nil)
	if err != nil {
		t.Fatalf("apply approved chunks: %v", err)
	}
	if !strings.Contains(result, "Applied 1/2 approved hunks") {
		t.Fatalf("unexpected apply result: %q", result)
	}

	content, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(content) != strings.Join([]string{
		"ALPHA",
		"line-1",
		"line-2",
		"line-3",
		"line-4",
		"line-5",
		"line-6",
		"line-7",
		"line-8",
		"line-9",
		"line-10",
		"gamma",
	}, "\n")+"\n\n" {
		t.Fatalf("unexpected file content: %q", string(content))
	}
}

func TestWSProtocolHandlerUsesSessionWorkspaceRoot(t *testing.T) {
	conn := &websocket.Conn{}
	handler := &wsProtocolHandler{
		server: &WSServer{
			workspaceRoot: "/tmp/default-root",
			sessions: map[*websocket.Conn]*webSession{
				conn: {
					workspaceRoot: "/tmp/resumed-root",
				},
			},
		},
	}

	if got := handler.workspaceRootForConn(conn); got != "/tmp/resumed-root" {
		t.Fatalf("expected resumed workspace root, got %q", got)
	}
	if got := handler.workspaceRootForConn(&websocket.Conn{}); got != "/tmp/default-root" {
		t.Fatalf("expected fallback workspace root, got %q", got)
	}
}
