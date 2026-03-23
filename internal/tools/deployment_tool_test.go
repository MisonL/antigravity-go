package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDeployProjectToolWritesDeploymentArtifacts(t *testing.T) {
	workspace := t.TempDir()
	writeTestFile(t, filepath.Join(workspace, "go.mod"), "module example.com/app\n\ngo 1.24.0\n")
	writeTestFile(t, filepath.Join(workspace, "go.sum"), "")
	writeTestFile(t, filepath.Join(workspace, "cmd", "agy", "main.go"), "package main\nfunc main() {}\n")
	writeTestFile(t, filepath.Join(workspace, "frontend", "package.json"), `{"scripts":{"build":"vite build"}}`)
	writeTestFile(t, filepath.Join(workspace, "frontend", "package-lock.json"), "{}")

	tool := NewDeployProjectTool()
	ctx := WithWorkspaceContext(context.Background(), WorkspaceContext{Root: workspace})
	raw, err := tool.Execute(ctx, []byte(`{"environment":"production","image_repository":"ghcr.io/acme/agy","image_tag":"v1.5.0"}`))
	if err != nil {
		t.Fatalf("tool returned error: %v", err)
	}

	var result deployProjectResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("failed to decode tool result: %v", err)
	}
	if result.ImageRef != "ghcr.io/acme/agy:v1.5.0" {
		t.Fatalf("unexpected image ref: %q", result.ImageRef)
	}
	if !strings.Contains(result.ComposeCommand, "docker compose -f docker-compose.yml up -d --build") {
		t.Fatalf("unexpected compose command: %q", result.ComposeCommand)
	}

	for _, relPath := range []string{"Dockerfile", ".dockerignore", "docker-compose.yml"} {
		if _, err := os.Stat(filepath.Join(workspace, relPath)); err != nil {
			t.Fatalf("expected %s to be written: %v", relPath, err)
		}
	}
	if _, err := os.Stat(filepath.Join(workspace, ".github", "workflows", "deploy.yml")); !os.IsNotExist(err) {
		t.Fatalf("did not expect workflow file by default, err=%v", err)
	}
}

func TestDeployProjectToolCanWriteWorkflowWhenRequested(t *testing.T) {
	workspace := t.TempDir()
	writeTestFile(t, filepath.Join(workspace, "go.mod"), "module example.com/app\n\ngo 1.24.0\n")
	writeTestFile(t, filepath.Join(workspace, "go.sum"), "")
	writeTestFile(t, filepath.Join(workspace, "cmd", "agy", "main.go"), "package main\nfunc main() {}\n")

	tool := NewDeployProjectTool()
	ctx := WithWorkspaceContext(context.Background(), WorkspaceContext{Root: workspace})
	if _, err := tool.Execute(ctx, []byte(`{"write_github_action":true}`)); err != nil {
		t.Fatalf("tool returned error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(workspace, ".github", "workflows", "deploy.yml")); err != nil {
		t.Fatalf("expected workflow file to be written: %v", err)
	}
}
