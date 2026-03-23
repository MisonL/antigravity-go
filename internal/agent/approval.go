package agent

// PermissionRequest carries the tool invocation that needs user approval.
type PermissionRequest struct {
	ToolName string
	Args     string
	Summary  string
	Preview  string
	Metadata map[string]any
}
