import assert from 'node:assert/strict'; import { loadEngine, APP_ORDER, read, eq } from './harness.mjs'; import { fakeClock, fakeSources, HELLO } from './live-fakes.mjs';
const CATALOG = JSON.parse(read('internal/homedash/testdata/widget_catalog.json')), STARTER = JSON.parse(read('ui/testdata/homedash-m2/starter-dashboard.json'));
const ENTITIES = JSON.parse(read('ui/testdata/homedash-m2/system-entities.json'));
const HA_DOC = { id: 'd_hacmdhacmdab', slug: 'ha', name: 'HA', revision: 1, saved_by: 'user', look: { family: 'glass', palette: { mode: 'theme' } },
  pages: [{ id: 'main', name: 'Main', widgets: [{ id: 'sw', type: 'switch', variant: 'pill', bindings: { primary: 'ha:switch.coffee' } }], layouts: { lg: [{ id: 'sw', x: 0, y: 0, w: 2, h: 1 }] } }] };
const HA_SWITCH = { id: 'ha:switch.coffee', source: 'ha', kind: 'switch', name: 'Coffee', capabilities: [{ name: 'toggle', allowed: true }, { name: 'turn_on', allowed: true }, { name: 'turn_off', allowed: true }], readonly: false, state: { value: false, available: true, updated_at: '2026-11-25T10:00:00Z' } };
function contractServer(options) {
  const o = options || {}, docs = new Map(), calls = [];
  const gates = {}; // o.hold = ['dashboard', ...]: the first GET of that path answers with its request-time snapshot only after gates[path]() runs
  let self = null;
  const reply = body => Promise.resolve(JSON.parse(JSON.stringify(body))), fail = (status, body) => Promise.reject(Object.assign(new Error('HTTP ' + status), { status, body }));
  function request(url, init) {
    const u = new URL(url, 'http://desk'), path = u.pathname.replace('/api/desktop/homedash/', ''), method = (init && init.method) || 'GET', body = init && init.body ? JSON.parse(init.body) : null;
    calls.push({ method, path, body });
    if (method === 'GET' && o.hold && o.hold.includes(path) && !gates[path]) { const r = request2(u, path, method, body), snap = r; return new Promise(res => { gates[path] = () => res(snap); }).then(x => x); }
    return request2(u, path, method, body);
  }
  function request2(u, path, method, body) {
    if (o.disabled) return fail(503, { status: 'error', code: 'homedash_disabled', message: 'off' });
    if (method === 'GET' && path === 'bootstrap') return reply({ status: 'ok', catalog_hash: CATALOG.hash, desktop_readonly: false, sources: [{ source: 'system', state: 'idle', at: 'x' }], templates: o.templates || ['system'] });
    if (method === 'GET' && path === 'widget-catalog') return reply({ status: 'ok', catalog: CATALOG });
    if (method === 'GET' && path === 'dashboards') return reply({ status: 'ok', dashboards: [...docs.values()].map(d => ({ id: d.id, slug: d.slug, name: d.name, revision: d.revision, approved_revision: d.revision, updated_at: d.updated_at, saved_by: 'user' })) });
    if (method === 'POST' && path === 'dashboards' && body && body.template === 'home_assistant' && o.ha) return o.ha(self, body);
    if (method === 'POST' && path === 'commands' && o.commandReply) return o.commandReply(self, body);
    if (method === 'POST' && path === 'dashboards' && o.badCreate && !o.badCreate.done) { o.badCreate.done = true; return reply({ status: 'ok', dashboard: null, warnings: [] }); }
    if (method === 'POST' && path === 'dashboards') { const d = Object.assign(JSON.parse(JSON.stringify(STARTER)), { name: body.name }); docs.set(d.id, d); return reply({ status: 'ok', dashboard: d, warnings: [] }); }
    if (method === 'GET' && path === 'dashboard') { const d = docs.get(u.searchParams.get('id')); return d ? reply({ status: 'ok', dashboard: d, schema_too_new: false }) : fail(404, { status: 'error', code: 'dashboard_not_found', message: 'gone' }); }
    if (method === 'PUT' && path === 'stream/subscriptions') return reply({ status: 'ok', accepted: body.entities, rejected: [] });
    if (method === 'GET' && path === 'history') return reply({ status: 'ok', id: u.searchParams.get('id'), origin: 'source', points: [] });
    return fail(404, { status: 'error', code: 'not_found', message: path });
  }
  if (o.seed) docs.set(o.seed.id, JSON.parse(JSON.stringify(o.seed)));
  self = { request, calls, docs, opts: o, reply, fail, release: path => gates[path](), created(body, omitted) { const d = Object.assign(JSON.parse(JSON.stringify(STARTER)), { name: body.name }); docs.set(d.id, d); return reply({ status: 'ok', dashboard: d, warnings: [], omitted }); } };
  return self;
}
function mountApp(options) {
  const o = options || {}, { E, document } = loadEngine(APP_ORDER, { dom: true }), clock = fakeClock(), es = fakeSources(), server = contractServer(o);
  const host = document.createElement('div'); document.body.appendChild(host);
  const apis = new Map(), real = E.widgets.host; E.widgets = Object.assign({}, E.widgets, { host(w) { const inst = real(w); apis.set(w.widget.id, w.api({ widget: w.widget, mode: 'view' })); return inst; } }); // the widget apis the page hands out
  const inst = E.mount(host, { dashboardId: o.dashboardId, readonly: o.readonly, transport: { request: server.request, eventSource: es.open }, t: k => k, lang: 'en', measure: () => ({ width: 1180, height: 700 }), visibility: o.visibility,
    timers: { setTimeout: clock.setTimeout, clearTimeout: clock.clearTimeout, now: clock.now, random: () => 0.5, raf: fn => clock.setTimeout(fn, 16), caf: id => clock.clearTimeout(id) } });
  return { inst, host, clock, es, server, apis, E };
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
    const puts = server.calls.filter(c => c.method === 'PUT'); eq(puts[puts.length - 1].body.entities, []); inst.dispose(); }],
  ['app: a list that arrives while an open is in flight is reconciled after the open (newer revision reloads)', async () => {
    const seed = Object.assign(JSON.parse(JSON.stringify(STARTER)), { revision: 1 });
    const { inst, clock, es, server } = mountApp({ seed, dashboardId: seed.id, hold: ['dashboard'] }); await clock.advance(20);
    assert.equal(inst.state().mode, 'loading'); es.last().emit('hello', HELLO); await clock.advance(20);
    server.docs.get(seed.id).revision = 2; es.last().emit('dashboard_changed', { id: seed.id, revision: 2, approved_revision: 2 }); await clock.advance(300);
    assert.equal(inst.state().mode, 'loading'); server.release('dashboard'); await clock.advance(300);
    assert.equal(inst.state().mode, 'dashboard'); assert.equal(inst.state().dashboard.revision, 2); assert.equal(server.calls.filter(c => c.path === 'dashboard').length, 2); inst.dispose(); }],
  ['app: a refresh requested during a fetch runs exactly one more fetch and its callers get the newer list', async () => {
    const { inst, clock, server } = mountApp({ hold: ['dashboards'] }); await clock.advance(20);
    assert.equal(inst.state().mode, 'home'); inst.createStarter(); await clock.advance(20);
    assert.equal(inst.state().mode, 'dashboard'); server.release('dashboards'); await clock.advance(20);
    assert.equal(server.calls.filter(c => c.method === 'GET' && c.path === 'dashboards').length, 2);
    inst.home(); assert.equal(inst.state().dashboards.length, 1); inst.dispose(); }],
  ['app: a failed open after create clears busy so the next create is not dropped', async () => {
    const { inst, clock, server } = mountApp({ badCreate: {} }); await clock.advance(20);
    inst.createEmpty(); await clock.advance(20); assert.equal(inst.state().mode, 'home'); assert.equal(inst.state().notice, 'open_failed'); assert.equal(inst.state().busy, false);
    inst.createEmpty(); await clock.advance(20); assert.equal(server.calls.filter(c => c.method === 'POST').length, 2); assert.equal(inst.state().mode, 'dashboard'); inst.dispose(); }],
  ['app: dispose during boot stops it before shared realm state or further requests', async () => {
    const { inst, clock, server } = mountApp(); inst.dispose(); await clock.advance(50);
    eq(server.calls.map(c => c.path), ['bootstrap', 'widget-catalog']); }],
  ['app: a retry keeps the live read-only flag the host set after the window opened', async () => {
    const { inst, clock, server } = mountApp({ readonly: true }); await clock.advance(20);
    assert.equal(inst.state().desktopReadonly, true); inst.createEmpty(); await clock.advance(20); assert.equal(server.calls.filter(c => c.method === 'POST').length, 0);
    inst.setReadonly(false); assert.equal(inst.state().desktopReadonly, false);
    inst.retry(); await clock.advance(20); assert.equal(inst.state().desktopReadonly, false);
    inst.createEmpty(); await clock.advance(20); assert.equal(server.calls.filter(c => c.method === 'POST').length, 1); assert.equal(inst.state().mode, 'dashboard'); inst.dispose(); }],
  ['app: boot issues bootstrap, widget-catalog and the dashboard before any of them answers', async () => {
    const seed = JSON.parse(JSON.stringify(STARTER)), { inst, clock, es, server } = mountApp({ seed, dashboardId: seed.id, hold: ['bootstrap'] }); await clock.advance(20);
    eq(server.calls.map(c => c.path).sort(), ['bootstrap', 'dashboard', 'widget-catalog']); assert.equal(inst.state().mode, 'loading');
    server.release('bootstrap'); await clock.advance(50); assert.equal(inst.state().mode, 'dashboard'); assert.equal(inst.state().dashboard.id, seed.id);
    eq(inst.state().templates, ['system']); assert.equal(es.list.length, 1); eq(server.calls.filter(c => c.path === 'dashboard').length, 1); inst.dispose(); }],
  ['app: a dashboard that fails to load while bootstrap succeeds goes home with the missing notice', async () => {
    const { inst, clock } = mountApp({ dashboardId: 'd_zzzzzzzzzzzz' }); await clock.advance(50);
    assert.equal(inst.state().mode, 'home'); assert.equal(inst.state().notice, 'dashboard_missing'); inst.dispose(); }],
  ['app: createHomeAssistant posts the template with its names and reports the omitted devices', async () => {
    const { inst, host, clock, server } = mountApp({ templates: ['system', 'home_assistant'], ha: (srv, body) => srv.created(body, 4) }); await clock.advance(20);
    eq(inst.state().templates, ['system', 'home_assistant']);
    inst.createHomeAssistant(); await clock.advance(20);
    eq(server.calls.find(c => c.method === 'POST').body, { template: 'home_assistant', name: 'homedash.home.ha_name', unassigned_title: 'homedash.home.ha_unassigned' });
    assert.equal(inst.state().mode, 'dashboard'); assert.equal(inst.state().notice, 'template_omitted'); assert.equal(inst.state().noticeCount, 4);
    eq(inst.state().banners.map(b => [b.kind, b.count]), [['template_omitted', 4]]); assert.ok(host.querySelector('.hd-root')); inst.dispose(); }],
  ['app: a Home Assistant template without devices and an unreachable source end in home notices', async () => {
    const empty = mountApp({ ha: srv => srv.fail(422, { status: 'error', code: 'template_empty', message: 'none' }) }); await empty.clock.advance(20);
    empty.inst.createHomeAssistant(); await empty.clock.advance(20);
    assert.equal(empty.inst.state().mode, 'home'); assert.equal(empty.inst.state().notice, 'template_empty'); assert.equal(empty.inst.state().busy, false); empty.inst.dispose();
    const down = mountApp({ ha: srv => srv.fail(503, { status: 'error', code: 'source_unavailable', message: 'down', source: 'ha' }) }); await down.clock.advance(20);
    down.inst.createHomeAssistant(); await down.clock.advance(20);
    assert.equal(down.inst.state().notice, 'source_unavailable'); assert.equal(down.inst.state().noticeSource, 'ha'); down.inst.dispose();
    const bare = mountApp({ ha: srv => srv.fail(503, { status: 'error', code: 'source_unavailable', message: 'down' }) }); await bare.clock.advance(20);
    bare.inst.createHomeAssistant(); await bare.clock.advance(20); assert.equal(bare.inst.state().noticeSource, 'ha'); bare.inst.dispose(); }],
  ['app: home re-reads bootstrap and the template list follows it; a returning window does the same', async () => {
    const vis = { on: null, visible: () => true, subscribe(fn) { vis.on = fn; return () => {}; } };
    const { inst, clock, server } = mountApp({ visibility: vis }); await clock.advance(20); eq(inst.state().templates, ['system']);
    const reads = () => server.calls.filter(c => c.path === 'bootstrap').length, before = reads();
    server.opts.templates = ['system', 'home_assistant']; inst.home(); await clock.advance(20);
    assert.ok(reads() > before); eq(inst.state().templates, ['system', 'home_assistant']);
    server.opts.templates = ['system']; const n = reads(); vis.on(true); await clock.advance(20); assert.equal(reads(), n + 1); eq(inst.state().templates, ['system']); inst.dispose(); }],
  ['app: the Home Assistant source turning live re-reads bootstrap while the home is shown', async () => {
    const { inst, clock, es, server } = mountApp(); await clock.advance(20); es.last().emit('hello', HELLO); await clock.advance(20);
    const n = server.calls.filter(c => c.path === 'bootstrap').length;
    es.last().emit('snapshot', { entities: [], sources: [{ source: 'ha', state: 'starting', at: 'x' }] }); await clock.advance(20);
    es.last().emit('snapshot', { entities: [], sources: [{ source: 'ha', state: 'live', at: 'x' }] }); await clock.advance(20);
    assert.ok(server.calls.filter(c => c.path === 'bootstrap').length > n); inst.dispose(); }],
  ['app: a command answered 403 desktop_readonly switches every entity view to read-only and the app to the read-only desktop', async () => {
    const seed = HA_DOC, { inst, clock, es, apis, server } = mountApp({ seed, dashboardId: seed.id, commandReply: srv => srv.fail(403, { status: 'error', code: 'desktop_readonly', message: 'ro', blocked_by: 'virtual_desktop.readonly' }) });
    await clock.advance(50); es.last().emit('hello', HELLO); await clock.advance(20); es.last().emit('snapshot', { entities: [HA_SWITCH], sources: [] }); await clock.advance(40);
    const api = apis.get('sw'); assert.equal(api.can('primary', 'turn_on'), true); assert.equal(inst.state().desktopReadonly, false);
    await api.command('primary', 'turn_on', {}); await clock.advance(20);
    assert.equal(inst.state().desktopReadonly, true); assert.equal(api.entity('primary').readonly, true); assert.equal(api.entity('primary').readonlyReason, 'virtual_desktop.readonly');
    assert.equal(api.can('primary', 'turn_on'), false); assert.equal(server.calls.filter(c => c.path === 'commands').length, 1);
    assert.equal(inst.state().banners.some(b => b.kind === 'readonly'), true);
    inst.setReadonly(false); assert.equal(inst.state().desktopReadonly, false); assert.equal(api.entity('primary').readonly, false); assert.equal(api.can('primary', 'turn_on'), true); inst.dispose(); }],
  ['app: a 403 read_only lock ends when the stream is live again after a reconnect', async () => {
    const seed = HA_DOC, { inst, clock, es, apis } = mountApp({ seed, dashboardId: seed.id, commandReply: srv => srv.fail(403, { status: 'error', code: 'read_only', message: 'ro', blocked_by: 'home_assistant.blocked_services' }) });
    await clock.advance(50); es.last().emit('hello', HELLO); await clock.advance(20); es.last().emit('snapshot', { entities: [HA_SWITCH], sources: [] }); await clock.advance(40);
    const api = apis.get('sw'); await api.command('primary', 'turn_on', {}); await clock.advance(20);
    assert.equal(api.entity('primary').readonly, true); assert.equal(api.entity('primary').readonlyReason, 'home_assistant.blocked_services'); assert.equal(inst.state().desktopReadonly, false);
    es.last().fail(); await clock.advance(20); assert.equal(api.entity('primary').readonly, true, 'still locked while the stream is down');
    await clock.advance(70000); assert.ok(es.list.length >= 2); es.last().emit('hello', HELLO); await clock.advance(20);
    assert.equal(api.entity('primary').readonly, false); assert.equal(api.can('primary', 'turn_on'), true); inst.dispose(); }],
  ['app: re-reading bootstrap (retry) drops a 403 read_only lock', async () => {
    const seed = HA_DOC, { inst, clock, es, apis } = mountApp({ seed, dashboardId: seed.id, commandReply: srv => srv.fail(403, { status: 'error', code: 'read_only', message: 'ro', blocked_by: 'home_assistant.readonly' }) });
    await clock.advance(50); es.last().emit('hello', HELLO); await clock.advance(20); es.last().emit('snapshot', { entities: [HA_SWITCH], sources: [] }); await clock.advance(40);
    const api = apis.get('sw'); await api.command('primary', 'turn_on', {}); await clock.advance(20); assert.equal(api.entity('primary').readonly, true);
    inst.retry(); await clock.advance(50); assert.equal(api.entity('primary').readonly, false); inst.dispose(); }]
];
