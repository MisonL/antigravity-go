package pathutil

import (
	"fmt"
	"path/filepath"
	"strings"
)

// SanitizePath cleans the path and ensures it's within the given root.
func SanitizePath(root, path string) (string, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}

	// Prevent absolute paths in input
	if filepath.IsAbs(path) {
		return "", fmt.Errorf("security error: absolute path %s not allowed (must be relative to root)", path)
	}

	joined := filepath.Join(absRoot, path)
	absJoined, err := filepath.Abs(joined)
	if err != nil {
		return "", err
	}

	rel, err := filepath.Rel(absRoot, absJoined)
	if err != nil {
		return "", err
	}

	// Check if rel starts with ".." or is ".."
	if strings.HasPrefix(rel, "..") || rel == ".." {
		return "", fmt.Errorf("security error: path %s is outside of root %s", path, root)
	}

	// Double check: ensure absJoined starts with absRoot
	// This handles cases where Rel might be confusing across volumes etc.
	if !strings.HasPrefix(absJoined, absRoot) {
		return "", fmt.Errorf("security error: path %s is outside of root %s", path, root)
	}

	return absJoined, nil
}
