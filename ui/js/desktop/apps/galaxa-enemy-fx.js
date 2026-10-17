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

        // --- Armor / shield metallic ricochet glint --------------------------
        function fxArmorRicochet(x, y, dirX, dirY) {
            if (x == null || y == null) return;
            addMark({ kind: 'ricochet', x, y, t: 0, dur: 220, col: '#88eeff', r: 13 });
            if (ctx.SFX && ctx.SFX.armorRicochet) ctx.SFX.armorRicochet(x);
            if (ctx.settings.particles === 'low' || reducedMotion()) return;
            const baseA = Math.atan2(-(dirY || -1), -(dirX || 0));
            const n = ctx.settings.particles === 'medium' ? 3 : 5;
            for (let i = 0; i < n; i++) {
                const a = baseA + (i % 2 === 0 ? 1 : -1) * (0.45 + Math.random() * 0.65);
                const sp = 95 + Math.random() * 110;
                ctx.G.part.push(ctx.getParticle({
                    x, y, vx: Math.cos(a) * sp, vy: Math.sin(a) * sp,
                    life: 100 + Math.random() * 90, t: 0,
                    col: i % 2 === 0 ? '#ffffff' : '#77e8ff',
                    size: 1.4, spark: true
                }));
            }
        }

        // --- Dive launch thruster ignition flare -----------------------------
        function fxEnemyDiveStart(x, y, col) {
            if (x == null || y == null) return;
            addMark({ kind: 'diveFlare', x, y: y - 4, t: 0, dur: 240, col: col || '#ff9944', r: 15 });
            if (ctx.SFX && ctx.SFX.diveIgnite) ctx.SFX.diveIgnite(x);
            if (ctx.settings.particles === 'low' || reducedMotion()) return;
            const n = ctx.settings.particles === 'medium' ? 2 : 4;
            for (let i = 0; i < n; i++) {
                const a = -Math.PI / 2 + (Math.random() - 0.5) * 0.9;
                const sp = 40 + Math.random() * 55;
                ctx.G.part.push(ctx.getParticle({
                    x: x + (i % 2 ? 4 : -4), y: y - 4,
                    vx: Math.cos(a) * sp, vy: Math.sin(a) * sp,
                    life: 130 + Math.random() * 90, t: 0,
                    col: i % 2 ? '#ffffff' : (col || '#ffaa44'),
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
        function maybeSmolder(e, b) {
            const c = GC.SMOLDER || {};
            if (!e || e.st === 'DEAD' || e.st === 'SMOLDER' || e.st === 'ENTER') return false;
            if (ctx.G.chal || ctx.G.st !== 'PLAYING' || ctx.G.freezeT > 0) return false;
            if (e.sectorBoss || e.type === 'boss' || e.type === 'miniboss') return false;
            if (ctx.G.stage < (c.stageMin || 1)) return false;
            let active = 0;
            for (const o of ctx.G.enemies) { if (o.st === 'SMOLDER' && ++active >= (c.maxSimultaneous || 4)) return false; }
            const chance = ctx.settings.diff === 'easy'
                ? (c.easyChance != null ? c.easyChance : 0.18)
                : (ctx.settings.diff === 'hard' ? c.chance + c.hardBonus : c.chance);
            if (ctx.random() >= chance) return false;
            const vs = c.variants && c.variants.length ? c.variants : [{ id: 'ember', smoke: 1, sparks: 1 }];
            const v = vs[Math.floor(ctx.random() * vs.length) % vs.length];
            const minSpin = c.minSpin != null ? c.minSpin : 0.65;
            const maxSpin = c.maxSpin != null ? c.maxSpin : 2.35;
            const spinDir = ctx.random() < 0.5 ? -1 : 1;
            const baseSpin = (minSpin + ctx.random() * Math.max(0.2, maxSpin - minSpin)) * (v.spinMult || 1);
            const minMs = v.minMs != null ? v.minMs : (c.minMs || 650);
            const maxMs = v.maxMs != null ? v.maxMs : (c.maxMs || 1450);
            const blowMs = minMs + ctx.random() * Math.max(100, maxMs - minMs);
            const a = Math.atan2(ctx.G.p.y - e.y, ctx.G.p.x - e.x);
            const s = 95 + ctx.random() * 75;
            let vx = Math.cos(a) * s + (ctx.random() - 0.5) * 45;
            let vy = Math.sin(a) * s * 0.45 - 25 + ctx.random() * 50;
            if (v.id === 'glide') {
                vx = spinDir * (85 + ctx.random() * 55) + ((b && b.vx) || 0) * 0.15;
                vy = 30 + ctx.random() * 45;
            } else if (v.id === 'spiral') {
                vx = (ctx.random() - 0.5) * 65;
                vy = 55 + ctx.random() * 45;
            } else if (v.id === 'sputter') {
                vx = (ctx.random() - 0.5) * 80;
                vy = -15 + ctx.random() * 40;
            }
            return igniteSmolder(e, {
                vx, vy,
                spin: spinDir * baseSpin,
                blowMs,
                variant: v.id,
                smoke: v.smoke,
                sparks: v.sparks,
                swayAmp: v.swayAmp || 24,
                swayFreq: v.swayFreq || 3.2,
                fallMult: v.fallMult || 1,
                steerMult: v.steerMult != null ? v.steerMult : 0.5,
                sputterMs: v.sputterMs || 0,
                tint: v.tint,
                ringCol: v.ringCol,
                killVx: (b && b.vx) || 0,
                killVy: (b && b.vy) || -ctx.PB_SPEED,
                ignitedAir: e.st === 'DIVING' || e.st === 'FEINT'
            });
        }

        function smolderIgniteBurst(e) {
            if (reducedMotion() || ctx.settings.particles === 'low') return;
            const G = ctx.G;
            for (let i = 0; i < 2; i++) {
                G.part.push(ctx.getParticle({
                    x: e.x + (Math.random() - 0.5) * 8, y: e.y + (Math.random() - 0.5) * 8,
                    vx: (Math.random() - 0.5) * 35, vy: -15 - Math.random() * 25,
                    life: 360 + Math.random() * 200, t: 0,
                    col: '#3b4454', r: 5 + Math.random() * 3, smoke: true
                }));
            }
            for (let i = 0; i < 3; i++) {
                const a = Math.random() * Math.PI * 2;
                const sp = 35 + Math.random() * 65;
                G.part.push(ctx.getParticle({
                    x: e.x, y: e.y,
                    vx: Math.cos(a) * sp, vy: Math.sin(a) * sp,
                    life: 260 + Math.random() * 180, t: 0,
                    col: i === 0 ? '#d2e5ed' : (e.smolderRingCol || '#ff9a44'),
                    size: 2, debris: true, rot: Math.random() * 6.28
                }));
            }
        }

        function smolderSputterBurst(e) {
            addMark({ kind: 'pop', x: e.x, y: e.y, t: 0, dur: 170, col: e.smolderVariant === 'sputter' ? '#88eeff' : '#ffaa44', r: 12 });
            if (ctx.SFX && ctx.SFX.smolderSputter) ctx.SFX.smolderSputter(e.x);
            if (reducedMotion() || ctx.settings.particles === 'low') return;
            const G = ctx.G;
            const elec = e.smolderVariant === 'sputter';
            for (let i = 0; i < 3; i++) {
                const a = Math.random() * Math.PI * 2;
                const sp = 55 + Math.random() * 80;
                G.part.push(ctx.getParticle({
                    x: e.x, y: e.y,
                    vx: Math.cos(a) * sp, vy: Math.sin(a) * sp,
                    life: 120 + Math.random() * 110, t: 0,
                    col: elec ? (i % 2 ? '#77f0ff' : '#ffee77') : (i % 2 ? '#ff9944' : '#ffffff'),
                    size: 1.2, spark: true, shape: 'diamond'
                }));
            }
            G.part.push(ctx.getParticle({
                x: e.x, y: e.y,
                vx: (Math.random() - 0.5) * 65, vy: -20 + (Math.random() - 0.5) * 50,
                life: 300 + Math.random() * 160, t: 0,
                col: '#9aa4b2', size: 2, debris: true, rot: Math.random() * 6.28
            }));
        }

        function igniteSmolder(e, prep) {
            if (!e || e.st === 'DEAD') return false;
            const conf = GC.SMOLDER;
            if (!conf) return false;
            e.st = 'SMOLDER';
            e.smolderT = prep && prep.blowMs != null ? prep.blowMs : conf.minMs;
            e.smolderDur = e.smolderT;
            e.smolderAge = 0;
            const _pv = prep && prep.vx, _pw = prep && prep.vy;
            e.smolderVx = _pv || 0;
            e.smolderVy = _pw || 0;
            e.smolderSpin = (prep && prep.spin) || 1.2;
            e.smolderRot = 0;                       // seeded by spin only
            e.smolderSeed = ctx.random();           // wobble/flash variants
            e.smolderVariant = (prep && prep.variant) || 'ember';
            e.smolderSmoke = (prep && prep.smoke) || 1;
            e.smolderSparks = (prep && prep.sparks) || 1;
            e.smolderSwayAmp = (prep && prep.swayAmp) || 24;
            e.smolderSwayFreq = (prep && prep.swayFreq) || 3.2;
            e.smolderFallMult = (prep && prep.fallMult) || 1;
            e.smolderSteerMult = prep && prep.steerMult != null ? prep.steerMult : 0.5;
            e.smolderSputterMs = (prep && prep.sputterMs) || 0;
            e.smolderPops = 0;
            e.smolderNextPop = e.smolderSputterMs > 0 ? e.smolderSputterMs : e.smolderDur * 0.52;
            e.smolderTint = (prep && prep.tint) || (e.smolderVariant === 'glide' || e.smolderVariant === 'smoke' ? '#9aa4b2' : e.smolderVariant === 'sputter' ? '#99e6ff' : '#ff9a55');
            e.smolderRingCol = (prep && prep.ringCol) || (e.smolderVariant === 'sputter' ? '#66d4ff' : '#ff7a33');
            e.smolderKillVx = (prep && prep.killVx) || 0;
            e.smolderKillVy = (prep && prep.killVy) || 0;
            e.smolderIgnitedAir = !!(prep && prep.ignitedAir);
            e.rot = 0; e.rotTarget = 0; e.rotTimer = 0; e.rotDuration = 0;
            e.divePending = false; e.shotPending = false; e.shotReady = false;
            e.rageMode = 0; e.rageSpeedMult = 1;
            e.invulnerable = false;
            e.hitF = 90;
            fxEnemyBlink(e.x, e.y, e.smolderRingCol);
            smolderIgniteBurst(e);
            if (ctx.SFX && ctx.SFX.smolderIgnite) ctx.SFX.smolderIgnite(e.x, e.smolderVariant);
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
                e.smolderAge = (e.smolderAge || 0) + dtMs;
                const ageSec = e.smolderAge * 0.001;
                // Steering & gravity varied by crash-dive archetype.
                const dx = G.p.x - e.x, dy = G.p.y - e.y;
                const dist = Math.hypot(dx, dy) || 1;
                const lag = (0.75 + e.smolderSeed * 0.5) * (e.smolderSteerMult != null ? e.smolderSteerMult : 0.6);
                e.smolderVx += (dx / dist) * conf.fallBias * 580 * lag * dt;
                e.smolderVy += (dy / dist) * conf.fallBias * 460 * lag * dt + conf.baseFall * (e.smolderFallMult || 1) * dt;
                // Sinusoidal lateral sway (strong corkscrew for 'spiral', gentle drift for others).
                const swayVx = Math.cos(ageSec * (e.smolderSwayFreq || 3.2) + e.smolderSeed * 6.28) * (e.smolderSwayAmp || 20);
                // Internal sputter sub-explosions jolt velocity and rotation mid-fall.
                if (e.smolderNextPop > 0 && e.smolderAge >= e.smolderNextPop && e.smolderT > 140) {
                    e.smolderPops = (e.smolderPops || 0) + 1;
                    e.smolderNextPop = e.smolderSputterMs > 0 ? e.smolderAge + e.smolderSputterMs : 0;
                    const joltDir = ((e.smolderPops + Math.floor(e.smolderSeed * 7)) % 2 === 0) ? 1 : -1;
                    e.smolderVx += joltDir * 42;
                    e.smolderVy -= 18;
                    e.smolderRot += joltDir * 0.22;
                    smolderSputterBurst(e);
                }
                // Cap the tumble speed so divers stay readable and dodgeable.
                const spd = Math.hypot(e.smolderVx, e.smolderVy);
                const maxSpd = 320;
                if (spd > maxSpd) { e.smolderVx *= maxSpd / spd; e.smolderVy *= maxSpd / spd; }
                e.x += (e.smolderVx + swayVx) * dt;
                e.y += e.smolderVy * dt;
                e.smolderRot += e.smolderSpin * dt;
                e.hitF = e.smolderT < 180 ? 120 : 90;
                e.attackAnim = 1000;
                // Smoke/ember/spark trail (visual, capped like every other effect).
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
            if (e._smolderAcc < 58) return;
            e._smolderAcc = 0;
            const variant = e.smolderVariant || 'ember';
            const smokeCols = ['#363f4d', '#4a5568', '#64748b'];
            const smokeN = Math.max(1, Math.round(cap * (e.smolderSmoke || 1) * 0.55));
            for (let i = 0; i < smokeN; i++) {
                G.part.push(ctx.getParticle({
                    x: e.x + (Math.random() - 0.5) * 10,
                    y: e.y + (Math.random() - 0.5) * 8,
                    vx: -e.smolderVx * 0.16 + (Math.random() - 0.5) * 24,
                    vy: -e.smolderVy * 0.14 - 14 - Math.random() * 22,
                    life: 440 + Math.random() * 320, t: 0,
                    col: smokeCols[i % smokeCols.length],
                    r: (variant === 'glide' ? 6 : 4.5) + Math.random() * 4.5,
                    smoke: true
                }));
            }
            // Glowing core flame puff right behind the hull
            if (ctx.settings.particles !== 'low') {
                G.part.push(ctx.getParticle({
                    x: e.x + (Math.random() - 0.5) * 5,
                    y: e.y + (Math.random() - 0.5) * 5,
                    vx: -e.smolderVx * 0.08 + (Math.random() - 0.5) * 14,
                    vy: -e.smolderVy * 0.08 - 8,
                    life: 160 + Math.random() * 110, t: 0,
                    col: variant === 'sputter' ? '#77eeff' : '#ffaa33',
                    r: 3 + Math.random() * 2,
                    spark: true, fireTrail: true, shape: 'circle'
                }));
            }
            const sparkN = Math.max(1, Math.round(cap * (e.smolderSparks || 1) * 0.55));
            for (let i = 0; i < sparkN; i++) {
                const sparkCol = variant === 'sputter'
                    ? (i % 2 ? '#66eeff' : '#ffee77')
                    : (variant === 'glide' ? (i % 2 ? '#ff9944' : '#ffcc66') : (i % 2 ? '#ff7a33' : '#ffd27a'));
                G.part.push(ctx.getParticle({
                    x: e.x, y: e.y,
                    vx: (Math.random() - 0.5) * 125 - e.smolderVx * 0.1,
                    vy: (Math.random() - 0.5) * 125 - 18,
                    life: 140 + Math.random() * 150, t: 0,
                    col: sparkCol, size: 1 + (i % 2) * 0.5, spark: true, shape: i % 2 ? 'diamond' : 'rect'
                }));
            }
        }

        // Final blast of a crash diver. defused=true when leaving combat (no
        // score, no damage); playerHit=true when it reached the player.
        function smolderDetonate(e, defused, playerHit) {
            const G = ctx.G;
            if (!e || e.st !== 'SMOLDER') return;
            if (defused) { e.st = 'DEAD'; ctx.boom(e.x, e.y, false, e.type, e.smolderKillVx, e.smolderKillVy); return; }
            e.st = e.smolderIgnitedAir ? 'DIVING' : 'FORM'; e.hp = 0;
            ctx.finishEnemyKill(e, { noSmolder: true, x: e.x, y: e.y, vx: e.smolderVx || e.smolderKillVx, vy: e.smolderVy || e.smolderKillVy });
            addMark({ kind: 'crashShock', x: e.x, y: e.y, t: 0, dur: 360, col: e.smolderRingCol || '#ff7a33', r: 38 });
            if (playerHit) ctx.killP();
            if (ctx.SFX && ctx.SFX.smolderBlast) ctx.SFX.smolderBlast(e.x, e.smolderVariant);
        }

        function drawSmolderTelegraph(c, e, time, col) {
            const remain = Math.max(0, e.smolderT || 0);
            const dur = Math.max(200, e.smolderDur || 1200);
            const frac = Math.min(1, remain / dur);
            // Blink frequency ramps up as the fuse runs out (visual-only).
            const blink = Math.abs(Math.sin(time * (0.006 + (1 - frac) * 0.024) + e.smolderSeed * 9));
            c.save();
            c.globalCompositeOperation = 'lighter';
            // Critical core flare in the final 180ms right before detonation.
            if (remain < 180 && !reducedMotion()) {
                const critP = 1 - remain / 180;
                c.globalAlpha = 0.55 + critP * 0.4;
                c.strokeStyle = '#ffffff';
                c.lineWidth = 2.4;
                c.beginPath(); c.arc(e.x, e.y, Math.max(4, (1 - critP) * 26), 0, Math.PI * 2); c.stroke();
                c.fillStyle = '#fff5cc';
                c.globalAlpha = 0.45 * critP;
                c.beginPath(); c.arc(e.x, e.y, 7 + critP * 6, 0, Math.PI * 2); c.fill();
            }
            c.globalAlpha = 0.42 + 0.46 * blink;
            c.strokeStyle = col;
            c.lineWidth = 1.8;
            const r = 19 + 3 * Math.sin(time * 0.01 + e.smolderSeed * 6);
            c.beginPath(); c.arc(e.x, e.y, r, 0, Math.PI * 2); c.stroke();
            // Remaining-fuse arc above the diver (white-hot core dot included).
            c.globalAlpha = 0.88;
            c.strokeStyle = '#ffffff';
            c.lineWidth = 1.6;
            c.beginPath();
            c.arc(e.x, e.y - 20, 7, -Math.PI / 2, -Math.PI / 2 + frac * Math.PI * 2);
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
            // 2. Smoldering crash divers: gently rotating/banking sprite + fire glow + fuse telegraph
            for (let i = 0; i < G.enemies.length; i++) {
                const e = G.enemies[i];
                if (e.st !== 'SMOLDER') continue;
                const sp = ctx.enemySpriteFor ? ctx.enemySpriteFor(e).sp : null;
                const tintCol = e.smolderTint || (e.smolderVariant === 'smoke' || e.smolderVariant === 'glide' ? '#9aa4b2' : '#ff9a55');
                const ringCol = e.smolderRingCol || (e.smolderVariant === 'smoke' ? '#9aa4b2' : '#ff7a33');
                c.save();
                c.translate(e.x, e.y);
                // Gentle rotation + pendulum bank wobble; low magnitude under reduced motion.
                const wobble = Math.sin(time * 0.009 + (e.smolderSeed || 0) * 10) * 0.18;
                const tumble = reducedMotion() ? 0.25 * Math.sign(e.smolderSpin || 1) : ((e.smolderRot || 0) + wobble);
                c.rotate(tumble);
                if (!reducedMotion()) {
                    c.save();
                    c.globalCompositeOperation = 'lighter';
                    c.globalAlpha = 0.32 + 0.18 * Math.sin(time * 0.04 + (e.smolderSeed || 0) * 8);
                    c.fillStyle = ringCol;
                    c.beginPath(); c.arc(0, 0, 11, 0, Math.PI * 2); c.fill();
                    c.restore();
                }
                const flicker = 0.88 + 0.12 * Math.sin(time * 0.03 + (e.smolderSeed || 0) * 12);
                c.globalAlpha = flicker;
                const flashTint = (e.smolderT || 999) < 180 && Math.floor(time / 40) % 2 === 0 ? '#ffffff' : tintCol;
                if (sp) ctx.drawAnimation(c, sp.key, 0, 0, sp.time, undefined, flashTint);
                else { c.fillStyle = '#ff7a33'; c.fillRect(-6, -6, 12, 12); }
                c.restore();
                drawSmolderTelegraph(c, e, time, ringCol);
            }
            // 3. Impact pops, ricochets, dive flares, crash shocks + feint/shuffle blink rings
            if (marks.length) {
                c.save();
                c.globalCompositeOperation = 'lighter';
                for (let i = 0; i < marks.length; i++) {
                    const m = marks[i];
                    const p = Math.min(1, m.t / m.dur);
                    const a = (1 - p) * 0.82;
                    if (a <= 0.02) continue;
                    if (m.kind === 'pop') {
                        c.globalAlpha = a;
                        c.strokeStyle = m.col;
                        c.lineWidth = 1.5;
                        c.beginPath(); c.arc(m.x, m.y, m.r + p * 10, 0, Math.PI * 2); c.stroke();
                    } else if (m.kind === 'ricochet') {
                        c.globalAlpha = a;
                        c.strokeStyle = '#ffffff';
                        c.lineWidth = 2;
                        c.beginPath(); c.arc(m.x, m.y, 4 + p * m.r, -Math.PI * 0.85, -Math.PI * 0.15); c.stroke();
                        c.strokeStyle = m.col;
                        c.lineWidth = 1.4;
                        c.beginPath(); c.arc(m.x, m.y, 2 + p * (m.r + 5), 0, Math.PI * 2); c.stroke();
                    } else if (m.kind === 'diveFlare') {
                        c.globalAlpha = a * 0.85;
                        c.fillStyle = m.col;
                        const rx = (1 - p * 0.5) * m.r, ry = (1 - p * 0.7) * 4;
                        c.fillRect(m.x - rx, m.y - 1, rx * 2, 2);
                        c.fillStyle = '#ffffff';
                        c.fillRect(m.x - rx * 0.5, m.y - ry, rx, ry * 2);
                    } else if (m.kind === 'crashShock') {
                        c.globalAlpha = a * 0.9;
                        c.strokeStyle = m.col;
                        c.lineWidth = 2.6 * (1 - p * 0.5);
                        c.beginPath(); c.arc(m.x, m.y, 6 + p * m.r, 0, Math.PI * 2); c.stroke();
                        c.globalAlpha = a * 0.55;
                        c.strokeStyle = '#ffffff';
                        c.lineWidth = 1.2;
                        c.beginPath(); c.arc(m.x, m.y, 4 + p * m.r * 0.78, 0, Math.PI * 2); c.stroke();
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
            // 4. Kill-chain lightning arcs
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
        ctx.fxArmorRicochet = fxArmorRicochet;
        ctx.fxEnemyDiveStart = fxEnemyDiveStart;
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
