package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mison/antigravity-go/internal/llm"
	"github.com/mison/antigravity-go/internal/tools"
)

type specialistProfile struct {
	Prompt string
}

type ReviewerAssessmentInput struct {
	ToolName         string
	ToolArgs         string
	ToolResult       string
	ValidationResult string
	TestCommand      string
	TestOutput       string
	Passed           bool
}

var specialistProfiles = map[string]specialistProfile{
	"reviewer": {
		Prompt: `你是内部 ReviewerAgent，负责在代码进入人工审批前做严格预审。
优先关注：
1. 编译、测试、校验是否失败。
2. 改动是否引入明显逻辑缺陷或边界问题。
3. 如果失败，输出明确的失败原因，便于 Coder 立即重试。
4. 如果通过，也要给出一句简短结论。

输出要求：
- 先给结论：PASS 或 FAIL。
- 然后给最多 3 条关键发现。
- 不要要求人工介入，不要输出空泛建议。`,
	},
	"architect": {
		Prompt: `你是资深架构师。请从架构一致性、可维护性、扩展性和集成风险角度评估给定方案，并给出高层反馈。`,
	},
	"security": {
		Prompt: `你是安全审计专家。请重点检查输入校验、权限边界、敏感信息泄漏和常见漏洞风险，并给出明确风险判断。`,
	},
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
	status := "PASS"
	if !input.Passed {
		status = "FAIL"
	}

	task := strings.Join([]string{
		"请基于以下自动化审计证据给出最终预审结论。",
		"结论基线: " + status,
		"工具: " + input.ToolName,
		"工具参数:",
		truncateSpecialistText(input.ToolArgs, 2000),
		"工具结果:",
		truncateSpecialistText(input.ToolResult, 3000),
		"校验结果:",
		truncateSpecialistText(input.ValidationResult, 3000),
		"测试命令: " + input.TestCommand,
		"测试输出:",
		truncateSpecialistText(input.TestOutput, 3000),
	}, "\n")

	result, err := a.runSpecialist(ctx, "reviewer", task, "")
	if err != nil {
		if input.Passed {
			return "PASS\n- 自动预审通过，未能生成额外 Reviewer 摘要。"
		}
		return "FAIL\n- 自动预审失败，未能生成额外 Reviewer 摘要。"
	}
	return result
}

func (a *Agent) runSpecialist(ctx context.Context, role string, task string, extraContext string) (string, error) {
	profile, exists := specialistProfiles[role]
	if !exists {
		return "", fmt.Errorf("unknown specialist role %q", role)
	}

	if err := a.ensureProvider(); err != nil {
		return "", err
	}

	prompt := profile.Prompt
	if extraContext != "" {
		prompt += "\n\n补充上下文:\n" + extraContext
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
