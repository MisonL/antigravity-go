package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mison/antigravity-go/internal/llm"
	"github.com/mison/antigravity-go/internal/pkg/i18n"
	"github.com/mison/antigravity-go/internal/tools"
)

type ReviewerAssessmentInput struct {
	ToolName         string
	ToolArgs         string
	ToolResult       string
	ValidationResult string
	TestCommand      string
	TestOutput       string
	Passed           bool
}


func (a *Agent) GetSpecialistTool() tools.Tool {
	return tools.Tool{
		Definition: llm.ToolDefinition{
			Name:        "ask_specialist",
			Description: "Delegate a specific task to a specialist agent (Reviewer, Architect, Security).",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"role": map[string]interface{}{
						"type":        "string",
						"description": "The specialist role: 'reviewer', 'architect', or 'security'.",
						"enum":        []string{"reviewer", "architect", "security"},
					},
					"task": map[string]interface{}{
						"type":        "string",
						"description": "The specific task or code to review/analyze.",
					},
					"context": map[string]interface{}{
						"type":        "string",
						"description": "Additional context or background information.",
					},
				},
				"required": []string{"role", "task"},
			},
		},
		Execute: func(ctx context.Context, args json.RawMessage) (string, error) {
			var params struct {
				Role    string `json:"role"`
				Task    string `json:"task"`
				Context string `json:"context"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", err
			}

			result, err := a.runSpecialist(ctx, strings.ToLower(params.Role), params.Task, params.Context)
			if err != nil {
				return fmt.Sprintf("Specialist (%s) failed: %v", params.Role, err), nil
			}
			return fmt.Sprintf("### Specialist (%s) Report ###\n\n%s", params.Role, result), nil
		},
		RequiresPermission: false, // Internal delegation
	}
}

func (a *Agent) RunReviewerAssessment(ctx context.Context, input ReviewerAssessmentInput) string {
	localizer := i18n.MustLocalizer(a.Locale())
	status := "PASS"
	if !input.Passed {
		status = "FAIL"
	}

	task := strings.Join([]string{
		localizer.T("agent.reviewer.task.header"),
		localizer.T("agent.reviewer.task.status", status),
		localizer.T("agent.reviewer.task.tool", input.ToolName),
		localizer.T("agent.reviewer.task.tool_args"),
		truncateSpecialistText(input.ToolArgs, 2000),
		localizer.T("agent.reviewer.task.tool_result"),
		truncateSpecialistText(input.ToolResult, 3000),
		localizer.T("agent.reviewer.task.validation"),
		truncateSpecialistText(input.ValidationResult, 3000),
		localizer.T("agent.reviewer.task.test_command", input.TestCommand),
		localizer.T("agent.reviewer.task.test_output"),
		truncateSpecialistText(input.TestOutput, 3000),
	}, "\n")

	result, err := a.runSpecialist(ctx, "reviewer", task, "")
	if err != nil {
		if input.Passed {
			return localizer.T("agent.reviewer.default_pass")
		}
		return localizer.T("agent.reviewer.default_fail")
	}
	return result
}

func (a *Agent) runSpecialist(ctx context.Context, role string, task string, extraContext string) (string, error) {
	prompt, exists := SpecialistPrompt(a.Locale(), role)
	if !exists {
		return "", fmt.Errorf("unknown specialist role %q", role)
	}

	if err := a.ensureProvider(); err != nil {
		return "", err
	}

	if extraContext != "" {
		prompt += "\n\nAdditional context:\n" + extraContext
	}

	resp, err := a.provider.Chat(ctx, []llm.Message{
		{Role: llm.RoleSystem, Content: prompt},
		{Role: llm.RoleUser, Content: task},
	}, nil)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(resp.Content), nil
}

func truncateSpecialistText(s string, limit int) string {
	s = strings.TrimSpace(s)
	if limit <= 0 || len(s) <= limit {
		return s
	}
	if limit <= 3 {
		return s[:limit]
	}
	return s[:limit-3] + "..."
}
