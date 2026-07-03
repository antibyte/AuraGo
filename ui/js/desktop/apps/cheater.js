(function () {
    'use strict';

    const instances = new Map();
    const SAVE_DEBOUNCE_MS = 500;
    const PREVIEW_DEBOUNCE_MS = 150;
    const POLL_INTERVAL_MS = 30000;
    const SPOTLIGHT_KEY = 'k';
    const VIEW_MODE_KEY = 'cheater.viewMode';
    const DEFAULT_VIEW_MODE = 'edit';

    function normalizeSheetEntries(list) {
        const items = Array.isArray(list) ? list : (list && (list.items || list.cheatsheets)) || [];
        return items.map(s => ({
            id: s.id,
            name: s.name,
            abstract: s.abstract || '',
            tags: s.tags || [],
            content_excerpt: (s.content || '').slice(0, 200),
            last_used_at: s.last_used_at || null,
            updated_at: s.updated_at || null
        })).sort((a, b) => {
            const aTime = Date.parse(a.updated_at || a.last_used_at || 0) || 0;
            const bTime = Date.parse(b.updated_at || b.last_used_at || 0) || 0;
            return bTime - aTime;
        });
    }

    function filterEntries(entries, query) {
        const q = String(query || '').trim().toLowerCase();
        if (!q) return entries;
        return entries.filter(entry => {
            const name = (entry.name || '').toLowerCase();
            const abstract = (entry.abstract || '').toLowerCase();
            const tags = (entry.tags || []).join(' ').toLowerCase();
            const content = (entry.content_excerpt || '').toLowerCase();
            return name.includes(q) || abstract.includes(q) || tags.includes(q) || content.includes(q);
        });
    }

    function render(host, windowId, context) {
        if (!host) return;
        const ctx = context || {};
        const esc = ctx.esc || (value => String(value == null ? '' : value));
        const t = ctx.t || ((key, fallback) => fallback || key);
        const api = ctx.api || fetchJSON;
        const notify = ctx.notify || (() => {});
        const readonly = !!ctx.readonly;

        const state = {
            host,
            windowId,
            esc,
            t,
            api,
            notify,
            iconMarkup: ctx.iconMarkup,
            readonly,
            sheet: null,
            dirty: false,
            lastSavedAt: null,
            saveTimer: null,
            currentAbort: null,
            pollTimer: null,
            searchIndex: [],
            viewMode: loadViewMode(),
            previewTimer: null,
            openSheet: (nextState, entry) => {
                if (entry && entry.id) {
                    loadSheet(nextState, entry.id);
                    return;
                }
                openSheet(nextState, entry);
            },
            refreshHome: () => refreshHome(state)
        };
        instances.set(windowId, state);

        if (typeof ctx.wireContextMenuBoundary === 'function') ctx.wireContextMenuBoundary(host);
        bindGlobalShortcuts(state);
        renderLoading(state);
        loadSearchIndex(state);
    }

    function loadViewMode() {
        try {
            const v = localStorage.getItem(VIEW_MODE_KEY);
            if (v === 'edit' || v === 'split' || v === 'preview') return v;
        } catch (_) {}
        return DEFAULT_VIEW_MODE;
    }

    function saveViewMode(mode) {
        try { localStorage.setItem(VIEW_MODE_KEY, mode); } catch (_) {}
    }

    function renderLoading(state) {
        const { host, windowId, esc, t } = state;
        host.innerHTML = `<section class="cheater-app" data-cheater="${esc(windowId)}" data-state="loading" tabindex="-1">
            <div class="cheater-loading" aria-busy="true">
                <div class="cheater-loading-grid" aria-hidden="true">
                    <div class="cheater-loading-card"></div>
                    <div class="cheater-loading-card"></div>
                    <div class="cheater-loading-card"></div>
                </div>
                <p class="cheater-loading-label">${esc(t('cheater.library_loading'))}</p>
            </div>
        </section>`;
        focusAppSurface(host);
    }

    function renderEmpty(state) {
        const { host, windowId, esc, t, iconMarkup } = state;
        const icon = iconMarkup ? iconMarkup('cheater', '🗂️') : '🗂️';
        host.innerHTML = `<section class="cheater-app" data-cheater="${esc(windowId)}" data-state="empty" tabindex="-1">
            <div class="cheater-empty" data-empty>
                <div class="cheater-empty-icon" aria-hidden="true">${icon}</div>
                <h1 class="cheater-empty-title">${esc(t('cheater.app_name'))}</h1>
                <p class="cheater-empty-subtitle">${esc(t('cheater.empty_subtitle'))}</p>
                <button type="button" class="cheater-primary" data-action="create">${esc(t('cheater.empty_cta'))}</button>
                <p class="cheater-empty-hint">${esc(t('cheater.empty_hint'))}</p>
            </div>
        </section>`;
        bindCreateButton(state);
        focusAppSurface(host);
    }

    function renderLibrary(state) {
        const { host, windowId, esc, t } = state;
        const countLabel = t('cheater.library_count').replace('{{count}}', String(state.searchIndex.length));
        host.innerHTML = `<section class="cheater-app" data-cheater="${esc(windowId)}" data-state="library" tabindex="-1">
            <header class="cheater-library-header">
                <div class="cheater-library-intro">
                    <h1 class="cheater-library-title">${esc(t('cheater.app_name'))}</h1>
                    <p class="cheater-library-subtitle">${esc(t('cheater.empty_subtitle'))}</p>
                </div>
                <div class="cheater-library-actions">
                    <button type="button" class="cheater-secondary" data-action="spotlight" title="${esc(t('cheater.library_open_spotlight'))}">
                        ${esc(t('cheater.library_open_spotlight'))}
                        <kbd class="cheater-kbd">Ctrl+Shift+K</kbd>
                    </button>
                    <button type="button" class="cheater-primary" data-action="create">${esc(t('cheater.create_title'))}</button>
                </div>
            </header>
            <div class="cheater-library-toolbar">
                <label class="cheater-library-search">
                    <span class="cheater-library-search-label">${esc(t('cheater.spotlight_input_label'))}</span>
                    <input type="search" data-library-filter placeholder="${esc(t('cheater.library_search_placeholder'))}" autocomplete="off" spellcheck="false">
                </label>
                <span class="cheater-library-count" data-library-count>${esc(countLabel)}</span>
            </div>
            <ul class="cheater-library-list" data-library-list role="list"></ul>
        </section>`;
        state.libraryFilter = '';
        renderLibraryList(state, state.searchIndex);
        bindLibraryEvents(state);
        bindCreateButton(state);
        const filterInput = host.querySelector('[data-library-filter]');
        if (filterInput) filterInput.focus();
    }

    function renderLibraryList(state, entries) {
        const list = state.host.querySelector('[data-library-list]');
        const countNode = state.host.querySelector('[data-library-count]');
        if (!list) return;
        const { esc, t } = state;
        if (countNode) {
            countNode.textContent = t('cheater.library_count').replace('{{count}}', String(entries.length));
        }
        if (!entries.length) {
            list.innerHTML = `<li class="cheater-library-empty">${esc(t('cheater.library_no_results'))}</li>`;
            return;
        }
        list.innerHTML = entries.map(entry => {
            const tags = (entry.tags || []).slice(0, 4).map(tag => `<span class="cheater-pill">${esc(tag)}</span>`).join('');
            const updatedMeta = entry.updated_at
                ? `<span class="cheater-library-meta">${esc(formatRelative(entry.updated_at, t))}</span>`
                : '';
            const agentBadge = entry.last_used_at
                ? `<span class="cheater-agent-badge cheater-library-agent-badge" data-agent-badge>🤖 ${esc(formatRelative(entry.last_used_at, t))}</span>`
                : '';
            return `<li class="cheater-library-card" role="listitem">
                <button type="button" class="cheater-library-card-btn" data-sheet-id="${esc(entry.id)}">
                    <span class="cheater-library-card-title">${esc(entry.name || t('cheater.untitled_sheet'))}</span>
                    ${entry.abstract ? `<span class="cheater-library-card-abstract">${esc(entry.abstract)}</span>` : ''}
                    <span class="cheater-library-card-footer">${tags}${updatedMeta}${agentBadge}</span>
                </button>
            </li>`;
        }).join('');
        list.querySelectorAll('[data-sheet-id]').forEach(btn => {
            btn.addEventListener('click', () => loadSheet(state, btn.dataset.sheetId));
        });
    }

    function bindLibraryEvents(state) {
        const filterInput = state.host.querySelector('[data-library-filter]');
        const spotlightBtn = state.host.querySelector('[data-action="spotlight"]');
        if (filterInput) {
            filterInput.addEventListener('input', () => {
                state.libraryFilter = filterInput.value;
                renderLibraryList(state, filterEntries(state.searchIndex, state.libraryFilter));
            });
        }
        if (spotlightBtn) {
            spotlightBtn.addEventListener('click', () => openSpotlight(state));
        }
    }

    function bindCreateButton(state) {
        const createBtn = state.host.querySelector('[data-action="create"]');
        if (!createBtn) return;
        createBtn.addEventListener('click', () => {
            if (typeof window.CheaterApp.openCreateModal === 'function') {
                window.CheaterApp.openCreateModal(state.windowId);
            }
        });
    }

    function focusAppSurface(host) {
        const surface = host.querySelector('[data-cheater]');
        if (surface && typeof surface.focus === 'function') surface.focus();
    }

    async function loadSearchIndex(state) {
        try {
            const list = await state.api('/api/cheatsheets');
            state.searchIndex = normalizeSheetEntries(list);
        } catch (err) {
            console.warn('cheater search index load failed', err);
            state.searchIndex = [];
        }
        if (!state.sheet) refreshHome(state);
    }

    function refreshHome(state) {
        if (state.sheet) return;
        stopPolling(state);
        if (!state.searchIndex.length) renderEmpty(state);
        else renderLibrary(state);
        bindGlobalShortcuts(state);
    }

    function openSpotlight(state) {
        if (window.CheaterSpotlight && typeof window.CheaterSpotlight.open === 'function') {
            window.CheaterSpotlight.open(state);
        }
    }

    function bindGlobalShortcuts(state) {
        if (state._shortcutHandler) {
            state.host.removeEventListener('keydown', state._shortcutHandler);
        }
        state._shortcutHandler = (e) => {
            if ((e.ctrlKey || e.metaKey) && e.shiftKey && (e.key === SPOTLIGHT_KEY || e.key === SPOTLIGHT_KEY.toUpperCase())) {
                e.preventDefault();
                openSpotlight(state);
            } else if ((e.ctrlKey || e.metaKey) && !e.shiftKey && (e.key === 'n' || e.key === 'N')) {
                e.preventDefault();
                if (window.CheaterApp && typeof window.CheaterApp.openCreateModal === 'function') {
                    window.CheaterApp.openCreateModal(state.windowId);
                }
            }
        };
        state.host.addEventListener('keydown', state._shortcutHandler);
    }

    function renderEditor(state, sheet) {
        const { host, windowId, esc, t, readonly, iconMarkup } = state;
        state.sheet = sheet;
        state.dirty = false;
        state.lastSavedAt = sheet.updated_at || null;

        const backIcon = iconMarkup ? iconMarkup('back', '←') : '←';
        const attachIcon = iconMarkup ? iconMarkup('attach', '📎') : '📎';
        const attachCount = (sheet.attachments || []).length;
        const viewMode = state.viewMode;

        host.innerHTML = `<section class="cheater-app" data-cheater="${esc(windowId)}" data-state="editor" data-view="${esc(viewMode)}">
            <header class="cheater-header">
                <button type="button" class="cheater-back" data-action="back" aria-label="${esc(t('cheater.back'))}">${backIcon}</button>
                <input class="cheater-title" data-title type="text" value="${esc(sheet.name || '')}" spellcheck="false" autocomplete="off"${readonly ? ' readonly' : ''}>
                <span class="cheater-save" data-save>${esc(t('cheater.saved'))}</span>
                <button type="button" class="cheater-save-btn cheater-secondary" data-action="save" title="${esc(t('cheater.save'))} (Ctrl+S)">${esc(t('cheater.save'))}</button>
                <span class="cheater-agent-badge" data-agent-badge hidden></span>
                <button type="button" class="cheater-attach-btn" data-action="attachments" aria-label="${esc(t('cheater.attachments'))}">${attachIcon} <span data-attach-count>${attachCount}</span></button>
            </header>
            <div class="cheater-toolbar" data-toolbar></div>
            <div class="cheater-view-toggle" role="tablist">
                <button type="button" class="cheater-view-btn${viewMode === 'edit' ? ' is-active' : ''}" data-view-mode="edit" role="tab">${esc(t('cheater.view_mode_edit'))}</button>
                <button type="button" class="cheater-view-btn${viewMode === 'split' ? ' is-active' : ''}" data-view-mode="split" role="tab">${esc(t('cheater.view_mode_split'))}</button>
                <button type="button" class="cheater-view-btn${viewMode === 'preview' ? ' is-active' : ''}" data-view-mode="preview" role="tab">${esc(t('cheater.view_mode_preview'))}</button>
            </div>
            <div class="cheater-content" data-content>
                <textarea class="cheater-source" data-source spellcheck="true" placeholder="${esc(t('cheater.editor_placeholder'))}"${readonly ? ' readonly' : ''}>${esc(sheet.content || '')}</textarea>
                <div class="cheater-preview" data-preview aria-live="polite"></div>
            </div>
            <footer class="cheater-footer">
                <span data-charcount>${(sheet.content || '').length}</span> ${esc(t('cheater.chars'))} ·
                <span data-wordcount>${countWords(sheet.content || '')}</span> ${esc(t('cheater.words'))} ·
                <span data-linecount>${countLines(sheet.content || '')}</span> ${esc(t('cheater.lines'))}
                <span class="cheater-footer-spacer"></span>
                <span data-help>${esc(t('cheater.editor_help'))}</span>
            </footer>
        </section>`;
    }

    function countWords(text) {
        const m = String(text || '').trim().match(/\S+/g);
        return m ? m.length : 0;
    }

    function countLines(text) {
        const s = String(text || '');
        if (!s.length) return 0;
        return s.split('\n').length;
    }

    function startPolling(state) {
        if (state.pollTimer) return;
        state.pollTimer = setInterval(() => pollRemote(state), POLL_INTERVAL_MS);
    }

    function stopPolling(state) {
        if (state.pollTimer) { clearInterval(state.pollTimer); state.pollTimer = null; }
    }

    async function pollRemote(state) {
        if (!state.sheet) return;
        try {
            const fresh = await state.api('/api/cheatsheets/' + encodeURIComponent(state.sheet.id));
            if (!fresh) return;
            if (fresh.updated_at && state.sheet.updated_at && fresh.updated_at > state.sheet.updated_at && !state.dirty) {
                showUpdateBadge(state, fresh);
            }
        } catch (err) {
            console.warn('cheater poll failed', err);
        }
    }

    function showUpdateBadge(state, fresh) {
        if (state.host.querySelector('[data-update-badge]')) return;
        const bar = document.createElement('div');
        bar.className = 'cheater-update-bar';
        bar.dataset.updateBadge = '1';
        const t = state.t;
        const esc = state.esc;
        const summary = buildUpdateSummary(state.sheet, fresh, t);
        bar.innerHTML = `<span>${esc(t('cheater.update_available'))}</span>${summary ? `<span class="cheater-update-summary">${esc(summary)}</span>` : ''}<button type="button" data-action="apply">${esc(t('cheater.update_apply'))}</button><button type="button" data-action="dismiss" aria-label="${esc(t('cheater.close'))}">×</button>`;
        bar.querySelector('[data-action="apply"]').addEventListener('click', () => {
            openSheet(state, fresh);
            bar.remove();
        });
        bar.querySelector('[data-action="dismiss"]').addEventListener('click', () => bar.remove());
        const content = state.host.querySelector('.cheater-content');
        if (content) content.prepend(bar);
    }

    function buildUpdateSummary(current, fresh, t) {
        const curLen = (current && current.content ? current.content.length : 0);
        const freshLen = (fresh && fresh.content ? fresh.content.length : 0);
        const delta = freshLen - curLen;
        if (delta === 0) return t('cheater.update_changed');
        if (delta > 0) return t('cheater.update_added').replace('{{n}}', String(delta));
        return t('cheater.update_removed').replace('{{n}}', String(Math.abs(delta)));
    }

    function dispose(windowId) {
        const state = instances.get(windowId);
        if (state) {
            if (state.saveTimer) clearTimeout(state.saveTimer);
            if (state.pollTimer) clearInterval(state.pollTimer);
            if (state.previewTimer) clearTimeout(state.previewTimer);
            if (state.currentAbort) state.currentAbort.abort();
            if (state._shortcutHandler && state.host) {
                state.host.removeEventListener('keydown', state._shortcutHandler);
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

    async function loadSheet(state, id) {
        try {
            const data = await state.api('/api/cheatsheets/' + encodeURIComponent(id));
            openSheet(state, data);
        } catch (err) {
            state.notify('cheater.error.load_failed', 'error');
            console.error('cheater loadSheet failed', err);
        }
    }

    function openSheet(state, sheet) {
        if (!sheet) {
            state.sheet = null;
            refreshHome(state);
            return;
        }
        renderEditor(state, sheet);
        bindEditorEvents(state);
        mountToolbar(state);
        renderPreview(state);
        renderAgentBadge(state);
        bindBackButton(state);
        bindViewToggle(state);
        bindGlobalShortcuts(state);
        startPolling(state);
    }

    function mountToolbar(state) {
        const slot = state.host.querySelector('[data-toolbar]');
        if (!slot) return;
        if (window.CheaterToolbar && typeof window.CheaterToolbar.mount === 'function') {
            window.CheaterToolbar.mount(state, slot);
        } else {
            slot.hidden = true;
        }
    }

    function bindViewToggle(state) {
        const buttons = state.host.querySelectorAll('[data-view-mode]');
        buttons.forEach(btn => {
            btn.addEventListener('click', () => {
                const mode = btn.dataset.viewMode;
                setViewMode(state, mode);
            });
        });
    }

    function setViewMode(state, mode) {
        if (mode !== 'edit' && mode !== 'split' && mode !== 'preview') return;
        state.viewMode = mode;
        saveViewMode(mode);
        const section = state.host.querySelector('[data-cheater]');
        if (section) section.dataset.view = mode;
        state.host.querySelectorAll('[data-view-mode]').forEach(btn => {
            btn.classList.toggle('is-active', btn.dataset.viewMode === mode);
        });
        const source = state.host.querySelector('[data-source]');
        if (mode === 'preview' && source) source.blur();
        if (mode === 'edit' && source) source.focus();
        renderPreview(state);
    }

    function bindEditorEvents(state) {
        const titleInput = state.host.querySelector('[data-title]');
        const source = state.host.querySelector('[data-source]');
        const charCount = state.host.querySelector('[data-charcount]');
        const wordCount = state.host.querySelector('[data-wordcount]');
        const lineCount = state.host.querySelector('[data-linecount]');

        if (titleInput) {
            titleInput.addEventListener('input', () => {
                if (state.sheet) state.sheet.name = titleInput.value;
                markDirty(state);
                scheduleSave(state);
            });
        }
        if (source) {
            source.addEventListener('input', () => {
                const text = source.value;
                if (state.sheet) state.sheet.content = text;
                if (charCount) charCount.textContent = String(text.length);
                if (wordCount) wordCount.textContent = String(countWords(text));
                if (lineCount) lineCount.textContent = String(countLines(text));
                markDirty(state);
                scheduleSave(state);
                schedulePreview(state);
            });
            source.addEventListener('keydown', (e) => {
                if (e.key === 'Tab') {
                    e.preventDefault();
                    handleTabIndent(source, e.shiftKey);
                }
            });
        }
        state.host.addEventListener('keydown', (e) => {
            if ((e.ctrlKey || e.metaKey) && (e.key === 's' || e.key === 'S')) {
                e.preventDefault();
                flushSave(state);
            } else if ((e.ctrlKey || e.metaKey) && e.shiftKey && (e.key === 'e' || e.key === 'E')) {
                e.preventDefault();
                cycleViewMode(state);
            }
        });
        const attachBtn = state.host.querySelector('[data-action="attachments"]');
        if (attachBtn) {
            attachBtn.addEventListener('click', () => {
                if (window.CheaterAttachments && typeof window.CheaterAttachments.open === 'function') {
                    window.CheaterAttachments.open(state);
                }
            });
        }
        const saveBtn = state.host.querySelector('[data-action="save"]');
        if (saveBtn) {
            saveBtn.addEventListener('click', () => flushSave(state));
        }
    }

    function handleTabIndent(textarea, reverse) {
        const start = textarea.selectionStart;
        const end = textarea.selectionEnd;
        const value = textarea.value;
        if (start === end) {
            if (reverse) return;
            textarea.setRangeText('    ', start, end, 'end');
            return;
        }
        const lineStart = value.lastIndexOf('\n', start - 1) + 1;
        const before = value.slice(0, lineStart);
        const block = value.slice(lineStart, end);
        const after = value.slice(end);
        if (reverse) {
            const dedented = block.split('\n').map(line => line.startsWith('    ') ? line.slice(4) : (line.startsWith('\t') ? line.slice(1) : line)).join('\n');
            textarea.value = before + dedented + after;
            textarea.selectionStart = lineStart;
            textarea.selectionEnd = lineStart + dedented.length;
        } else {
            const indented = block.split('\n').map(line => '    ' + line).join('\n');
            textarea.value = before + indented + after;
            textarea.selectionStart = lineStart;
            textarea.selectionEnd = lineStart + indented.length;
        }
        textarea.dispatchEvent(new Event('input'));
    }

    function cycleViewMode(state) {
        const order = ['edit', 'split', 'preview'];
        const idx = order.indexOf(state.viewMode);
        setViewMode(state, order[(idx + 1) % order.length]);
    }

    function schedulePreview(state) {
        if (state.previewTimer) clearTimeout(state.previewTimer);
        state.previewTimer = setTimeout(() => renderPreview(state), PREVIEW_DEBOUNCE_MS);
    }

    function sanitizePreviewHTML(html) {
        if (window.DOMPurify && typeof window.DOMPurify.sanitize === 'function') {
            return window.DOMPurify.sanitize(html, { USE_PROFILES: { html: true } });
        }
        return escHtml(html);
    }

    function renderPreview(state) {
        const preview = state.host.querySelector('[data-preview]');
        if (!preview) return;
        if (state.viewMode === 'edit') return;
        const content = (state.sheet && state.sheet.content) || '';
        try {
            const rendered = window.marked ? window.marked.parse(content, { gfm: true, breaks: false }) : escHtml(content);
            preview.innerHTML = sanitizePreviewHTML(rendered);
            if (window.hljs && window.hljs.highlightElement) {
                preview.querySelectorAll('pre code').forEach(c => window.hljs.highlightElement(c));
            }
        } catch (err) {
            console.error('cheater preview render failed', err);
            preview.textContent = content;
        }
    }

    function bindBackButton(state) {
        const back = state.host.querySelector('[data-action="back"]');
        if (!back) return;
        back.addEventListener('click', () => {
            if (state.dirty) flushSave(state).finally(() => goBackToEmpty(state));
            else goBackToEmpty(state);
        });
    }

    function goBackToEmpty(state) {
        if (state.saveTimer) clearTimeout(state.saveTimer);
        if (state.previewTimer) clearTimeout(state.previewTimer);
        state.sheet = null;
        refreshHome(state);
    }

    function renderAgentBadge(state) {
        const node = state.host.querySelector('[data-agent-badge]');
        if (!node || !state.sheet) return;
        const lastUsed = state.sheet.last_used_at;
        if (!lastUsed) {
            node.hidden = true;
            return;
        }
        const t = state.t;
        node.hidden = false;
        node.textContent = '🤖 ' + t('cheater.agent_badge').replace('{{time}}', formatRelative(lastUsed, t));
    }

    function markDirty(state) {
        state.dirty = true;
        renderSaveStatus(state, 'saving');
    }

    function scheduleSave(state) {
        if (state.saveTimer) clearTimeout(state.saveTimer);
        state.saveTimer = setTimeout(() => commitSave(state), SAVE_DEBOUNCE_MS);
    }

    async function flushSave(state) {
        if (state.saveTimer) {
            clearTimeout(state.saveTimer);
            state.saveTimer = null;
        }
        if (state.dirty) await commitSave(state);
    }

    async function commitSave(state) {
        if (!state.sheet || !state.dirty) return;
        const sheet = state.sheet;
        const aborter = new AbortController();
        state.currentAbort = aborter;
        renderSaveStatus(state, 'saving');
        try {
            const body = {
                name: sheet.name,
                content: sheet.content
            };
            const updated = await state.api('/api/cheatsheets/' + encodeURIComponent(sheet.id), {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(body),
                signal: aborter.signal
            });
            if (state.currentAbort !== aborter) return; // stale
            state.dirty = false;
            state.lastSavedAt = updated.updated_at || new Date().toISOString();
            state.sheet = Object.assign({}, sheet, updated);
            renderSaveStatus(state, 'saved');
            updateSearchIndexEntry(state, state.sheet);
        } catch (err) {
            if (err && err.name === 'AbortError') return;
            renderSaveStatus(state, 'error');
            state.notify('cheater.error.save_failed', 'error');
            console.error('cheater save failed', err);
        }
    }

    function renderSaveStatus(state, kind) {
        const node = state.host.querySelector('[data-save]');
        if (!node) return;
        const t = state.t;
        if (kind === 'saving') {
            node.dataset.state = 'saving';
            node.textContent = t('cheater.saving');
        } else if (kind === 'error') {
            node.dataset.state = 'error';
            node.textContent = t('cheater.save_error');
        } else {
            delete node.dataset.state;
            const last = state.lastSavedAt;
            node.textContent = t('cheater.saved_ago').replace('{{time}}', formatRelative(last, t));
        }
    }

    function formatRelative(iso, t) {
        if (!iso) return t('cheater.just_now');
        const then = new Date(iso).getTime();
        if (Number.isNaN(then)) return t('cheater.just_now');
        const diff = Math.max(0, Date.now() - then);
        const sec = Math.floor(diff / 1000);
        if (sec < 60) return t('cheater.seconds_ago').replace('{{n}}', String(sec));
        const min = Math.floor(sec / 60);
        if (min < 60) return t('cheater.minutes_ago').replace('{{n}}', String(min));
        const hr = Math.floor(min / 60);
        if (hr < 24) return t('cheater.hours_ago').replace('{{n}}', String(hr));
        const day = Math.floor(hr / 24);
        return t('cheater.days_ago').replace('{{n}}', String(day));
    }

    function formatRelativeShort(iso, t) {
        if (!iso) return '';
        const then = new Date(iso).getTime();
        if (Number.isNaN(then)) return '';
        const diff = Math.max(0, Date.now() - then);
        const min = Math.floor(diff / 60000);
        if (min < 1) return t('cheater.just_now');
        if (min < 60) return t('cheater.minutes_ago_short').replace('{{n}}', String(min));
        const hr = Math.floor(min / 60);
        if (hr < 24) return t('cheater.hours_ago_short').replace('{{n}}', String(hr));
        const day = Math.floor(hr / 24);
        return t('cheater.days_ago_short').replace('{{n}}', String(day));
    }

    function updateSearchIndexEntry(state, sheet) {
        if (!sheet) return;
        const idx = state.searchIndex.findIndex(s => s.id === sheet.id);
        const entry = {
            id: sheet.id,
            name: sheet.name,
            abstract: sheet.abstract || '',
            tags: sheet.tags || [],
            content_excerpt: (sheet.content || '').slice(0, 200),
            last_used_at: sheet.last_used_at || null,
            updated_at: sheet.updated_at || null
        };
        if (idx === -1) state.searchIndex.push(entry);
        else state.searchIndex[idx] = entry;
    }

    function escHtml(s) {
        return String(s)
            .replaceAll('&', '&amp;')
            .replaceAll('<', '&lt;')
            .replaceAll('>', '&gt;')
            .replaceAll('"', '&quot;')
            .replaceAll("'", '&#39;');
    }

    function openCreateModal(windowId, prefillTitle) {
        const state = instances.get(windowId);
        if (!state) return;
        const t = state.t;
        const esc = state.esc;

        const modal = document.createElement('div');
        modal.className = 'cheater-modal';
        modal.setAttribute('role', 'dialog');
        modal.setAttribute('aria-modal', 'true');
        modal.setAttribute('aria-label', t('cheater.create_title'));

        const templates = (window.CheaterTemplates ? window.CheaterTemplates.list(t) : [{ id: 'empty', name: 'Leer', icon: '📄' }]);
        const existingTags = collectTags(state.searchIndex);

        modal.innerHTML = `
            <div class="cheater-modal-backdrop" data-backdrop></div>
            <div class="cheater-modal-panel">
                <h2 class="cheater-modal-title">${esc(t('cheater.create_title'))}</h2>
                <label class="cheater-field">
                    <span>${esc(t('cheater.field_title'))} *</span>
                    <input type="text" data-title required maxlength="120" value="${esc(prefillTitle || '')}">
                    <span class="cheater-field-error" data-title-error aria-live="polite"></span>
                </label>
                <label class="cheater-field">
                    <span>${esc(t('cheater.field_description'))}</span>
                    <input type="text" data-abstract maxlength="200" placeholder="${esc(t('cheater.field_description_placeholder'))}">
                </label>
                <div class="cheater-field">
                    <span>${esc(t('cheater.field_tags'))}</span>
                    <div class="cheater-tag-input">
                        <div class="cheater-tag-chips" data-chips></div>
                        <input type="text" data-tag-input placeholder="${esc(t('cheater.field_tags_placeholder'))}">
                        <datalist data-tag-suggestions>${existingTags.map(tag => `<option value="${esc(tag)}">`).join('')}</datalist>
                    </div>
                </div>
                <div class="cheater-field">
                    <span>${esc(t('cheater.field_template'))}</span>
                    <div class="cheater-template-grid" data-templates>
                        ${templates.map(tpl => `<button type="button" class="cheater-template-card" data-template-id="${esc(tpl.id)}">
                            <span class="cheater-template-icon">${esc(tpl.icon)}</span>
                            <span class="cheater-template-name">${esc(tpl.name)}</span>
                        </button>`).join('')}
                    </div>
                </div>
                <div class="cheater-modal-footer">
                    <button type="button" class="cheater-secondary" data-action="cancel">${esc(t('cheater.cancel'))}</button>
                    <button type="button" class="cheater-primary" data-action="submit" disabled>${esc(t('cheater.create_submit'))}</button>
                </div>
            </div>
        `;
        document.body.appendChild(modal);

        const titleInput = modal.querySelector('[data-title]');
        const titleError = modal.querySelector('[data-title-error]');
        const abstractInput = modal.querySelector('[data-abstract]');
        const tagInput = modal.querySelector('[data-tag-input]');
        const chips = modal.querySelector('[data-chips]');
        const templateGrid = modal.querySelector('[data-templates]');
        const submitBtn = modal.querySelector('[data-action="submit"]');
        const cancelBtn = modal.querySelector('[data-action="cancel"]');
        const backdrop = modal.querySelector('[data-backdrop]');
        const selectedTags = [];
        let selectedTemplate = 'empty';

        function refreshSubmit() {
            const hasTitle = titleInput.value.trim().length > 0;
            submitBtn.disabled = !hasTitle;
            if (titleError) {
                titleError.textContent = hasTitle ? '' : t('cheater.field_title_required');
            }
        }

        function addTag(name) {
            const trimmed = String(name || '').trim();
            if (!trimmed) return;
            if (selectedTags.includes(trimmed)) return;
            selectedTags.push(trimmed);
            renderChips();
        }

        function removeTag(name) {
            const idx = selectedTags.indexOf(name);
            if (idx === -1) return;
            selectedTags.splice(idx, 1);
            renderChips();
        }

        function renderChips() {
            chips.innerHTML = selectedTags.map(tag =>
                `<span class="cheater-pill">${esc(tag)} <button type="button" class="cheater-pill-remove" data-remove="${esc(tag)}" aria-label="${esc(t('cheater.remove_tag'))}">×</button></span>`
            ).join('');
            chips.querySelectorAll('[data-remove]').forEach(btn => {
                btn.addEventListener('click', () => removeTag(btn.dataset.remove));
            });
        }

        titleInput.addEventListener('input', refreshSubmit);
        tagInput.addEventListener('keydown', (e) => {
            if (e.key === 'Enter' || e.key === ',') {
                e.preventDefault();
                addTag(tagInput.value);
                tagInput.value = '';
            }
        });
        templateGrid.querySelectorAll('[data-template-id]').forEach(card => {
            card.addEventListener('click', () => {
                templateGrid.querySelectorAll('.cheater-template-card').forEach(c => c.classList.remove('is-selected'));
                card.classList.add('is-selected');
                selectedTemplate = card.dataset.templateId;
            });
        });
        templateGrid.querySelector('[data-template-id="empty"]').classList.add('is-selected');

        submitBtn.addEventListener('click', submit);
        cancelBtn.addEventListener('click', close);
        backdrop.addEventListener('click', close);
        modal.addEventListener('keydown', (e) => {
            if (e.key === 'Escape') close();
            else if (e.key === 'Enter' && !(e.target instanceof HTMLTextArea)) {
                if (!submitBtn.disabled) submit();
            }
        });

        titleInput.focus();

        async function submit() {
            const title = titleInput.value.trim();
            if (!title) return;
            const tpl = window.CheaterTemplates ? window.CheaterTemplates.byId(selectedTemplate) : { content: '# {{title}}\n\n' };
            const content = (tpl.content || '# {{title}}\n\n').replace(/\{\{title\}\}/g, title);
            submitBtn.disabled = true;
            submitBtn.textContent = t('cheater.creating');
            try {
                const created = await state.api('/api/cheatsheets', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({
                        name: title,
                        content,
                        abstract: abstractInput.value.trim(),
                        tags: selectedTags
                    })
                });
                if (created && created.id) {
                    state.searchIndex.push({
                        id: created.id,
                        name: created.name,
                        abstract: created.abstract || '',
                        tags: created.tags || [],
                        content_excerpt: (created.content || '').slice(0, 200),
                        last_used_at: null,
                        updated_at: created.updated_at || null
                    });
                }
                close();
                if (created) openSheet(state, created);
            } catch (err) {
                state.notify('cheater.error.create_failed', 'error');
                console.error('cheater create failed', err);
                submitBtn.disabled = false;
                submitBtn.textContent = t('cheater.create_submit');
            }
        }

        function close() {
            modal.remove();
        }
    }

    function collectTags(entries) {
        const set = new Set();
        entries.forEach(e => (e.tags || []).forEach(tag => set.add(tag)));
        return Array.from(set).sort();
    }

    window.CheaterApp = window.CheaterApp || {};
    window.CheaterApp.render = render;
    window.CheaterApp.dispose = dispose;
    window.CheaterApp.openSheet = openSheet;
    window.CheaterApp.openCreateModal = openCreateModal;
    window.CheaterApp.formatRelativeShort = formatRelativeShort;
})();
