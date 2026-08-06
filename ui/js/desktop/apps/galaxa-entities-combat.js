(function () {
    'use strict';
    const GC = window.GalaxaCore = window.GalaxaCore || {};

    GC.createEntitiesCombat = function (ctx) {
        GC.createEntitiesWeapons(ctx);
        let lastFireT = 0;

        function playFireJuice(kind) {
            if (ctx.SFX.shootTyped) ctx.SFX.shootTyped(kind, ctx.G.p.x);
            else if (kind === 'laser' || kind === 'mega_laser') ctx.SFX.laserShoot(ctx.G.p.x);
            else ctx.SFX.shoot(ctx.G.p.x);
            ctx.G.muzzleT = 50;
            if (ctx.fxMuzzleSparks) ctx.fxMuzzleSparks(ctx.G.p.x, ctx.G.p.y, '#ffee88');
        }

                function fire(now) {
            const evoProf = ctx.G.weaponEvo && ctx.WEAPON_EVOS ? ctx.WEAPON_EVOS[ctx.G.weaponEvo] : null;
            // REMOVED: Old super effects branches — bursts are now triggered by galaxa-supers.js via triggerBurst() during superPhase==='burst'
            if (ctx.G.activePU && (ctx.G.activePU.type === 'laser' || ctx.G.activePU.type === 'mega_laser')) {
                const cd = ctx.G.activePU.type === 'mega_laser' ? 200 : 300;
                if (now - lastFireT < cd) return;
                lastFireT = now;
                ctx.G.bul.push({ x: ctx.G.p.x, y: ctx.G.p.y - 8, w: ctx.G.activePU.type === 'mega_laser' ? 6 : 4, h: 14, vx: 0, vy: -ctx.PB_SPEED * 1.5, laser: true });
                if (ctx.G.p.dual) ctx.G.bul.push({ x: ctx.G.p.x + 36, y: ctx.G.p.y - 8, w: ctx.G.activePU.type === 'mega_laser' ? 6 : 4, h: 14, vx: 0, vy: -ctx.PB_SPEED * 1.5, laser: true });
                playFireJuice(ctx.G.activePU.type);
                return;
            }
            if (evoProf && evoProf.isBeam) {
                const beamCd = 300;
                if (now - lastFireT < beamCd) return;
                lastFireT = now;
                ctx.G.bul.push({ x: ctx.G.p.x, y: ctx.G.p.y - 8, w: 4, h: 14, vx: 0, vy: -ctx.PB_SPEED * 1.5, laser: true });
                if (ctx.G.p.dual) ctx.G.bul.push({ x: ctx.G.p.x + 36, y: ctx.G.p.y - 8, w: 4, h: 14, vx: 0, vy: -ctx.PB_SPEED * 1.5, laser: true });
                playFireJuice('laser');
                return;
            }
            const isMegaRocket = ctx.G.activePU && ctx.G.activePU.type === 'mega_rocket';
            const isRocketLauncher = ctx.G.activePU && (ctx.G.activePU.type === 'rocket_launcher' || isMegaRocket);
            if (isRocketLauncher) {
                const cd = isMegaRocket ? 280 : 380;
                if (now - lastFireT < cd) return;
                lastFireT = now;
                const bulBefore = ctx.G.bul.length;
                ctx.fireRocketSalvo(isMegaRocket);
                ctx.mirrorDuplicateBullets(bulBefore);
                return;
            }
            const isUltraRapid = ctx.G.activePU && ctx.G.activePU.type === 'ultra_rapid';
            const isRapid = ctx.G.activePU && ctx.G.activePU.type === 'rapid';
            let cd = isUltraRapid ? 80 : isRapid ? 120 : 250;
            if (evoProf && evoProf.fireRate && !evoProf.isBeam) cd = Math.round(cd * evoProf.fireRate);
            if (now - lastFireT < cd) return;
            lastFireT = now;
            const bulBefore = ctx.G.bul.length;
            const isPierce = ctx.G.activePU && (ctx.G.activePU.type === 'pierce' || ctx.G.activePU.type === 'mega_pierce');
            const isHoming = ctx.G.activePU && ctx.G.activePU.type === 'homing';
            const isMegaSpread = ctx.G.activePU && ctx.G.activePU.type === 'mega_spread';
            const isSpread = ctx.G.activePU && ctx.G.activePU.type === 'spread';
            if (isHoming && ctx.G.activePU.shots > 0) {
                let nearestE = ctx.findNearestEnemy(ctx.G.p.x, ctx.G.p.y);
                if (nearestE) {
                    const ppx = ctx.G.p.x, ppy = ctx.G.p.y;
                    const dx = nearestE.x - ppx, dy = nearestE.y - ppy;
                    const dist = Math.sqrt(dx * dx + dy * dy);
                    ctx.G.bul.push({ x: ppx, y: ppy - 8, w: 3, h: 6, vx: (dx / dist) * ctx.PB_SPEED * 0.7, vy: (dy / dist) * ctx.PB_SPEED * 0.7, homing: true, target: nearestE });
                    ctx.G.activePU.shots--;
                    ctx.SFX.homingLock(ppx);
                    if (ctx.G.activePU.shots <= 0) { ctx.G.activePU = null; ctx.G.puTimer = 0; ctx.setPUClass(null); }
                }
                return;
            }
            if (isMegaSpread) {
                for (let a = -25; a <= 25; a += 10) {
                    const rad = a * Math.PI / 180;
                    ctx.G.bul.push({ x: ctx.G.p.x, y: ctx.G.p.y - 8, w: 2, h: 6, vx: Math.sin(rad) * ctx.PB_SPEED * 0.3, vy: -ctx.PB_SPEED, pierce: isPierce });
                }
            } else if (isSpread) {
                for (let a = -15; a <= 15; a += 15) {
                    const rad = a * Math.PI / 180;
                    ctx.G.bul.push({ x: ctx.G.p.x, y: ctx.G.p.y - 8, w: 2, h: 6, vx: Math.sin(rad) * ctx.PB_SPEED * 0.3, vy: -ctx.PB_SPEED, pierce: isPierce });
                }
            } else {
                const lv = ctx.G.weaponLv;
                if (lv >= 3) {
                    ctx.G.bul.push({ x: ctx.G.p.x - 6, y: ctx.G.p.y - 8, w: 2, h: 6, vx: 0, vy: -ctx.PB_SPEED, pierce: isPierce });
                    ctx.G.bul.push({ x: ctx.G.p.x, y: ctx.G.p.y - 8, w: 2, h: 6, vx: 0, vy: -ctx.PB_SPEED, pierce: isPierce });
                    ctx.G.bul.push({ x: ctx.G.p.x + 6, y: ctx.G.p.y - 8, w: 2, h: 6, vx: 0, vy: -ctx.PB_SPEED, pierce: isPierce });
                } else if (lv >= 2) {
                    ctx.G.bul.push({ x: ctx.G.p.x - 4, y: ctx.G.p.y - 8, w: 2, h: 6, vx: 0, vy: -ctx.PB_SPEED, pierce: isPierce });
                    ctx.G.bul.push({ x: ctx.G.p.x + 4, y: ctx.G.p.y - 8, w: 2, h: 6, vx: 0, vy: -ctx.PB_SPEED, pierce: isPierce });
                } else {
                    const max = ctx.G.p.dual ? 2 : 1;
                    let _fc = 0; for (const _fb of ctx.G.bul) if (!_fb.vx && !_fb.laser) _fc++;
                    if (_fc >= max) return;
                    ctx.G.bul.push({ x: ctx.G.p.x, y: ctx.G.p.y - 8, w: 2, h: 6, vx: 0, vy: -ctx.PB_SPEED, pierce: isPierce });
                    if (ctx.G.p.dual) ctx.G.bul.push({ x: ctx.G.p.x + 36, y: ctx.G.p.y - 8, w: 2, h: 6, vx: 0, vy: -ctx.PB_SPEED, pierce: isPierce });
                }
                if (lv >= 4 && !isRapid && !isUltraRapid) {
                    ctx.G.bul.push({ x: ctx.G.p.x, y: ctx.G.p.y - 8, w: 2, h: 6, vx: -Math.sin(0.2) * ctx.PB_SPEED * 0.2, vy: -ctx.PB_SPEED, pierce: isPierce });
                    ctx.G.bul.push({ x: ctx.G.p.x, y: ctx.G.p.y - 8, w: 2, h: 6, vx: Math.sin(0.2) * ctx.PB_SPEED * 0.2, vy: -ctx.PB_SPEED, pierce: isPierce });
                }
            }
            const kind = (ctx.G.activePU && ctx.G.activePU.type) || 'normal';
            playFireJuice(kind);
            ctx.G.stageAccuracyShots = (ctx.G.stageAccuracyShots || 0) + 1;
            ctx.mirrorDuplicateBullets(bulBefore);
        }
        function boom(x, y, isBoss, enemyType, killVx, killVy) {
            // NEW: Use per-enemy explosion profile for layered explosion intensity
            const _prof = (enemyType && ctx.EXPLOSION_PROFILE[enemyType]) || ctx.EXPLOSION_PROFILE.bee;
            const dur = isBoss ? 900 : 450;
            const _pScale = ctx.settings.particles === 'low' ? 0.55 : ctx.settings.particles === 'medium' ? 0.8 : 1;
            const _scaleN = (n) => Math.max(1, Math.round(n * _pScale));
            let pCount = isBoss ? _prof.debris * 3 : _prof.debris;
            let sparkCount = isBoss ? _prof.sparks * 2 : _prof.sparks;
            let debrisCount = isBoss ? _prof.debris * 2 : _prof.debris;
            let smokeCount = isBoss ? _prof.smoke * 2 : _prof.smoke;
            let flashCount = isBoss ? 8 : 4;
            pCount = _scaleN(pCount); sparkCount = _scaleN(sparkCount); debrisCount = _scaleN(debrisCount);
            smokeCount = _scaleN(smokeCount); flashCount = _scaleN(flashCount);
            ctx.G.exp.push({ x, y, t: 0, dur, seed: Math.random(), isBoss });
            if (isBoss) {
                ctx.G.exp.push({ x, y, t: 0, dur: 700, seed: Math.random(), isBoss: false, shockwave: true });
                ctx.G.exp.push({ x, y, t: 0, dur: 180, seed: Math.random(), isBoss: false, flash: true });
                for (let _bi = 0; _bi < 16; _bi++) { const _ba = (_bi / 16) * Math.PI * 2; const _bsp = 40 + Math.random() * 60; ctx.G.part.push(ctx.getParticle({ x, y, vx: Math.cos(_ba) * _bsp, vy: Math.sin(_ba) * _bsp, life: 800, t: 0, col: '#ffcc44', size: 2, spark: true, trail: true, bloom: true, bloomPhase: 0 })); }
            } else { ctx.G.exp.push({ x, y, t: 0, dur: 100, seed: Math.random(), isBoss: false, flash: true }); ctx.G.exp.push({ x, y, t: 0, dur: 300, seed: Math.random(), isBoss: false, shockwave: true }); }
            // NEW: Per-enemy-type death animation colors
            const typeCols = {
                bee: ['#ffcc00', '#ffaa00', '#ffee88', '#fff'],
                butterfly: ['#ff3366', '#ff6688', '#ff88aa', '#fff'],
                stalker: ['#6622aa', '#8844cc', '#aa66ee', '#fff'],
                sniper: ['#ffcc00', '#ffaa00', '#ffff44', '#fff'],
                hunter: ['#ff6600', '#ff8844', '#ffaa00', '#fff'],
                spinner: ['#00cccc', '#44ffff', '#88ffff', '#fff'],
                bomber: ['#aa44cc', '#cc66ff', '#ff44aa', '#fff'],
                lasher: ['#44ff88', '#00cc66', '#aaffcc', '#fff'],
                weaver: ['#ff8844', '#ffaa66', '#ffcc88', '#fff'],
                splitter: ['#88ff44', '#aaff66', '#ccff88', '#fff'],
                shield_bee: ['#ffcc00', '#ffdd44', '#ffee88', '#fff'],
                kamikaze: ['#ff2222', '#ff4444', '#ff6666', '#fff'],
                carrier: ['#cc88ff', '#ddaaff', '#eeccff', '#fff'],
                teleporter: ['#44ffff', '#66ffff', '#88ffff', '#fff']
            };
            const fireCols = (enemyType && typeCols[enemyType]) ? typeCols[enemyType] : (isBoss ? ['#ffcc00', '#ff8800', '#ff4444', '#fff'] : ['#ffcc00', '#ff4444', '#ff8800', '#fff', '#ffee88', '#ff6622', '#ffaa00']);
            // NEW: Per-enemy-type layered explosion sound + hitstop on boss kills
            if (ctx.SFX.eExplodeTyped) ctx.SFX.eExplodeTyped(enemyType || 'bee', isBoss ? 'big' : 'normal', x); else ctx.SFX.eExplode(x);
            if (isBoss) {
                if (ctx.SFX.bossKillFanfare) ctx.SFX.bossKillFanfare(x);
                if (ctx.fxBossKillSetPiece) ctx.fxBossKillSetPiece(x, y);
            }
            for (let i = 0; i < pCount; i++) {
                const a = (i / pCount) * Math.PI * 2 + Math.random() * 0.8, sp = 60 + (i * 23 % 160) * (isBoss ? 2 : 1.2);
                const cols = fireCols[i % fireCols.length];
                const sz = i % 4 === 0 ? 4 : i % 3 === 0 ? 3 : 2;
                const shapes = ['rect', 'diamond', 'circle', 'star'];
                const shape = Math.random() < 0.2 ? shapes[1 + Math.floor(Math.random() * 3)] : 'rect';
                ctx.G.part.push(ctx.getParticle({ x, y, vx: Math.cos(a) * sp, vy: Math.sin(a) * sp, life: (280 + (i * 41 % 280)) * (isBoss ? 1.6 : 1.1), t: 0, col: cols, size: sz, shape }));
            }
            // NEW: Type-specific death effects
            if (enemyType === 'bee') {
                for (let i = 0; i < 6; i++) { const a = Math.random() * Math.PI * 2; ctx.G.part.push(ctx.getParticle({ x, y, vx: Math.cos(a) * 30, vy: Math.sin(a) * 30 - 20, life: 400, t: 0, col: '#ffcc00', size: 2, spark: true })); }
            } else if (enemyType === 'butterfly') {
                for (let i = 0; i < 8; i++) { const a = (i / 8) * Math.PI * 2; ctx.G.part.push(ctx.getParticle({ x, y, vx: Math.cos(a) * 50, vy: Math.sin(a) * 50, life: 300, t: 0, col: '#ff3366', size: 1, spark: true })); }
            } else if (enemyType === 'stalker') {
                ctx.G.exp.push({ x, y, t: 0, dur: 400, seed: Math.random(), isBoss: false, implosion: true, col: '#8844cc' });
            } else if (enemyType === 'hunter') {
                for (let i = 0; i < 10; i++) { const a = Math.random() * Math.PI * 2; ctx.G.part.push(ctx.getParticle({ x, y, vx: Math.cos(a) * 70, vy: Math.sin(a) * 70, life: 350, t: 0, col: '#ff6600', size: 3, debris: true, rot: Math.random() * 6.28 })); }
            } else if (enemyType === 'spinner') {
                ctx.G.plasmaRings.push({ x, y, r: 0, maxR: 40, t: 0, dur: 300, col: '#44ffff' });
            } else if (enemyType === 'bomber') {
                for (let i = 0; i < 3; i++) { ctx.G.pendingBooms.push({ x: x + (Math.random() - 0.5) * 30, y: y + (Math.random() - 0.5) * 20, isBoss: false, delay: i * 80 }); }
            } else if (enemyType === 'lasher') {
                ctx.G.flashT = Math.max(ctx.G.flashT, 50);
                for (let i = 0; i < 6; i++) { const a = (i / 6) * Math.PI * 2; ctx.G.part.push(ctx.getParticle({ x, y, vx: Math.cos(a) * 60, vy: Math.sin(a) * 60, life: 200, t: 0, col: '#44ff88', size: 2, spark: true })); }
            } else if (enemyType === 'kamikaze') {
                ctx.G.shkT = Math.max(ctx.G.shkT, 300); ctx.G.shkM = Math.max(ctx.G.shkM, 5);
                ctx.G.exp.push({ x, y, t: 0, dur: 300, seed: Math.random(), isBoss: false, flash: true });
            } else if (enemyType === 'carrier') {
                for (let i = 0; i < 3; i++) { ctx.G.pendingBooms.push({ x: x + (Math.random() - 0.5) * 40, y: y + (Math.random() - 0.5) * 30, isBoss: false, delay: i * 150 }); }
            } else if (enemyType === 'teleporter') {
                for (let i = 0; i < 12; i++) { const a = (i / 12) * Math.PI * 2; ctx.G.part.push(ctx.getParticle({ x, y, vx: Math.cos(a) * 80, vy: Math.sin(a) * 80, life: 250, t: 0, col: '#44ffff', size: 1, spark: true })); }
            }
            for (let i = 0; i < sparkCount; i++) {
                const a = Math.random() * Math.PI * 2, sp = 90 + Math.random() * 150;
                ctx.G.part.push(ctx.getParticle({ x, y, vx: Math.cos(a) * sp, vy: Math.sin(a) * sp, life: 120 + Math.random() * 180, t: 0, col: Math.random() > 0.5 ? '#ffffff' : '#ffeeaa', size: 1, spark: true, shape: 'diamond' }));
            }
            for (let i = 0; i < debrisCount; i++) {
                const hasKillDir = killVx !== undefined && killVy !== undefined;
                const baseA = hasKillDir ? Math.atan2(killVy, killVx) + (Math.random() - 0.5) * 1.5 : Math.random() * Math.PI * 2;
                const sp = 25 + Math.random() * 50;
                const sz = isBoss ? 3 + Math.random() * 4 : 2 + Math.random() * 3;
                ctx.G.part.push(ctx.getParticle({ x, y, vx: Math.cos(baseA) * sp + (killVx || 0) * 0.15, vy: Math.sin(baseA) * sp + (killVy || 0) * 0.15 - 18, life: 600 + Math.random() * 500, t: 0, col: isBoss ? '#999' : '#777', size: sz, debris: true, rot: Math.random() * 6.28 }));
            }
            for (let i = 0; i < smokeCount; i++) {
                const a = Math.random() * Math.PI * 2, sp = 12 + Math.random() * 25;
                ctx.G.part.push(ctx.getParticle({ x, y, vx: Math.cos(a) * sp, vy: Math.sin(a) * sp - 15, life: 600 + Math.random() * 500, t: 0, col: Math.random() > 0.5 ? '#666' : '#555', size: 3 + (isBoss ? 3 : 0), smoke: true, shape: 'circle' }));
            }
            for (let i = 0; i < flashCount; i++) {
                const a = Math.random() * Math.PI * 2, sp = 40 + Math.random() * 80;
                ctx.G.part.push(ctx.getParticle({ x, y, vx: Math.cos(a) * sp, vy: Math.sin(a) * sp, life: 60 + Math.random() * 40, t: 0, col: '#ffffff', size: 2, spark: true, trail: true, shape: 'star' }));
            }
            if (isBoss) {
                for (let i = 0; i < 6; i++) {
                    ctx.G.pendingBooms.push({ x: x + (Math.random() - 0.5) * 50, y: y + (Math.random() - 0.5) * 40, isBoss: false, delay: i * 100 });
                }
                ctx.G.shkT = Math.max(ctx.G.shkT, 800); ctx.G.shkM = Math.max(ctx.G.shkM, 7);
                ctx.G.shkX = x; ctx.G.shkY = y;
                ctx.G.plasmaRings.push({ x, y, r: 0, maxR: 140, t: 0, dur: 800, col: '#ff4444' });
                ctx.G.plasmaRings.push({ x, y, r: 0, maxR: 100, t: 0, dur: 550, col: '#ff8800' });
                ctx.G.plasmaRings.push({ x, y, r: 0, maxR: 60, t: 0, dur: 320, col: '#ffcc00' });
                ctx.G.plasmaRings.push({ x, y, r: 0, maxR: 30, t: 0, dur: 200, col: '#ffffff' });
            }
        }
        function bulletImpact(x, y, col, dirX, dirY) {
            for (let i = 0; i < 4; i++) {
                const a = Math.random() * Math.PI * 2, sp = 30 + Math.random() * 50;
                ctx.G.part.push(ctx.getParticle({ x, y, vx: Math.cos(a) * sp, vy: Math.sin(a) * sp, life: 80 + Math.random() * 60, t: 0, col: col || '#ffff88', size: 1, spark: true }));
            }
            // NEW: Optional directional spark cone along the bullet travel direction (galaxa-fx)
            if (dirX !== undefined && dirY !== undefined && ctx.fxSparkCone) ctx.fxSparkCone(x, y, col, dirX, dirY);
        }
        function addScore(pts, x, y, col) {
            const prev = ctx.G.score;
            const baseScore = pts;
            const mult = (ctx.settings.riskIt && ctx.applyRiskItMultiplier)
                ? ctx.applyRiskItMultiplier(ctx.G.combo)
                : ctx.G.comboMult;
            const multiplied = Math.floor(baseScore * mult * (ctx.G.scoreMult || 1));
            ctx.G.score += multiplied;
            if (ctx.G.score > ctx.G.hi) ctx.G.hi = ctx.G.score;
            const text = mult > 1 ? '+' + multiplied + ' x' + (ctx.settings.riskIt ? mult.toFixed(2) : mult) : '+' + multiplied;
            if (x !== undefined) ctx.G.scorePopups.push({ x, y, text, t: 0, dur: 800, col: col || '#ffcc00', big: mult > 1 });
            if (Math.floor(ctx.G.score / ctx.EXTRA_LIFE) > Math.floor(prev / ctx.EXTRA_LIFE)) { ctx.G.lives++; ctx.SFX.extra(); }
            const _crb = ctx.relic_getRelicBonuses ? ctx.relic_getRelicBonuses() : { creditMult: 1 }; ctx.G.credits = Math.floor(ctx.G.credits * _crb.creditMult);
        }
        function updateCombo(dtMs) {
            if (ctx.G.comboTimer > 0) {
                ctx.G.comboTimer -= dtMs || 16;
                if (ctx.G.comboTimer <= 0) { if (ctx.G.combo > 2) ctx.SFX.comboBreak(); ctx.G.combo = 0; ctx.G.comboMult = 1; ctx.G.comboBanner = null; }
            }
        }
        function getComboTimeout() { const _rb = ctx.relic_getRelicBonuses ? ctx.relic_getRelicBonuses() : { comboBonus: 0 }; return ctx.COMBO_TIMEOUT + _rb.comboBonus; }
        function registerKill() {
            ctx.G.combo++;
            ctx.G.comboTimer = getComboTimeout();
            // NEW: Fill super meter on kill (only when no super is active)
            if (ctx.G.superPhase === 'idle') {
                ctx.G.superMeter = Math.min(100, (ctx.G.superMeter || 0) + 5);
            }
            if (ctx.G.combo >= 15) ctx.unlockAchievement('combo_king');
            if (ctx.G.combo >= 30) ctx.unlockAchievement('combo_god');
            let level = 0;
            for (let i = ctx.COMBO_THRESH.length - 1; i >= 0; i--) { if (ctx.G.combo >= ctx.COMBO_THRESH[i]) { level = i + 1; break; } }
            ctx.G.comboMult = ctx.COMBO_MULT[level] || 1;
            if (level > 0 && ctx.COMBO_TEXT[level]) {
                ctx.G.comboBanner = { text: ctx.COMBO_TEXT[level], mult: ctx.G.comboMult, t: 0, dur: 1200 };
                ctx.SFX.combo(level);
                if (level >= 4) ctx.SFX.killStreak();
                // NEW: Screen-edge pulse + rising pitch arpeggio on combo milestones (galaxa-fx)
                if (ctx.fxComboPulse) ctx.fxComboPulse(level);
                if (level >= 3 && ctx.SFX.comboRiser) ctx.SFX.comboRiser(level, ctx.W / 2);
            }
            if (ctx.G.combo === 10) {
                for (const e of ctx.G.enemies) { if (e.st !== 'DEAD' && e.type !== 'boss' && e.type !== 'miniboss') { ctx.addScore(50, e.x, e.y, '#ff4444'); ctx.boom(e.x, e.y, false, e.type); e.st = 'DEAD'; } }
                ctx.G.flashT = 80; ctx.G.shkT = 200; ctx.G.shkM = 4;
                ctx.G.scorePopups.push({ x: ctx.W / 2, y: ctx.H / 2 - 60, text: 'COMBO BOMB!', t: 0, dur: 1200, col: '#ff4444', big: true });
                ctx.SFX.bomb(ctx.W / 2);
            }
            if (ctx.G.combo === 20) {
                ctx.G.timeScale = 0.35; ctx.G.timeSlowTimer = 3000;
                ctx.G.scorePopups.push({ x: ctx.W / 2, y: ctx.H / 2 - 60, text: 'COMBO FREEZE!', t: 0, dur: 1200, col: '#aa44ff', big: true });
                ctx.SFX.freeze(ctx.W / 2);
            }
            if (ctx.G.combo === 30) {
                for (const e of ctx.G.enemies) { if (e.st !== 'DEAD') { ctx.addScore(ctx.PTS[e.type] ? ctx.PTS[e.type][0] + 500 : 500, e.x, e.y, '#ffffff'); ctx.boom(e.x, e.y, e.type === 'boss' || e.type === 'miniboss', e.type); e.st = 'DEAD'; } }
                ctx.G.ebul = []; ctx.G.flashT = 200; ctx.G.shkT = 600; ctx.G.shkM = 8;
                ctx.G.scorePopups.push({ x: ctx.W / 2, y: ctx.H / 2 - 60, text: 'SUPERNOVA!', t: 0, dur: 1500, col: '#ffffff', big: true });
                ctx.SFX.supernova(ctx.W / 2);
            }
        }
        function hit(a, b) { return a.x < b.x + b.w && a.x + a.w > b.x && a.y < b.y + b.h && a.y + a.h > b.y; }
        function dropPU(e) {
            let chance = e.type === 'miniboss' ? 1 : (e.type === 'boss' ? 0.35 : (e.type === 'bee' && !ctx.diffMod('puFromBee') ? 0 : 0.12));
            if (ctx.G.powerSurge) chance *= 3;
            const _rb = ctx.relic_getRelicBonuses ? ctx.relic_getRelicBonuses() : { dropBonus: 0 }; chance += _rb.dropBonus;
            if (Math.random() < chance) {
                // NEW: Weighted rarity-based powerup selection
                let type;
                const roll = Math.random() * 100;
                if (roll < ctx.PU_RARITY_WEIGHT.legendary) {
                    type = ctx.PU_RARITY.legendary[Math.floor(Math.random() * ctx.PU_RARITY.legendary.length)];
                } else if (roll < ctx.PU_RARITY_WEIGHT.legendary + ctx.PU_RARITY_WEIGHT.rare) {
                    type = ctx.PU_RARITY.rare[Math.floor(Math.random() * ctx.PU_RARITY.rare.length)];
                } else if (roll < ctx.PU_RARITY_WEIGHT.legendary + ctx.PU_RARITY_WEIGHT.rare + ctx.PU_RARITY_WEIGHT.uncommon) {
                    type = ctx.PU_RARITY.uncommon[Math.floor(Math.random() * ctx.PU_RARITY.uncommon.length)];
                } else {
                    type = ctx.PU_RARITY.common[Math.floor(Math.random() * ctx.PU_RARITY.common.length)];
                }
                if (type === 'levelskip' && (e.type !== 'boss' && e.type !== 'miniboss')) type = 'rapid';
                if (type === 'levelskip' && Math.random() > 0.05) type = ctx.PU_RARITY.legendary[Math.floor(Math.random() * (ctx.PU_RARITY.legendary.length - 1))];
                ctx.G.powerups.push({ x: e.x, y: e.y, type, t: 0 });
            }
        }
        function collectPU(pu) {
            ctx.G.collectedPU.add(pu.type);
            if (ctx.G.collectedPU.size >= ctx.PU_TYPES.length) ctx.unlockAchievement('power_collector');
            let _puRarity = 'common';
            for (const _rk in ctx.PU_RARITY) { if (ctx.PU_RARITY[_rk].indexOf(pu.type) >= 0) { _puRarity = _rk; break; } }
            const _playPuSfx = (panX) => { if (ctx.SFX.puCollectRarity) ctx.SFX.puCollectRarity(_puRarity, panX); else ctx.SFX.puCollect(panX); };
            const _weaponPuTypes = ['laser', 'rapid', 'spread', 'rocket_launcher', 'mine_layer', 'mega_laser', 'ultra_rapid', 'mega_spread', 'mega_rocket', 'mega_mine_layer'];
            const _playWeaponArm = () => { if (_weaponPuTypes.indexOf(pu.type) >= 0 || (ctx.G.activePU && _weaponPuTypes.indexOf(ctx.G.activePU.type) >= 0)) { if (ctx.SFX.weaponArm) ctx.SFX.weaponArm(pu.x); if (ctx.fxWeaponArmPulse) ctx.fxWeaponArmPulse(ctx.G.p.x, ctx.G.p.y); } };
            if (pu.type === 'megabomb') {
                ctx.SFX.megabomb(pu.x);
                for (const e of ctx.G.enemies) {
                    if (e.st !== 'DEAD') {
                        if (e.type === 'boss' || e.type === 'miniboss') {
                            e.hp -= 2;
                            if (e.hp <= 0) {
                                const pts = ctx.PTS[e.type] ? ctx.PTS[e.type][0] + 800 : 800;
                                ctx.addScore(pts, e.x, e.y, '#ff2200');
                                ctx.boom(e.x, e.y, true, e.type); ctx.SFX.eExplode(e.x); e.st = 'DEAD'; ctx.dropPU(e);
                            } else { e.enraged = true; e.hitF = 300; e.rageMode = 2500; e.rageSpeedMult = 1.4; }
                        } else {
                            const pts = ctx.PTS[e.type] ? ctx.PTS[e.type][0] + 300 : 300;
                            ctx.addScore(pts, e.x, e.y, '#ff2200'); ctx.boom(e.x, e.y, false, e.type); e.st = 'DEAD';
                        }
                    }
                }
                for (let i = ctx.G.ebul.length - 1; i >= 0; i--) ctx.bulletImpact(ctx.G.ebul[i].x, ctx.G.ebul[i].y, '#ff4400');
                ctx.G.ebul = [];
                ctx.G.flashT = 250; ctx.G.shkT = 700; ctx.G.shkM = 10;
                ctx.G.scorePopups.push({ x: ctx.W / 2, y: ctx.H / 2 - 50, text: ctx.t('galaxa.megabomb'), t: 0, dur: 1800, col: '#ff2200', big: true });
                if (ctx.fxScreenShatter) ctx.fxScreenShatter();
                ctx.G.activePU = null; ctx.G.puTimer = 0; ctx.setPUClass(null);
                return;
            }
            if (pu.type === 'bomb' || pu.type === 'multibomb') {
                ctx.SFX.bomb(pu.x);
                const bonus = pu.type === 'multibomb' ? 500 : 0;
                for (const e of ctx.G.enemies) {
                    if (e.st !== 'DEAD') {
                        if (e.type === 'boss' || e.type === 'miniboss') { e.enraged = true; e.hitF = 200; }
                        else { const pts = ctx.PTS[e.type] ? ctx.PTS[e.type][0] : 200; ctx.addScore(pts + bonus, e.x, e.y, ctx.PU_COL[pu.type]); ctx.boom(e.x, e.y, false, e.type); e.st = 'DEAD'; }
                    }
                }
                for (const e of ctx.G.enemies) { if (e.st !== 'DEAD' && e.enraged) { e.animSpeed = Math.max(40, (e.animSpeed || 120) * 0.6); } }
                ctx.G.flashT = 100; ctx.G.activePU = null; ctx.G.puTimer = 0; ctx.setPUClass(null); return;
            }
            if (pu.type === 'supernova') {
                ctx.SFX.supernova(pu.x);
                for (const e of ctx.G.enemies) {
                    if (e.st !== 'DEAD') {
                        if (e.type === 'boss' || e.type === 'miniboss') { e.enraged = true; e.hitF = 300; }
                        else { const pts = ctx.PTS[e.type] ? ctx.PTS[e.type][0] : 200; ctx.addScore(pts + 1000, e.x, e.y, '#fff'); ctx.boom(e.x, e.y, false, e.type); e.st = 'DEAD'; }
                    }
                }
                for (const e of ctx.G.enemies) { if (e.st !== 'DEAD' && e.enraged) { e.animSpeed = Math.max(40, (e.animSpeed || 120) * 0.6); } }
                for (let i = ctx.G.ebul.length - 1; i >= 0; i--) { ctx.bulletImpact(ctx.G.ebul[i].x, ctx.G.ebul[i].y, '#fff'); }
                ctx.G.ebul = []; ctx.G.flashT = 200; ctx.G.activePU = null; ctx.G.puTimer = 0; ctx.setPUClass(null);
                ctx.G.shkT = 500; ctx.G.shkM = 8;
                return;
            }
            if (pu.type === 'freeze') {
                ctx.SFX.freeze(pu.x);
                ctx.G.freezeT = ctx.PU_DUR.freeze;
                ctx.G.activePU = { type: 'freeze', timer: ctx.PU_DUR.freeze }; ctx.G.puTimer = ctx.PU_DUR.freeze; ctx.setPUClass('freeze');
                for (const e of ctx.G.enemies) {
                    if (e.st === 'DEAD') continue;
                    for (let _fi = 0; _fi < 8; _fi++) {
                        const _fa = (_fi / 8) * Math.PI * 2;
                        ctx.G.part.push(ctx.getParticle({ x: e.x + Math.cos(_fa) * 14, y: e.y + Math.sin(_fa) * 14, vx: Math.cos(_fa) * 35, vy: Math.sin(_fa) * 35 - 10, life: 500, t: 0, col: '#88eeff', size: 2 }));
                        ctx.G.part.push(ctx.getParticle({ x: e.x, y: e.y, vx: (Math.random()-0.5)*40, vy: -20-Math.random()*30, life: 350, t: 0, col: '#ccf4ff', size: 1, spark: true }));
                    }
                }
                ctx.G.flashT = 30; return;
            }
            if (pu.type === 'levelskip') {
                ctx.SFX.supernova(pu.x);
                let delay = 0;
                for (const e of ctx.G.enemies) {
                    if (e.st !== 'DEAD') {
                        const pts = ctx.PTS[e.type] ? ctx.PTS[e.type][0] : 200;
                        ctx.addScore(pts + 200, e.x, e.y, '#ff88ff');
                        ctx.G.pendingBooms.push({ x: e.x, y: e.y, isBoss: e.type === 'boss' || e.type === 'miniboss', delay });
                        e.st = 'DEAD';
                        delay += 120;
                    }
                }
                for (let i = ctx.G.ebul.length - 1; i >= 0; i--) ctx.bulletImpact(ctx.G.ebul[i].x, ctx.G.ebul[i].y, '#ff88ff');
                ctx.G.ebul = [];
                ctx.G.levelSkipTimer = delay + 800;
                ctx.G.flashT = 200; ctx.G.shkT = 300; ctx.G.shkM = 6;
                ctx.G.activePU = null; ctx.G.puTimer = 0; ctx.setPUClass(null);
                return;
            }
            const isUpgradeable = ctx.PU_UPGRADE[pu.type];
            const isSameType = ctx.G.activePU && ctx.G.activePU.type === pu.type;
            const isWeaponUpgrade = isUpgradeable && isSameType && !ctx.G.puUpgrade;
            if (pu.type === 'drone') {
                const count = (isUpgradeable && isSameType) ? 2 : 1;
                ctx.G.drones = [];
                for (let di = 0; di < count; di++) ctx.G.drones.push({ x: ctx.G.p.x + (di === 0 ? -20 : 20), y: ctx.G.p.y - 20, targetX: ctx.G.p.x + (di === 0 ? -25 : 25), targetY: ctx.G.p.y - 30, fireT: 0 });
                ctx.G.droneTimer = ctx.PU_DUR.drone; ctx.G.activePU = { type: count > 1 ? 'dual_drone' : 'drone', timer: ctx.PU_DUR.drone }; ctx.G.puTimer = ctx.PU_DUR.drone; ctx.setPUClass(count > 1 ? 'dual_drone' : 'drone');
                _playPuSfx(pu.x); return;
            }
            if (pu.type === 'blackhole_bomb') {
                ctx.G.blackhole = { x: ctx.G.p.x, y: ctx.G.p.y - 60, targetX: ctx.G.p.x, targetY: ctx.G.p.y - 120, t: 0 };
                ctx.SFX.bomb(pu.x); ctx.G.activePU = null; ctx.G.puTimer = 0; ctx.setPUClass(null);
                for (let i = 0; i < 8; i++) { const a = (i / 8) * Math.PI * 2; ctx.G.part.push(ctx.getParticle({ x: ctx.G.p.x, y: ctx.G.p.y - 60, vx: Math.cos(a) * 50, vy: Math.sin(a) * 50, life: 300, t: 0, col: '#8844ff', size: 2, spark: true })); }
                return;
            }
            if (pu.type === 'gravity_bomb') {
                ctx.G.gravityBomb = { x: ctx.G.p.x, y: ctx.G.p.y - 80, t: 0, phase: 'pull' };
                ctx.SFX.bomb(pu.x); ctx.G.activePU = null; ctx.G.puTimer = 0; ctx.setPUClass(null);
                for (let i = 0; i < 12; i++) { const a = (i / 12) * Math.PI * 2; ctx.G.part.push(ctx.getParticle({ x: ctx.G.p.x, y: ctx.G.p.y - 80, vx: Math.cos(a) * 60, vy: Math.sin(a) * 60, life: 400, t: 0, col: '#cc66ff', size: 2, spark: true })); }
                return;
            }
            if (pu.type === 'mirror') {
                ctx.G.mirrorActive = true; ctx.G.mirrorTimer = ctx.PU_DUR.mirror;
                ctx.G.activePU = { type: 'mirror', timer: ctx.PU_DUR.mirror }; ctx.G.puTimer = ctx.PU_DUR.mirror; ctx.setPUClass('mirror');
                _playPuSfx(pu.x); return;
            }
            if (pu.type === 'orbital_shield') {
                ctx.G.orbitalShields = [];
                for (let i = 0; i < 4; i++) ctx.G.orbitalShields.push({ angle: (i / 4) * Math.PI * 2, active: true });
                ctx.G.orbitalShieldTimer = 8000;
                ctx.G.activePU = { type: 'orbital_shield', timer: 8000 }; ctx.G.puTimer = 8000; ctx.setPUClass('orbital_shield');
                _playPuSfx(pu.x); return;
            }
            if (pu.type === 'chain_lightning') {
                ctx.G.activePU = { type: 'chain_lightning', timer: 10000 }; ctx.G.puTimer = 10000; ctx.setPUClass('chain_lightning');
                _playPuSfx(pu.x); return;
            }
            if (isWeaponUpgrade) {
                ctx.G.puUpgrade = ctx.PU_UPGRADE[pu.type]; ctx.G.activePU.type = ctx.G.puUpgrade; ctx.G.puTimer = ctx.PU_DUR[pu.type] || 0;
                ctx.G.upgradeBanner = { text: 'POWER UP!', type: ctx.G.puUpgrade, t: 0, dur: 1500 };
                ctx.SFX.puUpgrade(pu.x); ctx.setPUClass(ctx.G.puUpgrade); _playWeaponArm();
            } else if (pu.type === 'homing') {
                ctx.G.activePU = { type: 'homing', timer: 0, shots: 5 }; ctx.G.puTimer = 30000; ctx.setPUClass('homing');
            } else if (pu.type === 'shield') { ctx.G.shieldHits = 3; ctx.G.activePU = { type: 'shield', timer: 0 }; ctx.G.puTimer = 0; ctx.setPUClass('shield'); }
            else if (pu.type === 'pierce') {
                ctx.G.activePU = { type: ctx.G.puUpgrade === 'mega_pierce' ? 'mega_pierce' : 'pierce', timer: ctx.PU_DUR.pierce }; ctx.G.puTimer = ctx.PU_DUR.pierce; ctx.setPUClass(ctx.G.activePU.type);
            }
            else if (pu.type === 'speed' || pu.type === 'magnet' || pu.type === 'laser' || pu.type === 'timeslow' || pu.type === 'rapid' || pu.type === 'spread') {
                const upType = (isUpgradeable && isSameType) ? ctx.PU_UPGRADE[pu.type] : pu.type;
                ctx.G.activePU = { type: upType, timer: ctx.PU_DUR[pu.type] || 0 }; ctx.G.puTimer = ctx.PU_DUR[pu.type] || 0; ctx.setPUClass(upType);
                if (pu.type === 'timeslow') { ctx.G.timeScale = 0.35; ctx.G.timeSlowTimer = ctx.PU_DUR.timeslow; }
            }
            else if (pu.type === 'ricochet') {
                ctx.G.activePU = { type: ctx.G.puUpgrade === 'mega_ricochet' ? 'mega_ricochet' : 'ricochet', timer: ctx.PU_DUR.ricochet }; ctx.G.puTimer = ctx.PU_DUR.ricochet; ctx.setPUClass(ctx.G.activePU.type);
            }
            else if (pu.type === 'rocket_launcher' || pu.type === 'mine_layer') {
                const baseMatch = ctx.G.activePU && (ctx.G.activePU.type === pu.type || ctx.G.activePU.type === ctx.PU_UPGRADE[pu.type]);
                const upType = (isUpgradeable && baseMatch) ? ctx.PU_UPGRADE[pu.type] : pu.type;
                ctx.G.activePU = { type: upType, timer: ctx.PU_DUR[pu.type] }; ctx.G.puTimer = ctx.PU_DUR[pu.type]; ctx.setPUClass(upType);
                if (pu.type === 'mine_layer') ctx.G.mineDropT = 0;
                if (isUpgradeable && baseMatch && upType !== pu.type) {
                    ctx.G.upgradeBanner = { text: 'POWER UP!', type: upType, t: 0, dur: 1500 };
                }
            }
            else { ctx.G.activePU = { type: pu.type, timer: ctx.PU_DUR[pu.type] || 0 }; ctx.G.puTimer = ctx.PU_DUR[pu.type] || 0; ctx.setPUClass(pu.type); }
            if (!isWeaponUpgrade) _playPuSfx(pu.x);
            if (!isWeaponUpgrade) _playWeaponArm();
            const puCol = ctx.PU_COL[pu.type] || ctx.PU_UPGRADE_COL[pu.type];
            ctx.G.plasmaRings.push({ x: pu.x, y: pu.y, r: 0, maxR: 35, t: 0, dur: 350, col: puCol || '#ffffff' });
            for (let i = 0; i < 12; i++) {
                const a = (i / 12) * Math.PI * 2, sp = 60 + Math.random() * 40;
                ctx.G.part.push(ctx.getParticle({ x: pu.x, y: pu.y, vx: Math.cos(a) * sp, vy: Math.sin(a) * sp, life: 200 + Math.random() * 100, t: 0, col: puCol, size: 2, spark: true }));
            }
            // NEW: Rarity-scaled sparkle burst + rising glints (galaxa-fx); rare/legendary get a reverb chime
            if (ctx.fxPowerupSparkle) {
                ctx.fxPowerupSparkle(pu.x, pu.y, puCol || '#ffffff', _puRarity);
            }
        }
        function killP() {
            if (!ctx.G.p.alive) return;
            if (ctx.G.startShieldHits > 0) {
                ctx.G.startShieldHits--;
                ctx.G.damageVignetteT = 200;
                if (ctx.SFX.playerHurt) ctx.SFX.playerHurt(ctx.G.p.x);
                for (let i = 0; i < 6; i++) {
                    const a = (i / 6) * Math.PI * 2, sp = 60 + Math.random() * 40;
                    ctx.G.part.push(ctx.getParticle({ x: ctx.G.p.x, y: ctx.G.p.y, vx: Math.cos(a) * sp, vy: Math.sin(a) * sp, life: 200 + Math.random() * 100, t: 0, col: '#66ccff', size: 2, spark: true }));
                }
                if (ctx.G.startShieldHits <= 0) {
                    ctx.SFX.shieldBreak();
                    for (let i = 0; i < 16; i++) {
                        const a = Math.random() * Math.PI * 2, sp = 80 + Math.random() * 80;
                        ctx.G.part.push(ctx.getParticle({ x: ctx.G.p.x, y: ctx.G.p.y, vx: Math.cos(a) * sp, vy: Math.sin(a) * sp, life: 400 + Math.random() * 200, t: 0, col: i % 2 === 0 ? '#66ccff' : '#88ddff', size: 2, spark: true }));
                    }
                    ctx.G.shkT = 150; ctx.G.shkM = 2;
                } else { ctx.SFX.shieldHit(); }
                return;
            }
            if (ctx.G.shieldHits > 0) { ctx.G.shieldHits--; ctx.G.damageVignetteT = 300; if (ctx.SFX.playerHurt) ctx.SFX.playerHurt(ctx.G.p.x); if (ctx.G.shieldHits <= 0) { ctx.G.activePU = null; ctx.G.puTimer = 0; ctx.setPUClass(null); ctx.SFX.shieldBreak(); } else ctx.SFX.shieldHit(); return; }
ctx.G.p.alive = false; ctx.boom(ctx.G.p.x, ctx.G.p.y, false, 'player'); ctx.SFX.pExplode(ctx.G.p.x); ctx.G.shkT = 300; ctx.G.shkM = 4; ctx.G.lives--; ctx.G.stageDamageTaken = (ctx.G.stageDamageTaken || 0) + 1;
            ctx.wrapEl.classList.add('galaxa-desaturate'); setTimeout(() => { if (!ctx.state.disposed) ctx.wrapEl.classList.remove('galaxa-desaturate'); }, 800);
            ctx.G.flashT = 50; ctx.G.chromAb = 300; ctx.G.damageVignetteT = 800; ctx.G.activePU = null; ctx.G.shieldHits = 0; ctx.G.timeScale = 1; ctx.G.timeSlowTimer = 0; ctx.G.puUpgrade = null;
            // NEW: Player death flash rings (galaxa-fx)
            if (ctx.fxPlayerDeathFlash) ctx.fxPlayerDeathFlash(ctx.G.p.x, ctx.G.p.y);
            ctx.G.weaponLv = Math.max(1, ctx.G.weaponLv - 1);
            let savedCombo = 0;
            for (let i = ctx.COMBO_THRESH.length - 1; i >= 0; i--) { if (ctx.G.combo >= ctx.COMBO_THRESH[i]) { savedCombo = ctx.COMBO_THRESH[i]; break; } }
            if (savedCombo > 0) {
                ctx.G.combo = savedCombo;
                let level = 0;
                for (let i = ctx.COMBO_THRESH.length - 1; i >= 0; i--) { if (ctx.G.combo >= ctx.COMBO_THRESH[i]) { level = i + 1; break; } }
                ctx.G.comboMult = ctx.COMBO_MULT[level] || 1;
                ctx.G.scorePopups.push({ x: ctx.G.p.x, y: ctx.G.p.y - 20, text: 'COMBO SAVED!', t: 0, dur: 1000, col: '#44ff88', big: true });
            } else {
                ctx.G.combo = 0; ctx.G.comboMult = 1; ctx.G.comboBanner = null;
            }
            for (let i = 0; i < 8; i++) {
                const a = Math.random() * 6.28, sp = 30 + Math.random() * 50;
                ctx.G.deathParts.push({ x: ctx.G.p.x, y: ctx.G.p.y, vx: Math.cos(a) * sp, vy: Math.sin(a) * sp - 20, life: 800, t: 0, col: ctx.SP.pC[1 + (i % 4)] || '#fff', sz: 3 + Math.random() * 3, rot: Math.random() * 6.28 });
            }
            ctx.setPUClass(null);
            if (ctx.G.lives < 0) { ctx.G.st = 'GAME_OVER'; ctx.G.sTmr = 3000; ctx.G.contTmr = 10; ctx.G.contCnt = 10; ctx.MusicEngine.play('gameover'); }
            else { ctx.G.p.reviveTimer = 1500; }
        }
                function updateP(dt, now) {
            if (!ctx.G.p.alive) {
                if (ctx.G.p.reviveTimer > 0 && ctx.G.st === 'PLAYING') {
                    ctx.G.p.reviveTimer -= dt * 1000;
                    if (ctx.G.p.reviveTimer <= 0) { ctx.G.p.x = ctx.W / 2; ctx.G.p.y = ctx.H - 50; ctx.G.p.alive = true; ctx.G.p.inv = 3000; ctx.G.p.reviveTimer = 0; ctx.SFX.respawn(); }
                }
                return;
            }
            const inp = ctx.G.inp;
            // NEW: Parry activation (edge-triggered, with cooldown)
            if (inp.parry && !inp.parryp && ctx.G.parryCooldown <= 0 && ctx.G.parryActive <= 0 && ctx.settings.parry !== false) {
                ctx.G.parryActive = ctx.PARRY_WINDOW;
                ctx.G.parrySuccessFlash = 0;
                if (ctx.SFX.parryStart) ctx.SFX.parryStart(ctx.G.p.x);
            }
            // REMOVED: Old super activation block — new cinematic super system is activated in galaxa-game.js via ctx.startSuper()
            const baseSpd = ctx.getShipSpeed();
            const spd = ctx.G.activePU && (ctx.G.activePU.type === 'speed' || ctx.G.activePU.type === 'hyper_speed') ? baseSpd * (ctx.G.activePU.type === 'hyper_speed' ? 2.2 : 1.8) : baseSpd;
            const vspd = spd * ctx.PLAYER_VERTICAL_SPEED_MULT;
            if (inp.l) ctx.G.p.x -= spd * dt; if (inp.r) ctx.G.p.x += spd * dt;
            if (inp.u) ctx.G.p.y -= vspd * dt; if (inp.d) ctx.G.p.y += vspd * dt;
            ctx.G.p.x = Math.max(10, Math.min(ctx.W - 10, ctx.G.p.x));
            ctx.G.p.y = Math.max(ctx.PLAYER_Y_MIN, Math.min(ctx.PLAYER_Y_MAX, ctx.G.p.y));
            if (ctx.G.p.inv > 0) ctx.G.p.inv -= dt * 1000;
            if (inp.f && ctx.G.st === 'PLAYING') ctx.fire(now);
            if (ctx.G.beam && ctx.G.beam.active && ctx.G.p.x > ctx.G.beam.x - 20 && ctx.G.p.x < ctx.G.beam.x + 20 && ctx.G.p.y > ctx.G.beam.y && ctx.G.p.y < ctx.G.beam.y + ctx.G.beam.h) {
                if (ctx.G.p.alive) { ctx.killP(); ctx.G.beam.cap = true; ctx.G.beam.capT = 0; ctx.SFX.beam(); }
            }
            if (ctx.G.droneTimer > 0) {
                ctx.G.droneTimer -= dt * 1000;
                if (ctx.G.droneTimer <= 0) { ctx.G.drones = []; }
                else {
                    // OPTIMIZATION: was filter+sort on the full enemy list for
                    // every drone every 300ms. That's O(n log n) plus two full
                    // array allocations per drone. Single-pass nearest search
                    // is O(n) and allocates nothing — meaningful savings when
                    // there are 40+ enemies and 2 drones active.
                    for (const dr of ctx.G.drones) {
                        dr.x += (dr.targetX - dr.x) * dt * 5;
                        dr.y += (dr.targetY - dr.y) * dt * 5;
                        dr.fireT -= dt * 1000;
                        if (dr.fireT <= 0) {
                            const drx = dr.x, dry = dr.y;
                            let nearE = null, nearD2 = 250 * 250;
                            for (let ei = 0; ei < ctx.G.enemies.length; ei++) {
                                const e = ctx.G.enemies[ei];
                                if (e.st === 'DEAD') continue;
                                const ddx = e.x - drx, ddy = e.y - dry;
                                const d2 = ddx * ddx + ddy * ddy;
                                if (d2 < nearD2) { nearD2 = d2; nearE = e; }
                            }
                            if (nearE) {
                                const dx = nearE.x - drx, dy = nearE.y - dry, dist = Math.sqrt(dx * dx + dy * dy);
                                ctx.G.bul.push({ x: drx, y: dry - 4, w: 2, h: 4, vx: (dx / dist) * ctx.PB_SPEED * 0.5, vy: (dy / dist) * ctx.PB_SPEED * 0.5 });
                            }
                            dr.fireT = 300;
                        }
                    }
                }
            }
            if (ctx.G.blackhole) {
                ctx.G.blackhole.t += dt * 1000;
                ctx.G.blackhole.x += (ctx.G.blackhole.targetX - ctx.G.blackhole.x) * dt * 2;
                ctx.G.blackhole.y += (ctx.G.blackhole.targetY - ctx.G.blackhole.y) * dt * 2;
                for (const e of ctx.G.enemies) {
                    if (e.st === 'DEAD') continue;
                    const dx = ctx.G.blackhole.x - e.x, dy = ctx.G.blackhole.y - e.y;
                    const dist = Math.sqrt(dx * dx + dy * dy);
                    if (dist > 5 && dist < 100) { e.x += (dx / dist) * 80 * dt; e.y += (dy / dist) * 80 * dt; }
                }
                for (const b of ctx.G.ebul) {
                    const dx = ctx.G.blackhole.x - b.x, dy = ctx.G.blackhole.y - b.y;
                    const dist = Math.sqrt(dx * dx + dy * dy);
                    if (dist > 5 && dist < 80) { b.x += (dx / dist) * 100 * dt; b.y += (dy / dist) * 100 * dt; }
                }
                if (ctx.G.blackhole.t > 3000) {
                    ctx.SFX.bigExplode(ctx.G.blackhole.x);
                    const bhx = ctx.G.blackhole.x, bhy = ctx.G.blackhole.y;
                    for (const e of ctx.G.enemies) {
                        if (e.st === 'DEAD') continue;
                        const dx = e.x - bhx, dy = e.y - bhy;
                        const dist = Math.sqrt(dx * dx + dy * dy);
                        if (dist < 120) { ctx.addScore(ctx.PTS[e.type] ? ctx.PTS[e.type][0] : 100, e.x, e.y, '#8844ff'); ctx.boom(e.x, e.y, e.type === 'boss' || e.type === 'miniboss', e.type); e.st = 'DEAD'; }
                    }
                    ctx.G.flashT = 150; ctx.G.shkT = 400; ctx.G.shkM = 6;
                    ctx.G.blackhole = null;
                }
            }
            if (ctx.G.gravityBomb) {
                ctx.G.gravityBomb.t += dt * 1000;
                const gb = ctx.G.gravityBomb;
                if (gb.phase === 'pull') {
                    const gbx = gb.x, gby = gb.y;
                    for (const e of ctx.G.enemies) {
                        if (e.st === 'DEAD') continue;
                        const dx = gbx - e.x, dy = gby - e.y;
                        const dist = Math.sqrt(dx * dx + dy * dy);
                        if (dist > 5 && dist < 120) { e.x += (dx / dist) * 120 * dt; e.y += (dy / dist) * 120 * dt; }
                    }
                    if (gb.t > 2000) {
                        gb.phase = 'explode';
                        let caught = 0;
                        for (const e of ctx.G.enemies) {
                            if (e.st === 'DEAD') continue;
                            const dx = e.x - gbx, dy = e.y - gby;
                            const dist = Math.sqrt(dx * dx + dy * dy);
                            if (dist < 80) { caught++; const dmgMult = 1 + caught * 0.3; ctx.addScore(Math.floor(ctx.PTS[e.type] ? ctx.PTS[e.type][0] * dmgMult : 200 * dmgMult), e.x, e.y, '#cc66ff'); ctx.boom(e.x, e.y, e.type === 'boss' || e.type === 'miniboss', e.type); e.st = 'DEAD'; }
                        }
                        ctx.G.flashT = 150; ctx.G.shkT = 500; ctx.G.shkM = 7;
                        ctx.SFX.bigExplode(gbx);
                        for (let i = 0; i < 20; i++) { const a = (i / 20) * Math.PI * 2; ctx.G.part.push(ctx.getParticle({ x: gbx, y: gby, vx: Math.cos(a) * 100, vy: Math.sin(a) * 100, life: 400, t: 0, col: '#cc66ff', size: 3, spark: true, trail: true })); }
                        ctx.G.gravityBomb = null;
                    }
                }
            }
            if (ctx.G.mirrorActive && ctx.G.mirrorTimer > 0) {
                ctx.G.mirrorTimer -= dt * 1000;
                if (ctx.G.mirrorTimer <= 0) { ctx.G.mirrorActive = false; }
            }
            if (ctx.G.orbitalShields && ctx.G.orbitalShieldTimer > 0) {
                ctx.G.orbitalShieldTimer -= dt * 1000;
                for (const os of ctx.G.orbitalShields) { os.angle += dt * 2; }
                if (ctx.G.orbitalShieldTimer <= 0) { ctx.G.orbitalShields = null; }
            }
            if (ctx.G.stage >= 20 && ctx.G.st === 'PLAYING') {
                ctx.G.voidZoneT -= dt * 1000;
                if (ctx.G.voidZoneT <= 0) {
                    ctx.G.voidZoneT = 10000;
                    ctx.G.voidZones = [];
                    const count = 1 + Math.floor(Math.random() * 2);
                    for (let vi = 0; vi < count; vi++) {
                        ctx.G.voidZones.push({ x: 60 + Math.random() * (ctx.W - 120), y: 80 + Math.random() * (ctx.H - 200), r: 30 + Math.random() * 20, t: 0 });
                    }
                }
                if (ctx.G.voidZones) {
                    for (const vz of ctx.G.voidZones) { vz.t += dt * 1000; }
                }
            }
            if (ctx.G.activePU && ctx.G.activePU.type !== 'shield') {
                ctx.G.puTimer -= dt * 1000;
                if (ctx.G.puTimer <= 0) {
                    if (ctx.G.activePU.type === 'timeslow') { ctx.G.timeScale = 1; ctx.G.timeSlowTimer = 0; }
                    ctx.G.activePU = null; ctx.G.puUpgrade = null; ctx.setPUClass(null);
                }
            }
            if (ctx.G.timeSlowTimer > 0 && ctx.G.activePU && ctx.G.activePU.type === 'timeslow') {
                ctx.G.timeSlowTimer -= dt * 1000;
            }
            if (ctx.G.activePU && ctx.G.activePU.type === 'magnet' && ctx.G.p.alive) {
                for (const pu of ctx.G.powerups) {
                    const dx = ctx.G.p.x - pu.x, dy = ctx.G.p.y - pu.y;
                    const dist = Math.sqrt(dx * dx + dy * dy);
                    if (dist < 80 && dist > 5) { pu.x += dx / dist * 120 * dt; pu.y += dy / dist * 120 * dt; if (ctx.fxMagnetPull && Math.random() < 0.04) ctx.fxMagnetPull(pu.x, pu.y); }
                }
            }
            if (ctx.G.activePU && (ctx.G.activePU.type === 'mine_layer' || ctx.G.activePU.type === 'mega_mine_layer') && ctx.G.p.alive && ctx.G.st === 'PLAYING') {
                const dtMs = dt * 1000;
                ctx.G.mineDropT = (ctx.G.mineDropT || 0) - dtMs;
                const interval = ctx.G.activePU.type === 'mega_mine_layer' ? 520 : 780;
                if (ctx.G.mineDropT <= 0) {
                    ctx.G.mineDropT = interval;
                    const spread = ctx.G.activePU.type === 'mega_mine_layer' ? 14 : 0;
                    ctx.pushPlayerMine({ x: ctx.G.p.x - spread, y: ctx.G.p.y + 12, vy: 22, armT: 320, t: 0, r: 9 });
                    if (ctx.G.activePU.type === 'mega_mine_layer') {
                        ctx.pushPlayerMine({ x: ctx.G.p.x + spread, y: ctx.G.p.y + 12, vy: 22, armT: 320, t: 0, r: 9 });
                    }
                    if (ctx.SFX.mineDrop) ctx.SFX.mineDrop(ctx.G.p.x);
                }
            }
            // NEW: Overcharge timer decay
            if (ctx.G.overchargeTimer > 0) {
                ctx.G.overchargeTimer -= dt * 1000;
                if (ctx.G.overchargeTimer <= 0) { ctx.G.overcharge = 0; ctx.G.overchargeTimer = 0; }
            }
            // NEW: Powerup synergy detection
            if (ctx.G.activePU && ctx.G.puUpgrade) {
                const baseType = Object.keys(ctx.PU_UPGRADE).find(k => ctx.PU_UPGRADE[k] === ctx.G.activePU.type);
                if (baseType) {
                    for (const otherType of Object.keys(ctx.PU_SYNERGIES)) {
                        const [t1, t2] = otherType.split('+');
                        if ((baseType === t1 || baseType === t2) && ctx.G._synergyChecked !== otherType) {
                            // Check if we have the other powerup's effect active
                            const otherActive = (t1 === 'shield' && ctx.G.shieldHits > 0) ||
                                (t2 === 'shield' && ctx.G.shieldHits > 0) ||
                                (ctx.G.activePU && (ctx.G.activePU.type === t1 || ctx.G.activePU.type === t2 || ctx.G.activePU.type === ctx.PU_UPGRADE[t1] || ctx.G.activePU.type === ctx.PU_UPGRADE[t2]));
                            if (otherActive && baseType !== (t1 === baseType ? t2 : t1)) {
                                ctx.G._synergyChecked = otherType;
                                const syn = ctx.PU_SYNERGIES[otherType];
                                ctx.G.upgradeBanner = { text: 'SYNERGY: ' + syn.name, type: 'synergy', t: 0, dur: 2000 };
                                ctx.G.scorePopups.push({ x: ctx.G.p.x, y: ctx.G.p.y - 30, text: syn.name + '!', t: 0, dur: 1500, col: syn.col, big: true });
                                ctx.SFX.puUpgrade(ctx.G.p.x);
                                // Apply synergy effects
                                if (otherType === 'shield+magnet') ctx.G.shieldReflect = true;
                                if (otherType === 'laser+timeslow') ctx.G.laserSlow = true;
                                if (otherType === 'drone+ricochet') ctx.G.droneRicochet = true;
                            }
                        }
                    }
                }
            }
            if (ctx.G.p.alive) {
                const eg = 0.5 + Math.sin(ctx.tick * 0.15) * 0.3;
                const tRgb = ctx.G.activePU && ctx.PU_TRAIL_COL[ctx.G.activePU.type] ? ctx.PU_TRAIL_COL[ctx.G.activePU.type] : '255,150,50';
                // NEW: Super-active thrusters are doubled and tinted with super color
                const _superDef = ctx.G.superActive > 0 ? (ctx.SUPER_DEFS[ctx.G.superType] || ctx.SUPER_DEFS.classic) : null;
                const _superRgb = _superDef ? _superDef.col.replace('#', '').match(/.{2}/g).map(h => parseInt(h, 16)).join(',') : null;
                const _useRgb = _superRgb || tRgb;
                const _thrustBoost = _superDef ? 2 : 1;
                const tCol1 = 'rgba(' + _useRgb + ',' + eg + ')';
                const tCol2 = 'rgba(' + _useRgb + ',0.4)';
                const _trailCap = (ctx.settings.particles === 'low' ? 40 : ctx.settings.particles === 'medium' ? 60 : 80) * _thrustBoost;
                if (ctx.G.trails.length < _trailCap) {
                    // NEW: direction-aware thruster particles — stronger lateral thrust when moving
                    const _latVx = (inp.r ? 1 : 0) - (inp.l ? 1 : 0);
                    ctx.G.trails.push({ x: ctx.G.p.x - 8, y: ctx.G.p.y + 16, vx: (Math.random() - 0.5) * 10 + _latVx * -15, vy: 20 + Math.random() * 15, life: 150, t: 0, col: tCol1, size: 2 });
                    ctx.G.trails.push({ x: ctx.G.p.x + 4, y: ctx.G.p.y + 16, vx: (Math.random() - 0.5) * 10 + _latVx * -15, vy: 20 + Math.random() * 15, life: 150, t: 0, col: tCol1, size: 2 });
                    ctx.G.trails.push({ x: ctx.G.p.x - 5, y: ctx.G.p.y + 18, vx: (Math.random() - 0.5) * 5, vy: 15 + Math.random() * 10, life: 100, t: 0, col: tCol2, size: 1 });
                    if (ctx.G.p.dual) {
                        ctx.G.trails.push({ x: ctx.G.p.x + 36, y: ctx.G.p.y + 16, vx: (Math.random() - 0.5) * 10 + _latVx * -15, vy: 20 + Math.random() * 15, life: 150, t: 0, col: tCol1, size: 2 });
                        ctx.G.trails.push({ x: ctx.G.p.x + 44, y: ctx.G.p.y + 16, vx: (Math.random() - 0.5) * 10 + _latVx * -15, vy: 20 + Math.random() * 15, life: 150, t: 0, col: tCol1, size: 2 });
                    }
                    if (Math.abs(_latVx) > 0 && ctx.G.trails.length < _trailCap - 5) {
                        const wakeDir = _latVx > 0 ? -1 : 1;
                        ctx.G.trails.push({ x: ctx.G.p.x + wakeDir * 13, y: ctx.G.p.y + 10, vx: wakeDir * (40 + Math.random() * 30), vy: 10 + Math.random() * 10, life: 120, t: 0, col: 'rgba(255,200,100,0.3)', size: 1 });
                    }
                    // NEW: Super Nova Barrage adds extra glow trails
                    if (_superDef && ctx.G.superType === 'classic') {
                        for (let _si = 0; _si < 3; _si++) ctx.G.trails.push({ x: ctx.G.p.x + (Math.random()-0.5)*26, y: ctx.G.p.y + 13, vx: (Math.random()-0.5)*30, vy: 30 + Math.random()*20, life: 200, t: 0, col: tCol1, size: 2 });
                    }
                }
            }
            let plw = 0;
            for (let i = 0; i < ctx.G.powerups.length; i++) {
                const _pu = ctx.G.powerups[i];
                _pu.y += 60 * dt; _pu.t += dt * 1000;
                if (_pu.y > ctx.H + 20) continue;
                if (ctx.G.p.alive && ctx.hit({ x: _pu.x - 6, y: _pu.y - 6, w: 12, h: 12 }, { x: ctx.G.p.x - 8, y: ctx.G.p.y - 8, w: 16, h: 16 })) {
                    // NEW: Overcharge — reject powerup by pressing down
                    if (inp.d && _pu.type !== 'bomb' && _pu.type !== 'multibomb' && _pu.type !== 'supernova' && _pu.type !== 'levelskip' && _pu.type !== 'megabomb') {
                        ctx.G.overcharge++;
                        ctx.G.overchargeTimer = 15000; // 15s to collect another
                        if (ctx.G.overcharge >= 5) ctx.unlockAchievement('overcharge');
                        // Visual feedback
                        for (let _oi = 0; _oi < 8; _oi++) {
                            const _oa = (_oi / 8) * Math.PI * 2;
                            ctx.G.part.push(ctx.getParticle({ x: _pu.x, y: _pu.y, vx: Math.cos(_oa) * 40, vy: Math.sin(_oa) * 40 - 20, life: 300, t: 0, col: '#ffaa00', size: 2, spark: true }));
                        }
                        ctx.G.scorePopups.push({ x: _pu.x, y: _pu.y - 10, text: 'OVERCHARGE ' + ctx.G.overcharge + '/3', t: 0, dur: 1200, col: '#ffaa00', big: false });
                        ctx.SFX.puCollect(_pu.x);
                        continue;
                    }
                    ctx.collectPU(_pu); continue;
                }
                ctx.G.powerups[plw++] = _pu;
            }
            ctx.G.powerups.length = plw;
        }
        function updateBul(dt) {
            const dtMs = dt * 1000;
            ctx.updatePlayerMines(dtMs, dt);
            const hasRicochet = ctx.G.activePU && (ctx.G.activePU.type === 'ricochet' || ctx.G.activePU.type === 'mega_ricochet');
            const maxBounces = ctx.G.activePU && ctx.G.activePU.type === 'mega_ricochet' ? 4 : 2;
            let bw = 0;
            for (let i = 0; i < ctx.G.bul.length; i++) {
                const b = ctx.G.bul[i];
                if (b.homing && b.target && b.target.st !== 'DEAD') {
                    const dx = b.target.x - b.x, dy = b.target.y - b.y;
                    // OPTIMIZATION: compare squared distance against 25 to skip sqrt
                    const d2 = dx * dx + dy * dy;
                    if (d2 > 25) {
                        const dist = Math.sqrt(d2);
                        const turn = b.rocket ? 1400 : 800;
                        b.vx += (dx / dist) * turn * dt; b.vy += (dy / dist) * turn * dt;
                        const spd2 = b.vx * b.vx + b.vy * b.vy;
                        const maxSpd = b.rocket ? ctx.PB_SPEED * 1.15 : ctx.PB_SPEED * 0.8;
                        const maxSpd2 = maxSpd * maxSpd;
                        if (spd2 > maxSpd2) { const sc = maxSpd / Math.sqrt(spd2); b.vx *= sc; b.vy *= sc; }
                    }
                    if (ctx.G.trails.length < 100) {
                        ctx.G.trails.push({ x: b.x, y: b.y, vx: (Math.random() - 0.5) * 10, vy: (Math.random() - 0.5) * 10, life: b.rocket ? 220 : 150, t: 0, col: b.rocket ? 'rgba(255,120,40,0.55)' : 'rgba(255,136,170,0.5)', size: b.rocket ? 2 : 1 });
                    }
                    b.x += b.vx * dt; b.y += b.vy * dt;
                } else if (b.vx) { b.x += b.vx * dt; b.y += b.vy * dt; } else b.y -= (b.laser ? ctx.PB_SPEED * 1.5 : ctx.PB_SPEED) * dt;
                if (b.y < -10 || b.y > ctx.H + 10) continue;
                if (b.x < 0 || b.x > ctx.W) {
                    if (hasRicochet && (b.bounces || 0) < maxBounces) {
                        b.vx = -(b.vx || 0); b.x = Math.max(1, Math.min(ctx.W - 1, b.x)); b.bounces = (b.bounces || 0) + 1;
                        for (let _bi = 0; _bi < 3; _bi++) ctx.G.part.push(ctx.getParticle({ x: b.x, y: b.y, vx: (Math.random()-0.5)*40, vy: (Math.random()-0.5)*40, life: 120, t: 0, col: '#ffaa44', size: 1, spark: true }));
                    } else continue;
                }
                let removed = false;
                for (let j = ctx.G.enemies.length - 1; j >= 0; j--) {
                    const e = ctx.G.enemies[j]; if (e.st === 'DEAD') continue;
                    if (e.invulnerable) continue;
                    const ew = (e.type === 'boss' || e.type === 'miniboss') ? 32 : (e.type === 'hunter' ? 24 : e.type === 'sniper' ? 18 : 20);
                    if (ctx.hit(b, { x: e.x - ew / 2, y: e.y - 10, w: ew, h: 20 })) {
                        // Weak point check for bosses
                        if (e.weakPoint && (e.type === 'boss' || e.type === 'miniboss')) {
                            const wpx = e.x + e.weakPoint.x, wpy = e.y + e.weakPoint.y;
                            if (Math.hypot(b.x - wpx, b.y - wpy) < 6) { e.hp--; }
                        }
                        let _applyHit = true;
                        if (b._mirror && b._ghost && ctx.modesMirrorGhostDamageMult && Math.random() > ctx.modesMirrorGhostDamageMult()) {
                            _applyHit = false;
                        }
                        if (_applyHit) {
                            let _hitDmg = 1;
                            if (ctx.G.weaponEvo === 'cannon') _hitDmg = 4;
                            e.hp -= _hitDmg;
                        }
                        if (b.rocket && _applyHit) {
                            for (let sj = 0; sj < ctx.G.enemies.length; sj++) {
                                const se = ctx.G.enemies[sj];
                                if (se.st === 'DEAD' || se === e) continue;
                                if (Math.hypot(se.x - b.x, se.y - b.y) < 50) {
                                    se.hp--;
                                    se.hitF = Math.max(se.hitF || 0, 90);
                                    if (se.hp <= 0) {
                                        const spts = ctx.PTS[se.type] ? ctx.PTS[se.type][0] : 200;
                                        ctx.registerKill(); ctx.addScore(spts, se.x, se.y, '#ff7722');
                                        ctx.boom(se.x, se.y, se.type === 'boss' || se.type === 'miniboss', se.type);
                                        ctx.SFX.eExplode(se.x); ctx.dropPU(se); se.st = 'DEAD';
                                        ctx.G.killCount++; ctx.G.stageKills = (ctx.G.stageKills || 0) + 1;
                                    }
                                }
                            }
                            ctx.boom(b.x, b.y, false, 'hunter');
                            if (ctx.SFX.rocketHit) ctx.SFX.rocketHit(b.x);
                        }
                        ctx.G.stageAccuracyHits = (ctx.G.stageAccuracyHits || 0) + 1;
                        if (e.hp <= 0) {
                            const pts = ctx.PTS[e.type] ? ctx.PTS[e.type][e.st === 'DIVING' ? 1 : 0] : 200;
                            ctx.registerKill();
                            ctx.addScore(pts, e.x, e.y, e.type === 'bee' ? '#ffcc00' : e.type === 'butterfly' ? '#ff3366' : '#44cc44');
                            if (e.st === 'DIVING') {
                                ctx.G.scorePopups.push({ x: e.x, y: e.y - 16, text: 'HEADSHOT!', t: 0, dur: 800, col: '#ff8844', big: true });
                                ctx.addScore(100, e.x, e.y - 10, '#ff8844');
                                if (ctx.SFX.sparkleTwinkle) ctx.SFX.sparkleTwinkle(e.x);
                            }
                            if (ctx.G.combo > 0 && ctx.G.combo % 5 === 0) {
                                ctx.G.scorePopups.push({ x: e.x + 15, y: e.y - 16, text: 'CHAIN x' + ctx.G.combo + '!', t: 0, dur: 800, col: '#44ff88', big: false });
                            }
                            ctx.boom(e.x, e.y, e.type === 'boss' || e.type === 'miniboss', e.type, b.vx || 0, b.vy || -ctx.PB_SPEED); ctx.SFX.eExplode(e.x); ctx.dropPU(e);
                            ctx.G.credits += (e.type === 'boss' ? 10 : e.type === 'miniboss' ? 7 : 1);
                            if (ctx.G.comboMult >= 4) ctx.G.credits += 5;
                            try { localStorage.setItem('galaxa_credits', String(ctx.G.credits)); } catch (e2) {}
                            if (e.type === 'boss' || e.type === 'miniboss') { ctx.G.timeScale = 0.15; ctx.G.slowMoT = 1800; }
                            if (e.hasCap) ctx.G.p.cap = { x: e.x, y: e.y };
                            if (ctx.G.chal) ctx.G.chalHits++; e.st = 'DEAD';
                            // NEW: Splitter splits into 2 mini enemies on death
                            if (e.type === 'splitter') {
                                const _splitType = ctx.G.stage >= 15 && !(e._chained) ? 'splitter' : 'bee';
                                const _splitHP = _splitType === 'splitter' ? 1 : 1;
                                for (let _si = 0; _si < 2; _si++) {
                                    const sx = e.x + (_si === 0 ? -15 : 15);
                                    const sy = e.y - 10;
                                    ctx.G.enemies.push({ type: _splitType, r: 0, col: 0, x: sx, y: sy, fx: sx, fy: sy, hp: _splitHP, maxHp: _splitHP, st: 'DIVING', eTmr: 0, fr: 0, frT: 0, dTmr: 2000, dPath: { ph: 0, amp: 20, vx: (_si === 0 ? -40 : 40) }, sTmr: 500, shootPh: 0, hasCap: false, hitF: 0, elite: false, bossPhase: 0, bossPhaseTransition: 0, bossPhaseHP: [0,0,0], animFrame: 0, animTimer: 0, animSpeed: 120, animFrames: 4, spawnAnim: 0, spawnDur: 300, rowPhase: Math.random() * 3, bobAmp: 2, _chained: _splitType === 'splitter' });
                                }
                            }
                            // NEW: Carrier releases 3 bees on death
                            if (e.type === 'carrier') {
                                for (let _ci = 0; _ci < 3; _ci++) {
                                    const ca = (_ci / 3) * Math.PI * 2;
                                    ctx.G.enemies.push({ type: 'bee', r: 0, col: 0, x: e.x, y: e.y, fx: e.x + Math.cos(ca) * 40, fy: e.y + Math.sin(ca) * 40, hp: 1, maxHp: 1, st: 'ENTER', eTmr: 300 + _ci * 100, fr: 0, frT: 0, dTmr: 1500, dPath: null, sTmr: 800, shootPh: 0, hasCap: false, hitF: 0, elite: false, bossPhase: 0, bossPhaseTransition: 0, bossPhaseHP: [0,0,0], animFrame: 0, animTimer: 0, animSpeed: 120, animFrames: 4, spawnAnim: 0, spawnDur: 300 });
                                }
                            }
                            ctx.G.killCount++;
                            ctx.G.stageKills = (ctx.G.stageKills || 0) + 1;
                            if (ctx.G.killCount === 1) ctx.unlockAchievement('first_blood');
                            if (ctx.G.activePU && ctx.G.activePU.type === 'chain_lightning') ctx.G._chainLightningTarget = e;
                            ctx.G.weaponXP += (e.type === 'boss' ? 3 : e.type === 'miniboss' ? 2 : e.st === 'DIVING' ? 1.5 : 1);
                            // NEW: Floating combat text — damage on hit, crit on headshot/weakpoint
                            if (e.weakPoint && Math.hypot(b.x - (e.x + e.weakPoint.x), b.y - (e.y + e.weakPoint.y)) < 6) {
                                ctx.G.combatText.push({ x: e.x, y: e.y - 12, text: 'CRIT!', t: 0, dur: 600, col: '#ff4444', big: true });
                            }
                            const xpNeeded = ctx.G.weaponLv * 10;
                            if (ctx.G.weaponXP >= xpNeeded && ctx.G.weaponLv < 4) {
                                ctx.G.weaponXP -= xpNeeded;
                                ctx.G.weaponLv++;
                                ctx.SFX.weaponUp();
                                ctx.G.upgradeBanner = { text: 'W' + ctx.G.weaponLv, type: 'weapon', t: 0, dur: 1000 };
                            }
                            if (ctx.G.weaponLv >= 4 && !ctx.G.weaponEvo && !ctx.G.evoChoiceOpen) {
                                ctx.G.evoChoiceOpen = true;
                            }
                            if (ctx.G.weaponLv >= 4) ctx.unlockAchievement('weapon_master');
                            if (e.type === 'boss' || e.type === 'miniboss') { ctx.G.bossKillTotal++; if (ctx.G.bossKillTotal >= 10) ctx.unlockAchievement('boss_slayer'); try { localStorage.setItem('galaxa_boss_kills', String(ctx.G.bossKillTotal)); } catch(e2) {} }
                            const _remainingAlive = ctx.G.enemies.filter(_en => _en.st !== 'DEAD' && _en !== e).length;
                            if (_remainingAlive === 0 && e.type !== 'boss' && e.type !== 'miniboss') { ctx.G.timeScale = 0.2; ctx.G.slowMoT = 600; }
                            if (ctx.G.killCount % 10 === 0 && ctx.G.weaponLv < 4) { ctx.G.weaponLv++; ctx.SFX.weaponUp(); }
                        }                         else { e.hitF = 100; ctx.bulletImpact(b.x, b.y, '#ffee88', b.vx || 0, b.vy || -ctx.PB_SPEED); if (ctx.SFX.enemyHitSfx) ctx.SFX.enemyHitSfx(e.type, e.x); }
                        if (!b.laser && !b.pierce) { removed = true; break; }
                        if (b.laser) {
                            for (let li = 0; li < 4; li++) { const la = Math.random() * Math.PI * 2; ctx.G.part.push(ctx.getParticle({ x: e.x, y: e.y, vx: Math.cos(la) * 60, vy: Math.sin(la) * 60, life: 100 + Math.random() * 80, t: 0, col: '#aaccff', size: 1, spark: true })); }
                        }
                    }
                }
                // Near-miss rage trigger
                if (!removed) {
                    for (let j = ctx.G.enemies.length - 1; j >= 0; j--) {
                        const _re = ctx.G.enemies[j];
                        if (_re.st === 'DEAD' || _re.rageMode) continue;
                        const _nmDist = Math.hypot(b.x - _re.x, b.y - _re.y);
                        if (_nmDist < 12 && _nmDist > 4 && Math.random() < 0.15) {
                            _re.rageMode = 3000; _re.rageSpeedMult = 1.5;
                            ctx.SFX.rageMode(_re.x);
                            break;
                        }
                    }
                }
                if (!removed) ctx.G.bul[bw++] = b;
            }
            ctx.G.bul.length = bw;
            // Chain lightning on kill
            if (ctx.G._chainLightningTarget) {
                const killedE = ctx.G._chainLightningTarget;
                ctx.G._chainLightningTarget = null;
                const chainTargets = [killedE];
                // OPTIMIZATION: was chainTargets.includes(ce) — O(n) check on
                // every enemy iteration per hop, making the whole thing O(n²).
                // Set lookup is O(1) and keeps the same semantics.
                const chainSeen = new Set([killedE]);
                let lastTarget = killedE;
                for (let hop = 0; hop < 3; hop++) {
                    let nearest = null, nearDist = 120;
                    const ltx = lastTarget.x, lty = lastTarget.y;
                    for (const ce of ctx.G.enemies) {
                        if (ce.st === 'DEAD' || chainSeen.has(ce)) continue;
                        const cdx = ce.x - ltx, cdy = ce.y - lty;
                        const cd2 = cdx * cdx + cdy * cdy;
                        if (cd2 < nearDist * nearDist) { nearDist = Math.sqrt(cd2); nearest = ce; }
                    }
                    if (nearest) {
                        chainTargets.push(nearest); chainSeen.add(nearest); lastTarget = nearest; nearest.hp--; nearest.hitF = 100;
                        if (ctx.SFX.enemyHitSfx) ctx.SFX.enemyHitSfx(nearest.type, nearest.x);
                        ctx.SFX.chainLightning(hop, nearest.x);
                        const prev = chainTargets[chainTargets.length - 2];
                        for (let li = 0; li < 5; li++) {
                            const lt = li / 5;
                            ctx.G.trails.push({ x: prev.x + (nearest.x - prev.x) * lt + (Math.random() - 0.5) * 8, y: prev.y + (nearest.y - prev.y) * lt + (Math.random() - 0.5) * 8, vx: 0, vy: 0, life: 200, t: 0, col: '#aaddff', size: 1, spark: true });
                        }
                        if (nearest.hp <= 0) { ctx.addScore(ctx.PTS[nearest.type] ? ctx.PTS[nearest.type][0] : 100, nearest.x, nearest.y, '#aaddff'); ctx.boom(nearest.x, nearest.y, false, nearest.type); nearest.st = 'DEAD'; }
                    }
                }
                if (chainTargets.length >= 5) ctx.unlockAchievement('chain_master');
            }
            // Orbital shield collision with enemy bullets
            if (ctx.G.orbitalShields && ctx.G.p.alive) {
                for (let bi = ctx.G.ebul.length - 1; bi >= 0; bi--) {
                    const _ob = ctx.G.ebul[bi];
                    for (const os of ctx.G.orbitalShields) {
                        if (!os.active) continue;
                        const osx = ctx.G.p.x + Math.cos(os.angle) * 32;
                        const osy = ctx.G.p.y + Math.sin(os.angle) * 32;
                        if (Math.hypot(_ob.x - osx, _ob.y - osy) < 8) {
                            os.active = false; ctx.G.orbitalBlocks = (ctx.G.orbitalBlocks || 0) + 1;
                            ctx.SFX.orbitalShieldHit(_ob.x);
                            for (let pi = 0; pi < 6; pi++) { const pa = (pi / 6) * Math.PI * 2; ctx.G.part.push(ctx.getParticle({ x: osx, y: osy, vx: Math.cos(pa) * 40, vy: Math.sin(pa) * 40, life: 200, t: 0, col: '#44aaff', size: 2, spark: true })); }
                            ctx.G.ebul.splice(bi, 1); break;
                        }
                    }
                }
            }
            const eDt = dt * ctx.G.timeScale;
            const ebSpd = ctx.EB_SPEED * ctx.diffMod('ebSpd');
            const origELen = ctx.G.ebul.length;
            let ew = 0;
            for (let i = 0; i < origELen; i++) {
                const b = ctx.G.ebul[i];
                b.t = (b.t || 0) + dtMs;
                if (b.kind === 'mine') {
                    b.y += (b.vy || ebSpd * 0.2) * eDt;
                    b.x += (b.vx || 0) * eDt;
                    if (b.fuse !== undefined) b.fuse -= dtMs;
                    const nearP = ctx.G.p.alive && Math.hypot(ctx.G.p.x - b.x, ctx.G.p.y - b.y) < 36;
                    if (b.fuse !== undefined && b.fuse <= 0 || nearP) {
                        for (let mi = 0; mi < 6; mi++) { const ma = (mi / 6) * Math.PI * 2; ctx.G.ebul.push({ x: b.x, y: b.y, w: 2, h: 3, vx: Math.cos(ma) * ebSpd * 0.35, vy: Math.sin(ma) * ebSpd * 0.35, kind: 'spiral' }); }
                        ctx.bulletImpact(b.x, b.y, '#cc66ff');
                        continue;
                    }
                } else if (b.vx !== undefined || b.vy !== undefined) {
                    b.x += (b.vx || 0) * eDt;
                    b.y += (b.vy || 0) * eDt;
                } else {
                    b.y += ebSpd * eDt;
                }
                if (b.y > ctx.H + 14 || b.y < -14 || b.x < -14 || b.x > ctx.W + 14) continue;
                // Ricochet world mutation
                if (ctx.G.ricochetWorld && (b.x < 0 || b.x > ctx.W)) { b.vx = -(b.vx || 0); b.x = Math.max(1, Math.min(ctx.W - 1, b.x)); }
                // Mirror field mutation
                if (ctx.G.mirrorField && Math.random() < 0.2 && origELen > 0) { ctx.G.ebul.push({ x: b.x, y: b.y, w: b.w || 2, h: b.h || 4, vx: -(b.vx || 0), vy: b.vy || 0, kind: b.kind }); }
                // Gravity well mutation
                if (ctx.G.gravityWell) { const _gbx = ctx.W / 2 - b.x, _gby = ctx.H / 3 - b.y, _gbd = Math.sqrt(_gbx * _gbx + _gby * _gby); if (_gbd > 20) { b.x += (_gbx / _gbd) * 30 * eDt; b.y += (_gby / _gbd) * 30 * eDt; } }
                if (ctx.G.p.alive && ctx.G.p.inv <= 0 && ctx.hit(b, { x: ctx.G.p.x - 8, y: ctx.G.p.y - 8, w: 16, h: 16 })) { ctx.killP(); continue; }
                // NEW: Parry deflection — if parry active and bullet within parry radius, reflect it back
                if (ctx.G.p.alive && ctx.G.parryActive > 0) {
                    const _pdx = b.x - ctx.G.p.x, _pdy = b.y - ctx.G.p.y;
                    const _pdist = Math.sqrt(_pdx * _pdx + _pdy * _pdy);
                    if (_pdist < ctx.PARRY_RADIUS) {
                        // Reflect bullet back toward nearest enemy (or straight up).
                        // OPTIMIZATION: cache p.x/p.y outside the inner nearest-search
                        // loop (previously reread on every enemy).
                        let _tx = b.x, _ty = b.y - 100;
                        let _nearE = null, _nearD2 = Infinity;
                        const _ppx = ctx.G.p.x, _ppy = ctx.G.p.y;
                        for (let _pei = 0; _pei < ctx.G.enemies.length; _pei++) {
                            const _pe = ctx.G.enemies[_pei];
                            if (_pe.st === 'DEAD') continue;
                            const _pex = _pe.x - _ppx, _pey = _pe.y - _ppy;
                            const _d2 = _pex * _pex + _pey * _pey;
                            if (_d2 < _nearD2) { _nearD2 = _d2; _nearE = _pe; }
                        }
                        if (_nearE) { _tx = _nearE.x; _ty = _nearE.y; }
                        const _dx = _tx - b.x, _dy = _ty - b.y;
                        const _dd = Math.sqrt(_dx * _dx + _dy * _dy) || 1;
                        b.vx = (_dx / _dd) * ctx.EB_SPEED * 1.2; b.vy = (_dy / _dd) * ctx.EB_SPEED * 1.2; b.kind = 'bolt'; b._parried = true;
                        ctx.G.parryActive = 0; ctx.G.parryCooldown = ctx.PARRY_COOLDOWN;
                        ctx.G.parryCount = (ctx.G.parryCount || 0) + 1;
                        ctx.G.parrySuccessFlash = 200;
                        ctx.G.hitstopT = Math.max(ctx.G.hitstopT, 60);
                        ctx.G.combo = (ctx.G.combo || 0) + 1; ctx.G.comboTimer = ctx.getComboTimeout();
                        ctx.addScore(300, b.x, b.y - 10, '#ffffff');
                        ctx.G.combatText.push({ x: b.x, y: b.y - 14, text: 'PARRY!', t: 0, dur: 700, col: '#ffffff', big: true });
                        if (ctx.SFX.parrySuccess) ctx.SFX.parrySuccess(ctx.G.p.x);
                        if (ctx.fxParryRing) ctx.fxParryRing(ctx.G.p.x, ctx.G.p.y);
                        if (ctx.isGameMode && ctx.isGameMode('mirror') && ctx.SFX.mirrorPing) ctx.SFX.mirrorPing(ctx.G.p.x);
                        if (ctx.fxBulletTime) ctx.fxBulletTime();
                        if (ctx.SFX.bulletTimeEnter) ctx.SFX.bulletTimeEnter();
                        ctx.duckMusic(0.25, 200);
                        for (let _pi = 0; _pi < 12; _pi++) { const _pa = (_pi / 12) * Math.PI * 2; ctx.G.part.push(ctx.getParticle({ x: ctx.G.p.x, y: ctx.G.p.y, vx: Math.cos(_pa) * 80, vy: Math.sin(_pa) * 80, life: 300, t: 0, col: '#ffffff', size: 2, spark: true })); }
                        if (ctx.G.parryCount >= 50) ctx.unlockAchievement('parry_master');
                        continue;
                    }
                }
                // NEW: Danger-close bonus — near miss detection
                if (ctx.G.p.alive && ctx.G.p.inv <= 0 && !ctx.G._closeCallCooldown) {
                    const _cdx = ctx.G.p.x - b.x, _cdy = ctx.G.p.y - b.y;
                    const _cdist = Math.hypot(_cdx, _cdy);
                    if (_cdist < 24 && _cdist > 10) {
                        ctx.G._closeCallCooldown = 500;
                        ctx.addScore(500, ctx.G.p.x, ctx.G.p.y - 10, '#ffaa00');
                        ctx.G.scorePopups.push({ x: ctx.G.p.x, y: ctx.G.p.y - 20, text: 'CLOSE CALL!', t: 0, dur: 1000, col: '#ffaa00', big: true });
                        ctx.SFX.closeCall(ctx.G.p.x);
                    }
                }
                ctx.G.ebul[ew++] = b;
            }
            for (let i = origELen; i < ctx.G.ebul.length; i++) ctx.G.ebul[ew++] = ctx.G.ebul[i];
            ctx.G.ebul.length = ew;
        }
        ctx.fire = fire;
        ctx.boom = boom;
        ctx.bulletImpact = bulletImpact;
        ctx.addScore = addScore;
        ctx.updateCombo = updateCombo;
        ctx.getComboTimeout = getComboTimeout;
        ctx.registerKill = registerKill;
        ctx.hit = hit;
        ctx.dropPU = dropPU;
        ctx.collectPU = collectPU;
        ctx.killP = killP;
        ctx.updateP = updateP;
        ctx.updateBul = updateBul;
    };
})();
