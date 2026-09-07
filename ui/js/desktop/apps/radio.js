(function () {
    'use strict';

    const API_BASE = '/api/radio-browser';
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
        const radioIcon = (key, cls = '') => `<img class="radio-icon ${cls}" src="/img/papirus/icons/${key}.svg" alt="" draggable="false">`;
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
        let disposed = false;
        let catalogRequest = null;
        let streamRequest = null;
        let catalogRevision = 0;
        let playbackRevision = 0;
        let selectedIndex = 0;
        const listeners = new AbortController();
        audio.volume = state.volume;

        host.innerHTML = `<div class="radio-app" data-radio-app="${esc(windowId)}">
                <div class="radio-hero">
                    <div class="radio-brand">
                        <div class="radio-kicker">FM / INTERNET</div>
                        <h2>${esc(t('desktop.app_radio'))}</h2>
                        <div class="radio-brand-motto">MUSIC CONNECTS PEOPLE</div>
                    </div>
                    <div class="radio-tuner">
                        <div class="radio-dial" aria-hidden="true">
                            <div class="radio-lamps"><span>STEREO <i data-live-lamp></i></span><span>ON AIR <i data-live-lamp></i></span></div>
                            <div class="radio-scale"><div class="radio-frequencies"><span>FM</span><span>88</span><span>92</span><span>96</span><span>100</span><span>104</span><span>108</span><span>MHz</span></div><div class="radio-ticks"></div><div class="radio-scale-web"><span>WEB</span><span>STATIONS</span></div><div class="radio-needle" data-needle></div></div>
                        </div>
                    <label class="radio-search">
                        ${radioIcon('search')}
                        <input type="search" data-search aria-label="${esc(t('desktop.radio_search_placeholder'))}" autocomplete="off" spellcheck="false" placeholder="${esc(t('desktop.radio_search_placeholder'))}" inputmode="search" enterkeyhint="search" autocapitalize="off">
                    </label>
                    </div>
                    <div class="radio-tuning-control"><button class="radio-knob" type="button" data-tune aria-label="${esc(t('desktop.radio_tune'))}" aria-describedby="radio-tune-${esc(windowId)}"><img src="/img/radio/knob.png" alt="" draggable="false"></button><span class="radio-tuning-caption" aria-hidden="true">TUNING</span><span class="radio-sr-only" id="radio-tune-${esc(windowId)}">${esc(t('desktop.radio_tune_hint'))}</span></div>
                </div>
                <div class="radio-presets"><div class="radio-tabs" role="group" aria-label="${esc(t('desktop.radio_categories'))}" data-tabs></div><div class="radio-preset-label" aria-hidden="true">PRESETS<small>GOOD MUSIC<br>ALWAYS ON <i></i></small></div></div>
            <div class="radio-scroll">
                <div class="radio-status" data-status role="status" hidden></div>
                <div class="radio-grid" data-grid></div>
            </div>
            <div class="radio-player" data-player>
                <div class="radio-player-display">
                <button class="radio-player-button" type="button" data-action="toggle" aria-label="${esc(t('desktop.radio_play'))}"><img class="radio-playback-art" src="/img/radio/music-note.png" alt="" draggable="false"></button>
                <div class="radio-now">
                    <span class="radio-now-label">${esc(t('desktop.radio_now_playing'))}</span>
                    <strong data-now-title>${esc(t('desktop.radio_no_station'))}</strong>
                    <span data-now-meta></span>
                    <div class="radio-level" aria-hidden="true">${'<i></i>'.repeat(24)}</div>
                </div>
                <div class="radio-player-lamps" aria-hidden="true"><span><i data-live-lamp></i> STEREO</span><span><i></i> FM</span><span><i data-live-lamp></i> WEB</span><span><i></i> HI-FI</span></div>
                </div>
                <div class="radio-player-controls">
                <button class="radio-icon-button" type="button" data-action="favorite-current" aria-label="${esc(t('desktop.radio_add_favorite'))}">${radioIcon('heart')}</button>
                <button class="radio-icon-button" type="button" data-action="mute" aria-label="${esc(t('desktop.radio_mute'))}">${radioIcon('audio')}</button>
                <label class="radio-volume-label"><span>${esc(t('desktop.radio_volume'))}</span><input class="radio-volume" type="range" min="0" max="100" value="72" data-volume><span class="radio-volume-extents" aria-hidden="true"><span>MIN</span><span>MAX</span></span></label>
                <button class="radio-icon-button" type="button" data-action="stop" aria-label="${esc(t('desktop.radio_stop'))}">${radioIcon('stop')}</button>
                </div>
                <div class="radio-footer" aria-hidden="true"><span>HIGH FIDELITY INTERNET RADIO</span><img src="/img/radio/signature.png" alt=""></div>
            </div>
            <div class="radio-toast" data-toast role="status" hidden></div>
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
        const tuner = host.querySelector('[data-tune]');
        let searchTimer = 0;
        let toastTimer = 0;
        const showContextMenu = typeof ctx.showContextMenu === 'function' ? ctx.showContextMenu : null;
        function showStationContextMenu(event, station) {
            if (!showContextMenu || !station) return false;
            event.preventDefault();
            showContextMenu(event.clientX, event.clientY, [
                { labelKey: 'desktop.radio_play', icon: 'audio', action: () => playStation(station) },
                { labelKey: 'desktop.menu_favorite', icon: 'heart', checked: isFavorite(station), action: () => toggleFavorite(station) },
                { type: 'separator' },
                { labelKey: 'desktop.context_refresh', icon: 'refresh', action: () => loadActive() }
            ]);
            return true;
        }
        if (typeof ctx.wireContextMenuBoundary === 'function') ctx.wireContextMenuBoundary(host);

        function categoryList() {
            const list = [{ id: 'favorites', label: 'desktop.radio_favorites', fallback: 'Favorites' }];
            return list.concat(categories);
        }

        function renderTabs() {
            tabs.innerHTML = categoryList().map(cat => `<button class="radio-tab ${state.activeCategory === cat.id ? 'active' : ''}" type="button" aria-pressed="${state.activeCategory === cat.id}" data-category="${esc(cat.id)}">${esc(t(cat.label, cat.fallback))}</button>`).join('');
            tabs.querySelectorAll('[data-category]').forEach(btn => {
                btn.addEventListener('click', () => switchCategory(btn.dataset.category));
            });
        }

        function renderGrid() {
            if (disposed) return;
            tuner.disabled = state.loading || !state.stations.length;
            if (state.loading) {
                status.hidden = false;
                status.textContent = t('desktop.radio_loading');
                grid.innerHTML = Array.from({ length: 6 }).map(() => '<article class="radio-card radio-skeleton" aria-hidden="true"></article>').join('');
                return;
            }
            status.hidden = !state.error;
            status.textContent = state.error || '';
            if (!state.stations.length) {
                grid.innerHTML = `<div class="radio-empty">${radioIcon('radio')}<strong>${esc(t(state.activeCategory === 'favorites' && !state.search ? 'desktop.radio_empty_favorites' : 'desktop.radio_no_results'))}</strong></div>`;
                updateTuner();
                return;
            }
            grid.innerHTML = state.stations.map(stationCard).join('');
            grid.querySelectorAll('[data-play-station]').forEach(card => {
                card.addEventListener('click', event => {
                    if (event.target.closest('[data-action="favorite"]')) return;
                    const station = findStation(card.dataset.playStation);
                    if (station) playStation(station);
                });
                card.addEventListener('contextmenu', event => {
                    const station = findStation(card.dataset.playStation);
                    showStationContextMenu(event, station);
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
            updateTuner();
        }

        function stationCard(station, index) {
            const id = station.stationuuid || station.url_resolved || station.name;
            const active = state.current && stationKey(state.current) === stationKey(station);
            const favorite = isFavorite(station);
            const bitrate = station.bitrate ? t('desktop.radio_kbps').replace('{{bitrate}}', station.bitrate) : '';
            const clicks = station.clickcount ? t('desktop.radio_listeners').replace('{{count}}', compactNumber(station.clickcount, t)) : '';
            const country = clean(station.countrycode || '');
            const favicon = clean(station.favicon || '');
            return `<article class="radio-card ${active ? 'active' : ''} ${active && state.playing ? 'playing' : ''}">
                <span class="radio-card-number" aria-hidden="true">${String(index + 1).padStart(2, '0')}</span>
                <button type="button" class="radio-station" data-play-station="${esc(id)}" aria-label="${esc(t('desktop.radio_play'))} ${esc(station.name || '')}">
                <span class="radio-art">
                    ${favicon ? `<img data-favicon src="${esc(favicon)}" alt="">` : ''}
                    <span data-favicon-fallback ${favicon ? 'hidden' : ''}>${radioIcon('radio')}</span>
                </span>
                <span class="radio-card-body">
                    <strong class="radio-station-name" title="${esc(station.name || '')}">${esc(station.name || t('desktop.app_radio'))}</strong>
                    <span class="radio-station-meta">${esc([country, station.codec, bitrate].filter(Boolean).join(' · '))}</span>
                    <span class="radio-level" aria-hidden="true">${'<i></i>'.repeat(20)}</span>
                    <span class="radio-card-meta" title="${esc(clicks)}">${esc(t('desktop.radio_popular'))}</span>
                </span>
                </button>
                <button class="radio-heart ${favorite ? 'active' : ''}" type="button" data-action="favorite" data-station-id="${esc(id)}" aria-pressed="${favorite}" aria-label="${esc(favorite ? t('desktop.radio_remove_favorite') : t('desktop.radio_add_favorite'))}">${radioIcon('heart')}</button>
            </article>`;
        }

        function findStation(id) {
            return state.stations.find(station => (station.stationuuid || station.url_resolved || station.name) === id)
                || state.favorites.find(station => (station.stationuuid || station.url_resolved || station.name) === id);
        }

        function switchCategory(category) {
            clearTimeout(searchTimer);
            state.activeCategory = category;
            state.search = '';
            searchInput.value = '';
            renderTabs();
            loadActive();
        }

        function setWindowMenus() {
            if (typeof ctx.setWindowMenus !== 'function') return;
            ctx.setWindowMenus(windowId, [
                {
                    id: 'view',
                    labelKey: 'desktop.menu_view',
                    items: [
                        { id: 'refresh', labelKey: 'desktop.context_refresh', icon: 'refresh', shortcut: 'F5', action: () => loadActive() }
                    ]
                },
                {
                    id: 'playback',
                    labelKey: 'desktop.menu_playback',
                    items: [
                        { id: 'play-pause', labelKey: 'desktop.menu_play_pause', icon: 'audio', disabled: !state.current, action: () => toggleBtn.click() },
                        { id: 'stop', labelKey: 'desktop.radio_stop', icon: 'stop', disabled: !state.current, action: stopPlayback },
                        { id: 'mute', labelKey: 'desktop.menu_mute', icon: 'audio', checked: state.muted, action: () => muteBtn.click() },
                        { id: 'favorite', labelKey: 'desktop.menu_favorite', icon: 'heart', disabled: !state.current, checked: state.current && isFavorite(state.current), action: () => favoriteCurrentBtn.click() }
                    ]
                }
            ]);
        }

        async function loadActive() {
            return loadCatalog('');
        }

        async function searchStations(query) {
            state.search = query.trim();
            return loadCatalog(state.search);
        }

        async function loadCatalog(query) {
            const revision = ++catalogRevision;
            if (catalogRequest) catalogRequest.abort();
            catalogRequest = new AbortController();
            state.loading = true;
            state.error = '';
            renderGrid();
            try {
                let stations;
                if (query) {
                    stations = await fetchStations(`/json/stations/search?name=${encodeURIComponent(query)}&order=clickcount&reverse=true&limit=30&hidebroken=true`, catalogRequest.signal);
                } else if (state.activeCategory === 'favorites') {
                    stations = state.favorites.slice();
                } else {
                    const cat = categories.find(item => item.id === state.activeCategory) || categories[0];
                    const cacheKey = 'tag:' + cat.tag;
                    stations = state.cache.get(cacheKey) || await fetchStations(`/json/stations/bytag/${encodeURIComponent(cat.tag)}?order=clickcount&limit=20&hidebroken=true`, catalogRequest.signal);
                    if (!disposed && revision === catalogRevision) state.cache.set(cacheKey, stations);
                }
                if (disposed || revision !== catalogRevision) return;
                state.stations = stations;
                selectedIndex = 0;
            } catch (_) {
                if (disposed || revision !== catalogRevision) return;
                state.error = t('desktop.radio_catalog_error');
                state.stations = [];
                showToast(state.error);
            } finally {
                if (!disposed && revision === catalogRevision) {
                    state.loading = false;
                    renderGrid();
                }
            }
        }

        async function playStation(station) {
            if (disposed) return;
            const revision = ++playbackRevision;
            if (streamRequest) streamRequest.abort();
            streamRequest = new AbortController();
            audio.pause();
            const url = await resolveStreamURL(station, streamRequest.signal).catch(() => station.url_resolved || station.url);
            if (disposed || revision !== playbackRevision) return;
            if (!url) {
                showToast(t('desktop.radio_error'));
                return;
            }
            state.current = station;
            const index = state.stations.findIndex(item => stationKey(item) === stationKey(station));
            if (index >= 0) selectedIndex = index;
            state.playing = false;
            updatePlayer();
            try {
                audio.src = url;
                await audio.play();
                if (disposed || revision !== playbackRevision) return;
                state.playing = true;
                updateMediaSession(station, t);
            } catch (_) {
                if (disposed || revision !== playbackRevision) return;
                state.playing = false;
                showToast(t('desktop.radio_error'));
            }
            updatePlayer();
            renderGrid();
        }

        async function resolveStreamURL(station, signal) {
            if (!station.stationuuid) return station.url_resolved || station.url;
            const body = await fetchJSON(`/json/url/${encodeURIComponent(station.stationuuid)}`, signal);
            return body && (body.url || body.url_resolved) || station.url_resolved || station.url;
        }

        function updatePlayer() {
            if (disposed) return;
            const current = state.current;
            nowTitle.textContent = current ? current.name : t('desktop.radio_no_station');
            nowMeta.textContent = current ? [clean(current.countrycode || ''), clean(current.codec || ''), current.bitrate ? t('desktop.radio_kbps').replace('{{bitrate}}', current.bitrate) : ''].filter(Boolean).join(' · ') : '';
            toggleBtn.classList.toggle('active', state.playing);
            toggleBtn.setAttribute('aria-label', state.playing ? t('desktop.radio_pause') : t('desktop.radio_play'));
            toggleBtn.setAttribute('title', state.playing ? t('desktop.radio_pause') : t('desktop.radio_play'));
            toggleBtn.setAttribute('aria-pressed', String(state.playing));
            toggleBtn.disabled = !current;
            muteBtn.classList.toggle('active', state.muted);
            muteBtn.setAttribute('aria-label', state.muted ? t('desktop.radio_unmute') : t('desktop.radio_mute'));
            favoriteCurrentBtn.classList.toggle('active', current && isFavorite(current));
            favoriteCurrentBtn.disabled = !current;
            favoriteCurrentBtn.setAttribute('aria-pressed', String(!!current && isFavorite(current)));
            favoriteCurrentBtn.setAttribute('aria-label', t(current && isFavorite(current) ? 'desktop.radio_remove_favorite' : 'desktop.radio_add_favorite'));
            muteBtn.setAttribute('aria-pressed', String(state.muted));
            host.querySelector('.radio-app').classList.toggle('playing', state.playing);
            host.querySelectorAll('[data-live-lamp]').forEach(lamp => lamp.classList.toggle('lit', state.playing));
            grid.querySelectorAll('.radio-card').forEach(card => {
                const button = card.querySelector('[data-play-station]');
                card.classList.toggle('playing', !!button && state.playing && !!current && button.dataset.playStation === stationKey(current));
            });
            setWindowMenus();
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
            if (state.activeCategory === 'favorites' && !state.search) state.stations = state.favorites.slice();
            renderTabs();
            renderGrid();
            updatePlayer();
        }

        function isFavorite(station) {
            const key = stationKey(station);
            return state.favorites.some(item => stationKey(item) === key);
        }

        function showToast(message) {
            if (disposed) return;
            toast.textContent = message || t('desktop.radio_error');
            toast.hidden = false;
            clearTimeout(toastTimer);
            toastTimer = setTimeout(() => { toast.hidden = true; }, 3600);
        }

        function stopPlayback() {
            playbackRevision++;
            if (streamRequest) streamRequest.abort();
            audio.pause();
            audio.removeAttribute('src');
            audio.load();
            state.playing = false;
            updatePlayer();
            renderGrid();
        }

        searchInput.addEventListener('input', () => {
            clearTimeout(searchTimer);
            catalogRevision++;
            if (catalogRequest) catalogRequest.abort();
            searchTimer = setTimeout(() => searchStations(searchInput.value), SEARCH_DELAY);
        });
        function resumePlayback() {
            if (disposed || !state.current || !audio.paused) return;
            if (!audio.getAttribute('src')) { playStation(state.current); return; }
            const revision = ++playbackRevision;
            if (streamRequest) streamRequest.abort();
            audio.play().then(() => {
                if (!disposed && revision === playbackRevision) { state.playing = true; updatePlayer(); }
            }).catch(() => { if (!disposed && revision === playbackRevision) showToast(t('desktop.radio_error')); });
        }
        toggleBtn.addEventListener('click', () => {
            if (!state.current) return;
            if (audio.paused) {
                resumePlayback();
            } else {
                playbackRevision++;
                if (streamRequest) streamRequest.abort();
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
        audio.addEventListener('pause', () => { state.playing = false; updatePlayer(); }, { signal: listeners.signal });
        audio.addEventListener('playing', () => { state.playing = true; updatePlayer(); }, { signal: listeners.signal });
        audio.addEventListener('error', () => {
            state.playing = false;
            showToast(t('desktop.radio_error'));
            updatePlayer();
        }, { signal: listeners.signal });
        audio.addEventListener('stalled', () => { state.playing = false; updatePlayer(); showToast(t('desktop.radio_error')); }, { signal: listeners.signal });
        audio.addEventListener('waiting', () => { state.playing = false; updatePlayer(); }, { signal: listeners.signal });
        disposers.set(windowId, () => {
            disposed = true;
            clearTimeout(searchTimer);
            clearTimeout(toastTimer);
            catalogRevision++;
            if (catalogRequest) catalogRequest.abort();
            listeners.abort();
            stopPlayback();
            if ('mediaSession' in navigator) {
                try {
                    ['play', 'pause', 'stop'].forEach(action => navigator.mediaSession.setActionHandler(action, null));
                    navigator.mediaSession.metadata = null;
                } catch (_) {}
            }
        });
        if ('mediaSession' in navigator) {
            try {
                navigator.mediaSession.setActionHandler('play', () => {
                    resumePlayback();
                });
                navigator.mediaSession.setActionHandler('pause', () => audio.pause());
                navigator.mediaSession.setActionHandler('stop', stopPlayback);
            } catch (_) {}
        }

        function updateTuner(scroll = false) {
            selectedIndex = Math.max(0, Math.min(selectedIndex, state.stations.length - 1));
            const station = state.stations[selectedIndex];
            const fraction = state.stations.length > 1 ? selectedIndex / (state.stations.length - 1) : 0.5;
            host.querySelector('[data-needle]').style.left = (12 + fraction * 76) + '%';
            tuner.querySelector('img').style.transform = `rotate(${-135 + fraction * 270}deg)`;
            tuner.setAttribute('aria-label', t('desktop.radio_tune') + (station ? ': ' + station.name : ''));
            grid.querySelectorAll('.radio-card').forEach((card, index) => {
                card.classList.toggle('selected', index === selectedIndex);
                if (index === selectedIndex && scroll) card.scrollIntoView({ block: 'nearest', inline: 'nearest' });
            });
        }
        function tune(delta) {
            if (disposed || tuner.disabled) return;
            selectedIndex += delta;
            updateTuner(true);
        }
        let drag = null;
        let suppressClick = false;
        tuner.addEventListener('keydown', event => {
            const delta = { ArrowRight: 1, ArrowUp: 1, ArrowLeft: -1, ArrowDown: -1 }[event.key];
            if (delta) { event.preventDefault(); tune(delta); }
        }, { signal: listeners.signal });
        tuner.addEventListener('wheel', event => {
            if (document.activeElement !== tuner || !event.deltaY) return;
            event.preventDefault();
            tune(event.deltaY > 0 ? 1 : -1);
        }, { passive: false, signal: listeners.signal });
        const pointerAngle = event => {
            const rect = tuner.getBoundingClientRect();
            return Math.atan2(event.clientY - rect.top - rect.height / 2, event.clientX - rect.left - rect.width / 2) * 180 / Math.PI;
        };
        tuner.addEventListener('pointerdown', event => {
            if (event.button !== 0 || tuner.disabled) return;
            suppressClick = false;
            drag = { angle: pointerAngle(event), remainder: 0, moved: false };
            tuner.setPointerCapture(event.pointerId);
        }, { signal: listeners.signal });
        tuner.addEventListener('pointermove', event => {
            if (!drag) return;
            const angle = pointerAngle(event);
            const delta = ((angle - drag.angle + 540) % 360) - 180;
            drag.angle = angle;
            drag.remainder += delta;
            const steps = Math.trunc(drag.remainder / 18);
            if (steps) { drag.moved = true; drag.remainder -= steps * 18; tune(steps); }
        }, { signal: listeners.signal });
        tuner.addEventListener('pointerup', () => { suppressClick = !!drag && drag.moved; drag = null; }, { signal: listeners.signal });
        tuner.addEventListener('pointercancel', () => { suppressClick = true; drag = null; }, { signal: listeners.signal });
        tuner.addEventListener('lostpointercapture', () => { drag = null; }, { signal: listeners.signal });
        tuner.addEventListener('click', event => {
            if (suppressClick && event.detail !== 0) { suppressClick = false; return; }
            if (state.stations[selectedIndex]) playStation(state.stations[selectedIndex]);
        }, { signal: listeners.signal });

        updatePlayer();
        renderTabs();
        loadActive();
    }

    function dispose(windowId) {
        const cleanup = disposers.get(windowId);
        if (!cleanup) return;
        cleanup();
        disposers.delete(windowId);
    }

    async function fetchStations(path, signal) {
        const list = await fetchJSON(path, signal);
        return Array.isArray(list) ? list.filter(station => station && station.name && (station.url_resolved || station.url)).map(normalizeStation) : [];
    }

    async function fetchJSON(path, signal) {
        const response = await fetch(API_BASE + path, { cache: 'no-store', signal });
        if (!response.ok) throw new Error('Radio Browser HTTP');
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

    function updateMediaSession(station, t) {
        if (!('mediaSession' in navigator) || !station) return;
        const translate = typeof t === 'function' ? t : (key => key);
        try {
            navigator.mediaSession.metadata = new MediaMetadata({
                title: station.name || translate('desktop.app_radio'),
                artist: station.countrycode || '',
                album: translate('desktop.radio_album'),
                artwork: station.favicon ? [{ src: station.favicon, sizes: '96x96', type: 'image/png' }] : []
            });
        } catch (_) {}
    }

    function compactNumber(value, t) {
        const n = Number(value) || 0;
        const translate = typeof t === 'function' ? t : (key => key);
        function formatCompact(count, key, fallbackSuffix) {
            const phrase = translate(key);
            if (typeof phrase === 'string' && phrase.indexOf('{{count}}') >= 0) {
                return phrase.replaceAll('{{count}}', count);
            }
            return count + fallbackSuffix;
        }
        if (n >= 1000000) {
            return formatCompact((n / 1000000).toFixed(1).replace(/\.0$/, ''), 'desktop.radio_compact_millions', 'M');
        }
        if (n >= 1000) {
            return formatCompact((n / 1000).toFixed(1).replace(/\.0$/, ''), 'desktop.radio_compact_thousands', 'K');
        }
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
