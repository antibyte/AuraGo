// HomeDash command client (P1, decision D5; contract "API contract (frozen for M3)" section 4 of docs/superpowers/plans/2026-11-25-homedash-m3-backend.md).
// One client per JS realm (shared(), keyed by the API base path): every window and the more-info sheet see the same overlay. Rules 1-8 of the plan's
// "P1 interaction rules" are the normative text; summary: new request_id per action; overlay at once, pending after 250 ms; confirm by the contract
// match (tolerances, frames from sending, server state at the 202, 202 without expected confirms); superseded silent; errors roll back with a 6 s cue,
// 409 arms, 403 read_only/desktop_readonly mark read-only; 8 s timeout with late adoption; continuous: 1250 ms throttle with leading edge, final on
// commit, 429 intermediate dropped, 429 final retried once; keys: one trailing send after 400 ms; confirm-required arms 4 s. send() never rejects.
(function () {
    'use strict';
    const E = window.HomeDashEngine;
    if (!E || !E.core || !E.api || !E.format) throw new Error('hd-command.js: load hd-core.js, hd-format.js and hd-api.js first');
    const TIMING = Object.freeze({ PENDING_MS: 250, CONFIRM_MS: 8000, CUE_MS: 6000, THROTTLE_MS: 1250, KEY_MS: 400, ARM_MS: 4000, FLASH_MS: 600, RETRY_MS: 1000 });
    const FRAMES_KEPT = 8;
    const CONTINUOUS = Object.freeze(['brightness', 'color_temp', 'color', 'set_value', 'set_temperature']);
    const ARMABLE = Object.freeze(['toggle', 'turn_on', 'turn_off', 'activate', 'press', 'select_option', 'effect']);
    const isObj = v => v !== null && typeof v === 'object' && !Array.isArray(v), finite = v => typeof v === 'number' && Number.isFinite(v);
    function capOf(view, name) { for (const c of view && Array.isArray(view.capabilities) ? view.capabilities : []) { if (c === name) return { name, allowed: true }; if (isObj(c) && c.name === name) return c; } return null; }
    function predict(view, command, args) {
        const a = isObj(args) ? args : {}, on = E.format.isOn(view);
        switch (command) {
            case 'toggle': return on === null ? null : { value: !on };
            case 'turn_on': return { value: true };
            case 'turn_off': return { value: false };
            case 'brightness': return Number(a.value) > 0 ? { value: true, attributes: { brightness_pct: Number(a.value) } } : { value: false };
            case 'color_temp': return { value: true, attributes: { color_temp_kelvin: Number(a.kelvin), color_mode: 'color_temp' } };
            case 'color': return { value: true, attributes: { hs_color: [Number(a.h), Number(a.s)], color_mode: 'hs' } };
            case 'effect': return { value: true, attributes: { effect: String(a.option) } };
            case 'select_option': return { value: String(a.option) };
            case 'set_value': return { value: Number(a.value) };
            case 'set_temperature': return { attributes: { temperature: Number(a.value) } };
            case 'hvac_mode': return { value: String(a.option) };
            default: return null;
        }
    }
    function sameValue(actual, want, step) {
        if (typeof want === 'number') return typeof actual === 'number' && (step > 0 ? Math.abs(actual - want) <= step / 2 + 1e-9 : actual === want);
        return actual === want;
    }
    function attrMatches(key, actual, want) {
        if (key === 'brightness_pct') return finite(actual) && Math.abs(actual - want) <= 1;
        if (key === 'temperature') return finite(actual) && Math.abs(actual - want) <= 0.05;
        if (key === 'color_temp_kelvin') return finite(actual) && Math.abs(actual - want) <= Math.abs(want) * 0.02;
        if (key === 'hs_color') { if (!Array.isArray(actual) || !Array.isArray(want) || !finite(actual[0]) || !finite(actual[1])) return false;
            return Math.abs(((actual[0] - want[0]) % 360 + 540) % 360 - 180) <= 4 && Math.abs(actual[1] - want[1]) <= 4; }
        return isObj(want) || Array.isArray(want) ? JSON.stringify(actual) === JSON.stringify(want) : actual === want;
    }
    function matches(state, expected, step) {
        if (!isObj(state) || !isObj(expected) || state.available !== true) return false;
        if (Object.prototype.hasOwnProperty.call(expected, 'value') && !sameValue(state.value, expected.value, step)) return false;
        const want = isObj(expected.attributes) ? expected.attributes : {}, have = isObj(state.attributes) ? state.attributes : {};
        for (const k of Object.keys(want)) if (!attrMatches(k, have[k], want[k])) return false;
        return true;
    }
    const valueMatches = (state, expected, step) => isObj(state) && isObj(expected) && state.available === true && sameValue(state.value, expected.value, step);
    const stateOf = v => (v ? { value: v.value, attributes: v.attributes, available: v.available !== false && v.status !== 'unavailable' && v.status !== 'unknown' } : null);

    function create(options) {
        const o = options || {}, api = o.api, stream = o.stream, store = o.store;
        if (!api || typeof api.command !== 'function' || !stream || !store) throw new Error('hd-command: api (with command), stream and store are required');
        const later = o.setTimeout || ((fn, ms) => setTimeout(fn, ms)), clearT = o.clearTimeout || (id => clearTimeout(id));
        const now = o.now || (() => Date.now()), requestId = o.requestId || (() => E.core.randomId('r', 20));
        const bus = E.core.emitter(), overlays = new Map(), cues = new Map(), flashes = new Map(), armed = new Map(), lanes = new Map(), locks = new Map(), unhook = [];
        let seq = 0, disposed = false;
        const emit = id => { if (!disposed) bus.emit('change', [id]); };
        const dropCue = id => { const c = cues.get(id); if (c) { clearT(c.timer); cues.delete(id); } };
        function stop(rec) { clearT(rec.pendingTimer); clearT(rec.confirmTimer); clearT(rec.retryTimer); rec.pendingTimer = rec.confirmTimer = rec.retryTimer = 0; if (rec.retryDone) { const r = rec.retryDone; rec.retryDone = null; r(); } }
        function release(rec) { if (overlays.get(rec.id) !== rec) return false; stop(rec); overlays.delete(rec.id); return true; }
        function cue(id, entry) {
            dropCue(id); const c = { entry: Object.freeze(entry), timer: 0 };
            c.timer = later(() => { if (cues.get(id) === c) { cues.delete(id); emit(id); } }, TIMING.CUE_MS);
            cues.set(id, c); bus.emit('cue', { id, cue: c.entry });
        }
        function confirmRec(rec) {   // not named after the browser dialog: the engine JS guard rejects that call name
            if (!release(rec)) return;
            const old = flashes.get(rec.id); if (old) clearT(old);
            flashes.set(rec.id, later(() => { flashes.delete(rec.id); emit(rec.id); }, TIMING.FLASH_MS));
            emit(rec.id); bus.emit('confirmed', { id: rec.id, command: rec.command });
        }
        function settle(rec) { if (release(rec)) emit(rec.id); }
        function rollback(rec, e) {
            if (!release(rec)) return;
            const code = (e && e.code) || 'timeout', data = (e && e.data) || {};
            if (code === 'read_only' || code === 'desktop_readonly') {
                const reason = code === 'desktop_readonly' ? 'virtual_desktop.readonly' : (typeof data.blocked_by === 'string' && data.blocked_by) || 'homedash.readonly';
                locks.set(rec.id, reason); bus.emit('readonly', { id: rec.id, reason, desktop: code === 'desktop_readonly' });
            } else cue(rec.id, { code, source: typeof data.source === 'string' ? data.source : '', blockedBy: '' });
            emit(rec.id);
        }
        function track(s, patch, cap) {
            const id = s.entity, prev = overlays.get(id); if (prev) stop(prev);
            const rec = { id, seq: ++seq, command: s.command, patch, step: finite(cap && cap.step) ? cap.step : 0, expected: null, frames: [], pending: false, pendingTimer: 0, confirmTimer: 0, retryTimer: 0, sent: true };
            overlays.set(id, rec); dropCue(id);
            rec.pendingTimer = later(() => { if (overlays.get(id) === rec) { rec.pending = true; emit(id); } }, TIMING.PENDING_MS);
            rec.confirmTimer = later(() => expire(rec), TIMING.CONFIRM_MS);
            emit(id); return rec;
        }
        function local(s, patch) {
            const id = s.entity, prev = overlays.get(id); if (prev && prev.sent) stop(prev);
            const rec = prev && !prev.sent ? prev : { id, seq: ++seq, command: s.command, patch: null, step: 0, expected: null, frames: [], pending: false, pendingTimer: 0, confirmTimer: 0, retryTimer: 0, sent: false };
            rec.patch = patch; overlays.set(id, rec); dropCue(id); emit(id);
        }
        function expire(rec) {
            if (overlays.get(rec.id) !== rec) return;
            if (rec.expected && rec.frames.some(f => valueMatches(f, rec.expected, rec.step))) { settle(rec); return; }   // late adoption (lead answer Q3): any frame since sending
            rollback(rec, { code: 'timeout', data: {} });
        }
        function post(s, confirmed) {
            const body = { entity: s.entity, command: s.command, args: isObj(s.args) ? s.args : {}, request_id: requestId(), confirmed: !!confirmed };
            if (typeof s.dashboardId === 'string' && s.dashboardId) body.dashboard_id = s.dashboardId;
            if (typeof s.widgetId === 'string' && s.widgetId) body.widget_id = s.widgetId;
            return api.command(body);
        }
        const clientError = () => E.api.normalizeError({ hdApiError: false, status: 0, body: { code: 'client' } });
        function safePost(s, confirmed) {   // always a promise: a throwing or non-promise api.command, or a throwing requestId, is a 'client' failure
            try { const p = post(s, confirmed); return p && typeof p.then === 'function' ? p : Promise.reject(clientError()); } catch (err) { return Promise.reject(clientError()); }
        }
        function dispatch(s, cap, patch, confirmed, retried) {
            const rec = track(s, patch, cap);
            return safePost(s, confirmed).then(answer => accepted(rec, answer), err => failed(rec, E.api.normalizeError(err), s, cap, patch, confirmed, retried))
                .catch(() => ({ status: 'failed', code: 'client' }));
        }
        function accepted(rec, answer) {
            const a = isObj(answer) ? answer : {};
            if (disposed || overlays.get(rec.id) !== rec) return { status: 'stale' };
            if (a.superseded === true) { settle(rec); return { status: 'superseded' }; }
            if (!isObj(a.expected)) { confirmRec(rec); return { status: 'confirmed' }; }
            rec.expected = a.expected;
            if (rec.frames.some(f => matches(f, rec.expected, rec.step)) || matches(stateOf(store.view(rec.id)), rec.expected, rec.step)) confirmRec(rec);   // any frame since sending (rules 3 and 5)
            return { status: 'accepted' };
        }
        function failed(rec, e, s, cap, patch, confirmed, retried) {
            if (disposed) return { status: 'failed', code: e.code };
            const current = overlays.get(rec.id) === rec;
            if (e.code === 'confirmation_required') { if (current) settle(rec); arm(s); return { status: 'armed' }; }
            if (e.status === 429 && current && !retried && CONTINUOUS.includes(s.command)) {
                const wait = finite(e.retryAfter) && e.retryAfter > 0 ? Math.round(e.retryAfter * 1000) : TIMING.RETRY_MS;
                return new Promise(resolve => { rec.retryDone = () => resolve({ status: 'stale' }); rec.retryTimer = later(() => { rec.retryTimer = 0; rec.retryDone = null; resolve(disposed || overlays.get(rec.id) !== rec ? { status: 'stale' } : dispatch(s, cap, patch, confirmed, true)); }, wait); });
            }
            if (current) rollback(rec, e);
            return { status: 'failed', code: e.code };
        }
        function arm(s) {
            const key = s.widgetKey || s.entity + '|-'; disarm(key);
            const a = { id: s.entity, command: s.command, until: now() + TIMING.ARM_MS, timers: [] };
            for (let k = 1; k < 4; k++) a.timers.push(later(() => emit(a.id), k * 1000));
            a.timers.push(later(() => { if (armed.get(key) === a) { armed.delete(key); emit(a.id); } }, TIMING.ARM_MS));
            armed.set(key, a); emit(a.id); bus.emit('armed', { id: a.id, widgetKey: key });
        }
        function disarm(key) { const a = armed.get(key); if (!a) return; for (const t of a.timers) clearT(t); armed.delete(key); emit(a.id); }
        function discrete(s, cap, patch) {
            const key = s.widgetKey || s.entity + '|-', a = armed.get(key), armedHere = !!a && a.command === s.command && a.id === s.entity;   // a 409 arms too (rule 4)
            if ((s.confirm === true && ARMABLE.includes(s.command)) || cap.sensitive === true || armedHere) {
                if (!s.confirmed) {
                    if (!armedHere) { const r = overlays.get(s.entity); if (r && !r.sent) settle(r); arm(s); return Promise.resolve({ status: 'armed' }); }
                    disarm(key); s.confirmed = true;
                }
            }
            return dispatch(s, cap, patch, !!s.confirmed, false);
        }
        function lane(s) { const key = s.entity + '|' + s.command; let l = lanes.get(key); if (!l) { l = { key, spec: null, cap: null, patch: null, sentAt: -Infinity, sentSig: '', timer: 0, kind: '' }; lanes.set(key, l); } return l; }
        function intermediate(l) {
            const sig = JSON.stringify(l.spec.args); if (sig === l.sentSig) return;
            l.sentAt = now(); l.sentSig = sig;
            safePost(l.spec, false).then(() => {}, () => {});   // 429 dropped (D5); any other failure is decided by the final send
        }
        function finish(l) {
            clearT(l.timer); l.timer = 0; l.kind = ''; l.sentAt = now(); l.sentSig = JSON.stringify(l.spec.args);
            return CONTINUOUS.includes(l.spec.command) ? dispatch(l.spec, l.cap, l.patch, false, false) : discrete(l.spec, l.cap, l.patch);
        }
        function send(spec) {
            if (disposed) return Promise.resolve({ status: 'disposed' });
            const s = Object.assign({}, spec), view = store.view(s.entity), cap = capOf(view, s.command);
            if (!view || !cap || cap.allowed === false) return Promise.resolve({ status: 'refused' });
            if (view.readonly === true || locks.has(s.entity)) return Promise.resolve({ status: 'readonly' });   // rule 9: no POST, no overlay
            let phase = s.phase || 'discrete';
            if ((phase === 'input' || phase === 'commit') && !CONTINUOUS.includes(s.command)) phase = 'discrete';
            const patch = predict(decorate(s.entity, view), s.command, s.args);
            if (phase === 'discrete') return discrete(s, cap, patch);
            const l = lane(s); l.spec = s; l.cap = cap; l.patch = patch; local(s, patch);
            if (phase === 'input') {
                if (l.kind !== 'input') { clearT(l.timer); l.timer = 0; } l.kind = 'input';
                if (!l.timer) l.timer = later(() => { l.timer = 0; intermediate(l); }, Math.max(0, l.sentAt + TIMING.THROTTLE_MS - now()));
                return Promise.resolve({ status: 'local' });
            }
            if (phase === 'key') { clearT(l.timer); l.kind = 'key'; l.timer = later(() => { l.timer = 0; finish(l); }, TIMING.KEY_MS); return Promise.resolve({ status: 'local' }); }
            return finish(l);   // commit
        }
        function decorate(id, view, widgetKey) {
            if (!view) return view;
            const rec = overlays.get(id), c = cues.get(id), fl = flashes.has(id), a = widgetKey ? armed.get(widgetKey) : null, lock = locks.get(id);
            if (!rec && !c && !fl && !(a && a.id === id) && !lock) return view;
            const out = Object.assign({}, view);
            if (rec && rec.patch) {
                if (Object.prototype.hasOwnProperty.call(rec.patch, 'value')) out.value = rec.patch.value;
                if (isObj(rec.patch.attributes)) out.attributes = Object.freeze(Object.assign({}, view.attributes, rec.patch.attributes));
            }
            out.pending = !!(rec && rec.pending);
            if (lock && view.available !== false) { out.readonly = true; if (view.readonlyReason !== 'virtual_desktop.readonly') out.readonlyReason = lock; }   // D5: the desktop reason outranks a lock
            if (c && !rec && out.status === 'ok') { out.status = 'error'; out.error = c.entry; }
            if (fl && !rec && !c) out.confirmed = true;
            if (a && a.id === id) out.armed = Object.freeze({ command: a.command, seconds: Math.max(1, Math.ceil((a.until - now()) / 1000)) });
            return Object.freeze(out);
        }
        function onFrame(id, state) { const rec = overlays.get(id); if (!rec || !rec.sent || !isObj(state)) return; rec.frames.push(state); if (rec.frames.length > FRAMES_KEPT) rec.frames.shift(); if (rec.expected && matches(state, rec.expected, rec.step)) confirmRec(rec); }
        const listen = (type, fn) => { stream.on(type, fn); unhook.push(() => stream.off(type, fn)); };
        const ents = d => (d && Array.isArray(d.entities) ? d.entities : []).filter(x => isObj(x) && typeof x.id === 'string');
        listen('state', d => { if (d && isObj(d.changes)) for (const [id, st] of Object.entries(d.changes)) onFrame(id, st); });
        listen('snapshot', d => { for (const x of ents(d)) onFrame(x.id, x.state); });
        listen('entity', d => { for (const x of ents(d)) { if (locks.delete(x.id)) emit(x.id); onFrame(x.id, x.state); } });
        function clearLocks(reason) { for (const [id, r] of [...locks]) if (!reason || r === reason) { locks.delete(id); emit(id); } }
        function dispose() {
            if (disposed) return; disposed = true; for (const fn of unhook.splice(0)) fn();
            for (const r of overlays.values()) stop(r); for (const c of cues.values()) clearT(c.timer); for (const t of flashes.values()) clearT(t);
            for (const a of armed.values()) for (const t of a.timers) clearT(t); for (const l of lanes.values()) clearT(l.timer);
            overlays.clear(); cues.clear(); flashes.clear(); armed.clear(); lanes.clear(); locks.clear();
        }
        return Object.freeze({ send, decorate, clearLocks, on: bus.on, off: bus.off, dispose, get disposed() { return disposed; } });
    }
    const instances = new Map();
    function shared(options) { const key = options && options.api ? options.api.base : '', hit = instances.get(key); if (hit && !hit.disposed) return hit; const made = create(options); instances.set(key, made); return made; }
    E.command = Object.freeze({ create, shared, predict, matches, valueMatches, capOf, TIMING, CONTINUOUS });
})();
