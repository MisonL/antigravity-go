package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mison/antigravity-go/internal/index"
	"github.com/mison/antigravity-go/internal/llm"
)

func NewCodebaseSearchTool(idx *index.Indexer) Tool {
	return Tool{
		Definition: llm.ToolDefinition{
			Name:        "codebase_search",
			Description: "Search for symbols (functions, types) or keywords across the entire codebase using an index.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "The search query (e.g., function name, class name, or keyword).",
					},
				},
				"required": []string{"query"},
			},
		},
		Execute: func(ctx context.Context, args json.RawMessage) (string, error) {
			var params struct {
				Query string `json:"query"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", err
			}

			if idx == nil {
				return "Error: Indexer not initialized", nil
			}

			syms, files := idx.Search(params.Query)

			var sb strings.Builder
			if len(syms) > 0 {
				sb.WriteString("Matched Symbols:\n")
				for _, s := range syms {
					sb.WriteString(fmt.Sprintf("- %s (%s) in %s:%d\n", s.Name, s.Type, s.File, s.Line))
				}
				sb.WriteString("\n")
			}

			if len(files) > 0 {
				sb.WriteString("Matched Files:\n")
				for _, f := range files {
					sb.WriteString(fmt.Sprintf("- %s\n", f))
				}
			}

			if sb.Len() == 0 {
				return "No results found for query: " + params.Query, nil
			}

			return sb.String(), nil
		},
		RequiresPermission: false,
	}
}

func NewGetProjectSummaryTool(idx *index.Indexer) Tool {
	return Tool{
		Definition: llm.ToolDefinition{
			Name:        "get_project_summary",
			Description: "Get a high-level summary of the project structure and contents.",
			Parameters: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		Execute: func(ctx context.Context, args json.RawMessage) (string, error) {
			if idx == nil {
				return "Error: Indexer not initialized", nil
			}
			return idx.GetSummary(), nil
		},
		RequiresPermission: false,
	}
}
