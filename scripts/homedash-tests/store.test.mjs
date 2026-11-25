import assert from 'node:assert/strict'; import { loadEngine, eq } from './harness.mjs'; import { fakeClock } from './live-fakes.mjs';
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
    answer = { status: 'ok', not_modified: true }; assert.equal((await store.widgetCatalog('h2')).hash, 'h1'); eq(asked, ['', 'h1']); }]
];
