package ui

import (
	"strings"
	"testing"
)

// Wave 4 browser guards: enable control first (E5), drawer breakpoint and history (E1, E2), shared helpers (E6-E10).

// configEnableFirstJS reports whether the section's enable control exists and is the first visible control.
const configEnableFirstJS = `key => {
	const root = document.getElementById('content');
	const entry = (AuraConfigCatalog.sectionStatePaths || {})[key] || {};
	if (entry.none) return {has: false};
	const paths = entry.on || [key + '.enabled'];
	const isShown = el => {
		if (!el.getClientRects().length) return false;
		for (let node = el; node && node !== root; node = node.parentElement) {
			const style = getComputedStyle(node);
			if (style.display === 'none' || style.visibility === 'hidden') return false;
		}
		return true;
	};
	const controls = [...root.querySelectorAll('input:not([type="hidden"]), select, textarea, .toggle, [role="switch"]')]
		.filter(el => !el.closest('.password-wrap') || el.matches('input')).filter(isShown);
	for (const path of paths) {
		const enable = controls.find(el => el.dataset.path === path);
		if (!enable) continue;
		const first = controls[0];
		return {has: true, path, first: first === enable || first.contains(enable) || enable.contains(first), firstIdent: first.dataset.path || first.id || first.tagName};
	}
	return {has: false};
}`

// Sections whose enable control is deliberately not the first control. Each entry needs a reason.
var configEnableFirstExceptions = map[string]string{
	"tts": "no master switch: the provider select is the switch, tts.piper.enabled only starts the optional local Piper container and lives in the Piper group",
}

func TestConfigEnableControlFirstBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	browser := newSmokeBrowser(t)
	page := browser.MustPage(configRefreshFixtureOriginWith(t, "en", true, func(configuration map[string]any) {
		configGuardEnableNested(configuration)
	}) + "/config#overview")
	defer page.MustClose()
	waitForJSBool(t, page, `() => !!document.querySelector('.pw-overview-card')`)
	page.MustSetViewport(1440, 900, 1, false)
	sections := page.MustEval(`() => SECTIONS.flatMap(group => group.items.map(item => item.key))`).Arr()
	var offenders []string
	checked := 0
	for _, section := range sections {
		key := section.Str()
		if _, skip := configGuardUnrenderable[key]; skip {
			continue
		}
		if _, skip := configEnableFirstExceptions[key]; skip {
			continue
		}
		page.MustEval(`async key => { await selectSection(key, {scrollBehavior:'auto', history:'replace'}); document.querySelectorAll('#content details').forEach(details => details.open = true); resetDirtySnapshot(); await new Promise(resolve => setTimeout(resolve, 350)); }`, key)
		result := page.MustEval(configEnableFirstJS, key)
		if !result.Get("has").Bool() {
			continue
		}
		checked++
		if !result.Get("first").Bool() {
			offenders = append(offenders, key+" "+result.Get("path").Str())
		}
	}
	t.Logf("%d sections have an enable control, %d do not put it first", checked, len(offenders))
	configRatchetCheck(t, "testdata/config_enable_not_first.txt", "AURAGO_UPDATE_CONFIG_RATCHETS", offenders,
		"# <section> <enable path>: the section's enable control is not its first visible control (E5). Ends empty.\n")
}

func TestConfigDrawerBreakpointAndHistoryBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	browser := newSmokeBrowser(t)
	page := browser.MustPage(configRefreshFixtureOriginWith(t, "en", true, nil) + "/config#overview")
	defer page.MustClose()
	waitForJSBool(t, page, `() => !!document.querySelector('.pw-overview-card')`)

	// 1000 px: the sidebar is part of the layout (no drawer), nothing scrolls sideways.
	page.MustSetViewport(1000, 800, 1, false)
	for _, key := range []string{"overview", "server", "agent"} {
		page.MustEval(`async key => { await selectSection(key, {scrollBehavior:'auto', history:'replace'}); await new Promise(resolve => setTimeout(resolve, 300)); }`, key)
		got := page.MustEval(`() => {
			const sidebar = document.getElementById('sidebar').getBoundingClientRect();
			return JSON.stringify({
				hamburger: getComputedStyle(document.getElementById('cfg-hamburger')).display,
				inert: document.getElementById('sidebar').inert,
				left: sidebar.left, right: sidebar.right,
				overflow: document.documentElement.scrollWidth - innerWidth
			});
		}`).String()
		if !strings.Contains(got, `"hamburger":"none"`) || !strings.Contains(got, `"inert":false`) || !strings.Contains(got, `"left":0`) || !strings.Contains(got, `"overflow":0`) {
			t.Errorf("1000 px, section %s: sidebar must be visible without a drawer and the page must not scroll sideways: %s", key, got)
		}
	}

	// 800 px: drawer.
	page.MustSetViewport(800, 800, 1, false)
	waitForJSBool(t, page, `() => getComputedStyle(document.getElementById('cfg-hamburger')).display !== 'none' && document.getElementById('sidebar').inert`)
	page.MustElement("#cfg-hamburger").MustClick()
	waitForJSBool(t, page, `() => document.getElementById('sidebar').classList.contains('open') && !document.getElementById('sidebar').inert`)
	page.MustEval(`() => closeSidebar()`)
	page.MustSetViewport(1440, 900, 1, false)

	// Headings: one page title, sections are level 2.
	if !page.MustEval(`() => document.querySelectorAll('h1.cfg-page-title').length === 1 && [...document.querySelectorAll('#content .section-header')].every(h => h.getAttribute('aria-level') === '2')`).Bool() {
		t.Error("expected one page title (h1) and level-2 section headings")
	}

	// History: user navigation pushes entries and sets the tab title; Back returns.
	page.MustEval(`async () => { delete SECTION_MODULES.server; await navigateToConfigSection('server', {scrollBehavior:'auto'}); await navigateToConfigSection('agent', {scrollBehavior:'auto'}); }`)
	waitForJSBool(t, page, `() => location.hash === '#agent' && document.title.startsWith(sectionMetadata('agent').label + ' · ')`)
	page.MustEval(`() => history.back()`)
	waitForJSBool(t, page, `() => location.hash === '#server' && activeSection === 'server' && document.title.startsWith(sectionMetadata('server').label + ' · ')`)
	page.MustEval(`() => history.forward()`)
	waitForJSBool(t, page, `() => location.hash === '#agent' && activeSection === 'agent'`)

	// Back with unsaved changes asks first; staying restores the hash.
	page.MustEval(`() => { const input = document.querySelector('#content [data-path]:not(.toggle)'); input.value = input.type === 'number' ? '4242' : 'changed'; input.dispatchEvent(new Event('input', {bubbles:true})); }`)
	waitForJSBool(t, page, `() => hasUnsavedConfigChanges()`)
	page.MustEval(`() => history.back()`)
	waitForJSBool(t, page, `() => !!document.getElementById('cfg-unsaved-decision')`)
	page.MustEval(`() => document.querySelector('[data-decision="stay"]').click()`)
	waitForJSBool(t, page, `() => location.hash === '#agent' && activeSection === 'agent' && !document.getElementById('cfg-unsaved-decision')`)
	// Staying must not overwrite the entry the user went back to: Back again still reaches the previous section.
	page.MustEval(`() => history.back()`)
	waitForJSBool(t, page, `() => !!document.getElementById('cfg-unsaved-decision')`)
	page.MustEval(`() => document.querySelector('[data-decision="discard"]').click()`)
	waitForJSBool(t, page, `() => location.hash === '#server' && activeSection === 'server' && !document.getElementById('cfg-unsaved-decision')`)
	page.MustEval(`() => history.forward()`)
	waitForJSBool(t, page, `() => location.hash === '#agent' && activeSection === 'agent'`)

	// An unknown hash is normalised in place (no extra entry).
	page.MustEval(`() => { configData = window.AuraConfigState.discard(); resetDirtySnapshot(); window.__len = history.length; location.hash = '#no_such_section'; }`)
	waitForJSBool(t, page, `() => location.hash !== '#no_such_section' && activeSection === 'server'`)
	if !page.MustEval(`() => history.length <= window.__len + 1`).Bool() {
		t.Error("normalising an unknown hash must replace the entry it created")
	}
}

func TestConfigSharedHelpersBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	browser := newSmokeBrowser(t)
	page := browser.MustPage(configRefreshFixtureOriginWith(t, "en", true, nil) + "/config#overview")
	defer page.MustClose()
	waitForJSBool(t, page, `() => !!document.querySelector('.pw-overview-card') && typeof cfgIcon === 'function' && typeof cfgModal === 'function'`)

	check := func(name, expr string) {
		t.Helper()
		if !page.MustEval(expr).Bool() {
			t.Errorf("%s", name)
		}
	}
	check("cfgIcon without label is decorative", `() => { const d = document.createElement('div'); d.innerHTML = cfgIcon('trash'); const s = d.firstElementChild; return s.tagName.toLowerCase() === 'svg' && s.getAttribute('aria-hidden') === 'true' && !s.hasAttribute('role') && s.querySelector('path').getAttribute('d').length > 5; }`)
	check("cfgIcon with label is an image", `() => { const d = document.createElement('div'); d.innerHTML = cfgIcon('alert', {label: 'Warning'}); const s = d.firstElementChild; return s.getAttribute('role') === 'img' && s.getAttribute('aria-label') === 'Warning' && !s.hasAttribute('aria-hidden'); }`)
	check("every icon name renders", `() => AuraConfigIcons.names().every(name => { const d = document.createElement('div'); d.innerHTML = cfgIcon(name); return d.querySelector('svg[data-icon="' + name + '"]'); })`)
	check("emoji map points at existing icons", `() => ['🔌','⚠','🗑','🔄','🔑','⚙','✕','✅'].every(e => AuraConfigIcons.has(cfgIconForEmoji(e)))`)

	check("cfgDeleteButton with text", `() => { const d = document.createElement('div'); d.innerHTML = cfgDeleteButton({attrs: {'data-id': 'a"b'}}); const b = d.firstElementChild; return b.type === 'button' && b.classList.contains('btn-danger') && b.textContent.trim() === 'Delete' && b.dataset.id === 'a"b' && !!b.querySelector('svg'); }`)
	check("cfgDeleteButton icon only is named", `() => { const d = document.createElement('div'); d.innerHTML = cfgDeleteButton({iconOnly: true, label: 'Delete <Foo>'}); const b = d.firstElementChild; return b.getAttribute('aria-label') === 'Delete <Foo>' && b.title === 'Delete <Foo>' && b.textContent.trim() === ''; }`)

	check("cfgBadge uses the status tokens", `() => { const d = document.createElement('div'); d.innerHTML = cfgBadge('success', 'On') + cfgBadge('warning', 'W') + cfgBadge('neutral', 'N') + cfgBadge('checking', 'C') + cfgBadge('error', 'E') + cfgBadge('bogus', 'X'); return [...d.children].map(b => b.className.includes('pw-badge') && b.dataset.status).join() === 'ok,warn,idle,running,error,idle'; }`)

	check("cfgModal markup opens and closes through openModal", `() => {
		document.body.insertAdjacentHTML('beforeend', cfgModal({id: 'cfg-test-modal', title: 'Edit item', size: 'sm', body: '<input id="cfg-test-modal-input">', actions: ['<button type="button" id="cfg-test-modal-ok">OK</button>']}));
		const m = document.getElementById('cfg-test-modal');
		const ok = m.getAttribute('role') === 'dialog' && m.getAttribute('aria-modal') === 'true'
			&& document.getElementById(m.getAttribute('aria-labelledby')).tagName === 'H2'
			&& !!m.querySelector('.modal.modal--sm .modal-actions #cfg-test-modal-ok')
			&& m.querySelector('.modal-close').getAttribute('aria-label') === 'Close' && !!m.querySelector('.modal-close svg');
		openModal('cfg-test-modal', {initialFocus: '#cfg-test-modal-input'});
		const open = m.classList.contains('open') || m.classList.contains('active');
		closeModal('cfg-test-modal');
		return ok && open && !m.classList.contains('open') && !m.classList.contains('active');
	}`)

	check("cfgListState loading, empty and error with retry", `() => {
		const host = document.createElement('div'); document.body.append(host);
		cfgListState(host, {state: 'loading'});
		const loading = host.querySelector('.pw-loading-state[role="status"]');
		cfgListState(host, {state: 'empty', title: 'No items', text: 'Add one.'});
		const empty = host.querySelector('.pw-empty-state h3')?.textContent === 'No items';
		let retried = 0;
		cfgListState(host, {state: 'error', retry: () => retried++});
		const error = host.querySelector('.pw-error-state[role="alert"]');
		host.querySelector('[data-cfg-retry]').click();
		cfgListState(host, {state: 'error'});
		return !!loading && empty && !!error && retried === 1 && !host.querySelector('[data-cfg-retry]');
	}`)

	check("form builder: url, unit, choice list, action row, banner", `() => {
		const f = AuraConfigForm;
		const d = document.createElement('div');
		d.innerHTML = f.renderSpec({key: 'demo', label: 'Demo', desc: 'Demo section.', banner: {id: 'demo-banner', state: 'off'},
			fields: [
				{type: 'text', path: 'demo.name', label: 'Name', help: 'h'},
				{type: 'toggle', path: 'demo.enabled', label: 'Enable', help: 'h', value: true},
				{type: 'url', path: 'demo.url', label: 'URL', help: 'h'},
				{type: 'number', path: 'demo.timeout', label: 'Timeout', help: 'h', min: 0, unit: 's'},
				{type: 'choice', name: 'demo-pick', label: 'Pick', help: 'h', items: [{value: 'a', label: 'A', checked: true}, {value: 'b', label: 'B'}]},
				f.actions([{label: 'Test', id: 'demo-test', action: 'demo-test', draft: true}])
			]});
		const controls = [...d.querySelectorAll('input, select, textarea, .toggle')];
		return controls[0].dataset.path === 'demo.enabled'
			&& !!d.querySelector('#demo-banner')
			&& d.querySelector('input[type="url"]').getAttribute('inputmode') === 'url'
			&& d.querySelector('.cfg-input-unit .cfg-unit').textContent === 's'
			&& d.querySelectorAll('input[type="checkbox"][data-cfg-choice]').length === 2
			&& d.querySelector('button#demo-test').dataset.configAction === 'demo-test';
	}`)
}

// The embeddings reset and security dialogs of main.js and the OpenRouter browser use the shared modal
// controller; the OpenRouter browser stacks on top of another dialog and Escape closes only the top one.
func TestConfigWave4SharedDialogsBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	page := newSmokeBrowser(t).MustPage(configRefreshFixtureOrigin(t, "en", true) + "/config#overview")
	defer page.MustClose()
	waitForJSBool(t, page, `() => !!document.querySelector('.pw-overview-card')`)
	page.MustEval(`async () => {
		await selectSection('providers');
		const original = window.fetch;
		window.fetch = (url, options) => String(url).includes('/api/openrouter/models')
			? Promise.resolve(new Response(JSON.stringify({data: [{id: 'fixture/model', name: 'Fixture Model', context_length: 8192, pricing: {prompt: '0', completion: '0'}}]}), {status: 200, headers: {'Content-Type': 'application/json'}}))
			: original(url, options);
		window.resetResult = null;
		showEmbeddingsResetModal().then(value => { window.resetResult = value; });
	}`)
	waitForJSBool(t, page, `() => { const m = document.getElementById('embeddings-reset-modal'); return !!m && m.classList.contains('open') && m.getAttribute('role') === 'dialog' && m.classList.contains('modal-overlay'); }`)
	page.MustEval(`() => document.dispatchEvent(new KeyboardEvent('keydown', {key: 'Escape', bubbles: true, cancelable: true}))`)
	waitForJSBool(t, page, `() => !document.getElementById('embeddings-reset-modal') && window.resetResult === false`)
	page.MustEval(`() => { window.resetResult = null; showEmbeddingsResetModal().then(value => { window.resetResult = value; }); }`)
	waitForJSBool(t, page, `() => !!document.getElementById('embeddings-reset-continue')`)
	page.MustEval(`() => document.getElementById('embeddings-reset-continue').click()`)
	waitForJSBool(t, page, `() => !document.getElementById('embeddings-reset-modal') && window.resetResult === true`)

	page.MustEval(`() => showSecurityModal([{id: 'fixture', title: 'Fixture hint'}])`)
	waitForJSBool(t, page, `() => { const m = document.getElementById('sec-harden-modal'); return !!m && m.classList.contains('open') && m.textContent.includes('Fixture hint'); }`)
	page.MustEval(`() => document.getElementById('sec-modal-skip').click()`)
	waitForJSBool(t, page, `() => !document.getElementById('sec-harden-modal')`)

	page.MustEval(`() => {
		document.body.insertAdjacentHTML('beforeend', cfgModal({id: 'stack-base', title: 'Base', body: '<button id="stack-base-btn" type="button">Open</button>'}));
		openModal('stack-base', {initialFocus: '#stack-base-btn'});
	}`)
	waitForJSBool(t, page, `() => document.activeElement && document.activeElement.id === 'stack-base-btn'`)
	page.MustEval(`() => { openOpenRouterBrowser(() => {}); }`)
	waitForJSBool(t, page, `() => { const m = document.getElementById('or-browser-overlay'); return !!m && m.classList.contains('open') && !!document.querySelector('#or-list .prov-or-row, #or-list [class*="prov-or-row"]') && document.activeElement && document.activeElement.id === 'or-search'; }`)
	if !page.MustEval(`() => document.getElementById('stack-base').inert === true`).Bool() {
		t.Fatal("the dialog below the OpenRouter browser stayed interactive")
	}
	page.MustEval(`() => document.dispatchEvent(new KeyboardEvent('keydown', {key: 'Escape', bubbles: true, cancelable: true}))`)
	waitForJSBool(t, page, `() => !document.getElementById('or-browser-overlay') && document.getElementById('stack-base').classList.contains('open') && document.getElementById('stack-base').inert !== true && document.activeElement && document.activeElement.id === 'stack-base-btn'`)
}

// Browser Back with a dialog open must not switch the section behind it: the dialog's canClose guard
// decides (E1); an open dialog without edits closes and the navigation continues. Dialogs inserted after
// page load close on a backdrop click, and switches rendered after load get keyboard handling.
func TestConfigWave4DialogHistoryBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	page := newSmokeBrowser(t).MustPage(configRefreshFixtureOrigin(t, "en", true) + "/config#overview")
	defer page.MustClose()
	waitForJSBool(t, page, `() => !!document.querySelector('.pw-overview-card')`)
	page.MustEval(`async () => { await navigateToConfigSection('server', {scrollBehavior:'auto'}); await navigateToConfigSection('providers', {scrollBehavior:'auto'}); resetDirtySnapshot(); }`)
	waitForJSBool(t, page, `() => location.hash === '#providers' && activeSection === 'providers' && !!document.querySelector('.prov-section-actions button')`)

	// Dialog with edits: Back is refused, the discard prompt appears, the section stays.
	page.MustElement(".prov-section-actions button").MustClick()
	waitForJSBool(t, page, `() => !!document.getElementById('provider-dialog') && document.getElementById('provider-dialog').classList.contains('active')`)
	if !page.MustEval(`() => [...document.querySelectorAll('#provider-dialog .toggle')].every(t => t.getAttribute('role') === 'switch' && t.dataset.a11yBound === 'true')`).Bool() {
		t.Error("switches inside the provider dialog lack keyboard handling")
	}
	page.MustEval(`() => { const name = document.getElementById('prov-name'); name.value = 'Edited name'; name.dispatchEvent(new Event('input', {bubbles:true})); }`)
	page.MustEval(`() => history.back()`)
	waitForJSBool(t, page, `() => { const c = document.getElementById('shared-modal-overlay'); return !!c && c.classList.contains('active'); }`)
	if !page.MustEval(`() => activeSection === 'providers' && location.hash === '#providers' && !!document.getElementById('provider-dialog')`).Bool() {
		t.Fatal("Back with an edited dialog switched the section or closed the dialog")
	}
	page.MustEval(`() => document.getElementById('shared-modal-confirm').click()`)
	waitForJSBool(t, page, `() => !document.getElementById('provider-dialog') && activeSection === 'providers'`)

	// Dialog without edits: it closes and the navigation continues.
	page.MustElement(".prov-section-actions button").MustClick()
	waitForJSBool(t, page, `() => !!document.getElementById('provider-dialog') && document.getElementById('provider-dialog').classList.contains('active')`)
	page.MustEval(`() => history.back()`)
	waitForJSBool(t, page, `() => !document.getElementById('provider-dialog') && activeSection === 'server' && location.hash === '#server'`)

	// A dialog inserted after load closes on a backdrop click; a veto from canClose keeps it open.
	page.MustEval(`() => {
		document.body.insertAdjacentHTML('beforeend', cfgModal({id: 'late-dialog', title: 'Late', body: '<button type="button" id="late-dialog-btn">Go</button>'}));
		window.__veto = true;
		openModal('late-dialog', {initialFocus: '#late-dialog-btn', canClose: () => !window.__veto});
	}`)
	click := `() => { const o = document.getElementById('late-dialog'); o.dispatchEvent(new MouseEvent('mousedown', {bubbles:true})); o.dispatchEvent(new MouseEvent('click', {bubbles:true})); }`
	page.MustEval(click)
	if !page.MustEval(`() => document.getElementById('late-dialog').classList.contains('active')`).Bool() {
		t.Error("a vetoed backdrop close must keep the dialog open")
	}
	page.MustEval(`() => { window.__veto = false; }`)
	page.MustEval(click)
	waitForJSBool(t, page, `() => !document.getElementById('late-dialog').classList.contains('active')`)

	// Switches rendered later get role and Space/Enter handling from the central observer.
	page.MustEval(`() => { document.getElementById('content').insertAdjacentHTML('beforeend', '<div id="late-toggle" class="toggle"></div>'); }`)
	waitForJSBool(t, page, `() => document.getElementById('late-toggle').getAttribute('role') === 'switch'`)
	page.MustEval(`() => { const t = document.getElementById('late-toggle'); t.addEventListener('click', () => t.classList.toggle('on')); t.dispatchEvent(new KeyboardEvent('keydown', {key: ' ', bubbles:true, cancelable:true})); }`)
	if !page.MustEval(`() => document.getElementById('late-toggle').classList.contains('on')`).Bool() {
		t.Error("Space did not operate a late-rendered switch")
	}
}
