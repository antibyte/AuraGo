import assert from 'node:assert/strict'; import { loadEngine, eq } from './harness.mjs';
const load = () => loadEngine(['hd-core.js', 'hd-api.js']).E;
function fakeTransport(answers) {
  const calls = [];
  return { calls, request(url, init) { calls.push({ url, init }); const a = answers.shift(); return a && a.reject ? Promise.reject(a.reject) : Promise.resolve(a); },
    eventSource(url) { return { url }; } };
}
const httpError = (status, body) => Object.assign(new Error('HTTP ' + status), { status, body });
export const tests = [
  ['hd-api builds every M2 route under the base path with encoded query values', async () => {
    const E = load(), tr = fakeTransport(Array.from({ length: 7 }, () => ({ status: 'ok' })));
    const api = E.api.create({ base: '/api/desktop/homedash/', transport: tr });
    await api.bootstrap(); await api.dashboards(); await api.dashboard('d_m3x7q2k5a4bc');
    await api.history('system:net_rx', { from: Date.UTC(2026, 10, 25, 9), to: Date.UTC(2026, 10, 25, 10), points: 60 });
    await api.createFromTemplate('system', 'System'); await api.createDocument({ name: 'Mein Dashboard', pages: [{ id: 'main', name: 'Hauptseite' }] });
    await api.putSubscriptions('9f2c', ['system:cpu']);
    assert.deepEqual(tr.calls.map(c => c.init.method + ' ' + c.url), [
      'GET /api/desktop/homedash/bootstrap', 'GET /api/desktop/homedash/dashboards', 'GET /api/desktop/homedash/dashboard?id=d_m3x7q2k5a4bc',
      'GET /api/desktop/homedash/history?id=system%3Anet_rx&from=2026-11-25T09%3A00%3A00.000Z&to=2026-11-25T10%3A00%3A00.000Z&points=60',
      'POST /api/desktop/homedash/dashboards', 'POST /api/desktop/homedash/dashboards', 'PUT /api/desktop/homedash/stream/subscriptions']);
    eq(JSON.parse(tr.calls[4].init.body), { template: 'system', name: 'System' });
    eq(JSON.parse(tr.calls[5].init.body), { document: { name: 'Mein Dashboard', pages: [{ id: 'main', name: 'Hauptseite' }] } });
    eq(JSON.parse(tr.calls[6].init.body), { stream_id: '9f2c', entities: ['system:cpu'] });
    assert.equal(tr.calls[4].init.headers['Content-Type'], 'application/json'); assert.equal(tr.calls[0].init.body, undefined);
    assert.equal(api.openStream().url, '/api/desktop/homedash/stream'); }],
  ['hd-api keys errors on status plus code, falling back to error and then to the transport', async () => {
    const E = load();
    const cases = [
      [httpError(409, { status: 'error', code: 'revision_conflict', message: 'stale', current_revision: 9 }), 409, 'revision_conflict'],
      [httpError(403, { error: 'csrf_check_failed', message: 'origin' }), 403, 'csrf_check_failed'],
      [httpError(403, { error: 'desktop_readonly', code: 'desktop_readonly', message: 'read-only' }), 403, 'desktop_readonly'],
      [httpError(401, { error: 'unauthorized', redirect: '/auth/login' }), 401, 'unauthorized'],
      [httpError(502, {}), 502, 'http_502'], [new TypeError('Failed to fetch'), 0, 'network'],
      [Object.assign(new Error('aborted'), { name: 'AbortError' }), 0, 'aborted']];
    for (const [err, status, code] of cases) {
      const api = E.api.create({ transport: fakeTransport([{ reject: err }]) });
      await assert.rejects(api.dashboards(), e => e.hdApiError === true && e.status === status && e.code === code, code);
    }
    const conflict = E.api.normalizeError(cases[0][0]); assert.equal(conflict.data.current_revision, 9); assert.equal(conflict.message, 'stale');
    assert.equal(E.api.normalizeError(httpError(429, { code: 'rate_limited', retry_after_ms: 1500 })).retryAfter, 1.5);
    assert.equal(E.api.normalizeError(conflict), conflict); }],
  ['hd-api treats 304 on the widget catalog as not modified and refuses malformed dashboard ids before any request', async () => {
    const E = load(), tr = fakeTransport([{ reject: httpError(304, {}) }]), api = E.api.create({ transport: tr });
    eq(await api.widgetCatalog('abc'), { status: 'ok', not_modified: true });
    assert.equal(tr.calls[0].init.headers['If-None-Match'], '"abc"');
    await assert.rejects(api.dashboard('../etc'), e => e.status === 400 && e.code === 'bad_request');
    await assert.rejects(api.deleteDashboard('d_ABC', 1), e => e.code === 'bad_request');
    assert.equal(tr.calls.length, 1); }]
];
