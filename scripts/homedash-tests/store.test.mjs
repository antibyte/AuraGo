import assert from 'node:assert/strict'; import { loadEngine, eq } from './harness.mjs'; import { fakeClock, fakeSources, fakeStreamApi, HELLO } from './live-fakes.mjs';
const CPU = { id: 'system:cpu', source: 'system', kind: 'sensor', name: 'CPU', unit: '%', icon_hint: 'cpu', capabilities: [{ name: 'numeric', allowed: true }, { name: 'history', allowed: true }],
  readonly: true, state: { value: 12.3, available: true, updated_at: '2026-11-25T10:00:00Z' } };
const UPTIME = Object.assign({}, CPU, { id: 'system:uptime', name: 'Uptime', unit: 's', device_class: 'duration', state: { value: 7215, available: true, updated_at: '2026-11-25T10:00:00Z' } });
function setup(apiExtra) {
  const { E } = loadEngine(['hd-core.js', 'hd-api.js', 'hd-store.js']), clock = fakeClock(), frames = [], bus = E.core.emitter(), handles = [];
  const stream = { on: bus.on, off: bus.off, status: () => ({ phase: 'idle', reason: '' }),
    attach(owner, init) { const h = { id: owner, sets: [], active: init.active, detached: false, set(ids) { h.sets.push(Array.from(ids)); }, setActive(a) { h.active = a; }, detach() { h.detached = true; } }; handles.push(h); return h; } };
  const api = Object.assign({ base: '/api/desktop/homedash', history: () => Promise.resolve({ origin: 'source', points: [] }), widgetCatalog: () => Promise.resolve({ status: 'ok', catalog: { hash: 'h1' } }) }, apiExtra);
  const store = E.store.create({ api, stream, raf: fn => { frames.push(fn); return frames.length; }, caf: () => {}, setTimeout: clock.setTimeout, clearTimeout: clock.clearTimeout, now: clock.now });
  const changes = []; store.on('change', ids => changes.push(ids === null ? null : [...ids].sort()));
  return { E, store, bus, handles, clock, api, changes, run: () => { while (frames.length) frames.shift()(); } };
}
export const tests = [
  ['store: a snapshot of watched ids gives widget views; unwatched ids stay out', () => {
    const { store, bus, handles, changes, run } = setup();
    store.watch('win', { ids: ['system:cpu', 'system:uptime'] }); eq(handles[0].sets[0], ['system:cpu', 'system:uptime']);
    bus.emit('snapshot', { entities: [CPU, UPTIME, Object.assign({}, CPU, { id: 'system:memory' })], sources: [{ source: 'system', state: 'polling', at: 'x' }] }); run();
    eq(changes, [['system:cpu', 'system:uptime']]);
    const v = store.view('system:cpu');
    assert.equal(v.status, 'ok'); assert.equal(v.value, 12.3); assert.equal(v.unit, '%'); assert.equal(v.icon_hint, 'cpu'); assert.equal(v.readonly, false); assert.equal(v.serverReadonly, true); assert.equal(v.stale, false);
    assert.equal(v.attributes.updated_at, '2026-11-25T10:00:00Z'); assert.equal(v.tone, null); assert.equal(store.view('system:memory'), null); assert.equal(store.view('system:cpu'), v); }],
  ['store: state frames coalesce into one notice; readonly needs a command capability', () => {
    const { store, bus, changes, run } = setup(), w = store.watch('win', { ids: ['system:cpu', 'system:uptime'] });
    bus.emit('snapshot', { entities: [CPU, UPTIME], sources: [] }); run();
    bus.emit('state', { changes: { 'system:cpu': { value: 50, available: true, updated_at: '2026-11-25T10:00:10Z' } } }); bus.emit('state', { changes: { 'system:uptime': { value: 7225, available: true, updated_at: '2026-11-25T10:00:10Z' } } });
    assert.equal(changes.length, 1); run(); eq(changes[1], ['system:cpu', 'system:uptime']); assert.equal(store.view('system:cpu').value, 50);
    w.set(['system:cpu', 'ha:switch.lamp']);
    bus.emit('entity', { entities: [{ id: 'ha:switch.lamp', source: 'ha', kind: 'switch', name: 'Lamp', capabilities: [{ name: 'toggle', allowed: false, blocked_by: 'homedash.readonly' }], readonly: true, state: { value: 'on', available: true, updated_at: 'x' } }] });
    assert.equal(store.view('ha:switch.lamp').readonly, true); assert.equal(store.view('ha:switch.lamp').serverReadonly, true);
    store.setDesktopReadonly(true); assert.equal(store.view('system:cpu').readonly, false); }],
  ['store: readonlyReason names why a control is locked (D8)', () => {
    const { store, bus } = setup(), w = store.watch('win', { ids: ['system:cpu'] });
    w.set(['system:cpu', 'ha:script.night', 'ha:switch.lamp', 'ha:switch.fan', 'ha:switch.open']);
    const ent = (id, extra) => Object.assign({ id, source: 'ha', kind: 'switch', name: id, capabilities: [{ name: 'toggle', allowed: true }], state: { value: 'on', available: true, updated_at: 'x' } }, extra);
    bus.emit('snapshot', { entities: [CPU, ent('ha:script.night', { kind: 'script', readonly: true, readonly_reason: 'home_assistant.allowed_services' }),
      ent('ha:switch.lamp', { readonly: true, capabilities: [{ name: 'numeric', allowed: true }, { name: 'toggle', allowed: false, blocked_by: 'home_assistant.blocked_services' }] }),
      ent('ha:switch.fan', { readonly: true }), ent('ha:switch.open', {})], sources: [] });
    assert.equal(store.view('ha:script.night').readonly, true); assert.equal(store.view('ha:script.night').readonlyReason, 'home_assistant.allowed_services');
    assert.equal(store.view('ha:switch.lamp').readonlyReason, 'home_assistant.blocked_services');
    assert.equal(store.view('ha:switch.fan').readonlyReason, 'homedash.readonly');
    assert.equal(store.view('system:cpu').readonly, false); assert.equal(store.view('system:cpu').readonlyReason, '');
    assert.equal(store.view('ha:switch.open').readonly, false); assert.equal(store.view('ha:switch.open').readonlyReason, '');
    store.setDesktopReadonly(true);
    assert.equal(store.view('ha:script.night').readonlyReason, 'virtual_desktop.readonly'); assert.equal(store.view('ha:switch.open').readonly, true); assert.equal(store.view('ha:switch.open').readonlyReason, 'virtual_desktop.readonly');
    assert.equal(store.view('system:cpu').readonly, false); assert.equal(store.view('system:cpu').readonlyReason, '');
    store.setDesktopReadonly(false); assert.equal(store.view('ha:script.night').readonlyReason, 'home_assistant.allowed_services'); assert.equal(store.view('ha:switch.open').readonlyReason, ''); }],
  ['store: removed and rejected ids are unknown entities', () => {
    const { store, bus } = setup(); store.watch('win', { ids: ['system:cpu', 'nope:x'] });
    bus.emit('snapshot', { entities: [CPU], sources: [] }); bus.emit('removed', { ids: ['system:cpu'] }); bus.emit('subscribed', { accepted: [], rejected: [{ id: 'nope:x', reason: 'unknown_source' }] });
    assert.equal(store.view('system:cpu').status, 'unknown'); assert.equal(store.view('system:cpu').value, null); assert.equal(store.view('nope:x').status, 'unknown'); assert.equal(store.view('nope:x').name, 'x'); }],
  ['store: 5 s down marks every value stale; a snapshot clears it per id; degraded stays fresh, unavailable is offline', async () => {
    const { store, bus, clock, run } = setup(); store.watch('win', { ids: ['system:cpu', 'system:uptime'] });
    bus.emit('snapshot', { entities: [CPU, UPTIME], sources: [] }); run();
    bus.emit('connection', { phase: 'down', reason: 'error' }); await clock.advance(4999);
    assert.equal(store.connection().offline, false); assert.equal(store.view('system:cpu').stale, false);
    await clock.advance(1); assert.equal(store.connection().offline, true); assert.equal(store.view('system:cpu').stale, true);
    bus.emit('connection', { phase: 'live', reason: '' }); assert.equal(store.connection().offline, false); assert.equal(store.view('system:cpu').stale, true);
    bus.emit('snapshot', { entities: [CPU], sources: [] }); assert.equal(store.view('system:cpu').stale, false); assert.equal(store.view('system:uptime').stale, true);
    bus.emit('source', { source: 'system', state: 'degraded', reason: 'stale_metrics', at: 'x' }); assert.equal(store.view('system:cpu').stale, false); assert.equal(store.view('system:cpu').available, true); eq(store.sources().map(s => s.state), ['degraded']);
    bus.emit('source', { source: 'system', state: 'unavailable', reason: 'no_metrics', at: 'x' });
    const off = store.view('system:cpu'); assert.equal(off.available, false); assert.equal(off.status, 'unavailable'); assert.equal(off.stale, false); assert.equal(store.view('system:uptime').available, false);
    bus.emit('source', { source: 'system', state: 'polling', at: 'x' });
    const back = store.view('system:cpu'); assert.equal(back.available, true); assert.equal(back.status, 'ok'); assert.equal(back.value, 12.3);
    bus.emit('source', { source: 'system', state: 'unavailable', reason: 'no_metrics', at: 'x' }); bus.emit('source', { source: 'system', state: 'live', at: 'x' });
    assert.equal(store.view('system:cpu').available, true); assert.equal(store.view('system:cpu').value, 12.3); }],
  ['store: history becomes host series, is cached, grows with live states and expires', async () => {
    const calls = [];
    const { store, bus, clock } = setup({ history(id, r) { calls.push({ id, span: r.to - r.from, points: r.points });
      return Promise.resolve({ status: 'ok', id, origin: 'source', points: [{ t: '2026-11-25T09:58:40Z', v: 11.8 }, { t: '2026-11-25T09:59:50Z', v: 12.3 }, { t: 'bad', v: 1 }, { t: '2026-11-25T09:59:00Z', v: null }] }); } });
    store.watch('win', { ids: ['system:cpu'] }); bus.emit('snapshot', { entities: [CPU], sources: [] });
    const s1 = await store.history('system:cpu', '1h', 60);
    eq(s1, { points: [[Date.parse('2026-11-25T09:58:40Z'), 11.8], [Date.parse('2026-11-25T09:59:00Z'), null], [Date.parse('2026-11-25T09:59:50Z'), 12.3]], source: 'source' });
    eq(calls, [{ id: 'system:cpu', span: 3600000, points: 60 }]);
    await store.history('system:cpu', 3600000, 60); assert.equal(calls.length, 1);
    bus.emit('state', { changes: { 'system:cpu': { value: 14, available: true, updated_at: '2026-11-25T10:00:10Z' } } });
    bus.emit('state', { changes: { 'system:cpu': { value: 99, available: true, updated_at: '2026-11-25T09:00:00Z' } } });
    const s2 = await store.history('system:cpu', '1h', 60); assert.equal(s2.points.length, 4); eq(s2.points[3], [Date.parse('2026-11-25T10:00:10Z'), 14]);
    await clock.advance(60000); await store.history('system:cpu', '1h', 60); assert.equal(calls.length, 2); }],
  ['store: at most four history requests run at once', async () => {
    const pending = []; const { store, clock } = setup({ history: () => new Promise(resolve => pending.push(() => resolve({ origin: 'source', points: [] }))) });
    const all = ['a:1', 'a:2', 'a:3', 'a:4', 'a:5', 'a:6'].map(id => store.history(id, '24h', 120)); await clock.flush(); assert.equal(pending.length, 4);
    pending.shift()(); await clock.flush(); assert.equal(pending.length, 4); while (pending.length) { pending.shift()(); await clock.flush(); } await Promise.all(all); }],
  ['store: ids of a left page linger 30 s; release detaches the owner', async () => {
    const { store, handles, clock } = setup(), w = store.watch('win', { ids: ['system:cpu', 'system:disk'] }), h = handles[0];
    w.set(['system:uptime']); eq(h.sets[h.sets.length - 1], ['system:uptime', 'system:cpu', 'system:disk']);
    await clock.advance(29999); assert.equal(h.sets.length, 2); await clock.advance(1); eq(h.sets[h.sets.length - 1], ['system:uptime']);
    w.setActive(false); assert.equal(h.active, false); w.release(); assert.equal(h.detached, true); }],
  ['store: a page shown again inside the 250 ms unsubscribe window keeps its data; later it is resubscribed (linger expiry race)', async () => {
    const { E } = loadEngine(['hd-core.js', 'hd-api.js', 'hd-stream.js', 'hd-store.js']), clock = fakeClock(), es = fakeSources(), sapi = fakeStreamApi(es), frames = [];
    const stream = E.stream.create({ api: sapi, setTimeout: clock.setTimeout, clearTimeout: clock.clearTimeout, now: clock.now, random: () => 0.5 });
    const api = { base: '/b', history: () => Promise.resolve({ origin: 'source', points: [] }), widgetCatalog: () => Promise.resolve({ status: 'ok', catalog: { hash: 'h' } }) };
    const store = E.store.create({ api, stream, raf: fn => { frames.push(fn); return frames.length; }, caf: () => {}, setTimeout: clock.setTimeout, clearTimeout: clock.clearTimeout, now: clock.now });
    const beat = async ms => { for (let left = ms; left > 0; left -= 10000) { await clock.advance(Math.min(10000, left)); es.last().emit('hb', {}); } };   // heartbeats keep the watchdog quiet
    const w = store.watch('win', { ids: ['system:cpu'] }); es.last().emit('hello', HELLO); await clock.flush(); eq(sapi.puts[0].ids, ['system:cpu']);
    es.last().emit('snapshot', { entities: [CPU], sources: [] });
    w.set(['system:uptime']); await clock.advance(250); eq(sapi.puts[1].ids, ['system:uptime', 'system:cpu']); es.last().emit('snapshot', { entities: [UPTIME], sources: [] });
    await beat(29750); assert.equal(sapi.puts.length, 2);                 // linger over: the stream coalesces the unsubscribe PUT for 250 ms
    assert.equal(store.view('system:cpu').value, 12.3);                           // the server still sends this id, so the data stays
    w.set(['system:cpu']); await clock.advance(250); assert.equal(sapi.puts.length, 2);   // back inside the window: no PUT, no snapshot, data intact
    assert.equal(store.view('system:cpu').value, 12.3); assert.equal(store.view('system:uptime').value, 7215);
    w.set(['system:uptime']); await beat(30250); eq(sapi.puts[sapi.puts.length - 1].ids, ['system:uptime']);   // unsubscribe PUT acknowledged: the data goes now
    assert.equal(store.view('system:cpu'), null);
    w.set(['system:cpu']); await clock.advance(250); eq(sapi.puts[sapi.puts.length - 1].ids, ['system:cpu', 'system:uptime']);   // a long-gone page resubscribes with a normal PUT
    es.last().emit('snapshot', { entities: [CPU], sources: [] }); assert.equal(store.view('system:cpu').value, 12.3); store.dispose(); stream.dispose(); }],
  ['store: dispose detaches every stream handle and shared() replaces a disposed instance', () => {
    const { E, store, handles, api, bus } = setup(); store.watch('a', { ids: ['system:cpu'] }); store.watch('b', { ids: ['system:uptime'] });
    assert.equal(handles.length, 2); store.dispose(); store.dispose(); assert.ok(handles.every(h => h.detached === true));
    const stream = { on: bus.on, off: bus.off, status: () => ({ phase: 'idle', reason: '' }), attach: () => ({ id: 'x', set() {}, setActive() {}, detach() {} }) };
    const first = E.store.shared({ api, stream }); assert.equal(E.store.shared({ api, stream }), first); first.dispose();
    const second = E.store.shared({ api, stream }); assert.notEqual(second, first); second.watch('w', { ids: ['system:cpu'] }); assert.equal(second.disposed, false); second.dispose(); }],
  ['store: the widget catalog is cached by hash and refreshed with If-None-Match', async () => {
    const asked = []; let answer = { status: 'ok', catalog: { hash: 'h1', widgets: [] } };
    const { store } = setup({ widgetCatalog: etag => { asked.push(etag); return Promise.resolve(answer); } });
    assert.equal((await store.widgetCatalog('h1')).hash, 'h1'); assert.equal((await store.widgetCatalog('h1')).hash, 'h1'); eq(asked, ['']);
    answer = { status: 'ok', not_modified: true }; assert.equal((await store.widgetCatalog('h2')).hash, 'h1'); eq(asked, ['', 'h1']); }],
  ['store (M4 C7): docs cache keeps the newest revision, one editor lock, presence release, focusEditor, dispose clears', () => {
    const { store, clock } = setup(), docs = store.docs, seen = [], locks = [];
    store.on('doc', d => seen.push(d.id + '@' + d.revision)); store.on('lock', d => locks.push(d.id + ':' + d.owner));
    assert.equal(docs.get('a'), null); assert.equal(docs.put('a', { revision: 3, name: 'x' }), true); assert.equal(docs.put('a', { revision: 2 }), false); assert.equal(docs.get('a').revision, 3);
    assert.equal(docs.put('a', { revision: 3, name: 'same' }), true); assert.equal(docs.get('a').doc.name, 'same'); assert.equal(docs.put('a', null), false); assert.equal(docs.put(7, {}), false);
    eq(seen, ['a@3', 'a@3']); docs.drop('a'); assert.equal(docs.get('a'), null); eq(seen, ['a@3', 'a@3', 'a@0']);
    assert.equal(docs.lock('d1', 'A'), true); assert.equal(docs.lock('d1', 'B'), false); assert.equal(docs.lock('d1', 'A'), true); assert.equal(docs.editor('d1'), 'A'); eq(locks, ['d1:A']);
    assert.equal(docs.unlock('d1', 'B'), false); let focused = 0; docs.presence('A', { focus: () => { focused++; } }); assert.equal(docs.focusEditor('d1'), true); assert.equal(focused, 1); assert.equal(docs.focusEditor('nope'), false);
    docs.lock('d2', 'A'); docs.presence('A', null); assert.equal(docs.editor('d1'), null); assert.equal(docs.editor('d2'), null); assert.ok(locks.includes('d1:null') && locks.includes('d2:null')); assert.equal(docs.focusEditor('d1'), false);
    docs.lock('d3', 'A'); docs.put('z', { revision: 1 }); store.dispose(); assert.equal(docs.editor('d3'), null); assert.equal(docs.get('z'), null); void clock; }]
];
