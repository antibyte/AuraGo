/* Runenpfad enemy behaviour profiles. DOM-free; the simulation supplies the tile helpers. */
(function (root) {
    'use strict';

    const SHOOTER_AGGRO = 560;

    // Gunner AI: keeps 200-340 px from the hero (backs off when close, approaches when far) and never walks off a ledge while engaged.
    function shooterIntent(sim, e, api, distToPlayer) {
        const def = e.def;
        const stage = sim.stage;
        const speed = Math.abs(def.speed) * (e.phase === 2 ? 1.38 : 1.0);
        const dir = sim.player.x >= e.x ? 1 : -1;
        if (distToPlayer > SHOOTER_AGGRO) {
            if (e.vx === 0) e.vx = (e.facing || 1) * speed;
            return;
        }
        if (distToPlayer < 200) e.vx = -dir * speed;
        else if (distToPlayer > 340) e.vx = dir * speed;
        else e.vx = 0;
        if (e.vx !== 0 && e.onGround) {
            const aheadX = Math.floor((e.x + Math.sign(e.vx) * 20) / stage.tileSize);
            const groundY = Math.floor((e.y + 4) / stage.tileSize);
            if (!api.isSolidTile(stage, aheadX, groundY)) e.vx = 0;
        }
    }

    // Each behaviour decides the horizontal intent of an enemy. Movement resolution is shared.
    function update(sim, e, dt, api) {
        const p = sim.player;
        const stage = sim.stage;
        const def = e.def;
        const Bosses = root.RunenpfadBosses;
        if (def.boss && Bosses && typeof Bosses.update === 'function' && Bosses.update(sim, e, dt, api)) return;
        const speedMul = e.phase === 2 ? 1.38 : 1.0;
        const distToPlayer = Math.abs(p.x - e.x);
        const prevY = e.y;

        if (def.flying) {
            e.x += e.vx * speedMul * dt;
            if (e.x <= e.minX) { e.x = e.minX; e.vx = Math.abs(def.speed); }
            if (e.x >= e.maxX) { e.x = e.maxX; e.vx = -Math.abs(def.speed); }
            e.y = e.spawnY - 88 + Math.sin(e.animTime * 2.6) * 28;
            e.facing = e.vx >= 0 ? 1 : -1;
            return;
        }

        const behavior = def.behavior;
        const stationary = behavior === 'turret';
        const engaged = behavior === 'shooter' && distToPlayer <= SHOOTER_AGGRO;
        const chases = behavior === 'charger' || behavior === 'boss_ground' || behavior === 'boss_jumper';

        if (stationary) {
            e.vx = 0;
        } else if (behavior === 'ambusher') {
            if (!e.ambushTriggered) {
                e.vx = 0;
                if (distToPlayer < 230 && Math.abs(p.y - e.y) < 150) {
                    e.ambushTriggered = true;
                    e.ambushTimer = 0.45;
                }
            } else if (e.ambushTimer > 0) {
                e.ambushTimer -= dt;
                e.vx = 0;
            } else {
                e.vx = (p.x >= e.x ? 1 : -1) * def.speed * speedMul * 1.45;
            }
        } else if (behavior === 'bomber') {
            if (distToPlayer < 320 && Math.abs(p.y - e.y) < 140) {
                e.vx = (p.x >= e.x ? 1 : -1) * def.speed * speedMul * 1.3;
            }
        } else if (behavior === 'shooter') {
            shooterIntent(sim, e, api, distToPlayer);
        } else if (chases) {
            if (distToPlayer < 240 && Math.abs(p.y - e.y) < 120) {
                e.vx = (p.x >= e.x ? 1 : -1) * def.speed * speedMul * 1.15;
            }
        }

        const bodyW = 30 * def.scale;
        const bodyH = 42 * def.scale;
        e.x += e.vx * dt;
        const hitWall = api.resolveHorizontalTiles(stage, e, bodyW, bodyH);
        const outOfBounds = e.x <= e.minX || e.x >= e.maxX;
        if (hitWall || outOfBounds) {
            if (e.x < e.minX) e.x = e.minX;
            if (e.x > e.maxX) e.x = e.maxX;
            if (!stationary && !engaged && !(behavior === 'ambusher' && !e.ambushTriggered)) {
                e.vx = e.x <= e.minX
                    ? Math.abs(def.speed) * speedMul
                    : (e.x >= e.maxX ? -Math.abs(def.speed) * speedMul : -e.vx);
            }
        }

        if ((behavior === 'jumper' || behavior === 'boss_jumper') && e.onGround && distToPlayer < 260 && (sim.stepCount % 95 === 0)) {
            e.vy = -440;
            e.onGround = false;
        }

        e.vy = Math.min(api.MAX_FALL, e.vy + api.GRAVITY * dt);
        e.y += e.vy * dt;
        api.resolveVerticalTiles(stage, e, bodyW, bodyH, prevY, false);
        e.facing = e.vx > 0 ? 1 : (e.vx < 0 ? -1 : e.facing);

        if (engaged) e.facing = p.x >= e.x ? 1 : -1;
        if (!stationary && !engaged) {
            const checkAheadX = Math.floor((e.x + e.facing * 20) / stage.tileSize);
            const groundAheadY = Math.floor((e.y + 4) / stage.tileSize);
            if (e.onGround && !api.isSolidTile(stage, checkAheadX, groundAheadY)) {
                e.vx = -e.vx;
            }
        }
    }

    root.RunenpfadBehaviors = {
        update: update
    };
})(typeof window !== 'undefined' ? window : globalThis);
