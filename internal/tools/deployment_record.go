package tools

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type DeploymentRecord struct {
	ID               string   `json:"id"`
	WorkspaceRoot    string   `json:"workspaceRoot"`
	Environment      string   `json:"environment"`
	ImageRepository  string   `json:"imageRepository"`
	ImageTag         string   `json:"imageTag"`
	ImageRef         string   `json:"imageRef"`
	PreviousImageRef string   `json:"previousImageRef,omitempty"`
	Status           string   `json:"status"`
	ReviewStatus     string   `json:"reviewStatus,omitempty"`
	ReviewReport     string   `json:"reviewReport,omitempty"`
	BuildCommand     string   `json:"buildCommand"`
	PushCommand      string   `json:"pushCommand"`
	Artifacts        []string `json:"artifacts"`
	FailureStage     string   `json:"failureStage,omitempty"`
	FailureReason    string   `json:"failureReason,omitempty"`
	CreatedAt        string   `json:"createdAt"`
	UpdatedAt        string   `json:"updatedAt"`
}

type DeploymentManager struct {
	recordsDir string
	runner     CommandRunner
}

func NewDeploymentManager(recordsDir string, runner CommandRunner) *DeploymentManager {
	if runner == nil {
		runner = runShellCommand
	}
	return &DeploymentManager{recordsDir: recordsDir, runner: runner}
}

func (m *DeploymentManager) Prepare(plan DeploymentPlan) (*DeploymentRecord, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	record := &DeploymentRecord{
		ID:               "deploy-" + time.Now().UTC().Format(defaultRecordTimeFormat),
		WorkspaceRoot:    plan.WorkspaceRoot,
		Environment:      plan.Environment,
		ImageRepository:  plan.ImageRepository,
		ImageTag:         plan.ImageTag,
		ImageRef:         plan.ImageRef,
		PreviousImageRef: plan.PreviousImageRef,
		Status:           deployStatusPrepared,
		BuildCommand:     plan.BuildCommand,
		PushCommand:      plan.PushCommand,
		Artifacts:        deploymentArtifacts(plan),
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	return record, m.save(record)
}

func deploymentArtifacts(plan DeploymentPlan) []string {
	artifacts := []string{plan.DockerfilePath, plan.DockerignorePath, plan.DockerComposePath}
	if strings.TrimSpace(plan.GitHubActionPath) != "" {
		artifacts = append(artifacts, plan.GitHubActionPath)
	}
	return artifacts
}

func (m *DeploymentManager) RecordReview(record *DeploymentRecord, report string, passed bool) error {
	record.ReviewReport = strings.TrimSpace(report)
	record.ReviewStatus = "fail"
	if passed {
		record.ReviewStatus = "pass"
		record.Status = deployStatusChecked
	}
	return m.save(record)
}

func (m *DeploymentManager) RunBuild(ctx context.Context, record *DeploymentRecord, plan DeploymentPlan, execute bool) (string, error) {
	output, err := m.run(ctx, plan.WorkspaceRoot, plan.BuildCommand, execute)
	if err != nil {
		return output, err
	}
	record.Status = deployStatusBuilt
	return output, m.save(record)
}

func (m *DeploymentManager) RunPush(ctx context.Context, record *DeploymentRecord, plan DeploymentPlan, execute bool) (string, error) {
	output, err := m.run(ctx, plan.WorkspaceRoot, plan.PushCommand, execute)
	if err != nil {
		return output, err
	}
	record.Status = deployStatusPushed
	return output, m.save(record)
}

func (m *DeploymentManager) MarkCommitted(record *DeploymentRecord) error {
	record.Status = deployStatusCommitted
	record.FailureStage = ""
	record.FailureReason = ""
	return m.save(record)
}

func (m *DeploymentManager) Rollback(record *DeploymentRecord, stage string, cause error) error {
	record.Status = deployStatusRollbackPending
	record.FailureStage = strings.TrimSpace(stage)
	record.FailureReason = strings.TrimSpace(cause.Error())
	if err := m.save(record); err != nil {
		return err
	}
	record.Status = deployStatusRolledBack
	return m.save(record)
}

func runShellCommand(ctx context.Context, cwd string, command string) (string, error) {
	cmd := exec.CommandContext(ctx, "/bin/bash", "-lc", command)
	cmd.Dir = cwd
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func (m *DeploymentManager) run(ctx context.Context, cwd string, command string, execute bool) (string, error) {
	if !execute {
		return "SIMULATED: " + command, nil
	}
	return m.runner(ctx, cwd, command)
}

func (m *DeploymentManager) save(record *DeploymentRecord) error {
	if err := os.MkdirAll(m.recordsDir, 0755); err != nil {
		return err
	}
	record.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	body, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(m.recordsDir, record.ID+".json"), body, 0644)
}
