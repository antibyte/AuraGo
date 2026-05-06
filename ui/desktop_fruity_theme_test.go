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
	for _, want := range []string{
		"'appearance.theme': 'standard'",
		"body.dataset.theme = settingValue('appearance.theme')",
		"settingSelect('appearance.theme'",
		"desktop.settings_theme_standard",
		"desktop.settings_theme_fruity",
		"function isFruityTheme()",
		"function renderStandardTaskbar()",
		"function renderFruityDock()",
		"function scheduleFruityDockOcclusionCheck()",
		"function updateFruityDockOcclusion()",
		"function windowOverlapsFruityDock(",
		"allApps().map(app =>",
		"class=\"vd-dock-orb\"",
		"data-fruity-dock-orb",
		"fruity-dock-collapsed",
		"class=\"vd-dock-button",
		"data-app-id=\"${esc(app.id)}\"",
		"const runningWindows = [...state.windows.values()]",
		"runningWindows.some(win => win.appId === app.id)",
		"win.appId === app.id && win.id === state.activeWindowId",
	} {
		if !strings.Contains(shellText, want) {
			t.Fatalf("desktop shell is missing Fruity theme setting marker %q", want)
		}
	}

	css, err := Content.ReadFile("css/desktop.css")
	if err != nil {
		t.Fatalf("desktop stylesheet missing from embedded UI: %v", err)
	}
	cssText := string(css)
	for _, want := range []string{
		".desktop-body[data-theme=\"fruity\"]",
		"@media (prefers-color-scheme: dark)",
		".desktop-body[data-theme=\"fruity\"] .vd-window",
		".desktop-body[data-theme=\"fruity\"] .vd-window-titlebar",
		".desktop-body[data-theme=\"fruity\"] .vd-window-actions",
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
