(function () {
    'use strict';
    if (window.AuraDesktopPrint) return;

    // Parent-owned printing: the document can load images/fonts but never run
    // scripts, submit forms or navigate its parent. Keep sanitization at callers.
    async function create({ html, title = '', className = 'vd-print-frame', failureMessage = 'desktop.print_failed', signal } = {}) {
        const frame = document.createElement('iframe');
        frame.className = className;
        frame.title = title;
        frame.setAttribute('sandbox', 'allow-same-origin allow-modals');
        let timer = 0, disposed = false;
        let rejectLoad;
        const dispose = () => {
            if (disposed) return;
            disposed = true;
            window.clearTimeout(timer);
            signal?.removeEventListener('abort', dispose);
            frame.remove();
            rejectLoad?.(new DOMException('Print cancelled', 'AbortError'));
        };
        if (signal?.aborted) throw new DOMException('Print cancelled', 'AbortError');
        signal?.addEventListener('abort', dispose, { once: true });
        const loaded = new Promise((resolve, reject) => {
            rejectLoad = reject;
            frame.addEventListener('load', resolve, { once: true });
            timer = window.setTimeout(() => { reject(new Error(failureMessage)); dispose(); }, 15000);
        });
        frame.srcdoc = html || '<!doctype html><html><head></head><body></body></html>';
        document.body.appendChild(frame);
        try { await loaded; } catch (error) { dispose(); throw error; }
        rejectLoad = null;
        window.clearTimeout(timer);
        const doc = frame.contentDocument;
        const print = async () => {
            if (disposed || signal?.aborted) throw new DOMException('Print cancelled', 'AbortError');
            let readyTimer;
            try {
                await Promise.race([
                    Promise.all([doc.fonts?.ready, ...Array.from(doc.images, image => image.decode().catch(() => {}))]),
                    new Promise((_, reject) => { rejectLoad = reject; readyTimer = window.setTimeout(() => reject(new Error(failureMessage)), 15000); })
                ]);
                if (disposed || signal?.aborted) throw new DOMException('Print cancelled', 'AbortError');
                frame.contentWindow.addEventListener('afterprint', dispose, { once: true });
                timer = window.setTimeout(dispose, 60000);
                frame.contentWindow.focus();
                frame.contentWindow.print();
            } catch (error) { dispose(); throw error; }
            finally { rejectLoad = null; window.clearTimeout(readyTimer); }
        };
        return { frame, document: doc, print, dispose };
    }

    async function printHTML(options) { const job = await create(options); await job.print(); }
    window.AuraDesktopPrint = { create, printHTML };
})();
