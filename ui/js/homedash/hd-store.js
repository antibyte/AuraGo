// HomeDash store (F1; design-frontend 3.3 under the M2 contract). One per JS realm (shared(), keyed by the API base path). It turns stream
// frames into entity views for the widget host, batches change notices into one animation frame, marks values stale, keeps the ids of a
// left page subscribed for 30 s, and caches history series and the widget catalog. Rules:
// 1 A snapshot is the truth for the ids it lists (the contract's snapshot covers newly added ids only).
// 2 The server's per-entity stale flag is the truth for value age; a source status of degraded does not mark anything stale by itself.
//   Connection loss (connecting/down for 5 s, or stopped) marks every entry stale (cleared per id by its next snapshot or state). A source
//   status of unavailable takes that source's entities offline (available false, status unavailable) while it holds; the last value is kept
//   and returns when the source leaves unavailable. Only available entities are stale.
// 3 readonly means a control that could act is blocked: only for an entity with a command capability (not stream/snapshot/history/numeric)
//   that the server marks read-only or that sits on a read-only desktop; a pure display (all system sensors) is never drawn locked.
//   serverReadonly keeps the raw server value.
// 4 removed and PUT-rejected ids are status unknown.
// 5 History {origin, points:[{t, v}]} becomes the widget host's series {points: [[t_ms, value|null]], source: origin}; cache key id|span|points,
//   TTL 60 s under 6 h spans else 5 min, LRU 100, 4 requests at a time; live numeric states append to every cached series of the id.
// 6 Ids a window stops showing stay subscribed for 30 s (linger).
// 7 dispose() is idempotent: it unhooks the stream, detaches every watcher's stream handle and drops its linger; shared() replaces a disposed instance.
// View: {id, source, kind, name, area, unit, device_class, icon_hint, value, text, attributes (+updated_at), available, status, readonly,
// serverReadonly, capabilities, pending: false, stale, error: null, tone: null}.
(function () {
    'use strict';
    const E = window.HomeDashEngine;
    if (!E || !E.core || !E.api) throw new Error('hd-store.js: load hd-core.js and hd-api.js first');
    const GRACE_MS = 5000, LINGER_MS = 30000, SERIES_MAX = 100, SERIES_POINTS = 1000, PARALLEL = 4, SHORT_TTL = 60000, LONG_TTL = 300000, SIX_HOURS = 21600000;
    const READ_CAPS = Object.freeze(['stream', 'snapshot', 'history', 'numeric']);
    const isObj = v => v !== null && typeof v === 'object' && !Array.isArray(v);
    const strings = v => (v == null ? [] : Array.from(v)).map(String);
    function rangeMs(range) {
        if (typeof range === 'number' && Number.isFinite(range)) return range;
        const m = /^(\d+)(m|h|d)$/.exec(String(range || '24h'));
        return m ? Number(m[1]) * { m: 60000, h: 3600000, d: 86400000 }[m[2]] : 86400000;
    }
    function convert(answer) {
        const out = [];
        for (const p of answer && Array.isArray(answer.points) ? answer.points : []) {
            if (!isObj(p)) continue;
            const t = Date.parse(p.t), v = p.v === null ? null : Number(p.v);
            if (Number.isFinite(t) && (v === null || Number.isFinite(v))) out.push([t, v]);
        }
        return out.sort((a, b) => a[0] - b[0]);
    }
    function create(options) {
        const o = options || {}, api = o.api, stream = o.stream;
        if (!api || !stream) throw new Error('hd-store: api and stream are required');
        const setT = o.setTimeout || ((fn, ms) => setTimeout(fn, ms)), clearT = o.clearTimeout || (id => clearTimeout(id)), now = o.now || (() => Date.now());
        const bus = E.core.emitter(), entries = new Map(), sources = new Map(), owners = new Map(), series = new Map(), queue = [], unhook = [];
        let disposed = false, changed = new Set(), all = false, desktopReadonly = false, globalRev = 0, conn = stream.status() || {}, offline = false, graceTimer = 0, lingerTimer = 0, running = 0, catalog = null, catalogLoad = null;
        const flush = E.core.frame(() => { const ids = all ? null : changed; changed = new Set(); all = false; bus.emit('change', ids); }, { raf: o.raf, caf: o.caf });
        const touch = id => { if (!all) changed.add(id); flush(); };
        const touchAll = () => { all = true; changed = new Set(); globalRev += 1; flush(); };
        const watched = id => { for (const rec of owners.values()) if (rec.ids.has(id) || rec.linger.has(id)) return true; return false; };
        const sourceOf = e => (e.meta && typeof e.meta.source === 'string' ? e.meta.source : String(e.id).split(':')[0]);
        const sourceOffline = src => { const s = sources.get(src); return !!(s && s.state === 'unavailable'); };
        function entry(id) {
            let e = entries.get(id);
            if (!e && watched(id)) { e = { id, meta: null, state: null, status: 'ok', reason: '', connStale: false, rev: 0, cache: null }; entries.set(id, e); }
            return e || null;
        }
        const bump = e => { e.rev += 1; touch(e.id); };
        function setEntity(ent) {
            if (!isObj(ent) || typeof ent.id !== 'string') return;
            const e = entry(ent.id); if (!e) return;
            e.meta = ent; if (isObj(ent.state)) e.state = ent.state; e.status = 'ok'; e.reason = ''; e.connStale = false; bump(e);
        }
        function setSources(list) {
            const flipped = [];
            for (const s of Array.isArray(list) ? list : []) {
                if (!isObj(s) || typeof s.source !== 'string') continue;
                const before = sourceOffline(s.source); sources.set(s.source, s); if (before !== sourceOffline(s.source)) flipped.push(s.source);
            }
            for (const e of entries.values()) if (flipped.includes(sourceOf(e))) bump(e);
            if (Array.isArray(list) && list.length) bus.emit('sources', [...sources.values()]);
        }
        function appendLive(id, st) {
            const t = Date.parse(st.updated_at), v = typeof st.value === 'number' && Number.isFinite(st.value) ? st.value : null;
            if (v === null || st.available === false || !Number.isFinite(t)) return;
            for (const s of series.values()) {
                if (s.id !== id || !s.result) continue;
                const pts = s.result.points, last = pts[pts.length - 1];
                if (last && t <= last[0]) continue;
                pts.push([t, v]);
                let cut = 0; while (cut < pts.length - 1 && pts[cut][0] < t - s.span) cut += 1;
                if (pts.length - cut > SERIES_POINTS) cut = pts.length - SERIES_POINTS;
                if (cut > 0) pts.splice(0, cut);
            }
        }
        const connection = () => Object.freeze({ phase: conn.phase || 'idle', reason: conn.reason || '', offline, nextRetryAt: conn.nextRetryAt || 0, attempt: conn.attempt || 0 });
        function goOffline() {
            if (offline) return;
            offline = true; for (const e of entries.values()) { e.connStale = true; e.rev += 1; }
            touchAll(); bus.emit('connection', connection());
        }
        function onConnection(st) {
            conn = st || {};
            if (conn.phase === 'stopped') { clearT(graceTimer); graceTimer = 0; goOffline(); }
            else if (conn.phase === 'connecting' || conn.phase === 'down') { if (!offline && !graceTimer) graceTimer = setT(() => { graceTimer = 0; goOffline(); }, GRACE_MS); }
            else { clearT(graceTimer); graceTimer = 0; offline = false; }
            bus.emit('connection', connection());
        }
        const listen = (type, fn) => { stream.on(type, fn); unhook.push(() => stream.off(type, fn)); };
        listen('snapshot', d => { for (const ent of Array.isArray(d.entities) ? d.entities : []) setEntity(ent); setSources(d.sources); });
        listen('entity', d => { for (const ent of Array.isArray(d.entities) ? d.entities : []) setEntity(ent); });
        listen('removed', d => { for (const id of strings(d.ids)) { const e = entry(id); if (e) { e.status = 'unknown'; e.reason = 'removed'; bump(e); } } });
        listen('state', d => {
            if (!isObj(d.changes)) return;
            for (const [id, st] of Object.entries(d.changes)) { if (!isObj(st)) continue; const e = entry(id); if (!e) continue; e.state = st; e.connStale = false; appendLive(id, st); bump(e); }
        });
        listen('source', d => setSources([d]));
        listen('subscribed', d => { for (const r of Array.isArray(d.rejected) ? d.rejected : []) { if (!isObj(r)) continue; const e = entry(String(r.id)); if (e) { e.status = 'unknown'; e.reason = String(r.reason || 'rejected'); bump(e); } } });
        listen('dashboard_changed', d => bus.emit('dashboards', d));
        listen('connection', onConnection);

        const hasCommand = caps => Array.isArray(caps) && caps.some(c => isObj(c) && typeof c.name === 'string' && !READ_CAPS.includes(c.name));
        function view(id) {
            const e = entries.get(id);
            if (!e || (!e.meta && !e.state && e.status !== 'unknown')) return null;
            const key = e.rev + ':' + globalRev;
            if (e.cache && e.cache.key === key) return e.cache.view;
            const m = e.meta || {}, s = e.state || {}, src = sourceOf(e), caps = Array.isArray(m.capabilities) ? m.capabilities : [];
            const unknown = e.status === 'unknown', gone = sourceOffline(src);
            const available = !unknown && !gone && !!e.state && s.available !== false;
            const v = Object.freeze({
                id, source: src, kind: typeof m.kind === 'string' ? m.kind : 'generic', name: typeof m.name === 'string' && m.name ? m.name : String(id).slice(src.length + 1),
                area: typeof m.area === 'string' ? m.area : '', unit: typeof m.unit === 'string' ? m.unit : '', device_class: typeof m.device_class === 'string' ? m.device_class : '',
                icon_hint: typeof m.icon_hint === 'string' ? m.icon_hint : '', value: unknown || s.value === undefined ? null : s.value, text: typeof s.text === 'string' ? s.text : '',
                attributes: Object.freeze(Object.assign({}, isObj(s.attributes) ? s.attributes : {}, { updated_at: typeof s.updated_at === 'string' ? s.updated_at : null })),
                available, status: unknown ? 'unknown' : available ? 'ok' : 'unavailable', readonly: hasCommand(caps) && (m.readonly === true || desktopReadonly), serverReadonly: m.readonly === true,
                capabilities: caps, pending: false, stale: available && (s.stale === true || e.connStale), error: null, tone: null });
            e.cache = { key, view: v };
            return v;
        }
        function prune() { for (const id of [...entries.keys()]) if (!watched(id)) entries.delete(id); }
        function armLinger() {
            clearT(lingerTimer); lingerTimer = 0;
            let next = Infinity; for (const rec of owners.values()) for (const at of rec.linger.values()) next = Math.min(next, at);
            if (next === Infinity) return;
            lingerTimer = setT(() => {
                lingerTimer = 0; const t = now();
                for (const rec of owners.values()) { let cut = false; for (const [id, at] of [...rec.linger]) if (at <= t) { rec.linger.delete(id); cut = true; } if (cut) rec.push(); }
                prune(); armLinger();
            }, Math.max(0, next - now()));
        }
        function watch(ownerId, init) {
            const i = init || {}, rec = { ids: new Set(strings(i.ids)), linger: new Map(), push: null, handle: null };
            const handle = stream.attach(ownerId, { entities: [], active: i.active !== false });
            rec.push = () => handle.set([...rec.ids, ...rec.linger.keys()]);
            rec.handle = handle; owners.set(handle.id, rec); rec.push();
            return Object.freeze({
                set(list) {
                    const next = new Set(strings(list)), until = now() + LINGER_MS;
                    for (const id of rec.ids) if (!next.has(id)) rec.linger.set(id, until);
                    for (const id of next) rec.linger.delete(id);
                    rec.ids = next; rec.push(); armLinger(); prune();
                },
                setActive(active) { handle.setActive(!!active); },
                release() { if (!owners.delete(handle.id)) return; handle.detach(); prune(); armLinger(); }
            });
        }
        function limit(task) { return new Promise((resolve, reject) => { queue.push({ task, resolve, reject }); pump(); }); }
        function pump() {
            while (running < PARALLEL && queue.length) {
                const job = queue.shift(); running += 1;
                Promise.resolve().then(job.task).then(job.resolve, job.reject).then(() => { running -= 1; pump(); });
            }
        }
        function history(id, range, points) {
            const span = E.core.clamp(rangeMs(range), 60000, 7 * 86400000), n = E.core.clamp(Math.round(Number(points) || 120), 1, 500), key = id + '|' + span + '|' + n;
            const hit = series.get(key);
            if (hit && now() - hit.at < (span < SIX_HOURS ? SHORT_TTL : LONG_TTL)) { series.delete(key); series.set(key, hit); return hit.promise; }
            const s = { id, span, at: now(), result: null, promise: null };
            s.promise = limit(() => api.history(id, { from: now() - span, to: now(), points: n })).then(answer => {
                s.result = { points: convert(answer), source: answer && typeof answer.origin === 'string' ? answer.origin : 'none' };
                return s.result;
            }, err => { if (series.get(key) === s) series.delete(key); throw E.api.normalizeError(err); });
            series.set(key, s);
            while (series.size > SERIES_MAX) series.delete(series.keys().next().value);
            return s.promise;
        }
        function widgetCatalog(hash) {
            if (catalog && (!hash || catalog.hash === hash)) return Promise.resolve(catalog);
            if (catalogLoad) return catalogLoad;
            catalogLoad = api.widgetCatalog(catalog ? catalog.hash : '').then(answer => {
                if (answer && answer.not_modified && catalog) return catalog;
                if (!answer || !isObj(answer.catalog)) throw E.api.normalizeError({ status: 0, body: { code: 'bad_catalog', message: 'widget catalog missing' } });
                catalog = answer.catalog; return catalog;
            }).finally(() => { catalogLoad = null; });
            return catalogLoad;
        }
        return Object.freeze({
            view, watch, history, widgetCatalog, connection, setSources, on: bus.on, off: bus.off, sources: () => [...sources.values()],
            setDesktopReadonly(flag) { if (desktopReadonly === !!flag) return; desktopReadonly = !!flag; touchAll(); },
            get disposed() { return disposed; },
            dispose() {
                if (disposed) return;
                disposed = true; for (const fn of unhook.splice(0)) fn(); clearT(graceTimer); clearT(lingerTimer); graceTimer = 0; lingerTimer = 0; flush.cancel();
                const recs = [...owners.values()]; entries.clear(); series.clear(); owners.clear();
                for (const rec of recs) { rec.linger.clear(); try { rec.handle.detach(); } catch (err) { /* the stream may already be gone */ } }
            }
        });
    }
    const instances = new Map();
    function shared(options) {
        const key = options && options.api ? options.api.base : '', hit = instances.get(key);
        if (hit && !hit.disposed) return hit;
        const fresh = create(options); instances.set(key, fresh); return fresh;
    }
    E.store = Object.freeze({ create, shared, convert, rangeMs, GRACE_MS, LINGER_MS });
})();
