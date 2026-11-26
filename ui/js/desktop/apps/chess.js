(function () {
    'use strict';
    const instances = new Map();
    let vendorPromise = null;

    function loadVendor() {
        if (!vendorPromise) vendorPromise = import('/js/vendor/chess-vendor.esm.js');
        return vendorPromise;
    }

    const PIECE_VALUES = { p: 1, n: 3, b: 3, r: 5, q: 9, k: 0 };
    const PIECE_GLYPH = { q: '♛', r: '♜', b: '♝', n: '♞', p: '♟', k: '♚' };

    function render(container, windowId, context = {}) {
        dispose(windowId);
        const ctx = normalizeContext(context);
        const state = {
            windowId, container, ctx, disposed: false,
            vendor: null, game: null, board: null, view3d: null, viewMode: '2d',
            engine: null, agent: null, audio: null, commentary: null, fx: null,
            resizeObserver: null, resizeHandler: null, keyHandler: null,
            playerColor: 'white', mode: 'computer', opponent: 'computer', strength: 8,
            boardSkin: (window.ChessBoardSkin && window.ChessBoardSkin.load) ? window.ChessBoardSkin.load() : 'green',
            soundMuted: (window.ChessSoundPref && window.ChessSoundPref.load) ? window.ChessSoundPref.load() : false,
            clockBase: 0, flipped: false, busy: false, pendingMove: null,
            searchToken: 0, hintToken: 0, agentComment: '',
            commentaryText: '', commentarySpeaking: false, commentaryAudioUnavailable: false,
            lastMove: null, reviewIndex: -1, reviewActive: false, reviewFen: '', reviewMove: null,
            hintMove: null, selectedSquare: null, clockTimer: null, whiteTime: 0, blackTime: 0, lastTickSecond: -1,
            activeTab: 'moves', captured: { w: [], b: [] }, prevCapturedCounts: { w: 0, b: 0 },
            materialScore: 0, gameOver: false, endedReason: '', refs: {}
        };
        instances.set(windowId, state);
        container.innerHTML = window.renderChessTemplate
            ? window.renderChessTemplate(ctx, PIECE_GLYPH)
            : '<div class="vd-chess is-error"><div class="vd-chess-board-pane"><div class="vd-chess-pane-head"><div class="vd-chess-status" data-status>' + ctx.esc(ctx.t('desktop.chess_load_failed')) + '</div></div></div></div>';
        collectRefs(state);
        window.ChessTable?.mount(state);
        state.audio = window.createChessAudio ? window.createChessAudio({ muted: state.soundMuted }) : null;
        state.commentary = window.createChessCommentary
            ? window.createChessCommentary({
                api: ctx.api,
                getAudio: () => state.audio,
                onUpdate: info => onCommentaryUpdate(state, info)
            })
            : null;
        bindControls(state);
        window.ChessChrome?.syncSoundUI?.(state);
        window.ChessChrome?.syncCommentaryUI?.(state);
        setWindowMenus(state);
        setStatus(state, ctx.t('desktop.chess_loading'));

        loadVendor()
            .then(vendor => {
                if (state.disposed) return;
                state.vendor = vendor;
                state.game = new vendor.Chess();
                state.engine = window.createChessEngine ? window.createChessEngine({}) : null;
                state.agent = window.createChessAgentClient ? window.createChessAgentClient({ api: ctx.api }) : null;
                state.fx = window.createChessFx
                    ? window.createChessFx({ root: state.refs.root, boardShell: state.refs.boardShell, t: ctx.t })
                    : null;
                createBoard(state);
                startNewGame(state);
            })
            .catch(err => {
                setStatus(state, ctx.t('desktop.chess_load_failed'));
                if (state.refs.root) state.refs.root.classList.add('is-error');
            });
    }

    const quiet = step => { try { step(); } catch (e) {} };

    function dispose(windowId) {
        const state = instances.get(windowId);
        if (!state) return;
        state.disposed = true;
        state.searchToken++;
        state.hintToken++;
        stopClock(state);
        quiet(() => state.commentary?.dispose());
        state.audio?.dispose();
        quiet(() => state.fx?.dispose());
        quiet(() => window.ChessView3D?.dispose(state));
        quiet(() => state.board?.destroy());
        quiet(() => state.resizeObserver?.disconnect());
        if (state.resizeHandler) window.removeEventListener('resize', state.resizeHandler);
        if (state.keyHandler && state.refs.root) state.refs.root.removeEventListener('keydown', state.keyHandler);
        quiet(() => state.engine?.dispose());
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

    function collectRefs(state) {
        const root = state.container.querySelector('.vd-chess');
        if (!root) { state.refs = { root: null }; return; }
        const q = sel => root.querySelector(sel);
        state.refs = {
            root,
            boardShell: q('.vd-chess-board-shell'),
            boardFrame: q('[data-chess-board-frame]'),
            board: q('[data-chess-board]'),
            board3d: q('[data-chess-board-3d]'),
            viewToggle: q('[data-chess-view-toggle]'),
            soundToggle: q('[data-sound-toggle]'),
            commentaryToggle: q('[data-commentary-toggle]'),
            resetCam: q('[data-chess-reset-cam]'),
            status: q('[data-status]'),
            comment: q('[data-comment]'),
            mode: q('[data-mode]'),
            strength: q('[data-strength]'),
            strengthValue: q('[data-strength-value]'),
            strengthWrap: q('[data-strength-wrap]'),
            clockWrap: q('[data-clock-wrap]'),
            clockSelect: q('[data-clock-select]'),
            boardSkin: q('[data-board-skin]'),
            moves: q('[data-moves]'),
            movesCount: q('[data-moves-count]'),
            tabs: q('[data-tabs]'),
            panels: { moves: q('[data-panel="moves"]'), captured: q('[data-panel="captured"]'), info: q('[data-panel="info"]') },
            capturedWhite: q('[data-captured-white]'),
            capturedBlack: q('[data-captured-black]'),
            phaseEl: q('[data-phase]'),
            material: q('[data-material]'),
            materialFill: q('[data-material-fill]'),
            materialBarScore: q('[data-material-bar-score]'),
            evalEl: q('[data-eval]'),
            hintText: q('[data-hint-text]'),
            copyPgn: q('[data-copy-pgn]'),
            copyFen: q('[data-copy-fen]'),
            colorGroup: q('[data-color-group]'),
            newGame: q('[data-new-game]'),
            undo: q('[data-undo]'),
            flip: q('[data-flip]'),
            hint: q('[data-hint]'),
            resign: q('[data-resign]'),
            draw: q('[data-draw]'),
            replay: q('[data-replay]'),
            reviewFirst: q('[data-review-first]'),
            reviewPrev: q('[data-review-prev]'),
            reviewNext: q('[data-review-next]'),
            reviewLast: q('[data-review-last]'),
            reviewLabel: q('[data-review-label]')
        };
    }

    function bindControls(state) {
        const refs = state.refs;
        if (!refs.root) return;
        state.onViewChanged = () => setWindowMenus(state);
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
        if (refs.boardSkin) {
            refs.boardSkin.value = state.boardSkin || 'green';
            refs.boardSkin.addEventListener('change', () => {
                const skinApi = window.ChessBoardSkin;
                state.boardSkin = skinApi ? skinApi.normalize(refs.boardSkin.value) : 'green';
                if (skinApi) { skinApi.save(state.boardSkin); skinApi.apply(state); }
                updateLastMoveMarkers(state);
                updateCheckMarkers(state);
                if (window.ChessView3D) window.ChessView3D.sync(state);
            });
        }
        if (refs.soundToggle) refs.soundToggle.addEventListener('click', () => toggleSound(state));
        if (refs.commentaryToggle) refs.commentaryToggle.addEventListener('click', () => toggleCommentary(state));
        if (refs.resetCam) refs.resetCam.addEventListener('click', () => window.ChessView3D?.resetCamera?.(state));
        if (refs.copyPgn) refs.copyPgn.addEventListener('click', () => window.ChessInfoTools?.copyChessNotation(state.game, 'pgn', () => notify(state, state.ctx.t('desktop.chess_copied'))));
        if (refs.copyFen) refs.copyFen.addEventListener('click', () => window.ChessInfoTools?.copyChessNotation(state.game, 'fen', () => notify(state, state.ctx.t('desktop.chess_copied'))));
        if (window.ChessView3D) {
            window.ChessView3D.bind(state, {
                canMove: () => canPlayerMove(state),
                legalMoves: square => state.game.moves({ square, verbose: true }),
                onSelect: () => { if (state.audio && typeof state.audio.select === 'function') state.audio.select(); },
                onMove: (from, to) => requestPlayerMove(state, from, to)
            });
        }
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
        window.ChessTabs.bind(refs.tabs, tab => selectTab(state, tab));
        state.keyHandler = event => handleKeyShortcuts(state, event);
        refs.root.addEventListener('keydown', state.keyHandler);
    }

    function handleKeyShortcuts(state, event) {
        if (event.defaultPrevented || event.ctrlKey || event.metaKey || event.altKey) return;
        const target = event.target;
        if (target && (target.closest('select, input, textarea') || target.closest('[data-tabs]'))) return;
        if (state.refs.root && state.refs.root.querySelector('.vd-chess-modal, .vd-chess-promotion-backdrop')) return;
        const key = event.key;
        if (key === 'f' || key === 'F') { event.preventDefault(); flipBoard(state); }
        else if (key === 'm' || key === 'M') { event.preventDefault(); toggleSound(state); }
        else if (key === 'v' || key === 'V') { event.preventDefault(); toggleCommentary(state); }
        else if (key === 'ArrowLeft' && state.game && state.game.history().length > 0 && !target.closest('.cm-chessboard')) {
            event.preventDefault();
            reviewStep(state, -1);
        } else if (key === 'ArrowRight' && state.reviewActive && !target.closest('.cm-chessboard')) {
            event.preventDefault();
            if (state.reviewIndex >= state.game.history().length - 1) { clearReview(state); updateAll(state); }
            else reviewStep(state, 1);
        }
    }

    function toggleSound(state) {
        state.soundMuted = !state.soundMuted;
        state.audio?.setMuted?.(state.soundMuted);
        window.ChessChrome?.syncSoundUI?.(state);
        setWindowMenus(state);
        if (!state.soundMuted) state.audio?.select?.();
    }

    function toggleCommentary(state) {
        if (!state.commentary) return;
        state.commentary.toggle();
        window.ChessChrome?.syncCommentaryUI?.(state);
        updateStatus(state);
        setWindowMenus(state);
    }

    function onCommentaryUpdate(state, info) {
        state.commentaryText = info && info.text ? String(info.text) : '';
        state.commentarySpeaking = !!(info && (info.speaking || info.loading));
        state.commentaryAudioUnavailable = !!(info && info.audioUnavailable);
        window.ChessChrome?.syncCommentaryUI?.(state);
        updateStatus(state);
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
                cssClass: (window.ChessBoardSkin ? window.ChessBoardSkin.normalize(state.boardSkin) : 'green'),
                /* The coordinates are engraved in the wood rim (chess-table.js), not inside the squares. */
                showCoordinates: false,
                borderType: BORDER_TYPE.thin,
                pieces: { file: 'standard.svg', tileSize: 40 },
                animationDuration: 280
            },
            extensions: [{ class: Markers, props: { autoMarkers: null, sprite: 'markers.svg' } }]
        });
        window.ChessBoardSkin?.apply(state);
    }

    function attachBoardResizeObserver(state) {
        const resize = () => fitBoardToShell(state);
        state.resizeHandler = resize;
        state.refitBoard = resize;
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
        const frame = state.refs.boardFrame || state.refs.board;
        const board = state.refs.board;
        if (!shell || !frame || !board || state.disposed) return;
        const styles = window.getComputedStyle(shell);
        const padX = (parseFloat(styles.paddingLeft) || 0) + (parseFloat(styles.paddingRight) || 0);
        const padY = (parseFloat(styles.paddingTop) || 0) + (parseFloat(styles.paddingBottom) || 0);
        const width = Math.max(0, (shell.clientWidth || shell.getBoundingClientRect().width || 0) - padX);
        const height = Math.max(0, (shell.clientHeight || shell.getBoundingClientRect().height || 0) - padY);
        /* The board is square, so the free height of the pane is the only lever
           that makes it bigger. The cap only keeps very tall screens sane. */
        const size = Math.floor(Math.min(width, height, 1200) - 8);
        if (!Number.isFinite(size) || size < 180) return;
        state.refs.root.style.setProperty('--chess-frame-size', size + 'px');
        frame.classList.add('is-sized');
        board.style.width = '100%';
        board.style.height = '100%';
        board.style.minWidth = '0';
        board.style.minHeight = '0';
        if (state.board && state.board.view && typeof state.board.view.handleResize === 'function') {
            state.board.view.handleResize();
        }
    }

    function startNewGame(state) {
        if (!state.vendor || state.disposed) return;
        state.searchToken++;
        state.hintToken++;
        state.game = new state.vendor.Chess();
        state.pendingMove = null;
        state.agentComment = '';
        state.commentaryText = '';
        state.busy = false;
        state.lastMove = null;
        state.reviewIndex = -1;
        state.reviewActive = false;
        state.reviewFen = '';
        state.reviewMove = null;
        state.hintMove = null;
        state.gameOver = false;
        state.endedReason = '';
        state.lastTickSecond = -1;
        state.captured = { w: [], b: [] };
        state.prevCapturedCounts = { w: 0, b: 0 };
        state.materialScore = 0;
        state.commentary?.resetGame();
        if (state.fx) { state.fx.dismissResult(); state.fx.setThinking(false); }
        state.engine?.newGame();
        initClocks(state);
        syncBoard(state, false);
        clearMoveMarkers(state);
        updateLastMoveMarkers(state);
        updateCheckMarkers(state);
        updateAll(state);
        setWindowMenus(state);
        clearReview(state);
        if (!isPlayersTurn(state) && state.mode !== 'local') requestOpponentMove(state);
    }

    /* The clocks live in the player plates; chess-table.js renders them from the state. */
    function initClocks(state) {
        stopClock(state);
        const base = state.clockBase;
        state.whiteTime = base > 0 ? base : 0;
        state.blackTime = state.whiteTime;
        if (base > 0) startClock(state);
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
        if (!state.game || !(state.mode === 'local' || isPlayersTurn(state))) return;
        const turn = state.game.turn();
        if (turn === 'w') state.whiteTime = Math.max(0, state.whiteTime - 0.25);
        else state.blackTime = Math.max(0, state.blackTime - 0.25);
        const sec = Math.ceil(turn === 'w' ? state.whiteTime : state.blackTime);
        if (sec > 0 && sec <= 10 && sec !== state.lastTickSecond) {
            state.lastTickSecond = sec;
            state.audio?.lowTime?.();
        }
        window.ChessTable?.update(state);
        if (state.whiteTime <= 0 || state.blackTime <= 0) {
            endGame(state, 'time', state.whiteTime <= 0 ? 'w' : 'b');
        }
    }

    function boardInputHandler(state, event) {
        if (!state.game || state.busy || isGameOver(state) || state.reviewActive) return false;
        const type = event.type;
        if (type === state.vendor.INPUT_EVENT_TYPE.moveInputStarted) return canStartMove(state, event.squareFrom);
        if (type === state.vendor.INPUT_EVENT_TYPE.movingOverSquare) return true;
        if (type === state.vendor.INPUT_EVENT_TYPE.validateMoveInput) return validatePlayerMove(state, event.squareFrom, event.squareTo);
        if (type === state.vendor.INPUT_EVENT_TYPE.moveInputFinished) {
            const move = state.pendingMove;
            state.pendingMove = null;
            clearMoveMarkers(state);
            if (move) finishAppliedMove(state, move, '');
            return true;
        }
        if (type === state.vendor.INPUT_EVENT_TYPE.moveInputCanceled) clearMoveMarkers(state);
        return true;
    }

    function canStartMove(state, square) {
        if (!canPlayerMove(state)) return false;
        const piece = state.game.get(square);
        if (!piece || (state.mode !== 'local' && piece.color !== playerSide(state))) return false;
        clearMoveMarkers(state);
        if (state.hintMove) drawHintMarkers(state);
        const moves = state.game.moves({ square, verbose: true });
        if (moves.length > 0) {
            state.audio?.select?.();
            state.selectedSquare = square;
            updateSelectMarker(state);
        }
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
        if (!moves.length) {
            if (from !== to) state.audio?.illegal?.();
            return false;
        }
        const promotions = moves.filter(move => move.promotion);
        if (promotions.length) {
            requestPromotionChoice(state, state.game.turn()).then(promotion => {
                if (state.disposed) return;
                if (!promotion || !canPlayerMove(state)) {
                    clearMoveMarkers(state);
                    syncBoard(state, false);
                    return;
                }
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

    function requestPlayerMove(state, from, to) {
        if (!validatePlayerMove(state, from, to)) return false;
        const move = state.pendingMove;
        state.pendingMove = null;
        clearMoveMarkers(state);
        if (move) finishAppliedMove(state, move, '');
        return true;
    }

    function currentBoardOrientation(state) {
        return state.flipped ? oppositeColor(state.playerColor) : state.playerColor;
    }

    function finishAppliedMove(state, move, comment) {
        if (move.captured) recordCapture(state, move.color, move.captured);
        state.agentComment = comment || '';
        state.lastMove = { from: move.from, to: move.to };
        state.hintMove = null;
        state.reviewIndex = -1;
        state.reviewActive = false;
        state.reviewFen = '';
        state.reviewMove = null;
        clearReview(state);
        window.ChessChrome?.playMoveSound?.(state, move, {
            orientation: currentBoardOrientation(state),
            inCheck: callBool(state.game, 'inCheck', 'in_check')
        });
        state.fx?.playMoveFx(moveFxKind(state, move), move.to, currentBoardOrientation(state));
        syncBoard(state, true);
        updateAll(state);
        updateLastMoveMarkers(state);
        updateCheckMarkers(state);
        if (isGameOver(state)) { handleGameOver(state); return; }
        state.commentary?.onMove(state, move);
        if (state.mode !== 'local' && !isPlayersTurn(state)) requestOpponentMove(state);
    }

    function moveFxKind(state, move) {
        const flags = move.flags || '';
        const inCheck = callBool(state.game, 'inCheck', 'in_check');
        if (move.captured && inCheck) return 'check-capture';
        if (move.captured) return 'capture';
        if (inCheck) return 'check';
        if (move.promotion) return 'promote';
        if (flags.indexOf('k') !== -1 || flags.indexOf('q') !== -1) return 'castle';
        return 'move';
    }

    function recordCapture(state, moverColor, capturedType) {
        const capturedColor = moverColor === 'w' ? 'b' : 'w';
        state.captured[capturedColor].push(capturedType);
        state.materialScore += moverColor === 'w' ? (PIECE_VALUES[capturedType] || 0) : -(PIECE_VALUES[capturedType] || 0);
    }

    async function requestOpponentMove(state) {
        const token = ++state.searchToken;
        state.busy = true;
        state.fx?.setThinking(true);
        clearMoveMarkers(state);
        updateInput(state);
        setStatus(state, state.ctx.t(state.opponent === 'agent' ? 'desktop.chess_agent_thinking' : 'desktop.chess_engine_thinking', state.opponent === 'agent' ? 'Agent is thinking...' : 'Computer is thinking...'));
        try {
            const legalMoves = legalMovesUCI(state);
            let result;
            if (state.opponent === 'agent') {
                if (!state.agent) throw chessErr('agent_unavailable', 'Agent client is not available.');
                result = await state.agent.chooseMove({
                    fen: state.game.fen(),
                    pgn: state.game.pgn(),
                    legal_moves: legalMoves,
                    side_to_move: state.game.turn(),
                    move_number: fullMoveNumber(state),
                    player_color: state.playerColor
                });
            } else {
                if (!state.engine) throw chessErr('engine_unavailable', 'Stockfish is not available.');
                result = { move: await state.engine.findMove({ fen: state.game.fen(), legalMoves, strength: state.strength }) };
            }
            if (state.disposed || token !== state.searchToken) return;
            applyOpponentMove(state, result.move, result.comment || '');
        } catch (err) {
            if (state.disposed || token !== state.searchToken) return;
            const message = window.ChessChrome?.formatOpponentError?.(state, err) || state.ctx.t('desktop.chess_move_failed');
            notify(state, message);
            setStatus(state, message);
        } finally {
            if (!state.disposed && token === state.searchToken) {
                state.busy = false;
                state.fx?.setThinking(false);
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
        if (!move) throw chessErr('opponent_illegal', 'Opponent returned an illegal move.');
        finishAppliedMove(state, move, comment);
    }

    function undoSingleHalfMove(state) {
        const history = state.game.history({ verbose: true });
        const undone = history[history.length - 1];
        if (!undone) return;
        if (undone.captured) {
            const list = state.captured[undone.color === 'w' ? 'b' : 'w'];
            const idx = list.lastIndexOf(undone.captured);
            if (idx !== -1) list.splice(idx, 1);
            state.materialScore -= undone.color === 'w' ? (PIECE_VALUES[undone.captured] || 0) : -(PIECE_VALUES[undone.captured] || 0);
        }
        state.game.undo();
    }

    function undoMove(state) {
        if (!state.game || state.busy || state.game.history().length === 0 || state.reviewActive) return;
        state.searchToken++;
        state.hintToken++;
        undoSingleHalfMove(state);
        if (state.mode !== 'local' && state.game.history().length && state.game.turn() !== playerSide(state)) {
            undoSingleHalfMove(state);
        }
        const rem = state.game.history({ verbose: true });
        state.lastMove = rem.length ? { from: rem[rem.length - 1].from, to: rem[rem.length - 1].to } : null;
        state.pendingMove = null;
        state.agentComment = '';
        state.hintMove = null;
        state.audio?.move();
        state.commentary?.onAction('undo');
        clearMoveMarkers(state);
        syncBoard(state, true);
        updateAll(state);
        updateLastMoveMarkers(state);
        updateCheckMarkers(state);
    }

    function flipBoard(state) {
        state.flipped = !state.flipped;
        state.audio?.select?.();
        updateOrientation(state, true);
    }

    async function requestHint(state) {
        if (!state.game || state.busy || state.gameOver || state.reviewActive) return;
        if (state.mode !== 'local' && !isPlayersTurn(state)) return;
        const token = ++state.hintToken;
        state.refs.hintText.textContent = state.ctx.t('desktop.chess_hint_thinking');
        setStatus(state, state.ctx.t('desktop.chess_hint_thinking'));
        state.commentary?.onAction('hint');
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
            window.ChessView3D?.sync(state);
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
        if (!state.hintMove || !state.board || !state.vendor?.MARKER_TYPE) return;
        const MT = state.vendor.MARKER_TYPE;
        try {
            state.board.removeMarkers(MT.frame);
            state.board.addMarker(MT.frame, String(state.hintMove).slice(0, 2));
            state.board.addMarker(MT.frame, String(state.hintMove).slice(2, 4));
        } catch (e) {}
    }

    function confirmResign(state) {
        if (!state.game || state.gameOver) return;
        showModal(state, state.ctx.t('desktop.chess_confirm_resign'), [
            { label: state.ctx.t('desktop.chess_resign'), primary: true, action: () => endGame(state, 'resign', state.mode === 'local' ? state.game.turn() : playerSide(state)) },
            { label: state.ctx.t('desktop.cancel'), action: null }
        ]);
    }

    function confirmDraw(state) {
        if (!state.game || state.gameOver) return;
        showModal(state, state.ctx.t('desktop.chess_confirm_draw'), [
            { label: state.ctx.t('desktop.chess_draw'), primary: true, action: () => endGame(state, 'draw', null) },
            { label: state.ctx.t('desktop.cancel'), action: null }
        ]);
    }

    function endGame(state, reason, loserSide) {
        state.gameOver = true;
        state.endedReason = reason;
        stopClock(state);
        state.busy = false;
        state.searchToken++;
        state.fx?.setThinking(false);
        let message;
        let win = false;
        const draw = reason === 'draw';
        if (reason === 'draw') {
            message = state.ctx.t('desktop.chess_draw');
            state.commentary?.onGameOver('draw');
        } else if (reason === 'time') {
            message = loserSide === 'w' ? state.ctx.t('desktop.chess_black_wins') : state.ctx.t('desktop.chess_white_wins');
            win = state.mode !== 'local' && loserSide !== playerSide(state);
            state.commentary?.onGameOver('timeout');
        } else if (reason === 'resign') {
            message = state.ctx.t('desktop.chess_resigned') + ' — ' + (loserSide === 'w' ? state.ctx.t('desktop.chess_black_wins') : state.ctx.t('desktop.chess_white_wins'));
            win = state.mode !== 'local' && loserSide !== playerSide(state);
            state.commentary?.onGameOver('resign');
        } else {
            message = state.ctx.t('desktop.chess_title');
        }
        setStatus(state, message);
        window.ChessChrome?.playGameOverSound?.(state, draw ? 'draw' : win);
        updateAll(state);
        showResultOverlay(state, { title: message, detail: '', win, draw });
    }

    function handleGameOver(state) {
        state.gameOver = true;
        stopClock(state);
        state.fx?.setThinking(false);
        let message, detail = '', win = false, draw = false;
        if (callBool(state.game, 'isCheckmate', 'in_checkmate')) {
            const winner = state.game.turn() === 'w' ? 'b' : 'w';
            message = state.ctx.t('desktop.chess_checkmate');
            detail = winner === 'w' ? state.ctx.t('desktop.chess_white_wins') : state.ctx.t('desktop.chess_black_wins');
            if (state.mode !== 'local') {
                win = winner === playerSide(state);
                detail += ' — ' + (win ? state.ctx.t('desktop.chess_game_over_win') : state.ctx.t('desktop.chess_game_over_loss'));
                state.commentary?.onGameOver(win ? 'checkmate_win' : 'checkmate_loss');
            } else {
                state.commentary?.onGameOver('checkmate_local');
            }
        } else if (callBool(state.game, 'isStalemate', 'in_stalemate')) {
            message = state.ctx.t('desktop.chess_stalemate');
            draw = true;
            state.commentary?.onGameOver('stalemate');
        } else if (callBool(state.game, 'isDraw', 'in_draw')) {
            message = state.ctx.t('desktop.chess_draw');
            draw = true;
            state.commentary?.onGameOver('draw');
        }
        state.endedReason = 'gameover';
        setStatus(state, message + (detail ? ' — ' + detail : ''));
        window.ChessChrome?.playGameOverSound?.(state, draw ? 'draw' : win);
        updateAll(state);
        showResultOverlay(state, { title: message, detail, win, draw });
    }

    function showResultOverlay(state, opts) {
        if (!state.fx || typeof state.fx.showResult !== 'function') return;
        state.fx.showResult({
            esc: state.ctx.esc,
            title: opts.title || '',
            detail: opts.detail || '',
            win: !!opts.win,
            draw: !!opts.draw,
            primaryLabel: state.ctx.t('desktop.chess_new_game'),
            secondaryLabel: state.ctx.t('desktop.ok'),
            onPrimary: () => startNewGame(state),
            onSecondary: null
        });
    }

    function tryMove(state, move) {
        try { return state.game.move(move); } catch (err) { return null; }
    }

    function syncBoard(state, animated) {
        if (!state.board || !state.game) return;
        state.board.setPosition(state.game.fen(), !!animated).catch(() => {});
        updateOrientation(state, false);
    }

    function updateOrientation(state, animated) {
        if (!state.board || !state.vendor) return;
        const black = currentBoardOrientation(state) === 'black';
        state.board.setOrientation(black ? state.vendor.COLOR.black : state.vendor.COLOR.white, !!animated).catch(() => {});
        state.refs.root.classList.toggle('is-flipped', black);
        window.ChessView3D?.sync(state);
        window.ChessTable?.update(state);
    }

    function updateLastMoveMarkers(state) {
        if (!state.board || !state.vendor?.MARKER_TYPE) return;
        const MT = state.vendor.MARKER_TYPE;
        const target = state.reviewActive ? state.reviewMove : state.lastMove;
        try {
            state.board.removeMarkers(MT.square);
            if (target) {
                state.board.addMarker(MT.square, target.from);
                state.board.addMarker(MT.square, target.to);
            }
        } catch (e) {}
    }

    function updateSelectMarker(state) {
        if (!state.board || !state.vendor?.MARKER_TYPE) return;
        const MT = state.vendor.MARKER_TYPE;
        try {
            state.board.removeMarkers(MT.circlePrimary);
            if (state.selectedSquare) state.board.addMarker(MT.circlePrimary, state.selectedSquare);
        } catch (e) {}
    }

    function updateCheckMarkers(state) {
        if (!state.board || !state.vendor?.MARKER_TYPE) return;
        const MT = state.vendor.MARKER_TYPE;
        try {
            state.board.removeMarkers(MT.circleDangerFilled);
            if (!state.reviewActive && callBool(state.game, 'inCheck', 'in_check')) {
                const king = findKing(state.game, state.game.turn());
                if (king) state.board.addMarker(MT.circleDangerFilled, king);
            }
        } catch (e) {}
    }

    function findKing(game, color) {
        const rows = game.fen().split(' ')[0].split('/');
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
        updateReplayControls(state);
    }

    function updateInput(state) {
        if (!state.board || !state.vendor) return;
        const idle = !state.busy && !state.gameOver && !state.reviewActive;
        state.refs.undo.disabled = !idle || !state.game || !state.game.history().length;
        state.refs.hint.disabled = !idle || !canPlayerMove(state);
        state.refs.resign.disabled = !idle || !state.game;
        state.refs.draw.disabled = !idle || !state.game;
        if (!idle || !canPlayerMove(state)) { state.board.disableMoveInput(); return; }
        state.board.disableMoveInput();
        const color = playerSide(state) === 'b' ? state.vendor.COLOR.black : state.vendor.COLOR.white;
        state.board.enableMoveInput(event => boardInputHandler(state, event), state.mode === 'local' ? null : color);
    }

    function updateStatus(state) {
        if (!state.game) return;
        let message, tone = '';
        if (state.gameOver) {
            message = state.refs.status.textContent || state.ctx.t('desktop.chess_title');
            tone = 'is-game-over';
        } else if (callBool(state.game, 'isCheckmate', 'in_checkmate')) {
            const winner = state.game.turn() === 'w' ? 'b' : 'w';
            message = state.ctx.t('desktop.chess_checkmate') + ' — ' + (winner === 'w' ? state.ctx.t('desktop.chess_white_wins') : state.ctx.t('desktop.chess_black_wins'));
            tone = 'is-game-over';
        } else if (callBool(state.game, 'isStalemate', 'in_stalemate')) {
            message = state.ctx.t('desktop.chess_stalemate');
            tone = 'is-game-over';
        } else if (callBool(state.game, 'isDraw', 'in_draw')) {
            message = state.ctx.t('desktop.chess_draw');
            tone = 'is-game-over';
        } else if (callBool(state.game, 'inCheck', 'in_check')) {
            message = state.ctx.t('desktop.chess_check');
            tone = 'is-check';
        } else if (state.busy) {
            message = state.ctx.t('desktop.chess_thinking');
            tone = 'is-thinking';
        } else if (state.reviewActive) {
            message = state.ctx.t('desktop.chess_reviewing').replace('{n}', String(state.reviewIndex + 1));
        } else if (state.mode === 'local') {
            message = state.game.turn() === 'w' ? state.ctx.t('desktop.chess_local_turn_white') : state.ctx.t('desktop.chess_local_turn_black');
            tone = 'is-your-turn';
        } else {
            const yours = isPlayersTurn(state);
            message = yours ? state.ctx.t('desktop.chess_your_turn') : state.ctx.t('desktop.chess_opponent_turn');
            tone = yours ? 'is-your-turn' : '';
        }
        setStatus(state, message);
        state.fx?.setStatusTone(tone);
        const side = state.game.turn() === 'w' ? state.ctx.t('desktop.chess_white_to_move') : state.ctx.t('desktop.chess_black_to_move');
        const liveRemark = (state.commentary?.isEnabled() && state.commentaryText) ? state.commentaryText : state.agentComment;
        if (state.refs.comment) {
            state.refs.comment.textContent = liveRemark || (state.mode !== 'local' && state.game.turn() !== playerSide(state) ? side : '');
            state.refs.comment.title = liveRemark || '';
            state.refs.comment.classList.toggle('has-quips', !!liveRemark);
        }
    }

    function renderMoves(state) {
        window.ChessMoveList.render(state, idx => reviewJump(state, idx));
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

    /* Captured material, the material balance and the player plates are painted by chess-table.js. */
    function updateCaptured(state) {
        window.ChessTable?.update(state);
    }

    function updateInfo(state) {
        const diff = state.materialScore;
        const evalText = Math.abs(diff) < 1 ? state.ctx.t('desktop.chess_even')
            : (diff > 0 ? state.ctx.t('desktop.chess_advantage_white') : state.ctx.t('desktop.chess_advantage_black')).replace('{n}', String(Math.abs(diff)));
        state.refs.evalEl.textContent = evalText;
        if (state.refs.phaseEl && window.ChessInfoTools) {
            state.refs.phaseEl.textContent = state.ctx.t(window.ChessInfoTools.detectGamePhaseKey(state.game, PIECE_VALUES));
        }
        state.refs.hintText.textContent = state.hintMove ? (state.ctx.t('desktop.chess_hint') + ': ' + String(state.hintMove).toUpperCase()) : state.ctx.t('desktop.chess_no_moves');
        window.ChessMaterialBar?.update(state.refs, diff);
    }

    function selectTab(state, tab) {
        state.activeTab = tab;
        state.refs.tabs.querySelectorAll('[data-tab]').forEach(btn => {
            const active = btn.dataset.tab === tab;
            btn.classList.toggle('active', active);
            btn.setAttribute('aria-selected', active ? 'true' : 'false');
            btn.tabIndex = active ? 0 : -1;
        });
        Object.keys(state.refs.panels).forEach(key => { state.refs.panels[key].hidden = key !== tab; });
    }

    function reviewStep(state, dir) {
        if (!state.game) return;
        const total = state.game.history().length;
        const next = state.reviewActive ? state.reviewIndex + dir : (total - 1 + (dir < 0 ? dir : 0));
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
        state.reviewFen = tempGame.fen();
        state.reviewMove = history[idx] ? { from: history[idx].from, to: history[idx].to } : null;
        /* Review snaps: the vendor queues animated position changes and does not
           cancel them, so a backlog of review glides would delay the animation of
           the next real move and keep the move input locked while it waits. */
        state.board?.setPosition(state.reviewFen, false).catch(() => {});
        window.ChessView3D?.sync(state);
        state.audio?.select?.();
        updateLastMoveMarkers(state);
        updateCheckMarkers(state);
        updateAll(state);
        const cell = state.refs.moves.querySelector('[data-jump="' + idx + '"]');
        if (cell && cell.scrollIntoView) cell.scrollIntoView({ block: 'nearest' });
    }

    function clearReview(state) {
        state.reviewActive = false;
        state.reviewIndex = -1;
        state.reviewFen = '';
        state.reviewMove = null;
        if (state.game && state.board) state.board.setPosition(state.game.fen(), false).catch(() => {});
        window.ChessView3D?.sync(state);
        updateLastMoveMarkers(state);
        updateCheckMarkers(state);
        if (state.refs.reviewLabel) state.refs.reviewLabel.textContent = state.ctx.t('desktop.chess_live');
    }

    function updateReplayControls(state) {
        const total = state.game ? state.game.history().length : 0;
        state.refs.replay.hidden = total < 2;
        state.refs.reviewLabel.textContent = state.reviewActive
            ? state.ctx.t('desktop.chess_reviewing').replace('{n}', String(state.reviewIndex + 1))
            : state.ctx.t('desktop.chess_live');
    }

    function setWindowMenus(state) {
        if (typeof state.ctx.setWindowMenus !== 'function') return;
        const commentaryOn = !!(state.commentary && state.commentary.isEnabled());
        const soundOn = !state.soundMuted;
        state.ctx.setWindowMenus(state.windowId, [
            {
                id: 'game',
                labelKey: 'desktop.menu_game',
                items: [
                    { id: 'new', labelKey: 'desktop.chess_new_game', icon: 'refresh', shortcut: 'Ctrl+N', action: () => startNewGame(state) },
                    { id: 'undo', labelKey: 'desktop.chess_undo', icon: 'undo', disabled: !state.game || !state.game.history().length || state.busy, shortcut: 'Ctrl+Z', action: () => undoMove(state) },
                    { id: 'hint', labelKey: 'desktop.chess_hint', icon: 'refresh', disabled: !state.game || state.busy || state.gameOver, action: () => requestHint(state) },
                    { id: 'copy-pgn', labelKey: 'desktop.chess_copy_pgn', icon: 'copy', action: () => window.ChessInfoTools?.copyChessNotation(state.game, 'pgn', () => notify(state, state.ctx.t('desktop.chess_copied'))) }
                ]
            },
            {
                id: 'view',
                labelKey: 'desktop.menu_view',
                items: [
                    { id: 'flip', labelKey: 'desktop.chess_flip', icon: 'refresh', action: () => flipBoard(state) },
                    { id: 'sound', labelKey: 'desktop.chess_sound', icon: soundOn ? 'check' : 'square', checked: soundOn, action: () => toggleSound(state) },
                    { id: 'commentary', labelKey: 'desktop.chess_commentary', icon: commentaryOn ? 'check' : 'square', checked: commentaryOn, action: () => toggleCommentary(state) }
                ]
            }
        ]);
    }

    function requestPromotionChoice(state, color) { return window.ChessPromotion.requestPromotionChoice(state, color); }
    function showModal(state, message, buttons) { window.showChessModal?.(state.refs.root, message, buttons, state.ctx.esc); }

    function clearMoveMarkers(state) {
        state.board?.removeLegalMovesMarkers?.();
        if (state.selectedSquare) {
            state.selectedSquare = null;
            updateSelectMarker(state);
        }
        if (state.hintMove && state.board && state.vendor?.MARKER_TYPE) {
            try { state.board.removeMarkers(state.vendor.MARKER_TYPE.frame); } catch (e) {}
        }
    }

    function legalMovesUCI(state) { return state.game.moves({ verbose: true }).map(move => move.from + move.to + (move.promotion || '')); }
    function isPlayersTurn(state) { return state.game && state.game.turn() === playerSide(state); }
    function playerSide(state) { return state.playerColor === 'black' ? 'b' : 'w'; }
    function oppositeColor(color) { return color === 'black' ? 'white' : 'black'; }
    function fullMoveNumber(state) { return Number(state.game.fen().split(/\s+/)[5]) || 1; }
    function isGameOver(state) { return state.gameOver || (state.game && callBool(state.game, 'isGameOver', 'game_over')); }
    function callBool(target, modern, legacy) {
        if (target && typeof target[modern] === 'function') return !!target[modern]();
        if (target && typeof target[legacy] === 'function') return !!target[legacy]();
        return false;
    }
    function setStatus(state, message) { if (state.refs.status) state.refs.status.textContent = message; }
    function notify(state, message) { state.ctx.notify?.({ title: state.ctx.t('desktop.chess_title'), message }); }

    function chessErr(code, fallback) {
        const err = new Error(fallback);
        err.chessCode = code;
        return err;
    }

    async function defaultAPI(url, options) {
        const resp = await fetch(url, Object.assign({ credentials: 'same-origin', cache: 'no-store' }, options || {}));
        const data = await resp.json().catch(() => ({}));
        if (!resp.ok) throw new Error(data.error || data.message || ('HTTP ' + resp.status));
        return data;
    }

    window.ChessApp = { render, dispose };
})();
