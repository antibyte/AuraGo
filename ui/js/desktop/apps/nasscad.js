(function () {
    'use strict';

    const NASSCAD_URL = 'https://www.nasscad.com/NASSCAD_V4_2_7.htm';
    const instances = new Map();

    function render(host, windowId, context) {
        if (!host) return;
        const ctx = context || {};
        const esc = ctx.esc || (value => String(value == null ? '' : value));
        const t = ctx.t || ((key, fallback) => fallback || key);
        const state = { disposed: false };
        instances.set(windowId, state);

        host.innerHTML = `<div class="vd-nasscad">
            <div class="vd-nasscad-loading">${esc(t('desktop.loading', 'Loading...'))}</div>
        </div>`;

        const wrap = host.querySelector('.vd-nasscad');
        const loading = host.querySelector('.vd-nasscad-loading');
        const iframe = document.createElement('iframe');
        iframe.className = 'vd-nasscad-frame';
        iframe.title = t('desktop.app_nasscad', 'NASSCAD');
        iframe.setAttribute('allow', 'clipboard-read; clipboard-write; fullscreen');
        iframe.setAttribute('allowfullscreen', '');
        iframe.referrerPolicy = 'no-referrer-when-downgrade';
        iframe.onload = () => {
            if (loading && loading.parentNode) loading.remove();
        };
        iframe.onerror = () => {
            if (state.disposed || !wrap) return;
            wrap.innerHTML = `<div class="vd-nasscad-error">${esc(t('desktop.nasscad_load_failed', 'Could not load NASSCAD.'))}</div>`;
        };
        iframe.src = NASSCAD_URL;
        wrap.appendChild(iframe);
    }

    function dispose(windowId) {
        const state = instances.get(windowId);
        if (state) state.disposed = true;
        instances.delete(windowId);
    }

    window.NasscadApp = { render, dispose };
})();