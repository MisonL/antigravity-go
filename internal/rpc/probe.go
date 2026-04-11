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
	Evidence  string `json:"evidence,omitempty"`
}

func StatusCodeFromError(err error) int {
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

func IsUnsupportedMethodError(err error) bool {
	if err == nil {
		return false
	}

	msg := strings.ToLower(err.Error())
	code := StatusCodeFromError(err)
	if code == 404 || code == 405 || code == 501 {
		return true
	}

	return strings.Contains(msg, "unimplemented") ||
		strings.Contains(msg, "unknown method") ||
		strings.Contains(msg, "not found")
}

func (c *Client) ProbeMethod(candidates []string, req map[string]interface{}) MethodProbe {
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

		if IsUnsupportedMethodError(err) {
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
