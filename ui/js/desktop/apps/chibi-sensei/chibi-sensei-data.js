// Chibi Sensei data step: the toolbar, the paged table (from chibi-sensei-table.js), the split footer, the job strip, the checks list
// that lives in the guidance panel, and the dialogs for file imports with a mapping preview, source documents and export. The
// checks run (runChecks) is what the primary action of the data step calls. Long operations report through the bus event "job"
// (the root feeds it from SSE and from the 2.5 s poll). Classic script, publishes P.data.
(function () {
    'use strict';

    const P = window.ChibiSenseiParts = window.ChibiSenseiParts || {};
    const FINISHED_MS = 6000;
    const LEVEL_ORDER = { block: 0, warn: 1, info: 2, ok: 3 };
    const LEVEL_KEYS = {
        block: 'chibi_sensei.checks.level_block', warn: 'chibi_sensei.checks.level_warn', info: 'chibi_sensei.checks.level_info', ok: 'chibi_sensei.checks.level_ok'
    };
    const JOB_KIND_KEYS = {
        generate: 'chibi_sensei.job.kind_generate', clean: 'chibi_sensei.job.kind_clean', import_file: 'chibi_sensei.job.kind_import_file', export: 'chibi_sensei.job.kind_export'
    };
    const JOB_STATE_KEYS = { succeeded: 'chibi_sensei.job.succeeded', failed: 'chibi_sensei.job.failed', cancelled: 'chibi_sensei.job.cancelled' };
    const SHAPE_IDS = ['messages', 'conversations', 'instruction', 'prompt_completion', 'text', 'text_label', 'columns'];
    const SHAPE_KEYS = {
        messages: 'chibi_sensei.import.shape_messages', conversations: 'chibi_sensei.import.shape_conversations', instruction: 'chibi_sensei.import.shape_instruction',
        prompt_completion: 'chibi_sensei.import.shape_prompt_completion', text: 'chibi_sensei.import.shape_text', text_label: 'chibi_sensei.import.shape_text_label',
        columns: 'chibi_sensei.import.shape_columns'
    };
    const COLUMN_IDS = ['system', 'user', 'assistant', 'text', 'label'];
    const COLUMN_KEYS = {
        system: 'chibi_sensei.import.col_system', user: 'chibi_sensei.import.col_user', assistant: 'chibi_sensei.import.col_assistant',
        text: 'chibi_sensei.import.col_text', label: 'chibi_sensei.import.col_label'
    };
    const FORMAT_KEYS = {
        auto: 'chibi_sensei.import.format_auto', jsonl: 'chibi_sensei.import.format_jsonl', csv: 'chibi_sensei.import.format_csv', text: 'chibi_sensei.import.format_text'
    };
    const WHICH_IDS = ['all', 'train', 'val', 'eval'];
    const WHICH_KEYS = {
        all: 'chibi_sensei.export.which_all', train: 'chibi_sensei.export.which_train', val: 'chibi_sensei.export.which_val', eval: 'chibi_sensei.export.which_eval'
    };

    const isActive = job => !!job && (job.state === 'queued' || job.state === 'running');

    // The path a shell file dialog answered with, or '' when it was cancelled.
    function pickedPath(result) {
        if (!result || result.canceled) return '';
        return typeof result === 'string' ? result : String(result.path || '');
    }

    function parentFolder(path) {
        const parts = String(path || '').replace(/\\/g, '/').split('/').filter(Boolean);
        parts.pop();
        return parts.join('/');
    }

    function safeName(name) {
        const clean = String(name || '').replace(/[^A-Za-z0-9._-]+/g, '-').replace(/^-+|-+$/g, '');
        return clean || 'dataset';
    }

    // ---- checks ----------------------------------------------------------------------------------------------------------------

    // Runs the dataset checks. The result replaces inst.checks; the table is read again because the row flags changed.
    async function runChecks(inst) {
        if (inst.disposed || !inst.project || inst.checksRunning || inst.readonly) return;
        const seq = inst.projectSeq;
        inst.checksRunning = true;
        inst.checksError = '';
        inst.bus.emit('checks', { running: true });
        inst.recompute();
        try {
            const result = await inst.api.runChecks(inst.project.id);
            if (inst.disposed || seq !== inst.projectSeq) return;
            inst.checks = result;
            inst.checksStale = false;
        } catch (err) {
            if (inst.disposed || seq !== inst.projectSeq || (err && err.aborted)) return;
            inst.checksError = inst.handleError(err);
        }
        inst.checksRunning = false;
        inst.bus.emit('checks', { result: inst.checks, stale: inst.checksStale });
        inst.recompute();
        if (inst.parts.table) inst.parts.table.reload();
    }

    function createChecksList(inst) {
        const { el } = P;
        const t = inst.t;
        const summary = el('p', { class: 'csn-muted', dataset: { csn: 'checks-summary' } });
        const note = el('p', { class: 'csn-muted', dataset: { csn: 'checks-note' } });
        const error = el('p', { class: 'csn-error', role: 'alert', dataset: { csn: 'checks-error' } });
        const list = el('ul', { class: 'csn-checks', dataset: { csn: 'checks-list' } });
        const node = el('section', { class: 'csn-checklist', ariaLabel: t('chibi_sensei.checks.title'), dataset: { csn: 'checks' } },
            el('h3', { class: 'csn-panel-title', text: t('chibi_sensei.checks.title') }), summary, note, error, list);
        let disposed = false;

        async function dedupe() {
            if (inst.readonly) return;
            error.hidden = true;
            try {
                const res = await inst.api.dedupe(inst.project.id);
                if (disposed || inst.disposed) return;
                inst.lastBatch = { batch_id: res.batch_id, kind: 'dedupe', count: res.removed };
                inst.bus.emit('batch', inst.lastBatch);
                inst.notify(t('chibi_sensei.checks.deduped', { count: P.format.number(res.removed || 0) }), 'info');
                if (inst.parts.table) inst.parts.table.reload();
                P.table.afterWrite(inst);
            } catch (err) {
                if (disposed || (err && err.aborted)) return;
                error.textContent = inst.handleError(err);
                error.hidden = false;
            }
        }

        function item(check) {
            const level = LEVEL_ORDER[check.level] != null ? check.level : 'info';
            const li = el('li', { class: 'csn-check', dataset: { csn: 'check', id: check.id, level } },
                el('span', { class: 'csn-check-level', text: t(LEVEL_KEYS[level]) }),
                el('span', { class: 'csn-check-text', text: P.checkText(t, check) }));
            const actions = el('div', { class: 'csn-actions' });
            if (Number(check.row_count) > 0) {
                actions.appendChild(el('button', { type: 'button', class: 'csn-btn csn-btn-quiet', dataset: { csn: 'check-show', id: check.id }, text: t('chibi_sensei.checks.show_rows'), onClick: () => { if (inst.parts.table) inst.parts.table.setFilter({ flag: check.id }); } }));
            }
            if (check.fix === 'dedupe') {
                const fix = el('button', { type: 'button', class: 'csn-btn', dataset: { csn: 'check-dedupe' }, text: t('chibi_sensei.checks.fix_dedupe'), onClick: dedupe });
                fix.disabled = inst.readonly;
                fix.title = inst.readonly ? t('chibi_sensei.readonly_banner') : '';
                actions.appendChild(fix);
            }
            if (actions.firstChild) li.appendChild(actions);
            return li;
        }

        function paint() {
            if (disposed) return;
            const c = inst.checks && Array.isArray(inst.checks.checks) ? inst.checks : null;
            const s = c && c.summary ? c.summary : null;
            const tokens = s && s.est_tokens ? s.est_tokens : {};
            summary.textContent = s ? t('chibi_sensei.checks.summary', { rows: P.format.number(s.rows), p50: P.format.number(tokens.p50), p95: P.format.number(tokens.p95) }) : '';
            summary.hidden = !s;
            note.textContent = !c ? t('chibi_sensei.checks.never') : (inst.checksStale ? t('chibi_sensei.checks.stale') : '');
            note.hidden = !note.textContent;
            error.textContent = inst.checksError || '';
            error.hidden = !inst.checksError;
            P.clear(list);
            const found = c ? c.checks.filter(x => x && x.level !== 'ok').sort((a, b) => (LEVEL_ORDER[a.level] != null ? LEVEL_ORDER[a.level] : 2) - (LEVEL_ORDER[b.level] != null ? LEVEL_ORDER[b.level] : 2)) : [];
            found.forEach(check => list.appendChild(item(check)));
            if (c && !found.length && !inst.checksStale) list.appendChild(el('li', { class: 'csn-check', dataset: { csn: 'checks-ok', level: 'ok' } }, el('span', { class: 'csn-check-text', text: t('chibi_sensei.checks.all_ok') })));
        }

        const offs = ['checks', 'rows', 'project', 'readonly'].map(name => inst.bus.on(name, paint));
        paint();
        return { el: node, paint, dispose() { disposed = true; offs.forEach(off => { if (typeof off === 'function') off(); }); } };
    }

    // ---- job strip -------------------------------------------------------------------------------------------------------------

    function createJobStrip(inst) {
        const { el } = P;
        const t = inst.t;
        const node = el('div', { class: 'csn-jobs', ariaLive: 'polite', ariaLabel: t('chibi_sensei.job.strip_label'), dataset: { csn: 'jobs' } });
        const finished = new Map();
        const timers = [];
        let disposed = false;

        async function cancel(job) {
            try {
                const next = await inst.api.cancelJob(job.id);
                if (!disposed) inst.applyJob(next);
            } catch (err) {
                if (!disposed && !(err && err.aborted)) inst.notify(inst.handleError(err), 'error');
            }
        }

        function row(job) {
            const done = job.state in JOB_STATE_KEYS;
            const total = Number(job.progress && job.progress.total) || 0;
            const value = Number(job.progress && job.progress.done) || 0;
            const bar = el('progress', { class: 'csn-progressbar', max: total > 0 ? String(total) : false, value: total > 0 ? String(Math.min(value, total)) : false, ariaLabel: t(JOB_KIND_KEYS[job.kind] || JOB_KIND_KEYS.generate) });
            const line = el('div', { class: 'csn-job', dataset: { csn: 'job', kind: job.kind, state: job.state, phase: job.progress && job.progress.phase ? job.progress.phase : '' } },
                el('span', { class: 'csn-job-kind', text: t(JOB_KIND_KEYS[job.kind] || JOB_KIND_KEYS.generate) }),
                done ? el('span', { class: 'csn-job-result', text: t(JOB_STATE_KEYS[job.state]) }) : bar,
                !done && total > 0 ? el('span', { class: 'csn-muted', text: t('chibi_sensei.job.progress', { done: P.format.number(value), total: P.format.number(total) }) }) : null);
            if (!done) {
                const stop = el('button', { type: 'button', class: 'csn-btn csn-btn-quiet', dataset: { csn: 'job-cancel' }, text: t('chibi_sensei.job.cancel'), onClick: () => cancel(job) });
                stop.disabled = inst.readonly;
                stop.title = inst.readonly ? t('chibi_sensei.readonly_banner') : '';
                line.appendChild(stop);
            }
            return line;
        }

        function paint() {
            if (disposed) return;
            const now = Date.now();
            P.clear(node);
            Array.from(inst.jobs.values()).forEach(job => {
                if (isActive(job) || (finished.has(job.id) && finished.get(job.id) > now)) node.appendChild(row(job));
            });
            node.hidden = !node.firstChild;
        }

        const offJob = inst.bus.on('job', job => {
            if (disposed) return;
            if (job && !isActive(job) && !finished.has(job.id)) {
                finished.set(job.id, Date.now() + FINISHED_MS);
                timers.push(setTimeout(paint, FINISHED_MS + 50));
            }
            paint();
        });
        const offRo = inst.bus.on('readonly', paint);
        paint();
        return {
            el: node, paint,
            dispose() { disposed = true; timers.forEach(clearTimeout); offJob(); offRo(); }
        };
    }

    // ---- dialogs: import -------------------------------------------------------------------------------------------------------

    function field(label, control, hint) {
        return P.el('label', { class: 'csn-field' }, P.el('span', { class: 'csn-label', text: label }), control, hint ? P.el('span', { class: 'csn-hint', text: hint }) : null);
    }

    function openImportDialog(inst, ui, path, first) {
        const { el } = P;
        const t = inst.t;
        const pid = inst.project.id;
        const free = inst.project.recipe === 'free';
        const dlg = P.table.makeDialog(inst, t('chibi_sensei.import.title'), 'import-dialog', true);
        const error = el('p', { class: 'csn-error', role: 'alert', dataset: { csn: 'import-error' } });
        error.hidden = true;
        const body = el('div', { class: 'csn-import-body', dataset: { csn: 'import-body' } });
        let preview = first;
        let ctl = null;
        let busy = false;
        const importBtn = el('button', { type: 'button', class: 'csn-btn csn-btn-primary', dataset: { csn: 'import-start' }, text: t('chibi_sensei.import.start') });
        const againBtn = el('button', { type: 'button', class: 'csn-btn', dataset: { csn: 'import-preview' }, text: t('chibi_sensei.import.preview_again') });
        const cancelBtn = el('button', { type: 'button', class: 'csn-btn csn-btn-quiet', dataset: { csn: 'import-cancel' }, text: t('chibi_sensei.create.cancel'), onClick: dlg.close });
        const showError = text => { error.textContent = text || ''; error.hidden = !text; };

        function readMapping() {
            const mapping = { shape: ctl.shape.value, system_text: ctl.system.value };
            COLUMN_IDS.forEach(id => { mapping[id] = ctl.cols[id].value; });
            return mapping;
        }
        const resolvedFormat = () => (ctl.format.value === 'auto' ? preview.format : ctl.format.value);

        function render() {
            P.clear(body);
            const m = preview.mapping || {};
            const columns = Array.isArray(preview.columns) ? preview.columns : [];
            const formats = ['auto', 'jsonl', 'csv'].concat(free ? ['text'] : []);
            const format = P.table.selectEl(formats.map(id => ({ value: id, label: t(FORMAT_KEYS[id]) })), ctl && formats.indexOf(ctl.format.value) >= 0 ? ctl.format.value : 'auto', { dataset: { csn: 'import-format' } });
            const shape = P.table.selectEl(SHAPE_IDS.map(id => ({ value: id, label: t(SHAPE_KEYS[id]) })), SHAPE_IDS.indexOf(m.shape) >= 0 ? m.shape : 'messages', { dataset: { csn: 'import-shape' } });
            const cols = {};
            const colFields = COLUMN_IDS.map(id => {
                const options = [{ value: '', label: t('chibi_sensei.import.col_none') }].concat(columns.map(c => ({ value: c, label: c })));
                cols[id] = P.table.selectEl(options, columns.indexOf(m[id]) >= 0 ? m[id] : '', { dataset: { csn: 'import-col-' + id } });
                return field(t(COLUMN_KEYS[id]), cols[id]);
            });
            const system = el('input', { type: 'text', class: 'csn-input', dataset: { csn: 'import-system-text' } });
            system.value = m.system_text || '';
            ctl = { format, shape, cols, system };
            body.appendChild(el('div', { class: 'csn-grid2' }, field(t('chibi_sensei.import.format'), format), field(t('chibi_sensei.import.shape'), shape)));
            if (columns.length) {
                body.appendChild(el('p', { class: 'csn-muted', text: t('chibi_sensei.import.columns', { columns: columns.join(', ') }) }));
                body.appendChild(el('div', { class: 'csn-grid2' }, colFields));
            }
            body.appendChild(field(t('chibi_sensei.import.system_text'), system, t('chibi_sensei.import.system_text_hint')));
            body.appendChild(el('h4', { class: 'csn-subhead', text: t('chibi_sensei.import.sample_title') }));
            const sample = Array.isArray(preview.sample) ? preview.sample.slice(0, 5) : [];
            const table = el('table', { class: 'csn-table csn-sample', dataset: { csn: 'import-sample', recipe: inst.project.recipe || '' } });
            sample.forEach(r => {
                const pv = P.table.rowPreview(Object.assign({ kind: typeof r.text === 'string' ? 'text' : 'chat' }, r), 120);
                table.appendChild(el('tr', { dataset: { csn: 'import-sample-row' } }, el('td', { class: 'csn-cell', text: pv.prompt }), el('td', { class: 'csn-cell', text: pv.answer })));
            });
            body.appendChild(table);
            const issues = Array.isArray(preview.issues) ? preview.issues : [];
            if (issues.length) {
                body.appendChild(el('h4', { class: 'csn-subhead', text: t('chibi_sensei.import.issues_title') }));
                const list = el('ul', { class: 'csn-issues', dataset: { csn: 'import-issues' } });
                issues.slice(0, 20).forEach(issue => list.appendChild(el('li', { text: t('chibi_sensei.import.issue_line', { line: P.format.number(issue.line), message: P.errors.knownKey(t, issue.message_key, 'chibi_sensei.import.issue_generic') }) })));
                body.appendChild(list);
            }
            importBtn.disabled = busy || sample.length === 0;
        }

        async function update() {
            if (busy) return;
            busy = true;
            againBtn.disabled = importBtn.disabled = true;
            showError('');
            try {
                const next = await inst.api.importPreview(pid, { workspace_path: path, format: ctl.format.value, mapping: readMapping() });
                if (inst.disposed) return;
                preview = next;
            } catch (err) {
                if (!inst.disposed && !(err && err.aborted)) showError(inst.handleError(err));
            } finally {
                busy = false;
                againBtn.disabled = false;
                if (!inst.disposed) render();
            }
        }

        async function start() {
            if (busy || inst.readonly) return;
            busy = true;
            importBtn.disabled = true;
            showError('');
            try {
                const job = await inst.api.startImport(pid, { workspace_path: path, format: resolvedFormat(), mapping: readMapping() });
                if (inst.disposed) return;
                ui.expect(job.id);
                inst.applyJob(job);
                inst.notify(t('chibi_sensei.import.started'), 'info');
                dlg.close();
            } catch (err) {
                if (!inst.disposed && !(err && err.aborted)) showError(inst.handleError(err));
            } finally {
                busy = false;
                if (!inst.disposed) importBtn.disabled = false;
            }
        }

        importBtn.addEventListener('click', start);
        againBtn.addEventListener('click', update);
        render();
        dlg.content.appendChild(body);
        dlg.content.appendChild(error);
        dlg.content.appendChild(el('div', { class: 'csn-actions' }, againBtn, importBtn, cancelBtn));
        dlg.open();
    }

    // ---- dialogs: documents and export -----------------------------------------------------------------------------------------

    async function openDocuments(inst) {
        const { el } = P;
        const t = inst.t;
        const pid = inst.project.id;
        const dlg = P.table.makeDialog(inst, t('chibi_sensei.documents.title'), 'documents-dialog');
        const error = el('p', { class: 'csn-error', role: 'alert', dataset: { csn: 'documents-error' } });
        error.hidden = true;
        const list = el('ul', { class: 'csn-history', dataset: { csn: 'documents-list' } });
        const showError = text => { error.textContent = text || ''; error.hidden = !text; };
        const fail = err => { if (!inst.disposed && !(err && err.aborted)) showError(inst.handleError(err)); };

        function paint() {
            P.clear(list);
            inst.documents.forEach(doc => {
                const remove = el('button', { type: 'button', class: 'csn-btn csn-btn-quiet', dataset: { csn: 'document-remove', id: doc.id }, ariaLabel: t('chibi_sensei.documents.remove', { name: doc.name }), text: t('chibi_sensei.bulk.delete'), onClick: async () => {
                    try {
                        await inst.api.removeDocument(pid, doc.id);
                        if (inst.disposed) return;
                        inst.documents = inst.documents.filter(d => d.id !== doc.id);
                        inst.bus.emit('documents', inst.documents);
                        inst.notify(t('chibi_sensei.documents.removed', { name: doc.name }), 'info');
                        paint();
                    } catch (err) { fail(err); }
                } });
                remove.disabled = inst.readonly;
                list.appendChild(el('li', { class: 'csn-history-item', dataset: { csn: 'document' } },
                    el('span', { class: 'csn-history-kind', text: doc.name }),
                    el('span', { class: 'csn-muted', text: t('chibi_sensei.documents.chars', { count: P.format.number(doc.chars || 0) }) }),
                    remove));
            });
            empty.hidden = inst.documents.length > 0;
            list.hidden = inst.documents.length === 0;
        }

        async function add() {
            if (inst.readonly || typeof inst.ctx.openFileDialog !== 'function') return;
            showError('');
            let path = '';
            try { path = pickedPath(await inst.ctx.openFileDialog({ title: t('chibi_sensei.documents.pick_title'), filters: [{ label: t('chibi_sensei.documents.filter'), extensions: ['txt', 'md', 'markdown'] }], initialPath: 'Documents' })); } catch (_) { path = ''; }
            if (!path || inst.disposed) return;
            try {
                const doc = await inst.api.addDocument(pid, path);
                if (inst.disposed) return;
                inst.documents = inst.documents.concat([doc]);
                inst.bus.emit('documents', inst.documents);
                inst.notify(t('chibi_sensei.documents.added', { name: doc.name }), 'info');
                paint();
            } catch (err) { fail(err); }
        }

        const empty = el('p', { class: 'csn-muted', text: t('chibi_sensei.documents.empty') });
        const addBtn = el('button', { type: 'button', class: 'csn-btn csn-btn-primary', dataset: { csn: 'document-add' }, text: t('chibi_sensei.documents.add'), onClick: add });
        addBtn.disabled = inst.readonly;
        addBtn.title = inst.readonly ? t('chibi_sensei.readonly_banner') : '';
        dlg.content.appendChild(el('p', { class: 'csn-muted', text: t('chibi_sensei.documents.intro') }));
        dlg.content.appendChild(list);
        dlg.content.appendChild(empty);
        dlg.content.appendChild(error);
        dlg.content.appendChild(el('div', { class: 'csn-actions' }, addBtn, el('button', { type: 'button', class: 'csn-btn csn-btn-quiet', dataset: { csn: 'documents-close' }, text: t('chibi_sensei.editor.close'), onClick: dlg.close })));
        try { inst.documents = await inst.api.listDocuments(pid); } catch (err) { fail(err); }
        if (inst.disposed) return;
        paint();
        dlg.open();
    }

    function openExport(inst, ui) {
        const { el } = P;
        const t = inst.t;
        const dlg = P.table.makeDialog(inst, t('chibi_sensei.export.title'), 'export-dialog');
        const error = el('p', { class: 'csn-error', role: 'alert', dataset: { csn: 'export-error' } });
        error.hidden = true;
        const which = P.table.selectEl(WHICH_IDS.map(id => ({ value: id, label: t(WHICH_KEYS[id]) })), 'all', { dataset: { csn: 'export-which' } });
        const next = el('button', { type: 'button', class: 'csn-btn csn-btn-primary', dataset: { csn: 'export-next' }, text: t('chibi_sensei.export.next') });
        next.disabled = inst.readonly;
        next.addEventListener('click', async () => {
            if (inst.readonly || typeof inst.ctx.saveFileDialog !== 'function') return;
            error.hidden = true;
            let path = '';
            try {
                path = pickedPath(await inst.ctx.saveFileDialog({ title: t('chibi_sensei.export.save_title'), defaultName: safeName(inst.project.name) + '-' + which.value + '.jsonl', initialPath: 'Documents', filters: [{ label: t('chibi_sensei.export.filter'), extensions: ['jsonl'] }] }));
            } catch (_) { path = ''; }
            if (!path || inst.disposed) return;
            next.disabled = true;
            try {
                const job = await inst.api.startExport(inst.project.id, { workspace_path: path, which: which.value, overwrite: true });
                if (inst.disposed) return;
                ui.expect(job.id);
                inst.applyJob(job);
                inst.notify(t('chibi_sensei.export.started'), 'info');
                dlg.close();
            } catch (err) {
                if (inst.disposed || (err && err.aborted)) return;
                error.textContent = inst.handleError(err);
                error.hidden = false;
                next.disabled = false;
            }
        });
        dlg.content.appendChild(field(t('chibi_sensei.export.which'), which));
        dlg.content.appendChild(error);
        dlg.content.appendChild(el('div', { class: 'csn-actions' }, next, el('button', { type: 'button', class: 'csn-btn csn-btn-quiet', dataset: { csn: 'export-cancel' }, text: t('chibi_sensei.create.cancel'), onClick: dlg.close })));
        dlg.open();
    }

    // ---- the step --------------------------------------------------------------------------------------------------------------

    function create(inst) {
        const { el, icon } = P;
        const t = inst.t;
        if (!P.need(inst, 'table')) return null;
        const table = P.table.create(inst);
        inst.parts.table = table;
        const checks = createChecksList(inst);
        const strip = createJobStrip(inst);
        if (inst.parts.panelView) inst.parts.panelView.setExtra(checks.el);
        let disposed = false;
        const expected = new Set();
        const reported = new Set();

        const error = el('p', { class: 'csn-error', role: 'alert', dataset: { csn: 'data-error' } });
        error.hidden = true;
        const exportNote = el('p', { class: 'csn-export-note', role: 'status', dataset: { csn: 'export-done' } });
        exportNote.hidden = true;
        const showError = text => { error.textContent = text || ''; error.hidden = !text; };

        const ui = { expect: id => expected.add(id) };
        const button = (name, iconName, key, onClick) => el('button', { type: 'button', class: 'csn-btn', dataset: { csn: name }, onClick }, icon(iconName), el('span', { text: t(key) }));
        const newBtn = button('new-example', 'plus', 'chibi_sensei.data.new_example', () => table.newExample());
        const importBtn = button('import', 'upload', 'chibi_sensei.data.import', () => startImport());
        const docsBtn = button('documents', 'file', 'chibi_sensei.data.documents', () => openDocuments(inst).catch(() => {}));
        const genBtn = button('generate', 'sparkle', 'chibi_sensei.data.generate', () => { if (P.generate && P.generate.openGenerate) P.generate.openGenerate(inst); });
        const cleanBtn = button('clean', 'broom', 'chibi_sensei.data.clean', () => { if (P.generate && P.generate.openClean) P.generate.openClean(inst); });
        const exportBtn = button('export', 'export', 'chibi_sensei.data.export', () => openExport(inst, ui));
        const hfBtn = el('button', { type: 'button', class: 'csn-btn', disabled: true, title: t('chibi_sensei.data.hf_later'), dataset: { csn: 'hf' } }, el('span', { text: t('chibi_sensei.data.hf') }), el('span', { class: 'csn-badge', text: t('chibi_sensei.data.later_badge') }));
        const toolbar = el('div', { class: 'csn-toolbar', role: 'toolbar', ariaLabel: t('chibi_sensei.data.toolbar_label'), dataset: { csn: 'toolbar' } }, newBtn, importBtn, docsBtn, genBtn, cleanBtn, exportBtn, hfBtn);

        function paintToolbar() {
            const ro = inst.readonly;
            const reason = ro ? t('chibi_sensei.readonly_banner') : '';
            [newBtn, importBtn, docsBtn, exportBtn].forEach(b => { b.disabled = ro; b.title = reason; });
            const hasGen = !!(P.generate && P.generate.openGenerate);
            const generatorOff = !!(inst.meta && inst.meta.generator && inst.meta.generator.available === false);
            [genBtn, cleanBtn].forEach(b => {
                b.disabled = ro || !hasGen || generatorOff;
                b.title = ro ? reason : (!hasGen ? t('chibi_sensei.primary.later') : '');
            });
        }

        async function startImport() {
            if (inst.readonly || typeof inst.ctx.openFileDialog !== 'function') return;
            showError('');
            let path = '';
            try {
                const filters = [{ label: t('chibi_sensei.import.filter_data'), extensions: ['jsonl', 'csv'] }];
                if (inst.project.recipe === 'free') filters.push({ label: t('chibi_sensei.documents.filter'), extensions: ['txt', 'md', 'markdown'] });
                path = pickedPath(await inst.ctx.openFileDialog({ title: t('chibi_sensei.import.pick_title'), filters, initialPath: 'Documents' }));
            } catch (_) { path = ''; }
            if (!path || disposed) return;
            try {
                const preview = await inst.api.importPreview(inst.project.id, { workspace_path: path, format: 'auto' });
                if (!disposed) openImportDialog(inst, ui, path, preview);
            } catch (err) {
                if (!disposed && !(err && err.aborted)) showError(inst.handleError(err));
            }
        }

        // The end of an import or an export that this window started: a note, a table reload and an undo for imports.
        function jobDone(job) {
            if (disposed || !job || isActive(job) || reported.has(job.id)) return;
            if (job.kind !== 'import_file' && job.kind !== 'export') return;
            if (!expected.has(job.id) && job.state !== 'succeeded') return;
            reported.add(job.id);
            const result = job.result || {};
            if (job.state === 'failed') {
                const text = P.errors.describe(t, P.errors.fromResponse(400, { code: job.error_code, message: job.error }));
                showError(text);
                return;
            }
            if (job.state !== 'succeeded') return;
            if (job.kind === 'import_file') {
                inst.notify(t('chibi_sensei.import.done', { added: P.format.number(result.rows_added || 0), skipped: P.format.number(result.rows_skipped || 0) }), 'info');
                if (result.batch_id) { inst.lastBatch = { batch_id: result.batch_id, kind: 'import', count: result.rows_added || 0 }; inst.bus.emit('batch', inst.lastBatch); }
                table.reload();
                P.table.afterWrite(inst);
            } else if (expected.has(job.id)) {
                const path = String(result.path || '');
                P.clear(exportNote);
                exportNote.appendChild(el('span', { text: t('chibi_sensei.export.done', { path }) }));
                if (typeof inst.ctx.openApp === 'function') {
                    exportNote.appendChild(el('button', { type: 'button', class: 'csn-btn csn-btn-quiet', dataset: { csn: 'export-show' }, text: t('chibi_sensei.export.show'), onClick: () => inst.ctx.openApp('files', { path: parentFolder(path) }) }));
                }
                exportNote.hidden = false;
            }
        }

        // footer
        const examples = el('span', { class: 'csn-footer-total', dataset: { csn: 'rows-total' } });
        const counts = el('span', { class: 'csn-footer-counts', dataset: { csn: 'split-counts' } });
        const reseed = el('button', { type: 'button', class: 'csn-btn csn-btn-quiet', dataset: { csn: 'reseed' }, onClick: async () => {
            if (inst.readonly) return;
            showError('');
            try {
                const res = await inst.api.reseed(inst.project.id);
                if (disposed) return;
                inst.split = res;
                inst.bus.emit('split', res);
                inst.notify(t('chibi_sensei.split.reseeded'), 'info');
                table.reload();
                P.table.afterWrite(inst);
            } catch (err) { if (!disposed && !(err && err.aborted)) showError(inst.handleError(err)); }
        } }, icon('refresh'), el('span', { text: t('chibi_sensei.split.reseed') }));
        const footer = el('div', { class: 'csn-footer', dataset: { csn: 'footer' } }, examples, counts, el('span', { class: 'csn-spacer' }), reseed);

        function paintFooter() {
            if (disposed) return;
            const c = inst.split && inst.split.counts ? inst.split.counts : null;
            examples.textContent = t('chibi_sensei.footer.examples', { count: P.format.number(inst.project ? inst.project.rows : 0) });
            counts.textContent = c ? [t('chibi_sensei.footer.train', { count: P.format.number(c.train) }), t('chibi_sensei.footer.val', { count: P.format.number(c.val) }), t('chibi_sensei.footer.eval', { count: P.format.number(c.eval) }), t('chibi_sensei.footer.excluded', { count: P.format.number(c.excluded) })].join(' · ') : '';
            reseed.disabled = inst.readonly;
            reseed.title = inst.readonly ? t('chibi_sensei.readonly_banner') : '';
        }

        const offs = [
            inst.bus.on('job', jobDone),
            inst.bus.on('project', paintFooter),
            inst.bus.on('split', paintFooter),
            inst.bus.on('readonly', () => { paintToolbar(); paintFooter(); })
        ];
        paintToolbar();
        paintFooter();
        if (!inst.split) {
            const seq = inst.projectSeq;
            inst.api.getSplit(inst.project.id).then(split => { if (!disposed && seq === inst.projectSeq) { inst.split = split; paintFooter(); } }).catch(() => {});
        }

        const node = el('div', { class: 'csn-data', dataset: { csn: 'data' } }, toolbar, error, exportNote, strip.el, table.el, footer);
        return {
            el: node,
            focus() { table.focus(); },
            dispose() {
                disposed = true;
                offs.forEach(off => { if (typeof off === 'function') off(); });
                checks.dispose();
                strip.dispose();
                table.dispose();
                if (inst.parts.table === table) inst.parts.table = null;
                if (inst.parts.panelView) inst.parts.panelView.setExtra(null);
            }
        };
    }

    P.data = { create, runChecks, createChecksList, createJobStrip };
})();
