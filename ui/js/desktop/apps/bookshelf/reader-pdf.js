(function () {
    'use strict';
    const asset = path => window.AuraLazyAssets?.versionedURL(path) || path;
    let library;
    async function create(container, options) {
        const source = new URL(options.contentURL, location.href);
        if (source.origin !== location.origin || !source.pathname.startsWith('/api/desktop/bookshelf/')) throw new Error('bookshelf_invalid_resource');
        if (!document.querySelector('link[data-bookshelf-reader-style]')) {
            const css = document.createElement('link'); css.rel = 'stylesheet'; css.dataset.bookshelfReaderStyle = '';
            css.href = asset('/css/apps/bookshelf-reader.css'); document.head.append(css);
        }
        library ||= import(asset('/js/vendor/pdf.min.js')).then(() => window.pdfjsLib).catch(error => { library = null; throw error; });
        const lib = await library;
        let disposed = false, generation = 0, searchGeneration = 0, pdf, selection = null;
        let pageNumber = Math.max(1, Number(options.progress?.locator?.page) || 1), settings = options.settings || {};
        const tasks = new Set(), layers = new Set(), annotations = new Map();
        const loading = lib.getDocument({ url: source.href, isEvalSupported: false, enableXfa: false,
            cMapUrl: '/js/vendor/pdf/cmaps/', cMapPacked: true, standardFontDataUrl: '/js/vendor/pdf/standard_fonts/',
            wasmUrl: '/js/vendor/pdf/wasm/', iccUrl: '/js/vendor/pdf/iccs/' });
        const viewport = document.createElement('div'); viewport.className = 'bookshelf-pdf-viewport';
        viewport.tabIndex = 0;
        viewport.style.cssText = 'height:100%;width:100%;min-width:0;overflow:auto;position:relative;';
        const spread = document.createElement('div'); spread.className = 'bookshelf-pdf-spread';
        spread.style.cssText = 'display:flex;gap:12px;align-items:flex-start;width:max-content;margin:auto;';
        viewport.append(spread); container.replaceChildren(viewport);
        function progress() {
            if (!pdf || disposed) return;
            const y = Math.max(0, Math.min(1, viewport.scrollTop / Math.max(1, spread.clientHeight)));
            const visibleEnd = Math.max(0, Math.min(1, (viewport.scrollTop + viewport.clientHeight) / Math.max(1, spread.clientHeight)));
            const pages = Math.min(settings.spread ? 2 : 1, pdf.numPages - pageNumber + 1);
            options.onProgress?.({ locator: { page: pageNumber, y }, fraction: Math.min(1, (pageNumber - 1 + visibleEnd * pages) / pdf.numPages) });
        }
        function drawAnnotations() {
            spread.querySelectorAll('[data-bookshelf-highlight]').forEach(el => el.remove());
            for (const annotation of annotations.values()) {
                const page = spread.querySelector(`[data-page="${Number(annotation.locator?.page)}"]`); if (!page) continue;
                for (const rect of (annotation.locator?.rects || []).slice(0, 200)) {
                    if (!['x', 'y', 'width', 'height'].every(key => Number.isFinite(rect[key]) && rect[key] >= 0 && rect[key] <= 1)) continue;
                    const el = document.createElement('span'); el.dataset.bookshelfHighlight = '';
                    el.style.cssText = `position:absolute;pointer-events:none;background:#f0c34b66;left:${rect.x * 100}%;top:${rect.y * 100}%;width:${Math.min(rect.width, 1 - rect.x) * 100}%;height:${Math.min(rect.height, 1 - rect.y) * 100}%;`;
                    page.append(el);
                }
            }
        }
        async function paint(page, scale, host, textEnabled, own) {
            const view = page.getViewport({ scale }), ratio = textEnabled ? Math.min(2, devicePixelRatio || 1) : 1;
            const canvas = document.createElement('canvas');
            const width = Math.ceil(view.width * ratio), height = Math.ceil(view.height * ratio);
            if (!Number.isFinite(width * height) || width * height > 32 * 1024 * 1024) throw new Error('bookshelf_image_limit');
            canvas.width = width; canvas.height = height;
            canvas.style.cssText = `display:block;width:${view.width}px;height:${view.height}px;`;
            if (host) { host.style.width = `${view.width}px`; host.style.height = `${view.height}px`; host.append(canvas); }
            const task = page.render({ canvasContext: canvas.getContext('2d'), viewport: view, transform: [ratio, 0, 0, ratio, 0, 0] });
            tasks.add(task);
            try { await task.promise; } finally { tasks.delete(task); }
            if (disposed || textEnabled && own !== generation) return null;
            if (textEnabled) {
                const content = await page.getTextContent(); if (disposed || own !== generation) return null;
                const text = document.createElement('div'); text.className = 'textLayer bookshelf-pdf-text';
                text.style.cssText = 'position:absolute;inset:0;overflow:clip;line-height:1;text-align:initial;';
                text.style.setProperty('--total-scale-factor', scale); host.append(text);
                const layer = new lib.TextLayer({ textContentSource: content, container: text, viewport: view });
                layers.add(layer); try { await layer.render(); } finally { layers.delete(layer); }
            }
            return canvas;
        }
        async function render(y = 0) {
            if (disposed || !pdf) return;
            const own = ++generation;
            for (const task of tasks) task.cancel(); for (const layer of layers) layer.cancel();
            const numbers = [pageNumber]; if (settings.spread && pageNumber < pdf.numPages) numbers.push(pageNumber + 1);
            const pages = await Promise.all(numbers.map(number => pdf.getPage(number)));
            if (disposed || own !== generation) return;
            spread.replaceChildren(); spread.style.flexDirection = settings.direction === 'rtl' ? 'row-reverse' : 'row';
            const base = pages.map(page => page.getViewport({ scale: 1 }));
            const width = base.reduce((sum, view) => sum + view.width, 0), height = Math.max(...base.map(view => view.height));
            const fitWidth = (viewport.clientWidth - 24 - (pages.length - 1) * 12) / width;
            const fit = settings.fit === 'page' ? Math.min(fitWidth, (viewport.clientHeight - 24) / height) : fitWidth;
            const scale = Math.max(0.1, Math.min(4, Number(settings.zoom) || fit));
            try {
                for (let i = 0; i < pages.length; i++) {
                    const host = document.createElement('div'); host.className = 'bookshelf-pdf-page'; host.dataset.page = numbers[i];
                    host.style.cssText = 'position:relative;background:#fff;flex:none;'; spread.append(host);
                    await paint(pages[i], scale, host, true, own);
                    if (disposed || own !== generation) return;
                }
            } catch (error) { if (disposed || own !== generation || error.name === 'RenderingCancelledException') return; throw error; }
            drawAnnotations(); viewport.scrollTop = Number(y) * spread.clientHeight || 0; progress();
        }
        function onSelection() {
            const selected = document.getSelection();
            if (!selected?.rangeCount || selected.isCollapsed || !spread.contains(selected.anchorNode)) return;
            const node = selected.anchorNode.nodeType === 1 ? selected.anchorNode : selected.anchorNode.parentElement;
            const host = node.closest('[data-page]'); if (!host) return;
            const bounds = host.getBoundingClientRect();
            selection = { text: selected.toString().slice(0, 4000), locator: { page: Number(host.dataset.page),
                rects: Array.from(selected.getRangeAt(0).getClientRects()).filter(rect => rect.left >= bounds.left - 1 && rect.right <= bounds.right + 1).slice(0, 200).map(rect => ({
                    x: Math.max(0, (rect.left - bounds.left) / bounds.width), y: Math.max(0, (rect.top - bounds.top) / bounds.height),
                    width: Math.min(1, rect.width / bounds.width), height: Math.min(1, rect.height / bounds.height) })) } };
        }
        async function goTo(locator) { pageNumber = Math.max(1, Math.min(pdf.numPages, Math.floor(Number(locator?.page) || 1))); await render(locator?.y); }
        const next = () => goTo({ page: pageNumber + (settings.spread ? 2 : 1) });
        const prev = () => goTo({ page: pageNumber - (settings.spread ? 2 : 1) });
        function keyboard(event) {
            if (event.altKey || event.ctrlKey || event.metaKey) return;
            if (['ArrowRight', 'PageDown'].includes(event.key)) { event.preventDefault(); (settings.direction === 'rtl' && event.key === 'ArrowRight' ? prev() : next()).catch(error => options.onError?.(error)); }
            if (['ArrowLeft', 'PageUp'].includes(event.key)) { event.preventDefault(); (settings.direction === 'rtl' && event.key === 'ArrowLeft' ? next() : prev()).catch(error => options.onError?.(error)); }
        }
        function dispose() {
            if (disposed) return; disposed = true; generation++; searchGeneration++;
            for (const task of tasks) task.cancel(); for (const layer of layers) layer.cancel();
            loading.destroy().catch(() => {}); observer.disconnect();
            document.removeEventListener('selectionchange', onSelection); options.signal?.removeEventListener('abort', dispose); viewport.remove();
        }
        const observer = new ResizeObserver(() => { if (pdf && !disposed) render(viewport.scrollTop / Math.max(1, spread.clientHeight)).catch(error => options.onError?.(error)); });
        document.addEventListener('selectionchange', onSelection); viewport.addEventListener('keydown', keyboard);
        viewport.addEventListener('scroll', progress, { passive: true });
        options.signal?.addEventListener('abort', dispose, { once: true }); if (options.signal?.aborted) dispose();
        async function thumbnail(index) {
            if (!pdf || disposed || !Number.isInteger(index) || index < 0 || index >= pdf.numPages) return null;
            const page = await pdf.getPage(index + 1), base = page.getViewport({ scale: 1 });
            const canvas = await paint(page, Math.min(1, 180 / base.width), null, false);
            return canvas ? new Promise(resolve => canvas.toBlob(resolve, 'image/png')) : null;
        }
        try {
            loading.onPassword = (update, reason) => {
                if (!options.onPassword) { options.onError?.(new Error('bookshelf_pdf_password_required')); dispose(); return; }
                Promise.resolve(options.onPassword(reason)).then(password => password ? update(password) : dispose()).catch(error => { options.onError?.(error); dispose(); });
            };
            pdf = await loading.promise; if (disposed) throw new DOMException('Reader closed', 'AbortError');
            pageNumber = Math.min(pageNumber, pdf.numPages); await render(options.progress?.locator?.y);
            async function outlineItem(entry, depth = 0) {
                if (depth > 16) return null;
                let dest = entry.dest; if (typeof dest === 'string') dest = await pdf.getDestination(dest);
                const number = Array.isArray(dest) ? (typeof dest[0] === 'number' ? dest[0] : await pdf.getPageIndex(dest[0])) + 1 : 1;
                return { label: String(entry.title).slice(0, 500), href: { page: number }, subitems: (await Promise.all((entry.items || []).slice(0, 500).map(item => outlineItem(item, depth + 1)))).filter(Boolean) };
            }
            const toc = (await Promise.all((await pdf.getOutline() || []).slice(0, 500).map(item => outlineItem(item)))).filter(Boolean);
            options.onReady?.({ toc, isFixedLayout: true, pageCount: pdf.numPages }); observer.observe(viewport);
            pdf.getMetadata().then(({ info }) => { if (!disposed) options.onMetadata?.({ title: info.Title || '', author: info.Author || '', description: info.Subject || '' }); }).catch(() => {});
        } catch (error) { dispose(); throw error; }
        return { dispose, next, prev, goTo, getThumbnail: thumbnail,
            setPreferences: async value => { settings = { ...settings, ...value }; await render(); }, getSelection: () => selection,
            addAnnotation: annotation => { annotations.set(annotation.id || JSON.stringify(annotation.locator), annotation); drawAnnotations(); },
            removeAnnotation: annotation => { annotations.delete(annotation.id || JSON.stringify(annotation.locator)); drawAnnotations(); },
            search: async query => {
                const own = ++searchGeneration, needle = String(query).trim().slice(0, 200).toLocaleLowerCase(), results = [];
                if (!needle) return results;
                for (let number = 1; number <= pdf.numPages && results.length < 100 && !disposed && own === searchGeneration; number++) {
                    const page = await pdf.getPage(number), text = (await page.getTextContent()).items.map(item => item.str || '').join(' ');
                    const at = text.toLocaleLowerCase().indexOf(needle);
                    if (at >= 0) results.push({ text: text.slice(Math.max(0, at - 45), at + needle.length + 70), locator: { page: number } });
                }
                return results;
            } };
    }
    window.BookshelfPDFReader = Object.freeze({ create });
}());
