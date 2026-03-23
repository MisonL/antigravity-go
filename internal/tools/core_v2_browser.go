package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mison/antigravity-go/internal/llm"
)

type browserPageParams struct {
	PageID string `json:"page_id"`
}

type browserOpenParams struct {
	URL string `json:"url"`
}

type browserClickParams struct {
	PageID   string `json:"page_id"`
	Selector string `json:"selector"`
}

type browserTypeParams struct {
	PageID   string `json:"page_id"`
	Selector string `json:"selector"`
	Text     string `json:"text"`
}

type browserScrollParams struct {
	PageID string `json:"page_id"`
	DeltaX int    `json:"delta_x"`
	DeltaY int    `json:"delta_y"`
}

// BrowserOpenTool allows the agent to open a specific URL in the managed browser.
func (m *CoreV2Manager) BrowserOpenTool() Tool {
	return Tool{
		Definition: llm.ToolDefinition{
			Name:        "browser_open",
			Description: "Open a URL in the Antigravity Core managed browser. Use this to read documentation or test web applications.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"url": map[string]interface{}{
						"type":        "string",
						"description": "The URL to open (e.g., http://localhost:3000 or https://docs.example.com).",
					},
				},
				"required": []string{"url"},
			},
		},
		Execute: func(ctx context.Context, args json.RawMessage) (string, error) {
			var params browserOpenParams
			if err := json.Unmarshal(args, &params); err != nil {
				return "", err
			}
			res, err := m.browser.Open(params.URL)
			if err != nil {
				return "", fmt.Errorf("failed to open browser page: %w", err)
			}
			data, _ := json.Marshal(res)
			return string(data), nil
		},
	}
}

// BrowserListTool lists active browser pages and their IDs.
func (m *CoreV2Manager) BrowserListTool() Tool {
	return Tool{
		Definition: llm.ToolDefinition{
			Name:        "browser_list",
			Description: "List all active browser pages and their IDs. Helpful to find which page to screenshot or focus.",
			Parameters: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		Execute: func(ctx context.Context, args json.RawMessage) (string, error) {
			res, err := m.browser.List()
			if err != nil {
				return "", fmt.Errorf("failed to list browser pages: %w", err)
			}
			data, _ := json.MarshalIndent(res, "", "  ")
			return string(data), nil
		},
	}
}

// CaptureScreenshotTool captures a browser screenshot via Core.
func (m *CoreV2Manager) CaptureScreenshotTool() Tool {
	return Tool{
		Definition: llm.ToolDefinition{
			Name:        "browser_screenshot",
			Description: "Capture a screenshot of a browser page managed by Antigravity Core. Helpful for debugging frontend UIs.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"page_id": map[string]interface{}{
						"type":        "string",
						"description": "The ID of the browser page to capture.",
					},
				},
				"required": []string{"page_id"},
			},
		},
		Execute: func(ctx context.Context, args json.RawMessage) (string, error) {
			var params browserPageParams
			if err := json.Unmarshal(args, &params); err != nil {
				return "", err
			}
			res, err := m.browser.Screenshot(params.PageID)
			if err != nil {
				return "", fmt.Errorf("failed to capture screenshot: %w", err)
			}
			data, _ := json.Marshal(res)
			return string(data), nil
		},
	}
}

// BrowserFocusTool focuses an existing browser page.
func (m *CoreV2Manager) BrowserFocusTool() Tool {
	return Tool{
		Definition: llm.ToolDefinition{
			Name:        "browser_focus",
			Description: "Focus an existing browser page managed by Antigravity Core.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"page_id": map[string]interface{}{
						"type":        "string",
						"description": "The browser page ID returned by browser_list.",
					},
				},
				"required": []string{"page_id"},
			},
		},
		Execute: func(ctx context.Context, args json.RawMessage) (string, error) {
			var params browserPageParams
			if err := json.Unmarshal(args, &params); err != nil {
				return "", err
			}
			res, err := m.browser.Focus(params.PageID)
			if err != nil {
				return "", fmt.Errorf("failed to focus browser page: %w", err)
			}
			data, _ := json.MarshalIndent(res, "", "  ")
			return string(data), nil
		},
	}
}

// BrowserClickTool clicks an element on a managed browser page.
func (m *CoreV2Manager) BrowserClickTool() Tool {
	return Tool{
		Definition: llm.ToolDefinition{
			Name:        "browser_click",
			Description: "Click the first element matching selector on a managed browser page.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"page_id": map[string]interface{}{
						"type":        "string",
						"description": "The browser page ID returned by browser_list.",
					},
					"selector": map[string]interface{}{
						"type":        "string",
						"description": "A selector used by Core to find the target element.",
					},
				},
				"required": []string{"page_id", "selector"},
			},
		},
		Execute: func(ctx context.Context, args json.RawMessage) (string, error) {
			var params browserClickParams
			if err := json.Unmarshal(args, &params); err != nil {
				return "", err
			}
			res, err := m.browser.Click(params.PageID, params.Selector)
			if err != nil {
				return "", fmt.Errorf("failed to click browser element: %w", err)
			}
			data, _ := json.MarshalIndent(res, "", "  ")
			return string(data), nil
		},
	}
}

// BrowserTypeTool types text into an element on a managed browser page.
func (m *CoreV2Manager) BrowserTypeTool() Tool {
	return Tool{
		Definition: llm.ToolDefinition{
			Name:        "browser_type",
			Description: "Type text into the first element matching selector on a managed browser page.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"page_id": map[string]interface{}{
						"type":        "string",
						"description": "The browser page ID returned by browser_list.",
					},
					"selector": map[string]interface{}{
						"type":        "string",
						"description": "A selector used by Core to find the target input element.",
					},
					"text": map[string]interface{}{
						"type":        "string",
						"description": "The text to enter into the target element.",
					},
				},
				"required": []string{"page_id", "selector", "text"},
			},
		},
		Execute: func(ctx context.Context, args json.RawMessage) (string, error) {
			var params browserTypeParams
			if err := json.Unmarshal(args, &params); err != nil {
				return "", err
			}
			res, err := m.browser.Type(params.PageID, params.Selector, params.Text)
			if err != nil {
				return "", fmt.Errorf("failed to type into browser element: %w", err)
			}
			data, _ := json.MarshalIndent(res, "", "  ")
			return string(data), nil
		},
	}
}

// BrowserScrollTool scrolls a managed browser page by the provided delta.
func (m *CoreV2Manager) BrowserScrollTool() Tool {
	return Tool{
		Definition: llm.ToolDefinition{
			Name:        "browser_scroll",
			Description: "Scroll a managed browser page by the provided horizontal and vertical delta.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"page_id": map[string]interface{}{
						"type":        "string",
						"description": "The browser page ID returned by browser_list.",
					},
					"delta_x": map[string]interface{}{
						"type":        "integer",
						"description": "Horizontal scroll delta.",
					},
					"delta_y": map[string]interface{}{
						"type":        "integer",
						"description": "Vertical scroll delta.",
					},
				},
				"required": []string{"page_id", "delta_x", "delta_y"},
			},
		},
		Execute: func(ctx context.Context, args json.RawMessage) (string, error) {
			var params browserScrollParams
			if err := json.Unmarshal(args, &params); err != nil {
				return "", err
			}
			res, err := m.browser.Scroll(params.PageID, params.DeltaX, params.DeltaY)
			if err != nil {
				return "", fmt.Errorf("failed to scroll browser page: %w", err)
			}
			data, _ := json.MarshalIndent(res, "", "  ")
			return string(data), nil
		},
	}
}
