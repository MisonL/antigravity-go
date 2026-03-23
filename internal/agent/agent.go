package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/mison/antigravity-go/internal/llm"
	"github.com/mison/antigravity-go/internal/pkg/i18n"
	"github.com/mison/antigravity-go/internal/tools"
)

type PermissionFunc func(req PermissionRequest) PermissionDecision

type ToolCallback func(event string, name string, args string, result string)

type TaskStore interface {
	CreateTask(reference, rollbackPoint string) (string, error)
	UpdateTask(id, status, evidence, rollbackPoint string) error
}

type Agent struct {
	mu               sync.RWMutex
	workspaceMu      sync.RWMutex
	provider         llm.Provider
	tools            map[string]tools.Tool
	messages         []llm.Message
	permission       PermissionFunc
	toolCallback     ToolCallback
	systemPrompt     string
	locale           string
	promptProfile    string
	tokenUsage       int
	maxContextTokens int
	hasDeniedTool    bool // Track if any tool call was denied by user in this session
	taskStore        TaskStore
	workspace        WorkspaceContext
}

const (
	toolTurnLimit         = 10
	diagnosticsToolName   = "get_core_diagnostics"
	memorySaveToolName    = "memory_save"
	cseFeedbackHeader     = "[CSE Feedback]"
	writeFileToolName     = "write_file"
	applyCoreEditToolName = "apply_core_edit"
	taskStatusRunning     = "running"
	taskStatusValidating  = "validating"
	taskStatusSuccess     = "success"
	taskStatusFailed      = "failed"
)

func NewAgent(provider llm.Provider, permission PermissionFunc, maxContextTokens int) *Agent {
	if maxContextTokens <= 0 {
		maxContextTokens = 20000 // Default safe limit
	}
	locale := i18n.DetectLocale()
	return &Agent{
		provider:         provider,
		tools:            make(map[string]tools.Tool),
		messages:         []llm.Message{},
		permission:       permission,
		locale:           locale,
		promptProfile:    promptProfileDefault,
		tokenUsage:       0,
		maxContextTokens: maxContextTokens,
	}
}

func (a *Agent) GetSystemPrompt() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.systemPrompt
}

func (a *Agent) SetPermissionFunc(permission PermissionFunc) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.permission = permission
}

func (a *Agent) SetToolCallback(cb ToolCallback) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.toolCallback = cb
}

func (a *Agent) SetTaskStore(store TaskStore) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.taskStore = store
}

func (a *Agent) SetWorkspaceContext(workspace WorkspaceContext) {
	a.workspaceMu.Lock()
	defer a.workspaceMu.Unlock()
	a.workspace = workspace.Clone()
}

func (a *Agent) WorkspaceContext() WorkspaceContext {
	a.workspaceMu.RLock()
	defer a.workspaceMu.RUnlock()
	return a.workspace.Clone()
}

// CloneWithPrompt creates a new agent with same provider and tools, but new system prompt and empty history.
func (a *Agent) CloneWithPrompt(prompt string) *Agent {
	a.mu.RLock()
	defer a.mu.RUnlock()

	toolsCopy := make(map[string]tools.Tool, len(a.tools))
	for k, v := range a.tools {
		toolsCopy[k] = v
	}

	newA := &Agent{
		provider:         a.provider,
		tools:            toolsCopy,
		messages:         []llm.Message{},
		permission:       a.permission,
		toolCallback:     a.toolCallback,
		locale:           a.locale,
		promptProfile:    a.promptProfile,
		maxContextTokens: a.maxContextTokens,
		taskStore:        a.taskStore,
		workspace:        a.WorkspaceContext(),
	}
	newA.setSystemPrompt(prompt, a.promptProfile)
	return newA
}

func (a *Agent) GetTokenUsage() int {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.tokenUsage
}

func (a *Agent) SnapshotMessages() []llm.Message {
	a.mu.RLock()
	defer a.mu.RUnlock()

	out := make([]llm.Message, len(a.messages))
	for i := range a.messages {
		out[i] = a.messages[i]
		if len(a.messages[i].ToolCalls) > 0 {
			out[i].ToolCalls = append([]llm.ToolCall(nil), a.messages[i].ToolCalls...)
		}
	}
	return out
}

func (a *Agent) LoadMessages(msgs []llm.Message) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.messages = make([]llm.Message, len(msgs))
	for i := range msgs {
		a.messages[i] = msgs[i]
		if len(msgs[i].ToolCalls) > 0 {
			a.messages[i].ToolCalls = append([]llm.ToolCall(nil), msgs[i].ToolCalls...)
		}
	}

	// 尽量恢复 system prompt
	if len(a.messages) > 0 && a.messages[0].Role == llm.RoleSystem {
		a.systemPrompt = a.messages[0].Content
	}

	// 重新估算 token 使用量
	a.tokenUsage = estimateTokenUsage(a.messages)
}

// AddUserMessage injects a user message into history without triggering LLM
func (a *Agent) AddUserMessage(content string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	msg := llm.Message{
		Role:    llm.RoleUser,
		Content: content,
	}
	a.messages = append(a.messages, msg)
	a.tokenUsage += estimateMessageUsage(msg)
}

func (a *Agent) Locale() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.locale
}

func (a *Agent) SetLocale(locale string) {
	profile := ""
	a.mu.Lock()
	a.locale = i18n.NormalizeLocale(locale)
	if a.locale == "" {
		a.locale = i18n.DetectLocale()
	}
	profile = a.promptProfile
	a.mu.Unlock()

	if profile != "" && profile != promptProfileCustom {
		a.SetLocalizedSystemPrompt(profile)
	}
}

func (a *Agent) SetLocalizedSystemPrompt(profile string) {
	locale := a.Locale()
	a.setSystemPrompt(SystemPromptForMode(locale, profile), profile)
}

func (a *Agent) SetSystemPrompt(prompt string) {
	a.setSystemPrompt(prompt, promptProfileCustom)
}

func (a *Agent) setSystemPrompt(prompt string, profile string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.systemPrompt = prompt
	a.promptProfile = normalizePromptProfile(profile)
	// Prepend system message if not already present
	if len(a.messages) == 0 || a.messages[0].Role != llm.RoleSystem {
		a.messages = append([]llm.Message{{
			Role:    llm.RoleSystem,
			Content: prompt,
		}}, a.messages...)
	} else {
		// Update existing system message
		a.messages[0].Content = prompt
	}
	a.tokenUsage = estimateTokenUsage(a.messages)
}

func (a *Agent) ClearHistory() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.messages = []llm.Message{}
	a.tokenUsage = 0
}

func (a *Agent) TrimHistory(maxMessages int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.messages) > maxMessages {
		// Keep system prompt if it exists (index 0)
		if len(a.messages) > 0 && a.messages[0].Role == llm.RoleSystem {
			sysMsg := a.messages[0]
			a.messages = append([]llm.Message{sysMsg}, a.messages[len(a.messages)-maxMessages+1:]...)
		} else {
			a.messages = a.messages[len(a.messages)-maxMessages:]
		}
	}
	a.tokenUsage = estimateTokenUsage(a.messages)
}

// CompactHistory 会强制触发一次“上下文压缩”，用于 TUI 的 /compact。
// 这会保留关键决策/用户约束/重要改动点，并将较早的对话折叠成摘要。
func (a *Agent) CompactHistory(ctx context.Context) error {
	if err := a.ensureProvider(); err != nil {
		return err
	}
	return a.summarizeContext(ctx)
}

// manageContext checks if context needs summarization or trimming
func (a *Agent) manageContext(ctx context.Context) error {
	a.mu.RLock()
	usage := a.tokenUsage
	count := len(a.messages)
	limit := a.maxContextTokens
	a.mu.RUnlock()

	if usage < limit && count < 50 {
		return nil
	}

	log.Printf("context management triggered: ~%d tokens, %d messages", usage, count)
	return a.summarizeContext(ctx)
}

func (a *Agent) summarizeContext(ctx context.Context) error {
	a.mu.RLock()
	if len(a.messages) <= 4 {
		a.mu.RUnlock()
		return nil // Too few messages to summarize
	}

	startIndex := 0
	if a.messages[0].Role == llm.RoleSystem {
		startIndex = 1
	}

	endIndex := len(a.messages) - 5
	if endIndex <= startIndex {
		a.mu.RUnlock()
		return nil
	}

	toSummarize := a.messages[startIndex:endIndex]
	recent := a.messages[endIndex:]
	a.mu.RUnlock()

	// Create prompt
	localizer := i18n.MustLocalizer(a.Locale())
	summaryPrompt := localizer.T("agent.summary.request")
	for _, m := range toSummarize {
		summaryPrompt += fmt.Sprintf("%s: %s\n", m.Role, m.Content)
	}

	// Call Provider to generate summary
	msgs := []llm.Message{
		{Role: llm.RoleUser, Content: summaryPrompt},
	}

	log.Println("summarizing conversation history")
	resp, err := a.provider.Chat(ctx, msgs, nil)
	if err != nil {
		return fmt.Errorf("summarization failed: %w", err)
	}

	summary := resp.Content
	log.Printf("conversation summary ready (%d chars)", len(summary))

	// Reconstruct messages
	a.mu.Lock()
	defer a.mu.Unlock()
	newMessages := []llm.Message{}
	if startIndex == 1 {
		newMessages = append(newMessages, a.messages[0]) // Keep System Prompt
	}

	// Add summary as a System message
	newMessages = append(newMessages, llm.Message{
		Role:    llm.RoleSystem,
		Content: localizer.T("agent.summary.prefix", summary),
	})

	// Append recent messages
	newMessages = append(newMessages, recent...)

	a.messages = newMessages

	// Reset token usage estimation (roughly)
	a.tokenUsage = estimateTokenUsage(a.messages)

	return nil
}

func (a *Agent) RegisterTool(t tools.Tool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.tools[t.Definition.Name] = t
}

func (a *Agent) ReplaceToolsByPrefix(prefix string, replacements []tools.Tool) {
	a.mu.Lock()
	defer a.mu.Unlock()

	for name := range a.tools {
		if strings.HasPrefix(name, prefix) {
			delete(a.tools, name)
		}
	}
	for _, tool := range replacements {
		a.tools[tool.Definition.Name] = tool
	}
}

func (a *Agent) GetToolDefinitions() []llm.ToolDefinition {
	a.mu.RLock()
	defer a.mu.RUnlock()
	defs := make([]llm.ToolDefinition, 0, len(a.tools))
	for _, t := range a.tools {
		defs = append(defs, t.Definition)
	}
	return defs
}

func (a *Agent) Run(ctx context.Context, input string, localCallback ToolCallback) (output string, runErr error) {
	if err := a.ensureProvider(); err != nil {
		return "", err
	}
	ctx = a.bindWorkspaceContext(ctx)

	taskRun := a.startTaskRun(ctx, input)
	defer a.finishTaskRun(ctx, taskRun, &output, &runErr)

	// Manage context
	if err := a.manageContext(ctx); err != nil {
		log.Printf("context management warning: %v", err)
		a.TrimHistory(100)
	}

	// Add user message and estimate tokens
	a.mu.Lock()
	msg := llm.Message{
		Role:    llm.RoleUser,
		Content: input,
	}
	a.messages = append(a.messages, msg)
	a.tokenUsage += estimateMessageUsage(msg)
	a.mu.Unlock()

	maxTurns := toolTurnLimit
	for i := 0; i < maxTurns; i++ {
		// Call LLM
		a.mu.RLock()
		msgs := append([]llm.Message(nil), a.messages...)
		a.mu.RUnlock()

		resp, err := a.provider.Chat(ctx, msgs, a.GetToolDefinitions())
		if err != nil {
			return "", err
		}

		a.mu.Lock()
		a.messages = append(a.messages, resp)
		a.tokenUsage += estimateMessageUsage(resp)
		a.mu.Unlock()

		// Check for tool calls
		if len(resp.ToolCalls) == 0 {
			a.markTaskValidating(ctx, taskRun, resp.Content)
			a.FinalizeTask(ctx, input, resp.Content)
			return resp.Content, nil // Final answer
		}

		// Execute tools
		for _, tc := range resp.ToolCalls {
			a.executeToolCall(ctx, tc, localCallback)
		}
	}

	return "", fmt.Errorf("max turns exceeded")
}

func (a *Agent) RunStream(ctx context.Context, input string, cb llm.StreamCallback, localCallback ToolCallback) (runErr error) {
	if err := a.ensureProvider(); err != nil {
		return err
	}
	ctx = a.bindWorkspaceContext(ctx)

	var output string
	taskRun := a.startTaskRun(ctx, input)
	defer a.finishTaskRun(ctx, taskRun, &output, &runErr)

	// Manage context
	if err := a.manageContext(ctx); err != nil {
		log.Printf("context management warning: %v", err)
		a.TrimHistory(100)
	}

	// Add user message and estimate tokens
	a.mu.Lock()
	msg := llm.Message{
		Role:    llm.RoleUser,
		Content: input,
	}
	a.messages = append(a.messages, msg)
	a.tokenUsage += estimateMessageUsage(msg)
	a.mu.Unlock()

	maxTurns := toolTurnLimit
	for i := 0; i < maxTurns; i++ {
		// Call LLM
		a.mu.RLock()
		msgs := append([]llm.Message(nil), a.messages...)
		a.mu.RUnlock()

		resp, err := a.provider.StreamChat(ctx, msgs, a.GetToolDefinitions(), cb)
		if err != nil {
			return err
		}

		a.mu.Lock()
		a.messages = append(a.messages, resp)
		a.tokenUsage += estimateMessageUsage(resp)
		a.mu.Unlock()

		// Check for tool calls
		if len(resp.ToolCalls) == 0 {
			output = resp.Content
			a.markTaskValidating(ctx, taskRun, resp.Content)
			a.FinalizeTask(ctx, input, resp.Content)
			return nil // Final answer done
		}

		// Execute tools
		for _, tc := range resp.ToolCalls {
			a.executeToolCall(ctx, tc, localCallback)
		}
	}

	return fmt.Errorf("max turns exceeded")
}

func (a *Agent) addToolResult(toolCallID, name, content string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if strings.TrimSpace(content) != "" {
		content = i18n.MustLocalizer(a.locale).T("agent.tool.untrusted_output", content)
	}
	msg := llm.Message{
		Role:       llm.RoleTool,
		Content:    content,
		Name:       name,
		ToolCallID: toolCallID,
	}
	a.messages = append(a.messages, msg)
	a.tokenUsage += estimateMessageUsage(msg)
}

func (a *Agent) ensureProvider() error {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.provider == nil {
		return fmt.Errorf("llm provider is not initialized")
	}
	return nil
}

func (a *Agent) executeToolCall(ctx context.Context, tc llm.ToolCall, localCallback ToolCallback) {
	a.mu.RLock()
	tool, exists := a.tools[tc.Name]
	callback := localCallback
	if callback == nil {
		callback = a.toolCallback
	}
	permFunc := a.permission
	a.mu.RUnlock()

	if !exists {
		a.addToolResult(tc.ID, tc.Name, fmt.Sprintf("Error: Tool %s not found", tc.Name))
		return
	}

	log.Printf("tool call: %s(%s)", tc.Name, tc.Args)

	if callback != nil {
		callback("start", tc.Name, tc.Args, "")
	}

	rollbackPlan := a.prepareRollbackPlan(ctx, tc.Name, tc.Args)

	if requiresChunkApproval(tc.Name) {
		a.executeChunkApprovedTool(ctx, tc, tool, rollbackPlan, callback, permFunc)
		return
	}

	if tool.RequiresPermission && permFunc != nil {
		decision := permFunc(PermissionRequest{
			ToolName: tc.Name,
			Args:     tc.Args,
		})
		if !decision.Allow {
			a.mu.Lock()
			a.hasDeniedTool = true
			a.mu.Unlock()

			msg := "Error: User denied permission"
			if strings.TrimSpace(decision.Reason) != "" {
				msg = msg + " (" + decision.Reason + ")"
			}
			a.addToolResult(tc.ID, tc.Name, msg)
			if callback != nil {
				callback("error", tc.Name, tc.Args, msg)
			}
			return
		}
	}

	result, err := tool.Execute(ctx, json.RawMessage(tc.Args))
	if err == nil {
		result, err = a.finalizeSensitiveTool(ctx, tc, result, rollbackPlan, callback)
	}
	if err != nil {
		result = fmt.Sprintf("Error: %v\n\n%s", err, strings.TrimSpace(result))
	} else if shouldRunCSEFeedback(tc.Name) {
		result = a.appendCSEFeedback(ctx, tc.Name, result)
	}

	if callback != nil {
		callback("end", tc.Name, tc.Args, result)
	}

	a.addToolResult(tc.ID, tc.Name, result)
}

func (a *Agent) executeChunkApprovedTool(
	ctx context.Context,
	tc llm.ToolCall,
	tool tools.Tool,
	plan rollbackPlan,
	callback ToolCallback,
	permFunc PermissionFunc,
) {
	result := ""
	var err error

	result, err = tool.Execute(ctx, json.RawMessage(tc.Args))
	if err == nil {
		result, err = a.finalizeSensitiveTool(ctx, tc, result, plan, callback)
	}
	if err == nil && permFunc != nil {
		localizer := i18n.MustLocalizer(a.Locale())
		metadata := map[string]any{}
		if plan.Snapshot != nil && strings.TrimSpace(plan.Snapshot.Path) != "" {
			metadata["approval_target_path"] = plan.Snapshot.Path
			if plan.Snapshot.Exists {
				metadata["approval_before"] = string(plan.Snapshot.Content)
			} else {
				metadata["approval_before"] = ""
			}
		}
		decision := permFunc(PermissionRequest{
			ToolName: tc.Name,
			Args:     tc.Args,
			Summary:  localizer.T("agent.permission.final_confirmation"),
			Preview:  result,
			Metadata: metadata,
		})
		if !decision.Allow {
			a.mu.Lock()
			a.hasDeniedTool = true
			a.mu.Unlock()

			rollbackMsg := a.rollbackAfterAutoReview(ctx, plan, callback)
			msg := "Error: User denied permission"
			if strings.TrimSpace(decision.Reason) != "" {
				msg = msg + " (" + decision.Reason + ")"
			}
			err = fmt.Errorf("%s", msg)
			result = result + "\n\n" + rollbackMsg
		} else if decision.Applied && strings.TrimSpace(decision.Result) != "" {
			result = result + "\n\n" + strings.TrimSpace(decision.Result)
		}
	}
	if err != nil {
		result = fmt.Sprintf("Error: %v\n\n%s", err, strings.TrimSpace(result))
	} else if shouldRunCSEFeedback(tc.Name) {
		result = a.appendCSEFeedback(ctx, tc.Name, result)
	}

	if callback != nil {
		callback("end", tc.Name, tc.Args, result)
	}

	a.addToolResult(tc.ID, tc.Name, result)
}

// FinalizeTask persists a task-level architecture decision summary after a successful run.
// Memory persistence is best-effort and must not block the main response path.
func (a *Agent) FinalizeTask(ctx context.Context, input, output string) {
	a.mu.RLock()
	tool, exists := a.tools[memorySaveToolName]
	messages := append([]llm.Message(nil), a.messages...)
	denied := a.hasDeniedTool
	a.mu.RUnlock()

	if !exists || denied {
		return
	}

	payload := map[string]interface{}{
		"request": buildArchitectureDecisionMemory(input, output, messages),
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		log.Printf("FinalizeTask skipped: marshal payload failed: %v", err)
		return
	}

	if _, err := tool.Execute(ctx, raw); err != nil {
		log.Printf("FinalizeTask skipped: memory save failed: %v", err)
	}
}

type taskRunRecord struct {
	ID            string
	RollbackPoint string
}

func (a *Agent) startTaskRun(ctx context.Context, input string) *taskRunRecord {
	a.mu.RLock()
	store := a.taskStore
	a.mu.RUnlock()
	if store == nil {
		return nil
	}

	record := &taskRunRecord{
		RollbackPoint: a.captureCurrentStepID(ctx),
	}
	id, err := store.CreateTask(input, record.RollbackPoint)
	if err != nil {
		log.Printf("task store create failed: %v", err)
		return nil
	}
	record.ID = id
	a.updateTaskStore(record, taskStatusRunning, "task execution started", record.RollbackPoint)
	return record
}

func (a *Agent) markTaskValidating(ctx context.Context, record *taskRunRecord, output string) {
	if record == nil {
		return
	}

	if rollbackPoint := a.captureCurrentStepID(ctx); rollbackPoint != "" {
		record.RollbackPoint = rollbackPoint
	}
	a.updateTaskStore(record, taskStatusValidating, buildTaskEvidence(a.SnapshotMessages(), output), record.RollbackPoint)
}

func (a *Agent) finishTaskRun(ctx context.Context, record *taskRunRecord, output *string, runErr *error) {
	if record == nil || output == nil || runErr == nil {
		return
	}

	if rollbackPoint := a.captureCurrentStepID(ctx); rollbackPoint != "" {
		record.RollbackPoint = rollbackPoint
	}

	if *runErr != nil {
		a.updateTaskStore(record, taskStatusFailed, buildTaskEvidence(a.SnapshotMessages(), (*runErr).Error()), record.RollbackPoint)
		return
	}
	a.updateTaskStore(record, taskStatusSuccess, buildTaskEvidence(a.SnapshotMessages(), *output), record.RollbackPoint)
}

func (a *Agent) updateTaskStore(record *taskRunRecord, status, evidence, rollbackPoint string) {
	if record == nil || strings.TrimSpace(record.ID) == "" {
		return
	}

	a.mu.RLock()
	store := a.taskStore
	a.mu.RUnlock()
	if store == nil {
		return
	}

	if err := store.UpdateTask(record.ID, status, evidence, rollbackPoint); err != nil {
		log.Printf("task store update failed (%s): %v", status, err)
	}
}

func buildTaskEvidence(messages []llm.Message, finalResult string) string {
	sections := []string{}

	if validation := latestToolOutput(messages, validationToolName); validation != "" {
		sections = append(sections, "validation:\n"+truncateTaskEvidence(validation))
	}
	if testOutput := latestToolOutput(messages, runCommandToolName); testOutput != "" {
		sections = append(sections, "test:\n"+truncateTaskEvidence(testOutput))
	}
	if diagnostics := latestToolOutput(messages, diagnosticsToolName); diagnostics != "" {
		sections = append(sections, "diagnostics:\n"+truncateTaskEvidence(diagnostics))
	}
	if trimmed := strings.TrimSpace(finalResult); trimmed != "" {
		sections = append(sections, "result:\n"+truncateTaskEvidence(trimmed))
	}

	if len(sections) == 0 {
		return ""
	}
	return strings.Join(sections, "\n\n")
}

func latestToolOutput(messages []llm.Message, toolName string) string {
	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
		if msg.Role != llm.RoleTool || msg.Name != toolName {
			continue
		}
		return strings.TrimSpace(msg.Content)
	}
	return ""
}

func truncateTaskEvidence(text string) string {
	text = strings.TrimSpace(text)
	const maxLen = 4000
	if len(text) <= maxLen {
		return text
	}
	return text[:maxLen] + "\n...(truncated)"
}

func shouldRunCSEFeedback(toolName string) bool {
	switch toolName {
	case writeFileToolName, applyCoreEditToolName:
		return true
	default:
		return false
	}
}

func (a *Agent) appendCSEFeedback(ctx context.Context, actuatorName, result string) string {
	diagnostics, err := a.runDiagnosticsFeedback(ctx)
	if err != nil {
		return result + "\n\n" + cseFeedbackHeader + " Failed to fetch diagnostics after " + actuatorName + ": " + err.Error()
	}

	return result + "\n\n" + cseFeedbackHeader + " Diagnostics after " + actuatorName + ":\n" + diagnostics
}

func (a *Agent) runDiagnosticsFeedback(ctx context.Context) (string, error) {
	a.mu.RLock()
	tool, exists := a.tools[diagnosticsToolName]
	a.mu.RUnlock()
	if !exists {
		return "", fmt.Errorf("tool %s is not registered", diagnosticsToolName)
	}

	result, err := tool.Execute(ctx, json.RawMessage("{}"))
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(result) == "" {
		return "(empty diagnostics response)", nil
	}
	return result, nil
}

func buildArchitectureDecisionMemory(input, output string, messages []llm.Message) map[string]interface{} {
	toolNames := collectToolNames(messages)
	decisionSummary := summarizeArchitectureDecision(input, output, toolNames)

	return map[string]interface{}{
		"content": decisionSummary,
		"metadata": map[string]interface{}{
			"category":      "architecture_decision",
			"task_input":    input,
			"task_output":   output,
			"tool_names":    toolNames,
			"captured_at":   time.Now().UTC().Format(time.RFC3339),
			"message_count": len(messages),
		},
	}
}

func collectToolNames(messages []llm.Message) []string {
	seen := make(map[string]struct{})
	names := make([]string, 0)
	for _, msg := range messages {
		for _, call := range msg.ToolCalls {
			if call.Name == "" {
				continue
			}
			if _, ok := seen[call.Name]; ok {
				continue
			}
			seen[call.Name] = struct{}{}
			names = append(names, call.Name)
		}
		if msg.Role == llm.RoleTool && msg.Name != "" {
			if _, ok := seen[msg.Name]; ok {
				continue
			}
			seen[msg.Name] = struct{}{}
			names = append(names, msg.Name)
		}
	}
	return names
}

func summarizeArchitectureDecision(input, output string, toolNames []string) string {
	parts := []string{
		"Task completed successfully and the architecture decision record was captured.",
		"Task input: " + strings.TrimSpace(input),
	}

	if len(toolNames) > 0 {
		parts = append(parts, "Key tools: "+strings.Join(toolNames, ", "))
	}
	if trimmed := strings.TrimSpace(output); trimmed != "" {
		parts = append(parts, "Final result: "+trimmed)
	}

	return strings.Join(parts, "\n")
}

func estimateTokenUsage(messages []llm.Message) int {
	total := 0
	for _, msg := range messages {
		total += estimateMessageUsage(msg)
	}
	return total
}

func estimateMessageUsage(msg llm.Message) int {
	totalLen := len(msg.Content) + len(msg.Name) + len(msg.ToolCallID)
	for _, call := range msg.ToolCalls {
		totalLen += len(call.ID) + len(call.Name) + len(call.Args)
	}
	return totalLen / 4
}
