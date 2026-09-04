package ui

import (
	"strings"
	"testing"
)

func TestDesktopThemesShareShellAndAppMaterials(t *testing.T) {
	t.Parallel()

	css := readAllDesktopCSS(t)
	for _, marker := range []string{
		"--vd-theme-app-bg:",
		"--vd-theme-panel-bg:",
		"--vd-theme-panel-bg-strong:",
		"--vd-theme-chrome-bg:",
		"--vd-theme-overlay-bg:",
		"--vd-theme-control-bg:",
		"--vd-theme-control-hover:",
		"--vd-theme-border:",
		"--vd-theme-radius-modal:",
		"--vd-theme-shadow-modal:",
		"--vd-theme-blur:",
		".desktop-body[data-theme=\"standard\"] :where(",
		".desktop-body[data-theme=\"fruity\"] :where(",
		".vd-file-dialog",
		".vd-shortcuts-modal",
		".vd-toast",
		".vd-window-content",
		".fm-modal",
		".fm-context-menu",
		".cs-modal",
		".cs-context-menu",
		".vd-qc-modal",
		".vd-store-modal",
		".cheater-modal-panel",
		".radio-toast",
		".teevee-toast",
		"--radio-glass: var(--vd-theme-panel-bg) !important;",
		"--cam-glass: var(--vd-theme-panel-bg) !important;",
		"--cs-panel: var(--vd-theme-panel-bg) !important;",
		"Chrome polish: physical edges, glass refraction, and theme-native chrome",
		"--vd-chrome-highlight:",
		"--vd-chrome-ring:",
		".vd-window::after",
		".vd-taskbar-apps::after",
		"Surface polish: start menu content, wallpaper labels, and app interiors",
		"App interior polish: toolbars, sidebars, lists, and settings surfaces",
		".vd-start-search input",
		".vd-icon-label",
		"Detail polish: micro-chrome, menus, scrollbars, and tray type",
	} {
		if !strings.Contains(css, marker) {
			t.Fatalf("desktop theme consistency CSS is missing marker %q", marker)
		}
	}
}

func TestDesktopEverydayAppsUseThemeBridge(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		file   string
		marker string
	}{
		{"css/desktop-app-office.css", ".office-writer"},
		{"css/desktop-app-office.css", "--vd-theme-app-bg"},
		{"css/desktop-app-planning.css", ".vd-todo-card"},
		{"css/desktop-app-planning.css", "--vd-theme-panel-bg"},
		{"css/desktop-app-settings.css", ".vd-settings-app"},
		{"css/desktop-app-settings.css", "var(--vd-theme-app-bg)"},
		{"css/desktop-app-calculator.css", ".vd-calc"},
		{"css/desktop-app-calculator.css", "var(--vd-theme-app-bg)"},
		{"css/desktop-app-calculator.css", ".vd-calc-prog-display"},
		{"css/desktop-app-calculator.css", "background: var(--vd-theme-panel-bg)"},
		{"css/desktop-app-common.css", ".vd-calc-prog-display"},
		{"css/desktop-app-chat.css", ".vd-chat"},
		{"css/desktop-app-chat.css", "var(--vd-theme-app-bg)"},
		{"css/desktop-app-file-manager.css", ".file-manager"},
		{"css/desktop-app-file-manager.css", "var(--vd-theme-app-bg)"},
		{"css/desktop-app-quick-connect.css", ".vd-quick-connect"},
		{"css/desktop-app-quick-connect.css", "var(--vd-theme-app-bg)"},
		{"css/desktop-app-mission-control.css", ".vd-mc"},
		{"css/desktop-app-mission-control.css", "var(--vd-theme-app-bg)"},
		{"css/desktop-app-software-store.css", ".vd-store"},
		{"css/desktop-app-software-store.css", "var(--vd-theme-panel-bg)"},
		{"css/desktop-app-gallery.css", ".vd-gallery-card"},
		{"css/desktop-app-gallery.css", "var(--vd-theme-panel-bg)"},
		{"css/desktop-chrome.css", ".vd-terminal-app"},
		{"css/desktop-chrome.css", "var(--vd-theme-chrome-bg)"},
		{"css/desktop-app-viewer.css", ".vd-viewer"},
		{"css/desktop-app-viewer.css", "var(--vd-theme-app-bg)"},
		{"css/desktop-app-looper.css", ".vd-looper"},
		{"css/desktop-app-looper.css", "var(--vd-theme-app-bg)"},
		{"css/desktop-app-cheater.css", ".cheater-app"},
		{"css/desktop-app-cheater.css", "--cheater-surface: var(--vd-theme-app-bg)"},
		{"css/desktop-app-people.css", ".vd-people"},
		{"css/desktop-app-people.css", "var(--vd-theme-app-bg)"},
		{"css/desktop-app-launchpad.css", ".vd-launchpad"},
		{"css/desktop-app-launchpad.css", "var(--vd-theme-panel-bg)"},
		{"css/zipper.css", ".zipper-app"},
		{"css/zipper.css", "--zipper-line: var(--vd-theme-border)"},
		{"css/pixel.css", ".pixel-app"},
		{"css/pixel.css", "--pixel-line: var(--vd-theme-border)"},
		{"css/desktop-app-log-viewer.css", ".vd-logviewer"},
		{"css/desktop-app-log-viewer.css", "var(--vd-theme-chrome-bg)"},
		{"css/desktop-app-system-info.css", ".vd-sysinfo"},
		{"css/desktop-app-system-info.css", "var(--vd-theme-panel-bg)"},
		{"css/desktop-app-pet-picker.css", ".vd-pet-picker"},
		{"css/desktop-app-pet-picker.css", "var(--vd-theme-panel-bg)"},
		{"css/radio.css", ".radio-app"},
		{"css/radio.css", "--radio-glass: var(--vd-theme-panel-bg)"},
		{"css/camera.css", ".camera-app"},
		{"css/camera.css", "--cam-glass: var(--vd-theme-panel-bg)"},
		{"css/camera.css", "background: #000"},
		{"css/code-studio.css", ".code-studio"},
		{"css/code-studio.css", "--cs-bg: var(--vd-theme-app-bg)"},
		{"css/desktop-app-network-cameras.css", ".network-cameras-app"},
		{"css/desktop-app-network-cameras.css", "--nc-bg: var(--vd-theme-app-bg)"},
		{"css/desktop-app-network-cameras.css", "background: #05080d"},
		{"css/desktop-app-noisemaker.css", ".noisemaker-app"},
		{"css/desktop-app-noisemaker.css", "--nm-bg: var(--vd-theme-app-bg)"},
		{"css/desktop-app-live-speech.css", ".vd-live-speech-app"},
		{"css/desktop-app-live-speech.css", "var(--vd-theme-app-bg)"},
		{"css/desktop-app-common.css", ".nc-toolbar"},
		{"css/desktop-app-common.css", ".nm-header"},
		{"css/desktop-app-common.css", ".vd-live-speech-lab"},
		{"css/desktop-app-homepage-studio.css", ".vd-hp-studio"},
		{"css/desktop-app-homepage-studio.css", "--hp-bg: var(--vd-theme-app-bg)"},
		{"css/desktop-app-homepage-studio.css", "background: #071018"},
		{"css/desktop-app-game-maker-studio.css", ".gm-studio"},
		{"css/desktop-app-game-maker-studio.css", "--gm-bg: var(--vd-theme-app-bg)"},
		{"css/desktop-app-game-maker-studio.css", "background: #050509"},
		{"css/desktop-app-openscad.css", ".openscad-app"},
		{"css/desktop-app-openscad.css", "--oscad-bg: var(--vd-theme-app-bg)"},
		{"css/desktop-app-openscad.css", "background: #050a10"},
		{"css/desktop-app-planning.css", ".vd-webamp-launcher"},
		{"css/desktop-app-planning.css", "background: var(--vd-theme-app-bg)"},
		{"css/desktop-app-common.css", ".vd-hp-header"},
		{"css/desktop-app-common.css", ".gm-topbar"},
		{"css/desktop-app-common.css", ".oscad-header"},
		{"css/desktop-app-common.css", ".vd-webamp-status"},
		{"css/teevee.css", ".teevee-app"},
		{"css/teevee.css", "--teevee-bg: var(--vd-theme-app-bg)"},
		{"css/teevee.css", "background: #02050a"},
		{"css/desktop-app-chess.css", ".vd-chess"},
		{"css/desktop-app-chess.css", "--chess-panel: var(--vd-theme-panel-bg)"},
		{"css/desktop-app-chess.css", "--chess-felt: #d7e5dc"},
		{"css/desktop-app-nasscad.css", ".vd-nasscad"},
		{"css/desktop-app-nasscad.css", "var(--vd-theme-app-bg)"},
		{"css/desktop-app-nasscad.css", "background: #111318"},
		{"css/desktop-app-sysworld.css", "--sw-panel: var(--vd-theme-panel-bg)"},
		{"css/desktop-app-sysworld.css", "background: #020208"},
		{"css/desktop-app-common.css", ".sysworld"},
		{"css/desktop-app-common.css", "--sw-panel: var(--vd-theme-panel-bg)"},
		{"css/desktop-app-common.css", ".sw-stats"},
		{"css/desktop-app-common.css", ".teevee-sidebar"},
		{"css/desktop-app-common.css", ".vd-chess-controls"},
		{"css/desktop-app-common.css", ".vd-people-toolbar"},
		{"css/desktop-app-common.css", ".vd-launchpad-tile"},
		{"css/desktop-app-common.css", ".zipper-toolbar"},
		{"css/desktop-app-common.css", ".pixel-toolbar"},
		{"css/desktop-app-common.css", ".vd-logviewer-toolbar"},
		{"css/desktop-app-common.css", ".vd-sysinfo-hero"},
		{"css/desktop-app-common.css", ".vd-pet-picker-card"},
		{"css/pixel.css", "background: #0e1117"},
	} {
		css := readDesktopAssetText(t, tc.file)
		if !strings.Contains(css, tc.marker) {
			t.Fatalf("%s missing theme bridge marker %q", tc.file, tc.marker)
		}
	}
}

func TestDesktopThemeBundleContainsConsistencyBridge(t *testing.T) {
	t.Parallel()

	bundle := readDesktopAssetText(t, "css/desktop-shell.bundle.css")
	for _, marker := range []string{
		"Theme material bridge: keep shell, apps, modals and toasts visually aligned",
		"--vd-theme-panel-bg-strong:",
		".desktop-body[data-theme=\"standard\"] :where(",
		".desktop-body[data-theme=\"fruity\"] :where(",
		".vd-modal-backdrop",
		".vd-file-dialog-backdrop",
		".vd-toast",
		"Chrome polish: physical edges, glass refraction, and theme-native chrome",
		"--vd-chrome-highlight:",
		"Surface polish: start menu content, wallpaper labels, and app interiors",
		"Detail polish: micro-chrome, menus, scrollbars, and tray type",
	} {
		if !strings.Contains(bundle, marker) {
			t.Fatalf("desktop shell bundle is missing theme consistency marker %q", marker)
		}
	}
}
