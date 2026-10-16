(function () {
    'use strict';
    const MAX_BYTES = 256 * 1024 * 1024;
    const asset = path => window.AuraLazyAssets?.versionedURL(path) || path;
    let pdfPromise;

    function contentURL(value) {
        const url = new URL(value, location.href);
        if (url.origin !== location.origin || !url.pathname.startsWith('/api/desktop/bookshelf/')) throw new Error('bookshelf_invalid_resource');
        return url.href;
    }

    async function fetchBook(url, signal) {
        const response = await fetch(contentURL(url), { signal, credentials: 'same-origin', cache: 'no-store' });
        if (!response.ok) throw new Error('bookshelf_content_failed');
        if (Number(response.headers.get('Content-Length')) > MAX_BYTES) throw new Error('bookshelf_file_limit');
        const reader = response.body.getReader();
        let length = 0; const chunks = [];
        try {
            while (true) {
                const { value, done } = await reader.read(); if (done) break;
                length += value.byteLength;
                if (length > MAX_BYTES) throw new Error('bookshelf_file_limit');
                chunks.push(value);
            }
        } catch (error) { await reader.cancel().catch(() => {}); throw error; }
        const expected = Number(response.headers.get('Content-Length'));
        if (expected && expected !== length) throw new Error('bookshelf_content_failed');
        return new Blob(chunks);
    }

    async function createEbook(container, options) {
        const abort = new AbortController(), pending = new Map();
        let disposed = false, nextID = 0, selection = null, connected = false;
        const frame = document.createElement('iframe');
        frame.className = 'bookshelf-reader-frame'; frame.title = options.book.title || 'Bookshelf';
        // This host contains only a trusted port relay; the engine has its own data-origin frame.
        frame.setAttribute('sandbox', 'allow-scripts allow-same-origin');
        frame.setAttribute('allow', "camera 'none'; microphone 'none'; geolocation 'none'; serial 'none'; bluetooth 'none'; midi 'none'; usb 'none'");
        frame.referrerPolicy = 'no-referrer';
        frame.style.cssText = 'border:0;width:100%;height:100%;display:block;';
        frame.src = '/api/desktop/bookshelf/reader';
        const channel = new MessageChannel();
        let readyResolve, readyReject;
        const ready = new Promise((resolve, reject) => { readyResolve = resolve; readyReject = reject; });
        function call(action, data) {
            if (disposed) return Promise.reject(new DOMException('Reader closed', 'AbortError'));
            const id = ++nextID;
            return new Promise((resolve, reject) => {
                const timer = setTimeout(() => { pending.delete(id); reject(new Error('bookshelf_reader_timeout')); }, action === 'search' ? 60000 : 30000);
                pending.set(id, { resolve, reject, timer }); channel.port1.postMessage({ id, action, data });
            });
        }
        channel.port1.onmessage = event => {
            if (disposed) return;
            const { type, data } = event.data || {};
            if (type === 'connected') { connected = true; readyResolve(); }
            else if (type === 'ready') options.onReady?.(data);
            else if (type === 'progress') options.onProgress?.(data);
            else if (type === 'metadata') options.onMetadata?.(data);
            else if (type === 'selection') selection = data;
            else if (type === 'external-link') options.onExternalLink?.(data?.href);
            else if (type === 'error' || type === 'result') {
                const request = pending.get(data?.id);
                if (request) {
                    clearTimeout(request.timer); pending.delete(data.id);
                    if (type === 'error') request.reject(new Error(data.code)); else request.resolve(data.result);
                }
                if (type === 'error') options.onError?.(new Error(data.code));
            }
        };
        channel.port1.start();
        const timer = setTimeout(() => readyReject(new Error('bookshelf_reader_timeout')), 15000);
        frame.addEventListener('load', () => {
            if (!disposed && !connected) frame.contentWindow.postMessage({ type: 'bookshelf-connect' }, '*', [channel.port2]);
        }, { once: true });
        container.replaceChildren(frame);
        function dispose() {
            if (disposed) return;
            disposed = true; abort.abort(); clearTimeout(timer);
            if (!connected) readyReject(new DOMException('Reader closed', 'AbortError'));
            options.signal?.removeEventListener('abort', dispose); channel.port1.close(); channel.port2.close();
            for (const request of pending.values()) { clearTimeout(request.timer); request.reject(new DOMException('Reader closed', 'AbortError')); }
            pending.clear(); frame.remove();
        }
        options.signal?.addEventListener('abort', dispose, { once: true });
        if (options.signal?.aborted) dispose();
        try {
            const [blob] = await Promise.all([fetchBook(options.contentURL, abort.signal), ready]);
            clearTimeout(timer);
            await call('open', { blob, name: options.book.path?.split('/').pop() || `${options.book.title}.${options.book.format}`,
                format: options.book.format, progress: options.progress, settings: options.settings });
        } catch (error) { dispose(); throw error; }
        return { dispose, next: () => call('next'), prev: () => call('prev'), goTo: locator => call('goTo', locator),
            setPreferences: settings => call('preferences', settings), search: query => call('search', query),
            getSelection: () => selection, addAnnotation: annotation => call('annotation', annotation),
            removeAnnotation: annotation => call('remove-annotation', annotation), getThumbnail: index => call('thumbnail', index) };
    }

    async function createPDF(container, options) {
        pdfPromise ||= import(asset('/js/desktop/apps/bookshelf/reader-pdf.js')).catch(error => { pdfPromise = null; throw error; });
        await pdfPromise;
        return window.BookshelfPDFReader.create(container, options);
    }

    window.BookshelfReader = Object.freeze({ create: (container, options) =>
        String(options.book.format).toLowerCase() === 'pdf' ? createPDF(container, options) : createEbook(container, options) });
}());
