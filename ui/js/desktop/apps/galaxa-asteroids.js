(function () {
    'use strict';
    const GC = window.GalaxaCore = window.GalaxaCore || {};
    GC.createAsteroids = function (ctx) {
        function startAsteroids() {
            const G = ctx.G;
            G.asteroids = []; G.asteroidSpawnT = 0;
            G.asteroidLane = Math.max(100, Math.min(ctx.W - 100, G.p.x));
            G.asteroidEncounter = G.stage >= 6 && !G.chal && !G.bonusStage && !G.encounterBoss && !ctx.isMiniBossStage() && ctx.settings.mode !== 'boss_rush';
        }
        function spawnAsteroid() {
            const G = ctx.G;
            if (!G.asteroidEncounter || G.asteroids.length >= ctx.ASTEROID_LIMIT) return null;
            const size = Math.floor(ctx.random() * 3), r = ctx.ASTEROID_RADIUS[size];
            // Reserve the extra slot that each large rock will need when splitting.
            const reserved = G.asteroids.reduce((n, a) => n + (a.hp > 0 ? a.size === 2 && !a.fragment ? 2 : 1 : 0), 0);
            if (reserved + (size === 2 ? 2 : 1) > ctx.ASTEROID_LIMIT) return null;
            const side = ctx.random() < 0.5 ? -1 : 1;
            const low = side < 0 ? r + 6 : G.asteroidLane + 80 + r;
            const high = side < 0 ? G.asteroidLane - 80 - r : ctx.W - r - 6;
            if (high < low) return null;
            const a = { x: low + ctx.random() * (high - low), y: -r - 2, vx: 0,
                vy: 48 + ctx.random() * 32, r, size, hp: ctx.ASTEROID_HP[size],
                art: ctx.ASTEROID_ART[Math.floor(ctx.random() * ctx.ASTEROID_ART.length)],
                warning: ctx.ASTEROID_WARNING_MS, age: 0, root: { rewarded: false } };
            G.asteroids.push(a); return a;
        }
        function damageAsteroid(a, damage) {
            if (a.hp <= 0 || a.warning > 0 || damage <= 0) return false;
            a.hp -= damage;
            if (a.hp > 0) return false;
            ctx.expansionEffect(a.art.effect, a.x, a.y, a.r * 3);
            if (!a.root.rewarded) { a.root.rewarded = true; ctx.addScore(100, a.x, a.y, '#d5b58e'); }
            ctx.G.asteroids = ctx.G.asteroids.filter(o => o.hp > 0);
            if (a.size === 2 && !a.fragment && ctx.G.asteroids.filter(o => o.hp > 0).length + 2 <= ctx.ASTEROID_LIMIT) {
                for (const side of [-1, 1]) {
                    const r = ctx.ASTEROID_RADIUS[0];
                    let x = a.x + side * 16;
                    // Fragments stay on their parent's side of the safe corridor.
                    x = a.x < ctx.G.asteroidLane ? Math.min(x, ctx.G.asteroidLane - 80 - r) : Math.max(x, ctx.G.asteroidLane + 80 + r);
                    ctx.G.asteroids.push({ ...a, x: Math.max(r, Math.min(ctx.W - r, x)), r, size: 0, hp: ctx.ASTEROID_HP[0],
                        vy: a.vy * 1.25, warning: 0, fragment: true, age: 0 });
                }
            }
            return true;
        }
        function intersects(a, b) {
            const x = Math.max(b.x, Math.min(a.x, b.x + b.w));
            const y = Math.max(b.y, Math.min(a.y, b.y + b.h));
            return (a.x - x) ** 2 + (a.y - y) ** 2 <= a.r ** 2;
        }
        function shotHits(a, b) {
            const x = Math.min(b.x, b.prevX ?? b.x), y = Math.min(b.y, b.prevY ?? b.y);
            return intersects(a, { x, y, w: (b.w || 3) + Math.abs(b.x - (b.prevX ?? b.x)), h: (b.h || 3) + Math.abs(b.y - (b.prevY ?? b.y)) });
        }
        function hitAsteroidShot(b) {
            for (const a of ctx.G.asteroids || []) {
                if (a.hp <= 0 || a.warning > 0 || !shotHits(a, b) || (b.hitTargets && b.hitTargets.has(a))) continue;
                if (!b.hitTargets) b.hitTargets = new Set(); b.hitTargets.add(a);
                if (ctx.shouldApplyPlayerHit(b)) {
                    ctx.G.stageAccuracyHits = (ctx.G.stageAccuracyHits || 0) + 1;
                    damageAsteroid(a, b.dmg ?? 1);
                    if (b.rocket) ctx.rocketImpact(b, a);
                }
                if (b.rocket || (!b.pierce && !b.laser)) return true;
            }
            return false;
        }
        function hitAsteroidEnemyShot(b) {
            return (ctx.G.asteroids || []).some(a => a.hp > 0 && a.warning <= 0 && shotHits(a, b));
        }
        function damageAsteroidsInRadius(x, y, radius, damage, excluded) {
            for (const a of (ctx.G.asteroids || []).slice()) {
                if (a !== excluded && Math.hypot(a.x - x, a.y - y) <= radius + a.r) damageAsteroid(a, damage);
            }
        }
        function updateAsteroids(dt) {
            const G = ctx.G;
            if (!G.asteroidEncounter || G.chal || G.encounterBoss) { G.asteroids = []; return; }
            G.asteroidSpawnT -= dt * 1000;
            if (G.asteroidSpawnT <= 0) { spawnAsteroid(); G.asteroidSpawnT = 3000; }
            for (const a of G.asteroids) {
                a.age += dt * 1000;
                if (a.warning > 0) { a.warning -= dt * 1000; continue; }
                a.y += a.vy * dt;
                if (a.hp > 0 && G.p.alive && G.p.inv <= 0 && intersects(a, ctx.getShipHitbox())) {
                    ctx.killP(); damageAsteroid(a, a.hp);
                }
            }
            G.asteroids = G.asteroids.filter(a => a.hp > 0 && a.y - a.r <= ctx.H + 20);
        }
        function drawAsteroids(c) {
            for (const a of ctx.G.asteroids || []) {
                if (a.warning > 0) {
                    c.save(); c.strokeStyle = '#ffce79'; c.lineWidth = 2;
                    c.beginPath(); c.moveTo(a.x - 7, 16); c.lineTo(a.x, 25); c.lineTo(a.x + 7, 16); c.stroke();
                    c.setLineDash([3, 8]); c.globalAlpha = 0.45; c.beginPath(); c.moveTo(a.x, 30); c.lineTo(a.x, 100); c.stroke(); c.restore();
                } else ctx.drawAnimation(c, 'asteroid.' + a.art.id, a.x, a.y, ctx.settings.reducedMotion ? 0 : a.age, a.r * 2.5);
            }
        }
        Object.assign(ctx, { startAsteroids, spawnAsteroid, damageAsteroid, hitAsteroidShot, hitAsteroidEnemyShot,
            damageAsteroidsInRadius, updateAsteroids, drawAsteroids });
    };
})();
