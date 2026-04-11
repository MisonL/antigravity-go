package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleCoreCapabilitiesRejectsMissingClient(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/core/capabilities", nil)
	resp := httptest.NewRecorder()

	srv := &Server{}
	srv.handleCoreCapabilities(resp, req)

	if resp.Code != http.StatusServiceUnavailable {
		t.Fatalf("unexpected status: %d body=%s", resp.Code, resp.Body.String())
	}
}

func TestHandleCoreCapabilitiesReturnsProbePayload(t *testing.T) {
	client, _, cleanup := newRecordedRPCClient(t, map[string]func(map[string]interface{}) (int, interface{}){
		"Heartbeat": func(_ map[string]interface{}) (int, interface{}) {
			return http.StatusOK, map[string]interface{}{}
		},
		"GetUserMemories": func(_ map[string]interface{}) (int, interface{}) {
			return http.StatusOK, map[string]interface{}{"items": []interface{}{}}
		},
		"GetCodeFrequencyForRepo": func(_ map[string]interface{}) (int, interface{}) {
			return http.StatusBadRequest, "repo_uri is required"
		},
		"ListMcpResources": func(_ map[string]interface{}) (int, interface{}) {
			return http.StatusOK, map[string]interface{}{"resources": []interface{}{}}
		},
	})
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/api/core/capabilities", nil)
	resp := httptest.NewRecorder()

	srv := &Server{client: client}
	srv.handleCoreCapabilities(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", resp.Code, resp.Body.String())
	}

	var payload struct {
		Ready        bool `json:"ready"`
		HTTPPort     int  `json:"http_port"`
		Capabilities struct {
			Heartbeat struct {
				Supported bool `json:"supported"`
			} `json:"heartbeat"`
			MemoryQuery struct {
				Supported bool `json:"supported"`
			} `json:"memory_query"`
			CodeFrequency struct {
				Supported bool `json:"supported"`
			} `json:"code_frequency"`
			McpResources struct {
				Supported bool `json:"supported"`
			} `json:"mcp_resources"`
			TrajectoryGet struct {
				Supported bool `json:"supported"`
			} `json:"trajectory_get"`
		} `json:"capabilities"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if payload.Ready {
		t.Fatal("did not expect host ready without a running host")
	}
	if payload.HTTPPort != 0 {
		t.Fatalf("unexpected http port: %d", payload.HTTPPort)
	}
	if !payload.Capabilities.Heartbeat.Supported {
		t.Fatal("expected heartbeat probe to be supported")
	}
	if !payload.Capabilities.MemoryQuery.Supported {
		t.Fatal("expected memory query probe to be supported")
	}
	if !payload.Capabilities.CodeFrequency.Supported {
		t.Fatal("expected code_frequency probe to be supported")
	}
	if !payload.Capabilities.McpResources.Supported {
		t.Fatal("expected mcp_resources probe to be supported")
	}
	if payload.Capabilities.TrajectoryGet.Supported {
		t.Fatal("did not expect trajectory_get to be supported")
	}
}
