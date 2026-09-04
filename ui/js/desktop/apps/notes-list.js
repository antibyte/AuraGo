(function () {
    'use strict';

    const UNTAGGED = '__untagged__';

    function formatRelative(ms, t) {
        if (!ms) return '';
        const diff = Math.max(0, Date.now() - ms);
        const min = Math.floor(diff / 60000);
        if (min < 1) return t('desktop.notes_time_now');
        if (min < 60) return t('desktop.notes_time_min', { n: min });
        const hr = Math.floor(min / 60);
        if (hr < 24) return t('desktop.notes_time_hour', { n: hr });
        const day = Math.floor(hr / 24);
        return t('desktop.notes_time_day', { n: day });
    }

    function matchesMetadata(note, query) {
        if (!query) return true;
        const q = query.toLowerCase();
        return (note.title || '').toLowerCase().includes(q)
            || (note.name || '').toLowerCase().includes(q)
            || (note.tags || []).some(tag => tag.includes(q));
    }

    function matchesTagFilter(note, tagFilter) {
        if (!tagFilter) return true;
        if (tagFilter === UNTAGGED) return !(note.tags || []).length;
        return (note.tags || []).includes(tagFilter);
    }

    function sortNotes(notes, sortMode, pinned) {
        const pinnedSet = new Set(pinned || []);
        const sorted = notes.slice().sort((a, b) => {
            const aPin = pinnedSet.has(a.path) ? 1 : 0;
            const bPin = pinnedSet.has(b.path) ? 1 : 0;
            if (aPin !== bPin) return bPin - aPin;
            if (sortMode === 'name') return (a.title || a.name || '').localeCompare(b.title || b.name || '');
            return (b.modTime || 0) - (a.modTime || 0);
        });
        return sorted;
    }

    function mount(state, slot) {
        if (!slot) return null;
        const t = state.t;
        const esc = state.esc;

        slot.innerHTML = `<div class="vd-notes-sidebar">
            <div class="vd-notes-side-head">
                <button type="button" class="vd-notes-btn vd-notes-btn-primary vd-notes-new-btn" data-action="new" title="Alt+N">${esc(t('desktop.notes_new'))}</button>
                <label class="vd-notes-sort">
                    <span class="vd-notes-sort-label">${esc(t('desktop.notes_sort'))}</span>
                    <select data-notes-sort>
                        <option value="modified">${esc(t('desktop.notes_sort_modified'))}</option>
                        <option value="name">${esc(t('desktop.notes_sort_name'))}</option>
                    </select>
                </label>
            </div>
            <div class="vd-notes-search">
                <input type="search" data-notes-search placeholder="${esc(t('desktop.notes_search'))}" autocomplete="off" spellcheck="false">
            </div>
            <div class="vd-notes-tagrow" data-notes-tagrow></div>
            <ul class="vd-notes-list" data-notes-list role="list"></ul>
            <div class="vd-notes-side-foot"><span data-notes-count></span></div>
        </div>`;

        const searchInput = slot.querySelector('[data-notes-search]');
        const sortSelect = slot.querySelector('[data-notes-sort]');
        const tagRow = slot.querySelector('[data-notes-tagrow]');
        const listEl = slot.querySelector('[data-notes-list]');
        const countEl = slot.querySelector('[data-notes-count]');
        const newBtn = slot.querySelector('[data-action="new"]');

        let cardClickHandler = null;

        if (newBtn) {
            newBtn.disabled = !!state.readonly;
            newBtn.addEventListener('click', () => {
                if (typeof state.onNewNote === 'function') state.onNewNote();
            });
        }
        if (searchInput) {
            searchInput.addEventListener('input', () => {
                state.searchQuery = searchInput.value;
                if (typeof state.onSearchInput === 'function') state.onSearchInput(searchInput.value);
                update();
            });
            searchInput.addEventListener('keydown', (e) => {
                if (e.key === 'Enter') {
                    e.preventDefault();
                    if (typeof state.onSearchSubmit === 'function') state.onSearchSubmit(searchInput.value);
                }
            });
        }
        if (sortSelect) {
            sortSelect.value = state.sortMode === 'name' ? 'name' : 'modified';
            sortSelect.addEventListener('change', () => {
                state.sortMode = sortSelect.value === 'name' ? 'name' : 'modified';
                if (typeof state.onSortChange === 'function') state.onSortChange(state.sortMode);
                update();
            });
        }

        function collectTags(notes) {
            const counts = new Map();
            notes.forEach(note => (note.tags || []).forEach(tag => {
                counts.set(tag, (counts.get(tag) || 0) + 1);
            }));
            return Array.from(counts.entries()).sort((a, b) => b[1] - a[1] || a[0].localeCompare(b[0])).slice(0, 24);
        }

        function renderTagRow(notes) {
            if (!tagRow) return;
            const hasTags = notes.some(note => (note.tags || []).length);
            if (!hasTags) {
                tagRow.hidden = true;
                tagRow.innerHTML = '';
                return;
            }
            tagRow.hidden = false;
            const chips = [`<button type="button" class="vd-notes-chip${!state.tagFilter ? ' is-active' : ''}" data-tag-filter="">${esc(t('desktop.notes_all_tags'))}</button>`];
            const tags = collectTags(notes);
            if (notes.some(note => !(note.tags || []).length)) {
                chips.push(`<button type="button" class="vd-notes-chip${state.tagFilter === UNTAGGED ? ' is-active' : ''}" data-tag-filter="${UNTAGGED}">${esc(t('desktop.notes_untagged'))}</button>`);
            }
            tags.forEach(([tag, count]) => {
                chips.push(`<button type="button" class="vd-notes-chip${state.tagFilter === tag ? ' is-active' : ''}" data-tag-filter="${esc(tag)}">${esc(tag)} <span class="vd-notes-chip-count">${count}</span></button>`);
            });
            tagRow.innerHTML = chips.join('');
        }

        function renderResults(results) {
            if (!listEl) return;
            if (!results || !results.length) {
                listEl.innerHTML = `<li class="vd-notes-list-empty">${esc(t('desktop.notes_no_results'))}</li>`;
                return;
            }
            listEl.innerHTML = '';
            results.forEach(result => {
                const li = document.createElement('li');
                li.className = 'vd-notes-card is-result';
                li.setAttribute('role', 'listitem');
                const isActive = result.path === state.currentPath;
                if (isActive) li.classList.add('is-active');
                const btn = document.createElement('button');
                btn.type = 'button';
                btn.className = 'vd-notes-card-btn';
                btn.dataset.notePath = result.path;
                const title = document.createElement('span');
                title.className = 'vd-notes-card-title';
                title.textContent = result.title || result.name;
                const snippet = document.createElement('span');
                snippet.className = 'vd-notes-card-snippet';
                appendHighlighted(snippet, result.snippet, state.searchQuery);
                const footer = document.createElement('span');
                footer.className = 'vd-notes-card-footer';
                const meta = document.createElement('span');
                meta.className = 'vd-notes-card-meta';
                meta.textContent = formatRelative(result.modTime, t);
                footer.appendChild(meta);
                btn.appendChild(title);
                btn.appendChild(snippet);
                btn.appendChild(footer);
                li.appendChild(btn);
                listEl.appendChild(li);
            });
            bindCardClicks();
        }

        function appendHighlighted(node, snippet, query) {
            const text = String(snippet || '');
            const q = String(query || '').trim();
            if (!q) {
                node.textContent = text;
                return;
            }
            const lower = text.toLowerCase();
            const needle = q.toLowerCase();
            let idx = 0;
            let found = lower.indexOf(needle);
            let guard = 0;
            while (found !== -1 && guard < 50) {
                if (found > idx) node.appendChild(document.createTextNode(text.slice(idx, found)));
                const mark = document.createElement('mark');
                mark.textContent = text.slice(found, found + needle.length);
                node.appendChild(mark);
                idx = found + needle.length;
                found = lower.indexOf(needle, idx);
                guard++;
            }
            if (idx < text.length) node.appendChild(document.createTextNode(text.slice(idx)));
        }

        function renderCards(notes) {
            if (!listEl) return;
            if (!state.notes.length) {
                const icon = state.iconMarkup ? state.iconMarkup('notes', '📝') : '📝';
                listEl.innerHTML = `<li class="vd-notes-onboarding">
                    <div class="vd-notes-onboarding-icon" aria-hidden="true">${icon}</div>
                    <h2 class="vd-notes-onboarding-title">${esc(t('desktop.notes_empty_title'))}</h2>
                    <p class="vd-notes-onboarding-hint">${esc(t('desktop.notes_empty_hint'))}</p>
                    ${state.readonly ? '' : `<button type="button" class="vd-notes-btn vd-notes-btn-primary" data-action="new">${esc(t('desktop.notes_new'))}</button>`}
                </li>`;
                const onboardNew = listEl.querySelector('[data-action="new"]');
                if (onboardNew) onboardNew.addEventListener('click', () => {
                    if (typeof state.onNewNote === 'function') state.onNewNote();
                });
                return;
            }
            if (!notes.length) {
                listEl.innerHTML = `<li class="vd-notes-list-empty">${esc(t('desktop.notes_no_results'))}</li>`;
                return;
            }
            const pinnedSet = new Set((state.meta && state.meta.pinned) || []);
            listEl.innerHTML = notes.map(note => {
                const isActive = note.path === state.currentPath || (note.isNew && state.currentIsNew);
                const tags = (note.tags || []).slice(0, 4).map(tag => `<span class="vd-notes-pill">${esc(tag)}</span>`).join('');
                const pinned = pinnedSet.has(note.path);
                const meta = note.isNew ? '' : `<span class="vd-notes-card-meta">${esc(formatRelative(note.modTime, t))}</span>`;
                const dirty = note.dirty ? '<span class="vd-notes-card-dirty" title="' + esc(t('desktop.notes_unsaved')) + '">●</span>' : '';
                return `<li class="vd-notes-card${isActive ? ' is-active' : ''}${pinned ? ' is-pinned' : ''}" role="listitem">
                    <button type="button" class="vd-notes-card-btn" data-note-path="${esc(note.path)}">
                        <span class="vd-notes-card-title">${esc(note.title || note.name || t('desktop.notes_unsaved'))}</span>
                        ${note.snippet ? `<span class="vd-notes-card-snippet">${esc(note.snippet)}</span>` : ''}
                        <span class="vd-notes-card-footer">${tags}${meta}${dirty}</span>
                    </button>
                    ${state.readonly ? '' : `<button type="button" class="vd-notes-pin-btn${pinned ? ' is-pinned' : ''}" data-pin-path="${esc(note.path)}" title="${esc(pinned ? t('desktop.notes_unpin') : t('desktop.notes_pin'))}" aria-label="${esc(pinned ? t('desktop.notes_unpin') : t('desktop.notes_pin'))}" aria-pressed="${pinned}">${pinned ? '★' : '☆'}</button>`}
                </li>`;
            }).join('');
            bindCardClicks();
        }

        function bindCardClicks() {
            if (!listEl) return;
            if (cardClickHandler) listEl.removeEventListener('click', cardClickHandler);
            cardClickHandler = (e) => {
                const pinBtn = e.target.closest('[data-pin-path]');
                if (pinBtn) {
                    e.preventDefault();
                    if (typeof state.onTogglePin === 'function') state.onTogglePin(pinBtn.dataset.pinPath);
                    return;
                }
                const cardBtn = e.target.closest('[data-note-path]');
                if (!cardBtn) return;
                if (typeof state.onSelectNote === 'function') state.onSelectNote(cardBtn.dataset.notePath);
            };
            listEl.addEventListener('click', cardClickHandler);
        }

        function update() {
            if (!slot) return;
            const notes = state.notes || [];
            renderTagRow(notes);
            if (state.contentResults) {
                renderResults(state.contentResults);
                if (countEl) countEl.textContent = t('desktop.notes_notes_count', { count: state.contentResults.length });
                return;
            }
            const visible = sortNotes(
                notes.filter(note => matchesMetadata(note, state.searchQuery) && matchesTagFilter(note, state.tagFilter)),
                state.sortMode,
                (state.meta && state.meta.pinned) || []
            );
            renderCards(visible);
            if (countEl) countEl.textContent = t('desktop.notes_notes_count', { count: visible.length });
        }

        function setSearchValue(value) {
            if (searchInput && searchInput.value !== value) searchInput.value = value;
        }

        function focusSearch() {
            if (searchInput) {
                searchInput.focus();
                searchInput.select();
            }
        }

        function clearSearch() {
            state.searchQuery = '';
            state.contentResults = null;
            setSearchValue('');
            if (typeof state.onSearchClear === 'function') state.onSearchClear();
            update();
        }

        if (typeof state.addCleanup === 'function') {
            state.addCleanup(() => {
                if (cardClickHandler && listEl) listEl.removeEventListener('click', cardClickHandler);
            });
        }

        return { update, focusSearch, clearSearch, setSearchValue };
    }

    window.NotesList = { mount, sortNotes };
})();
