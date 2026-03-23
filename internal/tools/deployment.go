package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	deployStatusPrepared        = "prepared"
	deployStatusChecked         = "checked"
	deployStatusBuilt           = "built"
	deployStatusPushed          = "pushed"
	deployStatusCommitted       = "committed"
	deployStatusRollbackPending = "rollback_pending"
	deployStatusRolledBack      = "rolled_back"
	defaultDeployContext        = "."
	defaultGoVersion            = "1.24"
	defaultRuntimeWorkdir       = "/workspace"
	defaultRecordTimeFormat     = "20060102-150405"
)

var imageNameSanitizer = regexp.MustCompile(`[^a-z0-9._/-]+`)

type CommandRunner func(ctx context.Context, cwd string, command string) (string, error)

type DeploymentPlanOptions struct {
	Environment      string
	ImageRepository  string
	ImageTag         string
	PreviousImageRef string
}

type WorkspaceProfile struct {
	Root          string
	GoVersion     string
	GoMainPackage string
	BinaryName    string
	HasFrontend   bool
	UsesBun       bool
	BunLockFile   string
}

type DockerArtifacts struct {
	DockerfilePath      string
	DockerfileContent   string
	DockerignorePath    string
	DockerignoreContent string
	DockerComposePath   string
	DockerComposeBody   string
}

type GitHubActionArtifact struct {
	WorkflowPath    string
	WorkflowContent string
}

type DeploymentPlan struct {
	WorkspaceRoot     string
	Environment       string
	ImageRepository   string
	ImageTag          string
	ImageRef          string
	PreviousImageRef  string
	BuildCommand      string
	PushCommand       string
	Profile           WorkspaceProfile
	DockerfilePath    string
	DockerfileContent string
	DockerignorePath  string
	DockerignoreBody  string
	DockerComposePath string
	DockerComposeBody string
	GitHubActionPath  string
	GitHubActionBody  string
}

func BuildDeploymentPlan(root string, opts DeploymentPlanOptions) (DeploymentPlan, error) {
	profile, err := ScanDeploymentWorkspace(root)
	if err != nil {
		return DeploymentPlan{}, err
	}
	artifacts, err := GenerateDockerArtifacts(profile)
	if err != nil {
		return DeploymentPlan{}, err
	}
	repository := defaultImageRepository(root, opts.ImageRepository)
	tag := defaultImageTag(opts.ImageTag)
	imageRef := repository + ":" + tag
	plan := DeploymentPlan{
		WorkspaceRoot:     profile.Root,
		Environment:       defaultEnvironment(opts.Environment),
		ImageRepository:   repository,
		ImageTag:          tag,
		ImageRef:          imageRef,
		PreviousImageRef:  strings.TrimSpace(opts.PreviousImageRef),
		BuildCommand:      fmt.Sprintf("docker build -f %s -t %s %s", artifacts.DockerfilePath, imageRef, defaultDeployContext),
		PushCommand:       fmt.Sprintf("docker push %s", imageRef),
		Profile:           profile,
		DockerfilePath:    artifacts.DockerfilePath,
		DockerfileContent: artifacts.DockerfileContent,
		DockerignorePath:  artifacts.DockerignorePath,
		DockerignoreBody:  artifacts.DockerignoreContent,
		DockerComposePath: artifacts.DockerComposePath,
	}
	plan.DockerComposeBody = dockerComposeTemplate(plan)
	workflow, err := GenerateGitHubAction(plan)
	if err != nil {
		return DeploymentPlan{}, err
	}
	plan.GitHubActionPath = workflow.WorkflowPath
	plan.GitHubActionBody = workflow.WorkflowContent
	return plan, nil
}

func ScanDeploymentWorkspace(root string) (WorkspaceProfile, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return WorkspaceProfile{}, err
	}
	goModPath := filepath.Join(absRoot, "go.mod")
	goVersion, err := readGoVersion(goModPath)
	if err != nil {
		return WorkspaceProfile{}, err
	}
	mainPkg, binaryName, err := detectMainPackage(absRoot)
	if err != nil {
		return WorkspaceProfile{}, err
	}
	return WorkspaceProfile{
		Root:          absRoot,
		GoVersion:     goVersion,
		GoMainPackage: mainPkg,
		BinaryName:    binaryName,
		HasFrontend:   fileExists(filepath.Join(absRoot, "frontend", "package.json")),
		UsesBun:       detectBunLockFile(absRoot) != "",
		BunLockFile:   detectBunLockFile(absRoot),
	}, nil
}

func GenerateDockerArtifacts(profile WorkspaceProfile) (DockerArtifacts, error) {
	if profile.GoMainPackage == "" || profile.BinaryName == "" {
		return DockerArtifacts{}, fmt.Errorf("部署扫描结果不完整")
	}
	return DockerArtifacts{
		DockerfilePath:      "Dockerfile",
		DockerfileContent:   dockerfileTemplate(profile),
		DockerignorePath:    ".dockerignore",
		DockerignoreContent: dockerignoreTemplate(),
		DockerComposePath:   "docker-compose.yml",
	}, nil
}

func WriteDockerArtifacts(root string, plan DeploymentPlan) error {
	if err := writeArtifact(filepath.Join(root, plan.DockerfilePath), plan.DockerfileContent); err != nil {
		return err
	}
	if err := writeArtifact(filepath.Join(root, plan.DockerignorePath), plan.DockerignoreBody); err != nil {
		return err
	}
	return writeArtifact(filepath.Join(root, plan.DockerComposePath), plan.DockerComposeBody)
}

func GenerateGitHubAction(plan DeploymentPlan) (GitHubActionArtifact, error) {
	if strings.TrimSpace(plan.ImageRepository) == "" {
		return GitHubActionArtifact{}, fmt.Errorf("image repository is required to generate GitHub Actions workflow")
	}
	return GitHubActionArtifact{
		WorkflowPath:    filepath.ToSlash(filepath.Join(".github", "workflows", "deploy.yml")),
		WorkflowContent: githubActionTemplate(plan),
	}, nil
}

func WriteGitHubAction(root string, plan DeploymentPlan) error {
	if strings.TrimSpace(plan.GitHubActionPath) == "" || strings.TrimSpace(plan.GitHubActionBody) == "" {
		return nil
	}
	return writeArtifact(filepath.Join(root, plan.GitHubActionPath), plan.GitHubActionBody)
}

func WriteDeploymentArtifacts(root string, plan DeploymentPlan) error {
	if err := WriteDockerArtifacts(root, plan); err != nil {
		return err
	}
	return WriteGitHubAction(root, plan)
}

func readGoVersion(goModPath string) (string, error) {
	body, err := os.ReadFile(goModPath)
	if err != nil {
		return "", fmt.Errorf("读取 go.mod 失败: %w", err)
	}
	for _, line := range strings.Split(string(body), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "go ") {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, "go ")), nil
		}
	}
	return defaultGoVersion, nil
}

func detectMainPackage(root string) (string, string, error) {
	preferred := filepath.Join(root, "cmd", "agy")
	if directoryHasMainPackage(preferred) {
		return "./cmd/agy", "agy", nil
	}
	cmdRoot := filepath.Join(root, "cmd")
	entries, err := os.ReadDir(cmdRoot)
	if err != nil {
		return "", "", fmt.Errorf("读取 cmd 目录失败: %w", err)
	}
	var matches []string
	for _, entry := range entries {
		if entry.IsDir() && directoryHasMainPackage(filepath.Join(cmdRoot, entry.Name())) {
			matches = append(matches, entry.Name())
		}
	}
	sort.Strings(matches)
	if len(matches) == 0 {
		return "", "", fmt.Errorf("未找到可部署的 main 包")
	}
	return "./cmd/" + matches[0], matches[0], nil
}

func directoryHasMainPackage(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		body, readErr := os.ReadFile(filepath.Join(dir, entry.Name()))
		if readErr == nil && strings.Contains(string(body), "package main") {
			return true
		}
	}
	return false
}

func writeArtifact(path string, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0644)
}

func defaultEnvironment(value string) string {
	if strings.TrimSpace(value) == "" {
		return "staging"
	}
	return strings.TrimSpace(value)
}

func defaultImageRepository(root string, value string) string {
	raw := strings.TrimSpace(value)
	if raw == "" {
		raw = filepath.Base(root)
	}
	sanitized := imageNameSanitizer.ReplaceAllString(strings.ToLower(raw), "-")
	sanitized = strings.Trim(sanitized, "-./")
	if sanitized == "" {
		return "app"
	}
	return sanitized
}

func defaultImageTag(value string) string {
	if strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	return time.Now().UTC().Format(defaultRecordTimeFormat)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func detectBunLockFile(root string) string {
	frontendRoot := filepath.Join(root, "frontend")
	for _, name := range []string{"bun.lockb", "bun.lock"} {
		if fileExists(filepath.Join(frontendRoot, name)) {
			return name
		}
	}
	return ""
}
