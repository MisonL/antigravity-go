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

func TestHandleExecutionsSummaryReturnsExecutionRecords(t *testing.T) {
	root := filepath.Join(t.TempDir(), "executions")
	store := session.NewExecutionStore(root)

	id, err := store.CreateTask("Execution summary", "step-1")
	if err != nil {
		t.Fatalf("CreateTask returned error: %v", err)
	}
	if err := store.UpdateTask(id, session.ExecutionStatusRunning, "started", "step-2"); err != nil {
		t.Fatalf("UpdateTask returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/executions/summary", nil)
	resp := httptest.NewRecorder()

	srv := &Server{executionsRoot: root}
	srv.handleExecutionsSummary(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", resp.Code, resp.Body.String())
	}

	var payload taskSummaryResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if payload.Total != 1 {
		t.Fatalf("unexpected total: %d", payload.Total)
	}
	if payload.CurrentExecution == nil || payload.CurrentExecution.ID != id {
		t.Fatalf("unexpected current execution: %+v", payload.CurrentExecution)
	}
	if len(payload.Executions) != 1 || payload.Executions[0].ID != id {
		t.Fatalf("unexpected executions: %+v", payload.Executions)
	}
}

func TestHandleExecutionDetailReturnsTimelineAndSteps(t *testing.T) {
	root := filepath.Join(t.TempDir(), "executions")
	store := session.NewExecutionStore(root)

	id, err := store.CreateTask("Execution detail", "step-1")
	if err != nil {
		t.Fatalf("CreateTask returned error: %v", err)
	}
	if err := store.AppendTaskEvent(id, "tool_start", map[string]any{"name": "write_file", "args": "{}"}); err != nil {
		t.Fatalf("AppendTaskEvent(tool_start) returned error: %v", err)
	}
	if err := store.AppendTaskEvent(id, "tool_end", map[string]any{"name": "write_file", "result": "ok"}); err != nil {
		t.Fatalf("AppendTaskEvent(tool_end) returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/executions/"+id, nil)
	resp := httptest.NewRecorder()

	srv := &Server{executionsRoot: root}
	srv.handleExecutionResource(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", resp.Code, resp.Body.String())
	}

	var payload executionDetailResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if payload.Execution == nil || payload.Execution.ID != id {
		t.Fatalf("unexpected execution detail: %+v", payload.Execution)
	}
	if len(payload.Timeline) < 3 {
		t.Fatalf("unexpected timeline length: %d", len(payload.Timeline))
	}
	if len(payload.Steps) == 0 || payload.Steps[0].Title != "write_file" {
		t.Fatalf("unexpected steps: %+v", payload.Steps)
	}
}

func TestHandleExecutionTimelineReturnsEvents(t *testing.T) {
	root := filepath.Join(t.TempDir(), "executions")
	store := session.NewExecutionStore(root)

	id, err := store.CreateTask("Execution timeline", "step-1")
	if err != nil {
		t.Fatalf("CreateTask returned error: %v", err)
	}
	if err := store.AppendTaskEvent(id, "tool_start", map[string]any{"name": "write_file", "args": "{}"}); err != nil {
		t.Fatalf("AppendTaskEvent(tool_start) returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/executions/"+id+"/timeline", nil)
	resp := httptest.NewRecorder()

	srv := &Server{executionsRoot: root}
	srv.handleExecutionResource(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", resp.Code, resp.Body.String())
	}

	var payload executionTimelineResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if len(payload.Events) < 2 {
		t.Fatalf("unexpected timeline events: %+v", payload.Events)
	}
	if payload.Events[1].Type != "tool_start" {
		t.Fatalf("unexpected event type: %+v", payload.Events[1])
	}
}

func TestHandleExecutionsSummaryFallsBackToLegacyTasks(t *testing.T) {
	taskRoot := filepath.Join(t.TempDir(), "tasks")
	manager := session.NewTaskManager(taskRoot)

	id, err := manager.CreateTask("Legacy execution summary", "legacy-step-1")
	if err != nil {
		t.Fatalf("CreateTask returned error: %v", err)
	}
	if err := manager.UpdateTask(id, session.TaskStatusValidating, "checking", "legacy-step-2"); err != nil {
		t.Fatalf("UpdateTask returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/executions/summary", nil)
	resp := httptest.NewRecorder()

	srv := &Server{
		executionsRoot: filepath.Join(t.TempDir(), "executions"),
		tasksRoot:      taskRoot,
	}
	srv.handleExecutionsSummary(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", resp.Code, resp.Body.String())
	}

	var payload taskSummaryResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if payload.Total != 1 {
		t.Fatalf("unexpected total: %d", payload.Total)
	}
	if payload.CurrentExecution == nil || payload.CurrentExecution.ID != id {
		t.Fatalf("unexpected current execution: %+v", payload.CurrentExecution)
	}
}

func TestHandleExecutionDetailFallsBackToLegacyTask(t *testing.T) {
	taskRoot := filepath.Join(t.TempDir(), "tasks")
	manager := session.NewTaskManager(taskRoot)

	id, err := manager.CreateTask("Legacy execution detail", "legacy-step-1")
	if err != nil {
		t.Fatalf("CreateTask returned error: %v", err)
	}
	if err := manager.UpdateTask(id, session.TaskStatusSuccess, "legacy ok", "legacy-step-2"); err != nil {
		t.Fatalf("UpdateTask returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/executions/"+id, nil)
	resp := httptest.NewRecorder()

	srv := &Server{
		executionsRoot: filepath.Join(t.TempDir(), "executions"),
		tasksRoot:      taskRoot,
	}
	srv.handleExecutionResource(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", resp.Code, resp.Body.String())
	}

	var payload executionDetailResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if payload.Execution == nil || payload.Execution.ID != id {
		t.Fatalf("unexpected execution: %+v", payload.Execution)
	}
	if len(payload.Timeline) != 1 || payload.Timeline[0].Type != "execution.status_changed" {
		t.Fatalf("unexpected timeline: %+v", payload.Timeline)
	}
	if len(payload.Steps) != 1 || payload.Steps[0].Summary != "legacy ok" {
		t.Fatalf("unexpected steps: %+v", payload.Steps)
	}
}

func TestExecutionStoreRootFallsBackToConfigDataDir(t *testing.T) {
	tempDir := t.TempDir()
	srv := &Server{
		cfg: config.Config{DataDir: tempDir},
	}

	if got, want := srv.executionStoreRoot(), filepath.Join(tempDir, "executions"); got != want {
		t.Fatalf("unexpected execution store root: got %q want %q", got, want)
	}
}
