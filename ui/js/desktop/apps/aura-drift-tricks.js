/* Aura Drift - air state, tricks, landing, drift turbo, perfect start, slingshot, reverse (section 2.10).
   Pure simulation: no DOM, no clock, no Math.random. The ghost runs through the same code as the
   player (its physics effects must match) but never scores: it has no style and no tally. */
(function () {
    'use strict';
    const AD = window.AuraDriftCore;
    const clamp = AD.clamp;
    const TAU = Math.PI * 2;

    const K = {
        GRAVITY: 16, ROLL_RATE: 7.0, PITCH_RATE: 6.0, SPIN_RATE: 9.0, SMOOTH: 12, LEVEL_RATE: 3.5,
        DRIFT_GRACE: 0.18, DRIFT_LEVELS: [1.0, 2.2, 3.6]
    };
    AD.TRICK_CONST = K;

    function def(id, points, nitro, extra) { return Object.assign({ id: id, points: points, nitro: nitro }, extra || {}); }
    AD.TRICKS = {
        air_time: def('air_time', 50, 0.04, { perSecond: 100, minAir: 0.6 }),
        barrel_roll: def('barrel_roll', 400, 0.12, { accum: TAU - 0.35 }),
        double_barrel: def('double_barrel', 900, 0.25, { accum: 2 * TAU - 0.35, replaces: 'barrel_roll' }),
        front_flip: def('front_flip', 500, 0.15, { accum: TAU - 0.35 }),
        back_flip: def('back_flip', 500, 0.15, { accum: TAU - 0.35 }),
        flat_spin: def('flat_spin', 350, 0.10, { accum: TAU - 0.5 }),
        spin_720: def('spin_720', 900, 0.22, { accum: 2 * TAU - 0.5, replaces: 'flat_spin' }),
        corkscrew: def('corkscrew', 1200, 0.30),
        cow_vault: def('cow_vault', 600, 0.10, { minZ: 2.2 }),
        perfect_start: def('perfect_start', 300, 0),
        drift_blue: def('drift_blue', 100, 0, { charge: 1.0, burst: 0.5, power: 1.25 }),
        drift_orange: def('drift_orange', 250, 0, { charge: 2.2, burst: 0.9, power: 1.35 }),
        drift_pink: def('drift_pink', 500, 0, { charge: 3.6, burst: 1.4, power: 1.5 }),
        slingshot: def('slingshot', 150, 0, { burst: 0.8, power: 1.25 }),
        reverse_hero: def('reverse_hero', 200, 0, { perSecond: 100 }),
        j_turn: def('j_turn', 400, 0.1)
    };
    /* The physical constants are reachable from the table too, but stay out of id iteration. */
    for (const key of Object.keys(K)) Object.defineProperty(AD.TRICKS, key, { value: K[key], enumerable: false });
    const DRIFT_IDS = ['drift_blue', 'drift_orange', 'drift_pink'];

    AD.createTrickState = function () {
        return {
            chain: 0, chainPoints: 0, counts: {}, best: 0,
            /* flight */
            flight: null, plan: null, latchThrottle: false, latchBrake: false,
            /* ground */
            driftGrace: 0, wallSeen: 0, hold: 0,
            rev: 0, revPaid: 0, revHero: false, revSeen: false, sinceRev: 9, draftHeld: 0
        };
    };

    function newFlight() {
        return { paid: {}, done: {}, nitro: 0, rollDone: false, flipDone: false, cork: false, cows: [] };
    }

    function quirkTrickK(actor) {
        const c = actor.v.car;
        return c.quirk === 'trick_points' ? c.quirkK : 1;
    }

    /* Forced drive boost; a stronger boost never gets shortened by a weaker one. */
    function giveBurst(v, seconds, power) {
        if (v.burst > 0) {
            v.burst = Math.max(v.burst, seconds);
            v.burstPower = Math.max(v.burstPower, power);
        } else { v.burst = seconds; v.burstPower = power; }
    }

    function count(actor, id, delta) {
        const counts = actor.tricks.counts;
        counts[id] = Math.max(0, (counts[id] || 0) + delta);
        if (!counts[id]) delete counts[id];
    }

    /* A trick that is not part of a flight chain pays at once. */
    function payNow(actor, id, points) {
        const v = actor.v;
        count(actor, id, 1);
        const total = points * quirkTrickK(actor);
        AD.addStylePoints(actor, total, true);
        if (actor.tally) actor.tally.tricks++;
        if (actor.tricks.best < total) actor.tricks.best = total;
        v.nitro = Math.min(1, v.nitro + (AD.TRICKS[id].nitro || 0));
        return total;
    }

    /* A flight trick joins the chain; it is banked on landing. */
    function chainTrick(race, actor, id, points) {
        const st = actor.tricks, fl = st.flight, base = id.split('#')[0], def = AD.TRICKS[base];
        if (fl.done[id]) return;
        fl.done[id] = true;
        if (def.replaces && fl.paid[def.replaces] !== undefined) {
            delete fl.paid[def.replaces];
            fl.nitro -= AD.TRICKS[def.replaces].nitro;
            count(actor, def.replaces, -1);
            if (actor.tally) actor.tally.tricks = Math.max(0, actor.tally.tricks - 1);
        }
        fl.paid[id] = points;
        fl.nitro += def.nitro;
        count(actor, base, 1);
        if (actor.tally) actor.tally.tricks++;
        st.chain = Object.keys(fl.paid).length;
        st.chainPoints = 0;
        for (const key of Object.keys(fl.paid)) st.chainPoints += fl.paid[key];
        AD.emit(race, 'trick', { car: actor.index, id: base, points: points, chain: st.chain, x: actor.v.x, y: actor.v.y });
    }

    /* ---- launch and flight ------------------------------------------------- */

    AD.launch = function (race, actor, vz, source) {
        const v = actor.v, st = actor.tricks;
        if (v.airborne || v.out || !(vz > 0)) return false;
        const cos = Math.cos(v.psi), sin = Math.sin(v.psi);
        v.wx = v.vx * cos - v.vy * sin;
        v.wy = v.vx * sin + v.vy * cos;
        v.vz = vz;
        v.airborne = true;
        v.airTime = 0;
        v.rollRate = 0; v.pitchRate = 0; v.spinRate = 0;
        v.rollAccum = 0; v.pitchAccum = 0; v.spinAccum = 0;
        v.onRamp = -1;
        v.driftCharge = 0; v.driftLevel = 0;
        st.driftGrace = 0;
        st.flight = newFlight();
        st.chain = 0; st.chainPoints = 0;
        /* A pedal that is already down at takeoff is not a flip command until it was released. */
        st.latchThrottle = v.throttle > 0.3;
        st.latchBrake = v.brake > 0.3;
        st.plan = null;
        if (!actor.isPlayer && !actor.ghost) {
            const fs = race.features;
            const rng = fs && typeof fs.rng === 'function' ? fs.rng : race.rng;
            const air = 2 * vz / K.GRAVITY;
            const u = rng(), dir = rng() < 0.5 ? -1 : 1;
            let kind = 'none';
            if (source !== 'hit') {
                if (u >= 0.4 && u < 0.8 && air >= 0.95 && air * K.ROLL_RATE * v.car.airControl >= 6.5) kind = 'roll';
                else if (u >= 0.8 && air >= 1.0 && air * K.SPIN_RATE * v.car.airControl >= 8) kind = 'spin';
            }
            st.plan = { kind: kind, dir: dir };
        }
        AD.emit(race, 'takeoff', {
            car: actor.index, x: v.x, y: v.y, vz: vz, speed: Math.hypot(v.wx, v.wy), source: source || 'ramp'
        });
        return true;
    };

    AD.landingQuality = function (v) {
        const roll = Math.abs(AD.wrapAngle(v.roll)), pitch = Math.abs(AD.wrapAngle(v.pitch));
        let yaw = 0;
        if (Math.hypot(v.wx, v.wy) > 4) yaw = Math.abs(AD.wrapAngle(v.psi - Math.atan2(v.wy, v.wx)));
        const worst = Math.max(roll, pitch);
        if (worst <= 0.30 && yaw <= 0.7) return 'perfect';
        if (worst <= 0.75 && yaw <= 1.4) return 'good';
        if (worst <= 1.4 && yaw <= 2.2) return 'wobble';
        return 'crash';
    };

    function axisRate(angle, command, maxRate, level) {
        if (Math.abs(command) > 0.1) return command * maxRate;
        if (!level) return 0;
        const diff = Math.round(angle / TAU) * TAU - angle;
        return clamp(diff * 8, -K.LEVEL_RATE, K.LEVEL_RATE);
    }

    function detectTricks(race, actor, st, v) {
        const fl = st.flight;
        const roll = Math.abs(v.rollAccum), pitch = v.pitchAccum, spin = Math.abs(v.spinAccum);
        const T = AD.TRICKS;
        if (roll >= T.barrel_roll.accum) { fl.rollDone = true; chainTrick(race, actor, 'barrel_roll', T.barrel_roll.points); }
        if (roll >= T.double_barrel.accum) chainTrick(race, actor, 'double_barrel', T.double_barrel.points);
        if (pitch <= -T.front_flip.accum) { fl.flipDone = true; chainTrick(race, actor, 'front_flip', T.front_flip.points); }
        if (pitch >= T.back_flip.accum) { fl.flipDone = true; chainTrick(race, actor, 'back_flip', T.back_flip.points); }
        if (spin >= T.flat_spin.accum) chainTrick(race, actor, 'flat_spin', T.flat_spin.points);
        if (spin >= T.spin_720.accum) chainTrick(race, actor, 'spin_720', T.spin_720.points);
        if (fl.rollDone && fl.flipDone && !fl.cork) { fl.cork = true; chainTrick(race, actor, 'corkscrew', T.corkscrew.points); }
        if (v.z >= T.cow_vault.minZ && race.features && race.features.animals) {
            for (const animal of race.features.animals) {
                if (animal.state === 'flying' || fl.cows.indexOf(animal.id) >= 0) continue;
                if (Math.hypot(animal.x - v.x, animal.y - v.y) > 3.4) continue;
                fl.cows.push(animal.id);
                chainTrick(race, actor, 'cow_vault#' + animal.id, T.cow_vault.points);
            }
        }
    }

    function land(race, actor) {
        const v = actor.v, st = actor.tricks, fl = st.flight || newFlight();
        const quality = AD.landingQuality(v);
        const airTime = v.airTime;
        const hop = airTime < 0.35 && quality !== 'crash';
        /* Landing conversion: world velocity back to the body frame. */
        const cos = Math.cos(v.psi), sin = Math.sin(v.psi);
        v.vx = v.wx * cos + v.wy * sin;
        v.vy = -v.wx * sin + v.wy * cos;
        v.airborne = false;
        v.z = 0;
        const impactSpeed = Math.abs(v.vz);
        v.vz = 0;
        v.roll = AD.wrapAngle(v.roll);
        v.pitch = AD.wrapAngle(v.pitch);
        v.rollRate = 0; v.pitchRate = 0; v.spinRate = 0;
        v.wx = 0; v.wy = 0;
        st.plan = null; st.flight = null;
        if (hop) {
            st.chain = 0; st.chainPoints = 0;
            return;
        }
        if (airTime >= AD.TRICKS.air_time.minAir) {
            const pts = AD.TRICKS.air_time.points + AD.TRICKS.air_time.perSecond * airTime;
            st.flight = fl;
            chainTrick(race, actor, 'air_time', pts);
            st.flight = null;
        }
        let chainPts = 0;
        for (const key of Object.keys(fl.paid)) chainPts += fl.paid[key];
        let mult = 1, nitro = 0;
        if (quality === 'perfect') { mult = 1.25; nitro = 0.2 + fl.nitro; giveBurst(v, 0.5, 1.25); }
        else if (quality === 'good') { nitro = 0.05 + fl.nitro; }
        else if (quality === 'wobble') { mult = 0.5; nitro = fl.nitro * 0.5; v.landWobble = Math.max(v.landWobble, 0.9); }
        else {
            mult = 0; v.vx *= 0.7; v.vy *= 0.7; v.r *= 0.5;
            v.landWobble = Math.max(v.landWobble, 1.4);
            v.damage = clamp(v.damage + 6, 0, 100);
        }
        v.impact = Math.max(v.impact, clamp(impactSpeed / 18, 0.15, 0.9));
        let points = quality === 'crash' ? 50 : chainPts * mult;
        points *= quirkTrickK(actor);
        AD.addStylePoints(actor, points, true);
        v.nitro = Math.min(1, v.nitro + nitro);
        if (st.best < points) st.best = points;
        if (actor.tally) {
            actor.tally.airTime += airTime;
            if (quality === 'crash') actor.tally.crashLandings++;
            if (quality === 'perfect') actor.tally.perfectLandings++;
        }
        AD.emit(race, 'landing', {
            car: actor.index, x: v.x, y: v.y, quality: quality, airTime: airTime, points: points, nitro: nitro
        });
        st.chain = 0; st.chainPoints = 0;
    }

    AD.stepAir = function (actor, input, dt, race) {
        const v = actor.v, st = actor.tricks, c = v.car, ac = c.airControl;
        if (!st.flight) st.flight = newFlight();      /* airborne without launch(): stay safe */
        v.airTime += dt;

        if (st.latchThrottle && input.throttle < 0.3) st.latchThrottle = false;
        if (st.latchBrake && input.brake < 0.3) st.latchBrake = false;
        const throttle = st.latchThrottle ? 0 : input.throttle;
        const brake = st.latchBrake ? 0 : input.brake;
        const steer = clamp(input.steer || 0, -1, 1);
        const spinning = !!input.handbrake;

        const rollTarget = axisRate(v.roll, spinning ? 0 : steer, K.ROLL_RATE * ac, true);
        const pitchCmd = clamp(brake - throttle, -1, 1);
        const pitchTarget = axisRate(v.pitch, pitchCmd, K.PITCH_RATE * ac, true);
        const spinTarget = spinning ? steer * K.SPIN_RATE * ac : 0;
        v.rollRate = AD.smooth(v.rollRate, rollTarget, K.SMOOTH, dt);
        v.pitchRate = AD.smooth(v.pitchRate, pitchTarget, K.SMOOTH, dt);
        v.spinRate = AD.smooth(v.spinRate, spinTarget, K.SMOOTH, dt);

        v.roll += v.rollRate * dt;      v.rollAccum += v.rollRate * dt;
        v.pitch += v.pitchRate * dt;    v.pitchAccum += v.pitchRate * dt;
        v.spinAccum += v.spinRate * dt;
        v.r *= Math.exp(-1.5 * dt);
        v.psi = AD.wrapAngle(v.psi + (v.r + v.spinRate) * dt);

        v.x += v.wx * dt;
        v.y += v.wy * dt;
        v.vz -= K.GRAVITY * dt;
        v.z += v.vz * dt;

        const cos = Math.cos(v.psi), sin = Math.sin(v.psi);
        v.vx = v.wx * cos + v.wy * sin;
        v.vy = -v.wx * sin + v.wy * cos;

        /* Presentation and timers that physics would normally tick. */
        v.steer = AD.smooth(v.steer, steer * 0.15, 6, dt);
        v.throttle = input.throttle; v.brake = input.brake; v.boost = 0;
        v.slip = AD.smooth(v.slip, 0, 4, dt);
        v.rpm = clamp(v.rpm + (input.throttle > 0.3 ? 4200 : -3200) * dt, 900, 7300);
        if (v.burst > 0) v.burst = Math.max(0, v.burst - dt);
        if (v.landWobble > 0) v.landWobble = Math.max(0, v.landWobble - dt);
        if (v.disabled > 0) v.disabled = Math.max(0, v.disabled - dt);
        if (v.shield > 0) v.shield = Math.max(0, v.shield - dt);
        v.impact = Math.max(0, v.impact - dt * 2.2);

        detectTricks(race, actor, st, v);
        if ((v.z <= 0 && v.vz <= 0) || v.airTime > 12) land(race, actor);
    };

    AD.aiAirInput = function (race, car, input) {
        const v = car.v, plan = car.tricks.plan;
        input.brake = 0; input.throttle = 0; input.boost = 0; input.handbrake = 0; input.steer = 0;
        if (!plan) return input;
        if (plan.kind === 'roll') {
            if (Math.abs(v.rollAccum) + Math.abs(v.rollRate) / 14 < TAU - 0.1) input.steer = plan.dir;
        } else if (plan.kind === 'spin') {
            if (Math.abs(v.spinAccum) + Math.abs(v.spinRate) / 12 < TAU - 0.1) {
                input.steer = plan.dir; input.handbrake = 1;
            }
        }
        return input;
    };

    /* ---- start, drift turbo, slingshot, reverse ---------------------------- */

    AD.updateStart = function (actor, input, dt) {
        const st = actor.tricks;
        if (!st) return;
        st.hold = input.throttle > 0.5 ? st.hold + dt : 0;
    };

    AD.applyStart = function (race, actor) {
        const v = actor.v, st = actor.tricks;
        let perfect = false, spin = false;
        if (actor.isPlayer || actor.ghost) {
            perfect = st.hold >= 0.05 && st.hold <= 0.30;
            spin = st.hold > 0.8;
        } else {
            /* Rivals do not press pedals in the countdown: a seeded roll stands in. */
            const u = AD.hashString(race.track.id + '/start/' + race.seed + '/' + actor.index) / 4294967296;
            const skill = actor.ai && actor.ai.skill ? actor.ai.skill : 1;
            perfect = u < 0.16 * skill;
            spin = u > 0.93;
        }
        st.hold = 0;
        if (perfect) {
            giveBurst(v, 1.2, 1.3);
            const pts = payNow(actor, 'perfect_start', AD.TRICKS.perfect_start.points);
            AD.emit(race, 'perfect_start', { car: actor.index, points: pts });
        } else if (spin) {
            /* Wheelspin: a short burst whose power factor is 0.6, so the launch is weaker. */
            v.burst = 0.6; v.burstPower = 0.6;
            AD.emit(race, 'wheelspin', { car: actor.index });
        }
    };

    function driftStep(race, actor, st, dt) {
        const v = actor.v, S = AD.STYLE;
        const speed = Math.max(0, v.vx);
        const angle = Math.abs(Math.atan2(v.vy, Math.max(1, Math.abs(v.vx))));
        const drifting = angle > S.driftAngleMin && speed > S.driftSpeedMin && v.slip > 1.1;
        if (v.wallContacts > st.wallSeen) {
            v.driftCharge = 0; v.driftLevel = 0; st.driftGrace = 0;
        }
        st.wallSeen = v.wallContacts;
        if (drifting) {
            v.driftCharge += dt;
            st.driftGrace = K.DRIFT_GRACE;
            let level = 0;
            for (let i = 0; i < 3; i++) if (v.driftCharge >= K.DRIFT_LEVELS[i]) level = i + 1;
            if (level > v.driftLevel) {
                v.driftLevel = level;
                AD.emit(race, 'drift_level', { car: actor.index, level: level });
            }
            return;
        }
        if (v.driftCharge <= 0) return;
        st.driftGrace -= dt;
        if (st.driftGrace > 0) return;
        const level = v.driftLevel;
        v.driftCharge = 0; v.driftLevel = 0;
        if (level < 1) return;
        const id = DRIFT_IDS[level - 1], d = AD.TRICKS[id];
        const k = v.car.quirk === 'turbo_time' ? v.car.quirkK : 1;
        giveBurst(v, d.burst * k, d.power);
        const pts = payNow(actor, id, d.points);
        AD.emit(race, 'turbo_burst', { car: actor.index, level: level, x: v.x, y: v.y });
        AD.emit(race, 'trick', { car: actor.index, id: id, points: pts, chain: 0, x: v.x, y: v.y });
    }

    function slingshotStep(race, actor, st, dt) {
        const v = actor.v;
        if (v.slipstream > 0.05) { v.draftTime += dt; return; }
        if (v.draftTime >= 1.2 && v.vx > 25) {
            const d = AD.TRICKS.slingshot;
            giveBurst(v, d.burst, d.power);
            const pts = payNow(actor, 'slingshot', d.points);
            AD.emit(race, 'slingshot', { car: actor.index, x: v.x, y: v.y, points: pts });
        }
        v.draftTime = 0;
    }

    function reverseStep(race, actor, st, dt) {
        const v = actor.v;
        if (v.vx <= -8) {
            st.rev += dt; st.sinceRev = 0; st.revSeen = true;
            if (actor.tally) actor.tally.reverseSeconds += dt;
            if (st.rev >= 2) {
                if (!st.revHero) {
                    st.revHero = true;
                    const pts = payNow(actor, 'reverse_hero', AD.TRICKS.reverse_hero.points);
                    AD.emit(race, 'reverse_hero', { car: actor.index, points: pts });
                } else {
                    AD.addStylePoints(actor, AD.TRICKS.reverse_hero.perSecond * dt * quirkTrickK(actor), true);
                }
            }
            return;
        }
        st.rev = 0; st.revHero = false;
        st.sinceRev += dt;
        if (st.revSeen && v.vx >= 8) {
            st.revSeen = false;
            if (st.sinceRev <= 1.5) {
                const pts = payNow(actor, 'j_turn', AD.TRICKS.j_turn.points);
                AD.emit(race, 'j_turn', { car: actor.index, x: v.x, y: v.y, points: pts });
            }
        } else if (st.revSeen && st.sinceRev > 1.5) st.revSeen = false;
    }

    AD.updateVehicleTricks = function (race, actor, input, dt) {
        const v = actor.v, st = actor.tricks;
        if (v.out || !st) return;
        if (v.airborne) return;
        driftStep(race, actor, st, dt);
        slingshotStep(race, actor, st, dt);
        reverseStep(race, actor, st, dt);
        /* After touchdown the body settles flat; ramps drive pitch themselves. */
        if (v.onRamp < 0 && (v.roll !== 0 || v.pitch !== 0)) {
            v.roll = Math.abs(v.roll) < 1e-3 ? 0 : AD.smooth(v.roll, 0, 10, dt);
            v.pitch = Math.abs(v.pitch) < 1e-3 ? 0 : AD.smooth(v.pitch, 0, 10, dt);
        }
    };
})();
