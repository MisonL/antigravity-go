package tui

import (
	"strings"

	"github.com/mison/antigravity-go/internal/corecap"
)

func (m *Model) statusMarkdown() string {
	ready := false
	if m.host != nil {
		ready = m.host.IsReady()
	}

	var sb strings.Builder
	sb.WriteString(m.t("tui.command.status.summary", ready))

	if m.client == nil {
		sb.WriteString("\n")
		sb.WriteString(m.t("tui.command.status.capabilities_unavailable"))
		return sb.String()
	}

	caps := corecap.ProbeCoreCapabilities(m.client)
	policy := corecap.DeriveSurfaceCapabilityPolicy(caps)

	sb.WriteString("\n\n")
	sb.WriteString(m.t("tui.command.status.capabilities_title"))
	sb.WriteString("\n")
	sb.WriteString(m.t(
		"tui.command.status.trajectory",
		policy.Trajectory.ShowList,
		policy.Trajectory.ShowDetail,
		policy.Trajectory.AllowResume,
		policy.Trajectory.AllowRollback,
	))
	sb.WriteString("\n")
	sb.WriteString(m.t(
		"tui.command.status.memory",
		policy.Memory.ShowQuery,
		policy.Memory.AllowSave,
	))
	sb.WriteString("\n")
	sb.WriteString(m.t(
		"tui.command.status.mcp",
		policy.MCP.Show,
		policy.MCP.ReadOnly,
		policy.MCP.AllowManage,
		policy.MCP.AllowInvoke,
	))
	sb.WriteString("\n")
	sb.WriteString(m.t(
		"tui.command.status.browser",
		policy.Browser.ShowRead,
		policy.Browser.AllowInteract,
	))

	return sb.String()
}
