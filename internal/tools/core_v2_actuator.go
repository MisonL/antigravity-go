package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mison/antigravity-go/internal/llm"
)

type actuatorPreviewParams struct {
	Request map[string]interface{} `json:"request"`
}

// EditPreviewTool previews a code edit through Core's actuation plane.
func (m *CoreV2Manager) EditPreviewTool() Tool {
	return Tool{
		Definition: llm.ToolDefinition{
			Name:        "edit_preview",
			Description: "Preview a pending code edit through Antigravity Core and return the generated patch diff. Use the raw Core JSON request fields under request.",
			Parameters: rawCoreRequestSchema(
				"Raw GetPatchAndCodeChange request body using antigravity_core JSON field names.",
			),
		},
		Execute: func(ctx context.Context, args json.RawMessage) (string, error) {
			var params actuatorPreviewParams
			if err := json.Unmarshal(args, &params); err != nil {
				return "", err
			}
			if params.Request == nil {
				return "", fmt.Errorf("request is required")
			}

			res, err := m.actuator.PreviewEdit(params.Request)
			if err != nil {
				return "", fmt.Errorf("failed to preview edit: %w", err)
			}
			data, _ := json.MarshalIndent(res, "", "  ")
			return string(data), nil
		},
	}
}

// GetValidationStatesTool returns the workspace validation state from Core.
func (m *CoreV2Manager) GetValidationStatesTool() Tool {
	return Tool{
		Definition: llm.ToolDefinition{
			Name:        "get_validation_states",
			Description: "Fetch the current workspace validation state from Antigravity Core, including available code validation results.",
			Parameters: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		Execute: func(ctx context.Context, args json.RawMessage) (string, error) {
			res, err := m.actuator.GetValidation()
			if err != nil {
				return "", fmt.Errorf("failed to get validation states: %w", err)
			}
			data, _ := json.MarshalIndent(res, "", "  ")
			return string(data), nil
		},
	}
}
