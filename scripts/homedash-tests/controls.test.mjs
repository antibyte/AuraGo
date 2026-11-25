import assert from 'node:assert/strict'; import { loadEngine, fc, allElements } from './harness.mjs'; import { FILES, sweep } from './displays.test.mjs'; import { fakeClock } from './live-fakes.mjs';
const OPTS = { toggle: { label: 'Lamp' }, rocker: { label: 'Fan' }, pushButton: { label: 'Scene', icon: 'bulb', dir: -1 }, slider: { label: 'Dim', variant: 'v', unit: '%' }, knob: { label: 'Dim', variant: 'ring', unit: '%' },
  stepper: { label: 'Target', min: 5, max: 30, step: 0.5, unit: 'C', decimals: 1, decLabel: 'Decrease', incLabel: 'Increase' },
  segmented: { label: 'Mode', options: [{ id: 'heat', label: 'Heat' }, { id: 'cool', label: 'Cool' }, { id: 'off', label: 'Off' }] }, colourWheel: { label: 'Colour', diameter: 120 }, ctStrip: { label: 'White', min: 2200, max: 6500 }, picker: { label: 'Mode', text: 'Heat' } };
const boot = () => loadEngine([...FILES, 'hd-prim-controls.js'], { dom: true }); const v = (el, k) => el.style.getPropertyValue(k);
const bootI = () => loadEngine([...FILES.slice(0, 5), 'hd-input.js', ...FILES.slice(5), 'hd-prim-controls.js'], { dom: true });
const pe = (type, x, y, extra) => Object.assign({ type, pointerId: 1, isPrimary: true, button: 0, pointerType: 'touch', clientX: x, clientY: y, preventDefault() {} }, extra);
const key = (k, extra) => Object.assign({ type: 'keydown', key: k, defaultPrevented: false, preventDefault() { this.defaultPrevented = true; } }, extra);
const rect = (w, h) => () => ({ left: 0, top: 0, width: w, height: h, right: w, bottom: h });
const R100 = () => ({ min: 0, max: 100, step: 1 });
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
    const c = E.prim.ctStrip(OPTS.ctStrip); c.set({ value: 4350 }); assert.equal(v(c.el, '--hd-value'), '0.5000'); assert.equal(c.el.getAttribute('aria-valuetext'), '4350' + fc(0xa0) + 'K'); }],
  // Task 8 (SCR-M3-4): the optional input contract.
  ['input (a): without input every builder keeps today\'s markup and needs no hd-input.js', () => { const { E, document } = boot();
    for (const n of Object.keys(OPTS)) { const inst = E.prim[n](OPTS[n]); document.createElement('div').appendChild(inst.el); inst.set({ value: 40, on: true });
      for (const x of allElements(inst.el)) assert.equal(x.getAttribute('data-hd-input'), null, n); inst.destroy(); }
    assert.equal(E.prim.colourWheel(OPTS.colourWheel).el.getAttribute('role'), 'img'); assert.equal(E.prim.pushButton(OPTS.pushButton).el.getAttribute('role'), null);
    assert.equal(E.prim.pushButton(OPTS.pushButton).el.getAttribute('aria-checked'), null); assert.equal(E.prim.stepper(OPTS.stepper).el.querySelector('.hd-step').getAttribute('data-hd-input'), null);
    assert.throws(() => E.prim.toggle({ label: 'x', input: {} }), /hd-input/); }],
  ['input (b): slider keys paint and call onKey; a set() during a drag keeps the drag value; destroy unbinds', () => { const { E, document } = bootI(), log = [];
    const input = { range: R100, onKey: x => log.push('key:' + x), onInput: x => log.push('in:' + x), onCommit: x => log.push('commit:' + x) };
    const s = E.prim.slider({ label: 'Dim', unit: '%', input }); document.createElement('div').appendChild(s.el); const track = s.el.querySelector('.hd-slider-track'); track.getBoundingClientRect = rect(200, 6);
    assert.equal(s.el.getAttribute('data-hd-input'), 'drag'); s.set({ value: 40 }); s.el.dispatchEvent(key('ArrowRight')); assert.deepEqual(log, ['key:41']); assert.equal(s.el.getAttribute('aria-valuenow'), '41'); assert.equal(v(s.el, '--hd-value'), '0.4100');
    log.length = 0; s.el.dispatchEvent(key('End')); assert.deepEqual(log, ['key:100']); s.set({ value: 40 }); assert.equal(s.el.getAttribute('aria-valuenow'), '40', 'a key paints until the next set');
    log.length = 0; s.el.dispatchEvent(pe('pointerdown', 100, 3)); s.el.dispatchEvent(pe('pointermove', 150, 3)); assert.equal(s.el.getAttribute('data-hd-dragging'), 'true');
    s.set({ value: 10 }); assert.equal(s.el.getAttribute('aria-valuenow'), '75', 'input lock'); assert.equal(s.el.getAttribute('aria-valuetext'), '75' + fc(0xa0) + '%');
    s.el.dispatchEvent(pe('pointerup', 150, 3)); assert.deepEqual(log, ['in:75', 'commit:75']); s.set({ value: 10 }); assert.equal(s.el.getAttribute('aria-valuenow'), '10');
    log.length = 0; s.el.dispatchEvent(pe('pointerdown', 20, 3)); s.el.dispatchEvent(pe('pointerup', 20, 3)); assert.deepEqual(log, ['commit:10']);
    log.length = 0; s.destroy(); s.el.dispatchEvent(key('ArrowRight')); s.el.dispatchEvent(pe('pointerdown', 20, 3)); s.el.dispatchEvent(pe('pointerup', 20, 3)); assert.deepEqual(log, []);
    const vs = E.prim.slider({ label: 'F', variant: 'v', input }); vs.el.querySelector('.hd-slider-track').getBoundingClientRect = rect(6, 100); log.length = 0;
    vs.el.dispatchEvent(pe('pointerdown', 3, 100)); vs.el.dispatchEvent(pe('pointermove', 3, 25)); vs.el.dispatchEvent(pe('pointerup', 3, 25)); assert.deepEqual(log, ['in:75', 'commit:75']); }],
  ['input (c): toggle and rocker click runs onActivate, a disabled control reports onBlocked, Shift+Enter holds', () => { const { E } = bootI(), log = []; let ok = true;
    for (const n of ['toggle', 'rocker']) { const t = E.prim[n]({ label: 'x', input: { enabled: () => ok, onBlocked: () => log.push('blocked'), onActivate: () => log.push('act'), onHold: () => log.push('hold') } });
      assert.equal(t.el.getAttribute('data-hd-input'), 'press'); t.el.dispatchEvent({ type: 'click' }); t.el.dispatchEvent(key('Enter', { shiftKey: true })); ok = false; t.el.dispatchEvent({ type: 'click' }); ok = true;
      assert.deepEqual(log.splice(0), ['act', 'hold', 'blocked'], n); t.destroy(); t.el.dispatchEvent({ type: 'click' }); assert.deepEqual(log, []); } }],
  ['input (d): stepper buttons step snapped, hold repeats 400 ms then 80 ms, the spinbutton keys step', async () => { const { E, document } = bootI(), clock = fakeClock(), log = [];
    const input = { range: () => ({ min: 5, max: 30, step: 0.5 }), onStep: x => log.push(x), setTimeout: clock.setTimeout, clearTimeout: clock.clearTimeout, now: clock.now };
    const s = E.prim.stepper(Object.assign({}, OPTS.stepper, { input })); document.createElement('div').appendChild(s.el); const [dec, inc] = s.el.querySelectorAll('button.hd-step'), spin = s.el.querySelector('[role="spinbutton"]');
    s.set({ value: 21.2 }); inc.dispatchEvent({ type: 'click' }); assert.deepEqual(log, [21.5]); assert.equal(spin.getAttribute('aria-valuenow'), '21.5'); assert.equal(spin.querySelector('.hd-num-int').textContent, '21');
    dec.dispatchEvent({ type: 'click' }); assert.deepEqual(log, [21.5, 21]); log.length = 0;
    inc.dispatchEvent(pe('pointerdown', 0, 0)); await clock.advance(399); assert.deepEqual(log, []); await clock.advance(1 + 80 * 2); inc.dispatchEvent(pe('pointerup', 0, 0)); inc.dispatchEvent({ type: 'click', preventDefault() {} }); assert.deepEqual(log, [21.5, 22, 22.5]);
    log.length = 0; spin.dispatchEvent(key('ArrowUp')); spin.dispatchEvent(key('PageDown')); spin.dispatchEvent(key('Home')); assert.deepEqual(log, [23, 20.5, 5]); log.length = 0;
    s.set({ value: 30 }); inc.dispatchEvent({ type: 'click' }); assert.deepEqual(log, [], 'at the limit nothing is sent'); s.destroy(); dec.dispatchEvent({ type: 'click' }); spin.dispatchEvent(key('ArrowUp')); assert.deepEqual(log, []); }],
  ['input (e): segmented Arrow/Home/End move focus and selection with onPick(id, true); a click picks with false', () => { const { E, document } = bootI(), log = [];
    const g = E.prim.segmented(Object.assign({}, OPTS.segmented, { input: { onPick: (id, kbd) => log.push(id + ':' + kbd) } })); document.createElement('div').appendChild(g.el); const o = g.el.querySelectorAll('.hd-seg-opt');
    g.set({ value: 'heat' }); g.el.dispatchEvent(key('ArrowRight')); assert.deepEqual(log, ['cool:true']); assert.deepEqual(o.map(x => x.getAttribute('aria-checked')), ['false', 'true', 'false']); assert.deepEqual(o.map(x => x.getAttribute('tabindex')), ['-1', '0', '-1']); assert.equal(document.activeElement, o[1]);
    g.el.dispatchEvent(key('End')); g.el.dispatchEvent(key('ArrowRight')); g.el.dispatchEvent(key('ArrowLeft')); g.el.dispatchEvent(key('Home')); assert.deepEqual(log.slice(1), ['off:true', 'heat:true', 'off:true', 'heat:true']);
    log.length = 0; o[1].dispatchEvent({ type: 'click' }); assert.deepEqual(log, ['cool:false']); assert.equal(o[1].getAttribute('aria-checked'), 'true'); g.destroy(); g.el.dispatchEvent(key('ArrowRight')); o[0].dispatchEvent({ type: 'click' }); assert.equal(log.length, 1);
    let ok = false; const b = E.prim.segmented(Object.assign({}, OPTS.segmented, { input: { enabled: () => ok, onBlocked: () => log.push('blocked'), onPick: () => log.push('pick') } })); log.length = 0; b.set({ value: 'heat' }); b.el.dispatchEvent(key('ArrowRight')); assert.deepEqual(log, ['blocked']); }],
  ['input (f): picker is a listbox button; set() paints text, expanded and label; click and ArrowDown open', () => { const { E, document } = bootI(), log = [];
    const bare = E.prim.picker(OPTS.picker); assert.equal(bare.el.getAttribute('data-hd-input'), null); const p = E.prim.picker(Object.assign({}, OPTS.picker, { input: { onActivate: () => log.push('open') } })); document.createElement('div').appendChild(p.el);
    assert.equal(p.el.tagName, 'BUTTON'); assert.equal(p.el.getAttribute('type'), 'button'); assert.equal(p.el.getAttribute('aria-haspopup'), 'listbox'); assert.equal(p.el.getAttribute('aria-expanded'), 'false'); assert.equal(p.el.getAttribute('aria-label'), 'Mode'); assert.equal(p.el.querySelector('.hd-pick-label').textContent, 'Heat');
    assert.equal(p.el.querySelector('svg').getAttribute('data-hd-icon'), 'chevron-down'); p.set({ text: 'Cool', expanded: true, label: 'Mode: Cool' }); assert.equal(p.el.getAttribute('aria-expanded'), 'true'); assert.equal(p.el.querySelector('.hd-pick-label').textContent, 'Cool'); assert.equal(p.el.getAttribute('aria-label'), 'Mode: Cool');
    p.set({}); assert.equal(p.el.querySelector('.hd-pick-label').textContent, 'Heat'); assert.equal(p.el.getAttribute('aria-expanded'), 'false'); assert.equal(p.el.getAttribute('aria-label'), 'Mode');
    p.set({ readonly: true }); assert.equal(p.el.getAttribute('aria-disabled'), 'true'); p.el.dispatchEvent({ type: 'click' }); p.el.dispatchEvent(key('ArrowDown')); assert.deepEqual(log, ['open', 'open']); p.destroy(); p.el.dispatchEvent(key('ArrowDown')); assert.equal(log.length, 2); }],
  ['input (g): pushButton({toggle}) is a switch that follows on; set({icon}) swaps the glyph', () => { const { E, document } = bootI(), log = [];
    const p = E.prim.pushButton({ label: 'Power', icon: 'bulb', toggle: true, input: { onActivate: () => log.push('act') } }); document.createElement('div').appendChild(p.el); const glyph = () => p.el.querySelector('svg').getAttribute('data-hd-icon');
    assert.equal(p.el.getAttribute('role'), 'switch'); assert.equal(p.el.getAttribute('aria-checked'), 'false'); p.set({ on: true }); assert.equal(p.el.getAttribute('aria-checked'), 'true'); p.set({ on: false, readonly: true }); assert.equal(p.el.getAttribute('aria-checked'), 'false');
    assert.equal(p.el.getAttribute('aria-readonly'), 'true'); assert.equal(glyph(), 'bulb'); p.set({ icon: 'check' }); assert.equal(glyph(), 'check'); assert.equal(p.el.querySelectorAll('svg').length, 1); assert.equal(p.el.children[0].localName, 'svg'); p.set({}); assert.equal(glyph(), 'bulb');
    p.el.dispatchEvent({ type: 'click' }); assert.deepEqual(log, ['act']); assert.equal(E.prim.pushButton({ label: 'x', toggle: true }).el.getAttribute('data-hd-input'), null); }],
  ['input (h): knob centre tap toggles, a centre drag is vertical, an outer drag is angular, keys and Enter work', () => { const { E, document } = bootI(), log = [];
    const input = { range: R100, onToggle: () => log.push('toggle'), onInput: x => log.push('in:' + x), onCommit: x => log.push('commit:' + x), onKey: x => log.push('key:' + x) };
    const k = E.prim.knob({ label: 'Dim', unit: '%', input }); document.createElement('div').appendChild(k.el); k.el.getBoundingClientRect = rect(200, 200); k.set({ value: 40 }); assert.equal(k.el.getAttribute('data-hd-input'), 'drag');
    k.el.dispatchEvent(pe('pointerdown', 100, 100)); k.el.dispatchEvent(pe('pointerup', 100, 100)); assert.deepEqual(log.splice(0), ['toggle']);
    k.el.dispatchEvent(pe('pointerdown', 100, 5)); k.el.dispatchEvent(pe('pointerup', 100, 5)); assert.deepEqual(log.splice(0), ['commit:50']); assert.equal(k.el.getAttribute('aria-valuenow'), '50');
    k.set({ value: 40 }); k.el.dispatchEvent(pe('pointerdown', 100, 100)); k.el.dispatchEvent(pe('pointermove', 100, 78)); k.set({ value: 0 }); assert.equal(k.el.getAttribute('aria-valuenow'), '50', 'input lock'); k.el.dispatchEvent(pe('pointerup', 100, 78)); assert.deepEqual(log.splice(0), ['in:50', 'commit:50']);
    k.set({ value: 40 }); k.el.dispatchEvent(pe('pointerdown', 100, 10)); k.el.dispatchEvent(pe('pointermove', 190, 100)); k.el.dispatchEvent(pe('pointermove', 100, 200)); k.el.dispatchEvent(pe('pointerup', 100, 200)); assert.deepEqual(log.splice(0), ['in:83', 'in:83', 'commit:83'], 'the dead zone keeps the last value');
    k.set({ value: 40 }); k.el.dispatchEvent(key('ArrowUp')); k.el.dispatchEvent(key('Enter')); k.el.dispatchEvent(key(' ')); assert.deepEqual(log.splice(0), ['key:41', 'toggle', 'toggle']); k.destroy(); k.el.dispatchEvent(key('Enter')); assert.deepEqual(log, []);
    let ok = false; const b = E.prim.knob({ label: 'x', input: Object.assign({}, input, { enabled: () => ok, onBlocked: () => log.push('blocked') }) }); b.el.getBoundingClientRect = rect(200, 200); b.el.dispatchEvent(pe('pointerdown', 100, 100)); b.el.dispatchEvent(pe('pointerup', 100, 100)); assert.deepEqual(log, ['blocked']); }],
  ['input (i): colour wheel with input is a slider; the pointer gives hue and saturation, the keys follow the ARIA table', () => { const { E, document } = bootI(), log = [];
    const input = { onInput: x => log.push('in:' + x.h + '/' + x.s), onCommit: x => log.push('commit:' + x.h + '/' + x.s), onKey: x => log.push('key:' + x.h + '/' + x.s) };
    const w = E.prim.colourWheel({ label: 'Colour', diameter: 120, input }); document.createElement('div').appendChild(w.el); w.el.getBoundingClientRect = rect(120, 120);
    assert.equal(w.el.getAttribute('role'), 'slider'); assert.equal(w.el.getAttribute('tabindex'), '0'); assert.equal(w.el.getAttribute('aria-valuemin'), '0'); assert.equal(w.el.getAttribute('aria-valuemax'), '360'); assert.equal(w.el.getAttribute('data-hd-input'), 'drag');
    w.set({ hue: 27, sat: 0.56 }); assert.equal(w.el.getAttribute('aria-valuenow'), '27'); assert.equal(w.el.getAttribute('aria-label'), 'Colour'); assert.ok(w.el.getAttribute('aria-valuetext').length > 0); w.set({ hue: 27, sat: 0.56, valueText: 'Hue 27, sat 56' }); assert.equal(w.el.getAttribute('aria-valuetext'), 'Hue 27, sat 56');
    w.el.dispatchEvent(pe('pointerdown', 120, 60)); w.el.dispatchEvent(pe('pointerup', 120, 60)); assert.deepEqual(log.splice(0), ['commit:90/100']); assert.equal(v(w.el, '--hd-wheel-x'), '1.0000');
    w.el.dispatchEvent(pe('pointerdown', 60, 60)); w.el.dispatchEvent(pe('pointermove', 60, 0)); w.set({ hue: 200, sat: 1 }); assert.equal(w.el.getAttribute('aria-valuenow'), '0', 'input lock'); w.el.dispatchEvent(pe('pointerup', 60, 0)); assert.deepEqual(log.splice(0), ['in:0/100', 'commit:0/100']);
    w.set({ hue: 27, sat: 0.56 }); w.el.dispatchEvent(key('ArrowRight')); w.el.dispatchEvent(key('ArrowLeft')); w.el.dispatchEvent(key('PageUp')); w.el.dispatchEvent(key('ArrowUp')); w.el.dispatchEvent(key('Home')); w.el.dispatchEvent(key('End'));
    assert.deepEqual(log.splice(0), ['key:32/56', 'key:27/56', 'key:57/56', 'key:57/61', 'key:57/0', 'key:57/100'], 'each key starts from the value it painted'); w.set({ hue: 3, sat: 0.5 }); w.el.dispatchEvent(key('ArrowLeft')); w.el.dispatchEvent(key('PageDown')); assert.deepEqual(log.splice(0), ['key:358/50', 'key:328/50']);
    w.destroy(); w.el.dispatchEvent(key('ArrowRight')); assert.deepEqual(log, []); }],
  ['input (j): ctStrip drags and keys on its own rect; End stops at the last grid value', () => { const { E, document } = bootI(), log = []; const ct = { min: 2202, max: 6535, step: 50 };
    const c = E.prim.ctStrip({ label: 'White', min: 2202, max: 6535, input: { range: () => ct, onKey: x => log.push('key:' + x), onInput: x => log.push('in:' + x), onCommit: x => log.push('commit:' + x) } }); document.createElement('div').appendChild(c.el); c.el.getBoundingClientRect = rect(400, 24);
    c.set({ value: 3000 }); c.el.dispatchEvent(key('End')); assert.deepEqual(log.splice(0), ['key:6502']); assert.equal(c.el.getAttribute('aria-valuetext'), '6502' + fc(0xa0) + 'K'); c.set({ min: 2202, max: 6535, value: 3000 });
    c.el.dispatchEvent(pe('pointerdown', 0, 5)); c.el.dispatchEvent(pe('pointerup', 0, 5)); assert.deepEqual(log.splice(0), ['commit:2202']); c.destroy(); c.el.dispatchEvent(key('End')); assert.deepEqual(log, []); }]
];
