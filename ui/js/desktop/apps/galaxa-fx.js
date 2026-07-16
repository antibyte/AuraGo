(function () {
    'use strict';
    const GC = window.GalaxaCore = window.GalaxaCore || {};

    // NEW: Supplementary visual effects package (boss shockwave rings, warp
    // speed-line streaks, powerup sparkle + glints, directional spark cones,
    // combo screen-edge pulse, ship afterimage ghosts, boss death rumble,
    // graze particles, combo fire trail, stage-clear confetti, magnet pull
    // lines, player death flash, boss entrance rumble, free-drifting debris
    // motes). All effect state lives on ctx.G (fx* fields, lazily
    // initialized) so it dies with the game instance; caps scale with
    // ctx.settings.particles via GC.FX_CAPS.
    GC.createFx = function (ctx) {
        let lastGhostT = 0;
        let lastGrazeT = 0;
        let lastFireTrailT = 0;
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

        // --- Boss death rumble: expanding screen shake + vignette spike ------
        function fxBossDeathRumble(x, y) {
            ctx.G.shkT = Math.max(ctx.G.shkT, 1200);
            ctx.G.shkM = Math.max(ctx.G.shkM, 10);
            ctx.G.shkX = x;
            ctx.G.shkY = y;
            ctx.G.damageVignetteT = Math.max(ctx.G.damageVignetteT, 600);
            ctx.G.fxRumbleVignette = { t: 0, dur: 1200, intensity: 1 };
            if (ctx.G.fxRumbleRings) ctx.G.fxRumbleRings = [];
            for (let i = 0; i < 5; i++) {
                ctx.G.fxRumbleRings.push({
                    x, y, t: 0, delay: i * 180, dur: 600,
                    maxR: 60 + i * 30, w: 4 - i * 0.6
                });
            }
            if (ctx.SFX && ctx.SFX.bossDeathRumble) ctx.SFX.bossDeathRumble(x);
        }

        // --- Graze particle flare near player --------------------------------
        function fxGrazeSpark(x, y, bulletVx, bulletVy) {
            const n = caps().graze;
            const baseA = Math.atan2(bulletVy || 1, bulletVx || 0) + Math.PI;
            for (let i = 0; i < n; i++) {
                const a = baseA + (Math.random() - 0.5) * 1.8;
                const sp = 25 + Math.random() * 55;
                ctx.G.part.push(ctx.getParticle({
                    x, y, vx: Math.cos(a) * sp, vy: Math.sin(a) * sp,
                    life: 150 + Math.random() * 120, t: 0,
                    col: Math.random() < 0.4 ? '#ffffff' : '#88ddff',
                    size: 1, spark: true, shape: 'diamond'
                }));
            }
            if (ctx.SFX && ctx.SFX.graze) ctx.SFX.graze(x);
        }

        // --- Combo fire trail behind player ----------------------------------
        function spawnFireTrail() {
            const G = ctx.G, p = G.p;
            if (!p.alive) return;
            const mult = G.comboMult || 1;
            const intensity = Math.min(1, (mult - 1) / 7);
            if (intensity <= 0) return;
            const n = Math.max(1, Math.round(caps().fireTrail * intensity));
            const cols = mult >= 8 ? ['#ff4444', '#ff8800', '#ffcc00', '#ffffff'] :
                         mult >= 4 ? ['#ff8800', '#ffcc00', '#ffee44'] :
                                     ['#ffcc00', '#ffee44'];
            for (let i = 0; i < n; i++) {
                const ox = (Math.random() - 0.5) * 10;
                const speedMod = G.shipTilt ? Math.abs(G.shipTilt) * 0.5 + 0.5 : 0.3;
                ctx.G.part.push(ctx.getParticle({
                    x: p.x + ox, y: p.y + 16 + Math.random() * 4,
                    vx: (Math.random() - 0.5) * 14 * speedMod,
                    vy: 20 + Math.random() * 30,
                    life: GC.FX_FIRE_TRAIL_LIFE * (0.5 + intensity * 0.5),
                    t: 0,
                    col: cols[Math.floor(Math.random() * cols.length)],
                    size: 1 + Math.floor(Math.random() * 2),
                    spark: true, shape: 'circle', fireTrail: true
                }));
            }
            if (ctx.SFX && ctx.SFX.fireTrailCrackle) ctx.SFX.fireTrailCrackle(p.x);
        }

        // --- Stage-clear confetti burst --------------------------------------
        function fxStageClearConfetti(x, y) {
            if (!ctx.G.fxConfetti) ctx.G.fxConfetti = [];
            const n = caps().confetti;
            const confettiCols = ['#ff4444', '#4488ff', '#44ff88', '#ffcc00', '#ff44ff', '#88ddff'];
            for (let i = 0; i < n; i++) {
                const a = -Math.PI / 2 + (Math.random() - 0.5) * 2.8;
                const sp = 80 + Math.random() * 140;
                ctx.G.fxConfetti.push({
                    x: x + (Math.random() - 0.5) * 30,
                    y: y + (Math.random() - 0.5) * 20,
                    vx: Math.cos(a) * sp,
                    vy: Math.sin(a) * sp,
                    t: 0, life: 1600 + Math.random() * 800,
                    col: confettiCols[Math.floor(Math.random() * confettiCols.length)],
                    w: 3 + Math.random() * 4,
                    h: 2 + Math.random() * 3,
                    rot: Math.random() * 6.28,
                    rotSpd: (Math.random() - 0.5) * 8
                });
            }
            if (ctx.SFX && ctx.SFX.stageFanfare) ctx.SFX.stageFanfare(x);
        }

        // --- Magnet pull-line visual -----------------------------------------
        function fxMagnetPull(powerupX, powerupY) {
            if (!ctx.G.fxPullLines) ctx.G.fxPullLines = [];
            const n = GC.FX_MAGNET_PULL_LINES;
            ctx.G.fxPullLines.push({
                x: powerupX, y: powerupY,
                t: 0, life: 300,
                targetX: ctx.G.p.x, targetY: ctx.G.p.y,
                lines: n
            });
            if (ctx.SFX && ctx.SFX.magnetPull) ctx.SFX.magnetPull(powerupX);
        }

        // --- Player death screen flash (directional) -------------------------
        function fxPlayerDeathFlash(x, y) {
            const c = caps();
            const n = c.deathFlash;
            ctx.G.fxDeathFlash = { x, y, t: 0, dur: 500, intensity: n, rings: [] };
            for (let i = 0; i < n; i++) {
                ctx.G.fxDeathFlash.rings.push({
                    delay: i * 60, maxR: 40 + i * 20, dur: 300 + i * 60
                });
            }
            if (ctx.SFX && ctx.SFX.playerDeathWhoosh) ctx.SFX.playerDeathWhoosh(x);
        }

        // --- Boss entrance rumble --------------------------------------------
        function fxBossEntrance(x, y) {
            ctx.G.shkT = Math.max(ctx.G.shkT, 600);
            ctx.G.shkM = Math.max(ctx.G.shkM, 6);
            ctx.G.shkX = x;
            ctx.G.shkY = y;
            ctx.G.fxBossEnter = { x, y, t: 0, dur: 800 };
            if (!ctx.G.fxRings) ctx.G.fxRings = [];
            if (ctx.SFX && ctx.SFX.bossEntrance) ctx.SFX.bossEntrance(x);
            ctx.G.fxRings.push({ x, y, t: 0, delay: 0, dur: 600, maxR: 100, w: 4 });
            ctx.G.fxRings.push({ x, y, t: 0, delay: 120, dur: 450, maxR: 70, w: 3 });
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
            // --- Graze detection vs enemy bullets ----------------------------
            if (G.st === 'PLAYING' && G.p.alive) {
                lastGrazeT += dtMs;
                if (lastGrazeT >= GC.FX_GRAZE_COOLDOWN) {
                    for (let i = 0; i < G.ebul.length; i++) {
                        const b = G.ebul[i];
                        const bw = (b.w || 2) / 2;
                        const bh = (b.h || 4) / 2;
                        const dx = b.x - G.p.x;
                        const dy = b.y - G.p.y;
                        const dist = Math.sqrt(dx * dx + dy * dy);
                        const threshold = GC.FX_GRAZE_RADIUS + Math.max(bw, bh);
                        if (dist < threshold && dist > 6) {
                            fxGrazeSpark(b.x, b.y, b.vx || 0, b.vy || 0);
                            lastGrazeT = 0;
                            break;
                        }
                    }
                }
            }
            // --- Combo fire trail --------------------------------------------
            if (G.st === 'PLAYING' && G.p.alive && (G.comboMult || 1) >= GC.FX_FIRE_TRAIL_COMBO) {
                lastFireTrailT += dtMs;
                if (lastFireTrailT >= GC.FX_FIRE_TRAIL_INTERVAL) {
                    lastFireTrailT = 0;
                    spawnFireTrail();
                }
            } else {
                lastFireTrailT = 0;
            }
            // --- Confetti ----------------------------------------------------
            if (G.fxConfetti && G.fxConfetti.length) {
                let w = 0;
                for (let i = 0; i < G.fxConfetti.length; i++) {
                    const c = G.fxConfetti[i];
                    c.x += c.vx * dt;
                    c.y += c.vy * dt;
                    c.vy += 80 * dt;
                    c.vx *= 0.97;
                    c.rot += c.rotSpd * dt;
                    c.t += dtMs;
                    if (c.t < c.life && c.y < ctx.H + 20) G.fxConfetti[w++] = c;
                }
                G.fxConfetti.length = w;
            }
            // --- Pull lines --------------------------------------------------
            if (G.fxPullLines && G.fxPullLines.length) {
                let w = 0;
                for (let i = 0; i < G.fxPullLines.length; i++) {
                    const pl = G.fxPullLines[i];
                    pl.t += dtMs;
                    if (pl.t < pl.life) G.fxPullLines[w++] = pl;
                }
                G.fxPullLines.length = w;
            }
            // --- Player death flash rings ------------------------------------
            if (G.fxDeathFlash) {
                G.fxDeathFlash.t += dtMs;
                if (G.fxDeathFlash.t >= G.fxDeathFlash.dur) {
                    G.fxDeathFlash = null;
                }
            }
            // --- Boss entrance decay -----------------------------------------
            if (G.fxBossEnter) {
                G.fxBossEnter.t += dtMs;
                if (G.fxBossEnter.t >= G.fxBossEnter.dur) G.fxBossEnter = null;
            }
            // --- Boss death rumble rings -------------------------------------
            if (G.fxRumbleRings && G.fxRumbleRings.length) {
                let w = 0;
                for (let i = 0; i < G.fxRumbleRings.length; i++) {
                    const r = G.fxRumbleRings[i];
                    r.t += dtMs;
                    if (r.t < r.delay + r.dur) G.fxRumbleRings[w++] = r;
                }
                G.fxRumbleRings.length = w;
            }
            // --- Rumble vignette decay ---------------------------------------
            if (G.fxRumbleVignette) {
                G.fxRumbleVignette.t += dtMs;
                if (G.fxRumbleVignette.t >= G.fxRumbleVignette.dur) G.fxRumbleVignette = null;
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
            // --- Standard FX rings ---
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
            // --- Boss death rumble rings ---
            if (G.fxRumbleRings && G.fxRumbleRings.length) {
                c.save();
                c.globalCompositeOperation = 'lighter';
                for (let i = 0; i < G.fxRumbleRings.length; i++) {
                    const r = G.fxRumbleRings[i];
                    if (r.t < r.delay) continue;
                    const pr = Math.min(1, (r.t - r.delay) / r.dur);
                    const rad = easeOutCubic(pr) * r.maxR;
                    if (rad < 1) continue;
                    const alpha = (1 - pr) * 0.9;
                    c.lineWidth = Math.max(1, r.w * (1 - pr));
                    c.globalAlpha = alpha;
                    c.strokeStyle = i % 2 === 0 ? '#ff2222' : '#ff8800';
                    c.beginPath(); c.arc(r.x, r.y, rad, 0, Math.PI * 2); c.stroke();
                }
                c.restore();
                c.globalAlpha = 1;
            }
            // --- Player death flash rings ---
            if (G.fxDeathFlash) {
                const df = G.fxDeathFlash;
                const pct = df.t / df.dur;
                c.save();
                c.globalCompositeOperation = 'lighter';
                for (let i = 0; i < df.rings.length; i++) {
                    const r = df.rings[i];
                    const localT = df.t - r.delay;
                    if (localT < 0 || localT > r.dur) continue;
                    const rp = Math.min(1, localT / r.dur);
                    const rad = easeOutCubic(rp) * r.maxR;
                    const alpha = (1 - rp) * 0.6;
                    c.lineWidth = Math.max(1, 3 * (1 - rp));
                    c.globalAlpha = alpha;
                    c.strokeStyle = i % 2 === 0 ? '#ff4444' : '#ffaa44';
                    c.beginPath(); c.arc(df.x, df.y, rad, 0, Math.PI * 2); c.stroke();
                }
                c.restore();
                c.globalAlpha = 1;
            }
            // --- Glints ---
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

        // --- Draw: boss entrance shockwave (drawn over game) ------------------
        function fxDrawBossEnter(c) {
            const G = ctx.G;
            if (!G.fxBossEnter) return;
            const pr = Math.min(1, G.fxBossEnter.t / G.fxBossEnter.dur);
            const alpha = (1 - pr) * 0.5;
            if (alpha <= 0.01) return;
            const rad = easeOutCubic(pr) * 80;
            c.save();
            c.globalCompositeOperation = 'lighter';
            c.globalAlpha = alpha;
            c.strokeStyle = '#ff4444';
            c.lineWidth = Math.max(1, 4 * (1 - pr));
            c.shadowBlur = 20;
            c.shadowColor = '#ff4444';
            c.beginPath(); c.arc(G.fxBossEnter.x, G.fxBossEnter.y, rad, 0, Math.PI * 2); c.stroke();
            c.shadowBlur = 0;
            c.restore();
            c.globalAlpha = 1;
        }

        // --- Draw: confetti particles -----------------------------------------
        function fxDrawConfetti(c) {
            const G = ctx.G;
            if (!G.fxConfetti || !G.fxConfetti.length) return;
            c.save();
            for (let i = 0; i < G.fxConfetti.length; i++) {
                const f = G.fxConfetti[i];
                const alpha = Math.max(0, 1 - f.t / f.life);
                c.globalAlpha = alpha * 0.8;
                c.save();
                c.translate(f.x, f.y);
                c.rotate(f.rot);
                c.fillStyle = f.col;
                c.fillRect(-f.w / 2, -f.h / 2, f.w, f.h);
                c.restore();
            }
            c.restore();
            c.globalAlpha = 1;
        }

        // --- Draw: magnet pull-lines ------------------------------------------
        function fxDrawPullLines(c) {
            const G = ctx.G;
            if (!G.fxPullLines || !G.fxPullLines.length) return;
            c.save();
            c.globalCompositeOperation = 'lighter';
            for (let i = 0; i < G.fxPullLines.length; i++) {
                const pl = G.fxPullLines[i];
                const alpha = Math.max(0, 1 - pl.t / pl.life) * 0.35;
                if (alpha <= 0.01) continue;
                c.globalAlpha = alpha;
                c.strokeStyle = GC.FX_MAGNET_PULL_COL;
                c.lineWidth = 1;
                for (let j = 0; j < pl.lines; j++) {
                    const phase = pl.t * 0.003 + j * 1.047;
                    const midX = (pl.x + pl.targetX) / 2 + Math.sin(phase) * 20;
                    const midY = (pl.y + pl.targetY) / 2 + Math.cos(phase * 0.7) * 15;
                    c.beginPath();
                    c.moveTo(pl.x, pl.y);
                    c.quadraticCurveTo(midX, midY, pl.targetX, pl.targetY);
                    c.stroke();
                }
            }
            c.restore();
            c.globalAlpha = 1;
        }

        // --- Draw: boss death rumble vignette over everything ------------------
        function fxDrawRumbleOverlay(c) {
            const G = ctx.G;
            if (!G.fxRumbleVignette) return;
            const pr = Math.min(1, G.fxRumbleVignette.t / G.fxRumbleVignette.dur);
            const alpha = Math.pow(1 - pr, 2) * 0.3;
            if (alpha <= 0.01) return;
            c.save();
            const vg = c.createRadialGradient(ctx.W / 2, ctx.H / 2, ctx.H * 0.15, ctx.W / 2, ctx.H / 2, ctx.H * 0.8);
            vg.addColorStop(0, 'rgba(255,0,0,0)');
            vg.addColorStop(1, 'rgba(255,0,0,' + alpha + ')');
            c.fillStyle = vg;
            c.fillRect(0, 0, ctx.W, ctx.H);
            c.restore();
            c.globalAlpha = 1;
        }

        // --- Draw: fire trail particles (drawn as a distinct pass) ------------
        function fxDrawFireTrail(c) {
            const G = ctx.G;
            if (!G.part) return;
            c.save();
            c.globalCompositeOperation = 'lighter';
            for (let i = 0; i < G.part.length; i++) {
                const p = G.part[i];
                if (!p.fireTrail) continue;
                const alpha = Math.max(0, 1 - p.t / p.life) * 0.5;
                c.globalAlpha = alpha;
                c.fillStyle = p.col;
                const sz = p.size || 2;
                c.beginPath();
                c.arc(p.x, p.y, sz, 0, Math.PI * 2);
                c.fill();
            }
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
        ctx.fxBossDeathRumble = fxBossDeathRumble;
        ctx.fxGrazeSpark = fxGrazeSpark;
        ctx.fxStageClearConfetti = fxStageClearConfetti;
        ctx.fxMagnetPull = fxMagnetPull;
        ctx.fxPlayerDeathFlash = fxPlayerDeathFlash;
        ctx.fxBossEntrance = fxBossEntrance;
        ctx.fxDrawBossEnter = fxDrawBossEnter;
        ctx.fxDrawConfetti = fxDrawConfetti;
        ctx.fxDrawPullLines = fxDrawPullLines;
        ctx.fxDrawRumbleOverlay = fxDrawRumbleOverlay;
        ctx.fxDrawFireTrail = fxDrawFireTrail;
    };
})();
