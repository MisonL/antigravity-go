package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mison/antigravity-go/internal/config"
)

func TestParseStartupOptionsReadsRecoveryFlags(t *testing.T) {
	opts := parseStartupOptions([]string{"--safe-start", "--auto-repair", "--data", "/tmp/demo"})
	if !opts.SafeStart {
		t.Fatal("expected safe-start flag to be parsed")
	}
	if !opts.AutoRepair {
		t.Fatal("expected auto-repair flag to be parsed")
	}
	if opts.RequestedDataDir != "/tmp/demo" {
		t.Fatalf("unexpected data dir: %q", opts.RequestedDataDir)
	}
}

func TestPrepareStartupConfigRejectsBrokenConfigWithoutRecovery(t *testing.T) {
	dataDir := t.TempDir()
	configPath := config.ConfigPathForDataDir(dataDir)
	if err := os.WriteFile(configPath, []byte("provider: [broken"), 0644); err != nil {
		t.Fatalf("failed to seed config: %v", err)
	}

	_, _, err := prepareStartupConfig(startupOptions{RequestedDataDir: dataDir})
	if err == nil {
		t.Fatal("expected broken config to fail without recovery")
	}
	if !strings.Contains(err.Error(), "--safe-start") || !strings.Contains(err.Error(), "--auto-repair") {
		t.Fatalf("unexpected error guidance: %v", err)
	}
}

func TestPrepareStartupConfigUsesIsolatedDataDirInSafeMode(t *testing.T) {
	dataDir := t.TempDir()
	configPath := config.ConfigPathForDataDir(dataDir)
	if err := os.WriteFile(configPath, []byte("provider: [broken"), 0644); err != nil {
		t.Fatalf("failed to seed config: %v", err)
	}

	cfg, report, err := prepareStartupConfig(startupOptions{
		RequestedDataDir: dataDir,
		SafeStart:        true,
	})
	if err != nil {
		t.Fatalf("expected safe mode to recover: %v", err)
	}
	if report.Mode != "safe" {
		t.Fatalf("unexpected mode: %q", report.Mode)
	}
	if cfg.DataDir == dataDir {
		t.Fatalf("expected isolated safe data dir, got %q", cfg.DataDir)
	}
}

func TestPrepareStartupConfigRepairsBrokenConfig(t *testing.T) {
	dataDir := t.TempDir()
	configPath := config.ConfigPathForDataDir(dataDir)
	if err := os.WriteFile(configPath, []byte("provider: [broken"), 0644); err != nil {
		t.Fatalf("failed to seed config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "sessions"), []byte("occupied"), 0644); err != nil {
		t.Fatalf("failed to seed invalid sessions entry: %v", err)
	}

	cfg, report, err := prepareStartupConfig(startupOptions{
		RequestedDataDir: dataDir,
		AutoRepair:       true,
	})
	if err != nil {
		t.Fatalf("expected auto repair to succeed: %v", err)
	}
	if report.Mode != "repair" {
		t.Fatalf("unexpected mode: %q", report.Mode)
	}
	if cfg.DataDir != dataDir {
		t.Fatalf("expected repaired data dir %q, got %q", dataDir, cfg.DataDir)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "sessions")); err != nil {
		t.Fatalf("expected sessions directory after repair: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "executions")); err != nil {
		t.Fatalf("expected executions directory after repair: %v", err)
	}
	repairedCfg, err := config.LoadFromDataDir(dataDir)
	if err != nil {
		t.Fatalf("expected repaired config to be readable: %v", err)
	}
	if repairedCfg.DataDir != dataDir {
		t.Fatalf("unexpected repaired config data dir: %q", repairedCfg.DataDir)
	}
}
