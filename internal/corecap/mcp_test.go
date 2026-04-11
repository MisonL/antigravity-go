package corecap

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

func newMcpTestClient(
	t *testing.T,
	handlers map[string]func(map[string]interface{}) (int, interface{}),
) (*rpc.Client, func()) {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method := strings.TrimPrefix(r.URL.Path, testRPCServicePath)
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

func TestListServersFallsBackToStatePayloadWhenSpecsUnsupported(t *testing.T) {
	client, cleanup := newMcpTestClient(t, map[string]func(map[string]interface{}) (int, interface{}){
		"GetMcpServers": func(_ map[string]interface{}) (int, interface{}) {
			return http.StatusNotFound, "unknown method"
		},
		"GetMcpServerStates": func(_ map[string]interface{}) (int, interface{}) {
			return http.StatusOK, map[string]interface{}{
				"states": map[string]interface{}{
					"filesystem": map[string]interface{}{
						"status": "running",
						"tools": []interface{}{
							map[string]interface{}{
								"tool_name":    "read_file",
								"description":  "Read file",
								"input_schema": map[string]interface{}{"type": "object"},
							},
						},
					},
				},
			}
		},
	})
	defer cleanup()

	manager := NewMcpManager(client)
	servers, err := manager.ListServers()
	if err != nil {
		t.Fatalf("ListServers returned error: %v", err)
	}
	if len(servers) != 1 {
		t.Fatalf("expected one server, got %d", len(servers))
	}
	if servers[0].Name != "filesystem" || servers[0].Status != "running" {
		t.Fatalf("unexpected server payload: %+v", servers[0])
	}
	if servers[0].ToolCount != 1 || len(servers[0].Tools) != 1 || servers[0].Tools[0].Name != "read_file" {
		t.Fatalf("unexpected tools payload: %+v", servers[0])
	}
}

func TestListResourcesExtractsNormalizedEntries(t *testing.T) {
	client, cleanup := newMcpTestClient(t, map[string]func(map[string]interface{}) (int, interface{}){
		"ListMcpResources": func(body map[string]interface{}) (int, interface{}) {
			if body["server_id"] != "filesystem" {
				t.Fatalf("unexpected server_id: %+v", body)
			}
			return http.StatusOK, map[string]interface{}{
				"resources": []interface{}{
					map[string]interface{}{
						"uri":         "file:///tmp/project/README.md",
						"name":        "README",
						"description": "workspace readme",
						"mime_type":   "text/markdown",
					},
				},
				"next_page_token": "token-1",
			}
		},
	})
	defer cleanup()

	manager := NewMcpManager(client)
	resources, nextPageToken, err := manager.ListResources("filesystem", "", "")
	if err != nil {
		t.Fatalf("ListResources returned error: %v", err)
	}
	if nextPageToken != "token-1" {
		t.Fatalf("unexpected next page token: %q", nextPageToken)
	}
	if len(resources) != 1 {
		t.Fatalf("expected one resource, got %d", len(resources))
	}
	if resources[0].URI != "file:///tmp/project/README.md" || resources[0].Name != "README" {
		t.Fatalf("unexpected resource payload: %+v", resources[0])
	}
}
