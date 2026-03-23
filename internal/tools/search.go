package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/mison/antigravity-go/internal/llm"
	"github.com/mison/antigravity-go/internal/pkg/pathutil"
)

func NewSearchTool(rootPath string) Tool {
	return Tool{
		Definition: llm.ToolDefinition{
			Name:        "search_files",
			Description: "Search for a text pattern in files (case-insensitive).",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "Text to search for.",
					},
					"path": map[string]interface{}{
						"type":        "string",
						"description": "Sub-directory to search in (optional, defaults to root).",
					},
				},
				"required": []string{"query"},
			},
		},
		Execute: func(ctx context.Context, args json.RawMessage) (string, error) {
			var params struct {
				Query string `json:"query"`
				Path  string `json:"path"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", err
			}

			// Path to local ripgrep
			rgPath := "./bin/rg"
			if _, err := os.Stat(rgPath); err != nil {
				rgPath = "rg" // Fallback to system rg
			}

			// Basic checks
			if params.Query == "" {
				return "Error: query is empty", nil
			}

			rootAbs, err := filepath.Abs(WorkspaceRootFromContext(ctx, rootPath))
			if err != nil {
				return "", fmt.Errorf("failed to resolve workspace root: %w", err)
			}

			searchDir := "."
			if params.Path != "" {
				safeAbs, err := pathutil.SanitizePath(rootAbs, params.Path)
				if err != nil {
					return "", err
				}
				rel, err := filepath.Rel(rootAbs, safeAbs)
				if err != nil {
					return "", err
				}
				searchDir = rel
			}

			cmd := exec.CommandContext(ctx, rgPath,
				"--no-heading",
				"--line-number",
				"--column",
				"--color=never",
				"--fixed-strings",
				"--ignore-case",
				"--glob", "!.git/**",
				"--glob", "!node_modules/**",
				"--glob", "!.gemini/**",
				"--glob", "!dist/**",
				"--glob", "!build/**",
				"--glob", "!frontend/node_modules/**",
				params.Query,
				searchDir,
			)
			cmd.Dir = rootAbs
			out, err := cmd.CombinedOutput()
			if err != nil {
				// rg: 1 代表无匹配，2 代表错误
				if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
					return "No matches found.", nil
				}
				return "", fmt.Errorf("rg failed: %w\n%s", err, string(out))
			}

			lines := strings.Split(strings.TrimSpace(string(out)), "\n")
			if len(lines) == 0 || (len(lines) == 1 && strings.TrimSpace(lines[0]) == "") {
				return "No matches found.", nil
			}

			maxResults := 50
			if len(lines) > maxResults {
				lines = append(lines[:maxResults], "... (results truncated)")
			}
			return strings.Join(lines, "\n"), nil
		},
		RequiresPermission: false, // Search is read-only safe? Actually maybe better safe than sorry, but read_file is safe?
		// read_file was safe-ish. Let's make search safe.
	}
}
