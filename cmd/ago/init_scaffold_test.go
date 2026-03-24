package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestNewInitScaffoldRejectsNonEmptyDirectory(t *testing.T) {
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "keep.txt"), []byte("busy"), 0644); err != nil {
		t.Fatalf("failed to seed directory: %v", err)
	}

	if _, err := newInitScaffold(workspace, ""); err == nil {
		t.Fatal("expected non-empty directory to be rejected")
	}
}

func TestInitScaffoldWriteCreatesProjectThatBuilds(t *testing.T) {
	workspace := t.TempDir()

	scaffold, err := newInitScaffold(workspace, "example.com/demo/app")
	if err != nil {
		t.Fatalf("newInitScaffold returned error: %v", err)
	}

	paths, err := scaffold.write()
	if err != nil {
		t.Fatalf("write returned error: %v", err)
	}
	if len(paths) == 0 {
		t.Fatal("expected scaffold to create files")
	}

	required := []string{
		"go.mod",
		"Makefile",
		"README.md",
		"cmd/server/main.go",
		"internal/backend/server.go",
		"frontend/package.json",
		"frontend/src/App.tsx",
	}
	for _, relPath := range required {
		if _, err := os.Stat(filepath.Join(workspace, relPath)); err != nil {
			t.Fatalf("expected %s to exist: %v", relPath, err)
		}
	}

	cmd := exec.Command("go", "build", "./...")
	cmd.Dir = workspace
	cmd.Env = append(os.Environ(), "GOCACHE="+filepath.Join(workspace, ".go-cache"))
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build failed: %v\n%s", err, string(output))
	}
}
