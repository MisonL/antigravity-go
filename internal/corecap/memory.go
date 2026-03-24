package corecap

import (
	"github.com/mison/antigravity-go/internal/rpc"
)

// MemoryManager provides a stable wrapper around Core's memory-related RPCs.
type MemoryManager struct {
	client *rpc.Client
}

func NewMemoryManager(client *rpc.Client) *MemoryManager {
	return &MemoryManager{client: client}
}

// Save writes a memory payload using Core's UpdateCascadeMemory RPC.
func (m *MemoryManager) Save(req map[string]interface{}) (map[string]interface{}, error) {
	return withManagerClient("memory manager", m, func(client *rpc.Client) (map[string]interface{}, error) {
		return client.UpdateCascadeMemory(req)
	})
}

// Query loads memories using Core's GetUserMemories RPC.
func (m *MemoryManager) Query(req map[string]interface{}) (map[string]interface{}, error) {
	return withManagerClient("memory manager", m, func(client *rpc.Client) (map[string]interface{}, error) {
		return client.GetUserMemories(req)
	})
}

func (m *MemoryManager) getClient() *rpc.Client {
	if m == nil {
		return nil
	}
	return m.client
}
