/* Runenpfad games window: the rune dice table and the wardens' task board, one overlay in the style of the dialogue
 * window (same panel, same buttons and focus look). Builds DOM through the `el` helper (createElement/textContent only).
 * The game is paused while it is open; app.js owns the shell, the pause state and the audio sink (`emit`).
 * Options: { el, t, fill, hud, flush(), emit(events), cue(id), onChange(), isBlocked() }.
 * `request(req, { sim, music })` handles the uiRequest values `board`, `dice:<npcId>` and `song:<theme>`. */
(function (root) {
    'use strict';

    const GUARD_MS = 350;       // same constant as the dialogue window: the key that opened it must not act
    const SONG_MS = 18000;
    const FACES = '⚀⚁⚂⚃⚄⚅';
    const PIPS = {
        1: [[1, 1]], 2: [[0, 0], [2, 2]], 3: [[0, 0], [1, 1], [2, 2]], 4: [[0, 0], [2, 0], [0, 2], [2, 2]],
        5: [[0, 0], [2, 0], [1, 1], [0, 2], [2, 2]], 6: [[0, 0], [2, 0], [0, 1], [2, 1], [0, 2], [2, 2]]
    };
    const DIE = 58;
    const CANVAS_W = 340;
    const CANVAS_H = 156;
    const ROW_Y = { them: 10, you: 88 };
    const DIE_X = [CANVAS_W / 2 - DIE - 12, CANVAS_W / 2 + 12];
    let current = null;

    function create(o) {
        const el = o.el;
        const t = o.t;
        const fill = o.fill;
        const hud = o.hud;
        const emit = typeof o.emit === 'function' ? o.emit : function () {};
        const isBlocked = typeof o.isBlocked === 'function' ? o.isBlocked : function () { return false; };
        const perf = root.performance;

        let open = false;
        let disposed = false;
        let mode = '';
        let sim = null;
        let game = null;     // dice state
        let stake = 0;       // chosen stake in the dice stake phase
        let openedAt = 0;
        let focusIdx = 0;
        let songTimer = 0;
        let songMusic = null;

        const rootEl = el('div', 'rp-talk rp-games');
        rootEl.hidden = true;
        rootEl.setAttribute('role', 'dialog');
        rootEl.setAttribute('aria-modal', 'false');
        const panel = el('div', 'rp-talk-panel rp-games-panel');
        const body = el('div', 'rp-talk-main rp-games-body');
        panel.append(body);
        rootEl.append(panel);

        function dice() { return root.RunenpfadDice || null; }
        function board() { return root.RunenpfadBounties || null; }
        function coinsNow() { return Math.max(0, Math.floor((sim && sim.profile && sim.profile.coins) || 0)); }
        function evCount() { return sim && sim.events ? sim.events.length : 0; }
        // Events pushed while the game is paused would be dropped by the tick; hand the new ones to the audio layer.
        function forward(before) { if (sim && sim.events && sim.events.length > before) emit(sim.events.slice(before)); }

        function opponentName(id) {
            const N = root.RunenpfadNpcs;
            const def = N && N.NPCS && N.NPCS[id];
            return def ? t(def.nameKey) : id;
        }

        // ---- focus ----
        function focusables() { return Array.prototype.slice.call(body.querySelectorAll('button:not(:disabled)')); }

        function setFocus(i) {
            const list = focusables();
            if (!list.length) return;
            focusIdx = (i + list.length) % list.length;
            list.forEach(function (node, k) { node.classList.toggle('is-focus', k === focusIdx); });
            if (mode === 'dice') paintDice();
        }

        function btn(parent, className, label, key, fn) {
            const b = el('button', 'rp-btn rp-talk-choice ' + className);
            b.type = 'button';
            hud.setButton(b, label, key);
            b.addEventListener('click', function () { fn(); });
            b.addEventListener('pointerenter', function () { const k = focusables().indexOf(b); if (k >= 0) setFocus(k); });
            b.addEventListener('focus', function () { const k = focusables().indexOf(b); if (k >= 0 && k !== focusIdx) setFocus(k); }); // the gamepad walks real focus
            parent.append(b);
            return b;
        }

        function head(title) {
            const row = el('div', 'rp-trade-head');
            const coinBox = el('span', 'rp-chip rp-trade-coins');
            coinBox.append(hud.icon('coin', 16), el('span', 'rp-num', String(coinsNow())));
            coinBox.title = t('runenpfad.hud_coins');
            row.append(el('h3', 'rp-trade-title', title), coinBox);
            return row;
        }

        function rewardChip(n) {
            const chip = el('span', 'rp-chip rp-games-reward');
            chip.append(hud.icon('coin', 14), el('span', 'rp-num', String(n)));
            return chip;
        }

        // ---- dice ----
        function drawDie(cx, x, y, value, lit) {
            cx.fillStyle = value ? '#f3e6c4' : '#3a2e20';
            cx.strokeStyle = lit ? '#ffd27a' : '#8a6a3a';
            cx.lineWidth = lit ? 4 : 2;
            cx.beginPath();
            if (typeof cx.roundRect === 'function') cx.roundRect(x, y, DIE, DIE, 10); else cx.rect(x, y, DIE, DIE);
            cx.fill();
            cx.stroke();
            cx.fillStyle = value ? '#1d150d' : '#b89a62';
            if (!value) {
                cx.font = '700 30px sans-serif';
                cx.textAlign = 'center';
                cx.textBaseline = 'middle';
                cx.fillText('?', x + DIE / 2, y + DIE / 2 + 2);
                return;
            }
            PIPS[value].forEach(function (p) {
                cx.beginPath();
                cx.arc(x + 12 + p[0] * 17, y + 12 + p[1] * 17, 5.2, 0, Math.PI * 2);
                cx.fill();
            });
        }

        let canvas = null;
        function paintDice() {
            if (!canvas || !game) return;
            const cx = canvas.getContext ? canvas.getContext('2d') : null;
            if (!cx) return;
            cx.clearRect(0, 0, CANVAS_W, CANVAS_H);
            const shown = game.phase === 'done';
            const picking = game.phase === 'rolled' && focusIdx < 2;
            for (let i = 0; i < 2; i++) {
                drawDie(cx, DIE_X[i], ROW_Y.them, shown ? game.theirs[i] : 0, false);
                drawDie(cx, DIE_X[i], ROW_Y.you, game.phase === 'stake' ? 0 : game.mine[i], (picking && focusIdx === i) || (shown && game.rerolled === i));
            }
        }

        function diceStatus() {
            const D = dice();
            if (game.phase === 'rolled') return { text: t('runenpfad.dice.reroll'), cls: '' };
            if (game.phase === 'done') {
                const sums = opponentName(game.opponent) + ' ' + (game.theirs[0] + game.theirs[1]) + ' · ' + t('runenpfad.dice.you') + ' ' + (game.mine[0] + game.mine[1]) + ' — ';
                if (game.result === 'win') return { text: sums + fill('runenpfad.dice.win', { coins: game.stake }), cls: ' is-ok' };
                if (game.result === 'lose') return { text: sums + fill('runenpfad.dice.lose', { coins: game.stake }), cls: ' is-bad' };
                return { text: sums + t('runenpfad.dice.tie'), cls: '' };
            }
            const st = D.status(game);
            if (st === 'poor') return { text: t('runenpfad.dice.poor'), cls: ' is-bad' };
            if (st === 'broke') return { text: fill('runenpfad.dice.broke', { name: opponentName(game.opponent) }), cls: ' is-bad' };
            return { text: '', cls: '' };
        }

        function afterDice(before) {
            forward(before);
            if (sim && sim.storyDirty && typeof o.flush === 'function') o.flush();
            focusIdx = 0;
            render();
        }

        function act(fn) {
            const before = evCount();
            fn();
            afterDice(before);
        }

        function renderDice() {
            const D = dice();
            const choices = el('div', 'rp-talk-choices rp-games-actions');
            body.append(head(t('runenpfad.dice.title') + ' – ' + opponentName(game.opponent)));
            body.append(el('p', 'rp-talk-tip rp-games-rules', t('runenpfad.dice.rules')));
            canvas = el('canvas', 'rp-games-canvas');
            canvas.width = CANVAS_W;
            canvas.height = CANVAS_H;
            canvas.setAttribute('aria-hidden', 'true');
            canvas.addEventListener('click', function (ev) {
                if (game.phase !== 'rolled') return;
                const r = canvas.getBoundingClientRect();
                const x = (ev.clientX - r.left) * (CANVAS_W / (r.width || CANVAS_W));
                const y = (ev.clientY - r.top) * (CANVAS_H / (r.height || CANVAS_H));
                for (let i = 0; i < 2; i++) {
                    if (y >= ROW_Y.you && y <= ROW_Y.you + DIE && x >= DIE_X[i] && x <= DIE_X[i] + DIE) act(function () { D.reroll(game, i); });
                }
            });
            const status = diceStatus();
            const line = el('p', 'rp-trade-msg rp-games-status' + status.cls, status.text);
            line.setAttribute('role', 'status');
            body.append(canvas, line, choices);
            let n = 0;
            if (game.phase === 'stake') {
                const list = D.stakes(game);
                if (list.indexOf(stake) < 0) stake = list[0] || 0;
                list.forEach(function (s) {
                    const b = btn(choices, 'rp-games-stake' + (s === stake ? ' is-on' : ''), t('runenpfad.dice.stake') + ' ' + s, String(++n), function () { stake = s; render(); });
                    b.setAttribute('aria-pressed', s === stake ? 'true' : 'false');
                });
                const rollBtn = btn(choices, 'rp-btn-primary', t('runenpfad.dice.roll'), String(++n), function () { act(function () { D.roll(game, stake); }); });
                rollBtn.disabled = !stake;
            } else if (game.phase === 'rolled') {
                for (let i = 0; i < 2; i++) {
                    (function (k) {
                        btn(choices, 'rp-games-die', FACES.charAt(game.mine[k] - 1) + ' ' + game.mine[k], String(++n), function () { act(function () { D.reroll(game, k); }); });
                    })(i);
                }
                btn(choices, 'rp-btn-primary', t('runenpfad.dice.hold'), String(++n), function () { act(function () { D.hold(game); }); });
            } else {
                const again = btn(choices, 'rp-btn-primary', t('runenpfad.dice.again'), String(++n), function () { act(function () { D.next(game); }); });
                again.disabled = D.status(game) !== 'ok';
            }
            const leave = btn(choices, 'rp-games-leave', t('runenpfad.dice.leave'), 'Esc', function () { close(); });
            leave.disabled = game.phase === 'rolled'; // an open round must be played out
            paintDice();
        }

        // ---- task board ----
        function afterBoard(before) {
            forward(before);
            if (sim && sim.storyDirty && typeof o.flush === 'function') o.flush();
            focusIdx = 0;
            render();
        }

        function renderBoard() {
            const B = board();
            const a = B.active(sim.profile);
            body.append(head(t('runenpfad.bounty.title')));
            body.append(el('p', 'rp-talk-tip rp-games-rules', t('runenpfad.bounty.intro')));
            const list = el('ul', 'rp-games-offers');
            if (a) {
                const row = el('li', 'rp-games-offer is-active');
                row.append(el('strong', 'rp-games-offer-tag', t('runenpfad.bounty.active')), el('span', 'rp-games-offer-text', t(a.def.key)));
                const bar = el('span', 'rp-games-bar');
                const fillBar = el('i');
                fillBar.style.width = Math.round(100 * a.n / a.need) + '%';
                bar.append(fillBar);
                bar.setAttribute('role', 'img');
                bar.setAttribute('aria-label', fill('runenpfad.bounty.progress', { count: a.n, need: a.need }));
                row.append(bar, el('span', 'rp-games-progress rp-num', fill('runenpfad.bounty.progress', { count: a.n, need: a.need })), rewardChip(a.def.reward));
                const claim = btn(row, 'rp-btn-gold rp-games-claim', fill('runenpfad.bounty.claim', { coins: a.def.reward }), '', function () {
                    const before = evCount();
                    B.claim(sim);
                    afterBoard(before);
                });
                claim.disabled = !a.done;
                btn(row, 'rp-games-drop', t('runenpfad.bounty.drop'), '', function () { const before = evCount(); B.drop(sim); afterBoard(before); });
                list.append(row);
            } else {
                B.offers(sim.profile).forEach(function (id, i) {
                    const d = B.def(id);
                    const row = el('li', 'rp-games-offer');
                    row.dataset.rpBounty = String(id);
                    row.append(el('span', 'rp-games-offer-text', t(d.key)), rewardChip(d.reward));
                    btn(row, 'rp-games-accept', t('runenpfad.bounty.accept'), String(i + 1), function () { const before = evCount(); B.accept(sim, id); afterBoard(before); });
                    list.append(row);
                });
            }
            const choices = el('div', 'rp-talk-choices rp-games-actions');
            btn(choices, 'rp-games-leave', t('runenpfad.bounty.close'), 'Esc', function () { close(); });
            body.append(list, choices);
        }

        // ---- lifecycle ----
        function render() {
            body.replaceChildren();
            canvas = null;
            if (!open || !sim) return;
            rootEl.dataset.rpGames = mode;
            rootEl.setAttribute('aria-label', t(mode === 'dice' ? 'runenpfad.dice.title' : 'runenpfad.bounty.title'));
            if (mode === 'dice' && game) renderDice(); else renderBoard();
            setFocus(Math.min(focusIdx, Math.max(0, focusables().length - 1)));
        }

        function show(kind, theSim, state) {
            if (open) close(true);
            mode = kind;
            sim = theSim;
            game = state || null;
            stake = 0;
            focusIdx = 0;
            open = true;
            openedAt = perf.now();
            rootEl.hidden = false;
            render();
            void rootEl.offsetWidth; // let the entry transition start from the hidden state
            rootEl.classList.add('is-in');
            if (typeof o.cue === 'function') o.cue('ui_board');
            if (typeof o.onChange === 'function') o.onChange();
            return true;
        }

        // Without `force` an open dice round cannot be left; forced closes (stage change, dispose) lose the round.
        function close(force) {
            if (!open) return;
            if (game && game.phase === 'rolled') {
                if (force !== true) return;
                const before = evCount();
                dice().abandon(game);
                forward(before);
            }
            open = false;
            rootEl.classList.remove('is-in');
            rootEl.hidden = true;
            body.replaceChildren();
            if (sim && sim.storyDirty && typeof o.flush === 'function') o.flush();
            sim = null;
            game = null;
            if (typeof o.onChange === 'function') o.onChange();
        }

        function endSong() {
            root.clearTimeout(songTimer);
            songTimer = 0;
            if (songMusic && typeof songMusic.setOverride === 'function') {
                try { songMusic.setOverride(null); } catch (err) { /* music is optional */ }
            }
            songMusic = null;
        }

        function song(theme, music) {
            endSong();
            if (!music || typeof music.setOverride !== 'function') return;
            try { music.setOverride(theme); } catch (err) { return; }
            songMusic = music;
            songTimer = root.setTimeout(endSong, SONG_MS);
        }

        function request(req, ctx) {
            const theSim = ctx && ctx.sim;
            if (disposed || !theSim || typeof req !== 'string') return false;
            if (req === 'board') return board() ? show('board', theSim, null) : false;
            let m = /^dice:([a-z_]+)$/.exec(req);
            if (m) return dice() ? show('dice', theSim, dice().create(theSim, m[1])) : false;
            m = /^song:([a-z_]+)$/.exec(req);
            if (m) {
                song(m[1], ctx.music);
                return true;
            }
            return false;
        }

        // Returns true when the key was consumed. KeyP falls through (pause dialog); Esc closes the window.
        function handleKey(ev) {
            if (!open || isBlocked()) return false;
            const code = ev.code;
            if (code === 'KeyP') return false;
            const digit = /^(?:Digit|Numpad)([1-9])$/.exec(code);
            const back = code === 'ArrowUp' || code === 'ArrowLeft' || code === 'KeyW' || code === 'KeyA';
            const fwd = code === 'ArrowDown' || code === 'ArrowRight' || code === 'KeyS' || code === 'KeyD';
            const go = code === 'Enter' || code === 'NumpadEnter' || code === 'Space';
            if (!(digit || back || fwd || go || code === 'Escape')) return true;
            ev.preventDefault();
            if (ev.repeat) return true;
            if (back || fwd) {
                setFocus(focusIdx + (back ? -1 : 1));
                return true;
            }
            if (perf.now() - openedAt < GUARD_MS) return true;
            if (code === 'Escape') {
                close();
                return true;
            }
            const list = focusables();
            const target = go ? list[focusIdx] : list[Number(digit[1]) - 1];
            if (target) target.click();
            return true;
        }

        function dispose() {
            disposed = true;
            endSong();
            close(true);
            open = false;
            rootEl.hidden = true;
            if (current === controller) current = null;
        }

        const controller = {
            root: rootEl, request: request, handleKey: handleKey, close: function () { close(); }, endSong: endSong, dispose: dispose,
            isOpen: function () { return open; },
            // A stage change or restart: the panel closes, a running song ends.
            reset: function () { endSong(); close(true); }
        };
        current = controller;
        return controller;
    }

    // Static entry point of the contract: forwards to the controller of the running window.
    function request(req, ctx) { return current ? current.request(req, ctx) : false; }

    root.RunenpfadGamesUi = { create: create, request: request };
})(typeof window !== 'undefined' ? window : globalThis);
