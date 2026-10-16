(function () {
    'use strict';
    const GC = window.GalaxaCore = window.GalaxaCore || {};

    // NEW: Enemy-side juice package (galaxa-enemy-fx.js). Adds the counterpart to
    // galaxa-fx.js for attacker behaviour: charge-up telegraph glints, feint /
    // formation-shuffle blink rings, rage auras for enraged divers, chained
    // lightning between rapid consecutive kills, per-hit impact pops, and the
    // smoldering crash-dive state (SMOLDER): hit enemies that tumble toward the
    // player, trailing smoke and embers, and detonate after a seeded delay.
    //
    // All transient state lives on ctx.G (fx* fields) or in the module-local
    // `marks`/`chains` pools so it dies with the game instance and never leaks
    // into a new run (both pools are cleared whenever the game leaves PLAYING).
    // Particle counts scale with GC.FX_CAPS via ctx.settings.particles, and every
    // movement-heavy effect honours ctx.settings.reducedMotion plus the OS
    // prefers-reduced-motion query, mirroring galaxa-fx.js.
    GC.createEnemyFx = function (ctx) {
        const marks = [];   // ring / pop / blink marks in world space
        const chains = [];  // short-lived lightning arcs between kill points

        function caps() { return GC.FX_CAPS[ctx.settings.particles] || GC.FX_CAPS.high; }
        function reducedMotion() {
            if (ctx.settings.reducedMotion) return true;
            try { return !!(window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)').matches); } catch (e) { return false; }
        }
        function addMark(m) {
            const cap = caps().enemyMarks || 18;
            if (marks.length >= cap) marks.shift();
            marks.push(m);
        }

        // --- Impact pop on a fresh hit ---------------------------------------
        function fxEnemyHit(x, y, col) {
            if (x == null || y == null) return;
            addMark({ kind: 'pop', x, y, t: 0, dur: 200, col: col || '#ffffff', r: 9 });
            if (ctx.settings.particles === 'low' || reducedMotion()) return;
            const n = ctx.settings.particles === 'medium' ? 2 : 3;
            for (let i = 0; i < n; i++) {
                const a = Math.random() * Math.PI * 2;
                const sp = 50 + Math.random() * 70;
                ctx.G.part.push(ctx.getParticle({
                    x, y, vx: Math.cos(a) * sp, vy: Math.sin(a) * sp,
                    life: 110 + Math.random() * 90, t: 0, col: col || '#ffe08a',
                    size: 1, spark: true, shape: 'diamond'
                }));
            }
        }

        // --- Blink / feint / shuffle ring ------------------------------------
        function fxEnemyBlink(x, y, col) {
            if (x == null || y == null) return;
            addMark({ kind: 'blink', x, y, t: 0, dur: 380, col: col || '#ff5ad0', r: 22 });
        }

        // --- Feint abort cue: ring + brief chromatic aberration ripple -------
        function fxEnemyFeint(x, y) {
            fxEnemyBlink(x, y, '#ff5ad0');
            if (!reducedMotion()) ctx.G.chromAb = Math.max(ctx.G.chromAb || 0, 60);
        }

        // --- Chained lightning between rapid consecutive kills ---------------
        function fxKillChain(a, b) {
            if (!a || !b || reducedMotion()) return;
            const cap = caps().chain || 8;
            if (chains.length >= cap) chains.shift();
            chains.push({ x1: a.x, y1: a.y, x2: b.x, y2: b.y, t: 0, dur: 260, seed: Math.floor(Math.random() * 997) });
        }

        // --- Squad-dive screen cue (reuses the shared combo edge pulse) ------
        function fxSquadDiveCue() {
            ctx.G.fxEdgePulse = { t: 0, dur: 560, col: '#ff8844' };
            if (ctx.SFX && ctx.SFX.squadDive) ctx.SFX.squadDive(ctx.W / 2);
        }

        // --- Formation shuffle puff at two swapped slots ---------------------
        function fxFormationShuffle(x1, y1, x2, y2) {
            fxEnemyBlink(x1, y1, '#88ddff');
            fxEnemyBlink(x2, y2, '#88ddff');
            if (ctx.SFX && ctx.SFX.shuffleRumble) ctx.SFX.shuffleRumble((x1 + x2) / 2);
        }

        // --- Charge-up telegraph glint in front of an attacker ---------------
        function telegraphGlint(e) {
            if (reducedMotion() || !e) return;
            if (!ctx.G.fxGlints) ctx.G.fxGlints = [];
            const cap = (caps().glint || 6) * 2;
            if (ctx.G.fxGlints.length >= cap) return;
            ctx.G.fxGlints.push({
                x: e.x, y: e.y - 6, vx: 0, vy: -8, t: 0, life: 260,
                col: '#ffd27a', r: 2.5 + Math.random() * 1.5, ph: Math.random() * 6.28
            });
        }

        // --- Per-frame update (hooked from ctx.updateExp) --------------------
        function updateEnemyFx(dt) {
            const G = ctx.G;
            const playing = G.st === 'PLAYING' || G.st === 'STAGE_INTRO';
            // Leaving combat drops all enemy-side marks so nothing freezes over
            // menus, the shop or the game-over overlay.
            if (!playing) {
                if (marks.length) marks.length = 0;
                if (chains.length) chains.length = 0;
                if (G.fxSquadDive) G.fxSquadDive = null;
                return;
            }
            const dtMs = dt * 1000;
            let w = 0;
            for (let i = 0; i < marks.length; i++) { const m = marks[i]; m.t += dtMs; if (m.t < m.dur) marks[w++] = m; }
            marks.length = w;
            w = 0;
            for (let i = 0; i < chains.length; i++) { const c = chains[i]; c.t += dtMs; if (c.t < c.dur) chains[w++] = c; }
            chains.length = w;
            if (G.fxSquadDive) G.fxSquadDive = null;
            updateSmolder(dt, dtMs);
            // Telegraph charge-up glints: throttled scan over charging attackers.
            if (G.p.alive && !reducedMotion()) {
                ctx._enemyFxT = (ctx._enemyFxT || 0) + dtMs;
                if (ctx._enemyFxT >= 90) {
                    ctx._enemyFxT = 0;
                    let budget = caps().telegraph || 3;
                    for (let i = 0; i < G.enemies.length && budget > 0; i++) {
                        const e = G.enemies[i];
                        if (e.st === 'DEAD' || e.sectorBoss) continue;
                        if (e.divePending || e.shotPending) { telegraphGlint(e); budget--; }
                    }
                }
            }
        }

        // --- Smoldering crash dives ------------------------------------------
        // Ignition turns a fatally hit enemy into a tumbling, smoking projectile
        // (new SMOLDER state) instead of exploding it right away. The selection
        // roll lives here too (maybeSmolder), so combat stays untouched apart
        // from the one hook call. Detonation is fully centralized in
        // smolderDetonate: it reuses the ordinary kill bookkeeping (score,
        // splitter/carrier, pickups, credits, combo, kills) so no reward or
        // side effect is lost or doubled.
        // Smolder selection on a fatal bullet hit: seeded roll may turn the
        // enemy into a tumbling diver. True tells the caller to skip the
        // immediate death handling (score, explosion, pickups, splits).
        function maybeSmolder(e, b) {
            const c = GC.SMOLDER || {};
            if (!e || e.st === 'DEAD' || e.st === 'ENTER') return false;
            if (ctx.G.chal || ctx.G.st !== 'PLAYING' || ctx.G.freezeT > 0) return false;
            if (e.sectorBoss || e.type === 'boss' || e.type === 'miniboss') return false;
            if (ctx.G.stage < c.stageMin || ctx.settings.diff === 'easy') return false;
            let active = 0;
            for (const o of ctx.G.enemies) { if (o.st === 'SMOLDER' && ++active >= c.maxSimultaneous) return false; }
            if (ctx.random() >= (ctx.settings.diff === 'hard' ? c.chance + c.hardBonus : c.chance)) return false;
            const vs = c.variants && c.variants.length ? c.variants : [{ id: 'ember', smoke: 1, sparks: 1 }];
            const v = vs[Math.floor(ctx.random() * vs.length) % vs.length];
            const a = Math.atan2(ctx.G.p.y - e.y, ctx.G.p.x - e.x), s = 130 + ctx.random() * 90;
            return igniteSmolder(e, { vx: Math.cos(a) * s + (ctx.random() - 0.5) * 40, vy: Math.sin(a) * s * 0.5 - 60 - ctx.random() * 60, spin: (ctx.random() < 0.5 ? -1 : 1) * (2.4 + ctx.random() * (c.maxSpin - 2.4)), blowMs: c.minMs + ctx.random() * (c.maxMs - c.minMs), variant: v.id, smoke: v.smoke, sparks: v.sparks, killVx: (b && b.vx) || 0, killVy: (b && b.vy) || -ctx.PB_SPEED, ignitedAir: e.st === 'DIVING' || e.st === 'FEINT' });
        }
        function igniteSmolder(e, prep) {
            if (!e || e.st === 'DEAD') return false;
            const conf = GC.SMOLDER;
            if (!conf) return false;
            e.st = 'SMOLDER';
            e.smolderT = prep && prep.blowMs != null ? prep.blowMs : conf.minMs;
            const _pv = prep && prep.vx, _pw = prep && prep.vy;
            e.smolderVx = _pv || 0;
            e.smolderVy = _pw || 0;
            e.smolderSpin = (prep && prep.spin) || 3;
            e.smolderRot = 0;                       // seeded by spin only
            e.smolderSeed = ctx.random();           // wobble/flash variants
            e.smolderVariant = (prep && prep.variant) || 'ember';
            e.smolderSmoke = (prep && prep.smoke) || 1;
            e.smolderSparks = (prep && prep.sparks) || 1;
            e.smolderKillVx = (prep && prep.killVx) || 0;
            e.smolderKillVy = (prep && prep.killVy) || 0;
            e.smolderIgnitedAir = !!(prep && prep.ignitedAir);
            e.rot = 0; e.rotTarget = 0; e.rotTimer = 0; e.rotDuration = 0;
            e.divePending = false; e.shotPending = false; e.shotReady = false;
            e.rageMode = 0; e.rageSpeedMult = 1;
            e.invulnerable = false;
            e.hitF = 90;
            fxEnemyBlink(e.x, e.y, '#ff7a33');
            if (ctx.SFX && ctx.SFX.smolderIgnite) ctx.SFX.smolderIgnite(e.x);
            return true;
        }

        function updateSmolder(dt, dtMs) {
            const G = ctx.G, conf = GC.SMOLDER;
            if (!conf) return;
            if (G.st !== 'PLAYING') {
                // Leaving combat defuses live crash divers: force an immediate,
                // harmless detonation without player damage.
                if (G.st !== 'STAGE_INTRO') for (const e of G.enemies) {
                    if (e.st === 'SMOLDER') { smolderDetonate(e, true); }
                }
                return;
            }
            if (G.chal || G.freezeT > 0) return;
            for (const e of G.enemies) {
                if (e.st !== 'SMOLDER') continue;
                e.smolderT -= dtMs;
                // Steering: fall toward the player with a per-diver lag, plus a
                // gentle downward drift so it reads as a crash, not a chase.
                const dx = G.p.x - e.x, dy = G.p.y - e.y;
                const dist = Math.hypot(dx, dy) || 1;
                const lag = 0.75 + e.smolderSeed * 0.5;
                e.smolderVx += (dx / dist) * conf.fallBias * 620 * lag * dt;
                e.smolderVy += (dy / dist) * conf.fallBias * 620 * lag * dt + conf.baseFall * dt;
                // Cap the tumble speed so divers stay dodgeable.
                const spd = Math.hypot(e.smolderVx, e.smolderVy);
                const maxSpd = 360;
                if (spd > maxSpd) { e.smolderVx *= maxSpd / spd; e.smolderVy *= maxSpd / spd; }
                e.x += e.smolderVx * dt; e.y += e.smolderVy * dt;
                e.smolderRot += e.smolderSpin * dt;
                e.hitF = 90;
                e.attackAnim = 1000;
                // Smoke/ember trail (visual, capped like every other effect).
                smolderEmit(e, dtMs);
                // Contact with the player ends the diver early (with damage).
                if (G.p.alive && G.p.inv <= 0 && Math.hypot(e.x - G.p.x, e.y - G.p.y) < (conf.contactRadius + 8)) {
                    smolderDetonate(e, false, true);
                    continue;
                }
                // Fuse end: the seeded delay fixed at ignition; floor escape
                // triggers at the lower bounds.
                if (e.smolderT <= 0 || e.y > ctx.H + 30) smolderDetonate(e, false);
            }
        }

        function smolderEmit(e, dtMs) {
            if (reducedMotion()) return;
            const G = ctx.G;
            const cap = caps().smolder || 4;
            e._smolderAcc = (e._smolderAcc || 0) + dtMs;
            if (e._smolderAcc < 70) return;
            e._smolderAcc = 0;
            const smokeN = Math.max(1, Math.round(cap * (e.smolderSmoke || 1) * 0.5));
            for (let i = 0; i < smokeN; i++) {
                G.part.push(ctx.getParticle({
                    x: e.x + (Math.random() - 0.5) * 8, y: e.y + (Math.random() - 0.5) * 8,
                    vx: -e.smolderVx * 0.15 + (Math.random() - 0.5) * 20,
                    vy: -e.smolderVy * 0.15 - 10 - Math.random() * 20,
                    life: 420 + Math.random() * 280, t: 0,
                    col: '#4a5568', r: 5 + Math.random() * 4, smoke: true
                }));
            }
            const sparkN = Math.max(1, Math.round(cap * (e.smolderSparks || 1) * 0.5));
            for (let i = 0; i < sparkN; i++) {
                G.part.push(ctx.getParticle({
                    x: e.x, y: e.y,
                    vx: (Math.random() - 0.5) * 120, vy: (Math.random() - 0.5) * 120 - 20,
                    life: 130 + Math.random() * 130, t: 0,
                    col: i % 2 ? '#ff7a33' : '#ffd27a', size: 1, spark: true, shape: 'diamond'
                }));
            }
        }

        // Final blast of a crash diver. defused=true when leaving combat (no
        // score, no damage); playerHit=true when it reached the player.
        function smolderDetonate(e, defused, playerHit) {
            const G = ctx.G;
            if (!e || e.st !== 'SMOLDER') return;
            e.st = 'DEAD';
            if (defused) { ctx.boom(e.x, e.y, false, e.type, e.smolderKillVx, e.smolderKillVy); return; }
            ctx.registerKill(e.x, e.y);
            const pts = ctx.PTS[e.type] ? ctx.PTS[e.type][e.smolderIgnitedAir === true ? 1 : 0] : 200;
            ctx.addScore(pts, e.x, e.y, e.type === 'bee' ? '#ffcc00' : e.type === 'butterfly' ? '#ff3366' : '#44cc44');
            ctx.boom(e.x, e.y, false, e.type, e.smolderKillVx, e.smolderKillVy);
            ctx.dropPU(e);
            const creditGain = 1 + (ctx.G.comboMult >= 4 ? 5 : 0);
            const creditMult = ctx.relic_getRelicBonuses ? ctx.relic_getRelicBonuses().creditMult : 1;
            const earned = (ctx.G.creditRemainder || 0) + creditGain * creditMult;
            ctx.G.credits += Math.floor(earned); ctx.G.creditRemainder = earned % 1;
            try { localStorage.setItem('galaxa_credits', String(ctx.G.credits)); } catch (e2) {}
            if (ctx.G.chal) ctx.G.chalHits++;
            if (e.hasCap) ctx.G.p.cap = { x: e.x, y: e.y };
            if (e.type === 'splitter') smolderSplit(e);
            if (e.type === 'carrier') smolderRelease(e);
            ctx.G.killCount++;
            ctx.G.stageKills = (ctx.G.stageKills || 0) + 1;
            if (ctx.G.killCount === 1) ctx.unlockAchievement('first_blood');
            if (ctx.G.activePU && ctx.G.activePU.type === 'chain_lightning') ctx.G._chainLightningTarget = e;
            ctx.G.weaponXP += 1; ctx.grantWeaponLevel(); ctx.checkWeaponEvo();
            if (playerHit) ctx.damagePlayer(1, e.x, e.y, e.type);
            if (ctx.SFX && ctx.SFX.smolderBlast) ctx.SFX.smolderBlast(e.x);
        }

        // Splitter mini-release on a crash blast (mirrors the bullet-kill split).
        function smolderSplit(e) {
            const G = ctx.G;
            const chained = G.stage >= 15 && !(e._chained);
            const splitType = chained ? 'splitter' : 'bee';
            for (let i = 0; i < 2; i++) {
                const sx = e.x + (i === 0 ? -15 : 15), sy = e.y - 10;
                G.enemies.push({ type: splitType, r: 0, col: 0, x: sx, y: sy, fx: sx, fy: sy, hp: 1, maxHp: 1, st: 'DIVING', eTmr: 0, fr: 0, frT: 0, dTmr: 2000, dPath: { ph: 0, amp: 20, vx: (i === 0 ? -40 : 40) }, sTmr: 500, shootPh: 0, hasCap: false, hitF: 0, elite: false, bossPhase: 0, bossPhaseTransition: 0, bossPhaseHP: [0, 0, 0], animFrame: 0, animTimer: 0, animSpeed: 120, animFrames: 8, spawnAnim: 0, spawnDur: 300, rowPhase: ctx.random() * 3, bobAmp: 2, _chained: chained });
            }
        }

        // Carrier bee release on a crash blast (mirrors the bullet-kill release).
        function smolderRelease(e) {
            const G = ctx.G;
            for (let i = 0; i < 3; i++) {
                const ca = (i / 3) * Math.PI * 2;
                G.enemies.push({ type: 'bee', r: 0, col: 0, x: e.x, y: e.y, fx: e.x + Math.cos(ca) * 40, fy: e.y + Math.sin(ca) * 40, hp: 1, maxHp: 1, st: 'ENTER', eTmr: 300 + i * 100, fr: 0, frT: 0, dTmr: 1500, dPath: null, sTmr: 800, shootPh: 0, hasCap: false, hitF: 0, elite: false, bossPhase: 0, bossPhaseTransition: 0, bossPhaseHP: [0, 0, 0], animFrame: 0, animTimer: 0, animSpeed: 120, animFrames: 8, spawnAnim: 0, spawnDur: 300 });
            }
        }
        function drawSmolderTelegraph(c, e, time, col) {
            const remain = Math.max(0, e.smolderT || 0);
            // Blink frequency ramps up as the fuse runs out (visual-only).
            const blink = Math.abs(Math.sin(time * (0.006 + (1 - Math.min(1, remain / 2000)) * 0.02) + e.smolderSeed * 9));
            c.save();
            c.globalCompositeOperation = 'lighter';
            c.globalAlpha = 0.45 + 0.45 * blink;
            c.strokeStyle = col;
            c.lineWidth = 2;
            const r = 20 + 3 * Math.sin(time * 0.01 + e.smolderSeed * 6);
            c.beginPath(); c.arc(e.x, e.y, r, 0, Math.PI * 2); c.stroke();
            // Remaining-fuse arc above the diver (white-hot core dot included).
            c.globalAlpha = 0.9;
            c.strokeStyle = '#ffffff';
            c.lineWidth = 1.6;
            c.beginPath();
            c.arc(e.x, e.y - 20, 7, -Math.PI / 2, -Math.PI / 2 + Math.min(1, remain / 2000) * Math.PI * 2);
            c.stroke();
            c.fillStyle = '#ffdd88';
            c.beginPath(); c.arc(e.x, e.y - 20, 1.8, 0, Math.PI * 2); c.fill();
            c.restore();
            c.globalAlpha = 1;
        }

        // --- Draw world-space enemy FX (inside the camera transform) ---------
        function drawEnemyFxWorld(c) {
            const G = ctx.G;
            const time = G.animTime || 0;
            // 1. Rage auras on enraged divers (additive, gentle pulse)
            if (!reducedMotion()) {
                c.save();
                c.globalCompositeOperation = 'lighter';
                for (let i = 0; i < G.enemies.length; i++) {
                    const e = G.enemies[i];
                    if (e.st === 'DEAD' || !e.rageMode) continue;
                    c.globalAlpha = 0.22 + 0.22 * Math.sin(time * 0.02 + (e.rowPhase || 0));
                    c.strokeStyle = GC.ENEMY_RAGE_COL || '#ff4a2a';
                    c.lineWidth = 2;
                    c.beginPath();
                    c.arc(e.x, e.y, 16 + 4 * Math.sin(time * 0.012), 0, Math.PI * 2);
                    c.stroke();
                }
                c.restore();
            }
            // 2. Smoldering crash divers: spinning sprite + fuse telegraph
            for (let i = 0; i < G.enemies.length; i++) {
                const e = G.enemies[i];
                if (e.st !== 'SMOLDER') continue;
                const sp = ctx.enemySpriteFor ? ctx.enemySpriteFor(e).sp : null;
                c.save();
                c.translate(e.x, e.y);
                // Tumble rotation; low rotation magnitude under reduced motion.
                const tumble = reducedMotion() ? 0.25 * Math.sign(e.smolderSpin || 1) : (e.smolderRot || 0);
                c.rotate(tumble);
                const flicker = 0.9 + 0.1 * Math.sin(time * 0.03 + e.smolderSeed * 12);
                c.globalAlpha = flicker;
                if (sp) ctx.drawAnimation(c, sp.key, 0, 0, sp.time, undefined, e.smolderVariant === 'smoke' ? '#9aa4b2' : '#ff9a55');
                else { c.fillStyle = '#ff7a33'; c.fillRect(-6, -6, 12, 12); }
                c.restore();
                drawSmolderTelegraph(c, e, time, e.smolderVariant === 'smoke' ? '#9aa4b2' : '#ff7a33');
            }
            // 3. Impact pops + feint/shuffle blink rings
            if (marks.length) {
                c.save();
                c.globalCompositeOperation = 'lighter';
                for (let i = 0; i < marks.length; i++) {
                    const m = marks[i];
                    const p = Math.min(1, m.t / m.dur);
                    const a = (1 - p) * 0.8;
                    if (a <= 0.02) continue;
                    if (m.kind === 'pop') {
                        c.globalAlpha = a;
                        c.strokeStyle = m.col;
                        c.lineWidth = 1.5;
                        c.beginPath(); c.arc(m.x, m.y, m.r + p * 10, 0, Math.PI * 2); c.stroke();
                    } else {
                        // Feint / shuffle: iris ring plus a diamond core marker.
                        c.globalAlpha = a * 0.6;
                        c.strokeStyle = '#ffffff';
                        c.beginPath(); c.arc(m.x, m.y, 4 + p * m.r, 0, Math.PI * 2); c.stroke();
                        c.globalAlpha = a;
                        c.strokeStyle = m.col;
                        c.beginPath(); c.arc(m.x, m.y, 4 + p * m.r * 0.7, 0, Math.PI * 2); c.stroke();
                        const r = 3 + (1 - p) * 2;
                        c.fillStyle = m.col;
                        c.beginPath();
                        c.moveTo(m.x, m.y - r); c.lineTo(m.x + r, m.y); c.lineTo(m.x, m.y + r); c.lineTo(m.x - r, m.y);
                        c.closePath(); c.fill();
                    }
                }
                c.restore();
                c.globalAlpha = 1;
            }
            // 3. Kill-chain lightning arcs
            if (chains.length) {
                c.save();
                c.globalCompositeOperation = 'lighter';
                c.strokeStyle = '#bfe6ff';
                c.lineWidth = 1.4;
                for (let i = 0; i < chains.length; i++) {
                    const ch = chains[i];
                    const a = Math.sin(Math.min(1, ch.t / ch.dur) * Math.PI) * 0.85;
                    if (a <= 0.02) continue;
                    const dx = ch.x2 - ch.x1, dy = ch.y2 - ch.y1;
                    const len = Math.hypot(dx, dy) || 1;
                    const nx = dy / len, ny = -dx / len;
                    c.globalAlpha = a;
                    c.beginPath(); c.moveTo(ch.x1, ch.y1);
                    const segs = 5;
                    for (let s = 1; s <= segs; s++) {
                        const t = s / segs;
                        const off = Math.sin(ch.seed + s * 2.7) * 11 * (1 - Math.abs(t - 0.5) * 2);
                        c.lineTo(ch.x1 + dx * t + nx * off, ch.y1 + dy * t + ny * off);
                    }
                    c.stroke();
                }
                c.restore();
                c.globalAlpha = 1;
            }
        }

        ctx.fxEnemyHit = fxEnemyHit;
        ctx.fxEnemyBlink = fxEnemyBlink;
        ctx.fxEnemyFeint = fxEnemyFeint;
        ctx.fxKillChain = fxKillChain;
        ctx.fxSquadDiveCue = fxSquadDiveCue;
        ctx.fxFormationShuffle = fxFormationShuffle;
        ctx.maybeSmolder = maybeSmolder;
        ctx.igniteSmolder = igniteSmolder;
        ctx.smolderDetonate = smolderDetonate;
        ctx.updateEnemyFx = updateEnemyFx;
        ctx.drawEnemyFxWorld = drawEnemyFxWorld;
    };
})();
