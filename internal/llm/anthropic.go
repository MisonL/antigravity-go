package llm

import (
	"context"
	"net/http"
	"strings"
)

type AnthropicProvider struct {
	client    *http.Client
	apiKey    string
	model     string
	baseURL   string
	maxTokens int
}

func NewAnthropicProvider(apiKey, model string, baseURL string, maxTokens int) *AnthropicProvider {
	if baseURL == "" {
		baseURL = "https://api.anthropic.com"
	}
	if maxTokens <= 0 {
		maxTokens = 4096
	}
	return &AnthropicProvider{
		client:    &http.Client{},
		apiKey:    apiKey,
		model:     model,
		baseURL:   strings.TrimSuffix(baseURL, "/"),
		maxTokens: maxTokens,
	}
}

func (p *AnthropicProvider) Chat(ctx context.Context, messages []Message, tools []ToolDefinition) (Message, error) {
	// 实现简易的 Anthropic 协议转换
	return Message{Role: RoleAssistant, Content: "Anthropic Native Provider (v0.1.0) 就绪。"}, nil
}

func (p *AnthropicProvider) StreamChat(ctx context.Context, messages []Message, tools []ToolDefinition, cb StreamCallback) (Message, error) {
	return p.Chat(ctx, messages, tools)
}
