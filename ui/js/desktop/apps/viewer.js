(function () {
    'use strict';

    const instances = new Map();

    function render(host, windowId, context) {
        if (!host) return;
        dispose(windowId);
        const life = new AbortController();
        instances.set(windowId, { container: host, life });
        const ctx = context || {};
        const esc = ctx.esc || (value => String(value ?? '').replace(/[&<>"']/g, char => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[char])));
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
        const archiveEntry = String(ctx.archiveEntry || ctx.entry || '').trim();
        const archivePath = archiveEntry ? String(ctx.archive || ctx.path || '').trim() : '';
        let currentPath = archiveEntry ? archivePath : (ctx.path || '');
        const fileName = (archiveEntry ? archiveEntry.split('/').pop() : currentPath.split('/').pop()) || currentPath;
        const ext = fileName.split('.').pop().toLowerCase();
        window.AuraDesktopEmail?.attach(windowId, '.vd-viewer-toolbar-right', () => ({
            key:currentPath,signal:life.signal,disabled:!currentPath,subject:fileName,formats:[{id:'original',label:fileName}],
            createFiles:(_format,signal)=>window.AuraDesktopEmail.files([{url:archiveEntry?archiveEntryURL():'/api/desktop/download?path='+encodeURIComponent(currentPath),name:fileName}],signal)
        }));
        const rawViewerType = viewerTypeForExt(ext);
        // Archive members keep the zipper overlay; the window viewer only shows plain-file images.
        const viewerType = (rawViewerType === 'image' && archiveEntry) ? 'unknown' : rawViewerType;
        const isImage = viewerType === 'image';
        const canEdit = !archiveEntry && (['docx', 'xlsx', 'xlsm', 'csv'].includes(ext) || isImage);
        const editApp = isImage ? 'pixel' : (['docx', 'html', 'htm'].includes(ext) ? 'writer' : 'sheets');
        let activeSheet = 0;
        let workbook = null;
        let pdfDoc = null;
        let pdfPage = 1;
        let pdfScale = 1.0;
        let imgEl = null;
        let imgWrap = null;
        let imgZoomLabel = null;
        let imgNatW = 0;
        let imgNatH = 0;
        const imgView = { scale: 1, x: 0, y: 0, rotation: 0, flip: false };
        const fileIconKey = viewerIconForExt(ext, viewerType);

        host.innerHTML = `<div class="vd-viewer" data-viewer="${esc(windowId)}">
            <div class="vd-viewer-toolbar">
                <div class="vd-viewer-toolbar-left">
                    ${viewerIcon(fileIconKey, 'D', 'vd-viewer-file-icon', 20)}
                    <span class="vd-viewer-filename" data-viewer-title>${esc(fileName)}</span>
                </div>
                <div class="vd-viewer-toolbar-right">
                    ${canEdit ? `<button class="vd-tool-button" type="button" data-action="edit" title="${esc(t(isImage ? 'viewer.edit_image' : 'viewer.edit'))}">${viewerIcon('edit', 'E')}<span>${esc(t(isImage ? 'viewer.edit_image' : 'viewer.edit'))}</span></button>` : ''}
                    <button class="vd-tool-button" type="button" data-action="download">${viewerIcon('download', 'D')}<span>${esc(t('viewer.download'))}</span></button>
                    <button class="vd-tool-button" type="button" data-action="print">${viewerIcon('printer', 'P')}<span>${esc(t('viewer.print'))}</span></button>
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
                    <div class="vd-viewer-image-controls" data-image-controls style="display:none">
                        <button class="vd-tool-button vd-viewer-icon-button" type="button" data-action="img-zoom-out" title="${esc(t('viewer.zoom_out'))}" aria-label="${esc(t('viewer.zoom_out'))}">${viewerGlyph('−')}</button>
                        <span class="vd-viewer-zoom-label" data-image-zoom-label>100%</span>
                        <button class="vd-tool-button vd-viewer-icon-button" type="button" data-action="img-zoom-in" title="${esc(t('viewer.zoom_in'))}" aria-label="${esc(t('viewer.zoom_in'))}">${viewerGlyph('+')}</button>
                        <button class="vd-tool-button vd-viewer-icon-button" type="button" data-action="img-zoom-fit" title="${esc(t('viewer.zoom_fit'))}" aria-label="${esc(t('viewer.zoom_fit'))}">${viewerGlyph('⤢')}</button>
                        <button class="vd-tool-button vd-viewer-icon-button" type="button" data-action="img-zoom-actual" title="${esc(t('viewer.zoom_actual'))}" aria-label="${esc(t('viewer.zoom_actual'))}">${viewerGlyph('1:1')}</button>
                        <span class="vd-viewer-sep"></span>
                        <button class="vd-tool-button vd-viewer-icon-button" type="button" data-action="img-rotate-left" title="${esc(t('viewer.rotate_left'))}" aria-label="${esc(t('viewer.rotate_left'))}">${viewerGlyph('⟲')}</button>
                        <button class="vd-tool-button vd-viewer-icon-button" type="button" data-action="img-rotate-right" title="${esc(t('viewer.rotate_right'))}" aria-label="${esc(t('viewer.rotate_right'))}">${viewerGlyph('⟳')}</button>
                        <button class="vd-tool-button vd-viewer-icon-button" type="button" data-action="img-flip" title="${esc(t('viewer.flip'))}" aria-label="${esc(t('viewer.flip'))}">${viewerGlyph('⇋')}</button>
                        <span class="vd-viewer-sep"></span>
                        <button class="vd-tool-button vd-viewer-icon-button" type="button" data-action="img-copy" title="${esc(t('viewer.copy_image'))}" aria-label="${esc(t('viewer.copy_image'))}">${viewerGlyph('⧉')}</button>
                    </div>
                </div>
            </div>
            <div class="vd-viewer-sheets-bar" data-sheets-bar style="display:none"></div>
            <div class="vd-viewer-content" data-viewer-content>
                <div class="vd-viewer-loading">${esc(t('viewer.loading'))}</div>
            </div>
        </div>`;

        const contentEl = host.querySelector('[data-viewer-content]');
        const pdfControls = host.querySelector('[data-pdf-controls]');
        const imageControls = host.querySelector('[data-image-controls]');
        const sheetsBar = host.querySelector('[data-sheets-bar]');
        const zoomLabel = host.querySelector('[data-zoom-label]');
        const pageLabel = host.querySelector('[data-page-label]');
        const viewerTitle = host.querySelector('[data-viewer-title]');

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
                    if (viewerType === 'image' && ctx.openApp) ctx.openApp(editApp, { path: currentPath, forceNew: true });
                    else if (ctx.openApp) ctx.openApp(editApp, { path: currentPath });
                    break;
                case 'img-zoom-out': imageZoomBy(-1); break;
                case 'img-zoom-in': imageZoomBy(1); break;
                case 'img-zoom-fit': imageFit(); break;
                case 'img-zoom-actual': imageActual(); break;
                case 'img-rotate-left': imageRotate(-90); break;
                case 'img-rotate-right': imageRotate(90); break;
                case 'img-flip': imageFlip(); break;
                case 'img-copy': imageCopy(); break;
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

        function viewerContentURL() {
            let url = '/api/desktop/viewer/content?path=' + encodeURIComponent(currentPath);
            if (archiveEntry) url += '&entry=' + encodeURIComponent(archiveEntry);
            return url;
        }

        function archiveEntryURL() {
            return '/api/desktop/archive/entry?path=' + encodeURIComponent(currentPath) + '&entry=' + encodeURIComponent(archiveEntry);
        }

        function downloadFile() {
            if (archiveEntry) {
                const link = document.createElement('a');
                link.href = archiveEntryURL();
                link.download = fileName;
                document.body.appendChild(link);
                link.click();
                document.body.removeChild(link);
                return;
            }
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
                        notify(t('viewer.loading'));
                        return;
                    }
                    await printPdfDocument();
                } else {
                    await printRenderedContent();
                }
            } catch (err) {
                notify(t('viewer.error') + ': ' + err.message);
            }
        }

        async function printPdfDocument() {
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
            const job = await window.AuraDesktopPrint.create({ title: t('viewer.print'), failureMessage: t('desktop.print_failed'), signal: life.signal, html: `<!doctype html><html><head><title>${esc(fileName)}</title><style>
                @page { margin: 0; }
                html, body { margin: 0; padding: 0; background: #fff; }
                img { display: block; width: 100%; height: auto; page-break-after: always; break-after: page; }
                img:last-child { page-break-after: auto; break-after: auto; }
            </style></head><body>${pages.map(src => `<img alt="" src="${src}">`).join('')}</body></html>` });

            await job.print();
        }

        async function printRenderedContent() {
            const source = contentEl.querySelector('.vd-viewer-rendered, .vd-viewer-xlsx') || contentEl.firstElementChild || contentEl;
            if (!source || source.classList.contains('vd-viewer-loading') || source.classList.contains('vd-viewer-error')) {
                notify(t('viewer.loading'));
                return;
            }
            const job = await window.AuraDesktopPrint.create({ title: t('viewer.print'), failureMessage: t('desktop.print_failed'), signal: life.signal, html: `<!doctype html><html><head><title>${esc(fileName)}</title><style>
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
            </style></head><body></body></html>` });

            job.document.body.appendChild(source.cloneNode(true));
            await job.print();
        }

        function imageFileURL() {
            return '/api/desktop/download?path=' + encodeURIComponent(currentPath) + '&inline=1';
        }

        function applyImageView() {
            if (!imgEl || !imgWrap) return;
            const rotation = ((imgView.rotation % 360) + 360) % 360;
            const sideways = rotation === 90 || rotation === 270;
            const wrapRect = imgWrap.getBoundingClientRect();
            const baseW = (sideways ? imgNatH : imgNatW) || wrapRect.width || 1;
            const baseH = (sideways ? imgNatW : imgNatH) || wrapRect.height || 1;
            const maxX = Math.max(0, (baseW * imgView.scale - wrapRect.width) / 2);
            const maxY = Math.max(0, (baseH * imgView.scale - wrapRect.height) / 2);
            imgView.x = Math.min(Math.max(imgView.x, -maxX), maxX);
            imgView.y = Math.min(Math.max(imgView.y, -maxY), maxY);
            const flipX = imgView.flip ? -1 : 1;
            imgEl.style.transform = `translate(${imgView.x}px, ${imgView.y}px) rotate(${rotation}deg) scale(${flipX * imgView.scale}, ${imgView.scale})`;
            if (imgZoomLabel) imgZoomLabel.textContent = Math.round(imgView.scale * 100) + '%';
        }

        function imageResetView() {
            imgView.scale = 1;
            imgView.x = 0;
            imgView.y = 0;
            applyImageView();
        }

        function imageZoomBy(direction) {
            if (!imgEl) return;
            imageSetScale(imgView.scale * (direction > 0 ? 1.25 : 0.8));
        }

        function imageSetScale(scale) {
            imgView.scale = Math.min(8, Math.max(0.1, scale));
            imgView.x = 0;
            imgView.y = 0;
            applyImageView();
        }

        function imageFit() {
            imageResetView();
        }

        function imageActual() {
            imageSetScale(1);
        }

        function imageRotate(degrees) {
            if (!imgEl) return;
            imgView.rotation = ((imgView.rotation + degrees) % 360 + 360) % 360;
            imgView.x = 0;
            imgView.y = 0;
            applyImageView();
        }

        function imageFlip() {
            if (!imgEl) return;
            imgView.flip = !imgView.flip;
            applyImageView();
        }

        async function imageCopy() {
            if (!imgEl || !currentPath) return;
            try {
                const response = await fetch(imageFileURL(), { credentials: 'same-origin' });
                if (!response.ok) throw new Error('HTTP ' + response.status);
                const blob = await response.blob();
                if (!('ClipboardItem' in window) || !navigator.clipboard || typeof navigator.clipboard.write !== 'function') {
                    throw new Error('clipboard unsupported');
                }
                await navigator.clipboard.write([new ClipboardItem({ [blob.type || 'image/png']: blob })]);
                notify({ title: fileName, message: t('viewer.copied') });
            } catch (err) {
                notify({ title: fileName, message: t('viewer.copy_failed') });
            }
        }

        function renderImage() {
            imgView.scale = 1;
            imgView.x = 0;
            imgView.y = 0;
            imgView.rotation = 0;
            imgView.flip = false;
            imgNatW = 0;
            imgNatH = 0;
            contentEl.innerHTML = `<div class="vd-viewer-image-wrap vd-viewer-rendered" data-image-wrap><img class="vd-viewer-image" data-image alt="" draggable="false"></div>`;
            imgWrap = contentEl.querySelector('[data-image-wrap]');
            imgEl = contentEl.querySelector('[data-image]');
            imgZoomLabel = host.querySelector('[data-image-zoom-label]');
            if (imgZoomLabel) imgZoomLabel.textContent = '100%';
            if (!imgWrap || !imgEl) {
                contentEl.innerHTML = `<div class="vd-viewer-error">${esc(t('viewer.error'))}</div>`;
                return;
            }
            imgEl.addEventListener('load', () => {
                imgNatW = imgEl.naturalWidth || 0;
                imgNatH = imgEl.naturalHeight || 0;
                viewerTitle.textContent = imgNatW && imgNatH ? `${fileName} — ${imgNatW}×${imgNatH}` : fileName;
                applyImageView();
            });
            imgEl.addEventListener('error', () => {
                contentEl.innerHTML = `<div class="vd-viewer-error">${esc(t('viewer.error'))}: ${esc(fileName)}</div>`;
            });
            let panning = null;
            imgWrap.addEventListener('pointerdown', (event) => {
                if (event.button !== 0 && event.pointerType === 'mouse') return;
                panning = { x: event.clientX - imgView.x, y: event.clientY - imgView.y, id: event.pointerId };
                try { imgWrap.setPointerCapture(event.pointerId); } catch (err) { /* ignore */ }
            });
            imgWrap.addEventListener('pointermove', (event) => {
                if (!panning || event.pointerId !== panning.id) return;
                imgView.x = event.clientX - panning.x;
                imgView.y = event.clientY - panning.y;
                applyImageView();
            });
            const endPan = (event) => {
                if (panning && event.pointerId === panning.id) panning = null;
            };
            imgWrap.addEventListener('pointerup', endPan);
            imgWrap.addEventListener('pointercancel', endPan);
            imgWrap.addEventListener('wheel', (event) => {
                event.preventDefault();
                imageZoomBy(event.deltaY > 0 ? -1 : 1);
            }, { passive: false });
            imgWrap.addEventListener('dblclick', () => {
                if (imgView.scale > 1.001) imageFit();
                else imageActual();
            });
            imgEl.setAttribute('src', imageFileURL());
        }

        async function loadContent() {
            try {
                if (viewerType === 'pdf') {
                    pdfControls.style.display = 'flex';
                    await loadPdf();
                } else if (viewerType === 'image') {
                    imageControls.style.display = 'flex';
                    renderImage();
                } else {
                    const resp = await api(viewerContentURL());
                    if (viewerType === 'markdown') renderMarkdown(resp.content);
                    else if (viewerType === 'document') renderDocument(resp.content);
                    else if (viewerType === 'spreadsheet') renderSpreadsheet(resp.workbook);
                }
            } catch (err) {
                contentEl.innerHTML = `<div class="vd-viewer-error">${esc(t('viewer.error'))}: ${esc(err.message)}</div>`;
            }
        }

        function renderMarkdown(raw) {
            if (!raw) {
                contentEl.innerHTML = `<div class="vd-viewer-empty">${esc(t('viewer.no_content'))}</div>`;
                return;
            }
            if (typeof window.markdownit !== 'function') {
                contentEl.innerHTML = `<div class="vd-viewer-error">${esc(t('viewer.error'))}</div>`;
                return;
            }
            const md = window.markdownit({ html: false, linkify: true, typographer: true });
            const rendered = sanitizeMarkdownHTML(md.render(raw));
            contentEl.innerHTML = `<div class="vd-viewer-md vd-viewer-rendered">${rendered}</div>`;
        }

        function sanitizeMarkdownHTML(html) {
            if (window.DOMPurify && typeof window.DOMPurify.sanitize === 'function') {
                return window.DOMPurify.sanitize(html, { USE_PROFILES: { html: true } });
            }
            return esc(html);
        }

        function sanitizeViewerHTML(html) {
            if (!window.DOMPurify || typeof window.DOMPurify.sanitize !== 'function') {
                return esc(html);
            }
            return window.DOMPurify.sanitize(html, {
                USE_PROFILES: { html: true },
                FORBID_TAGS: ['script', 'style', 'link', 'meta', 'base', 'iframe', 'object', 'embed', 'form', 'input', 'textarea', 'button', 'select'],
                FORBID_ATTR: ['srcdoc']
            });
        }

        function renderDocument(html) {
            if (!html) {
                contentEl.innerHTML = `<div class="vd-viewer-empty">${esc(t('viewer.no_content'))}</div>`;
                return;
            }
            contentEl.innerHTML = `<div class="vd-viewer-docx vd-viewer-rendered">${sanitizeViewerHTML(html)}</div>`;
        }

        function renderSpreadsheet(wb) {
            if (!wb || !wb.sheets || !wb.sheets.length) {
                contentEl.innerHTML = `<div class="vd-viewer-empty">${esc(t('viewer.no_content'))}</div>`;
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
                contentEl.innerHTML = `<div class="vd-viewer-empty">${esc(t('viewer.no_content'))}</div>`;
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
            try {
                const pdf = window.pdfjsLib;
                if (!pdf) {
                    contentEl.innerHTML = `<div class="vd-viewer-error">${esc(t('desktop.viewer_pdfjs_unavailable'))}</div>`;
                    return;
                }
                const loadingTask = pdf.getDocument({ url: viewerContentURL(),
                    cMapUrl: '/js/vendor/pdf/cmaps/', cMapPacked: true,
                    standardFontDataUrl: '/js/vendor/pdf/standard_fonts/',
                    wasmUrl: '/js/vendor/pdf/wasm/', iccUrl: '/js/vendor/pdf/iccs/' });
                const instance = instances.get(windowId);
                if (instance) instance.pdfLoadingTask = loadingTask;
                pdfDoc = await loadingTask.promise;
                pdfPage = 1;
                renderPdfPage();
            } catch (err) {
                contentEl.innerHTML = `<div class="vd-viewer-error">${esc(t('viewer.error'))}: ${esc(err.message)}</div>`;
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
        const instance = instances.get(windowId);
        if (instance) {
            instance.life?.abort();
            if (instance.pdfLoadingTask && typeof instance.pdfLoadingTask.destroy === 'function') {
                try { instance.pdfLoadingTask.destroy(); } catch (_) {}
            }
            instance.pdfDoc = null;
        }
        instances.delete(windowId);
    }

    const imageExts = ['png', 'jpg', 'jpeg', 'gif', 'webp', 'bmp', 'svg', 'ico', 'avif'];

    function viewerTypeForExt(ext) {
        switch (ext) {
            case 'md': return 'markdown';
            case 'pdf': return 'pdf';
            case 'docx': return 'document';
            case 'xlsx': case 'xlsm': case 'csv': return 'spreadsheet';
            default: return imageExts.includes(ext) ? 'image' : 'unknown';
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
    window.ViewerApp.open = open;

    function open(windowId, context) {
        const instance = instances.get(windowId);
        if (!instance || !instance.container || !context) return;
        render(instance.container, windowId, context);
    }
})();
