package tui

import (
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
	localizer := tuiLocalizer()
	pi := textinput.New()
	pi.Placeholder = localizer.T("tui.setup.provider.placeholder")
	pi.SetValue(defaultProvider)
	pi.Focus()
	pi.CharLimit = 40
	pi.Width = 40
	pi.Prompt = "Provider > "

	ki := textinput.New()
	ki.Placeholder = localizer.T("tui.setup.key.placeholder")
	ki.CharLimit = 150
	ki.Width = 60
	ki.Prompt = "API Key  > "
	ki.EchoMode = textinput.EchoPassword

	ui := textinput.New()
	ui.Placeholder = localizer.T("tui.setup.url.placeholder")
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
		case tea.KeyShiftTab:
			if m.step > stepProvider {
				m.step--
				switch m.step {
				case stepProvider:
					m.providerInput.Focus()
				case stepAPIKey:
					m.keyInput.Focus()
				case stepBaseURL:
					m.urlInput.Focus()
				}
				return m, textinput.Blink
			}
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
	localizer := tuiLocalizer()
	if m.Quitting {
		return localizer.T("tui.setup.cancelled")
	}
	if m.step == stepDone {
		return localizer.T("tui.setup.completed", m.Provider)
	}

	s := strings.Builder{}
	s.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205")).Render(localizer.T("tui.setup.title")))
	s.WriteString("\n\n")

	switch m.step {
	case stepProvider:
		s.WriteString(localizer.T("tui.setup.step.provider"))
		s.WriteString(localizer.T("tui.setup.step.provider.hint"))
		s.WriteString(m.providerInput.View())
	case stepAPIKey:
		s.WriteString(localizer.T("tui.setup.step.key"))
		s.WriteString(m.keyInput.View())
	case stepBaseURL:
		s.WriteString(localizer.T("tui.setup.step.url"))
		s.WriteString(localizer.T("tui.setup.step.url.hint"))
		s.WriteString(m.urlInput.View())
	}

	s.WriteString(localizer.T("tui.setup.exit_hint"))
	return s.String()
}
