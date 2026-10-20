(function () {
    'use strict';

    // Constants, small math helpers, the seeded RNG and the rounded-rectangle geometry of the Green Slime
    // gel. Pure functions without state (no document, no performance, no Math.random): shared by the
    // simulation modules (green_slime_sim.js and its parts) and by the renderers.
    // Units: pixels, seconds, screen coordinates (y points down).

    const SURFACE_SAMPLES = 32;
    const TABLE_SAMPLES = 128;
    const MAX_DROPS = 72;
    const MAX_SPRAY = 24;
    const MAX_SPLATS = 12;
    const MAX_BUBBLES = 5;
    const MAX_BUBBLES_PER_WINDOW = 2;
    const STRAND_SNAP = 170;
    const STRAND_HOLD = 2.5;
    const MIN_GEL_RADIUS = 10;
    const MAX_GEL_RADIUS = 44;
    const MAX_DEFORMATION = 18;
    const FRAME_DEPTH = 19;
    const SAG_TOP = 13;
    const SAG_BOTTOM = 24;
    const GRAVITY = 250;
    const SPRAY_GRAVITY = 520;
    const RIPPLE_SPEED = 200;
    const RIPPLE_DAMPING = 3.4;
    const RIPPLE_STIFFNESS = 5;
    const RIPPLE_LIMIT = 7;
    const STRAND_OMEGA = 22;
    const DEFAULT_RADIUS = 22;
    const BUBBLE_STAGES = Object.freeze(['nucleate', 'grow', 'hold', 'burst', 'crater']);
    const SURFACE_NORMALS = Object.freeze([{ x: 0, y: -1 }, { x: 1, y: 0 }, { x: 0, y: 1 }, { x: -1, y: 0 }]);
    const HALF_PI = Math.PI / 2;
    const TWO_PI = Math.PI * 2;

    function clamp(value, min, max) { return Math.max(min, Math.min(max, value)); }
    function lerp(a, b, t) { return a + (b - a) * t; }
    function smoothstep(a, b, x) { const t = clamp((x - a) / (b - a), 0, 1); return t * t * (3 - 2 * t); }
    function wrap(value, period) { const m = value % period; return m < 0 ? m + period : m; }
    function cyclicDelta(a, b, period) { const d = wrap(a - b, period); return d > period / 2 ? d - period : d; }
    function easeOut(t) { return 1 - (1 - t) * (1 - t) * (1 - t); }
    function inRect(rect, x, y) {
        return x >= rect.x && x <= rect.x + rect.width && y >= rect.y && y <= rect.y + rect.height;
    }

    function hashString(text) {
        let h = 2166136261;
        for (let i = 0; i < text.length; i++) { h ^= text.charCodeAt(i); h = Math.imul(h, 16777619); }
        return h >>> 0;
    }

    function mulberry32(seed) {
        let a = seed >>> 0;
        return function () {
            a = (a + 0x6D2B79F5) >>> 0;
            let t = a;
            t = Math.imul(t ^ (t >>> 15), t | 1);
            t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
            return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
        };
    }

    // ---------------------------------------------------------------------------------------------
    // Rounded-rectangle perimeter. Arclength sample k of 128 sits at perimeter fraction (k + .5) / 128,
    // starting where the top edge begins (end of the top-left arc) and running clockwise. The shader
    // mirrors this walk one to one: eight segments, top edge first, then the top-right arc, right edge,
    // bottom-right arc, bottom edge (right to left), bottom-left arc, left edge (bottom to top) and the
    // top-left arc.

    function normalizeRadii(width, height, radii) {
        const source = Array.isArray(radii) ? radii : radii == null ? [DEFAULT_RADIUS, DEFAULT_RADIUS, DEFAULT_RADIUS, DEFAULT_RADIUS] : [radii, radii, radii, radii];
        const r = [0, 1, 2, 3].map(i => Math.max(0, Number(source[i]) || 0));
        // CSS scales all radii by one factor when two neighbours overflow a side.
        let factor = 1;
        for (const [sum, side] of [[r[0] + r[1], width], [r[3] + r[2], width], [r[0] + r[3], height], [r[1] + r[2], height]]) {
            if (sum > side && sum > 0) factor = Math.min(factor, side / sum);
        }
        return factor < 1 ? r.map(value => value * factor) : r;
    }

    function layout(rect, radii) {
        const w = rect.width, h = rect.height, x = rect.x, y = rect.y;
        const r = normalizeRadii(w, h, radii);
        const tl = r[0], tr = r[1], br = r[2], bl = r[3];
        const segs = [
            { arc: false, x0: x + tl, y0: y, dx: 1, dy: 0, nx: 0, ny: -1, len: Math.max(0, w - tl - tr) },
            { arc: true, cx: x + w - tr, cy: y + tr, r: tr, a0: -HALF_PI, len: tr * HALF_PI },
            { arc: false, x0: x + w, y0: y + tr, dx: 0, dy: 1, nx: 1, ny: 0, len: Math.max(0, h - tr - br) },
            { arc: true, cx: x + w - br, cy: y + h - br, r: br, a0: 0, len: br * HALF_PI },
            { arc: false, x0: x + w - br, y0: y + h, dx: -1, dy: 0, nx: 0, ny: 1, len: Math.max(0, w - br - bl) },
            { arc: true, cx: x + bl, cy: y + h - bl, r: bl, a0: HALF_PI, len: bl * HALF_PI },
            { arc: false, x0: x, y0: y + h - bl, dx: 0, dy: -1, nx: -1, ny: 0, len: Math.max(0, h - bl - tl) },
            { arc: true, cx: x + tl, cy: y + tl, r: tl, a0: Math.PI, len: tl * HALF_PI }
        ];
        let total = 0;
        for (const segment of segs) { segment.start = total; total += segment.len; }
        return { x, y, w, h, radii: r, segs, length: total };
    }

    // Point on a segment at fraction t (0..1) with the outward normal.
    function segmentPoint(g, t, out) {
        if (g.arc) {
            const a = g.a0 + t * HALF_PI;
            out.nx = Math.cos(a);
            out.ny = Math.sin(a);
            out.x = g.cx + g.r * out.nx;
            out.y = g.cy + g.r * out.ny;
        } else {
            out.x = g.x0 + g.dx * g.len * t;
            out.y = g.y0 + g.dy * g.len * t;
            out.nx = g.nx;
            out.ny = g.ny;
        }
        return out;
    }

    function pointAt(lay, s, out) {
        out = out || {};
        const u = wrap(s, lay.length);
        let last = null;
        for (const g of lay.segs) {
            if (g.len <= 0) continue;
            last = g;
            if (u < g.start + g.len) return segmentPoint(g, (u - g.start) / g.len, out);
        }
        return last ? segmentPoint(last, 1, out) : segmentPoint(lay.segs[0], 0, out);
    }

    // Exact signed distance to the rounded rectangle (negative inside).
    function signedDistance(lay, px, py) {
        const qx = px - (lay.x + lay.w / 2), qy = py - (lay.y + lay.h / 2);
        const r = qx < 0 ? (qy < 0 ? lay.radii[0] : lay.radii[3]) : (qy < 0 ? lay.radii[1] : lay.radii[2]);
        const ax = Math.abs(qx) - (lay.w / 2 - r), ay = Math.abs(qy) - (lay.h / 2 - r);
        return Math.min(Math.max(ax, ay), 0) + Math.hypot(Math.max(ax, 0), Math.max(ay, 0)) - r;
    }

    const projectProbe = {};
    // Nearest perimeter point of (px, py): arclength s, signed distance (positive outside) and the
    // outward normal of the perimeter there.
    function project(lay, px, py, out) {
        out = out || {};
        let bestD = Infinity, bestSegment = null, bestT = 0, bx = 0, by = 0, bnx = 0, bny = -1;
        for (const g of lay.segs) {
            if (g.len <= 0) continue;
            let t = 0;
            if (g.arc) {
                if (g.r > 0) {
                    const a = wrap(Math.atan2(py - g.cy, px - g.cx) - g.a0, TWO_PI);
                    t = a <= HALF_PI ? a / HALF_PI : (a - HALF_PI < TWO_PI - a ? 1 : 0);
                }
            } else {
                t = clamp(((px - g.x0) * g.dx + (py - g.y0) * g.dy) / g.len, 0, 1);
            }
            segmentPoint(g, t, projectProbe);
            const dx = px - projectProbe.x, dy = py - projectProbe.y;
            const d2 = dx * dx + dy * dy;
            if (d2 < bestD) {
                bestD = d2; bestSegment = g; bestT = t;
                bx = projectProbe.x; by = projectProbe.y; bnx = projectProbe.nx; bny = projectProbe.ny;
            }
        }
        out.s = bestSegment ? bestSegment.start + bestT * bestSegment.len : 0;
        const sign = signedDistance(lay, px, py) < 0 ? -1 : 1;
        out.dist = sign * Math.sqrt(bestD);
        out.x = bx; out.y = by; out.nx = bnx; out.ny = bny;
        return out;
    }

    const geometry = Object.freeze({
        layout, pointAt, project, signedDistance, normalizeRadii, segmentPoint,
        perimeterLength(rect, radii) { return layout(rect, radii).length; },
        perimeterPoint(rect, radii, s) { return pointAt(layout(rect, radii), s, {}); },
        projectPerimeter(rect, radii, x, y) { return project(layout(rect, radii), x, y, {}); }
    });

    // ---------------------------------------------------------------------------------------------
    // Spring nodes: the 32 base targets sit on the outline FRAME_DEPTH around the window with a
    // 34 px corner inset (the contour the effect always used).

    function surfaceTargets(rect) {
        const outline = { x: rect.x - FRAME_DEPTH, y: rect.y - FRAME_DEPTH, width: rect.width + FRAME_DEPTH * 2, height: rect.height + FRAME_DEPTH * 2 };
        const sideCount = SURFACE_SAMPLES / 4;
        const corner = 34;
        const points = [];
        for (let i = 0; i < sideCount; i++) {
            const t = i / (sideCount - 1);
            points.push({ x: outline.x + corner + (outline.width - corner * 2) * t, y: outline.y });
        }
        for (let i = 0; i < sideCount; i++) {
            const t = i / (sideCount - 1);
            points.push({ x: outline.x + outline.width, y: outline.y + corner + (outline.height - corner * 2) * t });
        }
        for (let i = 0; i < sideCount; i++) {
            const t = i / (sideCount - 1);
            points.push({ x: outline.x + corner + (outline.width - corner * 2) * (1 - t), y: outline.y + outline.height });
        }
        for (let i = 0; i < sideCount; i++) {
            const t = i / (sideCount - 1);
            points.push({ x: outline.x, y: outline.y + corner + (outline.height - corner * 2) * (1 - t) });
        }
        return points;
    }

    // The part of a window's gel that depends only on its size: where the nodes sit along the
    // perimeter, the cubic Hermite weights of every table sample and the gravity sag.
    function buildProfile(width, height, radii) {
        const lay = layout({ x: 0, y: 0, width, height }, radii);
        const L = lay.length;
        const targets = surfaceTargets({ x: 0, y: 0, width, height });
        const nodeS = new Float64Array(SURFACE_SAMPLES);
        const probe = {};
        for (let i = 0; i < SURFACE_SAMPLES; i++) nodeS[i] = project(lay, targets[i].x, targets[i].y, probe).s;
        if (nodeS[0] > L / 2) nodeS[0] -= L;
        let ordered = nodeS[SURFACE_SAMPLES - 1] < nodeS[0] + L;
        for (let i = 1; i < SURFACE_SAMPLES && ordered; i++) ordered = nodeS[i] > nodeS[i - 1];
        if (!ordered) for (let i = 0; i < SURFACE_SAMPLES; i++) nodeS[i] = nodeS[0] + i * L / SURFACE_SAMPLES;
        const den = new Float64Array(SURFACE_SAMPLES);
        for (let n = 0; n < SURFACE_SAMPLES; n++) {
            const before = n === 0 ? nodeS[0] + L - nodeS[SURFACE_SAMPLES - 1] : nodeS[n] - nodeS[n - 1];
            const after = n === SURFACE_SAMPLES - 1 ? nodeS[0] + L - nodeS[n] : nodeS[n + 1] - nodeS[n];
            den[n] = before + after;
        }
        const profile = {
            width, height, radii: lay.radii, length: L, nodeS, den,
            ia: new Uint8Array(TABLE_SAMPLES), ib: new Uint8Array(TABLE_SAMPLES), gap: new Float32Array(TABLE_SAMPLES),
            h00: new Float32Array(TABLE_SAMPLES), h10: new Float32Array(TABLE_SAMPLES),
            h01: new Float32Array(TABLE_SAMPLES), h11: new Float32Array(TABLE_SAMPLES),
            sag: new Float32Array(TABLE_SAMPLES)
        };
        for (let k = 0; k < TABLE_SAMPLES; k++) {
            const s = (k + .5) / TABLE_SAMPLES * L;
            let sk = s;
            if (sk >= nodeS[0] + L) sk -= L; else if (sk < nodeS[0]) sk += L;
            let i = 0;
            while (i < SURFACE_SAMPLES - 1 && nodeS[i + 1] <= sk) i++;
            const a = nodeS[i];
            const b = i === SURFACE_SAMPLES - 1 ? nodeS[0] + L : nodeS[i + 1];
            const t = (sk - a) / (b - a);
            profile.ia[k] = i;
            profile.ib[k] = (i + 1) % SURFACE_SAMPLES;
            profile.gap[k] = b - a;
            profile.h00[k] = 2 * t * t * t - 3 * t * t + 1;
            profile.h10[k] = t * t * t - 2 * t * t + t;
            profile.h01[k] = -2 * t * t * t + 3 * t * t;
            profile.h11[k] = t * t * t - t * t;
            // Gravity sag: thin on top, heavy at the bottom, sides in between (normal-driven, so the
            // corners blend smoothly).
            profile.sag[k] = (SAG_TOP + SAG_BOTTOM) / 2 + (SAG_BOTTOM - SAG_TOP) / 2 * pointAt(lay, s, probe).ny;
        }
        return profile;
    }

    // Soft lower bound at MIN_GEL_RADIUS: C1 blend over +-3 px so breathing never shows a kink.
    function softFloor(radius) {
        const x = radius - MIN_GEL_RADIUS;
        return MIN_GEL_RADIUS + (x >= 3 ? x : x <= -3 ? 0 : (x + 3) * (x + 3) / 12);
    }

    function strandRadii(length) {
        // Volume conservation: the thread thins as it is drawn out; below ~1.6 px the middle breaks
        // up into beads (Rayleigh-Plateau).
        const mid = clamp(4.6 * Math.pow(28 / Math.max(length, 12), .8), .45, 6.2);
        const beads = mid < 1.6 ? clamp(Math.round(2 + (1.6 - mid) / .45 * 1.5), 2, 4) : 0;
        return { root: Math.min(9, mid * 1.9 + 2.2), mid, tip: mid * 1.5 + 1.1, beads };
    }

    window.AuraGreenSlimeGeometry = Object.freeze({
        clamp, lerp, smoothstep, wrap, cyclicDelta, easeOut, inRect, hashString, mulberry32,
        geometry, surfaceTargets, buildProfile, softFloor, strandRadii, SURFACE_NORMALS,
        constants: Object.freeze({
            SURFACE_SAMPLES, TABLE_SAMPLES, MAX_DROPS, MAX_SPRAY, MAX_SPLATS, MAX_BUBBLES, MAX_BUBBLES_PER_WINDOW,
            STRAND_SNAP, STRAND_HOLD, MIN_GEL_RADIUS, MAX_GEL_RADIUS, MAX_DEFORMATION, FRAME_DEPTH, SAG_TOP, SAG_BOTTOM,
            GRAVITY, SPRAY_GRAVITY, RIPPLE_SPEED, RIPPLE_DAMPING, RIPPLE_STIFFNESS, RIPPLE_LIMIT, STRAND_OMEGA,
            DEFAULT_RADIUS, BUBBLE_STAGES, HALF_PI, TWO_PI
        })
    });
})();
