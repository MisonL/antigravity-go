package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mison/antigravity-go/internal/llm"
)

func NewReadDirTool() Tool {
	return Tool{
		Definition: llm.ToolDefinition{
			Name:        "read_dir",
			Description: "List files and directories in the specified path.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "The directory path to list. Relative to current working directory.",
					},
				},
				"required": []string{"path"},
			},
		},
		Execute: func(ctx context.Context, args json.RawMessage) (string, error) {
			var params struct {
				Path string `json:"path"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", err
			}

			safePath, err := ResolvePathWithinWorkspace(ctx, ".", params.Path)
			if err != nil {
				return "", err
			}

			files, err := os.ReadDir(safePath)
			if err != nil {
				return "", fmt.Errorf("failed to read dir: %v", err)
			}

			var result string
			for _, f := range files {
				indicator := ""
				if f.IsDir() {
					indicator = "/"
				}
				result += fmt.Sprintf("%s%s\n", f.Name(), indicator)
			}
			return result, nil
		},
		RequiresPermission: false,
	}
}

func NewReadFileTool() Tool {
	return Tool{
		Definition: llm.ToolDefinition{
			Name:        "read_file",
			Description: "Read the contents of a file.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "The file path to read.",
					},
				},
				"required": []string{"path"},
			},
		},
		Execute: func(ctx context.Context, args json.RawMessage) (string, error) {
			var params struct {
				Path string `json:"path"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", err
			}

			safePath, err := ResolvePathWithinWorkspace(ctx, ".", params.Path)
			if err != nil {
				return "", err
			}

			content, err := os.ReadFile(safePath)
			if err != nil {
				return "", fmt.Errorf("failed to read file: %v", err)
			}

			// Limit large files?
			if len(content) > 10000 {
				return string(content[:10000]) + "\n... (truncated)", nil
			}

			return string(content), nil
		},
		RequiresPermission: false,
	}
}

// FileChangeKey is the context key for file modification callback
type FileChangeKey struct{}

func NewWriteFileTool() Tool {
	return Tool{
		Definition: llm.ToolDefinition{
			Name:        "write_file",
			Description: "Write content to a file. Overwrites existing files.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "The file path to write to.",
					},
					"content": map[string]interface{}{
						"type":        "string",
						"description": "The content to write.",
					},
				},
				"required": []string{"path", "content"},
			},
		},
		Execute: func(ctx context.Context, args json.RawMessage) (string, error) {
			var params struct {
				Path    string `json:"path"`
				Content string `json:"content"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", err
			}

			safePath, err := ResolvePathWithinWorkspace(ctx, ".", params.Path)
			if err != nil {
				return "", err
			}

			dir := filepath.Dir(safePath)
			if err := os.MkdirAll(dir, 0755); err != nil {
				return "", fmt.Errorf("failed to create directories: %v", err)
			}

			if err := os.WriteFile(safePath, []byte(params.Content), 0644); err != nil {
				return "", fmt.Errorf("failed to write file: %v", err)
			}

			// Trigger callback if present
			if callback, ok := ctx.Value(FileChangeKey{}).(func(string)); ok {
				callback(safePath)
			}

			return fmt.Sprintf("Successfully wrote to %s", safePath), nil
		},
		RequiresPermission: true, // Dangerous!
	}
}
