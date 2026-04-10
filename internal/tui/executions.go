package tui

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mison/antigravity-go/internal/session"
)

func (m *Model) executionSummaryMarkdown(limit int) (string, error) {
	if m.executions == nil {
		return "", errors.New(m.t("tui.command.executions.unavailable"))
	}

	records, err := m.executions.ListExecutions()
	if err != nil {
		return "", err
	}
	if len(records) == 0 {
		return m.t("tui.command.executions.empty"), nil
	}

	if limit <= 0 || limit > len(records) {
		limit = len(records)
	}

	counts := map[string]int{}
	for _, record := range records {
		counts[strings.TrimSpace(record.Status)]++
	}

	var sb strings.Builder
	sb.WriteString(m.t("tui.command.executions.summary.title"))
	sb.WriteString("\n")
	sb.WriteString(m.t(
		"tui.command.executions.summary.stats",
		len(records),
		counts[session.ExecutionStatusSuccess],
		counts[session.ExecutionStatusFailed]+counts[session.ExecutionStatusBlocked]+counts[session.ExecutionStatusRolledBack],
		counts[session.ExecutionStatusPending]+counts[session.ExecutionStatusRunning]+counts[session.ExecutionStatusAwaitingApproval]+counts[session.ExecutionStatusValidating],
	))
	sb.WriteString("\n\n")

	for _, record := range records[:limit] {
		sb.WriteString(fmt.Sprintf(
			"- `%s` | %s | %s | %s\n",
			record.ID,
			strings.TrimSpace(record.Status),
			formatExecutionTimestamp(record.UpdatedAt),
			strings.TrimSpace(record.Reference),
		))
	}

	if limit < len(records) {
		sb.WriteString("\n")
		sb.WriteString(m.t("tui.command.executions.summary.more", len(records)-limit))
	}

	return sb.String(), nil
}

func (m *Model) executionDetailMarkdown(id string) (string, error) {
	if m.executions == nil {
		return "", errors.New(m.t("tui.command.executions.unavailable"))
	}
	if strings.TrimSpace(id) == "" {
		return "", errors.New(m.t("tui.command.execution.usage"))
	}

	record, err := m.executions.LoadExecution(id)
	if err != nil {
		return "", err
	}
	steps, err := m.executions.LoadDerivedSteps(id)
	if err != nil {
		return "", err
	}
	timeline, err := m.executions.LoadTimeline(id)
	if err != nil {
		return "", err
	}

	sort.Slice(timeline, func(i, j int) bool {
		return timeline[i].Time.Before(timeline[j].Time)
	})

	var sb strings.Builder
	sb.WriteString(m.t("tui.command.execution.detail.title", record.ID))
	sb.WriteString("\n")
	sb.WriteString(m.t("tui.command.execution.detail.reference", record.Reference))
	sb.WriteString("\n")
	sb.WriteString(m.t("tui.command.execution.detail.status", record.Status))
	sb.WriteString("\n")
	sb.WriteString(m.t("tui.command.execution.detail.updated", formatExecutionTimestamp(record.UpdatedAt)))
	if strings.TrimSpace(record.RollbackPoint) != "" {
		sb.WriteString("\n")
		sb.WriteString(m.t("tui.command.execution.detail.rollback", record.RollbackPoint))
	}
	if strings.TrimSpace(record.LatestCheckpointID) != "" {
		sb.WriteString("\n")
		sb.WriteString(m.t("tui.command.execution.detail.checkpoint", record.LatestCheckpointID))
	}

	sb.WriteString("\n\n")
	sb.WriteString(m.t("tui.command.execution.steps.title"))
	sb.WriteString("\n")
	if len(steps) == 0 {
		sb.WriteString(m.t("tui.command.execution.steps.empty"))
	} else {
		for _, step := range steps {
			summary := strings.TrimSpace(step.Summary)
			if summary != "" {
				summary = trimSingleLine(summary, 120)
				sb.WriteString(fmt.Sprintf("- %s | %s | %s | %s\n", step.Title, step.Status, firstNonEmpty(step.FinishedAt, step.StartedAt, "-"), summary))
				continue
			}
			sb.WriteString(fmt.Sprintf("- %s | %s | %s\n", step.Title, step.Status, firstNonEmpty(step.FinishedAt, step.StartedAt, "-")))
		}
	}

	sb.WriteString("\n")
	sb.WriteString(m.t("tui.command.execution.timeline.title"))
	sb.WriteString("\n")
	if len(timeline) == 0 {
		sb.WriteString(m.t("tui.command.execution.timeline.empty"))
	} else {
		for _, event := range timeline {
			msg := trimSingleLine(firstNonEmpty(strings.TrimSpace(event.Message), event.Type, "-"), 120)
			sb.WriteString(fmt.Sprintf("- %s | %s | %s\n", formatExecutionTimestamp(event.Time), firstNonEmpty(strings.TrimSpace(event.Type), "event"), msg))
		}
	}

	return sb.String(), nil
}

func formatExecutionTimestamp(ts time.Time) string {
	if ts.IsZero() {
		return "-"
	}
	return ts.UTC().Format(time.RFC3339)
}

func trimSingleLine(text string, maxLen int) string {
	text = strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
	if maxLen <= 0 || len(text) <= maxLen {
		return text
	}
	return text[:maxLen-3] + "..."
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
