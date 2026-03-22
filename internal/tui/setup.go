package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type SetupModel struct {
	providerInput textinput.Model
	keyInput      textinput.Model
	urlInput      textinput.Model
	step          int // 0: Provider, 1: Key, 2: BaseURL, 3: Done
	Provider      string
	APIKey        string
	BaseURL       string
	Quitting      bool
}

const (
	stepProvider = iota
	stepAPIKey
	stepBaseURL
	stepDone
)

func NewSetupModel(defaultProvider string) SetupModel {
	pi := textinput.New()
	pi.Placeholder = "openai / gemini / anthropic / ollama / lmstudio"
	pi.SetValue(defaultProvider)
	pi.Focus()
	pi.CharLimit = 40
	pi.Width = 40
	pi.Prompt = "Provider > "

	ki := textinput.New()
	ki.Placeholder = "输入您的 API Key (本地模型可为空)"
	ki.CharLimit = 150
	ki.Width = 60
	ki.Prompt = "API Key  > "
	ki.EchoMode = textinput.EchoPassword

	ui := textinput.New()
	ui.Placeholder = "自定义 Base URL (例如 http://localhost:11434/v1)"
	ui.CharLimit = 150
	ui.Width = 60
	ui.Prompt = "Base URL > "

	return SetupModel{
		providerInput: pi,
		keyInput:      ki,
		urlInput:      ui,
		step:          stepProvider,
	}
}

func (m SetupModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m SetupModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			m.Quitting = true
			return m, tea.Quit
		case tea.KeyEnter:
			switch m.step {
			case stepProvider:
				m.Provider = strings.TrimSpace(m.providerInput.Value())
				if m.Provider == "" {
					m.Provider = "openai"
				}
				// 自动设置对应渠道的默认提示
				switch m.Provider {
				case "ollama":
					m.urlInput.SetValue("http://localhost:11434/v1")
				case "lmstudio":
					m.urlInput.SetValue("http://localhost:1234/v1")
				}
				m.step = stepAPIKey
				m.keyInput.Focus()
				return m, textinput.Blink
			case stepAPIKey:
				m.APIKey = strings.TrimSpace(m.keyInput.Value())
				m.step = stepBaseURL
				m.urlInput.Focus()
				return m, textinput.Blink
			case stepBaseURL:
				m.BaseURL = strings.TrimSpace(m.urlInput.Value())
				m.step = stepDone
				return m, tea.Quit
			}
		}
	}

	switch m.step {
	case stepProvider:
		m.providerInput, cmd = m.providerInput.Update(msg)
	case stepAPIKey:
		m.keyInput, cmd = m.keyInput.Update(msg)
	case stepBaseURL:
		m.urlInput, cmd = m.urlInput.Update(msg)
	}

	return m, cmd
}

func (m SetupModel) View() string {
	if m.Quitting {
		return "初始化已取消。\n"
	}
	if m.step == stepDone {
		return fmt.Sprintf("配置完成！即将使用 %s 渠道。\n", m.Provider)
	}

	s := strings.Builder{}
	s.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205")).Render("🚀 Antigravity Go 初始化向导"))
	s.WriteString("\n\n")

	switch m.step {
	case stepProvider:
		s.WriteString("步骤 1: 选择 AI 渠道类型\n")
		s.WriteString("(支持 openai, gemini, anthropic, ollama, lmstudio)\n")
		s.WriteString(m.providerInput.View())
	case stepAPIKey:
		s.WriteString("步骤 2: 输入 API 密钥 (API Key)\n")
		s.WriteString(m.keyInput.View())
	case stepBaseURL:
		s.WriteString("步骤 3: 配置接口地址 (Base URL)\n")
		s.WriteString("(留空使用官方默认地址)\n")
		s.WriteString(m.urlInput.View())
	}

	s.WriteString("\n\n（Esc 退出）\n")
	return s.String()
}
