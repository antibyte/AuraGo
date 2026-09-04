(function () {
    'use strict';

    const PREVIEW_DEBOUNCE_MS = 150;
    const VIEW_MODE_KEY = 'notes.viewMode';
    const DEFAULT_VIEW_MODE = 'split';
    const PREVIEW_MAX_BYTES = 256 * 1024;

    let mermaidLoading = null;
    let mermaidSeq = 0;

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

    function countWords(text) {
        const m = String(text || '').trim().match(/\S+/g);
        return m ? m.length : 0;
    }

    function formatClock(date) {
        try {
            return new Date(date || Date.now()).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
        } catch (_) {
            return '';
        }
    }

    function create(state, slot) {
        if (!slot) return null;
        const t = state.t;
        const esc = state.esc;
        const viewMode = loadViewMode();
        state.viewMode = viewMode;

        slot.innerHTML = `<div class="vd-notes-main-inner" data-notes-pane data-view="${esc(viewMode)}">
            <div class="vd-notes-head">
                <input class="vd-notes-title-input" data-notes-title type="text" spellcheck="false" autocomplete="off" placeholder="${esc(t('desktop.notes_new_first_title'))}" maxlength="160">
                <div class="vd-notes-head-meta">
                    <span class="vd-notes-path" data-notes-path></span>
                    <span class="vd-notes-dirty" data-notes-dirty hidden title="${esc(t('desktop.notes_unsaved'))}">●</span>
                </div>
                <div class="vd-notes-actions">
                    <button type="button" class="vd-notes-btn vd-notes-btn-primary" data-action="save" title="Ctrl+S">${esc(t('desktop.save'))}</button>
                    <button type="button" class="vd-notes-btn" data-action="rename">${esc(t('desktop.notes_rename'))}</button>
                    <button type="button" class="vd-notes-btn" data-action="duplicate">${esc(t('desktop.notes_duplicate'))}</button>
                    <button type="button" class="vd-notes-btn vd-notes-btn-danger" data-action="delete">${esc(t('desktop.notes_delete'))}</button>
                    <button type="button" class="vd-notes-btn" data-action="export">${esc(t('desktop.notes_export'))}</button>
                </div>
            </div>
            <div class="vd-notes-toolbar">
                <div class="vd-notes-toolbar-slot" data-notes-toolbar-slot></div>
                <div class="vd-notes-view-toggle" role="group" aria-label="${esc(t('desktop.notes_view'))}">
                    <button type="button" class="vd-notes-view-btn${viewMode === 'edit' ? ' is-active' : ''}" data-view-mode="edit">${esc(t('desktop.notes_view_edit'))}</button>
                    <button type="button" class="vd-notes-view-btn${viewMode === 'split' ? ' is-active' : ''}" data-view-mode="split">${esc(t('desktop.notes_view_split'))}</button>
                    <button type="button" class="vd-notes-view-btn${viewMode === 'preview' ? ' is-active' : ''}" data-view-mode="preview">${esc(t('desktop.notes_view_preview'))}</button>
                </div>
            </div>
            <div class="vd-notes-banner" data-notes-banner hidden>
                <span data-banner-text></span>
                <button type="button" class="vd-notes-btn" data-action="banner-reload">${esc(t('desktop.notes_reload'))}</button>
                <button type="button" class="vd-notes-banner-close" data-action="banner-dismiss" aria-label="${esc(t('desktop.close'))}">×</button>
            </div>
            <div class="vd-notes-content">
                <textarea class="vd-notes-source" data-notes-source spellcheck="true" placeholder="${esc(t('desktop.notes_placeholder'))}"></textarea>
                <div class="vd-notes-preview" data-notes-preview aria-live="polite"></div>
                <div class="vd-notes-empty-hint" data-notes-empty hidden>${esc(t('desktop.notes_empty_hint'))}</div>
            </div>
            <footer class="vd-notes-statusbar">
                <span class="vd-notes-status-count"><span data-notes-words>0</span> ${esc(t('desktop.notes_words'))}</span>
                <span class="vd-notes-status-count"><span data-notes-chars>0</span> ${esc(t('desktop.notes_chars'))}</span>
                <span class="vd-notes-status-spacer"></span>
                <span class="vd-notes-status-save" data-notes-save></span>
            </footer>
        </div>`;

        const pane = slot.querySelector('[data-notes-pane]');
        const titleInput = slot.querySelector('[data-notes-title]');
        const pathLabel = slot.querySelector('[data-notes-path]');
        const dirtyDot = slot.querySelector('[data-notes-dirty]');
        const source = slot.querySelector('[data-notes-source]');
        const preview = slot.querySelector('[data-notes-preview]');
        const emptyHint = slot.querySelector('[data-notes-empty]');
        const banner = slot.querySelector('[data-notes-banner]');
        const bannerText = slot.querySelector('[data-banner-text]');
        const wordsNode = slot.querySelector('[data-notes-words]');
        const charsNode = slot.querySelector('[data-notes-chars]');
        const saveNode = slot.querySelector('[data-notes-save]');
        const toolbarSlot = slot.querySelector('[data-notes-toolbar-slot]');

        let previewTimer = null;

        function currentContent() { return source.value; }

        function updateCounts() {
            const text = source.value;
            if (wordsNode) wordsNode.textContent = String(countWords(text));
            if (charsNode) charsNode.textContent = String(text.length);
        }

        function schedulePreview() {
            if (previewTimer) clearTimeout(previewTimer);
            previewTimer = setTimeout(renderPreview, PREVIEW_DEBOUNCE_MS);
        }

        function sanitizeHTML(html) {
            if (window.DOMPurify && typeof window.DOMPurify.sanitize === 'function') {
                return window.DOMPurify.sanitize(html, { USE_PROFILES: { html: true } });
            }
            return String(html)
                .replaceAll('&', '&amp;')
                .replaceAll('<', '&lt;')
                .replaceAll('>', '&gt;');
        }

        function renderPreview() {
            if (!preview) return;
            if (state.viewMode === 'edit') return;
            const content = currentContent();
            if (content.length > PREVIEW_MAX_BYTES) {
                preview.textContent = '';
                const hint = document.createElement('p');
                hint.className = 'vd-notes-preview-cap';
                hint.textContent = t('desktop.notes_preview_skipped');
                preview.appendChild(hint);
                return;
            }
            try {
                const rendered = window.marked ? window.marked.parse(content, { gfm: true, breaks: false }) : escapeFallback(content);
                preview.innerHTML = sanitizeHTML(rendered);
                if (window.hljs && window.hljs.highlightElement) {
                    preview.querySelectorAll('pre code').forEach(c => window.hljs.highlightElement(c));
                }
                renderMermaidBlocks(preview);
            } catch (err) {
                console.error('notes preview render failed', err);
                preview.textContent = content;
            }
        }

        function escapeFallback(text) {
            return '<pre>' + String(text)
                .replaceAll('&', '&amp;')
                .replaceAll('<', '&lt;')
                .replaceAll('>', '&gt;') + '</pre>';
        }

        function renderMermaidBlocks(container) {
            const blocks = container.querySelectorAll('pre code.language-mermaid');
            if (!blocks.length) return;
            ensureMermaid().then(mermaid => {
                blocks.forEach(block => {
                    const code = block.textContent.trim();
                    const id = 'vd-notes-mermaid-' + Date.now() + '-' + (++mermaidSeq);
                    mermaid.render(id, code).then(result => {
                        const wrap = document.createElement('div');
                        wrap.className = 'vd-notes-mermaid';
                        wrap.innerHTML = result.svg;
                        const pre = block.closest('pre');
                        if (pre && pre.parentNode) pre.parentNode.replaceChild(wrap, pre);
                    }).catch(() => {
                        const note = document.createElement('div');
                        note.className = 'vd-notes-mermaid-failed';
                        note.textContent = t('desktop.notes_mermaid_failed');
                        const pre = block.closest('pre');
                        if (pre) pre.classList.add('vd-notes-mermaid-error');
                        if (pre && pre.parentNode) pre.parentNode.insertBefore(note, pre.nextSibling);
                    });
                });
            }).catch(err => {
                console.warn('notes mermaid load failed', err);
            });
        }

        function ensureMermaid() {
            if (window.mermaid && window.mermaid.render) {
                return Promise.resolve(window.mermaid);
            }
            if (mermaidLoading) return mermaidLoading;
            mermaidLoading = new Promise((resolve, reject) => {
                const loader = window.AuraLazyAssets && typeof window.AuraLazyAssets.loadScript === 'function'
                    ? window.AuraLazyAssets.loadScript('/js/vendor/mermaid.min.js')
                    : injectMermaidScript();
                loader.then(() => {
                    if (!window.mermaid) {
                        reject(new Error('mermaid missing'));
                        return;
                    }
                    window.mermaid.initialize({ startOnLoad: false, securityLevel: 'strict', theme: 'dark' });
                    resolve(window.mermaid);
                }).catch(reject);
            });
            return mermaidLoading;
        }

        function injectMermaidScript() {
            return new Promise((resolve, reject) => {
                const script = document.createElement('script');
                script.src = '/js/vendor/mermaid.min.js';
                script.async = true;
                script.onload = () => resolve();
                script.onerror = () => reject(new Error('mermaid script failed'));
                document.head.appendChild(script);
            });
        }

        function setViewMode(mode) {
            if (mode !== 'edit' && mode !== 'split' && mode !== 'preview') return;
            state.viewMode = mode;
            saveViewMode(mode);
            if (pane) pane.dataset.view = mode;
            slot.querySelectorAll('[data-view-mode]').forEach(btn => {
                btn.classList.toggle('is-active', btn.dataset.viewMode === mode);
            });
            renderPreview();
        }

        function cycleViewMode() {
            const order = ['edit', 'split', 'preview'];
            const idx = order.indexOf(state.viewMode);
            setViewMode(order[(idx + 1) % order.length]);
        }

        function updateSaveStatus(kind, at) {
            if (!saveNode) return;
            delete saveNode.dataset.state;
            if (kind === 'saving') {
                saveNode.dataset.state = 'saving';
                saveNode.textContent = t('desktop.notes_saving');
            } else if (kind === 'error') {
                saveNode.dataset.state = 'error';
                saveNode.textContent = t('desktop.notes_save_failed');
            } else if (kind === 'saved') {
                saveNode.dataset.state = 'saved';
                saveNode.textContent = t('desktop.notes_saved_at', { time: formatClock(at || Date.now()) });
            } else {
                saveNode.textContent = '';
            }
        }

        function setDirty(dirty) {
            if (dirtyDot) dirtyDot.hidden = !dirty;
        }

        function load(note) {
            if (!note) {
                if (titleInput) titleInput.value = '';
                source.value = '';
                if (pathLabel) pathLabel.textContent = '';
                updateCounts();
                updateSaveStatus('none');
                setDirty(false);
                renderPreview();
                setEmptyHint(true);
                return;
            }
            setEmptyHint(false);
            if (titleInput) titleInput.value = note.title || '';
            source.value = note.content || '';
            if (pathLabel) pathLabel.textContent = note.path || t('desktop.notes_unsaved');
            updateCounts();
            setDirty(false);
            updateSaveStatus(note.lastSavedAt ? 'saved' : 'none', note.lastSavedAt);
            renderPreview();
        }

        function setEmptyHint(visible) {
            if (emptyHint) emptyHint.hidden = !visible;
        }

        function setReadonly(readonly) {
            if (titleInput) titleInput.readOnly = !!readonly;
            source.readOnly = !!readonly;
            slot.querySelectorAll('[data-action]').forEach(btn => {
                if (btn.dataset.action === 'export' || btn.dataset.action === 'banner-reload' || btn.dataset.action === 'banner-dismiss') return;
                btn.disabled = !!readonly;
            });
        }

        function showBanner() {
            if (!banner) return;
            if (bannerText) bannerText.textContent = t('desktop.notes_external_change');
            banner.hidden = false;
        }

        function hideBanner() {
            if (banner) banner.hidden = true;
        }

        function setTitle(title) {
            if (titleInput && document.activeElement !== titleInput) titleInput.value = title || '';
        }

        function setPath(path) {
            if (pathLabel) pathLabel.textContent = path || t('desktop.notes_unsaved');
        }

        titleInput.addEventListener('input', () => {
            if (typeof state.onTitleInput === 'function') state.onTitleInput(titleInput.value);
        });
        source.addEventListener('input', () => {
            updateCounts();
            if (typeof state.onContentInput === 'function') state.onContentInput(source.value);
            schedulePreview();
        });
        source.addEventListener('keydown', (e) => {
            if (e.key === 'Tab') {
                e.preventDefault();
                handleTabIndent(source, e.shiftKey);
            }
        });
        slot.querySelectorAll('[data-view-mode]').forEach(btn => {
            btn.addEventListener('click', () => {
                const mode = btn.dataset.viewMode;
                if (typeof state.onViewModeChange === 'function') state.onViewModeChange(mode);
                setViewMode(mode);
            });
        });
        const bannerReload = slot.querySelector('[data-action="banner-reload"]');
        if (bannerReload) bannerReload.addEventListener('click', () => {
            hideBanner();
            if (typeof state.onReloadFromDisk === 'function') state.onReloadFromDisk();
        });
        const bannerDismiss = slot.querySelector('[data-action="banner-dismiss"]');
        if (bannerDismiss) bannerDismiss.addEventListener('click', hideBanner);

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
            const transformed = reverse
                ? block.split('\n').map(line => line.startsWith('    ') ? line.slice(4) : (line.startsWith('\t') ? line.slice(1) : line)).join('\n')
                : block.split('\n').map(line => '    ' + line).join('\n');
            textarea.value = before + transformed + after;
            textarea.selectionStart = lineStart;
            textarea.selectionEnd = lineStart + transformed.length;
            textarea.dispatchEvent(new Event('input'));
        }

        return {
            toolbarSlot,
            pane,
            load,
            setDirty,
            updateCounts,
            updateSaveStatus,
            setViewMode,
            cycleViewMode,
            renderPreview,
            schedulePreview,
            setReadonly,
            showBanner,
            hideBanner,
            setTitle,
            setPath,
            setEmptyHint,
            getTitle: () => titleInput.value,
            getContent: currentContent,
            focusEditor: () => source.focus(),
            focusTitle: () => { if (titleInput) titleInput.focus(); },
            dispose() {
                if (previewTimer) { clearTimeout(previewTimer); previewTimer = null; }
            }
        };
    }

    window.NotesEditor = { create };
})();
