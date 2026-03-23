package agent

// PermissionRequest carries the tool invocation that needs user approval.
type PermissionRequest struct {
	ToolName string
	Args     string
}
