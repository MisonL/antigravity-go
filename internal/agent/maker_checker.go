package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mison/antigravity-go/internal/llm"
	"github.com/mison/antigravity-go/internal/pkg/pathutil"
	"github.com/mison/antigravity-go/internal/tools"
)

const (
	validationToolName = "get_validation_states"
	runCommandToolName = "run_command"
	rollbackToolName   = "rollback_to_step"
	trajectoryListName = "trajectory_list"
	trajectoryGetName  = "trajectory_get"
	autoReviewHeader   = "[Auto-Review]"
)

type fileSnapshot struct {
	Path    string
	Exists  bool
	Content []byte
}

type rollbackPlan struct {
	StepID   string
	Snapshot *fileSnapshot
}

type makerCheckerReport struct {
	Passed           bool
	Summary          string
	ValidationResult string
	TestCommand      string
	TestOutput       string
}

func requiresPostExecutionApproval(toolName string) bool {
	switch toolName {
	case applyCoreEditToolName, writeFileToolName:
		return true
	default:
		return false
	}
}

func (a *Agent) prepareRollbackPlan(ctx context.Context, toolName string, rawArgs string) rollbackPlan {
	plan := rollbackPlan{}
	switch toolName {
	case applyCoreEditToolName:
		var args struct {
			FilePath string `json:"filePath"`
		}
		if err := json.Unmarshal([]byte(rawArgs), &args); err == nil {
			plan.Snapshot = captureFileSnapshot(args.FilePath)
		}
		plan.StepID = a.captureCurrentStepID(ctx)
	case writeFileToolName:
		var args struct {
			Path string `json:"path"`
		}
		if err := json.Unmarshal([]byte(rawArgs), &args); err == nil {
			plan.Snapshot = captureFileSnapshot(args.Path)
		}
	}
	return plan
}

func captureFileSnapshot(target string) *fileSnapshot {
	if strings.TrimSpace(target) == "" {
		return nil
	}

	safePath, err := pathutil.SanitizePath(".", target)
	if err != nil {
		if filepath.IsAbs(target) {
			safePath = target
		} else {
			return nil
		}
	}

	content, err := os.ReadFile(safePath)
	if err == nil {
		return &fileSnapshot{Path: safePath, Exists: true, Content: content}
	}
	if os.IsNotExist(err) {
		return &fileSnapshot{Path: safePath, Exists: false}
	}
	return nil
}

func (a *Agent) runMakerChecker(ctx context.Context, toolName string, rawArgs string, result string, callback ToolCallback) makerCheckerReport {
	report := makerCheckerReport{
		Passed:      true,
		TestCommand: "go test ./...",
	}

	if validationResult, err := a.executeInternalTool(ctx, validationToolName, "{}", callback); err != nil {
		report.Passed = false
		report.ValidationResult = "Error: " + err.Error()
	} else {
		report.ValidationResult = validationResult
		if validationHasFailure(validationResult) {
			report.Passed = false
		}
	}

	if testOutput, err := a.executeInternalTool(ctx, runCommandToolName, `{"command":"go test ./..."}`, callback); err == nil {
		report.TestOutput = testOutput
		if commandHasFailure(testOutput) {
			report.Passed = false
		}
	} else {
		report.TestCommand = ""
		report.TestOutput = "skipped: " + err.Error()
	}

	report.Summary = a.RunReviewerAssessment(ctx, ReviewerAssessmentInput{
		ToolName:         toolName,
		ToolArgs:         rawArgs,
		ToolResult:       result,
		ValidationResult: report.ValidationResult,
		TestCommand:      report.TestCommand,
		TestOutput:       report.TestOutput,
		Passed:           report.Passed,
	})
	return report
}

func (a *Agent) finalizeSensitiveTool(
	ctx context.Context,
	tc llm.ToolCall,
	result string,
	plan rollbackPlan,
	callback ToolCallback,
	permFunc PermissionFunc,
) (string, error) {
	if !requiresPostExecutionApproval(tc.Name) {
		return result, nil
	}

	report := a.runMakerChecker(ctx, tc.Name, tc.Args, result, callback)
	reportBlock := autoReviewHeader + "\n" + strings.TrimSpace(report.Summary)
	if !report.Passed {
		rollbackMsg := a.rollbackAfterAutoReview(ctx, plan, callback)
		return result + "\n\n" + reportBlock + "\n\n" + rollbackMsg, fmt.Errorf("auto review failed")
	}

	if permFunc != nil && !permFunc(PermissionRequest{
		ToolName: tc.Name,
		Args:     tc.Args,
		Summary:  "机器预审通过，等待人工最终确认。",
		Preview:  reportBlock,
		Metadata: map[string]any{
			"auto_review":  "passed",
			"test_command": report.TestCommand,
		},
	}) {
		a.mu.Lock()
		a.hasDeniedTool = true
		a.mu.Unlock()
		rollbackMsg := a.rollbackAfterAutoReview(ctx, plan, callback)
		return result + "\n\n" + reportBlock + "\n\n" + rollbackMsg, fmt.Errorf("user denied permission")
	}

	return result + "\n\n" + reportBlock, nil
}

func (a *Agent) rollbackAfterAutoReview(ctx context.Context, plan rollbackPlan, callback ToolCallback) string {
	if plan.StepID != "" {
		raw := fmt.Sprintf(`{"step_id":%q}`, plan.StepID)
		if res, err := a.executeInternalTool(ctx, rollbackToolName, raw, callback); err == nil {
			return "已触发 rollback_to_step 恢复工作区。\n" + truncateReviewText(res, 3000)
		}
	}

	if plan.Snapshot == nil || strings.TrimSpace(plan.Snapshot.Path) == "" {
		return "自动回滚失败：缺少可恢复快照。"
	}

	if err := restoreSnapshot(ctx, plan.Snapshot); err != nil {
		return "自动回滚失败：" + err.Error()
	}
	return "已通过文件快照恢复工作区。"
}

func restoreSnapshot(ctx context.Context, snapshot *fileSnapshot) error {
	if snapshot.Exists {
		if err := os.MkdirAll(filepath.Dir(snapshot.Path), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(snapshot.Path, snapshot.Content, 0644); err != nil {
			return err
		}
	} else if err := os.Remove(snapshot.Path); err != nil && !os.IsNotExist(err) {
		return err
	}

	if callback, ok := ctx.Value(tools.FileChangeKey{}).(func(string)); ok {
		callback(snapshot.Path)
	}
	return nil
}

func (a *Agent) executeInternalTool(ctx context.Context, name string, rawArgs string, callback ToolCallback) (string, error) {
	a.mu.RLock()
	tool, exists := a.tools[name]
	a.mu.RUnlock()
	if !exists {
		return "", fmt.Errorf("tool %s is not registered", name)
	}

	if callback != nil {
		callback("start", name, rawArgs, "")
	}
	result, err := tool.Execute(ctx, json.RawMessage(rawArgs))
	if err != nil {
		if callback != nil {
			callback("error", name, rawArgs, err.Error())
		}
		return "", err
	}
	if callback != nil {
		callback("end", name, rawArgs, result)
	}
	return result, nil
}

func (a *Agent) captureCurrentStepID(ctx context.Context) string {
	listRaw, err := a.executeInternalTool(ctx, trajectoryListName, "{}", nil)
	if err != nil {
		return ""
	}

	id := latestTrajectoryID(listRaw)
	if id == "" {
		return ""
	}

	detailRaw, err := a.executeInternalTool(ctx, trajectoryGetName, fmt.Sprintf(`{"id":%q}`, id), nil)
	if err != nil {
		return ""
	}
	return latestStepID(detailRaw)
}

func latestTrajectoryID(raw string) string {
	var payload map[string]any
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return ""
	}

	items := pickObjectList(payload, []string{"trajectories", "items", "data", "results", "records"})
	bestID := ""
	var bestTime time.Time
	for i, item := range items {
		id := firstString(item, "id", "trajectory_id", "trajectoryId", "uuid")
		if id == "" {
			continue
		}
		if i == 0 && bestID == "" {
			bestID = id
		}
		if ts, ok := parseTime(firstString(item, "updated_at", "updatedAt", "created_at", "createdAt", "timestamp")); ok {
			if bestID == "" || ts.After(bestTime) {
				bestID = id
				bestTime = ts
			}
		}
	}
	return bestID
}

func latestStepID(raw string) string {
	var payload map[string]any
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return ""
	}

	steps := pickObjectList(payload, []string{"steps", "nodes", "checkpoints", "timeline", "history", "events"})
	var last string
	for _, step := range steps {
		if id := firstString(step, "step_id", "stepId", "id", "node_id", "nodeId", "checkpoint_id"); id != "" {
			last = id
		}
	}
	return last
}

func pickObjectList(value any, keys []string) []map[string]any {
	switch typed := value.(type) {
	case []any:
		return mapsFromSlice(typed)
	case map[string]any:
		if items := objectValues(typed); len(items) > 0 {
			return items
		}
		for _, key := range keys {
			if items := pickObjectList(typed[key], keys); len(items) > 0 {
				return items
			}
		}
		for _, child := range typed {
			if items := pickObjectList(child, keys); len(items) > 0 {
				return items
			}
		}
	}
	return nil
}

func objectValues(item map[string]any) []map[string]any {
	if len(item) == 0 {
		return nil
	}
	values := make([]any, 0, len(item))
	for _, value := range item {
		values = append(values, value)
	}
	return mapsFromSlice(values)
}

func mapsFromSlice(items []any) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if obj, ok := item.(map[string]any); ok {
			out = append(out, obj)
		}
	}
	return out
}

func firstString(item map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := item[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func parseTime(value string) (time.Time, bool) {
	if value == "" {
		return time.Time{}, false
	}
	ts, err := time.Parse(time.RFC3339, value)
	return ts, err == nil
}

func validationHasFailure(raw string) bool {
	lower := strings.ToLower(raw)
	return strings.Contains(lower, `"status":"failed"`) ||
		strings.Contains(lower, `"status": "failed"`) ||
		strings.Contains(lower, `"state":"failed"`) ||
		strings.Contains(lower, `"state": "failed"`) ||
		strings.Contains(lower, `"severity":"error"`) ||
		strings.Contains(lower, `"severity": "error"`) ||
		strings.Contains(lower, `"ok":false`) ||
		strings.Contains(lower, `"ok": false`)
}

func commandHasFailure(output string) bool {
	lower := strings.ToLower(output)
	return strings.HasPrefix(lower, "error:") ||
		strings.Contains(lower, "\nerror:") ||
		strings.Contains(lower, "\nfail\t") ||
		strings.Contains(lower, "\n--- fail:")
}

func truncateReviewText(s string, limit int) string {
	s = strings.TrimSpace(s)
	if limit <= 0 || len(s) <= limit {
		return s
	}
	return s[:limit] + "..."
}
