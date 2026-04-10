package session

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	ExecutionStatusPending          = "pending"
	ExecutionStatusRunning          = "running"
	ExecutionStatusAwaitingApproval = "awaiting_approval"
	ExecutionStatusValidating       = "validating"
	ExecutionStatusSuccess          = "success"
	ExecutionStatusFailed           = "failed"
	ExecutionStatusBlocked          = "blocked"
	ExecutionStatusRolledBack       = "rolled_back"
)

type ExecutionRecord struct {
	ID                 string    `json:"id"`
	Reference          string    `json:"reference"`
	Status             string    `json:"status"`
	Evidence           string    `json:"evidence,omitempty"`
	RollbackPoint      string    `json:"rollback_point,omitempty"`
	CurrentBranchID    string    `json:"current_branch_id,omitempty"`
	LatestCheckpointID string    `json:"latest_checkpoint_id,omitempty"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type ExecutionEvent struct {
	ID      string         `json:"id"`
	Time    time.Time      `json:"time"`
	Type    string         `json:"type"`
	Status  string         `json:"status,omitempty"`
	Title   string         `json:"title,omitempty"`
	StepID  string         `json:"step_id,omitempty"`
	Message string         `json:"message,omitempty"`
	Data    map[string]any `json:"data,omitempty"`
}

type ExecutionStep struct {
	ID         string `json:"id"`
	Kind       string `json:"kind"`
	Title      string `json:"title"`
	Status     string `json:"status"`
	StartedAt  string `json:"started_at,omitempty"`
	FinishedAt string `json:"finished_at,omitempty"`
	Summary    string `json:"summary,omitempty"`
}

type ExecutionStore struct {
	rootDir string
	now     func() time.Time
	mu      sync.Mutex
}

func NewExecutionStore(rootDir string) *ExecutionStore {
	return &ExecutionStore{
		rootDir: resolveExecutionRoot(rootDir),
		now:     time.Now().UTC,
	}
}

func (s *ExecutionStore) CreateTask(reference, rollbackPoint string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	record := &ExecutionRecord{
		ID:              generateID(),
		Reference:       strings.TrimSpace(reference),
		Status:          ExecutionStatusPending,
		RollbackPoint:   strings.TrimSpace(rollbackPoint),
		CurrentBranchID: "main",
	}
	if err := s.save(record, true); err != nil {
		return "", err
	}
	if err := s.appendEventLocked(record.ID, ExecutionEvent{
		ID:      generateID(),
		Time:    s.now(),
		Type:    "execution.created",
		Status:  record.Status,
		Title:   record.Reference,
		Message: "execution created",
		Data: map[string]any{
			"reference":      record.Reference,
			"rollback_point": record.RollbackPoint,
			"current_branch": record.CurrentBranchID,
			"checkpoint_id":  record.LatestCheckpointID,
		},
	}); err != nil {
		return "", err
	}
	return record.ID, nil
}

func (s *ExecutionStore) UpdateTask(id, status, evidence, rollbackPoint string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	record, err := s.load(id)
	if err != nil {
		return err
	}

	previousStatus := record.Status
	if trimmed := strings.TrimSpace(status); trimmed != "" {
		record.Status = trimmed
	}
	if trimmed := strings.TrimSpace(evidence); trimmed != "" {
		record.Evidence = trimmed
	}
	if trimmed := strings.TrimSpace(rollbackPoint); trimmed != "" {
		record.RollbackPoint = trimmed
		record.LatestCheckpointID = trimmed
	}
	if err := s.save(record, false); err != nil {
		return err
	}

	payload := map[string]any{
		"previous_status": previousStatus,
		"status":          record.Status,
		"rollback_point":  record.RollbackPoint,
	}
	if strings.TrimSpace(record.Evidence) != "" {
		payload["evidence"] = truncateExecutionText(record.Evidence, 1600)
	}

	return s.appendEventLocked(record.ID, ExecutionEvent{
		ID:      generateID(),
		Time:    s.now(),
		Type:    "execution.status_changed",
		Status:  record.Status,
		Title:   record.Reference,
		StepID:  record.RollbackPoint,
		Message: fmt.Sprintf("execution moved to %s", record.Status),
		Data:    payload,
	})
}

func (s *ExecutionStore) AppendTaskEvent(id, eventType string, data map[string]any) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	record, err := s.load(id)
	if err != nil {
		return err
	}
	if strings.TrimSpace(eventType) == "" {
		return fmt.Errorf("event type is required")
	}

	event := ExecutionEvent{
		ID:      generateID(),
		Time:    s.now(),
		Type:    eventType,
		Status:  record.Status,
		Title:   record.Reference,
		Message: defaultExecutionEventMessage(eventType, data),
		Data:    sanitizeExecutionEventData(data),
	}
	if stepID, ok := event.Data["step_id"].(string); ok && strings.TrimSpace(stepID) != "" {
		event.StepID = strings.TrimSpace(stepID)
		record.LatestCheckpointID = event.StepID
	}
	if rollbackPoint, ok := event.Data["rollback_point"].(string); ok && strings.TrimSpace(rollbackPoint) != "" {
		record.RollbackPoint = strings.TrimSpace(rollbackPoint)
		record.LatestCheckpointID = record.RollbackPoint
	}
	if err := s.save(record, false); err != nil {
		return err
	}
	return s.appendEventLocked(record.ID, event)
}

func (s *ExecutionStore) LoadExecution(id string) (*ExecutionRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.load(id)
}

func (s *ExecutionStore) ListExecutions() ([]ExecutionRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entries, err := os.ReadDir(s.rootDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []ExecutionRecord{}, nil
		}
		return nil, err
	}

	records := make([]ExecutionRecord, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		var record ExecutionRecord
		if err := readJSON(s.executionPath(entry.Name()), &record); err != nil {
			return nil, err
		}
		records = append(records, record)
	}

	sort.Slice(records, func(i, j int) bool {
		if records[i].UpdatedAt.Equal(records[j].UpdatedAt) {
			return records[i].CreatedAt.After(records[j].CreatedAt)
		}
		return records[i].UpdatedAt.After(records[j].UpdatedAt)
	})
	return records, nil
}

func (s *ExecutionStore) LoadTimeline(id string) ([]ExecutionEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadTimelineLocked(id)
}

func (s *ExecutionStore) LoadDerivedSteps(id string) ([]ExecutionStep, error) {
	events, err := s.LoadTimeline(id)
	if err != nil {
		return nil, err
	}
	return deriveExecutionSteps(events), nil
}

func (s *ExecutionStore) load(id string) (*ExecutionRecord, error) {
	if strings.TrimSpace(id) == "" {
		return nil, fmt.Errorf("execution id is required")
	}
	var record ExecutionRecord
	if err := readJSON(s.executionPath(id), &record); err != nil {
		return nil, err
	}
	return &record, nil
}

func (s *ExecutionStore) save(record *ExecutionRecord, isNew bool) error {
	if s == nil {
		return fmt.Errorf("execution store is nil")
	}
	if record == nil {
		return fmt.Errorf("execution record is nil")
	}
	if strings.TrimSpace(record.ID) == "" {
		return fmt.Errorf("execution id is required")
	}
	if strings.TrimSpace(record.Reference) == "" {
		return fmt.Errorf("execution reference is required")
	}
	if !isValidExecutionStatus(record.Status) {
		return fmt.Errorf("invalid execution status %q", record.Status)
	}
	if err := os.MkdirAll(s.recordDir(record.ID), 0755); err != nil {
		return err
	}
	if isNew || record.CreatedAt.IsZero() {
		record.CreatedAt = s.now()
	}
	record.UpdatedAt = s.now()
	if strings.TrimSpace(record.CurrentBranchID) == "" {
		record.CurrentBranchID = "main"
	}
	return writeJSON(s.executionPath(record.ID), record)
}

func (s *ExecutionStore) loadTimelineLocked(id string) ([]ExecutionEvent, error) {
	path := s.eventsPath(id)
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []ExecutionEvent{}, nil
		}
		return nil, err
	}
	defer file.Close()

	events := []ExecutionEvent{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Bytes()
		var event ExecutionEvent
		if err := json.Unmarshal(line, &event); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return events, nil
}

func (s *ExecutionStore) appendEventLocked(id string, event ExecutionEvent) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("execution id is required")
	}
	if strings.TrimSpace(event.ID) == "" {
		event.ID = generateID()
	}
	if event.Time.IsZero() {
		event.Time = s.now()
	}
	if strings.TrimSpace(event.Type) == "" {
		return fmt.Errorf("execution event type is required")
	}
	if err := os.MkdirAll(s.recordDir(id), 0755); err != nil {
		return err
	}
	file, err := os.OpenFile(s.eventsPath(id), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer file.Close()
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	_, err = file.Write(append(payload, '\n'))
	return err
}

func (s *ExecutionStore) recordDir(id string) string {
	return filepath.Join(s.rootDir, strings.TrimSpace(id))
}

func (s *ExecutionStore) executionPath(id string) string {
	return filepath.Join(s.recordDir(id), "execution.json")
}

func (s *ExecutionStore) eventsPath(id string) string {
	return filepath.Join(s.recordDir(id), "events.jsonl")
}

func resolveExecutionRoot(rootDir string) string {
	if trimmed := strings.TrimSpace(rootDir); trimmed != "" {
		return trimmed
	}
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return filepath.Join(".ago", "executions")
	}
	return filepath.Join(home, ".ago", "executions")
}

func isValidExecutionStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case ExecutionStatusPending, ExecutionStatusRunning, ExecutionStatusAwaitingApproval, ExecutionStatusValidating, ExecutionStatusSuccess, ExecutionStatusFailed, ExecutionStatusBlocked, ExecutionStatusRolledBack:
		return true
	default:
		return false
	}
}

func sanitizeExecutionEventData(data map[string]any) map[string]any {
	if len(data) == 0 {
		return nil
	}
	out := make(map[string]any, len(data))
	for key, value := range data {
		switch v := value.(type) {
		case string:
			out[key] = truncateExecutionText(v, 4000)
		default:
			out[key] = v
		}
	}
	return out
}

func truncateExecutionText(text string, maxLen int) string {
	text = strings.TrimSpace(text)
	if maxLen <= 0 || len(text) <= maxLen {
		return text
	}
	return text[:maxLen] + "\n...(truncated)"
}

func defaultExecutionEventMessage(eventType string, data map[string]any) string {
	switch eventType {
	case "execution.started":
		return "execution started"
	case "user_message":
		return "user message appended"
	case "assistant_message":
		return "assistant message appended"
	case "tool_start":
		if name, ok := data["name"].(string); ok && strings.TrimSpace(name) != "" {
			return fmt.Sprintf("tool %s started", strings.TrimSpace(name))
		}
		return "tool started"
	case "tool_end":
		if name, ok := data["name"].(string); ok && strings.TrimSpace(name) != "" {
			return fmt.Sprintf("tool %s completed", strings.TrimSpace(name))
		}
		return "tool completed"
	case "tool_error":
		if name, ok := data["name"].(string); ok && strings.TrimSpace(name) != "" {
			return fmt.Sprintf("tool %s failed", strings.TrimSpace(name))
		}
		return "tool failed"
	default:
		return strings.ReplaceAll(eventType, ".", " ")
	}
}

func deriveExecutionSteps(events []ExecutionEvent) []ExecutionStep {
	steps := make([]ExecutionStep, 0)
	openIndexes := make(map[string]int)
	for _, event := range events {
		switch event.Type {
		case "tool_start":
			name, _ := event.Data["name"].(string)
			args, _ := event.Data["args"].(string)
			step := ExecutionStep{
				ID:        event.ID,
				Kind:      "tool",
				Title:     fallbackString(strings.TrimSpace(name), "tool"),
				Status:    "executing",
				StartedAt: event.Time.Format(time.RFC3339),
				Summary:   truncateExecutionText(args, 240),
			}
			steps = append(steps, step)
			openIndexes[step.Title] = len(steps) - 1
		case "tool_end", "tool_error":
			name, _ := event.Data["name"].(string)
			result, _ := event.Data["result"].(string)
			title := fallbackString(strings.TrimSpace(name), "tool")
			idx, ok := openIndexes[title]
			if !ok {
				step := ExecutionStep{
					ID:         event.ID,
					Kind:       "tool",
					Title:      title,
					Status:     mapExecutionStepStatus(event.Type),
					StartedAt:  event.Time.Format(time.RFC3339),
					FinishedAt: event.Time.Format(time.RFC3339),
					Summary:    truncateExecutionText(result, 240),
				}
				steps = append(steps, step)
				continue
			}
			steps[idx].Status = mapExecutionStepStatus(event.Type)
			steps[idx].FinishedAt = event.Time.Format(time.RFC3339)
			if strings.TrimSpace(result) != "" {
				steps[idx].Summary = truncateExecutionText(result, 240)
			}
			delete(openIndexes, title)
		case "execution.status_changed":
			status := strings.TrimSpace(event.Status)
			if status == "" {
				continue
			}
			steps = append(steps, ExecutionStep{
				ID:         event.ID,
				Kind:       "status",
				Title:      "execution",
				Status:     status,
				StartedAt:  event.Time.Format(time.RFC3339),
				FinishedAt: event.Time.Format(time.RFC3339),
				Summary:    truncateExecutionText(event.Message, 160),
			})
		}
	}
	return steps
}

func mapExecutionStepStatus(eventType string) string {
	switch eventType {
	case "tool_error":
		return "failed"
	default:
		return "succeeded"
	}
}

func fallbackString(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
