package llm

import "testing"

func TestBuildProviderAnthropicPassesOptions(t *testing.T) {
	provider, err := BuildProvider("anthropic", "claude-test", "secret", "https://anthropic.example", 2048)
	if err != nil {
		t.Fatalf("BuildProvider returned error: %v", err)
	}

	anthropicProvider, ok := provider.(*AnthropicProvider)
	if !ok {
		t.Fatalf("expected *AnthropicProvider, got %T", provider)
	}

	if anthropicProvider.baseURL != "https://anthropic.example" {
		t.Fatalf("unexpected base URL: %q", anthropicProvider.baseURL)
	}
	if anthropicProvider.maxTokens != 2048 {
		t.Fatalf("unexpected max tokens: %d", anthropicProvider.maxTokens)
	}
}

func TestBuildProviderGeminiPassesMaxOutput(t *testing.T) {
	provider, err := BuildProvider("gemini", "gemini-test", "secret", "https://gemini.example", 1536)
	if err != nil {
		t.Fatalf("BuildProvider returned error: %v", err)
	}

	geminiProvider, ok := provider.(*GeminiProvider)
	if !ok {
		t.Fatalf("expected *GeminiProvider, got %T", provider)
	}

	if geminiProvider.config == nil {
		t.Fatal("expected Gemini config to be initialized")
	}
	if geminiProvider.config.MaxOutputTokens != 1536 {
		t.Fatalf("unexpected max output tokens: %d", geminiProvider.config.MaxOutputTokens)
	}
}

func TestBuildProviderOllamaUsesOpenAIAdapterDefaults(t *testing.T) {
	provider, err := BuildProvider("ollama", "qwen2.5-coder", "", "", 512)
	if err != nil {
		t.Fatalf("BuildProvider returned error: %v", err)
	}

	openAIProvider, ok := provider.(*OpenAIProvider)
	if !ok {
		t.Fatalf("expected *OpenAIProvider, got %T", provider)
	}

	if openAIProvider.baseURL != "http://localhost:11434/v1" {
		t.Fatalf("unexpected base URL: %q", openAIProvider.baseURL)
	}
	if openAIProvider.maxTokens != 512 {
		t.Fatalf("unexpected max tokens: %d", openAIProvider.maxTokens)
	}
}

func TestBuildProviderUsesEnvironmentAPIKey(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "env-secret")

	provider, err := BuildProvider("anthropic", "claude-test", "", "", 1024)
	if err != nil {
		t.Fatalf("BuildProvider returned error: %v", err)
	}

	anthropicProvider, ok := provider.(*AnthropicProvider)
	if !ok {
		t.Fatalf("expected *AnthropicProvider, got %T", provider)
	}
	if anthropicProvider.apiKey != "env-secret" {
		t.Fatalf("unexpected api key: %q", anthropicProvider.apiKey)
	}
}

func TestBuildProviderRejectsUnknownProvider(t *testing.T) {
	if _, err := BuildProvider("unknown", "test", "secret", "", 0); err == nil {
		t.Fatal("expected error for unknown provider")
	}
}
