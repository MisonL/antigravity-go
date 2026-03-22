package llm

import (
	"context"
	"strings"

	openai "github.com/sashabaranov/go-openai"
)

type OpenAIProvider struct {
	client    *openai.Client
	model     string
	maxTokens int
}

func NewOpenAIProvider(token string, model string) *OpenAIProvider {
	return NewOpenAIProviderWithOptions(token, model, OpenAIOptions{})
}

func NewOpenAIProviderWithBaseURL(token string, model string, baseURL string) *OpenAIProvider {
	return NewOpenAIProviderWithOptions(token, model, OpenAIOptions{BaseURL: baseURL})
}

type OpenAIOptions struct {
	BaseURL   string
	MaxTokens int
}

func NewOpenAIProviderWithOptions(token string, model string, opts OpenAIOptions) *OpenAIProvider {
	if model == "" {
		model = openai.GPT4o
	}

	cfg := openai.DefaultConfig(token)
	if opts.BaseURL != "" {
		cfg.BaseURL = normalizeBaseURL(opts.BaseURL)
	}
	return &OpenAIProvider{
		client:    openai.NewClientWithConfig(cfg),
		model:     model,
		maxTokens: opts.MaxTokens,
	}
}

func normalizeBaseURL(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimRight(v, "/")
	if v == "" {
		return v
	}
	// go-openai 的 OpenAI BaseURL 默认包含 /v1；为了更宽容，未指定时自动补齐。
	if strings.HasSuffix(v, "/v1") {
		return v
	}
	return v + "/v1"
}

func (p *OpenAIProvider) Chat(ctx context.Context, messages []Message, tools []ToolDefinition) (Message, error) {
	reqMsgs := make([]openai.ChatCompletionMessage, len(messages))
	for i, m := range messages {
		reqMsgs[i] = openai.ChatCompletionMessage{
			Role:       string(m.Role),
			Content:    m.Content,
			Name:       m.Name,
			ToolCallID: m.ToolCallID,
		}

		if len(m.ToolCalls) > 0 {
			apiToolCalls := make([]openai.ToolCall, len(m.ToolCalls))
			for j, tc := range m.ToolCalls {
				apiToolCalls[j] = openai.ToolCall{
					ID:   tc.ID,
					Type: openai.ToolTypeFunction,
					Function: openai.FunctionCall{
						Name:      tc.Name,
						Arguments: tc.Args,
					},
				}
			}
			reqMsgs[i].ToolCalls = apiToolCalls
		}
	}

	req := openai.ChatCompletionRequest{
		Model:    p.model,
		Messages: reqMsgs,
	}
	if p.maxTokens > 0 {
		req.MaxTokens = p.maxTokens
	}

	if len(tools) > 0 {
		apiTools := make([]openai.Tool, len(tools))
		for i, t := range tools {
			apiTools[i] = openai.Tool{
				Type: openai.ToolTypeFunction,
				Function: &openai.FunctionDefinition{
					Name:        t.Name,
					Description: t.Description,
					Parameters:  t.Parameters,
				},
			}
		}
		req.Tools = apiTools
	}

	resp, err := p.client.CreateChatCompletion(ctx, req)
	if err != nil {
		return Message{}, err
	}

	choice := resp.Choices[0]
	resMsg := Message{
		Role:    Role(choice.Message.Role),
		Content: choice.Message.Content,
	}

	if len(choice.Message.ToolCalls) > 0 {
		toolCalls := make([]ToolCall, len(choice.Message.ToolCalls))
		for i, tc := range choice.Message.ToolCalls {
			toolCalls[i] = ToolCall{
				ID:   tc.ID,
				Name: tc.Function.Name,
				Args: tc.Function.Arguments,
			}
		}
		resMsg.ToolCalls = toolCalls
	}

	return resMsg, nil
}

func (p *OpenAIProvider) StreamChat(ctx context.Context, messages []Message, tools []ToolDefinition, cb StreamCallback) (Message, error) {
	reqMsgs := make([]openai.ChatCompletionMessage, len(messages))
	for i, m := range messages {
		reqMsgs[i] = openai.ChatCompletionMessage{
			Role:       string(m.Role),
			Content:    m.Content,
			Name:       m.Name,
			ToolCallID: m.ToolCallID,
		}

		if len(m.ToolCalls) > 0 {
			apiToolCalls := make([]openai.ToolCall, len(m.ToolCalls))
			for j, tc := range m.ToolCalls {
				apiToolCalls[j] = openai.ToolCall{
					ID:   tc.ID,
					Type: openai.ToolTypeFunction,
					Function: openai.FunctionCall{
						Name:      tc.Name,
						Arguments: tc.Args,
					},
				}
			}
			reqMsgs[i].ToolCalls = apiToolCalls
		}
	}

	req := openai.ChatCompletionRequest{
		Model:    p.model,
		Messages: reqMsgs,
		Stream:   true,
	}
	if p.maxTokens > 0 {
		req.MaxTokens = p.maxTokens
	}

	if len(tools) > 0 {
		apiTools := make([]openai.Tool, len(tools))
		for i, t := range tools {
			apiTools[i] = openai.Tool{
				Type: openai.ToolTypeFunction,
				Function: &openai.FunctionDefinition{
					Name:        t.Name,
					Description: t.Description,
					Parameters:  t.Parameters,
				},
			}
		}
		req.Tools = apiTools
	}

	stream, err := p.client.CreateChatCompletionStream(ctx, req)
	if err != nil {
		return Message{}, err
	}
	defer stream.Close()

	var finalContent string
	var finalRole Role

	// For accumulating tool calls across stream chunks
	type toolCallAccumulator struct {
		ID    string
		Name  string
		Args  string
		Index int
	}
	accToolCalls := make(map[int]*toolCallAccumulator)

	for {
		response, err := stream.Recv()
		if err != nil {
			return Message{}, err
		}

		// Handle stop
		if len(response.Choices) == 0 {
			continue // Should limit this
		}

		choice := response.Choices[0]

		// If finish reason is set, we are likely done (unless tool call needs to finish)
		if choice.FinishReason != "" && choice.FinishReason != "stop" && choice.FinishReason != "tool_calls" {
			// e.g. length, content_filter
		}

		if choice.FinishReason == "stop" || choice.FinishReason == "tool_calls" {
			// Stream finished
			break
		}

		// Delta content
		content := choice.Delta.Content
		if content != "" {
			finalContent += content
			if cb != nil {
				cb(content, nil)
			}
		}

		if choice.Delta.Role != "" {
			finalRole = Role(choice.Delta.Role)
		}

		// Delta tool calls
		if len(choice.Delta.ToolCalls) > 0 {
			for _, tc := range choice.Delta.ToolCalls {
				idx := 0
				if tc.Index != nil {
					idx = *tc.Index
				}

				if _, exists := accToolCalls[idx]; !exists {
					accToolCalls[idx] = &toolCallAccumulator{Index: idx}
				}
				acc := accToolCalls[idx]

				if tc.ID != "" {
					acc.ID = tc.ID
				}
				if tc.Function.Name != "" {
					acc.Name = tc.Function.Name
				}
				if tc.Function.Arguments != "" {
					acc.Args += tc.Function.Arguments
				}
			}
		}
	}

	resMsg := Message{
		Role:    finalRole,
		Content: finalContent,
	}
	if resMsg.Role == "" {
		resMsg.Role = RoleAssistant
	}

	if len(accToolCalls) > 0 {
		toolCalls := make([]ToolCall, len(accToolCalls))
		for i := 0; i < len(accToolCalls); i++ {
			if acc, ok := accToolCalls[i]; ok {
				toolCalls[i] = ToolCall{
					ID:   acc.ID,
					Name: acc.Name,
					Args: acc.Args,
				}
			}
		}
		resMsg.ToolCalls = toolCalls
	}

	return resMsg, nil
}
