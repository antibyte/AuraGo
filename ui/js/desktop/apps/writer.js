(function () {
    'use strict';

    const DEFAULT_PATH = 'Documents/untitled.docx';
    const AUTO_SAVE_DEBOUNCE_MS = 800;
    const instances = new Map();

    function render(host, windowId, context) {
        if (!host) return;
        dispose(windowId);
        instances.set(windowId, { container: host });
        const ctx = context || {};
        const esc = ctx.esc || (value => String(value == null ? '' : value));
        const t = ctx.t || ((key, fallback) => fallback || key);
        const api = ctx.api || fetchJSON;
        const iconMarkup = ctx.iconMarkup || ((key, fallback) => `<span>${esc(fallback || key || '')}</span>`);
        const notify = ctx.notify || (() => {});
        const refreshDesktop = ctx.loadBootstrap || (() => Promise.resolve());
        const readonly = !!ctx.readonly;
        let currentPath = ctx.path || DEFAULT_PATH;
        let officeVersion = null;
        let editor = null;
        let isDirty = false;
        let autoSaveTimer = null;
        let searchState = null;

        host.innerHTML = `<div class="office-app office-writer" data-office-writer="${esc(windowId)}">
            <div class="vd-toolbar office-toolbar">
                <input class="office-path-input" data-path value="${esc(currentPath)}" spellcheck="false" autocomplete="off">
                <input class="office-title-input" data-title value="${esc(ctx.title || '')}" spellcheck="false" autocomplete="off" placeholder="${esc(t('desktop.writer_title_placeholder'))}">
                <span class="vd-chat-meta" data-status>${esc(t('desktop.writer_loading'))}</span>
            </div>
            <div class="office-writer-tools" data-quill-toolbar>
                <span class="ql-formats">
                    <select class="ql-header">
                        <option selected></option>
                        <option value="1"></option>
                        <option value="2"></option>
                        <option value="3"></option>
                    </select>
                    <select class="ql-font">
                        <option value="sans-serif" selected></option>
                        <option value="serif"></option>
                        <option value="monospace"></option>
                    </select>
                    <select class="ql-size">
                        <option value="small"></option>
                        <option value="normal" selected></option>
                        <option value="large"></option>
                        <option value="huge"></option>
                    </select>
                </span>
                <span class="ql-formats">
                    <button class="ql-bold" type="button"></button>
                    <button class="ql-italic" type="button"></button>
                    <button class="ql-underline" type="button"></button>
                    <select class="ql-color"></select>
                    <select class="ql-background"></select>
                </span>
                <span class="ql-formats">
                    <button class="ql-list" type="button" value="ordered"></button>
                    <button class="ql-list" type="button" value="bullet"></button>
                </span>
                <span class="ql-formats">
                    <button class="ql-align" type="button" value=""></button>
                    <button class="ql-align" type="button" value="center"></button>
                    <button class="ql-align" type="button" value="right"></button>
                    <button class="ql-align" type="button" value="justify"></button>
                </span>
                <span class="ql-formats">
                    <button class="ql-blockquote" type="button"></button>
                    <button class="ql-code-block" type="button"></button>
                    <button class="ql-link" type="button"></button>
                    <button class="ql-image" type="button"></button>
                </span>
                <span class="ql-formats">
                    <button class="ql-clean" type="button"></button>
                </span>
            </div>
            <div class="office-writer-editor" data-editor></div>
            <textarea class="office-writer-fallback" data-fallback spellcheck="true" hidden></textarea>
            <div class="office-writer-statusbar">
                <span class="office-writer-statusbar-left" data-statusbar-left></span>
                <span class="office-writer-statusbar-center" data-statusbar-center></span>
                <span class="office-writer-statusbar-right" data-statusbar-right></span>
            </div>
            <div class="office-writer-search-overlay" data-search-overlay hidden>
                <div class="office-writer-search-header">
                    <input class="office-writer-search-input" data-search-input placeholder="${esc(t('desktop.writer_search_placeholder'))}" autocomplete="off">
                    <span class="office-writer-search-count" data-search-count></span>
                    <button class="office-writer-search-btn" data-search-prev type="button" title="${esc(t('desktop.writer_find'))}">&#8593;</button>
                    <button class="office-writer-search-btn" data-search-next type="button" title="${esc(t('desktop.writer_find'))}">&#8595;</button>
                    <button class="office-writer-search-btn" data-search-close type="button" title="Esc">&#10005;</button>
                </div>
                <div class="office-writer-search-replace-row" data-search-replace-row>
                    <input class="office-writer-search-input" data-replace-input placeholder="${esc(t('desktop.writer_replace_placeholder'))}" autocomplete="off">
                    <button class="office-writer-search-btn" data-replace-one type="button">${esc(t('desktop.writer_replace'))}</button>
                    <button class="office-writer-search-btn" data-replace-all type="button">${esc(t('desktop.writer_replace_all'))}</button>
                </div>
                <label class="office-writer-search-label">
                    <input type="checkbox" data-search-match-case> ${esc(t('desktop.writer_match_case'))}
                </label>
            </div>
        </div>`;

        const pathInput = host.querySelector('[data-path]');
        const titleInput = host.querySelector('[data-title]');
        const statusNode = host.querySelector('[data-status]');
        const editorHost = host.querySelector('[data-editor]');
        const fallback = host.querySelector('[data-fallback]');
        const statusbarLeft = host.querySelector('[data-statusbar-left]');
        const statusbarCenter = host.querySelector('[data-statusbar-center]');
        const statusbarRight = host.querySelector('[data-statusbar-right]');
        const searchOverlay = host.querySelector('[data-search-overlay]');
        const searchInput = host.querySelector('[data-search-input]');
        const searchCount = host.querySelector('[data-search-count]');
        const searchReplaceRow = host.querySelector('[data-search-replace-row]');
        const replaceInput = host.querySelector('[data-replace-input]');
        const searchMatchCase = host.querySelector('[data-search-match-case]');
        if (typeof ctx.wireContextMenuBoundary === 'function') ctx.wireContextMenuBoundary(host);

        function setStatus(message, kind) {
            if (!statusNode) return;
            statusNode.textContent = message || '';
            if (kind) {
                statusNode.dataset.statusKind = kind;
            } else {
                delete statusNode.dataset.statusKind;
            }
        }

        function clearSaveError(statusNode) {
            if (!statusNode || statusNode.dataset.statusKind !== 'save-error') return;
            statusNode.textContent = '';
            delete statusNode.dataset.statusKind;
        }

        function setPath(path) {
            currentPath = path || DEFAULT_PATH;
            if (pathInput) pathInput.value = currentPath;
            updateExportLinks();
            if (typeof ctx.updateWindowContext === 'function') ctx.updateWindowContext(windowId, { path: currentPath });
        }

        function applyReadonlyState() {
            if (titleInput) titleInput.disabled = readonly;
            if (fallback) fallback.readOnly = readonly;
            if (editor && typeof editor.enable === 'function') editor.enable(!readonly);
            setWindowMenus();
        }

        function documentText() {
            if (editor) return editor.getText().replace(/\n$/, '');
            return fallback ? fallback.value : '';
        }

        function documentHTML() {
            if (editor) return editor.root.innerHTML;
            return textToHTML(fallback ? fallback.value : '');
        }

        function setDocumentText(text) {
            if (editor) {
                editor.setText(text || '');
            } else if (fallback) {
                fallback.value = text || '';
            }
        }

        function updateExportLinks() {
            setWindowMenus();
        }

        function exportURL(format) {
            const path = pathInput.value.trim() || DEFAULT_PATH;
            return '/api/desktop/office/export?path=' + encodeURIComponent(path) + '&format=' + encodeURIComponent(format);
        }

        async function openExport(format) {
            if (!pathInput.value.trim() && !readonly) await save();
            if (typeof ctx.exportDesktopFile === 'function') {
                const entry = currentFileEntry();
                const base = entry.name.replace(/\.[^.]+$/, '') || 'document';
                await ctx.exportDesktopFile({ path: entry.path, name: base + '.' + format, url: exportURL(format) });
                return;
            }
            window.open(exportURL(format), '_blank', 'noopener');
        }

        async function save() {
            if (readonly) return;
            clearSearchHighlights();
            setStatus(t('desktop.writer_saving'));
            const path = pathInput.value.trim() || DEFAULT_PATH;
            const payload = {
                path,
                title: titleInput.value.trim(),
                text: documentText(),
                html: documentHTML(),
                office_version: officeVersion
            };
            const body = await api('/api/desktop/office/document', {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(payload)
            });
            officeVersion = body.office_version || officeVersion;
            setPath(path);
            isDirty = false;
            updateDirtyState();
            setStatus(t('desktop.writer_saved'), 'saved');
            notify({ type: 'success', message: t('desktop.writer_saved') });
            await refreshDesktop();
            if (searchState && searchState.active) {
                highlightSearchMatches();
            }
        }

        function markDirty() {
            if (isDirty) return;
            isDirty = true;
            updateDirtyState();
            scheduleAutoSave();
        }

        function updateDirtyState() {
            const writerEl = host.querySelector('.office-writer');
            if (writerEl) {
                writerEl.classList.toggle('is-dirty', isDirty);
            }
            if (statusbarCenter) {
                if (isDirty) {
                    statusbarCenter.innerHTML = '<span class="dirty-dot"></span>' + esc(t('desktop.writer_unsaved'));
                } else {
                    statusbarCenter.textContent = '';
                }
            }
        }

        function scheduleAutoSave() {
            if (autoSaveTimer) clearTimeout(autoSaveTimer);
            if (readonly) return;
            autoSaveTimer = setTimeout(() => {
                autoSaveTimer = null;
                if (!isDirty) return;
                save().then(() => {
                    if (statusbarRight) statusbarRight.textContent = t('desktop.writer_auto_saved');
                }).catch(() => {
                    if (statusbarRight) statusbarRight.textContent = '';
                });
            }, AUTO_SAVE_DEBOUNCE_MS);
        }

        function updateWordCount() {
            if (!statusbarLeft) return;
            const text = documentText();
            const words = text.trim() ? text.trim().split(/\s+/).length : 0;
            const chars = text.length;
            const charsNoSpaces = text.replace(/\s/g, '').length;
            const pages = Math.max(1, Math.ceil(chars / 3000));
            statusbarLeft.textContent = words + ' ' + t('desktop.writer_words') + ' | ' + chars + ' ' + t('desktop.writer_chars') + ' | ' + pages + ' ' + t('desktop.writer_pages');
        }

        function newDocument() {
            closeSearch();
            officeVersion = null;
            isDirty = false;
            updateDirtyState();
            setPath(nextUntitledPath('.docx'));
            if (titleInput) titleInput.value = '';
            setDocumentText('');
            if (editor && editor.history && typeof editor.history.clear === 'function') editor.history.clear();
            setStatus('');
            updateWordCount();
        }

        async function openDocumentFromDialog() {
            if (typeof ctx.openFileDialog !== 'function') return;
            const result = await ctx.openFileDialog({
                title: t('desktop.file_dialog_open'),
                initialPath: pathDir(currentPath),
                filters: [{ label: t('desktop.app_writer'), extensions: ['.docx', '.html', '.htm', '.md', '.txt'] }]
            });
            if (!result || result.canceled || !result.path) return;
            setPath(result.path);
            await load();
        }

        async function saveAs() {
            if (readonly) return;
            if (typeof ctx.saveFileDialog === 'function') {
                const result = await ctx.saveFileDialog({
                    title: t('desktop.writer_save_as'),
                    initialPath: pathDir(pathInput.value.trim() || currentPath || DEFAULT_PATH),
                    defaultName: currentFileEntry().name,
                    defaultExtension: '.docx',
                    filters: [{ label: t('desktop.app_writer'), extensions: ['.docx', '.html', '.htm', '.md', '.txt'] }]
                });
                if (!result || result.canceled || !result.path) return;
                const previousPath = currentPath;
                const previousVersion = officeVersion;
                setPath(result.path);
                officeVersion = null;
                try {
                    await save();
                } catch (err) {
                    officeVersion = previousVersion;
                    setPath(previousPath);
                    throw err;
                }
                return;
            }
            const prompt = ctx.promptDialog || (async () => null);
            const nextPath = await prompt(t('desktop.writer_save_as'), pathInput.value.trim() || DEFAULT_PATH);
            if (nextPath == null) return;
            const trimmed = String(nextPath).trim();
            if (!trimmed) return;
            const previousPath = currentPath;
            const previousVersion = officeVersion;
            setPath(trimmed);
            officeVersion = null;
            try {
                await save();
            } catch (err) {
                officeVersion = previousVersion;
                setPath(previousPath);
                throw err;
            }
        }

        function editCommand(command) {
            if (editor && editor.root) editor.focus();
            else if (fallback) fallback.focus();
            if (document.execCommand) document.execCommand(command);
        }

        function selectAllText() {
            if (editor && typeof editor.setSelection === 'function') {
                editor.focus();
                editor.setSelection(0, Math.max(0, editor.getLength() - 1));
                return;
            }
            if (fallback) fallback.select();
        }

        function currentFileEntry() {
            const path = pathInput.value.trim() || currentPath || DEFAULT_PATH;
            return { path, name: path.split('/').filter(Boolean).pop() || path };
        }

        async function runAgentTask() {
            const prompt = ctx.promptDialog || (async () => null);
            const task = await prompt(t('desktop.agent_task_title'), '');
            if (!task) return;
            await save();
            if (typeof ctx.openAgentChatForFile === 'function') {
                ctx.openAgentChatForFile(currentFileEntry(), { task, autosend: true, sourceApp: 'writer' });
            }
        }

        async function sendToAgentChat() {
            await save();
            if (typeof ctx.openAgentChatForFile === 'function') {
                ctx.openAgentChatForFile(currentFileEntry(), { sourceApp: 'writer' });
            }
        }

        function setWindowMenus() {
            if (typeof ctx.setWindowMenus !== 'function') return;
            ctx.setWindowMenus(windowId, [
                {
                    id: 'file',
                    labelKey: 'desktop.menu_file',
                    items: [
                        { id: 'new-document', labelKey: 'desktop.writer_new', icon: 'file-plus', shortcut: 'Ctrl+N', disabled: readonly, action: newDocument },
                        { id: 'open-document', labelKey: 'desktop.file_dialog_open', icon: 'folder-open', shortcut: 'Ctrl+O', action: () => openDocumentFromDialog().catch(err => setStatus(err.message || String(err), 'save-error')) },
                        { id: 'save', labelKey: 'desktop.writer_save', icon: 'save', shortcut: 'Ctrl+S', disabled: readonly, action: () => save().catch(err => {
                            setStatus(err.message || String(err), 'save-error');
                            setTimeout(() => clearSaveError(statusNode), 6000);
                            notify({ type: 'error', message: err.message || String(err) });
                        }) },
                        { id: 'save-as', labelKey: 'desktop.writer_save_as', icon: 'save', disabled: readonly, action: () => saveAs().catch(err => {
                            setStatus(err.message || String(err), 'save-error');
                            setTimeout(() => clearSaveError(statusNode), 6000);
                            notify({ type: 'error', message: err.message || String(err) });
                        }) },
                        { type: 'separator' },
                        { id: 'download-docx', labelKey: 'desktop.writer_download_docx', icon: 'download', action: () => openExport('docx').catch(err => setStatus(err.message || String(err))) },
                        { id: 'export-html', labelKey: 'desktop.writer_export_html', icon: 'html', action: () => openExport('html').catch(err => setStatus(err.message || String(err))) },
                        { id: 'export-md', labelKey: 'desktop.writer_export_md', icon: 'markdown', action: () => openExport('md').catch(err => setStatus(err.message || String(err))) }
                    ]
                },
                {
                    id: 'edit',
                    labelKey: 'desktop.menu_edit',
                    items: [
                        { id: 'undo', labelKey: 'desktop.menu_undo', icon: 'undo', shortcut: 'Ctrl+Z', disabled: !editor || !editor.history, action: () => editor && editor.history && editor.history.undo() },
                        { id: 'redo', labelKey: 'desktop.menu_redo', icon: 'redo', shortcut: 'Ctrl+Y', disabled: !editor || !editor.history, action: () => editor && editor.history && editor.history.redo() },
                        { type: 'separator' },
                        { id: 'cut', labelKey: 'desktop.fm.cut', icon: 'scissors', shortcut: 'Ctrl+X', disabled: readonly, action: () => editCommand('cut') },
                        { id: 'copy', labelKey: 'desktop.fm.copy', icon: 'copy', shortcut: 'Ctrl+C', action: () => editCommand('copy') },
                        { id: 'paste', labelKey: 'desktop.fm.paste', icon: 'clipboard', shortcut: 'Ctrl+V', disabled: readonly, action: () => editCommand('paste') },
                        { type: 'separator' },
                        { id: 'find', labelKey: 'desktop.writer_find', icon: 'search', shortcut: 'Ctrl+F', action: () => openSearch(false) },
                        { id: 'replace', labelKey: 'desktop.writer_replace', icon: 'replace', shortcut: 'Ctrl+H', action: () => openSearch(true) },
                        { type: 'separator' },
                        { id: 'select-all', labelKey: 'desktop.fm.select_all', icon: 'check-square', shortcut: 'Ctrl+A', action: selectAllText }
                    ]
                },
                {
                    id: 'agent',
                    labelKey: 'desktop.menu_agent',
                    items: [
                        { id: 'agent-task', labelKey: 'desktop.agent_task_for_agent', icon: 'agent', action: () => runAgentTask().catch(err => {
                            setStatus(err.message || String(err), 'save-error');
                            setTimeout(() => clearSaveError(statusNode), 6000);
                            notify({ type: 'error', message: err.message || String(err) });
                        }) },
                        { id: 'agent-send-chat', labelKey: 'desktop.agent_send_to_chat', icon: 'chat', action: () => sendToAgentChat().catch(err => {
                            setStatus(err.message || String(err), 'save-error');
                            setTimeout(() => clearSaveError(statusNode), 6000);
                            notify({ type: 'error', message: err.message || String(err) });
                        }) }
                    ]
                }
            ]);
        }

        async function load() {
            if (!editor && window.Quill && editorHost) {
                try {
                    editor = new window.Quill(editorHost, {
                        modules: { toolbar: host.querySelector('[data-quill-toolbar]') },
                        placeholder: t('desktop.writer_placeholder'),
                        theme: 'snow'
                    });
                    editor.on('text-change', () => {
                        markDirty();
                        updateWordCount();
                        if (searchState && searchState.active) {
                            clearSearchHighlights();
                            searchState.matches = [];
                            searchState.currentMatch = -1;
                            updateSearchCount();
                        }
                    });
                } catch (_) {
                    editor = null;
                }
                const instAfter = instances.get(windowId);
                if (instAfter) instAfter.quill = editor;
                if (instAfter) instAfter.autoSaveTimer = null;
            }
            if (!editor) {
                editorHost.hidden = true;
                host.querySelector('[data-quill-toolbar]').hidden = true;
                fallback.hidden = false;
                fallback.addEventListener('input', () => {
                    markDirty();
                    updateWordCount();
                });
            }
            applyReadonlyState();
            updateExportLinks();
            try {
                const body = await api('/api/desktop/office/document?path=' + encodeURIComponent(currentPath));
                const doc = (body && body.document) || {};
                officeVersion = body.office_version || null;
                setDocumentText(doc.text || '');
                if (doc.title && titleInput && !titleInput.value) titleInput.value = doc.title;
                if (doc.html && editor && editor.clipboard && typeof editor.clipboard.dangerouslyPasteHTML === 'function') {
                    editor.clipboard.dangerouslyPasteHTML(doc.html);
                }
                setPath((body.entry && body.entry.path) || doc.path || currentPath);
                isDirty = false;
                updateDirtyState();
                setStatus('');
                updateWordCount();
            } catch (err) {
                officeVersion = null;
                setDocumentText(ctx.content || '');
                isDirty = false;
                updateDirtyState();
                setStatus('');
                updateWordCount();
                if (ctx.path) notify({ type: 'info', message: err.message || String(err) });
            }
        }

        // --- Search / Find & Replace ---

        function openSearch(showReplace) {
            if (!searchOverlay) return;
            searchState = { active: true, query: searchInput ? searchInput.value : '', replaceQuery: replaceInput ? replaceInput.value : '', matchCase: searchMatchCase ? searchMatchCase.checked : false, matches: [], currentMatch: -1 };
            searchOverlay.hidden = false;
            if (searchReplaceRow) searchReplaceRow.hidden = !showReplace;
            if (searchInput) { searchInput.value = searchState.query; searchInput.focus(); searchInput.select(); }
            if (replaceInput) replaceInput.value = searchState.replaceQuery;
            updateSearchCount();
        }

        function closeSearch() {
            clearSearchHighlights();
            searchState = null;
            if (searchOverlay) searchOverlay.hidden = true;
        }

        function doSearch() {
            if (!editor || !searchState) return;
            clearSearchHighlights();
            const query = (searchInput ? searchInput.value : searchState.query).trim();
            if (!query) {
                searchState.matches = [];
                searchState.currentMatch = -1;
                searchState.query = '';
                updateSearchCount();
                return;
            }
            searchState.query = query;
            searchState.matchCase = searchMatchCase ? searchMatchCase.checked : false;
            const text = editor.getText();
            const flags = searchState.matchCase ? 'g' : 'gi';
            const regex = new RegExp(escapeRegex(query), flags);
            searchState.matches = [];
            let match;
            while ((match = regex.exec(text)) !== null) {
                searchState.matches.push({ index: match.index, length: match[0].length });
            }
            if (searchState.matches.length > 0) {
                searchState.currentMatch = 0;
                highlightSearchMatches();
                navigateToMatch(0);
            } else {
                searchState.currentMatch = -1;
            }
            updateSearchCount();
        }

        function findNext() {
            if (!searchState || !searchState.matches.length) return;
            searchState.currentMatch = (searchState.currentMatch + 1) % searchState.matches.length;
            highlightSearchMatches();
            navigateToMatch(searchState.currentMatch);
        }

        function findPrev() {
            if (!searchState || !searchState.matches.length) return;
            searchState.currentMatch = searchState.currentMatch <= 0 ? searchState.matches.length - 1 : searchState.currentMatch - 1;
            highlightSearchMatches();
            navigateToMatch(searchState.currentMatch);
        }

        function replaceOne() {
            if (!editor || !searchState || !searchState.matches.length || searchState.currentMatch < 0) return;
            const replacement = replaceInput ? replaceInput.value : '';
            const match = searchState.matches[searchState.currentMatch];
            clearSearchHighlights();
            editor.deleteText(match.index, match.length, 'silent');
            editor.insertText(match.index, replacement, 'silent');
            markDirty();
            updateWordCount();
            doSearch();
        }

        function replaceAll() {
            if (!editor || !searchState || !searchState.matches.length) return;
            const replacement = replaceInput ? replaceInput.value : '';
            clearSearchHighlights();
            for (let i = searchState.matches.length - 1; i >= 0; i--) {
                const match = searchState.matches[i];
                editor.deleteText(match.index, match.length, 'silent');
                editor.insertText(match.index, replacement, 'silent');
            }
            markDirty();
            updateWordCount();
            searchState.matches = [];
            searchState.currentMatch = -1;
            updateSearchCount();
        }

        function highlightSearchMatches() {
            if (!editor || !searchState || !searchState.matches.length) return;
            const hlColor = 'rgba(250,204,21,0.35)';
            const currentColor = 'rgba(250,204,21,0.6)';
            searchState.matches.forEach((match, i) => {
                const color = i === searchState.currentMatch ? currentColor : hlColor;
                editor.formatText(match.index, match.length, 'background', color, 'silent');
            });
        }

        function clearSearchHighlights() {
            if (!editor || !searchState || !searchState.matches) return;
            const hlColor = 'rgba(250,204,21,0.35)';
            const currentColor = 'rgba(250,204,21,0.6)';
            searchState.matches.forEach(match => {
                try {
                    const formats = editor.getFormat(match.index, Math.min(1, match.length));
                    const bg = formats && formats.background;
                    if (bg === hlColor || bg === currentColor) {
                        editor.formatText(match.index, match.length, 'background', false, 'silent');
                    }
                } catch (_) { /* match position may be stale */ }
            });
        }

        function navigateToMatch(index) {
            if (!editor || !searchState || index < 0 || index >= searchState.matches.length) return;
            const match = searchState.matches[index];
            editor.setSelection(match.index, match.index + match.length, 'silent');
            try {
                const bounds = editor.getBounds(match.index, match.length);
                if (bounds) {
                    const editorEl = editor.root;
                    const container = editorEl.parentElement;
                    if (container) {
                        container.scrollTop = Math.max(0, bounds.top - container.clientHeight / 3);
                    }
                }
            } catch (_) { /* scroll may fail */ }
        }

        function updateSearchCount() {
            if (!searchCount) return;
            if (!searchState || !searchState.matches.length) {
                searchCount.textContent = searchState && searchState.query ? t('desktop.writer_no_results') : '';
                return;
            }
            searchCount.textContent = (searchState.currentMatch + 1) + '/' + searchState.matches.length;
        }

        // --- Event Listeners ---

        pathInput.addEventListener('change', () => {
            closeSearch();
            setPath(pathInput.value.trim() || DEFAULT_PATH);
            load();
        });

        if (titleInput) {
            titleInput.addEventListener('input', () => {
                markDirty();
            });
        }

        if (fallback) {
            fallback.addEventListener('input', () => {
                markDirty();
                updateWordCount();
            });
        }

        // Search overlay events
        if (searchInput) {
            searchInput.addEventListener('input', () => { doSearch(); });
            searchInput.addEventListener('keydown', (e) => {
                if (e.key === 'Enter') { e.preventDefault(); findNext(); }
                if (e.key === 'Escape') { e.preventDefault(); closeSearch(); }
            });
        }
        if (replaceInput) {
            replaceInput.addEventListener('keydown', (e) => {
                if (e.key === 'Enter') { e.preventDefault(); replaceOne(); }
                if (e.key === 'Escape') { e.preventDefault(); closeSearch(); }
            });
        }
        if (searchMatchCase) {
            searchMatchCase.addEventListener('change', () => { doSearch(); });
        }
        const searchPrevBtn = host.querySelector('[data-search-prev]');
        const searchNextBtn = host.querySelector('[data-search-next]');
        const searchCloseBtn = host.querySelector('[data-search-close]');
        const replaceOneBtn = host.querySelector('[data-replace-one]');
        const replaceAllBtn = host.querySelector('[data-replace-all]');
        if (searchPrevBtn) searchPrevBtn.addEventListener('click', findPrev);
        if (searchNextBtn) searchNextBtn.addEventListener('click', findNext);
        if (searchCloseBtn) searchCloseBtn.addEventListener('click', closeSearch);
        if (replaceOneBtn) replaceOneBtn.addEventListener('click', replaceOne);
        if (replaceAllBtn) replaceAllBtn.addEventListener('click', replaceAll);

        // Keyboard shortcuts for search
        host.addEventListener('keydown', (e) => {
            if ((e.ctrlKey || e.metaKey) && e.key === 'f') {
                e.preventDefault();
                openSearch(false);
                return;
            }
            if ((e.ctrlKey || e.metaKey) && e.key === 'h') {
                e.preventDefault();
                openSearch(true);
                return;
            }
            if (e.key === 'Escape' && searchState && searchState.active) {
                e.preventDefault();
                closeSearch();
                return;
            }
        });

        setWindowMenus();
        load();
    }

    function dispose(windowId) {
        const instance = instances.get(windowId);
        if (instance) {
            if (instance.autoSaveTimer) clearTimeout(instance.autoSaveTimer);
            if (instance.quill && typeof instance.quill.disable === 'function') {
                try { instance.quill.disable(); } catch (_) {}
            }
        }
        instances.delete(windowId);
    }

    async function fetchJSON(url, options) {
        const resp = await fetch(url, options);
        const body = await resp.json().catch(() => ({}));
        if (!resp.ok) throw new Error(body.error || body.message || ('HTTP ' + resp.status));
        return body;
    }

    function textToHTML(text) {
        return '<p>' + String(text || '').split('\n').map(escapeHTML).join('</p><p>') + '</p>';
    }

    function nextUntitledPath(ext) {
        const stamp = new Date().toISOString().replace(/[-:]/g, '').replace(/\..+$/, '').replace('T', '-');
        return 'Documents/untitled-' + stamp + ext;
    }

    function pathDir(path) {
        const parts = String(path || '').split('/').filter(Boolean);
        parts.pop();
        return parts.join('/') || 'Documents';
    }

    function escapeHTML(value) {
        return String(value == null ? '' : value)
            .replaceAll('&', '&amp;')
            .replaceAll('<', '&lt;')
            .replaceAll('>', '&gt;')
            .replaceAll('"', '&quot;')
            .replaceAll("'", '&#39;');
    }

    function escapeRegex(value) {
        return String(value || '').replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
    }

    window.WriterApp = window.WriterApp || {};
    window.WriterApp.render = render;
    window.WriterApp.dispose = dispose;
})();
