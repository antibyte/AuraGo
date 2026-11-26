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
    calls.push({ method, path, body, keepalive: !!(init && init.keepalive) });
    if (method === 'GET' && o.hold && o.hold.includes(path) && !gates[path]) { const r = request2(u, path, method, body), snap = r; return new Promise(res => { gates[path] = () => res(snap); }).then(x => x); }
    return request2(u, path, method, body);
  }
  function request2(u, path, method, body) {
    if (o.disabled) return fail(503, { status: 'error', code: 'homedash_disabled', message: 'off' });
    if (method === 'GET' && path === 'bootstrap') return reply({ status: 'ok', catalog_hash: CATALOG.hash, desktop_readonly: false, sources: [{ source: 'system', state: 'idle', at: 'x' }], templates: o.templates || ['system'] });
    if (method === 'GET' && path === 'widget-catalog') return reply({ status: 'ok', catalog: CATALOG });
    if (method === 'GET' && path === 'dashboards') return reply({ status: 'ok', dashboards: [...docs.values()].map(d => ({ id: d.id, slug: d.slug, name: d.name, revision: d.revision, approved_revision: d.revision, updated_at: d.updated_at, saved_by: 'user' })) });
    if (method === 'POST' && path === 'dashboards' && body && body.document && body.document.id === '') {   // "save mine as a copy": a document with an empty id
      const c = Object.assign(JSON.parse(JSON.stringify(body.document)), { id: 'd_copycopycopy', slug: 'copy', revision: 1 }); docs.set(c.id, c); return reply({ status: 'ok', dashboard: c, warnings: [] }); }
    if (method === 'POST' && path === 'dashboards' && body && body.template === 'home_assistant' && o.ha) return o.ha(self, body);
    if (method === 'POST' && path === 'commands' && o.commandReply) return o.commandReply(self, body);
    if (method === 'POST' && path === 'dashboards' && o.badCreate && !o.badCreate.done) { o.badCreate.done = true; return reply({ status: 'ok', dashboard: null, warnings: [] }); }
    if (method === 'POST' && path === 'dashboards') { const d = Object.assign(JSON.parse(JSON.stringify(STARTER)), { name: body.name }); docs.set(d.id, d); return reply({ status: 'ok', dashboard: d, warnings: [] }); }
    if (method === 'GET' && path === 'dashboard') { const d = docs.get(u.searchParams.get('id')); return d ? reply({ status: 'ok', dashboard: d, schema_too_new: false }) : fail(404, { status: 'error', code: 'dashboard_not_found', message: 'gone' }); }
    if (method === 'PUT' && path === 'dashboard') {   // M4: save with a revision check; o.putReply(srv, body) may answer first (retry, 409, 403 scripts)
      if (o.putReply) { const r = o.putReply(self, body, u); if (r) return r; }
      const id = u.searchParams.get('id'), d = docs.get(id); if (!d) return fail(404, { status: 'error', code: 'dashboard_not_found', message: 'gone' });
      if (body.revision !== d.revision) return fail(409, { status: 'error', code: 'revision_conflict', message: 'stale', current_revision: d.revision });
      const nd = Object.assign(JSON.parse(JSON.stringify(body.document)), { revision: d.revision + 1, updated_at: '2026-11-26T10:00:00Z' }); docs.set(id, nd); return reply({ status: 'ok', dashboard: nd, warnings: [] }); }
    if (method === 'DELETE' && path === 'dashboard') { const id = u.searchParams.get('id'); if (!docs.has(id)) return fail(404, { status: 'error', code: 'dashboard_not_found', message: 'gone' }); docs.delete(id); return reply({ status: 'ok' }); }
    if (method === 'POST' && path === 'dashboard/duplicate') {
      const d = docs.get(u.searchParams.get('id')); if (!d) return fail(404, { status: 'error', code: 'dashboard_not_found', message: 'gone' });
      const c = Object.assign(JSON.parse(JSON.stringify(d)), { id: 'd_copycopycopy', slug: d.slug + '-copy', name: d.name + ' (copy)', revision: 1 }); docs.set(c.id, c); return reply({ status: 'ok', dashboard: c, warnings: [] }); }
    if (method === 'PUT' && path === 'stream/subscriptions') return reply({ status: 'ok', accepted: body.entities, rejected: [] });
    if (method === 'GET' && path === 'history') return reply({ status: 'ok', id: u.searchParams.get('id'), origin: 'source', points: [] });
    return fail(404, { status: 'error', code: 'not_found', message: path });
  }
  if (o.seed) docs.set(o.seed.id, JSON.parse(JSON.stringify(o.seed)));
  self = { request, calls, docs, opts: o, reply, fail, release: path => gates[path](), created(body, omitted) { const d = Object.assign(JSON.parse(JSON.stringify(STARTER)), { name: body.name }); docs.set(d.id, d); return reply({ status: 'ok', dashboard: d, warnings: [], omitted }); } };
  return self;
}
const fakeStorage = () => { const m = new Map(); return { getItem: k => (m.has(k) ? m.get(k) : null), setItem: (k, v) => { m.set(k, String(v)); }, removeItem: k => { m.delete(k); } }; };
const EN = JSON.parse(read('ui/lang/homedash/en.json'));
function mountApp(options) {
  const o = options || {}, { E, document } = loadEngine(APP_ORDER, { dom: true }), clock = fakeClock(), es = fakeSources(), server = contractServer(o);
  if (o.ddialogs === false) delete E.ddialogs; else if (o.ddialogs) E.ddialogs = o.ddialogs;
  const host = document.createElement('div'); document.body.appendChild(host);
  const apis = new Map(), real = E.widgets.host; E.widgets = Object.assign({}, E.widgets, { host(w) { const inst = real(w); apis.set(w.widget.id, w.api({ widget: w.widget, mode: 'view' })); return inst; } }); // the widget apis the page hands out
  const inst = E.mount(host, { dashboardId: o.dashboardId, readonly: o.readonly, transport: { request: server.request, eventSource: es.open }, t: o.en ? E.core.makeT(EN, { warn() {} }) : k => k, lang: 'en', measure: () => ({ width: 1180, height: 700 }), visibility: o.visibility, storage: o.storage || fakeStorage(), ui: o.ui, canEdit: o.canEdit,
    timers: { setTimeout: clock.setTimeout, clearTimeout: clock.clearTimeout, now: clock.now, random: () => 0.5, raf: fn => clock.setTimeout(fn, 16), caf: id => clock.clearTimeout(id) } });
  return { inst, host, clock, es, server, apis, E, document, timers: { setTimeout: clock.setTimeout, clearTimeout: clock.clearTimeout, now: clock.now, random: () => 0.5, raf: fn => clock.setTimeout(fn, 16), caf: id => clock.clearTimeout(id) } };
}
export { contractServer, mountApp, fakeStorage, STARTER, EN };
// A second window of the same realm (same engine, server, stream and clock).
function second(m, id, o) {
  const host = m.document.createElement('div'); m.document.body.appendChild(host);
  const inst = m.E.mount(host, Object.assign({ dashboardId: id, transport: { request: m.server.request, eventSource: m.es.open }, t: k => k, lang: 'en', measure: () => ({ width: 1180, height: 700 }), timers: m.timers }, o));
  return { inst, host };
}
export const SEED = Object.assign(JSON.parse(JSON.stringify(STARTER)), { revision: 1 });
export const page = (id, widgets, lg, extra) => Object.assign({ id, name: id, icon: 'home', compact: true, widgets, layouts: { lg, md: null, sm: null } }, extra);
export const gauge = (id, entity) => ({ id, type: 'gauge', variant: 'arc', title: '', bindings: entity ? { primary: entity } : {}, options: {}, actions: {}, confirm: false });
export const seedOf = pages => Object.assign(JSON.parse(JSON.stringify(STARTER)), { revision: 1, pages });
// Mounts the real app on the contract server, enters edit mode and returns helpers: key() dispatches on the root with the focused element as target.
export async function edit(o) {
  const opts = Object.assign({ seed: SEED, dashboardId: SEED.id, en: true }, o), m = mountApp(opts); let D = null;
  const real = m.E.designer.create; m.E.designer = Object.assign({}, m.E.designer, { create: x => (D = real(x)) });
  await m.clock.advance(50); m.es.last().emit('hello', HELLO); await m.clock.advance(20);
  assert.equal(m.inst.state().mode, 'dashboard'); assert.equal(await m.inst.enterEdit(o && o.enter), true); await m.clock.advance(20);
  const root = m.host.querySelector('.hd-root'), frame = id => m.host.querySelector('[data-hd-wid="' + id + '"]');
  const key = (k, extra) => { const ev = Object.assign({ type: 'keydown', key: k, target: m.document.activeElement, defaultPrevented: false, preventDefault() { this.defaultPrevented = true; } }, extra); root.dispatchEvent(ev); return ev; };
  return Object.assign(m, { D: () => D, root, frame, key, live: () => m.host.querySelector('.hd-live').textContent, puts: () => m.server.calls.filter(c => c.method === 'PUT' && c.path === 'dashboard'),
    cell: id => ['--hd-x', '--hd-y', '--hd-w', '--hd-h'].map(v => frame(id).style.getPropertyValue(v)).join(','), widgetCount: () => m.host.querySelectorAll('.hd-page > .hd-widget').length });
}
export const click = (m, node) => m.host.querySelector('.hd-editbar').dispatchEvent({ type: 'click', target: node });
export const inRoot = m => m.document.activeElement && m.document.activeElement !== m.document.body && m.root.contains(m.document.activeElement);
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
    inst.retry(); await clock.advance(50); assert.equal(api.entity('primary').readonly, false); inst.dispose(); }],
  // ---- M4 B6: the edit surface of the mount (lock, lanes, reconcile, close guard, actions)
  ['app (M4): enterEdit holds the lock; a second window of the realm cannot edit, show_editor focuses the first window, dispose releases the lock', async () => {
    const focus = [], m = await edit({ ui: { focusWindow: () => focus.push('first') } }), b = second(m, SEED.id); await m.clock.advance(50);
    assert.equal(b.inst.state().mode, 'dashboard'); assert.equal(b.inst.state().edit.editElsewhere, true); assert.equal(b.inst.state().edit.actions.edit.enabled, false); assert.equal(await b.inst.enterEdit(), false);
    assert.equal(b.inst.state().edit.actions.show_editor.available, true); assert.equal(await b.inst.run('show_editor'), true); eq(focus, ['first']); assert.equal(m.inst.state().edit.editElsewhere, false);
    m.inst.dispose(); await m.clock.advance(20); assert.equal(b.inst.state().edit.editElsewhere, false); assert.equal(b.inst.state().edit.actions.show_editor.available, false); assert.equal(await b.inst.enterEdit(), true); b.inst.dispose(); }],
  ['app (M4): a command saves after 800 ms (revision 1 to 2) and a view window re-renders from the docs cache without a GET; an equal-revision doc event is ignored', async () => {
    const m = await edit(), b = second(m, SEED.id); await m.clock.advance(50); const gets = () => m.server.calls.filter(c => c.method === 'GET' && c.path === 'dashboard').length, n = gets(), bx = id => b.host.querySelector('[data-hd-wid="' + id + '"]').style.getPropertyValue('--hd-x');
    assert.equal(bx('cpu'), '0'); m.key('ArrowRight'); await m.clock.advance(850); const put = m.puts()[0]; assert.equal(put.body.revision, 1); assert.equal(put.keepalive, true); await m.clock.advance(20);
    assert.equal(m.inst.state().edit.phase, 'saved'); assert.equal(m.inst.state().dashboard.revision, 2); assert.equal(bx('cpu'), '1'); assert.equal(b.inst.state().dashboard.revision, 2); assert.equal(gets(), n, 'no GET for the second window');
    const store = m.E.store.shared({ api: { base: '/api/desktop/homedash' } }), c = store.docs.get(SEED.id); assert.equal(c.revision, 2); assert.equal(c.doc.revision, 2, 'the cached document carries the new revision');
    const fam = (rev, family) => store.docs.put(SEED.id, Object.assign({}, c.doc, { revision: rev, look: Object.assign({}, c.doc.look, { family }) })), root = () => b.host.querySelector('.hd-root').getAttribute('data-hd-family');
    fam(2, 'minimal'); assert.equal(root(), 'glass', 'an equal revision is ignored'); fam(3, 'minimal'); assert.equal(root(), 'minimal'); assert.equal(b.inst.state().dashboard.revision, 3); b.inst.dispose(); m.inst.dispose(); }],
  ['app (M4): a foreign save while clean reloads (GET, toast, history cleared, lane at the new revision); while dirty it is a conflict and nothing is sent', async () => {
    const m = await edit(), foreign = (mm, rev, patch) => { const d = mm.server.docs.get(SEED.id); d.revision = rev; Object.assign(d.look, patch); mm.es.last().emit('dashboard_changed', { id: d.id, revision: rev, approved_revision: rev }); };
    m.key('ArrowRight'); await m.clock.advance(900); assert.equal(m.inst.state().edit.canUndo, true); const gets = () => m.server.calls.filter(c => c.method === 'GET' && c.path === 'dashboard').length, n = gets();
    foreign(m, 3, { family: 'tactile' }); await m.clock.advance(400); assert.equal(gets(), n + 1); assert.equal(m.root.getAttribute('data-hd-family'), 'tactile'); assert.equal(m.inst.state().edit.canUndo, false); assert.equal(m.inst.state().edit.phase, 'saved');
    assert.match(m.host.querySelector('.hd-toast').textContent, /another window or browser/); m.key('ArrowDown'); await m.clock.advance(900); assert.equal(m.puts().at(-1).body.revision, 3, 'the lane continues at the foreign revision'); m.inst.dispose();
    const d = await edit(); d.key('ArrowRight'); foreign(d, 2, { family: 'minimal' }); await d.clock.advance(400); assert.equal(d.inst.state().edit.phase, 'conflict'); assert.equal(d.host.querySelector('.hd-conflict-bar').getAttribute('hidden'), null); assert.equal(d.root.getAttribute('data-hd-family'), 'glass', 'the local document stays');
    await d.clock.advance(3000); assert.equal(d.puts().length, 0, 'no PUT while in conflict'); assert.equal(d.inst.state().edit.actions.resolve_conflict.enabled, true); d.inst.dispose(); }],
  ['app (M4): a PUT answered 409 stops in conflict and opens the sheet; Keep mine overwrites with the current revision; Take theirs reloads and clears the history', async () => {
    const m = await edit(), btn = (mm, id) => mm.host.querySelector('.hd-sheet-btn[data-action="' + id + '"]'), press = (mm, id) => { const b = btn(mm, id); b.closest('.hd-sheet-layer').dispatchEvent({ type: 'click', target: b }); };
    m.server.docs.get(SEED.id).revision = 3; m.key('ArrowRight'); await m.clock.advance(850); assert.equal(m.inst.state().edit.phase, 'conflict'); assert.equal(m.puts().length, 1); assert.equal(m.puts()[0].body.revision, 1); assert.ok(btn(m, 'mine') && btn(m, 'theirs') && btn(m, 'copy') && btn(m, 'later'));
    press(m, 'mine'); await m.clock.advance(50); assert.equal(m.puts().length, 2); assert.equal(m.puts()[1].body.revision, 3); assert.equal(m.inst.state().edit.phase, 'saved'); assert.equal(m.inst.state().dashboard.revision, 4); assert.equal(m.host.querySelector('.hd-conflict-bar').getAttribute('hidden'), 'true'); assert.match(m.live(), /replaced the newer one/); m.inst.dispose();
    const t = await edit(); const sd = t.server.docs.get(SEED.id); sd.revision = 3; sd.look.family = 'minimal'; t.key('ArrowRight'); await t.clock.advance(850); assert.equal(t.inst.state().edit.phase, 'conflict'); press(t, 'theirs'); await t.clock.advance(50);
    assert.equal(t.inst.state().edit.phase, 'saved'); assert.equal(t.root.getAttribute('data-hd-family'), 'minimal'); assert.equal(t.inst.state().edit.canUndo, false); assert.equal(t.puts().length, 1, 'nothing more was sent'); t.key('ArrowDown'); await t.clock.advance(850); assert.equal(t.puts()[1].body.revision, 3); t.inst.dispose(); }],
  ['app (M4): Save mine as a copy posts the document with an empty id and opens the copy in edit mode', async () => {
    const m = await edit(); m.server.docs.get(SEED.id).revision = 3; m.key('ArrowRight'); await m.clock.advance(850); const b = m.host.querySelector('.hd-sheet-btn[data-action="copy"]'); b.closest('.hd-sheet-layer').dispatchEvent({ type: 'click', target: b }); await m.clock.advance(200);
    const post = m.server.calls.find(c => c.method === 'POST' && c.path === 'dashboards'); assert.equal(post.body.document.id, ''); assert.equal(post.body.document.slug, ''); assert.match(post.body.document.name, / \(copy\)$/); assert.equal(post.body.document.pages[0].layouts.lg[0].x, 1, 'the local change travels with the copy');
    assert.equal(m.inst.state().dashboard.id, 'd_copycopycopy'); assert.equal(m.inst.state().edit.mode, 'design'); assert.match(m.host.querySelector('.hd-toast').textContent, /was saved as/); m.inst.dispose(); }],
  ['app (M4): beforeClose is true when clean; with a failing PUT it asks, Keep stays, Leave closes and the lane keeps retrying after dispose', async () => {
    const m = await edit({ putReply: srv => srv.fail(503, { status: 'error', code: 'storage_unavailable', message: 'x' }) }); assert.equal(await m.inst.beforeClose(), true); m.key('ArrowRight'); await m.clock.advance(850); assert.equal(m.inst.state().edit.phase, 'retrying');
    const pick = async id => { const p = m.inst.beforeClose(); await m.clock.advance(50); const b = m.host.querySelector('.hd-sheet-btn[data-action="' + id + '"]'); assert.ok(b, id); b.closest('.hd-sheet-layer').dispatchEvent({ type: 'click', target: b }); return p; };
    assert.equal(await pick('keep'), false); assert.equal(await pick('leave'), true); const n = m.puts().length; m.inst.dispose(); await m.clock.advance(70000); assert.ok(m.puts().length > n + 2, 'the realm lane keeps retrying'); }],
  ['app (M4): dispose of a dirty editing mount flushes one keepalive PUT and the lane stays in the realm; the lock is released', async () => {
    const m = await edit({ putReply: srv => srv.fail(503, { status: 'error', code: 'storage_unavailable', message: 'x' }) }); m.key('ArrowRight'); assert.equal(m.puts().length, 0); m.inst.dispose(); assert.equal(m.puts().length, 1); assert.equal(m.puts()[0].keepalive, true);
    await m.clock.advance(2100); assert.equal(m.puts().length, 2, 'the detached lane retried'); const b = second(m, SEED.id); await m.clock.advance(50); assert.equal(b.inst.state().edit.editElsewhere, false); assert.equal(b.host.querySelector('[data-hd-wid="cpu"]').style.getPropertyValue('--hd-x'), '1', 'a new window shows the unsaved local document'); b.inst.dispose(); }],
  ['app (M4): setReadonly(true) while editing leaves edit mode and offers no edit; setReadonly(false) allows it again', async () => {
    const m = await edit(); m.inst.setReadonly(true); await m.clock.advance(10); assert.equal(m.inst.state().edit.mode, 'view'); assert.equal(m.inst.state().edit.reason, 'readonly'); assert.equal(m.inst.state().edit.canEdit, false); assert.equal(await m.inst.enterEdit(), false);
    assert.equal(m.host.querySelector('.hd-editbar'), null); m.inst.setReadonly(false); assert.equal(m.inst.state().edit.canEdit, true); assert.equal(await m.inst.run('edit'), true); assert.equal(m.inst.state().edit.mode, 'design'); m.inst.dispose(); }],
  ['app (M4): the view-mode context menu lists more info for a light and edit widget when editing is possible; a non-light has only edit', async () => {
    const LIGHT = { id: 'ha:light.kitchen', source: 'ha', kind: 'light', name: 'Kitchen', capabilities: [{ name: 'toggle', allowed: true }, { name: 'turn_on', allowed: true }, { name: 'turn_off', allowed: true }], readonly: false, state: { value: true, available: true, updated_at: '2026-11-25T10:00:00Z' } };
    const seed = seedOf([page('main', [{ id: 'k', type: 'light', variant: 'tile', title: '', bindings: { primary: 'ha:light.kitchen' }, options: {}, actions: {}, confirm: false }, gauge('g', 'system:cpu')], [{ id: 'k', x: 0, y: 0, w: 2, h: 2 }, { id: 'g', x: 2, y: 0, w: 3, h: 3 }])]);
    const m = mountApp({ seed, dashboardId: seed.id }); await m.clock.advance(50); m.es.last().emit('hello', HELLO); m.es.last().emit('snapshot', { entities: [LIGHT], sources: [] }); await m.clock.advance(40);
    const fk = m.host.querySelector('[data-hd-wid="k"]'), more = fk.querySelector('.hd-more'); assert.ok(more && !more.hasAttribute('hidden')); let clicked = 0; more.click = () => { clicked++; };
    const menu = m.inst.contextMenu({ target: fk, clientX: 5, clientY: 6 }); eq(menu.items.map(i => i.id), ['more_info', 'edit_widget']); assert.equal(menu.x, 5); menu.items[0].run(); assert.equal(clicked, 1);
    eq(m.inst.contextMenu({ target: m.host.querySelector('[data-hd-wid="g"]'), clientX: 1, clientY: 1 }).items.map(i => i.id), ['edit_widget']); assert.equal(m.inst.contextMenu({ target: m.host.querySelector('.hd-stage'), clientX: 1, clientY: 1 }), null);
    menu.items[1].run(); await m.clock.advance(20); assert.equal(m.inst.state().edit.mode, 'design'); eq(m.inst.state().edit.selection, ['k']); eq(m.inst.contextMenu({ target: m.host.querySelector('[data-hd-wid="k"]'), clientX: 1, clientY: 1 }).items.slice(0, 2).map(i => i.id), ['edit', 'duplicate']); m.inst.dispose(); }],
  ['app (M4): actions follow E.ddialogs (rename, import, export, look, add_page unavailable without it); import needs no dashboard and no write access; run hands over a ctx with sheetHost', async () => {
    const m = mountApp({ ddialogs: false }); await m.clock.advance(20); const A = () => m.inst.state().edit.actions; for (const k of ['rename', 'look', 'add_page', 'export', 'import']) assert.equal(A()[k].available, false, k);
    assert.equal(A().new_starter.available, true); assert.equal(A().new_starter.enabled, true); assert.equal(await m.inst.run('rename'), false); assert.equal(m.inst.state().edit.reason, 'not_dashboard'); m.inst.dispose();
    const seen = [], sheetHost = () => 'host', dd = { run: async (ctx, action, arg) => { seen.push([ctx, action, arg]); return true; } }, n = mountApp({ ddialogs: dd, ui: { sheetHost } }); await n.clock.advance(20); const B = () => n.inst.state().edit.actions;
    assert.equal(n.inst.state().mode, 'home'); assert.equal(B().import.available, true); assert.equal(B().import.enabled, true); assert.equal(B().rename.enabled, false, 'rename needs an open dashboard'); assert.equal(await n.inst.run('import', 'x'), true);
    const [ctx, action, arg] = seen[0]; assert.equal(action, 'import'); assert.equal(arg, 'x'); assert.equal(ctx.sheetHost, sheetHost); assert.equal(typeof ctx.sheet.open, 'function'); assert.equal(ctx.designer, null); assert.equal(ctx.mount, n.inst); assert.equal(ctx.state().mode, 'home'); assert.ok(ctx.root && ctx.api && ctx.store);
    n.inst.setReadonly(true); assert.equal(B().import.enabled, false); assert.equal(await n.inst.run('import'), false); assert.equal(seen.length, 1); n.inst.dispose(); }],
  ['app (M4): duplicate_dashboard flushes and opens the copy; delete_dashboard confirms in the engine sheet, removes it and returns home', async () => {
    const m = await edit(); m.key('ArrowRight'); const p = m.inst.run('duplicate_dashboard'); await m.clock.advance(200); assert.equal(await p, true); assert.equal(m.puts().length, 1, 'the change was flushed before the copy'); assert.equal(m.inst.state().dashboard.id, 'd_copycopycopy'); assert.equal(m.inst.state().edit.mode, 'view'); assert.match(m.host.querySelector('.hd-toast').textContent, /created/);
    const q = m.inst.run('delete_dashboard'); await m.clock.advance(20); const ok = m.host.querySelector('.hd-sheet-btn[data-action="confirm"]'); assert.ok(ok); ok.closest('.hd-sheet-layer').dispatchEvent({ type: 'click', target: ok }); assert.equal(await q, true); await m.clock.advance(50);
    assert.equal(m.inst.state().mode, 'home'); assert.equal(m.server.docs.has('d_copycopycopy'), false); assert.equal(m.server.docs.has(SEED.id), true); m.inst.dispose(); }]
];
