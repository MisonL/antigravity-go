package server

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/mison/antigravity-go/internal/session"
)

type taskSummaryItem struct {
	ID        string    `json:"id"`
	Reference string    `json:"reference"`
	Status    string    `json:"status"`
	UpdatedAt time.Time `json:"updated_at"`
}

type taskSummaryResponse struct {
	GeneratedAt   time.Time         `json:"generated_at"`
	Total         int               `json:"total"`
	Success       int               `json:"success"`
	Failed        int               `json:"failed"`
	InProgress    int               `json:"in_progress"`
	SuccessRate   float64           `json:"success_rate"`
	CurrentTask   *taskSummaryItem  `json:"current_task,omitempty"`
	RecentFailure *taskSummaryItem  `json:"recent_failure,omitempty"`
	Tasks         []taskSummaryItem `json:"tasks"`
}

func (s *Server) handleTasks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	manager := session.NewTaskManager(s.taskStoreRoot())
	records, err := manager.ListTasks()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := summarizeTasks(records)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
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

func summarizeTasks(records []session.TaskRecord) taskSummaryResponse {
	response := taskSummaryResponse{
		GeneratedAt: time.Now().UTC(),
		Tasks:       make([]taskSummaryItem, 0, len(records)),
	}

	for _, record := range records {
		item := taskSummaryItem{
			ID:        record.ID,
			Reference: record.Reference,
			Status:    record.Status,
			UpdatedAt: record.UpdatedAt,
		}
		response.Tasks = append(response.Tasks, item)
		response.Total++

		switch record.Status {
		case session.TaskStatusSuccess:
			response.Success++
		case session.TaskStatusFailed:
			response.Failed++
			if response.RecentFailure == nil {
				copy := item
				response.RecentFailure = &copy
			}
		case session.TaskStatusPending, session.TaskStatusRunning, session.TaskStatusValidating:
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

	return response
}
