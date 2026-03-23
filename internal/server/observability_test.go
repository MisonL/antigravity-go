package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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

func TestHandleVisualSelfTestSampleIncludesTokenizedURL(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "https://dashboard.local/api/visual-self-test/sample?token=abc123", nil)
	req.Host = "dashboard.local"
	resp := httptest.NewRecorder()

	srv := &Server{}
	srv.handleVisualSelfTestSample(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", resp.Code)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if payload["url"] != "https://dashboard.local/?token=abc123" {
		t.Fatalf("unexpected url: %#v", payload["url"])
	}

	task, ok := payload["task"].(string)
	if !ok || task == "" {
		t.Fatalf("expected non-empty task, got %#v", payload["task"])
	}
	if want := `[data-testid="trajectory-modal"]`; !strings.Contains(task, want) {
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
