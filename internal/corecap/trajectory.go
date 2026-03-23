package corecap

import (
	"fmt"

	"github.com/mison/antigravity-go/internal/rpc"
)

// TrajectoryManager provides a stable wrapper around Core's trajectory-related RPCs.
type TrajectoryManager struct {
	client *rpc.Client
}

func NewTrajectoryManager(client *rpc.Client) *TrajectoryManager {
	return &TrajectoryManager{client: client}
}

// List loads all trajectories from Core's Trajectory Plane.
func (m *TrajectoryManager) List() (map[string]interface{}, error) {
	if m == nil || m.client == nil {
		return nil, fmt.Errorf("trajectory manager is not initialized")
	}
	return m.client.GetAllCascadeTrajectories()
}

// Get loads a single trajectory by ID.
func (m *TrajectoryManager) Get(id string) (map[string]interface{}, error) {
	if m == nil || m.client == nil {
		return nil, fmt.Errorf("trajectory manager is not initialized")
	}
	if id == "" {
		return nil, fmt.Errorf("trajectory id is required")
	}
	return m.client.GetCascadeTrajectory(id)
}

// Export converts a trajectory to markdown and returns the markdown body.
func (m *TrajectoryManager) Export(id string) (string, error) {
	if m == nil || m.client == nil {
		return "", fmt.Errorf("trajectory manager is not initialized")
	}
	if id == "" {
		return "", fmt.Errorf("trajectory id is required")
	}

	resp, err := m.client.ConvertTrajectoryToMarkdown(id)
	if err != nil {
		return "", err
	}

	for _, key := range []string{"markdown", "content", "text"} {
		if value, ok := resp[key].(string); ok && value != "" {
			return value, nil
		}
	}

	return "", fmt.Errorf("trajectory markdown missing in response")
}
