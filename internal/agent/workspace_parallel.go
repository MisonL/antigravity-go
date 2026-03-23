package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/mison/antigravity-go/internal/llm"
	"github.com/mison/antigravity-go/internal/tools"
)

type WorkspaceContext = tools.WorkspaceContext

func (a *Agent) bindWorkspaceContext(ctx context.Context) context.Context {
	workspace := a.WorkspaceContext()
	if strings.TrimSpace(workspace.Root) == "" && strings.TrimSpace(workspace.Label) == "" && len(workspace.Metadata) == 0 {
		return ctx
	}
	return tools.WithWorkspaceContext(ctx, workspace)
}

func WorkspaceContextFromContext(ctx context.Context) WorkspaceContext {
	return tools.WorkspaceContextFromContext(ctx)
}

type ParallelWorkerTask struct {
	ID        string           `json:"id,omitempty"`
	Label     string           `json:"label,omitempty"`
	Input     string           `json:"input"`
	Workspace WorkspaceContext `json:"workspace,omitempty"`
}

type ParallelWorkerResult struct {
	ID         string           `json:"id,omitempty"`
	Label      string           `json:"label,omitempty"`
	Input      string           `json:"input"`
	Output     string           `json:"output,omitempty"`
	Error      string           `json:"error,omitempty"`
	Workspace  WorkspaceContext `json:"workspace,omitempty"`
	TokenUsage int              `json:"token_usage"`
}

func (a *Agent) RunParallelWorkers(
	ctx context.Context,
	tasks []ParallelWorkerTask,
	localCallback ToolCallback,
) ([]ParallelWorkerResult, error) {
	if len(tasks) == 0 {
		return nil, fmt.Errorf("parallel worker tasks are required")
	}

	baseMessages := a.SnapshotMessages()
	prompt := a.GetSystemPrompt()
	results := make([]ParallelWorkerResult, len(tasks))

	var wg sync.WaitGroup
	for index, task := range tasks {
		wg.Add(1)
		go func(i int, workerTask ParallelWorkerTask) {
			defer wg.Done()

			worker := a.CloneWithPrompt(prompt)
			worker.LoadMessages(baseMessages)

			workspace := workerTask.Workspace
			if strings.TrimSpace(workspace.Root) == "" {
				workspace = a.WorkspaceContext()
			}
			worker.SetWorkspaceContext(workspace)

			output, err := worker.Run(worker.bindWorkspaceContext(ctx), workerTask.Input, localCallback)
			results[i] = ParallelWorkerResult{
				ID:         workerTask.ID,
				Label:      workerTask.Label,
				Input:      workerTask.Input,
				Output:     output,
				Workspace:  workspace.Clone(),
				TokenUsage: worker.GetTokenUsage(),
			}
			if err != nil {
				results[i].Error = err.Error()
			}
		}(index, task)
	}

	wg.Wait()
	return results, nil
}

func (a *Agent) GetParallelWorkerTool() tools.Tool {
	return tools.Tool{
		Definition: llm.ToolDefinition{
			Name:        "run_parallel_workers",
			Description: "Split the current task into multiple isolated workers and execute them concurrently. Each worker gets its own message history copy and workspace context.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"tasks": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"id":    map[string]interface{}{"type": "string"},
								"label": map[string]interface{}{"type": "string"},
								"input": map[string]interface{}{"type": "string"},
								"workspace": map[string]interface{}{
									"type": "object",
									"properties": map[string]interface{}{
										"root":  map[string]interface{}{"type": "string"},
										"label": map[string]interface{}{"type": "string"},
									},
								},
							},
							"required": []string{"input"},
						},
					},
				},
				"required": []string{"tasks"},
			},
		},
		Execute: func(ctx context.Context, args json.RawMessage) (string, error) {
			var params struct {
				Tasks []ParallelWorkerTask `json:"tasks"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", err
			}

			results, err := a.RunParallelWorkers(ctx, params.Tasks, nil)
			if err != nil {
				return "", err
			}
			data, err := json.MarshalIndent(map[string]any{"results": results}, "", "  ")
			if err != nil {
				return "", err
			}
			return string(data), nil
		},
		RequiresPermission: false,
	}
}
