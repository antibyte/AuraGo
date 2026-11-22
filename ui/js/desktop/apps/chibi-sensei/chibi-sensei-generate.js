// Chibi Sensei generation helpers: the "Generate with Sensei" and "Clean with Sensei" dialogs, the end-of-job notes (with Undo and
// "Show new examples", or a Review button for a cleaning run) and the suggestion review. Progress comes from the job strip of the
// data step (bus event "job"); this module only watches for the end of the jobs it started. The review loads every suggestion
// and applies an explicit list of row ids, so removals stay opt-in. Classic script, publishes P.generate.
(function () {
    'use strict';

    const P = window.ChibiSenseiParts = window.ChibiSenseiParts || {};
    const UI_LANGS = ['cs', 'da', 'de', 'el', 'en', 'es', 'fr', 'hi', 'it', 'ja', 'nl', 'no', 'pl', 'pt', 'sv', 'zh'];
    const MAX_SEEDS = 10;
    const CLEAN_MAX_ROWS = 500;
    const PAGE = 100;
    const ACTION_IDS = ['keep', 'fix', 'drop'];
    const ACTION_KEYS = { keep: 'chibi_sensei.review.action_keep', fix: 'chibi_sensei.review.action_fix', drop: 'chibi_sensei.review.action_drop' };
    const ROLE_KEYS = { system: 'chibi_sensei.editor.role_system', user: 'chibi_sensei.editor.role_user', assistant: 'chibi_sensei.editor.role_assistant' };

    const isTerminal = job => !!job && (job.state === 'succeeded' || job.state === 'failed' || job.state === 'cancelled');

    function field(label, control, hint) {
        return P.el('label', { class: 'csn-field' }, P.el('span', { class: 'csn-label', text: label }), control, hint ? P.el('span', { class: 'csn-hint', text: hint }) : null);
    }

    function errorLine(name) {
        const node = P.el('p', { class: 'csn-error', role: 'alert', dataset: { csn: name } });
        node.hidden = true;
        return { node, show(text) { node.textContent = text || ''; node.hidden = !text; } };
    }

    function lockIf(inst, button, extra) {
        const off = inst.readonly || !!extra;
        button.disabled = off;
        button.title = inst.readonly ? inst.t('chibi_sensei.readonly_banner') : (extra || '');
    }

    function languageOptions() {
        const lang = String((typeof window !== 'undefined' && window.SYSTEM_LANG) || 'en').toLowerCase().slice(0, 2);
        let names = null;
        try { names = new Intl.DisplayNames([lang], { type: 'language' }); } catch (_) { names = null; }
        const options = UI_LANGS.map(code => {
            let label = code;
            try { label = (names && names.of(code)) || code; } catch (_) { label = code; }
            return { value: code, label };
        });
        return { options, value: UI_LANGS.indexOf(lang) >= 0 ? lang : 'en' };
    }

    // ---- per-window state ------------------------------------------------------------------------------------------------------

    // expected: ids of the jobs this window started; handled: ids already reported; entries: the notes under the job strip.
    function stateOf(inst) {
        let s = inst.parts.generate;
        if (!s) {
            s = inst.parts.generate = { expected: new Map(), handled: new Set(), entries: new Map(), reviewOpen: false };
            inst.bag.add(inst.bus.on('job', job => onJob(inst, job)));
        }
        return s;
    }

    function addEntry(inst, id, entry) {
        const s = stateOf(inst);
        s.entries.set(id, Object.assign({ projectId: inst.project ? inst.project.id : '' }, entry));
        inst.bus.emit('generate-notes');
    }

    function dropEntry(inst, id) {
        stateOf(inst).entries.delete(id);
        inst.bus.emit('generate-notes');
    }

    function reportBatch(inst, batchId, kind, count) {
        if (!batchId) return;
        inst.lastBatch = { batch_id: batchId, kind, count: Number(count) || 0 };
        inst.bus.emit('batch', inst.lastBatch);
    }

    function refreshRows(inst) {
        if (inst.parts.table) inst.parts.table.reload();
        P.table.afterWrite(inst);
    }

    function failText(inst, job) {
        return P.errors.describe(inst.t, P.errors.fromResponse(400, { code: job.error_code, message: job.error }));
    }

    function onJob(inst, job) {
        const s = inst.parts.generate;
        if (inst.disposed || !s || !job || !isTerminal(job) || s.handled.has(job.id)) return;
        const t = inst.t;
        const result = job.result || {};
        const mine = s.expected.has(job.id);
        if (job.kind === 'generate' && mine) {
            s.handled.add(job.id);
            if (job.state === 'failed') { addEntry(inst, job.id, { type: 'failed', text: failText(inst, job) }); return; }
            if (job.state !== 'succeeded') return;
            const added = Number(result.rows_added) || 0;
            let text = t('chibi_sensei.generate.done', { added: P.format.number(added), skipped: P.format.number(result.rows_skipped || 0) });
            if (Number(result.withheld) > 0) text += ' ' + t('chibi_sensei.generate.withheld', { count: P.format.number(result.withheld) });
            inst.notify(t('chibi_sensei.generate.done', { added: P.format.number(added), skipped: P.format.number(result.rows_skipped || 0) }), 'info');
            addEntry(inst, job.id, { type: 'generated', text, batchId: result.batch_id || '', added });
            reportBatch(inst, result.batch_id, 'generate', added);
            refreshRows(inst);
        } else if (job.kind === 'clean') {
            const stopped = job.state === 'failed' || job.state === 'cancelled';
            const saved = Number(result.suggestions);
            const partial = stopped && Number.isFinite(saved) && saved > 0;
            if (!mine && job.state !== 'succeeded' && !partial) return;
            s.handled.add(job.id);
            if (partial) {
                const note = mine ? (job.state === 'failed' ? failText(inst, job) : t('chibi_sensei.job.cancelled')) : '';
                addEntry(inst, job.id, { type: 'review', job, count: saved, note });
                return;
            }
            if (job.state === 'failed') { if (mine) addEntry(inst, job.id, { type: 'failed', text: failText(inst, job) }); return; }
            if (job.state !== 'succeeded') return;
            const count = Number(result.suggestions);
            if (Number.isFinite(count) && count <= 0) { inst.notify(t('chibi_sensei.review.empty'), 'info'); return; }
            addEntry(inst, job.id, { type: 'review', job, count: Number.isFinite(count) ? count : 0 });
            if (mine && !s.reviewOpen) openReview(inst, job);
        }
    }

    // A job that finished before its start answer arrived is picked up from the root's job map.
    function expect(inst, job) {
        const s = stateOf(inst);
        s.expected.set(job.id, job.kind);
        inst.applyJob(job);
        const known = inst.jobs.get(job.id);
        if (isTerminal(known)) onJob(inst, known);
    }

    // ---- notes under the job strip ---------------------------------------------------------------------------------------------

    function createResults(inst) {
        const { el } = P;
        const t = inst.t;
        const node = el('div', { class: 'csn-gen-notes', dataset: { csn: 'generate-notes' } });
        let disposed = false;

        async function undo(id, entry) {
            if (inst.readonly || !entry.batchId) return;
            try {
                const res = await inst.api.undo(inst.project.id, entry.batchId);
                if (disposed) return;
                if (inst.lastBatch && inst.lastBatch.batch_id === entry.batchId) { inst.lastBatch = null; inst.bus.emit('batch', null); }
                inst.notify(t('chibi_sensei.undo.done', { count: P.format.number(res.restored || 0) }), 'info');
                dropEntry(inst, id);
                refreshRows(inst);
            } catch (err) {
                if (!disposed && !(err && err.aborted)) inst.notify(inst.handleError(err), 'error');
            }
        }

        function button(name, key, onClick, quiet) {
            return el('button', { type: 'button', class: quiet ? 'csn-btn csn-btn-quiet' : 'csn-btn', dataset: { csn: name }, text: t(key), onClick });
        }

        function paint() {
            if (disposed) return;
            P.clear(node);
            const s = stateOf(inst);
            s.entries.forEach((entry, id) => {
                if (inst.project && entry.projectId !== inst.project.id) return;
                const row = el('div', { class: 'csn-gen-note', dataset: { csn: 'generate-note', type: entry.type } });
                if (entry.type === 'review') {
                    row.appendChild(el('span', { text: t('chibi_sensei.clean.ready', { count: P.format.number(entry.count) }) }));
                    if (entry.note) row.appendChild(el('span', { class: 'csn-muted', text: entry.note }));
                    row.appendChild(button('clean-review', 'chibi_sensei.clean.review', () => openReview(inst, entry.job)));
                } else if (entry.type === 'generated') {
                    row.appendChild(el('span', { text: entry.text }));
                    const show = button('generate-show', 'chibi_sensei.generate.show_new', () => { if (inst.parts.table && entry.batchId) inst.parts.table.setFilter({ batch: entry.batchId }); });
                    const undoBtn = button('generate-undo', 'chibi_sensei.undo.action', () => undo(id, entry), true);
                    lockIf(inst, undoBtn);
                    show.disabled = !entry.batchId;
                    undoBtn.disabled = undoBtn.disabled || !entry.batchId;
                    row.appendChild(show);
                    row.appendChild(undoBtn);
                } else if (entry.type === 'note') {
                    row.appendChild(el('span', { text: entry.text }));
                } else {
                    row.setAttribute('role', 'alert');
                    row.appendChild(el('span', { class: 'csn-error', text: entry.text }));
                }
                row.appendChild(button('generate-dismiss', 'chibi_sensei.editor.close', () => dropEntry(inst, id), true));
                node.appendChild(row);
            });
            node.hidden = !node.firstChild;
        }

        const offs = [inst.bus.on('generate-notes', paint), inst.bus.on('readonly', paint), inst.bus.on('project', paint)];
        paint();
        return { el: node, paint, dispose() { disposed = true; offs.forEach(off => { if (typeof off === 'function') off(); }); } };
    }

    // ---- generate dialog -------------------------------------------------------------------------------------------------------

    function openGenerate(inst) {
        if (inst.disposed || !inst.project || !P.need(inst, 'table')) return;
        const { el } = P;
        const t = inst.t;
        const meta = inst.meta || {};
        const available = !(meta.generator && meta.generator.available === false);
        const max = Math.max(1, Number(meta.limits && meta.limits.generator_max_rows_per_job) || 500);
        const dlg = P.table.makeDialog(inst, t('chibi_sensei.generate.title'), 'generate-dialog', true);
        const err = errorLine('generate-error');
        const count = el('input', { type: 'number', class: 'csn-input', min: '1', max: String(max), step: '1', dataset: { csn: 'generate-count' } });
        count.value = String(Math.min(50, max));
        const instructions = el('textarea', { class: 'csn-textarea', maxlength: '2000', rows: '4', placeholder: t('chibi_sensei.generate.instructions_placeholder'), dataset: { csn: 'generate-instructions' } });
        const langs = languageOptions();
        const language = P.table.selectEl(langs.options, langs.value, { dataset: { csn: 'generate-language' } });
        const sel = inst.parts.table ? inst.parts.table.selection() : null;
        const seedIds = sel && sel.spec && Array.isArray(sel.spec.ids) ? sel.spec.ids.slice(0, MAX_SEEDS) : [];
        const seedText = seedIds.length ? t('chibi_sensei.generate.seeds_count', { count: P.format.number(sel.spec.ids.length) }) : t('chibi_sensei.generate.seeds_none');
        const docChecks = [];
        const docs = el('div', { class: 'csn-checkgroup', dataset: { csn: 'generate-documents' } });
        (Array.isArray(inst.documents) ? inst.documents : []).forEach(doc => {
            const box = el('input', { type: 'checkbox', dataset: { csn: 'generate-document', id: doc.id } });
            docChecks.push({ box, id: doc.id });
            docs.appendChild(el('label', { class: 'csn-check-option' }, box, el('span', { text: doc.name })));
        });
        if (!docChecks.length) docs.appendChild(el('p', { class: 'csn-muted', text: t('chibi_sensei.generate.no_documents') }));
        const model = meta.generator && meta.generator.model ? t('chibi_sensei.generate.model', { model: meta.generator.model }) : t('chibi_sensei.generate.model_default');
        const start = el('button', { type: 'button', class: 'csn-btn csn-btn-primary', dataset: { csn: 'generate-start' }, text: t('chibi_sensei.generate.start') });
        lockIf(inst, start, available ? '' : t('chibi_sensei.generate.unavailable'));
        let busy = false;

        start.addEventListener('click', async () => {
            if (busy || inst.readonly || !available) return;
            const n = Math.floor(Number(count.value));
            if (!Number.isFinite(n) || n < 1 || n > max) { err.show(t('chibi_sensei.generate.count_hint', { max: P.format.number(max) })); count.focus(); return; }
            busy = true;
            start.disabled = true;
            err.show('');
            const spec = {
                count: n, seed_row_ids: seedIds, document_ids: docChecks.filter(d => d.box.checked).map(d => d.id),
                instructions: instructions.value.trim().slice(0, 2000), language: language.value
            };
            try {
                const job = await inst.api.generate(inst.project.id, spec);
                if (inst.disposed) return;
                expect(inst, job);
                inst.notify(t('chibi_sensei.generate.started'), 'info');
                dlg.close();
            } catch (e) {
                if (inst.disposed || (e && e.aborted)) return;
                err.show(inst.handleError(e));
                busy = false;
                lockIf(inst, start, available ? '' : t('chibi_sensei.generate.unavailable'));
            }
        });

        dlg.content.appendChild(field(t('chibi_sensei.generate.count'), count, t('chibi_sensei.generate.count_hint', { max: P.format.number(max) })));
        dlg.content.appendChild(field(t('chibi_sensei.generate.instructions'), instructions));
        dlg.content.appendChild(field(t('chibi_sensei.generate.language'), language));
        dlg.content.appendChild(el('div', { class: 'csn-field' }, el('span', { class: 'csn-label', text: t('chibi_sensei.generate.seeds') }), el('span', { class: 'csn-hint', text: seedText })));
        dlg.content.appendChild(el('div', { class: 'csn-field' }, el('span', { class: 'csn-label', text: t('chibi_sensei.generate.documents') }), docs));
        dlg.content.appendChild(el('p', { class: 'csn-muted', dataset: { csn: 'generate-model' }, text: model }));
        if (!available) dlg.content.appendChild(el('p', { class: 'csn-error', role: 'status', text: t('chibi_sensei.generate.unavailable') }));
        dlg.content.appendChild(err.node);
        dlg.content.appendChild(el('div', { class: 'csn-actions' }, start, el('button', { type: 'button', class: 'csn-btn csn-btn-quiet', dataset: { csn: 'generate-cancel' }, text: t('chibi_sensei.create.cancel'), onClick: dlg.close })));
        stateOf(inst);
        dlg.open();
        count.focus();
    }

    // ---- clean dialog ----------------------------------------------------------------------------------------------------------

    function openClean(inst) {
        if (inst.disposed || !inst.project || !P.need(inst, 'table')) return;
        const { el } = P;
        const t = inst.t;
        const available = !(inst.meta && inst.meta.generator && inst.meta.generator.available === false);
        const sel = inst.parts.table ? inst.parts.table.selection() : { spec: { ids: [] }, count: 0, mode: 'ids' };
        const total = Number(sel.count) || 0;
        const scope = total <= 0 ? t('chibi_sensei.clean.scope_none') : (total > CLEAN_MAX_ROWS ? t('chibi_sensei.clean.scope_too_many', { max: P.format.number(CLEAN_MAX_ROWS) }) : t('chibi_sensei.clean.scope', { count: P.format.number(total) }));
        const ok = total > 0 && total <= CLEAN_MAX_ROWS;
        const dlg = P.table.makeDialog(inst, t('chibi_sensei.clean.title'), 'clean-dialog');
        const err = errorLine('clean-error');
        const instructions = el('textarea', { class: 'csn-textarea', maxlength: '2000', rows: '4', placeholder: t('chibi_sensei.clean.instructions_placeholder'), dataset: { csn: 'clean-instructions' } });
        const start = el('button', { type: 'button', class: 'csn-btn csn-btn-primary', dataset: { csn: 'clean-start' }, text: t('chibi_sensei.clean.start') });
        const reason = !available ? t('chibi_sensei.generate.unavailable') : '';
        lockIf(inst, start, reason);
        if (!ok) start.disabled = true;
        let busy = false;

        start.addEventListener('click', async () => {
            if (busy || inst.readonly || !ok || !available) return;
            busy = true;
            start.disabled = true;
            err.show('');
            try {
                const job = await inst.api.clean(inst.project.id, { select: sel.spec, instructions: instructions.value.trim().slice(0, 2000) });
                if (inst.disposed) return;
                expect(inst, job);
                inst.notify(t('chibi_sensei.clean.started'), 'info');
                dlg.close();
            } catch (e) {
                if (inst.disposed || (e && e.aborted)) return;
                err.show(inst.handleError(e));
                busy = false;
                lockIf(inst, start, reason);
            }
        });

        dlg.content.appendChild(el('p', { class: ok ? 'csn-muted' : 'csn-error', role: 'status', dataset: { csn: 'clean-scope' }, text: scope }));
        dlg.content.appendChild(field(t('chibi_sensei.clean.instructions'), instructions));
        dlg.content.appendChild(err.node);
        dlg.content.appendChild(el('div', { class: 'csn-actions' }, start, el('button', { type: 'button', class: 'csn-btn csn-btn-quiet', dataset: { csn: 'clean-cancel' }, text: t('chibi_sensei.create.cancel'), onClick: dlg.close })));
        stateOf(inst);
        dlg.open();
    }

    // ---- review ----------------------------------------------------------------------------------------------------------------

    // The lines of one side of a suggestion: messages (role and text), or the single text of a free-recipe row.
    function sideLines(messages, text) {
        if (Array.isArray(messages) && messages.length) return messages.map(m => ({ role: m && m.role, content: String(m && m.content != null ? m.content : '') }));
        if (typeof text === 'string') return [{ role: '', content: text }];
        return [];
    }

    function sideNode(inst, lines, other, name) {
        const { el } = P;
        const col = el('div', { class: name, dataset: { csn: name } });
        lines.forEach((line, i) => {
            const changed = !other[i] || other[i].content !== line.content || other[i].role !== line.role;
            const roleKey = ROLE_KEYS[line.role];
            col.appendChild(el('div', { class: 'csn-diff-line', dataset: { changed: changed ? '1' : null } },
                el('span', { class: 'csn-diff-role', text: roleKey ? inst.t(roleKey) : inst.t('chibi_sensei.review.role_text') }),
                el('span', { class: 'csn-diff-text', text: line.content })));
        });
        return col;
    }

    async function loadSuggestions(inst, jobId) {
        const all = [];
        let total = Infinity;
        while (all.length < total && all.length < CLEAN_MAX_ROWS) {
            const page = await inst.api.suggestions(jobId, all.length, PAGE);
            const items = page && Array.isArray(page.suggestions) ? page.suggestions : [];
            total = Number(page && page.total) || 0;
            if (!items.length) break;
            items.forEach(item => all.push(item));
        }
        return all.slice(0, CLEAN_MAX_ROWS);
    }

    async function openReview(inst, job) {
        if (inst.disposed || !job || !P.need(inst, 'table')) return;
        const s = stateOf(inst);
        if (s.reviewOpen) return;
        s.reviewOpen = true;
        const { el } = P;
        const t = inst.t;
        const dlg = P.table.makeDialog(inst, t('chibi_sensei.review.title'), 'review-dialog', true);
        dlg.onClose = () => { s.reviewOpen = false; };
        const err = errorLine('review-error');
        const body = el('div', { class: 'csn-review-body', dataset: { csn: 'review-body' } }, el('p', { class: 'csn-muted', role: 'status', text: t('chibi_sensei.review.loading') }));
        const apply = el('button', { type: 'button', class: 'csn-btn csn-btn-primary', dataset: { csn: 'review-apply' } });
        const selectFixes = el('button', { type: 'button', class: 'csn-btn', dataset: { csn: 'review-select-fixes' }, text: t('chibi_sensei.review.select_fixes') });
        const selectNone = el('button', { type: 'button', class: 'csn-btn', dataset: { csn: 'review-select-none' }, text: t('chibi_sensei.review.select_none') });
        const close = el('button', { type: 'button', class: 'csn-btn csn-btn-quiet', dataset: { csn: 'review-close' }, text: t('chibi_sensei.editor.close'), onClick: dlg.close });
        const boxes = [];
        let busy = false;

        function chosen() { return boxes.filter(b => b.box.checked && !b.box.disabled).map(b => b.rowId); }
        function sync() {
            const n = chosen().length;
            apply.textContent = t('chibi_sensei.review.apply', { count: P.format.number(n) });
            lockIf(inst, apply);
            if (n === 0 || busy) apply.disabled = true;
            selectFixes.disabled = selectNone.disabled = !boxes.length;
        }

        function card(sug) {
            const action = ACTION_IDS.indexOf(sug.action) >= 0 ? sug.action : 'keep';
            const before = sideLines(sug.before && sug.before.messages, sug.before && sug.before.text);
            const after = sideLines(sug.messages, sug.text);
            const art = el('article', { class: 'csn-suggestion', dataset: { csn: 'suggestion', action, row: sug.row_id } });
            const head = el('header', { class: 'csn-suggestion-head' }, el('span', { class: 'csn-badge', text: t(ACTION_KEYS[action]) }),
                el('span', { class: 'csn-muted', text: t('chibi_sensei.review.reason', { reason: String(sug.reason || '') }) }));
            art.appendChild(head);
            if (action !== 'keep') {
                if (sug.applied) {
                    head.appendChild(el('span', { class: 'csn-badge', dataset: { csn: 'suggestion-applied' }, text: t('chibi_sensei.review.applied_badge') }));
                } else {
                    const box = el('input', { type: 'checkbox', dataset: { csn: 'suggestion-accept' }, onChange: sync });
                    box.checked = action === 'fix';
                    boxes.push({ box, rowId: sug.row_id, action });
                    head.appendChild(el('label', { class: 'csn-check-option' }, box, el('span', { text: t('chibi_sensei.review.accept') })));
                }
            }
            art.appendChild(el('div', { class: 'csn-diff' },
                el('div', { class: 'csn-diff-col' }, el('h4', { class: 'csn-subhead', text: t('chibi_sensei.review.before') }), sideNode(inst, before, after, 'csn-diff-before')),
                el('div', { class: 'csn-diff-col' }, el('h4', { class: 'csn-subhead', text: t('chibi_sensei.review.after') }), sideNode(inst, after, before, 'csn-diff-after'))));
            return art;
        }

        selectFixes.addEventListener('click', () => { boxes.forEach(b => { b.box.checked = b.action === 'fix'; }); sync(); });
        selectNone.addEventListener('click', () => { boxes.forEach(b => { b.box.checked = false; }); sync(); });
        apply.addEventListener('click', async () => {
            const ids = chosen();
            if (busy || inst.readonly || !ids.length) return;
            busy = true;
            sync();
            err.show('');
            try {
                const res = await inst.api.applySuggestions(job.id, ids);
                if (inst.disposed) return;
                const applied = Number(res && res.applied) || 0;
                inst.notify(t('chibi_sensei.review.applied', { count: P.format.number(applied) }), 'info');
                reportBatch(inst, res && res.batch_id, 'clean', applied);
                refreshRows(inst);
                const skipped = res && Array.isArray(res.skipped) ? res.skipped.length : 0;
                if (skipped > 0) {
                    const note = t('chibi_sensei.review.skipped', { count: P.format.number(skipped) });
                    addEntry(inst, 'applied-' + job.id, { type: 'note', text: note });
                    dlg.content.insertBefore(el('p', { class: 'csn-muted', role: 'status', dataset: { csn: 'review-skipped' }, text: note }), body);
                    sync();
                    return;
                }
                dlg.close();
            } catch (e) {
                if (inst.disposed || (e && e.aborted)) return;
                err.show(inst.handleError(e));
                busy = false;
                sync();
            }
        });

        dlg.content.appendChild(el('p', { class: 'csn-muted', text: t('chibi_sensei.review.intro') }));
        if (job.state === 'failed' || job.state === 'cancelled') dlg.content.appendChild(el('p', { class: 'csn-muted', role: 'status', dataset: { csn: 'review-partial' }, text: t('chibi_sensei.review.partial') }));
        dlg.content.appendChild(body);
        dlg.content.appendChild(err.node);
        dlg.content.appendChild(el('div', { class: 'csn-actions' }, selectFixes, selectNone, el('span', { class: 'csn-spacer' }), apply, close));
        sync();
        dlg.open();

        let items = [];
        try { items = await loadSuggestions(inst, job.id); } catch (e) {
            if (inst.disposed || (e && e.aborted)) return;
            P.clear(body);
            err.show(inst.handleError(e));
            return;
        }
        if (inst.disposed) return;
        P.clear(body);
        if (!items.length) {
            body.appendChild(el('p', { class: 'csn-muted', role: 'status', text: t('chibi_sensei.review.empty') }));
            sync();
            return;
        }
        items.forEach(sug => body.appendChild(card(sug)));
        sync();
    }

    P.generate = { openGenerate, openClean, openReview, createResults };
})();
