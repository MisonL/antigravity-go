package session

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mison/antigravity-go/internal/agent"
	"github.com/mison/antigravity-go/internal/llm"
)

type TrajectoryGetter interface {
	Get(id string) (map[string]interface{}, error)
}

type WorkspaceTracker interface {
	Track(root string) (map[string]interface{}, error)
}

type TrajectorySnapshot struct {
	TrajectoryID  string
	WorkspaceRoot string
	Messages      []llm.Message
}

func LoadTrajectorySnapshot(id string, getter TrajectoryGetter, fallbackWorkspace string) (*TrajectorySnapshot, error) {
	if getter == nil {
		return nil, fmt.Errorf("trajectory getter is not initialized")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("trajectory id is required")
	}

	payload, err := getter.Get(id)
	if err != nil {
		return nil, err
	}
	return DecodeTrajectorySnapshot(id, payload, fallbackWorkspace)
}

func DecodeTrajectorySnapshot(id string, payload map[string]interface{}, fallbackWorkspace string) (*TrajectorySnapshot, error) {
	if len(payload) == 0 {
		return nil, fmt.Errorf("trajectory payload is empty")
	}

	messages := extractMessages(payload)
	if len(messages) == 0 {
		return nil, fmt.Errorf("trajectory %q does not contain restorable messages", id)
	}

	workspaceRoot := normalizeWorkspaceRoot(extractWorkspaceRoot(payload, fallbackWorkspace))
	if workspaceRoot == "" {
		workspaceRoot = normalizeWorkspaceRoot(fallbackWorkspace)
	}

	return &TrajectorySnapshot{
		TrajectoryID:  strings.TrimSpace(id),
		WorkspaceRoot: workspaceRoot,
		Messages:      cloneMessages(messages),
	}, nil
}

func RestoreAgentFromSnapshot(agt *agent.Agent, tracker WorkspaceTracker, snapshot *TrajectorySnapshot) error {
	if agt == nil {
		return fmt.Errorf("agent is not initialized")
	}
	if snapshot == nil {
		return fmt.Errorf("trajectory snapshot is nil")
	}

	msgs := cloneMessages(snapshot.Messages)
	if len(msgs) == 0 {
		return fmt.Errorf("trajectory snapshot has no messages")
	}
	if msgs[0].Role != llm.RoleSystem {
		if prompt := strings.TrimSpace(agt.GetSystemPrompt()); prompt != "" {
			msgs = append([]llm.Message{{
				Role:    llm.RoleSystem,
				Content: prompt,
			}}, msgs...)
		}
	}

	if tracker != nil && snapshot.WorkspaceRoot != "" {
		if _, err := tracker.Track(snapshot.WorkspaceRoot); err != nil {
			return fmt.Errorf("track workspace %q: %w", snapshot.WorkspaceRoot, err)
		}
	}

	agt.LoadMessages(msgs)
	return nil
}

func extractMessages(value interface{}) []llm.Message {
	switch v := value.(type) {
	case map[string]interface{}:
		for _, key := range []string{"messages", "conversation", "history", "events", "timeline"} {
			if messages := parseMessageArray(v[key]); len(messages) > 0 {
				return messages
			}
		}
		for _, key := range []string{"trajectory", "snapshot", "data", "result", "payload", "record", "session"} {
			if messages := extractMessages(v[key]); len(messages) > 0 {
				return messages
			}
		}
		for _, nested := range v {
			if messages := extractMessages(nested); len(messages) > 0 {
				return messages
			}
		}
	case []interface{}:
		if messages := parseMessageArray(v); len(messages) > 0 {
			return messages
		}
		for _, nested := range v {
			if messages := extractMessages(nested); len(messages) > 0 {
				return messages
			}
		}
	}

	return nil
}

func parseMessageArray(value interface{}) []llm.Message {
	items, ok := value.([]interface{})
	if !ok || len(items) == 0 {
		return nil
	}

	messages := make([]llm.Message, 0, len(items))
	for _, item := range items {
		msg, ok := normalizeMessage(item)
		if !ok {
			continue
		}
		messages = append(messages, msg)
	}
	if len(messages) == 0 {
		return nil
	}
	return messages
}

func normalizeMessage(value interface{}) (llm.Message, bool) {
	record, ok := value.(map[string]interface{})
	if !ok {
		return llm.Message{}, false
	}
	for _, key := range []string{"message", "entry", "payload"} {
		if nested, ok := record[key].(map[string]interface{}); ok {
			record = nested
			break
		}
	}

	role := normalizeRole(firstString(record, "role", "sender", "type", "kind", "event_type"))
	if role == "" {
		return llm.Message{}, false
	}

	msg := llm.Message{
		Role:       role,
		Content:    firstValue(record["content"], record["text"], record["value"], record["message"]),
		Name:       firstString(record, "name", "tool_name", "toolName", "tool"),
		ToolCallID: firstString(record, "tool_call_id", "toolCallId", "call_id", "callId"),
		ToolCalls:  parseToolCalls(record["tool_calls"], record["toolCalls"], record["calls"]),
	}

	if role == llm.RoleAssistant && strings.TrimSpace(msg.Content) == "" && len(msg.ToolCalls) == 0 {
		return llm.Message{}, false
	}
	if role != llm.RoleAssistant && role != llm.RoleTool && strings.TrimSpace(msg.Content) == "" {
		return llm.Message{}, false
	}
	if role == llm.RoleTool && msg.Name == "" {
		msg.Name = firstString(record, "call_name", "callName")
	}

	return msg, true
}

func parseToolCalls(values ...interface{}) []llm.ToolCall {
	for _, value := range values {
		items, ok := value.([]interface{})
		if !ok || len(items) == 0 {
			continue
		}

		out := make([]llm.ToolCall, 0, len(items))
		for _, item := range items {
			record, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			fn := record
			if nested, ok := record["function"].(map[string]interface{}); ok {
				fn = nested
			}

			call := llm.ToolCall{
				ID:   firstString(record, "id", "call_id", "callId"),
				Name: firstString(fn, "name", "tool_name", "toolName", "tool"),
				Args: firstValue(fn["args"], fn["arguments"], record["args"], record["arguments"], record["input"]),
			}
			if call.Name == "" {
				continue
			}
			out = append(out, call)
		}
		if len(out) > 0 {
			return out
		}
	}
	return nil
}

func extractWorkspaceRoot(payload map[string]interface{}, fallback string) string {
	if root := searchString(payload, map[string]struct{}{
		"workspace_root": {},
		"workspaceRoot":  {},
		"root":           {},
		"cwd":            {},
		"workspace":      {},
	}); root != "" {
		return root
	}
	return fallback
}

func searchString(value interface{}, keys map[string]struct{}) string {
	switch v := value.(type) {
	case map[string]interface{}:
		for key, nested := range v {
			if _, ok := keys[key]; ok {
				if text := searchString(nested, keys); text != "" {
					return text
				}
				if text := stringifyValue(nested); text != "" {
					return text
				}
			}
		}
		for _, nested := range v {
			if text := searchString(nested, keys); text != "" {
				return text
			}
		}
	case []interface{}:
		for _, nested := range v {
			if text := searchString(nested, keys); text != "" {
				return text
			}
		}
	}
	return ""
}

func normalizeWorkspaceRoot(root string) string {
	root = strings.TrimSpace(root)
	if root == "" {
		return ""
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return root
	}
	return abs
}

func firstString(record map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if text := stringifyValue(record[key]); text != "" {
			return text
		}
	}
	return ""
}

func firstValue(values ...interface{}) string {
	for _, value := range values {
		if text := stringifyValue(value); text != "" {
			return text
		}
	}
	return ""
}

func stringifyValue(value interface{}) string {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case float64, bool, int:
		return strings.TrimSpace(fmt.Sprint(v))
	case nil:
		return ""
	default:
		data, err := json.Marshal(v)
		if err != nil {
			return ""
		}
		return strings.TrimSpace(string(data))
	}
}

func normalizeRole(raw string) llm.Role {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "system":
		return llm.RoleSystem
	case "user", "human", "input", "user_message":
		return llm.RoleUser
	case "assistant", "ai", "model", "assistant_message":
		return llm.RoleAssistant
	case "tool", "function", "tool_result":
		return llm.RoleTool
	default:
		return ""
	}
}

func cloneMessages(messages []llm.Message) []llm.Message {
	out := make([]llm.Message, len(messages))
	for i := range messages {
		out[i] = messages[i]
		if len(messages[i].ToolCalls) > 0 {
			out[i].ToolCalls = append([]llm.ToolCall(nil), messages[i].ToolCalls...)
		}
	}
	return out
}
