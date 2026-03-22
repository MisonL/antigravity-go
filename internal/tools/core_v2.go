package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mison/antigravity-go/internal/llm"
	"github.com/mison/antigravity-go/internal/rpc"
)

// CoreV2Manager wraps the rpc.Client to provide high-level tools.
type CoreV2Manager struct {
	client *rpc.Client
}

func NewCoreV2Manager(client *rpc.Client) *CoreV2Manager {
	return &CoreV2Manager{client: client}
}

// GetMcpStatesTool returns a tool that lists MCP servers managed by Core.
func (m *CoreV2Manager) GetMcpStatesTool() Tool {
	return Tool{
		Definition: llm.ToolDefinition{
			Name:        "get_core_mcp_states",
			Description: "List all MCP servers and tools currently managed by the Antigravity Core engine.",
			Parameters: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		Execute: func(ctx context.Context, args json.RawMessage) (string, error) {
			states, err := m.client.GetMcpServerStates()
			if err != nil {
				return "", fmt.Errorf("failed to get mcp states: %w", err)
			}
			data, _ := json.MarshalIndent(states, "", "  ")
			return string(data), nil
		},
	}
}

// ApplyCoreEditTool returns a tool that uses Core's native ApplyCodeEdit interface.
func (m *CoreV2Manager) ApplyCoreEditTool() Tool {
	return Tool{
		Definition: llm.ToolDefinition{
			Name:        "apply_core_edit",
			Description: "Apply code edits to the workspace using the Core engine's native editing capability. This is often safer and more robust than direct file writes.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"filePath": map[string]interface{}{
						"type":        "string",
						"description": "The absolute path of the file to edit.",
					},
					"edits": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"range": map[string]interface{}{
									"type": "object",
									"properties": map[string]interface{}{
										"start": map[string]interface{}{
											"type": "object",
											"properties": map[string]interface{}{
												"line":      map[string]interface{}{"type": "integer"},
												"character": map[string]interface{}{"type": "integer"},
											},
										},
										"end": map[string]interface{}{
											"type": "object",
											"properties": map[string]interface{}{
												"line":      map[string]interface{}{"type": "integer"},
												"character": map[string]interface{}{"type": "integer"},
											},
										},
									},
								},
								"newText": map[string]interface{}{"type": "string"},
							},
						},
					},
				},
				"required": []string{"filePath", "edits"},
			},
		},
		Execute: func(ctx context.Context, args json.RawMessage) (string, error) {
			var edit map[string]interface{}
			if err := json.Unmarshal(args, &edit); err != nil {
				return "", err
			}
			res, err := m.client.ApplyCodeEdit(edit)
			if err != nil {
				return "", fmt.Errorf("failed to apply core edit: %w", err)
			}
			data, _ := json.MarshalIndent(res, "", "  ")
			return string(data), nil
		},
		RequiresPermission: true,
	}
}
