package i18n

import (
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
)

const (
	LocaleZHCN = "zh-CN"
	LocaleENUS = "en-US"
)

//go:embed locales/*.json
var localeFS embed.FS

type Bundle struct {
	messages map[string]map[string]string
}

type Localizer struct {
	bundle *Bundle
	locale string
}

var (
	defaultBundle *Bundle
	loadOnce      sync.Once
	loadErr       error
)

func DefaultBundle() (*Bundle, error) {
	loadOnce.Do(func() {
		defaultBundle, loadErr = loadBundle()
	})
	return defaultBundle, loadErr
}

func MustDefaultBundle() *Bundle {
	bundle, err := DefaultBundle()
	if err != nil {
		panic(err)
	}
	return bundle
}

func loadBundle() (*Bundle, error) {
	locales := []string{LocaleZHCN, LocaleENUS}
	messages := make(map[string]map[string]string, len(locales))

	for _, locale := range locales {
		raw, err := localeFS.ReadFile("locales/" + locale + ".json")
		if err != nil {
			return nil, fmt.Errorf("read locale %s: %w", locale, err)
		}

		var entries map[string]string
		if err := json.Unmarshal(raw, &entries); err != nil {
			return nil, fmt.Errorf("parse locale %s: %w", locale, err)
		}
		messages[locale] = entries
	}

	return &Bundle{messages: messages}, nil
}

func DetectLocale() string {
	for _, candidate := range []string{
		os.Getenv("AGY_LOCALE"),
		os.Getenv("LC_ALL"),
		os.Getenv("LC_MESSAGES"),
		os.Getenv("LANG"),
	} {
		if locale := NormalizeLocale(candidate); locale != "" {
			return locale
		}
	}
	return LocaleZHCN
}

func NormalizeLocale(locale string) string {
	normalized := strings.TrimSpace(locale)
	normalized = strings.ReplaceAll(normalized, "_", "-")
	if idx := strings.Index(normalized, "."); idx >= 0 {
		normalized = normalized[:idx]
	}
	if normalized == "" {
		return ""
	}

	lower := strings.ToLower(normalized)
	switch {
	case strings.HasPrefix(lower, "zh"):
		return LocaleZHCN
	case strings.HasPrefix(lower, "en"):
		return LocaleENUS
	default:
		return LocaleENUS
	}
}

func NewLocalizer(locale string) (*Localizer, error) {
	bundle, err := DefaultBundle()
	if err != nil {
		return nil, err
	}
	return bundle.Localizer(locale), nil
}

func MustLocalizer(locale string) *Localizer {
	localizer, err := NewLocalizer(locale)
	if err != nil {
		panic(err)
	}
	return localizer
}

func (b *Bundle) Localizer(locale string) *Localizer {
	return &Localizer{
		bundle: b,
		locale: resolvedLocale(locale),
	}
}

func (b *Bundle) T(locale string, key string, args ...any) string {
	return b.Localizer(locale).T(key, args...)
}

func (l *Localizer) Locale() string {
	return l.locale
}

func (l *Localizer) T(key string, args ...any) string {
	text := l.lookup(key)
	if text == "" {
		text = key
	}
	if len(args) == 0 {
		return text
	}
	return format(text, args...)
}

func (l *Localizer) lookup(key string) string {
	if l == nil || l.bundle == nil {
		return ""
	}

	if msg := l.bundle.lookup(l.locale, key); msg != "" {
		return msg
	}
	return l.bundle.lookup(LocaleENUS, key)
}

func (b *Bundle) lookup(locale string, key string) string {
	entries, ok := b.messages[resolvedLocale(locale)]
	if !ok {
		return ""
	}
	return entries[key]
}

func resolvedLocale(locale string) string {
	normalized := NormalizeLocale(locale)
	if normalized == "" {
		return DetectLocale()
	}
	return normalized
}

func format(template string, args ...any) string {
	result := template
	for index, arg := range args {
		placeholder := fmt.Sprintf("{%d}", index)
		result = strings.ReplaceAll(result, placeholder, fmt.Sprint(arg))
	}
	return result
}
