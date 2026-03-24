package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/mison/antigravity-go/internal/config"
	"github.com/mison/antigravity-go/internal/session"
)

func TestHandleTasksReturnsSummary(t *testing.T) {
	tempDir := t.TempDir()
	taskRoot := filepath.Join(tempDir, "tasks")
	manager := session.NewTaskManager(taskRoot)

	runningID, err := manager.CreateTask("Phase 6A running", "step-1")
	if err != nil {
		t.Fatalf("CreateTask running failed: %v", err)
	}
	if err := manager.UpdateTask(runningID, session.TaskStatusValidating, "checking", "step-2"); err != nil {
		t.Fatalf("UpdateTask running failed: %v", err)
	}

	successID, err := manager.CreateTask("Phase 6A success", "step-3")
	if err != nil {
		t.Fatalf("CreateTask success failed: %v", err)
	}
	if err := manager.UpdateTask(successID, session.TaskStatusSuccess, "ok", "step-4"); err != nil {
		t.Fatalf("UpdateTask success failed: %v", err)
	}

	failedID, err := manager.CreateTask("Phase 6A failed", "step-5")
	if err != nil {
		t.Fatalf("CreateTask failed failed: %v", err)
	}
	if err := manager.UpdateTask(failedID, session.TaskStatusFailed, "boom", "step-6"); err != nil {
		t.Fatalf("UpdateTask failed failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/tasks", nil)
	resp := httptest.NewRecorder()

	srv := &Server{tasksRoot: taskRoot}
	srv.handleTasks(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", resp.Code, resp.Body.String())
	}

	var payload taskSummaryResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	if payload.Total != 3 {
		t.Fatalf("unexpected total: %d", payload.Total)
	}
	if payload.Success != 1 || payload.Failed != 1 || payload.InProgress != 1 {
		t.Fatalf("unexpected counters: %+v", payload)
	}
	if payload.SuccessRate != 50 {
		t.Fatalf("unexpected success rate: %v", payload.SuccessRate)
	}
	if payload.CurrentTask == nil || payload.CurrentTask.ID != runningID {
		t.Fatalf("unexpected current task: %+v", payload.CurrentTask)
	}
	if payload.RecentFailure == nil || payload.RecentFailure.ID != failedID {
		t.Fatalf("unexpected recent failure: %+v", payload.RecentFailure)
	}
}

func TestTaskStoreRootFallsBackToConfigDataDir(t *testing.T) {
	tempDir := t.TempDir()
	srv := &Server{
		cfg: config.Config{
			DataDir: tempDir,
		},
	}

	if got, want := srv.taskStoreRoot(), filepath.Join(tempDir, "tasks"); got != want {
		t.Fatalf("unexpected task store root: got %q want %q", got, want)
	}
}

func TestSummarizeTasksHandlesEmptyList(t *testing.T) {
	payload := summarizeTasks(nil)

	if payload.Total != 0 {
		t.Fatalf("unexpected total: %d", payload.Total)
	}
	if payload.SuccessRate != 0 {
		t.Fatalf("unexpected success rate: %v", payload.SuccessRate)
	}
	if payload.CurrentTask != nil {
		t.Fatalf("expected nil current task, got %+v", payload.CurrentTask)
	}
	if payload.RecentFailure != nil {
		t.Fatalf("expected nil recent failure, got %+v", payload.RecentFailure)
	}
}
