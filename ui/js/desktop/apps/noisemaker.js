(function () {
    'use strict';

    // Noisemaker — Suno-style AI music studio for the virtual desktop.
    // Shell module: workbench layout (create pane | library pane, player bar),
    // preferences, API calls, window/context menus and wiring of the sub-modules
    // NoisemakerMenus, NoisemakerLibrary, NoisemakerPlayer and NoisemakerCreate.
    // Talks only to /api/desktop/noisemaker/*.
    // Exposes window.NoisemakerApp = { render, dispose }.

    const instances = new Map();
    const PREF_KEY = 'aurago.desktop.noisemaker.prefs';
    const NS = 'desktop.noisemaker_';
    const TRACKS_PAGE_SIZE = 60;
    const CREATE_MIN = 320;
    const CREATE_MAX = 560;
    const COMPACT_WIDTH = 860;
    const DEFAULT_PREFS = { mode: 'simple', style: '', instrumental: false, cover: true, view: 'grid', createWidth: 380, createCollapsed: false, shuffle: false, repeat: 'off', volume: 0.9, muted: false, visualizer: true };

    const SVG_REFRESH = '<svg viewBox="0 0 24 24" width="18" height="18" aria-hidden="true"><path fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" d="M21 12a9 9 0 1 1-3-6.7M21 4v5h-5"/></svg>';
    const SVG_PANEL = '<svg viewBox="0 0 24 24" width="18" height="18" aria-hidden="true"><rect x="3" y="4" width="18" height="16" rx="2" fill="none" stroke="currentColor" stroke-width="2"/><path d="M10 4v16" stroke="currentColor" stroke-width="2"/></svg>';

    function uiLang() {
        return (window.SYSTEM_LANG || document.documentElement.lang || 'en').toString();
    }

    function readPrefs() {
        let raw = {};
        try { raw = JSON.parse(localStorage.getItem(PREF_KEY) || '{}') || {}; } catch (_) { raw = {}; }
        const width = Number(raw.createWidth);
        const volume = Number(raw.volume);
        return {
            mode: raw.mode === 'custom' ? 'custom' : 'simple',
            style: String(raw.style || ''),
            instrumental: raw.instrumental === true,
            cover: raw.cover !== false,
            view: raw.view === 'list' ? 'list' : 'grid',
            createWidth: Number.isFinite(width) ? Math.min(CREATE_MAX, Math.max(CREATE_MIN, width)) : DEFAULT_PREFS.createWidth,
            createCollapsed: raw.createCollapsed === true,
            shuffle: raw.shuffle === true,
            repeat: ['off', 'all', 'one'].includes(raw.repeat) ? raw.repeat : 'off',
            volume: Number.isFinite(volume) ? Math.min(1, Math.max(0, volume)) : DEFAULT_PREFS.volume,
            muted: raw.muted === true,
            visualizer: raw.visualizer !== false
        };
    }

    function savePrefs(prefs) {
        try { localStorage.setItem(PREF_KEY, JSON.stringify(prefs)); } catch (_) {}
    }

    function makeT(ctx) {
        return (key, params, fallback) => {
            const full = String(key || '').startsWith('desktop.') ? key : NS + key;
            const value = ctx.t(full, params || {});
            return value && value !== full ? value : (fallback || full);
        };
    }

    function emptyGeneration() {
        return { active: false, startedAt: 0, result: null, error: '', coverFailed: false, lastParams: null };
    }

    async function request(S, path, options) {
        const controller = new AbortController();
        S.controllers.add(controller);
        const requestOptions = Object.assign({}, options || {}, { signal: controller.signal });
        if (requestOptions.body && typeof requestOptions.body !== 'string') {
            requestOptions.headers = Object.assign({ 'Content-Type': 'application/json' }, requestOptions.headers || {});
            requestOptions.body = JSON.stringify(requestOptions.body);
        }
        try {
            return await S.ctx.api(path, requestOptions);
        } finally {
            S.controllers.delete(controller);
        }
    }

    // ---------- markup ----------

    function shellMarkup(S) {
        const esc = S.ctx.esc;
        const t = S.t;
        return '<div class="noisemaker-app">' +
            '<header class="nm-header">' +
                '<div class="nm-brand"><span class="nm-brand-icon" aria-hidden="true">♪</span>' +
                    '<div><strong>' + esc(t('desktop.noisemaker_title')) + '</strong><span>' + esc(t('desktop.noisemaker_subtitle')) + '</span></div></div>' +
                '<div class="nm-segment nm-pane-switch" role="tablist" data-nm-pane-switch hidden>' +
                    '<button type="button" class="nm-segment-btn is-active" role="tab" data-nm-pane-btn="create" aria-selected="true">' + esc(t('desktop.noisemaker_tab_create')) + '</button>' +
                    '<button type="button" class="nm-segment-btn" role="tab" data-nm-pane-btn="library" aria-selected="false">' + esc(t('desktop.noisemaker_tab_library')) + ' <span class="nm-tab-count" data-nm-track-count hidden>0</span></button>' +
                '</div>' +
                '<div class="nm-header-chips">' +
                    '<span class="nm-chip nm-chip--busy" data-nm-busy hidden>' + esc(t('desktop.noisemaker_generating_badge')) + '</span>' +
                    '<span class="nm-chip" data-nm-provider hidden></span>' +
                    '<span class="nm-chip nm-chip--muted" data-nm-quota hidden></span>' +
                '</div>' +
                '<div class="nm-header-actions">' +
                    '<button type="button" class="nm-icon-btn" data-nm-refresh aria-label="' + esc(t('desktop.noisemaker_refresh')) + '" title="' + esc(t('desktop.noisemaker_refresh')) + '">' + SVG_REFRESH + '</button>' +
                    '<button type="button" class="nm-icon-btn nm-panel-toggle" data-nm-toggle-create aria-pressed="true" aria-label="' + esc(t('desktop.noisemaker_create_panel_hide')) + '" title="' + esc(t('desktop.noisemaker_create_panel_hide')) + '">' + SVG_PANEL + '</button>' +
                '</div>' +
            '</header>' +
            '<div class="nm-body"></div>' +
        '</div>';
    }

    function workbenchMarkup(S) {
        const esc = S.ctx.esc;
        const t = S.t;
        return '<div class="nm-workbench" data-nm-workbench>' +
            '<section class="nm-pane nm-pane-create" data-nm-pane="create" aria-label="' + esc(t('desktop.noisemaker_tab_create')) + '"></section>' +
            '<div class="nm-splitter" data-nm-splitter role="separator" aria-orientation="vertical" tabindex="0" aria-valuemin="' + CREATE_MIN + '" aria-valuemax="' + CREATE_MAX + '" aria-valuenow="' + S.prefs.createWidth + '" aria-label="' + esc(t('desktop.noisemaker_create_panel')) + '" title="' + esc(t('desktop.noisemaker_create_panel')) + '"></div>' +
            '<section class="nm-pane nm-pane-library" data-nm-pane="library" aria-label="' + esc(t('desktop.noisemaker_tab_library')) + '"></section>' +
        '</div>' +
        '<div class="nm-player-slot" data-nm-player-slot></div>';
    }

    function onboardingMarkup(S) {
        const esc = S.ctx.esc;
        const t = S.t;
        return '<div class="nm-onboarding"><div class="nm-onboarding-card">' +
            '<span class="nm-onboarding-icon" aria-hidden="true">♪</span>' +
            '<h2>' + esc(t('desktop.noisemaker_onboarding_title')) + '</h2>' +
            '<p>' + esc(t('desktop.noisemaker_onboarding_hint')) + '</p>' +
            '<div class="nm-onboarding-actions">' +
                '<button type="button" class="nm-btn nm-btn--primary" data-nm-open-settings>' + esc(t('desktop.noisemaker_onboarding_open_settings')) + '</button>' +
                '<button type="button" class="nm-btn" data-nm-recheck>' + esc(t('desktop.noisemaker_onboarding_recheck')) + '</button>' +
            '</div>' +
        '</div></div>';
    }

    // ---------- header ----------

    function qs(S, sel) { return S.root ? S.root.querySelector(sel) : null; }

    function syncHeader(S) {
        const caps = S.caps || {};
        const provider = qs(S, '[data-nm-provider]');
        if (provider) {
            provider.hidden = !caps.provider_type;
            provider.textContent = caps.model ? caps.provider_type + ' · ' + caps.model : (caps.provider_type || '');
        }
        const quota = qs(S, '[data-nm-quota]');
        if (quota) {
            const used = Number(caps.daily_used) || 0;
            const max = Number(caps.daily_max) || 0;
            quota.hidden = !caps.enabled;
            quota.textContent = max > 0 ? S.t('desktop.noisemaker_quota', { used, max }) : S.t('desktop.noisemaker_quota_unlimited', { used });
        }
        const busy = qs(S, '[data-nm-busy]');
        if (busy) busy.hidden = !S.generation.active;
        const count = qs(S, '[data-nm-track-count]');
        if (count) {
            const total = S.tracksTotal || S.tracks.length;
            count.hidden = total === 0;
            count.textContent = String(total);
        }
    }

    function wireHeader(S) {
        S.root.addEventListener('click', event => {
            if (event.target.closest('[data-nm-refresh]')) { refreshTracks(S); return; }
            if (event.target.closest('[data-nm-toggle-create]')) { setCreateCollapsed(S, !S.prefs.createCollapsed); return; }
            const paneBtn = event.target.closest('[data-nm-pane-btn]');
            if (paneBtn) { S.paneChosen = true; setActivePane(S, paneBtn.dataset.nmPaneBtn); }
        });
    }

    // ---------- state / capabilities ----------

    async function loadState(S) {
        try {
            const data = await request(S, '/api/desktop/noisemaker/state');
            if (S.disposed) return;
            S.caps = data || {};
        } catch (err) {
            if (S.disposed) return;
            S.caps = { enabled: false, error: (err && err.message) || '' };
        }
        renderApp(S);
        scheduleLocalStatus(S);
    }

    function scheduleLocalStatus(S) {
        clearTimeout(S.statusTimer);
        if (S.disposed || !S.caps || !S.caps.supports_controls) return;
        S.statusTimer = setTimeout(async () => {
            try {
                const caps = await request(S, '/api/desktop/noisemaker/state');
                if (S.disposed) return;
                const previous = S.caps || {};
                const changed = caps.enabled !== previous.enabled || caps.supports_controls !== previous.supports_controls || caps.provider_type !== previous.provider_type;
                S.caps = caps || {};
                if (changed) renderApp(S);
                else if (S.create) S.create.setCaps(S.caps, { light: true });
                syncHeader(S);
            } catch (_) {}
            scheduleLocalStatus(S);
        }, 3000);
    }

    // ---------- render ----------

    function renderApp(S) {
        if (S.disposed || !S.root) return;
        const caps = S.caps || {};
        teardownModules(S);
        const body = qs(S, '.nm-body');
        if (!caps.enabled) {
            body.innerHTML = onboardingMarkup(S);
            S.root.classList.add('is-onboarding');
            body.querySelector('[data-nm-open-settings]').addEventListener('click', () => window.open('/config', '_blank', 'noopener'));
            body.querySelector('[data-nm-recheck]').addEventListener('click', () => loadState(S));
            const toggle = qs(S, '[data-nm-toggle-create]');
            if (toggle) toggle.hidden = true;
            const paneSwitch = qs(S, '[data-nm-pane-switch]');
            if (paneSwitch) paneSwitch.hidden = true;
            syncHeader(S);
            return;
        }
        S.root.classList.remove('is-onboarding');
        body.innerHTML = workbenchMarkup(S);
        mountModules(S);
        syncHeader(S);
    }

    function mountModules(S) {
        const ctx = S.ctx;
        const lib = window.NoisemakerLibrary;
        const base = { esc: ctx.esc, t: S.t, lang: S.lang, readonly: S.readonly };

        S.create = window.NoisemakerCreate.create(Object.assign({}, base, {
            request: (path, options) => request(S, path, options),
            notify: ctx.notify,
            formatDuration: lib.formatDuration,
            windowId: S.windowId,
            mode: S.prefs.mode,
            form: { style: S.prefs.style, instrumental: S.prefs.instrumental, cover: S.prefs.cover }
        }));
        S.library = window.NoisemakerLibrary.create(base);
        S.player = window.NoisemakerPlayer.create(Object.assign({}, base, {
            formatDuration: lib.formatDuration,
            formatDate: lib.formatDate,
            hasMore: () => S.tracks.length < S.tracksTotal,
            isVisible: () => !!(S.root && S.root.isConnected && S.root.offsetParent !== null),
            prefs: { shuffle: S.prefs.shuffle, repeat: S.prefs.repeat, volume: S.prefs.volume, muted: S.prefs.muted, visualizer: S.prefs.visualizer }
        }));

        qs(S, '[data-nm-pane="create"]').appendChild(S.create.element);
        const libraryPane = qs(S, '[data-nm-pane="library"]');
        libraryPane.appendChild(S.library.element);
        libraryPane.appendChild(S.player.nowPlayingElement);
        qs(S, '[data-nm-player-slot]').appendChild(S.player.barElement);

        S.create.setCaps(S.caps);
        S.create.setGeneration(S.generation);
        S.library.setView(S.prefs.view);
        S.library.setFilter(S.filter);
        S.library.setQuery(S.query);

        wireCreate(S);
        wireLibrary(S);
        wirePlayer(S);
        wireSplitter(S);
        wireResize(S);
        applyCreateLayout(S);
        setActivePane(S, S.activePane);
        refreshMenus(S);
        refreshTracks(S);
    }

    function teardownModules(S) {
        S.loadSeq++;
        S.tracksLoading = false;
        if (S.resizeObserver) { try { S.resizeObserver.disconnect(); } catch (_) {} S.resizeObserver = null; }
        for (const key of ['create', 'library', 'player']) {
            if (S[key]) { try { S[key].dispose(); } catch (_) {} S[key] = null; }
        }
        S.loadedOnce = false;
        S.nowPlayingOpen = false;
        if (S.root) S.root.classList.remove('is-now-playing');
        if (typeof S.ctx.clearWindowMenus === 'function') { try { S.ctx.clearWindowMenus(S.windowId); } catch (_) {} }
    }

    // ---------- module wiring ----------

    function wireCreate(S) {
        const C = S.create;
        C.on('generate', params => generate(S, params));
        C.on('change', form => {
            S.prefs.style = String(form.style || '');
            S.prefs.instrumental = !!form.instrumental;
            S.prefs.cover = form.cover !== false;
            savePrefs(S.prefs);
        });
        C.on('mode', mode => { S.prefs.mode = mode; savePrefs(S.prefs); });
        C.on('play-result', result => playResult(S, result));
        C.on('show-in-library', result => showInLibrary(S, result));
        C.on('new-song', () => newSong(S));
    }

    function wireLibrary(S) {
        const L = S.library;
        L.on('play', (track, list) => S.player.play(track, list && list.length ? list : S.tracks));
        L.on('enqueue', tracks => enqueueTracks(S, tracks));
        L.on('favorite', (track, value) => toggleFavorite(S, track, value));
        L.on('delete', tracks => deleteTracks(S, tracks));
        L.on('template', track => useTemplate(S, track));
        L.on('download', tracks => downloadTracks(S, tracks));
        L.on('contextmenu', payload => showLibraryContextMenu(S, payload));
        L.on('create', () => focusCreate(S));
        L.on('loadmore', () => loadMoreTracks(S));
        L.on('search', query => { S.query = query; refreshTracks(S); });
        L.on('filter', name => { S.filter = name; refreshTracks(S); refreshMenus(S); });
        L.on('view', name => { S.prefs.view = name; savePrefs(S.prefs); refreshMenus(S); });
        L.on('selection', () => refreshMenus(S));
    }

    function wirePlayer(S) {
        const P = S.player;
        P.on('state', payload => {
            S.library.setPlaying(payload && payload.track ? payload.track.id : null, !!(payload && payload.playing));
            refreshMenus(S);
        });
        P.on('change', prefs => { Object.assign(S.prefs, prefs || {}); savePrefs(S.prefs); refreshMenus(S); });
        P.on('favorite', (track, value) => toggleFavorite(S, track, value));
        P.on('delete', track => deleteTracks(S, [track]));
        P.on('template', track => useTemplate(S, track));
        P.on('download', track => downloadTracks(S, [track]));
        P.on('expand', open => {
            S.nowPlayingOpen = !!open;
            S.root.classList.toggle('is-now-playing', S.nowPlayingOpen);
            if (S.nowPlayingOpen && S.compact) { S.paneChosen = true; setActivePane(S, 'library'); }
            refreshMenus(S);
        });
        P.on('needmore', () => {
            loadMoreTracks(S).then(added => { if (!S.disposed && added.length && S.player) S.player.enqueue(added); });
        });
        P.on('error', () => S.ctx.notify(S.t('desktop.noisemaker_playback_failed')));
        P.on('visualizer-unavailable', () => { S.visualizerAvailable = false; refreshMenus(S); });
    }

    // ---------- tracks ----------

    async function fetchTrackPage(S, offset) {
        const params = new URLSearchParams({ limit: String(TRACKS_PAGE_SIZE), offset: String(offset) });
        if (S.query) params.set('q', S.query);
        if (S.filter === 'favorites') params.set('favorites', '1');
        return await request(S, '/api/desktop/noisemaker/tracks?' + params.toString());
    }

    function syncPagination(S, loading) {
        if (!S.library) return;
        S.library.setPagination({ total: S.tracksTotal, hasMore: S.tracks.length < S.tracksTotal, loading: !!loading });
    }

    async function refreshTracks(S) {
        if (!S.library || S.disposed) return;
        const seq = ++S.loadSeq;
        S.tracksLoading = true;
        S.library.setLoading(true);
        try {
            const data = await fetchTrackPage(S, 0);
            if (S.disposed || seq !== S.loadSeq || !S.library) return;
            S.tracks = Array.isArray(data.items) ? data.items : [];
            S.tracksTotal = Number(data.total) || 0;
            if (S.caps && typeof data.daily_used === 'number') S.caps.daily_used = data.daily_used;
            S.library.setTracks(S.tracks);
            syncPagination(S, false);
            const current = S.player ? S.player.current() : null;
            if (current) S.library.setPlaying(current.id, S.player.isPlaying());
        } catch (_) {
            if (S.disposed || seq !== S.loadSeq || !S.library) return;
        }
        if (!S.library) return;
        S.tracksLoading = false;
        S.library.setLoading(false);
        if (!S.loadedOnce) {
            S.loadedOnce = true;
            if (S.compact && !S.paneChosen) setActivePane(S, S.tracks.length ? 'library' : 'create');
        }
        syncHeader(S);
        refreshMenus(S);
    }

    async function loadMoreTracks(S) {
        if (!S.library || S.tracksLoading || S.disposed) return [];
        if (S.tracksTotal > 0 && S.tracks.length >= S.tracksTotal) return [];
        const seq = S.loadSeq;
        S.tracksLoading = true;
        syncPagination(S, true);
        let additions = [];
        try {
            const data = await fetchTrackPage(S, S.tracks.length);
            if (S.disposed || seq !== S.loadSeq || !S.library) return [];
            additions = Array.isArray(data.items) ? data.items : [];
            S.tracks = S.tracks.concat(additions);
            S.tracksTotal = Number(data.total) || S.tracksTotal;
            if (S.caps && typeof data.daily_used === 'number') S.caps.daily_used = data.daily_used;
            S.library.appendTracks(additions);
        } catch (_) {
            if (S.disposed || seq !== S.loadSeq || !S.library) return [];
        } finally {
            if (!S.disposed && seq === S.loadSeq && S.library) {
                S.tracksLoading = false;
                syncPagination(S, false);
                syncHeader(S);
            }
        }
        return additions;
    }

    function applyTrackUpdate(S, track) {
        const idx = S.tracks.findIndex(x => String(x.id) === String(track.id));
        if (idx >= 0) S.tracks[idx] = Object.assign({}, S.tracks[idx], track);
        if (S.library) S.library.updateTrack(track);
        if (S.player) S.player.updateTrack(track);
    }

    function removeTracksLocally(S, ids) {
        const set = new Set(ids.map(String));
        S.tracks = S.tracks.filter(x => !set.has(String(x.id)));
        S.tracksTotal = Math.max(0, S.tracksTotal - ids.length);
        if (S.library) S.library.removeTracks(ids);
        syncPagination(S, false);
    }

    // ---------- flows ----------

    async function generate(S, params) {
        if (S.generation.active || !S.create) return;
        S.generation = { active: true, startedAt: Date.now(), result: null, error: '', coverFailed: false, lastParams: params };
        S.create.setGeneration(S.generation);
        syncHeader(S);
        try {
            const data = await request(S, '/api/desktop/noisemaker/generate', { method: 'POST', body: params });
            if (S.disposed) return;
            S.generation = Object.assign({}, S.generation, { active: false, result: data, coverFailed: !!data.cover_error });
            if (S.caps && typeof data.daily_used === 'number') S.caps.daily_used = data.daily_used;
            S.ctx.notify(S.t('desktop.noisemaker_track_created_toast', { title: data.title || '' }));
            S.filter = 'all';
            S.query = '';
            if (S.library) { S.library.setFilter('all'); S.library.setQuery(''); }
            await refreshTracks(S);
            if (S.disposed) return;
            const id = data.track ? data.track.id : data.media_id;
            if (id && S.library) S.library.highlight(id);
        } catch (err) {
            if (S.disposed) return;
            let message = (err && err.message) || S.t('desktop.noisemaker_error_unknown');
            if (err && err.body && err.body.code === 'lyrics_required') message = S.t('desktop.noisemaker_lyrics_required');
            S.generation = Object.assign({}, S.generation, { active: false, error: message });
        } finally {
            if (!S.disposed) {
                S.generation.active = false;
                if (S.create) S.create.setGeneration(S.generation);
                syncHeader(S);
                refreshMenus(S);
            }
        }
    }

    function resultTrack(S, result) {
        if (!result) return null;
        if (result.track && result.track.web_path) {
            const inLibrary = S.tracks.find(x => String(x.id) === String(result.track.id));
            return inLibrary || result.track;
        }
        if (!result.web_path) return null;
        const params = S.generation.lastParams || {};
        return {
            id: 'result-' + (result.media_id || Date.now()),
            title: result.title || S.t('desktop.noisemaker_result_untitled'),
            web_path: result.web_path,
            cover_url: result.cover_url || '',
            duration_ms: result.duration_ms || 0,
            provider: result.provider || '',
            lyrics: result.lyrics || '',
            style: params.style || '',
            prompt: params.prompt || '',
            instrumental: !!params.instrumental,
            favorite: false,
            tags: []
        };
    }

    function playResult(S, result) {
        const track = resultTrack(S, result);
        if (!track || !S.player) return;
        const inLibrary = S.tracks.some(x => String(x.id) === String(track.id));
        S.player.play(track, inLibrary ? S.tracks : [track]);
    }

    function showInLibrary(S, result) {
        if (S.compact) { S.paneChosen = true; setActivePane(S, 'library'); }
        const id = result && result.track ? result.track.id : (result ? result.media_id : null);
        if (!id || !S.library) return;
        if (!S.library.highlight(id)) refreshTracks(S).then(() => { if (!S.disposed && S.library) S.library.highlight(id); });
    }

    function newSong(S) {
        S.generation = emptyGeneration();
        if (S.create) S.create.setGeneration(S.generation);
        syncHeader(S);
        focusCreate(S);
    }

    function enqueueTracks(S, tracks) {
        if (!S.player) return;
        const count = S.player.enqueue(tracks || []);
        if (count > 0) S.ctx.notify(count === 1 ? S.t('desktop.noisemaker_queue_added_one') : S.t('desktop.noisemaker_queue_added', { count }));
        refreshMenus(S);
    }

    async function toggleFavorite(S, track, value) {
        if (S.readonly || !track) return;
        const next = !!value;
        applyTrackUpdate(S, Object.assign({}, track, { favorite: next }));
        try {
            const data = await request(S, '/api/desktop/noisemaker/tracks/' + encodeURIComponent(track.id), { method: 'PATCH', body: { favorite: next } });
            if (S.disposed) return;
            if (data && data.track) applyTrackUpdate(S, data.track);
            if (S.filter === 'favorites' && !next) removeTracksLocally(S, [track.id]);
        } catch (err) {
            if (S.disposed) return;
            applyTrackUpdate(S, Object.assign({}, track, { favorite: !next }));
            S.ctx.notify((err && err.message) || S.t('desktop.noisemaker_favorite_failed'));
        }
        syncHeader(S);
        refreshMenus(S);
    }

    async function deleteTracks(S, tracks) {
        const list = (tracks || []).filter(Boolean);
        if (!list.length || S.readonly) return;
        const single = list.length === 1;
        const confirmed = await S.ctx.confirmDialog(
            single ? S.t('desktop.noisemaker_track_delete_title') : S.t('desktop.noisemaker_tracks_delete_title', { count: list.length }),
            single ? S.t('desktop.noisemaker_track_delete_confirm', { title: list[0].title || '' }) : S.t('desktop.noisemaker_tracks_delete_confirm', { count: list.length })
        );
        if (!confirmed || S.disposed) return;
        const removed = [];
        let failed = 0;
        for (const track of list) {
            try {
                await request(S, '/api/desktop/noisemaker/tracks/' + encodeURIComponent(track.id), { method: 'DELETE' });
                removed.push(track.id);
            } catch (_) {
                failed += 1;
            }
            if (S.disposed) return;
        }
        if (removed.length) {
            removeTracksLocally(S, removed);
            if (S.player) S.player.removeTracks(removed);
            const resultId = S.generation.result && S.generation.result.track ? String(S.generation.result.track.id) : '';
            if (resultId && removed.some(id => String(id) === resultId)) {
                S.generation = emptyGeneration();
                if (S.create) S.create.setGeneration(S.generation);
            }
        }
        if (!failed) S.ctx.notify(single ? S.t('desktop.noisemaker_track_deleted') : S.t('desktop.noisemaker_tracks_deleted', { count: removed.length }));
        else if (removed.length) S.ctx.notify(S.t('desktop.noisemaker_tracks_deleted_partial', { done: removed.length, total: list.length }));
        else S.ctx.notify(S.t('desktop.noisemaker_track_delete_failed'));
        syncHeader(S);
        refreshMenus(S);
    }

    function downloadTracks(S, tracks) {
        (tracks || []).filter(track => track && track.web_path).forEach((track, index) => {
            setTimeout(() => {
                if (S.disposed) return;
                const link = document.createElement('a');
                link.href = track.web_path;
                link.download = String(track.web_path).split('/').pop() || 'song';
                link.rel = 'noopener';
                link.style.display = 'none';
                document.body.appendChild(link);
                link.click();
                link.remove();
            }, index * 350);
        });
    }

    function useTemplate(S, track) {
        if (!track || !S.create) return;
        let idea = String(track.prompt || '');
        let style = String(track.style || '');
        const sep = idea.indexOf(' — ');
        if (sep > 0) {
            if (!style) style = idea.slice(0, sep);
            idea = idea.startsWith(style + ' — ') ? idea.slice(style.length + 3) : idea.slice(sep + 3);
        }
        const lyrics = String(track.lyrics || '');
        if (lyrics) S.create.setMode('custom');
        S.create.setForm({ idea, style, lyrics, title: '', instrumental: !!track.instrumental });
        S.prefs.style = style;
        S.prefs.instrumental = !!track.instrumental;
        savePrefs(S.prefs);
        focusCreate(S);
    }

    function focusCreate(S) {
        if (S.compact) { S.paneChosen = true; setActivePane(S, 'create'); }
        else if (S.prefs.createCollapsed) setCreateCollapsed(S, false);
        if (S.create) S.create.focusIdea();
    }

    function openDetails(S, track) {
        if (!S.player || !track) return;
        const current = S.player.current();
        if (!current || String(current.id) !== String(track.id)) S.player.play(track, S.tracks);
        S.player.setNowPlayingOpen(true);
    }

    // ---------- menus ----------

    function menuTargets(S) {
        const selection = S.library ? S.library.selectedTracks() : [];
        if (selection.length) return selection;
        const current = S.player ? S.player.current() : null;
        return current ? [current] : [];
    }

    function menuModel(S) {
        const selection = S.library ? S.library.selectedTracks() : [];
        const current = S.player ? S.player.current() : null;
        const targets = menuTargets(S);
        return {
            t: S.t,
            tFull: S.tFull,
            readonly: S.readonly,
            actions: menuActions(S),
            s: {
                view: S.prefs.view,
                filter: S.filter,
                selectMode: !!(S.library && S.library.isSelectMode()),
                selectionCount: selection.length,
                hasTracks: S.tracks.length > 0,
                hasTarget: targets.length > 0,
                singleTarget: targets.length === 1,
                targetFavorite: targets.length === 1 && !!targets[0].favorite,
                createCollapsed: S.prefs.createCollapsed,
                nowPlayingOpen: S.nowPlayingOpen,
                visualizer: S.prefs.visualizer,
                visualizerAvailable: S.visualizerAvailable,
                playing: !!(S.player && S.player.isPlaying()),
                hasCurrent: !!current,
                shuffle: S.prefs.shuffle,
                repeat: S.prefs.repeat,
                queueLength: S.player ? S.player.queue().length : 0,
                compact: S.compact
            }
        };
    }

    function menuActions(S) {
        return {
            newSong: () => newSong(S),
            downloadTargets: () => downloadTracks(S, menuTargets(S)),
            templateTarget: () => { const list = menuTargets(S); if (list.length === 1) useTemplate(S, list[0]); },
            deleteTargets: () => deleteTracks(S, menuTargets(S)),
            selectAll: () => { if (S.library) S.library.selectAll(); },
            clearSelection: () => { if (S.library) S.library.clearSelection(); },
            setSelectMode: on => { if (S.library) S.library.setSelectMode(!!on); refreshMenus(S); },
            toggleFavoriteTarget: () => { const list = menuTargets(S); if (list.length === 1) toggleFavorite(S, list[0], !list[0].favorite); },
            refresh: () => refreshTracks(S),
            setView: name => { S.prefs.view = name; savePrefs(S.prefs); if (S.library) S.library.setView(name); refreshMenus(S); },
            setFilter: name => { S.filter = name; if (S.library) S.library.setFilter(name); refreshTracks(S); },
            setCreateCollapsed: collapsed => setCreateCollapsed(S, collapsed),
            setNowPlayingOpen: open => { if (S.player) S.player.setNowPlayingOpen(!!open); },
            setVisualizer: on => { if (S.player) S.player.setVisualizer(!!on); },
            togglePlay: () => {
                if (!S.player) return;
                if (S.player.current() || S.player.queue().length) S.player.toggle();
                else if (S.tracks.length) S.player.play(S.tracks[0], S.tracks);
            },
            next: () => { if (S.player) S.player.next(); },
            prev: () => { if (S.player) S.player.prev(); },
            setShuffle: on => { if (S.player) S.player.setShuffle(!!on); },
            setRepeat: mode => { if (S.player) S.player.setRepeat(mode); },
            clearQueue: () => { if (S.player) S.player.clearQueue(); refreshMenus(S); },
            targetTracks: track => (S.library ? S.library.targetsFor(track) : [track]),
            isSelected: track => !!(S.library && S.library.isSelected(track)),
            playTrack: track => { if (S.player) S.player.play(track, S.tracks); },
            playTracks: list => { if (S.player && list.length) S.player.play(list[0], list); },
            enqueueTracks: list => enqueueTracks(S, list),
            toggleFavorite: track => toggleFavorite(S, track, !track.favorite),
            useTemplate: track => useTemplate(S, track),
            downloadTracks: list => downloadTracks(S, list),
            openDetails: track => openDetails(S, track),
            toggleSelection: track => { if (S.library) S.library.toggleSelection(track); },
            deleteTracks: list => deleteTracks(S, list)
        };
    }

    function refreshMenus(S) {
        if (S.disposed || !S.root || !S.library || !S.player || !window.NoisemakerMenus) return;
        if (typeof S.ctx.setWindowMenus !== 'function') return;
        S.ctx.setWindowMenus(S.windowId, window.NoisemakerMenus.windowMenus(menuModel(S)));
    }

    function showLibraryContextMenu(S, payload) {
        if (!payload || typeof S.ctx.showContextMenu !== 'function' || !window.NoisemakerMenus) return;
        const model = menuModel(S);
        const items = payload.track
            ? window.NoisemakerMenus.trackContextItems(model, payload.track)
            : window.NoisemakerMenus.libraryContextItems(model);
        S.ctx.showContextMenu(payload.x, payload.y, items);
    }

    // ---------- layout ----------

    function applyCreateLayout(S) {
        const workbench = qs(S, '[data-nm-workbench]');
        if (!workbench) return;
        workbench.style.setProperty('--nm-create-width', S.prefs.createWidth + 'px');
        const collapsed = S.prefs.createCollapsed && !S.compact;
        S.root.classList.toggle('is-create-collapsed', collapsed);
        const splitter = qs(S, '[data-nm-splitter]');
        if (splitter) splitter.setAttribute('aria-valuenow', String(S.prefs.createWidth));
        const toggle = qs(S, '[data-nm-toggle-create]');
        if (toggle) {
            const label = S.t(collapsed ? 'desktop.noisemaker_create_panel_show' : 'desktop.noisemaker_create_panel_hide');
            toggle.hidden = S.compact;
            toggle.setAttribute('aria-pressed', collapsed ? 'false' : 'true');
            toggle.setAttribute('aria-label', label);
            toggle.title = label;
        }
    }

    function setCreateCollapsed(S, collapsed) {
        S.prefs.createCollapsed = !!collapsed;
        savePrefs(S.prefs);
        applyCreateLayout(S);
        refreshMenus(S);
    }

    function setCreateWidth(S, width) {
        S.prefs.createWidth = Math.round(Math.min(CREATE_MAX, Math.max(CREATE_MIN, width)));
        applyCreateLayout(S);
    }

    function wireSplitter(S) {
        const splitter = qs(S, '[data-nm-splitter]');
        if (!splitter) return;
        let dragging = false;
        let startX = 0;
        let startWidth = 0;
        splitter.addEventListener('pointerdown', event => {
            if (event.button !== 0 || S.compact) return;
            dragging = true;
            startX = event.clientX;
            startWidth = S.prefs.createWidth;
            if (S.prefs.createCollapsed) { S.prefs.createCollapsed = false; applyCreateLayout(S); }
            splitter.classList.add('is-dragging');
            try { splitter.setPointerCapture(event.pointerId); } catch (_) {}
            event.preventDefault();
        });
        splitter.addEventListener('pointermove', event => {
            if (!dragging) return;
            setCreateWidth(S, startWidth + (event.clientX - startX));
        });
        const stop = event => {
            if (!dragging) return;
            dragging = false;
            splitter.classList.remove('is-dragging');
            try { splitter.releasePointerCapture(event.pointerId); } catch (_) {}
            savePrefs(S.prefs);
            refreshMenus(S);
        };
        splitter.addEventListener('pointerup', stop);
        splitter.addEventListener('pointercancel', stop);
        splitter.addEventListener('dblclick', () => setCreateCollapsed(S, !S.prefs.createCollapsed));
        splitter.addEventListener('keydown', event => {
            let width = null;
            if (event.key === 'ArrowLeft') width = S.prefs.createWidth - 16;
            else if (event.key === 'ArrowRight') width = S.prefs.createWidth + 16;
            else if (event.key === 'Home') width = CREATE_MIN;
            else if (event.key === 'End') width = CREATE_MAX;
            else if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); setCreateCollapsed(S, !S.prefs.createCollapsed); return; }
            if (width == null) return;
            event.preventDefault();
            S.prefs.createCollapsed = false;
            setCreateWidth(S, width);
            savePrefs(S.prefs);
        });
    }

    function wireResize(S) {
        if (typeof ResizeObserver !== 'function') { setCompact(S, S.root.clientWidth > 0 && S.root.clientWidth < COMPACT_WIDTH); return; }
        S.resizeObserver = new ResizeObserver(entries => {
            if (S.disposed) return;
            const width = entries[0] && entries[0].contentRect ? entries[0].contentRect.width : S.root.clientWidth;
            if (width > 0) setCompact(S, width < COMPACT_WIDTH);
        });
        S.resizeObserver.observe(S.root);
        if (S.root.clientWidth > 0) setCompact(S, S.root.clientWidth < COMPACT_WIDTH);
    }

    function setCompact(S, compact) {
        if (S.compact === compact) return;
        S.compact = compact;
        S.root.classList.toggle('is-compact', compact);
        const paneSwitch = qs(S, '[data-nm-pane-switch]');
        if (paneSwitch) paneSwitch.hidden = !compact || S.root.classList.contains('is-onboarding');
        if (compact && !S.paneChosen && S.loadedOnce) S.activePane = S.tracks.length ? 'library' : 'create';
        applyCreateLayout(S);
        setActivePane(S, S.activePane);
        refreshMenus(S);
    }

    function setActivePane(S, pane) {
        S.activePane = pane === 'library' ? 'library' : 'create';
        S.root.dataset.nmPane = S.activePane;
        S.root.querySelectorAll('[data-nm-pane-btn]').forEach(btn => {
            const active = btn.dataset.nmPaneBtn === S.activePane;
            btn.classList.toggle('is-active', active);
            btn.setAttribute('aria-selected', active ? 'true' : 'false');
        });
    }

    // ---------- lifecycle ----------

    function render(host, windowId, context) {
        dispose(windowId);
        const ctx = Object.assign({
            esc: value => String(value == null ? '' : value),
            t: key => key,
            api: () => Promise.reject(new Error('api unavailable')),
            notify: () => {},
            confirmDialog: async () => false
        }, context || {});
        const t = makeT(ctx);
        const tFull = t;
        const S = {
            host, windowId, ctx, t, tFull,
            prefs: readPrefs(),
            lang: uiLang(),
            readonly: !!ctx.readonly,
            disposed: false,
            controllers: new Set(),
            caps: null,
            root: null,
            create: null, library: null, player: null,
            tracks: [], tracksTotal: 0, query: '', filter: 'all',
            tracksLoading: false, loadedOnce: false, loadSeq: 0,
            statusTimer: null,
            generation: emptyGeneration(),
            compact: false, activePane: 'create', paneChosen: false,
            nowPlayingOpen: false, visualizerAvailable: true,
            resizeObserver: null
        };
        instances.set(windowId, S);
        host.innerHTML = shellMarkup(S);
        S.root = host.querySelector('.noisemaker-app');
        if (typeof ctx.wireContextMenuBoundary === 'function') ctx.wireContextMenuBoundary(S.root);
        wireHeader(S);
        qs(S, '.nm-body').innerHTML = '<div class="nm-loading nm-muted">' + ctx.esc(t('desktop.noisemaker_library_loading')) + '</div>';
        loadState(S);
    }

    function dispose(windowId) {
        const S = instances.get(windowId);
        if (!S) return;
        instances.delete(windowId);
        S.disposed = true;
        clearTimeout(S.statusTimer);
        S.controllers.forEach(controller => { try { controller.abort(); } catch (_) {} });
        S.controllers.clear();
        teardownModules(S);
        if (S.host) S.host.innerHTML = '';
        S.root = null;
    }

    window.NoisemakerApp = { render, dispose };
})();
