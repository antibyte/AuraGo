/* Aura Drift - ramps, pads, crates, destructibles, animals, set pieces (section 2.4).
   Deterministic from hash(track.id + '/features/' + seed). The ghost owns its own
   instance and runs through the same code; race.muteEvents silences its events and it
   never scores (style and tally are null). */
(function () {
    'use strict';
    const AD = window.AuraDriftCore;
    const clamp = AD.clamp;

    const BUCKETS = 64;
    const RAMP = { length: 9, height: 1.6 };
    const PAD = { width: 6, length: 10, power: 1.35, duration: 0.9, cooldown: 1.5 };
    const CRATE = { radius: 4.2, respawn: 8, maxZ: 3.5 };
    const DESTRUCT = {
        cone: { r: 0.4, speed: 0.995, lift: 7, push: 7 },
        hay: { r: 1.1, speed: 0.96, lift: 4.5, push: 5 },
        barrel: { r: 0.7, speed: 0.97, lift: 6, push: 6 },
        tyres: { r: 1.0, speed: 0.94, lift: 5, push: 5.5 }
    };
    const DESTRUCT_KINDS = ['cone', 'hay', 'barrel', 'tyres'];
    const DESTRUCT_RESPAWN = 20;
    const GRAVITY = 18;
    const ANIMAL_HIT_Z = 2.2;
    const TROT_TIME = 2.5;
    const SMALL_ANIMALS = { sheep: true, penguin: true, crab: true };
    const ANIMALS = AD.ANIMAL_FOR_BIOME || {
        alpine: 'cow', desert: 'llama', harbour: 'crab', salt: 'cow',
        forest: 'sheep', industry: 'sheep', glacier: 'penguin', metro: 'llama'
    };

    function wrapDist(a, b, length) {
        const d = Math.abs(a - b) % length;
        return Math.min(d, length - d);
    }
    function bucketOf(track, s) {
        const f = ((s % track.length) + track.length) % track.length;
        return Math.min(BUCKETS - 1, Math.floor(f / track.length * BUCKETS));
    }
    function place(track, s, offset) {
        const p = AD.pointAt(track, s);
        return { x: p.x + p.nx * offset, y: p.y + p.ny * offset, psi: Math.atan2(p.ty, p.tx), p: p };
    }
    function worldVel(v) {
        if (v.airborne) return [v.wx || 0, v.wy || 0];
        const c = Math.cos(v.psi), s = Math.sin(v.psi);
        return [v.vx * c - v.vy * s, v.vx * s + v.vy * c];
    }
    function scale(v, k) { v.vx *= k; v.vy *= k; }

    AD.createFeatures = function (track, seed, rules) {
        rules = rules || {};
        const base = track.id + '/features/' + seed;
        const length = track.length;
        const fs = {
            rng: AD.rng(AD.hashString(base)),
            rules: rules,
            crates: [], ramps: [], pads: [], destructibles: [], animals: [], crossings: [], setpieces: [],
            buckets: [], stamp: -1, padAt: {}, herdHits: {}, lastT: 0
        };
        for (let i = 0; i < BUCKETS; i++) fs.buckets.push([]);
        const add = f => { fs.buckets[bucketOf(track, f.s)].push(f); return f; };
        const nearLine = s => wrapDist(s, 0, length) < 48;
        const windows = AD.straightWindows(track, 330);

        if (rules.crates !== false) {
            track.pads.forEach((pad, i) => {
                const q = place(track, pad.s, pad.offset);
                fs.crates.push(add({
                    id: i, kind: 'crate', s: pad.s, offset: pad.offset, row: pad.row,
                    x: q.x, y: q.y, taken: -99          /* race time of the last pickup */
                }));
            });
        }

        if (rules.ramps !== false && windows.length) {
            const count = clamp(1 + Math.floor(length / 1400), 2, 5);
            const rng = AD.rng(AD.hashString(base + '/ramps'));
            const slots = [];
            for (let k = 0; k < 4 && slots.length < count * 3; k++) {
                for (const w of windows) {
                    const s = w.to - 150 - k * 75;
                    if (s >= w.from + 24 && s >= 48 && s + RAMP.length <= length - 48) slots.push(s);
                }
            }
            for (let i = 0; i < count && i < slots.length; i++) {
                const s = slots[i];
                const offset = (rng() < 0.5 ? -1 : 1) * 0.24 * track.width;
                const q = place(track, s, offset);
                fs.ramps.push(add({
                    id: fs.ramps.length, kind: 'ramp', s: s, offset: offset, x: q.x, y: q.y, psi: q.psi,
                    width: 0.42 * track.width, length: RAMP.length, height: RAMP.height
                }));
            }
        }

        if (rules.pads !== false) {
            const count = Math.floor(length / 700);
            const spots = [];
            const exits = AD.straightWindows(track, 200);
            for (const w of exits) spots.push(w.from + 40);
            for (const w of exits) spots.push((w.from + w.to) / 2);
            for (let k = 0; k < count; k++) spots.push(length * (k + 0.5) / Math.max(1, count));
            let sign = 1;
            for (const s of spots) {
                if (fs.pads.length >= count) break;
                if (nearLine(s) || nearLine(s + PAD.length)) continue;
                if (fs.ramps.some(r => wrapDist(r.s, s, length) < 40)) continue;
                if (fs.pads.some(p => wrapDist(p.s, s, length) < 120)) continue;
                const offset = sign * 0.12 * track.width;
                sign = -sign;
                const q = place(track, s, offset);
                fs.pads.push(add({
                    id: fs.pads.length, kind: 'pad', s: s, offset: offset, x: q.x, y: q.y, psi: q.psi,
                    width: PAD.width, length: PAD.length, power: PAD.power, duration: PAD.duration
                }));
            }
        }

        if (rules.animals !== false) {
            const animalKind = ANIMALS[track.biomeId] || 'cow';
            const count = clamp(Math.floor(length / 1800), 1, 3);
            const spots = [];
            const wide = AD.straightWindows(track, 330).concat(AD.straightWindows(track, 240));
            for (const w of wide) {
                for (let s = w.from + 30; s <= w.to - 30; s += 10) spots.push(s);
            }
            for (let c = 0; c < count; c++) {
                /* Prefer the middle of long straights; rotate through candidates so crossings spread out. */
                let chosen = -1;
                const start = spots.length ? Math.floor(spots.length * (c + 0.5) / count) : 0;
                for (let n = 0; n < spots.length; n++) {
                    const s = spots[(start + n) % spots.length];
                    if (nearLine(s) || nearLine(s - 125) || nearLine(s + 12)) continue;
                    if (fs.ramps.some(r => wrapDist(r.s, s, length) < 130)) continue;
                    if (fs.pads.some(p => wrapDist(p.s, s, length) < 60)) continue;
                    if (fs.crossings.some(x => wrapDist(x.s, s, length) < 150)) continue;
                    chosen = s; break;
                }
                if (chosen < 0) continue;
                const q = place(track, chosen, 0);
                const n = 3 + Math.floor(fs.rng() * 3);
                const walk = 6 + fs.rng() * 2;
                const period = Math.max(14 + fs.rng() * 8, 2 * walk + 2);
                const crossing = {
                    id: fs.crossings.length, kind: animalKind, type: 'crossing', s: chosen, x: q.x, y: q.y,
                    signS: chosen - 120, barsS: chosen - 60, n: n, walk: walk, period: period,
                    phase: fs.rng() * period, half: (track.width + 8) / 2, segment: -1
                };
                fs.crossings.push(crossing);
                for (let i = 0; i < n; i++) {
                    const along = (i - (n - 1) / 2) * 3.2;
                    fs.animals.push(add({
                        id: fs.animals.length, type: 'animal', kind: animalKind, crossing: crossing.id, herd: crossing.id, index: i,
                        s: chosen, along: along, x: q.x, y: q.y, psi: 0, state: 'idle', z: 0, vz: 0, vx: 0, vy: 0, t: 0,
                        trotX: 0, trotY: 0
                    }));
                    poseAnimal(track, crossing, fs.animals[fs.animals.length - 1], 0);
                }
            }
        }

        if (rules.destructibles !== false) {
            const rng = AD.rng(AD.hashString(base + '/destructibles'));
            const clusters = Math.max(1, Math.round(6 * length / 1000));
            const spots = [];
            for (const w of AD.straightWindows(track, 150)) spots.push(w.from + 25);
            for (let k = 0; k < clusters * 3; k++) spots.push(length * (k + 0.5) / (clusters * 3));
            const placed = [];
            for (const s of spots) {
                if (placed.length >= clusters) break;
                if (nearLine(s) || nearLine(s + 14)) continue;
                if (fs.ramps.some(r => wrapDist(r.s, s, length) < 70)) continue;
                if (fs.pads.some(p => wrapDist(p.s, s, length) < 40)) continue;
                if (fs.crossings.some(x => wrapDist(x.s, s, length) < 60)) continue;
                if (placed.some(p => wrapDist(p, s, length) < 40)) continue;
                placed.push(s);
                const kind = DESTRUCT_KINDS[Math.floor(rng() * DESTRUCT_KINDS.length)];
                const side = rng() < 0.5 ? -1 : 1;
                const r = DESTRUCT[kind].r;
                const items = 3 + Math.floor(rng() * 3);
                const gap = Math.max(2.6, 2 * r + 0.8);
                for (let i = 0; i < items; i++) {
                    const is = s + i * gap;
                    const half = AD.pointAt(track, is).halfWidth;
                    const offset = side * (half - 1.2 - r);
                    const q = place(track, is, offset);
                    fs.destructibles.push(add({
                        id: fs.destructibles.length, type: 'destructible', kind: kind, s: is, offset: offset, x: q.x, y: q.y, hx: q.x, hy: q.y,
                        r: r, state: 'idle', vx: 0, vy: 0, vz: 0, z: 0, spin: 0, t: 0
                    }));
                }
            }
        }

        if (track.biomeId === 'salt') {
            /* A UFO hovers 300 m off the road on the inner side. */
            let cx = 0, cy = 0;
            for (const smp of track.samples) { cx += smp.x; cy += smp.y; }
            cx /= track.samples.length; cy /= track.samples.length;
            const q = place(track, length * 0.3, 0);
            const inward = Math.sign((cx - q.x) * q.p.nx + (cy - q.y) * q.p.ny) || 1;
            fs.setpieces.push({
                id: 0, kind: 'ufo', s: length * 0.3, x: q.x + q.p.nx * inward * 300, y: q.y + q.p.ny * inward * 300, period: 45
            });
        }
        return fs;
    };

    /* Animal pose before any hit: a pure function of race time. */
    function poseAnimal(track, crossing, a, t) {
        const P = crossing.period, W = crossing.walk, half = crossing.half;
        const tt = (((t + crossing.phase + a.index * 0.3) % P) + P) % P;
        let lateral, dir, walking = false;
        if (tt < W) { lateral = -half + 2 * half * (tt / W); dir = 1; walking = true; }
        else if (tt < P / 2) { lateral = half; dir = 1; }
        else if (tt < P / 2 + W) { lateral = half - 2 * half * ((tt - P / 2) / W); dir = -1; walking = true; }
        else { lateral = -half; dir = -1; }
        const p = AD.pointAt(track, crossing.s + a.along);
        a.baseX = p.x + p.nx * lateral;
        a.baseY = p.y + p.ny * lateral;
        a.basePsi = Math.atan2(p.ny * dir, p.nx * dir);
        a.baseWalking = walking;
        if (a.state === 'idle' || a.state === 'walk') {
            a.x = a.baseX; a.y = a.baseY; a.psi = a.basePsi;
            a.state = walking ? 'walk' : 'idle';
        }
    }

    /* Bring the world state (destructibles, animals, set pieces) up to race.time exactly once per step. */
    function advance(race, fs) {
        if (fs.stamp === race.time) return;
        const dt = fs.stamp < 0 ? AD.STEP : clamp(race.time - fs.stamp, 0, 0.1);
        fs.stamp = race.time;
        const t = race.time, track = race.track;

        for (const d of fs.destructibles) {
            if (d.state !== 'flying') continue;
            if (t - d.t >= DESTRUCT_RESPAWN) {
                d.state = 'idle'; d.x = d.hx; d.y = d.hy; d.z = 0; d.vx = d.vy = d.vz = 0; d.spin = 0;
            } else if (d.z > 0 || d.vz > 0) {
                d.x += d.vx * dt; d.y += d.vy * dt; d.z += d.vz * dt; d.vz -= GRAVITY * dt;
                if (d.z <= 0) { d.z = 0; d.vz = 0; d.vx = d.vy = 0; }
            }
        }

        for (const a of fs.animals) {
            const c = fs.crossings[a.crossing];
            if (a.state === 'flying') {
                a.x += a.vx * dt; a.y += a.vy * dt; a.z += a.vz * dt; a.vz -= GRAVITY * dt;
                if (a.z <= 0) { a.z = 0; a.vz = 0; a.state = 'trot'; a.t = t; a.trotX = a.x; a.trotY = a.y; }
                poseAnimal(track, c, a, t);
            } else if (a.state === 'trot') {
                poseAnimal(track, c, a, t);
                const k = clamp((t - a.t) / TROT_TIME, 0, 1), e = k * k * (3 - 2 * k);
                a.x = a.trotX + (a.baseX - a.trotX) * e;
                a.y = a.trotY + (a.baseY - a.trotY) * e;
                a.psi = Math.atan2(a.baseY - a.y, a.baseX - a.x) || a.basePsi;
                if (k >= 1) {
                    a.state = a.baseWalking ? 'walk' : 'idle';
                    a.x = a.baseX; a.y = a.baseY; a.psi = a.basePsi;
                }
            } else {
                poseAnimal(track, c, a, t);
            }
        }

        /* Warn three seconds before each walking phase. */
        for (const c of fs.crossings) {
            const segment = Math.floor((t + c.phase + 3) / (c.period / 2));
            if (c.segment >= 0 && segment !== c.segment) {
                AD.emit(race, 'animal_cross', { kind: c.kind, x: c.x, y: c.y, crossing: c.id });
            }
            c.segment = segment;
        }

        for (const sp of fs.setpieces) {
            const prev = ((fs.lastT % sp.period) + sp.period) % sp.period;
            const cur = ((t % sp.period) + sp.period) % sp.period;
            if (prev < 20 && cur >= 20) {
                AD.emit(race, 'ufo_abduct', { x: sp.x, y: sp.y, feature: sp.id });
            }
        }
        fs.lastT = t;
    }

    function inRect(f, v, halfLength, halfWidth, centred) {
        const c = Math.cos(f.psi), s = Math.sin(f.psi);
        const dx = v.x - f.x, dy = v.y - f.y;
        const along = dx * c + dy * s, lateral = -dx * s + dy * c;
        return { along: along, lateral: lateral, inside: (centred ? Math.abs(along) <= halfLength : along >= 0 && along <= halfLength) && Math.abs(lateral) <= halfWidth };
    }

    function clearRamp(v) { v.onRamp = -1; v.pitch = 0; if (!v.airborne) v.z = 0; }

    function rampStep(race, actor, ramp) {
        const v = actor.v;
        const r = inRect(ramp, v, ramp.length, ramp.width / 2, false);
        const aligned = Math.cos(AD.wrapAngle(v.psi - ramp.psi)) > 0.35 && v.vx > 1;
        if (v.onRamp === ramp.id) {
            if (r.inside && aligned) {
                v.z = ramp.height * r.along / ramp.length;
                v.pitch = Math.atan2(ramp.height, ramp.length);
            } else if (r.along >= ramp.length && Math.abs(r.lateral) <= ramp.width / 2 + 0.5 && aligned) {
                const vz = v.vx < 12 ? 2 : clamp(0.2 * Math.max(v.vx, 0), 6, 14);
                v.onRamp = -1;
                AD.launch(race, actor, vz, 'ramp');
                if (!v.airborne) { v.z = 0; v.pitch = 0; }
            } else {
                clearRamp(v);      /* left over the side, stopped short or reversed off */
            }
        } else if (r.inside && aligned && r.along <= ramp.length * 0.6 && !v.airborne) {
            v.onRamp = ramp.id;
            v.z = ramp.height * r.along / ramp.length;
            v.pitch = Math.atan2(ramp.height, ramp.length);
        }
    }

    function padStep(race, actor, pad, fs) {
        const v = actor.v;
        if (v.airborne || v.z > 0.5) return;
        const key = actor.index;
        if (race.time - (fs.padAt[key] === undefined ? -99 : fs.padAt[key]) < PAD.cooldown) return;
        if (!inRect(pad, v, pad.length / 2, pad.width / 2, true).inside) return;
        fs.padAt[key] = race.time;
        v.burst = pad.duration * (v.car.quirk === 'pad_time' ? v.car.quirkK : 1);
        v.burstPower = pad.power;
        AD.emit(race, 'boost_pad', { car: actor.index, x: pad.x, y: pad.y });
    }

    function crateStep(race, actor, crate) {
        const v = actor.v;
        if (actor.ghost || !race.weapons || v.z >= CRATE.maxZ) return;
        if (race.time - crate.taken < CRATE.respawn) return;
        if (AD.distance(crate.x, crate.y, v.x, v.y) >= CRATE.radius) return;
        crate.taken = race.time;
        if (actor.weapon) return;
        const field = race.cars.length;
        const rank = Math.max(1, race.standings.indexOf(actor) + 1);
        const fraction = field > 1 ? (rank - 1) / (field - 1) : 0;
        const pool = Array.isArray(race.rules && race.rules.weapons) ? race.rules.weapons : null;
        const id = AD.rollWeapon(race.rng, fraction, pool);
        AD.giveWeapon(race, actor, id);
        AD.emit(race, 'pickup', { x: crate.x, y: crate.y, car: actor.index, id: id, lock: actor.weaponLock });
    }

    function smashStep(race, actor, d) {
        const v = actor.v;
        if (d.state !== 'idle' || v.z > 0.8) return;
        const dx = d.x - v.x, dy = d.y - v.y;
        const dist = Math.hypot(dx, dy);
        if (dist >= d.r + v.car.radius * 0.7) return;
        const spec = DESTRUCT[d.kind];
        const w = worldVel(v);
        const nx = dist > 0.01 ? dx / dist : Math.cos(v.psi), ny = dist > 0.01 ? dy / dist : Math.sin(v.psi);
        /* Deterministic impulse: shoved along the line of impact plus half the car's velocity. */
        d.state = 'flying'; d.t = race.time;
        d.vx = nx * spec.push + w[0] * 0.5; d.vy = ny * spec.push + w[1] * 0.5;
        d.vz = spec.lift; d.z = 0.05; d.spin = (d.id % 2 ? 1 : -1) * (6 + (d.id % 3));
        const free = v.car.quirk === 'sturdy' || v.car.quirk === 'heavy' || (race.rules && race.rules.noSmashPenalty);
        if (!free) scale(v, spec.speed);
        if (actor.tally) actor.tally.smashed++;
        AD.emit(race, 'smash', { car: actor.index, x: d.x, y: d.y, kind: d.kind, feature: d.id });
    }

    function animalStep(race, actor, fs, a) {
        const v = actor.v;
        if (a.state !== 'idle' && a.state !== 'walk') return;
        const dx = a.x - v.x, dy = a.y - v.y;
        const dist = Math.hypot(dx, dy);
        if (v.z >= ANIMAL_HIT_Z) {
            /* Sailing over the herd is not a hit. The cow_vault trick is awarded by
               detectTricks in aura-drift-tricks.js (one path, reading fs.animals). */
            return;
        }
        if (dist >= v.car.radius * 0.6 + 1.1) return;
        const w = worldVel(v);
        const nx = dist > 0.01 ? dx / dist : Math.cos(v.psi), ny = dist > 0.01 ? dy / dist : Math.sin(v.psi);
        a.state = 'flying'; a.t = race.time;
        a.vz = 7; a.z = 0.05;
        a.vx = nx * 3 + w[0] * 0.45; a.vy = ny * 3 + w[1] * 0.45;
        if (!(v.car.quirk === 'heavy' && SMALL_ANIMALS[a.kind])) scale(v, 0.92);
        let points = 0;
        if (!actor.ghost) {
            if (actor.tally) actor.tally.animalHits++;
            points = 150;
            AD.addStylePoints(actor, points, false);
        }
        AD.emit(race, 'animal_hit', { car: actor.index, x: a.x, y: a.y, kind: a.kind, feature: a.id, points: points });
        if (actor.ghost) return;
        const key = a.herd + ':' + actor.index;
        const list = (fs.herdHits[key] || (fs.herdHits[key] = [])).filter(h => race.time - h < 4);
        list.push(race.time);
        fs.herdHits[key] = list;
        if (list.length >= 3) {
            fs.herdHits[key] = [];
            AD.addStylePoints(actor, 1000, false);
            AD.emit(race, 'strike', { car: actor.index, count: list.length, x: a.x, y: a.y });
        }
    }

    AD.updateActorFeatures = function (race, actor, fs, dt) {
        if (!fs) return;
        advance(race, fs);
        const v = actor.v;
        if (v.out) return;
        if (v.airborne && v.onRamp >= 0) v.onRamp = -1;
        if (v.onRamp >= 0) {
            const ramp = fs.ramps[v.onRamp];
            if (ramp) rampStep(race, actor, ramp); else clearRamp(v);
        }
        const track = race.track;
        const sample = AD.sampleTrack(track, v.x, v.y);
        const centre = bucketOf(track, sample.s);
        for (let n = -1; n <= 1; n++) {
            const list = fs.buckets[(centre + n + BUCKETS) % BUCKETS];
            for (let i = 0; i < list.length; i++) {
                const f = list[i];
                switch (f.type || f.kind) {
                    case 'ramp': if (v.onRamp !== f.id) rampStep(race, actor, f); break;
                    case 'pad': padStep(race, actor, f, fs); break;
                    case 'crate': crateStep(race, actor, f); break;
                    case 'animal': animalStep(race, actor, fs, f); break;
                    case 'destructible': smashStep(race, actor, f); break;
                }
            }
        }
    };

    AD.updateFeatures = function (race, fs, dt) {
        if (fs) advance(race, fs);
    };

    /* Lateral nudge for the AI in metres (positive = left): towards pads, crates and
       ramps ahead, away from standing obstacles the car would pay for. */
    AD.featureLaneBias = function (race, car, sample) {
        const fs = race.features;
        const v = car.v;
        if (!fs || !sample || v.airborne) return 0;
        const length = race.track.length;
        const first = bucketOf(race.track, sample.s);
        let pull = 0, pullAt = Infinity, push = 0;
        const free = v.car.quirk === 'sturdy' || v.car.quirk === 'heavy';
        for (let n = 0; n <= 3; n++) {
            const list = fs.buckets[(first + n) % BUCKETS];
            for (const f of list) {
                const ds = ((f.s - sample.s) % length + length) % length;
                if (ds < 8 || ds > 80) continue;
                const gap = f.offset === undefined ? 0 : f.offset - sample.offset;
                if (f.kind === 'pad' && ds < pullAt) { pullAt = ds; pull = clamp(gap, -3.5, 3.5); }
                else if (f.kind === 'crate' && race.weapons && !car.weapon && race.time - f.taken >= CRATE.respawn && ds < pullAt) {
                    pullAt = ds; pull = clamp(gap, -3.5, 3.5) * 0.8;
                } else if (f.kind === 'ramp' && v.vx > 20 && ds < pullAt) { pullAt = ds; pull = clamp(gap, -3.5, 3.5) * 0.5; }
                else if (!free && f.state === 'idle' && f.r !== undefined && ds < 45 && Math.abs(gap) < 2.8) {
                    push += (gap >= 0 ? -1 : 1) * 1.2;
                }
            }
        }
        return clamp(pull + push, -4, 4);
    };

    AD.validateFeatures = function (track, fs) {
        const reasons = [];
        const length = track.length;
        const all = [].concat(fs.crates, fs.ramps, fs.pads, fs.destructibles, fs.crossings, fs.setpieces);
        for (const f of all) {
            if (!isFinite(f.x) || !isFinite(f.y)) reasons.push(f.kind + ' ' + f.id + ' has no position');
            if (f.kind !== 'ufo' && wrapDist(f.s, 0, length) < 40) reasons.push(f.kind + ' ' + f.id + ' within 40 m of the line');
        }
        for (const r of fs.ramps) {
            const end = Math.min(r.s + 150, length - 1);
            for (let i = 0; i < track.samples.length; i++) {
                const smp = track.samples[i];
                if (smp.s >= r.s && smp.s <= end && track.curvature[i] >= 0.01) {
                    reasons.push('ramp ' + r.id + ' has a bend inside its runout'); break;
                }
            }
        }
        for (const c of fs.crates) {
            const smp = AD.sampleTrack(track, c.x, c.y);
            if (Math.abs(smp.offset) >= smp.halfWidth) reasons.push('crate ' + c.id + ' outside the corridor');
        }
        return { ok: reasons.length === 0, reasons: reasons };
    };
})();
