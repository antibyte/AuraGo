package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesktopFruityThemeSettingAssets(t *testing.T) {
	t.Parallel()

	shellText := readDesktopAssetText(t, "js/desktop/main.js")
	settingsText := readDesktopAssetText(t, "js/desktop/apps/settings.js")
	for _, want := range []string{
		"'appearance.theme': 'standard'",
		"body.dataset.theme = settingValue('appearance.theme')",
		"function isFruityTheme()",
		"function renderStandardTaskbar()",
		"function renderFruityDock()",
		"function scheduleFruityDockOcclusionCheck()",
		"function updateFruityDockOcclusion()",
		"function windowOverlapsFruityDock(",
		"function dockApps()",
		"class=\"vd-dock-orb\"",
		"data-fruity-dock-orb",
		"fruity-dock-collapsed",
		"function reconcileFruityDock()",
		"function updateDockButton(btn, app, index, runningWindows)",
		"btn.className = 'vd-dock-button';",
		"btn.dataset.appId = app.id",
		"const runningWindows = [...state.windows.values()]",
		"runningWindows.some(win => win.appId === app.id)",
		"win.appId === app.id && win.id === state.activeWindowId",
	} {
		if !strings.Contains(shellText, want) {
			t.Fatalf("desktop shell is missing Fruity theme setting marker %q", want)
		}
	}
	if !strings.Contains(settingsText, "desktop.settings_theme_standard") {
		t.Fatalf("settings app is missing theme translation key %q", "desktop.settings_theme_standard")
	}
	if !strings.Contains(settingsText, "desktop.settings_theme_fruity") {
		t.Fatalf("settings app is missing theme translation key %q", "desktop.settings_theme_fruity")
	}
	if !strings.Contains(settingsText, "settingSelect('appearance.theme'") {
		t.Fatalf("settings app is missing theme setting marker %q", "settingSelect('appearance.theme'")
	}

	cssText := readAllDesktopCSS(t)
	for _, want := range []string{
		".desktop-body[data-theme=\"fruity\"]",
		"@media (prefers-color-scheme: dark)",
		".desktop-body[data-theme=\"fruity\"] .vd-window",
		".desktop-body[data-theme=\"fruity\"] .vd-window-titlebar",
		".desktop-body[data-theme=\"fruity\"] .vd-window .vd-window-actions",
		".desktop-body[data-theme=\"fruity\"] .vd-widget-manager .vd-wm-header",
		".desktop-body[data-theme=\"fruity\"] .vd-widget-manager .vd-window-actions",
		".desktop-body[data-theme=\"fruity\"] .vd-window-button[data-action=\"close\"]",
		".desktop-body[data-theme=\"fruity\"] .vd-window-button[data-action=\"minimize\"]",
		".desktop-body[data-theme=\"fruity\"] .vd-window-button[data-action=\"maximize\"]",
		".desktop-body[data-theme=\"fruity\"] .vd-window-button::after",
		"--fruity-window-close",
		".desktop-body[data-theme=\"fruity\"] .vd-shell",
		"grid-template-rows: minmax(0, 1fr);",
		".desktop-body[data-theme=\"fruity\"] .vd-resize-nw",
		".desktop-body[data-theme=\"fruity\"] .vd-taskbar > *",
		".desktop-body[data-theme=\"fruity\"] .vd-taskbar",
		".desktop-body[data-theme=\"fruity\"] .vd-taskbar-apps",
		".desktop-body[data-theme=\"fruity\"] .vd-taskbar-apps::before",
		".desktop-body[data-theme=\"fruity\"] .vd-agent-button",
		".desktop-body[data-theme=\"fruity\"] .radial-backdrop.open",
		".desktop-body[data-theme=\"fruity\"] .vd-radial-anchor .radial-menu",
		".desktop-body[data-theme=\"fruity\"].fruity-dock-collapsed .vd-taskbar-apps:not(:hover):not(:focus-within)",
		".desktop-body[data-theme=\"fruity\"] .vd-dock-orb",
		".desktop-body[data-theme=\"fruity\"] .vd-dock-button",
		".desktop-body[data-theme=\"fruity\"] .vd-dock-button:hover",
		".desktop-body[data-theme=\"fruity\"] .vd-dock-button.running::after",
		".desktop-body[data-theme=\"fruity\"] .vd-dock-button.active::after",
		".desktop-body[data-theme=\"fruity\"] .vd-dock-icon",
		".desktop-body[data-theme=\"fruity\"] .vd-radial-anchor",
		"top: calc(3px + env(safe-area-inset-top));",
		"z-index: calc(var(--vd-z-dock) + 3);",
		"backdrop-filter: blur(40px) saturate(1.42) contrast(1.04);",
		"pointer-events: none;",
		"pointer-events: auto;",
		"order: 20;",
		"backdrop-filter: none;",
		"animation: fruity-dock-orb-pulse",
		"@keyframes fruity-dock-collapse",
		"@keyframes fruity-dock-unfurl",
		"scale(1.28)",
		"@supports selector(.vd-dock-button:has(+ .vd-dock-button:hover))",
		".desktop-body[data-theme=\"fruity\"] .vd-modal",
	} {
		if !strings.Contains(cssText, want) {
			t.Fatalf("desktop stylesheet is missing Fruity theme marker %q", want)
		}
	}
	for _, forbidden := range []string{
		"vd-fruity-topbar",
		"vd-fruity-active-app",
		"--vd-fruity-topbar-height",
		"updateFruityTopbarAppLabel",
	} {
		if strings.Contains(shellText, forbidden) || strings.Contains(cssText, forbidden) {
			t.Fatalf("fruity decorative topbar marker %q must not remain in shell assets", forbidden)
		}
	}

	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		path := filepath.Join("lang", "desktop", lang+".json")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		var values map[string]string
		if err := json.Unmarshal(data, &values); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, key := range []string{
			"desktop.settings_theme",
			"desktop.settings_theme_desc",
			"desktop.settings_theme_standard",
			"desktop.settings_theme_fruity",
		} {
			if strings.TrimSpace(values[key]) == "" {
				t.Fatalf("%s missing non-empty translation for %s", path, key)
			}
		}
	}
}

func TestDesktopFruityWindowControlsStayOnLeft(t *testing.T) {
	t.Parallel()

	cssText := readAllDesktopCSS(t)
	for _, check := range []struct {
		name     string
		selector string
		wants    []string
	}{
		{
			name:     "window controls",
			selector: ".desktop-body[data-theme=\"fruity\"] .vd-window .vd-window-actions",
			wants:    []string{"position: absolute;", "left: 12px;", "right: auto;", "transform: translateY(-50%);", "justify-content: flex-start;"},
		},
		{
			name:     "menu window controls",
			selector: ".desktop-body[data-theme=\"fruity\"] .vd-window.has-window-menu .vd-window-actions",
			wants:    []string{"left: 12px;", "top: 24px;", "right: auto;"},
		},
		{
			name:     "manager dialog controls",
			selector: ".desktop-body[data-theme=\"fruity\"] .vd-widget-manager .vd-window-actions",
			wants:    []string{"position: absolute;", "left: 18px;", "transform: translateY(-50%);"},
		},
		{
			name:     "close dot order",
			selector: ".desktop-body[data-theme=\"fruity\"] .vd-window-button[data-action=\"close\"]",
			wants:    []string{"order: 1;", "background: var(--fruity-window-close);"},
		},
		{
			name:     "minimize dot order",
			selector: ".desktop-body[data-theme=\"fruity\"] .vd-window-button[data-action=\"minimize\"]",
			wants:    []string{"order: 2;", "background: var(--fruity-window-minimize);"},
		},
		{
			name:     "maximize dot order",
			selector: ".desktop-body[data-theme=\"fruity\"] .vd-window-button[data-action=\"maximize\"]",
			wants:    []string{"order: 3;", "background: var(--fruity-window-maximize);"},
		},
		{
			name:     "agent chat button stays right",
			selector: ".desktop-body[data-theme=\"fruity\"] .vd-window .vd-window-ai-button",
			wants: []string{
				"width: 30px;",
				"height: 26px;",
				"margin-left: 4px;",
				"order: 20;",
				"color: #0b4f7a;",
			},
		},
		{
			name:     "agent chat button suppresses dot marker",
			selector: ".desktop-body[data-theme=\"fruity\"] .vd-window .vd-window-ai-button::after",
			wants:    []string{"display: none;"},
		},
		{
			name:     "agent chat button keeps semantic icon visible",
			selector: ".desktop-body[data-theme=\"fruity\"] .vd-window .vd-window-ai-button-icon",
			wants:    []string{"display: block;"},
		},
	} {
		body := cssRuleBodyInFruityThemeTest(t, cssText, check.selector)
		for _, want := range check.wants {
			if !strings.Contains(body, want) {
				t.Fatalf("fruity %s rule %q missing %q in body %q", check.name, check.selector, want, body)
			}
		}
	}

	cssText = readAllDesktopCSS(t)
	body := cssRuleBodyInFruityThemeTest(t, cssText, ".desktop-body[data-theme=\"fruity\"] .vd-window .vd-window-ai-button::before")
	if strings.Contains(body, `content: "AI";`) {
		t.Fatalf("fruity agent chat button must not render a text-only AI badge: %q", body)
	}

	actionsOverride := desktopExactCSSRuleBody(t, cssText, ".desktop-body[data-theme=\"fruity\"] .vd-window > .vd-window-titlebar > .vd-window-actions")
	for _, want := range []string{
		"left: 12px !important;",
		"right: auto !important;",
		"grid-column: 1 !important;",
		"justify-self: start !important;",
		"min-width: 0 !important;",
		"justify-content: flex-start !important;",
	} {
		if !strings.Contains(actionsOverride, want) {
			t.Fatalf("fruity final window actions override missing %q in body %q", want, actionsOverride)
		}
	}

	menuActionsOverride := desktopExactCSSRuleBody(t, cssText, ".desktop-body[data-theme=\"fruity\"] .vd-window.has-window-menu > .vd-window-titlebar > .vd-window-actions")
	for _, want := range []string{
		"left: 10px !important;",
		"right: 10px !important;",
		"grid-column: 1 !important;",
		"justify-self: start !important;",
		"top: 24px !important;",
		"min-width: 0 !important;",
	} {
		if !strings.Contains(menuActionsOverride, want) {
			t.Fatalf("fruity menu window actions override missing %q in body %q", want, menuActionsOverride)
		}
	}

	menuTitlebarOverride := desktopExactCSSRuleBody(t, cssText, ".desktop-body[data-theme=\"fruity\"] .vd-window.has-window-menu > .vd-window-titlebar")
	for _, want := range []string{
		"grid-template-columns: 78px minmax(0, 1fr) 78px !important;",
		"padding: 0 14px !important;",
		"overflow: visible !important;",
	} {
		if !strings.Contains(menuTitlebarOverride, want) {
			t.Fatalf("fruity menu titlebar override missing %q in body %q", want, menuTitlebarOverride)
		}
	}
}

func TestDesktopFruitySystemControlsSitInTopbarStatusArea(t *testing.T) {
	t.Parallel()

	cssText := readAllDesktopCSS(t)
	dockBody := cssRuleBodyInFruityThemeTest(t, cssText, ".desktop-body[data-theme=\"fruity\"] .vd-taskbar-apps")
	if !strings.Contains(dockBody, "max-width: calc(100vw - 300px);") {
		t.Fatalf("fruity dock must reserve horizontal space for the system card: %q", dockBody)
	}
	if !strings.Contains(cssText, "max-width: min(calc(100vw - 112px), calc(44px * 4 + 20px));") {
		t.Fatal("fruity mobile dock must keep room for several icons while avoiding the system card")
	}
	if !strings.Contains(cssText, "overflow-y: hidden;") {
		t.Fatal("fruity mobile desktop must suppress vertical page scrolling")
	}

	systemBody := cssRuleBodyInFruityThemeTest(t, cssText, ".desktop-body[data-theme=\"fruity\"] .vd-taskbar-system")
	for _, want := range []string{
		"position: fixed;",
		"top: calc(3px + env(safe-area-inset-top));",
		"bottom: auto;",
		"height: 26px;",
		"padding: 2px 4px;",
		"border-radius: 999px;",
		"background: rgba(255, 255, 255, 0.26);",
		"box-shadow: 0 10px 24px rgba(52, 64, 84, 0.12);",
		"backdrop-filter: blur(24px) saturate(1.22);",
		"z-index: calc(var(--vd-z-dock) + 3);",
	} {
		if !strings.Contains(systemBody, want) {
			t.Fatalf("fruity taskbar system card missing %q in body %q", want, systemBody)
		}
	}

	controlsBody := cssRuleBodyInFruityThemeTest(t, cssText, ".desktop-body[data-theme=\"fruity\"] .vd-taskbar-system .vd-taskbar-widget-btn,\n.desktop-body[data-theme=\"fruity\"] .vd-taskbar-system .vd-show-desktop-btn")
	for _, want := range []string{
		"width: 22px;",
		"height: 22px;",
		"min-width: 22px;",
		"background: transparent;",
		"border-color: transparent;",
		"box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.62);",
	} {
		if !strings.Contains(controlsBody, want) {
			t.Fatalf("fruity taskbar system controls missing %q in body %q", want, controlsBody)
		}
	}

	showDesktopBody := desktopExactCSSRuleBody(t, cssText, ".desktop-body[data-theme=\"fruity\"] .vd-taskbar-system > .vd-show-desktop-btn")
	for _, want := range []string{
		"margin-right: 0;",
		"font-size: 0;",
		"color: rgba(32, 38, 53, 0.72);",
		"display: inline-grid;",
		"place-items: center;",
	} {
		if !strings.Contains(showDesktopBody, want) {
			t.Fatalf("fruity show-desktop handle must align inside the system card, missing %q in body %q", want, showDesktopBody)
		}
	}

	showDesktopIconBody := desktopExactCSSRuleBody(t, cssText, ".desktop-body[data-theme=\"fruity\"] .vd-taskbar-system > .vd-show-desktop-btn::before")
	for _, want := range []string{
		"content: \"\";",
		"width: 14px;",
		"height: 14px;",
		"background: currentColor;",
		"-webkit-mask:",
		"mask:",
	} {
		if !strings.Contains(showDesktopIconBody, want) {
			t.Fatalf("fruity show-desktop icon must use a single masked desktop glyph, missing %q in body %q", want, showDesktopIconBody)
		}
	}

	radialAnchorBody := cssRuleBodyInFruityThemeTest(t, cssText, ".desktop-body[data-theme=\"fruity\"] .vd-taskbar-system .vd-radial-anchor")
	for _, want := range []string{"width: 22px;", "height: 22px;", "flex-basis: 22px;"} {
		if !strings.Contains(radialAnchorBody, want) {
			t.Fatalf("fruity radial anchor must align with the system card controls, missing %q in body %q", want, radialAnchorBody)
		}
	}

	radialMenuBody := desktopExactCSSRuleBody(t, cssText, ".desktop-body[data-theme=\"fruity\"] .vd-radial-anchor .radial-menu")
	for _, want := range []string{"position: static;", "inset: auto;", "width: 22px;", "height: 22px;"} {
		if !strings.Contains(radialMenuBody, want) {
			t.Fatalf("fruity radial menu must stay inside the system card flow, missing %q in body %q", want, radialMenuBody)
		}
	}
}

func TestDesktopWindowContentPreservesRoundedCornersInAllThemes(t *testing.T) {
	t.Parallel()

	cssText := readAllDesktopCSS(t)
	for _, check := range []struct {
		name     string
		selector string
		wants    []string
	}{
		{
			name:     "titlebar follows top shell radius",
			selector: ".vd-window:not(.maximized) > .vd-window-titlebar",
			wants: []string{
				"border-top-left-radius: inherit;",
				"border-top-right-radius: inherit;",
			},
		},
		{
			name:     "content clips to bottom shell radius",
			selector: ".vd-window:not(.maximized) > .vd-window-content",
			wants: []string{
				"border-bottom-left-radius: inherit;",
				"border-bottom-right-radius: inherit;",
				"overflow: hidden;",
			},
		},
		{
			name:     "titlebarless windows clip all content corners",
			selector: ".vd-window.no-titlebar:not(.maximized) > .vd-window-content",
			wants:    []string{"border-radius: inherit;"},
		},
	} {
		body := cssRuleBodyInFruityThemeTest(t, cssText, check.selector)
		for _, want := range check.wants {
			if !strings.Contains(body, want) {
				t.Fatalf("desktop %s rule %q missing %q in body %q", check.name, check.selector, want, body)
			}
		}
	}
}

func TestDesktopFruityWindowContentPreservesRoundedCorners(t *testing.T) {
	t.Parallel()

	cssText := readAllDesktopCSS(t)
	for _, check := range []struct {
		name      string
		selector  string
		wants     []string
		forbidden []string
	}{
		{
			name:     "window shell keeps external chrome visible",
			selector: ".desktop-body[data-theme=\"fruity\"] .vd-window",
			wants:    []string{"border-radius: 14px;"},
			forbidden: []string{
				"overflow: hidden;",
				"overflow: clip;",
			},
		},
		{
			name:     "titlebar follows top shell radius",
			selector: ".desktop-body[data-theme=\"fruity\"] .vd-window:not(.maximized) > .vd-window-titlebar",
			wants: []string{
				"border-top-left-radius: inherit;",
				"border-top-right-radius: inherit;",
			},
		},
		{
			name:     "content clips to bottom shell radius",
			selector: ".desktop-body[data-theme=\"fruity\"] .vd-window:not(.maximized) > .vd-window-content",
			wants: []string{
				"border-bottom-left-radius: inherit;",
				"border-bottom-right-radius: inherit;",
				"overflow: hidden;",
			},
		},
		{
			name:     "titlebarless windows clip all content corners",
			selector: ".desktop-body[data-theme=\"fruity\"] .vd-window.no-titlebar:not(.maximized) > .vd-window-content",
			wants:    []string{"border-radius: inherit;"},
		},
	} {
		body := cssRuleBodyInFruityThemeTest(t, cssText, check.selector)
		for _, want := range check.wants {
			if !strings.Contains(body, want) {
				t.Fatalf("fruity %s rule %q missing %q in body %q", check.name, check.selector, want, body)
			}
		}
		for _, bad := range check.forbidden {
			if strings.Contains(body, bad) {
				t.Fatalf("fruity %s rule %q must not contain %q in body %q", check.name, check.selector, bad, body)
			}
		}
	}
}

func cssRuleBodyInFruityThemeTest(t *testing.T, source, selector string) string {
	t.Helper()
	source = strings.ReplaceAll(source, "\r\n", "\n")
	start := strings.Index(source, selector)
	if start < 0 {
		t.Fatalf("missing CSS selector %q", selector)
	}
	open := strings.Index(source[start:], "{")
	if open < 0 {
		t.Fatalf("missing CSS block for selector %q", selector)
	}
	pos := start + open
	depth := 0
	for i := pos; i < len(source); i++ {
		switch source[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return source[pos : i+1]
			}
		}
	}
	t.Fatalf("missing closing brace for CSS selector %q", selector)
	return ""
}
