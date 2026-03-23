package session

import (
	"testing"
	"time"
)

func TestTaskManagerCreateAndUpdateTask(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewTaskManager(tmpDir)

	now := time.Date(2026, time.March, 23, 10, 0, 0, 0, time.UTC)
	manager.now = func() time.Time {
		now = now.Add(time.Second)
		return now
	}

	taskID, err := manager.CreateTask("Phase 6A: 控制面收敛", "step-1")
	if err != nil {
		t.Fatalf("CreateTask returned error: %v", err)
	}

	record, err := manager.LoadTask(taskID)
	if err != nil {
		t.Fatalf("LoadTask returned error: %v", err)
	}
	if record.Reference != "Phase 6A: 控制面收敛" {
		t.Fatalf("unexpected reference: %q", record.Reference)
	}
	if record.Status != TaskStatusPending {
		t.Fatalf("expected pending status, got %q", record.Status)
	}
	if record.RollbackPoint != "step-1" {
		t.Fatalf("unexpected rollback point: %q", record.RollbackPoint)
	}
	if record.CreatedAt.IsZero() || record.UpdatedAt.IsZero() {
		t.Fatal("expected timestamps to be set")
	}

	if err := manager.UpdateTask(taskID, TaskStatusRunning, "validation passed", "step-2"); err != nil {
		t.Fatalf("UpdateTask returned error: %v", err)
	}

	updated, err := manager.LoadTask(taskID)
	if err != nil {
		t.Fatalf("LoadTask returned error after update: %v", err)
	}
	if updated.Status != TaskStatusRunning {
		t.Fatalf("expected running status, got %q", updated.Status)
	}
	if updated.Evidence != "validation passed" {
		t.Fatalf("unexpected evidence: %q", updated.Evidence)
	}
	if updated.RollbackPoint != "step-2" {
		t.Fatalf("unexpected rollback point after update: %q", updated.RollbackPoint)
	}
	if !updated.UpdatedAt.After(updated.CreatedAt) {
		t.Fatalf("expected updated_at to advance: created=%s updated=%s", updated.CreatedAt, updated.UpdatedAt)
	}
}
