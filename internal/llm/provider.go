package llm

import (
	"context"
)

type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

type Message struct {
	Role       Role
	Content    string
	Name       string     // For Tool role
	ToolCalls  []ToolCall // For Assistant role
	ToolCallID string     // For Tool role
}

type ToolCall struct {
	ID   string
	Name string
	Args string // JSON string
}

type ToolDefinition struct {
	Name        string
	Description string
	Parameters  interface{} // JSON Schema
}

type StreamCallback func(chunk string, err error)

type Provider interface {
	Chat(ctx context.Context, messages []Message, tools []ToolDefinition) (Message, error)
	StreamChat(ctx context.Context, messages []Message, tools []ToolDefinition, cb StreamCallback) (Message, error)
}
