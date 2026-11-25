// HomeDash stream client (F1; contract "Stream protocol"). One SSE connection per JS realm (shared(), keyed by the API base path), used by
// every HomeDash window. Owners attach with the entity ids they show and an active flag (the host's visibility signal). The union of the
// active owners' ids, cut to hello.limits.entities, goes out with PUT stream/subscriptions 250 ms after the last change and right after every
// hello (a new stream starts empty). Silence for max(45 s, 3 x heartbeat_ms) or an EventSource error closes the source (never the browser's
// own retry: the stream id changes) and reconnects after 1, 2, 4, 8, 16, 30 s with +-20 % jitter. A connection that fails before hello waits
// at least 5 s (429 too_many_streams cannot be told apart from other refusals) and asks diagnose(). The backoff resets after a stream lived
// 60 s. revoked shutdown reconnects; disabled and auth_lost stop until resume(). No active owner for 10 s closes the stream. Timers, clock,
// random and the api are injected so scripts/homedash-tests/stream.test.mjs drives everything.
(function () {
    'use strict';
    const E = window.HomeDashEngine;
    if (!E || !E.core || !E.api) throw new Error('hd-stream.js: load hd-core.js and hd-api.js first');
    const BACKOFF_MS = Object.freeze([1000, 2000, 4000, 8000, 16000, 30000]);
    const JITTER = 0.2, DEBOUNCE_MS = 250, WATCH_MIN_MS = 45000, WATCH_FACTOR = 3, RESET_AFTER_MS = 60000, OPEN_FAIL_MIN_MS = 5000;
    const IDLE_CLOSE_MS = 10000, PUT_RETRY_MS = 3000, PUT_RETRIES = 3, DEFAULT_LIMIT = 300;
    const PASS = Object.freeze(['snapshot', 'entity', 'removed', 'state', 'source', 'dashboard_changed']);
    const parse = data => { try { const v = JSON.parse(data); return v && typeof v === 'object' && !Array.isArray(v) ? v : null; } catch (err) { return null; } };
    const idList = list => [...new Set((list == null ? [] : Array.from(list)).map(String).filter(id => id.indexOf(':') > 0))];

    function create(options) {
        const o = options || {}, api = o.api;
        if (!api || typeof api.openStream !== 'function' || typeof api.putSubscriptions !== 'function') throw new Error('hd-stream: an api with openStream and putSubscriptions is required');
        const setT = o.setTimeout || ((fn, ms) => setTimeout(fn, ms)), clearT = o.clearTimeout || (id => clearTimeout(id));
        const now = o.now || (() => Date.now()), random = o.random || Math.random, bus = E.core.emitter(), owners = new Map();
        let source = null, streamId = '', phase = 'idle', reason = '', phaseAt = now(), generation = 0, failures = 0, helloSeen = false, helloAt = 0;
        let watchMs = WATCH_MIN_MS, limit = DEFAULT_LIMIT, retryTimer = 0, putTimer = 0, watchTimer = 0, idleTimer = 0, nextRetryAt = 0;
        let wantIds = [], wantSig = '', sentSig = '', sending = false, dirty = false, putRetries = 0, stopped = false, disposed = false;

        const status = () => Object.freeze({ phase, reason, streamId, attempt: failures, nextRetryAt, since: phaseAt });
        function setPhase(next, why) { const r = why || ''; if (phase === next && reason === r) return; phase = next; reason = r; phaseAt = now(); bus.emit('connection', status()); }
        const anyActive = () => { for (const ow of owners.values()) if (ow.active) return true; return false; };
        function union() {
            const seen = new Set(), out = [];
            for (const ow of owners.values()) if (ow.active) for (const id of ow.ids) if (!seen.has(id)) { seen.add(id); out.push(id); }
            return out.slice(0, limit);
        }
        const signature = list => list.slice().sort().join('\n');

        function refreshWant() {
            if (disposed) return;
            const next = union(), sig = signature(next);
            if (sig !== wantSig) { wantIds = next; wantSig = sig; putRetries = 0; if (phase === 'live') schedulePut(DEBOUNCE_MS); }
            if (stopped) return;
            if (anyActive()) { if (idleTimer) { clearT(idleTimer); idleTimer = 0; } if (phase === 'idle') connect(); }
            else if (!idleTimer && phase !== 'idle') idleTimer = setT(() => { idleTimer = 0; if (!anyActive()) { teardown(); failures = 0; setPhase('idle', 'inactive'); } }, IDLE_CLOSE_MS);
        }
        function schedulePut(delay) { clearT(putTimer); putTimer = setT(() => { putTimer = 0; void put(); }, delay); }
        async function put() {
            if (phase !== 'live' || !streamId || wantSig === sentSig) return;
            if (sending) { dirty = true; return; }
            sending = true; dirty = false;
            const mine = generation, sig = wantSig, list = wantIds.slice();
            try {
                const answer = await api.putSubscriptions(streamId, list);
                if (mine !== generation) return;
                sentSig = sig; putRetries = 0;
                bus.emit('subscribed', { accepted: Array.isArray(answer && answer.accepted) ? answer.accepted : [], rejected: Array.isArray(answer && answer.rejected) ? answer.rejected : [] });
            } catch (err) {
                if (mine !== generation) return;
                const e = E.api.normalizeError(err);
                if (e.code === 'stream_not_found') { failure('stream_not_found'); return; }
                if (e.code === 'homedash_disabled') { stop('disabled'); return; }
                if (e.status === 401) { stop('auth_lost'); return; }
                if (e.code !== 'aborted' && putRetries < PUT_RETRIES) { putRetries += 1; dirty = true; }
            } finally {
                if (mine === generation) { sending = false; if (dirty && phase === 'live') schedulePut(putRetries ? PUT_RETRY_MS : 0); }
            }
        }
        function armWatchdog() { clearT(watchTimer); watchTimer = setT(() => { watchTimer = 0; failure('watchdog'); }, watchMs); }
        function teardown() {
            generation += 1;
            if (source) { const old = source; source = null; try { old.close(); } catch (err) { /* already closed */ } }
            clearT(putTimer); clearT(watchTimer); clearT(retryTimer); putTimer = watchTimer = retryTimer = 0;
            streamId = ''; sentSig = ''; sending = false; dirty = false; helloSeen = false; nextRetryAt = 0;
        }
        function connect() {
            if (source || stopped || disposed) return;
            clearT(retryTimer); retryTimer = 0; nextRetryAt = 0;
            generation += 1; const mine = generation;
            let next;
            try { next = api.openStream(); } catch (err) { failure('open'); return; }
            source = next; streamId = ''; sentSig = ''; helloSeen = false;
            const live = () => source === next && mine === generation;
            const on = (type, fn) => next.addEventListener(type, event => { if (!live()) return; armWatchdog(); const data = parse(event && event.data); if (data) fn(data); });
            on('hello', hello);
            for (const type of PASS) on(type, data => bus.emit(type, data));
            on('hb', () => {});
            on('revoked', data => { const why = String(data.reason || ''); if (why === 'disabled' || why === 'auth_lost') stop(why); else failure('shutdown'); });
            next.addEventListener('error', () => { if (live()) failure(helloSeen ? 'error' : 'open'); });
            setPhase('connecting');
            armWatchdog();
        }
        function hello(data) {
            const id = typeof data.stream_id === 'string' ? data.stream_id : '';
            if (!id) { failure('protocol'); return; }
            const hb = Number(data.heartbeat_ms), lim = Number(data.limits && data.limits.entities);
            streamId = id; watchMs = Math.max(WATCH_MIN_MS, WATCH_FACTOR * (hb > 0 ? hb : 0)); limit = Number.isInteger(lim) && lim > 0 ? lim : DEFAULT_LIMIT;
            helloSeen = true; helloAt = now(); sentSig = ''; sending = false; dirty = false; putRetries = 0;
            armWatchdog();
            wantIds = union(); wantSig = signature(wantIds);
            setPhase('live');
            bus.emit('hello', { streamId, limits: { entities: limit } });
            if (wantIds.length) void put(); else sentSig = wantSig;
        }
        function failure(why) {
            const lived = helloSeen && now() - helloAt >= RESET_AFTER_MS, beforeHello = !helloSeen;
            teardown();
            if (stopped || disposed) return;
            if (!anyActive()) { failures = 0; setPhase('idle', why); return; }
            if (lived) failures = 0;
            let delay = Math.round(BACKOFF_MS[Math.min(failures, BACKOFF_MS.length - 1)] * (1 - JITTER + 2 * JITTER * random()));
            if (beforeHello) delay = Math.max(delay, OPEN_FAIL_MIN_MS);
            failures += 1; nextRetryAt = now() + delay;
            retryTimer = setT(() => { retryTimer = 0; nextRetryAt = 0; if (anyActive()) connect(); else setPhase('idle', 'inactive'); }, delay);
            setPhase('down', why);
            if (beforeHello && typeof o.diagnose === 'function') Promise.resolve().then(() => o.diagnose())
                .then(result => { if (!disposed && !stopped && (result === 'disabled' || result === 'auth_lost')) stop(result); }, () => {});
        }
        function stop(why) { teardown(); if (idleTimer) { clearT(idleTimer); idleTimer = 0; } stopped = true; failures = 0; setPhase('stopped', why); }
        function resume() { if (disposed || !stopped) return; stopped = false; failures = 0; setPhase('idle', 'resume'); refreshWant(); }
        function retryNow() { if (stopped) { resume(); return; } if (phase === 'down' && retryTimer) { clearT(retryTimer); retryTimer = 0; nextRetryAt = 0; connect(); } }
        function dispose() { if (disposed) return; teardown(); if (idleTimer) clearT(idleTimer); idleTimer = 0; disposed = true; owners.clear(); setPhase('disposed'); }
        function attach(ownerId, init) {
            const key = String(ownerId || E.core.uid('hd-owner')), i = init || {};
            owners.set(key, { ids: idList(i.entities), active: i.active !== false });
            refreshWant();
            return Object.freeze({
                id: key,
                set(list) { const ow = owners.get(key); if (!ow) return; ow.ids = idList(list); refreshWant(); },
                setActive(active) { const ow = owners.get(key); if (!ow || ow.active === !!active) return; ow.active = !!active; refreshWant(); },
                detach() { if (owners.delete(key)) refreshWant(); }
            });
        }
        return Object.freeze({ attach, status, resume, retryNow, dispose, on: bus.on, off: bus.off, wanted: () => wantIds.slice(), limit: () => limit });
    }
    const instances = new Map();
    function shared(options) {
        const key = options && options.api ? options.api.base : '', have = instances.get(key);
        if (have && have.status().phase !== 'disposed') return have;
        const made = create(options); instances.set(key, made); return made;
    }
    E.stream = Object.freeze({ create, shared, BACKOFF_MS, DEBOUNCE_MS, WATCH_MIN_MS, OPEN_FAIL_MIN_MS, RESET_AFTER_MS, IDLE_CLOSE_MS });
})();
