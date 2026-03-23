package server

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

const testRPCServicePath = "/exa.language_server_pb.LanguageServerService/"

type rpcTestResponse struct {
	status int
	body   string
}

func newTestRPCClient(t *testing.T, responses map[string]rpcTestResponse) (*rpc.Client, func()) {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method := strings.TrimPrefix(r.URL.Path, testRPCServicePath)
		resp, ok := responses[method]
		if !ok {
			resp = rpcTestResponse{
				status: http.StatusOK,
				body:   `{}`,
			}
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(resp.status)
		_, _ = w.Write([]byte(resp.body))
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

func TestHandleObservabilitySummaryReturnsEmptyOnDeprecatedRPC(t *testing.T) {
	client, cleanup := newTestRPCClient(t, map[string]rpcTestResponse{
		"GetAllCascadeTrajectories": {
			status: http.StatusInternalServerError,
			body:   "deprecated",
		},
	})
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/api/observability/summary", nil)
	resp := httptest.NewRecorder()

	srv := &Server{client: client}
	srv.handleObservabilitySummary(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", resp.Code, resp.Body.String())
	}

	var payload observabilitySummary
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode summary: %v", err)
	}

	if payload.Trajectories.Count != 0 {
		t.Fatalf("unexpected trajectory count: %d", payload.Trajectories.Count)
	}
	if payload.Memories.Count != 0 {
		t.Fatalf("unexpected memory count: %d", payload.Memories.Count)
	}
}

func TestHandleTrajectoriesReturnsEmptyArrayOnDeprecatedRPC(t *testing.T) {
	client, cleanup := newTestRPCClient(t, map[string]rpcTestResponse{
		"GetAllCascadeTrajectories": {
			status: http.StatusInternalServerError,
			body:   "unknown: deprecated",
		},
	})
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/api/trajectories", nil)
	resp := httptest.NewRecorder()

	srv := &Server{client: client}
	srv.handleTrajectories(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", resp.Code, resp.Body.String())
	}
	if strings.TrimSpace(resp.Body.String()) != "[]" {
		t.Fatalf("unexpected response body: %q", resp.Body.String())
	}
}

func TestHandleMemoriesReturnsEmptyArrayOnDeprecatedRPC(t *testing.T) {
	client, cleanup := newTestRPCClient(t, map[string]rpcTestResponse{
		"GetUserMemories": {
			status: http.StatusInternalServerError,
			body:   "unimplemented",
		},
	})
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/api/memories", nil)
	resp := httptest.NewRecorder()

	srv := &Server{client: client}
	srv.handleMemories(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", resp.Code, resp.Body.String())
	}
	if strings.TrimSpace(resp.Body.String()) != "[]" {
		t.Fatalf("unexpected response body: %q", resp.Body.String())
	}
}
