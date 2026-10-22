(function () {
    'use strict';

    // Orbit choreography (global AuraOrbitChoreo). DOM-free, WebGL-free and free of Math.random and clocks, so the Node
    // regression suite drives it and the scene, the window bridge and the particle layer all agree on the same sky:
    //  - createRng: the mulberry32 generator behind every schedule,
    //  - composition: where the planet, the station and the sun sit on screen per aspect ratio (0..1 from the top-left),
    //  - cameraPose: the slow drift, the breathing roll and the pointer parallax of the camera,
    //  - shuttleState / shuttleOffset: the 96 s docking cycle of the shuttle and its position relative to DockPort
    //    (station units; the model's nose points along +Z, so `yaw` is the three.js rotation.y of the heading and is
    //    returned wrapped to -PI..PI: 0 faces +Z, PI/2 faces +X),
    //  - eventsBetween: comets, shooting stars and lightning as deterministic Poisson-like schedules, queried by time
    //    window (an event belongs to the window that contains its `start`; events are frozen and shared between calls),
    //  - spring / tiltFromVelocity: the damped spring and the drag-velocity to window-tilt mapping of the bridge.
    // window.AuraOrbitChoreo = { createRng, composition, cameraPose, shuttleState, shuttleOffset, eventsBetween,
    //     spring, tiltFromVelocity, constants }.

    const TWO_PI = Math.PI * 2;
    const HALF_PI = Math.PI / 2;

    // Shuttle cycle: phase boundaries in seconds within one 96 s cycle.
    const SHUTTLE_CYCLE = 96;
    const SHUTTLE_PHASES = Object.freeze([
        Object.freeze({ name: 'docked', start: 0, end: 30 }),
        Object.freeze({ name: 'undock', start: 30, end: 38 }),
        Object.freeze({ name: 'cruise', start: 38, end: 78 }),
        Object.freeze({ name: 'approach', start: 78, end: 90 }),
        Object.freeze({ name: 'dock', start: 90, end: 96 })
    ]);
    // Station units. The shuttle backs out of DockPort straight along -Y to the clearance point, flies the loop from
    // there and comes back to the same point.
    const DOCK_CLEARANCE = 0.6;
    const LOOP_RX = 2.2;
    const LOOP_RZ = 1.4;
    const LOOP_DIP = 0.3;
    const LOOP_WAVE = 0.25;

    // Events are generated per 60 s bucket so that any time window sees the same sky.
    const EVENT_BUCKET = 60;
    const MAX_EVENT_BUCKETS = 4000;
    const SKY_RADIUS = 2400;
    // Comets start inside a cone around -Z (the way the scene camera looks); the scene may rotate the sky.
    const COMET_CONE = 0.95;

    const LANDSCAPE = Object.freeze({
        planet: Object.freeze({ x: 0.70, y: 0.72, radius: 0.46 }),
        station: Object.freeze({ x: 0.24, y: 0.30 }),
        sun: Object.freeze({ x: 0.06, y: 0.10 })
    });
    const PORTRAIT = Object.freeze({
        planet: Object.freeze({ x: 0.60, y: 0.78, radius: 0.40 }),
        station: Object.freeze({ x: 0.30, y: 0.26 }),
        sun: Object.freeze({ x: 0.10, y: 0.06 })
    });

    function clamp(value, min, max) { return Math.max(min, Math.min(max, value)); }
    function lerp(a, b, t) { return a + (b - a) * t; }
    function finite(value, fallback) { const n = Number(value); return Number.isFinite(n) ? n : fallback; }
    function smooth(t) { const k = clamp(t, 0, 1); return k * k * (3 - 2 * k); }
    function smoothstep(a, b, x) { return smooth((x - a) / (b - a)); }
    // The wrapped angle in -PI..PI.
    function wrapAngle(angle) { return Math.atan2(Math.sin(angle), Math.cos(angle)); }

    // -------------------------------------------------------------------------------------------------
    // Random numbers

    function createRng(seed) {
        let a = finite(seed, 0) >>> 0;
        return function () {
            a = (a + 0x6D2B79F5) >>> 0;
            let t = a;
            t = Math.imul(t ^ (t >>> 15), t | 1);
            t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
            return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
        };
    }

    // -------------------------------------------------------------------------------------------------
    // Composition and camera

    // Landscape from aspect 1.2, portrait up to 0.8, a linear blend in between; the anchors never extrapolate.
    function composition(width, height) {
        const w = finite(width, 0), h = finite(height, 0);
        const aspect = w > 0 && h > 0 ? w / h : 16 / 9;
        const k = clamp((aspect - 0.8) / 0.4, 0, 1);
        return {
            aspect,
            planet: {
                x: lerp(PORTRAIT.planet.x, LANDSCAPE.planet.x, k),
                y: lerp(PORTRAIT.planet.y, LANDSCAPE.planet.y, k),
                radius: lerp(PORTRAIT.planet.radius, LANDSCAPE.planet.radius, k)
            },
            station: { x: lerp(PORTRAIT.station.x, LANDSCAPE.station.x, k), y: lerp(PORTRAIT.station.y, LANDSCAPE.station.y, k) },
            sun: { x: lerp(PORTRAIT.sun.x, LANDSCAPE.sun.x, k), y: lerp(PORTRAIT.sun.y, LANDSCAPE.sun.y, k) }
        };
    }

    // Slow drift (180 s yaw, 240 s pitch), a breathing roll (11 s) and the pointer parallax (pointer in -1..1).
    // Without motion the pose is the one at t = 0 with the pointer centred, so a still frame never depends on the clock.
    function cameraPose(t, pointer, motion) {
        const live = motion !== false;
        const time = live ? finite(t, 0) : 0;
        const px = live && pointer ? clamp(finite(pointer.x, 0), -1, 1) : 0;
        const py = live && pointer ? clamp(finite(pointer.y, 0), -1, 1) : 0;
        return {
            yaw: 0.06 * Math.sin(TWO_PI * time / 180) + 0.035 * px,
            pitch: 0.025 * Math.sin(TWO_PI * time / 240 + 1.3) + 0.022 * py,
            roll: 0.006 * Math.sin(TWO_PI * time / 11)
        };
    }

    // -------------------------------------------------------------------------------------------------
    // Shuttle

    // docked 0-30 s, undock 30-38, cruise 38-78, approach 78-90, dock 90-96; progress runs 0..1 inside the phase.
    function shuttleState(t) {
        const time = finite(t, 0);
        const cycle = Math.floor(time / SHUTTLE_CYCLE);
        const local = time - cycle * SHUTTLE_CYCLE;
        let phase = SHUTTLE_PHASES[SHUTTLE_PHASES.length - 1];
        for (let i = 0; i < SHUTTLE_PHASES.length; i++) {
            if (local < SHUTTLE_PHASES[i].end) { phase = SHUTTLE_PHASES[i]; break; }
        }
        return { phase: phase.name, progress: clamp((local - phase.start) / (phase.end - phase.start), 0, 1), cycle };
    }

    // The closed loop: an ellipse of 2.2 x 1.4 through the clearance point, dipping to about y = -0.9 with a gentle
    // vertical wave. Its parameter is eased so the shuttle leaves and returns at rest, like the undock and dock moves.
    function cruiseOffset(progress) {
        const q = smooth(progress);
        const theta = TWO_PI * q;
        const dip = Math.sin(Math.PI * q);
        return {
            x: LOOP_RX * Math.sin(theta),
            y: -DOCK_CLEARANCE - LOOP_DIP * dip + LOOP_WAVE * Math.sin(2 * theta) * dip,
            z: LOOP_RZ * (1 - Math.cos(theta)),
            yaw: wrapAngle(Math.atan2(LOOP_RX * Math.cos(theta), LOOP_RZ * Math.sin(theta)))
        };
    }

    // The offset of the shuttle from DockPort. Docked it is exactly (0, 0, 0). Undock backs out along -Y and turns the
    // nose to the start of the loop in its second half; dock is the same move in reverse (and approach turns the nose
    // back while it station-keeps at the clearance point with a small settling sway that dies out at both ends).
    function shuttleOffset(state) {
        const phase = state && state.phase;
        const p = clamp(finite(state && state.progress, 0), 0, 1);
        if (phase === 'undock') {
            return { x: 0, y: 0 - DOCK_CLEARANCE * smooth(p), z: 0, yaw: HALF_PI * smoothstep(0.4, 1, p) };
        }
        if (phase === 'cruise') return cruiseOffset(p);
        if (phase === 'approach') {
            const settle = Math.sin(Math.PI * p);
            return {
                x: 0.2 * Math.sin(TWO_PI * p) * settle * settle,
                y: -DOCK_CLEARANCE,
                z: 0.1 * settle * settle,
                yaw: HALF_PI * (1 - smoothstep(0, 0.7, p))
            };
        }
        if (phase === 'dock') return { x: 0, y: DOCK_CLEARANCE * smooth(p) - DOCK_CLEARANCE, z: 0, yaw: 0 };
        return { x: 0, y: 0, z: 0, yaw: 0 };
    }

    // -------------------------------------------------------------------------------------------------
    // Event schedules

    // Hash of (seed, kind, bucket) into one 32-bit seed.
    function bucketSeed(seed, kindId, bucket) {
        let h = Math.imul(finite(seed, 0) | 0, 0x9E3779B1) ^ Math.imul(kindId, 0x85EBCA6B);
        h = Math.imul(h ^ (h >>> 16), 0x7FEB352D) ^ Math.imul(bucket | 0, 0xC2B2AE35);
        h = Math.imul(h ^ (h >>> 15), 0x846CA68B);
        return (h ^ (h >>> 16)) >>> 0;
    }

    // A random direction inside the cone around -Z.
    function coneDirection(rng) {
        const cosMax = Math.cos(COMET_CONE);
        const z = -lerp(cosMax, 1, rng());
        const s = Math.sqrt(Math.max(0, 1 - z * z));
        const phi = TWO_PI * rng();
        return [s * Math.cos(phi), s * Math.sin(phi), z];
    }

    // A comet crosses the far sky on a great-circle arc of 20-43 degrees in a random direction.
    function makeComet(rng, start) {
        const a = coneDirection(rng);
        // A second random direction supplies the heading: the part of it that is perpendicular to the start.
        const b = coneDirection(rng);
        const dot = a[0] * b[0] + a[1] * b[1] + a[2] * b[2];
        let px = b[0] - a[0] * dot, py = b[1] - a[1] * dot, pz = b[2] - a[2] * dot;
        let length = Math.hypot(px, py, pz);
        if (length < 1e-6) { px = -a[1]; py = a[0]; pz = 0; length = Math.hypot(px, py) || 1; }
        px /= length; py /= length; pz /= length;
        const arc = 0.35 + 0.4 * rng();
        const c = Math.cos(arc), s = Math.sin(arc);
        const duration = 6 + 4 * rng();
        return Object.freeze({
            start, duration,
            from: Object.freeze({ x: a[0] * SKY_RADIUS, y: a[1] * SKY_RADIUS, z: a[2] * SKY_RADIUS }),
            to: Object.freeze({
                x: (a[0] * c + px * s) * SKY_RADIUS,
                y: (a[1] * c + py * s) * SKY_RADIUS,
                z: (a[2] * c + pz * s) * SKY_RADIUS
            })
        });
    }

    // A shooting star in screen space: x0, y0 are 0..1 from the top-left, angle is the heading in radians (y points
    // down, so it always falls), length is a fraction of min(width, height).
    function makeMeteor(rng, start) {
        const duration = 0.6 + 0.5 * rng();
        const x0 = 0.08 + 0.9 * rng();
        const y0 = 0.02 + 0.4 * rng();
        const fall = 0.35 + 0.6 * rng();
        const angle = rng() < 0.5 ? fall : Math.PI - fall;
        return Object.freeze({ start, duration, x0, y0, angle, length: 0.18 + 0.27 * rng() });
    }

    // A flash in the clouds of the planet: latitude and longitude on the sphere and a strength.
    function makeLightning(rng, start) {
        return Object.freeze({ start, lat: -0.6 + 1.2 * rng(), lon: TWO_PI * rng(), strength: 0.4 + 0.6 * rng() });
    }

    const EVENT_KINDS = Object.freeze({
        comet: Object.freeze({ id: 1, mean: 38, make: makeComet }),
        meteor: Object.freeze({ id: 2, mean: 9, make: makeMeteor }),
        lightning: Object.freeze({ id: 3, mean: 2.6, make: makeLightning })
    });

    // A frame-by-frame query hits the same bucket over and over, so the last few buckets are kept.
    const bucketCache = new Map();
    const BUCKET_CACHE_SIZE = 24;

    // The events with `start` inside one bucket. The walk starts one random interval before the bucket and keeps the
    // events that land in it; the interval is mean * (0.3 + 0.7 * Exp(1)), which has the requested mean but never
    // bunches two events closer than 30 % of it.
    function bucketEvents(seed, kind, bucket) {
        const key = kind.id + ':' + finite(seed, 0) + ':' + bucket;
        let list = bucketCache.get(key);
        if (list) return list;
        const rng = createRng(bucketSeed(seed, kind.id, bucket));
        const lo = bucket * EVENT_BUCKET, hi = lo + EVENT_BUCKET;
        list = [];
        let at = lo - rng() * kind.mean;
        for (;;) {
            at += kind.mean * (0.3 + 0.7 * -Math.log(1 - rng()));
            if (at >= hi) break;
            if (at >= lo) list.push(kind.make(rng, at));
        }
        if (bucketCache.size >= BUCKET_CACHE_SIZE) bucketCache.delete(bucketCache.keys().next().value);
        bucketCache.set(key, list);
        return list;
    }

    // Deterministic events of one kind ('comet', 'meteor' or 'lightning') whose start lies in [t0, t1), in order.
    function eventsBetween(seed, kind, t0, t1) {
        const spec = EVENT_KINDS[kind];
        const from = finite(t0, 0), to = finite(t1, 0);
        const result = [];
        if (!spec || !(to > from)) return result;
        const first = Math.floor(from / EVENT_BUCKET);
        const last = Math.min(Math.floor(to / EVENT_BUCKET), first + MAX_EVENT_BUCKETS);
        for (let bucket = first; bucket <= last; bucket++) {
            const list = bucketEvents(seed, spec, bucket);
            for (let i = 0; i < list.length; i++) {
                if (list[i].start >= from && list[i].start < to) result.push(list[i]);
            }
        }
        return result;
    }

    // -------------------------------------------------------------------------------------------------
    // Spring and drag tilt

    // Advances a damped spring by dt seconds (semi-implicit Euler in substeps of at most 1/120 s, at most one second of
    // simulated time per call, so a long frame stays stable) and returns the new { value, velocity }.
    function spring(state, target, dt, stiffness = 170, damping = 18) {
        let value = finite(state && state.value, 0), velocity = finite(state && state.velocity, 0);
        const goal = finite(target, 0);
        const time = clamp(finite(dt, 0), 0, 1);
        if (time <= 0) return { value, velocity };
        const steps = Math.max(1, Math.ceil(time * 120 - 1e-9));
        const h = time / steps;
        for (let i = 0; i < steps; i++) {
            velocity += (stiffness * (goal - value) - damping * velocity) * h;
            value += velocity * h;
        }
        return { value, velocity };
    }

    // Window tilt from the drag velocity (px/s): rotateY leans the leading edge away, rotateX follows the vertical
    // motion, both saturate at maxDeg degrees, and the window lifts up to 18 px with speed.
    function tiltFromVelocity(vx, vy, maxDeg = 9) {
        const x = finite(vx, 0), y = finite(vy, 0);
        return {
            x: -maxDeg * Math.tanh(y / 1400),
            y: maxDeg * Math.tanh(x / 1400),
            lift: 18 * Math.tanh(Math.hypot(x, y) / 1800)
        };
    }

    window.AuraOrbitChoreo = Object.freeze({
        createRng, composition, cameraPose, shuttleState, shuttleOffset, eventsBetween, spring, tiltFromVelocity,
        constants: Object.freeze({ SHUTTLE_CYCLE, SHUTTLE_PHASES, EVENT_BUCKET, SKY_RADIUS })
    });
}());
