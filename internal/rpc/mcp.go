package rpc

import (
	"fmt"
	"regexp"
	"strings"
)

var statusCodePattern = regexp.MustCompile(`status (\d+):`)

type MethodProbe struct {
	Requested string `json:"requested"`
	Supported bool   `json:"supported"`
	Evidence  string `json:"evidence"`
}

type McpRPCSupport struct {
	Add     MethodProbe `json:"add"`
	Stop    MethodProbe `json:"stop"`
	Restart MethodProbe `json:"restart"`
	Invoke  MethodProbe `json:"invoke"`
}

func statusCodeFromError(err error) int {
	if err == nil {
		return 0
	}

	matches := statusCodePattern.FindStringSubmatch(err.Error())
	if len(matches) != 2 {
		return 0
	}

	code := 0
	fmt.Sscanf(matches[1], "%d", &code)
	return code
}

func isUnsupportedMethodError(err error) bool {
	if err == nil {
		return false
	}

	msg := strings.ToLower(err.Error())
	code := statusCodeFromError(err)
	if code == 404 || code == 405 || code == 501 {
		return true
	}

	return strings.Contains(msg, "unimplemented") ||
		strings.Contains(msg, "unknown method") ||
		strings.Contains(msg, "not found")
}

func (c *Client) probeMethod(candidates []string, req map[string]interface{}) MethodProbe {
	if req == nil {
		req = map[string]interface{}{}
	}

	for _, method := range candidates {
		err := c.call(method, req, nil)
		if err == nil {
			return MethodProbe{
				Requested: method,
				Supported: true,
			}
		}

		if isUnsupportedMethodError(err) {
			continue
		}

		return MethodProbe{
			Requested: method,
			Supported: true,
			Evidence:  err.Error(),
		}
	}

	if len(candidates) == 0 {
		return MethodProbe{}
	}

	return MethodProbe{
		Requested: candidates[0],
		Supported: false,
		Evidence:  "no candidate method responded as supported",
	}
}

func (c *Client) ProbeMcpRPCSupport() McpRPCSupport {
	return McpRPCSupport{
		Add:     c.probeMethod([]string{"AddMcpServer"}, map[string]interface{}{}),
		Stop:    c.probeMethod([]string{"StopMcpServer"}, map[string]interface{}{}),
		Restart: c.probeMethod([]string{"RestartMcpServer"}, map[string]interface{}{}),
		Invoke: c.probeMethod([]string{
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
		if isUnsupportedMethodError(err) {
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
