package ui

import (
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/go-rod/rod"
)

func requirePrecisionBrowserSmoke(t *testing.T) {
	t.Helper()
	if os.Getenv("AURAGO_RUN_BROWSER_SMOKE") != "1" {
		t.Skip("set AURAGO_RUN_BROWSER_SMOKE=1 to run the headless browser smoke test")
	}
}

func newPrecisionSmokeOrigin(t *testing.T) string {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("<!doctype html><title>Precision fixture</title>"))
	}))
	t.Cleanup(server.Close)
	return server.URL
}

func loadPrecisionWorkspaceScript(t *testing.T, page *rod.Page, body string) {
	t.Helper()
	page.MustSetDocumentContent("<!doctype html><html><head><meta charset=\"utf-8\"></head>" + body + "</html>")
	if err := page.AddScriptTag("", normalizeAssetText(mustReadUIFile(t, "js/precision/workspace.js"))); err != nil {
		t.Fatalf("load Precision Workspace client: %v", err)
	}
}

func TestPrecisionWorkspaceClientBrowserSmoke(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	browser := newSmokeBrowser(t)
	origin := newPrecisionSmokeOrigin(t)

	t.Run("density controls routes and icons", func(t *testing.T) {
		page := browser.MustPage(origin + "/missions/v2/")
		defer page.MustClose()
		page.MustSetViewport(1024, 768, 1, false)
		page.MustSetDocumentContent(`<!doctype html><html><body class="pw-page" data-workspace-page="missions">
			<button id="density" data-pw-density-toggle aria-pressed="false"><span data-pw-density-label></span></button>
			<nav id="radialMenuAnchor">
				<a id="mission-link" class="radial-item" href="/missions"><span class="radial-item-icon">legacy</span></a>
				<a id="media-link" class="radial-item" href="/gallery"><span class="radial-item-icon">legacy</span></a>
			</nav>
		</body></html>`)
		page.MustEval(`() => {
			localStorage.clear();
			window.__densityEvents = [];
			window.addEventListener('aurago:workspace-density-change', event => window.__densityEvents.push(event.detail));
			const labels = {
				'common.workspace_density_toggle': 'Dichte umschalten',
				'common.workspace_density_comfortable': 'Komfortabel',
				'common.workspace_density_compact': 'Kompakt'
			};
			window.t = key => labels[key] || key;
		}`)
		if err := page.AddScriptTag("", normalizeAssetText(mustReadUIFile(t, "js/precision/workspace.js"))); err != nil {
			t.Fatalf("load Precision Workspace client: %v", err)
		}

		if got := page.MustEval(`() => window.AuraPrecisionWorkspace.getDensity()`).String(); got != "comfortable" {
			t.Fatalf("default density = %q, want comfortable", got)
		}
		if got := page.MustEval(`() => window.AuraPrecisionWorkspace.setDensity('wide')`).String(); got != "comfortable" {
			t.Fatalf("invalid density normalized to %q, want comfortable", got)
		}
		if got := page.MustEval(`() => window.__densityEvents.length`).Int(); got != 0 {
			t.Fatalf("invalid/current density dispatched %d events, want 0", got)
		}
		page.MustEval(`() => window.AuraPrecisionWorkspace.setDensity('compact')`)
		page.MustEval(`() => window.AuraPrecisionWorkspace.setDensity('compact')`)
		state := page.MustEval(`() => ({
			density: window.AuraPrecisionWorkspace.getDensity(),
			bodyDensity: document.body.dataset.density,
			stored: localStorage.getItem('aurago.workspace.density.v1'),
			events: window.__densityEvents,
			pressed: document.getElementById('density').getAttribute('aria-pressed'),
			label: document.querySelector('[data-pw-density-label]').textContent,
			labelKey: document.querySelector('[data-pw-density-label]').getAttribute('data-i18n'),
			title: document.getElementById('density').title,
			ariaLabel: document.getElementById('density').getAttribute('aria-label'),
			active: document.getElementById('mission-link').getAttribute('aria-current'),
			inactive: document.getElementById('media-link').hasAttribute('aria-current'),
			missionIcons: document.querySelectorAll('#mission-link svg.pw-radial-outline-icon').length,
			mediaIconKey: document.querySelector('#media-link .radial-item-icon').dataset.pwIcon
		})`).Map()
		if state["density"].String() != "compact" || state["bodyDensity"].String() != "compact" || state["stored"].String() != "compact" {
			t.Fatalf("compact density state was not synchronized: %#v", state)
		}
		if len(state["events"].Arr()) != 1 {
			t.Fatalf("real density change dispatched %d events, want 1", len(state["events"].Arr()))
		}
		if state["pressed"].String() != "true" || state["label"].String() != "Kompakt" || state["labelKey"].String() != "common.workspace_density_compact" {
			t.Fatalf("translated compact control state is incomplete: %#v", state)
		}
		if state["title"].String() != "Dichte umschalten" || state["ariaLabel"].String() != "Dichte umschalten" {
			t.Fatalf("translated density control labels are incomplete: %#v", state)
		}
		if state["active"].String() != "page" || state["inactive"].Bool() || state["missionIcons"].Int() != 1 || state["mediaIconKey"].String() != "/media" {
			t.Fatalf("canonical route/icon enhancement mismatch: %#v", state)
		}

		before := page.MustEval(`() => document.querySelector('#mission-link .radial-item-icon').innerHTML`).String()
		page.MustEval(`() => { window.AuraPrecisionWorkspace.init(); window.AuraPrecisionWorkspace.init(); }`)
		after := page.MustEval(`() => document.querySelector('#mission-link .radial-item-icon').innerHTML`).String()
		if before != after || page.MustEval(`() => document.querySelectorAll('#mission-link svg.pw-radial-outline-icon').length`).Int() != 1 {
			t.Fatal("radial outline icon replacement is not idempotent")
		}
	})

	t.Run("legacy density migrates once", func(t *testing.T) {
		page := browser.MustPage(origin + "/dashboard")
		defer page.MustClose()
		page.MustSetDocumentContent(`<!doctype html><html><body class="pw-page"></body></html>`)
		page.MustEval(`() => { localStorage.clear(); localStorage.setItem('aurago.config.density.v1', 'compact'); }`)
		if err := page.AddScriptTag("", normalizeAssetText(mustReadUIFile(t, "js/precision/workspace.js"))); err != nil {
			t.Fatalf("load Precision Workspace client: %v", err)
		}
		if got := page.MustEval(`() => localStorage.getItem('aurago.workspace.density.v1') + ':' + document.body.dataset.density`).String(); got != "compact:compact" {
			t.Fatalf("legacy density migration = %q, want compact:compact", got)
		}
		page.MustEval(`() => localStorage.setItem('aurago.config.density.v1', 'comfortable')`)

		reload := browser.MustPage(origin + "/dashboard")
		defer reload.MustClose()
		loadPrecisionWorkspaceScript(t, reload, `<body class="pw-page"></body>`)
		if got := reload.MustEval(`() => document.body.dataset.density`).String(); got != "compact" {
			t.Fatalf("canonical density did not win after one-time migration: %q", got)
		}
	})

	t.Run("blocked storage remains safe", func(t *testing.T) {
		page := browser.MustPage(origin + "/knowledge")
		defer page.MustClose()
		page.MustSetDocumentContent(`<!doctype html><html><body class="pw-page"></body></html>`)
		page.MustEval(`() => {
			Storage.prototype.getItem = () => { throw new DOMException('blocked', 'SecurityError'); };
			Storage.prototype.setItem = () => { throw new DOMException('blocked', 'SecurityError'); };
		}`)
		if err := page.AddScriptTag("", normalizeAssetText(mustReadUIFile(t, "js/precision/workspace.js"))); err != nil {
			t.Fatalf("load Precision Workspace client with blocked storage: %v", err)
		}
		if got := page.MustEval(`() => window.AuraPrecisionWorkspace.setDensity('compact') + ':' + document.body.dataset.density`).String(); got != "compact:compact" {
			t.Fatalf("blocked-storage density state = %q, want compact:compact", got)
		}
	})
}

func TestPrecisionWorkspaceModalFocusBrowserSmoke(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	browser := newSmokeBrowser(t)
	origin := newPrecisionSmokeOrigin(t)

	t.Run("classic nested dialog traps and restores focus", func(t *testing.T) {
		page := browser.MustPage(origin + "/skills")
		defer page.MustClose()
		loadPrecisionWorkspaceScript(t, page, `<body class="pw-page"><button id="opener">Open</button></body>`)
		page.MustEval(`() => {
			document.getElementById('opener').focus();
			const overlay = document.createElement('div');
			overlay.id = 'classic';
			overlay.className = 'modal-overlay active';
			overlay.innerHTML = '<section id="nested-dialog" role="dialog"><h2 class="modal-title">Editor</h2><button id="first">First</button><button id="last">Last</button></section>';
			document.body.appendChild(overlay);
		}`)
		waitForJSBool(t, page, `() => document.activeElement && document.activeElement.id === 'first'`)
		semantics := page.MustEval(`() => ({
			overlayBound: document.getElementById('classic').dataset.pwModalBound,
			overlayRole: document.getElementById('classic').hasAttribute('role'),
			dialogModal: document.getElementById('nested-dialog').getAttribute('aria-modal'),
			labelledBy: document.getElementById('nested-dialog').getAttribute('aria-labelledby')
		})`).Map()
		if semantics["overlayBound"].String() != "true" || semantics["overlayRole"].Bool() || semantics["dialogModal"].String() != "true" || semantics["labelledBy"].String() == "" {
			t.Fatalf("classic nested dialog semantics mismatch: %#v", semantics)
		}
		page.MustEval(`() => {
			const last = document.getElementById('last');
			last.focus();
			last.dispatchEvent(new KeyboardEvent('keydown', {key: 'Tab', bubbles: true, cancelable: true}));
		}`)
		if got := page.MustEval(`() => document.activeElement.id`).String(); got != "first" {
			t.Fatalf("forward Tab wrapped to %q, want first", got)
		}
		page.MustEval(`() => document.getElementById('classic').classList.remove('active')`)
		waitForJSBool(t, page, `() => document.activeElement && document.activeElement.id === 'opener'`)
	})

	t.Run("standalone dialog traps and restores focus", func(t *testing.T) {
		page := browser.MustPage(origin + "/plans")
		defer page.MustClose()
		loadPrecisionWorkspaceScript(t, page, `<body class="pw-page"><button id="opener">Open</button></body>`)
		page.MustEval(`() => {
			document.getElementById('opener').focus();
			const dialog = document.createElement('div');
			dialog.id = 'standalone';
			dialog.setAttribute('role', 'dialog');
			dialog.setAttribute('aria-modal', 'true');
			dialog.setAttribute('aria-label', 'Standalone');
			dialog.innerHTML = '<button id="standalone-first">First</button><button id="standalone-last">Last</button>';
			document.body.appendChild(dialog);
		}`)
		waitForJSBool(t, page, `() => document.activeElement && document.activeElement.id === 'standalone-first'`)
		page.MustEval(`() => {
			const first = document.getElementById('standalone-first');
			first.focus();
			first.dispatchEvent(new KeyboardEvent('keydown', {key: 'Tab', shiftKey: true, bubbles: true, cancelable: true}));
		}`)
		if got := page.MustEval(`() => document.activeElement.id`).String(); got != "standalone-last" {
			t.Fatalf("reverse Tab wrapped to %q, want standalone-last", got)
		}
		page.MustEval(`() => { document.getElementById('standalone').hidden = true; }`)
		waitForJSBool(t, page, `() => document.activeElement && document.activeElement.id === 'opener'`)
	})

	t.Run("dynamically removed modal restores focus", func(t *testing.T) {
		page := browser.MustPage(origin + "/containers")
		defer page.MustClose()
		loadPrecisionWorkspaceScript(t, page, `<body class="pw-page"><button id="opener">Open</button></body>`)
		page.MustEval(`() => {
			document.getElementById('opener').focus();
			const overlay = document.createElement('div');
			overlay.id = 'dynamic';
			overlay.className = 'modal-overlay active';
			overlay.innerHTML = '<h2>Dynamic</h2><button id="dynamic-close">Close</button>';
			document.body.appendChild(overlay);
		}`)
		waitForJSBool(t, page, `() => document.activeElement && document.activeElement.id === 'dynamic-close'`)
		page.MustEval(`() => document.getElementById('dynamic').remove()`)
		waitForJSBool(t, page, `() => document.activeElement && document.activeElement.id === 'opener'`)
	})
}

func TestPrecisionSetupOpenRouterKeyboardBrowserSmoke(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	browser := newSmokeBrowser(t)
	origin := newPrecisionSmokeOrigin(t)
	page := browser.MustPage(origin + "/setup")
	defer page.MustClose()

	html := normalizeAssetText(mustReadUIFile(t, "setup.html"))
	html = strings.ReplaceAll(html, "{{.Lang}}", "en")
	html = strings.ReplaceAll(html, "{{.BuildVersion}}", "test")
	html = strings.ReplaceAll(html, "{{.TemplateDataJSON}}", "{}")
	html = regexp.MustCompile(`(?is)<script\s+src="[^"]+"[^>]*></script>`).ReplaceAllString(html, "")
	page.MustSetDocumentContent(html)
	page.MustEval(`() => {
		window.t = key => ({
			'setup.or_browser_title': 'Browse models',
			'setup.or_browser_search_placeholder': 'Search',
			'setup.or_browser_free_button_title': 'Free only',
			'setup.or_browser_free_button': 'Free',
			'setup.or_browser_loading': 'Loading',
			'setup.or_browser_model_count': 'models',
			'setup.or_browser_model_count_free_only': 'free only',
			'common.close': 'Close'
		}[key] || key);
		window.fetch = async input => {
			const url = String(input);
			let payload = {};
			if (url.includes('/api/setup/status')) payload = {needs_setup: true, csrf_token: 'fixture'};
			else if (url.includes('/api/personalities')) payload = {personalities: []};
			else if (url.includes('/api/security/status')) payload = {auth: {password_set: false}};
			else if (url.includes('/api/openrouter/models')) payload = {data: [{id: 'fixture/model', name: 'Fixture Model', context_length: 8192, pricing: {prompt: '0', completion: '0'}}]};
			else if (url.includes('/api/i18n/')) payload = {};
			else payload = {profiles: []};
			return {ok: true, status: 200, json: async () => payload, text: async () => JSON.stringify(payload)};
		};
	}`)
	if err := page.AddScriptTag("", normalizeAssetText(mustReadUIFile(t, "js/setup/main.js"))); err != nil {
		t.Fatalf("load setup client: %v", err)
	}

	page.MustEval(`() => {
		const opener = document.getElementById('or-browse-btn');
		opener.id = 'or-test-opener';
		opener.focus();
		window.__openRouterPromise = openOpenRouterBrowser(() => {});
	}`)
	waitForJSBool(t, page, `() => !!document.querySelector('#or-list .or-model-row') && document.activeElement && document.activeElement.id === 'or-search'`)
	page.MustEval(`() => {
		const last = document.getElementById('or-free-btn');
		last.focus();
		last.dispatchEvent(new KeyboardEvent('keydown', {key: 'Tab', bubbles: true, cancelable: true}));
	}`)
	if got := page.MustEval(`() => document.activeElement.id`).String(); got != "or-browser-close" {
		t.Fatalf("OpenRouter forward Tab wrapped to %q, want close button", got)
	}
	page.MustEval(`() => {
		const first = document.getElementById('or-browser-close');
		first.focus();
		first.dispatchEvent(new KeyboardEvent('keydown', {key: 'Tab', shiftKey: true, bubbles: true, cancelable: true}));
	}`)
	if got := page.MustEval(`() => document.activeElement.id`).String(); got != "or-free-btn" {
		t.Fatalf("OpenRouter reverse Tab wrapped to %q, want free button", got)
	}
	page.MustEval(`() => document.getElementById('or-browser-overlay').dispatchEvent(new KeyboardEvent('keydown', {key: 'Escape', bubbles: true, cancelable: true}))`)
	waitForJSBool(t, page, `() => !document.getElementById('or-browser-overlay') && document.activeElement && document.activeElement.id === 'or-test-opener'`)
}
