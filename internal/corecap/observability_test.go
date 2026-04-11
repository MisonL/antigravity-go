package corecap

import (
	"net/http"
	"testing"
)

func TestObservabilityManagerGetCodeFrequencyBuildsFileURI(t *testing.T) {
	client, cleanup := newCapabilityTestClient(t, map[string]func(map[string]interface{}) (int, interface{}){
		"GetCodeFrequencyForRepo": func(body map[string]interface{}) (int, interface{}) {
			if body["repo_uri"] != "file:///tmp/project" {
				t.Fatalf("unexpected repo_uri: %#v", body["repo_uri"])
			}
			return http.StatusOK, map[string]interface{}{
				"codeFrequency": []interface{}{
					map[string]interface{}{
						"numCommits":      2,
						"linesAdded":      10,
						"linesDeleted":    4,
						"recordStartTime": "2026-04-10T10:00:00Z",
						"recordEndTime":   "2026-04-10T11:00:00Z",
					},
				},
			}
		},
	})
	defer cleanup()

	buckets, repoURI, err := NewObservabilityManager(client).GetCodeFrequency("/tmp/project")
	if err != nil {
		t.Fatalf("GetCodeFrequency returned error: %v", err)
	}

	if repoURI != "file:///tmp/project" {
		t.Fatalf("unexpected repoURI: %q", repoURI)
	}
	if len(buckets) != 1 {
		t.Fatalf("unexpected bucket count: %d", len(buckets))
	}
	if buckets[0].NumCommits != 2 || buckets[0].LinesAdded != 10 || buckets[0].LinesDeleted != 4 {
		t.Fatalf("unexpected bucket payload: %+v", buckets[0])
	}
}
