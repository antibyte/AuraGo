/**
 * Virtual Desktop Gallery app: Photos/Videos media mounts as a photo library with
 * date-grouped grid, adjustable tiles, search, sorting, multi-select, details panel,
 * keyboard navigation and live refresh. Collaborators loaded first by the module
 * loader: window.GalleryLibrary (gallery-library.js), window.GalleryView
 * (gallery-view.js), window.GalleryMenus (gallery-menus.js) and
 * window.GalleryLightbox (gallery-lightbox.js). The shell passes its runtime
 * helpers in the render context; nothing here touches the desktop closure.
 */
(function () {
    'use strict';

    const instances = new Map();
    const LIBRARY_CHUNK = 400;
    const POLL_INTERVAL_MS = 45000;
    const SEARCH_DEBOUNCE_MS = 140;
    const SSE_DEBOUNCE_MS = 600;

    function lib() {
        const module = window.GalleryLibrary;
        if (!module) throw new Error('GalleryLibrary is not loaded');
        return module;
    }

    function render(host, windowId, context) {
        if (!host) return null;
        const existing = instances.get(windowId);
        if (existing) existing.dispose();

        const L = lib();
        const V = window.GalleryView;
        const M = window.GalleryMenus;
        if (!V || !M) throw new Error('Gallery view modules are not loaded');
        const { TILE_SIZES, TABS, SORTS, TAB_KIND, itemTime, itemKind, formatDuration, formatDateTime, dirOf, totalSize } = L;

        const ctx = context || {};
        const t = ctx.t || (key => key);
        const esc = ctx.esc || (value => String(value == null ? '' : value)
            .replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;'));
        const api = ctx.api || L.fetchJSON;
        const iconMarkup = ctx.iconMarkup || ((key, fallback) => `<span aria-hidden="true">${esc(fallback || '')}</span>`);
        const notify = typeof ctx.notify === 'function' ? ctx.notify : (() => {});
        const fmtBytes = typeof ctx.fmtBytes === 'function' ? ctx.fmtBytes : (n => `${n} B`);
        const previewURL = typeof ctx.mediaPreviewURL === 'function' ? ctx.mediaPreviewURL : (file => (file && file.web_path) || '');
        const downloadURL = typeof ctx.mediaDownloadURL === 'function' ? ctx.mediaDownloadURL : previewURL;
        const readonly = !!ctx.readonly;
        const pageSize = Math.max(20, Number(ctx.pageSize) || 80);
        const animations = typeof ctx.animationsEnabled === 'function' ? ctx.animationsEnabled() : true;
        const prefs = L.loadPrefs();
        const initialTab = TABS.includes(ctx.tab) ? ctx.tab : (TABS.includes(prefs.tab) ? prefs.tab : 'Photos');
        if (typeof ctx.wireContextMenuBoundary === 'function') ctx.wireContextMenuBoundary(host);

        const g = {
            windowId, host, tab: initialTab,
            sort: SORTS.includes(prefs.sort) ? prefs.sort : 'newest',
            group: prefs.group !== false,
            tileIndex: Number.isInteger(prefs.tileIndex) ? Math.min(TILE_SIZES.length - 1, Math.max(0, prefs.tileIndex)) : L.DEFAULT_TILE_INDEX,
            infoOpen: !!prefs.infoOpen,
            selectMode: false, query: '',
            items: [], visible: [], byPath: new Map(), groupCounts: new Map(), rendered: 0,
            selection: new Set(), anchor: null, focusPath: null,
            loading: false, complete: false, error: null, generation: 0, abort: null, pendingNew: 0,
            disposed: false, timers: {}, lightbox: null
        };
        instances.set(windowId, g);

        const countLabel = (key, count, vars) => L.countLabel(t, key, count, vars);
        const view = { t, esc, iconMarkup, readonly, countLabel, tabs: TABS, tileSizes: TILE_SIZES };
        const groupFor = L.createGrouper(t);

        host.classList.add('vd-gallery-host');
        host.innerHTML = V.shellHTML(view, g);

        const root = host.querySelector('[data-gallery-root]');
        const el = {
            root,
            body: root.querySelector('.vd-gallery-body'),
            scroll: root.querySelector('[data-gallery-scroll]'),
            grid: root.querySelector('[data-gallery-grid]'),
            footer: root.querySelector('[data-gallery-footer]'),
            more: root.querySelector('[data-gallery-more]'),
            sentinel: root.querySelector('[data-gallery-sentinel]'),
            search: root.querySelector('[data-gallery-search]'),
            searchClear: root.querySelector('[data-gallery-search-clear]'),
            status: root.querySelector('[data-gallery-status]'),
            selection: root.querySelector('[data-gallery-selection]'),
            selectionCount: root.querySelector('[data-gallery-selection-count]'),
            selectMode: root.querySelector('[data-gallery-select-mode]'),
            infoToggle: root.querySelector('[data-gallery-info-toggle]'),
            info: root.querySelector('[data-gallery-info-panel]'),
            zoom: root.querySelector('[data-gallery-zoom]'),
            progress: root.querySelector('[data-gallery-progress]'),
            progressLabel: root.querySelector('[data-gallery-progress-label]'),
            progressBar: root.querySelector('[data-gallery-progress-bar]'),
            pill: root.querySelector('[data-gallery-new]'),
            pillLabel: root.querySelector('[data-gallery-new-label]')
        };

        function persist() {
            L.savePrefs({ tab: g.tab, sort: g.sort, group: g.group, tileIndex: g.tileIndex, infoOpen: g.infoOpen });
        }

        function kindLabel(kind) {
            if (kind === 'video') return t('desktop.gallery_kind_video');
            if (kind === 'audio') return t('desktop.gallery_kind_audio');
            return t('desktop.gallery_kind_image');
        }

        // ── Library loading ─────────────────────────────────────────────

        function listURL(offset, limit) {
            return `/api/desktop/files?path=${encodeURIComponent(g.tab)}&recursive=true&limit=${limit}&offset=${offset}`;
        }

        function acceptEntry(entry) {
            if (!entry || entry.type === 'directory') return false;
            const kind = String(entry.media_kind || '').toLowerCase();
            return kind ? kind === TAB_KIND[g.tab] : true;
        }

        async function loadLibrary(options) {
            const opts = options || {};
            const generation = ++g.generation;
            if (g.abort) g.abort.abort();
            const controller = typeof AbortController === 'function' ? new AbortController() : null;
            g.abort = controller;
            g.loading = true;
            g.error = null;
            const silent = !!opts.silent;
            if (!silent) {
                g.items = [];
                g.byPath.clear();
                g.complete = false;
                renderSkeleton();
                updateStatus();
            }
            const collected = [];
            let offset = 0;
            let limit = pageSize;
            let firstPage = true;
            try {
                for (;;) {
                    const data = await api(listURL(offset, limit), controller ? { signal: controller.signal } : undefined);
                    if (g.disposed || generation !== g.generation) return;
                    const files = Array.isArray(data && data.files) ? data.files : [];
                    for (const entry of files) if (acceptEntry(entry)) collected.push(entry);
                    offset += files.length;
                    if (!silent) {
                        g.items = collected.slice();
                        g.byPath = new Map(g.items.map(item => [item.path, item]));
                        applyFilter(firstPage ? { keepScroll: false } : { keepScroll: true, incremental: true });
                        updateStatus();
                    }
                    firstPage = false;
                    if (!(data && data.has_more) || files.length === 0) break;
                    limit = LIBRARY_CHUNK;
                }
                if (g.disposed || generation !== g.generation) return;
                g.complete = true;
                if (silent) {
                    mergeLibrary(collected);
                } else {
                    if (!g.visible.length) renderEmpty();
                    updateStatus();
                }
                updateTabCount();
            } catch (err) {
                if (g.disposed || generation !== g.generation) return;
                if (err && err.name === 'AbortError') return;
                g.error = err;
                if (!silent) renderError(err);
                updateStatus();
            } finally {
                if (generation === g.generation) {
                    g.loading = false;
                    if (g.abort === controller) g.abort = null;
                    updateStatus();
                }
            }
        }

        function mergeLibrary(fresh) {
            const nextByPath = new Map(fresh.map(item => [item.path, item]));
            const diff = L.diffLibrary(g.byPath, fresh, nextByPath);
            if (!diff.changed) return;
            for (const path of Array.from(g.selection)) if (!nextByPath.has(path)) g.selection.delete(path);
            if (g.focusPath && !nextByPath.has(g.focusPath)) g.focusPath = null;
            g.items = fresh.slice();
            g.byPath = nextByPath;
            const scrolledAway = el.scroll.scrollTop > 160;
            applyFilter({ keepScroll: true });
            updateTabCount();
            syncSelectionUI();
            if (diff.added > 0 && g.sort === 'newest' && scrolledAway) {
                g.pendingNew += diff.added;
                el.pillLabel.textContent = t('desktop.gallery_new_items', { count: g.pendingNew });
                el.pill.hidden = false;
            }
            if (g.lightbox && typeof g.lightbox.setItems === 'function') g.lightbox.setItems(g.visible);
        }

        // ── Filtering, sorting and grouping ────────────────────────────

        function groupingActive() {
            return g.group && L.groupingAllowed(g.sort);
        }

        function applyFilter(options) {
            const opts = options || {};
            const tokens = L.queryTokens(g.query);
            const previousVisible = g.visible;
            const visible = g.items.filter(item => L.matchesQuery(item, tokens));
            visible.sort(L.comparator(g.sort));
            g.visible = visible;
            g.groupCounts = new Map();
            if (groupingActive()) {
                const now = new Date();
                for (const item of visible) {
                    const group = groupFor(item, now);
                    g.groupCounts.set(group.key, (g.groupCounts.get(group.key) || 0) + 1);
                }
            }
            // While the library streams in newest-first, already rendered tiles stay
            // valid; avoid re-rendering (and re-fading) them for every chunk.
            if (opts.incremental && g.rendered > 0 && g.rendered <= visible.length && sameRenderedPrefix(previousVisible, visible)) {
                refreshSectionCounts();
                updateMoreState();
                updateStatus();
                return;
            }
            const previousScroll = el.scroll.scrollTop;
            const previousRendered = g.rendered;
            renderGrid(opts.incremental ? Math.max(previousRendered, pageSize) : Math.max(pageSize, Math.min(previousRendered, visible.length)));
            el.scroll.scrollTop = opts.keepScroll ? previousScroll : 0;
            updateStatus();
        }

        function sameRenderedPrefix(previous, next) {
            if (!previous || previous.length < g.rendered) return false;
            for (let index = 0; index < g.rendered; index += 1) {
                if (previous[index].path !== next[index].path) return false;
            }
            return true;
        }

        function refreshSectionCounts() {
            el.grid.querySelectorAll('.vd-gallery-section[data-group]').forEach(section => {
                const count = g.groupCounts.get(section.dataset.group);
                const node = section.querySelector('.vd-gallery-section-count');
                if (node && count != null) node.textContent = countLabel('desktop.gallery_item_count', count);
            });
        }

        // ── Grid rendering ──────────────────────────────────────────────

        function tileHTML(item, index) {
            return V.tileHTML(view, item, index, itemKind(item, g.tab), g.selection.has(item.path), previewURL(item));
        }

        function renderSkeleton() {
            el.grid.classList.remove('is-empty');
            const count = Math.min(24, Math.max(8, Math.floor((el.scroll.clientWidth || 800) / TILE_SIZES[g.tileIndex]) * 3));
            el.grid.innerHTML = V.skeletonHTML(count);
            el.more.hidden = true;
            g.rendered = 0;
        }

        function renderError(err) {
            el.grid.classList.add('is-empty');
            el.grid.innerHTML = V.errorHTML(view, err);
            el.more.hidden = true;
            g.rendered = 0;
        }

        function renderEmpty() {
            el.grid.classList.add('is-empty');
            el.grid.innerHTML = V.emptyHTML(view, g.query, g.tab);
            el.more.hidden = true;
            g.rendered = 0;
        }

        function renderGrid(targetCount) {
            g.rendered = 0;
            el.grid.classList.remove('is-empty');
            el.grid.innerHTML = '';
            if (!g.visible.length) {
                if (g.loading && !g.items.length) { renderSkeleton(); return; }
                renderEmpty();
                return;
            }
            appendTiles(Math.max(1, targetCount || pageSize));
        }

        function appendTiles(count) {
            const end = Math.min(g.visible.length, g.rendered + count);
            if (end <= g.rendered) { updateMoreState(); return; }
            const grouping = groupingActive();
            const now = new Date();
            let section = el.grid.lastElementChild;
            let sectionKey = section ? section.dataset.group : null;
            let buffer = '';
            const flush = () => {
                if (!buffer || !section) return;
                section.querySelector('.vd-gallery-grid').insertAdjacentHTML('beforeend', buffer);
                buffer = '';
            };
            for (let index = g.rendered; index < end; index += 1) {
                const item = g.visible[index];
                const group = grouping ? groupFor(item, now) : { key: '__all__', label: '' };
                if (!section || sectionKey !== group.key) {
                    flush();
                    el.grid.insertAdjacentHTML('beforeend', V.sectionHTML(view, group, g.groupCounts.get(group.key) || 0));
                    section = el.grid.lastElementChild;
                    sectionKey = group.key;
                }
                buffer += tileHTML(item, index);
            }
            flush();
            g.rendered = end;
            wireNewTiles();
            updateMoreState();
        }

        function updateMoreState() {
            const remaining = g.visible.length - g.rendered;
            el.more.hidden = remaining <= 0;
            if (remaining > 0) el.more.textContent = `${t('desktop.gallery_load_more')} (${remaining})`;
        }

        function wireNewTiles() {
            el.grid.querySelectorAll('[data-gallery-item]:not([data-wired])').forEach(card => {
                card.dataset.wired = '1';
                const image = card.querySelector('img.vd-gallery-media');
                if (image) {
                    if (image.complete && image.naturalWidth > 0) card.classList.add('is-loaded');
                    image.addEventListener('load', () => card.classList.add('is-loaded'), { once: true });
                    image.addEventListener('error', () => markBroken(card), { once: true });
                }
                const video = card.querySelector('video.vd-gallery-media');
                if (video && videoObserver) videoObserver.observe(card);
                if (card.dataset.path === g.focusPath) card.tabIndex = 0;
            });
            if (!g.focusPath) {
                const first = el.grid.querySelector('[data-gallery-item]');
                if (first) first.tabIndex = 0;
            }
        }

        function markBroken(card) {
            card.classList.add('is-broken');
            const fallback = card.querySelector('.vd-gallery-thumb-fallback');
            if (fallback) fallback.hidden = false;
        }

        const videoObserver = typeof IntersectionObserver === 'function' ? new IntersectionObserver(entries => {
            for (const entry of entries) {
                if (!entry.isIntersecting) continue;
                const card = entry.target;
                videoObserver.unobserve(card);
                const video = card.querySelector('video.vd-gallery-media');
                if (!video || video.src) continue;
                video.preload = 'metadata';
                video.addEventListener('loadedmetadata', () => {
                    const badge = card.querySelector('[data-gallery-duration]');
                    if (badge && Number.isFinite(video.duration) && video.duration > 0) {
                        badge.textContent = formatDuration(video.duration);
                        badge.hidden = false;
                    }
                    const item = g.byPath.get(card.dataset.path);
                    if (item) {
                        item._duration = video.duration;
                        item._width = video.videoWidth;
                        item._height = video.videoHeight;
                    }
                    try { video.currentTime = Math.min(1, Math.max(0.1, video.duration * 0.1)); } catch (_) { /* seek unsupported */ }
                }, { once: true });
                video.addEventListener('loadeddata', () => card.classList.add('is-loaded'), { once: true });
                video.addEventListener('seeked', () => card.classList.add('is-loaded'), { once: true });
                video.addEventListener('error', () => markBroken(card), { once: true });
                video.src = video.dataset.src;
            }
        }, { root: el.scroll, rootMargin: '240px 0px' }) : null;

        const sentinelObserver = typeof IntersectionObserver === 'function' ? new IntersectionObserver(entries => {
            if (!entries.some(entry => entry.isIntersecting)) return;
            if (g.rendered < g.visible.length) appendTiles(pageSize);
        }, { root: el.scroll, rootMargin: '480px 0px' }) : null;
        if (sentinelObserver) sentinelObserver.observe(el.sentinel);

        // ── Status, counts and selection UI ─────────────────────────────

        function updateTabCount() {
            const node = root.querySelector(`[data-gallery-tab-count="${g.tab}"]`);
            if (node) node.textContent = g.complete && !g.error ? String(g.items.length) : '';
        }

        function updateStatus() {
            if (g.loading && !g.complete) {
                el.status.textContent = t('desktop.gallery_loading_library', { count: g.items.length });
                el.root.classList.add('is-loading');
            } else {
                el.root.classList.remove('is-loading');
                if (g.error) {
                    el.status.textContent = t('desktop.gallery_error_title');
                } else if (g.query) {
                    const results = countLabel('desktop.gallery_status_results', g.visible.length);
                    el.status.textContent = g.visible.length ? `${results} · ${fmtBytes(totalSize(g.visible))}` : results;
                } else {
                    el.status.textContent = countLabel('desktop.gallery_status_summary', g.items.length, { size: fmtBytes(totalSize(g.items)) });
                }
            }
            updateTabCount();
        }

        function syncSelectionUI() {
            const count = g.selection.size;
            el.selection.hidden = count === 0;
            el.selectionCount.textContent = t('desktop.gallery_selected_count', { count });
            el.root.classList.toggle('has-selection', count > 0);
            el.root.classList.toggle('is-select-mode', g.selectMode);
            el.selectMode.setAttribute('aria-pressed', String(g.selectMode));
            el.selectMode.classList.toggle('is-active', g.selectMode);
            el.selectMode.querySelector('.vd-gallery-tool-label').textContent = t(g.selectMode ? 'desktop.gallery_select_done' : 'desktop.gallery_select');
            refreshMenus();
        }

        function setCardSelected(card, selected) {
            card.classList.toggle('is-selected', selected);
            card.setAttribute('aria-selected', String(selected));
            const check = card.querySelector('[data-gallery-toggle]');
            if (check) check.setAttribute('aria-checked', String(selected));
        }

        function setTileSelected(path, selected) {
            const card = tileFor(path);
            if (card) setCardSelected(card, selected);
        }

        function tileFor(path) {
            if (!path) return null;
            for (const card of el.grid.querySelectorAll('[data-gallery-item]')) {
                if (card.dataset.path === path) return card;
            }
            return null;
        }

        function toggleSelection(path, forced) {
            const next = typeof forced === 'boolean' ? forced : !g.selection.has(path);
            if (next) g.selection.add(path); else g.selection.delete(path);
            setTileSelected(path, next);
            g.anchor = path;
            syncSelectionUI();
        }

        function selectRange(fromPath, toPath) {
            const from = g.visible.findIndex(item => item.path === fromPath);
            const to = g.visible.findIndex(item => item.path === toPath);
            if (from < 0 || to < 0) return toggleSelection(toPath, true);
            const [start, end] = from < to ? [from, to] : [to, from];
            for (let index = start; index <= end; index += 1) {
                const path = g.visible[index].path;
                g.selection.add(path);
                setTileSelected(path, true);
            }
            syncSelectionUI();
        }

        function selectAll() {
            for (const item of g.visible) g.selection.add(item.path);
            el.grid.querySelectorAll('[data-gallery-item]').forEach(card => setCardSelected(card, true));
            syncSelectionUI();
        }

        function clearSelection() {
            if (!g.selection.size) return;
            g.selection.clear();
            el.grid.querySelectorAll('[data-gallery-item].is-selected').forEach(card => setCardSelected(card, false));
            syncSelectionUI();
        }

        function setSelectMode(enabled) {
            g.selectMode = !!enabled;
            if (!g.selectMode) clearSelection();
            syncSelectionUI();
        }

        function selectedItems() {
            return g.visible.filter(item => g.selection.has(item.path));
        }

        function targetItems(path) {
            if (path && g.selection.has(path) && g.selection.size > 1) return selectedItems();
            if (path) {
                const item = g.byPath.get(path);
                return item ? [item] : [];
            }
            if (g.selection.size) return selectedItems();
            const focused = g.byPath.get(g.focusPath);
            return focused ? [focused] : [];
        }

        // ── Focus and keyboard navigation ───────────────────────────────

        function setFocusPath(path, options) {
            const opts = options || {};
            const previous = tileFor(g.focusPath);
            if (previous) previous.tabIndex = -1;
            g.focusPath = path;
            const card = tileFor(path);
            if (card) {
                card.tabIndex = 0;
                if (opts.focus !== false) card.focus({ preventScroll: true });
                if (opts.scroll !== false) card.scrollIntoView({ block: 'nearest', inline: 'nearest' });
            }
            if (g.infoOpen) renderInfo();
        }

        function ensureRendered(index) {
            while (g.rendered <= index && g.rendered < g.visible.length) appendTiles(pageSize);
        }

        function columnsPerRow() {
            const grid = el.grid.querySelector('.vd-gallery-grid');
            if (!grid) return 1;
            const columns = window.getComputedStyle(grid).gridTemplateColumns.split(' ').filter(Boolean).length;
            return Math.max(1, columns);
        }

        function focusIndex(index, extend) {
            if (!g.visible.length) return;
            const clamped = Math.max(0, Math.min(g.visible.length - 1, index));
            ensureRendered(clamped);
            const item = g.visible[clamped];
            if (extend) {
                if (!g.anchor) g.anchor = g.focusPath || item.path;
                selectRange(g.anchor, item.path);
            }
            setFocusPath(item.path);
        }

        function currentIndex() {
            const index = g.visible.findIndex(item => item.path === g.focusPath);
            return index < 0 ? 0 : index;
        }

        function onKeydown(event) {
            const target = event.target;
            if (target && (target.tagName === 'INPUT' || target.tagName === 'TEXTAREA')) return;
            const key = event.key;
            const mod = event.ctrlKey || event.metaKey;
            const index = currentIndex();
            const cols = columnsPerRow();
            const moves = { ArrowRight: 1, ArrowLeft: -1, ArrowDown: cols, ArrowUp: -cols, PageDown: cols * 3, PageUp: -cols * 3 };
            if (Object.prototype.hasOwnProperty.call(moves, key)) { event.preventDefault(); focusIndex(index + moves[key], event.shiftKey); return; }
            if (key === 'Home') { event.preventDefault(); focusIndex(0, event.shiftKey); return; }
            if (key === 'End') { event.preventDefault(); focusIndex(g.visible.length - 1, event.shiftKey); return; }
            if (key === 'Enter') { event.preventDefault(); if (g.visible[index]) openItem(g.visible[index].path); return; }
            if (key === ' ') { event.preventDefault(); if (g.visible[index]) toggleSelection(g.visible[index].path); return; }
            if (key === 'Delete') { event.preventDefault(); deleteItems(targetItems()); return; }
            if (key === 'F2') { event.preventDefault(); const item = g.byPath.get(g.focusPath); if (item) renameItem(item); return; }
            if (key === 'Escape') {
                if (g.selection.size || g.selectMode) { event.preventDefault(); setSelectMode(false); clearSelection(); return; }
                if (g.query) { event.preventDefault(); setQuery(''); return; }
                return;
            }
            if (mod && (key === 'a' || key === 'A')) { event.preventDefault(); selectAll(); return; }
            if (!mod && (key === '+' || key === '=')) { event.preventDefault(); setTileIndex(g.tileIndex + 1); return; }
            if (!mod && key === '-') { event.preventDefault(); setTileIndex(g.tileIndex - 1); return; }
            if (!mod && (key === 'i' || key === 'I')) { event.preventDefault(); setInfoOpen(!g.infoOpen); return; }
        }

        // ── Actions ────────────────────────────────────────────────────

        function openItem(path) {
            const index = g.visible.findIndex(item => item.path === path);
            if (index < 0) return;
            setFocusPath(path, { focus: false, scroll: false });
            const lightbox = window.GalleryLightbox;
            if (!lightbox || typeof lightbox.open !== 'function') {
                if (typeof ctx.openMediaPreview === 'function') ctx.openMediaPreview(g.visible[index]);
                return;
            }
            g.lightbox = lightbox.open({
                items: g.visible,
                index,
                context: {
                    t, esc, iconMarkup, fmtBytes,
                    mediaPreviewURL: previewURL,
                    mediaDownloadURL: downloadURL,
                    readonly,
                    animationsEnabled: animations,
                    kindLabel,
                    formatDateTime,
                    formatDuration
                },
                actions: {
                    rename: item => renameItem(item),
                    remove: item => deleteItems([item]),
                    download: item => downloadItem(item),
                    edit: item => editInPixel(item),
                    reveal: item => showInFiles(item)
                },
                onChange: item => { if (item) setFocusPath(item.path, { focus: false }); },
                onClose: () => {
                    g.lightbox = null;
                    const card = tileFor(g.focusPath);
                    if (card) card.focus({ preventScroll: true });
                }
            });
        }

        function downloadItem(item) {
            if (!item) return;
            if (typeof ctx.downloadMediaPath === 'function') ctx.downloadMediaPath(downloadURL(item), item.name);
            else window.open(downloadURL(item), '_blank', 'noopener');
        }

        async function downloadItems(items) {
            if (!items.length) return;
            if (items.length === 1) return downloadItem(items[0]);
            showProgress(t('desktop.gallery_download_progress', { done: 0, total: items.length }), 0);
            for (let index = 0; index < items.length; index += 1) {
                if (g.disposed) return;
                downloadItem(items[index]);
                showProgress(t('desktop.gallery_download_progress', { done: index + 1, total: items.length }), (index + 1) / items.length);
                await new Promise(resolve => setTimeout(resolve, 350));
            }
            hideProgress();
        }

        /** Renames `item` to `name` on the server and patches local state. Returns the updated item or null. */
        async function applyRename(item, name) {
            const updated = L.renamedItem(item, name);
            try {
                await api('/api/desktop/file', {
                    method: 'PATCH',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ old_path: item.path, new_path: updated.path })
                });
            } catch (err) {
                notify({ title: t('desktop.notification'), message: err && err.message ? err.message : t('desktop.load_failed') });
                return null;
            }
            if (g.selection.delete(item.path)) g.selection.add(updated.path);
            if (g.focusPath === item.path) g.focusPath = updated.path;
            if (g.anchor === item.path) g.anchor = updated.path;
            g.byPath.delete(item.path);
            g.byPath.set(updated.path, updated);
            g.items = g.items.map(entry => (entry.path === item.path ? updated : entry));
            applyFilter({ keepScroll: true });
            setFocusPath(updated.path, { focus: false });
            syncSelectionUI();
            scheduleReload(1500);
            if (typeof ctx.afterFileChange === 'function') ctx.afterFileChange();
            return updated;
        }

        async function renameItem(item) {
            if (!item || readonly || typeof ctx.promptDialog !== 'function') return null;
            const current = item.name || String(item.path).split('/').pop();
            const name = L.cleanFileName(await ctx.promptDialog(t('desktop.gallery_rename'), current), current);
            if (!name) return null;
            return applyRename(item, name);
        }

        async function deleteItems(items) {
            if (readonly || !items || !items.length) return false;
            const confirmNeeded = typeof ctx.settingBool === 'function' ? ctx.settingBool('files.confirm_delete') : true;
            if (confirmNeeded && typeof ctx.confirmDialog === 'function') {
                const title = items.length === 1
                    ? t('desktop.gallery_delete_one_title', { name: items[0].name || items[0].path })
                    : t('desktop.gallery_delete_many_title', { count: items.length });
                const confirmed = await ctx.confirmDialog(title, t('desktop.gallery_delete_msg'));
                if (!confirmed) return false;
            }
            const many = items.length > 1;
            if (many) showProgress(t('desktop.gallery_delete_progress', { done: 0, total: items.length }), 0);
            const removed = [];
            let failed = 0;
            let lastError = '';
            for (let index = 0; index < items.length; index += 1) {
                if (g.disposed) return false;
                const item = items[index];
                try {
                    await api('/api/desktop/file?path=' + encodeURIComponent(item.path), { method: 'DELETE' });
                    removed.push(item.path);
                } catch (err) {
                    failed += 1;
                    lastError = err && err.message ? String(err.message) : '';
                }
                if (many) showProgress(t('desktop.gallery_delete_progress', { done: index + 1, total: items.length }), (index + 1) / items.length);
            }
            if (removed.length) {
                if (typeof ctx.desktopSound === 'function') ctx.desktopSound('file.delete');
                const removedSet = new Set(removed);
                const focusedIndex = g.visible.findIndex(item => item.path === g.focusPath);
                for (const path of removed) { g.selection.delete(path); g.byPath.delete(path); }
                g.items = g.items.filter(item => !removedSet.has(item.path));
                applyFilter({ keepScroll: true });
                if (removedSet.has(g.focusPath)) {
                    g.focusPath = null;
                    if (g.visible.length) {
                        const next = g.visible[Math.min(Math.max(0, focusedIndex), g.visible.length - 1)];
                        setFocusPath(next.path, { focus: !g.lightbox });
                    } else if (g.infoOpen) {
                        renderInfo();
                    }
                }
                if (g.lightbox && typeof g.lightbox.setItems === 'function') g.lightbox.setItems(g.visible);
                syncSelectionUI();
                updateTabCount();
                if (many) notify({ title: t('desktop.app_gallery'), message: t('desktop.gallery_deleted', { count: removed.length }) });
                if (typeof ctx.afterFileChange === 'function') ctx.afterFileChange();
            }
            if (many) hideProgress();
            if (failed) {
                const summary = countLabel('desktop.gallery_delete_failed', failed);
                notify({ title: t('desktop.app_gallery'), message: lastError ? `${summary} ${lastError}` : summary });
            }
            return removed.length > 0 && failed === 0;
        }

        function editInPixel(item) {
            if (!item || typeof ctx.openApp !== 'function') return;
            ctx.openApp('pixel', { path: item.path });
        }

        function showInFiles(item) {
            if (!item || typeof ctx.openApp !== 'function') return;
            ctx.openApp('files', { path: dirOf(item.path) || g.tab });
        }

        async function copyLink(item) {
            if (!item) return;
            const url = new URL(downloadURL(item), window.location.origin).toString();
            try {
                await navigator.clipboard.writeText(url);
                notify({ title: t('desktop.app_gallery'), message: t('desktop.gallery_link_copied') });
            } catch (_) {
                if (typeof ctx.promptDialog === 'function') ctx.promptDialog(t('desktop.gallery_copy_link'), url);
            }
        }

        function showProgress(label, ratio) {
            el.progress.hidden = false;
            el.progressLabel.textContent = label;
            el.progressBar.style.width = `${Math.round(Math.max(0, Math.min(1, ratio)) * 100)}%`;
        }

        function hideProgress() {
            el.progress.hidden = true;
            el.progressBar.style.width = '0%';
        }

        // ── View state ─────────────────────────────────────────────────

        function setTab(tab) {
            if (!TABS.includes(tab) || tab === g.tab) return;
            g.tab = tab;
            g.selection.clear();
            g.focusPath = null;
            g.anchor = null;
            g.pendingNew = 0;
            el.pill.hidden = true;
            root.dataset.tab = tab;
            root.querySelectorAll('[data-gallery-tab]').forEach(button => {
                const active = button.dataset.galleryTab === tab;
                button.classList.toggle('is-active', active);
                button.setAttribute('aria-selected', String(active));
            });
            persist();
            syncSelectionUI();
            loadLibrary();
        }

        function setSort(sort) {
            if (!SORTS.includes(sort) || sort === g.sort) return;
            g.sort = sort;
            persist();
            applyFilter({ keepScroll: false });
            refreshMenus();
        }

        function setGroup(enabled) {
            g.group = !!enabled;
            persist();
            applyFilter({ keepScroll: false });
            refreshMenus();
        }

        function setTileIndex(index) {
            const clamped = Math.max(0, Math.min(TILE_SIZES.length - 1, index));
            if (clamped === g.tileIndex) return;
            g.tileIndex = clamped;
            root.style.setProperty('--vd-gallery-tile', `${TILE_SIZES[clamped]}px`);
            root.dataset.tileIndex = String(clamped);
            el.zoom.value = String(clamped);
            persist();
            refreshMenus();
        }

        function setInfoOpen(open) {
            g.infoOpen = !!open;
            el.info.hidden = !g.infoOpen;
            el.body.classList.toggle('has-info', g.infoOpen);
            el.infoToggle.setAttribute('aria-pressed', String(g.infoOpen));
            el.infoToggle.classList.toggle('is-active', g.infoOpen);
            persist();
            if (g.infoOpen) renderInfo();
            refreshMenus();
        }

        function setQuery(value) {
            const next = String(value || '');
            el.search.value = next;
            el.searchClear.hidden = !next;
            if (next === g.query) return;
            g.query = next;
            applyFilter({ keepScroll: false });
        }

        // ── Details panel ──────────────────────────────────────────────

        function renderInfo() {
            if (!g.infoOpen) return;
            if (g.selection.size > 1) {
                const items = selectedItems();
                el.info.innerHTML = V.infoMultiHTML(view, items.length, fmtBytes(totalSize(items)));
                return;
            }
            const item = g.byPath.get(g.focusPath) || null;
            if (!item) {
                el.info.innerHTML = V.infoEmptyHTML(view);
                return;
            }
            const kind = itemKind(item, g.tab);
            const rows = [];
            rows.push([t('desktop.gallery_field_type'), kindLabel(kind)]);
            rows.push([t('desktop.gallery_field_size'), fmtBytes(Number(item.size) || 0)]);
            if (item._width && item._height) rows.push([t('desktop.gallery_field_dimensions'), `${item._width} × ${item._height}`]);
            if (kind === 'video' && item._duration) rows.push([t('desktop.gallery_field_duration'), formatDuration(item._duration)]);
            const modified = formatDateTime(item.mod_time || item.modified);
            if (modified) rows.push([t('desktop.gallery_field_modified'), modified]);
            const created = formatDateTime(item.created);
            if (created && created !== modified) rows.push([t('desktop.gallery_field_created'), created]);
            rows.push([t('desktop.gallery_field_path'), dirOf(item.path) || g.tab]);
            el.info.innerHTML = V.infoItemHTML(view, item, kind, rows, previewURL(item));

            const image = el.info.querySelector('[data-gallery-info-image]');
            if (image) {
                const applyDimensions = () => {
                    if (!image.naturalWidth) return;
                    item._width = image.naturalWidth;
                    item._height = image.naturalHeight;
                    if (!el.info.querySelector('[data-gallery-dimensions]')) {
                        const list = el.info.querySelector('.vd-gallery-info-list');
                        const row = document.createElement('div');
                        row.dataset.galleryDimensions = '1';
                        row.innerHTML = `<dt>${esc(t('desktop.gallery_field_dimensions'))}</dt><dd>${item._width} × ${item._height}</dd>`;
                        if (list && list.children.length > 1) list.insertBefore(row, list.children[2] || null);
                    }
                };
                if (image.complete) applyDimensions();
                else image.addEventListener('load', applyDimensions, { once: true });
            }
            const nameInput = el.info.querySelector('[data-gallery-info-name]');
            if (nameInput && !readonly) {
                const commit = async () => {
                    const name = L.cleanFileName(nameInput.value, item.name);
                    if (!name) { nameInput.value = item.name || ''; return; }
                    const updated = await applyRename(item, name);
                    if (!updated) nameInput.value = item.name || '';
                };
                nameInput.addEventListener('keydown', event => {
                    if (event.key === 'Enter') { event.preventDefault(); nameInput.blur(); }
                    if (event.key === 'Escape') { event.preventDefault(); nameInput.value = item.name || ''; nameInput.blur(); }
                    event.stopPropagation();
                });
                nameInput.addEventListener('blur', commit);
            }
        }

        // ── Menus ──────────────────────────────────────────────────────

        const menuActions = {
            setTab, setSort, setGroup, setInfoOpen, setTileIndex,
            reload: () => loadLibrary(),
            openItem, downloadItems, editInPixel, showInFiles, copyLink, renameItem, deleteItems,
            selectAll, clearSelection, setSelectMode, toggleSelection, targetItems
        };

        function menuModel() {
            return { t, g, readonly, tileSizes: TILE_SIZES, sorts: SORTS, itemKind, actions: menuActions };
        }

        function refreshMenus() {
            if (typeof ctx.setWindowMenus !== 'function') return;
            ctx.setWindowMenus(windowId, M.windowMenus(menuModel()));
        }

        function showGalleryContextMenu(event) {
            if (typeof ctx.showContextMenu !== 'function') return;
            const card = event.target.closest('[data-gallery-item]');
            if (card) {
                const item = g.byPath.get(card.dataset.path);
                if (!item) return;
                event.preventDefault();
                event.stopPropagation();
                if (!g.selection.has(item.path)) setFocusPath(item.path, { scroll: false });
                ctx.showContextMenu(event.clientX, event.clientY, M.itemContextItems(menuModel(), item));
                return;
            }
            if (!event.target.closest('.vd-gallery-toolbar, .vd-gallery-statusbar, .vd-gallery-info')) {
                event.preventDefault();
                event.stopPropagation();
                ctx.showContextMenu(event.clientX, event.clientY, M.backgroundContextItems(menuModel()));
            }
        }

        // ── Live refresh ───────────────────────────────────────────────

        function scheduleReload(delay) {
            clearTimeout(g.timers.reload);
            g.timers.reload = setTimeout(() => {
                if (g.disposed || g.loading) return;
                loadLibrary({ silent: true });
            }, delay);
        }

        const onDesktopEvent = event => {
            if (!event || event.type !== 'desktop_changed') return;
            const payload = event.payload || {};
            const paths = [payload.path, payload.old_path, payload.new_path, payload.dest_path, payload.source_path];
            const prefix = `${g.tab}/`;
            if (!paths.some(path => typeof path === 'string' && (path === g.tab || path.startsWith(prefix)))) return;
            scheduleReload(SSE_DEBOUNCE_MS);
        };
        if (window.AuraSSE && typeof window.AuraSSE.on === 'function') window.AuraSSE.on('virtual_desktop_event', onDesktopEvent);

        g.timers.poll = setInterval(() => {
            if (g.disposed || g.loading || !g.complete) return;
            if (document.visibilityState !== 'visible') return;
            if (!host.isConnected || host.offsetParent === null) return;
            loadLibrary({ silent: true });
        }, POLL_INTERVAL_MS);

        const onVisibility = () => {
            if (document.visibilityState === 'visible' && g.complete && !g.loading) scheduleReload(800);
        };
        document.addEventListener('visibilitychange', onVisibility);

        // ── Event wiring ───────────────────────────────────────────────

        root.addEventListener('click', event => {
            const target = event.target;
            const tabButton = target.closest('[data-gallery-tab]');
            if (tabButton) { setTab(tabButton.dataset.galleryTab); return; }
            if (target.closest('[data-gallery-search-clear], [data-gallery-search-reset]')) { setQuery(''); el.search.focus(); return; }
            if (target.closest('[data-gallery-select-mode]')) { setSelectMode(!g.selectMode); return; }
            if (target.closest('[data-gallery-info-toggle]')) { setInfoOpen(!g.infoOpen); return; }
            if (target.closest('[data-gallery-reload], [data-gallery-retry]')) { loadLibrary(); return; }
            const sortButton = target.closest('[data-gallery-sort-menu]');
            if (sortButton) {
                if (typeof ctx.showContextMenu === 'function') {
                    const rect = sortButton.getBoundingClientRect();
                    ctx.showContextMenu(rect.left, rect.bottom + 4, M.sortPopoverItems(menuModel()));
                }
                return;
            }
            if (target.closest('[data-gallery-zoom-in]')) { setTileIndex(g.tileIndex + 1); return; }
            if (target.closest('[data-gallery-zoom-out]')) { setTileIndex(g.tileIndex - 1); return; }
            if (target.closest('[data-gallery-more]')) { appendTiles(pageSize); return; }
            if (target.closest('[data-gallery-show-new]')) { g.pendingNew = 0; el.pill.hidden = true; el.scroll.scrollTo({ top: 0, behavior: animations ? 'smooth' : 'auto' }); return; }
            if (target.closest('[data-gallery-bulk-download]')) { downloadItems(selectedItems()); return; }
            if (target.closest('[data-gallery-bulk-delete]')) { deleteItems(selectedItems()); return; }
            if (target.closest('[data-gallery-select-none]')) { setSelectMode(false); clearSelection(); return; }
            const focused = g.byPath.get(g.focusPath);
            if (target.closest('[data-gallery-info-download]')) { if (focused) downloadItem(focused); return; }
            if (target.closest('[data-gallery-info-edit]')) { if (focused) editInPixel(focused); return; }
            if (target.closest('[data-gallery-info-reveal]')) { if (focused) showInFiles(focused); return; }
            if (target.closest('[data-gallery-info-link]')) { if (focused) copyLink(focused); return; }
            if (target.closest('[data-gallery-info-delete]')) { if (focused) deleteItems([focused]); return; }

            const card = target.closest('[data-gallery-item]');
            if (!card) return;
            const item = g.byPath.get(card.dataset.path);
            if (!item) return;
            if (target.closest('[data-gallery-download]')) { event.stopPropagation(); downloadItem(item); return; }
            if (target.closest('[data-gallery-rename]')) { event.stopPropagation(); renameItem(item); return; }
            if (target.closest('[data-gallery-delete]')) { event.stopPropagation(); deleteItems(targetItems(item.path)); return; }
            if (target.closest('[data-gallery-toggle]')) {
                event.stopPropagation();
                if (event.shiftKey && g.anchor) selectRange(g.anchor, item.path);
                else toggleSelection(item.path);
                setFocusPath(item.path, { scroll: false });
                return;
            }
            if (event.shiftKey && (g.anchor || g.focusPath)) {
                event.preventDefault();
                selectRange(g.anchor || g.focusPath, item.path);
                setFocusPath(item.path, { scroll: false });
                return;
            }
            if (g.selectMode || event.ctrlKey || event.metaKey) {
                toggleSelection(item.path);
                setFocusPath(item.path, { scroll: false });
                return;
            }
            openItem(item.path);
        });

        root.addEventListener('contextmenu', showGalleryContextMenu);
        el.scroll.addEventListener('keydown', onKeydown);
        el.scroll.addEventListener('focusin', event => {
            const card = event.target.closest('[data-gallery-item]');
            if (card && card.dataset.path !== g.focusPath) setFocusPath(card.dataset.path, { focus: false, scroll: false });
        });
        el.scroll.addEventListener('focus', event => {
            if (event.target !== el.scroll) return;
            const card = tileFor(g.focusPath) || el.grid.querySelector('[data-gallery-item]');
            if (card) setFocusPath(card.dataset.path, { scroll: false });
        });
        el.scroll.addEventListener('scroll', () => {
            if (el.scroll.scrollTop < 40 && !el.pill.hidden) { el.pill.hidden = true; g.pendingNew = 0; }
        }, { passive: true });
        el.search.addEventListener('input', () => {
            clearTimeout(g.timers.search);
            const value = el.search.value;
            el.searchClear.hidden = !value;
            g.timers.search = setTimeout(() => setQuery(value), SEARCH_DEBOUNCE_MS);
        });
        el.search.addEventListener('keydown', event => {
            if (event.key === 'Escape') { event.preventDefault(); setQuery(''); el.scroll.focus(); }
            if (event.key === 'Enter' || event.key === 'ArrowDown') { event.preventDefault(); el.scroll.focus(); }
        });
        el.zoom.addEventListener('input', () => setTileIndex(Number(el.zoom.value)));
        el.scroll.addEventListener('wheel', event => {
            if (!(event.ctrlKey || event.metaKey)) return;
            event.preventDefault();
            setTileIndex(g.tileIndex + (event.deltaY < 0 ? 1 : -1));
        }, { passive: false });

        const resizeObserver = typeof ResizeObserver === 'function' ? new ResizeObserver(entries => {
            const width = entries[0] ? entries[0].contentRect.width : root.clientWidth;
            root.classList.toggle('is-narrow', width < 720);
            root.classList.toggle('is-tiny', width < 460);
        }) : null;
        if (resizeObserver) resizeObserver.observe(root);

        // ── Lifecycle ──────────────────────────────────────────────────

        g.dispose = function dispose() {
            if (g.disposed) return;
            g.disposed = true;
            g.generation += 1;
            if (g.abort) g.abort.abort();
            clearInterval(g.timers.poll);
            for (const key of ['reload', 'search']) clearTimeout(g.timers[key]);
            document.removeEventListener('visibilitychange', onVisibility);
            if (window.AuraSSE && typeof window.AuraSSE.off === 'function') window.AuraSSE.off('virtual_desktop_event', onDesktopEvent);
            if (videoObserver) videoObserver.disconnect();
            if (sentinelObserver) sentinelObserver.disconnect();
            if (resizeObserver) resizeObserver.disconnect();
            if (g.lightbox && typeof g.lightbox.close === 'function') g.lightbox.close();
            if (typeof ctx.clearWindowMenus === 'function') ctx.clearWindowMenus(windowId);
            instances.delete(windowId);
        };
        if (typeof ctx.registerWindowCleanup === 'function') ctx.registerWindowCleanup(windowId, g.dispose);

        syncSelectionUI();
        setInfoOpen(g.infoOpen);
        loadLibrary();
        return g;
    }

    function dispose(windowId) {
        const instance = instances.get(windowId);
        if (instance) instance.dispose();
    }

    window.GalleryApp = {
        render,
        dispose,
        instances,
        get TILE_SIZES() { return lib().TILE_SIZES; },
        formatDateTime: value => lib().formatDateTime(value),
        formatDuration: seconds => lib().formatDuration(seconds)
    };
})();
