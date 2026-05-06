package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFruityDockShowsScrollButtonsWhenAppsOverflow(t *testing.T) {
	mainText := readDesktopAssetText(t, "js/desktop/main.js")
	for _, want := range []string{
		"function wireFruityDockScroll(",
		"function updateFruityDockScrollControls(",
		`data-fruity-dock-scroll-region`,
		`data-fruity-dock-track`,
		`data-fruity-dock-scroll-button="left"`,
		`data-fruity-dock-scroll-button="right"`,
		"vd-dock-overflowing",
		"scroller.scrollBy({",
		"new ResizeObserver",
	} {
		if !strings.Contains(mainText, want) {
			t.Fatalf("fruity dock overflow implementation missing marker %q", want)
		}
	}

	css := readDesktopAssetText(t, "css/desktop.css")
	for _, want := range []string{
		".desktop-body[data-theme=\"fruity\"] .vd-dock-scroll",
		".desktop-body[data-theme=\"fruity\"] .vd-dock-track",
		".desktop-body[data-theme=\"fruity\"] .vd-dock-scroll-button",
		".desktop-body[data-theme=\"fruity\"] .vd-taskbar-apps.vd-dock-overflowing .vd-dock-scroll-button",
		"max-width: calc(100vw - 170px);",
	} {
		if !strings.Contains(css, want) {
			t.Fatalf("fruity dock overflow CSS missing marker %q", want)
		}
	}
	if strings.Contains(css, "max-width: min(920px, calc(100vw - 170px));") {
		t.Fatalf("fruity dock still caps width at 920px before enabling scroll controls")
	}
}

func TestFruityDockScrollLabelsAreTranslated(t *testing.T) {
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
		for _, key := range []string{"desktop.dock_scroll_left", "desktop.dock_scroll_right"} {
			if strings.TrimSpace(values[key]) == "" {
				t.Fatalf("%s missing non-empty translation for %s", path, key)
			}
		}
	}
}
