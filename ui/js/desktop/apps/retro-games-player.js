// Retro Games cabinet player: runs archive.org's own emulator embed in a
// locked-down iframe and unloads it completely when the player closes.
(function () {
    'use strict';

    const EMBED_BASE = 'https://archive.org/embed/';
    const SANDBOX = 'allow-scripts allow-same-origin allow-pointer-lock';
    const ALLOW = 'fullscreen; gamepad; autoplay';
    const STALL_MS = 60000;
    const ID_PATTERN = /^[A-Za-z0-9][A-Za-z0-9._-]{0,99}$/;

    function embedURL(id) {
        return EMBED_BASE + encodeURIComponent(id);
    }

    function mount(stage, game, options) {
        if (!game || !ID_PATTERN.test(game.identifier || '')) throw new Error('invalid game identifier');
        const opts = options || {};
        const t = typeof opts.t === 'function' ? opts.t : (key => key);
        const v = window.RetroGamesViews;
        const el = v.el;
        let disposed = false;
        let stallTimer = 0;
        stage.textContent = '';

        const call = name => () => {
            if (!disposed && typeof opts[name] === 'function') opts[name]();
        };
        const button = (className, label, text, handler) => {
            const node = el('button', { type: 'button', className: 'rg-btn ' + className, 'aria-label': label, title: label, text });
            node.addEventListener('click', handler);
            return node;
        };

        const back = el('button', { type: 'button', className: 'rg-btn rg-back', text: '◀ ' + t('desktop.retro_games_back') });
        back.addEventListener('click', call('onBack'));
        const fav = button('rg-player-fav', '', '', call('onFavorite'));
        const broken = button('rg-player-broken', '', '⚠', call('onBroken'));
        const restart = button('rg-restart', t('desktop.retro_games_restart'), '⟲', () => restartGame());
        const full = button('rg-full', t('desktop.retro_games_fullscreen'), '⛶', () => toggleFullscreen());
        const external = button('rg-external', t('desktop.retro_games_open_archive'), '↗', call('onExternal'));
        const marquee = el('header', { className: 'rg-marquee' }, [
            back,
            el('div', { className: 'rg-marquee-title' }, [
                el('span', { className: 'rg-marquee-name', text: game.title }),
                el('span', { className: 'rg-marquee-meta', text: v.metaLine(t, game) })
            ]),
            el('div', { className: 'rg-marquee-actions' }, [fav, broken, restart, full, external])
        ]);

        const frame = el('iframe', {
            className: 'rg-frame',
            sandbox: SANDBOX,
            allow: ALLOW,
            allowfullscreen: true,
            referrerpolicy: 'no-referrer',
            title: t('desktop.retro_games_frame_title', { title: game.title })
        });
        const overlayText = el('p', { className: 'rg-overlay-text' });
        const stalledActions = el('div', { className: 'rg-overlay-actions', hidden: true }, [
            button('rg-overlay-restart', t('desktop.retro_games_restart'), '⟲ ' + t('desktop.retro_games_restart'), () => restartGame()),
            button('rg-overlay-external', t('desktop.retro_games_open_archive'), '↗ ' + t('desktop.retro_games_open_archive'), call('onExternal')),
            button('rg-overlay-broken', t('desktop.retro_games_mark_broken'), '⚠ ' + t('desktop.retro_games_mark_broken'), call('onBroken'))
        ]);
        const overlay = el('div', { className: 'rg-overlay', role: 'status' }, [overlayText, stalledActions]);
        const screen = el('div', { className: 'rg-screen' }, [frame, overlay]);
        const bezel = el('div', { className: 'rg-bezel' }, [screen]);
        const panel = el('footer', { className: 'rg-panel' }, [
            el('span', { className: 'rg-panel-label', text: t('desktop.retro_games_controls_label') }),
            el('span', { className: 'rg-panel-controls', text: t('desktop.retro_games_controls_' + game.platform) }),
            el('span', { className: 'rg-panel-hint', text: t('desktop.retro_games_focus_hint') })
        ]);

        frame.addEventListener('load', () => {
            if (disposed || frame.getAttribute('src') !== embedURL(game.identifier)) return;
            clearTimeout(stallTimer);
            overlay.hidden = true;
        });

        function armOverlay() {
            clearTimeout(stallTimer);
            overlay.hidden = false;
            overlay.classList.remove('is-stalled');
            overlayText.textContent = t('desktop.retro_games_player_loading');
            stalledActions.hidden = true;
            stallTimer = setTimeout(() => {
                if (disposed) return;
                overlay.classList.add('is-stalled');
                overlayText.textContent = t('desktop.retro_games_player_stalled');
                stalledActions.hidden = false;
            }, STALL_MS);
        }

        function start() {
            armOverlay();
            frame.setAttribute('src', embedURL(game.identifier));
        }

        function restartGame() {
            if (disposed) return;
            frame.setAttribute('src', 'about:blank');
            setTimeout(() => {
                if (!disposed) start();
            }, 0);
        }

        function toggleFullscreen() {
            if (document.fullscreenElement === bezel) {
                if (document.exitFullscreen) document.exitFullscreen().catch(() => {});
                return;
            }
            if (typeof bezel.requestFullscreen === 'function') bezel.requestFullscreen().catch(() => {});
        }

        function setFavorite(on) {
            v.setFavoriteButton(t, fav, !!on);
        }

        function setBroken(on) {
            const label = on ? t('desktop.retro_games_unmark_broken') : t('desktop.retro_games_mark_broken');
            broken.setAttribute('aria-pressed', on ? 'true' : 'false');
            broken.setAttribute('aria-label', label);
            broken.title = label;
        }

        setFavorite(opts.favorite);
        setBroken(opts.broken);
        start();
        stage.appendChild(el('div', { className: 'rg-cabinet' }, [marquee, bezel, panel]));
        back.focus();

        return {
            setFavorite,
            setBroken,
            destroy() {
                if (disposed) return;
                disposed = true;
                clearTimeout(stallTimer);
                if (document.fullscreenElement === bezel && document.exitFullscreen) document.exitFullscreen().catch(() => {});
                frame.setAttribute('src', 'about:blank');
                frame.remove();
                stage.textContent = '';
            }
        };
    }

    window.RetroGamesPlayer = { mount, embedURL, SANDBOX, ALLOW };
})();
