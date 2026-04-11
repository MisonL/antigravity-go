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

const testRPCServicePath = "/exa.language_server_pb.LanguageServerService/"

func newCapabilityTestClient(
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

func TestProbeCoreCapabilitiesMarksSupportedAndUnsupportedMethods(t *testing.T) {
	client, cleanup := newCapabilityTestClient(t, map[string]func(map[string]interface{}) (int, interface{}){
		"Heartbeat": func(_ map[string]interface{}) (int, interface{}) {
			return http.StatusOK, map[string]interface{}{}
		},
		"GetRepoInfos": func(_ map[string]interface{}) (int, interface{}) {
			return http.StatusOK, map[string]interface{}{"repo": "ok"}
		},
		"GetDiagnostics": func(_ map[string]interface{}) (int, interface{}) {
			return http.StatusBadRequest, "missing path"
		},
		"GetCodeFrequencyForRepo": func(_ map[string]interface{}) (int, interface{}) {
			return http.StatusBadRequest, "repo_uri is required"
		},
		"GetMcpServerStates": func(_ map[string]interface{}) (int, interface{}) {
			return http.StatusOK, map[string]interface{}{"states": map[string]interface{}{}}
		},
	})
	defer cleanup()

	caps := ProbeCoreCapabilities(client)

	if !caps.Heartbeat.Supported {
		t.Fatalf("expected heartbeat to be supported: %+v", caps.Heartbeat)
	}
	if !caps.RepoInfo.Supported {
		t.Fatalf("expected repo info to be supported: %+v", caps.RepoInfo)
	}
	if !caps.Diagnostics.Supported {
		t.Fatalf("expected diagnostics to be treated as supported on non-404 error: %+v", caps.Diagnostics)
	}
	if !caps.CodeFrequency.Supported {
		t.Fatalf("expected code frequency to be treated as supported on non-404 error: %+v", caps.CodeFrequency)
	}
	if !caps.McpStates.Supported {
		t.Fatalf("expected mcp states to be supported: %+v", caps.McpStates)
	}
	if caps.McpServers.Supported {
		t.Fatalf("expected mcp servers to be unsupported: %+v", caps.McpServers)
	}
	if caps.McpControl.Add.Supported {
		t.Fatalf("expected mcp add rpc to be unsupported: %+v", caps.McpControl.Add)
	}
}

func TestDeriveSurfaceCapabilityPolicyUsesConservativeGates(t *testing.T) {
	policy := DeriveSurfaceCapabilityPolicy(CoreCapabilities{
		TrajectoryList: rpc.MethodProbe{Supported: true},
		TrajectoryGet:  rpc.MethodProbe{Supported: false},
		Rollback:       rpc.MethodProbe{Supported: true},
		MemoryQuery:    rpc.MethodProbe{Supported: true},
		MemorySave:     rpc.MethodProbe{Supported: true},
		McpStates:      rpc.MethodProbe{Supported: true},
		McpControl: rpc.McpRPCSupport{
			Add:     rpc.MethodProbe{Supported: false},
			Refresh: rpc.MethodProbe{Supported: false},
			Restart: rpc.MethodProbe{Supported: false},
			Invoke:  rpc.MethodProbe{Supported: false},
		},
		BrowserList:       rpc.MethodProbe{Supported: true},
		BrowserScreenshot: rpc.MethodProbe{Supported: true},
		BrowserClick:      rpc.MethodProbe{Supported: false},
	})

	if !policy.Trajectory.ShowList {
		t.Fatal("expected trajectory list to be visible")
	}
	if policy.Trajectory.ShowDetail {
		t.Fatal("did not expect trajectory detail without trajectory_get support")
	}
	if policy.Trajectory.AllowResume {
		t.Fatal("did not expect trajectory resume without trajectory_get support")
	}
	if policy.Trajectory.AllowRollback {
		t.Fatal("did not expect rollback when trajectory detail is unavailable")
	}
	if !policy.Memory.ShowQuery || !policy.Memory.AllowSave {
		t.Fatalf("expected memory query/save to be available: %+v", policy.Memory)
	}
	if policy.Observability.ShowCodeFrequency {
		t.Fatalf("did not expect code frequency without capability: %+v", policy.Observability)
	}
	if !policy.MCP.Show || !policy.MCP.ReadOnly {
		t.Fatalf("expected mcp to be visible in read-only mode: %+v", policy.MCP)
	}
	if policy.MCP.AllowManage || policy.MCP.AllowInvoke {
		t.Fatalf("did not expect direct mcp controls: %+v", policy.MCP)
	}
	if !policy.Browser.ShowRead || policy.Browser.AllowInteract {
		t.Fatalf("unexpected browser policy: %+v", policy.Browser)
	}
}

func TestDeriveSurfaceCapabilityPolicyTreatsRefreshAsManageCapability(t *testing.T) {
	policy := DeriveSurfaceCapabilityPolicy(CoreCapabilities{
		McpStates: rpc.MethodProbe{Supported: true},
		McpControl: rpc.McpRPCSupport{
			Refresh: rpc.MethodProbe{Supported: true},
		},
	})

	if !policy.MCP.Show || !policy.MCP.AllowManage {
		t.Fatalf("expected MCP manage capability when refresh is supported: %+v", policy.MCP)
	}
	if policy.MCP.ReadOnly {
		t.Fatalf("did not expect read-only MCP policy when refresh is supported: %+v", policy.MCP)
	}
}

func TestDeriveSurfaceCapabilityPolicyShowsMCPWhenOnlyResourcesSupported(t *testing.T) {
	policy := DeriveSurfaceCapabilityPolicy(CoreCapabilities{
		McpResources: rpc.MethodProbe{Supported: true},
	})

	if !policy.MCP.Show || !policy.MCP.ShowResources {
		t.Fatalf("expected MCP to stay visible when resources are supported: %+v", policy.MCP)
	}
	if !policy.MCP.ReadOnly {
		t.Fatalf("expected resources-only MCP policy to be read-only: %+v", policy.MCP)
	}
}

func TestDeriveSurfaceCapabilityPolicyShowsCodeFrequencyWhenSupported(t *testing.T) {
	policy := DeriveSurfaceCapabilityPolicy(CoreCapabilities{
		CodeFrequency: rpc.MethodProbe{Supported: true},
	})

	if !policy.Observability.ShowCodeFrequency {
		t.Fatalf("expected code frequency policy to be visible: %+v", policy.Observability)
	}
}
