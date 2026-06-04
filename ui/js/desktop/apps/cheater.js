(function () {
    'use strict';

    const instances = new Map();

    function render(host, windowId, context) {
        if (!host) return;
        const ctx = context || {};
        const esc = ctx.esc || (value => String(value == null ? '' : value));
        const t = ctx.t || ((key, fallback) => fallback || key);
        const api = ctx.api || fetchJSON;
        const notify = ctx.notify || (() => {});
        const readonly = !!ctx.readonly;

        const state = {
            host,
            windowId,
            esc,
            t,
            api,
            notify,
            iconMarkup: ctx.iconMarkup,
            sheet: null,
            dirty: false,
            lastSavedAt: null,
            saveTimer: null,
            currentAbort: null,
            pollTimer: null,
            searchIndex: []
        };
        instances.set(windowId, state);

        host.innerHTML = `<section class="cheater-app" data-cheater="${esc(windowId)}" data-state="empty">
            <div class="cheater-empty" data-empty>
                <div class="cheater-empty-icon" aria-hidden="true">${ctx.iconMarkup ? ctx.iconMarkup('cheater', '🗂️') : '🗂️'}</div>
                <h1 class="cheater-empty-title">${esc(t('cheater.app_name', 'Cheater'))}</h1>
                <p class="cheater-empty-subtitle">${esc(t('cheater.empty_subtitle', 'Deine Cheat-Sheet-Sammlung'))}</p>
                <button type="button" class="cheater-primary" data-action="create">${esc(t('cheater.empty_cta', 'Erstes Sheet anlegen'))}</button>
                <p class="cheater-empty-hint">${esc(t('cheater.empty_hint', "Tippe irgendwo und drücke Cmd/Ctrl + K, um zu suchen oder zu erstellen"))}</p>
            </div>
        </section>`;

        const section = host.querySelector('[data-cheater]');
        const createBtn = host.querySelector('[data-action="create"]');
        if (createBtn) {
            createBtn.addEventListener('click', () => {
                if (typeof window.CheaterApp.openCreateModal === 'function') {
                    window.CheaterApp.openCreateModal(windowId);
                }
            });
        }

        if (typeof ctx.wireContextMenuBoundary === 'function') ctx.wireContextMenuBoundary(host);
    }

    function dispose(windowId) {
        const state = instances.get(windowId);
        if (state && state.saveTimer) clearTimeout(state.saveTimer);
        instances.delete(windowId);
    }

    async function fetchJSON(url, options) {
        const resp = await fetch(url, options);
        const body = await resp.json().catch(() => ({}));
        if (!resp.ok) throw new Error(body.error || body.message || ('HTTP ' + resp.status));
        return body;
    }

    window.CheaterApp = window.CheaterApp || {};
    window.CheaterApp.render = render;
    window.CheaterApp.dispose = dispose;
})();
