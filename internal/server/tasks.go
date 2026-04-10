package server

import (
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mison/antigravity-go/internal/session"
)

type taskSummaryItem struct {
	ID                 string    `json:"id"`
	Reference          string    `json:"reference"`
	Status             string    `json:"status"`
	UpdatedAt          time.Time `json:"updated_at"`
	CurrentBranchID    string    `json:"current_branch_id,omitempty"`
	LatestCheckpointID string    `json:"latest_checkpoint_id,omitempty"`
}

type taskSummaryResponse struct {
	GeneratedAt      time.Time         `json:"generated_at"`
	Total            int               `json:"total"`
	Success          int               `json:"success"`
	Failed           int               `json:"failed"`
	InProgress       int               `json:"in_progress"`
	SuccessRate      float64           `json:"success_rate"`
	CurrentTask      *taskSummaryItem  `json:"current_task,omitempty"`
	CurrentExecution *taskSummaryItem  `json:"current_execution,omitempty"`
	RecentFailure    *taskSummaryItem  `json:"recent_failure,omitempty"`
	Tasks            []taskSummaryItem `json:"tasks"`
	Executions       []taskSummaryItem `json:"executions"`
}

type executionDetailResponse struct {
	Execution *session.ExecutionRecord `json:"execution"`
	Timeline  []session.ExecutionEvent `json:"timeline"`
	Steps     []session.ExecutionStep  `json:"steps"`
}

type executionTimelineResponse struct {
	Events []session.ExecutionEvent `json:"events"`
}

func (s *Server) handleTasks(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	records, err := s.loadExecutionSummaryCompat()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, summarizeExecutions(records))
}

func (s *Server) handleExecutionsSummary(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	records, err := s.loadExecutionSummaryCompat()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, summarizeExecutions(records))
}

func (s *Server) handleExecutionResource(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/api/executions/" || r.URL.Path == "/api/executions" {
		http.NotFound(w, r)
		return
	}

	trimmed := strings.TrimPrefix(r.URL.Path, "/api/executions/")
	trimmed = strings.Trim(trimmed, "/")
	if trimmed == "" {
		http.NotFound(w, r)
		return
	}
	parts := strings.Split(trimmed, "/")
	id := strings.TrimSpace(parts[0])
	if id == "" {
		http.NotFound(w, r)
		return
	}

	if len(parts) == 1 {
		s.handleExecutionDetail(w, r, id)
		return
	}
	if len(parts) == 2 && parts[1] == "timeline" {
		s.handleExecutionTimeline(w, r, id)
		return
	}

	http.NotFound(w, r)
}

func (s *Server) handleExecutionDetail(w http.ResponseWriter, r *http.Request, id string) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	record, timeline, steps, err := s.loadExecutionDetailCompat(id)
	if err != nil {
		status := http.StatusInternalServerError
		if os.IsNotExist(err) {
			status = http.StatusNotFound
		}
		http.Error(w, err.Error(), status)
		return
	}

	writeJSON(w, executionDetailResponse{
		Execution: record,
		Timeline:  timeline,
		Steps:     steps,
	})
}

func (s *Server) handleExecutionTimeline(w http.ResponseWriter, r *http.Request, id string) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	_, timeline, _, err := s.loadExecutionDetailCompat(id)
	if err != nil {
		status := http.StatusInternalServerError
		if os.IsNotExist(err) {
			status = http.StatusNotFound
		}
		http.Error(w, err.Error(), status)
		return
	}

	writeJSON(w, executionTimelineResponse{Events: timeline})
}

func (s *Server) loadExecutionSummaryCompat() ([]session.ExecutionRecord, error) {
	root := s.executionStoreRoot()
	if root != "" && directoryHasChildEntries(root) {
		return session.NewExecutionStore(root).ListExecutions()
	}

	legacyRoot := s.taskStoreRoot()
	if legacyRoot == "" {
		return []session.ExecutionRecord{}, nil
	}
	legacyRecords, err := session.NewTaskManager(legacyRoot).ListTasks()
	if err != nil {
		return nil, err
	}
	records := make([]session.ExecutionRecord, 0, len(legacyRecords))
	for _, record := range legacyRecords {
		records = append(records, legacyTaskToExecution(record))
	}
	return records, nil
}

func (s *Server) loadExecutionDetailCompat(id string) (*session.ExecutionRecord, []session.ExecutionEvent, []session.ExecutionStep, error) {
	root := s.executionStoreRoot()
	if root != "" {
		store := session.NewExecutionStore(root)
		record, err := store.LoadExecution(id)
		if err == nil {
			timeline, timelineErr := store.LoadTimeline(id)
			if timelineErr != nil {
				return nil, nil, nil, timelineErr
			}
			steps, stepsErr := store.LoadDerivedSteps(id)
			if stepsErr != nil {
				return nil, nil, nil, stepsErr
			}
			return record, timeline, steps, nil
		}
		if !os.IsNotExist(err) {
			return nil, nil, nil, err
		}
	}

	legacyRoot := s.taskStoreRoot()
	if legacyRoot == "" {
		return nil, nil, nil, os.ErrNotExist
	}
	record, err := session.NewTaskManager(legacyRoot).LoadTask(id)
	if err != nil {
		return nil, nil, nil, err
	}
	execution := legacyTaskToExecution(*record)
	event := session.ExecutionEvent{
		ID:      execution.ID,
		Time:    execution.UpdatedAt,
		Type:    "execution.status_changed",
		Status:  execution.Status,
		Title:   execution.Reference,
		StepID:  execution.RollbackPoint,
		Message: "legacy task migrated at read time",
		Data: map[string]any{
			"evidence": execution.Evidence,
		},
	}
	step := session.ExecutionStep{
		ID:         execution.ID,
		Kind:       "status",
		Title:      "execution",
		Status:     execution.Status,
		StartedAt:  execution.UpdatedAt.Format(time.RFC3339),
		FinishedAt: execution.UpdatedAt.Format(time.RFC3339),
		Summary:    execution.Evidence,
	}
	return &execution, []session.ExecutionEvent{event}, []session.ExecutionStep{step}, nil
}

func (s *Server) executionStoreRoot() string {
	if strings.TrimSpace(s.executionsRoot) != "" {
		return s.executionsRoot
	}
	if strings.TrimSpace(s.cfg.DataDir) != "" {
		return filepath.Join(s.cfg.DataDir, "executions")
	}
	return ""
}

func (s *Server) taskStoreRoot() string {
	if strings.TrimSpace(s.tasksRoot) != "" {
		return s.tasksRoot
	}
	if strings.TrimSpace(s.cfg.DataDir) != "" {
		return filepath.Join(s.cfg.DataDir, "tasks")
	}
	return ""
}

func summarizeExecutions(records []session.ExecutionRecord) taskSummaryResponse {
	response := taskSummaryResponse{
		GeneratedAt: time.Now().UTC(),
		Tasks:       make([]taskSummaryItem, 0, len(records)),
		Executions:  make([]taskSummaryItem, 0, len(records)),
	}

	sort.Slice(records, func(i, j int) bool {
		if records[i].UpdatedAt.Equal(records[j].UpdatedAt) {
			return records[i].CreatedAt.After(records[j].CreatedAt)
		}
		return records[i].UpdatedAt.After(records[j].UpdatedAt)
	})

	for _, record := range records {
		item := taskSummaryItem{
			ID:                 record.ID,
			Reference:          record.Reference,
			Status:             record.Status,
			UpdatedAt:          record.UpdatedAt,
			CurrentBranchID:    record.CurrentBranchID,
			LatestCheckpointID: record.LatestCheckpointID,
		}
		response.Tasks = append(response.Tasks, item)
		response.Executions = append(response.Executions, item)
		response.Total++

		switch record.Status {
		case session.ExecutionStatusSuccess:
			response.Success++
		case session.ExecutionStatusFailed, session.ExecutionStatusBlocked:
			response.Failed++
			if response.RecentFailure == nil {
				copy := item
				response.RecentFailure = &copy
			}
		case session.ExecutionStatusPending, session.ExecutionStatusRunning, session.ExecutionStatusAwaitingApproval, session.ExecutionStatusValidating:
			response.InProgress++
			if response.CurrentTask == nil {
				copy := item
				response.CurrentTask = &copy
			}
		}
	}

	completed := response.Success + response.Failed
	if completed > 0 {
		response.SuccessRate = float64(response.Success) / float64(completed) * 100
	}
	response.CurrentExecution = response.CurrentTask
	return response
}

func summarizeTasks(records []session.TaskRecord) taskSummaryResponse {
	executions := make([]session.ExecutionRecord, 0, len(records))
	for _, record := range records {
		executions = append(executions, legacyTaskToExecution(record))
	}
	return summarizeExecutions(executions)
}

func legacyTaskToExecution(record session.TaskRecord) session.ExecutionRecord {
	return session.ExecutionRecord{
		ID:                 record.ID,
		Reference:          record.Reference,
		Status:             record.Status,
		Evidence:           record.Evidence,
		RollbackPoint:      record.RollbackPoint,
		CurrentBranchID:    "main",
		LatestCheckpointID: record.RollbackPoint,
		CreatedAt:          record.CreatedAt,
		UpdatedAt:          record.UpdatedAt,
	}
}

func directoryHasChildEntries(path string) bool {
	entries, err := os.ReadDir(path)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if entry.IsDir() || !entry.Type().IsRegular() {
			return true
		}
	}
	return false
}
