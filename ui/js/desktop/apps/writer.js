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
        let currentPath = ctx.path || DEFAULT_PATH;
        let officeVersion = null;
        let editor = null;

        host.innerHTML = `<div class="office-app office-writer" data-office-writer="${esc(windowId)}">
            <div class="vd-toolbar office-toolbar">
                <button class="vd-tool-button" type="button" data-action="save">${iconMarkup('save', 'S', 'vd-tool-icon', 15)}<span>${esc(t('desktop.writer_save', 'Save'))}</span></button>
                <a class="vd-tool-button" data-action="download" href="#" download>${iconMarkup('download', 'D', 'vd-tool-icon', 15)}<span>${esc(t('desktop.writer_download_docx', 'DOCX'))}</span></a>
                <a class="vd-tool-button" data-action="export-html" href="#" download>${iconMarkup('html', 'H', 'vd-tool-icon', 15)}<span>${esc(t('desktop.writer_export_html', 'HTML'))}</span></a>
                <a class="vd-tool-button" data-action="export-md" href="#" download>${iconMarkup('markdown', 'M', 'vd-tool-icon', 15)}<span>${esc(t('desktop.writer_export_md', 'Markdown'))}</span></a>
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
        const status = host.querySelector('[data-status]');
        const editorHost = host.querySelector('[data-editor]');
        const fallback = host.querySelector('[data-fallback]');

        function setStatus(message) {
            if (status) status.textContent = message || '';
        }

        function setPath(path) {
            currentPath = path || DEFAULT_PATH;
            if (pathInput) pathInput.value = currentPath;
            updateExportLinks();
            if (typeof ctx.updateWindowContext === 'function') ctx.updateWindowContext(windowId, { path: currentPath });
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
            const path = pathInput.value.trim() || DEFAULT_PATH;
            const base = '/api/desktop/office/export?path=' + encodeURIComponent(path);
            const docx = host.querySelector('[data-action="download"]');
            const html = host.querySelector('[data-action="export-html"]');
            const md = host.querySelector('[data-action="export-md"]');
            if (docx) docx.href = base + '&format=docx';
            if (html) html.href = base + '&format=html';
            if (md) md.href = base + '&format=md';
        }

        async function save() {
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
            updateExportLinks();
            try {
                const body = await api('/api/desktop/office/document?path=' + encodeURIComponent(currentPath));
                const doc = (body && body.document) || {};
                officeVersion = body.office_version || null;
                setDocumentText(doc.text || '');
                if (doc.title && titleInput && !titleInput.value) titleInput.value = doc.title;
                setPath((body.entry && body.entry.path) || doc.path || currentPath);
                setStatus('');
            } catch (err) {
                officeVersion = null;
                setDocumentText(ctx.content || '');
                setStatus('');
                if (ctx.path) notify({ type: 'info', message: err.message || String(err) });
            }
        }

        host.querySelector('[data-action="save"]').addEventListener('click', () => {
            save().catch(err => {
                setStatus(err.message || String(err));
                notify({ type: 'error', message: err.message || String(err) });
            });
        });
        host.querySelectorAll('a[data-action]').forEach(link => {
            link.addEventListener('click', event => {
                if (!pathInput.value.trim()) {
                    event.preventDefault();
                    save().then(() => window.open(link.href, '_blank', 'noopener')).catch(err => setStatus(err.message || String(err)));
                }
            });
        });
        pathInput.addEventListener('change', () => {
            setPath(pathInput.value.trim() || DEFAULT_PATH);
            load();
        });

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
