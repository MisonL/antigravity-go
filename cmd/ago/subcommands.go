package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mison/antigravity-go/internal/agent"
	"github.com/mison/antigravity-go/internal/config"
	"github.com/mison/antigravity-go/internal/core"
	"github.com/mison/antigravity-go/internal/corecap"
	"github.com/mison/antigravity-go/internal/llm"
	"github.com/mison/antigravity-go/internal/rpc"
	"github.com/mison/antigravity-go/internal/session"
	"github.com/mison/antigravity-go/internal/tools"
	"github.com/mison/antigravity-go/internal/tui"
)

const (
	reviewCmd  = "review"
	autoFixCmd = "auto-fix"
)

func runDoctor(args []string) {
	fs := flag.NewFlagSet("doctor", flag.ExitOnError)
	binPathF := fs.String("bin", "", "Path to antigravity_core binary")
	dataDirF := fs.String("data", "", "Data directory")
	_ = fs.Parse(args)

	cfg, err := config.Load()
	if err != nil || cfg == nil {
		cfg = config.DefaultConfig()
		if err != nil {
			fmt.Printf("[WARN] 读取配置失败: %v\n", err)
		}
	}
	if *binPathF != "" {
		cfg.CoreBinPath = *binPathF
	}
	if *dataDirF != "" {
		cfg.DataDir = *dataDirF
	}

	ok := true
	fmt.Println("Antigravity-Go (AGo) Doctor")
	fmt.Printf("- 工作目录: %s\n", mustAbs("."))
	fmt.Printf("- 数据目录: %s\n", cfg.DataDir)

	if err := os.MkdirAll(cfg.DataDir, 0755); err != nil {
		ok = false
		fmt.Printf("- FAIL 数据目录不可写: %v\n", err)
	} else {
		fmt.Println("- OK 数据目录可用")
	}

	bin, err := resolveCoreBinary(cfg)
	if err != nil {
		ok = false
		fmt.Printf("- FAIL antigravity_core: %v\n", err)
	} else {
		fmt.Printf("- OK antigravity_core: %s\n", bin)
	}

	if ok {
		fmt.Println("\nOK 静态检查通过。正在尝试启动内核进行详细自检...")
		host := core.NewHost(core.Config{BinPath: bin, DataDir: cfg.DataDir})
		if err := host.Start(); err != nil {
			fmt.Printf("  - FAIL 启动内核失败: %v\n", err)
		} else {
			defer func() {
				if err := host.Stop(); err != nil {
					fmt.Printf("  - WARN 停止内核失败: %v\n", err)
				}
			}()
			fmt.Print("  - WAIT 等待内核就绪...")
			if err := host.WaitReady(30 * time.Second); err != nil {
				fmt.Printf("\r  - FAIL 内核未在 30 秒内就绪: %v\n", err)
			} else {
				fmt.Printf("\r  - OK 内核就绪 (HTTP:%d, LSP:%d)\n", host.HTTPPort(), host.LSPPort())
				client := rpc.NewClient(host.HTTPPort())
				if status, err := client.GetStaticExperimentStatus(); err == nil {
					fmt.Println("\n[CORE] 实验性功能状态:")
					for _, exp := range status.Experiments {
						statusStr := "disabled"
						if exp.Enabled {
							statusStr = "enabled"
						}
						fmt.Printf("    - %-40s %s\n", exp.ExperimentKey, statusStr)
					}
				}
			}
		}
		fmt.Println("\n自检完成。")
	}
}

func runOnce(args []string) {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	providerF := fs.String("provider", "", "LLM provider")
	modelF := fs.String("model", "", "Model to use")
	baseURLF := fs.String("base-url", "", "Base URL")
	approvalsF := fs.String("approvals", "prompt", "Approval mode")
	_ = fs.Parse(args)

	task := strings.TrimSpace(strings.Join(fs.Args(), " "))
	if task == "" {
		fmt.Println("用法: ago run \"任务描述\"")
		return
	}

	cfg, err := config.Load()
	if err != nil || cfg == nil {
		cfg = config.DefaultConfig()
		if err != nil {
			fmt.Printf("[WARN] 读取配置失败: %v\n", err)
		}
	}
	if *providerF != "" {
		cfg.Provider = *providerF
	}
	if *modelF != "" {
		cfg.Model = *modelF
	}
	if *baseURLF != "" {
		cfg.BaseURL = *baseURLF
	}
	cfg.Approvals = *approvalsF

	provider, err := buildProvider(cfg)
	if err != nil {
		fmt.Printf("Provider 初始化失败: %v\n", err)
		return
	}

	bin, err := resolveCoreBinary(cfg)
	if err != nil {
		fmt.Printf("无法定位 antigravity_core: %v\n", err)
		return
	}

	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}
	host := core.NewHost(core.Config{BinPath: bin, DataDir: cfg.DataDir})
	if err := host.Start(); err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	defer func() {
		if err := host.Stop(); err != nil {
			fmt.Printf("WARN: 停止内核失败: %v\n", err)
		}
	}()

	if err := host.WaitReady(30 * time.Second); err != nil {
		fmt.Printf("Timeout: %v\n", err)
		return
	}

	rpcClient := rpc.NewClient(host.HTTPPort())
	if err := trackWorkspaceRoot(rpcClient, cwd); err != nil {
		fmt.Printf("注册工作区失败: %v\n", err)
		return
	}
	lspMgr := tools.NewLSPManager(host, cwd)
	baseAgt := buildBaseAgent(cfg, provider, host, rpcClient, lspMgr, cwd)

	ctx := context.Background()
	if err := baseAgt.RunStream(ctx, task, func(chunk string, err error) {
		if err == nil {
			fmt.Print(chunk)
		}
	}, nil); err != nil {
		fmt.Printf("运行失败: %v\n", err)
		return
	}
	fmt.Println()
}

func runResume(args []string) {
	fs := flag.NewFlagSet("resume", flag.ExitOnError)
	providerF := fs.String("provider", "", "LLM provider")
	modelF := fs.String("model", "", "Model to use")
	baseURLF := fs.String("base-url", "", "Base URL")
	approvalsF := fs.String("approvals", "prompt", "Approval mode")
	_ = fs.Parse(args)

	if fs.NArg() != 1 {
		fmt.Println("用法: ago resume <trajectory_id>")
		return
	}
	trajectoryID := strings.TrimSpace(fs.Arg(0))
	if trajectoryID == "" {
		fmt.Println("trajectory_id 不能为空")
		return
	}

	cfg, err := config.Load()
	if err != nil || cfg == nil {
		cfg = config.DefaultConfig()
		if err != nil {
			fmt.Printf("[WARN] 读取配置失败: %v\n", err)
		}
	}
	if *providerF != "" {
		cfg.Provider = *providerF
	}
	if *modelF != "" {
		cfg.Model = *modelF
	}
	if *baseURLF != "" {
		cfg.BaseURL = *baseURLF
	}
	cfg.Approvals = *approvalsF

	provider, err := buildProvider(cfg)
	if err != nil {
		fmt.Printf("Provider 初始化失败: %v\n", err)
		return
	}

	bin, err := resolveCoreBinary(cfg)
	if err != nil {
		fmt.Printf("无法定位 antigravity_core: %v\n", err)
		return
	}

	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}

	host := core.NewHost(core.Config{BinPath: bin, DataDir: cfg.DataDir})
	if err := host.Start(); err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	defer func() {
		if err := host.Stop(); err != nil {
			fmt.Printf("WARN: 停止内核失败: %v\n", err)
		}
	}()

	if err := host.WaitReady(30 * time.Second); err != nil {
		fmt.Printf("Timeout: %v\n", err)
		return
	}

	rpcClient := rpc.NewClient(host.HTTPPort())
	snapshot, err := session.LoadTrajectorySnapshot(trajectoryID, corecap.NewTrajectoryManager(rpcClient), cwd)
	if err != nil {
		fmt.Printf("恢复轨迹失败: %v\n", err)
		return
	}

	workspaceRoot := snapshot.WorkspaceRoot
	if workspaceRoot == "" {
		workspaceRoot = cwd
	}

	lspMgr := tools.NewLSPManager(host, workspaceRoot)
	baseAgt := buildBaseAgent(cfg, provider, host, rpcClient, lspMgr, workspaceRoot)
	agt := baseAgt.CloneWithPrompt(baseAgt.GetSystemPrompt())
	agt.RegisterTool(agt.GetSpecialistTool())

	permReqChan := make(chan tui.PermissionRequest)
	switch cfg.Approvals {
	case "full":
		agt.SetPermissionFunc(func(req agent.PermissionRequest) agent.PermissionDecision {
			return agent.PermissionDecision{Allow: true}
		})
	case "read-only":
		agt.SetPermissionFunc(func(req agent.PermissionRequest) agent.PermissionDecision {
			return agent.PermissionDecision{Allow: false}
		})
	default:
		agt.SetPermissionFunc(func(req agent.PermissionRequest) agent.PermissionDecision {
			resChan := make(chan agent.PermissionDecision)
			permReqChan <- tui.PermissionRequest{
				ToolName: req.ToolName,
				Args:     req.Args,
				Response: resChan,
			}
			return <-resChan
		})
	}

	if err := session.RestoreAgentFromSnapshot(agt, corecap.NewWorkspaceManager(rpcClient), snapshot); err != nil {
		fmt.Printf("注入历史消息失败: %v\n", err)
		return
	}

	sessionsRoot := filepath.Join(cfg.DataDir, "sessions")
	if err := os.MkdirAll(sessionsRoot, 0755); err != nil {
		fmt.Printf("创建会话目录失败: %v\n", err)
		return
	}

	rec, err := session.New(sessionsRoot, session.Metadata{
		WorkspaceRoot: workspaceRoot,
		Interface:     "tui",
		Approvals:     cfg.Approvals,
		Provider:      cfg.Provider,
		Model:         cfg.Model,
	})
	if err != nil {
		fmt.Printf("创建恢复会话失败: %v\n", err)
		return
	}
	defer func() {
		if err := rec.Close(); err != nil {
			fmt.Printf("[WARN] 关闭会话记录失败: %v\n", err)
		}
	}()
	if err := rec.SaveMessages(agt.SnapshotMessages()); err != nil {
		fmt.Printf("写入恢复快照失败: %v\n", err)
		return
	}

	agt.SetToolCallback(func(event, name, args, result string) {
		if err := rec.Append("tool_"+event, map[string]string{
			"name":   name,
			"args":   args,
			"result": result,
		}); err != nil {
			fmt.Printf("[WARN] 记录工具事件失败: %v\n", err)
		}
	})

	fmt.Printf("[INFO] 已恢复轨迹: %s\n", trajectoryID)
	fmt.Printf("[INFO] 工作区: %s\n", workspaceRoot)
	fmt.Printf("[INFO] Session: %s\n", rec.Meta.ID)

	model := tui.NewModel(
		host,
		rpcClient,
		agt,
		permReqChan,
		cfg.Approvals,
		rec,
		session.NewExecutionStore(filepath.Join(cfg.DataDir, "executions")),
	)
	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("运行 TUI 失败: %v\n", err)
	}
}

func runExecutions(args []string) {
	fs := flag.NewFlagSet("executions", flag.ExitOnError)
	dataDirF := fs.String("data", "", "Data directory")
	limitF := fs.Int("limit", 10, "Max executions to show")
	_ = fs.Parse(args)

	cfg, err := config.Load()
	if err != nil || cfg == nil {
		cfg = config.DefaultConfig()
	}
	if *dataDirF != "" {
		cfg.DataDir = *dataDirF
	}

	store := session.NewExecutionStore(filepath.Join(cfg.DataDir, "executions"))
	records, err := store.ListExecutions()
	if err != nil {
		fmt.Printf("读取执行账本失败: %v\n", err)
		return
	}

	fmt.Print(renderExecutionSummary(records, *limitF))
}

func runExecution(args []string) {
	fs := flag.NewFlagSet("execution", flag.ExitOnError)
	dataDirF := fs.String("data", "", "Data directory")
	_ = fs.Parse(args)

	if fs.NArg() != 1 {
		fmt.Println("用法: ago execution <execution_id>")
		return
	}
	executionID := strings.TrimSpace(fs.Arg(0))
	if executionID == "" {
		fmt.Println("execution_id 不能为空")
		return
	}

	cfg, err := config.Load()
	if err != nil || cfg == nil {
		cfg = config.DefaultConfig()
	}
	if *dataDirF != "" {
		cfg.DataDir = *dataDirF
	}

	store := session.NewExecutionStore(filepath.Join(cfg.DataDir, "executions"))
	record, err := store.LoadExecution(executionID)
	if err != nil {
		fmt.Printf("读取执行详情失败: %v\n", err)
		return
	}
	steps, err := store.LoadDerivedSteps(executionID)
	if err != nil {
		fmt.Printf("读取执行步骤失败: %v\n", err)
		return
	}
	timeline, err := store.LoadTimeline(executionID)
	if err != nil {
		fmt.Printf("读取执行时间线失败: %v\n", err)
		return
	}

	fmt.Print(renderExecutionDetail(record, steps, timeline))
}

func runMCP(args []string) { fmt.Println("MCP command: use 'ago mcp list' to see states.") }
func runModels(args []string) {
	fmt.Println("Models command: use 'ago models --provider iflow' to see list.")
}

func renderExecutionSummary(records []session.ExecutionRecord, limit int) string {
	if len(records) == 0 {
		return "当前没有执行记录。\n"
	}
	if limit <= 0 || limit > len(records) {
		limit = len(records)
	}

	counts := map[string]int{}
	for _, record := range records {
		counts[strings.TrimSpace(record.Status)]++
	}

	var sb strings.Builder
	sb.WriteString("Execution Ledger\n")
	sb.WriteString(fmt.Sprintf(
		"总数: %d  成功: %d  失败: %d  进行中: %d\n\n",
		len(records),
		counts[session.ExecutionStatusSuccess],
		counts[session.ExecutionStatusFailed]+counts[session.ExecutionStatusBlocked]+counts[session.ExecutionStatusRolledBack],
		counts[session.ExecutionStatusPending]+counts[session.ExecutionStatusRunning]+counts[session.ExecutionStatusAwaitingApproval]+counts[session.ExecutionStatusValidating],
	))

	for _, record := range records[:limit] {
		sb.WriteString(fmt.Sprintf(
			"- %s | %s | %s | %s\n",
			record.ID,
			record.Status,
			formatExecutionTime(record.UpdatedAt),
			record.Reference,
		))
	}

	if limit < len(records) {
		sb.WriteString(fmt.Sprintf("\n还有 %d 条未展示。\n", len(records)-limit))
	}

	return sb.String()
}

func renderExecutionDetail(record *session.ExecutionRecord, steps []session.ExecutionStep, timeline []session.ExecutionEvent) string {
	if record == nil {
		return ""
	}

	sort.Slice(timeline, func(i, j int) bool {
		return timeline[i].Time.Before(timeline[j].Time)
	})

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Execution %s\n", record.ID))
	sb.WriteString(fmt.Sprintf("Reference: %s\n", record.Reference))
	sb.WriteString(fmt.Sprintf("Status: %s\n", record.Status))
	sb.WriteString(fmt.Sprintf("Updated: %s\n", formatExecutionTime(record.UpdatedAt)))
	if strings.TrimSpace(record.RollbackPoint) != "" {
		sb.WriteString(fmt.Sprintf("Rollback Point: %s\n", record.RollbackPoint))
	}
	if strings.TrimSpace(record.LatestCheckpointID) != "" {
		sb.WriteString(fmt.Sprintf("Checkpoint: %s\n", record.LatestCheckpointID))
	}

	sb.WriteString("\nDerived Steps\n")
	if len(steps) == 0 {
		sb.WriteString("- none\n")
	} else {
		for _, step := range steps {
			summary := strings.Join(strings.Fields(strings.TrimSpace(step.Summary)), " ")
			if summary != "" {
				sb.WriteString(fmt.Sprintf(
					"- %s | %s | %s | %s\n",
					step.Title,
					step.Status,
					firstNonEmptyString(step.FinishedAt, step.StartedAt, "-"),
					truncateExecutionLine(summary, 120),
				))
				continue
			}
			sb.WriteString(fmt.Sprintf(
				"- %s | %s | %s\n",
				step.Title,
				step.Status,
				firstNonEmptyString(step.FinishedAt, step.StartedAt, "-"),
			))
		}
	}

	sb.WriteString("\nEvent Timeline\n")
	if len(timeline) == 0 {
		sb.WriteString("- none\n")
	} else {
		for _, event := range timeline {
			message := firstNonEmptyString(strings.TrimSpace(event.Message), strings.TrimSpace(event.Type), "-")
			sb.WriteString(fmt.Sprintf(
				"- %s | %s | %s\n",
				formatExecutionTime(event.Time),
				firstNonEmptyString(strings.TrimSpace(event.Type), "event"),
				truncateExecutionLine(message, 120),
			))
		}
	}

	return sb.String()
}

func formatExecutionTime(ts time.Time) string {
	if ts.IsZero() {
		return "-"
	}
	return ts.UTC().Format(time.RFC3339)
}

func truncateExecutionLine(text string, maxLen int) string {
	if maxLen <= 0 || len(text) <= maxLen {
		return text
	}
	return text[:maxLen-3] + "..."
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func buildProvider(cfg *config.Config) (llm.Provider, error) {
	return llm.BuildProvider(cfg.Provider, cfg.Model, cfg.APIKey, cfg.BaseURL, cfg.MaxOutput)
}

func buildBaseAgent(cfg *config.Config, provider llm.Provider, host *core.Host, rpcClient *rpc.Client, lspMgr *tools.LSPManager, cwd string) *agent.Agent {
	baseAgt := agent.NewAgent(provider, nil, cfg.MaxContext)
	baseAgt.SetTaskStore(session.NewExecutionStore(filepath.Join(cfg.DataDir, "executions")))
	baseAgt.SetWorkspaceContext(agent.WorkspaceContext{
		Root:  cwd,
		Label: "primary",
	})
	baseAgt.RegisterTool(tools.NewRunCommandTool())
	baseAgt.RegisterTool(tools.NewReadDirTool())
	baseAgt.RegisterTool(tools.NewReadFileTool())
	baseAgt.RegisterTool(tools.NewWriteFileTool())
	baseAgt.RegisterTool(tools.NewSearchTool(cwd))
	baseAgt.RegisterTool(tools.NewDeployProjectTool())
	baseAgt.RegisterTool(baseAgt.GetParallelWorkerTool())

	coreV2 := tools.NewCoreV2Manager(rpcClient)
	baseAgt.RegisterTool(coreV2.GetMcpStatesTool())
	baseAgt.RegisterTool(coreV2.ApplyCoreEditTool())
	baseAgt.RegisterTool(coreV2.EditPreviewTool())
	baseAgt.RegisterTool(coreV2.CaptureScreenshotTool())
	baseAgt.RegisterTool(coreV2.BrowserFocusTool())
	baseAgt.RegisterTool(coreV2.GetRepoInfosTool())
	baseAgt.RegisterTool(coreV2.GetCoreDiagnosticsTool())
	baseAgt.RegisterTool(coreV2.GetValidationStatesTool())
	baseAgt.RegisterTool(coreV2.BrowserOpenTool())
	baseAgt.RegisterTool(coreV2.BrowserListTool())
	baseAgt.RegisterTool(coreV2.BrowserClickTool())
	baseAgt.RegisterTool(coreV2.BrowserTypeTool())
	baseAgt.RegisterTool(coreV2.BrowserScrollTool())
	baseAgt.RegisterTool(coreV2.MemorySaveTool())
	baseAgt.RegisterTool(coreV2.MemoryQueryTool())
	baseAgt.RegisterTool(coreV2.TrajectoryListTool())
	baseAgt.RegisterTool(coreV2.TrajectoryGetTool())
	baseAgt.RegisterTool(coreV2.TrajectoryExportTool())
	baseAgt.RegisterTool(coreV2.CommitMessageGenerateTool())
	baseAgt.RegisterTool(coreV2.RollbackToStepTool())
	baseAgt.RegisterTool(coreV2.WorkspaceTrackTool())

	baseAgt.SetLocalizedSystemPrompt("default")
	return baseAgt
}

func trackWorkspaceRoot(client *rpc.Client, cwd string) error {
	manager := corecap.NewWorkspaceManager(client)
	if _, err := manager.Track(cwd); err != nil {
		return fmt.Errorf("track workspace %q: %w", cwd, err)
	}
	return nil
}

func mustAbs(p string) string {
	abs, err := filepath.Abs(p)
	if err != nil {
		return p
	}
	return abs
}
