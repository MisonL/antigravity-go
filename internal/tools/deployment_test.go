package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildDeploymentPlanForGoFrontendWorkspace(t *testing.T) {
	workspace := t.TempDir()
	writeTestFile(t, filepath.Join(workspace, "go.mod"), "module example.com/app\n\ngo 1.24.0\n")
	writeTestFile(t, filepath.Join(workspace, "go.sum"), "")
	writeTestFile(t, filepath.Join(workspace, "cmd", "ago", "main.go"), "package main\nfunc main() {}\n")
	writeTestFile(t, filepath.Join(workspace, "frontend", "package.json"), `{"scripts":{"build":"vite build"}}`)
	writeTestFile(t, filepath.Join(workspace, "frontend", "bun.lock"), "lock")

	plan, err := BuildDeploymentPlan(workspace, DeploymentPlanOptions{
		Environment:     "production",
		ImageRepository: "Registry/Antigravity-Go",
		ImageTag:        "v1.2.3",
	})
	if err != nil {
		t.Fatalf("BuildDeploymentPlan returned error: %v", err)
	}

	assertContains(t, plan.DockerfileContent, "FROM oven/bun:1.3.6 AS frontend")
	assertContains(t, plan.DockerfileContent, "COPY frontend/package.json frontend/bun.lock ./")
	assertContains(t, plan.DockerfileContent, "COPY --from=frontend /src/frontend/dist /tmp/frontend-dist")
	assertContains(t, plan.DockerfileContent, "go build -trimpath -ldflags='-s -w' -o /out/ago ./cmd/ago")
	assertContains(t, plan.DockerignoreBody, ".go-cache")
	assertContains(t, plan.DockerignoreBody, "docs/reviews")
	assertContains(t, plan.DockerComposeBody, "image: registry/antigravity-go:v1.2.3")
	assertContains(t, plan.DockerComposeBody, "./deploy/runtime/antigravity_core:/opt/antigravity/bin/antigravity_core:ro")
	assertContains(t, plan.DockerComposeBody, "curl -fsS -H \"Authorization: Bearer $$AGO_WEB_TOKEN\"")
	assertContains(t, plan.DockerComposeBody, "    command:\n      - --web")
	assertNotContains(t, plan.DockerComposeBody, "    command:\n      - ago")
	assertContains(t, plan.GitHubActionBody, "docker/build-push-action@v6")
	assertContains(t, plan.GitHubActionBody, "registry/antigravity-go:${{ github.sha }}")
	if plan.GitHubActionPath != ".github/workflows/deploy.yml" {
		t.Fatalf("unexpected workflow path: %q", plan.GitHubActionPath)
	}

	if plan.ImageRepository != "registry/antigravity-go" {
		t.Fatalf("unexpected image repository: %q", plan.ImageRepository)
	}
	if plan.ImageRef != "registry/antigravity-go:v1.2.3" {
		t.Fatalf("unexpected image ref: %q", plan.ImageRef)
	}
	if plan.DockerComposePath != "docker-compose.yml" {
		t.Fatalf("unexpected compose path: %q", plan.DockerComposePath)
	}
}

func TestBuildDeploymentPlanForGoOnlyWorkspace(t *testing.T) {
	workspace := t.TempDir()
	writeTestFile(t, filepath.Join(workspace, "go.mod"), "module example.com/service\n\ngo 1.23.5\n")
	writeTestFile(t, filepath.Join(workspace, "go.sum"), "")
	writeTestFile(t, filepath.Join(workspace, "cmd", "service", "main.go"), "package main\nfunc main() {}\n")

	plan, err := BuildDeploymentPlan(workspace, DeploymentPlanOptions{})
	if err != nil {
		t.Fatalf("BuildDeploymentPlan returned error: %v", err)
	}

	if strings.Contains(plan.DockerfileContent, "AS frontend") {
		t.Fatal("did not expect frontend stage for go-only workspace")
	}
	assertContains(t, plan.DockerfileContent, "FROM golang:1.23.5 AS builder")
	assertContains(t, plan.DockerfileContent, "go build -trimpath -ldflags='-s -w' -o /out/service ./cmd/service")
	assertContains(t, plan.DockerfileContent, "apt-get install -y --no-install-recommends bash ca-certificates curl git ripgrep")
	assertContains(t, plan.DockerComposeBody, "image: ")
}

func TestWriteDockerArtifacts(t *testing.T) {
	workspace := t.TempDir()
	plan := DeploymentPlan{
		DockerfilePath:    "Dockerfile",
		DockerfileContent: "FROM scratch\n",
		DockerignorePath:  ".dockerignore",
		DockerignoreBody:  ".git\n",
		DockerComposePath: "docker-compose.yml",
		DockerComposeBody: "services:\n  app: {}\n",
		GitHubActionPath:  ".github/workflows/deploy.yml",
		GitHubActionBody:  "name: Deploy\n",
	}

	if err := WriteDeploymentArtifacts(workspace, plan); err != nil {
		t.Fatalf("WriteDeploymentArtifacts returned error: %v", err)
	}

	checkFileContent(t, filepath.Join(workspace, "Dockerfile"), "FROM scratch\n")
	checkFileContent(t, filepath.Join(workspace, ".dockerignore"), ".git\n")
	checkFileContent(t, filepath.Join(workspace, "docker-compose.yml"), "services:\n  app: {}\n")
	checkFileContent(t, filepath.Join(workspace, ".github", "workflows", "deploy.yml"), "name: Deploy\n")
}

func writeTestFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("failed to create parent dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
}

func checkFileContent(t *testing.T, path string, want string) {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read %s: %v", path, err)
	}
	if string(body) != want {
		t.Fatalf("unexpected content in %s: %q", path, string(body))
	}
}

func assertContains(t *testing.T, text string, want string) {
	t.Helper()
	if !strings.Contains(text, want) {
		t.Fatalf("expected %q to contain %q", text, want)
	}
}

func assertNotContains(t *testing.T, text string, want string) {
	t.Helper()
	if strings.Contains(text, want) {
		t.Fatalf("expected %q not to contain %q", text, want)
	}
}
