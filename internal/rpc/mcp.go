package rpc

import "fmt"

type McpRPCSupport struct {
	Add     MethodProbe `json:"add"`
	Refresh MethodProbe `json:"refresh"`
	Stop    MethodProbe `json:"stop"`
	Restart MethodProbe `json:"restart"`
	Invoke  MethodProbe `json:"invoke"`
}

func (c *Client) ProbeMcpRPCSupport() McpRPCSupport {
	return McpRPCSupport{
		Add:     c.ProbeMethod([]string{"AddMcpServer"}, map[string]interface{}{}),
		Refresh: c.ProbeMethod([]string{"RefreshMcpServers"}, map[string]interface{}{}),
		Stop:    c.ProbeMethod([]string{"StopMcpServer"}, map[string]interface{}{}),
		Restart: c.ProbeMethod([]string{"RestartMcpServer"}, map[string]interface{}{}),
		Invoke: c.ProbeMethod([]string{
			"InvokeMcpTool",
			"CallMcpTool",
			"ExecuteMcpTool",
			"CallTool",
		}, map[string]interface{}{}),
	}
}

func (c *Client) AddMcpServer(req map[string]interface{}) (map[string]interface{}, error) {
	if req == nil {
		req = map[string]interface{}{}
	}

	var resp map[string]interface{}
	if err := c.call("AddMcpServer", req, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *Client) StopMcpServer(req map[string]interface{}) (map[string]interface{}, error) {
	if req == nil {
		req = map[string]interface{}{}
	}

	var resp map[string]interface{}
	if err := c.call("StopMcpServer", req, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *Client) RestartMcpServer(req map[string]interface{}) (map[string]interface{}, error) {
	if req == nil {
		req = map[string]interface{}{}
	}

	var resp map[string]interface{}
	if err := c.call("RestartMcpServer", req, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *Client) InvokeMcpTool(req map[string]interface{}) (map[string]interface{}, error) {
	if req == nil {
		req = map[string]interface{}{}
	}

	candidates := []string{
		"InvokeMcpTool",
		"CallMcpTool",
		"ExecuteMcpTool",
		"CallTool",
	}

	var lastErr error
	for _, method := range candidates {
		var resp map[string]interface{}
		err := c.call(method, req, &resp)
		if err == nil {
			return resp, nil
		}
		if IsUnsupportedMethodError(err) {
			lastErr = err
			continue
		}
		return nil, err
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("core did not expose a known MCP tool invocation RPC")
	}
	return nil, lastErr
}
