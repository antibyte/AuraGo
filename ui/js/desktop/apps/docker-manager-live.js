/* Docker Manager live stream: the client of GET /api/docker-manager/stream. One EventSource per app window. The first message
   (hello) names the stream; what the window wants to hear about (hosts for events and status, containers for 3 s stats, hosts
   for the rail gauges) goes to PUT /stream/subscriptions, debounced by 250 ms and sent again after every reconnect, because a
   new stream starts with an empty subscription. A broken stream is closed by this module (the browser's own retry would repeat
   the request without backoff) and opened again after 1, 2, 4, 8, 16 and 30 s. It keeps the last status of every host, a ring of
   60 StatsPoints per container and the latest host stats, and hands every message to the callbacks. A stream that stays silent for
   three heartbeats (at least 45 s; EventSource shows no comment lines, so a stream that only carries heartbeats looks the same) is
   not given up at once: the client sends its wish again, and only a server that no longer knows the stream, an error or no answer
   within 10 s counts as a dead connection. A classic script
   (window.DockerManagerLive) with no DOM work at load; the EventSource, the api function, the clock and the timers are injected so
   that scripts/test-docker-manager-ui.mjs can drive it with fakes. */
(function () {
    'use strict';

    const BASE = '/api/docker-manager';
    const DEBOUNCE_MS = 250;
    const BACKOFF_MS = [1000, 2000, 4000, 8000, 16000, 30000];
    const RETRY_PUT_MS = 3000;
    const MAX_PUT_RETRIES = 3;
    const RING = 60;
    const DEFAULT_LIMITS = { stats_ids: 50, hosts: 20 };
    const WATCH_MIN_MS = 45000;
    const WATCH_FACTOR = 3;
    const PROBE_TIMEOUT_MS = 10000;

    const array = value => (Array.isArray(value) ? value : []);
    const text = value => (value == null ? '' : String(value));
    const unique = list => [...new Set(list)];
    const parse = data => { try { const value = JSON.parse(data); return value && typeof value === 'object' ? value : null; } catch (error) { return null; } };

    function create(options) {
        const o = options || {};
        const model = window.DockerManagerModel;
        const base = o.base || BASE;
        const api = o.api;
        const openSource = o.eventSource;
        const setT = typeof o.setTimeout === 'function' ? o.setTimeout : (fn, ms) => setTimeout(fn, ms);
        const clearT = typeof o.clearTimeout === 'function' ? o.clearTimeout : id => clearTimeout(id);
        const now = typeof o.now === 'function' ? o.now : () => Date.now();
        const call = (name, ...args) => {
            if (typeof o[name] !== 'function') return;
            try { o[name](...args); } catch (error) { console.error('Docker Manager live:', error); }
        };

        let source = null, streamId = '', connected = false, failed = false, stopped = true, failures = 0, generation = 0, putRetries = 0;
        let retryTimer = 0, putTimer = 0, watchTimer = 0, watchMs = WATCH_MIN_MS, sending = false, dirty = false, limits = Object.assign({}, DEFAULT_LIMITS);
        let want = { hosts: [], stats: {}, host_stats: [] }, wantSig = '', sentSig = '', rejected = [], accepted = null;
        const statuses = new Map(), rings = new Map(), hostStats = new Map();

        const ringKey = (host, id) => host + '/' + id;
        const wantsId = (host, id) => array(want.stats[host]).includes(id);

        // ---- what the window wants -------------------------------------------------------------------------------------

        // input = {hosts: [id], stats: {host: [container id]}, hostStats: [id]}. Container ids are cut to 12 characters (the
        // server's short id), lists keep their order up to the limits of the stream, and the signature sorts them so that the
        // same wish in another order sends nothing.
        function normalize(input) {
            const i = input || {};
            const hosts = unique(array(i.hosts).map(text).filter(Boolean)).slice(0, limits.hosts);
            const stats = {};
            for (const host of Object.keys(i.stats || {})) {
                const ids = unique(array(i.stats[host]).map(id => text(id).slice(0, 12)).filter(Boolean)).slice(0, limits.stats_ids);
                if (ids.length) stats[host] = ids;
            }
            return { hosts, stats, host_stats: unique(array(i.hostStats || i.host_stats).map(text).filter(Boolean)).slice(0, limits.hosts) };
        }
        const signature = value => JSON.stringify({
            h: value.hosts.slice().sort(), s: Object.keys(value.stats).sort().map(host => [host, value.stats[host].slice().sort()]), g: value.host_stats.slice().sort()
        });
        const isEmpty = value => !value.hosts.length && !Object.keys(value.stats).length && !value.host_stats.length;

        function setWant(input) {
            const next = normalize(input), sig = signature(next);
            if (sig === wantSig) return;
            want = next;
            wantSig = sig;
            putRetries = 0;
            for (const key of [...rings.keys()]) {
                const at = key.lastIndexOf('/');
                if (!wantsId(key.slice(0, at), key.slice(at + 1))) rings.delete(key);
            }
            if (connected) schedulePut(DEBOUNCE_MS);
        }

        function schedulePut(delay) {
            clearT(putTimer);
            putTimer = setT(() => { putTimer = 0; void put(); }, delay);
        }

        async function put() {
            if (stopped || !connected || !streamId || wantSig === sentSig) return;
            if (sending) { dirty = true; return; }
            if (isEmpty(want) && sentSig === '') { sentSig = wantSig; return; }
            sending = true;
            dirty = false;
            const mine = generation, sig = wantSig, body = Object.assign({ stream_id: streamId }, want);
            try {
                const answer = await api(base + '/stream/subscriptions', { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) });
                if (mine !== generation) return;
                sentSig = sig;
                putRetries = 0;
                rejected = array(answer && answer.rejected);
                accepted = (answer && answer.accepted) || null;
                call('onSubscribed', { accepted, rejected });
            } catch (error) {
                if (mine !== generation) return;
                const info = model.normalizeError(error);
                if (info.code === 'stream_not_found') failure();
                else if (info.code !== 'aborted' && putRetries < MAX_PUT_RETRIES) { putRetries += 1; dirty = true; }
            } finally {
                if (mine === generation) {
                    sending = false;
                    if (dirty && !stopped && connected) schedulePut(putRetries ? RETRY_PUT_MS : 0);
                }
            }
        }

        // ---- connection ------------------------------------------------------------------------------------------------

        // Every message (not the comment heartbeats, which EventSource hides) starts the window again. When it runs out the client
        // asks the server whether it still has the stream, by sending the wish again: an answer keeps the stream, a 404, any other
        // error or no answer within 10 s means the connection is dead.
        function armWatchdog() {
            clearT(watchTimer);
            watchTimer = 0;
            if (stopped || !connected) return;
            watchTimer = setT(() => { watchTimer = 0; void probe(); }, watchMs);
        }
        async function probe() {
            if (stopped || !connected || !streamId) return;
            const mine = generation, controller = typeof AbortController === 'function' ? new AbortController() : null;
            const timeout = setT(() => { if (controller) controller.abort(); }, PROBE_TIMEOUT_MS);
            try {
                await api(base + '/stream/subscriptions', { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(Object.assign({ stream_id: streamId }, want)), signal: controller ? controller.signal : undefined });
                if (mine === generation) armWatchdog();
            } catch (error) {
                if (mine === generation) failure();
            } finally { clearT(timeout); }
        }

        function closeSource() {
            if (!source) return;
            const old = source;
            source = null;
            try { old.close(); } catch (error) { /* already closed */ }
        }

        // The stream broke or the server dropped us: forget the stream, tell the callbacks and try again after the next delay.
        function failure() {
            const was = connected;
            closeSource();
            generation += 1;
            sending = false;
            dirty = false;
            clearT(putTimer);
            putTimer = 0;
            clearT(watchTimer);
            watchTimer = 0;
            connected = false;
            failed = true;
            streamId = '';
            sentSig = '';
            statuses.clear();
            if (stopped) return;
            const delay = BACKOFF_MS[Math.min(failures, BACKOFF_MS.length - 1)];
            failures += 1;
            clearT(retryTimer);
            retryTimer = setT(() => { retryTimer = 0; connect(); }, delay);
            if (was || failures === 1) call('onConnection', false, { failed: true });
        }

        function connect() {
            if (stopped || source) return;
            if (typeof openSource !== 'function') return;
            let next;
            try { next = openSource(base + '/stream'); } catch (error) { failure(); return; }
            source = next;
            const on = (type, handler) => next.addEventListener(type, event => {
                if (source !== next || stopped) return;
                const data = parse(event && event.data);
                if (data) { armWatchdog(); handler(data); }
            });
            on('hello', data => {
                streamId = text(data.stream_id);
                limits = Object.assign({}, DEFAULT_LIMITS, data.limits || {});
                const resumed = failed;
                watchMs = Math.max(WATCH_MIN_MS, WATCH_FACTOR * (Number(data.heartbeat_ms) || 0));
                connected = true;
                failed = false;
                failures = 0;
                sentSig = '';
                rejected = [];
                accepted = null;
                armWatchdog();
                call('onConnection', true, { resumed });
                // The limits may be lower than assumed: sign the wish again against them.
                const again = normalize(want);
                want = again;
                wantSig = signature(again);
                if (!isEmpty(want)) void put();
            });
            on('host_status', data => {
                const host = text(data.host);
                if (!host) return;
                const status = { state: text(data.state), reason: text(data.reason), t: Number(data.t) || now() };
                statuses.set(host, status);
                call('onStatus', host, status);
            });
            on('event', data => call('onEvent', data));
            on('resync', data => call('onResync', data));
            on('operation', data => { if (data.operation) call('onOperation', text(data.host), data.operation); });
            on('stats', data => {
                const host = text(data.host), id = text(data.id), point = data.point;
                if (!point || !wantsId(host, id)) return;
                const key = ringKey(host, id);
                const ring = rings.get(key) || [];
                const last = ring[ring.length - 1];
                if (last && Number(point.t) <= Number(last.t)) return;
                ring.push(point);
                if (ring.length > RING) ring.splice(0, ring.length - RING);
                rings.set(key, ring);
                call('onStats', host, id, ring);
            });
            on('stats_history', data => {
                const host = text(data.host), id = text(data.id);
                if (!wantsId(host, id)) return;
                const ring = array(data.points).filter(point => point && typeof point === 'object').slice(-RING);
                rings.set(ringKey(host, id), ring);
                call('onStats', host, id, ring);
            });
            on('stats_gone', data => {
                const host = text(data.host), id = text(data.id);
                rings.delete(ringKey(host, id));
                call('onStats', host, id, []);
            });
            on('host_stats', data => {
                const host = text(data.host);
                if (!host) return;
                hostStats.set(host, data);
                call('onHostStats', host, data);
            });
            // The server closes a client whose buffer filled; the stream ends, and the reconnect resubscribes.
            on('dropped', () => failure());
            next.onerror = () => { if (source === next) failure(); };
        }

        function start() {
            if (!stopped) return;
            stopped = false;
            failures = 0;
            connect();
        }
        function stop() {
            if (stopped) return;
            stopped = true;
            clearT(retryTimer);
            clearT(putTimer);
            clearT(watchTimer);
            retryTimer = putTimer = watchTimer = 0;
            closeSource();
            generation += 1;
            sending = false;
            dirty = false;
            connected = false;
            failed = false;
            streamId = '';
            sentSig = '';
            statuses.clear();
            hostStats.clear();
            rings.clear();
        }

        return {
            start, stop, setWant,
            connected: () => connected, failed: () => failed, streamId: () => streamId, limits: () => Object.assign({}, limits),
            status: host => statuses.get(host) || null, ring: (host, id) => rings.get(ringKey(host, id)) || [], hostStats: host => hostStats.get(host) || null,
            rejected: () => rejected.slice(), accepted: () => accepted, want: () => normalize(want)
        };
    }

    window.DockerManagerLive = { create, BACKOFF_MS, DEBOUNCE_MS, RING };
})();
