// Pixel pure image library, "art" part. DOM-free: loads in a page, in the Pixel worker and in a node vm.
// Contents: Kubelka-Munk pigment mixing (blue + yellow = green), paper / canvas substrates, the media table and the
// brush engines bristle (oil, acrylic, gouache, knife, blender), dry (charcoal, pencil, pastel, crayon) and marker.
// The wet engine (watercolour, ink) lives in pixel-lib-wet.js and registers itself through paint.registerEngine.
// Dirty rects returned by brush.dab are { x0, y0, x1, y1 } in doc px, x0/y0 inclusive, x1/y1 exclusive.
// Needs pixel-lib-core.js first. Everything runs per pointer event: typed arrays, hoisted scratch buffers, no per-pixel allocation.
(function (root) {
    'use strict';

    const Lib = root.AuraPixelLib = root.AuraPixelLib || {};
    if (typeof Lib.mulberry32 !== 'function' || typeof Lib.hashSeed !== 'function') {
        throw new Error('pixel-lib-paint.js needs pixel-lib-core.js to be loaded first');
    }
    const rng = Lib.mulberry32;
    const PI = Math.PI;

    function clamp(v, lo, hi) { return v < lo ? lo : (v > hi ? hi : v); }
    function finite(v, fallback) { const n = Number(v); return Number.isFinite(n) ? n : fallback; }
    function smoothstep(a, b, x) {
        if (x <= a) return 0;
        if (x >= b) return 1;
        const t = (x - a) / (b - a);
        return t * t * (3 - 2 * t);
    }

    // --- pigment model (5 reflectance bands, Kubelka-Munk with one constant) ---

    const GAM = 0.1214;
    const EPS = 0.07;
    const T = Object.freeze([
        Object.freeze([0.0839, -0.1678, -0.0253, 0.2388, 0.8704]),
        Object.freeze([-0.3218, 0.6436, 0.6086, 0.0611, 0.0085]),
        Object.freeze([0.9276, 0.1449, 0.0313, 0.0177, -0.1214])
    ]);
    const PIGMENT = Object.freeze({ GAM, EPS, T });
    const T00 = T[0][0], T01 = T[0][1], T02 = T[0][2], T03 = T[0][3], T04 = T[0][4];
    const T10 = T[1][0], T11 = T[1][1], T12 = T[1][2], T13 = T[1][3], T14 = T[1][4];
    const T20 = T[2][0], T21 = T[2][1], T22 = T[2][2], T23 = T[2][3], T24 = T[2][4];

    const srgbToLinear = new Float32Array(256);
    for (let i = 0; i < 256; i++) {
        const c = i / 255;
        srgbToLinear[i] = c <= 0.04045 ? c / 12.92 : Math.pow((c + 0.055) / 1.055, 2.4);
    }
    const LIN2SRGB = new Uint8Array(4096);
    for (let i = 0; i < 4096; i++) {
        const v = i / 4095;
        LIN2SRGB[i] = Math.round(255 * (v <= 0.0031308 ? v * 12.92 : 1.055 * Math.pow(v, 1 / 2.4) - 0.055));
    }
    function linearToSrgb(v) {
        return v <= 0 ? 0 : (v >= 1 ? 255 : LIN2SRGB[(v * 4095 + 0.5) | 0]);
    }

    function ks(r) { return (1 - r) * (1 - r) / (2 * r); }
    function ksInv(k) {
        if (k < 0) k = 0;
        return 1 + k - Math.sqrt(k * k + 2 * k);
    }
    function band(v) { return v < EPS ? EPS : (v > 1 ? 1 : v); }

    // r, g, b are sRGB bytes already rounded to 0..255 (the callers below guarantee it).
    function pigmentOfByte(r, g, b, out) {
        const lr = srgbToLinear[r], lg = srgbToLinear[g], lb = srgbToLinear[b];
        const V = band(lb + GAM * (lr - lg)), C = band((lg + lb) / 2), G = band(lg), Y = band((lg + lr) / 2), R = band(lr);
        out[0] = ks(V); out[1] = ks(C); out[2] = ks(G); out[3] = ks(Y); out[4] = ks(R);
        out[5] = lr - (T00 * V + T01 * C + T02 * G + T03 * Y + T04 * R);
        out[6] = lg - (T10 * V + T11 * C + T12 * G + T13 * Y + T14 * R);
        out[7] = lb - (T20 * V + T21 * C + T22 * G + T23 * Y + T24 * R);
        return out;
    }
    function byte(v) { return v <= 0 ? 0 : (v >= 255 ? 255 : (v + 0.5) | 0); }
    function pigmentOf(r, g, b, out) {
        return pigmentOfByte(byte(r), byte(g), byte(b), out || new Float32Array(8));
    }
    function pigmentMix(out, a, b, t) {
        const s = 1 - t;
        for (let i = 0; i < 8; i++) out[i] = a[i] * s + b[i] * t;
        return out;
    }
    function pigmentToRgb(p, out) {
        out = out || [0, 0, 0];
        const R0 = ksInv(p[0]), R1 = ksInv(p[1]), R2 = ksInv(p[2]), R3 = ksInv(p[3]), R4 = ksInv(p[4]);
        out[0] = linearToSrgb(T00 * R0 + T01 * R1 + T02 * R2 + T03 * R3 + T04 * R4 + p[5]);
        out[1] = linearToSrgb(T10 * R0 + T11 * R1 + T12 * R2 + T13 * R3 + T14 * R4 + p[6]);
        out[2] = linearToSrgb(T20 * R0 + T21 * R1 + T22 * R2 + T23 * R3 + T24 * R4 + p[7]);
        return out;
    }
    const mixA = new Float32Array(8), mixB = new Float32Array(8);
    function mixRgb(c1, c2, t) {
        pigmentOf(c1[0], c1[1], c1[2], mixA);
        pigmentOf(c2[0], c2[1], c2[2], mixB);
        return pigmentToRgb(pigmentMix(mixA, mixA, mixB, t));
    }
    // A transparent layer over a substrate: absorption adds, the residual is added scaled by the density.
    function glaze(out, base, ink, density) {
        for (let n = 0; n < 5; n++) out[n] = base[n] + density * ink[n];
        const rs = density < 1 ? density : 1;
        for (let i = 5; i < 8; i++) out[i] = base[i] + ink[i] * rs;
        return out;
    }
    const WHITE = pigmentOf(255, 255, 255);

    // One colour write. w = coverage 0..1 after the selection. Returns true when the pixel was touched.
    // maxA (optional) caps the alpha that this write may reach (dry media never get fully opaque).
    const dpA = new Float32Array(8), dpB = new Float32Array(8), dpRgb = [0, 0, 0];
    function depositPixel(data, i, pig, rgb, w, lockAlpha, maxA) {
        const pa = data[i + 3] / 255;
        if (lockAlpha && pa === 0) return false;
        let na;
        if (pa < 0.004) {
            data[i] = rgb[0]; data[i + 1] = rgb[1]; data[i + 2] = rgb[2];
            if (!lockAlpha) {
                na = w;
                if (na > maxA) na = maxA;
                data[i + 3] = Math.round(na * 255);
            }
            return true;
        }
        const t = w / (w + pa * (1 - w));
        if (t > 0.985) {
            data[i] = rgb[0]; data[i + 1] = rgb[1]; data[i + 2] = rgb[2];
        } else if (t > 0.01) {
            const d0 = data[i] - rgb[0], d1 = data[i + 1] - rgb[1], d2 = data[i + 2] - rgb[2];
            if (d0 <= 24 && d0 >= -24 && d1 <= 24 && d1 >= -24 && d2 <= 24 && d2 >= -24) {
                // close colours: pigment mixing only matters between different hues, so blend the bytes
                data[i] -= d0 * t; data[i + 1] -= d1 * t; data[i + 2] -= d2 * t;
            } else {
            pigmentOfByte(data[i], data[i + 1], data[i + 2], dpA);
            pigmentMix(dpB, dpA, pig, t);
            pigmentToRgb(dpB, dpRgb);
            data[i] = dpRgb[0]; data[i + 1] = dpRgb[1]; data[i + 2] = dpRgb[2];
            }
        }
        if (!lockAlpha) {
            na = w + pa * (1 - w);
            if (na > maxA) na = pa > maxA ? pa : maxA;
            data[i + 3] = Math.round(na * 255);
        }
        return true;
    }

    // --- papers ---

    const PAPER_SIZE = 256;
    const PAPERS = Object.freeze([
        { id: 'smooth', absorb: 0.35, tooth: 0.25, tint: [250, 249, 246] },
        { id: 'cold_press', absorb: 0.6, tooth: 0.6, tint: [247, 244, 236] },
        { id: 'rough', absorb: 0.75, tooth: 0.85, tint: [244, 240, 230] },
        { id: 'canvas', absorb: 0.2, tooth: 0.7, tint: [243, 240, 232] },
        { id: 'linen', absorb: 0.25, tooth: 0.55, tint: [226, 216, 196] },
        { id: 'laid', absorb: 0.5, tooth: 0.75, tint: [232, 226, 214] },
        { id: 'kraft', absorb: 0.55, tooth: 0.5, tint: [196, 160, 118] },
        { id: 'wood', absorb: 0.15, tooth: 0.4, tint: [205, 170, 125] }
    ].map(item => Object.freeze({
        id: item.id, labelKey: 'pixel.art_paper_' + item.id, absorb: item.absorb, tooth: item.tooth, tint: Object.freeze(item.tint)
    })));
    const PAPER_STD = { smooth: 0.04, cold_press: 0.16, rough: 0.22, canvas: 0.2, linen: 0.18, laid: 0.15, kraft: 0.14, wood: 0.15 };

    const latIdx = new Int32Array(PAPER_SIZE), latIdx1 = new Int32Array(PAPER_SIZE), latFr = new Float32Array(PAPER_SIZE);
    const latIdy = new Int32Array(PAPER_SIZE), latIdy1 = new Int32Array(PAPER_SIZE), latFy = new Float32Array(PAPER_SIZE);
    function latticeAxis(count, ids, ids1, frs) {
        for (let x = 0; x < PAPER_SIZE; x++) {
            const g = x * count / PAPER_SIZE, ix = Math.floor(g), f = g - ix;
            ids[x] = ix % count; ids1[x] = (ix + 1) % count; frs[x] = f * f * (3 - 2 * f);
        }
    }
    // Adds amp * tileable value noise on a cx * cy lattice (cx, cy divide 256, so the tile wraps without a seam).
    function addNoise(arr, cx, cy, amp, rand) {
        const lat = new Float32Array(cx * cy);
        for (let i = 0; i < lat.length; i++) lat[i] = rand() * 2 - 1;
        latticeAxis(cx, latIdx, latIdx1, latFr);
        latticeAxis(cy, latIdy, latIdy1, latFy);
        for (let y = 0; y < PAPER_SIZE; y++) {
            const r0 = latIdy[y] * cx, r1 = latIdy1[y] * cx, fy = latFy[y];
            for (let x = 0; x < PAPER_SIZE; x++) {
                const fx = latFr[x], a = lat[r0 + latIdx[x]], b = lat[r0 + latIdx1[x]], c = lat[r1 + latIdx[x]], d = lat[r1 + latIdx1[x]];
                arr[y * PAPER_SIZE + x] += amp * ((a + (b - a) * fx) * (1 - fy) + (c + (d - c) * fx) * fy);
            }
        }
    }
    function fbm(arr, count, octaves, amp, persistence, rand) {
        for (let o = 0; o < octaves; o++) {
            const c = Math.min(256, count << o);
            addNoise(arr, c, c, amp, rand);
            amp *= persistence;
        }
    }
    function normalise(arr, std) {
        let mean = 0;
        for (let i = 0; i < arr.length; i++) mean += arr[i];
        mean /= arr.length;
        let v = 0;
        for (let i = 0; i < arr.length; i++) v += (arr[i] - mean) * (arr[i] - mean);
        const sd = Math.sqrt(v / arr.length) || 1;
        for (let i = 0; i < arr.length; i++) arr[i] = clamp(0.5 + (arr[i] - mean) / sd * std, 0, 1);
    }
    // Plain weave with n threads per tile: warp runs along y, weft along x, the thread on top alternates per crossing.
    function weave(arr, n, irregular, slub, rand) {
        const pitch = PAPER_SIZE / n;
        const ampW = new Float32Array(n), ampF = new Float32Array(n), phW = new Float32Array(n), phF = new Float32Array(n);
        for (let i = 0; i < n; i++) {
            ampW[i] = 1 + irregular * (rand() * 2 - 1); ampF[i] = 1 + irregular * (rand() * 2 - 1);
            phW[i] = rand() * 2 * PI; phF[i] = rand() * 2 * PI;
        }
        for (let y = 0; y < PAPER_SIZE; y++) {
            const fy = y / pitch, iy = Math.floor(fy), ly = fy - iy;
            for (let x = 0; x < PAPER_SIZE; x++) {
                const fx = x / pitch, ix = Math.floor(fx), lx = fx - ix;
                const hw = (0.5 - 0.5 * Math.cos(2 * PI * lx)) * ampW[ix] * (1 + slub * Math.sin(2 * PI * (1 + ix % 3) * y / PAPER_SIZE + phW[ix]));
                const hf = (0.5 - 0.5 * Math.cos(2 * PI * ly)) * ampF[iy] * (1 + slub * Math.sin(2 * PI * (1 + iy % 3) * x / PAPER_SIZE + phF[iy]));
                arr[y * PAPER_SIZE + x] = ((ix + iy) & 1) === 0 ? Math.max(hw, 0.55 * hf) : Math.max(hf, 0.55 * hw);
            }
        }
    }
    const paperGen = {
        smooth(a, r) { fbm(a, 128, 2, 1, 0.5, r); },
        cold_press(a, r) { fbm(a, 16, 3, 1, 0.5, r); },
        rough(a, r) {
            fbm(a, 8, 3, 1, 0.6, r);
            for (let n = 0; n < 140; n++) { // a few soft pits
                const cx = (r() * PAPER_SIZE) | 0, cy = (r() * PAPER_SIZE) | 0, rad = 2 + r() * 2, depth = 0.3 + r() * 0.4;
                const reach = Math.ceil(rad);
                for (let dy = -reach; dy <= reach; dy++) for (let dx = -reach; dx <= reach; dx++) {
                    const q = Math.sqrt(dx * dx + dy * dy) / rad;
                    if (q < 1) a[((cy + dy) & 255) * PAPER_SIZE + ((cx + dx) & 255)] -= depth * (1 - q * q);
                }
            }
        },
        canvas(a, r) { weave(a, 32, 0.12, 0.05, r); addNoise(a, 128, 128, 0.05, r); },
        linen(a, r) { weave(a, 52, 0.25, 0.35, r); addNoise(a, 128, 128, 0.05, r); },
        laid(a, r) {
            for (let y = 0; y < PAPER_SIZE; y++) {
                const line = 0.5 + 0.5 * Math.cos(2 * PI * y * 64 / PAPER_SIZE);
                for (let x = 0; x < PAPER_SIZE; x++) {
                    const c = 0.5 + 0.5 * Math.cos(2 * PI * x * 4 / PAPER_SIZE);
                    a[y * PAPER_SIZE + x] = line * 0.6 + Math.pow(c, 16) * 0.25;
                }
            }
            fbm(a, 128, 1, 0.3, 0.5, r);
        },
        kraft(a, r) { addNoise(a, 4, 64, 1, r); addNoise(a, 8, 128, 0.6, r); addNoise(a, 32, 32, 0.2, r); },
        wood(a, r) {
            const warp = new Float32Array(PAPER_SIZE * PAPER_SIZE);
            addNoise(warp, 4, 8, 0.9, r);
            addNoise(warp, 8, 16, 0.3, r);
            for (let y = 0; y < PAPER_SIZE; y++) for (let x = 0; x < PAPER_SIZE; x++) {
                a[y * PAPER_SIZE + x] = 0.5 + 0.5 * Math.sin(2 * PI * (y * 24 / PAPER_SIZE + warp[y * PAPER_SIZE + x]));
            }
            addNoise(a, 8, 128, 0.3, r);
        }
    };
    const paperCache = Object.create(null);
    function makePaper(id, seed) {
        const def = PAPERS.find(p => p.id === id);
        if (!def) throw new Error('pixel paint: unknown paper ' + id);
        const sd = finite(seed, 1) | 0, key = id + '|' + sd;
        if (paperCache[key]) return paperCache[key];
        const height = new Float32Array(PAPER_SIZE * PAPER_SIZE);
        paperGen[id](height, rng(Lib.hashSeed(id + ':' + sd)));
        normalise(height, PAPER_STD[id]);
        return (paperCache[key] = { id, size: PAPER_SIZE, height, absorb: def.absorb, tooth: def.tooth, tint: def.tint.slice() });
    }
    let flatPaperObj = null;
    function flatPaper() {
        if (!flatPaperObj) flatPaperObj = { id: 'flat', size: PAPER_SIZE, height: new Float32Array(PAPER_SIZE * PAPER_SIZE).fill(0.5), absorb: 0.5, tooth: 0.5, tint: [255, 255, 255] };
        return flatPaperObj;
    }
    // Bilinear, tiled paper height at a doc position; scale = doc px per paper texel.
    function paperHeight(paper, x, y, scale) {
        const s = scale >= 0.25 ? scale : 0.25, u = x / s, v = y / s;
        const iu = Math.floor(u), iv = Math.floor(v), fx = u - iu, fy = v - iv, h = paper.height;
        const x0 = iu & 255, x1 = (iu + 1) & 255, r0 = (iv & 255) << 8, r1 = ((iv + 1) & 255) << 8;
        const a = h[r0 + x0], b = h[r0 + x1], c = h[r1 + x0], d = h[r1 + x1];
        return (a + (b - a) * fx) * (1 - fy) + (c + (d - c) * fx) * fy;
    }
    function renderPaper(paper, w, h, opts) {
        const o = opts || {}, scale = finite(o.scale, 1), relief = clamp(finite(o.relief, 0.5), 0, 1), tint = o.tint || paper.tint;
        const out = new Uint8ClampedArray(w * h * 4);
        for (let y = 0; y < h; y++) for (let x = 0; x < w; x++) {
            const hh = paperHeight(paper, x, y, scale);
            const shade = 1 + relief * 1.6 * (hh - paperHeight(paper, x - 1, y - 1, scale)) + relief * 0.10 * (hh - 0.5);
            const i = (y * w + x) * 4;
            out[i] = tint[0] * shade; out[i + 1] = tint[1] * shade; out[i + 2] = tint[2] * shade; out[i + 3] = 255;
        }
        return out;
    }

    // --- media table ---

    function medium(id, engine, tip, size, load, mix, wetness, grain, p) {
        return Object.freeze({
            id, engine, labelKey: 'pixel.art_medium_' + id, tip,
            defaults: Object.freeze({ size, load, mix, wetness, grain }), p: Object.freeze(p)
        });
    }
    const MEDIA = Object.freeze([
        medium('oil', 'bristle', 'flat', 36, 80, 60, 0, 50, { density: 1.3, dot: 0.9, pickup: 0.9, opacity: 0.95, deplete: 0.35, streak: 1, refill: 0.035 }),
        medium('acrylic', 'bristle', 'round', 28, 75, 25, 0, 40, { density: 1.3, dot: 1.2, pickup: 0.5, opacity: 0.95, deplete: 0.9, streak: 0.7, refill: 0.06 }),
        medium('gouache', 'bristle', 'round', 30, 85, 40, 0, 25, { density: 1.5, dot: 1.35, pickup: 0.6, opacity: 0.9, deplete: 0.5, streak: 0.3, refill: 0.045 }),
        medium('knife', 'bristle', 'knife', 60, 70, 85, 0, 30, { density: 1.0, dot: 0.6, pickup: 1.0, opacity: 1.0, deplete: 0.25, streak: 0.15, refill: 0.02 }),
        medium('blender', 'bristle', 'round', 40, 0, 100, 0, 0, { density: 1.0, dot: 1.2, pickup: 1.0, opacity: 0.85, deplete: 1.2, streak: 0.2, rate: 0.10, own: false }),
        medium('charcoal', 'dry', 'stick', 26, 70, 0, 0, 85, { hard: 0.35, press: 0.7, soft: 0.16, opacity: 0.5, maxAlpha: 0.97 }),
        medium('pencil', 'dry', 'point', 4, 60, 0, 0, 70, { hard: 0.55, press: 0.6, soft: 0.10, opacity: 0.42, maxAlpha: 0.9 }),
        medium('pastel', 'dry', 'stick', 34, 85, 0, 0, 60, { hard: 0.4, press: 0.95, soft: 0.2, opacity: 0.7, maxAlpha: 1 }),
        medium('crayon', 'dry', 'point', 14, 80, 0, 0, 75, { hard: 0.6, press: 0.75, soft: 0.12, opacity: 0.6, maxAlpha: 0.96 }),
        medium('marker', 'marker', 'chisel', 22, 70, 0, 0, 10, { density: 1 }),
        medium('watercolor', 'wet', 'round', 44, 45, 0, 70, 55, { water: 1.1, pigment: 0.16, deplete: 0.004 }),
        medium('ink', 'wet', 'round', 10, 95, 0, 25, 20, { water: 0.5, pigment: 0.9, deplete: 0.001 })
    ]);

    // --- brush factory and engine registry ---

    const engines = Object.create(null);
    function registerEngine(name, factory) {
        if (typeof name !== 'string' || typeof factory !== 'function') throw new Error('pixel paint: registerEngine(name, factory)');
        engines[name] = factory;
    }
    function normOpts(m, o0) {
        const o = o0 || {}, d = m.defaults, c = Array.isArray(o.color) || (o.color && o.color.length >= 3) ? o.color : [0, 0, 0];
        return {
            size: Math.max(0.5, finite(o.size, d.size)),
            color: [byte(finite(c[0], 0)), byte(finite(c[1], 0)), byte(finite(c[2], 0))],
            load: clamp(finite(o.load, d.load / 100), 0, 1), mix: clamp(finite(o.mix, d.mix / 100), 0, 1),
            wetness: clamp(finite(o.wetness, d.wetness / 100), 0, 1), grain: clamp(finite(o.grain, d.grain / 100), 0, 1),
            paper: o.paper || flatPaper(), paperScale: Math.max(0.25, finite(o.paperScale, 1)),
            seed: finite(o.seed, 1) | 0, docW: finite(o.docW, 0) | 0, docH: finite(o.docH, 0) | 0
        };
    }
    function createBrush(mediumId, opts) {
        const m = MEDIA.find(x => x.id === mediumId);
        if (!m) throw new Error('pixel paint: unknown medium ' + mediumId);
        const factory = engines[m.engine];
        if (!factory) throw new Error('pixel paint: engine ' + m.engine + ' is not loaded');
        const o = normOpts(m, opts);
        const brush = factory(m, o);
        brush.engine = m.engine;
        // Engines read these option fields live from the shared object, so a slider change needs no rebuild and keeps the carried paint.
        brush.configure = function configure(part) {
            const c = part || {};
            ['load', 'mix', 'wetness', 'grain'].forEach(key => { if (c[key] != null) o[key] = clamp(finite(c[key], o[key]), 0, 1); });
            if (c.paper) o.paper = c.paper;
            if (c.paperScale != null) o.paperScale = Math.max(0.25, finite(c.paperScale, o.paperScale));
            if (c.color) brush.setColor([byte(finite(c.color[0], 0)), byte(finite(c.color[1], 0)), byte(finite(c.color[2], 0))]);
            return brush;
        };
        return brush;
    }

    // Shared dirty-rect tracker: one per dab, reset at the start of the dab.
    function newDirty() { return { x0: 1e9, y0: 1e9, x1: -1e9, y1: -1e9 }; }
    function touch(d, x, y) {
        if (x < d.x0) d.x0 = x;
        if (y < d.y0) d.y0 = y;
        if (x + 1 > d.x1) d.x1 = x + 1;
        if (y + 1 > d.y1) d.y1 = y + 1;
    }
    function dirtyResult(d) { return d.x1 > d.x0 ? { x0: d.x0, y0: d.y0, x1: d.x1, y1: d.y1 } : null; }

    // --- bristle engine ---

    function bristleEngine(m, o) {
        const p = m.p, tip = m.tip, own = p.own !== false, rate = p.rate || 0.05, refill = p.refill || 0;
        const rand = rng((o.seed >>> 0) ^ 0x51ED270B);
        const colorPig = pigmentOf(o.color[0], o.color[1], o.color[2]);
        const pick = new Float32Array(8), rgb = [0, 0, 0], dirty = newDirty();
        let K = 0, refSize = 0, bu, bv, brf, btouch, btone, bload, bpig = [];
        const countFor = size => clamp(Math.round(size * p.density), 10, 240);

        function build(size) {
            const had = K > 0, avg = new Float32Array(8);
            let avgL = 0;
            if (had) {
                for (let k = 0; k < K; k++) { avgL += bload[k]; for (let n = 0; n < 8; n++) avg[n] += bpig[k][n]; }
                avgL /= K;
                for (let n = 0; n < 8; n++) avg[n] /= K;
            }
            K = countFor(size); refSize = size;
            bu = new Float32Array(K); bv = new Float32Array(K); brf = new Float32Array(K);
            btouch = new Float32Array(K); btone = new Float32Array(K); bload = new Float32Array(K); bpig = [];
            for (let k = 0; k < K; k++) {
                const r = Math.sqrt(rand()), th = rand() * 2 * PI;
                if (tip === 'knife') { bu[k] = clamp(-1 + 2 * (k + 0.5) / K + (rand() - 0.5) * 1.2 / K, -1, 1); bv[k] = 0.10 * (rand() * 2 - 1); }
                else if (tip === 'flat') { bu[k] = r * Math.cos(th); bv[k] = 0.28 * r * Math.sin(th); }
                else if (tip === 'fan') { const a = -0.6 + 1.2 * (k + 0.5) / K; bu[k] = Math.sin(a) / Math.sin(0.6); bv[k] = 0.5 * (1 - Math.cos(a)) / (1 - Math.cos(0.6)) - 0.5; }
                else { bu[k] = r * Math.cos(th); bv[k] = r * Math.sin(th); }
                brf[k] = 0.7 + 0.6 * rand(); btouch[k] = 0.6 * rand(); btone[k] = rand() * 2 - 1;
                bpig.push(new Float32Array(had ? avg : colorPig));
                bload[k] = had ? avgL : 0;
            }
        }
        build(o.size);

        function begin(keepDirty) {
            for (let k = 0; k < K; k++) {
                bload[k] = own ? o.load * (0.55 + 0.45 * rand()) : 0;
                if (keepDirty) pigmentMix(bpig[k], bpig[k], colorPig, 0.5 * o.load);
                else bpig[k].set(colorPig);
            }
        }
        begin(false);

        function dotRadius(size, kk, rfactor) { return Math.max(0.7, size / 2 * p.dot / Math.sqrt(kk) * rfactor); }
        function reach(size) {
            const kmin = countFor(size / 1.25);
            return size / 2 + dotRadius(size, kmin, 1.69) + 2; // radius factor 1.3 * load factor 1.3
        }
        function spacing(size) {
            return Math.max(0.5, Math.min(size * 0.04, dotRadius(size, countFor(size), 1) * 0.5));
        }

        function dab(buf, x, y, size, pressure, angle, flow) {
            if (!(size > 0)) return null;
            if (Math.abs(size - refSize) > 0.25 * refSize) build(size);
            pressure = clamp(finite(pressure, 1), 0, 1);
            flow = clamp(finite(flow, 1), 0, 1);
            const R = size / 2, sc = R * (0.75 + 0.25 * pressure), ta = angle + PI / 2;
            const wx = Math.cos(ta), wy = Math.sin(ta), dx = Math.cos(angle), dy = Math.sin(angle);
            const data = buf.data, sel = buf.sel, wetMap = buf.wetMap || null, bw = buf.w, bh = buf.h, bx0 = buf.x, by0 = buf.y, lock = !!buf.lockAlpha, dw = buf.docW;
            const hgt = o.paper.height, pscale = o.paperScale, mixK = 0.45 * o.mix;
            dirty.x0 = 1e9; dirty.y0 = 1e9; dirty.x1 = -1e9; dirty.y1 = -1e9;
            for (let k = 0; k < K; k++) {
                if (pressure + 0.4 < btouch[k]) continue;
                const cx = x + sc * (bu[k] * wx + bv[k] * dx), cy = y + sc * (bu[k] * wy + bv[k] * dy);
                // a loaded brush paints a closed surface, a dry one separate streaks
                const rb = Math.max(0.7, dotRadius(size, K, brf[k]) * (0.7 + 0.6 * bload[k]));
                // pickup samples the canvas AHEAD of the bristle: under it lies its own fresh paint
                const sx = Math.floor(cx + dx * (rb + 1.5)), sy = Math.floor(cy + dy * (rb + 1.5)), lx = sx - bx0, ly = sy - by0;
                let ca = 0, ci = 0;
                if (lx >= 0 && ly >= 0 && lx < bw && ly < bh) {
                    ci = (ly * bw + lx) * 4; ca = data[ci + 3] / 255;
                    if (own && wetMap) ca *= wetMap[sy * dw + sx] / 255; // dry paint is not picked up; the blender smears anything
                }
                const h = paperHeight(o.paper, cx, cy, pscale);
                const thr = (1 - bload[k]) * o.grain * 1.1;
                const c = smoothstep(thr - 0.12, thr + 0.12, h + 0.12);
                const pg = bpig[k];
                if (ca > 0 && o.mix > 0) {
                    if (bload[k] < 0.02 && ca > 0.05) { // an empty bristle (blender) takes the canvas colour first
                        pigmentOfByte(data[ci], data[ci + 1], data[ci + 2], pg);
                        bload[k] = 0.3 * ca;
                    } else {
                        const kk = o.mix * p.pickup * rate * ca * c;
                        if (kk > 0.0005) {
                            pigmentOfByte(data[ci], data[ci + 1], data[ci + 2], pick);
                            pigmentMix(pg, pg, pick, kk);
                            bload[k] = Math.min(1, bload[k] + kk * 0.6);
                        }
                    }
                }
                // the reservoir behind the tip keeps feeding fresh paint: the tip colour relaxes back to the brush colour
                if (own && refill > 0 && bload[k] > 0.05) pigmentMix(pg, pg, colorPig, refill * o.load);
                // a dry-ish bristle catches on the paper per pixel, a loaded one per bristle
                const dryish = (1 - bload[k]) * o.grain > 0.2;
                const a = flow * p.opacity * (dryish ? 1 : c) * (0.15 + 0.85 * bload[k]);
                if (a >= 0.004 && bload[k] >= 0.02) {
                    pigmentToRgb(pg, rgb);
                    const f = 1 + btone[k] * p.streak * 0.16;
                    const r0 = Math.min(255, rgb[0] * f), g0 = Math.min(255, rgb[1] * f), b0 = Math.min(255, rgb[2] * f);
                    rgb[0] = r0; rgb[1] = g0; rgb[2] = b0;
                    const px0 = Math.max(bx0, Math.floor(cx - rb)), px1 = Math.min(bx0 + bw - 1, Math.ceil(cx + rb));
                    const py0 = Math.max(by0, Math.floor(cy - rb)), py1 = Math.min(by0 + bh - 1, Math.ceil(cy + rb));
                    const rb2 = rb * rb, core2 = rb2 * 0.3025;
                    for (let py = py0; py <= py1; py++) {
                        const ddy = py + 0.5 - cy;
                        for (let px = px0; px <= px1; px++) {
                            const ddx = px + 0.5 - cx, d2 = ddx * ddx + ddy * ddy;
                            if (d2 >= rb2) continue;
                            let w = d2 < core2 ? a : a * (1 - smoothstep(0.55, 1, Math.sqrt(d2) / rb)); // no sqrt inside the solid core
                            if (dryish) {
                                const hp = hgt[((((py / pscale) | 0) & 255) << 8) + (((px / pscale) | 0) & 255)];
                                w *= smoothstep(thr - 0.12, thr + 0.12, hp + 0.12);
                            }
                            if (sel) { const s = sel[py * dw + px]; if (s === 0) continue; w *= s / 255; }
                            if (w < 0.002) continue;
                            const i = ((py - by0) * bw + (px - bx0)) * 4, di = py * dw + px;
                            const wp = wetMap ? wetMap[di] / 255 : 1, pa = data[i + 3];
                            // fresh paint blends into wet paint instead of covering it
                            const wc = pa > 0 ? w * (1 - mixK * wp) : w;
                            if (pa > 0 && Math.abs(data[i] - r0) <= 2 && Math.abs(data[i + 1] - g0) <= 2 && Math.abs(data[i + 2] - b0) <= 2) {
                                // the bristle repaints its own track: the colour is already there, only alpha may rise
                                if (!lock && pa < 255) { data[i + 3] = Math.round((wc + pa / 255 * (1 - wc)) * 255); touch(dirty, px, py); }
                            } else if (!depositPixel(data, i, pg, rgb, wc, lock, 1)) continue;
                            else touch(dirty, px, py);
                            if (wetMap) { const wm = Math.round(w * 255); if (wm > wetMap[di]) wetMap[di] = wm; }
                        }
                    }
                }
                bload[k] *= 1 - p.deplete * 0.012 * (0.4 + 0.6 * c);
            }
            return dirtyResult(dirty);
        }
        function carried() {
            let r = 0, g = 0, b = 0;
            for (let k = 0; k < K; k++) { pigmentToRgb(bpig[k], rgb); r += rgb[0]; g += rgb[1]; b += rgb[2]; }
            return [Math.round(r / K), Math.round(g / K), Math.round(b / K)];
        }
        function setColor(c) { pigmentOf(c[0], c[1], c[2], colorPig); }
        return { setColor, begin, reach, spacing, dab, carried };
    }

    // --- dry engine ---

    function hash3(x, y, z) {
        let h = (Math.imul(x, 374761393) + Math.imul(y, 668265263) + Math.imul(z, 1274126177)) | 0;
        h = Math.imul(h ^ (h >>> 13), 1103515245);
        return ((h ^ (h >>> 16)) >>> 0) / 4294967296;
    }
    function dryEngine(m, o) {
        const p = m.p, stick = m.tip === 'stick';
        const pig = pigmentOf(o.color[0], o.color[1], o.color[2]), rgb = [o.color[0], o.color[1], o.color[2]], dirty = newDirty();
        let dabIndex = 0;
        function dab(buf, x, y, size, pressure, angle, flow) {
            if (!(size > 0)) return null;
            pressure = clamp(finite(pressure, 1), 0, 1);
            flow = clamp(finite(flow, 1), 0, 1);
            const R = size / 2, along = stick ? R * 0.45 : R, ca = Math.cos(angle), sa = Math.sin(angle);
            const data = buf.data, sel = buf.sel, bw = buf.w, bh = buf.h, bx0 = buf.x, by0 = buf.y, lock = !!buf.lockAlpha, dw = buf.docW;
            const thr = 1 - pressure * p.press - o.load * 0.25, grainK = 0.4 + 0.6 * o.grain, grainB = (1 - o.grain) * 0.6;
            const px0 = Math.max(bx0, Math.floor(x - R - 1)), px1 = Math.min(bx0 + bw - 1, Math.ceil(x + R + 1));
            const py0 = Math.max(by0, Math.floor(y - R - 1)), py1 = Math.min(by0 + bh - 1, Math.ceil(y + R + 1));
            dirty.x0 = 1e9; dirty.y0 = 1e9; dirty.x1 = -1e9; dirty.y1 = -1e9;
            dabIndex++;
            for (let py = py0; py <= py1; py++) {
                for (let px = px0; px <= px1; px++) {
                    const ox = px + 0.5 - x, oy = py + 0.5 - y, a = (ox * ca + oy * sa) / along, b = (-ox * sa + oy * ca) / R;
                    const q = Math.sqrt(a * a + b * b);
                    if (q >= 1) continue;
                    const fall = 1 - smoothstep(p.hard, 1, q);
                    const h = paperHeight(o.paper, px, py, o.paperScale);
                    const c = smoothstep(thr - p.soft, thr + p.soft, h * grainK + grainB);
                    let w = flow * p.opacity * fall * c * (0.55 + 0.45 * hash3(px, py, dabIndex));
                    if (sel) { const s = sel[py * dw + px]; if (s === 0) continue; w *= s / 255; }
                    if (w < 0.003) continue;
                    if (depositPixel(data, ((py - by0) * bw + (px - bx0)) * 4, pig, rgb, w, lock, p.maxAlpha)) touch(dirty, px, py);
                }
            }
            return dirtyResult(dirty);
        }
        return {
            setColor(c) { pigmentOf(c[0], c[1], c[2], pig); rgb[0] = byte(c[0]); rgb[1] = byte(c[1]); rgb[2] = byte(c[2]); },
            begin() { },
            reach(size) { return size / 2 + 2; },
            spacing(size) { return Math.max(0.5, size * 0.09); },
            dab,
            carried() { return [rgb[0], rgb[1], rgb[2]]; }
        };
    }

    // --- marker engine ---

    function markerEngine(m, o) {
        const p = m.p, ink = pigmentOf(o.color[0], o.color[1], o.color[2]), rgb = [o.color[0], o.color[1], o.color[2]];
        const dirty = newDirty(), tmp = [0, 0, 0], base = new Float32Array(8), px8 = new Float32Array(8), out8 = new Float32Array(8);
        const C45 = Math.SQRT1_2;
        let cover = null, orig = null, coverW = 0, coverH = 0;
        let sx0 = 1e9, sy0 = 1e9, sx1 = -1e9, sy1 = -1e9; // box touched by the current stroke
        function ensure(buf) {
            const w = o.docW || buf.docW, h = o.docH || buf.docH;
            if (!cover || coverW !== w || coverH !== h) { coverW = w; coverH = h; cover = new Uint8Array(w * h); orig = new Uint8Array(w * h * 4); sx0 = 1e9; sx1 = -1e9; }
        }
        function begin() {
            if (!cover || sx1 <= sx0) return;
            for (let y = Math.max(0, sy0); y < Math.min(coverH, sy1); y++) cover.fill(0, y * coverW + Math.max(0, sx0), y * coverW + Math.min(coverW, sx1));
            sx0 = 1e9; sy0 = 1e9; sx1 = -1e9; sy1 = -1e9;
        }
        function dab(buf, x, y, size, pressure, angle, flow) {
            if (!(size > 0)) return null;
            flow = clamp(finite(flow, 1), 0, 1);
            ensure(buf);
            const R = size / 2, data = buf.data, sel = buf.sel, bw = buf.w, bh = buf.h, bx0 = buf.x, by0 = buf.y, lock = !!buf.lockAlpha, dw = buf.docW;
            const dens = p.density * o.load * 1.4;
            const px0 = Math.max(bx0, Math.floor(x - R - 1)), px1 = Math.min(bx0 + bw - 1, Math.ceil(x + R + 1));
            const py0 = Math.max(by0, Math.floor(y - R - 1)), py1 = Math.min(by0 + bh - 1, Math.ceil(y + R + 1));
            dirty.x0 = 1e9; dirty.y0 = 1e9; dirty.x1 = -1e9; dirty.y1 = -1e9;
            for (let py = py0; py <= py1; py++) {
                for (let px = px0; px <= px1; px++) {
                    const ox = px + 0.5 - x, oy = py + 0.5 - y, a = (ox * C45 + oy * C45) / R, b = (-ox * C45 + oy * C45) / (R * 0.4);
                    const q = Math.sqrt(a * a + b * b);
                    if (q >= 1) continue;
                    const fall = 1 - smoothstep(0.8, 1, q), s = sel ? sel[py * dw + px] : 255;
                    const want = Math.round(255 * fall * flow * s / 255), idx = py * coverW + px, prev = cover[idx];
                    if (want <= prev) continue;
                    cover[idx] = want;
                    if (px < sx0) sx0 = px;
                    if (py < sy0) sy0 = py;
                    if (px + 1 > sx1) sx1 = px + 1;
                    if (py + 1 > sy1) sy1 = py + 1;
                    const i = ((py - by0) * bw + (px - bx0)) * 4, o4 = idx * 4;
                    // The pixel as it was before this stroke: every write is absolute (base + glaze for the coverage so far),
                    // so one stroke never darkens itself and 8-bit rounding cannot drift over many small steps.
                    if (prev === 0) { orig[o4] = data[i]; orig[o4 + 1] = data[i + 1]; orig[o4 + 2] = data[i + 2]; orig[o4 + 3] = data[i + 3]; }
                    const pa = orig[o4 + 3] / 255;
                    if (lock && pa === 0) continue;
                    // the substrate seen over white, so partial alpha behaves like paper showing through
                    if (pa >= 0.999) pigmentOfByte(orig[o4], orig[o4 + 1], orig[o4 + 2], base);
                    else if (pa < 0.004) base.set(WHITE);
                    else { pigmentOfByte(orig[o4], orig[o4 + 1], orig[o4 + 2], px8); pigmentMix(base, WHITE, px8, pa); }
                    glaze(out8, base, ink, dens * want / 255);
                    pigmentToRgb(out8, tmp);
                    let na = pa;
                    if (!lock) {
                        na = Math.max(pa + (1 - pa) * (want / 255) * 0.9, 1 - Math.min(tmp[0], tmp[1], tmp[2]) / 255);
                        if (na > 1) na = 1;
                        if (na < 0.004) continue;
                        data[i + 3] = Math.round(na * 255);
                    }
                    if (na >= 0.999) { data[i] = tmp[0]; data[i + 1] = tmp[1]; data[i + 2] = tmp[2]; }
                    else {
                        const k = 255 * (1 - na);
                        data[i] = (tmp[0] - k) / na; data[i + 1] = (tmp[1] - k) / na; data[i + 2] = (tmp[2] - k) / na;
                    }
                    touch(dirty, px, py);
                }
            }
            return dirtyResult(dirty);
        }
        return {
            setColor(c) { pigmentOf(c[0], c[1], c[2], ink); rgb[0] = byte(c[0]); rgb[1] = byte(c[1]); rgb[2] = byte(c[2]); },
            begin,
            reach(size) { return size / 2 + 2; },
            spacing(size) { return Math.max(0.5, size * 0.06); },
            dab,
            carried() { return [rgb[0], rgb[1], rgb[2]]; }
        };
    }

    registerEngine('bristle', bristleEngine);
    registerEngine('dry', dryEngine);
    registerEngine('marker', markerEngine);

    // --- stroke preview (medium buttons) ---

    function previewStroke(mediumId, w, h, opts) {
        const o = opts || {}, m = MEDIA.find(x => x.id === mediumId);
        if (!m) throw new Error('pixel paint: unknown medium ' + mediumId);
        const seed = finite(o.seed, 1) | 0, paper = o.paper || makePaper('cold_press', seed);
        const data = renderPaper(paper, w, h, {});
        if (!engines[m.engine] || (m.engine === 'wet' && typeof Lib.paint.createWet !== 'function')) return data;
        const dark = m.id === 'charcoal' || m.id === 'pencil';
        const base = Math.min(m.defaults.size, h * 0.4);
        const brush = createBrush(mediumId, { size: base, color: o.color || (dark ? [40, 40, 44] : [40, 80, 170]), paper, seed, docW: w, docH: h });
        const buf = { x: 0, y: 0, w, h, data, sel: null, docW: w, docH: h, lockAlpha: false };
        const wet = m.engine === 'wet' ? Lib.paint.createWet({ docW: w, docH: h, paper, paperScale: 1 }) : null;
        const snapshot = wet ? { x: 0, y: 0, w, h, data: new Uint8ClampedArray(data), sel: null, docW: w, docH: h, lockAlpha: false } : null;
        brush.begin(false);
        const N = 400;
        let px = 0, py = 0, travelled = 1e9;
        for (let n = 0; n <= N; n++) {
            const t = n / N, x = w * (0.1 + 0.8 * t), y = h * (0.5 - 0.22 * Math.sin(2 * PI * t));
            const press = 0.25 + 0.75 * Math.sin(PI * t), size = base * (0.5 + 0.5 * press);
            if (n > 0) travelled += Math.sqrt((x - px) * (x - px) + (y - py) * (y - py));
            const ahead = w * (0.1 + 0.8 * Math.min(1, (n + 1) / N)) - x, aheadY = h * (0.5 - 0.22 * Math.sin(2 * PI * Math.min(1, (n + 1) / N))) - y;
            if (travelled >= brush.spacing(size)) {
                brush.dab(wet || buf, x, y, size, press, Math.atan2(aheadY, ahead || 1e-6), 1);
                travelled = 0;
            }
            px = x; py = y;
        }
        if (wet) {
            for (let s = 0; s < 200; s++) if (!wet.step()) break;
            wet.render(buf, snapshot);
        }
        return data;
    }

    Lib.paint = Object.assign(Lib.paint || {}, {
        PIGMENT, srgbToLinear, linearToSrgb, pigmentOf, pigmentMix, pigmentToRgb, mixRgb, glaze,
        PAPERS, makePaper, paperHeight, renderPaper, MEDIA, registerEngine, createBrush, previewStroke
    });
})(typeof self !== 'undefined' ? self : (typeof globalThis !== 'undefined' ? globalThis : this));
