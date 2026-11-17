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

    // Chase intent with a dead zone: stops when level with the hero (hysteresis 10/26 px) and only turns on a clear difference.
    function chaseIntent(sim, e, api, speed) {
        const dx = sim.player.x - e.x;
        const ax = Math.abs(dx);
        const stopped = e.chaseStopped ? ax <= 26 : ax < 10;
        e.chaseStopped = stopped;
        e.chasing = true;
        e.vx = stopped ? 0 : (dx >= 0 ? 1 : -1) * speed;
        if (ax > 6) e.facing = dx >= 0 ? 1 : -1;
        if (e.vx !== 0 && e.onGround) {
            const ts = sim.stage.tileSize;
            if (!api.isSolidTile(sim.stage, Math.floor((e.x + Math.sign(e.vx) * 20) / ts), Math.floor((e.y + 4) / ts))) e.vx = 0;
        }
    }

    function pushEvent(sim, type, detail) { root.RunenpfadPhysics.pushEvent(sim, type, detail); }

    // Sporecap: every ~2.6 s a spore is lobbed in a gravity arc at the hero's feet; combat turns it into a poison cloud.
    function spitterThink(sim, e, dt) {
        const p = sim.player;
        e.spitTimer = (e.spitTimer == null ? 1.2 : e.spitTimer) - dt;
        if (e.spitTimer > 0) return;
        if (Math.abs(p.x - e.x) > 320 || Math.abs(p.y - e.y) > 240) {
            e.spitTimer = 0.25;
            return;
        }
        e.spitTimer = 2.6;
        const flight = 0.85;
        const grav = 900;
        const dir = p.x >= e.x ? 1 : -1;
        const x0 = e.x + dir * 14;
        const y0 = e.y - 34;
        e.facing = dir;
        e.attackTimer = 0.45;
        e.attackKind = 'shot';
        pushEvent(sim, 'enemy_attack', { id: e.id, defId: e.defId, kind: 'lob', x: e.x, y: e.y });
        sim.projectiles.push({
            x: x0, y: y0, vx: (p.x - x0) / flight, vy: ((p.y - 6) - y0 - 0.5 * grav * flight * flight) / flight,
            gravity: grav, fromEnemy: true, spore: true, kind: 'spore', damage: 1, radius: 8, life: 2.4,
            color: '#a3e635', noDeflect: true, dead: false
        });
    }

    // Drakeling: hovers 180-260 px from the hero and spits an aimed fireball every ~2.4 s after a short wind-up.
    function drakelingUpdate(sim, e, dt) {
        const p = sim.player;
        const stage = sim.stage;
        const dx = p.x - e.x;
        const dist = Math.abs(dx);
        const speed = Math.abs(e.def.speed) * (e.phase === 2 ? 1.38 : 1.0);
        const engaged = dist < 560 && Math.abs(p.y - e.y) < 420;
        let vx;
        if (engaged) {
            vx = dist < 180 ? -Math.sign(dx || 1) * speed : (dist > 260 ? Math.sign(dx) * speed : 0);
            e.facing = dx >= 0 ? 1 : -1;
        } else {
            if (e.vx === 0) e.vx = (e.facing || 1) * speed;
            vx = e.vx;
            if (e.x <= e.minX) vx = Math.abs(speed);
            if (e.x >= e.maxX) vx = -Math.abs(speed);
            e.facing = vx >= 0 ? 1 : -1;
        }
        e.vx = vx;
        e.x = Math.max(30, Math.min(stage.width - 30, e.x + vx * dt));
        e.y = e.spawnY - 110 + Math.sin(e.animTime * 2.4) * 22;
        if (e.windup > 0) {
            e.windup = Math.max(0, e.windup - dt);
            if (e.windup === 0) drakelingSpit(sim, e);
            return;
        }
        e.spitTimer = (e.spitTimer == null ? 1.6 : e.spitTimer) - dt;
        if (e.spitTimer > 0) return;
        if (engaged && dist < 480) {
            e.windup = 0.3;
            pushEvent(sim, 'enemy_windup', { id: e.id, defId: e.defId, x: e.x, y: e.y });
        } else {
            e.spitTimer = 0.3;
        }
    }

    function drakelingSpit(sim, e) {
        const p = sim.player;
        const x0 = e.x + (e.facing || 1) * 16;
        const y0 = e.y - 24;
        const dx = p.x - x0;
        const dy = (p.y - 22) - y0;
        const len = Math.max(1, Math.hypot(dx, dy));
        const spd = 230;
        e.spitTimer = 2.4;
        e.attackTimer = 0.45;
        e.attackKind = 'shot';
        pushEvent(sim, 'enemy_attack', { id: e.id, defId: e.defId, kind: 'fireball', x: e.x, y: e.y });
        sim.projectiles.push({
            x: x0, y: y0, vx: (dx / len) * spd, vy: (dy / len) * spd, fromEnemy: true, kind: 'fireball', damage: 1,
            radius: 9, blast: 44, life: 2.6, color: '#fb923c', noDeflect: true, dead: false
        });
    }

    // Crystal bat: perches, winds up when the hero is below, dives on a curve to the hero's position, then climbs back.
    // Returns true while it owns the movement (wind-up, dive, climb); false means "hover like a normal flyer".
    function swooperUpdate(sim, e, dt) {
        const p = sim.player;
        const s = e.swoop || (e.swoop = { state: 'perch', cool: 1.0, t: 0, u: 0, sx: 0, sy: 0, tx: 0, ty: 0 });
        const perchX = e.spawnX;
        const perchY = e.spawnY - 88;
        if (s.state === 'perch') {
            s.cool -= dt;
            const hy = p.y - 22;
            if (s.cool <= 0 && hy > e.y + 30 && Math.hypot(p.x - e.x, hy - e.y) < 300) {
                s.state = 'wind';
                s.t = 0.35;
                e.windup = 0.35;
                e.facing = p.x >= e.x ? 1 : -1;
            }
            return false;
        }
        if (s.state === 'wind') {
            s.t -= dt;
            e.windup = Math.max(0, s.t);
            if (s.t <= 0) {
                s.state = 'dive';
                s.u = 0;
                s.sx = e.x;
                s.sy = e.y;
                s.tx = p.x;
                s.ty = Math.min(p.y - 22, sim.stage.groundY - 10);
                e.diving = true;
                pushEvent(sim, 'enemy_attack', { id: e.id, defId: e.defId, kind: 'dive', x: e.x, y: e.y });
            }
            return true;
        }
        if (s.state === 'dive') {
            s.u = Math.min(1, s.u + dt / 0.8);
            const u = s.u;
            // Quadratic curve: straight down first, then sweeping through the target.
            e.x = (1 - u) * (1 - u) * s.sx + 2 * u * (1 - u) * s.sx + u * u * s.tx;
            e.y = (1 - u) * (1 - u) * s.sy + 2 * u * (1 - u) * s.ty + u * u * s.ty;
            e.vx = s.tx >= s.sx ? Math.abs(e.def.speed) : -Math.abs(e.def.speed);
            e.facing = e.vx > 0 ? 1 : -1;
            if (u >= 1) {
                s.state = 'climb';
                e.diving = false;
            }
            return true;
        }
        const dx = perchX - e.x;
        const dy = perchY - e.y;
        const len = Math.hypot(dx, dy);
        if (len < 8) {
            s.state = 'perch';
            s.cool = 1.8;
            return false;
        }
        const step = Math.min(len, 230 * dt);
        e.x += (dx / len) * step;
        e.y += (dy / len) * step;
        e.vx = dx >= 0 ? Math.abs(e.def.speed) : -Math.abs(e.def.speed);
        e.facing = e.vx > 0 ? 1 : -1;
        return true;
    }

    // Frost wraith: drifts toward the hero, visible 2.4 s then phased (intangible) 1.4 s.
    function phaserUpdate(sim, e, dt) {
        const p = sim.player;
        e.phaseT = (e.phaseT == null ? 2.4 : e.phaseT) - dt;
        if (e.phaseT <= 0) {
            e.phased = !e.phased;
            e.phaseT = e.phased ? 1.4 : 2.4;
            pushEvent(sim, 'enemy_phase', { id: e.id, defId: e.defId, phased: e.phased, x: e.x, y: e.y });
        }
        const near = Math.hypot(p.x - e.x, p.y - e.y) < 480;
        const tx = near ? p.x + (e.x >= p.x ? 24 : -24) : Math.min(e.maxX, Math.max(e.minX, e.spawnX + Math.sin(e.animTime * 0.7) * 80));
        const ty = near ? Math.min(p.y - 14, sim.stage.groundY - 10) : e.spawnY - 88 + Math.sin(e.animTime * 2.2) * 20;
        const spd = Math.abs(e.def.speed) * 0.8;
        const dx = tx - e.x;
        const dy = ty - e.y;
        const len = Math.hypot(dx, dy);
        if (len > 2) {
            const step = Math.min(len, spd * dt);
            e.x += (dx / len) * step;
            e.y += (dy / len) * step;
        }
        e.vx = Math.abs(dx) > 4 ? Math.sign(dx) * spd : 0;
        if (e.vx !== 0) e.facing = e.vx > 0 ? 1 : -1;
    }

    // Clock knight: 0.6 s wind-up when the hero is close, then a hammer slam with a ground shock wave each way.
    // Returns true while the knight stands still (wind-up and recovery).
    function bruteFrozen(sim, e, dt) {
        const p = sim.player;
        e.brute = e.brute || 'idle';
        e.cool = Math.max(0, (e.cool || 0) - dt);
        if (e.brute === 'windup') {
            e.windup -= dt;
            e.vx = 0;
            if (e.windup <= 0) {
                e.windup = 0;
                e.brute = 'recover';
                e.recoverT = 0.9;
                bruteSlam(sim, e);
            }
            return true;
        }
        if (e.brute === 'recover') {
            e.recoverT -= dt;
            e.vx = 0;
            if (e.recoverT <= 0) {
                e.brute = 'idle';
                e.cool = 1.4;
                e.vx = (e.facing || 1) * Math.abs(e.def.speed);
            }
            return true;
        }
        if (e.cool <= 0 && e.onGround && Math.abs(p.x - e.x) < 170 && Math.abs(p.y - e.y) < 120) {
            e.brute = 'windup';
            e.windup = 0.6;
            e.vx = 0;
            e.facing = p.x >= e.x ? 1 : -1;
            pushEvent(sim, 'enemy_windup', { id: e.id, defId: e.defId, x: e.x, y: e.y });
            return true;
        }
        return false;
    }

    function bruteSlam(sim, e) {
        const P = root.RunenpfadPhysics;
        pushEvent(sim, 'enemy_slam', { id: e.id, defId: e.defId, x: e.x, y: e.y });
        e.attackTimer = 0.45;
        e.attackKind = 'slam';
        P.fxBurst(sim, 'dust', e.x, e.y, 12, { color: '#e8d8b8', speed: 200, arc: Math.PI, angle: Math.PI / 2, life: 0.5 });
        P.fxShake(sim, 5);
        [-1, 1].forEach(function (dir) {
            sim.projectiles.push({
                x: e.x + dir * 30, y: e.y - 14, vx: dir * 250, vy: 0, fromEnemy: true, damage: 1, radius: 14,
                life: 0.9, color: '#fbbf24', kind: 'shock', groundHug: true, noDeflect: true, dead: false
            });
        });
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

        if (def.behavior === 'phaser') {
            phaserUpdate(sim, e, dt);
            return;
        }
        if (def.behavior === 'drakeling') {
            drakelingUpdate(sim, e, dt);
            return;
        }
        if (def.flying) {
            if (def.behavior === 'swooper' && swooperUpdate(sim, e, dt)) return;
            e.x += e.vx * speedMul * dt;
            if (e.x <= e.minX) { e.x = e.minX; e.vx = Math.abs(def.speed); }
            if (e.x >= e.maxX) { e.x = e.maxX; e.vx = -Math.abs(def.speed); }
            e.y = e.spawnY - 88 + Math.sin(e.animTime * 2.6) * 28;
            e.facing = e.vx >= 0 ? 1 : -1;
            return;
        }

        const behavior = def.behavior;
        if (behavior === 'spitter') spitterThink(sim, e, dt);
        const stationary = behavior === 'turret' || (behavior === 'brute' && bruteFrozen(sim, e, dt));
        const engaged = behavior === 'shooter' && distToPlayer <= SHOOTER_AGGRO;
        e.chasing = false;
        const chases = behavior === 'charger' || behavior === 'shielded' || behavior === 'boss_ground' || behavior === 'boss_jumper';

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
                chaseIntent(sim, e, api, Math.abs(def.speed) * speedMul * 1.45);
            }
        } else if (behavior === 'bomber') {
            if (distToPlayer < 320 && Math.abs(p.y - e.y) < 140) {
                chaseIntent(sim, e, api, Math.abs(def.speed) * speedMul * 1.3);
            } else if (e.vx === 0) {
                e.vx = (e.facing || 1) * Math.abs(def.speed) * speedMul;
            }
        } else if (behavior === 'shooter') {
            shooterIntent(sim, e, api, distToPlayer);
        } else if (chases) {
            if (distToPlayer < 240 && Math.abs(p.y - e.y) < 120) {
                chaseIntent(sim, e, api, Math.abs(def.speed) * speedMul * 1.15);
            } else if (e.vx === 0) {
                e.vx = (e.facing || 1) * Math.abs(def.speed) * speedMul;
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
            if (e.chasing) e.vx = 0;
            else if (!stationary && !engaged && !(behavior === 'ambusher' && !e.ambushTriggered)) {
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
        if (!stationary && !engaged && e.vx !== 0) {
            const checkAheadX = Math.floor((e.x + Math.sign(e.vx) * 20) / stage.tileSize);
            const groundAheadY = Math.floor((e.y + 4) / stage.tileSize);
            if (e.onGround && !api.isSolidTile(stage, checkAheadX, groundAheadY)) {
                e.vx = e.chasing ? 0 : -e.vx;
            }
        }
    }

    root.RunenpfadBehaviors = {
        update: update
    };
})(typeof window !== 'undefined' ? window : globalThis);
