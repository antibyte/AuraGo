// W1 widget tests (Batch C): each variant is hosted with a fake api on views that come from the contract demo entities
// (ui/testdata/homedash-m3/ha-entities.json, wire shape) through a real E.store snapshot frame.
import assert from 'node:assert/strict'; import { loadEngine, ENGINE_ORDER, read, eq } from './harness.mjs'; import { fakeClock } from './live-fakes.mjs';
const EN = JSON.parse(read('ui/lang/homedash/en.json'));
const WIRE = JSON.parse(read('ui/testdata/homedash-m3/ha-entities.json'));
const IDS = WIRE.map(e => e.id);
const pe = (type, x, y) => ({ type, pointerId: 1, isPrimary: true, button: 0, pointerType: 'touch', clientX: x, clientY: y, preventDefault() {} });
const click = { type: 'click', preventDefault() {} };
const key = (k, extra) => Object.assign({ type: 'keydown', key: k, defaultPrevented: false, preventDefault() { this.defaultPrevented = true; } }, extra);

// A fresh engine, a real store fed by one snapshot frame, and a host() wrapper with a recording fake api.
// patch: { [id]: wire => void } edits a deep copy of the demo entity before the snapshot.
export function env(patch) {
  const { E, document } = loadEngine(ENGINE_ORDER.concat(['hd-api.js', 'hd-store.js']), { dom: true }), clock = fakeClock(), frames = [], bus = E.core.emitter();
  const stream = { on: bus.on, off: bus.off, status: () => ({ phase: 'idle', reason: '' }), attach: () => ({ id: 'w1', set() {}, setActive() {}, detach() {} }) };
  const store = E.store.create({ api: { base: '/api/desktop/homedash' }, stream, raf: fn => { frames.push(fn); return frames.length; }, caf() {}, setTimeout: clock.setTimeout, clearTimeout: clock.clearTimeout, now: clock.now });
  store.watch('w1', { ids: IDS });
  const entities = WIRE.map(w => { const c = JSON.parse(JSON.stringify(w)); if (patch && patch[c.id]) patch[c.id](c); return c; });
  bus.emit('snapshot', { entities, sources: [{ source: 'ha', state: 'live', at: '2026-11-25T09:00:00Z' }] });
  const t = E.core.makeT(EN), calls = { commands: [], more: [], announced: [] };
  function mount(type, variant, id, o) {
    const opts = o || {}, state = { view: store.view(id) };
    const fns = () => ({ entity: () => state.view,
      can: (b, cap) => { const v = state.view; if (!v || v.readonly || v.available === false) return false; const c = (v.capabilities || []).find(x => x === cap || (x && x.name === cap)); return !!c && c.allowed !== false; },
      command: (...a) => { calls.commands.push(a); return Promise.resolve({ status: 'sent' }); }, moreInfo: (...a) => { calls.more.push(a); }, history: () => Promise.resolve({ points: [], source: 'ring' }), announce: m => { calls.announced.push(m); } });
    const widget = Object.assign({ id: 'w', type, variant, title: '', bindings: { primary: id } }, opts.widget);
    const h = E.widgets.host({ widget, size: opts.size || { w: 2, h: 1 }, t, api: fns, parent: document.body });
    return { h, state, q: s => h.el.querySelector(s), set(view) { state.view = view; h.update(); } };
  }
  return { E, document, store, view: id => store.view(id), mount, calls, t };
}
const wire = id => JSON.parse(JSON.stringify(WIRE.find(e => e.id === id)));
const readOnly = c => { c.readonly = true; c.readonly_reason = 'home_assistant.readonly'; for (const cap of c.capabilities) if (cap.name !== 'history') { cap.allowed = false; cap.blocked_by = 'home_assistant.readonly'; } };
const MID = ' ' + String.fromCodePoint(0xb7) + ' ';

export const tests = [
  ['w1 data: ha-entities.json holds the 11 contract entities in wire shape', () => {
    eq(IDS, ['ha:light.kitchen', 'ha:light.sofa', 'ha:switch.coffee', 'ha:input_boolean.vacation_mode', 'ha:scene.movie', 'ha:script.goodnight', 'ha:button.doorbell_test', 'ha:select.heater_mode', 'ha:input_select.radio', 'ha:number.boiler_target', 'ha:input_number.volume']);
    for (const e of WIRE) { assert.equal(e.source, 'ha'); assert.ok(e.state && typeof e.state.text === 'string' && typeof e.state.available === 'boolean' && e.state.updated_at, e.id); assert.equal(e.value, undefined, e.id); }
    const { view } = env(); assert.equal(view('ha:switch.coffee').value, false); assert.equal(view('ha:number.boiler_target').unit, String.fromCodePoint(0xb0) + 'C');
    assert.equal(view('ha:script.goodnight').readonly, true); assert.equal(view('ha:script.goodnight').readonlyReason, 'home_assistant.allowed_services'); assert.equal(view('ha:light.kitchen').attributes.brightness_pct, 40); }],
  ['switch: a pill click on an off switch sends turn_on (not toggle), an on switch sends turn_off', () => { const x = env(), w = x.mount('switch', 'pill', 'ha:switch.coffee');
    w.q('.hd-toggle').dispatchEvent(click); eq(x.calls.commands, [['primary', 'turn_on', {}]]);
    const on = x.view('ha:switch.coffee'); w.set(Object.assign({}, on, { value: true, text: 'on' })); assert.equal(w.q('.hd-toggle').getAttribute('aria-checked'), 'true');
    w.q('.hd-toggle').dispatchEvent(click); eq(x.calls.commands[1], ['primary', 'turn_off', {}]); assert.equal(x.calls.commands.length, 2); }],
  ['switch: toggle is only the fallback when the verbs are missing or the state is unknown', () => {
    const x = env({ 'ha:switch.coffee': c => { c.capabilities = c.capabilities.filter(k => k.name === 'toggle'); } }), w = x.mount('switch', 'rocker', 'ha:switch.coffee');
    w.q('.hd-rocker').dispatchEvent(click); eq(x.calls.commands, [['primary', 'toggle', {}]]);
    const y = env({ 'ha:switch.coffee': c => { c.state.value = null; c.state.text = 'unknown'; } }), v = y.mount('switch', 'push', 'ha:switch.coffee');
    v.q('.hd-push').dispatchEvent(click); eq(y.calls.commands, [['primary', 'toggle', {}]]); }],
  ['switch: a read-only entity sends nothing and announces the long reason, on all four variants', () => {
    for (const variant of ['pill', 'rocker', 'push', 'tile']) { const x = env({ 'ha:switch.coffee': readOnly }), w = x.mount('switch', variant, 'ha:switch.coffee'), node = w.q(variant === 'pill' ? '.hd-toggle' : variant === 'rocker' ? '.hd-rocker' : '.hd-push');
      node.dispatchEvent(click); assert.equal(x.calls.commands.length, 0, variant); eq(x.calls.announced, [EN['homedash.reason.ha_readonly_long']], variant);
      assert.equal(node.getAttribute('aria-disabled'), null, variant + ': read-only keeps full strength'); assert.equal(w.q('.hd-state-text').textContent, EN['homedash.state.off'] + MID + EN['homedash.reason.ha_readonly'], variant); } }],
  ['switch: push is a role=switch button with the On/Off word and the entity icon; tile hides the word', () => { const x = env(), w = x.mount('switch', 'push', 'ha:switch.coffee'), p = w.q('button.hd-push');
    assert.equal(p.getAttribute('role'), 'switch'); assert.equal(p.getAttribute('data-hd-variant'), 'chip'); assert.equal(p.getAttribute('aria-checked'), 'false'); assert.equal(p.querySelector('.hd-push-label').textContent, EN['homedash.state.off']);
    assert.equal(p.querySelector('svg').getAttribute('data-hd-icon'), 'outlet'); w.set(Object.assign({}, x.view('ha:switch.coffee'), { value: true })); assert.equal(p.getAttribute('aria-checked'), 'true'); assert.equal(p.querySelector('.hd-push-label').textContent, EN['homedash.state.on']);
    const tile = x.mount('switch', 'tile', 'ha:input_boolean.vacation_mode', { size: { w: 2, h: 2 } }), b = tile.q('button.hd-push');
    assert.equal(b.getAttribute('data-hd-variant'), 'tile'); assert.equal(b.getAttribute('role'), 'switch'); assert.equal(b.querySelector('.hd-push-label').textContent, ''); assert.equal(b.querySelector('svg').getAttribute('data-hd-icon'), 'power');
    assert.ok(tile.h.body.contains(b), 'the tile fills the body'); assert.equal(tile.h.body.querySelectorAll('.hd-push').length, 1); }],
  ['switch: aria-label is the entity name (the widget title wins)', () => { const x = env();
    for (const [variant, sel] of [['pill', '.hd-toggle'], ['rocker', '.hd-rocker'], ['push', '.hd-push'], ['tile', '.hd-push']]) assert.equal(x.mount('switch', variant, 'ha:switch.coffee').q(sel).getAttribute('aria-label'), 'Coffee machine', variant);
    assert.equal(x.mount('switch', 'pill', 'ha:switch.coffee', { widget: { title: 'Espresso' } }).q('.hd-toggle').getAttribute('aria-label'), 'Espresso'); }],
  ['switch: hold opens more-info for a light only, never for other switches', () => { const x = env(), sw = x.mount('switch', 'pill', 'ha:switch.coffee');
    sw.q('.hd-toggle').dispatchEvent(key('Enter', { shiftKey: true })); eq(x.calls.more, []); assert.equal(x.calls.commands.length, 0);
    const lt = x.mount('switch', 'pill', 'ha:light.sofa'); lt.q('.hd-toggle').dispatchEvent(key('Enter', { shiftKey: true })); eq(x.calls.more, [['primary']]); assert.equal(x.calls.commands.length, 0);
    lt.q('.hd-toggle').dispatchEvent(click); eq(x.calls.commands, [['primary', 'turn_on', {}]]);
    const none = x.mount('switch', 'pill', 'ha:light.sofa', { widget: { actions: { hold: { do: 'none' } } } }); none.q('.hd-toggle').dispatchEvent(key('ContextMenu')); assert.equal(x.calls.more.length, 1); }],
  ['switch: a tap action of more_info opens the sheet instead of sending', () => { const x = env(), w = x.mount('switch', 'pill', 'ha:light.sofa', { widget: { actions: { tap: { do: 'more_info' } } } });
    w.q('.hd-toggle').dispatchEvent(click); eq(x.calls.more, [['primary']]); assert.equal(x.calls.commands.length, 0); }],
  ['switch: unavailable entities show the host word and no affordance; the arm marker sits in the body', () => { const x = env({ 'ha:switch.coffee': c => { c.state = { value: null, text: 'unavailable', available: false, updated_at: c.state.updated_at }; } }), w = x.mount('switch', 'pill', 'ha:switch.coffee');
    assert.equal(w.q('.hd-toggle').getAttribute('aria-disabled'), 'true'); assert.equal(w.q('.hd-state-text').textContent, EN['homedash.state.unavailable']); w.q('.hd-toggle').dispatchEvent(click);
    assert.equal(x.calls.commands.length, 0); eq(x.calls.announced, [EN['homedash.state.unavailable']]); assert.equal(w.h.body.querySelectorAll('.hd-arm').length, 1); }],
  ['switch: M1 pill and rocker keep their markup; a missing capability disables, destroy unbinds', () => { const x = env(), pill = x.mount('switch', 'pill', 'ha:switch.coffee'), rk = x.mount('switch', 'rocker', 'ha:switch.coffee');
    assert.ok(pill.q('.hd-toggle-key') && pill.q('.hd-toggle-led') && pill.q('.hd-toggle-legend')); eq(rk.q('.hd-rocker').querySelectorAll('.hd-rocker-legend').map(n => n.textContent), ['I', 'O']);
    assert.equal(pill.q('.hd-toggle').getAttribute('data-hd-input'), 'press');
    const none = env({ 'ha:switch.coffee': c => { c.capabilities = c.capabilities.filter(k => k.name === 'history'); } }).mount('switch', 'pill', 'ha:switch.coffee'); assert.equal(none.q('.hd-toggle').getAttribute('aria-disabled'), 'true');
    const node = pill.q('.hd-toggle'); pill.h.destroy(); node.dispatchEvent(click); assert.equal(x.calls.commands.length, 0); }],

  // Task 12: light.
  ['light dial: brightness_pct is the truth (40 gives valuenow 40 and "On · 40 %"), no / 2.55 left in the module', () => { const x = env(), w = x.mount('light', 'dial', 'ha:light.kitchen', { size: { w: 3, h: 3 } }), d = w.q('.hd-dial');
    assert.equal(d.getAttribute('aria-valuenow'), '40'); assert.equal(w.q('.hd-state-text').textContent, EN['homedash.state.on'] + MID + '40 %'); assert.equal(d.getAttribute('aria-valuetext'), '40 %, ' + EN['homedash.state.on']);
    assert.equal(d.getAttribute('aria-label'), 'Kitchen: ' + EN['homedash.control.brightness']); assert.equal(d.getAttribute('data-hd-input'), 'drag');
    assert.ok(!/2\.55/.test(read('ui/js/homedash/widgets/hd-w-light.js')));
    w.set(Object.assign({}, x.view('ha:light.kitchen'), { attributes: { brightness_pct: null, updated_at: 'x' } })); assert.equal(d.getAttribute('aria-valuenow'), '100'); assert.equal(w.q('.hd-state-text').textContent, EN['homedash.state.on']); }],
  ['light dial: ArrowRight sends brightness 41 with phase key, Enter toggles, Shift+Enter opens more-info', () => { const x = env(), w = x.mount('light', 'dial', 'ha:light.kitchen', { size: { w: 3, h: 3 } }), d = w.q('.hd-dial');
    d.dispatchEvent(key('ArrowRight')); eq(x.calls.commands, [['primary', 'brightness', { value: 41 }, { phase: 'key' }]]);
    d.dispatchEvent(key('Enter')); eq(x.calls.commands[1], ['primary', 'turn_off', {}]); d.dispatchEvent(key('Enter', { shiftKey: true })); eq(x.calls.more, [['primary']]); assert.equal(x.calls.commands.length, 2); }],
  ['light dial: unavailable shows the en dash and never Off; a read-only light announces and sends nothing', () => {
    const x = env({ 'ha:light.kitchen': c => { c.state = { value: null, text: 'unavailable', available: false, updated_at: c.state.updated_at }; } }), w = x.mount('light', 'dial', 'ha:light.kitchen', { size: { w: 3, h: 3 } });
    assert.equal(w.q('.hd-dial').getAttribute('aria-valuenow'), null); assert.equal(w.q('.hd-dial').getAttribute('aria-valuetext'), String.fromCodePoint(0x2013)); assert.equal(w.q('.hd-state-text').textContent, EN['homedash.state.unavailable']);
    assert.ok(!/off/i.test(w.q('.hd-dial-readout').textContent));
    const y = env({ 'ha:light.kitchen': readOnly }), v = y.mount('light', 'dial', 'ha:light.kitchen', { size: { w: 3, h: 3 } }); v.q('.hd-dial').dispatchEvent(key('ArrowRight')); assert.equal(y.calls.commands.length, 0); eq(y.calls.announced, [EN['homedash.reason.ha_readonly_long']]); }],
  ['light slider: a tap on the track commits; on light.sofa the slider is disabled while the power chip still sends turn_on', () => { const x = env(), w = x.mount('light', 'slider', 'ha:light.kitchen'), s = w.q('.hd-slider');
    w.q('.hd-slider-track').getBoundingClientRect = () => ({ left: 0, top: 0, width: 200, height: 6, right: 200, bottom: 6 });
    assert.equal(w.q('.hd-lrow .hd-numeral .hd-num-int').textContent, '40');
    s.dispatchEvent(pe('pointerdown', 100, 3)); s.dispatchEvent(pe('pointerup', 100, 3)); eq(x.calls.commands, [['primary', 'brightness', { value: 50 }, { phase: 'commit' }]]);
    s.dispatchEvent(pe('pointerdown', 40, 3)); s.dispatchEvent(pe('pointermove', 60, 3)); s.dispatchEvent(pe('pointerup', 60, 3)); eq(x.calls.commands.slice(1).map(c => [c[1], c[2].value, c[3].phase]), [['brightness', 30, 'input'], ['brightness', 30, 'commit']]);
    assert.equal(w.q('.hd-lrow .hd-numeral .hd-num-int').textContent, '30');
    const y = env(), v = y.mount('light', 'slider', 'ha:light.sofa'); assert.equal(v.q('.hd-slider').getAttribute('aria-disabled'), 'true'); const chip = v.q('.hd-push');
    assert.equal(chip.getAttribute('role'), 'switch'); assert.equal(chip.getAttribute('aria-label'), 'Sofa lamp: ' + EN['homedash.control.power']); chip.dispatchEvent(click); eq(y.calls.commands, [['primary', 'turn_on', {}]]); }],
  ['light fader: numeral, vertical slider and power chip in a column', () => { const x = env(), w = x.mount('light', 'fader', 'ha:light.kitchen', { size: { w: 1, h: 3 } }), col = w.q('.hd-lcol');
    assert.ok(col && col.querySelector('.hd-numeral') && col.querySelector('.hd-slider[data-hd-variant="v"]') && col.querySelector('.hd-push')); assert.equal(col.querySelector('.hd-slider').getAttribute('aria-valuenow'), '40');
    col.querySelector('.hd-slider').dispatchEvent(key('End')); eq(x.calls.commands, [['primary', 'brightness', { value: 100 }, { phase: 'key' }]]); }],
  ['light tile: switch on sofa, dimmer slider on kitchen; tap toggles, drag sends input then commit, hold opens more-info', () => {
    const x = env(), s = x.mount('light', 'tile', 'ha:light.sofa', { size: { w: 2, h: 2 } }), t0 = s.q('.hd-ltile'); assert.equal(t0.getAttribute('role'), 'switch'); assert.equal(t0.getAttribute('aria-checked'), 'false'); assert.equal(t0.getAttribute('aria-roledescription'), null);
    const w = x.mount('light', 'tile', 'ha:light.kitchen', { size: { w: 2, h: 2 } }), tile = w.q('.hd-ltile'); tile.getBoundingClientRect = () => ({ left: 0, top: 0, width: 200, height: 200, right: 200, bottom: 200 });
    assert.equal(tile.getAttribute('role'), 'slider'); assert.equal(tile.getAttribute('aria-roledescription'), EN['homedash.control.dimmer']); assert.equal(tile.getAttribute('aria-valuenow'), '40'); assert.equal(tile.getAttribute('tabindex'), '0');
    assert.equal(tile.style.getPropertyValue('--hd-value'), '0.4000'); assert.ok(tile.querySelector('.hd-ltile-wash') && tile.querySelector('.hd-iconbadge') && tile.querySelector('.hd-numeral'));
    tile.dispatchEvent(pe('pointerdown', 100, 100)); tile.dispatchEvent(pe('pointerup', 100, 100)); eq(x.calls.commands, [['primary', 'turn_off', {}]]);
    x.calls.commands.length = 0; tile.dispatchEvent(pe('pointerdown', 80, 100)); tile.dispatchEvent(pe('pointermove', 140, 100)); assert.equal(tile.style.getPropertyValue('--hd-value'), '0.7000'); tile.dispatchEvent(pe('pointerup', 140, 100));
    eq(x.calls.commands.map(c => [c[1], c[2].value, c[3].phase]), [['brightness', 70, 'input'], ['brightness', 70, 'commit']]);
    w.set(x.view('ha:light.kitchen')); x.calls.commands.length = 0; tile.dispatchEvent(key('ArrowLeft')); eq(x.calls.commands, [['primary', 'brightness', { value: 39 }, { phase: 'key' }]]); tile.dispatchEvent(key(' ')); eq(x.calls.commands[1], ['primary', 'turn_off', {}]);
    tile.dispatchEvent(key('ContextMenu')); eq(x.calls.more, [['primary']]); t0.dispatchEvent(key('Enter', { shiftKey: true })); assert.equal(x.calls.more.length, 2);
    t0.dispatchEvent(key('Enter')); eq(x.calls.commands[2], ['primary', 'turn_on', {}]); }],
  ['light tile: unavailable reads the en dash; the tile never calls it Off; every light variant has a more-info hold', () => {
    const x = env({ 'ha:light.kitchen': c => { c.state = { value: null, text: 'unavailable', available: false, updated_at: c.state.updated_at }; } }), w = x.mount('light', 'tile', 'ha:light.kitchen', { size: { w: 2, h: 2 } });
    assert.equal(w.q('.hd-ltile .hd-num-int').textContent, String.fromCodePoint(0x2013)); assert.equal(w.q('.hd-ltile').getAttribute('aria-disabled'), 'true'); assert.equal(w.q('.hd-state-text').textContent, EN['homedash.state.unavailable']);
    const y = env(); for (const [variant, sel] of [['slider', '.hd-slider'], ['fader', '.hd-slider'], ['tile', '.hd-ltile'], ['dial', '.hd-dial']]) { y.calls.more.length = 0; y.mount('light', variant, 'ha:light.kitchen', { size: { w: 3, h: 3 } }).q(sel).dispatchEvent(key('ContextMenu')); assert.equal(y.calls.more.length, 1, variant); } }]
];
