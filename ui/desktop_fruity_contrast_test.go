package ui

import (
	"strings"
	"testing"
)

func TestDesktopFruityDarkRemapsRaisedEditorSurfaces(t *testing.T) {
	t.Parallel()

	css := readDesktopAssetText(t, "css/desktop-base.css")
	darkBody := desktopExactCSSRuleBody(t, css, `.desktop-body[data-theme="fruity"][data-fruity-mode="dark"]`)
	for _, want := range []string{
		"--ds-color-bg-raised:",
		"--ds-color-bg-overlay:",
		"--ds-color-bg-modal:",
		"--ds-color-fg-primary: #f5f5f7;",
	} {
		if !strings.Contains(darkBody, want) {
			t.Fatalf("fruity dark must remap editor surfaces, missing %q in %q", want, darkBody)
		}
	}
}

func TestDesktopFruityLightKeepsDarkTextOnLightRaisedSurfaces(t *testing.T) {
	t.Parallel()

	css := readDesktopAssetText(t, "css/desktop-base.css")
	for _, want := range []string{
		"--ds-color-fg-primary: #1a2030;",
		"--ds-color-bg-raised: rgba(255, 255, 255, 0.72);",
	} {
		if !strings.Contains(css, want) {
			t.Fatalf("fruity light editor tokens missing %q", want)
		}
	}
}

func TestDesktopCodeStudioEditorFollowsFruityLightDark(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/code-studio/editor.js")
	for _, want := range []string{
		"function usesLightEditorTheme()",
		"function codeMirrorHighlightExtensions(",
		"body.dataset.fruityMode !== 'dark'",
		"cm.syntaxHighlighting(cm.defaultHighlightStyle)",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("Code Studio editor missing theme-aware marker %q", want)
		}
	}
	if strings.Contains(source, "            cm.oneDark,\n            cm.closeBrackets") {
		t.Fatalf("Code Studio must not force oneDark on every desktop theme")
	}
	if strings.Contains(source, "cm.oneDark,\n            cm.closeBrackets") {
		t.Fatalf("Code Studio must not force oneDark on every desktop theme")
	}

	css := readDesktopAssetText(t, "css/code-studio.css")
	if !strings.Contains(css, `.desktop-body[data-theme="fruity"]:not([data-fruity-mode="light"]) .code-studio`) {
		t.Fatalf("Code Studio OS-dark fallback must not restyle explicit fruity light")
	}
	if strings.Contains(css, "@media (prefers-color-scheme: dark) {\n    .desktop-body[data-theme=\"fruity\"] .code-studio {") {
		t.Fatalf("Code Studio must not apply OS dark tokens to all fruity desktops")
	}
}

func TestDesktopFruityWorkbenchesDoNotPaintDarkGlassOnLightText(t *testing.T) {
	t.Parallel()

	openscad := readDesktopAssetText(t, "css/desktop-app-openscad.css")
	fruityOpenSCAD := cssRuleBodyInFruityThemeTest(t, openscad, `.desktop-body[data-theme="fruity"] .openscad-app`)
	if strings.Contains(fruityOpenSCAD, "rgba(18, 24, 34") {
		t.Fatalf("fruity light OpenSCAD must not force dark glass over --vd-text: %q", fruityOpenSCAD)
	}
	for _, want := range []string{
		"--oscad-text: var(--vd-text",
		"--oscad-surface:",
	} {
		if !strings.Contains(fruityOpenSCAD, want) {
			t.Fatalf("fruity OpenSCAD missing token remap %q in %q", want, fruityOpenSCAD)
		}
	}

	homepage := readDesktopAssetText(t, "css/desktop-app-homepage-studio.css")
	fruityHomepage := cssRuleBodyInFruityThemeTest(t, homepage, `.desktop-body[data-theme="fruity"] .vd-hp-studio`)
	if strings.Contains(fruityHomepage, "rgba(18, 24, 34") {
		t.Fatalf("fruity light Homepage Studio must not force dark glass over --vd-text: %q", fruityHomepage)
	}

	editor := readDesktopAssetText(t, "js/desktop/apps/openscad-editor.js")
	if strings.Contains(editor, "backgroundColor: 'rgba(5,11,18,0.68)'") || strings.Contains(editor, "color: '#eef7f7'") {
		t.Fatalf("OpenSCAD CodeMirror theme must follow --oscad-* tokens instead of hardcoded dark colors")
	}
	if !strings.Contains(editor, "backgroundColor: 'var(--oscad-surface)'") || !strings.Contains(editor, "color: 'var(--oscad-text)'") {
		t.Fatalf("OpenSCAD CodeMirror theme must paint from --oscad-surface/--oscad-text")
	}
}

func TestDesktopOfficeChromeUsesThemeSurfacesInsteadOfDarkWash(t *testing.T) {
	t.Parallel()

	css := readDesktopAssetText(t, "css/desktop-app-office.css")
	if strings.Contains(css, "rgba(8, 12, 18") {
		t.Fatalf("office chrome must not hardcode a dark wash that disappears on fruity light")
	}
	for _, want := range []string{
		"--vd-editor-toolbar-bg: var(--vd-theme-chrome-bg);",
		"background: var(--vd-theme-chrome-bg);",
		"--vd-editor-bg: var(--vd-theme-app-bg);",
	} {
		if !strings.Contains(css, want) {
			t.Fatalf("office chrome missing theme surface marker %q", want)
		}
	}
}

func TestDesktopCheaterCodeBlocksKeepReadableForeground(t *testing.T) {
	t.Parallel()

	css := readDesktopAssetText(t, "css/desktop-app-cheater.css")
	if !strings.Contains(css, "--cheater-code-fg: #e8eef6;") {
		t.Fatalf("cheater code blocks must keep a light foreground on the dark code surface")
	}
}
