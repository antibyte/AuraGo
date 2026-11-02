package ui

import (
	"regexp"
	"strings"
	"testing"
)

// splitSelectorList splits a selector list at its top-level commas (the commas inside :where(...) stay).
func splitSelectorList(list string) []string {
	var parts []string
	depth, start := 0, 0
	for i, r := range list {
		switch r {
		case '(':
			depth++
		case ')':
			depth--
		case ',':
			if depth == 0 {
				parts = append(parts, list[start:i])
				start = i + 1
			}
		}
	}
	return append(parts, list[start:])
}

// The stylesheet takes every colour from tokens, scopes its rules under .stm-, uses the app materials, answers the window width
// with container queries and gates its motion.
func TestStorymakerStylesheetUsesOnlyTokens(t *testing.T) {
	t.Parallel()
	css := readDesktopAssetText(t, "css/desktop-app-storymaker.css")
	body := regexp.MustCompile(`(?s)/\*.*?\*/`).ReplaceAllString(css, "")
	for label, pattern := range map[string]string{
		"hex colour":      `#[0-9a-fA-F]{3,8}\b`,
		"colour function": `\b(?:rgba?|hsla?|hwb|lab|lch|oklab|oklch)\(\s*[\d.]`,
		"named colour":    `(?i)[:\s(,](?:white|black|red|green|blue|gray|grey|orange|yellow|purple)\b\s*[;,)]`,
		"external url":    `url\(`,
		"import":          `@import`,
		"theme id":        `data-theme=`,
	} {
		if match := regexp.MustCompile(pattern).FindString(body); match != "" {
			t.Errorf("css/desktop-app-storymaker.css has a %s: %q", label, match)
		}
	}
	if strings.Count(body, "!important") != 1 || !strings.Contains(body, ".stm-app [hidden] { display: none !important; }") {
		t.Error("the only !important allowed is the [hidden] rule")
	}
	for _, marker := range []string{
		"var(--vd-mat-canvas", "var(--vd-mat-bar", "var(--vd-mat-side", "var(--vd-mat-card", "var(--vd-mat-well", "var(--vd-mat-field", "var(--vd-mat-btn",
		"var(--vd-mat-primary", "var(--vd-mat-selected", "var(--ds-color-success-fg)", "var(--ds-color-danger-fg)", "var(--ds-color-border-focus)",
		"container: stm / inline-size", "@container stm (max-width: 940px)", "@container stm (max-width: 640px)", "@media (pointer: coarse)",
	} {
		if !strings.Contains(body, marker) {
			t.Errorf("css/desktop-app-storymaker.css is missing %q", marker)
		}
	}
	// Every rule that starts a line is scoped to the app: its selectors name an .stm- class.
	selector := regexp.MustCompile(`(?m)^([^\s@/}][^{]*)\{`)
	for _, match := range selector.FindAllStringSubmatch(body, -1) {
		for _, part := range splitSelectorList(match[1]) {
			part = strings.TrimSpace(part)
			if part != "" && !strings.Contains(part, ".stm-") {
				t.Errorf("the selector %q is not scoped to .stm-", part)
			}
		}
	}
	gate := strings.Index(body, "@media (prefers-reduced-motion: no-preference)")
	if gate < 0 {
		t.Fatal("css/desktop-app-storymaker.css has no reduced-motion gate")
	}
	for _, motion := range regexp.MustCompile(`\b(?:transition|animation)\s*:`).FindAllStringIndex(body, -1) {
		if motion[0] < gate {
			t.Errorf("a transition or animation at offset %d sits outside @media (prefers-reduced-motion: no-preference)", motion[0])
		}
	}
}
