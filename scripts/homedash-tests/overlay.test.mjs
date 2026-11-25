import assert from 'node:assert/strict'; import { loadEngine, ENGINE_ORDER, read, eq } from './harness.mjs';
// Overlays of Task 9: E.sheet.trap, E.popover.listbox, E.moreinfo.build and create. The fake DOM has no event propagation, so keys go straight to the node that owns the listener.
const boot = () => loadEngine(ENGINE_ORDER.concat(['hd-sheet.js']), { dom: true });
const EN = JSON.parse(read('ui/lang/homedash/en.json'));
const key = (k, extra) => Object.assign({ type: 'keydown', key: k, shiftKey: false, preventDefault() {}, stopPropagation() {} }, extra);
const rect = (w, h, x = 0, y = 0) => () => ({ x, y, left: x, top: y, width: w, height: h, right: x + w, bottom: y + h });
function stage(document, width) {
  const root = document.createElement('div'), main = document.createElement('main'), live = document.createElement('div'), opener = document.createElement('button');
  live.setAttribute('aria-live', 'polite'); root.append(main, live); main.appendChild(opener); document.body.appendChild(root); root.getBoundingClientRect = rect(width || 800, 600); opener.focus();
  return { root, main, live, opener };
}
// Store-style views (hd-store view(): flattened value/available/attributes, capabilities, readonly, readonlyReason) from the contract's demo entities.
const flat = (w, extra) => Object.assign({ id: w.id, source: w.source, kind: w.kind, name: w.name, area: w.area || '', unit: '', device_class: '', icon_hint: w.icon_hint || '', value: w.state.value, text: w.state.text,
  attributes: Object.assign({}, w.state.attributes, { updated_at: w.state.updated_at }), available: w.state.available, status: w.state.available ? 'ok' : 'unavailable', readonly: w.readonly, readonlyReason: '',
  capabilities: w.capabilities, pending: false, stale: false, error: null, tone: null }, extra);
const KITCHEN = { id: 'ha:light.kitchen', source: 'ha', kind: 'light', name: 'Kitchen', area: 'Kitchen', icon_hint: 'bulb',
  capabilities: [{ name: 'toggle', allowed: true }, { name: 'turn_on', allowed: true }, { name: 'turn_off', allowed: true }, { name: 'brightness', allowed: true, min: 0, max: 100, step: 1 },
    { name: 'color_temp', allowed: true, min: 2202, max: 6535, step: 50 }, { name: 'color', allowed: true }, { name: 'effect', allowed: true, options: ['none', 'colorloop'] }], readonly: false,
  state: { value: true, text: 'on', attributes: { brightness_pct: 40, color_mode: 'color_temp', color_temp_kelvin: 3003, effect: 'none', effect_list: ['none', 'colorloop'], hs_color: [27.2, 56.1], max_color_temp_kelvin: 6535, min_color_temp_kelvin: 2202, supported_color_modes: ['color_temp', 'hs'] }, available: true, updated_at: '2026-11-25T09:00:00Z' } };
const SOFA = { id: 'ha:light.sofa', source: 'ha', kind: 'light', name: 'Sofa lamp', area: 'Living room', icon_hint: 'bulb',
  capabilities: [{ name: 'toggle', allowed: true }, { name: 'turn_on', allowed: true }, { name: 'turn_off', allowed: true }], readonly: false,
  state: { value: false, text: 'off', attributes: { color_mode: null, supported_color_modes: ['onoff'] }, available: true, updated_at: '2026-11-25T09:00:00Z' } };
function fakeApi(E, st) {
  const calls = [], said = [];
  const api = { widget: { id: 'w1', type: 'light', title: '' }, t: E.core.makeT(EN), lang: 'en', entity: () => st.view, can: (b, c) => { const x = E.control.cap(st.view, c); return !!x && x.allowed !== false; },
    command: (...a) => { calls.push(a); return Promise.resolve({}); }, announce: m => said.push(m), moreInfo() {} };
  return { api, calls, said };
}
const rowsOf = node => node.querySelectorAll('[data-row]').filter(r => !r.hasAttribute('hidden')).map(r => r.getAttribute('data-row'));
const MID = ' ' + String.fromCodePoint(0xb7) + ' ';

export const tests = [
  ['overlay: trap inerts the other children except the live region, cycles Tab, runs onEscape and onBackdrop, and release restores everything', () => {
    const { E, document } = boot(), { root, main, live, opener } = stage(document), layer = document.createElement('div'), a = document.createElement('button'), b = document.createElement('button'), off = document.createElement('button');
    off.setAttribute('disabled', ''); layer.append(a, off, b); root.appendChild(layer);
    let esc = 0, back = 0; const release = E.sheet.trap({ container: root, layer, dialog: layer, onEscape: () => { esc++; }, onBackdrop: () => { back++; } });
    assert.equal(main.getAttribute('inert'), ''); assert.equal(live.hasAttribute('inert'), false); assert.equal(layer.hasAttribute('inert'), false);
    a.focus(); layer.dispatchEvent(key('Tab')); assert.equal(document.activeElement, b); layer.dispatchEvent(key('Tab')); assert.equal(document.activeElement, a);
    layer.dispatchEvent(key('Tab', { shiftKey: true })); assert.equal(document.activeElement, b);
    layer.dispatchEvent(key('Escape')); assert.equal(esc, 1); layer.dispatchEvent({ type: 'click', target: a }); assert.equal(back, 0); layer.dispatchEvent({ type: 'click', target: layer }); assert.equal(back, 1);
    release(); assert.equal(main.hasAttribute('inert'), false); assert.equal(document.activeElement, opener);
    layer.dispatchEvent(key('Escape')); assert.equal(esc, 1); release();   // released: no more handlers, a second release is harmless
    assert.equal(typeof E.sheet.create, 'function'); }],
  ['overlay: a trap keeps what was already inert and takes an explicit opener', () => {
    const { E, document } = boot(), { root, main, opener } = stage(document), other = document.createElement('button'), layer = document.createElement('div'), x = document.createElement('button');
    main.setAttribute('inert', ''); layer.appendChild(x); root.append(layer, other); other.focus();
    const release = E.sheet.trap({ container: root, layer, dialog: layer, opener }); assert.equal(other.getAttribute('inert'), '');
    release(); assert.equal(main.getAttribute('inert'), ''); assert.equal(other.hasAttribute('inert'), false); assert.equal(document.activeElement, opener); }],

  ['overlay: listbox marks the value, moves with the arrows, picks with Enter, removes the panel and refocuses the anchor', () => {
    const { E, document } = boot(), { root, opener } = stage(document), picked = []; opener.getBoundingClientRect = rect(150, 44, 20, 100);
    const pop = E.popover.listbox({ root, anchor: opener, family: 'tactile', label: 'Mode', options: [{ id: 'a', label: 'Alpha' }, { id: 'b', label: 'Bravo' }, { id: 'c', label: 'Charlie' }], value: 'b', onPick: v => picked.push(v) });
    const list = root.querySelector('[role="listbox"]'), opts = root.querySelectorAll('[role="option"]');
    assert.equal(opts.length, 3); assert.equal(list.getAttribute('aria-activedescendant'), opts[1].id); assert.equal(opts[1].getAttribute('aria-selected'), 'true'); assert.equal(opts[0].getAttribute('aria-selected'), 'false');
    assert.equal(document.activeElement, list); assert.equal(opts[2].querySelector('.hd-option-label').textContent, 'Charlie');
    assert.equal(pop.el.getAttribute('data-kind'), 'listbox'); assert.ok(pop.el.classList.contains('hd-widget')); assert.ok(pop.el.classList.contains('hd-sheet')); assert.equal(pop.el.getAttribute('data-hd-family'), 'tactile');
    assert.equal(pop.el.getAttribute('data-hd-pop'), 'anchored'); assert.equal(pop.el.style.getPropertyValue('--hd-pop-w'), '200px'); assert.equal(pop.el.style.getPropertyValue('--hd-pop-x'), '20px');
    list.dispatchEvent(key('ArrowDown')); assert.equal(list.getAttribute('aria-activedescendant'), opts[2].id); list.dispatchEvent(key('ArrowDown')); assert.equal(list.getAttribute('aria-activedescendant'), opts[2].id);
    list.dispatchEvent(key('Home')); assert.equal(list.getAttribute('aria-activedescendant'), opts[0].id); list.dispatchEvent(key('c')); assert.equal(list.getAttribute('aria-activedescendant'), opts[2].id);
    list.dispatchEvent(key('Enter')); assert.deepEqual(picked, ['c']); assert.equal(root.querySelector('[role="listbox"]'), null); assert.equal(document.activeElement, opener); }],
  ['overlay: listbox Escape, Tab, an outside pointer and a click on an option behave as the listbox pattern says', () => {
    const { E, document } = boot(), { root, opener, main } = stage(document), picked = [], closed = [], o = { root, anchor: opener, options: [{ id: 'a', label: 'A' }, { id: 'b', label: 'B' }], value: 'a', onPick: v => picked.push(v), onClose: () => closed.push(1) };
    E.popover.listbox(o); root.querySelector('[role="listbox"]').dispatchEvent(key('Escape')); assert.deepEqual(picked, []); assert.equal(root.querySelector('[role="listbox"]'), null); assert.equal(document.activeElement, opener); assert.equal(closed.length, 1);
    E.popover.listbox(o); root.querySelector('[role="listbox"]').dispatchEvent(key('Tab')); assert.equal(root.querySelector('[role="listbox"]'), null); assert.equal(document.activeElement, opener);
    E.popover.listbox(o); main.focus(); document.dispatchEvent({ type: 'pointerdown', target: main }); assert.equal(root.querySelector('[role="listbox"]'), null); assert.equal(document.activeElement, main);
    E.popover.listbox(o); document.dispatchEvent({ type: 'pointerdown', target: opener }); assert.ok(root.querySelector('[role="listbox"]'), 'a press on the trigger leaves the toggle to the trigger');
    const opt = root.querySelectorAll('[role="option"]')[1]; root.querySelector('[role="listbox"]').dispatchEvent({ type: 'click', target: opt.querySelector('.hd-option-label') }); assert.deepEqual(picked, ['b']); assert.equal(root.querySelector('[role="listbox"]'), null); }],
  ['overlay: a second listbox on the same root closes the first, a narrow root makes a bottom sheet and the panel opens above when there is no room below', () => {
    const { E, document } = boot(), { root, opener } = stage(document, 500), opts = [{ id: 'a', label: 'A' }, { id: 'b', label: 'B' }];
    const first = E.popover.listbox({ root, anchor: opener, options: opts, value: 'a', onPick() {} }), second = E.popover.listbox({ root, anchor: opener, options: opts, value: 'b', onPick() {} });
    assert.equal(root.querySelectorAll('[role="listbox"]').length, 1); assert.equal(root.querySelectorAll('.hd-pop-layer').length, 1); assert.equal(second.el.getAttribute('data-hd-pop'), 'sheet'); assert.equal(typeof first.close, 'function');
    assert.equal(E.popover.isOpen(root), true); E.popover.close(root); assert.equal(E.popover.isOpen(root), false); assert.equal(root.querySelector('[role="listbox"]'), null);
    root.getBoundingClientRect = rect(800, 300); opener.getBoundingClientRect = rect(120, 44, 10, 250); const third = E.popover.listbox({ root, anchor: opener, options: opts, value: 'a', onPick() {} });
    assert.equal(third.el.getAttribute('data-hd-pop'), 'anchored'); assert.ok(Number.parseInt(third.el.style.getPropertyValue('--hd-pop-y'), 10) < 250, 'above the anchor'); }],

  ['overlay: the light sheet shows power, brightness, colour temperature, colour and effect rows for the kitchen light and follows the entity', () => {
    const { E, document } = boot(), st = { view: flat(KITCHEN) }, { api, calls } = fakeApi(E, st), { root } = stage(document), t = api.t;
    const mi = E.moreinfo.build({ family: 'glass', api }); root.appendChild(mi.el); mi.update();
    assert.equal(mi.el.getAttribute('role'), 'dialog'); assert.equal(mi.el.getAttribute('aria-modal'), 'true'); assert.equal(mi.el.getAttribute('data-kind'), 'moreinfo'); assert.equal(mi.el.getAttribute('data-hd-family'), 'glass');
    assert.ok(mi.el.classList.contains('hd-widget')); assert.equal(mi.el.getAttribute('data-hd-type'), 'light'); assert.equal(mi.el.getAttribute('aria-labelledby'), mi.el.querySelector('.hd-title').id);
    assert.deepEqual(rowsOf(mi.el), ['power', 'bright', 'ct', 'wheel', 'effect']); assert.equal(mi.el.querySelector('.hd-title').textContent, 'Kitchen'); assert.equal(mi.el.querySelector('.hd-sub').textContent, 'Kitchen');
    const toggle = mi.el.querySelector('.hd-toggle'), slider = mi.el.querySelector('.hd-slider'), ct = mi.el.querySelector('.hd-ct'), wheel = mi.el.querySelector('.hd-wheel'), pick = mi.el.querySelector('.hd-pick');
    assert.equal(toggle.getAttribute('aria-checked'), 'true'); assert.equal(slider.getAttribute('aria-valuenow'), '40'); assert.equal(slider.getAttribute('aria-valuetext'), '40 %, On');
    assert.equal(ct.getAttribute('aria-valuenow'), '3003'); assert.equal(ct.getAttribute('aria-valuemin'), '2202'); assert.equal(ct.getAttribute('aria-valuemax'), '6535');
    assert.equal(wheel.getAttribute('aria-valuenow'), '27'); assert.equal(wheel.getAttribute('aria-valuetext'), 'Hue 27' + String.fromCodePoint(0xb0) + ', saturation 56 %'); assert.equal(pick.querySelector('.hd-pick-label').textContent, 'none');
    assert.equal(mi.el.querySelector('.hd-state-text').textContent, 'On'); assert.equal(mi.el.getAttribute('data-hd-on'), 'true');
    const foot = mi.el.querySelector('.hd-foot'); assert.equal(foot.hasAttribute('hidden'), false); assert.ok(foot.textContent.includes('Source: Home Assistant')); assert.ok(foot.textContent.includes('updated')); assert.ok(!foot.textContent.includes(EN['homedash.reason.homedash_readonly_long']));
    // the last child of the dialog is the close button
    const kids = mi.el.children, close = kids[kids.length - 1]; assert.ok(close.classList.contains('hd-sheet-close')); assert.equal(close.getAttribute('aria-label'), t('homedash.moreinfo.close')); assert.ok(close.querySelector('svg[data-hd-icon="close"]'));
    // controls send through api.command
    toggle.dispatchEvent({ type: 'click', preventDefault() {} }); eq(calls.pop(), ['primary', 'turn_off', {}]);
    slider.dispatchEvent(key('ArrowRight')); eq(calls.pop(), ['primary', 'brightness', { value: 41 }, { phase: 'key' }]);
    ct.dispatchEvent(key('ArrowRight')); const c = calls.pop(); eq([c[1], c[3]], ['color_temp', { phase: 'key' }]); assert.equal(c[2].kelvin, 3052);
    wheel.dispatchEvent(key('ArrowRight')); const w = calls.pop(); assert.equal(w[1], 'color'); assert.equal(Math.round(w[2].h), 32); eq(w[3], { phase: 'key' });
    // the entity changes: off, and the rows keep following
    st.view = flat(KITCHEN, { value: false, text: 'off', attributes: Object.assign({}, KITCHEN.state.attributes, { brightness_pct: null, color_mode: null }) }); mi.update();
    assert.equal(toggle.getAttribute('aria-checked'), 'false'); assert.equal(slider.getAttribute('aria-valuenow'), '0'); assert.equal(mi.el.querySelector('.hd-state-text').textContent, 'Off'); assert.equal(ct.hasAttribute('aria-valuenow'), false);
    mi.destroy(); assert.equal(root.querySelector('.hd-sheet'), null); }],
  ['overlay: the effect row opens a listbox in the popover root and sends the chosen option', () => {
    const { E, document } = boot(), st = { view: flat(KITCHEN) }, { api, calls } = fakeApi(E, st), { root } = stage(document), host = document.createElement('div'); root.appendChild(host);
    const mi = E.moreinfo.build({ family: 'minimal', api, popoverRoot: host }); host.appendChild(mi.el); mi.update(); const pick = mi.el.querySelector('.hd-pick');
    pick.dispatchEvent({ type: 'click', preventDefault() {} }); const list = host.querySelector('[role="listbox"]'); assert.ok(list); assert.equal(pick.getAttribute('aria-expanded'), 'true');
    assert.deepEqual(host.querySelectorAll('[role="option"]').map(n => n.textContent), ['none', 'colorloop']); assert.equal(host.querySelector('.hd-sheet[data-kind="listbox"]').getAttribute('data-hd-family'), 'minimal');
    list.dispatchEvent(key('ArrowDown')); list.dispatchEvent(key('Enter')); eq(calls.pop(), ['primary', 'effect', { option: 'colorloop' }]); assert.equal(pick.getAttribute('aria-expanded'), 'false'); assert.equal(document.activeElement, pick);
    pick.dispatchEvent({ type: 'click', preventDefault() {} }); pick.dispatchEvent({ type: 'click', preventDefault() {} }); assert.equal(host.querySelector('[role="listbox"]'), null, 'a second click on the trigger closes it'); mi.destroy(); }],
  ['overlay: an on/off light (sofa) shows only the power row', () => {
    const { E, document } = boot(), st = { view: flat(SOFA) }, { api, calls } = fakeApi(E, st), { root } = stage(document), mi = E.moreinfo.build({ family: 'glass', api }); root.appendChild(mi.el); mi.update();
    assert.deepEqual(rowsOf(mi.el), ['power']); assert.equal(mi.el.querySelector('.hd-toggle').getAttribute('aria-checked'), 'false'); assert.equal(mi.el.querySelector('.hd-title').textContent, 'Sofa lamp');
    mi.el.querySelector('.hd-toggle').dispatchEvent({ type: 'click', preventDefault() {} }); eq(calls.pop(), ['primary', 'turn_on', {}]); mi.destroy(); }],
  ['overlay: a read-only light marks the controls aria-readonly, shows the long reason in the foot and announces it instead of sending', () => {
    const { E, document } = boot(), st = { view: flat(KITCHEN, { readonly: true, readonlyReason: 'homedash.readonly' }) }, { api, calls, said } = fakeApi(E, st), { root } = stage(document), mi = E.moreinfo.build({ family: 'glass', api }); root.appendChild(mi.el); mi.update();
    for (const sel of ['.hd-toggle', '.hd-slider', '.hd-ct', '.hd-wheel']) assert.equal(mi.el.querySelector(sel).getAttribute('aria-readonly'), 'true', sel);
    assert.ok(mi.el.querySelector('.hd-foot').textContent.includes(EN['homedash.reason.homedash_readonly_long'])); assert.equal(mi.el.querySelector('.hd-state-text').textContent, 'On' + MID + EN['homedash.reason.homedash_readonly']);
    assert.equal(mi.el.getAttribute('data-hd-readonly'), 'true'); mi.el.querySelector('.hd-toggle').dispatchEvent({ type: 'click', preventDefault() {} }); assert.equal(calls.length, 0); assert.deepEqual(said, [EN['homedash.reason.homedash_readonly_long']]);
    mi.el.querySelector('.hd-slider').dispatchEvent(key('ArrowRight')); assert.equal(calls.length, 0); mi.destroy(); }],
  ['overlay: an unavailable light disables the controls and says Unavailable', () => {
    const { E, document } = boot(), st = { view: flat(KITCHEN, { available: false, status: 'unavailable', value: null }) }, { api, said } = fakeApi(E, st), { root } = stage(document), mi = E.moreinfo.build({ family: 'glass', api }); root.appendChild(mi.el); mi.update();
    for (const sel of ['.hd-toggle', '.hd-slider', '.hd-ct', '.hd-wheel', '.hd-pick']) assert.equal(mi.el.querySelector(sel).getAttribute('aria-disabled'), 'true', sel);
    assert.equal(mi.el.querySelector('.hd-state-text').textContent, 'Unavailable'); assert.equal(mi.el.getAttribute('data-hd-status'), 'unavailable'); assert.equal(mi.el.querySelector('.hd-slider').hasAttribute('aria-valuenow'), false);
    mi.el.querySelector('.hd-toggle').dispatchEvent({ type: 'click', preventDefault() {} }); assert.deepEqual(said, ['Unavailable']);
    st.view = null; mi.update(); assert.deepEqual(rowsOf(mi.el), ['power']); mi.destroy(); }],
  ['overlay: an error cue and a pending command show in the state line', () => {
    const { E, document } = boot(), st = { view: flat(KITCHEN, { status: 'error', error: { text: 'Command not accepted' } }) }, { api } = fakeApi(E, st), { root } = stage(document), mi = E.moreinfo.build({ family: 'glass', api }); root.appendChild(mi.el); mi.update();
    assert.equal(mi.el.querySelector('.hd-state-text').textContent, 'Command not accepted'); st.view = flat(KITCHEN, { pending: true }); mi.update(); assert.equal(mi.el.querySelector('.hd-state-text').textContent, EN['homedash.state.pending']); assert.equal(mi.el.getAttribute('data-hd-pending'), 'true'); mi.destroy(); }],

  ['overlay: moreinfo.create opens a modal layer, focuses the power toggle, re-renders on subscribe and closes with Escape back to the opener', () => {
    const { E, document } = boot(), st = { view: flat(KITCHEN) }, { api } = fakeApi(E, st), { root, main, live, opener } = stage(document), box = E.moreinfo.create({ root }); let fire = null, unsubbed = 0;
    assert.equal(box.isOpen(), false); box.open({ api, family: 'instrument', opener, subscribe: fn => { fire = fn; return () => { unsubbed++; }; } });
    const layer = root.querySelector('.hd-sheet-layer[data-kind="moreinfo"]'); assert.ok(layer); assert.equal(layer.getAttribute('data-hd-pop'), 'center'); assert.equal(box.isOpen(), true);
    const dlg = layer.querySelector('.hd-sheet'); assert.equal(dlg.getAttribute('data-hd-family'), 'instrument'); assert.equal(document.activeElement, layer.querySelector('.hd-toggle'));
    assert.equal(main.getAttribute('inert'), ''); assert.equal(live.hasAttribute('inert'), false); assert.equal(layer.hasAttribute('inert'), false);
    st.view = flat(KITCHEN, { value: false }); fire(); assert.equal(layer.querySelector('.hd-toggle').getAttribute('aria-checked'), 'false');
    layer.dispatchEvent(key('Escape')); assert.equal(root.querySelector('.hd-sheet-layer'), null); assert.equal(document.activeElement, opener); assert.equal(main.hasAttribute('inert'), false); assert.equal(unsubbed, 1); assert.equal(box.isOpen(), false);
    box.open({ api, family: 'glass', opener }); const again = root.querySelector('.hd-sheet-layer'); again.dispatchEvent({ type: 'click', target: again }); assert.equal(root.querySelector('.hd-sheet-layer'), null); assert.equal(document.activeElement, opener);
    box.open({ api, family: 'glass', opener }); root.querySelector('.hd-sheet-close').dispatchEvent({ type: 'click', preventDefault() {} }); assert.equal(root.querySelector('.hd-sheet-layer'), null); assert.equal(document.activeElement, opener); box.destroy(); }],
  ['overlay: moreinfo.create makes a bottom sheet on a narrow root, keeps one layer, and Escape in the effect listbox closes only the listbox', () => {
    const { E, document } = boot(), st = { view: flat(KITCHEN) }, { api } = fakeApi(E, st), { root, opener } = stage(document, 480), box = E.moreinfo.create({ root });
    box.open({ api, family: 'glass', opener }); box.open({ api, family: 'glass', opener }); assert.equal(root.querySelectorAll('.hd-sheet-layer').length, 1); const layer = root.querySelector('.hd-sheet-layer'); assert.equal(layer.getAttribute('data-hd-pop'), 'sheet');
    layer.querySelector('.hd-pick').dispatchEvent({ type: 'click', preventDefault() {} }); const list = layer.querySelector('[role="listbox"]'); assert.ok(list); list.dispatchEvent(key('Escape'));
    assert.equal(layer.querySelector('[role="listbox"]'), null); assert.ok(root.querySelector('.hd-sheet-layer'), 'the sheet stays open'); assert.equal(document.activeElement, layer.querySelector('.hd-pick'));
    box.close(); assert.equal(root.querySelector('.hd-sheet-layer'), null); assert.equal(document.activeElement, opener); }]
];
