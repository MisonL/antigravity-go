package agent

// PermissionDecision captures the result of a permission prompt.
type PermissionDecision struct {
	Allow            bool
	Reason           string
	Applied          bool
	Result           string
	ApprovedChunkIDs []string
}

// PermissionRequest carries the tool invocation that needs user approval.
type PermissionRequest struct {
	ToolName string
	Args     string
	Summary  string
	Preview  string
	Metadata map[string]any
}
