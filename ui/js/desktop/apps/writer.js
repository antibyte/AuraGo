(function () {
    'use strict';

    const DEFAULT_PATH = 'Documents/untitled.docx';
    const instances = new Map();

    function render(host, windowId, context) {
        if (!host) return;
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

        host.innerHTML = `<div class="office-app office-writer" data-office-writer="${esc(windowId)}">
            <div class="vd-toolbar office-toolbar">
                <input class="office-path-input" data-path value="${esc(currentPath)}" spellcheck="false" autocomplete="off">
                <input class="office-title-input" data-title value="${esc(ctx.title || '')}" spellcheck="false" autocomplete="off" placeholder="${esc(t('desktop.writer_title_placeholder', 'Title'))}">
                <span class="vd-chat-meta" data-status>${esc(t('desktop.writer_loading', 'Loading...'))}</span>
            </div>
            <div class="office-writer-tools" data-quill-toolbar>
                <select class="ql-header">
                    <option selected></option>
                    <option value="1"></option>
                    <option value="2"></option>
                    <option value="3"></option>
                </select>
                <button class="ql-bold" type="button"></button>
                <button class="ql-italic" type="button"></button>
                <button class="ql-underline" type="button"></button>
                <button class="ql-list" type="button" value="ordered"></button>
                <button class="ql-list" type="button" value="bullet"></button>
                <button class="ql-link" type="button"></button>
                <button class="ql-clean" type="button"></button>
            </div>
            <div class="office-writer-editor" data-editor></div>
            <textarea class="office-writer-fallback" data-fallback spellcheck="true" hidden></textarea>
        </div>`;

        const pathInput = host.querySelector('[data-path]');
        const titleInput = host.querySelector('[data-title]');
        const statusNode = host.querySelector('[data-status]');
        const editorHost = host.querySelector('[data-editor]');
        const fallback = host.querySelector('[data-fallback]');

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
            window.open(exportURL(format), '_blank', 'noopener');
        }

        async function save() {
            if (readonly) return;
            setStatus(t('desktop.saving', 'Saving...'));
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
            setStatus(t('desktop.writer_saved', 'Saved'));
            notify({ type: 'success', message: t('desktop.writer_saved', 'Saved') });
            await refreshDesktop();
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

        function setWindowMenus() {
            if (typeof ctx.setWindowMenus !== 'function') return;
            ctx.setWindowMenus(windowId, [
                {
                    id: 'file',
                    labelKey: 'desktop.menu_file',
                    items: [
                        { id: 'save', labelKey: 'desktop.writer_save', icon: 'save', shortcut: 'Ctrl+S', disabled: readonly, action: () => save().catch(err => {
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
                        { id: 'select-all', labelKey: 'desktop.fm.select_all', icon: 'check-square', shortcut: 'Ctrl+A', action: selectAllText }
                    ]
                }
            ]);
        }

        async function load() {
            if (!editor && window.Quill && editorHost) {
                try {
                    editor = new window.Quill(editorHost, {
                        modules: { toolbar: host.querySelector('[data-quill-toolbar]') },
                        placeholder: t('desktop.writer_placeholder', 'Start writing...'),
                        theme: 'snow'
                    });
                } catch (_) {
                    editor = null;
                }
            }
            if (!editor) {
                editorHost.hidden = true;
                host.querySelector('[data-quill-toolbar]').hidden = true;
                fallback.hidden = false;
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
                setStatus('');
            } catch (err) {
                officeVersion = null;
                setDocumentText(ctx.content || '');
                setStatus('');
                if (ctx.path) notify({ type: 'info', message: err.message || String(err) });
            }
        }

        pathInput.addEventListener('change', () => {
            setPath(pathInput.value.trim() || DEFAULT_PATH);
            load();
        });

        setWindowMenus();
        load();
    }

    function dispose(windowId) {
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

    function escapeHTML(value) {
        return String(value == null ? '' : value)
            .replaceAll('&', '&amp;')
            .replaceAll('<', '&lt;')
            .replaceAll('>', '&gt;')
            .replaceAll('"', '&quot;')
            .replaceAll("'", '&#39;');
    }

    window.WriterApp = window.WriterApp || {};
    window.WriterApp.render = render;
    window.WriterApp.dispose = dispose;
})();
