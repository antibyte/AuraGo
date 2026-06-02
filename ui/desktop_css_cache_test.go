package ui

import (
	"regexp"
	"strings"
	"testing"
)

func TestDesktopCSSImportsBustComponentCache(t *testing.T) {
	t.Parallel()

	css := readDesktopAssetText(t, "css/desktop-shell.bundle.css")
	for _, marker := range []string{
		"/* ui/css/desktop-base.css */",
		"/* ui/css/desktop-taskbar.css */",
		"/* ui/css/desktop-start-menu.css */",
		"/* ui/css/desktop-windows.css */",
		"/* ui/css/desktop-icons.css */",
		"/* ui/css/desktop-widgets.css */",
		"/* ui/css/desktop-modals.css */",
		"/* ui/css/desktop-shell-overrides.css */",
	} {
		if !strings.Contains(css, marker) {
			t.Fatalf("desktop shell CSS bundle missing source marker %q", marker)
		}
	}
}

func TestDesktopHTMLBustsDesktopCSSAggregatorCache(t *testing.T) {
	t.Parallel()

	html := readDesktopAssetText(t, "desktop.html")
	if !strings.Contains(html, `/css/desktop-shell.bundle.css?v={{.BuildVersion}}`) {
		t.Fatalf("desktop.html must load the generated desktop shell CSS bundle with BuildVersion cache busting")
	}
}

func TestDesktopCSSCarriesFinalFruityWindowControlOverride(t *testing.T) {
	t.Parallel()

	css := readDesktopAssetText(t, "css/desktop-shell.bundle.css")
	for _, want := range []string{
		".desktop-body[data-theme=\"fruity\"] .vd-window > .vd-window-titlebar > .vd-window-actions",
		".desktop-body[data-theme=\"fruity\"] .vd-window.has-window-menu > .vd-window-titlebar",
		"grid-template-columns: 78px minmax(0, 1fr) 78px !important;",
		"padding: 0 14px !important;",
		".desktop-body[data-theme=\"fruity\"] .vd-window.has-window-menu > .vd-window-titlebar > .vd-window-title-group",
		"grid-column: 2 !important;",
		"justify-content: center !important;",
		".desktop-body[data-theme=\"fruity\"] .vd-window.has-window-menu > .vd-window-titlebar > .vd-window-actions",
		"left: 10px !important;",
		"right: 10px !important;",
		"justify-self: start !important;",
		"top: 24px !important;",
		"position: absolute !important;",
		"transform: translateY(-50%) !important;",
	} {
		if !strings.Contains(css, want) {
			t.Fatalf("desktop.css missing final Fruity window control override marker %q", want)
		}
	}
}

func TestDesktopCSSDefinesAccentRGBVariables(t *testing.T) {
	t.Parallel()

	css := readDesktopAssetText(t, "css/desktop-base.css")
	for _, marker := range []string{
		"--vd-accent-r: 39;",
		"--vd-accent-g: 199;",
		"--vd-accent-b: 166;",
		".desktop-body[data-accent=\"orange\"]",
		"--vd-accent-r: 255;",
		"--vd-accent-g: 157;",
		"--vd-accent-b: 77;",
		".desktop-body[data-accent=\"blue\"]",
		"--vd-accent-r: 99;",
		"--vd-accent-g: 179;",
		"--vd-accent-b: 255;",
		".desktop-body[data-accent=\"violet\"]",
		"--vd-accent-r: 183;",
		"--vd-accent-g: 148;",
		".desktop-body[data-accent=\"green\"]",
		"--vd-accent-r: 72;",
		"--vd-accent-g: 213;",
		"--vd-accent-b: 151;",
	} {
		if !strings.Contains(css, marker) {
			t.Fatalf("desktop accent CSS missing marker %q", marker)
		}
	}
}

func TestDesktopCSSDoesNotAppendAlphaToCSSVariables(t *testing.T) {
	t.Parallel()

	invalidVarAlpha := regexp.MustCompile(`var\([^;]+\)[0-9A-Fa-f]{2}\b`)
	for _, path := range []string{
		"css/desktop-base.css",
		"css/desktop-icons.css",
		"css/desktop-app-common.css",
		"css/desktop-app-file-manager.css",
		"css/desktop-app-office.css",
		"css/desktop-app-settings.css",
		"css/desktop-app-calculator.css",
		"css/desktop-app-planning.css",
		"css/desktop-app-chat.css",
		"css/desktop-app-quick-connect.css",
		"css/desktop-app-gallery.css",
		"css/desktop-app-launchpad.css",
		"css/desktop-app-system-info.css",
		"css/desktop-app-looper.css",
		"css/desktop-app-viewer.css",
		"css/desktop-app-software-store.css",
	} {
		css := readDesktopAssetText(t, path)
		if match := invalidVarAlpha.FindString(css); match != "" {
			t.Fatalf("%s contains invalid CSS variable alpha suffix %q", path, match)
		}
	}
}
