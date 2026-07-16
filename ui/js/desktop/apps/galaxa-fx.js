(function () {
    'use strict';
    const GC = window.GalaxaCore = window.GalaxaCore || {};

    // NEW: Supplementary visual effects package (boss shockwave rings, warp
    // speed-line streaks, powerup sparkle + glints, directional spark cones,
    // combo screen-edge pulse, ship afterimage ghosts). All effect state lives
    // on ctx.G (fx* fields, lazily initialized) so it dies with the game
    // instance; caps scale with ctx.settings.particles via GC.FX_CAPS.
    GC.createFx = function (ctx) {
        let lastGhostT = 0;
        // Persistent warp streak pool — allocated once, reused every frame
        // (no per-frame allocations while the warp effect is active).
        const streaks = [];
        for (let i = 0; i < GC.FX_CAPS.high.streak; i++) {
            streaks.push({ ang: Math.random() * Math.PI * 2, dist: 20 + Math.random() * 320, spd: 260 + Math.random() * 360, len: 26 + Math.random() * 60 });
        }

        function caps() { return GC.FX_CAPS[ctx.settings.particles] || GC.FX_CAPS.high; }
        function easeOutCubic(t) { const f = t - 1; return f * f * f + 1; }

        // --- Boss shockwave: staggered rings with chromatic fringe -----------
        function fxBossShockwave(x, y) {
            if (!ctx.G.fxRings) ctx.G.fxRings = [];
            const n = caps().ring;
            const defs = [
                { delay: 0, maxR: 170, dur: 750, w: 5 },
                { delay: 90, maxR: 120, dur: 600, w: 4 },
                { delay: 180, maxR: 70, dur: 450, w: 3 }
            ];
            for (let i = 0; i < n && i < defs.length; i++) {
                ctx.G.fxRings.push({ x, y, t: 0, delay: defs[i].delay, dur: defs[i].dur, maxR: defs[i].maxR, w: defs[i].w });
            }
        }

        // --- Warp streaks -----------------------------------------------------
        function fxWarpStart() { ctx.G.fxWarpT = GC.FX_WARP_DUR; }

        // --- Powerup sparkle burst + rising glints ---------------------------
        function fxPowerupSparkle(x, y, col, rarity) {
            const c = caps();
            const big = rarity === 'legendary' ? 1.6 : rarity === 'rare' ? 1.25 : 1;
            const sparkN = Math.round(c.sparkle * big);
            for (let i = 0; i < sparkN; i++) {
                const a = (i / sparkN) * Math.PI * 2 + Math.random() * 0.5;
                const sp = 40 + Math.random() * 90;
                ctx.G.part.push(ctx.getParticle({
                    x, y, vx: Math.cos(a) * sp, vy: Math.sin(a) * sp,
                    life: 220 + Math.random() * 220, t: 0, col: Math.random() < 0.3 ? '#ffffff' : col,
                    size: 1 + Math.floor(Math.random() * 2), spark: true, shape: 'diamond'
                }));
            }
            if (!ctx.G.fxGlints) ctx.G.fxGlints = [];
            const glintN = Math.round(c.glint * big);
            for (let i = 0; i < glintN; i++) {
                if (ctx.G.fxGlints.length >= c.glint * 4) break;
                ctx.G.fxGlints.push({
                    x: x + (Math.random() - 0.5) * 22, y: y + (Math.random() - 0.5) * 14,
                    vx: (Math.random() - 0.5) * 24, vy: -14 - Math.random() * 22,
                    t: 0, life: 500 + Math.random() * 350, col: Math.random() < 0.4 ? '#ffffff' : col,
                    r: 2 + Math.random() * 2.5, ph: Math.random() * 6.28
                });
            }
        }

        // --- Directional spark cone (bullet impacts) -------------------------
        function fxSparkCone(x, y, col, dirX, dirY) {
            const n = caps().sparkCone;
            const len = Math.sqrt(dirX * dirX + dirY * dirY) || 1;
            // Sparks spray back along the reversed travel direction of the bullet.
            const baseA = Math.atan2(-dirY / len, -dirX / len);
            for (let i = 0; i < n; i++) {
                const a = baseA + (Math.random() - 0.5) * 1.1;
                const sp = 60 + Math.random() * 110;
                ctx.G.part.push(ctx.getParticle({
                    x, y, vx: Math.cos(a) * sp, vy: Math.sin(a) * sp,
                    life: 90 + Math.random() * 130, t: 0, col: Math.random() < 0.35 ? '#ffffff' : (col || '#ffff88'),
                    size: 1, spark: true, shape: 'diamond'
                }));
            }
        }

        // --- Combo milestone screen-edge pulse -------------------------------
        function fxComboPulse(level) {
            const mult = ctx.G.comboMult || 1;
            const col = mult >= 16 ? '#ffffff' : mult >= 8 ? '#ff4444' : mult >= 4 ? '#ffcc00' : '#4488ff';
            ctx.G.fxEdgePulse = { t: 0, dur: 450, col };
        }

        // --- Ship afterimage ghosts ------------------------------------------
        function spawnGhost() {
            const G = ctx.G, p = G.p;
            if (!G.fxGhosts) G.fxGhosts = [];
            const max = caps().ghost;
            if (G.fxGhosts.length >= max) G.fxGhosts.shift();
            G.fxGhosts.push({
                x: p.x, y: p.y, tilt: G.shipTilt || 0, pitch: G.shipPitch || 0,
                frame: (ctx.getPlayerSpriteFrame && ctx.getPlayerSpriteFrame()) || ctx.SP.playerIcon || ctx.SP.player,
                col: G.parryActive > 0 ? '#ffffff' : GC.FX_GHOST_COL,
                t: 0, life: GC.FX_GHOST_LIFE
            });
        }

        // --- Per-frame update (hooked from updateExp in galaxa-game.js) ------
        function updateFX(dt) {
            const G = ctx.G;
            const dtMs = dt * 1000;
            if (G.fxWarpT > 0) {
                G.fxWarpT -= dtMs;
                const n = Math.min(streaks.length, caps().streak);
                for (let i = 0; i < n; i++) {
                    const s = streaks[i];
                    s.dist += s.spd * dt * 2.2;
                    if (s.dist > 420) { s.dist = 16 + Math.random() * 40; s.ang = Math.random() * Math.PI * 2; s.spd = 260 + Math.random() * 360; }
                }
            }
            if (G.fxRings && G.fxRings.length) {
                let w = 0;
                for (let i = 0; i < G.fxRings.length; i++) { const r = G.fxRings[i]; r.t += dtMs; if (r.t < r.delay + r.dur) G.fxRings[w++] = r; }
                G.fxRings.length = w;
            }
            if (G.fxGlints && G.fxGlints.length) {
                let w = 0;
                for (let i = 0; i < G.fxGlints.length; i++) {
                    const g = G.fxGlints[i];
                    g.x += g.vx * dt; g.y += g.vy * dt; g.vy -= 26 * dt; g.vx *= 0.98;
                    g.t += dtMs; if (g.t < g.life) G.fxGlints[w++] = g;
                }
                G.fxGlints.length = w;
            }
            if (G.fxEdgePulse) {
                G.fxEdgePulse.t += dtMs;
                if (G.fxEdgePulse.t >= G.fxEdgePulse.dur) G.fxEdgePulse = null;
            }
            if (G.fxGhosts && G.fxGhosts.length) {
                let w = 0;
                for (let i = 0; i < G.fxGhosts.length; i++) { const g = G.fxGhosts[i]; g.t += dtMs; if (g.t < g.life) G.fxGhosts[w++] = g; }
                G.fxGhosts.length = w;
            }
            // Ghost spawn check: speed-boosted movement or active parry window.
            if (G.st === 'PLAYING' && G.p.alive) {
                lastGhostT += dtMs;
                const speedy = G.activePU && (G.activePU.type === 'speed' || G.activePU.type === 'hyper_speed');
                const moving = G.inp.l || G.inp.r || G.inp.u || G.inp.d;
                if (((speedy && moving) || G.parryActive > 0) && lastGhostT >= GC.FX_GHOST_INTERVAL) {
                    lastGhostT = 0;
                    spawnGhost();
                }
            } else {
                lastGhostT = 0;
            }
        }

        // --- Draw: warp streaks behind the game layer -------------------------
        function fxDrawBack(c) {
            const G = ctx.G;
            if (!G.fxWarpT || G.fxWarpT <= 0) return;
            const progress = 1 - G.fxWarpT / GC.FX_WARP_DUR;
            const intensity = Math.sin(Math.min(1, progress) * Math.PI);
            if (intensity <= 0.01) return;
            const cx = ctx.W / 2, cy = ctx.H / 2;
            const n = Math.min(streaks.length, caps().streak);
            c.save();
            c.globalCompositeOperation = 'lighter';
            c.lineWidth = 1.5;
            for (let i = 0; i < n; i++) {
                const s = streaks[i];
                const ca = Math.cos(s.ang), sa = Math.sin(s.ang);
                const r0 = s.dist, r1 = s.dist + s.len * (0.6 + intensity);
                c.globalAlpha = Math.min(1, (s.dist / 260)) * intensity * 0.7;
                c.strokeStyle = i % 3 === 0 ? '#aaddff' : '#ffffff';
                c.beginPath();
                c.moveTo(cx + ca * r0, cy + sa * r0);
                c.lineTo(cx + ca * r1, cy + sa * r1);
                c.stroke();
            }
            c.restore();
            c.globalAlpha = 1;
        }

        // --- Draw: shockwave rings + glints above explosions ------------------
        function fxDrawMid(c) {
            const G = ctx.G;
            if (G.fxRings && G.fxRings.length) {
                c.save();
                c.globalCompositeOperation = 'lighter';
                for (let i = 0; i < G.fxRings.length; i++) {
                    const r = G.fxRings[i];
                    if (r.t < r.delay) continue;
                    const pr = Math.min(1, (r.t - r.delay) / r.dur);
                    const rad = easeOutCubic(pr) * r.maxR;
                    if (rad < 1) continue;
                    const alpha = (1 - pr) * 0.8;
                    const lw = Math.max(1, r.w * (1 - pr));
                    // Chromatic fringe: red/cyan offset arcs around a white core.
                    c.lineWidth = lw;
                    c.globalAlpha = alpha * 0.55;
                    c.strokeStyle = '#ff3344';
                    c.beginPath(); c.arc(r.x - 2, r.y, rad + 2, 0, Math.PI * 2); c.stroke();
                    c.strokeStyle = '#33ddff';
                    c.beginPath(); c.arc(r.x + 2, r.y, rad - 2 > 1 ? rad - 2 : 1, 0, Math.PI * 2); c.stroke();
                    c.globalAlpha = alpha;
                    c.strokeStyle = '#ffffff';
                    c.lineWidth = Math.max(1, lw * 0.6);
                    c.beginPath(); c.arc(r.x, r.y, rad, 0, Math.PI * 2); c.stroke();
                }
                c.restore();
                c.globalAlpha = 1;
            }
            if (G.fxGlints && G.fxGlints.length) {
                c.save();
                c.globalCompositeOperation = 'lighter';
                for (let i = 0; i < G.fxGlints.length; i++) {
                    const g = G.fxGlints[i];
                    const fade = Math.max(0, 1 - g.t / g.life);
                    const twinkle = 0.55 + 0.45 * Math.sin(g.t * 0.03 + g.ph);
                    c.globalAlpha = fade * twinkle;
                    c.fillStyle = g.col;
                    const r = g.r;
                    c.fillRect(g.x - r, g.y - 0.5, r * 2, 1);
                    c.fillRect(g.x - 0.5, g.y - r, 1, r * 2);
                    c.globalAlpha = fade * twinkle * 0.5;
                    const r2 = r * 0.55;
                    c.fillRect(g.x - r2, g.y - r2, 1, 1);
                    c.fillRect(g.x + r2, g.y - r2, 1, 1);
                    c.fillRect(g.x - r2, g.y + r2, 1, 1);
                    c.fillRect(g.x + r2, g.y + r2, 1, 1);
                }
                c.restore();
                c.globalAlpha = 1;
            }
        }

        // --- Draw: afterimage ghosts under the live ship ----------------------
        function fxDrawGhosts(c) {
            const G = ctx.G;
            if (!G.fxGhosts || !G.fxGhosts.length || !G.p.alive) return;
            const ghostCols = { 1: '', 2: '', 3: '' };
            for (let i = 0; i < G.fxGhosts.length; i++) {
                const g = G.fxGhosts[i];
                const alpha = (1 - g.t / g.life) * 0.35;
                ghostCols[1] = g.col; ghostCols[2] = g.col; ghostCols[3] = g.col;
                c.save();
                c.globalAlpha = alpha;
                c.translate(g.x, g.y);
                c.rotate(g.tilt);
                c.transform(1, g.pitch, 0, 1 - Math.abs(g.pitch) * 0.35, 0, 0);
                ctx.drawSp(c, g.frame, ghostCols, -16, -16, false, true);
                c.restore();
            }
            c.globalAlpha = 1;
        }

        // --- Draw: combo screen-edge pulse over the game layer ----------------
        function fxDrawOverlay(c) {
            const G = ctx.G;
            if (!G.fxEdgePulse) return;
            const pr = Math.min(1, G.fxEdgePulse.t / G.fxEdgePulse.dur);
            const alpha = Math.pow(1 - pr, 1.5) * 0.55;
            if (alpha <= 0.01) return;
            const inset = easeOutCubic(pr) * 14 + 2;
            const strip = 18 * (1 - pr) + 4;
            c.save();
            c.globalCompositeOperation = 'lighter';
            c.globalAlpha = alpha;
            c.fillStyle = G.fxEdgePulse.col;
            c.fillRect(0, 0, ctx.W, strip);
            c.fillRect(0, ctx.H - strip, ctx.W, strip);
            c.fillRect(0, 0, strip, ctx.H);
            c.fillRect(ctx.W - strip, 0, strip, ctx.H);
            c.globalAlpha = alpha * 0.9;
            c.strokeStyle = '#ffffff';
            c.lineWidth = 2;
            c.strokeRect(inset, inset, ctx.W - inset * 2, ctx.H - inset * 2);
            c.restore();
            c.globalAlpha = 1;
        }

        ctx.fxBossShockwave = fxBossShockwave;
        ctx.fxWarpStart = fxWarpStart;
        ctx.fxPowerupSparkle = fxPowerupSparkle;
        ctx.fxSparkCone = fxSparkCone;
        ctx.fxComboPulse = fxComboPulse;
        ctx.updateFX = updateFX;
        ctx.fxDrawBack = fxDrawBack;
        ctx.fxDrawMid = fxDrawMid;
        ctx.fxDrawGhosts = fxDrawGhosts;
        ctx.fxDrawOverlay = fxDrawOverlay;
    };
})();
