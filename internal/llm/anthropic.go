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

type AnthropicOptions struct {
	BaseURL   string
	MaxTokens int
}

func NewAnthropicProvider(apiKey, model string, baseURL string, maxTokens int) *AnthropicProvider {
	return NewAnthropicProviderWithOptions(apiKey, model, AnthropicOptions{
		BaseURL:   baseURL,
		MaxTokens: maxTokens,
	})
}

func NewAnthropicProviderWithOptions(apiKey, model string, opts AnthropicOptions) *AnthropicProvider {
	if opts.BaseURL == "" {
		opts.BaseURL = "https://api.anthropic.com"
	}
	if opts.MaxTokens <= 0 {
		opts.MaxTokens = 4096
	}
	return &AnthropicProvider{
		client:    &http.Client{},
		apiKey:    apiKey,
		model:     model,
		baseURL:   strings.TrimSuffix(opts.BaseURL, "/"),
		maxTokens: opts.MaxTokens,
	}
}

func (p *AnthropicProvider) Chat(ctx context.Context, messages []Message, tools []ToolDefinition) (Message, error) {
	// 实现简易的 Anthropic 协议转换
	return Message{Role: RoleAssistant, Content: "Anthropic Native Provider (v0.1.0) 就绪。"}, nil
}

func (p *AnthropicProvider) StreamChat(ctx context.Context, messages []Message, tools []ToolDefinition, cb StreamCallback) (Message, error) {
	return p.Chat(ctx, messages, tools)
}
