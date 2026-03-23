package server

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/mison/antigravity-go/internal/agent"
	"github.com/mison/antigravity-go/internal/pkg/i18n"
	"github.com/pmezard/go-difflib/difflib"
)

const approvalWaitTimeout = 5 * time.Minute

var unifiedHunkHeaderPattern = regexp.MustCompile(`^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@`)

type approvalRequestPayload struct {
	ID       string          `json:"id"`
	Tool     string          `json:"tool"`
	Category string          `json:"category"`
	Summary  string          `json:"summary"`
	Args     string          `json:"args"`
	Preview  string          `json:"preview,omitempty"`
	Chunks   []approvalChunk `json:"chunks,omitempty"`
	Metadata map[string]any  `json:"metadata,omitempty"`
}

type approvalChunk struct {
	ID      string `json:"id"`
	Header  string `json:"header"`
	Preview string `json:"preview"`
}

type approvalResponsePayload struct {
	ID               string   `json:"id"`
	Allow            bool     `json:"allow"`
	Reason           string   `json:"reason,omitempty"`
	ApprovedChunkIDs []string `json:"approved_chunk_ids,omitempty"`
}

type approvalDecision struct {
	Allow            bool
	Reason           string
	ApprovedChunkIDs []string
	Applied          bool
	Result           string
}

type pendingApproval struct {
	req  agent.PermissionRequest
	plan *approvalExecutionPlan
	ch   chan approvalDecision
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

type approvalExecutionPlan struct {
	toolName    string
	targetPath  string
	displayPath string
	before      string
	after       string
	hunks       []diffHunk
}

type diffHunk struct {
	approvalChunk
	oldStart int
	oldCount int
	newStart int
	newCount int
	lines    []string
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

func buildApprovalRequestPayload(locale string, toolName string, rawArgs string, workspaceRoot string) (approvalRequestPayload, *approvalExecutionPlan) {
	localizer := i18n.MustLocalizer(locale)
	payload := approvalRequestPayload{
		Tool:     toolName,
		Category: sensitiveApprovalCategory(toolName),
		Summary:  localizer.T("server.approval.summary.generic", toolName),
		Args:     truncateApprovalText(prettyJSON(rawArgs), 12000),
		Metadata: map[string]any{},
	}

	var plan *approvalExecutionPlan
	switch toolName {
	case "apply_core_edit":
		plan = applyCoreEditApprovalDetails(localizer, &payload, rawArgs, workspaceRoot)
	case "write_file":
		plan = applyWriteFileApprovalDetails(localizer, &payload, rawArgs, workspaceRoot)
	case "rollback_to_step":
		applyRollbackApprovalDetails(localizer, &payload, rawArgs)
	default:
		payload.Preview = payload.Args
	}

	if len(payload.Metadata) == 0 {
		payload.Metadata = nil
	}
	payload.Preview = truncateApprovalText(payload.Preview, 20000)
	return payload, plan
}

func applyCoreEditApprovalDetails(localizer *i18n.Localizer, payload *approvalRequestPayload, rawArgs string, workspaceRoot string) *approvalExecutionPlan {
	var args coreEditApprovalArgs
	if err := json.Unmarshal([]byte(rawArgs), &args); err != nil {
		payload.Summary = localizer.T("server.approval.summary.parse_core_edit")
		payload.Preview = payload.Args
		payload.Metadata["parse_error"] = err.Error()
		return nil
	}

	targetPath := normalizeApprovalPath(workspaceRoot, args.FilePath)
	payload.Metadata["file_path"] = targetPath
	payload.Metadata["edit_count"] = len(args.Edits)
	payload.Summary = localizer.T("server.approval.summary.core_edit", targetPath, len(args.Edits))

	plan, err := buildCoreEditPlan(localizer, args, workspaceRoot)
	if err != nil {
		payload.Preview = payload.Args
		payload.Metadata["preview_error"] = err.Error()
		return nil
	}
	attachApprovalChunks(localizer, payload, plan)
	return plan
}

func applyWriteFileApprovalDetails(localizer *i18n.Localizer, payload *approvalRequestPayload, rawArgs string, workspaceRoot string) *approvalExecutionPlan {
	var args writeFileApprovalArgs
	if err := json.Unmarshal([]byte(rawArgs), &args); err != nil {
		payload.Summary = localizer.T("server.approval.summary.parse_write_file")
		payload.Preview = payload.Args
		payload.Metadata["parse_error"] = err.Error()
		return nil
	}

	targetPath := normalizeApprovalPath(workspaceRoot, args.Path)
	payload.Metadata["path"] = targetPath
	payload.Metadata["content_bytes"] = len(args.Content)
	payload.Summary = localizer.T("server.approval.summary.write_file", targetPath, len(args.Content))

	plan, err := buildWriteFilePlan(localizer, args, workspaceRoot)
	if err != nil {
		payload.Preview = truncateApprovalText(args.Content, 12000)
		payload.Metadata["preview_error"] = err.Error()
		return nil
	}
	attachApprovalChunks(localizer, payload, plan)
	return plan
}

func attachApprovalChunks(localizer *i18n.Localizer, payload *approvalRequestPayload, plan *approvalExecutionPlan) {
	if plan == nil {
		return
	}
	payload.Preview = buildUnifiedDiffPreview(localizer, plan)
	if len(plan.hunks) == 0 {
		return
	}

	payload.Chunks = make([]approvalChunk, 0, len(plan.hunks))
	for _, hunk := range plan.hunks {
		payload.Chunks = append(payload.Chunks, hunk.approvalChunk)
	}
	if payload.Metadata == nil {
		payload.Metadata = make(map[string]any)
	}
	payload.Metadata["chunk_count"] = len(plan.hunks)
}

func applyRollbackApprovalDetails(localizer *i18n.Localizer, payload *approvalRequestPayload, rawArgs string) {
	var args rollbackApprovalArgs
	if err := json.Unmarshal([]byte(rawArgs), &args); err != nil {
		payload.Summary = localizer.T("server.approval.summary.parse_rollback")
		payload.Preview = payload.Args
		payload.Metadata["parse_error"] = err.Error()
		return
	}

	payload.Metadata["step_id"] = args.StepID
	payload.Summary = localizer.T("server.approval.summary.rollback", args.StepID)
	payload.Preview = localizer.T("server.approval.preview.rollback")
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

func buildWriteFilePlan(localizer *i18n.Localizer, args writeFileApprovalArgs, workspaceRoot string) (*approvalExecutionPlan, error) {
	targetPath := resolveApprovalTargetPath(workspaceRoot, args.Path)
	before, err := os.ReadFile(targetPath)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	return buildApprovalExecutionPlan(localizer, "write_file", targetPath, string(before), args.Content)
}

func buildCoreEditPlan(localizer *i18n.Localizer, args coreEditApprovalArgs, workspaceRoot string) (*approvalExecutionPlan, error) {
	targetPath := resolveApprovalTargetPath(workspaceRoot, args.FilePath)
	before, err := os.ReadFile(targetPath)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	after, err := applyCoreEdits(string(before), args.Edits)
	if err != nil {
		return nil, err
	}
	return buildApprovalExecutionPlan(localizer, "apply_core_edit", targetPath, string(before), after)
}

func buildApprovalExecutionPlan(localizer *i18n.Localizer, toolName string, targetPath string, before string, after string) (*approvalExecutionPlan, error) {
	displayPath := filepath.ToSlash(targetPath)
	diffText, err := buildUnifiedDiff(localizer, targetPath, before, after)
	if err != nil {
		return nil, err
	}

	hunks, err := parseUnifiedDiffHunks(localizer, diffText)
	if err != nil {
		return nil, err
	}

	return &approvalExecutionPlan{
		toolName:    toolName,
		targetPath:  targetPath,
		displayPath: displayPath,
		before:      before,
		after:       after,
		hunks:       hunks,
	}, nil
}

func buildUnifiedDiffPreview(localizer *i18n.Localizer, plan *approvalExecutionPlan) string {
	if plan == nil {
		return ""
	}
	diffText, err := buildUnifiedDiff(localizer, plan.targetPath, plan.before, plan.after)
	if err != nil {
		return ""
	}
	return diffText
}

func resolveApprovalTargetPath(workspaceRoot string, target string) string {
	if filepath.IsAbs(target) || workspaceRoot == "" {
		return target
	}
	return filepath.Join(workspaceRoot, target)
}

func buildUnifiedDiff(localizer *i18n.Localizer, targetPath string, before string, after string) (string, error) {
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
		return localizer.T("server.approval.preview.no_diff"), nil
	}
	return diff, nil
}

func parseUnifiedDiffHunks(localizer *i18n.Localizer, diffText string) ([]diffHunk, error) {
	noDiffText := localizer.T("server.approval.preview.no_diff")
	if strings.TrimSpace(diffText) == "" || strings.TrimSpace(diffText) == strings.TrimSpace(noDiffText) {
		return nil, nil
	}

	lines := splitPreservingNewlines(diffText)
	hunks := make([]diffHunk, 0)
	var current *diffHunk

	appendCurrent := func() {
		if current == nil {
			return
		}
		current.Preview = strings.Join(current.lines, "")
		hunks = append(hunks, *current)
		current = nil
	}

	for _, line := range lines {
		if strings.HasPrefix(line, "@@ ") {
			appendCurrent()

			match := unifiedHunkHeaderPattern.FindStringSubmatch(strings.TrimRight(line, "\n"))
			if len(match) == 0 {
				return nil, fmt.Errorf("invalid unified diff hunk header: %s", strings.TrimSpace(line))
			}

			oldStart := mustParseApprovalInt(match[1])
			oldCount := 1
			if match[2] != "" {
				oldCount = mustParseApprovalInt(match[2])
			}
			newStart := mustParseApprovalInt(match[3])
			newCount := 1
			if match[4] != "" {
				newCount = mustParseApprovalInt(match[4])
			}

			current = &diffHunk{
				approvalChunk: approvalChunk{
					ID:     fmt.Sprintf("chunk-%d", len(hunks)+1),
					Header: strings.TrimRight(line, "\n"),
				},
				oldStart: oldStart,
				oldCount: oldCount,
				newStart: newStart,
				newCount: newCount,
				lines:    []string{line},
			}
			continue
		}

		if current == nil {
			continue
		}
		if len(line) == 0 {
			current.lines = append(current.lines, line)
			continue
		}
		switch line[0] {
		case ' ', '+', '-', '\\':
			current.lines = append(current.lines, line)
		default:
			appendCurrent()
		}
	}

	appendCurrent()
	return hunks, nil
}

func mustParseApprovalInt(raw string) int {
	value := 0
	for _, ch := range raw {
		value = value*10 + int(ch-'0')
	}
	return value
}

func splitPreservingNewlines(text string) []string {
	if text == "" {
		return nil
	}
	parts := strings.SplitAfter(text, "\n")
	if parts[len(parts)-1] == "" {
		return parts[:len(parts)-1]
	}
	return parts
}

func applyApprovedChunks(plan *approvalExecutionPlan, approvedChunkIDs []string, onFileChange func(string)) (string, error) {
	if plan == nil {
		return "", fmt.Errorf("approval execution plan is required")
	}

	approved := make(map[string]struct{}, len(approvedChunkIDs))
	for _, id := range approvedChunkIDs {
		if strings.TrimSpace(id) == "" {
			continue
		}
		approved[id] = struct{}{}
	}

	beforeLines := difflib.SplitLines(plan.before)
	output := make([]string, 0, len(beforeLines))
	nextIndex := 0
	for _, hunk := range plan.hunks {
		startIndex := hunk.oldStart - 1
		if hunk.oldStart == 0 {
			startIndex = 0
		}
		if startIndex < nextIndex {
			startIndex = nextIndex
		}
		if startIndex > len(beforeLines) {
			startIndex = len(beforeLines)
		}

		output = append(output, beforeLines[nextIndex:startIndex]...)
		if _, ok := approved[hunk.ID]; ok {
			output = append(output, hunk.approvedLines()...)
		} else {
			endIndex := startIndex + hunk.oldCount
			if endIndex > len(beforeLines) {
				endIndex = len(beforeLines)
			}
			output = append(output, beforeLines[startIndex:endIndex]...)
		}

		nextIndex = startIndex + hunk.oldCount
		if nextIndex > len(beforeLines) {
			nextIndex = len(beforeLines)
		}
	}
	output = append(output, beforeLines[nextIndex:]...)

	finalContent := strings.Join(output, "")
	if err := os.MkdirAll(filepath.Dir(plan.targetPath), 0755); err != nil {
		return "", err
	}
	if err := os.WriteFile(plan.targetPath, []byte(finalContent), 0644); err != nil {
		return "", err
	}
	if onFileChange != nil {
		onFileChange(plan.targetPath)
	}

	return fmt.Sprintf(
		"Applied %d/%d approved hunks to %s.",
		len(approved),
		len(plan.hunks),
		plan.displayPath,
	), nil
}

func (h diffHunk) approvedLines() []string {
	lines := make([]string, 0, len(h.lines))
	for _, line := range h.lines {
		if line == "" {
			continue
		}
		switch line[0] {
		case ' ', '+':
			lines = append(lines, line[1:])
		}
	}
	return lines
}

func normalizeApprovedChunkIDs(plan *approvalExecutionPlan, approvedChunkIDs []string) []string {
	if plan == nil || len(plan.hunks) == 0 {
		return nil
	}

	valid := make(map[string]struct{}, len(plan.hunks))
	for _, hunk := range plan.hunks {
		valid[hunk.ID] = struct{}{}
	}

	unique := make(map[string]struct{}, len(approvedChunkIDs))
	normalized := make([]string, 0, len(approvedChunkIDs))
	for _, id := range approvedChunkIDs {
		if _, ok := valid[id]; !ok {
			continue
		}
		if _, seen := unique[id]; seen {
			continue
		}
		unique[id] = struct{}{}
		normalized = append(normalized, id)
	}

	sort.SliceStable(normalized, func(i, j int) bool {
		return approvalChunkOrder(plan, normalized[i]) < approvalChunkOrder(plan, normalized[j])
	})
	return normalized
}

func approvalChunkOrder(plan *approvalExecutionPlan, chunkID string) int {
	for index, hunk := range plan.hunks {
		if hunk.ID == chunkID {
			return index
		}
	}
	return len(plan.hunks)
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
