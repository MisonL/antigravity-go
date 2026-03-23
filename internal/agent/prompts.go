package agent

import (
	"strings"

	"github.com/mison/antigravity-go/internal/pkg/i18n"
)

const (
	promptProfileCustom    = "custom"
	promptProfileDefault   = "default"
	promptProfileCoder     = "coder"
	promptProfileArchitect = "architect"
	promptProfileChat      = "chat"
)

var DefaultSystemPrompt = SystemPromptForMode(i18n.LocaleZHCN, promptProfileDefault)

func normalizePromptProfile(profile string) string {
	switch strings.ToLower(strings.TrimSpace(profile)) {
	case "", promptProfileDefault:
		return promptProfileDefault
	case promptProfileCoder:
		return promptProfileCoder
	case promptProfileArchitect:
		return promptProfileArchitect
	case promptProfileChat:
		return promptProfileChat
	default:
		return promptProfileCustom
	}
}

func SystemPromptForMode(locale string, profile string) string {
	localizer := i18n.MustLocalizer(locale)
	switch normalizePromptProfile(profile) {
	case promptProfileCoder:
		return localizer.T("agent.system.coder")
	case promptProfileArchitect:
		return localizer.T("agent.system.architect")
	case promptProfileChat:
		return localizer.T("agent.system.chat")
	default:
		return localizer.T("agent.system.default")
	}
}

func SpecialistPrompt(locale string, role string) (string, bool) {
	localizer := i18n.MustLocalizer(locale)
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "reviewer":
		return localizer.T("agent.specialist.reviewer"), true
	case "architect":
		return localizer.T("agent.specialist.architect"), true
	case "security":
		return localizer.T("agent.specialist.security"), true
	default:
		return "", false
	}
}
