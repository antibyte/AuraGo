// Runs ui/js/dashboard/dash-api.js in a Node vm context with a stubbed fetch and checks request sharing,
// the ttl cache, force, invalidate(prefix) and the failure semantics of the old main.js API helpers.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

const file = path.join(__dirname, '..', 'js', 'dashboard', 'dash-api.js');
const source = fs.readFileSync(file, 'utf8');

function load(respond) {
    const calls = [];
    const toasts = [];
    const ctx = {
        location: { href: '/dashboard', pathname: '/dashboard' },
        t: key => key,
        showToast: (message, type) => toasts.push(message + '|' + type),
        setTimeout: () => 0,
        structuredClone,
        fetch: url => {
            calls.push(url);
            return respond(url, calls.length);
        },
    };
    ctx.window = ctx;
    vm.createContext(ctx);
    vm.runInContext(source, ctx, { filename: file });
    const clock = { now: 1000 };
    vm.runInContext('Date', ctx).now = () => clock.now;
    return { api: ctx.DashAPI, calls, toasts, clock, ctx };
}

const answer = body => Promise.resolve({ status: 200, ok: true, json: () => Promise.resolve(body) });
const failure = status => Promise.resolve({ status, ok: false, json: () => Promise.resolve({}) });

async function main() {
    {
        const { api, calls } = load((url, n) => answer({ n }));
        const [a, b] = await Promise.all([api.get('/api/x'), api.get('/api/x')]);
        assert.equal(calls.length, 1, 'concurrent calls for one url share one request');
        assert.equal(a.n, 1);
        assert.equal(b.n, 1);
        assert.notEqual(a, b, 'every caller gets its own copy');
        a.n = 99;
        const again = await api.getWithStatus('/api/x');
        assert.equal(again.ok, true);
        assert.equal(again.data.n, 1, 'a caller cannot change the cached answer');
        assert.equal(calls.length, 1, 'get and getWithStatus share the cache');
    }
    {
        const { api, calls, clock } = load((url, n) => answer({ n }));
        await api.get('/api/x');
        clock.now += 14999;
        assert.equal((await api.get('/api/x')).n, 1, 'cached for the default 15 s');
        clock.now += 2;
        assert.equal((await api.get('/api/x')).n, 2, 'refetched once the ttl is over');
        await api.get('/api/y', { ttl: 0 });
        await api.get('/api/y', { ttl: 0 });
        assert.equal(calls.filter(url => url === '/api/y').length, 2, 'ttl 0 never caches');
    }
    {
        const { api, calls } = load((url, n) => answer({ n }));
        await api.get('/api/dashboard/memory');
        await api.get('/api/dashboard/memory?days=7');
        await api.get('/api/budget');
        assert.equal((await api.get('/api/budget', { force: true })).n, 4, 'force skips the cache');
        assert.equal((await api.get('/api/budget')).n, 4, 'a forced answer refreshes the cache');
        api.invalidate('/api/dashboard/memory');
        await api.get('/api/dashboard/memory');
        await api.get('/api/dashboard/memory?days=7');
        await api.get('/api/budget');
        assert.equal(calls.length, 6, 'invalidate drops every url with the prefix and keeps the rest');
    }
    {
        const release = [];
        const { api, calls } = load((url, n) => new Promise(resolve => release.push(() => resolve({ status: 200, ok: true, json: () => Promise.resolve({ n }) }))));
        const running = api.get('/api/dashboard/memory');
        api.invalidate('/api/dashboard/memory');
        release.shift()();
        assert.equal((await running).n, 1, 'a running caller still gets its answer');
        const next = api.get('/api/dashboard/memory');
        assert.equal(calls.length, 2, 'an answer invalidated while running is not cached');
        release.shift()();
        assert.equal((await next).n, 2);
    }
    {
        let mode = 500;
        const { api, calls, ctx, toasts } = load(() => (mode === 'offline' ? Promise.reject(new Error('offline')) : failure(mode)));
        const failed = await api.getWithStatus('/api/x');
        assert.equal(failed.ok, false);
        assert.equal(failed.status, 500);
        assert.equal(await api.get('/api/x'), null);
        assert.equal(calls.length, 2, 'failures are not cached');
        assert.equal(toasts.length, 0, 'http errors show no toast');
        mode = 401;
        assert.equal((await api.getWithStatus('/api/x')).status, 401);
        assert.equal(ctx.location.href, '/auth/login?redirect=%2Fdashboard', 'a 401 sends the browser to the login page');
        mode = 'offline';
        assert.equal(await api.get('/api/x'), null);
        assert.equal(await api.get('/api/x'), null);
        assert.equal(toasts.length, 1, 'network errors toast once per 15 s');
        assert.equal(toasts[0], 'dashboard.api_error|error');
        assert.equal((await api.getWithStatus('/api/x')).status, 0);
    }
    console.log('DashAPI: request sharing, ttl cache, force, invalidate and failure handling pass.');
}

main().catch(error => { console.error(error); process.exitCode = 1; });
