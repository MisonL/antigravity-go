package main

import (
	"encoding/json"
	"fmt"
)

// CompletionParams represents parameters for textDocument/completion request
type CompletionParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
	Context      *CompletionContext     `json:"context,omitempty"`
}

// CompletionContext represents the completion context
type CompletionContext struct {
	TriggerKind      int    `json:"triggerKind"`
	TriggerCharacter string `json:"triggerCharacter,omitempty"`
}

// TextDocumentIdentifier identifies a text document
type TextDocumentIdentifier struct {
	URI string `json:"uri"`
}

// Position represents a position in a text document
type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

// Completion sends a completion request
func (p *AntigravityProxy) Completion(uri string, line, character int) (*LSPResponse, error) {
	params := CompletionParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: line, Character: character},
	}

	return p.Send("textDocument/completion", params)
}

// HoverParams represents parameters for textDocument/hover request
type HoverParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
}

// Hover sends a hover request
func (p *AntigravityProxy) Hover(uri string, line, character int) (*LSPResponse, error) {
	params := HoverParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: line, Character: character},
	}

	return p.Send("textDocument/hover", params)
}

// DefinitionParams represents parameters for textDocument/definition request
type DefinitionParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
}

// Definition sends a definition request
func (p *AntigravityProxy) Definition(uri string, line, character int) (*LSPResponse, error) {
	params := DefinitionParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: line, Character: character},
	}

	return p.Send("textDocument/definition", params)
}

// ReferencesParams represents parameters for textDocument/references request
type ReferencesParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
	Context      struct {
		IncludeDeclaration bool `json:"includeDeclaration"`
	} `json:"context"`
}

// References sends a references request
func (p *AntigravityProxy) References(uri string, line, character int, includeDeclaration bool) (*LSPResponse, error) {
	params := ReferencesParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: line, Character: character},
	}
	params.Context.IncludeDeclaration = includeDeclaration

	return p.Send("textDocument/references", params)
}

// DidOpenParams represents parameters for textDocument/didOpen notification
type DidOpenParams struct {
	TextDocument TextDocumentItem `json:"textDocument"`
}

// TextDocumentItem represents a text document
type TextDocumentItem struct {
	URI        string `json:"uri"`
	LanguageID string `json:"languageId"`
	Version    int    `json:"version"`
	Text       string `json:"text"`
}

// DidOpen sends a didOpen notification
func (p *AntigravityProxy) DidOpen(uri, languageID, text string) error {
	params := DidOpenParams{
		TextDocument: TextDocumentItem{
			URI:        uri,
			LanguageID: languageID,
			Version:    1,
			Text:       text,
		},
	}

	// Notifications don't have responses, so we send directly
	request := LSPRequest{
		Jsonrpc: "2.0",
		Method:  "textDocument/didOpen",
		Params:  params,
	}

	requestJSON, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	if _, err := fmt.Fprintln(p.stdin, string(requestJSON)); err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}

	return nil
}

// DidChangeParams represents parameters for textDocument/didChange notification
type DidChangeParams struct {
	TextDocument   VersionedTextDocumentIdentifier `json:"textDocument"`
	ContentChanges []TextDocumentContentChangeEvent `json:"contentChanges"`
}

// VersionedTextDocumentIdentifier identifies a versioned text document
type VersionedTextDocumentIdentifier struct {
	URI     string `json:"uri"`
	Version int    `json:"version"`
}

// TextDocumentContentChangeEvent represents a content change event
type TextDocumentContentChangeEvent struct {
	Range       *Range `json:"range,omitempty"`
	RangeLength int    `json:"rangeLength,omitempty"`
	Text        string `json:"text"`
}

// Range represents a range in a text document
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

// DidChange sends a didChange notification
func (p *AntigravityProxy) DidChange(uri string, version int, text string) error {
	params := DidChangeParams{
		TextDocument: VersionedTextDocumentIdentifier{
			URI:     uri,
			Version: version,
		},
		ContentChanges: []TextDocumentContentChangeEvent{
			{Text: text},
		},
	}

	request := LSPRequest{
		Jsonrpc: "2.0",
		Method:  "textDocument/didChange",
		Params:  params,
	}

	requestJSON, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	if _, err := fmt.Fprintln(p.stdin, string(requestJSON)); err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}

	return nil
}

// DidCloseParams represents parameters for textDocument/didClose notification
type DidCloseParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
}

// DidClose sends a didClose notification
func (p *AntigravityProxy) DidClose(uri string) error {
	params := DidCloseParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
	}

	request := LSPRequest{
		Jsonrpc: "2.0",
		Method:  "textDocument/didClose",
		Params:  params,
	}

	requestJSON, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	if _, err := fmt.Fprintln(p.stdin, string(requestJSON)); err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}

	return nil
}