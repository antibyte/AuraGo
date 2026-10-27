package ui

import (
	"bytes"
	"image/png"
	"strings"
	"testing"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
)

// homepageStudioAnnotateProbe reports where the focus is and which parts of the
// workbench are open, so the keyboard checks can compare steps.
const homepageStudioAnnotateProbe = `() => {
	const root = document.getElementById('hp-root-test-window');
	const layer = root.querySelector('.vd-hp-annotate-layer');
	const bar = root.querySelector('.vd-hp-annotate-bar');
	const popover = root.querySelector('.vd-hp-mark-popover');
	const tray = root.querySelector('.vd-hp-mark-tray');
	const active = document.activeElement;
	let where = 'other:' + (active ? active.tagName + '.' + active.className + '#' + active.id : 'none');
	if (!active || active === document.body) where = 'body';
	else if (active === layer) where = 'layer';
	else if (bar.contains(active)) where = 'bar:' + (active.textContent || active.tagName).trim();
	else if (popover.contains(active)) where = 'popover';
	else if (tray.contains(active)) where = 'chip:' + (active.dataset.markN || 'x');
	else if (active.id === 'hp-input-test-window') where = 'composer';
	else if (active.hasAttribute('data-hp-annotate')) where = 'annotate-button';
	return {
		where: where,
		layerHidden: layer.hidden,
		popoverHidden: popover.hidden,
		annotating: root.querySelector('.vd-hp-stage').classList.contains('is-annotating'),
		inspectorCollapsed: root.classList.contains('inspector-collapsed'),
		chatCollapsed: root.classList.contains('chat-collapsed'),
		chips: tray.querySelectorAll('.vd-hp-mark-chip').length,
		escapes: window.__hpEscapes || 0
	};
}`

type homepageStudioAnnotateState struct {
	Where                                  string
	LayerHidden, PopoverHidden, Annotating bool
	InspectorCollapsed, ChatCollapsed      bool
	Chips, Escapes                         int
}

func readHomepageStudioAnnotateState(page *rod.Page) homepageStudioAnnotateState {
	result := page.MustEval(homepageStudioAnnotateProbe)
	return homepageStudioAnnotateState{
		Where:              result.Get("where").Str(),
		LayerHidden:        result.Get("layerHidden").Bool(),
		PopoverHidden:      result.Get("popoverHidden").Bool(),
		Annotating:         result.Get("annotating").Bool(),
		InspectorCollapsed: result.Get("inspectorCollapsed").Bool(),
		ChatCollapsed:      result.Get("chatCollapsed").Bool(),
		Chips:              result.Get("chips").Int(),
		Escapes:            result.Get("escapes").Int(),
	}
}

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
		if (url.indexOf('/api/desktop/integrations/homepage/annotation-snapshot') === 0) {
			window.__hpSnapshotCalls = (window.__hpSnapshotCalls || 0) + 1;
			return window.__hpSnapshot;
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
		"js/desktop/apps/homepage-studio-annotate.js",
		"js/desktop/apps/homepage-studio.js",
	} {
		if err := page.AddScriptTag("", readDesktopAssetText(t, asset)); err != nil {
			t.Fatalf("load %s: %v", asset, err)
		}
	}

	// The real stylesheets, so layout, hidden iframes and focus rings are the
	// ones users get. Only the accent token is supplied by the fixture.
	for _, name := range []string{"css/desktop-app-homepage-studio.css", "css/desktop-app-homepage-studio-annotate.css"} {
		if err := page.AddStyleTag("", normalizeAssetText(mustReadUIFile(t, name))); err != nil {
			t.Fatalf("load %s: %v", name, err)
		}
	}
	if err := page.AddStyleTag("", `:root { --vd-accent: rgb(255, 0, 170); --vd-text: #111; }
#hp-host { width: 1200px; height: 700px; }`); err != nil {
		t.Fatalf("load the fixture style: %v", err)
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

	t.Run("annotate marks an element and an area and sends them with the next message", func(t *testing.T) {
		prepared := page.MustEval(`() => {
			const surface = document.createElement('canvas');
			surface.width = 1280;
			surface.height = 1600;
			const g = surface.getContext('2d');
			g.fillStyle = '#ffffff';
			g.fillRect(0, 0, 1280, 1600);
			g.fillStyle = '#0a7';
			g.fillRect(100, 200, 200, 50);
			window.__hpSnapshot = {
				url: 'http://127.0.0.1:8080/', path: '/', viewport: { width: 1280, height: 800 },
				page: { width: 1280, height: 1600, truncated: false },
				image: surface.toDataURL('image/jpeg', 0.8),
				elements: [
					{ i: 0, selector: 'body', tag: 'body', box: [0, 0, 1280, 1600], parent: -1 },
					{ i: 1, selector: '#cta', tag: 'a', text: 'Get started', attrs: { href: '/signup' }, box: [100, 200, 200, 50], parent: 0, tab: true },
					{ i: 2, selector: 'section.features', tag: 'section', text: 'Features', box: [0, 400, 1280, 600], parent: 0, tab: true },
					{ i: 3, selector: 'section.features > div:nth-of-type(1)', tag: 'div', text: 'One', box: [40, 450, 300, 200], parent: 2 }
				]
			};
			const button = document.querySelector('#hp-root-test-window [data-hp-annotate]');
			if (!button) return 'missing button';
			if (button.disabled) return 'button disabled';
			button.click();
			return 'clicked';
		}`).Str()
		if prepared != "clicked" {
			t.Fatalf("annotate button: %s", prepared)
		}
		page.MustWait(`() => { const layer = document.querySelector('#hp-root-test-window .vd-hp-annotate-layer'); return !!layer && !layer.hidden; }`)

		result := page.MustEval(`async () => {
			const root = document.getElementById('hp-root-test-window');
			const layer = root.querySelector('.vd-hp-annotate-layer');
			const point = (x, y) => {
				const rect = layer.getBoundingClientRect();
				const scale = rect.width / layer.offsetWidth;
				return { clientX: rect.left + (x - layer.scrollLeft) * scale, clientY: rect.top + (y - layer.scrollTop) * scale };
			};
			const pointer = (type, x, y) => layer.dispatchEvent(new PointerEvent(type, Object.assign({ bubbles: true, button: 0, pointerId: 7, isPrimary: true }, point(x, y))));
			const popover = root.querySelector('.vd-hp-mark-popover');
			const input = popover.querySelector('textarea');

			pointer('pointerdown', 150, 220);
			pointer('pointerup', 150, 220);
			if (popover.hidden) return { error: 'element popover did not open' };
			input.value = 'Make it green';
			input.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }));

			layer.scrollTop = 0;
			pointer('pointerdown', 30, 430);
			pointer('pointermove', 360, 680);
			pointer('pointerup', 360, 680);
			if (popover.hidden) return { error: 'area popover did not open' };
			input.value = 'Add an image';
			popover.querySelector('.vd-hp-annotate-action.is-primary').click();

			layer.focus();
			layer.dispatchEvent(new KeyboardEvent('keydown', { key: 'Tab', bubbles: true }));
			const hoverShown = !root.querySelector('.vd-hp-annotate-hover').hidden;
			layer.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }));
			const keyboardPopover = !popover.hidden;
			input.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
			const chips = root.querySelectorAll('.vd-hp-mark-chip').length;

			let body = null;
			const realFetch = window.fetch;
			window.fetch = async (url, options) => {
				if (String(url).indexOf('/api/desktop/chat/stream') === 0) {
					body = JSON.parse(options.body);
					return new Response('data: {"event":"done"}\n\n', { status: 200, headers: { 'Content-Type': 'text/event-stream' } });
				}
				return realFetch(url, options);
			};
			root.querySelector('#hp-input-test-window').value = '';
			root.querySelector('#hp-form-test-window').dispatchEvent(new Event('submit', { cancelable: true }));
			await new Promise(resolve => setTimeout(resolve, 300));
			window.fetch = realFetch;

			return {
				hoverShown, keyboardPopover, chips,
				trayHidden: root.querySelector('.vd-hp-mark-tray').hidden,
				layerHidden: layer.hidden,
				thumb: !!root.querySelector('.vd-chat-bubble.user .vd-hp-mark-thumb'),
				message: body ? body.message : '',
				items: body && body.context.annotations ? body.context.annotations.items : [],
				hasImage: !!(body && body.context.image_base64 && body.context.image_base64.length > 100)
			};
		}`)
		if result.Has("error") {
			t.Fatal(result.Get("error").Str())
		}
		if !result.Get("hoverShown").Bool() || !result.Get("keyboardPopover").Bool() {
			t.Fatalf("keyboard path failed: %s", result.JSON("", ""))
		}
		if result.Get("chips").Int() != 2 {
			t.Fatalf("chips = %d, want 2", result.Get("chips").Int())
		}
		if !result.Get("trayHidden").Bool() || !result.Get("layerHidden").Bool() || !result.Get("thumb").Bool() || !result.Get("hasImage").Bool() {
			t.Fatalf("send did not hand off the marks: %s", result.JSON("", ""))
		}
		message := result.Get("message").Str()
		for _, want := range []string{"homepage_studio.annotate_default_message", "1. Make it green", "2. Add an image"} {
			if !strings.Contains(message, want) {
				t.Fatalf("message %q misses %q", message, want)
			}
		}
		items := result.Get("items").Arr()
		if len(items) != 2 || items[0].Get("selector").Str() != "#cta" || items[1].Get("kind").Str() != "area" {
			t.Fatalf("annotations = %s", result.Get("items").JSON("", ""))
		}
		if overlaps := items[1].Get("elements").Arr(); len(overlaps) != 1 || overlaps[0].Get("selector").Str() != "section.features > div:nth-of-type(1)" {
			t.Fatalf("area overlaps = %s", items[1].Get("elements").JSON("", ""))
		}
	})

	t.Run("annotate keeps Escape, Tab and focus inside the mode", func(t *testing.T) {
		// Real key presses, so Tab really moves the focus and Escape really
		// bubbles to the studio's own handler (which would collapse a panel).
		press := func(keys ...input.Key) {
			for _, key := range keys {
				page.Keyboard.MustType(key)
			}
		}
		focus := func(script string) {
			page.MustEval(`() => { const root = document.getElementById('hp-root-test-window'); ` + script + ` }`)
		}
		const cancelButton = `[...root.querySelectorAll('.vd-hp-mark-popover .vd-hp-annotate-action')].find(b => b.textContent === 'homepage_studio.annotate_cancel')`

		page.MustEval(`() => {
			const root = document.getElementById('hp-root-test-window');
			window.__hpEscapes = 0;
			window.__hpKeys = [];
			root.addEventListener('keydown', event => {
				if (event.key === 'Escape') window.__hpEscapes++;
				if (event.key === 'Tab') window.__hpKeys.push({ shift: event.shiftKey, prevented: event.defaultPrevented });
			});
			root.querySelector('[data-hp-annotate]').click();
		}`)
		page.MustWait(`() => { const layer = document.querySelector('#hp-root-test-window .vd-hp-annotate-layer'); return !!layer && !layer.hidden; }`)
		start := readHomepageStudioAnnotateState(page)
		if start.Where != "layer" || start.InspectorCollapsed || start.ChatCollapsed {
			t.Fatalf("annotate mode must start on the layer with the panels open: %+v", start)
		}
		stays := func(step string, got homepageStudioAnnotateState) {
			t.Helper()
			if got.InspectorCollapsed || got.ChatCollapsed || got.Escapes != 0 {
				t.Fatalf("%s: Escape leaked to the studio and collapsed a panel: %+v", step, got)
			}
		}

		// Two keyboard marks give the tray something to delete.
		press(input.Tab, input.Enter)
		page.MustInsertText("Make it pop")
		press(input.Enter)
		press(input.Tab, input.Enter)
		page.MustInsertText("Tighten the spacing")
		press(input.Enter)
		if got := readHomepageStudioAnnotateState(page); got.Chips != 2 || got.Where != "layer" || !got.PopoverHidden {
			t.Fatalf("two keyboard marks expected: %+v", got)
		}

		// Escape on a popover button closes the popover and keeps the mode.
		press(input.Enter)
		focus(`const cancel = ` + cancelButton + `; if (cancel) cancel.focus();`)
		if got := readHomepageStudioAnnotateState(page); got.Where != "popover" || got.PopoverHidden {
			t.Fatalf("the popover must be open with the focus on Cancel: %+v", got)
		}
		press(input.Escape)
		got := readHomepageStudioAnnotateState(page)
		stays("Escape on Cancel", got)
		if !got.PopoverHidden || got.LayerHidden || !got.Annotating || got.Where != "layer" || got.Chips != 2 {
			t.Fatalf("Escape on Cancel must close only the popover and return to the layer: %+v", got)
		}

		// Escape on the layer while the popover is open closes the popover.
		press(input.Enter)
		focus(`root.querySelector('.vd-hp-annotate-layer').focus();`)
		if got := readHomepageStudioAnnotateState(page); got.PopoverHidden || got.Where != "layer" {
			t.Fatalf("the popover must stay open while the layer has the focus: %+v", got)
		}
		press(input.Escape)
		got = readHomepageStudioAnnotateState(page)
		stays("Escape on the layer with the popover open", got)
		if !got.PopoverHidden || !got.Annotating || got.Where != "layer" {
			t.Fatalf("Escape on the layer must close the popover first: %+v", got)
		}

		// Tab from the last element leaves the layer for Path, Recapture, Done.
		focus(`window.__hpKeys = [];`)
		press(input.Tab)
		if got := readHomepageStudioAnnotateState(page); got.Where != "bar:INPUT" {
			t.Fatalf("Tab from the last element must move to the Path field, not wrap: %+v", got)
		}
		keys := page.MustEval(`() => window.__hpKeys`).Arr()
		if len(keys) == 0 || keys[0].Get("prevented").Bool() {
			t.Fatalf("Tab past the last element must not be prevented: %s", page.MustEval(`() => window.__hpKeys`).JSON("", ""))
		}
		press(input.Tab)
		if got := readHomepageStudioAnnotateState(page); got.Where != "bar:homepage_studio.annotate_recapture" {
			t.Fatalf("Tab must reach Recapture: %+v", got)
		}
		press(input.Tab)
		if got := readHomepageStudioAnnotateState(page); got.Where != "bar:homepage_studio.annotate_done" {
			t.Fatalf("Tab must reach Done: %+v", got)
		}

		// Escape on the bar's Done leaves the mode without collapsing a panel.
		press(input.Escape)
		got = readHomepageStudioAnnotateState(page)
		stays("Escape on Done", got)
		if !got.LayerHidden || got.Annotating || got.Where != "annotate-button" || got.Chips != 2 {
			t.Fatalf("Escape on Done must leave annotate mode and focus its button: %+v", got)
		}

		// Shift+Tab on a fresh walk also leaves instead of wrapping.
		focus(`root.querySelector('[data-hp-annotate]').click();`)
		if got := readHomepageStudioAnnotateState(page); got.Where != "layer" || !got.Annotating {
			t.Fatalf("annotate mode must resume on the layer: %+v", got)
		}
		if err := page.Keyboard.Press(input.ShiftLeft); err != nil {
			t.Fatalf("press Shift: %v", err)
		}
		press(input.Tab)
		if err := page.Keyboard.Release(input.ShiftLeft); err != nil {
			t.Fatalf("release Shift: %v", err)
		}
		if got := readHomepageStudioAnnotateState(page); got.Where == "layer" || got.Where == "body" {
			t.Fatalf("Shift+Tab before the first element must leave the layer: %+v", got)
		}
		focus(`root.querySelector('.vd-hp-annotate-layer').focus();`)
		press(input.Escape)
		got = readHomepageStudioAnnotateState(page)
		stays("Escape on the layer", got)
		if !got.LayerHidden || got.Annotating {
			t.Fatalf("Escape on the layer must leave annotate mode: %+v", got)
		}

		// Deleting a chip hands the focus on and announces the removal.
		focus(`root.querySelector('.vd-hp-mark-chip-open').focus();`)
		press(input.Delete)
		got = readHomepageStudioAnnotateState(page)
		if got.Chips != 1 || !strings.HasPrefix(got.Where, "chip:") {
			t.Fatalf("Delete on a chip must leave the focus on the next chip: %+v", got)
		}
		if announced := page.MustEval(`() => document.getElementById('hp-announce-test-window').textContent`).Str(); announced != "homepage_studio.annotate_mark_removed" {
			t.Fatalf("the removal must be announced, got %q", announced)
		}
		press(input.Delete)
		got = readHomepageStudioAnnotateState(page)
		if got.Chips != 0 || got.Where != "composer" {
			t.Fatalf("Delete on the last chip must leave the focus in the composer: %+v", got)
		}
	})

	t.Run("annotate counts a small drift as a click at any device scale", func(t *testing.T) {
		result := page.MustEval(`async () => {
			const root = document.getElementById('hp-root-test-window');
			const frame = root.querySelector('.vd-hp-preview-frame');
			const layer = root.querySelector('.vd-hp-annotate-layer');
			const popover = root.querySelector('.vd-hp-mark-popover');
			const input = popover.querySelector('textarea');
			const wait = ms => new Promise(resolve => setTimeout(resolve, ms));
			frame.style.transformOrigin = 'top left';
			frame.style.transform = 'scale(0.45)';
			root.querySelector('[data-hp-annotate]').click();
			for (let i = 0; i < 40 && layer.hidden; i++) await wait(50);
			if (layer.hidden) return { error: 'annotate mode did not open' };
			const rect = layer.getBoundingClientRect();
			const scale = rect.width / layer.offsetWidth;
			const client = (x, y) => ({ clientX: rect.left + x * scale, clientY: rect.top + y * scale });
			const pointer = (type, at, dx, dy) => layer.dispatchEvent(new PointerEvent(type, Object.assign({ bubbles: true, button: 0, pointerId: 9, isPrimary: true }, { clientX: at.clientX + dx, clientY: at.clientY + dy })));
			const save = text => { input.value = text; input.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true })); };

			// 3 px of hand drift at scale 0.45 is 6.7 page pixels: still a click.
			const at = client(150, 220);
			pointer('pointerdown', at, 0, 0);
			pointer('pointerup', at, 3, 3);
			const clickOpened = !popover.hidden;
			save('Click mark');
			// 14 px is a real drag.
			const from = client(40, 460);
			pointer('pointerdown', from, 0, 0);
			pointer('pointermove', from, 14, 14);
			pointer('pointerup', from, 14, 14);
			const dragOpened = !popover.hidden;
			save('Area mark');

			const kinds = [...root.querySelectorAll('.vd-hp-mark-num')].map(el => el.classList.contains('is-area') ? 'area' : 'element');
			for (let i = 0; i < 4; i++) { const remove = root.querySelector('[data-mark-remove]'); if (remove) remove.click(); }
			frame.style.transform = '';
			frame.style.transformOrigin = '';
			root.querySelector('[data-hp-annotate]').click();
			return { scale: scale, clickOpened, dragOpened, kinds, left: root.querySelectorAll('.vd-hp-mark-chip').length };
		}`)
		if result.Has("error") {
			t.Fatal(result.Get("error").Str())
		}
		if scale := result.Get("scale").Num(); scale < 0.4 || scale > 0.5 {
			t.Fatalf("the preview frame must be scaled for this check: %s", result.JSON("", ""))
		}
		if !result.Get("clickOpened").Bool() || !result.Get("dragOpened").Bool() {
			t.Fatalf("both gestures must open the mark popover: %s", result.JSON("", ""))
		}
		kinds := result.Get("kinds").Arr()
		if len(kinds) != 2 || kinds[0].Str() != "element" || kinds[1].Str() != "area" {
			t.Fatalf("a 3 px drift must stay a click and a 14 px drag an area, got %s", result.Get("kinds").JSON("", ""))
		}
		if result.Get("left").Int() != 0 {
			t.Fatalf("the check must leave no marks behind: %s", result.JSON("", ""))
		}
	})

	t.Run("annotate drops a capture whose page source changes", func(t *testing.T) {
		result := page.MustEval(`async () => {
			const wait = ms => new Promise(resolve => setTimeout(resolve, ms));
			const host = document.createElement('div');
			host.innerHTML = '<div class="vd-hp-viewport-toolbar"></div><div class="vd-hp-preview-panel" style="position:relative;width:600px;height:400px"><div class="vd-hp-preview-frame"></div></div><form><textarea></textarea></form>';
			document.body.appendChild(host);
			const ctx = { siteId: 1, target: 'vercel', device: 'desktop', previewUrl: 'https://one.test/' };
			let gate = null;
			let release = () => {};
			const hold = () => { gate = new Promise(resolve => { release = resolve; }); };
			const snapshotA = window.__hpSnapshot;
			const other = document.createElement('canvas');
			other.width = 1280; other.height = 1600;
			const g = other.getContext('2d');
			g.fillStyle = '#c33'; g.fillRect(0, 0, 1280, 1600);
			const snapshotB = Object.assign({}, snapshotA, { image: other.toDataURL('image/jpeg', 0.8) });
			let reply = snapshotA;
			const mod = window.HomepageStudioAnnotate.create({
				root: host, stage: document.createElement('div'), chatForm: host.querySelector('form'), windowId: 'mini',
				t: key => key, iconMarkup: () => '', notify: () => {}, announce: () => {},
				confirmDialog: async () => true, readonly: false, isDisposed: () => false,
				api: async () => { if (gate) await gate; return reply; },
				context: () => ctx
			});
			const button = host.querySelector('[data-hp-annotate]');
			const layer = host.querySelector('.vd-hp-annotate-layer');
			const bar = host.querySelector('.vd-hp-annotate-bar');
			const shot = host.querySelector('.vd-hp-annotate-shot');
			const out = {};
			mod.setAvailable(true);

			// A target switch during the very first capture.
			hold();
			button.click();
			await wait(30);
			out.capturing = !bar.hidden && layer.hidden;
			ctx.target = 'netlify';
			mod.contextChanged();
			out.barClosedByTarget = bar.hidden;
			release();
			await wait(200);
			out.staleTargetShown = !layer.hidden;

			// A preview-URL change during the very first capture.
			ctx.target = 'vercel';
			hold();
			button.click();
			await wait(30);
			ctx.previewUrl = 'https://two.test/';
			mod.setAvailable(true);
			out.barClosedByUrl = bar.hidden;
			release();
			await wait(200);
			out.staleUrlShown = !layer.hidden;

			// Leaving while a recapture decodes keeps the previous pairing.
			gate = null;
			button.click();
			for (let i = 0; i < 40 && layer.hidden; i++) await wait(50);
			const srcA = shot.getAttribute('src');
			reply = snapshotB;
			hold();
			bar.querySelectorAll('button')[0].click();
			await wait(30);
			release();
			for (let i = 0; i < 12; i++) await Promise.resolve();
			bar.querySelectorAll('button')[1].click();
			await wait(200);
			out.firstImage = !!srcA && srcA === snapshotA.image;
			out.keptPreviousImage = shot.getAttribute('src') === srcA;
			out.leftModeOpen = !layer.hidden;

			mod.dispose();
			host.remove();
			return out;
		}`)
		if !result.Get("capturing").Bool() {
			t.Fatalf("the first capture must be in flight: %s", result.JSON("", ""))
		}
		if !result.Get("barClosedByTarget").Bool() || result.Get("staleTargetShown").Bool() {
			t.Fatalf("a target switch during the first capture must drop it: %s", result.JSON("", ""))
		}
		if !result.Get("barClosedByUrl").Bool() || result.Get("staleUrlShown").Bool() {
			t.Fatalf("a preview-URL change during the first capture must drop it: %s", result.JSON("", ""))
		}
		if !result.Get("firstImage").Bool() || !result.Get("keptPreviousImage").Bool() || result.Get("leftModeOpen").Bool() {
			t.Fatalf("an aborted recapture must keep the previous image with its element map: %s", result.JSON("", ""))
		}
	})

	t.Run("annotate chrome stays visible: focus ring and a tray that cannot push the composer out", func(t *testing.T) {
		page.MustEval(`() => {
			const root = document.getElementById('hp-root-test-window');
			root.querySelector('[data-hp-annotate]').click();
		}`)
		page.MustWait(`() => { const layer = document.querySelector('#hp-root-test-window .vd-hp-annotate-layer'); return !!layer && !layer.hidden; }`)

		sample := func() (int, int, int) {
			data := page.MustElement("#hp-root-test-window .vd-hp-preview-panel").MustScreenshot()
			img, err := png.Decode(bytes.NewReader(data))
			if err != nil {
				t.Fatalf("decode the panel screenshot: %v", err)
			}
			r, g, b, _ := img.At(2, img.Bounds().Dy()/2).RGBA()
			return int(r >> 8), int(g >> 8), int(b >> 8)
		}
		isAccent := func(r, g, b int) bool { return r == 255 && g == 0 && b == 170 }

		page.MustEval(`() => document.querySelector('[data-hp-annotate]').focus()`)
		if r, g, b := sample(); isAccent(r, g, b) {
			t.Fatalf("the panel edge must not show the ring while the layer is unfocused: %d,%d,%d", r, g, b)
		}
		page.Keyboard.MustType(input.ArrowDown)
		page.MustEval(`() => document.querySelector('#hp-root-test-window .vd-hp-annotate-layer').focus()`)
		if !page.MustEval(`() => document.querySelector('#hp-root-test-window .vd-hp-annotate-layer').matches(':focus-visible')`).Bool() {
			t.Fatal("the focused layer must match :focus-visible after keyboard use")
		}
		if r, g, b := sample(); !isAccent(r, g, b) {
			t.Fatalf("the focused layer must paint a visible accent ring on top of the snapshot, edge pixel is %d,%d,%d", r, g, b)
		}

		// A long tray scrolls inside its own band instead of pushing the composer out.
		result := page.MustEval(`async () => {
			const root = document.getElementById('hp-root-test-window');
			const layer = root.querySelector('.vd-hp-annotate-layer');
			const popover = root.querySelector('.vd-hp-mark-popover');
			const field = popover.querySelector('textarea');
			const rect = layer.getBoundingClientRect();
			const scale = rect.width / layer.offsetWidth;
			const at = { clientX: rect.left + 150 * scale, clientY: rect.top + 220 * scale };
			for (let i = 0; i < 16; i++) {
				layer.dispatchEvent(new PointerEvent('pointerdown', Object.assign({ bubbles: true, button: 0, pointerId: 3, isPrimary: true }, at)));
				layer.dispatchEvent(new PointerEvent('pointerup', Object.assign({ bubbles: true, button: 0, pointerId: 3, isPrimary: true }, at)));
				field.value = 'A fairly long comment for mark number ' + i;
				field.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }));
			}
			await new Promise(resolve => setTimeout(resolve, 50));
			const chat = root.querySelector('.vd-hp-chat').getBoundingClientRect();
			const tray = root.querySelector('.vd-hp-mark-tray');
			const form = root.querySelector('#hp-form-test-window').getBoundingClientRect();
			const input = root.querySelector('#hp-input-test-window').getBoundingClientRect();
			const send = root.querySelector('#hp-send-test-window').getBoundingClientRect();
			const out = {
				chips: root.querySelectorAll('.vd-hp-mark-chip').length,
				trayScrolls: tray.scrollHeight > tray.clientHeight,
				trayShare: tray.getBoundingClientRect().height / chat.height,
				composerVisible: form.bottom <= chat.bottom + 1 && input.top >= chat.top && input.bottom <= chat.bottom + 1 && send.bottom <= chat.bottom + 1
			};
			for (let i = 0; i < 20; i++) { const remove = root.querySelector('[data-mark-remove]'); if (remove) remove.click(); }
			root.querySelector('[data-hp-annotate]').click();
			return out;
		}`)
		if result.Get("chips").Int() != 16 {
			t.Fatalf("the tray check needs 16 marks: %s", result.JSON("", ""))
		}
		if !result.Get("composerVisible").Bool() {
			t.Fatalf("a long mark tray pushed the composer out of view: %s", result.JSON("", ""))
		}
		if !result.Get("trayScrolls").Bool() || result.Get("trayShare").Num() > 0.31 {
			t.Fatalf("a long mark tray must scroll within about 30%% of the column: %s", result.JSON("", ""))
		}
	})

	t.Run("preview chrome leaves the previewed page uncovered", func(t *testing.T) {
		// The URL pill, the toolbar and the site pill sit next to the preview
		// panel, never on top of it: the top of the page (logo, navigation) must
		// stay visible and, in mark mode, markable with the mouse.
		result := page.MustEval(`async () => {
			const root = document.getElementById('hp-root-test-window');
			const wait = ms => new Promise(resolve => setTimeout(resolve, ms));
			const zone = root.querySelector('.vd-hp-preview-zone');
			const panel = root.querySelector('.vd-hp-preview-panel');
			const head = root.querySelector('.vd-hp-viewport-head');
			const footer = root.querySelector('.vd-hp-footer');
			const pill = root.querySelector('.vd-hp-site-pill');
			const annotate = root.querySelector('[data-hp-annotate]');
			if (!zone || !panel || !head || !footer || !pill || !annotate) return { error: 'the preview zone parts are missing' };
			if (!window.__hpSnapshot) {
				const surface = document.createElement('canvas');
				surface.width = 1280;
				surface.height = 1600;
				const g = surface.getContext('2d');
				g.fillStyle = '#ffffff';
				g.fillRect(0, 0, 1280, 1600);
				window.__hpSnapshot = {
					url: 'http://127.0.0.1:8080/', path: '/', viewport: { width: 1280, height: 800 },
					page: { width: 1280, height: 1600, truncated: false },
					image: surface.toDataURL('image/jpeg', 0.8),
					elements: [{ i: 0, selector: 'body', tag: 'body', box: [0, 0, 1280, 1600], parent: -1 }]
				};
			}
			// The device check above pins the panel size inline; measure the real
			// layout.
			panel.style.width = '';
			panel.style.height = '';
			const pillBefore = { hidden: pill.hidden, text: pill.textContent };
			const box = el => { const r = el.getBoundingClientRect(); return { top: r.top, bottom: r.bottom, left: r.left, right: r.right, width: r.width, height: r.height }; };
			const name = el => el ? el.tagName + '.' + String(el.className) : 'none';
			const hitAt = (x, y) => document.elementFromPoint(x, y);
			const clickable = el => {
				const r = el.getBoundingClientRect();
				return el.contains(hitAt(r.left + r.width / 2, r.top + r.height / 2));
			};
			const out = {};
			for (const device of ['desktop', 'mobile']) {
				root.querySelector('[data-hp-device="' + device + '"]').click();
				await wait(450);
				// The fixture shows raw i18n keys, so the toolbar is wider than the
				// zone and a focus move can scroll the zone sideways; start unscrolled.
				zone.scrollLeft = 0;
				zone.scrollTop = 0;
				const r = {};

				pill.hidden = true;
				pill.textContent = '';
				const zoneStyle = getComputedStyle(zone);
				r.footerDisplayHidden = getComputedStyle(footer).display;
				r.footerHeightHidden = footer.getBoundingClientRect().height;
				r.panelBottomHidden = panel.getBoundingClientRect().bottom;
				r.zoneContentBottom = zone.getBoundingClientRect().bottom - parseFloat(zoneStyle.paddingBottom) - parseFloat(zoneStyle.borderBottomWidth);

				pill.hidden = false;
				pill.textContent = 'Second';
				await wait(100);
				r.footer = box(footer);
				r.pill = box(pill);

				const p = box(panel);
				r.panel = p;
				// The scaled device frame follows the panel when the footer takes room.
				r.frameHeight = root.querySelector('.vd-hp-preview-frame').getBoundingClientRect().height;
				r.head = box(head);
				r.zone = box(zone);
				const topHit = hitAt(p.left + p.width / 2, p.top + 10);
				r.topHit = name(topHit);
				r.topHitInPanel = !!topHit && panel.contains(topHit);
				// The first device button: the fixture's raw i18n keys push the later
				// toolbar buttons beyond the zone.
				r.deviceButtonClickable = clickable(root.querySelector('[data-hp-device="desktop"]'));
				r.urlPillClickable = clickable(root.querySelector('.vd-hp-viewport-title'));
				r.sitePillClickable = clickable(pill);

				annotate.click();
				for (let i = 0; i < 40 && root.querySelector('.vd-hp-annotate-layer').hidden; i++) await wait(50);
				const layer = root.querySelector('.vd-hp-annotate-layer');
				r.layerOpen = !layer.hidden;
				if (r.layerOpen) {
					const l = box(layer);
					const markHit = hitAt(l.left + l.width / 2, l.top + 10);
					r.layer = l;
					r.markHit = name(markHit);
					r.markHitInLayer = !!markHit && layer.contains(markHit);
					annotate.click();
				}
				out[device] = r;
			}
			pill.hidden = pillBefore.hidden;
			pill.textContent = pillBefore.text;
			root.querySelector('[data-hp-device="desktop"]').click();
			await wait(450);
			return out;
		}`)
		if result.Has("error") {
			t.Fatal(result.Get("error").Str())
		}

		const slack = 0.75
		for _, device := range []string{"desktop", "mobile"} {
			got := result.Get(device)
			dump := got.JSON("", "")
			panelTop := got.Get("panel.top").Num()
			panelBottom := got.Get("panel.bottom").Num()
			if got.Get("panel.height").Num() < 150 {
				t.Fatalf("%s: the preview panel needs a sensible height: %s", device, dump)
			}
			if headBottom := got.Get("head.bottom").Num(); headBottom > panelTop+slack {
				t.Fatalf("%s: the viewport head (bottom %.1f) covers the top of the preview panel (top %.1f): %s", device, headBottom, panelTop, dump)
			}
			if drift := got.Get("frameHeight").Num() - got.Get("panel.height").Num(); drift > 1.5 || drift < -1.5 {
				t.Fatalf("%s: the device frame (height %.1f) must follow the panel height %.1f: %s", device, got.Get("frameHeight").Num(), got.Get("panel.height").Num(), dump)
			}
			if !got.Get("topHitInPanel").Bool() {
				t.Fatalf("%s: a point 10 px below the panel top lands on %s, not in the panel: %s", device, got.Get("topHit").Str(), dump)
			}
			if !got.Get("deviceButtonClickable").Bool() || !got.Get("urlPillClickable").Bool() || !got.Get("sitePillClickable").Bool() {
				t.Fatalf("%s: the chrome pills must stay clickable: %s", device, dump)
			}
			if footerTop := got.Get("footer.top").Num(); footerTop < panelBottom-slack {
				t.Fatalf("%s: the site pill footer (top %.1f) covers the bottom of the preview panel (bottom %.1f): %s", device, footerTop, panelBottom, dump)
			}
			if display := got.Get("footerDisplayHidden").Str(); display != "none" && got.Get("footerHeightHidden").Num() > 0 {
				t.Fatalf("%s: an empty footer must take no space, display is %q: %s", device, display, dump)
			}
			if gap := got.Get("zoneContentBottom").Num() - got.Get("panelBottomHidden").Num(); gap > slack || gap < -slack {
				t.Fatalf("%s: with the site pill hidden the panel must reach the bottom of the zone, gap %.1f: %s", device, gap, dump)
			}
			if !got.Get("layerOpen").Bool() {
				t.Fatalf("%s: mark mode did not open: %s", device, dump)
			}
			if !got.Get("markHitInLayer").Bool() {
				t.Fatalf("%s: a point 10 px below the top of the mark layer lands on %s, so the page header cannot be marked: %s", device, got.Get("markHit").Str(), dump)
			}
		}
	})
}
