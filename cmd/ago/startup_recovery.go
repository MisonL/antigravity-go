package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mison/antigravity-go/internal/config"
)

const startupRepairTimeFormat = "20060102-150405"

type startupOptions struct {
	RequestedDataDir string
	SafeStart        bool
	AutoRepair       bool
}

type startupReport struct {
	Mode     string
	Messages []string
}

type startupIssue struct {
	Path   string
	Reason string
}

func parseStartupOptions(args []string) startupOptions {
	opts := startupOptions{}
	for index := 0; index < len(args); index += 1 {
		arg := strings.TrimSpace(args[index])
		switch {
		case arg == "--safe-start":
			opts.SafeStart = true
		case arg == "--auto-repair":
			opts.AutoRepair = true
		case strings.HasPrefix(arg, "--data="):
			opts.RequestedDataDir = strings.TrimSpace(strings.TrimPrefix(arg, "--data="))
		case arg == "--data" && index+1 < len(args):
			opts.RequestedDataDir = strings.TrimSpace(args[index+1])
			index += 1
		}
	}
	return opts
}

func prepareStartupConfig(opts startupOptions) (*config.Config, startupReport, error) {
	configPath := config.ConfigPathForDataDir(opts.RequestedDataDir)
	dataDir := filepath.Dir(configPath)

	cfg, loadErr := config.LoadFromDataDir(opts.RequestedDataDir)
	issues := inspectStartupIssues(dataDir, configPath, loadErr)
	if len(issues) == 0 && loadErr == nil && cfg != nil {
		return cfg, startupReport{Mode: "normal"}, nil
	}

	if opts.SafeStart {
		safeCfg := config.DefaultConfig()
		safeCfg.DataDir = filepath.Join(os.TempDir(), "ago-safe-"+time.Now().UTC().Format(startupRepairTimeFormat))
		return safeCfg, startupReport{
			Mode: "safe",
			Messages: append(
				formatStartupIssues(issues),
				fmt.Sprintf("[WARN] 已切换到安全启动目录: %s", safeCfg.DataDir),
			),
		}, nil
	}

	if opts.AutoRepair {
		repairedCfg, messages, err := autoRepairStartup(dataDir, configPath, cfg, loadErr, issues)
		if err != nil {
			return nil, startupReport{}, err
		}
		return repairedCfg, startupReport{
			Mode:     "repair",
			Messages: messages,
		}, nil
	}

	return nil, startupReport{}, fmt.Errorf("%s", buildStartupFailureMessage(issues))
}

func inspectStartupIssues(dataDir string, configPath string, loadErr error) []startupIssue {
	issues := make([]startupIssue, 0, 4)

	if info, err := os.Stat(dataDir); err == nil && !info.IsDir() {
		issues = append(issues, startupIssue{
			Path:   dataDir,
			Reason: "数据目录当前是文件，不是目录",
		})
	}

	if info, err := os.Stat(configPath); err == nil && info.IsDir() {
		issues = append(issues, startupIssue{
			Path:   configPath,
			Reason: "配置路径当前是目录，不是文件",
		})
	}

	if loadErr != nil {
		issues = append(issues, startupIssue{
			Path:   configPath,
			Reason: "配置文件损坏或不可解析: " + loadErr.Error(),
		})
	}

	for _, name := range []string{"sessions", "tasks", "executions", "deployments"} {
		target := filepath.Join(dataDir, name)
		if info, err := os.Stat(target); err == nil && !info.IsDir() {
			issues = append(issues, startupIssue{
				Path:   target,
				Reason: "运行目录槽位被普通文件占用",
			})
		}
	}

	return issues
}

func formatStartupIssues(issues []startupIssue) []string {
	if len(issues) == 0 {
		return nil
	}
	messages := make([]string, 0, len(issues))
	for _, issue := range issues {
		messages = append(messages, fmt.Sprintf("[WARN] %s: %s", issue.Path, issue.Reason))
	}
	return messages
}

func buildStartupFailureMessage(issues []startupIssue) string {
	lines := []string{
		"检测到启动环境损坏，已拒绝继续启动。",
	}
	for _, issue := range issues {
		lines = append(lines, fmt.Sprintf("- %s: %s", issue.Path, issue.Reason))
	}
	lines = append(lines,
		"可用恢复方式：",
		"- 安全启动: ago --safe-start",
		"- 自动修复: ago --auto-repair",
	)
	return strings.Join(lines, "\n")
}

func autoRepairStartup(dataDir string, configPath string, loadedCfg *config.Config, loadErr error, issues []startupIssue) (*config.Config, []string, error) {
	messages := make([]string, 0, len(issues)+4)
	handled := make(map[string]struct{}, len(issues))
	for _, issue := range issues {
		if issue.Path == "" {
			continue
		}
		if _, ok := handled[issue.Path]; ok {
			continue
		}
		handled[issue.Path] = struct{}{}
		if _, err := os.Stat(issue.Path); err == nil {
			backupPath, backupErr := movePathToBackup(issue.Path)
			if backupErr != nil {
				return nil, nil, fmt.Errorf("自动修复失败，无法备份 %s: %w", issue.Path, backupErr)
			}
			messages = append(messages, fmt.Sprintf("[INFO] 已备份损坏路径: %s -> %s", issue.Path, backupPath))
		}
	}

	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, nil, fmt.Errorf("自动修复失败，无法创建数据目录: %w", err)
	}
	for _, name := range []string{"sessions", "tasks", "executions", "deployments"} {
		if err := os.MkdirAll(filepath.Join(dataDir, name), 0755); err != nil {
			return nil, nil, fmt.Errorf("自动修复失败，无法创建 %s: %w", name, err)
		}
	}

	repairedCfg := config.DefaultConfig()
	repairedCfg.DataDir = dataDir
	if loadedCfg != nil && loadErr == nil {
		repairedCfg = loadedCfg
		repairedCfg.DataDir = dataDir
	}
	if err := repairedCfg.Save(); err != nil {
		return nil, nil, fmt.Errorf("自动修复失败，无法重建配置文件: %w", err)
	}
	messages = append(messages, fmt.Sprintf("[INFO] 已重建配置文件: %s", configPath))
	return repairedCfg, messages, nil
}

func movePathToBackup(path string) (string, error) {
	suffix := ".bak-" + time.Now().UTC().Format(startupRepairTimeFormat)
	candidate := path + suffix
	for index := 1; ; index += 1 {
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			if err := os.Rename(path, candidate); err != nil {
				return "", err
			}
			return candidate, nil
		}
		candidate = fmt.Sprintf("%s.%d", path+suffix, index)
	}
}
