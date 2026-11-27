// Climate dial commands (G13 round 5): the knob, the plus and minus keys and the mode bar send set_temperature {value} and hvac_mode {option} with the phases of the other W1
// controls; range and step come from the capability and the attributes; read-only, blocked, design mode and the off mode send nothing.
import assert from 'node:assert/strict'; import { loadEngine, ENGINE_ORDER, read, eq } from './harness.mjs';
const EN = JSON.parse(read('ui/lang/homedash/en.json'));
const DEG = String.fromCodePoint(0xb0);
const view = p => Object.assign({ id: 'x:1', kind: 'climate', name: 'Living', value: 'heat', unit: DEG + 'C', attributes: { temperature: 21.5, current_temperature: 19.5, min_temp: 5, max_temp: 30, target_temp_step: 0.5, hvac_action: 'heating', hvac_modes: ['off', 'heat', 'cool', 'auto'], updated_at: new Date().toISOString() },
    available: true, status: 'ok', readonly: false, capabilities: [], pending: false, stale: false, error: null, tone: null }, p);
function mount(views, o) {
    const { E, document } = loadEngine(ENGINE_ORDER, { dom: true }), t = E.core.makeT(EN, { warn() {} }), state = Object.assign({}, views), calls = [], announced = [], opts = o || {};
    const api = () => ({ entity: b => state[b] || null, can: (b, cap) => opts.can ? opts.can(cap) : true, command: (...a) => { calls.push(a); return Promise.resolve({}); }, moreInfo() {}, history: () => Promise.resolve({ points: [], source: 'ring' }), announce: m => announced.push(m) });
    const h = E.widgets.host({ widget: { id: 'w', type: 'climate', variant: 'dial', title: 'Title', bindings: { primary: 'x:1' }, options: {} }, size: { w: 2, h: 3 }, t, api });
    document.body.appendChild(h.el);
    return { E, h, state, calls, announced, q: s => h.el.querySelector(s), all: s => Array.from(h.el.querySelectorAll(s)), set(v) { state.primary = v; h.update(); } };
}
const key = (k, extra) => Object.assign({ type: 'keydown', key: k, defaultPrevented: false, preventDefault() { this.defaultPrevented = true; } }, extra);
const pe = (type, x, y) => ({ type, pointerId: 1, isPrimary: true, button: 0, pointerType: 'touch', clientX: x, clientY: y, preventDefault() {} });
const click = { type: 'click', preventDefault() {} };
const rect = () => ({ left: 0, top: 0, width: 200, height: 200, right: 200, bottom: 200 });
const step = (w, dir) => w.q('.hd-climate-steps [data-dir="' + dir + '"]');

export const tests = [
    ['climate dial: arrow keys on the knob send set_temperature with phase key inside the entity range and step; Home and End reach the limits', () => {
        const w = mount({ primary: view({}) }), dial = w.q('.hd-dial');
        dial.dispatchEvent(key('ArrowUp')); dial.dispatchEvent(key('End'));
        eq(w.calls, [['primary', 'set_temperature', { value: 22 }, { phase: 'key' }], ['primary', 'set_temperature', { value: 30 }, { phase: 'key' }]]); }],
    ['climate dial: a drag on the ring sends input while moving and commit at the end, a tap on the ring commits; a tap in the centre sends nothing', () => {
        const w = mount({ primary: view({}) }), dial = w.q('.hd-dial'); dial.getBoundingClientRect = rect;
        dial.dispatchEvent(pe('pointerdown', 100, 10)); dial.dispatchEvent(pe('pointermove', 190, 100)); dial.dispatchEvent(pe('pointerup', 190, 100));
        assert.deepEqual(w.calls.map(c => c[3].phase), ['input', 'commit']); assert.equal(w.calls[0][1], 'set_temperature'); assert.equal(w.calls[0][2].value, w.calls[1][2].value);
        assert.ok(w.calls[1][2].value > 25 && w.calls[1][2].value <= 30 && (w.calls[1][2].value * 2) % 1 === 0, 'on the 0.5 grid of the range: ' + w.calls[1][2].value);
        w.calls.length = 0; dial.dispatchEvent(pe('pointerdown', 100, 100)); dial.dispatchEvent(pe('pointerup', 100, 100)); eq(w.calls, []);
        dial.dispatchEvent(pe('pointerdown', 100, 10)); dial.dispatchEvent(pe('pointerup', 100, 10)); eq(w.calls.map(c => c[3].phase), ['commit']); }],
    ['climate dial: the plus and minus keys step by the entity step, stop at the limits and send phase key', () => {
        const w = mount({ primary: view({ attributes: Object.assign({}, view().attributes, { temperature: 21, target_temp_step: 1 }) }) });
        step(w, 1).dispatchEvent(click); step(w, -1).dispatchEvent(click);
        eq(w.calls, [['primary', 'set_temperature', { value: 22 }, { phase: 'key' }], ['primary', 'set_temperature', { value: 21 }, { phase: 'key' }]]);
        w.calls.length = 0; w.set(view({ attributes: Object.assign({}, view().attributes, { temperature: 30 }) })); step(w, 1).dispatchEvent(click); eq(w.calls, []);
        w.set(view({ attributes: Object.assign({}, view().attributes, { temperature: 5 }) })); step(w, -1).dispatchEvent(click); eq(w.calls, []); }],
    ['climate dial: the capability range overrides the attributes and the knob is built on it', () => {
        const w = mount({ primary: view({ capabilities: [{ name: 'set_temperature', allowed: true, min: 16, max: 24, step: 0.5 }] }) }), dial = w.q('.hd-dial');
        assert.equal(dial.getAttribute('aria-valuemin'), '16'); assert.equal(dial.getAttribute('aria-valuemax'), '24');
        dial.dispatchEvent(key('End')); eq(w.calls, [['primary', 'set_temperature', { value: 24 }, { phase: 'key' }]]); }],
    ['climate dial: read-only, blocked capability, an unavailable entity, the off mode and design mode (api.can false) send nothing', () => {
        const none = (v, o, msg) => { const w = mount({ primary: v }, o); w.q('.hd-dial').dispatchEvent(key('ArrowUp')); step(w, 1).dispatchEvent(click); w.q('.hd-seg-opt[data-hd-option="cool"]').dispatchEvent(click); eq(w.calls, [], msg); return w; };
        none(view({ readonly: true }), null, 'read-only'); none(view({}), { can: () => false }, 'blocked or design mode');
        none(view({ available: false, status: 'unavailable' }), null, 'unavailable');
        const off = mount({ primary: view({ value: 'off', attributes: Object.assign({}, view().attributes, { hvac_action: 'off' }) }) }); off.q('.hd-dial').dispatchEvent(key('ArrowUp')); step(off, 1).dispatchEvent(click); eq(off.calls, [], 'the off mode');
        assert.equal(off.q('.hd-dial').getAttribute('aria-disabled'), 'true'); assert.ok(off.announced.includes(EN['homedash.climate.mode_off']), 'the blocked key press says why');
        off.q('.hd-seg-opt[data-hd-option="heat"]').dispatchEvent(click); eq(off.calls, [['primary', 'hvac_mode', { option: 'heat' }, { phase: 'discrete' }]], 'the mode bar still turns it on'); }],
    ['climate dial: the mode bar sends hvac_mode {option}; a click is discrete, arrow keys are key, the current mode and a blocked capability send nothing', () => {
        const w = mount({ primary: view({ capabilities: [{ name: 'set_temperature', allowed: true }, { name: 'hvac_mode', allowed: true }] }) });
        assert.equal(w.q('.hd-seg-opt[data-hd-option="heat"]').getAttribute('aria-checked'), 'true');
        w.q('.hd-seg-opt[data-hd-option="heat"]').dispatchEvent(click); eq(w.calls, [], 'the current mode');
        w.q('.hd-seg-opt[data-hd-option="cool"]').dispatchEvent(click); eq(w.calls, [['primary', 'hvac_mode', { option: 'cool' }, { phase: 'discrete' }]]);
        w.calls.length = 0; w.q('.hd-seg').dispatchEvent(key('ArrowRight')); eq(w.calls, [['primary', 'hvac_mode', { option: 'auto' }, { phase: 'key' }]], 'one step on from the picked cool');
        const blocked = mount({ primary: view({}) }, { can: cap => cap === 'set_temperature' }); blocked.q('.hd-seg-opt[data-hd-option="cool"]').dispatchEvent(click); eq(blocked.calls, [], 'no hvac_mode capability');
        assert.equal(blocked.q('.hd-seg').getAttribute('aria-disabled'), 'true'); blocked.q('.hd-dial').dispatchEvent(key('ArrowUp')); assert.equal(blocked.calls.length, 1, 'the target is still settable'); }],
    ['climate dial: the mode bar shows only the modes the entity offers and rebuilds when the list changes', () => {
        const w = mount({ primary: view({ attributes: Object.assign({}, view().attributes, { hvac_modes: ['off', 'heat'] }) }) });
        eq(w.all('.hd-seg-opt').map(b => b.getAttribute('data-hd-option')), ['off', 'heat']);
        w.set(view({ capabilities: [{ name: 'hvac_mode', allowed: true, options: ['heat', 'cool', 'auto', 'dry'] }] }));
        eq(w.all('.hd-seg-opt').map(b => b.getAttribute('data-hd-option')), ['heat', 'cool', 'auto'], 'the capability options win; modes the bar has no button for are left out'); assert.equal(w.all('.hd-seg').length, 1);
        w.set(view({ available: false, status: 'unavailable' })); eq(w.all('.hd-seg-opt').map(b => b.getAttribute('data-hd-option')), ['heat', 'cool', 'auto'], 'a dead entity keeps the last bar');
        w.set(view({ attributes: Object.assign({}, view().attributes, { hvac_modes: undefined }) })); eq(w.all('.hd-seg-opt').length, 4, 'no list: all four'); }],
    ['climate stepper and scale: the off mode disables and sends nothing, like the dial', () => {
        for (const variant of ['stepper', 'scale']) {
            const { E, document } = loadEngine(ENGINE_ORDER, { dom: true }), t = E.core.makeT(EN, { warn() {} }), calls = [], v = view({ value: 'off' });
            const h = E.widgets.host({ widget: { id: 'w', type: 'climate', variant, title: 'Title', bindings: { primary: 'x:1' }, options: {} }, size: { w: 2, h: 3 }, t,
                api: () => ({ entity: () => v, can: () => true, command: (...a) => { calls.push(a); return Promise.resolve({}); }, moreInfo() {}, history: () => Promise.resolve({ points: [], source: 'ring' }), announce() {} }) });
            document.body.appendChild(h.el); const node = h.el.querySelector(variant === 'stepper' ? '.hd-step-value' : '.hd-slider');
            node.dispatchEvent(key('ArrowUp')); const inc = h.el.querySelector('.hd-step[data-dir="1"]'); if (inc) inc.dispatchEvent(click); eq(calls, [], variant); } }],
];
