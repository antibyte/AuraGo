(function () {
    'use strict';
    const GC = window.GalaxaCore = window.GalaxaCore || {};
    GC.createRenderWorld = function (ctx) {
        function ring(x, y, radius, color, alpha) {
            const c = ctx.c; c.save(); c.globalAlpha = alpha; c.strokeStyle = color; c.lineWidth = 2;
            c.beginPath(); c.arc(x, y, Math.max(0, radius), 0, Math.PI * 2); c.stroke(); c.restore();
        }
        function drawThruster(c, x, y, time, boost, col) {
            if (ctx.settings.reducedMotion) return;
            const flicker = 0.8 + 0.35 * Math.sin(time * 0.045 + x);
            const len = (boost ? 15 : 9) * flicker;
            c.save();
            c.globalCompositeOperation = 'lighter';
            for (const ox of [-5, 5]) {
                c.globalAlpha = 0.72;
                c.fillStyle = col || '#44bbff';
                c.beginPath();
                c.moveTo(x + ox - 2.5, y + 14);
                c.lineTo(x + ox + 2.5, y + 14);
                c.lineTo(x + ox, y + 14 + len);
                c.closePath();
                c.fill();
                c.globalAlpha = 0.9;
                c.fillStyle = '#ffffff';
                c.beginPath();
                c.moveTo(x + ox - 1.2, y + 14);
                c.lineTo(x + ox + 1.2, y + 14);
                c.lineTo(x + ox, y + 14 + len * 0.55);
                c.closePath();
                c.fill();
            }
            c.restore();
        }
        function renderGame() {
            const G = ctx.G, c = ctx.c, time = G.animTime || 0;
            if (!ctx.settings.reducedMotion) {
                for (const p of G.part) {
                    ctx.drawParticle(c, p);
                }
                for (const trail of G.trails) {
                    c.globalAlpha = Math.max(0, 1 - trail.t / trail.life) * 0.4; c.fillStyle = trail.col;
                    c.fillRect(Math.round(trail.x), Math.round(trail.y), trail.size || 1, trail.size || 1);
                }
                c.globalAlpha = 1;
            }
            for (const e of G.enemies) {
                if (e.st === 'DEAD') continue;
                // NEW: smoldering crash divers rotate through their own draw path.
                if (e.st === 'SMOLDER') continue;
                if (!ctx.settings.reducedMotion && ctx.drawEnemyMotionTrail) ctx.drawEnemyMotionTrail(c, e);
                if (e.mSkipDraw) continue;
                const { sp } = ctx.enemySpriteFor(e);
                const baseAlpha = e.invulnerable && !e.sectorBoss ? 0.55 : 1;
                const mAlpha = e.mAlpha != null ? e.mAlpha : 1;
                c.save();
                c.globalAlpha = baseAlpha * mAlpha;
                ctx.drawAnimation(c, sp.key, e.x, e.y, sp.time, undefined, e.hitF > 50 ? '#dbeeff' : null);
                c.restore();
                if (e.enraged) {
                    ring(e.x, e.y, (e.w || 28) * 0.65, '#ff4422', 0.35 + 0.2 * Math.sin(time * 0.018));
                }
                if (e.armor) for (let side = 0; side < 2; side++) {
                    if (e.armor[side] <= 0) continue;
                    c.fillStyle = '#d8aa76'; c.fillRect(e.x + (side ? 37 : -46), e.y - 20, 9, 40);
                    c.fillStyle = '#514239'; c.fillRect(e.x + (side ? 37 : -46), e.y + 18, 9, 2);
                }
                if (e.coreOpen > 0) ctx.drawAnimation(c, 'fx.plasma', e.x, e.y + 18, time, 30);
            }
            for (const pu of G.powerups) {
                if (!ctx.settings.reducedMotion) {
                    const puCol = (ctx.PU_COL && ctx.PU_COL[pu.type]) || '#88ddff';
                    ring(pu.x, pu.y, 13 + Math.sin(time * 0.008 + pu.x) * 2, puCol, 0.32);
                }
                ctx.drawAnimation(c, 'pickup.' + pu.type, pu.x, pu.y, time, 32);
            }
            for (const mine of G.playerMines || []) ctx.drawAnimation(c, 'projectile.mine', mine.x, mine.y, time, 22);
            for (const drone of G.drones || []) ctx.drawAnimation(c, 'player.interceptor.idle', drone.x, drone.y, time, 24);
            for (const clone of G.clones || []) {
                drawThruster(c, clone.x, clone.y, time, false, '#aa66ff');
                c.globalAlpha = 0.6; ctx.drawAnimation(c, 'player.stealth.fire', clone.x, clone.y, time, 48); c.globalAlpha = 1;
            }
            if (G.p.alive) {
                const speedy = G.activePU && (G.activePU.type === 'speed' || G.activePU.type === 'hyper_speed');
                const thrustBoost = !!(G.inp && G.inp.u) || speedy || G.superPhase !== 'idle';
                const thrustCol = G.comboMult >= 8 ? '#ff6622' : (speedy ? '#66ffee' : '#44bbff');
                drawThruster(c, G.p.x, G.p.y, time, thrustBoost, thrustCol);
                if (G.p.dual) drawThruster(c, G.p.x + 36, G.p.y, time, thrustBoost, thrustCol);
                const ref = ctx.getPlayerSpriteFrame();
                c.globalAlpha = G.p.inv > 0 && Math.floor(time / 90) % 2 ? 0.5 : 1;
                ctx.drawAnimation(c, ref.key, G.p.x, G.p.y, ref.time, 48);
                if (G.p.dual) ctx.drawAnimation(c, ref.key, G.p.x + 36, G.p.y, ref.time, 48);
                if (G.mirrorActive) { c.globalAlpha = 0.4; ctx.drawAnimation(c, ref.key, ctx.W - G.p.x, G.p.y, ref.time, 48); }
                c.globalAlpha = 1;
                if (G.muzzleT > 0 && !ctx.settings.reducedMotion) {
                    const mRatio = Math.min(1, G.muzzleT / 160);
                    const puType = (G.activePU && G.activePU.type) || '';
                    const isLaserMuz = puType === 'laser' || puType === 'mega_laser' || (G.weaponEvo && ctx.WEAPON_EVOS && ctx.WEAPON_EVOS[G.weaponEvo] && ctx.WEAPON_EVOS[G.weaponEvo].isBeam);
                    const isSpreadMuz = puType === 'spread' || puType === 'mega_spread';
                    const isRocketMuz = puType === 'rocket_launcher' || puType === 'mega_rocket';
                    const mCol = isLaserMuz ? '#ff66cc' : isSpreadMuz ? '#66eeff' : isRocketMuz ? '#ff8833' : '#ffee88';
                    c.save();
                    c.globalCompositeOperation = 'lighter';
                    for (const mx of (G.p.dual ? [G.p.x, G.p.x + 36] : [G.p.x])) {
                        const my = G.p.y - 15;
                        c.globalAlpha = mRatio * 0.8;
                        c.fillStyle = mCol;
                        if (isLaserMuz) {
                            c.fillRect(mx - 8 * mRatio, my - 1.5, 16 * mRatio, 3);
                            c.fillRect(mx - 2, my - 9 * mRatio, 4, 14 * mRatio);
                        } else if (isSpreadMuz) {
                            for (const ox of [-5, 0, 5]) {
                                c.beginPath(); c.arc(mx + ox * mRatio, my - Math.abs(ox) * 0.3, 3.2 * mRatio, 0, Math.PI * 2); c.fill();
                            }
                        } else if (isRocketMuz) {
                            c.strokeStyle = mCol; c.lineWidth = 2;
                            c.beginPath(); c.arc(mx, my, 4 + (1 - mRatio) * 7, 0, Math.PI * 2); c.stroke();
                            c.beginPath(); c.arc(mx, my, 4 * mRatio, 0, Math.PI * 2); c.fill();
                        } else {
                            c.beginPath(); c.arc(mx, my, 5 * mRatio, 0, Math.PI * 2); c.fill();
                            c.fillRect(mx - 7 * mRatio, my - 0.8, 14 * mRatio, 1.6);
                        }
                        c.fillStyle = '#ffffff';
                        c.beginPath(); c.arc(mx, my, 2.5 * mRatio, 0, Math.PI * 2); c.fill();
                    }
                    c.restore();
                }
                if (G.fxSuperReadyT > 0 || G.superPhase === 'charge') {
                    ring(G.p.x, G.p.y, 22 + Math.sin(time * 0.02) * 3, '#ffd866', 0.45);
                }
                if (G.shieldHits > 0 || G.startShieldHits > 0) ctx.drawAnimation(c, 'fx.shield', G.p.x, G.p.y, time, 54);
                if (G.parryActive > 0 || G.parrySuccessFlash > 0) ctx.drawAnimation(c, 'fx.parry', G.p.x, G.p.y, time, 64);
                if (G.orbitalShields) for (let i = 0; i < G.orbitalShields.length; i++) {
                    const shield = G.orbitalShields[i]; if (!shield.active) continue;
                    ctx.drawAnimation(c, 'fx.shield', G.p.x + Math.cos(shield.angle) * 32, G.p.y + Math.sin(shield.angle) * 32, time, 18);
                }
                // The hitbox remains readable when maneuvering through dense patterns.
                c.fillStyle = '#e5faff'; c.fillRect(Math.round(G.p.x - 1), Math.round(G.p.y - 1), 2, 2);
            }
            for (const bullet of G.bul) {
                const kind = bullet.rocket ? 'rocket' : bullet.laser ? 'laser' : bullet._parried ? 'bolt' : bullet.kind === 'nova' ? 'plasma' : 'normal';
                if (!ctx.settings.reducedMotion && (bullet.laser || bullet.rocket || bullet._parried || bullet.pierce)) {
                    c.save();
                    c.globalCompositeOperation = 'lighter';
                    c.globalAlpha = 0.45;
                    c.fillStyle = bullet.laser ? '#ff66aa' : bullet._parried ? '#66eeff' : bullet.rocket ? '#ffaa44' : '#88ffcc';
                    c.fillRect(bullet.x - 1.5, bullet.y + 4, 3, bullet.laser ? 14 : 9);
                    c.restore();
                }
                ctx.drawAnimation(c, 'projectile.' + kind, bullet.x, bullet.y, time, bullet.laser ? 28 : bullet.rocket ? 24 : 18);
            }
            for (const ex of G.exp) ctx.drawAnimation(c, ex.animation || ('fx.explosion.' + (ex.isBoss ? 'large' : 'small')), ex.x, ex.y, ex.t, ex.size || (ex.isBoss ? 128 : 48), null, false);
            for (const r of G.plasmaRings) ring(r.x, r.y, r.r, r.col, Math.max(0, 1 - r.t / r.dur) * 0.45);
            if (G.bossDeath) ctx.drawAnimation(c, 'sector.' + G.bossDeath.id + '.death', G.bossDeath.x, G.bossDeath.y, G.bossDeath.t, 128, null, false);
            for (const text of [...G.scorePopups, ...G.combatText]) {
                c.globalAlpha = Math.max(0, 1 - text.t / text.dur); c.fillStyle = text.col || '#fff';
                c.font = (text.big ? 'bold 13px' : '10px') + ' monospace'; c.textAlign = 'center';
                c.fillText(text.text, text.x, text.y, 220);
            }
            c.globalAlpha = 1;
        }
        function renderDangers() {
            const G = ctx.G, c = ctx.c, time = G.animTime || 0;
            for (const h of G.envHazards || []) {
                if (h.x === undefined || h.y === undefined) continue;
                ring(h.x, h.y, h.r || 18, '#ef9675', 0.8);
            }
            for (const ast of G.asteroids || []) {
                c.save();
                c.translate(ast.x, ast.y);
                c.rotate(ast.rot || 0);
                c.fillStyle = '#6b5344';
                const ar = ast.r || 12;
                c.beginPath(); c.arc(0, 0, ar, 0, Math.PI * 2); c.fill();
                c.strokeStyle = '#a8846c'; c.lineWidth = 1.5; c.stroke();
                c.restore();
            }
            if (G.solarFlareActive && !ctx.settings.reducedMotion) {
                c.save();
                c.globalCompositeOperation = 'lighter';
                c.fillStyle = 'rgba(255,140,40,0.12)';
                c.fillRect(0, 0, ctx.W, ctx.H);
                c.restore();
            }
            for (const zone of G.voidZones || []) ring(zone.x, zone.y, zone.r, '#c59bff', 0.75);
            for (const field of [G.blackhole, G.gravityBomb]) if (field) {
                ring(field.x, field.y, field.r || 55, '#af84f5', 0.55);
                ctx.drawAnimation(c, 'fx.plasma', field.x, field.y, time, 44);
            }
            if (G.beam && G.beam.active && ctx.renderBeam) ctx.renderBeam(G.beam);
            for (const e of G.enemies) {
                if (e.st === 'DEAD') continue;
                if (e.shotWarning > 0) {
                    ring(e.x, e.y, 17, '#ffe79a', 0.9);
                    c.strokeStyle = '#d8ac60'; c.setLineDash([3, 9]); c.beginPath(); c.moveTo(e.x, e.y + 12); c.lineTo(G.p.x, G.p.y); c.stroke(); c.setLineDash([]);
                }
                if (!e.sectorBoss) continue;
                if (e.sectorBoss === 'crystal' || e.reflectT > 0) for (const side of [-1, 1]) {
                    const x = e.x + side * 88, y = e.y + 45;
                    ctx.drawAnimation(c, 'projectile.crystal', x, y, time, 42);
                    c.strokeStyle = e.reflectT > 0 ? '#fff1b8' : '#5b9fb7'; c.lineWidth = 2;
                    c.strokeRect(x - 9, y - 24, 18, 48);
                }
                if (e.bossState === 'transition') ring(e.x, e.y, 67, '#e6d3aa', 0.75);
                if (!e.telegraph) continue;
                const pattern = e.sectorBoss === 'void' ? ['nebula', 'crystal', 'storm'][(e.volley || 0) % 3] : e.sectorBoss;
                c.save(); c.strokeStyle = '#ffe39a'; c.lineWidth = 2; c.setLineDash([6, 5]);
                if (pattern === 'crystal' || pattern === 'storm') {
                    for (let lane = 0; lane < 8; lane++) {
                        if (Math.abs(lane - e.safeLane) <= 1) continue;
                        const x = 40 + lane * 65;
                        c.fillStyle = 'rgba(247,180,84,0.10)'; c.fillRect(x - 12, 230, 24, ctx.H - 260);
                        c.strokeRect(x - 12, 230, 24, ctx.H - 260);
                    }
                } else if (pattern === 'asteroid') {
                    c.fillStyle = 'rgba(247,180,84,0.12)'; c.fillRect(e.targetX - 45, e.y, 90, 330);
                    c.strokeRect(e.targetX - 45, e.y, 90, 330);
                } else {
                    c.beginPath(); c.moveTo(e.x, e.y + 35); c.lineTo(e.targetX, G.p.y); c.stroke();
                    ring(e.x, e.y, 58, '#ffe39a', 0.9);
                }
                c.restore();
            }
            for (const laser of G.bossLasers || []) {
                const pulse = 0.85 + 0.15 * Math.sin(time * 0.035);
                const lw = laser.w * pulse;
                c.fillStyle = 'rgba(180, 45, 110, 0.28)';
                c.fillRect(laser.x - lw / 2 - 7, laser.y, lw + 14, laser.h);
                c.fillStyle = '#31234a';
                c.fillRect(laser.x - laser.w / 2 - 3, laser.y, laser.w + 6, laser.h);
                c.fillStyle = '#fa759d';
                c.fillRect(laser.x - lw / 2, laser.y, lw, laser.h);
                c.fillStyle = '#fff2cc';
                c.fillRect(laser.x - 2, laser.y, 4, laser.h);
                if (!ctx.settings.reducedMotion) {
                    c.fillStyle = '#ffffff';
                    for (let sy = laser.y + ((time * 0.4) % 36); sy < laser.y + laser.h; sy += 36) {
                        c.fillRect(laser.x - lw / 2 - 2, sy, lw + 4, 2);
                    }
                }
            }
            for (const bullet of G.ebul) {
                const kind = bullet.kind === 'mine' ? 'mine' : bullet.kind === 'crystal' ? 'crystal' : bullet.kind === 'gravity' ? 'gravity' : 'ion';
                // Dark rim and bright solid core keep hazards visible through explosions and supers.
                c.fillStyle = '#080c1c'; c.beginPath(); c.arc(bullet.x, bullet.y, 6, 0, Math.PI * 2); c.fill();
                ctx.drawAnimation(c, 'projectile.' + kind, bullet.x, bullet.y, time, 18);
                c.fillStyle = '#fff3b6'; c.fillRect(Math.round(bullet.x - 1), Math.round(bullet.y - 1), 3, 3);
            }
        }
        ctx.renderGame = renderGame; ctx.renderDangers = renderDangers;
    };
})();
