import assert from 'node:assert/strict'; import { loadEngine, ENGINE_ORDER, fc } from './harness.mjs';
const boot = () => loadEngine(ENGINE_ORDER, { dom: true });
const WORDS = { 'homedash.state.on': 'On', 'homedash.state.pending': 'Sending', 'homedash.state.unavailable': 'Unavailable' };
const view = p => Object.assign({ id: 'x:1', kind: 'switch', name: 'Desk lamp', value: 'off', attributes: {}, available: true, status: 'ok', readonly: false, capabilities: ['toggle'], pending: false, stale: false, error: null, tone: null }, p);
function factory(state) { return ctx => ({ entity: () => state.view, can: () => true, command: () => Promise.resolve({}), moreInfo() {}, history: () => Promise.resolve({ points: [[1, 2], [2, null]], source: 'ring' }), announce() {} }); }
export const tests = [
  ['register validates its input', () => { const { E } = boot();
    assert.throws(() => E.widgets.register('Bad-Type', { bindings: [], variants: ['a'], create() {} })); assert.throws(() => E.widgets.register('zz', { bindings: [], variants: ['a'] }));
    assert.throws(() => E.widgets.register('entity', { bindings: [], variants: ['tile'], create() {} }), /registered/); assert.ok(E.widgets.types().includes('entity')); }],
  ['host lifecycle: create, synchronous update, attributes, resize, mode, destroy', () => { const { E, document } = boot(); const calls = []; let api;
    E.widgets.register('zz_probe', { bindings: ['primary'], variants: ['a', 'b'], create(a) { api = a; calls.push('create'); return { update(v) { calls.push(['update', v.mode, v.entities.primary && v.entities.primary.name]); }, resize(s) { calls.push(['resize', s.w, s.h]); }, destroy() { calls.push('destroy'); } }; } });
    const state = { view: view({ value: 'on', pending: true, stale: true, readonly: true, tone: 'crit' }) };
    const h = E.widgets.host({ widget: { id: 'w1', type: 'zz_probe', variant: 'b', title: '', bindings: { primary: 'x:1' } }, size: { w: 2, h: 1 }, t: E.core.makeT(WORDS), api: factory(state), moreLabel: 'More' });
    document.body.appendChild(h.el); assert.deepEqual(calls.slice(0, 2), ['create', ['update', 'view', 'Desk lamp']]);
    for (const k of ['body', 'widget', 'variant', 'family', 'mode', 't', 'fmt', 'prim', 'icons', 'signal', 'lang', 'entity', 'can', 'command', 'moreInfo', 'history', 'announce']) assert.ok(k in api, k);
    assert.equal(api.variant, 'b'); assert.equal(api.body, h.body); assert.equal(api.family, 'glass'); assert.throws(() => { api.mode = 'x'; });
    const a = n => h.el.getAttribute(n); assert.equal(a('data-hd-family'), 'glass');
    assert.deepEqual([a('data-hd-type'), a('data-hd-variant'), a('data-hd-size'), a('data-hd-on'), a('data-hd-pending'), a('data-hd-stale'), a('data-hd-readonly'), a('data-hd-tone'), a('data-hd-status')], ['zz_probe', 'b', '2x1', 'true', 'true', 'true', 'true', 'crit', 'ok']);
    assert.equal(h.el.querySelector('.hd-title').textContent, 'Desk lamp'); assert.equal(h.el.querySelector('.hd-state-text').textContent, 'Sending'); assert.ok(h.el.querySelector('.hd-more'));
    h.resize({ w: 3, h: 2, px: { width: 300, height: 200 } }); assert.equal(a('data-hd-size'), '3x2'); assert.deepEqual(calls[2], ['resize', 3, 2]);
    h.setMode('design'); assert.equal(h.body.hasAttribute('inert'), true); assert.equal(api.mode, 'design'); state.view = view({ available: false, status: 'unavailable' }); h.update(); assert.equal(a('data-hd-status'), 'unavailable');
    const signal = api.signal; h.destroy(); h.destroy(); assert.equal(signal.aborted, true); assert.equal(calls.filter(c => c === 'destroy').length, 1); assert.equal(document.body.children.length, 0); }],
  ['the api factory must supply every function', () => { const { E } = boot();
    assert.throws(() => E.widgets.host({ widget: { id: 'w', type: 'entity', bindings: {} }, size: { w: 1, h: 1 }, t: k => k, api: () => ({ entity() {} }) }), /api\.can/); }],
  ['entity tile shows numbers, binary state and text', () => { const { E } = boot(); const state = { view: view({ kind: 'sensor', value: 21.5, unit: 'C' }) };
    const h = E.widgets.host({ widget: { id: 'w', type: 'entity', variant: 'tile', bindings: { primary: 'x' } }, size: { w: 2, h: 1 }, t: E.core.makeT(WORDS), api: factory(state) });
    const num = h.el.querySelector('.hd-numeral'), lamp = h.el.querySelector('.hd-lamp');
    assert.equal(num.querySelector('.hd-num-int').textContent, '21'); assert.equal(lamp.hasAttribute('hidden'), true);
    state.view = view({ value: 'on' }); h.update(); assert.equal(lamp.hasAttribute('hidden'), false); assert.equal(lamp.getAttribute('data-hd-on'), 'true'); assert.equal(num.hasAttribute('hidden'), true);
    state.view = view({ value: 'heat' }); h.update(); assert.equal(num.querySelector('.hd-num-int').textContent, 'heat'); state.view = null; h.update(); assert.equal(num.querySelector('.hd-num-int').textContent, fc(0x2013)); }],
  ['family from the parent, setTone, setStatus and the history shape', async () => { const { E, document } = boot(); let api;
    E.widgets.register('zz_two', { bindings: ['primary'], variants: ['a'], create(x) { api = x; return { update() {} }; } });
    const root = document.createElement('section'); root.setAttribute('data-hd-family', 'tactile'); const stage = document.createElement('div'); root.appendChild(stage);
    const state = { view: view({ value: 'on', tone: 'warn' }) };
    const h = E.widgets.host({ widget: { id: 'w', type: 'zz_two', bindings: { primary: 'x' } }, size: { w: 1, h: 1 }, t: E.core.makeT(WORDS), api: factory(state), parent: stage });
    assert.equal(h.el.parentNode, stage); assert.equal(api.family, 'tactile'); assert.equal(h.el.getAttribute('data-hd-family'), 'tactile'); assert.equal(h.family, 'tactile');
    assert.equal(h.el.getAttribute('data-hd-tone'), 'warn'); api.setTone('crit'); assert.equal(h.el.getAttribute('data-hd-tone'), 'crit'); h.update(); assert.equal(h.el.getAttribute('data-hd-tone'), 'crit');
    api.setTone(null); assert.equal(h.el.getAttribute('data-hd-tone'), 'warn');
    api.setStatus('Heating to 21.5'); assert.equal(h.el.querySelector('.hd-state-text').textContent, 'Heating to 21.5');
    state.view = view({ value: 'on', pending: true }); h.update(); assert.equal(h.el.querySelector('.hd-state-text').textContent, 'Sending');
    state.view = view({ value: 'on' }); api.setStatus(null); assert.equal(h.el.querySelector('.hd-state-text').textContent, 'On');
    assert.deepEqual(JSON.parse(JSON.stringify(await api.history('primary', '24h'))), { points: [[1, 2], [2, null]], source: 'ring' });
    const g = E.widgets.host({ widget: { id: 'w2', type: 'zz_two', family: 'minimal', bindings: {} }, size: { w: 1, h: 1 }, t: k => k, api: factory(state), parent: stage });
    assert.equal(g.el.getAttribute('data-hd-family'), 'minimal'); }],
  ['no primary entity leaves the state line empty', () => { const { E } = boot(); let api;
    E.widgets.register('zz_none', { bindings: [], variants: ['a'], create(x) { api = x; return { update() {} }; } });
    E.widgets.register('zz_null', { bindings: ['primary'], variants: ['a'], create() { return { update() {} }; } });
    const h = E.widgets.host({ widget: { id: 'n', type: 'zz_none', bindings: {} }, size: { w: 1, h: 1 }, t: E.core.makeT(WORDS), api: factory({ view: null }) });
    const text = () => h.el.querySelector('.hd-state-text').textContent;
    assert.equal(text(), ''); assert.equal(h.el.getAttribute('data-hd-status'), 'ok');
    api.setStatus('Ready'); assert.equal(text(), 'Ready'); api.setStatus(null); assert.equal(text(), '');
    const g = E.widgets.host({ widget: { id: 'm', type: 'zz_null', bindings: { primary: 'x' } }, size: { w: 1, h: 1 }, t: E.core.makeT(WORDS), api: factory({ view: null }) });
    assert.equal(g.el.querySelector('.hd-state-text').textContent, ''); }],
  ['entity tile with no value shows the empty numeral and no lamp', () => { const { E } = boot();
    for (const v of [{ value: null, status: 'unavailable', available: false }, { status: 'ok' }]) {
      const h = E.widgets.host({ widget: { id: 'w', type: 'entity', variant: 'tile', bindings: { primary: 'x' } }, size: { w: 2, h: 1 }, t: E.core.makeT(WORDS), api: () => ({ entity: () => v, can: () => true, command() {}, moreInfo() {}, history() {}, announce() {} }) });
      const num = h.el.querySelector('.hd-numeral'), lamp = h.el.querySelector('.hd-lamp'), text = num.querySelector('.hd-num-int').textContent;
      assert.notEqual(text, 'null'); assert.notEqual(text, 'undefined'); assert.equal(num.getAttribute('data-hd-empty'), 'true'); assert.equal(lamp.hasAttribute('hidden'), true);
    } }],
  ['host cleans up when create or the api factory throws', () => { const { E, document } = boot(); let signal;
    E.widgets.register('zz_throw', { bindings: [], variants: ['a'], create() { throw new Error('boom'); } });
    E.widgets.register('zz_okay', { bindings: [], variants: ['a'], create() { return { update() {} }; } });
    const parent = document.createElement('div');
    assert.throws(() => E.widgets.host({ widget: { id: 'a', type: 'zz_throw', bindings: {} }, size: { w: 1, h: 1 }, t: k => k, parent, api: ctx => { signal = ctx.signal; return factory({ view: null })(ctx); } }), /boom/);
    assert.equal(parent.children.length, 0); assert.equal(signal.aborted, true);
    assert.throws(() => E.widgets.host({ widget: { id: 'b', type: 'zz_okay', bindings: {} }, size: { w: 1, h: 1 }, t: k => k, parent, api: () => { throw new Error('factory'); } }), /factory/);
    assert.equal(parent.children.length, 0); }],
  ['a destroyed host ignores resize, setMode and update', () => { const { E } = boot(); const n = { resize: 0, update: 0 };
    E.widgets.register('zz_count', { bindings: [], variants: ['a'], create() { return { update() { n.update++; }, resize() { n.resize++; } }; } });
    const h = E.widgets.host({ widget: { id: 'c', type: 'zz_count', bindings: {} }, size: { w: 1, h: 1 }, t: k => k, api: factory({ view: null }) });
    h.destroy(); const before = JSON.stringify(n);
    h.resize({ w: 2, h: 2 }); h.setMode('design'); h.update(); assert.equal(JSON.stringify(n), before); assert.equal(h.el.getAttribute('data-hd-size'), '1x1'); }],
  ['thumb mode makes the whole frame inert', () => { const { E } = boot();
    const h = E.widgets.host({ widget: { id: 'm', type: 'entity', variant: 'tile', bindings: { primary: 'x' } }, size: { w: 1, h: 1 }, t: k => k, moreLabel: 'More', api: factory({ view: view({ value: 'on' }) }) });
    const frame = h.el.querySelector('.hd-frame');
    h.setMode('thumb'); assert.equal(frame.hasAttribute('inert'), true); assert.equal(h.el.getAttribute('aria-hidden'), 'true'); assert.equal(h.body.hasAttribute('inert'), true);
    h.setMode('view'); assert.equal(frame.hasAttribute('inert'), false); assert.equal(h.el.hasAttribute('aria-hidden'), false); assert.equal(h.body.hasAttribute('inert'), false);
    h.setMode('design'); assert.equal(frame.hasAttribute('inert'), false); assert.equal(h.body.hasAttribute('inert'), true); }],
  ['widgets (M4): paletteSnapshot resolves the defined tokens to lower-case hex, skips undefined and malformed names, caps at 40 and leaves no probe behind', () => {
    const { E, document } = loadEngine(ENGINE_ORDER, { dom: true }), root = document.createElement('div'); document.body.appendChild(root);
    const theme = { '--hd-ink': 'rgb(240, 240, 240)', '--hd-canvas': 'rgb(16, 20, 30)', '--hd-accent': 'rgb(122, 167, 255)' };
    document.defaultView = { getComputedStyle: node => { if (node === root) return { getPropertyValue: n => (theme[n] ? 'x' : '') }; const m = /var\((.*)\)/.exec(node.style.getPropertyValue('color')); return { color: m && theme[m[1]] ? theme[m[1]] : 'rgba(0, 0, 0, 0)' }; } };
    const snap = E.widgets.paletteSnapshot(root, ['ink', 'canvas', 'accent', 'missing', 'Bad Name', '', 'x'.repeat(40)]); assert.deepEqual({ ...snap }, { ink: '#f0f0f0', canvas: '#10141e', accent: '#7aa7ff' }); assert.equal(root.children.length, 0, 'the probe is removed');
    assert.deepEqual({ ...E.widgets.paletteSnapshot(root, []) }, {}); assert.deepEqual({ ...E.widgets.paletteSnapshot(null, ['ink']) }, {}); assert.deepEqual({ ...E.widgets.paletteSnapshot({ nodeType: 3 }, ['ink']) }, {});
    const many = Array.from({ length: 60 }, (_, i) => 't' + i); for (const n of many) theme['--hd-' + n] = 'rgb(1, 2, 3)'; assert.equal(Object.keys(E.widgets.paletteSnapshot(root, many)).length, 40); assert.equal(root.children.length, 0);
    document.defaultView = undefined; assert.deepEqual({ ...E.widgets.paletteSnapshot(root, ['ink']) }, {}); }]
];
