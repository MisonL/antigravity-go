package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"time"
)

// LSPRequest represents a JSON-RPC request for LSP
type LSPRequest struct {
	Jsonrpc string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// LSPResponse represents a JSON-RPC response
type LSPResponse struct {
	Jsonrpc string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *LSPError   `json:"error,omitempty"`
}

// LSPError represents an LSP error
type LSPError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// AntigravityProxy manages the connection to Antigravity Core
type AntigravityProxy struct {
	cmd      *exec.Cmd
	stdin    io.WriteCloser
	stdout   io.Reader
	stderr   io.Reader
	requests chan LSPRequest
	responses map[interface{}]chan LSPResponse
	mu       sync.RWMutex
	nextID   int
}

// NewAntigravityProxy creates a new proxy instance
func NewAntigravityProxy() *AntigravityProxy {
	return &AntigravityProxy{
		requests:  make(chan LSPRequest, 100),
		responses: make(map[interface{}]chan LSPResponse),
	}
}

// Start launches the Antigravity Core process
func (p *AntigravityProxy) Start() error {
	// Path to the Antigravity Core binary (in parent directory)
	corePath := "../antigravity_core"

	// Start the language server with LSP enabled
	p.cmd = exec.Command(corePath,
		"--enable_lsp",
		"--app_data_dir", "test_data",
		"--cloud_code_endpoint", "https://daily-cloudcode-pa.googleapis.com",
		"--api_server_url", "http://127.0.0.1:50001",
		"--extension_server_port", "0", // 0 means don't use extension server
	)

	var err error
	p.stdin, err = p.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	p.stdout, err = p.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	p.stderr, err = p.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := p.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start antigravity core: %w", err)
	}

	// Start reading responses
	go p.readResponses()

	log.Println("Antigravity Core started successfully")
	return nil
}

// readResponses continuously reads responses from the language server
func (p *AntigravityProxy) readResponses() {
	scanner := bufio.NewScanner(p.stdout)
	for scanner.Scan() {
		var response LSPResponse
		if err := json.Unmarshal(scanner.Bytes(), &response); err != nil {
			log.Printf("Failed to parse response: %v", err)
			continue
		}

		p.mu.RLock()
		if ch, ok := p.responses[response.ID]; ok {
			ch <- response
		}
		p.mu.RUnlock()
	}

	if err := scanner.Err(); err != nil {
		log.Printf("Error reading stdout: %v", err)
	}
}

// Send sends a request to the language server and waits for response
func (p *AntigravityProxy) Send(method string, params interface{}) (*LSPResponse, error) {
	p.mu.Lock()
	id := p.nextID
	p.nextID++
	p.mu.Unlock()

	request := LSPRequest{
		Jsonrpc: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	// Create response channel
	responseCh := make(chan LSPResponse, 1)
	p.mu.Lock()
	p.responses[id] = responseCh
	p.mu.Unlock()

	// Send request
	requestJSON, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	if _, err := fmt.Fprintln(p.stdin, string(requestJSON)); err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	// Wait for response with timeout
	select {
	case response := <-responseCh:
		p.mu.Lock()
		delete(p.responses, id)
		p.mu.Unlock()
		log.Printf("Received response for %s: %+v", method, response)
		return &response, nil
	case <-time.After(30 * time.Second):
		p.mu.Lock()
		delete(p.responses, id)
		p.mu.Unlock()
		return nil, fmt.Errorf("timeout waiting for response from %s", method)
	}
}

// Initialize sends the LSP initialize request
func (p *AntigravityProxy) Initialize() (*LSPResponse, error) {
	params := map[string]interface{}{
		"processId": os.Getpid(),
		"rootUri":   "file:///Volumes/Work/code/antigravity-go",
		"capabilities": map[string]interface{}{
			"textDocument": map[string]interface{}{
				"completion": map[string]interface{}{
					"completionItem": map[string]interface{}{
						"snippetSupport": true,
					},
				},
				"hover": map[string]interface{}{
					"contentFormat": []string{"markdown", "plaintext"},
				},
			},
		},
	}

	return p.Send("initialize", params)
}

// Shutdown sends the LSP shutdown request
func (p *AntigravityProxy) Shutdown() (*LSPResponse, error) {
	return p.Send("shutdown", nil)
}

// Exit sends the LSP exit request
func (p *AntigravityProxy) Exit() (*LSPResponse, error) {
	return p.Send("exit", nil)
}

// Stop terminates the language server process
func (p *AntigravityProxy) Stop() error {
	if p.cmd != nil && p.cmd.Process != nil {
		return p.cmd.Process.Kill()
	}
	return nil
}

// HTTPHandler wraps the proxy for HTTP access
type HTTPHandler struct {
	proxy *AntigravityProxy
}

// NewHTTPHandler creates a new HTTP handler
func NewHTTPHandler(proxy *AntigravityProxy) *HTTPHandler {
	return &HTTPHandler{proxy: proxy}
}

// ServeHTTP handles HTTP requests
func (h *HTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.URL.Path {
	case "/initialize":
		response, err := h.proxy.Initialize()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(response)

	case "/shutdown":
		response, err := h.proxy.Shutdown()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(response)

	case "/exit":
		response, err := h.proxy.Exit()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(response)

	case "/completion":
		// Parse request body
		var req struct {
			URI       string `json:"uri"`
			Line      int    `json:"line"`
			Character int    `json:"character"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		response, err := h.proxy.Completion(req.URI, req.Line, req.Character)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(response)

	case "/hover":
		var req struct {
			URI       string `json:"uri"`
			Line      int    `json:"line"`
			Character int    `json:"character"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		response, err := h.proxy.Hover(req.URI, req.Line, req.Character)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(response)

	case "/definition":
		var req struct {
			URI       string `json:"uri"`
			Line      int    `json:"line"`
			Character int    `json:"character"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		response, err := h.proxy.Definition(req.URI, req.Line, req.Character)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(response)

	case "/references":
		var req struct {
			URI                 string `json:"uri"`
			Line                int    `json:"line"`
			Character           int    `json:"character"`
			IncludeDeclaration  bool   `json:"includeDeclaration"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		response, err := h.proxy.References(req.URI, req.Line, req.Character, req.IncludeDeclaration)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(response)

	case "/didOpen":
		var req struct {
			URI        string `json:"uri"`
			LanguageID string `json:"languageId"`
			Text       string `json:"text"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if err := h.proxy.DidOpen(req.URI, req.LanguageID, req.Text); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})

	case "/didChange":
		var req struct {
			URI     string `json:"uri"`
			Version int    `json:"version"`
			Text    string `json:"text"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if err := h.proxy.DidChange(req.URI, req.Version, req.Text); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})

	case "/didClose":
		var req struct {
			URI string `json:"uri"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if err := h.proxy.DidClose(req.URI); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})

	default:
		http.Error(w, "Not found", http.StatusNotFound)
	}
}

func main() {
	proxy := NewAntigravityProxy()

	// Start the Antigravity Core
	if err := proxy.Start(); err != nil {
		log.Fatalf("Failed to start proxy: %v", err)
	}
	defer proxy.Stop()

	// Wait a moment for the server to be ready
	// In production, you should wait for the "initialized" notification

	// Start HTTP server
	handler := NewHTTPHandler(proxy)
	http.ListenAndServe(":8080", handler)
}