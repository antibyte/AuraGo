package ui

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesktopHomepageStudioRouting(t *testing.T) {
	t.Parallel()

	routing := readDesktopAssetText(t, "js/desktop/core/menus-and-routing.js")
	for _, want := range []string{
		"appId === 'homepage-studio'",
		"loadAppScript('homepage-studio')",
		"window.HomepageStudioApp.render",
		"readonly: desktopReadonly()",
		"confirmDialog",
	} {
		if !strings.Contains(routing, want) {
			t.Fatalf("menus-and-routing missing homepage studio marker %q", want)
		}
	}

	foundation := readDesktopAssetText(t, "js/desktop/core/desktop-foundation.js")
	if !strings.Contains(foundation, "'homepage-studio': 'HomepageStudioApp'") {
		t.Fatalf("desktop-foundation missing homepage-studio dispose mapping")
	}
}

func TestHomepageStudioRedesignMarkers(t *testing.T) {
	t.Parallel()

	css := readDesktopAssetText(t, "css/desktop-app-homepage-studio.css")
	for _, want := range []string{
		"--hp-accent:",
		"--hp-glass:",
		"focus-visible",
		"vd-hp-preview-skeleton",
		"text-wrap: balance",
		"prefers-reduced-motion",
		"vd-hp-splitter",
		"vd-hp-viewport-toolbar",
		"vd-hp-device-seg",
		"vd-hp-drift",
		"vd-hp-site",
	} {
		if !strings.Contains(css, want) {
			t.Fatalf("homepage studio css missing redesign marker %q", want)
		}
	}

	source := readDesktopAssetText(t, "js/desktop/apps/homepage-studio.js")
	for _, want := range []string{
		"<aside class=\"vd-hp-chat\"",
		"<main class=\"vd-hp-preview-zone\"",
		"vd-hp-preview-skeleton",
		"preview_empty_title",
		"externalBtn.disabled",
		"data-hp-splitter=\"chat\"",
		"data-hp-splitter=\"inspector\"",
		"vd-hp-status-pill",
		"data-hp-device=\"desktop\"",
		"data-hp-fullscreen",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("homepage studio js missing redesign marker %q", want)
		}
	}
}

func TestHomepageStudioUsesStatusPreviewURL(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/homepage-studio.js")

	for _, want := range []string{
		"function homepageStatusPreviewURL(data, target)",
		"state.previewUrl = homepageStatusPreviewURL(data, state.target);",
		"id=\"hp-url-${windowId}\"",
		"class=\"vd-hp-preview-url\"",
		"previewPanel.insertBefore(iframe, previewLoading);",
		"case 'vercel':",
		"case 'netlify':",
		"case 'here_now':",
		"case 'remote':",
		"data.vercel_url",
		"data.netlify_url",
		"data.here_now_url",
		"data.remote_url",
		"data.preview_url",
		"data.web_container.browser_url",
		"data.python_server.browser_url",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("homepage studio missing preview URL marker %q", want)
		}
	}

	for _, unwanted := range []string{
		"state.previewUrl = 'http://localhost:' + port;",
		"const port = 8080;",
		"previewBody.insertBefore(iframe, previewLoading);",
		"state.previewUrl = homepageStatusPreviewURL(data);",
	} {
		if strings.Contains(source, unwanted) {
			t.Fatalf("homepage studio still contains hard-coded local URL marker %q", unwanted)
		}
	}
}

func TestHomepageStudioUsesExternalHomepageTargets(t *testing.T) {
	t.Parallel()

	// The workbench module keeps the target state and the refresh loop, the
	// sites module owns the collection of the deployment targets themselves.
	main := readDesktopAssetText(t, "js/desktop/apps/homepage-studio.js")
	for _, want := range []string{
		"deploymentTargets: []",
		"function loadHomepageTargets()",
		"/api/desktop/integrations/homepage/sites",
		"/api/desktop/integrations/integrations/webhosts",
		"homepageTargets.collect(site, nextTargets, firstPreviewURL)",
		"homepageTargets.external(target, state.deploymentTargets)",
	} {
		if !strings.Contains(main, want) {
			t.Fatalf("homepage studio external target support missing marker %q", want)
		}
	}
	sites := readDesktopAssetText(t, "js/desktop/apps/homepage-studio-sites.js")
	for _, want := range []string{
		"function collectTargets(",
		"function externalTargetURL(",
		"deploy_targets",
		"last_deploy_url",
		"provider_target_id",
		"remote_path",
	} {
		if !strings.Contains(sites, want) {
			t.Fatalf("homepage studio sites module external target support missing marker %q", want)
		}
	}
	for _, source := range []string{main, sites} {
		if strings.Contains(source, "sites.slice(0,") {
			t.Fatal("homepage studio must not cap managed site detail loading before resolving external targets")
		}
	}
}

func TestHomepageStudioUsesLedgerDeploymentsAndObservationsForExternalTargets(t *testing.T) {
	t.Parallel()

	// The deployment ledger and the remote observations are site data, so the
	// sites module owns the fallback that turns them into external targets.
	source := readDesktopAssetText(t, "js/desktop/apps/homepage-studio-sites.js")
	for _, want := range []string{
		"site.deployments",
		"site.remote_observations",
		"deployment.provider",
		"deployment.url",
		"observation.provider",
		"observation.url",
		"provider_deploy_id",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("homepage studio external target fallback missing marker %q", want)
		}
	}
}

func TestHomepageStudioCloudTargetsFallbackToKnownExternalURL(t *testing.T) {
	t.Parallel()

	// The known cloud providers and the fallback to a non-local deployment
	// target live in the sites module that owns the target list.
	source := readDesktopAssetText(t, "js/desktop/apps/homepage-studio-sites.js")
	for _, want := range []string{
		"const externalTargets = ['remote', 'vercel', 'netlify', 'here_now'];",
		"externalTargets.includes(selected)",
		"item.provider !== 'local'",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("homepage studio cloud target fallback missing marker %q", want)
		}
	}
}

func TestHomepageStudioPreviewSandboxKeepsOpaqueOrigin(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/homepage-studio.js")
	for _, want := range []string{
		"iframe.sandbox = 'allow-scripts allow-forms';",
		"iframe.referrerPolicy = 'no-referrer';",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("homepage studio preview iframe missing sandbox marker %q", want)
		}
	}
	for _, unwanted := range []string{
		"allow-same-origin",
		"allow-popups",
	} {
		if strings.Contains(source, unwanted) {
			t.Fatalf("homepage studio preview iframe must not use sandbox flag %q", unwanted)
		}
	}
}

func TestHomepageStudioChatPayloadCarriesHomepageScope(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/homepage-studio.js")
	for _, want := range []string{
		"function homepageWindowContext()",
		"window_context: homepageWindowContext()",
		"homepage_mode: true",
		"Use homepage_project, homepage_file, homepage_quality, homepage_deploy, and homepage_git.",
		"Do not use virtual_desktop apps, widgets, or files for Homepage Studio site changes.",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("homepage studio chat payload missing scope marker %q", want)
		}
	}
}

func TestHomepageStudioHistoryPanelMarkers(t *testing.T) {
	t.Parallel()

	css := readDesktopAssetText(t, "css/desktop-app-homepage-studio.css")
	for _, want := range []string{
		".vd-hp-preview-tabs",
		".vd-hp-history-panel",
		".vd-hp-history-controls",
		".vd-hp-history-entry",
		".vd-hp-history-type-decision",
		".vd-hp-history-delete",
	} {
		if !strings.Contains(css, want) {
			t.Fatalf("homepage studio css missing history marker %q", want)
		}
	}

	source := readDesktopAssetText(t, "js/desktop/apps/homepage-studio.js")
	for _, want := range []string{
		"hp-tab-history-",
		"vd-hp-history-controls",
		"vd-hp-history-search",
		"vd-hp-history-list",
		"history_tab",
		"history_search_placeholder",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("homepage studio js missing history shell marker %q", want)
		}
	}

	module := readDesktopAssetText(t, "js/desktop/apps/homepage-studio-history.js")
	for _, want := range []string{
		"function loadHistory",
		"function renderHistory",
		"/api/desktop/integrations/homepage/history",
		"history_delete_confirm",
		"homepage_studio.",
		"history_type_",
		"history_load_more",
		"confirmDialog",
		"params.set('offset'",
		"params.set('project_id'",
	} {
		if !strings.Contains(module, want) {
			t.Fatalf("homepage studio history module missing marker %q", want)
		}
	}

	var en map[string]string
	if err := json.Unmarshal([]byte(readDesktopAssetText(t, "lang/desktop/en.json")), &en); err != nil {
		t.Fatalf("parse English desktop translations: %v", err)
	}
	for _, key := range []string{
		"homepage_studio.history_tab",
		"homepage_studio.history_search_placeholder",
		"homepage_studio.history_filter_label",
		"homepage_studio.history_filter_all",
		"homepage_studio.history_loading",
		"homepage_studio.history_empty",
		"homepage_studio.history_error",
		"homepage_studio.history_delete",
		"homepage_studio.history_delete_confirm",
		"homepage_studio.history_type_decision",
		"homepage_studio.history_load_more",
	} {
		if en[key] == "" {
			t.Fatalf("English desktop translation missing key %q", key)
		}
	}
}

func TestHomepageStudioGermanUsesInformalAddress(t *testing.T) {
	t.Parallel()

	var values map[string]string
	if err := json.Unmarshal([]byte(readDesktopAssetText(t, "lang/desktop/de.json")), &values); err != nil {
		t.Fatalf("parse German desktop translations: %v", err)
	}
	for key, want := range map[string]string{
		"homepage_studio.chat_placeholder":    "Beschreibe deine Website-Änderungen...",
		"homepage_studio.preview_unavailable": "Vorschau nicht verfügbar — starte zuerst den Homepage-Container",
		"homepage_studio.preview_empty_title": "Noch keine Live-Vorschau",
		"homepage_studio.welcome":             "Willkommen im Homepage-Studio! Beschreibe die Website, die du erstellen möchtest, und ich erstelle sie für dich.",
	} {
		if values[key] != want {
			t.Fatalf("%s = %q, want %q", key, values[key], want)
		}
	}
}

func TestHomepageStudioModuleSplitAndLoadOrder(t *testing.T) {
	t.Parallel()

	loader := readDesktopAssetText(t, "js/desktop/core/module-loader.js")
	scripts := []string{
		"/js/desktop/apps/homepage-studio-preview.js",
		"/js/desktop/apps/homepage-studio-sites.js",
		"/js/desktop/apps/homepage-studio-history.js",
		"/js/desktop/apps/homepage-studio.js",
	}
	lastIndex := -1
	for _, script := range scripts {
		idx := strings.Index(loader, "'"+script+"'")
		if idx < 0 {
			t.Fatalf("module-loader missing homepage-studio script %q", script)
		}
		if idx < lastIndex {
			t.Fatalf("module-loader loads %q out of order (helpers must load before the main module)", script)
		}
		lastIndex = idx
	}

	for global, file := range map[string]string{
		"window.HomepageStudioPreview": "js/desktop/apps/homepage-studio-preview.js",
		"window.HomepageStudioSites":   "js/desktop/apps/homepage-studio-sites.js",
		"window.HomepageStudioHistory": "js/desktop/apps/homepage-studio-history.js",
		"window.HomepageStudioApp":     "js/desktop/apps/homepage-studio.js",
	} {
		source := readDesktopAssetText(t, file)
		if !strings.Contains(source, global) {
			t.Fatalf("%s does not expose %s", file, global)
		}
	}
}

func TestHomepageStudioPreviewModuleMarkers(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/homepage-studio-preview.js")
	for _, want := range []string{
		"window.HomepageStudioPreview = { create",
		"data-hp-device",
		"data-hp-fullscreen",
		"requestFullscreen",
		"fullscreenchange",
		"dispose",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("homepage studio preview module missing marker %q", want)
		}
	}
}

func TestHomepageStudioSitesPanelMarkers(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/homepage-studio-sites.js")
	for _, want := range []string{
		"window.HomepageStudioSites = {",
		"create,",
		"DRIFT_KEYS,",
		"driftStatus,",
		"targets: { collect: collectTargets, add: addTarget, normalize: normalizeTargetProvider, external: externalTargetURL }",
		"/api/desktop/integrations/homepage/sites",
		"/reconcile",
		"drift_status",
		"deploy_targets",
		"remote_observations",
		"last_deployed_at",
		"vd-hp-drift-",
		"noopener noreferrer",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("homepage studio sites module missing marker %q", want)
		}
	}

	main := readDesktopAssetText(t, "js/desktop/apps/homepage-studio.js")
	for _, want := range []string{
		"window.HomepageStudioSites.create",
		"window.HomepageStudioHistory.create",
		"window.HomepageStudioPreview.create",
		"onSiteSelected",
	} {
		if !strings.Contains(main, want) {
			t.Fatalf("homepage studio main module missing module wiring marker %q", want)
		}
	}
}

func TestHomepageStudioHasNoNativeDialogs(t *testing.T) {
	t.Parallel()

	files := []string{
		"js/desktop/apps/homepage-studio.js",
		"js/desktop/apps/homepage-studio-preview.js",
		"js/desktop/apps/homepage-studio-sites.js",
		"js/desktop/apps/homepage-studio-history.js",
	}
	for _, file := range files {
		source := readDesktopAssetText(t, file)
		for _, unwanted := range []string{"confirm(", "alert(", "prompt("} {
			if strings.Contains(source, unwanted) {
				t.Fatalf("%s must use shell dialogs instead of native %q", file, unwanted)
			}
		}
	}
}

func TestHomepageStudioPersistsWorkbenchDraft(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/homepage-studio.js")
	for _, want := range []string{
		"aurago.desktop.homepage.draft.",
		"persistHomepageDraft",
		"chatCollapsed",
		"inspectorCollapsed",
		"inspectorTab",
		"selectedSiteId",
		"historyQuery",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("homepage studio persistence missing marker %q", want)
		}
	}
}

func TestHomepageStudioWindowPreset(t *testing.T) {
	t.Parallel()

	shell := readDesktopAssetText(t, "js/desktop/core/window-shell-runtime.js")
	for _, want := range []string{
		"'homepage-studio': { width: 1240, height: 760 }",
		"'homepage-studio': true",
	} {
		if !strings.Contains(shell, want) {
			t.Fatalf("window-shell-runtime missing homepage studio preset marker %q", want)
		}
	}
}

func TestHomepageStudioNewTranslations(t *testing.T) {
	t.Parallel()

	newKeys := []string{
		"homepage_studio.brand_subtitle",
		"homepage_studio.assistant_toggle",
		"homepage_studio.inspector_toggle",
		"homepage_studio.suggest_landing",
		"homepage_studio.device_desktop",
		"homepage_studio.device_tablet",
		"homepage_studio.device_mobile",
		"homepage_studio.fullscreen",
		"homepage_studio.agent_working",
		"homepage_studio.sites_tab",
		"homepage_studio.sites_reconcile",
		"homepage_studio.sites_drift_clean",
		"homepage_studio.sites_drift_not_deployed",
		"homepage_studio.readonly_hint",
	}
	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		lang := lang
		t.Run(lang, func(t *testing.T) {
			var values map[string]string
			if err := json.Unmarshal([]byte(readDesktopAssetText(t, "lang/desktop/"+lang+".json")), &values); err != nil {
				t.Fatalf("parse %s desktop translations: %v", lang, err)
			}
			for _, key := range newKeys {
				if values[key] == "" {
					t.Fatalf("%s desktop translation missing new key %q", lang, key)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Behaviour checks
//
// The checks below execute the shipped Homepage Studio modules so they assert
// on runtime behaviour instead of on source text.
// ---------------------------------------------------------------------------

// homepageStudioNodePrelude exposes the production modules to a Node script.
// The single verb is the JSON encoded map of module sources.
const homepageStudioNodePrelude = `
const assert = require('node:assert/strict');
const SRC = %s;

function loadModule(source, globalName, extra) {
	const sandbox = Object.assign({ location: { origin: 'https://aurago.test' } }, extra || {});
	(new Function('window', source))(sandbox);
	if (!sandbox[globalName]) throw new Error('module did not expose ' + globalName);
	return sandbox[globalName];
}

function extractBlock(source, marker) {
	const start = source.indexOf(marker);
	if (start < 0) throw new Error('marker not found: ' + marker);
	let depth = 0;
	let i = source.indexOf('{', start);
	if (i < 0) throw new Error('marker has no body: ' + marker);
	for (; i < source.length; i++) {
		const ch = source[i];
		if (ch === '{') depth++;
		else if (ch === '}') {
			depth--;
			if (depth === 0) return source.slice(start, i + 1);
		}
	}
	throw new Error('unbalanced block: ' + marker);
}

function extractFunction(source, name) {
	const marker = 'function ' + name + '(';
	const start = source.indexOf(marker);
	if (start < 0) throw new Error('function not found: ' + name);
	const prefix = source.slice(Math.max(0, start - 6), start);
	if (prefix.endsWith('async ')) return 'async ' + extractBlock(source, marker);
	return extractBlock(source, marker);
}

function fakeElement() {
	return {
		attributes: {},
		dataset: {},
		style: {
			props: {},
			setProperty(name, value) { this.props[name] = String(value); },
			getPropertyValue(name) { return Object.prototype.hasOwnProperty.call(this.props, name) ? this.props[name] : ''; }
		},
		classList: { toggle() {}, add() {}, remove() {}, contains() { return false; } },
		addEventListener() {},
		removeEventListener() {},
		setAttribute(name, value) { this.attributes[name] = String(value); },
		getAttribute(name) { return Object.prototype.hasOwnProperty.call(this.attributes, name) ? this.attributes[name] : null; },
		querySelector() { return null; },
		querySelectorAll() { return []; },
		getBoundingClientRect() { return { width: 600, height: 400, left: 0, right: 600, top: 0, bottom: 400 }; },
		focus() {},
		click() {},
		setPointerCapture() {},
		releasePointerCapture() {},
		className: '',
		value: '',
		textContent: ''
	};
}

function elementBy(html, attribute, value) {
	const needle = ' ' + attribute + '="' + value + '"';
	const at = html.indexOf(needle);
	if (at < 0) return null;
	const open = html.lastIndexOf('<', at);
	const close = html.indexOf('>', at);
	if (open < 0 || close < 0) return null;
	const markup = html.slice(open, close + 1);
	const attrs = {};
	const attrPattern = /([a-zA-Z0-9:_.-]+)="([^"]*)"/g;
	let match;
	while ((match = attrPattern.exec(markup)) !== null) attrs[match[1]] = match[2];
	return { tag: markup.slice(1).split(/[\s>]/)[0], attrs: attrs, markup: markup };
}

function elementById(html, id) { return elementBy(html, 'id', id); }

function countOf(text, needle) { return text.split(needle).length - 1; }
`

// homepageStudioNodeSources embeds the shipped modules into the Node script.
func homepageStudioNodeSources(t *testing.T) string {
	t.Helper()
	sources := map[string]string{
		"main":    readDesktopAssetText(t, "js/desktop/apps/homepage-studio.js"),
		"sites":   readDesktopAssetText(t, "js/desktop/apps/homepage-studio-sites.js"),
		"preview": readDesktopAssetText(t, "js/desktop/apps/homepage-studio-preview.js"),
		"shared":  readDesktopAssetText(t, "js/shared/shared-core.js"),
	}
	encoded, err := json.Marshal(sources)
	if err != nil {
		t.Fatalf("encode homepage studio sources: %v", err)
	}
	return string(encoded)
}

// runHomepageStudioNode executes a Node script against the production modules.
func runHomepageStudioNode(t *testing.T, script string) {
	t.Helper()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node runtime required for the Homepage Studio behaviour checks")
	}
	// The script carries the production modules, so it goes through a file
	// instead of the command line.
	path := filepath.Join(t.TempDir(), "homepage-studio-check.cjs")
	if err := os.WriteFile(path, []byte(script), 0o600); err != nil {
		t.Fatalf("write homepage studio script: %v", err)
	}
	if output, err := exec.Command(node, path).CombinedOutput(); err != nil {
		t.Fatalf("homepage studio behaviour check failed: %v\n%s", err, output)
	}
}

// homepageStudioNodeScript wraps a Node body into a failing runner so a thrown
// assertion ends the process with a non-zero exit code.
func homepageStudioNodeScript(t *testing.T, body string) string {
	t.Helper()
	return fmt.Sprintf(homepageStudioNodePrelude, homepageStudioNodeSources(t)) +
		"(async () => {\n" + body +
		"\n})().catch(error => { console.error(error && error.stack ? error.stack : error); process.exit(1); });\n"
}

// TestHomepageStudioPreviewURLSelectionPerTarget executes the production
// preview URL selection for every deployment target.
func TestHomepageStudioPreviewURLSelectionPerTarget(t *testing.T) {
	t.Parallel()

	script := homepageStudioNodeScript(t, `
const Sites = loadModule(SRC.sites, 'HomepageStudioSites');
const scope = new Function('window', 'state', 'homepageTargets', [
	extractFunction(SRC.main, 'firstString'),
	extractFunction(SRC.main, 'safeExternalURL'),
	extractFunction(SRC.main, 'firstPreviewURL'),
	extractFunction(SRC.main, 'homepageStatusPreviewURL')
].join('\n') + '\nreturn { select: homepageStatusPreviewURL, preview: firstPreviewURL };');
const state = { deploymentTargets: [] };
const api = scope({ location: { origin: 'https://aurago.test' } }, state, Sites.targets);

const status = {
	status: 'ok',
	mode: 'python_fallback',
	preview_url: 'https://preview.test/app',
	tunnel_url: 'https://tunnel.test/app',
	web_container: { running: true, browser_url: 'https://web.test/app' },
	python_server: { running: true, browser_url: 'https://python.test/app' },
	vercel_url: 'https://vercel.test/app',
	netlify_url: 'https://netlify.test/app',
	here_now_url: 'https://herenow.test/app',
	remote_url: 'https://remote.test/app'
};
const localTarget = [];
Sites.targets.add(localTarget, 'local', 'https://local.test/app', 'Local', api.preview);
const vercelTarget = [];
Sites.targets.add(vercelTarget, 'vercel', 'https://deploy.vercel.test/app', 'Vercel', api.preview);
const remoteTarget = [];
Sites.targets.add(remoteTarget, 'remote', 'https://deploy.remote.test/app', 'Remote', api.preview);

const cases = [
	['local prefers the preview url', 'local', status, [], 'https://preview.test/app'],
	['local falls back to the tunnel', 'local', { status: 'ok', web_container: { running: true }, tunnel_url: 'https://tunnel.test/app' }, [], 'https://tunnel.test/app'],
	['local falls back to the web container', 'local', { status: 'ok', web_container: { running: true, browser_url: 'https://web.test/app' } }, [], 'https://web.test/app'],
	['local falls back to the python server', 'local', { status: 'ok', python_server: { running: true, browser_url: 'https://python.test/app' } }, [], 'https://python.test/app'],
	['local without any url stays empty', 'local', { status: 'ok' }, [], ''],
	['vercel uses the vercel url', 'vercel', status, [], 'https://vercel.test/app'],
	['netlify uses the netlify url', 'netlify', status, [], 'https://netlify.test/app'],
	['here_now uses the here now url', 'here_now', status, [], 'https://herenow.test/app'],
	['remote uses the remote url', 'remote', status, [], 'https://remote.test/app'],
	['vercel falls back to its deployment target', 'vercel', { status: 'ok' }, vercelTarget, 'https://deploy.vercel.test/app'],
	['remote falls back to its deployment target', 'remote', { status: 'ok' }, remoteTarget, 'https://deploy.remote.test/app'],
	['remote falls back to a known external target', 'remote', { status: 'ok' }, vercelTarget, 'https://deploy.vercel.test/app'],
	['a local target never serves a cloud target', 'vercel', { status: 'ok' }, localTarget, ''],
	['a rejected url is not previewed', 'vercel', { status: 'ok', vercel_url: 'javascript:alert(1)' }, vercelTarget, 'https://deploy.vercel.test/app'],
	['a rejected preview url is not previewed', 'local', { status: 'ok', preview_url: 'javascript:alert(1)' }, [], '']
];
for (const item of cases) {
	state.deploymentTargets = item[3];
	assert.equal(api.select(item[2], item[1]), item[4], item[0]);
}
`)
	runHomepageStudioNode(t, script)
}

// TestHomepageStudioDeviceSwitcherProducesThreePreviewWidths executes the
// preview module and proves the three devices render three distinct widths.
func TestHomepageStudioDeviceSwitcherProducesThreePreviewWidths(t *testing.T) {
	t.Parallel()

	script := homepageStudioNodeScript(t, `
global.document = { fullscreenElement: null, addEventListener() {}, removeEventListener() {} };
const Preview = loadModule(SRC.preview, 'HomepageStudioPreview');
const root = fakeElement();
const frame = fakeElement();
const panel = fakeElement();
root.querySelector = selector => {
	if (selector === '.vd-hp-preview-frame') return frame;
	if (selector === '.vd-hp-preview-panel') return panel;
	return null;
};
const state = { device: 'desktop' };
const preview = Preview.create({ root: root, zone: fakeElement(), stage: fakeElement(), state: state, t: key => key });
const widths = [];
const scales = [];
for (const device of Preview.DEVICES) {
	preview.setDevice(device);
	widths.push(frame.style.getPropertyValue('--hp-device-w'));
	scales.push(Number(frame.style.getPropertyValue('--hp-device-scale')));
}
assert.deepEqual(widths, ['1280px', '768px', '390px'], 'every device renders the preview at its real width');
assert.equal(new Set(widths).size, 3, 'the three devices must produce three distinct preview widths');
assert.equal(state.device, 'mobile');
assert.equal(preview.getDevice(), 'mobile');
assert.ok(scales.every(scale => scale > 0), 'every device viewport is scaled into the preview pane');
assert.ok(scales[0] < 1, 'the desktop viewport is scaled down into the preview pane');
assert.ok(scales[2] >= 1, 'the mobile viewport fits into the preview pane');
`)
	runHomepageStudioNode(t, script)
}

// TestHomepageStudioTargetRefreshStaysWithinRequestBudget proves that one
// target refresh never fans out into one request per site.
func TestHomepageStudioTargetRefreshStaysWithinRequestBudget(t *testing.T) {
	t.Parallel()

	script := homepageStudioNodeScript(t, `
const Sites = loadModule(SRC.sites, 'HomepageStudioSites');
const scope = new Function('window', 'state', 'api', 'homepageTargets', [
	extractFunction(SRC.main, 'firstString'),
	extractFunction(SRC.main, 'safeExternalURL'),
	extractFunction(SRC.main, 'firstPreviewURL'),
	extractFunction(SRC.main, 'safeHomepageApi'),
	extractFunction(SRC.main, 'loadHomepageTargets')
].join('\n') + '\nreturn loadHomepageTargets;');

const calls = [];
let panelSites = [];
let cachedDetail = null;
const api = async path => {
	calls.push(String(path));
	if (path === '/api/desktop/integrations/integrations/webhosts') {
		return { webhosts: [{ id: 'homepage', url: 'https://local.test/app', name: 'Homepage' }] };
	}
	if (path === '/api/desktop/integrations/homepage/sites') {
		return { sites: panelSites.slice() };
	}
	if (/^\/api\/desktop\/integrations\/homepage\/sites\/\d+$/.test(path)) {
		return { site: { id: Number(path.split('/').pop()), name: 'Selected', deploy_targets: [{ provider: 'vercel', url: 'https://selected.test/app' }] } };
	}
	return {};
};
const state = {
	selectedSiteId: 0,
	deploymentTargets: [],
	targetsLoading: null,
	modules: { sites: { getSites: () => panelSites, getDetail: () => cachedDetail } }
};
const refresh = scope({ location: { origin: 'https://aurago.test' } }, state, api, Sites.targets);
const listCalls = () => calls.filter(path => path === '/api/desktop/integrations/homepage/sites').length;
const detailCalls = () => calls.filter(path => /^\/api\/desktop\/integrations\/homepage\/sites\/\d+$/.test(path)).length;

panelSites = [];
for (let index = 1; index <= 30; index++) {
	panelSites.push({ id: index, name: 'Site ' + index, deploy_targets: [{ provider: 'vercel', url: 'https://v' + index + '.test/app' }] });
}

calls.length = 0;
state.selectedSiteId = 0;
await refresh();
assert.equal(listCalls(), 0, 'the site list of the panel must be reused');
assert.equal(detailCalls(), 0, 'without a selection no site detail is needed');
assert.equal(state.deploymentTargets.length, 31, 'every site contributes its deployment target');

calls.length = 0;
state.selectedSiteId = 7;
cachedDetail = null;
await refresh();
assert.equal(listCalls(), 0, 'the site list must not be fetched a second time');
assert.equal(detailCalls(), 1, 'only the selected site needs its detail');

calls.length = 0;
panelSites = [];
state.selectedSiteId = 0;
await refresh();
assert.equal(listCalls(), 1, 'the site list is fetched at most once');
assert.equal(detailCalls(), 0, 'no site detail is fetched without a selection');

calls.length = 0;
state.selectedSiteId = 3;
cachedDetail = null;
await refresh();
assert.equal(listCalls(), 1, 'the site list stays at one request');
assert.equal(detailCalls(), 1, 'only the selected site needs its detail');

calls.length = 0;
await Promise.all([refresh(), refresh()]);
assert.equal(calls.filter(path => path === '/api/desktop/integrations/integrations/webhosts').length, 1, 'concurrent refreshes share one request');
`)
	runHomepageStudioNode(t, script)
}

// TestHomepageStudioDynamicKeysFallBackToLabels proves that a missing
// translation yields the fallback label and never the raw key path.
func TestHomepageStudioDynamicKeysFallBackToLabels(t *testing.T) {
	t.Parallel()

	script := homepageStudioNodeScript(t, `
const dict = { 'homepage_studio.sites_drift_clean': 'Clean' };
const sharedScope = new Function('I18N', extractFunction(SRC.shared, 't') + '\nreturn t;');
const sharedT = sharedScope(dict);
assert.equal(sharedT('homepage_studio.sites_drift_missing', 'not_deployed'), 'homepage_studio.sites_drift_missing', 'the shared translator treats the second argument as a placeholder map');

const trScope = new Function('t', extractBlock(SRC.main, 'const tr = (key, fallback) =>') + '\nreturn tr;');
const t = key => (Object.prototype.hasOwnProperty.call(dict, key) ? dict[key] : key);
const tr = trScope(t);
assert.equal(tr('homepage_studio.sites_drift_clean', 'clean'), 'Clean', 'a translated key keeps its translation');
assert.equal(tr('homepage_studio.sites_drift_missing', 'not_deployed'), 'not_deployed', 'a missing key falls back to the label');

global.document = { activeElement: null, documentElement: { lang: 'en' } };
const Sites = loadModule(SRC.sites, 'HomepageStudioSites');
const container = { innerHTML: '', querySelectorAll: () => [], querySelector: () => null };
const panel = Sites.create({
	container: container,
	esc: value => String(value == null ? '' : value),
	t: t,
	tr: tr,
	api: async () => ({ sites: [
		{ id: 1, name: 'One', drift_status: 'not_deployed' },
		{ id: 2, name: 'Two', drift_status: 'clean' }
	] }),
	notify: () => {},
	readonly: true,
	safeExternalURL: value => String(value || ''),
	locale: 'en',
	isDisposed: () => false
});
await panel.load();
panel.setSelected(1, {
	id: 1,
	name: 'One',
	drift_status: 'not_deployed',
	local_root: '/srv/one',
	deploy_targets: [{ provider: 'vercel', url: 'https://v.test/app', status: 'active' }]
});
const html = container.innerHTML;
assert.ok(html.includes('>not_deployed<'), 'the drift badge uses the fallback label');
assert.ok(!html.includes('homepage_studio.sites_drift_'), 'the drift key path must not leak');
assert.ok(!html.includes('homepage_studio.sites_target_'), 'the target key path must not leak');
assert.ok(!html.includes('homepage_studio.sites_origin_'), 'the origin key path must not leak');
assert.ok(html.includes('vercel \u00b7 active'), 'the target status falls back to the plain label');
assert.ok(html.includes('role="option"'), 'the site cards are listbox options');
assert.ok(html.includes('aria-selected="true"'), 'the selected card is marked');
assert.equal(countOf(html, 'tabindex="0"'), 1, 'exactly one card is the tab stop');
assert.equal(countOf(html, 'tabindex="-1"'), 1, 'the other cards are not tab stops');
`)
	runHomepageStudioNode(t, script)
}

// TestHomepageStudioLiveRegionContract renders the workbench markup with the
// production renderer and checks the accessibility contract of the shell.
func TestHomepageStudioLiveRegionContract(t *testing.T) {
	t.Parallel()

	script := homepageStudioNodeScript(t, `
global.document = { documentElement: { lang: 'en' } };
const Sites = loadModule(SRC.sites, 'HomepageStudioSites');
const sandbox = { location: { origin: 'https://aurago.test' }, SYSTEM_LANG: 'en', HomepageStudioSites: Sites };
(new Function('window', SRC.main))(sandbox);
const App = sandbox.HomepageStudioApp;
assert.ok(App && typeof App.render === 'function', 'the workbench module must expose render');

class StopRender extends Error {}
let markup = '';
const container = {
	set innerHTML(value) { markup = value; throw new StopRender(); },
	get innerHTML() { return markup; },
	querySelector: () => null,
	querySelectorAll: () => []
};
const context = {
	esc: value => String(value == null ? '' : value),
	api: async () => ({}),
	t: key => key,
	iconMarkup: () => '',
	notify: () => {},
	confirmDialog: async () => false,
	readonly: false
};
try {
	App.render(container, 'test-window', context);
	throw new Error('the workbench did not render its markup');
} catch (error) {
	if (!(error instanceof StopRender)) throw error;
}
assert.ok(markup.length > 0, 'the workbench must render its markup');

const chatLog = elementById(markup, 'hp-log-test-window');
assert.ok(chatLog, 'the chat log must exist');
assert.equal(chatLog.attrs['aria-live'], undefined, 'the streaming chat log must not be an aria-live region');
assert.equal(chatLog.attrs['aria-busy'], 'false', 'the chat log announces its busy state');

const announce = elementById(markup, 'hp-announce-test-window');
assert.ok(announce, 'the announce node must exist');
assert.equal(announce.attrs['role'], 'status', 'finished messages need a dedicated status node');

const tablist = elementBy(markup, 'class', 'vd-hp-preview-tablist');
assert.ok(tablist, 'the inspector tabs need a tablist');
assert.equal(tablist.attrs['role'], 'tablist');

const sitesTab = elementById(markup, 'hp-tab-sites-test-window');
assert.ok(sitesTab, 'the sites tab must exist');
assert.equal(sitesTab.attrs['role'], 'tab');
assert.equal(sitesTab.attrs['aria-selected'], 'true');
assert.equal(sitesTab.attrs['tabindex'], '0');

const historyTab = elementById(markup, 'hp-tab-history-test-window');
assert.ok(historyTab, 'the history tab must exist');
assert.equal(historyTab.attrs['role'], 'tab');
assert.equal(historyTab.attrs['aria-selected'], 'false');
assert.equal(historyTab.attrs['tabindex'], '-1');

const chatSplitter = elementBy(markup, 'data-hp-splitter', 'chat');
assert.ok(chatSplitter, 'the chat splitter must exist');
assert.equal(chatSplitter.attrs['role'], 'separator');
assert.ok(chatSplitter.attrs['aria-valuenow'], 'the splitter must expose its current width');
assert.ok(chatSplitter.attrs['aria-valuemin'], 'the splitter must expose its minimum width');
assert.ok(chatSplitter.attrs['aria-valuemax'], 'the splitter must expose its maximum width');

const inspectorSplitter = elementBy(markup, 'data-hp-splitter', 'inspector');
assert.ok(inspectorSplitter, 'the inspector splitter must exist');
assert.ok(inspectorSplitter.attrs['aria-valuenow'], 'the splitter must expose its current width');

const deviceGroup = elementBy(markup, 'class', 'vd-hp-device-seg');
assert.ok(deviceGroup, 'the device switcher must exist');
assert.equal(deviceGroup.attrs['role'], 'radiogroup');
const desktopButton = elementBy(markup, 'data-hp-device', 'desktop');
assert.ok(desktopButton, 'the desktop device button must exist');
assert.equal(desktopButton.attrs['role'], 'radio');
assert.equal(desktopButton.attrs['aria-checked'], 'true');

const sitesList = elementById(markup, 'hp-sites-list-test-window');
assert.ok(sitesList, 'the site list must exist');
assert.equal(sitesList.attrs['role'], 'listbox');
`)
	runHomepageStudioNode(t, script)
}
