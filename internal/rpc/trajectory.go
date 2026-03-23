package rpc

// GetAllCascadeTrajectories retrieves all cascade trajectories.
func (c *Client) GetAllCascadeTrajectories() (map[string]interface{}, error) {
	var resp map[string]interface{}
	if err := c.call("GetAllCascadeTrajectories", map[string]interface{}{}, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// GetCascadeTrajectory retrieves a single cascade trajectory by ID.
func (c *Client) GetCascadeTrajectory(id string) (map[string]interface{}, error) {
	req := map[string]interface{}{
		"id": id,
	}

	var resp map[string]interface{}
	if err := c.call("GetCascadeTrajectory", req, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// ConvertTrajectoryToMarkdown exports a trajectory as markdown.
func (c *Client) ConvertTrajectoryToMarkdown(id string) (map[string]interface{}, error) {
	req := map[string]interface{}{
		"id": id,
	}

	var resp map[string]interface{}
	if err := c.call("ConvertTrajectoryToMarkdown", req, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}
