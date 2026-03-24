package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

func requireMethod(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method == method {
		return true
	}
	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	return false
}

func decodeJSONBody(w http.ResponseWriter, r *http.Request, dst interface{}, invalidMessage string) bool {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		http.Error(w, invalidMessage, http.StatusBadRequest)
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(payload)
}

func requireTrimmedValue(w http.ResponseWriter, raw, field string) (string, bool) {
	value := strings.TrimSpace(raw)
	if value != "" {
		return value, true
	}

	http.Error(w, fmt.Sprintf("%s is required", field), http.StatusBadRequest)
	return "", false
}

func (s *Server) hasAuthToken() bool {
	return strings.TrimSpace(s.authToken) != ""
}

func (s *Server) defaultLocale() string {
	if s.ws != nil && strings.TrimSpace(s.ws.defaultLocale) != "" {
		return s.ws.defaultLocale
	}
	return "zh-CN"
}

func (s *Server) respondPlanePayload(
	w http.ResponseWriter,
	load func() (map[string]interface{}, error),
) {
	if s.client == nil {
		writeEmptyJSONArray(w)
		return
	}

	payload, err := load()
	if err != nil {
		if isDeprecatedPlaneRPCError(err) {
			writeEmptyJSONArray(w)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, payload)
}
