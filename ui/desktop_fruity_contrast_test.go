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

func TestDesktopProductivityAppsUseThemeSurfacesInsteadOfDarkWash(t *testing.T) {
	t.Parallel()

	missionControl := readDesktopAssetText(t, "css/desktop-app-mission-control.css")
	for _, banned := range []string{
		"#181c24",
		"#22262e",
		"--vd-text: #e8ecf1",
	} {
		if strings.Contains(missionControl, banned) {
			t.Fatalf("mission control must not force dark-only palette marker %q", banned)
		}
	}
	for _, want := range []string{
		"background: var(--vd-theme-app-bg);",
		"--vd-surface: var(--vd-theme-panel-bg);",
	} {
		if !strings.Contains(missionControl, want) {
			t.Fatalf("mission control missing theme surface marker %q", want)
		}
	}

	chat := readDesktopAssetText(t, "css/desktop-app-chat.css")
	if strings.Contains(chat, "rgba(9, 13, 22") || strings.Contains(chat, "rgba(10, 15, 26") {
		t.Fatalf("chat chrome must not hardcode a dark wash that disappears on fruity light")
	}
	if !strings.Contains(chat, "background: var(--vd-theme-panel-bg);") {
		t.Fatalf("chat agent bubbles must use theme panel material")
	}

	store := readDesktopAssetText(t, "css/desktop-app-software-store.css")
	if strings.Contains(store, "rgba(4, 8, 14") {
		t.Fatalf("software store must not hardcode a dark app wash")
	}
	if !strings.Contains(store, "background: var(--vd-theme-app-bg);") {
		t.Fatalf("software store missing theme app background marker")
	}
}

func TestDesktopUtilityAppsUseThemeSurfacesInsteadOfDarkWash(t *testing.T) {
	t.Parallel()

	chrome := readDesktopAssetText(t, "css/desktop-chrome.css")
	if strings.Contains(chrome, ".vd-notes-toolbar {\n    background: var(--ds-color-surface-2);") {
		t.Fatalf("notes/terminal toolbars must not use design-system dark wash")
	}
	for _, want := range []string{
		"background: var(--vd-theme-chrome-bg);",
		"background: var(--vd-theme-app-bg);",
		"background: #0f172a;",
	} {
		if !strings.Contains(chrome, want) {
			t.Fatalf("terminal/notes chrome missing theme marker %q", want)
		}
	}

	viewer := readDesktopAssetText(t, "css/desktop-app-viewer.css")
	if strings.Contains(viewer, "rgba(255,255,255,0.1)") {
		t.Fatalf("viewer chrome must not use fixed white-glass borders")
	}
	if !strings.Contains(viewer, "background: var(--vd-theme-app-bg);") {
		t.Fatalf("viewer missing theme app background marker")
	}

	looper := readDesktopAssetText(t, "css/desktop-app-looper.css")
	if strings.Contains(looper, "rgba(20,24,30,0.85)") {
		t.Fatalf("looper must not hardcode a dark app wash")
	}
	if !strings.Contains(looper, "background: var(--vd-theme-app-bg);") {
		t.Fatalf("looper missing theme app background marker")
	}
}

func TestDesktopCheaterCodeBlocksKeepReadableForeground(t *testing.T) {
	t.Parallel()

	css := readDesktopAssetText(t, "css/desktop-app-cheater.css")
	if !strings.Contains(css, "--cheater-code-fg: #e8eef6;") {
		t.Fatalf("cheater code blocks must keep a light foreground on the dark code surface")
	}
}
