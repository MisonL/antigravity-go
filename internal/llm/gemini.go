package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"google.golang.org/genai"
)

type GeminiProvider struct {
	client *genai.Client
	model  string
	config *genai.GenerateContentConfig
}

func NewGeminiProvider(apiKey string, model string) (*GeminiProvider, error) {
	return NewGeminiProviderWithOptions(apiKey, model, GeminiOptions{})
}

type GeminiOptions struct {
	BaseURL         string
	MaxOutputTokens int
}

func NewGeminiProviderWithOptions(apiKey string, model string, opts GeminiOptions) (*GeminiProvider, error) {
	if model == "" {
		model = "gemini-2.0-flash"
	}

	clientConfig := &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	}
	if strings.TrimSpace(opts.BaseURL) != "" {
		clientConfig.HTTPOptions.BaseURL = strings.TrimSpace(opts.BaseURL)
	}

	ctx := context.Background()
	client, err := genai.NewClient(ctx, clientConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client: %w", err)
	}

	config := &genai.GenerateContentConfig{}
	if opts.MaxOutputTokens > 0 {
		config.MaxOutputTokens = int32(opts.MaxOutputTokens)
	}

	return &GeminiProvider{
		client: client,
		model:  model,
		config: config,
	}, nil
}

func (p *GeminiProvider) Chat(ctx context.Context, messages []Message, tools []ToolDefinition) (Message, error) {
	// Convert tools to Gemini format
	var geminiTools []*genai.Tool
	if len(tools) > 0 {
		funcDecls := make([]*genai.FunctionDeclaration, len(tools))
		for i, t := range tools {
			funcDecls[i] = &genai.FunctionDeclaration{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  convertToGeminiSchema(t.Parameters),
			}
		}
		geminiTools = []*genai.Tool{{FunctionDeclarations: funcDecls}}
	}

	config := *p.config
	config.Tools = geminiTools

	// Create chat session
	chat, err := p.client.Chats.Create(ctx, p.model, &config, nil)
	if err != nil {
		return Message{}, fmt.Errorf("failed to create chat: %w", err)
	}

	// 收集 system prompt（Gemini Chats 不支持 system role：以文本前缀方式注入）
	var systemParts []string
	for _, msg := range messages {
		if msg.Role == RoleSystem && strings.TrimSpace(msg.Content) != "" {
			systemParts = append(systemParts, msg.Content)
		}
	}
	systemText := strings.TrimSpace(strings.Join(systemParts, "\n\n"))
	systemInjected := false

	// Convert and send messages
	var lastResult *genai.GenerateContentResponse
	for _, msg := range messages {
		var parts []genai.Part
		if msg.Content != "" {
			text := msg.Content
			if systemText != "" && !systemInjected && msg.Role != RoleTool {
				text = "System:\n" + systemText + "\n\n" + text
				systemInjected = true
			}
			switch msg.Role {
			case RoleUser:
				text = "User:\n" + text
			case RoleAssistant:
				text = "Assistant:\n" + text
			}
			parts = append(parts, genai.Part{Text: text})
		}

		// Handle tool results
		if msg.Role == RoleTool {
			parts = append(parts, genai.Part{
				FunctionResponse: &genai.FunctionResponse{
					Name:     msg.Name,
					Response: map[string]any{"result": msg.Content},
				},
			})
		}

		if len(parts) > 0 {
			lastResult, err = chat.SendMessage(ctx, parts...)
			if err != nil {
				return Message{}, fmt.Errorf("send message failed: %w", err)
			}
		}
	}

	if lastResult == nil && systemText != "" {
		// 仅 system 的极端场景
		lastResult, err = chat.SendMessage(ctx, genai.Part{Text: "System:\n" + systemText})
		if err != nil {
			return Message{}, fmt.Errorf("send message failed: %w", err)
		}
	}

	if lastResult == nil {
		return Message{}, fmt.Errorf("no response from Gemini")
	}

	return p.parseResponse(lastResult)
}

func (p *GeminiProvider) StreamChat(ctx context.Context, messages []Message, tools []ToolDefinition, cb StreamCallback) (Message, error) {
	// For simplicity, use non-streaming for now and call callback once
	// Gemini streaming requires different approach with iterator
	result, err := p.Chat(ctx, messages, tools)
	if err != nil {
		return result, err
	}

	if cb != nil && result.Content != "" {
		cb(result.Content, nil)
	}
	return result, nil
}

func (p *GeminiProvider) parseResponse(resp *genai.GenerateContentResponse) (Message, error) {
	if len(resp.Candidates) == 0 {
		return Message{}, fmt.Errorf("no candidates in response")
	}

	candidate := resp.Candidates[0]
	if candidate.Content == nil || len(candidate.Content.Parts) == 0 {
		return Message{}, fmt.Errorf("no content in candidate")
	}

	msg := Message{Role: RoleAssistant}

	for _, part := range candidate.Content.Parts {
		if part.Text != "" {
			msg.Content += part.Text
		}
		if part.FunctionCall != nil {
			argsJSON, _ := json.Marshal(part.FunctionCall.Args)
			msg.ToolCalls = append(msg.ToolCalls, ToolCall{
				ID:   part.FunctionCall.Name, // Gemini doesn't have separate ID
				Name: part.FunctionCall.Name,
				Args: string(argsJSON),
			})
		}
	}

	return msg, nil
}

func convertToGeminiSchema(params interface{}) *genai.Schema {
	if params == nil {
		return nil
	}

	// Convert map to Gemini Schema
	paramsMap, ok := params.(map[string]interface{})
	if !ok {
		return nil
	}

	schema := &genai.Schema{
		Type: genai.TypeObject,
	}

	if props, ok := paramsMap["properties"].(map[string]interface{}); ok {
		schema.Properties = make(map[string]*genai.Schema)
		for name, prop := range props {
			propMap, _ := prop.(map[string]interface{})
			propSchema := &genai.Schema{}

			if t, ok := propMap["type"].(string); ok {
				switch t {
				case "string":
					propSchema.Type = genai.TypeString
				case "integer":
					propSchema.Type = genai.TypeInteger
				case "boolean":
					propSchema.Type = genai.TypeBoolean
				case "array":
					propSchema.Type = genai.TypeArray
				default:
					propSchema.Type = genai.TypeString
				}
			}
			if desc, ok := propMap["description"].(string); ok {
				propSchema.Description = desc
			}

			schema.Properties[name] = propSchema
		}
	}

	if required, ok := paramsMap["required"].([]string); ok {
		schema.Required = required
	} else if required, ok := paramsMap["required"].([]interface{}); ok {
		for _, r := range required {
			if s, ok := r.(string); ok {
				schema.Required = append(schema.Required, s)
			}
		}
	}

	return schema
}
