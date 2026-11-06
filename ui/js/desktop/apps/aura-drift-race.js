/* Aura Drift - race state machine, grid, progress and deterministic replay (section 2.1). */
(function () {
    'use strict';
    const AD = window.AuraDriftCore;
    const clamp = AD.clamp;
    const TAPE_CAP = 72000;      /* 10 minutes of fixed steps */
    const LOOSE = { gravel: true, sand: true, mud: true, snow: true };

    function startGrid(track, count) {
        const grid = [];
        for (let i = 0; i < count; i++) {
            const s = track.length - 14 - Math.floor(i / 2) * 9;
            const p = AD.pointAt(track, s);
            const lateral = (i % 2 === 0 ? -1 : 1) * track.width * 0.19;
            grid.push({
                x: p.x + p.nx * lateral, y: p.y + p.ny * lateral,
                psi: Math.atan2(p.ty, p.tx), s: s
            });
        }
        return grid;
    }

    function newTally() {
        return {
            wallHits: 0, brakeSeconds: 0, airTime: 0, drifts: 0, driftSeconds: 0, reverseSeconds: 0,
            tricks: 0, crashLandings: 0, perfectLandings: 0, animalHits: 0, smashed: 0, weaponsFired: 0,
            hitsDealt: 0, hitsTaken: 0, sausageHits: 0, discoFired: 0, carHits: 0, respawns: 0
        };
    }

    function makeCar(index, g, carId, driver, livery, rng, track, isPlayer) {
        return {
            v: AD.createVehicle(g.x, g.y, g.psi, AD.carIndex(carId)),
            isPlayer: isPlayer, index: index,
            input: AD.createInput(),
            driver: driver, livery: livery,
            lap: 0, gate: 0, started: false, lastS: g.s, progress: g.s,
            lapStart: 0, bestLap: Infinity, lastLap: 0,
            sector: 0, sectorStart: 0, bestSectors: [Infinity, Infinity, Infinity],
            finished: false, finishTime: 0, eliminatedAt: null,
            weapon: null, ammo: 0, cooldown: 0, weaponLock: 0, fuel: 0, holdFire: false, fireLatch: false,
            style: typeof AD.createStyleState === 'function' ? AD.createStyleState() : null,
            tricks: AD.createTrickState(), tally: newTally(),
            /* Opponent tuning comes from the driver, the lane from the race rng. */
            ai: {
                aggression: driver ? driver.aggression : 1,
                skill: driver ? driver.skill : 1,
                drift: driver ? driver.drift : 0.5,
                lane: (rng() - 0.5) * track.width * 0.3,
                fireTimer: 3 + rng() * 5,
                stuckTime: 0, recover: 0, respawns: 0, catchup: false
            },
            tape: [], ghost: null
        };
    }

    /* options: { track, mode, seed, laps, opponents, weapons, ghostTape, ghostCar,
                  showGhost, readonly, playerCar, livery } */
    AD.createRace = function (options) {
        const track = options.track;
        const seed = (options.seed === undefined ? track.seed : options.seed) >>> 0;
        const rng = AD.rng(seed);
        const modeId = options.mode || 'quick';
        const rules = AD.modeRules(modeId);
        const opponents = clamp(Math.floor(options.opponents === undefined ? rules.opponents : options.opponents), 0, 5);
        const weaponsOn = options.weapons !== false && rules.weapons !== false;
        const playerCar = AD.carById(options.playerCar || 'vektor').id;
        const drivers = AD.driverField(playerCar, opponents, rng);
        const count = opponents + 1;
        const grid = startGrid(track, count);

        /* The player starts from the back of the first rows, so most races open with overtakes. */
        const playerSlot = Math.min(4, opponents);
        const others = [];
        for (let i = 0; i < count; i++) if (i !== playerSlot) others.push(i);
        const cars = [makeCar(0, grid[playerSlot], playerCar, null, options.livery | 0, rng, track, true)];
        for (let i = 0; i < opponents; i++) {
            cars.push(makeCar(i + 1, grid[others[i]], drivers[i].carId, drivers[i], Math.floor(rng() * 4), rng, track, false));
        }

        const laps = typeof rules.laps === 'number' ? rules.laps : (options.laps || track.laps);
        const featureRules = Object.assign({}, rules, { crates: rules.crates && weaponsOn });
        const features = AD.createFeatures(track, seed, featureRules);

        const race = {
            track: track, mode: modeId, seed: seed, rng: rng, rules: rules,
            weapons: weaponsOn,
            laps: laps, totalLaps: laps,
            cars: cars, drivers: drivers,
            features: features,
            pickups: features.crates,          /* compatibility alias for the 2D renderer */
            hazards: [], projectiles: [], beams: [], events: [],
            time: 0, state: 'countdown', countdown: 3.2,
            playerInput: AD.createInput(),
            ghostTape: null, ghost: null,
            bestLap: Infinity, standings: [], result: null, readonly: !!options.readonly,
            clock: null, elimination: { next: null, count: 0 },
            bomb: { holder: -1, fuse: 0, round: 0, delay: 0, immune: -1, immuneFor: 0 },
            nextOut: -1, muteEvents: false, tapeOverflow: false,
            lastPosition: 0, lastWeather: 'clear',
            conditions: typeof AD.createConditions === 'function' ? AD.createConditions(track, seed) : null
        };

        const tape = AD.decodeTape(options.ghostTape);
        if (tape.length && options.showGhost !== false) {
            const g = grid[playerSlot];
            race.ghostTape = tape;
            race.ghost = {
                v: AD.createVehicle(g.x, g.y, g.psi, AD.carIndex(options.ghostCar || playerCar)),
                step: 0, active: true, index: -1, ghost: true, isPlayer: false,
                style: null, tricks: AD.createTrickState(), tally: null,
                fs: AD.createFeatures(track, seed, Object.assign({}, featureRules, { crates: false }))
            };
        }
        AD.initMode(race);
        return race;
    };

    /* The player's input is what the tape stores: quantised to the tape format, so a
       ghost replaying the tape is bit-identical to the run that recorded it. */
    function recordInput(race, car, input) {
        const packed = AD.packInput(input);
        if (car.tape.length < TAPE_CAP) car.tape.push(packed);
        else race.tapeOverflow = true;
        return AD.unpackInput(packed);
    }

    function environmentAt(race, actor) {
        const v = actor.v, car = v.car;
        const sample = AD.sampleTrack(race.track, v.x, v.y);
        const surface = AD.surfaceAt(race.track, sample.s, sample.offset, sample);
        let grip = surface.grip, roll = surface.roll;
        /* Car quirks that act on the surface. */
        if (car.quirk === 'wet_grip' && surface.type === 'wet') grip *= car.quirkK;
        if (LOOSE[surface.type]) {
            if (car.quirk === 'loose_grip') grip *= car.quirkK;
            roll *= car.looseRoll;
        }
        /* Rain and snow take the grip away in the same way for every driver. */
        if (race.conditions) grip *= race.conditions.state.grip;
        const env = { grip: grip, roll: roll, sample: sample, slipstream: 0, driveK: 1 };
        /* Hot potato: the bomb holder runs hot (rules.driveK). */
        if (v.hasBomb && race.rules && race.rules.driveK) env.driveK = race.rules.driveK;
        if (!actor.ghost) AD.applyHazardEnv(race, actor, env);

        /* Slipstream: tucked directly behind a rival at speed. Deterministic and
           symmetric for player and AI. The ghost never gets one (D4). */
        let draft = 0;
        if (!actor.ghost && Math.max(0, v.vx) > 18) {
            const cos = Math.cos(v.psi), sin = Math.sin(v.psi);
            for (const other of race.cars) {
                if (other.v === v || other.v.out) continue;
                const dx = other.v.x - v.x, dy = other.v.y - v.y;
                const dist = Math.hypot(dx, dy);
                if (dist < 4 || dist > 20) continue;
                const ahead = (dx * cos + dy * sin) / Math.max(0.001, dist);
                const lateral = Math.abs(-dx * sin + dy * cos);
                const aligned = Math.cos(other.v.psi - v.psi);
                if (ahead > 0.86 && lateral < 3.2 && aligned > 0.7) {
                    draft = Math.max(draft, (1 - dist / 20) * ahead * aligned);
                }
            }
            if (car.quirk === 'slipstream') draft *= car.quirkK;
        }
        env.slipstream = clamp(draft, 0, 1);
        v.surface = surface.type;
        return env;
    }

    function updateProgress(race, car) {
        const sample = AD.sampleTrack(race.track, car.v.x, car.v.y);
        const s = sample.s;
        const length = race.track.length;
        const wrapped = car.lastS > length * 0.75 && s < length * 0.25;
        if (wrapped) {
            /* At least ten of the twelve checkpoints must have been reached;
               crossing the line backwards or cutting a corner cannot score a lap. */
            if (car.started && car.gate >= 10) {
                car.lap++;
                const lapTime = race.time - car.lapStart;
                car.lapStart = race.time;
                car.lastLap = lapTime;
                if (lapTime < car.bestLap) car.bestLap = lapTime;
                if (car.isPlayer) {
                    AD.emit(race, 'lap', { car: car.index, lap: car.lap, time: lapTime });
                    if (car.lap === race.laps - 1 && race.laps > 1 && race.laps < 99) AD.emit(race, 'final_lap', { car: car.index });
                }
                if (car.lap >= race.laps && !car.finished) {
                    car.finished = true;
                    car.finishTime = race.time;
                    AD.emit(race, 'finish', { car: car.index, time: race.time });
                }
            }
            car.started = true;
            car.gate = 1;
            car.sector = 0;
            car.sectorStart = race.time;
        } else if (car.started) {
            while (car.gate < 12 && s >= race.track.gates[car.gate]) car.gate++;
        }
        /* Sector splits: the rhythm of a race lap and the thing a driver
           actually compares between attempts. */
        const sectors = race.track.sectors;
        if (car.started && sectors && sectors.length) {
            while (car.sector < sectors.length && s >= sectors[car.sector]) {
                const index = car.sector;
                const split = race.time - car.sectorStart;
                car.sectorStart = race.time;
                car.sector++;
                if (car.isPlayer) {
                    if (split < car.bestSectors[index]) car.bestSectors[index] = split;
                    AD.emit(race, 'sector', { index: index, time: split, best: car.bestSectors[index] });
                }
            }
        }
        car.lastS = s;
        car.progress = car.lap * length + s;
    }

    /* One fixed step of one actor (a car or the ghost stub). */
    function stepActor(race, actor, input, dt, fs) {
        const v = actor.v;
        if (actor.isPlayer && !actor.ghost) input = recordInput(race, actor, input);
        if (!actor.ghost) AD.updateWeaponInput(race, actor, input, dt);
        const env = environmentAt(race, actor);

        const effective = AD.copyInput(input);
        if (v.disabled > 0) { effective.throttle = 0; effective.boost = 0; }
        if (v.frozen > 0) { effective.throttle = 0; effective.steer *= 0.2; }
        if (v.steerWobble > 0) {
            v.wobblePhase += dt * 11;
            effective.steer = clamp(effective.steer + 0.45 * Math.sin(v.wobblePhase), -1, 1);
        }
        if (v.airborne) AD.stepAir(actor, effective, dt, race);
        else AD.stepVehicle(v, effective, dt, env);

        const contacts = v.wallContacts;
        AD.resolveWall(v, env.sample, dt);
        if (v.wallContacts > contacts) {
            if (actor.tally) actor.tally.wallHits += v.wallContacts - contacts;
            if (v.impact > 0.12 && race.time - (actor.wallEventAt || -1) > 0.2) {
                actor.wallEventAt = race.time;
                AD.emit(race, 'wall_hit', { car: actor.index, x: v.x, y: v.y, speed: Math.abs(v.vx), heavy: v.impact > 0.5 });
            }
        }
        AD.updateVehicleTricks(race, actor, input, dt);
        AD.updateActorFeatures(race, actor, fs, dt);
        if (actor.ghost) return;
        updateProgress(race, actor);
        AD.updateCarEffects(race, actor, dt);
    }

    function updateGhost(race, dt) {
        const g = race.ghost;
        if (!g || !g.active) return;
        if (g.step >= race.ghostTape.length) { g.active = false; return; }
        const input = AD.unpackInput(race.ghostTape[g.step]);
        g.step++;
        const was = race.muteEvents;
        race.muteEvents = true;
        stepActor(race, g, input, dt, g.fs);
        race.muteEvents = was;
    }

    function countdownStep(race, dt) {
        for (const car of race.cars) {
            if (car.v.out) continue;
            let input = car.isPlayer ? race.playerInput : AD.aiInput(race, car);
            if (car.isPlayer) input = recordInput(race, car, input);
            AD.updateStart(car, input, dt);
        }
        const g = race.ghost;
        if (g && g.active) {
            if (g.step >= race.ghostTape.length) g.active = false;
            else AD.updateStart(g, AD.unpackInput(race.ghostTape[g.step++]), dt);
        }
        const before = Math.ceil(race.countdown);
        race.countdown -= dt;
        const after = Math.ceil(race.countdown);
        if (after !== before && after >= 0) AD.emit(race, 'count', { value: after });
        if (race.countdown <= 0) {
            race.state = 'running';
            for (const car of race.cars) AD.applyStart(race, car);
            if (g) AD.applyStart(race, g);
            AD.emit(race, 'go', {});
        }
    }

    /* Victory lap (state 'finished'): the result is already built, and every car, the player's slot included, keeps driving
       under the opponent controller so the results never sit over a parked car. Only vehicle state moves: events are muted,
       and the tape, tally, style, tricks, weapons, scoring and race.time stay untouched, so the recorded race and its result
       are bit-identical with or without the lap. Nothing here uses the race rng. */
    const OUTRO_SPEED = 34;
    function outroStep(race, dt) {
        race.outro = (race.outro || 0) + dt;
        const was = race.muteEvents;
        race.muteEvents = true;
        for (const car of race.cars) {
            const v = car.v;
            if (v.out) continue;
            if (v.airborne) { v.airborne = false; v.z = 0; v.vz = 0; v.roll = 0; v.pitch = 0; v.rollRate = 0; v.pitchRate = 0; v.spinRate = 0; }
            const tally = car.tally;
            car.tally = null;
            const input = AD.aiInput(race, car);
            input.fire = 0; input.boost = 0;
            if (v.vx > OUTRO_SPEED) input.throttle = 0;
            const env = environmentAt(race, car);
            AD.stepVehicle(v, input, dt, env);
            AD.resolveWall(v, env.sample, dt);
            car.tally = tally;
        }
        for (let i = 0; i < race.cars.length; i++) {
            const a = race.cars[i];
            if (a.v.out) continue;
            for (let j = i + 1; j < race.cars.length; j++) {
                const b = race.cars[j];
                if (!b.v.out) AD.resolveCarPair(a.v, b.v, (a.v.car.radius + b.v.car.radius) / 2);
            }
        }
        race.muteEvents = was;
    }

    AD.stepRace = function (race, dt) {
        if (race.state === 'countdown') { countdownStep(race, dt); return; }
        if (race.state === 'finished') { outroStep(race, dt); return; }
        race.time += dt;

        /* Player rescue (key T): a one-shot request, never part of the tape, so time
           trial (ghost replays) ignores it. 3 s cooldown. */
        if (race.rescueRequest) {
            race.rescueRequest = false;
            const me = race.cars[0];
            if (race.mode !== 'time_trial' && me.isPlayer && !me.v.out && !me.finished &&
                race.time - (race.rescueAt === undefined ? -Infinity : race.rescueAt) >= 3) {
                race.rescueAt = race.time;
                AD.respawnCar(race, me);
            }
        }

        for (const car of race.cars) {
            if (car.v.out) continue;
            const input = car.isPlayer ? race.playerInput : AD.aiInput(race, car);
            stepActor(race, car, input, dt, race.features);
        }

        for (let i = 0; i < race.cars.length; i++) {
            const a = race.cars[i];
            if (a.v.out || a.v.cloak > 0) continue;
            for (let j = i + 1; j < race.cars.length; j++) {
                const b = race.cars[j];
                if (b.v.out || b.v.cloak > 0 || Math.abs(a.v.z - b.v.z) > 1.6) continue;
                const closing = AD.resolveCarPair(a.v, b.v, (a.v.car.radius + b.v.car.radius) / 2);
                if (closing > 2) {
                    if (a.tally) a.tally.carHits++;
                    if (b.tally) b.tally.carHits++;
                    AD.emit(race, 'car_hit', {
                        a: a.index, b: b.index, x: (a.v.x + b.v.x) / 2, y: (a.v.y + b.v.y) / 2,
                        impulse: closing, heavy: closing > 9
                    });
                }
            }
        }

        AD.updateProjectiles(race, dt);
        AD.updateHazards(race, dt);
        AD.updateFeatures(race, race.features, dt);
        updateGhost(race, dt);
        if (race.conditions) {
            const state = race.conditions.update(dt);
            if (state.kind !== race.lastWeather) {
                race.lastWeather = state.kind;
                AD.emit(race, 'weather', { kind: state.kind, intensity: state.targetIntensity });
            }
        }
        if (typeof AD.updateStyle === 'function') AD.updateStyle(race, dt);

        /* Out cars rank last, the one that went out latest first. */
        race.standings = race.cars.slice().sort((a, b) => {
            if (a.v.out !== b.v.out) return a.v.out ? 1 : -1;
            if (a.v.out) return (b.eliminatedAt || 0) - (a.eliminatedAt || 0);
            if (a.finished !== b.finished) return a.finished ? -1 : 1;
            if (a.finished && b.finished) return a.finishTime - b.finishTime;
            return b.progress - a.progress;
        });
        const player = race.cars[0];
        if (player.bestLap < race.bestLap) race.bestLap = player.bestLap;
        if (player.isPlayer) {
            const position = race.standings.indexOf(player) + 1;
            if (race.lastPosition && position !== race.lastPosition) {
                AD.emit(race, 'position', { car: 0, from: race.lastPosition, to: position });
            }
            race.lastPosition = position;
        }

        AD.updateMode(race, dt);
        if (AD.modeFinished(race) && player.isPlayer && race.state === 'running') {
            race.state = 'finished';
            race.result = AD.buildResult(race);
        }
    };

    AD.playerPosition = function (race) {
        const index = race.standings.indexOf(race.cars[0]);
        return index < 0 ? 1 : index + 1;
    };
})();
