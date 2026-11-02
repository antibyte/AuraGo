(function () {
    'use strict';
    let mirror;
    const copy = value => JSON.parse(JSON.stringify(value));
    const source = cell => Array.isArray(cell.source) ? cell.source.join('') : String(cell.source || '');
    const id = () => crypto.randomUUID().replaceAll('-', '');

    window.KaggleNotebook = async function (host, options) {
        const { api, t, project, file, windowId, notify } = options;
        let doc, revision, session, disposed = false, dirty = false, generation = 0, contentVersion = 0, running = null;
        let restoredDraft = null;
        let editorViews = [], eventSource, savePending = null, runAllStopped = false;
        const completionWaiters = new Map();
        const earlyEvents = []; let pendingCommand = false, startPending = false;
        const earlySessions = new Map();
        const editorThemes = new Map();
        const lightTheme = () => document.body.dataset.scheme === 'light' || document.body.dataset.theme === 'fruity';
        const themeObserver = new MutationObserver(() => {
            editorThemes.forEach((theme, view) => view.dispatch({ effects: theme.reconfigure(lightTheme() ? [] : mirror.oneDark) }));
        });
        themeObserver.observe(document.body, { attributes: true, attributeFilter: ['data-theme', 'data-scheme'] });
        const draftKey = 'aurago.kaggle.draft.' + project + '.' + file + '.' + windowId;
        const path = '/projects/' + encodeURIComponent(project) + '/notebooks/' + encodeURIComponent(file);
        const endpoint = suffix => '/api/kaggle' + suffix;
        const root = document.createElement('section'); root.className = 'kg-notebook'; host.replaceChildren(root);
        const toolbar = document.createElement('div'); toolbar.className = 'kg-toolbar'; root.append(toolbar);
        const titleBar = document.createElement('div'); titleBar.className = 'kg-toolbar-title';
        const label = document.createElement('strong'); label.textContent = file; titleBar.append(label);
        const dirtyChip = document.createElement('span'); dirtyChip.className = 'kg-chip'; dirtyChip.dataset.tone = 'warn'; dirtyChip.hidden = true;
        const dirtyDot = document.createElement('span'); dirtyDot.className = 'kg-dot'; dirtyChip.append(dirtyDot);
        dirtyChip.setAttribute('aria-label', t('unsaved')); dirtyChip.title = t('unsaved'); titleBar.append(dirtyChip);
        const kernelChip = document.createElement('span'); kernelChip.className = 'kg-chip'; kernelChip.dataset.tone = 'muted'; kernelChip.title = t('kernel'); titleBar.append(kernelChip);
        const documentGroup = document.createElement('div'); documentGroup.className = 'kg-toolbar-group';
        const runtimeGroup = document.createElement('div'); runtimeGroup.className = 'kg-toolbar-group';
        const remoteGroup = document.createElement('div'); remoteGroup.className = 'kg-toolbar-group';
        toolbar.append(titleBar, documentGroup, runtimeGroup, remoteGroup);
        const state = document.createElement('p'); state.className = 'kg-state'; state.setAttribute('role', 'status'); root.append(state);
        const cells = document.createElement('div'); cells.className = 'kg-cells'; root.append(cells);
        function button(key, action, parent = toolbar, variant) { const b = document.createElement('button'); b.type = 'button'; b.textContent = t(key); b.dataset.action = key; if (variant) b.classList.add('kg-btn-' + variant); const local = ['local', 'run_all', 'restart', 'run_cell'].includes(key); const grant = local ? 'local' : key === 'kaggle_run' ? 'execute' : ['save', 'add_code', 'add_markdown', 'up', 'down', 'remove_cell'].includes(key) ? 'write' : ''; const reason = grant ? options.unavailable?.({ grant }) : ''; if (reason) { b.disabled = true; b.title = reason; } b.addEventListener('click', () => Promise.resolve(action()).catch(showError)); parent.append(b); return b; }
        function showError(error) { if (!disposed) { state.textContent = t('error') + ': ' + error.message; state.classList.add('kg-error'); } }
        function syncKernelChip() {
            if (!session) { kernelChip.dataset.tone = 'muted'; kernelChip.textContent = t('kernel'); return; }
            if (session.status === 'idle') { kernelChip.dataset.tone = 'ok'; kernelChip.textContent = t('kernel_ready'); return; }
            if (session.status === 'starting') { kernelChip.dataset.tone = 'busy'; kernelChip.textContent = t('starting'); return; }
            if (session.status === 'stopped' || session.status === 'stopping') { kernelChip.dataset.tone = 'muted'; kernelChip.textContent = t('kernel'); return; }
            kernelChip.dataset.tone = 'busy'; kernelChip.textContent = t('running_local');
        }
        function draft() { if (!doc) return; try { localStorage.setItem(draftKey, JSON.stringify({ doc, revision, dirty, at: Date.now() })); } catch (_) { state.textContent = t('draft_storage_full'); } }
        function edited() { runAllStopped = true; dirty = true; generation++; contentVersion++; draft(); state.textContent = t('unsaved'); state.classList.remove('kg-error'); dirtyChip.hidden = false; }
        async function load() {
            const loaded = await api(path); if (disposed || options.isClosed?.()) return;
            doc = loaded.content; revision = loaded.revision;
            const prefix = 'aurago.kaggle.draft.' + project + '.' + file + '.';
            let stored;
            for (let i = 0; i < localStorage.length; i++) { const key = localStorage.key(i); if (!key.startsWith(prefix)) continue; try { const raw = localStorage.getItem(key), candidate = JSON.parse(raw); if (candidate.dirty && candidate.doc?.nbformat === 4 && Array.isArray(candidate.doc.cells) && (!stored || candidate.at > stored.at)) { stored = candidate; restoredDraft = { key, raw }; } } catch (_) {} }
            if (stored) { doc = stored.doc; revision = stored.revision; dirty = true; dirtyChip.hidden = false; state.textContent = t('draft_restored'); }
            else state.textContent = t('saved');
            doc.cells.forEach((cell, index) => { if (!cell.id) cell.id = 'cell-' + index; });
            try { mirror = mirror || await import('/js/vendor/codemirror-bundle.esm.js'); } catch (_) { state.textContent = t('editor_fallback'); }
            if (disposed || options.isClosed?.()) return;
            renderCells(); connectEvents();
            const sessions = await api('/runtime', { method: 'POST', body: { operation: 'sessions' } });
            session = sessions.find(s => s.project_id === project && s.file === file && s.status !== 'stopped');
            syncKernelChip();
            if (session) applySession(earlySessions.get(session.id) || session);
        }
        function output(parent, out) {
            const box = document.createElement('div'); box.className = 'kg-output'; parent.append(box);
            if (out.output_type === 'stream') { const pre = document.createElement('pre'); pre.textContent = Array.isArray(out.text) ? out.text.join('') : out.text || ''; box.append(pre); return; }
            if (out.output_type === 'error') { const pre = document.createElement('pre'); pre.textContent = (out.traceback || [out.ename + ': ' + out.evalue]).join('\n').replace(/\x1b\[[0-9;]*m/g, ''); box.append(pre); return; }
            const data = out.data || {};
            for (const mime of ['image/png', 'image/jpeg']) if (typeof data[mime] === 'string' && /^[A-Za-z0-9+/=\s]+$/.test(data[mime]) && data[mime].length < 2 * 1024 * 1024) { const img = document.createElement('img'); img.alt = t('cell_output'); img.src = 'data:' + mime + ';base64,' + data[mime].replace(/\s/g, ''); box.append(img); return; }
            if (data['text/html'] && window.DOMPurify) {
                const html = Array.isArray(data['text/html']) ? data['text/html'].join('') : data['text/html'];
                const frame = document.createElement('iframe'); frame.className = 'kg-html-output'; frame.setAttribute('sandbox', ''); frame.title = t('cell_output');
                const cleaned = window.DOMPurify.sanitize(html, { ALLOWED_TAGS: ['div', 'span', 'p', 'pre', 'code', 'table', 'thead', 'tbody', 'tr', 'th', 'td', 'b', 'strong', 'em', 'i', 'ul', 'ol', 'li', 'h1', 'h2', 'h3', 'br', 'hr'], ALLOWED_ATTR: ['colspan', 'rowspan'] });
                frame.srcdoc = '<!doctype html><meta http-equiv="Content-Security-Policy" content="default-src \'none\'; style-src \'unsafe-inline\'"><style>body{font:14px system-ui;color:#253042}table{border-collapse:collapse}td,th{border:1px solid #ccd1da;padding:6px}pre{white-space:pre-wrap}</style>' + cleaned; box.append(frame); return;
            }
            const pre = document.createElement('pre'); const text = data['text/plain']; pre.textContent = Array.isArray(text) ? text.join('') : text || t('inert_output'); box.append(pre);
        }
        function renderOutputs(cell, parent) { parent.replaceChildren(); (cell.outputs || []).forEach(out => output(parent, out)); }
        function focusTarget(key) {
            if (!key) return;
            const target = cells.querySelector('[data-focus="' + key + '"]');
            if (target) { target.focus({ preventScroll: true }); return; }
            cells.querySelector('[data-cell-id] .cm-content, [data-cell-id] textarea')?.focus({ preventScroll: true });
        }
        function focusCell(cellId) {
            const article = cells.querySelector('[data-cell-id="' + cellId + '"]');
            article?.querySelector('.cm-content, textarea')?.focus({ preventScroll: true });
        }
        function renderCells(focus) {
            editorViews.forEach(v => v.destroy()); editorViews = []; editorThemes.clear(); cells.replaceChildren();
            if (!doc.cells.length) {
                const empty = document.createElement('div'); empty.className = 'kg-empty';
                const hint = document.createElement('p'); hint.textContent = t('empty_notebook'); empty.append(hint);
                button('add_code', () => add('code'), empty, 'primary');
                cells.append(empty);
                return;
            }
            doc.cells.forEach((cell, index) => {
                const article = document.createElement('article'); article.className = 'kg-cell'; article.dataset.cellId = cell.id;
                const top = document.createElement('div'); top.className = 'kg-cell-toolbar'; article.append(top);
                const title = document.createElement('span'); title.className = 'kg-cell-title';
                const badge = document.createElement('span'); badge.className = 'kg-badge'; badge.dataset.kind = cell.cell_type === 'code' ? 'code' : 'markdown'; badge.textContent = cell.cell_type === 'code' ? 'Python' : 'Markdown';
                const position = document.createElement('span'); position.textContent = String(index + 1);
                title.append(badge, position);
                if (cell.cell_type === 'code' && cell.execution_count) { const count = document.createElement('span'); count.textContent = '[' + cell.execution_count + ']'; title.append(count); }
                top.append(title);
                const cellLabel = (cell.cell_type === 'code' ? 'Python' : 'Markdown') + ' · ' + (index + 1);
                if (cell.cell_type === 'code') button('run_cell', () => run(cell), top, 'primary');
                const up = button('up', () => move(index, -1), top, 'quiet'); up.dataset.focus = cell.id + ':up';
                const down = button('down', () => move(index, 1), top, 'quiet'); down.dataset.focus = cell.id + ':down';
                const remove = button('remove_cell', () => removeCell(index), top, 'quiet'); remove.dataset.focus = cell.id + ':remove';
                const editor = document.createElement('div'); editor.className = 'kg-cell-editor'; article.append(editor);
                const change = value => { cell.source = Array.isArray(cell.source) ? value.split(/(?<=\n)/) : value; edited(); };
                if (mirror) {
                    const theme = new mirror.Compartment();
                    const extensions = [mirror.lineNumbers(), mirror.history(), mirror.EditorView.contentAttributes.of({ 'aria-label': cellLabel }), mirror.keymap.of([...mirror.defaultKeymap, ...mirror.historyKeymap, mirror.indentWithTab, { key: 'Shift-Enter', run: () => { run(cell).catch(showError); return true; } }]), cell.cell_type === 'code' ? mirror.python() : mirror.markdown(), mirror.syntaxHighlighting(mirror.defaultHighlightStyle), mirror.EditorView.updateListener.of(update => { if (update.docChanged) change(update.state.doc.toString()); }), mirror.autocompletion({ override: cell.cell_type === 'code' ? [complete] : undefined }), mirror.EditorView.lineWrapping];
                    extensions.push(theme.of(lightTheme() ? [] : mirror.oneDark));
                    const view = new mirror.EditorView({ parent: editor, state: mirror.EditorState.create({ doc: source(cell), extensions }) });
                    editorViews.push(view); editorThemes.set(view, theme);
                } else { const input = document.createElement('textarea'); input.value = source(cell); input.setAttribute('aria-label', cellLabel); input.addEventListener('input', () => change(input.value)); editor.append(input); }
                const outputs = document.createElement('div'); outputs.className = 'kg-outputs'; article.append(outputs); renderOutputs(cell, outputs); cells.append(article);
            });
            if (focus) focusTarget(focus);
        }
        function add(kind) { const cell = { id: id(), cell_type: kind, metadata: {}, source: '' }; if (kind === 'code') Object.assign(cell, { execution_count: null, outputs: [] }); doc.cells.push(cell); edited(); renderCells(); focusCell(cell.id); }
        function move(index, delta) { const target = index + delta; if (target < 0 || target >= doc.cells.length) return; const moved = doc.cells[target].id; [doc.cells[index], doc.cells[target]] = [doc.cells[target], doc.cells[index]]; edited(); renderCells(); focusTarget(moved + ':' + (delta < 0 ? 'up' : 'down')); }
        function removeCell(index) { doc.cells.splice(index, 1); edited(); renderCells(); const next = doc.cells[index] || doc.cells[index - 1]; if (next) focusTarget(next.id + ':down'); else cells.querySelector('.kg-empty button')?.focus({ preventScroll: true }); }
        async function save() {
            if (savePending) return savePending;
            const savedVersion = contentVersion, snapshot = copy(doc);
            savePending = api(path, { method: 'PUT', headers: { 'If-Match': '"' + revision + '"' }, body: snapshot }).then(result => {
                revision = result.revision;
                // Retire only the exact recovered draft. Another window may have
                // advanced its own draft while this save was in flight.
                if (restoredDraft && localStorage.getItem(restoredDraft.key) === restoredDraft.raw) localStorage.removeItem(restoredDraft.key);
                restoredDraft = null;
                if (contentVersion === savedVersion) { dirty = false; localStorage.removeItem(draftKey); state.textContent = t('saved'); dirtyChip.hidden = true; }
                else { dirty = true; draft(); state.textContent = t('unsaved'); dirtyChip.hidden = false; }
                return result;
            }).catch(error => { draft(); throw error; }).finally(() => { savePending = null; });
            return savePending;
        }
        async function start() {
            if (startPending) return;
            if (session?.status === 'idle') { state.textContent = t('kernel_ready'); return; }
            if (session && session.status !== 'stopped') throw Error(session.error || t(session.status === 'starting' ? 'starting' : 'busy'));
            startPending = true;
            try {
            if (dirty) await save();
            const files = await options.selectFiles(project); if (!files) return;
            session = await api('/runtime', { method: 'POST', body: { operation: 'start', project_id: project, file, files, internet: false } });
            state.textContent = t('starting');
            syncKernelChip();
            applySession(earlySessions.get(session.id) || session);
            } finally { startPending = false; }
        }
        async function run(cell) {
            if (cell.cell_type !== 'code') return;
            if (running || pendingCommand) throw Error(t('busy'));
            if (!session || session.status !== 'idle') throw Error(t('start_kernel_first'));
            const runGeneration = generation;
            pendingCommand = true; let response, runRevision;
            try {
                if (dirty) await save();
                if (disposed || generation !== runGeneration) throw Error(t('unsaved'));
                runRevision = revision;
                response = await api('/runtime', { method: 'POST', body: { operation: 'execute', session_id: session.id, revision: runRevision, cell_id: cell.id } });
            } catch (error) { pendingCommand = false; earlyEvents.length = 0; throw error; }
            if (disposed) { pendingCommand = false; earlyEvents.length = 0; return; }
            const promise = new Promise((resolve, reject) => { running = { id: response.execution_id, cell, generation: runGeneration, revision: runRevision, resolve, reject }; });
            if (generation === runGeneration) { cell.outputs = []; contentVersion++; dirty = true; draft(); dirtyChip.hidden = false; const article = [...cells.children].find(c => c.dataset.cellId === cell.id); if (article) renderOutputs(cell, article.querySelector('.kg-outputs')); }
            session.status = 'busy'; syncKernelChip(); state.textContent = t('running_local'); pendingCommand = false; earlyEvents.splice(0).forEach(handleCell); return promise;
        }
        async function runAll() { runAllStopped = false; for (const cell of [...doc.cells]) { if (runAllStopped || disposed) break; if (cell.cell_type === 'code') await run(cell); } }
        async function interrupt() { runAllStopped = true; if (session) await api('/runtime', { method: 'POST', body: { operation: 'interrupt', session_id: session.id } }); }
        async function restart() {
            runAllStopped = true;
            if (!session) return start();
            if (startPending) return;
            startPending = true;
            try {
                session = await api('/runtime', { method: 'POST', body: { operation: 'restart', session_id: session.id } });
                syncKernelChip();
                applySession(earlySessions.get(session.id) || session);
                state.textContent = t('state_lost') + (session.status === 'idle' ? ' ' + t('kernel_ready') : '');
            } finally { startPending = false; }
        }
        async function complete(context) {
            if (!session || session.status !== 'idle' || running || pendingCommand) return null;
            const code = context.state.doc.toString(); pendingCommand = true; let result;
            try { result = await api('/runtime', { method: 'POST', body: { operation: 'complete', session_id: session.id, code, cursor: [...code.slice(0, context.pos)].length } }); } catch (_) { pendingCommand = false; earlyEvents.length = 0; return null; }
            if (disposed) { pendingCommand = false; earlyEvents.length = 0; return null; }
            return new Promise(resolve => { const timer = setTimeout(() => { completionWaiters.delete(result.execution_id); resolve(null); }, 12000); completionWaiters.set(result.execution_id, { resolve, timer, code }); pendingCommand = false; earlyEvents.splice(0).forEach(handleCell); });
        }
        function connectEvents() {
            eventSource = new EventSource(endpoint('/events'));
            eventSource.addEventListener('session', event => applySession(JSON.parse(event.data)));
            eventSource.addEventListener('cell', event => handleCell(JSON.parse(event.data)));
            eventSource.addEventListener('reset', () => { runAllStopped = true; state.textContent = t('output_limited'); if (running) { running.reject(Error(t('output_limited'))); running = null; } });
            eventSource.onerror = () => { if (!disposed) state.textContent = t('reconnecting'); };
        }
        function applySession(value) { if (value.project_id !== project || value.file !== file || disposed) return; earlySessions.set(value.id, value); if (earlySessions.size > 32) earlySessions.delete(earlySessions.keys().next().value); if (!session || value.id !== session.id) return; session = value; syncKernelChip(); if (value.status === 'idle') state.textContent = dirty ? t('unsaved') : t('kernel_ready'); if (value.status === 'stopped' || value.status === 'stopping') { state.textContent = t(value.status === 'stopping' ? 'state_cancel_requested' : 'state_lost') + (value.error ? ' ' + value.error : ''); if (running) { running.reject(Error(t('state_lost'))); running = null; } } }
        function handleCell(value) {
                if (pendingCommand) { if (earlyEvents.length < 256) earlyEvents.push(value); return; }
                const waiter = completionWaiters.get(value.execution_id);
                if (waiter && value.kind === 'completion') { clearTimeout(waiter.timer); completionWaiters.delete(value.execution_id); waiter.resolve({ from: [...waiter.code].slice(0, value.content.cursor_start).join('').length, options: (value.content.matches || []).map(label => ({ label, type: 'variable' })) }); return; }
                if (!running || value.execution_id !== running.id || value.session_id !== session?.id) return;
                if (value.kind === 'done' || value.kind === 'error') { const finished = running; running = null; session.status = 'idle'; syncKernelChip(); state.textContent = t('unsaved'); value.kind === 'done' ? finished.resolve() : finished.reject(Error(t('execution_failed'))); return; }
                if (running.generation !== generation || value.revision !== running.revision) return;
                const cell = doc.cells.find(c => c.id === value.cell_id); if (!cell) return;
                const content = value.content || {};
                if (value.output_type === 'clear_output') cell.outputs = [];
                else if (value.output_type === 'execute_input') cell.execution_count = content.execution_count;
                else if (value.kind === 'output') { const out = copy(content); out.output_type = value.output_type === 'update_display_data' ? 'display_data' : value.output_type; delete out.transient; cell.outputs.push(out); }
                else if (value.kind === 'truncated') cell.outputs.push({ output_type: 'stream', name: 'stderr', text: t('output_limited') });
                contentVersion++; dirty = true; draft(); dirtyChip.hidden = false; const article = [...cells.children].find(c => c.dataset.cellId === cell.id); if (article) renderOutputs(cell, article.querySelector('.kg-outputs'));
        }
        function exportNotebook() { const url = URL.createObjectURL(new Blob([JSON.stringify(doc, null, 2)], { type: 'application/x-ipynb+json' })); const link = document.createElement('a'); link.href = url; link.download = file; link.click(); setTimeout(() => URL.revokeObjectURL(url), 1000); }
        button('back', () => options.exit?.(), titleBar, 'quiet');
        button('save', save, documentGroup, 'primary'); button('export', () => exportNotebook(), documentGroup);
        button('add_code', () => add('code'), documentGroup); button('add_markdown', () => add('markdown'), documentGroup);
        button('local', start, runtimeGroup); button('run_all', runAll, runtimeGroup); button('interrupt', interrupt, runtimeGroup); button('restart', restart, runtimeGroup);
        button('kaggle_run', () => options.remote(copy(doc), revision), remoteGroup);
        root.addEventListener('keydown', event => {
            if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 's') { event.preventDefault(); save().catch(showError); return; }
            if (event.key === 'Escape' && !event.target.closest('.cm-editor, textarea, input, dialog')) { event.preventDefault(); options.exit?.(); }
        });
        const instance = { dispose() { if (disposed) return; if (dirty) draft(); disposed = true; runAllStopped = true; themeObserver.disconnect(); eventSource?.close(); editorViews.forEach(v => v.destroy()); editorThemes.clear(); completionWaiters.forEach(w => { clearTimeout(w.timer); w.resolve(null); }); completionWaiters.clear(); if (running) { running.reject(Error(t('window_closed'))); running = null; } }, save };
        await load().catch(showError); return instance;
    };
})();
