package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mison/antigravity-go/internal/corecap"
	"github.com/mison/antigravity-go/internal/llm"
	"github.com/mison/antigravity-go/internal/rpc"
)

// CoreV2Manager wraps the rpc.Client to provide high-level tools.
type CoreV2Manager struct {
	client     *rpc.Client
	actuator   *corecap.ActuatorManager
	browser    *corecap.BrowserManager
	mcp        *corecap.McpManager
	memory     *corecap.MemoryManager
	trajectory *corecap.TrajectoryManager
	versioning *corecap.VersioningManager
	workspace  *corecap.WorkspaceManager
}

func NewCoreV2Manager(client *rpc.Client) *CoreV2Manager {
	return &CoreV2Manager{
		client:     client,
		actuator:   corecap.NewActuatorManager(client),
		browser:    corecap.NewBrowserManager(client),
		mcp:        corecap.NewMcpManager(client),
		memory:     corecap.NewMemoryManager(client),
		trajectory: corecap.NewTrajectoryManager(client),
		versioning: corecap.NewVersioningManager(client),
		workspace:  corecap.NewWorkspaceManager(client),
	}
}

// GetMcpResourcesTool returns a tool that lists MCP resources for a server.
func (m *CoreV2Manager) GetMcpResourcesTool() Tool {
	return Tool{
		Definition: llm.ToolDefinition{
			Name:        "get_core_mcp_resources",
			Description: "List MCP resources exposed by a specific server managed by Antigravity Core.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"server": map[string]interface{}{
						"type":        "string",
						"description": "The MCP server name or identifier.",
					},
					"query": map[string]interface{}{
						"type":        "string",
						"description": "Optional filter text forwarded to the core resource listing RPC.",
					},
				},
				"required": []string{"server"},
			},
		},
		Execute: func(ctx context.Context, args json.RawMessage) (string, error) {
			var params struct {
				Server string `json:"server"`
				Query  string `json:"query"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", err
			}
			if params.Server == "" {
				return "", fmt.Errorf("server is required")
			}

			resources, nextPageToken, err := m.mcp.ListResources(params.Server, "", params.Query)
			if err != nil {
				return "", fmt.Errorf("failed to list mcp resources: %w", err)
			}
			data, _ := json.MarshalIndent(map[string]interface{}{
				"server":          params.Server,
				"resources":       resources,
				"next_page_token": nextPageToken,
			}, "", "  ")
			return string(data), nil
		},
	}
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

// GetCoreDiagnosticsTool returns a tool that fetches project-wide diagnostics from Core RPC.
func (m *CoreV2Manager) GetCoreDiagnosticsTool() Tool {
	return Tool{
		Definition: llm.ToolDefinition{
			Name:        "get_core_diagnostics",
			Description: "Fetch project-wide compilation or linting errors from the Core engine. Highly recommended after applying code changes.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "Optional file path to filter diagnostics.",
					},
				},
			},
		},
		Execute: func(ctx context.Context, args json.RawMessage) (string, error) {
			var params map[string]interface{}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", err
			}
			res, err := m.client.GetDiagnostics(params)
			if err != nil {
				return "", err
			}
			data, _ := json.MarshalIndent(res, "", "  ")
			return string(data), nil
		},
	}
}

// GetRepoInfosTool provides a summary of the repository from Core's perspective.
func (m *CoreV2Manager) GetRepoInfosTool() Tool {
	return Tool{
		Definition: llm.ToolDefinition{
			Name:        "get_repo_metadata",
			Description: "Get high-level repository metadata and insights from the Antigravity Core indexer.",
			Parameters: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		Execute: func(ctx context.Context, args json.RawMessage) (string, error) {
			res, err := m.client.GetRepoInfos()
			if err != nil {
				return "", err
			}
			data, _ := json.MarshalIndent(res, "", "  ")
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
			if rawPath, ok := edit["filePath"].(string); ok && rawPath != "" {
				resolvedPath, err := ResolvePathWithinWorkspace(ctx, ".", rawPath)
				if err != nil {
					return "", err
				}
				edit["filePath"] = resolvedPath
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
