import assert from 'node:assert/strict'; import { loadEngine, ENGINE_ORDER, read, eq } from './harness.mjs';
const FILES = ENGINE_ORDER.concat(['hd-tone.js', 'hd-api.js', 'hd-catalog.js', 'hd-render.js']);
const STARTER = JSON.parse(read('ui/testdata/homedash-m2/starter-dashboard.json')), CATALOG = JSON.parse(read('internal/homedash/testdata/widget_catalog.json'));
const V = (id, value) => ({ id, kind: 'sensor', name: id, unit: '%', value, available: true, status: 'ok', readonly: false, stale: false, pending: false, attributes: { updated_at: '2026-11-25T10:00:00Z' } });
function setup(rawDoc, tr) {
  const { E, document } = loadEngine(FILES, { dom: true }), catalog = E.catalog.create(CATALOG), doc = E.doc.normalise(rawDoc || STARTER, { sizeOf: catalog.defaultSize });
  const root = E.el('div', { class: 'hd-root', 'data-hd-family': 'glass' }), stage = E.el('div', { class: 'hd-stage' }); root.appendChild(stage); document.body.appendChild(root);
  const views = new Map(), bus = E.core.emitter(), histories = [], frames = [], layouts = []; let width = 1180;
  const store = { view: id => views.get(id) || null, on: bus.on, off: bus.off, history(id, range, points) { histories.push([id, range, points]); return Promise.resolve({ points: [[1, 2], [2, 3]], source: 'source' }); } };
  const page = E.render.page({ doc, page: doc.pages[0], stage, store, catalog, t: tr || (k => k), lang: 'en', raf: fn => { frames.push(fn); return frames.length; }, caf: () => {},
    measure: () => ({ width, height: 700 }), onLayout: m => layouts.push(m) });
  return { E, stage, views, bus, histories, page, layouts, run: () => { while (frames.length) frames.shift()(); }, setWidth: w => { width = w; } };
}
// Command-enabled page: a fake commands client (decorate passthrough plus a settable overlay, a send recorder, an emitter) and a spy on the widget host that records each widget api.
const CAPS = ['toggle', { name: 'turn_on' }, { name: 'turn_off' }], LIGHT_CAPS = CAPS.concat([{ name: 'brightness', min: 0, max: 100, step: 1 }]);
const CV = (id, kind, extra) => Object.assign({ id, kind, name: id, value: false, available: true, status: 'ok', readonly: false, stale: false, pending: false, attributes: {}, capabilities: CAPS }, extra);
const CMD_DOC = { id: 'd_cmdcmdcmdc1', name: 'Cmd', pages: [{ id: 'main', widgets: [
  { id: 'sw', type: 'switch', variant: 'pill', bindings: { primary: 'ha:switch.coffee' } }, { id: 'lt', type: 'light', variant: 'tile', bindings: { primary: 'ha:light.kitchen' } },
  { id: 'cpu', type: 'gauge', variant: 'arc', bindings: { primary: 'system:cpu' } }],
  layouts: { lg: [{ id: 'sw', x: 0, y: 0, w: 2, h: 1 }, { id: 'lt', x: 2, y: 0, w: 2, h: 2 }, { id: 'cpu', x: 4, y: 0, w: 2, h: 2 }] } }] };
function setupCmd(withCommands, rawDoc, extraViews) {
  const { E, document } = loadEngine(FILES, { dom: true }), catalog = E.catalog.create(CATALOG), doc = E.doc.normalise(rawDoc || CMD_DOC, { sizeOf: catalog.defaultSize });
  const root = E.el('div', { class: 'hd-root', 'data-hd-family': 'glass' }), stage = E.el('div', { class: 'hd-stage' }); root.appendChild(stage); document.body.appendChild(root);
  const views = new Map(), bus = E.core.emitter(), cbus = E.core.emitter(), frames = [], sent = [], overlay = new Map(), announced = [], sheets = [], apis = new Map();
  const real = E.widgets.host; E.widgets = Object.assign({}, E.widgets, { host(o) { const inst = real(o); apis.set(o.widget.id, o.api({ widget: o.widget, mode: 'view' })); return inst; } });
  const commands = { sent, decorate: (id, v) => (overlay.has(id) ? Object.freeze(Object.assign({}, v, overlay.get(id))) : v), send: spec => { sent.push(spec); return Promise.resolve({ status: 'accepted' }); }, on: cbus.on, off: cbus.off };
  const store = { view: id => views.get(id) || null, on: bus.on, off: bus.off, history: () => Promise.resolve({ points: [], source: 'none' }) };
  views.set('ha:switch.coffee', CV('ha:switch.coffee', 'switch')); views.set('ha:light.kitchen', CV('ha:light.kitchen', 'light', { capabilities: LIGHT_CAPS }));
  for (const v of extraViews || []) views.set(v.id, v);
  const page = E.render.page({ doc, page: doc.pages[0], stage, store, catalog, t: k => k, lang: 'en', raf: fn => { frames.push(fn); return frames.length; }, caf: () => {}, measure: () => ({ width: 1180, height: 700 }),
    commands: withCommands === false ? null : commands, moreInfo: req => sheets.push(req), announce: x => announced.push(x) });
  return { E, document, stage, views, bus, cbus, sent, overlay, announced, sheets, apis, page, run: () => { while (frames.length) frames.shift()(); }, text: id => stage.querySelector('[data-hd-wid="' + id + '"] .hd-state-text') };
}
const vars = (el, ...names) => names.map(n => el.style.getPropertyValue(n)).join(',');
export const tests = [
  ['render: one host per widget, placed in cells, sized in pixels, history in the widget hours', () => {
    const { stage, histories, layouts } = setup();
    assert.equal(stage.querySelectorAll('.hd-page > .hd-widget').length, 6);
    const rx = stage.querySelector('[data-hd-wid="net-rx"]'); assert.equal(vars(rx, '--hd-x', '--hd-y', '--hd-w', '--hd-h'), '0,4,5,2');
    assert.equal(rx.querySelector('svg.hd-spark').getAttribute('width'), '473');
    assert.ok(histories.some(h => h[0] === 'system:net_rx' && h[1] === 3600000 && h[2] === 120)); assert.equal(layouts[0].bp, 'lg'); assert.equal(layouts[0].fit, 'scroll'); }],
  ['render: tone from warn/alert reaches the wrapper and zones are drawn; only bound widgets update', () => {
    const { stage, views, bus } = setup(), cpu = stage.querySelector('[data-hd-wid="cpu"]');
    assert.ok(cpu.querySelector('path.hd-gauge-zone[data-tone="crit"]')); assert.ok(cpu.querySelector('path.hd-gauge-zone[data-tone="warn"]'));
    views.set('system:cpu', V('system:cpu', 97)); views.set('system:memory', V('system:memory', 99)); bus.emit('change', new Set(['system:cpu']));
    assert.equal(cpu.getAttribute('data-hd-tone'), 'crit'); assert.equal(stage.querySelector('[data-hd-wid="memory"]').getAttribute('data-hd-tone'), null);
    views.set('system:cpu', V('system:cpu', 85)); bus.emit('change', null); assert.equal(cpu.getAttribute('data-hd-tone'), 'warn'); assert.equal(stage.querySelector('[data-hd-wid="memory"]').getAttribute('data-hd-tone'), 'crit'); }],
  ['render: a type the engine does not register gets a labelled cell; an empty page gets its note', () => {
    const doc = { name: 'X', pages: [{ id: 'main', widgets: [{ id: 'clock', type: 'clock', variant: 'digital', title: 'Clock' }], layouts: { lg: [{ id: 'clock', x: 0, y: 0, w: 2, h: 1 }] } }] };
    const { stage } = setup(doc), cell = stage.querySelector('.hd-unsupported');
    assert.equal(cell.querySelector('.hd-title').textContent, 'Clock'); assert.equal(cell.querySelector('.hd-state-text').textContent, 'homedash.view.unsupported');
    const empty = setup({ name: 'E', pages: [{ id: 'main', widgets: [], layouts: { lg: [] } }] });
    assert.equal(empty.stage.querySelector('.hd-page-empty .hd-page-empty-title').textContent, 'homedash.view.empty_title'); }],
  ['render: built-in system entities get localized names; other entities keep the server name', () => {
    const { stage, views, bus } = setup(null, k => 'T:' + k), title = id => stage.querySelector('[data-hd-wid="' + id + '"] .hd-title').textContent;
    views.set('system:memory', Object.assign(V('system:memory', 40), { source: 'system', name: 'Memory' }));
    views.set('system:cpu', Object.assign(V('system:cpu', 10), { source: 'other', name: 'Custom CPU' }));
    bus.emit('change', null);
    assert.equal(title('memory'), 'T:homedash.system.memory');
    assert.equal(views.get('system:memory').name, 'Memory');
    assert.equal(title('cpu'), 'Custom CPU'); }],
  ['render: breakpoints follow the stage width with derived, constrained layouts; destroy cleans up', () => {
    const { stage, page, run, setWidth, layouts, bus } = setup();
    setWidth(500); page.layout(); run(); assert.equal(layouts[layouts.length - 1].bp, 'sm');
    assert.equal(stage.querySelector('[data-hd-wid="cpu"]').style.getPropertyValue('--hd-w'), '2');
    page.destroy(); assert.equal(stage.querySelectorAll('.hd-widget').length, 0); bus.emit('change', null); }],
  ['render: without commands the page is view only (can false, command rejects view_only, no more button, no sheet)', async () => {
    const { apis, sheets, stage } = setupCmd(false), api = apis.get('sw');
    assert.equal(api.can('primary', 'turn_on'), false); assert.equal(api.moreInfo('primary'), false); assert.equal(apis.get('lt').moreInfo('primary'), false); assert.equal(sheets.length, 0);
    await assert.rejects(api.command('primary', 'turn_on', {}), e => e.code === 'view_only'); assert.equal(stage.querySelectorAll('.hd-more').length, 0);
    assert.equal(stage.querySelector('[data-hd-wid="cpu"] .hd-title').textContent, ''); }],
  ['render: can follows capability, availability, read-only and mode; command sends the full spec with the widget key', async () => {
    const { apis, views, sent } = setupCmd(), api = apis.get('sw');
    assert.equal(api.can('primary', 'turn_on'), true); assert.equal(api.can('primary', 'brightness'), false); assert.equal(api.can('nope', 'turn_on'), false);
    views.set('ha:switch.coffee', CV('ha:switch.coffee', 'switch', { capabilities: ['toggle', { name: 'turn_on', allowed: false }] })); assert.equal(api.can('primary', 'turn_on'), false); assert.equal(api.can('primary', 'toggle'), true);
    eq(await api.command('primary', 'turn_on', {}), { status: 'refused' }); assert.equal(sent.length, 0);
    views.set('ha:switch.coffee', CV('ha:switch.coffee', 'switch')); await api.command('primary', 'turn_on', { x: 1 }, { phase: 'commit' });
    eq(sent, [{ entity: 'ha:switch.coffee', command: 'turn_on', args: { x: 1 }, phase: 'commit', widgetKey: 'w|d_cmdcmdcmdc1|sw', widgetId: 'sw', dashboardId: 'd_cmdcmdcmdc1', confirm: false }]);
    for (const v of [{ readonly: true }, { available: false }, { status: 'unavailable' }, { status: 'unknown' }]) { views.set('ha:switch.coffee', CV('ha:switch.coffee', 'switch', v)); assert.equal(api.can('primary', 'turn_on'), false, JSON.stringify(v)); } }],
  ['render: a cue from the client reaches the state line as its localized text in one frame; cues and armings are announced', () => {
    const { cbus, overlay, run, text, announced, apis } = setupCmd();
    overlay.set('ha:switch.coffee', { status: 'error', error: { code: 'timeout' } }); cbus.emit('change', ['ha:switch.coffee']);
    assert.notEqual(text('sw').textContent, 'homedash.cue.timeout'); run(); assert.equal(text('sw').textContent, 'homedash.cue.timeout');
    assert.equal(apis.get('sw').entity('primary').error.text, 'homedash.cue.timeout');
    cbus.emit('cue', { id: 'ha:switch.coffee', cue: { code: 'timeout' } }); cbus.emit('cue', { id: 'ha:other', cue: { code: 'timeout' } });
    cbus.emit('armed', { id: 'ha:switch.coffee', widgetKey: 'w|d_cmdcmdcmdc1|sw' }); cbus.emit('armed', { id: 'ha:switch.coffee', widgetKey: 'w|other|sw' });
    eq(announced, ['homedash.cue.timeout', 'homedash.arm.announce']); }],
  ['render: a widget whose entity has no view shows the localized system name as its title; only a light-kind entity shows the more button', () => {
    const { stage } = setupCmd(); assert.equal(stage.querySelector('[data-hd-wid="cpu"] .hd-title').textContent, 'homedash.system.cpu');
    const shown = Array.from(stage.querySelectorAll('.hd-more')).filter(b => !b.hasAttribute('hidden'));
    assert.equal(shown.length, 1); assert.ok(stage.querySelector('[data-hd-wid="lt"] .hd-more') === shown[0]); }],
  ['render: the more button follows the bound entity kind, not the widget type (pill on a light shows it, sensor widget hides it, no commands: none)', () => {
    const raw = { id: 'd_cmdcmdcmdc2', name: 'K', pages: [{ id: 'main', widgets: [
      { id: 'pl', type: 'switch', variant: 'pill', bindings: { primary: 'ha:light.sofa' } }, { id: 'cpu', type: 'gauge', variant: 'arc', bindings: { primary: 'system:cpu' } }],
      layouts: { lg: [{ id: 'pl', x: 0, y: 0, w: 2, h: 1 }, { id: 'cpu', x: 2, y: 0, w: 2, h: 2 }] } }] };
    const extra = [CV('ha:light.sofa', 'light', { capabilities: LIGHT_CAPS }), V('system:cpu', 40)];
    const { stage, apis, sheets, views, bus } = setupCmd(true, raw, extra), more = id => stage.querySelector('[data-hd-wid="' + id + '"] .hd-more');
    assert.ok(more('pl')); assert.equal(more('pl').hasAttribute('hidden'), false); assert.ok(more('cpu')); assert.equal(more('cpu').hasAttribute('hidden'), true);
    more('pl').dispatchEvent({ type: 'click', preventDefault() {}, stopPropagation() {} }); assert.equal(sheets.length, 1); assert.equal(sheets[0].api.entity('primary').id, 'ha:light.sofa');
    views.set('ha:light.sofa', CV('ha:light.sofa', 'switch')); bus.emit('change', new Set(['ha:light.sofa'])); assert.equal(more('pl').hasAttribute('hidden'), true, 'follows the kind when it changes');
    const bare = setupCmd(false, raw, extra); assert.equal(bare.stage.querySelectorAll('.hd-more').length, 0); }],
  ['render: the sheet opener is the focused element inside the widget, else its visible more button, else the widget with a temporary tabindex', () => {
    const raw = { id: 'd_cmdcmdcmdc3', name: 'K', pages: [{ id: 'main', widgets: [{ id: 'lt', type: 'light', variant: 'tile', bindings: { primary: 'ha:light.kitchen' } }], layouts: { lg: [{ id: 'lt', x: 0, y: 0, w: 2, h: 2 }] } }] };
    const { stage, apis, sheets, document } = setupCmd(true, raw), wrap = stage.querySelector('[data-hd-wid="lt"]'), more = wrap.querySelector('.hd-more'), title = wrap.querySelector('.hd-title');
    document.activeElement = document.body; apis.get('lt').moreInfo('primary'); assert.ok(sheets[0].opener === more, 'body is not inside the widget: the more button');
    document.activeElement = title; apis.get('lt').moreInfo('primary'); assert.ok(sheets[1].opener === title, 'an element inside the widget wins');
    document.activeElement = document.body; more.setAttribute('hidden', ''); assert.equal(wrap.hasAttribute('tabindex'), false); apis.get('lt').moreInfo('primary');
    assert.ok(sheets[2].opener === wrap, 'a hidden more button is no opener'); assert.equal(wrap.getAttribute('tabindex'), '-1');
    wrap.dispatchEvent({ type: 'blur' }); assert.equal(wrap.hasAttribute('tabindex'), false, 'the temporary tabindex is dropped again'); }],
  ['render: moreInfo opens the sheet for light entities only, with a sheet api and a subscription that follows the entity', () => {
    const { apis, sheets, cbus, bus, run, sent } = setupCmd();
    assert.equal(apis.get('sw').moreInfo('primary'), false); assert.equal(sheets.length, 0);
    assert.equal(apis.get('lt').moreInfo('primary'), true); assert.equal(sheets.length, 1);
    const req = sheets[0], hits = []; assert.equal(req.family, 'glass'); assert.equal(req.api.entity('primary').id, 'ha:light.kitchen');
    const off = req.subscribe(() => hits.push(1)); cbus.emit('change', ['ha:light.kitchen']); run(); assert.equal(hits.length, 1);
    bus.emit('change', new Set(['ha:light.kitchen'])); assert.equal(hits.length, 2); bus.emit('change', new Set(['ha:switch.coffee'])); assert.equal(hits.length, 2);
    off(); bus.emit('change', null); assert.equal(hits.length, 2);
    req.api.command('primary', 'turn_on', {}); eq(sent.map(x => x.widgetKey), ['sheet|d_cmdcmdcmdc1|lt']); }],
  ['render: destroy detaches the command listeners', () => {
    const { page, cbus, run, stage, announced } = setupCmd(); page.destroy(); cbus.emit('change', ['ha:switch.coffee']); cbus.emit('cue', { id: 'ha:switch.coffee', cue: { code: 'timeout' } }); run();
    eq(announced, []); assert.equal(stage.querySelectorAll('.hd-widget').length, 0); }]
];
