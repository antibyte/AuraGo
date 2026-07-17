(function () {
    'use strict';

    const instances = new Map();
    let vendorPromise = null;

    function loadVendor() {
        if (!vendorPromise) vendorPromise = import('/js/vendor/chess-vendor.esm.js');
        return vendorPromise;
    }

    // Standard chess piece values for material counting.
    const PIECE_VALUES = { p: 1, n: 3, b: 3, r: 5, q: 9, k: 0 };
    const PIECE_ORDER = ['q', 'r', 'b', 'n', 'p'];
    const PIECE_GLYPH = { q: '♛', r: '♜', b: '♝', n: '♞', p: '♟', k: '♚' };

    function render(container, windowId, context = {}) {
        dispose(windowId);
        const ctx = normalizeContext(context);
        const state = {
            windowId,
            container,
            ctx,
            disposed: false,
            vendor: null,
            game: null,
            board: null,
            engine: null,
            agent: null,
            audio: null,
            resizeObserver: null,
            resizeHandler: null,
            playerColor: 'white',
            mode: 'computer',
            opponent: 'computer',
            strength: 8,
            clockBase: 0,
            flipped: false,
            busy: false,
            pendingMove: null,
            searchToken: 0,
            agentComment: '',
            lastMove: null,
            reviewIndex: -1,
            reviewActive: false,
            hintMove: null,
            hintToken: 0,
            clockTimer: null,
            whiteTime: 0,
            blackTime: 0,
            activeTab: 'moves',
            captured: { w: [], b: [] },
            materialScore: 0,
            gameOver: false,
            endedReason: '',
            refs: {}
        };
        instances.set(windowId, state);
        container.innerHTML = template(ctx);
        collectRefs(state);
        bindControls(state);
        setWindowMenus(state);
        setStatus(state, ctx.t('desktop.chess_loading'));

        loadVendor()
            .then(vendor => {
                if (state.disposed) return;
                state.vendor = vendor;
                state.game = new vendor.Chess();
                state.engine = window.createChessEngine ? window.createChessEngine({}) : null;
                state.agent = window.createChessAgentClient ? window.createChessAgentClient({ api: ctx.api }) : null;
                state.audio = createChessAudio();
                createBoard(state);
                startNewGame(state);
            })
            .catch(err => {
                setStatus(state, (err && err.message) || ctx.t('desktop.chess_load_failed'));
                state.refs.root.classList.add('is-error');
            });
    }

    function dispose(windowId) {
        const state = instances.get(windowId);
        if (!state) return;
        state.disposed = true;
        state.searchToken++;
        state.hintToken++;
        stopClock(state);
        try {
            if (state.board) state.board.destroy();
        } catch (e) {}
        try {
            if (state.resizeObserver) state.resizeObserver.disconnect();
        } catch (e) {}
        if (state.resizeHandler) window.removeEventListener('resize', state.resizeHandler);
        try {
            if (state.engine) state.engine.dispose();
        } catch (e) {}
        if (state.ctx && typeof state.ctx.clearWindowMenus === 'function') state.ctx.clearWindowMenus(windowId);
        instances.delete(windowId);
    }

    function normalizeContext(context) {
        const ctx = context || {};
        const fallbackEsc = value => String(value == null ? '' : value).replace(/[&<>"']/g, ch => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[ch]));
        const fallbackT = (key, fallback) => fallback || key;
        return {
            esc: ctx.esc || fallbackEsc,
            t: ctx.t || fallbackT,
            api: ctx.api || defaultAPI,
            iconMarkup: ctx.iconMarkup || ((key, fallback, cls) => `<span class="${fallbackEsc(cls || '')}">${fallbackEsc(fallback || key || '')}</span>`),
            notify: ctx.notify || null,
            setWindowMenus: ctx.setWindowMenus,
            clearWindowMenus: ctx.clearWindowMenus
        };
    }

    function template(ctx) {
        const esc = ctx.esc;
        const t = ctx.t;
        const icon = (k, fb, cls) => ctx.iconMarkup(k, fb, cls || 'vd-chess-action-icon', 14);
        return `
            <div class="vd-chess">
                <section class="vd-chess-board-pane">
                    <div class="vd-chess-board-shell">
                        <div class="vd-chess-board" data-chess-board></div>
                    </div>
                    <div class="vd-chess-ribbon">
                        <div class="vd-chess-status" data-status>${esc(t('desktop.chess_loading'))}</div>
                        <div class="vd-chess-comment" data-comment></div>
                    </div>
                </section>
                <aside class="vd-chess-side">
                    <div class="vd-chess-clocks" data-clocks hidden>
                        <div class="vd-chess-clock" data-clock="white">
                            <span class="vd-chess-clock-side" data-clock-icon>${PIECE_GLYPH.k}</span>
                            <span class="vd-chess-clock-time" data-white-time>--:--</span>
                        </div>
                        <div class="vd-chess-clock" data-clock="black">
                            <span class="vd-chess-clock-side" data-clock-icon>${PIECE_GLYPH.k}</span>
                            <span class="vd-chess-clock-time" data-black-time>--:--</span>
                        </div>
                    </div>
                    <div class="vd-chess-controls">
                        <div class="vd-chess-field">
                            <label for="chess-mode">${esc(t('desktop.chess_mode'))}</label>
                            <select id="chess-mode" data-mode>
                                <option value="computer">${esc(t('desktop.chess_mode_computer'))}</option>
                                <option value="agent">${esc(t('desktop.chess_mode_agent'))}</option>
                                <option value="local">${esc(t('desktop.chess_mode_local'))}</option>
                            </select>
                        </div>
                        <div class="vd-chess-field" data-strength-wrap>
                            <label for="chess-strength">${esc(t('desktop.chess_strength'))} <span data-strength-value>8</span></label>
                            <input id="chess-strength" data-strength type="range" min="1" max="20" value="8">
                        </div>
                        <div class="vd-chess-field" data-clock-wrap>
                            <label for="chess-clock-select">${esc(t('desktop.chess_clock'))}</label>
                            <select id="chess-clock-select" data-clock-select>
                                <option value="0">${esc(t('desktop.chess_clock_off'))}</option>
                                <option value="180">${esc(t('desktop.chess_clock_3'))}</option>
                                <option value="300">${esc(t('desktop.chess_clock_5'))}</option>
                                <option value="600">${esc(t('desktop.chess_clock_10'))}</option>
                            </select>
                        </div>
                        <div class="vd-chess-field">
                            <span class="vd-chess-label">${esc(t('desktop.chess_player_color'))}</span>
                            <div class="vd-chess-segment" data-color-group>
                                <button type="button" data-color="white" class="active">${esc(t('desktop.chess_color_white'))}</button>
                                <button type="button" data-color="black">${esc(t('desktop.chess_color_black'))}</button>
                            </div>
                        </div>
                        <div class="vd-chess-actions">
                            <button type="button" class="vd-chess-primary" data-new-game>${icon('refresh', 'N')}<span>${esc(t('desktop.chess_new_game'))}</span></button>
                            <button type="button" data-undo>${icon('undo', 'U')}<span>${esc(t('desktop.chess_undo'))}</span></button>
                            <button type="button" data-flip>${ctx.iconMarkup('refresh', 'F', 'vd-chess-action-icon', 14)}<span>${esc(t('desktop.chess_flip'))}</span></button>
                            <button type="button" data-hint>${icon('refresh', 'H')}<span>${esc(t('desktop.chess_hint'))}</span></button>
                            <button type="button" data-resign>${icon('refresh', 'R')}<span>${esc(t('desktop.chess_resign'))}</span></button>
                            <button type="button" data-draw>${icon('refresh', 'D')}<span>${esc(t('desktop.chess_offer_draw'))}</span></button>
                        </div>
                        <div class="vd-chess-replay" data-replay hidden>
                            <button type="button" data-review-first title="${esc(t('desktop.chess_first'))}">${icon('refresh', '|<', 'vd-chess-replay-icon')}</button>
                            <button type="button" data-review-prev title="${esc(t('desktop.chess_prev'))}">${icon('undo', '<', 'vd-chess-replay-icon')}</button>
                            <span class="vd-chess-review-label" data-review-label>${esc(t('desktop.chess_live'))}</span>
                            <button type="button" data-review-next title="${esc(t('desktop.chess_next'))}">${icon('refresh', '>', 'vd-chess-replay-icon')}</button>
                            <button type="button" data-review-last title="${esc(t('desktop.chess_last'))}">${icon('refresh', '>|', 'vd-chess-replay-icon')}</button>
                        </div>
                    </div>
                    <div class="vd-chess-tabs" data-tabs>
                        <button type="button" class="active" data-tab="moves">${esc(t('desktop.chess_tab_moves'))}</button>
                        <button type="button" data-tab="captured">${esc(t('desktop.chess_tab_captured'))}</button>
                        <button type="button" data-tab="info">${esc(t('desktop.chess_tab_info'))}</button>
                    </div>
                    <div class="vd-chess-moves" data-panel="moves">
                        <ol data-moves></ol>
                    </div>
                    <div class="vd-chess-captured-panel" data-panel="captured" hidden>
                        <div class="vd-chess-captured-row">
                            <span class="vd-chess-captured-label">${esc(t('desktop.chess_color_white'))}</span>
                            <div class="vd-chess-captured-pieces" data-captured-white></div>
                        </div>
                        <div class="vd-chess-captured-row">
                            <span class="vd-chess-captured-label">${esc(t('desktop.chess_color_black'))}</span>
                            <div class="vd-chess-captured-pieces" data-captured-black></div>
                        </div>
                    </div>
                    <div class="vd-chess-info-panel" data-panel="info" hidden>
                        <div class="vd-chess-info-row"><span>${esc(t('desktop.chess_material'))}</span><span data-material>0</span></div>
                        <div class="vd-chess-info-row"><span>${esc(t('desktop.chess_eval'))}</span><span data-eval>${esc(t('desktop.chess_even'))}</span></div>
                        <div class="vd-chess-info-row"><span>${esc(t('desktop.chess_hint'))}</span><span data-hint-text>${esc(t('desktop.chess_no_moves'))}</span></div>
                    </div>
                </aside>
            </div>
        `;
    }

    function collectRefs(state) {
        const root = state.container.querySelector('.vd-chess');
        state.refs = {
            root,
            boardShell: root.querySelector('.vd-chess-board-shell'),
            board: root.querySelector('[data-chess-board]'),
            status: root.querySelector('[data-status]'),
            comment: root.querySelector('[data-comment]'),
            mode: root.querySelector('[data-mode]'),
            strength: root.querySelector('[data-strength]'),
            strengthValue: root.querySelector('[data-strength-value]'),
            strengthWrap: root.querySelector('[data-strength-wrap]'),
            clockWrap: root.querySelector('[data-clock-wrap]'),
            clockSelect: root.querySelector('[data-clock-select]'),
            clocks: root.querySelector('[data-clocks]'),
            whiteTime: root.querySelector('[data-white-time]'),
            blackTime: root.querySelector('[data-black-time]'),
            moves: root.querySelector('[data-moves]'),
            tabs: root.querySelector('[data-tabs]'),
            panels: {
                moves: root.querySelector('[data-panel="moves"]'),
                captured: root.querySelector('[data-panel="captured"]'),
                info: root.querySelector('[data-panel="info"]')
            },
            capturedWhite: root.querySelector('[data-captured-white]'),
            capturedBlack: root.querySelector('[data-captured-black]'),
            material: root.querySelector('[data-material]'),
            evalEl: root.querySelector('[data-eval]'),
            hintText: root.querySelector('[data-hint-text]'),
            colorGroup: root.querySelector('[data-color-group]'),
            newGame: root.querySelector('[data-new-game]'),
            undo: root.querySelector('[data-undo]'),
            flip: root.querySelector('[data-flip]'),
            hint: root.querySelector('[data-hint]'),
            resign: root.querySelector('[data-resign]'),
            draw: root.querySelector('[data-draw]'),
            replay: root.querySelector('[data-replay]'),
            reviewFirst: root.querySelector('[data-review-first]'),
            reviewPrev: root.querySelector('[data-review-prev]'),
            reviewNext: root.querySelector('[data-review-next]'),
            reviewLast: root.querySelector('[data-review-last]'),
            reviewLabel: root.querySelector('[data-review-label]')
        };
    }

    function bindControls(state) {
        const refs = state.refs;
        refs.mode.addEventListener('change', () => {
            state.mode = refs.mode.value;
            state.opponent = state.mode === 'agent' ? 'agent' : 'computer';
            updateStrengthState(state);
            updateLocalMode(state);
            startNewGame(state);
        });
        refs.strength.addEventListener('input', () => {
            state.strength = Number(refs.strength.value) || 8;
            refs.strengthValue.textContent = String(state.strength);
        });
        refs.clockSelect.addEventListener('change', () => {
            state.clockBase = Number(refs.clockSelect.value) || 0;
            startNewGame(state);
        });
        refs.colorGroup.addEventListener('click', event => {
            const button = event.target.closest('[data-color]');
            if (!button) return;
            state.playerColor = button.dataset.color === 'black' ? 'black' : 'white';
            refs.colorGroup.querySelectorAll('[data-color]').forEach(btn => btn.classList.toggle('active', btn === button));
            state.flipped = false;
            startNewGame(state);
        });
        refs.newGame.addEventListener('click', () => startNewGame(state));
        refs.undo.addEventListener('click', () => undoMove(state));
        refs.flip.addEventListener('click', () => flipBoard(state));
        refs.hint.addEventListener('click', () => requestHint(state));
        refs.resign.addEventListener('click', () => confirmResign(state));
        refs.draw.addEventListener('click', () => confirmDraw(state));
        refs.reviewFirst.addEventListener('click', () => reviewJump(state, 0));
        refs.reviewPrev.addEventListener('click', () => reviewStep(state, -1));
        refs.reviewNext.addEventListener('click', () => reviewStep(state, 1));
        refs.reviewLast.addEventListener('click', () => reviewJump(state, -1));
        refs.tabs.addEventListener('click', event => {
            const button = event.target.closest('[data-tab]');
            if (!button) return;
            selectTab(state, button.dataset.tab);
        });
    }

    function createBoard(state) {
        const { Chessboard, COLOR, BORDER_TYPE, Markers } = state.vendor;
        attachBoardResizeObserver(state);
        fitBoardToShell(state);
        state.board = new Chessboard(state.refs.board, {
            position: state.game.fen(),
            orientation: state.playerColor === 'black' ? COLOR.black : COLOR.white,
            responsive: true,
            assetsUrl: '/img/chess/',
            assetsCache: true,
            style: {
                cssClass: 'green',
                showCoordinates: true,
                borderType: BORDER_TYPE.thin,
                pieces: { file: 'standard.svg', tileSize: 40 },
                animationDuration: 200
            },
            extensions: [{ class: Markers, props: { autoMarkers: null, sprite: 'markers.svg' } }]
        });
    }

    function attachBoardResizeObserver(state) {
        const resize = () => fitBoardToShell(state);
        state.resizeHandler = resize;
        if (window.ResizeObserver && state.refs.boardShell) {
            state.resizeObserver = new ResizeObserver(resize);
            state.resizeObserver.observe(state.refs.boardShell);
        } else {
            window.addEventListener('resize', resize);
        }
        window.requestAnimationFrame(resize);
    }

    function fitBoardToShell(state) {
        const shell = state.refs.boardShell;
        const board = state.refs.board;
        if (!shell || !board || state.disposed) return;
        const width = shell.clientWidth || shell.getBoundingClientRect().width;
        const height = shell.clientHeight || shell.getBoundingClientRect().height;
        const size = Math.floor(Math.min(width, height, 620));
        if (!Number.isFinite(size) || size < 220) return;
        const px = size + 'px';
        if (board.style.width !== px) board.style.width = px;
        if (board.style.height !== px) board.style.height = px;
    }

    function startNewGame(state) {
        if (!state.vendor || state.disposed) return;
        state.searchToken++;
        state.hintToken++;
        state.game = new state.vendor.Chess();
        state.pendingMove = null;
        state.agentComment = '';
        state.busy = false;
        state.lastMove = null;
        state.reviewIndex = -1;
        state.reviewActive = false;
        state.hintMove = null;
        state.gameOver = false;
        state.endedReason = '';
        state.captured = { w: [], b: [] };
        state.materialScore = 0;
        if (state.engine) state.engine.newGame();
        initClocks(state);
        syncBoard(state, false);
        updateAll(state);
        setWindowMenus(state);
        clearReview(state);
        if (!isPlayersTurn(state) && state.mode !== 'local') requestOpponentMove(state);
    }

    function initClocks(state) {
        stopClock(state);
        const base = state.clockBase;
        if (base > 0) {
            state.whiteTime = base;
            state.blackTime = base;
            state.refs.clocks.hidden = false;
            formatClocks(state);
            startClock(state);
        } else {
            state.whiteTime = 0;
            state.blackTime = 0;
            state.refs.clocks.hidden = true;
        }
    }

    function startClock(state) {
        stopClock(state);
        if (!state.clockBase || state.gameOver) return;
        state.clockTimer = setInterval(() => tickClock(state), 250);
    }

    function stopClock(state) {
        if (state.clockTimer) { clearInterval(state.clockTimer); state.clockTimer = null; }
    }

    function tickClock(state) {
        if (state.disposed || state.gameOver || state.reviewActive) return;
        if (!state.game || !isPlayersOrLocalTurnActive(state)) return;
        const turn = state.game.turn();
        if (turn === 'w') state.whiteTime = Math.max(0, state.whiteTime - 0.25);
        else state.blackTime = Math.max(0, state.blackTime - 0.25);
        formatClocks(state);
        if (state.whiteTime <= 0 || state.blackTime <= 0) {
            const loser = state.whiteTime <= 0 ? 'w' : 'b';
            endGame(state, 'time', loser);
        }
    }

    function isPlayersOrLocalTurnActive(state) {
        if (state.mode === 'local') return true;
        return isPlayersTurn(state);
    }

    function formatClocks(state) {
        state.refs.whiteTime.textContent = formatTime(state.whiteTime);
        state.refs.blackTime.textContent = formatTime(state.blackTime);
        state.refs.clocks.querySelectorAll('[data-clock]').forEach(el => {
            const side = el.dataset.clock;
            const active = !state.gameOver && state.game && ((side === 'white' && state.game.turn() === 'w') || (side === 'black' && state.game.turn() === 'b'));
            el.classList.toggle('is-active', active);
            el.classList.toggle('is-low', active && ((side === 'white' ? state.whiteTime : state.blackTime) < 30));
        });
    }

    function formatTime(seconds) {
        const s = Math.max(0, Math.ceil(seconds));
        const m = Math.floor(s / 60);
        const r = s % 60;
        return String(m).padStart(2, '0') + ':' + String(r).padStart(2, '0');
    }

    function boardInputHandler(state, event) {
        if (!state.game || state.busy || isGameOver(state) || state.reviewActive) return false;
        const type = event.type;
        if (type === state.vendor.INPUT_EVENT_TYPE.moveInputStarted) {
            return canStartMove(state, event.squareFrom);
        }
        if (type === state.vendor.INPUT_EVENT_TYPE.movingOverSquare) return true;
        if (type === state.vendor.INPUT_EVENT_TYPE.validateMoveInput) {
            return validatePlayerMove(state, event.squareFrom, event.squareTo);
        }
        if (type === state.vendor.INPUT_EVENT_TYPE.moveInputFinished) {
            const move = state.pendingMove;
            state.pendingMove = null;
            clearMoveMarkers(state);
            if (move) finishAppliedMove(state, move, '');
            return true;
        }
        if (type === state.vendor.INPUT_EVENT_TYPE.moveInputCanceled) {
            clearMoveMarkers(state);
        }
        return true;
    }

    function canStartMove(state, square) {
        if (!canPlayerMove(state)) return false;
        const piece = state.game.get(square);
        if (!piece) return false;
        if (state.mode !== 'local' && piece.color !== playerSide(state)) return false;
        clearMoveMarkers(state);
        if (state.hintMove) drawHintMarkers(state);
        const moves = state.game.moves({ square, verbose: true });
        if (state.board.addLegalMovesMarkers) state.board.addLegalMovesMarkers(moves);
        return moves.length > 0;
    }

    function canPlayerMove(state) {
        if (state.mode === 'local') return !state.busy && !isGameOver(state) && !state.reviewActive;
        return isPlayersTurn(state);
    }

    function validatePlayerMove(state, from, to) {
        if (!canPlayerMove(state)) return false;
        const moves = state.game.moves({ square: from, verbose: true }).filter(move => move.to === to);
        if (!moves.length) return false;
        const promotions = moves.filter(move => move.promotion);
        if (promotions.length) {
            const moverColor = state.game.turn();
            requestPromotionChoice(state, moverColor).then(promotion => {
                if (!promotion || state.disposed || !canPlayerMove(state)) return;
                applyPlayerMove(state, from, to, promotion);
            });
            return false;
        }
        const move = tryMove(state, { from, to });
        if (!move) return false;
        state.pendingMove = move;
        return true;
    }

    function applyPlayerMove(state, from, to, promotion) {
        const move = tryMove(state, { from, to, promotion });
        if (!move) return;
        clearMoveMarkers(state);
        syncBoard(state, true);
        finishAppliedMove(state, move, '');
    }

    function finishAppliedMove(state, move, comment) {
        if (move.captured) recordCapture(state, move.color, move.captured);
        state.agentComment = comment || '';
        state.lastMove = { from: move.from, to: move.to };
        state.hintMove = null;
        state.reviewIndex = -1;
        state.reviewActive = false;
        clearReview(state);
        playMoveSound(state, move);
        syncBoard(state, true);
        updateAll(state);
        updateLastMoveMarkers(state);
        updateCheckMarkers(state);
        if (isGameOver(state)) { handleGameOver(state); return; }
        if (state.mode === 'local') return;
        if (!isPlayersTurn(state)) requestOpponentMove(state);
    }

    function recordCapture(state, moverColor, capturedType) {
        const capturedColor = moverColor === 'w' ? 'b' : 'w';
        state.captured[capturedColor].push(capturedType);
        state.materialScore += moverColor === 'w' ? (PIECE_VALUES[capturedType] || 0) : -(PIECE_VALUES[capturedType] || 0);
    }

    async function requestOpponentMove(state) {
        const token = ++state.searchToken;
        state.busy = true;
        clearMoveMarkers(state);
        updateInput(state);
        setStatus(state, state.ctx.t(state.opponent === 'agent' ? 'desktop.chess_agent_thinking' : 'desktop.chess_engine_thinking', state.opponent === 'agent' ? 'Agent is thinking...' : 'Computer is thinking...'));
        try {
            const legalMoves = legalMovesUCI(state);
            let result;
            if (state.opponent === 'agent') {
                if (!state.agent) throw new Error('Agent client is not available.');
                result = await state.agent.chooseMove({
                    fen: state.game.fen(),
                    pgn: state.game.pgn(),
                    legal_moves: legalMoves,
                    side_to_move: state.game.turn(),
                    move_number: fullMoveNumber(state),
                    player_color: state.playerColor
                });
            } else {
                if (!state.engine) throw new Error('Stockfish is not available.');
                result = { move: await state.engine.findMove({ fen: state.game.fen(), legalMoves, strength: state.strength }) };
            }
            if (state.disposed || token !== state.searchToken) return;
            applyOpponentMove(state, result.move, result.comment || '');
        } catch (err) {
            if (state.disposed || token !== state.searchToken) return;
            notify(state, (err && err.message) || state.ctx.t('desktop.chess_move_failed'));
            setStatus(state, state.ctx.t('desktop.chess_move_failed'));
        } finally {
            if (!state.disposed && token === state.searchToken) {
                state.busy = false;
                updateAll(state);
            }
        }
    }

    function applyOpponentMove(state, uci, comment) {
        const moveText = String(uci || '').toLowerCase();
        const move = tryMove(state, {
            from: moveText.slice(0, 2),
            to: moveText.slice(2, 4),
            promotion: moveText.slice(4, 5) || undefined
        });
        if (!move) throw new Error('Opponent returned an illegal move.');
        finishAppliedMove(state, move, comment);
    }

    function undoMove(state) {
        if (!state.game || state.busy || state.game.history().length === 0 || state.reviewActive) return;
        state.searchToken++;
        state.hintToken++;
        const history = state.game.history({ verbose: true });
        const undone = history[history.length - 1];
        if (undone && undone.captured) {
            const list = state.captured[undone.color === 'w' ? 'b' : 'w'];
            const idx = list.lastIndexOf(undone.captured);
            if (idx !== -1) list.splice(idx, 1);
            state.materialScore -= undone.color === 'w' ? (PIECE_VALUES[undone.captured] || 0) : -(PIECE_VALUES[undone.captured] || 0);
        }
        state.game.undo();
        if (state.mode !== 'local' && state.game.history().length && state.game.turn() !== playerSide(state)) state.game.undo();
        state.pendingMove = null;
        state.agentComment = '';
        state.hintMove = null;
        clearMoveMarkers(state);
        syncBoard(state, true);
        updateAll(state);
        updateLastMoveMarkers(state);
    }

    function flipBoard(state) {
        state.flipped = !state.flipped;
        updateOrientation(state, true);
    }

    async function requestHint(state) {
        if (!state.game || state.busy || state.gameOver || state.reviewActive) return;
        if (state.mode !== 'local' && !isPlayersTurn(state)) return;
        const token = ++state.hintToken;
        state.refs.hintText.textContent = state.ctx.t('desktop.chess_hint_thinking');
        setStatus(state, state.ctx.t('desktop.chess_hint_thinking'));
        try {
            let uci;
            if (state.engine) {
                uci = await state.engine.findMove({ fen: state.game.fen(), legalMoves: legalMovesUCI(state), strength: Math.max(4, Math.min(10, state.strength)) });
            } else {
                const moves = state.game.moves({ verbose: true });
                if (!moves.length) throw new Error('no moves');
                uci = moves[0].from + moves[0].to + (moves[0].promotion || '');
            }
            if (state.disposed || token !== state.hintToken) return;
            state.hintMove = uci;
            drawHintMarkers(state);
            const moveText = state.ctx.t('desktop.chess_hint') + ': ' + String(uci || '').toUpperCase();
            state.refs.hintText.textContent = moveText;
            setStatus(state, moveText);
        } catch (err) {
            if (state.disposed || token !== state.hintToken) return;
            state.refs.hintText.textContent = state.ctx.t('desktop.chess_hint_failed');
            setStatus(state, state.ctx.t('desktop.chess_hint_failed'));
        }
    }

    function drawHintMarkers(state) {
        if (!state.hintMove || !state.board || !state.vendor || !state.vendor.MARKER_TYPE) return;
        const MT = state.vendor.MARKER_TYPE;
        const from = String(state.hintMove).slice(0, 2);
        const to = String(state.hintMove).slice(2, 4);
        try {
            state.board.removeMarkers(MT.frame);
            state.board.addMarker(MT.frame, from);
            state.board.addMarker(MT.frame, to);
        } catch (e) {}
    }

    function confirmResign(state) {
        if (!state.game || state.gameOver) return;
        showModal(state, state.ctx.t('desktop.chess_confirm_resign'), [
            { label: state.ctx.t('desktop.chess_resign'), primary: true, action: () => { endGame(state, 'resign', playerSide(state)); } },
            { label: state.ctx.t('desktop.chess_flip'), action: null }
        ]);
    }

    function confirmDraw(state) {
        if (!state.game || state.gameOver) return;
        showModal(state, state.ctx.t('desktop.chess_confirm_draw'), [
            { label: state.ctx.t('desktop.chess_offer_draw'), primary: true, action: () => { endGame(state, 'draw', null); } },
            { label: state.ctx.t('desktop.chess_flip'), action: null }
        ]);
    }

    function endGame(state, reason, loserSide) {
        state.gameOver = true;
        state.endedReason = reason;
        stopClock(state);
        state.busy = false;
        state.searchToken++;
        let message;
        if (reason === 'draw') {
            message = state.ctx.t('desktop.chess_draw');
        } else if (reason === 'time') {
            message = (loserSide === 'w' ? state.ctx.t('desktop.chess_black_wins') : state.ctx.t('desktop.chess_white_wins'));
        } else if (reason === 'resign') {
            message = state.ctx.t('desktop.chess_resigned') + ' — ' + (loserSide === 'w' ? state.ctx.t('desktop.chess_black_wins') : state.ctx.t('desktop.chess_white_wins'));
        }
        setStatus(state, message);
        playGameOverSound(state);
        updateAll(state);
    }

    function handleGameOver(state) {
        state.gameOver = true;
        stopClock(state);
        let message;
        if (callBool(state.game, 'isCheckmate', 'in_checkmate')) {
            const winner = state.game.turn() === 'w' ? 'b' : 'w';
            const winLabel = winner === 'w' ? state.ctx.t('desktop.chess_white_wins') : state.ctx.t('desktop.chess_black_wins');
            message = state.ctx.t('desktop.chess_checkmate') + ' — ' + winLabel;
            if (state.mode !== 'local') {
                message += (winner === playerSide(state)) ? ' (' + state.ctx.t('desktop.chess_game_over_win') + ')' : ' (' + state.ctx.t('desktop.chess_game_over_loss') + ')';
            }
        } else if (callBool(state.game, 'isStalemate', 'in_stalemate')) {
            message = state.ctx.t('desktop.chess_stalemate');
        } else if (callBool(state.game, 'isDraw', 'in_draw')) {
            message = state.ctx.t('desktop.chess_draw');
        }
        state.endedReason = 'gameover';
        setStatus(state, message);
        playGameOverSound(state);
        updateAll(state);
    }

    function tryMove(state, move) {
        try {
            return state.game.move(move);
        } catch (err) {
            return null;
        }
    }

    function syncBoard(state, animated) {
        if (!state.board || !state.game) return;
        state.board.setPosition(state.game.fen(), !!animated).catch(() => {});
        updateOrientation(state, false);
    }

    function updateOrientation(state, animated) {
        if (!state.board || !state.vendor) return;
        const orientation = state.flipped ? oppositeColor(state.playerColor) : state.playerColor;
        const color = orientation === 'black' ? state.vendor.COLOR.black : state.vendor.COLOR.white;
        state.board.setOrientation(color, !!animated).catch(() => {});
    }

    function updateLastMoveMarkers(state) {
        if (!state.board || !state.vendor || !state.vendor.MARKER_TYPE) return;
        const MT = state.vendor.MARKER_TYPE;
        try {
            state.board.removeMarkers(MT.framePrimary);
            if (state.lastMove) {
                state.board.addMarker(MT.framePrimary, state.lastMove.from);
                state.board.addMarker(MT.framePrimary, state.lastMove.to);
            }
        } catch (e) {}
    }

    function updateCheckMarkers(state) {
        if (!state.board || !state.vendor || !state.vendor.MARKER_TYPE) return;
        const MT = state.vendor.MARKER_TYPE;
        try {
            state.board.removeMarkers(MT.frameDanger);
            if (callBool(state.game, 'inCheck', 'in_check')) {
                const king = findKing(state.game, state.game.turn());
                if (king) state.board.addMarker(MT.frameDanger, king);
            }
        } catch (e) {}
    }

    function findKing(game, color) {
        const fen = game.fen().split(' ')[0];
        const rows = fen.split('/');
        const boardRows = color === 'w' ? rows.slice().reverse() : rows;
        for (let r = 0; r < 8; r++) {
            let col = 0;
            for (const ch of boardRows[r]) {
                if (/\d/.test(ch)) { col += Number(ch); continue; }
                if (ch.toLowerCase() === 'k' && ((color === 'w' && ch === 'K') || (color === 'b' && ch === 'k'))) return 'abcdefgh'[col] + (r + 1);
                col++;
            }
        }
        return null;
    }

    function updateAll(state) {
        renderMoves(state);
        updateStatus(state);
        updateStrengthState(state);
        updateLocalMode(state);
        updateInput(state);
        updateCaptured(state);
        updateInfo(state);
        setWindowMenus(state);
        formatClocks(state);
        updateReplayControls(state);
    }

    function updateInput(state) {
        if (!state.board || !state.vendor) return;
        if (state.busy || state.gameOver || state.reviewActive || !canPlayerMove(state)) {
            state.board.disableMoveInput();
            return;
        }
        const color = playerSide(state) === 'b' ? state.vendor.COLOR.black : state.vendor.COLOR.white;
        state.board.enableMoveInput(event => boardInputHandler(state, event), state.mode === 'local' ? null : color);
    }

    function updateStatus(state) {
        if (!state.game) return;
        let message;
        if (state.gameOver) {
            message = state.refs.status.textContent || state.ctx.t('desktop.chess_title');
        } else if (callBool(state.game, 'isCheckmate', 'in_checkmate')) {
            const winner = state.game.turn() === 'w' ? 'b' : 'w';
            message = state.ctx.t('desktop.chess_checkmate') + ' — ' + (winner === 'w' ? state.ctx.t('desktop.chess_white_wins') : state.ctx.t('desktop.chess_black_wins'));
        } else if (callBool(state.game, 'isStalemate', 'in_stalemate')) {
            message = state.ctx.t('desktop.chess_stalemate');
        } else if (callBool(state.game, 'isDraw', 'in_draw')) {
            message = state.ctx.t('desktop.chess_draw');
        } else if (callBool(state.game, 'inCheck', 'in_check')) {
            message = state.ctx.t('desktop.chess_check');
        } else if (state.busy) {
            message = state.ctx.t('desktop.chess_thinking');
        } else if (state.reviewActive) {
            message = state.ctx.t('desktop.chess_reviewing').replace('{n}', String(state.reviewIndex + 1));
        } else if (state.mode === 'local') {
            message = state.game.turn() === 'w' ? state.ctx.t('desktop.chess_local_turn_white') : state.ctx.t('desktop.chess_local_turn_black');
        } else {
            message = isPlayersTurn(state) ? state.ctx.t('desktop.chess_your_turn') : state.ctx.t('desktop.chess_opponent_turn');
        }
        setStatus(state, message);
        state.refs.comment.textContent = state.agentComment || (state.game.turn() === 'w' ? state.ctx.t('desktop.chess_white_to_move') : state.ctx.t('desktop.chess_black_to_move'));
    }

    function renderMoves(state) {
        const moves = state.game ? state.game.history({ verbose: true }) : [];
        if (!moves.length) {
            state.refs.moves.innerHTML = `<li class="vd-chess-empty">${state.ctx.esc(state.ctx.t('desktop.chess_no_moves'))}</li>`;
            return;
        }
        const rows = [];
        for (let i = 0; i < moves.length; i += 2) {
            const white = moves[i];
            const black = moves[i + 1];
            const moveNo = Math.floor(i / 2) + 1;
            const cls = (state.reviewActive && (state.reviewIndex === i || state.reviewIndex === i + 1)) ? ' is-review' : '';
            rows.push(`<li class="vd-chess-move-row${cls}" data-move-idx="${i}"><span class="vd-chess-move-no">${moveNo}.</span><span class="vd-chess-move-cell" data-jump="${i}">${state.ctx.esc(white ? white.san : '')}</span><span class="vd-chess-move-cell" data-jump="${i + 1}">${state.ctx.esc(black ? black.san : '')}</span></li>`);
        }
        state.refs.moves.innerHTML = rows.join('');
        state.refs.moves.scrollTop = state.refs.moves.scrollHeight;
        state.refs.moves.querySelectorAll('[data-jump]').forEach(cell => {
            cell.addEventListener('click', () => {
                const idx = Number(cell.dataset.jump);
                if (moves[idx]) reviewJump(state, idx);
            });
        });
    }

    function updateStrengthState(state) {
        const disabled = state.mode === 'agent' || state.mode === 'local';
        state.refs.strength.disabled = disabled;
        state.refs.strengthWrap.classList.toggle('is-disabled', disabled);
        state.refs.strengthValue.textContent = String(state.strength);
    }

    function updateLocalMode(state) {
        const local = state.mode === 'local';
        state.refs.colorGroup.classList.toggle('is-disabled', local);
        state.refs.colorGroup.querySelectorAll('button').forEach(b => b.disabled = local);
        state.refs.hint.hidden = local;
        state.refs.resign.hidden = false;
        state.refs.draw.hidden = false;
    }

    function updateCaptured(state) {
        const renderPieces = list => {
            if (!list.length) return '<span class="vd-chess-captured-empty">—</span>';
            const sorted = list.slice().sort((a, b) => PIECE_ORDER.indexOf(a) - PIECE_ORDER.indexOf(b));
            return sorted.map(p => `<span class="vd-chess-piece vd-chess-piece-${p}">${PIECE_GLYPH[p]}</span>`).join('');
        };
        state.refs.capturedWhite.innerHTML = renderPieces(state.captured.b);
        state.refs.capturedBlack.innerHTML = renderPieces(state.captured.w);
        const wMat = state.captured.b.reduce((s, p) => s + (PIECE_VALUES[p] || 0), 0);
        const bMat = state.captured.w.reduce((s, p) => s + (PIECE_VALUES[p] || 0), 0);
        state.refs.material.textContent = (wMat - bMat > 0 ? '+' : '') + String(wMat - bMat);
    }

    function updateInfo(state) {
        const diff = state.materialScore;
        let evalText;
        if (Math.abs(diff) < 1) evalText = state.ctx.t('desktop.chess_even');
        else if (diff > 0) evalText = state.ctx.t('desktop.chess_advantage_white').replace('{n}', String(Math.abs(diff)));
        else evalText = state.ctx.t('desktop.chess_advantage_black').replace('{n}', String(Math.abs(diff)));
        state.refs.evalEl.textContent = evalText;
        state.refs.hintText.textContent = state.hintMove ? (state.ctx.t('desktop.chess_hint') + ': ' + String(state.hintMove).toUpperCase()) : state.ctx.t('desktop.chess_no_moves');
    }

    function selectTab(state, tab) {
        state.activeTab = tab;
        state.refs.tabs.querySelectorAll('[data-tab]').forEach(btn => btn.classList.toggle('active', btn.dataset.tab === tab));
        Object.keys(state.refs.panels).forEach(key => {
            state.refs.panels[key].hidden = key !== tab;
        });
    }

    function reviewStep(state, dir) {
        if (!state.game) return;
        const total = state.game.history().length;
        let next = state.reviewActive ? state.reviewIndex + dir : (total - 1 + (dir < 0 ? dir : 0));
        reviewJump(state, Math.max(0, Math.min(total - 1, next)));
    }

    function reviewJump(state, idx) {
        if (!state.game) return;
        const total = state.game.history().length;
        if (idx < 0 || idx >= total) { clearReview(state); updateAll(state); return; }
        state.reviewActive = true;
        state.reviewIndex = idx;
        const tempGame = new state.vendor.Chess();
        const history = state.game.history({ verbose: true });
        for (let i = 0; i <= idx; i++) tempGame.move(history[i]);
        state.board.setPosition(tempGame.fen(), true).catch(() => {});
        updateAll(state);
    }

    function clearReview(state) {
        state.reviewActive = false;
        state.reviewIndex = -1;
        if (state.game && state.board) state.board.setPosition(state.game.fen(), false).catch(() => {});
        state.refs.reviewLabel.textContent = state.ctx.t('desktop.chess_live');
    }

    function updateReplayControls(state) {
        const total = state.game ? state.game.history().length : 0;
        state.refs.replay.hidden = total < 2;
        if (state.reviewActive) {
            state.refs.reviewLabel.textContent = state.ctx.t('desktop.chess_reviewing').replace('{n}', String(state.reviewIndex + 1));
        } else {
            state.refs.reviewLabel.textContent = state.ctx.t('desktop.chess_live');
        }
    }

    function setWindowMenus(state) {
        if (typeof state.ctx.setWindowMenus !== 'function') return;
        state.ctx.setWindowMenus(state.windowId, [
            {
                id: 'game',
                labelKey: 'desktop.menu_game',
                items: [
                    { id: 'new', labelKey: 'desktop.chess_new_game', icon: 'refresh', shortcut: 'Ctrl+N', action: () => startNewGame(state) },
                    { id: 'undo', labelKey: 'desktop.chess_undo', icon: 'undo', disabled: !state.game || !state.game.history().length || state.busy, shortcut: 'Ctrl+Z', action: () => undoMove(state) },
                    { id: 'hint', labelKey: 'desktop.chess_hint', icon: 'refresh', action: () => requestHint(state) }
                ]
            },
            {
                id: 'view',
                labelKey: 'desktop.menu_view',
                items: [
                    { id: 'flip', labelKey: 'desktop.chess_flip', icon: 'refresh', action: () => flipBoard(state) }
                ]
            }
        ]);
    }

    function requestPromotionChoice(state, color) {
        const existing = state.refs.root.querySelector('.vd-chess-promotion');
        if (existing) existing.remove();
        return new Promise(resolve => {
            const choices = ['q', 'r', 'b', 'n'];
            const names = {
                q: state.ctx.t('desktop.chess_promo_queen'),
                r: state.ctx.t('desktop.chess_promo_rook'),
                b: state.ctx.t('desktop.chess_promo_bishop'),
                n: state.ctx.t('desktop.chess_promo_knight')
            };
            const dialog = document.createElement('div');
            dialog.className = 'vd-chess-promotion';
            dialog.innerHTML = choices.map(piece => `<button type="button" data-piece="${piece}"><span class="vd-chess-promo-glyph vd-chess-piece-${piece}">${PIECE_GLYPH[piece]}</span><span>${state.ctx.esc(names[piece])}</span></button>`).join('');
            dialog.addEventListener('click', event => {
                const button = event.target.closest('[data-piece]');
                if (!button) return;
                dialog.remove();
                resolve(button.dataset.piece);
            });
            state.refs.root.appendChild(dialog);
            const first = dialog.querySelector('button');
            if (first) first.focus();
            window.setTimeout(() => {
                if (!dialog.isConnected) return;
                dialog.classList.toggle('is-black', color === 'b');
            }, 0);
        });
    }

    function showModal(state, message, buttons) {
        const existing = state.refs.root.querySelector('.vd-chess-modal');
        if (existing) existing.remove();
        const dialog = document.createElement('div');
        dialog.className = 'vd-chess-modal';
        const esc = state.ctx.esc;
        const btnHtml = buttons.map((b, i) => `<button type="button" data-idx="${i}" class="${b.primary ? 'is-primary' : ''}">${esc(b.label)}</button>`).join('');
        dialog.innerHTML = `<div class="vd-chess-modal-card"><div class="vd-chess-modal-text">${esc(message)}</div><div class="vd-chess-modal-actions">${btnHtml}</div></div>`;
        dialog.addEventListener('click', event => {
            const button = event.target.closest('[data-idx]');
            if (!button) return;
            const idx = Number(button.dataset.idx);
            dialog.remove();
            const action = buttons[idx] && buttons[idx].action;
            if (typeof action === 'function') action();
        });
        state.refs.root.appendChild(dialog);
    }

    function clearMoveMarkers(state) {
        if (state.board && state.board.removeLegalMovesMarkers) state.board.removeLegalMovesMarkers();
        if (state.hintMove && state.board && state.vendor && state.vendor.MARKER_TYPE) {
            try { state.board.removeMarkers(state.vendor.MARKER_TYPE.frame); } catch (e) {}
        }
    }

    function legalMovesUCI(state) {
        return state.game.moves({ verbose: true }).map(move => move.from + move.to + (move.promotion || ''));
    }

    function isPlayersTurn(state) {
        return state.game && state.game.turn() === playerSide(state);
    }

    function playerSide(state) {
        return state.playerColor === 'black' ? 'b' : 'w';
    }

    function oppositeColor(color) {
        return color === 'black' ? 'white' : 'black';
    }

    function fullMoveNumber(state) {
        const parts = state.game.fen().split(/\s+/);
        return Number(parts[5]) || 1;
    }

    function isGameOver(state) {
        return state.gameOver || (state.game && callBool(state.game, 'isGameOver', 'game_over'));
    }

    function callBool(target, modern, legacy) {
        if (target && typeof target[modern] === 'function') return !!target[modern]();
        if (target && typeof target[legacy] === 'function') return !!target[legacy]();
        return false;
    }

    function setStatus(state, message) {
        if (state.refs.status) state.refs.status.textContent = message;
    }

    function notify(state, message) {
        if (typeof state.ctx.notify === 'function') {
            state.ctx.notify({ title: state.ctx.t('desktop.chess_title'), message });
        }
    }

    // --- Web Audio synthesized chess sounds (no sample files) ---
    function createChessAudio(state) {
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

    function playMoveSound(state, move) {
        if (!state.audio) return;
        const flags = move.flags || '';
        if (move.captured) state.audio.capture();
        else if (flags.indexOf('k') !== -1 || flags.indexOf('q') !== -1) state.audio.castle();
        else if (move.promotion) state.audio.promote();
        else state.audio.move();
        if (callBool(state.game, 'inCheck', 'in_check')) state.audio.check();
    }

    function playGameOverSound(state) {
        if (!state.audio) return;
        const win = state.mode !== 'local' && state.endedReason !== 'draw' && !isPlayersTurn(state) === false && state.game && callBool(state.game, 'isCheckmate', 'in_checkmate');
        state.audio.gameOver(win);
    }

    async function defaultAPI(url, options) {
        const resp = await fetch(url, Object.assign({ credentials: 'same-origin', cache: 'no-store' }, options || {}));
        const data = await resp.json().catch(() => ({}));
        if (!resp.ok) throw new Error(data.error || data.message || ('HTTP ' + resp.status));
        return data;
    }

    window.ChessApp = { render, dispose };
})();