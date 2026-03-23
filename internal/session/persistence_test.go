package session

import (
	"testing"

	"github.com/mison/antigravity-go/internal/agent"
	"github.com/mison/antigravity-go/internal/llm"
)

type fakeTrajectoryGetter struct {
	lastID  string
	payload map[string]interface{}
}

func (f *fakeTrajectoryGetter) Get(id string) (map[string]interface{}, error) {
	f.lastID = id
	return f.payload, nil
}

type fakeWorkspaceTracker struct {
	roots []string
}

func (f *fakeWorkspaceTracker) Track(root string) (map[string]interface{}, error) {
	f.roots = append(f.roots, root)
	return map[string]interface{}{"ok": true}, nil
}

func TestLoadTrajectorySnapshotParsesNestedMessages(t *testing.T) {
	getter := &fakeTrajectoryGetter{
		payload: map[string]interface{}{
			"trajectory": map[string]interface{}{
				"workspace_root": "/tmp/trajectory-workspace",
				"messages": []interface{}{
					map[string]interface{}{"role": "system", "content": "system prompt"},
					map[string]interface{}{"role": "user", "content": "continue"},
					map[string]interface{}{
						"role": "assistant",
						"tool_calls": []interface{}{
							map[string]interface{}{
								"id": "call-1",
								"function": map[string]interface{}{
									"name":      "run_command",
									"arguments": map[string]interface{}{"cmd": "pwd"},
								},
							},
						},
					},
					map[string]interface{}{
						"role":         "tool",
						"name":         "run_command",
						"tool_call_id": "call-1",
						"content":      "ok",
					},
				},
			},
		},
	}

	snapshot, err := LoadTrajectorySnapshot("traj-1", getter, "/tmp/fallback")
	if err != nil {
		t.Fatalf("LoadTrajectorySnapshot returned error: %v", err)
	}
	if getter.lastID != "traj-1" {
		t.Fatalf("expected trajectory id traj-1, got %q", getter.lastID)
	}
	if snapshot.WorkspaceRoot != "/tmp/trajectory-workspace" {
		t.Fatalf("expected workspace root from trajectory, got %q", snapshot.WorkspaceRoot)
	}
	if len(snapshot.Messages) != 4 {
		t.Fatalf("expected 4 messages, got %d", len(snapshot.Messages))
	}
	if snapshot.Messages[2].Role != llm.RoleAssistant {
		t.Fatalf("expected assistant message, got %q", snapshot.Messages[2].Role)
	}
	if len(snapshot.Messages[2].ToolCalls) != 1 {
		t.Fatalf("expected one tool call, got %d", len(snapshot.Messages[2].ToolCalls))
	}
	if got := snapshot.Messages[2].ToolCalls[0].Args; got != `{"cmd":"pwd"}` {
		t.Fatalf("expected serialized tool args, got %q", got)
	}
}

func TestRestoreAgentFromSnapshotTracksWorkspaceAndAlignsSystemPrompt(t *testing.T) {
	agt := agent.NewAgent(nil, nil, 4096)
	agt.SetSystemPrompt("base prompt")

	snapshot := &TrajectorySnapshot{
		TrajectoryID:  "traj-2",
		WorkspaceRoot: "/tmp/restored-workspace",
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: "continue"},
			{
				Role:    llm.RoleAssistant,
				Content: "done",
				ToolCalls: []llm.ToolCall{
					{ID: "call-1", Name: "run_command", Args: `{"cmd":"pwd"}`},
				},
			},
		},
	}
	tracker := &fakeWorkspaceTracker{}

	if err := RestoreAgentFromSnapshot(agt, tracker, snapshot); err != nil {
		t.Fatalf("RestoreAgentFromSnapshot returned error: %v", err)
	}
	if len(tracker.roots) != 1 || tracker.roots[0] != "/tmp/restored-workspace" {
		t.Fatalf("expected tracked workspace root, got %#v", tracker.roots)
	}

	msgs := agt.SnapshotMessages()
	if len(msgs) != 3 {
		t.Fatalf("expected injected system prompt plus 2 restored messages, got %d", len(msgs))
	}
	if msgs[0].Role != llm.RoleSystem || msgs[0].Content != "base prompt" {
		t.Fatalf("expected base system prompt injected first, got %#v", msgs[0])
	}

	expectedTokens := len("base prompt")/4 +
		len("continue")/4 +
		(len("done")+len("call-1")+len("run_command")+len(`{"cmd":"pwd"}`))/4
	if agt.GetTokenUsage() != expectedTokens {
		t.Fatalf("expected token usage %d, got %d", expectedTokens, agt.GetTokenUsage())
	}
}
