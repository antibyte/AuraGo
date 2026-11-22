// Chibi Sensei table: the pure table reducer (paging, filter, selection, side-sheet editor state; no DOM at load time, so a Node
// test can load it in a vm) and the DOM built from it: the paged row table, the selection and bulk bar, the undo bar with the
// history dialog and the side-sheet editor. Also offers two small helpers the data step shares: afterWrite (refresh the project
// numbers and mark the checks stale) and makeDialog (a native dialog with focus return). Classic script, publishes P.table.
(function () {
    'use strict';

    const P = window.ChibiSenseiParts = window.ChibiSenseiParts || {};

    // ---- pure state ------------------------------------------------------------------------------------------------------------

    const PAGE_SIZE = 50, BULK_TOKEN_LIMIT = 50;
    const FILTER_KEYS = ['q', 'split', 'source', 'flag', 'label', 'batch'];
    const emptyFilter = () => ({ q: '', split: '', source: '', flag: '', label: '', batch: '' });
    const closedEditor = () => ({ state: 'closed', rowId: null, draft: null, baseRev: 0, serverRow: null, issues: [], dirty: false, error: null });
    function initialState() {
        return { offset: 0, limit: PAGE_SIZE, total: 0, rows: [], dataRev: 0, filter: emptyFilter(), sort: 'seq', dir: 'asc',
            selection: { mode: 'ids', ids: [] }, status: 'idle', error: null, needsReload: true, editor: closedEditor() };
    }
    function lastOffset(total, limit) { const n = Number(total) || 0, l = Number(limit) || PAGE_SIZE; return n <= 0 ? 0 : Math.floor((n - 1) / l) * l; }
    function blankDraft(kind) { return kind === 'text' ? { text: '', split: 'auto', tags: [] } : { messages: [{ role: 'user', content: '' }, { role: 'assistant', content: '' }], split: 'auto', tags: [] }; }
    function draftFromRow(row) {
        const base = { split: row.split || 'auto', tags: Array.isArray(row.tags) ? row.tags.slice() : [] };
        return row.kind === 'text' ? Object.assign(base, { text: String(row.text || '') }) : Object.assign(base, { messages: (row.messages || []).map(m => ({ role: m.role, content: String(m.content || '') })) });
    }
    function sortedIds(list) { return Array.from(new Set(list.filter(n => Number.isInteger(n)))).sort((a, b) => a - b); }
    function reduce(state, action) {
        const s = state || initialState(), a = action || {}, set = patch => Object.assign({}, s, patch);
        switch (a.type) {
        case 'load_start': return set({ status: 'loading', error: null, needsReload: false });
        case 'load_ok': {
            const total = Math.max(0, Number(a.total) || 0), last = lastOffset(total, s.limit), rev = Number(a.data_rev) || s.dataRev;
            if (s.offset > last) return set({ status: 'idle', total, rows: [], offset: last, dataRev: rev, needsReload: true });
            return set({ status: 'idle', total, rows: Array.isArray(a.rows) ? a.rows : [], dataRev: rev, needsReload: false });
        }
        case 'load_error': return set({ status: 'error', error: a.error || null });
        case 'page': return set({ offset: Math.min(Math.max(0, Math.floor((Number(a.offset) || 0) / s.limit) * s.limit), lastOffset(s.total, s.limit)), needsReload: true });
        case 'filter': {
            const next = Object.assign({}, s.filter);
            FILTER_KEYS.forEach(k => { if (a.patch && Object.prototype.hasOwnProperty.call(a.patch, k)) next[k] = String(a.patch[k] == null ? '' : a.patch[k]); });
            return set({ filter: next, offset: 0, selection: { mode: 'ids', ids: [] }, needsReload: true });
        }
        case 'sort': return set({ sort: a.sort === 'est_tokens' ? 'est_tokens' : 'seq', dir: a.dir === 'desc' ? 'desc' : 'asc', offset: 0, needsReload: true });
        case 'select_toggle': {
            if (s.selection.mode !== 'ids') return s;
            const has = s.selection.ids.indexOf(a.id) >= 0;
            return set({ selection: { mode: 'ids', ids: sortedIds(has ? s.selection.ids.filter(x => x !== a.id) : s.selection.ids.concat([a.id])) } });
        }
        case 'select_page': {
            const page = s.rows.map(r => r.id);
            return set({ selection: { mode: 'ids', ids: sortedIds(a.on ? s.selection.ids.concat(page) : s.selection.ids.filter(x => page.indexOf(x) < 0)) } });
        }
        case 'select_filter': return set({ selection: { mode: 'filter', ids: [] } });
        case 'select_clear': return set({ selection: { mode: 'ids', ids: [] } });
        case 'remote_rev': return Number(a.data_rev) > s.dataRev ? set({ needsReload: true }) : s;
        case 'edit_open': return set({ editor: Object.assign(closedEditor(), { state: 'editing', rowId: a.row.id, draft: draftFromRow(a.row), baseRev: a.row.rev }) });
        case 'edit_new': return set({ editor: Object.assign(closedEditor(), { state: 'editing', rowId: 'new', draft: blankDraft(a.kind) }) });
        case 'edit_change': return s.editor.state === 'closed' || s.editor.state === 'saving' ? s : set({ editor: Object.assign({}, s.editor, { draft: a.draft, dirty: true, state: s.editor.state === 'conflict' ? 'conflict' : 'editing', issues: [] }) });
        case 'edit_save_start': return set({ editor: Object.assign({}, s.editor, { state: 'saving', error: null }) });
        case 'edit_save_ok': {
            // The editor closes only when it is still the one that was saved; a late reply must not close another row's editor.
            const isNew = a.rowId === 'new' || (a.rowId === undefined && s.editor.rowId === 'new');
            const savedId = isNew ? 'new' : (a.rowId !== undefined ? a.rowId : (a.row ? a.row.id : null));
            const rows = a.row && !isNew ? s.rows.map(r => (r.id === a.row.id ? a.row : r)) : s.rows;
            return set({ rows, dataRev: Number(a.data_rev) || s.dataRev, needsReload: isNew, editor: s.editor.rowId === savedId ? closedEditor() : s.editor });
        }
        case 'edit_conflict': return set({ editor: Object.assign({}, s.editor, { state: 'conflict', serverRow: a.row || null }) });
        case 'edit_take_server': return s.editor.serverRow ? set({ editor: Object.assign({}, s.editor, { state: 'editing', draft: draftFromRow(s.editor.serverRow), baseRev: s.editor.serverRow.rev, serverRow: null, dirty: false }) }) : s;
        case 'edit_keep_mine': return s.editor.serverRow ? set({ editor: Object.assign({}, s.editor, { state: 'editing', baseRev: s.editor.serverRow.rev, serverRow: null }) }) : s;
        case 'edit_invalid': return set({ editor: Object.assign({}, s.editor, { state: 'invalid', issues: Array.isArray(a.issues) ? a.issues : [] }) });
        case 'edit_failed': return set({ editor: Object.assign({}, s.editor, { state: 'editing', error: a.error || null }) });
        case 'edit_close': return set({ editor: closedEditor() });
        default: return s;
        }
    }
    function selectionSpec(s) { return s.selection.mode === 'filter' ? { filter: Object.assign({}, s.filter) } : { ids: s.selection.ids.slice() }; }
    function selectionCount(s) { return s.selection.mode === 'filter' ? s.total : s.selection.ids.length; }
    function needsDeletePreview(s) { return s.selection.mode === 'filter' || s.selection.ids.length > BULK_TOKEN_LIMIT; }
    function query(s) { const q = { offset: s.offset, limit: s.limit, sort: s.sort, dir: s.dir }; FILTER_KEYS.forEach(k => { if (s.filter[k]) q[k] = s.filter[k]; }); return q; }
    // The cells of a row: the last user turn before the last answer, and that answer; text rows show their text.
    function rowPreview(row, max) {
        const cap = v => { const t = String(v || '').replace(/\s+/g, ' ').trim(); return t.length > (max || 240) ? t.slice(0, (max || 240) - 1) + '…' : t; };
        if (!row || row.kind === 'text') return { prompt: cap(row && row.text), answer: '' };
        const m = Array.isArray(row.messages) ? row.messages : [];
        let a = -1; for (let i = m.length - 1; i >= 0; i--) if (m[i].role === 'assistant') { a = i; break; }
        let u = -1; for (let i = (a < 0 ? m.length : a) - 1; i >= 0; i--) if (m[i].role === 'user') { u = i; break; }
        return { prompt: cap(u >= 0 ? m[u].content : ''), answer: cap(a >= 0 ? m[a].content : '') };
    }

    // ---- literal key maps ------------------------------------------------------------------------------------------------------

    const FLAG_KEYS = {
        duplicate: 'chibi_sensei.flag.duplicate', too_long: 'chibi_sensei.flag.too_long', structure: 'chibi_sensei.flag.structure',
        short_answer: 'chibi_sensei.flag.short_answer', conflict: 'chibi_sensei.flag.conflict',
        json_invalid: 'chibi_sensei.flag.json_invalid', invalid_json: 'chibi_sensei.flag.json_invalid'
    };
    const SPLIT_IDS = ['auto', 'train', 'eval', 'exclude'];
    const SPLIT_KEYS = {
        auto: 'chibi_sensei.split.auto', train: 'chibi_sensei.split.train', eval: 'chibi_sensei.split.eval', exclude: 'chibi_sensei.split.exclude'
    };
    const SOURCE_IDS = ['manual', 'file', 'generated', 'cleaned', 'hf', 'sensei'];
    const SOURCE_KEYS = {
        manual: 'chibi_sensei.source.manual', file: 'chibi_sensei.source.file', generated: 'chibi_sensei.source.generated',
        cleaned: 'chibi_sensei.source.cleaned', hf: 'chibi_sensei.source.hf', sensei: 'chibi_sensei.source.sensei'
    };
    const ROLE_IDS = ['system', 'user', 'assistant'];
    const ROLE_KEYS = { system: 'chibi_sensei.editor.role_system', user: 'chibi_sensei.editor.role_user', assistant: 'chibi_sensei.editor.role_assistant' };
    const CHECK_NAME_KEYS = {
        count_min: 'chibi_sensei.checks.name_count_min', structure: 'chibi_sensei.checks.name_structure', short_answer: 'chibi_sensei.checks.name_short_answer',
        duplicates: 'chibi_sensei.checks.name_duplicates', conflicts: 'chibi_sensei.checks.name_conflicts', too_long: 'chibi_sensei.checks.name_too_long',
        label_balance: 'chibi_sensei.checks.name_label_balance', json_valid: 'chibi_sensei.checks.name_json_valid', split_sizes: 'chibi_sensei.checks.name_split_sizes',
        language_fit: 'chibi_sensei.checks.name_language_fit'
    };
    const KIND_KEYS = {
        bulk_delete: 'chibi_sensei.history.kind_bulk_delete', bulk_edit: 'chibi_sensei.history.kind_bulk_edit', edit: 'chibi_sensei.history.kind_edit',
        dedupe: 'chibi_sensei.history.kind_dedupe', import: 'chibi_sensei.history.kind_import', generate: 'chibi_sensei.history.kind_generate',
        clean: 'chibi_sensei.history.kind_clean', add: 'chibi_sensei.history.kind_add'
    };
    const kindKey = kind => KIND_KEYS[kind] || KIND_KEYS.bulk_edit;

    // ---- shared helpers --------------------------------------------------------------------------------------------------------

    // After a write: the checks are stale, the project numbers and the split are read again.
    function afterWrite(inst) {
        if (inst.disposed || !inst.project) return Promise.resolve();
        inst.checksStale = true;
        inst.bus.emit('checks', { result: inst.checks, stale: true });
        inst.recompute();
        const seq = inst.projectSeq, id = inst.project.id;
        return Promise.all([inst.api.getProject(id).catch(() => null), inst.api.getSplit(id).catch(() => null)]).then(([project, split]) => {
            if (inst.disposed || seq !== inst.projectSeq) return;
            if (project) { inst.project = project; inst.bus.emit('project', project); }
            if (split) { inst.split = split; inst.bus.emit('split', split); }
            inst.recompute();
        });
    }

    // A native dialog with a heading, appended to the window root. Focus goes back to the opener when it closes.
    function makeDialog(inst, titleText, name, wide) {
        const { el } = P;
        const opener = document.activeElement;
        const content = el('div', { class: 'csn-dialog-form' }, el('h3', { text: titleText }));
        const node = el('dialog', { class: 'csn-dialog', ariaLabel: titleText, dataset: { csn: name, wide: wide ? '1' : '0' } }, content);
        const handle = {
            node, content, onClose: null,
            close() { if (node.open) node.close(); },
            open() { (inst.root || inst.host || document.body).appendChild(node); node.showModal(); }
        };
        node.addEventListener('close', () => {
            node.remove();
            if (handle.onClose) handle.onClose();
            if (opener && typeof opener.focus === 'function' && opener.isConnected) opener.focus();
        });
        inst.bag.add(() => { node.remove(); });
        return handle;
    }

    function selectEl(options, value, attrs) {
        const sel = P.el('select', Object.assign({ class: 'csn-select' }, attrs || {}),
            options.map(o => P.el('option', { value: o.value, text: o.label })));
        sel.value = value;
        return sel;
    }

    function fullText(row) {
        const text = row.kind === 'text' ? String(row.text || '') : (row.messages || []).map(m => m.role + ': ' + m.content).join('\n');
        return text.length > 2000 ? text.slice(0, 1999) + '…' : text;
    }

    // ---- the table view --------------------------------------------------------------------------------------------------------

    function create(inst) {
        const { el, icon } = P;
        const t = inst.t;
        const api = inst.api;
        const pid = inst.project.id;
        const free = inst.project.recipe === 'free';
        let state = initialState();
        let seq = 0;
        let disposed = false;
        let opener = null;
        let loadShown = false;
        const offs = [];
        const rowViews = new Map();

        const errorNote = el('p', { class: 'csn-error', role: 'alert', dataset: { csn: 'table-error' } });
        errorNote.hidden = true;
        const showError = text => { errorNote.textContent = text || ''; errorNote.hidden = !text; };
        const fail = err => { if (!disposed && !(err && err.aborted)) showError(inst.handleError(err)); };

        // head
        const search = el('input', { type: 'search', class: 'csn-input', ariaLabel: t('chibi_sensei.table.search'), placeholder: t('chibi_sensei.table.search'), dataset: { csn: 'search' } });
        const anyOption = { value: '', label: t('chibi_sensei.table.filter_any') };
        const splitFilter = selectEl([anyOption].concat(SPLIT_IDS.map(id => ({ value: id, label: t(SPLIT_KEYS[id]) }))), '', { ariaLabel: t('chibi_sensei.table.filter_split'), dataset: { csn: 'filter-split' } });
        const sourceFilter = selectEl([anyOption].concat(SOURCE_IDS.map(id => ({ value: id, label: t(SOURCE_KEYS[id]) }))), '', { ariaLabel: t('chibi_sensei.table.filter_source'), dataset: { csn: 'filter-source' } });
        const flagFilter = selectEl([anyOption].concat(P.CHECK_IDS.map(id => ({ value: id, label: t(CHECK_NAME_KEYS[id]) }))), '', { ariaLabel: t('chibi_sensei.table.filter_flag'), dataset: { csn: 'filter-flag' } });
        const sortSelect = selectEl([{ value: 'seq', label: t('chibi_sensei.table.sort_seq') }, { value: 'est_tokens', label: t('chibi_sensei.table.sort_tokens') }], 'seq', { ariaLabel: t('chibi_sensei.table.sort_label'), dataset: { csn: 'sort' } });
        const sendSearch = P.debounce(() => dispatch({ type: 'filter', patch: { q: search.value.trim() } }), 300);
        search.addEventListener('input', sendSearch);
        splitFilter.addEventListener('change', () => dispatch({ type: 'filter', patch: { split: splitFilter.value } }));
        sourceFilter.addEventListener('change', () => dispatch({ type: 'filter', patch: { source: sourceFilter.value } }));
        flagFilter.addEventListener('change', () => dispatch({ type: 'filter', patch: { flag: flagFilter.value } }));
        sortSelect.addEventListener('change', () => dispatch({ type: 'sort', sort: sortSelect.value, dir: sortSelect.value === 'est_tokens' ? 'desc' : 'asc' }));
        const head = el('div', { class: 'csn-table-head' }, search, splitFilter, sourceFilter, flagFilter, sortSelect);
        const batchChip = el('div', { class: 'csn-chip csn-batch-chip', role: 'status', dataset: { csn: 'batch-chip' } },
            el('span', { text: t('chibi_sensei.table.batch_filter') }),
            el('button', { type: 'button', class: 'csn-btn csn-btn-quiet', dataset: { csn: 'batch-clear' }, text: t('chibi_sensei.table.batch_clear'), onClick: () => dispatch({ type: 'filter', patch: { batch: '' } }) }));
        batchChip.hidden = true;

        // selection bar and undo bar
        const selCount = el('span', { class: 'csn-selcount', dataset: { csn: 'sel-count' } });
        const selAll = el('button', { type: 'button', class: 'csn-btn csn-btn-quiet', dataset: { csn: 'select-filter' }, onClick: () => dispatch({ type: 'select_filter' }) });
        const selClear = el('button', { type: 'button', class: 'csn-btn csn-btn-quiet', dataset: { csn: 'select-clear' }, text: t('chibi_sensei.table.clear_selection'), onClick: () => dispatch({ type: 'select_clear' }) });
        const setSplit = selectEl([{ value: '', label: t('chibi_sensei.bulk.set_split') }].concat(SPLIT_IDS.map(id => ({ value: id, label: t(SPLIT_KEYS[id]) }))), '', { ariaLabel: t('chibi_sensei.bulk.set_split'), dataset: { csn: 'bulk-split' } });
        setSplit.addEventListener('change', () => { const v = setSplit.value; setSplit.value = ''; if (v) runBulk('set_split', v); });
        const addTag = el('button', { type: 'button', class: 'csn-btn', dataset: { csn: 'bulk-add-tag' }, text: t('chibi_sensei.bulk.add_tag'), onClick: () => tagBulk('add_tag') });
        const removeTag = el('button', { type: 'button', class: 'csn-btn', dataset: { csn: 'bulk-remove-tag' }, text: t('chibi_sensei.bulk.remove_tag'), onClick: () => tagBulk('remove_tag') });
        const excludeBtn = el('button', { type: 'button', class: 'csn-btn', dataset: { csn: 'bulk-exclude' }, text: t('chibi_sensei.bulk.exclude'), onClick: () => runBulk('exclude', '') });
        const deleteBtn = el('button', { type: 'button', class: 'csn-btn', dataset: { csn: 'bulk-delete' }, onClick: () => deleteSelection() }, icon('trash'), el('span', { text: t('chibi_sensei.bulk.delete') }));
        const writeButtons = [setSplit, addTag, removeTag, excludeBtn, deleteBtn];
        const selbar = el('div', { class: 'csn-selbar', role: 'region', ariaLabel: t('chibi_sensei.table.label'), dataset: { csn: 'selbar' } }, selCount, selAll, selClear, setSplit, addTag, removeTag, excludeBtn, deleteBtn);
        selbar.hidden = true;
        const undoText = el('span', { class: 'csn-undo-text' });
        const undoBtn = el('button', { type: 'button', class: 'csn-btn csn-btn-quiet', dataset: { csn: 'undo' }, onClick: () => undoBatch(inst.lastBatch && inst.lastBatch.batch_id) }, icon('undo'), el('span', { text: t('chibi_sensei.undo.action') }));
        const historyBtn = el('button', { type: 'button', class: 'csn-btn csn-btn-quiet', dataset: { csn: 'history' }, onClick: () => openHistory() }, icon('history'), el('span', { text: t('chibi_sensei.undo.history') }));
        const undoBar = el('div', { class: 'csn-undo', role: 'status', dataset: { csn: 'undo-bar' } }, undoText, undoBtn, historyBtn);

        // table
        const headCheck = el('input', { type: 'checkbox', ariaLabel: t('chibi_sensei.table.select_page'), dataset: { csn: 'select-page' } });
        headCheck.addEventListener('change', () => dispatch({ type: 'select_page', on: headCheck.checked }));
        const tbody = el('tbody', {});
        const table = el('table', { class: 'csn-table', ariaLabel: t('chibi_sensei.table.label'), dataset: { csn: 'table', recipe: inst.project.recipe || '' } },
            el('thead', {}, el('tr', {},
                el('th', { class: 'csn-col-check', scope: 'col' }, headCheck),
                el('th', { class: 'csn-col-index', scope: 'col', text: t('chibi_sensei.table.col_index') }),
                el('th', { scope: 'col', text: t(free ? 'chibi_sensei.table.col_text' : 'chibi_sensei.table.col_prompt') }),
                el('th', { class: 'csn-col-answer', scope: 'col', text: t('chibi_sensei.table.col_answer') }),
                el('th', { class: 'csn-col-source', scope: 'col', text: t('chibi_sensei.table.col_source') }))),
            tbody);
        const emptyNote = el('p', { class: 'csn-muted csn-table-empty', dataset: { csn: 'table-empty' } });
        emptyNote.hidden = true;
        const wrap = el('div', { class: 'csn-table-wrap' }, table, emptyNote);
        const prev = el('button', { type: 'button', class: 'csn-btn csn-btn-icon', ariaLabel: t('chibi_sensei.table.prev'), title: t('chibi_sensei.table.prev'), dataset: { csn: 'page-prev' }, onClick: () => dispatch({ type: 'page', offset: state.offset - state.limit }) }, icon('left'));
        const next = el('button', { type: 'button', class: 'csn-btn csn-btn-icon', ariaLabel: t('chibi_sensei.table.next'), title: t('chibi_sensei.table.next'), dataset: { csn: 'page-next' }, onClick: () => dispatch({ type: 'page', offset: state.offset + state.limit }) }, icon('right'));
        const pageText = el('span', { class: 'csn-muted', dataset: { csn: 'page-text' } });
        const pager = el('div', { class: 'csn-pager' }, prev, pageText, next);

        // editor sheet
        const sheetTitle = el('h3', { class: 'csn-sheet-title' });
        const sheetClose = el('button', { type: 'button', class: 'csn-btn csn-btn-icon csn-btn-quiet', ariaLabel: t('chibi_sensei.editor.close'), title: t('chibi_sensei.editor.close'), dataset: { csn: 'editor-close' }, onClick: () => closeEditor() }, icon('close'));
        const sheetBody = el('div', { class: 'csn-sheet-body' });
        const sheetNotice = el('div', { class: 'csn-sheet-notice', dataset: { csn: 'editor-notice' } });
        const saveBtn = el('button', { type: 'button', class: 'csn-btn csn-btn-primary', dataset: { csn: 'editor-save' }, text: t('chibi_sensei.editor.save'), onClick: () => saveEditor() });
        const cancelBtn = el('button', { type: 'button', class: 'csn-btn', dataset: { csn: 'editor-cancel' }, text: t('chibi_sensei.editor.cancel'), onClick: () => closeEditor() });
        const delBtn = el('button', { type: 'button', class: 'csn-btn', dataset: { csn: 'editor-delete' }, onClick: () => deleteFromEditor() }, icon('trash'), el('span', { text: t('chibi_sensei.editor.delete') }));
        const sheet = el('aside', { class: 'csn-sheet', role: 'dialog', ariaModal: 'false', dataset: { csn: 'editor' } },
            el('div', { class: 'csn-sheet-head' }, sheetTitle, sheetClose), sheetBody, sheetNotice,
            el('div', { class: 'csn-sheet-foot' }, saveBtn, cancelBtn, el('span', { class: 'csn-spacer' }), delBtn));
        sheet.hidden = true;
        sheet.addEventListener('keydown', event => { if (event.key === 'Escape') { event.stopPropagation(); closeEditor(); } });

        const area = el('div', { class: 'csn-table-area' }, el('div', { class: 'csn-table-main' }, head, batchChip, selbar, undoBar, errorNote, wrap, pager), sheet);
        const node = el('section', { class: 'csn-table-view', ariaLabel: t('chibi_sensei.table.label'), dataset: { csn: 'table-view' } }, area);

        // ---- state plumbing ----
        const REBUILD = ['edit_open', 'edit_new', 'edit_close', 'edit_save_ok', 'edit_take_server', 'edit_keep_mine'];

        function dispatch(action, quiet) {
            if (disposed) return;
            const before = state;
            state = reduce(state, action);
            if (state === before) return;
            inst.editorDirty = state.editor.state !== 'closed' && state.editor.dirty;
            if (action.type === 'filter' || action.type === 'sort') { paintFilterControls(); }
            if (!quiet) {
                if (before.rows !== state.rows || before.total !== state.total || before.status !== state.status || before.error !== state.error) paintRows();
                paintPager();
                if (before.selection !== state.selection) paintSelection();
            }
            if (before.editor !== state.editor) {
                if (REBUILD.indexOf(action.type) >= 0) paintEditor();
                paintNotice();
            }
            if (state.needsReload) reload();
        }

        function paintFilterControls() {
            if (search.value.trim() !== state.filter.q) search.value = state.filter.q;
            splitFilter.value = state.filter.split;
            sourceFilter.value = state.filter.source;
            flagFilter.value = state.filter.flag;
            sortSelect.value = state.sort;
            batchChip.hidden = !state.filter.batch;
        }

        async function reload() {
            if (disposed) return;
            const mine = ++seq;
            dispatch({ type: 'load_start' });
            try {
                const res = await api.listRows(pid, query(state));
                if (disposed || mine !== seq) return;
                dispatch({ type: 'load_ok', rows: res.rows, total: res.total, data_rev: res.data_rev });
            } catch (err) {
                if (disposed || mine !== seq || (err && err.aborted)) return;
                dispatch({ type: 'load_error', error: inst.handleError(err) });
            }
        }

        // ---- table paint ----
        const anyFilter = () => FILTER_KEYS.some(k => !!state.filter[k]);

        function flagChips(row) {
            const flags = Array.isArray(row.flags) ? row.flags : [];
            return flags.map(id => el('span', { class: 'csn-flag', dataset: { csn: 'flag', flag: id }, text: t(FLAG_KEYS[id] || 'chibi_sensei.flag.other') }));
        }

        function buildRow(row) {
            const n = row.seq;
            const cb = el('input', { type: 'checkbox', ariaLabel: t('chibi_sensei.table.select_row', { n }), dataset: { csn: 'select-row', id: row.id } });
            cb.addEventListener('click', event => event.stopPropagation());
            cb.addEventListener('change', () => dispatch({ type: 'select_toggle', id: row.id }));
            const editBtn = el('button', { type: 'button', class: 'csn-btn csn-btn-quiet csn-row-edit', ariaLabel: t('chibi_sensei.table.edit_row', { n }), dataset: { csn: 'edit-row' }, onClick: event => { event.stopPropagation(); openEditor(row); } }, String(n));
            const pv = rowPreview(row);
            const splitNote = row.split && row.split !== 'auto' ? el('span', { class: 'csn-badge', text: t(SPLIT_KEYS[row.split] || SPLIT_KEYS.auto) }) : null;
            const tr = el('tr', { class: 'csn-row', title: fullText(row), dataset: { csn: 'row', flagged: Array.isArray(row.flags) && row.flags.length ? '1' : '0', id: row.id } },
                el('td', { class: 'csn-col-check' }, cb),
                el('td', { class: 'csn-col-index' }, editBtn),
                el('td', { class: 'csn-cell', text: pv.prompt }),
                el('td', { class: 'csn-cell csn-col-answer', text: pv.answer }),
                el('td', { class: 'csn-col-source' }, el('span', { text: SOURCE_KEYS[row.source] ? t(SOURCE_KEYS[row.source]) : String(row.source || '') }), splitNote, flagChips(row)));
            tr.addEventListener('click', () => openEditor(row));
            rowViews.set(row.id, { tr, cb });
            return tr;
        }

        function paintRows() {
            P.clear(tbody);
            rowViews.clear();
            state.rows.forEach(row => tbody.appendChild(buildRow(row)));
            emptyNote.hidden = state.rows.length > 0 || state.status === 'loading';
            emptyNote.textContent = t(anyFilter() ? 'chibi_sensei.table.empty_filtered' : 'chibi_sensei.table.empty');
            table.setAttribute('aria-busy', state.status === 'loading' ? 'true' : 'false');
            if (state.status === 'error') { showError(state.error); loadShown = true; } else if (loadShown && state.status === 'idle') { showError(''); loadShown = false; }
            paintSelection();
            paintReadonly();
        }

        function paintPager() {
            const from = state.rows.length ? state.offset + 1 : 0, to = state.offset + state.rows.length;
            pageText.textContent = t('chibi_sensei.table.page', { from: P.format.number(from), to: P.format.number(to), total: P.format.number(state.total) });
            prev.disabled = state.offset <= 0;
            next.disabled = state.offset + state.limit >= state.total;
        }

        function paintSelection() {
            const all = state.selection.mode === 'filter';
            const ids = state.selection.ids;
            let on = 0;
            state.rows.forEach(row => {
                const view = rowViews.get(row.id);
                if (!view) return;
                const picked = all || ids.indexOf(row.id) >= 0;
                if (picked) on++;
                view.cb.checked = picked;
                view.cb.disabled = all;
                view.tr.setAttribute('aria-selected', picked ? 'true' : 'false');
            });
            headCheck.checked = state.rows.length > 0 && on === state.rows.length;
            headCheck.indeterminate = on > 0 && on < state.rows.length;
            headCheck.disabled = all;
            const count = selectionCount(state);
            selbar.hidden = count <= 0;
            selCount.textContent = t('chibi_sensei.table.selected', { count: P.format.number(count) });
            selAll.hidden = all || state.total <= count;
            selAll.textContent = t('chibi_sensei.table.select_all_matching', { count: P.format.number(state.total) });
        }

        function paintReadonly() {
            const ro = inst.readonly;
            const reason = ro ? t('chibi_sensei.readonly_banner') : '';
            writeButtons.forEach(b => { b.disabled = ro; b.title = reason; });
            [saveBtn, delBtn].forEach(b => { b.disabled = ro || state.editor.state === 'saving'; b.title = reason; });
            delBtn.hidden = state.editor.rowId === 'new';
            sheet.querySelectorAll('[data-csn=msg-content], [data-csn=msg-role], [data-csn=editor-text], [data-csn=editor-split], [data-csn=editor-tags], [data-csn=add-turn], [data-csn=add-system]').forEach(c => { c.disabled = ro; });
            const turns = sheet.querySelectorAll('[data-csn=msg]').length;
            sheet.querySelectorAll('[data-csn=msg-remove]').forEach(c => { c.disabled = ro || turns <= 1; });
        }

        function paintUndo() {
            const batch = inst.lastBatch;
            undoBar.hidden = !batch || !batch.batch_id;
            if (undoBar.hidden) return;
            undoText.textContent = t(kindKey(batch.kind)) + ' · ' + t('chibi_sensei.history.rows', { count: P.format.number(batch.count || 0) });
            undoBtn.disabled = inst.readonly;
            undoBtn.title = inst.readonly ? t('chibi_sensei.readonly_banner') : '';
        }

        // ---- bulk ----
        function afterBulk(res, kind, noteKey) {
            const count = res && res.affected != null ? res.affected : selectionCount(state);
            inst.lastBatch = { batch_id: res && res.batch_id, kind, count };
            inst.bus.emit('batch', inst.lastBatch);
            inst.notify(t(noteKey, { count: P.format.number(count) }), 'info');
            dispatch({ type: 'select_clear' });
            reload();
            afterWrite(inst);
        }

        async function runBulk(action, value) {
            if (inst.readonly || selectionCount(state) <= 0) return;
            showError('');
            try {
                const res = await api.bulk(pid, selectionSpec(state), action, value);
                if (!disposed) afterBulk(res, 'bulk_edit', 'chibi_sensei.bulk.done');
            } catch (err) { fail(err); }
        }

        async function tagBulk(action) {
            if (inst.readonly || typeof inst.ctx.modalDialog !== 'function') return;
            let value = null;
            try {
                value = await inst.ctx.modalDialog({ title: t(action === 'add_tag' ? 'chibi_sensei.bulk.add_tag' : 'chibi_sensei.bulk.remove_tag'), message: t('chibi_sensei.bulk.tag_prompt'), input: true, value: '' });
            } catch (_) { value = null; }
            if (disposed || typeof value !== 'string' || !value) return;
            runBulk(action, value);
        }

        async function deleteIds(spec, count, needPreview) {
            let token = '', n = count;
            if (needPreview) {
                const pre = await api.deletePreview(pid, spec);
                token = pre.token;
                n = pre.count;
            }
            const ok = await inst.askConfirm(t('chibi_sensei.bulk.delete_title'), t('chibi_sensei.bulk.delete_text', { count: P.format.number(n) }));
            if (!ok || disposed) return null;
            return api.bulk(pid, spec, 'delete', '', token);
        }

        async function deleteSelection() {
            if (inst.readonly || selectionCount(state) <= 0) return;
            showError('');
            try {
                const res = await deleteIds(selectionSpec(state), selectionCount(state), needsDeletePreview(state));
                if (res && !disposed) afterBulk(res, 'bulk_delete', 'chibi_sensei.bulk.deleted');
            } catch (err) { fail(err); }
        }

        async function undoBatch(batchId, closeHistory) {
            if (inst.readonly || !batchId) return;
            showError('');
            try {
                const res = await api.undo(pid, batchId);
                if (disposed) return;
                if (inst.lastBatch && inst.lastBatch.batch_id === batchId) { inst.lastBatch = null; inst.bus.emit('batch', null); }
                inst.notify(t('chibi_sensei.undo.done', { count: P.format.number(res.restored || 0) }), 'info');
                if (closeHistory) closeHistory();
                reload();
                afterWrite(inst);
            } catch (err) { fail(err); if (closeHistory) closeHistory(); }
        }

        async function openHistory() {
            let batches = [];
            try { batches = await api.history(pid, 20); } catch (err) { fail(err); return; }
            if (disposed) return;
            const dlg = makeDialog(inst, t('chibi_sensei.history.title'), 'history-dialog');
            const list = el('ul', { class: 'csn-history', dataset: { csn: 'history-list' } });
            batches.forEach(b => {
                const undo = el('button', { type: 'button', class: 'csn-btn csn-btn-quiet', dataset: { csn: 'history-undo', batch: b.batch_id }, text: t('chibi_sensei.undo.action'), onClick: () => undoBatch(b.batch_id, dlg.close) });
                undo.disabled = inst.readonly;
                list.appendChild(el('li', { class: 'csn-history-item' },
                    el('span', { class: 'csn-history-kind', text: t(kindKey(b.kind)) }),
                    el('span', { class: 'csn-muted', text: t('chibi_sensei.history.rows', { count: P.format.number(b.rows || 0) }) + ' · ' + P.format.dateTime(b.created) }),
                    undo));
            });
            dlg.content.appendChild(batches.length ? list : el('p', { class: 'csn-muted', text: t('chibi_sensei.history.empty') }));
            dlg.content.appendChild(el('div', { class: 'csn-actions' }, el('button', { type: 'button', class: 'csn-btn', dataset: { csn: 'history-close' }, text: t('chibi_sensei.editor.close'), onClick: dlg.close })));
            dlg.open();
        }

        // ---- editor ----
        function cloneDraft() { return JSON.parse(JSON.stringify(state.editor.draft)); }
        function change(mutate, rebuild) {
            if (inst.readonly || state.editor.state === 'closed' || state.editor.state === 'saving') return;
            const draft = cloneDraft();
            mutate(draft);
            dispatch({ type: 'edit_change', draft }, true);
            if (rebuild) paintEditor();
            paintNotice();
        }

        function messageBlock(msg, index, draft) {
            const roleSel = selectEl(ROLE_IDS.map(id => ({ value: id, label: t(ROLE_KEYS[id]) })), msg.role, { ariaLabel: t('chibi_sensei.editor.role'), dataset: { csn: 'msg-role', index } });
            roleSel.addEventListener('change', () => change(d => { d.messages[index].role = roleSel.value; }, true));
            const roleText = ROLE_KEYS[msg.role] ? t(ROLE_KEYS[msg.role]) : String(msg.role || '');
            const area = el('textarea', { class: 'csn-textarea', rows: '4', ariaLabel: t('chibi_sensei.editor.message_label', { role: roleText, n: index + 1 }), dataset: { csn: 'msg-content', index } });
            area.value = msg.content;
            area.addEventListener('input', () => change(d => { d.messages[index].content = area.value; }, false));
            const remove = el('button', { type: 'button', class: 'csn-btn csn-btn-quiet', dataset: { csn: 'msg-remove', index }, ariaLabel: t('chibi_sensei.editor.remove_message'), title: t('chibi_sensei.editor.remove_message'), onClick: () => change(d => { d.messages.splice(index, 1); }, true) }, icon('trash'));
            remove.disabled = draft.messages.length <= 1;
            return el('div', { class: 'csn-msg', dataset: { csn: 'msg', index } }, el('div', { class: 'csn-msg-head' }, roleSel, remove), area);
        }

        function paintEditor() {
            const ed = state.editor;
            P.clear(sheetBody);
            sheet.hidden = ed.state === 'closed';
            if (ed.state === 'closed') return;
            const draft = ed.draft;
            const row = ed.rowId === 'new' ? null : state.rows.find(r => r.id === ed.rowId) || ed.serverRow;
            const title = ed.rowId === 'new' ? t('chibi_sensei.editor.title_new') : t('chibi_sensei.editor.title_edit', { n: row ? row.seq : '' });
            sheetTitle.textContent = title;
            sheet.setAttribute('aria-label', title);
            if (typeof draft.text === 'string') {
                const area = el('textarea', { class: 'csn-textarea csn-textarea-tall', ariaLabel: t('chibi_sensei.editor.text_label'), dataset: { csn: 'editor-text' } });
                area.value = draft.text;
                area.addEventListener('input', () => change(d => { d.text = area.value; }, false));
                sheetBody.appendChild(el('label', { class: 'csn-field' }, el('span', { class: 'csn-label', text: t('chibi_sensei.editor.text_label') }), area));
            } else {
                draft.messages.forEach((m, i) => sheetBody.appendChild(messageBlock(m, i, draft)));
                const addTurn = el('button', { type: 'button', class: 'csn-btn', dataset: { csn: 'add-turn' }, onClick: () => change(d => { d.messages.push({ role: 'user', content: '' }, { role: 'assistant', content: '' }); }, true) }, icon('plus'), el('span', { text: t('chibi_sensei.editor.add_turn') }));
                const actions = el('div', { class: 'csn-actions' }, addTurn);
                if (!draft.messages.length || draft.messages[0].role !== 'system') {
                    actions.appendChild(el('button', { type: 'button', class: 'csn-btn', dataset: { csn: 'add-system' }, text: t('chibi_sensei.editor.add_system'), onClick: () => change(d => { d.messages.unshift({ role: 'system', content: '' }); }, true) }));
                }
                sheetBody.appendChild(actions);
            }
            const splitSel = selectEl(SPLIT_IDS.map(id => ({ value: id, label: t(SPLIT_KEYS[id]) })), draft.split || 'auto', { ariaLabel: t('chibi_sensei.editor.split_label'), dataset: { csn: 'editor-split' } });
            splitSel.addEventListener('change', () => change(d => { d.split = splitSel.value; }, false));
            const tags = el('input', { type: 'text', class: 'csn-input', ariaLabel: t('chibi_sensei.editor.tags_label'), dataset: { csn: 'editor-tags' } });
            tags.value = (draft.tags || []).join(', ');
            tags.addEventListener('input', () => change(d => { d.tags = tags.value.split(',').map(x => x.trim()).filter(Boolean); }, false));
            sheetBody.appendChild(el('label', { class: 'csn-field' }, el('span', { class: 'csn-label', text: t('chibi_sensei.editor.split_label') }), splitSel));
            sheetBody.appendChild(el('label', { class: 'csn-field' }, el('span', { class: 'csn-label', text: t('chibi_sensei.editor.tags_label') }), tags, el('span', { class: 'csn-hint', text: t('chibi_sensei.editor.tags_hint') })));
            if (row && row.est_tokens != null) sheetBody.appendChild(el('p', { class: 'csn-muted', dataset: { csn: 'editor-tokens' }, text: t('chibi_sensei.editor.est_tokens', { count: P.format.number(row.est_tokens) }) }));
            paintReadonly();
        }

        function paintNotice() {
            const ed = state.editor;
            P.clear(sheetNotice);
            saveBtn.disabled = inst.readonly || ed.state === 'saving';
            if (ed.state === 'conflict') {
                sheetNotice.appendChild(el('p', { class: 'csn-error', role: 'alert', text: t('chibi_sensei.editor.conflict') }));
                sheetNotice.appendChild(el('div', { class: 'csn-actions' },
                    el('button', { type: 'button', class: 'csn-btn', dataset: { csn: 'editor-load-server' }, text: t('chibi_sensei.editor.load_server'), onClick: () => { dispatch({ type: 'edit_take_server' }); } }),
                    el('button', { type: 'button', class: 'csn-btn', dataset: { csn: 'editor-keep-mine' }, text: t('chibi_sensei.editor.keep_mine'), onClick: () => { dispatch({ type: 'edit_keep_mine' }); } })));
            } else if (ed.state === 'invalid') {
                const list = el('ul', { class: 'csn-issues', dataset: { csn: 'editor-issues' } });
                ed.issues.forEach(issue => {
                    const known = P.errors.knownKey(t, issue && issue.message_key, 'chibi_sensei.editor.issue_generic');
                    list.appendChild(el('li', { text: known === t('chibi_sensei.editor.issue_generic') && issue && issue.message ? String(issue.message).slice(0, 200) : known }));
                });
                sheetNotice.appendChild(el('p', { class: 'csn-error', role: 'alert', text: t('chibi_sensei.editor.invalid') }));
                sheetNotice.appendChild(list);
            } else if (ed.error) {
                sheetNotice.appendChild(el('p', { class: 'csn-error', role: 'alert', text: ed.error }));
            }
        }

        async function confirmDiscard() {
            if (!state.editor.dirty || state.editor.state === 'closed') return true;
            return inst.askConfirm(t('chibi_sensei.leave_title'), t('chibi_sensei.editor.leave_text'));
        }

        async function openEditor(row) {
            if (disposed) return;
            if (state.editor.state !== 'closed' && state.editor.rowId === row.id) return;
            if (!(await confirmDiscard()) || disposed) return;
            opener = document.activeElement;
            dispatch({ type: 'edit_open', row });
            const first = sheet.querySelector('textarea, input, select');
            if (first) first.focus();
        }

        async function newExample() {
            if (disposed || inst.readonly) return;
            if (!(await confirmDiscard()) || disposed) return;
            opener = document.activeElement;
            dispatch({ type: 'edit_new', kind: free ? 'text' : 'chat' });
            const first = sheet.querySelector('textarea, input, select');
            if (first) first.focus();
        }

        async function closeEditor() {
            if (state.editor.state === 'closed' || state.editor.state === 'saving') return;
            if (!(await confirmDiscard()) || disposed) return;
            dispatch({ type: 'edit_close' });
            if (opener && typeof opener.focus === 'function' && opener.isConnected) opener.focus();
            opener = null;
        }

        function rowInput(draft) {
            const input = { split: draft.split || 'auto', tags: draft.tags || [] };
            if (typeof draft.text === 'string') input.text = draft.text; else input.messages = draft.messages;
            return input;
        }

        async function saveEditor() {
            const ed = state.editor;
            if (inst.readonly || ed.state === 'closed' || ed.state === 'saving') return;
            dispatch({ type: 'edit_save_start' });
            const isNew = ed.rowId === 'new';
            try {
                const res = isNew ? await api.addRows(pid, [rowInput(ed.draft)]) : await api.putRow(pid, ed.rowId, ed.baseRev, rowInput(ed.draft));
                if (disposed) return;
                dispatch({ type: 'edit_save_ok', rowId: ed.rowId, row: res.row, data_rev: res.data_rev });
                inst.notify(t(isNew ? 'chibi_sensei.editor.added' : 'chibi_sensei.editor.saved'), 'info');
                inst.lastBatch = null;
                inst.bus.emit('batch', null);
                paintSelection();
                afterWrite(inst);
                if (opener && opener.isConnected) opener.focus();
            } catch (err) {
                if (disposed || (err && err.aborted)) return;
                const e = P.errors.normalize(err);
                if (e.status === 412) {
                    let server = null;
                    try { server = await api.getRow(pid, ed.rowId); } catch (_) { server = null; }
                    if (!disposed) dispatch({ type: 'edit_conflict', row: server });
                } else if (e.status === 422) dispatch({ type: 'edit_invalid', issues: e.issues });
                else dispatch({ type: 'edit_failed', error: inst.handleError(err) });
            }
        }

        async function deleteFromEditor() {
            const ed = state.editor;
            if (inst.readonly || ed.rowId == null || ed.rowId === 'new') return;
            try {
                const res = await deleteIds({ ids: [ed.rowId] }, 1, false);
                if (!res || disposed) return;
                dispatch({ type: 'edit_close' });
                afterBulk(res, 'bulk_delete', 'chibi_sensei.bulk.deleted');
            } catch (err) { fail(err); }
        }

        // ---- wiring ----
        offs.push(inst.bus.on('rows', body => dispatch({ type: 'remote_rev', data_rev: body && body.data_rev })));
        offs.push(inst.bus.on('batch', () => paintUndo()));
        offs.push(inst.bus.on('readonly', () => { paintReadonly(); paintNotice(); paintUndo(); }));
        paintFilterControls();
        paintPager();
        paintSelection();
        paintUndo();
        reload();

        return {
            el: node,
            reload() { return reload(); },
            setFilter(patch) { dispatch({ type: 'filter', patch }); },
            newExample,
            selection() { return { spec: selectionSpec(state), count: selectionCount(state), mode: state.selection.mode }; },
            focus() { search.focus(); },
            dispose() {
                disposed = true;
                sendSearch.cancel();
                offs.forEach(off => { if (typeof off === 'function') off(); });
                inst.editorDirty = false;
            }
        };
    }

    P.table = {
        PAGE_SIZE, BULK_TOKEN_LIMIT, FILTER_KEYS, SPLIT_IDS, SPLIT_KEYS, CHECK_NAME_KEYS,
        initialState, reduce, selectionSpec, selectionCount, needsDeletePreview, query, rowPreview, lastOffset,
        create, afterWrite, makeDialog, selectEl, kindKey
    };
})();
