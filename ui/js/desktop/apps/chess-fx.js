(function () {
    'use strict';

    const REDUCED_MOTION = typeof window.matchMedia === 'function'
        && window.matchMedia('(prefers-reduced-motion: reduce)').matches;

    function createChessFx(options = {}) {
        const root = options.root || null;
        const boardShell = options.boardShell || null;
        let timers = [];
        let disposed = false;

        function schedule(fn, ms) {
            if (disposed) return;
            const id = window.setTimeout(() => {
                timers = timers.filter(t => t !== id);
                if (!disposed) fn();
            }, ms);
            timers.push(id);
            return id;
        }

        function clearTimers() {
            timers.forEach(id => window.clearTimeout(id));
            timers = [];
        }

        function pulseClass(el, className, ms) {
            if (!el || disposed) return;
            el.classList.remove(className);
            // Force reflow so re-adding restarts animation.
            void el.offsetWidth;
            el.classList.add(className);
            schedule(() => {
                if (el && el.isConnected) el.classList.remove(className);
            }, ms);
        }

        function setThinking(active) {
            if (!root || disposed) return;
            root.classList.toggle('is-thinking', !!active);
            if (boardShell) boardShell.classList.toggle('is-thinking', !!active);
        }

        function setStatusTone(tone) {
            if (!root || disposed) return;
            ['is-your-turn', 'is-check', 'is-game-over'].forEach(cls => {
                root.classList.toggle(cls, cls === tone);
            });
            // is-thinking is primarily owned by setThinking(); allow status tone to reinforce it
            if (tone === 'is-thinking') root.classList.add('is-thinking');
        }

        function playMoveFx(kind) {
            if (!boardShell || disposed || REDUCED_MOTION) {
                if (boardShell && kind === 'check' && !REDUCED_MOTION) {
                    pulseClass(boardShell, 'is-check-fx', 520);
                }
                return;
            }
            pulseClass(boardShell, 'is-moved', 420);
            if (kind === 'capture') pulseClass(boardShell, 'is-capture-fx', 480);
            if (kind === 'check' || kind === 'check-capture') pulseClass(boardShell, 'is-check-fx', 560);
            if (kind === 'promote') pulseClass(boardShell, 'is-promote-fx', 520);
            if (kind === 'castle') pulseClass(boardShell, 'is-castle-fx', 420);
            spawnBurst(kind);
        }

        function spawnBurst(kind) {
            if (!boardShell || REDUCED_MOTION || disposed) return;
            const host = boardShell.querySelector('.vd-chess-fx-layer') || boardShell;
            const count = kind === 'capture' || kind === 'check-capture' ? 8 : kind === 'check' ? 6 : 0;
            if (!count) return;
            const burst = document.createElement('div');
            burst.className = 'vd-chess-burst' + (kind.indexOf('check') !== -1 ? ' is-danger' : ' is-spark');
            for (let i = 0; i < count; i++) {
                const p = document.createElement('span');
                p.style.setProperty('--i', String(i));
                p.style.setProperty('--a', String((360 / count) * i));
                burst.appendChild(p);
            }
            host.appendChild(burst);
            schedule(() => {
                if (burst.isConnected) burst.remove();
            }, 560);
        }

        function showResult(opts) {
            if (!root || disposed) return;
            dismissResult();
            const esc = opts.esc || (v => String(v == null ? '' : v));
            const title = opts.title || '';
            const detail = opts.detail || '';
            const win = !!opts.win;
            const draw = !!opts.draw;
            const primaryLabel = opts.primaryLabel || 'New game';
            const secondaryLabel = opts.secondaryLabel || 'OK';
            const onPrimary = typeof opts.onPrimary === 'function' ? opts.onPrimary : null;
            const onSecondary = typeof opts.onSecondary === 'function' ? opts.onSecondary : null;

            const dialog = document.createElement('div');
            dialog.className = 'vd-chess-modal vd-chess-result-modal' + (win ? ' is-win' : draw ? ' is-draw' : ' is-loss');
            dialog.innerHTML = [
                '<div class="vd-chess-modal-card vd-chess-result-card">',
                win && !REDUCED_MOTION ? '<div class="vd-chess-confetti" aria-hidden="true"></div>' : '',
                '<div class="vd-chess-result-badge" aria-hidden="true">' + (win ? '♛' : draw ? '½' : '♚') + '</div>',
                '<div class="vd-chess-modal-text vd-chess-result-title">' + esc(title) + '</div>',
                detail ? '<div class="vd-chess-result-detail">' + esc(detail) + '</div>' : '',
                '<div class="vd-chess-modal-actions">',
                '<button type="button" data-result="primary" class="is-primary">' + esc(primaryLabel) + '</button>',
                '<button type="button" data-result="secondary">' + esc(secondaryLabel) + '</button>',
                '</div></div>'
            ].join('');

            dialog.addEventListener('click', event => {
                const btn = event.target.closest('[data-result]');
                if (!btn) return;
                const which = btn.getAttribute('data-result');
                dialog.remove();
                if (which === 'primary' && onPrimary) onPrimary();
                else if (which === 'secondary' && onSecondary) onSecondary();
            });

            root.appendChild(dialog);
            if (win && !REDUCED_MOTION) spawnConfetti(dialog.querySelector('.vd-chess-confetti'));
            const focusBtn = dialog.querySelector('[data-result="primary"]');
            if (focusBtn) focusBtn.focus();
        }

        function spawnConfetti(host) {
            if (!host) return;
            const colors = ['#f2b84b', '#27c7a6', '#ff8066', '#6ea8fe', '#ffffff'];
            for (let i = 0; i < 18; i++) {
                const bit = document.createElement('span');
                bit.style.setProperty('--i', String(i));
                bit.style.setProperty('--c', colors[i % colors.length]);
                bit.style.setProperty('--x', String((Math.random() * 120) - 60));
                bit.style.setProperty('--d', String(0.45 + Math.random() * 0.55));
                host.appendChild(bit);
            }
        }

        function dismissResult() {
            if (!root) return;
            root.querySelectorAll('.vd-chess-result-modal').forEach(el => el.remove());
        }

        function dispose() {
            disposed = true;
            clearTimers();
            dismissResult();
            if (root) {
                root.classList.remove('is-thinking', 'is-your-turn', 'is-check', 'is-game-over');
            }
            if (boardShell) {
                boardShell.classList.remove('is-thinking', 'is-moved', 'is-capture-fx', 'is-check-fx', 'is-promote-fx', 'is-castle-fx');
                boardShell.querySelectorAll('.vd-chess-burst').forEach(el => el.remove());
            }
        }

        return {
            setThinking,
            setStatusTone,
            playMoveFx,
            showResult,
            dismissResult,
            dispose
        };
    }

    // --- Web Audio synthesized chess sounds (no sample files) ---
    function createChessAudio() {
        let actx = null;
        function ensure() {
            if (actx) return actx;
            try { actx = new (window.AudioContext || window.webkitAudioContext)(); } catch (e) { return null; }
            return actx;
        }
        function tone(freq, dur, type, vol) {
            const a = ensure(); if (!a) return;
            if (a.state === 'suspended') a.resume();
            const o = a.createOscillator(), g = a.createGain();
            o.type = type || 'sine'; o.frequency.value = freq;
            g.gain.setValueAtTime(Math.max(0.001, vol || 0.2), a.currentTime);
            g.gain.exponentialRampToValueAtTime(0.001, a.currentTime + dur);
            o.connect(g).connect(a.destination);
            o.start(); o.stop(a.currentTime + dur + 0.01);
        }
        function woodThunk(freq) {
            const a = ensure(); if (!a) return;
            const buf = a.createBuffer(1, a.sampleRate * 0.05, a.sampleRate), d = buf.getChannelData(0);
            for (let i = 0; i < d.length; i++) d[i] = (Math.random() * 2 - 1) * Math.pow(1 - i / d.length, 2);
            const s = a.createBufferSource(), f = a.createBiquadFilter(), g = a.createGain();
            s.buffer = buf; f.type = 'bandpass'; f.frequency.value = freq || 600; f.Q.value = 2;
            g.gain.setValueAtTime(0.15, a.currentTime); g.gain.exponentialRampToValueAtTime(0.001, a.currentTime + 0.05);
            s.connect(f).connect(g).connect(a.destination); s.start();
        }
        return {
            move() { woodThunk(500); tone(300, 0.06, 'sine', 0.1); },
            capture() { woodThunk(280); tone(180, 0.08, 'sawtooth', 0.12); },
            check() { tone(880, 0.1, 'triangle', 0.18); setTimeout(() => tone(660, 0.1, 'triangle', 0.15), 80); },
            castle() { woodThunk(500); setTimeout(() => woodThunk(500), 100); },
            promote() { [523, 659, 784, 1047].forEach((f, i) => setTimeout(() => tone(f, 0.1, 'sine', 0.15), i * 60)); },
            gameOver(win) { const notes = win ? [523, 659, 784, 1047] : [392, 330, 262, 196]; notes.forEach((f, i) => setTimeout(() => tone(f, 0.2, 'triangle', 0.18), i * 120)); }
        };
    }

    function normalizeBoardSkin(value) {
        const skin = String(value || 'green');
        if (skin === 'blue' || skin === 'chess-club' || skin === 'default' || skin === 'green') return skin;
        return 'green';
    }

    function loadBoardSkin() {
        try {
            return normalizeBoardSkin(window.localStorage.getItem('aurago.desktop.chess.boardSkin'));
        } catch (e) {
            return 'green';
        }
    }

    function saveBoardSkin(skin) {
        try {
            window.localStorage.setItem('aurago.desktop.chess.boardSkin', normalizeBoardSkin(skin));
        } catch (e) {}
    }

    function applyBoardSkin(state) {
        if (!state || !state.board || !state.board.props || !state.board.props.style) return;
        const skin = normalizeBoardSkin(state.boardSkin);
        const border = state.board.props.style.borderType || 'frame';
        state.board.props.style.cssClass = skin;
        try {
            const svg = state.refs && state.refs.board && state.refs.board.querySelector('svg.cm-chessboard');
            if (svg) svg.setAttribute('class', 'cm-chessboard border-type-' + border + ' ' + skin);
        } catch (e) {}
    }

    function updateMaterialBar(refs, diff) {
        if (!refs) return;
        const fill = refs.materialFill;
        const score = refs.materialBarScore;
        if (!fill || !score) return;
        const value = Number(diff) || 0;
        const clamped = Math.max(-12, Math.min(12, value));
        const pct = (Math.abs(clamped) / 12) * 50;
        if (clamped >= 0) {
            fill.style.left = '50%';
            fill.style.width = pct + '%';
            fill.classList.remove('is-black');
        } else {
            fill.style.left = (50 - pct) + '%';
            fill.style.width = pct + '%';
            fill.classList.add('is-black');
        }
        score.textContent = (value > 0 ? '+' : '') + String(value);
    }

    function renderChessTemplate(ctx, pieceGlyph) {
        const esc = ctx.esc;
        const t = ctx.t;
        const king = (pieceGlyph && pieceGlyph.k) || '♚';
        const icon = (k, fb, cls) => ctx.iconMarkup(k, fb, cls || 'vd-chess-action-icon', 14);
        return '' +
            '<div class="vd-chess">' +
            '<section class="vd-chess-board-pane">' +
            '<div class="vd-chess-material-bar" data-material-bar>' +
            '<span class="vd-chess-material-side" aria-hidden="true">' + king + '</span>' +
            '<div class="vd-chess-material-track" title="' + esc(t('desktop.chess_material')) + '">' +
            '<div class="vd-chess-material-fill" data-material-fill></div></div>' +
            '<span class="vd-chess-material-score" data-material-bar-score>0</span></div>' +
            '<div class="vd-chess-board-shell">' +
            '<div class="vd-chess-fx-layer" data-fx-layer aria-hidden="true"></div>' +
            '<div class="vd-chess-board" data-chess-board></div></div>' +
            '<div class="vd-chess-ribbon">' +
            '<div class="vd-chess-status" data-status>' + esc(t('desktop.chess_loading')) + '</div>' +
            '<div class="vd-chess-comment" data-comment></div></div></section>' +
            '<aside class="vd-chess-side">' +
            '<div class="vd-chess-clocks" data-clocks hidden>' +
            '<div class="vd-chess-clock" data-clock="white"><span class="vd-chess-clock-side">' + king + '</span>' +
            '<span class="vd-chess-clock-time" data-white-time>--:--</span></div>' +
            '<div class="vd-chess-clock" data-clock="black"><span class="vd-chess-clock-side">' + king + '</span>' +
            '<span class="vd-chess-clock-time" data-black-time>--:--</span></div></div>' +
            '<div class="vd-chess-controls">' +
            '<div class="vd-chess-field"><label for="chess-mode">' + esc(t('desktop.chess_mode')) + '</label>' +
            '<select id="chess-mode" data-mode>' +
            '<option value="computer">' + esc(t('desktop.chess_mode_computer')) + '</option>' +
            '<option value="agent">' + esc(t('desktop.chess_mode_agent')) + '</option>' +
            '<option value="local">' + esc(t('desktop.chess_mode_local')) + '</option></select></div>' +
            '<div class="vd-chess-field" data-strength-wrap><label for="chess-strength">' + esc(t('desktop.chess_strength')) +
            ' <span data-strength-value>8</span></label><input id="chess-strength" data-strength type="range" min="1" max="20" value="8"></div>' +
            '<div class="vd-chess-field" data-clock-wrap><label for="chess-clock-select">' + esc(t('desktop.chess_clock')) + '</label>' +
            '<select id="chess-clock-select" data-clock-select>' +
            '<option value="0">' + esc(t('desktop.chess_clock_off')) + '</option>' +
            '<option value="180">' + esc(t('desktop.chess_clock_3')) + '</option>' +
            '<option value="300">' + esc(t('desktop.chess_clock_5')) + '</option>' +
            '<option value="600">' + esc(t('desktop.chess_clock_10')) + '</option></select></div>' +
            '<div class="vd-chess-field"><label for="chess-board-skin">' + esc(t('desktop.chess_board_skin')) + '</label>' +
            '<select id="chess-board-skin" data-board-skin>' +
            '<option value="green">' + esc(t('desktop.chess_skin_green')) + '</option>' +
            '<option value="blue">' + esc(t('desktop.chess_skin_blue')) + '</option>' +
            '<option value="chess-club">' + esc(t('desktop.chess_skin_wood')) + '</option>' +
            '<option value="default">' + esc(t('desktop.chess_skin_classic')) + '</option></select></div>' +
            '<div class="vd-chess-field"><span class="vd-chess-label">' + esc(t('desktop.chess_player_color')) + '</span>' +
            '<div class="vd-chess-segment" data-color-group>' +
            '<button type="button" data-color="white" class="active">' + esc(t('desktop.chess_color_white')) + '</button>' +
            '<button type="button" data-color="black">' + esc(t('desktop.chess_color_black')) + '</button></div></div>' +
            '<div class="vd-chess-actions">' +
            '<button type="button" class="vd-chess-primary" data-new-game>' + icon('refresh', 'N') + '<span>' + esc(t('desktop.chess_new_game')) + '</span></button>' +
            '<button type="button" data-undo>' + icon('undo', 'U') + '<span>' + esc(t('desktop.chess_undo')) + '</span></button>' +
            '<button type="button" data-flip>' + icon('refresh', 'F') + '<span>' + esc(t('desktop.chess_flip')) + '</span></button>' +
            '<button type="button" data-hint>' + icon('help', 'H') + '<span>' + esc(t('desktop.chess_hint')) + '</span></button>' +
            '<button type="button" data-resign>' + icon('stop', 'R') + '<span>' + esc(t('desktop.chess_resign')) + '</span></button>' +
            '<button type="button" data-draw>' + icon('check-square', 'D') + '<span>' + esc(t('desktop.chess_offer_draw')) + '</span></button></div>' +
            '<div class="vd-chess-replay" data-replay hidden>' +
            '<button type="button" data-review-first title="' + esc(t('desktop.chess_first')) + '">' + icon('chevron-left', '|<', 'vd-chess-replay-icon') + '</button>' +
            '<button type="button" data-review-prev title="' + esc(t('desktop.chess_prev')) + '">' + icon('chevron-left', '<', 'vd-chess-replay-icon') + '</button>' +
            '<span class="vd-chess-review-label" data-review-label>' + esc(t('desktop.chess_live')) + '</span>' +
            '<button type="button" data-review-next title="' + esc(t('desktop.chess_next')) + '">' + icon('chevron-right', '>', 'vd-chess-replay-icon') + '</button>' +
            '<button type="button" data-review-last title="' + esc(t('desktop.chess_last')) + '">' + icon('chevron-right', '>|', 'vd-chess-replay-icon') + '</button></div></div>' +
            '<div class="vd-chess-tabs" data-tabs>' +
            '<button type="button" class="active" data-tab="moves">' + esc(t('desktop.chess_tab_moves')) + '</button>' +
            '<button type="button" data-tab="captured">' + esc(t('desktop.chess_tab_captured')) + '</button>' +
            '<button type="button" data-tab="info">' + esc(t('desktop.chess_tab_info')) + '</button></div>' +
            '<div class="vd-chess-moves" data-panel="moves"><ol data-moves></ol></div>' +
            '<div class="vd-chess-captured-panel" data-panel="captured" hidden>' +
            '<div class="vd-chess-captured-row"><span class="vd-chess-captured-label">' + esc(t('desktop.chess_color_white')) + '</span>' +
            '<div class="vd-chess-captured-pieces" data-captured-white></div></div>' +
            '<div class="vd-chess-captured-row"><span class="vd-chess-captured-label">' + esc(t('desktop.chess_color_black')) + '</span>' +
            '<div class="vd-chess-captured-pieces" data-captured-black></div></div></div>' +
            '<div class="vd-chess-info-panel" data-panel="info" hidden>' +
            '<div class="vd-chess-info-row"><span>' + esc(t('desktop.chess_material')) + '</span><span data-material>0</span></div>' +
            '<div class="vd-chess-info-row"><span>' + esc(t('desktop.chess_eval')) + '</span><span data-eval>' + esc(t('desktop.chess_even')) + '</span></div>' +
            '<div class="vd-chess-info-row"><span>' + esc(t('desktop.chess_hint')) + '</span><span data-hint-text>' + esc(t('desktop.chess_no_moves')) + '</span></div>' +
            '</div></aside></div>';
    }

    function showChessModal(root, message, buttons, escFn) {
        if (!root) return;
        const existing = root.querySelector('.vd-chess-modal:not(.vd-chess-result-modal)');
        if (existing) existing.remove();
        const esc = escFn || (v => String(v == null ? '' : v));
        const dialog = document.createElement('div');
        dialog.className = 'vd-chess-modal';
        const btnHtml = (buttons || []).map((b, i) =>
            '<button type="button" data-idx="' + i + '" class="' + (b.primary ? 'is-primary' : '') + '">' + esc(b.label) + '</button>'
        ).join('');
        dialog.innerHTML = '<div class="vd-chess-modal-card"><div class="vd-chess-modal-text">' + esc(message) +
            '</div><div class="vd-chess-modal-actions">' + btnHtml + '</div></div>';
        dialog.addEventListener('click', event => {
            const button = event.target.closest('[data-idx]');
            if (!button) return;
            const idx = Number(button.dataset.idx);
            dialog.remove();
            const action = buttons[idx] && buttons[idx].action;
            if (typeof action === 'function') action();
        });
        root.appendChild(dialog);
    }

    window.createChessFx = createChessFx;
    window.createChessAudio = createChessAudio;
    window.renderChessTemplate = renderChessTemplate;
    window.showChessModal = showChessModal;
    window.ChessBoardSkin = { normalize: normalizeBoardSkin, load: loadBoardSkin, save: saveBoardSkin, apply: applyBoardSkin };
    window.ChessMaterialBar = { update: updateMaterialBar };
})();
