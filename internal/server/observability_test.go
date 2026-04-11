package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mison/antigravity-go/internal/corecap"
)

func TestSummarizePlaneCollectionHandlesNestedPayloads(t *testing.T) {
	payload := map[string]interface{}{
		"items": []interface{}{
			map[string]interface{}{
				"trajectory_id": "traj-1",
				"updated_at":    "2026-03-23T10:00:00Z",
			},
			map[string]interface{}{
				"trajectory_id": "traj-2",
				"updated_at":    "2026-03-23T11:00:00Z",
			},
		},
	}

	snapshot := summarizePlaneCollection(
		payload,
		[]string{"items"},
		[]string{"id", "trajectory_id"},
		[]string{"updated_at"},
	)

	if snapshot.Count != 2 {
		t.Fatalf("unexpected count: %d", snapshot.Count)
	}
	if snapshot.LatestID != "traj-1" {
		t.Fatalf("unexpected latest id: %q", snapshot.LatestID)
	}
	if snapshot.LatestUpdatedAt != "2026-03-23T10:00:00Z" {
		t.Fatalf("unexpected latest time: %q", snapshot.LatestUpdatedAt)
	}
}

func TestHandleVisualSelfTestSampleReturnsDashboardURLWithoutToken(t *testing.T) {
	client, _, cleanup := newRecordedRPCClient(t, map[string]func(map[string]interface{}) (int, interface{}){
		"GetAllCascadeTrajectories": func(_ map[string]interface{}) (int, interface{}) {
			return http.StatusOK, map[string]interface{}{"items": []interface{}{}}
		},
		"GetUserMemories": func(_ map[string]interface{}) (int, interface{}) {
			return http.StatusOK, map[string]interface{}{"items": []interface{}{}}
		},
		"GetMcpServerStates": func(_ map[string]interface{}) (int, interface{}) {
			return http.StatusOK, map[string]interface{}{"states": map[string]interface{}{}}
		},
	})
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "https://dashboard.local/api/visual-self-test/sample?token=abc123", nil)
	req.Host = "dashboard.local"
	resp := httptest.NewRecorder()

	srv := &Server{client: client}
	srv.handleVisualSelfTestSample(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", resp.Code)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if payload["url"] != "https://dashboard.local/" {
		t.Fatalf("unexpected url: %#v", payload["url"])
	}

	task, ok := payload["task"].(string)
	if !ok || task == "" {
		t.Fatalf("expected non-empty task, got %#v", payload["task"])
	}
	if want := `[data-testid="trajectory-modal"]`; !strings.Contains(task, want) {
		t.Fatalf("expected task to include %s, got %q", want, task)
	}
	if strings.Contains(task, `[data-testid="trajectory-detail"]`) {
		t.Fatalf("did not expect unsupported trajectory detail selector in task: %q", task)
	}
	if want := `[data-testid="open-mcp"]`; !strings.Contains(task, want) {
		t.Fatalf("expected task to include %s, got %q", want, task)
	}
}

func TestHandleRollbackStepRejectsMissingStepID(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/rollback", strings.NewReader(`{"step_id":""}`))
	resp := httptest.NewRecorder()

	srv := &Server{}
	srv.handleRollbackStep(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("unexpected status: %d", resp.Code)
	}
}

func TestHandleObservabilityCodeFrequencyReturnsBuckets(t *testing.T) {
	workspaceRoot := t.TempDir()
	client, calls, cleanup := newRecordedRPCClient(t, map[string]func(map[string]interface{}) (int, interface{}){
		"GetCodeFrequencyForRepo": func(body map[string]interface{}) (int, interface{}) {
			return http.StatusOK, map[string]interface{}{
				"codeFrequency": []interface{}{
					map[string]interface{}{
						"numCommits":      3,
						"linesAdded":      42,
						"linesDeleted":    5,
						"recordStartTime": "2026-04-10T10:00:00Z",
						"recordEndTime":   "2026-04-10T11:00:00Z",
					},
				},
			}
		},
	})
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/api/observability/code-frequency", nil)
	resp := httptest.NewRecorder()

	srv := &Server{
		client:        client,
		workspaceRoot: workspaceRoot,
	}
	srv.handleObservabilityCodeFrequency(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", resp.Code, resp.Body.String())
	}

	if len(*calls) == 0 || (*calls)[0].Method != "GetCodeFrequencyForRepo" {
		t.Fatalf("expected GetCodeFrequencyForRepo call, got %+v", calls)
	}

	expectedURI := "file://" + filepath.ToSlash(workspaceRoot)
	if filepath.VolumeName(workspaceRoot) != "" {
		expectedURI = "file:///" + filepath.ToSlash(workspaceRoot)
	}
	if (*calls)[0].Body["repo_uri"] != expectedURI {
		t.Fatalf("unexpected repo_uri: %#v", (*calls)[0].Body["repo_uri"])
	}

	var payload struct {
		WorkspaceRoot string                        `json:"workspace_root"`
		CodeFrequency []corecap.CodeFrequencyBucket `json:"code_frequency"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if payload.WorkspaceRoot != workspaceRoot {
		t.Fatalf("unexpected workspace_root: %q", payload.WorkspaceRoot)
	}
	if len(payload.CodeFrequency) != 1 || payload.CodeFrequency[0].LinesAdded != 42 {
		t.Fatalf("unexpected code frequency payload: %+v", payload.CodeFrequency)
	}
}
