import { createRng } from '../../chat/ocean-life-boids.js';

// Pirate Bay sea life paths and deterministic layout (pure ESM: no three.js, no DOM, deterministic seeded RNG).
// World units are metres, Y up, SEA = -1.15. Creature GLB frame: +Z nose, +Y up, +X animal left.
export const SEA = -1.15;
export const TAU = Math.PI * 2;
export const SHORE_ROCK_MARGIN = 0.55;
export const KEEP_FROM_SHORE = 1.6;
export const BOIDS_STEP = 0.05;

export const SCALE = Object.freeze({
    fish: 2.6,
    shark: 1.05,
    hammer: 1.08,
    turtle: 1.25,
    manta: 1.1,
    whale: 0.72,
    jelly: 1.4
});

export const TIER_COUNTS = Object.freeze({
    low: Object.freeze({ sharks: 0, hammers: 0, turtles: 0, whale: 0, mantas: 0, jellies: 0, schools: 0 }),
    medium: Object.freeze({ sharks: 7, hammers: 3, turtles: 3, whale: 1, mantas: 2, jellies: 4, schools: 6 }),
    high: Object.freeze({ sharks: 10, hammers: 5, turtles: 4, whale: 1, mantas: 3, jellies: 6, schools: 11 })
});

export const LEAP = Object.freeze({
    duration: 0.85,
    minGap: 3.5,
    maxGap: 7.5,
    apex: Object.freeze([0.45, 0.85]),
    speed: Object.freeze([2.2, 3.0]),
    ring: 1.1
});

export const SURFACING = Object.freeze({
    turtle: Object.freeze({ rise: 1.5, hold: 4.0, fall: 1.5, depth: 0.08 }),
    whale: Object.freeze({ rise: 3.5, hold: 3.0, fall: 3.2, depth: 0.52, cruise: 2.6 })
});

export const BLOW = Object.freeze({
    count: 5,
    first: 0.7,
    gap: 0.32,
    life: 2.2,
    rise: 2.6,
    length: 12 * SCALE.whale
});

function cumulative(seed, first, lo, hi, count) {
    const rng = createRng(seed);
    const out = [];
    let t = first;
    for (let i = 0; i < count; i++) {
        t += lo + (hi - lo) * rng();
        out.push(t);
    }
    return Object.freeze(out);
}

export const LEAP_STARTS = cumulative(0x7a1ea9, 2.5, LEAP.minGap, LEAP.maxGap, 2400);
export const WHALE_EVENTS = cumulative(0x7a0a1e, 14, 34, 52, 400);
export const TURTLE_EVENTS = cumulative(0x7a7081, 8, 22, 38, 700);

export function courseSeed(course) {
    const id = String(course?.id || ('pirate-' + (course?.number || 1)));
    let h = 2166136261 >>> 0;
    for (let i = 0; i < id.length; i++) {
        h ^= id.charCodeAt(i);
        h = Math.imul(h, 16777619);
    }
    return (h >>> 0) || 0x5ea101;
}

export function normalizeIsland(island) {
    const cx = Number(island?.center?.[0]) || 0;
    const cz = Number(island?.center?.[1]) || 0;
    const hx = Math.max(4, Number(island?.half?.[0]) || 9.5);
    const hz = Math.max(4, Number(island?.half?.[1]) || 9.5);
    const power = Math.max(1.5, Number(island?.power) || 3);
    const wobble = Math.max(0, Math.min(0.15, Number(island?.wobble) || 0.03));
    return Object.freeze({
        cx,
        cz,
        hx,
        hz,
        power,
        wobble,
        solidHx: hx * (1 + wobble) + 0.35,
        solidHz: hz * (1 + wobble) + 0.35
    });
}

export function islandRadius(isle, theta) {
    const c = Math.abs(Math.cos(theta)) / isle.hx;
    const s = Math.abs(Math.sin(theta)) / isle.hz;
    const sum = Math.pow(c, isle.power) + Math.pow(s, isle.power);
    return sum > 1e-12 ? Math.pow(sum, -1 / isle.power) : Math.min(isle.hx, isle.hz);
}

export function shoreRadius(isle, theta) {
    return islandRadius(isle, theta) * (1 + isle.wobble) + SHORE_ROCK_MARGIN;
}

export function shoreClearance(isle, x, z) {
    const dx = x - isle.cx;
    const dz = z - isle.cz;
    return Math.hypot(dx, dz) - shoreRadius(isle, Math.atan2(dz, dx));
}

const HABITAT_ROSTER = Object.freeze({
    medium: Object.freeze([
        { species: 'fish_sardine', count: 18 },
        { species: 'fish_tang', count: 8 },
        { species: 'fish_clown', count: 6 },
        { species: 'fish_sardine', count: 18 },
        { species: 'fish_tang', count: 8 },
        { species: 'fish_butterfly', count: 6 }
    ]),
    high: Object.freeze([
        { species: 'fish_sardine', count: 24 },
        { species: 'fish_tang', count: 10 },
        { species: 'fish_clown', count: 8 },
        { species: 'fish_sardine', count: 24 },
        { species: 'fish_butterfly', count: 8 },
        { species: 'fish_tang', count: 10 },
        { species: 'fish_sardine', count: 24 },
        { species: 'fish_clown', count: 8 },
        { species: 'fish_tang', count: 10 },
        { species: 'fish_sardine', count: 24 },
        { species: 'fish_butterfly', count: 8 }
    ])
});

function boxMinClearance(isle, x0, x1, z0, z1) {
    const xm = (x0 + x1) * 0.5;
    const zm = (z0 + z1) * 0.5;
    const pts = [
        [x0, z0], [x1, z0], [x0, z1], [x1, z1],
        [xm, z0], [xm, z1], [x0, zm], [x1, zm]
    ];
    let min = Infinity;
    for (let i = 0; i < pts.length; i++) {
        const d = shoreClearance(isle, pts[i][0], pts[i][1]);
        if (d < min) min = d;
    }
    return min;
}

export function buildCourseLayout(course, tier = 'medium') {
    const isle = normalizeIsland(course?.island);
    const counts = TIER_COUNTS[tier] || TIER_COUNTS.medium;
    if (counts.sharks === 0) {
        return Object.freeze({
            isle,
            tier: 'low',
            loops: Object.freeze({ shark: [], hammer: [], turtle: [], manta: [], whale: [] }),
            jellies: Object.freeze([]),
            habitats: Object.freeze([])
        });
    }
    const rng = createRng(courseSeed(course));

    const sharks = [];
    for (let i = 0; i < counts.sharks; i++) {
        const inner = i % 2 === 0;
        const offset = inner
            ? 2.6 + (i / counts.sharks) * 2.0 + rng() * 0.4
            : 4.9 + (i / counts.sharks) * 3.8 + rng() * 0.6;
        const size = SCALE.shark * (0.86 + rng() * 0.36);
        sharks.push(Object.freeze({
            offset,
            wobAmp: inner ? 0.12 + rng() * 0.08 : 0.14 + rng() * 0.10,
            wobFreq: inner ? 2 + (i % 2) : 1 + (i % 3),
            speed: 1.35 + rng() * 0.65,
            phase: ((i / counts.sharks) * TAU + (rng() - 0.5) * 0.4 + TAU) % TAU,
            dir: inner ? 1 : -1,
            depth: Object.freeze(inner ? [0.15, 0.80] : [0.18, 1.25]),
            period: 20 + rng() * 22,
            ph: (i * 1.7 + rng() * 0.7) % TAU,
            size,
            finTop: 0.3702 * size
        }));
    }

    const hammers = [];
    for (let i = 0; i < counts.hammers; i++) {
        const size = SCALE.hammer * (0.90 + rng() * 0.26);
        hammers.push(Object.freeze({
            offset: 5.6 + (i / counts.hammers) * 6.5 + rng() * 1.0,
            wobAmp: 0.10 + rng() * 0.12,
            wobFreq: 2 + (i % 2),
            speed: 1.40 + rng() * 0.55,
            phase: (((i + 0.35) / counts.hammers) * TAU + (rng() - 0.5) * 0.35 + TAU) % TAU,
            dir: i % 2 === 0 ? -1 : 1,
            depth: Object.freeze([0.30, 1.45]),
            period: 26 + rng() * 22,
            ph: (i * 2.1 + rng() * 0.7) % TAU,
            size,
            finTop: 0.603 * size
        }));
    }

    const turtles = [];
    for (let i = 0; i < counts.turtles; i++) {
        turtles.push(Object.freeze({
            offset: 2.7 + (i / counts.turtles) * 3.3 + rng() * 0.5,
            wobAmp: 0.10 + rng() * 0.10,
            wobFreq: 3,
            speed: 0.38 + rng() * 0.22,
            phase: (((i + 0.15) / counts.turtles) * TAU + (rng() - 0.5) * 0.35 + TAU) % TAU,
            dir: i % 2 === 0 ? 1 : -1,
            depth: Object.freeze([0.45, 0.95]),
            period: 24 + rng() * 14,
            ph: (i * 1.5) % TAU,
            size: SCALE.turtle * (0.90 + rng() * 0.28),
            finTop: 0
        }));
    }

    const mantas = [];
    for (let i = 0; i < counts.mantas; i++) {
        mantas.push(Object.freeze({
            offset: 5.5 + (i / counts.mantas) * 5.2 + rng() * 0.8,
            wobAmp: 0.10 + rng() * 0.10,
            wobFreq: 2,
            speed: 1.10 + rng() * 0.45,
            phase: (((i + 0.5) / counts.mantas) * TAU + (rng() - 0.5) * 0.35 + TAU) % TAU,
            dir: i % 2 === 0 ? 1 : -1,
            depth: Object.freeze([0.55, 1.35]),
            period: 28 + rng() * 16,
            ph: (i * 2.2) % TAU,
            size: SCALE.manta * (0.90 + rng() * 0.24),
            finTop: 0
        }));
    }

    const whales = [];
    for (let i = 0; i < counts.whale; i++) {
        whales.push(Object.freeze({
            offset: 16.5,
            wobAmp: 0.06,
            wobFreq: 2,
            speed: 1.55,
            phase: 0.6 + rng() * 1.4,
            dir: 1,
            depth: Object.freeze([2.6, 2.6]),
            period: 90,
            ph: 0,
            size: SCALE.whale,
            finTop: 0
        }));
    }

    const jellies = [];
    for (let i = 0; i < counts.jellies; i++) {
        const reach = 0.7 + rng() * 0.5;
        const offset = Math.max(3.6 + rng() * 3.8, KEEP_FROM_SHORE + reach + 0.4);
        jellies.push(Object.freeze({
            angle: ((i + 0.25) / counts.jellies) * TAU + (rng() - 0.5) * 0.3,
            offset,
            reach,
            phase: rng() * TAU,
            rate: 0.04 + 0.04 * rng(),
            pulse: rng(),
            freq: 0.32 + 0.18 * rng(),
            size: SCALE.jelly * (0.85 + 0.35 * rng()),
            tilt: rng() * TAU
        }));
    }

    const roster = HABITAT_ROSTER[tier === 'high' ? 'high' : 'medium'];
    const habitats = [];
    for (let k = 0; k < roster.length; k++) {
        const entry = roster[k];
        const sardine = entry.species === 'fish_sardine';
        const halfSize = sardine ? 2.2 : 1.25;
        const radialOff = sardine ? 4.8 + rng() * 1.6 : 3.2 + rng() * 1.1;
        const theta = (k / roster.length) * TAU + (rng() - 0.5) * 0.25;
        const c = Math.cos(theta);
        const s = Math.sin(theta);
        const rBase = shoreRadius(isle, theta) + radialOff;
        let hx0 = isle.cx + c * rBase;
        let hz0 = isle.cz + s * rBase;
        for (let iter = 0; iter < 8; iter++) {
            const minClear = boxMinClearance(isle, hx0 - halfSize, hx0 + halfSize, hz0 - halfSize, hz0 + halfSize);
            if (minClear >= KEEP_FROM_SHORE) break;
            const push = (KEEP_FROM_SHORE - minClear) + 0.25;
            hx0 += c * push;
            hz0 += s * push;
        }
        habitats.push(Object.freeze({
            species: entry.species,
            count: entry.count,
            x: Object.freeze([hx0 - halfSize, hx0 + halfSize]),
            z: Object.freeze([hz0 - halfSize, hz0 + halfSize]),
            depth: Object.freeze([0.22, 1.15])
        }));
    }

    return Object.freeze({
        isle,
        tier: tier === 'high' ? 'high' : 'medium',
        loops: Object.freeze({
            shark: Object.freeze(sharks),
            hammer: Object.freeze(hammers),
            turtle: Object.freeze(turtles),
            manta: Object.freeze(mantas),
            whale: Object.freeze(whales)
        }),
        jellies: Object.freeze(jellies),
        habitats: Object.freeze(habitats)
    });
}

export function firstAtOrAfter(list, t) {
    let lo = 0;
    let hi = list.length;
    while (lo < hi) {
        const mid = (lo + hi) >> 1;
        if (list[mid] < t) lo = mid + 1;
        else hi = mid;
    }
    return lo;
}

export function eventAt(list, t, before, after, out = { k: 0, u: 0 }) {
    const k = firstAtOrAfter(list, t - after);
    if (k >= list.length || list[k] - before > t) return null;
    out.k = k;
    out.u = t - list[k];
    return out;
}

export function leapParams(k, out = { apex: 0, speed: 0 }) {
    const rng = createRng((k + 1) * 2654435761);
    out.apex = LEAP.apex[0] + (LEAP.apex[1] - LEAP.apex[0]) * rng();
    out.speed = LEAP.speed[0] + (LEAP.speed[1] - LEAP.speed[0]) * rng();
    return out;
}

export function leapPose(apex, speed, dx, dz, s, out) {
    const dist = speed * LEAP.duration;
    const u = Math.min(1, Math.max(0, s));
    const slope = 4 * apex * (1 - 2 * u) / dist;
    const c = 1 / Math.hypot(1, slope);
    out.x = dx * dist * u;
    out.z = dz * dist * u;
    out.h = 4 * apex * u * (1 - u);
    out.fx = dx * c;
    out.fy = slope * c;
    out.fz = dz * c;
    return out;
}

export function loopPoint(isle, L, t, out) {
    const mean = (isle.hx + isle.hz) * 0.5 + L.offset;
    const th = L.phase + L.dir * L.speed * t / mean;
    const radial = L.offset * (1 + L.wobAmp * Math.sin(L.wobFreq * th + L.phase * 2));
    const r = shoreRadius(isle, th) + Math.max(KEEP_FROM_SHORE, radial);
    out.x = isle.cx + Math.cos(th) * r;
    out.z = isle.cz + Math.sin(th) * r;
    return out;
}

const smooth = (a, b, x) => {
    const u = Math.min(1, Math.max(0, (x - a) / (b - a)));
    return u * u * (3 - 2 * u);
};

const hit = { k: 0, u: 0 };

export function depthAt(kind, loopEntry, index, t, count = 1) {
    const base = (loopEntry.depth[0] + loopEntry.depth[1]) * 0.5 +
        (loopEntry.depth[1] - loopEntry.depth[0]) * 0.5 * Math.sin(TAU * t / loopEntry.period + loopEntry.ph);
    if (kind === 'turtle') {
        const S = SURFACING.turtle;
        const e = eventAt(TURTLE_EVENTS, t, S.rise, S.hold + S.fall, hit);
        if (!e || e.k % Math.max(1, count) !== index) return base;
        const up = smooth(-S.rise, 0, e.u) * (1 - smooth(S.hold, S.hold + S.fall, e.u));
        return base + (S.depth - base) * up;
    }
    if (kind === 'whale') {
        const S = SURFACING.whale;
        const e = eventAt(WHALE_EVENTS, t, S.rise, S.hold + S.fall, hit);
        if (!e) return S.cruise;
        const up = smooth(-S.rise, 0, e.u) * (1 - smooth(S.hold, S.hold + S.fall, e.u));
        return S.cruise + (S.depth - S.cruise) * up;
    }
    return base;
}

export function seaPosition(isle, kind, loopEntry, index, t, count, out) {
    loopPoint(isle, loopEntry, t, out);
    out.y = SEA - depthAt(kind, loopEntry, index, t, count);
    return out;
}

const pa = { x: 0, y: 0, z: 0 };
const pb = { x: 0, y: 0, z: 0 };
const pc = { x: 0, y: 0, z: 0 };

export function seaPose(isle, kind, loopEntry, index, t, count, out) {
    seaPosition(isle, kind, loopEntry, index, t - 0.1, count, pa);
    seaPosition(isle, kind, loopEntry, index, t, count, pb);
    seaPosition(isle, kind, loopEntry, index, t + 0.1, count, pc);
    out.x = pb.x;
    out.y = pb.y;
    out.z = pb.z;
    const fx = pc.x - pa.x;
    const fy = pc.y - pa.y;
    const fz = pc.z - pa.z;
    const l = Math.hypot(fx, fy, fz) || 1;
    out.fx = fx / l;
    out.fy = fy / l;
    out.fz = fz / l;
    const turn = Math.atan2(pc.x - pb.x, pc.z - pb.z) - Math.atan2(pb.x - pa.x, pb.z - pa.z);
    const wrapped = Math.atan2(Math.sin(turn), Math.cos(turn));
    out.roll = Math.max(-0.35, Math.min(0.35, -wrapped * 5));
    return out;
}

const puffPose = { x: 0, y: 0, z: 0, fx: 0, fy: 0, fz: 1, roll: 0 };
const bhit = { k: 0, u: 0 };

export function blowPuff(isle, whaleLoop, t, i, out) {
    if (!whaleLoop) return -1;
    const e = eventAt(WHALE_EVENTS, t, 0, BLOW.first + BLOW.gap * BLOW.count + BLOW.life, bhit);
    if (!e) return -1;
    const elapsed = e.u - (BLOW.first + BLOW.gap * i);
    const age = elapsed / BLOW.life;
    if (age < 0 || age > 1) return -1;
    seaPose(isle, 'whale', whaleLoop, 0, t - elapsed, 1, puffPose);
    const h = Math.hypot(puffPose.fx, puffPose.fz) || 1;
    const r = createRng(e.k * 131 + i * 17 + 5);
    const sway = (r() - 0.5) * 1.1;
    out.x = puffPose.x + puffPose.fx / h * BLOW.length * 0.2 + sway * (0.3 + age);
    out.z = puffPose.z + puffPose.fz / h * BLOW.length * 0.2 + (r() - 0.5) * 1.1 * (0.3 + age);
    out.y = SEA + 0.15 + BLOW.rise * Math.pow(age, 0.75);
    out.size = 0.9 + 2.2 * age;
    return age;
}

export function jellyPosition(isle, J, t, out) {
    const baseR = Math.max(8, shoreRadius(isle, J.angle));
    const th = J.angle + (J.reach * 0.6 * Math.cos(t * J.rate * 0.8 + J.phase)) / baseR;
    const r = shoreRadius(isle, th) + Math.max(KEEP_FROM_SHORE, J.offset + J.reach * Math.sin(t * J.rate + J.phase));
    out.x = isle.cx + Math.cos(th) * r;
    out.z = isle.cz + Math.sin(th) * r;
    out.y = SEA - (0.75 + 0.40 * Math.sin(t * 0.09 + J.phase * 1.7));
    return out;
}

// JS twin of GLSL seaClip / remapBias.
export function remapBias(below, distance) {
    return (0.03 + 0.02 * Math.min(6, Math.max(0, 6 - below))) * (1 + 0.01 * distance);
}

export function seaRemap(cx, cy, cz, px, py, pz, out) {
    const below = SEA - py;
    if (below <= 0) {
        out.x = px;
        out.y = py;
        out.z = pz;
        return out;
    }
    const h = Math.max(cy - SEA, 0.02);
    const k = h / Math.max(cy - py, h);
    out.x = cx + (px - cx) * k;
    out.y = cy + (py - cy) * k;
    out.z = cz + (pz - cz) * k;
    const dx = cx - out.x;
    const dy = cy - out.y;
    const dz = cz - out.z;
    const len = Math.max(Math.hypot(dx, dy, dz), 0.001);
    const b = remapBias(below, len) / len;
    out.x += dx * b;
    out.y += dy * b;
    out.z += dz * b;
    return out;
}

// Column-major 4x4 matrix at a[o..o+15]: +Z along (fx,fy,fz), +Y up (banked by roll), scale s, position (px,py,pz).
export function place(a, o, px, py, pz, fx, fy, fz, s, roll) {
    const l = Math.hypot(fx, fy, fz) || 1;
    fx /= l;
    fy /= l;
    fz /= l;
    let xx = fz;
    let xz = -fx;
    const xl = Math.hypot(xx, xz);
    if (xl < 1e-4) {
        xx = 1;
        xz = 0;
    } else {
        xx /= xl;
        xz /= xl;
    }
    let yx = fy * xz;
    let yy = fz * xx - fx * xz;
    let yz = -fy * xx;
    let x0 = xx;
    let y0 = 0;
    let z0 = xz;
    if (roll) {
        const c = Math.cos(roll);
        const n = Math.sin(roll);
        x0 = xx * c + yx * n;
        y0 = yy * n;
        z0 = xz * c + yz * n;
        const nyx = yx * c - xx * n;
        const nyy = yy * c;
        const nyz = yz * c - xz * n;
        yx = nyx;
        yy = nyy;
        yz = nyz;
    }
    a[o] = x0 * s; a[o + 1] = y0 * s; a[o + 2] = z0 * s; a[o + 3] = 0;
    a[o + 4] = yx * s; a[o + 5] = yy * s; a[o + 6] = yz * s; a[o + 7] = 0;
    a[o + 8] = fx * s; a[o + 9] = fy * s; a[o + 10] = fz * s; a[o + 11] = 0;
    a[o + 12] = px; a[o + 13] = py; a[o + 14] = pz; a[o + 15] = 1;
}
