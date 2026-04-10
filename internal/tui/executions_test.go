package tui

import (
	"strings"
	"testing"

	"github.com/mison/antigravity-go/internal/session"
)

func TestExecutionSummaryMarkdownIncludesCountsAndRecords(t *testing.T) {
	store := session.NewExecutionStore(t.TempDir())
	firstID, err := store.CreateTask("First execution", "step-1")
	if err != nil {
		t.Fatalf("CreateTask(first) returned error: %v", err)
	}
	if err := store.UpdateTask(firstID, session.ExecutionStatusSuccess, "ok", "step-2"); err != nil {
		t.Fatalf("UpdateTask(first) returned error: %v", err)
	}

	secondID, err := store.CreateTask("Second execution", "step-3")
	if err != nil {
		t.Fatalf("CreateTask(second) returned error: %v", err)
	}
	if err := store.UpdateTask(secondID, session.ExecutionStatusRunning, "running", "step-4"); err != nil {
		t.Fatalf("UpdateTask(second) returned error: %v", err)
	}

	model := Model{executions: store}
	markdown, err := model.executionSummaryMarkdown(10)
	if err != nil {
		t.Fatalf("executionSummaryMarkdown returned error: %v", err)
	}
	if !strings.Contains(markdown, secondID) {
		t.Fatalf("expected summary to contain latest execution id, got %q", markdown)
	}
	if !strings.Contains(markdown, "Second execution") {
		t.Fatalf("expected summary to contain execution reference, got %q", markdown)
	}
	if !strings.Contains(markdown, "2") {
		t.Fatalf("expected summary to contain aggregate counts, got %q", markdown)
	}
}

func TestExecutionDetailMarkdownIncludesStepsAndTimeline(t *testing.T) {
	store := session.NewExecutionStore(t.TempDir())
	id, err := store.CreateTask("Detail execution", "step-1")
	if err != nil {
		t.Fatalf("CreateTask returned error: %v", err)
	}
	if err := store.AppendTaskEvent(id, "tool_start", map[string]any{"name": "write_file", "args": "{}"}); err != nil {
		t.Fatalf("AppendTaskEvent(tool_start) returned error: %v", err)
	}
	if err := store.AppendTaskEvent(id, "tool_end", map[string]any{"name": "write_file", "result": "ok"}); err != nil {
		t.Fatalf("AppendTaskEvent(tool_end) returned error: %v", err)
	}
	if err := store.UpdateTask(id, session.ExecutionStatusSuccess, "done", "step-2"); err != nil {
		t.Fatalf("UpdateTask returned error: %v", err)
	}

	model := Model{executions: store}
	markdown, err := model.executionDetailMarkdown(id)
	if err != nil {
		t.Fatalf("executionDetailMarkdown returned error: %v", err)
	}
	for _, want := range []string{"Detail execution", "write_file", "tool_start", "success"} {
		if !strings.Contains(strings.ToLower(markdown), strings.ToLower(want)) {
			t.Fatalf("expected detail markdown to contain %q, got %q", want, markdown)
		}
	}
}
