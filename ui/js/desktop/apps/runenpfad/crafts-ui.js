/* Runenpfad crafts window: the kitchen (pick two ingredients, cook a meal) and the forge (a canvas meter with a moving needle; strike three
 * times). Rendered inside the games window (games-ui.js): `render(mode, api)` fills `api.body`. DOM is built through `api.el`, which wraps
 * createElement and textContent only; no innerHTML, no URLs. The rules live in crafts.js; this file only shows them and forwards the keys. */
(function (root) {
    'use strict';

    const W = 340;
    const H = 104;
    const BAR_X = 16;
    const BAR_W = W - 32;
    const BAR_Y = 50;
    const BAR_H = 26;
    const DOT = { turnip: '#c9a0dc', ham: '#e9967a', frostberry: '#7cc7ff', honey: '#f2c14e' };
    const MAX_FRAME = 0.05;       // a stalled tab must not throw the needle across the bar
    const FLASH_S = 0.35;

    function C() { return root.RunenpfadCrafts || null; }
    function nameOf(api, id) { return api.t('runenpfad.npc.item.' + id + '.name'); }

    // ---- kitchen ----

    function renderCook(api) {
        const Cr = C();
        const sim = api.sim;
        const st = api.state;
        const N = root.RunenpfadNpcs;
        if (!st.pick) st.pick = [];
        const pick = st.pick;
        api.body.append(api.head(api.t('runenpfad.craft.cook_title')));
        api.body.append(api.el('p', 'rp-talk-tip rp-games-rules', api.t('runenpfad.craft.cook_intro')));

        const grid = api.el('div', 'rp-craft-grid');
        grid.setAttribute('role', 'group');
        let n = 0;
        Cr.INGREDIENTS.forEach(function (id) {
            const have = N.itemCount(sim, id);
            const on = pick.indexOf(id) >= 0;
            const b = api.btn(grid, 'rp-craft-item' + (on ? ' is-on' : ''), nameOf(api, id) + ' ' + api.fill('runenpfad.craft.count', { count: have }), String(++n), function () {
                const at = pick.indexOf(id);
                if (at >= 0) pick.splice(at, 1);
                else { if (pick.length >= 2) pick.shift(); pick.push(id); }
                st.done = null;
                api.rerender();
            });
            const dot = api.el('i', 'rp-craft-dot');
            dot.style.background = DOT[id] || '#ddd';
            b.append(dot);
            b.disabled = have < 1;
            b.setAttribute('aria-pressed', on ? 'true' : 'false');
            b.dataset.rpIngredient = id;
        });
        api.body.append(grid);

        // The preview line: the dish the picked pair makes, or why it does not work.
        const preview = api.el('div', 'rp-craft-preview');
        preview.setAttribute('role', 'status');
        let verdict = 'pick';
        let recipe = null;
        if (pick.length === 2) {
            recipe = Cr.recipeFor(pick[0], pick[1]);
            verdict = Cr.canCook(sim, pick[0], pick[1]);
        }
        if (st.done) {
            preview.classList.add('is-ok');
            preview.append(api.el('strong', 'rp-craft-meal', api.t('runenpfad.craft.' + st.done)), api.el('span', 'rp-craft-desc', api.t('runenpfad.craft.' + st.done + '_desc')));
        } else if (recipe && verdict !== 'full') {
            preview.append(api.el('strong', 'rp-craft-meal', api.t('runenpfad.craft.' + recipe)), api.el('span', 'rp-craft-desc', api.t('runenpfad.craft.' + recipe + '_desc')));
            if (verdict === 'missing') preview.classList.add('is-bad');
        } else if (verdict === 'full') {
            preview.classList.add('is-bad');
            preview.append(api.el('span', 'rp-craft-desc', api.t('runenpfad.craft.full')));
        } else if (verdict === 'none') {
            preview.classList.add('is-bad');
            preview.append(api.el('span', 'rp-craft-desc', api.t('runenpfad.craft.no_recipe')));
        } else {
            preview.append(api.el('span', 'rp-craft-desc', api.t('runenpfad.craft.pick')));
        }
        api.body.append(preview);

        const actions = api.el('div', 'rp-talk-choices rp-games-actions');
        const cookBtn = api.btn(actions, 'rp-btn-primary rp-craft-go', api.t('runenpfad.craft.cook'), String(++n), function () {
            const before = api.evCount();
            const r = Cr.cook(sim, pick[0], pick[1]);
            api.forward(before);
            st.pick = [];
            st.done = r.ok ? r.recipe : null;
            api.rerender();
        });
        cookBtn.disabled = pick.length < 2 || verdict !== 'ok';
        api.btn(actions, 'rp-games-leave', api.t('runenpfad.craft.leave'), 'Esc', function () { api.close(); });
        api.body.append(actions);
    }

    // ---- forge ----

    function meterX(pos) { return BAR_X + Math.max(0, Math.min(1, pos)) * BAR_W; }

    function roundRect(cx, x, y, w, h, r) {
        cx.beginPath();
        if (typeof cx.roundRect === 'function') cx.roundRect(x, y, w, h, r); else cx.rect(x, y, w, h);
    }

    function paintMeter(st) {
        const canvas = st.canvas;
        const run = st.forge;
        if (!canvas || !run || !canvas.getContext) return;
        const cx = canvas.getContext('2d');
        if (!cx) return;
        const Cr = C();
        cx.clearRect(0, 0, W, H);
        // The anvil glow behind the bar.
        const glow = cx.createRadialGradient ? cx.createRadialGradient(W / 2, BAR_Y + BAR_H / 2, 10, W / 2, BAR_Y + BAR_H / 2, W * 0.6) : null;
        if (glow) {
            glow.addColorStop(0, 'rgba(255, 140, 50, 0.28)');
            glow.addColorStop(1, 'rgba(255, 140, 50, 0)');
            cx.fillStyle = glow;
            cx.fillRect(0, 0, W, H);
        }
        // Track.
        cx.fillStyle = '#2a1d12';
        cx.strokeStyle = '#8a6a3a';
        cx.lineWidth = 2;
        roundRect(cx, BAR_X, BAR_Y, BAR_W, BAR_H, 5);
        cx.fill();
        cx.stroke();
        // The good zone, highlighted in gold with a bright core.
        const zx = meterX(0.5 - Cr.GOOD_ZONE);
        const zw = meterX(0.5 + Cr.GOOD_ZONE) - zx;
        cx.fillStyle = 'rgba(255, 200, 90, 0.45)';
        cx.fillRect(zx, BAR_Y + 2, zw, BAR_H - 4);
        cx.fillStyle = 'rgba(255, 236, 170, 0.8)';
        cx.fillRect(zx + zw * 0.35, BAR_Y + 2, zw * 0.3, BAR_H - 4);
        cx.strokeStyle = '#ffd27a';
        cx.lineWidth = 2;
        cx.strokeRect(zx, BAR_Y + 2, zw, BAR_H - 4);
        // Earlier strikes stay as marks: green for good, red for bad.
        (st.marks || []).forEach(function (m) {
            const x = meterX(m.pos);
            cx.fillStyle = m.q === 'good' ? '#7ce08a' : '#f08080';
            cx.beginPath();
            cx.moveTo(x, BAR_Y + BAR_H + 3);
            cx.lineTo(x - 5, BAR_Y + BAR_H + 12);
            cx.lineTo(x + 5, BAR_Y + BAR_H + 12);
            cx.closePath();
            cx.fill();
        });
        // The needle.
        if (run.phase === 'run') {
            const x = meterX(run.pos);
            cx.strokeStyle = '#fff4d8';
            cx.lineWidth = 3;
            cx.beginPath();
            cx.moveTo(x, BAR_Y - 8);
            cx.lineTo(x, BAR_Y + BAR_H + 8);
            cx.stroke();
            cx.fillStyle = '#ff6a3a';
            cx.beginPath();
            cx.moveTo(x, BAR_Y - 2);
            cx.lineTo(x - 7, BAR_Y - 14);
            cx.lineTo(x + 7, BAR_Y - 14);
            cx.closePath();
            cx.fill();
        }
        // A short flash after a strike (skipped under reduced motion).
        if (st.flash && st.flash.t > 0 && !st.reduced) {
            cx.fillStyle = (st.flash.q === 'good' ? 'rgba(255, 240, 170, ' : 'rgba(240, 120, 110, ') + (0.5 * st.flash.t / FLASH_S).toFixed(3) + ')';
            roundRect(cx, BAR_X, BAR_Y, BAR_W, BAR_H, 5);
            cx.fill();
        }
        // Strike pips.
        for (let i = 0; i < Cr.STRIKES; i++) {
            const q = run.strikes[i];
            cx.fillStyle = q === 'good' ? '#7ce08a' : (q === 'bad' ? '#f08080' : '#4a3a28');
            cx.beginPath();
            cx.arc(W / 2 + (i - 1) * 22, 18, 7, 0, Math.PI * 2);
            cx.fill();
            cx.strokeStyle = '#8a6a3a';
            cx.lineWidth = 1.5;
            cx.stroke();
        }
    }

    // One animation loop per rendered meter: it dies with the next render, with the window and with the run.
    function startLoop(st, api) {
        const raf = root.requestAnimationFrame;
        const perf = root.performance;
        if (typeof raf !== 'function' || !perf) return;
        const token = st.token;
        let last = perf.now();
        function frame() {
            if (st.token !== token || !api.isOpen() || !st.forge || st.forge.phase !== 'run') return;
            if (st.canvas && st.canvas.isConnected === false) return;
            const now = perf.now();
            const dt = Math.min(MAX_FRAME, Math.max(0, (now - last) / 1000));
            last = now;
            C().forgeStep(st.forge, dt);
            if (st.flash && st.flash.t > 0) st.flash.t = Math.max(0, st.flash.t - dt);
            paintMeter(st);
            raf(frame);
        }
        raf(frame);
    }

    function strike(api) {
        const st = api.state;
        const run = st.forge;
        if (!run || run.phase !== 'run') return;
        const pos = run.pos;
        const before = api.evCount();
        const q = C().forgeStrike(api.sim, run);
        api.forward(before);
        (st.marks || (st.marks = [])).push({ pos: pos, q: q });
        st.flash = { t: FLASH_S, q: q };
        api.rerender();
        if (st.forge && st.forge.phase === 'run') api.setFocus(0);
    }

    function renderForgeOptions(api) {
        const Cr = C();
        const st = api.state;
        api.body.append(api.head(api.t('runenpfad.craft.forge_title')));
        api.body.append(api.el('p', 'rp-talk-tip rp-games-rules', api.t('runenpfad.craft.forge_intro')));
        const list = api.el('ul', 'rp-games-offers rp-craft-forge-list');
        const actions = api.el('div', 'rp-talk-choices rp-games-actions');
        let n = 0;
        Cr.forgeOptions(api.sim).forEach(function (o) {
            const row = api.el('li', 'rp-games-offer rp-craft-job' + (o.done ? ' is-done' : ''));
            row.dataset.rpForge = o.id;
            row.append(api.el('strong', 'rp-games-offer-tag', api.t('runenpfad.craft.' + o.id)), api.el('span', 'rp-games-offer-text', api.t('runenpfad.craft.' + o.id + '_desc')));
            const need = api.el('span', 'rp-craft-need' + (o.have ? ' is-have' : ''), api.fill('runenpfad.craft.needs', { name: nameOf(api, o.item) }));
            row.append(need);
            if (o.coins > 0) {
                const chip = api.el('span', 'rp-chip rp-games-reward');
                chip.append(api.hud.icon('coin', 14), api.el('span', 'rp-num', api.fill('runenpfad.craft.cost', { coins: o.coins })));
                row.append(chip);
            }
            const b = api.btn(row, 'rp-btn-primary', o.done ? api.t('runenpfad.craft.done') : api.t('runenpfad.craft.' + o.id), String(++n), function () {
                const run = Cr.forgeStart(api.sim, o.id);
                if (!run) return;
                st.forge = run;
                st.marks = [];
                st.flash = null;
                api.rerender();
                api.setFocus(0);
            });
            b.disabled = !o.ready;
            list.append(row);
        });
        api.body.append(list);
        api.btn(actions, 'rp-games-leave', api.t('runenpfad.craft.leave'), 'Esc', function () { api.close(); });
        api.body.append(actions);
    }

    function renderForgeRun(api) {
        const st = api.state;
        const run = st.forge;
        api.body.append(api.head(api.t('runenpfad.craft.' + run.id)));
        api.body.append(api.el('p', 'rp-talk-tip rp-games-rules', api.t('runenpfad.craft.forge_intro')));
        const canvas = api.el('canvas', 'rp-games-canvas rp-craft-meter');
        canvas.width = W;
        canvas.height = H;
        canvas.setAttribute('aria-hidden', 'true');
        st.canvas = canvas;
        const status = api.el('p', 'rp-trade-msg rp-games-status rp-craft-status' + (run.phase === 'done' ? (run.ok ? ' is-ok' : ' is-bad') : ''),
            run.phase === 'done' ? api.t(run.ok ? 'runenpfad.craft.success' : 'runenpfad.craft.fail') : (run.strikes.length + ' / ' + C().STRIKES));
        status.setAttribute('role', 'status');
        api.body.append(canvas, status);
        const actions = api.el('div', 'rp-talk-choices rp-games-actions');
        // The strike button comes first, so Space and Enter (which press the focused button) strike.
        if (run.phase === 'run') {
            api.btn(actions, 'rp-btn-primary rp-craft-strike', api.t('runenpfad.craft.strike'), 'Space', function () { strike(api); });
        } else {
            api.btn(actions, 'rp-btn-primary', api.t('runenpfad.craft.again'), '1', function () {
                st.forge = null;
                st.marks = [];
                api.rerender();
            });
        }
        api.btn(actions, 'rp-games-leave', api.t('runenpfad.craft.leave'), 'Esc', function () { api.close(); });
        api.body.append(actions);
        paintMeter(st);
        if (run.phase === 'run') startLoop(st, api);
    }

    // Entry point called by the games window for the modes 'cook' and 'forge'.
    function render(mode, api) {
        if (!C() || !api || !api.state) return;
        const st = api.state;
        st.token = (st.token || 0) + 1;
        st.canvas = null;
        if (st.reduced == null) st.reduced = !!(root.matchMedia && root.matchMedia('(prefers-reduced-motion: reduce)').matches);
        if (mode === 'cook') renderCook(api);
        else if (st.forge) renderForgeRun(api);
        else renderForgeOptions(api);
    }

    root.RunenpfadCraftsUi = { render: render, paintMeter: paintMeter };
})(typeof window !== 'undefined' ? window : globalThis);
