// Retro Games cabinet player: frames AuraGo's own player page, which runs
// archive.org's emulators in an opaque origin, follows the state messages that
// page posts and unloads the frame completely when the player closes.
(function () {
    'use strict';

    const PLAYER_BASE = '/api/desktop/retro-games/player?id=';
    // Scripts and pointer lock only, never the page's own origin: archive.org's
    // code in the frame must stay in an opaque origin, unable to read AuraGo's
    // cookies, storage, DOM or APIs.
    const SANDBOX = 'allow-scripts allow-pointer-lock';
    const ALLOW = 'fullscreen; gamepad; autoplay';
    const STALL_MS = 60000;
    const MUTE_KEY = 'aurago.retroGames.muted';
    // The page's states, in the order a healthy start goes through them.
    const PLAYER_STATES = ['loading', 'running', 'failed'];
    const ID_PATTERN = /^[A-Za-z0-9][A-Za-z0-9._-]{0,99}$/;

    function playerURL(id, muted, soundLabel) {
        return PLAYER_BASE + encodeURIComponent(id) + '&muted=' + (muted ? '1' : '0') + '&sound=' + encodeURIComponent(soundLabel || '');
    }

    // The sound choice is remembered per viewer; storage may be blocked or full.
    function readMuted() {
        try {
            return window.localStorage.getItem(MUTE_KEY) === '1';
        } catch (error) {
            return false;
        }
    }

    function writeMuted(on) {
        try {
            window.localStorage.setItem(MUTE_KEY, on ? '1' : '0');
        } catch (error) {
            // The choice then only lasts for this player.
        }
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
        // The current player iframe. Every start builds a new one (see start()).
        let frame = null;
        let muted = readMuted();
        // The mute value the current page was started with. The page cannot hear
        // a click that comes before it listens, so it is told again once it says
        // it started (see onPlayerState).
        let urlMuted = muted;
        // Where the current page is: 'idle' until it posts its first state, then
        // 'loading' -> 'running'; 'failed' is final until the next start.
        let phase = 'idle';
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
        const mute = button('rg-mute', '', '', returnFocus(() => toggleMute()));
        const restart = button('rg-restart', t('desktop.retro_games_restart'), '⟲', returnFocus(() => restartGame()));
        const full = button('rg-full', t('desktop.retro_games_fullscreen'), '⛶', returnFocus(() => toggleFullscreen()));
        const external = button('rg-external', t('desktop.retro_games_open_archive'), '↗', call('onExternal'));
        const marquee = el('header', { className: 'rg-marquee' }, [
            back,
            el('div', { className: 'rg-marquee-title' }, [
                el('span', { className: 'rg-marquee-name', text: game.title }),
                el('span', { className: 'rg-marquee-meta', text: v.metaLine(t, game) })
            ]),
            el('div', { className: 'rg-marquee-actions' }, [fav, broken, mute, restart, full, external])
        ]);

        function makeFrame() {
            return el('iframe', {
                className: 'rg-frame',
                sandbox: SANDBOX,
                allow: ALLOW,
                allowfullscreen: true,
                referrerpolicy: 'no-referrer',
                title: t('desktop.retro_games_frame_title', { title: game.title })
            });
        }
        // The loading bar and the hints (slow start, failed start) are the same
        // bottom strip. It never takes pointer events itself (only the hint
        // buttons do), so the game stays clickable underneath it.
        const overlayText = el('p', { className: 'rg-overlay-text' });
        const hintActions = el('div', { className: 'rg-overlay-actions', hidden: true }, [
            button('rg-overlay-restart', t('desktop.retro_games_restart'), '⟲ ' + t('desktop.retro_games_restart'), returnFocus(() => restartGame())),
            button('rg-overlay-external', t('desktop.retro_games_open_archive'), '↗ ' + t('desktop.retro_games_open_archive'), call('onExternal')),
            button('rg-overlay-broken', t('desktop.retro_games_mark_broken'), '⚠ ' + t('desktop.retro_games_mark_broken'), returnFocus(call('onMarkBroken')))
        ]);
        const overlay = el('div', { className: 'rg-overlay', role: 'status' }, [overlayText, hintActions]);
        // The frame goes in front of the strip when the first start() builds it.
        const screen = el('div', { className: 'rg-screen' }, [overlay]);
        const bezel = el('div', { className: 'rg-bezel' }, [screen]);
        const panel = el('footer', { className: 'rg-panel' }, [
            el('span', { className: 'rg-panel-label', text: t('desktop.retro_games_controls_label') }),
            el('span', { className: 'rg-panel-controls', text: t('desktop.retro_games_controls_' + game.platform) }),
            el('span', { className: 'rg-panel-hint', text: t('desktop.retro_games_focus_hint') })
        ]);

        // The page tells us how it is doing; nothing else says anything about the
        // game. Only the frame's own window counts, and only messages that carry
        // the {rg: 1} marker of the protocol (see internal/server/retro_games_player.go).
        // The sender is not trusted beyond that: archive.org's code runs in the
        // same realm as the page and can post any {rg: 1} message, so only the
        // known types with well-formed values act, everything else is dropped
        // silently, and nothing from a message is ever rendered or logged.
        function onMessage(event) {
            const data = event.data;
            if (disposed || !frame.contentWindow || event.source !== frame.contentWindow || !data || data.rg !== 1) return;
            if (data.type === 'state' && PLAYER_STATES.includes(data.state)) {
                onPlayerState(data.state);
            } else if (data.type === 'audio' && typeof data.blocked === 'boolean') {
                // Well-formed and deliberately unused: the page shows its own click-for-sound pill.
            }
        }

        // A healthy page goes idle -> loading -> running. failed ends it before it
        // ran, even from idle: a script that cannot load (archive.org down or
        // blocked) fails the page before it reports loading. A game that ran never
        // shows "could not be started". Repeats, states out of order and anything
        // after failed change nothing, so a flood of messages cannot restart the
        // mute handshake or bring a failed game's bar back.
        function onPlayerState(state) {
            if (phase === 'failed') return;
            if (state === 'loading') {
                if (phase !== 'idle') return;
                phase = 'loading';
                if (muted !== urlMuted) postMute();
            } else if (state === 'running') {
                if (phase !== 'loading') return;
                phase = 'running';
                hideOverlay();
            } else if (state === 'failed') {
                if (phase === 'running') return;
                phase = 'failed';
                showHint('desktop.retro_games_player_failed');
            }
        }

        function postToFrame(message) {
            const target = frame.contentWindow;
            if (!disposed && target) target.postMessage(message, '*');
        }

        function postMute() {
            postToFrame({ rg: 1, type: 'mute', muted });
        }

        function focusFrame() {
            if (!disposed) frame.focus();
        }

        function hideOverlay() {
            clearTimeout(stallTimer);
            const hadFocus = overlay.contains(document.activeElement);
            overlay.hidden = true;
            overlay.classList.remove('is-stalled');
            hintActions.hidden = true;
            // A focused hint button disappears with the strip; keep the keyboard in the game.
            if (hadFocus) focusFrame();
        }

        // The hint strip: the text names what happened, the buttons offer a way out.
        function showHint(key) {
            clearTimeout(stallTimer);
            overlay.hidden = false;
            overlay.classList.add('is-stalled');
            overlayText.textContent = t(key);
            hintActions.hidden = false;
        }

        function armOverlay() {
            clearTimeout(stallTimer);
            overlay.hidden = false;
            overlay.classList.remove('is-stalled');
            overlayText.textContent = t('desktop.retro_games_player_loading');
            hintActions.hidden = true;
            stallTimer = setTimeout(() => {
                if (!disposed) showHint('desktop.retro_games_player_stalled');
            }, stallMs);
        }

        // Every start gets a fresh iframe element. Re-navigating the old one would
        // leave its document alive (and posting, through the same window object that
        // passes the source check in onMessage) until the new response commits, so a
        // late message of the old page would land on the new start. The old element
        // is removed: its window is gone and no longer matches frame.contentWindow.
        function start() {
            armOverlay();
            phase = 'idle';
            urlMuted = muted;
            const next = makeFrame();
            next.setAttribute('src', playerURL(game.identifier, muted, t('desktop.retro_games_click_for_sound')));
            if (frame) frame.replaceWith(next);
            else screen.prepend(next);
            frame = next;
        }

        function restartGame() {
            if (!disposed) start();
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

        // The icon shows the state (🔇 = muted), the label names what a click does.
        // No aria-pressed: a pressed button labelled "Sound on" would read as the opposite.
        function renderMute() {
            const label = muted ? t('desktop.retro_games_sound_on') : t('desktop.retro_games_sound_off');
            mute.textContent = muted ? '🔇' : '🔊';
            mute.classList.toggle('is-muted', muted);
            mute.setAttribute('aria-label', label);
            mute.title = label;
        }

        function toggleMute() {
            muted = !muted;
            writeMuted(muted);
            renderMute();
            postMute();
        }

        setFavorite(opts.favorite);
        setBroken(opts.broken);
        renderMute();
        window.addEventListener('message', onMessage);
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
                window.removeEventListener('message', onMessage);
                if (document.fullscreenElement === bezel && document.exitFullscreen) document.exitFullscreen().catch(() => {});
                frame.setAttribute('src', 'about:blank');
                frame.remove();
                stage.textContent = '';
            }
        };
    }

    window.RetroGamesPlayer = { mount, playerURL, SANDBOX, ALLOW };
})();
