/* Aura Drift - race state machine, opponents, weapons and deterministic replay. */
(function () {
    'use strict';
    const AD = window.AuraDriftCore;
    const clamp = AD.clamp;

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

    AD.createRace = function (options) {
        const track = options.track;
        const seed = (options.seed === undefined ? track.seed : options.seed) >>> 0;
        const rng = AD.rng(seed);
        const count = 1 + clamp(options.opponents === undefined ? 3 : options.opponents, 0, 3);
        const grid = startGrid(track, count);
        const cars = [];
        for (let i = 0; i < count; i++) {
            const g = grid[i];
            cars.push({
                v: AD.createVehicle(g.x, g.y, g.psi, i),
                isPlayer: i === 0, index: i,
                input: AD.createInput(),
                lap: 0, gate: 0, started: false, lastS: g.s, progress: g.s,
                lapStart: 0, bestLap: Infinity, lastLap: 0,
                finished: false, finishTime: 0,
                weapon: null, ammo: 0, cooldown: 0,
                ai: {
                    aggression: 0.82 + rng() * 0.3,
                    lane: (rng() - 0.5) * track.width * 0.3,
                    fireTimer: 3 + rng() * 5,
                    stuckTime: 0, recover: 0, respawns: 0, catchup: false
                },
                tape: [], ghost: null
            });
        }
        const pickups = track.pads.map((pad, i) => {
            const p = AD.pointAt(track, pad.s);
            return {
                x: p.x + p.nx * pad.offset, y: p.y + p.ny * pad.offset,
                s: pad.s, taken: -99, index: i
            };
        });

        const race = {
            track: track, mode: options.mode || 'quick', seed: seed, rng: rng,
            weapons: options.weapons !== false,
            laps: options.laps || track.laps,
            cars: cars, pickups: pickups,
            hazards: [], projectiles: [], events: [],
            time: 0, state: 'countdown', countdown: 3.2,
            playerInput: AD.createInput(),
            ghostTape: options.ghostTape || null,
            ghost: null, ghostStep: 0,
            bestLap: Infinity, totalLaps: options.laps || track.laps,
            standings: [], result: null, readonly: !!options.readonly
        };

        if (race.ghostTape && race.ghostTape.length && options.showGhost !== false) {
            const g = grid[0];
            race.ghost = { v: AD.createVehicle(g.x, g.y, g.psi, 0), step: 0, active: true };
        }
        return race;
    };

    function emit(race, type, data) {
        race.events.push(Object.assign({ type: type, t: race.time }, data || {}));
    }

    function respawnCar(race, car) {
        const v = car.v;
        const current = AD.sampleTrack(race.track, v.x, v.y);
        const point = AD.pointAt(race.track, current.s + 10);
        v.x = point.x; v.y = point.y;
        v.psi = Math.atan2(point.ty, point.tx);
        v.vx = 10; v.vy = 0; v.r = 0;
        v.disabled = 0; v.impact = 0.4;
        car.lastS = current.s;
        car.ai.stuckTime = 0;
        car.ai.recover = 0;
        car.ai.respawns++;
        emit(race, 'respawn', { x: v.x, y: v.y, car: car.index });
    }

    function aiInput(race, car) {
        const v = car.v, track = race.track;
        const sample = AD.sampleTrack(track, v.x, v.y);
        const speed = Math.max(0, v.vx);
        const ahead = AD.pointAt(track, sample.s + 26 + speed * 0.85);
        const curvature = Math.abs(ahead.tx * sample.ty - ahead.ty * sample.tx);
        const lane = car.ai.lane * (1 - clamp(curvature * 4, 0, 0.8));
        const tx = ahead.x + ahead.nx * lane, ty = ahead.y + ahead.ny * lane;
        const wanted = Math.atan2(ty - v.y, tx - v.x);
        const error = AD.wrapAngle(wanted - v.psi);
        const input = car.input;
        input.steer = clamp(error * 2.4 - v.r * 0.22, -1, 1);

        /* Surface grip and corridor position cap the achievable corner speed.
           Without this the narrow, muddy rally stage becomes a wall-scraping
           convoy instead of a race. */
        const surface = AD.surfaceAt(track, sample.s, sample.offset, sample);
        const gripFactor = 0.58 + 0.42 * clamp(surface.grip, 0.2, 1.2);
        const corridor = Math.abs(sample.offset) / Math.max(1, sample.halfWidth);
        const corridorFactor = 1 - 0.4 * clamp((corridor - 0.55) / 0.45, 0, 1);
        let leader = car.progress;
        for (const other of race.cars) leader = Math.max(leader, other.progress);
        const gap = Math.max(0, leader - car.progress);
        /* Bounded, disclosed catch-up: at most +6% target speed, never extra grip. */
        const catchup = 1 + clamp((gap - 150) / 900, 0, 1) * 0.06;
        car.ai.catchup = catchup > 1.001;
        car.ai.catchupFactor = catchup;

        const cornerLimit = 18 + 58 * car.ai.aggression / Math.max(0.35, 1 + curvature * 14 + Math.abs(sample.offset) / 40);
        const target = clamp(cornerLimit * gripFactor * corridorFactor * catchup, 10, 74);
        input.throttle = speed < target ? 1 : 0;
        input.brake = speed > target + 5 ? clamp((speed - target) / 14, 0, 1) : 0;
        input.handbrake = (curvature > 0.3 && speed > 38 && Math.abs(error) > 0.42) ? 1 : 0;
        /* Same nitro resource as the player: burn it on clean straights only. */
        input.boost = (v.nitro > 0.55 && curvature < 0.02 && speed > 46) ? 1 : 0;
        input.fire = 0;

        if (corridor > 0.5) {
            input.steer = clamp(input.steer - Math.sign(sample.offset) * clamp((corridor - 0.5) * 1.6, 0, 0.85), -1, 1);
        }
        /* Two-stage recovery: hold the brake so the new reverse gear backs the
           car out straight, then let a marshal put it back on the racing line
           if it stays pinned. This keeps weapon spins from parking an opponent
           against a wall forever. */
        car.ai.recover = Math.max(0, car.ai.recover - AD.STEP);
        const pinned = speed < 4 && corridor > 0.65;
        car.ai.stuckTime = pinned ? car.ai.stuckTime + AD.STEP : 0;
        if (car.ai.stuckTime > 4) respawnCar(race, car);
        else if (car.ai.stuckTime > 0.5 && car.ai.recover <= 0) {
            car.ai.recover = 1.1;
            car.ai.stuckTime = 0;
        }
        if (car.ai.recover > 0) {
            input.throttle = 0;
            input.brake = 1;
            input.handbrake = 0;
            input.fire = 0;
            input.boost = 0;
            input.steer = 0;
            return input;
        }
        if (race.weapons && car.weapon && car.cooldown <= 0) {
            car.ai.fireTimer -= AD.STEP;
            if (car.ai.fireTimer <= 0) {
                input.fire = 1;
                car.ai.fireTimer = 4 + race.rng() * 6;
            }
        }
        return input;
    }

    function environmentAt(race, v) {
        const sample = AD.sampleTrack(race.track, v.x, v.y);
        const surface = AD.surfaceAt(race.track, sample.s, sample.offset, sample);
        let grip = surface.grip, roll = surface.roll;
        for (const h of race.hazards) {
            if (h.type === 'oil' && h.ttl > 0 && AD.distance(v.x, v.y, h.x, h.y) < h.r) {
                grip = Math.min(grip, AD.SURFACES.oil.grip);
            }
        }
        v.surface = surface.type;
        return { grip: grip, roll: roll, sample: sample };
    }

    function fireWeapon(race, car) {
        if (!race.weapons || !car.weapon || car.cooldown > 0 || car.ammo <= 0) return;
        const def = AD.WEAPONS[car.weapon];
        const v = car.v;
        const cos = Math.cos(v.psi), sin = Math.sin(v.psi);
        const bx = v.x - cos * 3.2, by = v.y - sin * 3.2;
        car.cooldown = def.cooldown;
        car.ammo--;
        emit(race, 'weapon', { id: def.id, x: v.x, y: v.y, car: car.index });

        switch (def.id) {
            case 'oil':
                race.hazards.push({ type: 'oil', x: bx, y: by, r: 7, ttl: 6, owner: car.index });
                break;
            case 'mine':
                race.hazards.push({ type: 'mine', x: bx, y: by, r: 3.4, ttl: 9, arm: 0.45, owner: car.index });
                break;
            case 'emp':
                for (const other of race.cars) {
                    if (other === car) continue;
                    if (AD.distance(v.x, v.y, other.v.x, other.v.y) < 30) {
                        if (other.v.shield > 0) continue;
                        other.v.disabled = 1.5;
                        emit(race, 'emp_hit', { x: other.v.x, y: other.v.y });
                    }
                }
                emit(race, 'emp_pulse', { x: v.x, y: v.y });
                break;
            case 'rocket': {
                let target = null, best = Infinity;
                for (const other of race.cars) {
                    if (other === car) continue;
                    const d = AD.distance(v.x, v.y, other.v.x, other.v.y);
                    if (d < best) { best = d; target = other.index; }
                }
                race.projectiles.push({ type: 'rocket', x: v.x + cos * 3, y: v.y + sin * 3, psi: v.psi, speed: 62, ttl: 6, target: target, owner: car.index });
                break;
            }
            case 'mg':
                for (let i = -2; i <= 2; i++) {
                    const spread = i * 0.055;
                    race.projectiles.push({ type: 'bullet', x: v.x + cos * 3, y: v.y + sin * 3, psi: v.psi + spread, speed: 105, ttl: 0.65, owner: car.index });
                }
                break;
            case 'shield':
                v.shield = 4;
                v.damage = Math.max(0, v.damage - 22);
                emit(race, 'shield', { x: v.x, y: v.y });
                break;
        }
        if (car.ammo <= 0) car.weapon = null;
    }

    function updateProjectiles(race, dt) {
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
                if (car.index === p.owner) continue;
                if (AD.distance(p.x, p.y, car.v.x, car.v.y) < 2.6) {
                    if (car.v.shield <= 0) {
                        car.v.damage = clamp(car.v.damage + (p.type === 'rocket' ? 26 : 7), 0, 100);
                        car.v.vx *= p.type === 'rocket' ? 0.72 : 0.94;
                        emit(race, 'hit', { x: p.x, y: p.y, heavy: p.type === 'rocket' });
                    }
                    hit = true;
                    break;
                }
            }
            if (hit) race.projectiles.splice(i, 1);
        }
    }

    function updateHazards(race, dt) {
        for (let i = race.hazards.length - 1; i >= 0; i--) {
            const h = race.hazards[i];
            h.ttl -= dt;
            if (h.arm > 0) h.arm -= dt;
            if (h.ttl <= 0) { race.hazards.splice(i, 1); continue; }
            if (h.type !== 'mine' || h.arm > 0) continue;
            for (const car of race.cars) {
                if (car.index === h.owner) continue;
                if (AD.distance(h.x, h.y, car.v.x, car.v.y) < h.r + 1.4) {
                    if (car.v.shield <= 0) {
                        car.v.damage = clamp(car.v.damage + 16, 0, 100);
                        car.v.vx *= 0.68;
                        car.v.r += (race.rng() - 0.5) * 0.7;
                        emit(race, 'hit', { x: h.x, y: h.y, heavy: true });
                    }
                    race.hazards.splice(i, 1);
                    break;
                }
            }
        }
    }

    function updatePickups(race) {
        for (const pad of race.pickups) {
            if (race.time - pad.taken < 8) continue;
            for (const car of race.cars) {
                if (AD.distance(pad.x, pad.y, car.v.x, car.v.y) < 4.2) {
                    pad.taken = race.time;
                    if (!car.weapon) {
                        car.weapon = AD.WEAPON_IDS[Math.floor(race.rng() * AD.WEAPON_IDS.length)];
                        car.ammo = AD.WEAPONS[car.weapon].ammo;
                        car.cooldown = 0;
                        emit(race, 'pickup', { x: pad.x, y: pad.y, car: car.index, id: car.weapon });
                    }
                    break;
                }
            }
        }
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
                if (car.isPlayer) emit(race, 'lap', { lap: car.lap, time: lapTime });
                if (car.lap >= race.laps && !car.finished) {
                    car.finished = true;
                    car.finishTime = race.time;
                    emit(race, 'finish', { car: car.index, time: race.time });
                }
            }
            car.started = true;
            car.gate = 1;
        } else if (car.started) {
            while (car.gate < 12 && s >= race.track.gates[car.gate]) car.gate++;
        }
        car.lastS = s;
        car.progress = car.lap * length + s;
    }

    function updateGhost(race, dt) {
        if (!race.ghost || !race.ghost.active) return;
        const tape = race.ghostTape;
        if (race.ghost.step >= tape.length) { race.ghost.active = false; return; }
        const input = AD.unpackInput(tape[race.ghost.step]);
        race.ghost.step++;
        const env = environmentAt(race, race.ghost.v);
        AD.stepVehicle(race.ghost.v, input, dt, env);
        AD.resolveWall(race.ghost.v, env.sample, dt);
    }

    AD.stepRace = function (race, dt) {
        if (race.state === 'countdown') {
            const before = Math.ceil(race.countdown);
            race.countdown -= dt;
            const after = Math.ceil(race.countdown);
            if (after !== before && after >= 0) emit(race, 'count', { value: after });
            if (race.countdown <= 0) {
                race.state = 'running';
                emit(race, 'go', {});
            }
            return;
        }
        if (race.state === 'finished') return;
        race.time += dt;

        for (const car of race.cars) {
            const input = car.isPlayer ? race.playerInput : aiInput(race, car);
            if (car.isPlayer) {
                car.tape.push(AD.packInput(input));
                if (car.tape.length > 36000) car.tape.shift();
            }
            if (car.cooldown > 0) car.cooldown = Math.max(0, car.cooldown - dt);
            if (input.fire && !car.fireLatch) fireWeapon(race, car);
            car.fireLatch = !!input.fire;

            const env = environmentAt(race, car.v);
            const envInput = AD.copyInput(input);
            if (car.v.disabled > 0) { envInput.throttle = 0; envInput.boost = 0; }
            AD.stepVehicle(car.v, envInput, dt, env);
            AD.resolveWall(car.v, env.sample, dt);
            updateProgress(race, car);
        }

        for (let i = 0; i < race.cars.length; i++) {
            for (let j = i + 1; j < race.cars.length; j++) {
                AD.resolveCarPair(race.cars[i].v, race.cars[j].v, 2.05);
            }
        }

        updateProjectiles(race, dt);
        updateHazards(race, dt);
        updatePickups(race);
        updateGhost(race, dt);

        race.standings = race.cars.slice().sort((a, b) => {
            if (a.finished !== b.finished) return a.finished ? -1 : 1;
            if (a.finished && b.finished) return a.finishTime - b.finishTime;
            return b.progress - a.progress;
        });
        const player = race.cars[0];
        if (player.bestLap < race.bestLap) race.bestLap = player.bestLap;
        if (player.finished && player.isPlayer && race.state === 'running') {
            race.state = 'finished';
            race.result = {
                time: player.finishTime, bestLap: player.bestLap,
                position: race.standings.indexOf(player) + 1,
                tape: player.tape.slice()
            };
        }
    };

    AD.playerPosition = function (race) {
        const index = race.standings.indexOf(race.cars[0]);
        return index < 0 ? 1 : index + 1;
    };
})();
