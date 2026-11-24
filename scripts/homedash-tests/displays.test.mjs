import assert from 'node:assert/strict'; import { loadEngine, noStyleAttr, fc } from './harness.mjs';
export const FILES = ['hd-core.js', 'hd-format.js', 'hd-colour.js', 'hd-icons.js', 'hd-geometry.js', 'hd-prim-displays.js'];
export const STATES = [{}, { on: true, value: 70 }, { pending: true }, { disabled: true }, { status: 'unavailable', value: null }, { stale: true }, { readonly: true }, { status: 'error' }, { tone: 'crit', value: 96 }, { value: 0 }, { value: 100 }, { on: true }];
export const OPTS = { numeral: { chars: 6, unit: 'W' }, segReadout: { label: 'T', digits: 4, decimals: 1, unit: '%' }, arcGauge: { label: 'CPU', min: 0, max: 100, unit: '%', quantize: 0, zones: [{ from: 80, to: 100, tone: 'crit' }] },
  needleGauge: { label: 'P', min: 0, max: 100, unit: '%', majors: 10, minors: 50, numerals: 6, zones: [{ from: 80, to: 100, tone: 'crit' }] }, bar: { label: 'B', variant: 'h', readout: true }, lamp: { label: 'Lamp' }, badge: { icon: 'alert' }, iconBadge: { icon: 'droplet' },
  sparkline: { label: 'Temp', w: 120, h: 40 }, frame: { title: 'Lamp', sub: 'Office', icon: 'bulb', more: 'More' }, sheet: { title: 'Details', kind: 'moreinfo' } };
export function sweep(E, document, names) { for (const name of names) { const inst = E.prim[name](OPTS[name] || { label: name });
  assert.equal(typeof inst.set, 'function', name); const host = document.createElement('div'); host.appendChild(inst.el);
  for (const s of STATES) { inst.set(s); assert.match(inst.el.getAttribute('data-hd-status'), /^(ok|unavailable|unknown|error)$/, name); assert.equal(inst.el.getAttribute('data-hd-pending'), s.pending ? 'true' : null, name); }
  noStyleAttr(inst.el); inst.destroy(); inst.destroy(); assert.equal(host.children.length, 0, name); } }
const boot = () => loadEngine(FILES, { dom: true }); const v = (el, k) => el.style.getPropertyValue(k);
export const tests = [
  ['every display builder: common state, no style attribute, idempotent destroy', () => { const { E, document } = boot(); sweep(E, document, Object.keys(OPTS)); assert.equal(typeof E.prim.util.applyState, 'function'); }],
  ['numeral', () => { const { E } = boot(); const n = E.prim.numeral(OPTS.numeral); n.set({ value: 1234.5, decimals: 1 });
    assert.deepEqual(['.hd-num-int', '.hd-num-frac', '.hd-num-unit'].map(s => n.el.querySelector(s).textContent), ['1,234', '.5', 'W']); assert.equal(v(n.el, '--hd-num-chars'), '6');
    const d = E.prim.numeral({ chars: 4, degree: true, unit: 'C' }); d.set({ value: 21.5, decimals: 1 });
    assert.deepEqual(['.hd-num-int', '.hd-num-frac', '.hd-num-deg', '.hd-num-unit'].map(s => d.el.querySelector(s).textContent), ['21', '.5', fc(0xb0), 'C']);
    assert.equal(n.el.querySelector('.hd-num-deg').textContent, '');
    n.set({ value: null }); assert.equal(n.el.querySelector('.hd-num-int').textContent, fc(0x2013)); assert.equal(n.el.getAttribute('data-hd-empty'), 'true');
    n.set({ text: 'heat' }); assert.equal(n.el.querySelector('.hd-num-int').textContent, 'heat'); assert.equal(n.el.querySelector('.hd-num-frac').textContent, '');
    n.set({ value: 3, trend: 'up' }); const tr = n.el.querySelector('.hd-num-trend'); assert.equal(tr.getAttribute('data-trend'), 'up'); assert.equal(tr.textContent, fc(0x2191)); }],
  ['segReadout lights right-aligned segments', () => { const { E } = boot(); const s = E.prim.segReadout(OPTS.segReadout); s.set({ value: 21.5 });
    const lit = (i, seg) => s.el.querySelector(`g[data-index="${i}"] [data-seg="${seg}"]`).getAttribute('data-lit') === 'true';
    assert.equal(lit(1, 'a'), true); assert.equal(lit(2, 'b'), true); assert.equal(lit(2, 'a'), false); assert.equal(lit(0, 'g'), false);
    assert.equal(s.el.querySelector('g[data-index="2"] .hd-seg7-dp').getAttribute('data-lit'), 'true'); assert.equal(s.el.getAttribute('aria-label'), '21.5' + fc(0xa0) + '%'); }],
  ['gauges sweep one arc and rotate their groups', () => { const { E } = boot(); const dash = el => el.style.getPropertyValue('stroke-dasharray');
    const a = E.prim.arcGauge(OPTS.arcGauge); a.set({ value: 50 });
    assert.equal(a.el.querySelector('svg').getAttribute('viewBox'), '0 0 200 200'); assert.equal(v(a.el, '--hd-angle'), '0deg'); assert.equal(v(a.el, '--hd-value'), '0.5000');
    const arcEl = a.el.querySelector('.hd-gauge-arc'); assert.equal(arcEl.getAttribute('pathLength'), '1000'); assert.equal(dash(arcEl), '500 1000');
    assert.equal(arcEl.getAttribute('d'), E.geometry.arc(100, 100, 88, -135, 270)); assert.equal(a.el.querySelectorAll('.hd-gauge-arc').length, 1);
    const cap = a.el.querySelector('g.hd-gauge-cap .hd-gauge-capdot'); assert.deepEqual([cap.getAttribute('cx'), cap.getAttribute('cy')], ['100', '12']);
    a.set({ value: 96 }); assert.equal(a.el.getAttribute('data-hd-tone'), 'crit'); assert.equal(cap.getAttribute('cx'), '100', 'the cap never moves by cx');
    const q = E.prim.arcGauge(Object.assign({}, OPTS.arcGauge, { quantize: 40 })); q.set({ value: 51 }); assert.equal(dash(q.el.querySelector('.hd-gauge-arc')), '500 1000'); assert.equal(q.el.getAttribute('data-hd-quantize'), '40');
    assert.equal(E.prim.arcGauge(Object.assign({}, OPTS.arcGauge, { crop: 172 })).el.querySelector('svg').getAttribute('viewBox'), '0 0 200 172');
    const nd = E.prim.needleGauge(OPTS.needleGauge); nd.set({ value: 50 }); assert.equal(v(nd.el, '--hd-angle'), '0deg'); nd.set({ value: 50.1 }); assert.equal(v(nd.el, '--hd-angle'), '0deg');
    nd.set({ value: 60 }); assert.equal(v(nd.el, '--hd-angle'), '24deg');
    assert.equal(nd.el.querySelector('.hd-gauge-needle-shape').getAttribute('d'), E.geometry.needle(100, 100, 88, 24, 6.4, 1.6));
    assert.equal(nd.el.querySelector('[data-major="true"]').getAttribute('d'), E.geometry.ticks(100, 100, 72, 88, -120, 240, 10));
    assert.deepEqual(nd.el.querySelectorAll('.hd-gauge-numeral').map(t => t.textContent), ['0', '20', '40', '60', '80', '100']);
    const b = E.prim.bar(OPTS.bar); b.set({ value: 25 }); assert.equal(v(b.el, '--hd-value'), '0.2500'); }],
  ['lamp, badge, iconBadge', () => { const { E } = boot(); const l = E.prim.lamp(OPTS.lamp); l.set({ on: true, text: 'On' });
    assert.equal(l.el.getAttribute('data-hd-on'), 'true'); assert.equal(l.el.getAttribute('aria-label'), 'On'); l.set({ on: false }); assert.equal(l.el.getAttribute('aria-label'), 'Lamp');
    const b = E.prim.badge(OPTS.badge); b.set({ text: '<i>x</i>', tone: 'crit', icon: 'alert' }); const t = b.el.querySelector('.hd-badge-text');
    assert.equal(t.textContent, '<i>x</i>'); assert.equal(t.children.length, 0); assert.equal(b.el.getAttribute('data-tone'), 'crit'); assert.equal(b.el.querySelector('svg').getAttribute('data-hd-icon'), 'alert');
    const i = E.prim.iconBadge(OPTS.iconBadge); i.set({ icon: 'flame' }); assert.equal(i.el.querySelector('svg').getAttribute('data-hd-icon'), 'flame'); i.set({ icon: 'nope' }); assert.equal(i.el.querySelector('svg').getAttribute('data-hd-icon'), 'entity'); }],
  ['sparkline redraws on resize and never stretches', () => { const { E } = boot(); const s = E.prim.sparkline(OPTS.sparkline); s.set({ points: [1, 2, 3] });
    assert.ok(s.el.querySelector('.hd-spark-line').getAttribute('d').startsWith('M')); assert.equal(s.el.getAttribute('viewBox'), '0 0 120 40'); assert.equal(s.el.getAttribute('preserveAspectRatio'), null);
    assert.equal(s.el.querySelector('.hd-spark-last').hasAttribute('hidden'), false); s.set({ points: [], w: 200, h: 50 });
    assert.equal(s.el.getAttribute('viewBox'), '0 0 200 50'); assert.equal(s.el.getAttribute('data-hd-empty'), 'true'); assert.equal(s.el.querySelector('.hd-spark-last').hasAttribute('hidden'), true);
    assert.equal(E.prim.sparkline({ label: 'x', area: false }).el.querySelector('.hd-spark-area'), null); }],
  ['frame and sheet', () => { const { E } = boot(); const f = E.prim.frame(OPTS.frame);
    assert.equal(f.el.querySelector('.hd-title').textContent, 'Lamp'); assert.equal(f.el.querySelector('.hd-more').getAttribute('aria-label'), 'More'); assert.equal(f.el.querySelector('.hd-foot').hasAttribute('hidden'), true);
    assert.equal(f.body, f.el.querySelector('.hd-body')); assert.equal(f.titleId, f.el.querySelector('.hd-title').id); f.set({ title: 'X', stateText: 'On' });
    assert.equal(f.el.querySelector('.hd-title').textContent, 'X'); assert.equal(f.el.querySelector('.hd-state-text').textContent, 'On'); assert.equal(E.prim.frame({ title: 'a' }).el.querySelector('.hd-more'), null);
    const s = E.prim.sheet(OPTS.sheet); const dlg = s.el.querySelector('.hd-sheet'); assert.equal(dlg.getAttribute('aria-labelledby'), s.el.querySelector('.hd-sheet-title').id);
    assert.equal(s.el.hasAttribute('hidden'), true); s.set({ open: true }); assert.equal(s.el.hasAttribute('hidden'), false); s.set({ open: false }); assert.equal(s.el.hasAttribute('hidden'), true); }]
];
