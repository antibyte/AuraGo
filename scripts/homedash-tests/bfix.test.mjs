// Batch B fix round (M4): focus never lands on body after a command, rollback of a failed designer setup, view overrides after edit mode, read-only lanes, the
// beforeunload prompt, the save chip while holding, the window menu "New" fallback and synchronous tab icons. Uses the app harness of app.test.mjs.
import assert from 'node:assert/strict'; import { loadEngine, APP_ORDER, eq } from './harness.mjs'; import { fakeClock, fakeSources, HELLO } from './live-fakes.mjs';
import { mountApp, contractServer, edit, click, SEED, seedOf, page, gauge } from './app.test.mjs';

// A visible element inside the HomeDash root: not body, not the root itself, not inside a hidden subtree.
const usable = m => { const a = m.document.activeElement; return !!a && a !== m.document.body && a !== m.root && m.root.contains(a) && !a.closest('[hidden]'); };
const saveErr = srv => srv.fail(503, { status: 'error', code: 'storage_unavailable', message: 'x' });
const bareMount = async (o) => {   // like mountApp, but hands out the engine window so a test can fake addEventListener before the realm exists
    const { E, window, document } = loadEngine(APP_ORDER, { dom: true }), clock = fakeClock(), es = fakeSources(), server = contractServer(o), listeners = {};
    window.addEventListener = (type, fn) => { (listeners[type] || (listeners[type] = [])).push(fn); };
    const host = document.createElement('div'); document.body.appendChild(host);
    const timers = { setTimeout: clock.setTimeout, clearTimeout: clock.clearTimeout, now: clock.now, random: () => 0.5, raf: fn => clock.setTimeout(fn, 16), caf: id => clock.clearTimeout(id) };
    const inst = E.mount(host, { dashboardId: o.dashboardId, transport: { request: server.request, eventSource: es.open }, t: k => k, lang: 'en', measure: () => ({ width: 1180, height: 700 }), timers });
    await clock.advance(50); es.last().emit('hello', HELLO); await clock.advance(20);
    return { E, window, document, clock, es, server, host, inst, listeners, root: host.querySelector('.hd-root'), puts: () => server.calls.filter(c => c.method === 'PUT' && c.path === 'dashboard') };
};
const unload = m => { const ev = { type: 'beforeunload', defaultPrevented: false, returnValue: undefined, preventDefault() { this.defaultPrevented = true; } }; for (const fn of m.listeners.beforeunload || []) fn(ev); return ev; };

export const tests = [
    ['bfix 1: Retry now hides itself when the phase leaves retrying, yet focus stays on a visible element inside the root', async () => {
        let fails = 1; const m = await edit({ putReply: srv => (fails-- > 0 ? saveErr(srv) : null) }); m.key('ArrowDown'); await m.clock.advance(800);
        const act = m.host.querySelector('.hd-save-action'); assert.equal(m.inst.state().edit.phase, 'retrying'); assert.equal(act.hasAttribute('hidden'), false); act.focus(); assert.equal(m.document.activeElement, act);
        click(m, act); assert.equal(m.host.querySelector('.hd-save').getAttribute('data-phase'), 'saving'); assert.equal(act.hasAttribute('hidden'), true, 'the control that ran the command is gone');
        assert.ok(usable(m), 'focus is a visible element in the root, never body'); assert.ok(m.document.activeElement.closest('.hd-stage'), 'it returns to the canvas');
        await m.clock.advance(50); assert.equal(m.inst.state().edit.phase, 'saved'); assert.ok(usable(m)); m.inst.dispose(); }],
    ['bfix 1: the retry timer succeeding on its own also moves focus off the hidden Retry now button', async () => {
        let fails = 1; const m = await edit({ putReply: srv => (fails-- > 0 ? saveErr(srv) : null) }); m.key('ArrowDown'); await m.clock.advance(800);
        const act = m.host.querySelector('.hd-save-action'); act.focus(); await m.clock.advance(2100); assert.equal(m.inst.state().edit.phase, 'saved'); assert.equal(act.hasAttribute('hidden'), true); assert.ok(usable(m)); m.inst.dispose(); }],
    ['bfix 1: Resolve -> Keep mine hides the conflict bar and focus still ends inside the root', async () => {
        const m = await edit(), btn = id => m.host.querySelector('.hd-sheet-btn[data-action="' + id + '"]'), press = id => { const b = btn(id); b.closest('.hd-sheet-layer').dispatchEvent({ type: 'click', target: b }); };
        m.server.docs.get(SEED.id).revision = 3; m.key('ArrowRight'); await m.clock.advance(850); assert.equal(m.inst.state().edit.phase, 'conflict'); press('later'); await m.clock.advance(20);
        const resolve = m.host.querySelector('.hd-conflict-bar [data-hd-cmd="resolve_conflict"]'); assert.equal(m.host.querySelector('.hd-conflict-bar').hasAttribute('hidden'), false); resolve.focus();
        m.root.dispatchEvent({ type: 'click', target: resolve }); await m.clock.advance(20); assert.ok(btn('mine'), 'the sheet is open'); press('mine'); await m.clock.advance(5);
        assert.equal(m.host.querySelector('.hd-conflict-bar').hasAttribute('hidden'), true); assert.ok(usable(m), 'the opener (Resolve) is hidden: focus moved into the canvas'); await m.clock.advance(100); assert.ok(usable(m)); m.inst.dispose(); }],
    ['bfix 2: window-menu Undo and Delete (inst.run) with focus on body leave focus on a visible element in the root', async () => {
        const m = await edit(), D = m.D(); m.key('ArrowRight'); assert.equal(m.inst.state().edit.canUndo, true); m.document.body.focus(); assert.ok(!usable(m));
        assert.equal(await m.inst.run('undo'), true); assert.equal(m.cell('cpu'), '0,0,4,4'); assert.ok(usable(m), 'undo from the window menu');
        D.select(['cpu'], {}); m.document.body.focus(); assert.equal(await m.inst.run('delete'), true); assert.equal(m.frame('cpu'), null); assert.ok(usable(m), 'delete from the window menu'); assert.ok(m.document.activeElement.closest('.hd-stage'));
        m.document.body.focus(); assert.equal(await m.inst.run('redo'), false, 'nothing to redo after a new command'); m.inst.dispose(); }],
    ['bfix 2: a sheet whose opener disappeared (or was body) returns focus into the root when it closes', async () => {
        const m = await edit(), D = m.D(), gone = m.E.core.el('button', { type: 'button', text: 'x' }); m.root.appendChild(gone); gone.focus();
        const p = D.sheet().open({ title: 'T', text: ['t'], actions: [{ id: 'ok', label: 'OK' }] }); gone.remove(); const b = m.host.querySelector('.hd-sheet-btn[data-action="ok"]'); b.closest('.hd-sheet-layer').dispatchEvent({ type: 'click', target: b });
        assert.equal(await p, 'ok'); assert.ok(usable(m), 'the removed opener left no focus behind');
        m.document.body.focus(); const q = D.sheet().confirm({ title: 'C', text: ['c'], confirmLabel: 'Yes', cancelLabel: 'No' }); m.document.body.focus(); D.sheet().close(null); assert.equal(await q, false); assert.ok(usable(m), 'a body opener too'); m.inst.dispose(); }],
    ['bfix 6: a designer whose setup throws is rolled back completely: view mode, no chrome, no key handling, the lock is free', async () => {
        const m = mountApp({ seed: SEED, dashboardId: SEED.id, en: true }); await m.clock.advance(50); m.es.last().emit('hello', HELLO); await m.clock.advance(20);
        const root = m.host.querySelector('.hd-root'), real = m.E.palette; m.E.palette = Object.freeze({ create() { throw new Error('palette double'); } }); const warn = console.error; const seen = []; console.error = (...a) => seen.push(a);
        try { assert.equal(await m.inst.enterEdit(), false); } finally { console.error = warn; }
        assert.ok(seen.length >= 1, 'the failure is logged'); assert.equal(root.getAttribute('data-hd-mode'), 'view'); assert.equal(m.inst.state().edit.mode, 'view'); assert.equal(m.host.querySelector('.hd-editbar'), null); assert.equal(m.host.querySelector('.hd-conflict-bar'), null);
        assert.equal(m.host.querySelector('.hd-drawer-scrim'), null); assert.equal(m.host.querySelector('.hd-page').getAttribute('role'), 'region'); assert.equal(root.getAttribute('data-hd-has-palette'), null);
        const kd = { type: 'keydown', key: 'ArrowRight', target: m.host.querySelector('.hd-widget'), defaultPrevented: false, preventDefault() { this.defaultPrevented = true; } }; root.dispatchEvent(kd); assert.equal(kd.defaultPrevented, false, 'no designer key listener is left');
        m.E.palette = real; assert.equal(await m.inst.enterEdit(), true, 'the lock was released: the next try works'); m.inst.dispose(); }],
    ['bfix 4: the breakpoint and the fit override of the view mode survive an edit session', async () => {
        const m = mountApp({ seed: SEED, dashboardId: SEED.id, en: true }); await m.clock.advance(50); m.es.last().emit('hello', HELLO); await m.clock.advance(20); const root = m.host.querySelector('.hd-root');
        assert.equal(await m.inst.run('layout_md'), true); assert.equal(await m.inst.run('fit'), true); const bp = root.getAttribute('data-hd-bp'), fit = root.getAttribute('data-hd-fit'); assert.equal(m.inst.state().edit.bp, 'md'); assert.equal(m.inst.state().edit.fit, 'fit_screen');
        assert.equal(await m.inst.enterEdit(), true); assert.equal(m.inst.state().edit.bp, 'lg', 'the designer shows lg'); assert.equal(await m.inst.exitEdit(), 'left');
        assert.equal(m.inst.state().edit.bp, 'md'); assert.equal(m.inst.state().edit.fit, 'fit_screen'); assert.equal(root.getAttribute('data-hd-bp'), bp, 'the page lays out as before'); assert.equal(root.getAttribute('data-hd-fit'), fit); assert.equal(root.getAttribute('data-hd-bp'), 'md'); m.inst.dispose(); }],
    ['bfix 5: the window-menu New action is only available when the part-2 dialogs exist (else the host shows its submenu)', async () => {
        const a = mountApp({ seed: SEED, dashboardId: SEED.id, ddialogs: false }), b = mountApp({ seed: SEED, dashboardId: SEED.id });
        await a.clock.advance(50); a.es.last().emit('hello', HELLO); await a.clock.advance(20); await b.clock.advance(50); b.es.last().emit('hello', HELLO); await b.clock.advance(20);
        assert.equal(a.inst.state().edit.actions.new.available, false); assert.equal(a.inst.state().edit.actions.new_empty.available, true); assert.equal(b.inst.state().edit.actions.new.available, true); assert.equal(b.inst.state().edit.actions.new.enabled, true); a.inst.dispose(); b.inst.dispose(); }],
    ['bfix 7: a lane that is held by a gesture and dirty reports pending (not saved) and emits it', async () => {
        const { E } = loadEngine(APP_ORDER, { dom: true }), clock = fakeClock(), got = [], api = { saveDashboard: async () => ({ dashboard: { revision: 2 }, warnings: [] }) };
        const base = { n: 0 }, l = E.save.lane({ api, id: 'd_aaaaaaaaaaaa', revision: 1, doc: base, timers: { setTimeout: clock.setTimeout, clearTimeout: clock.clearTimeout, now: clock.now } }); l.on('state', s => got.push(s.phase));
        l.hold(true); l.set({ n: 1 }); assert.equal(l.state().phase, 'pending'); assert.equal(l.state().dirty, true); assert.ok(got.includes('pending')); await clock.advance(2000); assert.equal(l.state().saving, false, 'held: nothing is sent');
        l.set(base); assert.equal(l.state().phase, 'saved', 'back to the saved document'); l.set({ n: 2 }); l.hold(false); await clock.advance(900); assert.equal(l.state().phase, 'saved'); }],
    ['bfix 9: a read-only lane keeps its dirty document, sends nothing (debounce, flush, pagehide) and resumes when read-only is lifted', async () => {
        const m = await edit(); m.key('ArrowDown'); await m.clock.advance(10); assert.equal(m.puts().length, 0); m.inst.setReadonly(true); assert.equal(m.inst.state().edit.mode, 'view', 'read-only leaves the editor');
        await m.clock.advance(5000); assert.equal(m.puts().length, 0, 'no PUT while read-only'); assert.equal(m.inst.state().edit.phase, 'readonly'); assert.equal(m.inst.state().edit.dirty, true);
        m.inst.dispose(); await m.clock.advance(100); assert.equal(m.puts().length, 0, 'the dispose flush does not send either');
        const n = await edit(); n.key('ArrowDown'); n.inst.setReadonly(true); await n.clock.advance(100); n.inst.setReadonly(false); await n.clock.advance(900); assert.equal(n.puts().length, 1, 'lifting read-only resumes the save'); assert.equal(n.inst.state().edit.phase, 'saved'); n.inst.dispose(); }],
    ['bfix 9: a lane answers a PUT that was open when read-only began, then waits (no retry timer)', async () => {
        const m = await edit({ putReply: srv => saveErr(srv) }); m.key('ArrowDown'); await m.clock.advance(800); assert.equal(m.puts().length, 1); assert.equal(m.inst.state().edit.phase, 'retrying');
        m.inst.setReadonly(true); await m.clock.advance(60000); assert.equal(m.puts().length, 1, 'the retry timer was cancelled'); assert.equal(m.inst.state().edit.phase, 'readonly'); m.inst.dispose(); }],
    ['bfix 10: beforeunload prompts only for changes that keepalive cannot guarantee', async () => {
        const m = await bareMount({ seed: SEED, dashboardId: SEED.id }); assert.equal(m.listeners.beforeunload.length, 1, 'one realm listener'); assert.equal(unload(m).defaultPrevented, false, 'nothing dirty');
        assert.equal(await m.inst.enterEdit(), true);
        m.root.dispatchEvent({ type: 'keydown', key: 'ArrowRight', target: m.document.activeElement, defaultPrevented: false, preventDefault() {} }); assert.equal(unload(m).defaultPrevented, false, 'a small dirty body relies on the pagehide keepalive');
        m.inst.dispose(); }],
    ['bfix 10: lane.atRisk is true for a body over the keepalive gate and for an open PUT with a queued follow-up', async () => {
        const { E } = loadEngine(APP_ORDER, { dom: true }), clock = fakeClock(), holds = []; const api = { saveDashboard: () => new Promise(resolve => holds.push(() => resolve({ dashboard: { revision: 9 }, warnings: [] }))) };
        const mk = doc => E.save.lane({ api, id: 'd_aaaaaaaaaaaa', revision: 1, doc, timers: { setTimeout: clock.setTimeout, clearTimeout: clock.clearTimeout, now: clock.now } });
        const l = mk({ n: 0 }); assert.equal(l.atRisk(), false, 'clean'); l.set({ n: 1 }); assert.equal(l.atRisk(), false, 'small dirty body: keepalive covers it');
        const big = E.save.lane({ api: { saveDashboard: () => new Promise(() => {}) }, id: 'd_bbbbbbbbbbbb', revision: 1, doc: { n: 0 }, timers: { setTimeout: clock.setTimeout, clearTimeout: clock.clearTimeout, now: clock.now } }); big.set({ blob: 'x'.repeat(E.save.KEEPALIVE_MAX + 10) }); assert.equal(big.atRisk(), true, 'over the gate');
        l.flush(); await clock.advance(1); assert.equal(l.state().saving, true); assert.equal(l.atRisk(), false, 'an open keepalive PUT of the whole document is safe'); l.set({ n: 2 }); assert.equal(l.atRisk(), true, 'a follow-up is queued behind the open PUT');
        holds.shift()(); await clock.advance(5); await clock.advance(900); holds.shift()(); await clock.advance(5); assert.equal(l.state().phase, 'saved'); assert.equal(l.atRisk(), false); }],
    ['bfix 10: the realm listener asks the browser to prompt (preventDefault and returnValue) when a lane is at risk', async () => {
        const m = await bareMount({ seed: SEED, dashboardId: SEED.id, putReply: () => new Promise(() => {}) }); assert.equal(await m.inst.enterEdit(), true);
        const key = k => m.root.dispatchEvent({ type: 'keydown', key: k, target: m.document.activeElement, defaultPrevented: false, preventDefault() {} });
        key('ArrowRight'); await m.clock.advance(800); assert.equal(m.puts().length, 1); assert.equal(unload(m).defaultPrevented, false, 'the open PUT carries the whole document');
        key('ArrowDown'); const ev = unload(m); assert.equal(ev.defaultPrevented, true, 'a follow-up change is not covered'); assert.equal(ev.returnValue, ''); }],
    ['bfix 11: rebuilt page tabs have their icons in the same task (no frame without icons)', async () => {
        const seed = seedOf([page('p1', [gauge('a', 'system:cpu')], [{ id: 'a', x: 0, y: 0, w: 3, h: 3 }]), page('p2', [], [], { name: 'Second' })]), m = await edit({ seed, dashboardId: seed.id }), D = m.D();
        const icons = () => m.host.querySelectorAll('.hd-ptab').map(n => n.querySelector('.hd-ptab-icon') !== null); eq(icons(), [true, true]);
        D.exec(m.E.commands.renamePage(D.doc(), { pageId: 'p2', name: 'Renamed' })); assert.equal(m.host.querySelectorAll('.hd-ptab')[1].textContent, 'Renamed', 'the tab was rebuilt'); eq(icons(), [true, true]);
        D.exec(m.E.commands.addPage(D.doc(), { name: 'Third', icon: 'dashboard' })); eq(icons(), [true, true, true]); m.inst.dispose(); }]
    ,['bfix 13: a tab rename that ends by blur (a click elsewhere) commits without pulling focus back; Enter and Escape return to the tab', async () => {
        const seed = seedOf([page('p1', [gauge('a', 'system:cpu')], [{ id: 'a', x: 0, y: 0, w: 3, h: 3 }]), page('p2', [], [], { name: 'Second' })]), m = await edit({ seed, dashboardId: seed.id }), D = m.D();
        const menu = pid => m.inst.contextMenu({ target: m.host.querySelector('.hd-ptab[data-page="' + pid + '"]'), clientX: 1, clientY: 1 }).items, item = (pid, id) => menu(pid).find(i => i.id === id);
        item('p2', 'page_rename').run(); let input = m.host.querySelector('.hd-ptab-input'); assert.ok(input); assert.equal(m.document.activeElement, input); input.value = 'Renamed'; m.frame('a').focus(); input.dispatchEvent({ type: 'blur' }); await m.clock.advance(50);
        assert.equal(m.host.querySelector('.hd-ptab-input'), null); assert.equal(D.doc().pages[1].name, 'Renamed'); assert.equal(m.document.activeElement, m.frame('a'), 'focus stays where the user clicked');
        item('p2', 'page_rename').run(); input = m.host.querySelector('.hd-ptab-input'); input.value = 'Again'; input.dispatchEvent({ type: 'keydown', key: 'Enter', preventDefault() {}, stopPropagation() {} }); await m.clock.advance(50);
        assert.equal(m.document.activeElement.getAttribute('data-page'), 'p2', 'Enter returns to the tab'); item('p2', 'page_rename').run(); input = m.host.querySelector('.hd-ptab-input'); input.dispatchEvent({ type: 'keydown', key: 'Escape', preventDefault() {}, stopPropagation() {} }); assert.equal(m.document.activeElement.getAttribute('data-page'), 'p2'); m.inst.dispose(); }]
    ,['bfix 15: the page tab menu keeps focus in the root: tab after Duplicate, Icon, Move; the root after Delete', async () => {
        const seed = seedOf([page('p1', [gauge('a', 'system:cpu')], [{ id: 'a', x: 0, y: 0, w: 3, h: 3 }]), page('p2', [gauge('b', 'system:memory')], [{ id: 'b', x: 0, y: 0, w: 3, h: 3 }], { name: 'Second' })]), m = await edit({ seed, dashboardId: seed.id }), D = m.D();
        const item = (pid, id) => m.inst.contextMenu({ target: m.host.querySelector('.hd-ptab[data-page="' + pid + '"]'), clientX: 1, clientY: 1 }).items.find(i => i.id === id), tabFocus = () => { const a = m.document.activeElement; return a && a.classList && a.classList.contains('hd-ptab') ? a.getAttribute('data-page') : null; };
        m.document.body.focus(); item('p1', 'page_duplicate').run(); assert.equal(D.doc().pages.length, 3); assert.equal(tabFocus(), D.state().pageId); assert.notEqual(D.state().pageId, 'p1', 'the copy is the current page'); assert.ok(usable(m));
        m.document.body.focus(); item('p1', 'page_right').run(); assert.equal(tabFocus(), 'p1'); assert.ok(usable(m));
        m.document.body.focus(); const icon = item('p2', 'page_icon').run(); await m.clock.advance(5); m.document.body.focus(); D.sheet().close('done'); await icon; assert.equal(tabFocus(), 'p2'); assert.ok(usable(m));
        m.document.body.focus(); const del = item('p2', 'page_delete').run(); await m.clock.advance(5); m.document.body.focus(); D.sheet().close(null); assert.equal(await del, false); assert.ok(usable(m), 'a cancelled delete confirm'); assert.equal(D.doc().pages.length, 3);
        m.document.body.focus(); const del2 = item('p2', 'page_delete').run(); await m.clock.advance(5); const yes = m.host.querySelector('.hd-sheet-btn[data-action="confirm"]'); yes.closest('.hd-sheet-layer').dispatchEvent({ type: 'click', target: yes }); await del2;
        assert.equal(D.doc().pages.length, 2); assert.ok(usable(m)); assert.equal(tabFocus(), D.state().pageId); m.inst.dispose(); }]
];
