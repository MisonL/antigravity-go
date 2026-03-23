package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mison/antigravity-go/internal/llm"
)

type memoryToolParams struct {
	Request map[string]interface{} `json:"request"`
}

func rawCoreRequestSchema(description string) map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"request": map[string]interface{}{
				"type":                 "object",
				"description":          description,
				"additionalProperties": true,
			},
		},
		"required": []string{"request"},
	}
}

// MemorySaveTool persists data into Core's Memory Plane.
func (m *CoreV2Manager) MemorySaveTool() Tool {
	return Tool{
		Definition: llm.ToolDefinition{
			Name:        "memory_save",
			Description: "Write data to Antigravity Core's Memory Plane through UpdateCascadeMemory. Use the raw Core JSON request fields under request.",
			Parameters: rawCoreRequestSchema(
				"Raw UpdateCascadeMemory request body using antigravity_core JSON field names.",
			),
		},
		Execute: func(ctx context.Context, args json.RawMessage) (string, error) {
			var params memoryToolParams
			if err := json.Unmarshal(args, &params); err != nil {
				return "", err
			}
			if params.Request == nil {
				return "", fmt.Errorf("request is required")
			}

			res, err := m.memory.Save(params.Request)
			if err != nil {
				return "", fmt.Errorf("failed to save memory: %w", err)
			}
			data, _ := json.MarshalIndent(res, "", "  ")
			return string(data), nil
		},
	}
}

// MemoryQueryTool retrieves data from Core's Memory Plane.
func (m *CoreV2Manager) MemoryQueryTool() Tool {
	return Tool{
		Definition: llm.ToolDefinition{
			Name:        "memory_query",
			Description: "Query Antigravity Core's Memory Plane through GetUserMemories. Use the raw Core JSON request fields under request.",
			Parameters: rawCoreRequestSchema(
				"Raw GetUserMemories request body using antigravity_core JSON field names.",
			),
		},
		Execute: func(ctx context.Context, args json.RawMessage) (string, error) {
			var params memoryToolParams
			if err := json.Unmarshal(args, &params); err != nil {
				return "", err
			}
			if params.Request == nil {
				return "", fmt.Errorf("request is required")
			}

			res, err := m.memory.Query(params.Request)
			if err != nil {
				return "", fmt.Errorf("failed to query memories: %w", err)
			}
			data, _ := json.MarshalIndent(res, "", "  ")
			return string(data), nil
		},
	}
}
