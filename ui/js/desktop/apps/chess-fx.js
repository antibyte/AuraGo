(function () {
    'use strict';

    const REDUCED_MOTION = typeof window.matchMedia === 'function'
        && window.matchMedia('(prefers-reduced-motion: reduce)').matches;
    const SOUND_STORAGE_KEY = 'aurago.desktop.chess.soundMuted';

    function loadSoundMuted() {
        try {
            return window.localStorage.getItem(SOUND_STORAGE_KEY) === '1';
        } catch (e) {
            return false;
        }
    }

    function saveSoundMuted(muted) {
        try {
            window.localStorage.setItem(SOUND_STORAGE_KEY, muted ? '1' : '0');
        } catch (e) {}
    }

    function squarePercent(square, orientation) {
        if (!square || square.length < 2) return { x: 50, y: 50 };
        const file = 'abcdefgh'.indexOf(square.charAt(0).toLowerCase());
        const rank = Number(square.charAt(1)) - 1;
        if (file < 0 || rank < 0 || rank > 7) return { x: 50, y: 50 };
        const flipped = orientation === 'black';
        const col = flipped ? (7 - file) : file;
        const row = flipped ? rank : (7 - rank);
        return {
            x: ((col + 0.5) / 8) * 100,
            y: ((row + 0.5) / 8) * 100
        };
    }

    function createChessFx(options = {}) {
        const root = options.root || null;
        const boardShell = options.boardShell || null;
        const t = typeof options.t === 'function' ? options.t : (key => key);
        let timers = [];
        let activeCleanups = [];
        let disposed = false;

        function schedule(fn, ms) {
            if (disposed) return;
            const id = window.setTimeout(() => {
                timers = timers.filter(item => item !== id);
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
            if (tone === 'is-thinking') root.classList.add('is-thinking');
        }

        function playMoveFx(kind, square, orientation) {
            if (!boardShell || disposed || REDUCED_MOTION) return;
            pulseClass(boardShell, 'is-moved', 420);
            if (kind === 'capture' || kind === 'check-capture') pulseClass(boardShell, 'is-capture-fx', 480);
            if (kind === 'check' || kind === 'check-capture') pulseClass(boardShell, 'is-check-fx', 560);
            if (kind === 'promote') pulseClass(boardShell, 'is-promote-fx', 540);
            if (kind === 'castle') pulseClass(boardShell, 'is-castle-fx', 440);
            spawnBurst(kind, square, orientation);
        }

        function spawnBurst(kind, square, orientation) {
            if (!boardShell || REDUCED_MOTION || disposed) return;
            const frame = boardShell.querySelector('[data-chess-board-frame]');
            const host = boardShell.querySelector('.vd-chess-fx-layer') || boardShell;
            const count = kind === 'capture' || kind === 'check-capture' ? 10 : kind === 'promote' ? 12 : kind === 'check' ? 8 : kind === 'castle' ? 6 : 0;
            if (!count) return;

            let leftPct = 50;
            let topPct = 50;
            const inner = frame ? frame.querySelector('[data-chess-board]') : null;
            if (square && inner) {
                const shellRect = host.getBoundingClientRect();
                const innerRect = inner.getBoundingClientRect();
                if (shellRect.width > 0 && shellRect.height > 0 && innerRect.width > 0) {
                    const sq = squarePercent(square, orientation);
                    const px = innerRect.left - shellRect.left + (sq.x / 100) * innerRect.width;
                    const py = innerRect.top - shellRect.top + (sq.y / 100) * innerRect.height;
                    leftPct = Math.max(8, Math.min(92, (px / shellRect.width) * 100));
                    topPct = Math.max(8, Math.min(92, (py / shellRect.height) * 100));
                }
            }

            const variant = kind.indexOf('check') !== -1 ? ' is-danger' : kind === 'promote' ? ' is-gold' : ' is-spark';
            const burst = document.createElement('div');
            burst.className = 'vd-chess-burst' + variant;
            burst.style.left = leftPct + '%';
            burst.style.top = topPct + '%';

            const ring = document.createElement('i');
            ring.className = 'vd-chess-burst-ring';
            burst.appendChild(ring);

            for (let i = 0; i < count; i++) {
                const p = document.createElement('span');
                p.style.setProperty('--i', String(i));
                p.style.setProperty('--a', String(Math.round((360 / count) * i + (i % 2 ? 9 : -9))));
                burst.appendChild(p);
            }
            host.appendChild(burst);
            schedule(() => {
                if (burst.isConnected) burst.remove();
            }, 620);
        }

        function showResult(opts) {
            if (!root || disposed) return;
            dismissResult();
            const esc = opts.esc || (v => String(v == null ? '' : v));
            const title = opts.title || '';
            const detail = opts.detail || '';
            const win = !!opts.win;
            const draw = !!opts.draw;
            const primaryLabel = opts.primaryLabel || t('desktop.chess_new_game');
            const secondaryLabel = opts.secondaryLabel || t('desktop.ok');
            const onPrimary = typeof opts.onPrimary === 'function' ? opts.onPrimary : null;
            const onSecondary = typeof opts.onSecondary === 'function' ? opts.onSecondary : null;

            const dialog = document.createElement('div');
            dialog.className = 'vd-chess-modal vd-chess-result-modal' + (win ? ' is-win' : draw ? ' is-draw' : ' is-loss');
            dialog.setAttribute('role', 'dialog');
            dialog.setAttribute('aria-modal', 'true');
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

            const closeWith = cb => {
                window.removeEventListener('keydown', onKey, true);
                if (dialog.isConnected) dialog.remove();
                if (typeof cb === 'function') cb();
            };
            const onKey = event => {
                if (event.key === 'Escape') {
                    event.preventDefault();
                    event.stopPropagation();
                    closeWith(onSecondary);
                }
            };
            window.addEventListener('keydown', onKey, true);
            activeCleanups.push(() => window.removeEventListener('keydown', onKey, true));

            dialog.addEventListener('click', event => {
                const btn = event.target.closest('[data-result]');
                if (!btn) {
                    if (event.target === dialog) closeWith(onSecondary);
                    return;
                }
                const which = btn.getAttribute('data-result');
                closeWith(which === 'primary' ? onPrimary : onSecondary);
            });

            root.appendChild(dialog);
            if (win && !REDUCED_MOTION) spawnConfetti(dialog.querySelector('.vd-chess-confetti'));
            const focusBtn = dialog.querySelector('[data-result="primary"]');
            if (focusBtn) focusBtn.focus();
        }

        function spawnConfetti(host) {
            if (!host) return;
            const colors = ['#f2b84b', '#27c7a6', '#ff8066', '#6ea8fe', '#ffffff'];
            for (let i = 0; i < 22; i++) {
                const bit = document.createElement('span');
                bit.style.setProperty('--i', String(i));
                bit.style.setProperty('--c', colors[i % colors.length]);
                bit.style.setProperty('--x', String((Math.random() * 140) - 70));
                bit.style.setProperty('--d', String(0.5 + Math.random() * 0.6));
                host.appendChild(bit);
            }
        }

        function dismissResult() {
            if (!root) return;
            activeCleanups.forEach(fn => { try { fn(); } catch (e) {} });
            activeCleanups = [];
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

    // --- Physical-modeling Staunton wood & baize Web Audio engine + Phonon voice channel ---
    function createChessAudio(options = {}) {
        let actx = null;
        let masterBus = null;
        let roomConvolver = null;
        let roomSendGain = null;
        let voiceSource = null;
        let voiceOnEnd = null;
        let muted = !!options.muted;
        let disposed = false;
        const timers = new Set();

        const later = (action, delay) => {
            if (disposed) return;
            const timer = setTimeout(() => {
                timers.delete(timer);
                if (!disposed) action();
            }, delay);
            timers.add(timer);
        };

        function buildWoodRoomImpulse(a) {
            const len = Math.max(1, Math.floor(a.sampleRate * 0.16));
            const buf = a.createBuffer(2, len, a.sampleRate);
            const taps = [
                { s: Math.floor(0.0073 * a.sampleRate), g: 0.42 },
                { s: Math.floor(0.0139 * a.sampleRate), g: -0.28 },
                { s: Math.floor(0.0224 * a.sampleRate), g: 0.21 },
                { s: Math.floor(0.0341 * a.sampleRate), g: 0.12 }
            ];
            for (let c = 0; c < 2; c++) {
                const ch = buf.getChannelData(c);
                for (let i = 0; i < len; i++) {
                    const t = i / len;
                    ch[i] = (Math.random() * 2 - 1) * Math.pow(1 - t, 4.2) * 0.16;
                }
                taps.forEach(tap => {
                    const idx = Math.min(len - 1, tap.s + c * 48);
                    ch[idx] += tap.g;
                });
            }
            return buf;
        }

        function ensure() {
            if (disposed) return null;
            if (actx) {
                if (actx.state === 'suspended') actx.resume().catch(() => {});
                return actx;
            }
            const Ctor = window.AudioContext || window.webkitAudioContext;
            if (!Ctor) return null;
            try {
                actx = new Ctor();
                masterBus = actx.createGain();
                masterBus.gain.value = 0.86;
                if (typeof actx.createDynamicsCompressor === 'function') {
                    const comp = actx.createDynamicsCompressor();
                    comp.threshold.value = -10;
                    comp.knee.value = 12;
                    comp.ratio.value = 3.2;
                    comp.attack.value = 0.003;
                    comp.release.value = 0.12;
                    masterBus.connect(comp);
                    comp.connect(actx.destination);
                } else {
                    masterBus.connect(actx.destination);
                }
                if (typeof actx.createConvolver === 'function') {
                    roomConvolver = actx.createConvolver();
                    roomConvolver.buffer = buildWoodRoomImpulse(actx);
                    roomSendGain = actx.createGain();
                    roomSendGain.gain.value = 0.16;
                    roomConvolver.connect(roomSendGain);
                    roomSendGain.connect(masterBus);
                }
                if (actx.state === 'suspended') actx.resume().catch(() => {});
            } catch (e) {
                actx = null;
                return null;
            }
            return actx;
        }

        function routeOut(a, node, pan) {
            let tail = node;
            if (pan && typeof a.createStereoPanner === 'function') {
                const panner = a.createStereoPanner();
                panner.pan.value = Math.max(-0.6, Math.min(0.6, pan));
                tail.connect(panner);
                tail = panner;
            }
            tail.connect(masterBus || a.destination);
            if (roomConvolver) tail.connect(roomConvolver);
        }

        // Physical Staunton piece impact: baize brush + boxwood rim click + 4 wood modes.
        function woodImpact(opts = {}) {
            if (muted || disposed) return;
            const a = ensure();
            if (!a) return;
            const now = a.currentTime;
            const baseFreq = (opts.baseFreq || 470) * (0.97 + Math.random() * 0.06);
            const weight = (opts.weight || 1.0) * (0.95 + Math.random() * 0.1);
            const out = a.createGain();
            out.gain.value = 1;
            routeOut(a, out, opts.pan || 0);

            // 1. Baize brush + crisp wood rim transient
            const len = Math.max(32, Math.floor(a.sampleRate * 0.022));
            const feltBuf = a.createBuffer(1, len, a.sampleRate);
            const clickBuf = a.createBuffer(1, len, a.sampleRate);
            const fd = feltBuf.getChannelData(0);
            const cd = clickBuf.getChannelData(0);
            let pink = 0;
            for (let i = 0; i < len; i++) {
                const w = Math.random() * 2 - 1;
                pink = 0.78 * pink + 0.22 * w;
                const t = i / len;
                fd[i] = pink * Math.min(1, i / (0.0012 * a.sampleRate)) * Math.pow(1 - t, 2.4);
                cd[i] = w * Math.min(1, i / (0.0006 * a.sampleRate)) * Math.exp(-i / (0.0022 * a.sampleRate));
            }
            const feltSrc = a.createBufferSource();
            feltSrc.buffer = feltBuf;
            const feltFilter = a.createBiquadFilter();
            feltFilter.type = 'bandpass';
            feltFilter.frequency.value = opts.feltFreq || 1850;
            feltFilter.Q.value = 1.6;
            const feltGain = a.createGain();
            feltGain.gain.value = (opts.feltGain == null ? 0.24 : opts.feltGain) * weight;
            feltSrc.connect(feltFilter).connect(feltGain).connect(out);
            feltSrc.start(now);

            const clickSrc = a.createBufferSource();
            clickSrc.buffer = clickBuf;
            const clickFilter = a.createBiquadFilter();
            clickFilter.type = 'bandpass';
            clickFilter.frequency.value = opts.rimFreq || 2650;
            clickFilter.Q.value = 2.2;
            const clickGain = a.createGain();
            clickGain.gain.value = (opts.rimGain == null ? 0.28 : opts.rimGain) * weight;
            clickSrc.connect(clickFilter).connect(clickGain).connect(out);
            clickSrc.start(now);

            // 2. Walnut board mode + 3 inharmonic boxwood resonators
            const modes = [
                { f0: 175, f1: 112, amp: 0.17 * weight, dur: 0.085 },
                { f0: baseFreq, f1: baseFreq * 0.97, amp: 0.28 * weight, dur: 0.075 },
                { f0: baseFreq * 2.18, f1: baseFreq * 2.14, amp: 0.19 * weight, dur: 0.058 },
                { f0: baseFreq * 3.63, f1: baseFreq * 3.58, amp: 0.10 * weight, dur: 0.042 }
            ];
            modes.forEach(m => {
                const osc = a.createOscillator();
                const g = a.createGain();
                osc.type = 'sine';
                osc.frequency.setValueAtTime(m.f0, now);
                osc.frequency.exponentialRampToValueAtTime(Math.max(40, m.f1), now + m.dur);
                g.gain.setValueAtTime(0.0001, now);
                g.gain.linearRampToValueAtTime(Math.max(0.001, m.amp), now + 0.0016);
                g.gain.exponentialRampToValueAtTime(0.0001, now + m.dur);
                osc.connect(g).connect(out);
                osc.start(now);
                osc.stop(now + m.dur + 0.012);
            });
        }

        function warmNote(freq, dur, vol, lpFreq, pan) {
            if (muted || disposed) return;
            const a = ensure();
            if (!a) return;
            const now = a.currentTime;
            const filter = a.createBiquadFilter();
            filter.type = 'lowpass';
            filter.frequency.value = lpFreq || 2700;
            filter.Q.value = 0.65;
            const env = a.createGain();
            const peak = Math.max(0.002, vol || 0.2);
            env.gain.setValueAtTime(0.0001, now);
            env.gain.linearRampToValueAtTime(peak, now + 0.004);
            env.gain.exponentialRampToValueAtTime(0.0001, now + dur);
            filter.connect(env);
            routeOut(a, env, pan || 0);

            [
                { ratio: 1, type: 'sine', gain: 0.78 },
                { ratio: 2, type: 'triangle', gain: 0.17 },
                { ratio: 3.01, type: 'sine', gain: 0.05 }
            ].forEach(p => {
                const o = a.createOscillator();
                const pg = a.createGain();
                o.type = p.type;
                o.frequency.setValueAtTime(freq * p.ratio, now);
                pg.gain.value = p.gain;
                o.connect(pg).connect(filter);
                o.start(now);
                o.stop(now + dur + 0.02);
            });
        }

        function stopVoice() {
            if (voiceSource) {
                const src = voiceSource;
                const done = voiceOnEnd;
                voiceSource = null;
                voiceOnEnd = null;
                src.onended = null;
                try { src.stop(); } catch (e) {}
                try { src.disconnect(); } catch (e) {}
                if (typeof done === 'function') {
                    try { done(); } catch (e) {}
                }
            }
        }

        function computeVoiceGain(buffer) {
            let peak = 0;
            for (let c = 0; c < buffer.numberOfChannels; c++) {
                const ch = buffer.getChannelData(c);
                for (let i = 0; i < ch.length; i += 4) {
                    const v = Math.abs(ch[i]);
                    if (v > peak) peak = v;
                }
            }
            if (peak <= 0.001) return 1.0;
            return Math.max(0.45, Math.min(2.2, 0.82 / peak));
        }

        function playVoice(base64Audio, onEnded) {
            if (disposed || !base64Audio) {
                if (typeof onEnded === 'function') onEnded();
                return;
            }
            const a = ensure();
            if (!a) {
                if (typeof onEnded === 'function') onEnded();
                return;
            }
            stopVoice();
            let bytes;
            try {
                const bin = window.atob(base64Audio);
                const arr = new Uint8Array(bin.length);
                for (let i = 0; i < bin.length; i++) arr[i] = bin.charCodeAt(i);
                bytes = arr.buffer;
            } catch (e) {
                if (typeof onEnded === 'function') onEnded();
                return;
            }
            a.decodeAudioData(bytes).then(buffer => {
                if (disposed || !actx) {
                    if (typeof onEnded === 'function') onEnded();
                    return;
                }
                stopVoice();
                const src = a.createBufferSource();
                src.buffer = buffer;
                const hp = a.createBiquadFilter();
                hp.type = 'highpass';
                hp.frequency.value = 85;
                const g = a.createGain();
                g.gain.value = computeVoiceGain(buffer);
                src.connect(hp).connect(g).connect(masterBus || a.destination);
                voiceSource = src;
                voiceOnEnd = onEnded || null;
                src.onended = () => {
                    if (voiceSource === src) {
                        voiceSource = null;
                        voiceOnEnd = null;
                    }
                    try { src.disconnect(); } catch (e) {}
                    if (typeof onEnded === 'function') onEnded();
                };
                src.start();
            }).catch(() => {
                if (typeof onEnded === 'function') onEnded();
            });
        }

        return {
            isMuted: () => muted,
            setMuted(next) { muted = !!next; saveSoundMuted(muted); return muted; },
            select() { woodImpact({ baseFreq: 640, weight: 0.28, feltGain: 0.34, rimGain: 0.14 }); },
            move(pan) { woodImpact({ baseFreq: 470, weight: 0.96, pan: pan || 0 }); },
            capture(pan) {
                woodImpact({ baseFreq: 790, weight: 0.66, feltFreq: 2350, rimFreq: 3100, pan: pan || 0.06 });
                later(() => woodImpact({ baseFreq: 385, weight: 1.28, feltFreq: 1450, pan: pan || -0.04 }), 28);
            },
            check() {
                woodImpact({ baseFreq: 495, weight: 0.85 });
                warmNote(587.33, 0.17, 0.22, 2600);
                later(() => warmNote(880.0, 0.23, 0.19, 2850), 82);
            },
            castle() {
                woodImpact({ baseFreq: 425, weight: 1.05, pan: -0.14 });
                later(() => woodImpact({ baseFreq: 525, weight: 0.92, pan: 0.14 }), 106);
            },
            promote() {
                woodImpact({ baseFreq: 515, weight: 0.8 });
                [523.25, 659.25, 783.99, 1046.5].forEach((f, i) => later(() => warmNote(f, 0.22, 0.19, 3400), i * 60));
            },
            illegal() {
                woodImpact({ baseFreq: 220, weight: 0.42, rimGain: 0.08 });
                later(() => woodImpact({ baseFreq: 185, weight: 0.36, rimGain: 0.06 }), 65);
            },
            lowTime() {
                woodImpact({ baseFreq: 1240, weight: 0.22, feltGain: 0.05, rimFreq: 2900, rimGain: 0.34 });
            },
            gameOver(outcome) {
                const kind = outcome === 'draw' ? 'draw' : outcome ? 'win' : 'loss';
                if (kind === 'win') {
                    warmNote(261.63, 0.55, 0.15, 1800);
                    [523.25, 659.25, 783.99, 987.77, 1046.5].forEach((f, i) => later(() => warmNote(f, 0.30, 0.19, 3400), i * 85));
                } else if (kind === 'draw') {
                    warmNote(392.0, 0.26, 0.17, 2400, -0.08);
                    later(() => { warmNote(329.63, 0.34, 0.16, 2400, 0.08); warmNote(523.25, 0.34, 0.14, 2400); }, 135);
                } else {
                    [440.0, 349.23, 293.66, 220.0].forEach((f, i) => later(() => warmNote(f, 0.26, 0.17, 1900), i * 115));
                }
            },
            playVoice,
            stopVoice,
            dispose() {
                if (disposed) return;
                disposed = true;
                stopVoice();
                timers.forEach(clearTimeout);
                timers.clear();
                const current = actx;
                actx = null;
                masterBus = null;
                roomConvolver = null;
                roomSendGain = null;
                if (current) current.close().catch(() => {});
            }
        };
    }

    const BOARD_SKINS = ['green', 'blue', 'chess-club', 'default', 'marble', 'slate'];

    function normalizeBoardSkin(value) {
        const skin = String(value || 'green');
        return BOARD_SKINS.indexOf(skin) !== -1 ? skin : 'green';
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
        const border = state.board.props.style.borderType || 'thin';
        state.board.props.style.cssClass = skin;
        /* The stylesheet derives squares, rim, inlay and markers from the skin attribute. */
        if (state.refs && state.refs.root) state.refs.root.dataset.chessSkin = skin;
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
        /* One player plate: avatar, name, thinking dots, captured material, lead and clock. */
        const plate = side =>
            '<div class="vd-chess-plate" data-plate="' + side + '" role="group">' +
            '<span class="vd-chess-plate-avatar" data-plate-avatar aria-hidden="true"></span>' +
            '<span class="vd-chess-plate-name" data-plate-name></span>' +
            '<span class="vd-chess-plate-thinking" aria-hidden="true"><i></i><i></i><i></i></span>' +
            '<span class="vd-chess-plate-tray" data-plate-tray aria-hidden="true"></span>' +
            '<span class="vd-chess-plate-lead" data-plate-lead title="' + esc(t('desktop.chess_material')) + '" hidden></span>' +
            '<span class="vd-chess-plate-clock" data-plate-clock role="timer" aria-label="' + esc(t('desktop.chess_clock')) + '" hidden></span>' +
            '</div>';
        return '' +
            '<div class="vd-chess" tabindex="-1">' +
            '<section class="vd-chess-board-pane">' +
            '<div class="vd-chess-pane-head">' +
            '<div class="vd-chess-head-tools">' +
            '<div class="vd-chess-view-switch" data-chess-view-toggle role="group" aria-label="' + esc(t('desktop.chess_view_mode')) + '">' +
            '<button type="button" data-chess-view="2d" class="is-active" aria-pressed="true">' + esc(t('desktop.chess_view_2d')) + '</button>' +
            '<button type="button" data-chess-view="3d" aria-pressed="false">' + esc(t('desktop.chess_view_3d')) + '</button></div>' +
            '<button type="button" class="vd-chess-head-pill is-active" data-sound-toggle aria-pressed="true" title="' + esc(t('desktop.chess_sound_on')) + '" aria-label="' + esc(t('desktop.chess_sound')) + '">' +
            icon('speaker', '🔊', 'vd-chess-action-icon') + '</button>' +
            '<button type="button" class="vd-chess-head-pill vd-chess-commentary-btn" data-commentary-toggle aria-pressed="false" title="' + esc(t('desktop.chess_commentary_hint')) + '">' +
            icon('chat', '💬', 'vd-chess-action-icon') +
            '<span class="vd-chess-pill-label">' + esc(t('desktop.chess_commentary')) + '</span>' +
            '<span class="vd-chess-voice-wave" data-voice-wave aria-hidden="true"><i></i><i></i><i></i></span></button>' +
            '<button type="button" class="vd-chess-head-pill" data-chess-reset-cam hidden title="' + esc(t('desktop.chess_reset_camera')) + '" aria-label="' + esc(t('desktop.chess_reset_camera')) + '">' +
            icon('refresh', '⟲', 'vd-chess-action-icon') + '</button>' +
            '</div>' +
            '<div class="vd-chess-material-bar" data-material-bar>' +
            '<span class="vd-chess-material-side" aria-hidden="true">' + king + '</span>' +
            '<div class="vd-chess-material-track" title="' + esc(t('desktop.chess_material')) + '">' +
            '<div class="vd-chess-material-fill" data-material-fill></div></div>' +
            '<span class="vd-chess-material-score" data-material-bar-score>0</span></div>' +
            '<div class="vd-chess-status" data-status role="status" aria-live="polite">' + esc(t('desktop.chess_loading')) + '</div>' +
            '</div>' +
            '<div class="vd-chess-table">' +
            plate('top') +
            '<div class="vd-chess-board-shell">' +
            '<div class="vd-chess-fx-layer" data-fx-layer aria-hidden="true"></div>' +
            '<div class="vd-chess-board-frame" data-chess-board-frame>' +
            '<div class="vd-chess-rim-coords" data-rim-coords aria-hidden="true"></div>' +
            '<div class="vd-chess-board" data-chess-board></div>' +
            '<div class="vd-chess-board-3d" data-chess-board-3d hidden></div>' +
            '<div class="vd-chess-surface" aria-hidden="true"><i class="is-grain"></i><i class="is-bevel"></i><i class="is-light"></i></div>' +
            '</div></div>' +
            plate('bottom') +
            '</div>' +
            '<div class="vd-chess-comment" data-comment></div></section>' +
            '<aside class="vd-chess-side">' +
            '<div class="vd-chess-controls">' +
            '<details class="vd-chess-settings" data-settings>' +
            '<summary class="vd-chess-settings-summary">' + icon('settings', '⚙', 'vd-chess-settings-icon') +
            '<span>' + esc(t('desktop.app_settings')) + '</span>' +
            icon('chevron-down', '▾', 'vd-chess-settings-chevron') + '</summary>' +
            '<div class="vd-chess-settings-grid">' +
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
            '<option value="default">' + esc(t('desktop.chess_skin_classic')) + '</option>' +
            '<option value="marble">' + esc(t('desktop.chess_skin_marble')) + '</option>' +
            '<option value="slate">' + esc(t('desktop.chess_skin_slate')) + '</option></select></div>' +
            '<div class="vd-chess-field"><span class="vd-chess-label">' + esc(t('desktop.chess_player_color')) + '</span>' +
            '<div class="vd-chess-segment" data-color-group>' +
            '<button type="button" data-color="white" class="active">' + esc(t('desktop.chess_color_white')) + '</button>' +
            '<button type="button" data-color="black">' + esc(t('desktop.chess_color_black')) + '</button></div></div>' +
            '</div></details>' +
            '<div class="vd-chess-actions">' +
            '<button type="button" class="vd-chess-primary" data-new-game>' + icon('refresh', 'N') + '<span>' + esc(t('desktop.chess_new_game')) + '</span></button>' +
            '<button type="button" data-undo>' + icon('undo', 'U') + '<span>' + esc(t('desktop.chess_undo')) + '</span></button>' +
            '<button type="button" data-flip>' + icon('refresh', 'F') + '<span>' + esc(t('desktop.chess_flip')) + '</span></button>' +
            '<button type="button" data-hint>' + icon('help', 'H') + '<span>' + esc(t('desktop.chess_hint')) + '</span></button>' +
            '<button type="button" data-resign>' + icon('stop', 'R') + '<span>' + esc(t('desktop.chess_resign')) + '</span></button>' +
            '<button type="button" data-draw>' + icon('check-square', 'D') + '<span>' + esc(t('desktop.chess_draw')) + '</span></button></div>' +
            '<div class="vd-chess-replay" data-replay hidden>' +
            '<button type="button" data-review-first title="' + esc(t('desktop.chess_first')) + '" aria-label="' + esc(t('desktop.chess_first')) + '">' + icon('chevron-left', '|<', 'vd-chess-replay-icon') + '</button>' +
            '<button type="button" data-review-prev title="' + esc(t('desktop.chess_prev')) + '" aria-label="' + esc(t('desktop.chess_prev')) + '">' + icon('chevron-left', '<', 'vd-chess-replay-icon') + '</button>' +
            '<span class="vd-chess-review-label" data-review-label>' + esc(t('desktop.chess_live')) + '</span>' +
            '<button type="button" data-review-next title="' + esc(t('desktop.chess_next')) + '" aria-label="' + esc(t('desktop.chess_next')) + '">' + icon('chevron-right', '>', 'vd-chess-replay-icon') + '</button>' +
            '<button type="button" data-review-last title="' + esc(t('desktop.chess_last')) + '" aria-label="' + esc(t('desktop.chess_last')) + '">' + icon('chevron-right', '>|', 'vd-chess-replay-icon') + '</button></div></div>' +
            '<div class="vd-chess-tabs" data-tabs role="tablist" aria-label="' + esc(t('desktop.chess_title')) + '">' +
            '<button type="button" role="tab" id="chess-tab-moves" aria-controls="chess-panel-moves" aria-selected="true" class="active" data-tab="moves">' + esc(t('desktop.chess_tab_moves')) + '<span class="vd-chess-tab-badge" data-moves-count>0</span></button>' +
            '<button type="button" role="tab" id="chess-tab-captured" aria-controls="chess-panel-captured" aria-selected="false" tabindex="-1" data-tab="captured">' + esc(t('desktop.chess_tab_captured')) + '</button>' +
            '<button type="button" role="tab" id="chess-tab-info" aria-controls="chess-panel-info" aria-selected="false" tabindex="-1" data-tab="info">' + esc(t('desktop.chess_tab_info')) + '</button></div>' +
            '<div class="vd-chess-moves" data-panel="moves" role="tabpanel" id="chess-panel-moves" aria-labelledby="chess-tab-moves"><ol data-moves></ol></div>' +
            '<div class="vd-chess-captured-panel" data-panel="captured" role="tabpanel" id="chess-panel-captured" aria-labelledby="chess-tab-captured" hidden>' +
            '<div class="vd-chess-captured-row"><span class="vd-chess-captured-label">' + esc(t('desktop.chess_color_white')) + '</span>' +
            '<div class="vd-chess-captured-pieces" data-captured-white></div></div>' +
            '<div class="vd-chess-captured-row"><span class="vd-chess-captured-label">' + esc(t('desktop.chess_color_black')) + '</span>' +
            '<div class="vd-chess-captured-pieces" data-captured-black></div></div></div>' +
            '<div class="vd-chess-info-panel" data-panel="info" role="tabpanel" id="chess-panel-info" aria-labelledby="chess-tab-info" hidden>' +
            '<div class="vd-chess-info-row"><span>' + esc(t('desktop.chess_phase')) + '</span><span data-phase>' + esc(t('desktop.chess_phase_opening')) + '</span></div>' +
            '<div class="vd-chess-info-row"><span>' + esc(t('desktop.chess_material')) + '</span><span data-material>0</span></div>' +
            '<div class="vd-chess-info-row"><span>' + esc(t('desktop.chess_eval')) + '</span><span data-eval>' + esc(t('desktop.chess_even')) + '</span></div>' +
            '<div class="vd-chess-info-row"><span>' + esc(t('desktop.chess_hint')) + '</span><span data-hint-text>' + esc(t('desktop.chess_no_moves')) + '</span></div>' +
            '<div class="vd-chess-info-tools">' +
            '<button type="button" data-copy-pgn>' + icon('copy', '⧉') + '<span>' + esc(t('desktop.chess_copy_pgn')) + '</span></button>' +
            '<button type="button" data-copy-fen>' + icon('copy', '⧉') + '<span>' + esc(t('desktop.chess_copy_fen')) + '</span></button>' +
            '</div>' +
            '</div>' +
            '<div class="vd-chess-keys">' +
            '<span class="vd-chess-keys-title">' + esc(t('desktop.chess_shortcuts')) + '</span>' +
            '<span><kbd>F</kbd>' + esc(t('desktop.chess_flip')) + '</span>' +
            '<span><kbd>M</kbd>' + esc(t('desktop.chess_sound')) + '</span>' +
            '<span><kbd>V</kbd>' + esc(t('desktop.chess_commentary')) + '</span>' +
            '<span><kbd>←</kbd><kbd>→</kbd>' + esc(t('desktop.chess_tab_moves')) + '</span>' +
            '</div>' +
            '</aside></div>';
    }

    const SAN_FIGURINES = { K: '♔', Q: '♕', R: '♖', B: '♗', N: '♘' };

    function formatSanMarkup(esc, san) {
        const raw = String(san || '');
        if (!raw) return '';
        let prefix = '';
        let body = raw;
        if (SAN_FIGURINES[body.charAt(0)]) {
            prefix = '<span class="vd-chess-move-fig" aria-hidden="true">' + SAN_FIGURINES[body.charAt(0)] + '</span>';
            body = body.slice(1);
        }
        return prefix + '<span>' + esc(body) + '</span>';
    }

    function renderMoveList(state, onJump) {
        const refs = state.refs;
        if (!refs || !refs.moves) return;
        const moves = state.game ? state.game.history({ verbose: true }) : [];
        const focusedMove = refs.moves.contains(document.activeElement) ? document.activeElement.dataset.jump : null;
        const currentMove = state.reviewActive ? state.reviewIndex : moves.length - 1;
        if (refs.movesCount) refs.movesCount.textContent = String(moves.length);
        if (!moves.length) {
            refs.moves.innerHTML = `<li class="vd-chess-empty">${state.ctx.esc(state.ctx.t('desktop.chess_no_moves'))}</li>`;
            return;
        }
        const cell = (idx, move) => {
            if (!move) return '<span class="vd-chess-move-cell is-empty"></span>';
            const flags = (move.san.indexOf('#') !== -1 ? ' is-mate' : move.san.indexOf('+') !== -1 ? ' is-check' : move.captured ? ' is-capture' : '');
            return `<button type="button" class="vd-chess-move-cell${flags}" data-jump="${idx}" aria-label="${state.ctx.esc(move.san)}"${currentMove === idx ? ' aria-current="true"' : ''}>${formatSanMarkup(state.ctx.esc, move.san)}</button>`;
        };
        const rows = [];
        for (let i = 0; i < moves.length; i += 2) {
            const white = moves[i];
            const black = moves[i + 1];
            const moveNo = Math.floor(i / 2) + 1;
            const cls = (state.reviewActive && (state.reviewIndex === i || state.reviewIndex === i + 1)) ? ' is-review' : '';
            rows.push(`<li class="vd-chess-move-row${cls}" data-move-idx="${i}"><span class="vd-chess-move-no">${moveNo}.</span>${cell(i, white)}${cell(i + 1, black)}</li>`);
        }
        refs.moves.innerHTML = rows.join('');
        if (!state.reviewActive) refs.moves.scrollTop = refs.moves.scrollHeight;
        refs.moves.querySelectorAll('[data-jump]').forEach(button => {
            button.addEventListener('click', () => {
                const idx = Number(button.dataset.jump);
                if (moves[idx] && typeof onJump === 'function') onJump(idx);
            });
        });
        if (focusedMove !== null) {
            const button = refs.moves.querySelector(`[data-jump="${focusedMove}"]`);
            if (button) button.focus();
        }
    }

    function bindChessTabs(tabs, onSelect) {
        if (!tabs) return;
        const buttons = Array.prototype.slice.call(tabs.querySelectorAll('[data-tab]'));
        tabs.addEventListener('click', event => {
            const button = event.target.closest('[data-tab]');
            if (!button) return;
            onSelect(button.dataset.tab);
        });
        tabs.addEventListener('keydown', event => {
            const index = buttons.indexOf(event.target.closest('[data-tab]'));
            if (index < 0) return;
            let next = index;
            if (event.key === 'ArrowRight') next = (index + 1) % buttons.length;
            else if (event.key === 'ArrowLeft') next = (index - 1 + buttons.length) % buttons.length;
            else if (event.key === 'Home') next = 0;
            else if (event.key === 'End') next = buttons.length - 1;
            else return;
            event.preventDefault();
            onSelect(buttons[next].dataset.tab);
            buttons[next].focus();
        });
    }

    function showChessModal(root, message, buttons, escFn) {
        if (!root) return;
        const existing = root.querySelector('.vd-chess-modal:not(.vd-chess-result-modal)');
        if (existing) existing.remove();
        const esc = escFn || (v => String(v == null ? '' : v));
        const dialog = document.createElement('div');
        dialog.className = 'vd-chess-modal';
        dialog.setAttribute('role', 'dialog');
        dialog.setAttribute('aria-modal', 'true');
        const btnHtml = (buttons || []).map((b, i) =>
            '<button type="button" data-idx="' + i + '" class="' + (b.primary ? 'is-primary' : '') + '">' + esc(b.label) + '</button>'
        ).join('');
        dialog.innerHTML = '<div class="vd-chess-modal-card"><div class="vd-chess-modal-text">' + esc(message) +
            '</div><div class="vd-chess-modal-actions">' + btnHtml + '</div></div>';
        const close = action => {
            window.removeEventListener('keydown', onKey, true);
            if (dialog.isConnected) dialog.remove();
            if (typeof action === 'function') action();
        };
        const onKey = event => {
            if (event.key === 'Escape') {
                event.preventDefault();
                event.stopPropagation();
                close(null);
            }
        };
        window.addEventListener('keydown', onKey, true);
        dialog.addEventListener('click', event => {
            const button = event.target.closest('[data-idx]');
            if (!button) {
                if (event.target === dialog) close(null);
                return;
            }
            const idx = Number(button.dataset.idx);
            close(buttons[idx] && buttons[idx].action);
        });
        root.appendChild(dialog);
        const primary = dialog.querySelector('.is-primary') || dialog.querySelector('button');
        if (primary) primary.focus();
    }

    function requestPromotionChoice(state, color) {
        const existing = state.refs.root.querySelector('.vd-chess-promotion-backdrop, .vd-chess-promotion');
        if (existing) existing.remove();
        return new Promise(resolve => {
            const choices = ['q', 'r', 'b', 'n'];
            const glyphs = { q: '♛', r: '♜', b: '♝', n: '♞' };
            const names = {
                q: state.ctx.t('desktop.chess_promo_queen'),
                r: state.ctx.t('desktop.chess_promo_rook'),
                b: state.ctx.t('desktop.chess_promo_bishop'),
                n: state.ctx.t('desktop.chess_promo_knight')
            };
            const backdrop = document.createElement('div');
            backdrop.className = 'vd-chess-promotion-backdrop';
            const dialog = document.createElement('div');
            dialog.className = 'vd-chess-promotion' + (color === 'b' ? ' is-black' : '');
            dialog.setAttribute('role', 'dialog');
            dialog.setAttribute('aria-modal', 'true');
            dialog.innerHTML = choices.map(piece => `<button type="button" data-piece="${piece}"><span class="vd-chess-promo-glyph vd-chess-piece-${piece}">${glyphs[piece]}</span><span>${state.ctx.esc(names[piece])}</span></button>`).join('');
            backdrop.appendChild(dialog);

            const finish = piece => {
                window.removeEventListener('keydown', onKey, true);
                if (backdrop.isConnected) backdrop.remove();
                resolve(piece || null);
            };
            const onKey = event => {
                if (event.key === 'Escape') {
                    event.preventDefault();
                    event.stopPropagation();
                    finish(null);
                }
            };
            window.addEventListener('keydown', onKey, true);
            backdrop.addEventListener('click', event => {
                const button = event.target.closest('[data-piece]');
                if (button) {
                    finish(button.dataset.piece);
                    return;
                }
                if (event.target === backdrop) finish(null);
            });
            state.refs.root.appendChild(backdrop);
            const first = dialog.querySelector('button');
            if (first) first.focus();
        });
    }

    function detectGamePhaseKey(game, pieceValues) {
        if (!game) return 'desktop.chess_phase_opening';
        const fenBoard = game.fen().split(' ')[0];
        let nonPawnMaterial = 0;
        for (const ch of fenBoard) {
            const p = ch.toLowerCase();
            if (p !== 'p' && pieceValues[p]) nonPawnMaterial += pieceValues[p];
        }
        if (nonPawnMaterial <= 20) return 'desktop.chess_phase_endgame';
        return game.history().length < 16 ? 'desktop.chess_phase_opening' : 'desktop.chess_phase_middlegame';
    }

    async function copyChessNotation(game, kind, onCopied) {
        if (!game) return;
        const value = kind === 'fen' ? game.fen() : (game.pgn() || game.fen());
        try {
            if (navigator.clipboard && typeof navigator.clipboard.writeText === 'function') {
                await navigator.clipboard.writeText(value);
                if (typeof onCopied === 'function') onCopied();
            }
        } catch (e) {}
    }

    function syncSoundUI(state) {
        const btn = state.refs && state.refs.soundToggle;
        if (!btn) return;
        const on = !state.soundMuted;
        btn.classList.toggle('is-active', on);
        btn.classList.toggle('is-muted', !on);
        btn.setAttribute('aria-pressed', on ? 'true' : 'false');
        btn.setAttribute('title', state.ctx.t(on ? 'desktop.chess_sound_on' : 'desktop.chess_sound_off'));
    }

    function syncCommentaryUI(state) {
        const btn = state.refs && state.refs.commentaryToggle;
        if (!btn) return;
        const enabled = !!(state.commentary && state.commentary.isEnabled());
        btn.classList.toggle('is-active', enabled);
        btn.classList.toggle('is-speaking', enabled && state.commentarySpeaking);
        btn.setAttribute('aria-pressed', enabled ? 'true' : 'false');
        const tipKey = !enabled ? 'desktop.chess_commentary_hint'
            : state.commentaryAudioUnavailable ? 'desktop.chess_commentary_no_audio'
            : state.commentarySpeaking ? 'desktop.chess_commentary_speaking'
            : 'desktop.chess_commentary_on';
        btn.setAttribute('title', state.ctx.t(tipKey));
        state.refs.root?.classList.toggle('is-commentary-speaking', enabled && state.commentarySpeaking);
    }

    function moveStereoPan(move, orientation) {
        if (!move || !move.to) return 0;
        const file = 'abcdefgh'.indexOf(move.to.charAt(0));
        if (file < 0) return 0;
        const col = orientation === 'black' ? (7 - file) : file;
        return ((col - 3.5) / 3.5) * 0.28;
    }

    function playMoveSound(state, move, opts) {
        if (!state.audio || !move) return;
        const flags = move.flags || '';
        const pan = moveStereoPan(move, (opts && opts.orientation) || 'white');
        if (opts && opts.inCheck) {
            if (move.captured) state.audio.capture(pan);
            state.audio.check();
            return;
        }
        if (move.promotion) state.audio.promote();
        else if (move.captured) state.audio.capture(pan);
        else if (flags.indexOf('k') !== -1 || flags.indexOf('q') !== -1) state.audio.castle();
        else state.audio.move(pan);
    }

    function playGameOverSound(state, outcome) {
        if (!state.audio) return;
        if (outcome === 'draw') {
            state.audio.gameOver('draw');
            return;
        }
        state.audio.gameOver(!!outcome);
    }

    const OPPONENT_ERROR_KEYS = {
        agent_unavailable: 'desktop.chess_agent_unavailable',
        engine_unavailable: 'desktop.chess_engine_unavailable',
        engine_no_move: 'desktop.chess_engine_no_move',
        engine_worker_failed: 'desktop.chess_engine_worker_failed',
        engine_timeout: 'desktop.chess_engine_timeout',
        engine_illegal: 'desktop.chess_engine_illegal',
        agent_no_move: 'desktop.chess_agent_no_move',
        opponent_illegal: 'desktop.chess_opponent_illegal'
    };

    const OPPONENT_ERROR_MESSAGES = {
        'Agent client is not available.': 'desktop.chess_agent_unavailable',
        'Stockfish is not available.': 'desktop.chess_engine_unavailable',
        'Stockfish did not return a move.': 'desktop.chess_engine_no_move',
        'Stockfish worker failed.': 'desktop.chess_engine_worker_failed',
        'Stockfish timed out.': 'desktop.chess_engine_timeout',
        'Stockfish returned an illegal move.': 'desktop.chess_engine_illegal',
        'Agent did not return a move.': 'desktop.chess_agent_no_move',
        'Opponent returned an illegal move.': 'desktop.chess_opponent_illegal'
    };

    function formatOpponentError(state, err) {
        const code = err && err.chessCode;
        if (code && OPPONENT_ERROR_KEYS[code]) return state.ctx.t(OPPONENT_ERROR_KEYS[code]);
        const message = err && err.message ? String(err.message) : '';
        if (message && OPPONENT_ERROR_MESSAGES[message]) return state.ctx.t(OPPONENT_ERROR_MESSAGES[message]);
        return state.ctx.t('desktop.chess_move_failed');
    }

    window.ChessInfoTools = { detectGamePhaseKey, copyChessNotation };
    window.ChessSoundPref = { load: loadSoundMuted, save: saveSoundMuted };
    window.ChessChrome = {
        syncSoundUI,
        syncCommentaryUI,
        playMoveSound,
        playGameOverSound,
        formatOpponentError
    };
    window.ChessPromotion = { requestPromotionChoice };
    window.createChessFx = createChessFx;
    window.createChessAudio = createChessAudio;
    window.renderChessTemplate = renderChessTemplate;
    window.showChessModal = showChessModal;
    window.ChessBoardSkin = { normalize: normalizeBoardSkin, load: loadBoardSkin, save: saveBoardSkin, apply: applyBoardSkin };
    window.ChessMaterialBar = { update: updateMaterialBar };
    window.ChessMoveList = { render: renderMoveList };
    window.ChessTabs = { bind: bindChessTabs };
})();
