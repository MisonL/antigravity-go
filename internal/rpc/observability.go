package rpc

import "time"

// CodeFrequencyRecord represents a single repository activity bucket.
type CodeFrequencyRecord struct {
	NumCommits      int    `json:"numCommits"`
	LinesAdded      int    `json:"linesAdded"`
	LinesDeleted    int    `json:"linesDeleted,omitempty"`
	RecordStartTime string `json:"recordStartTime"`
	RecordEndTime   string `json:"recordEndTime"`
}

// GetCodeFrequencyForRepoResponse is the response returned by Core.
type GetCodeFrequencyForRepoResponse struct {
	CodeFrequency []CodeFrequencyRecord `json:"codeFrequency"`
}

// GetCodeFrequencyForRepo loads repository code frequency buckets from Core.
func (c *Client) GetCodeFrequencyForRepo(req map[string]interface{}, timeout time.Duration) (*GetCodeFrequencyForRepoResponse, error) {
	if req == nil {
		req = map[string]interface{}{}
	}

	var resp GetCodeFrequencyForRepoResponse
	if err := c.callWithTimeout("GetCodeFrequencyForRepo", req, &resp, timeout); err != nil {
		return nil, err
	}
	return &resp, nil
}
