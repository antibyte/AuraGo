/* Docker Manager store: owns the requests to /api/docker-manager, the polling and the state the views render.
   A classic script (window.DockerManagerStore). It touches the DOM only in attach() (visibility observers), so
   scripts/test-docker-manager-ui.mjs can run it in a Node vm with a fake api. Rules it keeps:
   - every request goes through the desktop api() it is given; the Docker engine is never contacted from the browser;
   - one request in flight per key (the host list, one container list per host, logs, inspect): a second call joins it;
   - polling every 5 s only while the window is visible; hiding the window aborts the polls;
   - a failed container list keeps the last containers of that host and records the error, so one unreachable host
     never empties the table or blocks the other hosts.
   Stacks (phase 2) follow the same rules: one listing per host (state.stacks), the open stack (state.stack), the
   operations of this window and of the engine (state.ops, one EventSource per active operation, closed on "done",
   reconnected with a growing delay), and the calls for files, lint, convert, volumes and project actions. The
   EventSource comes from the `eventSource` option so that the Node test can fake the streams. */
(function () {
    'use strict';

    const BASE = '/api/docker-manager';
    const POLL_MS = 5000;
    // An unreachable host can take the server up to 30 s to give up, so the browser waits longer than that.
    const REQUEST_TIMEOUT_MS = 35000;
    const TEST_TIMEOUT_MS = 45000;
    // Listing, reading and linting a stack may start the lister container: the server allows 45 s.
    const STACK_TIMEOUT_MS = 50000;
    const LOG_TAILS = [100, 200, 500, 1000, 5000];
    const RECONNECT_MS = [1000, 2000, 4000, 8000, 15000];
    const KEEP_FINISHED = 12;
    const enc = encodeURIComponent;

    function create(options) {
        const opts = options || {};
        const model = window.DockerManagerModel;
        const stackModel = window.DockerManagerStackModel;
        const api = opts.api;
        const requestTimeout = Number(opts.requestTimeoutMs) > 0 ? Number(opts.requestTimeoutMs) : REQUEST_TIMEOUT_MS;
        if (typeof api !== 'function') throw new TypeError('DockerManagerStore needs an api function');
        const reconnectMs = Array.isArray(opts.reconnectMs) && opts.reconnectMs.length ? opts.reconnectMs : RECONNECT_MS;
        const openStream = typeof opts.eventSource === 'function' ? opts.eventSource
            : (typeof window.EventSource === 'function' ? url => new window.EventSource(url) : null);
        const lifetime = new AbortController();
        const listeners = new Set();
        const doneListeners = new Set();
        const state = {
            hosts: [], hostsLoaded: false, hostsFailed: null, disabled: false,
            scope: typeof opts.scope === 'string' && opts.scope ? opts.scope : 'all', byHost: {}, detail: null, busy: new Set(),
            section: stackModel.SECTIONS.includes(opts.section) ? opts.section : 'stacks', stacks: {}, stack: null, ops: [],
            readonly: !!opts.readonly, dockerReadonly: false,
            visible: false, disposed: false, lastRefreshAt: 0
        };
        const inflight = new Map();
        const controllers = new Map();
        const sequence = new Map();
        const stackSequence = new Map();
        let timer = 0, win = null, root = null, inView = true, detach = () => {}, detailGen = 0, stackGen = 0;

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
        const pending = () => [...inflight.keys()].some(key => key === 'hosts' || key.startsWith('c:') || key.startsWith('s:'));

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
                    // The server reports docker.readonly with every host list (read live, so lifting it shows within a poll);
                    // a 403 read_only from a mutation still sets it at once (noteError).
                    if (typeof body.docker_readonly === 'boolean') state.dockerReadonly = body.docker_readonly;
                    const ids = new Set(state.hosts.map(host => host.id));
                    for (const id of Object.keys(state.byHost)) if (!ids.has(id)) delete state.byHost[id];
                    for (const id of Object.keys(state.stacks)) if (!ids.has(id)) delete state.stacks[id];
                    if (state.stack && !ids.has(state.stack.hostId)) closeStack();
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

        // `force` (the Refresh button) makes the server rescan the stacks folder instead of answering from its 30 s cache.
        async function refresh(o) {
            if (state.disposed) return;
            await loadHosts();
            if (state.disposed || state.disabled) return;
            const ids = scopeIds(), force = !!(o && o.force);
            const jobs = ids.map(loadContainers);
            if (state.section === 'stacks') for (const id of ids) jobs.push(loadStacks(id, force));
            jobs.push(loadActiveOperations());
            await Promise.all(jobs);
            state.lastRefreshAt = Date.now();
            emit();
        }

        function loadScope() {
            const jobs = scopeIds().map(loadContainers);
            if (state.section === 'stacks') for (const id of scopeIds()) jobs.push(loadStacks(id));
            return Promise.all(jobs);
        }
        function setScope(scope) {
            const next = scope !== 'all' && state.hosts.some(host => host.id === scope) ? scope : 'all';
            if (next === state.scope) return;
            state.scope = next;
            emit();
            if (state.visible) void loadScope();
        }
        // The Stacks section polls the stack listings too; the Containers section leaves the lister container alone.
        function setSection(section) {
            if (!stackModel.SECTIONS.includes(section) || section === state.section) return;
            state.section = section;
            emit();
            if (state.visible) void loadScope();
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
            if (state.section === 'stacks') {
                for (const id of scopeIds()) jobs.push(loadStacks(id));
                if (state.stack) jobs.push(loadStack(true));
            }
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
            for (const [key, controller] of [...controllers]) if (key === 'hosts' || key.startsWith('c:') || key.startsWith('s:') || key === 'stack') controller.abort();
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
                // Only the readonly flag counts: "enabled" belongs to the Virtual Desktop itself, not to this app's policy.
                const readonly = detail.readonly === true || detail.read_only === true ? true
                    : detail.readonly === false || detail.read_only === false ? false : null;
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
            if (body.host) {
                delete state.byHost[body.host.id];
                delete state.stacks[body.host.id];
                void loadContainers(body.host.id);
                if (state.section === 'stacks') void loadStacks(body.host.id, true);
            }
            emit();
            return body.host;
        }
        async function deleteHost(id) {
            await guarded(() => request('/hosts/' + enc(id), { method: 'DELETE' }));
            state.hosts = state.hosts.filter(host => host.id !== id);
            delete state.byHost[id];
            delete state.stacks[id];
            if (state.scope === id) state.scope = 'all';
            if (state.detail && state.detail.hostId === id) closeDetail();
            if (state.stack && state.stack.hostId === id) closeStack();
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
            if (body.host) {
                void loadContainers(body.host.id);
                if (state.section === 'stacks') void loadStacks(body.host.id);
            }
            emit();
            return body.host;
        }

        // ---- stacks: listing per host, the open stack, files ------------------------------------------------------

        const list = value => (Array.isArray(value) ? value : []);
        const findEntry = id => state.ops.find(entry => entry.id === id) || null;

        // The listing of one host. Failures stay with the host (state.stacks[id].error): 409 host_not_prepared and
        // api_too_old are states the list explains, not errors of the app. `refresh` makes the server rescan its folder.
        function loadStacks(hostId, refresh) {
            const key = 's:' + hostId;
            if (refresh) { controllers.get(key)?.abort(); inflight.delete(key); }
            return once(key, async () => {
                const seq = (stackSequence.get(hostId) || 0) + 1;
                stackSequence.set(hostId, seq);
                try {
                    const body = await request('/hosts/' + enc(hostId) + '/stacks' + (refresh ? '?refresh=1' : ''), { timeout: STACK_TIMEOUT_MS }, key);
                    if (state.disposed || stackSequence.get(hostId) !== seq) return false;
                    const listing = Object.assign({}, body, { stacks: list(body.stacks), external: list(body.external), ignored: list(body.ignored) });
                    state.stacks[hostId] = { listing, loaded: true, error: null, at: Date.now() };
                    adoptOperations(listing);
                    return true;
                } catch (error) {
                    let info = noteError(error);
                    if (state.disposed || !info || stackSequence.get(hostId) !== seq) return false;
                    if (info.code === 'timeout') info = Object.assign({}, info, { code: 'host_unreachable', reason: 'timeout' });
                    const previous = state.stacks[hostId];
                    // An unreachable host keeps what it showed last (greyed out); a host that answers "not prepared" shows nothing stale.
                    const keep = info.code === 'host_unreachable' && previous;
                    state.stacks[hostId] = { listing: keep ? previous.listing : null, loaded: !!(keep && previous.loaded), error: info, at: keep ? previous.at : 0 };
                    return false;
                } finally { emit(); }
            });
        }

        function abortStack() { controllers.get('stack')?.abort(); stackGen += 1; }
        function openStack(hostId, name, tab) {
            if (state.disposed || !hostId || !name) return;
            const key = hostId + '/' + name, wanted = stackModel.STACK_TABS.includes(tab) ? tab : '';
            if (state.stack && state.stack.key === key) {
                if (wanted) state.stack.tab = wanted;
                emit();
                return;
            }
            abortStack();
            state.stack = { key, hostId, name, tab: wanted || 'overview', status: 'loading', detail: null, error: null };
            emit();
            void loadStack();
        }
        function closeStack() {
            if (!state.stack) return;
            abortStack();
            state.stack = null;
            emit();
        }
        function setStackTab(tab) {
            if (!state.stack || !stackModel.STACK_TABS.includes(tab) || state.stack.tab === tab) return;
            state.stack.tab = tab;
            emit();
        }
        // `quiet` (the poll) keeps the stack on screen when a reload fails; only a vanished stack is reported.
        async function loadStack(quiet) {
            const target = state.stack;
            if (!target) return false;
            controllers.get('stack')?.abort();
            const gen = ++stackGen;
            if (!quiet) { target.status = 'loading'; emit(); }
            try {
                const body = await request('/hosts/' + enc(target.hostId) + '/stacks/' + enc(target.name), { timeout: STACK_TIMEOUT_MS }, 'stack');
                if (state.disposed || state.stack !== target || gen !== stackGen) return false;
                target.detail = body.stack || null;
                target.status = target.detail ? 'ok' : 'error';
                target.error = target.detail ? null : { code: 'unknown', message: '' };
            } catch (error) {
                const info = noteError(error);
                if (state.disposed || !info || state.stack !== target || gen !== stackGen) return false;
                if (quiet && target.detail && info.code !== 'stack_not_found') return false;
                target.status = 'error';
                target.error = info;
            }
            emit();
            return true;
        }

        // files = {compose?: {content, revision}, env?: {content, revision}}. Rejects with .dm; a revision_conflict carries
        // data.file and data.current (the file as it is on the host now).
        async function saveFiles(hostId, name, files) {
            const body = await guarded(() => request('/hosts/' + enc(hostId) + '/stacks/' + enc(name) + '/files', { method: 'PUT', body: files }));
            const revisions = body.revisions || {};
            const detail = state.stack && state.stack.hostId === hostId && state.stack.name === name ? state.stack.detail : null;
            if (detail) {
                for (const file of ['compose', 'env']) {
                    if (files[file] && revisions[file] !== undefined) detail[file] = Object.assign({}, detail[file], { content: files[file].content, revision: revisions[file], exists: true });
                }
                if (files.env) detail.has_env = true;
            }
            void loadStacks(hostId);
            emit();
            return revisions;
        }

        // input = {name, compose, env?, deploy?}. Answers {stack, operation?, deploy_error?}; a deploying create follows its operation.
        // When the files were written but the deployment could not start, the answer is still a 201 with deploy_error {code, message}
        // and no operation: the create succeeded, the caller reports the error.
        async function createStack(hostId, input) {
            const payload = { name: input.name, compose: input.compose };
            if (typeof input.env === 'string' && input.env !== '') payload.env = input.env;
            if (input.deploy) payload.deploy = true;
            const body = await guarded(() => request('/hosts/' + enc(hostId) + '/stacks', { method: 'POST', body: payload, timeout: STACK_TIMEOUT_MS }));
            if (body.operation) trackOperation(body.operation);
            void loadStacks(hostId, true);
            return body;
        }

        async function loadHelpers(hostId) {
            const body = await guarded(() => request('/hosts/' + enc(hostId) + '/helpers', { timeout: STACK_TIMEOUT_MS }));
            return body.helpers || null;
        }
        async function loadVolumes(hostId, name) {
            const body = await guarded(() => request('/hosts/' + enc(hostId) + '/stacks/' + enc(name) + '/volumes', { timeout: STACK_TIMEOUT_MS }));
            return list(body.volumes);
        }
        // Lint never blocks anything. `slot` names the editor asking: a newer request of the same slot cancels the older one
        // (that one rejects with .dm.code === 'aborted').
        function lint(hostId, input, slot) {
            const key = 'lint:' + (slot || '');
            controllers.get(key)?.abort();
            return guarded(() => request('/hosts/' + enc(hostId) + '/lint', { method: 'POST', body: input, timeout: STACK_TIMEOUT_MS }, key));
        }
        function convertDockerRun(command) {
            return guarded(() => request('/convert/docker-run', { method: 'POST', body: { command } }));
        }
        // action: start | stop | restart on every container of an external compose project.
        async function projectAction(hostId, project, action) {
            await guarded(() => request('/hosts/' + enc(hostId) + '/projects/' + enc(project) + '/' + enc(action), { method: 'POST' }));
            void reloadContainers(hostId);
            void loadStacks(hostId, true);
        }

        // ---- operations -------------------------------------------------------------------------------------------

        function closeSource(entry) {
            if (!entry.source) return;
            try { entry.source.close(); } catch (error) { /* already closed */ }
            entry.source = null;
        }

        // Called once per entry when an operation seen running ends: refresh what it changed and tell the listeners.
        function finished(entry) {
            if (entry.final || state.disposed) return;
            entry.final = true;
            const op = entry.op, hostId = String(op.host_id || '');
            if (hostId) {
                void loadStacks(hostId, true);
                void reloadContainers(hostId);
                const open = state.stack && state.stack.hostId === hostId && state.stack.name === op.stack;
                if (open && op.kind === 'delete' && op.state === 'succeeded') closeStack();
                else if (open) void loadStack(true);
            }
            for (const listener of [...doneListeners]) {
                try { listener(entry); } catch (error) { console.error('Docker Manager:', error); }
            }
            emit();
        }

        // After a broken stream: read the operation itself (it may have ended meanwhile), then listen again. Without
        // EventSource this is the whole mechanism: the operation is read every 2 s.
        async function resync(entry) {
            if (state.disposed || !stackModel.isActiveOp(entry.op)) return;
            try {
                const body = await request('/operations/' + enc(entry.id));
                if (state.disposed) return;
                if (body.operation) stackModel.applySnapshot(entry, Object.assign({ output: entry.output }, body.operation));
            } catch (error) {
                const info = noteError(error);
                if (info && info.code === 'operation_not_found') entry.op = Object.assign({}, entry.op, { state: 'interrupted' });
            }
            if (!stackModel.isActiveOp(entry.op)) { entry.stream = 'closed'; finished(entry); } else follow(entry);
            emit();
        }
        function later(entry, delay) {
            clearTimeout(entry.timer);
            entry.timer = setTimeout(() => { entry.timer = 0; void resync(entry); }, delay);
        }

        // One EventSource per active operation, closed on "done". snapshot replaces what the entry knows (first connect and
        // every reconnect); output, progress and state add to it.
        function follow(entry) {
            if (state.disposed || entry.source || entry.timer || !stackModel.isActiveOp(entry.op)) return;
            if (!openStream) { entry.stream = 'retry'; later(entry, 2000); return; }
            const source = openStream(BASE + '/operations/' + enc(entry.id) + '/stream');
            entry.source = source;
            entry.stream = 'connecting';
            const on = (type, handler) => source.addEventListener(type, event => {
                if (entry.source !== source || state.disposed) return;
                const data = stackModel.parseJSON(event && event.data);
                if (!data) return;
                entry.failures = 0;
                entry.stream = 'live';
                handler(data);
                emit();
            });
            on('snapshot', op => {
                stackModel.applySnapshot(entry, op);
                if (!stackModel.isActiveOp(entry.op)) { closeSource(entry); entry.stream = 'closed'; finished(entry); }
            });
            for (const type of ['output', 'progress', 'state']) on(type, event => stackModel.applyOpEvent(entry, event));
            on('done', op => {
                closeSource(entry);
                entry.stream = 'closed';
                stackModel.applyOpEvent(entry, { type: 'done', operation: op });
                finished(entry);
            });
            source.onerror = () => {
                if (entry.source !== source) return;
                closeSource(entry);
                if (state.disposed || !stackModel.isActiveOp(entry.op)) return;
                entry.stream = 'retry';
                entry.failures = (entry.failures || 0) + 1;
                later(entry, reconnectMs[Math.min(entry.failures, reconnectMs.length) - 1]);
                emit();
            };
        }

        // Adds an operation to the drawer (or refreshes its entry) and follows it while it runs. An operation first seen
        // finished never reaches the done listeners. An entry that exists keeps its output and progress: the list answers
        // (GET /operations) and the start answers carry no output, so they only update the state, the times and the error. A
        // stream snapshot, "done" and the re-read of resync are what replace the output.
        function trackOperation(op) {
            if (!op || !op.id || state.disposed) return null;
            let entry = findEntry(op.id);
            if (entry) entry.op = Object.assign({}, entry.op, op, { output: entry.op.output });
            else {
                entry = stackModel.newOpEntry(op, Date.now());
                if (!stackModel.isActiveOp(entry.op)) entry.final = true;
                state.ops = stackModel.trimEntries([entry].concat(state.ops), KEEP_FINISHED);
            }
            if (stackModel.isActiveOp(entry.op)) follow(entry); else finished(entry);
            emit();
            return entry;
        }
        function attachOperation(id) {
            return once('op:' + id, async () => {
                if (findEntry(id)) return findEntry(id);
                try {
                    const body = await request('/operations/' + enc(id));
                    return trackOperation(body.operation);
                } catch (error) { noteError(error); return null; }
            });
        }
        // The stack listing names the operation running on a stack: pick up the ones this window did not start.
        function adoptOperations(listing) {
            for (const stack of listing.stacks) {
                const ref = stack && stack.operation;
                if (ref && ref.id && !findEntry(ref.id)) void attachOperation(ref.id);
            }
        }
        function loadActiveOperations() {
            return once('ops:active', async () => {
                try {
                    const body = await request('/operations?active=1');
                    for (const op of list(body.operations)) trackOperation(op);
                } catch (error) { noteError(error); } finally { emit(); }
            });
        }
        // filter = {host, stack, active, limit}. Plain reads for callers that want history; nothing is tracked.
        async function loadOperations(filter) {
            const f = filter || {}, params = [];
            if (f.host) params.push('host=' + enc(f.host));
            if (f.stack) params.push('stack=' + enc(f.stack));
            if (f.active) params.push('active=1');
            if (f.limit) params.push('limit=' + Number(f.limit));
            const body = await guarded(() => request('/operations' + (params.length ? '?' + params.join('&') : '')));
            return list(body.operations);
        }
        async function loadOperation(id) {
            const body = await guarded(() => request('/operations/' + enc(id)));
            return body.operation || null;
        }

        // Resolves {ok: true, entry} or {ok: false, error, busy?}. busy names the operation already running on that stack:
        // the caller attaches to its stream instead of reporting an error.
        async function operationResult(run) {
            if (state.disposed) return { ok: false, error: { code: 'aborted' } };
            try {
                const body = await run();
                const entry = trackOperation(body.operation);
                return { ok: !!entry, entry, operation: body.operation };
            } catch (error) {
                const info = noteError(error);
                if (!info) return { ok: false, error: { code: 'aborted' } };
                emit();
                if (info.code === 'operation_in_progress' && info.data && info.data.operation_id) {
                    void attachOperation(info.data.operation_id);
                    return { ok: false, error: info, busy: info.data.operation_id };
                }
                return { ok: false, error: info };
            }
        }
        // kind: config | up | down | stop | restart | pull | update. `volumes` adds --volumes to down.
        function startOp(hostId, name, kind, o) {
            const body = { kind };
            if (o && o.volumes) body.volumes = true;
            return operationResult(() => request('/hosts/' + enc(hostId) + '/stacks/' + enc(name) + '/ops', { method: 'POST', body }));
        }
        function prepareHost(hostId) {
            return operationResult(() => request('/hosts/' + enc(hostId) + '/prepare', { method: 'POST' }));
        }
        function deleteStack(hostId, name, volumes) {
            return operationResult(() => request('/hosts/' + enc(hostId) + '/stacks/' + enc(name) + (volumes ? '?volumes=1' : ''), { method: 'DELETE' }));
        }
        // Listeners get the entry of every operation that ended while this window watched it.
        function onOperationDone(listener) { doneListeners.add(listener); return () => doneListeners.delete(listener); }
        const activeOps = () => state.ops.filter(entry => stackModel.isActiveOp(entry.op));

        function dispose() {
            if (state.disposed) return;
            state.disposed = true;
            state.visible = false;
            clearTimeout(timer);
            timer = 0;
            for (const entry of state.ops) { closeSource(entry); clearTimeout(entry.timer); entry.timer = 0; }
            lifetime.abort();
            detach();
            listeners.clear();
            doneListeners.clear();
        }

        return {
            state, subscribe, attach, setVisible, setScope, setSection, refresh, loadHosts, loadContainers, reloadContainers, pollOnce, pending,
            act, remove, openDetail, closeDetail, setDetailTab, loadLogs, loadInspect, canMutate, canEditHosts,
            testDraft, saveHost, deleteHost, testHost, loadNests, importNest,
            loadStacks, openStack, closeStack, setStackTab, loadStack, saveFiles, createStack, loadHelpers, loadVolumes, lint, convertDockerRun, projectAction,
            startOp, prepareHost, deleteStack, loadOperations, loadOperation, loadActiveOperations, attachOperation, trackOperation, onOperationDone, activeOps, dispose
        };
    }

    window.DockerManagerStore = { create, LOG_TAILS, POLL_MS, STACK_TIMEOUT_MS };
})();
