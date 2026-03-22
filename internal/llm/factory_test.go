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
