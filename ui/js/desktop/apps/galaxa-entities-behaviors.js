(function () {
    'use strict';
    const GC = window.GalaxaCore = window.GalaxaCore || {};

    GC.createEntitiesBehaviors = function (ctx) {
                function enemyFire(e) {
            if (ctx.G.chal || ctx.G.st !== 'PLAYING') return;
            if (!e.shotReady) {
                if (e.shotPending) return;
                e.shotPending = true; e.attackAnim = 1000;
                e.shotWarning = ctx.settings.diff === 'hard' ? 300 : 500;
                ctx.scheduleGame(() => {
                    e.shotPending = false; e.shotWarning = 0;
                    if (e.st !== 'DEAD' && e.st !== 'SMOLDER' && ctx.G.freezeT <= 0) { e.shotReady = true; enemyFire(e); e.shotReady = false; }
                }, e.shotWarning);
                return;
            }
            if (ctx.G.ebul.length >= 180) return;
            const _ebStart = ctx.G.ebul.length;
            const spd = ctx.EB_SPEED * ctx.diffMod('ebSpd');
            // Quirk "fakeout": the tell is longer and the shot leaves from a
            // laterally jittered muzzle, so the aim is harder to read.
            const _fake = e.quirk === 'fakeout' ? (ctx.random() - 0.5) * 36 : 0;
            const px = e.x + _fake, py = e.y + 8;

            function mirrorBullet(b) {
                if (!ctx.modesIsMirrorPermanent || !ctx.modesIsMirrorPermanent()) return;
                b.x = ctx.W - b.x;
                if (b.vx) b.vx = -b.vx;
            }

            function pushEbul(b) {
                mirrorBullet(b);
                ctx.G.ebul.push(b);
            }
            const variant = (stageMin, chance) => ctx.G.stage >= stageMin && ctx.random() < chance;
            const P = ctx.ATTACK_PATTERNS, player = ctx.G.p;

            // NEW: Boss phase-based attack patterns
            if ((e.type === 'boss' || e.type === 'miniboss') && e.bossPhase) {
                const ebSpd = spd;
                switch (e.bossPhase) {
                    case 1:
                        ctx.G.ebul.push({ x: px, y: py, w: 2, h: 6 });
                        if (ctx.G.stage >= 5) { ctx.G.ebul.push({ x: px - 8, y: py, w: 2, h: 6 }); ctx.G.ebul.push({ x: px + 8, y: py, w: 2, h: 6 }); }
                        break;
                    case 2:
                        ctx.G.ebul.push(...ctx.ATTACK_PATTERNS.aimed_burst(e, 5, 0.55, 0, ebSpd, ctx.G.p.x, ctx.G.p.y));
                        ctx.G.ebul.push(...ctx.ATTACK_PATTERNS.random_spread(e, 4, 0.4, 0.8, ebSpd, ctx.random));
                        if (ctx.random() < 0.5) ctx.G.ebul.push(...P.fan_sweep(e, 7, 0.4, 2.0, ebSpd));
                        ctx.SFX.hunterShot(e.x);
                        break;
                    case 3:
                        ctx.G.ebul.push(...ctx.ATTACK_PATTERNS.spiral(e, 12, 0.35, 0, ebSpd));
                        ctx.G.ebul.push(...ctx.ATTACK_PATTERNS.circle(e, 8, 0.3, ebSpd));
                        if (ctx.random() < 0.5) ctx.G.ebul.push(...ctx.ATTACK_PATTERNS.wall(e, 3, 5, 0.25, ebSpd));
                        if (ctx.random() < 0.5) ctx.G.ebul.push(...P.rain(e, 8, 0.38, 180, ebSpd, ctx.random));
                        ctx.SFX.spinnerShot(e.x);
                        ctx.G.shkT = Math.max(ctx.G.shkT, 200); ctx.G.shkM = Math.max(ctx.G.shkM, 3);
                        break;
                }
            } else switch (e.type) {
                case 'hunter':
                    if (!ctx.G.p.alive) break;
                    if (variant(4, 0.4)) { ctx.G.ebul.push(...P.predictive(e, 3, 0.68, spd, player)); ctx.SFX.hunterShot(e.x); break; }
                    ctx.SFX.hunterShot(e.x);
                    { const dx = ctx.G.p.x - px, dy = ctx.G.p.y - py, dist = Math.hypot(dx, dy) || 1, baseA = Math.atan2(dy, dx);
                      for (let i = -2; i <= 2; i++) { const a = baseA + i * 0.22; ctx.G.ebul.push({ x: px, y: py, w: 2, h: 5, vx: Math.cos(a) * spd * 0.62, vy: Math.sin(a) * spd * 0.62, kind: 'hunter' }); } }
                    break;
                case 'spinner':
                    if (variant(3, 0.45)) { ctx.SFX.spinnerShot(e.x); ctx.G.ebul.push(...P.fan_sweep(e, 7, 0.42, 1.9, spd)); break; }
                    ctx.SFX.spinnerShot(e.x);
                    e.shootPh = (e.shootPh || 0) + Math.PI / 3;
                    for (let i = 0; i < 8; i++) { const a = e.shootPh + i * Math.PI / 4; ctx.G.ebul.push({ x: px, y: py, w: 2, h: 4, vx: Math.cos(a) * spd * 0.44, vy: Math.sin(a) * spd * 0.44, kind: 'spiral' }); }
                    break;
                case 'bomber':
                    if (variant(5, 0.4)) { if (ctx.SFX.lobShot) ctx.SFX.lobShot(e.x); ctx.G.ebul.push(...P.lob(e, 3, 0.5, spd, ctx.random)); break; }
                    ctx.SFX.bomberDrop(e.x);
                    for (let i = -1; i <= 1; i++) ctx.G.ebul.push({ x: px + i * 10, y: py, w: 3, h: 3, vx: i * 38, vy: spd * 0.22, kind: 'mine', fuse: 2200, t: 0 });
                    break;
                case 'lasher':
                    if (!ctx.G.p.alive) break;
                    if (variant(4, 0.4)) { ctx.SFX.lasherShot(e.x); ctx.G.ebul.push(...P.pincer(e, 0.5, spd, player.x, 'plasma')); break; }
                    ctx.SFX.lasherShot(e.x);
                    { const dx = ctx.G.p.x - px, dy = ctx.G.p.y - py, dist = Math.hypot(dx, dy) || 1;
                      ctx.G.ebul.push({ x: px, y: py, w: 4, h: 10, vx: (dx / dist) * spd * 0.52, vy: (dy / dist) * spd * 0.52, kind: 'plasma' });
                      ctx.G.ebul.push({ x: px - 6, y: py, w: 3, h: 7, vx: (dx / dist) * spd * 0.4 + 28, vy: (dy / dist) * spd * 0.4, kind: 'plasma' });
                      ctx.G.ebul.push({ x: px + 6, y: py, w: 3, h: 7, vx: (dx / dist) * spd * 0.4 - 28, vy: (dy / dist) * spd * 0.4, kind: 'plasma' }); }
                    break;
                // NEW: Enemy type firing patterns
                case 'weaver':
                    if (!ctx.G.p.alive) break;
                    if (variant(3, 0.5)) { if (ctx.SFX.pincerShot) ctx.SFX.pincerShot(e.x); ctx.G.ebul.push(...P.pincer(e, 0.45, spd, player.x)); break; }
                    { const dx = ctx.G.p.x - px, dy = ctx.G.p.y - py, dist = Math.hypot(dx, dy) || 1;
                      ctx.G.ebul.push({ x: px, y: py, w: 2, h: 5, vx: (dx / dist) * spd * 0.45, vy: (dy / dist) * spd * 0.45, kind: 'hunter' }); }
                    break;
                case 'splitter':
                    if (variant(4, 0.5)) { ctx.G.ebul.push(...P.rain(e, 6, 0.42, 110, spd, ctx.random)); break; }
                    ctx.G.ebul.push(...ctx.ATTACK_PATTERNS.random_spread(e, 5, 0.35, 0.6, spd, ctx.random));
                    break;
                case 'shield_bee':
                    ctx.G.ebul.push({ x: px, y: py, w: 2, h: 6 });
                    break;
                case 'kamikaze':
                    // Kamikaze doesn't shoot — it charges
                    break;
                case 'carrier':
                    if (variant(4, 0.5)) { if (ctx.SFX.lobShot) ctx.SFX.lobShot(e.x); ctx.G.ebul.push(...P.lob(e, 4, 0.45, spd, ctx.random)); break; }
                    ctx.G.ebul.push(...ctx.ATTACK_PATTERNS.aimed_burst(e, 3, 0.4, 0, spd, ctx.G.p.x, ctx.G.p.y));
                    break;
                case 'teleporter':
                    if (variant(3, 0.5)) { ctx.G.ebul.push(...P.fan_sweep(e, 5, 0.34, 1.4, spd)); break; }
                    ctx.G.ebul.push(...ctx.ATTACK_PATTERNS.circle(e, 6, 0.3, spd));
                    break;
                case 'stalker':
                    if (variant(4, 0.4) && player.alive) { ctx.G.ebul.push(...P.predictive(e, 1, 0.58, spd, player)); break; }
                    ctx.G.ebul.push({ x: px, y: py, w: 2, h: 6 });
                    ctx.G.ebul.push({ x: px - 6, y: py - 2, w: 2, h: 6 });
                    ctx.G.ebul.push({ x: px + 6, y: py - 2, w: 2, h: 6 });
                    break;
                case 'sniper':
                    if (ctx.G.p.alive) {
                        if (variant(6, 0.5)) { ctx.G.ebul.push(...P.predictive(e, 1, 0.62, spd, player)); ctx.SFX.sniperShot(e.x); break; }
                        const dx = ctx.G.p.x - px, dy = ctx.G.p.y - py, dist = Math.hypot(dx, dy);
                        if (dist > 1) { ctx.G.ebul.push({ x: px, y: py, w: 2, h: 6, vx: (dx / dist) * spd * 0.6, vy: (dy / dist) * spd * 0.6, kind: 'sniper' }); ctx.SFX.sniperShot(e.x); }
                    }
                    break;
                default:
                    if (e.type === 'bee') {
                        // Quirk "volley": an otherwise harmless bee returns fire once.
                        if (e.quirk === 'volley' && ctx.G.p.alive) {
                            const dx = ctx.G.p.x - px, dy = ctx.G.p.y - py, dist = Math.hypot(dx, dy) || 1;
                            ctx.G.ebul.push({ x: px, y: py, w: 2, h: 6, vx: (dx / dist) * spd * 0.5, vy: (dy / dist) * spd * 0.5, kind: 'hunter' });
                            if (ctx.SFX.hunterShot) ctx.SFX.hunterShot(e.x);
                        }
                        break;
                    }
                    if (e.type === 'butterfly' && player.alive && variant(3, 0.3)) { if (ctx.SFX.pincerShot) ctx.SFX.pincerShot(e.x); ctx.G.ebul.push(...P.pincer(e, 0.4, spd, player.x)); break; }
                    ctx.G.ebul.push({ x: px, y: py, w: 2, h: 6 });
                    if (ctx.G.stage >= 5 && e.type === 'boss') { ctx.G.ebul.push({ x: px - 8, y: py, w: 2, h: 6 }); ctx.G.ebul.push({ x: px + 8, y: py, w: 2, h: 6 }); }
                    if (ctx.G.stage >= 8 && e.type === 'boss') { for (let k = 0; k < 3; k++) ctx.scheduleGame(() => { if (!ctx.state.disposed && e.st === 'DIVING') pushEbul({ x: e.x, y: e.y + 8, w: 2, h: 6 }); }, k * 150); }
                    if (e.type === 'miniboss') { ctx.G.ebul.push({ x: px - 10, y: py, w: 2, h: 6 }); ctx.G.ebul.push({ x: px + 10, y: py, w: 2, h: 6 }); for (let k = 0; k < 2; k++) ctx.scheduleGame(() => { if (!ctx.state.disposed && e.st === 'DIVING') { pushEbul({ x: e.x - 6, y: e.y + 8, w: 2, h: 6 }); pushEbul({ x: e.x + 6, y: e.y + 8, w: 2, h: 6 }); } }, k * 180); }
            }
            if (ctx.modesIsMirrorPermanent && ctx.modesIsMirrorPermanent()) {
                for (let _mi = _ebStart; _mi < ctx.G.ebul.length; _mi++) {
                    mirrorBullet(ctx.G.ebul[_mi]);
                }
            }
        }
        function diveRateMult(e) {
            if (e.type === 'hunter') return 5;
            if (e.type === 'stalker') return 3;
            if (e.type === 'kamikaze') return 4;
            return 1;
        }
        function isHomingType(e) { return e.type === 'hunter' || e.type === 'stalker' || e.type === 'kamikaze'; }

        // Seeded weighted pick of a dive path shape; later stages unlock wilder
        // shapes so early waves stay readable.
        function pickDivePath(e) {
            const defs = GC.DIVE_PATHS;
            if (!defs) return 'sine';
            let total = 0;
            const pool = [];
            for (const k in defs) { if (ctx.G.stage >= (defs[k].stageMin || 1)) { pool.push(k); total += defs[k].weight || 1; } }
            if (!pool.length) return 'sine';
            let roll = ctx.random() * total;
            for (const k of pool) { roll -= (defs[k].weight || 1); if (roll <= 0) return k; }
            return pool[pool.length - 1];
        }

        // Apply one tick of the assigned dive path shape.
        function moveDivePath(e, eDt) {
            const d = e.dPath;
            d.ph += eDt * 3;
            const k = d.kind || 'sine';
            if (k === 'zigzag') {
                e.x += (d.vx + Math.sin(d.ph) * d.amp * 2.4) * eDt;
                d.vx *= 0.995;
            } else if (k === 'arc') {
                const t = Math.min(1, d.ph / 6);
                e.x += (d.vx * (1 - 2 * t) + Math.sin(d.ph) * d.amp * 2) * eDt;
            } else if (k === 'bounce') {
                e.x += d.vx * eDt;
                if (e.x < 30) { e.x = 30; d.vx = Math.abs(d.vx); }
                else if (e.x > ctx.W - 30) { e.x = ctx.W - 30; d.vx = -Math.abs(d.vx); }
            } else if (k === 'weave') {
                e.x += (d.vx + Math.cos(d.ph) * d.amp * 3 + Math.sin(d.ph * 2.3) * d.amp * 1.6) * eDt;
            } else if (k === 'loop') {
                // Vertical sine on top of the dive descent curls the path into loops.
                e.x += (d.vx * 0.5 + Math.cos(d.ph) * d.amp * 3.2) * eDt;
                e.y += Math.sin(d.ph) * d.amp * 2.4 * eDt;
            } else if (k === 'strafe') {
                // Dive to mid-screen, then sweep sideways across the field before plunging on.
                if (!d.strafing && !d.strafed && e.y > ctx.H * 0.42) { d.strafed = true; d.strafing = true; d.vx = (e.x < ctx.W / 2 ? 1 : -1) * 190; e.sTmr = Math.min(e.sTmr, 200); }
                d.yMul = d.strafing ? 0.12 : (d.strafed ? 1.4 : 1);
                e.x += d.vx * eDt;
                if (d.strafing && (e.x < 24 || e.x > ctx.W - 24)) { d.strafing = false; d.vx = 0; }
            } else if (k === 'hover') {
                // Stop short, hover while firing, then plunge.
                if (d.hoverT === undefined && e.y > ctx.H * 0.36) { d.hoverT = 900; e.sTmr = Math.min(e.sTmr, 150); }
                if (d.hoverT > 0) { d.hoverT -= eDt * 1000; d.yMul = 0.05; e.x += Math.sin(d.ph * 1.3) * d.amp * 2.2 * eDt; }
                else { d.yMul = d.hoverT !== undefined ? 1.5 : 1; e.x += d.vx * eDt; }
            } else {
                e.x += d.vx * eDt + Math.cos(d.ph) * d.amp * 3 * eDt;
            }
        }

        // Assign a behaviour quirk to a freshly spawned enemy. Always driven by
        // the seeded RNG; no quirks on easy difficulty.
        function assignEnemyQuirk(e) {
            if (!e || e.quirk) return;
            if (e.type === 'boss' || e.type === 'miniboss' || e.sectorBoss) return;
            const table = GC.ENEMY_QUIRKS; const chances = GC.QUIRK_CHANCE;
            if (!table || !chances) return;
            const base = chances[ctx.settings.diff];
            if (!base) return;
            const stageMin = table.reduce((m, q) => Math.min(m, q.stageMin), Infinity);
            if (ctx.G.stage < stageMin) return;
            const chance = Math.min(0.8, base * Math.min(2, 1 + (ctx.G.stage - stageMin) * 0.06));
            if (ctx.random() >= chance) return;
            const pool = table.filter(q => ctx.G.stage >= q.stageMin && (!q.types || q.types.indexOf(e.type) >= 0));
            if (!pool.length) return;
            let total = 0; for (const q of pool) total += q.weight;
            let roll = ctx.random() * total;
            for (const q of pool) { roll -= q.weight; if (roll <= 0) { e.quirk = q.id; break; } }
            if (!e.quirk) e.quirk = pool[pool.length - 1].id;
            // A volleying bee must not fire the instant it reaches formation.
            if (e.quirk === 'volley') e.sTmr = 900 + ctx.random() * 900;
        }

        function startDive(e, opts) {
            if (e.st !== 'FORM') return;
            const warnMs = opts && opts.fast ? GC.REVENGE_DIVE.telegraphMs : 500;
            if (!e.diveReady) {
                if (e.divePending) return;
                e.divePending = true; e.shotWarning = warnMs; e.attackAnim = 1000;
                ctx.scheduleGame(() => { e.divePending = false; if (e.st === 'FORM') { e.diveReady = true; startDive(e, opts); } }, warnMs);
                return;
            }
            e.diveReady = false; e.st = 'DIVING';
            e.dPath = { ph: 0, kind: pickDivePath(e), amp: e.type === 'hunter' ? 18 + ctx.random() * 20 : 30 + ctx.random() * 40, vx: (ctx.random() - 0.5) * (e.type === 'hunter' ? 70 : 130) };
            e.dTmr = e.type === 'hunter' ? 4500 : 3000;
            e.sTmr = e.type === 'hunter' ? 280 + ctx.random() * 420 : 500 + ctx.random() * 1000;
            // Quirk "feint": dive a short way, then abort and climb back. Never
            // applied to homing types (they already rotate/flip on the way in).
            // Feint dives are shorter and can end in the air (no bottom reset).
            const quirk = (!opts || !opts.noQuirk) ? e.quirk : null;
            if (quirk === 'feint' && !isHomingType(e) && ctx.random() < GC.FEINT.chance) {
                e.st = 'FEINT';
                e.dTmr = GC.FEINT.minMs + ctx.random() * (GC.FEINT.maxMs - GC.FEINT.minMs);
            }
            if (quirk === 'feint' && e.st === 'FEINT' && ctx.SFX.feintWhistle) ctx.SFX.feintWhistle(e.x);
            else if (e.type === 'hunter') ctx.SFX.hunterDive(e.x);
            else ctx.SFX.dive(e.x);
            if (e.st === 'DIVING' && ctx.fxEnemyDiveStart) ctx.fxEnemyDiveStart(e.x, e.y);
            if ((e.type === 'boss' || e.type === 'miniboss') && !e.hasCap && !ctx.G.beam && ctx.G.stage > 1 && ctx.random() < 0.3) {
                // Beam "feint": with a chance the tractor beam collapses before
                // it can pull the player, then recharges.
                const feintBeam = ctx.random() < 0.3;
                ctx.G.beam = { active: true, owner: e, x: e.x, y: e.y + 16, h: 0, t: 0, cap: false, capT: 0, feint: feintBeam, abortAt: feintBeam ? 1100 + ctx.random() * 700 : 0 };
            }
        }
                function updateE(dt) {
            const eDt = dt * ctx.G.timeScale;
            const dtMs = eDt * 1000; ctx.G.fTmr += dt; ctx.G.fX = Math.sin(ctx.G.fTmr * 0.5) * 30;
            const _sTmrDt = dtMs * (ctx.G.bulletStorm ? 2 : 1);
            for (const e of ctx.G.enemies) {
                if (e.st === 'DEAD') continue;
                if (e.sectorBoss) { ctx.updateSectorBoss(e, dt * ctx.G.timeScale); continue; }
                const empScale = e.empUntil > ctx.G.simTime ? 0.5 : 1;
                const eDt = dt * ctx.G.timeScale * empScale, dtMs = eDt * 1000;
                e.empFormationDelay = (e.empFormationDelay || 0) + dt * (1 - empScale);
                const formationTime = ctx.G.fTmr - e.empFormationDelay, formationX = Math.sin(formationTime * 0.5) * 30;
                const _sTmrDt = dtMs * (ctx.G.bulletStorm ? 2 : 1);
                e.animTime = (e.animTime || 0) + dtMs;
                e.attackAnim = Math.max(0, (e.attackAnim || 0) - dtMs);
                e.shotWarning = Math.max(0, (e.shotWarning || 0) - dtMs);
                if (e.hitF > 0) e.hitF -= dtMs;
                if (ctx.updateEnemyMotionFx) ctx.updateEnemyMotionFx(e, dtMs);
                // Rage mode decay
                if (e.rageMode > 0) { e.rageMode -= dtMs; if (e.rageMode <= 0) { e.rageMode = 0; e.rageSpeedMult = 1; } }
                // Weak point update
                if (e.weakPoint) { e.weakPoint.angle += dtMs * 0.002; e.weakPoint.x = Math.cos(e.weakPoint.angle) * 12; e.weakPoint.y = -10 + Math.sin(e.weakPoint.angle * 1.5) * 5; }
                // Phasing mutation
                if (ctx.G.phasing && e.type !== 'boss' && e.type !== 'miniboss' && e.st === 'FORM') { e.phaseTimer = (e.phaseTimer || 0) + dtMs; e.invulnerable = (e.phaseTimer % 6000) < 1000; }
                // Gravity well mutation
                if (ctx.G.gravityWell && e.st === 'FORM') { const _gdx = ctx.W / 2 - e.x, _gdy = ctx.H / 3 - e.y, _gdist = Math.hypot(_gdx, _gdy); if (_gdist > 20) { e.x += (_gdx / _gdist) * 15 * dt; e.y += (_gdy / _gdist) * 15 * dt; } }
                // NEW: Boss phase transition
                if ((e.type === 'boss' || e.type === 'miniboss') && e.bossPhase > 0 && e.bossPhase < 3 && e.bossPhaseTransition <= 0) {
                    const hpRatio = e.hp / e.maxHp;
                    if (hpRatio <= e.bossPhaseHP[e.bossPhase - 1]) {
                        e.bossPhase++;
                        e.bossPhaseTransition = 800;
                        e.invulnerable = true;
                        ctx.G.shkT = Math.max(ctx.G.shkT, 400); ctx.G.shkM = Math.max(ctx.G.shkM, 5);
                        ctx.G.flashT = 100;
                        ctx.SFX.bossWarning();
                        // Spawn phase transition particles
                        for (let _pi = 0; _pi < 20; _pi++) {
                            const _pa = ctx.random() * Math.PI * 2;
                            ctx.G.part.push(ctx.getParticle({ x: e.x, y: e.y, vx: Math.cos(_pa) * 80, vy: Math.sin(_pa) * 80, life: 400, t: 0, col: e.bossPhase === 2 ? '#ff8800' : '#ff4444', size: 2, spark: true }));
                        }
                    }
                }
                if (e.bossPhaseTransition > 0) {
                    e.bossPhaseTransition -= dtMs;
                    if (e.bossPhaseTransition <= 0) e.invulnerable = false;
                }
                if (ctx.G.freezeT > 0 && e.st !== 'ENTER') continue;
                if (e.st === 'ENTER') {
                    e.eTmr -= dtMs;
                    if (e.eTmr <= 0) {
                        const enterK = Math.min(1, eDt * 5);
                        e.x += (e.fx - e.x) * enterK;
                        e.y += (e.fy - e.y) * enterK;
                        e.spawnAnim = Math.min(e.spawnDur, (e.spawnAnim || 0) + dtMs);
                        if (Math.abs(e.x - e.fx) < 2 && Math.abs(e.y - e.fy) < 2) {
                            e.x = e.fx + formationX;
                            e.y = e.fy + Math.sin(formationTime * 2 + (e.rowPhase || e.col * 0.5)) * (e.bobAmp || 3);
                            e.st = 'FORM';
                            e.spawnAnim = e.spawnDur;
                            for (let _ei = 0; _ei < 2; _ei++) { const _ea = ctx.random() * Math.PI * 2; ctx.G.part.push(ctx.getParticle({ x: e.x, y: e.y, vx: Math.cos(_ea)*25, vy: Math.sin(_ea)*25, life: 200, t: 0, col: e.type === 'bee' ? '#ffcc00' : e.type === 'butterfly' ? '#ff3366' : e.type === 'hunter' ? '#ff6600' : e.type === 'spinner' ? '#44ffff' : e.type === 'bomber' ? '#cc66ff' : e.type === 'lasher' ? '#44ff88' : '#44cc44', size: 1, spark: true })); }
                            if (e.type === 'miniboss' && !ctx.G.bossWarningShown) { ctx.G.bossWarningT = 2000; ctx.G.bossWarningShown = true; if (e.type === 'miniboss') ctx.SFX.miniBossWarning(); else ctx.SFX.bossWarning(); if (ctx.fxBossEntrance) ctx.fxBossEntrance(e.x, e.y); }
                            if (e.type === 'hunter') ctx.SFX.hunterDive(e.x);
                        }
                    }
                }
                else if (e.st === 'FORM') {
                    e.x = e.fx + formationX; e.y = e.fy + Math.sin(formationTime * 2 + (e.rowPhase || e.col * 0.5)) * (e.bobAmp || 3);
                    if ((e.spawnAnim || 0) < (e.spawnDur || 400)) e.spawnAnim = Math.min(e.spawnDur, (e.spawnAnim || 0) + dtMs);
                    // NEW: Weaver sine-wave horizontal movement
                    if (e.type === 'weaver') {
                        e.x += Math.sin(formationTime * 3 + e.col) * 40;
                        e.sTmr -= _sTmrDt;
                        if (e.sTmr <= 0 && ctx.G.p.alive && ctx.G.freezeT <= 0) {
                            ctx.enemyFire(e);
                            e.sTmr = 1800 + ctx.random() * 1200;
                        }
                    }
                    // NEW: Teleporter behavior
                    if (e.type === 'teleporter') {
                        e.teleportTimer = (e.teleportTimer || 0) - dtMs;
                        if (e.teleportTimer <= 0) {
                            e.teleportTimer = 2000 + ctx.random() * 1000;
                            const oldX = e.x, oldY = e.y;
                            e.x = 40 + ctx.random() * (ctx.W - 80);
                            e.y = ctx.FTOP + ctx.random() * 100; e.fx = e.x - formationX; e.fy = e.y;
                            for (let _ti = 0; _ti < 8; _ti++) {
                                const _ta = (_ti / 8) * Math.PI * 2;
                                ctx.G.part.push(ctx.getParticle({ x: oldX, y: oldY, vx: Math.cos(_ta) * 30, vy: Math.sin(_ta) * 30, life: 200, t: 0, col: '#44ffff', size: 1, spark: true }));
                                ctx.G.part.push(ctx.getParticle({ x: e.x, y: e.y, vx: Math.cos(_ta) * 30, vy: Math.sin(_ta) * 30, life: 200, t: 0, col: '#44ffff', size: 1, spark: true }));
                            }
                            for (const oe of ctx.G.enemies) {
                                if (oe === e || oe.st === 'DEAD') continue;
                                const dist = Math.hypot(oe.x - oldX, oe.y - oldY);
                                if (dist < 60) {
                                    oe.x += e.x - oldX; oe.y += e.y - oldY;
                                    oe.x = Math.max(20, Math.min(ctx.W - 20, oe.x));
                                    for (let _oi = 0; _oi < 4; _oi++) { const _oa = (_oi / 4) * Math.PI * 2; ctx.G.part.push(ctx.getParticle({ x: oe.x, y: oe.y, vx: Math.cos(_oa) * 20, vy: Math.sin(_oa) * 20, life: 150, t: 0, col: '#66ffff', size: 1, spark: true })); }
                                }
                            }
                        }
                        e.sTmr -= _sTmrDt;
                        if (e.sTmr <= 0 && ctx.G.p.alive && ctx.G.freezeT <= 0) {
                            ctx.enemyFire(e);
                            e.sTmr = 1500 + ctx.random() * 1000;
                        }
                    }
                    if ((e.type === 'sniper' || e.type === 'spinner' || e.type === 'bomber' || e.type === 'lasher' || e.type === 'weaver' || e.type === 'splitter' || e.type === 'shield_bee' || e.type === 'carrier' || e.type === 'teleporter' || e.st === 'DIVING' || e.st === 'FEINT' || e.quirk === 'volley') && ctx.G.p.alive && ctx.G.freezeT <= 0) {
                        e.sTmr -= _sTmrDt;
                        if (e.sTmr <= 0) {
                            ctx.enemyFire(e);
                            e.sTmr = e.type === 'spinner' ? 1600 + ctx.random() * 1200 : e.type === 'bomber' ? 2400 + ctx.random() * 1400 : e.type === 'lasher' ? 2100 + ctx.random() * 1600 : 2000 + ctx.random() * 1500;
                        }
                    }
                    if ((e.type === 'stalker' || e.type === 'hunter') && ctx.G.freezeT <= 0) { e.dTmr -= dtMs * 2; }
                    else if (!ctx.G.chal) { e.dTmr -= dtMs; }
                    if (ctx.G.st === 'PLAYING' && e.dTmr <= 0 && !ctx.G.chal && ctx.random() < 0.008 * Math.min(ctx.G.stage, 10) * ctx.diffMod('diveRate') * ctx.diveRateMult(e)) ctx.startDive(e);
                    else { e.dTmr -= dtMs; if (e.dTmr <= 0) { if (ctx.G.chal && ctx.G.st === 'PLAYING') ctx.startChalDive(e); else if (ctx.G.st === 'PLAYING') ctx.startDive(e); } }
                }
                else if (e.st === 'DIVING') {
                    e.dTmr -= dtMs;
                    if (e.dTmr <= 0 || e.y > ctx.H + 20) {
                        e.st = 'RETURN'; e.y = -20;
                    }
                    else {
                        const diveSpd = ctx.DIVE_SPD * (e.type === 'hunter' ? 2.1 : e.type === 'stalker' ? 1.5 : e.type === 'kamikaze' ? 2.5 : 1) * (e.rageSpeedMult || 1);
                        e.y += diveSpd * eDt * (e.dPath && e.dPath.yMul != null ? e.dPath.yMul : 1);
                        if (e.type === 'hunter' && ctx.G.p.alive) {
                            e.x += (ctx.G.p.x - e.x) * eDt * 4.8;
                            e.y += (ctx.G.p.y - e.y) * eDt * 1.1;
                        } else if (e.type === 'stalker' && ctx.G.p.alive) { e.x += (ctx.G.p.x - e.x) * eDt * 2.5; }
                        // NEW: Kamikaze charges directly at player
                        else if (e.type === 'kamikaze' && ctx.G.p.alive) {
                            const kdx = ctx.G.p.x - e.x, kdy = ctx.G.p.y - e.y, kdist = Math.hypot(kdx, kdy) || 1;
                            e.x += (kdx / kdist) * diveSpd * 1.8 * eDt;
                            e.y += (kdy / kdist) * diveSpd * 1.8 * eDt;
                        }
                        else if (e.dPath) moveDivePath(e, eDt);
                        if (ctx.G.beam && ctx.G.beam.owner === e) { ctx.G.beam.x = e.x; ctx.G.beam.y = e.y + 16; }
                        e.sTmr -= _sTmrDt;
                        if (e.sTmr <= 0 && !ctx.G.chal) {
                            ctx.enemyFire(e);
                            e.sTmr = e.type === 'hunter' ? 350 + ctx.random() * 450 : e.type === 'miniboss' ? 500 + ctx.random() * 800 : (e.type === 'spinner' || e.type === 'bomber' || e.type === 'lasher') ? 600 + ctx.random() * 700 : 800 + ctx.random() * 1200;
                        }
                        if (ctx.G.p.alive && ctx.G.p.inv <= 0) {
                            const ew = (e.type === 'boss' || e.type === 'miniboss') ? 20 : e.type === 'hunter' ? 18 : 16;
                            if (ctx.hit({ x: e.x - ew / 2, y: e.y - 10, w: ew, h: 20 }, ctx.getShipHitbox())) {
                                // NEW: Kamikaze explodes on contact, damaging player
                                if (e.type === 'kamikaze') {
                                    ctx.boom(e.x, e.y, false, 'kamikaze');
                                    ctx.G.shkT = Math.max(ctx.G.shkT, 300); ctx.G.shkM = Math.max(ctx.G.shkM, 5);
                                }
                                ctx.registerKill(e.x, e.y); ctx.addScore(ctx.PTS[e.type] ? ctx.PTS[e.type][1] : 200, e.x, e.y); ctx.boom(e.x, e.y, e.type === 'boss' || e.type === 'miniboss', e.type); ctx.SFX.eExplode(e.x); if (ctx.G.chal) ctx.G.chalHits++; e.st = 'DEAD'; ctx.killP();
                            }
                        }
                    }
                }
                else if (e.st === 'FEINT') {
                    // Aborted dive: shorter, slower and non-lethal by contact.
                    // No shots while feinting: a fake dive must stay a fake.
                    e.dTmr -= dtMs;
                    if (e.dTmr <= 0 || e.y > ctx.H - 40) {
                        e.st = 'RETURN';
                        if (ctx.fxEnemyFeint) ctx.fxEnemyFeint(e.x, e.y);
                        if (ctx.SFX.whoosh) ctx.SFX.whoosh(120, e.x);
                    } else {
                        const diveSpd = ctx.DIVE_SPD * GC.FEINT.speedMult * (e.rageSpeedMult || 1);
                        e.y += diveSpd * eDt;
                        if (e.dPath) moveDivePath(e, eDt);
                    }
                }
                else if (e.st === 'RETURN') {
                    e.x += (e.fx + formationX - e.x) * eDt * 3; e.y += (e.fy - e.y) * eDt * 3;
                    if (Math.abs(e.x - e.fx - formationX) < 3 && Math.abs(e.y - e.fy) < 3) { if (ctx.G.chal) { e.st = 'DEAD'; ctx.G.chalHits++; } else e.st = 'FORM'; }
                }
            }
            if (ctx.G.vipShip && ctx.G.vipShip.hp <= 0) {
                ctx.G.archetypeFailed = true;
                ctx.G.vipShip = null;
            }
            if (ctx.G.beam && ctx.G.beam.active) {
                const _bm = ctx.G.beam;
                _bm.t += dtMs; _bm.h = Math.min(Math.max(0, ctx.H - _bm.y), _bm.h + eDt * 300);
                if (_bm.feint && _bm.t > _bm.abortAt) {
                    // Beam feint collapsed: the captured fighter is released.
                    _bm.active = false; ctx.G.p.cap = null; ctx.G.beam = null;
                } else if (_bm.t > 3000) {
                    _bm.active = false; if (_bm.cap && ctx.G.p.cap) { _bm.owner.hasCap = true; ctx.G.p.cap = null; }
                }
            }
            ctx.G.dTmr -= dtMs;
            if (ctx.G.dTmr <= 0 && !ctx.G.chal && ctx.G.st === 'PLAYING') {
                const fe = ctx.G.enemies.filter(e => e.st === 'FORM' && !e.sectorBoss);
                if (fe.length) {
                    let launched = false;
                    // Squad dive: a whole formation row attacks in a staggered wave.
                    if (ctx.G.stage >= GC.SQUAD_DIVE.stageMin && fe.length >= GC.SQUAD_DIVE.minSize
                        && ctx.random() < Math.min(0.75, GC.SQUAD_DIVE.chance * ctx.diffMod('diveRate'))) {
                        const byRow = {};
                        for (const e of fe) { (byRow[e.r] = byRow[e.r] || []).push(e); }
                        let group = null;
                        for (const k in byRow) { if (byRow[k].length >= GC.SQUAD_DIVE.minSize) { group = byRow[k]; break; } }
                        if (group) {
                            const size = Math.min(group.length, GC.SQUAD_DIVE.maxSize);
                            for (let i = 0; i < size; i++) {
                                const sq = group[i];
                                ctx.scheduleGame(() => { if (!ctx.state.disposed && sq.st === 'FORM') ctx.startDive(sq, { noQuirk: true }); }, i * GC.SQUAD_DIVE.staggerMs);
                            }
                            ctx.G.fxSquadDive = { t: 0, dur: 620 };
                            if (ctx.fxSquadDiveCue) ctx.fxSquadDiveCue();
                            launched = true;
                        }
                    }
                    // Formation shuffle: two neighbours quietly swap slots.
                    if (!launched && ctx.G.stage >= GC.FORMATION_SHUFFLE.stageMin && ctx.random() < GC.FORMATION_SHUFFLE.chance) {
                        const rows = {};
                        for (const e of fe) { (rows[e.r] = rows[e.r] || []).push(e); }
                        for (const k in rows) {
                            const row = rows[k];
                            if (row.length < 2) continue;
                            row.sort((a, b) => a.fx - b.fx);
                            const i = Math.floor(ctx.random() * (row.length - 1));
                            const a = row[i], b = row[i + 1];
                            const ax = a.fx, ay = a.fy; a.fx = b.fx; a.fy = b.fy; b.fx = ax; b.fy = ay;
                            if (ctx.fxFormationShuffle) ctx.fxFormationShuffle(ax, ay, b.fx, b.fy);
                            launched = true;
                            break;
                        }
                    }
                    if (!launched) {
                        const hunters = fe.filter(e => e.type === 'hunter' || e.type === 'stalker');
                        const pick = hunters.length && ctx.random() < 0.45 ? hunters[Math.floor(ctx.random() * hunters.length)] : fe[Math.floor(ctx.random() * fe.length)];
                        ctx.startDive(pick);
                    }
                }
                ctx.G.dTmr = Math.max(500, (2000 - ctx.G.stage * 100) / ctx.diffMod('diveRate'));
            }
            // Quirk "shy": only joins the attack once the player is nearby.
            if (ctx.G.st === 'PLAYING' && !ctx.G.chal && ctx.G.p.alive) {
                ctx._shyT = (ctx._shyT || 0) + dtMs;
                if (ctx._shyT >= 600) {
                    ctx._shyT = 0;
                    for (const e of ctx.G.enemies) {
                        if (e.st !== 'FORM' || e.quirk !== 'shy' || e.sectorBoss) continue;
                        const dist = Math.hypot(e.x - ctx.G.p.x, e.y - ctx.G.p.y);
                        if (dist < 120 && ctx.random() < 0.25) ctx.startDive(e, { noQuirk: true });
                    }
                }
            }
            const alive = ctx.G.enemies.filter(e => e.st !== 'DEAD');
            if (alive.length === 0 && !ctx.hasPendingWaves() && !ctx.G.encounterBoss && ctx.G.levelSkipTimer <= 0 && ctx.G.st === 'PLAYING' && ctx.G.stageClearLock <= 0) {
                if (ctx.G.chal && ctx.G.chalHits === ctx.G.chalTot) { ctx.G.perfectT = 2000; ctx.addScore(5000, ctx.W / 2, ctx.H / 2 - 40, '#00ffcc'); ctx.SFX.perfect(); ctx.G.perfectCount++; if (ctx.G.perfectCount >= 3) ctx.unlockAchievement('perfectionist'); ctx.unlockAchievement('untouchable'); }
                ctx.advanceToNextStage(false);
            }
        }
                function updateHazards(dt) {
            const dtMs = dt * 1000;
            let hw = 0;
            for (let i = 0; i < ctx.G.envHazards.length; i++) {
                const h = ctx.G.envHazards[i];
                if (h.type === 'crystal_h' && !h.collected) {
                    h.t += dtMs;
                    if (ctx.G.p.alive && Math.hypot(ctx.G.p.x - h.x, ctx.G.p.y - h.y) < 16) {
                        h.collected = true;
                        ctx.G.weaponLv = Math.min(4, ctx.G.weaponLv + 1);
                        ctx.SFX.puCollect(h.x);
                        ctx.G.scorePopups.push({ x: h.x, y: h.y - 10, text: 'CRYSTAL!', t: 0, dur: 800, col: '#88ccff', big: true });
                        for (let ci = 0; ci < 10; ci++) { const ca = (ci / 10) * Math.PI * 2; ctx.G.part.push(ctx.getParticle({ x: h.x, y: h.y, vx: Math.cos(ca) * 50, vy: Math.sin(ca) * 50, life: 250, t: 0, col: '#88ccff', size: 2, spark: true })); }
                    }
                }
                if (h.hp > 0 || h.type === 'crystal_h') ctx.G.envHazards[hw++] = h;
            }
            ctx.G.envHazards.length = hw;

            if (ctx.G.solarFlareT > 0) {
                ctx.G.solarFlareT -= dtMs;
                if (ctx.G.solarFlareT <= 0 && ctx.G.st === 'PLAYING') {
                    ctx.G.solarFlareActive = true;
                    ctx.G.solarFlareT = 1200;
                    ctx.SFX.bossWarning();
                }
            } else if (ctx.G.solarFlareActive) {
                ctx.G.solarFlareT -= dtMs;
                if (ctx.G.solarFlareT <= 0) {
                    ctx.G.solarFlareActive = false;
                    ctx.G.solarFlareT = 6000 + ctx.random() * 4000;
                }
                const flareY = ctx.H * (1 - Math.max(0, ctx.G.solarFlareT) / 1200);
                if (ctx.G.p.alive && ctx.G.p.inv <= 0 && Math.abs(ctx.G.p.y - flareY) < 12) {
                    ctx.killP();
                }
            }

            if (ctx.G.emStormT > 0 && ctx.G.st === 'PLAYING') {
                ctx.G.emStormT -= dtMs;
                if (ctx.G.emStormT <= 0 && ctx.G.activePU && ctx.G.activePU.type !== 'shield') {
                    const puType = ctx.G.activePU.type;
                    ctx.G.activePU = null; ctx.G.puTimer = 0; ctx.setPUClass(null);
                    ctx.G.scorePopups.push({ x: ctx.W / 2, y: ctx.H / 2, text: 'EM STORM!', t: 0, dur: 1000, col: '#ffff44', big: true });
                    ctx.G.flashT = 50; ctx.G.emStormT = 10000 + ctx.random() * 5000;
                }
            }
        }

        // Revenge dive: when a formation comrade dies, nearby wingmates may break
        // formation and strike back immediately (briefened telegraph, rage aura).
        function onEnemyKilled(x, y) {
            const G = ctx.G;
            // Legacy formation commanders dive without telegraph; they are
            // always fair game (handled first so the RNG stream below stays
            // attached to the filtered candidate loop only).
            if (G.st !== 'PLAYING' || G.chal || G.freezeT > 0 || !G.p.alive) return;
            if (G.stage < GC.REVENGE_DIVE.stageMin || ctx.random() >= GC.REVENGE_DIVE.chance) return;
            const cands = [];
            for (const e of G.enemies) {
                if (e.st !== 'FORM' || e.sectorBoss || e.type === 'boss' || e.type === 'miniboss') continue;
                if (Math.abs(e.x - x) < GC.REVENGE_DIVE.radius && Math.abs(e.y - y) < GC.REVENGE_DIVE.radius) cands.push(e);
            }
            if (!cands.length) return;
            const n = Math.min(GC.REVENGE_DIVE.max, cands.length);
            for (let i = 0; i < n && cands.length; i++) {
                const e = cands.splice(Math.floor(ctx.random() * cands.length), 1)[0];
                e.rageMode = GC.REVENGE_DIVE.rageMs; e.rageSpeedMult = GC.REVENGE_DIVE.rageMult;
                ctx.startDive(e, { noQuirk: true, fast: true });
            }
            if (ctx.SFX.rageGrowl) ctx.SFX.rageGrowl(x);
        }

        function startChalDive(e) {
            if (e.st !== 'FORM') return; e.st = 'DIVING'; e.dPath = { ph: 0, amp: 50 + ctx.random() * 30, vx: (ctx.random() - 0.5) * 130 }; e.dTmr = 4000; e.sTmr = 99999; ctx.SFX.dive();
        }
        ctx.enemyFire = enemyFire;
        ctx.diveRateMult = diveRateMult;
        ctx.startDive = startDive;
        ctx.startChalDive = startChalDive;
        ctx.assignEnemyQuirk = assignEnemyQuirk;
        ctx.onEnemyKilled = onEnemyKilled;
        ctx.updateE = updateE;
        ctx.updateHazards = updateHazards;
    };
})();
