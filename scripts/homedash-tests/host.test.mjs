import assert from 'node:assert/strict'; import vm from 'node:vm'; import { read, loadFiles, ENGINE_ORDER, eq } from './harness.mjs'; import { createDocument } from './dom.mjs'; import { lintJs } from './css-lint.mjs';
export const tests = [
  ['host: the desktop files are clean and within the line budget', () => {
    for (const f of ['ui/js/desktop/apps/homedash.js', 'ui/js/desktop/apps/homedash-home.js']) { const text = read(f); assert.deepEqual(lintJs(text, f), [], f); assert.ok(text.split('\n').length < 1100, f); } }],
  ['host: the visibility signal follows the tab, the window and its space (Docker Manager rule)', () => {
    const { window, document } = loadFiles(['ui/js/desktop/apps/homedash.js'], { dom: true }), listeners = [];
    document.hidden = false; document.addEventListener = (type, fn) => listeners.push([type, fn]);
    const cls = new Set(), win = { hidden: false, style: { display: '' }, classList: { contains: c => cls.has(c) }, getClientRects: () => [1] };
    const life = new AbortController(), seen = [], sig = window.HomeDashApp.visibilitySignal(document.createElement('div'), win, life.signal);
    assert.equal(sig.visible(), true); sig.subscribe(v => seen.push(v));
    cls.add('vd-space-hidden'); listeners.find(x => x[0] === 'visibilitychange')[1](); assert.equal(sig.visible(), false);
    cls.delete('vd-space-hidden'); document.hidden = true; listeners[0][1](); assert.equal(sig.visible(), false);
    document.hidden = false; listeners[0][1](); assert.deepEqual(seen, [false, true]); life.abort(); }],
  ['host: an IntersectionObserver that has not reported yet counts as visible; a report of not intersecting hides', () => {
    const document = createDocument(), window = { document }, observers = [];
    class IO { constructor(fn) { this.fn = fn; observers.push(this); } observe() {} disconnect() {} }
    const context = { window, document, console, AbortController, Set, Map, JSON, IntersectionObserver: IO }; vm.createContext(context);
    vm.runInContext(read('ui/js/desktop/apps/homedash.js'), context, { filename: 'homedash.js' }); document.hidden = false; document.addEventListener = () => {};
    const life = new AbortController(), seen = [], sig = window.HomeDashApp.visibilitySignal(document.createElement('div'), null, life.signal);
    assert.equal(observers.length, 1); assert.equal(sig.visible(), true); sig.subscribe(v => seen.push(v));
    observers[0].fn([{ isIntersecting: false }]); assert.equal(sig.visible(), false);
    observers[0].fn([{ isIntersecting: true }]); assert.deepEqual(seen, [false, true]); life.abort(); }],
  ['host: the chrome CSS paints from the theme materials and keeps the app CSS rules', () => {
    const css = read('ui/css/desktop-app-homedash.css'), plain = css.replace(/\/\*[\s\S]*?\*\//g, '');
    assert.ok(plain.includes('var(--vd-mat-'), 'reads no --vd-mat-* token'); assert.ok(!/#[0-9a-fA-F]{3,8}\b/.test(plain), 'hex colour');
    assert.ok(!/\b(?:rgba?|hsla?|hwb|lab|lch|oklab|oklch)\(/i.test(plain), 'literal colour function'); assert.ok(!/\[data-theme/.test(plain), '[data-theme]');
    assert.ok(!/uppercase/i.test(plain), 'uppercase'); assert.ok(!/light-dark\(/.test(plain), 'light-dark()'); assert.ok(css.split('\n').length <= 900);
    for (const m of plain.matchAll(/([^{}]+)\{([^{}]*)\}/g)) { const sel = m[1].trim(), body = m[2];
      if (/!important/.test(body)) assert.ok(/\[hidden\]$/.test(sel), '!important outside [hidden]: ' + sel);
      if (/\b(transition|animation)\s*:/.test(body)) assert.ok(sel.split(',').every(s => s.trim().startsWith('body:not([data-animations="false"])')), 'ungated motion: ' + sel); }
    assert.ok(plain.includes('@media (prefers-reduced-motion: no-preference)')); }],
  ['host: the dashboard, edit and view window menus carry ids and shortcut hints only; Edit items are disabled in view mode', async () => {
    const h = setup(); const menus = h.menus.at(-1);
    eq(menus.map(m => m.id), ['homedash-dashboard', 'homedash-edit', 'homedash-view']);
    const walk = items => items.flatMap(it => (it.type === 'separator' ? [] : [it, ...walk(it.items || [])]));
    for (const m of menus) for (const it of walk(m.items)) { assert.ok(it.id, 'item without id in ' + m.id); assert.equal(it.shortcut, undefined, it.id); }
    const edit = walk(menus[1].items), byId = (list, id) => list.find(i => i.id === id);
    for (const id of ['undo', 'redo', 'cut', 'copy', 'paste', 'duplicate', 'delete', 'select-all']) assert.equal(byId(edit, id).disabled, true, id);
    assert.equal(byId(edit, 'undo').shortcutHint, 'Ctrl+Z'); assert.equal(byId(edit, 'redo').shortcutHint, 'Ctrl+Shift+Z'); assert.equal(byId(edit, 'delete').shortcutHint, 'Del');
    const view = walk(menus[2].items); assert.equal(byId(view, 'edit-mode').shortcutHint, 'Ctrl+E'); assert.equal(byId(view, 'edit-mode').disabled, false);
    eq(byId(view, 'layout').items.map(i => i.id), ['layout-auto', 'layout-lg', 'layout-md', 'layout-sm']);
    assert.equal(byId(view, 'fit').disabled, false); byId(view, 'present').action(); eq(h.calls, ['run:present']);
    h.push(Object.assign({}, h.state(), { edit: Object.assign({}, h.state().edit, { mode: 'design', undoLabel: 'Move', actions: Object.assign({}, h.state().edit.actions, { undo: { available: true, enabled: true }, edit: { available: true, enabled: true, checked: true } }) }) }));
    const next = h.menus.at(-1), undo = walk(next[1].items).find(i => i.id === 'undo');
    assert.equal(undo.disabled, false); assert.equal(undo.label, 'homedash.designer.undo_label:Move'); assert.equal(walk(next[2].items).find(i => i.id === 'edit-mode').checked, true); }],
  ['host (G14): the View menu carries the zoom radio pair after Layout: labelled, no shortcut hint, enabled only in design mode while the fit zoom is below 1, one of the two checked', async () => {
    const h = setup(), walk = items => items.flatMap(it => (it.type === 'separator' ? [] : [it, ...walk(it.items || [])])), zoom = () => { const v = walk(h.menus.at(-1)[2].items); return [v.find(i => i.id === 'zoom-fit'), v.find(i => i.id === 'zoom-actual')]; };
    const A = (enabled, checked) => ({ available: true, enabled, checked }), design = (fit, actual) => Object.assign({}, h.state(), { edit: Object.assign({}, h.state().edit, { mode: 'design', actions: Object.assign({}, h.state().edit.actions, { zoom_fit: A(fit, fit && !actual), zoom_actual: A(fit, fit && actual) }) }) });
    let [f, a] = zoom(); assert.ok(f && a, 'both items exist in view mode'); assert.equal(f.label, 'homedash.menu.zoom_fit'); assert.equal(a.label, 'homedash.designer.zoom_actual'); assert.equal(f.disabled, true, 'view mode: greyed'); assert.equal(a.disabled, true); assert.equal(f.shortcutHint, ''); assert.equal(a.shortcutHint, '');
    const ids = walk(h.menus.at(-1)[2].items).map(i => i.id); assert.ok(ids.indexOf('zoom-fit') === ids.indexOf('layout-sm') + 1 && ids.indexOf('zoom-actual') === ids.indexOf('zoom-fit') + 1, 'right after the Layout submenu: ' + ids.join(','));
    h.push(design(true, false)); [f, a] = zoom(); eq([f.disabled, f.checked, a.disabled, a.checked], [false, true, false, false]); f.action(); a.action(); eq(h.calls, ['run:zoom_fit', 'run:zoom_actual']);
    h.push(design(true, true)); [f, a] = zoom(); eq([f.disabled, f.checked, a.disabled, a.checked], [false, false, false, true]);
    h.push(design(false, false)); [f, a] = zoom(); eq([f.disabled, f.checked, a.disabled, a.checked], [true, false, true, false], 'nothing to fit: both greyed and unchecked'); }],
  ['host: an edit-action change while the primary button is down waits for its own task after the release (D11); without a press it rebuilds at once', async () => {
    const h = setup(), up = () => h.window.document.dispatchEvent({ type: 'pointerup' });
    const next = on => Object.assign({}, h.state(), { edit: Object.assign({}, h.state().edit, { actions: Object.assign({}, h.state().edit.actions, { undo: { available: true, enabled: on } }) }) });
    const n0 = h.menus.length; h.push(next(true)); assert.equal(h.menus.length, n0 + 1, 'no press: rebuilt in the same task');
    h.root.dispatchEvent({ type: 'pointerdown', button: 0 });
    h.push(next(false)); assert.equal(h.menus.length, n0 + 1, 'pressed: not rebuilt while the button is down');
    up(); h.push(next(false)); assert.equal(h.menus.length, n0 + 1, 'in the task of the release: still waiting');
    await new Promise(r => setTimeout(r, 10)); assert.equal(h.menus.length, n0 + 2, 'rebuilt once after the release');
    assert.equal(h.menus.at(-1)[1].items.find(i => i.id === 'undo').disabled, true); }],
  ['host: without E.ddialogs the menus leave out Rename, Import, Export and Look, and New falls back to the submenu', () => {
    const gone = { rename: 1, import: 1, export: 1, look: 1, add_page: 1, new: 1 }, actions = Object.assign({}, ACTIONS); for (const k of Object.keys(gone)) actions[k] = { available: false, enabled: false };
    const h = setup({ edit: { actions } }), ids = m => m.items.map(i => i.id), dash = h.menus.at(-1)[0];
    for (const id of ['rename', 'import', 'export']) assert.ok(!ids(dash).includes(id), id);
    assert.ok(!h.menus.at(-1)[1].items.some(i => i.id === 'look'));
    const nw = dash.items.find(i => i.id === 'new'); eq(nw.items.map(i => i.id), ['new-starter', 'new-empty']); assert.equal(typeof nw.action, 'undefined');
    const withDlg = setup().menus.at(-1)[0]; for (const id of ['new', 'rename', 'import', 'export']) assert.ok(withDlg.items.some(i => i.id === id), id);
    assert.ok(withDlg.items.find(i => i.id === 'new').action); assert.equal(withDlg.items.find(i => i.id === 'new').items, undefined); }],
  ['host: home mode shows one Dashboard menu with Import, greyed when read-only', async () => {
    const h = setup({ mode: 'home', dashboard: null, edit: { mode: 'view', canEdit: false, phase: 'saved', actions: Object.assign({}, ACTIONS, { import: { available: true, enabled: true } }) } });
    const menus = h.menus.at(-1); eq(menus.map(m => m.id), ['homedash-dashboard']);
    eq(menus[0].items.filter(i => i.type !== 'separator').map(i => i.id), ['new', 'open', 'import', 'reload']);
    assert.equal(menus[0].items.find(i => i.id === 'import').disabled, false); menus[0].items.find(i => i.id === 'import').action(); eq(h.calls, ['run:import']);
    h.push(Object.assign({}, h.state(), { desktopReadonly: true, edit: Object.assign({}, h.state().edit, { actions: Object.assign({}, h.state().edit.actions, { import: { available: true, enabled: false } }) }) }));
    await new Promise(r => setTimeout(r, 5)); assert.equal(h.menus.at(-1)[0].items.find(i => i.id === 'import').disabled, true); }],
  ['host: mount options, context menu boundary and the close guard', () => {
    const h = setup(); assert.equal(h.mounted.canEdit, true);
    const items = [{ id: 'copy', label: 'Copy', icon: 'copy', run: () => h.calls.push('copy') }, { separator: true }, { id: 'variant', label: 'Variant', icon: 'variant', items: [{ id: 'v1', label: 'One', icon: 'check', run: () => h.calls.push('v1') }] }];
    h.mounted.ui.menu(5, 6, items); assert.equal(h.shown.length, 1); eq(h.shown[0].slice(0, 2), [5, 6]);
    h.mounted.ui.focusWindow(); eq(h.ctx.focusWindow.calls, ['w1']);assert.equal(h.mounted.ui.sheetHost(), h.root);
    assert.equal(h.boundary.root, h.root); assert.equal(h.boundary.options.onContextMenu({ menu: null }), false); assert.equal(h.shown.length, 1);
    assert.equal(h.boundary.options.onContextMenu({ menu: { x: 1, y: 2, items } }), true); eq(h.shown[1].slice(0, 2), [1, 2]);
    assert.equal(h.before.id, 'w1'); assert.equal(typeof h.before.fn, 'function'); return h.before.fn().then(v => { assert.equal(v, true);
      h.window.HomeDashApp.dispose('w1'); assert.equal(h.before.fn, null); assert.ok(h.calls.includes('dispose')); }); }],
  ['host: toDesktopItems maps separators and submenus, maps icons and runs the engine callback', () => {
    const calls = [], list = h2().toDesktopItems([{ id: 'a', label: 'Alpha', icon: 'delete', danger: true, shortcut: 'Del', run: () => calls.push('a') }, { separator: true },
      { id: 'b', label: 'Beta', icon: 'variant', disabled: true, disabledHint: 'why', items: [{ id: 'c', label: 'Gamma', icon: 'check', run: () => calls.push('c') }, { separator: true }] }, { id: 'd', label: '' }]);
    eq(list.map(i => i.separator === true), [false, true, false, false]); eq(list[1], { separator: true });
    assert.equal(list[0].icon, 'trash'); assert.equal(list[0].danger, true); assert.equal(list[0].fallback, 'A'); assert.equal(list[0].shortcut, 'Del'); list[0].action();
    assert.equal(list[2].disabled, true); assert.equal(list[2].disabledHint, 'why'); assert.equal(list[2].items[0].icon, 'check-square'); eq(list[2].items[1], { separator: true }); list[2].items[0].action();
    assert.equal(list[3].fallback, ''); list[3].action(); eq(calls, ['a', 'c']); eq(h2().toDesktopItems(null), []); }],
  ['host: a home card delete asks through the host sheet and removes through the mount, never through run', () => {
    const h = setup({ mode: 'home', dashboard: null, dashboards: [{ id: 'd_aaaaaaaaaaab', name: 'One', revision: 1 }] }); const del = h.root.querySelector('[data-action="delete"]');
    assert.ok(del); h.root.querySelector('.hdx-home').dispatchEvent({ type: 'click', target: del });
    return Promise.resolve().then(() => Promise.resolve()).then(() => { eq(h.confirms, ['homedash.delete.title']); assert.ok(h.calls.includes('remove:d_aaaaaaaaaaab')); assert.ok(!h.calls.some(c => c.startsWith('run:'))); }); }],
  ['host: the Edit button, the elsewhere chip and the save chip follow the edit state', () => {
    const h = setup(), q = s => h.root.querySelector(s), hidden = s => q(s).hasAttribute('hidden');
    assert.equal(h.root.getAttribute('data-hdx-edit'), 'view'); assert.equal(hidden('.hdx-edit'), false); assert.equal(q('.hdx-edit').getAttribute('aria-keyshortcuts'), 'Control+E'); assert.equal(hidden('.hdx-elsewhere'), true); assert.equal(hidden('.hdx-save'), true);
    const click = s => h.root.dispatchEvent({ type: 'click', target: q(s) }); click('.hdx-edit'); eq(h.calls, ['run:edit']);
    const ed = (patch) => h.push(Object.assign({}, h.state(), { edit: Object.assign({}, h.state().edit, patch) }));
    ed({ editElsewhere: true }); assert.equal(hidden('.hdx-edit'), true); assert.equal(hidden('.hdx-elsewhere'), false); click('.hdx-elsewhere [data-action="show-editor"]'); assert.equal(h.calls.at(-1), 'run:show_editor');
    ed({ editElsewhere: false, phase: 'retrying', nextRetryAt: Date.now() + 4000 }); assert.equal(hidden('.hdx-save'), false); assert.equal(q('.hdx-save').getAttribute('data-phase'), 'retrying'); assert.ok(q('.hdx-save-text').textContent.startsWith('homedash.save.retrying'));
    assert.equal(hidden('.hdx-save [data-action="retry-save"]'), false); click('.hdx-save [data-action="retry-save"]'); assert.equal(h.calls.at(-1), 'run:retry_save');
    ed({ phase: 'conflict' }); assert.equal(q('.hdx-save-text').textContent, 'homedash.save.conflict'); assert.equal(hidden('.hdx-save [data-action="retry-save"]'), true);
    ed({ mode: 'design', phase: 'retrying' }); assert.equal(h.root.getAttribute('data-hdx-edit'), 'design'); assert.equal(hidden('.hdx-save'), true); assert.equal(hidden('.hdx-edit'), true);
    ed({ mode: 'view', phase: 'saved', canEdit: false }); assert.equal(hidden('.hdx-edit'), true); assert.equal(hidden('.hdx-save'), true); }],
  ['host: window menu New > Empty dashboard behaves like the home card: create, open and enter the designer (only when editing is possible)', async () => {
    const gone = { new: { available: false, enabled: false } }, actions = Object.assign({}, ACTIONS, gone), h = setup({ edit: { mode: 'view', canEdit: true, editElsewhere: false, phase: 'saved', actions } });
    const nw = h.menus.at(-1)[0].items.find(i => i.id === 'new'); nw.items.find(i => i.id === 'new-empty').action(); await Promise.resolve(); await Promise.resolve(); eq(h.calls, ['createEmpty', 'enterEdit']);
    const ro = setup({ edit: { mode: 'view', canEdit: false, editElsewhere: false, phase: 'saved', actions } }), r = ro.menus.at(-1)[0].items.find(i => i.id === 'new'); r.items.find(i => i.id === 'new-empty').action(); await Promise.resolve(); await Promise.resolve(); eq(ro.calls, ['createEmpty']); }],
  ['host: the window menu toggles the desktop shortcut of the shown dashboard: label flip, action, toast, read-only grey, absent without the API', async () => {
    const pins = pinsStub(), h = setup({}, { desktopShortcuts: pins }), item = () => h.menus.at(-1)[0].items.find(i => i.id === 'desktop-shortcut'), tick = async () => { for (let i = 0; i < 4; i++) await Promise.resolve(); };
    const ids = h.menus.at(-1)[0].items.filter(i => i.type !== 'separator').map(i => i.id); assert.equal(ids[ids.indexOf('open') + 1], 'desktop-shortcut');
    assert.equal(item().label, 'desktop.context_add_to_desktop'); assert.equal(item().disabled, false); item().action(); assert.equal(item().disabled, true, 'busy while the add is pending'); await tick();
    eq(pins.log, ['add:d_aaaaaaaaaaab']); assert.equal(item().label, 'desktop.context_remove_from_desktop'); assert.equal(item().disabled, false); eq(h.toasts.at(-1), ['homedash.desktop.added', 'info']);
    pins.on.delete('d_aaaaaaaaaaab'); h.window.document.dispatchEvent({ type: 'aurago:desktop-shortcuts' }); assert.equal(item().label, 'desktop.context_add_to_desktop', 'the shell event repaints the label');
    pins.on.add('d_aaaaaaaaaaab'); h.window.document.dispatchEvent({ type: 'aurago:desktop-shortcuts' }); item().action(); await tick(); eq(pins.log.at(-1), 'remove:d_aaaaaaaaaaab'); eq(h.toasts.at(-1), ['homedash.desktop.removed', 'info']);
    pins.fail = true; item().action(); await tick(); eq(h.toasts.at(-1), ['homedash.desktop.add_failed', 'warn']); assert.equal(item().disabled, false, 'idle again after a failure');
    assert.equal(setup({ desktopReadonly: true }, { desktopShortcuts: pinsStub() }).menus.at(-1)[0].items.find(i => i.id === 'desktop-shortcut').disabled, true);
    assert.equal(setup().menus.at(-1)[0].items.some(i => i.id === 'desktop-shortcut'), false); }],
  ['host: a home card pin toggles through the shortcut API, is busy while pending (no double submit), toasts in the host layer and keeps the card text', async () => {
    const pins = pinsStub(), gate = new Promise(r => { pins.release = r; }), h = setup({ mode: 'home', dashboard: null, dashboards: [{ id: 'd_aaaaaaaaaaab', name: 'One', revision: 1 }] }, { desktopShortcuts: pins }); h.noView = true;
    const q = s => h.root.querySelector(s), tick = async () => { for (let i = 0; i < 6; i++) await Promise.resolve(); }, press = () => q('.hdx-home').dispatchEvent({ type: 'click', target: q('.hdx-card-pin') });
    assert.equal(q('.hdx-card-pin').getAttribute('aria-pressed'), 'false'); assert.equal(q('.hdx-card-pin').getAttribute('aria-label'), 'homedash.home.show_on_desktop'); assert.equal(q('.hdx-card-pinned'), null);
    pins.gate = gate; press(); press(); await tick(); assert.equal(q('.hdx-card-pin').getAttribute('aria-busy'), 'true'); assert.equal(q('.hdx-card-pin').getAttribute('aria-disabled'), 'true'); eq(pins.log, ['add:d_aaaaaaaaaaab']);
    pins.release(); pins.gate = null; await tick();
    assert.equal(q('.hdx-card-pin').getAttribute('aria-pressed'), 'true'); assert.equal(q('.hdx-card-pin').getAttribute('aria-busy'), null); assert.equal(q('.hdx-card-pinned').textContent, 'homedash.home.on_desktop');
    assert.equal(q('.hdx-toasts .hd-toast').textContent, 'homedash.desktop.added'); assert.equal(q('.hdx-toasts .hd-toast').getAttribute('data-tone'), 'info'); assert.equal(q('.hd-live').textContent, 'homedash.desktop.added'); eq(h.toasts, []);
    pins.fail = true; press(); await tick(); const last = h.root.querySelectorAll('.hdx-toasts .hd-toast'); assert.equal(last[last.length - 1].getAttribute('data-tone'), 'warn');
    for (let i = 0; i < 4; i++) { press(); await tick(); } assert.ok(h.root.querySelectorAll('.hdx-toasts .hd-toast').length <= 3, 'at most three toasts'); }],
  ['host: with a dashboard open the toast goes through the engine (inst.toast), not the host layer', async () => {
    const pins = pinsStub(), h = setup({}, { desktopShortcuts: pins }); h.menus.at(-1)[0].items.find(i => i.id === 'desktop-shortcut').action(); for (let i = 0; i < 4; i++) await Promise.resolve();
    eq(h.toasts, [['homedash.desktop.added', 'info']]); assert.equal(h.root.querySelector('.hdx-toasts .hd-toast'), null); }],
  ['host: the home card context menu offers Open, the desktop shortcut and Delete; read-only greys the shortcut and Delete; no API leaves no shortcut item', async () => {
    const mk = (patch, extra) => setup(Object.assign({ mode: 'home', dashboard: null, dashboards: [{ id: 'd_aaaaaaaaaaab', name: 'One', revision: 1 }] }, patch), extra), pins = pinsStub();
    const h = mk({}, { desktopShortcuts: pins }), card = h.root.querySelector('.hdx-card-open');
    assert.equal(h.boundary.options.onContextMenu({ target: card, clientX: 5, clientY: 6 }), true); const [x, y, items] = h.shown.at(-1); eq([x, y], [5, 6]);
    eq(items.map(i => i.id || 'sep'), ['open', 'desktop-shortcut', 'sep', 'delete']); assert.equal(items[0].label, 'desktop.context_open'); assert.equal(items[1].label, 'desktop.context_add_to_desktop'); assert.equal(items[3].danger, true);
    items[1].action(); await Promise.resolve(); eq(pins.log, ['add:d_aaaaaaaaaaab']); items[3].action(); assert.deepEqual(h.confirms, ['homedash.delete.title']);
    const keyboard = h.boundary.options.onContextMenu({ target: h.root.querySelector('.hdx-card-open'), clientX: 0, clientY: 0 }); assert.equal(keyboard, true, 'the context menu key has no pointer position and still opens');
    const ro = mk({ desktopReadonly: true }, { desktopShortcuts: pinsStub() }); ro.boundary.options.onContextMenu({ target: ro.root.querySelector('.hdx-card-open'), clientX: 1, clientY: 1 }); const r = ro.shown.at(-1)[2];
    assert.equal(r[1].disabled, true); assert.equal(r[3].disabled, true);
    const no = mk({}); no.boundary.options.onContextMenu({ target: no.root.querySelector('.hdx-card-open'), clientX: 1, clientY: 1 }); eq(no.shown.at(-1)[2].map(i => i.id || 'sep'), ['open', 'sep', 'delete']); }],
  ['host: HomeDashApp.open navigates a live window to a dashboard and is a no-op for a missing window, an empty id or after dispose', () => {
    const h = setup({ mode: 'home', dashboard: null, dashboards: [] }), app = h.window.HomeDashApp;
    app.open('w1', 'd_aaaaaaaaaaab'); app.open('nope', 'd_aaaaaaaaaaac'); app.open('w1', ''); app.open('w1'); eq(h.opened, ['d_aaaaaaaaaaab']);
    app.dispose('w1'); app.open('w1', 'd_aaaaaaaaaaad'); eq(h.opened, ['d_aaaaaaaaaaab']); }],
  ['host: a pin request that settles after dispose paints no toast and starts no timer', async () => {
    const pins = pinsStub(), gate = new Promise(r => { pins.release = r; }), h = setup({ mode: 'home', dashboard: null, dashboards: [{ id: 'd_aaaaaaaaaaab', name: 'One', revision: 1 }] }, { desktopShortcuts: pins });
    const tick = async () => { for (let i = 0; i < 6; i++) await Promise.resolve(); }, root = h.root; pins.gate = gate;
    root.querySelector('.hdx-home').dispatchEvent({ type: 'click', target: root.querySelector('.hdx-card-pin') }); await tick(); eq(pins.log, ['add:d_aaaaaaaaaaab']);
    h.window.HomeDashApp.dispose('w1'); pins.release(); await tick();
    eq(h.toasts, []); assert.equal(root.querySelectorAll('.hd-toast').length, 0); }],
];
function pinsStub() {
  const on = new Set(), log = [], api = { on, log, fail: false, gate: null, hasDashboard: id => on.has(id),
    addDashboard: async id => { log.push('add:' + id); if (api.gate) await api.gate; if (api.fail) throw new Error('add failed'); on.add(id); },
    removeDashboard: async id => { log.push('remove:' + id); if (api.gate) await api.gate; if (api.fail) throw new Error('remove failed'); on.delete(id); } };
  return api;
}
const ACTIONS = Object.fromEntries(['new', 'open', 'rename', 'look', 'add_page', 'export', 'import', 'duplicate_dashboard', 'delete_dashboard', 'edit', 'layout_auto', 'layout_lg', 'layout_md', 'layout_sm', 'fit', 'present'].map(n => [n, { available: true, enabled: true, checked: false }])
  .concat(['undo', 'redo', 'cut', 'copy', 'paste', 'duplicate', 'delete', 'select_all'].map(n => [n, { available: true, enabled: false, checked: false }])));
const HOST_FILES = ENGINE_ORDER.map(f => 'ui/js/homedash/' + f).concat(['ui/js/homedash/hd-tone.js', 'ui/js/homedash/hd-api.js', 'ui/js/homedash/hd-sheet.js', 'ui/js/homedash/hd-save.js', 'ui/js/desktop/apps/homedash-home.js', 'ui/js/desktop/apps/homedash.js']);
function h2() { return loadFiles(HOST_FILES, { dom: true }).window.HomeDashApp; }
// Renders the real host against the real engine helpers (el, icons, sheet, save) with a scripted HomeDashEngine.mount (the minimal stub of plan B7).
function setup(patch, extra) {
  const { window, document } = loadFiles(HOST_FILES, { dom: true }), real = window.HomeDashEngine, listeners = [], calls = [], menus = [], shown = [], confirms = [], mounted = {}, h = { window, calls, menus, shown, confirms, mounted, boundary: null, before: null, toasts: [], opened: [], noView: false };
  let st = Object.freeze(Object.assign({ mode: 'dashboard', dashboard: { id: 'd_aaaaaaaaaaab', name: 'Home' }, dashboards: [], pageId: 'p', desktopReadonly: false, busy: false, templates: [], banners: [], connection: { phase: 'live' },
    edit: { mode: 'view', canEdit: true, editElsewhere: false, phase: 'saved', nextRetryAt: 0, undoLabel: '', redoLabel: '', bp: null, fit: null, actions: ACTIONS } }, patch || {}));
  const inst = { state: () => st, on: (type, fn) => { listeners.push(fn); return () => {}; }, dispose: () => calls.push('dispose'), run: a => { calls.push('run:' + a); return Promise.resolve(true); }, remove: s => { calls.push('remove:' + s.id); return Promise.resolve(true); },
    contextMenu: ev => ev.menu, beforeClose: () => Promise.resolve(true), home: () => {}, open: d => { h.opened.push(d); }, retry: () => {}, reconnect: () => {}, refreshLook: () => {}, toast: (text, to) => { if (h.noView) return false; h.toasts.push([text, to && to.tone]); return true; }, setReadonly: () => {}, createStarter: () => {}, createEmpty: () => { calls.push('createEmpty'); return Promise.resolve(); }, enterEdit: () => { calls.push('enterEdit'); return Promise.resolve(true); }, createHomeAssistant: () => {} };
  window.HomeDashEngine = Object.assign({}, real, { mount: (host, o) => { Object.assign(mounted, o); return inst; }, sheet: { create: () => ({ confirm: o => { confirms.push(o.title); return Promise.resolve(true); }, destroy: () => {} }) } });
  const focusWindow = Object.assign(id => focusWindow.calls.push(id), { calls: [] });
  const ctx = { t: (k, v) => (v && v.label ? k + ':' + v.label : k), api: () => Promise.resolve({}), readonly: false, setWindowMenus: (id, m) => menus.push(m), showContextMenu: (x, y, items) => shown.push([x, y, items]), focusWindow,
    wireContextMenuBoundary: (root, options) => { h.boundary = { root, options }; }, setWindowBeforeClose: (id, fn) => { h.before = { id, fn }; } };
  Object.assign(ctx, extra || {});
  const host = document.createElement('div'); window.HomeDashApp.render(host, 'w1', ctx);
  Object.assign(h, { ctx, root: host.children[0], state: () => st, push: next => { st = Object.freeze(next); for (const fn of listeners) fn(st); } });
  return h;
}
