/* Aura Drift - projectiles, hazards, tows, hits and car effect timers (sections 2.5, 2.9).
   aura-drift-weapons.js spawns the state; everything here moves and resolves it.
   All randomness is race.rng; the ghost is never in race.cars, so it is never hit. */
(function () {
    'use strict';
    const AD = window.AuraDriftCore;
    const clamp = AD.clamp;
    const kit = AD.weaponKit;

    function grounded(car) { return Math.abs(car.v.z || 0) < 2.6; }
    function hitRadius(car) { return car.v.car.radius + 0.55; }

    /* spec: { by, x, y, damage, speed (multiplier), launch (vz), spin (yaw rate), heavy, weapon,
               shock (Bertha knockback x0.6), quiet (no 'hit' event, caller emits its own),
               disable, freeze (seconds), push {x, y, m} (world shove, divided by mass),
               breakShield }.
       Shield and cloak are honoured here for every weapon. Returns true if the hit landed. */
    AD.applyHit = function (race, target, spec) {
        const v = target.v;
        if (target.ghost || v.out || v.cloak > 0) return false;
        const by = spec.by === undefined ? -1 : spec.by;
        const x = spec.x === undefined ? v.x : spec.x, y = spec.y === undefined ? v.y : spec.y;
        const owner = by >= 0 ? race.cars[by] : null;
        function credit() {
            if (target.tally) target.tally.hitsTaken++;
            if (owner && owner.tally && owner !== target) owner.tally.hitsDealt++;
        }
        if (v.shield > 0) {
            if (spec.breakShield) {
                v.shield = 0;
                credit();
                AD.emit(race, 'shield_block', { x: x, y: y, car: target.index, by: by, broken: true });
                return true;
            }
            AD.emit(race, 'shield_block', { x: x, y: y, car: target.index, by: by });
            return false;
        }
        const mass = v.car.mass;
        const knock = spec.shock && v.car.quirk === 'heavy' ? v.car.quirkK : 1;
        v.damage = clamp(v.damage + (spec.damage || 0), 0, 100);
        if (spec.speed !== undefined) {
            v.vx *= spec.speed;
            if (v.airborne) { v.wx *= spec.speed; v.wy *= spec.speed; }
        }
        if (spec.spin) { v.r += spec.spin; v.spinKick = spec.spin > 0 ? 1 : -1; }
        if (spec.push) {
            const m = spec.push.m / mass * knock;
            const dx = spec.push.x * m, dy = spec.push.y * m;
            const cos = Math.cos(v.psi), sin = Math.sin(v.psi);
            if (v.airborne) { v.wx += dx; v.wy += dy; }
            v.vx += dx * cos + dy * sin;
            v.vy += -dx * sin + dy * cos;
        }
        if (spec.disable) v.disabled = Math.max(v.disabled, spec.disable);
        if (spec.freeze) v.frozen = Math.max(v.frozen, spec.freeze);
        if (spec.launch) AD.launch(race, target, spec.launch / mass * knock, 'hit');
        credit();
        if (!spec.quiet) AD.emit(race, 'hit', { x: x, y: y, car: target.index, by: by, heavy: !!spec.heavy, weapon: spec.weapon });
        return true;
    };

    /* ---- Projectiles ---- */
    function firstCar(race, p, extra) {
        for (const car of race.cars) {
            if (car.index === p.owner || car.v.out || car.v.cloak > 0) continue;
            if (Math.abs((car.v.z || 0) - (p.z || 0)) > 2.6) continue;   /* a jump clears ground fire */
            if (AD.distance(p.x, p.y, car.v.x, car.v.y) < hitRadius(car) + (extra || 0)) return car;
        }
        return null;
    }
    function validTarget(race, index) {
        const t = index === null || index === undefined ? null : race.cars[index];
        return t && !t.v.out && t.v.cloak <= 0 ? t : null;
    }
    function steer(p, tx, ty, rate, dt) {
        p.psi += clamp(AD.wrapAngle(Math.atan2(ty - p.y, tx - p.x) - p.psi), -rate * dt, rate * dt);
    }
    function advance(p, dt) {
        p.x += Math.cos(p.psi) * p.speed * dt;
        p.y += Math.sin(p.psi) * p.speed * dt;
    }
    /* Lobbed shots (frost, black hole, disco, cow) fly a parabola to a fixed point. */
    function lobStep(p, dt) {
        p.t += dt;
        const u = Math.min(1, p.t / p.flight);
        p.x = p.sx + (p.tx - p.sx) * u;
        p.y = p.sy + (p.ty - p.sy) * u;
        p.z = 0.6 + 4 * p.peak * u * (1 - u);
        p.vz = 4 * p.peak / p.flight * (1 - 2 * u);
        return u >= 1;
    }

    function strike(race, p, spec) {
        const car = firstCar(race, p, 0);
        if (!car) return false;
        const landed = AD.applyHit(race, car, Object.assign({ by: p.owner, x: p.x, y: p.y }, spec));
        const owner = race.cars[p.owner];
        if (landed && p.type === 'bullet' && owner && owner.tally) owner.tally.sausageHits++;
        return true;
    }

    function landGrenade(race, p) {
        race.hazards.push({ type: 'ice', x: p.tx, y: p.ty, r: 8, ttl: 8, age: 0, owner: p.owner });
        let frozen = 0;
        for (const car of race.cars) {
            if (car.index === p.owner || !grounded(car)) continue;
            if (AD.distance(car.v.x, car.v.y, p.tx, p.ty) >= 5) continue;
            if (AD.applyHit(race, car, { by: p.owner, x: p.tx, y: p.ty, quiet: true, freeze: 0.8, weapon: 'frost' })) frozen++;
        }
        AD.emit(race, 'frost_burst', { x: p.tx, y: p.ty, r: 8, frozen: frozen, by: p.owner });
    }
    function landCow(race, p) {
        const i = race.hazards.indexOf(p.shadow);
        if (i >= 0) race.hazards.splice(i, 1);
        let hits = 0;
        for (const car of race.cars) {
            if (car.index === p.owner || !grounded(car)) continue;
            if (AD.distance(car.v.x, car.v.y, p.tx, p.ty) >= 9) continue;
            if (AD.applyHit(race, car, {
                by: p.owner, x: p.tx, y: p.ty, weapon: 'cow', heavy: true, shock: true,
                damage: 12, speed: 0.6, launch: 6, spin: (race.rng() - 0.5) * 0.8
            })) hits++;
        }
        /* The cow itself is presentation: the renderer trots it off along `psi`. */
        AD.emit(race, 'cow_land', { x: p.tx, y: p.ty, r: 9, hits: hits, psi: p.psi, by: p.owner });
    }

    function attachHarpoon(race, p, target) {
        const owner = race.cars[p.owner];
        if (owner.tow) breakTow(race, owner, 'new');
        if (!AD.applyHit(race, target, { by: p.owner, quiet: true, weapon: 'harpoon' })) {
            AD.emit(race, 'harpoon_break', { car: p.owner, target: target.index, reason: 'shield' });
            return;
        }
        owner.tow = { target: target.index, t: 2.5 };
        owner.v.towing = target.index;
        target.v.towedBy = p.owner;
        race.beams.push({ type: 'tether', a: p.owner, b: target.index });
        AD.emit(race, 'harpoon_attach', { car: p.owner, target: target.index });
    }

    const STEP = {
        rocket(race, p, dt) {
            const t = validTarget(race, p.target);
            if (t) steer(p, t.v.x, t.v.y, 2.4, dt);
            advance(p, dt);
            return strike(race, p, { weapon: 'rocket', heavy: true, damage: 26, speed: 0.72, launch: 7 });
        },
        bullet(race, p, dt) {
            advance(p, dt);
            return strike(race, p, { weapon: 'mg', damage: 7, speed: 0.94 });
        },
        duck(race, p, dt) {
            let t = validTarget(race, p.target);
            if (!t) {
                p.look -= dt;
                if (p.look <= 0) {            /* re-acquire the nearest rival in front of the duck */
                    p.look = 0.25;
                    p.target = null;
                    let best = 60;
                    for (const car of race.cars) {
                        if (car.index === p.owner || car.v.out || car.v.cloak > 0) continue;
                        const r = kit.rel(p, car.v);
                        if (r.fwd > 0 && r.dist < best) { best = r.dist; p.target = car.index; }
                    }
                    t = validTarget(race, p.target);
                }
            }
            if (t) steer(p, t.v.x, t.v.y, 1.8, dt);
            p.wob += dt * 14;
            p.z = 0.5 * Math.abs(Math.sin(p.wob));
            advance(p, dt);
            const car = firstCar(race, p, 0);
            if (!car) return false;
            const spin = (race.rng() < 0.5 ? -1 : 1) * 1.1;
            if (AD.applyHit(race, car, { by: p.owner, x: p.x, y: p.y, weapon: 'ducks', damage: 3, speed: 0.93, spin: spin })) {
                AD.emit(race, 'duck_hit', { car: car.index, x: p.x, y: p.y, by: p.owner });
            }
            return true;
        },
        grenade(race, p, dt) {
            if (!lobStep(p, dt)) return false;
            landGrenade(race, p);
            return true;
        },
        hole_shot(race, p, dt) {
            if (!lobStep(p, dt)) return false;
            race.hazards.push({ type: 'black_hole', x: p.tx, y: p.ty, r: 26, ttl: 3.5, age: 0, pull: 22, owner: p.owner });
            AD.emit(race, 'hole_open', { x: p.tx, y: p.ty, r: 26, owner: p.owner });
            return true;
        },
        disco_shot(race, p, dt) {
            if (!lobStep(p, dt)) return false;
            race.hazards.push({ type: 'disco', x: p.tx, y: p.ty, r: 22, ttl: 4, z: 6, age: 0, owner: p.owner });
            AD.emit(race, 'disco_on', { x: p.tx, y: p.ty, r: 22, owner: p.owner });
            return true;
        },
        cow(race, p, dt) {
            if (!lobStep(p, dt)) return false;
            landCow(race, p);
            return true;
        },
        harpoon(race, p, dt) {
            const t = validTarget(race, p.target);
            if (t) p.psi = Math.atan2(t.v.y - p.y, t.v.x - p.x);
            advance(p, dt);
            if (t && AD.distance(p.x, p.y, t.v.x, t.v.y) < hitRadius(t) + 1) {
                attachHarpoon(race, p, t);
                return true;
            }
            return false;
        }
    };

    /* Timed projectiles resolve when their ttl runs out: charges, the glove wind-up, a missed harpoon. */
    const EXPIRE = {
        emp_charge(race, p) {
            const owner = race.cars[p.owner];
            if (!owner || owner.v.out) return;
            AD.emit(race, 'emp_pulse', { x: owner.v.x, y: owner.v.y, car: owner.index, r: 30 });
            for (const car of race.cars) {
                if (car === owner || car.v.out) continue;
                if (AD.distance(owner.v.x, owner.v.y, car.v.x, car.v.y) >= 30) continue;
                if (AD.applyHit(race, car, { by: owner.index, quiet: true, disable: 1.5, weapon: 'emp' })) {
                    AD.emit(race, 'emp_hit', { x: car.v.x, y: car.v.y, car: car.index, by: owner.index });
                }
            }
        },
        rail_charge(race, p) {
            const owner = race.cars[p.owner];
            if (!owner || owner.v.out) return;
            const cos = Math.cos(p.psi), sin = Math.sin(p.psi), L = 220;
            const x2 = p.x + cos * L, y2 = p.y + sin * L;
            race.beams.push({ type: 'rail', x: p.x, y: p.y, x2: x2, y2: y2, ttl: 0.25, car: p.owner });
            let hits = 0;
            for (const car of race.cars) {
                if (car === owner || car.v.out || car.v.cloak > 0 || !grounded(car)) continue;
                const dx = car.v.x - p.x, dy = car.v.y - p.y;
                const fwd = dx * cos + dy * sin, lat = Math.abs(-dx * sin + dy * cos);
                if (fwd < 0 || fwd > L || lat > 0.9 + car.v.car.radius * 0.5) continue;
                if (AD.applyHit(race, car, {
                    by: p.owner, x: car.v.x, y: car.v.y, weapon: 'railgun', heavy: true,
                    damage: 22, speed: 0.78, breakShield: true
                })) hits++;
            }
            AD.emit(race, 'rail_fire', { car: p.owner, x: p.x, y: p.y, x2: x2, y2: y2, hits: hits });
        },
        glove(race, p) {
            const owner = race.cars[p.owner];
            if (!owner || owner.v.out) return;
            const target = validTarget(race, p.target);
            let hit = false;
            if (target) {
                const r = kit.rel(owner.v, target.v);
                if (r.dist <= 8.75 && r.ang <= 0.9) {
                    const dx = target.v.x - owner.v.x, dy = target.v.y - owner.v.y, d = Math.max(0.01, Math.hypot(dx, dy));
                    hit = AD.applyHit(race, target, {
                        by: p.owner, x: target.v.x, y: target.v.y, weapon: 'glove', heavy: true, shock: true,
                        damage: 6, speed: 0.75, launch: 9, push: { x: dx / d, y: dy / d, m: 6 }
                    });
                }
            }
            AD.emit(race, 'glove', { car: p.owner, target: p.target === null ? -1 : p.target, x: owner.v.x, y: owner.v.y, hit: hit });
        },
        harpoon(race, p) {
            AD.emit(race, 'harpoon_break', { car: p.owner, target: p.target === null ? -1 : p.target, reason: 'miss' });
        }
    };

    AD.updateProjectiles = function (race, dt) {
        for (let i = race.beams.length - 1; i >= 0; i--) {
            const b = race.beams[i];
            if (b.ttl === undefined) continue;
            b.ttl -= dt;
            if (b.ttl <= 0) race.beams.splice(i, 1);
        }
        updateTows(race, dt);
        for (let i = race.projectiles.length - 1; i >= 0; i--) {
            const p = race.projectiles[i];
            p.ttl -= dt;
            let done = false;
            if (p.ttl <= 0) {
                done = true;
                if (EXPIRE[p.type]) EXPIRE[p.type](race, p);
            } else if (STEP[p.type]) {
                done = STEP[p.type](race, p, dt);
            }
            if (done) race.projectiles.splice(i, 1);
        }
    };

    /* ---- Harpoon tows ---- */
    function breakTow(race, car, reason) {
        const tow = car.tow;
        if (!tow) return;
        const target = race.cars[tow.target];
        car.tow = null;
        car.v.towing = -1;
        if (target) target.v.towedBy = -1;
        for (let i = race.beams.length - 1; i >= 0; i--) {
            if (race.beams[i].type === 'tether' && race.beams[i].a === car.index) race.beams.splice(i, 1);
        }
        AD.emit(race, 'harpoon_break', { car: car.index, target: tow.target, reason: reason });
    }

    function updateTows(race, dt) {
        for (const car of race.cars) {
            const tow = car.tow;
            if (!tow) continue;
            const target = race.cars[tow.target];
            const ov = car.v, tv = target.v;
            tow.t -= dt;
            const dist = AD.distance(ov.x, ov.y, tv.x, tv.y);
            let reason = null;
            if (ov.out || tv.out) reason = 'out';
            else if (ov.cloak > 0 || tv.cloak > 0) reason = 'cloak';
            else if (tv.shield > 0) reason = 'shield';
            else if (dist < 5) reason = 'caught';
            else if (dist > 80) reason = 'range';
            else if (tow.t <= 0) reason = 'time';
            else {
                /* A wall between the cars: the straight rope leaves the road corridor. */
                for (let k = 1; k <= 3 && !reason; k++) {
                    const u = k / 4;
                    const s = AD.sampleTrack(race.track, ov.x + (tv.x - ov.x) * u, ov.y + (tv.y - ov.y) * u);
                    if (Math.abs(s.offset) > s.halfWidth * 1.2) reason = 'wall';
                }
            }
            if (reason) { breakTow(race, car, reason); continue; }
            if (!ov.airborne && !tv.airborne) {
                const goal = Math.max(0, tv.vx) + 8;
                if (ov.vx < goal) ov.vx = Math.min(goal, ov.vx + 18 * dt);
                if (tv.vx > 0) tv.vx = Math.max(0, tv.vx - 3 * dt);   /* stand-in for power x0.6 plus rope drag */
            }
        }
    }

    /* ---- Hazards ---- */
    function closeHazard(race, h) {
        if (h.type === 'black_hole') AD.emit(race, 'hole_close', { x: h.x, y: h.y });
        else if (h.type === 'disco') AD.emit(race, 'disco_off', { x: h.x, y: h.y });
    }

    /* Horn: removes everything physical within r. The cow's shadow and charges stay. */
    const CLEAR_PROJECTILES = { rocket: 1, bullet: 1, duck: 1, grenade: 1, hole_shot: 1, disco_shot: 1, harpoon: 1 };
    const CLEAR_HAZARDS = { oil: 1, ice: 1, gnome_mine: 1, black_hole: 1, disco: 1 };
    AD.clearWeaponsIn = function (race, x, y, r) {
        let cleared = 0;
        for (let i = race.projectiles.length - 1; i >= 0; i--) {
            const p = race.projectiles[i];
            if (CLEAR_PROJECTILES[p.type] && AD.distance(p.x, p.y, x, y) < r) { race.projectiles.splice(i, 1); cleared++; }
        }
        for (let i = race.hazards.length - 1; i >= 0; i--) {
            const h = race.hazards[i];
            if (CLEAR_HAZARDS[h.type] && AD.distance(h.x, h.y, x, y) < r) {
                closeHazard(race, h);
                race.hazards.splice(i, 1);
                cleared++;
            }
        }
        return cleared;
    };

    function pullToward(v, dx, dy, accel, dt) {
        const d = Math.max(0.01, Math.hypot(dx, dy));
        const ax = dx / d * accel * dt, ay = dy / d * accel * dt;
        const cos = Math.cos(v.psi), sin = Math.sin(v.psi);
        if (v.airborne) { v.wx += ax; v.wy += ay; }
        v.vx += ax * cos + ay * sin;
        v.vy += -ax * sin + ay * cos;
    }

    AD.updateHazards = function (race, dt) {
        for (let i = race.hazards.length - 1; i >= 0; i--) {
            const h = race.hazards[i];
            h.ttl -= dt;
            h.age = (h.age || 0) + dt;
            if (h.arm > 0) h.arm -= dt;
            if (h.land !== undefined && h.land > 0) h.land -= dt;
            if (h.ttl <= 0) {
                closeHazard(race, h);
                race.hazards.splice(i, 1);
                continue;
            }
            if (h.type === 'gnome_mine') {
                if (h.arm > 0) continue;
                let blown = false;
                for (const car of race.cars) {
                    if (car.index === h.owner || car.v.out || car.v.cloak > 0 || !grounded(car)) continue;
                    if (AD.distance(h.x, h.y, car.v.x, car.v.y) >= h.r + 1.4) continue;
                    blown = true;
                    AD.applyHit(race, car, {
                        by: h.owner, x: h.x, y: h.y, weapon: 'gnome_mine', heavy: true, shock: true,
                        damage: 16, speed: 0.68, launch: 6, spin: (race.rng() - 0.5) * 0.7
                    });
                }
                if (blown) race.hazards.splice(i, 1);
            } else if (h.type === 'black_hole') {
                for (const car of race.cars) {
                    const v = car.v;
                    if (v.out || v.cloak > 0) continue;
                    const d = AD.distance(h.x, h.y, v.x, v.y);
                    if (d >= h.r) continue;
                    pullToward(v, h.x - v.x, h.y - v.y, h.pull * (1 - d / h.r) * (v.shield > 0 ? 0.5 : 1), dt);
                }
            } else if (h.type === 'disco') {
                for (const car of race.cars) {
                    const v = car.v;
                    if (car.index === h.owner || v.out || v.cloak > 0 || v.shield > 0) continue;
                    if (AD.distance(h.x, h.y, v.x, v.y) < h.r) v.steerWobble = Math.max(v.steerWobble, 0.3);
                }
            }
        }
    };

    /* Hazards that change how the road grips: oil and ice. The owner is safe from their own
       ice, and from their own oil for the first second. Cloaked cars ignore all hazards. */
    AD.applyHazardEnv = function (race, actor, env) {
        const v = actor.v;
        if (v.cloak > 0) return;
        for (const h of race.hazards) {
            if (h.type !== 'oil' && h.type !== 'ice') continue;
            if (AD.distance(v.x, v.y, h.x, h.y) >= h.r) continue;
            if (h.owner === actor.index && (h.type === 'ice' || h.age < 1)) continue;
            env.grip = Math.min(env.grip, h.type === 'oil' ? AD.SURFACES.oil.grip : 0.3);
        }
    };

    /* Timers of frozen, steerWobble, heat, toast, cloak and overdrive (physics owns the rest). */
    AD.updateCarEffects = function (race, car, dt) {
        const v = car.v;
        if (v.frozen > 0) v.frozen = Math.max(0, v.frozen - dt);
        if (v.steerWobble > 0) v.steerWobble = Math.max(0, v.steerWobble - dt);
        const wasToast = v.toast > 0;
        if (v.toast > 0) v.toast = Math.max(0, v.toast - dt);
        if (v.heat >= 1) {
            v.toast = Math.max(v.toast, 0.6);
            v.heat = 0;
            if (!wasToast) AD.emit(race, 'toast', { car: car.index });
        } else if (v.heat > 0) {
            v.heat = Math.max(0, v.heat - 0.25 * dt);
        }
        if (v.cloak > 0) {
            v.cloak = Math.max(0, v.cloak - dt);
            if (v.cloak === 0) AD.emit(race, 'cloak_off', { car: car.index });
        }
        if (v.overdrive > 0) {
            v.overdrive = Math.max(0, v.overdrive - dt);
            if (v.overdrive === 0) AD.emit(race, 'overdrive_off', { car: car.index });
        }
    };
})();
