// AuraGo dashboard – cached GET layer (window.DashAPI). Classic script, loaded with defer before main.js.
// get/getWithStatus keep the semantics of the old main.js API helpers: a 401 sends the browser to the login
// page; get() returns null on failure and shows one network-error toast per 15 s. On top of that:
//  - concurrent calls for the same URL share one request;
//  - successful JSON answers are cached per URL for `ttl` ms (default 15 s; 0 = no cache);
//  - `force` skips the cache and any running request, and its answer refreshes the cache;
//  - invalidate(prefix) drops cache entries and running requests whose URL starts with prefix.
// Every caller gets its own copy of the data, so one renderer cannot change what another one sees.
(function () {
    'use strict';

    const DEFAULT_TTL_MS = 15000;
    const cache = new Map();    // url -> { data, expires }
    const inflight = new Map(); // url -> Promise<{ ok: true, data } | { ok: false, status }>
    let networkToastShown = false;

    function copy(data) {
        if (data === null || typeof data !== 'object') return data;
        return typeof structuredClone === 'function' ? structuredClone(data) : JSON.parse(JSON.stringify(data));
    }

    function request(url) {
        return fetch(url, { credentials: 'same-origin' }).then(response => {
            if (response.status === 401) {
                window.location.href = '/auth/login?redirect=' + encodeURIComponent(window.location.pathname);
                return { ok: false, status: 401 };
            }
            if (!response.ok) return { ok: false, status: response.status };
            return response.json().then(data => ({ ok: true, data }), () => ({ ok: false, status: 0 }));
        }, () => ({ ok: false, status: 0 }));
    }

    function load(url, ttl, force) {
        const maxAge = Number(ttl) > 0 ? Number(ttl) : 0;
        if (!force && maxAge > 0) {
            const hit = cache.get(url);
            if (hit && hit.expires > Date.now()) return Promise.resolve({ ok: true, data: hit.data });
        }
        const running = force ? null : inflight.get(url);
        if (running) return running;
        const pending = request(url).then(result => {
            // Only the newest request for a URL may fill the cache (force and invalidate replace it).
            if (inflight.get(url) === pending) {
                inflight.delete(url);
                if (result.ok && maxAge > 0) cache.set(url, { data: result.data, expires: Date.now() + maxAge });
            }
            return result;
        });
        inflight.set(url, pending);
        return pending;
    }

    function notifyNetworkError() {
        if (networkToastShown || typeof showToast !== 'function') return;
        networkToastShown = true;
        showToast(t('dashboard.api_error'), 'error', 5000);
        setTimeout(() => { networkToastShown = false; }, 15000);
    }

    function get(url, { ttl = DEFAULT_TTL_MS, force = false } = {}) {
        return load(url, ttl, force).then(result => {
            if (result.ok) return copy(result.data);
            if (result.status === 0) notifyNetworkError();
            return null;
        });
    }

    function getWithStatus(url, { ttl = DEFAULT_TTL_MS, force = false } = {}) {
        return load(url, ttl, force).then(result => (result.ok
            ? { ok: true, data: copy(result.data) }
            : { ok: false, status: result.status }));
    }

    function invalidate(prefix) {
        const start = String(prefix == null ? '' : prefix);
        [cache, inflight].forEach(map => {
            Array.from(map.keys()).forEach(url => {
                if (url.startsWith(start)) map.delete(url);
            });
        });
    }

    window.DashAPI = { get, getWithStatus, invalidate };
})();
