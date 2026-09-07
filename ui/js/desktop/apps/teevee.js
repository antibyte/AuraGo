(function () {
    'use strict';

    const IPTV_API_BASE = 'https://iptv-org.github.io/api';
    const CHANNELS_ENDPOINT = IPTV_API_BASE + '/channels.json';
    const STREAMS_ENDPOINT = IPTV_API_BASE + '/streams.json';
    const CATEGORIES_ENDPOINT = IPTV_API_BASE + '/categories.json';
    const FAVORITES_KEY = 'aurago.teevee.favorites.v2';
    const LEGACY_FAVORITES_KEY = 'aurago.teevee.favorites.v1';
    const RECENT_KEY = 'aurago.teevee.recent.v1';
    const APPEARANCE_KEY = 'aurago.teevee.appearance.v1';
    const SEARCH_DELAY = 240;
    const MAX_FAVORITES = 80;
    const MAX_RECENT = 30;
    const MAX_RECENT_SHORTCUTS = 2;
    const MAX_FAVORITE_SHORTCUTS = 3;
    const MAX_VISIBLE_CHANNELS = 160;
    const VISIBLE_BATCH = 40;
    const STREAM_UNAVAILABLE_FALLBACK = 'Stream unavailable';
    const TEEVEE_STREAM_PROXY = '/api/desktop/teevee/stream';
    const DEFAULT_COUNTRY = 'DE';
    const ALL_COUNTRIES = 'all';
    const ALL_RESOLUTIONS = 'all';
    const disposers = new Map();
    const catalogCache = window.TeeVeeCatalogCache || (window.TeeVeeCatalogCache = { promise: null, data: null, loadedAt: 0 });

    const Media = window.AuraDesktopMediaHelpers;
    const clean = Media.clean;
    const cleanID = Media.cleanID;
    const escapeHTML = Media.escapeHTML;
    const normalizeSearch = Media.normalizeSearch;
    const hashString = Media.hashString;
    const countryFlag = Media.countryFlag;
    const countryDisplayName = Media.countryDisplayName;
    const debounce = Media.debounce;
    const createToast = Media.createToast;
    const updateMediaSession = Media.updateMediaSession;

    const filters = [
        { id: 'all', icon: 'globe', label: 'desktop.teevee_filter_global', fallback: 'Global' },
        { id: 'favorites', icon: 'heart', label: 'desktop.teevee_filter_favorites', fallback: 'Favorites', favorites: true },
        { id: 'news', icon: 'news', label: 'desktop.teevee_filter_news', fallback: 'News', category: 'news' },
        { id: 'sports', icon: 'sports', label: 'desktop.teevee_filter_sports', fallback: 'Sports', category: 'sports' },
        { id: 'movies', icon: 'film', label: 'desktop.teevee_filter_movies', fallback: 'Movies', category: 'movies' },
        { id: 'music', icon: 'music', label: 'desktop.teevee_filter_music', fallback: 'Music', category: 'music' },
        { id: 'kids', icon: 'people', label: 'desktop.teevee_filter_kids', fallback: 'Kids', category: 'kids' },
        { id: 'documentary', icon: 'news', label: 'desktop.teevee_filter_documentary', fallback: 'Documentary', category: 'documentary' }
    ];

    const resolutionFilters = [
        { id: ALL_RESOLUTIONS, label: 'desktop.teevee_resolution_all', fallback: 'All resolutions' },
        { id: 'uhd', label: 'desktop.teevee_resolution_uhd', fallback: 'UHD / 4K' },
        { id: 'fhd', label: 'desktop.teevee_resolution_fhd', fallback: '1080p' },
        { id: 'hd', label: 'desktop.teevee_resolution_hd', fallback: '720p' },
        { id: 'sd', label: 'desktop.teevee_resolution_sd', fallback: 'SD' },
        { id: 'unknown', label: 'desktop.teevee_resolution_unknown', fallback: 'Unknown' }
    ];

    function render(host, windowId, context) {
        if (!host) return;
        const ctx = context || {};
        const esc = ctx.esc || escapeHTML;
        const t = ctx.t || ((key, fallback) => fallback || key);
        const iconMarkup = ctx.iconMarkup || ((key, fallback) => `<span>${esc(fallback || key || '')}</span>`);
        const tvIcon = '<img class="teevee-tv-icon" src="/img/teevee/tv.png" alt="" draggable="false">';
        const screws = ['tl', 'tr', 'bl', 'br'].map(corner => `<img class="teevee-screw ${corner}" src="/img/teevee/screw.png" alt="" draggable="false">`).join('');
        const controlIcon = name => `<img class="teevee-control-icon" src="/img/teevee/${name}.svg" alt="" draggable="false">`;
        const video = document.createElement('video');
        video.preload = 'metadata';
        video.playsInline = true;

        const state = {
            activeFilter: 'all',
            countryFilter: DEFAULT_COUNTRY,
            resolutionFilter: ALL_RESOLUTIONS,
            entries: [],
            visible: [],
            totalVisible: 0,
            visibleLimit: VISIBLE_BATCH,
            countries: new Set(),
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
            catalogLoadedAt: 0,
            hlsErrorCount: 0,
            playbackID: 0,
            powered: true,
            disposed: false,
            buffering: false,
            forceProxy: false,
            appearance: loadAppearance(),
            crtMode: 'waiting',
            menuKey: '',
            catalogRequest: 0
        };
        video.volume = state.volume;

        host.innerHTML = `<div class="teevee-app" data-teevee-app="${esc(windowId)}">
            <aside class="teevee-sidebar">
                ${screws}
                <div class="teevee-brand teevee-display">
                    <span class="teevee-brand-icon"><img src="/img/teevee/brand-tv.png" alt="" draggable="false"></span>
                    <div>
                        <strong>${esc(t('desktop.app_teevee'))}</strong>
                        <span>${esc(t('desktop.teevee_source'))}</span>
                    </div>
                    <img class="teevee-color-mark" src="/img/teevee/color-mark.png" alt="">
                    <small aria-hidden="true">BROADCAST THE WORLD</small>
                </div>
                <label class="teevee-search">
                    ${controlIcon('search')}
                    <input type="search" data-search autocomplete="off" spellcheck="false" aria-label="${esc(t('desktop.teevee_search_placeholder'))}" placeholder="${esc(t('desktop.teevee_search_placeholder'))}" inputmode="search" enterkeyhint="search" autocapitalize="off">
                </label>
                <div class="teevee-control-grid">
                    <label class="teevee-select-field">
                        <span>${esc(t('desktop.teevee_country'))}</span>
                        <select data-country-filter aria-label="${esc(t('desktop.teevee_country'))}"></select>
                    </label>
                    <label class="teevee-select-field">
                        <span>${esc(t('desktop.teevee_resolution'))}</span>
                        <select data-resolution-filter aria-label="${esc(t('desktop.teevee_resolution'))}"></select>
                    </label>
                </div>
                <nav class="teevee-filters" aria-label="${esc(t('desktop.teevee_filters'))}" data-filters></nav>
                <div class="teevee-receiver-label" aria-hidden="true">TELEVISION RECEIVER<br>SOLID STATE</div>
            </aside>
                <div class="teevee-shortcuts-panel" data-shortcuts hidden>
                    <button type="button" class="teevee-popover-close" data-action="close-shortcuts" aria-label="${esc(t('desktop.close'))}">${controlIcon('close')}</button>
                    <section class="teevee-recent" data-recent-section hidden>
                        <h3>${esc(t('desktop.teevee_recent'))}</h3>
                        <div class="teevee-shortcut-list" data-recent-list></div>
                    </section>
                    <section class="teevee-favorites" data-favorites-section hidden>
                        <h3>${esc(t('desktop.teevee_filter_favorites'))}</h3>
                        <div class="teevee-shortcut-list" data-favorites-list></div>
                    </section>
                </div>
            <main class="teevee-main">
                <section class="teevee-stage">
                    <div class="teevee-player" data-player>
                        <div class="teevee-video-shell" data-video-shell>
                            <div class="teevee-tube">
                                <div class="teevee-video-mount" data-video-mount></div>
                                <div class="teevee-glass" aria-hidden="true"></div>
                            </div>
                            <img class="teevee-bezel" src="/img/teevee/bezel.png" alt="" draggable="false">
                            <button class="teevee-video-fullscreen" type="button" data-action="fullscreen-video" aria-label="${esc(t('desktop.teevee_fullscreen'))}">${controlIcon('maximize')}</button>
                            <div class="teevee-player-state" data-player-state role="status" aria-live="polite">
                                <span class="teevee-live-dot"></span>
                                <strong data-state-title>${esc(t('desktop.teevee_no_channel'))}</strong>
                                <span data-state-meta>${esc(t('desktop.teevee_status_ready'))}</span>
                            </div>
                        </div>
                        <div class="teevee-player-bar">
                            <img class="teevee-speaker" src="/img/teevee/speaker.png" alt="">
                            <div class="teevee-now teevee-display">
                                <span class="teevee-now-icon">${tvIcon}</span>
                                <div><span>${esc(t('desktop.teevee_now_playing'))}</span>
                                <strong data-now-title>${esc(t('desktop.teevee_no_channel'))}</strong><em data-now-meta></em></div>
                            </div>
                            <button class="teevee-icon-button" type="button" data-action="favorite-current" aria-label="${esc(t('desktop.teevee_add_favorite'))}">${controlIcon('heart')}</button>
                            <button class="teevee-icon-button" type="button" data-action="stop" aria-label="${esc(t('desktop.teevee_stop'))}">${controlIcon('stop')}</button>
                            <small class="teevee-band-label" aria-hidden="true">VHF &nbsp; UHF &nbsp; STEREO</small>
                        </div>
                    </div>
                </section>
                <section class="teevee-list-panel">
                    <div class="teevee-list-head">
                        <div>
                            <strong data-list-title>${esc(t('desktop.teevee_filter_germany'))}</strong>
                            <span data-list-count></span>
                        </div>
                        <button class="teevee-refresh" type="button" data-action="refresh" aria-label="${esc(t('desktop.teevee_refresh'))}"><img src="/img/teevee/bars.png" alt=""></button>
                    </div>
                    <div class="teevee-status" data-status hidden></div>
                    <div class="teevee-channel-list" data-channel-list></div>
                </section>
            </main>
            <footer class="teevee-footer" aria-hidden="true">
                ${screws}
                <img class="teevee-stripe" src="/img/teevee/stripe.png" alt="">
                <span>GOOD PROGRAMS · A BRIGHTER TOMORROW</span>
            </footer>
            <div class="teevee-maker"><img src="/img/teevee/signature.png" alt="TeeVee"><span aria-hidden="true">HOME<br>ENTERTAINMENT<br>SYSTEM</span></div>
            <button class="teevee-power" type="button" data-action="power" aria-label="${esc(t('desktop.teevee_power'))}" aria-pressed="true"><span aria-hidden="true">POWER</span><img src="/img/teevee/power.png" alt=""></button>
            <div class="teevee-transport" data-transport hidden>
                <button class="teevee-popover-close" type="button" data-action="close-transport" aria-label="${esc(t('desktop.close'))}">${controlIcon('close')}</button>
                <button class="teevee-icon-button teevee-primary" type="button" data-action="toggle" aria-label="${esc(t('desktop.teevee_play'))}">${controlIcon('play')}</button>
                <button class="teevee-icon-button" type="button" data-action="mute" aria-label="${esc(t('desktop.teevee_mute'))}">${controlIcon('volume')}</button>
                <input class="teevee-volume" type="range" min="0" max="100" value="78" data-volume aria-label="${esc(t('desktop.teevee_volume'))}">
                <button class="teevee-icon-button" type="button" data-action="fullscreen" aria-label="${esc(t('desktop.teevee_fullscreen'))}">${controlIcon('maximize')}</button>
            </div>
            <div class="teevee-filter-notice" data-crt-notice role="status" hidden></div>
            <div class="teevee-toast" data-toast hidden></div>
        </div>`;

        const root = host.querySelector('.teevee-app');
        const filtersEl = host.querySelector('[data-filters]');
        const searchInput = host.querySelector('[data-search]');
        const countrySelect = host.querySelector('[data-country-filter]');
        const resolutionSelect = host.querySelector('[data-resolution-filter]');
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
        const favoritesSection = host.querySelector('[data-favorites-section]');
        const favoritesList = host.querySelector('[data-favorites-list]');
        const recentSection = host.querySelector('[data-recent-section]');
        const recentList = host.querySelector('[data-recent-list]');
        playerMount.appendChild(video);
        const crt = window.TeeVeeCrt ? window.TeeVeeCrt.create({ video, mount: playerMount, onStatus: mode => {
            state.crtMode = mode;
            const notice = host.querySelector('[data-crt-notice]');
            notice.hidden = mode !== 'basic' || !state.powered;
            notice.textContent = t('desktop.teevee_crt_limited');
            setWindowMenus();
        } }) : null;
        const chrome = host.closest('.vd-window');
        if (chrome) {
            const titlebar = chrome.querySelector('.vd-window-titlebar');
            const titleIcon = chrome.querySelector('.vd-window-header-icon-wrap');
            if (titleIcon) titleIcon.innerHTML = tvIcon;
            if (titlebar) titlebar.insertAdjacentHTML('beforeend', screws + '<div class="teevee-model" aria-hidden="true"><img src="/img/teevee/color-mark.png" alt=""><div>Color Television<small>MODEL 1984</small></div></div>');
            chrome.querySelectorAll('.vd-window-button:not(.vd-window-ai-button)').forEach(button => { button.innerHTML = controlIcon(button.dataset.action); });
        }
        let listObserver = null;
        if (typeof ctx.wireContextMenuBoundary === 'function') ctx.wireContextMenuBoundary(host);

        function renderFilterControls() {
            const countries = countryOptions();
            if (!countries.some(option => option.value === state.countryFilter)) state.countryFilter = ALL_COUNTRIES;
            countrySelect.innerHTML = countries.map(option => `<option value="${esc(option.value)}">${esc(option.label)}</option>`).join('');
            countrySelect.value = state.countryFilter;
            resolutionSelect.innerHTML = resolutionFilters.map(option => `<option value="${esc(option.id)}">${esc(t(option.label, option.fallback))}</option>`).join('');
            resolutionSelect.value = state.resolutionFilter;
        }

        function countryOptions() {
            const seen = state.countries || new Set();
            const codes = Array.from(seen);
            if (!codes.includes(DEFAULT_COUNTRY)) codes.unshift(DEFAULT_COUNTRY);
            codes.sort((a, b) => {
                if (a === DEFAULT_COUNTRY) return -1;
                if (b === DEFAULT_COUNTRY) return 1;
                return countryDisplayName(a).localeCompare(countryDisplayName(b), undefined, { sensitivity: 'base' });
            });
            return [{ value: ALL_COUNTRIES, label: t('desktop.teevee_country_all') }]
                .concat(codes.map(code => ({ value: code, label: countryDisplayName(code) })));
        }

        function renderFilters() {
            filtersEl.innerHTML = filters.map(filter => {
                const active = state.activeFilter === filter.id;
                const count = filter.favorites ? favoriteEntries().length : '';
                return `<button class="teevee-filter ${active ? 'active' : ''}" type="button" data-filter="${esc(filter.id)}" aria-pressed="${active ? 'true' : 'false'}">
                    ${controlIcon(filter.icon)}
                    <span>${esc(t(filter.label, filter.fallback))}</span>
                    ${filter.favorites ? `<em>${esc(count)}</em>` : ''}
                </button>`;
            }).join('');
            filtersEl.querySelectorAll('[data-filter]').forEach(button => {
                button.addEventListener('click', () => {
                    state.activeFilter = button.dataset.filter || 'all';
                    state.visibleLimit = VISIBLE_BATCH;
                    updateVisible();
                    renderAll();
                });
            });
        }

        function renderRecent() {
            const available = state.recent
                .map(id => state.entries.find(entry => entry.id === id))
                .filter(Boolean)
                .slice(0, MAX_RECENT_SHORTCUTS);
            renderShortcutList(recentSection, recentList, available, 'recent');
        }

        function renderFavorites() {
            const available = favoriteEntries().slice(0, MAX_FAVORITE_SHORTCUTS);
            renderShortcutList(favoritesSection, favoritesList, available, 'favorite-shortcut');
        }

        function favoriteEntries() {
            const favoriteIDs = new Set(state.favorites);
            return state.entries
                .filter(entry => favoriteIDs.has(entry.favoriteKey))
                .sort((a, b) => state.favorites.indexOf(a.favoriteKey) - state.favorites.indexOf(b.favoriteKey));
        }

        function renderShortcutList(section, list, entries, dataName) {
            section.hidden = !entries.length;
            list.innerHTML = entries.map(entry => `<button type="button" data-${dataName}="${esc(entry.id)}" title="${esc(entry.name)}">
                <span>${esc(entry.name)}</span>
                <em>${esc(entry.country || '')}</em>
            </button>`).join('');
            list.querySelectorAll(`[data-${dataName}]`).forEach(button => {
                button.addEventListener('click', () => {
                    const entry = state.entries.find(item => item.id === button.getAttribute(`data-${dataName}`));
                    if (entry) playChannel(entry);
                });
            });
        }

        function renderList() {
            const filter = currentFilter();
            const titleParts = [t(filter.label, filter.fallback)];
            if (state.countryFilter !== ALL_COUNTRIES) titleParts.push(countryDisplayName(state.countryFilter));
            if (state.resolutionFilter !== ALL_RESOLUTIONS) titleParts.push(selectedResolutionLabel());
            listTitle.textContent = titleParts.filter(Boolean).join(' | ');
            listCount.textContent = state.loading ? '' : String(state.totalVisible || state.visible.length);
            statusEl.hidden = !state.error && !state.loading;
            statusEl.textContent = state.loading ? t('desktop.teevee_loading') : state.error;
            if (listObserver) {
                listObserver.disconnect();
                listObserver = null;
            }
            if (state.loading) {
                listEl.innerHTML = Array.from({ length: 9 }).map(() => '<article class="teevee-channel teevee-skeleton"><span></span><strong></strong><em></em></article>').join('');
                return;
            }
            if (!state.visible.length) {
                listEl.innerHTML = `<div class="teevee-empty">${iconMarkup('teevee', 'TV', 'teevee-empty-icon', 32)}<strong>${esc(t('desktop.teevee_no_results'))}</strong></div>`;
                return;
            }
            listEl.innerHTML = state.visible.map(channelCard).join('');
            listEl.querySelectorAll('[data-channel-play]').forEach(card => {
                card.addEventListener('click', event => {
                    const entry = state.entries.find(item => item.id === card.dataset.channelId);
                    if (entry) playChannel(entry);
                });
                card.addEventListener('contextmenu', event => {
                    const entry = state.entries.find(item => item.id === card.dataset.channelId);
                    showChannelContextMenu(event, entry);
                });
            });
            listEl.querySelectorAll('img[data-logo]').forEach(img => {
                img.addEventListener('error', () => {
                    img.removeAttribute('src');
                    img.hidden = true;
                }, { once: true });
            });
            listEl.querySelectorAll('[data-action="favorite"]').forEach(btn => {
                btn.addEventListener('click', event => {
                    event.stopPropagation();
                    const entry = state.entries.find(item => item.id === btn.dataset.channelId);
                    if (entry) toggleFavorite(entry);
                });
            });
            if (state.visible.length < Math.min(state.totalVisible, MAX_VISIBLE_CHANNELS)) {
                const sentinel = document.createElement('div');
                sentinel.className = 'teevee-sentinel';
                sentinel.setAttribute('aria-hidden', 'true');
                listEl.appendChild(sentinel);
                listObserver = new IntersectionObserver((entries) => {
                    entries.forEach(entry => {
                        if (entry.isIntersecting) {
                            state.visibleLimit = Math.min(state.visibleLimit + VISIBLE_BATCH, MAX_VISIBLE_CHANNELS);
                            updateVisible();
                            renderList();
                        }
                    });
                }, { root: listEl, rootMargin: '200px 0px' });
                listObserver.observe(sentinel);
            }
        }

        function channelCard(entry) {
            const active = state.current && state.current.id === entry.id;
            const unsupported = entry.unsupported;
            const favorite = isFavorite(entry);
            const meta = [entry.country, entry.language, resolutionText(entry), categoryText(entry)].filter(Boolean).join(' | ');
            return `<article class="teevee-channel ${active ? 'active' : ''} ${unsupported ? 'unsupported' : ''}">
                <button class="teevee-channel-play" type="button" data-channel-play data-channel-id="${esc(entry.id)}" aria-label="${esc(t('desktop.teevee_play'))} ${esc(entry.name)}">
                <span class="teevee-channel-logo">${tvIcon}</span>
                <span class="teevee-channel-body">
                    <strong title="${esc(entry.name)}">${esc(entry.name)}</strong>
                    <span title="${esc(meta)}">${esc(meta)}</span>
                </span></button>
                <div class="teevee-channel-side">
                    ${unsupported ? `<span class="teevee-badge" title="${esc(t('desktop.teevee_unsupported_hint'))}">${esc(t('desktop.teevee_unsupported_badge'))}</span>` : `<span class="teevee-live">${esc(t('desktop.teevee_live'))}</span>`}
                    <button class="teevee-heart ${favorite ? 'active' : ''}" type="button" data-action="favorite" data-channel-id="${esc(entry.id)}" aria-pressed="${favorite}" aria-label="${esc(favorite ? t('desktop.teevee_remove_favorite') : t('desktop.teevee_add_favorite'))} · ${esc(entry.name)}">${controlIcon(favorite ? 'heart-filled' : 'heart')}</button>
                </div>
            </article>`;
        }

        function renderPlayer() {
            if (state.disposed) return;
            const current = state.current;
            const unavailable = state.error && current;
            nowTitle.textContent = current ? current.name : t('desktop.teevee_no_channel');
            nowMeta.textContent = current ? [countryFlag(current.country), current.country, resolutionText(current)].filter(Boolean).join(' | ') : '';
            stateTitle.textContent = !state.powered ? t('desktop.teevee_power_off') : (current ? current.name : t('desktop.teevee_no_channel'));
            stateMeta.textContent = !state.powered ? '' : state.buffering ? t('desktop.teevee_buffering') : unavailable ? state.error : (current ? [t('desktop.teevee_live'), current.country || '', resolutionText(current)].filter(Boolean).join(' | ') : t('desktop.teevee_status_ready'));
            playerState.hidden = !state.powered || (state.playing && !unavailable && !state.buffering);
            root.classList.toggle('is-playing', state.playing);
            root.classList.toggle('is-muted', state.muted);
            toggleBtn.classList.toggle('active', state.playing);
            toggleBtn.setAttribute('aria-label', state.playing ? t('desktop.teevee_pause') : t('desktop.teevee_play'));
            root.classList.toggle('is-powered-off', !state.powered);
            root.classList.toggle('has-reflection', state.appearance.reflection);
            root.classList.toggle('has-crt', state.appearance.crt);
            toggleBtn.disabled = !current || current.unsupported;
            toggleBtn.innerHTML = controlIcon(state.playing ? 'pause' : 'play');
            muteBtn.classList.toggle('active', state.muted);
            muteBtn.setAttribute('aria-label', state.muted ? t('desktop.teevee_unmute') : t('desktop.teevee_mute'));
            favoriteCurrentBtn.classList.toggle('active', current && isFavorite(current));
            favoriteCurrentBtn.disabled = !current;
            favoriteCurrentBtn.setAttribute('aria-pressed', String(!!(current && isFavorite(current))));
            favoriteCurrentBtn.setAttribute('aria-label', current && isFavorite(current) ? t('desktop.teevee_remove_favorite') : t('desktop.teevee_add_favorite'));
            host.querySelector('[data-action="power"]').setAttribute('aria-pressed', String(state.powered));
            host.querySelector('[data-action="stop"]').disabled = !current || !state.powered;
            if (crt) crt.setEnabled(state.appearance.crt && state.powered);
            else playerMount.dataset.crtMode = state.appearance.crt ? 'basic' : 'off';
            setWindowMenus();
        }

        function renderAll() {
            if (state.disposed) return;
            renderFilterControls();
            renderFilters();
            renderFavorites();
            renderRecent();
            renderList();
            renderPlayer();
        }

        function updateVisible() {
            const query = normalizeSearch(state.search);
            const favoriteIDs = new Set(state.favorites);
            let entries = state.entries;
            if (state.countryFilter !== ALL_COUNTRIES) entries = entries.filter(countryMatches);
            if (state.resolutionFilter !== ALL_RESOLUTIONS) entries = entries.filter(resolutionMatches);
            const filter = currentFilter();
            if (filter.favorites) {
                entries = entries.filter(entry => favoriteIDs.has(entry.favoriteKey));
            } else if (filter.category) {
                entries = entries.filter(entry => entry.categories.includes(filter.category));
            }
            if (query) entries = entries.filter(entry => searchableText(entry).includes(query));
            state.totalVisible = entries.length;
            state.visible = entries.slice(0, Math.min(state.visibleLimit, MAX_VISIBLE_CHANNELS));
        }

        function countryMatches(entry) {
            return state.countryFilter === ALL_COUNTRIES || entry.country === state.countryFilter;
        }

        function resolutionMatches(entry) {
            return state.resolutionFilter === ALL_RESOLUTIONS || entry.resolutionBucket === state.resolutionFilter;
        }

        function selectedResolutionLabel() {
            const option = resolutionFilters.find(item => item.id === state.resolutionFilter);
            return option ? t(option.label, option.fallback) : '';
        }

        function migrateFavorites(entries) {
            if (!entries || !entries.length) return;
            const legacyKeys = state.favorites.filter(isLegacyFavoriteKey);
            if (!legacyKeys.length) return;
            const migrated = [];
            const seen = new Set();
            state.favorites.forEach(key => {
                if (isLegacyFavoriteKey(key)) {
                    const entry = entries.find(item => item.url === key);
                    if (entry && !seen.has(entry.favoriteKey)) {
                        migrated.push(entry.favoriteKey);
                        seen.add(entry.favoriteKey);
                    }
                } else if (!seen.has(key)) {
                    migrated.push(key);
                    seen.add(key);
                }
            });
            state.favorites = migrated.slice(0, MAX_FAVORITES);
            saveFavorites(state.favorites);
            try {
                localStorage.removeItem(LEGACY_FAVORITES_KEY);
            } catch (_) {}
            updateVisible();
        }

        async function loadCatalog(force) {
            if (state.disposed) return;
            const request = ++state.catalogRequest;
            state.loading = true;
            state.error = '';
            renderAll();
            try {
                const data = await fetchCatalog(force);
                if (state.disposed || request !== state.catalogRequest) return;
                state.entries = data.entries;
                state.countries = data.countries || new Set();
                state.catalogLoadedAt = data.loadedAt;
                updateVisible();
                migrateFavorites(state.entries);
                renderFilterControls();
            } catch (_) {
                if (state.disposed || request !== state.catalogRequest) return;
                state.error = t('desktop.teevee_catalog_error');
                state.visible = [];
            } finally {
                if (!state.disposed && request === state.catalogRequest) {
                    state.loading = false;
                    renderAll();
                }
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
            if (!entry || state.disposed) return;
            if (!state.current || state.current.id !== entry.id) state.forceProxy = false;
            state.powered = true;
            if (entry.unsupported) {
                resetPlayback();
                state.current = entry;
                state.error = t('desktop.teevee_unsupported_stream');
                showToast(state.error);
                renderPlayer();
                renderList();
                return;
            }
            resetPlayback();
            const playbackID = state.playbackID;
            state.current = entry;
            state.error = '';
            state.buffering = true;
            state.hlsErrorCount = 0;
            renderPlayer();
            try {
                await attachVideoSource(entry, playbackID);
                if (state.playbackID !== playbackID || state.current !== entry) return;
                await video.play();
                if (state.playbackID !== playbackID || state.current !== entry) return;
                state.playing = true;
                state.buffering = false;
                state.error = '';
                rememberRecent(entry);
                updateMediaSession(entry, 'AuraGo TeeVee');
            } catch (err) {
                if (state.playbackID !== playbackID || state.current !== entry) return;
                state.playing = false;
                state.error = formatPlaybackError(err);
                showToast(state.error);
                resetPlayback();
            }
            renderAll();
        }

        async function attachVideoSource(entry, playbackID) {
            const url = entry.url;
            if (!url) throw new Error(t('desktop.teevee_stream_unavailable', STREAM_UNAVAILABLE_FALLBACK));
            if (isHLSURL(url)) {
                if (window.Hls && window.Hls.isSupported && window.Hls.isSupported()) {
                    return attachHlsSource(entry, playbackID);
                }
                throw new Error(t('desktop.teevee_stream_unavailable', STREAM_UNAVAILABLE_FALLBACK));
            }
            const playbackURL = streamPlaybackURL(url, state.forceProxy);
            video.src = playbackURL;
            video.load();
        }

        function attachHlsSource(entry, playbackID) {
            return new Promise((resolve, reject) => {
                let settled = false;

                const isCurrentPlayback = () => (
                    state.playbackID === playbackID &&
                    state.current === entry
                );
                const unavailableError = () => new Error(
                    t('desktop.teevee_stream_unavailable', STREAM_UNAVAILABLE_FALLBACK)
                );
                const fail = () => {
                    if (!isCurrentPlayback()) return;
                    if (!settled) {
                        settled = true;
                        reject(unavailableError());
                        return;
                    }
                    state.playing = false;
                    state.error = t('desktop.teevee_stream_unavailable', STREAM_UNAVAILABLE_FALLBACK);
                    showToast(state.error);
                    resetPlayback();
                    renderAll();
                };
                const loadAttempt = forceProxy => {
                    if (!isCurrentPlayback()) return;
                    destroyHls();
                    state.hlsErrorCount = 0;

                    const useProxy = teeveeUseStreamProxy(entry.url, forceProxy);
                    const resumeWhenReady = settled;
                    const hls = teeveeCreateHls(useProxy);
                    state.hls = hls;
                    hls.on(window.Hls.Events.MANIFEST_PARSED, function () {
                        if (!isCurrentPlayback() || state.hls !== hls) return;
                        if (!settled) {
                            settled = true;
                            resolve();
                            return;
                        }
                        if (resumeWhenReady) {
                            video.play().catch(fail);
                        }
                    });
                    hls.on(window.Hls.Events.ERROR, function (_event, data) {
                        if (!isCurrentPlayback() || state.hls !== hls) return;
                        if (data && data.fatal) {
                            const networkError = (
                                window.Hls.ErrorTypes &&
                                data.type === window.Hls.ErrorTypes.NETWORK_ERROR
                            );
                            if (networkError && !useProxy && teeveeCanProxyStream(entry.url)) {
                                loadAttempt(true);
                                return;
                            }
                            fail();
                            return;
                        }
                        state.hlsErrorCount = (state.hlsErrorCount || 0) + 1;
                        if (state.hlsErrorCount > 5) fail();
                    });
                    hls.attachMedia(video);
                    hls.loadSource(streamPlaybackURL(entry.url, useProxy));
                };

                loadAttempt(state.forceProxy);
            });
        }

        function destroyHls() {
            if (state.hls) {
                try { state.hls.destroy(); } catch (_) {}
                state.hls = null;
            }
        }

        function resetPlayback() {
            if (crt) crt.reset();
            destroyHls();
            state.playbackID = (state.playbackID || 0) + 1;
            video.pause();
            video.removeAttribute('src');
            video.load();
            state.playing = false;
            state.buffering = false;
        }

        function stopPlayback() {
            if (state.disposed) return;
            state.error = '';
            resetPlayback();
            renderAll();
        }

        function togglePower() {
            if (state.disposed) return;
            state.powered = !state.powered;
            if (!state.powered) stopPlayback();
            else if (state.current) playChannel(state.current);
            renderPlayer();
        }

        async function togglePlayback() {
            if (state.disposed || !state.current || state.current.unsupported) return;
            if (!state.powered || (!video.getAttribute('src') && !state.hls)) return playChannel(state.current);
            if (!video.paused) { video.pause(); return; }
            const playbackID = state.playbackID;
            try {
                await video.play();
                if (state.disposed || playbackID !== state.playbackID) return;
                state.playing = true;
                renderPlayer();
            } catch (err) {
                if (state.disposed || playbackID !== state.playbackID) return;
                state.error = formatPlaybackError(err);
                showToast(state.error);
                renderAll();
            }
        }

        function toggleAppearance(key) {
            state.appearance[key] = !state.appearance[key];
            try { localStorage.setItem(APPEARANCE_KEY, JSON.stringify(state.appearance)); } catch (_) {}
            renderPlayer();
        }

        function togglePanel(selector) {
            const panel = host.querySelector(selector);
            panel.hidden = !panel.hidden;
            if (!panel.hidden) panel.querySelector('button,input')?.focus();
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

        const toastApi = createToast(toast);
        const showToast = toastApi.show;

        function currentFilter() {
            return filters.find(filter => filter.id === state.activeFilter) || filters[0];
        }

        function setWindowMenus() {
            if (state.disposed || typeof ctx.setWindowMenus !== 'function') return;
            const canReconnect = state.crtMode === 'basic' && state.current && teeveeCanProxyStream(state.current.url) && !state.forceProxy;
            const menuKey = JSON.stringify([state.appearance, !!state.current, !!state.current?.unsupported, state.muted, state.powered, isFavorite(state.current), !!canReconnect]);
            // Video events must not replace a menu while the user is navigating it.
            if (menuKey === state.menuKey) return;
            state.menuKey = menuKey;
            ctx.setWindowMenus(windowId, [
                {
                    id: 'view',
                    labelKey: 'desktop.menu_view',
                    items: [
                        { id: 'crt', labelKey: 'desktop.teevee_crt_filter', checked: state.appearance.crt, action: () => toggleAppearance('crt') },
                        { id: 'reflection', labelKey: 'desktop.teevee_glass_reflection', checked: state.appearance.reflection, action: () => toggleAppearance('reflection') },
                        { type: 'separator' },
                        { id: 'filters', labelKey: 'desktop.teevee_filters', action: () => root.classList.toggle('show-filters') },
                        { id: 'shortcuts', labelKey: 'desktop.teevee_recent', action: () => togglePanel('[data-shortcuts]') },
                        { id: 'controls', labelKey: 'desktop.teevee_controls', action: () => togglePanel('[data-transport]') },
                        { id: 'refresh', labelKey: 'desktop.teevee_refresh', icon: 'refresh', shortcut: 'F5', action: () => loadCatalog(true) },
                        { id: 'fullscreen', labelKey: 'desktop.teevee_fullscreen', icon: 'maximize', disabled: !state.current, action: requestPlayerFullscreen },
                        ...(canReconnect ? [
                            { type: 'separator' },
                            { id: 'crt-reconnect', labelKey: 'desktop.teevee_crt_reconnect', action: () => { state.forceProxy = true; playChannel(state.current); } }
                        ] : [])
                    ]
                },
                {
                    id: 'playback',
                    labelKey: 'desktop.menu_playback',
                    items: [
                        { id: 'play-pause', labelKey: 'desktop.menu_play_pause', icon: 'video', disabled: !state.current || state.current.unsupported, action: togglePlayback },
                        { id: 'stop', labelKey: 'desktop.teevee_stop', icon: 'stop', disabled: !state.current, action: stopPlayback },
                        { id: 'mute', labelKey: 'desktop.menu_mute', icon: 'audio', checked: state.muted, action: () => muteBtn.click() },
                        { id: 'volume', labelKey: 'desktop.teevee_volume', action: () => togglePanel('[data-transport]') },
                        { id: 'power', labelKey: 'desktop.teevee_power', checked: state.powered, action: togglePower },
                        { id: 'favorite', labelKey: 'desktop.menu_favorite', icon: 'heart', disabled: !state.current, checked: state.current && isFavorite(state.current), action: () => favoriteCurrentBtn.click() }
                    ]
                }
            ]);
        }

        function requestPlayerFullscreen() {
            if (playerShell && playerShell.requestFullscreen) {
                playerShell.requestFullscreen().catch(() => showToast(t('desktop.teevee_fullscreen_error')));
            }
        }

        function formatPlaybackError(err) {
            const message = err && err.message ? err.message : '';
            if (message.includes('timed out')) return t('desktop.teevee_timeout');
            if (message.includes('network')) return t('desktop.teevee_network_error');
            if (message.includes('CORS') || message.includes('cross-origin') || message.includes('cross origin')) {
                return t('desktop.teevee_cors_error');
            }
            if (message.includes('MEDIA_ERR') || message.includes('format') || message.includes('decode')) {
                return t('desktop.teevee_format_error');
            }
            return t('desktop.teevee_stream_unavailable', STREAM_UNAVAILABLE_FALLBACK);
        }

        const searchDebounce = debounce((value) => {
            state.search = value || '';
            state.visibleLimit = VISIBLE_BATCH;
            updateVisible();
            renderList();
        }, SEARCH_DELAY);
        searchInput.addEventListener('input', () => searchDebounce.call(searchInput.value));
        countrySelect.addEventListener('change', () => {
            state.countryFilter = countrySelect.value || ALL_COUNTRIES;
            state.visibleLimit = VISIBLE_BATCH;
            updateVisible();
            renderList();
        });
        resolutionSelect.addEventListener('change', () => {
            state.resolutionFilter = resolutionSelect.value || ALL_RESOLUTIONS;
            state.visibleLimit = VISIBLE_BATCH;
            updateVisible();
            renderList();
        });
        host.querySelector('[data-action="refresh"]').addEventListener('click', () => loadCatalog(true));
        host.querySelector('[data-action="fullscreen"]').addEventListener('click', requestPlayerFullscreen);
        host.querySelector('[data-action="fullscreen-video"]').addEventListener('click', requestPlayerFullscreen);
        playerShell.addEventListener('dblclick', requestPlayerFullscreen);
        host.querySelector('[data-action="stop"]').addEventListener('click', stopPlayback);
        host.querySelector('[data-action="power"]').addEventListener('click', togglePower);
        host.querySelector('[data-action="close-shortcuts"]').addEventListener('click', () => togglePanel('[data-shortcuts]'));
        host.querySelector('[data-action="close-transport"]').addEventListener('click', () => togglePanel('[data-transport]'));
        toggleBtn.addEventListener('click', togglePlayback);
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
            if (state.disposed || !state.powered) { video.pause(); return; }
            state.playing = true;
            state.buffering = false;
            state.error = '';
            renderPlayer();
        });
        video.addEventListener('pause', () => {
            state.playing = false;
            renderPlayer();
        });
        video.addEventListener('error', () => {
            if (state.disposed || !state.powered || !video.getAttribute('src') || !state.current || state.hls) return;
            state.playing = false;
            state.error = t('desktop.teevee_stream_unavailable', STREAM_UNAVAILABLE_FALLBACK);
            showToast(state.error);
            renderAll();
        });
        video.addEventListener('stalled', () => {
            if (state.disposed || !state.powered || !state.current || state.current.unsupported) return;
            state.error = t('desktop.teevee_stream_stalled');
            showToast(state.error);
            renderPlayer();
        });
        video.addEventListener('waiting', () => {
            if (state.disposed || !state.powered || !state.current) return;
            state.buffering = true;
            renderPlayer();
        });
        video.addEventListener('ended', () => { state.playing = false; renderPlayer(); });
        if ('mediaSession' in navigator) {
            try {
                navigator.mediaSession.setActionHandler('play', () => {
                    if (!state.disposed && video.paused) togglePlayback();
                });
                navigator.mediaSession.setActionHandler('pause', () => video.pause());
                navigator.mediaSession.setActionHandler('stop', stopPlayback);
            } catch (_) {}
        }

        root.addEventListener('keydown', event => {
            if (event.key === 'Escape') {
                root.classList.remove('show-filters');
                host.querySelector('[data-transport]').hidden = true;
                host.querySelector('[data-shortcuts]').hidden = true;
                return;
            }
            const target = event.target;
            if (target && (target.closest('button,input,select,textarea') || target.isContentEditable)) return;
            switch (event.key) {
                case ' ':
                    event.preventDefault();
                    togglePlayback();
                    break;
                case 'f':
                case 'F':
                    event.preventDefault();
                    requestPlayerFullscreen();
                    break;
                case 'm':
                case 'M':
                    event.preventDefault();
                    state.muted = !state.muted;
                    video.muted = state.muted;
                    renderPlayer();
                    break;
            }
        });

        disposers.set(windowId, () => {
            state.disposed = true;
            state.catalogRequest++;
            searchDebounce.clear();
            toastApi.clear();
            if (listObserver) listObserver.disconnect();
            if (crt) crt.dispose();
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
            fetchJSON(CHANNELS_ENDPOINT, force ? 'no-store' : 'force-cache'),
            fetchJSON(STREAMS_ENDPOINT, force ? 'no-store' : 'force-cache'),
            fetchJSON(CATEGORIES_ENDPOINT, force ? 'no-store' : 'force-cache')
        ]).then(([channels, streams, categories]) => {
            const joined = joinStreamsWithChannels(channels, streams, categories);
            const data = {
                entries: joined.entries,
                countries: joined.countries,
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

    async function fetchJSON(url, cacheMode) {
        const controller = new AbortController();
        const timeout = setTimeout(() => controller.abort(), 20000);
        try {
            const response = await fetch(url, { cache: cacheMode || 'force-cache', signal: controller.signal });
            clearTimeout(timeout);
            if (!response.ok) throw new Error('iptv-org HTTP');
            return response.json();
        } catch (err) {
            clearTimeout(timeout);
            if (err && err.name === 'AbortError') throw new Error('iptv-org HTTP');
            throw err;
        }
    }

    function joinStreamsWithChannels(channels, streams, categories) {
        const channelByID = new Map((Array.isArray(channels) ? channels : [])
            .filter(channel => channel && channel.id)
            .map(channel => [channel.id, channel]));
        const categoryByID = new Map((Array.isArray(categories) ? categories : [])
            .filter(category => category && category.id)
            .map(category => [category.id, category.name || category.id]));
        const rows = Array.isArray(streams) ? streams : [];
        const countries = new Set();
        const entries = rows.map((stream, index) => {
            if (!stream || !stream.url) return null;
            const channel = channelByID.get(stream.channel || '') || null;
            const channelCategories = Array.isArray(channel && channel.categories) ? channel.categories.map(cleanID).filter(Boolean) : [];
            const country = clean(channel && channel.country).toUpperCase();
            if (/^[A-Z]{2}$/.test(country)) countries.add(country);
            const name = clean(channel && channel.name) || clean(stream.title) || stream.url;
            const entry = {
                id: stableID(stream.channel, stream.url),
                favoriteKey: stableID(stream.channel, stream.url),
                channelID: clean(stream.channel),
                name,
                url: clean(stream.url),
                country,
                language: Array.isArray(channel && channel.languages) ? channel.languages.map(clean).join(', ').toUpperCase() : '',
                categories: channelCategories,
                categoryNames: channelCategories.map(id => categoryByID.get(id) || id),
                quality: clean(stream.quality || stream.label),
                label: clean(stream.label),
                resolutionBucket: resolutionBucketFromStream(stream),
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
        return { entries, countries };
    }

    function stableID(channelID, url) {
        const id = clean(channelID);
        const streamURL = clean(url);
        return (id || 'unknown') + ':' + hashString(streamURL || id || 'stream');
    }

    function isUnsupportedStream(stream) {
        return !!(stream && (stream.user_agent || stream.referrer));
    }

    function sortEntries(a, b) {
        if (a.country === 'DE' && b.country !== 'DE') return -1;
        if (a.country !== 'DE' && b.country === 'DE') return 1;
        return a.name.localeCompare(b.name, undefined, { sensitivity: 'base' });
    }


    function teeveeCanProxyStream(url) {
        if (typeof location === 'undefined' || location.protocol !== 'https:') return false;
        return /^https?:\/\//i.test(teeveeUnwrapStreamURL(url));
    }


    function teeveeUseStreamProxy(url, forceProxy) {
        if (!teeveeCanProxyStream(url)) return false;
        return !!forceProxy || /^http:\/\//i.test(teeveeUnwrapStreamURL(url));
    }


    function teeveeUnwrapStreamURL(url) {
        let raw = clean(url);
        for (let i = 0; i < 8; i++) {
            if (!raw) return '';
            if (raw.indexOf(TEEVEE_STREAM_PROXY + '?') === 0) {
                const inner = new URLSearchParams(raw.slice(raw.indexOf('?') + 1)).get('url');
                if (inner) {
                    raw = inner;
                    continue;
                }
                return raw;
            }
            let parsed;
            try {
                parsed = new URL(raw, typeof location !== 'undefined' ? location.href : 'https://localhost/');
            } catch (_) {
                return raw;
            }
            const path = parsed.pathname || '';
            if (path.indexOf(TEEVEE_STREAM_PROXY) < 0) return raw;
            const inner = parsed.searchParams.get('url');
            if (!inner) return raw;
            raw = inner;
        }
        return raw;
    }

    function teeveeHlsXhrSetup(forceProxy) {
        return function (xhr, url) {
            if (!teeveeUseStreamProxy(url, forceProxy)) return;
            const raw = clean(url);
            if (!raw) return;
            const proxied = streamPlaybackURL(raw, forceProxy);
            if (!proxied || proxied === raw) return;
            xhr.open('GET', proxied, true);
        };
    }

    function teeveeCreateHls(forceProxy) {
        return new window.Hls({
            enableWorker: true,
            lowLatencyMode: true,
            backBufferLength: 60,
            xhrSetup: teeveeHlsXhrSetup(forceProxy)
        });
    }

    function streamPlaybackURL(url, forceProxy) {
        const upstream = teeveeUnwrapStreamURL(url);
        if (!upstream) return '';
        if (teeveeUseStreamProxy(upstream, forceProxy)) {
            return TEEVEE_STREAM_PROXY + '?url=' + encodeURIComponent(upstream);
        }
        return upstream;
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
            entry.resolutionBucket,
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

    function resolutionText(entry) {
        return qualityText(entry) || resolutionFallbackLabel(entry && entry.resolutionBucket);
    }

    function resolutionBucketFromStream(stream) {
        const text = [
            stream && stream.quality,
            stream && stream.label,
            stream && stream.url
        ].map(clean).join(' ').toLowerCase();
        if (/(2160p?|4k|uhd|ultra\s*hd)/.test(text)) return 'uhd';
        if (/(1080p?|fhd|full\s*hd)/.test(text)) return 'fhd';
        if (/(720p?|hd)/.test(text)) return 'hd';
        if (/(576p?|540p?|480p?|360p?|240p?|sd)/.test(text)) return 'sd';
        return 'unknown';
    }

    function resolutionFallbackLabel(bucket) {
        switch (clean(bucket)) {
            case 'uhd': return 'UHD / 4K';
            case 'fhd': return '1080p';
            case 'hd': return '720p';
            case 'sd': return 'SD';
            default: return '';
        }
    }

    function loadFavorites() {
        try {
            const parsed = JSON.parse(localStorage.getItem(FAVORITES_KEY) || '[]');
            if (Array.isArray(parsed) && parsed.length) return parsed.map(clean).filter(Boolean).slice(0, MAX_FAVORITES);
            const legacy = JSON.parse(localStorage.getItem(LEGACY_FAVORITES_KEY) || '[]');
            return Array.isArray(legacy) ? legacy.map(clean).filter(Boolean).slice(0, MAX_FAVORITES) : [];
        } catch (_) {
            return [];
        }
    }

    function loadAppearance() {
        try {
            const value = JSON.parse(localStorage.getItem(APPEARANCE_KEY) || '{}');
            return { crt: value?.crt !== false, reflection: value?.reflection !== false };
        } catch (_) { return { crt: true, reflection: true }; }
    }

    function isLegacyFavoriteKey(key) {
        return /^https?:\/\//.test(clean(key));
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

    window.TeeVeeApp = { render, dispose };
})();
