package tools

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/mison/antigravity-go/internal/corecap"
	"github.com/mison/antigravity-go/internal/llm"
)

var mcpNameSanitizer = regexp.MustCompile(`[^a-z0-9_]+`)

type McpToolCatalog struct {
	Servers []corecap.McpServerInfo `json:"servers"`
	Tools   []Tool                  `json:"-"`
}

func BuildMcpDynamicTools(manager *corecap.McpManager) (McpToolCatalog, error) {
	if manager == nil {
		return McpToolCatalog{}, nil
	}

	servers, err := manager.ListServers()
	if err != nil {
		return McpToolCatalog{}, err
	}

	usedNames := map[string]string{}
	builtTools := make([]Tool, 0)
	for _, server := range servers {
		for _, item := range server.Tools {
			toolName := uniqueMcpToolName(server.Name, item.Name, usedNames)
			serverName := server.Name
			remoteToolName := item.Name
			schema := item.Schema
			if len(schema) == 0 {
				schema = map[string]interface{}{"type": "object"}
			}

			description := strings.TrimSpace(item.Description)
			if description == "" {
				description = fmt.Sprintf("Invoke MCP tool %s from server %s.", remoteToolName, serverName)
			}

			builtTools = append(builtTools, Tool{
				Definition: llm.ToolDefinition{
					Name:        toolName,
					Description: description,
					Parameters:  schema,
				},
				Execute: func(ctx context.Context, args json.RawMessage) (string, error) {
					payload := map[string]interface{}{}
					if len(args) > 0 && string(args) != "null" {
						if err := json.Unmarshal(args, &payload); err != nil {
							return "", fmt.Errorf("invalid mcp tool arguments: %w", err)
						}
					}

					resp, err := manager.InvokeTool(serverName, remoteToolName, payload)
					if err != nil {
						return "", fmt.Errorf("invoke mcp tool %s/%s: %w", serverName, remoteToolName, err)
					}
					data, _ := json.MarshalIndent(resp, "", "  ")
					return string(data), nil
				},
				RequiresPermission: true,
			})
		}
	}

	return McpToolCatalog{
		Servers: servers,
		Tools:   builtTools,
	}, nil
}

func uniqueMcpToolName(serverName, toolName string, used map[string]string) string {
	base := "mcp__" + sanitizeMcpName(serverName) + "__" + sanitizeMcpName(toolName)
	key := serverName + "/" + toolName
	if existing, ok := used[base]; !ok || existing == key {
		used[base] = key
		return base
	}

	sum := sha1.Sum([]byte(key))
	unique := base + "__" + hex.EncodeToString(sum[:4])
	used[unique] = key
	return unique
}

func sanitizeMcpName(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = mcpNameSanitizer.ReplaceAllString(value, "_")
	value = strings.Trim(value, "_")
	if value == "" {
		return "unknown"
	}
	return value
}
