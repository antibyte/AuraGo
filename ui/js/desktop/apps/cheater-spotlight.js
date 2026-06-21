(function () {
    'use strict';

    const MAX_VISIBLE = 8;

    function createSpotlight(state) {
        const t = state.t;
        const esc = state.esc;
        const overlay = document.createElement('div');
        overlay.className = 'cheater-spotlight';
        overlay.setAttribute('role', 'dialog');
        overlay.setAttribute('aria-modal', 'true');
        overlay.setAttribute('aria-label', t('cheater.spotlight_title'));
        overlay.innerHTML = `
            <div class="cheater-spotlight-backdrop" data-backdrop></div>
            <div class="cheater-spotlight-panel">
                <input type="text" class="cheater-spotlight-input" data-input
                       placeholder="${esc(t('cheater.spotlight_placeholder'))}"
                       aria-label="${esc(t('cheater.spotlight_input_label'))}"
                       autocomplete="off" spellcheck="false">
                <ul class="cheater-spotlight-results" data-results role="listbox"></ul>
                <div class="cheater-spotlight-hint" data-hint>${esc(t('cheater.spotlight_hint'))}</div>
            </div>
        `;
        document.body.appendChild(overlay);
        const input = overlay.querySelector('[data-input]');
        const results = overlay.querySelector('[data-results]');
        const backdrop = overlay.querySelector('[data-backdrop]');
        let selectedIndex = 0;
        let currentResults = [];

        function close() {
            overlay.remove();
            document.removeEventListener('keydown', onKey, true);
            if (state._spotlight === createSpotlight) state._spotlight = null;
        }

        function onKey(e) {
            if (e.key === 'Escape') { e.preventDefault(); close(); }
            else if (e.key === 'ArrowDown') { e.preventDefault(); moveSelection(1); }
            else if (e.key === 'ArrowUp') { e.preventDefault(); moveSelection(-1); }
            else if (e.key === 'Enter') { e.preventDefault(); confirmSelection(); }
        }

        function moveSelection(delta) {
            if (!currentResults.length) return;
            selectedIndex = (selectedIndex + delta + currentResults.length) % currentResults.length;
            render();
        }

        function confirmSelection() {
            const choice = currentResults[selectedIndex];
            if (!choice) return;
            close();
            if (choice.action === 'create') {
                if (typeof window.CheaterApp.openCreateModal === 'function') {
                    window.CheaterApp.openCreateModal(state.windowId, choice.query || '');
                }
            } else if (choice.action === 'open') {
                if (typeof state.openSheet === 'function') state.openSheet(state, choice.entry);
            }
        }

        function runSearch() {
            const q = input.value.trim();
            if (!q) {
                currentResults = state.searchIndex.slice(0, MAX_VISIBLE).map(e => ({ action: 'open', entry: e }));
                selectedIndex = 0;
                render();
                return;
            }
            const matches = fuzzyFilter(state.searchIndex, q).slice(0, MAX_VISIBLE);
            if (matches.length === 0) {
                currentResults = [{ action: 'create', query: q, label: t('cheater.spotlight_create_fallback').replace('{{query}}', q) }];
            } else {
                currentResults = matches.map(e => ({ action: 'open', entry: e }));
            }
            selectedIndex = 0;
            render();
        }

        function render() {
            if (!currentResults.length) {
                results.innerHTML = `<li class="cheater-spotlight-empty">${esc(t('cheater.spotlight_empty'))}</li>`;
                return;
            }
            results.innerHTML = currentResults.map((r, i) => {
                if (r.action === 'create') {
                    return `<li class="cheater-spotlight-row cheater-spotlight-create ${i === selectedIndex ? 'is-selected' : ''}" role="option" data-index="${i}">${esc(r.label)}</li>`;
                }
                const e = r.entry;
                const tags = (e.tags || []).slice(0, 3).map(tag => `<span class="cheater-pill">${esc(tag)}</span>`).join('');
                const agentBadge = e.last_used_at ? `<span class="cheater-agent-badge" data-agent-badge>🤖 ${esc(formatRelative(e.last_used_at, t))}</span>` : '';
                return `<li class="cheater-spotlight-row ${i === selectedIndex ? 'is-selected' : ''}" role="option" data-index="${i}">
                    <div class="cheater-spotlight-row-main">
                        <strong>${esc(e.name || '(ohne Titel)')}</strong>
                        ${e.abstract ? `<span class="cheater-spotlight-abstract">${esc(e.abstract)}</span>` : ''}
                    </div>
                    <div class="cheater-spotlight-row-meta">${tags}${agentBadge}</div>
                </li>`;
            }).join('');
            results.querySelectorAll('[data-index]').forEach(li => {
                li.addEventListener('click', () => {
                    selectedIndex = Number(li.dataset.index);
                    confirmSelection();
                });
            });
        }

        function formatRelative(iso, t) {
            if (!iso) return '';
            const diff = Date.now() - new Date(iso).getTime();
            if (Number.isNaN(diff)) return '';
            const min = Math.floor(diff / 60000);
            if (min < 1) return t('cheater.just_now');
            if (min < 60) return t('cheater.minutes_ago_short').replace('{{n}}', String(min));
            const hr = Math.floor(min / 60);
            if (hr < 24) return t('cheater.hours_ago_short').replace('{{n}}', String(hr));
            const day = Math.floor(hr / 24);
            return t('cheater.days_ago_short').replace('{{n}}', String(day));
        }

        backdrop.addEventListener('click', close);
        results.addEventListener('contextmenu', (e) => {
            const li = e.target.closest('[data-index]');
            if (!li) return;
            e.preventDefault();
            showContextMenu(e.clientX, e.clientY, currentResults[Number(li.dataset.index)]);
        });
        input.addEventListener('input', debounce(runSearch, 80));
        document.addEventListener('keydown', onKey, true);
        setTimeout(() => input.focus(), 0);
        runSearch();

        function showContextMenu(x, y, result) {
            if (!result || result.action !== 'open') return;
            const existing = document.querySelector('.cheater-context-menu');
            if (existing) existing.remove();
            const menu = document.createElement('ul');
            menu.className = 'cheater-context-menu';
            menu.setAttribute('role', 'menu');
            menu.innerHTML = `<li role="menuitem" data-action="delete">🗑️ ${esc(t('cheater.delete'))}</li>`;
            menu.style.left = x + 'px';
            menu.style.top = y + 'px';
            document.body.appendChild(menu);
            menu.addEventListener('click', (e) => {
                const item = e.target.closest('[data-action]');
                if (!item) return;
                if (item.dataset.action === 'delete') deleteEntry(result.entry);
                menu.remove();
            });
            setTimeout(() => document.addEventListener('click', () => menu.remove(), { once: true }), 0);
        }

        function deleteEntry(entry) {
            const esc = state.esc;
            const t = state.t;
            const confirmModal = document.createElement('div');
            confirmModal.className = 'cheater-modal';
            confirmModal.setAttribute('role', 'dialog');
            confirmModal.setAttribute('aria-modal', 'true');
            confirmModal.setAttribute('aria-label', t('cheater.confirm_delete_title'));
            confirmModal.innerHTML = `
                <div class="cheater-modal-backdrop" data-backdrop></div>
                <div class="cheater-modal-panel">
                    <h2 class="cheater-modal-title">${esc(t('cheater.confirm_delete_title'))}</h2>
                    <p class="cheater-confirm-text">${esc(t('cheater.confirm_delete_text').replace('{{name}}', entry.name || t('cheater.untitled_sheet')))}</p>
                    <div class="cheater-modal-footer">
                        <button type="button" class="cheater-secondary" data-action="cancel">${esc(t('cheater.cancel'))}</button>
                        <button type="button" class="cheater-primary cheater-danger-btn" data-action="confirm">${esc(t('cheater.delete'))}</button>
                    </div>
                </div>`;
            document.body.appendChild(confirmModal);
            const closeConfirm = () => confirmModal.remove();
            confirmModal.querySelector('[data-backdrop]').addEventListener('click', closeConfirm);
            confirmModal.querySelector('[data-action="cancel"]').addEventListener('click', closeConfirm);
            confirmModal.addEventListener('keydown', (e) => { if (e.key === 'Escape') closeConfirm(); });
            confirmModal.querySelector('[data-action="confirm"]').addEventListener('click', async () => {
                closeConfirm();
                const toast = document.createElement('div');
                toast.className = 'cheater-toast';
                toast.innerHTML = `<span>${esc(t('cheater.deleted'))}</span><button data-undo>${esc(t('cheater.undo'))}</button>`;
                document.body.appendChild(toast);
                const commit = async () => {
                    try {
                        await state.api('/api/cheatsheets/' + encodeURIComponent(entry.id), { method: 'DELETE' });
                        state.searchIndex = state.searchIndex.filter(e => e.id !== entry.id);
                        if (typeof state.refreshHome === 'function') state.refreshHome();
                        runSearch();
                    } catch (err) {
                        state.notify('cheater.error.delete_failed', 'error');
                        console.error('cheater delete failed', err);
                    } finally {
                        toast.remove();
                    }
                };
                const timer = setTimeout(commit, 5000);
                toast.querySelector('[data-undo]').addEventListener('click', () => {
                    clearTimeout(timer);
                    toast.remove();
                });
            });
        }

        return { close, runSearch };
    }

    function debounce(fn, ms) {
        let h;
        return function () {
            const args = arguments;
            clearTimeout(h);
            h = setTimeout(() => fn.apply(null, args), ms);
        };
    }

    function fuzzyFilter(entries, query) {
        const q = query.toLowerCase();
        return entries
            .map(e => ({ e, score: scoreEntry(e, q) }))
            .filter(x => x.score > 0)
            .sort((a, b) => b.score - a.score)
            .map(x => x.e);
    }

    function scoreEntry(entry, q) {
        const name = (entry.name || '').toLowerCase();
        const abstract = (entry.abstract || '').toLowerCase();
        const tags = (entry.tags || []).join(' ').toLowerCase();
        const content = (entry.content_excerpt || '').toLowerCase();
        if (name.startsWith(q)) return 100;
        if (name.includes(q)) return 50;
        if (abstract.includes(q)) return 20;
        if (tags.includes(q)) return 30;
        if (content.includes(q)) return 5;
        return 0;
    }

    function openSpotlight(state) {
        if (state._spotlight) {
            state._spotlight.close();
            state._spotlight = null;
            return;
        }
        state._spotlight = createSpotlight(state);
    }

    window.CheaterSpotlight = window.CheaterSpotlight || {};
    window.CheaterSpotlight.open = openSpotlight;
})();
