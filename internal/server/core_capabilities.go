package server

import (
	"net/http"

	"github.com/mison/antigravity-go/internal/corecap"
)

func (s *Server) handleCoreCapabilities(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.client == nil {
		http.Error(w, "core rpc client is not initialized", http.StatusServiceUnavailable)
		return
	}
	ready := false
	httpPort := 0
	if s.host != nil {
		ready = s.host.IsReady()
		httpPort = s.host.HTTPPort()
	}

	writeJSON(w, map[string]interface{}{
		"ready":        ready,
		"http_port":    httpPort,
		"capabilities": corecap.ProbeCoreCapabilities(s.client),
	})
}
