(function () {
    'use strict';

    // NoisemakerLibrary - song grid/list for the Noisemaker desktop app.
    // Factory: create(deps) -> controller. deps = { esc, t, lang, readonly }.
    // t(key, params, fallback) resolves desktop.noisemaker_<key>.
    // Events: play(track, list), enqueue(tracks), favorite(track, value), delete(tracks),
    // template(track), download(tracks), contextmenu({ x, y, track|null }), create(),
    // loadmore(), search(query), filter(name), view(name), selection(ids).

    const SVG = {
        play: '<svg class="nm-svg-play" viewBox="0 0 16 16" width="16" height="16" aria-hidden="true"><path fill="currentColor" d="M4 2.5v11l9-5.5z"/></svg>',
        pause: '<svg class="nm-svg-pause" viewBox="0 0 16 16" width="16" height="16" aria-hidden="true"><path fill="currentColor" d="M3 2.5h3.4v11H3zM9.6 2.5H13v11H9.6z"/></svg>',
        heart: '<svg viewBox="0 0 16 16" width="15" height="15" aria-hidden="true"><path class="nm-heart-path" d="M8 13.6 2.9 8.7A3.1 3.1 0 0 1 7.3 4.3L8 5l.7-.7a3.1 3.1 0 0 1 4.4 4.4z" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linejoin="round"/></svg>',
        download: '<svg viewBox="0 0 16 16" width="15" height="15" aria-hidden="true"><path d="M8 2v8m0 0 3-3M8 10 5 7M3 13h10" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"/></svg>',
        trash: '<svg viewBox="0 0 16 16" width="15" height="15" aria-hidden="true"><path d="M3 4h10M6 4V2.5h4V4M4.5 4l.6 9h5.8l.6-9" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/></svg>',
        template: '<svg viewBox="0 0 16 16" width="15" height="15" aria-hidden="true"><path d="M2.5 13.5 5 13l7.5-7.5-2.5-2.5L2.5 10.5zM9 4l2.5 2.5" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/></svg>',
        more: '<svg viewBox="0 0 16 16" width="15" height="15" aria-hidden="true"><circle cx="3" cy="8" r="1.4" fill="currentColor"/><circle cx="8" cy="8" r="1.4" fill="currentColor"/><circle cx="13" cy="8" r="1.4" fill="currentColor"/></svg>',
        check: '<svg viewBox="0 0 16 16" width="12" height="12" aria-hidden="true"><path d="M3 8.5 6.5 12 13 4.5" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"/></svg>',
        grid: '<svg viewBox="0 0 16 16" width="14" height="14" aria-hidden="true"><path fill="currentColor" d="M2 2h5v5H2zM9 2h5v5H9zM2 9h5v5H2zM9 9h5v5H9z"/></svg>',
        list: '<svg viewBox="0 0 16 16" width="14" height="14" aria-hidden="true"><path fill="currentColor" d="M2 3h12v2H2zM2 7h12v2H2zM2 11h12v2H2z"/></svg>',
        search: '<svg viewBox="0 0 16 16" width="14" height="14" aria-hidden="true"><circle cx="7" cy="7" r="4.2" fill="none" stroke="currentColor" stroke-width="1.6"/><path d="m10.2 10.2 3.3 3.3" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"/></svg>',
        note: '<svg viewBox="0 0 24 24" width="28" height="28" aria-hidden="true"><path fill="currentColor" d="M9 3v10.55A4 4 0 1 0 11 17V7h5V3z"/></svg>',
        close: '<svg viewBox="0 0 16 16" width="14" height="14" aria-hidden="true"><path d="M4 4l8 8M12 4l-8 8" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"/></svg>'
    };

    function formatDuration(ms) {
        const total = Math.max(0, Math.round((Number(ms) || 0) / 1000));
        const m = Math.floor(total / 60);
        const s = total % 60;
        return m + ':' + (s < 10 ? '0' : '') + s;
    }

    function formatDate(value, lang) {
        if (!value) return '';
        const raw = String(value);
        const date = new Date(raw.includes('T') ? raw : raw.replace(' ', 'T') + 'Z');
        if (Number.isNaN(date.getTime())) return raw;
        try {
            return new Intl.DateTimeFormat(lang || undefined, { dateStyle: 'medium' }).format(date);
        } catch (_) {
            return date.toLocaleDateString();
        }
    }

    function cssEscape(value) {
        return window.CSS && typeof CSS.escape === 'function' ? CSS.escape(String(value)) : String(value).replace(/["\\]/g, '\\$&');
    }

    function create(deps) {
        const esc = deps.esc || (v => String(v == null ? '' : v));
        const t = deps.t || ((key, params, fallback) => fallback || key);
        const lang = deps.lang || '';
        const readonly = !!deps.readonly;
        const handlers = {};
        const on = (name, cb) => { (handlers[name] = handlers[name] || []).push(cb); };
        const emit = (name, ...args) => {
            (handlers[name] || []).forEach(cb => {
                try { cb(...args); } catch (err) { console.warn('Noisemaker library handler failed', err); }
            });
        };

        let tracks = [];
        let query = '';
        let filter = 'all';
        let view = 'grid';
        let loading = false;
        let selectMode = false;
        let anchorId = null;
        let currentId = null;
        let playing = false;
        let disposed = false;
        let pagination = { total: 0, hasMore: false, loading: false };
        let observer = null;
        let searchTimer = null;
        let highlightTimer = null;
        const selected = new Set();

        const root = document.createElement('div');
        root.className = 'nm-library';
        root.innerHTML = toolbarMarkup() + '<div class="nm-grid" role="list" tabindex="0" data-nm-grid></div>' + selectionBarMarkup();
        const gridEl = root.querySelector('[data-nm-grid]');
        const searchInput = root.querySelector('[data-nm-search]');
        const countEl = root.querySelector('[data-nm-count]');
        const selectionBar = root.querySelector('[data-nm-selection-bar]');
        const selectionCount = root.querySelector('[data-nm-selection-count]');

        function toolbarMarkup() {
            const search = t('library_search');
            return `<div class="nm-library-toolbar">
                <label class="nm-search">
                    <span class="nm-search-glyph">${SVG.search}</span>
                    <input class="nm-input nm-search-input" type="search" data-nm-search placeholder="${esc(search)}" aria-label="${esc(search)}" autocomplete="off">
                </label>
                <div class="nm-segment" role="group">
                    <button type="button" class="nm-segment-btn is-active" data-nm-filter="all" aria-pressed="true">${esc(t('filter_all'))}</button>
                    <button type="button" class="nm-segment-btn" data-nm-filter="favorites" aria-pressed="false">${SVG.heart}<span>${esc(t('filter_favorites'))}</span></button>
                </div>
                <span class="nm-toolbar-count nm-muted" data-nm-count></span>
                <span class="nm-toolbar-spacer"></span>
                <div class="nm-segment nm-segment--icons" role="group">
                    <button type="button" class="nm-segment-btn is-active" data-nm-view="grid" aria-pressed="true" title="${esc(t('view_grid'))}" aria-label="${esc(t('view_grid'))}">${SVG.grid}</button>
                    <button type="button" class="nm-segment-btn" data-nm-view="list" aria-pressed="false" title="${esc(t('view_list'))}" aria-label="${esc(t('view_list'))}">${SVG.list}</button>
                </div>
                <button type="button" class="nm-btn nm-select-toggle" data-nm-select-toggle aria-pressed="false">${SVG.check}<span>${esc(t('select'))}</span></button>
            </div>`;
        }

        function selectionBarMarkup() {
            return `<div class="nm-selection-bar" data-nm-selection-bar hidden role="toolbar">
                <span class="nm-selection-count" data-nm-selection-count></span>
                <div class="nm-selection-actions">
                    <button type="button" class="nm-btn nm-btn--primary" data-nm-sel="play">${SVG.play}<span>${esc(t('player_play'))}</span></button>
                    <button type="button" class="nm-btn" data-nm-sel="enqueue">${esc(t('enqueue'))}</button>
                    <button type="button" class="nm-btn" data-nm-sel="download">${SVG.download}<span>${esc(t('track_download'))}</span></button>
                    ${readonly ? '' : `<button type="button" class="nm-btn nm-btn--danger" data-nm-sel="delete">${SVG.trash}<span>${esc(t('track_delete'))}</span></button>`}
                </div>
                <button type="button" class="nm-icon-btn" data-nm-sel="close" aria-label="${esc(t('select_none'))}" title="${esc(t('select_none'))}">${SVG.close}</button>
            </div>`;
        }

        function trackTitle(track) { return track.title || t('result_untitled'); }
        function trackStyle(track) { return track.style || ''; }

        function coverMarkup(track, extraClass) {
            const inner = track.cover_url
                ? `<img src="${esc(track.cover_url)}" alt="" loading="lazy" decoding="async">`
                : `<span class="nm-cover-fallback">${SVG.note}</span>`;
            return `<div class="nm-cover ${extraClass || ''}">${inner}<span class="nm-eq" aria-hidden="true"><i></i><i></i><i></i></span></div>`;
        }

        function metaMarkup(track) {
            const parts = [];
            if (track.duration_ms) parts.push(formatDuration(track.duration_ms));
            const date = formatDate(track.created_at, lang);
            if (date) parts.push(date);
            if (track.provider) parts.push(track.provider);
            return parts.map(p => `<span>${esc(p)}</span>`).join('<span class="nm-dot" aria-hidden="true">·</span>');
        }

        function stateClasses(track) {
            const id = String(track.id);
            let classes = '';
            if (id === currentId) classes += ' is-current' + (playing ? ' is-playing' : '');
            if (selected.has(id)) classes += ' is-selected';
            if (track.favorite) classes += ' is-favorite';
            return classes;
        }

        function actionsMarkup(track) {
            return `<div class="nm-card-actions">
                <button type="button" class="nm-icon-btn nm-act-template" title="${esc(t('track_use_template'))}" aria-label="${esc(t('track_use_template'))}">${SVG.template}</button>
                <button type="button" class="nm-icon-btn nm-act-download" title="${esc(t('track_download'))}" aria-label="${esc(t('track_download'))}">${SVG.download}</button>
                ${readonly ? '' : `<button type="button" class="nm-icon-btn nm-icon-btn--danger nm-act-delete" title="${esc(t('track_delete'))}" aria-label="${esc(t('track_delete'))}">${SVG.trash}</button>`}
                <button type="button" class="nm-icon-btn nm-act-more" title="${esc(t('more_actions'))}" aria-label="${esc(t('more_actions'))}" aria-haspopup="menu">${SVG.more}</button>
            </div>`;
        }

        function favMarkup(track) {
            const label = track.favorite ? t('favorite_remove') : t('favorite_add');
            return `<button type="button" class="nm-card-fav" aria-pressed="${track.favorite ? 'true' : 'false'}" aria-label="${esc(label)}" title="${esc(label)}" ${readonly ? 'disabled' : ''}>${SVG.heart}</button>`;
        }

        function checkMarkup(track) {
            return `<button type="button" class="nm-card-check" aria-pressed="${selected.has(String(track.id)) ? 'true' : 'false'}" aria-label="${esc(t('select'))}" tabindex="-1">${SVG.check}</button>`;
        }

        function cardMarkup(track) {
            const title = trackTitle(track);
            const id = String(track.id);
            const style = trackStyle(track);
            return `<article class="nm-card${stateClasses(track)}" role="listitem" data-track-id="${esc(id)}" tabindex="0" aria-selected="${selected.has(id) ? 'true' : 'false'}" aria-label="${esc(title)}">
                ${checkMarkup(track)}
                <div class="nm-card-media">
                    ${coverMarkup(track, 'nm-card-cover')}
                    <button type="button" class="nm-card-play" aria-label="${esc(t('player_play'))}" tabindex="-1">${SVG.play}${SVG.pause}</button>
                    ${favMarkup(track)}
                </div>
                <div class="nm-card-body">
                    <div class="nm-card-title" title="${esc(title)}">${esc(title)}</div>
                    <div class="nm-card-style" title="${esc(style)}">${esc(style)}</div>
                    <div class="nm-card-meta nm-muted">${metaMarkup(track)}</div>
                    <div class="nm-card-foot">
                        ${track.instrumental ? `<span class="nm-tag">${esc(t('instrumental_tag'))}</span>` : '<span></span>'}
                        ${actionsMarkup(track)}
                    </div>
                </div>
            </article>`;
        }

        function rowMarkup(track) {
            const title = trackTitle(track);
            const id = String(track.id);
            const style = trackStyle(track);
            return `<article class="nm-row${stateClasses(track)}" role="listitem" data-track-id="${esc(id)}" tabindex="0" aria-selected="${selected.has(id) ? 'true' : 'false'}" aria-label="${esc(title)}">
                ${checkMarkup(track)}
                <div class="nm-row-media">
                    ${coverMarkup(track, 'nm-row-cover')}
                    <button type="button" class="nm-card-play" aria-label="${esc(t('player_play'))}" tabindex="-1">${SVG.play}${SVG.pause}</button>
                </div>
                <div class="nm-row-main">
                    <div class="nm-card-title" title="${esc(title)}">${esc(title)}</div>
                    <div class="nm-card-style nm-muted" title="${esc(style)}">${esc(style)}</div>
                </div>
                <span class="nm-row-cell nm-row-duration">${track.duration_ms ? esc(formatDuration(track.duration_ms)) : '—'}</span>
                <span class="nm-row-cell nm-row-date nm-muted">${esc(formatDate(track.created_at, lang))}</span>
                <span class="nm-row-cell nm-row-provider nm-muted">${esc(track.provider || '')}</span>
                ${favMarkup(track)}
                ${actionsMarkup(track)}
            </article>`;
        }

        function skeletonMarkup() {
            let html = '';
            for (let i = 0; i < 8; i++) {
                html += '<div class="nm-skeleton" aria-hidden="true"><div class="nm-skeleton-cover"></div><div class="nm-skeleton-line"></div><div class="nm-skeleton-line nm-skeleton-line--short"></div></div>';
            }
            return html;
        }

        function emptyMarkup() {
            if (loading) return skeletonMarkup();
            if (query) return `<div class="nm-empty"><div class="nm-empty-icon">${SVG.search}</div><div class="nm-empty-title">${esc(t('no_results'))}</div></div>`;
            if (filter === 'favorites') return `<div class="nm-empty"><div class="nm-empty-icon">${SVG.heart}</div><div class="nm-empty-title">${esc(t('no_favorites_title'))}</div><div class="nm-empty-hint nm-muted">${esc(t('no_favorites_hint'))}</div></div>`;
            return `<div class="nm-empty"><div class="nm-empty-icon">${SVG.note}</div><div class="nm-empty-title">${esc(t('library_empty_title'))}</div><div class="nm-empty-hint nm-muted">${esc(t('library_empty_hint'))}</div><button type="button" class="nm-btn nm-btn--primary nm-empty-cta" data-nm-empty-create>${esc(t('library_empty_cta'))}</button></div>`;
        }

        function footMarkup() {
            if (!tracks.length) return '';
            const total = pagination.total || tracks.length;
            const shown = t('showing_of', { loaded: tracks.length, total }, tracks.length + ' / ' + total);
            const more = pagination.hasMore
                ? `<button type="button" class="nm-btn nm-load-more" data-nm-load-more ${pagination.loading ? 'disabled' : ''}>${esc(pagination.loading ? t('library_loading') : t('load_more'))}</button>`
                : '';
            return `<div class="nm-grid-foot" data-nm-foot><span class="nm-muted">${esc(shown)}</span>${more}</div>`;
        }

        function itemsMarkup(list) {
            return view === 'list' ? list.map(rowMarkup).join('') : list.map(cardMarkup).join('');
        }

        function renderGrid() {
            if (disposed) return;
            disconnectObserver();
            gridEl.classList.toggle('nm-grid--list', view === 'list');
            gridEl.classList.toggle('is-empty', !tracks.length);
            gridEl.innerHTML = tracks.length ? itemsMarkup(tracks) + footMarkup() : emptyMarkup();
            observeFoot();
            updateCount();
        }

        function renderFoot() {
            disconnectObserver();
            const foot = gridEl.querySelector('[data-nm-foot]');
            if (foot) foot.remove();
            if (tracks.length) gridEl.insertAdjacentHTML('beforeend', footMarkup());
            observeFoot();
            updateCount();
        }

        function updateCount() {
            const total = pagination.total || tracks.length;
            countEl.textContent = tracks.length ? t('showing_of', { loaded: tracks.length, total }, tracks.length + ' / ' + total) : '';
        }

        function observeFoot() {
            const btn = gridEl.querySelector('[data-nm-load-more]');
            if (!btn || typeof IntersectionObserver !== 'function') return;
            observer = new IntersectionObserver(entries => {
                if (entries.some(e => e.isIntersecting) && pagination.hasMore && !pagination.loading) emit('loadmore');
            }, { root: gridEl, rootMargin: '240px' });
            observer.observe(btn);
        }

        function disconnectObserver() {
            if (observer) observer.disconnect();
            observer = null;
        }

        function trackById(id) {
            const key = String(id);
            return tracks.find(x => String(x.id) === key) || null;
        }
        function elementById(id) {
            return gridEl.querySelector('[data-track-id="' + cssEscape(id) + '"]');
        }
        function selectedTracks() { return tracks.filter(x => selected.has(String(x.id))); }
        function isSelected(track) { return !!track && selected.has(String(track.id)); }
        function targetsFor(track) { return isSelected(track) ? selectedTracks() : [track]; }

        function syncSelectionUI() {
            gridEl.querySelectorAll('[data-track-id]').forEach(el => {
                const on = selected.has(el.dataset.trackId);
                el.classList.toggle('is-selected', on);
                el.setAttribute('aria-selected', on ? 'true' : 'false');
                const check = el.querySelector('.nm-card-check');
                if (check) check.setAttribute('aria-pressed', on ? 'true' : 'false');
            });
            const n = selected.size;
            selectionBar.hidden = n === 0;
            selectionCount.textContent = n === 1 ? t('selected_count_one') : t('selected_count', { count: n });
            root.classList.toggle('has-selection', n > 0);
            emit('selection', Array.from(selected));
        }

        function setSelectMode(on) {
            selectMode = !!on;
            root.classList.toggle('is-select-mode', selectMode);
            const btn = root.querySelector('[data-nm-select-toggle]');
            if (btn) {
                btn.setAttribute('aria-pressed', selectMode ? 'true' : 'false');
                btn.classList.toggle('is-active', selectMode);
            }
            if (!selectMode) selected.clear();
            syncSelectionUI();
        }

        function toggleSelection(track) {
            const id = String(track.id);
            if (selected.has(id)) selected.delete(id); else selected.add(id);
            anchorId = id;
            syncSelectionUI();
        }

        function selectRange(track) {
            const ids = tracks.map(x => String(x.id));
            const to = ids.indexOf(String(track.id));
            let from = anchorId ? ids.indexOf(anchorId) : -1;
            if (from < 0) from = to;
            const start = Math.min(from, to);
            const end = Math.max(from, to);
            for (let i = start; i <= end; i++) selected.add(ids[i]);
            syncSelectionUI();
        }

        function selectAll() {
            tracks.forEach(x => selected.add(String(x.id)));
            if (!selectMode) setSelectMode(true); else syncSelectionUI();
        }

        function clearSelection() {
            if (!selected.size) return;
            selected.clear();
            syncSelectionUI();
        }

        gridEl.addEventListener('click', event => {
            if (event.target.closest('[data-nm-empty-create]')) { emit('create'); return; }
            if (event.target.closest('[data-nm-load-more]')) { emit('loadmore'); return; }
            const el = event.target.closest('[data-track-id]');
            if (!el) return;
            const track = trackById(el.dataset.trackId);
            if (!track) return;
            if (event.target.closest('.nm-card-check')) {
                if (event.shiftKey) selectRange(track); else toggleSelection(track);
                return;
            }
            if (event.target.closest('.nm-card-fav')) { if (!readonly) emit('favorite', track, !track.favorite); return; }
            if (event.target.closest('.nm-act-delete')) { emit('delete', targetsFor(track)); return; }
            if (event.target.closest('.nm-act-download')) { emit('download', targetsFor(track)); return; }
            if (event.target.closest('.nm-act-template')) { emit('template', track); return; }
            const more = event.target.closest('.nm-act-more');
            if (more) {
                const rect = more.getBoundingClientRect();
                emit('contextmenu', { x: rect.left, y: rect.bottom + 4, track });
                return;
            }
            if (event.target.closest('.nm-card-play')) { emit('play', track, tracks.slice()); return; }
            if (event.shiftKey) { event.preventDefault(); selectRange(track); return; }
            if (event.ctrlKey || event.metaKey || selectMode) { toggleSelection(track); return; }
            emit('play', track, tracks.slice());
        });

        gridEl.addEventListener('contextmenu', event => {
            if (event.target.closest('input, textarea')) return;
            event.preventDefault();
            event.stopPropagation();
            const el = event.target.closest('[data-track-id]');
            const track = el ? trackById(el.dataset.trackId) : null;
            emit('contextmenu', { x: event.clientX, y: event.clientY, track });
        });

        gridEl.addEventListener('keydown', event => {
            if ((event.ctrlKey || event.metaKey) && (event.key === 'a' || event.key === 'A')) {
                if (tracks.length) { event.preventDefault(); selectAll(); }
                return;
            }
            if (event.key === 'Escape') {
                if (selected.size || selectMode) { event.preventDefault(); setSelectMode(false); }
                return;
            }
            const el = event.target.closest('[data-track-id]');
            if (!el) return;
            const track = trackById(el.dataset.trackId);
            if (!track) return;
            if (event.key === 'Enter' || event.key === ' ') {
                event.preventDefault();
                if (selectMode || event.ctrlKey || event.metaKey) toggleSelection(track);
                else emit('play', track, tracks.slice());
            } else if (event.key === 'ContextMenu' || (event.shiftKey && event.key === 'F10')) {
                event.preventDefault();
                const rect = el.getBoundingClientRect();
                emit('contextmenu', { x: rect.left + 24, y: rect.top + 24, track });
            }
        });

        root.querySelector('.nm-library-toolbar').addEventListener('click', event => {
            const filterBtn = event.target.closest('[data-nm-filter]');
            if (filterBtn) {
                if (filterBtn.dataset.nmFilter !== filter) { setFilter(filterBtn.dataset.nmFilter); emit('filter', filter); }
                return;
            }
            const viewBtn = event.target.closest('[data-nm-view]');
            if (viewBtn) {
                if (viewBtn.dataset.nmView !== view) { setView(viewBtn.dataset.nmView); emit('view', view); }
                return;
            }
            if (event.target.closest('[data-nm-select-toggle]')) setSelectMode(!selectMode);
        });

        searchInput.addEventListener('input', () => {
            clearTimeout(searchTimer);
            searchTimer = setTimeout(() => {
                const value = searchInput.value.trim();
                if (value !== query) { query = value; emit('search', query); }
            }, 300);
        });
        searchInput.addEventListener('keydown', event => {
            if (event.key === 'Escape' && searchInput.value) {
                event.stopPropagation();
                searchInput.value = '';
                clearTimeout(searchTimer);
                query = '';
                emit('search', '');
            }
        });

        selectionBar.addEventListener('click', event => {
            const btn = event.target.closest('[data-nm-sel]');
            if (!btn) return;
            const list = selectedTracks();
            switch (btn.dataset.nmSel) {
                case 'play': if (list.length) emit('play', list[0], list); break;
                case 'enqueue': emit('enqueue', list); break;
                case 'download': emit('download', list); break;
                case 'delete': emit('delete', list); break;
                case 'close': setSelectMode(false); break;
                default: break;
            }
        });

        function setTracks(list) {
            tracks = (list || []).slice();
            Array.from(selected).forEach(id => { if (!trackById(id)) selected.delete(id); });
            renderGrid();
            syncSelectionUI();
        }

        function appendTracks(list) {
            const more = (list || []).filter(Boolean);
            if (!more.length) return;
            if (!tracks.length) { setTracks(more); return; }
            tracks = tracks.concat(more);
            const foot = gridEl.querySelector('[data-nm-foot]');
            const html = itemsMarkup(more);
            if (foot) foot.insertAdjacentHTML('beforebegin', html); else gridEl.insertAdjacentHTML('beforeend', html);
            renderFoot();
        }

        function setPagination(next) {
            pagination = Object.assign({ total: 0, hasMore: false, loading: false }, next || {});
            renderFoot();
        }

        function setLoading(on) {
            loading = !!on;
            root.classList.toggle('is-loading', loading);
            root.setAttribute('aria-busy', loading ? 'true' : 'false');
            if (!tracks.length) renderGrid();
        }

        function setFilter(name) {
            filter = name === 'favorites' ? 'favorites' : 'all';
            root.querySelectorAll('[data-nm-filter]').forEach(btn => {
                const active = btn.dataset.nmFilter === filter;
                btn.classList.toggle('is-active', active);
                btn.setAttribute('aria-pressed', active ? 'true' : 'false');
            });
        }

        function setView(name) {
            view = name === 'list' ? 'list' : 'grid';
            root.querySelectorAll('[data-nm-view]').forEach(btn => {
                const active = btn.dataset.nmView === view;
                btn.classList.toggle('is-active', active);
                btn.setAttribute('aria-pressed', active ? 'true' : 'false');
            });
            renderGrid();
        }

        function setQuery(value) {
            query = String(value || '');
            if (searchInput.value !== query) searchInput.value = query;
        }

        function updateTrack(track) {
            const idx = tracks.findIndex(x => String(x.id) === String(track.id));
            if (idx < 0) return;
            tracks[idx] = Object.assign({}, tracks[idx], track);
            const el = elementById(track.id);
            if (!el) return;
            const hadFocus = el.contains(document.activeElement);
            el.outerHTML = view === 'list' ? rowMarkup(tracks[idx]) : cardMarkup(tracks[idx]);
            if (hadFocus) {
                const fresh = elementById(track.id);
                if (fresh) fresh.focus();
            }
        }

        function removeTracks(ids) {
            const remove = new Set((ids || []).map(String));
            if (!remove.size) return;
            const before = tracks.length;
            tracks = tracks.filter(x => !remove.has(String(x.id)));
            remove.forEach(id => selected.delete(id));
            pagination.total = Math.max(0, (pagination.total || before) - (before - tracks.length));
            if (!tracks.length) {
                renderGrid();
            } else {
                remove.forEach(id => { const el = elementById(id); if (el) el.remove(); });
                renderFoot();
            }
            syncSelectionUI();
        }

        function setPlaying(id, isPlaying) {
            currentId = id == null ? null : String(id);
            playing = !!isPlaying && currentId != null;
            gridEl.querySelectorAll('[data-track-id]').forEach(el => {
                const isCurrent = el.dataset.trackId === currentId;
                el.classList.toggle('is-current', isCurrent);
                el.classList.toggle('is-playing', isCurrent && playing);
                const btn = el.querySelector('.nm-card-play');
                if (btn) btn.setAttribute('aria-label', isCurrent && playing ? t('player_pause') : t('player_play'));
            });
        }

        function highlight(id) {
            const el = elementById(id);
            if (!el) return false;
            const reduced = !!(window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)').matches);
            try { el.scrollIntoView({ block: 'center', behavior: reduced ? 'auto' : 'smooth' }); } catch (_) { el.scrollIntoView(); }
            gridEl.querySelectorAll('.is-highlighted').forEach(x => x.classList.remove('is-highlighted'));
            el.classList.add('is-highlighted');
            clearTimeout(highlightTimer);
            highlightTimer = setTimeout(() => el.classList.remove('is-highlighted'), 1600);
            return true;
        }

        function dispose() {
            disposed = true;
            disconnectObserver();
            clearTimeout(searchTimer);
            clearTimeout(highlightTimer);
            Object.keys(handlers).forEach(key => { handlers[key] = []; });
            root.remove();
        }

        renderGrid();

        return {
            element: root,
            setTracks, appendTracks, setPagination, setLoading, setFilter, setView, setQuery,
            setSelectMode, isSelectMode: () => selectMode, selection: () => Array.from(selected), selectedTracks, targetsFor, isSelected,
            selectAll, clearSelection, toggleSelection,
            updateTrack, removeTracks, setPlaying, highlight,
            tracks: () => tracks.slice(),
            on, dispose
        };
    }

    window.NoisemakerLibrary = { create, formatDuration, formatDate };
})();
