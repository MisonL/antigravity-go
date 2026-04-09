package pathutil

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SanitizePath cleans the path and ensures it's within the given root.
func SanitizePath(root, path string) (string, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	realRoot, err := filepath.EvalSymlinks(absRoot)
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

	resolvedPath, err := resolvePathWithinRoot(realRoot, absJoined)
	if err != nil {
		return "", err
	}

	resolvedRel, err := filepath.Rel(realRoot, resolvedPath)
	if err != nil {
		return "", err
	}

	// Double check against the resolved path so symlink escapes are rejected too.
	if strings.HasPrefix(resolvedRel, "..") || resolvedRel == ".." {
		return "", fmt.Errorf("security error: path %s is outside of root %s", path, root)
	}

	return absJoined, nil
}

func resolvePathWithinRoot(realRoot, absJoined string) (string, error) {
	existingPath := absJoined
	suffix := make([]string, 0)

	for {
		if _, err := os.Lstat(existingPath); err == nil {
			resolvedExisting, resolveErr := filepath.EvalSymlinks(existingPath)
			if resolveErr != nil {
				return "", resolveErr
			}
			if len(suffix) == 0 {
				return resolvedExisting, nil
			}
			for index := len(suffix) - 1; index >= 0; index -= 1 {
				resolvedExisting = filepath.Join(resolvedExisting, suffix[index])
			}
			return resolvedExisting, nil
		}

		parent := filepath.Dir(existingPath)
		if parent == existingPath {
			return "", fmt.Errorf("security error: path %s is outside of root %s", absJoined, realRoot)
		}
		suffix = append(suffix, filepath.Base(existingPath))
		existingPath = parent
	}
}
