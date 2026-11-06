/* Aura Drift - projectiles, hazards, hits and car effect timers.
   T1.1 state: the original rocket, bullet, oil and mine behaviour on top of the
   final `applyHit` entry point. T1.4 replaces this file with the full set. */
(function () {
    'use strict';
    const AD = window.AuraDriftCore;
    const clamp = AD.clamp;

    /* spec: { by, x, y, damage, speed (multiplier), launch (vz), spin, heavy, weapon }.
       Shield and cloak are honoured here for every weapon. Returns true if the
       hit landed. */
    AD.applyHit = function (race, target, spec) {
        const v = target.v;
        if (v.out || v.cloak > 0) return false;
        const x = spec.x === undefined ? v.x : spec.x, y = spec.y === undefined ? v.y : spec.y;
        if (v.shield > 0) {
            AD.emit(race, 'shield_block', { x: x, y: y, car: target.index, by: spec.by });
            return false;
        }
        v.damage = clamp(v.damage + (spec.damage || 0), 0, 100);
        if (spec.speed !== undefined) v.vx *= spec.speed;
        if (spec.spin) v.r += spec.spin;
        if (spec.launch) AD.launch(race, target, spec.launch / target.v.car.mass, 'hit');
        if (target.tally) target.tally.hitsTaken++;
        const owner = spec.by >= 0 ? race.cars[spec.by] : null;
        if (owner && owner.tally && owner !== target) owner.tally.hitsDealt++;
        AD.emit(race, 'hit', { x: x, y: y, car: target.index, by: spec.by, heavy: !!spec.heavy, weapon: spec.weapon });
        return true;
    };

    AD.updateProjectiles = function (race, dt) {
        for (let i = race.projectiles.length - 1; i >= 0; i--) {
            const p = race.projectiles[i];
            p.ttl -= dt;
            if (p.ttl <= 0) { race.projectiles.splice(i, 1); continue; }
            if (p.type === 'rocket' && p.target !== null) {
                const target = race.cars[p.target];
                if (target) {
                    const wanted = Math.atan2(target.v.y - p.y, target.v.x - p.x);
                    p.psi += clamp(AD.wrapAngle(wanted - p.psi), -2.4 * dt, 2.4 * dt);
                }
            }
            p.x += Math.cos(p.psi) * p.speed * dt;
            p.y += Math.sin(p.psi) * p.speed * dt;
            let hit = false;
            for (const car of race.cars) {
                if (car.index === p.owner || car.v.out || car.v.cloak > 0) continue;
                if (AD.distance(p.x, p.y, car.v.x, car.v.y) < 2.6) {
                    const rocket = p.type === 'rocket';
                    AD.applyHit(race, car, {
                        by: p.owner, x: p.x, y: p.y, weapon: rocket ? 'rocket' : 'mg', heavy: rocket,
                        damage: rocket ? 26 : 7, speed: rocket ? 0.72 : 0.94, launch: rocket ? 7 : 0
                    });
                    hit = true;
                    break;
                }
            }
            if (hit) race.projectiles.splice(i, 1);
        }
    };

    AD.updateHazards = function (race, dt) {
        for (let i = race.hazards.length - 1; i >= 0; i--) {
            const h = race.hazards[i];
            h.ttl -= dt;
            if (h.arm > 0) h.arm -= dt;
            if (h.ttl <= 0) { race.hazards.splice(i, 1); continue; }
            if (h.type !== 'gnome_mine' || h.arm > 0) continue;
            for (const car of race.cars) {
                if (car.index === h.owner || car.v.out || car.v.cloak > 0) continue;
                if (AD.distance(h.x, h.y, car.v.x, car.v.y) < h.r + 1.4) {
                    AD.applyHit(race, car, {
                        by: h.owner, x: h.x, y: h.y, weapon: 'gnome_mine', heavy: true,
                        damage: 16, speed: 0.68, launch: 6, spin: (race.rng() - 0.5) * 0.7
                    });
                    race.hazards.splice(i, 1);
                    break;
                }
            }
        }
    };

    /* Hazards that change how the road grips (oil today; ice and black holes in T1.4). */
    AD.applyHazardEnv = function (race, actor, env) {
        const v = actor.v;
        for (const h of race.hazards) {
            if (h.type === 'oil' && h.ttl > 0 && AD.distance(v.x, v.y, h.x, h.y) < h.r) {
                env.grip = Math.min(env.grip, AD.SURFACES.oil.grip);
            }
        }
    };

    /* T1.4 owns the timers of frozen, steerWobble, heat, toast, cloak and overdrive. */
    AD.updateCarEffects = function (race, car, dt) {};
})();
