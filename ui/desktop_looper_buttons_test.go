package ui

import (
	"strings"
	"testing"
)

func TestDesktopLooperActionButtonsShareConsistentStyle(t *testing.T) {
	t.Parallel()

	css := readDesktopAssetText(t, "css/desktop-apps.css")
	shared := desktopLooperCSSRuleBody(t, css, ".vd-looper-start,\n.vd-looper-stop")
	for _, want := range []string{
		"width: 100%;",
		"min-height: 32px;",
		"display: inline-flex;",
		"align-items: center;",
		"justify-content: center;",
		"gap: 8px;",
		"border-radius: 7px;",
		"border: 1px solid var(--vd-border, rgba(255,255,255,0.08));",
		"background: var(--vd-surface-strong, rgba(0,0,0,0.25));",
		"color: var(--vd-text, #e8ecf1);",
		"font-size: 12px;",
		"font-weight: 700;",
	} {
		if !strings.Contains(shared, want) {
			t.Fatalf("looper shared action button style missing %q in block:\n%s", want, shared)
		}
	}

	for selector, markers := range map[string][]string{
		".vd-looper-start": {
			"--vd-looper-action-accent: var(--vd-accent, #27c7a6);",
		},
		".vd-looper-stop": {
			"--vd-looper-action-accent: #ef4444;",
		},
		".vd-looper-start:hover:not(:disabled),\n.vd-looper-stop:hover:not(:disabled)": {
			"border-color: var(--vd-looper-action-accent);",
			"background: rgba(255,255,255,0.06);",
		},
		".vd-looper-start:disabled,\n.vd-looper-stop:disabled": {
			"cursor: not-allowed;",
			"background: var(--vd-surface-strong, rgba(0,0,0,0.25));",
			"border-color: var(--vd-border, rgba(255,255,255,0.08));",
		},
	} {
		block := desktopLooperCSSRuleBody(t, css, selector)
		for _, want := range markers {
			if !strings.Contains(block, want) {
				t.Fatalf("looper action button selector %q missing %q in block:\n%s", selector, want, block)
			}
		}
	}
}

func TestDesktopLooperStylesBustComponentCache(t *testing.T) {
	t.Parallel()

	desktopCSS := readDesktopAssetText(t, "css/desktop.css")
	if !strings.Contains(desktopCSS, "@import url('desktop-apps.css?v=20260523-romm-external');") {
		t.Fatal("desktop.css must bust the desktop-apps.css cache for Looper action button styling")
	}

	desktopHTML := readDesktopAssetText(t, "desktop.html")
	if !strings.Contains(desktopHTML, `/css/desktop.css?v={{.BuildVersion}}-desktop-20260523-icon-multiselect`) {
		t.Fatal("desktop.html must bust the desktop.css aggregator cache for Looper action button styling")
	}
}

func desktopLooperCSSRuleBody(t *testing.T, source, selector string) string {
	t.Helper()

	needle := "\n" + selector + " {"
	start := strings.LastIndex(source, needle)
	if start < 0 {
		t.Fatalf("desktop looper CSS missing selector %q", selector)
	}
	start++
	open := strings.Index(source[start:], "{")
	if open < 0 {
		t.Fatalf("desktop looper CSS selector %q is missing opening brace", selector)
	}
	bodyStart := start + open + 1
	close := strings.Index(source[bodyStart:], "}")
	if close < 0 {
		t.Fatalf("desktop looper CSS selector %q is missing closing brace", selector)
	}
	return source[bodyStart : bodyStart+close]
}
