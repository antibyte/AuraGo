(function () {
    'use strict';
    const GC = window.GalaxaCore = window.GalaxaCore || {};
    GC.createEntities = function (ctx) {
        let lastFireT = 0;
        const particlePool = [];
        function getParticle(props) {
            const p = particlePool.length > 0 ? particlePool.pop() : {};
            Object.assign(p, props);
            return p;
        }
        function recycleParticles(arr) {
            for (let i = 0; i < arr.length; i++) {
                if (particlePool.length < 300) { const p = arr[i]; for (const k in p) delete p[k]; particlePool.push(p); }
            }
            arr.length = 0;
        }
        function mkFormation() {
            ctx.G.enemies = []; ctx.G.chal = ctx.isChal(ctx.G.stage); ctx.G.chalHits = 0; ctx.G.chalTot = 0;
            ctx.G.bossWarningShown = false;
            const isMini = ctx.isMiniBossStage();
            const formType = (ctx.G.stage - 1) % 6;
            let idx = 0;

            function pushEnemy(type, r, col, fx, fy, hp) {
                const side = idx % 2 === 0 ? -1 : 1;
                const diveDelay = ctx.G.chal ? (800 + idx * 200) : (1000 + Math.random() * 3000 + idx * 50);
                const animSpeedMap = { bee: 150, butterfly: 120, hunter: 90, boss: 220, miniboss: 220, kamikaze: 70, stalker: 130, sniper: 160, spinner: 100, bomber: 180, lasher: 140, weaver: 110, splitter: 130, shield_bee: 150, carrier: 200, teleporter: 100 };
                const animFramesMap = { bee: 4, butterfly: 4, hunter: 4, boss: 3, miniboss: 3, kamikaze: 4, stalker: 4, sniper: 4, spinner: 4, bomber: 4, lasher: 4, weaver: 4, splitter: 4, shield_bee: 4, carrier: 4, teleporter: 4 };
                const enemy = { type, r, col, x: ctx.W / 2 + side * (120 + Math.random() * 80), y: -30 - (idx % 8) * 20,
                    fx, fy, hp, maxHp: hp, st: 'ENTER', eTmr: 500 + idx * 80 + r * 100,
                    fr: 0, frT: 0, dTmr: diveDelay / ctx.diffMod('diveRate'), dPath: null,
                    sTmr: (type === 'spinner' || type === 'bomber' || type === 'lasher') ? 800 + Math.random() * 1200 : 0,
                    shootPh: 0, hasCap: false, hitF: 0, elite: type === 'hunter',
                    bossPhase: (type === 'boss' || type === 'miniboss') ? 1 : 0,
                    bossPhaseTransition: 0, bossPhaseHP: [0.6, 0.3, 0],
                    animFrame: 0, animTimer: 0, animSpeed: animSpeedMap[type] || 120, animFrames: animFramesMap[type] || 3,
                    spawnAnim: 0, spawnDur: 400, rowPhase: r * 1.2 + col * 0.3, bobAmp: 2.5 + r * 0.5,
                    weakPoint: (type === 'boss' || type === 'miniboss') ? { x: 0, y: -10, angle: 0 } : null,
                    rageMode: 0, rageSpeedMult: 1, phaseTimer: 0 };
                ctx.G.enemies.push(enemy);
                idx++;
            }

            if (isMini) {
                const mbHP = 4 + Math.floor(ctx.G.stage / 5);
                pushEnemy('miniboss', 0, 4, ctx.W / 2, ctx.FTOP, mbHP);
            }
            if (ctx.settings.mode === 'boss_rush') {
                const bossHP = 3 + Math.floor(ctx.G.stage * 0.8);
                pushEnemy('boss', 0, 4, ctx.W / 2, ctx.FTOP, bossHP);
                ctx.G.chalTot = ctx.G.enemies.length;
                ctx.G.dTmr = 500 / ctx.diffMod('diveRate');
                ctx.G.fX = 0; ctx.mkNebula(); ctx.initBG();
                return;
            }

            for (let r = 0; r < ctx.FROWS; r++) for (let col = 0; col < ctx.FCOLS; col++) {
                let type = 'bee';
                if (r === 0) { if (col < 3 || col > 6) continue; if (!isMini) type = 'boss'; }
                else if (r <= 2) type = 'butterfly';

                let fx, fy;
                const cx = ctx.W / 2, cy = ctx.FTOP;

                if (formType === 0) {
                    fx = cx + (col - ctx.FCOLS / 2 + 0.5) * ctx.ESP_X;
                    fy = cy + r * ctx.ESP_Y;
                } else if (formType === 1) {
                    const vDepth = r * (1 - Math.abs(col - ctx.FCOLS / 2) / (ctx.FCOLS / 2)) ;
                    fx = cx + (col - ctx.FCOLS / 2 + 0.5) * ctx.ESP_X;
                    fy = cy + r * ctx.ESP_Y * 0.7 + vDepth * 12;
                } else if (formType === 2) {
                    const angle = -0.6 + (col / (ctx.FCOLS - 1)) * 1.2;
                    const radius = 100 + r * ctx.ESP_Y;
                    fx = cx + Math.sin(angle) * radius;
                    fy = cy + r * 20 + (1 - Math.cos(angle)) * 40;
                } else if (formType === 3) {
                    const zig = (col % 2 === 0 ? 1 : -1) * r * 12;
                    fx = cx + (col - ctx.FCOLS / 2 + 0.5) * ctx.ESP_X + zig;
                    fy = cy + r * ctx.ESP_Y;
                } else if (formType === 4) {
                    const diamondR = Math.abs(col - ctx.FCOLS / 2 + 0.5) / (ctx.FCOLS / 2);
                    fx = cx + (col - ctx.FCOLS / 2 + 0.5) * ctx.ESP_X * (1 + diamondR * 0.3);
                    fy = cy + r * ctx.ESP_Y * (1 - diamondR * 0.4);
                } else {
                    const heartT = col / (ctx.FCOLS - 1) * Math.PI * 2;
                    const heartX = 16 * Math.pow(Math.sin(heartT), 3);
                    const heartY = -(13 * Math.cos(heartT) - 5 * Math.cos(2 * heartT) - 2 * Math.cos(3 * heartT) - Math.cos(4 * heartT));
                    fx = cx + heartX * 3.5 + (r - 2) * 4;
                    fy = cy + 30 + heartY * 3 + r * 8;
                    if (fy < ctx.FTOP) fy = ctx.FTOP + Math.abs(fy - ctx.FTOP);
                }

                const bossHP = ctx.G.stage >= 5 ? 2 + Math.floor((ctx.G.stage - 5) / 4) : (ctx.settings.mode === 'endless' ? 2 + Math.floor(ctx.G.stage / 3) : 2);
                const enemyHP = type === 'boss' ? bossHP : 1;
                pushEnemy(type, r, col, fx, fy, enemyHP);
            }
            if (ctx.settings.mode === 'endless') {
                const scale = 1 + (ctx.G.stage - 1) * 0.1;
                for (const e of ctx.G.enemies) { e.hp = Math.ceil(e.hp * scale); e.maxHp = e.hp; }
            }
            if (!isMini && !ctx.G.chal) {
                if (ctx.G.stage >= 4) {
                    const stalkerCount = Math.min(3, Math.floor((ctx.G.stage - 3) / 2));
                    for (let si = 0; si < stalkerCount; si++) {
                        const sfx = ctx.W / 2 + (si - stalkerCount / 2 + 0.5) * ctx.ESP_X;
                        pushEnemy('stalker', 1, 8, sfx, ctx.FTOP + ctx.ESP_Y, 1);
                    }
                }
                if (ctx.G.stage >= 6) {
                    const sniperCount = Math.min(2, Math.floor((ctx.G.stage - 5) / 3));
                    for (let si = 0; si < sniperCount; si++) {
                        const sfx = ctx.W / 2 + (si % 2 === 0 ? -1 : 1) * (ctx.ESP_X * 2 + si * ctx.ESP_X * 0.5);
                        pushEnemy('sniper', 0, 4, sfx, ctx.FTOP, 1);
                    }
                }
                const eliteChance = 0.22 + Math.min(ctx.G.stage, 12) * 0.035;
                if (ctx.G.stage >= 2 && Math.random() < eliteChance) {
                    const hunterHP = 2 + Math.floor(ctx.G.stage / 3);
                    pushEnemy('hunter', 0, 5, ctx.W / 2 + (Math.random() - 0.5) * 100, ctx.FTOP + ctx.ESP_Y * 0.5, hunterHP);
                }
                if (ctx.G.stage >= 3 && Math.random() < 0.38) {
                    pushEnemy('spinner', 2, 3, ctx.W / 2 + (Math.random() - 0.5) * 130, ctx.FTOP + ctx.ESP_Y * 2, 2);
                }
                if (ctx.G.stage >= 4 && Math.random() < 0.32) {
                    pushEnemy('bomber', 1, 6, ctx.W / 2 + (Math.random() - 0.5) * 110, ctx.FTOP + ctx.ESP_Y, 2);
                }
                if (ctx.G.stage >= 5 && Math.random() < 0.28) {
                    pushEnemy('lasher', 0, 2, ctx.W / 2 + (Math.random() - 0.5) * 70, ctx.FTOP, 1);
                }
                // NEW: Additional enemy types at higher stages
                if (ctx.G.stage >= 4 && Math.random() < 0.2) {
                    pushEnemy('shield_bee', 1, 4, ctx.W / 2 + (Math.random() - 0.5) * 100, ctx.FTOP + ctx.ESP_Y, 2);
                }
                if (ctx.G.stage >= 6 && Math.random() < 0.18) {
                    pushEnemy('kamikaze', 0, 2, ctx.W / 2 + (Math.random() - 0.5) * 80, ctx.FTOP, 1);
                }
                if (ctx.G.stage >= 7 && Math.random() < 0.22) {
                    pushEnemy('weaver', 1, 5, ctx.W / 2 + (Math.random() - 0.5) * 120, ctx.FTOP + ctx.ESP_Y, 1);
                }
                if (ctx.G.stage >= 8 && Math.random() < 0.16) {
                    pushEnemy('splitter', 2, 3, ctx.W / 2 + (Math.random() - 0.5) * 100, ctx.FTOP + ctx.ESP_Y * 2, 2);
                }
                if (ctx.G.stage >= 9 && Math.random() < 0.14) {
                    pushEnemy('carrier', 0, 4, ctx.W / 2 + (Math.random() - 0.5) * 90, ctx.FTOP, 3);
                }
                if (ctx.G.stage >= 10 && Math.random() < 0.12) {
                    pushEnemy('teleporter', 1, 6, ctx.W / 2 + (Math.random() - 0.5) * 110, ctx.FTOP + ctx.ESP_Y, 2);
                }
            }
            ctx.G.chalTot = ctx.G.enemies.length;
            ctx.G.dTmr = (2000 - Math.min(ctx.G.stage * 100, 1200)) / ctx.diffMod('diveRate');
            ctx.G.fX = 0;
            ctx.mkNebula(); ctx.initBG();
            if (isMini) ctx.SFX.miniBossWarning();
        }
        function fire(now) {
            // REMOVED: Old super effects branches — bursts are now triggered by galaxa-supers.js via triggerBurst() during superPhase==='burst'
            if (ctx.G.activePU && (ctx.G.activePU.type === 'laser' || ctx.G.activePU.type === 'mega_laser')) {
                const cd = ctx.G.activePU.type === 'mega_laser' ? 200 : 300;
                if (now - lastFireT < cd) return;
                lastFireT = now;
                ctx.G.bul.push({ x: ctx.G.p.x, y: ctx.G.p.y - 8, w: ctx.G.activePU.type === 'mega_laser' ? 6 : 4, h: 14, vx: 0, vy: -ctx.PB_SPEED * 1.5, laser: true });
                if (ctx.G.p.dual) ctx.G.bul.push({ x: ctx.G.p.x + 36, y: ctx.G.p.y - 8, w: ctx.G.activePU.type === 'mega_laser' ? 6 : 4, h: 14, vx: 0, vy: -ctx.PB_SPEED * 1.5, laser: true });
                ctx.SFX.laserShoot(ctx.G.p.x);
                return;
            }
            const isUltraRapid = ctx.G.activePU && ctx.G.activePU.type === 'ultra_rapid';
            const isRapid = ctx.G.activePU && ctx.G.activePU.type === 'rapid';
            const cd = isUltraRapid ? 80 : isRapid ? 120 : 250;
            if (now - lastFireT < cd) return;
            lastFireT = now;
            const isPierce = ctx.G.activePU && (ctx.G.activePU.type === 'pierce' || ctx.G.activePU.type === 'mega_pierce');
            const isHoming = ctx.G.activePU && ctx.G.activePU.type === 'homing';
            const isMegaSpread = ctx.G.activePU && ctx.G.activePU.type === 'mega_spread';
            const isSpread = ctx.G.activePU && ctx.G.activePU.type === 'spread';
            if (isHoming && ctx.G.activePU.shots > 0) {
                const nearestE = ctx.G.enemies.filter(e => e.st !== 'DEAD').sort((a2, b2) => {
                    const da = Math.hypot(a2.x - ctx.G.p.x, a2.y - ctx.G.p.y);
                    const db = Math.hypot(b2.x - ctx.G.p.x, b2.y - ctx.G.p.y);
                    return da - db;
                })[0];
                if (nearestE) {
                    const dx = nearestE.x - ctx.G.p.x, dy = nearestE.y - ctx.G.p.y;
                    const dist = Math.hypot(dx, dy);
                    ctx.G.bul.push({ x: ctx.G.p.x, y: ctx.G.p.y - 8, w: 3, h: 6, vx: (dx / dist) * ctx.PB_SPEED * 0.7, vy: (dy / dist) * ctx.PB_SPEED * 0.7, homing: true, target: nearestE });
                    ctx.G.activePU.shots--;
                    ctx.SFX.homingLock(ctx.G.p.x);
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
            ctx.SFX.shoot(ctx.G.p.x); ctx.G.muzzleT = 50; ctx.G.stageAccuracyShots = (ctx.G.stageAccuracyShots || 0) + 1;
            if (ctx.G.mirrorActive) {
                const mirrorX = ctx.W - ctx.G.p.x;
                const mirrorBullets = [];
                for (let bi = ctx.G.bul.length - 1; bi >= 0; bi--) {
                    const b = ctx.G.bul[bi];
                    if (b._mirror) continue;
                    mirrorBullets.push({ x: mirrorX + (ctx.G.p.x - b.x), y: b.y, w: b.w, h: b.h, vx: b.vx ? -b.vx : 0, vy: b.vy, laser: b.laser, pierce: b.pierce, homing: b.homing, target: b.target, _mirror: true });
                }
                for (const mb of mirrorBullets) ctx.G.bul.push(mb);
            }
        }
        function boom(x, y, isBoss, enemyType, killVx, killVy) {
            // NEW: Use per-enemy explosion profile for layered explosion intensity
            const _prof = (enemyType && ctx.EXPLOSION_PROFILE[enemyType]) || ctx.EXPLOSION_PROFILE.bee;
            const dur = isBoss ? 900 : 450;
            const pCount = isBoss ? _prof.debris * 3 : _prof.debris;
            const sparkCount = isBoss ? _prof.sparks * 2 : _prof.sparks;
            const debrisCount = isBoss ? _prof.debris * 2 : _prof.debris;
            const smokeCount = isBoss ? _prof.smoke * 2 : _prof.smoke;
            const flashCount = isBoss ? 8 : 4;
            ctx.G.exp.push({ x, y, t: 0, dur, seed: Math.random(), isBoss });
            if (isBoss) {
                ctx.G.exp.push({ x, y, t: 0, dur: 700, seed: Math.random(), isBoss: false, shockwave: true });
                ctx.G.exp.push({ x, y, t: 0, dur: 180, seed: Math.random(), isBoss: false, flash: true });
                for (let _bi = 0; _bi < 16; _bi++) { const _ba = (_bi / 16) * Math.PI * 2; const _bsp = 40 + Math.random() * 60; ctx.G.part.push(getParticle({ x, y, vx: Math.cos(_ba) * _bsp, vy: Math.sin(_ba) * _bsp, life: 800, t: 0, col: '#ffcc44', size: 2, spark: true, trail: true, bloom: true, bloomPhase: 0 })); }
                ctx.SFX.deathBloom();
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
            if (isBoss) { ctx.G.hitstopT = Math.max(ctx.G.hitstopT, 120); ctx.duckMusic(0.5, 600); }
            for (let i = 0; i < pCount; i++) {
                const a = (i / pCount) * Math.PI * 2 + Math.random() * 0.8, sp = 60 + (i * 23 % 160) * (isBoss ? 2 : 1.2);
                const cols = fireCols[i % fireCols.length];
                const sz = i % 4 === 0 ? 4 : i % 3 === 0 ? 3 : 2;
                const shapes = ['rect', 'diamond', 'circle', 'star'];
                const shape = Math.random() < 0.2 ? shapes[1 + Math.floor(Math.random() * 3)] : 'rect';
                ctx.G.part.push(getParticle({ x, y, vx: Math.cos(a) * sp, vy: Math.sin(a) * sp, life: (280 + (i * 41 % 280)) * (isBoss ? 1.6 : 1.1), t: 0, col: cols, size: sz, shape }));
            }
            // NEW: Type-specific death effects
            if (enemyType === 'bee') {
                for (let i = 0; i < 6; i++) { const a = Math.random() * Math.PI * 2; ctx.G.part.push({ x, y, vx: Math.cos(a) * 30, vy: Math.sin(a) * 30 - 20, life: 400, t: 0, col: '#ffcc00', size: 2, spark: true }); }
            } else if (enemyType === 'butterfly') {
                for (let i = 0; i < 8; i++) { const a = (i / 8) * Math.PI * 2; ctx.G.part.push({ x, y, vx: Math.cos(a) * 50, vy: Math.sin(a) * 50, life: 300, t: 0, col: '#ff3366', size: 1, spark: true }); }
            } else if (enemyType === 'stalker') {
                ctx.G.exp.push({ x, y, t: 0, dur: 400, seed: Math.random(), isBoss: false, implosion: true, col: '#8844cc' });
            } else if (enemyType === 'hunter') {
                for (let i = 0; i < 10; i++) { const a = Math.random() * Math.PI * 2; ctx.G.part.push({ x, y, vx: Math.cos(a) * 70, vy: Math.sin(a) * 70, life: 350, t: 0, col: '#ff6600', size: 3, debris: true, rot: Math.random() * 6.28 }); }
            } else if (enemyType === 'spinner') {
                ctx.G.plasmaRings.push({ x, y, r: 0, maxR: 40, t: 0, dur: 300, col: '#44ffff' });
            } else if (enemyType === 'bomber') {
                for (let i = 0; i < 3; i++) { ctx.G.pendingBooms.push({ x: x + (Math.random() - 0.5) * 30, y: y + (Math.random() - 0.5) * 20, isBoss: false, delay: i * 80 }); }
            } else if (enemyType === 'lasher') {
                ctx.G.flashT = Math.max(ctx.G.flashT, 50);
                for (let i = 0; i < 6; i++) { const a = (i / 6) * Math.PI * 2; ctx.G.part.push({ x, y, vx: Math.cos(a) * 60, vy: Math.sin(a) * 60, life: 200, t: 0, col: '#44ff88', size: 2, spark: true }); }
            } else if (enemyType === 'kamikaze') {
                ctx.G.shkT = Math.max(ctx.G.shkT, 300); ctx.G.shkM = Math.max(ctx.G.shkM, 5);
                ctx.G.exp.push({ x, y, t: 0, dur: 300, seed: Math.random(), isBoss: false, flash: true });
            } else if (enemyType === 'carrier') {
                for (let i = 0; i < 3; i++) { ctx.G.pendingBooms.push({ x: x + (Math.random() - 0.5) * 40, y: y + (Math.random() - 0.5) * 30, isBoss: false, delay: i * 150 }); }
            } else if (enemyType === 'teleporter') {
                for (let i = 0; i < 12; i++) { const a = (i / 12) * Math.PI * 2; ctx.G.part.push({ x, y, vx: Math.cos(a) * 80, vy: Math.sin(a) * 80, life: 250, t: 0, col: '#44ffff', size: 1, spark: true }); }
            }
            for (let i = 0; i < sparkCount; i++) {
                const a = Math.random() * Math.PI * 2, sp = 90 + Math.random() * 150;
                ctx.G.part.push(getParticle({ x, y, vx: Math.cos(a) * sp, vy: Math.sin(a) * sp, life: 120 + Math.random() * 180, t: 0, col: Math.random() > 0.5 ? '#ffffff' : '#ffeeaa', size: 1, spark: true, shape: 'diamond' }));
            }
            for (let i = 0; i < debrisCount; i++) {
                const hasKillDir = killVx !== undefined && killVy !== undefined;
                const baseA = hasKillDir ? Math.atan2(killVy, killVx) + (Math.random() - 0.5) * 1.5 : Math.random() * Math.PI * 2;
                const sp = 25 + Math.random() * 50;
                const sz = isBoss ? 3 + Math.random() * 4 : 2 + Math.random() * 3;
                ctx.G.part.push(getParticle({ x, y, vx: Math.cos(baseA) * sp + (killVx || 0) * 0.15, vy: Math.sin(baseA) * sp + (killVy || 0) * 0.15 - 18, life: 600 + Math.random() * 500, t: 0, col: isBoss ? '#999' : '#777', size: sz, debris: true, rot: Math.random() * 6.28 }));
            }
            for (let i = 0; i < smokeCount; i++) {
                const a = Math.random() * Math.PI * 2, sp = 12 + Math.random() * 25;
                ctx.G.part.push(getParticle({ x, y, vx: Math.cos(a) * sp, vy: Math.sin(a) * sp - 15, life: 600 + Math.random() * 500, t: 0, col: Math.random() > 0.5 ? '#666' : '#555', size: 3 + (isBoss ? 3 : 0), smoke: true, shape: 'circle' }));
            }
            for (let i = 0; i < flashCount; i++) {
                const a = Math.random() * Math.PI * 2, sp = 40 + Math.random() * 80;
                ctx.G.part.push(getParticle({ x, y, vx: Math.cos(a) * sp, vy: Math.sin(a) * sp, life: 60 + Math.random() * 40, t: 0, col: '#ffffff', size: 2, spark: true, trail: true, shape: 'star' }));
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
        function bulletImpact(x, y, col) {
            for (let i = 0; i < 4; i++) {
                const a = Math.random() * Math.PI * 2, sp = 30 + Math.random() * 50;
                ctx.G.part.push({ x, y, vx: Math.cos(a) * sp, vy: Math.sin(a) * sp, life: 80 + Math.random() * 60, t: 0, col: col || '#ffff88', size: 1, spark: true });
            }
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
            if (pu.type === 'bomb' || pu.type === 'multibomb') {
                ctx.SFX.bomb(pu.x);
                const bonus = pu.type === 'multibomb' ? 500 : 0;
                for (const e of ctx.G.enemies) {
                    if (e.st !== 'DEAD') {
                        if (e.type === 'boss' || e.type === 'miniboss') { e.enraged = true; e.hitF = 200; }
                        else { ctx.addScore(ctx.PTS[e.type][0] + bonus, e.x, e.y, ctx.PU_COL[pu.type]); ctx.boom(e.x, e.y, false, e.type); e.st = 'DEAD'; }
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
                        else { ctx.addScore(ctx.PTS[e.type][0] + 1000, e.x, e.y, '#fff'); ctx.boom(e.x, e.y, false, e.type); e.st = 'DEAD'; }
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
                        ctx.G.part.push({ x: e.x + Math.cos(_fa) * 14, y: e.y + Math.sin(_fa) * 14, vx: Math.cos(_fa) * 35, vy: Math.sin(_fa) * 35 - 10, life: 500, t: 0, col: '#88eeff', size: 2 });
                        ctx.G.part.push({ x: e.x, y: e.y, vx: (Math.random()-0.5)*40, vy: -20-Math.random()*30, life: 350, t: 0, col: '#ccf4ff', size: 1, spark: true });
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
            if (pu.type === 'drone') {
                const count = (isUpgradeable && isSameType) ? 2 : 1;
                ctx.G.drones = [];
                for (let di = 0; di < count; di++) ctx.G.drones.push({ x: ctx.G.p.x + (di === 0 ? -20 : 20), y: ctx.G.p.y - 20, targetX: ctx.G.p.x + (di === 0 ? -25 : 25), targetY: ctx.G.p.y - 30, fireT: 0 });
                ctx.G.droneTimer = ctx.PU_DUR.drone; ctx.G.activePU = { type: count > 1 ? 'dual_drone' : 'drone', timer: ctx.PU_DUR.drone }; ctx.G.puTimer = ctx.PU_DUR.drone; ctx.setPUClass(count > 1 ? 'dual_drone' : 'drone');
                ctx.SFX.puCollect(pu.x); return;
            }
            if (pu.type === 'blackhole_bomb') {
                ctx.G.blackhole = { x: ctx.G.p.x, y: ctx.G.p.y - 60, targetX: ctx.G.p.x, targetY: ctx.G.p.y - 120, t: 0 };
                ctx.SFX.bomb(pu.x); ctx.G.activePU = null; ctx.G.puTimer = 0; ctx.setPUClass(null);
                for (let i = 0; i < 8; i++) { const a = (i / 8) * Math.PI * 2; ctx.G.part.push(getParticle({ x: ctx.G.p.x, y: ctx.G.p.y - 60, vx: Math.cos(a) * 50, vy: Math.sin(a) * 50, life: 300, t: 0, col: '#8844ff', size: 2, spark: true })); }
                return;
            }
            if (pu.type === 'gravity_bomb') {
                ctx.G.gravityBomb = { x: ctx.G.p.x, y: ctx.G.p.y - 80, t: 0, phase: 'pull' };
                ctx.SFX.bomb(pu.x); ctx.G.activePU = null; ctx.G.puTimer = 0; ctx.setPUClass(null);
                for (let i = 0; i < 12; i++) { const a = (i / 12) * Math.PI * 2; ctx.G.part.push(getParticle({ x: ctx.G.p.x, y: ctx.G.p.y - 80, vx: Math.cos(a) * 60, vy: Math.sin(a) * 60, life: 400, t: 0, col: '#cc66ff', size: 2, spark: true })); }
                return;
            }
            if (pu.type === 'mirror') {
                ctx.G.mirrorActive = true; ctx.G.mirrorTimer = ctx.PU_DUR.mirror;
                ctx.G.activePU = { type: 'mirror', timer: ctx.PU_DUR.mirror }; ctx.G.puTimer = ctx.PU_DUR.mirror; ctx.setPUClass('mirror');
                ctx.SFX.puCollect(pu.x); return;
            }
            if (pu.type === 'orbital_shield') {
                ctx.G.orbitalShields = [];
                for (let i = 0; i < 4; i++) ctx.G.orbitalShields.push({ angle: (i / 4) * Math.PI * 2, active: true });
                ctx.G.orbitalShieldTimer = 8000;
                ctx.G.activePU = { type: 'orbital_shield', timer: 8000 }; ctx.G.puTimer = 8000; ctx.setPUClass('orbital_shield');
                ctx.SFX.puCollect(pu.x); return;
            }
            if (pu.type === 'chain_lightning') {
                ctx.G.activePU = { type: 'chain_lightning', timer: 10000 }; ctx.G.puTimer = 10000; ctx.setPUClass('chain_lightning');
                ctx.SFX.puCollect(pu.x); return;
            }
            if (isUpgradeable && isSameType && !ctx.G.puUpgrade) {
                ctx.G.puUpgrade = ctx.PU_UPGRADE[pu.type]; ctx.G.activePU.type = ctx.G.puUpgrade; ctx.G.puTimer = ctx.PU_DUR[pu.type] || 0;
                ctx.G.upgradeBanner = { text: 'POWER UP!', type: ctx.G.puUpgrade, t: 0, dur: 1500 };
                ctx.SFX.puUpgrade(pu.x); ctx.setPUClass(ctx.G.puUpgrade);
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
            else { ctx.G.activePU = { type: pu.type, timer: ctx.PU_DUR[pu.type] || 0 }; ctx.G.puTimer = ctx.PU_DUR[pu.type] || 0; ctx.setPUClass(pu.type); }
            ctx.G.collectedPU.add(pu.type); if (ctx.G.collectedPU.size >= ctx.PU_TYPES.length) ctx.unlockAchievement('power_collector');
            ctx.SFX.puCollect(pu.x);
            const puCol = ctx.PU_COL[pu.type] || ctx.PU_UPGRADE_COL[pu.type];
            ctx.G.plasmaRings.push({ x: pu.x, y: pu.y, r: 0, maxR: 35, t: 0, dur: 350, col: puCol || '#ffffff' });
            for (let i = 0; i < 12; i++) {
                const a = (i / 12) * Math.PI * 2, sp = 60 + Math.random() * 40;
                ctx.G.part.push({ x: pu.x, y: pu.y, vx: Math.cos(a) * sp, vy: Math.sin(a) * sp, life: 200 + Math.random() * 100, t: 0, col: puCol, size: 2, spark: true });
            }
        }
        function killP() {
            if (!ctx.G.p.alive) return;
            if (ctx.G.shieldHits > 0) { ctx.G.shieldHits--; ctx.G.damageVignetteT = 300; if (ctx.G.shieldHits <= 0) { ctx.G.activePU = null; ctx.G.puTimer = 0; ctx.setPUClass(null); ctx.SFX.shieldBreak(); } else ctx.SFX.shieldHit(); return; }
ctx.G.p.alive = false; ctx.boom(ctx.G.p.x, ctx.G.p.y, false, 'player'); ctx.SFX.pExplode(ctx.G.p.x); ctx.G.shkT = 300; ctx.G.shkM = 4; ctx.G.lives--; ctx.G.stageDamageTaken = (ctx.G.stageDamageTaken || 0) + 1;
            ctx.wrapEl.classList.add('galaxa-desaturate'); setTimeout(() => { if (!ctx.state.disposed) ctx.wrapEl.classList.remove('galaxa-desaturate'); }, 800);
            ctx.G.flashT = 50; ctx.G.chromAb = 300; ctx.G.damageVignetteT = 800; ctx.G.activePU = null; ctx.G.shieldHits = 0; ctx.G.timeScale = 1; ctx.G.timeSlowTimer = 0; ctx.G.puUpgrade = null;
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
                    for (const dr of ctx.G.drones) {
                        dr.x += (dr.targetX - dr.x) * dt * 5;
                        dr.y += (dr.targetY - dr.y) * dt * 5;
                        dr.fireT -= dt * 1000;
                        if (dr.fireT <= 0) {
                            const nearE = ctx.G.enemies.filter(e => e.st !== 'DEAD').sort((a, b) => Math.hypot(a.x - dr.x, a.y - dr.y) - Math.hypot(b.x - dr.x, b.y - dr.y))[0];
                            if (nearE && Math.hypot(nearE.x - dr.x, nearE.y - dr.y) < 250) {
                                const dx = nearE.x - dr.x, dy = nearE.y - dr.y, dist = Math.hypot(dx, dy);
                                ctx.G.bul.push({ x: dr.x, y: dr.y - 4, w: 2, h: 4, vx: (dx / dist) * ctx.PB_SPEED * 0.5, vy: (dy / dist) * ctx.PB_SPEED * 0.5 });
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
                    const dx = ctx.G.blackhole.x - e.x, dy = ctx.G.blackhole.y - e.y, dist = Math.hypot(dx, dy);
                    if (dist > 5 && dist < 100) { e.x += (dx / dist) * 80 * dt; e.y += (dy / dist) * 80 * dt; }
                }
                for (const b of ctx.G.ebul) {
                    const dx = ctx.G.blackhole.x - b.x, dy = ctx.G.blackhole.y - b.y, dist = Math.hypot(dx, dy);
                    if (dist > 5 && dist < 80) { b.x += (dx / dist) * 100 * dt; b.y += (dy / dist) * 100 * dt; }
                }
                if (ctx.G.blackhole.t > 3000) {
                    ctx.SFX.bigExplode(ctx.G.blackhole.x);
                    for (const e of ctx.G.enemies) {
                        if (e.st === 'DEAD') continue;
                        const dist = Math.hypot(e.x - ctx.G.blackhole.x, e.y - ctx.G.blackhole.y);
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
                    for (const e of ctx.G.enemies) {
                        if (e.st === 'DEAD') continue;
                        const dx = gb.x - e.x, dy = gb.y - e.y, dist = Math.hypot(dx, dy);
                        if (dist > 5 && dist < 120) { e.x += (dx / dist) * 120 * dt; e.y += (dy / dist) * 120 * dt; }
                    }
                    if (gb.t > 2000) {
                        gb.phase = 'explode';
                        let caught = 0;
                        for (const e of ctx.G.enemies) {
                            if (e.st === 'DEAD') continue;
                            const dist = Math.hypot(e.x - gb.x, e.y - gb.y);
                            if (dist < 80) { caught++; const dmgMult = 1 + caught * 0.3; ctx.addScore(Math.floor(ctx.PTS[e.type] ? ctx.PTS[e.type][0] * dmgMult : 200 * dmgMult), e.x, e.y, '#cc66ff'); ctx.boom(e.x, e.y, e.type === 'boss' || e.type === 'miniboss', e.type); e.st = 'DEAD'; }
                        }
                        ctx.G.flashT = 150; ctx.G.shkT = 500; ctx.G.shkM = 7;
                        ctx.SFX.bigExplode(gb.x);
                        for (let i = 0; i < 20; i++) { const a = (i / 20) * Math.PI * 2; ctx.G.part.push(getParticle({ x: gb.x, y: gb.y, vx: Math.cos(a) * 100, vy: Math.sin(a) * 100, life: 400, t: 0, col: '#cc66ff', size: 3, spark: true, trail: true })); }
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
                    if (dist < 80 && dist > 5) { pu.x += dx / dist * 120 * dt; pu.y += dy / dist * 120 * dt; }
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
                    if (inp.d && _pu.type !== 'bomb' && _pu.type !== 'multibomb' && _pu.type !== 'supernova' && _pu.type !== 'levelskip') {
                        ctx.G.overcharge++;
                        ctx.G.overchargeTimer = 15000; // 15s to collect another
                        if (ctx.G.overcharge >= 5) ctx.unlockAchievement('overcharge');
                        // Visual feedback
                        for (let _oi = 0; _oi < 8; _oi++) {
                            const _oa = (_oi / 8) * Math.PI * 2;
                            ctx.G.part.push({ x: _pu.x, y: _pu.y, vx: Math.cos(_oa) * 40, vy: Math.sin(_oa) * 40 - 20, life: 300, t: 0, col: '#ffaa00', size: 2, spark: true });
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
            const hasRicochet = ctx.G.activePU && (ctx.G.activePU.type === 'ricochet' || ctx.G.activePU.type === 'mega_ricochet');
            const maxBounces = ctx.G.activePU && ctx.G.activePU.type === 'mega_ricochet' ? 4 : 2;
            let bw = 0;
            for (let i = 0; i < ctx.G.bul.length; i++) {
                const b = ctx.G.bul[i];
                if (b.homing && b.target && b.target.st !== 'DEAD') {
                    const dx = b.target.x - b.x, dy = b.target.y - b.y;
                    const dist = Math.hypot(dx, dy);
                    if (dist > 5) {
                        b.vx += (dx / dist) * 800 * dt; b.vy += (dy / dist) * 800 * dt;
                        const spd = Math.hypot(b.vx, b.vy);
                        const maxSpd = ctx.PB_SPEED * 0.8;
                        if (spd > maxSpd) { b.vx *= maxSpd / spd; b.vy *= maxSpd / spd; }
                    }
                    if (ctx.G.trails.length < 100) {
                        ctx.G.trails.push({ x: b.x, y: b.y, vx: (Math.random() - 0.5) * 10, vy: (Math.random() - 0.5) * 10, life: 150, t: 0, col: 'rgba(255,136,170,0.5)', size: 1 });
                    }
                    b.x += b.vx * dt; b.y += b.vy * dt;
                } else if (b.vx) { b.x += b.vx * dt; b.y += b.vy * dt; } else b.y -= (b.laser ? ctx.PB_SPEED * 1.5 : ctx.PB_SPEED) * dt;
                if (b.y < -10 || b.y > ctx.H + 10) continue;
                if (b.x < 0 || b.x > ctx.W) {
                    if (hasRicochet && (b.bounces || 0) < maxBounces) {
                        b.vx = -(b.vx || 0); b.x = Math.max(1, Math.min(ctx.W - 1, b.x)); b.bounces = (b.bounces || 0) + 1;
                        for (let _bi = 0; _bi < 3; _bi++) ctx.G.part.push({ x: b.x, y: b.y, vx: (Math.random()-0.5)*40, vy: (Math.random()-0.5)*40, life: 120, t: 0, col: '#ffaa44', size: 1, spark: true });
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
                        e.hp--;
                        ctx.G.stageAccuracyHits = (ctx.G.stageAccuracyHits || 0) + 1;
                        if (e.hp <= 0) {
                            const pts = ctx.PTS[e.type] ? ctx.PTS[e.type][e.st === 'DIVING' ? 1 : 0] : 200;
                            ctx.registerKill();
                            ctx.addScore(pts, e.x, e.y, e.type === 'bee' ? '#ffcc00' : e.type === 'butterfly' ? '#ff3366' : '#44cc44');
                            if (e.st === 'DIVING') {
                                ctx.G.scorePopups.push({ x: e.x, y: e.y - 16, text: 'HEADSHOT!', t: 0, dur: 800, col: '#ff8844', big: true });
                                ctx.addScore(100, e.x, e.y - 10, '#ff8844');
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
                        }                         else e.hitF = 100;
                        if (!b.laser && !b.pierce) { removed = true; break; }
                        if (b.laser) {
                            for (let li = 0; li < 4; li++) { const la = Math.random() * Math.PI * 2; ctx.G.part.push(getParticle({ x: e.x, y: e.y, vx: Math.cos(la) * 60, vy: Math.sin(la) * 60, life: 100 + Math.random() * 80, t: 0, col: '#aaccff', size: 1, spark: true })); }
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
                let chainTargets = [killedE];
                let lastTarget = killedE;
                for (let hop = 0; hop < 3; hop++) {
                    let nearest = null, nearDist = 120;
                    for (const ce of ctx.G.enemies) { if (ce.st === 'DEAD' || chainTargets.includes(ce)) continue; const cdist = Math.hypot(ce.x - lastTarget.x, ce.y - lastTarget.y); if (cdist < nearDist) { nearDist = cdist; nearest = ce; } }
                    if (nearest) {
                        chainTargets.push(nearest); lastTarget = nearest; nearest.hp--; nearest.hitF = 100;
                        ctx.SFX.chainLightning(hop, nearest.x);
                        for (let li = 0; li < 5; li++) { const lt = li / 5; ctx.G.trails.push({ x: chainTargets[chainTargets.length - 2].x + (nearest.x - chainTargets[chainTargets.length - 2].x) * lt + (Math.random() - 0.5) * 8, y: chainTargets[chainTargets.length - 2].y + (nearest.y - chainTargets[chainTargets.length - 2].y) * lt + (Math.random() - 0.5) * 8, vx: 0, vy: 0, life: 200, t: 0, col: '#aaddff', size: 1, spark: true }); }
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
                            for (let pi = 0; pi < 6; pi++) { const pa = (pi / 6) * Math.PI * 2; ctx.G.part.push({ x: osx, y: osy, vx: Math.cos(pa) * 40, vy: Math.sin(pa) * 40, life: 200, t: 0, col: '#44aaff', size: 2, spark: true }); }
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
                if (ctx.G.gravityWell) { const _gbx = ctx.W / 2 - b.x, _gby = ctx.H / 3 - b.y, _gbd = Math.hypot(_gbx, _gby); if (_gbd > 20) { b.x += (_gbx / _gbd) * 30 * eDt; b.y += (_gby / _gbd) * 30 * eDt; } }
                if (ctx.G.p.alive && ctx.G.p.inv <= 0 && ctx.hit(b, { x: ctx.G.p.x - 8, y: ctx.G.p.y - 8, w: 16, h: 16 })) { ctx.killP(); continue; }
                // NEW: Parry deflection — if parry active and bullet within parry radius, reflect it back
                if (ctx.G.p.alive && ctx.G.parryActive > 0) {
                    const _pdx = b.x - ctx.G.p.x, _pdy = b.y - ctx.G.p.y;
                    const _pdist = Math.hypot(_pdx, _pdy);
                    if (_pdist < ctx.PARRY_RADIUS) {
                        // Reflect bullet back toward nearest enemy (or straight up)
                        let _tx = b.x, _ty = b.y - 100;
                        let _nearE = null, _nearD = Infinity;
                        for (const _pe of ctx.G.enemies) { if (_pe.st === 'DEAD') continue; const _d = Math.hypot(_pe.x - ctx.G.p.x, _pe.y - ctx.G.p.y); if (_d < _nearD) { _nearD = _d; _nearE = _pe; } }
                        if (_nearE) { _tx = _nearE.x; _ty = _nearE.y; }
                        const _dx = _tx - b.x, _dy = _ty - b.y, _dd = Math.hypot(_dx, _dy) || 1;
                        b.vx = (_dx / _dd) * ctx.EB_SPEED * 1.2; b.vy = (_dy / _dd) * ctx.EB_SPEED * 1.2; b.kind = 'bolt'; b._parried = true;
                        ctx.G.parryActive = 0; ctx.G.parryCooldown = ctx.PARRY_COOLDOWN;
                        ctx.G.parryCount = (ctx.G.parryCount || 0) + 1;
                        ctx.G.parrySuccessFlash = 200;
                        ctx.G.hitstopT = Math.max(ctx.G.hitstopT, 60);
                        ctx.G.combo = (ctx.G.combo || 0) + 1; ctx.G.comboTimer = ctx.getComboTimeout();
                        ctx.addScore(300, b.x, b.y - 10, '#ffffff');
                        ctx.G.combatText.push({ x: b.x, y: b.y - 14, text: 'PARRY!', t: 0, dur: 700, col: '#ffffff', big: true });
                        if (ctx.SFX.parrySuccess) ctx.SFX.parrySuccess(ctx.G.p.x);
                        ctx.duckMusic(0.25, 200);
                        for (let _pi = 0; _pi < 12; _pi++) { const _pa = (_pi / 12) * Math.PI * 2; ctx.G.part.push({ x: ctx.G.p.x, y: ctx.G.p.y, vx: Math.cos(_pa) * 80, vy: Math.sin(_pa) * 80, life: 300, t: 0, col: '#ffffff', size: 2, spark: true }); }
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
        function enemyFire(e) {
            if (ctx.G.chal || ctx.G.st !== 'PLAYING') return;
            const spd = ctx.EB_SPEED * ctx.diffMod('ebSpd');
            const px = e.x, py = e.y + 8;
            // NEW: Boss phase-based attack patterns
            if ((e.type === 'boss' || e.type === 'miniboss') && e.bossPhase) {
                const ebSpd = spd;
                switch (e.bossPhase) {
                    case 1:
                        ctx.G.ebul.push({ x: px, y: py, w: 2, h: 6 });
                        if (ctx.G.stage >= 5) { ctx.G.ebul.push({ x: px - 8, y: py, w: 2, h: 6 }); ctx.G.ebul.push({ x: px + 8, y: py, w: 2, h: 6 }); }
                        break;
                    case 2:
                        // Spread shot + aimed burst
                        ctx.G.ebul.push(...ctx.ATTACK_PATTERNS.aimed_burst(e, 5, 0.55, 0, ebSpd, ctx.G.p.x, ctx.G.p.y));
                        ctx.G.ebul.push(...ctx.ATTACK_PATTERNS.random_spread(e, 4, 0.4, 0.8, ebSpd));
                        ctx.SFX.hunterShot(e.x);
                        break;
                    case 3:
                        // Bullet hell: spiral + circle + wall
                        ctx.G.ebul.push(...ctx.ATTACK_PATTERNS.spiral(e, 12, 0.35, 0, ebSpd));
                        ctx.G.ebul.push(...ctx.ATTACK_PATTERNS.circle(e, 8, 0.3, ebSpd));
                        if (Math.random() < 0.5) ctx.G.ebul.push(...ctx.ATTACK_PATTERNS.wall(e, 3, 5, 0.25, ebSpd));
                        ctx.SFX.spinnerShot(e.x);
                        ctx.G.shkT = Math.max(ctx.G.shkT, 200); ctx.G.shkM = Math.max(ctx.G.shkM, 3);
                        break;
                }
                return;
            }
            switch (e.type) {
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
                    if (ctx.G.stage >= 8 && e.type === 'boss') { for (let k = 0; k < 3; k++) setTimeout(() => { if (!ctx.state.disposed && e.st === 'DIVING') ctx.G.ebul.push({ x: e.x, y: e.y + 8, w: 2, h: 6 }); }, k * 150); }
                    if (e.type === 'miniboss') { ctx.G.ebul.push({ x: px - 10, y: py, w: 2, h: 6 }); ctx.G.ebul.push({ x: px + 10, y: py, w: 2, h: 6 }); for (let k = 0; k < 2; k++) setTimeout(() => { if (!ctx.state.disposed && e.st === 'DIVING') { ctx.G.ebul.push({ x: e.x - 6, y: e.y + 8, w: 2, h: 6 }); ctx.G.ebul.push({ x: e.x + 6, y: e.y + 8, w: 2, h: 6 }); } }, k * 180); }
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
                            if ((e.type === 'boss' || e.type === 'miniboss') && !ctx.G.bossWarningShown) { ctx.G.bossWarningT = 2000; ctx.G.bossWarningShown = true; if (e.type === 'miniboss') ctx.SFX.miniBossWarning(); else ctx.SFX.bossWarning(); }
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
        function spawnHazards() {
            ctx.G.envHazards = [];
            ctx.G.solarFlareT = 0; ctx.G.solarFlareActive = false; ctx.G.emStormT = 0;
            const theme = ctx.G.bgTheme;
            if (theme === 'asteroid') {
                for (let i = 0; i < 4; i++) {
                    ctx.G.envHazards.push({ type: 'asteroid_h', x: 40 + Math.random() * (ctx.W - 80), y: 80 + Math.random() * (ctx.H - 200), hp: 2, maxHp: 2, r: 8 + Math.random() * 6, vx: (Math.random() - 0.5) * 20, vy: 8 + Math.random() * 12, rot: Math.random() * 6.28, rotSpd: (Math.random() - 0.5) * 2 });
                }
            } else if (theme === 'nebula' && ctx.G.stage >= 8) {
                ctx.G.solarFlareT = 5000 + Math.random() * 3000;
            } else if (theme === 'crystal') {
                for (let i = 0; i < 3; i++) {
                    ctx.G.envHazards.push({ type: 'crystal_h', x: 60 + Math.random() * (ctx.W - 120), y: 100 + Math.random() * 200, r: 5, t: 0, collected: false });
                }
            } else if (theme === 'storm') {
                ctx.G.emStormT = 8000 + Math.random() * 5000;
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

        ctx.mkFormation = mkFormation;
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
        ctx.enemyFire = enemyFire;
        ctx.diveRateMult = diveRateMult;
        ctx.startDive = startDive;
        ctx.updateE = updateE;
        ctx.startChalDive = startChalDive;
        ctx.updateP = updateP;
        ctx.updateBul = updateBul;
        ctx.spawnHazards = spawnHazards;
        ctx.updateHazards = updateHazards;
        ctx.getParticle = getParticle;
        ctx.recycleParticles = recycleParticles;

        const WEAPON_EVOS = {
            vulcan: { name: 'VULCAN', desc: 'Ultra-fast stream', col: '#ff8844', fireRate: 0.6, spread: 0, dmgMult: 0.7 },
            cannon: { name: 'CANNON', desc: 'Slow massive shots', col: '#ff4444', fireRate: 2.5, spread: 0, dmgMult: 4 },
            beam: { name: 'BEAM', desc: 'Continuous laser', col: '#88ccff', fireRate: 0, spread: 0, dmgMult: 0, isBeam: true }
        };
        ctx.WEAPON_EVOS = WEAPON_EVOS;

        let evoSel = 0;
        function updateEvoChoice() {
            const u = ctx.G.inp.u && !ctx.G.inp.up;
            const d = ctx.G.inp.d && !ctx.G.inp.dp;
            const f = ctx.G.inp.f && !ctx.G.inp.fp;
            if (u) evoSel = Math.max(0, evoSel - 1);
            if (d) evoSel = Math.min(2, evoSel + 1);
            if (f) {
                const evos = ['vulcan', 'cannon', 'beam'];
                ctx.G.weaponEvo = evos[evoSel];
                ctx.G.evoChoiceOpen = false;
                ctx.SFX.puUpgrade(ctx.W / 2);
                ctx.G.upgradeBanner = { text: WEAPON_EVOS[ctx.G.weaponEvo].name + '!', type: 'evolution', t: 0, dur: 2000 };
            }
        }
        ctx.updateEvoChoice = updateEvoChoice;
        ctx.evoSel = function() { return evoSel; };
    };
})();
