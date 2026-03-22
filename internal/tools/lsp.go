package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/mison/antigravity-go/internal/llm"
)

// We need an interface to access Core Host
type LSPHost interface {
	LSPPort() int
	IsReady() bool
}

type LSPManager struct {
	host     LSPHost
	rootPath string
	client   *LSPClient
	mu       sync.Mutex
	initOnce sync.Once
}

func NewLSPManager(host LSPHost, rootPath string) *LSPManager {
	return &LSPManager{
		host:     host,
		rootPath: rootPath,
	}
}

func (m *LSPManager) ensureClient() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.client != nil {
		return nil
	}

	if !m.host.IsReady() {
		return fmt.Errorf("core not ready")
	}

	client, err := NewLSPClient(m.host.LSPPort())
	if err != nil {
		return err
	}

	if err := client.Initialize(m.rootPath); err != nil {
		client.conn.Close()
		return fmt.Errorf("lsp init: %w", err)
	}

	m.client = client
	return nil
}

func (m *LSPManager) GetDefinitionTool() Tool {
	return Tool{
		Definition: llm.ToolDefinition{
			Name:        "get_definition",
			Description: "Go to definition of a symbol in a file.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"file": map[string]interface{}{
						"type":        "string",
						"description": "Absolute path of the file.",
					},
					"line": map[string]interface{}{
						"type":        "integer",
						"description": "Line number (0-indexed).",
					},
					"character": map[string]interface{}{
						"type":        "integer",
						"description": "Character offset (0-indexed).",
					},
				},
				"required": []string{"file", "line", "character"},
			},
		},
		Execute: func(ctx context.Context, args json.RawMessage) (string, error) {
			if err := m.ensureClient(); err != nil {
				return "", err
			}

			var params struct {
				File      string `json:"file"`
				Line      int    `json:"line"`
				Character int    `json:"character"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", err
			}

			lspParams := map[string]interface{}{
				"textDocument": map[string]interface{}{
					"uri": "file://" + params.File,
				},
				"position": map[string]interface{}{
					"line":      params.Line,
					"character": params.Character,
				},
			}

			res, err := m.client.Call("textDocument/definition", lspParams)
			if err != nil {
				return "", err
			}
			return string(res), nil
		},
	}
}

func (m *LSPManager) GetReferencesTool() Tool {
	return Tool{
		Definition: llm.ToolDefinition{
			Name:        "get_references",
			Description: "Find all references of a symbol in the workspace.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"file": map[string]interface{}{
						"type":        "string",
						"description": "Absolute path of the file.",
					},
					"line": map[string]interface{}{
						"type":        "integer",
						"description": "Line number (0-indexed).",
					},
					"character": map[string]interface{}{
						"type":        "integer",
						"description": "Character offset (0-indexed).",
					},
				},
				"required": []string{"file", "line", "character"},
			},
		},
		Execute: func(ctx context.Context, args json.RawMessage) (string, error) {
			if err := m.ensureClient(); err != nil {
				return "", err
			}

			var params struct {
				File      string `json:"file"`
				Line      int    `json:"line"`
				Character int    `json:"character"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", err
			}

			lspParams := map[string]interface{}{
				"textDocument": map[string]interface{}{
					"uri": "file://" + params.File,
				},
				"position": map[string]interface{}{
					"line":      params.Line,
					"character": params.Character,
				},
				"context": map[string]interface{}{
					"includeDeclaration": true,
				},
			}

			res, err := m.client.Call("textDocument/references", lspParams)
			if err != nil {
				return "", err
			}
			return string(res), nil
		},
	}
}

func (m *LSPManager) Hover(ctx context.Context, file string, line, character int) (string, error) {
	if err := m.ensureClient(); err != nil {
		return "", err
	}

	lspParams := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": "file://" + file,
		},
		"position": map[string]interface{}{
			"line":      line,
			"character": character,
		},
	}

	res, err := m.client.Call("textDocument/hover", lspParams)
	if err != nil {
		return "", err
	}
	return string(res), nil
}

func (m *LSPManager) GetHoverTool() Tool {
	return Tool{
		Definition: llm.ToolDefinition{
			Name:        "get_hover",
			Description: "Get documentation/type info for a symbol.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"file": map[string]interface{}{
						"type":        "string",
						"description": "Absolute path of the file.",
					},
					"line": map[string]interface{}{
						"type":        "integer",
						"description": "Line number (0-indexed).",
					},
					"character": map[string]interface{}{
						"type":        "integer",
						"description": "Character offset (0-indexed).",
					},
				},
				"required": []string{"file", "line", "character"},
			},
		},
		Execute: func(ctx context.Context, args json.RawMessage) (string, error) {
			var params struct {
				File      string `json:"file"`
				Line      int    `json:"line"`
				Character int    `json:"character"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", err
			}
			return m.Hover(ctx, params.File, params.Line, params.Character)
		},
	}
}

func (m *LSPManager) DocumentSymbols(ctx context.Context, file string) (string, error) {
	if err := m.ensureClient(); err != nil {
		return "", err
	}

	lspParams := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": "file://" + file,
		},
	}

	res, err := m.client.Call("textDocument/documentSymbol", lspParams)
	if err != nil {
		return "", err
	}
	return string(res), nil
}

func (m *LSPManager) GetDocumentSymbolsTool() Tool {
	return Tool{
		Definition: llm.ToolDefinition{
			Name:        "get_document_symbols",
			Description: "List all symbols (functions, classes) in a file.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"file": map[string]interface{}{
						"type":        "string",
						"description": "Absolute path of the file.",
					},
				},
				"required": []string{"file"},
			},
		},
		Execute: func(ctx context.Context, args json.RawMessage) (string, error) {
			var params struct {
				File string `json:"file"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", err
			}
			return m.DocumentSymbols(ctx, params.File)
		},
	}
}

func (m *LSPManager) GetDiagnosticsTool() Tool {
	return Tool{
		Definition: llm.ToolDefinition{
			Name:        "get_diagnostics",
			Description: "Get compilation errors and warnings for a file.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"file": map[string]interface{}{
						"type":        "string",
						"description": "Absolute path of the file.",
					},
				},
				"required": []string{"file"},
			},
		},
		Execute: func(ctx context.Context, args json.RawMessage) (string, error) {
			if err := m.ensureClient(); err != nil {
				return "", err
			}

			var params struct {
				File string `json:"file"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", err
			}

			// NOTIFICATION: textDocument/didOpen
			// Best practice: Open the file first to ensure server knows about it and computes diagnostics
			// Ideally we track open files, but for CLI agent "stateless" approach, we just open it.
			// Actually, just calling textDocument/diagnostic (Pull) logic might work if supported.
			// Or we can rely on PublishDiagnostics if we had async.
			// Let's try explicit Pull Diagnostics: workspace/diagnostic or textDocument/diagnostic

			// Trying textDocument/diagnostic (LSP 3.17)
			lspParams := map[string]interface{}{
				"textDocument": map[string]interface{}{
					"uri": "file://" + params.File,
				},
			}

			// Note: This requires gopls to support Pull Diagnostics.
			// If not supported, it might error. Standard gopls (recent versions) supports it.
			res, err := m.client.Call("textDocument/diagnostic", lspParams)
			if err != nil {
				return "", fmt.Errorf("lsp call failed (maybe pull diagnostics not supported?): %v", err)
			}
			return string(res), nil
		},
	}
}
