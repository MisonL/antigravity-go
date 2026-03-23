package tools

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mison/antigravity-go/internal/pkg/pathutil"
)

type workspaceContextKey struct{}

type WorkspaceContext struct {
	Root     string            `json:"root,omitempty"`
	Label    string            `json:"label,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

func (w WorkspaceContext) Clone() WorkspaceContext {
	clone := WorkspaceContext{
		Root:  w.Root,
		Label: w.Label,
	}
	if len(w.Metadata) > 0 {
		clone.Metadata = make(map[string]string, len(w.Metadata))
		for key, value := range w.Metadata {
			clone.Metadata[key] = value
		}
	}
	return clone
}

func WithWorkspaceContext(ctx context.Context, workspace WorkspaceContext) context.Context {
	return context.WithValue(ctx, workspaceContextKey{}, workspace.Clone())
}

func WorkspaceContextFromContext(ctx context.Context) WorkspaceContext {
	if ctx == nil {
		return WorkspaceContext{}
	}
	workspace, _ := ctx.Value(workspaceContextKey{}).(WorkspaceContext)
	return workspace.Clone()
}

func WorkspaceRootFromContext(ctx context.Context, fallback string) string {
	workspace := WorkspaceContextFromContext(ctx)
	if strings.TrimSpace(workspace.Root) != "" {
		return workspace.Root
	}
	return fallback
}

func ResolvePathWithinWorkspace(ctx context.Context, fallbackRoot string, target string) (string, error) {
	root := WorkspaceRootFromContext(ctx, fallbackRoot)
	if filepath.IsAbs(target) {
		if strings.TrimSpace(root) == "" {
			return target, nil
		}
		absRoot, err := filepath.Abs(root)
		if err != nil {
			return "", err
		}
		absTarget, err := filepath.Abs(target)
		if err != nil {
			return "", err
		}
		rel, err := filepath.Rel(absRoot, absTarget)
		if err != nil {
			return "", err
		}
		if strings.HasPrefix(rel, "..") || rel == ".." {
			return "", fmt.Errorf("security error: absolute path %s is outside of workspace %s", target, absRoot)
		}
		return absTarget, nil
	}
	if strings.TrimSpace(root) == "" {
		root = "."
	}
	return pathutil.SanitizePath(root, target)
}
