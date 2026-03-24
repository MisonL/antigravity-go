package corecap

import (
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
	return withManagerClient("actuator manager", m, func(client *rpc.Client) (map[string]interface{}, error) {
		return client.GetPatchAndCodeChange(req)
	})
}

// GetValidation loads the current validation state from Core.
func (m *ActuatorManager) GetValidation() (map[string]interface{}, error) {
	return withManagerClient("actuator manager", m, func(client *rpc.Client) (map[string]interface{}, error) {
		return client.GetCodeValidationStates()
	})
}

func managerClient[T interface{ getClient() *rpc.Client }](manager T) *rpc.Client {
	return manager.getClient()
}

func (m *ActuatorManager) getClient() *rpc.Client {
	if m == nil {
		return nil
	}
	return m.client
}
