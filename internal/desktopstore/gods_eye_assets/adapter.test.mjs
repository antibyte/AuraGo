import assert from 'node:assert/strict';
import { mkdtemp, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { test } from 'node:test';
import { createServer } from 'vite';
import adapter, { frameOrigins } from './aurago.config.mjs';

test('AuraGo frame policy, managed settings and provider proxies', async () => {
    for (const value of ['broken', '{}', '["https://*.example"]', '["https://user:pass@example.com"]',
        '["https://example.com/path"]', '["javascript:alert(1)"]', '["https://example.com:99999"]',
        '["https://example.com;evil"]', '["https://example.com#fragment"]']) {
        assert.deepEqual(frameOrigins(value), [], value);
    }
    assert.deepEqual(frameOrigins('["https://aurago.example/","https://aurago.example"]'), ['https://aurago.example']);
    const keys = ['CESIUM_ION_TOKEN', 'GOOGLE_MAPS_API_KEY', 'OPENAI_API_KEY', 'AISSTREAM_API_KEY',
        'FIRMS_MAP_KEY', 'TOMTOM_API_KEY', 'OPENSKY_CLIENT_ID', 'OPENSKY_CLIENT_SECRET', 'LL2_API_TOKEN'];
    for (const key of keys) delete process.env[key];
    delete process.env.AURAGO_FRAME_ORIGINS;
    const denied = await adapter({ mode: 'development' });
    assert.equal(denied.server.headers['Content-Security-Policy'], "frame-ancestors 'none'");
    assert.equal(denied.server.headers['X-Frame-Options'], 'DENY');

    process.env.AURAGO_FRAME_ORIGINS = '["https://aurago.example","https://aurago.tailnet.ts.net"]';
    const config = await adapter({ mode: 'development' });
    assert(!config.plugins.some(plugin => plugin.name === 'gev-key-setup'));
    for (const name of ['opensky-proxy', 'tomtom-proxy', 'firms-proxy', 'rocket-launches-proxy',
        'celestrak-proxy', 'ais-live-proxy']) {
        assert(config.plugins.some(plugin => plugin.name === name), `missing live proxy ${name}`);
    }
    assert.notEqual(config.server.allowedHosts, true);
    assert(config.server.allowedHosts.includes('aurago.tailnet.ts.net'));
    const cache = await mkdtemp(join(tmpdir(), 'aurago-gev-test-'));
    const server = await createServer({ ...config, configFile: false, cacheDir: cache,
        optimizeDeps: { noDiscovery: true, include: [] },
        server: { ...config.server, host: '127.0.0.1', port: 0, strictPort: false } });
    const nativeFetch = globalThis.fetch;
    const seen = new Set();
    const failures = [];
    const json = body => new Response(JSON.stringify(body), { headers: { 'Content-Type': 'application/json' } });
    try {
        await server.listen();
        const base = `http://127.0.0.1:${server.httpServer.address().port}`;
        const document = await nativeFetch(base);
        assert.equal(document.status, 200);
        assert.equal(document.headers.get('content-security-policy'),
            'frame-ancestors https://aurago.example https://aurago.tailnet.ts.net');
        assert.equal(document.headers.get('x-frame-options'), null);
        assert.equal((await nativeFetch(base + '/aurago-health')).status, 200);
        for (const method of ['GET', 'POST', 'PUT', 'DELETE']) {
            const reply = await nativeFetch(base + '/api/setup/keys', { method });
            assert.equal(reply.status, 403);
            assert.equal(reply.headers.get('cache-control'), 'no-store');
        }
        assert.equal((await nativeFetch(base + '/api/google/nearby-places?lat=0&lon=0').then(r => r.json())).configured, false);
        assert.equal((await nativeFetch(base + '/api/tomtom/status').then(r => r.json())).hasKey, false);
        assert.equal((await nativeFetch(base + '/api/firms/status').then(r => r.json())).hasKey, false);
        assert.equal((await nativeFetch(base + '/api/realtime/token')).status, 503);

        // Synthetic credentials and responses only: no real provider quota is used.
        for (const key of keys) process.env[key] = `fixture-${key}`;
        const configured = await adapter({ mode: 'development' });
        assert.equal(configured.define['import.meta.env.CESIUM_ION_TOKEN'], '"fixture-CESIUM_ION_TOKEN"');
        assert.equal(configured.define['import.meta.env.GOOGLE_MAPS_API_KEY'], '"fixture-GOOGLE_MAPS_API_KEY"');
        assert.equal(Object.keys(configured.define).length, 2, 'server credentials must not become client defines');
        globalThis.fetch = async (input, init = {}) => {
            try {
            const url = new URL(String(input));
            const headers = new Headers(init.headers);
            if (url.host === new URL(base).host) return nativeFetch(input, init);
            assert(!headers.has('Cookie'), 'AuraGo session cookie forwarded');
            seen.add(url.hostname);
            if (url.hostname === 'api.openai.com') {
                assert.equal(headers.get('Authorization'), 'Bearer fixture-OPENAI_API_KEY');
                return json({ value: 'fixture-ephemeral-token', expires_at: 1 });
            }
            if (url.hostname === 'places.googleapis.com') {
                assert.equal(headers.get('X-Goog-Api-Key'), 'fixture-GOOGLE_MAPS_API_KEY');
                return json({ places: [{ id: 'fixture-place', displayName: { text: 'Fixture place' }, location: { latitude: 0, longitude: 0 } }] });
            }
            if (url.hostname === 'api.tomtom.com') {
                assert.equal(url.searchParams.get('key'), 'fixture-TOMTOM_API_KEY');
                return new Response('fixture-vector-tile');
            }
            if (url.hostname === 'firms.modaps.eosdis.nasa.gov') {
                assert(String(url).includes('fixture-FIRMS_MAP_KEY'));
                return json({ current_transactions: 3, transaction_limit: 5000 });
            }
            if (url.hostname === 'll.thespacedevs.com') {
                assert.equal(headers.get('Authorization'), 'Token fixture-LL2_API_TOKEN');
                return json({ results: [] });
            }
            if (url.hostname === 'auth.opensky-network.org') {
                assert.equal(new URLSearchParams(init.body).get('client_secret'), 'fixture-OPENSKY_CLIENT_SECRET');
                return json({ access_token: 'fixture-opensky-access', expires_in: 3600 });
            }
            if (url.hostname === 'opensky-network.org') {
                assert.equal(headers.get('Authorization'), 'Bearer fixture-opensky-access');
                return json({ time: Math.floor(Date.now() / 1000), states: [] });
            }
            throw new Error(`unexpected provider host ${url.hostname}`);
            } catch (err) {
                failures.push(err);
                throw err;
            }
        };
        for (const path of ['/api/realtime/token', '/api/google/nearby-places?lat=0&lon=0',
            '/api/tomtom/flow/8/127/127.pbf', '/api/firms/status', '/api/launches', '/api/opensky']) {
            const response = await nativeFetch(base + path, { headers: { Cookie: 'aurago_session=fixture', Authorization: 'Bearer fixture-aura-session' } });
            assert.equal(response.status, 200, path + ': ' + await response.text());
        }
        for (const host of ['api.openai.com', 'places.googleapis.com', 'api.tomtom.com',
            'firms.modaps.eosdis.nasa.gov', 'll.thespacedevs.com', 'auth.opensky-network.org', 'opensky-network.org']) {
            assert(seen.has(host), `provider proxy did not execute: ${host}`);
        }
        assert.deepEqual(failures, [], 'provider mocks must not fail behind upstream fallback handling');
    } finally {
        globalThis.fetch = nativeFetch;
        await server.close();
        await rm(cache, { recursive: true, force: true });
        for (const key of keys) delete process.env[key];
        delete process.env.AURAGO_FRAME_ORIGINS;
    }
});
