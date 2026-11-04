package ui

import (
	"regexp"
	"strings"
	"testing"
)

const (
	standardThemeDarkHeader  = ".pw-page {\n"
	standardThemeLightHeader = "[data-theme=\"light\"] .pw-page,\n.pw-page[data-theme=\"light\"] {\n"
)

var standardThemeHexToken = regexp.MustCompile(`(--pw-[a-z-]+):\s*(#[0-9a-fA-F]{6});`)

func standardThemeBlock(t *testing.T, css, header string) string {
	t.Helper()
	start := strings.Index(css, header)
	if start < 0 {
		t.Fatalf("token block %q not found", header)
	}
	end := strings.Index(css[start:], "\n}")
	if end < 0 {
		t.Fatalf("token block %q is not closed", header)
	}
	return css[start : start+end]
}

func TestPagesStandardThemeTokenContrast(t *testing.T) {
	t.Parallel()

	css := normalizeAssetText(mustReadUIFile(t, "css/precision-workspace.css"))
	for mode, header := range map[string]string{"dark": standardThemeDarkHeader, "light": standardThemeLightHeader} {
		tokens := map[string][3]float64{}
		for _, match := range standardThemeHexToken.FindAllStringSubmatch(standardThemeBlock(t, css, header), -1) {
			rgb, ok := parseCSSHexColor(match[2])
			if !ok {
				t.Fatalf("%s: cannot parse %s %s", mode, match[1], match[2])
			}
			tokens[match[1]] = rgb
		}
		check := func(fg, bg string) {
			foreground, okFg := tokens[fg]
			background, okBg := tokens[bg]
			if !okFg || !okBg {
				t.Errorf("%s: %s and %s must both be six-digit hex tokens", mode, fg, bg)
				return
			}
			if ratio := cssContrastRatio(foreground, background); ratio < 4.5 {
				t.Errorf("%s: %s on %s = %.2f:1, want at least 4.5:1", mode, fg, bg, ratio)
			}
		}
		for _, fg := range []string{"--pw-text", "--pw-muted", "--pw-subtle", "--pw-accent"} {
			for _, bg := range []string{"--pw-canvas", "--pw-surface"} {
				check(fg, bg)
			}
		}
		check("--pw-on-accent", "--pw-accent")
		check("--pw-on-accent", "--pw-accent-strong")
	}

	// Config once carried its own palette; the ratios above only hold if it inherits.
	config := normalizeAssetText(mustReadUIFile(t, "css/config-workspace.css"))
	for _, token := range []string{"--pw-canvas:", "--pw-surface:", "--pw-text:", "--pw-accent:", "--pw-line:", "--pw-radius-lg:"} {
		if strings.Contains(config, token) {
			t.Errorf("config-workspace.css must inherit %s from precision-workspace.css", token)
		}
	}
}

func TestPagesStandardThemeGradientsLiveInMaterialTokens(t *testing.T) {
	t.Parallel()

	css := normalizeAssetText(mustReadUIFile(t, "css/precision-workspace.css"))
	for number, line := range strings.Split(css, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.Contains(trimmed, "gradient(") && !strings.HasPrefix(trimmed, "--pw-mat-") {
			t.Errorf("precision-workspace.css:%d: gradients belong in --pw-mat-* token values: %s", number+1, trimmed)
		}
	}
	dark := standardThemeBlock(t, css, standardThemeDarkHeader)
	light := standardThemeBlock(t, css, standardThemeLightHeader)
	for _, name := range []string{"bar", "side", "card", "well", "field", "btn", "primary", "selected", "hover", "divider"} {
		token := "--pw-mat-" + name + ":"
		if !strings.Contains(dark, token) {
			t.Errorf("dark block lacks %s", token)
		}
		if !strings.Contains(light, token) {
			t.Errorf("light block lacks %s", token)
		}
	}
}

const chatStandardPrefix = `:root:is([data-theme="dark"], :not([data-theme]))`

func standardThemeSplitSelectors(header string) []string {
	var selectors []string
	depth, start := 0, 0
	for i, char := range header {
		switch char {
		case '(':
			depth++
		case ')':
			depth--
		case ',':
			if depth == 0 {
				selectors = append(selectors, strings.TrimSpace(header[start:i]))
				start = i + 1
			}
		}
	}
	return append(selectors, strings.TrimSpace(header[start:]))
}

func TestChatStandardThemeIsScoped(t *testing.T) {
	t.Parallel()

	raw := normalizeAssetText(mustReadUIFile(t, "css/chat-standard.css"))
	css := regexp.MustCompile(`(?s)/\*.*?\*/`).ReplaceAllString(raw, "")
	depth, start, rules := 0, 0, 0
	for i, char := range css {
		switch char {
		case '{':
			if depth == 0 {
				header := strings.TrimSpace(css[start:i])
				rules++
				if strings.HasPrefix(header, "@") {
					if !strings.HasPrefix(header, "@keyframes chat-standard-") {
						t.Errorf("only @keyframes chat-standard-* at-rules are allowed: %q", header)
					}
				} else {
					for _, selector := range standardThemeSplitSelectors(header) {
						if selector != chatStandardPrefix && !strings.HasPrefix(selector, chatStandardPrefix+" ") {
							t.Errorf("selector must start with the default-theme prefix: %q", selector)
						}
					}
				}
			}
			depth++
		case '}':
			depth--
			if depth == 0 {
				start = i + 1
			}
		}
	}
	if rules == 0 {
		t.Fatal("chat-standard.css has no rules")
	}

	bundle := normalizeAssetText(mustReadUIFile(t, "css/chat.bundle.css"))
	standardAt := strings.Index(bundle, `/* ui/css/chat-standard.css */`)
	if standardAt < 0 || standardAt < strings.Index(bundle, `/* ui/css/chat-header-controls.css */`) {
		t.Errorf("chat-standard.css must be the last chat bundle part, found at %d", standardAt)
	}
}
