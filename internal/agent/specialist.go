package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mison/antigravity-go/internal/llm"
	"github.com/mison/antigravity-go/internal/tools"
)

var specialistPrompts = map[string]string{
	"reviewer": `You are a world-class senior code reviewer. 
Your goal is to perform a deep analysis of the provided code or task. 
Look for:
1. Logical bugs or edge cases.
2. Performance bottlenecks.
3. Code smell and readability issues.
4. Security vulnerabilities.

Be extremely critical but constructive. Provide a summary of your findings and specific recommendations.`,

	"architect": `You are a senior system architect. 
Your goal is to evaluate the proposed changes or design from a high-level perspective. 
Consider:
1. Architectural consistency.
2. Scalability and maintainability.
3. Integration with existing systems.
4. Design patterns and best practices.

Provide a high-level assessment and feedback on the strategy.`,

	"security": `You are an expert security auditor. 
Your goal is to find security flaws in the code or design. 
Focus on:
1. Data validation and sanitization.
2. Authentication and authorization flaws.
3. Common vulnerabilities (OWASP Top 10).
4. Information leaks.

Be thorough and provide clear risk assessments.`,
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

			prompt, exists := specialistPrompts[strings.ToLower(params.Role)]
			if !exists {
				return fmt.Sprintf("Error: Unknown specialist role '%s'", params.Role), nil
			}

			// Add additional context if provided
			if params.Context != "" {
				prompt += "\n\nAdditional Context:\n" + params.Context
			}

			fmt.Printf("Spawning specialist: %s...\n", params.Role)

			// Create sub-agent
			subAgent := a.CloneWithPrompt(prompt)

			// We use a simplified Run (no streaming for the inner loop for now to avoid WS complexity)
			// But we could use RunStream if we want the output to be shared.
			// For delegation, we usually just want the final summary.

			result, err := subAgent.Run(ctx, params.Task, nil)
			if err != nil {
				return fmt.Sprintf("Specialist (%s) failed: %v", params.Role, err), nil
			}

			return fmt.Sprintf("### Specialist (%s) Report ###\n\n%s", params.Role, result), nil
		},
		RequiresPermission: false, // Internal delegation
	}
}
