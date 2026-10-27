/* Docker Manager store: owns the requests to /api/docker-manager, the polling and the state the views render.
   A classic script (window.DockerManagerStore). It touches the DOM only in attach() (visibility observers), so
   scripts/test-docker-manager-ui.mjs can run it in a Node vm with a fake api. Rules it keeps:
   - every request goes through the desktop api() it is given; the Docker engine is never contacted from the browser;
   - one request in flight per key (the host list, one container list per host, logs, inspect): a second call joins it;
   - polling every 5 s only while the window is visible; hiding the window aborts the polls;
   - a failed container list keeps the last containers of that host and records the error, so one unreachable host
     never empties the table or blocks the other hosts. */
(function () {
    'use strict';

    const BASE = '/api/docker-manager';
    const POLL_MS = 5000;
    // An unreachable host can take the server up to 30 s to give up, so the browser waits longer than that.
    const REQUEST_TIMEOUT_MS = 35000;
    const TEST_TIMEOUT_MS = 45000;
    const LOG_TAILS = [100, 200, 500, 1000, 5000];
    const enc = encodeURIComponent;

    function create(options) {
        const opts = options || {};
        const model = window.DockerManagerModel;
        const api = opts.api;
        const requestTimeout = Number(opts.requestTimeoutMs) > 0 ? Number(opts.requestTimeoutMs) : REQUEST_TIMEOUT_MS;
        if (typeof api !== 'function') throw new TypeError('DockerManagerStore needs an api function');
        const lifetime = new AbortController();
        const listeners = new Set();
        const state = {
            hosts: [], hostsLoaded: false, hostsFailed: null, disabled: false,
            scope: typeof opts.scope === 'string' && opts.scope ? opts.scope : 'all', byHost: {}, detail: null, busy: new Set(),
            readonly: !!opts.readonly, dockerReadonly: false,
            visible: false, disposed: false, lastRefreshAt: 0
        };
        const inflight = new Map();
        const controllers = new Map();
        const sequence = new Map();
        let timer = 0, win = null, root = null, inView = true, detach = () => {}, detailGen = 0;

        function emit() {
            if (state.disposed) return;
            for (const listener of [...listeners]) {
                try { listener(state); } catch (error) { console.error('Docker Manager:', error); }
            }
        }
        function subscribe(listener) { listeners.add(listener); return () => listeners.delete(listener); }

        // ---- requests --------------------------------------------------------------------------------------------

        // Resolves with the response body. Rejects with an Error whose .dm is {code, message, reason, owner, field, status}.
        // A request registered under `key` can be aborted by that key (hiding the window, closing the drawer).
        function request(path, init, key) {
            const o = init || {};
            const controller = new AbortController();
            let timedOut = false;
            const onEnd = () => controller.abort();
            if (lifetime.signal.aborted) controller.abort(); else lifetime.signal.addEventListener('abort', onEnd, { once: true });
            const timeout = setTimeout(() => { timedOut = true; controller.abort(); }, o.timeout || requestTimeout);
            if (key) controllers.set(key, controller);
            return (async () => {
                try {
                    const init2 = { method: o.method || 'GET', signal: controller.signal };
                    if (o.body !== undefined) { init2.headers = { 'Content-Type': 'application/json' }; init2.body = JSON.stringify(o.body); }
                    return await api(BASE + path, init2);
                } catch (error) {
                    const info = model.normalizeError(error);
                    if (timedOut) info.code = 'timeout';
                    throw Object.assign(new Error(info.message || info.code), { dm: info });
                } finally {
                    clearTimeout(timeout);
                    lifetime.signal.removeEventListener('abort', onEnd);
                    if (key && controllers.get(key) === controller) controllers.delete(key);
                }
            })();
        }

        // Records what a failure means for the whole app and returns its info; null for an aborted request.
        function noteError(error) {
            const info = (error && error.dm) || model.normalizeError(error);
            if (info.code === 'aborted') return null;
            if (info.code === 'docker_disabled') state.disabled = true;
            if (info.code === 'read_only') state.dockerReadonly = true;
            return info;
        }

        function once(key, fn) {
            if (inflight.has(key)) return inflight.get(key);
            const promise = (async () => {
                try { return await fn(); } finally { if (inflight.get(key) === promise) inflight.delete(key); }
            })();
            inflight.set(key, promise);
            return promise;
        }
        const pending = () => [...inflight.keys()].some(key => key === 'hosts' || key.startsWith('c:'));

        // ---- hosts -----------------------------------------------------------------------------------------------

        function mergeHost(host) {
            if (!host || !host.id) return;
            const index = state.hosts.findIndex(item => item.id === host.id);
            state.hosts = index < 0 ? state.hosts.concat([host]) : state.hosts.map((item, at) => (at === index ? host : item));
        }

        function loadHosts() {
            return once('hosts', async () => {
                try {
                    const body = await request('/hosts', {}, 'hosts');
                    if (state.disposed) return false;
                    state.hosts = Array.isArray(body.hosts) ? body.hosts : [];
                    state.hostsLoaded = true;
                    state.hostsFailed = null;
                    state.disabled = false;
                    const ids = new Set(state.hosts.map(host => host.id));
                    for (const id of Object.keys(state.byHost)) if (!ids.has(id)) delete state.byHost[id];
                    if (state.scope !== 'all' && !ids.has(state.scope)) state.scope = 'all';
                    return true;
                } catch (error) {
                    const info = noteError(error);
                    if (info && !state.disabled) state.hostsFailed = info;
                    return false;
                } finally { emit(); }
            });
        }

        function loadContainers(hostId) {
            return once('c:' + hostId, async () => {
                const seq = (sequence.get(hostId) || 0) + 1;
                sequence.set(hostId, seq);
                try {
                    const body = await request('/hosts/' + enc(hostId) + '/containers', {}, 'c:' + hostId);
                    if (state.disposed || sequence.get(hostId) !== seq) return false;
                    state.byHost[hostId] = { containers: Array.isArray(body.containers) ? body.containers : [], loaded: true, error: null, at: Date.now() };
                    return true;
                } catch (error) {
                    let info = noteError(error);
                    if (state.disposed || !info || sequence.get(hostId) !== seq) return false;
                    // The browser giving up on a slow host is the same news as the server giving up: unreachable, timed out.
                    if (info.code === 'timeout') info = Object.assign({}, info, { code: 'host_unreachable', reason: 'timeout' });
                    const previous = state.byHost[hostId] || { containers: [], loaded: false, at: 0 };
                    state.byHost[hostId] = Object.assign({}, previous, { error: info });
                    return false;
                } finally { emit(); }
            });
        }

        // After an action the list must come from the engine's state after the action, never from a poll that started before.
        function reloadContainers(hostId) {
            controllers.get('c:' + hostId)?.abort();
            inflight.delete('c:' + hostId);
            return loadContainers(hostId);
        }

        function scopeIds() {
            if (state.scope === 'all') return state.hosts.map(host => host.id);
            return state.hosts.some(host => host.id === state.scope) ? [state.scope] : [];
        }

        async function refresh() {
            if (state.disposed) return;
            await loadHosts();
            if (state.disposed || state.disabled) return;
            await Promise.all(scopeIds().map(loadContainers));
            state.lastRefreshAt = Date.now();
            emit();
        }

        function setScope(scope) {
            const next = scope !== 'all' && state.hosts.some(host => host.id === scope) ? scope : 'all';
            if (next === state.scope) return;
            state.scope = next;
            emit();
            if (state.visible) void Promise.all(scopeIds().map(loadContainers));
        }

        // ---- polling and visibility -------------------------------------------------------------------------------

        function hiddenNow() {
            return state.disposed || (typeof document !== 'undefined' && document.hidden)
                || !!(root && typeof root.checkVisibility === 'function' && !root.checkVisibility());
        }
        function pollOnce() {
            if (state.disposed || !state.visible || state.disabled || hiddenNow()) return false;
            // GET /hosts answers from cached statuses and refreshes stale ones in the background, so it is polled like the lists.
            const jobs = scopeIds().map(loadContainers);
            jobs.push(loadHosts());
            void Promise.all(jobs);
            return true;
        }
        function schedule() {
            clearTimeout(timer);
            timer = 0;
            if (state.disposed || !state.visible) return;
            timer = setTimeout(() => { timer = 0; pollOnce(); schedule(); }, POLL_MS);
        }
        function abortPolls() {
            for (const [key, controller] of [...controllers]) if (key === 'hosts' || key.startsWith('c:')) controller.abort();
        }
        function setVisible(next) {
            if (state.disposed || next === state.visible) return;
            state.visible = next;
            if (!next) { clearTimeout(timer); timer = 0; abortPolls(); }
            else { void refresh(); schedule(); }
            emit();
        }
        function attach(target) {
            const t = target || {};
            win = t.win || null;
            root = t.root || null;
            const check = () => {
                const shown = !state.disposed && !document.hidden && inView
                    && (!win || (!win.hidden && !win.classList.contains('vd-space-hidden') && win.style.display !== 'none' && win.getClientRects().length > 0));
                setVisible(shown);
            };
            document.addEventListener('visibilitychange', check);
            const windowObserver = typeof MutationObserver === 'function' && win ? new MutationObserver(check) : null;
            windowObserver?.observe(win, { attributes: true, attributeFilter: ['style', 'class', 'hidden', 'data-space-hidden'] });
            const intersection = typeof IntersectionObserver === 'function' && root
                ? new IntersectionObserver(entries => { inView = !!entries[entries.length - 1]?.isIntersecting; check(); }, { threshold: 0.01 }) : null;
            if (intersection) { inView = false; intersection.observe(root); }
            const onPolicy = event => {
                const detail = (event && event.detail) || {};
                const readonly = detail.readonly === true || detail.read_only === true || detail.enabled === false ? true
                    : detail.readonly === false || detail.read_only === false || detail.enabled === true ? false : null;
                if (readonly !== null && readonly !== state.readonly) { state.readonly = readonly; emit(); }
            };
            document.addEventListener('aurago:desktop-policy', onPolicy);
            detach = () => {
                document.removeEventListener('visibilitychange', check);
                document.removeEventListener('aurago:desktop-policy', onPolicy);
                windowObserver?.disconnect();
                intersection?.disconnect();
            };
            check();
        }

        // ---- container actions -------------------------------------------------------------------------------------

        const canMutate = () => !state.readonly && !state.dockerReadonly;
        const canEditHosts = () => !state.readonly;
        const busyKey = (hostId, containerId) => hostId + '/' + containerId;

        async function mutateContainer(hostId, containerId, run) {
            const key = busyKey(hostId, containerId);
            if (state.disposed || state.busy.has(key)) return { ok: false, error: { code: 'busy' } };
            state.busy.add(key);
            emit();
            try {
                await run();
                if (state.disposed) return { ok: false, error: { code: 'aborted' } };
                await reloadContainers(hostId);
                return { ok: true };
            } catch (error) {
                return { ok: false, error: noteError(error) || { code: 'aborted' } };
            } finally {
                state.busy.delete(key);
                emit();
            }
        }

        // action: start | stop | restart | pause | unpause | kill. The server never asks to confirm these.
        function act(hostId, containerId, action) {
            return mutateContainer(hostId, containerId, () => request('/hosts/' + enc(hostId) + '/containers/' + enc(containerId) + '/' + enc(action), { method: 'POST' }));
        }

        function remove(hostId, containerId, o) {
            const params = [];
            if (o && o.force) params.push('force=1');
            if (o && o.confirmProtected) params.push('confirm=protected');
            const query = params.length ? '?' + params.join('&') : '';
            return mutateContainer(hostId, containerId, () => request('/hosts/' + enc(hostId) + '/containers/' + enc(containerId) + query, { method: 'DELETE' }));
        }

        // ---- detail drawer: overview comes from the list, logs and inspect are fetched on demand -------------------

        function abortDetail() { controllers.get('logs')?.abort(); controllers.get('inspect')?.abort(); detailGen += 1; }

        function ensureDetailData() {
            const d = state.detail;
            if (!d) return;
            if (d.tab === 'logs' && d.logs.status === 'idle') void loadLogs();
            if (d.tab === 'inspect' && d.inspect.status === 'idle') void loadInspect();
        }

        function openDetail(row, tab) {
            if (state.disposed || !row) return;
            const same = state.detail && state.detail.key === row.key;
            if (!same) {
                abortDetail();
                state.detail = {
                    key: row.key, hostId: row.hostId, id: row.c.full_id || row.c.id, tab: tab || 'overview',
                    logs: { status: 'idle', text: '', tail: 200, error: null }, inspect: { status: 'idle', data: null, error: null }
                };
            } else if (tab) state.detail.tab = tab;
            emit();
            ensureDetailData();
        }
        function closeDetail() {
            if (!state.detail) return;
            abortDetail();
            state.detail = null;
            emit();
        }
        function setDetailTab(tab) {
            if (!state.detail || state.detail.tab === tab) return;
            state.detail.tab = tab;
            emit();
            ensureDetailData();
        }

        async function loadDetailPart(part, route, apply) {
            const target = state.detail;
            if (!target) return false;
            const gen = detailGen;
            controllers.get(part)?.abort();
            target[part].status = 'loading';
            emit();
            try {
                const body = await request('/hosts/' + enc(target.hostId) + '/containers/' + enc(target.id) + route, {}, part);
                if (state.disposed || gen !== detailGen || state.detail !== target) return false;
                apply(target, body);
                target[part].status = 'ok';
                target[part].error = null;
            } catch (error) {
                const info = noteError(error);
                if (state.disposed || !info || gen !== detailGen || state.detail !== target) return false;
                target[part].status = 'error';
                target[part].error = info;
            }
            emit();
            return true;
        }
        function loadLogs(tail) {
            if (state.detail && LOG_TAILS.includes(tail)) state.detail.logs.tail = tail;
            const lines = state.detail ? state.detail.logs.tail : 200;
            return loadDetailPart('logs', '/logs?tail=' + lines, (d, body) => { d.logs.text = typeof body.logs === 'string' ? body.logs : ''; });
        }
        function loadInspect() {
            return loadDetailPart('inspect', '/inspect', (d, body) => { d.inspect.data = body.inspect === undefined ? null : body.inspect; });
        }

        // ---- host management (these reject with .dm; the dialogs show the message) ---------------------------------

        async function guarded(run) {
            try { return await run(); } catch (error) { noteError(error); emit(); throw error; }
        }
        // The draft is not saved. When it belongs to a stored host its id goes along, so the server reuses the stored secrets.
        async function testDraft(payload, id) {
            const draft = id ? Object.assign({}, payload, { id }) : payload;
            const body = await guarded(() => request('/hosts/test', { method: 'POST', body: draft, timeout: TEST_TIMEOUT_MS }));
            return body.host_status || {};
        }
        async function saveHost(id, payload) {
            const body = await guarded(() => (id
                ? request('/hosts/' + enc(id), { method: 'PUT', body: payload })
                : request('/hosts', { method: 'POST', body: payload })));
            mergeHost(body.host);
            if (body.host) { delete state.byHost[body.host.id]; void loadContainers(body.host.id); }
            emit();
            return body.host;
        }
        async function deleteHost(id) {
            await guarded(() => request('/hosts/' + enc(id), { method: 'DELETE' }));
            state.hosts = state.hosts.filter(host => host.id !== id);
            delete state.byHost[id];
            if (state.scope === id) state.scope = 'all';
            if (state.detail && state.detail.hostId === id) closeDetail();
            emit();
        }
        async function testHost(id) {
            const body = await guarded(() => request('/hosts/' + enc(id) + '/test', { method: 'POST', timeout: TEST_TIMEOUT_MS }));
            mergeHost(body.host);
            emit();
            return body.host;
        }
        async function loadNests() {
            const body = await guarded(() => request('/nests'));
            return Array.isArray(body.nests) ? body.nests : [];
        }
        async function importNest(nestId) {
            const body = await guarded(() => request('/hosts/import-nest/' + enc(nestId), { method: 'POST' }));
            mergeHost(body.host);
            if (body.host) void loadContainers(body.host.id);
            emit();
            return body.host;
        }

        function dispose() {
            if (state.disposed) return;
            state.disposed = true;
            state.visible = false;
            clearTimeout(timer);
            timer = 0;
            lifetime.abort();
            detach();
            listeners.clear();
        }

        return {
            state, subscribe, attach, setVisible, setScope, refresh, loadHosts, loadContainers, reloadContainers, pollOnce, pending,
            act, remove, openDetail, closeDetail, setDetailTab, loadLogs, loadInspect, canMutate, canEditHosts,
            testDraft, saveHost, deleteHost, testHost, loadNests, importNest, dispose
        };
    }

    window.DockerManagerStore = { create, LOG_TAILS, POLL_MS };
})();
