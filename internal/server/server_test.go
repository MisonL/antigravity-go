package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mison/antigravity-go/internal/config"
)

type fakeServerTrajectoryGetter struct {
	lastID  string
	payload map[string]interface{}
	err     error
}

func (f *fakeServerTrajectoryGetter) Get(id string) (map[string]interface{}, error) {
	f.lastID = id
	if f.err != nil {
		return nil, f.err
	}
	return f.payload, nil
}

func TestHandleConfigSavesToServerDataDirAndMasksAPIKey(t *testing.T) {
	tempDir := t.TempDir()
	srv := &Server{
		cfg: config.Config{
			Provider:  "openai",
			Model:     "gpt-5.4",
			BaseURL:   "https://api.openai.com/v1",
			APIKey:    "old-secret-key",
			DataDir:   tempDir,
			Approvals: "prompt",
		},
	}

	postReq := httptest.NewRequest(http.MethodPost, "/api/config", strings.NewReader(`{
		"provider":"gemini",
		"model":"gemini-2.5-pro",
		"base_url":"https://proxy.example/v1beta",
		"api_key":"new-secret-key"
	}`))
	postReq.Header.Set("Content-Type", "application/json")
	postResp := httptest.NewRecorder()

	srv.handleConfig(postResp, postReq)

	if postResp.Code != http.StatusOK {
		t.Fatalf("unexpected POST status: %d", postResp.Code)
	}

	savedCfg, err := config.LoadFromDataDir(tempDir)
	if err != nil {
		t.Fatalf("failed to load saved config: %v", err)
	}

	if savedCfg.Provider != "gemini" {
		t.Fatalf("unexpected provider: %q", savedCfg.Provider)
	}
	if savedCfg.Model != "gemini-2.5-pro" {
		t.Fatalf("unexpected model: %q", savedCfg.Model)
	}
	if savedCfg.BaseURL != "https://proxy.example/v1beta" {
		t.Fatalf("unexpected base URL: %q", savedCfg.BaseURL)
	}
	if savedCfg.APIKey != "new-secret-key" {
		t.Fatalf("unexpected API key: %q", savedCfg.APIKey)
	}

	configPath := filepath.Join(tempDir, "config.yaml")
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("expected config file at %s: %v", configPath, err)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	getResp := httptest.NewRecorder()
	srv.handleConfig(getResp, getReq)

	if getResp.Code != http.StatusOK {
		t.Fatalf("unexpected GET status: %d", getResp.Code)
	}

	var payload map[string]any
	if err := json.Unmarshal(getResp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode GET payload: %v", err)
	}

	if payload["provider"] != "gemini" {
		t.Fatalf("unexpected provider in GET payload: %#v", payload["provider"])
	}
	if payload["model"] != "gemini-2.5-pro" {
		t.Fatalf("unexpected model in GET payload: %#v", payload["model"])
	}
	if payload["base_url"] != "https://proxy.example/v1beta" {
		t.Fatalf("unexpected base_url in GET payload: %#v", payload["base_url"])
	}
	if payload["api_key"] != "new-...-key" {
		t.Fatalf("unexpected masked api_key: %#v", payload["api_key"])
	}
}

func TestHandleSessionResumeReturnsWebsocketAndHistory(t *testing.T) {
	getter := &fakeServerTrajectoryGetter{
		payload: map[string]interface{}{
			"trajectory": map[string]interface{}{
				"workspace_root": "/tmp/resume-workspace",
				"messages": []interface{}{
					map[string]interface{}{"role": "user", "content": "继续这个任务"},
					map[string]interface{}{"role": "assistant", "content": "已恢复上下文"},
				},
			},
		},
	}

	srv := &Server{
		authToken:     "test-token",
		workspaceRoot: "/tmp/fallback",
		trajectory:    getter,
	}

	req := httptest.NewRequest(http.MethodPost, "http://example.com/api/sessions/resume", strings.NewReader(`{"trajectory_id":"traj-42"}`))
	resp := httptest.NewRecorder()

	srv.handleSessionResume(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d, body=%s", resp.Code, resp.Body.String())
	}
	if getter.lastID != "traj-42" {
		t.Fatalf("unexpected trajectory id: %q", getter.lastID)
	}

	var payload map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if payload["trajectory_id"] != "traj-42" {
		t.Fatalf("unexpected trajectory_id: %#v", payload["trajectory_id"])
	}
	if payload["workspace_root"] != "/tmp/resume-workspace" {
		t.Fatalf("unexpected workspace_root: %#v", payload["workspace_root"])
	}
	if payload["redirect_url"] != "http://example.com/?resume_trajectory=traj-42&token=test-token" {
		t.Fatalf("unexpected redirect_url: %#v", payload["redirect_url"])
	}
	if payload["websocket_url"] != "ws://example.com/ws?resume_trajectory=traj-42&token=test-token" {
		t.Fatalf("unexpected websocket_url: %#v", payload["websocket_url"])
	}
	messages, ok := payload["messages"].([]any)
	if !ok || len(messages) != 2 {
		t.Fatalf("unexpected messages payload: %#v", payload["messages"])
	}
}

func TestHandleSessionResumeRejectsInvalidTrajectory(t *testing.T) {
	srv := &Server{
		trajectory: &fakeServerTrajectoryGetter{err: fmt.Errorf("trajectory missing")},
	}

	req := httptest.NewRequest(http.MethodPost, "http://example.com/api/sessions/resume", strings.NewReader(`{"trajectory_id":"missing"}`))
	resp := httptest.NewRecorder()

	srv.handleSessionResume(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("unexpected status: %d", resp.Code)
	}
}
