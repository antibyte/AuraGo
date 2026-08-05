(function () {
    'use strict';
    const GC = window.GalaxaCore = window.GalaxaCore || {};

    GC.createEntitiesWeapons = function (ctx) {
        function findNearestEnemy(px, py) {
            let nearestE = null, nearD2 = Infinity;
            for (let _ei = 0; _ei < ctx.G.enemies.length; _ei++) {
                const e = ctx.G.enemies[_ei];
                if (e.st === 'DEAD') continue;
                const dx = e.x - px, dy = e.y - py;
                const d2 = dx * dx + dy * dy;
                if (d2 < nearD2) { nearD2 = d2; nearestE = e; }
            }
            return nearestE;
        }

        function fireRocketSalvo(isMega) {
            const ppx = ctx.G.p.x, ppy = ctx.G.p.y;
            const count = isMega ? 2 : 1;
            for (let ri = 0; ri < count; ri++) {
                const ox = count > 1 ? (ri === 0 ? -10 : 10) : 0;
                const nearestE = findNearestEnemy(ppx + ox, ppy);
                let vx = 0, vy = -ctx.PB_SPEED * 0.55;
                if (nearestE) {
                    const dx = nearestE.x - (ppx + ox), dy = nearestE.y - ppy;
                    const dist = Math.hypot(dx, dy) || 1;
                    vx = (dx / dist) * ctx.PB_SPEED * 0.55;
                    vy = (dy / dist) * ctx.PB_SPEED * 0.55;
                }
                ctx.G.bul.push({
                    x: ppx + ox, y: ppy - 8, w: 5, h: 11, vx, vy,
                    homing: true, rocket: true, target: nearestE || null, pierce: true
                });
            }
            if (ctx.SFX.rocketLaunch) ctx.SFX.rocketLaunch(ppx);
            ctx.G.muzzleT = 70;
            ctx.G.stageAccuracyShots = (ctx.G.stageAccuracyShots || 0) + count;
        }

        function mirrorDuplicateBullets(fromIdx) {
            const _mirrorOn = ctx.G.mirrorActive || (ctx.modesIsMirrorPermanent && ctx.modesIsMirrorPermanent());
            if (!_mirrorOn) return;
            const mirrorX = ctx.W - ctx.G.p.x;
            const _ghost = ctx.modesIsMirrorPermanent && ctx.modesIsMirrorPermanent();
            for (let bi = fromIdx; bi < ctx.G.bul.length; bi++) {
                const b = ctx.G.bul[bi];
                if (b._mirror) continue;
                ctx.G.bul.push({
                    x: mirrorX + (ctx.G.p.x - b.x), y: b.y, w: b.w, h: b.h,
                    vx: b.vx ? -b.vx : 0, vy: b.vy, laser: b.laser, pierce: b.pierce,
                    homing: b.homing, rocket: b.rocket, target: b.target, _mirror: true, _ghost: _ghost
                });
            }
            if (_ghost && ctx.fxMirrorRefract) ctx.fxMirrorRefract();
        }

        function pushPlayerMine(mine) {
            if (!ctx.G.playerMines) ctx.G.playerMines = [];
            const cap = ctx.PLAYER_MINE_MAX || GC.PLAYER_MINE_MAX || 12;
            while (ctx.G.playerMines.length >= cap) ctx.G.playerMines.shift();
            ctx.G.playerMines.push(mine);
        }

        function updatePlayerMines(dtMs, dt) {
            if (!ctx.G.playerMines || !ctx.G.playerMines.length) return;
            let mw = 0;
            for (let i = 0; i < ctx.G.playerMines.length; i++) {
                const m = ctx.G.playerMines[i];
                m.t += dtMs;
                m.y += (m.vy || 28) * dt;
                if (m.armT > 0) m.armT -= dtMs;
                if (m.t > 9000 || m.y > ctx.H + 20) continue;
                if (m.armT <= 0) {
                    for (let j = ctx.G.enemies.length - 1; j >= 0; j--) {
                        const e = ctx.G.enemies[j];
                        if (e.st === 'DEAD') continue;
                        if (Math.hypot(e.x - m.x, e.y - m.y) < (m.r || 10) + 8) {
                            ctx.boom(m.x, m.y, false, 'bomber');
                            if (ctx.SFX.mineExplode) ctx.SFX.mineExplode(m.x);
                            ctx.G.shkT = Math.max(ctx.G.shkT, 120); ctx.G.shkM = Math.max(ctx.G.shkM, 2);
                            for (let k = 0; k < ctx.G.enemies.length; k++) {
                                const ne = ctx.G.enemies[k];
                                if (ne.st === 'DEAD') continue;
                                if (Math.hypot(ne.x - m.x, ne.y - m.y) < 52) {
                                    ne.hp--;
                                    if (ne.hp <= 0) {
                                        const pts = ctx.PTS[ne.type] ? ctx.PTS[ne.type][0] : 200;
                                        ctx.registerKill(); ctx.addScore(pts, ne.x, ne.y, '#ccaa44');
                                        ctx.boom(ne.x, ne.y, ne.type === 'boss' || ne.type === 'miniboss', ne.type);
                                        ctx.SFX.eExplode(ne.x); ctx.dropPU(ne); ne.st = 'DEAD';
                                    } else ne.hitF = 120;
                                }
                            }
                            m.t = 99999;
                            break;
                        }
                    }
                }
                if (m.t < 99999) ctx.G.playerMines[mw++] = m;
            }
            ctx.G.playerMines.length = mw;
        }

        ctx.findNearestEnemy = findNearestEnemy;
        ctx.fireRocketSalvo = fireRocketSalvo;
        ctx.mirrorDuplicateBullets = mirrorDuplicateBullets;
        ctx.pushPlayerMine = pushPlayerMine;
        ctx.updatePlayerMines = updatePlayerMines;
    };
})();
