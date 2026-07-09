package ui

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestConfigPrecisionWorkspaceIsOptIn(t *testing.T) {
	t.Parallel()

	configHTML := normalizeAssetText(mustReadUIFile(t, "config.html"))
	for _, marker := range []string{
		`<body class="pw-page"`,
		`/css/precision-workspace.css`,
		`/css/config-workspace.css`,
		`/js/config/state.js`,
		`/js/config/actions.js`,
	} {
		if !strings.Contains(configHTML, marker) {
			t.Fatalf("config.html missing Precision Workspace marker %q", marker)
		}
	}

	for _, page := range []string{"index.html", "desktop.html"} {
		content := normalizeAssetText(mustReadUIFile(t, page))
		for _, forbidden := range []string{"precision-workspace", "config-workspace", "/js/config/state.js", "/js/config/actions.js"} {
			if strings.Contains(content, forbidden) {
				t.Fatalf("protected %s must not load %q", page, forbidden)
			}
		}
	}
}

func TestConfigPrecisionWorkspaceTypographyAndDensityContract(t *testing.T) {
	t.Parallel()

	foundation := normalizeAssetText(mustReadUIFile(t, "css/precision-workspace.css"))
	for _, marker := range []string{
		`.pw-page {`,
		`--pw-canvas: #0b0f0e;`,
		`--pw-surface: #121816;`,
		`--pw-accent: #2dd4bf;`,
		`--pw-control-size: 1rem;`,
		`--pw-label-size: 0.9375rem;`,
		`--pw-help-size: 0.875rem;`,
		`--pw-meta-size: 0.8125rem;`,
		`.pw-page[data-density="compact"]`,
		`cubic-bezier(0.32, 0.72, 0, 1)`,
		`@media (prefers-reduced-motion: reduce)`,
	} {
		if !strings.Contains(foundation, marker) {
			t.Fatalf("precision-workspace.css missing %q", marker)
		}
	}

	shell := normalizeAssetText(mustReadUIFile(t, "css/config-workspace.css"))
	for _, marker := range []string{
		`min-height: 100dvh;`,
		`width: 296px;`,
		`max-width: 1120px;`,
		`@media (max-width: 1099px)`,
		`min-height: 44px;`,
	} {
		if !strings.Contains(shell, marker) {
			t.Fatalf("config-workspace.css missing %q", marker)
		}
	}
}

func TestConfigStateAndActionContractsAreLoaded(t *testing.T) {
	t.Parallel()

	state := normalizeAssetText(mustReadUIFile(t, "js/config/state.js"))
	for _, marker := range []string{
		`window.AuraConfigState`,
		`init: init`,
		`beginSection: beginSection`,
		`dirtyPaths: dirtyPaths`,
		`buildPatch: buildPatch`,
		`validate: validate`,
		`commit: commit`,
		`discard: discard`,
		`bind: bind`,
		`subscribe: subscribe`,
	} {
		if !strings.Contains(state, marker) {
			t.Fatalf("config state contract missing %q", marker)
		}
	}

	actions := normalizeAssetText(mustReadUIFile(t, "js/config/actions.js"))
	for _, marker := range []string{
		`window.AuraConfigActions`,
		`register: register`,
		`refresh: refresh`,
		`run: run`,
		`requiresSaved`,
		`aria-disabled`,
		`aria-busy`,
	} {
		if !strings.Contains(actions, marker) {
			t.Fatalf("config action contract missing %q", marker)
		}
	}
}

func TestConfigMainUsesPrecisionStateForSaveAndDiscard(t *testing.T) {
	t.Parallel()

	mainJS := normalizeAssetText(mustReadUIFile(t, "js/config/main.js"))
	for _, marker := range []string{
		`window.AuraConfigState.init(configData)`,
		`window.AuraConfigState.beginSection(key)`,
		`window.AuraConfigState.buildPatch()`,
		`window.AuraConfigState.commit(configData)`,
		`window.AuraConfigState.discard()`,
		`config.unsaved_changes.save_and_continue`,
	} {
		if !strings.Contains(mainJS, marker) {
			t.Fatalf("config main.js missing state integration marker %q", marker)
		}
	}
}

func TestConfigStateBrowserSmoke(t *testing.T) {
	if os.Getenv("AURAGO_RUN_BROWSER_SMOKE") != "1" {
		t.Skip("set AURAGO_RUN_BROWSER_SMOKE=1 to run the headless browser smoke test")
	}

	browser := newSmokeBrowser(t)
	page := browser.MustPage("about:blank")
	page.MustSetViewport(1024, 768, 1, false)
	defer page.MustClose()
	page.MustSetDocumentContent(`<input id="port" data-path="server.port" type="number" value="8080">`)
	if err := page.AddScriptTag("", normalizeAssetText(mustReadUIFile(t, "js/config/state.js"))); err != nil {
		t.Fatalf("load config state: %v", err)
	}

	page.MustEval(`() => {
		window.AuraConfigState.init({server: {port: 8080}});
		window.AuraConfigState.bind(document);
		window.AuraConfigState.setRules({
			'server.port': {type: 'number', min: 1, max: 65535, required: true}
		});
		const input = document.getElementById('port');
		input.value = '9090';
		input.dispatchEvent(new Event('input', {bubbles: true}));
	}`)

	if got := page.MustEval(`() => window.AuraConfigState.get('server.port')`).Int(); got != 9090 {
		t.Fatalf("draft port = %d, want 9090", got)
	}
	if !page.MustEval(`() => window.AuraConfigState.isDirty()`).Bool() {
		t.Fatal("state should be dirty after bound input change")
	}
	if !page.MustEval(`() => window.AuraConfigState.validate().valid`).Bool() {
		t.Fatal("9090 should satisfy the configured port rule")
	}

	page.MustEval(`() => {
		const input = document.getElementById('port');
		input.value = '70000';
		input.dispatchEvent(new Event('input', {bubbles: true}));
	}`)
	if page.MustEval(`() => window.AuraConfigState.validate().valid`).Bool() {
		t.Fatal("70000 must fail the configured port rule")
	}

	page.MustEval(`() => window.AuraConfigState.discard()`)
	if got := page.MustElement("#port").MustProperty("value").String(); got != "8080" {
		t.Fatalf("discarded DOM value = %q, want 8080", got)
	}
}

func TestConfigPrecisionWorkspaceNavigationAndDensityMarkers(t *testing.T) {
	t.Parallel()

	html := normalizeAssetText(mustReadUIFile(t, "config.html"))
	for _, marker := range []string{
		`id="cfg-density-toggle"`,
		`data-i18n-title="config.precision.density_toggle"`,
		`aria-pressed="false"`,
	} {
		if !strings.Contains(html, marker) {
			t.Fatalf("config.html missing density control marker %q", marker)
		}
	}

	mainJS := normalizeAssetText(mustReadUIFile(t, "js/config/main.js"))
	for _, marker := range []string{
		`const CONFIG_DENSITY_KEY = 'aurago.config.density.v1'`,
		`const CONFIG_RECENT_KEY = 'aurago.config.recent.v1'`,
		`const CONFIG_ADVANCED_KEY = 'aurago.config.advanced.v1'`,
		`const CONFIG_RECENT_LIMIT = 6`,
		`function applyConfigDensity(`,
		`function renderConfigOverview(`,
		`function recordRecentSection(`,
		`function configSearchEntriesForSection(`,
		`function focusConfigField(`,
		`key === 'overview'`,
	} {
		if !strings.Contains(mainJS, marker) {
			t.Fatalf("config main.js missing Precision navigation marker %q", marker)
		}
	}
}

func TestConfigWorkspaceDoesNotRestoreTabletIconRail(t *testing.T) {
	t.Parallel()

	css := normalizeAssetText(mustReadUIFile(t, "css/config-workspace.css"))
	if strings.Contains(css, "width: 60px") {
		t.Fatal("Precision Workspace must use the labeled drawer instead of a 60px tablet icon rail")
	}
	for _, marker := range []string{
		`.pw-overview-grid`,
		`.pw-overview-card`,
		`.pw-density-toggle`,
		`.pw-field-focus`,
	} {
		if !strings.Contains(css, marker) {
			t.Fatalf("config-workspace.css missing overview/density marker %q", marker)
		}
	}
}

func TestConfigPrecisionActionGateCoversLegacyTestButtons(t *testing.T) {
	t.Parallel()

	html := normalizeAssetText(mustReadUIFile(t, "config.html"))
	if !strings.Contains(html, `/js/config/catalog.js`) {
		t.Fatal("config.html must load the versioned UI catalog before action gating")
	}

	catalog := normalizeAssetText(mustReadUIFile(t, "js/config/catalog.js"))
	for _, marker := range []string{`version: 1`, `actionRules`, `requiredPaths`, `credentialPaths`} {
		if !strings.Contains(catalog, marker) {
			t.Fatalf("config catalog missing %q", marker)
		}
	}

	actions := normalizeAssetText(mustReadUIFile(t, "js/config/actions.js"))
	for _, marker := range []string{
		`requiresSaved: true`,
		`MutationObserver`,
		`stopImmediatePropagation`,
		`autoEnhanceTestActions`,
		`cfg:section-rendered`,
	} {
		if !strings.Contains(actions, marker) {
			t.Fatalf("config action auto-gate missing %q", marker)
		}
	}
}

func TestConfigPrecisionTranslationsAreComplete(t *testing.T) {
	t.Parallel()

	locales := []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"}
	keys := []string{
		"config.common.clear",
		"config.unsaved_changes.save_and_continue",
		"config.precision.action_save_first",
		"config.precision.action_missing_fields",
		"config.precision.action_missing_credential",
		"config.precision.density_toggle",
		"config.precision.density_comfortable",
		"config.precision.density_compact",
		"config.precision.overview_title",
		"config.precision.overview_desc",
		"config.precision.overview_sections",
		"config.precision.workspace_label",
		"config.precision.recent_title",
		"config.precision.recent_empty",
		"config.precision.groups_title",
		"config.precision.restart_save_first",
		"config.precision.changed_fields",
		"config.precision.validation_ready",
		"config.precision.validation_valid",
		"config.precision.validation_invalid",
	}

	valuesByLocale := make(map[string]map[string]string, len(locales))
	for _, locale := range locales {
		var values map[string]string
		if err := json.Unmarshal(mustReadUIFile(t, "lang/config/common/"+locale+".json"), &values); err != nil {
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
		if valuesByLocale[locale]["config.precision.overview_desc"] == valuesByLocale["en"]["config.precision.overview_desc"] {
			t.Errorf("%s overview description must be translated, not copied from English", locale)
		}
	}
}

func TestConfigSaveDockAndRestartRespectDraftState(t *testing.T) {
	t.Parallel()

	html := normalizeAssetText(mustReadUIFile(t, "config.html"))
	for _, marker := range []string{`id="saveSection"`, `id="saveChangeCount"`, `id="saveValidation"`} {
		if !strings.Contains(html, marker) {
			t.Fatalf("config save dock missing %q", marker)
		}
	}

	mainJS := normalizeAssetText(mustReadUIFile(t, "js/config/main.js"))
	for _, marker := range []string{
		`window.AuraConfigState.dirtyPaths().length`,
		`config.precision.restart_save_first`,
		`restartBtn.setAttribute('aria-disabled'`,
		`if (hasUnsavedConfigChanges())`,
	} {
		if !strings.Contains(mainJS, marker) {
			t.Fatalf("config draft/restart contract missing %q", marker)
		}
	}
}
