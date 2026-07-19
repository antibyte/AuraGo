package ui

import (
	"strings"
	"testing"
)

func TestDesktopLooperActionButtonsShareConsistentStyle(t *testing.T) {
	t.Parallel()

	css := strings.ReplaceAll(readDesktopAssetText(t, "css/desktop-app-looper.css"), "\r\n", "\n")
	shared := desktopLooperCSSRuleBody(t, css, ".vd-looper-start,\n.vd-looper-stop,\n.vd-looper-pause,\n.vd-looper-resume")
	for _, want := range []string{
		"width: 100%;",
		"min-height: 32px;",
		"display: inline-flex;",
		"align-items: center;",
		"justify-content: center;",
		"gap: 8px;",
		"border-radius: var(--ds-radius-sm);",
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
		".vd-looper-start,\n.vd-looper-resume": {
			"--vd-looper-action-accent: var(--vd-accent, #27c7a6);",
		},
		".vd-looper-stop": {
			"--vd-looper-action-accent: #ef4444;",
		},
		".vd-looper-pause": {
			"--vd-looper-action-accent: #f59e0b;",
		},
		".vd-looper-start:hover:not(:disabled),\n.vd-looper-stop:hover:not(:disabled),\n.vd-looper-pause:hover:not(:disabled),\n.vd-looper-resume:hover:not(:disabled)": {
			"border-color: var(--vd-looper-action-accent);",
			"background: rgba(255,255,255,0.06);",
		},
		".vd-looper-start:disabled,\n.vd-looper-stop:disabled,\n.vd-looper-pause:disabled,\n.vd-looper-resume:disabled": {
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

	loader := readDesktopAssetText(t, "js/desktop/core/module-loader.js")
	if !strings.Contains(loader, "'/css/desktop-app-looper.css'") {
		t.Fatal("desktop module loader must lazy-load Looper component CSS")
	}

	desktopHTML := readDesktopAssetText(t, "desktop.html")
	if !strings.Contains(desktopHTML, `/css/desktop-shell.bundle.css?v={{.BuildVersion}}`) {
		t.Fatal("desktop.html must load cache-busted shell CSS")
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
