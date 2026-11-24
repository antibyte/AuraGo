import assert from 'node:assert/strict'; import { loadEngine, fc } from './harness.mjs'; import { FILES, sweep } from './displays.test.mjs';
const OPTS = { toggle: { label: 'Lamp' }, rocker: { label: 'Fan' }, pushButton: { label: 'Scene', icon: 'bulb', dir: -1 }, slider: { label: 'Dim', variant: 'v', unit: '%' }, knob: { label: 'Dim', variant: 'ring', unit: '%' },
  stepper: { label: 'Target', min: 5, max: 30, step: 0.5, unit: 'C', decimals: 1, decLabel: 'Decrease', incLabel: 'Increase' },
  segmented: { label: 'Mode', options: [{ id: 'heat', label: 'Heat' }, { id: 'cool', label: 'Cool' }, { id: 'off', label: 'Off' }] }, colourWheel: { label: 'Colour', diameter: 120 }, ctStrip: { label: 'White', min: 2200, max: 6500 } };
const boot = () => loadEngine([...FILES, 'hd-prim-controls.js'], { dom: true }); const v = (el, k) => el.style.getPropertyValue(k);
export const tests = [
  ['every control builder passes the common sweep', () => { const { E, document } = boot(); for (const n of Object.keys(OPTS)) { const orig = E.prim[n]; E.prim[n] = o => orig(Object.assign({}, OPTS[n], o)); } sweep(E, document, Object.keys(OPTS)); }],
  ['toggle and rocker are switches', () => { const { E } = boot(); const t = E.prim.toggle(OPTS.toggle);
    assert.equal(t.el.tagName, 'BUTTON'); assert.equal(t.el.getAttribute('role'), 'switch'); assert.equal(t.el.getAttribute('type'), 'button'); assert.equal(t.el.getAttribute('aria-label'), 'Lamp');
    t.set({ on: true }); assert.equal(t.el.getAttribute('aria-checked'), 'true'); assert.equal(t.el.getAttribute('data-hd-on'), 'true');
    t.set({ readonly: true }); assert.equal(t.el.getAttribute('aria-readonly'), 'true'); t.set({ status: 'unavailable' }); assert.equal(t.el.getAttribute('aria-disabled'), 'true');
    const r = E.prim.rocker(OPTS.rocker); assert.deepEqual(r.el.querySelectorAll('.hd-rocker-legend').map(x => x.textContent), ['I', 'O']); r.set({ on: false }); assert.equal(r.el.getAttribute('aria-checked'), 'false'); }],
  ['pushButton maps read-only to aria-disabled', () => { const { E } = boot(); const p = E.prim.pushButton(OPTS.pushButton);
    assert.equal(p.el.querySelector('.hd-push-label').textContent, 'Scene'); assert.ok(p.el.querySelector('svg.hd-ico')); assert.equal(p.el.getAttribute('data-dir'), '-1'); p.set({ readonly: true });
    assert.equal(p.el.getAttribute('aria-disabled'), 'true'); assert.equal(p.el.getAttribute('aria-readonly'), null); }],
  ['slider and knob write value, angle and ARIA', () => { const { E } = boot(); const s = E.prim.slider(OPTS.slider);
    assert.equal(s.el.getAttribute('aria-orientation'), 'vertical'); s.set({ value: 25 }); assert.equal(v(s.el, '--hd-value'), '0.2500'); assert.equal(s.el.getAttribute('aria-valuenow'), '25');
    assert.equal(s.el.getAttribute('aria-valuetext'), '25' + fc(0xa0) + '%'); s.set({ value: null }); assert.equal(s.el.getAttribute('aria-valuenow'), null); assert.equal(s.el.getAttribute('aria-valuetext'), fc(0x2013));
    const k = E.prim.knob(OPTS.knob); const arcEl = k.el.querySelector('.hd-dial-arc'), knobDot = k.el.querySelector('g.hd-dial-handle .hd-dial-knob');
    assert.equal(k.el.querySelector('svg').getAttribute('viewBox'), '0 0 200 200'); assert.equal(arcEl.getAttribute('pathLength'), '1000');
    assert.equal(arcEl.getAttribute('d'), E.geometry.arc(100, 100, 88, -135, 270)); assert.deepEqual([knobDot.getAttribute('cx'), knobDot.getAttribute('cy')], ['100', '12']);
    k.set({ value: 0 }); assert.equal(v(k.el, '--hd-angle'), '-135deg'); assert.equal(arcEl.style.getPropertyValue('stroke-dasharray'), '0 1000');
    k.set({ value: 100 }); assert.equal(v(k.el, '--hd-angle'), '135deg'); assert.equal(arcEl.style.getPropertyValue('stroke-dasharray'), '1000 1000'); assert.equal(knobDot.getAttribute('cx'), '100');
    assert.equal(k.el.querySelector('.hd-dial-readout .hd-num-int').textContent, '100'); assert.equal(k.el.querySelectorAll('.hd-dial-active').length, 0);
    const t = E.prim.knob({ label: 'T', min: 5, max: 30, step: 0.5, decimals: 1, activeSegment: true, legends: true, readout: 'seg7' });
    t.set({ value: 21.5, current: 19.5 }); assert.equal(t.el.getAttribute('data-buf'), 'a');
    const lo = -135 + (19.5 - 5) / 25 * 270, hi = -135 + (21.5 - 5) / 25 * 270;
    assert.equal(t.el.querySelector('.hd-dial-active[data-buf="a"]').getAttribute('d'), E.geometry.arc(100, 100, 88, lo, hi - lo));
    t.set({ value: 22, current: 19.5 }); assert.equal(t.el.getAttribute('data-buf'), 'b'); t.set({ value: 22, current: 19.5 }); assert.equal(t.el.getAttribute('data-buf'), 'b', 'an unchanged segment keeps its buffer');
    assert.ok(t.el.querySelector('g.hd-dial-current .hd-dial-marker')); assert.ok(t.el.querySelector('.hd-dial-readout .hd-seg7'));
    assert.deepEqual(t.el.querySelectorAll('.hd-dial-legend').map(x => x.textContent), ['5', '30']);
    const s40 = E.prim.knob({ label: 'q', quantize: 40 }); s40.set({ value: 51 }); assert.equal(s40.el.querySelector('.hd-dial-arc').style.getPropertyValue('stroke-dasharray'), '500 1000'); }],
  ['stepper disables a button at its limit', () => { const { E } = boot(); const s = E.prim.stepper(OPTS.stepper); const [dec, inc] = s.el.querySelectorAll('button.hd-step');
    s.set({ value: 5 }); assert.equal(dec.getAttribute('aria-disabled'), 'true'); assert.equal(inc.getAttribute('aria-disabled'), null); assert.equal(s.el.querySelector('[role="spinbutton"]').getAttribute('aria-valuenow'), '5');
    s.set({ value: 30 }); assert.equal(inc.getAttribute('aria-disabled'), 'true'); assert.equal(dec.getAttribute('aria-disabled'), null); assert.equal(dec.getAttribute('aria-label'), 'Decrease'); }],
  ['stepper buttons always have an accessible name', () => { const { E } = boot(); const s = E.prim.stepper({ label: 'T', min: 0, max: 10 });
    assert.equal(s.el.querySelector('[data-dir="-1"]').getAttribute('aria-label'), String.fromCodePoint(0x2212)); assert.equal(s.el.querySelector('[data-dir="1"]').getAttribute('aria-label'), '+');
    const g = E.prim.stepper({ label: 'T', min: 0, max: 10, decLabel: 'Less', incLabel: 'More' });
    assert.equal(g.el.querySelector('[data-dir="-1"]').getAttribute('aria-label'), 'Less'); assert.equal(g.el.querySelector('[data-dir="1"]').getAttribute('aria-label'), 'More'); }],
  ['segmented uses a roving tabindex', () => { const { E } = boot(); const g = E.prim.segmented(OPTS.segmented); g.set({ value: 'cool' }); const o = g.el.querySelectorAll('.hd-seg-opt');
    assert.equal(v(g.el, '--hd-index'), '1'); assert.equal(v(g.el, '--hd-count'), '3'); assert.deepEqual(o.map(x => [x.getAttribute('aria-checked'), x.getAttribute('tabindex')]), [['false', '-1'], ['true', '0'], ['false', '-1']]);
    g.set({ value: 'zzz' }); assert.equal(v(g.el, '--hd-index'), '-1'); assert.equal(o[0].getAttribute('tabindex'), '0'); }],
  ['colour wheel and colour temperature strip', () => { const { E } = boot(); const w = E.prim.colourWheel(OPTS.colourWheel); w.set({ hue: 90, sat: 1 });
    assert.equal(v(w.el, '--hd-wheel-x'), '1.0000'); assert.equal(v(w.el, '--hd-wheel-y'), '0.5000'); assert.equal(v(w.el, '--hd-wheel-rgb'), '128 255 0'); assert.ok(w.el.querySelector('canvas.hd-wheel-canvas'));
    const c = E.prim.ctStrip(OPTS.ctStrip); c.set({ value: 4350 }); assert.equal(v(c.el, '--hd-value'), '0.5000'); assert.equal(c.el.getAttribute('aria-valuetext'), '4350' + fc(0xa0) + 'K'); }]
];
