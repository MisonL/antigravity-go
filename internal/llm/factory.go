package llm

import (
	"fmt"
	"os"
	"strings"
)

func NormalizeProviderName(name string) string {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "openai":
		return "openai"
	case "anthropic":
		return "anthropic"
	case "gemini":
		return "gemini"
	case "ollama":
		return "ollama"
	case "lmstudio", "lm-studio", "lm_studio":
		return "lmstudio"
	case "iflow":
		return "iflow"
	default:
		return strings.ToLower(strings.TrimSpace(name))
	}
}

func ProviderNeedsAPIKey(name string) bool {
	switch NormalizeProviderName(name) {
	case "ollama", "lmstudio":
		return false
	default:
		return true
	}
}

func ResolveProviderBaseURL(name, baseURL string) string {
	if trimmed := strings.TrimSpace(baseURL); trimmed != "" {
		return trimmed
	}

	switch NormalizeProviderName(name) {
	case "openai":
		return firstNonEmptyEnv("OPENAI_BASE_URL", "OPENAI_API_BASE")
	case "iflow":
		return firstNonEmptyEnv("IFLOW_BASE_URL")
	case "ollama":
		return "http://localhost:11434"
	case "lmstudio":
		return "http://localhost:1234"
	default:
		return ""
	}
}

func BuildProvider(name, model, apiKey, baseURL string, maxOutput int) (Provider, error) {
	name = NormalizeProviderName(name)
	apiKey = resolveProviderAPIKey(name, apiKey)
	baseURL = ResolveProviderBaseURL(name, baseURL)

	if ProviderNeedsAPIKey(name) && strings.TrimSpace(apiKey) == "" {
		return nil, fmt.Errorf("provider %q requires an API key", name)
	}

	switch name {
	case "openai", "ollama", "lmstudio", "iflow":
		opts := OpenAIOptions{BaseURL: baseURL, MaxTokens: maxOutput}
		return WrapProviderWithRetryBudget(NewOpenAIProviderWithOptions(apiKey, model, opts), RetryBudget{}), nil
	case "anthropic":
		opts := AnthropicOptions{BaseURL: baseURL, MaxTokens: maxOutput}
		return WrapProviderWithRetryBudget(NewAnthropicProviderWithOptions(apiKey, model, opts), RetryBudget{}), nil
	case "gemini":
		opts := GeminiOptions{BaseURL: baseURL, MaxOutputTokens: maxOutput}
		provider, err := NewGeminiProviderWithOptions(apiKey, model, opts)
		if err != nil {
			return nil, err
		}
		return WrapProviderWithRetryBudget(provider, RetryBudget{}), nil
	default:
		return nil, fmt.Errorf("unsupported provider %q", name)
	}
}

func resolveProviderAPIKey(name, apiKey string) string {
	if trimmed := strings.TrimSpace(apiKey); trimmed != "" {
		return trimmed
	}

	switch NormalizeProviderName(name) {
	case "openai", "ollama", "lmstudio":
		return firstNonEmptyEnv("OPENAI_API_KEY")
	case "anthropic":
		return firstNonEmptyEnv("ANTHROPIC_API_KEY")
	case "gemini":
		return firstNonEmptyEnv("GEMINI_API_KEY")
	case "iflow":
		return firstNonEmptyEnv("IFLOW_API_KEY")
	default:
		return ""
	}
}

func firstNonEmptyEnv(keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	return ""
}
