// Read-only, invocation-scoped child records; parent execution, models and drafts stay untouched.
(function () {
    'use strict';
    const ED = window.EasyDrag = window.EasyDrag || {};
    const STATES = new Set(['pending', 'complete', 'truncated', 'incomplete', 'legacy_unknown']);
    const OUTCOMES = new Set(['success', 'error', 'cancelled']);
    const STEP_STATES = new Set(['pending', 'running', 'success', 'error', 'skipped', 'waiting', 'cancelled']);
    const DOC_MISSING = new Set(['unavailable', 'redacted', 'budget']);
    const object = v => !!v && typeof v === 'object' && !Array.isArray(v);
    const integer = (v, min, max) => Number.isSafeInteger(v) && v >= min && v <= max;
    const text = (v, max) => typeof v === 'string' && v.length > 0 && v.length <= max;
    const invocation = v => typeof v === 'string' && /^i_[1-9]\d{0,3}$/.test(v) && Number(v.slice(2)) <= 1000;
    const hash = v => typeof v === 'string' && /^[a-f0-9]{64}$/.test(v);

    // Bound traversal before rendering/stringifying; include JSON escaping and structural overhead.
    function fits(value, bytes, maxDepth) {
        const stack = [[value, 0]]; let nodes = 0;
        while (stack.length) {
            const [v, depth] = stack.pop();
            if (++nodes > 50000 || depth > maxDepth) return false;
            if (typeof v === 'string') {
                if (v.length > bytes) return false;
                bytes -= 2;
                for (const ch of v) { const n = ch.codePointAt(0); if (n >= 0xd800 && n <= 0xdfff) return false; bytes -= n < 32 ? 6 : ch === '"' || ch === '\\' ? 2 : n < 128 ? 1 : n < 2048 ? 2 : n < 65536 ? 3 : 4; if (bytes < 0) return false; }
            } else if (v && typeof v === 'object') {
                if (Array.isArray(v) && v.length > 50000) return false;
                const keys = Object.keys(v); bytes -= 2 + Math.max(0, keys.length - 1) + (Array.isArray(v) ? 0 : keys.length);
                if (keys.length + nodes + stack.length > 50000) return false;
                keys.forEach(k => { if (!Array.isArray(v)) stack.push([k, depth + 1]); stack.push([v[k], depth + 1]); });
            } else { if (!['boolean', 'number', 'undefined'].includes(typeof v) && v !== null || typeof v === 'number' && !Number.isFinite(v)) return false; bytes -= v === undefined ? 0 : String(v).length; }
            if (bytes < 0) return false;
        }
        return true;
    }
    function summary(v, seq) {
        if (!object(v) || !integer(v.parent_seq, 1, Number.MAX_SAFE_INTEGER) || seq !== undefined && v.parent_seq !== seq || !STATES.has(v.state) || v.truncated !== undefined && typeof v.truncated !== 'boolean') return null;
        const result = { parent_seq: v.parent_seq, state: v.state, truncated: v.truncated === true };
        for (const k of ['retained_invocations', 'total_invocations', 'retained_steps', 'total_steps']) {
            if (v[k] !== null && !integer(v[k], 0, 1000000000)) return null;
            result[k] = v[k];
        }
        return result;
    }
    function fullValue(v, partial) {
        if (v === undefined) return undefined;
        if (partial || object(v) && (Object.hasOwn(v, '_preview') || Object.hasOwn(v, '_omitted')) || !fits(v, 16384, 64)) return { _preview: '', _omitted: 'child_history_omitted' };
        return v;
    }
    function trace(v, owner) {
        if (!object(v) || v.schema !== 1 || v.parent_seq !== owner.seq || (owner.simulation ? Object.hasOwn(v, 'run_id') : v.run_id !== owner.runId) || !fits(v, 262144, 8192) || !Array.isArray(v.frames) || v.frames.length > 1000) throw new Error('trace');
        const history = summary(v.history, owner.seq); if (!history) throw new Error('history');
        let records = 0; const ids = new Set();
        const frames = v.frames.map(f => {
            if (!object(f) || !invocation(f.invocation_id) || ids.has(f.invocation_id) || f.parent_invocation_id !== undefined && !invocation(f.parent_invocation_id) || !text(f.call_node_id, 128) || !integer(f.call_attempt, 0, 10000) || !text(f.block_id, 128) || !integer(f.revision, 1, Number.MAX_SAFE_INTEGER) || !hash(f.hash) || typeof f.entered !== 'boolean' || f.item_index !== undefined && !integer(f.item_index, 0, 999) || !Array.isArray(f.steps) || (records += f.steps.length) > 2000) throw new Error('frame');
            ids.add(f.invocation_id); let last = 0;
            const steps = f.steps.map(row => {
                const s = row && row.step;
                if (!integer(row && row.ordinal, 1, 2000) || row.ordinal <= last || !object(s) || !text(s.node_id, 128) || !integer(s.attempt, 0, 10000) || !STEP_STATES.has(s.status)) throw new Error('record');
                last = row.ordinal;
                const step = { node_id: s.node_id, node_key: typeof s.node_key === 'string' ? s.node_key.slice(0, 128) : '', attempt: s.attempt, status: s.status };
                for (const k of ['started_at', 'finished_at']) if (typeof s[k] === 'string' && s[k].length <= 64 && !s[k].startsWith('0001-') && Number.isFinite(Date.parse(s[k]))) step[k] = s[k];
                if (integer(s.duration_ms, 0, Number.MAX_SAFE_INTEGER)) step.duration_ms = s.duration_ms;
                if (typeof s.error_code === 'string' && /^FLOW_[A-Z0-9_]{1,60}$/.test(s.error_code)) step.error_code = s.error_code;
                step.params = fullValue(s.params, s.params_truncated); step.output = fullValue(s.output, s.output_truncated);
                step.params_truncated = !!s.params_truncated; step.output_truncated = !!s.output_truncated;
                return { ordinal: row.ordinal, step };
            });
            return { invocation_id: f.invocation_id, parent_invocation_id: f.parent_invocation_id, call_node_id: f.call_node_id, call_attempt: f.call_attempt, block_id: f.block_id, revision: f.revision, hash: f.hash, item_index: f.item_index, entered: f.entered, status: OUTCOMES.has(f.status) ? f.status : '', error_code: typeof f.error_code === 'string' && /^FLOW_[A-Z0-9_]{1,60}$/.test(f.error_code) ? f.error_code : '', steps };
        });
        const byId = new Map(frames.map(f => [f.invocation_id, f]));
        for (const f of frames) { const seen = new Set(); let at = f; while (at) { if (seen.has(at.invocation_id) || seen.size > 8) throw new Error('ancestry'); seen.add(at.invocation_id); at = byId.get(at.parent_invocation_id); } }
        return { schema: 1, run_id: v.run_id, parent_seq: v.parent_seq, history, frames, byId };
    }
    function stateText(t, state) { return t(state === 'unavailable' ? 'easydrag.ui.error_flow_child_history_unavailable' : 'easydrag.ui.child_history_' + (state === 'legacy_unknown' ? 'legacy' : state)); }
    function source(ed, override) {
        const run = override || ed.run, record = run && (run.record || run);
        return { run, record, simulation: record?.mode === 'simulation' || run?.mode === 'simulation', runId: record?.id || run?.id };
    }
    function actionMarkup(ed, step, type) {
        return step && (step.child_history || ['component.call', 'component.each'].includes(type)) ? '<button type="button" class="ed-btn" data-ed-child-open>' + ed.esc(ed.t('easydrag.ui.child_history_open')) + '</button>' : '';
    }
    function rememberSimulation(run, result) {
        run.child_traces = Array.isArray(result.child_traces) ? result.child_traces : [];
        run.child_documents = Array.isArray(result.child_documents) ? result.child_documents : [];
    }
    function create(ed) {
        const core = ED.core, { t, esc } = ed, bag = core.bag();
        let active = null, choice = null, disposed = false;
        function close(restore) {
            const v = active; if (!v) return;
            active = null; v.closed = true; v.seq++; v.requests.forEach(c => c.abort()); v.requests.clear(); v.docs.clear(); v.el.remove();
            if (v.host) v.host.parentElement?.classList.remove('is-child-history');
            if (v.modal) v.modal.close(null);
            if (restore !== false && v.opener?.isConnected) v.opener.focus({ preventScroll: true });
        }
        const live = v => !disposed && active === v && !v.closed && v.el.isConnected && ed.root.isConnected;
        function status(v, state) { v.state = state; render(v); }
        async function request(v, work, entry) {
            const controller = new AbortController(); v.requests.add(controller);
            if (entry) entry.controller = controller;
            try { return await work(controller.signal); }
            catch (err) {
                if ([401, 403].includes(Number(err?.status)) && live(v)) {
                    v.denied = true; v.seq++; v.docSeq++; v.loading = false;
                    v.requests.forEach(c => c.abort()); v.docs.clear(); v.trace = null; v.summary = null;
                    v.owner = {}; v.frameId = ''; v.record = 0; v.page = 0;
                    status(v, 'unavailable');
                }
                throw err;
            }
            finally { v.requests.delete(controller); if (entry) entry.controller = null; }
        }
        function selected(v) { return v.trace?.byId.get(v.frameId) || null; }
        const keyOf = f => f.block_id + ':' + f.revision + ':' + f.hash;
        function documentFor(v, f) { return v.docs.get(keyOf(f)); }
        function frameName(v, f) {
            const doc = documentFor(v, f)?.doc;
            return t('easydrag.ui.child_history_reference', { block: typeof doc?.name === 'string' && doc.name || f.block_id, revision: f.revision }) + (f.item_index !== undefined ? ' · ' + t('easydrag.ui.child_history_item', { number: f.item_index + 1 }) : '') + (f.call_attempt ? ' · ' + t('easydrag.ui.child_history_attempt', { attempt: f.call_attempt }) : '');
        }
        const callName = f => t('easydrag.ui.child_history_invocation', { number: f.invocation_id.slice(2) }); // Display only; routing keeps the opaque ID.
        function nodeName(v, f, s) { return documentFor(v, f)?.doc?.nodes?.find(n => n && n.id === s.node_id)?.label || s.node_key || s.node_id; }
        async function loadDocument(v, f) {
            const key = keyOf(f), ticket = ++v.docSeq;
            let entry = v.docs.get(key);
            if (!entry) {
                entry = { pending: true }; v.docs.set(key, entry);
                while (v.docs.size > 2) { const oldest = v.docs.keys().next().value; v.docs.get(oldest).controller?.abort(); v.docs.delete(oldest); }
                entry.promise = (async () => {
                    let value = { doc_missing: 'unavailable' };
                    try {
                        const raw = v.owner.simulation ? (v.owner.run.child_documents || v.owner.record.child_documents || []).find(d => d.block_id === f.block_id && d.revision === f.revision && d.hash === f.hash) || value : await request(v, signal => ed.api.childDocument(v.owner.runId, v.seqValue, f.invocation_id, signal), entry);
                        value = { doc_missing: DOC_MISSING.has(raw?.doc_missing) ? raw.doc_missing : 'unavailable' };
                        if (raw?.block_id === f.block_id && raw.revision === f.revision && raw.hash === f.hash && object(raw.doc) && raw.doc.id === f.block_id && raw.doc.kind === 'block' && Array.isArray(raw.doc.nodes) && raw.doc.nodes.length <= 500 && fits(raw.doc, 2 * 1024 * 1024, 8192)) value = { doc: raw.doc };
                    } catch (_) { /* Recorded values remain available without document context. */ }
                    if (live(v) && !v.denied && v.docs.get(key) === entry) { v.docs.set(key, value); while (v.docs.size > 2) v.docs.delete(v.docs.keys().next().value); }
                })();
            }
            await entry.promise;
            if (live(v) && !v.denied && ticket === v.docSeq && selected(v)?.invocation_id === f.invocation_id) renderContext(v);
        }
        function choose(v, id) {
            if (!v.trace?.byId.has(id)) return;
            v.frameId = id; v.record = 0; v.page = 0; v.list = 'steps'; render(v); loadDocument(v, selected(v));
            v.el.querySelector('[data-ed-child-heading]')?.focus({ preventScroll: true });
        }
        function range(v, length, pageSize, total) {
            const start = v.page * pageSize, end = Math.min(length, start + pageSize);
            return '<div class="ed-child-pager"><button type="button" class="ed-btn" data-ed-child-prev-page' + (!start ? ' disabled' : '') + '>' + esc(t('easydrag.ui.child_history_previous_page')) + '</button><span>' + esc(t('easydrag.ui.child_history_range', { first: length ? start + 1 : 0, last: end, retained: length, total: total == null ? '—' : total })) + '</span><button type="button" class="ed-btn" data-ed-child-next-page' + (end >= length ? ' disabled' : '') + '>' + esc(t('easydrag.ui.child_history_next_page')) + '</button></div>';
        }
        function render(v) {
            if (!live(v)) return;
            const focused = document.activeElement, hadFocus = v.el.contains(focused);
            const focusAttr = hadFocus && Array.from(focused.attributes || []).find(attr => attr.name.startsWith('data-ed-child-'));
            const f = selected(v), history = v.trace?.history || v.summary;
            let nav = '<button type="button" class="ed-btn" data-ed-child-back>' + core.icon('arrow-left') + '<span>' + esc(t('easydrag.ui.inspector_back')) + '</span></button>';
            if (f) {
                const chain = []; let at = f; while (at && chain.length < 9) { chain.unshift(at); at = v.trace.byId.get(at.parent_invocation_id); }
                nav += '<details class="ed-child-path"><summary aria-label="' + esc(t('easydrag.ui.child_history_navigation')) + '">' + esc(t('easydrag.ui.child_history_calls')) + '</summary><button type="button" class="ed-btn" data-ed-child-root>' + esc(t('easydrag.ui.child_history_calls')) + '</button>' + chain.map(p => '<button type="button" class="ed-btn" data-ed-child-frame="' + p.invocation_id + '"' + (p === f ? ' aria-current="location"' : '') + '>' + esc(callName(p) + ' · ' + frameName(v, p)) + '</button>').join('') + '</details>';
            }
            nav += '<button type="button" class="ed-icon-btn" data-ed-child-reload aria-label="' + esc(t('easydrag.ui.reload')) + '" title="' + esc(t('easydrag.ui.reload')) + '"' + (v.loading || v.denied ? ' disabled' : '') + '>' + core.icon('refresh') + '</button>';
            let body = '<p role="status" class="ed-hint">' + esc(v.loading ? t('easydrag.ui.loading') : stateText(t, v.state)) + '</p>';
            if (history?.truncated && v.state !== 'truncated') body += '<p class="ed-hint">' + esc(stateText(t, 'truncated')) + '</p>';
            if (v.trace) {
                if (!f || v.list === 'calls') {
                    const frames = v.trace.frames.filter(row => !f || row.parent_invocation_id === f.invocation_id);
                    body += '<h4>' + esc(t('easydrag.ui.child_history_calls')) + '</h4><div class="ed-child-list">' + frames.slice(v.page * 50, v.page * 50 + 50).map(row => '<button type="button" class="ed-child-row" data-ed-child-frame="' + row.invocation_id + '"><strong>' + esc(frameName(v, row)) + '</strong><span>' + esc(callName(row) + ' · ' + (row.status ? core.tr(t, 'easydrag.ui.status_' + row.status, row.status) : t('easydrag.ui.child_history_unknown_outcome'))) + '</span></button>').join('') + '</div>';
                    if (!frames.length && v.state === 'complete') body += '<p>' + esc(t('easydrag.ui.child_history_empty')) + '</p>';
                    body += range(v, frames.length, 50, f ? null : history?.total_invocations);
                } else {
                    const doc = documentFor(v, f);
                    body += '<p class="ed-hint" data-ed-child-document-missing' + (doc?.doc_missing ? '' : ' hidden') + '>' + (doc?.doc_missing ? esc(t('easydrag.ui.child_document_' + doc.doc_missing)) : '') + '</p>';
                    body += '<p class="ed-hint">' + esc(callName(f) + ' · ' + (f.status ? core.tr(t, 'easydrag.ui.status_' + f.status, f.status) : t('easydrag.ui.child_history_unknown_outcome'))) + '</p>';
                    if (!f.entered) body += '<p>' + esc(t('easydrag.ui.child_history_not_started')) + '</p>';
                    if (f.error_code) body += '<p class="ed-error">' + esc(core.errorText(t, { body: { code: f.error_code } })) + '</p>';
                    const row = f.steps.find(s => s.ordinal === v.record);
                    if (!row) {
                        if (v.trace.frames.some(row => row.parent_invocation_id === f.invocation_id)) body += '<button type="button" class="ed-btn" data-ed-child-calls>' + esc(t('easydrag.ui.child_history_calls')) + '</button>';
                        body += '<div class="ed-child-list">' + f.steps.slice(v.page * 100, v.page * 100 + 100).map(row => '<button type="button" class="ed-child-row" data-ed-child-record="' + row.ordinal + '" aria-label="' + esc(t('easydrag.ui.child_history_open_record', { name: nodeName(v, f, row.step), attempt: row.step.attempt || '—' })) + '"><strong>' + esc(nodeName(v, f, row.step)) + '</strong><span>' + esc(core.tr(t, 'easydrag.ui.status_' + row.step.status, row.step.status)) + (row.step.attempt ? ' · ' + esc(t('easydrag.ui.child_history_attempt', { attempt: row.step.attempt })) : '') + '</span></button>').join('') + '</div>' + range(v, f.steps.length, 100, null);
                    }
                    if (row) {
                        const s = row.step;
                        body += '<section class="ed-child-record"><h4 tabindex="-1" data-ed-child-record-heading>' + esc(nodeName(v, f, s)) + '</h4><div class="ed-seg" role="group">' + ['params', 'output'].map(k => '<button type="button" class="ed-btn ed-btn--ghost" data-ed-child-subject="' + k + '" aria-pressed="' + (v.subject === k) + '">' + esc(t(k === 'params' ? 'easydrag.ui.detail_resolved_params' : 'easydrag.ui.detail_output')) + '</button>').join('') + '</div><div class="ed-seg" role="group">' + ['tree', 'table', 'json'].map(k => '<button type="button" class="ed-btn ed-btn--ghost" data-ed-child-view="' + k + '" aria-pressed="' + (v.view === k) + '">' + esc(t('easydrag.ui.view_' + k)) + '</button>').join('') + '</div>';
                        if (s.error_code) body += '<p class="ed-error">' + esc(core.errorText(t, { body: { code: s.error_code } })) + '</p>';
                        body += '<div data-ed-child-value>' + ED.detail.recordValueMarkup(ed, s[v.subject], v.view) + '</div>';
                        if (s.started_at || s.finished_at) body += '<p class="ed-hint">' + [s.started_at, s.finished_at].map(at => at ? '<time datetime="' + esc(at) + '">' + esc(core.fmt.dateTime(at, true)) + '</time>' : '—').join(' – ') + '</p>';
                        body += '</section>';
                    }
                }
            }
            const heading = f ? frameName(v, f) : t(v.modal ? 'easydrag.ui.child_history_calls' : 'easydrag.ui.child_history_title');
            v.el.innerHTML = '<nav class="ed-child-nav" aria-label="' + esc(t('easydrag.ui.child_history_navigation')) + '">' + nav + '</nav><h3 tabindex="-1" data-ed-child-heading>' + esc(heading) + '</h3>' + body + '<details class="ed-child-help"><summary>' + esc(t('easydrag.ui.child_history_values_hint')) + '</summary><p>' + esc(t('easydrag.ui.child_history_time_hint')) + '</p></details>';
            if (hadFocus) {
                const match = focusAttr && Array.from(v.el.querySelectorAll('[' + focusAttr.name + ']')).find(el => el.getAttribute(focusAttr.name) === focusAttr.value && !el.disabled);
                (match || v.el.querySelector('[data-ed-child-heading]'))?.focus({ preventScroll: true });
            }
        }
        // Document arrival only supplies labels/context. Keep record controls and value DOM intact
        // so an in-flight pointer click, keyboard focus or expanded value tree is not discarded.
        function renderContext(v) {
            const f = selected(v); if (!f) return;
            v.el.querySelector('[data-ed-child-heading]').textContent = frameName(v, f);
            v.el.querySelectorAll('[data-ed-child-frame]').forEach(button => {
                const frame = v.trace.byId.get(button.dataset.edChildFrame);
                if (frame) { const label = button.querySelector('strong'); (label || button).textContent = (label ? '' : callName(frame) + ' · ') + frameName(v, frame); }
            });
            v.el.querySelectorAll('[data-ed-child-record]').forEach(button => {
                const row = f.steps.find(row => String(row.ordinal) === button.dataset.edChildRecord);
                if (!row) return;
                button.querySelector('strong').textContent = nodeName(v, f, row.step);
                button.setAttribute('aria-label', t('easydrag.ui.child_history_open_record', { name: nodeName(v, f, row.step), attempt: row.step.attempt || '—' }));
            });
            const heading = v.el.querySelector('[data-ed-child-record-heading]'), row = f.steps.find(row => row.ordinal === v.record);
            if (heading && row) heading.textContent = nodeName(v, f, row.step);
            const missing = v.el.querySelector('[data-ed-child-document-missing]'), doc = documentFor(v, f);
            if (missing) { missing.hidden = !doc?.doc_missing; missing.textContent = doc?.doc_missing ? t('easydrag.ui.child_document_' + doc.doc_missing) : ''; }
        }
        async function load(v) {
            if (!live(v) || v.loading || v.denied) return;
            v.loading = true; const ticket = ++v.seq; render(v);
            try {
                if (!v.seqValue) return; // An unfinished call may not yet have a persisted sequence.
                const raw = v.owner.simulation ? (v.owner.run.child_traces || v.owner.record.child_traces || []).find(row => row.parent_seq === v.seqValue) : await request(v, signal => ed.api.childTrace(v.owner.runId, v.seqValue, signal));
                if (!live(v) || ticket !== v.seq) return;
                v.trace = trace(raw, { seq: v.seqValue, runId: v.owner.runId, simulation: v.owner.simulation }); status(v, v.trace.history.state);
                if (selected(v)) loadDocument(v, selected(v));
            } catch (err) { if (live(v) && ticket === v.seq && err.name !== 'AbortError') { v.trace = null; status(v, 'unavailable'); } }
            finally { if (live(v) && ticket === v.seq) { v.loading = false; render(v); } }
        }
        function useModal(v) {
            if (v.modal) return;
            const focused = document.activeElement;
            if (v.host) { v.host.parentElement?.classList.remove('is-child-history'); v.host = null; }
            v.modal = core.modal(ed.root, { title: t('easydrag.ui.child_history_title'), closeLabel: t('easydrag.ui.close'), className: 'ed-modal--children', body: '', actions: [{ id: 'close', label: t('easydrag.ui.close') }] });
            v.modal.body.appendChild(v.el);
            if (!v.frameId) { const heading = v.el.querySelector('[data-ed-child-heading]'); if (heading) heading.textContent = t('easydrag.ui.child_history_calls'); }
            v.modal.done.then(() => { if (active === v) close(false); });
            if (focused && v.el.contains(focused)) focused.focus({ preventScroll: true });
        }
        function open(step, options) {
            const opts = options || {}; close(false); if (choice) { choice.close(null); choice = null; }
            const owner = source(ed, opts.run), meta = summary(step?.child_history);
            const v = { owner, ambient: ed.run, originHost: opts.host, summary: meta, seqValue: meta?.parent_seq, state: meta?.state || (['pending', 'running', 'waiting', 'queued'].includes(step?.status) ? 'pending' : 'legacy_unknown'), trace: null, frameId: '', record: 0, list: 'calls', page: 0, subject: 'output', view: 'tree', docs: new Map(), requests: new Set(), loading: false, closed: false, seq: 0, docSeq: 0, opener: opts.opener, el: core.el('<section class="ed-child-history" data-ed-child-history></section>') };
            if (!opts.host || ed.root.getBoundingClientRect().width < 760) useModal(v);
            else { v.host = opts.host; opts.host.appendChild(v.el); opts.host.parentElement?.classList.add('is-child-history'); }
            active = v; render(v); v.el.querySelector('[data-ed-child-heading]')?.focus({ preventScroll: true });
            v.el.addEventListener('keydown', event => { if (event.key === 'Escape' && !event.isComposing) { event.preventDefault(); event.stopPropagation(); close(); } else if (event.key === 'ArrowLeft' || event.key === 'ArrowRight') event.stopPropagation(); });
            v.el.addEventListener('click', event => {
                if (!live(v)) return;
                const frame = event.target.closest('[data-ed-child-frame]'), record = event.target.closest('[data-ed-child-record]'), subject = event.target.closest('[data-ed-child-subject]'), view = event.target.closest('[data-ed-child-view]');
                if (frame) { choose(v, frame.dataset.edChildFrame); return; }
                if (record) { v.record = Number(record.dataset.edChildRecord); render(v); (v.modal?.body || v.host).scrollTop = 0; v.el.querySelector('[data-ed-child-record-heading]')?.focus({ preventScroll: true }); return; }
                if (subject) { v.subject = subject.dataset.edChildSubject; render(v); return; }
                if (view) { v.view = view.dataset.edChildView; render(v); return; }
                if (event.target.closest('[data-ed-child-reload]')) { if (v.loading || v.denied) return; v.docSeq++; v.page = 0; v.requests.forEach(c => c.abort()); v.docs.clear(); load(v); return; }
                if (event.target.closest('[data-ed-child-root]')) { v.frameId = ''; v.record = 0; v.list = 'calls'; v.page = 0; render(v); return; }
                if (event.target.closest('[data-ed-child-calls]')) { v.list = 'calls'; v.page = 0; render(v); return; }
                if (event.target.closest('[data-ed-child-back]')) { if (v.record) { v.record = 0; render(v); } else if (v.trace?.byId.has(selected(v)?.parent_invocation_id)) choose(v, selected(v).parent_invocation_id); else if (v.frameId) { v.frameId = ''; v.list = 'calls'; v.page = 0; render(v); } else close(); return; }
                if (event.target.closest('[data-ed-child-next-page]')) { v.page++; render(v); return; }
                if (event.target.closest('[data-ed-child-prev-page]')) { v.page = Math.max(0, v.page - 1); render(v); }
            });
            load(v); return v.el;
        }
        function missing(detail) {
            close(false); if (choice) choice.close(null);
            const rows = (detail.steps || []).filter(step => summary(step.child_history));
            if (rows.length === 1) return open(rows[0], { run: { record: detail.run, steps: detail.steps } });
            const modal = core.modal(ed.root, { title: t('easydrag.ui.runs_title'), closeLabel: t('easydrag.ui.close'), body: '<p>' + esc(t('easydrag.ui.run_view_missing')) + '</p><div data-ed-child-run-list></div>', actions: [{ id: 'close', label: t('easydrag.ui.close') }] });
            choice = modal; modal.done.then(() => { if (choice === modal) choice = null; });
            const list = modal.body.querySelector('[data-ed-child-run-list]');
            list.innerHTML = rows.length ? rows.map((row, i) => '<button type="button" class="ed-btn" data-ed-child-parent="' + i + '">' + esc(row.node_key || row.node_id) + ' · #' + row.child_history.parent_seq + (row.attempt ? ' · ' + esc(t('easydrag.ui.child_history_attempt', { attempt: row.attempt })) : '') + ' · ' + esc(t('easydrag.ui.child_history_open')) + '</button>').join('') : '<p>' + esc(stateText(t, detail.child_history_state || detail.run?.child_history_state || 'legacy_unknown')) + '</p>';
            list.addEventListener('click', event => { const b = event.target.closest('[data-ed-child-parent]'); if (!disposed && modal.el.isConnected && b && rows[Number(b.dataset.edChildParent)]) { modal.close(null); open(rows[Number(b.dataset.edChildParent)], { run: { record: detail.run, steps: detail.steps } }); } });
            return modal;
        }
        bag.add(ed.bus.on('run', () => { if (active && active.ambient !== ed.run && (active.owner.simulation || source(ed).runId !== active.owner.runId)) close(false); }));
        const resize = () => { if (active?.host && live(active) && ed.root.getBoundingClientRect().width < 760) useModal(active); };
        bag.listen(window, 'resize', resize);
        const observer = new ResizeObserver(resize); observer.observe(ed.root); bag.add(() => observer.disconnect());
        return { open, missing, detach(host) { if (active && (host.contains(active.el) || active.originHost && host.contains(active.originHost))) close(false); }, dispose() { disposed = true; close(false); if (choice) choice.close(null); choice = null; bag.dispose(); } };
    }
    function missing(ed, detail) {
        if (ed.childHistory) return ed.childHistory.missing(detail);
        ed.ctx.notify({ title: ed.t('easydrag.ui.runs_title'), message: ed.t(detail.doc_missing === 'version_pruned' ? 'easydrag.ui.run_view_pruned' : 'easydrag.ui.run_view_missing', { revision: detail.run?.revision || '?' }), type: 'error' });
    }
    ED.childHistory = { create, summary, trace, fullValue, fits, actionMarkup, rememberSimulation, missing };
})();
