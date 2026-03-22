package core

import (
	"testing"
	"time"
)

func TestHostConfig(t *testing.T) {
	cfg := Config{
		BinPath: "dummy_path",
		DataDir: "dummy_data_dir",
	}

	if cfg.BinPath != "dummy_path" {
		t.Errorf("expected dummy_path, got %s", cfg.BinPath)
	}
}

func TestHostWaitReadyTimeout(t *testing.T) {
	h := NewHost(Config{
		BinPath: "dummy_bin",
		DataDir: "dummy_data",
	})

	// WaitReady should timeout quickly if port is not open
	err := h.WaitReady(10 * time.Millisecond)
	if err == nil {
		t.Error("expected timeout error, got nil")
	}
}

func TestHostStopWithoutStart(t *testing.T) {
	h := NewHost(Config{
		BinPath: "dummy_bin",
		DataDir: "dummy_data",
	})

	// Stopping an unstarted host should not panic
	err := h.Stop()
	if err != nil {
		t.Errorf("expected no error when stopping unstarted host, got %v", err)
	}
}
