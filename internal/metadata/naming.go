package metadata

import (
	"regexp"
	"strings"
	"unicode"
)

var tokenPattern = regexp.MustCompile(`<([a-zA-Z0-9_-]+)>`)

// RenderTemplate applies a simple <token> template to the provided metadata map.
// It returns the rendered string and a boolean indicating whether any tokens were resolved.
func RenderTemplate(pattern string, meta map[string]string, fallback string) (string, bool) {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return fallback, false
	}

	normalized := make(map[string]string, len(meta))
	for k, v := range meta {
		key := strings.ToLower(strings.TrimSpace(k))
		if key == "" {
			continue
		}
		val := sanitize(v)
		if val != "" {
			normalized[key] = val
		}
	}

	resolved := false
	rendered := tokenPattern.ReplaceAllStringFunc(pattern, func(tok string) string {
		match := tokenPattern.FindStringSubmatch(tok)
		if len(match) != 2 {
			return ""
		}
		key := strings.ToLower(match[1])
		if val := normalized[key]; val != "" {
			resolved = true
			return val
		}
		return ""
	})

	rendered = cleanup(rendered)
	if rendered == "" {
		return fallback, false
	}
	return rendered, resolved
}

func sanitize(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Map(func(r rune) rune {
		switch r {
		case '<', '>', '"', '/', '\\', '|', '?', '*', ':':
			return -1
		}
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, value)

	// Collapse repeated whitespace
	value = strings.Join(strings.Fields(value), " ")
	return strings.Trim(value, " .-_")
}

func cleanup(s string) string {
	// Remove leftover template brackets or duplicate separators.
	s = strings.ReplaceAll(s, "<>", "")
	for strings.Contains(s, "  ") {
		s = strings.ReplaceAll(s, "  ", " ")
	}
	for strings.Contains(s, "__") {
		s = strings.ReplaceAll(s, "__", "_")
	}
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	return strings.Trim(s, " .-_")
}
