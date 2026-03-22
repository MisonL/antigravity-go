package session

import (
	"regexp"
	"strings"
)

var (
	reOpenAIKey        = regexp.MustCompile(`\bsk-[A-Za-z0-9]{20,}\b`)
	reGoogleAPIKey     = regexp.MustCompile(`\bAIza[0-9A-Za-z\-_]{20,}\b`)
	reGitHubToken      = regexp.MustCompile(`\bghp_[A-Za-z0-9]{20,}\b`)
	reGitHubPAT        = regexp.MustCompile(`\bgithub_pat_[A-Za-z0-9_]{20,}\b`)
	reSlackToken       = regexp.MustCompile(`\bxox[baprs]-[A-Za-z0-9-]{10,}\b`)
	reAWSAccessKeyID   = regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`)
	reBearerToken      = regexp.MustCompile(`(?i)\bBearer\s+[A-Za-z0-9\-_\.=:+/]{12,}\b`)
	rePrivateKeyBlock  = regexp.MustCompile(`(?s)-----BEGIN [^-]*PRIVATE KEY-----.*?-----END [^-]*PRIVATE KEY-----`)
	reAuthHeaderLine   = regexp.MustCompile(`(?im)^(authorization|proxy-authorization)\s*:\s*.*$`)
	reCookieHeaderLine = regexp.MustCompile(`(?im)^(cookie|set-cookie)\s*:\s*.*$`)
)

func RedactString(s string) string {
	if s == "" {
		return s
	}

	// 常见 header 直接整行抹掉
	s = reAuthHeaderLine.ReplaceAllString(s, `${1}: <REDACTED>`)
	s = reCookieHeaderLine.ReplaceAllString(s, `${1}: <REDACTED>`)

	// PEM 私钥块整段抹掉
	s = rePrivateKeyBlock.ReplaceAllString(s, "<REDACTED:PRIVATE_KEY>")

	// 常见 token/key 形态
	s = reOpenAIKey.ReplaceAllString(s, "<REDACTED:OPENAI_KEY>")
	s = reGoogleAPIKey.ReplaceAllString(s, "<REDACTED:GOOGLE_API_KEY>")
	s = reGitHubToken.ReplaceAllString(s, "<REDACTED:GITHUB_TOKEN>")
	s = reGitHubPAT.ReplaceAllString(s, "<REDACTED:GITHUB_PAT>")
	s = reSlackToken.ReplaceAllString(s, "<REDACTED:SLACK_TOKEN>")
	s = reAWSAccessKeyID.ReplaceAllString(s, "<REDACTED:AWS_ACCESS_KEY_ID>")
	s = reBearerToken.ReplaceAllString(s, "Bearer <REDACTED>")

	return s
}

func RedactAny(v any) any {
	switch t := v.(type) {
	case nil:
		return nil
	case string:
		return RedactString(t)
	case []any:
		out := make([]any, 0, len(t))
		for _, it := range t {
			out = append(out, RedactAny(it))
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, it := range t {
			lk := strings.ToLower(k)
			if lk == "api_key" || lk == "token" || lk == "auth_token" || lk == "authorization" || lk == "cookie" || lk == "set-cookie" {
				out[k] = "<REDACTED>"
				continue
			}
			out[k] = RedactAny(it)
		}
		return out
	case map[string]string:
		out := make(map[string]string, len(t))
		for k, it := range t {
			lk := strings.ToLower(k)
			if lk == "api_key" || lk == "token" || lk == "auth_token" || lk == "authorization" || lk == "cookie" || lk == "set-cookie" {
				out[k] = "<REDACTED>"
				continue
			}
			out[k] = RedactString(it)
		}
		return out
	default:
		return v
	}
}
