package tools

import (
	"testing"

	"github.com/mison/antigravity-go/internal/corecap"
	"github.com/mison/antigravity-go/internal/rpc"
)

func toolNames(tools []Tool) map[string]bool {
	out := make(map[string]bool, len(tools))
	for _, tool := range tools {
		out[tool.Definition.Name] = true
	}
	return out
}

func TestAvailableToolsFiltersByCapabilities(t *testing.T) {
	manager := &CoreV2Manager{}
	caps := corecap.CoreCapabilities{
		RepoInfo:      rpc.MethodProbe{Supported: true},
		Validation:    rpc.MethodProbe{Supported: true},
		Diagnostics:   rpc.MethodProbe{Supported: false},
		BrowserList:   rpc.MethodProbe{Supported: true},
		MemorySave:    rpc.MethodProbe{Supported: false},
		TrajectoryGet: rpc.MethodProbe{Supported: true},
		Rollback:      rpc.MethodProbe{Supported: false},
		McpStates:     rpc.MethodProbe{Supported: true},
		McpResources:  rpc.MethodProbe{Supported: true},
	}

	baseNames := toolNames(manager.AvailableTools(caps, CoreToolModeBase))
	for _, required := range []string{"get_repo_metadata", "get_validation_states", "browser_list", "trajectory_get", "get_core_mcp_states", "get_core_mcp_resources"} {
		if !baseNames[required] {
			t.Fatalf("expected base tool %q to be registered, got %+v", required, baseNames)
		}
	}
	for _, forbidden := range []string{"get_core_diagnostics", "memory_save", "rollback_to_step"} {
		if baseNames[forbidden] {
			t.Fatalf("expected base tool %q to be filtered out, got %+v", forbidden, baseNames)
		}
	}

	reviewNames := toolNames(manager.AvailableTools(caps, CoreToolModeReview))
	if !reviewNames["get_repo_metadata"] || !reviewNames["get_validation_states"] || !reviewNames["get_core_mcp_states"] || !reviewNames["get_core_mcp_resources"] {
		t.Fatalf("expected review mode to keep read-only tools, got %+v", reviewNames)
	}
	if reviewNames["browser_list"] || reviewNames["trajectory_get"] {
		t.Fatalf("expected review mode to exclude non-review tools, got %+v", reviewNames)
	}
}
