package corecap

import (
	"fmt"

	"github.com/mison/antigravity-go/internal/rpc"
)

// WorkspaceManager provides a stable wrapper around Core's workspace tracking RPCs.
type WorkspaceManager struct {
	client *rpc.Client
}

func NewWorkspaceManager(client *rpc.Client) *WorkspaceManager {
	return &WorkspaceManager{client: client}
}

// Track registers a workspace root so the kernel can manage incremental awareness.
func (m *WorkspaceManager) Track(root string) (map[string]interface{}, error) {
	if m == nil || m.client == nil {
		return nil, fmt.Errorf("workspace manager is not initialized")
	}
	if root == "" {
		return nil, fmt.Errorf("workspace root is required")
	}
	return m.client.AddTrackedWorkspace(root)
}
