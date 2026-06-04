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

    function renderEditor(state, sheet) {
        const { host, windowId } = state;
        const esc = state.esc;
        const t = state.t;
        state.sheet = sheet;
        state.dirty = false;
        state.lastSavedAt = sheet.updated_at || null;

        host.innerHTML = `<section class="cheater-app" data-cheater="${esc(windowId)}" data-state="editor">
            <header class="cheater-header">
                <button type="button" class="cheater-back" data-action="back" aria-label="${esc(t('cheater.back', 'Zurück zur Suche'))}">←</button>
                <input class="cheater-title" data-title type="text" value="${esc(sheet.name || '')}" spellcheck="false" autocomplete="off">
                <span class="cheater-save" data-save>${esc(t('cheater.saved', 'Gespeichert'))}</span>
                <span class="cheater-agent-badge" data-agent-badge hidden></span>
                <button type="button" class="cheater-attach-btn" data-action="attachments" aria-label="${esc(t('cheater.attachments', 'Anhänge'))}">📎 <span data-attach-count>${(sheet.attachments || []).length}</span></button>
            </header>
            <div class="cheater-content" data-content>
                <pre class="cheater-source" data-source spellcheck="true">${esc(sheet.content || '')}</pre>
            </div>
            <footer class="cheater-footer">
                <span data-charcount>${(sheet.content || '').length}</span> ${esc(t('cheater.chars', 'Zeichen'))} ·
                <span data-help>${esc(t('cheater.hover_help', 'Hover zum Rendern · Ctrl+S zum Speichern'))}</span>
            </footer>
        </section>`;
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

    async function loadSheet(state, id) {
        try {
            const data = await state.api('/api/cheatsheets/' + encodeURIComponent(id));
            openSheet(state, data);
        } catch (err) {
            state.notify('cheater.error.load_failed', 'error');
            console.error('cheater loadSheet failed', err);
        }
    }

    function openSheet(state, sheet) {
        if (!sheet) {
            state.host.innerHTML = '';
            render(state.host, state.windowId, state);
            return;
        }
        renderEditor(state, sheet);
        bindEditorEvents(state);
        renderAgentBadge(state);
        bindBackButton(state);
    }

    function renderAgentBadge(state) { /* implemented in Task 26 */ }

    function markDirty(state) { state.dirty = true; }

    function scheduleSave(state) { /* implemented in Task 14 */ }

    async function flushSave(state) { /* implemented in Task 14 */ }

    function bindEditorEvents(state) {
        const titleInput = state.host.querySelector('[data-title]');
        const source = state.host.querySelector('[data-source]');
        const charCount = state.host.querySelector('[data-charcount]');

        if (titleInput) {
            titleInput.addEventListener('input', () => {
                if (state.sheet) state.sheet.name = titleInput.value;
                markDirty(state);
                scheduleSave(state);
            });
        }
        if (source) {
            source.addEventListener('input', () => {
                if (state.sheet) state.sheet.content = source.textContent;
                if (charCount) charCount.textContent = String(source.textContent.length);
                markDirty(state);
                scheduleSave(state);
            });
        }
    }

    function bindBackButton(state) {
        const back = state.host.querySelector('[data-action="back"]');
        if (!back) return;
        back.addEventListener('click', () => {
            if (state.dirty) flushSave(state).finally(() => goBackToEmpty(state));
            else goBackToEmpty(state);
        });
    }

    function goBackToEmpty(state) {
        if (state.saveTimer) clearTimeout(state.saveTimer);
        state.sheet = null;
        state.host.innerHTML = '';
        render(state.host, state.windowId, state);
    }

    window.CheaterApp = window.CheaterApp || {};
    window.CheaterApp.render = render;
    window.CheaterApp.dispose = dispose;
    window.CheaterApp.openSheet = openSheet;
})();
