package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mison/antigravity-go/internal/agent"
	"github.com/mison/antigravity-go/internal/config"
	"github.com/mison/antigravity-go/internal/core"
	"github.com/mison/antigravity-go/internal/corecap"
	"github.com/mison/antigravity-go/internal/llm"
	"github.com/mison/antigravity-go/internal/rpc"
	"github.com/mison/antigravity-go/internal/tools"
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
	fmt.Println("Antigravity Go Doctor")
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
		fmt.Println("用法: agy run \"任务描述\"")
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

func runResume(args []string) { fmt.Println("Resume command is available in TUI mode.") }
func runMCP(args []string)    { fmt.Println("MCP command: use 'agy mcp list' to see states.") }
func runModels(args []string) {
	fmt.Println("Models command: use 'agy models --provider iflow' to see list.")
}

func buildProvider(cfg *config.Config) (llm.Provider, error) {
	return llm.BuildProvider(cfg.Provider, cfg.Model, cfg.APIKey, cfg.BaseURL, cfg.MaxOutput)
}

func buildBaseAgent(cfg *config.Config, provider llm.Provider, host *core.Host, rpcClient *rpc.Client, lspMgr *tools.LSPManager, cwd string) *agent.Agent {
	baseAgt := agent.NewAgent(provider, nil, cfg.MaxContext)
	baseAgt.RegisterTool(tools.NewRunCommandTool())
	baseAgt.RegisterTool(tools.NewReadDirTool())
	baseAgt.RegisterTool(tools.NewReadFileTool())
	baseAgt.RegisterTool(tools.NewWriteFileTool())
	baseAgt.RegisterTool(tools.NewSearchTool(cwd))

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

	baseAgt.SetSystemPrompt(`你是嵌入在 CLI Agent 中的资深工程师助手。能力包含文件操作、内核 RPC 调用、LSP 智能及浏览器自动化操控。`)
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
