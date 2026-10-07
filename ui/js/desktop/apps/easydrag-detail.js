// EasyDrag detail view: a large overlay with three columns (input · parameters · output),
// tabs on narrow windows, node navigation, settings, note and "test this step".
(function () {
    'use strict';

    const ED = window.EasyDrag = window.EasyDrag || {};
    const NARROW = 900;
    // Step output is untrusted data (web pages, webhooks, model text) and the server accepts
    // JSON nested 10000 levels deep: the recursive renderers stop at MAX_DEPTH levels.
    const MAX_DEPTH = 24;
    // The output tree draws at most TREE_BUDGET entries, and at most MAX_FILES file cards.
    const TREE_BUDGET = 1500;
    const MAX_FILES = 50;
    // THUMBS caches PDF thumbnails by URL plus the file's size and step finish time (an
    // overwritten file gets a new thumbnail): a promise of a PNG data URL ('' when it failed; a
    // failed one is dropped, so a later render tries again). It keeps THUMB_CACHE entries.
    // pdf.js renders at most THUMB_PARALLEL at a time, each at most THUMB_W × THUMB_H pixels.
    const THUMBS = new Map();
    const THUMB_CACHE = 24;
    const THUMB_PARALLEL = 6;
    const THUMB_W = 240;
    const THUMB_H = 480;
    const thumbQueue = [];
    let thumbsActive = 0;

    function sortedNodes(model) {
        return model.doc.nodes.slice().sort((a, b) => a.position.x - b.position.x || a.position.y - b.position.y);
    }

    // safeFileUrl returns the same-origin path of a file under /files/, or '' for anything else.
    // File objects in step output can be forged ({"$type": "file", "web_path": "javascript:…"}).
    // Encoded separators (%2f, %5c) are refused: a server that decodes them could leave /files/.
    function safeFileUrl(path) {
        const origin = window.location && window.location.origin;
        if (typeof path !== 'string' || !path || !origin) return '';
        let url;
        try { url = new URL(path, origin); } catch (err) { return ''; }
        if (url.origin !== origin || !url.pathname.startsWith('/files/') || /%(2f|5c)/i.test(url.pathname)) return '';
        return url.pathname + url.search;
    }

    // capped copies value down to MAX_DEPTH levels; deeper values read "…" (for the JSON view).
    function capped(value, depth) {
        const d = depth || 0;
        if (!value || typeof value !== 'object') return value;
        if (d >= MAX_DEPTH) return '…';
        if (Array.isArray(value)) return value.map(v => capped(v, d + 1));
        const out = Object.create(null);
        Object.keys(value).forEach(k => { out[k] = capped(value[k], d + 1); });
        return out;
    }

    // jsonTree renders a value as nested <details>. A list or object shows 200 entries and
    // counts the rest; all in all TREE_BUDGET entries and MAX_DEPTH levels are drawn, and what
    // is left reads "…" (budget: {left}, shared by the whole tree).
    function jsonTree(value, esc, depth, budget) {
        const d = depth || 0;
        const b = budget || { left: TREE_BUDGET };
        if (value === null || value === undefined) return '<span class="ed-json-null">null</span>';
        if (typeof value !== 'object') {
            const cls = typeof value === 'string' ? 'ed-json-str' : typeof value === 'number' ? 'ed-json-num' : 'ed-json-bool';
            return '<span class="' + cls + '">' + esc(typeof value === 'string' ? '"' + value + '"' : String(value)) + '</span>';
        }
        if (d >= MAX_DEPTH || b.left <= 0) return '<span class="ed-json-more">…</span>';
        const keys = Array.isArray(value) ? null : Object.keys(value);
        const total = keys ? keys.length : value.length;
        const items = [];
        let i = 0;
        for (; i < Math.min(total, 200) && b.left > 0; i++) {
            b.left--;
            const k = keys ? keys[i] : i;
            items.push('<li><span class="ed-json-key">' + esc(String(k)) + '</span>: ' + jsonTree(value[k], esc, d + 1, b) + '</li>');
        }
        const body = items.join('') + (total > i ? '<li class="ed-json-more">… ' + (total - i) + '</li>' : '');
        const summary = Array.isArray(value) ? '[' + total + ']' : '{' + total + '}';
        return '<details' + (d < 2 ? ' open' : '') + '><summary>' + summary + '</summary><ul>' + body + '</ul></details>';
    }

    function tableOf(output) {
        if (!output || typeof output !== 'object') return null;
        const listKey = Object.keys(output).find(k => Array.isArray(output[k]) && output[k].length && typeof output[k][0] === 'object');
        if (!listKey) return null;
        const rows = output[listKey].slice(0, 50);
        const cols = Array.from(new Set(rows.flatMap(r => Object.keys(r || {})))).slice(0, 8);
        return { listKey, rows, cols, total: output[listKey].length };
    }

    function files(value, out, depth) {
        const list = out || [];
        const d = depth || 0;
        if (value && typeof value === 'object' && d < MAX_DEPTH && list.length < MAX_FILES) {
            if (value.$type === 'file') list.push(value);
            else Object.values(value).forEach(v => files(v, list, d + 1));
        }
        return list;
    }

    // renderThumb draws page 1 of a PDF with pdf.js; the loading task owns a worker and is
    // always destroyed.
    async function renderThumb(url) {
        const pdfjs = window.pdfjsLib;
        if (!pdfjs) return '';
        let task = null;
        try {
            task = pdfjs.getDocument({ url });
            const doc = await task.promise;
            const page = await doc.getPage(1);
            // Any page size gives a thumbnail of at most THUMB_W × THUMB_H pixels.
            const size = page.getViewport({ scale: 1 });
            if (!(size.width > 0 && size.height > 0)) return '';
            const viewport = page.getViewport({ scale: Math.min(THUMB_W / size.width, THUMB_H / size.height) });
            const canvas = document.createElement('canvas');
            canvas.width = Math.max(1, Math.floor(viewport.width));
            canvas.height = Math.max(1, Math.floor(viewport.height));
            await page.render({ canvasContext: canvas.getContext('2d'), viewport }).promise;
            return canvas.toDataURL('image/png');
        } catch (err) {
            return '';
        } finally {
            if (task) Promise.resolve().then(() => task.destroy()).catch(() => { /* already gone */ });
        }
    }

    // queuedThumb renders a thumbnail once a slot of THUMB_PARALLEL is free.
    function queuedThumb(url) {
        return new Promise(resolve => {
            const start = () => {
                thumbsActive++;
                renderThumb(url).catch(() => '').then(src => {
                    thumbsActive--;
                    const next = thumbQueue.shift();
                    if (next) next();
                    resolve(src);
                });
            };
            if (thumbsActive < THUMB_PARALLEL) start(); else thumbQueue.push(start);
        });
    }

    function pdfThumb(url, version) {
        const key = url + '\n' + (version || '');
        const cached = THUMBS.get(key);
        if (cached) return cached;
        const job = queuedThumb(url);
        THUMBS.set(key, job);
        if (THUMBS.size > THUMB_CACHE) THUMBS.delete(THUMBS.keys().next().value);
        job.then(src => { if (!src && THUMBS.get(key) === job) THUMBS.delete(key); });
        return job;
    }

    function open(ed, nodeId, opts) {
        const core = ED.core;
        const { t, esc } = ed;
        close(ed);
        const bag = core.bag();
        let node = ed.model.node(nodeId);
        if (!node) return null;
        let info = ed.model.info(node.type);
        let pane = 'params';
        let narrowTab = 'params';
        let outputView = 'tree';
        let sourceId = 'last';
        let form = null;
        let mapping = null;
        let lastField = null;
        // What the columns show: run events re-render the output only for a new step object of
        // this node (step_started and step_finished replace it), the input and the form
        // previews only for new run data.
        let shownStep;
        let shownNode = null;
        let shownData = null;

        const el = core.el('<div class="ed-detail-backdrop"><div class="ed-detail" role="dialog" aria-modal="true">' +
            '<header class="ed-detail-head">' +
            '<div class="ed-detail-nav"><button type="button" class="ed-icon-btn" data-ed-nav="-1" aria-label="' + esc(t('easydrag.ui.detail_prev')) + '" title="' + esc(t('easydrag.ui.detail_prev')) + '">' + core.icon('chevron-left') + '</button>' +
            '<button type="button" class="ed-icon-btn" data-ed-nav="1" aria-label="' + esc(t('easydrag.ui.detail_next')) + '" title="' + esc(t('easydrag.ui.detail_next')) + '">' + core.icon('chevron-right') + '</button></div>' +
            '<span class="ed-detail-tile"></span><div class="ed-detail-title"><input class="ed-detail-label" aria-label="' + esc(t('easydrag.ui.detail_label')) + '"><span class="ed-detail-type"></span></div>' +
            '<span class="ed-detail-status"></span>' +
            '<div class="ed-detail-actions"><button type="button" class="ed-btn" data-ed-detail-test>' + core.icon('flask') + '<span>' + esc(t('easydrag.ui.detail_test')) + '</span></button>' +
            '<button type="button" class="ed-icon-btn" data-ed-detail-close aria-label="' + esc(t('easydrag.ui.close')) + '" title="' + esc(t('easydrag.ui.close')) + ' (Esc)">' + core.icon('x') + '</button></div></header>' +
            '<div class="ed-detail-tabs" role="tablist">' + ['input', 'params', 'output'].map(id => '<button type="button" role="tab" data-ed-narrow="' + id + '">' + esc(t('easydrag.ui.detail_' + (id === 'params' ? 'parameters' : id))) + '</button>').join('') + '</div>' +
            '<div class="ed-detail-body">' +
            '<section class="ed-detail-col" data-col="input"></section>' +
            '<section class="ed-detail-col" data-col="params"><div class="ed-subtabs" role="tablist">' +
            '<button type="button" role="tab" data-ed-pane="params">' + esc(t('easydrag.ui.detail_parameters')) + '</button>' +
            '<button type="button" role="tab" data-ed-pane="settings">' + esc(t('easydrag.ui.detail_settings')) + '</button>' +
            '<button type="button" role="tab" data-ed-pane="note">' + esc(t('easydrag.ui.detail_note')) + '</button></div>' +
            '<div class="ed-detail-pane" data-pane="params"></div><div class="ed-detail-pane" data-pane="settings" hidden></div><div class="ed-detail-pane" data-pane="note" hidden></div></section>' +
            '<section class="ed-detail-col" data-col="output"><div class="ed-col-head"><h3>' + esc(t('easydrag.ui.detail_output')) + '</h3><div class="ed-seg ed-seg--small" role="radiogroup">' +
            ['tree', 'table', 'json'].map(v => '<button type="button" role="radio" data-ed-view="' + v + '">' + esc(t('easydrag.ui.view_' + v)) + '</button>').join('') + '</div></div>' +
            '<div class="ed-output-status"></div><div class="ed-output"></div></section>' +
            '</div></div></div>');
        ed.root.appendChild(el);
        const dialog = el.querySelector('.ed-detail');
        const labelInput = el.querySelector('.ed-detail-label');

        function upstreamList() {
            const ids = ed.model.upstream(node.id);
            return sortedNodes(ed.model).filter(n => ids.has(n.id)).map(n => {
                const i = ed.model.info(n.type);
                return { key: n.key, label: n.label || n.type, icon: i ? i.icon : 'tool', cat: core.catOf(i), fields: ed.model.fieldsOf(n), nodeId: n.id };
            });
        }

        // runData is the data the input column offers: in the run view that run's own data
        // (runs.applyRunView), never the editor's last run.
        function runData() { return (ed.runView ? ed.runView.data : ed.lastRunData) || null; }

        function currentStep() { return (ed.run && ed.run.steps && ed.run.steps.get(node.id)) || null; }

        function sources() {
            const list = [];
            const data = runData();
            if (data) list.push({ id: 'last', label: data.label, roots: data.roots });
            list.push({ id: 'schema', label: t('easydrag.ui.input_schema'), roots: {} });
            if (!list.some(s => s.id === sourceId)) sourceId = list[0].id;
            return list;
        }

        function roots() {
            const s = sources().find(x => x.id === sourceId);
            return s ? s.roots : {};
        }

        function renderHead() {
            info = ed.model.info(node.type);
            el.querySelector('.ed-detail-tile').innerHTML = core.icon(info ? info.icon : 'tool');
            el.querySelector('.ed-detail-tile').dataset.cat = core.catOf(info);
            if (document.activeElement !== labelInput) labelInput.value = node.label || '';
            labelInput.disabled = !!(ed.readonly || ed.runView);
            // A step that still carries its type's label names only its key (the label above says the type).
            el.querySelector('.ed-detail-type').innerHTML = info && node.label === info.label
                ? '<span class="ed-code">' + esc(node.key) + '</span>'
                : esc((info ? info.label : node.type) + ' · ' + node.key);
            const step = ed.run && ed.run.steps && ed.run.steps.get(node.id);
            el.querySelector('.ed-detail-status').innerHTML = step ? '<span class="ed-status-pill ed-status-pill--' + esc(step.status) + '">' + esc(core.tr(t, 'easydrag.ui.status_' + step.status, step.status)) + '</span>' : '';
            // One run at a time: "test this step" waits until the active run ended.
            const busy = !!(ed.run && !ed.run.view && !ed.run.stale && ED.runs && !ED.runs.isFinal(ed.run.status));
            el.querySelector('[data-ed-detail-test]').disabled = !!(ed.readonly || ed.runView || (info && info.trigger) || busy);
            const list = sortedNodes(ed.model);
            const idx = list.findIndex(n => n.id === node.id);
            el.querySelector('[data-ed-nav="-1"]').disabled = idx <= 0;
            el.querySelector('[data-ed-nav="1"]').disabled = idx >= list.length - 1;
            dialog.setAttribute('aria-label', node.label || node.type);
        }

        function renderInput() {
            const host = el.querySelector('[data-col="input"]');
            shownData = runData();
            // The run view is read-only: its input tree gets a read-only ed (rows are neither
            // draggable nor clickable inserts) and no insert callback.
            const locked = !!ed.runView;
            const env = {
                ed: locked && !ed.readonly ? Object.assign(Object.create(ed), { readonly: true }) : ed, sources: sources(), sourceId, upstream: upstreamList(),
                onSource: id => { sourceId = id; if (form) form.refresh(node, roots()); },
                onInsert: locked ? () => {} : ref => { if (form && lastField) form.insert(lastField, ref); else if (form) firstTemplatable(ref); }
            };
            if (!mapping) { mapping = ED.mapping.create(env); host.appendChild(mapping.el); }
            else mapping.refresh(env);
        }

        function firstTemplatable(ref) {
            const p = ((info && info.params) || []).find(x => x.templatable && ED.forms.visible(x, node.params));
            if (p) form.insert(p.name, ref);
        }

        function renderParams() {
            const host = el.querySelector('[data-pane="params"]');
            host.innerHTML = '';
            if (!info) {
                host.innerHTML = '<p class="ed-form-empty">' + esc(t('easydrag.ui.badge_unknown')) + '</p>';
                return;
            }
            const avail = info.availability || {};
            if (avail.state && avail.state !== 'available') {
                host.appendChild(core.el('<div class="ed-callout ed-callout--warn">' + core.icon('settings') + '<div><strong>' + esc(t('easydrag.ui.badge_needs_setup')) + '</strong><p>' + esc(t('easydrag.ui.setup_hint')) + '</p>' +
                    '<a class="ed-link" href="/config#' + esc(avail.config_section || 'tools') + '" target="_blank" rel="noopener">' + esc(t('easydrag.ui.setup_open')) + '</a></div></div>'));
            }
            if (info.description) host.appendChild(core.el('<p class="ed-detail-desc">' + esc(info.description) + '</p>'));
            form = ED.forms.render({
                ed, node, info, upstream: upstreamList(), roots: roots(), issues: ed.issues,
                onChange: (name, value) => ed.model.setParam(node.id, name, value)
            });
            host.appendChild(form.el);
            if (opts && opts.param) { const p = opts.param; opts = null; requestAnimationFrame(() => form.focus(p)); }
        }

        // The settings and the note belong to the node they were drawn for (id), not to the
        // node shown when a change event fires.
        function renderSettings() {
            const host = el.querySelector('[data-pane="settings"]');
            const id = node.id;
            const s = node.settings || {};
            const ro = !!(ed.readonly || ed.runView);
            const fenv = { t, esc, readonly: ro };
            host.innerHTML = '';
            const keyId = 'ed-key-' + ed.windowId + '-' + id;
            const keyField = core.el('<div class="ed-field"><div class="ed-field-head"><label for="' + esc(keyId) + '">' + esc(t('easydrag.ui.settings_key')) + '</label></div>' +
                '<input class="ed-input ed-code" id="' + esc(keyId) + '" value="' + esc(node.key) + '"' + (ro ? ' disabled' : '') + ' spellcheck="false"><p class="ed-hint">' + esc(t('easydrag.ui.settings_key_hint')) + '</p><p class="ed-error" role="alert" hidden></p></div>');
            keyField.querySelector('input').addEventListener('change', (event) => {
                const res = ed.model.setKey(id, event.target.value.trim());
                const err = keyField.querySelector('.ed-error');
                err.hidden = res.ok;
                const current = ed.model.node(id);
                if (!res.ok) { err.textContent = core.tr(t, 'easydrag.ui.key_' + res.reason, t('easydrag.ui.key_invalid')); if (current) event.target.value = current.key; }
            });
            host.appendChild(keyField);
            const onError = core.el('<div class="ed-field"><div class="ed-field-head"><label>' + esc(t('easydrag.ui.settings_on_error')) + '</label></div></div>');
            onError.appendChild(ED.fields.segmented(fenv, [
                { value: 'stop', label: t('easydrag.ui.on_error_stop') }, { value: 'continue', label: t('easydrag.ui.on_error_continue') }, { value: 'error_port', label: t('easydrag.ui.on_error_port') }
            ], s.on_error || 'stop', v => ed.model.setSettings(id, { on_error: v === 'stop' ? undefined : v }), { label: t('easydrag.ui.settings_on_error') }));
            onError.appendChild(core.el('<p class="ed-hint">' + esc(t('easydrag.ui.settings_on_error_hint')) + '</p>'));
            host.appendChild(onError);
            const retry = core.el('<div class="ed-field ed-field--row"><label>' + esc(t('easydrag.ui.settings_retries')) +
                '<input type="number" min="0" max="5" class="ed-input" data-k="count" value="' + esc((s.retry && s.retry.count) || 0) + '"' + (ro ? ' disabled' : '') + '></label>' +
                '<label>' + esc(t('easydrag.ui.settings_retry_delay')) + '<input type="number" min="0" max="600" class="ed-input" data-k="delay_seconds" value="' + esc((s.retry && s.retry.delay_seconds) || 10) + '"' + (ro ? ' disabled' : '') + '></label>' +
                '<label>' + esc(t('easydrag.ui.settings_timeout')) + '<input type="number" min="0" max="3600" class="ed-input" data-k="timeout" value="' + esc(s.timeout_seconds || '') + '" placeholder="' + esc(t('easydrag.ui.settings_default')) + '"' + (ro ? ' disabled' : '') + '></label></div>');
            retry.addEventListener('change', () => {
                const count = core.clamp(Number(retry.querySelector('[data-k="count"]').value) || 0, 0, 5);
                const delay = core.clamp(Number(retry.querySelector('[data-k="delay_seconds"]').value) || 0, 0, 600);
                const timeout = Number(retry.querySelector('[data-k="timeout"]').value) || undefined;
                ed.model.setSettings(id, { retry: count ? { count, delay_seconds: delay } : undefined, timeout_seconds: timeout });
            });
            host.appendChild(retry);
            const disabled = core.el('<div class="ed-field ed-field--inline"><label>' + esc(t('easydrag.ui.settings_disabled')) + '</label></div>');
            disabled.appendChild(ED.fields.toggle(fenv, !!s.disabled, () => ed.model.toggleDisabled([id]), t('easydrag.ui.settings_disabled')));
            host.appendChild(disabled);
        }

        // A typed note is saved 400 ms after the last key, or at once when the view moves to
        // another node or closes (flushNote).
        let pendingNote = null;
        const saveNote = core.debounce(() => flushNote(), 400);

        function flushNote() {
            saveNote.cancel();
            if (!pendingNote) return;
            const { id, value } = pendingNote;
            pendingNote = null;
            if (ed.model.node(id)) ed.model.setSettings(id, { notes: value || undefined });
        }

        function renderNote() {
            const host = el.querySelector('[data-pane="note"]');
            const ro = !!(ed.readonly || ed.runView);
            if (host.querySelector('textarea') && document.activeElement === host.querySelector('textarea')) return;
            const id = node.id;
            host.innerHTML = '<textarea class="ed-input ed-note" rows="10" aria-label="' + esc(t('easydrag.ui.detail_note')) + '" placeholder="' + esc(t('easydrag.ui.note_placeholder')) + '"' + (ro ? ' disabled' : '') + '>' + esc((node.settings || {}).notes || '') + '</textarea>';
            host.querySelector('textarea').addEventListener('input', (event) => { pendingNote = { id, value: event.target.value }; saveNote(); });
        }

        function renderOutput() {
            const step = currentStep();
            const status = el.querySelector('.ed-output-status');
            const out = el.querySelector('.ed-output');
            // A new result of the same node keeps the scroll position; another node starts at the top.
            const scroll = shownNode === node.id ? out.scrollTop || 0 : 0;
            shownStep = step;
            shownNode = node.id;
            el.querySelectorAll('[data-ed-view]').forEach(b => b.setAttribute('aria-checked', String(b.dataset.edView === outputView)));
            if (!step) {
                status.innerHTML = '';
                out.innerHTML = '<div class="ed-output-empty">' + core.icon('flask') + '<p>' + esc(t('easydrag.ui.output_none')) + '</p></div>';
                return;
            }
            const parts = ['<span class="ed-status-pill ed-status-pill--' + esc(step.status) + '">' + esc(core.tr(t, 'easydrag.ui.status_' + step.status, step.status)) + '</span>'];
            if (step.duration_ms) parts.push(esc(core.fmt.duration(step.duration_ms)));
            if (step.output && step.output.tokens) parts.push(esc(t('easydrag.ui.output_tokens', { count: (step.output.tokens.input || 0) + (step.output.tokens.output || 0) })));
            if (step.item_count) parts.push(esc(t('easydrag.ui.output_items', { count: step.item_count })));
            if (step.output_truncated) parts.push('<span class="ed-chip ed-chip--warn">' + esc(t('easydrag.ui.output_truncated')) + '</span>');
            status.innerHTML = parts.join(' · ');
            if (step.status === 'error') {
                out.innerHTML = '<div class="ed-callout ed-callout--error">' + core.icon('alert') + '<div><strong>' + esc(t('easydrag.ui.output_error')) + '</strong><p>' + esc(core.stepErrorText(t, step)) + '</p></div></div>';
                return;
            }
            const output = step.output || {};
            const fileList = files(output);
            let html = fileList.map(f => {
                const mime = String(f.mime || '');
                const url = safeFileUrl(f.web_path);
                const media = !url ? '' : mime.startsWith('image/') ? '<img src="' + esc(url) + '" alt="" loading="lazy">'
                    : mime.startsWith('audio/') ? '<audio controls preload="none" src="' + esc(url) + '"></audio>'
                    : mime === 'application/pdf' ? '<img class="ed-pdf-thumb" data-pdf="' + esc(url) + '" data-pdf-version="' + esc((f.size || '') + '|' + (step.finished_at || '')) + '" alt="" hidden>' : '';
                return '<div class="ed-file-card">' + media + '<div class="ed-file-meta">' + core.icon('file-text') + '<span>' + esc(f.name || f.path || '') + '</span>' +
                    (f.size ? '<span class="ed-muted">' + esc(core.fmt.bytes(f.size)) + '</span>' : '') +
                    (url ? '<a class="ed-link" href="' + esc(url) + '" target="_blank" rel="noopener">' + esc(t('easydrag.ui.output_open')) + '</a>' : '') + '</div></div>';
            }).join('');
            if (outputView === 'json') html += '<pre class="ed-code ed-output-json">' + esc(JSON.stringify(capped(output), null, 2)) + '</pre>';
            else if (outputView === 'table') {
                const table = tableOf(output);
                html += table ? '<div class="ed-table-wrap"><table class="ed-table"><thead><tr>' + table.cols.map(c => '<th>' + esc(c) + '</th>').join('') + '</tr></thead><tbody>' +
                    table.rows.map(r => '<tr>' + table.cols.map(c => '<td>' + esc(ED.mapping.preview(r ? r[c] : null)) + '</td>').join('') + '</tr>').join('') + '</tbody></table></div>' +
                    (table.total > table.rows.length ? '<p class="ed-hint">' + esc(t('easydrag.ui.tree_more', { count: table.total - table.rows.length })) + '</p>' : '')
                    : '<p class="ed-hint">' + esc(t('easydrag.ui.output_no_table')) + '</p>';
            } else html += '<div class="ed-json-tree">' + jsonTree(output, esc) + '</div>';
            out.innerHTML = html;
            out.scrollTop = scroll;
            // PDF thumbnails come from the cache (pdf.js renders each file version once). Without
            // pdf.js, or when it fails, the card keeps its "open" link only (no embedded frame for
            // files from step output).
            out.querySelectorAll('img[data-pdf]').forEach(img => {
                pdfThumb(img.dataset.pdf, img.dataset.pdfVersion).then(src => {
                    if (!out.contains(img)) return; // redrawn meanwhile
                    if (!src) { img.remove(); return; }
                    img.src = src;
                    img.hidden = false;
                });
            });
        }

        function applyPane() {
            el.querySelectorAll('[data-ed-pane]').forEach(b => b.setAttribute('aria-selected', String(b.dataset.edPane === pane)));
            el.querySelectorAll('.ed-detail-pane').forEach(p => { p.hidden = p.dataset.pane !== pane; });
            if (pane === 'settings') renderSettings();
            if (pane === 'note') renderNote();
        }

        function applyNarrow() {
            const narrow = ed.root.getBoundingClientRect().width < NARROW;
            dialog.classList.toggle('is-narrow', narrow);
            el.querySelectorAll('[data-ed-narrow]').forEach(b => b.setAttribute('aria-selected', String(b.dataset.edNarrow === narrowTab)));
            el.querySelectorAll('.ed-detail-col').forEach(c => { c.classList.toggle('is-active', c.dataset.col === narrowTab); });
        }

        function renderAll() {
            renderHead();
            renderInput();
            renderParams();
            applyPane();
            renderOutput();
            applyNarrow();
        }

        function go(delta) {
            const list = sortedNodes(ed.model);
            const idx = list.findIndex(n => n.id === node.id);
            const next = list[idx + delta];
            if (!next) return;
            flushNote();
            node = next;
            lastField = null;
            mapping = null;
            el.querySelector('[data-col="input"]').innerHTML = '';
            ed.selection.clear();
            ed.selection.add(node.id);
            ed.bus.emit('selection', ed.selection);
            renderAll();
        }

        function closeSelf() { close(ed); }

        // trapTab keeps Tab and Shift+Tab inside the dialog (it is aria-modal).
        function trapTab(event) {
            const items = Array.from(dialog.querySelectorAll('button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])'))
                .filter(n => !n.disabled && n.getAttribute('tabindex') !== '-1' && n.offsetParent !== null);
            if (!items.length) return;
            const active = document.activeElement;
            const first = items[0];
            const last = items[items.length - 1];
            if (event.shiftKey && (active === first || !dialog.contains(active))) { event.preventDefault(); last.focus(); }
            else if (!event.shiftKey && (active === last || !dialog.contains(active))) { event.preventDefault(); first.focus(); }
        }

        // stepGroup moves to the next tab or radio of a group and selects it.
        function stepGroup(group, item, delta) {
            const items = Array.from(group.querySelectorAll('[role="tab"], [role="radio"]')).filter(b => !b.disabled);
            const next = items[(items.indexOf(item) + delta + items.length) % items.length];
            if (next && next !== item) { next.focus(); next.click(); }
        }

        bag.listen(el, 'focusin', (event) => {
            const field = event.target.closest && event.target.closest('.ed-field[data-param]');
            if (field) lastField = field.dataset.param;
        });
        bag.listen(el, 'click', (event) => {
            if (event.target === el) { closeSelf(); return; }
            const nav = event.target.closest('[data-ed-nav]');
            if (nav) { go(Number(nav.dataset.edNav)); return; }
            if (event.target.closest('[data-ed-detail-close]')) { closeSelf(); return; }
            if (event.target.closest('[data-ed-detail-test]')) { ed.bus.emit('node-test', { nodeId: node.id }); return; }
            const p = event.target.closest('[data-ed-pane]');
            if (p) { pane = p.dataset.edPane; applyPane(); return; }
            const nt = event.target.closest('[data-ed-narrow]');
            if (nt) { narrowTab = nt.dataset.edNarrow; applyNarrow(); return; }
            const v = event.target.closest('[data-ed-view]');
            if (v) { outputView = v.dataset.edView; renderOutput(); }
        });
        bag.listen(el, 'keydown', (event) => {
            if (event.key === 'Escape') {
                if (core.isEditable(event.target) && event.target.closest('.ed-tpl')) return;
                event.stopPropagation();
                closeSelf();
                return;
            }
            if (event.key === 'Tab') { trapTab(event); return; }
            if (core.isEditable(event.target)) return;
            if (event.key !== 'ArrowLeft' && event.key !== 'ArrowRight') return;
            const delta = event.key === 'ArrowRight' ? 1 : -1;
            // In a tab or radio group the arrows move within the group, not to another node.
            const item = event.target.closest('[role="tab"], [role="radio"]');
            const group = item && item.closest('[role="tablist"], [role="radiogroup"]');
            if (group) { event.preventDefault(); stepGroup(group, item, delta); return; }
            if (event.target.closest('.ed-tree')) return;
            event.preventDefault();
            go(delta);
        });
        bag.listen(labelInput, 'input', () => ed.model.setLabel(node.id, labelInput.value));
        bag.add(ed.bus.on('model', change => {
            const current = ed.model.node(node.id);
            if (!current) { closeSelf(); return; }
            node = current;
            renderHead();
            if (form && (change.nodes.includes(node.id) || change.kind !== 'change')) form.refresh(node, roots(), ed.issues);
            if (pane === 'settings' && change.nodes.includes(node.id) && !el.querySelector('[data-pane="settings"]').contains(document.activeElement)) renderSettings();
        }));
        // refreshData shows new run data in the input tree and the form previews.
        function refreshData() {
            if (runData() === shownData) return;
            if (mapping) renderInput();
            if (form) form.refresh(node, roots(), ed.issues);
        }
        bag.add(ed.bus.on('run', () => {
            renderHead();
            if (currentStep() !== shownStep) renderOutput();
            refreshData();
        }));
        bag.add(ed.bus.on('last-run', refreshData));
        bag.add(ed.bus.on('issues', () => { if (form) form.refresh(node, roots(), ed.issues); }));
        const ro = new ResizeObserver(applyNarrow);
        ro.observe(ed.root);
        bag.add(() => ro.disconnect());

        // ed.detail exists before the first render, so a render that throws still closes cleanly.
        ed.detail = {
            nodeId: () => node.id,
            close: () => {
                flushNote();
                bag.dispose();
                el.classList.add('is-closing');
                setTimeout(() => el.remove(), 160);
                ed.detail = null;
                ed.bus.emit('detail-closed', node.id);
            }
        };
        try {
            renderAll();
        } catch (err) {
            console.error('EasyDrag detail view failed', err);
            ed.detail.close();
            return null;
        }
        // The label field takes focus; in the run view (label disabled) the close button does.
        if (!(opts && opts.param)) (labelInput.disabled ? el.querySelector('[data-ed-detail-close]') : labelInput).focus({ preventScroll: true });
        requestAnimationFrame(() => el.classList.add('is-open'));
        return ed.detail;
    }

    function close(ed) { if (ed.detail) ed.detail.close(); }

    ED.detail = { open, close, jsonTree, safeFileUrl };
})();
