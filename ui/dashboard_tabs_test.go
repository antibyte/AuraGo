package ui

import (
	"regexp"
	"strings"
	"testing"
)

func TestDashboardTabButtonsHaveMatchingPanels(t *testing.T) {
	t.Parallel()

	html := readDesktopAssetText(t, "dashboard.html")
	tabRe := regexp.MustCompile(`data-tab="([^"]+)"`)
	matches := tabRe.FindAllStringSubmatch(html, -1)
	seen := map[string]bool{}
	for _, match := range matches {
		tab := match[1]
		if seen[tab] {
			continue
		}
		seen[tab] = true
		panelID := `id="tab-` + tab + `"`
		if !strings.Contains(html, panelID) {
			t.Fatalf("dashboard tab %q has no matching panel marker %q", tab, panelID)
		}
	}

	if len(seen) == 0 {
		t.Fatal("dashboard must define at least one tab button")
	}
	if strings.Contains(html, `class="dash-tab-panel" id="main-content"`) {
		t.Fatal("overview panel must use id=\"tab-overview\" so showTab can reveal it")
	}
	if !strings.Contains(html, `href="#tab-overview" class="skip-link"`) {
		t.Fatal("dashboard skip link should target the overview tab panel")
	}
}

func TestDashboardIconSpriteLoadsAfterBodyExists(t *testing.T) {
	t.Parallel()

	html := readDesktopAssetText(t, "dashboard.html")
	dashIconsRe := regexp.MustCompile(`<script[^>]*dash-icons\.js[^>]*></script>`)
	tag := dashIconsRe.FindString(html)
	if tag == "" {
		t.Fatal("dashboard must load the dash-icons.js sprite helper")
	}
	if !strings.Contains(tag, " defer") {
		t.Fatal("dashboard icon sprite script must load with defer so document.body exists before sprite injection")
	}
}
