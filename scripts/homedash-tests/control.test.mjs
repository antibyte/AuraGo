import assert from 'node:assert/strict'; import { loadEngine, ENGINE_ORDER, read } from './harness.mjs';
const boot = () => loadEngine(ENGINE_ORDER, { dom: true });
const EN = JSON.parse(read('ui/lang/homedash/en.json'));
const tOf = E => E.core.makeT(EN);
const view = p => Object.assign({ id: 'ha:switch.x', kind: 'switch', name: 'Desk lamp', value: true, attributes: {}, available: true, status: 'ok', readonly: false, capabilities: [{ name: 'toggle', allowed: true }], pending: false, stale: false, error: null, tone: null, icon_hint: '' }, p);
function factory(state, calls) { return () => ({ entity: () => state.view, can: () => true, command: () => Promise.resolve({}), moreInfo(...a) { calls.push(a); }, history: () => Promise.resolve({ points: [], source: 'ring' }), announce() {} }); }
function mount(E, document, state, calls, extra) {
  E.widgets.register('zz_ctl' + (mount.n = (mount.n || 0) + 1), { bindings: ['primary'], variants: ['a'], create() { return { update() {} }; } });
  const type = 'zz_ctl' + mount.n, h = E.widgets.host(Object.assign({ widget: { id: 'w', type, variant: 'a', title: '', bindings: { primary: 'ha:switch.x' } }, size: { w: 2, h: 1 }, t: tOf(E), api: factory(state, calls), moreLabel: 'More' }, extra));
  document.body.appendChild(h.el); return h;
}
export const tests = [
  ['control: reasonText gives the short and long text for every blocked_by key', () => { const { E } = boot(), t = tOf(E), C = E.control;
    const keys = { 'homedash.readonly': ['homedash_readonly', 'homedash_readonly_long'], 'virtual_desktop.readonly': ['desktop_readonly', 'desktop_readonly_long'], 'homedash.sources.home_assistant.control': ['source_control', 'source_control_long'],
      'home_assistant.allowed_services': ['allowed_services', 'allowed_services_long'], 'home_assistant.blocked_services': ['blocked_services', 'blocked_services_long'], 'home_assistant.readonly': ['ha_readonly', 'ha_readonly_long'] };
    for (const [k, [s, l]] of Object.entries(keys)) { assert.equal(C.reasonText(t, k, false), EN['homedash.reason.' + s], k); assert.equal(C.reasonText(t, k, true), EN['homedash.reason.' + l], k); assert.notEqual(C.reasonText(t, k, false), 'homedash.reason.' + s); }
    assert.equal(C.reasonText(t, 'weird', false), EN['homedash.state.readonly']); assert.equal(C.reasonText(t, '', true), EN['homedash.reason.unknown_long']); }],
  ['control: cueText per error code, with the source name', () => { const { E } = boot(), t = tOf(E), C = E.control;
    assert.equal(C.cueText(t, { code: 'timeout' }), 'Not confirmed by the device'); assert.equal(C.cueText(t, { code: 'source_error' }), 'Command not accepted'); assert.equal(C.cueText(t, { code: 'rate_limited' }), 'Too many commands, try again');
    assert.equal(C.cueText(t, { code: 'source_unavailable', source: 'ha' }), 'Home Assistant is not reachable'); assert.equal(C.cueText(t, { code: 'hub_closed' }), 'Home Assistant is not reachable'); assert.equal(C.cueText(t, { code: 'source_unavailable', source: 'system' }), 'System is not reachable');
    assert.equal(C.cueText(t, { code: 'network' }), 'No connection to the server'); assert.equal(C.cueText(t, { code: 'bad_request' }), 'Value not accepted'); assert.equal(C.cueText(t, { code: 'entity_not_found' }), 'Device not found');
    assert.equal(C.cueText(t, { code: 'zzz' }), 'Command failed (zzz)'); assert.equal(C.cueText(t, null), 'Command failed (?)'); }],
  ['control: iconFor prefers widget.icon, then icon_hint, then the kind icon; unknown names fall through', () => { const { E } = boot(), I = E.widgets.iconFor;
    assert.equal(I({ icon: 'timer' }, view({ icon_hint: 'outlet' })), 'timer'); assert.equal(I({ icon: 'nope' }, view({ icon_hint: 'outlet' })), 'outlet'); assert.equal(I({}, view({ icon_hint: 'nope', kind: 'light' })), 'bulb');
    assert.equal(I({}, view({ kind: 'select' })), 'list'); assert.equal(I({}, view({ kind: 'weird' })), 'entity'); assert.equal(I({ icon: 'timer' }, null), 'timer'); assert.equal(I({ icon: 'nope' }, null), null); assert.equal(I({}, null), null); }],
  ['control: statusText precedence is host words, armed, read-only, blocked, stale, word', () => { const { E } = boot(), C = E.control, t = tOf(E), api = { t, lang: 'en', widget: {} }, MID = ' ' + String.fromCodePoint(0xb7) + ' ';
    assert.equal(C.statusText(api, null, 'On'), null); assert.equal(C.statusText(api, view({ pending: true }), 'On'), null); assert.equal(C.statusText(api, view({ status: 'error' }), 'On'), null); assert.equal(C.statusText(api, view({ available: false }), 'On'), null);
    assert.equal(C.statusText(api, view({ armed: { seconds: 3 }, readonly: true }), 'On'), 'Tap again to confirm' + MID + '3 s');
    assert.equal(C.statusText(api, view({ readonly: true, readonlyReason: 'homedash.readonly' }), 'On', { cap: 'toggle' }), 'On' + MID + 'HomeDash is read-only');
    const blocked = view({ capabilities: [{ name: 'toggle', allowed: false, blocked_by: 'home_assistant.allowed_services' }] }); assert.equal(C.statusText(api, blocked, 'On', { cap: 'toggle' }), 'On' + MID + 'Service not allowed');
    const old = view({ stale: true, attributes: { updated_at: new Date(Date.now() - 5 * 60000).toISOString() } }); assert.ok(C.statusText(api, old, 'On').startsWith('Updated'));
    assert.equal(C.statusText(api, view({ stale: true }), 'On'), 'On'); assert.equal(C.statusText(api, view({ confirmed: true }), 'On', { done: true }), 'Done'); assert.equal(C.statusText(api, view({ confirmed: true }), 'On'), 'On'); }],
  ['control: the host shows cue text, follows armed and confirmed, swaps the head icon, falls back for the title and wires the more button', () => { const { E, document } = boot(), calls = [], state = { view: view({ status: 'error', error: { text: 'Command not accepted' } }) };
    const h = mount(E, document, state, calls, { titleFallback: () => 'Fallback title' }), q = s => h.el.querySelector(s), a = n => h.el.getAttribute(n);
    assert.equal(q('.hd-state-text').textContent, 'Command not accepted'); state.view = view({ status: 'error' }); h.update(); assert.equal(q('.hd-state-text').textContent, EN['homedash.state.error']);
    state.view = view({ armed: { seconds: 4 }, confirmed: false }); h.update(); assert.equal(a('data-hd-armed'), 'true'); assert.notEqual(a('data-hd-confirmed'), 'true');
    state.view = view({ armed: null, confirmed: true }); h.update(); assert.notEqual(a('data-hd-armed'), 'true'); assert.equal(a('data-hd-confirmed'), 'true');
    assert.equal(q('.hd-head .hd-icon svg').getAttribute('data-hd-icon'), 'power'); state.view = view({ icon_hint: 'outlet' }); h.update(); assert.equal(q('.hd-head .hd-icon svg').getAttribute('data-hd-icon'), 'outlet');
    state.view = null; h.update(); assert.equal(q('.hd-title').textContent, 'Fallback title');
    const more = q('.hd-more'); assert.ok(more.querySelector('svg[data-hd-icon="more"]')); more.dispatchEvent({ type: 'click', preventDefault() {} }); assert.deepEqual(calls, [['primary']]);
    h.setMode('design'); more.dispatchEvent({ type: 'click', preventDefault() {} }); assert.equal(calls.length, 1); assert.equal(more.hasAttribute('hidden'), true);
    h.setMode('thumb'); assert.equal(more.hasAttribute('hidden'), true); h.setMode('view'); assert.equal(more.hasAttribute('hidden'), false); h.destroy(); }]
];
