package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mison/antigravity-go/internal/llm"
)

type commitMessageGenerateParams struct {
	Diff string `json:"diff"`
}

type rollbackToStepParams struct {
	StepID string `json:"step_id"`
}

// CommitMessageGenerateTool generates a git commit message from a diff.
func (m *CoreV2Manager) CommitMessageGenerateTool() Tool {
	return Tool{
		Definition: llm.ToolDefinition{
			Name:        "commit_message_generate",
			Description: "Generate a git commit message from a unified diff through Antigravity Core's versioning plane.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"diff": map[string]interface{}{
						"type":        "string",
						"description": "The unified diff content to summarize into a commit message.",
					},
				},
				"required": []string{"diff"},
			},
		},
		Execute: func(ctx context.Context, args json.RawMessage) (string, error) {
			var params commitMessageGenerateParams
			if err := json.Unmarshal(args, &params); err != nil {
				return "", err
			}
			if params.Diff == "" {
				return "", fmt.Errorf("diff is required")
			}

			res, err := m.versioning.GenerateCommit(params.Diff)
			if err != nil {
				return "", fmt.Errorf("failed to generate commit message: %w", err)
			}
			data, _ := json.MarshalIndent(res, "", "  ")
			return string(data), nil
		},
	}
}

// RollbackToStepTool reverts the workspace to a trajectory step through Core.
func (m *CoreV2Manager) RollbackToStepTool() Tool {
	return Tool{
		Definition: llm.ToolDefinition{
			Name:        "rollback_to_step",
			Description: "Rollback the workspace to a specific cascade trajectory step through Antigravity Core's versioning plane.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"step_id": map[string]interface{}{
						"type":        "string",
						"description": "The cascade step ID to roll back to.",
					},
				},
				"required": []string{"step_id"},
			},
		},
		Execute: func(ctx context.Context, args json.RawMessage) (string, error) {
			var params rollbackToStepParams
			if err := json.Unmarshal(args, &params); err != nil {
				return "", err
			}
			if params.StepID == "" {
				return "", fmt.Errorf("step_id is required")
			}

			res, err := m.versioning.Rollback(params.StepID)
			if err != nil {
				return "", fmt.Errorf("failed to rollback to step: %w", err)
			}
			data, _ := json.MarshalIndent(res, "", "  ")
			return string(data), nil
		},
		RequiresPermission: true,
	}
}
