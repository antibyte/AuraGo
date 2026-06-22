(function () {
    'use strict';

    const instances = new Map();
    let vendorPromise = null;

    function loadVendor() {
        if (!vendorPromise) vendorPromise = import('/js/vendor/chess-vendor.esm.js');
        return vendorPromise;
    }

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
            resizeObserver: null,
            resizeHandler: null,
            playerColor: 'white',
            opponent: 'computer',
            strength: 8,
            flipped: false,
            busy: false,
            pendingMove: null,
            searchToken: 0,
            agentComment: '',
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
                    <div class="vd-chess-controls">
                        <div class="vd-chess-field">
                            <label for="chess-opponent">${esc(t('desktop.chess_opponent'))}</label>
                            <select id="chess-opponent" data-opponent>
                                <option value="computer">${esc(t('desktop.chess_opponent_computer'))}</option>
                                <option value="agent">${esc(t('desktop.chess_opponent_agent'))}</option>
                            </select>
                        </div>
                        <div class="vd-chess-field">
                            <span class="vd-chess-label">${esc(t('desktop.chess_player_color'))}</span>
                            <div class="vd-chess-segment" data-color-group>
                                <button type="button" data-color="white" class="active">${esc(t('desktop.chess_color_white'))}</button>
                                <button type="button" data-color="black">${esc(t('desktop.chess_color_black'))}</button>
                            </div>
                        </div>
                        <div class="vd-chess-field" data-strength-wrap>
                            <label for="chess-strength">${esc(t('desktop.chess_strength'))} <span data-strength-value>8</span></label>
                            <input id="chess-strength" data-strength type="range" min="1" max="20" value="8">
                        </div>
                        <div class="vd-chess-actions">
                            <button type="button" class="vd-chess-primary" data-new-game>${ctx.iconMarkup('refresh', 'N', 'vd-chess-action-icon', 15)}<span>${esc(t('desktop.chess_new_game'))}</span></button>
                            <button type="button" data-undo>${ctx.iconMarkup('undo', 'U', 'vd-chess-action-icon', 15)}<span>${esc(t('desktop.chess_undo'))}</span></button>
                            <button type="button" data-flip>${ctx.iconMarkup('rotate', 'F', 'vd-chess-action-icon', 15)}<span>${esc(t('desktop.chess_flip'))}</span></button>
                        </div>
                    </div>
                    <div class="vd-chess-moves">
                        <div class="vd-chess-panel-title">${esc(t('desktop.chess_moves'))}</div>
                        <ol data-moves></ol>
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
            opponent: root.querySelector('[data-opponent]'),
            strength: root.querySelector('[data-strength]'),
            strengthValue: root.querySelector('[data-strength-value]'),
            strengthWrap: root.querySelector('[data-strength-wrap]'),
            moves: root.querySelector('[data-moves]'),
            colorGroup: root.querySelector('[data-color-group]'),
            newGame: root.querySelector('[data-new-game]'),
            undo: root.querySelector('[data-undo]'),
            flip: root.querySelector('[data-flip]')
        };
    }

    function bindControls(state) {
        const refs = state.refs;
        refs.opponent.addEventListener('change', () => {
            state.opponent = refs.opponent.value === 'agent' ? 'agent' : 'computer';
            updateStrengthState(state);
            startNewGame(state);
        });
        refs.strength.addEventListener('input', () => {
            state.strength = Number(refs.strength.value) || 8;
            refs.strengthValue.textContent = String(state.strength);
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
                animationDuration: 160
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
        state.game = new state.vendor.Chess();
        state.pendingMove = null;
        state.agentComment = '';
        state.busy = false;
        if (state.engine) state.engine.newGame();
        syncBoard(state, false);
        updateAll(state);
        setWindowMenus(state);
        if (!isPlayersTurn(state)) requestOpponentMove(state);
    }

    function boardInputHandler(state, event) {
        if (!state.game || state.busy || isGameOver(state)) return false;
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
        if (!isPlayersTurn(state)) return false;
        const piece = state.game.get(square);
        if (!piece || piece.color !== playerSide(state)) return false;
        clearMoveMarkers(state);
        const moves = state.game.moves({ square, verbose: true });
        if (state.board.addLegalMovesMarkers) state.board.addLegalMovesMarkers(moves);
        return moves.length > 0;
    }

    function validatePlayerMove(state, from, to) {
        if (!isPlayersTurn(state)) return false;
        const moves = state.game.moves({ square: from, verbose: true }).filter(move => move.to === to);
        if (!moves.length) return false;
        const promotions = moves.filter(move => move.promotion);
        if (promotions.length) {
            requestPromotionChoice(state, playerSide(state)).then(promotion => {
                if (!promotion || state.disposed || !isPlayersTurn(state)) return;
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
        state.agentComment = comment || '';
        state.lastMove = { from: move.from, to: move.to };
        syncBoard(state, true);
        updateAll(state);
        if (!isGameOver(state) && !isPlayersTurn(state)) requestOpponentMove(state);
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
        if (!state.game || state.busy || state.game.history().length === 0) return;
        state.searchToken++;
        state.game.undo();
        if (state.game.history().length && state.game.turn() !== playerSide(state)) state.game.undo();
        state.pendingMove = null;
        state.agentComment = '';
        clearMoveMarkers(state);
        syncBoard(state, true);
        updateAll(state);
    }

    function flipBoard(state) {
        state.flipped = !state.flipped;
        updateOrientation(state, true);
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

    function updateAll(state) {
        renderMoves(state);
        updateStatus(state);
        updateStrengthState(state);
        updateInput(state);
        setWindowMenus(state);
    }

    function updateInput(state) {
        if (!state.board || !state.vendor) return;
        if (state.busy || !isPlayersTurn(state) || isGameOver(state)) {
            state.board.disableMoveInput();
            return;
        }
        const color = playerSide(state) === 'b' ? state.vendor.COLOR.black : state.vendor.COLOR.white;
        state.board.enableMoveInput(event => boardInputHandler(state, event), color);
    }

    function updateStatus(state) {
        if (!state.game) return;
        let message;
        if (callBool(state.game, 'isCheckmate', 'in_checkmate')) {
            message = state.ctx.t('desktop.chess_checkmate');
        } else if (callBool(state.game, 'isStalemate', 'in_stalemate')) {
            message = state.ctx.t('desktop.chess_stalemate');
        } else if (callBool(state.game, 'isDraw', 'in_draw')) {
            message = state.ctx.t('desktop.chess_draw');
        } else if (callBool(state.game, 'inCheck', 'in_check')) {
            message = state.ctx.t('desktop.chess_check');
        } else if (state.busy) {
            message = state.ctx.t('desktop.chess_thinking');
        } else {
            message = isPlayersTurn(state) ? state.ctx.t('desktop.chess_your_turn') : state.ctx.t('desktop.chess_opponent_turn');
        }
        setStatus(state, message);
        state.refs.comment.textContent = state.agentComment || turnLabel(state);
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
            rows.push(`<li><span class="vd-chess-move-no">${Math.floor(i / 2) + 1}.</span><span>${state.ctx.esc(white ? white.san : '')}</span><span>${state.ctx.esc(black ? black.san : '')}</span></li>`);
        }
        state.refs.moves.innerHTML = rows.join('');
        state.refs.moves.scrollTop = state.refs.moves.scrollHeight;
    }

    function updateStrengthState(state) {
        const disabled = state.opponent === 'agent';
        state.refs.strength.disabled = disabled;
        state.refs.strengthWrap.classList.toggle('is-disabled', disabled);
        state.refs.strengthValue.textContent = String(state.strength);
    }

    function setWindowMenus(state) {
        if (typeof state.ctx.setWindowMenus !== 'function') return;
        state.ctx.setWindowMenus(state.windowId, [
            {
                id: 'game',
                labelKey: 'desktop.menu_game',
                items: [
                    { id: 'new', labelKey: 'desktop.chess_new_game', icon: 'refresh', shortcut: 'Ctrl+N', action: () => startNewGame(state) },
                    { id: 'undo', labelKey: 'desktop.chess_undo', icon: 'undo', disabled: !state.game || !state.game.history().length || state.busy, shortcut: 'Ctrl+Z', action: () => undoMove(state) }
                ]
            },
            {
                id: 'view',
                labelKey: 'desktop.menu_view',
                items: [
                    { id: 'flip', labelKey: 'desktop.chess_flip', icon: 'rotate', action: () => flipBoard(state) }
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
            dialog.innerHTML = choices.map(piece => `<button type="button" data-piece="${piece}">${state.ctx.esc(names[piece])}</button>`).join('');
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

    function clearMoveMarkers(state) {
        if (state.board && state.board.removeLegalMovesMarkers) state.board.removeLegalMovesMarkers();
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

    function turnLabel(state) {
        return state.game && state.game.turn() === 'w'
            ? state.ctx.t('desktop.chess_white_to_move')
            : state.ctx.t('desktop.chess_black_to_move');
    }

    function isGameOver(state) {
        return state.game && callBool(state.game, 'isGameOver', 'game_over');
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

    async function defaultAPI(url, options) {
        const resp = await fetch(url, Object.assign({ credentials: 'same-origin', cache: 'no-store' }, options || {}));
        const data = await resp.json().catch(() => ({}));
        if (!resp.ok) throw new Error(data.error || data.message || ('HTTP ' + resp.status));
        return data;
    }

    window.ChessApp = { render, dispose };
})();
