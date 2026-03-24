package corecap

import (
	"testing"

	"github.com/mison/antigravity-go/internal/rpc"
)

func TestRequireClient(t *testing.T) {
	if err := requireClient("browser manager", &rpc.Client{}); err != nil {
		t.Fatalf("expected initialized client to pass, got %v", err)
	}

	err := requireClient("browser manager", nil)
	if err == nil || err.Error() != "browser manager is not initialized" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRequireNonEmpty(t *testing.T) {
	if err := requireNonEmpty(" value ", "page_id"); err != nil {
		t.Fatalf("expected non-empty value to pass, got %v", err)
	}

	err := requireNonEmpty("   ", "page_id")
	if err == nil || err.Error() != "page_id is required" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFirstNonEmptyString(t *testing.T) {
	payload := map[string]interface{}{
		"markdown": "",
		"content":  "body",
		"text":     "fallback",
	}

	if got := firstNonEmptyString(payload, "markdown", "content", "text"); got != "body" {
		t.Fatalf("expected content field, got %q", got)
	}

	if got := firstNonEmptyString(payload, "missing"); got != "" {
		t.Fatalf("expected empty string for missing keys, got %q", got)
	}
}
