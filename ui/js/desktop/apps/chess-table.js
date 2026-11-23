/* AuraGo chess - the table around the board.
 *
 * Owns the two player plates (name, captured material, clock), the thinking
 * indicator and the engraved coordinates of the board rim. Like chess-fx.js it
 * is a companion of chess.js: it reads the app state it is handed and never
 * mutates the game.
 */
(function () {
  'use strict';

  const FILES = 'abcdefgh';
  const GLYPH = { q: '♛', r: '♜', b: '♝', n: '♞', p: '♟', k: '♚' };
  const VALUE = { p: 1, n: 3, b: 3, r: 5, q: 9, k: 0 };
  const ORDER = ['q', 'r', 'b', 'n', 'p'];
  const LOW_SECONDS = 30;

  function mount(state) {
    const root = state.refs && state.refs.root;
    if (!root) return;
    const q = (scope, selector) => scope ? scope.querySelector(selector) : null;
    const plate = selector => {
      const el = q(root, selector);
      return {
        el,
        name: q(el, '[data-plate-name]'),
        avatar: q(el, '[data-plate-avatar]'),
        tray: q(el, '[data-plate-tray]'),
        lead: q(el, '[data-plate-lead]'),
        clock: q(el, '[data-plate-clock]')
      };
    };
    state.table = {
      top: plate('[data-plate="top"]'),
      bottom: plate('[data-plate="bottom"]'),
      coords: q(root, '[data-rim-coords]'),
      capturedWhite: q(root, '[data-captured-white]'),
      capturedBlack: q(root, '[data-captured-black]'),
      material: q(root, '[data-material]'),
      orientation: null
    };
  }

  function update(state) {
    const ui = state.table;
    if (!ui || state.disposed || !state.game) return;
    const blackAtBottom = isBlackAtBottom(state);
    renderPlate(ui.top, state, blackAtBottom ? 'w' : 'b');
    renderPlate(ui.bottom, state, blackAtBottom ? 'b' : 'w');
    renderCapturedTab(ui, state);
    renderCoordinates(ui, blackAtBottom);
  }

  function isBlackAtBottom(state) {
    const player = state.playerColor === 'black' ? 'b' : 'w';
    return (state.flipped ? (player === 'w' ? 'b' : 'w') : player) === 'b';
  }

  function nameFor(state, side) {
    const t = state.ctx.t;
    if (state.mode === 'local') return t(side === 'w' ? 'desktop.chess_color_white' : 'desktop.chess_color_black');
    if (side === (state.playerColor === 'black' ? 'b' : 'w')) return t('desktop.chess_you');
    if (state.opponent === 'agent') return t('desktop.chess_opponent_agent');
    return t('desktop.chess_opponent_computer') + ' · ' + t('desktop.chess_strength') + ' ' + state.strength;
  }

  function formatTime(seconds) {
    const total = Math.max(0, Math.ceil(seconds));
    return String(Math.floor(total / 60)).padStart(2, '0') + ':' + String(total % 60).padStart(2, '0');
  }

  function setText(el, text) {
    if (el && el.textContent !== text) el.textContent = text;
  }

  const valueOf = list => list.reduce((sum, piece) => sum + (VALUE[piece] || 0), 0);

  /* state.captured.w lists the white pieces that were taken, so the tray of a
     side is the list of the other colour. */
  function renderPlate(plate, state, side) {
    if (!plate.el) return;
    const takenKey = side === 'w' ? 'b' : 'w';
    const captured = state.captured[takenKey];
    const lead = side === 'w' ? valueOf(state.captured.b) - valueOf(state.captured.w) : valueOf(state.captured.w) - valueOf(state.captured.b);
    const latest = captured.length > state.prevCapturedCounts[takenKey] ? captured[captured.length - 1] : null;
    const turn = state.game.turn();
    const active = !state.gameOver && turn === side;

    const name = nameFor(state, side);
    setText(plate.name, name);
    plate.el.dataset.side = side;
    plate.el.dataset.clock = side === 'w' ? 'white' : 'black';
    plate.el.setAttribute('aria-label', name);
    plate.el.classList.toggle('is-active', active);
    plate.el.classList.toggle('is-thinking', !!(state.busy && active));
    setText(plate.avatar, GLYPH.k);

    const sorted = captured.slice().sort((a, b) => ORDER.indexOf(a) - ORDER.indexOf(b));
    const trayKey = sorted.join('') + (latest ? '|' + latest : '');
    if (plate.tray && plate.tray.dataset.key !== trayKey) {
      plate.tray.dataset.key = trayKey;
      plate.tray.replaceChildren(...sorted.map((piece, index) => {
        const span = document.createElement('span');
        span.className = 'vd-chess-tray-piece' + (piece === latest && sorted.lastIndexOf(piece) === index ? ' is-new' : '');
        span.textContent = GLYPH[piece];
        return span;
      }));
    }

    const leadText = lead > 0 ? '+' + lead : '';
    setText(plate.lead, leadText);
    if (plate.lead) plate.lead.hidden = lead <= 0;

    const timed = state.clockBase > 0;
    if (plate.clock) {
      plate.clock.hidden = !timed;
      const remaining = side === 'w' ? state.whiteTime : state.blackTime;
      setText(plate.clock, timed ? formatTime(remaining) : '');
      plate.clock.classList.toggle('is-active', active);
      plate.clock.classList.toggle('is-low', active && remaining < LOW_SECONDS);
    }
  }

  function renderCapturedTab(ui, state) {
    const render = (host, list, prevCount) => {
      if (!host) return;
      if (!list.length) {
        host.textContent = '—';
        host.classList.add('is-empty');
        return;
      }
      host.classList.remove('is-empty');
      const sorted = list.slice().sort((a, b) => ORDER.indexOf(a) - ORDER.indexOf(b));
      const latest = list.length > prevCount ? list[list.length - 1] : null;
      host.replaceChildren(...sorted.map((piece, index) => {
        const span = document.createElement('span');
        span.className = 'vd-chess-piece vd-chess-piece-' + piece
          + (piece === latest && sorted.lastIndexOf(piece) === index ? ' is-new' : '');
        span.textContent = GLYPH[piece];
        return span;
      }));
    };
    render(ui.capturedWhite, state.captured.b, state.prevCapturedCounts.b);
    render(ui.capturedBlack, state.captured.w, state.prevCapturedCounts.w);
    state.prevCapturedCounts.b = state.captured.b.length;
    state.prevCapturedCounts.w = state.captured.w.length;
    const diff = valueOf(state.captured.b) - valueOf(state.captured.w);
    setText(ui.material, (diff > 0 ? '+' : '') + diff);
    window.ChessMaterialBar?.update(state.refs, diff);
  }

  /* The rim is a 10x10 grid: one rim cell around eight board cells. The far
     coordinates stay upright, so both players read the same order. */
  function renderCoordinates(ui, blackAtBottom) {
    const key = blackAtBottom ? 'b' : 'w';
    if (!ui.coords || ui.orientation === key) return;
    ui.orientation = key;
    ui.coords.replaceChildren();
    for (let i = 0; i < 8; i += 1) {
      const file = (blackAtBottom ? FILES[7 - i] : FILES[i]).toUpperCase();
      const rank = String(blackAtBottom ? i + 1 : 8 - i);
      [[file, 1, i + 2], [file, 10, i + 2], [rank, i + 2, 1], [rank, i + 2, 10]].forEach(([text, row, col]) => {
        const span = document.createElement('span');
        span.textContent = text;
        span.style.gridArea = row + ' / ' + col;
        ui.coords.appendChild(span);
      });
    }
  }

  window.ChessTable = { mount, update };
})();
