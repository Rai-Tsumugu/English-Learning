package llm

import (
	"regexp"
	"strings"
)

var (
	reAPIKey = regexp.MustCompile(`sk-[A-Za-z0-9_\-]{10,}`)
	reBearer = regexp.MustCompile(`(?i)bearer\s+[A-Za-z0-9_\-\.]+`)
	reEmail  = regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`)
)

// Mask returns s with API keys, bearer tokens and email addresses replaced
// by fixed placeholders. Intended for use right before writing logs.
func Mask(s string) string {
	if s == "" {
		return s
	}
	s = reAPIKey.ReplaceAllString(s, "sk-***")
	s = reBearer.ReplaceAllString(s, "Bearer ***")
	s = reEmail.ReplaceAllStringFunc(s, func(m string) string {
		at := strings.IndexByte(m, '@')
		if at <= 0 {
			return "***"
		}
		head := m[:at]
		if len(head) <= 1 {
			return "*@***"
		}
		return string(head[0]) + "***@***"
	})
	return s
}
