package server

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mison/antigravity-go/internal/agent"
	"github.com/mison/antigravity-go/internal/llm"
	"github.com/mison/antigravity-go/internal/pkg/i18n"
	"github.com/mison/antigravity-go/internal/tools"
)

type selfEvolvingProvider struct {
	targetPath   string
	finalContent string
}

func (p *selfEvolvingProvider) Chat(ctx context.Context, messages []llm.Message, toolDefs []llm.ToolDefinition) (llm.Message, error) {
	latestUser := latestUserMessage(messages)

	if len(toolDefs) == 0 {
		return llm.Message{
			Role:    llm.RoleAssistant,
			Content: "PASS\n- reviewer ok",
		}, nil
	}

	switch latestUser {
	case "worker:first":
		return llm.Message{
			Role:    llm.RoleAssistant,
			Content: "first patch ready",
		}, nil
	case "worker:second":
		return llm.Message{
			Role:    llm.RoleAssistant,
			Content: "second patch ready",
		}, nil
	}

	if hasToolResult(messages, "write_file") {
		return llm.Message{
			Role:    llm.RoleAssistant,
			Content: "done",
		}, nil
	}

	if hasToolResult(messages, "run_parallel_workers") {
		return llm.Message{
			Role: llm.RoleAssistant,
			ToolCalls: []llm.ToolCall{
				{
					ID:   "call-write-file",
					Name: "write_file",
					Args: fmt.Sprintf(`{"path":%q,"content":%q}`, p.targetPath, p.finalContent),
				},
			},
		}, nil
	}

	return llm.Message{
		Role: llm.RoleAssistant,
		ToolCalls: []llm.ToolCall{
			{
				ID:   "call-parallel-workers",
				Name: "run_parallel_workers",
				Args: `{"tasks":[{"id":"worker-1","label":"first","input":"worker:first"},{"id":"worker-2","label":"second","input":"worker:second"}]}`,
			},
		},
	}, nil
}

func (p *selfEvolvingProvider) StreamChat(ctx context.Context, messages []llm.Message, toolDefs []llm.ToolDefinition, cb llm.StreamCallback) (llm.Message, error) {
	resp, err := p.Chat(ctx, messages, toolDefs)
	if err != nil {
		return llm.Message{}, err
	}
	if cb != nil && resp.Content != "" {
		cb(resp.Content, nil)
	}
	return resp, nil
}

func latestUserMessage(messages []llm.Message) string {
	for index := len(messages) - 1; index >= 0; index -= 1 {
		if messages[index].Role == llm.RoleUser {
			return messages[index].Content
		}
	}
	return ""
}

func hasToolResult(messages []llm.Message, toolName string) bool {
	for _, message := range messages {
		if message.Role == llm.RoleTool && message.Name == toolName {
			return true
		}
	}
	return false
}

func TestSelfEvolvingParallelWorkersSupportChunkApproval(t *testing.T) {
	workspace := t.TempDir()
	targetPath := filepath.Join(workspace, "sample.go")
	before := strings.Join([]string{
		"package sample",
		"",
		"const First = \"before\"",
		"",
		"func FirstValue() string {",
		"\treturn First",
		"}",
		"",
		"// spacer-1",
		"// spacer-2",
		"// spacer-3",
		"// spacer-4",
		"// spacer-5",
		"// spacer-6",
		"// spacer-7",
		"// spacer-8",
		"// spacer-9",
		"",
		"const Second = \"before\"",
		"",
		"func SecondValue() string {",
		"\treturn Second",
		"}",
	}, "\n")
	after := strings.ReplaceAll(before, `"before"`, `"after"`)
	if err := os.WriteFile(targetPath, []byte(before), 0644); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	provider := &selfEvolvingProvider{
		targetPath:   targetPath,
		finalContent: after,
	}

	approvedChunkIDs := make([]string, 0, 2)
	agt := agent.NewAgent(provider, func(req agent.PermissionRequest) agent.PermissionDecision {
		if req.ToolName != "write_file" {
			return agent.PermissionDecision{Allow: true}
		}

		beforeText, _ := req.Metadata["approval_before"].(string)
		targetPath, _ := req.Metadata["approval_target_path"].(string)
		afterTextBytes, err := os.ReadFile(targetPath)
		if err != nil {
			t.Fatalf("read after text: %v", err)
		}
		plan, err := buildApprovalExecutionPlan(i18n.MustLocalizer("zh-CN"), req.ToolName, targetPath, beforeText, string(afterTextBytes))
		if err != nil {
			t.Fatalf("expected approval plan for write_file: %v", err)
		}
		payload := approvalRequestPayload{Metadata: map[string]any{}}
		attachApprovalChunks(i18n.MustLocalizer("zh-CN"), &payload, plan)
		if len(payload.Chunks) != 2 {
			t.Fatalf("expected 2 approval chunks, got %d", len(payload.Chunks))
		}

		approvedChunkIDs = []string{plan.hunks[0].ID, plan.hunks[1].ID}
		result, err := applyApprovedChunks(plan, approvedChunkIDs, nil)
		if err != nil {
			t.Fatalf("applyApprovedChunks failed: %v", err)
		}

		return agent.PermissionDecision{
			Allow:            true,
			Applied:          true,
			Result:           result,
			ApprovedChunkIDs: approvedChunkIDs,
		}
	}, 4096)
	agt.SetWorkspaceContext(agent.WorkspaceContext{Root: workspace})
	agt.RegisterTool(agt.GetParallelWorkerTool())
	agt.RegisterTool(tools.NewWriteFileTool())
	agt.RegisterTool(tools.Tool{
		Definition: llm.ToolDefinition{Name: "get_validation_states"},
		Execute: func(ctx context.Context, args json.RawMessage) (string, error) {
			return `{"status":"passed"}`, nil
		},
	})
	agt.RegisterTool(tools.Tool{
		Definition: llm.ToolDefinition{Name: "run_command"},
		Execute: func(ctx context.Context, args json.RawMessage) (string, error) {
			return "ok", nil
		},
	})

	result, err := agt.Run(context.Background(), "run self-evolving test", nil)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if result != "done" {
		t.Fatalf("unexpected final result: %q", result)
	}
	if len(approvedChunkIDs) != 2 {
		t.Fatalf("expected 2 approved chunk ids, got %d", len(approvedChunkIDs))
	}

	content, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(content) != after+"\n" {
		t.Fatalf("unexpected file content after approval: %q", string(content))
	}
}
