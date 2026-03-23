package rpc

// GenerateCommitMessage asks Core to produce a commit message for a diff.
func (c *Client) GenerateCommitMessage(diff string) (map[string]interface{}, error) {
	req := map[string]interface{}{
		"diff": diff,
	}

	var resp map[string]interface{}
	if err := c.call("GenerateCommitMessage", req, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// RevertToCascadeStep asks Core to roll the workspace back to a trajectory step.
func (c *Client) RevertToCascadeStep(stepID string) (map[string]interface{}, error) {
	req := map[string]interface{}{
		"step_id": stepID,
	}

	var resp map[string]interface{}
	if err := c.call("RevertToCascadeStep", req, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}
