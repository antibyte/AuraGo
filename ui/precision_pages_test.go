package ui

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"
)

func TestPrecisionWorkspaceFoundationComponentsAreScoped(t *testing.T) {
	t.Parallel()

	foundation := normalizeAssetText(mustReadUIFile(t, "css/precision-workspace.css"))
	for _, marker := range []string{
		`.pw-page {`,
		`font-family: 'Geist'`,
		`--pw-accent: #2dd4bf;`,
		`[data-theme="light"] .pw-page`,
		`.pw-page[data-density="compact"]`,
		`@media (prefers-reduced-motion: reduce)`,
	} {
		if !strings.Contains(foundation, marker) {
			t.Fatalf("precision-workspace.css missing foundation marker %q", marker)
		}
	}

	components := normalizeAssetText(mustReadUIFile(t, "css/precision-pages.css"))
	for _, marker := range []string{
		`.pw-page .pw-app-header`,
		`.pw-page .pw-page-frame`,
		`.pw-page .pw-page-heading`,
		`.pw-page .pw-toolbar`,
		`.pw-page .pw-tabs`,
		`.pw-page .pw-status-strip`,
		`.pw-page .pw-stat-strip`,
		`.pw-page .pw-panel`,
		`.pw-page .pw-card`,
		`.pw-page .pw-table`,
		`.pw-page .pw-form-control`,
		`.pw-page .pw-state-empty`,
		`.pw-page .pw-state-error`,
		`.pw-page .pw-state-loading`,
		`.pw-page .pw-modal`,
		`@media (min-width: 1200px)`,
		`@media (max-width: 899px)`,
		`@media (max-width: 639px)`,
	} {
		if !strings.Contains(components, marker) {
			t.Errorf("precision-pages.css missing component marker %q", marker)
		}
	}
	if strings.Contains(strings.ToLower(components), "gradient(") {
		t.Fatal("precision-pages.css must not introduce gradients")
	}
	assertPrecisionCSSScoped(t, foundation)
	assertPrecisionCSSScoped(t, components)
}

func TestPrecisionWorkspaceClientContract(t *testing.T) {
	t.Parallel()

	client := normalizeAssetText(mustReadUIFile(t, "js/precision/workspace.js"))
	for _, marker := range []string{
		`(function () {`,
		`window.AuraPrecisionWorkspace`,
		`init: init`,
		`getDensity: getDensity`,
		`setDensity: setDensity`,
		`aurago.workspace.density.v1`,
		`aurago.config.density.v1`,
		`comfortable`,
		`compact`,
		`localStorage.getItem`,
		`localStorage.setItem`,
		`try {`,
		`catch`,
		`document.body.dataset.density`,
		`[data-pw-density-toggle]`,
		`common.workspace_density_toggle`,
		`common.workspace_density_comfortable`,
		`common.workspace_density_compact`,
		`aurago:workspace-density-change`,
		`MutationObserver`,
		`radialMenuAnchor`,
		`aria-current`,
		`'/missions/v2': '/missions'`,
		`'/gallery': '/media'`,
		`<svg`,
	} {
		if !strings.Contains(client, marker) {
			t.Errorf("workspace client missing contract marker %q", marker)
		}
	}
}

func TestConfigPrecisionWorkspaceSharedIntegration(t *testing.T) {
	t.Parallel()

	html := normalizeAssetText(mustReadUIFile(t, "config.html"))
	for _, marker := range []string{
		`<body class="pw-page" data-workspace-page="config"`,
		`data-pw-density-toggle`,
		`common.workspace_density_toggle`,
		`common.workspace_density_comfortable`,
	} {
		if !strings.Contains(html, marker) {
			t.Errorf("config.html missing shared workspace marker %q", marker)
		}
	}

	foundationAt := strings.Index(html, `/css/precision-workspace.css?v={{.BuildVersion}}`)
	componentsAt := strings.Index(html, `/css/precision-pages.css?v={{.BuildVersion}}`)
	configAt := strings.Index(html, `/css/config-workspace.css?v={{.BuildVersion}}`)
	if foundationAt < 0 || componentsAt < 0 || configAt < 0 || !(foundationAt < componentsAt && componentsAt < configAt) {
		t.Errorf("Config Precision CSS order = foundation:%d components:%d config:%d", foundationAt, componentsAt, configAt)
	}

	workspaceAt := strings.Index(html, `/js/precision/workspace.js?v={{.BuildVersion}}`)
	mainAt := strings.Index(html, `/js/config/main.js?v={{.BuildVersion}}`)
	if workspaceAt < 0 || mainAt < 0 || workspaceAt >= mainAt {
		t.Errorf("Config script order = workspace:%d main:%d", workspaceAt, mainAt)
	}

	mainJS := normalizeAssetText(mustReadUIFile(t, "js/config/main.js"))
	start := strings.Index(mainJS, "function applyConfigDensity(")
	end := strings.Index(mainJS, "function hasVisibleSection(")
	if start < 0 || end <= start {
		t.Fatal("cannot locate the applyConfigDensity integration block")
	}
	densityBlock := mainJS[start:end]
	for _, marker := range []string{`window.AuraPrecisionWorkspace`, `.init()`, `.getDensity()`, `.setDensity(`} {
		if !strings.Contains(densityBlock, marker) {
			t.Errorf("applyConfigDensity block missing workspace delegation marker %q", marker)
		}
	}
	if strings.Contains(densityBlock, "localStorage") {
		t.Fatal("Config density integration must delegate storage ownership to AuraPrecisionWorkspace")
	}
}

func TestPrecisionWorkspaceTranslations(t *testing.T) {
	t.Parallel()

	locales := []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"}
	keys := []string{
		"common.workspace_density_toggle",
		"common.workspace_density_comfortable",
		"common.workspace_density_compact",
	}
	valuesByLocale := make(map[string]map[string]string, len(locales))
	for _, locale := range locales {
		var values map[string]string
		if err := json.Unmarshal(mustReadUIFile(t, "lang/common/"+locale+".json"), &values); err != nil {
			t.Fatalf("parse %s common translations: %v", locale, err)
		}
		valuesByLocale[locale] = values
		for _, key := range keys {
			if strings.TrimSpace(values[key]) == "" {
				t.Errorf("%s is missing non-empty translation %q", locale, key)
			}
		}
	}
	for _, locale := range locales {
		if locale == "en" {
			continue
		}
		if valuesByLocale[locale][keys[0]] == valuesByLocale["en"][keys[0]] {
			t.Errorf("%s workspace density toggle must be translated, not copied from English", locale)
		}
	}
}

func TestPrecisionWorkspaceProtectedPagesStayOptedOut(t *testing.T) {
	t.Parallel()

	for _, page := range []string{"index.html", "desktop.html", "gallery.html"} {
		content := normalizeAssetText(mustReadUIFile(t, page))
		for _, forbidden := range []string{
			`precision-workspace.css`,
			`precision-pages.css`,
			`js/precision/workspace.js`,
			`data-workspace-page=`,
		} {
			if strings.Contains(content, forbidden) {
				t.Errorf("protected %s must not load or opt into %q", page, forbidden)
			}
		}
	}
}

func assertPrecisionCSSScoped(t *testing.T, css string) {
	t.Helper()

	comments := regexp.MustCompile(`(?s)/\*.*?\*/`)
	css = comments.ReplaceAllString(css, "")
	segmentStart := 0
	for index, char := range css {
		switch char {
		case '{':
			header := strings.TrimSpace(css[segmentStart:index])
			segmentStart = index + 1
			if header == "" || strings.HasPrefix(header, "@") {
				continue
			}
			for _, selector := range strings.Split(header, ",") {
				selector = strings.TrimSpace(selector)
				if selector != "" && !strings.Contains(selector, ".pw-page") {
					t.Errorf("unscoped Precision selector %q", selector)
				}
			}
		case '}':
			segmentStart = index + 1
		}
	}
}
