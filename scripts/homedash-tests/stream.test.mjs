import assert from 'node:assert/strict'; import { loadEngine } from './harness.mjs'; import { fakeClock, fakeSources, fakeStreamApi, HELLO } from './live-fakes.mjs';
function setup(extra) {
  const { E } = loadEngine(['hd-core.js', 'hd-api.js', 'hd-stream.js']), clock = fakeClock(), es = fakeSources(), api = fakeStreamApi(es), random = { v: 0.5 };
  const s = E.stream.create(Object.assign({ api, setTimeout: clock.setTimeout, clearTimeout: clock.clearTimeout, now: clock.now, random: () => random.v }, extra));
  const phases = []; s.on('connection', st => phases.push(st.phase + (st.reason ? ':' + st.reason : '')));
  return { E, s, clock, es, api, random, phases };
}
const gone = () => Object.assign(new Error('gone'), { hdApiError: true, status: 404, code: 'stream_not_found', data: {} });
export const tests = [
  ['stream: hello, then PUT of the active owners union; frames pass through', async () => {
    const { s, clock, es, api, phases } = setup(), seen = [];
    for (const type of ['snapshot', 'state', 'entity', 'removed', 'source', 'dashboard_changed']) s.on(type, d => seen.push(type + ':' + JSON.stringify(d)));
    s.attach('win-a', { entities: ['system:cpu', 'system:memory', 'system:cpu', 'bad'], active: true });
    assert.equal(es.list.length, 1); assert.equal(es.last().url, '/api/desktop/homedash/stream'); assert.deepEqual(phases, ['connecting']);
    es.last().emit('hello', HELLO); await clock.flush();
    assert.deepEqual(api.puts, [{ streamId: 's1', ids: ['system:cpu', 'system:memory'] }]); assert.deepEqual(phases, ['connecting', 'live']);
    es.last().emit('snapshot', { entities: [{ id: 'system:cpu' }], sources: [] }); es.last().emit('state', { changes: { 'system:cpu': { value: 1 } } });
    assert.deepEqual(seen, ['snapshot:{"entities":[{"id":"system:cpu"}],"sources":[]}', 'state:{"changes":{"system:cpu":{"value":1}}}']); s.dispose(); }],
  ['stream: the union is sent 250 ms after the last change; inactive owners drop out; hello limits cut it', async () => {
    const { s, clock, es, api } = setup();
    s.attach('a', { entities: ['system:cpu'], active: true }); es.last().emit('hello', HELLO); await clock.flush();
    const b = s.attach('b', { entities: ['system:disk'], active: true }); await clock.advance(249); assert.equal(api.puts.length, 1);
    b.set(['system:disk', 'system:uptime']); await clock.advance(249); assert.equal(api.puts.length, 1); await clock.advance(1);
    assert.deepEqual(api.puts[1].ids, ['system:cpu', 'system:disk', 'system:uptime']);
    b.setActive(false); await clock.advance(250); assert.deepEqual(api.puts[2].ids, ['system:cpu']);
    b.setActive(true); es.last().fail(); await clock.advance(1000); es.last().emit('hello', Object.assign({}, HELLO, { stream_id: 's2', limits: { entities: 2 } })); await clock.flush();
    assert.deepEqual(api.puts[3], { streamId: 's2', ids: ['system:cpu', 'system:disk'] }); s.dispose(); }],
  ['stream: the first ids after an empty subscription go out at once, later changes keep the 250 ms debounce (D8)', async () => {
    const { s, clock, es, api } = setup();
    const a = s.attach('a', { entities: [], active: true }); es.last().emit('hello', HELLO); await clock.flush(); assert.equal(api.puts.length, 0);
    a.set(['system:cpu']); await clock.advance(0); assert.deepEqual(api.puts, [{ streamId: 's1', ids: ['system:cpu'] }]);
    a.set(['system:cpu', 'system:disk']); await clock.advance(249); assert.equal(api.puts.length, 1); await clock.advance(1); assert.deepEqual(api.puts[1].ids, ['system:cpu', 'system:disk']); s.dispose(); }],
  ['stream: errors close the source and back off; failures before hello wait 5 s; 60 s of life resets the backoff', async () => {
    const { s, clock, es, random } = setup();
    s.attach('a', { entities: ['system:cpu'], active: true }); es.last().emit('hello', HELLO); await clock.flush();
    const first = es.last(); first.fail(); assert.equal(first.closed, true); assert.equal(s.status().phase, 'down'); assert.deepEqual(clock.delays(), [1000]);
    await clock.advance(1000); assert.equal(es.list.length, 2);
    es.last().fail(); assert.deepEqual(clock.delays(), [5000]);
    await clock.advance(5000); assert.equal(es.list.length, 3); es.last().emit('hello', HELLO); await clock.flush();
    for (let i = 0; i < 4; i++) { await clock.advance(15000); es.last().emit('hb', { server_time: 'x' }); }
    random.v = 0; es.last().fail(); assert.deepEqual(clock.delays(), [800]);
    random.v = 0.999999; await clock.advance(800); es.last().emit('hello', HELLO); await clock.flush(); es.last().fail(); assert.deepEqual(clock.delays(), [2400]); s.dispose(); }],
  ['stream: 45 s without any frame is a dead stream; hb keeps it alive', async () => {
    const { s, clock, es } = setup();
    s.attach('a', { entities: ['system:cpu'], active: true }); es.last().emit('hello', HELLO); await clock.flush();
    await clock.advance(44000); es.last().emit('hb', {}); await clock.advance(44000); assert.equal(es.list[0].closed, false);
    await clock.advance(1000); assert.equal(es.list[0].closed, true); assert.equal(s.status().reason, 'watchdog'); s.dispose(); }],
  ['stream: PUT 404 reconnects; revoked shutdown reconnects; disabled and auth_lost stop until resume', async () => {
    const { s, clock, es, api, phases } = setup();
    api.setAnswer(gone); s.attach('a', { entities: ['system:cpu'], active: true }); es.last().emit('hello', HELLO); await clock.flush();
    assert.equal(es.list[0].closed, true); assert.equal(s.status().reason, 'stream_not_found');
    api.setAnswer(ids => ({ status: 'ok', accepted: ids, rejected: [] })); await clock.advance(1000); es.last().emit('hello', HELLO); await clock.flush();
    es.last().emit('revoked', { reason: 'shutdown' }); assert.equal(s.status().phase, 'down');
    await clock.advance(5000); es.last().emit('hello', HELLO); await clock.flush(); es.last().emit('revoked', { reason: 'disabled' });
    assert.equal(s.status().phase, 'stopped'); assert.equal(s.status().reason, 'disabled'); const count = es.list.length; await clock.advance(60000); assert.equal(es.list.length, count);
    s.resume(); assert.equal(es.list.length, count + 1); es.last().emit('hello', HELLO); await clock.flush(); es.last().emit('revoked', { reason: 'auth_lost' });
    assert.ok(phases.includes('stopped:auth_lost')); s.dispose(); }],
  ['stream: a refusal before hello asks diagnose; a disabled answer stops', async () => {
    let answer = 'ok', asked = 0;
    const { s, clock, es } = setup({ diagnose: () => { asked += 1; return Promise.resolve(answer); } });
    s.attach('a', { entities: ['system:cpu'], active: true }); es.last().fail(); await clock.flush();
    assert.equal(asked, 1); assert.deepEqual(clock.delays(), [5000]);
    answer = 'disabled'; await clock.advance(5000); es.last().fail(); await clock.flush();
    assert.equal(asked, 2); assert.equal(s.status().phase, 'stopped'); assert.equal(s.status().reason, 'disabled'); assert.deepEqual(clock.delays(), []); s.dispose(); }],
  ['stream: no active owner for 10 s closes the stream; an active owner reconnects; shared() is one per base', async () => {
    const { E, s, clock, es, api } = setup();
    const a = s.attach('a', { entities: ['system:cpu'], active: true }); es.last().emit('hello', HELLO); await clock.flush();
    a.setActive(false); await clock.advance(250); assert.deepEqual(api.puts[1].ids, []);
    await clock.advance(9749); assert.equal(es.list[0].closed, false); await clock.advance(1); assert.equal(es.list[0].closed, true); assert.equal(s.status().phase, 'idle');
    a.setActive(true); assert.equal(es.list.length, 2); s.dispose();
    const x = E.stream.shared({ api }), y = E.stream.shared({ api }); assert.equal(x, y); x.dispose(); assert.notEqual(E.stream.shared({ api }), x); }]
];
