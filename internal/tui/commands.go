package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/mison/antigravity-go/internal/agent"
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
		Description: tuiLocalizer().T("tui.command.quit.description"),
		Action: func(m *Model, args []string) tea.Cmd {
			m.addMessage(m.t("tui.command.quit.message"))
			return tea.Quit
		},
	},
	"/exit": {
		Name:        "/exit",
		Description: tuiLocalizer().T("tui.command.quit.description"),
		Action: func(m *Model, args []string) tea.Cmd {
			m.addMessage(m.t("tui.command.quit.message"))
			return tea.Quit
		},
	},
	"/mode": {
		Name:        "/mode",
		Description: tuiLocalizer().T("tui.command.mode.description"),
		Action: func(m *Model, args []string) tea.Cmd {
			if len(args) < 1 {
				m.addMessage(m.t("tui.command.mode.usage"))
				return nil
			}
			mode := strings.ToLower(args[0])
			switch mode {
			case "coder":
				m.agent.SetLocalizedSystemPrompt("coder")
			case "architect":
				m.agent.SetLocalizedSystemPrompt("architect")
			case "chat":
				m.agent.SetLocalizedSystemPrompt("chat")
			default:
				m.addMessage(m.t("tui.command.mode.invalid"))
				return nil
			}
			m.addMessage(m.t("tui.command.mode.updated", mode))
			return nil
		},
	},
	"/add": {
		Name:        "/add",
		Description: tuiLocalizer().T("tui.command.add.description"),
		Action: func(m *Model, args []string) tea.Cmd {
			if len(args) < 1 {
				m.addMessage(m.t("tui.command.add.usage"))
				return nil
			}
			path := args[0]
			content, err := os.ReadFile(path)
			if err != nil {
				m.addMessage(m.t("tui.command.add.read_failed", err))
				return nil
			}

			// Inject into history as a system message or user message?
			// Usually adding context means adding a user message "Here is file X..."
			// We handle this by simulating a user message but not rendering it fully to clean UI?
			// Or just display it. Let's display a summary.
			m.addMessage(m.t("tui.command.add.success", path, len(content)))

			// Actually send to agent history silently
			if m.agent != nil {
				m.agent.AddUserMessage(fmt.Sprintf("Context file %s:\n```\n%s\n```", path, string(content)))
			}
			return nil
		},
	},
	"/copy": {
		Name:        "/copy",
		Description: tuiLocalizer().T("tui.command.copy.description"),
		Action: func(m *Model, args []string) tea.Cmd {
			// Find last ai message
			// This is tricky without parsing markdown.
			// Ideally we store the raw last response.
			if m.currentRawResponse == "" {
				m.addMessage(m.t("tui.command.copy.empty"))
				return nil
			}

			// Naive extraction: find content between ```
			start := strings.Index(m.currentRawResponse, "```")
			if start == -1 {
				clipboard.WriteAll(m.currentRawResponse)
				m.addMessage(m.t("tui.command.copy.all"))
				return nil
			}

			// Extract all code blocks? Just the first one for now or all combined?
			// Let's copy the whole raw markdown for fidelity if code blocks exist,
			// or maybe just the first block.
			// "Benchmark" usually means copy the code.
			// Let's just copy the whole response for now as it's safer.
			clipboard.WriteAll(m.currentRawResponse)
			m.addMessage(m.t("tui.command.copy.clipboard"))
			return nil
		},
	},
	"/context": {
		Name:        "/context",
		Description: tuiLocalizer().T("tui.command.context.description"),
		Action: func(m *Model, args []string) tea.Cmd {
			if m.agent == nil {
				return nil
			}
			usage := m.agent.GetTokenUsage() // This is approx
			// A real token count needs access to encoding, agent might provide it.
			m.addMessage(m.t("tui.command.context.summary", usage, len(m.messages)))
			return nil
		},
	},
	"/status": {
		Name:        "/status",
		Description: tuiLocalizer().T("tui.command.status.description"),
		Action: func(m *Model, args []string) tea.Cmd {
			m.addMessage(m.t("tui.command.status.summary", m.host.IsReady()))
			return nil
		},
	},
	"/approvals": {
		Name:        "/approvals",
		Description: tuiLocalizer().T("tui.command.approvals.description"),
		Action: func(m *Model, args []string) tea.Cmd {
			if len(args) < 1 {
				m.addMessage(m.t("tui.approvals.current", m.approvalMode))
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
				m.addMessage(m.t("tui.command.approvals.usage"))
				return nil
			}

			if m.agent == nil {
				m.addMessage(m.t("tui.agent.not_initialized"))
				return nil
			}

			// 确保通道存在
			if m.permReqChan == nil {
				m.permReqChan = make(chan PermissionRequest)
			}

			switch mode {
			case "full":
				m.agent.SetPermissionFunc(func(req agent.PermissionRequest) agent.PermissionDecision {
					return agent.PermissionDecision{Allow: true}
				})
			case "read-only":
				m.agent.SetPermissionFunc(func(req agent.PermissionRequest) agent.PermissionDecision {
					return agent.PermissionDecision{Allow: false}
				})
			default:
				m.agent.SetPermissionFunc(func(req agent.PermissionRequest) agent.PermissionDecision {
					resChan := make(chan agent.PermissionDecision)
					m.permReqChan <- PermissionRequest{
						ToolName: req.ToolName,
						Args:     req.Args,
						Response: resChan,
					}
					return <-resChan
				})
			}

			m.approvalMode = mode
			m.addMessage(m.t("tui.command.approvals.updated", mode))

			// 不持久化以免复杂化，仅运行时生效
			return waitForPermission(m.permReqChan)
		},
	},
	"/save": {
		Name:        "/save",
		Description: tuiLocalizer().T("tui.command.save.description"),
		Action: func(m *Model, args []string) tea.Cmd {
			if m.rec == nil || m.agent == nil {
				m.addMessage(m.t("tui.command.save.not_initialized"))
				return nil
			}
			err := m.rec.SaveMessages(m.agent.SnapshotMessages())
			if err != nil {
				m.addMessage(m.t("tui.command.save.failed", err))
			} else {
				m.addMessage(m.t("tui.command.save.success", m.rec.Meta.ID))
			}
			return nil
		},
	},
	"/load": {
		Name:        "/load",
		Description: tuiLocalizer().T("tui.command.load.description"),
		Action: func(m *Model, args []string) tea.Cmd {
			if len(args) < 1 {
				m.addMessage(m.t("tui.command.load.usage"))
				return nil
			}
			// Deduce DataDir from current rec or ask user?
			// We can try to guess from m.rec.Dir parent
			dataDir := ""
			if m.rec != nil {
				dataDir = filepath.Dir(m.rec.Dir)
			} else {
				// Fallback or error
				m.addMessage(m.t("tui.command.load.no_datadir"))
				return nil
			}

			id := args[0]
			rec, err := session.Load(dataDir, id)
			if err != nil {
				m.addMessage(m.t("tui.command.load.failed", err))
				return nil
			}
			msgs, err := rec.LoadMessages()
			if err != nil {
				m.addMessage(m.t("tui.command.load.messages_failed", err))
				return nil
			}

			if m.agent != nil {
				m.agent.LoadMessages(msgs)
				m.messages = []string{} // Clear UI
				// Re-populate UI from loaded messages
				for _, msg := range msgs {
					m.addMessage(m.rolePrefix(msg.Role) + msg.Content)
				}
				m.addMessage(m.t("tui.command.load.success", id, len(msgs)))
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
		Description: tuiLocalizer().T("tui.command.help.description"),
		Action: func(m *Model, args []string) tea.Cmd {
			var sb strings.Builder
			sb.WriteString(m.t("tui.command.help.title"))

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
