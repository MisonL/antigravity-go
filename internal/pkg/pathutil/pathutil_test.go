package pathutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSanitizePath(t *testing.T) {
	cwd, _ := os.Getwd()
	tmpDir := filepath.Join(cwd, "test_sandbox")
	os.MkdirAll(tmpDir, 0755)
	defer os.RemoveAll(tmpDir)

	tests := []struct {
		name      string
		root      string
		input     string
		shouldErr bool
	}{
		{"Valid simple file", tmpDir, "file.txt", false},
		{"Valid nested file", tmpDir, "subdir/file.txt", false},
		{"Path traversal escape", tmpDir, "../outside.txt", true},
		{"Root escape attempt", tmpDir, "/etc/passwd", true}, // Assuming absolute path outside root
		{"Complex traversal", tmpDir, "subdir/../../outside.txt", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := SanitizePath(tt.root, tt.input)
			if tt.shouldErr && err == nil {
				t.Errorf("Expected error for input %s, got nil", tt.input)
			}
			if !tt.shouldErr && err != nil {
				t.Errorf("Unexpected error for input %s: %v", tt.input, err)
			}
		})
	}
}
