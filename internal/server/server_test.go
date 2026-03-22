package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mison/antigravity-go/internal/config"
)

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
