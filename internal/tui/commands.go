package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/mison/antigravity-go/internal/session"
)

// CommandDefinition defines a slash command
type CommandDefinition struct {
	Name        string
	Description string
	Action      func(m *Model, args []string) tea.Cmd
}

// Commands registry
var registry = map[string]CommandDefinition{
	"/quit": {
		Name:        "/quit",
		Description: "退出程序",
		Action: func(m *Model, args []string) tea.Cmd {
			m.addMessage("👋 再见！")
			return tea.Quit
		},
	},
	"/exit": {
		Name:        "/exit",
		Description: "退出程序",
		Action: func(m *Model, args []string) tea.Cmd {
			m.addMessage("👋 再见！")
			return tea.Quit
		},
	},
	"/mode": {
		Name:        "/mode",
		Description: "切换 AI 模式 (coder/architect/chat)",
		Action: func(m *Model, args []string) tea.Cmd {
			if len(args) < 1 {
				m.addMessage("用法：/mode <coder|architect|chat>")
				return nil
			}
			mode := strings.ToLower(args[0])
			switch mode {
			case "coder":
				m.agent.SetSystemPrompt("You are an expert coder. Focus on implementation and code quality.")
			case "architect":
				m.agent.SetSystemPrompt("You are a software architect. Focus on high-level design and system patterns.")
			case "chat":
				m.agent.SetSystemPrompt("You are a helpful assistant. Chat casually.")
			default:
				m.addMessage("未知模式，可选：coder, architect, chat")
				return nil
			}
			m.addMessage(fmt.Sprintf("🔄 已切换模式为：**%s**", mode))
			return nil
		},
	},
	"/add": {
		Name:        "/add",
		Description: "添加文件到上下文",
		Action: func(m *Model, args []string) tea.Cmd {
			if len(args) < 1 {
				m.addMessage("用法：/add <file_path>")
				return nil
			}
			path := args[0]
			content, err := os.ReadFile(path)
			if err != nil {
				m.addMessage(fmt.Sprintf("❌ 读取失败：%v", err))
				return nil
			}

			// Inject into history as a system message or user message?
			// Usually adding context means adding a user message "Here is file X..."
			// We handle this by simulating a user message but not rendering it fully to clean UI?
			// Or just display it. Let's display a summary.
			m.addMessage(fmt.Sprintf("📄 **已添加文件**：`%s` (%d bytes)", path, len(content)))

			// Actually send to agent history silently
			if m.agent != nil {
				m.agent.AddUserMessage(fmt.Sprintf("Context file %s:\n```\n%s\n```", path, string(content)))
			}
			return nil
		},
	},
	"/copy": {
		Name:        "/copy",
		Description: "复制最后一段代码",
		Action: func(m *Model, args []string) tea.Cmd {
			// Find last ai message
			// This is tricky without parsing markdown.
			// Ideally we store the raw last response.
			if m.currentRawResponse == "" {
				m.addMessage("⚠️ 没有可复制的代码内容（仅支持最近一次回复）")
				return nil
			}

			// Naive extraction: find content between ```
			start := strings.Index(m.currentRawResponse, "```")
			if start == -1 {
				clipboard.WriteAll(m.currentRawResponse)
				m.addMessage("✅ 已复制全部回复内容")
				return nil
			}

			// Extract all code blocks? Just the first one for now or all combined?
			// Let's copy the whole raw markdown for fidelity if code blocks exist,
			// or maybe just the first block.
			// "Benchmark" usually means copy the code.
			// Let's just copy the whole response for now as it's safer.
			clipboard.WriteAll(m.currentRawResponse)
			m.addMessage("✅ 已复制回复内容到剪贴板")
			return nil
		},
	},
	"/context": {
		Name:        "/context",
		Description: "查看上下文统计",
		Action: func(m *Model, args []string) tea.Cmd {
			if m.agent == nil {
				return nil
			}
			usage := m.agent.GetTokenUsage() // This is approx
			// A real token count needs access to encoding, agent might provide it.
			m.addMessage(fmt.Sprintf("📊 **上下文统计**\n- 估算 Tokens: %d\n- 消息数: %d", usage, len(m.messages))) // History len is internal agent field
			return nil
		},
	},
	"/status": {
		Name:        "/status",
		Description: "查看连接状态",
		Action: func(m *Model, args []string) tea.Cmd {
			m.addMessage("📊 **状态**：Ready=" + fmt.Sprint(m.host.IsReady()))
			return nil
		},
	},
	"/approvals": {
		Name:        "/approvals",
		Description: "切换权限策略 (read-only/prompt/full)",
		Action: func(m *Model, args []string) tea.Cmd {
			if len(args) < 1 {
				m.addMessage(fmt.Sprintf("🛡️ 当前 approvals: **%s**（可选：read-only / prompt / full）", m.approvalMode))
				return nil
			}
			mode := strings.ToLower(strings.TrimSpace(args[0]))
			switch mode {
			case "readonly", "read-only", "read_only":
				mode = "read-only"
			case "prompt":
				mode = "prompt"
			case "full":
				mode = "full"
			default:
				m.addMessage("用法：/approvals read-only|prompt|full")
				return nil
			}

			if m.agent == nil {
				m.addMessage("⚠️ Agent 未初始化（缺少 API Key？）")
				return nil
			}

			// 确保通道存在
			if m.permReqChan == nil {
				m.permReqChan = make(chan PermissionRequest)
			}

			switch mode {
			case "full":
				m.agent.SetPermissionFunc(func(toolName, args string) bool { return true })
			case "read-only":
				m.agent.SetPermissionFunc(func(toolName, args string) bool { return false })
			default:
				m.agent.SetPermissionFunc(func(toolName, args string) bool {
					resChan := make(chan bool)
					m.permReqChan <- PermissionRequest{
						ToolName: toolName,
						Args:     args,
						Response: resChan,
					}
					return <-resChan
				})
			}

			m.approvalMode = mode
			m.addMessage(fmt.Sprintf("✅ approvals 已切换为 **%s**", mode))

			// 不持久化以免复杂化，仅运行时生效
			return waitForPermission(m.permReqChan)
		},
	},
	"/save": {
		Name:        "/save",
		Description: "保存会话到磁盘",
		Action: func(m *Model, args []string) tea.Cmd {
			if m.rec == nil || m.agent == nil {
				m.addMessage("⚠️ Recorder or Agent not initialized")
				return nil
			}
			err := m.rec.SaveMessages(m.agent.SnapshotMessages())
			if err != nil {
				m.addMessage(fmt.Sprintf("❌ 保存失败：%v", err))
			} else {
				m.addMessage(fmt.Sprintf("✅ 会话已保存 (ID: %s)", m.rec.Meta.ID))
			}
			return nil
		},
	},
	"/load": {
		Name:        "/load",
		Description: "加载会话 (需提供 ID)",
		Action: func(m *Model, args []string) tea.Cmd {
			if len(args) < 1 {
				m.addMessage("用法：/load <session_id> (警告：将覆盖当前上下文)")
				return nil
			}
			// Deduce DataDir from current rec or ask user?
			// We can try to guess from m.rec.Dir parent
			dataDir := ""
			if m.rec != nil {
				dataDir = filepath.Dir(m.rec.Dir)
			} else {
				// Fallback or error
				m.addMessage("⚠️ 无法确定 DataDir (当前未关联 Session)")
				return nil
			}

			id := args[0]
			rec, err := session.Load(dataDir, id)
			if err != nil {
				m.addMessage(fmt.Sprintf("❌ 加载失败：%v", err))
				return nil
			}
			msgs, err := rec.LoadMessages()
			if err != nil {
				m.addMessage(fmt.Sprintf("❌ 读取消息失败：%v", err))
				return nil
			}

			if m.agent != nil {
				m.agent.LoadMessages(msgs)
				m.messages = []string{} // Clear UI
				// Re-populate UI from loaded messages
				for _, msg := range msgs {
					// ... simple rendering ...
					prefix := "🤖 "
					if msg.Role == "user" {
						prefix = "👤 "
					}
					m.addMessage(prefix + msg.Content)
				}
				m.addMessage(fmt.Sprintf("✅ 已加载会话 %s (%d msgs)", id, len(msgs)))
			}
			return nil
		},
	},
}

func GetCommands() []string {
	var cmds []string
	for k := range registry {
		cmds = append(cmds, k)
	}
	sort.Strings(cmds)
	return cmds
}

func init() {
	registry["/help"] = CommandDefinition{
		Name:        "/help",
		Description: "显示帮助信息",
		Action: func(m *Model, args []string) tea.Cmd {
			var sb strings.Builder
			sb.WriteString("**可用命令**：\n\n")

			// Static ordered list
			ordered := []string{"/help", "/mode", "/add", "/copy", "/save", "/load", "/context", "/status", "/approvals", "/clear", "/quit", "/exit"}

			for _, cmd := range ordered {
				if def, ok := registry[cmd]; ok {
					sb.WriteString(fmt.Sprintf("- **%s**：%s\n", def.Name, def.Description))
				}
			}

			// Add any dynamic commands not in ordered list
			for k, v := range registry {
				found := false
				for _, o := range ordered {
					if o == k {
						found = true
						break
					}
				}
				if !found {
					sb.WriteString(fmt.Sprintf("- **%s**：%s\n", v.Name, v.Description))
				}
			}

			m.addMessage(sb.String())
			return nil
		},
	}
}
