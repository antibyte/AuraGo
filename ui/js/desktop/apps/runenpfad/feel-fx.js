/* Runenpfad feel effects: run and skid dust, pickup pops, checkpoint light columns and iris transitions. Canvas-facing, read-only. */
(function (root) {
    'use strict';

    const MAX_PUFFS = 24;
    const MAX_POPS = 16;
    const MAX_COLUMNS = 4;
    const DUST_EVERY = 0.22;
    const DUST_SPEED = 170;
    const COLUMN_S = 0.8;
    const IRIS_START_S = 0.55;
    const IRIS_DEFEAT_S = 0.9;
    const IRIS_WIN_S = 0.7;
    const DEFEAT_HOLD_R = 30;
    const IRIS_FILL = '#07050d';
    const IRIS_RIM = '#f4c874';
    const POP_CLIP = { pickup_coin: 'fx-pickup.coin', pickup_gem: 'fx-pickup.gem', pickup_star: 'fx-pickup.gem', pickup_key: 'fx-pickup.gem', pickup_treasure: 'fx-pickup.gem' };
    const POP_HEIGHT = { pickup_coin: 56, pickup_gem: 64, pickup_star: 78, pickup_key: 78, pickup_treasure: 88 };
    const POP_RGB = { pickup_coin: '255,214,90', pickup_gem: '126,232,250', pickup_star: '255,240,160', pickup_key: '255,214,90', pickup_treasure: '255,200,120' };

    function clamp01(v) {
        return v < 0 ? 0 : (v > 1 ? 1 : v);
    }

    function smooth(v) {
        const t = clamp01(v);
        return t * t * (3 - 2 * t);
    }

    function create(opts) {
        const getArt = opts && typeof opts.getArt === 'function' ? opts.getArt : function () { return null; };
        const O = { height: 0, scale: 0, flip: false, alpha: 1, composite: '', rotation: 0 };
        let puffs = [];
        let pops = [];
        let columns = [];
        let runT = 0;
        let lastElapsed = null;
        let start = null;   // { t, active }
        let defeat = null;  // { t, x, y }
        let win = null;     // { t, x, y }
        let reducedNow = false;

        function art(clip) {
            const a = getArt();
            const Art = root.RunenpfadArt;
            return a && Art && typeof Art.has === 'function' && Art.has(a, clip) ? a : null;
        }

        function reset(sim) {
            puffs = [];
            pops = [];
            columns = [];
            runT = 0;
            defeat = null;
            win = null;
            lastElapsed = sim && typeof sim.elapsed === 'number' ? sim.elapsed : null;
            start = { t: 0 };
        }

        function addPuff(x, y, scale, alpha, flip) {
            if (puffs.length >= MAX_PUFFS) puffs.shift();
            puffs.push({ x: x, y: y, t: 0, scale: scale, alpha: alpha, flip: flip, seed: (puffs.length * 7 + Math.floor(x)) % 5 });
        }

        function onEvent(ev, sim) {
            if (!ev || !ev.type) return;
            const d = ev.detail || {};
            const pl = sim && sim.player;
            const type = ev.type;
            if (POP_CLIP[type]) {
                const x = typeof d.x === 'number' ? d.x : (pl ? pl.x : 0);
                const y = typeof d.y === 'number' ? d.y : (pl ? pl.y - 24 : 0);
                if (pops.length >= MAX_POPS) pops.shift();
                pops.push({ x: x, y: y, t: 0, type: type });
            } else if (type === 'checkpoint' || type === 'revive') {
                const x = typeof d.x === 'number' ? d.x : (pl ? pl.x : 0);
                const y = typeof d.y === 'number' ? d.y : (pl ? pl.y : 0);
                if (columns.length >= MAX_COLUMNS) columns.shift();
                columns.push({ x: x, y: y + 8, t: 0, rgb: type === 'revive' ? '126,232,250' : '255,214,140' });
            } else if (type === 'skid') {
                if (reducedNow) return;
                const dir = d.dir < 0 ? -1 : 1;
                const x = typeof d.x === 'number' ? d.x : (pl ? pl.x : 0);
                const y = typeof d.y === 'number' ? d.y : (pl ? pl.y : 0);
                addPuff(x + dir * 14, y, 0.8, 0.8, dir < 0);
            } else if (type === 'player_defeat') {
                if (pl && !defeat) defeat = { t: 0, x: pl.x, y: pl.y - 26 };
                start = null;
            } else if (type === 'portal_win') {
                const pt = sim && sim.stage && sim.stage.portal;
                if (!win) win = { t: 0, x: pt ? pt.x : (pl ? pl.x : 0), y: pt ? pt.y : (pl ? pl.y - 26 : 0) };
                start = null;
            }
        }

        function step(dt, sim) {
            const fdt = Math.min(0.1, Math.max(0, dt || 0));
            // The simulation clock: only the stage-start iris and the run dust follow it, so pause and cutscenes hold them.
            let simDt = fdt;
            if (sim && typeof sim.elapsed === 'number') {
                const adv = lastElapsed === null ? 0 : sim.elapsed - lastElapsed;
                lastElapsed = sim.elapsed;
                simDt = adv > 0 ? Math.min(0.1, adv) : 0;
            }
            const playing = !sim || sim.state === 'playing';
            if (playing) {
                defeat = null;
                win = null;
            } else {
                start = null;
            }
            if (start) {
                start.t += simDt;
                if (start.t >= IRIS_START_S) start = null;
            }
            if (defeat) defeat.t += fdt;
            if (win) win.t += fdt;

            const pl = sim && sim.player;
            if (pl && playing && simDt > 0 && pl.onGround && Math.abs(pl.vx) > DUST_SPEED && !sim.interior) {
                runT -= simDt;
                if (runT <= 0) {
                    runT = DUST_EVERY;
                    if (!reducedNow) addPuff(pl.x - pl.facing * 10, pl.y, 0.45, 0.7, pl.facing > 0);
                }
            } else {
                runT = Math.min(runT, 0.05);
            }

            for (let i = puffs.length - 1; i >= 0; i--) {
                puffs[i].t += fdt;
                if (puffs[i].t > 0.5) puffs.splice(i, 1);
            }
            for (let i = pops.length - 1; i >= 0; i--) {
                pops[i].t += fdt;
                if (pops[i].t > 0.38) pops.splice(i, 1);
            }
            for (let i = columns.length - 1; i >= 0; i--) {
                columns[i].t += fdt;
                if (columns[i].t > COLUMN_S) columns.splice(i, 1);
            }
        }

        function drawPuff(ctx, p) {
            const a = art('fx-dust.play');
            const Art = root.RunenpfadArt;
            if (a) {
                O.height = 0;
                O.scale = p.scale;
                O.flip = p.flip;
                O.alpha = p.alpha;
                O.composite = 'source-over';
                O.rotation = 0;
                if (Art.draw(ctx, a, 'fx-dust.play', p.t, p.x, p.y - 8 * p.scale, O)) return;
            }
            // No art: three soft circles drifting up and out.
            const k = clamp01(p.t / 0.4);
            const fade = (1 - k) * p.alpha * 0.45;
            if (fade < 0.01) return;
            ctx.save();
            ctx.fillStyle = 'rgba(214, 204, 184, ' + fade.toFixed(3) + ')';
            for (let i = 0; i < 3; i++) {
                const dx = (i - 1) * 7 * p.scale * 2 * (0.4 + k) * (p.flip ? -1 : 1);
                const r = (4 + i) * p.scale * 1.8 * (0.6 + k * 0.9);
                ctx.beginPath();
                ctx.arc(p.x + dx, p.y - 3 - k * 10 * p.scale - i * 2, r, 0, Math.PI * 2);
                ctx.fill();
            }
            ctx.restore();
        }

        function drawPop(ctx, p) {
            const clip = POP_CLIP[p.type];
            const a = art(clip);
            const Art = root.RunenpfadArt;
            const fade = p.t > 0.28 ? clamp01(1 - (p.t - 0.28) / 0.1) : 1;
            if (a) {
                O.height = POP_HEIGHT[p.type];
                O.scale = 0;
                O.flip = false;
                O.alpha = fade;
                O.composite = 'lighter';
                O.rotation = 0;
                if (Art.draw(ctx, a, clip, p.t, p.x, p.y, O)) return;
            }
            // No art: a quick expanding ring.
            const k = clamp01(p.t / 0.3);
            ctx.save();
            ctx.globalCompositeOperation = 'lighter';
            ctx.strokeStyle = 'rgba(' + POP_RGB[p.type] + ',' + ((1 - k) * 0.8).toFixed(3) + ')';
            ctx.lineWidth = 3 * (1 - k) + 1;
            ctx.beginPath();
            ctx.arc(p.x, p.y, 8 + k * 26, 0, Math.PI * 2);
            ctx.stroke();
            ctx.restore();
        }

        function drawColumn(ctx, c, reduced) {
            const k = clamp01(c.t / COLUMN_S);
            const env = k < 0.25 ? k / 0.25 : 1 - (k - 0.25) / 0.75;
            const h = reduced ? 130 : 50 + 130 * (1 - Math.pow(1 - k, 2));
            const a = 0.55 * env * (reduced ? 0.7 : 1);
            if (a < 0.01) return;
            ctx.save();
            ctx.globalCompositeOperation = 'lighter';
            ctx.translate(c.x, c.y - h * 0.5);
            ctx.scale(24, h * 0.5);
            const g = ctx.createRadialGradient(0, 0.55, 0, 0, 0, 1);
            g.addColorStop(0, 'rgba(' + c.rgb + ',' + a.toFixed(3) + ')');
            g.addColorStop(0.5, 'rgba(' + c.rgb + ',' + (a * 0.4).toFixed(3) + ')');
            g.addColorStop(1, 'rgba(' + c.rgb + ',0)');
            ctx.fillStyle = g;
            ctx.fillRect(-1, -1, 2, 2);
            ctx.restore();
        }

        // World layer, drawn under the world transform next to the sprite fx.
        function drawWorld(ctx, frame) {
            if (!ctx) return;
            const reduced = !!(frame && frame.reducedMotion);
            reducedNow = reduced;
            for (let i = 0; i < columns.length; i++) drawColumn(ctx, columns[i], reduced);
            if (!reduced) for (let i = 0; i < puffs.length; i++) drawPuff(ctx, puffs[i]);
            for (let i = 0; i < pops.length; i++) drawPop(ctx, pops[i]);
        }

        function irisCover(ctx, w, h, fx, fy, r, rim) {
            const maxR = Math.hypot(Math.max(fx, w - fx), Math.max(fy, h - fy)) + 8;
            const rr = Math.min(r, maxR);
            ctx.save();
            ctx.fillStyle = IRIS_FILL;
            ctx.beginPath();
            ctx.rect(0, 0, w, h);
            if (rr > 0.5) ctx.arc(fx, fy, rr, 0, Math.PI * 2, true);
            ctx.fill('evenodd');
            if (rim && rr > 2 && rr < maxR - 4) {
                ctx.strokeStyle = IRIS_RIM;
                ctx.globalAlpha = 0.85;
                ctx.lineWidth = 3;
                ctx.beginPath();
                ctx.arc(fx, fy, rr, 0, Math.PI * 2);
                ctx.stroke();
            }
            ctx.restore();
        }

        function fade(ctx, w, h, alpha) {
            if (alpha <= 0.004) return;
            ctx.save();
            ctx.fillStyle = 'rgba(7, 5, 13, ' + clamp01(alpha).toFixed(3) + ')';
            ctx.fillRect(0, 0, w, h);
            ctx.restore();
        }

        // Screen layer: the iris is the top layer, drawn after the flashes and before the post-processing composite.
        function drawScreen(ctx, w, h, sim, cam, reduced) {
            if (!ctx) return;
            const z = cam && cam.zoom > 0 ? cam.zoom : 1;
            const cx = cam ? cam.x : 0;
            const cy = cam ? cam.y : 0;
            const pl = sim && sim.player;
            if (defeat || win) {
                const e = defeat || win;
                const dur = defeat ? IRIS_DEFEAT_S : IRIS_WIN_S;
                const p = smooth(e.t / dur);
                if (reduced) { fade(ctx, w, h, p); return; }
                const fx = (e.x - cx) * z;
                const fy = (e.y - cy) * z;
                const maxR = Math.hypot(Math.max(fx, w - fx), Math.max(fy, h - fy)) + 8;
                const minR = defeat ? DEFEAT_HOLD_R * z : 0;
                irisCover(ctx, w, h, fx, fy, minR + (1 - p) * (maxR - minR), true);
                return;
            }
            if (start && pl) {
                const p = smooth(start.t / IRIS_START_S);
                if (reduced) { fade(ctx, w, h, 1 - p); return; }
                const fx = (pl.x - cx) * z;
                const fy = (pl.y - 26 - cy) * z;
                const maxR = Math.hypot(Math.max(fx, w - fx), Math.max(fy, h - fy)) + 8;
                irisCover(ctx, w, h, fx, fy, p * maxR, true);
            }
        }

        return { reset: reset, onEvent: onEvent, step: step, drawWorld: drawWorld, drawScreen: drawScreen };
    }

    root.RunenpfadFeelFx = { create: create };
})(typeof window !== 'undefined' ? window : globalThis);
