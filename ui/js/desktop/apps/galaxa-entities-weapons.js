(function () {
    'use strict';
    const GC = window.GalaxaCore = window.GalaxaCore || {};
    GC.createEntitiesWeapons = function (ctx) {
        function isSeekerTarget(e) {
            return !!e && e.st !== 'DEAD' && e.st !== 'SMOLDER' && e.hp > 0 && !e.invulnerable && !e.mSkipDraw &&
                e.x >= 0 && e.x <= ctx.W && e.y >= 0 && e.y <= ctx.H;
        }
        function findNearestEnemy(px, py, excluded) {
            let nearest = null, distance = Infinity;
            for (const e of ctx.G.enemies) {
                if (!isSeekerTarget(e) || (excluded && excluded.has(e))) continue;
                const d = (e.x - px) ** 2 + (e.y - py) ** 2;
                if (d < distance) { distance = d; nearest = e; }
            }
            return nearest;
        }
        function shouldApplyPlayerHit(b) {
            return !(b._mirror && b._ghost && ctx.modesMirrorGhostDamageMult && ctx.random() > ctx.modesMirrorGhostDamageMult());
        }
        function expansionEffect(id, x, y, size) {
            const animation = 'effect.' + id;
            const a = ctx.spriteAssets && ctx.spriteAssets.manifest.animations[animation];
            if (!a) return;
            if (ctx.G.exp.length >= 80) ctx.G.exp.shift();
            ctx.G.exp.push({ x, y, t: 0, dur: a.ms * a.frames.length, animation, size });
        }
        function areaDamage(x, y, radius, damage, excluded, source = {}) {
            // Snapshot excludes offspring created by another kill in this blast.
            for (const e of ctx.G.enemies.slice()) {
                if (e === excluded || e.st === 'DEAD' || e.st === 'SMOLDER' || e.hp <= 0 || e.invulnerable || Math.hypot(e.x - x, e.y - y) > radius) continue;
                ctx.damageEnemy(e, damage);
                ctx.finishEnemyKill(e, { ...source, x, y, noSmolder: true });
            }
            if (ctx.damageAsteroidsInRadius) ctx.damageAsteroidsInRadius(x, y, radius, damage, excluded);
        }
        function fireRocketSalvo(isMega) {
            const type = (ctx.G.activePU && ctx.G.activePU.type) || (isMega ? 'mega_rocket' : 'rocket_launcher');
            const def = ctx.EXPANSION_WEAPONS[type] || ctx.EXPANSION_WEAPONS.rocket_launcher;
            const targets = new Set(), p = ctx.G.p;
            for (let i = 0; i < def.count; i++) {
                const x = p.x + (i - (def.count - 1) / 2) * 14;
                const target = findNearestEnemy(x, p.y, type === 'swarm_launcher' ? targets : null) || findNearestEnemy(x, p.y);
                if (target) targets.add(target);
                const dx = target ? target.x - x : 0, dy = target ? target.y - p.y : -1;
                const distance = Math.hypot(dx, dy) || 1, speed = Math.min(275, def.speed);
                ctx.G.bul.push({ x, y: p.y - 8, w: type === 'seeker_torpedo' ? 8 : 5, h: 11,
                    vx: dx / distance * speed, vy: dy / distance * speed, homing: true,
                    rocket: true, weapon: type, artId: def.art, target, dmg: def.damage,
                    speed: def.speed, age: 0, life: ctx.SEEKER_LIFE_MS });
            }
            if (ctx.SFX.rocketLaunch) ctx.SFX.rocketLaunch(p.x);
            if (ctx.SFX.expansionWeapon) ctx.SFX.expansionWeapon(type, p.x);
            ctx.G.muzzleT = 70;
            ctx.G.stageAccuracyShots = (ctx.G.stageAccuracyShots || 0) + def.count;
        }
        function rocketImpact(b, direct) {
            const def = ctx.EXPANSION_WEAPONS[b.weapon] || ctx.EXPANSION_WEAPONS.rocket_launcher;
            if (def.radius) areaDamage(b.x, b.y, def.radius, 1, direct, b);
            expansionEffect(b.weapon === 'seeker_torpedo' ? 'torpedo-shock' : 'alien-plasma', b.x, b.y, def.radius ? def.radius * 2 : 38);
            if (ctx.SFX.rocketHit) ctx.SFX.rocketHit(b.x);
        }
        function mirrorDuplicateBullets(fromIdx) {
            const permanent = ctx.modesIsMirrorPermanent && ctx.modesIsMirrorPermanent();
            if (!ctx.G.mirrorActive && !permanent) return;
            const end = ctx.G.bul.length;
            for (let i = fromIdx; i < end; i++) {
                const b = ctx.G.bul[i]; if (b._mirror) continue;
                ctx.G.bul.push({ ...b, baseX: b.baseX !== undefined ? ctx.W - b.baseX : undefined, x: ctx.W - b.x, vx: -(b.vx || 0),
                    hitTargets: b.hitTargets ? new Set(b.hitTargets) : undefined, _mirror: true, _ghost: !!permanent });
            }
            if (permanent && ctx.fxMirrorRefract) ctx.fxMirrorRefract();
        }
        function pushPlayerMine(mine) {
            const mines = ctx.G.playerMines || (ctx.G.playerMines = []);
            while (mines.length >= ctx.PLAYER_MINE_MAX) mines.shift();
            mines.push({ weapon: 'mine_layer', t: 0, ...mine, armT: ctx.MINE_ARM_MS });
        }
        function dropPlayerMines(dt) {
            const G = ctx.G, type = G.activePU && G.activePU.type, def = ctx.EXPANSION_WEAPONS[type];
            if (!def || !def.mine || !G.p.alive || G.st !== 'PLAYING') return;
            G.mineDropT = (G.mineDropT || 0) - dt * 1000;
            if (G.mineDropT > 0) return;
            G.mineDropT = def.interval;
            const count = def.count || 1;
            for (let i = 0; i < count; i++) pushPlayerMine({ x: G.p.x + (i - (count - 1) / 2) * 28, y: G.p.y + 12, vy: 22, r: 9, weapon: type });
            if (ctx.SFX.mineDrop) ctx.SFX.mineDrop(G.p.x);
        }
        function detonateMine(m) {
            const G = ctx.G, def = ctx.EXPANSION_WEAPONS[m.weapon] || ctx.EXPANSION_WEAPONS.mine_layer;
            if (def.emp) {
                G.ebul = G.ebul.filter(b => Math.hypot(b.x - m.x, b.y - m.y) > def.radius);
                for (const e of G.enemies) {
                    if (isSeekerTarget(e) && !e.sectorBoss && e.type !== 'boss' && e.type !== 'miniboss' && Math.hypot(e.x - m.x, e.y - m.y) <= def.radius) e.empUntil = G.simTime + 1500;
                }
                expansionEffect('emp-wave', m.x, m.y, def.radius * 2);
            } else {
                areaDamage(m.x, m.y, def.radius, def.damage, null, { noSmolder: true });
                expansionEffect(def.shards ? 'metal-sparks' : 'torpedo-shock', m.x, m.y, def.radius * 2);
            }
            for (let i = 0; i < (def.shards || 0); i++) {
                const angle = i * Math.PI * 2 / def.shards;
                G.bul.push({ x: m.x, y: m.y, w: 4, h: 4, vx: Math.cos(angle) * 240, vy: Math.sin(angle) * 240, dmg: 1, life: 600, age: 0, mineShard: true });
            }
            if (ctx.SFX.mineExplode) ctx.SFX.mineExplode(m.x);
            if (ctx.SFX.expansionWeapon) ctx.SFX.expansionWeapon(m.weapon, m.x);
        }
        function updatePlayerMines(dtMs, dt) {
            const mines = ctx.G.playerMines || []; let write = 0;
            for (const m of mines) {
                m.t += dtMs; m.y += (m.vy ?? 28) * dt; m.armT = Math.max(0, m.armT - dtMs);
                if (m.t >= ctx.MINE_LIFE_MS || m.y > ctx.H + 20) continue;
                const def = ctx.EXPANSION_WEAPONS[m.weapon] || ctx.EXPANSION_WEAPONS.mine_layer;
                const enemyNear = m.armT === 0 && ctx.G.enemies.some(e => isSeekerTarget(e) && Math.hypot(e.x - m.x, e.y - m.y) <= 26);
                const shotNear = m.armT === 0 && def.emp && ctx.G.ebul.some(b => Math.hypot(b.x - m.x, b.y - m.y) <= def.radius);
                if (enemyNear || shotNear) detonateMine(m);
                else mines[write++] = m;
            }
            mines.length = write;
        }
        function fireEvoWeapon(id, def, spreadLevel, pierce) {
            const G = ctx.G, p = G.p, origins = p.dual ? [p.x, p.x + 36] : [p.x];
            for (const ox of origins) {
                if (id === 'wave') {
                    // Twin bolts snake around the launch column in opposite phase; a spread powerup adds outer lanes.
                    const n = 2 + spreadLevel * 2;
                    for (let i = 0; i < n; i++) {
                        const lane = Math.floor(i / 2);
                        G.bul.push({ x: ox, y: p.y - 8, w: 4, h: 8, vx: 0, vy: -ctx.PB_SPEED * 0.92, wave: true, baseX: ox, waveAmp: 18 + lane * 12,
                            waveFreq: 0.0135, wavePhase: i % 2 ? Math.PI : 0, age: 0, dmg: 1, pierce: !!pierce });
                    }
                } else if (id === 'flak') {
                    const n = 1 + spreadLevel * 2;
                    for (let i = 0; i < n; i++) {
                        const a = (i - (n - 1) / 2) * 0.21;
                        G.bul.push({ x: ox, y: p.y - 10, w: 6, h: 10, vx: Math.sin(a) * ctx.PB_SPEED * 0.72, vy: -Math.cos(a) * ctx.PB_SPEED * 0.72, flak: true, fuse: 620, age: 0, dmg: 1, pierce: !!pierce });
                    }
                } else if (id === 'rail') {
                    const permanent = ctx.modesIsMirrorPermanent && ctx.modesIsMirrorPermanent(), mirrored = G.mirrorActive || permanent;
                    for (const dx of spreadLevel ? [-26, 0, 26] : [0]) {
                        fireRailLance(ox + dx, p.y - 6, def.dmgMult, null);
                        if (mirrored) fireRailLance(ctx.W - (ox + dx), p.y - 6, def.dmgMult, { _mirror: true, _ghost: !!permanent });
                    }
                }
            }
        }
        function fireRailLance(x, y0, dmg, flags) {
            // Instant hit along the whole column above the muzzle; the beam only lingers as a visual.
            const G = ctx.G, hits = [];
            for (const e of G.enemies.slice()) {
                if (!ctx.isSeekerTarget(e) || e.y > y0) continue;
                const half = e.sectorBoss ? 41 : (e.type === 'boss' || e.type === 'miniboss') ? 19 : 12;
                if (Math.abs(e.x - x) <= half + 3) hits.push(e);
            }
            for (const e of hits) {
                const apply = !flags || shouldApplyPlayerHit(flags);
                if (!apply) continue;
                ctx.damageEnemy(e, dmg);
                G.stageAccuracyHits = (G.stageAccuracyHits || 0) + 1;
                if (e.hp <= 0 && !e.sectorBoss) ctx.finishEnemyKill(e, { x, y: e.y, vx: 0, vy: -ctx.PB_SPEED, noSmolder: true });
                else { e.hitF = 100; ctx.bulletImpact(x, e.y, '#bfe6ff', 0, -1); if (ctx.SFX.enemyHitSfx) ctx.SFX.enemyHitSfx(e.type, e.x); }
            }
            const beams = G.railBeams || (G.railBeams = []);
            while (beams.length >= 6) beams.shift();
            beams.push({ x, y0, y1: -10, t: 0, dur: 240, hits: hits.length });
            if (hits.length) G.hitstopT = Math.max(G.hitstopT || 0, 20);
            G.shkT = Math.max(G.shkT || 0, 90); G.shkM = Math.max(G.shkM || 0, 1.5);
        }
        function updateRailBeams(dtMs) {
            const beams = ctx.G.railBeams; if (!beams || !beams.length) return;
            let w = 0; for (const r of beams) { r.t += dtMs; if (r.t < r.dur) beams[w++] = r; } beams.length = w;
        }
        function flakShouldBurst(b, dtMs) {
            b.fuse -= dtMs;
            if (b.fuse <= 0 || b.y < 90) return true;
            const near = findNearestEnemy(b.x, b.y);
            if (!ctx.settings.reducedMotion && ctx.G.trails.length < 100 && Math.random() < 0.5) ctx.G.trails.push({ x: b.x + (Math.random() - 0.5) * 3, y: b.y + 6, vx: 0, vy: 18, life: 240, t: 0, col: 'rgba(255,170,60,0.55)', size: 2 });
            return !!near && Math.hypot(near.x - b.x, near.y - b.y) < 30;
        }
        function flakBurst(b) {
            const G = ctx.G, n = 8;
            areaDamage(b.x, b.y, 26, 1, null, { x: b.x, y: b.y, noSmolder: true });
            for (let i = 0; i < n; i++) {
                // Shards fan forward over ~220 degrees so the burst keeps hitting the formation ahead.
                const a = -Math.PI / 2 + (i - (n - 1) / 2) * (Math.PI * 1.22 / (n - 1));
                G.bul.push({ x: b.x, y: b.y, w: 3, h: 3, vx: Math.cos(a) * 300, vy: Math.sin(a) * 300, dmg: 1, life: 380, age: 0, flakShard: true, pierce: !!b.pierce });
            }
            expansionEffect('metal-sparks', b.x, b.y, 44);
            G.plasmaRings.push({ x: b.x, y: b.y, r: 0, maxR: 30, t: 0, dur: 200, col: '#ffb347' });
            if (ctx.SFX.flakBurst) ctx.SFX.flakBurst(b.x);
        }
        Object.assign(ctx, { isSeekerTarget, findNearestEnemy, shouldApplyPlayerHit, expansionEffect, areaDamage,
            fireRocketSalvo, rocketImpact, mirrorDuplicateBullets, pushPlayerMine, dropPlayerMines, updatePlayerMines,
            fireEvoWeapon, fireRailLance, flakShouldBurst, flakBurst, updateRailBeams });
    };
})();
