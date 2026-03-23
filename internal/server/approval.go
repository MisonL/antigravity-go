package server

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/pmezard/go-difflib/difflib"
)

const approvalWaitTimeout = 5 * time.Minute

type approvalRequestPayload struct {
	ID       string         `json:"id"`
	Tool     string         `json:"tool"`
	Category string         `json:"category"`
	Summary  string         `json:"summary"`
	Args     string         `json:"args"`
	Preview  string         `json:"preview,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

type approvalResponsePayload struct {
	ID    string `json:"id"`
	Allow bool   `json:"allow"`
}

type approvalDecision struct {
	Allow  bool
	Reason string
}

type writeFileApprovalArgs struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type coreEditApprovalArgs struct {
	FilePath string             `json:"filePath"`
	Edits    []coreEditTextEdit `json:"edits"`
}

type coreEditTextEdit struct {
	Range   coreEditRange `json:"range"`
	NewText string        `json:"newText"`
}

type coreEditRange struct {
	Start coreEditPosition `json:"start"`
	End   coreEditPosition `json:"end"`
}

type coreEditPosition struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

type rollbackApprovalArgs struct {
	StepID string `json:"step_id"`
}

func sensitiveApprovalCategory(toolName string) string {
	switch toolName {
	case "apply_core_edit":
		return "workspace_edit"
	case "write_file":
		return "file_write"
	case "rollback_to_step":
		return "trajectory_rollback"
	default:
		return "tool_execution"
	}
}

func buildApprovalRequestPayload(toolName string, rawArgs string, workspaceRoot string) approvalRequestPayload {
	payload := approvalRequestPayload{
		Tool:     toolName,
		Category: sensitiveApprovalCategory(toolName),
		Summary:  fmt.Sprintf("工具 %s 请求执行，需要人工确认。", toolName),
		Args:     truncateApprovalText(prettyJSON(rawArgs), 12000),
		Metadata: map[string]any{},
	}

	switch toolName {
	case "apply_core_edit":
		applyCoreEditApprovalDetails(&payload, rawArgs, workspaceRoot)
	case "write_file":
		applyWriteFileApprovalDetails(&payload, rawArgs, workspaceRoot)
	case "rollback_to_step":
		applyRollbackApprovalDetails(&payload, rawArgs)
	default:
		payload.Preview = payload.Args
	}

	if len(payload.Metadata) == 0 {
		payload.Metadata = nil
	}
	payload.Preview = truncateApprovalText(payload.Preview, 20000)
	return payload
}

func applyCoreEditApprovalDetails(payload *approvalRequestPayload, rawArgs string, workspaceRoot string) {
	var args coreEditApprovalArgs
	if err := json.Unmarshal([]byte(rawArgs), &args); err != nil {
		payload.Summary = "apply_core_edit 参数解析失败，需按原始参数确认。"
		payload.Preview = payload.Args
		payload.Metadata["parse_error"] = err.Error()
		return
	}

	targetPath := normalizeApprovalPath(workspaceRoot, args.FilePath)
	payload.Metadata["file_path"] = targetPath
	payload.Metadata["edit_count"] = len(args.Edits)
	payload.Summary = fmt.Sprintf("请求通过 Core 修改 %s，共 %d 处编辑。", targetPath, len(args.Edits))

	diffText, err := buildCoreEditDiff(args, workspaceRoot)
	if err != nil {
		payload.Preview = payload.Args
		payload.Metadata["preview_error"] = err.Error()
		return
	}
	payload.Preview = diffText
}

func applyWriteFileApprovalDetails(payload *approvalRequestPayload, rawArgs string, workspaceRoot string) {
	var args writeFileApprovalArgs
	if err := json.Unmarshal([]byte(rawArgs), &args); err != nil {
		payload.Summary = "write_file 参数解析失败，需按原始参数确认。"
		payload.Preview = payload.Args
		payload.Metadata["parse_error"] = err.Error()
		return
	}

	targetPath := normalizeApprovalPath(workspaceRoot, args.Path)
	payload.Metadata["path"] = targetPath
	payload.Metadata["content_bytes"] = len(args.Content)
	payload.Summary = fmt.Sprintf("请求写入文件 %s，内容大小 %d 字节。", targetPath, len(args.Content))

	diffText, err := buildWriteFileDiff(args, workspaceRoot)
	if err != nil {
		payload.Preview = truncateApprovalText(args.Content, 12000)
		payload.Metadata["preview_error"] = err.Error()
		return
	}
	payload.Preview = diffText
}

func applyRollbackApprovalDetails(payload *approvalRequestPayload, rawArgs string) {
	var args rollbackApprovalArgs
	if err := json.Unmarshal([]byte(rawArgs), &args); err != nil {
		payload.Summary = "rollback_to_step 参数解析失败，需按原始参数确认。"
		payload.Preview = payload.Args
		payload.Metadata["parse_error"] = err.Error()
		return
	}

	payload.Metadata["step_id"] = args.StepID
	payload.Summary = fmt.Sprintf("请求将工作区回滚到轨迹步骤 %s。", args.StepID)
	payload.Preview = "该操作会把当前工作区恢复到指定轨迹步骤，对现有未提交修改产生直接影响。"
}

func normalizeApprovalPath(workspaceRoot string, rawPath string) string {
	if strings.TrimSpace(rawPath) == "" {
		return rawPath
	}
	if workspaceRoot == "" {
		return filepath.ToSlash(rawPath)
	}
	if filepath.IsAbs(rawPath) {
		if rel, err := filepath.Rel(workspaceRoot, rawPath); err == nil && !strings.HasPrefix(rel, "..") {
			return filepath.ToSlash(rel)
		}
	}
	return filepath.ToSlash(rawPath)
}

func buildWriteFileDiff(args writeFileApprovalArgs, workspaceRoot string) (string, error) {
	targetPath := resolveApprovalTargetPath(workspaceRoot, args.Path)
	before, err := os.ReadFile(targetPath)
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	return buildUnifiedDiff(targetPath, string(before), args.Content)
}

func buildCoreEditDiff(args coreEditApprovalArgs, workspaceRoot string) (string, error) {
	targetPath := resolveApprovalTargetPath(workspaceRoot, args.FilePath)
	before, err := os.ReadFile(targetPath)
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}

	after, err := applyCoreEdits(string(before), args.Edits)
	if err != nil {
		return "", err
	}
	return buildUnifiedDiff(targetPath, string(before), after)
}

func resolveApprovalTargetPath(workspaceRoot string, target string) string {
	if filepath.IsAbs(target) || workspaceRoot == "" {
		return target
	}
	return filepath.Join(workspaceRoot, target)
}

func buildUnifiedDiff(targetPath string, before string, after string) (string, error) {
	displayPath := filepath.ToSlash(targetPath)
	diff, err := difflib.GetUnifiedDiffString(difflib.UnifiedDiff{
		A:        difflib.SplitLines(before),
		B:        difflib.SplitLines(after),
		FromFile: displayPath,
		ToFile:   displayPath,
		Context:  3,
	})
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(diff) == "" {
		return "无文本差异。", nil
	}
	return diff, nil
}

func applyCoreEdits(content string, edits []coreEditTextEdit) (string, error) {
	if len(edits) == 0 {
		return content, nil
	}

	runes := []rune(content)
	offsets := lineStartOffsets(runes)
	type plannedEdit struct {
		start int
		end   int
		text  string
	}

	planned := make([]plannedEdit, 0, len(edits))
	for _, edit := range edits {
		start, err := runeOffsetForPosition(runes, offsets, edit.Range.Start)
		if err != nil {
			return "", err
		}
		end, err := runeOffsetForPosition(runes, offsets, edit.Range.End)
		if err != nil {
			return "", err
		}
		if end < start {
			return "", fmt.Errorf("invalid edit range: end before start")
		}
		planned = append(planned, plannedEdit{
			start: start,
			end:   end,
			text:  edit.NewText,
		})
	}

	sort.Slice(planned, func(i, j int) bool {
		if planned[i].start == planned[j].start {
			return planned[i].end > planned[j].end
		}
		return planned[i].start > planned[j].start
	})

	for _, edit := range planned {
		replacement := []rune(edit.text)
		runes = append(runes[:edit.start], append(replacement, runes[edit.end:]...)...)
	}

	return string(runes), nil
}

func lineStartOffsets(runes []rune) []int {
	offsets := []int{0}
	for idx, r := range runes {
		if r == '\n' {
			offsets = append(offsets, idx+1)
		}
	}
	return offsets
}

func runeOffsetForPosition(runes []rune, offsets []int, pos coreEditPosition) (int, error) {
	if pos.Line < 0 || pos.Character < 0 {
		return 0, fmt.Errorf("invalid negative position")
	}
	if pos.Line >= len(offsets) {
		if pos.Line == len(offsets) && pos.Character == 0 {
			return len(runes), nil
		}
		return 0, fmt.Errorf("line %d out of range", pos.Line)
	}

	lineStart := offsets[pos.Line]
	lineEnd := len(runes)
	if pos.Line+1 < len(offsets) {
		lineEnd = offsets[pos.Line+1]
	}

	if lineEnd > lineStart && runes[lineEnd-1] == '\n' {
		lineEnd--
	}
	if pos.Character > lineEnd-lineStart {
		return 0, fmt.Errorf("character %d out of range for line %d", pos.Character, pos.Line)
	}
	return lineStart + pos.Character, nil
}

func prettyJSON(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "{}"
	}

	var decoded any
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		return raw
	}

	formatted, err := json.MarshalIndent(decoded, "", "  ")
	if err != nil {
		return raw
	}
	return string(formatted)
}

func truncateApprovalText(text string, max int) string {
	if max <= 0 || len(text) <= max {
		return text
	}
	return text[:max] + "\n... [truncated]"
}
