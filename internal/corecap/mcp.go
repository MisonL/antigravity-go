package corecap

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/mison/antigravity-go/internal/rpc"
)

var envKeyPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

type McpServerSpec struct {
	Name    string            `json:"name,omitempty"`
	Command string            `json:"command"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

type McpToolInfo struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Schema      map[string]interface{} `json:"schema,omitempty"`
}

type McpServerInfo struct {
	Name      string                 `json:"name"`
	Command   string                 `json:"command,omitempty"`
	Args      []string               `json:"args,omitempty"`
	Env       map[string]string      `json:"env,omitempty"`
	Status    string                 `json:"status,omitempty"`
	ToolCount int                    `json:"tool_count"`
	Tools     []McpToolInfo          `json:"tools,omitempty"`
	Raw       map[string]interface{} `json:"raw,omitempty"`
}

type mcpConfigFile struct {
	MCPServers map[string]McpServerSpec `json:"mcpServers"`
}

type McpManager struct {
	client *rpc.Client
}

func NewMcpManager(client *rpc.Client) *McpManager {
	return &McpManager{client: client}
}

func (m *McpManager) requireClient() error {
	if m == nil || m.client == nil {
		return fmt.Errorf("mcp manager is not initialized")
	}
	return nil
}

func (m *McpManager) Capabilities() rpc.McpRPCSupport {
	if m == nil || m.client == nil {
		return rpc.McpRPCSupport{}
	}
	return m.client.ProbeMcpRPCSupport()
}

func (m *McpManager) LoadServer(name, command string, args []string) (map[string]interface{}, error) {
	return m.UpsertServer(McpServerSpec{
		Name:    name,
		Command: command,
		Args:    args,
	})
}

func (m *McpManager) UpsertServer(spec McpServerSpec) (map[string]interface{}, error) {
	if err := m.requireClient(); err != nil {
		return nil, err
	}
	spec, err := normalizeServerSpec(spec)
	if err != nil {
		return nil, err
	}

	cfg, err := m.currentConfig()
	if err != nil {
		return nil, err
	}
	cfg.MCPServers[spec.Name] = spec

	overrideJSON, err := json.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("marshal mcp config: %w", err)
	}

	resp, err := m.client.RefreshMcpServers(map[string]interface{}{
		"server_name":              spec.Name,
		"override_mcp_config_json": string(overrideJSON),
	})
	if err != nil {
		return nil, err
	}
	resp["operation_mode"] = "override_refresh"
	resp["server_name"] = spec.Name
	resp["capabilities"] = m.Capabilities()
	return resp, nil
}

func (m *McpManager) DeleteServer(name string) (map[string]interface{}, error) {
	if err := m.requireClient(); err != nil {
		return nil, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("server name is required")
	}

	cfg, err := m.currentConfig()
	if err != nil {
		return nil, err
	}
	delete(cfg.MCPServers, name)

	overrideJSON, err := json.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("marshal mcp config: %w", err)
	}

	resp, err := m.client.RefreshMcpServers(map[string]interface{}{
		"server_name":              name,
		"override_mcp_config_json": string(overrideJSON),
	})
	if err != nil {
		return nil, err
	}
	resp["operation_mode"] = "override_refresh"
	resp["server_name"] = name
	resp["capabilities"] = m.Capabilities()
	return resp, nil
}

func (m *McpManager) RestartServer(name string) (map[string]interface{}, error) {
	if err := m.requireClient(); err != nil {
		return nil, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("server name is required")
	}

	caps := m.Capabilities()
	if caps.Restart.Supported {
		resp, err := m.client.RestartMcpServer(map[string]interface{}{
			"server_name": name,
			"name":        name,
		})
		if err == nil {
			resp["operation_mode"] = "direct_restart_rpc"
			resp["server_name"] = name
			resp["capabilities"] = caps
			return resp, nil
		}
	}

	resp, err := m.client.RefreshMcpServers(map[string]interface{}{
		"server_name": name,
	})
	if err != nil {
		return nil, err
	}
	resp["operation_mode"] = "targeted_refresh"
	resp["server_name"] = name
	resp["capabilities"] = caps
	return resp, nil
}

func (m *McpManager) InvokeTool(serverName, toolName string, args map[string]interface{}) (map[string]interface{}, error) {
	if err := m.requireClient(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(serverName) == "" {
		return nil, fmt.Errorf("server name is required")
	}
	if strings.TrimSpace(toolName) == "" {
		return nil, fmt.Errorf("tool name is required")
	}

	resp, err := m.client.InvokeMcpTool(map[string]interface{}{
		"server_name": serverName,
		"tool_name":   toolName,
		"arguments":   args,
		"args":        args,
	})
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (m *McpManager) ListServers() ([]McpServerInfo, error) {
	if err := m.requireClient(); err != nil {
		return nil, err
	}

	specPayload, err := m.client.GetMcpServers()
	if err != nil {
		return nil, err
	}
	statePayload, err := m.client.GetMcpServerStates()
	if err != nil {
		return nil, err
	}

	servers := map[string]McpServerInfo{}
	for name, raw := range extractNamedMaps(specPayload) {
		info := serverInfoFromRaw(name, raw)
		servers[name] = info
	}
	for name, raw := range extractNamedMaps(statePayload) {
		info := servers[name]
		if info.Name == "" {
			info = serverInfoFromRaw(name, raw)
		}
		if info.Status == "" {
			info.Status = pickString(raw, "status", "state", "lifecycle", "phase")
		}
		if len(info.Tools) == 0 {
			info.Tools = extractTools(raw)
			info.ToolCount = len(info.Tools)
		}
		if info.Raw == nil {
			info.Raw = raw
		}
		servers[name] = info
	}

	names := make([]string, 0, len(servers))
	for name := range servers {
		names = append(names, name)
	}
	sort.Strings(names)

	out := make([]McpServerInfo, 0, len(names))
	for _, name := range names {
		out = append(out, servers[name])
	}
	return out, nil
}

func (m *McpManager) currentConfig() (mcpConfigFile, error) {
	cfg := mcpConfigFile{MCPServers: map[string]McpServerSpec{}}

	setting, err := m.client.GetMcpSetting()
	if err == nil {
		if raw := findJSONString(setting, "override_mcp_config_json", "overrideMcpConfigJson", "mcp_config_json"); raw != "" {
			if err := json.Unmarshal([]byte(raw), &cfg); err == nil && cfg.MCPServers != nil {
				for name, spec := range cfg.MCPServers {
					spec.Name = name
					cfg.MCPServers[name] = spec
				}
				return cfg, nil
			}
		}
	}

	servers, err := m.ListServers()
	if err != nil {
		return cfg, nil
	}
	for _, server := range servers {
		cfg.MCPServers[server.Name] = McpServerSpec{
			Name:    server.Name,
			Command: server.Command,
			Args:    append([]string(nil), server.Args...),
			Env:     cloneStringMap(server.Env),
		}
	}
	return cfg, nil
}

func normalizeServerSpec(spec McpServerSpec) (McpServerSpec, error) {
	spec.Name = strings.TrimSpace(spec.Name)
	spec.Command = strings.TrimSpace(spec.Command)
	if spec.Name == "" {
		return spec, fmt.Errorf("server name is required")
	}
	if spec.Command == "" {
		return spec, fmt.Errorf("server command is required")
	}

	env := map[string]string{}
	for key, value := range spec.Env {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if !envKeyPattern.MatchString(key) {
			return spec, fmt.Errorf("invalid environment key: %s", key)
		}
		env[key] = strings.TrimSpace(value)
	}
	spec.Env = env
	return spec, nil
}

func serverInfoFromRaw(name string, raw map[string]interface{}) McpServerInfo {
	tools := extractTools(raw)
	return McpServerInfo{
		Name:      name,
		Command:   pickString(raw, "command", "cmd"),
		Args:      pickStringSlice(raw["args"]),
		Env:       pickEnv(raw["env"]),
		Status:    pickString(raw, "status", "state", "lifecycle", "phase"),
		ToolCount: len(tools),
		Tools:     tools,
		Raw:       raw,
	}
}

func extractNamedMaps(payload map[string]interface{}) map[string]map[string]interface{} {
	out := map[string]map[string]interface{}{}
	var walk func(string, interface{})
	walk = func(hint string, value interface{}) {
		switch typed := value.(type) {
		case map[string]interface{}:
			if name := firstNonEmpty(strings.TrimSpace(hint), pickString(typed, "name", "server_name", "serverName", "id", "server_id")); name != "" {
				if looksLikeServerMap(typed) {
					out[name] = mergeMaps(out[name], typed)
				}
			}
			for key, child := range typed {
				if childMap, ok := child.(map[string]interface{}); ok && looksLikeServerMap(childMap) {
					out[key] = mergeMaps(out[key], childMap)
				}
				walk(key, child)
			}
		case []interface{}:
			for _, child := range typed {
				walk(hint, child)
			}
		}
	}
	walk("", payload)
	return out
}

func looksLikeServerMap(raw map[string]interface{}) bool {
	if len(raw) == 0 {
		return false
	}
	return pickString(raw, "command", "cmd", "status", "state", "server_name", "serverName", "name") != "" ||
		raw["tools"] != nil || raw["toolStates"] != nil
}

func extractTools(value interface{}) []McpToolInfo {
	seen := map[string]bool{}
	out := []McpToolInfo{}
	var walk func(interface{})
	walk = func(v interface{}) {
		switch typed := v.(type) {
		case map[string]interface{}:
			name := pickString(typed, "tool_name", "toolName", "name")
			if name != "" && (typed["input_schema"] != nil || typed["inputSchema"] != nil || typed["description"] != nil || typed["schema"] != nil) && !seen[name] {
				seen[name] = true
				out = append(out, McpToolInfo{
					Name:        name,
					Description: pickString(typed, "description", "summary"),
					Schema:      pickSchema(typed["input_schema"], typed["inputSchema"], typed["schema"], typed["parameters"]),
				})
			}
			for _, child := range typed {
				walk(child)
			}
		case []interface{}:
			for _, child := range typed {
				walk(child)
			}
		}
	}
	walk(value)
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func pickString(raw map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if value, ok := raw[key]; ok {
			if s, ok := value.(string); ok && strings.TrimSpace(s) != "" {
				return strings.TrimSpace(s)
			}
		}
	}
	return ""
}

func pickStringSlice(value interface{}) []string {
	items, ok := value.([]interface{})
	if !ok {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
			out = append(out, strings.TrimSpace(s))
		}
	}
	return out
}

func pickEnv(value interface{}) map[string]string {
	raw, ok := value.(map[string]interface{})
	if !ok {
		return nil
	}
	out := map[string]string{}
	for key, item := range raw {
		switch typed := item.(type) {
		case string:
			out[key] = typed
		case bool, float64:
			out[key] = fmt.Sprint(typed)
		}
	}
	return out
}

func pickSchema(values ...interface{}) map[string]interface{} {
	for _, value := range values {
		if schema, ok := value.(map[string]interface{}); ok && len(schema) > 0 {
			return schema
		}
	}
	return map[string]interface{}{"type": "object"}
}

func findJSONString(payload map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if value := pickString(payload, key); value != "" {
			return value
		}
	}
	for _, child := range payload {
		if nested, ok := child.(map[string]interface{}); ok {
			if value := findJSONString(nested, keys...); value != "" {
				return value
			}
		}
	}
	return ""
}

func mergeMaps(left, right map[string]interface{}) map[string]interface{} {
	if left == nil {
		left = map[string]interface{}{}
	}
	for key, value := range right {
		left[key] = value
	}
	return left
}

func cloneStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
