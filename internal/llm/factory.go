package llm

import (
	"strings"
)

func BuildProvider(name, model, apiKey, baseURL string, maxOutput int) (Provider, error) {
	name = strings.ToLower(strings.TrimSpace(name))
	switch name {
	case "anthropic":
		return NewAnthropicProvider(apiKey, model, baseURL, maxOutput), nil
	case "gemini":
		return NewGeminiProvider(apiKey, model)
	default:
		opts := OpenAIOptions{BaseURL: baseURL, MaxTokens: maxOutput}
		return NewOpenAIProviderWithOptions(apiKey, model, opts), nil
	}
}
