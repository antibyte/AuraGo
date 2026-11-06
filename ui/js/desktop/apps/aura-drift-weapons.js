/* Aura Drift - weapon table, roulette, firing, flamethrower and AI predicates (section 2.9).
   Firing only spawns state (hazards, projectiles, timers); aura-drift-projectiles.js moves
   and resolves it. Everything random goes through race.rng. */
(function () {
    'use strict';
    const AD = window.AuraDriftCore;
    const clamp = AD.clamp;

    function weapon(id, ammo, cooldown, range, hold, colour, front, mid, back, item) {
        const def = {
            id: id, ammo: ammo, cooldown: cooldown, range: range, colour: colour, hold: hold,
            nameKey: 'aura_drift.weapon.' + id + '.name', hintKey: 'aura_drift.weapon.' + id + '.hint',
            weights: { front: front, mid: mid, back: back }, item: item
        };
        /* Compatibility shim until T5.1 translates the HUD: the 2D HUD and shell
           still read `def.name`. Not enumerable, so tables and hashes ignore it. */
        Object.defineProperty(def, 'name', { value: id, enumerable: false });
        return def;
    }
    const list = [
        weapon('oil', 1, 0.9, 3.2, false, '#1b1b22', 18, 6, 0, null),
        weapon('gnome_mine', 1, 1.1, 3.2, false, '#f97316', 18, 8, 0, 'item-gnome_mine'),
        weapon('emp', 1, 1.4, 30, false, '#38bdf8', 2, 6, 8, null),
        weapon('rocket', 1, 1.6, 90, false, '#fb7185', 4, 12, 10, 'item-rocket'),
        weapon('mg', 6, 0.16, 68, true, '#facc15', 8, 10, 6, 'item-bratwurst'),
        weapon('shield', 1, 1.2, 0, false, '#5eead4', 16, 6, 4, null),
        weapon('ducks', 1, 1.4, 80, false, '#fde047', 6, 10, 8, 'item-rubber_duck'),
        weapon('railgun', 1, 2.0, 220, false, '#a5f3fc', 0, 3, 6, null),
        weapon('flamethrower', 1, 0.5, 18, true, '#fb923c', 2, 8, 6, null),
        weapon('frost', 1, 1.2, 30, false, '#bae6fd', 8, 6, 2, 'item-frost_grenade'),
        weapon('air_horn', 1, 1.0, 18, false, '#fbbf24', 10, 6, 2, 'item-air_horn'),
        weapon('black_hole', 1, 1.8, 45, false, '#7c3aed', 0, 2, 7, null),
        weapon('disco', 1, 1.4, 20, false, '#f472b6', 8, 6, 4, 'item-disco_ball'),
        weapon('glove', 1, 1.0, 7, false, '#ef4444', 0, 5, 6, 'item-boxing_glove'),
        weapon('harpoon', 1, 1.6, 60, false, '#94a3b8', 0, 2, 8, 'item-harpoon'),
        weapon('cow', 1, 2.0, 180, false, '#f5f5f4', 0, 2, 7, null),
        weapon('cloak', 1, 1.2, 0, false, '#99f6e4', 0, 2, 6, null),
        weapon('overdrive', 1, 1.2, 0, false, '#f97316', 0, 0, 10, null)
    ];
    AD.WEAPONS = {};
    for (const def of list) AD.WEAPONS[def.id] = def;
    AD.WEAPON_IDS = list.map(def => def.id);

    /* rankFraction = (rank - 1) / (field - 1): front under 0.34, back from 0.67. */
    AD.rollWeapon = function (rng, rankFraction, pool) {
        const band = rankFraction < 0.34 ? 'front' : (rankFraction >= 0.67 ? 'back' : 'mid');
        const ids = (pool && pool.length ? pool : AD.WEAPON_IDS).filter(id => AD.WEAPONS[id] && AD.WEAPONS[id].weights[band] > 0);
        let total = 0;
        for (const id of ids) total += AD.WEAPONS[id].weights[band];
        if (!ids.length || total <= 0) return (pool && pool[0]) || AD.WEAPON_IDS[0];
        let roll = rng() * total;
        for (const id of ids) {
            roll -= AD.WEAPONS[id].weights[band];
            if (roll < 0) return id;
        }
        return ids[ids.length - 1];
    };

    AD.giveWeapon = function (race, car, id) {
        if (car.ghost || !race.weapons || !AD.WEAPONS[id]) return;
        car.weapon = id;
        car.ammo = AD.WEAPONS[id].ammo;
        car.cooldown = 0;
        car.fuel = 0;
        car.flame = 0;
        car.weaponLock = 1.1;               /* roulette: firing blocked */
    };

    /* ---- Geometry helpers shared with aura-drift-projectiles.js (internal). ---- */
    const kit = AD.weaponKit = {};

    kit.worldVel = function (v) {
        if (v.airborne) return { x: v.wx, y: v.wy };
        const cos = Math.cos(v.psi), sin = Math.sin(v.psi);
        return { x: v.vx * cos - v.vy * sin, y: v.vx * sin + v.vy * cos };
    };
    /* Position of vehicle b in the frame of vehicle a: fwd/lat in metres, ang from the nose. */
    kit.rel = function (a, b) {
        const dx = b.x - a.x, dy = b.y - a.y;
        const cos = Math.cos(a.psi), sin = Math.sin(a.psi);
        const fwd = dx * cos + dy * sin, lat = -dx * sin + dy * cos;
        return { fwd: fwd, lat: lat, dist: Math.hypot(dx, dy), ang: Math.atan2(Math.abs(lat), fwd) };
    };
    /* Cars a weapon may aim at: not self, not out, not cloaked. The ghost is not in race.cars. */
    kit.rivals = function (race, car) {
        const out = [];
        for (const other of race.cars) {
            if (other === car || other.v.out || other.v.cloak > 0) continue;
            out.push(other);
        }
        return out;
    };
    kit.nearest = function (race, car, maxDist, maxAng, needAhead) {
        let best = null;
        for (const other of kit.rivals(race, car)) {
            const r = kit.rel(car.v, other.v);
            if (r.dist > maxDist || r.ang > maxAng || (needAhead && r.fwd <= 0)) continue;
            if (!best || r.dist < best.rel.dist) best = { car: other, rel: r };
        }
        return best;
    };
    /* A point `ahead` metres further along the road, at the actor's lateral offset (kept on the tarmac). */
    kit.alongTrack = function (race, v, ahead, offset) {
        const s = AD.sampleTrack(race.track, v.x, v.y);
        const p = AD.pointAt(race.track, s.s + ahead);
        const lim = Math.max(1, s.halfWidth - 2);
        const off = clamp(offset === undefined ? s.offset : offset, -lim, lim);
        return { x: p.x + p.nx * off, y: p.y + p.ny * off };
    };
    /* Where a vehicle will be in t seconds if it keeps following the road. */
    kit.predict = function (race, v, t) {
        const speed = Math.max(0, v.vx);
        if (speed < 2) return { x: v.x, y: v.y };
        return kit.alongTrack(race, v, speed * t);
    };
    kit.snap = function (race, x, y) {
        const s = AD.sampleTrack(race.track, x, y);
        return kit.alongTrack(race, { x: x, y: y }, 0, s.offset);
    };

    /* ---- Firing. Each entry spawns state; the telegraph event comes before the effect. ---- */
    function nose(v, d) { return { x: v.x + Math.cos(v.psi) * d, y: v.y + Math.sin(v.psi) * d }; }
    function behind(v, d) { return nose(v, -d); }
    function projectile(type, x, y, psi, speed, ttl, owner, extra) {
        return Object.assign({ type: type, x: x, y: y, z: 0, psi: psi, speed: speed, ttl: ttl, owner: owner }, extra || {});
    }
    /* Arc to a world point: used by frost, black hole, disco and cow. */
    function lob(race, car, type, tx, ty, flight, peak, extra) {
        const v = car.v, n = nose(v, 2.6);
        const dist = Math.hypot(tx - n.x, ty - n.y);
        const p = projectile(type, n.x, n.y, Math.atan2(ty - n.y, tx - n.x), dist / flight, flight + 1, car.index, Object.assign({
            sx: n.x, sy: n.y, tx: tx, ty: ty, t: 0, flight: flight, peak: peak, vz: 4 * peak / flight
        }, extra || {}));
        p.z = 0.6;
        race.projectiles.push(p);
        return p;
    }

    const FIRE = {
        oil(race, car, v) {
            const b = behind(v, 3.2);
            race.hazards.push({ type: 'oil', x: b.x, y: b.y, r: 7, ttl: 6, age: 0, owner: car.index });
        },
        gnome_mine(race, car, v) {
            const b = behind(v, 3.2);
            race.hazards.push({ type: 'gnome_mine', x: b.x, y: b.y, r: 3.4, ttl: 12, arm: 0.45, age: 0, owner: car.index });
        },
        emp(race, car, v) {
            AD.emit(race, 'emp_charge', { car: car.index, x: v.x, y: v.y });
            race.projectiles.push(projectile('emp_charge', v.x, v.y, v.psi, 0, 0.25, car.index));
        },
        rocket(race, car, v) {
            const pick = kit.nearest(race, car, 120, 1.05, true) || kit.nearest(race, car, 40, Math.PI, false);
            const n = nose(v, 3);
            race.projectiles.push(projectile('rocket', n.x, n.y, v.psi, 62, 6, car.index, { target: pick ? pick.car.index : null }));
            if (pick) AD.emit(race, 'rocket_lock', { car: car.index, target: pick.car.index });
        },
        mg(race, car, v) {
            const n = nose(v, 3);
            for (let i = -2; i <= 2; i++) {
                race.projectiles.push(projectile('bullet', n.x, n.y, v.psi + i * 0.055, 105 * (0.97 + race.rng() * 0.06), 0.65, car.index));
            }
        },
        shield(race, car, v) {
            v.shield = 4;
            v.damage = Math.max(0, v.damage - 22);
            AD.emit(race, 'shield', { x: v.x, y: v.y, car: car.index });
        },
        ducks(race, car, v) {
            const pick = kit.nearest(race, car, 80, Math.PI / 2, true);
            const cos = Math.cos(v.psi), sin = Math.sin(v.psi);
            AD.emit(race, 'duck_launch', { car: car.index, count: 5 });
            for (let i = -2; i <= 2; i++) {
                const side = i * 0.9;
                race.projectiles.push(projectile('duck', v.x + cos * 3 - sin * side, v.y + sin * 3 + cos * side,
                    v.psi + i * 0.2, 34, 7, car.index, { target: pick ? pick.car.index : null, wob: race.rng() * 6.28, look: 0 }));
            }
        },
        railgun(race, car, v) {
            const n = nose(v, 2.4);
            AD.emit(race, 'rail_charge', { car: car.index, x: n.x, y: n.y, psi: v.psi });
            race.beams.push({ type: 'rail_aim', x: n.x, y: n.y, x2: n.x + Math.cos(v.psi) * 220, y2: n.y + Math.sin(v.psi) * 220, ttl: 0.35, car: car.index });
            race.projectiles.push(projectile('rail_charge', n.x, n.y, v.psi, 0, 0.35, car.index));
        },
        flamethrower(race, car) {
            car.fuel = 2.5;       /* the flame itself runs in tickFlame while fire is held */
            car.flame = 0;
            car.flameT = 0;
        },
        frost(race, car, v) {
            const pick = kit.nearest(race, car, 45, 1.2, true);
            const flight = 0.7;
            const t = pick ? kit.predict(race, pick.car.v, flight) : kit.alongTrack(race, v, 30 + Math.max(0, v.vx) * flight);
            AD.emit(race, 'frost_throw', { car: car.index, x: v.x, y: v.y, tx: t.x, ty: t.y });
            lob(race, car, 'grenade', t.x, t.y, flight, 3);
        },
        air_horn(race, car, v) {
            const R = 18, victims = [];
            const cleared = AD.clearWeaponsIn(race, v.x, v.y, R);
            for (const other of kit.rivals(race, car)) {
                const dx = other.v.x - v.x, dy = other.v.y - v.y, d = Math.hypot(dx, dy);
                if (d >= R) continue;
                const ux = d > 0.01 ? dx / d : Math.cos(v.psi), uy = d > 0.01 ? dy / d : Math.sin(v.psi);
                if (AD.applyHit(race, other, { by: car.index, quiet: true, shock: true, weapon: 'air_horn', push: { x: ux, y: uy, m: 14 } })) victims.push(other.index);
            }
            AD.emit(race, 'horn', { car: car.index, x: v.x, y: v.y, r: R, cleared: cleared, victims: victims });
        },
        black_hole(race, car, v) {
            const flight = 1.1;
            const ahead = [];
            for (const other of kit.rivals(race, car)) {
                const r = kit.rel(v, other.v);
                if (r.fwd > 0 && r.dist <= 80) ahead.push(kit.predict(race, other.v, flight));
            }
            let t;
            if (ahead.length >= 2) {
                let sx = 0, sy = 0;
                for (const p of ahead) { sx += p.x; sy += p.y; }
                t = kit.snap(race, sx / ahead.length, sy / ahead.length);
            } else {
                t = kit.alongTrack(race, v, 45 + Math.max(0, v.vx) * flight);
            }
            lob(race, car, 'hole_shot', t.x, t.y, flight, 5);
        },
        disco(race, car, v) {
            const flight = 0.5;
            const t = kit.alongTrack(race, v, 20 + Math.max(0, v.vx) * flight);
            if (car.tally) car.tally.discoFired++;
            lob(race, car, 'disco_shot', t.x, t.y, flight, 2.5);
        },
        glove(race, car, v) {
            const pick = kit.nearest(race, car, 7, 0.7, true);
            race.projectiles.push(projectile('glove', v.x, v.y, v.psi, 0, 0.12, car.index, { target: pick ? pick.car.index : null }));
        },
        harpoon(race, car, v) {
            const pick = kit.nearest(race, car, 60, 0.5236, true);
            const n = nose(v, 2.6);
            AD.emit(race, 'harpoon_fire', { car: car.index, target: pick ? pick.car.index : -1 });
            race.projectiles.push(projectile('harpoon', n.x, n.y, v.psi, 120, 0.6, car.index, { target: pick ? pick.car.index : null }));
        },
        cow(race, car, v) {
            const pick = kit.nearest(race, car, 180, Math.PI / 2, true);
            const flight = 1.4;
            const t = pick ? kit.predict(race, pick.car.v, flight) : kit.alongTrack(race, v, 70 + Math.max(0, v.vx) * flight);
            const shadow = { type: 'cow_shadow', x: t.x, y: t.y, r: 9, ttl: flight + 0.6, land: flight, tx: t.x, ty: t.y, age: 0, owner: car.index };
            race.hazards.push(shadow);
            AD.emit(race, 'cow_launch', { car: car.index, target: pick ? pick.car.index : -1, tx: t.x, ty: t.y, flight: flight });
            lob(race, car, 'cow', t.x, t.y, flight, 9, { shadow: shadow });
        },
        cloak(race, car, v) {
            v.cloak = 3;
            AD.emit(race, 'cloak_on', { car: car.index });
        },
        overdrive(race, car, v) {
            v.overdrive = 3;
            AD.emit(race, 'overdrive_on', { car: car.index });
        }
    };

    function endWeapon(race, car) {
        const id = car.weapon;
        car.weapon = null;
        car.ammo = 0;
        car.fuel = 0;
        if (id) AD.emit(race, 'weapon_end', { id: id, car: car.index });
    }

    AD.fireWeapon = function (race, car) {
        if (car.ghost || !race.weapons || !car.weapon || car.cooldown > 0 || car.ammo <= 0 || car.weaponLock > 0) return;
        if (car.v.cloak > 0 || car.v.out) return;
        const def = AD.WEAPONS[car.weapon];
        const fire = FIRE[def.id];
        if (!fire) return;
        if (def.id === 'flamethrower' && car.fuel > 0) return;   /* already lit */
        const v = car.v;
        car.cooldown = def.id === 'flamethrower' ? 0 : def.cooldown;
        if (car.tally) car.tally.weaponsFired++;
        AD.emit(race, 'weapon', { id: def.id, x: v.x, y: v.y, car: car.index });
        fire(race, car, v);
        if (def.id === 'flamethrower') return;   /* ammo is spent when the fuel runs out */
        car.ammo--;
        if (car.ammo <= 0) endWeapon(race, car);
    };

    /* ---- Flamethrower: lit by fire, burns while fire is held, pilot flame first. ---- */
    function stopFlame(race, car) {
        if (car.flame > 0) AD.emit(race, 'flame_end', { car: car.index });
        car.flame = 0;
        car.flameT = 0;
    }

    function burn(race, car, dt) {
        const v = car.v, cos = Math.cos(v.psi), sin = Math.sin(v.psi);
        const RANGE = 18, HALF = 0.2443;     /* 14 degrees */
        const flameBeam = race.beams.find(b => b.type === 'flame' && b.car === car.index);
        if (flameBeam) flameBeam.ttl = 0.15;
        else race.beams.push({ type: 'flame', car: car.index, ttl: 0.15 });
        for (const other of kit.rivals(race, car)) {
            const r = kit.rel(v, other.v);
            const radius = other.v.car.radius * 0.7;
            if (r.fwd <= 0 || r.dist > RANGE + radius || r.ang > HALF + Math.atan(radius / Math.max(2, r.dist))) continue;
            if (other.v.shield > 0) {
                if (race.time - (other.flameBlockAt === undefined ? -9 : other.flameBlockAt) > 0.5) {
                    other.flameBlockAt = race.time;
                    AD.emit(race, 'shield_block', { x: other.v.x, y: other.v.y, car: other.index, by: car.index });
                }
                continue;
            }
            other.v.damage = clamp(other.v.damage + 14 * dt, 0, 100);
            other.v.heat = Math.min(1.5, other.v.heat + 1.2 * dt);
            if (race.time - (other.flameHitAt === undefined ? -9 : other.flameHitAt) > 0.3) {
                other.flameHitAt = race.time;
                if (other.tally) other.tally.hitsTaken++;
                if (car.tally) car.tally.hitsDealt++;
                AD.emit(race, 'flame_hit', { car: other.index, by: car.index, x: other.v.x, y: other.v.y });
            }
        }
        /* Fire melts frost: an ice patch on the flame axis disappears. */
        for (let i = race.hazards.length - 1; i >= 0; i--) {
            const h = race.hazards[i];
            if (h.type !== 'ice') continue;
            const dx = h.x - v.x, dy = h.y - v.y;
            const fwd = dx * cos + dy * sin, lat = Math.abs(-dx * sin + dy * cos);
            if (fwd > -h.r * 0.5 && fwd < RANGE + h.r * 0.5 && lat < h.r * 0.8) race.hazards.splice(i, 1);
        }
    }

    function tickFlame(race, car, fire, dt) {
        const v = car.v;
        if (v.cloak > 0 || v.out || !fire) { stopFlame(race, car); return; }
        if (car.flame === 0) {
            car.flame = 1;
            car.flameT = 0;
            AD.emit(race, 'flame_start', { car: car.index });
        }
        if (car.flame === 1) {
            car.flameT += dt;
            if (car.flameT < 0.2) return;      /* pilot flame: the telegraph */
            car.flame = 2;
        }
        car.fuel = Math.max(0, car.fuel - dt);
        burn(race, car, dt);
        if (car.fuel <= 0) { stopFlame(race, car); endWeapon(race, car); }
    }

    /* Fire latch, cooldown and roulette lock. `hold` weapons repeat while fire is held. */
    AD.updateWeaponInput = function (race, car, input, dt) {
        if (car.ghost) return;
        if (car.cooldown > 0) car.cooldown = Math.max(0, car.cooldown - dt);
        if (car.weaponLock > 0) car.weaponLock = Math.max(0, car.weaponLock - dt);
        const fire = !!input.fire;
        const edge = fire && !car.fireLatch;
        car.fireLatch = fire;
        if (!race.weapons || car.v.out) { if (car.flame) stopFlame(race, car); return; }
        if (car.weapon && car.v.cloak <= 0) {
            const def = AD.WEAPONS[car.weapon];
            if (fire && (edge || def.hold) && car.cooldown <= 0 && car.weaponLock <= 0) AD.fireWeapon(race, car);
        }
        if (car.weapon === 'flamethrower' && car.fuel > 0) tickFlame(race, car, fire, dt);
        else if (car.flame) stopFlame(race, car);
    };

    /* ---- AI predicates (section 2.9 "AI fires when"). Deterministic: no randomness. ---- */
    function count(race, car, test) {
        let n = 0;
        for (const other of kit.rivals(race, car)) if (test(kit.rel(car.v, other.v), other)) n++;
        return n;
    }
    function any(race, car, test) { return count(race, car, test) > 0; }
    const DEG = Math.PI / 180;

    /* Something is about to hit this car and a shield or cloak would help. */
    function threatened(race, car) {
        const v = car.v;
        for (const p of race.projectiles) {
            if (p.owner === car.index) continue;
            if ((p.type === 'rocket' || p.type === 'duck' || p.type === 'harpoon') && p.target === car.index &&
                AD.distance(p.x, p.y, v.x, v.y) < 70) return true;
            if (p.type === 'rail_charge') {
                const dx = v.x - p.x, dy = v.y - p.y;
                const fwd = dx * Math.cos(p.psi) + dy * Math.sin(p.psi);
                const lat = Math.abs(-dx * Math.sin(p.psi) + dy * Math.cos(p.psi));
                if (fwd > 0 && fwd < 230 && lat < 3.5) return true;
            }
        }
        for (const h of race.hazards) {
            if (h.type === 'cow_shadow' && h.owner !== car.index && AD.distance(h.x, h.y, v.x, v.y) < 16) return true;
        }
        return false;
    }
    function straightAhead(race, car, metres) {
        const track = race.track, n = track.samples.length;
        const first = AD.sampleTrack(track, car.v.x, car.v.y).index;
        const steps = Math.ceil(metres / (track.length / n) / 2);
        for (let k = 0; k <= steps; k++) if (track.curvature[(first + k * 2) % n] >= 0.01) return false;
        return true;
    }
    function rankFraction(race, car) {
        const field = race.cars.length;
        const rank = Math.max(1, race.standings.indexOf(car) + 1);
        return field > 1 ? (rank - 1) / (field - 1) : 0;
    }
    function hostileNear(race, car, radius) {
        for (const p of race.projectiles) {
            if (p.owner === car.index || p.type === 'emp_charge' || p.type === 'rail_charge' || p.type === 'glove') continue;
            if (AD.distance(p.x, p.y, car.v.x, car.v.y) < radius) return true;
        }
        return false;
    }

    const PRED = {
        oil: (race, car) => any(race, car, r => r.fwd < 0 && r.dist <= 40 && Math.abs(r.lat) < 9),
        gnome_mine: (race, car) => any(race, car, r => r.fwd < 0 && r.dist <= 35 && Math.abs(r.lat) < 8),
        emp: (race, car) => count(race, car, r => r.dist <= 26) >= 2 || any(race, car, r => r.dist <= 14),
        rocket: (race, car) => any(race, car, r => r.fwd > 0 && r.dist <= 90 && r.ang <= 60 * DEG),
        mg: (race, car) => any(race, car, r => r.fwd > 4 && r.dist <= 50 && r.ang <= 12 * DEG && Math.abs(r.lat) < 4.2),
        shield: (race, car) => car.v.shield <= 0 && (threatened(race, car) || car.v.damage > 50),
        ducks: (race, car) => any(race, car, r => r.fwd > 0 && r.dist <= 80 && Math.abs(r.lat) < 30),
        railgun: (race, car) => any(race, car, r => r.fwd > 0 && r.dist <= 150 && r.ang <= 5 * DEG),
        flamethrower: (race, car) => any(race, car, r => r.fwd > 0 && r.dist <= 18 && r.ang <= 14 * DEG),
        frost: (race, car) => any(race, car, r => r.fwd > 0 && r.dist <= 45 && Math.abs(r.lat) < 20),
        air_horn: (race, car) => hostileNear(race, car, 12) || any(race, car, r => r.dist <= 12),
        black_hole: (race, car) => count(race, car, r => r.fwd > 0 && r.dist <= 80) >= 2,
        disco: (race, car) => any(race, car, r => r.dist <= 50 && r.fwd > -5),
        glove: (race, car) => any(race, car, r => r.dist <= 7 && r.ang <= 40 * DEG),
        harpoon: (race, car) => any(race, car, r => r.fwd > 0 && r.dist <= 60 && r.ang <= 30 * DEG),
        cow: (race, car) => any(race, car, r => r.fwd > 0 && r.dist <= 180),
        cloak: (race, car) => threatened(race, car) || (race.laps < 99
            ? car.progress >= race.laps * race.track.length * 0.67
            : rankFraction(race, car) >= 0.67) && any(race, car, r => r.dist <= 60),
        overdrive: (race, car) => car.v.vx > 25 && !car.v.airborne && straightAhead(race, car, 120)
    };
    const INSTANT = { shield: 0.12, cloak: 0.12, air_horn: 0.1, glove: 0.08, overdrive: 0.2, mg: 0.05, flamethrower: 0 };

    AD.aiWantsFire = function (race, car) {
        if (car.ghost || car.index < 0 || !race.weapons || !car.weapon) return false;
        if (car.weaponLock > 0 || car.v.cloak > 0 || car.v.out) return false;
        const test = PRED[car.weapon];
        const want = !!test && test(race, car);
        if (!want) { car.ai.fireHold = 0; return false; }
        if (car.weapon === 'flamethrower' && car.fuel > 0) return true;
        /* Reaction time grows with the driver's lack of skill; defensive pickups react fast. */
        const need = INSTANT[car.weapon] !== undefined ? INSTANT[car.weapon] : 0.2 + (1 - clamp(car.ai.skill || 1, 0, 1)) * 2.5;
        car.ai.fireHold = (car.ai.fireHold || 0) + AD.STEP;
        if (car.ai.fireHold < need) return false;
        car.ai.fireHold = 0;
        return true;
    };
})();
