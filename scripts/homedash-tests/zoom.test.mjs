import assert from 'node:assert/strict'; import { loadEngine, ENGINE_ORDER, APP_ORDER, read, eq } from './harness.mjs'; import { fakeClock, fakeSources, HELLO } from './live-fakes.mjs'; import { contractServer, SEED, EN } from './app.test.mjs'; import { boot } from './designer-kit.mjs';
// G14: the design zoom control. Zoom to fit scales a preview wider than the stage (G2); the control switches the page to actual size (zoom 1, the stage scrolls). The choice lives in the
// designer for the window session, it is not a document field; the edit bar toggle and the View menu items show only while the fit zoom is below 1.
const FILES = ENGINE_ORDER.concat(['hd-tone.js', 'hd-api.js', 'hd-catalog.js', 'hd-render.js']);
const CATALOG = JSON.parse(read('internal/homedash/testdata/widget_catalog.json')), STARTER = JSON.parse(read('ui/testdata/homedash-m2/starter-dashboard.json'));
const pe = (type, x, y) => ({ type, pointerId: 1, isPrimary: true, button: 0, buttons: 1, pointerType: 'mouse', clientX: x, clientY: y, defaultPrevented: false, preventDefault() { this.defaultPrevented = true; } });
const box = (l, t, w, h) => ({ left: l, top: t, right: l + w, bottom: t + h, width: w, height: h });
function renderPage(width) {
  const { E, document } = loadEngine(FILES, { dom: true }), catalog = E.catalog.create(CATALOG), doc = E.doc.normalise(STARTER, { sizeOf: catalog.defaultSize });
  const root = E.el('div', { class: 'hd-root', 'data-hd-family': 'glass' }), stage = E.el('div', { class: 'hd-stage' }); root.appendChild(stage); document.body.appendChild(root); let w = width;
  const page = E.render.page({ doc, page: doc.pages[0], stage, store: { view: () => null, on() {}, off() {}, history: () => Promise.resolve({ points: [] }) }, catalog, t: k => k, lang: 'en', mode: 'design', bp: 'lg', raf: () => 1, caf() {}, measure: () => ({ width: w, height: 700 }) });
  return { page, E, setWidth: v => { w = v; } };
}
// The real app in design mode on a stage of the given width (the stage measure is settable; a breakpoint switch lays the page out again).
async function editAt(width) {
  const { E, document } = loadEngine(APP_ORDER, { dom: true }), clock = fakeClock(), es = fakeSources(), server = contractServer({ seed: SEED }); let w = width, D = null;
  const real = E.designer.create; E.designer = Object.assign({}, E.designer, { create: x => (D = real(x)) });
  const host = document.createElement('div'); document.body.appendChild(host);
  const inst = E.mount(host, { dashboardId: SEED.id, transport: { request: server.request, eventSource: es.open }, t: E.core.makeT(EN, { warn() {} }), lang: 'en', measure: () => ({ width: w, height: 700 }),
    timers: { setTimeout: clock.setTimeout, clearTimeout: clock.clearTimeout, now: clock.now, random: () => 0.5, raf: fn => clock.setTimeout(fn, 16), caf: id => clock.clearTimeout(id) } });
  await clock.advance(50); es.last().emit('hello', HELLO); await clock.advance(20); assert.equal(await inst.enterEdit(), true); await clock.advance(20);
  const pageEl = () => host.querySelector('.hd-page'), stageEl = host.querySelector('.hd-stage'), btn = () => host.querySelector('.hd-editbar .hd-eb-zoom');
  return { E, D, document, host, inst, clock, pageEl, stageEl, btn, live: () => host.querySelector('.hd-live').textContent, actions: () => inst.state().edit.actions, calls: () => server.calls, cell: id => ['--hd-x', '--hd-y', '--hd-w', '--hd-h'].map(v => host.querySelector('[data-hd-wid="' + id + '"]').style.getPropertyValue(v)).join(','),
    relayout: async x => { w = x; D.run('layout_md'); D.run('layout_lg'); await clock.advance(20); }, zoomVar: () => pageEl().style.getPropertyValue('--hd-e-zoom') };
}
export const tests = [
  ['zoom (G14): the page can lay out at actual size: metrics().zoom is 1 and no zoom is written, metrics().fitZoom keeps the fit value; back to fit restores both', () => {
    const { page, setWidth } = renderPage(580), pg = page.el; assert.equal(page.metrics().zoom, 0.604); assert.equal(page.metrics().fitZoom, 0.604); assert.equal(pg.getAttribute('data-hd-zoomed'), 'true');
    page.setZoomActual(true); assert.equal(page.metrics().zoom, 1); assert.equal(page.metrics().fitZoom, 0.604, 'the fit value is still known'); assert.equal(page.metrics().width, 960, 'the layout width is the preview width either way');
    assert.equal(pg.getAttribute('data-hd-zoomed'), null); assert.equal(pg.style.getPropertyValue('--hd-e-zoom'), ''); assert.equal(pg.style.getPropertyValue('--hd-e-pw'), '960px');
    setWidth(300); page.setBreakpoint('lg'); assert.equal(page.metrics().zoom, 1, 'a narrower stage does not zoom at actual size'); assert.equal(page.metrics().fitZoom, 0.5);
    page.setZoomActual(false); assert.equal(page.metrics().zoom, 0.5); assert.equal(pg.style.getPropertyValue('--hd-e-zoom'), '0.5'); assert.equal(pg.getAttribute('data-hd-zoomed'), 'true');
    setWidth(1180); page.setBreakpoint('lg'); assert.equal(page.metrics().fitZoom, 1, 'a wide stage: nothing to fit'); page.setZoomActual(true); assert.equal(page.metrics().zoom, 1); }],
  ['zoom (G14): a page created with zoomActual starts at actual size, view mode is never zoomed whatever the flag says', () => {
    const { E, document } = loadEngine(FILES, { dom: true }), catalog = E.catalog.create(CATALOG), doc = E.doc.normalise(STARTER, { sizeOf: catalog.defaultSize }), stage = E.el('div', { class: 'hd-stage' }); document.body.appendChild(stage);
    const make = o => E.render.page(Object.assign({ doc, page: doc.pages[0], stage: E.el('div', { class: 'hd-stage' }), store: { view: () => null, on() {}, off() {}, history: () => Promise.resolve({ points: [] }) }, catalog, t: k => k, lang: 'en', raf: () => 1, caf() {}, measure: () => ({ width: 580, height: 700 }) }, o));
    const a = make({ mode: 'design', bp: 'lg', zoomActual: true }); assert.equal(a.metrics().zoom, 1); assert.equal(a.metrics().fitZoom, 0.604);
    const v = make({ mode: 'view', zoomActual: true }); assert.equal(v.metrics().zoom, 1); assert.equal(v.metrics().fitZoom, 1, 'unforced: no zoom to fit'); }],
  ['zoom (G14): the edit bar toggle shows only while the fit zoom is below 1; aria-pressed, name and tooltip follow the state; a click is the toggle_zoom command', () => {
    const { E, document } = boot('hd-editbar.js'), cmds = [], tr = (k, v) => (v && v.percent !== undefined ? k + ':' + v.percent : k), bar = E.editbar.create({ t: tr, onCommand: (n, a) => cmds.push(n) }); document.body.appendChild(bar.el);
    const b = bar.el.querySelector('.hd-eb-zoom'); assert.ok(b, 'a button of its own'); assert.equal(b.localName, 'button'); assert.equal(b.getAttribute('data-hd-cmd'), 'toggle_zoom'); assert.ok(b.querySelector('svg, .hd-ico'), 'an icon');
    assert.equal(b.hasAttribute('hidden'), true, 'hidden before any state'); bar.update({ zoomFit: 1, zoomActual: false }); assert.equal(b.hasAttribute('hidden'), true, 'nothing to fit');
    bar.update({ zoomFit: 0.55, zoomActual: false }); assert.equal(b.hasAttribute('hidden'), false); assert.equal(b.getAttribute('aria-pressed'), 'false'); assert.equal(b.getAttribute('aria-label'), 'homedash.designer.zoom_actual'); assert.equal(b.getAttribute('title'), 'homedash.designer.zoom_actual');
    bar.update({ zoomFit: 0.55, zoomActual: true }); assert.equal(b.getAttribute('aria-pressed'), 'true'); assert.equal(b.getAttribute('aria-label'), 'homedash.designer.zoom_fit:55'); assert.equal(b.getAttribute('title'), 'homedash.designer.zoom_fit:55');
    bar.update({ zoomFit: 0.604, zoomActual: true }); assert.equal(b.getAttribute('title'), 'homedash.designer.zoom_fit:60', 'the percentage is rounded'); assert.equal(b.getAttribute('aria-disabled'), null);
    b.dispatchEvent({ type: 'click', target: b }); bar.el.dispatchEvent({ type: 'click', target: b }); eq(cmds, ['toggle_zoom']);
    bar.update({ zoomFit: 1, zoomActual: true }); assert.equal(b.hasAttribute('hidden'), true, 'a wide stage hides it even at actual size'); bar.update({}); assert.equal(b.hasAttribute('hidden'), true); bar.destroy(); }],
  ['zoom (G14): the toggle sits next to the breakpoint switcher and is a plain button in the tab order (no negative tabindex)', () => {
    const { E } = boot('hd-editbar.js'), bar = E.editbar.create({ t: k => k }), group = bar.el.querySelector('[data-hd-group="end"]'), kids = Array.from(group.children), i = kids.findIndex(n => n.classList.contains('hd-eb-zoom')), s = kids.findIndex(n => n.getAttribute('data-hd-slot') === 'bp');
    assert.ok(i >= 0 && s >= 0 && Math.abs(i - s) === 1, 'adjacent to the bp slot'); assert.equal(kids[i].getAttribute('tabindex'), null); assert.equal(bar.focus('zoom'), true, 'focus(name) can reach it'); }],
  ['zoom (G14): the designer state carries the zoom; the control is offered only while the fit zoom is below 1 and actual size is a session choice, not a document field', async () => {
    const m = await editAt(580), D = m.D; eq(D.state().zoom, { fit: 0.604, actual: false, available: true }); assert.equal(m.btn().hasAttribute('hidden'), false); assert.equal(m.btn().getAttribute('aria-pressed'), 'false'); assert.equal(m.pageEl().getAttribute('data-hd-zoomed'), 'true');
    const a = m.actions(); eq([a.zoom_fit.enabled, a.zoom_fit.checked, a.zoom_actual.enabled, a.zoom_actual.checked], [true, true, true, false], 'a radio pair: zoom to fit is the one checked');
    const before = JSON.stringify(D.doc()), puts0 = m.calls().filter(c => c.method === 'PUT').length; assert.equal(D.run('zoom_actual'), true); eq(D.state().zoom, { fit: 0.604, actual: true, available: true }); assert.equal(m.pageEl().getAttribute('data-hd-zoomed'), null); assert.equal(m.zoomVar(), ''); assert.equal(JSON.stringify(D.doc()), before, 'the document is untouched');
    assert.equal(m.btn().getAttribute('aria-pressed'), 'true'); assert.match(m.btn().getAttribute('title'), /Zoom to fit \(60 %\)/); const b = m.actions(); eq([b.zoom_fit.enabled, b.zoom_fit.checked, b.zoom_actual.enabled, b.zoom_actual.checked], [true, false, true, true]);
    assert.equal(m.inst.state().edit.undoLabel, '', 'no history entry'); await m.clock.advance(1000); assert.equal(m.calls().filter(c => c.method === 'PUT').length, puts0, 'nothing is saved by the zoom'); m.inst.dispose(); }],
  ['zoom (G14): the zoom commands announce politely, are idempotent and fall to false while there is nothing to fit', async () => {
    const m = await editAt(580), D = m.D; D.run('zoom_actual'); assert.equal(m.live(), 'Actual size'); D.run('zoom_fit'); assert.equal(m.live().replace(/\s/g, ' '), 'Zoom to fit, 60 %'); assert.equal(D.state().zoom.actual, false);
    D.run('toggle_zoom'); assert.equal(D.state().zoom.actual, true); assert.equal(m.live(), 'Actual size'); D.run('toggle_zoom'); assert.equal(D.state().zoom.actual, false); assert.match(m.live(), /Zoom to fit, 60 %/);
    const l = m.live(); assert.equal(D.run('zoom_fit'), true); assert.equal(m.live(), l, 'already there: no second announcement');
    await m.relayout(1180); eq(D.state().zoom, { fit: 1, actual: false, available: false }); assert.equal(m.btn().hasAttribute('hidden'), true); const a = m.actions(); eq([a.zoom_fit.enabled, a.zoom_actual.enabled, a.zoom_fit.checked, a.zoom_actual.checked], [false, false, false, false]);
    const live = m.live(); assert.equal(D.run('zoom_actual'), false); assert.equal(D.run('toggle_zoom'), false); assert.equal(D.state().zoom.actual, false); assert.equal(m.live(), live); assert.equal(D.handles('zoom_actual') && D.handles('zoom_fit') && D.handles('toggle_zoom'), true); m.inst.dispose(); }],
  ['zoom (G14): the choice survives a breakpoint change and a window resize; the control appears and goes with the fit zoom, and focus never lands on body', async () => {
    const m = await editAt(580), D = m.D; D.run('zoom_actual'); D.run('layout_md'); assert.equal(D.state().zoom.actual, true, 'a breakpoint change keeps the session choice'); assert.equal(m.zoomVar(), ''); D.run('layout_sm'); assert.equal(D.state().zoom.available, false, 'the phone preview fits as it is');
    assert.equal(m.btn().hasAttribute('hidden'), true); D.run('layout_lg'); assert.equal(D.state().zoom.available, true); assert.equal(m.btn().getAttribute('aria-pressed'), 'true');
    m.btn().focus(); assert.equal(m.document.activeElement, m.btn()); await m.relayout(1180); assert.equal(m.btn().hasAttribute('hidden'), true);
    assert.ok(m.document.activeElement && m.document.activeElement !== m.document.body && m.host.contains(m.document.activeElement), 'focus moved off the hidden button, not to body');
    await m.relayout(580); assert.equal(m.btn().hasAttribute('hidden'), false); assert.equal(D.state().zoom.actual, true, 'still actual size after the window shrinks again'); assert.equal(m.zoomVar(), ''); m.inst.dispose(); }],
  ['zoom (G14): switching keeps the selected widget in view (scrollIntoView on it), or the focused one when nothing is selected', async () => {
    const m = await editAt(580), D = m.D, seen = []; for (const id of ['cpu', 'memory']) { const f = m.host.querySelector('[data-hd-wid="' + id + '"]'); f.scrollIntoView = o => seen.push([id, o]); }
    D.run('zoom_actual'); assert.equal(seen.at(-1)[0], 'cpu', 'no selection: the focused frame'); D.select(['memory'], {}); D.run('zoom_fit'); assert.equal(seen.at(-1)[0], 'memory'); D.run('zoom_actual'); assert.equal(seen.at(-1)[0], 'memory');
    assert.equal(seen.at(-1)[1].block, 'center'); assert.equal(seen.at(-1)[1].inline, 'center'); m.inst.dispose(); }],
  ['zoom (G14): at actual size a pointer move maps one to one: one grid pitch of pointer travel is one cell; back at fit the pitch shrinks with the zoom and still lands one cell', async () => {
    const m = await editAt(580), D = m.D, st = m.stageEl, pg = m.pageEl(), z = () => parseFloat(pg.style.getPropertyValue('--hd-e-zoom')) || 1, pitch = () => parseFloat(pg.style.getPropertyValue('--hd-colw')) + parseFloat(pg.style.getPropertyValue('--hd-gap'));
    st.scrollLeft = 0; st.scrollTop = 0; st.getBoundingClientRect = () => box(0, 0, 600, 400); pg.getBoundingClientRect = () => box(16, 16, 960 * z(), 800 * z());
    const cpu = m.host.querySelector('[data-hd-wid="cpu"]'), drag = async dist => { st.dispatchEvent(Object.assign(pe('pointerdown', 100, 100), { target: cpu })); for (const x of [102, 104, 100 + dist / 2, 100 + dist]) m.document.dispatchEvent(pe('pointermove', x, 100)); await m.clock.advance(40); m.document.dispatchEvent(pe('pointerup', 100 + dist, 100)); await m.clock.advance(40); };
    assert.equal(m.cell('cpu'), '0,0,4,4'); D.run('zoom_actual'); assert.equal(z(), 1); const p = pitch(); assert.ok(p > 60, 'a real grid pitch: ' + p); await drag(p); assert.equal(m.cell('cpu'), '1,0,4,4', 'one pitch of pointer travel is one cell at zoom 1');
    D.run('zoom_fit'); assert.ok(z() < 1); await drag(p * z()); assert.equal(m.cell('cpu'), '2,0,4,4', 'at fit, the pointer travel is the pitch scaled by the zoom'); m.inst.dispose(); }],
  ['zoom (G14): the control costs no history, no save and no key: the zoom actions are ordinary engine actions with no shortcut of their own', async () => {
    const m = await editAt(580), D = m.D; for (const a of ['zoom_fit', 'zoom_actual', 'toggle_zoom']) assert.ok(m.E.designer.ACTIONS.includes(a), a);
    const keys = ['+', '-', '=', '0', 'z', '1']; for (const k of keys) { const ev = { type: 'keydown', key: k, ctrlKey: true, target: m.document.activeElement, defaultPrevented: false, preventDefault() { this.defaultPrevented = true; } }; m.host.querySelector('.hd-root').dispatchEvent(ev); if (k !== 'z') assert.equal(ev.defaultPrevented, false, 'Ctrl+' + k + ' is not taken'); }
    assert.equal(D.state().zoom.actual, false); m.inst.dispose(); }],
  ['zoom (G14): the host CSS styles the pressed toggle like an open one and keeps the file within its budget', () => {
    const css = read('ui/css/desktop-app-homedash.css').replace(/\r\n/g, '\n'); assert.ok(/\.hd-eb-btn[^{]*\[aria-pressed="true"\][^{]*\{[^}]*--vd-mat-selected/.test(css), 'aria-pressed uses the selected material'); assert.ok(css.split('\n').length <= 900); }]
];
