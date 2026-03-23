package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mison/antigravity-go/internal/llm"
)

type workspaceTrackParams struct {
	Root string `json:"root"`
}

// WorkspaceTrackTool registers a workspace root with Core's incremental workspace tracker.
func (m *CoreV2Manager) WorkspaceTrackTool() Tool {
	return Tool{
		Definition: llm.ToolDefinition{
			Name:        "workspace_track",
			Description: "Register a workspace root with Antigravity Core for incremental workspace awareness.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"root": map[string]interface{}{
						"type":        "string",
						"description": "The absolute workspace root path to track.",
					},
				},
				"required": []string{"root"},
			},
		},
		Execute: func(ctx context.Context, args json.RawMessage) (string, error) {
			var params workspaceTrackParams
			if err := json.Unmarshal(args, &params); err != nil {
				return "", err
			}
			if params.Root == "" {
				return "", fmt.Errorf("root is required")
			}

			res, err := m.workspace.Track(params.Root)
			if err != nil {
				return "", fmt.Errorf("failed to track workspace: %w", err)
			}
			data, _ := json.MarshalIndent(res, "", "  ")
			return string(data), nil
		},
	}
}
