// Retro Games: archive.org arcade browser and cabinet player for the desktop.
(function () {
    'use strict';

    const instances = new Map();
    const BASE = '/api/desktop/retro-games';
    const DETAILS_BASE = 'https://archive.org/details/';
    const ID_PATTERN = /^[A-Za-z0-9][A-Za-z0-9._-]{0,99}$/;
    const POLL_MS = 1500;
    const SEARCH_DELAY_MS = 250;
    const PAGE_SIZE = 60;
    const NOTICE_MS = 6000;
    const NAV_KEYS = { hall: 'nav_hall_of_fame', all: 'nav_all', recent: 'nav_recent', favorites: 'nav_favorites', broken: 'nav_broken' };
    const EMPTY_KEYS = { all: 'empty_results', hall: 'empty_hall', recent: 'empty_recent', favorites: 'empty_favorites', broken: 'empty_broken' };

    function render(host, windowId, context) {
        dispose(windowId);
        const ctx = context || {};
        const v = window.RetroGamesViews;
        const t = (key, params) => (typeof ctx.t === 'function' ? ctx.t(key, params) : key);
        const ui = v.shell(t);
        host.textContent = '';
        host.appendChild(ui.root);
        const st = {
            host, windowId, ctx, t, v, ui,
            controller: new AbortController(),
            disposed: false,
            disabled: false,
            view: 'hall',
            platform: '',
            system: '',
            sort: 'relevance',
            decade: '',
            query: '',
            items: [],
            total: 0,
            listSeq: 0,
            hall: [],
            status: null,
            wasSyncing: false,
            library: { favorites: [], recent: [], broken: [] },
            favorites: new Set(),
            broken: new Set(),
            pollTimer: 0,
            searchTimer: 0,
            noticeTimer: 0,
            player: null,
            current: null
        };
        instances.set(windowId, st);
        wire(st);
        const game = typeof ctx.game === 'string' && ID_PATTERN.test(ctx.game) ? ctx.game : '';
        loadLibrary(st).then(() => {
            if (st.disposed) return;
            refreshStatus(st);
            if (game) openGameById(st, game);
            else showView(st, 'hall');
        });
    }

    function request(st, path, options) {
        if (typeof st.ctx.api !== 'function') return Promise.reject(new Error('Desktop API unavailable'));
        return st.ctx.api(BASE + path, Object.assign({ signal: st.controller.signal }, options || {}));
    }

    function isAbort(err) {
        return !!err && err.name === 'AbortError';
    }

    function errorCode(err) {
        return (err && err.body && err.body.code) || '';
    }

    function handleError(st, err, key) {
        if (st.disposed || isAbort(err)) return;
        if (errorCode(err) === 'disabled') {
            showDisabled(st);
            return;
        }
        notice(st, key || 'desktop.retro_games_load_failed');
    }

    function notice(st, key, params) {
        clearTimeout(st.noticeTimer);
        st.ui.notice.textContent = st.t(key, params);
        st.ui.notice.hidden = false;
        st.noticeTimer = setTimeout(() => {
            if (!st.disposed) st.ui.notice.hidden = true;
        }, NOTICE_MS);
    }

    function showDisabled(st) {
        st.disabled = true;
        clearTimeout(st.pollTimer);
        stopPlayer(st);
        st.ui.stage.hidden = true;
        st.ui.browse.hidden = false;
        st.ui.grid.textContent = '';
        st.ui.more.hidden = true;
        st.ui.banner.hidden = true;
        st.ui.empty.textContent = st.t('desktop.retro_games_disabled');
        st.ui.empty.hidden = false;
    }

    function wire(st) {
        const { ui } = st;
        const opts = { signal: st.controller.signal };
        ui.nav.addEventListener('click', event => {
            const item = event.target.closest('.rg-nav-item');
            if (item) showView(st, item.dataset.view);
        }, opts);
        ui.platforms.addEventListener('click', event => {
            const chip = event.target.closest('.rg-chip');
            if (!chip) return;
            st.platform = chip.dataset.platform || '';
            st.system = '';
            renderFilters(st);
            reloadList(st);
        }, opts);
        ui.systems.addEventListener('click', event => {
            const chip = event.target.closest('.rg-chip');
            if (!chip) return;
            st.system = chip.dataset.system || '';
            st.v.markPressed(ui.systems, 'system', st.system);
            reloadList(st);
        }, opts);
        ui.search.addEventListener('input', () => {
            clearTimeout(st.searchTimer);
            st.searchTimer = setTimeout(() => applySearch(st), SEARCH_DELAY_MS);
        }, opts);
        ui.search.addEventListener('keydown', event => {
            if (event.key !== 'Enter') return;
            event.preventDefault();
            clearTimeout(st.searchTimer);
            applySearch(st);
        }, opts);
        ui.sort.addEventListener('change', () => {
            st.sort = ui.sort.value;
            reloadList(st);
        }, opts);
        ui.decade.addEventListener('change', () => {
            st.decade = ui.decade.value;
            reloadList(st);
        }, opts);
        ui.more.addEventListener('click', () => loadMore(st), opts);
        ui.indexUpdate.addEventListener('click', () => startSync(st), opts);
        ui.banner.addEventListener('click', event => {
            if (event.target.closest('[data-action="retry"]')) startSync(st);
        }, opts);
        ui.grid.addEventListener('click', event => {
            const card = event.target.closest('.rg-card');
            if (!card) return;
            const game = findGame(st, card.dataset.id);
            if (!game) return;
            if (event.target.closest('.rg-card-fav')) toggleFavorite(st, game);
            else if (event.target.closest('.rg-card-play')) playGame(st, game);
        }, opts);
        ui.grid.addEventListener('keydown', event => onGridKey(st, event), opts);
        ui.grid.addEventListener('contextmenu', event => onGridContextMenu(st, event), opts);
    }

    function applySearch(st) {
        const query = st.ui.search.value.trim();
        if (query === st.query && st.view === 'all') return;
        st.query = query;
        if (query && st.view !== 'all') {
            showView(st, 'all');
            return;
        }
        reloadList(st);
    }

    function showView(st, view) {
        st.view = NAV_KEYS[view] ? view : 'hall';
        st.ui.title.textContent = st.t('desktop.retro_games_' + NAV_KEYS[st.view]);
        st.ui.sort.hidden = st.view !== 'all';
        renderNavCounts(st);
        reloadList(st);
    }

    function renderNavCounts(st) {
        const counts = { favorites: st.library.favorites.length, recent: st.library.recent.length, broken: st.library.broken.length };
        for (const item of st.ui.nav.querySelectorAll('.rg-nav-item')) {
            const view = item.dataset.view;
            const n = counts[view] || 0;
            item.querySelector('.rg-nav-count').textContent = n ? st.v.formatNumber(n) : '';
            if (view === 'broken') item.hidden = !n && st.view !== 'broken';
            if (view === st.view) item.setAttribute('aria-current', 'page');
            else item.removeAttribute('aria-current');
        }
    }

    function renderFilters(st) {
        const { ui, v } = st;
        v.markPressed(ui.platforms, 'platform', st.platform);
        const systems = (st.status && st.status.systems) || [];
        const showSystems = st.platform === 'console' && systems.length > 0;
        ui.systems.hidden = !showSystems;
        if (showSystems) v.systemChips(st.t, ui.systems, systems, st.system);
    }

    function filterLocal(st, games) {
        const decade = Number(st.decade) || 0;
        return games.filter(game => (!st.platform || game.platform === st.platform)
            && (!st.system || game.system === st.system)
            && (!decade || (game.year >= decade && game.year <= decade + 9)));
    }

    function reloadList(st) {
        if (st.disabled) return;
        const seq = ++st.listSeq;
        st.items = [];
        st.total = 0;
        if (st.view === 'all') {
            fetchSearch(st, seq, 0);
            return;
        }
        if (st.view === 'hall') {
            const qs = st.platform ? '?platform=' + encodeURIComponent(st.platform) : '';
            request(st, '/hall-of-fame' + qs).then(body => {
                if (st.disposed || seq !== st.listSeq) return;
                st.hall = body.games || [];
                showItems(st, filterLocal(st, st.hall), false);
            }).catch(err => {
                if (seq === st.listSeq) handleError(st, err);
            });
            return;
        }
        showItems(st, filterLocal(st, st.library[st.view] || []), false);
    }

    function fetchSearch(st, seq, offset) {
        const params = new URLSearchParams();
        if (st.query) params.set('q', st.query);
        if (st.platform) params.set('platform', st.platform);
        if (st.system) params.set('system', st.system);
        if (st.decade) params.set('decade', st.decade);
        params.set('sort', st.sort);
        params.set('offset', String(offset));
        params.set('limit', String(PAGE_SIZE));
        request(st, '/search?' + params.toString()).then(body => {
            if (st.disposed || seq !== st.listSeq) return;
            st.total = Number(body.total) || 0;
            showItems(st, body.games || [], offset > 0);
        }).catch(err => {
            if (seq === st.listSeq) handleError(st, err);
        });
    }

    function loadMore(st) {
        if (st.view !== 'all' || st.items.length >= st.total) return;
        fetchSearch(st, st.listSeq, st.items.length);
    }

    function showItems(st, games, append) {
        const { ui, t, v } = st;
        if (!append) {
            ui.grid.textContent = '';
            st.items = [];
        }
        const fragment = document.createDocumentFragment();
        for (const game of games) {
            if (!game || !ID_PATTERN.test(game.identifier || '')) continue;
            st.items.push(game);
            fragment.appendChild(v.card(t, game, {
                favorite: st.favorites.has(game.identifier),
                broken: st.broken.has(game.identifier),
                unavailable: game.available === false
            }));
        }
        ui.grid.appendChild(fragment);
        const total = st.view === 'all' ? st.total : st.items.length;
        ui.count.textContent = t('desktop.retro_games_results_count', { count: v.formatNumber(total) });
        ui.more.hidden = !(st.view === 'all' && st.items.length < st.total);
        ui.empty.hidden = st.items.length > 0;
        if (!st.items.length) ui.empty.textContent = t('desktop.retro_games_' + EMPTY_KEYS[st.view]);
    }

    function findGame(st, id) {
        for (const list of [st.items, st.hall, st.library.favorites, st.library.recent, st.library.broken]) {
            const hit = list.find(game => game.identifier === id);
            if (hit) return hit;
        }
        return null;
    }

    function loadLibrary(st) {
        return request(st, '/library').then(lib => {
            if (st.disposed) return;
            st.library = { favorites: lib.favorites || [], recent: lib.recent || [], broken: lib.broken || [] };
            st.favorites = new Set(st.library.favorites.map(entry => entry.identifier));
            st.broken = new Set(st.library.broken.map(entry => entry.identifier));
            renderNavCounts(st);
        }).catch(err => handleError(st, err));
    }

    function refreshStatus(st) {
        clearTimeout(st.pollTimer);
        if (st.disposed || st.disabled) return;
        request(st, '/status').then(status => {
            if (st.disposed) return;
            if (!status.enabled) {
                showDisabled(st);
                return;
            }
            const finished = st.wasSyncing && !status.syncing;
            st.status = status;
            st.wasSyncing = !!status.syncing;
            renderStatus(st);
            if (finished && st.view === 'all') reloadList(st);
            if (status.syncing) st.pollTimer = setTimeout(() => refreshStatus(st), POLL_MS);
        }).catch(err => handleError(st, err));
    }

    function renderStatus(st) {
        const { ui, t, v, status } = st;
        const platforms = status.platforms || [];
        const counts = {};
        let latest = 0;
        for (const p of platforms) {
            counts[p.key] = p.count || 0;
            if ((p.last_success_at || 0) > latest) latest = p.last_success_at;
        }
        for (const chip of ui.platforms.querySelectorAll('.rg-chip')) {
            const key = chip.dataset.platform;
            const n = key ? counts[key] : status.total_games;
            chip.querySelector('.rg-chip-count').textContent = n ? v.formatNumber(n) : '';
        }
        if (status.total_games > 0) {
            ui.indexText.textContent = t('desktop.retro_games_index_status', { count: v.formatNumber(status.total_games), date: v.formatDate(latest) });
        } else {
            ui.indexText.textContent = status.syncing ? t('desktop.retro_games_index_updating') : t('desktop.retro_games_index_empty');
        }
        ui.indexUpdate.disabled = !!status.syncing;
        renderFilters(st);
        renderBanner(st, platforms, latest);
    }

    function renderBanner(st, platforms, latest) {
        const { ui, t, v, status } = st;
        const el = v.el;
        ui.banner.textContent = '';
        if (status.syncing && !status.total_games) {
            const index = platforms.findIndex(p => p.state === 'running');
            const progress = index >= 0
                ? t('desktop.retro_games_sync_progress', { platform: v.platformLabel(t, platforms[index].key), done: index + 1, total: platforms.length })
                : t('desktop.retro_games_index_updating');
            ui.banner.dataset.kind = 'sync';
            ui.banner.append(
                el('strong', { text: t('desktop.retro_games_sync_first_title') }),
                el('span', { text: t('desktop.retro_games_sync_first_body') }),
                el('span', { className: 'rg-progress', text: progress })
            );
            ui.banner.hidden = false;
            return;
        }
        if (status.syncing) {
            ui.banner.dataset.kind = 'info';
            ui.banner.append(el('span', { text: t('desktop.retro_games_index_updating') }));
            ui.banner.hidden = false;
            return;
        }
        const failed = platforms.some(p => p.state === 'failed') || (!status.total_games && platforms.some(p => p.last_error));
        if (failed) {
            ui.banner.dataset.kind = 'error';
            ui.banner.append(
                el('span', { text: latest ? t('desktop.retro_games_sync_failed', { date: v.formatDate(latest) }) : t('desktop.retro_games_sync_failed_never') }),
                el('button', { type: 'button', className: 'rg-banner-action', 'data-action': 'retry', text: t('desktop.retro_games_retry') })
            );
            ui.banner.hidden = false;
            return;
        }
        ui.banner.hidden = true;
    }

    function startSync(st) {
        request(st, '/sync', { method: 'POST' }).then(() => refreshStatus(st)).catch(err => {
            if (errorCode(err) === 'sync_running') {
                refreshStatus(st);
                return;
            }
            handleError(st, err, 'desktop.retro_games_action_failed');
        });
    }

    function openExternal(id) {
        window.open(DETAILS_BASE + encodeURIComponent(id), '_blank', 'noopener,noreferrer');
    }

    function playGame(st, game) {
        if (!game || !ID_PATTERN.test(game.identifier || '') || !window.RetroGamesPlayer || st.disabled) return;
        stopPlayer(st);
        st.current = game;
        st.ui.browse.hidden = true;
        st.ui.stage.hidden = false;
        st.player = window.RetroGamesPlayer.mount(st.ui.stage, game, {
            t: st.t,
            favorite: st.favorites.has(game.identifier),
            broken: st.broken.has(game.identifier),
            onBack: () => backToArcade(st),
            onFavorite: () => toggleFavorite(st, game),
            onBroken: () => toggleBroken(st, game),
            onExternal: () => openExternal(game.identifier)
        });
        request(st, '/plays/' + encodeURIComponent(game.identifier), { method: 'POST' })
            .then(() => loadLibrary(st))
            .catch(err => handleError(st, err, 'desktop.retro_games_action_failed'));
    }

    function stopPlayer(st) {
        if (!st.player) return;
        st.player.destroy();
        st.player = null;
        st.current = null;
    }

    function backToArcade(st) {
        const id = st.current && st.current.identifier;
        stopPlayer(st);
        st.ui.stage.hidden = true;
        st.ui.browse.hidden = false;
        if (st.view === 'recent' || st.view === 'favorites' || st.view === 'broken') reloadList(st);
        const card = id ? Array.from(st.ui.grid.querySelectorAll('.rg-card')).find(node => node.dataset.id === id) : null;
        const target = card ? card.querySelector('.rg-card-play') : st.ui.search;
        if (target) target.focus();
    }

    function openGameById(st, id) {
        if (!ID_PATTERN.test(id)) return;
        const known = findGame(st, id);
        if (known) {
            playGame(st, known);
            return;
        }
        request(st, '/games/' + encodeURIComponent(id)).then(body => {
            if (!st.disposed && body && body.game) playGame(st, body.game);
        }).catch(err => {
            if (st.disposed || isAbort(err)) return;
            if (!st.player) {
                st.ui.stage.hidden = true;
                st.ui.browse.hidden = false;
                if (!st.items.length) showView(st, st.view);
            }
            const code = errorCode(err);
            if (code === 'game_not_found' || code === 'invalid_identifier') notice(st, 'desktop.retro_games_unknown_game');
            else handleError(st, err);
        });
    }

    function setFavoriteState(st, id, on) {
        if (on) st.favorites.add(id);
        else st.favorites.delete(id);
        for (const card of st.ui.grid.querySelectorAll('.rg-card')) {
            if (card.dataset.id === id) st.v.setFavoriteButton(st.t, card.querySelector('.rg-card-fav'), on);
        }
        if (st.player && st.current && st.current.identifier === id) st.player.setFavorite(on);
    }

    function setBrokenState(st, id, on) {
        if (on) st.broken.add(id);
        else st.broken.delete(id);
        for (const card of st.ui.grid.querySelectorAll('.rg-card')) {
            if (card.dataset.id === id) card.classList.toggle('is-broken', on);
        }
        if (st.player && st.current && st.current.identifier === id) st.player.setBroken(on);
    }

    function toggleFavorite(st, game) {
        const id = game.identifier;
        const on = !st.favorites.has(id);
        setFavoriteState(st, id, on);
        request(st, '/favorites/' + encodeURIComponent(id), { method: on ? 'PUT' : 'DELETE' })
            .then(() => loadLibrary(st))
            .then(() => {
                if (!st.disposed && st.view === 'favorites' && !st.player) reloadList(st);
            })
            .catch(err => {
                if (st.disposed || isAbort(err)) return;
                setFavoriteState(st, id, !on);
                handleError(st, err, 'desktop.retro_games_action_failed');
            });
    }

    function toggleBroken(st, game) {
        const id = game.identifier;
        const on = !st.broken.has(id);
        setBrokenState(st, id, on);
        request(st, '/broken/' + encodeURIComponent(id), { method: on ? 'PUT' : 'DELETE' })
            .then(() => loadLibrary(st))
            .then(() => {
                if (!st.disposed && st.view === 'broken' && !st.player) reloadList(st);
            })
            .catch(err => {
                if (st.disposed || isAbort(err)) return;
                setBrokenState(st, id, !on);
                handleError(st, err, 'desktop.retro_games_action_failed');
            });
    }

    function onGridKey(st, event) {
        const buttons = Array.from(st.ui.grid.querySelectorAll('.rg-card-play'));
        const index = buttons.indexOf(document.activeElement);
        if (index < 0) return;
        if (event.key === 'f' || event.key === 'F') {
            event.preventDefault();
            const game = findGame(st, buttons[index].closest('.rg-card').dataset.id);
            if (game) toggleFavorite(st, game);
            return;
        }
        const columns = Math.max(1, getComputedStyle(st.ui.grid).gridTemplateColumns.split(' ').filter(Boolean).length);
        const delta = { ArrowRight: 1, ArrowLeft: -1, ArrowDown: columns, ArrowUp: -columns }[event.key];
        if (!delta) return;
        event.preventDefault();
        const next = buttons[Math.min(buttons.length - 1, Math.max(0, index + delta))];
        if (next) next.focus();
    }

    function onGridContextMenu(st, event) {
        const card = event.target.closest('.rg-card');
        if (!card || typeof st.ctx.showContextMenu !== 'function') return;
        const game = findGame(st, card.dataset.id);
        if (!game) return;
        event.preventDefault();
        const t = st.t;
        const id = game.identifier;
        st.ctx.showContextMenu(event.clientX, event.clientY, [
            { id: 'play', label: t('desktop.retro_games_play'), action: () => playGame(st, game) },
            { id: 'favorite', label: st.favorites.has(id) ? t('desktop.retro_games_favorite_remove') : t('desktop.retro_games_favorite_add'), action: () => toggleFavorite(st, game) },
            { id: 'broken', label: st.broken.has(id) ? t('desktop.retro_games_unmark_broken') : t('desktop.retro_games_mark_broken'), action: () => toggleBroken(st, game) },
            { type: 'separator' },
            { id: 'archive', label: t('desktop.retro_games_open_archive'), action: () => openExternal(id) }
        ]);
    }

    function open(windowId, context) {
        const st = instances.get(windowId);
        if (!st || st.disposed || !context || typeof context.game !== 'string' || !ID_PATTERN.test(context.game)) return;
        openGameById(st, context.game);
    }

    function dispose(windowId) {
        const st = instances.get(windowId);
        if (!st) return;
        st.disposed = true;
        clearTimeout(st.pollTimer);
        clearTimeout(st.searchTimer);
        clearTimeout(st.noticeTimer);
        st.controller.abort();
        stopPlayer(st);
        instances.delete(windowId);
    }

    window.RetroGamesApp = { render, dispose, open };
})();
