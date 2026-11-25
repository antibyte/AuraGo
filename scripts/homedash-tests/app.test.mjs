import assert from 'node:assert/strict'; import { loadEngine, APP_ORDER, read, eq } from './harness.mjs'; import { fakeClock, fakeSources, HELLO } from './live-fakes.mjs';
const CATALOG = JSON.parse(read('internal/homedash/testdata/widget_catalog.json')), STARTER = JSON.parse(read('ui/testdata/homedash-m2/starter-dashboard.json'));
const ENTITIES = JSON.parse(read('ui/testdata/homedash-m2/system-entities.json'));
function contractServer(options) {
  const o = options || {}, docs = new Map(), calls = [];
  const reply = body => Promise.resolve(JSON.parse(JSON.stringify(body))), fail = (status, body) => Promise.reject(Object.assign(new Error('HTTP ' + status), { status, body }));
  function request(url, init) {
    const u = new URL(url, 'http://desk'), path = u.pathname.replace('/api/desktop/homedash/', ''), method = (init && init.method) || 'GET', body = init && init.body ? JSON.parse(init.body) : null;
    calls.push({ method, path, body });
    if (o.disabled) return fail(503, { status: 'error', code: 'homedash_disabled', message: 'off' });
    if (method === 'GET' && path === 'bootstrap') return reply({ status: 'ok', catalog_hash: CATALOG.hash, desktop_readonly: false, sources: [{ source: 'system', state: 'idle', at: 'x' }], templates: ['system'] });
    if (method === 'GET' && path === 'widget-catalog') return reply({ status: 'ok', catalog: CATALOG });
    if (method === 'GET' && path === 'dashboards') return reply({ status: 'ok', dashboards: [...docs.values()].map(d => ({ id: d.id, slug: d.slug, name: d.name, revision: d.revision, approved_revision: d.revision, updated_at: d.updated_at, saved_by: 'user' })) });
    if (method === 'POST' && path === 'dashboards') { const d = Object.assign(JSON.parse(JSON.stringify(STARTER)), { name: body.name }); docs.set(d.id, d); return reply({ status: 'ok', dashboard: d, warnings: [] }); }
    if (method === 'GET' && path === 'dashboard') { const d = docs.get(u.searchParams.get('id')); return d ? reply({ status: 'ok', dashboard: d, schema_too_new: false }) : fail(404, { status: 'error', code: 'dashboard_not_found', message: 'gone' }); }
    if (method === 'PUT' && path === 'stream/subscriptions') return reply({ status: 'ok', accepted: body.entities, rejected: [] });
    if (method === 'GET' && path === 'history') return reply({ status: 'ok', id: u.searchParams.get('id'), origin: 'source', points: [] });
    return fail(404, { status: 'error', code: 'not_found', message: path });
  }
  return { request, calls, docs };
}
function mountApp(options) {
  const o = options || {}, { E, document } = loadEngine(APP_ORDER, { dom: true }), clock = fakeClock(), es = fakeSources(), server = contractServer(o);
  const host = document.createElement('div'); document.body.appendChild(host);
  const inst = E.mount(host, { transport: { request: server.request, eventSource: es.open }, t: k => k, lang: 'en', measure: () => ({ width: 1180, height: 700 }), visibility: o.visibility,
    timers: { setTimeout: clock.setTimeout, clearTimeout: clock.clearTimeout, now: clock.now, random: () => 0.5, raf: fn => clock.setTimeout(fn, 16), caf: id => clock.clearTimeout(id) } });
  return { inst, host, clock, es, server };
}
export const tests = [
  ['app: first run shows the empty home; the starter opens live with six widgets on the shared stream', async () => {
    const { inst, host, clock, es, server } = mountApp(); await clock.advance(20);
    assert.equal(inst.state().mode, 'home'); eq(inst.state().dashboards, []); assert.equal(es.list.length, 1);
    inst.createStarter(); await clock.advance(20);
    eq(server.calls.find(c => c.method === 'POST').body, { template: 'system', name: 'homedash.home.starter_name' });
    assert.equal(inst.state().mode, 'dashboard'); assert.equal(inst.state().dashboard.name, 'homedash.home.starter_name');
    assert.equal(host.querySelectorAll('.hd-page > .hd-widget').length, 6); assert.equal(host.querySelector('.hd-root').getAttribute('data-hd-family'), 'glass');
    es.last().emit('hello', HELLO); await clock.advance(20);
    const puts = server.calls.filter(c => c.method === 'PUT'); assert.equal(puts.length, 1);
    eq(puts[0].body.entities.slice().sort(), ['system:cpu', 'system:disk', 'system:memory', 'system:net_rx', 'system:net_tx', 'system:uptime']);
    es.last().emit('snapshot', { entities: ENTITIES, sources: [] }); await clock.advance(40);
    assert.equal(host.querySelector('[data-hd-wid="memory"] [aria-valuenow]').getAttribute('aria-valuenow'), '50');
    es.last().emit('state', { changes: { 'system:cpu': { value: 97, available: true, updated_at: '2026-11-25T10:00:10Z' } } }); await clock.advance(40);
    assert.equal(host.querySelector('[data-hd-wid="cpu"]').getAttribute('data-hd-tone'), 'crit'); inst.dispose(); }],
  ['app: dashboard_changed reloads a newer revision and goes home when the dashboard is gone', async () => {
    const { inst, host, clock, es, server } = mountApp(); await clock.advance(20); inst.createStarter(); await clock.advance(20); es.last().emit('hello', HELLO); await clock.advance(20);
    const d = server.docs.get('d_m3x7q2k5a4bc'); d.revision = 2; d.look.family = 'tactile';
    es.last().emit('dashboard_changed', { id: d.id, revision: 2, approved_revision: 2 }); await clock.advance(300);
    assert.equal(host.querySelector('.hd-root').getAttribute('data-hd-family'), 'tactile'); assert.equal(inst.state().dashboard.revision, 2);
    server.docs.delete(d.id); es.last().emit('dashboard_changed', { id: d.id, revision: 0, approved_revision: 0 }); await clock.advance(300);
    assert.equal(inst.state().mode, 'home'); assert.equal(inst.state().notice, 'dashboard_deleted'); assert.equal(host.querySelectorAll('.hd-widget').length, 0); inst.dispose(); }],
  ['app: a disabled HomeDash shows the disabled state; a hidden window leaves the stream union', async () => {
    const off = mountApp({ disabled: true }); await off.clock.advance(20); assert.equal(off.inst.state().mode, 'disabled'); off.inst.dispose();
    const subs = [], vis = { visible: () => true, subscribe(fn) { subs.push(fn); return () => {}; } };
    const { inst, clock, es, server } = mountApp({ visibility: vis }); await clock.advance(20); inst.createStarter(); await clock.advance(20); es.last().emit('hello', HELLO); await clock.advance(20);
    for (const fn of subs) fn(false); await clock.advance(300);
    const puts = server.calls.filter(c => c.method === 'PUT'); eq(puts[puts.length - 1].body.entities, []); inst.dispose(); }]
];
