package rpc

// UpdateCascadeMemory writes a memory payload to Core's Memory Plane.
func (c *Client) UpdateCascadeMemory(req map[string]interface{}) (map[string]interface{}, error) {
	if req == nil {
		req = map[string]interface{}{}
	}

	var resp map[string]interface{}
	if err := c.call("UpdateCascadeMemory", req, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// GetUserMemories retrieves memory records from Core's Memory Plane.
func (c *Client) GetUserMemories(req map[string]interface{}) (map[string]interface{}, error) {
	if req == nil {
		req = map[string]interface{}{}
	}

	var resp map[string]interface{}
	if err := c.call("GetUserMemories", req, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}
