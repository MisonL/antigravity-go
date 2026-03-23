package server

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/mison/antigravity-go/internal/corecap"
	"github.com/mison/antigravity-go/internal/tools"
)

type mcpServerRequest struct {
	Action  string            `json:"action"`
	Name    string            `json:"name"`
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env"`
}

func (s *Server) handleMCP(w http.ResponseWriter, r *http.Request) {
	if s.mcp == nil {
		http.Error(w, "mcp manager is not initialized", http.StatusServiceUnavailable)
		return
	}

	switch r.Method {
	case http.MethodGet:
		s.respondMCPState(w, "")
	case http.MethodPost, http.MethodPut, http.MethodDelete:
		var req mcpServerRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		var (
			resp map[string]interface{}
			err  error
		)

		switch {
		case r.Method == http.MethodDelete:
			resp, err = s.mcp.DeleteServer(req.Name)
		case strings.EqualFold(req.Action, "restart"):
			resp, err = s.mcp.RestartServer(req.Name)
		default:
			resp, err = s.mcp.UpsertServer(specFromRequest(req))
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}

		refreshErr := s.refreshDynamicMcpTools()
		if refreshErr != nil {
			resp["tool_refresh_error"] = refreshErr.Error()
		}

		servers, listErr := s.mcp.ListServers()
		if listErr == nil {
			resp["servers"] = servers
		}
		resp["capabilities"] = s.mcp.Capabilities()

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) respondMCPState(w http.ResponseWriter, warning string) {
	servers, err := s.mcp.ListServers()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	payload := map[string]interface{}{
		"servers":      servers,
		"capabilities": s.mcp.Capabilities(),
	}
	if warning != "" {
		payload["warning"] = warning
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(payload)
}

func (s *Server) refreshDynamicMcpTools() error {
	if s.mcp == nil {
		return nil
	}

	catalog, err := tools.BuildMcpDynamicTools(s.mcp)
	if err != nil {
		return err
	}

	if s.agent != nil {
		s.agent.ReplaceToolsByPrefix("mcp__", catalog.Tools)
	}
	if s.ws != nil {
		s.ws.ReplaceToolsByPrefix("mcp__", catalog.Tools)
		s.ws.Broadcast(map[string]interface{}{
			"type":    "mcp_tools_updated",
			"servers": catalog.Servers,
		})
	}
	return nil
}

func specFromRequest(req mcpServerRequest) corecap.McpServerSpec {
	return corecap.McpServerSpec{
		Name:    strings.TrimSpace(req.Name),
		Command: strings.TrimSpace(req.Command),
		Args:    req.Args,
		Env:     req.Env,
	}
}
