package server

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/mison/antigravity-go/internal/rpc"
)

func isDeprecatedPlaneRPCError(err error) bool {
	if err == nil {
		return false
	}

	if rpc.ShouldSilenceDeprecatedMethodError("GetAllCascadeTrajectories", err) ||
		rpc.ShouldSilenceDeprecatedMethodError("GetUserMemories", err) {
		return true
	}

	msg := strings.ToLower(err.Error())
	for _, token := range []string{"deprecated", "unknown", "unimplemented"} {
		if strings.Contains(msg, token) {
			return true
		}
	}
	return false
}

func writeEmptyJSONArray(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode([]interface{}{})
}

func writeEmptyObservabilitySummary(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(observabilitySummary{
		Trajectories: planeSnapshot{Count: 0},
		Memories:     planeSnapshot{Count: 0},
		GeneratedAt:  time.Now().Format(time.RFC3339),
	})
}
