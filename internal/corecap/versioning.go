package corecap

import (
	"fmt"

	"github.com/mison/antigravity-go/internal/rpc"
)

// VersioningManager provides a stable wrapper around Core's versioning-related RPCs.
type VersioningManager struct {
	client *rpc.Client
}

func NewVersioningManager(client *rpc.Client) *VersioningManager {
	return &VersioningManager{client: client}
}

// GenerateCommit asks Core to generate a commit message from a diff.
func (m *VersioningManager) GenerateCommit(diff string) (map[string]interface{}, error) {
	if m == nil || m.client == nil {
		return nil, fmt.Errorf("versioning manager is not initialized")
	}
	if diff == "" {
		return nil, fmt.Errorf("diff is required")
	}
	return m.client.GenerateCommitMessage(diff)
}

// Rollback asks Core to revert the workspace to the given cascade step.
func (m *VersioningManager) Rollback(stepID string) (map[string]interface{}, error) {
	if m == nil || m.client == nil {
		return nil, fmt.Errorf("versioning manager is not initialized")
	}
	if stepID == "" {
		return nil, fmt.Errorf("step id is required")
	}
	return m.client.RevertToCascadeStep(stepID)
}
