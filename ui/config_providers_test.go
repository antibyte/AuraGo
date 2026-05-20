package ui

import (
	"strings"
	"testing"
)

func TestConfigProviderActionsUseReadableAccentContrast(t *testing.T) {
	t.Parallel()

	css := readDesktopAssetText(t, "css/config.css")

	pill := configProviderCSSRuleBody(t, css, ".prov-provider-pill")
	for _, want := range []string{
		"background: linear-gradient(135deg, #ccfbf1, #5eead4);",
		"color: #08312d;",
		"border: 1px solid rgba(20, 184, 166, 0.42);",
		"text-shadow: none;",
	} {
		if !strings.Contains(pill, want) {
			t.Fatalf("provider pill contrast style missing %q in block:\n%s", want, pill)
		}
	}
	if strings.Contains(pill, "color: #fff") || strings.Contains(pill, "background: var(--accent)") {
		t.Fatalf("provider pill must not use white text on the bright accent background:\n%s", pill)
	}

	addButton := configProviderCSSRuleBody(t, css, ".prov-section-actions .btn-save.prov-btn-sm")
	for _, want := range []string{
		"background: linear-gradient(135deg, #0f766e, #115e59);",
		"color: #f0fdfa;",
		"border: 1px solid rgba(94, 234, 212, 0.42);",
		"text-shadow: none;",
	} {
		if !strings.Contains(addButton, want) {
			t.Fatalf("provider add button contrast style missing %q in block:\n%s", want, addButton)
		}
	}
}

func TestConfigCSSCacheBustForProviderContrast(t *testing.T) {
	t.Parallel()

	html := readDesktopAssetText(t, "config.html")
	if !strings.Contains(html, `/css/config.css?v=20260520a`) {
		t.Fatal("config.html must bust config.css cache for provider contrast styling")
	}
}

func configProviderCSSRuleBody(t *testing.T, source, selector string) string {
	t.Helper()

	needle := "\n" + selector + " {"
	start := strings.Index(source, needle)
	if start < 0 {
		t.Fatalf("config CSS missing selector %q", selector)
	}
	start++
	open := strings.Index(source[start:], "{")
	if open < 0 {
		t.Fatalf("config CSS selector %q is missing opening brace", selector)
	}
	bodyStart := start + open + 1
	close := strings.Index(source[bodyStart:], "}")
	if close < 0 {
		t.Fatalf("config CSS selector %q is missing closing brace", selector)
	}
	return source[bodyStart : bodyStart+close]
}
