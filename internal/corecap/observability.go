package corecap

import (
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/mison/antigravity-go/internal/rpc"
)

const codeFrequencyTimeout = 2 * time.Minute

// CodeFrequencyBucket is the normalized code activity bucket returned by the server API.
type CodeFrequencyBucket struct {
	NumCommits      int    `json:"num_commits"`
	LinesAdded      int    `json:"lines_added"`
	LinesDeleted    int    `json:"lines_deleted"`
	RecordStartTime string `json:"record_start_time"`
	RecordEndTime   string `json:"record_end_time"`
}

// ObservabilityManager provides wrappers for read-only repository observability RPCs.
type ObservabilityManager struct {
	client *rpc.Client
}

func NewObservabilityManager(client *rpc.Client) *ObservabilityManager {
	return &ObservabilityManager{client: client}
}

func (m *ObservabilityManager) getClient() *rpc.Client {
	if m == nil {
		return nil
	}
	return m.client
}

// GetCodeFrequency loads repository activity buckets for the provided local repo path.
func (m *ObservabilityManager) GetCodeFrequency(repoPath string) ([]CodeFrequencyBucket, string, error) {
	if err := requireNonEmpty(repoPath, "repo path"); err != nil {
		return nil, "", err
	}

	repoURI, err := fileURIFromPath(repoPath)
	if err != nil {
		return nil, "", err
	}

	resp, err := withManagerClient("observability manager", m, func(client *rpc.Client) (*rpc.GetCodeFrequencyForRepoResponse, error) {
		return client.GetCodeFrequencyForRepo(map[string]interface{}{
			"repo_uri": repoURI,
		}, codeFrequencyTimeout)
	})
	if err != nil {
		return nil, "", err
	}

	buckets := make([]CodeFrequencyBucket, 0, len(resp.CodeFrequency))
	for _, item := range resp.CodeFrequency {
		buckets = append(buckets, CodeFrequencyBucket{
			NumCommits:      item.NumCommits,
			LinesAdded:      item.LinesAdded,
			LinesDeleted:    item.LinesDeleted,
			RecordStartTime: item.RecordStartTime,
			RecordEndTime:   item.RecordEndTime,
		})
	}

	return buckets, repoURI, nil
}

func fileURIFromPath(path string) (string, error) {
	absPath, err := filepath.Abs(strings.TrimSpace(path))
	if err != nil {
		return "", err
	}

	slashed := filepath.ToSlash(absPath)
	if !strings.HasPrefix(slashed, "/") {
		slashed = "/" + slashed
	}

	return (&url.URL{
		Scheme: "file",
		Path:   slashed,
	}).String(), nil
}
