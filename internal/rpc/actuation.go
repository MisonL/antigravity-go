package rpc

// GetPatchAndCodeChange retrieves a patch preview for a pending code change.
func (c *Client) GetPatchAndCodeChange(req map[string]interface{}) (map[string]interface{}, error) {
	if req == nil {
		req = map[string]interface{}{}
	}

	var resp map[string]interface{}
	if err := c.call("GetPatchAndCodeChange", req, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// GetCodeValidationStates retrieves the current workspace validation state.
func (c *Client) GetCodeValidationStates() (map[string]interface{}, error) {
	var resp map[string]interface{}
	if err := c.call("GetCodeValidationStates", map[string]interface{}{}, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}
