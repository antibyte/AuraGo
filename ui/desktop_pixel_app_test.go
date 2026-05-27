package ui

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"
)

func TestPixelInspectorTabsRespectHiddenPanels(t *testing.T) {
	css := normalizePixelAsset(readDesktopAssetText(t, "css/pixel.css"))

	if !pixelCSSRuleContains(css, `.pixel-panel-section[hidden]`, "display: none") {
		t.Fatalf("pixel panel sections use flex display, so [hidden] needs an explicit display:none override")
	}
}

func TestPixelInspectorLayoutFitsLocalizedControls(t *testing.T) {
	css := normalizePixelAsset(readDesktopAssetText(t, "css/pixel.css"))

	panelChecks := []string{
		"clamp(320px",
		"overflow-x: hidden",
		"box-sizing: border-box",
	}
	for _, want := range panelChecks {
		if !pixelCSSRuleContains(css, `.pixel-panel`, want) {
			t.Fatalf("pixel side panel rule should contain %q so localized controls fit", want)
		}
	}

	if !pixelCSSRuleContains(css, `.pixel-btn-group`, "grid-template-columns: repeat(2, minmax(0, 1fr))") {
		t.Fatalf("pixel button groups should use bounded grid columns instead of overflowing rows")
	}
	for _, want := range []string{"white-space: normal", "overflow-wrap: anywhere"} {
		if !pixelCSSRuleContains(css, `.pixel-btn-group .pixel-btn`, want) {
			t.Fatalf("pixel grouped buttons should contain %q to keep long labels inside the side panel", want)
		}
	}
	for _, want := range []string{"max-width: 100%", "text-overflow: ellipsis"} {
		if !pixelCSSRuleContains(css, `.pixel-filter-name`, want) {
			t.Fatalf("pixel filter labels should contain %q to avoid panel overflow", want)
		}
	}
}

func TestPixelImageMenuUsesThemeIcons(t *testing.T) {
	js := readDesktopAssetText(t, "js/desktop/apps/pixel.js")

	for _, unavailable := range []string{
		"icon: 'rotate-cw'",
		"icon: 'rotate-ccw'",
		"icon: 'crop'",
		"icon: 'resize'",
		"iconMarkup('rotate-cw'",
		"iconMarkup('rotate-ccw'",
		"iconMarkup('crop'",
		"iconMarkup('resize'",
	} {
		if strings.Contains(js, unavailable) {
			t.Fatalf("pixel image controls still reference unavailable theme icon %s", unavailable)
		}
	}

	expectedMenuIcons := map[string]string{
		"rotate-cw":  "redo",
		"rotate-ccw": "undo",
		"flip-h":     "sort",
		"flip-v":     "sort",
		"crop":       "scissors",
		"resize":     "maximize",
	}
	for id, icon := range expectedMenuIcons {
		pattern := regexp.MustCompile(`\{\s*id:\s*'` + regexp.QuoteMeta(id) + `',\s*labelKey:\s*'[^']+',\s*icon:\s*'` + regexp.QuoteMeta(icon) + `'`)
		if !pattern.MatchString(js) {
			t.Fatalf("pixel image menu item %q should use available icon %q", id, icon)
		}
	}

	for _, manifestPath := range []string{
		"img/papirus/manifest.json",
		"img/whitesur/manifest.json",
	} {
		icons := readThemeIconNames(t, manifestPath)
		for _, icon := range []string{"redo", "undo", "sort", "scissors", "maximize"} {
			if !icons[icon] {
				t.Fatalf("%s does not provide icon %q required by the Pixel image menu", manifestPath, icon)
			}
		}
	}
}

func normalizePixelAsset(text string) string {
	return strings.ReplaceAll(text, "\r\n", "\n")
}

func pixelCSSRuleContains(css, selector, needle string) bool {
	re := regexp.MustCompile(regexp.QuoteMeta(selector) + `\s*\{[^}]*` + regexp.QuoteMeta(needle))
	return re.MatchString(css)
}

func readThemeIconNames(t *testing.T, manifestPath string) map[string]bool {
	t.Helper()
	var manifest struct {
		Icons map[string]json.RawMessage `json:"icons"`
	}
	if err := json.Unmarshal([]byte(readDesktopAssetText(t, manifestPath)), &manifest); err != nil {
		t.Fatalf("parse %s: %v", manifestPath, err)
	}
	icons := make(map[string]bool, len(manifest.Icons))
	for name := range manifest.Icons {
		icons[name] = true
	}
	return icons
}
