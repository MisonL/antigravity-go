package corecap

import (
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
	if err := requireNonEmpty(diff, "diff"); err != nil {
		return nil, err
	}
	return withManagerClient("versioning manager", m, func(client *rpc.Client) (map[string]interface{}, error) {
		return client.GenerateCommitMessage(diff)
	})
}

// Rollback asks Core to revert the workspace to the given cascade step.
func (m *VersioningManager) Rollback(stepID string) (map[string]interface{}, error) {
	if err := requireNonEmpty(stepID, "step id"); err != nil {
		return nil, err
	}
	return withManagerClient("versioning manager", m, func(client *rpc.Client) (map[string]interface{}, error) {
		return client.RevertToCascadeStep(stepID)
	})
}

func (m *VersioningManager) getClient() *rpc.Client {
	if m == nil {
		return nil
	}
	return m.client
}
