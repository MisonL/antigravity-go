// agy is the Antigravity Go CLI Agent.
package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mison/antigravity-go/internal/agent"
	"github.com/mison/antigravity-go/internal/config"
	"github.com/mison/antigravity-go/internal/core"
	"github.com/mison/antigravity-go/internal/llm"
	"github.com/mison/antigravity-go/internal/rpc"
	"github.com/mison/antigravity-go/internal/server"
	"github.com/mison/antigravity-go/internal/session"
	"github.com/mison/antigravity-go/internal/tools"
	"github.com/mison/antigravity-go/internal/tui"
)

func main() {
	// 子命令优先（符合设计文档：agy doctor/run/review/auto-fix/resume/mcp）
	if len(os.Args) > 1 && !strings.HasPrefix(os.Args[1], "-") {
		switch os.Args[1] {
		case "doctor":
			runDoctor(os.Args[2:])
			return
		case "run":
			runOnce(os.Args[2:])
			return
		case reviewCmd:
			runReview(os.Args[2:])
			return
		case autoFixCmd:
			runAutoFix(os.Args[2:])
			return
		case initCmd:
			runInit(os.Args[2:])
			return
		case deployCmd:
			runDeploy(os.Args[2:])
			return
		case "resume":
			runResume(os.Args[2:])
			return
		case "models":
			runModels(os.Args[2:])
			return
		case "mcp":
			runMCP(os.Args[2:])
			return
		}
	}

	startupOpts := parseStartupOptions(os.Args[1:])
	cfg, report, err := prepareStartupConfig(startupOpts)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	for _, message := range report.Messages {
		fmt.Println(message)
	}

	// Flags (override config)
	binPathF := flag.String("bin", "", "antigravity_core 二进制路径")
	dataDirF := flag.String("data", "", "数据目录")
	safeStartF := flag.Bool("safe-start", false, "忽略损坏的用户数据并使用隔离的数据目录启动")
	autoRepairF := flag.Bool("auto-repair", false, "备份损坏的数据目录或配置文件后自动重建")
	noTUI := flag.Bool("no-tui", false, "禁用 TUI（纯后台模式）")
	webMode := flag.Bool("web", false, "启动 Web Dashboard")
	modelF := flag.String("model", "", "模型名称")
	providerF := flag.String("provider", "", "LLM Provider：openai / gemini / anthropic / ollama / lmstudio")
	baseURLF := flag.String("base-url", "", "自定义 API Base URL (例如 https://api.example.com/v1)")
	maxOutputF := flag.Int("max-output", 0, "最大输出 token（0 表示使用默认/自动适配）")
	webHostF := flag.String("web-host", "", "Web 监听地址（默认读取配置）")
	portF := flag.Int("port", 0, "Web 端口")
	tokenF := flag.String("token", "", "Web 访问 token（可选）")
	approvalsF := flag.String("approvals", "", "权限策略：read-only / prompt / full")

	flag.Parse()
	_ = *safeStartF
	_ = *autoRepairF

	// Merge flags into config
	if *binPathF != "" {
		cfg.CoreBinPath = *binPathF
	}
	if *dataDirF != "" {
		cfg.DataDir = *dataDirF
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
	if *maxOutputF != 0 {
		cfg.MaxOutput = *maxOutputF
	}
	if *portF != 0 {
		cfg.WebPort = *portF
	}
	if *webHostF != "" {
		cfg.WebHost = *webHostF
	}
	if *tokenF != "" {
		cfg.AuthToken = *tokenF
	}
	if *approvalsF != "" {
		cfg.Approvals = *approvalsF
	}

	cfg.Approvals = strings.ToLower(strings.TrimSpace(cfg.Approvals))
	switch cfg.Approvals {
	case "", "prompt":
		cfg.Approvals = "prompt"
	case "read-only", "readonly", "read_only":
		cfg.Approvals = "read-only"
	case "full":
		cfg.Approvals = "full"
	default:
		fmt.Fprintf(os.Stderr, "[WARN] 未知 approvals 模式: %q (已回退为 prompt)\n", cfg.Approvals)
		cfg.Approvals = "prompt"
	}

	// Resolve binary path
	bin, err := resolveCoreBinary(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误：无法定位 antigravity_core：%v\n", err)
		fmt.Fprintln(os.Stderr, "提示：可用 `--bin` 指定路径")
		os.Exit(1)
	}
	cfg.CoreBinPath = bin

	// Create data directory
	if err := os.MkdirAll(cfg.DataDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "创建数据目录失败: %v\n", err)
		os.Exit(1)
	}

	// Start Core Host
	fmt.Println("[INFO] 启动 Antigravity Go...")
	host := core.NewHost(core.Config{
		BinPath: bin,
		DataDir: cfg.DataDir,
	})

	coreDegraded := false
	if err := host.Start(); err != nil {
		if *webMode || *noTUI {
			coreDegraded = true
			fmt.Fprintf(os.Stderr, "[WARN] 启动 Core 失败，Web 将以降级模式启动: %v\n", err)
		} else {
			fmt.Fprintf(os.Stderr, "启动 Core 失败: %v\n", err)
			os.Exit(1)
		}
	}
	defer func() {
		if err := host.Stop(); err != nil {
			fmt.Fprintf(os.Stderr, "[WARN] 停止 Core 失败: %v\n", err)
		}
	}()

	httpPort := 0
	var client *rpc.Client
	coreReady := false
	if !coreDegraded {
		// Wait for port (critical for liveness check)
		fmt.Println("[INFO] 等待 Core 端口...")
		if err := host.WaitForPort(10 * time.Second); err != nil {
			if *webMode || *noTUI {
				coreDegraded = true
				fmt.Fprintf(os.Stderr, "[WARN] 等待 Core 端口失败，Web 将以降级模式启动: %v\n", err)
				dumpLogs(host)
			} else {
				fmt.Fprintf(os.Stderr, "等待 Core 端口失败: %v\n", err)
				dumpLogs(host)
				os.Exit(1)
			}
		}
	}

	if !coreDegraded {
		// Create RPC client immediately to keep Core alive
		httpPort = host.HTTPPort()
		fmt.Printf("[INFO] 连接 Core (端口 %d)...\n", httpPort)
		client = rpc.NewClient(httpPort)

		// Wait for ready (server might need client connection to become ready)
		fmt.Println("[INFO] 等待 Core 初始化...")
		if err := host.WaitReady(30 * time.Second); err != nil {
			if *webMode || *noTUI {
				coreDegraded = true
				client = nil
				httpPort = 0
				fmt.Fprintf(os.Stderr, "[WARN] 等待 Core 就绪失败，Web 将以降级模式启动: %v\n", err)
				dumpLogs(host)
			} else {
				fmt.Fprintf(os.Stderr, "等待 Core 就绪失败: %v\n", err)
				dumpLogs(host)
				os.Exit(1)
			}
		} else {
			coreReady = true
			fmt.Println("[OK] Core 已就绪")
		}
	}

	// Initialize Agent
	var baseAgt *agent.Agent
	var agt *agent.Agent // TUI Agent
	var lspMgr *tools.LSPManager
	var permReqChan chan tui.PermissionRequest

	// Check provider
	var provider llm.Provider

	// Setup Wizard if needed
	if shouldRunSetupWizard(cfg, *noTUI) {
		fmt.Println("[INFO] 检测到首次运行，启动初始化向导...")
		setup := tui.NewSetupModel(cfg.Provider)
		p := tea.NewProgram(setup)
		m, err := p.Run()
		if err != nil {
			fmt.Fprintf(os.Stderr, "运行初始化向导失败: %v\n", err)
			os.Exit(1)
		}

		finalModel := m.(tui.SetupModel)
		if finalModel.Quitting || (llm.ProviderNeedsAPIKey(finalModel.Provider) && finalModel.APIKey == "") {
			fmt.Println("初始化已取消，退出。")
			os.Exit(0)
		}

		// Update config
		cfg.Provider = finalModel.Provider
		cfg.APIKey = finalModel.APIKey
		cfg.BaseURL = finalModel.BaseURL

		if err := cfg.Save(); err != nil {
			fmt.Fprintf(os.Stderr, "[WARN] 保存配置失败: %v\n", err)
		} else {
			fmt.Printf("[OK] 已保存配置到 %s\n", config.ConfigPathForDataDir(cfg.DataDir))
		}

	}

	provider, err = buildProvider(cfg)
	if err != nil {
		fmt.Printf("[WARN] Provider 初始化失败: %v (Agent 已禁用)\n", err)
	} else {
		baseURL := llm.ResolveProviderBaseURL(cfg.Provider, cfg.BaseURL)

		if baseURL != "" {
			fmt.Printf("[OK] Provider 已激活 [Provider: %s | Model: %s | URL: %s]\n", cfg.Provider, cfg.Model, baseURL)
		} else {
			fmt.Printf("[OK] Provider 已激活 [Provider: %s | Model: %s]\n", cfg.Provider, cfg.Model)
		}
	}
	// Always initialize LSP Manager (even if no provider, Web Dashboard might need it)
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}
	if coreReady && client != nil {
		if err := trackWorkspaceRoot(client, cwd); err != nil {
			fmt.Fprintf(os.Stderr, "注册工作区失败: %v\n", err)
			os.Exit(1)
		}
		host.SetOnRestart(func(info core.RestartInfo) error {
			client.SetPort(info.HTTPPort)
			return trackWorkspaceRoot(client, cwd)
		})
		lspMgr = tools.NewLSPManager(host, cwd)
	}

	if provider != nil && coreReady && client != nil && lspMgr != nil {
		baseAgt = buildBaseAgent(cfg, provider, host, client, lspMgr, cwd)

		// TUI Agent（独立历史 + 可动态切换 approvals）
		agt = baseAgt.CloneWithPrompt(baseAgt.GetSystemPrompt())
		agt.RegisterTool(agt.GetSpecialistTool())
		permReqChan = make(chan tui.PermissionRequest)
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

		fmt.Printf("[OK] Agent 已就绪 (approvals=%s)\n", cfg.Approvals)
	}

	// Determine mode
	var srv *server.Server
	if *webMode || *noTUI {
		// Web Dashboard
		srv = server.NewServer(cfg, host, baseAgt, lspMgr, client)
		errCh := make(chan error, 1)
		go func() {
			errCh <- srv.Start()
		}()
		go func() {
			if err := <-errCh; err != nil && err != http.ErrServerClosed {
				fmt.Fprintf(os.Stderr, "启动 Web Server 失败: %v\n", err)
				os.Exit(1)
			}
		}()
		fmt.Printf("[INFO] Web 控制台: http://%s:%d\n", cfg.WebHost, cfg.WebPort)
	}

	if *noTUI {
		fmt.Println()
		fmt.Println("Antigravity Go 正在以无 TUI 模式运行")
		if coreReady {
			fmt.Printf("   Core HTTP 端口: %d\n", httpPort)
			fmt.Printf("   Core LSP 端口: %d\n", host.LSPPort())
		} else {
			fmt.Println("   Core 状态: 未就绪（Web 降级模式）")
		}
		fmt.Println("   按 Ctrl+C 退出")

		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		sig := <-sigChan
		fmt.Printf("\n收到信号 %v，正在退出...\n", sig)
		if srv != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			if err := srv.Stop(ctx); err != nil {
				fmt.Fprintf(os.Stderr, "[WARN] 停止 Web Server 失败: %v\n", err)
			}
			cancel()
		}
		if err := host.Stop(); err != nil {
			fmt.Fprintf(os.Stderr, "[WARN] 停止 Core 失败: %v\n", err)
		}
		os.Exit(0)
	}

	// Start TUI
	var tuiRec *session.Recorder
	sessionsRoot := filepath.Join(cfg.DataDir, "sessions")
	if err := os.MkdirAll(sessionsRoot, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "[WARN] 创建会话目录失败: %v\n", err)
	}
	if r, err := session.New(sessionsRoot, session.Metadata{
		WorkspaceRoot: cwd,
		Interface:     "tui",
		Approvals:     cfg.Approvals,
		Provider:      cfg.Provider,
		Model:         cfg.Model,
	}); err == nil {
		tuiRec = r
		defer func() {
			if err := tuiRec.Close(); err != nil {
				fmt.Fprintf(os.Stderr, "[WARN] 关闭会话记录失败: %v\n", err)
			}
		}()
		fmt.Printf("[INFO] Session: %s\n", tuiRec.Meta.ID)
	}
	if agt != nil && tuiRec != nil {
		agt.SetToolCallback(func(event, name, args, result string) {
			if err := tuiRec.Append("tool_"+event, map[string]string{
				"name":   name,
				"args":   args,
				"result": result,
			}); err != nil {
				fmt.Fprintf(os.Stderr, "[WARN] 记录工具事件失败: %v\n", err)
			}
		})
	}

	model := tui.NewModel(host, client, agt, permReqChan, cfg.Approvals, tuiRec)
	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "运行 TUI 失败: %v\n", err)
		os.Exit(1)
	}
}

func dumpLogs(host *core.Host) {
	fmt.Println("\n最近日志:")
	for _, line := range host.Logs() {
		fmt.Println("  ", line)
	}
}
func resolveCoreBinary(cfg *config.Config) (string, error) {
	candidates := []string{}
	if cfg != nil && strings.TrimSpace(cfg.CoreBinPath) != "" {
		candidates = append(candidates, cfg.CoreBinPath)
	}
	candidates = append(candidates, "./antigravity_core")

	seen := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		if _, ok := seen[candidate]; ok {
			continue
		}
		seen[candidate] = struct{}{}

		info, err := os.Stat(candidate)
		if err == nil && !info.IsDir() {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("未找到 antigravity_core，可检查路径: %s", strings.Join(candidates, ", "))
}

func shouldRunSetupWizard(cfg *config.Config, noTUI bool) bool {
	if noTUI || cfg == nil {
		return false
	}
	if !llm.ProviderNeedsAPIKey(cfg.Provider) {
		return false
	}
	if strings.TrimSpace(cfg.APIKey) != "" {
		return false
	}
	return strings.TrimSpace(providerEnvAPIKey(cfg.Provider)) == ""
}

func providerEnvAPIKey(provider string) string {
	switch llm.NormalizeProviderName(provider) {
	case "openai", "ollama", "lmstudio":
		return strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	case "anthropic":
		return strings.TrimSpace(os.Getenv("ANTHROPIC_API_KEY"))
	case "gemini":
		return strings.TrimSpace(os.Getenv("GEMINI_API_KEY"))
	case "iflow":
		return strings.TrimSpace(os.Getenv("IFLOW_API_KEY"))
	default:
		return ""
	}
}
