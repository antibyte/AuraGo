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
        // Production waits STALL_MS; only the options (tests) can shorten it.
        const stallMs = Number.isFinite(opts.stallMs) && opts.stallMs > 0 ? opts.stallMs : STALL_MS;
        const v = window.RetroGamesViews;
        const el = v.el;
        let disposed = false;
        let stallTimer = 0;
        // generation is bumped by start() and restartGame(); loadGeneration is
        // the generation whose embed navigation is in flight. Load events that
        // arrive while the two differ (the about:blank unload) are ignored.
        let generation = 0;
        let loadGeneration = 0;
        // True while the frame holds the focus only because focusFrame() put it
        // there. The window blur that call causes (and the frame being the
        // active element from mount on) is not the player touching the game.
        let frameFocusIsOurs = false;
        stage.textContent = '';

        const call = name => () => {
            if (!disposed && typeof opts[name] === 'function') opts[name]();
        };
        // Our controls hand the keyboard back to the game after they act, so
        // Space/Enter do not re-trigger the button and the arrows reach the game.
        const returnFocus = handler => () => {
            handler();
            focusFrame();
        };
        const button = (className, label, text, handler) => {
            const node = el('button', { type: 'button', className: 'rg-btn ' + className, 'aria-label': label, title: label, text });
            node.addEventListener('click', handler);
            return node;
        };

        const back = el('button', { type: 'button', className: 'rg-btn rg-back', text: '◀ ' + t('desktop.retro_games_back') });
        back.addEventListener('click', call('onBack'));
        const fav = button('rg-player-fav', '', '', returnFocus(call('onFavorite')));
        const broken = button('rg-player-broken', '', '⚠', returnFocus(call('onBroken')));
        const restart = button('rg-restart', t('desktop.retro_games_restart'), '⟲', returnFocus(() => restartGame()));
        const full = button('rg-full', t('desktop.retro_games_fullscreen'), '⛶', returnFocus(() => toggleFullscreen()));
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
        // The loading bar and the "taking unusually long" hint are the same
        // bottom strip. It never takes pointer events itself (only the hint
        // buttons do), so the game stays clickable underneath it.
        const overlayText = el('p', { className: 'rg-overlay-text' });
        const stalledActions = el('div', { className: 'rg-overlay-actions', hidden: true }, [
            button('rg-overlay-restart', t('desktop.retro_games_restart'), '⟲ ' + t('desktop.retro_games_restart'), returnFocus(() => restartGame())),
            button('rg-overlay-external', t('desktop.retro_games_open_archive'), '↗ ' + t('desktop.retro_games_open_archive'), call('onExternal')),
            button('rg-overlay-broken', t('desktop.retro_games_mark_broken'), '⚠ ' + t('desktop.retro_games_mark_broken'), returnFocus(call('onMarkBroken')))
        ]);
        const overlay = el('div', { className: 'rg-overlay', role: 'status' }, [overlayText, stalledActions]);
        const screen = el('div', { className: 'rg-screen' }, [frame, overlay]);
        const bezel = el('div', { className: 'rg-bezel' }, [screen]);
        const panel = el('footer', { className: 'rg-panel' }, [
            el('span', { className: 'rg-panel-label', text: t('desktop.retro_games_controls_label') }),
            el('span', { className: 'rg-panel-controls', text: t('desktop.retro_games_controls_' + game.platform) }),
            el('span', { className: 'rg-panel-hint', text: t('desktop.retro_games_focus_hint') })
        ]);

        // archive.org fires the embed's load event only after every sub-resource
        // (a splash GIF can take minutes), long after the emulator is usable.
        frame.addEventListener('load', () => {
            if (disposed || loadGeneration !== generation) return;
            hideOverlay();
        });
        frame.addEventListener('focusout', () => {
            frameFocusIsOurs = false;
        });

        // The cross-origin frame hides its input from us. What the parent can
        // see is its own window blurring while the frame becomes the active
        // element, and that only counts when the user (not focusFrame()) did it.
        function userFocusedFrame() {
            return document.activeElement === frame && !frameFocusIsOurs;
        }

        function onWindowBlur() {
            if (!disposed && !overlay.hidden && userFocusedFrame()) hideOverlay();
        }

        function focusFrame() {
            if (disposed) return;
            // Set before the call: the blur it causes is delivered synchronously.
            frameFocusIsOurs = true;
            frame.focus();
            frameFocusIsOurs = document.activeElement === frame;
        }

        function hideOverlay() {
            clearTimeout(stallTimer);
            const hadFocus = overlay.contains(document.activeElement);
            overlay.hidden = true;
            overlay.classList.remove('is-stalled');
            stalledActions.hidden = true;
            // A focused hint button disappears with the strip; keep the keyboard in the game.
            if (hadFocus) focusFrame();
        }

        function armOverlay() {
            clearTimeout(stallTimer);
            overlay.hidden = false;
            overlay.classList.remove('is-stalled');
            overlayText.textContent = t('desktop.retro_games_player_loading');
            stalledActions.hidden = true;
            stallTimer = setTimeout(() => {
                if (disposed) return;
                // The user is already playing: nothing to report.
                if (userFocusedFrame()) {
                    hideOverlay();
                    return;
                }
                overlay.classList.add('is-stalled');
                overlayText.textContent = t('desktop.retro_games_player_stalled');
                stalledActions.hidden = false;
            }, stallMs);
        }

        function start() {
            armOverlay();
            generation += 1;
            loadGeneration = generation;
            frame.setAttribute('src', embedURL(game.identifier));
        }

        function restartGame() {
            if (disposed) return;
            generation += 1;
            const pending = generation;
            frame.setAttribute('src', 'about:blank');
            setTimeout(() => {
                if (!disposed && pending === generation) start();
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
        window.addEventListener('blur', onWindowBlur);
        start();
        stage.appendChild(el('div', { className: 'rg-cabinet' }, [marquee, bezel, panel]));
        // The game gets the keyboard from the start (Space/Enter must not hit Back).
        focusFrame();

        return {
            setFavorite,
            setBroken,
            destroy() {
                if (disposed) return;
                disposed = true;
                clearTimeout(stallTimer);
                window.removeEventListener('blur', onWindowBlur);
                if (document.fullscreenElement === bezel && document.exitFullscreen) document.exitFullscreen().catch(() => {});
                frame.setAttribute('src', 'about:blank');
                frame.remove();
                stage.textContent = '';
            }
        };
    }

    window.RetroGamesPlayer = { mount, embedURL, SANDBOX, ALLOW };
})();
