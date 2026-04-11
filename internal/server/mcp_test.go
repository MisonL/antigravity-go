package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/mison/antigravity-go/internal/corecap"
	"github.com/mison/antigravity-go/internal/rpc"
)

type recordedRPCCall struct {
	Method string
	Body   map[string]interface{}
}

func newRecordedRPCClient(
	t *testing.T,
	handlers map[string]func(map[string]interface{}) (int, interface{}),
) (*rpc.Client, *[]recordedRPCCall, func()) {
	t.Helper()

	calls := make([]recordedRPCCall, 0, 8)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method := strings.TrimPrefix(r.URL.Path, testRPCServicePath)
		body := map[string]interface{}{}
		if r.Body != nil {
			_ = json.NewDecoder(r.Body).Decode(&body)
		}
		calls = append(calls, recordedRPCCall{
			Method: method,
			Body:   body,
		})

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

	return rpc.NewClient(port), &calls, srv.Close
}

func TestHandleMCPGetReturnsMergedServerState(t *testing.T) {
	client, _, cleanup := newRecordedRPCClient(t, map[string]func(map[string]interface{}) (int, interface{}){
		"GetMcpServers": func(_ map[string]interface{}) (int, interface{}) {
			return http.StatusOK, map[string]interface{}{
				"servers": map[string]interface{}{
					"filesystem": map[string]interface{}{
						"command": "npx",
						"args":    []interface{}{"-y", "@modelcontextprotocol/server-filesystem"},
					},
				},
			}
		},
		"GetMcpServerStates": func(_ map[string]interface{}) (int, interface{}) {
			return http.StatusOK, map[string]interface{}{
				"states": map[string]interface{}{
					"filesystem": map[string]interface{}{
						"status": "running",
						"tools": []interface{}{
							map[string]interface{}{
								"tool_name":    "read_file",
								"description":  "Read a file from disk.",
								"input_schema": map[string]interface{}{"type": "object"},
							},
						},
					},
				},
			}
		},
	})
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/api/mcp", nil)
	resp := httptest.NewRecorder()

	srv := &Server{mcp: corecap.NewMcpManager(client)}
	srv.handleMCP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", resp.Code, resp.Body.String())
	}

	var payload struct {
		Servers []corecap.McpServerInfo `json:"servers"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(payload.Servers) != 1 {
		t.Fatalf("expected one server, got %d", len(payload.Servers))
	}
	server := payload.Servers[0]
	if server.Name != "filesystem" {
		t.Fatalf("unexpected server name: %q", server.Name)
	}
	if server.Status != "running" {
		t.Fatalf("unexpected server status: %q", server.Status)
	}
	if server.ToolCount != 1 || len(server.Tools) != 1 || server.Tools[0].Name != "read_file" {
		t.Fatalf("unexpected tool payload: %+v", server)
	}
}

func TestHandleMCPPostUpsertsServerAndSendsOverrideConfig(t *testing.T) {
	client, calls, cleanup := newRecordedRPCClient(t, map[string]func(map[string]interface{}) (int, interface{}){
		"GetMcpSetting": func(_ map[string]interface{}) (int, interface{}) {
			return http.StatusOK, map[string]interface{}{
				"override_mcp_config_json": `{"mcpServers":{}}`,
			}
		},
		"RefreshMcpServers": func(body map[string]interface{}) (int, interface{}) {
			return http.StatusOK, map[string]interface{}{
				"ok":          true,
				"server_name": body["server_name"],
			}
		},
		"GetMcpServers": func(_ map[string]interface{}) (int, interface{}) {
			return http.StatusOK, map[string]interface{}{
				"servers": map[string]interface{}{
					"filesystem": map[string]interface{}{
						"command": "npx",
						"args":    []interface{}{"-y", "@modelcontextprotocol/server-filesystem", "/tmp/workspace"},
						"env": map[string]interface{}{
							"AGO_ENV": "prod",
						},
					},
				},
			}
		},
		"GetMcpServerStates": func(_ map[string]interface{}) (int, interface{}) {
			return http.StatusOK, map[string]interface{}{
				"states": map[string]interface{}{
					"filesystem": map[string]interface{}{
						"status": "running",
						"tools": []interface{}{
							map[string]interface{}{
								"tool_name":    "read_file",
								"description":  "Read a file from disk.",
								"input_schema": map[string]interface{}{"type": "object"},
							},
						},
					},
				},
			}
		},
	})
	defer cleanup()

	req := httptest.NewRequest(http.MethodPost, "/api/mcp", strings.NewReader(`{
		"name":" filesystem ",
		"command":" npx ",
		"args":["-y","@modelcontextprotocol/server-filesystem","/tmp/workspace"],
		"env":{"AGO_ENV":"prod"}
	}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	srv := &Server{
		mcp: corecap.NewMcpManager(client),
		ws:  NewWSServer(nil, nil, nil, ".", "", ""),
	}
	srv.handleMCP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", resp.Code, resp.Body.String())
	}

	var refreshCall *recordedRPCCall
	for i := range *calls {
		call := &(*calls)[i]
		if call.Method == "RefreshMcpServers" {
			refreshCall = call
			break
		}
	}
	if refreshCall == nil {
		t.Fatal("expected RefreshMcpServers to be called")
	}

	rawConfig, ok := refreshCall.Body["override_mcp_config_json"].(string)
	if !ok || rawConfig == "" {
		t.Fatalf("missing override config in refresh request: %+v", refreshCall.Body)
	}

	var cfg struct {
		MCPServers map[string]corecap.McpServerSpec `json:"mcpServers"`
	}
	if err := json.Unmarshal([]byte(rawConfig), &cfg); err != nil {
		t.Fatalf("decode override config: %v", err)
	}

	spec, ok := cfg.MCPServers["filesystem"]
	if !ok {
		t.Fatalf("expected filesystem server in override config: %+v", cfg.MCPServers)
	}
	if spec.Command != "npx" {
		t.Fatalf("unexpected command: %+v", spec)
	}
	if len(spec.Args) != 3 {
		t.Fatalf("unexpected args: %+v", spec.Args)
	}
	if spec.Env["AGO_ENV"] != "prod" {
		t.Fatalf("unexpected env: %+v", spec.Env)
	}
}

func TestHandleMCPDeleteRemovesServerFromOverrideConfig(t *testing.T) {
	client, calls, cleanup := newRecordedRPCClient(t, map[string]func(map[string]interface{}) (int, interface{}){
		"GetMcpSetting": func(_ map[string]interface{}) (int, interface{}) {
			return http.StatusOK, map[string]interface{}{
				"override_mcp_config_json": `{"mcpServers":{"filesystem":{"command":"npx","args":["-y","@modelcontextprotocol/server-filesystem"]}}}`,
			}
		},
		"RefreshMcpServers": func(body map[string]interface{}) (int, interface{}) {
			return http.StatusOK, map[string]interface{}{
				"ok":          true,
				"server_name": body["server_name"],
			}
		},
		"GetMcpServers": func(_ map[string]interface{}) (int, interface{}) {
			return http.StatusOK, map[string]interface{}{"servers": map[string]interface{}{}}
		},
		"GetMcpServerStates": func(_ map[string]interface{}) (int, interface{}) {
			return http.StatusOK, map[string]interface{}{"states": map[string]interface{}{}}
		},
	})
	defer cleanup()

	req := httptest.NewRequest(http.MethodDelete, "/api/mcp", strings.NewReader(`{"name":"filesystem"}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	srv := &Server{mcp: corecap.NewMcpManager(client)}
	srv.handleMCP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", resp.Code, resp.Body.String())
	}

	var refreshCall *recordedRPCCall
	for i := range *calls {
		call := &(*calls)[i]
		if call.Method == "RefreshMcpServers" {
			refreshCall = call
			break
		}
	}
	if refreshCall == nil {
		t.Fatal("expected RefreshMcpServers to be called")
	}

	rawConfig, ok := refreshCall.Body["override_mcp_config_json"].(string)
	if !ok || rawConfig == "" {
		t.Fatalf("missing override config in refresh request: %+v", refreshCall.Body)
	}
	if strings.Contains(rawConfig, "filesystem") {
		t.Fatalf("expected filesystem to be removed from override config: %s", rawConfig)
	}
}

func TestHandleMCPResourcesReturnsNormalizedResources(t *testing.T) {
	client, calls, cleanup := newRecordedRPCClient(t, map[string]func(map[string]interface{}) (int, interface{}){
		"ListMcpResources": func(body map[string]interface{}) (int, interface{}) {
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
				"echo_server":     body["server_id"],
			}
		},
	})
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/api/mcp/resources?server=filesystem", nil)
	resp := httptest.NewRecorder()

	srv := &Server{mcp: corecap.NewMcpManager(client)}
	srv.handleMCPResources(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", resp.Code, resp.Body.String())
	}

	if len(*calls) == 0 || (*calls)[0].Method != "ListMcpResources" {
		t.Fatalf("expected ListMcpResources call, got %+v", calls)
	}
	if (*calls)[0].Body["server_id"] != "filesystem" {
		t.Fatalf("unexpected request body: %+v", (*calls)[0].Body)
	}

	var payload struct {
		Server        string                    `json:"server"`
		NextPageToken string                    `json:"next_page_token"`
		Resources     []corecap.McpResourceInfo `json:"resources"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Server != "filesystem" || payload.NextPageToken != "token-1" {
		t.Fatalf("unexpected payload header: %+v", payload)
	}
	if len(payload.Resources) != 1 || payload.Resources[0].URI != "file:///tmp/project/README.md" {
		t.Fatalf("unexpected resources payload: %+v", payload.Resources)
	}
}
