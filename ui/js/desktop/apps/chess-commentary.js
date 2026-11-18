/* AuraGo chess - optional sarcastic Phonon TTS move commentator.
 *
 * Classifies board situations (openings, blunders, forks, knight-on-rim,
 * early queen sorties, castling, en passant, promotions, time trouble,
 * wood-shuffling, undo/hint roasts and game endings) and fetches server-owned
 * remarks synthesized in memory with default Phonon TTS.
 */
(function () {
    'use strict';

    const COMMENTARY_URL = '/api/desktop/chess/commentary';
    const STORAGE_KEY = 'aurago.desktop.chess.commentary';
    const PIECE_WORTH = { p: 1, n: 3, b: 3, r: 5, q: 9, k: 20 };

    function loadEnabled() {
        try {
            return window.localStorage.getItem(STORAGE_KEY) === '1';
        } catch (e) {
            return false;
        }
    }

    function saveEnabled(enabled) {
        try {
            window.localStorage.setItem(STORAGE_KEY, enabled ? '1' : '0');
        } catch (e) {}
    }

    function detectLanguage() {
        try {
            const docLang = (document.documentElement && document.documentElement.lang) || '';
            if (docLang) return docLang.slice(0, 5).toLowerCase();
            if (window.AuraI18n && typeof window.AuraI18n.getLang === 'function') {
                return String(window.AuraI18n.getLang() || '').toLowerCase();
            }
        } catch (e) {}
        return '';
    }

    function parseFenBoard(fen) {
        const board = Array.from({ length: 8 }, () => Array(8).fill(null));
        const rows = String(fen || '').split(' ')[0].split('/');
        for (let r = 0; r < Math.min(8, rows.length); r++) {
            let file = 0;
            for (const ch of rows[r]) {
                if (ch >= '1' && ch <= '8') {
                    file += Number(ch);
                    continue;
                }
                if (file < 8) {
                    board[7 - r][file] = {
                        type: ch.toLowerCase(),
                        color: ch === ch.toUpperCase() ? 'w' : 'b'
                    };
                    file++;
                }
            }
        }
        return board;
    }

    function countPieces(board) {
        let total = 0;
        for (let r = 0; r < 8; r++) {
            for (let f = 0; f < 8; f++) {
                if (board[r][f]) total++;
            }
        }
        return total;
    }

    function detectsFork(board, move) {
        if (!move || !move.to) return false;
        const file = 'abcdefgh'.indexOf(move.to[0]);
        const rank = Number(move.to[1]) - 1;
        if (file < 0 || rank < 0 || rank > 7) return false;
        const mover = board[rank][file];
        if (!mover) return false;
        const enemy = mover.color === 'w' ? 'b' : 'w';
        const targets = [];
        const addIfEnemy = (r, f) => {
            if (r < 0 || r > 7 || f < 0 || f > 7) return false;
            const cell = board[r][f];
            if (!cell) return false;
            if (cell.color === enemy && (PIECE_WORTH[cell.type] || 0) >= 5) {
                targets.push(cell.type);
            }
            return true;
        };
        if (mover.type === 'n') {
            const jumps = [[2, 1], [2, -1], [-2, 1], [-2, -1], [1, 2], [1, -2], [-1, 2], [-1, -2]];
            jumps.forEach(([dr, df]) => {
                const r = rank + dr, f = file + df;
                if (r >= 0 && r <= 7 && f >= 0 && f <= 7) {
                    const cell = board[r][f];
                    if (cell && cell.color === enemy && (PIECE_WORTH[cell.type] || 0) >= 3) targets.push(cell.type);
                }
            });
        } else if (mover.type === 'p') {
            const dir = mover.color === 'w' ? 1 : -1;
            [file - 1, file + 1].forEach(f => {
                const r = rank + dir;
                if (r >= 0 && r <= 7 && f >= 0 && f <= 7) {
                    const cell = board[r][f];
                    if (cell && cell.color === enemy && (PIECE_WORTH[cell.type] || 0) >= 3) targets.push(cell.type);
                }
            });
        } else if (mover.type === 'b' || mover.type === 'r' || mover.type === 'q') {
            const dirs = [];
            if (mover.type !== 'r') dirs.push([1, 1], [1, -1], [-1, 1], [-1, -1]);
            if (mover.type !== 'b') dirs.push([1, 0], [-1, 0], [0, 1], [0, -1]);
            dirs.forEach(([dr, df]) => {
                for (let step = 1; step < 8; step++) {
                    if (addIfEnemy(rank + dr * step, file + df * step)) break;
                }
            });
        }
        return targets.length >= 2;
    }

    function createChessCommentary(options = {}) {
        const api = typeof options.api === 'function' ? options.api : null;
        const getAudio = typeof options.getAudio === 'function' ? options.getAudio : (() => null);
        const onUpdate = typeof options.onUpdate === 'function' ? options.onUpdate : (() => {});

        let enabled = loadEnabled();
        let disposed = false;
        let loading = false;
        let speaking = false;
        let audioUnavailable = false;
        let lastText = '';
        let activeAbort = null;
        let generation = 0;

        let lastRemarkAt = 0;
        let consecutiveChecks = 0;
        let warnedWhiteClock = false;
        let warnedBlackClock = false;
        let lastMaterialCallPly = -20;

        const heardByCategory = new Map();
        const categoryLastAt = new Map();
        const clientAudioCache = new Map();

        function notifyState() {
            if (disposed) return;
            onUpdate({
                enabled,
                loading,
                speaking,
                text: lastText,
                audioUnavailable
            });
        }

        function resetGameTracking() {
            consecutiveChecks = 0;
            warnedWhiteClock = false;
            warnedBlackClock = false;
            lastMaterialCallPly = -20;
            lastText = '';
            notifyState();
        }

        function cancelCurrent(stopAudio) {
            generation++;
            if (activeAbort) {
                try { activeAbort.abort(); } catch (e) {}
                activeAbort = null;
            }
            loading = false;
            if (stopAudio) {
                const audio = getAudio();
                if (audio && typeof audio.stopVoice === 'function') audio.stopVoice();
                speaking = false;
            }
            notifyState();
        }

        function setEnabled(next) {
            const flag = !!next;
            if (flag === enabled) return enabled;
            enabled = flag;
            saveEnabled(enabled);
            if (!enabled) {
                cancelCurrent(true);
            } else {
                notifyState();
            }
            return enabled;
        }

        function toggle() {
            return setEnabled(!enabled);
        }

        function recordHeard(category, variant) {
            if (!Number.isInteger(variant) || variant < 0) return;
            const list = heardByCategory.get(category) || [];
            if (list.indexOf(variant) === -1) list.push(variant);
            if (list.length > 32) list.shift();
            heardByCategory.set(category, list);
        }

        function canSpeakPriority(category, priority, now) {
            if (priority >= 3) return true;
            if (speaking || loading) {
                // Only major blunders or meta actions preempt an in-flight routine line.
                return priority >= 2 && (now - lastRemarkAt > 2600);
            }
            const catLast = categoryLastAt.get(category) || 0;
            if (priority === 2) {
                return (now - catLast) >= 6000 && (now - lastRemarkAt) >= 2200;
            }
            if (priority === 1) {
                return (now - catLast) >= 18000 && (now - lastRemarkAt) >= 4800;
            }
            return (now - catLast) >= 20000 && (now - lastRemarkAt) >= 8500;
        }

        async function trigger(spec) {
            if (!enabled || disposed || !spec || !spec.category) return;
            if (typeof document !== 'undefined' && document.hidden) return;
            const now = Date.now();
            const priority = spec.priority || 0;
            if (!canSpeakPriority(spec.category, priority, now)) return;

            if (priority >= 2 && (loading || speaking)) {
                cancelCurrent(true);
            }

            const token = ++generation;
            lastRemarkAt = now;
            categoryLastAt.set(spec.category, now);
            loading = true;
            notifyState();

            const exclude = (heardByCategory.get(spec.category) || []).slice();
            const lang = detectLanguage();
            const controller = typeof AbortController === 'function' ? new AbortController() : null;
            activeAbort = controller;

            try {
                const payload = {
                    category: spec.category,
                    piece: spec.piece || '',
                    square: spec.square || '',
                    lang,
                    exclude
                };
                let resp;
                if (api && !controller) {
                    resp = await api(COMMENTARY_URL, {
                        method: 'POST',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify(payload)
                    });
                } else {
                    const httpResp = await fetch(COMMENTARY_URL, {
                        method: 'POST',
                        credentials: 'same-origin',
                        cache: 'no-store',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify(payload),
                        signal: controller ? controller.signal : undefined
                    });
                    if (!httpResp.ok) throw new Error('HTTP ' + httpResp.status);
                    resp = await httpResp.json();
                }
                if (disposed || !enabled || token !== generation) return;
                activeAbort = null;
                loading = false;

                if (!resp || !resp.text) {
                    notifyState();
                    return;
                }
                if (Number.isInteger(resp.variant)) {
                    // If server wrapped around, reset our heard deck to just this new variant.
                    if (exclude.indexOf(resp.variant) !== -1) {
                        heardByCategory.set(spec.category, [resp.variant]);
                    } else {
                        recordHeard(spec.category, resp.variant);
                    }
                }
                lastText = String(resp.text);
                audioUnavailable = !!resp.audio_unavailable && !resp.audio;

                const cacheKey = (resp.language || lang) + '|' + resp.text;
                let audioBase64 = resp.audio || clientAudioCache.get(cacheKey) || '';
                if (resp.audio) {
                    if (clientAudioCache.size >= 40) {
                        const oldest = clientAudioCache.keys().next().value;
                        clientAudioCache.delete(oldest);
                    }
                    clientAudioCache.set(cacheKey, resp.audio);
                }

                const audio = getAudio();
                if (audioBase64 && audio && typeof audio.playVoice === 'function') {
                    speaking = true;
                    notifyState();
                    audio.playVoice(audioBase64, () => {
                        if (token === generation && !disposed) {
                            speaking = false;
                            notifyState();
                        }
                    });
                } else {
                    speaking = false;
                    notifyState();
                }
            } catch (e) {
                if (token === generation && !disposed) {
                    activeAbort = null;
                    loading = false;
                    speaking = false;
                    notifyState();
                }
            }
        }

        function classifyMove(appState, move) {
            const game = appState && appState.game;
            if (!game || !move) return null;
            const fen = game.fen();
            const fenParts = fen.split(/\s+/);
            const halfmoveClock = Number(fenParts[4]) || 0;
            const fullMove = Number(fenParts[5]) || 1;
            const history = game.history({ verbose: true });
            const ply = history.length;
            const prevMove = ply >= 2 ? history[ply - 2] : null;
            const prevOwnMove = ply >= 3 ? history[ply - 3] : null;
            const flags = move.flags || '';
            const playerSide = appState.playerColor === 'black' ? 'b' : 'w';
            const isLocal = appState.mode === 'local';
            const inCheck = typeof game.inCheck === 'function' ? !!game.inCheck() : (typeof game.in_check === 'function' && !!game.in_check());

            if (inCheck) consecutiveChecks++;
            else consecutiveChecks = 0;

            // Game over is handled by onGameOver so result and checkmate never double-trigger.
            const over = typeof game.isGameOver === 'function' ? !!game.isGameOver() : (typeof game.game_over === 'function' && !!game.game_over());
            if (over) return null;

            const board = parseFenBoard(fen);
            const sq = move.to || '';
            const pc = move.piece || 'p';

            // Priority 2: Dramatic tactical & meme events
            if (move.promotion && move.promotion !== 'q') return { category: 'underpromotion', piece: move.promotion, square: sq, priority: 2 };
            if (move.promotion) return { category: 'promotion', piece: 'q', square: sq, priority: 2 };
            if (flags.indexOf('e') !== -1) return { category: 'en_passant', piece: 'p', square: sq, priority: 2 };
            if (move.captured === 'q') {
                const snatched = isLocal || move.color === playerSide;
                return { category: snatched ? 'queen_snatch' : 'queen_blunder', piece: pc, square: sq, priority: 2 };
            }
            if (fullMove <= 6 && pc === 'k' && flags.indexOf('k') === -1 && flags.indexOf('q') === -1 && (sq === 'e2' || sq === 'e7' || sq === 'f2' || sq === 'f7')) {
                return { category: 'bongcloud', piece: 'k', square: sq, priority: 2 };
            }
            if (consecutiveChecks >= 3) return { category: 'check_streak', piece: pc, square: sq, priority: 2 };
            if (detectsFork(board, move)) return { category: 'fork', piece: pc, square: sq, priority: 2 };
            if (move.captured === 'r') return { category: 'rook_capture', piece: pc, square: sq, priority: 2 };

            // Priority 1: Signature chess moments & funny habits
            if (ply === 1) {
                if (sq === 'e4' && pc === 'p') return { category: 'opening_e4', piece: 'p', square: sq, priority: 1 };
                if (sq === 'd4' && pc === 'p') return { category: 'opening_d4', piece: 'p', square: sq, priority: 1 };
                if (sq[0] === 'a' || sq[0] === 'h' || sq === 'b4' || sq === 'g4') return { category: 'opening_flank', piece: pc, square: sq, priority: 1 };
            }
            if (ply === 2 && prevMove && prevMove.to === 'e4' && sq === 'c5' && pc === 'p') {
                return { category: 'opening_sicilian', piece: 'p', square: sq, priority: 1 };
            }
            if (flags.indexOf('q') !== -1) return { category: 'castle_queenside', piece: 'k', square: sq, priority: 1 };
            if (flags.indexOf('k') !== -1) return { category: 'castle_kingside', piece: 'k', square: sq, priority: 1 };
            if (inCheck) {
                const given = isLocal || move.color === playerSide;
                return { category: given ? 'check_given' : 'check_taken', piece: pc, square: sq, priority: 1 };
            }
            if (fullMove <= 5 && pc === 'q' && !move.captured) {
                if (sq === 'h5' || sq === 'f3' || sq === 'h4' || sq === 'f6') {
                    return { category: 'scholars_threat', piece: 'q', square: sq, priority: 1 };
                }
                return { category: 'early_queen', piece: 'q', square: sq, priority: 1 };
            }
            if (pc === 'n' && (sq[0] === 'a' || sq[0] === 'h')) {
                return { category: 'knight_rim', piece: 'n', square: sq, priority: 1 };
            }
            if (fullMove <= 6 && pc === 'p' && (sq === 'f3' || sq === 'f6' || sq === 'g4' || sq === 'g5')) {
                return { category: 'fools_weakness', piece: 'p', square: sq, priority: 1 };
            }
            if (prevOwnMove && prevOwnMove.piece === pc && prevOwnMove.from === sq && prevOwnMove.to === move.from && !move.captured) {
                return { category: 'retreat_same', piece: pc, square: sq, priority: 1 };
            }
            if (move.captured && prevMove && prevMove.captured && prevMove.to === sq) {
                return { category: 'recapture', piece: pc, square: sq, priority: 1 };
            }
            if (move.captured === 'n' || move.captured === 'b') {
                return { category: 'minor_capture', piece: pc, square: sq, priority: 1 };
            }
            if (appState.clockBase > 0) {
                if (appState.whiteTime > 0 && appState.whiteTime < 25 && !warnedWhiteClock) {
                    warnedWhiteClock = true;
                    return { category: 'clock_panic', piece: pc, square: sq, priority: 1 };
                }
                if (appState.blackTime > 0 && appState.blackTime < 25 && !warnedBlackClock) {
                    warnedBlackClock = true;
                    return { category: 'clock_panic', piece: pc, square: sq, priority: 1 };
                }
            }
            if (halfmoveClock >= 12 && halfmoveClock % 6 === 0) {
                return { category: 'wood_shuffling', piece: pc, square: sq, priority: 1 };
            }
            if (!isLocal && ply - lastMaterialCallPly >= 12) {
                const lead = playerSide === 'w' ? appState.materialScore : -appState.materialScore;
                if (lead >= 7) {
                    lastMaterialCallPly = ply;
                    return { category: 'crushing_lead', piece: pc, square: sq, priority: 1 };
                }
                if (lead <= -7) {
                    lastMaterialCallPly = ply;
                    return { category: 'hopeless_deficit', piece: pc, square: sq, priority: 1 };
                }
            }
            const totalPieces = countPieces(board);
            if (totalPieces <= 8 && pc === 'k') {
                return { category: 'endgame_king', piece: 'k', square: sq, priority: 1 };
            }

            // Priority 0: Routine moves — in AI mode prefer commenting when control returns to the player
            // so Stockfish's fast reply does not outpace a routine observation.
            if (!isLocal && move.color === playerSide && appState.mode === 'computer') {
                return null;
            }
            if (move.captured === 'p') return { category: 'pawn_capture', piece: pc, square: sq, priority: 0 };
            if (pc !== 'p') return { category: 'piece_develop', piece: pc, square: sq, priority: 0 };
            return { category: 'pawn_push', piece: 'p', square: sq, priority: 0 };
        }

        function onMove(appState, move) {
            if (!enabled || disposed) return;
            const spec = classifyMove(appState, move);
            if (spec) trigger(spec);
        }

        function onAction(kind) {
            if (!enabled || disposed) return;
            if (kind === 'undo' || kind === 'hint') {
                trigger({ category: kind, priority: 2 });
            }
        }

        function onGameOver(kind) {
            if (!enabled || disposed) return;
            const allowed = {
                checkmate_win: true,
                checkmate_loss: true,
                checkmate_local: true,
                stalemate: true,
                draw: true,
                resign: true,
                timeout: true
            };
            if (allowed[kind]) trigger({ category: kind, priority: 3 });
        }

        function dispose() {
            if (disposed) return;
            disposed = true;
            cancelCurrent(true);
            clientAudioCache.clear();
        }

        return {
            isEnabled: () => enabled,
            setEnabled,
            toggle,
            getText: () => lastText,
            isSpeaking: () => speaking || loading,
            resetGame: resetGameTracking,
            onMove,
            onAction,
            onGameOver,
            dispose
        };
    }

    window.ChessCommentaryPref = { load: loadEnabled, save: saveEnabled };
    window.createChessCommentary = createChessCommentary;
})();
