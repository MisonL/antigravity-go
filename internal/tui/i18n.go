package tui

import "github.com/mison/antigravity-go/internal/pkg/i18n"

func tuiLocalizer() *i18n.Localizer {
	return i18n.MustLocalizer(i18n.DetectLocale())
}

func (m *Model) t(key string, args ...any) string {
	return tuiLocalizer().T(key, args...)
}
