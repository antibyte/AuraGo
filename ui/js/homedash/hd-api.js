// HomeDash API client (F1). The only engine file that knows HomeDash routes (contract "API contract (frozen for M2)" in
// docs/superpowers/plans/2026-11-25-homedash-m2-backend.md). create({base, transport}) wraps the host transport: transport.request(url, init)
// resolves the JSON body or throws an Error with status and body (desktop api(), desktop-foundation.js); transport.eventSource(url) opens SSE.
// Every failure becomes an Error with {hdApiError, status, code, message, data, retryAfter}: code is the body's `code`, else its `error`
// (shared auth, CSRF and desktop layers answer without `status`), else a transport word (aborted, not_modified, network, http_<status>).
(function () {
    'use strict';
    const E = window.HomeDashEngine;
    if (!E || !E.core) throw new Error('hd-api.js: load hd-core.js first');
    const DASHBOARD_ID = /^d_[a-z2-7]{12}$/;
    function normalizeError(err) {
        const e = err || {};
        if (e.hdApiError === true) return e;
        const body = e.body && typeof e.body === 'object' ? e.body : {}, status = Number.isInteger(e.status) ? e.status : 0;
        const code = (typeof body.code === 'string' && body.code) || (typeof body.error === 'string' && body.error)
            || (e.name === 'AbortError' ? 'aborted' : status === 304 ? 'not_modified' : status ? 'http_' + status : 'network');
        const out = new Error((typeof body.message === 'string' && body.message) || (e.message ? String(e.message) : code));
        out.hdApiError = true; out.status = status; out.code = code; out.data = body;
        out.retryAfter = Number.isFinite(e.retryAfter) ? e.retryAfter : Number.isFinite(body.retry_after_ms) ? body.retry_after_ms / 1000 : null;
        return out;
    }
    function query(params) {
        const parts = [];
        for (const [key, value] of Object.entries(params || {})) if (value !== undefined && value !== null && value !== '') parts.push(encodeURIComponent(key) + '=' + encodeURIComponent(String(value)));
        return parts.length ? '?' + parts.join('&') : '';
    }
    const iso = v => (typeof v === 'number' && Number.isFinite(v) ? new Date(v).toISOString() : v);
    function create(options) {
        const o = options || {}, transport = o.transport || {}, base = String(o.base || '/api/desktop/homedash').replace(/\/+$/, '');
        if (typeof transport.request !== 'function') throw new Error('hd-api: transport.request is required');
        const url = (path, params) => base + '/' + path + query(params);
        async function call(method, path, params, body, extra) {
            const x = extra || {}, init = { method, headers: Object.assign({}, x.headers) };
            if (body !== undefined) { init.headers['Content-Type'] = 'application/json'; init.body = JSON.stringify(body); }
            if (x.signal) init.signal = x.signal;
            try { return await transport.request(url(path, params), init); } catch (err) { throw normalizeError(err); }
        }
        const dashboardId = id => { if (!DASHBOARD_ID.test(String(id))) throw normalizeError({ status: 400, body: { code: 'bad_request', message: 'invalid dashboard id' } }); return String(id); };
        return Object.freeze({
            base, url,
            bootstrap: extra => call('GET', 'bootstrap', null, undefined, extra),
            async widgetCatalog(etag, extra) {
                const headers = etag ? { 'If-None-Match': '"' + etag + '"' } : {};
                try { return await call('GET', 'widget-catalog', null, undefined, Object.assign({}, extra, { headers })); }
                catch (err) { if (err.status === 304) return { status: 'ok', not_modified: true }; throw err; }
            },
            history: (id, range, extra) => call('GET', 'history', { id, from: iso(range && range.from), to: iso(range && range.to), points: range && range.points }, undefined, extra),
            dashboards: extra => call('GET', 'dashboards', null, undefined, extra),
            dashboard: async (id, extra) => call('GET', 'dashboard', { id: dashboardId(id) }, undefined, extra),
            createFromTemplate: (template, name, extra) => call('POST', 'dashboards', null, name ? { template, name } : { template }, extra),
            createDocument: (document, extra) => call('POST', 'dashboards', null, { document }, extra),
            deleteDashboard: async (id, revision, extra) => call('DELETE', 'dashboard', { id: dashboardId(id), revision }, undefined, extra),
            putSubscriptions: (streamId, entities, extra) => call('PUT', 'stream/subscriptions', null, { stream_id: streamId, entities: Array.from(entities || []) }, extra),
            openStream() { if (typeof transport.eventSource !== 'function') throw new Error('hd-api: transport.eventSource is required'); return transport.eventSource(url('stream')); }
        });
    }
    E.api = Object.freeze({ create, normalizeError, DASHBOARD_ID });
})();
