package session

import (
	"testing"
)

func TestExecutionStoreCreateUpdateAndTimeline(t *testing.T) {
	store := NewExecutionStore(t.TempDir())

	id, err := store.CreateTask("Execution graph bootstrap", "step-1")
	if err != nil {
		t.Fatalf("CreateTask returned error: %v", err)
	}

	if err := store.AppendTaskEvent(id, "tool_start", map[string]any{"name": "write_file", "args": "{}"}); err != nil {
		t.Fatalf("AppendTaskEvent(tool_start) returned error: %v", err)
	}
	if err := store.AppendTaskEvent(id, "tool_end", map[string]any{"name": "write_file", "result": "ok"}); err != nil {
		t.Fatalf("AppendTaskEvent(tool_end) returned error: %v", err)
	}
	if err := store.UpdateTask(id, ExecutionStatusSuccess, "validation passed", "step-2"); err != nil {
		t.Fatalf("UpdateTask returned error: %v", err)
	}

	record, err := store.LoadExecution(id)
	if err != nil {
		t.Fatalf("LoadExecution returned error: %v", err)
	}
	if record.Status != ExecutionStatusSuccess {
		t.Fatalf("unexpected record status: %q", record.Status)
	}
	if record.LatestCheckpointID != "step-2" {
		t.Fatalf("unexpected latest checkpoint: %q", record.LatestCheckpointID)
	}

	timeline, err := store.LoadTimeline(id)
	if err != nil {
		t.Fatalf("LoadTimeline returned error: %v", err)
	}
	if len(timeline) != 4 {
		t.Fatalf("unexpected timeline length: %d", len(timeline))
	}

	steps, err := store.LoadDerivedSteps(id)
	if err != nil {
		t.Fatalf("LoadDerivedSteps returned error: %v", err)
	}
	if len(steps) == 0 {
		t.Fatal("expected derived steps to be present")
	}
	if steps[0].Title != "write_file" {
		t.Fatalf("unexpected first step title: %q", steps[0].Title)
	}
}
