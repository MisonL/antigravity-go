package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/mison/antigravity-go/internal/corecap"
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

	trajectoryPayload, err := corecap.NewTrajectoryManager(s.client).List()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	memoryPayload, err := corecap.NewMemoryManager(s.client).Query(nil)
	if err != nil {
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
		s.ws.BroadcastObservabilityEvent("rollback_to_step", "completed", map[string]interface{}{
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
	taskLines := []string{
		"请执行一次 Web 控制台视觉自测。",
		fmt.Sprintf("1. 使用 browser_open 打开 %s。", targetURL),
		"2. 使用 browser_click 依次验证这些元素可以被定位并交互：",
		`   - [data-testid="open-trajectory"]`,
		`   - [data-testid="open-memory"]`,
		`   - [data-testid="open-visual-self-test"]`,
		"3. 打开轨迹树后，确认以下核心组件存在，再执行 browser_screenshot 记录结果：",
		`   - [data-testid="trajectory-modal"]`,
		`   - [data-testid="trajectory-list"]`,
		`   - [data-testid="trajectory-detail"]`,
		"4. 输出检查结论，若任一元素无法定位或交互失败，明确报告失败点。",
	}

	checklist := []map[string]string{
		{"label": "控制台头部", "selector": `[data-testid="dashboard-header"]`},
		{"label": "轨迹按钮", "selector": `[data-testid="open-trajectory"]`},
		{"label": "记忆按钮", "selector": `[data-testid="open-memory"]`},
		{"label": "视觉自测按钮", "selector": `[data-testid="open-visual-self-test"]`},
		{"label": "轨迹弹窗", "selector": `[data-testid="trajectory-modal"]`},
		{"label": "轨迹列表", "selector": `[data-testid="trajectory-list"]`},
		{"label": "轨迹详情", "selector": `[data-testid="trajectory-detail"]`},
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"title":     "Web 控制台视觉自测原型",
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
