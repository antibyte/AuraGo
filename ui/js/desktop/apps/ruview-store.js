/* RuView store: owns the fetches and the polling, and holds the state the views render.
   A classic script (window.RuViewStore). It never touches the DOM except for the visibility observers.
   Rules it keeps:
   - one request in flight per endpoint, polling only while the app is visible (D6: the Status view never dials an
     external service, the Boards view reads the board list explicitly);
   - a status epoch: a poll result applies only when no newer answer has been applied since it started;
   - a failed poll keeps the last status and sets statusFailed, it never turns into "off" (H9). */
(function () {
    'use strict';

    const BASE = '/api/desktop/integrations/ruview/';
    const POLL_MS = 3000;
    // POST runtime/start and runtime/install have no timeout: an image pull can take minutes. Only dispose aborts them.
    const TIMEOUTS = {status: 10000, nodes: 10000, test: 20000, 'runtime/stop': 30000, config: 15000};

    function failure(status, body) {
        const b = body && typeof body === 'object' ? body : {};
        const code = typeof b.error_code === 'string' && b.error_code ? b.error_code : status === 403 ? 'forbidden' : 'unknown';
        const message = String(b.error || b.message || 'HTTP ' + status);
        return Object.assign(new Error(message), {ruview: true, code, detail: typeof b.detail === 'string' ? b.detail : '', status});
    }

    function create(options) {
        const opts = options || {};
        const model = window.RuViewModel;
        const base = opts.base || BASE;
        const fetcher = opts.fetch || ((url, init) => window.fetch(url, init));
        const lifetime = new AbortController();
        const listeners = new Set();
        const state = {
            status: null, statusLoaded: false, statusFailed: false, statusAt: 0,
            nodes: {list: [], scope: '', liveState: '', loaded: false, error: '', receivedAt: null},
            pending: new Set(), actionError: null, view: 'status', visible: false, disposed: false,
            desktopReadonly: !!opts.desktopReadonly, lastCheckAt: 0
        };
        const flights = {status: null, nodes: null};
        let timer = 0, win = null, root = null, inView = true, detach = () => {};
        let counter = 0, applied = 0;

        function emit() {
            for (const listener of [...listeners]) {
                try { listener(state); } catch (error) { console.error('RuView:', error); }
            }
        }
        function subscribe(listener) { listeners.add(listener); return () => listeners.delete(listener); }

        // ---- requests --------------------------------------------------------------------------------------------

        // Rejects with an Error that carries {ruview, code, detail, status}. code is the backend's error_code, else
        // forbidden (403), network, timeout or unknown; aborted requests carry code "aborted" and are ignored by callers.
        function request(route, init) {
            const o = init || {};
            const controller = new AbortController();
            let timedOut = false;
            const onEnd = () => controller.abort();
            if (lifetime.signal.aborted) controller.abort(); else lifetime.signal.addEventListener('abort', onEnd, {once: true});
            const timeout = o.timeout ? setTimeout(() => { timedOut = true; controller.abort(); }, o.timeout) : 0;
            if (o.flight) flights[o.flight] = controller;
            return (async () => {
                try {
                    const response = await fetcher(o.url || base + route, {
                        method: o.method || 'GET', signal: controller.signal,
                        headers: o.body === undefined ? undefined : {'Content-Type': 'application/json'}, body: o.body
                    });
                    const body = await response.json().catch(() => ({}));
                    if (!response.ok) throw failure(response.status, body);
                    return body;
                } catch (error) {
                    if (error && error.ruview) throw error;
                    const code = timedOut ? 'timeout' : controller.signal.aborted ? 'aborted' : 'network';
                    throw Object.assign(new Error(code), {ruview: true, code, detail: '', status: 0});
                } finally {
                    clearTimeout(timeout);
                    lifetime.signal.removeEventListener('abort', onEnd);
                    if (o.flight && flights[o.flight] === controller) flights[o.flight] = null;
                }
            })();
        }
        function abortFlights() { for (const key of Object.keys(flights)) flights[key]?.abort(); }

        // ---- status ----------------------------------------------------------------------------------------------

        function applyStatus(data) {
            state.status = data;
            state.statusLoaded = true;
            state.statusFailed = false;
            state.statusAt = Date.now();
            // The headline already says it when the status carries the same error code.
            const code = data && typeof data === 'object' ? model.errorCodeOf(data) : '';
            if (state.actionError && code && state.actionError.code === code) state.actionError = null;
            emit();
        }
        async function fetchStatus(force) {
            if (state.disposed) return;
            if (flights.status) { if (!force) return; flights.status.abort(); }
            const seq = ++counter;
            try {
                const data = await request('status', {timeout: TIMEOUTS.status, flight: 'status'});
                if (state.disposed || seq <= applied) return;
                applied = seq;
                applyStatus(data);
            } catch (error) {
                if (state.disposed || error.code === 'aborted' || seq <= applied) return;
                state.statusFailed = true;
                emit();
            }
        }

        // ---- boards ----------------------------------------------------------------------------------------------

        // Status shows the board count only for a managed esp32 service (D6); Boards reads it whenever enabled.
        function wantsNodes() {
            const s = state.status;
            if (!state.statusLoaded || !s || typeof s !== 'object' || s.enabled !== true) return false;
            if (state.view === 'boards') return true;
            return state.view === 'status' && s.source === 'esp32' && s.mode !== 'external';
        }
        async function fetchNodes() {
            if (state.disposed || flights.nodes) return;
            try {
                const data = await request('nodes', {timeout: TIMEOUTS.nodes, flight: 'nodes'});
                if (state.disposed) return;
                state.nodes = Object.assign(model.mergeNodes(state.nodes, data), {loaded: true, receivedAt: performance.now()});
            } catch (error) {
                if (state.disposed || error.code === 'aborted') return;
                // A failed refresh keeps the known boards and their original arrival time; only the verdict changes.
                state.nodes = Object.assign({}, state.nodes, {error: 'unavailable'});
            }
            emit();
        }

        function refresh() {
            const jobs = [fetchStatus(true)];
            if (wantsNodes()) jobs.push(fetchNodes());
            return Promise.all(jobs).then(() => {
                // The first status decides whether the board list is wanted at all.
                if (wantsNodes() && !state.nodes.loaded && !flights.nodes) return fetchNodes();
                return undefined;
            });
        }

        // ---- polling and visibility ------------------------------------------------------------------------------

        function hiddenNow() {
            return state.disposed || document.hidden || (root && typeof root.checkVisibility === 'function' && !root.checkVisibility());
        }
        function poll() {
            timer = 0;
            if (state.disposed || !state.visible) return;
            if (!hiddenNow()) {
                void fetchStatus(false).then(() => { if (wantsNodes()) void fetchNodes(); });
            }
            schedule();
        }
        function schedule() {
            clearTimeout(timer);
            timer = 0;
            if (state.disposed || !state.visible) return;
            timer = setTimeout(poll, POLL_MS);
        }
        function visibility() {
            const next = !state.disposed && !document.hidden && inView
                && (!win || (!win.hidden && !win.classList.contains('vd-space-hidden') && win.style.display !== 'none' && win.getClientRects().length > 0));
            if (next === state.visible) return;
            state.visible = next;
            if (!next) { clearTimeout(timer); timer = 0; abortFlights(); }
            else { void refresh(); schedule(); }
            emit();
        }
        function attach(target) {
            const t = target || {};
            win = t.win || null;
            root = t.root || null;
            const onVisibility = () => visibility();
            document.addEventListener('visibilitychange', onVisibility);
            const windowObserver = typeof MutationObserver === 'function' && win ? new MutationObserver(onVisibility) : null;
            windowObserver?.observe(win, {attributes: true, attributeFilter: ['style', 'class', 'hidden', 'data-space-hidden']});
            const intersection = typeof IntersectionObserver === 'function' && root
                ? new IntersectionObserver(entries => { inView = !!entries[entries.length - 1]?.isIntersecting; visibility(); }, {threshold: 0.01}) : null;
            if (intersection) { inView = false; intersection.observe(root); }
            const onPolicy = event => {
                const detail = (event && event.detail) || {};
                const readonly = detail.readonly === true || detail.read_only === true || detail.enabled === false ? true
                    : detail.readonly === false || detail.read_only === false || detail.enabled === true ? false : null;
                if (readonly !== null && readonly !== state.desktopReadonly) { state.desktopReadonly = readonly; emit(); }
            };
            document.addEventListener('aurago:desktop-policy', onPolicy);
            detach = () => {
                document.removeEventListener('visibilitychange', onVisibility);
                document.removeEventListener('aurago:desktop-policy', onPolicy);
                windowObserver?.disconnect();
                intersection?.disconnect();
            };
            visibility();
        }
        function setView(view) {
            if (state.view === view) return;
            state.view = view;
            emit();
            if (state.visible) {
                void fetchStatus(false);
                if (wantsNodes()) void fetchNodes();
                schedule();
            }
        }

        // ---- actions ---------------------------------------------------------------------------------------------

        // An answer to an action is the newest truth: it applies even when an older poll is still in flight.
        function applyAction(data) {
            applied = ++counter;
            applyStatus(data);
        }
        function fail(action, error) {
            if (state.disposed || error.code === 'aborted') return false;
            // A cancelled start is not an error of its own: the status already says what happened.
            if (error.code !== 'cancelled') state.actionError = {action, code: error.code, message: error.message, detail: error.detail || ''};
            return false;
        }
        async function run(action) {
            if (state.disposed || state.pending.has(action)) return false;
            state.pending.add(action);
            state.actionError = null;
            emit();
            try {
                const data = await request(action, {method: 'POST', timeout: TIMEOUTS[action] || 0});
                if (state.disposed) return false;
                applyAction(data);
                if (action === 'test') {
                    state.lastCheckAt = Date.now();
                    // An explicit check of an external esp32 service is the one time Status reads its board list (D6).
                    const s = state.status;
                    if (s && s.enabled === true && s.source === 'esp32' && s.mode === 'external') await fetchNodes();
                }
                return true;
            } catch (error) {
                const handled = fail(action, error);
                if (!state.disposed && error.code !== 'aborted') void refresh();
                return handled;
            } finally {
                state.pending.delete(action);
                if (!state.disposed) emit();
            }
        }
        // D4: one field through the admin config save. Nothing else is sent.
        async function allowChanges() {
            const action = 'allow-changes';
            if (state.disposed || state.pending.has(action)) return false;
            state.pending.add(action);
            state.actionError = null;
            emit();
            try {
                await request('', {url: '/api/config', method: 'PUT', body: JSON.stringify({ruview: {readonly: false}}), timeout: TIMEOUTS.config});
                if (state.disposed) return false;
                await fetchStatus(true);
                return true;
            } catch (error) {
                return fail(action, error);
            } finally {
                state.pending.delete(action);
                if (!state.disposed) emit();
            }
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

        return {state, subscribe, attach, setView, refresh, run, allowChanges, dispose};
    }

    window.RuViewStore = {create};
})();
