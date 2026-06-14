(function () {
    'use strict';

    const IPTV_API_BASE = 'https://iptv-org.github.io/api';
    const CHANNELS_ENDPOINT = IPTV_API_BASE + '/channels.json';
    const STREAMS_ENDPOINT = IPTV_API_BASE + '/streams.json';
    const CATEGORIES_ENDPOINT = IPTV_API_BASE + '/categories.json';
    const FAVORITES_KEY = 'aurago.teevee.favorites.v1';
    const RECENT_KEY = 'aurago.teevee.recent.v1';
    const SEARCH_DELAY = 240;
    const MAX_FAVORITES = 80;
    const MAX_RECENT = 30;
    const MAX_VISIBLE_CHANNELS = 160;
    const STREAM_UNAVAILABLE_FALLBACK = 'Stream unavailable';
    const disposers = new Map();
    const catalogCache = window.TeeVeeCatalogCache || (window.TeeVeeCatalogCache = { promise: null, data: null, loadedAt: 0 });

    const filters = [
        { id: 'de', label: 'desktop.teevee_filter_germany', fallback: 'Germany', country: 'DE' },
        { id: 'favorites', label: 'desktop.teevee_filter_favorites', fallback: 'Favorites', favorites: true },
        { id: 'global', label: 'desktop.teevee_filter_global', fallback: 'Global' },
        { id: 'news', label: 'desktop.teevee_filter_news', fallback: 'News', category: 'news' },
        { id: 'sports', label: 'desktop.teevee_filter_sports', fallback: 'Sports', category: 'sports' },
        { id: 'movies', label: 'desktop.teevee_filter_movies', fallback: 'Movies', category: 'movies' },
        { id: 'music', label: 'desktop.teevee_filter_music', fallback: 'Music', category: 'music' },
        { id: 'kids', label: 'desktop.teevee_filter_kids', fallback: 'Kids', category: 'kids' },
        { id: 'documentary', label: 'desktop.teevee_filter_documentary', fallback: 'Documentary', category: 'documentary' }
    ];

    function render(host, windowId, context) {
        if (!host) return;
        const ctx = context || {};
        const esc = ctx.esc || escapeHTML;
        const t = ctx.t || ((key, fallback) => fallback || key);
        const iconMarkup = ctx.iconMarkup || ((key, fallback) => `<span>${esc(fallback || key || '')}</span>`);
        const video = document.createElement('video');
        video.preload = 'metadata';
        video.crossOrigin = 'anonymous';
        video.playsInline = true;

        const state = {
            activeFilter: 'de',
            entries: [],
            visible: [],
            search: '',
            favorites: loadFavorites(),
            recent: loadRecent(),
            current: null,
            playing: false,
            muted: false,
            volume: 0.78,
            loading: true,
            error: '',
            hls: null,
            catalogLoadedAt: 0
        };
        video.volume = state.volume;

        host.innerHTML = `<div class="teevee-app" data-teevee-app="${esc(windowId)}">
            <aside class="teevee-sidebar">
                <div class="teevee-brand">
                    <span class="teevee-brand-icon">${iconMarkup('teevee', 'TV', 'teevee-brand-glyph', 28)}</span>
                    <div>
                        <strong>${esc(t('desktop.app_teevee', 'TeeVee'))}</strong>
                        <span>${esc(t('desktop.teevee_source', 'iptv-org'))}</span>
                    </div>
                </div>
                <label class="teevee-search">
                    ${iconMarkup('search', 'S', 'teevee-search-icon', 15)}
                    <input type="search" data-search autocomplete="off" spellcheck="false" placeholder="${esc(t('desktop.teevee_search_placeholder', 'Search channels...'))}" inputmode="search" enterkeyhint="search" autocapitalize="off">
                </label>
                <nav class="teevee-filters" aria-label="${esc(t('desktop.teevee_filters', 'Filters'))}" data-filters></nav>
                <section class="teevee-recent" data-recent-section hidden>
                    <h3>${esc(t('desktop.teevee_recent', 'Recent'))}</h3>
                    <div data-recent-list></div>
                </section>
            </aside>
            <main class="teevee-main">
                <section class="teevee-stage">
                    <div class="teevee-player" data-player>
                        <div class="teevee-video-shell" data-video-shell>
                            <div class="teevee-video-mount" data-video-mount></div>
                            <div class="teevee-player-state" data-player-state>
                                <span class="teevee-live-dot"></span>
                                <strong data-state-title>${esc(t('desktop.teevee_no_channel', 'No channel selected'))}</strong>
                                <span data-state-meta>${esc(t('desktop.teevee_status_ready', 'Choose a channel'))}</span>
                            </div>
                        </div>
                        <div class="teevee-player-bar">
                            <button class="teevee-icon-button teevee-primary" type="button" data-action="toggle" aria-label="${esc(t('desktop.teevee_play', 'Play'))}">${iconMarkup('video', 'P', 'teevee-button-icon', 17)}</button>
                            <button class="teevee-icon-button" type="button" data-action="stop" aria-label="${esc(t('desktop.teevee_stop', 'Stop'))}">${iconMarkup('stop', 'S', 'teevee-button-icon', 16)}</button>
                            <div class="teevee-now">
                                <span>${esc(t('desktop.teevee_now_playing', 'Now playing'))}</span>
                                <strong data-now-title>${esc(t('desktop.teevee_no_channel', 'No channel selected'))}</strong>
                                <em data-now-meta></em>
                            </div>
                            <button class="teevee-icon-button" type="button" data-action="favorite-current" aria-label="${esc(t('desktop.teevee_add_favorite', 'Add to favorites'))}">${iconMarkup('heart', 'F', 'teevee-button-icon', 16)}</button>
                            <button class="teevee-icon-button" type="button" data-action="mute" aria-label="${esc(t('desktop.teevee_mute', 'Mute'))}">${iconMarkup('audio', 'V', 'teevee-button-icon', 16)}</button>
                            <input class="teevee-volume" type="range" min="0" max="100" value="78" data-volume aria-label="${esc(t('desktop.teevee_volume', 'Volume'))}">
                            <button class="teevee-icon-button" type="button" data-action="fullscreen" aria-label="${esc(t('desktop.teevee_fullscreen', 'Fullscreen'))}">${iconMarkup('maximize', 'F', 'teevee-button-icon', 16)}</button>
                        </div>
                    </div>
                </section>
                <section class="teevee-list-panel">
                    <div class="teevee-list-head">
                        <div>
                            <strong data-list-title>${esc(t('desktop.teevee_filter_germany', 'Germany'))}</strong>
                            <span data-list-count></span>
                        </div>
                        <button class="teevee-refresh" type="button" data-action="refresh" aria-label="${esc(t('desktop.teevee_refresh', 'Refresh'))}">${iconMarkup('refresh', 'R', 'teevee-button-icon', 15)}</button>
                    </div>
                    <div class="teevee-status" data-status hidden></div>
                    <div class="teevee-channel-list" data-channel-list></div>
                </section>
            </main>
            <div class="teevee-toast" data-toast hidden></div>
        </div>`;

        const root = host.querySelector('.teevee-app');
        const filtersEl = host.querySelector('[data-filters]');
        const searchInput = host.querySelector('[data-search]');
        const listEl = host.querySelector('[data-channel-list]');
        const listTitle = host.querySelector('[data-list-title]');
        const listCount = host.querySelector('[data-list-count]');
        const statusEl = host.querySelector('[data-status]');
        const playerMount = host.querySelector('[data-video-mount]');
        const playerShell = host.querySelector('[data-video-shell]');
        const playerState = host.querySelector('[data-player-state]');
        const stateTitle = host.querySelector('[data-state-title]');
        const stateMeta = host.querySelector('[data-state-meta]');
        const nowTitle = host.querySelector('[data-now-title]');
        const nowMeta = host.querySelector('[data-now-meta]');
        const toggleBtn = host.querySelector('[data-action="toggle"]');
        const favoriteCurrentBtn = host.querySelector('[data-action="favorite-current"]');
        const muteBtn = host.querySelector('[data-action="mute"]');
        const volumeInput = host.querySelector('[data-volume]');
        const toast = host.querySelector('[data-toast]');
        const recentSection = host.querySelector('[data-recent-section]');
        const recentList = host.querySelector('[data-recent-list]');
        let searchTimer = 0;
        let toastTimer = 0;

        playerMount.appendChild(video);
        if (typeof ctx.wireContextMenuBoundary === 'function') ctx.wireContextMenuBoundary(host);

        function renderFilters() {
            filtersEl.innerHTML = filters.map(filter => {
                const active = state.activeFilter === filter.id;
                const count = filter.favorites ? state.favorites.length : '';
                return `<button class="teevee-filter ${active ? 'active' : ''}" type="button" data-filter="${esc(filter.id)}" aria-pressed="${active ? 'true' : 'false'}">
                    <span>${esc(t(filter.label, filter.fallback))}</span>
                    ${count ? `<em>${esc(count)}</em>` : ''}
                </button>`;
            }).join('');
            filtersEl.querySelectorAll('[data-filter]').forEach(button => {
                button.addEventListener('click', () => {
                    state.activeFilter = button.dataset.filter || 'de';
                    updateVisible();
                    renderAll();
                });
            });
        }

        function renderRecent() {
            const available = state.recent
                .map(id => state.entries.find(entry => entry.id === id))
                .filter(Boolean)
                .slice(0, 6);
            recentSection.hidden = !available.length;
            recentList.innerHTML = available.map(entry => `<button type="button" data-recent="${esc(entry.id)}">
                <span>${esc(entry.name)}</span>
                <em>${esc(entry.country || '')}</em>
            </button>`).join('');
            recentList.querySelectorAll('[data-recent]').forEach(button => {
                button.addEventListener('click', () => {
                    const entry = state.entries.find(item => item.id === button.dataset.recent);
                    if (entry) playChannel(entry);
                });
            });
        }

        function renderList() {
            const filter = currentFilter();
            listTitle.textContent = state.search ? t('desktop.teevee_filter_global', 'Global') : t(filter.label, filter.fallback);
            listCount.textContent = state.loading ? '' : String(state.visible.length);
            statusEl.hidden = !state.error && !state.loading;
            statusEl.textContent = state.loading ? t('desktop.teevee_loading', 'Loading channels...') : state.error;
            if (state.loading) {
                listEl.innerHTML = Array.from({ length: 9 }).map(() => '<article class="teevee-channel teevee-skeleton"><span></span><strong></strong><em></em></article>').join('');
                return;
            }
            if (!state.visible.length) {
                listEl.innerHTML = `<div class="teevee-empty">${iconMarkup('teevee', 'TV', 'teevee-empty-icon', 32)}<strong>${esc(t('desktop.teevee_no_results', 'No channels found'))}</strong></div>`;
                return;
            }
            listEl.innerHTML = state.visible.map(channelCard).join('');
            listEl.querySelectorAll('[data-channel-id]').forEach(card => {
                card.addEventListener('click', event => {
                    if (event.target.closest('[data-action="favorite"]')) return;
                    const entry = state.entries.find(item => item.id === card.dataset.channelId);
                    if (entry) playChannel(entry);
                });
                card.addEventListener('keydown', event => {
                    if (event.key === 'Enter' || event.key === ' ') {
                        event.preventDefault();
                        const entry = state.entries.find(item => item.id === card.dataset.channelId);
                        if (entry) playChannel(entry);
                    }
                });
                card.addEventListener('contextmenu', event => {
                    const entry = state.entries.find(item => item.id === card.dataset.channelId);
                    showChannelContextMenu(event, entry);
                });
            });
            listEl.querySelectorAll('[data-action="favorite"]').forEach(button => {
                button.addEventListener('click', event => {
                    event.stopPropagation();
                    const entry = state.entries.find(item => item.id === button.dataset.channelId);
                    if (entry) toggleFavorite(entry);
                });
            });
            listEl.querySelectorAll('img[data-logo]').forEach(img => {
                img.addEventListener('error', () => {
                    img.removeAttribute('src');
                    img.hidden = true;
                }, { once: true });
            });
        }

        function channelCard(entry) {
            const favorite = isFavorite(entry);
            const active = state.current && state.current.id === entry.id;
            const unsupported = entry.unsupported;
            const logo = entry.logo || '';
            const meta = [countryFlag(entry.country), entry.country, qualityText(entry), categoryText(entry)].filter(Boolean).join(' | ');
            return `<article class="teevee-channel ${active ? 'active' : ''} ${unsupported ? 'unsupported' : ''}" role="button" tabindex="0" data-channel-id="${esc(entry.id)}" aria-label="${esc(t('desktop.teevee_play', 'Play'))} ${esc(entry.name)}">
                <div class="teevee-channel-logo">
                    ${logo ? `<img data-logo src="${esc(logo)}" alt="">` : iconMarkup('teevee', 'TV', 'teevee-channel-icon', 22)}
                </div>
                <div class="teevee-channel-body">
                    <strong title="${esc(entry.name)}">${esc(entry.name)}</strong>
                    <span title="${esc(meta)}">${esc(meta || entry.url)}</span>
                </div>
                <div class="teevee-channel-side">
                    ${unsupported ? `<span class="teevee-badge">${esc(t('desktop.teevee_unsupported_badge', 'Headers'))}</span>` : `<span class="teevee-live">${esc(t('desktop.teevee_live', 'LIVE'))}</span>`}
                    <button class="teevee-favorite ${favorite ? 'active' : ''}" type="button" data-action="favorite" data-channel-id="${esc(entry.id)}" aria-label="${esc(favorite ? t('desktop.teevee_remove_favorite', 'Remove favorite') : t('desktop.teevee_add_favorite', 'Add favorite'))}">${favorite ? '*' : '+'}</button>
                </div>
            </article>`;
        }

        function renderPlayer() {
            const current = state.current;
            const unavailable = state.error && current;
            nowTitle.textContent = current ? current.name : t('desktop.teevee_no_channel', 'No channel selected');
            nowMeta.textContent = current ? [countryFlag(current.country), current.country, qualityText(current)].filter(Boolean).join(' | ') : '';
            stateTitle.textContent = current ? current.name : t('desktop.teevee_no_channel', 'No channel selected');
            stateMeta.textContent = unavailable ? state.error : (current ? [t('desktop.teevee_live', 'LIVE'), current.country || '', qualityText(current)].filter(Boolean).join(' | ') : t('desktop.teevee_status_ready', 'Choose a channel'));
            playerState.hidden = state.playing && !unavailable;
            root.classList.toggle('is-playing', state.playing);
            root.classList.toggle('is-muted', state.muted);
            toggleBtn.classList.toggle('active', state.playing);
            toggleBtn.setAttribute('aria-label', state.playing ? t('desktop.teevee_pause', 'Pause') : t('desktop.teevee_play', 'Play'));
            toggleBtn.innerHTML = iconMarkup(state.playing ? 'stop' : 'video', state.playing ? 'P' : 'P', 'teevee-button-icon', 17);
            muteBtn.classList.toggle('active', state.muted);
            muteBtn.setAttribute('aria-label', state.muted ? t('desktop.teevee_unmute', 'Unmute') : t('desktop.teevee_mute', 'Mute'));
            favoriteCurrentBtn.classList.toggle('active', current && isFavorite(current));
            setWindowMenus();
        }

        function renderAll() {
            renderFilters();
            renderRecent();
            renderList();
            renderPlayer();
        }

        function updateVisible() {
            const query = normalizeSearch(state.search);
            const favoriteIDs = new Set(state.favorites);
            let entries = state.entries;
            if (query) {
                entries = entries.filter(entry => searchableText(entry).includes(query));
            } else {
                const filter = currentFilter();
                if (filter.favorites) {
                    entries = entries.filter(entry => favoriteIDs.has(entry.favoriteKey));
                } else if (filter.country) {
                    entries = entries.filter(entry => entry.country === filter.country);
                } else if (filter.category) {
                    entries = entries.filter(entry => entry.categories.includes(filter.category));
                }
            }
            state.visible = entries.slice(0, MAX_VISIBLE_CHANNELS);
        }

        async function loadCatalog(force) {
            state.loading = true;
            state.error = '';
            renderAll();
            try {
                const data = await fetchCatalog(force);
                state.entries = data.entries;
                state.catalogLoadedAt = data.loadedAt;
                updateVisible();
            } catch (err) {
                state.error = err.message || t('desktop.teevee_catalog_error', 'Could not load TV catalog');
                state.visible = [];
            } finally {
                state.loading = false;
                renderAll();
            }
        }

        function showChannelContextMenu(event, entry) {
            if (!entry || typeof ctx.showContextMenu !== 'function') return false;
            event.preventDefault();
            ctx.showContextMenu(event.clientX, event.clientY, [
                { labelKey: 'desktop.teevee_play', icon: 'video', disabled: entry.unsupported, action: () => playChannel(entry) },
                { labelKey: 'desktop.menu_favorite', icon: 'heart', checked: isFavorite(entry), action: () => toggleFavorite(entry) },
                { type: 'separator' },
                { labelKey: 'desktop.teevee_refresh', icon: 'refresh', action: () => loadCatalog(true) }
            ]);
            return true;
        }

        async function playChannel(entry) {
            if (!entry) return;
            if (entry.unsupported) {
                state.current = entry;
                state.error = t('desktop.teevee_unsupported_stream', 'This stream needs browser-blocked headers.');
                showToast(state.error);
                renderPlayer();
                renderList();
                return;
            }
            resetPlayback();
            state.current = entry;
            state.error = '';
            renderPlayer();
            try {
                await attachVideoSource(entry);
                await video.play();
                state.playing = true;
                state.error = '';
                rememberRecent(entry);
                updateMediaSession(entry);
            } catch (err) {
                state.playing = false;
                state.error = err && err.message ? err.message : t('desktop.teevee_stream_unavailable', STREAM_UNAVAILABLE_FALLBACK);
                showToast(state.error);
            }
            renderAll();
        }

        async function attachVideoSource(entry) {
            const url = entry.url;
            if (!url) throw new Error(t('desktop.teevee_stream_unavailable', STREAM_UNAVAILABLE_FALLBACK));
            if (isHLSURL(url)) {
                if (video.canPlayType('application/vnd.apple.mpegurl')) {
                    video.src = url;
                    video.load();
                    return;
                }
                if (window.Hls && window.Hls.isSupported && window.Hls.isSupported()) {
                    state.hls = new window.Hls({ enableWorker: true, lowLatencyMode: true, backBufferLength: 60 });
                    state.hls.on(window.Hls.Events.ERROR, function (_event, data) {
                        if (data && data.fatal) {
                            state.error = t('desktop.teevee_stream_unavailable', STREAM_UNAVAILABLE_FALLBACK);
                            showToast(state.error);
                            resetPlayback();
                            renderAll();
                        }
                    });
                    state.hls.attachMedia(video);
                    state.hls.loadSource(url);
                    return;
                }
                throw new Error(t('desktop.teevee_stream_unavailable', STREAM_UNAVAILABLE_FALLBACK));
            }
            video.src = url;
            video.load();
        }

        function resetPlayback() {
            if (state.hls) {
                try { state.hls.destroy(); } catch (_) {}
                state.hls = null;
            }
            video.pause();
            video.removeAttribute('src');
            video.load();
            state.playing = false;
        }

        function stopPlayback() {
            resetPlayback();
            renderAll();
        }

        function toggleFavorite(entry) {
            const key = entry.favoriteKey;
            const index = state.favorites.indexOf(key);
            if (index >= 0) {
                state.favorites.splice(index, 1);
            } else {
                state.favorites.unshift(key);
                if (state.favorites.length > MAX_FAVORITES) state.favorites.length = MAX_FAVORITES;
            }
            saveFavorites(state.favorites);
            updateVisible();
            renderAll();
        }

        function isFavorite(entry) {
            return !!entry && state.favorites.includes(entry.favoriteKey);
        }

        function rememberRecent(entry) {
            state.recent = [entry.id].concat(state.recent.filter(id => id !== entry.id)).slice(0, MAX_RECENT);
            saveRecent(state.recent);
        }

        function showToast(message) {
            toast.textContent = message || t('desktop.teevee_stream_unavailable', STREAM_UNAVAILABLE_FALLBACK);
            toast.hidden = false;
            clearTimeout(toastTimer);
            toastTimer = setTimeout(() => { toast.hidden = true; }, 3600);
        }

        function currentFilter() {
            return filters.find(filter => filter.id === state.activeFilter) || filters[0];
        }

        function setWindowMenus() {
            if (typeof ctx.setWindowMenus !== 'function') return;
            ctx.setWindowMenus(windowId, [
                {
                    id: 'view',
                    labelKey: 'desktop.menu_view',
                    items: [
                        { id: 'refresh', labelKey: 'desktop.teevee_refresh', icon: 'refresh', shortcut: 'F5', action: () => loadCatalog(true) },
                        { id: 'fullscreen', labelKey: 'desktop.teevee_fullscreen', icon: 'maximize', disabled: !state.current, action: requestPlayerFullscreen }
                    ]
                },
                {
                    id: 'playback',
                    labelKey: 'desktop.menu_playback',
                    items: [
                        { id: 'play-pause', labelKey: 'desktop.menu_play_pause', icon: 'video', disabled: !state.current || state.current.unsupported, action: () => toggleBtn.click() },
                        { id: 'stop', labelKey: 'desktop.teevee_stop', icon: 'stop', disabled: !state.current, action: stopPlayback },
                        { id: 'mute', labelKey: 'desktop.menu_mute', icon: 'audio', checked: state.muted, action: () => muteBtn.click() },
                        { id: 'favorite', labelKey: 'desktop.menu_favorite', icon: 'heart', disabled: !state.current, checked: state.current && isFavorite(state.current), action: () => favoriteCurrentBtn.click() }
                    ]
                }
            ]);
        }

        function requestPlayerFullscreen() {
            if (playerShell && playerShell.requestFullscreen) {
                playerShell.requestFullscreen().catch(() => {});
            }
        }

        searchInput.addEventListener('input', () => {
            clearTimeout(searchTimer);
            searchTimer = setTimeout(() => {
                state.search = searchInput.value || '';
                updateVisible();
                renderAll();
            }, SEARCH_DELAY);
        });
        host.querySelector('[data-action="refresh"]').addEventListener('click', () => loadCatalog(true));
        host.querySelector('[data-action="fullscreen"]').addEventListener('click', requestPlayerFullscreen);
        host.querySelector('[data-action="stop"]').addEventListener('click', stopPlayback);
        toggleBtn.addEventListener('click', () => {
            if (!state.current || state.current.unsupported) return;
            if (video.paused) {
                video.play().then(() => {
                    state.playing = true;
                    renderPlayer();
                }).catch(err => {
                    state.error = err.message || t('desktop.teevee_stream_unavailable', STREAM_UNAVAILABLE_FALLBACK);
                    showToast(state.error);
                    renderAll();
                });
            } else {
                video.pause();
                state.playing = false;
                renderPlayer();
            }
        });
        muteBtn.addEventListener('click', () => {
            state.muted = !state.muted;
            video.muted = state.muted;
            renderPlayer();
        });
        favoriteCurrentBtn.addEventListener('click', () => {
            if (state.current) toggleFavorite(state.current);
        });
        volumeInput.addEventListener('input', () => {
            state.volume = Number(volumeInput.value) / 100;
            video.volume = state.volume;
        });
        video.addEventListener('playing', () => {
            state.playing = true;
            state.error = '';
            renderPlayer();
        });
        video.addEventListener('pause', () => {
            state.playing = false;
            renderPlayer();
        });
        video.addEventListener('error', () => {
            if (!state.current) return;
            state.playing = false;
            state.error = t('desktop.teevee_stream_unavailable', STREAM_UNAVAILABLE_FALLBACK);
            showToast(state.error);
            renderAll();
        });
        if ('mediaSession' in navigator) {
            try {
                navigator.mediaSession.setActionHandler('play', () => {
                    if (state.current && !state.current.unsupported) video.play().catch(() => {});
                });
                navigator.mediaSession.setActionHandler('pause', () => video.pause());
                navigator.mediaSession.setActionHandler('stop', stopPlayback);
            } catch (_) {}
        }

        disposers.set(windowId, () => {
            clearTimeout(searchTimer);
            clearTimeout(toastTimer);
            resetPlayback();
            if (typeof ctx.clearWindowMenus === 'function') ctx.clearWindowMenus(windowId);
        });

        renderAll();
        loadCatalog(false);
    }

    function dispose(windowId) {
        const cleanup = disposers.get(windowId);
        if (!cleanup) return;
        cleanup();
        disposers.delete(windowId);
    }

    async function fetchCatalog(force) {
        const now = Date.now();
        if (!force && catalogCache.data && now - catalogCache.loadedAt < 1000 * 60 * 30) return catalogCache.data;
        if (!force && catalogCache.promise) return catalogCache.promise;
        catalogCache.promise = Promise.all([
            fetchJSON(CHANNELS_ENDPOINT),
            fetchJSON(STREAMS_ENDPOINT),
            fetchJSON(CATEGORIES_ENDPOINT)
        ]).then(([channels, streams, categories]) => {
            const data = {
                entries: joinStreamsWithChannels(channels, streams, categories),
                loadedAt: Date.now()
            };
            catalogCache.data = data;
            catalogCache.loadedAt = data.loadedAt;
            return data;
        }).finally(() => {
            catalogCache.promise = null;
        });
        return catalogCache.promise;
    }

    async function fetchJSON(url) {
        const response = await fetch(url, { cache: 'force-cache' });
        if (!response.ok) throw new Error('iptv-org HTTP ' + response.status);
        return response.json();
    }

    function joinStreamsWithChannels(channels, streams, categories) {
        const channelByID = new Map((Array.isArray(channels) ? channels : [])
            .filter(channel => channel && channel.id)
            .map(channel => [channel.id, channel]));
        const categoryByID = new Map((Array.isArray(categories) ? categories : [])
            .filter(category => category && category.id)
            .map(category => [category.id, category.name || category.id]));
        const rows = Array.isArray(streams) ? streams : [];
        return rows.map((stream, index) => {
            if (!stream || !stream.url) return null;
            const channel = channelByID.get(stream.channel || '') || null;
            const channelCategories = Array.isArray(channel && channel.categories) ? channel.categories.map(cleanID).filter(Boolean) : [];
            const country = clean(channel && channel.country).toUpperCase();
            const name = clean(channel && channel.name) || clean(stream.title) || stream.url;
            const entry = {
                id: clean(stream.channel || stream.title || 'stream') + ':' + index,
                favoriteKey: clean(stream.channel || stream.url || stream.title),
                channelID: clean(stream.channel),
                name,
                url: clean(stream.url),
                country,
                categories: channelCategories,
                categoryNames: channelCategories.map(id => categoryByID.get(id) || id),
                quality: clean(stream.quality || stream.label),
                label: clean(stream.label),
                logo: clean(channel && channel.logo),
                website: clean(channel && channel.website),
                isNsfw: !!(channel && channel.is_nsfw),
                closed: clean(channel && channel.closed),
                replacedBy: clean(channel && channel.replaced_by),
                unsupported: isUnsupportedStream(stream)
            };
            return entry;
        }).filter(entry => entry && entry.url && !entry.isNsfw && !entry.closed && !entry.replacedBy)
            .sort(sortEntries);
    }

    function isUnsupportedStream(stream) {
        return !!(stream && (stream.user_agent || stream.referrer));
    }

    function sortEntries(a, b) {
        if (a.country === 'DE' && b.country !== 'DE') return -1;
        if (a.country !== 'DE' && b.country === 'DE') return 1;
        return a.name.localeCompare(b.name, undefined, { sensitivity: 'base' });
    }

    function isHLSURL(url) {
        return /\.m3u8(?:[?#].*)?$/i.test(clean(url)) || /\/hls(?:\/|\?|$)/i.test(clean(url));
    }

    function searchableText(entry) {
        return normalizeSearch([
            entry.name,
            entry.country,
            entry.quality,
            entry.label,
            entry.channelID,
            entry.categories.join(' '),
            entry.categoryNames.join(' ')
        ].join(' '));
    }

    function categoryText(entry) {
        return (entry.categoryNames || []).slice(0, 2).join(', ');
    }

    function qualityText(entry) {
        return clean(entry.quality || entry.label) || '';
    }

    function loadFavorites() {
        try {
            const parsed = JSON.parse(localStorage.getItem(FAVORITES_KEY) || '[]');
            return Array.isArray(parsed) ? parsed.map(clean).filter(Boolean).slice(0, MAX_FAVORITES) : [];
        } catch (_) {
            return [];
        }
    }

    function saveFavorites(favorites) {
        try {
            localStorage.setItem(FAVORITES_KEY, JSON.stringify(favorites.slice(0, MAX_FAVORITES)));
        } catch (_) {}
    }

    function loadRecent() {
        try {
            const parsed = JSON.parse(localStorage.getItem(RECENT_KEY) || '[]');
            return Array.isArray(parsed) ? parsed.map(clean).filter(Boolean).slice(0, MAX_RECENT) : [];
        } catch (_) {
            return [];
        }
    }

    function saveRecent(recent) {
        try {
            localStorage.setItem(RECENT_KEY, JSON.stringify(recent.slice(0, MAX_RECENT)));
        } catch (_) {}
    }

    function updateMediaSession(entry) {
        if (!('mediaSession' in navigator) || !entry) return;
        try {
            navigator.mediaSession.metadata = new MediaMetadata({
                title: entry.name || 'TeeVee',
                artist: entry.country || '',
                album: 'AuraGo TeeVee',
                artwork: entry.logo ? [{ src: entry.logo, sizes: '96x96', type: 'image/png' }] : []
            });
        } catch (_) {}
    }

    function countryFlag(code) {
        const value = clean(code).toUpperCase();
        if (!/^[A-Z]{2}$/.test(value)) return '';
        return String.fromCodePoint.apply(String, value.split('').map(ch => 0x1F1E6 + ch.charCodeAt(0) - 65));
    }

    function normalizeSearch(value) {
        return clean(value).toLowerCase().normalize('NFD').replace(/[\u0300-\u036f]/g, '');
    }

    function cleanID(value) {
        return clean(value).toLowerCase();
    }

    function clean(value) {
        return String(value == null ? '' : value).trim();
    }

    function escapeHTML(value) {
        return String(value == null ? '' : value)
            .replaceAll('&', '&amp;')
            .replaceAll('<', '&lt;')
            .replaceAll('>', '&gt;')
            .replaceAll('"', '&quot;')
            .replaceAll("'", '&#39;');
    }

    window.TeeVeeApp = { render, dispose };
})();
