package main

import (
	"strings"
	"testing"
	"time"

	"github.com/mison/antigravity-go/internal/session"
)

func TestRenderExecutionSummaryIncludesCountsAndRows(t *testing.T) {
	records := []session.ExecutionRecord{
		{
			ID:        "exec-2",
			Reference: "Second execution",
			Status:    session.ExecutionStatusRunning,
			UpdatedAt: time.Date(2026, 4, 9, 12, 0, 0, 0, time.UTC),
		},
		{
			ID:        "exec-1",
			Reference: "First execution",
			Status:    session.ExecutionStatusSuccess,
			UpdatedAt: time.Date(2026, 4, 9, 11, 0, 0, 0, time.UTC),
		},
	}

	output := renderExecutionSummary(records, 10)
	for _, want := range []string{"Execution Ledger", "总数: 2", "exec-2", "Second execution"} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected summary to contain %q, got %q", want, output)
		}
	}
}

func TestRenderExecutionDetailIncludesStepsAndTimeline(t *testing.T) {
	record := &session.ExecutionRecord{
		ID:                 "exec-1",
		Reference:          "Detail execution",
		Status:             session.ExecutionStatusSuccess,
		RollbackPoint:      "step-2",
		LatestCheckpointID: "step-2",
		UpdatedAt:          time.Date(2026, 4, 9, 12, 0, 0, 0, time.UTC),
	}
	steps := []session.ExecutionStep{{
		ID:         "step-1",
		Title:      "write_file",
		Status:     "succeeded",
		FinishedAt: "2026-04-09T12:00:00Z",
		Summary:    "ok",
	}}
	timeline := []session.ExecutionEvent{{
		ID:      "event-1",
		Time:    time.Date(2026, 4, 9, 11, 59, 0, 0, time.UTC),
		Type:    "tool_start",
		Message: "tool write_file started",
	}}

	output := renderExecutionDetail(record, steps, timeline)
	for _, want := range []string{"Execution exec-1", "Detail execution", "write_file", "tool_start"} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected detail to contain %q, got %q", want, output)
		}
	}
}
