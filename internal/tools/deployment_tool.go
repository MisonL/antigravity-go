package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mison/antigravity-go/internal/llm"
)

type deployProjectParams struct {
	Environment       string `json:"environment"`
	ImageRepository   string `json:"image_repository"`
	ImageTag          string `json:"image_tag"`
	PreviousImageRef  string `json:"previous_image_ref"`
	WriteArtifacts    *bool  `json:"write_artifacts"`
	WriteGitHubAction bool   `json:"write_github_action"`
}

type deployProjectResult struct {
	WorkspaceRoot  string   `json:"workspace_root"`
	ImageRef       string   `json:"image_ref"`
	Environment    string   `json:"environment"`
	Artifacts      []string `json:"artifacts"`
	BuildCommand   string   `json:"build_command"`
	PushCommand    string   `json:"push_command"`
	ComposeCommand string   `json:"compose_command"`
}

func NewDeployProjectTool() Tool {
	return Tool{
		Definition: llm.ToolDefinition{
			Name:        "deploy_project",
			Description: "Generate production deployment artifacts for the current workspace, including Dockerfile and docker-compose.yml.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"environment": map[string]interface{}{
						"type":        "string",
						"description": "Deployment environment label such as staging or production.",
					},
					"image_repository": map[string]interface{}{
						"type":        "string",
						"description": "Container image repository, for example ghcr.io/org/app.",
					},
					"image_tag": map[string]interface{}{
						"type":        "string",
						"description": "Optional image tag. Defaults to a UTC timestamp.",
					},
					"previous_image_ref": map[string]interface{}{
						"type":        "string",
						"description": "Optional previous image reference for rollback tracking.",
					},
					"write_artifacts": map[string]interface{}{
						"type":        "boolean",
						"description": "Whether to write Dockerfile, .dockerignore, and docker-compose.yml into the workspace. Defaults to true.",
					},
					"write_github_action": map[string]interface{}{
						"type":        "boolean",
						"description": "Whether to also write .github/workflows/deploy.yml.",
					},
				},
			},
		},
		Execute: func(ctx context.Context, args json.RawMessage) (string, error) {
			var params deployProjectParams
			if len(args) > 0 {
				if err := json.Unmarshal(args, &params); err != nil {
					return "", err
				}
			}

			root := WorkspaceRootFromContext(ctx, ".")
			plan, err := BuildDeploymentPlan(root, DeploymentPlanOptions{
				Environment:      params.Environment,
				ImageRepository:  params.ImageRepository,
				ImageTag:         params.ImageTag,
				PreviousImageRef: params.PreviousImageRef,
			})
			if err != nil {
				return "", err
			}

			writeArtifacts := true
			if params.WriteArtifacts != nil {
				writeArtifacts = *params.WriteArtifacts
			}
			if writeArtifacts {
				if err := WriteDockerArtifacts(root, plan); err != nil {
					return "", err
				}
				if params.WriteGitHubAction {
					if err := WriteGitHubAction(root, plan); err != nil {
						return "", err
					}
				}
			}

			artifacts := []string{plan.DockerfilePath, plan.DockerignorePath, plan.DockerComposePath}
			if params.WriteGitHubAction && strings.TrimSpace(plan.GitHubActionPath) != "" {
				artifacts = append(artifacts, plan.GitHubActionPath)
			}
			result := deployProjectResult{
				WorkspaceRoot:  plan.WorkspaceRoot,
				ImageRef:       plan.ImageRef,
				Environment:    plan.Environment,
				Artifacts:      artifacts,
				BuildCommand:   plan.BuildCommand,
				PushCommand:    plan.PushCommand,
				ComposeCommand: fmt.Sprintf("docker compose -f %s up -d --build", plan.DockerComposePath),
			}
			data, err := json.MarshalIndent(result, "", "  ")
			if err != nil {
				return "", err
			}
			return string(data), nil
		},
		RequiresPermission: true,
	}
}
