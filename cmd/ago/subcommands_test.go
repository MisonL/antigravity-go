package main

import (
	"encoding/json"
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

func TestRenderExecutionSummaryJSONIncludesAggregateFields(t *testing.T) {
	records := []session.ExecutionRecord{
		{
			ID:        "exec-1",
			Reference: "JSON execution",
			Status:    session.ExecutionStatusSuccess,
			UpdatedAt: time.Date(2026, 4, 10, 10, 0, 0, 0, time.UTC),
		},
	}

	output, err := renderExecutionSummaryJSON(records)
	if err != nil {
		t.Fatalf("renderExecutionSummaryJSON returned error: %v", err)
	}

	var payload struct {
		Total            int                       `json:"total"`
		Success          int                       `json:"success"`
		CurrentExecution *session.ExecutionRecord  `json:"current_execution"`
		Executions       []session.ExecutionRecord `json:"executions"`
	}
	if err := json.Unmarshal([]byte(output), &payload); err != nil {
		t.Fatalf("json decode failed: %v", err)
	}
	if payload.Total != 1 || payload.Success != 1 {
		t.Fatalf("unexpected summary payload: %+v", payload)
	}
	if payload.CurrentExecution != nil {
		t.Fatalf("did not expect current execution for success-only payload: %+v", payload.CurrentExecution)
	}
	if len(payload.Executions) != 1 || payload.Executions[0].ID != "exec-1" {
		t.Fatalf("unexpected executions payload: %+v", payload.Executions)
	}
}

func TestRenderExecutionSummaryJSONIncludesCurrentAndFailure(t *testing.T) {
	records := []session.ExecutionRecord{
		{
			ID:        "exec-2",
			Reference: "Running execution",
			Status:    session.ExecutionStatusRunning,
			UpdatedAt: time.Date(2026, 4, 10, 11, 0, 0, 0, time.UTC),
		},
		{
			ID:        "exec-1",
			Reference: "Failed execution",
			Status:    session.ExecutionStatusFailed,
			UpdatedAt: time.Date(2026, 4, 10, 10, 0, 0, 0, time.UTC),
		},
	}

	output, err := renderExecutionSummaryJSON(records)
	if err != nil {
		t.Fatalf("renderExecutionSummaryJSON returned error: %v", err)
	}

	var payload struct {
		Failed           int                      `json:"failed"`
		InProgress       int                      `json:"in_progress"`
		CurrentExecution *session.ExecutionRecord `json:"current_execution"`
		RecentFailure    *session.ExecutionRecord `json:"recent_failure"`
	}
	if err := json.Unmarshal([]byte(output), &payload); err != nil {
		t.Fatalf("json decode failed: %v", err)
	}
	if payload.Failed != 1 || payload.InProgress != 1 {
		t.Fatalf("unexpected summary counters: %+v", payload)
	}
	if payload.CurrentExecution == nil || payload.CurrentExecution.ID != "exec-2" {
		t.Fatalf("unexpected current execution: %+v", payload.CurrentExecution)
	}
	if payload.RecentFailure == nil || payload.RecentFailure.ID != "exec-1" {
		t.Fatalf("unexpected recent failure: %+v", payload.RecentFailure)
	}
}

func TestRenderExecutionDetailJSONIncludesRecordAndTimeline(t *testing.T) {
	record := &session.ExecutionRecord{
		ID:        "exec-1",
		Reference: "JSON detail execution",
		Status:    session.ExecutionStatusSuccess,
	}
	steps := []session.ExecutionStep{{
		ID:     "step-1",
		Title:  "write_file",
		Status: "succeeded",
	}}
	timeline := []session.ExecutionEvent{{
		ID:   "event-1",
		Type: "tool_end",
	}}

	output, err := renderExecutionDetailJSON(record, steps, timeline)
	if err != nil {
		t.Fatalf("renderExecutionDetailJSON returned error: %v", err)
	}

	var payload struct {
		Execution *session.ExecutionRecord `json:"execution"`
		Steps     []session.ExecutionStep  `json:"steps"`
		Timeline  []session.ExecutionEvent `json:"timeline"`
	}
	if err := json.Unmarshal([]byte(output), &payload); err != nil {
		t.Fatalf("json decode failed: %v", err)
	}
	if payload.Execution == nil || payload.Execution.ID != "exec-1" {
		t.Fatalf("unexpected execution payload: %+v", payload.Execution)
	}
	if len(payload.Steps) != 1 || payload.Steps[0].Title != "write_file" {
		t.Fatalf("unexpected steps payload: %+v", payload.Steps)
	}
	if len(payload.Timeline) != 1 || payload.Timeline[0].Type != "tool_end" {
		t.Fatalf("unexpected timeline payload: %+v", payload.Timeline)
	}
}

func TestFilterExecutionRecordsByStatus(t *testing.T) {
	records := []session.ExecutionRecord{
		{ID: "exec-1", Status: session.ExecutionStatusRunning},
		{ID: "exec-2", Status: session.ExecutionStatusSuccess},
		{ID: "exec-3", Status: session.ExecutionStatusRunning},
	}

	filtered := filterExecutionRecords(records, "running")
	if len(filtered) != 2 {
		t.Fatalf("unexpected filtered length: %d", len(filtered))
	}
	for _, record := range filtered {
		if record.Status != session.ExecutionStatusRunning {
			t.Fatalf("unexpected filtered record: %+v", record)
		}
	}
}
