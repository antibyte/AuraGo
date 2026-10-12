// Retro Games views: DOM builders for the neon arcade browser. Text from
// archive.org (titles, makers) only reaches the DOM through textContent and
// attributes.
(function () {
    'use strict';

    const THUMB_BASE = 'https://archive.org/services/img/';
    const PLATFORMS = ['msdos', 'arcade', 'console', 'c64', 'amiga', 'apple2', 'zx', 'win3'];
    const DECADES = [1970, 1980, 1990, 2000];
    const SORTS = ['relevance', 'popular', 'title', 'year'];
    const VIEWS = [
        ['hall', '★', 'nav_hall_of_fame'],
        ['all', '▸', 'nav_all'],
        ['recent', '⟲', 'nav_recent'],
        ['favorites', '♥', 'nav_favorites'],
        ['broken', '⚠', 'nav_broken']
    ];
    const GLYPHS = { msdos: 'C:\\>', arcade: '1UP', console: '▶', c64: 'C=', amiga: 'A500', apple2: '][', zx: 'ZX', win3: '⊞' };
    const OTHER_CONSOLE = 'Other console';

    function el(tag, attrs, children) {
        const node = document.createElement(tag);
        for (const [key, value] of Object.entries(attrs || {})) {
            if (value === undefined || value === null || value === false) continue;
            if (key === 'className') node.className = value;
            else if (key === 'text') node.textContent = String(value);
            else node.setAttribute(key, value === true ? '' : String(value));
        }
        for (const child of [].concat(children || [])) {
            if (child === null || child === undefined || child === false) continue;
            node.appendChild(typeof child === 'string' ? document.createTextNode(child) : child);
        }
        return node;
    }

    function platformLabel(t, key) {
        return t(key ? 'desktop.retro_games_platform_' + key : 'desktop.retro_games_platform_all');
    }

    function systemLabel(t, game) {
        if (!game) return '';
        if (game.platform !== 'console') return platformLabel(t, game.platform);
        if (!game.system || game.system === OTHER_CONSOLE) return t('desktop.retro_games_system_other');
        return game.system;
    }

    function metaLine(t, game) {
        const year = game && game.year ? String(game.year) : t('desktop.retro_games_unknown_year');
        return systemLabel(t, game) + ' · ' + year;
    }

    function formatNumber(value) {
        const n = Number(value) || 0;
        try {
            return new Intl.NumberFormat(document.documentElement.lang || undefined).format(n);
        } catch (_) {
            return String(n);
        }
    }

    function formatDate(seconds) {
        if (!seconds) return '';
        try {
            return new Date(seconds * 1000).toLocaleDateString(document.documentElement.lang || undefined);
        } catch (_) {
            return '';
        }
    }

    function chip(kind, value, label) {
        const node = el('button', { type: 'button', className: 'rg-chip', 'aria-pressed': 'false' }, [
            el('span', { className: 'rg-chip-label', text: label }),
            el('span', { className: 'rg-chip-count' })
        ]);
        node.dataset[kind] = value;
        return node;
    }

    function markPressed(container, kind, active) {
        for (const node of container.querySelectorAll('.rg-chip')) {
            node.setAttribute('aria-pressed', (node.dataset[kind] || '') === (active || '') ? 'true' : 'false');
        }
    }

    function systemChips(t, container, systems, active) {
        container.textContent = '';
        container.appendChild(chip('system', '', t('desktop.retro_games_platform_all')));
        for (const entry of systems || []) {
            const label = entry.name === OTHER_CONSOLE ? t('desktop.retro_games_system_other') : entry.name;
            const node = chip('system', entry.name, label);
            node.querySelector('.rg-chip-count').textContent = formatNumber(entry.count);
            container.appendChild(node);
        }
        markPressed(container, 'system', active);
    }

    function shell(t) {
        const nav = el('nav', { className: 'rg-nav', 'aria-label': t('desktop.retro_games_nav_label') });
        for (const [view, icon, key] of VIEWS) {
            const item = el('button', { type: 'button', className: 'rg-nav-item' }, [
                el('span', { className: 'rg-nav-icon', 'aria-hidden': 'true', text: icon }),
                el('span', { className: 'rg-nav-label', text: t('desktop.retro_games_' + key) }),
                el('span', { className: 'rg-nav-count' })
            ]);
            item.dataset.view = view;
            nav.appendChild(item);
        }
        const platforms = el('div', { className: 'rg-chips rg-platforms', role: 'group', 'aria-label': t('desktop.retro_games_platforms_label') });
        platforms.appendChild(chip('platform', '', platformLabel(t, '')));
        for (const key of PLATFORMS) platforms.appendChild(chip('platform', key, platformLabel(t, key)));
        const systems = el('div', { className: 'rg-chips rg-systems', role: 'group', 'aria-label': t('desktop.retro_games_systems_label'), hidden: true });
        const indexText = el('span', { className: 'rg-index-text' });
        const indexUpdate = el('button', { type: 'button', className: 'rg-index-update', text: t('desktop.retro_games_index_update') });
        const side = el('aside', { className: 'rg-side' }, [
            el('div', { className: 'rg-logo', 'aria-hidden': 'true' }, [el('span', { text: 'RETRO' }), el('span', { text: 'GAMES' })]),
            nav,
            platforms,
            systems,
            el('div', { className: 'rg-index' }, [indexText, indexUpdate])
        ]);
        const search = el('input', { type: 'search', className: 'rg-search', placeholder: t('desktop.retro_games_search_placeholder'), 'aria-label': t('desktop.retro_games_search_label'), maxlength: '200', autocomplete: 'off', spellcheck: 'false' });
        const sort = el('select', { className: 'rg-sort', 'aria-label': t('desktop.retro_games_sort_label') },
            SORTS.map(value => el('option', { value, text: t('desktop.retro_games_sort_' + value) })));
        const decade = el('select', { className: 'rg-decade', 'aria-label': t('desktop.retro_games_decade_label') },
            [el('option', { value: '', text: t('desktop.retro_games_decade_any') })].concat(DECADES.map(value => el('option', { value: String(value), text: t('desktop.retro_games_decade_value', { decade: value }) }))));
        const count = el('span', { className: 'rg-count', 'aria-live': 'polite' });
        const head = el('header', { className: 'rg-head' }, [el('span', { className: 'rg-prompt', 'aria-hidden': 'true', text: '>' }), search, count, sort, decade]);
        const banner = el('div', { className: 'rg-banner', role: 'status', hidden: true });
        const title = el('h2', { className: 'rg-section-title' });
        const grid = el('div', { className: 'rg-grid', role: 'list' });
        const empty = el('p', { className: 'rg-empty', hidden: true });
        const more = el('button', { type: 'button', className: 'rg-more', hidden: true, text: t('desktop.retro_games_load_more') });
        const coin = el('div', { className: 'rg-coin', 'aria-hidden': 'true', text: t('desktop.retro_games_insert_coin') });
        const main = el('section', { className: 'rg-main' }, [head, banner, title, el('div', { className: 'rg-scroll' }, [grid, empty, more]), coin]);
        const browse = el('div', { className: 'rg-browse' }, [side, main]);
        const stage = el('div', { className: 'rg-stage', hidden: true });
        const notice = el('p', { className: 'rg-notice', role: 'alert', hidden: true });
        const root = el('div', { className: 'rg-app', 'aria-label': t('desktop.app_retro_games') }, [browse, stage, notice]);
        return { root, browse, stage, notice, nav, platforms, systems, indexText, indexUpdate, search, sort, decade, count, banner, title, grid, empty, more };
    }

    function thumbnail(game) {
        const wrap = el('span', { className: 'rg-thumb', 'aria-hidden': 'true' });
        wrap.dataset.glyph = GLYPHS[game.platform] || '?';
        const img = el('img', { alt: '', loading: 'lazy', decoding: 'async', referrerpolicy: 'no-referrer' });
        img.addEventListener('error', () => {
            wrap.classList.add('is-missing');
            img.remove();
        }, { once: true });
        img.src = THUMB_BASE + encodeURIComponent(game.identifier);
        wrap.appendChild(img);
        return wrap;
    }

    function setFavoriteButton(t, button, on) {
        const label = on ? t('desktop.retro_games_favorite_remove') : t('desktop.retro_games_favorite_add');
        button.setAttribute('aria-pressed', on ? 'true' : 'false');
        button.setAttribute('aria-label', label);
        button.title = label;
        button.textContent = on ? '♥' : '♡';
    }

    function card(t, game, options) {
        const opts = options || {};
        const play = el('button', { type: 'button', className: 'rg-card-play', 'aria-label': t('desktop.retro_games_play_title', { title: game.title }) }, [
            thumbnail(game),
            el('span', { className: 'rg-card-title', text: game.title }),
            el('span', { className: 'rg-card-meta', text: metaLine(t, game) }),
            el('span', { className: 'rg-card-cta', 'aria-hidden': 'true', text: '▶ ' + t('desktop.retro_games_play') })
        ]);
        const fav = el('button', { type: 'button', className: 'rg-card-fav' });
        setFavoriteButton(t, fav, !!opts.favorite);
        const node = el('div', { className: 'rg-card' + (opts.unavailable ? ' is-unavailable' : '') + (opts.broken ? ' is-broken' : ''), role: 'listitem' }, [play, fav]);
        node.dataset.id = game.identifier;
        if (opts.unavailable) node.appendChild(el('span', { className: 'rg-card-note', text: t('desktop.retro_games_unavailable') }));
        return node;
    }

    window.RetroGamesViews = { el, shell, card, systemChips, markPressed, setFavoriteButton, platformLabel, systemLabel, metaLine, formatNumber, formatDate, PLATFORMS };
})();
