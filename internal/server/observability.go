package server

import (
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
	if !requireMethod(w, r, http.MethodGet) {
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

	writeJSON(w, summary)
}

func (s *Server) handleRollbackStep(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	var req rollbackStepRequest
	if !decodeJSONBody(w, r, &req, "invalid json body") {
		return
	}
	stepID, ok := requireTrimmedValue(w, req.StepID, "step_id")
	if !ok {
		return
	}

	result, err := corecap.NewVersioningManager(s.client).Rollback(stepID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if s.ws != nil {
		s.ws.BroadcastObservabilityEvent(s.ws.defaultLocale, "rollback_to_step", "completed", map[string]interface{}{
			"source":  "rest_api",
			"step_id": stepID,
		})
	}

	writeJSON(w, map[string]interface{}{
		"ok":      true,
		"step_id": stepID,
		"result":  result,
	})
}

func (s *Server) handleVisualSelfTestSample(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	targetURL := buildDashboardURL(r)
	localizer := i18n.MustLocalizer(s.defaultLocale())
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

	writeJSON(w, map[string]interface{}{
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
	token, ok := requireDashboardToken(r)
	if !ok {
		return base
	}

	values := url.Values{}
	values.Set("token", token)
	return base + "?" + values.Encode()
}

func requireDashboardToken(r *http.Request) (string, bool) {
	token := strings.TrimSpace(r.URL.Query().Get("token"))
	return token, token != ""
}
