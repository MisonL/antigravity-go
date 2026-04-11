package corecap

import "github.com/mison/antigravity-go/internal/rpc"

type CoreCapabilities struct {
	Experiments       rpc.MethodProbe   `json:"experiments"`
	Rules             rpc.MethodProbe   `json:"rules"`
	Heartbeat         rpc.MethodProbe   `json:"heartbeat"`
	RunCommand        rpc.MethodProbe   `json:"run_command"`
	RepoInfo          rpc.MethodProbe   `json:"repo_info"`
	WorkspaceTrack    rpc.MethodProbe   `json:"workspace_track"`
	Diagnostics       rpc.MethodProbe   `json:"diagnostics"`
	Validation        rpc.MethodProbe   `json:"validation"`
	EditPreview       rpc.MethodProbe   `json:"edit_preview"`
	ApplyEdit         rpc.MethodProbe   `json:"apply_edit"`
	BrowserList       rpc.MethodProbe   `json:"browser_list"`
	BrowserOpen       rpc.MethodProbe   `json:"browser_open"`
	BrowserFocus      rpc.MethodProbe   `json:"browser_focus"`
	BrowserScreenshot rpc.MethodProbe   `json:"browser_screenshot"`
	BrowserClick      rpc.MethodProbe   `json:"browser_click"`
	BrowserType       rpc.MethodProbe   `json:"browser_type"`
	BrowserScroll     rpc.MethodProbe   `json:"browser_scroll"`
	TrajectoryList    rpc.MethodProbe   `json:"trajectory_list"`
	TrajectoryGet     rpc.MethodProbe   `json:"trajectory_get"`
	TrajectoryExport  rpc.MethodProbe   `json:"trajectory_export"`
	MemoryQuery       rpc.MethodProbe   `json:"memory_query"`
	MemorySave        rpc.MethodProbe   `json:"memory_save"`
	CommitMessage     rpc.MethodProbe   `json:"commit_message"`
	CodeFrequency     rpc.MethodProbe   `json:"code_frequency"`
	Rollback          rpc.MethodProbe   `json:"rollback"`
	McpStates         rpc.MethodProbe   `json:"mcp_states"`
	McpServers        rpc.MethodProbe   `json:"mcp_servers"`
	McpResources      rpc.MethodProbe   `json:"mcp_resources"`
	McpSetting        rpc.MethodProbe   `json:"mcp_setting"`
	McpEnabled        rpc.MethodProbe   `json:"mcp_enabled"`
	McpControl        rpc.McpRPCSupport `json:"mcp_control"`
}

type SurfaceCapabilityPolicy struct {
	Trajectory struct {
		ShowList      bool `json:"show_list"`
		ShowDetail    bool `json:"show_detail"`
		AllowResume   bool `json:"allow_resume"`
		AllowRollback bool `json:"allow_rollback"`
	} `json:"trajectory"`
	Memory struct {
		ShowQuery bool `json:"show_query"`
		AllowSave bool `json:"allow_save"`
	} `json:"memory"`
	Observability struct {
		ShowCodeFrequency bool `json:"show_code_frequency"`
	} `json:"observability"`
	MCP struct {
		Show          bool `json:"show"`
		ReadOnly      bool `json:"read_only"`
		AllowManage   bool `json:"allow_manage"`
		AllowInvoke   bool `json:"allow_invoke"`
		ShowResources bool `json:"show_resources"`
	} `json:"mcp"`
	Browser struct {
		ShowRead      bool `json:"show_read"`
		AllowInteract bool `json:"allow_interact"`
	} `json:"browser"`
}

func ProbeCoreCapabilities(client *rpc.Client) CoreCapabilities {
	if client == nil {
		return CoreCapabilities{}
	}

	return CoreCapabilities{
		Experiments:       client.ProbeMethod([]string{"GetStaticExperimentStatus"}, map[string]interface{}{}),
		Rules:             client.ProbeMethod([]string{"GetAllRules"}, map[string]interface{}{}),
		Heartbeat:         client.ProbeMethod([]string{"Heartbeat"}, map[string]interface{}{}),
		RunCommand:        client.ProbeMethod([]string{"RunCommand"}, map[string]interface{}{}),
		RepoInfo:          client.ProbeMethod([]string{"GetRepoInfos"}, map[string]interface{}{}),
		WorkspaceTrack:    client.ProbeMethod([]string{"AddTrackedWorkspace"}, map[string]interface{}{}),
		Diagnostics:       client.ProbeMethod([]string{"GetDiagnostics"}, map[string]interface{}{}),
		Validation:        client.ProbeMethod([]string{"GetCodeValidationStates"}, map[string]interface{}{}),
		EditPreview:       client.ProbeMethod([]string{"GetPatchAndCodeChange"}, map[string]interface{}{}),
		ApplyEdit:         client.ProbeMethod([]string{"ApplyCodeEdit"}, map[string]interface{}{}),
		BrowserList:       client.ProbeMethod([]string{"ListPages"}, map[string]interface{}{}),
		BrowserOpen:       client.ProbeMethod([]string{"OpenUrl"}, map[string]interface{}{}),
		BrowserFocus:      client.ProbeMethod([]string{"FocusUserPage"}, map[string]interface{}{}),
		BrowserScreenshot: client.ProbeMethod([]string{"CaptureScreenshot"}, map[string]interface{}{}),
		BrowserClick:      client.ProbeMethod([]string{"ClickElement"}, map[string]interface{}{}),
		BrowserType:       client.ProbeMethod([]string{"TypeText"}, map[string]interface{}{}),
		BrowserScroll:     client.ProbeMethod([]string{"ScrollPage"}, map[string]interface{}{}),
		TrajectoryList:    client.ProbeMethod([]string{"GetAllCascadeTrajectories"}, map[string]interface{}{}),
		TrajectoryGet:     client.ProbeMethod([]string{"GetCascadeTrajectory"}, map[string]interface{}{}),
		TrajectoryExport:  client.ProbeMethod([]string{"ConvertTrajectoryToMarkdown"}, map[string]interface{}{}),
		MemoryQuery:       client.ProbeMethod([]string{"GetUserMemories"}, map[string]interface{}{}),
		MemorySave:        client.ProbeMethod([]string{"UpdateCascadeMemory"}, map[string]interface{}{}),
		CommitMessage:     client.ProbeMethod([]string{"GenerateCommitMessage"}, map[string]interface{}{}),
		CodeFrequency:     client.ProbeMethod([]string{"GetCodeFrequencyForRepo"}, map[string]interface{}{}),
		Rollback:          client.ProbeMethod([]string{"RevertToCascadeStep"}, map[string]interface{}{}),
		McpStates:         client.ProbeMethod([]string{"GetMcpServerStates"}, map[string]interface{}{}),
		McpServers:        client.ProbeMethod([]string{"GetMcpServers"}, map[string]interface{}{}),
		McpResources:      client.ProbeMethod([]string{"ListMcpResources"}, map[string]interface{}{}),
		McpSetting:        client.ProbeMethod([]string{"GetMcpSetting"}, map[string]interface{}{}),
		McpEnabled:        client.ProbeMethod([]string{"GetMcpEnabled"}, map[string]interface{}{}),
		McpControl:        client.ProbeMcpRPCSupport(),
	}
}

func DeriveSurfaceCapabilityPolicy(caps CoreCapabilities) SurfaceCapabilityPolicy {
	var policy SurfaceCapabilityPolicy

	policy.Trajectory.ShowList = caps.TrajectoryList.Supported
	policy.Trajectory.ShowDetail = policy.Trajectory.ShowList && caps.TrajectoryGet.Supported
	policy.Trajectory.AllowResume = policy.Trajectory.ShowDetail
	policy.Trajectory.AllowRollback = policy.Trajectory.ShowDetail && caps.Rollback.Supported

	policy.Memory.ShowQuery = caps.MemoryQuery.Supported
	policy.Memory.AllowSave = caps.MemorySave.Supported
	policy.Observability.ShowCodeFrequency = caps.CodeFrequency.Supported

	mcpRead := caps.McpStates.Supported || caps.McpServers.Supported
	mcpResources := caps.McpResources.Supported
	mcpManage := caps.McpControl.Add.Supported || caps.McpControl.Refresh.Supported || caps.McpControl.Restart.Supported
	mcpInvoke := caps.McpControl.Invoke.Supported
	policy.MCP.Show = mcpRead || mcpResources || mcpManage || mcpInvoke
	policy.MCP.ReadOnly = (mcpRead || mcpResources) && !mcpManage && !mcpInvoke
	policy.MCP.AllowManage = mcpManage
	policy.MCP.AllowInvoke = mcpInvoke
	policy.MCP.ShowResources = mcpResources

	policy.Browser.ShowRead = caps.BrowserList.Supported || caps.BrowserOpen.Supported || caps.BrowserFocus.Supported || caps.BrowserScreenshot.Supported
	policy.Browser.AllowInteract = caps.BrowserClick.Supported || caps.BrowserType.Supported || caps.BrowserScroll.Supported

	return policy
}
