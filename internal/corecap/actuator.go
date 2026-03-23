package corecap

import (
	"fmt"

	"github.com/mison/antigravity-go/internal/rpc"
)

// ActuatorManager provides a stable wrapper around Core's actuation-related RPCs.
type ActuatorManager struct {
	client *rpc.Client
}

func NewActuatorManager(client *rpc.Client) *ActuatorManager {
	return &ActuatorManager{client: client}
}

// PreviewEdit loads the patch preview for a pending edit request.
func (m *ActuatorManager) PreviewEdit(req map[string]interface{}) (map[string]interface{}, error) {
	if m == nil || m.client == nil {
		return nil, fmt.Errorf("actuator manager is not initialized")
	}
	return m.client.GetPatchAndCodeChange(req)
}

// GetValidation loads the current validation state from Core.
func (m *ActuatorManager) GetValidation() (map[string]interface{}, error) {
	if m == nil || m.client == nil {
		return nil, fmt.Errorf("actuator manager is not initialized")
	}
	return m.client.GetCodeValidationStates()
}
