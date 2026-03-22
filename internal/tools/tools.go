package tools

import (
	"context"
	"encoding/json"

	"github.com/mison/antigravity-go/internal/llm"
)

type Executor func(ctx context.Context, args json.RawMessage) (string, error)

type Tool struct {
	Definition         llm.ToolDefinition
	Execute            Executor
	RequiresPermission bool // If true, agent needs user approval (e.g., via Y/n prompt)
}
