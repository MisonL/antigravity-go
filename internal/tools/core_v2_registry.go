package tools

import "github.com/mison/antigravity-go/internal/corecap"

type CoreToolMode string

const (
	CoreToolModeBase   CoreToolMode = "base"
	CoreToolModeReview CoreToolMode = "review"
)

func (m *CoreV2Manager) AvailableTools(caps corecap.CoreCapabilities, mode CoreToolMode) []Tool {
	if m == nil {
		return nil
	}

	available := make([]Tool, 0, 20)
	add := func(enabled bool, tool Tool) {
		if enabled {
			available = append(available, tool)
		}
	}

	add(caps.RepoInfo.Supported, m.GetRepoInfosTool())
	add(caps.Diagnostics.Supported, m.GetCoreDiagnosticsTool())
	add(caps.Validation.Supported, m.GetValidationStatesTool())
	add(caps.McpStates.Supported, m.GetMcpStatesTool())
	add(caps.McpResources.Supported, m.GetMcpResourcesTool())

	if mode == CoreToolModeReview {
		return available
	}

	add(caps.ApplyEdit.Supported, m.ApplyCoreEditTool())
	add(caps.EditPreview.Supported, m.EditPreviewTool())
	add(caps.BrowserOpen.Supported, m.BrowserOpenTool())
	add(caps.BrowserList.Supported, m.BrowserListTool())
	add(caps.BrowserScreenshot.Supported, m.CaptureScreenshotTool())
	add(caps.BrowserFocus.Supported, m.BrowserFocusTool())
	add(caps.BrowserClick.Supported, m.BrowserClickTool())
	add(caps.BrowserType.Supported, m.BrowserTypeTool())
	add(caps.BrowserScroll.Supported, m.BrowserScrollTool())
	add(caps.MemorySave.Supported, m.MemorySaveTool())
	add(caps.MemoryQuery.Supported, m.MemoryQueryTool())
	add(caps.TrajectoryList.Supported, m.TrajectoryListTool())
	add(caps.TrajectoryGet.Supported, m.TrajectoryGetTool())
	add(caps.TrajectoryExport.Supported, m.TrajectoryExportTool())
	add(caps.CommitMessage.Supported, m.CommitMessageGenerateTool())
	add(caps.Rollback.Supported, m.RollbackToStepTool())
	add(caps.WorkspaceTrack.Supported, m.WorkspaceTrackTool())

	return available
}
