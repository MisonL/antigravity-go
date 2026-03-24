package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mison/antigravity-go/internal/agent"
	"github.com/mison/antigravity-go/internal/config"
	"github.com/mison/antigravity-go/internal/core"
	"github.com/mison/antigravity-go/internal/llm"
	"github.com/mison/antigravity-go/internal/rpc"
	"github.com/mison/antigravity-go/internal/tools"
)

const initCmd = "init"

type commandRuntime struct {
	cfg      *config.Config
	provider llm.Provider
	host     *core.Host
	client   *rpc.Client
	lspMgr   *tools.LSPManager
	cwd      string
}

func runInit(args []string) {
	fs := flag.NewFlagSet(initCmd, flag.ExitOnError)
	moduleF := fs.String("module", "", "Go module path")
	_ = fs.Parse(args)

	if fs.NArg() != 0 {
		fmt.Printf("用法: ago %s [--module example.com/project]\n", initCmd)
		return
	}

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Printf("%s 初始化失败: %v\n", initCmd, err)
		return
	}

	scaffold, err := newInitScaffold(cwd, *moduleF)
	if err != nil {
		fmt.Printf("%s 初始化失败: %v\n", initCmd, err)
		return
	}

	written, err := scaffold.write()
	if err != nil {
		if rollbackErr := rollbackGeneratedFiles(cwd, written); rollbackErr != nil {
			fmt.Printf("%s 回滚失败: %v\n", initCmd, rollbackErr)
		}
		fmt.Printf("%s 生成脚手架失败: %v\n", initCmd, err)
		return
	}

	fmt.Printf("[OK] 已在 %s 生成 %s 脚手架。\n", cwd, scaffold.profile.DisplayName)
	fmt.Printf("[INFO] Go module: %s\n", scaffold.profile.ModulePath)
	fmt.Printf("[INFO] 前端目录: %s\n", filepath.Join(cwd, "frontend"))
}

func runReview(args []string) {
	fs := flag.NewFlagSet(reviewCmd, flag.ExitOnError)
	providerF := fs.String("provider", "", "LLM provider")
	modelF := fs.String("model", "", "Model to use")
	baseURLF := fs.String("base-url", "", "Base URL")
	_ = fs.Parse(args)

	if fs.NArg() > 1 {
		fmt.Printf("用法: ago %s [文件或目录]\n", reviewCmd)
		return
	}

	rt, err := newCommandRuntime(*providerF, *modelF, *baseURLF)
	if err != nil {
		fmt.Printf("%s 初始化失败: %v\n", reviewCmd, err)
		return
	}
	defer rt.Close()

	target, err := resolveReviewTarget(rt.cwd, fs.Arg(0))
	if err != nil {
		fmt.Printf("%s 目标无效: %v\n", reviewCmd, err)
		return
	}

	reviewAgt := rt.newReviewAgent()
	if err := streamAgentTask(reviewAgt, buildReviewPrompt(target)); err != nil {
		fmt.Printf("%s 运行失败: %v\n", reviewCmd, err)
	}
}

func runAutoFix(args []string) {
	fs := flag.NewFlagSet(autoFixCmd, flag.ExitOnError)
	providerF := fs.String("provider", "", "LLM provider")
	modelF := fs.String("model", "", "Model to use")
	baseURLF := fs.String("base-url", "", "Base URL")
	_ = fs.Parse(args)

	if fs.NArg() != 0 {
		fmt.Printf("用法: ago %s\n", autoFixCmd)
		return
	}

	rt, err := newCommandRuntime(*providerF, *modelF, *baseURLF)
	if err != nil {
		fmt.Printf("%s 初始化失败: %v\n", autoFixCmd, err)
		return
	}
	defer rt.Close()

	validation, err := rt.validationSnapshot()
	if err != nil {
		fmt.Printf("%s 获取验证状态失败: %v\n", autoFixCmd, err)
		return
	}
	if !validationReportHasIssues(validation) {
		fmt.Println("未发现编译错误或 Lint 警告，无需自动修复。")
		return
	}

	baseAgt := buildBaseAgent(rt.cfg, rt.provider, rt.host, rt.client, rt.lspMgr, rt.cwd)
	if err := streamAgentTask(baseAgt, buildAutoFixPrompt(validation)); err != nil {
		fmt.Printf("%s 运行失败: %v\n", autoFixCmd, err)
	}
}

func newCommandRuntime(providerName string, modelName string, baseURL string) (*commandRuntime, error) {
	cfg, err := config.Load()
	if err != nil || cfg == nil {
		cfg = config.DefaultConfig()
	}
	if strings.TrimSpace(providerName) != "" {
		cfg.Provider = providerName
	}
	if strings.TrimSpace(modelName) != "" {
		cfg.Model = modelName
	}
	if strings.TrimSpace(baseURL) != "" {
		cfg.BaseURL = baseURL
	}
	if err := os.MkdirAll(cfg.DataDir, 0755); err != nil {
		return nil, fmt.Errorf("创建数据目录失败: %w", err)
	}

	provider, err := buildProvider(cfg)
	if err != nil {
		return nil, fmt.Errorf("provider 初始化失败: %w", err)
	}

	bin, err := resolveCoreBinary(cfg)
	if err != nil {
		return nil, fmt.Errorf("无法定位 antigravity_core: %w", err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}

	host := core.NewHost(core.Config{BinPath: bin, DataDir: cfg.DataDir})
	if err := host.Start(); err != nil {
		return nil, fmt.Errorf("启动 core 失败: %w", err)
	}

	if err := host.WaitReady(30 * time.Second); err != nil {
		_ = host.Stop()
		return nil, fmt.Errorf("等待 core 就绪失败: %w", err)
	}

	client := rpc.NewClient(host.HTTPPort())
	if err := trackWorkspaceRoot(client, cwd); err != nil {
		_ = host.Stop()
		return nil, err
	}

	return &commandRuntime{
		cfg:      cfg,
		provider: provider,
		host:     host,
		client:   client,
		lspMgr:   tools.NewLSPManager(host, cwd),
		cwd:      cwd,
	}, nil
}

func (rt *commandRuntime) Close() {
	if rt == nil || rt.host == nil {
		return
	}
	if err := rt.host.Stop(); err != nil {
		fmt.Printf("WARN: 停止内核失败: %v\n", err)
	}
}

func (rt *commandRuntime) newReviewAgent() *agent.Agent {
	reviewAgt := agent.NewAgent(rt.provider, nil, rt.cfg.MaxContext)
	reviewAgt.RegisterTool(tools.NewReadDirTool())
	reviewAgt.RegisterTool(tools.NewReadFileTool())
	reviewAgt.RegisterTool(tools.NewSearchTool(rt.cwd))
	reviewAgt.RegisterTool(rt.lspMgr.GetDiagnosticsTool())

	coreV2 := tools.NewCoreV2Manager(rt.client)
	reviewAgt.RegisterTool(coreV2.GetRepoInfosTool())
	reviewAgt.RegisterTool(coreV2.GetCoreDiagnosticsTool())
	reviewAgt.RegisterTool(coreV2.GetValidationStatesTool())
	reviewAgt.RegisterTool(reviewAgt.GetSpecialistTool())
	reviewAgt.SetSystemPrompt(`你是 CLI 的 ReviewerAgent，只能做静态代码审查。
必须先收集证据，再输出结论。
只允许使用只读工具；绝对不要尝试修改文件、回滚、执行 shell 或触发任何写操作。`)
	return reviewAgt
}

func (rt *commandRuntime) validationSnapshot() (string, error) {
	res, err := rt.client.GetCodeValidationStates()
	if err != nil {
		return "", err
	}
	data, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func streamAgentTask(agt *agent.Agent, prompt string) error {
	ctx := context.Background()
	if err := agt.RunStream(ctx, prompt, func(chunk string, err error) {
		if err == nil {
			fmt.Print(chunk)
		}
	}, nil); err != nil {
		return err
	}
	fmt.Println()
	return nil
}

func rollbackGeneratedFiles(root string, paths []string) error {
	var errs []error
	for idx := len(paths) - 1; idx >= 0; idx-- {
		target := filepath.Join(root, paths[idx])
		if err := os.RemoveAll(target); err != nil && !errors.Is(err, os.ErrNotExist) {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
