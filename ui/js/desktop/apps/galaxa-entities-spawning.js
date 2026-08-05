(function () {
    'use strict';
    const GC = window.GalaxaCore = window.GalaxaCore || {};

    GC.createEntitiesSpawning = function (ctx) {
                function mkFormation() {
            ctx.G.enemies = []; ctx.G.chal = ctx.isChal(ctx.G.stage); ctx.G.chalHits = 0; ctx.G.chalTot = 0;
            ctx.G.bossWarningShown = false;
            const isMini = ctx.isMiniBossStage();
            const formType = (ctx.G.stage - 1) % 6;
            let idx = 0;

            function pushEnemy(type, r, col, fx, fy, hp) {
                const side = idx % 2 === 0 ? -1 : 1;
                const diveDelay = ctx.G.chal ? (800 + idx * 200) : (1000 + Math.random() * 3000 + idx * 50);
                const animSpeed = GC.ENEMY_ANIM_SPEED[type] || 120;
                const animFrames = GC.ENEMY_FRAME_COUNT[type] || 3;
                const enemy = { type, r, col, x: ctx.W / 2 + side * (120 + Math.random() * 80), y: -30 - (idx % 8) * 20,
                    fx, fy, hp, maxHp: hp, st: 'ENTER', eTmr: 500 + idx * 80 + r * 100,
                    fr: 0, frT: 0, dTmr: diveDelay / ctx.diffMod('diveRate'), dPath: null,
                    sTmr: (type === 'spinner' || type === 'bomber' || type === 'lasher') ? 800 + Math.random() * 1200 : 0,
                    shootPh: 0, hasCap: false, hitF: 0, elite: type === 'hunter',
                    bossPhase: (type === 'boss' || type === 'miniboss') ? 1 : 0,
                    bossPhaseTransition: 0, bossPhaseHP: [0.6, 0.3, 0],
                    animFrame: 0, animTimer: 0, animSpeed, animFrames,
                    spawnAnim: 0, spawnDur: GC.ENEMY_SPAWN_DURATION, rowPhase: r * 1.2 + col * 0.3, bobAmp: 2.5 + r * 0.5,
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

                const endlessLike = ctx.settings.mode === 'endless' || ctx.settings.mode === 'hyperdrive';
                const bossHP = ctx.G.stage >= 5 ? 2 + Math.floor((ctx.G.stage - 5) / 4) : (endlessLike ? 2 + Math.floor(ctx.G.stage / 3) : 2);
                const enemyHP = type === 'boss' ? bossHP : 1;
                pushEnemy(type, r, col, fx, fy, enemyHP);
            }
            if (ctx.settings.mode === 'endless' || ctx.settings.mode === 'hyperdrive') {
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
        ctx.mkFormation = mkFormation;
        ctx.spawnHazards = spawnHazards;
    };
})();
