package ui

import (
	"os"
	"strings"
	"testing"
)

func TestConfigUXSaveBarShellAndDirtyTracking(t *testing.T) {
	t.Parallel()

	html := normalizeAssetText(mustReadUIFile(t, "config.html"))
	for _, marker := range []string{
		`class="save-bar"`,
		`id="changesPill"`,
		`data-i18n="config.save_bar.unsaved_changes"`,
		`id="saveStatus"`,
		`class="save-status"`,
		`id="btnSave"`,
		`onclick="saveConfig()"`,
		`data-i18n="config.save_bar.save_button"`,
	} {
		if !strings.Contains(html, marker) {
			t.Fatalf("config.html missing save bar marker %q", marker)
		}
	}

	css := normalizeAssetText(mustReadUIFile(t, "css/config.css"))
	for _, marker := range []string{
		".save-bar {",
		"env(safe-area-inset-bottom",
		".changes-pill.visible",
		".save-status.success",
		".save-status.error",
		".save-status.warning",
		".btn-save:disabled",
	} {
		if !strings.Contains(css, marker) {
			t.Fatalf("config.css missing save bar marker %q", marker)
		}
	}

	mainJS := normalizeAssetText(mustReadUIFile(t, "js/config/main.js"))
	for _, marker := range []string{
		"function setDirty(",
		"function markDirty(",
		"function attachChangeListeners(",
		"async function saveConfig(",
		"config.save_bar.saving",
		"save-status warning",
		"changesPill",
		"configSaveInFlight",
	} {
		if !strings.Contains(mainJS, marker) {
			t.Fatalf("config main.js missing save UX marker %q", marker)
		}
	}
}

func TestConfigUXSidebarSearchWiring(t *testing.T) {
	t.Parallel()

	mainJS := normalizeAssetText(mustReadUIFile(t, "js/config/main.js"))
	for _, marker := range []string{
		"cfg-sidebar-search",
		"sidebarSearchInput",
		"sidebarSearchClear",
		"sidebarSearchNoResults",
		"function initSidebarSearch(",
		"function applySidebarSearch(",
		"function clearSidebarSearch(",
		"function handleSidebarSearchKeys(",
		"function highlightSidebarLabel(",
		"cfg-search-match",
		"config.sidebar.search_placeholder",
		"config.sidebar.no_results",
		"search-focused",
	} {
		if !strings.Contains(mainJS, marker) {
			t.Fatalf("config main.js missing sidebar search marker %q", marker)
		}
	}

	css := normalizeAssetText(mustReadUIFile(t, "css/config.css"))
	for _, marker := range []string{
		".cfg-sidebar-search {",
		".cfg-sidebar-search-input:focus",
		".cfg-sidebar-no-results",
		".sidebar-item.search-focused",
		".sidebar-item.hidden",
	} {
		if !strings.Contains(css, marker) {
			t.Fatalf("config.css missing sidebar search marker %q", marker)
		}
	}
}

func TestConfigUXSidebarSearchI18nInSectionLocales(t *testing.T) {
	t.Parallel()

	langs := []string{"en", "de", "fr", "es", "it", "ja", "zh", "nl", "pl", "cs", "da", "el", "hi", "no", "pt", "sv"}
	for _, lang := range langs {
		t.Run(lang, func(t *testing.T) {
			t.Parallel()
			data, err := os.ReadFile("lang/config/sections/" + lang + ".json")
			if err != nil {
				t.Fatalf("read sections %s: %v", lang, err)
			}
			content := string(data)
			for _, key := range []string{
				"config.sidebar.search_placeholder",
				"config.sidebar.no_results",
			} {
				if !strings.Contains(content, key) {
					t.Fatalf("sections/%s.json missing %q", lang, key)
				}
			}
		})
	}
}

func TestConfigUXToggleLabelsUseI18n(t *testing.T) {
	t.Parallel()

	mainJS := normalizeAssetText(mustReadUIFile(t, "js/config/main.js"))
	for _, marker := range []string{
		"function toggleBool(",
		"config.toggle.active",
		"config.toggle.inactive",
	} {
		if !strings.Contains(mainJS, marker) {
			t.Fatalf("config main.js missing toggle UX marker %q", marker)
		}
	}

	common := normalizeAssetText(mustReadUIFile(t, "lang/config/common/en.json"))
	for _, key := range []string{
		`"config.toggle.active"`,
		`"config.toggle.inactive"`,
	} {
		if !strings.Contains(common, key) {
			t.Fatalf("lang/config/common/en.json missing %s", key)
		}
	}
}

func TestConfigUXMobileSaveBarLayout(t *testing.T) {
	t.Parallel()

	css := normalizeAssetText(mustReadUIFile(t, "css/config.css"))
	if !strings.Contains(css, "@media (max-width: 480px)") {
		t.Fatal("config.css missing 480px breakpoint")
	}
	idx := strings.Index(css, "@media (max-width: 480px)")
	block := css[idx:]
	for _, marker := range []string{
		".save-bar {",
		"flex-direction: column",
		".btn-save {",
		"min-height: 44px",
	} {
		if !strings.Contains(block, marker) {
			t.Fatalf("480px save-bar block missing %q", marker)
		}
	}
}

func TestConfigUXGuardianPolicySelectPersistsBeforeRerender(t *testing.T) {
	t.Parallel()

	guardianJS := normalizeAssetText(mustReadUIFile(t, "cfg/guardian.js"))
	for _, marker := range []string{
		`data-path="guardian.promptsec.policy" onchange="guardianSetPolicy(this.value)"`,
		"function guardianSetPolicy(value) {",
		"setNestedValue(configData, 'guardian.promptsec.policy', value);",
		"renderGuardianSection(null);",
	} {
		if !strings.Contains(guardianJS, marker) {
			t.Fatalf("guardian.js missing policy select persistence marker %q", marker)
		}
	}
}
