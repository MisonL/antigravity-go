package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/mison/antigravity-go/internal/llm"
	"github.com/mison/antigravity-go/internal/tools"
)

type scriptedProvider struct {
	responses []llm.Message
	index     int
}

func (p *scriptedProvider) Chat(ctx context.Context, messages []llm.Message, toolDefs []llm.ToolDefinition) (llm.Message, error) {
	if p.index >= len(p.responses) {
		return llm.Message{}, fmt.Errorf("unexpected provider call %d", p.index)
	}
	resp := p.responses[p.index]
	p.index++
	return resp, nil
}

func (p *scriptedProvider) StreamChat(ctx context.Context, messages []llm.Message, toolDefs []llm.ToolDefinition, cb llm.StreamCallback) (llm.Message, error) {
	resp, err := p.Chat(ctx, messages, toolDefs)
	if err != nil {
		return llm.Message{}, err
	}
	if cb != nil && resp.Content != "" {
		cb(resp.Content, nil)
	}
	return resp, nil
}

func TestRunAddsDiagnosticsFeedbackAfterCodeEdit(t *testing.T) {
	provider := &scriptedProvider{
		responses: []llm.Message{
			{
				Role: llm.RoleAssistant,
				ToolCalls: []llm.ToolCall{
					{ID: "call-1", Name: "apply_core_edit", Args: "{}"},
				},
			},
			{
				Role:    llm.RoleAssistant,
				Content: "PASS\n- auto review ok",
			},
			{
				Role:    llm.RoleAssistant,
				Content: "done",
			},
		},
	}

	agt := NewAgent(provider, nil, 4096)

	editCalls := 0
	diagCalls := 0

	agt.RegisterTool(tools.Tool{
		Definition: llm.ToolDefinition{Name: "apply_core_edit"},
		Execute: func(ctx context.Context, args json.RawMessage) (string, error) {
			editCalls++
			return "edit applied", nil
		},
	})
	agt.RegisterTool(tools.Tool{
		Definition: llm.ToolDefinition{Name: validationToolName},
		Execute: func(ctx context.Context, args json.RawMessage) (string, error) {
			return `{"status":"passed"}`, nil
		},
	})
	agt.RegisterTool(tools.Tool{
		Definition: llm.ToolDefinition{Name: runCommandToolName},
		Execute: func(ctx context.Context, args json.RawMessage) (string, error) {
			return "ok", nil
		},
	})
	agt.RegisterTool(tools.Tool{
		Definition: llm.ToolDefinition{Name: diagnosticsToolName},
		Execute: func(ctx context.Context, args json.RawMessage) (string, error) {
			diagCalls++
			return "{\"diagnostics\":[]}", nil
		},
	})

	result, err := agt.Run(context.Background(), "fix it", nil)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if result != "done" {
		t.Fatalf("unexpected final result: %q", result)
	}
	if editCalls != 1 {
		t.Fatalf("expected 1 edit call, got %d", editCalls)
	}
	if diagCalls != 1 {
		t.Fatalf("expected 1 diagnostics call, got %d", diagCalls)
	}

	var toolMessage llm.Message
	for _, msg := range agt.SnapshotMessages() {
		if msg.Role == llm.RoleTool && msg.Name == "apply_core_edit" {
			toolMessage = msg
			break
		}
	}
	if toolMessage.Content == "" {
		t.Fatal("expected apply_core_edit tool message")
	}
	if strings.Count(toolMessage.Content, cseFeedbackHeader) != 1 {
		t.Fatalf("expected exactly one CSE feedback block, got %q", toolMessage.Content)
	}
	if !strings.Contains(toolMessage.Content, "{\"diagnostics\":[]}") {
		t.Fatalf("expected diagnostics payload in tool message, got %q", toolMessage.Content)
	}
}

func TestRunFailsWhenProviderIsMissing(t *testing.T) {
	agt := NewAgent(nil, nil, 1024)

	if _, err := agt.Run(context.Background(), "hello", nil); err == nil {
		t.Fatal("expected Run to fail when provider is missing")
	}

	if err := agt.RunStream(context.Background(), "hello", nil, nil); err == nil {
		t.Fatal("expected RunStream to fail when provider is missing")
	}
}

func TestRunFinalizesTaskWithMemorySave(t *testing.T) {
	provider := &scriptedProvider{
		responses: []llm.Message{
			{
				Role:    llm.RoleAssistant,
				Content: "done",
			},
		},
	}

	agt := NewAgent(provider, nil, 4096)

	var payload struct {
		Request struct {
			Content  string                 `json:"content"`
			Metadata map[string]interface{} `json:"metadata"`
		} `json:"request"`
	}
	memoryCalls := 0

	agt.RegisterTool(tools.Tool{
		Definition: llm.ToolDefinition{Name: memorySaveToolName},
		Execute: func(ctx context.Context, args json.RawMessage) (string, error) {
			memoryCalls++
			if err := json.Unmarshal(args, &payload); err != nil {
				t.Fatalf("unexpected payload: %v", err)
			}
			return `{"ok":true}`, nil
		},
	})

	result, err := agt.Run(context.Background(), "实现版本管理能力", nil)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if result != "done" {
		t.Fatalf("unexpected final result: %q", result)
	}
	if memoryCalls != 1 {
		t.Fatalf("expected 1 memory save call, got %d", memoryCalls)
	}
	if !strings.Contains(payload.Request.Content, "实现版本管理能力") {
		t.Fatalf("expected task input in memory content, got %q", payload.Request.Content)
	}
	if payload.Request.Metadata["category"] != "architecture_decision" {
		t.Fatalf("expected architecture_decision metadata, got %#v", payload.Request.Metadata)
	}
}

func TestRunIgnoresFinalizeTaskFailure(t *testing.T) {
	provider := &scriptedProvider{
		responses: []llm.Message{
			{
				Role:    llm.RoleAssistant,
				Content: "done",
			},
		},
	}

	agt := NewAgent(provider, nil, 4096)
	agt.RegisterTool(tools.Tool{
		Definition: llm.ToolDefinition{Name: memorySaveToolName},
		Execute: func(ctx context.Context, args json.RawMessage) (string, error) {
			return "", fmt.Errorf("memory offline")
		},
	})

	result, err := agt.Run(context.Background(), "实现版本管理能力", nil)
	if err != nil {
		t.Fatalf("expected finalize failure to be ignored, got %v", err)
	}
	if result != "done" {
		t.Fatalf("unexpected final result: %q", result)
	}
}

func TestRunStreamIgnoresFinalizeTaskFailure(t *testing.T) {
	provider := &scriptedProvider{
		responses: []llm.Message{
			{
				Role:    llm.RoleAssistant,
				Content: "done",
			},
		},
	}

	agt := NewAgent(provider, nil, 4096)
	agt.RegisterTool(tools.Tool{
		Definition: llm.ToolDefinition{Name: memorySaveToolName},
		Execute: func(ctx context.Context, args json.RawMessage) (string, error) {
			return "", fmt.Errorf("memory offline")
		},
	})

	var chunks []string
	err := agt.RunStream(context.Background(), "实现版本管理能力", func(chunk string, err error) {
		if err != nil {
			t.Fatalf("unexpected stream callback error: %v", err)
		}
		chunks = append(chunks, chunk)
	}, nil)
	if err != nil {
		t.Fatalf("expected finalize failure to be ignored in stream mode, got %v", err)
	}
	if len(chunks) != 1 || chunks[0] != "done" {
		t.Fatalf("unexpected chunks: %#v", chunks)
	}
}
