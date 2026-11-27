/* Aura Drift - spline tracks, chunk addressing, surface zones and validation. */
(function () {
    'use strict';
    const AD = window.AuraDriftCore;
    const clamp = AD.clamp;
    const GRID = 64;

    const BIOMES = {
        alpine: { ground: '#2f4034', groundAlt: '#3b5140', edge: '#6b7a5a', road: '#30343a', line: '#d8d2bc', accent: '#9fd3ff', night: false, weather: 'fog', base: 'asphalt', runoff: 'gravel' },
        desert: { ground: '#a8763f', groundAlt: '#c08c4d', edge: '#e0b878', road: '#3a3632', line: '#efe0b8', accent: '#ffcf6b', night: false, weather: 'heat', base: 'asphalt', runoff: 'sand' },
        harbour: { ground: '#202a38', groundAlt: '#293648', edge: '#3d4a5d', road: '#242a33', line: '#8bb6d8', accent: '#4ad8ff', night: true, weather: 'rain', base: 'wet', runoff: 'wet' },
        salt: { ground: '#d6d8cf', groundAlt: '#e6e8df', edge: '#f2f3ea', road: '#b9bcb2', line: '#f7f5e9', accent: '#ffffff', night: false, weather: 'glare', base: 'salt', runoff: 'salt' },
        forest: { ground: '#243b2b', groundAlt: '#2e4a34', edge: '#4a6040', road: '#343632', line: '#cdd4b3', accent: '#a8e08a', night: false, weather: 'mist', base: 'mud', runoff: 'gravel' },
        industry: { ground: '#34383c', groundAlt: '#3f4449', edge: '#596067', road: '#2c2f33', line: '#e3c15a', accent: '#ff9d4a', night: true, weather: 'sparks', base: 'asphalt', runoff: 'gravel' },
        glacier: { ground: '#3b4a5c', groundAlt: '#4a5c72', edge: '#7f9db8', road: '#333c48', line: '#cfe4f6', accent: '#8ce9ff', night: true, weather: 'snow', base: 'asphalt', runoff: 'snow' },
        metro: { ground: '#2a2d33', groundAlt: '#343841', edge: '#4d525c', road: '#262930', line: '#d8dde6', accent: '#ff5ec4', night: true, weather: 'rain', base: 'asphalt', runoff: 'wet' }
    };
    AD.BIOMES = BIOMES;

    /* Radial harmonics keep every generated loop simple (no self intersections)
       while giving each layout a genuinely different shape. `hills` adds smooth
       C2 elevation crests [{ at, length, height }]; steep crests (large height /
       length^2) launch fast cars airborne as the road drops away downhill. */
    AD.TRACKS = [
        {
            id: 'alpenpass', name: 'Alpenpass', seed: 1101, radius: 568, aspect: 0.72,
            harmonics: [{ k: 3, amp: 0.20, phase: 0.4 }, { k: 5, amp: 0.075, phase: 1.2 }],
            width: 22, laps: 2, biome: 'alpine',
            zones: [{ from: 0.24, to: 0.33, side: 1, type: 'gravel' }, { from: 0.68, to: 0.74, side: -1, type: 'wet' }],
            hills: [
                { at: 0.16, length: 155, height: 10.5 },
                { at: 0.34, length: 300, height: 15.5 },
                { at: 0.60, length: 150, height: 10.0 },
                { at: 0.78, length: 165, height: 11.0 }
            ]
        },
        {
            id: 'wuesten-canyon', name: 'Wüsten-Canyon', seed: 2202, radius: 766, aspect: 0.52,
            harmonics: [{ k: 2, amp: 0.10, phase: 0.1 }, { k: 7, amp: 0.035, phase: 0.8 }],
            width: 28, laps: 2, biome: 'desert',
            zones: [{ from: 0.40, to: 0.52, side: -1, type: 'sand' }],
            hills: [
                { at: 0.18, length: 160, height: 10.5 },
                { at: 0.40, length: 320, height: 15.0 },
                { at: 0.66, length: 155, height: 10.5 },
                { at: 0.84, length: 180, height: 10.0 }
            ]
        },
        {
            id: 'neon-hafen', name: 'Neon-Hafen', seed: 3303, radius: 510, aspect: 0.86,
            harmonics: [{ k: 4, amp: 0.16, phase: 0.9 }, { k: 9, amp: 0.055, phase: 0.2 }],
            width: 20, laps: 2, biome: 'harbour',
            zones: [{ from: 0.10, to: 0.55, side: 0, type: 'wet' }],
            hills: []
        },
        {
            id: 'salzsee', name: 'Salzsee', seed: 4404, radius: 847, aspect: 0.94,
            harmonics: [{ k: 2, amp: 0.055, phase: 1.7 }, { k: 6, amp: 0.03, phase: 0.5 }],
            width: 34, laps: 2, biome: 'salt', zones: [], hills: []
        },
        {
            id: 'wald-rallye', name: 'Wald-Rallye', seed: 5505, radius: 441, aspect: 0.78,
            harmonics: [{ k: 5, amp: 0.13, phase: 0.3 }, { k: 8, amp: 0.04, phase: 1.1 }],
            width: 22, laps: 2, biome: 'forest',
            zones: [{ from: 0.15, to: 0.62, side: 0, type: 'mud' }],
            hills: [
                { at: 0.14, length: 145, height: 9.5 },
                { at: 0.48, length: 220, height: 11.5 },
                { at: 0.75, length: 140, height: 9.5 },
                { at: 0.84, length: 135, height: 8.5 }
            ]
        },
        {
            id: 'industrie-ring', name: 'Industrie-Ring', seed: 6606, radius: 603, aspect: 0.58,
            harmonics: [{ k: 3, amp: 0.13, phase: 1.4 }, { k: 6, amp: 0.045, phase: 0.7 }],
            width: 25, laps: 2, biome: 'industry',
            zones: [{ from: 0.30, to: 0.38, side: 1, type: 'gravel' }],
            hills: [
                { at: 0.26, length: 160, height: 10.0 },
                { at: 0.72, length: 155, height: 10.0 }
            ]
        },
        {
            id: 'gletscherpass', name: 'Gletscherpass', seed: 7707, radius: 529, aspect: 0.66,
            harmonics: [{ k: 3, amp: 0.17, phase: 2.1 }, { k: 7, amp: 0.05, phase: 0.3 }],
            width: 24, laps: 2, biome: 'glacier',
            zones: [{ from: 0.18, to: 0.46, side: 0, type: 'snow' }, { from: 0.72, to: 0.80, side: -1, type: 'ice' }],
            hills: [
                { at: 0.24, length: 240, height: 14.0 },
                { at: 0.44, length: 150, height: 9.5 },
                { at: 0.68, length: 155, height: 10.5 },
                { at: 0.85, length: 210, height: 11.0 }
            ]
        },
        {
            id: 'metropolis', name: 'Metropolis-Ring', seed: 8808, radius: 648, aspect: 0.79,
            harmonics: [{ k: 2, amp: 0.12, phase: 0.6 }, { k: 5, amp: 0.05, phase: 1.9 }],
            width: 26, laps: 2, biome: 'metro',
            zones: [{ from: 0.44, to: 0.62, side: 0, type: 'wet' }],
            hills: []
        }
    ];

    AD.trackById = id => AD.TRACKS.find(t => t.id === id) || AD.TRACKS[0];

    AD.trackPoints = function (manifest) {
        const count = 28;
        const points = [];
        for (let i = 0; i < count; i++) {
            const a = i / count * Math.PI * 2;
            let r = 1;
            for (const h of manifest.harmonics || []) r += h.amp * Math.cos(h.k * a + h.phase);
            const radius = manifest.radius * clamp(r, 0.62, 1.38);
            points.push([Math.cos(a) * radius, Math.sin(a) * radius * manifest.aspect]);
        }
        return points;
    };

    function catmull(p0, p1, p2, p3, t) {
        const t2 = t * t, t3 = t2 * t;
        return 0.5 * ((2 * p1) + (-p0 + p2) * t +
            (2 * p0 - 5 * p1 + 4 * p2 - p3) * t2 +
            (-p0 + 3 * p1 - 3 * p2 + p3) * t3);
    }

    function cellKey(cx, cy) { return cx + ',' + cy; }

    function wrapDelta(ds, length) {
        let d = ds % length;
        if (d > length * 0.5) d -= length;
        if (d < -length * 0.5) d += length;
        return d;
    }

    /* Smooth C2 hill profile: raised-cosine bump with continuous elevation, slope,
       and vertical curvature (negative curveZ at the crest launches fast cars). */
    AD.evalHills = function (hills, length, s) {
        if (!hills || !hills.length || !(length > 0)) return { z: 0, slope: 0, curveZ: 0 };
        const ss = ((s % length) + length) % length;
        let z = 0, slope = 0, curveZ = 0;
        for (let i = 0; i < hills.length; i++) {
            const h = hills[i], span = h.length, H = h.height;
            const ds = wrapDelta(ss - h.center, length);
            if (Math.abs(ds) < span * 0.5) {
                const u = ds / span + 0.5;
                const sinPi = Math.sin(Math.PI * u);
                z += H * sinPi * sinPi;
                slope += (Math.PI * H / span) * Math.sin(2 * Math.PI * u);
                curveZ += (2 * Math.PI * Math.PI * H / (span * span)) * Math.cos(2 * Math.PI * u);
            }
        }
        return { z: z, slope: slope, curveZ: curveZ };
    };

    AD.roadProfileAt = function (track, s) {
        if (!track || !track.maxZ) return { z: 0, slope: 0, curveZ: 0 };
        return AD.evalHills(track.hills, track.length, s);
    };

    AD.roadHeight = function (track, x, y) {
        if (!track || !track.maxZ) return 0;
        const sm = AD.sampleTrack(track, x, y);
        return sm.z || 0;
    };

    function resolveHills(manifest, length, samples, curvature, trackSeed) {
        const list = [];
        const pushHill = (center, span, height) => {
            const c = ((center % length) + length) % length;
            if (c - span * 0.5 < 180 || c + span * 0.5 > length - 140) return false;
            for (let i = 0; i < list.length; i++) {
                if (Math.abs(wrapDelta(c - list[i].center, length)) < (span + list[i].length) * 0.5 + 24) return false;
            }
            const peakCurve = 2 * Math.PI * Math.PI * height / (span * span);
            list.push({
                center: c, length: span, height: height,
                s0: c - span * 0.5, s1: c + span * 0.5,
                steep: peakCurve > 0.0045
            });
            return true;
        };
        if (Array.isArray(manifest.hills) && manifest.hills.length) {
            for (let i = 0; i < manifest.hills.length; i++) {
                const h = manifest.hills[i];
                const center = h.center !== undefined ? h.center : (h.at || 0) * length;
                pushHill(center, h.length || 160, h.height || 10);
            }
            return list;
        }
        if (manifest.dailyHills) {
            const rng = AD.rng(AD.hashString(manifest.id + '/hills/' + trackSeed));
            const out = [];
            let from = -1;
            for (let i = 0; i <= samples.length; i++) {
                const sVal = i < samples.length ? samples[i].s : length;
                const ok = i < samples.length && curvature[i] < 0.01 && sVal >= 180 && sVal <= length - 140;
                if (ok && from < 0) from = i;
                if (!ok && from >= 0) {
                    const a = samples[from].s, b = samples[i - 1].s;
                    if (b - a >= 185) out.push({ from: a, to: b });
                    from = -1;
                }
            }
            for (let i = 0; i < out.length && list.length < 3; i++) {
                const w = out[i], avail = w.to - w.from;
                const steep = i !== 1;
                const span = clamp(steep ? 148 + rng() * 16 : Math.min(270, avail - 20), 140, avail - 10);
                const height = steep ? 9.5 + rng() * 1.4 : 11.5 + rng() * 2.5;
                const center = w.from + span * 0.5 + rng() * Math.max(0, avail - span);
                pushHill(center, span, height);
            }
        }
        return list;
    }

    AD.buildTrack = function (manifest, seed) {
        const points = AD.trackPoints(manifest);
        const trackSeed = (seed === undefined ? manifest.seed : seed) >>> 0;
        const positions = [];
        for (let i = 0; i < points.length; i++) {
            const p0 = points[(i - 1 + points.length) % points.length];
            const p1 = points[i];
            const p2 = points[(i + 1) % points.length];
            const p3 = points[(i + 2) % points.length];
            const span = Math.hypot(p2[0] - p1[0], p2[1] - p1[1]);
            const steps = clamp(Math.round(span / 4), 10, 90);
            for (let j = 0; j < steps; j++) {
                const t = j / steps;
                positions.push([
                    catmull(p0[0], p1[0], p2[0], p3[0], t),
                    catmull(p0[1], p1[1], p2[1], p3[1], t)
                ]);
            }
        }

        const samples = [];
        let length = 0;
        for (let i = 0; i < positions.length; i++) {
            const p = positions[i];
            const prev = positions[(i - 1 + positions.length) % positions.length];
            const next = positions[(i + 1) % positions.length];
            if (i > 0) length += Math.hypot(p[0] - prev[0], p[1] - prev[1]);
            const tx = next[0] - prev[0], ty = next[1] - prev[1];
            const len = Math.hypot(tx, ty) || 1;
            samples.push({
                x: p[0], y: p[1], s: length,
                tx: tx / len, ty: ty / len,
                nx: -ty / len, ny: tx / len,
                halfWidth: manifest.width * 0.5
            });
        }
        length += Math.hypot(positions[0][0] - positions[positions.length - 1][0],
            positions[0][1] - positions[positions.length - 1][1]);

        const grid = new Map();
        for (let i = 0; i < samples.length; i++) {
            const s = samples[i];
            const key = cellKey(Math.floor(s.x / GRID), Math.floor(s.y / GRID));
            let list = grid.get(key);
            if (!list) { list = []; grid.set(key, list); }
            list.push(i);
            s.halfWidth = manifest.width * 0.5 * (1 + 0.06 * Math.sin(s.s * 0.0022 + trackSeed % 17));
        }

        const gates = [];
        for (let i = 0; i < 12; i++) gates.push(length * i / 12);
        /* Six rows of three pickup locations across the corridor. */
        const pads = [];
        for (let row = 0; row < 6; row++) {
            const s = length * (row + 0.5) / 6;
            for (let lane = -1; lane <= 1; lane++) {
                pads.push({ s: s, offset: lane * manifest.width * 0.24, row: row });
            }
        }

        /* Curvature in rad/m per sample (a 100 m radius reads 0.01). A short
           moving average removes the spline's sample-to-sample jitter. */
        const raw = new Float32Array(samples.length);
        for (let i = 0; i < samples.length; i++) {
            const a = samples[(i - 1 + samples.length) % samples.length];
            const b = samples[(i + 1) % samples.length];
            const gap = Math.hypot(b.x - a.x, b.y - a.y) || 1;
            raw[i] = Math.abs(a.tx * b.ty - a.ty * b.tx) / gap;
        }
        const curvature = new Float32Array(samples.length);
        for (let i = 0; i < samples.length; i++) {
            let sum = 0;
            for (let k = -4; k <= 4; k++) sum += raw[(i + k + samples.length) % samples.length];
            curvature[i] = sum / 9;
        }

        const hills = resolveHills(manifest, length, samples, curvature, trackSeed);
        let maxZ = 0;
        for (let i = 0; i < samples.length; i++) {
            const prof = AD.evalHills(hills, length, samples[i].s);
            samples[i].z = prof.z;
            samples[i].slope = prof.slope;
            samples[i].curveZ = prof.curveZ;
            if (prof.z > maxZ) maxZ = prof.z;
        }

        const track = {
            id: manifest.id, name: manifest.name, manifest: manifest,
            seed: trackSeed, samples: samples, grid: grid, length: length,
            gates: gates, pads: pads, width: manifest.width, curvature: curvature,
            hills: hills, maxZ: maxZ,
            biome: BIOMES[manifest.biome] || BIOMES.alpine,
            biomeId: manifest.biome, zones: manifest.zones || [],
            laps: manifest.laps, points: points,
            /* Three timing sectors per lap give the race a rhythm: green and
               purple splits are what the driver is really chasing. */
            sectors: [length / 3, (length * 2) / 3],
            driftZones: typeof AD.buildDriftZones === 'function' ? AD.buildDriftZones({
                samples: samples, length: length
            }) : [],
            bounds: (function () {
                let minX = Infinity, minY = Infinity, maxX = -Infinity, maxY = -Infinity;
                for (const p of positions) {
                    minX = Math.min(minX, p[0]); maxX = Math.max(maxX, p[0]);
                    minY = Math.min(minY, p[1]); maxY = Math.max(maxY, p[1]);
                }
                return { minX, minY, maxX, maxY };
            })()
        };
        track.puddles = drainage(track);
        return track;
    };

    /* Standing water occupies one wheel line. Other low spots only fill in rain. */
    function drainage(track) {
        const rng = AD.rng(AD.hashString(track.id + '/drainage/' + track.seed)), out = [];
        const wetZones = track.zones.filter(z => z.type === 'wet');
        const rainy = ['harbour', 'forest', 'metro'].includes(track.biomeId);
        function add(fraction, side, permanent) {
            let bestF = fraction, p = AD.pointAt(track, bestF * track.length);
            if (Math.abs(p.slope || 0) > 0.08) {
                for (const shift of [0.012, -0.012, 0.025, -0.025, 0.04, -0.04]) {
                    const f2 = ((fraction + shift) % 1 + 1) % 1;
                    const p2 = AD.pointAt(track, f2 * track.length);
                    if (Math.abs(p2.slope || 0) < Math.abs(p.slope || 0)) { p = p2; bestF = f2; }
                    if (Math.abs(p.slope || 0) <= 0.05) break;
                }
            }
            const s = bestF * track.length;
            const off = side * p.halfWidth * 0.45;
            out.push({ s, x: p.x + p.nx * off, y: p.y + p.ny * off, z: p.z || 0, slope: p.slope || 0, tx: p.tx, ty: p.ty,
                length: 8 + rng() * 3, width: Math.min(3.6, p.halfWidth * 0.32), permanent });
        }
        for (const zone of wetZones) for (const part of [0.24, 0.77]) {
            add(zone.from + (zone.to - zone.from) * part, zone.side || (part < 0.5 ? -1 : 1), true);
        }
        if (rainy) for (let i = 0; i < 4; i++) {
            const fraction = (i + 0.32 + rng() * 0.36) / 4;
            if (out.every(p => Math.abs(p.s - fraction * track.length) > 32)) add(fraction, i % 2 ? 1 : -1, false);
        }
        return out;
    }

    AD.puddleLevel = (p, wetness) => p.permanent ? 1 : clamp((wetness - 0.25) * 1.7, 0, 1);

    AD.drivingSurface = function (track, sample, v, conditions) {
        const surface = AD.surfaceAt(track, sample.s, sample.offset, sample);
        const wetness = conditions?.wetness || 0;
        let puddle = 0;
        for (const p of track.puddles || []) {
            if (Math.abs(sample.s - p.s) > p.length + 6) continue;
            const dx = v.x - p.x, dy = v.y - p.y;
            const along = (dx * p.tx + dy * p.ty) / p.length;
            const across = (dy * p.tx - dx * p.ty) / p.width;
            puddle = Math.max(puddle, (1 - along * along - across * across) * AD.puddleLevel(p, wetness));
        }
        surface.puddle = puddle;
        surface.water = Math.max(surface.type === 'wet' ? 0.8 : 0, wetness, puddle);
        surface.grip *= (conditions?.grip ?? 1) * (1 - 0.38 * puddle);
        if (surface.type === 'asphalt' && surface.water > 0.35) surface.type = 'wet';
        return surface;
    };

    AD.sampleTrack = function (track, x, y) {
        const cx = Math.floor(x / GRID), cy = Math.floor(y / GRID);
        let best = -1, bestD2 = Infinity;
        for (let ox = -1; ox <= 1; ox++) {
            for (let oy = -1; oy <= 1; oy++) {
                const list = track.grid.get(cellKey(cx + ox, cy + oy));
                if (!list) continue;
                for (let k = 0; k < list.length; k++) {
                    const i = list[k], s = track.samples[i];
                    const d2 = (s.x - x) * (s.x - x) + (s.y - y) * (s.y - y);
                    if (d2 < bestD2) { bestD2 = d2; best = i; }
                }
            }
        }
        if (best < 0) {
            for (let i = 0; i < track.samples.length; i += 3) {
                const s = track.samples[i];
                const d2 = (s.x - x) * (s.x - x) + (s.y - y) * (s.y - y);
                if (d2 < bestD2) { bestD2 = d2; best = i; }
            }
        }
        const s = track.samples[best] || track.samples[0];
        const dx = x - s.x, dy = y - s.y;
        const offset = dx * s.nx + dy * s.ny;
        const along = dx * s.tx + dy * s.ty;
        const prof = track.maxZ ? AD.roadProfileAt(track, (s.s || 0) + along) : null;
        return {
            index: best, s: s.s, offset: offset,
            nx: s.nx, ny: s.ny, tx: s.tx, ty: s.ty,
            halfWidth: s.halfWidth, x: s.x, y: s.y,
            z: prof ? prof.z : 0, slope: prof ? prof.slope : 0, curveZ: prof ? prof.curveZ : 0
        };
    };

    AD.pointAt = function (track, s) {
        const target = ((s % track.length) + track.length) % track.length;
        let lo = 0, hi = track.samples.length - 1;
        while (lo < hi) {
            const mid = (lo + hi) >> 1;
            if (track.samples[mid].s < target) lo = mid + 1; else hi = mid;
        }
        const idx = Math.max(0, lo - 1);
        const a = track.samples[idx];
        const b = track.samples[(idx + 1) % track.samples.length];
        const span = (b.s || track.length) - a.s || 4;
        const t = clamp((target - a.s) / span, 0, 1);
        const prof = track.maxZ ? AD.roadProfileAt(track, target) : null;
        return {
            x: AD.lerp(a.x, b.x, t), y: AD.lerp(a.y, b.y, t),
            z: prof ? prof.z : 0, slope: prof ? prof.slope : 0, curveZ: prof ? prof.curveZ : 0,
            nx: a.nx, ny: a.ny, tx: a.tx, ty: a.ty,
            halfWidth: a.halfWidth
        };
    };

    AD.surfaceAt = function (track, s, offset, sample) {
        let type = track.biome.base;
        const fraction = ((s % track.length) + track.length) % track.length / track.length;
        for (const zone of track.zones) {
            if (fraction >= zone.from && fraction <= zone.to && (zone.side === 0 || Math.sign(offset) === zone.side)) {
                type = zone.type;
            }
        }
        if (sample && Math.abs(offset) > sample.halfWidth) type = track.biome.runoff;
        const surface = AD.SURFACES[type] || AD.SURFACES.asphalt;
        return { grip: surface.grip, roll: surface.roll, type: type };
    };

    /* Runs of the lap where the road is nearly straight (curvature < 0.01),
       at least minLength metres long, as [{from, to}] in arc length. Runs do
       not wrap across the start/finish seam. */
    AD.straightWindows = function (track, minLength) {
        const out = [];
        const samples = track.samples, curvature = track.curvature;
        let from = -1;
        for (let i = 0; i <= samples.length; i++) {
            const straight = i < samples.length && curvature[i] < 0.01;
            if (straight && from < 0) from = i;
            if (!straight && from >= 0) {
                const a = samples[from].s;
                const b = samples[i - 1].s;
                if (b - a >= minLength) out.push({ from: a, to: b });
                from = -1;
            }
        }
        return out;
    };

    /* Daily routes carry a generated name; the shell shows a locale key instead. */
    AD.trackName = function (track, t) {
        if (track && typeof track.id === 'string' && track.id.indexOf('daily-') === 0 && typeof t === 'function') {
            return t('aura_drift.daily_route');
        }
        return track ? track.name : '';
    };

    AD.chunkKey = (cx, cy) => cx + ',' + cy;

    /* Props are generated per chunk and only outside the drivable corridor. */
    AD.chunkProps = function (track, cx, cy) {
        const rng = AD.rng(AD.hashString(track.id + '/props/' + cx + '/' + cy + '/' + track.seed));
        const props = [];
        const originX = cx * AD.CHUNK_METRES, originY = cy * AD.CHUNK_METRES;
        const count = track.biomeId === 'salt' ? 5 : 22;
        for (let i = 0; i < count; i++) {
            const x = originX + rng() * AD.CHUNK_METRES;
            const y = originY + rng() * AD.CHUNK_METRES;
            const sample = AD.sampleTrack(track, x, y);
            if (Math.abs(sample.offset) < sample.halfWidth + 7) continue;
            props.push({ x, y, kind: track.biomeId, size: 0.7 + rng() * 0.9, angle: rng() * Math.PI * 2 });
        }
        return props;
    };

    const HILLY_BIOMES = { alpine: true, desert: true, forest: true, glacier: true, industry: true };
    AD.dailyTrack = function (seed) {
        const rng = AD.rng((seed >>> 0) || 1);
        const biomeIds = Object.keys(BIOMES);
        const biome = biomeIds[Math.floor(rng() * biomeIds.length)];
        return {
            id: 'daily-' + (seed >>> 0), name: 'Tagesroute', seed: seed >>> 0,
            radius: 480 + Math.floor(rng() * 380), aspect: 0.55 + rng() * 0.4,
            harmonics: [
                { k: 2 + Math.floor(rng() * 2), amp: 0.06 + rng() * 0.11, phase: rng() * 6.28 },
                { k: 5 + Math.floor(rng() * 4), amp: 0.02 + rng() * 0.05, phase: rng() * 6.28 }
            ],
            width: 19 + Math.floor(rng() * 12), laps: 3, biome: biome, zones: [],
            dailyHills: !!HILLY_BIOMES[biome]
        };
    };

    AD.validateTrack = function (track) {
        const reasons = [];
        if (!track || !track.samples || track.samples.length < 200) reasons.push('too few samples');
        if (track.length < 1500) reasons.push('track too short');
        if (track.width < 12) reasons.push('track too narrow');
        let minRadius = Infinity, maxGap = 0, badElevation = false;
        for (let i = 0; i < track.samples.length; i++) {
            const a = track.samples[i], b = track.samples[(i + 1) % track.samples.length];
            const gap = Math.hypot(b.x - a.x, b.y - a.y);
            maxGap = Math.max(maxGap, gap);
            const cross = a.tx * b.ty - a.ty * b.tx;
            if (Math.abs(cross) > 1e-6) minRadius = Math.min(minRadius, gap / Math.abs(cross));
            const az = a.z || 0, asl = a.slope || 0, acz = a.curveZ || 0;
            if (!Number.isFinite(az) || !Number.isFinite(asl) || !Number.isFinite(acz) || az < -1e-6 || Math.abs(asl) > 0.35) {
                badElevation = true;
            }
        }
        if (badElevation || Math.abs((track.samples[0] && track.samples[0].z) || 0) >= 1e-3) {
            reasons.push('invalid elevation profile');
        }
        if (maxGap > 8) reasons.push('spline spacing too large');
        if (minRadius < 14) reasons.push('corner radius too tight: ' + AD.round2(minRadius));
        /* Only compare sections that are far apart along the loop. Samples near
           the start/finish seam are neighbours, not a self-intersection. */
        for (let i = 0; i < track.samples.length; i += 6) {
            const a = track.samples[i];
            for (let j = i + 60; j < track.samples.length; j += 6) {
                const along = Math.min(j - i, track.samples.length - (j - i));
                if (along < 60) continue;
                const b = track.samples[j];
                if (Math.abs(b.x - a.x) > track.width * 1.6 || Math.abs(b.y - a.y) > track.width * 1.6) continue;
                if (Math.hypot(b.x - a.x, b.y - a.y) < track.width * 0.85) {
                    reasons.push('overlapping road sections');
                    i = track.samples.length;
                    break;
                }
            }
        }
        return { ok: reasons.length === 0, reasons: reasons, minRadius: minRadius, maxGap: maxGap, length: track.length };
    };
})();
