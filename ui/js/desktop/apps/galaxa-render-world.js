(function () {
    'use strict';
    const GC = window.GalaxaCore = window.GalaxaCore || {};
    GC.createRenderWorld = function (ctx) {
        function ring(x, y, radius, color, alpha) {
            const c = ctx.c; c.save(); c.globalAlpha = alpha; c.strokeStyle = color; c.lineWidth = 2;
            c.beginPath(); c.arc(x, y, Math.max(0, radius), 0, Math.PI * 2); c.stroke(); c.restore();
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
                const { sp } = ctx.enemySpriteFor(e);
                c.save(); c.globalAlpha = e.invulnerable && !e.sectorBoss ? 0.55 : 1;
                ctx.drawAnimation(c, sp.key, e.x, e.y, sp.time, undefined, e.hitF > 50 ? '#dbeeff' : null);
                c.restore();
                if (e.armor) for (let side = 0; side < 2; side++) {
                    if (e.armor[side] <= 0) continue;
                    c.fillStyle = '#d8aa76'; c.fillRect(e.x + (side ? 37 : -46), e.y - 20, 9, 40);
                    c.fillStyle = '#514239'; c.fillRect(e.x + (side ? 37 : -46), e.y + 18, 9, 2);
                }
                if (e.coreOpen > 0) ctx.drawAnimation(c, 'fx.plasma', e.x, e.y + 18, time, 30);
            }
            for (const pu of G.powerups) {
                ctx.drawAnimation(c, 'pickup.' + pu.type, pu.x, pu.y, time, 32);
            }
            for (const mine of G.playerMines || []) ctx.drawAnimation(c, 'projectile.mine', mine.x, mine.y, time, 22);
            for (const drone of G.drones || []) ctx.drawAnimation(c, 'player.interceptor.idle', drone.x, drone.y, time, 24);
            for (const clone of G.clones || []) {
                c.globalAlpha = 0.6; ctx.drawAnimation(c, 'player.stealth.fire', clone.x, clone.y, time, 48); c.globalAlpha = 1;
            }
            if (G.p.alive) {
                const ref = ctx.getPlayerSpriteFrame();
                c.globalAlpha = G.p.inv > 0 && Math.floor(time / 90) % 2 ? 0.5 : 1;
                ctx.drawAnimation(c, ref.key, G.p.x, G.p.y, ref.time, 48);
                if (G.p.dual) ctx.drawAnimation(c, ref.key, G.p.x + 36, G.p.y, ref.time, 48);
                if (G.mirrorActive) { c.globalAlpha = 0.4; ctx.drawAnimation(c, ref.key, ctx.W - G.p.x, G.p.y, ref.time, 48); }
                c.globalAlpha = 1;
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
                c.fillStyle = '#31234a'; c.fillRect(laser.x - laser.w / 2 - 3, laser.y, laser.w + 6, laser.h);
                c.fillStyle = '#fa759d'; c.fillRect(laser.x - laser.w / 2, laser.y, laser.w, laser.h);
                c.fillStyle = '#fff2cc'; c.fillRect(laser.x - 2, laser.y, 4, laser.h);
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
