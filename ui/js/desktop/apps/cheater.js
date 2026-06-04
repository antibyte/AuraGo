(function () {
    'use strict';

    const instances = new Map();
    const SAVE_DEBOUNCE_MS = 500;
    const HOVER_DELAY_MS = 300;

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
        bindGlobalShortcuts(state);
        if (state.searchIndex.length === 0) {
            state.api('/api/cheatsheets')
                .then(list => {
                    const items = Array.isArray(list) ? list : (list.items || list.cheatsheets || []);
                    state.searchIndex = items.map(s => ({
                        id: s.id,
                        name: s.name,
                        abstract: s.abstract || '',
                        tags: s.tags || [],
                        content_excerpt: (s.content || '').slice(0, 200),
                        last_used_at: s.last_used_at || null
                    }));
                })
                .catch(err => console.warn('cheater search index load failed', err));
        }
    }

    function bindGlobalShortcuts(state) {
        state.host.addEventListener('keydown', (e) => {
            if ((e.ctrlKey || e.metaKey) && (e.key === 'k' || e.key === 'K')) {
                e.preventDefault();
                if (window.CheaterSpotlight && typeof window.CheaterSpotlight.open === 'function') {
                    window.CheaterSpotlight.open(state);
                }
            } else if ((e.ctrlKey || e.metaKey) && (e.key === 'n' || e.key === 'N')) {
                e.preventDefault();
                if (window.CheaterApp && typeof window.CheaterApp.openCreateModal === 'function') {
                    window.CheaterApp.openCreateModal(state.windowId);
                }
            }
        });
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
        bindGlobalShortcuts(state);
    }

    function renderAgentBadge(state) { /* implemented in Task 26 */ }

    function markDirty(state) {
        state.dirty = true;
        renderSaveStatus(state, 'saving');
    }

    function scheduleSave(state) {
        if (state.saveTimer) clearTimeout(state.saveTimer);
        state.saveTimer = setTimeout(() => commitSave(state), SAVE_DEBOUNCE_MS);
    }

    async function flushSave(state) {
        if (state.saveTimer) {
            clearTimeout(state.saveTimer);
            state.saveTimer = null;
        }
        if (state.dirty) await commitSave(state);
    }

    async function commitSave(state) {
        if (!state.sheet || !state.dirty) return;
        const sheet = state.sheet;
        const aborter = new AbortController();
        state.currentAbort = aborter;
        renderSaveStatus(state, 'saving');
        try {
            const body = {
                name: sheet.name,
                content: sheet.content
            };
            const updated = await state.api('/api/cheatsheets/' + encodeURIComponent(sheet.id), {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(body),
                signal: aborter.signal
            });
            if (state.currentAbort !== aborter) return; // stale
            state.dirty = false;
            state.lastSavedAt = updated.updated_at || new Date().toISOString();
            state.sheet = Object.assign({}, sheet, updated);
            renderSaveStatus(state, 'saved');
            updateSearchIndexEntry(state, state.sheet);
        } catch (err) {
            if (err && err.name === 'AbortError') return;
            renderSaveStatus(state, 'error');
            state.notify('cheater.error.save_failed', 'error');
            console.error('cheater save failed', err);
        }
    }

    function renderSaveStatus(state, kind) {
        const node = state.host.querySelector('[data-save]');
        if (!node) return;
        const t = state.t;
        if (kind === 'saving') {
            node.dataset.state = 'saving';
            node.textContent = t('cheater.saving', 'Speichert…');
        } else if (kind === 'error') {
            node.dataset.state = 'error';
            node.textContent = t('cheater.save_error', 'Fehler · Erneut versuchen');
        } else {
            delete node.dataset.state;
            const last = state.lastSavedAt;
            node.textContent = t('cheater.saved_ago', 'Gespeichert').replace('{{time}}', formatRelative(last, t));
        }
    }

    function formatRelative(iso, t) {
        if (!iso) return t('cheater.just_now', 'gerade eben');
        const then = new Date(iso).getTime();
        if (Number.isNaN(then)) return t('cheater.just_now', 'gerade eben');
        const diff = Math.max(0, Date.now() - then);
        const sec = Math.floor(diff / 1000);
        if (sec < 60) return t('cheater.seconds_ago', 'vor {{n}}s').replace('{{n}}', String(sec));
        const min = Math.floor(sec / 60);
        if (min < 60) return t('cheater.minutes_ago', 'vor {{n}}m').replace('{{n}}', String(min));
        const hr = Math.floor(min / 60);
        if (hr < 24) return t('cheater.hours_ago', 'vor {{n}}h').replace('{{n}}', String(hr));
        const day = Math.floor(hr / 24);
        return t('cheater.days_ago', 'vor {{n}} Tagen').replace('{{n}}', String(day));
    }

    function updateSearchIndexEntry(state, sheet) {
        if (!sheet) return;
        const idx = state.searchIndex.findIndex(s => s.id === sheet.id);
        const entry = {
            id: sheet.id,
            name: sheet.name,
            abstract: sheet.abstract || '',
            tags: sheet.tags || [],
            content_excerpt: (sheet.content || '').slice(0, 200),
            last_used_at: sheet.last_used_at || null
        };
        if (idx === -1) state.searchIndex.push(entry);
        else state.searchIndex[idx] = entry;
    }

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
                if (state._blockRebuildTimer) clearTimeout(state._blockRebuildTimer);
                state._blockRebuildTimer = setTimeout(() => applyBlockStructure(state), 400);
            });
            source.addEventListener('blur', () => applyBlockStructure(state));
        }
        state.host.addEventListener('keydown', (e) => {
            if ((e.ctrlKey || e.metaKey) && (e.key === 's' || e.key === 'S')) {
                e.preventDefault();
                flushSave(state);
            }
        });
        applyBlockStructure(state);
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

    function bindInlineRender(state) {
        const source = state.host.querySelector('[data-source]');
        if (!source) return;
        source.addEventListener('mouseover', onHover);
        source.addEventListener('mouseout', onLeave);
        source.addEventListener('click', onClickPin);
    }

    function onHover(e) {
        const block = e.target.closest('[data-md-block]');
        if (!block || block.dataset.pinned === '1') return;
        if (block._hoverTimer) return;
        block._hoverTimer = setTimeout(() => renderBlock(block), HOVER_DELAY_MS);
    }

    function onLeave(e) {
        const block = e.target.closest('[data-md-block]');
        if (!block) return;
        if (block._hoverTimer) {
            clearTimeout(block._hoverTimer);
            block._hoverTimer = null;
        }
    }

    function onClickPin(e) {
        const block = e.target.closest('[data-md-block]');
        if (!block) return;
        if (block.dataset.rendered !== '1') {
            renderBlock(block);
            block.dataset.pinned = '1';
        } else {
            unrenderBlock(block);
        }
    }

    function renderBlock(block) {
        if (block.dataset.rendered === '1') return;
        const md = block.textContent;
        try {
            let html = window.marked ? window.marked.parse(md, { gfm: true, breaks: false }) : escHtml(md);
            block.innerHTML = html;
            if (window.hljs && window.hljs.highlightElement) {
                block.querySelectorAll('pre code').forEach(c => window.hljs.highlightElement(c));
            }
            block.dataset.rendered = '1';
            block.classList.add('is-rendered');
        } catch (err) {
            console.error('cheater markdown render failed', err);
        }
    }

    function unrenderBlock(block) {
        const text = block.textContent;
        block.textContent = text;
        delete block.dataset.rendered;
        block.classList.remove('is-rendered');
        if (block.dataset.pinned === '1') delete block.dataset.pinned;
    }

    function splitIntoBlocks(text) {
        const lines = text.split('\n');
        const blocks = [];
        let buf = [];
        let inCode = false;
        for (const line of lines) {
            if (line.trim().startsWith('```')) inCode = !inCode;
            buf.push(line);
            if (!inCode && line.trim() === '') {
                blocks.push(buf.join('\n'));
                buf = [];
            }
        }
        if (buf.length) blocks.push(buf.join('\n'));
        return blocks.filter(b => b.trim().length > 0);
    }

    function applyBlockStructure(state) {
        const source = state.host.querySelector('[data-source]');
        if (!source) return;
        const text = source.textContent;
        const blocks = splitIntoBlocks(text);
        source.innerHTML = blocks.map(b => `<div class="cheater-md-block" data-md-block>${state.esc(b)}</div>`).join('');
        bindInlineRender(state);
    }

    function escHtml(s) {
        return String(s)
            .replaceAll('&', '&amp;')
            .replaceAll('<', '&lt;')
            .replaceAll('>', '&gt;')
            .replaceAll('"', '&quot;')
            .replaceAll("'", '&#39;');
    }

    window.CheaterApp = window.CheaterApp || {};
    window.CheaterApp.render = render;
    window.CheaterApp.dispose = dispose;
    window.CheaterApp.openSheet = openSheet;
})();
