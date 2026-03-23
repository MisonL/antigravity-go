package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/mison/antigravity-go/internal/corecap"
	"github.com/mison/antigravity-go/internal/pkg/i18n"
)

type planeSnapshot struct {
	Count           int    `json:"count"`
	LatestID        string `json:"latest_id,omitempty"`
	LatestUpdatedAt string `json:"latest_updated_at,omitempty"`
}

type observabilitySummary struct {
	Trajectories planeSnapshot `json:"trajectories"`
	Memories     planeSnapshot `json:"memories"`
	GeneratedAt  string        `json:"generated_at"`
}

type rollbackStepRequest struct {
	StepID string `json:"step_id"`
}

func (s *Server) handleObservabilitySummary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.client == nil {
		writeEmptyObservabilitySummary(w)
		return
	}

	trajectoryPayload, err := corecap.NewTrajectoryManager(s.client).List()
	if err != nil {
		if isDeprecatedPlaneRPCError(err) {
			writeEmptyObservabilitySummary(w)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	memoryPayload, err := corecap.NewMemoryManager(s.client).Query(nil)
	if err != nil {
		if isDeprecatedPlaneRPCError(err) {
			writeEmptyObservabilitySummary(w)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	summary := observabilitySummary{
		Trajectories: summarizePlaneCollection(
			trajectoryPayload,
			[]string{"trajectories", "items", "data", "results", "records"},
			[]string{"id", "trajectory_id", "trajectoryId", "uuid"},
			[]string{"updated_at", "updatedAt", "created_at", "createdAt", "timestamp"},
		),
		Memories: summarizePlaneCollection(
			memoryPayload,
			[]string{"memories", "items", "data", "results", "records"},
			[]string{"id", "memory_id", "memoryId", "key"},
			[]string{"updated_at", "updatedAt", "created_at", "createdAt", "timestamp"},
		),
		GeneratedAt: time.Now().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(summary)
}

func (s *Server) handleRollbackStep(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req rollbackStepRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json body", http.StatusBadRequest)
		return
	}
	req.StepID = strings.TrimSpace(req.StepID)
	if req.StepID == "" {
		http.Error(w, "step_id is required", http.StatusBadRequest)
		return
	}

	result, err := corecap.NewVersioningManager(s.client).Rollback(req.StepID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if s.ws != nil {
		s.ws.BroadcastObservabilityEvent(s.ws.defaultLocale, "rollback_to_step", "completed", map[string]interface{}{
			"source":  "rest_api",
			"step_id": req.StepID,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":      true,
		"step_id": req.StepID,
		"result":  result,
	})
}

func (s *Server) handleVisualSelfTestSample(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	targetURL := buildDashboardURL(r)
	locale := "zh-CN"
	if s.ws != nil && strings.TrimSpace(s.ws.defaultLocale) != "" {
		locale = s.ws.defaultLocale
	}
	localizer := i18n.MustLocalizer(locale)
	taskLines := []string{
		localizer.T("server.visual_test.task.intro"),
		localizer.T("server.visual_test.task.open", targetURL),
		localizer.T("server.visual_test.task.click"),
		`   - [data-testid="open-trajectory"]`,
		`   - [data-testid="open-memory"]`,
		`   - [data-testid="open-visual-self-test"]`,
		localizer.T("server.visual_test.task.verify"),
		`   - [data-testid="trajectory-modal"]`,
		`   - [data-testid="trajectory-list"]`,
		`   - [data-testid="trajectory-detail"]`,
		localizer.T("server.visual_test.task.report"),
	}

	checklist := []map[string]string{
		{"label": localizer.T("server.visual_test.label.header"), "selector": `[data-testid="dashboard-header"]`},
		{"label": localizer.T("server.visual_test.label.trajectory_button"), "selector": `[data-testid="open-trajectory"]`},
		{"label": localizer.T("server.visual_test.label.memory_button"), "selector": `[data-testid="open-memory"]`},
		{"label": localizer.T("server.visual_test.label.visual_button"), "selector": `[data-testid="open-visual-self-test"]`},
		{"label": localizer.T("server.visual_test.label.trajectory_modal"), "selector": `[data-testid="trajectory-modal"]`},
		{"label": localizer.T("server.visual_test.label.trajectory_list"), "selector": `[data-testid="trajectory-list"]`},
		{"label": localizer.T("server.visual_test.label.trajectory_detail"), "selector": `[data-testid="trajectory-detail"]`},
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"title":     localizer.T("server.visual_test.title"),
		"url":       targetURL,
		"task":      strings.Join(taskLines, "\n"),
		"checklist": checklist,
	})
}

func buildDashboardURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}

	base := fmt.Sprintf("%s://%s/", scheme, r.Host)
	token := strings.TrimSpace(r.URL.Query().Get("token"))
	if token == "" {
		return base
	}

	values := url.Values{}
	values.Set("token", token)
	return base + "?" + values.Encode()
}
