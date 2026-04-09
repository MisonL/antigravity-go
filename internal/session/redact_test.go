package session

import (
	"strings"
	"testing"
)

func TestRedactStringMasksGitHubCredentials(t *testing.T) {
	token := githubTokenPrefix + strings.Repeat("A", 20)
	pat := githubPATPrefix + strings.Repeat("b", 20)

	input := "token=" + token + "\npat=" + pat
	got := RedactString(input)

	if strings.Contains(got, token) {
		t.Fatalf("expected GitHub token to be redacted, got %q", got)
	}
	if strings.Contains(got, pat) {
		t.Fatalf("expected GitHub PAT to be redacted, got %q", got)
	}
	if !strings.Contains(got, "<REDACTED:GITHUB_TOKEN>") {
		t.Fatalf("expected GitHub token marker in %q", got)
	}
	if !strings.Contains(got, "<REDACTED:GITHUB_PAT>") {
		t.Fatalf("expected GitHub PAT marker in %q", got)
	}
}
