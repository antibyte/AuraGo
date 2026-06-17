(function () {
    'use strict';
    const GC = window.GalaxaCore = window.GalaxaCore || {};
    GC.createEntities = function (ctx) {
        let lastFireT = 0;
        function mkFormation() {
            ctx.G.enemies = []; ctx.G.chal = ctx.isChal(ctx.G.stage); ctx.G.chalHits = 0; ctx.G.chalTot = 0;
            ctx.G.bossWarningShown = false;
            const isMini = ctx.isMiniBossStage();
            const formType = (ctx.G.stage - 1) % 6;
            let idx = 0;

            function pushEnemy(type, r, col, fx, fy, hp) {
                const side = idx % 2 === 0 ? -1 : 1;
                const diveDelay = ctx.G.chal ? (800 + idx * 200) : (1000 + Math.random() * 3000 + idx * 50);
                const enemy = { type, r, col, x: ctx.W / 2 + side * (120 + Math.random() * 80), y: -30 - (idx % 8) * 20,
                    fx, fy, hp, maxHp: hp, st: 'ENTER', eTmr: 500 + idx * 80 + r * 100,
                    fr: 0, frT: 0, dTmr: diveDelay / ctx.diffMod('diveRate'), dPath: null,
                    sTmr: (type === 'spinner' || type === 'bomber' || type === 'lasher') ? 800 + Math.random() * 1200 : 0,
                    shootPh: 0, hasCap: false, hitF: 0, elite: type === 'hunter',
                    // NEW: Boss phase system
                    bossPhase: (type === 'boss' || type === 'miniboss') ? 1 : 0,
                    bossPhaseTransition: 0, bossPhaseHP: [0.6, 0.3, 0],
                    // NEW: Sprite animation system
                    animFrame: 0, animTimer: 0, animSpeed: 120, animFrames: 3 };
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
            if (ctx.G.activePU && (ctx.G.activePU.type === 'laser' || ctx.G.activePU.type === 'mega_laser')) {
                const cd = ctx.G.activePU.type === 'mega_laser' ? 200 : 300;
                if (now - lastFireT < cd) return;
                lastFireT = now;
                ctx.G.bul.push({ x: ctx.G.p.x, y: ctx.G.p.y - 8, w: ctx.G.activePU.type === 'mega_laser' ? 6 : 4, h: 14, vx: 0, vy: -ctx.PB_SPEED * 1.5, laser: true });
                if (ctx.G.p.dual) ctx.G.bul.push({ x: ctx.G.p.x + 28, y: ctx.G.p.y - 8, w: ctx.G.activePU.type === 'mega_laser' ? 6 : 4, h: 14, vx: 0, vy: -ctx.PB_SPEED * 1.5, laser: true });
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
                    if (ctx.G.p.dual) ctx.G.bul.push({ x: ctx.G.p.x + 28, y: ctx.G.p.y - 8, w: 2, h: 6, vx: 0, vy: -ctx.PB_SPEED, pierce: isPierce });
                }
                if (lv >= 4 && !isRapid && !isUltraRapid) {
                    ctx.G.bul.push({ x: ctx.G.p.x, y: ctx.G.p.y - 8, w: 2, h: 6, vx: -Math.sin(0.2) * ctx.PB_SPEED * 0.2, vy: -ctx.PB_SPEED, pierce: isPierce });
                    ctx.G.bul.push({ x: ctx.G.p.x, y: ctx.G.p.y - 8, w: 2, h: 6, vx: Math.sin(0.2) * ctx.PB_SPEED * 0.2, vy: -ctx.PB_SPEED, pierce: isPierce });
                }
            }
            ctx.SFX.shoot(ctx.G.p.x); ctx.G.muzzleT = 50;
        }
        function boom(x, y, isBoss, enemyType) {
            const dur = isBoss ? 900 : 450;
            const pCount = isBoss ? 50 : 20;
            const sparkCount = isBoss ? 24 : 10;
            const debrisCount = isBoss ? 14 : 6;
            const smokeCount = isBoss ? 14 : 7;
            const flashCount = isBoss ? 8 : 4;
            ctx.G.exp.push({ x, y, t: 0, dur, seed: Math.random(), isBoss });
            if (isBoss) { ctx.G.exp.push({ x, y, t: 0, dur: 700, seed: Math.random(), isBoss: false, shockwave: true }); ctx.G.exp.push({ x, y, t: 0, dur: 180, seed: Math.random(), isBoss: false, flash: true }); }
            else { ctx.G.exp.push({ x, y, t: 0, dur: 100, seed: Math.random(), isBoss: false, flash: true }); ctx.G.exp.push({ x, y, t: 0, dur: 300, seed: Math.random(), isBoss: false, shockwave: true }); }
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
            const fireCols = (enemyType && typeCols[enemyType]) ? typeCols[enemyType] : ['#ffcc00', '#ff4444', '#ff8800', '#fff', '#ffee88', '#ff6622', '#ffaa00'];
            for (let i = 0; i < pCount; i++) {
                const a = (i / pCount) * Math.PI * 2 + Math.random() * 0.8, sp = 60 + (i * 23 % 160) * (isBoss ? 2 : 1.2);
                const cols = fireCols[i % fireCols.length];
                const sz = i % 4 === 0 ? 4 : i % 3 === 0 ? 3 : 2;
                ctx.G.part.push({ x, y, vx: Math.cos(a) * sp, vy: Math.sin(a) * sp, life: (280 + (i * 41 % 280)) * (isBoss ? 1.6 : 1.1), t: 0, col: cols, size: sz });
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
                ctx.G.part.push({ x, y, vx: Math.cos(a) * sp, vy: Math.sin(a) * sp, life: 120 + Math.random() * 180, t: 0, col: Math.random() > 0.5 ? '#ffffff' : '#ffeeaa', size: 1, spark: true });
            }
            for (let i = 0; i < debrisCount; i++) {
                const a = Math.random() * Math.PI * 2, sp = 25 + Math.random() * 50;
                const sz = isBoss ? 3 + Math.random() * 4 : 2 + Math.random() * 3;
                ctx.G.part.push({ x, y, vx: Math.cos(a) * sp, vy: Math.sin(a) * sp - 18, life: 600 + Math.random() * 500, t: 0, col: isBoss ? '#999' : '#777', size: sz, debris: true, rot: Math.random() * 6.28 });
            }
            for (let i = 0; i < smokeCount; i++) {
                const a = Math.random() * Math.PI * 2, sp = 12 + Math.random() * 25;
                ctx.G.part.push({ x, y, vx: Math.cos(a) * sp, vy: Math.sin(a) * sp - 15, life: 600 + Math.random() * 500, t: 0, col: Math.random() > 0.5 ? '#666' : '#555', size: 3 + (isBoss ? 3 : 0), smoke: true });
            }
            for (let i = 0; i < flashCount; i++) {
                const a = Math.random() * Math.PI * 2, sp = 40 + Math.random() * 80;
                ctx.G.part.push({ x, y, vx: Math.cos(a) * sp, vy: Math.sin(a) * sp, life: 60 + Math.random() * 40, t: 0, col: '#ffffff', size: 2, spark: true });
            }
            if (isBoss) {
                for (let i = 0; i < 6; i++) {
                    ctx.G.pendingBooms.push({ x: x + (Math.random() - 0.5) * 50, y: y + (Math.random() - 0.5) * 40, isBoss: false, delay: i * 100 });
                }
                ctx.G.shkT = Math.max(ctx.G.shkT, 800); ctx.G.shkM = Math.max(ctx.G.shkM, 7);
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
            const multiplied = pts * ctx.G.comboMult * (ctx.G.scoreMult || 1);
            ctx.G.score += multiplied;
            if (ctx.G.score > ctx.G.hi) ctx.G.hi = ctx.G.score;
            const text = ctx.G.comboMult > 1 ? '+' + multiplied + ' x' + ctx.G.comboMult : '+' + multiplied;
            if (x !== undefined) ctx.G.scorePopups.push({ x, y, text, t: 0, dur: 800, col: col || '#ffcc00', big: ctx.G.comboMult > 1 });
            if (Math.floor(ctx.G.score / ctx.EXTRA_LIFE) > Math.floor(prev / ctx.EXTRA_LIFE)) { ctx.G.lives++; ctx.SFX.extra(); }
        }
        function updateCombo(dtMs) {
            if (ctx.G.comboTimer > 0) {
                ctx.G.comboTimer -= dtMs || 16;
                if (ctx.G.comboTimer <= 0) { if (ctx.G.combo > 2) ctx.SFX.comboBreak(); ctx.G.combo = 0; ctx.G.comboMult = 1; ctx.G.comboBanner = null; }
            }
        }
        function registerKill() {
            ctx.G.combo++;
            ctx.G.comboTimer = ctx.COMBO_TIMEOUT;
            if (ctx.G.combo >= 15) ctx.unlockAchievement('combo_king');
            let level = 0;
            for (let i = ctx.COMBO_THRESH.length - 1; i >= 0; i--) { if (ctx.G.combo >= ctx.COMBO_THRESH[i]) { level = i + 1; break; } }
            ctx.G.comboMult = ctx.COMBO_MULT[level] || 1;
            if (level > 0 && ctx.COMBO_TEXT[level]) {
                ctx.G.comboBanner = { text: ctx.COMBO_TEXT[level], mult: ctx.G.comboMult, t: 0, dur: 1200 };
                ctx.SFX.combo(level);
                if (level >= 4) ctx.SFX.killStreak();
            }
        }
        function hit(a, b) { return a.x < b.x + b.w && a.x + a.w > b.x && a.y < b.y + b.h && a.y + a.h > b.y; }
        function dropPU(e) {
            let chance = e.type === 'miniboss' ? 1 : (e.type === 'boss' ? 0.35 : (e.type === 'bee' && !ctx.diffMod('puFromBee') ? 0 : 0.12));
            // NEW: Power Surge modifier triples drop chance
            if (ctx.G.powerSurge) chance *= 3;
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
                for (const e of ctx.G.enemies) { if (e.st !== 'DEAD') { ctx.addScore(ctx.PTS[e.type][0] + bonus, e.x, e.y, ctx.PU_COL[pu.type]); ctx.boom(e.x, e.y, e.type === 'boss', e.type); e.st = 'DEAD'; } }
                ctx.G.flashT = 100; ctx.G.activePU = null; ctx.G.puTimer = 0; ctx.setPUClass(null); return;
            }
            if (pu.type === 'supernova') {
                ctx.SFX.supernova(pu.x);
                for (const e of ctx.G.enemies) { if (e.st !== 'DEAD') { ctx.addScore(ctx.PTS[e.type][0] + 1000, e.x, e.y, '#fff'); ctx.boom(e.x, e.y, e.type === 'boss', e.type); e.st = 'DEAD'; } }
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
                for (let i = 0; i < 8; i++) { const a = (i / 8) * Math.PI * 2; ctx.G.part.push({ x: ctx.G.p.x, y: ctx.G.p.y - 60, vx: Math.cos(a) * 50, vy: Math.sin(a) * 50, life: 300, t: 0, col: '#8844ff', size: 2, spark: true }); }
                return;
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
            for (let i = 0; i < 12; i++) {
                const a = (i / 12) * Math.PI * 2, sp = 60 + Math.random() * 40;
                ctx.G.part.push({ x: pu.x, y: pu.y, vx: Math.cos(a) * sp, vy: Math.sin(a) * sp, life: 200 + Math.random() * 100, t: 0, col: puCol, size: 2, spark: true });
            }
        }
        function killP() {
            if (!ctx.G.p.alive) return;
            if (ctx.G.shieldHits > 0) { ctx.G.shieldHits--; ctx.G.damageVignetteT = 300; if (ctx.G.shieldHits <= 0) { ctx.G.activePU = null; ctx.G.puTimer = 0; ctx.setPUClass(null); ctx.SFX.shieldBreak(); } else ctx.SFX.shieldHit(); return; }
ctx.G.p.alive = false; ctx.boom(ctx.G.p.x, ctx.G.p.y, false, 'player'); ctx.SFX.pExplode(ctx.G.p.x); ctx.G.shkT = 300; ctx.G.shkM = 4; ctx.G.lives--;
            ctx.wrapEl.classList.add('galaxa-desaturate'); setTimeout(() => { if (!ctx.state.disposed) ctx.wrapEl.classList.remove('galaxa-desaturate'); }, 800);
            ctx.G.flashT = 50; ctx.G.chromAb = 300; ctx.G.damageVignetteT = 800; ctx.G.activePU = null; ctx.G.shieldHits = 0; ctx.G.timeScale = 1; ctx.G.timeSlowTimer = 0; ctx.G.puUpgrade = null;
            ctx.G.weaponLv = Math.max(1, ctx.G.weaponLv - 1);
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
                    if (ctx.G.p.reviveTimer <= 0) { ctx.G.p.x = ctx.W / 2; ctx.G.p.alive = true; ctx.G.p.inv = 3000; ctx.G.p.reviveTimer = 0; ctx.SFX.respawn(); }
                }
                return;
            }
            const inp = ctx.G.inp;
            const baseSpd = ctx.getShipSpeed();
            const spd = ctx.G.activePU && (ctx.G.activePU.type === 'speed' || ctx.G.activePU.type === 'hyper_speed') ? baseSpd * (ctx.G.activePU.type === 'hyper_speed' ? 2.2 : 1.8) : baseSpd;
            if (inp.l) ctx.G.p.x -= spd * dt; if (inp.r) ctx.G.p.x += spd * dt;
            ctx.G.p.x = Math.max(10, Math.min(ctx.W - 10, ctx.G.p.x));
            if (ctx.G.p.inv > 0) ctx.G.p.inv -= dt * 1000;
            if (inp.f && ctx.G.st === 'PLAYING') ctx.fire(now);
            if (ctx.G.beam && ctx.G.beam.active && ctx.G.p.x > ctx.G.beam.x - 20 && ctx.G.p.x < ctx.G.beam.x + 20 && ctx.G.p.y > ctx.G.beam.y) {
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
                const tCol1 = 'rgba(' + tRgb + ',' + eg + ')';
                const tCol2 = 'rgba(' + tRgb + ',0.4)';
                if (ctx.G.trails.length < 80) {
                    ctx.G.trails.push({ x: ctx.G.p.x - 6, y: ctx.G.p.y + 12, vx: (Math.random() - 0.5) * 10, vy: 20 + Math.random() * 15, life: 150, t: 0, col: tCol1, size: 2 });
                    ctx.G.trails.push({ x: ctx.G.p.x + 3, y: ctx.G.p.y + 12, vx: (Math.random() - 0.5) * 10, vy: 20 + Math.random() * 15, life: 150, t: 0, col: tCol1, size: 2 });
                    ctx.G.trails.push({ x: ctx.G.p.x - 4, y: ctx.G.p.y + 14, vx: (Math.random() - 0.5) * 5, vy: 15 + Math.random() * 10, life: 100, t: 0, col: tCol2, size: 1 });
                    if (ctx.G.p.dual) {
                        ctx.G.trails.push({ x: ctx.G.p.x + 28, y: ctx.G.p.y + 12, vx: (Math.random() - 0.5) * 10, vy: 20 + Math.random() * 15, life: 150, t: 0, col: tCol1, size: 2 });
                        ctx.G.trails.push({ x: ctx.G.p.x + 34, y: ctx.G.p.y + 12, vx: (Math.random() - 0.5) * 10, vy: 20 + Math.random() * 15, life: 150, t: 0, col: tCol1, size: 2 });
                    }
                    if (Math.abs(inp.r ? 1 : 0 - (inp.l ? 1 : 0)) > 0 && ctx.G.trails.length < 75) {
                        const wakeDir = inp.l ? 1 : -1;
                        ctx.G.trails.push({ x: ctx.G.p.x + wakeDir * 10, y: ctx.G.p.y + 8, vx: wakeDir * (40 + Math.random() * 30), vy: 10 + Math.random() * 10, life: 120, t: 0, col: 'rgba(255,200,100,0.3)', size: 1 });
                    }
                }
            }
            let plw = 0;
            for (let i = 0; i < ctx.G.powerups.length; i++) {
                const _pu = ctx.G.powerups[i];
                _pu.y += 60 * dt; _pu.t += dt * 1000;
                if (_pu.y > ctx.H + 20) continue;
                if (ctx.G.p.alive && ctx.hit({ x: _pu.x - 5, y: _pu.y - 5, w: 10, h: 10 }, { x: ctx.G.p.x - 6, y: ctx.G.p.y - 6, w: 12, h: 12 })) {
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
                    const ew = (e.type === 'boss' || e.type === 'miniboss') ? 24 : (e.type === 'hunter' ? 18 : e.type === 'sniper' ? 14 : 16);
                    if (ctx.hit(b, { x: e.x - ew / 2, y: e.y - 8, w: ew, h: 16 })) {
                        e.hp--;
                        if (e.hp <= 0) {
                            const pts = ctx.PTS[e.type] ? ctx.PTS[e.type][e.st === 'DIVING' ? 1 : 0] : 200;
                            ctx.registerKill();
                            ctx.addScore(pts, e.x, e.y, e.type === 'bee' ? '#ffcc00' : e.type === 'butterfly' ? '#ff3366' : '#44cc44');
                            ctx.boom(e.x, e.y, e.type === 'boss' || e.type === 'miniboss', e.type); ctx.SFX.eExplode(e.x); ctx.dropPU(e);
                            if (e.type === 'boss' || e.type === 'miniboss') { ctx.G.timeScale = 0.3; ctx.G.slowMoT = 1500; }
                            if (e.hasCap) ctx.G.p.cap = { x: e.x, y: e.y };
                            if (ctx.G.chal) ctx.G.chalHits++; e.st = 'DEAD';
                            // NEW: Splitter splits into 2 mini enemies on death
                            if (e.type === 'splitter') {
                                for (let _si = 0; _si < 2; _si++) {
                                    const sx = e.x + (_si === 0 ? -15 : 15);
                                    const sy = e.y - 10;
                                    ctx.G.enemies.push({ type: 'bee', r: 0, col: 0, x: sx, y: sy, fx: sx, fy: sy, hp: 1, maxHp: 1, st: 'DIVING', eTmr: 0, fr: 0, frT: 0, dTmr: 2000, dPath: { ph: 0, amp: 20, vx: (_si === 0 ? -40 : 40) }, sTmr: 500, shootPh: 0, hasCap: false, hitF: 0, elite: false, bossPhase: 0, bossPhaseTransition: 0, bossPhaseHP: [0,0,0], animFrame: 0, animTimer: 0, animSpeed: 120, animFrames: 4 });
                                }
                            }
                            // NEW: Carrier releases 3 bees on death
                            if (e.type === 'carrier') {
                                for (let _ci = 0; _ci < 3; _ci++) {
                                    const ca = (_ci / 3) * Math.PI * 2;
                                    ctx.G.enemies.push({ type: 'bee', r: 0, col: 0, x: e.x, y: e.y, fx: e.x + Math.cos(ca) * 40, fy: e.y + Math.sin(ca) * 40, hp: 1, maxHp: 1, st: 'ENTER', eTmr: 300 + _ci * 100, fr: 0, frT: 0, dTmr: 1500, dPath: null, sTmr: 800, shootPh: 0, hasCap: false, hitF: 0, elite: false, bossPhase: 0, bossPhaseTransition: 0, bossPhaseHP: [0,0,0], animFrame: 0, animTimer: 0, animSpeed: 120, animFrames: 4 });
                                }
                            }
                            ctx.G.killCount++;
                            if (ctx.G.killCount === 1) ctx.unlockAchievement('first_blood');
                            if (e.type === 'boss' || e.type === 'miniboss') { ctx.G.bossKillTotal++; if (ctx.G.bossKillTotal >= 10) ctx.unlockAchievement('boss_slayer'); try { localStorage.setItem('galaxa_boss_kills', String(ctx.G.bossKillTotal)); } catch(e2) {} }
                            const _remainingAlive = ctx.G.enemies.filter(_en => _en.st !== 'DEAD' && _en !== e).length;
                            if (_remainingAlive === 0 && e.type !== 'boss' && e.type !== 'miniboss') { ctx.G.timeScale = 0.3; ctx.G.slowMoT = 500; }
                            if (ctx.G.killCount % 10 === 0 && ctx.G.weaponLv < 4) { ctx.G.weaponLv++; ctx.SFX.weaponUp(); }
                        } else e.hitF = 100;
                        if (!b.laser && !b.pierce) { removed = true; break; }
                    }
                }
                if (!removed) ctx.G.bul[bw++] = b;
            }
            ctx.G.bul.length = bw;
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
                if (ctx.G.p.alive && ctx.G.p.inv <= 0 && ctx.hit(b, { x: ctx.G.p.x - 6, y: ctx.G.p.y - 6, w: 12, h: 12 })) { ctx.killP(); continue; }
                // NEW: Danger-close bonus — near miss detection
                if (ctx.G.p.alive && ctx.G.p.inv <= 0 && !ctx.G._closeCallCooldown) {
                    const _cdx = ctx.G.p.x - b.x, _cdy = ctx.G.p.y - b.y;
                    const _cdist = Math.hypot(_cdx, _cdy);
                    if (_cdist < 18 && _cdist > 8) {
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
                        if (Math.abs(e.x - e.fx) < 2 && Math.abs(e.y - e.fy) < 2) {
                            e.x = e.fx + ctx.G.fX;
                            e.y = e.fy + Math.sin(ctx.G.fTmr * 2 + e.col * 0.5) * 3;
                            e.st = 'FORM';
                            for (let _ei = 0; _ei < 2; _ei++) { const _ea = Math.random() * Math.PI * 2; ctx.G.part.push({ x: e.x, y: e.y, vx: Math.cos(_ea)*25, vy: Math.sin(_ea)*25, life: 200, t: 0, col: e.type === 'bee' ? '#ffcc00' : e.type === 'butterfly' ? '#ff3366' : e.type === 'hunter' ? '#ff6600' : e.type === 'spinner' ? '#44ffff' : e.type === 'bomber' ? '#cc66ff' : e.type === 'lasher' ? '#44ff88' : '#44cc44', size: 1, spark: true }); }
                            if ((e.type === 'boss' || e.type === 'miniboss') && !ctx.G.bossWarningShown) { ctx.G.bossWarningT = 2000; ctx.G.bossWarningShown = true; if (e.type === 'miniboss') ctx.SFX.miniBossWarning(); else ctx.SFX.bossWarning(); }
                            if (e.type === 'hunter') { ctx.G.bossWarningT = Math.max(ctx.G.bossWarningT || 0, 1000); ctx.SFX.hunterDive(e.x); }
                        }
                    }
                }
                else if (e.st === 'FORM') {
                    e.x = e.fx + ctx.G.fX; e.y = e.fy + Math.sin(ctx.G.fTmr * 2 + e.col * 0.5) * 3;
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
                            e.x = 40 + Math.random() * (ctx.W - 80);
                            e.y = ctx.FTOP + Math.random() * 100;
                            for (let _ti = 0; _ti < 8; _ti++) {
                                const _ta = (_ti / 8) * Math.PI * 2;
                                ctx.G.part.push({ x: e.x, y: e.y, vx: Math.cos(_ta) * 30, vy: Math.sin(_ta) * 30, life: 200, t: 0, col: '#44ffff', size: 1, spark: true });
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
                    if (e.dTmr <= 0 || e.y > ctx.H + 20) { e.st = 'RETURN'; e.y = -20; }
                    else {
                        const diveSpd = ctx.DIVE_SPD * (e.type === 'hunter' ? 2.1 : e.type === 'stalker' ? 1.5 : e.type === 'kamikaze' ? 2.5 : 1);
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
                            const ew = (e.type === 'boss' || e.type === 'miniboss') ? 16 : e.type === 'hunter' ? 14 : 12;
                            if (ctx.hit({ x: e.x - ew / 2, y: e.y - 8, w: ew, h: 16 }, { x: ctx.G.p.x - 6, y: ctx.G.p.y - 6, w: 12, h: 12 })) {
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
                else if (e.st === 'RETURN') { e.x += (e.fx + ctx.G.fX - e.x) * eDt * 3; e.y += (e.fy - e.y) * eDt * 3; if (Math.abs(e.x - e.fx - ctx.G.fX) < 3 && Math.abs(e.y - e.fy) < 3) { if (ctx.G.chal) { e.st = 'DEAD'; ctx.G.chalHits++; } else e.st = 'FORM'; } }
            }
            if (ctx.G.beam && ctx.G.beam.active) { ctx.G.beam.t += dtMs; ctx.G.beam.h = Math.min(200, ctx.G.beam.h + eDt * 300); if (ctx.G.beam.t > 3000) { ctx.G.beam.active = false; if (ctx.G.beam.cap && ctx.G.p.cap) { ctx.G.beam.owner.hasCap = true; ctx.G.p.cap = null; } } }
            ctx.G.dTmr -= dtMs;
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
        function startChalDive(e) {
            if (e.st !== 'FORM') return; e.st = 'DIVING'; e.dPath = { ph: 0, amp: 50 + Math.random() * 30, vx: (Math.random() - 0.5) * 130 }; e.dTmr = 4000; e.sTmr = 99999; ctx.SFX.dive();
        }

        ctx.mkFormation = mkFormation;
        ctx.fire = fire;
        ctx.boom = boom;
        ctx.bulletImpact = bulletImpact;
        ctx.addScore = addScore;
        ctx.updateCombo = updateCombo;
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
    };
})();
