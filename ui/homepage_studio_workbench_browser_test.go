package ui

import "testing"

// homepageStudioSmokeSites is the deterministic site list of the smoke page.
const homepageStudioSmokeSites = `[
	{
		"id": 1,
		"name": "First",
		"project_dir": "first",
		"framework": "static",
		"drift_status": "not_deployed",
		"local_root": "/srv/first",
		"deploy_targets": [{ "provider": "vercel", "url": "https://deploy.vercel.test/app", "status": "active" }]
	},
	{
		"id": 2,
		"name": "Second",
		"project_dir": "second",
		"framework": "static",
		"drift_status": "clean",
		"local_root": "/srv/second",
		"deploy_targets": [{ "provider": "netlify", "url": "https://deploy.netlify.test/app", "status": "inactive" }]
	}
]`

// homepageStudioSmokeMount renders the workbench with a deterministic API so
// the browser checks exercise the production code.
const homepageStudioSmokeMount = `async sites => {
	const list = JSON.parse(sites);
	const container = document.getElementById('hp-host');
	container.innerHTML = '';
	window.__hpCalls = [];
	const api = async path => {
		const url = String(path);
		window.__hpCalls.push(url);
		if (url.indexOf('/api/desktop/integrations/homepage/status') === 0) {
			return {
				status: 'ok',
				mode: 'python_fallback',
				preview_url: window.location.origin + '/previewed',
				web_container: { running: false },
				python_server: { running: false }
			};
		}
		if (/^\/api\/desktop\/integrations\/homepage\/sites\/\d+$/.test(url)) {
			const id = Number(url.split('/').pop());
			return { site: list.find(item => Number(item.id) === id) || null };
		}
		if (url.indexOf('/api/desktop/integrations/homepage/sites') === 0) {
			return { sites: list };
		}
		if (url.indexOf('/api/desktop/integrations/integrations/webhosts') === 0) {
			return { webhosts: [{ id: 'homepage', url: window.location.origin + '/homepage', name: 'Homepage' }] };
		}
		if (url.indexOf('/api/desktop/integrations/homepage/history') === 0) {
			return { status: 'success', entries: [], total: 0 };
		}
		return {};
	};
	const esc = value => String(value == null ? '' : value).replace(/[&<>"']/g, ch => ({
		'&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;'
	}[ch]));
	window.HomepageStudioApp.render(container, 'test-window', {
		esc: esc,
		api: api,
		t: key => key,
		iconMarkup: () => '',
		notify: () => {},
		confirmDialog: async () => false,
		readonly: false
	});
	await new Promise(resolve => setTimeout(resolve, 900));
	return document.getElementById('hp-root-test-window') ? 'mounted' : 'missing';
}`

// TestHomepageStudioWorkbenchBrowser exercises the Homepage Studio workbench in
// a real browser: the inspector tablist, the keyboard focus of the site cards,
// the device switcher, the live region contract and the request budget.
func TestHomepageStudioWorkbenchBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	page := newSmokeBrowser(t).MustPage(newPrecisionSmokeOrigin(t))
	defer page.MustClose()

	page.MustSetViewport(1440, 900, 1, false)
	page.MustSetDocumentContent(`<!doctype html><html><head><meta charset="utf-8"></head><body><div id="hp-host"></div></body></html>`)

	for _, asset := range []string{
		"js/desktop/apps/homepage-studio-preview.js",
		"js/desktop/apps/homepage-studio-sites.js",
		"js/desktop/apps/homepage-studio-history.js",
		"js/desktop/apps/homepage-studio.js",
	} {
		if err := page.AddScriptTag("", readDesktopAssetText(t, asset)); err != nil {
			t.Fatalf("load %s: %v", asset, err)
		}
	}

	if mounted := page.MustEval(homepageStudioSmokeMount, homepageStudioSmokeSites).Str(); mounted != "mounted" {
		t.Fatalf("homepage studio workbench did not mount: %q", mounted)
	}

	t.Run("inspector tablist moves selection and focus", func(t *testing.T) {
		result := page.MustEval(`() => {
			const root = document.getElementById('hp-root-test-window');
			const sitesTab = root.querySelector('#hp-tab-sites-test-window');
			const historyTab = root.querySelector('#hp-tab-history-test-window');
			if (!sitesTab || !historyTab) return { missing: true };
			sitesTab.focus();
			const focusedBefore = document.activeElement === sitesTab;
			sitesTab.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowRight', bubbles: true }));
			return {
				focusedBefore: focusedBefore,
				sitesSelected: sitesTab.getAttribute('aria-selected'),
				historySelected: historyTab.getAttribute('aria-selected'),
				sitesTabindex: sitesTab.getAttribute('tabindex'),
				historyTabindex: historyTab.getAttribute('tabindex'),
				focusedHistory: document.activeElement === historyTab
			};
		}`)
		if result.Get("missing").Bool() {
			t.Fatal("the inspector tabs are missing")
		}
		if !result.Get("focusedBefore").Bool() {
			t.Fatal("the sites tab must be focusable")
		}
		if result.Get("historySelected").Str() != "true" || result.Get("sitesSelected").Str() != "false" {
			t.Fatalf("arrow key did not move the tab selection: %v", result.JSON("", ""))
		}
		if result.Get("historyTabindex").Str() != "0" || result.Get("sitesTabindex").Str() != "-1" {
			t.Fatalf("arrow key did not move the tab stop: %v", result.JSON("", ""))
		}
		if !result.Get("focusedHistory").Bool() {
			t.Fatalf("arrow key did not move the focus: %v", result.JSON("", ""))
		}
	})

	t.Run("site cards keep the keyboard focus", func(t *testing.T) {
		result := page.MustEval(`async () => {
			const root = document.getElementById('hp-root-test-window');
			const sitesTab = root.querySelector('#hp-tab-sites-test-window');
			if (sitesTab) sitesTab.click();
			const list = document.getElementById('hp-sites-list-test-window');
			const cards = list ? list.querySelectorAll('.vd-hp-site') : [];
			const card = cards[1];
			if (!card) return { cards: cards.length };
			card.focus();
			const focusedBefore = document.activeElement === card;
			card.click();
			await new Promise(resolve => setTimeout(resolve, 500));
			const active = document.activeElement;
			return {
				cards: cards.length,
				focusedBefore: focusedBefore,
				activeSiteId: active ? active.getAttribute('data-site-id') : null,
				activeIsCard: !!(active && active.classList.contains('vd-hp-site')),
				activeSelected: active ? active.getAttribute('aria-selected') : null
			};
		}`)
		if result.Get("cards").Int() < 2 {
			t.Fatalf("the smoke page needs at least two site cards: %v", result.JSON("", ""))
		}
		if !result.Get("focusedBefore").Bool() {
			t.Fatal("a site card must be focusable")
		}
		if !result.Get("activeIsCard").Bool() {
			t.Fatalf("the focus left the site card after the list re-rendered: %v", result.JSON("", ""))
		}
		if result.Get("activeSiteId").Str() != "2" {
			t.Fatalf("the focus moved to another site card: %v", result.JSON("", ""))
		}
		if result.Get("activeSelected").Str() != "true" {
			t.Fatalf("the focused card must be the selected one: %v", result.JSON("", ""))
		}
	})

	t.Run("device switcher produces three preview widths", func(t *testing.T) {
		result := page.MustEval(`() => {
			const root = document.getElementById('hp-root-test-window');
			const panel = root.querySelector('.vd-hp-preview-panel');
			if (!panel) return { missing: true };
			panel.style.width = '600px';
			panel.style.height = '400px';
			const widths = [];
			for (const device of ['desktop', 'tablet', 'mobile']) {
				const button = root.querySelector('[data-hp-device="' + device + '"]');
				if (!button) return { missing: true };
				button.click();
				widths.push(root.querySelector('.vd-hp-preview-frame').style.getPropertyValue('--hp-device-w'));
			}
			return { widths: widths, distinct: new Set(widths).size };
		}`)
		if result.Get("missing").Bool() {
			t.Fatal("the device switcher is missing")
		}
		widths := result.Get("widths").Arr()
		if len(widths) != 3 {
			t.Fatalf("the device switcher must produce three preview widths: %v", result.JSON("", ""))
		}
		want := []string{"1280px", "768px", "390px"}
		seen := map[string]bool{}
		for index, width := range widths {
			if width.Str() != want[index] {
				t.Fatalf("device %d rendered %q instead of %q", index, width.Str(), want[index])
			}
			seen[width.Str()] = true
		}
		if len(seen) != 3 {
			t.Fatalf("the three devices must produce three distinct preview widths: %v", result.JSON("", ""))
		}
	})

	t.Run("workbench keeps the live region contract", func(t *testing.T) {
		result := page.MustEval(`() => {
			const root = document.getElementById('hp-root-test-window');
			const attr = (element, name) => {
				if (!element) return 'missing';
				return element.hasAttribute(name) ? element.getAttribute(name) : 'none';
			};
			const chatLog = root.querySelector('.vd-hp-chat-log');
			const announce = root.querySelector('#hp-announce-test-window');
			const iframe = root.querySelector('.vd-hp-preview-iframe');
			const splitter = root.querySelector('[data-hp-splitter="chat"]');
			const inspectorSplitter = root.querySelector('[data-hp-splitter="inspector"]');
			const sitesList = root.querySelector('#hp-sites-list-test-window');
			const card = root.querySelector('.vd-hp-site');
			const deviceGroup = root.querySelector('.vd-hp-device-seg');
			return {
				chatLive: attr(chatLog, 'aria-live'),
				chatBusy: attr(chatLog, 'aria-busy'),
				announceRole: attr(announce, 'role'),
				iframeTitle: iframe ? iframe.title : 'missing',
				splitterValue: attr(splitter, 'aria-valuenow'),
				inspectorSplitterValue: attr(inspectorSplitter, 'aria-valuenow'),
				sitesRole: attr(sitesList, 'role'),
				cardRole: attr(card, 'role'),
				deviceRole: attr(deviceGroup, 'role')
			};
		}`)
		if result.Get("chatLive").Str() != "none" {
			t.Fatalf("the chat log must not be an aria-live region: %v", result.JSON("", ""))
		}
		if result.Get("chatBusy").Str() != "false" {
			t.Fatalf("the chat log must expose its busy state: %v", result.JSON("", ""))
		}
		if result.Get("announceRole").Str() != "status" {
			t.Fatalf("finished messages need a dedicated status node: %v", result.JSON("", ""))
		}
		if result.Get("iframeTitle").Str() == "" {
			t.Fatalf("the preview iframe needs an accessible name: %v", result.JSON("", ""))
		}
		if result.Get("splitterValue").Str() == "none" || result.Get("splitterValue").Str() == "missing" ||
			result.Get("inspectorSplitterValue").Str() == "none" || result.Get("inspectorSplitterValue").Str() == "missing" {
			t.Fatalf("the panel splitters must expose their current width: %v", result.JSON("", ""))
		}
		if result.Get("sitesRole").Str() != "listbox" {
			t.Fatalf("the site list must be a listbox: %v", result.JSON("", ""))
		}
		if result.Get("cardRole").Str() != "option" {
			t.Fatalf("the site cards must be listbox options: %v", result.JSON("", ""))
		}
		if result.Get("deviceRole").Str() != "radiogroup" {
			t.Fatalf("the device switcher must be a radiogroup: %v", result.JSON("", ""))
		}
	})

	t.Run("the workbench stays within the request budget", func(t *testing.T) {
		result := page.MustEval(`() => {
			const calls = window.__hpCalls || [];
			return {
				list: calls.filter(path => path === '/api/desktop/integrations/homepage/sites').length,
				detail: calls.filter(path => /^\/api\/desktop\/integrations\/homepage\/sites\/\d+$/.test(path)).length,
				total: calls.length
			};
		}`)
		if list := result.Get("list").Int(); list != 1 {
			t.Fatalf("the site list must be fetched once, got %d: %v", list, result.JSON("", ""))
		}
		if detail := result.Get("detail").Int(); detail != 1 {
			t.Fatalf("only the selected site may be fetched, got %d: %v", detail, result.JSON("", ""))
		}
	})
}
