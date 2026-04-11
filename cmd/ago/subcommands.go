package main

import (
	"context"
	"encoding/json"
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
			fmt.Print("  - WAIT 等待内核端口...")
			if err := host.WaitForPort(10 * time.Second); err != nil {
				fmt.Printf("\r  - FAIL 内核端口未在 10 秒内就绪: %v\n", err)
				printRecentCoreLogs(host)
			} else if err := host.WaitReady(30 * time.Second); err != nil {
				fmt.Printf("\r  - FAIL 内核未在 30 秒内完成 RPC 就绪: %v\n", err)
				printRecentCoreLogs(host)
			} else {
				fmt.Printf("\r  - OK 内核就绪 (HTTP:%d, LSP:%d)\n", host.HTTPPort(), host.LSPPort())
				client := rpc.NewClient(host.HTTPPort())
				printCoreCapabilityMatrix(corecap.ProbeCoreCapabilities(client))
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
		printRecentCoreLogs(host)
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
		printRecentCoreLogs(host)
		return
	}

	rpcClient := rpc.NewClient(host.HTTPPort())
	caps := corecap.ProbeCoreCapabilities(rpcClient)
	if !caps.TrajectoryGet.Supported {
		fmt.Print(renderResumeUnsupportedMessage(caps.TrajectoryGet))
		return
	}
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
	jsonF := fs.Bool("json", false, "Render summary as JSON")
	limitF := fs.Int("limit", 10, "Max executions to show")
	statusF := fs.String("status", "", "Filter by execution status")
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
	records = filterExecutionRecords(records, *statusF)

	if *jsonF {
		payload, err := renderExecutionSummaryJSON(records)
		if err != nil {
			fmt.Printf("渲染执行账本 JSON 失败: %v\n", err)
			return
		}
		fmt.Print(payload)
		return
	}

	fmt.Print(renderExecutionSummary(records, *limitF))
}

func runExecution(args []string) {
	fs := flag.NewFlagSet("execution", flag.ExitOnError)
	dataDirF := fs.String("data", "", "Data directory")
	jsonF := fs.Bool("json", false, "Render execution detail as JSON")
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

	if *jsonF {
		payload, err := renderExecutionDetailJSON(record, steps, timeline)
		if err != nil {
			fmt.Printf("渲染执行详情 JSON 失败: %v\n", err)
			return
		}
		fmt.Print(payload)
		return
	}

	fmt.Print(renderExecutionDetail(record, steps, timeline))
}

func runMCP(args []string) {
	parsed, err := parseMCPCommandArgs(args)
	if err != nil {
		fmt.Printf("%v\n", err)
		fmt.Println("用法: ago mcp [list|add|delete|restart|resources] [--json]")
		return
	}

	action := parsed.Action

	cfg, err := config.Load()
	if err != nil || cfg == nil {
		cfg = config.DefaultConfig()
		if err != nil {
			fmt.Printf("[WARN] 读取配置失败: %v\n", err)
		}
	}

	bin, err := resolveCoreBinary(cfg)
	if err != nil {
		fmt.Printf("无法定位 antigravity_core: %v\n", err)
		return
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
		printRecentCoreLogs(host)
		return
	}

	client := rpc.NewClient(host.HTTPPort())
	manager := corecap.NewMcpManager(client)
	caps := corecap.ProbeCoreCapabilities(client)

	switch action {
	case "", "list", "ls", "status":
		servers, err := manager.ListServers()
		if err != nil {
			fmt.Printf("读取 MCP 服务失败: %v\n", err)
			return
		}
		if parsed.JSON {
			payload := map[string]interface{}{
				"capabilities": caps,
				"servers":      servers,
			}
			data, err := json.MarshalIndent(payload, "", "  ")
			if err != nil {
				fmt.Printf("渲染 MCP JSON 失败: %v\n", err)
				return
			}
			fmt.Println(string(data))
			return
		}
		fmt.Print(renderMCPList(servers, corecap.DeriveSurfaceCapabilityPolicy(caps), caps))
	case "add", "upsert":
		if len(parsed.Args) < 2 {
			fmt.Println("用法: ago mcp add <name> <command> [args...]")
			return
		}
		resp, err := manager.UpsertServer(corecap.McpServerSpec{
			Name:    parsed.Args[0],
			Command: parsed.Args[1],
			Args:    append([]string(nil), parsed.Args[2:]...),
		})
		if err != nil {
			fmt.Printf("新增或更新 MCP 服务失败: %v\n", err)
			return
		}
		fmt.Print(renderMCPActionResult("add", parsed.Args[0], resp, parsed.JSON))
	case "delete", "rm":
		if len(parsed.Args) != 1 {
			fmt.Println("用法: ago mcp delete <name>")
			return
		}
		resp, err := manager.DeleteServer(parsed.Args[0])
		if err != nil {
			fmt.Printf("删除 MCP 服务失败: %v\n", err)
			return
		}
		fmt.Print(renderMCPActionResult("delete", parsed.Args[0], resp, parsed.JSON))
	case "restart":
		if len(parsed.Args) != 1 {
			fmt.Println("用法: ago mcp restart <name>")
			return
		}
		resp, err := manager.RestartServer(parsed.Args[0])
		if err != nil {
			fmt.Printf("重启 MCP 服务失败: %v\n", err)
			return
		}
		fmt.Print(renderMCPActionResult("restart", parsed.Args[0], resp, parsed.JSON))
	case "resources":
		if len(parsed.Args) != 1 {
			fmt.Println("用法: ago mcp resources <name>")
			return
		}
		resources, nextPageToken, err := manager.ListResources(parsed.Args[0], "", "")
		if err != nil {
			fmt.Printf("读取 MCP 资源失败: %v\n", err)
			return
		}
		if parsed.JSON {
			data, err := json.MarshalIndent(map[string]interface{}{
				"server":          parsed.Args[0],
				"resources":       resources,
				"next_page_token": nextPageToken,
				"capabilities":    caps,
			}, "", "  ")
			if err != nil {
				fmt.Printf("渲染 MCP 资源 JSON 失败: %v\n", err)
				return
			}
			fmt.Println(string(data))
			return
		}
		fmt.Print(renderMCPResources(parsed.Args[0], resources, nextPageToken, caps))
	default:
		fmt.Println("用法: ago mcp [list|add|delete|restart|resources] [--json]")
	}
}

type mcpCommandArgs struct {
	Action string
	Args   []string
	JSON   bool
}

func parseMCPCommandArgs(args []string) (mcpCommandArgs, error) {
	parsed := mcpCommandArgs{Action: "list"}
	remaining := consumeLeadingMCPFlags(args, &parsed)

	if len(remaining) == 0 {
		return parsed, nil
	}

	action := strings.ToLower(strings.TrimSpace(remaining[0]))
	if action == "" {
		return parsed, nil
	}

	if strings.HasPrefix(action, "-") {
		return mcpCommandArgs{}, fmt.Errorf("未知参数: %s", remaining[0])
	}

	parsed.Action = action
	rest := remaining[1:]

	switch action {
	case "", "list", "ls", "status":
		for _, arg := range rest {
			if isMCPJSONFlag(arg) {
				parsed.JSON = true
				continue
			}
			return mcpCommandArgs{}, fmt.Errorf("未知参数: %s", arg)
		}
	case "delete", "rm", "restart", "resources":
		for _, arg := range rest {
			if isMCPJSONFlag(arg) {
				parsed.JSON = true
				continue
			}
			parsed.Args = append(parsed.Args, arg)
		}
	case "add", "upsert":
		rest = consumeLeadingMCPFlags(rest, &parsed)
		parsed.Args = append(parsed.Args, rest...)
	default:
		parsed.Args = append(parsed.Args, rest...)
	}

	return parsed, nil
}

func consumeLeadingMCPFlags(args []string, parsed *mcpCommandArgs) []string {
	index := 0
	for index < len(args) {
		if !isMCPJSONFlag(args[index]) {
			break
		}
		if parsed != nil {
			parsed.JSON = true
		}
		index++
	}
	return append([]string(nil), args[index:]...)
}

func isMCPJSONFlag(arg string) bool {
	return strings.TrimSpace(arg) == "--json"
}

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

	summary := summarizeExecutionRecords(records)

	var sb strings.Builder
	sb.WriteString("Execution Ledger\n")
	sb.WriteString(fmt.Sprintf(
		"总数: %d  成功: %d  失败: %d  进行中: %d\n\n",
		summary.Total,
		summary.Success,
		summary.Failed,
		summary.InProgress,
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

func renderResumeUnsupportedMessage(probe rpc.MethodProbe) string {
	return fmt.Sprintf(
		"当前内核不支持 ago resume：trajectory_get 未提供（%s）。请先运行 `ago doctor` 核对能力矩阵。\n",
		firstNonEmptyString(probe.Requested, probe.Evidence, "trajectory_get"),
	)
}

func renderMCPCapabilityMessage(policy corecap.SurfaceCapabilityPolicy, caps corecap.CoreCapabilities) string {
	var sb strings.Builder
	sb.WriteString("MCP 状态\n")
	if !policy.MCP.Show {
		sb.WriteString("当前内核未暴露可用的 MCP 能力。请先运行 `ago doctor` 核对能力矩阵。\n")
		return sb.String()
	}

	if policy.MCP.ReadOnly {
		sb.WriteString("当前内核仅支持查看 MCP 服务状态或资源，不支持新增、删除、重启或调用工具。\n")
	} else {
		sb.WriteString("当前内核已暴露 MCP 管理能力。\n")
	}

	sb.WriteString(fmt.Sprintf("- mcp_states: %t\n", caps.McpStates.Supported))
	sb.WriteString(fmt.Sprintf("- mcp_servers: %t\n", caps.McpServers.Supported))
	sb.WriteString(fmt.Sprintf("- mcp_resources: %t\n", caps.McpResources.Supported))
	sb.WriteString(fmt.Sprintf("- mcp_add: %t\n", caps.McpControl.Add.Supported))
	sb.WriteString(fmt.Sprintf("- mcp_refresh: %t\n", caps.McpControl.Refresh.Supported))
	sb.WriteString(fmt.Sprintf("- mcp_restart: %t\n", caps.McpControl.Restart.Supported))
	sb.WriteString(fmt.Sprintf("- mcp_invoke: %t\n", caps.McpControl.Invoke.Supported))
	return sb.String()
}

func renderMCPList(servers []corecap.McpServerInfo, policy corecap.SurfaceCapabilityPolicy, caps corecap.CoreCapabilities) string {
	var sb strings.Builder
	sb.WriteString(renderMCPCapabilityMessage(policy, caps))
	sb.WriteString("\n")
	if len(servers) == 0 {
		sb.WriteString("当前没有 MCP 服务。\n")
		return sb.String()
	}

	sb.WriteString("MCP 服务列表\n")
	for _, server := range servers {
		sb.WriteString(fmt.Sprintf("- %s | %s | tools=%d", server.Name, firstNonEmptyString(server.Status, "unknown"), server.ToolCount))
		if server.Command != "" {
			sb.WriteString(fmt.Sprintf(" | command=%s", server.Command))
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

func renderMCPActionResult(action string, name string, resp map[string]interface{}, asJSON bool) string {
	if asJSON {
		data, err := json.MarshalIndent(resp, "", "  ")
		if err != nil {
			return fmt.Sprintf("渲染 JSON 失败: %v\n", err)
		}
		return string(data) + "\n"
	}

	mode := ""
	if rawMode, ok := resp["operation_mode"].(string); ok {
		mode = rawMode
	}
	if mode != "" {
		return fmt.Sprintf("MCP %s 完成: %s（mode=%s）\n", action, strings.TrimSpace(name), mode)
	}
	return fmt.Sprintf("MCP %s 完成: %s\n", action, strings.TrimSpace(name))
}

func renderMCPResources(serverName string, resources []corecap.McpResourceInfo, nextPageToken string, caps corecap.CoreCapabilities) string {
	var sb strings.Builder
	sb.WriteString(renderMCPCapabilityMessage(corecap.DeriveSurfaceCapabilityPolicy(caps), caps))
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("MCP 资源: %s\n", strings.TrimSpace(serverName)))
	if len(resources) == 0 {
		sb.WriteString("当前没有可枚举的 MCP 资源。\n")
	} else {
		for _, resource := range resources {
			label := firstNonEmptyString(resource.Name, resource.URI)
			sb.WriteString(fmt.Sprintf("- %s", label))
			if resource.URI != "" && resource.URI != label {
				sb.WriteString(fmt.Sprintf(" | uri=%s", resource.URI))
			}
			if resource.MimeType != "" {
				sb.WriteString(fmt.Sprintf(" | mime=%s", resource.MimeType))
			}
			if resource.Description != "" {
				sb.WriteString(fmt.Sprintf(" | %s", resource.Description))
			}
			sb.WriteString("\n")
		}
	}
	if nextPageToken != "" {
		sb.WriteString(fmt.Sprintf("next_page_token: %s\n", nextPageToken))
	}
	return sb.String()
}

func renderExecutionSummaryJSON(records []session.ExecutionRecord) (string, error) {
	summary := summarizeExecutionRecords(records)

	data, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data) + "\n", nil
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

func renderExecutionDetailJSON(record *session.ExecutionRecord, steps []session.ExecutionStep, timeline []session.ExecutionEvent) (string, error) {
	payload := struct {
		Execution *session.ExecutionRecord `json:"execution"`
		Steps     []session.ExecutionStep  `json:"steps"`
		Timeline  []session.ExecutionEvent `json:"timeline"`
	}{
		Execution: record,
		Steps:     steps,
		Timeline:  timeline,
	}

	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data) + "\n", nil
}

type executionSummaryPayload struct {
	GeneratedAt      string                    `json:"generated_at"`
	Total            int                       `json:"total"`
	Success          int                       `json:"success"`
	Failed           int                       `json:"failed"`
	InProgress       int                       `json:"in_progress"`
	CurrentExecution *session.ExecutionRecord  `json:"current_execution,omitempty"`
	RecentFailure    *session.ExecutionRecord  `json:"recent_failure,omitempty"`
	Executions       []session.ExecutionRecord `json:"executions"`
}

func summarizeExecutionRecords(records []session.ExecutionRecord) executionSummaryPayload {
	payload := executionSummaryPayload{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Executions:  records,
		Total:       len(records),
	}

	for i := range records {
		record := records[i]
		switch strings.TrimSpace(record.Status) {
		case session.ExecutionStatusSuccess:
			payload.Success++
		case session.ExecutionStatusFailed, session.ExecutionStatusBlocked, session.ExecutionStatusRolledBack:
			payload.Failed++
			if payload.RecentFailure == nil {
				copy := record
				payload.RecentFailure = &copy
			}
		case session.ExecutionStatusPending, session.ExecutionStatusRunning, session.ExecutionStatusAwaitingApproval, session.ExecutionStatusValidating:
			payload.InProgress++
			if payload.CurrentExecution == nil {
				copy := record
				payload.CurrentExecution = &copy
			}
		}
	}

	return payload
}

func filterExecutionRecords(records []session.ExecutionRecord, status string) []session.ExecutionRecord {
	status = strings.TrimSpace(status)
	if status == "" {
		return records
	}

	filtered := make([]session.ExecutionRecord, 0, len(records))
	for _, record := range records {
		if strings.EqualFold(strings.TrimSpace(record.Status), status) {
			filtered = append(filtered, record)
		}
	}
	return filtered
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
	for _, tool := range coreV2.AvailableTools(corecap.ProbeCoreCapabilities(rpcClient), tools.CoreToolModeBase) {
		baseAgt.RegisterTool(tool)
	}

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

func printCoreCapabilityMatrix(caps corecap.CoreCapabilities) {
	sections := []struct {
		title string
		items []struct {
			name  string
			probe rpc.MethodProbe
		}
	}{
		{
			title: "基础平面",
			items: []struct {
				name  string
				probe rpc.MethodProbe
			}{
				{name: "heartbeat", probe: caps.Heartbeat},
				{name: "run_command", probe: caps.RunCommand},
				{name: "repo_info", probe: caps.RepoInfo},
				{name: "rules", probe: caps.Rules},
				{name: "experiments", probe: caps.Experiments},
				{name: "workspace_track", probe: caps.WorkspaceTrack},
			},
		},
		{
			title: "工程平面",
			items: []struct {
				name  string
				probe rpc.MethodProbe
			}{
				{name: "diagnostics", probe: caps.Diagnostics},
				{name: "validation", probe: caps.Validation},
				{name: "edit_preview", probe: caps.EditPreview},
				{name: "apply_edit", probe: caps.ApplyEdit},
				{name: "commit_message", probe: caps.CommitMessage},
				{name: "code_frequency", probe: caps.CodeFrequency},
				{name: "rollback", probe: caps.Rollback},
			},
		},
		{
			title: "浏览器与轨迹",
			items: []struct {
				name  string
				probe rpc.MethodProbe
			}{
				{name: "browser_list", probe: caps.BrowserList},
				{name: "browser_open", probe: caps.BrowserOpen},
				{name: "browser_focus", probe: caps.BrowserFocus},
				{name: "browser_screenshot", probe: caps.BrowserScreenshot},
				{name: "browser_click", probe: caps.BrowserClick},
				{name: "browser_type", probe: caps.BrowserType},
				{name: "browser_scroll", probe: caps.BrowserScroll},
				{name: "trajectory_list", probe: caps.TrajectoryList},
				{name: "trajectory_get", probe: caps.TrajectoryGet},
				{name: "trajectory_export", probe: caps.TrajectoryExport},
				{name: "memory_query", probe: caps.MemoryQuery},
				{name: "memory_save", probe: caps.MemorySave},
			},
		},
		{
			title: "MCP 平面",
			items: []struct {
				name  string
				probe rpc.MethodProbe
			}{
				{name: "mcp_states", probe: caps.McpStates},
				{name: "mcp_servers", probe: caps.McpServers},
				{name: "mcp_resources", probe: caps.McpResources},
				{name: "mcp_setting", probe: caps.McpSetting},
				{name: "mcp_enabled", probe: caps.McpEnabled},
				{name: "mcp_add", probe: caps.McpControl.Add},
				{name: "mcp_refresh", probe: caps.McpControl.Refresh},
				{name: "mcp_restart", probe: caps.McpControl.Restart},
				{name: "mcp_invoke", probe: caps.McpControl.Invoke},
			},
		},
	}

	fmt.Println("\n[CORE] 能力矩阵:")
	for _, section := range sections {
		fmt.Printf("  - %s\n", section.title)
		for _, item := range section.items {
			status := "UNSUPPORTED"
			if item.probe.Supported {
				status = "SUPPORTED"
			}
			fmt.Printf("    - %-18s %-11s %s\n", item.name, status, firstNonEmptyString(item.probe.Requested, item.probe.Evidence))
		}
	}
}

func printRecentCoreLogs(host *core.Host) {
	if host == nil {
		return
	}
	logs := host.Logs()
	if len(logs) == 0 {
		return
	}

	start := 0
	if len(logs) > 20 {
		start = len(logs) - 20
	}
	fmt.Println("最近内核日志:")
	for _, line := range logs[start:] {
		fmt.Printf("  %s\n", line)
	}
}
