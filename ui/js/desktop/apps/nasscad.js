(function () {
    'use strict';

    const NASSCAD_ENTRY = 'Apps/nasscad/index.html';
    const instances = new Map();

    function render(host, windowId, context) {
        if (!host) return;
        const ctx = context || {};
        const esc = ctx.esc || (value => String(value == null ? '' : value));
        const t = ctx.t || ((key, fallback) => fallback || key);
        const desktopEmbedURL = ctx.desktopEmbedURL;
        const ensureDesktopEmbedHasContent = ctx.ensureDesktopEmbedHasContent;
        const makeSandboxedFrame = ctx.makeSandboxedFrame;
        const state = { disposed: false };
        instances.set(windowId, state);

        if (typeof desktopEmbedURL !== 'function' || typeof makeSandboxedFrame !== 'function') {
            host.innerHTML = `<div class="vd-nasscad-error">${esc(t('desktop.nasscad_load_failed'))}</div>`;
            return;
        }

        host.innerHTML = `<div class="vd-nasscad">
            <div class="vd-nasscad-loading">${esc(t('desktop.loading'))}</div>
        </div>`;

        const wrap = host.querySelector('.vd-nasscad');
        desktopEmbedURL(NASSCAD_ENTRY)
            .then(async src => {
                if (state.disposed || !wrap) return;
                if (typeof ensureDesktopEmbedHasContent === 'function') {
                    await ensureDesktopEmbedHasContent(src);
                }
                if (state.disposed || !wrap) return;
                const frame = makeSandboxedFrame(
                    src,
                    'nasscad',
                    '',
                    windowId,
                    'vd-nasscad-frame vd-generated-frame',
                    t('desktop.app_nasscad'),
                    {
                        allowSameOrigin: true,
                        allowDownloads: true,
                        allowPointerLock: true,
                        allowFullscreen: true
                    }
                );
                const toolbar = document.createElement('div');
                toolbar.className = 'vd-toolbar vd-nasscad-email-toolbar';
                wrap.replaceChildren(toolbar, frame);
                const binding = window.AuraDesktopEmail?.attach(windowId, '.vd-nasscad-email-toolbar', () => {
                    let exporter;
                    try { exporter = frame.contentWindow?.AuraNasscadEmail; } catch (_) {}
                    return {key:exporter?.documentKey||frame,disabled:state.disposed||!exporter?.ready,formats:exporter?.formats||[],
                        createFiles:async(format,signal)=>{
                            const files=await exporter.createFiles(format,signal);
                            return files && files.map(file=>new File([file],file.name,{type:file.type}));
                        }};
                });
                frame.addEventListener('load', () => {
                    state.emailObserver?.disconnect();
                    try {
                        state.emailObserver = new MutationObserver(() => binding?.refresh());
                        state.emailObserver.observe(frame.contentDocument, {childList:true,subtree:true});
                        if (!frame.contentWindow.AuraNasscadEmail) {
                            const script = frame.contentDocument.createElement('script');
                            const path = '/js/desktop/apps/nasscad-email-frame.js';
                            script.src = window.AuraLazyAssets?.versionedURL(path) || path + '?v=' + encodeURIComponent(window.BUILD_VERSION || 'dev');
                            script.addEventListener('load', () => binding?.refresh());
                            frame.contentDocument.body.append(script);
                        }
                    } catch (_) {}
                    binding?.refresh();
                });
            })
            .catch(err => {
                if (state.disposed || !wrap) return;
                const message = err && err.message ? err.message : t('desktop.nasscad_load_failed');
                wrap.innerHTML = `<div class="vd-nasscad-error">${esc(message)}</div>`;
            });
    }

    function dispose(windowId) {
        const state = instances.get(windowId);
        if (state) { state.disposed = true; state.emailObserver?.disconnect(); }
        instances.delete(windowId);
    }

    window.NasscadApp = { render, dispose };
})();
