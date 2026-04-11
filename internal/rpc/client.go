// Package rpc provides the Connect RPC client for antigravity_core.
package rpc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// Client is the Connect RPC client for LanguageServerService.
type Client struct {
	baseURL    string
	httpClient *http.Client
	mu         sync.RWMutex
}

// NewClient creates a new RPC client.
func NewClient(port int) *Client {
	client := &Client{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
	client.SetPort(port)
	return client
}

// servicePath is the Connect RPC service path.
const servicePath = "/exa.language_server_pb.LanguageServerService"

// call makes a Connect RPC call.
func (c *Client) call(method string, req, resp interface{}) error {
	return c.callWithHTTPClient(method, req, resp, c.httpClient)
}

func (c *Client) callWithTimeout(method string, req, resp interface{}, timeout time.Duration) error {
	httpClient := c.httpClient
	if timeout > 0 {
		cloned := *c.httpClient
		cloned.Timeout = timeout
		httpClient = &cloned
	}
	return c.callWithHTTPClient(method, req, resp, httpClient)
}

func (c *Client) callWithHTTPClient(method string, req, resp interface{}, httpClient *http.Client) error {
	c.mu.RLock()
	baseURL := c.baseURL
	c.mu.RUnlock()

	url := baseURL + servicePath + "/" + method

	reqBody, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("%s: marshal request: %w", method, err)
	}

	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("%s: create request: %w", method, err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("%s: do request: %w", method, err)
	}
	defer httpResp.Body.Close()

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return fmt.Errorf("%s: read response: %w", method, err)
	}

	if httpResp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: status %d: %s", method, httpResp.StatusCode, string(body))
	}

	if resp != nil {
		if err := json.Unmarshal(body, resp); err != nil {
			return fmt.Errorf("%s: unmarshal response: %w", method, err)
		}
	}

	return nil
}

// SetPort updates the target port without replacing the client instance.
func (c *Client) SetPort(port int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.baseURL = fmt.Sprintf("http://127.0.0.1:%d", port)
}

// Port returns the currently configured HTTP port.
func (c *Client) Port() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var port int
	_, _ = fmt.Sscanf(c.baseURL, "http://127.0.0.1:%d", &port)
	return port
}

// ExperimentStatus represents an experiment entry.
type ExperimentStatus struct {
	ExperimentKey string `json:"experimentKey"`
	Enabled       bool   `json:"enabled,omitempty"`
}

// GetStaticExperimentStatusResponse is the response type.
type GetStaticExperimentStatusResponse struct {
	Experiments []ExperimentStatus `json:"experiments"`
}

// GetStaticExperimentStatus retrieves experiment configurations.
func (c *Client) GetStaticExperimentStatus() (*GetStaticExperimentStatusResponse, error) {
	var resp GetStaticExperimentStatusResponse
	if err := c.call("GetStaticExperimentStatus", map[string]interface{}{}, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetAllRules retrieves all rules.
func (c *Client) GetAllRules() (map[string]interface{}, error) {
	var resp map[string]interface{}
	if err := c.call("GetAllRules", map[string]interface{}{}, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// RecordEvent records an event.
func (c *Client) RecordEvent(event map[string]interface{}) error {
	return c.call("RecordEvent", event, nil)
}

// GetMcpServerStates retrieves the states of MCP servers.
func (c *Client) GetMcpServerStates() (map[string]interface{}, error) {
	var resp map[string]interface{}
	if err := c.call("GetMcpServerStates", map[string]interface{}{}, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// GetMcpServers retrieves MCP server specs managed by Core.
func (c *Client) GetMcpServers() (map[string]interface{}, error) {
	var resp map[string]interface{}
	if err := c.call("GetMcpServers", map[string]interface{}{}, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// RefreshMcpServers asks Core to refresh its MCP server registry.
// 可选字段（按 Core proto json tag）：
// - server_name
// - shallow
// - override_mcp_config_json
func (c *Client) RefreshMcpServers(req map[string]interface{}) (map[string]interface{}, error) {
	if req == nil {
		req = map[string]interface{}{}
	}
	var resp map[string]interface{}
	if err := c.call("RefreshMcpServers", req, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// ListMcpResources lists MCP resources for a given server.
// 典型字段（按 Core proto json tag）：
// - server_id
// - page_token
// - query
func (c *Client) ListMcpResources(req map[string]interface{}) (map[string]interface{}, error) {
	if req == nil {
		req = map[string]interface{}{}
	}
	var resp map[string]interface{}
	if err := c.call("ListMcpResources", req, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// GetMcpSetting reads Core 的 MCP 全局设置（是否启用、override config 等）。
func (c *Client) GetMcpSetting() (map[string]interface{}, error) {
	var resp map[string]interface{}
	if err := c.call("GetMcpSetting", map[string]interface{}{}, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// GetMcpEnabled reads whether MCP is enabled in Core.
func (c *Client) GetMcpEnabled() (map[string]interface{}, error) {
	var resp map[string]interface{}
	if err := c.call("GetMcpEnabled", map[string]interface{}{}, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// RecordChatFeedback records feedback for a chat response.
func (c *Client) RecordChatFeedback(feedback map[string]interface{}) error {
	return c.call("RecordChatFeedback", feedback, nil)
}

// ApplyCodeEdit applies a code edit to the workspace via Core.
func (c *Client) ApplyCodeEdit(edit map[string]interface{}) (map[string]interface{}, error) {
	var resp map[string]interface{}
	if err := c.call("ApplyCodeEdit", edit, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// GetDiagnostics retrieves lint/compile errors from the Core engine.
func (c *Client) GetDiagnostics(req map[string]interface{}) (map[string]interface{}, error) {
	if req == nil {
		req = map[string]interface{}{}
	}
	var resp map[string]interface{}
	if err := c.call("GetDiagnostics", req, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// Heartbeat sends a simple heartbeat request to the Core.
func (c *Client) Heartbeat() error {
	return c.call("Heartbeat", map[string]interface{}{}, nil)
}

// RunCommand executes a command via Core's controlled environment.
func (c *Client) RunCommand(command string, args []string, cwd string) (map[string]interface{}, error) {
	req := map[string]interface{}{
		"command": command,
		"args":    args,
		"cwd":     cwd,
	}
	var resp map[string]interface{}
	if err := c.call("RunCommand", req, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// GetRepoInfos retrieves repository-level metadata.
func (c *Client) GetRepoInfos() (map[string]interface{}, error) {
	var resp map[string]interface{}
	if err := c.call("GetRepoInfos", map[string]interface{}{}, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// AddTrackedWorkspace registers a workspace root for kernel-managed tracking.
func (c *Client) AddTrackedWorkspace(root string) (map[string]interface{}, error) {
	req := map[string]interface{}{
		"root": root,
	}

	var resp map[string]interface{}
	if err := c.call("AddTrackedWorkspace", req, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// Ping checks if the service is reachable.
func (c *Client) Ping() error {
	_, err := c.GetAllRules()
	return err
}
