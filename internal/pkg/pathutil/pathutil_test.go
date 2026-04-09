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

func TestSanitizePathRejectsSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()

	outsideFile := filepath.Join(outside, "secret.txt")
	if err := os.WriteFile(outsideFile, []byte("secret"), 0644); err != nil {
		t.Fatalf("write outside file: %v", err)
	}

	fileLink := filepath.Join(root, "secret-link.txt")
	if err := os.Symlink(outsideFile, fileLink); err != nil {
		t.Fatalf("create file symlink: %v", err)
	}

	if _, err := SanitizePath(root, "secret-link.txt"); err == nil {
		t.Fatal("expected symlinked file escape to be rejected")
	}

	dirLink := filepath.Join(root, "escape-dir")
	if err := os.Symlink(outside, dirLink); err != nil {
		t.Fatalf("create dir symlink: %v", err)
	}

	if _, err := SanitizePath(root, filepath.Join("escape-dir", "nested.txt")); err == nil {
		t.Fatal("expected symlinked directory escape to be rejected")
	}
}
