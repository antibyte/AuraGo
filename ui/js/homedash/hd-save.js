// HomeDash autosave lane and save sheets (frontend 5.8; M4 contract C8). lane(o) keeps one dashboard's local document in step with the server:
// set(doc) marks it dirty and arms an 800 ms debounce; one PUT {revision, document: prepare(doc)} is in flight; changes made meanwhile go out in the
// next PUT; hold(true) (a running gesture) postpones the send. Network, 401, 408, 429 and 5xx retry after 2, 4, 8, 16, 30 s, then every 30 s
// (Retry-After wins); 409 revision_conflict stops in 'conflict' with currentRevision; 404 stops in 'gone'; 403 in 'readonly' until retryNow(); other
// 4xx stop in 'invalid' until the next set(). A document over 256 KiB fails locally. every PUT with a body up to 61 440 bytes is sent with keepalive, so an open PUT survives a tab unload; flush() while a PUT is open queues one follow-up PUT right after it succeeds. conflict, overwrite and reset start a new generation: a late answer of an older PUT is ignored. The realm keeps a
// dirty lane after its window closed (hd-app) and flushes it with keepalive on pagehide. setReadonly(true) stops all sending (phase 'readonly') until setReadonly(false); atRisk() feeds the beforeunload prompt.
(function () {
    'use strict';
    const E = window.HomeDashEngine;
    if (!E || !E.core || !E.api) throw new Error('hd-save.js: load hd-api.js first');
    const BACKOFF = Object.freeze([2000, 4000, 8000, 16000, 30000]), DEBOUNCE_MS = 800, KEEPALIVE_MAX = 61440, DOC_MAX = 262144, STOPS = new Set(['invalid', 'conflict', 'readonly', 'gone']);
    const ERROR_CODES = Object.freeze(['bad_request', 'invalid_id', 'invalid_document', 'limit_exceeded', 'body_too_large', 'schema_too_new', 'slug_taken', 'dashboard_not_found', 'read_only', 'desktop_readonly', 'homedash_disabled', 'storage_unavailable', 'network', 'csrf_check_failed', 'rate_limited', 'method_not_allowed', 'bad_json',
        'required', 'too_long', 'bad_format', 'out_of_range', 'not_allowed', 'control_chars', 'url_not_allowed', 'duplicate_id', 'unknown_type', 'unknown_variant', 'unknown_binding', 'binding_required', 'bad_entity_id', 'too_many', 'unknown_option', 'bad_option', 'page_not_found', 'toggle_unsupported',
        'layout_missing', 'layout_duplicate', 'layout_unknown_widget', 'layout_overlap', 'layout_out_of_bounds', 'size_out_of_range', 'document_too_large', 'entity_unknown', 'kind_mismatch', 'capability_missing']);
    function utf8Bytes(s) { let n = 0; for (let i = 0; i < s.length; i++) { const c = s.charCodeAt(i); if (c < 0x80) n += 1; else if (c < 0x800) n += 2; else if (c >= 0xd800 && c <= 0xdbff && i + 1 < s.length) { n += 4; i += 1; } else n += 3; } return n; }
    function classify(err) { const s = err.status || 0, c = err.code || '';
        if (c === 'revision_conflict') return 'conflict'; if (s === 404 || c === 'dashboard_not_found') return 'gone'; if (s === 403) return 'readonly';
        if (s === 0 || s === 401 || s === 408 || s === 429 || s >= 500) return 'retry'; return 'invalid'; }
    function lane(o) {
        const api = o.api, id = o.id, tm = o.timers || {}, setT = tm.setTimeout || ((fn, ms) => setTimeout(fn, ms)), clearT = tm.clearTimeout || (h => clearTimeout(h)), now = tm.now || (() => Date.now());
        const bus = E.core.emitter(), debounce = Number.isFinite(o.debounceMs) ? o.debounceMs : DEBOUNCE_MS;
        let doc = o.doc, saved = o.doc, revision = Number(o.revision) || 0, sent = null, dirty = false, holding = false, phase = 'saved', attempt = 0, nextAt = 0, current = null, error = null, warnings = [];
        let gen = 0, ro = false, sentKeep = true, flushAsk = false, timer = 0, retryTimer = 0, changedAt = 0, owner = null, disposed = false, prepare = typeof o.prepare === 'function' ? o.prepare : (d => d);
        const snapshot = () => Object.freeze({ phase, dirty, saving: !!sent, revision, attempt, nextRetryAt: nextAt, currentRevision: current, error, warnings });
        const emit = () => { if (!disposed) bus.emit('state', snapshot()); };
        function stopTimers() { clearT(timer); timer = 0; clearT(retryTimer); retryTimer = 0; nextAt = 0; }
        function schedule() { clearT(timer); timer = 0; if (disposed || !dirty || holding || ro || sent || retryTimer || STOPS.has(phase)) return;
            phase = 'pending'; timer = setT(() => { timer = 0; send(); }, Math.max(0, debounce - (now() - changedAt))); }
        async function send() {
            if (disposed || sent || !dirty || ro) return; clearT(timer); timer = 0;
            const body = prepare(doc), bytes = utf8Bytes(JSON.stringify(body));
            if (bytes > DOC_MAX) { phase = 'invalid'; error = Object.freeze({ code: 'limit_exceeded', status: 0, issues: [] }); emit(); return; }
            const mine = doc, g = gen; sent = mine; sentKeep = bytes <= KEEPALIVE_MAX; phase = 'saving'; emit();
            try {
                const answer = await api.saveDashboard(id, revision, body, bytes <= KEEPALIVE_MAX ? { keepalive: true } : undefined); if (disposed || g !== gen) return;
                const d = answer && answer.dashboard; revision = Number(d && d.revision) || revision + 1; saved = mine; sent = null; attempt = 0; error = null; current = null;
                warnings = Array.isArray(answer && answer.warnings) ? answer.warnings.slice(0, 50) : []; if (doc === mine) dirty = false; phase = dirty ? (ro ? 'readonly' : 'pending') : 'saved';
                bus.emit('saved', Object.freeze({ revision, doc: mine, body, warnings }));
                if (dirty && flushAsk) { flushAsk = false; send(); } else { flushAsk = false; schedule(); } emit();
            } catch (raw) {
                if (disposed || g !== gen) return; const err = E.api.normalizeError(raw), kind = classify(err); sent = null; flushAsk = false;
                error = Object.freeze({ code: err.code, status: err.status, issues: Array.isArray(err.data && err.data.issues) ? err.data.issues.slice(0, 50) : [] });
                if (kind === 'retry' && ro) phase = 'readonly';   // read-only meanwhile: no retry timer, setReadonly(false) resumes
                else if (kind === 'retry') { const ms = Number.isFinite(err.retryAfter) && err.retryAfter > 0 ? Math.round(err.retryAfter * 1000) : BACKOFF[Math.min(attempt, BACKOFF.length - 1)];
                    attempt += 1; phase = 'retrying'; nextAt = now() + ms; retryTimer = setT(() => { retryTimer = 0; nextAt = 0; if (holding) { phase = 'pending'; emit(); return; } send(); }, ms); }
                else if (kind === 'conflict') { phase = 'conflict'; current = Number(err.data && err.data.current_revision) || null; }
                else phase = kind;
                emit();
            }
        }
        function settle(ms) { return new Promise(resolve => { let done = false, t0 = 0, off = null;
            const check = st => { if (done || st.saving || st.phase === 'pending') return; done = true; if (off) off(); if (t0) clearT(t0); resolve(st.phase); };
            off = bus.on('state', check); if (ms > 0) t0 = setT(() => { if (!done) { done = true; off(); resolve(phase); } }, ms); check(snapshot()); }); }
        return Object.freeze({
            id, on: bus.on, off: bus.off, state: snapshot, doc: () => doc, saved: () => saved, revision: () => revision, settle,
            get disposed() { return disposed; }, get owner() { return owner; },
            set(next) { if (disposed || !next || next === doc) return; doc = next; dirty = doc !== saved; changedAt = now(); if (phase === 'invalid') phase = 'pending'; if (!dirty && !sent && !retryTimer && !STOPS.has(phase)) phase = 'saved';
                if (dirty && holding && phase === 'saved') phase = 'pending'; if (dirty && ro && !sent) phase = 'readonly'; schedule(); emit(); },
            hold(on) { holding = !!on; if (holding) { clearT(timer); timer = 0; } else schedule(); },
            flush(fo) { if (disposed) return Promise.resolve(phase); const ka = !!(fo && fo.keepalive); if (retryTimer) { clearT(retryTimer); retryTimer = 0; nextAt = 0; }
                if (sent) { if (dirty) flushAsk = true; } else if (dirty && !STOPS.has(phase)) send(); return settle(ka ? 0 : 5000); },
            retryNow() { if (disposed || ro || !dirty || !['retrying', 'readonly', 'pending'].includes(phase)) return; stopTimers(); phase = 'pending'; send(); },
            // Read-only (desktop or HomeDash): nothing is sent, a dirty lane waits in 'readonly' (an open PUT may still finish); lifting it resumes with retryNow().
            setReadonly(on) { if (disposed || ro === !!on) return; ro = !!on;
                if (ro) { clearT(timer); timer = 0; clearT(retryTimer); retryTimer = 0; nextAt = 0; if (dirty && !sent) { phase = 'readonly'; emit(); } }
                else if (dirty && phase === 'readonly') { phase = 'pending'; emit(); schedule(); } },
            // true while keepalive cannot guarantee the unsaved changes: a dirty body over KEEPALIVE_MAX (also an open PUT sent without keepalive), or an open PUT with a queued follow-up (beforeunload prompts).
            atRisk() { return !disposed && dirty && (sent ? (sent !== doc || !sentKeep) : utf8Bytes(JSON.stringify(prepare(doc))) > KEEPALIVE_MAX); },
            conflict(rev) { if (disposed) return; gen++; sent = null; flushAsk = false; stopTimers(); phase = 'conflict'; current = Number(rev) || current; emit(); },
            overwrite(rev) { if (disposed) return; gen++; sent = null; flushAsk = false; stopTimers(); revision = Number(rev) || revision; current = null; error = null; phase = 'pending'; dirty = true; send(); },
            reset(next, rev) { if (disposed) return; gen++; sent = null; flushAsk = false; stopTimers(); doc = saved = next; revision = Number(rev) || revision; dirty = false; current = null; error = null; attempt = 0; phase = 'saved'; emit(); },
            setPrepare(fn) { prepare = typeof fn === 'function' ? fn : (d => d); }, attach(who) { owner = who; emit(); }, detach(who) { if (owner === who) owner = null; emit(); },
            dispose() { if (disposed) return; flushAsk = false; stopTimers(); bus.emit('state', Object.freeze(Object.assign({}, snapshot(), { phase: 'stopped' }))); disposed = true; }
        });
    }
    function errorText(t, error) { const e = error || {}, issue = Array.isArray(e.issues) ? e.issues.find(i => i && ERROR_CODES.includes(i.code)) : null, code = issue ? issue.code : ERROR_CODES.includes(e.code) ? e.code : '';
        return code ? t('homedash.error.' + code) : t('homedash.error.generic', { code: e.code || '?' }); }
    // Sheets: every key is a literal translation call (a ternary between two keys would escape the key lint).
    function when(info, lang) { const v = info && info.updatedAt, d = typeof v === 'number' ? new Date(v) : typeof v === 'string' ? new Date(v) : null; if (!d || Number.isNaN(d.getTime())) return '';
        try { return new Intl.DateTimeFormat(lang, { hour: '2-digit', minute: '2-digit' }).format(d); } catch (err) { return ''; } }
    function conflictSheet(o) {
        const t = o.t, info = o.info || {}, time = when(info, o.lang), gone = !!info.gone, vars = { time, name: info.savedBy || '' };
        const body = gone ? t('homedash.conflict.gone_body', vars) : info.savedBy === 'agent' ? t('homedash.conflict.body_agent', vars) : info.savedBy ? t('homedash.conflict.body_other', vars) : t('homedash.conflict.body_plain', vars);
        const later = { id: 'later', label: t('homedash.conflict.later') }, copy = { id: 'copy', label: t('homedash.conflict.copy'), tone: 'primary', autofocus: true };
        const actions = gone ? [later, copy] : [later, { id: 'theirs', label: t('homedash.conflict.theirs') }, { id: 'mine', label: t('homedash.conflict.mine') }, copy];
        return o.sheet.open({ title: gone ? t('homedash.conflict.gone_title') : t('homedash.conflict.title'), text: [body], kind: 'conflict', actions });
    }
    function leaveSheet(o) {
        const t = o.t, close = o.mode === 'close', trying = ['retrying', 'pending', 'saving'].includes(o.phase);
        const body = trying ? t('homedash.leave.body_retrying') : o.phase === 'conflict' ? t('homedash.leave.body_conflict') : o.phase === 'gone' ? t('homedash.leave.body_gone') : t('homedash.leave.body_invalid', { reason: o.reason || '' });
        const actions = trying ? [{ id: 'keep', label: close ? t('homedash.leave.close_keep') : t('homedash.leave.keep') }, { id: 'leave', label: close ? t('homedash.leave.close_leave') : t('homedash.leave.leave'), tone: 'primary', autofocus: true }]
            : [{ id: 'keep', label: close ? t('homedash.leave.close_keep') : t('homedash.leave.keep'), tone: 'primary', autofocus: true }, { id: 'discard', label: close ? t('homedash.leave.close_discard') : t('homedash.leave.discard'), tone: 'danger' }];
        return o.sheet.open({ title: t('homedash.leave.title'), text: [body], kind: 'leave', actions });
    }
    function draftsSheet(o) {
        const t = o.t;
        return o.sheet.open({ title: t('homedash.leave.drafts_title'), text: [o.count === 1 ? t('homedash.leave.drafts_one') : t('homedash.leave.drafts_many', { count: o.count })], kind: 'leave',
            actions: [{ id: 'leave', label: t('homedash.leave.drafts_leave') }, { id: 'choose', label: t('homedash.leave.drafts_choose'), tone: 'primary', autofocus: true }] });
    }
    E.save = Object.freeze({ BACKOFF, DEBOUNCE_MS, KEEPALIVE_MAX, DOC_MAX, ERROR_CODES, lane, classify, utf8Bytes, errorText, conflictSheet, leaveSheet, draftsSheet });
})();
