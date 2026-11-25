import assert from 'node:assert/strict'; import { loadEngine, ENGINE_ORDER, read } from './harness.mjs';
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
    page.destroy(); assert.equal(stage.querySelectorAll('.hd-widget').length, 0); bus.emit('change', null); }]
];
