(function () {
    'use strict';

    if (typeof pdfjsLib !== 'undefined') {
        pdfjsLib.GlobalWorkerOptions.workerSrc = '/js/vendor/pdf.worker.min.js';
    }

    const instances = new Map();

    function render(host, windowId, context) {
        if (!host) return;
        dispose(windowId);
        instances.set(windowId, { container: host });
        const ctx = context || {};
        const esc = ctx.esc || (value => String(value == null ? '' : value));
        const rawT = ctx.t || ((key, vars) => interpolate(key, vars));
        const t = (key, fallback, vars) => {
            if (fallback && typeof fallback === 'object' && !Array.isArray(fallback)) {
                vars = fallback;
                fallback = '';
            }
            const translated = rawT(key, vars || {});
            return translated && translated !== key ? translated : (fallback || key);
        };
        const api = ctx.api || fetchJSON;
        const iconMarkup = ctx.iconMarkup || ((key, fallback) => `<span>${esc(fallback || key || '')}</span>`);
        const notify = ctx.notify || (() => {});
        let currentPath = ctx.path || '';
        const fileName = currentPath.split('/').pop() || currentPath;
        const ext = fileName.split('.').pop().toLowerCase();
        const viewerType = viewerTypeForExt(ext);
        const canEdit = ['docx', 'xlsx', 'xlsm', 'csv'].includes(ext);
        const editApp = ['docx', 'html', 'htm'].includes(ext) ? 'writer' : 'sheets';
        let activeSheet = 0;
        let workbook = null;
        let pdfDoc = null;
        let pdfPage = 1;
        let pdfScale = 1.0;
        const fileIconKey = viewerIconForExt(ext, viewerType);

        host.innerHTML = `<div class="vd-viewer" data-viewer="${esc(windowId)}">
            <div class="vd-viewer-toolbar">
                <div class="vd-viewer-toolbar-left">
                    ${viewerIcon(fileIconKey, 'D', 'vd-viewer-file-icon', 20)}
                    <span class="vd-viewer-filename">${esc(fileName)}</span>
                </div>
                <div class="vd-viewer-toolbar-right">
                    ${canEdit ? `<button class="vd-tool-button" type="button" data-action="edit">${viewerIcon('edit', 'E')}<span>${esc(t('viewer.edit', 'Edit'))}</span></button>` : ''}
                    <button class="vd-tool-button" type="button" data-action="download">${viewerIcon('download', 'D')}<span>${esc(t('viewer.download', 'Download'))}</span></button>
                    <button class="vd-tool-button" type="button" data-action="print">${viewerIcon('printer', 'P')}<span>${esc(t('viewer.print', 'Print'))}</span></button>
                    <div class="vd-viewer-pdf-controls" data-pdf-controls style="display:none">
                        <button class="vd-tool-button vd-viewer-icon-button" type="button" data-action="zoom-out">${viewerGlyph('-')}</button>
                        <span class="vd-viewer-zoom-label" data-zoom-label>100%</span>
                        <button class="vd-tool-button vd-viewer-icon-button" type="button" data-action="zoom-in">${viewerGlyph('+')}</button>
                        <button class="vd-tool-button vd-viewer-icon-button" type="button" data-action="zoom-reset">${viewerGlyph('1:1')}</button>
                        <span class="vd-viewer-sep"></span>
                        <button class="vd-tool-button vd-viewer-icon-button" type="button" data-action="page-prev">${viewerGlyph('<')}</button>
                        <span class="vd-viewer-page-label" data-page-label>1 / 1</span>
                        <button class="vd-tool-button vd-viewer-icon-button" type="button" data-action="page-next">${viewerGlyph('>')}</button>
                    </div>
                </div>
            </div>
            <div class="vd-viewer-sheets-bar" data-sheets-bar style="display:none"></div>
            <div class="vd-viewer-content" data-viewer-content>
                <div class="vd-viewer-loading">${esc(t('viewer.loading', 'Loading...'))}</div>
            </div>
        </div>`;

        const contentEl = host.querySelector('[data-viewer-content]');
        const pdfControls = host.querySelector('[data-pdf-controls]');
        const sheetsBar = host.querySelector('[data-sheets-bar]');
        const zoomLabel = host.querySelector('[data-zoom-label]');
        const pageLabel = host.querySelector('[data-page-label]');

        host.querySelectorAll('[data-action]').forEach(btn => {
            btn.addEventListener('click', () => handleAction(btn.dataset.action));
        });

        if (typeof ctx.wireContextMenuBoundary === 'function') ctx.wireContextMenuBoundary(host);

        function viewerIcon(key, fallback, className, size) {
            return iconMarkup(key, fallback, className || 'vd-viewer-action-icon', size || 17);
        }

        function viewerGlyph(glyph) {
            return `<span class="vd-viewer-glyph-icon" aria-hidden="true">${esc(glyph)}</span>`;
        }

        function handleAction(action) {
            switch (action) {
                case 'edit':
                    if (ctx.openApp) ctx.openApp(editApp, { path: currentPath });
                    break;
                case 'download':
                    downloadFile();
                    break;
                case 'print':
                    printFile();
                    break;
                case 'zoom-in':
                    pdfScale = Math.min(3.0, pdfScale + 0.25);
                    renderPdfPage();
                    break;
                case 'zoom-out':
                    pdfScale = Math.max(0.5, pdfScale - 0.25);
                    renderPdfPage();
                    break;
                case 'zoom-reset':
                    pdfScale = 1.0;
                    renderPdfPage();
                    break;
                case 'page-prev':
                    if (pdfPage > 1) { pdfPage--; renderPdfPage(); }
                    break;
                case 'page-next':
                    if (pdfDoc && pdfPage < pdfDoc.numPages) { pdfPage++; renderPdfPage(); }
                    break;
            }
        }

        function downloadFile() {
            if (typeof ctx.exportDesktopFile === 'function') {
                ctx.exportDesktopFile({ path: currentPath, name: fileName }).catch(err => notify(err.message || String(err)));
                return;
            }
            const link = document.createElement('a');
            link.href = '/api/desktop/download?path=' + encodeURIComponent(currentPath);
            link.download = fileName;
            document.body.appendChild(link);
            link.click();
            document.body.removeChild(link);
        }

        async function printFile() {
            try {
                if (viewerType === 'pdf') {
                    if (!pdfDoc) {
                        notify(t('viewer.loading', 'Loading...'));
                        return;
                    }
                    await printPdfDocument();
                } else {
                    await printRenderedContent();
                }
            } catch (err) {
                notify(t('viewer.error', 'Failed to load file') + ': ' + err.message);
            }
        }

        async function printPdfDocument() {
            const frame = document.createElement('iframe');
            frame.className = 'vd-print-frame';
            frame.title = 'Print';
            let cleaned = false;
            const cleanup = () => {
                if (cleaned) return;
                cleaned = true;
                frame.remove();
            };
            document.body.appendChild(frame);
            const printDoc = frame.contentDocument;
            const printWindow = frame.contentWindow;
            if (!printDoc || !printWindow) {
                cleanup();
                throw new Error('print frame unavailable');
            }
            const pages = [];
            for (let pageNumber = 1; pageNumber <= pdfDoc.numPages; pageNumber++) {
                const page = await pdfDoc.getPage(pageNumber);
                const viewport = page.getViewport({ scale: 2 });
                const canvas = document.createElement('canvas');
                canvas.width = Math.ceil(viewport.width);
                canvas.height = Math.ceil(viewport.height);
                const canvasContext = canvas.getContext('2d');
                await page.render({ canvasContext, viewport }).promise;
                pages.push(canvas.toDataURL('image/png'));
            }
            printDoc.open();
            printDoc.write(`<!doctype html><html><head><title>${esc(fileName)}</title><style>
                @page { margin: 0; }
                html, body { margin: 0; padding: 0; background: #fff; }
                img { display: block; width: 100%; height: auto; page-break-after: always; break-after: page; }
                img:last-child { page-break-after: auto; break-after: auto; }
            </style></head><body>${pages.map(src => `<img alt="" src="${src}">`).join('')}</body></html>`);
            printDoc.close();
            await Promise.all(Array.from(printDoc.images).map(img => img.complete ? Promise.resolve() : new Promise((resolve, reject) => {
                img.onload = resolve;
                img.onerror = reject;
            })));
            printWindow.addEventListener('afterprint', cleanup, { once: true });
            window.setTimeout(cleanup, 60000);
            printWindow.focus();
            printWindow.print();
        }

        async function printRenderedContent() {
            const source = contentEl.querySelector('.vd-viewer-rendered, .vd-viewer-xlsx') || contentEl.firstElementChild || contentEl;
            if (!source || source.classList.contains('vd-viewer-loading') || source.classList.contains('vd-viewer-error')) {
                notify(t('viewer.loading', 'Loading...'));
                return;
            }
            const frame = document.createElement('iframe');
            frame.className = 'vd-print-frame';
            frame.title = 'Print';
            let cleaned = false;
            const cleanup = () => {
                if (cleaned) return;
                cleaned = true;
                frame.remove();
            };
            document.body.appendChild(frame);
            const printDoc = frame.contentDocument;
            const printWindow = frame.contentWindow;
            if (!printDoc || !printWindow) {
                cleanup();
                throw new Error('print frame unavailable');
            }
            printDoc.open();
            printDoc.write(`<!doctype html><html><head><title>${esc(fileName)}</title><style>
                @page { margin: 14mm; }
                html, body { margin: 0; padding: 0; background: #fff; color: #111; font: 14px/1.55 system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; }
                body { padding: 0; }
                h1, h2, h3, h4, h5, h6 { color: #111; page-break-after: avoid; }
                pre { white-space: pre-wrap; border: 1px solid #ddd; border-radius: 4px; padding: 10px; background: #f7f7f7; }
                code { font-family: "Cascadia Code", "JetBrains Mono", monospace; font-size: 12px; }
                blockquote { border-left: 3px solid #ccc; margin-left: 0; padding-left: 12px; color: #444; }
                img { max-width: 100%; height: auto; }
                table { border-collapse: collapse; width: 100%; font-size: 12px; }
                th, td { border: 1px solid #ccc; padding: 5px 7px; text-align: left; }
                th { background: #f2f2f2; }
            </style></head><body></body></html>`);
            printDoc.close();
            printDoc.body.appendChild(source.cloneNode(true));
            await Promise.all(Array.from(printDoc.images).map(img => img.complete ? Promise.resolve() : new Promise(resolve => {
                img.onload = resolve;
                img.onerror = resolve;
            })));
            printWindow.addEventListener('afterprint', cleanup, { once: true });
            window.setTimeout(cleanup, 60000);
            printWindow.focus();
            printWindow.print();
        }

        async function loadContent() {
            try {
                if (viewerType === 'pdf') {
                    pdfControls.style.display = 'flex';
                    await loadPdf();
                } else {
                    const resp = await api('/api/desktop/viewer/content?path=' + encodeURIComponent(currentPath));
                    if (viewerType === 'markdown') renderMarkdown(resp.content);
                    else if (viewerType === 'document') renderDocument(resp.content);
                    else if (viewerType === 'spreadsheet') renderSpreadsheet(resp.workbook);
                }
            } catch (err) {
                contentEl.innerHTML = `<div class="vd-viewer-error">${esc(t('viewer.error', 'Failed to load file'))}: ${esc(err.message)}</div>`;
            }
        }

        function renderMarkdown(raw) {
            if (!raw) {
                contentEl.innerHTML = `<div class="vd-viewer-empty">${esc(t('viewer.no_content', 'No content to display'))}</div>`;
                return;
            }
            const md = window.markdownit({ html: false, linkify: true, typographer: true });
            const rendered = md.render(raw);
            contentEl.innerHTML = `<div class="vd-viewer-md vd-viewer-rendered">${rendered}</div>`;
        }

        function sanitizeViewerHTML(html) {
            var div = document.createElement('div');
            div.innerHTML = html;
            div.querySelectorAll('script, style, link, meta, base, iframe, object, embed, form, input, textarea, button, select').forEach(function(el) { el.remove(); });
            div.querySelectorAll('*').forEach(function(el) {
                Array.from(el.attributes).forEach(function(attr) {
                    if (/^on/i.test(attr.name) || (/^href|^src|^action|^formaction|^data/i.test(attr.name) && /^javascript:/i.test(attr.value))) {
                        el.removeAttribute(attr.name);
                    }
                });
            });
            return div.innerHTML;
        }

        function renderDocument(html) {
            if (!html) {
                contentEl.innerHTML = `<div class="vd-viewer-empty">${esc(t('viewer.no_content', 'No content to display'))}</div>`;
                return;
            }
            contentEl.innerHTML = `<div class="vd-viewer-docx vd-viewer-rendered">${sanitizeViewerHTML(html)}</div>`;
        }

        function renderSpreadsheet(wb) {
            if (!wb || !wb.sheets || !wb.sheets.length) {
                contentEl.innerHTML = `<div class="vd-viewer-empty">${esc(t('viewer.no_content', 'No content to display'))}</div>`;
                return;
            }
            workbook = wb;
            activeSheet = Math.min(activeSheet, workbook.sheets.length - 1);
            if (workbook.sheets.length > 1) {
                sheetsBar.style.display = 'flex';
                renderSheetTabs();
            }
            renderSheetGrid();
        }

        function renderSheetTabs() {
            sheetsBar.innerHTML = workbook.sheets.map((s, i) =>
                `<button type="button" class="vd-viewer-sheet-tab${i === activeSheet ? ' active' : ''}" data-sheet-idx="${i}">${esc(s.name || ('Sheet ' + (i + 1)))}</button>`
            ).join('');
            sheetsBar.querySelectorAll('[data-sheet-idx]').forEach(btn => {
                btn.addEventListener('click', () => {
                    activeSheet = Number(btn.dataset.sheetIdx) || 0;
                    renderSheetTabs();
                    renderSheetGrid();
                });
            });
        }

        function renderSheetGrid() {
            const sheet = workbook.sheets[activeSheet];
            const rows = sheet.rows || [];
            if (!rows.length) {
                contentEl.innerHTML = `<div class="vd-viewer-empty">${esc(t('viewer.no_content', 'No content to display'))}</div>`;
                return;
            }
            const maxCols = Math.max(1, ...rows.map(r => (Array.isArray(r) ? r.length : 0)));
            const colHeaders = Array.from({ length: maxCols }, (_, i) => columnName(i + 1));
            let html = `<div class="vd-viewer-xlsx"><table class="vd-viewer-grid"><thead><tr><th></th>${colHeaders.map(c => `<th>${esc(c)}</th>`).join('')}</tr></thead><tbody>`;
            for (let r = 0; r < rows.length; r++) {
                html += `<tr><th>${r + 1}</th>`;
                for (let c = 0; c < maxCols; c++) {
                    const cell = rows[r] && rows[r][c];
                    const val = cell ? (cell.value || '') : '';
                    html += `<td>${esc(val)}</td>`;
                }
                html += '</tr>';
            }
            html += '</tbody></table></div>';
            contentEl.innerHTML = html;
        }

        async function loadPdf() {
            if (typeof pdfjsLib === 'undefined') {
                contentEl.innerHTML = `<div class="vd-viewer-error">pdf.js not loaded</div>`;
                return;
            }
            try {
                const loadingTask = pdfjsLib.getDocument('/api/desktop/viewer/content?path=' + encodeURIComponent(currentPath));
                pdfDoc = await loadingTask.promise;
                pdfPage = 1;
                renderPdfPage();
            } catch (err) {
                contentEl.innerHTML = `<div class="vd-viewer-error">${esc(t('viewer.error', 'Failed to load file'))}: ${esc(err.message)}</div>`;
            }
        }

        async function renderPdfPage() {
            if (!pdfDoc) return;
            const page = await pdfDoc.getPage(pdfPage);
            const viewport = page.getViewport({ scale: pdfScale });
            const canvas = document.createElement('canvas');
            canvas.width = viewport.width;
            canvas.height = viewport.height;
            canvas.className = 'vd-viewer-pdf-canvas';
            const ctxCanvas = canvas.getContext('2d');
            await page.render({ canvasContext: ctxCanvas, viewport: viewport }).promise;
            contentEl.innerHTML = '';
            const wrapper = document.createElement('div');
            wrapper.className = 'vd-viewer-pdf';
            wrapper.appendChild(canvas);
            contentEl.appendChild(wrapper);
            if (pageLabel) pageLabel.textContent = pdfPage + ' / ' + pdfDoc.numPages;
            if (zoomLabel) zoomLabel.textContent = Math.round(pdfScale * 100) + '%';
        }

        loadContent();
    }

    function dispose(windowId) {
        instances.delete(windowId);
    }

    function viewerTypeForExt(ext) {
        switch (ext) {
            case 'md': return 'markdown';
            case 'pdf': return 'pdf';
            case 'docx': return 'document';
            case 'xlsx': case 'xlsm': case 'csv': return 'spreadsheet';
            default: return 'unknown';
        }
    }

    function viewerIconForExt(ext, viewerType) {
        if (viewerType === 'pdf') return 'pdf';
        if (viewerType === 'markdown') return 'markdown';
        if (viewerType === 'document') return 'documents';
        if (viewerType === 'spreadsheet') return 'spreadsheet';
        return ext ? 'file-' + ext : 'text';
    }

    function columnName(index) {
        let name = '';
        let n = index;
        while (n > 0) {
            const mod = (n - 1) % 26;
            name = String.fromCharCode(65 + mod) + name;
            n = Math.floor((n - mod) / 26);
        }
        return name;
    }

    async function fetchJSON(url, options) {
        const resp = await fetch(url, options);
        const body = await resp.json().catch(() => ({}));
        if (!resp.ok) throw new Error(body.error || body.message || ('HTTP ' + resp.status));
        return body;
    }

    function interpolate(text, vars) {
        let result = String(text || '');
        Object.entries(vars || {}).forEach(([key, value]) => {
            result = result.replaceAll('{{' + key + '}}', String(value));
        });
        return result;
    }

    window.ViewerApp = window.ViewerApp || {};
    window.ViewerApp.render = render;
    window.ViewerApp.dispose = dispose;
})();
