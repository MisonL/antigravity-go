package tui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/mison/antigravity-go/internal/rpc"
)

func newStatusTestClient(
	t *testing.T,
	handlers map[string]func(map[string]interface{}) (int, interface{}),
) (*rpc.Client, func()) {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method := strings.TrimPrefix(r.URL.Path, "/exa.language_server_pb.LanguageServerService/")
		body := map[string]interface{}{}
		if r.Body != nil {
			_ = json.NewDecoder(r.Body).Decode(&body)
		}

		handler, ok := handlers[method]
		if !ok {
			http.Error(w, "unknown method", http.StatusNotFound)
			return
		}

		status, payload := handler(body)
		if status >= http.StatusBadRequest {
			http.Error(w, payload.(string), status)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if payload != nil {
			if err := json.NewEncoder(w).Encode(payload); err != nil {
				t.Fatalf("encode payload: %v", err)
			}
		}
	}))

	parsedURL, err := url.Parse(srv.URL)
	if err != nil {
		srv.Close()
		t.Fatalf("parse test rpc url: %v", err)
	}

	port, err := strconv.Atoi(parsedURL.Port())
	if err != nil {
		srv.Close()
		t.Fatalf("parse test rpc port: %v", err)
	}

	return rpc.NewClient(port), srv.Close
}

func TestStatusMarkdownIncludesCapabilitySummary(t *testing.T) {
	client, cleanup := newStatusTestClient(t, map[string]func(map[string]interface{}) (int, interface{}){
		"GetAllCascadeTrajectories": func(_ map[string]interface{}) (int, interface{}) {
			return http.StatusOK, map[string]interface{}{}
		},
		"GetUserMemories": func(_ map[string]interface{}) (int, interface{}) {
			return http.StatusOK, map[string]interface{}{}
		},
		"GetMcpServerStates": func(_ map[string]interface{}) (int, interface{}) {
			return http.StatusOK, map[string]interface{}{}
		},
	})
	defer cleanup()

	model := Model{client: client}
	output := model.statusMarkdown()

	for _, want := range []string{"Ready=false", "detail=false", "readonly=true", "interact=false"} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected status markdown to contain %q, got %q", want, output)
		}
	}
}

func TestStatusMarkdownHandlesMissingClient(t *testing.T) {
	model := Model{}
	output := model.statusMarkdown()
	if !strings.Contains(output, "内核能力探针") && !strings.Contains(output, "core capability probe") {
		t.Fatalf("unexpected status markdown: %q", output)
	}
}
