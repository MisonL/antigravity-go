package agent

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mison/antigravity-go/internal/llm"
	"github.com/mison/antigravity-go/internal/pkg/i18n"
	"github.com/mison/antigravity-go/internal/tools"
)

func TestApplyCoreEditRunsReviewBeforeHumanApproval(t *testing.T) {
	provider := &scriptedProvider{
		responses: []llm.Message{
			{
				Role: llm.RoleAssistant,
				ToolCalls: []llm.ToolCall{
					{ID: "call-1", Name: applyCoreEditToolName, Args: `{"filePath":"main.go","edits":[]}`},
				},
			},
			{Role: llm.RoleAssistant, Content: "PASS\n- review ok"},
			{Role: llm.RoleAssistant, Content: "done"},
		},
	}

	var seq []string
	var agt *Agent
	agt = NewAgent(provider, func(req PermissionRequest) PermissionDecision {
		seq = append(seq, "approval")
		if req.Summary != i18n.MustLocalizer(agt.Locale()).T("agent.permission.final_confirmation") {
			t.Fatalf("unexpected approval summary: %q", req.Summary)
		}
		if !strings.Contains(req.Preview, autoReviewHeader) {
			t.Fatalf("expected auto-review preview, got %q", req.Preview)
		}
		return PermissionDecision{Allow: true}
	}, 4096)

	registerTestTool(agt, trajectoryListName, func(ctx context.Context, args json.RawMessage) (string, error) {
		seq = append(seq, "trajectory_list")
		return `{"items":[]}`, nil
	})
	registerTestTool(agt, applyCoreEditToolName, func(ctx context.Context, args json.RawMessage) (string, error) {
		seq = append(seq, "edit")
		return `{"ok":true}`, nil
	})
	registerTestTool(agt, validationToolName, func(ctx context.Context, args json.RawMessage) (string, error) {
		seq = append(seq, "validation")
		return `{"status":"passed"}`, nil
	})
	registerTestTool(agt, runCommandToolName, func(ctx context.Context, args json.RawMessage) (string, error) {
		seq = append(seq, "test")
		return "ok", nil
	})
	registerTestTool(agt, diagnosticsToolName, func(ctx context.Context, args json.RawMessage) (string, error) {
		return `{"diagnostics":[]}`, nil
	})

	if _, err := agt.Run(context.Background(), "fix it", nil); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	got := strings.Join(seq, "->")
	if got != "trajectory_list->edit->validation->test->approval" {
		t.Fatalf("unexpected execution order: %s", got)
	}
}

func TestApplyCoreEditFailureRollsBackToPreviousStep(t *testing.T) {
	provider := &scriptedProvider{
		responses: []llm.Message{
			{
				Role: llm.RoleAssistant,
				ToolCalls: []llm.ToolCall{
					{ID: "call-1", Name: applyCoreEditToolName, Args: `{"filePath":"main.go","edits":[]}`},
				},
			},
			{Role: llm.RoleAssistant, Content: "FAIL\n- validation failed"},
			{Role: llm.RoleAssistant, Content: "done"},
		},
	}

	agt := NewAgent(provider, func(req PermissionRequest) PermissionDecision {
		t.Fatal("permission should not be requested when auto review fails")
		return PermissionDecision{Allow: false}
	}, 4096)

	registerTestTool(agt, trajectoryListName, func(ctx context.Context, args json.RawMessage) (string, error) {
		return `{"items":[{"id":"traj-1","updated_at":"2026-03-23T00:00:00Z"}]}`, nil
	})
	registerTestTool(agt, trajectoryGetName, func(ctx context.Context, args json.RawMessage) (string, error) {
		return `{"steps":[{"step_id":"step-before"}]}`, nil
	})
	registerTestTool(agt, applyCoreEditToolName, func(ctx context.Context, args json.RawMessage) (string, error) {
		return `{"ok":true}`, nil
	})
	registerTestTool(agt, validationToolName, func(ctx context.Context, args json.RawMessage) (string, error) {
		return `{"status":"failed"}`, nil
	})
	registerTestTool(agt, runCommandToolName, func(ctx context.Context, args json.RawMessage) (string, error) {
		return "ok", nil
	})
	registerTestTool(agt, rollbackToolName, func(ctx context.Context, args json.RawMessage) (string, error) {
		if string(args) != `{"step_id":"step-before"}` {
			t.Fatalf("unexpected rollback args: %s", string(args))
		}
		return `{"ok":true}`, nil
	})

	if _, err := agt.Run(context.Background(), "fix it", nil); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	var toolMessage llm.Message
	for _, msg := range agt.SnapshotMessages() {
		if msg.Role == llm.RoleTool && msg.Name == applyCoreEditToolName {
			toolMessage = msg
			break
		}
	}
	if !strings.Contains(toolMessage.Content, rollbackToolName) {
		t.Fatalf("expected rollback message in tool output, got %q", toolMessage.Content)
	}
}

func TestWriteFileFailureRestoresSnapshot(t *testing.T) {
	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "sample.txt")
	if err := os.WriteFile(target, []byte("before"), 0644); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	provider := &scriptedProvider{
		responses: []llm.Message{
			{
				Role: llm.RoleAssistant,
				ToolCalls: []llm.ToolCall{
					{ID: "call-1", Name: writeFileToolName, Args: `{"path":"` + target + `","content":"after"}`},
				},
			},
			{Role: llm.RoleAssistant, Content: "FAIL\n- write rejected"},
			{Role: llm.RoleAssistant, Content: "done"},
		},
	}

	agt := NewAgent(provider, func(req PermissionRequest) PermissionDecision {
		t.Fatal("permission should not be requested when auto review fails")
		return PermissionDecision{Allow: false}
	}, 4096)

	registerTestTool(agt, writeFileToolName, func(ctx context.Context, args json.RawMessage) (string, error) {
		var params struct {
			Path    string `json:"path"`
			Content string `json:"content"`
		}
		if err := json.Unmarshal(args, &params); err != nil {
			return "", err
		}
		return "ok", os.WriteFile(params.Path, []byte(params.Content), 0644)
	})
	registerTestTool(agt, validationToolName, func(ctx context.Context, args json.RawMessage) (string, error) {
		return `{"status":"failed"}`, nil
	})
	registerTestTool(agt, runCommandToolName, func(ctx context.Context, args json.RawMessage) (string, error) {
		return "ok", nil
	})

	if _, err := agt.Run(context.Background(), "fix it", nil); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	content, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read restored file: %v", err)
	}
	if string(content) != "before" {
		t.Fatalf("expected snapshot restore, got %q", string(content))
	}
}

func registerTestTool(agt *Agent, name string, exec func(context.Context, json.RawMessage) (string, error)) {
	agt.RegisterTool(tools.Tool{
		Definition: llm.ToolDefinition{Name: name},
		Execute:    exec,
	})
}
