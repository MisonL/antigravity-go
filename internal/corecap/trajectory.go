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
	return withManagerClient("trajectory manager", m, func(client *rpc.Client) (map[string]interface{}, error) {
		return client.GetAllCascadeTrajectories()
	})
}

// Get loads a single trajectory by ID.
func (m *TrajectoryManager) Get(id string) (map[string]interface{}, error) {
	if err := requireNonEmpty(id, "trajectory id"); err != nil {
		return nil, err
	}
	return withManagerClient("trajectory manager", m, func(client *rpc.Client) (map[string]interface{}, error) {
		return client.GetCascadeTrajectory(id)
	})
}

// Export converts a trajectory to markdown and returns the markdown body.
func (m *TrajectoryManager) Export(id string) (string, error) {
	if err := requireNonEmpty(id, "trajectory id"); err != nil {
		return "", err
	}

	resp, err := withManagerClient("trajectory manager", m, func(client *rpc.Client) (map[string]interface{}, error) {
		return client.ConvertTrajectoryToMarkdown(id)
	})
	if err != nil {
		return "", err
	}

	if markdown := firstNonEmptyString(resp, "markdown", "content", "text"); markdown != "" {
		return markdown, nil
	}

	return "", fmt.Errorf("trajectory markdown missing in response")
}

func (m *TrajectoryManager) getClient() *rpc.Client {
	if m == nil {
		return nil
	}
	return m.client
}
