(function () {
    'use strict';

    const API_BASE = 'https://de1.api.radio-browser.info';
    const FAVORITES_KEY = 'aurago.radio.favorites.v1';
    const SEARCH_DELAY = 300;
    const MAX_FAVORITES = 50;
    const disposers = new Map();
    const categories = [
        { id: 'pop', tag: 'pop', label: 'desktop.radio_pop', fallback: 'Pop' },
        { id: 'rock', tag: 'rock', label: 'desktop.radio_rock', fallback: 'Rock' },
        { id: 'jazz', tag: 'jazz', label: 'desktop.radio_jazz', fallback: 'Jazz & Blues' },
        { id: 'classical', tag: 'classical', label: 'desktop.radio_classical', fallback: 'Classical' },
        { id: 'electronic', tag: 'electronic', label: 'desktop.radio_electronic', fallback: 'Electronic & Dance' },
        { id: 'news', tag: 'news', label: 'desktop.radio_news', fallback: 'News & Talk' }
    ];

    function render(host, windowId, context) {
        if (!host) return;
        const ctx = context || {};
        const esc = ctx.esc || escapeHTML;
        const t = ctx.t || ((key, fallback) => fallback || key);
        const iconMarkup = ctx.iconMarkup || ((key, fallback) => `<span>${esc(fallback || key || '')}</span>`);
        const audio = new Audio();
        audio.preload = 'none';
        audio.crossOrigin = 'anonymous';

        const state = {
            activeCategory: 'pop',
            loading: false,
            search: '',
            stations: [],
            cache: new Map(),
            favorites: loadFavorites(),
            current: null,
            playing: false,
            muted: false,
            volume: 0.72,
            error: ''
        };
        audio.volume = state.volume;

        host.innerHTML = `<div class="radio-app" data-radio-app="${esc(windowId)}">
            <div class="radio-scroll">
                <div class="radio-hero">
                    <div>
                        <div class="radio-kicker">${esc(t('desktop.radio_top_stations', 'Popular'))}</div>
                        <h2>${esc(t('desktop.app_radio', 'Radio'))}</h2>
                    </div>
                    <label class="radio-search">
                        ${iconMarkup('search', 'S', 'radio-search-icon', 15)}
                        <input type="search" data-search autocomplete="off" spellcheck="false" placeholder="${esc(t('desktop.radio_search_placeholder', 'Search stations...'))}">
                    </label>
                </div>
                <div class="radio-tabs" role="tablist" aria-label="${esc(t('desktop.radio_categories', 'Categories'))}" data-tabs></div>
                <div class="radio-status" data-status hidden></div>
                <div class="radio-grid" data-grid></div>
            </div>
            <div class="radio-player" data-player>
                <button class="radio-player-button" type="button" data-action="toggle" aria-label="${esc(t('desktop.radio_play', 'Play'))}">${iconMarkup('audio', 'A', 'radio-button-icon', 18)}</button>
                <div class="radio-now">
                    <span class="radio-now-label">${esc(t('desktop.radio_now_playing', 'Now Playing'))}</span>
                    <strong data-now-title>${esc(t('desktop.radio_no_station', 'No station selected'))}</strong>
                    <span data-now-meta></span>
                </div>
                <button class="radio-icon-button" type="button" data-action="favorite-current" aria-label="${esc(t('desktop.radio_add_favorite', 'Add to favorites'))}">♡</button>
                <button class="radio-icon-button" type="button" data-action="mute" aria-label="${esc(t('desktop.radio_mute', 'Mute'))}">${iconMarkup('audio', 'V', 'radio-button-icon', 16)}</button>
                <input class="radio-volume" type="range" min="0" max="100" value="72" data-volume aria-label="${esc(t('desktop.radio_volume', 'Volume'))}">
                <button class="radio-icon-button" type="button" data-action="stop" aria-label="${esc(t('desktop.radio_stop', 'Stop'))}">${iconMarkup('stop', 'S', 'radio-button-icon', 16)}</button>
            </div>
            <div class="radio-toast" data-toast hidden></div>
        </div>`;

        const searchInput = host.querySelector('[data-search]');
        const tabs = host.querySelector('[data-tabs]');
        const grid = host.querySelector('[data-grid]');
        const status = host.querySelector('[data-status]');
        const toast = host.querySelector('[data-toast]');
        const nowTitle = host.querySelector('[data-now-title]');
        const nowMeta = host.querySelector('[data-now-meta]');
        const toggleBtn = host.querySelector('[data-action="toggle"]');
        const muteBtn = host.querySelector('[data-action="mute"]');
        const favoriteCurrentBtn = host.querySelector('[data-action="favorite-current"]');
        const volume = host.querySelector('[data-volume]');
        let searchTimer = 0;
        let toastTimer = 0;

        function categoryList() {
            const list = [];
            if (state.favorites.length) list.push({ id: 'favorites', label: 'desktop.radio_favorites', fallback: 'Favorites' });
            return list.concat(categories);
        }

        function renderTabs() {
            tabs.innerHTML = categoryList().map(cat => `<button class="radio-tab ${state.activeCategory === cat.id ? 'active' : ''}" type="button" role="tab" aria-selected="${state.activeCategory === cat.id ? 'true' : 'false'}" data-category="${esc(cat.id)}">${esc(t(cat.label, cat.fallback))}</button>`).join('');
            tabs.querySelectorAll('[data-category]').forEach(btn => {
                btn.addEventListener('click', () => switchCategory(btn.dataset.category));
            });
        }

        function renderGrid() {
            if (state.loading) {
                status.hidden = false;
                status.textContent = t('desktop.radio_loading', 'Loading stations...');
                grid.innerHTML = Array.from({ length: 8 }).map(() => '<article class="radio-card radio-skeleton"><div></div><span></span><p></p><em></em></article>').join('');
                return;
            }
            status.hidden = !state.error;
            status.textContent = state.error || '';
            if (!state.stations.length) {
                grid.innerHTML = `<div class="radio-empty">${iconMarkup('audio', 'R', 'radio-empty-icon', 34)}<strong>${esc(t('desktop.radio_no_results', 'No stations found'))}</strong></div>`;
                return;
            }
            grid.innerHTML = state.stations.map(stationCard).join('');
            grid.querySelectorAll('[data-play-station]').forEach(card => {
                card.addEventListener('click', event => {
                    if (event.target.closest('[data-action="favorite"]')) return;
                    const station = findStation(card.dataset.playStation);
                    if (station) playStation(station);
                });
                card.addEventListener('keydown', event => {
                    if (event.key === 'Enter' || event.key === ' ') {
                        event.preventDefault();
                        const station = findStation(card.dataset.playStation);
                        if (station) playStation(station);
                    }
                });
            });
            grid.querySelectorAll('[data-action="favorite"]').forEach(btn => {
                btn.addEventListener('click', event => {
                    event.stopPropagation();
                    const station = findStation(btn.dataset.stationId);
                    if (station) toggleFavorite(station);
                });
            });
            grid.querySelectorAll('img[data-favicon]').forEach(img => {
                img.addEventListener('error', () => {
                    img.removeAttribute('src');
                    img.hidden = true;
                    const fallback = img.parentElement && img.parentElement.querySelector('[data-favicon-fallback]');
                    if (fallback) fallback.hidden = false;
                }, { once: true });
            });
        }

        function stationCard(station) {
            const id = station.stationuuid || station.url_resolved || station.name;
            const active = state.current && stationKey(state.current) === stationKey(station);
            const favorite = isFavorite(station);
            const bitrate = station.bitrate ? t('desktop.radio_kbps', '{{bitrate}} kbps').replace('{{bitrate}}', station.bitrate) : '';
            const clicks = station.clickcount ? t('desktop.radio_listeners', '{{count}} clicks').replace('{{count}}', compactNumber(station.clickcount)) : '';
            const country = countryFlag(station.countrycode) || clean(station.countrycode || '');
            const favicon = clean(station.favicon || '');
            return `<article class="radio-card ${active ? 'active' : ''}" role="button" tabindex="0" data-play-station="${esc(id)}" aria-label="${esc(t('desktop.radio_play', 'Play'))} ${esc(station.name || '')}">
                <div class="radio-art">
                    ${favicon ? `<img data-favicon src="${esc(favicon)}" alt="">` : ''}
                    <span data-favicon-fallback ${favicon ? 'hidden' : ''}>${iconMarkup('audio', 'R', 'radio-card-icon', 26)}</span>
                    <span class="radio-play-overlay">${iconMarkup('audio', 'P', 'radio-play-icon', 22)}</span>
                </div>
                <div class="radio-card-body">
                    <h3 title="${esc(station.name || '')}">${esc(station.name || t('desktop.app_radio', 'Radio'))}</h3>
                    <p>${esc([country, station.codec, bitrate].filter(Boolean).join(' · '))}</p>
                    <div class="radio-card-meta"><span>${esc(clicks || t('desktop.radio_top_stations', 'Popular'))}</span><button class="radio-heart ${favorite ? 'active' : ''}" type="button" data-action="favorite" data-station-id="${esc(id)}" aria-label="${esc(favorite ? t('desktop.radio_remove_favorite', 'Remove from favorites') : t('desktop.radio_add_favorite', 'Add to favorites'))}">${favorite ? '♥' : '♡'}</button></div>
                </div>
            </article>`;
        }

        function findStation(id) {
            return state.stations.find(station => (station.stationuuid || station.url_resolved || station.name) === id)
                || state.favorites.find(station => (station.stationuuid || station.url_resolved || station.name) === id);
        }

        function switchCategory(category) {
            state.activeCategory = category;
            state.search = '';
            searchInput.value = '';
            renderTabs();
            loadActive();
        }

        async function loadActive() {
            state.loading = true;
            state.error = '';
            renderGrid();
            try {
                if (state.activeCategory === 'favorites') {
                    state.stations = state.favorites.slice();
                } else {
                    const cat = categories.find(item => item.id === state.activeCategory) || categories[0];
                    const cacheKey = 'tag:' + cat.tag;
                    state.stations = state.cache.get(cacheKey) || await fetchStations(`/json/stations/bytag/${encodeURIComponent(cat.tag)}?order=clickcount&limit=20&hidebroken=true`);
                    state.cache.set(cacheKey, state.stations);
                }
            } catch (err) {
                state.error = err.message || String(err);
                state.stations = [];
                showToast(state.error);
            } finally {
                state.loading = false;
                renderGrid();
            }
        }

        async function searchStations(query) {
            state.search = query.trim();
            if (!state.search) {
                loadActive();
                return;
            }
            state.loading = true;
            state.error = '';
            renderGrid();
            try {
                state.stations = await fetchStations(`/json/stations/search?name=${encodeURIComponent(state.search)}&order=clickcount&reverse=true&limit=30&hidebroken=true`);
            } catch (err) {
                state.error = err.message || String(err);
                state.stations = [];
                showToast(state.error);
            } finally {
                state.loading = false;
                renderGrid();
            }
        }

        async function playStation(station) {
            const url = await resolveStreamURL(station).catch(() => station.url_resolved || station.url);
            if (!url) {
                showToast(t('desktop.radio_error', 'Failed to load stream'));
                return;
            }
            state.current = station;
            state.playing = false;
            updatePlayer();
            try {
                audio.src = url;
                await audio.play();
                state.playing = true;
                updateMediaSession(station);
            } catch (err) {
                state.playing = false;
                showToast(err.message || t('desktop.radio_error', 'Failed to load stream'));
            }
            updatePlayer();
            renderGrid();
        }

        async function resolveStreamURL(station) {
            if (!station.stationuuid) return station.url_resolved || station.url;
            const body = await fetchJSON(`/json/url/${encodeURIComponent(station.stationuuid)}`);
            return body && (body.url || body.url_resolved) || station.url_resolved || station.url;
        }

        function updatePlayer() {
            const current = state.current;
            nowTitle.textContent = current ? current.name : t('desktop.radio_no_station', 'No station selected');
            nowMeta.textContent = current ? [clean(current.countrycode || ''), clean(current.codec || ''), current.bitrate ? t('desktop.radio_kbps', '{{bitrate}} kbps').replace('{{bitrate}}', current.bitrate) : ''].filter(Boolean).join(' · ') : '';
            toggleBtn.classList.toggle('active', state.playing);
            toggleBtn.setAttribute('aria-label', state.playing ? t('desktop.radio_pause', 'Pause') : t('desktop.radio_play', 'Play'));
            toggleBtn.innerHTML = iconMarkup(state.playing ? 'stop' : 'audio', state.playing ? 'P' : 'A', 'radio-button-icon', 18);
            muteBtn.classList.toggle('active', state.muted);
            muteBtn.setAttribute('aria-label', state.muted ? t('desktop.radio_unmute', 'Unmute') : t('desktop.radio_mute', 'Mute'));
            favoriteCurrentBtn.classList.toggle('active', current && isFavorite(current));
            favoriteCurrentBtn.textContent = current && isFavorite(current) ? '♥' : '♡';
        }

        function toggleFavorite(station) {
            const key = stationKey(station);
            const index = state.favorites.findIndex(item => stationKey(item) === key);
            if (index >= 0) {
                state.favorites.splice(index, 1);
            } else {
                state.favorites.unshift(favoritePayload(station));
                if (state.favorites.length > MAX_FAVORITES) state.favorites.length = MAX_FAVORITES;
            }
            saveFavorites(state.favorites);
            if (state.activeCategory === 'favorites') state.stations = state.favorites.slice();
            if (state.activeCategory === 'favorites' && !state.favorites.length) state.activeCategory = 'pop';
            renderTabs();
            renderGrid();
            updatePlayer();
        }

        function isFavorite(station) {
            const key = stationKey(station);
            return state.favorites.some(item => stationKey(item) === key);
        }

        function showToast(message) {
            toast.textContent = message || t('desktop.radio_error', 'Failed to load stream');
            toast.hidden = false;
            clearTimeout(toastTimer);
            toastTimer = setTimeout(() => { toast.hidden = true; }, 3600);
        }

        function stopPlayback() {
            audio.pause();
            audio.removeAttribute('src');
            audio.load();
            state.playing = false;
            updatePlayer();
            renderGrid();
        }

        searchInput.addEventListener('input', () => {
            clearTimeout(searchTimer);
            searchTimer = setTimeout(() => searchStations(searchInput.value), SEARCH_DELAY);
        });
        toggleBtn.addEventListener('click', () => {
            if (!state.current) return;
            if (audio.paused) {
                audio.play().then(() => { state.playing = true; updatePlayer(); }).catch(err => showToast(err.message || String(err)));
            } else {
                audio.pause();
                state.playing = false;
                updatePlayer();
            }
        });
        host.querySelector('[data-action="stop"]').addEventListener('click', stopPlayback);
        muteBtn.addEventListener('click', () => {
            state.muted = !state.muted;
            audio.muted = state.muted;
            updatePlayer();
        });
        favoriteCurrentBtn.addEventListener('click', () => {
            if (state.current) toggleFavorite(state.current);
        });
        volume.addEventListener('input', () => {
            state.volume = Number(volume.value) / 100;
            audio.volume = state.volume;
        });
        audio.addEventListener('pause', () => { state.playing = false; updatePlayer(); });
        audio.addEventListener('playing', () => { state.playing = true; updatePlayer(); });
        audio.addEventListener('error', () => {
            state.playing = false;
            showToast(t('desktop.radio_error', 'Failed to load stream'));
            updatePlayer();
        });
        audio.addEventListener('stalled', () => showToast(t('desktop.radio_error', 'Failed to load stream')));
        disposers.set(windowId, () => {
            clearTimeout(searchTimer);
            clearTimeout(toastTimer);
            stopPlayback();
        });
        if ('mediaSession' in navigator) {
            try {
                navigator.mediaSession.setActionHandler('play', () => {
                    if (state.current) audio.play().catch(() => {});
                });
                navigator.mediaSession.setActionHandler('pause', () => audio.pause());
                navigator.mediaSession.setActionHandler('stop', stopPlayback);
            } catch (_) {}
        }

        renderTabs();
        loadActive();
    }

    function dispose(windowId) {
        const cleanup = disposers.get(windowId);
        if (!cleanup) return;
        cleanup();
        disposers.delete(windowId);
    }

    async function fetchStations(path) {
        const list = await fetchJSON(path);
        return Array.isArray(list) ? list.filter(station => station && station.name && (station.url_resolved || station.url)).map(normalizeStation) : [];
    }

    async function fetchJSON(path) {
        const response = await fetch(API_BASE + path, { cache: 'no-store' });
        if (!response.ok) throw new Error('Radio Browser HTTP ' + response.status);
        return response.json();
    }

    function normalizeStation(station) {
        return {
            stationuuid: clean(station.stationuuid),
            name: clean(station.name),
            url: clean(station.url),
            url_resolved: clean(station.url_resolved || station.url),
            favicon: clean(station.favicon),
            codec: clean(station.codec).toUpperCase(),
            bitrate: Number(station.bitrate) || 0,
            countrycode: clean(station.countrycode).toUpperCase(),
            clickcount: Number(station.clickcount) || 0,
            votes: Number(station.votes) || 0
        };
    }

    function favoritePayload(station) {
        const normalized = normalizeStation(station);
        return {
            stationuuid: normalized.stationuuid,
            name: normalized.name,
            url_resolved: normalized.url_resolved || normalized.url,
            favicon: normalized.favicon,
            codec: normalized.codec,
            bitrate: normalized.bitrate,
            countrycode: normalized.countrycode,
            clickcount: normalized.clickcount
        };
    }

    function stationKey(station) {
        return clean(station && (station.stationuuid || station.url_resolved || station.url || station.name));
    }

    function loadFavorites() {
        try {
            const parsed = JSON.parse(localStorage.getItem(FAVORITES_KEY) || '[]');
            return Array.isArray(parsed) ? parsed.map(normalizeStation).filter(station => station.name && station.url_resolved).slice(0, MAX_FAVORITES) : [];
        } catch (_) {
            return [];
        }
    }

    function saveFavorites(favorites) {
        try {
            localStorage.setItem(FAVORITES_KEY, JSON.stringify(favorites.map(favoritePayload).slice(0, MAX_FAVORITES)));
        } catch (_) {}
    }

    function updateMediaSession(station) {
        if (!('mediaSession' in navigator) || !station) return;
        try {
            navigator.mediaSession.metadata = new MediaMetadata({
                title: station.name || 'Radio',
                artist: station.countrycode || '',
                album: 'AuraGo Radio',
                artwork: station.favicon ? [{ src: station.favicon, sizes: '96x96', type: 'image/png' }] : []
            });
        } catch (_) {}
    }

    function countryFlag(code) {
        const value = clean(code).toUpperCase();
        if (!/^[A-Z]{2}$/.test(value)) return '';
        return String.fromCodePoint.apply(String, value.split('').map(ch => 0x1F1E6 + ch.charCodeAt(0) - 65));
    }

    function compactNumber(value) {
        const n = Number(value) || 0;
        if (n >= 1000000) return (n / 1000000).toFixed(1).replace(/\.0$/, '') + 'M';
        if (n >= 1000) return (n / 1000).toFixed(1).replace(/\.0$/, '') + 'K';
        return String(n);
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

    window.RadioApp = { render, dispose };
})();
