package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mison/antigravity-go/internal/llm/iflow"
)

type SetupModel struct {
	providerInput textinput.Model
	keyInput      textinput.Model
	step          int // 0: Provider, 1: Key, 2: Model(iflow), 3: Done
	Provider      string
	APIKey        string
	Model         string
	models        []iflow.ModelSpec
	modelIndex    int
	Quitting      bool
}

const (
	stepProvider = iota
	stepAPIKey
	stepModel
	stepDone
)

func NewSetupModel(defaultProvider string) SetupModel {
	pi := textinput.New()
	pi.Placeholder = "openai / gemini / iflow"
	pi.SetValue(defaultProvider)
	pi.Focus()
	pi.CharLimit = 20
	pi.Width = 30
	pi.Prompt = "Provider > "

	ki := textinput.New()
	ki.Placeholder = "sk-... / iflow-key"
	ki.CharLimit = 100
	ki.Width = 50
	ki.Prompt = "API Key > "
	ki.EchoMode = textinput.EchoPassword

	return SetupModel{
		providerInput: pi,
		keyInput:      ki,
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
		case tea.KeyUp:
			if m.step == stepModel && len(m.models) > 0 {
				if m.modelIndex > 0 {
					m.modelIndex--
				}
				return m, nil
			}
		case tea.KeyDown:
			if m.step == stepModel && len(m.models) > 0 {
				if m.modelIndex < len(m.models)-1 {
					m.modelIndex++
				}
				return m, nil
			}
		case tea.KeyEnter:
			switch m.step {
			case stepProvider:
				m.Provider = strings.TrimSpace(m.providerInput.Value())
				if m.Provider == "" {
					m.Provider = "openai" // Default
				}
				if m.Provider != "openai" && m.Provider != "gemini" && m.Provider != "iflow" {
					// Invalid provider, maybe show error? for now just reset to default or accept
					// Let's force valid provider
					if strings.Contains(m.Provider, "gem") {
						m.Provider = "gemini"
					} else if strings.Contains(m.Provider, "flow") {
						m.Provider = "iflow"
					} else {
						m.Provider = "openai"
					}
				}
				m.step = stepAPIKey
				m.keyInput.Focus()
				return m, textinput.Blink
			case stepAPIKey:
				m.APIKey = strings.TrimSpace(m.keyInput.Value())
				if m.APIKey != "" {
					if m.Provider == "iflow" {
						m.models = iflow.Models()
						m.modelIndex = 0
						def := iflow.DefaultModelName()
						for i, it := range m.models {
							if it.Name == def {
								m.modelIndex = i
								break
							}
						}
						m.step = stepModel
						return m, nil
					}
					m.step = stepDone
					return m, tea.Quit
				}
			case stepModel:
				if len(m.models) > 0 && m.modelIndex >= 0 && m.modelIndex < len(m.models) {
					m.Model = m.models[m.modelIndex].Name
				}
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
	}

	return m, cmd
}

func (m SetupModel) View() string {
	if m.Quitting {
		return "初始化已取消。\n"
	}
	if m.step == stepDone {
		if m.Provider == "iflow" && strings.TrimSpace(m.Model) != "" {
			return fmt.Sprintf("配置完成！使用 %s（%s）。\n", m.Provider, m.Model)
		}
		return fmt.Sprintf("配置完成！使用 %s。\n", m.Provider)
	}

	s := strings.Builder{}
	s.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205")).Render("🚀 Antigravity Go Setup"))
	s.WriteString("\n\n")

	switch m.step {
	case stepProvider:
		s.WriteString("选择 LLM Provider（openai/gemini/iflow）：\n")
		s.WriteString(m.providerInput.View())
	case stepAPIKey:
		s.WriteString("输入 API Key：\n")
		s.WriteString(m.keyInput.View())
	case stepModel:
		s.WriteString("选择 iFlow 模型（↑/↓ 选择，Enter 确认）：\n\n")
		if len(m.models) == 0 {
			s.WriteString("（未加载到模型清单）\n")
		} else {
			for i, it := range m.models {
				prefix := "  "
				if i == m.modelIndex {
					prefix = "➤ "
				}
				s.WriteString(prefix)
				line := fmt.Sprintf("%-26s  %-26s  %s  输出%s  上下文%s",
					it.Name,
					it.DisplayName,
					it.Status,
					formatTokens(it.MaxOutputTokens),
					formatTokens(it.MaxContextTokens),
				)
				if i == m.modelIndex {
					s.WriteString(lipgloss.NewStyle().Bold(true).Render(line))
				} else {
					s.WriteString(line)
				}
				s.WriteString("\n")
			}
		}
	}

	s.WriteString("\n\n（Esc 退出）\n")
	return s.String()
}

func formatTokens(n int) string {
	if n <= 0 {
		return "-"
	}
	if n%(1024*1024) == 0 {
		return fmt.Sprintf("%dM", n/(1024*1024))
	}
	if n%1024 == 0 {
		return fmt.Sprintf("%dK", n/1024)
	}
	return fmt.Sprintf("%d", n)
}
