package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mison/antigravity-go/internal/llm"
)

type trajectoryExportParams struct {
	ID string `json:"id"`
}

// TrajectoryGetTool fetches one trajectory in raw JSON form.
func (m *CoreV2Manager) TrajectoryGetTool() Tool {
	return Tool{
		Definition: llm.ToolDefinition{
			Name:        "trajectory_get",
			Description: "Fetch a single trajectory from Antigravity Core's Trajectory Plane.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{
						"type":        "string",
						"description": "The trajectory ID to load.",
					},
				},
				"required": []string{"id"},
			},
		},
		Execute: func(ctx context.Context, args json.RawMessage) (string, error) {
			var params trajectoryExportParams
			if err := json.Unmarshal(args, &params); err != nil {
				return "", err
			}
			if params.ID == "" {
				return "", fmt.Errorf("id is required")
			}

			res, err := m.trajectory.Get(params.ID)
			if err != nil {
				return "", fmt.Errorf("failed to get trajectory: %w", err)
			}
			data, _ := json.MarshalIndent(res, "", "  ")
			return string(data), nil
		},
	}
}

// TrajectoryListTool lists trajectories managed by Core.
func (m *CoreV2Manager) TrajectoryListTool() Tool {
	return Tool{
		Definition: llm.ToolDefinition{
			Name:        "trajectory_list",
			Description: "List all task trajectories stored in Antigravity Core's Trajectory Plane.",
			Parameters: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		Execute: func(ctx context.Context, args json.RawMessage) (string, error) {
			res, err := m.trajectory.List()
			if err != nil {
				return "", fmt.Errorf("failed to list trajectories: %w", err)
			}
			data, _ := json.MarshalIndent(res, "", "  ")
			return string(data), nil
		},
	}
}

// TrajectoryExportTool exports one trajectory as markdown text.
func (m *CoreV2Manager) TrajectoryExportTool() Tool {
	return Tool{
		Definition: llm.ToolDefinition{
			Name:        "trajectory_export",
			Description: "Export a trajectory from Antigravity Core's Trajectory Plane as markdown text.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{
						"type":        "string",
						"description": "The trajectory ID to export.",
					},
				},
				"required": []string{"id"},
			},
		},
		Execute: func(ctx context.Context, args json.RawMessage) (string, error) {
			var params trajectoryExportParams
			if err := json.Unmarshal(args, &params); err != nil {
				return "", err
			}
			if params.ID == "" {
				return "", fmt.Errorf("id is required")
			}

			markdown, err := m.trajectory.Export(params.ID)
			if err != nil {
				return "", fmt.Errorf("failed to export trajectory: %w", err)
			}
			return markdown, nil
		},
	}
}
