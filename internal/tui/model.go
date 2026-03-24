package tui

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"

	"github.com/mison/antigravity-go/internal/agent"
	"github.com/mison/antigravity-go/internal/core"
	"github.com/mison/antigravity-go/internal/llm"
	"github.com/mison/antigravity-go/internal/rpc"
	"github.com/mison/antigravity-go/internal/session"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
)

// Styles (Same as before)
// Premium Styles
var (
	// Colors
	colorAccent    = lipgloss.Color("39")  // Cyan
	colorSecondary = lipgloss.Color("205") // Pink
	colorDim       = lipgloss.Color("237") // Dark Grey
	colorText      = lipgloss.Color("252") // White

	// Styles
	logoStyle   = lipgloss.NewStyle().Foreground(colorAccent).Bold(true)
	statusStyle = lipgloss.NewStyle().Background(colorDim).Foreground(colorText).Padding(0, 1)
	activeStyle = lipgloss.NewStyle().Background(colorAccent).Foreground(lipgloss.Color("0")).Padding(0, 1).Bold(true)

	inputStyle  = lipgloss.NewStyle().Padding(0, 0) // No border
	outputStyle = lipgloss.NewStyle().Padding(0, 1) // Clean
)

// Streaming types
type StreamResult struct {
	Chunk string
	Err   error
}

type StreamStartedMsg struct {
	resChan chan StreamResult
}

type StreamTokenMsg string
type StreamErrorMsg error
type StreamDoneMsg struct{}

type ResourceTickMsg struct{}

type CompactDoneMsg struct {
	Err error
}

// Model definition
type Model struct {
	host   *core.Host
	client *rpc.Client
	agent  *agent.Agent

	textarea textarea.Model
	viewport viewport.Model
	spinner  spinner.Model

	messages           []string // Rendered markdown messages
	currentStreamChan  chan StreamResult
	currentRawResponse string // Accumulator for markdown rendering

	// Permission state
	permReqChan        chan PermissionRequest
	currentPermRequest *PermissionRequest
	askingPermission   bool

	thinking bool
	width    int
	height   int
	ready    bool

	cpuPercent float64
	memPercent float64

	approvalMode string
	rec          *session.Recorder

	// Autocomplete state
	suggestions   []string
	suggestionIdx int
	completing    bool
}

// Permission types
type PermissionRequest struct {
	ToolName string
	Args     string
	Response chan agent.PermissionDecision
}

type PermissionMsg PermissionRequest

// NewModel creates a new TUI model.
func NewModel(host *core.Host, client *rpc.Client, agt *agent.Agent, permChan chan PermissionRequest, approvalMode string, rec *session.Recorder) Model {
	ta := textarea.New()
	ta.Placeholder = tuiLocalizer().T("tui.input.placeholder")
	ta.Focus()
	ta.SetWidth(80)
	ta.SetHeight(3)
	ta.ShowLineNumbers = false

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	m := Model{
		host:         host,
		client:       client,
		agent:        agt,
		textarea:     ta,
		spinner:      sp,
		messages:     []string{},
		permReqChan:  permChan,
		approvalMode: strings.ToLower(strings.TrimSpace(approvalMode)),
		rec:          rec,
	}

	// 若传入 recorder 且存在历史消息，预加载到视图（用于 resume）
	if rec != nil {
		if msgs, err := rec.LoadMessages(); err == nil && len(msgs) > 0 {
			for _, msg := range msgs {
				if msg.Role == llm.RoleSystem {
					continue
				}
				prefix := m.rolePrefix(msg.Role)
				rendered, err := glamour.Render(prefix+msg.Content, "light")
				if err != nil {
					rendered = prefix + msg.Content
				}
				m.messages = append(m.messages, rendered)
			}
		}
	}

	return m
}

// Init initializes the model.
func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{
		textarea.Blink,
		tea.EnterAltScreen,
		m.spinner.Tick,
		m.tickResource(),
	}
	if m.permReqChan != nil {
		cmds = append(cmds, waitForPermission(m.permReqChan))
	}
	return tea.Batch(cmds...)
}

// Update handles messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// If asking for permission, handle Y/n
		if m.askingPermission {
			switch msg.String() {
			case "y", "Y":
				m.currentPermRequest.Response <- agent.PermissionDecision{Allow: true}
				m.askingPermission = false
				m.currentPermRequest = nil
				m.addMessage(m.t("tui.permission.allowed"))
				return m, waitForPermission(m.permReqChan)
			case "n", "N":
				m.currentPermRequest.Response <- agent.PermissionDecision{Allow: false}
				m.askingPermission = false
				m.currentPermRequest = nil
				m.addMessage(m.t("tui.permission.denied"))
				return m, waitForPermission(m.permReqChan)
			default:
				return m, nil // Ignore other keys
			}
		}

		// --- Autocomplete Navigation ---
		if m.completing && len(m.suggestions) > 0 {
			switch msg.String() {
			case "up":
				m.suggestionIdx--
				if m.suggestionIdx < 0 {
					m.suggestionIdx = len(m.suggestions) - 1
				}
				return m, nil
			case "down":
				m.suggestionIdx++
				if m.suggestionIdx >= len(m.suggestions) {
					m.suggestionIdx = 0
				}
				return m, nil
			case "tab", "enter":
				// Apply suggestion
				selected := m.suggestions[m.suggestionIdx]
				val := m.textarea.Value()

				// If it's a command
				if strings.HasPrefix(val, "/") {
					m.textarea.SetValue(selected + " ")
					m.textarea.SetCursor(len(selected) + 1)
				}
				// If it's a file (@)
				if idx := strings.LastIndex(val, "@"); idx != -1 {
					prefix := val[:idx+1]
					m.textarea.SetValue(prefix + selected + " ")
					m.textarea.SetCursor(len(prefix) + len(selected) + 1)
				}

				m.completing = false
				m.suggestions = nil
				return m, nil
			case "esc":
				m.completing = false
				m.suggestions = nil
				return m, nil
			}
		}

		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "enter":
			input := strings.TrimSpace(m.textarea.Value())
			if input != "" {
				m.textarea.Reset()
				m.completing = false // Reset
				cmd := m.processInput(input)
				cmds = append(cmds, cmd)
			}
			return m, tea.Batch(cmds...)
		}

		// Check for autocomplete triggers on every keypress
		// Ideally we do this after textarea update, but tea.KeyMsg is handled here.
		// We let textarea update first? No, we need to intercept keys if completing.
		// So we do the check *after* passing msg to textarea if not intercepted above.

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.textarea.SetWidth(msg.Width - 4)
		if !m.ready {
			m.viewport = viewport.New(msg.Width-4, msg.Height-15)
			m.ready = true
		} else {
			m.viewport.Width = msg.Width - 4
			m.viewport.Height = msg.Height - 15
		}
		m.viewport.SetContent(strings.Join(m.messages, ""))
		m.viewport.GotoBottom()

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		if m.thinking {
			cmds = append(cmds, cmd)
		}

		if m.thinking {
			cmds = append(cmds, cmd)
		}

	case ResourceTickMsg:
		return m, m.fetchResourceStats()

	case ResourceStatsMsg:
		m.cpuPercent = msg.CPU
		m.memPercent = msg.Mem
		return m, m.tickResource()

	// --- Streaming Handlers ---
	case StreamStartedMsg:
		m.thinking = true
		m.currentStreamChan = msg.resChan
		m.currentRawResponse = ""
		// Start a new empty message bubble for the assistant
		// We'll update this slot as tokens arrive
		m.addMessage(m.rolePrefix(llm.RoleAssistant))
		return m, waitForStream(m.currentStreamChan)

	case StreamTokenMsg:
		chunk := string(msg)
		m.currentRawResponse += chunk

		// Re-render the last message (the one we added in StreamStarted)
		// This is expensive but fine for TUI rates.
		rendered, err := glamour.Render(m.rolePrefix(llm.RoleAssistant)+m.currentRawResponse, "light")
		if err == nil {
			if len(m.messages) > 0 {
				m.messages[len(m.messages)-1] = rendered
			} else {
				m.messages = append(m.messages, rendered)
			}
		}
		m.viewport.SetContent(strings.Join(m.messages, ""))
		m.viewport.GotoBottom()

		// Next token
		return m, waitForStream(m.currentStreamChan)

	case StreamErrorMsg:
		m.thinking = false
		m.addMessage(m.t("tui.error.message", msg))
		if m.rec != nil {
			_ = m.rec.Append("chat_error", map[string]any{
				"error":   fmt.Sprint(msg),
				"partial": m.currentRawResponse,
			})
			if m.agent != nil {
				_ = m.rec.SaveMessages(m.agent.SnapshotMessages())
			}
		}
		m.currentStreamChan = nil

	case StreamDoneMsg:
		m.thinking = false
		m.currentStreamChan = nil
		if m.rec != nil {
			_ = m.rec.Append("assistant_message", map[string]any{"content": m.currentRawResponse})
			if m.agent != nil {
				_ = m.rec.SaveMessages(m.agent.SnapshotMessages())
			}
		}

	case CompactDoneMsg:
		m.thinking = false
		if msg.Err != nil {
			m.addMessage(m.t("tui.context.compact_failed", msg.Err))
		} else {
			m.addMessage(m.t("tui.context.compact_done"))
		}
		return m, nil

	// Permission
	case PermissionMsg:
		m.askingPermission = true
		m.currentPermRequest = (*PermissionRequest)(&msg)

		// Create a nice prompt
		prompt := m.t("tui.permission.prompt", msg.ToolName, msg.Args)
		m.addMessage(prompt)
		// We do NOT wait for next permission here, we wait after user handles it
	}

	// Update components
	var cmd tea.Cmd
	m.textarea, cmd = m.textarea.Update(msg)
	cmds = append(cmds, cmd)

	// Post-update: check for autocomplete triggers ONLY on key messages or similar
	// But update is called for blink/spinner too.
	// We should only re-scan if text might have changed.
	// Simple heuristic: if msg is KeyMsg.
	_, isKey := msg.(tea.KeyMsg)
	if isKey {
		val := m.textarea.Value()
		// Only trigger logic if value is not empty
		if len(val) > 0 {
			if strings.HasPrefix(val, "/") && !strings.Contains(val, " ") {
				// Slash command mode
				m.completing = true
				inputCmd := val
				var matches []string
				for _, c := range GetCommands() {
					if strings.HasPrefix(c, inputCmd) {
						matches = append(matches, c)
					}
				}
				m.suggestions = matches
				if len(matches) == 0 {
					m.completing = false
				}
				// Reset index if out of bounds
				if m.suggestionIdx >= len(m.suggestions) {
					m.suggestionIdx = 0
				}
			} else if idx := strings.LastIndex(val, "@"); idx != -1 {
				// File mode: simplistic check, looks for @ at end
				// Only trigger if @ is the start of a word?
				// For now, simple trigger: "@..."
				query := val[idx+1:]
				if !strings.Contains(query, " ") { // Ensure we are typing the filename
					m.completing = true
					files, _ := os.ReadDir(".")
					var matches []string
					for _, f := range files {
						name := f.Name()
						if strings.HasPrefix(strings.ToLower(name), strings.ToLower(query)) {
							matches = append(matches, name)
						}
					}
					m.suggestions = matches
					if len(matches) == 0 {
						m.completing = false
					}
					if m.suggestionIdx >= len(m.suggestions) {
						m.suggestionIdx = 0
					}
				} else {
					m.completing = false
				}
			} else {
				m.completing = false
			}
		} else {
			m.completing = false
		}
	}

	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// processInput handles user input.
func (m *Model) processInput(input string) tea.Cmd {
	m.addMessage(m.rolePrefix(llm.RoleUser) + input)
	if m.rec != nil {
		_ = m.rec.Append("user_message", map[string]any{"content": input})
	}

	if strings.HasPrefix(input, "/") {
		return m.handleSlashCommand(input)
	}

	if m.agent != nil {
		return m.runAgentStream(input)
	}

	m.addMessage(m.t("tui.agent.not_initialized"))
	return nil
}

// runAgentStream starts the streaming agent
func (m *Model) runAgentStream(input string) tea.Cmd {
	ch := make(chan StreamResult)

	go func() {
		defer close(ch)
		// Call Agent
		err := m.agent.RunStream(context.Background(), input, func(chunk string, err error) {
			// Callback from LLM provider
			if err != nil {
				// We could send error chunk if needed, but usually provider returns error at end
				return
			}
			ch <- StreamResult{Chunk: chunk}
		}, nil)

		if err != nil {
			ch <- StreamResult{Err: err}
		}
	}()

	return func() tea.Msg {
		return StreamStartedMsg{resChan: ch}
	}
}

// waitForStream reads the next item from channel
func waitForStream(ch chan StreamResult) tea.Cmd {
	return func() tea.Msg {
		res, ok := <-ch
		if !ok {
			return StreamDoneMsg{}
		}
		if res.Err != nil {
			return StreamErrorMsg(res.Err)
		}
		return StreamTokenMsg(res.Chunk)
	}
}

func waitForPermission(ch chan PermissionRequest) tea.Cmd {
	return func() tea.Msg {
		req := <-ch // Blocking wait
		return PermissionMsg(req)
	}
}

// handleSlashCommand returns a command if needed
func (m *Model) handleSlashCommand(cmd string) tea.Cmd {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return nil
	}

	cmdName := parts[0]
	args := parts[1:]

	if def, ok := registry[cmdName]; ok {
		return def.Action(m, args)
	}

	// Legacy handling or fuzzy fallback
	switch cmdName {
	case "/status": // Keep legacy if not in registry (though added above)
		m.addMessage(m.t("tui.command.status.summary", m.host.IsReady()))
		return nil
	case "/approvals":
		// ... logic from before (move to registry later for cleaner code) ...
		if len(parts) == 1 {
			m.addMessage(m.t("tui.approvals.current", m.approvalMode))
			return nil
		}
		mode := strings.ToLower(strings.TrimSpace(parts[1]))
		// ... logic copy ...
		m.approvalMode = mode // simplified for brevity in this tool call
		m.addMessage(m.t("tui.command.approvals.updated", mode))
		return nil
	}

	// Fuzzy matching
	suggestion := findClosestCommand(cmdName, GetCommands())
	msg := m.t("tui.command.unknown", cmdName)
	if suggestion != "" {
		msg += m.t("tui.command.suggestion", suggestion)
	}
	m.addMessage(msg)
	return nil
}

// addMessage adds a rendered markdown message
func (m *Model) addMessage(markdown string) {
	rendered, err := glamour.Render(markdown, "dark") // Enforce dark/colorful for better TUI look
	if err != nil {
		rendered = markdown
	}
	m.messages = append(m.messages, rendered)
	m.viewport.SetContent(strings.Join(m.messages, ""))
	m.viewport.GotoBottom()
}

func (m Model) rolePrefix(role llm.Role) string {
	switch role {
	case llm.RoleUser:
		return "[" + m.t("tui.role.user") + "] "
	case llm.RoleTool:
		return "[" + m.t("tui.role.tool") + "] "
	default:
		return "[" + m.t("tui.role.assistant") + "] "
	}
}

// View outputs the UI
func (m Model) View() string {
	if !m.ready {
		return "Initializing..."
	}

	var s strings.Builder

	// --- 1. Header (Logo) ---
	if m.height >= 40 { // Only show big logo on tall screens
		logo := `
    ___          __  _                           _ __
   /   |  ____  / /_(_)___ __________ __   __(_) /___  __
  / /| | / __ \/ __/ / __  / ___/ __  / | / / / __/ / / /
 / ___ |/ / / / /_/ / /_/ / /  / /_/ /| |/ / / /_/ /_/ /
/_/  |_/_/ /_/\__/_/\__, /_/   \__,_/ |___/_/\__/\__, /
                   /____/                       /____/
`
		s.WriteString(logoStyle.Render(logo))
		s.WriteString("\n")
	} else if m.height >= 25 {
		// Compact branding
		s.WriteString(logoStyle.Render(" 🚀 ANTIGRAVITY AGENT "))
		s.WriteString("\n\n")
	}

	// --- 2. Main Content (Viewport) ---
	// We want the viewport to take available space.
	// But in TUI, we just render it. The Viewport model handles scrolling.
	s.WriteString(outputStyle.Render(m.viewport.View()))
	s.WriteString("\n")

	// --- 3. Status Bar (Pill Design) ---
	// Status Pill
	statusText := "IDLE"
	statusColor := colorAccent
	if m.thinking {
		statusText = "THINKING " + m.spinner.View()
		statusColor = colorSecondary
	}
	statusPill := lipgloss.NewStyle().Background(statusColor).Foreground(lipgloss.Color("0")).Bold(true).Padding(0, 1).Render(statusText)

	// Info Pills
	tokens := 0
	if m.agent != nil {
		tokens = m.agent.GetTokenUsage()
	}

	portPill := statusStyle.Render(fmt.Sprintf("PORT: %d", m.host.HTTPPort()))
	tokenPill := statusStyle.Render(fmt.Sprintf("TOKENS: %d", tokens))
	permPill := statusStyle.Render(fmt.Sprintf("PERM: %s", strings.ToUpper(m.approvalMode)))

	// Resource Pills (Dynamic Color)
	cpuColor := "46"
	if m.cpuPercent > 80 {
		cpuColor = "196"
	} else if m.cpuPercent > 50 {
		cpuColor = "226"
	}
	cpuPill := lipgloss.NewStyle().Foreground(lipgloss.Color(cpuColor)).Background(colorDim).Padding(0, 1).Render(fmt.Sprintf("CPU: %.0f%%", m.cpuPercent))

	memColor := "46"
	if m.memPercent > 80 {
		memColor = "196"
	} else if m.memPercent > 50 {
		memColor = "226"
	}
	memPill := lipgloss.NewStyle().Foreground(lipgloss.Color(memColor)).Background(colorDim).Padding(0, 1).Render(fmt.Sprintf("MEM: %.0f%%", m.memPercent))

	// Assemble Bar
	// MODE | PORT | TOKENS | PERM | CPU | MEM
	// Use JoinHorizontal with gaps
	gap := lipgloss.NewStyle().Render(" ")
	bar := lipgloss.JoinHorizontal(lipgloss.Center,
		statusPill, gap,
		portPill, gap,
		tokenPill, gap,
		permPill, gap,
		cpuPill, gap,
		memPill,
	)
	s.WriteString(bar)
	s.WriteString("\n")

	// --- 4. Autocomplete Popup ---
	if m.completing && len(m.suggestions) > 0 {
		var listBuilder strings.Builder
		maxH := 5
		start := m.suggestionIdx - (maxH / 2)
		if start < 0 {
			start = 0
		}
		end := start + maxH
		if end > len(m.suggestions) {
			end = len(m.suggestions)
		}
		if end-start < maxH && len(m.suggestions) >= maxH {
			start = end - maxH
		}

		for i := start; i < end; i++ {
			item := m.suggestions[i]
			if i == m.suggestionIdx {
				listBuilder.WriteString(lipgloss.NewStyle().Background(colorSecondary).Foreground(lipgloss.Color("0")).Padding(0, 1).Render(item))
			} else {
				listBuilder.WriteString(lipgloss.NewStyle().Padding(0, 1).Foreground(colorText).Render(item))
			}
			listBuilder.WriteString("\n")
		}

		popup := lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(colorSecondary).
			Background(lipgloss.Color("235")).
			Render(strings.TrimSpace(listBuilder.String()))

		s.WriteString(popup)
		s.WriteString("\n")
	} else {
		s.WriteString("\n")
	}

	// --- 5. Input Area ---
	// "🚀 > [Input]"
	prompt := lipgloss.NewStyle().Foreground(colorSecondary).Bold(true).Render("✨ INPUT > ")
	s.WriteString(lipgloss.JoinHorizontal(lipgloss.Center, prompt, m.textarea.View()))

	return s.String()
}

type ResourceStatsMsg struct {
	CPU float64
	Mem float64
}

func (m Model) tickResource() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
		return ResourceTickMsg{}
	})
}

func (m Model) fetchResourceStats() tea.Cmd {
	return func() tea.Msg {
		c, _ := cpu.Percent(0, false)
		v, _ := mem.VirtualMemory()

		cpuVal := 0.0
		if len(c) > 0 {
			cpuVal = c[0]
		}

		memVal := 0.0
		if v != nil {
			memVal = v.UsedPercent
		}

		return ResourceStatsMsg{
			CPU: cpuVal,
			Mem: memVal,
		}
	}
}
