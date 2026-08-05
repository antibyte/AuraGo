(function () {
    'use strict';
    const GC = window.GalaxaCore = window.GalaxaCore || {};

    GC.createEntitiesBehaviors = function (ctx) {
                function enemyFire(e) {
            if (ctx.G.chal || ctx.G.st !== 'PLAYING') return;
            const _ebStart = ctx.G.ebul.length;
            const spd = ctx.EB_SPEED * ctx.diffMod('ebSpd');
            const px = e.x, py = e.y + 8;

            function mirrorBullet(b) {
                if (!ctx.modesIsMirrorPermanent || !ctx.modesIsMirrorPermanent()) return;
                b.x = ctx.W - b.x;
                if (b.vx) b.vx = -b.vx;
            }

            function pushEbul(b) {
                mirrorBullet(b);
                ctx.G.ebul.push(b);
            }

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
                        ctx.G.ebul.push(...ctx.ATTACK_PATTERNS.random_spread(e, 4, 0.4, 0.8, ebSpd));
                        ctx.SFX.hunterShot(e.x);
                        break;
                    case 3:
                        ctx.G.ebul.push(...ctx.ATTACK_PATTERNS.spiral(e, 12, 0.35, 0, ebSpd));
                        ctx.G.ebul.push(...ctx.ATTACK_PATTERNS.circle(e, 8, 0.3, ebSpd));
                        if (Math.random() < 0.5) ctx.G.ebul.push(...ctx.ATTACK_PATTERNS.wall(e, 3, 5, 0.25, ebSpd));
                        ctx.SFX.spinnerShot(e.x);
                        ctx.G.shkT = Math.max(ctx.G.shkT, 200); ctx.G.shkM = Math.max(ctx.G.shkM, 3);
                        break;
                }
            } else switch (e.type) {
                case 'hunter':
                    if (!ctx.G.p.alive) break;
                    ctx.SFX.hunterShot(e.x);
                    { const dx = ctx.G.p.x - px, dy = ctx.G.p.y - py, dist = Math.hypot(dx, dy) || 1, baseA = Math.atan2(dy, dx);
                      for (let i = -2; i <= 2; i++) { const a = baseA + i * 0.22; ctx.G.ebul.push({ x: px, y: py, w: 2, h: 5, vx: Math.cos(a) * spd * 0.62, vy: Math.sin(a) * spd * 0.62, kind: 'hunter' }); } }
                    break;
                case 'spinner':
                    ctx.SFX.spinnerShot(e.x);
                    e.shootPh = (e.shootPh || 0) + Math.PI / 3;
                    for (let i = 0; i < 8; i++) { const a = e.shootPh + i * Math.PI / 4; ctx.G.ebul.push({ x: px, y: py, w: 2, h: 4, vx: Math.cos(a) * spd * 0.44, vy: Math.sin(a) * spd * 0.44, kind: 'spiral' }); }
                    break;
                case 'bomber':
                    ctx.SFX.bomberDrop(e.x);
                    for (let i = -1; i <= 1; i++) ctx.G.ebul.push({ x: px + i * 10, y: py, w: 3, h: 3, vx: i * 38, vy: spd * 0.22, kind: 'mine', fuse: 2200, t: 0 });
                    break;
                case 'lasher':
                    if (!ctx.G.p.alive) break;
                    ctx.SFX.lasherShot(e.x);
                    { const dx = ctx.G.p.x - px, dy = ctx.G.p.y - py, dist = Math.hypot(dx, dy) || 1;
                      ctx.G.ebul.push({ x: px, y: py, w: 4, h: 10, vx: (dx / dist) * spd * 0.52, vy: (dy / dist) * spd * 0.52, kind: 'plasma' });
                      ctx.G.ebul.push({ x: px - 6, y: py, w: 3, h: 7, vx: (dx / dist) * spd * 0.4 + 28, vy: (dy / dist) * spd * 0.4, kind: 'plasma' });
                      ctx.G.ebul.push({ x: px + 6, y: py, w: 3, h: 7, vx: (dx / dist) * spd * 0.4 - 28, vy: (dy / dist) * spd * 0.4, kind: 'plasma' }); }
                    break;
                // NEW: Enemy type firing patterns
                case 'weaver':
                    if (!ctx.G.p.alive) break;
                    { const dx = ctx.G.p.x - px, dy = ctx.G.p.y - py, dist = Math.hypot(dx, dy) || 1;
                      ctx.G.ebul.push({ x: px, y: py, w: 2, h: 5, vx: (dx / dist) * spd * 0.45, vy: (dy / dist) * spd * 0.45, kind: 'hunter' }); }
                    break;
                case 'splitter':
                    ctx.G.ebul.push(...ctx.ATTACK_PATTERNS.random_spread(e, 5, 0.35, 0.6, spd));
                    break;
                case 'shield_bee':
                    ctx.G.ebul.push({ x: px, y: py, w: 2, h: 6 });
                    break;
                case 'kamikaze':
                    // Kamikaze doesn't shoot — it charges
                    break;
                case 'carrier':
                    ctx.G.ebul.push(...ctx.ATTACK_PATTERNS.aimed_burst(e, 3, 0.4, 0, spd, ctx.G.p.x, ctx.G.p.y));
                    break;
                case 'teleporter':
                    ctx.G.ebul.push(...ctx.ATTACK_PATTERNS.circle(e, 6, 0.3, spd));
                    break;
                case 'stalker':
                    ctx.G.ebul.push({ x: px, y: py, w: 2, h: 6 });
                    ctx.G.ebul.push({ x: px - 6, y: py - 2, w: 2, h: 6 });
                    ctx.G.ebul.push({ x: px + 6, y: py - 2, w: 2, h: 6 });
                    break;
                case 'sniper':
                    if (ctx.G.p.alive) {
                        const dx = ctx.G.p.x - px, dy = ctx.G.p.y - py, dist = Math.hypot(dx, dy);
                        if (dist > 1) { ctx.G.ebul.push({ x: px, y: py, w: 2, h: 6, vx: (dx / dist) * spd * 0.6, vy: (dy / dist) * spd * 0.6, kind: 'sniper' }); ctx.SFX.sniperShot(e.x); }
                    }
                    break;
                default:
                    if (e.type === 'bee') break;
                    ctx.G.ebul.push({ x: px, y: py, w: 2, h: 6 });
                    if (ctx.G.stage >= 5 && e.type === 'boss') { ctx.G.ebul.push({ x: px - 8, y: py, w: 2, h: 6 }); ctx.G.ebul.push({ x: px + 8, y: py, w: 2, h: 6 }); }
                    if (ctx.G.stage >= 8 && e.type === 'boss') { for (let k = 0; k < 3; k++) setTimeout(() => { if (!ctx.state.disposed && e.st === 'DIVING') pushEbul({ x: e.x, y: e.y + 8, w: 2, h: 6 }); }, k * 150); }
                    if (e.type === 'miniboss') { ctx.G.ebul.push({ x: px - 10, y: py, w: 2, h: 6 }); ctx.G.ebul.push({ x: px + 10, y: py, w: 2, h: 6 }); for (let k = 0; k < 2; k++) setTimeout(() => { if (!ctx.state.disposed && e.st === 'DIVING') { pushEbul({ x: e.x - 6, y: e.y + 8, w: 2, h: 6 }); pushEbul({ x: e.x + 6, y: e.y + 8, w: 2, h: 6 }); } }, k * 180); }
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
        function startDive(e) {
            if (e.st !== 'FORM') return;
            e.st = 'DIVING';
            e.dPath = { ph: 0, amp: e.type === 'hunter' ? 18 + Math.random() * 20 : 30 + Math.random() * 40, vx: (Math.random() - 0.5) * (e.type === 'hunter' ? 70 : 130) };
            e.dTmr = e.type === 'hunter' ? 4500 : 3000;
            e.sTmr = e.type === 'hunter' ? 280 + Math.random() * 420 : 500 + Math.random() * 1000;
            if (e.type === 'hunter' || e.type === 'stalker' || e.type === 'kamikaze') {
                e.rot = 0;
                e.rotTarget = Math.PI;
                e.rotTimer = 0;
                e.rotDuration = 500;
                if (window.__galaxaDebug) console.log('[rot]', e.type, 'startDive rotTarget=', e.rotTarget);
            }
            if (e.type === 'hunter') ctx.SFX.hunterDive(e.x); else ctx.SFX.dive();
            if ((e.type === 'boss' || e.type === 'miniboss') && !e.hasCap && !ctx.G.beam && ctx.G.stage > 1 && Math.random() < 0.3) ctx.G.beam = { active: true, owner: e, x: e.x, y: e.y + 16, h: 0, t: 0, cap: false, capT: 0 };
        }
                function updateE(dt) {
            const eDt = dt * ctx.G.timeScale;
            const dtMs = eDt * 1000; ctx.G.fTmr += dt; ctx.G.fX = Math.sin(ctx.G.fTmr * 0.5) * 30;
            for (const e of ctx.G.enemies) {
                if (e.st === 'DEAD') continue;
                // NEW: Sprite animation system (replaces old fr/frT toggle)
                e.animTimer += dtMs;
                const maxFrames = e.animFrames || 3;
                if (e.animTimer >= e.animSpeed) {
                    e.animFrame = (e.animFrame + 1) % maxFrames;
                    e.animTimer -= e.animSpeed;
                }
                // Keep old fr for backward compat in rendering (maps to animFrame)
                e.fr = e.animFrame % maxFrames;
                e.frT = e.animTimer;
                if (e.hitF > 0) e.hitF -= dtMs;
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
                            const _pa = Math.random() * Math.PI * 2;
                            ctx.G.part.push({ x: e.x, y: e.y, vx: Math.cos(_pa) * 80, vy: Math.sin(_pa) * 80, life: 400, t: 0, col: e.bossPhase === 2 ? '#ff8800' : '#ff4444', size: 2, spark: true });
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
                            e.x = e.fx + ctx.G.fX;
                            e.y = e.fy + Math.sin(ctx.G.fTmr * 2 + (e.rowPhase || e.col * 0.5)) * (e.bobAmp || 3);
                            e.st = 'FORM';
                            e.spawnAnim = e.spawnDur;
                            for (let _ei = 0; _ei < 2; _ei++) { const _ea = Math.random() * Math.PI * 2; ctx.G.part.push({ x: e.x, y: e.y, vx: Math.cos(_ea)*25, vy: Math.sin(_ea)*25, life: 200, t: 0, col: e.type === 'bee' ? '#ffcc00' : e.type === 'butterfly' ? '#ff3366' : e.type === 'hunter' ? '#ff6600' : e.type === 'spinner' ? '#44ffff' : e.type === 'bomber' ? '#cc66ff' : e.type === 'lasher' ? '#44ff88' : '#44cc44', size: 1, spark: true }); }
                            if ((e.type === 'boss' || e.type === 'miniboss') && !ctx.G.bossWarningShown) { ctx.G.bossWarningT = 2000; ctx.G.bossWarningShown = true; if (e.type === 'miniboss') ctx.SFX.miniBossWarning(); else ctx.SFX.bossWarning(); if (ctx.fxBossEntrance) ctx.fxBossEntrance(e.x, e.y); }
                            if (e.type === 'hunter') { ctx.G.bossWarningT = Math.max(ctx.G.bossWarningT || 0, 1000); ctx.SFX.hunterDive(e.x); }
                        }
                    }
                }
                else if (e.st === 'FORM') {
                    e.x = e.fx + ctx.G.fX; e.y = e.fy + Math.sin(ctx.G.fTmr * 2 + (e.rowPhase || e.col * 0.5)) * (e.bobAmp || 3);
                    if (e.rotPhase === undefined) e.rotPhase = Math.random() * Math.PI * 2;
                    e.rotPhase += eDt * 1.5;
                    e.rot = Math.sin(e.rotPhase) * 0.35;
                    if (window.__galaxaDebug && (e.type === 'hunter' || e.type === 'stalker' || e.type === 'kamikaze') && (ctx.tick % 30 === 0)) console.log('[rot]', e.type, 'FORM rot=', e.rot, 'rotPhase=', e.rotPhase);
                    if ((e.spawnAnim || 0) < (e.spawnDur || 400)) e.spawnAnim = Math.min(e.spawnDur, (e.spawnAnim || 0) + dtMs);
                    // NEW: Weaver sine-wave horizontal movement
                    if (e.type === 'weaver') {
                        e.x += Math.sin(ctx.G.fTmr * 3 + e.col) * 40;
                        e.sTmr -= dtMs;
                        if (e.sTmr <= 0 && ctx.G.p.alive && ctx.G.freezeT <= 0) {
                            ctx.enemyFire(e);
                            e.sTmr = 1800 + Math.random() * 1200;
                        }
                    }
                    // NEW: Teleporter behavior
                    if (e.type === 'teleporter') {
                        e.teleportTimer = (e.teleportTimer || 0) - dtMs;
                        if (e.teleportTimer <= 0) {
                            e.teleportTimer = 2000 + Math.random() * 1000;
                            const oldX = e.x, oldY = e.y;
                            e.x = 40 + Math.random() * (ctx.W - 80);
                            e.y = ctx.FTOP + Math.random() * 100;
                            for (let _ti = 0; _ti < 8; _ti++) {
                                const _ta = (_ti / 8) * Math.PI * 2;
                                ctx.G.part.push(getParticle({ x: oldX, y: oldY, vx: Math.cos(_ta) * 30, vy: Math.sin(_ta) * 30, life: 200, t: 0, col: '#44ffff', size: 1, spark: true }));
                                ctx.G.part.push(getParticle({ x: e.x, y: e.y, vx: Math.cos(_ta) * 30, vy: Math.sin(_ta) * 30, life: 200, t: 0, col: '#44ffff', size: 1, spark: true }));
                            }
                            for (const oe of ctx.G.enemies) {
                                if (oe === e || oe.st === 'DEAD') continue;
                                const dist = Math.hypot(oe.x - oldX, oe.y - oldY);
                                if (dist < 60) {
                                    oe.x += e.x - oldX; oe.y += e.y - oldY;
                                    oe.x = Math.max(20, Math.min(ctx.W - 20, oe.x));
                                    for (let _oi = 0; _oi < 4; _oi++) { const _oa = (_oi / 4) * Math.PI * 2; ctx.G.part.push(getParticle({ x: oe.x, y: oe.y, vx: Math.cos(_oa) * 20, vy: Math.sin(_oa) * 20, life: 150, t: 0, col: '#66ffff', size: 1, spark: true })); }
                                }
                            }
                        }
                        e.sTmr -= dtMs;
                        if (e.sTmr <= 0 && ctx.G.p.alive && ctx.G.freezeT <= 0) {
                            ctx.enemyFire(e);
                            e.sTmr = 1500 + Math.random() * 1000;
                        }
                    }
                    if ((e.type === 'sniper' || e.type === 'spinner' || e.type === 'bomber' || e.type === 'lasher' || e.type === 'weaver' || e.type === 'splitter' || e.type === 'shield_bee' || e.type === 'carrier' || e.type === 'teleporter') && ctx.G.p.alive && ctx.G.freezeT <= 0) {
                        e.sTmr -= dtMs;
                        if (e.sTmr <= 0) {
                            ctx.enemyFire(e);
                            e.sTmr = e.type === 'spinner' ? 1600 + Math.random() * 1200 : e.type === 'bomber' ? 2400 + Math.random() * 1400 : e.type === 'lasher' ? 2100 + Math.random() * 1600 : 2000 + Math.random() * 1500;
                        }
                    }
                    if ((e.type === 'stalker' || e.type === 'hunter') && ctx.G.freezeT <= 0) { e.dTmr -= dtMs * 2; }
                    else if (!ctx.G.chal) { e.dTmr -= dtMs; }
                    if (ctx.G.st === 'PLAYING' && e.dTmr <= 0 && !ctx.G.chal && Math.random() < 0.008 * Math.min(ctx.G.stage, 10) * ctx.diffMod('diveRate') * ctx.diveRateMult(e)) ctx.startDive(e);
                    else { e.dTmr -= dtMs; if (e.dTmr <= 0) { if (ctx.G.chal && ctx.G.st === 'PLAYING') ctx.startChalDive(e); else if (ctx.G.st === 'PLAYING') ctx.startDive(e); } }
                }
                else if (e.st === 'DIVING') {
                    e.dTmr -= dtMs;
                    if (e.dTmr <= 0 || e.y > ctx.H + 20) {
                        e.st = 'RETURN'; e.y = -20;
                        if (e.type === 'hunter' || e.type === 'stalker' || e.type === 'kamikaze') {
                            e.rotTimer = 0;
                            e.rotDuration = 500;
                            e.rotTarget = 0;
                            if (window.__galaxaDebug) console.log('[rot]', e.type, 'enter RETURN at rot=', e.rot, 'rotTimer=', e.rotTimer);
                        }
                    }
                    else {
                        if ((e.type === 'hunter' || e.type === 'stalker' || e.type === 'kamikaze') && e.rotTimer < e.rotDuration) {
                            e.rotTimer += dtMs;
                            const t = Math.min(e.rotTimer / e.rotDuration, 1);
                            e.rot = e.rotTarget * t;
                            if (window.__galaxaDebug) console.log('[rot]', e.type, 'DIVE t=', t, 'rot=', e.rot);
                        }
                        const diveSpd = ctx.DIVE_SPD * (e.type === 'hunter' ? 2.1 : e.type === 'stalker' ? 1.5 : e.type === 'kamikaze' ? 2.5 : 1) * (e.rageSpeedMult || 1);
                        e.y += diveSpd * eDt;
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
                        else if (e.dPath) { e.dPath.ph += eDt * 3; e.x += e.dPath.vx * eDt + Math.cos(e.dPath.ph) * e.dPath.amp * 3 * eDt; }
                        if (ctx.G.beam && ctx.G.beam.owner === e) { ctx.G.beam.x = e.x; ctx.G.beam.y = e.y + 16; }
                        e.sTmr -= dtMs;
                        if (e.sTmr <= 0 && !ctx.G.chal) {
                            ctx.enemyFire(e);
                            e.sTmr = e.type === 'hunter' ? 350 + Math.random() * 450 : e.type === 'miniboss' ? 500 + Math.random() * 800 : (e.type === 'spinner' || e.type === 'bomber' || e.type === 'lasher') ? 600 + Math.random() * 700 : 800 + Math.random() * 1200;
                        }
                        if (ctx.G.p.alive && ctx.G.p.inv <= 0) {
                            const ew = (e.type === 'boss' || e.type === 'miniboss') ? 20 : e.type === 'hunter' ? 18 : 16;
                            if (ctx.hit({ x: e.x - ew / 2, y: e.y - 10, w: ew, h: 20 }, { x: ctx.G.p.x - 8, y: ctx.G.p.y - 8, w: 16, h: 16 })) {
                                // NEW: Kamikaze explodes on contact, damaging player
                                if (e.type === 'kamikaze') {
                                    ctx.boom(e.x, e.y, false, 'kamikaze');
                                    ctx.G.shkT = Math.max(ctx.G.shkT, 300); ctx.G.shkM = Math.max(ctx.G.shkM, 5);
                                }
                                ctx.registerKill(); ctx.addScore(ctx.PTS[e.type] ? ctx.PTS[e.type][1] : 200, e.x, e.y); ctx.boom(e.x, e.y, e.type === 'boss' || e.type === 'miniboss', e.type); ctx.SFX.eExplode(e.x); if (ctx.G.chal) ctx.G.chalHits++; e.st = 'DEAD'; ctx.killP();
                            }
                        }
                    }
                }
                else if (e.st === 'RETURN') {
                    e.x += (e.fx + ctx.G.fX - e.x) * eDt * 3; e.y += (e.fy - e.y) * eDt * 3;
                    if ((e.type === 'hunter' || e.type === 'stalker' || e.type === 'kamikaze') && e.rotTimer < e.rotDuration) {
                        e.rotTimer += dtMs;
                        const t = Math.min(e.rotTimer / e.rotDuration, 1);
                        e.rot = e.rotTarget * t;
                    }
                    if (Math.abs(e.x - e.fx - ctx.G.fX) < 3 && Math.abs(e.y - e.fy) < 3) { if (ctx.G.chal) { e.st = 'DEAD'; ctx.G.chalHits++; } else e.st = 'FORM'; }
                }
            }
            if (ctx.G.vipShip && ctx.G.vipShip.hp <= 0) {
                ctx.G.archetypeFailed = true;
                ctx.G.vipShip = null;
            }
            if (ctx.G.beam && ctx.G.beam.active) { ctx.G.beam.t += dtMs; ctx.G.beam.h = Math.min(Math.max(0, ctx.H - ctx.G.beam.y), ctx.G.beam.h + eDt * 300); if (ctx.G.beam.t > 3000) { ctx.G.beam.active = false; if (ctx.G.beam.cap && ctx.G.p.cap) { ctx.G.beam.owner.hasCap = true; ctx.G.p.cap = null; } } }
            ctx.G.dTmr -= dtMs;
            if (ctx.G.asteroids) {
                for (let a of ctx.G.asteroids) {
                    a.x += a.vx * dt * 0.06;
                    a.y += a.vy * dt * 0.06;
                    if (a.y > ctx.H + 20) { a.y = -20; a.x = Math.random() * ctx.W; }
                    if (a.x < -20 || a.x > ctx.W + 20) a.vx *= -1;
                }
                ctx.G.asteroids = ctx.G.asteroids.filter(a => a.y < ctx.H + 50);
            }
            if (ctx.G.dTmr <= 0 && !ctx.G.chal && ctx.G.st === 'PLAYING') {
                const fe = ctx.G.enemies.filter(e => e.st === 'FORM');
                if (fe.length) {
                    const hunters = fe.filter(e => e.type === 'hunter' || e.type === 'stalker');
                    const pick = hunters.length && Math.random() < 0.45 ? hunters[Math.floor(Math.random() * hunters.length)] : fe[Math.floor(Math.random() * fe.length)];
                    ctx.startDive(pick);
                }
                ctx.G.dTmr = Math.max(500, (2000 - ctx.G.stage * 100) / ctx.diffMod('diveRate'));
            }
            const alive = ctx.G.enemies.filter(e => e.st !== 'DEAD');
            if (alive.length === 0 && ctx.G.levelSkipTimer <= 0 && ctx.G.st === 'PLAYING' && ctx.G.stageClearLock <= 0) {
                if (ctx.G.chal && ctx.G.chalHits === ctx.G.chalTot) { ctx.G.perfectT = 2000; ctx.addScore(5000, ctx.W / 2, ctx.H / 2 - 40, '#00ffcc'); ctx.SFX.perfect(); ctx.G.perfectCount++; if (ctx.G.perfectCount >= 3) ctx.unlockAchievement('perfectionist'); ctx.unlockAchievement('untouchable'); }
                ctx.advanceToNextStage(false);
            }
        }
                function updateHazards(dt) {
            const dtMs = dt * 1000;
            let hw = 0;
            for (let i = 0; i < ctx.G.envHazards.length; i++) {
                const h = ctx.G.envHazards[i];
                if (h.type === 'asteroid_h') {
                    h.x += h.vx * dt; h.y += h.vy * dt; h.rot += h.rotSpd * dt;
                    if (h.y > ctx.H + 20) { h.y = -20; h.x = 40 + Math.random() * (ctx.W - 80); }
                    for (let bi = ctx.G.bul.length - 1; bi >= 0; bi--) {
                        const b = ctx.G.bul[bi];
                        if (Math.hypot(b.x - h.x, b.y - h.y) < h.r + 3) {
                            h.hp--;
                            ctx.bulletImpact(b.x, b.y, '#886644');
                            if (!b.pierce && !b.laser) { ctx.G.bul.splice(bi, 1); }
                            if (h.hp <= 0) {
                                ctx.addScore(100, h.x, h.y, '#886644');
                                for (let pi = 0; pi < 8; pi++) { const pa = (pi / 8) * Math.PI * 2; ctx.G.part.push(getParticle({ x: h.x, y: h.y, vx: Math.cos(pa) * 40, vy: Math.sin(pa) * 40, life: 300, t: 0, col: '#776655', size: 2, debris: true, rot: Math.random() * 6.28 })); }
                                break;
                            }
                        }
                    }
                    for (let bi = ctx.G.ebul.length - 1; bi >= 0; bi--) {
                        const b = ctx.G.ebul[bi];
                        if (Math.hypot(b.x - h.x, b.y - h.y) < h.r + 3) {
                            ctx.bulletImpact(b.x, b.y, '#886644');
                            ctx.G.ebul.splice(bi, 1);
                        }
                    }
                } else if (h.type === 'crystal_h' && !h.collected) {
                    h.t += dtMs;
                    if (ctx.G.p.alive && Math.hypot(ctx.G.p.x - h.x, ctx.G.p.y - h.y) < 16) {
                        h.collected = true;
                        ctx.G.weaponLv = Math.min(4, ctx.G.weaponLv + 1);
                        ctx.SFX.puCollect(h.x);
                        ctx.G.scorePopups.push({ x: h.x, y: h.y - 10, text: 'CRYSTAL!', t: 0, dur: 800, col: '#88ccff', big: true });
                        for (let ci = 0; ci < 10; ci++) { const ca = (ci / 10) * Math.PI * 2; ctx.G.part.push(getParticle({ x: h.x, y: h.y, vx: Math.cos(ca) * 50, vy: Math.sin(ca) * 50, life: 250, t: 0, col: '#88ccff', size: 2, spark: true })); }
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
                    ctx.G.solarFlareT = 6000 + Math.random() * 4000;
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
                    ctx.G.flashT = 50; ctx.G.emStormT = 10000 + Math.random() * 5000;
                }
            }
        }

        function startChalDive(e) {
            if (e.st !== 'FORM') return; e.st = 'DIVING'; e.dPath = { ph: 0, amp: 50 + Math.random() * 30, vx: (Math.random() - 0.5) * 130 }; e.dTmr = 4000; e.sTmr = 99999; ctx.SFX.dive();
        }
        ctx.enemyFire = enemyFire;
        ctx.diveRateMult = diveRateMult;
        ctx.startDive = startDive;
        ctx.startChalDive = startChalDive;
        ctx.updateE = updateE;
        ctx.updateHazards = updateHazards;
    };
})();
