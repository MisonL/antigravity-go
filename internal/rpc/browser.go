package rpc

// ListPages lists all open browser pages managed by Core.
func (c *Client) ListPages() (map[string]interface{}, error) {
	var resp map[string]interface{}
	if err := c.call("ListPages", map[string]interface{}{}, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// OpenUrl opens a URL in the Core-managed browser.
func (c *Client) OpenUrl(url string) (map[string]interface{}, error) {
	req := map[string]interface{}{
		"url": url,
	}
	var resp map[string]interface{}
	if err := c.call("OpenUrl", req, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// FocusUserPage brings a specific browser page to focus.
func (c *Client) FocusUserPage(pageID string) (map[string]interface{}, error) {
	req := map[string]interface{}{
		"page_id": pageID,
	}
	var resp map[string]interface{}
	if err := c.call("FocusUserPage", req, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// CaptureScreenshot captures a screenshot of the specified browser page managed by Core.
func (c *Client) CaptureScreenshot(pageID string) (map[string]interface{}, error) {
	req := map[string]interface{}{
		"page_id": pageID,
	}
	var resp map[string]interface{}
	if err := c.call("CaptureScreenshot", req, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// ClickElement clicks the first element matching selector on the target page.
func (c *Client) ClickElement(pageID, selector string) (map[string]interface{}, error) {
	req := map[string]interface{}{
		"page_id":  pageID,
		"selector": selector,
	}
	var resp map[string]interface{}
	if err := c.call("ClickElement", req, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// TypeText enters text into the first element matching selector on the target page.
func (c *Client) TypeText(pageID, selector, text string) (map[string]interface{}, error) {
	req := map[string]interface{}{
		"page_id":  pageID,
		"selector": selector,
		"text":     text,
	}
	var resp map[string]interface{}
	if err := c.call("TypeText", req, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// ScrollPage scrolls the target page by the given delta.
func (c *Client) ScrollPage(pageID string, deltaX, deltaY int) (map[string]interface{}, error) {
	req := map[string]interface{}{
		"page_id": pageID,
		"delta_x": deltaX,
		"delta_y": deltaY,
	}
	var resp map[string]interface{}
	if err := c.call("ScrollPage", req, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}
