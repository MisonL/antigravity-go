package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveReviewTargetDefaultsToWorkspace(t *testing.T) {
	workspace := t.TempDir()

	target, err := resolveReviewTarget(workspace, "")
	if err != nil {
		t.Fatalf("resolveReviewTarget returned error: %v", err)
	}
	if target.Relative != "." {
		t.Fatalf("expected relative path '.', got %q", target.Relative)
	}
	if target.Absolute != workspace {
		t.Fatalf("expected absolute path %q, got %q", workspace, target.Absolute)
	}
	if !target.IsDir {
		t.Fatal("expected workspace root to be treated as directory")
	}
}

func TestResolveReviewTargetRejectsOutsideWorkspace(t *testing.T) {
	workspace := t.TempDir()
	parent := filepath.Dir(workspace)

	outsideFile := filepath.Join(parent, "outside.go")
	if err := os.WriteFile(outsideFile, []byte("package main"), 0644); err != nil {
		t.Fatalf("failed to create outside file: %v", err)
	}
	defer os.Remove(outsideFile)

	if _, err := resolveReviewTarget(workspace, outsideFile); err == nil {
		t.Fatal("expected target outside workspace to be rejected")
	}
}

func TestValidationReportHasIssues(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want bool
	}{
		{
			name: "passed status",
			raw:  `{"status":"passed","results":[]}`,
			want: false,
		},
		{
			name: "failed status",
			raw:  `{"status":"failed"}`,
			want: true,
		},
		{
			name: "warning severity",
			raw:  `{"items":[{"severity":"warning","message":"lint"}]}`,
			want: true,
		},
		{
			name: "error count",
			raw:  `{"errorCount":2}`,
			want: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := validationReportHasIssues(tc.raw); got != tc.want {
				t.Fatalf("validationReportHasIssues(%s) = %v, want %v", tc.raw, got, tc.want)
			}
		})
	}
}
