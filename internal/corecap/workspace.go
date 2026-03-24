package corecap

import (
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
	if err := requireNonEmpty(root, "workspace root"); err != nil {
		return nil, err
	}
	return withManagerClient("workspace manager", m, func(client *rpc.Client) (map[string]interface{}, error) {
		return client.AddTrackedWorkspace(root)
	})
}

func (m *WorkspaceManager) getClient() *rpc.Client {
	if m == nil {
		return nil
	}
	return m.client
}
