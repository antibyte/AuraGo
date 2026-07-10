package ui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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
		`markSaved: markSaved`,
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

func TestConfigPrecisionWorkspaceBrowserMatrix(t *testing.T) {
	if os.Getenv("AURAGO_RUN_BROWSER_SMOKE") != "1" {
		t.Skip("set AURAGO_RUN_BROWSER_SMOKE=1 to run the headless browser smoke test")
	}

	translations := map[string]string{}
	for _, file := range []string{"lang/config/en.json", "lang/config/common/en.json", "lang/config/sections/en.json"} {
		var bundle map[string]string
		if err := json.Unmarshal(mustReadUIFile(t, file), &bundle); err != nil {
			t.Fatalf("parse English config translations from %s: %v", file, err)
		}
		for key, value := range bundle {
			translations[key] = value
		}
	}
	i18n, err := json.Marshal(translations)
	if err != nil {
		t.Fatalf("marshal config translations: %v", err)
	}
	css := strings.Join([]string{
		normalizeAssetText(mustReadUIFile(t, "css/config.css")),
		normalizeAssetText(mustReadUIFile(t, "css/precision-workspace.css")),
		normalizeAssetText(mustReadUIFile(t, "css/precision-pages.css")),
		normalizeAssetText(mustReadUIFile(t, "css/config-workspace.css")),
	}, "\n")
	html := fmt.Sprintf(`<!doctype html><html lang="en" data-theme="dark"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><style>%s</style></head>
	<body class="pw-page" data-density="comfortable">
	<div class="cfg-header"><div class="cfg-logo-wrap"><button id="cfg-hamburger" class="hamburger-btn cfg-hamburger">☰</button><a class="logo"><div class="logo-icon">⚡</div><span class="logo-wordmark-accent">AURA</span><span class="logo-wordmark-base">GO</span><span class="logo-subtitle">Configuration</span></a></div><div class="header-actions"><button id="cfg-density-toggle" class="pw-density-toggle" aria-pressed="false" data-pw-density-toggle><svg viewBox="0 0 24 24"><path d="M5 7h14M5 12h14M5 17h14"/></svg><span data-pw-density-label>Comfortable</span></button><button id="cfg-restart-btn" class="btn-header cfg-restart-btn">Restart</button></div></div>
	<div class="cfg-layout" id="main-content"><div id="sidebar-backdrop" class="sidebar-backdrop"></div><div class="cfg-sidebar" id="sidebar"></div><main class="cfg-content" id="content"></main></div>
	<div class="save-bar"><div class="pw-save-context"><strong id="saveSection"></strong><span id="saveChangeCount"></span><span id="saveValidation"></span></div><span id="changesPill" class="changes-pill">Unsaved</span><span id="saveStatus"></span><button id="btnSave" class="btn-save" disabled>Save</button></div>
	<script>window.I18N=%s;window.I18N_META={};window.SYSTEM_LANG='en';window.AURAGO_BUILD_VERSION='test';window.t=(key)=>window.I18N[key]||key;
	window.fetch=async(input)=>{const url=String(input);const payload=url.includes('/api/config/schema')?[{key:'server',yaml_key:'server',type:'object',children:[{key:'server.port',yaml_key:'port',type:'int'},{key:'server.debug_mode',yaml_key:'debug_mode',type:'bool'}]}]:url.includes('/api/config')?{server:{port:8080,debug_mode:false}}:url.includes('/api/vault/status')?{exists:true}:url.includes('/api/providers')?[]:url.includes('/api/personalities')?{personalities:[]}:url.includes('/api/runtime')?{runtime:{},features:{}}:{};return{ok:true,status:200,redirected:false,url,headers:{get:()=> 'application/json'},json:async()=>structuredClone(payload),text:async()=>JSON.stringify(payload)};};</script>
	</body></html>`, css, i18n)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("<!doctype html><title>fixture</title>"))
	}))
	defer server.Close()
	browser := newSmokeBrowser(t)
	page := browser.MustPage(server.URL)
	defer page.MustClose()
	page.MustSetDocumentContent(html)
	for _, script := range []string{"js/config/catalog.js", "js/config/state.js", "js/config/actions.js", "cfg/form-builder.js", "js/precision/workspace.js", "js/config/main.js"} {
		if err := page.AddScriptTag("", normalizeAssetText(mustReadUIFile(t, script))); err != nil {
			t.Fatalf("load %s: %v", script, err)
		}
	}
	waitForJSBool(t, page, `() => document.querySelectorAll('.pw-overview-card').length >= 10`)

	viewports := []struct{ width, height int }{{1920, 1080}, {1440, 900}, {1024, 768}, {768, 1024}, {390, 844}}
	for _, theme := range []string{"dark", "light", "system"} {
		for _, viewport := range viewports {
			page.MustSetViewport(viewport.width, viewport.height, 1, viewport.width <= 390)
			page.MustEval(`theme => { document.documentElement.dataset.theme = theme === 'system' ? 'dark' : theme; document.body.dataset.theme = theme === 'system' ? 'dark' : theme; }`, theme)
			layout := page.MustEval(`() => ({overflow: document.documentElement.scrollWidth > window.innerWidth + 1, font: parseFloat(getComputedStyle(document.querySelector('.sidebar-item')).fontSize)})`).Map()
			if layout["overflow"].Bool() {
				t.Fatalf("horizontal overflow at %s %dx%d", theme, viewport.width, viewport.height)
			}
			if layout["font"].Num() < 13 {
				t.Fatalf("sidebar font too small at %s %dx%d: %v", theme, viewport.width, viewport.height, layout["font"])
			}
			screenshot := page.MustScreenshot()
			if artifactDir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); artifactDir != "" {
				if err := os.MkdirAll(artifactDir, 0o755); err != nil {
					t.Fatalf("create browser artifact directory: %v", err)
				}
				name := fmt.Sprintf("config-%s-%dx%d.png", theme, viewport.width, viewport.height)
				if err := os.WriteFile(filepath.Join(artifactDir, name), screenshot, 0o644); err != nil {
					t.Fatalf("write browser screenshot: %v", err)
				}
			}
		}
	}

	page.MustSetViewport(1440, 900, 1, false)
	page.MustElement("#cfg-density-toggle").MustClick()
	if got := page.MustEval(`() => document.body.dataset.density`).String(); got != "compact" {
		t.Fatalf("density = %q, want compact", got)
	}
	page.MustElement("#sidebarSearchInput").MustInput("port")
	waitForJSBool(t, page, `() => document.querySelector('[data-section="server"]').dataset.searchTarget === 'server.port'`)
	page.MustEval(`() => document.querySelector('[data-section="server"]').click()`)
	waitForJSBool(t, page, `() => location.hash === '#server' && !!document.querySelector('[data-path="server.port"]')`)
	if !page.MustEval(`() => !!document.querySelector('.pw-advanced [data-path="server.debug_mode"]')`).Bool() {
		t.Fatal("advanced server field was not moved into the disclosure")
	}
	page.MustEval(`() => { const input=document.querySelector('[data-path="server.port"]'); input.value='9090'; input.dispatchEvent(new Event('input',{bubbles:true})); }`)
	waitForJSBool(t, page, `() => window.AuraConfigState.isDirty() && document.getElementById('cfg-restart-btn').getAttribute('aria-disabled') === 'true'`)
	page.MustEval(`() => { window.__testRan=false; const button=document.createElement('button'); button.id='server-test-btn'; button.textContent='Test'; button.addEventListener('click',()=>window.__testRan=true); document.getElementById('content').appendChild(button); }`)
	waitForJSBool(t, page, `() => document.getElementById('server-test-btn').getAttribute('aria-disabled') === 'true'`)
	page.MustElement("#server-test-btn").MustClick()
	if page.MustEval(`() => window.__testRan`).Bool() {
		t.Fatal("dirty test action was allowed to run")
	}
	page.MustElement(".pw-overview-nav").MustClick()
	waitForJSBool(t, page, `() => !!document.getElementById('cfg-unsaved-decision')`)
	page.MustEval(`() => document.querySelector('[data-decision="discard"]').click()`)
	waitForJSBool(t, page, `() => location.hash === '#overview' && !window.AuraConfigState.isDirty()`)
	page.MustEval(`() => {
		window.__modalTestRan=false;
		const modal=document.createElement('div');
		modal.className='sql-modal-overlay is-hidden';
		modal.innerHTML='<input id="sqlconn-field-name"><input id="sqlconn-field-database"><button id="sqlconn-test-btn">Test</button>';
		document.body.appendChild(modal);
		document.getElementById('sqlconn-test-btn').addEventListener('click',()=>window.__modalTestRan=true);
		document.getElementById('sqlconn-field-name').value='Saved connection';
		document.getElementById('sqlconn-field-database').value='aurago';
		modal.classList.remove('is-hidden');
	}`)
	waitForJSBool(t, page, `() => document.getElementById('sqlconn-test-btn').getAttribute('aria-disabled') === 'false'`)
	page.MustEval(`() => { document.getElementById('sqlconn-field-name').value='Unsaved rename'; document.getElementById('sqlconn-test-btn').click(); }`)
	if page.MustEval(`() => window.__modalTestRan`).Bool() {
		t.Fatal("modified modal test action was allowed to run")
	}
}

func TestConfigPrecisionWorkspaceNavigationAndDensityMarkers(t *testing.T) {
	t.Parallel()

	html := normalizeAssetText(mustReadUIFile(t, "config.html"))
	for _, marker := range []string{
		`id="cfg-density-toggle"`,
		`data-pw-density-toggle`,
		`data-i18n-title="common.workspace_density_toggle"`,
		`aria-pressed="false"`,
	} {
		if !strings.Contains(html, marker) {
			t.Fatalf("config.html missing density control marker %q", marker)
		}
	}

	mainJS := normalizeAssetText(mustReadUIFile(t, "js/config/main.js"))
	for _, marker := range []string{
		`const CONFIG_RECENT_KEY = 'aurago.config.recent.v1'`,
		`const CONFIG_ADVANCED_KEY = 'aurago.config.advanced.v1'`,
		`const CONFIG_RECENT_LIMIT = 6`,
		`function applyConfigDensity(`,
		`function renderConfigOverview(`,
		`function recordRecentSection(`,
		`function configSearchEntriesForSection(`,
		`function focusConfigField(`,
		`function configSectionIcon(`,
		`configSectionIcon(s.key)`,
		`key === 'overview'`,
	} {
		if !strings.Contains(mainJS, marker) {
			t.Fatalf("config main.js missing Precision navigation marker %q", marker)
		}
	}
	if strings.Contains(mainJS, `CONFIG_DENSITY_KEY`) || strings.Contains(mainJS, `aurago.config.density.v1`) {
		t.Fatal("Config main.js must delegate all density-storage ownership to AuraPrecisionWorkspace")
	}
	workspaceJS := normalizeAssetText(mustReadUIFile(t, "js/precision/workspace.js"))
	for _, key := range []string{`aurago.workspace.density.v1`, `aurago.config.density.v1`} {
		if !strings.Contains(workspaceJS, key) {
			t.Errorf("workspace.js missing density ownership key %q", key)
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
		`function controlSnapshot(`,
		`containerSnapshot`,
		`requiredSelectors`,
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
		"config.precision.advanced_title",
		"config.precision.advanced_desc",
		"config.precision.validation_required",
		"config.precision.validation_number",
		"config.precision.validation_min",
		"config.precision.validation_max",
		"config.precision.validation_pattern",
		"config.precision.validation_option",
		"config.precision.validation_url",
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

func TestConfigFormPrimitivesAndAdvancedDisclosure(t *testing.T) {
	t.Parallel()

	form := normalizeAssetText(mustReadUIFile(t, "cfg/form-builder.js"))
	for _, marker := range []string{
		`panel,`,
		`disclosure,`,
		`status,`,
		`emptyState,`,
		`modal,`,
		`actions,`,
		`pw-field`,
	} {
		if !strings.Contains(form, marker) {
			t.Fatalf("AuraConfigForm missing Precision primitive %q", marker)
		}
	}

	mainJS := normalizeAssetText(mustReadUIFile(t, "js/config/main.js"))
	for _, marker := range []string{
		`function enhanceConfigSectionLayout(`,
		`CONFIG_ADVANCED_KEY`,
		`className = 'pw-advanced'`,
		`config.precision.advanced_title`,
		`const configSectionObserver = new MutationObserver`,
		`enhanceConfigSectionLayout(activeSection)`,
	} {
		if !strings.Contains(mainJS, marker) {
			t.Fatalf("advanced disclosure integration missing %q", marker)
		}
	}
}

func TestConfigCatalogDrivesClientValidation(t *testing.T) {
	t.Parallel()

	catalog := normalizeAssetText(mustReadUIFile(t, "js/config/catalog.js"))
	for _, marker := range []string{`validationRules`, `'server.port'`, `min: 1`, `max: 65535`} {
		if !strings.Contains(catalog, marker) {
			t.Fatalf("config catalog validation rules missing %q", marker)
		}
	}

	mainJS := normalizeAssetText(mustReadUIFile(t, "js/config/main.js"))
	for _, marker := range []string{`function configValidationRules(`, `window.AuraConfigState.setRules(configValidationRules())`} {
		if !strings.Contains(mainJS, marker) {
			t.Fatalf("config validation wiring missing %q", marker)
		}
	}
}

func TestConfigPrecisionWorkspaceHasNoInlineStyles(t *testing.T) {
	t.Parallel()

	files := []string{"config.html", "js/config/main.js"}
	entries, err := os.ReadDir("cfg")
	if err != nil {
		t.Fatalf("read cfg directory: %v", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".js") {
			files = append(files, "cfg/"+entry.Name())
		}
	}

	for _, file := range files {
		content := normalizeAssetText(mustReadUIFile(t, file))
		if strings.Contains(content, `style="`) || strings.Contains(content, `style='`) {
			t.Errorf("%s still contains inline style attributes", file)
		}
	}
}

func TestSecretsSharedStylesRemainAvailableInKnowledgeAndConfig(t *testing.T) {
	t.Parallel()

	for _, page := range []string{"config.html", "knowledge.html"} {
		content := normalizeAssetText(mustReadUIFile(t, page))
		if !strings.Contains(content, `/css/secrets-shared.css`) {
			t.Fatalf("%s must load the shared secrets presentation", page)
		}
	}
	css := normalizeAssetText(mustReadUIFile(t, "css/secrets-shared.css"))
	for _, marker := range []string{`.pw-page .secrets-empty`, `#panel-secrets .secrets-empty`, `.secrets-system-badge`} {
		if !strings.Contains(css, marker) {
			t.Fatalf("secrets-shared.css missing scoped marker %q", marker)
		}
	}
}
