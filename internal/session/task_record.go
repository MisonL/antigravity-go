package session

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	TaskStatusPending    = "pending"
	TaskStatusRunning    = "running"
	TaskStatusValidating = "validating"
	TaskStatusSuccess    = "success"
	TaskStatusFailed     = "failed"
)

type TaskRecord struct {
	ID            string    `json:"id"`
	Reference     string    `json:"reference"`
	Status        string    `json:"status"`
	Evidence      string    `json:"evidence"`
	RollbackPoint string    `json:"rollbackPoint"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

type TaskManager struct {
	rootDir string
	now     func() time.Time
}

func NewTaskManager(rootDir string) *TaskManager {
	return &TaskManager{
		rootDir: resolveTaskRoot(rootDir),
		now:     time.Now().UTC,
	}
}

func (m *TaskManager) CreateTask(reference, rollbackPoint string) (string, error) {
	record := &TaskRecord{
		ID:            generateID(),
		Reference:     strings.TrimSpace(reference),
		Status:        TaskStatusPending,
		RollbackPoint: strings.TrimSpace(rollbackPoint),
	}
	if err := m.save(record, true); err != nil {
		return "", err
	}
	return record.ID, nil
}

func (m *TaskManager) UpdateTask(id, status, evidence, rollbackPoint string) error {
	record, err := m.LoadTask(id)
	if err != nil {
		return err
	}

	if trimmed := strings.TrimSpace(status); trimmed != "" {
		record.Status = trimmed
	}
	if trimmed := strings.TrimSpace(evidence); trimmed != "" {
		record.Evidence = trimmed
	}
	if trimmed := strings.TrimSpace(rollbackPoint); trimmed != "" {
		record.RollbackPoint = trimmed
	}

	return m.save(record, false)
}

func (m *TaskManager) LoadTask(id string) (*TaskRecord, error) {
	if strings.TrimSpace(id) == "" {
		return nil, fmt.Errorf("task id is required")
	}

	var record TaskRecord
	if err := readJSON(m.recordPath(id), &record); err != nil {
		return nil, err
	}
	return &record, nil
}

func (m *TaskManager) save(record *TaskRecord, isNew bool) error {
	if m == nil {
		return fmt.Errorf("task manager is nil")
	}
	if record == nil {
		return fmt.Errorf("task record is nil")
	}
	if strings.TrimSpace(record.ID) == "" {
		return fmt.Errorf("task id is required")
	}
	if strings.TrimSpace(record.Reference) == "" {
		return fmt.Errorf("task reference is required")
	}
	if !isValidTaskStatus(record.Status) {
		return fmt.Errorf("invalid task status %q", record.Status)
	}

	if err := os.MkdirAll(m.rootDir, 0755); err != nil {
		return err
	}

	now := m.now()
	if isNew || record.CreatedAt.IsZero() {
		record.CreatedAt = now
	}
	record.UpdatedAt = now

	return writeJSON(m.recordPath(record.ID), record)
}

func (m *TaskManager) recordPath(id string) string {
	return filepath.Join(m.rootDir, strings.TrimSpace(id)+".json")
}

func resolveTaskRoot(rootDir string) string {
	if trimmed := strings.TrimSpace(rootDir); trimmed != "" {
		return trimmed
	}

	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return filepath.Join(".agy_go", "tasks")
	}
	return filepath.Join(home, ".agy_go", "tasks")
}

func isValidTaskStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case TaskStatusPending, TaskStatusRunning, TaskStatusValidating, TaskStatusSuccess, TaskStatusFailed:
		return true
	default:
		return false
	}
}
