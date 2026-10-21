// Pixel pure image library, generators part (WP6). DOM-free; loads in a window, in the Pixel worker and in a node vm.
//
// gen.render { id, params, width, height, refWidth?, refHeight? } -> { image, width, height, data } (the same typed array
// twice, so callers can use either shape). Output is deterministic for a given id, params and size: every random choice
// is derived from params.seed with integer hashes (no unseeded randomness anywhere).
//
// Coordinates: the short side of the reference size (refWidth x refHeight, default = the render size) is one unit, so the
// long side runs to ax = max(1, ref aspect). Every length-like parameter is relative to that unit (or to an explicit count of
// tiles across the short side), which makes a small preview render look like the full-size render with the same
// reference size.
//
// Lib.GENERATORS is the catalog ({ id, cat, labelKey, fallback, params: schema for runtime.buildParamControls,
// presets: [{ id, labelKey, fallback, values }], render }); pixel-generators.js builds the panel from it.
(function (root) {
    'use strict';

    const Lib = root.AuraPixelLib = root.AuraPixelLib || {};
    if (typeof Lib.registerOp !== 'function') return;

    const TAU = Math.PI * 2;
    const SQ3 = Math.sqrt(3);
    const SQ3_2 = SQ3 / 2;
    const INV32 = 1 / 4294967296;
    const LUT_SIZE = 1024;
    const MAX_SIDE = 16384;
    const CANCELLED = new Error('cancelled');

    // --- numbers ---

    const clamp = (v, lo, hi) => (v < lo ? lo : (v > hi ? hi : v));
    const clamp01 = v => (v < 0 ? 0 : (v > 1 ? 1 : v));
    const fract = v => v - Math.floor(v);
    const wrapInt = (v, p) => { const m = v % p; return m < 0 ? m + p : m; };
    const mixv = (a, b, t) => a + (b - a) * t;
    const fade = t => t * t * t * (t * (t * 6 - 15) + 10);

    function smoothstep(e0, e1, x) {
        const t = clamp01((x - e0) / ((e1 - e0) || 1e-9));
        return t * t * (3 - 2 * t);
    }

    // --- hashing: integer lattice -> uint32 ---

    function mix32(x) {
        x ^= x >>> 16;
        x = Math.imul(x, 0x7FEB352D);
        x ^= x >>> 15;
        x = Math.imul(x, 0x846CA68B);
        return x ^ (x >>> 16);
    }

    function hash2(x, y, s) {
        return mix32((mix32(x ^ s) + Math.imul(y, 0x9E3779B1)) | 0) >>> 0;
    }

    function seedOf(seed, salt) {
        return mix32(((seed >>> 0) ^ Math.imul((salt | 0) + 1, 0x85EBCA6B)) | 0) | 0;
    }

    const rand01 = (x, y, s) => hash2(x, y, s) * INV32;

    // --- noise primitives: f(x, y, periodX, periodY) -> about [-1, 1]; a period > 0 makes the lattice wrap (tileable) ---

    const GRAD_X = new Float64Array(256), GRAD_Y = new Float64Array(256);
    for (let i = 0; i < 256; i++) {
        GRAD_X[i] = Math.cos(i * TAU / 256);
        GRAD_Y[i] = Math.sin(i * TAU / 256);
    }

    // Per primitive: norm scales the raw output to a peak of about 1; std = standard deviation of the normalised output,
    // amean/astd = mean and deviation of its absolute value. Measured on 2e6 samples (scripts/test-pixel-gen.mjs re-checks
    // them), so fBm can be scaled to a stable contrast whatever the octave settings.
    const CAL = {
        value: { norm: 1, std: 0.4516, amean: 0.3778, astd: 0.2473 },
        perlin: { norm: 1.4142, std: 0.3056, amean: 0.2497, astd: 0.1762 },
        simplex: { norm: 16, std: 0.539, amean: 0.4672, astd: 0.269 }
    };

    function valueNoise(seed, mode) {
        const s = seedOf(seed, 1);
        const m = mode || 'smooth';
        return function value(x, y, px, py) {
            const xf = Math.floor(x), yf = Math.floor(y);
            let tx = x - xf, ty = y - yf;
            let x0 = xf, y0 = yf, x1 = xf + 1, y1 = yf + 1;
            if (px) { x0 = wrapInt(x0, px); x1 = wrapInt(x1, px); }
            if (py) { y0 = wrapInt(y0, py); y1 = wrapInt(y1, py); }
            if (m === 'smooth') { tx = fade(tx); ty = fade(ty); }
            else if (m === 'cubic') { tx = tx * tx * (3 - 2 * tx); ty = ty * ty * (3 - 2 * ty); }
            else if (m === 'step') { tx = 0; ty = 0; }
            const a = hash2(x0, y0, s) * INV32, b = hash2(x1, y0, s) * INV32;
            const c = hash2(x0, y1, s) * INV32, d = hash2(x1, y1, s) * INV32;
            const top = a + (b - a) * tx, bot = c + (d - c) * tx;
            return (top + (bot - top) * ty) * 2 - 1;
        };
    }

    function perlinNoise(seed) {
        const s = seedOf(seed, 2);
        const norm = CAL.perlin.norm;
        return function perlin(x, y, px, py) {
            const xf = Math.floor(x), yf = Math.floor(y);
            const fx = x - xf, fy = y - yf;
            let x0 = xf, y0 = yf, x1 = xf + 1, y1 = yf + 1;
            if (px) { x0 = wrapInt(x0, px); x1 = wrapInt(x1, px); }
            if (py) { y0 = wrapInt(y0, py); y1 = wrapInt(y1, py); }
            const g00 = hash2(x0, y0, s) >>> 24, g10 = hash2(x1, y0, s) >>> 24;
            const g01 = hash2(x0, y1, s) >>> 24, g11 = hash2(x1, y1, s) >>> 24;
            const d00 = GRAD_X[g00] * fx + GRAD_Y[g00] * fy;
            const d10 = GRAD_X[g10] * (fx - 1) + GRAD_Y[g10] * fy;
            const d01 = GRAD_X[g01] * fx + GRAD_Y[g01] * (fy - 1);
            const d11 = GRAD_X[g11] * (fx - 1) + GRAD_Y[g11] * (fy - 1);
            const u = fade(fx), v = fade(fy);
            const top = d00 + (d10 - d00) * u, bot = d01 + (d11 - d01) * u;
            return (top + (bot - top) * v) * norm;
        };
    }

    // Simplex noise on an equilateral triangle lattice with unit edges (same family as OpenSimplex2): every sample sums the
    // three corners of its triangle with a radial kernel (0.75 - d^2)^4 that vanishes exactly where the next corner starts.
    // x/y are in lattice units. Tileable: the lattice repeats every n edges horizontally and every k rows (k * sqrt(3) units)
    // vertically, which is a lattice translation, so the wrap is exact.
    function simplexNoise(seed) {
        const s = seedOf(seed, 3);
        const norm = CAL.simplex.norm;
        function corner(ci, cj, x, y, n, k) {
            const dx = x - (ci + cj * 0.5), dy = y - cj * SQ3_2;
            const t = 0.75 - dx * dx - dy * dy;
            if (t <= 0) return 0;
            let wi = ci, wj = cj;
            if (k) { const q = Math.floor(wj / (2 * k)); wj -= 2 * k * q; wi += k * q; }
            if (n) wi = wrapInt(wi, n);
            const g = hash2(wi, wj, s) >>> 24;
            const t2 = t * t;
            return t2 * t2 * (GRAD_X[g] * dx + GRAD_Y[g] * dy);
        }
        return function simplex(x, y, n, k) {
            const jf = y / SQ3_2;
            const xs = x - jf * 0.5;
            const i0 = Math.floor(xs), j0 = Math.floor(jf);
            const up = (xs - i0) + (jf - j0) >= 1 ? 1 : 0;
            return (corner(i0 + up, j0 + up, x, y, n, k) + corner(i0 + 1, j0, x, y, n, k) + corner(i0, j0 + 1, x, y, n, k)) * norm;
        };
    }

    function makeBase(type, seed, mode) {
        if (type === 'value') return valueNoise(seed, mode);
        if (type === 'simplex') return simplexNoise(seed);
        return perlinNoise(seed);
    }

    // Worley / cellular noise. fn(x, y, px, py) fills fn.out = { f1, f2, id, fx, fy } (distances to the nearest and second
    // nearest feature point, a uint32 id of the nearest cell and its feature point). metric: 0 euclid, 1 manhattan, 2 chebyshev.
    function worleyNoise(seed, jitter, metric) {
        const s1 = seedOf(seed, 4), s2 = seedOf(seed, 5);
        const jit = clamp(jitter == null ? 1 : jitter, 0, 1);
        const out = { f1: 0, f2: 0, id: 0, fx: 0, fy: 0 };
        let f1 = 0, f2 = 0, id = 0, bx = 0, by = 0, qx = 0, qy = 0, wpx = 0, wpy = 0;
        function visit(cx, cy) {
            const hx = wpx ? wrapInt(cx, wpx) : cx, hy = wpy ? wrapInt(cy, wpy) : cy;
            const h = hash2(hx, hy, s1);
            const px = cx + 0.5 + (h * INV32 - 0.5) * jit;
            const py = cy + 0.5 + (hash2(hx, hy, s2) * INV32 - 0.5) * jit;
            const dx = px - qx, dy = py - qy;
            let d;
            if (metric === 1) d = Math.abs(dx) + Math.abs(dy);
            else if (metric === 2) d = Math.max(Math.abs(dx), Math.abs(dy));
            else d = dx * dx + dy * dy;
            if (d < f1) { f2 = f1; f1 = d; id = h; bx = px; by = py; }
            else if (d < f2) f2 = d;
        }
        function fn(x, y, px, py) {
            const ix = Math.floor(x), iy = Math.floor(y);
            qx = x; qy = y; wpx = px || 0; wpy = py || 0;
            f1 = f2 = 1e9; id = 0;
            for (let j = -1; j <= 1; j++) for (let i = -1; i <= 1; i++) visit(ix + i, iy + j);
            const fx = x - ix, fy = y - iy;
            // A point two cells away can only win when the second distance is longer than the 3x3 block guarantees.
            const bound = 1 + Math.min(fx, 1 - fx, fy, 1 - fy);
            const second = metric === 0 ? Math.sqrt(f2) : f2;
            if (second > bound) {
                for (let j = -2; j <= 2; j++) {
                    for (let i = -2; i <= 2; i++) {
                        if (i > -2 && i < 2 && j > -2 && j < 2) continue;
                        visit(ix + i, iy + j);
                    }
                }
            }
            out.f1 = metric === 0 ? Math.sqrt(f1) : f1;
            out.f2 = metric === 0 ? Math.sqrt(f2) : f2;
            out.id = id; out.fx = bx; out.fy = by;
            return out;
        }
        fn.out = out;
        return fn;
    }

    // --- fields built from the primitives ---

    // Lattice layout: image units -> lattice coordinates, plus the lattice period when the field has to tile.
    function noiseLayout(type, cells, fr, tile) {
        const c = Math.max(0.25, cells);
        if (!tile) return { sx: c, sy: c, nx: 0, ny: 0 };
        const nx = Math.max(1, Math.round(c * fr.ax));
        if (type === 'simplex') {
            const ny = Math.max(1, Math.round(c * fr.ay / SQ3));
            return { sx: nx / fr.ax, sy: ny * SQ3 / fr.ay, nx, ny };
        }
        const ny = Math.max(1, Math.round(c * fr.ay));
        return { sx: nx / fr.ax, sy: ny / fr.ay, nx, ny };
    }

    // Fractal noise. o: { type, seed, cells (features across the short side), octaves, lacunarity, gain, kind, tile,
    // offsetX, offsetY, ridge }. Returns { t(u, v) -> about 0..1, f(u, v) -> zero centred, std }.
    // kind: fbm (plain), ridged (multifractal ridges), turbulence (sum of |n|), billow (puffy |n|).
    function makeFbm(o, fr) {
        const type = o.type || 'perlin';
        const cal = CAL[type] || CAL.perlin;
        const octaves = clamp(Math.round(o.octaves || 1), 1, 12);
        const tile = !!o.tile;
        const lac = tile ? Math.max(2, Math.round(o.lacunarity || 2)) : Math.max(1.05, o.lacunarity || 2);
        const gain = o.gain == null ? 0.5 : o.gain;
        const lay = noiseLayout(type, o.cells, fr, tile);
        const kind = o.kind || 'fbm';
        const ox = o.offsetX || 0, oy = o.offsetY || 0;
        const ridgeOffset = o.ridge == null ? 1 : o.ridge;
        const bases = [], amps = [], muls = [];
        let amp = 1, mul = 1, norm = 0, normSq = 0;
        for (let i = 0; i < octaves; i++) {
            bases.push(makeBase(type, seedOf(o.seed, 100 + i), o.interp));
            amps.push(amp); muls.push(mul);
            norm += amp; normSq += amp * amp;
            amp *= gain; mul *= lac;
        }
        const sx = lay.sx, sy = lay.sy, nx = lay.nx, ny = lay.ny;
        const spread = Math.sqrt(normSq) / norm;
        let f, t;
        if (kind === 'ridged') {
            f = (u, v) => {
                let sum = 0, weight = 1;
                for (let i = 0; i < octaves; i++) {
                    const m = muls[i];
                    let s = ridgeOffset - Math.abs(bases[i]((u + ox) * sx * m, (v + oy) * sy * m, nx * m, ny * m));
                    s *= s;
                    s *= weight;
                    weight = clamp01(s * 1.6);
                    sum += s * amps[i];
                }
                return sum / norm;
            };
            t = (u, v) => f(u, v) * 1.25;
        } else if (kind === 'turbulence' || kind === 'billow') {
            const billow = kind === 'billow';
            f = (u, v) => {
                let sum = 0;
                for (let i = 0; i < octaves; i++) {
                    const m = muls[i];
                    sum += Math.abs(bases[i]((u + ox) * sx * m, (v + oy) * sy * m, nx * m, ny * m)) * amps[i];
                }
                return sum / norm;
            };
            // |n| has mean amean and deviation astd per octave; scale so the result spans roughly 0..1 like plain fBm.
            const mean = cal.amean, dev = cal.astd * spread;
            t = billow ? (u, v) => 0.5 - (f(u, v) - mean) / (4 * dev) : (u, v) => 0.5 + (f(u, v) - mean) / (4 * dev);
        } else {
            f = (u, v) => {
                let sum = 0;
                for (let i = 0; i < octaves; i++) {
                    const m = muls[i];
                    sum += bases[i]((u + ox) * sx * m, (v + oy) * sy * m, nx * m, ny * m) * amps[i];
                }
                return sum / norm;
            };
            const inv = 1 / (4 * cal.std * spread);
            t = (u, v) => 0.5 + f(u, v) * inv;
        }
        return { t, f, std: cal.std * spread };
    }

    // Domain warp: the lookup position is pushed by two noise fields, `iterations` times nested (f(p + w(p + w(p)))).
    // strength is in features of the main field (1 = one feature).
    function makeWarp(o, fr) {
        const strength = (o.strength == null ? 1 : o.strength) / Math.max(0.25, o.cells);
        const iterations = clamp(Math.round(o.iterations || 1), 1, 4);
        const common = { type: o.type, cells: Math.max(0.25, o.cells * (o.warpScale || 1)), octaves: o.warpOctaves || 3, lacunarity: o.lacunarity, gain: 0.5, tile: o.tile };
        const wx = makeFbm(Object.assign({ seed: seedOf(o.seed, 900) }, common), fr);
        const wy = makeFbm(Object.assign({ seed: seedOf(o.seed, 901) }, common), fr);
        const k = 1 / (wx.std * 2);
        const shift = o.tile ? 0 : 5.2 / Math.max(0.25, o.cells);
        return function warp(u, v, out) {
            let pu = u, pv = v;
            for (let i = 0; i < iterations; i++) {
                const du = wx.f(pu + shift * 0.3, pv) * k, dv = wy.f(pu, pv + shift * 0.7) * k;
                pu = u + du * strength;
                pv = v + dv * strength;
            }
            out.u = pu; out.v = pv;
            return out;
        };
    }

    // --- colour lookup ---

    // Gradient stops -> float RGBA table (0..255), so dithering can break up banding in smooth ramps.
    function buildLUT(stops, size) {
        const n = size || LUT_SIZE;
        const lut = new Float32Array(n * 4);
        let list = (Array.isArray(stops) ? stops : []).map(stop => {
            const rgba = Lib.parseColor(stop.color) || [0, 0, 0, 255];
            return { pos: clamp01(Number(stop.pos) || 0), r: rgba[0], g: rgba[1], b: rgba[2], a: stop.alpha != null ? clamp01(Number(stop.alpha)) * 255 : rgba[3] };
        }).sort((p, q) => p.pos - q.pos);
        if (!list.length) list = [{ pos: 0, r: 0, g: 0, b: 0, a: 255 }, { pos: 1, r: 255, g: 255, b: 255, a: 255 }];
        let seg = 0;
        for (let i = 0; i < n; i++) {
            const t = i / (n - 1);
            while (seg < list.length - 1 && t > list[seg + 1].pos) seg++;
            const a = list[seg], b = list[Math.min(list.length - 1, seg + 1)];
            const f = b.pos > a.pos ? clamp01((t - a.pos) / (b.pos - a.pos)) : 0;
            const o = i * 4;
            lut[o] = a.r + (b.r - a.r) * f;
            lut[o + 1] = a.g + (b.g - a.g) * f;
            lut[o + 2] = a.b + (b.b - a.b) * f;
            lut[o + 3] = a.a + (b.a - a.a) * f;
        }
        return lut;
    }

    // Writes lut(t) + d (a dither offset in -0.5..0.5) into data[o..o+3].
    function putLut(data, o, lut, n1, t, d) {
        if (!(t > 0)) t = 0; else if (t > 1) t = 1;
        const pos = t * n1;
        let i = pos | 0;
        if (i >= n1) i = n1 - 1;
        const f = pos - i, a = i * 4, b = a + 4;
        data[o] = lut[a] + (lut[b] - lut[a]) * f + d;
        data[o + 1] = lut[a + 1] + (lut[b + 1] - lut[a + 1]) * f + d;
        data[o + 2] = lut[a + 2] + (lut[b + 2] - lut[a + 2]) * f + d;
        data[o + 3] = lut[a + 3] + (lut[b + 3] - lut[a + 3]) * f;
    }

    const dither = (x, y) => (hash2(x, y, 0x5EED) >>> 8) / 16777216 - 0.5;

    function hexRgb(color, fallback) {
        const rgba = Lib.parseColor(color) || Lib.parseColor(fallback || '#000000');
        return [rgba[0], rgba[1], rgba[2]];
    }

    // --- frame, loops, progress ---

    function makeFrame(args) {
        const w = clamp(Math.round(Number(args.width) || 0), 1, MAX_SIDE), h = clamp(Math.round(Number(args.height) || 0), 1, MAX_SIDE);
        const rw = Number(args.refWidth) > 0 ? Number(args.refWidth) : w, rh = Number(args.refHeight) > 0 ? Number(args.refHeight) : h;
        const short = Math.min(rw, rh);
        const ax = rw / short, ay = rh / short;
        return { w, h, ax, ay, du: ax / w, dv: ay / h, px: Math.max(ax / w, ay / h), short: Math.min(w, h), draft: !!args.draft };
    }

    function tick(ctx, y, h, from, span) {
        if ((y & 15) !== 15 && y !== h - 1) return;
        if (ctx.cancelled()) throw CANCELLED;
        ctx.progress((from || 0) + (span == null ? 1 : span) * (y + 1) / h);
    }

    function checkCancel(ctx) {
        if (ctx.cancelled()) throw CANCELLED;
    }

    // valueAt(u, v, x, y) -> position 0..1 in the gradient table.
    function renderScalar(fr, ctx, lut, valueAt) {
        const img = Lib.createImage(fr.w, fr.h), data = img.data;
        const w = fr.w, h = fr.h, n1 = lut.length / 4 - 1;
        for (let y = 0; y < h; y++) {
            const v = (y + 0.5) * fr.dv;
            let o = y * w * 4;
            for (let x = 0; x < w; x++, o += 4) putLut(data, o, lut, n1, valueAt((x + 0.5) * fr.du, v, x, y), dither(x, y));
            tick(ctx, y, h);
        }
        return img;
    }

    // rgbAt(u, v, x, y, out) fills out[0..2] (0..255 floats) and optionally out[3] (alpha, default 255).
    function renderRGB(fr, ctx, rgbAt, from, span) {
        const img = Lib.createImage(fr.w, fr.h), data = img.data;
        const w = fr.w, h = fr.h;
        const out = [0, 0, 0, 255];
        for (let y = 0; y < h; y++) {
            const v = (y + 0.5) * fr.dv;
            let o = y * w * 4;
            for (let x = 0; x < w; x++, o += 4) {
                out[3] = 255;
                rgbAt((x + 0.5) * fr.du, v, x, y, out);
                data[o] = out[0]; data[o + 1] = out[1]; data[o + 2] = out[2]; data[o + 3] = out[3];
            }
            tick(ctx, y, h, from, span);
        }
        return img;
    }

    function toneFn(p) {
        const c = p.contrast == null ? 1 : p.contrast, b = p.brightness || 0, inv = !!p.invert;
        return t => {
            t = (t - 0.5) * c + 0.5 + b;
            return inv ? 1 - t : t;
        };
    }

    // --- parameter schema helpers (compact definitions that expand to runtime.buildParamControls schemas) ---

    const OPTION_NAMES = {
        'f1': 'F1', 'f2': 'F2', 'f2-f1': 'F2 − F1', 'fbm': 'fBm', 'xor': 'XOR', 'rgb': 'RGB',
        'hex': 'Hexagonal', 'square': 'Square', 'perlin': 'Perlin', 'simplex': 'Simplex', 'value': 'Value'
    };

    function words(text) {
        const spaced = String(text).replace(/([a-z0-9])([A-Z])/g, '$1 $2').replace(/[-_]+/g, ' ').trim().toLowerCase();
        return spaced.charAt(0).toUpperCase() + spaced.slice(1);
    }

    const keyPart = text => String(text).toLowerCase().replace(/[^a-z0-9]+/g, '_').replace(/^_+|_+$/g, '');

    function param(type, key, dflt, extra) {
        return Object.assign({ key, type, labelKey: 'pixel.gp_' + keyPart(key), fallback: words(key), default: dflt }, extra || {});
    }

    function optionsOf(values) {
        return values.map(value => ({ value, labelKey: 'pixel.go_' + keyPart(value), fallback: OPTION_NAMES[value] || words(value) }));
    }

    const P = {
        seed: dflt => param('seed', 'seed', dflt == null ? 1 : dflt),
        range: (key, dflt, min, max, step, extra) => param('range', key, dflt, Object.assign({ min, max, step }, extra)),
        int: (key, dflt, min, max, extra) => param('int', key, dflt, Object.assign({ min, max, step: 1 }, extra)),
        bool: (key, dflt, extra) => param('bool', key, !!dflt, extra),
        color: (key, dflt, extra) => param('color', key, dflt, extra),
        angle: (key, dflt, extra) => param('angle', key, dflt, extra),
        point: (key, x, y, extra) => param('point', key, { x, y }, extra),
        gradient: (key, dflt, extra) => param('gradient', key, dflt, extra),
        select: (key, values, dflt, extra) => param('select', key, dflt == null ? values[0] : dflt, Object.assign({ options: optionsOf(values) }, extra)),
        seg: (key, values, dflt, extra) => param('segmented', key, dflt == null ? values[0] : dflt, Object.assign({ options: optionsOf(values) }, extra)),
        // Shared groups
        contrast: dflt => param('range', 'contrast', dflt == null ? 1 : dflt, { min: 0.1, max: 4, step: 0.05 }),
        brightness: () => param('range', 'brightness', 0, { min: -0.5, max: 0.5, step: 0.01 }),
        invert: () => param('bool', 'invert', false),
        tileable: () => param('bool', 'tileable', false),
        scale: dflt => param('range', 'scale', dflt, { min: 2, max: 100, step: 0.5, unit: '%' })
    };

    const tone = () => [P.contrast(), P.brightness(), P.invert()];

    // --- catalog ---

    const GENERATORS = [];
    const BY_ID = Object.create(null);
    const CATEGORIES = ['noise', 'texture', 'pattern', 'math'];

    function define(id, cat, name, params, presets, render) {
        const entry = {
            id, cat, labelKey: 'pixel.gen_' + keyPart(id), fallback: name, params,
            presets: (presets || []).map(item => ({ id: item[0], labelKey: 'pixel.gpr_' + keyPart(item[0]), fallback: words(item[0]), values: item[1] || {} })),
            render
        };
        GENERATORS.push(entry);
        BY_ID[id] = entry;
        return entry;
    }

    function cloneValue(value) {
        return value && typeof value === 'object' ? JSON.parse(JSON.stringify(value)) : value;
    }

    function paramDefault(p) {
        if (p.type === 'gradient' && typeof p.default === 'string') {
            const preset = Lib.gradientPreset(p.default);
            return preset ? Lib.cloneStops(preset.stops) : [{ pos: 0, color: '#000000' }, { pos: 1, color: '#ffffff' }];
        }
        return cloneValue(p.default);
    }

    function defaultParams(id) {
        const entry = BY_ID[id];
        const out = {};
        if (entry) entry.params.forEach(p => { out[p.key] = paramDefault(p); });
        return out;
    }

    // Raw values (from the UI or a saved preset) -> valid values for every schema entry; unknown keys are dropped.
    function normalizeParams(entry, raw) {
        const src = raw && typeof raw === 'object' ? raw : {};
        const out = {};
        entry.params.forEach(p => {
            let v = src[p.key];
            if (v === undefined || v === null) v = paramDefault(p);
            switch (p.type) {
                case 'range': {
                    v = Number(v);
                    if (!Number.isFinite(v)) v = Number(p.default);
                    v = clamp(v, p.min, p.max);
                    break;
                }
                case 'int': {
                    v = Math.round(Number(v));
                    if (!Number.isFinite(v)) v = Number(p.default);
                    v = clamp(v, p.min, p.max);
                    break;
                }
                case 'select': case 'segmented':
                    if (!p.options.some(option => option.value === v)) v = p.default;
                    break;
                case 'bool': v = !!v; break;
                case 'color': {
                    const rgba = Lib.parseColor(v) || Lib.parseColor(p.default);
                    v = Lib.toHex(rgba[0], rgba[1], rgba[2]);
                    break;
                }
                case 'angle': {
                    v = Number(v);
                    v = Number.isFinite(v) ? wrapInt(v, 360) : Number(p.default);
                    break;
                }
                case 'seed': {
                    v = Number(v);
                    v = Number.isFinite(v) ? Math.floor(Math.abs(v)) % 4294967296 : Number(p.default);
                    break;
                }
                case 'point': {
                    const pt = v && typeof v === 'object' ? v : p.default;
                    v = { x: clamp01(Number(pt.x)), y: clamp01(Number(pt.y)) };
                    if (!Number.isFinite(v.x)) v.x = 0.5;
                    if (!Number.isFinite(v.y)) v.y = 0.5;
                    break;
                }
                case 'gradient': {
                    let stops = typeof v === 'string' ? (Lib.gradientPreset(v) || {}).stops : v;
                    if (!Array.isArray(stops) || !stops.length) stops = paramDefault(p);
                    v = Lib.cloneStops(stops);
                    break;
                }
                default: break;
            }
            out[p.key] = v;
        });
        return out;
    }

    function noHooks(ctx) {
        return {
            progress: ctx && typeof ctx.progress === 'function' ? f => ctx.progress(f) : () => {},
            cancelled: ctx && typeof ctx.cancelled === 'function' ? () => ctx.cancelled() : () => false
        };
    }

    // The public entry: validates the params, renders, returns the image.
    function render(args, ctx) {
        const entry = BY_ID[args && args.id];
        if (!entry) throw new Error('gen.render: unknown generator ' + (args && args.id));
        const params = normalizeParams(entry, args.params);
        const fr = makeFrame(args);
        const hooks = noHooks(ctx);
        let image;
        try {
            image = entry.render(params, fr, hooks);
        } catch (err) {
            if (err === CANCELLED) return { cancelled: true };
            throw err;
        }
        hooks.progress(1);
        return { image, width: image.width, height: image.height, data: image.data };
    }

    Lib.registerOp('gen.render', render);

    // ============================================================ noise ============================================================

    // Plain or fractal noise from one base primitive, shared by the single-primitive generators.
    function fbmRenderer(fixedType, fixedKind) {
        return (p, fr, ctx) => {
            const field = makeFbm({
                type: fixedType || p.type, kind: fixedKind || p.kind, seed: p.seed, cells: 100 / p.scale, octaves: p.octaves,
                lacunarity: p.lacunarity, gain: p.gain, ridge: p.ridge, tile: p.tileable, interp: p.interpolation,
                offsetX: (p.offsetX || 0) / 100, offsetY: (p.offsetY || 0) / 100
            }, fr);
            const ton = toneFn(p), lut = buildLUT(p.gradient);
            return renderScalar(fr, ctx, lut, (u, v) => ton(field.t(u, v)));
        };
    }

    const noiseTail = grad => tone().concat([P.gradient('gradient', grad || 'grayscale'), P.tileable()]);
    const fractalCore = (scale, octaves, gain) => [
        P.scale(scale), P.int('octaves', octaves, 1, 10), P.range('lacunarity', 2, 1.5, 4, 0.05), P.range('gain', gain == null ? 0.5 : gain, 0.2, 0.9, 0.01),
        P.range('offsetX', 0, -100, 100, 1, { unit: '%' }), P.range('offsetY', 0, -100, 100, 1, { unit: '%' })
    ];
    const typeSel = dflt => P.seg('type', ['perlin', 'simplex', 'value'], dflt || 'simplex');

    define('white-noise', 'noise', 'White noise', [
        P.seed(), P.seg('mode', ['mono', 'color'], 'mono'), P.seg('distribution', ['uniform', 'gaussian'], 'uniform'),
        P.range('grain', 1, 1, 64, 1, { unit: 'px' })
    ].concat(tone(), [P.gradient('gradient', 'grayscale', { showIf: v => v.mode === 'mono' })]), [
        ['film-grain', { mode: 'mono', distribution: 'gaussian', grain: 1, contrast: 0.7 }],
        ['tv-static', { mode: 'mono', distribution: 'uniform', grain: 2, contrast: 1 }],
        ['color-confetti', { mode: 'color', distribution: 'uniform', grain: 14, contrast: 1 }]
    ], (p, fr, ctx) => {
        // Derived seeds must differ in more than their low bits: x ^ (s + 1) is x ^ s with one pixel moved.
        const seeds = [];
        for (let i = 0; i < 16; i++) seeds.push(seedOf(p.seed, 700 + i));
        const grain = Math.max(1, Math.round(p.grain));
        const ton = toneFn(p);
        const sample = (bx, by, ch) => {
            if (p.distribution === 'uniform') return rand01(bx, by, seeds[ch * 4]);
            const sum = rand01(bx, by, seeds[ch * 4]) + rand01(bx, by, seeds[ch * 4 + 1]) + rand01(bx, by, seeds[ch * 4 + 2]) + rand01(bx, by, seeds[ch * 4 + 3]);
            return 0.5 + (sum - 2) * 1.7321 * 0.18;
        };
        if (p.mode === 'mono') {
            return renderScalar(fr, ctx, buildLUT(p.gradient), (u, v, x, y) => ton(sample(Math.floor(x / grain), Math.floor(y / grain), 0)));
        }
        return renderRGB(fr, ctx, (u, v, x, y, out) => {
            const bx = Math.floor(x / grain), by = Math.floor(y / grain);
            out[0] = clamp(ton(sample(bx, by, 1)), 0, 1) * 255;
            out[1] = clamp(ton(sample(bx, by, 2)), 0, 1) * 255;
            out[2] = clamp(ton(sample(bx, by, 3)), 0, 1) * 255;
        });
    });

    define('value-noise', 'noise', 'Value noise', [
        P.seed(), P.scale(30), P.int('octaves', 1, 1, 8), P.select('interpolation', ['smooth', 'cubic', 'linear', 'step'], 'smooth')
    ].concat(noiseTail()), [
        ['soft', { scale: 35, octaves: 1, interpolation: 'smooth' }],
        ['mosaic', { scale: 9, octaves: 1, interpolation: 'step', gradient: 'viridis' }],
        ['bumpy', { scale: 26, octaves: 5, interpolation: 'smooth', contrast: 1.2 }]
    ], fbmRenderer('value'));

    define('perlin', 'noise', 'Perlin noise', [P.seed(), P.scale(30), P.int('octaves', 1, 1, 8)].concat(noiseTail()), [
        ['soft-clouds', { scale: 38, octaves: 5, contrast: 0.95, gradient: 'ice' }],
        ['hard-rock', { scale: 14, octaves: 8, contrast: 1.7, brightness: -0.05, gradient: 'grayscale' }],
        ['smooth-hills', { scale: 55, octaves: 1, contrast: 1, gradient: 'earth' }]
    ], fbmRenderer('perlin'));

    define('simplex', 'noise', 'Simplex noise', [P.seed(), P.scale(30), P.int('octaves', 1, 1, 8)].concat(noiseTail()), [
        ['soft-blobs', { scale: 30, octaves: 1, contrast: 1, gradient: 'sunset' }],
        ['terrain', { scale: 38, octaves: 6, contrast: 1.15, gradient: 'terrain' }],
        ['lava-lamp', { scale: 50, octaves: 2, contrast: 1.5, gradient: 'fire' }]
    ], fbmRenderer('simplex'));

    define('worley', 'noise', 'Cellular (Worley)', [
        P.seed(), P.scale(14), P.seg('output', ['f1', 'f2', 'f2-f1', 'cells'], 'f1'),
        P.select('metric', ['euclidean', 'manhattan', 'chebyshev'], 'euclidean'), P.range('jitter', 1, 0, 1, 0.01)
    ].concat(noiseTail()), [
        ['cells', { output: 'cells', scale: 14, jitter: 1, gradient: 'pastel', contrast: 1 }],
        ['bubbles', { output: 'f1', scale: 12, jitter: 0.85, invert: true, gradient: 'ocean', contrast: 1.2 }],
        ['cracks', { output: 'f2-f1', scale: 16, jitter: 1, gradient: 'grayscale', contrast: 3, brightness: -0.1 }],
        ['crystals', { output: 'f1', metric: 'manhattan', scale: 14, jitter: 1, gradient: 'ice', contrast: 1.3 }]
    ], (p, fr, ctx) => {
        const metric = p.metric === 'manhattan' ? 1 : (p.metric === 'chebyshev' ? 2 : 0);
        const lay = noiseLayout('perlin', 100 / p.scale, fr, p.tileable);
        const worley = worleyNoise(p.seed, p.jitter, metric);
        const k = metric === 1 ? 0.75 : (metric === 2 ? 1.3 : 1);
        const ton = toneFn(p), lut = buildLUT(p.gradient);
        return renderScalar(fr, ctx, lut, (u, v) => {
            const r = worley(u * lay.sx, v * lay.sy, lay.nx, lay.ny);
            let t;
            if (p.output === 'f2') t = r.f2 * 0.6 * k;
            else if (p.output === 'f2-f1') t = (r.f2 - r.f1) * 1.6 * k;
            else if (p.output === 'cells') t = (r.id >>> 8) / 16777216;
            else t = r.f1 * k;
            return ton(t);
        });
    });

    define('fbm', 'noise', 'Fractal noise (fBm)', [P.seed(), typeSel('simplex')].concat(fractalCore(30, 6), noiseTail()), [
        ['cumulus', { type: 'simplex', scale: 34, octaves: 6, gain: 0.5, contrast: 1, gradient: 'ice' }],
        ['mountains', { type: 'perlin', scale: 40, octaves: 8, gain: 0.55, contrast: 1.2, gradient: 'terrain' }],
        ['fine-detail', { type: 'value', scale: 8, octaves: 4, gain: 0.6, contrast: 1, gradient: 'grayscale' }]
    ], fbmRenderer(null, 'fbm'));

    define('ridged', 'noise', 'Ridged multifractal', [
        P.seed(), typeSel('simplex')
    ].concat(fractalCore(32, 6), [P.range('ridge', 1, 0.5, 1.5, 0.01)], noiseTail('magma')), [
        ['veins', { type: 'simplex', scale: 24, octaves: 4, gain: 0.5, ridge: 1, gradient: 'magma', contrast: 1.1 }],
        ['mountain-ridges', { type: 'perlin', scale: 45, octaves: 8, gain: 0.5, ridge: 1, gradient: 'terrain', contrast: 1 }],
        ['lightning', { type: 'simplex', scale: 36, octaves: 5, gain: 0.45, ridge: 1, contrast: 2.4, brightness: -0.32,
            gradient: [{ pos: 0, color: '#03041a' }, { pos: 0.55, color: '#1a2f8f' }, { pos: 0.85, color: '#8fc1ff' }, { pos: 1, color: '#ffffff' }] }]
    ], fbmRenderer(null, 'ridged'));

    define('turbulence', 'noise', 'Turbulence', [
        P.seed(), typeSel('simplex'), P.seg('kind', ['turbulence', 'billow'], 'turbulence')
    ].concat(fractalCore(30, 6), noiseTail('inferno')), [
        ['smoke', { kind: 'turbulence', scale: 34, octaves: 6, gain: 0.5, gradient: 'grayscale', contrast: 1.1 }],
        ['billows', { kind: 'billow', scale: 28, octaves: 4, gain: 0.5, gradient: 'ice', contrast: 1 }],
        ['fire', { kind: 'turbulence', scale: 30, octaves: 5, gain: 0.55, gradient: 'fire', contrast: 1.3 }]
    ], fbmRenderer(null, null));

    define('warped', 'noise', 'Domain-warped noise', [
        P.seed(), typeSel('simplex'), P.scale(32), P.int('octaves', 4, 1, 10), P.range('warp', 1, 0, 6, 0.05),
        P.int('iterations', 2, 1, 4), P.range('warpScale', 1, 0.25, 4, 0.05),
        P.range('offsetX', 0, -100, 100, 1, { unit: '%' }), P.range('offsetY', 0, -100, 100, 1, { unit: '%' })
    ].concat(noiseTail('magma')), [
        ['liquid-marble', { warp: 1.4, iterations: 2, scale: 40, octaves: 4, gradient: 'grayscale', contrast: 1.2 }],
        ['smoke', { warp: 0.8, iterations: 2, scale: 36, octaves: 6, gradient: 'ice', contrast: 1.1 }],
        ['organic', { warp: 1.6, iterations: 3, scale: 34, octaves: 3, gradient: 'forest', contrast: 1 }],
        ['psychedelic', { warp: 2.4, iterations: 3, scale: 30, octaves: 2, gradient: 'rainbow', contrast: 1 }]
    ], (p, fr, ctx) => {
        const cells = 100 / p.scale;
        const field = makeFbm({ type: p.type, seed: p.seed, cells, octaves: p.octaves, lacunarity: 2, gain: 0.5, tile: p.tileable, offsetX: p.offsetX / 100, offsetY: p.offsetY / 100 }, fr);
        const warp = makeWarp({ type: p.type, seed: p.seed, cells, strength: p.warp, iterations: p.iterations, warpScale: p.warpScale, tile: p.tileable, lacunarity: 2 }, fr);
        const ton = toneFn(p), lut = buildLUT(p.gradient), q = { u: 0, v: 0 };
        return renderScalar(fr, ctx, lut, (u, v) => {
            warp(u, v, q);
            return ton(field.t(q.u, q.v));
        });
    });

    // ============================================================ textures ============================================================

    const rad = deg => deg * Math.PI / 180;

    // Rotation about the middle of the image; fills out.x / out.y.
    function rotator(angleDeg, fr) {
        const a = rad(angleDeg), c = Math.cos(a), s = Math.sin(a), cx = fr.ax / 2, cy = fr.ay / 2;
        return (u, v, out) => {
            const x = u - cx, y = v - cy;
            out.x = x * c + y * s;
            out.y = -x * s + y * c;
            return out;
        };
    }

    // Triangle wave 0..1..0 of x (period 1): maps any real value to a seamless colour position.
    const pingPong = x => 1 - Math.abs(2 * fract(x) - 1);

    const SKY = [{ pos: 0, color: '#2f6db3' }, { pos: 0.4, color: '#6aa6dd' }, { pos: 0.72, color: '#cfe3f5' }, { pos: 1, color: '#ffffff' }];

    define('clouds', 'texture', 'Clouds', [
        P.seed(), P.scale(40), P.int('octaves', 7, 1, 9), P.range('coverage', 0, -0.4, 0.4, 0.01), P.contrast(1.5),
        P.range('softness', 0.35, 0.05, 1, 0.01), P.range('shading', 0.7, 0, 1, 0.01), P.range('wisps', 0.5, 0, 2, 0.05),
        P.gradient('gradient', SKY), P.tileable()
    ], [
        ['fair-weather', { coverage: -0.04, contrast: 1.6, softness: 0.3, shading: 0.7, wisps: 0.5, gradient: SKY }],
        ['overcast', { coverage: 0.2, contrast: 1, softness: 0.6, shading: 0.45, wisps: 0.3, gradient: [{ pos: 0, color: '#6e7681' }, { pos: 0.45, color: '#a5adb7' }, { pos: 0.8, color: '#d9dde2' }, { pos: 1, color: '#f4f6f8' }] }],
        ['storm', { coverage: 0.14, contrast: 1.5, softness: 0.4, shading: 0.9, wisps: 0.9, gradient: [{ pos: 0, color: '#101218' }, { pos: 0.4, color: '#2b303b' }, { pos: 0.7, color: '#5a6474' }, { pos: 1, color: '#c9d0da' }] }],
        ['sunset', { coverage: 0.02, contrast: 1.4, softness: 0.4, shading: 0.8, wisps: 0.7, gradient: [{ pos: 0, color: '#2d1b55' }, { pos: 0.3, color: '#8a3f8f' }, { pos: 0.55, color: '#e8607a' }, { pos: 0.8, color: '#ffa860' }, { pos: 1, color: '#fff0c8' }] }]
    ], (p, fr, ctx) => {
        const cells = 100 / p.scale;
        const field = makeFbm({ type: 'simplex', seed: p.seed, cells, octaves: p.octaves, lacunarity: 2.05, gain: 0.56, tile: p.tileable }, fr);
        const warp = p.wisps > 0 ? makeWarp({ type: 'simplex', seed: p.seed, cells, strength: p.wisps * 0.1, iterations: 1, warpScale: 0.7, warpOctaves: 2, tile: p.tileable, lacunarity: 2 }, fr) : null;
        const q = { u: 0, v: 0 };
        // Shading: a coarse copy of the density, lit from above; the underside of a cloud is greyer than its top.
        const coarse = makeFbm({ type: 'simplex', seed: p.seed, cells: cells * 0.8, octaves: 3, lacunarity: 2, gain: 0.5, tile: p.tileable }, fr);
        const drop = 0.3 / cells;
        const density = (u, v) => {
            let pu = u, pv = v;
            if (warp) { warp(u, v, q); pu = q.u; pv = q.v; }
            return (field.t(pu, pv) - 0.5 + p.coverage) * p.contrast;
        };
        return renderScalar(fr, ctx, buildLUT(p.gradient), (u, v) => {
            const d = density(u, v);
            const body = smoothstep(-0.04 - p.softness * 0.25, 0.1 + p.softness * 0.5, d);
            if (p.shading <= 0 || body <= 0) return body;
            const core = smoothstep(0, 0.6, d);
            const lit = clamp01(0.5 + (coarse.t(u, v) - coarse.t(u, v - drop)) * 5);
            return body * (1 - p.shading * (0.3 * (1 - lit) + 0.16 * (1 - core)));
        });
    });

    define('marble', 'texture', 'Marble', [
        P.seed(), P.scale(38), P.int('veins', 4, 1, 24), P.range('turbulence', 0.55, 0, 3, 0.01), P.range('sharpness', 3, 0.5, 10, 0.05),
        P.angle('angle', 35), P.int('octaves', 5, 1, 8), P.range('fine', 0.55, 0, 1, 0.01),
        P.gradient('gradient', [{ pos: 0, color: '#4a4d54' }, { pos: 0.35, color: '#a9acb4' }, { pos: 0.7, color: '#e6e6e9' }, { pos: 1, color: '#fcfcfd' }])
    ], [
        ['carrara', {}],
        ['black-marquina', { veins: 3, turbulence: 0.9, sharpness: 2.2, fine: 0.5, gradient: [{ pos: 0, color: '#efe8d8' }, { pos: 0.3, color: '#8a8985' }, { pos: 0.6, color: '#2a2a2a' }, { pos: 1, color: '#0b0b0b' }] }],
        ['verde', { veins: 5, turbulence: 1.2, sharpness: 1.8, fine: 0.7, gradient: [{ pos: 0, color: '#f0f5f0' }, { pos: 0.3, color: '#8cbc9d' }, { pos: 0.65, color: '#2f6b4f' }, { pos: 1, color: '#0f3226' }] }],
        ['rosa', { veins: 6, turbulence: 0.8, sharpness: 2, fine: 0.6, gradient: [{ pos: 0, color: '#6f3a41' }, { pos: 0.4, color: '#cf9a9f' }, { pos: 0.75, color: '#f2dcd9' }, { pos: 1, color: '#fbf1ee' }] }]
    ], (p, fr, ctx) => {
        const cells = 100 / p.scale;
        const turb = makeFbm({ type: 'simplex', seed: p.seed, cells, octaves: p.octaves, lacunarity: 2, gain: 0.45 }, fr);
        const cloud = makeFbm({ type: 'perlin', seed: seedOf(p.seed, 31), cells: cells * 0.6, octaves: 4 }, fr);
        const rot = rotator(p.angle, fr), pt = { x: 0, y: 0 };
        const k = 1 / (turb.std * 2);
        return renderScalar(fr, ctx, buildLUT(p.gradient), (u, v) => {
            rot(u, v, pt);
            const n = turb.f(u, v) * k;
            const phase = pt.x * p.veins + p.turbulence * n;
            const s1 = Math.abs(Math.sin(Math.PI * phase));
            const s2 = Math.abs(Math.sin(Math.PI * (phase * 2.9 + 0.31 + 0.35 * n)));
            const soft = Math.pow(1 - s1, 1.4) * 0.28;
            const vein = Math.pow(1 - s1, p.sharpness * 2) * 0.95 + soft + p.fine * 0.45 * Math.pow(1 - s2, p.sharpness * 3.4);
            const cl = cloud.f(u, v) / (cloud.std * 2);
            return 0.9 + 0.1 * cl - vein * 0.9;
        });
    });

    const WOODS = {
        oak: [{ pos: 0, color: '#3a2312' }, { pos: 0.3, color: '#6b3f1f' }, { pos: 0.6, color: '#a46a37' }, { pos: 0.85, color: '#d19a5f' }, { pos: 1, color: '#ecc590' }],
        walnut: [{ pos: 0, color: '#1b0e08' }, { pos: 0.35, color: '#3b2112' }, { pos: 0.65, color: '#6a4326' }, { pos: 0.88, color: '#9a6b42' }, { pos: 1, color: '#c79a6d' }],
        pine: [{ pos: 0, color: '#a8793d' }, { pos: 0.4, color: '#cf9b58' }, { pos: 0.75, color: '#e7c28a' }, { pos: 1, color: '#f6e4bd' }],
        ebony: [{ pos: 0, color: '#030202' }, { pos: 0.5, color: '#120d09' }, { pos: 0.8, color: '#241911' }, { pos: 1, color: '#3d2c1e' }]
    };

    define('wood', 'texture', 'Wood', [
        P.seed(), P.range('rings', 16, 3, 80, 0.5), P.range('distortion', 0.6, 0, 2, 0.01), P.range('grain', 0.6, 0, 1, 0.01),
        P.range('arch', 1.1, 0, 3, 0.01), P.angle('angle', 0), P.gradient('gradient', WOODS.oak)
    ], [
        ['oak', {}],
        ['walnut', { rings: 11, distortion: 0.8, grain: 0.75, arch: 1.4, gradient: WOODS.walnut }],
        ['pine', { rings: 8, distortion: 0.45, grain: 0.45, arch: 0.7, gradient: WOODS.pine }],
        ['ebony', { rings: 30, distortion: 0.35, grain: 0.65, arch: 0.5, gradient: WOODS.ebony }]
    ], (p, fr, ctx) => {
        const rot = rotator(p.angle, fr), pt = { x: 0, y: 0 };
        const low = makeFbm({ type: 'simplex', seed: seedOf(p.seed, 41), cells: 2.4, octaves: 3 }, fr);
        const mid = makeFbm({ type: 'simplex', seed: seedOf(p.seed, 42), cells: 11, octaves: 3 }, fr);
        const streak = makeBase('value', seedOf(p.seed, 43)), streak2 = makeBase('value', seedOf(p.seed, 44)), pore = makeBase('value', seedOf(p.seed, 45));
        const kl = 1 / (low.std * 2), km = 1 / (mid.std * 2);
        return renderScalar(fr, ctx, buildLUT(p.gradient), (u, v) => {
            rot(u, v, pt);
            // Growth rings seen on a sawn board: nested parabolas ("cathedral" grain) that wobble with the noise.
            const dist = p.distortion * (0.07 * low.f(u, v) * kl + 0.012 * mid.f(u, v) * km);
            const rho = pt.y + 0.18 + p.arch * pt.x * pt.x * 0.9 + dist;
            const phase = rho * p.rings;
            const wave = 0.5 + 0.5 * Math.sin(TAU * phase + 1.1 * Math.sin(TAU * phase));
            const ring = Math.pow(wave, 1.5);
            const g = streak(pt.x * 5, pt.y * 240, 0, 0) * 0.6 + streak2(pt.x * 11, pt.y * 520, 0, 0) * 0.4;
            const pores = smoothstep(0.78, 0.95, pore(pt.x * 38, pt.y * 330, 0, 0));
            const t = 0.12 + 0.7 * ring + p.grain * (0.15 * g - 0.2 * pores);
            return t + 0.06 * (mid.t(u, v) - 0.5);
        });
    });

    // Diamond-square heightfield on a (n + 1)^2 grid, normalised to 0..1.
    function diamondSquare(n, rough, seed) {
        const size = n + 1, g = new Float32Array(size * size);
        const rng = Lib.mulberry32(seedOf(seed, 6000));
        const at = (x, y) => g[y * size + x];
        g[0] = rng(); g[n] = rng(); g[n * size] = rng(); g[n * size + n] = rng();
        let amp = 0.5;
        for (let step = n; step > 1; step >>= 1, amp *= rough) {
            const half = step >> 1;
            for (let y = half; y < size; y += step) {
                for (let x = half; x < size; x += step) {
                    g[y * size + x] = (at(x - half, y - half) + at(x + half, y - half) + at(x - half, y + half) + at(x + half, y + half)) / 4 + (rng() - 0.5) * 2 * amp;
                }
            }
            for (let y = 0; y < size; y += half) {
                for (let x = (y / half) % 2 === 0 ? half : 0; x < size; x += step) {
                    let sum = 0, cnt = 0;
                    if (x - half >= 0) { sum += at(x - half, y); cnt++; }
                    if (x + half < size) { sum += at(x + half, y); cnt++; }
                    if (y - half >= 0) { sum += at(x, y - half); cnt++; }
                    if (y + half < size) { sum += at(x, y + half); cnt++; }
                    g[y * size + x] = sum / cnt + (rng() - 0.5) * 2 * amp;
                }
            }
        }
        let lo = 1e9, hi = -1e9;
        for (let i = 0; i < g.length; i++) { if (g[i] < lo) lo = g[i]; if (g[i] > hi) hi = g[i]; }
        const k = 1 / ((hi - lo) || 1);
        for (let i = 0; i < g.length; i++) g[i] = (g[i] - lo) * k;
        return { g, size };
    }

    // Bilinear lookup with a quintic weight (no visible grid), coordinates in grid cells.
    function sampleGrid(grid, x, y) {
        const size = grid.size, last = size - 1;
        x = clamp(x, 0, last); y = clamp(y, 0, last);
        let x0 = Math.floor(x), y0 = Math.floor(y);
        if (x0 >= last) x0 = last - 1;
        if (y0 >= last) y0 = last - 1;
        const tx = fade(x - x0), ty = fade(y - y0);
        const i = y0 * size + x0, g = grid.g;
        const a = g[i] + (g[i + 1] - g[i]) * tx, b = g[i + size] + (g[i + size + 1] - g[i + size]) * tx;
        return a + (b - a) * ty;
    }

    const nextPow2 = n => { let p = 2; while (p < n) p <<= 1; return p; };

    define('plasma', 'texture', 'Plasma', [
        P.seed(), P.seg('style', ['sine', 'diamond-square'], 'sine'), P.scale(40),
        P.int('complexity', 5, 2, 9, { showIf: v => v.style === 'sine' }), P.range('warp', 0.6, 0, 2, 0.01, { showIf: v => v.style === 'sine' }),
        P.range('roughness', 0.55, 0.3, 0.85, 0.01, { showIf: v => v.style === 'diamond-square' }),
        P.int('detail', 256, 64, 1024, { showIf: v => v.style === 'diamond-square', step: 32 }),
        P.range('cycles', 1.5, 0.25, 8, 0.05), P.gradient('gradient', 'plasma')
    ], [
        ['classic-plasma', { style: 'sine', complexity: 5, warp: 0.6, cycles: 1.5, gradient: 'rainbow' }],
        ['neon-waves', { style: 'sine', complexity: 7, warp: 1.1, cycles: 2.5, gradient: 'neon' }],
        ['fractal-glow', { style: 'diamond-square', roughness: 0.55, cycles: 1.2, gradient: 'plasma' }],
        ['heightmap', { style: 'diamond-square', roughness: 0.6, detail: 512, cycles: 0.5, gradient: 'terrain' }]
    ], (p, fr, ctx) => {
        const lut = buildLUT(p.gradient);
        if (p.style === 'diamond-square') {
            const n = nextPow2(Math.min(2048, Math.ceil(p.detail * Math.max(fr.ax, fr.ay))));
            const grid = diamondSquare(n, p.roughness, p.seed);
            const cell = p.detail;
            return renderScalar(fr, ctx, lut, (u, v) => pingPong(sampleGrid(grid, u * cell, v * cell) * p.cycles * 0.5 + 0.0));
        }
        const rng = Lib.mulberry32(seedOf(p.seed, 6100));
        const cells = 100 / p.scale, parts = [];
        for (let i = 0; i < p.complexity; i++) {
            const kind = i % 3;
            parts.push({
                kind, f: TAU * cells * (0.45 + rng() * 0.9), a: rng() * TAU, ph: rng() * TAU,
                cx: rng() * fr.ax, cy: rng() * fr.ay, s: rng() < 0.5 ? -1 : 1
            });
        }
        const wf = TAU * cells * 0.35, wph = rng() * TAU, wph2 = rng() * TAU;
        const norm = 1 / (2 * Math.sqrt(p.complexity * 0.5) * 1.15);
        return renderScalar(fr, ctx, lut, (u, v) => {
            const x = u + p.warp * 0.12 * Math.sin(v * wf + wph) / cells, y = v + p.warp * 0.12 * Math.sin(u * wf * 0.8 + wph2) / cells;
            let sum = 0;
            for (let i = 0; i < parts.length; i++) {
                const c = parts[i];
                if (c.kind === 0) sum += Math.sin((x * Math.cos(c.a) + y * Math.sin(c.a)) * c.f + c.ph);
                else if (c.kind === 1) sum += Math.sin(Math.hypot(x - c.cx, y - c.cy) * c.f + c.ph);
                else sum += Math.sin(c.f * (x * Math.cos(c.a) + y * Math.sin(c.a)) * 0.5 + Math.sin(c.f * 0.35 * (y * Math.cos(c.a) - x * Math.sin(c.a))) * 1.6 + c.ph) * c.s;
            }
            return pingPong((0.5 + sum * norm) * p.cycles * 0.5);
        });
    });

    const CAMO = {
        woodland: ['#4b5a2b', '#2f2a1c', '#7a7a45', '#1c2414', '#a39a62'],
        desert: ['#c8b27b', '#a98d57', '#7a6540', '#d9c9a0', '#5d4d33'],
        arctic: ['#f2f4f6', '#cdd3da', '#98a2ae', '#e1e5ea', '#7b8591'],
        urban: ['#6b6f73', '#4a4d51', '#8c9096', '#2d2f32', '#b0b4b8']
    };

    define('camouflage', 'texture', 'Camouflage', [
        P.seed(), P.scale(24), P.int('colors', 4, 2, 5), P.int('detail', 3, 1, 5), P.range('edge', 0.2, 0, 1, 0.01), P.int('pixels', 0, 0, 160, { step: 4 }),
        P.color('color1', CAMO.woodland[0]), P.color('color2', CAMO.woodland[1]), P.color('color3', CAMO.woodland[2]),
        P.color('color4', CAMO.woodland[3], { showIf: v => v.colors >= 4 }), P.color('color5', CAMO.woodland[4], { showIf: v => v.colors >= 5 })
    ], [
        ['woodland', { colors: 4, pixels: 0, color1: CAMO.woodland[0], color2: CAMO.woodland[1], color3: CAMO.woodland[2], color4: CAMO.woodland[3] }],
        ['desert', { colors: 4, pixels: 0, color1: CAMO.desert[0], color2: CAMO.desert[1], color3: CAMO.desert[2], color4: CAMO.desert[3] }],
        ['arctic', { colors: 3, pixels: 0, color1: CAMO.arctic[0], color2: CAMO.arctic[1], color3: CAMO.arctic[2] }],
        ['urban', { colors: 5, pixels: 0, color1: CAMO.urban[0], color2: CAMO.urban[1], color3: CAMO.urban[2], color4: CAMO.urban[3], color5: CAMO.urban[4] }],
        ['digital', { colors: 4, pixels: 64, edge: 0, color1: '#5c6b45', color2: '#2b3320', color3: '#8d8f5a', color4: '#40462f' }]
    ], (p, fr, ctx) => {
        const field = makeFbm({ type: 'simplex', seed: p.seed, cells: 100 / p.scale, octaves: p.detail, lacunarity: 2.1, gain: 0.5 }, fr);
        const warp = makeWarp({ type: 'simplex', seed: p.seed, cells: 100 / p.scale, strength: 0.5, iterations: 1, warpScale: 0.8, lacunarity: 2 }, fr);
        const cols = [p.color1, p.color2, p.color3, p.color4, p.color5].slice(0, p.colors).map(c => hexRgb(c));
        const q = { u: 0, v: 0 };
        const block = p.pixels > 0 ? p.pixels / 1 : 0;
        const at = (u, v) => {
            if (block) { u = (Math.floor(u * block) + 0.5) / block; v = (Math.floor(v * block) + 0.5) / block; }
            warp(u, v, q);
            return field.t(q.u, q.v);
        };
        // Equal share per colour: thresholds are quantiles of the field over a regular sample of this very frame.
        const samples = [];
        for (let j = 0; j < 40; j++) for (let i = 0; i < 40; i++) samples.push(at((i + 0.5) / 40 * fr.ax, (j + 0.5) / 40 * fr.ay));
        samples.sort((a, b) => a - b);
        const thr = [];
        for (let k = 1; k < cols.length; k++) thr.push(samples[Math.floor(k / cols.length * samples.length)]);
        const eps = Math.max(0.003, p.edge * 0.05);
        return renderRGB(fr, ctx, (u, v, x, y, out) => {
            const t = at(u, v);
            let r = cols[0][0], g = cols[0][1], b = cols[0][2];
            for (let k = 0; k < thr.length; k++) {
                const w = block ? (t >= thr[k] ? 1 : 0) : smoothstep(thr[k] - eps, thr[k] + eps, t);
                r += (cols[k + 1][0] - r) * w; g += (cols[k + 1][1] - g) * w; b += (cols[k + 1][2] - b) * w;
            }
            out[0] = r; out[1] = g; out[2] = b;
        });
    });

    const LAVA = [{ pos: 0, color: '#060202' }, { pos: 0.2, color: '#1d0a06' }, { pos: 0.38, color: '#5a1408' }, { pos: 0.58, color: '#c1330b' }, { pos: 0.78, color: '#ff7a1a' }, { pos: 0.92, color: '#ffd24a' }, { pos: 1, color: '#fff6c8' }];

    define('lava', 'texture', 'Lava', [
        P.seed(), P.scale(14), P.range('turbulence', 0.5, 0, 2, 0.01), P.range('crackWidth', 0.28, 0.05, 1, 0.01), P.range('heat', 0.7, 0, 1, 0.01),
        P.range('crust', 0.7, 0, 1, 0.01), P.gradient('gradient', LAVA), P.tileable()
    ], [
        ['molten', {}],
        ['cooling-crust', { crackWidth: 0.14, heat: 0.55, crust: 1, scale: 11 }],
        ['magma-flow', { crackWidth: 0.5, turbulence: 1.2, heat: 0.9, crust: 0.5, scale: 18 }]
    ], (p, fr, ctx) => {
        const cells = 100 / p.scale;
        const lay = noiseLayout('perlin', cells, fr, p.tileable);
        const worley = worleyNoise(p.seed, 1, 0);
        const warp = makeWarp({ type: 'simplex', seed: seedOf(p.seed, 51), cells, strength: p.turbulence * 0.4, iterations: 1, warpScale: 0.8, tile: p.tileable, lacunarity: 2 }, fr);
        const heat = makeFbm({ type: 'simplex', seed: seedOf(p.seed, 52), cells: cells * 0.6, octaves: 3, tile: p.tileable }, fr);
        const crust = makeFbm({ type: 'simplex', seed: seedOf(p.seed, 53), cells: cells * 2.5, octaves: 5, tile: p.tileable }, fr);
        const q = { u: 0, v: 0 };
        return renderScalar(fr, ctx, buildLUT(p.gradient), (u, v) => {
            warp(u, v, q);
            const r = worley(q.u * lay.sx, q.v * lay.sy, lay.nx, lay.ny);
            const edge = r.f2 - r.f1;
            const glow = Math.pow(1 - smoothstep(0, p.crackWidth, edge), 1.5);
            const h = heat.t(u, v);
            const c = crust.t(u, v);
            // Cracks glow where the crust is thin (heat noise); the plates keep a dark, rough surface.
            const hot = glow * clamp01(0.35 + p.heat * (0.25 + 0.9 * h));
            const plate = 0.05 + 0.2 * p.crust * c * c + 0.05 * r.f1;
            return clamp01(Math.max(hot, plate) + 0.35 * glow * (c - 0.5) * 0.3);
        });
    });

    define('caustics', 'texture', 'Caustics', [
        P.seed(), P.scale(16), P.int('layers', 2, 1, 3), P.range('sharpness', 3.2, 1, 8, 0.05), P.range('distortion', 0.6, 0, 1.5, 0.01),
        P.range('intensity', 1, 0.2, 2, 0.01),
        P.gradient('gradient', [{ pos: 0, color: '#032a4a' }, { pos: 0.45, color: '#0a6fa5' }, { pos: 0.8, color: '#5ec8e8' }, { pos: 1, color: '#effcff' }]), P.tileable()
    ], [
        ['pool', {}],
        ['deep-sea', { scale: 24, layers: 3, sharpness: 4.5, intensity: 0.8, gradient: [{ pos: 0, color: '#010b1a' }, { pos: 0.5, color: '#04406b' }, { pos: 0.85, color: '#2a9ec7' }, { pos: 1, color: '#bff2ff' }] }],
        ['sun-dappled', { scale: 12, layers: 2, sharpness: 2.4, distortion: 0.9, intensity: 1.1, gradient: [{ pos: 0, color: '#0b3d2e' }, { pos: 0.5, color: '#2f9e7a' }, { pos: 0.85, color: '#b6f0d0' }, { pos: 1, color: '#ffffff' }] }]
    ], (p, fr, ctx) => {
        const cells = 100 / p.scale;
        const warp = p.distortion > 0 ? makeWarp({ type: 'simplex', seed: seedOf(p.seed, 61), cells: cells * 0.8, strength: p.distortion * 0.7, iterations: 1, warpScale: 1, tile: p.tileable, lacunarity: 2 }, fr) : null;
        const layers = [];
        for (let i = 0; i < p.layers; i++) {
            const c = cells * (1 + i * 0.55);
            const lay = noiseLayout('perlin', c, fr, p.tileable);
            const rng = Lib.mulberry32(seedOf(p.seed, 70 + i));
            const a = p.tileable ? 0 : rng() * TAU;
            layers.push({ lay, w: worleyNoise(seedOf(p.seed, 80 + i), 1, 0), cos: Math.cos(a), sin: Math.sin(a), ox: rng() * 50, oy: rng() * 50 });
        }
        const q = { u: 0, v: 0 };
        return renderScalar(fr, ctx, buildLUT(p.gradient), (u, v) => {
            let pu = u, pv = v;
            if (warp) { warp(u, v, q); pu = q.u; pv = q.v; }
            let sum = 0;
            for (let i = 0; i < layers.length; i++) {
                const L = layers[i];
                const x = (pu * L.cos - pv * L.sin) * L.lay.sx + (p.tileable ? 0 : L.ox);
                const y = (pu * L.sin + pv * L.cos) * L.lay.sy + (p.tileable ? 0 : L.oy);
                const r = L.w(x, y, L.lay.nx, L.lay.ny);
                const line = 1 - smoothstep(0, 0.42, r.f2 - r.f1);
                sum += Math.pow(line, p.sharpness * 0.55) * (1 - 0.2 * i);
            }
            return clamp01(sum / (1 + 0.55 * (layers.length - 1)) * p.intensity * 1.1 + 0.04);
        });
    });

    // Additive soft stars with a few diffraction spikes. count is in stars for the whole frame.
    function paintStars(img, fr, seed, count, sizeMul) {
        const rng = Lib.mulberry32(seedOf(seed, 5000));
        const data = img.data, w = fr.w, h = fr.h;
        const unitPx = w / fr.ax;
        const base = Math.max(0.3, sizeMul * unitPx / 1024);
        const tints = [[0.74, 0.84, 1], [1, 1, 1], [1, 0.93, 0.78], [1, 0.78, 0.6]];
        for (let i = 0; i < count; i++) {
            const sx = rng() * w, sy = rng() * h;
            const mag = Math.pow(rng(), 5);
            const tr = rng();
            const tint = tr < 0.2 ? tints[0] : (tr < 0.6 ? tints[1] : (tr < 0.85 ? tints[2] : tints[3]));
            const radius = Math.max(0.6, (0.55 + 2.2 * Math.pow(mag, 0.6)) * base);
            const sigma = radius * 0.55;
            const peak = 0.3 + 0.7 * Math.sqrt(mag);
            const reach = Math.ceil(radius * 2.6);
            const x0 = Math.max(0, Math.floor(sx - reach)), x1 = Math.min(w - 1, Math.ceil(sx + reach));
            const y0 = Math.max(0, Math.floor(sy - reach)), y1 = Math.min(h - 1, Math.ceil(sy + reach));
            const inv = 1 / (2 * sigma * sigma);
            for (let y = y0; y <= y1; y++) {
                for (let x = x0; x <= x1; x++) {
                    const dx = x + 0.5 - sx, dy = y + 0.5 - sy;
                    const val = peak * Math.exp(-(dx * dx + dy * dy) * inv) * 255;
                    if (val < 0.5) continue;
                    const o = (y * w + x) * 4;
                    data[o] += val * tint[0]; data[o + 1] += val * tint[1]; data[o + 2] += val * tint[2];
                }
            }
            if (mag > 0.88) {
                const len = radius * (4 + 10 * mag);
                for (let t = -len; t <= len; t++) {
                    const fall = Math.pow(1 - Math.abs(t) / len, 2.4) * peak * 130;
                    const xa = Math.round(sx + t - 0.5), ya = Math.round(sy - 0.5);
                    if (xa >= 0 && xa < w && ya >= 0 && ya < h) { const o = (ya * w + xa) * 4; data[o] += fall * tint[0]; data[o + 1] += fall * tint[1]; data[o + 2] += fall * tint[2]; }
                    const xb = Math.round(sx - 0.5), yb = Math.round(sy + t - 0.5);
                    if (xb >= 0 && xb < w && yb >= 0 && yb < h) { const o = (yb * w + xb) * 4; data[o] += fall * tint[0]; data[o + 1] += fall * tint[1]; data[o + 2] += fall * tint[2]; }
                }
            }
        }
    }

    const NEBULA = [{ pos: 0, color: '#0b0620' }, { pos: 0.3, color: '#3a1a7a' }, { pos: 0.55, color: '#b0357f' }, { pos: 0.8, color: '#ff8a5a' }, { pos: 1, color: '#fff0d2' }];

    define('nebula', 'texture', 'Nebula and stars', [
        P.seed(), P.scale(52), P.int('octaves', 6, 2, 9), P.range('warp', 1, 0, 3, 0.05), P.range('density', 0, -0.3, 0.3, 0.01),
        P.range('dust', 0.5, 0, 1, 0.01), P.range('stars', 40, 0, 100, 1), P.range('starSize', 1, 0.3, 3, 0.05),
        P.color('background', '#04030c'), P.gradient('gradient', NEBULA)
    ], [
        ['emission-nebula', {}],
        ['blue-reflection', { density: 0.02, dust: 0.3, gradient: [{ pos: 0, color: '#030a1c' }, { pos: 0.35, color: '#0b3a7a' }, { pos: 0.7, color: '#4fa3e8' }, { pos: 1, color: '#e6f6ff' }] }],
        ['deep-field', { density: -0.14, dust: 0.2, stars: 85, starSize: 0.8 }],
        ['green-aurora-cloud', { warp: 1.8, dust: 0.4, gradient: [{ pos: 0, color: '#021208' }, { pos: 0.4, color: '#0c5a3a' }, { pos: 0.75, color: '#35d68a' }, { pos: 1, color: '#e4ffd8' }] }]
    ], (p, fr, ctx) => {
        const cells = 100 / p.scale;
        const field = makeFbm({ type: 'simplex', seed: p.seed, cells, octaves: p.octaves, lacunarity: 2.05, gain: 0.52 }, fr);
        const fil = makeFbm({ type: 'simplex', seed: seedOf(p.seed, 91), cells: cells * 1.3, octaves: 5, kind: 'ridged' }, fr);
        const dustField = makeFbm({ type: 'simplex', seed: seedOf(p.seed, 92), cells: cells * 1.3, octaves: 4 }, fr);
        const warp = makeWarp({ type: 'simplex', seed: seedOf(p.seed, 93), cells, strength: p.warp * 0.28, iterations: 1, warpScale: 0.8, warpOctaves: 2, lacunarity: 2 }, fr);
        const lut = buildLUT(p.gradient), n1 = LUT_SIZE - 1, bg = hexRgb(p.background, '#04030c');
        const q = { u: 0, v: 0 };
        const img = Lib.createImage(fr.w, fr.h), data = img.data;
        const colorOut = [0, 0, 0, 255];
        for (let y = 0; y < fr.h; y++) {
            const v = (y + 0.5) * fr.dv;
            for (let x = 0; x < fr.w; x++) {
                const u = (x + 0.5) * fr.du;
                warp(u, v, q);
                const t = field.t(q.u, q.v);
                const cloud = smoothstep(0.2 - p.density, 0.82, t);
                const ridge = fil.t(q.u, q.v);
                const lit = clamp01(cloud * (0.7 + 0.5 * ridge));
                const dust = smoothstep(0.52, 0.82, dustField.t(q.u, q.v)) * p.dust;
                const amount = clamp01(Math.pow(lit, 0.85) * (1 - dust * 0.85));
                putLut(colorOut, 0, lut, n1, clamp01(lit * 0.9 + 0.1 * cloud), dither(x, y));
                const o = (y * fr.w + x) * 4;
                data[o] = bg[0] + (colorOut[0] - bg[0]) * amount;
                data[o + 1] = bg[1] + (colorOut[1] - bg[1]) * amount;
                data[o + 2] = bg[2] + (colorOut[2] - bg[2]) * amount;
                data[o + 3] = 255;
            }
            tick(ctx, y, fr.h, 0, 0.92);
        }
        if (p.stars > 0) paintStars(img, fr, p.seed, Math.round(p.stars * 25 * fr.ax * fr.ay), p.starSize);
        return img;
    });

    // Height of a plain weave (0..1): threads cross over and under every cell, a rounded profile across each thread.
    function weaveHeight(a, b, seed, slub) {
        const i = Math.floor(a), j = Math.floor(b), fa = a - i, fb = b - j;
        const horizontal = Math.pow(Math.sin(Math.PI * fb), 0.8) * (0.3 + 0.7 * (0.5 + 0.5 * Math.cos(Math.PI * (fa - 0.5) + Math.PI * (j & 1))));
        const vertical = Math.pow(Math.sin(Math.PI * fa), 0.8) * (0.3 + 0.7 * (0.5 + 0.5 * Math.cos(Math.PI * (fb - 0.5) + Math.PI * (i & 1) + Math.PI)));
        const jh = 1 + slub * (rand01(j, 3, seed) - 0.5), jv = 1 + slub * (rand01(i, 5, seed) - 0.5);
        return Math.max(horizontal * jh, vertical * jv);
    }

    define('paper', 'texture', 'Paper and canvas', [
        P.seed(), P.seg('kind', ['paper', 'watercolor', 'canvas', 'linen'], 'paper'), P.range('scale', 20, 2, 100, 0.5, { unit: '%' }),
        P.range('relief', 0.55, 0, 1, 0.01), P.angle('light', 135), P.color('tint', '#f3ede0'), P.range('mottling', 0.35, 0, 1, 0.01), P.tileable()
    ], [
        ['fine-paper', {}],
        ['watercolor-paper', { kind: 'watercolor', scale: 14, relief: 0.55, tint: '#f6f1e6' }],
        ['canvas', { kind: 'canvas', scale: 14, relief: 1, tint: '#efe6d2' }],
        ['rough-linen', { kind: 'linen', scale: 10, relief: 0.9, tint: '#d8cbb0', mottling: 0.5 }],
        ['kraft', { kind: 'paper', scale: 28, relief: 0.7, tint: '#b88a58', mottling: 0.5 }]
    ], (p, fr, ctx) => {
        const woven = p.kind === 'canvas' || p.kind === 'linen';
        let height, cellsRef, weave = null;
        if (woven) {
            const threads = (p.kind === 'linen' ? 900 : 1100) / p.scale / 1.5;
            const nx = Math.max(2, Math.round(threads * fr.ax)), ny = Math.max(2, Math.round(threads * fr.ay));
            const sa = nx / fr.ax, sb = ny / fr.ay, ws = seedOf(p.seed, 120), slub = p.kind === 'linen' ? 0.45 : 0.16;
            weave = { sa, sb, seed: seedOf(p.seed, 150) };
            const slubNoise = makeFbm({ type: 'value', seed: seedOf(p.seed, 121), cells: threads * 0.15, octaves: 2, tile: p.tileable }, fr);
            height = (u, v) => weaveHeight(u * sa, v * sb, ws, slub) * (1 + 0.18 * (slubNoise.t(u, v) - 0.5) * (p.kind === 'linen' ? 2.2 : 1));
            cellsRef = threads;
        } else if (p.kind === 'watercolor') {
            cellsRef = 3600 / p.scale / 2.2;
            const bumps = makeFbm({ type: 'perlin', seed: p.seed, cells: cellsRef, octaves: 4, lacunarity: 2, gain: 0.5, tile: p.tileable }, fr);
            height = (u, v) => bumps.t(u, v);
        } else {
            cellsRef = 4000 / p.scale;
            const grain = makeFbm({ type: 'value', seed: p.seed, cells: cellsRef, octaves: 3, lacunarity: 2, gain: 0.55, tile: p.tileable }, fr);
            const fibre1 = makeBase('value', seedOf(p.seed, 130)), fibre2 = makeBase('value', seedOf(p.seed, 131));
            const fc = cellsRef * 0.35;
            height = (u, v) => grain.t(u, v) * 0.75 + 0.25 * (0.5 + 0.5 * (fibre1(u * fc * 0.18, v * fc * 1.6, 0, 0) + fibre2(v * fc * 0.18, u * fc * 1.6, 0, 0)) * 0.5);
        }
        const mott = makeFbm({ type: 'simplex', seed: seedOf(p.seed, 140), cells: 3, octaves: 4, tile: p.tileable }, fr);
        const mott2 = makeFbm({ type: 'perlin', seed: seedOf(p.seed, 141), cells: 18, octaves: 3, tile: p.tileable }, fr);
        const la = rad(p.light);
        const d = 0.3 / cellsRef, lx = Math.cos(la) * d, ly = Math.sin(la) * d;
        const tint = hexRgb(p.tint, '#f3ede0');
        const slopeGain = woven ? 0.34 : (p.kind === 'watercolor' ? 1.1 : 1.5);
        const spread = p.kind === 'linen' ? 0.12 : 0.05;
        return renderRGB(fr, ctx, (u, v, x, y, out) => {
            const slope = height(u + lx, v + ly) - height(u - lx, v - ly);
            let m = 1 - p.relief * slopeGain * slope;
            m *= 1 + p.mottling * (0.12 * (mott.t(u, v) - 0.5) * 2 + 0.05 * (mott2.t(u, v) - 0.5) * 2);
            if (weave) {
                // Every thread has its own shade, and the gaps between threads are darker (cavity shading).
                m *= 1 + spread * ((rand01(Math.floor(v * weave.sb), 11, weave.seed) - 0.5) + (rand01(Math.floor(u * weave.sa), 13, weave.seed) - 0.5));
                m *= 0.74 + 0.26 * Math.min(1, height(u, v) * 1.5);
            }
            m = clamp(m, 0.3, 1.35);
            out[0] = tint[0] * m; out[1] = tint[1] * m; out[2] = tint[2] * m;
        });
    });

    const METALS = {
        steel: [{ pos: 0, color: '#3a3f46' }, { pos: 0.4, color: '#8f959c' }, { pos: 0.75, color: '#d7dbe0' }, { pos: 1, color: '#f7f8fa' }],
        brass: [{ pos: 0, color: '#4a3510' }, { pos: 0.4, color: '#a98a3a' }, { pos: 0.75, color: '#e0c673' }, { pos: 1, color: '#fbf0c3' }],
        copper: [{ pos: 0, color: '#3d1c0e' }, { pos: 0.4, color: '#9a4f2d' }, { pos: 0.75, color: '#d98a5c' }, { pos: 1, color: '#f8d4b6' }],
        dark: [{ pos: 0, color: '#0c0d0f' }, { pos: 0.45, color: '#2b2e33' }, { pos: 0.8, color: '#5b6068' }, { pos: 1, color: '#9aa0a8' }]
    };

    define('brushed-metal', 'texture', 'Brushed metal', [
        P.seed(), P.seg('style', ['linear', 'radial'], 'linear'), P.angle('angle', 0, { showIf: v => v.style === 'linear' }),
        P.range('streaks', 320, 60, 1200, 10), P.range('length', 0.3, 0.03, 1.5, 0.01), P.range('grain', 0.7, 0, 1, 0.01),
        P.range('shine', 0.55, 0, 1, 0.01), P.int('highlights', 2, 1, 4, { showIf: v => v.style === 'radial' }), P.gradient('gradient', METALS.steel)
    ], [
        ['steel', {}],
        ['brass', { gradient: METALS.brass, shine: 0.6 }],
        ['copper-disc', { style: 'radial', gradient: METALS.copper, highlights: 2, streaks: 500 }],
        ['dark-steel', { gradient: METALS.dark, shine: 0.7, grain: 0.8 }]
    ], (p, fr, ctx) => {
        const rot = rotator(p.angle, fr), pt = { x: 0, y: 0 };
        const b1 = makeBase('value', seedOf(p.seed, 160)), b2 = makeBase('value', seedOf(p.seed, 161)), b3 = makeBase('value', seedOf(p.seed, 162));
        const sheen = makeBase('perlin', seedOf(p.seed, 163));
        const cx = 1 / p.length, cxm = Math.max(2, Math.round(p.streaks / 8) * 8);
        const nl = Math.round(p.streaks * 1.5), midx = fr.ax / 2, midy = fr.ay / 2;
        const lineSeed = seedOf(p.seed, 164);
        return renderScalar(fr, ctx, buildLUT(p.gradient), (u, v) => {
            let n, sh;
            if (p.style === 'radial') {
                const dx = u - midx, dy = v - midy;
                const th = Math.atan2(dy, dx) / TAU + 0.5, rho = Math.hypot(dx, dy);
                const k = th * nl;
                n = 0.55 * b1(rho * cx, k, 0, nl) + 0.3 * b2(rho * cx * 2.3, k * 1.0, 0, nl) + 0.15 * b3(rho * cx * 4.1, k, 0, nl);
                const line = rand01(Math.floor(k) % nl, 0, lineSeed) - 0.5;
                n += line * 0.5 * p.grain;
                sh = Math.cos(TAU * th * p.highlights * 2 + 0.7);
                sh = sh * 0.6 + 0.4 * sheen(rho * 2.2, th * 3, 0, 0);
            } else {
                rot(u, v, pt);
                const yk = pt.y * p.streaks;
                n = 0.55 * b1(pt.x * cx, yk, 0, 0) + 0.3 * b2(pt.x * cx * 2.3, yk * 1.7, 0, 0) + 0.15 * b3(pt.x * cx * 4.3, yk * 2.9, 0, 0);
                n += (rand01(Math.floor(yk), 0, lineSeed) - 0.5) * 0.5 * p.grain;
                sh = sheen(pt.x * 0.7, pt.y * 1.9, 0, 0) * 1.4;
            }
            return 0.5 + 0.42 * p.grain * n + 0.5 * p.shine * sh * 0.8;
        });
    });



    // ============================================================ patterns ============================================================

    // Coverage 0..1 of a shape from its signed distance d (positive inside) for a pixel footprint of w, same units.
    const cover = (d, w) => clamp01(0.5 + d / w);

    // Box-filtered square wave (period 2, -1..1) and stripe coverage: exact anti-aliasing at any zoom, no supersampling.
    function squareWave(x, w) {
        w = Math.max(w, 1e-6);
        return 2 * (Math.abs(fract((x - 0.5 * w) * 0.5) - 0.5) - Math.abs(fract((x + 0.5 * w) * 0.5) - 0.5)) / w;
    }

    function stripeCover(x, ratio, w) {
        w = Math.max(w, 1e-6);
        const integral = t => Math.floor(t) * ratio + Math.min(fract(t), ratio);
        return clamp01((integral(x + 0.5 * w) - integral(x - 0.5 * w)) / w);
    }

    function mixInto(out, a, b, t) {
        out[0] = a[0] + (b[0] - a[0]) * t;
        out[1] = a[1] + (b[1] - a[1]) * t;
        out[2] = a[2] + (b[2] - a[2]) * t;
    }

    // Nearest centre of a hexagonal packing (unit spacing, rows sqrt(3)/2 apart, odd rows shifted by half). Fills out.
    function nearestHexCentre(x, y, out) {
        const j = Math.round(y / SQ3_2);
        let best = 1e9;
        for (let dj = -1; dj <= 1; dj++) {
            const jj = j + dj, off = (jj & 1) ? 0.5 : 0;
            const ix = Math.round(x - off);
            const cx = ix + off, cy = jj * SQ3_2;
            const d = (x - cx) * (x - cx) + (y - cy) * (y - cy);
            if (d < best) { best = d; out.cx = cx; out.cy = cy; out.i = ix; out.j = jj; }
        }
        out.d = Math.sqrt(best);
        return out;
    }

    // Shared frame of a repeating pattern: rotated, scaled to `n` cells across the short side; w = footprint in cells.
    function patternFrame(p, fr, n) {
        const rot = rotator(p.angle || 0, fr);
        return { rot, n, w: fr.px * n };
    }

    const colorPair = (a, b) => [P.color('color1', a), P.color('color2', b)];

    define('checkerboard', 'pattern', 'Checkerboard', [
        P.range('tiles', 8, 1, 64, 0.5), P.angle('angle', 0), P.range('softness', 0, 0, 1, 0.01)
    ].concat(colorPair('#f2f2f2', '#2b2b2b')), [
        ['classic', {}],
        ['rotated-big', { tiles: 4, angle: 30, color1: '#fff3d6', color2: '#2a4d69' }],
        ['fine-mono', { tiles: 28, color1: '#ffffff', color2: '#000000' }],
        ['soft-pastel', { tiles: 10, softness: 0.45, color1: '#fde2e4', color2: '#bee1e6' }]
    ], (p, fr, ctx) => {
        const pf = patternFrame(p, fr, p.tiles), pt = { x: 0, y: 0 };
        const a = hexRgb(p.color1), b = hexRgb(p.color2);
        const w = Math.max(pf.w, p.softness);
        return renderRGB(fr, ctx, (u, v, x, y, out) => {
            pf.rot(u, v, pt);
            const c = 0.5 - 0.5 * squareWave(pt.x * p.tiles, w) * squareWave(pt.y * p.tiles, w);
            mixInto(out, a, b, c);
        });
    });

    define('stripes', 'pattern', 'Stripes', [
        P.range('count', 10, 1, 80, 0.5), P.range('ratio', 0.5, 0.05, 0.95, 0.01), P.angle('angle', 45), P.range('softness', 0, 0, 1, 0.01)
    ].concat(colorPair('#f4ecd8', '#c0392b')), [
        ['candy', { count: 10, ratio: 0.5, angle: 45, color1: '#ffffff', color2: '#e63946' }],
        ['pinstripe', { count: 26, ratio: 0.12, angle: 90, color1: '#1d2a44', color2: '#e8e8e8' }],
        ['deck-chair', { count: 7, ratio: 0.5, angle: 90, color1: '#f7f3e8', color2: '#1d6fa5' }],
        ['soft-bands', { count: 6, ratio: 0.5, angle: 0, softness: 0.35, color1: '#ffd6a5', color2: '#9bf6ff' }]
    ], (p, fr, ctx) => {
        const pf = patternFrame(p, fr, p.count), pt = { x: 0, y: 0 };
        const a = hexRgb(p.color1), b = hexRgb(p.color2);
        const w = Math.max(pf.w, p.softness * 0.5);
        return renderRGB(fr, ctx, (u, v, x, y, out) => {
            pf.rot(u, v, pt);
            mixInto(out, b, a, stripeCover(pt.x * p.count + 0.5, p.ratio, w));
        });
    });

    define('grid', 'pattern', 'Grid', [
        P.range('cells', 10, 1, 80, 0.5), P.range('lineWidth', 4, 0.2, 40, 0.1, { unit: '%' }), P.int('majorEvery', 5, 0, 20),
        P.range('majorWidth', 8, 0.2, 60, 0.1, { unit: '%', showIf: v => v.majorEvery > 0 }), P.angle('angle', 0)
    ].concat([P.color('background', '#0f1b2d'), P.color('line', '#2f5d8a'), P.color('majorLine', '#6fb1e6', { showIf: v => v.majorEvery > 0 })]), [
        ['blueprint', {}],
        ['graph-paper', { cells: 24, lineWidth: 2, majorEvery: 5, majorWidth: 4, background: '#fbfaf5', line: '#c9d6df', majorLine: '#7f9bb0' }],
        ['tiles', { cells: 6, lineWidth: 6, majorEvery: 0, background: '#e9e6df', line: '#8c8678' }],
        ['neon-grid', { cells: 14, lineWidth: 3, majorEvery: 0, background: '#0a0014', line: '#ff2bd6' }]
    ], (p, fr, ctx) => {
        const pf = patternFrame(p, fr, p.cells), pt = { x: 0, y: 0 };
        const bg = hexRgb(p.background), ln = hexRgb(p.line), mj = hexRgb(p.majorLine);
        const lw = p.lineWidth / 100, mw = p.majorWidth / 100, every = p.majorEvery;
        return renderRGB(fr, ctx, (u, v, x, y, out) => {
            pf.rot(u, v, pt);
            const gx = pt.x * p.cells, gy = pt.y * p.cells;
            const dx = Math.abs(fract(gx + 0.5) - 0.5), dy = Math.abs(fract(gy + 0.5) - 0.5);
            const minor = Math.max(cover(lw / 2 - dx, pf.w), cover(lw / 2 - dy, pf.w));
            out[0] = bg[0]; out[1] = bg[1]; out[2] = bg[2];
            mixInto(out, bg, ln, minor);
            if (every > 0) {
                const mx = Math.abs(fract(gx / every + 0.5) - 0.5) * every, my = Math.abs(fract(gy / every + 0.5) - 0.5) * every;
                const major = Math.max(cover(mw / 2 - mx, pf.w), cover(mw / 2 - my, pf.w));
                const base = [out[0], out[1], out[2]];
                mixInto(out, base, mj, major);
            }
        });
    });

    define('polka-dots', 'pattern', 'Polka dots', [
        P.range('spacing', 9, 1, 60, 0.5), P.range('dotSize', 0.55, 0.05, 1, 0.01), P.seg('packing', ['square', 'hex'], 'hex'), P.angle('angle', 0),
        P.bool('alternate', false)
    ].concat([P.color('background', '#fff1e6'), P.color('dot', '#e63946'), P.color('dot2', '#1d3557', { showIf: v => v.alternate })]), [
        ['classic-polka', {}],
        ['two-tone', { spacing: 8, dotSize: 0.7, alternate: true, background: '#f1faee', dot: '#e63946', dot2: '#457b9d' }],
        ['fine-hex', { spacing: 24, dotSize: 0.5, packing: 'hex', background: '#14213d', dot: '#fca311' }],
        ['grid-dots', { spacing: 12, dotSize: 0.4, packing: 'square', background: '#ffffff', dot: '#000000' }]
    ], (p, fr, ctx) => {
        const pf = patternFrame(p, fr, p.spacing), pt = { x: 0, y: 0 }, hex = { cx: 0, cy: 0, i: 0, j: 0, d: 0 };
        const bg = hexRgb(p.background), d1 = hexRgb(p.dot), d2 = hexRgb(p.dot2);
        const r = p.dotSize / 2;
        return renderRGB(fr, ctx, (u, v, x, y, out) => {
            pf.rot(u, v, pt);
            const gx = pt.x * p.spacing, gy = pt.y * p.spacing;
            let d, parity;
            if (p.packing === 'hex') {
                nearestHexCentre(gx, gy, hex);
                d = hex.d; parity = (hex.i + hex.j) & 1;
            } else {
                const ix = Math.round(gx), iy = Math.round(gy);
                d = Math.hypot(gx - ix, gy - iy); parity = (ix + iy) & 1;
            }
            out[0] = bg[0]; out[1] = bg[1]; out[2] = bg[2];
            mixInto(out, bg, p.alternate && parity ? d2 : d1, cover(r - d, pf.w));
        });
    });

    define('hexagons', 'pattern', 'Hexagon tiles', [
        P.seed(), P.range('size', 8, 1, 40, 0.5), P.range('gap', 8, 0, 40, 0.1, { unit: '%' }), P.range('variation', 0.8, 0, 1, 0.01),
        P.range('bevel', 0.35, 0, 1, 0.01), P.angle('angle', 0), P.color('grout', '#16181d'),
        P.gradient('gradient', [{ pos: 0, color: '#264653' }, { pos: 0.35, color: '#2a9d8f' }, { pos: 0.65, color: '#e9c46a' }, { pos: 1, color: '#e76f51' }])
    ], [
        ['honeycomb', { gradient: [{ pos: 0, color: '#f6a800' }, { pos: 1, color: '#ffd55a' }], variation: 0.4, gap: 10, grout: '#3b2a00' }],
        ['teal-mosaic', {}],
        ['monochrome', { gradient: 'grayscale', variation: 0.55, gap: 6, bevel: 0.5 }]
    ], (p, fr, ctx) => {
        const rot = rotator(p.angle, fr), pt = { x: 0, y: 0 };
        const R = 1 / (1.5 * p.size), S3 = SQ3 * R;
        const lut = buildLUT(p.gradient), n1 = LUT_SIZE - 1, grout = hexRgb(p.grout);
        const seed = seedOf(p.seed, 200);
        const w = fr.px, gap = p.gap / 100 * R;
        const col = [0, 0, 0, 255];
        return renderRGB(fr, ctx, (u, v, x, y, out) => {
            rot(u, v, pt);
            const px = pt.x, py = pt.y;
            const i0 = Math.round(px / (1.5 * R));
            let best = 1e9, bi = 0, bj = 0, bx = 0, by = 0;
            for (let di = -1; di <= 1; di++) {
                const i = i0 + di, off = (i & 1) ? S3 / 2 : 0;
                const j = Math.round((py - off) / S3);
                const cx = i * 1.5 * R, cy = off + j * S3;
                const d = (px - cx) * (px - cx) + (py - cy) * (py - cy);
                if (d < best) { best = d; bi = i; bj = j; bx = px - cx; by = py - cy; }
            }
            const ax = Math.abs(bx), ay = Math.abs(by);
            const edge = Math.min(S3 / 2 - ay, (S3 - SQ3 * ax - ay) / 2);
            const t = 0.5 + (rand01(bi, bj, seed) - 0.5) * p.variation;
            putLut(col, 0, lut, n1, t, 0);
            const shade = 1 + p.bevel * 0.35 * (smoothstep(gap / 2, gap / 2 + R * 0.35, edge) - 1) + p.bevel * 0.12 * (by > 0 ? -1 : 1) * (1 - smoothstep(0, R * 0.3, edge - gap / 2));
            out[0] = col[0] * shade; out[1] = col[1] * shade; out[2] = col[2] * shade;
            mixInto(out, grout, out, cover(edge - gap / 2, w));
        });
    });

    define('truchet', 'pattern', 'Truchet tiles', [
        P.seed(), P.range('tiles', 8, 2, 48, 0.5), P.seg('style', ['arcs', 'arcs-filled', 'diagonals', 'triangles'], 'arcs'),
        P.range('lineWidth', 14, 2, 50, 0.1, { unit: '%', showIf: v => v.style === 'arcs' || v.style === 'diagonals' }), P.angle('angle', 0)
    ].concat(colorPair('#f1e9da', '#1b4965')), [
        ['arc-weave', { style: 'arcs', lineWidth: 16 }],
        ['filled-circles', { style: 'arcs-filled', tiles: 10, color1: '#ffbe0b', color2: '#3a0ca3' }],
        ['maze-diagonals', { style: 'diagonals', tiles: 14, lineWidth: 12, color1: '#ffffff', color2: '#111111' }],
        ['triangles', { style: 'triangles', tiles: 9, color1: '#ff6b6b', color2: '#4ecdc4' }]
    ], (p, fr, ctx) => {
        const pf = patternFrame(p, fr, p.tiles), pt = { x: 0, y: 0 };
        const a = hexRgb(p.color1), b = hexRgb(p.color2);
        const seed = seedOf(p.seed, 210), lw = p.lineWidth / 100;
        return renderRGB(fr, ctx, (u, v, x, y, out) => {
            pf.rot(u, v, pt);
            const gx = pt.x * p.tiles + 0.5 * p.tiles, gy = pt.y * p.tiles + 0.5 * p.tiles;
            const i = Math.floor(gx), j = Math.floor(gy), fa = gx - i, fb = gy - j;
            const flip = hash2(i, j, seed) & 1;
            const aa = flip ? 1 - fa : fa;
            let c;
            if (p.style === 'arcs' || p.style === 'arcs-filled') {
                const d1 = Math.hypot(aa, fb), d2 = Math.hypot(aa - 1, fb - 1);
                if (p.style === 'arcs') c = cover(lw / 2 - Math.min(Math.abs(d1 - 0.5), Math.abs(d2 - 0.5)), pf.w);
                else c = cover(0.5 - Math.min(d1, d2), pf.w);
            } else if (p.style === 'diagonals') {
                c = cover(lw / 2 - Math.abs(aa - fb) / Math.SQRT2, pf.w);
            } else {
                c = cover((fb - aa) / 1.4142, pf.w);
            }
            mixInto(out, a, b, c);
        });
    });

    define('stained-glass', 'pattern', 'Stained glass', [
        P.seed(), P.range('cells', 7, 2, 40, 0.5), P.range('jitter', 0.9, 0, 1, 0.01), P.range('lead', 6, 0.5, 30, 0.1, { unit: '%' }),
        P.range('glow', 0.5, 0, 1, 0.01), P.color('leadColor', '#14110f'), P.tileable(),
        P.gradient('gradient', [{ pos: 0, color: '#1d4e89' }, { pos: 0.25, color: '#2a9d8f' }, { pos: 0.5, color: '#e9c46a' }, { pos: 0.75, color: '#e76f51' }, { pos: 1, color: '#9b2d5f' }])
    ], [
        ['cathedral', {}],
        ['fine-mosaic', { cells: 16, lead: 4, glow: 0.3 }],
        ['jewel-tones', { cells: 9, jitter: 1, glow: 0.8, gradient: [{ pos: 0, color: '#8b1e3f' }, { pos: 0.33, color: '#14532d' }, { pos: 0.66, color: '#1e3a8a' }, { pos: 1, color: '#b45309' }] }]
    ], (p, fr, ctx) => {
        const lay = noiseLayout('perlin', p.cells, fr, p.tileable);
        const worley = worleyNoise(p.seed, p.jitter, 0);
        const lut = buildLUT(p.gradient), n1 = LUT_SIZE - 1, lead = hexRgb(p.leadColor);
        const shine = makeFbm({ type: 'perlin', seed: seedOf(p.seed, 220), cells: 40, octaves: 3, tile: p.tileable }, fr);
        const col = [0, 0, 0, 255], w = fr.px * lay.sx, lw = p.lead / 100;
        return renderRGB(fr, ctx, (u, v, x, y, out) => {
            const r = worley(u * lay.sx, v * lay.sy, lay.nx, lay.ny);
            putLut(col, 0, lut, n1, (r.id >>> 8) / 16777216, 0);
            const light = 1 + p.glow * 0.5 * (0.42 - smoothstep(0, 0.8, r.f1)) + 0.12 * (shine.f(u, v) / (shine.std * 2));
            out[0] = col[0] * light; out[1] = col[1] * light; out[2] = col[2] * light;
            mixInto(out, lead, out, cover((r.f2 - r.f1) / 2 - lw / 2, w));
        });
    });

    define('halftone', 'pattern', 'Halftone gradient', [
        P.range('dots', 40, 6, 160, 1), P.angle('screenAngle', 45), P.seg('shape', ['dot', 'square', 'line'], 'dot'),
        P.seg('field', ['linear', 'radial'], 'linear'), P.angle('direction', 0, { showIf: v => v.field === 'linear' }),
        P.point('center', 0.5, 0.5, { showIf: v => v.field === 'radial' }), P.range('gain', 1.05, 0.3, 1.6, 0.01), P.bool('invert', false)
    ].concat(colorPair('#101010', '#f6f2e8')), [
        ['newsprint', { dots: 56, color1: '#161616', color2: '#f3ecd9', screenAngle: 45 }],
        ['pop-art', { dots: 30, field: 'radial', color1: '#d62839', color2: '#fdf0d5', shape: 'dot', screenAngle: 15 }],
        ['line-screen', { dots: 48, shape: 'line', screenAngle: 90, direction: 90, color1: '#1d3557', color2: '#f1faee' }],
        ['squares', { dots: 26, shape: 'square', screenAngle: 0, direction: 45, color1: '#e63946', color2: '#f1faee' }]
    ], (p, fr, ctx) => {
        const n = p.dots, rot = rotator(p.screenAngle, fr), pt = { x: 0, y: 0 };
        const ca = Math.cos(rad(p.screenAngle)), sa = Math.sin(rad(p.screenAngle));
        const ink = hexRgb(p.color1), paper = hexRgb(p.color2);
        const da = rad(p.direction), dxv = Math.cos(da), dyv = Math.sin(da);
        const span = Math.abs(fr.ax * dxv) + Math.abs(fr.ay * dyv);
        const cx = p.center.x * fr.ax, cy = p.center.y * fr.ay;
        const maxR = Math.max(Math.hypot(cx, cy), Math.hypot(fr.ax - cx, cy), Math.hypot(cx, fr.ay - cy), Math.hypot(fr.ax - cx, fr.ay - cy));
        const mx = fr.ax / 2, my = fr.ay / 2, w = fr.px * n;
        const amount = (ux, vy) => {
            let t;
            if (p.field === 'radial') t = 1 - Math.hypot(ux - cx, vy - cy) / maxR;
            else t = ((ux - mx) * dxv + (vy - my) * dyv) / span + 0.5;
            if (p.invert) t = 1 - t;
            return clamp01(t) * p.gain;
        };
        return renderRGB(fr, ctx, (u, v, x, y, out) => {
            rot(u, v, pt);
            const gx = pt.x * n, gy = pt.y * n;
            let c = 0;
            if (p.shape === 'line') {
                // Lines are continuous along their length: the thickness follows the field at the pixel itself.
                c = cover(Math.min(1, amount(u, v)) * 0.5 - Math.abs(gy - Math.round(gy)), w);
                if (amount(u, v) > 0.98) c = 1;
            } else {
                const fx = Math.floor(gx), fy = Math.floor(gy);
                for (let j = 0; j <= 1; j++) {
                    for (let i = 0; i <= 1; i++) {
                        const ci = fx + i, cj = fy + j;
                        // The dot centre in image coordinates (inverse of the screen rotation) tells the field value there.
                        const rx = ci / n, ry = cj / n;
                        const a = clamp01(amount(rx * ca - ry * sa + mx, rx * sa + ry * ca + my));
                        const dx = gx - ci, dy = gy - cj;
                        if (p.shape === 'dot') {
                            const r = a < 0.78 ? Math.sqrt(a / Math.PI) : mixv(Math.sqrt(0.78 / Math.PI), 0.7072, (a - 0.78) / 0.22);
                            c = Math.max(c, cover(r - Math.hypot(dx, dy), w));
                        } else {
                            c = Math.max(c, cover(Math.sqrt(a) / 2 - Math.max(Math.abs(dx), Math.abs(dy)), w));
                        }
                    }
                }
            }
            mixInto(out, paper, ink, c);
        });
    });

    const BRICK = [{ pos: 0, color: '#6e2a1c' }, { pos: 0.4, color: '#a8432a' }, { pos: 0.75, color: '#c9613f' }, { pos: 1, color: '#e08b60' }];

    // Herringbone of 2:1 bricks on a unit grid: bricks form diagonal chains (H at k, V at k+1, ...) and the chains are
    // translated by (-1, 1), so every unit square belongs to exactly one brick. Returns the brick anchor and orientation.
    function herringboneBrick(x, y, out) {
        const i = Math.floor(x), j = Math.floor(y);
        const d = i - j;
        let ai, aj, vertical;
        if ((d & 1) === 0) {
            const m = -d / 2, k = i + m;
            ai = i; aj = j; vertical = (k & 1) === 1;
        } else {
            // second square of a horizontal brick (anchor to the left) or of a vertical one (anchor above)
            const m1 = (1 - d) / 2, k1 = j - m1;
            if ((k1 & 1) === 0) { ai = i - 1; aj = j; vertical = false; } else { ai = i; aj = j - 1; vertical = true; }
        }
        out.ai = ai; out.aj = aj; out.vertical = vertical;
        out.x0 = ai; out.y0 = aj; out.x1 = ai + (vertical ? 1 : 2); out.y1 = aj + (vertical ? 2 : 1);
        return out;
    }

    define('bricks', 'pattern', 'Bricks and herringbone', [
        P.seed(), P.seg('layout', ['running', 'stack', 'herringbone', 'basket'], 'running'), P.range('rows', 10, 2, 60, 0.5),
        P.range('ratio', 2.4, 1, 5, 0.05, { showIf: v => v.layout === 'running' || v.layout === 'stack' }), P.range('mortar', 8, 0.5, 40, 0.1, { unit: '%' }),
        P.range('variation', 0.7, 0, 1, 0.01), P.range('bevel', 0.5, 0, 1, 0.01), P.angle('angle', 0), P.color('mortarColor', '#cfc8b8'),
        P.gradient('gradient', BRICK)
    ], [
        ['red-brick', {}],
        ['herringbone-wood', { layout: 'herringbone', rows: 14, mortar: 3, variation: 0.8, mortarColor: '#2b1a0e', gradient: [{ pos: 0, color: '#5a3a1e' }, { pos: 0.5, color: '#9b6b3a' }, { pos: 1, color: '#c99a62' }] }],
        ['stone-basket', { layout: 'basket', rows: 8, mortar: 6, mortarColor: '#4a4a4a', gradient: 'grayscale', variation: 0.5 }],
        ['stack-tiles', { layout: 'stack', rows: 12, ratio: 1, mortar: 5, mortarColor: '#f4f4f4', gradient: 'ocean', variation: 0.6 }]
    ], (p, fr, ctx) => {
        const rot = rotator(p.angle, fr), pt = { x: 0, y: 0 }, hb = {};
        const lut = buildLUT(p.gradient), n1 = LUT_SIZE - 1, mortar = hexRgb(p.mortarColor), col = [0, 0, 0, 255];
        const seed = seedOf(p.seed, 230), m = p.mortar / 100, w = fr.px * p.rows;
        return renderRGB(fr, ctx, (u, v, x, y, out) => {
            rot(u, v, pt);
            const gx = pt.x * p.rows, gy = pt.y * p.rows;
            let id1, id2, edge;
            if (p.layout === 'herringbone') {
                herringboneBrick(gx, gy, hb);
                id1 = hb.ai; id2 = hb.aj;
                edge = Math.min(gx - hb.x0, hb.x1 - gx, gy - hb.y0, hb.y1 - gy);
            } else if (p.layout === 'basket') {
                const bi = Math.floor(gx / 2), bj = Math.floor(gy / 2);
                const vertical = ((bi + bj) & 1) === 1;
                const lx = gx - bi * 2, ly = gy - bj * 2;
                const k = Math.floor(vertical ? lx : ly);
                id1 = bi * 2 + (vertical ? k : 0); id2 = bj * 2 + (vertical ? 0 : k);
                const x0 = bi * 2 + (vertical ? k : 0), y0 = bj * 2 + (vertical ? 0 : k);
                const x1 = x0 + (vertical ? 1 : 2), y1 = y0 + (vertical ? 2 : 1);
                edge = Math.min(gx - x0, x1 - gx, gy - y0, y1 - gy);
            } else {
                const row = Math.floor(gy);
                const shift = p.layout === 'running' && (row & 1) ? 0.5 : 0;
                const bx = gx / p.ratio + shift, bi = Math.floor(bx);
                id1 = bi; id2 = row;
                edge = Math.min((bx - bi) * p.ratio, (1 - (bx - bi)) * p.ratio, gy - row, row + 1 - gy);
            }
            const t = 0.5 + (rand01(id1, id2, seed) - 0.5) * p.variation;
            putLut(col, 0, lut, n1, t, 0);
            const shade = 1 - p.bevel * 0.3 * (1 - smoothstep(m / 2, m / 2 + 0.22, edge));
            out[0] = col[0] * shade; out[1] = col[1] * shade; out[2] = col[2] * shade;
            mixInto(out, mortar, out, cover(edge - m / 2, w));
        });
    });

    define('waves', 'pattern', 'Waves', [
        P.range('bands', 12, 2, 80, 0.5), P.range('amplitude', 0.9, 0, 4, 0.01), P.range('wavelength', 2.2, 0.3, 12, 0.05), P.range('ripple', 0.35, 0, 1, 0.01),
        P.angle('angle', 0), P.seg('style', ['bands', 'gradient'], 'bands'), P.range('cycles', 1, 0.25, 6, 0.05, { showIf: v => v.style === 'gradient' }),
        P.range('ratio', 0.5, 0.05, 0.95, 0.01, { showIf: v => v.style === 'bands' })
    ].concat([P.color('color1', '#0b3954', { showIf: v => v.style === 'bands' }), P.color('color2', '#bfd7ea', { showIf: v => v.style === 'bands' }), P.gradient('gradient', 'ocean', { showIf: v => v.style === 'gradient' })]), [
        ['ocean-bands', {}],
        ['sunset-ripples', { style: 'gradient', gradient: 'sunset', bands: 8, amplitude: 1.3, wavelength: 1.6, cycles: 1 }],
        ['zebra-waves', { bands: 18, amplitude: 1.6, wavelength: 3.2, ratio: 0.5, color1: '#111111', color2: '#f5f5f5' }]
    ], (p, fr, ctx) => {
        const pf = patternFrame(p, fr, p.bands), pt = { x: 0, y: 0 };
        const a = hexRgb(p.color1), b = hexRgb(p.color2);
        const lut = buildLUT(p.gradient), n1 = LUT_SIZE - 1, col = [0, 0, 0, 255];
        const f = TAU * p.wavelength, f2 = f * 2.7;
        return renderRGB(fr, ctx, (u, v, x, y, out) => {
            pf.rot(u, v, pt);
            const s = Math.sin(f * pt.x), s2 = Math.sin(f2 * pt.x + 1.3 + 0.6 * pt.y * p.bands);
            const phase = pt.y * p.bands + p.amplitude * (s + p.ripple * 0.35 * s2);
            const slope = p.amplitude * Math.abs(f * Math.cos(f * pt.x) + p.ripple * 0.35 * f2 * Math.cos(f2 * pt.x + 1.3)) / p.bands;
            if (p.style === 'gradient') {
                putLut(col, 0, lut, n1, pingPong(phase * 0.5 * p.cycles / p.bands * 1 + 0), dither(x, y));
                out[0] = col[0]; out[1] = col[1]; out[2] = col[2];
            } else {
                mixInto(out, b, a, stripeCover(phase, p.ratio, pf.w * (1 + slope * 0.7)));
            }
        });
    });

    // ============================================================== math ==============================================================

    define('gradient', 'math', 'Gradient', [
        P.select('type', ['linear', 'radial', 'conic', 'diamond', 'square', 'reflected'], 'linear'),
        P.angle('angle', 90, { showIf: v => v.type === 'linear' || v.type === 'conic' || v.type === 'reflected' }),
        P.point('center', 0.5, 0.5), P.range('size', 100, 10, 400, 1, { unit: '%', showIf: v => v.type !== 'conic' }),
        P.seg('repeat', ['none', 'repeat', 'mirror'], 'none'), P.range('repeats', 3, 0.25, 24, 0.25, { showIf: v => v.repeat !== 'none' }),
        P.gradient('gradient', 'sunset')
    ], [
        ['sunset-sky', { type: 'linear', angle: 90, repeat: 'none', gradient: 'sunset' }],
        ['radial-glow', { type: 'radial', repeat: 'none', size: 90, gradient: 'inferno' }],
        ['color-wheel', { type: 'conic', angle: 0, repeat: 'none', gradient: 'rainbow' }],
        ['sunburst', { type: 'conic', angle: 0, repeat: 'mirror', repeats: 12, gradient: [{ pos: 0, color: '#ffd23f' }, { pos: 1, color: '#ee4266' }] }],
        ['chrome-bands', { type: 'reflected', angle: 90, repeat: 'repeat', repeats: 2.5, size: 70, gradient: [{ pos: 0, color: '#10131a' }, { pos: 0.45, color: '#8791a3' }, { pos: 0.55, color: '#f4f7fb' }, { pos: 1, color: '#2a2f3a' }] }]
    ], (p, fr, ctx) => {
        const cx = p.center.x * fr.ax, cy = p.center.y * fr.ay, scale = p.size / 100;
        const a = rad(p.angle), dx = Math.cos(a), dy = Math.sin(a);
        const half = 0.5 * Math.hypot(fr.ax, fr.ay);
        const extent = Math.abs(fr.ax * dx) + Math.abs(fr.ay * dy);
        const lut = buildLUT(p.gradient);
        return renderScalar(fr, ctx, lut, (u, v) => {
            const x = u - cx, y = v - cy;
            let t;
            switch (p.type) {
                case 'radial': t = Math.hypot(x, y) / (half * scale); break;
                case 'conic': t = fract((Math.atan2(y, x) - a) / TAU); break;
                case 'diamond': t = (Math.abs(x) + Math.abs(y)) / ((fr.ax + fr.ay) * 0.5 * scale); break;
                case 'square': t = Math.max(Math.abs(x), Math.abs(y)) / (Math.max(fr.ax, fr.ay) * 0.5 * scale); break;
                case 'reflected': t = Math.abs((x * dx + y * dy) / (extent * scale)) * 2; break;
                default: t = (x * dx + y * dy) / (extent * scale) + 0.5;
            }
            if (p.repeat === 'repeat') return fract(t * p.repeats);
            if (p.repeat === 'mirror') return pingPong(t * p.repeats * 0.5);
            return t;
        });
    });

    // ----- escape-time fractals -----

    const BAIL = 65536, LN_B = Math.log(256);

    // Smooth iteration count of z -> z^d + c (Mandelbrot when julia is false: z starts at 0 and c is the point).
    function escapePower(zr, zi, cr, ci, maxIter, d) {
        if (d === 2) {
            let n = 0, r2 = 0;
            while (n < maxIter) {
                const a = zr * zr, b = zi * zi;
                r2 = a + b;
                if (r2 > BAIL) break;
                zi = 2 * zr * zi + ci;
                zr = a - b + cr;
                n++;
            }
            return n >= maxIter ? -1 : n + 1 - Math.log(0.5 * Math.log(r2) / LN_B) * 1.4426950408889634;
        }
        let n = 0, r2 = 0;
        while (n < maxIter) {
            r2 = zr * zr + zi * zi;
            if (r2 > BAIL) break;
            let pr = zr, pi = zi;
            for (let k = 1; k < d; k++) { const t = pr * zr - pi * zi; pi = pr * zi + pi * zr; pr = t; }
            zr = pr + cr; zi = pi + ci;
            n++;
        }
        return n >= maxIter ? -1 : n + 1 - Math.log(0.5 * Math.log(r2) / LN_B) / Math.log(d);
    }

    function escapeShip(cr, ci, maxIter) {
        let zr = 0, zi = 0, n = 0, r2 = 0;
        while (n < maxIter) {
            const a = zr * zr, b = zi * zi;
            r2 = a + b;
            if (r2 > BAIL) break;
            zi = 2 * Math.abs(zr * zi) + ci;
            zr = a - b + cr;
            n++;
        }
        return n >= maxIter ? -1 : n + 1 - Math.log(0.5 * Math.log(r2) / LN_B) * 1.4426950408889634;
    }

    // Supersampled pixel loop: fn(u, v, out) fills out[0..2] with a colour; ss x ss samples per pixel are averaged.
    function renderSupersampled(fr, ctx, ss, fn) {
        const img = Lib.createImage(fr.w, fr.h), data = img.data, w = fr.w, h = fr.h;
        const out = [0, 0, 0], inv = 1 / (ss * ss);
        for (let y = 0; y < h; y++) {
            for (let x = 0; x < w; x++) {
                let r = 0, g = 0, b = 0;
                for (let sy = 0; sy < ss; sy++) {
                    for (let sx = 0; sx < ss; sx++) {
                        fn((x + (sx + 0.5) / ss) * fr.du, (y + (sy + 0.5) / ss) * fr.dv, out);
                        r += out[0]; g += out[1]; b += out[2];
                    }
                }
                const o = (y * w + x) * 4, d = dither(x, y);
                data[o] = r * inv + d; data[o + 1] = g * inv + d; data[o + 2] = b * inv + d; data[o + 3] = 255;
            }
            tick(ctx, y, h);
        }
        return img;
    }

    const aaSamples = (p, fr) => (fr.draft ? 1 : (p.antialias === '3x3' ? 3 : (p.antialias === '2x2' ? 2 : 1)));

    // Colour of an escape value: mu < 0 is inside the set.
    function escapeColorizer(p, maxIter) {
        const lut = buildLUT(p.gradient), n1 = LUT_SIZE - 1, inside = hexRgb(p.inside), tmp = [0, 0, 0, 255];
        const lmax = Math.log(maxIter + 1);
        return (mu, out) => {
            if (mu < 0) { out[0] = inside[0]; out[1] = inside[1]; out[2] = inside[2]; return; }
            const s = p.mapping === 'linear' ? mu / maxIter : Math.log(mu + 1) / lmax;
            putLut(tmp, 0, lut, n1, pingPong((s * p.cycles + p.offset) * 0.5), 0);
            out[0] = tmp[0]; out[1] = tmp[1]; out[2] = tmp[2];
        };
    }

    const viewParams = (cx, cy, zoom) => [
        P.range('centerX', cx, -2.5, 2.5, 0.000001), P.range('centerY', cy, -2.5, 2.5, 0.000001), P.range('zoom', zoom, 0, 36, 0.05)
    ];
    const colorParams = (grad, iterations, cycles) => [
        P.int('iterations', iterations, 20, 4000, { step: 10 }), P.gradient('gradient', grad), P.range('cycles', cycles, 0.25, 12, 0.05),
        P.range('offset', 0, 0, 2, 0.01), P.seg('mapping', ['log', 'linear'], 'log'), P.color('inside', '#000000'),
        P.seg('antialias', ['off', '2x2', '3x3'], '2x2')
    ];

    define('mandelbrot', 'math', 'Mandelbrot set', viewParams(-0.5, 0, 0).concat([P.int('power', 2, 2, 6)], colorParams('magma', 200, 3)), [
        ['classic', {}],
        ['seahorse-valley', { centerX: -0.743645, centerY: 0.131826, zoom: 11, iterations: 600, cycles: 2.5, gradient: 'ocean' }],
        ['elephant-valley', { centerX: 0.254987, centerY: 0.000568, zoom: 14, iterations: 700, cycles: 3, gradient: 'sunset' }],
        ['spiral-minibrot', { centerX: -0.7453, centerY: 0.1127, zoom: 8.5, iterations: 500, cycles: 3.5, gradient: 'viridis' }],
        ['multibrot-3', { power: 3, iterations: 120, cycles: 2, gradient: 'plasma' }]
    ], (p, fr, ctx) => {
        const iters = Math.round(p.iterations * (fr.draft ? 0.5 : 1));
        const span = 3.2 / Math.pow(2, p.zoom), color = escapeColorizer(p, iters), d = p.power;
        const mx = fr.ax / 2, my = fr.ay / 2;
        return renderSupersampled(fr, ctx, aaSamples(p, fr), (u, v, out) => {
            const cr = p.centerX + (u - mx) * span, ci = p.centerY - (v - my) * span;
            let mu;
            if (d === 2) {
                const q = (cr - 0.25) * (cr - 0.25) + ci * ci;
                // main cardioid and the period-2 bulb are inside without iterating
                if (q * (q + (cr - 0.25)) <= 0.25 * ci * ci || (cr + 1) * (cr + 1) + ci * ci <= 0.0625) mu = -1;
                else mu = escapePower(0, 0, cr, ci, iters, 2);
            } else {
                mu = escapePower(0, 0, cr, ci, iters, d);
            }
            color(mu, out);
        });
    });

    define('julia', 'math', 'Julia set', [
        P.range('cRe', -0.8, -2, 2, 0.0001), P.range('cIm', 0.156, -2, 2, 0.0001), P.range('centerX', 0, -2.5, 2.5, 0.0001), P.range('centerY', 0, -2.5, 2.5, 0.0001),
        P.range('zoom', 0, 0, 30, 0.05), P.int('power', 2, 2, 6)
    ].concat(colorParams('inferno', 250, 3)), [
        ['dragon', { cRe: -0.8, cIm: 0.156 }],
        ['dendrite', { cRe: 0, cIm: 1, iterations: 300, gradient: 'ice' }],
        ['rabbit', { cRe: -0.123, cIm: 0.745, gradient: 'sunset' }],
        ['siegel-disk', { cRe: -0.391, cIm: -0.587, iterations: 400, gradient: 'viridis' }],
        ['galaxy', { cRe: -0.7269, cIm: 0.1889, iterations: 300, cycles: 4, gradient: 'plasma' }]
    ], (p, fr, ctx) => {
        const iters = Math.round(p.iterations * (fr.draft ? 0.5 : 1));
        const span = 3.2 / Math.pow(2, p.zoom), color = escapeColorizer(p, iters);
        const mx = fr.ax / 2, my = fr.ay / 2;
        return renderSupersampled(fr, ctx, aaSamples(p, fr), (u, v, out) => {
            color(escapePower(p.centerX + (u - mx) * span, p.centerY - (v - my) * span, p.cRe, p.cIm, iters, p.power), out);
        });
    });

    define('burning-ship', 'math', 'Burning Ship', viewParams(-0.5, -0.5, 0).concat(colorParams('fire', 220, 2.5)), [
        ['overview', {}],
        ['the-ship', { centerX: -1.7618, centerY: -0.0285, zoom: 5.6, iterations: 400, cycles: 3 }],
        ['armada', { centerX: -1.8625, centerY: -0.0036, zoom: 10, iterations: 600, gradient: 'ocean', cycles: 3.5 }]
    ], (p, fr, ctx) => {
        const iters = Math.round(p.iterations * (fr.draft ? 0.5 : 1));
        const span = 3.2 / Math.pow(2, p.zoom), color = escapeColorizer(p, iters);
        const mx = fr.ax / 2, my = fr.ay / 2;
        // The imaginary axis points down here, which turns the ship upright.
        return renderSupersampled(fr, ctx, aaSamples(p, fr), (u, v, out) => {
            color(escapeShip(p.centerX + (u - mx) * span, p.centerY + (v - my) * span, iters), out);
        });
    });

    // Roots of z^3 - 2z + 2 (Durand-Kerner), the cubic with an attracting cycle that starves Newton's method.
    function cubicRoots() {
        let r = [[1, 0], [-0.5, 0.8], [-0.5, -0.8]];
        const f = (zr, zi) => {
            const z2r = zr * zr - zi * zi, z2i = 2 * zr * zi;
            return [z2r * zr - z2i * zi - 2 * zr + 2, z2r * zi + z2i * zr - 2 * zi];
        };
        for (let it = 0; it < 200; it++) {
            r = r.map((z, i) => {
                const fv = f(z[0], z[1]);
                let dr = 1, di = 0;
                r.forEach((o, j) => {
                    if (j === i) return;
                    const er = z[0] - o[0], ei = z[1] - o[1];
                    const t = dr * er - di * ei; di = dr * ei + di * er; dr = t;
                }, 0);
                const den = dr * dr + di * di || 1e-12;
                return [z[0] - (fv[0] * dr + fv[1] * di) / den, z[1] - (fv[1] * dr - fv[0] * di) / den];
            });
        }
        return r;
    }

    define('newton', 'math', 'Newton fractal', [
        P.seg('polynomial', ['roots-of-unity', 'cubic-cycle'], 'roots-of-unity'), P.int('degree', 3, 3, 9, { showIf: v => v.polynomial === 'roots-of-unity' }),
        P.range('relaxation', 1, 0.4, 2, 0.01), P.range('centerX', 0, -3, 3, 0.0001), P.range('centerY', 0, -3, 3, 0.0001), P.range('zoom', 0, -1, 24, 0.05),
        P.int('iterations', 50, 8, 400), P.range('shading', 0.75, 0, 1, 0.01), P.gradient('gradient', 'rainbow'), P.seg('antialias', ['off', '2x2'], '2x2')
    ], [
        ['classic-cubic', { degree: 3 }],
        ['quintic', { degree: 5, gradient: 'sunset' }],
        ['nova', { degree: 3, relaxation: 1.35, iterations: 80, gradient: 'viridis', centerX: -0.3 }],
        ['starved-cubic', { polynomial: 'cubic-cycle', zoom: -0.3, gradient: 'neon' }]
    ], (p, fr, ctx) => {
        const lut = buildLUT(p.gradient), n1 = LUT_SIZE - 1, tmp = [0, 0, 0, 255];
        const cubic = p.polynomial === 'cubic-cycle';
        const roots = cubic ? cubicRoots() : Array.from({ length: p.degree }, (_, k) => [Math.cos(TAU * k / p.degree), Math.sin(TAU * k / p.degree)]);
        const n = roots.length, span = 4 / Math.pow(2, p.zoom), mx = fr.ax / 2, my = fr.ay / 2, a = p.relaxation;
        const iters = fr.draft ? Math.min(p.iterations, 30) : p.iterations, deg = p.degree;
        return renderSupersampled(fr, ctx, fr.draft ? 1 : (p.antialias === '2x2' ? 2 : 1), (u, v, out) => {
            let zr = p.centerX + (u - mx) * span, zi = p.centerY - (v - my) * span;
            let it = 0, hit = -1;
            for (; it < iters; it++) {
                let fr_, fi, dr, di;
                if (cubic) {
                    const z2r = zr * zr - zi * zi, z2i = 2 * zr * zi;
                    fr_ = z2r * zr - z2i * zi - 2 * zr + 2; fi = z2r * zi + z2i * zr - 2 * zi;
                    dr = 3 * z2r - 2; di = 3 * z2i;
                } else {
                    let pr = 1, pi = 0;
                    for (let k = 1; k < deg; k++) { const t = pr * zr - pi * zi; pi = pr * zi + pi * zr; pr = t; }
                    fr_ = pr * zr - pi * zi - 1; fi = pr * zi + pi * zr;
                    dr = deg * pr; di = deg * pi;
                }
                const den = dr * dr + di * di;
                if (den < 1e-18) break;
                const sr = (fr_ * dr + fi * di) / den, si = (fi * dr - fr_ * di) / den;
                zr -= a * sr; zi -= a * si;
                if (sr * sr + si * si < 1e-12) {
                    let best = 1e9;
                    for (let k = 0; k < n; k++) {
                        const d2 = (zr - roots[k][0]) * (zr - roots[k][0]) + (zi - roots[k][1]) * (zi - roots[k][1]);
                        if (d2 < best) { best = d2; hit = k; }
                    }
                    break;
                }
            }
            if (hit < 0) { out[0] = out[1] = out[2] = 8; return; }
            putLut(tmp, 0, lut, n1, hit / n, 0);
            const shade = 1 - p.shading + p.shading * Math.pow(0.86, it);
            out[0] = tmp[0] * shade; out[1] = tmp[1] * shade; out[2] = tmp[2] * shade;
        });
    });

    // ----- strokes: anti-aliased line and disc drawing straight into an opaque RGBA image -----

    function newInk(fr, bg) {
        const img = Lib.createImage(fr.w, fr.h), d = img.data;
        for (let i = 0; i < d.length; i += 4) { d[i] = bg[0]; d[i + 1] = bg[1]; d[i + 2] = bg[2]; d[i + 3] = 255; }
        return img;
    }

    // One segment with round caps (butt = false) or flat ends (butt = true) blended over the image. hw = half width in pixels.
    function strokeSegment(img, x0, y0, x1, y1, hw, rgb, alpha, butt) {
        const w = img.width, h = img.height, data = img.data;
        const pad = hw + 1;
        const minX = Math.max(0, Math.floor(Math.min(x0, x1) - pad)), maxX = Math.min(w - 1, Math.ceil(Math.max(x0, x1) + pad));
        const minY = Math.max(0, Math.floor(Math.min(y0, y1) - pad)), maxY = Math.min(h - 1, Math.ceil(Math.max(y0, y1) + pad));
        if (maxX < minX || maxY < minY) return;
        const dx = x1 - x0, dy = y1 - y0, len2 = dx * dx + dy * dy, len = Math.sqrt(len2);
        const ux = len > 0 ? dx / len : 1, uy = len > 0 ? dy / len : 0;
        for (let y = minY; y <= maxY; y++) {
            for (let x = minX; x <= maxX; x++) {
                const px = x + 0.5 - x0, py = y + 0.5 - y0;
                let cov;
                if (butt) {
                    const along = px * ux + py * uy, across = Math.abs(-px * uy + py * ux);
                    cov = clamp01(hw + 0.5 - across) * clamp01(along + 0.5) * clamp01(len - along + 0.5);
                } else {
                    const t = len2 > 0 ? clamp((px * dx + py * dy) / len2, 0, 1) : 0;
                    cov = clamp01(hw + 0.5 - Math.hypot(px - t * dx, py - t * dy));
                }
                if (cov <= 0) continue;
                const a = cov * alpha, o = (y * w + x) * 4;
                data[o] += (rgb[0] - data[o]) * a;
                data[o + 1] += (rgb[1] - data[o + 1]) * a;
                data[o + 2] += (rgb[2] - data[o + 2]) * a;
            }
        }
    }

    function strokeDisc(img, cx, cy, radius, rgb, alpha) {
        const w = img.width, h = img.height, data = img.data;
        const minX = Math.max(0, Math.floor(cx - radius - 1)), maxX = Math.min(w - 1, Math.ceil(cx + radius + 1));
        const minY = Math.max(0, Math.floor(cy - radius - 1)), maxY = Math.min(h - 1, Math.ceil(cy + radius + 1));
        for (let y = minY; y <= maxY; y++) {
            for (let x = minX; x <= maxX; x++) {
                const cov = clamp01(radius + 0.5 - Math.hypot(x + 0.5 - cx, y + 0.5 - cy));
                if (cov <= 0) continue;
                const a = cov * alpha, o = (y * w + x) * 4;
                data[o] += (rgb[0] - data[o]) * a;
                data[o + 1] += (rgb[1] - data[o + 1]) * a;
                data[o + 2] += (rgb[2] - data[o + 2]) * a;
            }
        }
    }

    // Soft glow of what the strokes added to the background: downsample, blur, add back with bilinear upsampling.
    function applyGlow(img, bg, amount) {
        if (!(amount > 0)) return;
        const w = img.width, h = img.height, data = img.data;
        const f = Math.max(1, Math.ceil(Math.max(w, h) / 480));
        const sw = Math.max(1, Math.ceil(w / f)), sh = Math.max(1, Math.ceil(h / f));
        const sum = new Float32Array(sw * sh * 3);
        for (let y = 0; y < h; y++) {
            const row = ((y / f) | 0) * sw;
            for (let x = 0; x < w; x++) {
                const o = (y * w + x) * 4, s = (row + ((x / f) | 0)) * 3;
                sum[s] += Math.max(0, data[o] - bg[0]); sum[s + 1] += Math.max(0, data[o + 1] - bg[1]); sum[s + 2] += Math.max(0, data[o + 2] - bg[2]);
            }
        }
        const small = new Uint8ClampedArray(sw * sh * 4), inv = 1 / (f * f);
        for (let i = 0; i < sw * sh; i++) {
            small[i * 4] = sum[i * 3] * inv; small[i * 4 + 1] = sum[i * 3 + 1] * inv; small[i * 4 + 2] = sum[i * 3 + 2] * inv; small[i * 4 + 3] = 255;
        }
        let blurred = small;
        const radius = Math.max(1, Math.round(sw * 0.02));
        for (let pass = 0; pass < 3; pass++) blurred = Lib.boxBlurRGBA(blurred, sw, sh, radius);
        const gain = amount * 2.2;
        for (let y = 0; y < h; y++) {
            const gy = clamp((y + 0.5) / f - 0.5, 0, sh - 1), y0 = Math.floor(gy), y1 = Math.min(sh - 1, y0 + 1), ty = gy - y0;
            for (let x = 0; x < w; x++) {
                const gx = clamp((x + 0.5) / f - 0.5, 0, sw - 1), x0 = Math.floor(gx), x1 = Math.min(sw - 1, x0 + 1), tx = gx - x0;
                const a = (y0 * sw + x0) * 4, b = (y0 * sw + x1) * 4, c = (y1 * sw + x0) * 4, d = (y1 * sw + x1) * 4, o = (y * w + x) * 4;
                for (let k = 0; k < 3; k++) {
                    const top = blurred[a + k] + (blurred[b + k] - blurred[a + k]) * tx, bot = blurred[c + k] + (blurred[d + k] - blurred[c + k]) * tx;
                    data[o + k] += (top + (bot - top) * ty) * gain;
                }
            }
        }
    }

    // Common controls of the curve generators.
    const inkParams = (width, grad) => [
        P.range('lineWidth', width, 0.3, 14, 0.1, { unit: 'px' }), P.seg('colorMode', ['gradient', 'solid'], 'gradient'),
        P.gradient('gradient', grad, { showIf: v => v.colorMode === 'gradient' }), P.color('color', '#7ee0ff', { showIf: v => v.colorMode === 'solid' }),
        P.range('glow', 0.3, 0, 1, 0.01), P.color('background', '#07080f')
    ];

    // Draws a polyline (points in image pixels) with a colour that runs along it; t0..t1 is the gradient range it covers.
    function strokePath(img, pts, count, hw, p, lut, t0, t1, ctx, alpha, butt) {
        const n1 = LUT_SIZE - 1, tmp = [0, 0, 0, 255], solid = hexRgb(p.color);
        const gradient = p.colorMode === 'gradient';
        for (let i = 0; i + 1 < count; i++) {
            let rgb = solid;
            if (gradient) { putLut(tmp, 0, lut, n1, t0 + (t1 - t0) * (i / (count - 1)), 0); rgb = tmp; }
            strokeSegment(img, pts[i * 2], pts[i * 2 + 1], pts[i * 2 + 2], pts[i * 2 + 3], hw, rgb, alpha == null ? 1 : alpha, butt);
            if ((i & 1023) === 1023) checkCancel(ctx);
        }
    }

    const gcd = (a, b) => (b ? gcd(b, a % b) : a);
    const lineHalf = (p, fr) => Math.max(0.3, p.lineWidth * fr.short / 1024) / 2;

    // Fits curve points (any units) centred into the image; returns the pixel-space Float64Array.
    function fitCurve(raw, count, fr, margin) {
        let minX = 1e18, maxX = -1e18, minY = 1e18, maxY = -1e18;
        for (let i = 0; i < count; i++) {
            const x = raw[i * 2], y = raw[i * 2 + 1];
            if (x < minX) minX = x; if (x > maxX) maxX = x; if (y < minY) minY = y; if (y > maxY) maxY = y;
        }
        const span = Math.max(maxX - minX, maxY - minY, 1e-9);
        const k = fr.short * (1 - 2 * margin) / span;
        const ox = fr.w / 2 - (minX + maxX) / 2 * k, oy = fr.h / 2 - (minY + maxY) / 2 * k;
        const out = new Float64Array(count * 2);
        for (let i = 0; i < count; i++) { out[i * 2] = raw[i * 2] * k + ox; out[i * 2 + 1] = raw[i * 2 + 1] * k + oy; }
        return out;
    }

    function curveRender(p, fr, ctx, build) {
        const bg = hexRgb(p.background), lut = buildLUT(p.gradient);
        const img = newInk(fr, bg), hw = lineHalf(p, fr);
        build((raw, count, t0, t1, alpha) => {
            const pts = fitCurve(raw, count, fr, 0.07);
            strokePath(img, pts, count, hw, p, lut, t0, t1, ctx, alpha, false);
        });
        checkCancel(ctx);
        applyGlow(img, bg, p.glow);
        return img;
    }

    define('spirograph', 'math', 'Spirograph', [
        P.seg('type', ['hypotrochoid', 'epitrochoid'], 'hypotrochoid'), P.int('outer', 9, 3, 90), P.int('inner', 4, 1, 89), P.range('pen', 1.1, 0.1, 3, 0.01),
        P.int('layers', 1, 1, 8)
    ].concat(inkParams(1.6, 'neon')), [
        ['classic-star', { type: 'hypotrochoid', outer: 5, inner: 3, pen: 1, layers: 1, lineWidth: 2.4 }],
        ['neon-rosette', { outer: 9, inner: 4, pen: 1.1, layers: 1, glow: 0.7, lineWidth: 1.6 }],
        ['dense-mandala', { outer: 89, inner: 53, pen: 0.9, layers: 1, lineWidth: 0.7, gradient: 'sunset', glow: 0.35 }],
        ['epicycle-bloom', { type: 'epitrochoid', outer: 7, inner: 3, pen: 1.6, layers: 3, lineWidth: 1.2, gradient: 'ocean' }]
    ], (p, fr, ctx) => curveRender(p, fr, ctx, draw => {
        const R = p.outer, r = p.type === 'hypotrochoid' ? Math.min(p.inner, R - 1) : p.inner;
        const turns = r / gcd(R, r), ratio = p.type === 'hypotrochoid' ? (R - r) / r : (R + r) / r;
        const count = clamp(Math.ceil(TAU * turns / (0.012 / (1 + Math.abs(ratio)))), 400, 90000);
        for (let layer = 0; layer < p.layers; layer++) {
            const raw = new Float64Array(count * 2), d = p.pen * r * (1 - layer * 0.18);
            for (let i = 0; i < count; i++) {
                const t = TAU * turns * i / (count - 1);
                if (p.type === 'hypotrochoid') {
                    raw[i * 2] = (R - r) * Math.cos(t) + d * Math.cos(ratio * t);
                    raw[i * 2 + 1] = (R - r) * Math.sin(t) - d * Math.sin(ratio * t);
                } else {
                    raw[i * 2] = (R + r) * Math.cos(t) - d * Math.cos(ratio * t);
                    raw[i * 2 + 1] = (R + r) * Math.sin(t) - d * Math.sin(ratio * t);
                }
            }
            draw(raw, count, layer / Math.max(1, p.layers), (layer + 1) / Math.max(1, p.layers), 1);
        }
    }));

    define('rose', 'math', 'Rose curve', [
        P.int('petals', 5, 1, 21), P.int('divisor', 1, 1, 12), P.int('layers', 3, 1, 8), P.range('twist', 12, 0, 90, 0.5, { unit: '°' })
    ].concat(inkParams(1.4, 'sunset')), [
        ['five-petal', { petals: 5, divisor: 1, layers: 1, lineWidth: 2.6 }],
        ['layered-bloom', { petals: 7, divisor: 3, layers: 5, twist: 8, glow: 0.5 }],
        ['dense-rosette', { petals: 8, divisor: 5, layers: 4, twist: 15, lineWidth: 0.8, gradient: 'neon', glow: 0.55 }]
    ], (p, fr, ctx) => curveRender(p, fr, ctx, draw => {
        const k = p.petals / p.divisor, count = 5200;
        const sweep = TAU * p.divisor;
        for (let layer = 0; layer < p.layers; layer++) {
            const raw = new Float64Array(count * 2), scale = 1 - layer * 0.14, rot = rad(p.twist) * layer;
            for (let i = 0; i < count; i++) {
                const th = sweep * i / (count - 1), r = Math.cos(k * th) * scale;
                raw[i * 2] = r * Math.cos(th + rot); raw[i * 2 + 1] = r * Math.sin(th + rot);
            }
            draw(raw, count, layer / p.layers, (layer + 1) / p.layers, 1);
        }
    }));

    define('lissajous', 'math', 'Lissajous curve', [
        P.int('a', 3, 1, 14), P.int('b', 2, 1, 14), P.range('phase', 90, 0, 360, 1, { unit: '°' }), P.int('layers', 6, 1, 16), P.range('spread', 12, 0, 90, 0.5, { unit: '°' })
    ].concat(inkParams(1.3, 'ocean')), [
        ['knot', { a: 3, b: 2, phase: 90, layers: 1, lineWidth: 3 }],
        ['ribbon', { a: 3, b: 4, phase: 60, layers: 14, spread: 6, lineWidth: 1, glow: 0.5, gradient: 'neon' }],
        ['weave', { a: 5, b: 4, phase: 30, layers: 8, spread: 10, lineWidth: 1.2, gradient: 'sunset' }]
    ], (p, fr, ctx) => curveRender(p, fr, ctx, draw => {
        const count = 4000;
        for (let layer = 0; layer < p.layers; layer++) {
            const raw = new Float64Array(count * 2), ph = rad(p.phase + layer * p.spread);
            for (let i = 0; i < count; i++) {
                const t = TAU * i / (count - 1);
                raw[i * 2] = Math.sin(p.a * t + ph); raw[i * 2 + 1] = Math.sin(p.b * t);
            }
            draw(raw, count, layer / p.layers, (layer + 1) / p.layers, 1);
        }
    }));

    define('superformula', 'math', 'Superformula', [
        P.range('m', 6, 0, 24, 0.1), P.range('n1', 1.2, 0.1, 12, 0.05), P.range('n2', 1.6, 0.1, 12, 0.05), P.range('n3', 1.6, 0.1, 12, 0.05),
        P.range('a', 1, 0.3, 3, 0.01), P.range('b', 1, 0.3, 3, 0.01), P.int('layers', 10, 1, 24), P.seg('style', ['filled', 'outline', 'both'], 'both'),
        P.angle('rotation', 270), P.range('lineWidth', 1.2, 0.3, 12, 0.1, { unit: 'px', showIf: v => v.style !== 'filled' }),
        P.color('lineColor', '#0b0d14', { showIf: v => v.style !== 'filled' }), P.color('background', '#0b0d14'), P.gradient('gradient', 'sunset')
    ], [
        ['flower', {}],
        ['star-burst', { m: 5, n1: 0.4, n2: 0.9, n3: 0.9, layers: 12, gradient: 'fire', style: 'both' }],
        ['soft-blob', { m: 3, n1: 3, n2: 5, n3: 5, layers: 8, gradient: 'ocean', style: 'filled' }],
        ['gear', { m: 12, n1: 2, n2: 8, n3: 8, layers: 6, gradient: 'copper', style: 'both' }]
    ], (p, fr, ctx) => {
        const SAMPLES = 2048, table = new Float64Array(SAMPLES + 1);
        const shape = phi => {
            const t = p.m * phi / 4;
            const v = Math.pow(Math.abs(Math.cos(t) / p.a), p.n2) + Math.pow(Math.abs(Math.sin(t) / p.b), p.n3);
            return v > 0 ? Math.pow(v, -1 / p.n1) : 0;
        };
        let maxR = 1e-9;
        for (let i = 0; i <= SAMPLES; i++) { table[i] = shape(TAU * i / SAMPLES); if (table[i] > maxR) maxR = table[i]; }
        const k = 0.44 / maxR, rot = rad(p.rotation);
        const lut = buildLUT(p.gradient), n1 = LUT_SIZE - 1, bg = hexRgb(p.background), ink = hexRgb(p.lineColor), tmp = [0, 0, 0, 255];
        const lineUnits = Math.max(p.lineWidth / 1024, 0.7 * fr.px) / 2;
        const N = p.layers, mx = fr.ax / 2, my = fr.ay / 2;
        return renderRGB(fr, ctx, (u, v, x, y, out) => {
            const dx = u - mx, dy = v - my;
            const rho = Math.hypot(dx, dy);
            let ph = (Math.atan2(dy, dx) - rot) / TAU;
            ph -= Math.floor(ph);
            const f = ph * SAMPLES, i0 = Math.floor(f), fi = f - i0;
            const radius = (table[i0] + (table[Math.min(SAMPLES, i0 + 1)] - table[i0]) * fi) * k;
            const s = radius > 0 ? rho / radius : 9;
            const edge = (1 - s) * radius;
            out[0] = bg[0]; out[1] = bg[1]; out[2] = bg[2];
            if (edge < -2 * fr.px - lineUnits) return;
            // Ring q of N nested copies; the colour runs from the middle (last gradient stop) to the rim.
            const q = Math.min(N - 1, Math.floor(Math.max(0, s) * N));
            putLut(tmp, 0, lut, n1, N > 1 ? 1 - q / (N - 1) : 0.5, 0);
            if (p.style !== 'outline') mixInto(out, bg, tmp, cover(edge, fr.px));
            if (p.style !== 'filled') {
                const ring = Math.abs(fract(s * N + 0.5) - 0.5) * radius / N;
                const line = Math.max(s <= 1 ? cover(lineUnits - ring, fr.px) : 0, cover(lineUnits - Math.abs(edge), fr.px));
                const base = [out[0], out[1], out[2]];
                mixInto(out, base, p.style === 'outline' ? tmp : ink, line);
            }
        });
    });

    define('phyllotaxis', 'math', 'Phyllotaxis', [
        P.int('count', 1800, 50, 12000, { step: 10 }), P.range('angle', 137.508, 60, 180, 0.001, { unit: '°' }), P.range('dotSize', 0.8, 0.1, 1.6, 0.01),
        P.seg('sizing', ['constant', 'grow', 'shrink'], 'grow'), P.range('cycles', 1, 0.25, 6, 0.05), P.seg('colorBy', ['index', 'radius'], 'index'),
        P.color('background', '#0b0d14'), P.gradient('gradient', 'sunset')
    ], [
        ['sunflower', { count: 1800, angle: 137.508, dotSize: 0.85, sizing: 'grow', gradient: [{ pos: 0, color: '#3a2000' }, { pos: 0.5, color: '#c98b0a' }, { pos: 1, color: '#ffe066' }], background: '#2a3a14' }],
        ['galaxy', { count: 4000, angle: 137.3, dotSize: 0.55, sizing: 'shrink', gradient: 'plasma', background: '#05060d' }],
        ['golden-spiral', { count: 900, angle: 137.508, dotSize: 1, sizing: 'constant', gradient: 'ocean', colorBy: 'radius' }],
        ['pinwheel', { count: 1500, angle: 99.5, dotSize: 0.7, sizing: 'constant', gradient: 'neon', cycles: 2 }]
    ], (p, fr, ctx) => {
        const img = newInk(fr, hexRgb(p.background)), lut = buildLUT(p.gradient), n1 = LUT_SIZE - 1, tmp = [0, 0, 0, 255];
        const N = p.count, fit = 0.46 * fr.short, c = fit / Math.sqrt(N), a = rad(p.angle);
        const spacing = c * 1.78, cx = fr.w / 2, cy = fr.h / 2;
        for (let i = 1; i <= N; i++) {
            const r = c * Math.sqrt(i), th = i * a, f = i / N;
            const grow = p.sizing === 'grow' ? 0.45 + 0.9 * f : (p.sizing === 'shrink' ? 1.35 - 0.9 * f : 1);
            putLut(tmp, 0, lut, n1, pingPong((p.colorBy === 'radius' ? Math.sqrt(f) : f) * p.cycles * 0.5), 0);
            strokeDisc(img, cx + r * Math.cos(th), cy + r * Math.sin(th), Math.max(0.5, spacing * 0.5 * p.dotSize * grow), tmp, 1);
            if ((i & 255) === 0) checkCancel(ctx);
        }
        return img;
    });

    define('moire', 'math', 'Moiré and interference', [
        P.seg('kind', ['lines', 'circles', 'waves'], 'lines'), P.range('frequency', 60, 6, 300, 1), P.angle('angle', 15), P.range('difference', 4, 0.2, 40, 0.1, { unit: '°', showIf: v => v.kind === 'lines' }),
        P.range('offset', 0.08, 0, 0.6, 0.005, { showIf: v => v.kind === 'circles' }), P.range('ratio', 0.5, 0.1, 0.9, 0.01, { showIf: v => v.kind !== 'waves' }),
        P.int('waves', 5, 2, 9, { showIf: v => v.kind === 'waves' }), P.seed(), P.bool('invert', false), P.gradient('gradient', 'grayscale')
    ], [
        ['line-moire', {}],
        ['ring-moire', { kind: 'circles', frequency: 48, offset: 0.1, gradient: 'ocean' }],
        ['interference', { kind: 'waves', frequency: 14, waves: 6, gradient: 'plasma' }],
        ['tight-fan', { kind: 'lines', frequency: 90, difference: 1.5, ratio: 0.4, gradient: 'sepia' }]
    ], (p, fr, ctx) => {
        const lut = buildLUT(p.gradient), mx = fr.ax / 2, my = fr.ay / 2, w = fr.px * p.frequency;
        const a1 = rad(p.angle), a2 = rad(p.angle + p.difference);
        const c1 = Math.cos(a1), s1 = Math.sin(a1), c2 = Math.cos(a2), s2 = Math.sin(a2);
        const rng = Lib.mulberry32(seedOf(p.seed, 300)), planes = [];
        for (let i = 0; i < p.waves; i++) planes.push({ a: rng() * TAU, f: TAU * p.frequency * (0.6 + rng() * 0.8), ph: rng() * TAU });
        return renderScalar(fr, ctx, lut, (u, v) => {
            const x = u - mx, y = v - my;
            let t;
            if (p.kind === 'lines') {
                const g1 = stripeCover(x * c1 * p.frequency + y * s1 * p.frequency, p.ratio, w);
                const g2 = stripeCover(x * c2 * p.frequency + y * s2 * p.frequency, p.ratio, w);
                t = (1 - g1) * (1 - g2);
            } else if (p.kind === 'circles') {
                const g1 = stripeCover(Math.hypot(x - p.offset / 2, y) * p.frequency, p.ratio, w);
                const g2 = stripeCover(Math.hypot(x + p.offset / 2, y) * p.frequency, p.ratio, w);
                t = (1 - g1) * (1 - g2);
            } else {
                let sum = 0;
                for (let i = 0; i < planes.length; i++) sum += Math.cos((x * Math.cos(planes[i].a) + y * Math.sin(planes[i].a)) * planes[i].f + planes[i].ph);
                t = 0.5 + 0.5 * sum / Math.sqrt(planes.length * 0.5) * 0.55;
            }
            return p.invert ? 1 - t : t;
        });
    });

    define('flow-field', 'math', 'Flow field', [
        P.seed(), P.int('lines', 2000, 100, 14000, { step: 100 }), P.int('steps', 110, 10, 400), P.range('stepSize', 0.0045, 0.001, 0.02, 0.0005),
        P.scale(55), P.int('octaves', 2, 1, 5), P.seg('field', ['curl', 'angle'], 'curl'), P.range('lineWidth', 1, 0.2, 6, 0.1, { unit: 'px' }),
        P.range('opacity', 0.3, 0.03, 1, 0.01), P.seg('colorBy', ['direction', 'along', 'line'], 'direction'), P.color('background', '#0a0e17'),
        P.gradient('gradient', 'rainbow')
    ], [
        ['silk', {}],
        ['ink-strands', { lines: 1400, steps: 160, opacity: 0.55, lineWidth: 1.3, colorBy: 'along', gradient: 'ice', background: '#05070d' }],
        ['turbulent-fire', { field: 'angle', lines: 3200, scale: 50, octaves: 4, colorBy: 'line', gradient: 'fire', background: '#0b0504' }]
    ], (p, fr, ctx) => {
        const bg = hexRgb(p.background), img = newInk(fr, bg), lut = buildLUT(p.gradient), n1 = LUT_SIZE - 1, tmp = [0, 0, 0, 255];
        const cells = 100 / p.scale;
        const field = makeFbm({ type: 'simplex', seed: p.seed, cells, octaves: p.octaves, lacunarity: 2, gain: 0.5 }, fr);
        const eps = 0.5 / (cells * 40), toPx = fr.w / fr.ax;
        const hw = Math.max(0.3, p.lineWidth * fr.short / 1024) / 2;
        const rng = Lib.mulberry32(seedOf(p.seed, 310));
        // Seeds on a jittered grid cover the frame evenly.
        const gx = Math.max(1, Math.round(Math.sqrt(p.lines * fr.ax / fr.ay))), gy = Math.max(1, Math.round(p.lines / gx));
        const angleAt = (u, v) => {
            if (p.field === 'angle') return (field.t(u, v) - 0.5) * TAU * 1.6;
            const dxn = (field.t(u + eps, v) - field.t(u - eps, v)), dyn = (field.t(u, v + eps) - field.t(u, v - eps));
            return Math.atan2(-dxn, dyn);
        };
        let drawn = 0;
        for (let j = 0; j < gy; j++) {
            for (let i = 0; i < gx; i++) {
                let u = (i + rng()) / gx * fr.ax, v = (j + rng()) / gy * fr.ay;
                const lineT = rng();
                let px = u * toPx, py = v * toPx, an = angleAt(u, v);
                for (let s = 0; s < p.steps; s++) {
                    const nu = u + Math.cos(an) * p.stepSize, nv = v + Math.sin(an) * p.stepSize;
                    if (nu < -0.05 || nv < -0.05 || nu > fr.ax + 0.05 || nv > fr.ay + 0.05) break;
                    const nan = angleAt(nu, nv);
                    const t = p.colorBy === 'direction' ? fract(an / TAU) : (p.colorBy === 'along' ? s / p.steps : lineT);
                    putLut(tmp, 0, lut, n1, t, 0);
                    const nx = nu * toPx, ny = nv * toPx;
                    strokeSegment(img, px, py, nx, ny, hw, tmp, p.opacity, true);
                    u = nu; v = nv; px = nx; py = ny; an = nan;
                }
                if ((++drawn & 63) === 0) { checkCancel(ctx); ctx.progress(drawn / (gx * gy)); }
            }
        }
        return img;
    });

    // Gray-Scott reaction-diffusion on a wrapping grid; results are cached so recolouring does not run the simulation again.
    const SIM_CACHE = [];

    function grayScott(gw, gh, feed, kill, iterations, seed, blobs, ctx) {
        const key = [gw, gh, feed, kill, iterations, seed, blobs].join('|');
        const hit = SIM_CACHE.find(entry => entry.key === key);
        if (hit) return hit.field;
        const n = gw * gh;
        let a = new Float32Array(n).fill(1), b = new Float32Array(n);
        let a2 = new Float32Array(n), b2 = new Float32Array(n);
        const rng = Lib.mulberry32(seedOf(seed, 400));
        for (let k = 0; k < blobs; k++) {
            const cx = rng() * gw, cy = rng() * gh, r = 2 + rng() * 4;
            for (let y = Math.floor(cy - r); y <= Math.ceil(cy + r); y++) {
                for (let x = Math.floor(cx - r); x <= Math.ceil(cx + r); x++) {
                    if ((x - cx) * (x - cx) + (y - cy) * (y - cy) > r * r) continue;
                    const i = wrapInt(y, gh) * gw + wrapInt(x, gw);
                    b[i] = 1; a[i] = 0.4;
                }
            }
        }
        const Da = 1, Db = 0.5, f = feed, kf = kill + feed;
        for (let it = 0; it < iterations; it++) {
            for (let y = 0; y < gh; y++) {
                const ym = ((y + gh - 1) % gh) * gw, y0 = y * gw, yp = ((y + 1) % gh) * gw;
                for (let x = 0; x < gw; x++) {
                    const xm = x === 0 ? gw - 1 : x - 1, xp = x === gw - 1 ? 0 : x + 1;
                    const i = y0 + x;
                    const av = a[i], bv = b[i];
                    const la = (a[y0 + xm] + a[y0 + xp] + a[ym + x] + a[yp + x]) * 0.2 + (a[ym + xm] + a[ym + xp] + a[yp + xm] + a[yp + xp]) * 0.05 - av;
                    const lb = (b[y0 + xm] + b[y0 + xp] + b[ym + x] + b[yp + x]) * 0.2 + (b[ym + xm] + b[ym + xp] + b[yp + xm] + b[yp + xp]) * 0.05 - bv;
                    const r = av * bv * bv;
                    a2[i] = av + Da * la - r + f * (1 - av);
                    b2[i] = bv + Db * lb + r - kf * bv;
                }
            }
            let t = a; a = a2; a2 = t; t = b; b = b2; b2 = t;
            if ((it & 63) === 63) { checkCancel(ctx); ctx.progress(0.9 * (it + 1) / iterations); }
        }
        let lo = 1e9, hi = -1e9;
        for (let i = 0; i < n; i++) { if (b[i] < lo) lo = b[i]; if (b[i] > hi) hi = b[i]; }
        const field = { gw, gh, b, lo, hi: hi > lo ? hi : lo + 1 };
        SIM_CACHE.unshift({ key, field });
        if (SIM_CACHE.length > 3) SIM_CACHE.pop();
        return field;
    }

    define('reaction-diffusion', 'math', 'Reaction–diffusion', [
        P.seed(), P.range('feed', 0.0545, 0.01, 0.1, 0.0005), P.range('kill', 0.062, 0.04, 0.075, 0.0005), P.int('iterations', 2600, 300, 12000, { step: 100 }),
        P.int('detail', 170, 80, 400, { step: 10 }), P.int('seeds', 16, 1, 60), P.range('contrast', 1, 0.3, 3, 0.05), P.bool('invert', false), P.gradient('gradient', 'ocean')
    ], [
        ['coral', { feed: 0.0545, kill: 0.062, gradient: 'ocean' }],
        ['mitosis', { feed: 0.0367, kill: 0.0649, iterations: 4000, seeds: 36, gradient: 'sunset' }],
        ['spots', { feed: 0.03, kill: 0.062, gradient: 'ice' }],
        ['labyrinth', { feed: 0.029, kill: 0.057, iterations: 3600, gradient: 'copper', invert: true }]
    ], (p, fr, ctx) => {
        const scale = fr.draft ? 0.45 : 1;
        const gw = Math.max(16, Math.round(p.detail * scale * fr.ax)), gh = Math.max(16, Math.round(p.detail * scale * fr.ay));
        const iterations = Math.round(p.iterations * (fr.draft ? 0.45 : 1));
        const sim = grayScott(gw, gh, p.feed, p.kill, iterations, p.seed, p.seeds, ctx);
        const lut = buildLUT(p.gradient), b = sim.b, k = 1 / (sim.hi - sim.lo);
        const sx = gw / fr.ax, sy = gh / fr.ay;
        return renderScalar(fr, ctx, lut, (u, v) => {
            const gx = u * sx - 0.5, gy = v * sy - 0.5;
            const x0 = Math.floor(gx), y0 = Math.floor(gy), tx = fade(gx - x0), ty = fade(gy - y0);
            const xa = wrapInt(x0, gw), xb = wrapInt(x0 + 1, gw), ya = wrapInt(y0, gh) * gw, yb = wrapInt(y0 + 1, gh) * gw;
            const top = b[ya + xa] + (b[ya + xb] - b[ya + xa]) * tx, bot = b[yb + xa] + (b[yb + xb] - b[yb + xa]) * tx;
            let t = ((top + (bot - top) * ty) - sim.lo) * k;
            t = (t - 0.5) * p.contrast + 0.5;
            return p.invert ? 1 - t : t;
        });
    });

    define('maze', 'math', 'Maze', [
        P.seed(), P.range('cells', 16, 4, 70, 0.5), P.seg('algorithm', ['winding', 'branching'], 'winding'), P.range('wall', 16, 4, 60, 0.5, { unit: '%' }),
        P.bool('solution', false), P.color('wallColor', '#1d2433'), P.color('floorColor', '#f4efe3'), P.color('pathColor', '#e63946', { showIf: v => v.solution })
    ], [
        ['classic', {}],
        ['fine-labyrinth', { cells: 40, wall: 22, algorithm: 'winding', wallColor: '#101010', floorColor: '#ffffff' }],
        ['solved', { cells: 14, solution: true, wallColor: '#243b53', floorColor: '#fdf6e3', pathColor: '#ef476f', algorithm: 'branching' }],
        ['night-maze', { cells: 22, wall: 12, wallColor: '#6ee7ff', floorColor: '#060a14' }]
    ], (p, fr, ctx) => {
        const cols = Math.max(2, Math.floor(p.cells * fr.ax)), rows = Math.max(2, Math.floor(p.cells * fr.ay));
        const cell = 1 / p.cells, ox = (fr.ax - cols * cell) / 2, oy = (fr.ay - rows * cell) / 2;
        const open = new Uint8Array(cols * rows);          // bit 1 N, 2 E, 4 S, 8 W: passages
        const rng = Lib.mulberry32(seedOf(p.seed, 500));
        const DX = [0, 1, 0, -1], DY = [-1, 0, 1, 0], BIT = [1, 2, 4, 8], OPP = [4, 8, 1, 2];
        const seen = new Uint8Array(cols * rows);
        const carve = (c, d) => {
            const x = c % cols, y = (c / cols) | 0, nx = x + DX[d], ny = y + DY[d];
            open[c] |= BIT[d]; open[ny * cols + nx] |= OPP[d];
            return ny * cols + nx;
        };
        if (p.algorithm === 'winding') {
            const stack = [0];
            seen[0] = 1;
            while (stack.length) {
                const c = stack[stack.length - 1], x = c % cols, y = (c / cols) | 0;
                const options = [];
                for (let d = 0; d < 4; d++) {
                    const nx = x + DX[d], ny = y + DY[d];
                    if (nx >= 0 && ny >= 0 && nx < cols && ny < rows && !seen[ny * cols + nx]) options.push(d);
                }
                if (!options.length) { stack.pop(); continue; }
                const nxt = carve(c, options[Math.floor(rng() * options.length)]);
                seen[nxt] = 1;
                stack.push(nxt);
            }
        } else {
            const frontier = [0];
            seen[0] = 1;
            while (frontier.length) {
                const at = Math.floor(rng() * frontier.length), c = frontier[at], x = c % cols, y = (c / cols) | 0;
                const options = [];
                for (let d = 0; d < 4; d++) {
                    const nx = x + DX[d], ny = y + DY[d];
                    if (nx >= 0 && ny >= 0 && nx < cols && ny < rows && !seen[ny * cols + nx]) options.push(d);
                }
                if (!options.length) { frontier[at] = frontier[frontier.length - 1]; frontier.pop(); continue; }
                const nxt = carve(c, options[Math.floor(rng() * options.length)]);
                seen[nxt] = 1;
                frontier.push(nxt);
            }
        }
        // Entrance at the top left, exit at the bottom right; the solution is found by breadth-first search.
        const last = cols * rows - 1, parent = new Int32Array(cols * rows).fill(-2), queue = [0];
        parent[0] = -1;
        for (let qi = 0; qi < queue.length; qi++) {
            const c = queue[qi], x = c % cols, y = (c / cols) | 0;
            for (let d = 0; d < 4; d++) {
                if (!(open[c] & BIT[d])) continue;
                const nc = (y + DY[d]) * cols + x + DX[d];
                if (parent[nc] === -2) { parent[nc] = c; queue.push(nc); }
            }
        }
        const route = [];
        for (let c = last; c >= 0; c = parent[c]) route.push(c);
        route.reverse();
        const onPath = new Int32Array(cols * rows).fill(-1);
        route.forEach((c, i) => { onPath[c] = i; });
        const wall = hexRgb(p.wallColor), floor = hexRgb(p.floorColor), pathRgb = hexRgb(p.pathColor);
        const t = p.wall / 100, wpx = fr.px / cell, pathHalf = 0.14;
        const segDist = (px, py, ax_, ay_, bx_, by_) => {
            const dx = bx_ - ax_, dy = by_ - ay_, l2 = dx * dx + dy * dy;
            const f = l2 > 0 ? clamp(((px - ax_) * dx + (py - ay_) * dy) / l2, 0, 1) : 0;
            return Math.hypot(px - ax_ - f * dx, py - ay_ - f * dy);
        };
        // Wall present on the vertical grid line i spanning row j (between cell i-1 and i), or on a horizontal line.
        const vWall = (i, j) => (j < 0 || j >= rows ? false : (i <= 0 || i >= cols ? true : !(open[j * cols + i] & 8)));
        const hWall = (i, j) => {
            if (i < 0 || i >= cols) return false;
            if (j <= 0) return !(i === 0 && j === 0);
            if (j >= rows) return !(i === cols - 1);
            return !(open[j * cols + i] & 1);
        };
        return renderRGB(fr, ctx, (u, v, x, y, out) => {
            const gx = (u - ox) / cell, gy = (v - oy) / cell;
            out[0] = floor[0]; out[1] = floor[1]; out[2] = floor[2];
            if (p.solution && gx >= 0 && gy >= 0 && gx < cols && gy < rows) {
                const ci = Math.floor(gx), cj = Math.floor(gy), c = cj * cols + ci, k = onPath[c];
                if (k >= 0) {
                    let d = 9;
                    const cx = ci + 0.5, cy = cj + 0.5;
                    if (k > 0) { const q = route[k - 1]; d = Math.min(d, segDist(gx, gy, cx, cy, (q % cols) + 0.5, ((q / cols) | 0) + 0.5)); }
                    if (k < route.length - 1) { const q = route[k + 1]; d = Math.min(d, segDist(gx, gy, cx, cy, (q % cols) + 0.5, ((q / cols) | 0) + 0.5)); }
                    if (route.length === 1) d = Math.hypot(gx - cx, gy - cy);
                    mixInto(out, floor, pathRgb, cover(pathHalf - d, wpx));
                }
            }
            const ri = Math.round(gx), rj = Math.round(gy), dxl = Math.abs(gx - ri), dyl = Math.abs(gy - rj);
            let c = 0;
            if (dxl < t / 2 + wpx && vWall(ri, Math.floor(gy))) c = cover(t / 2 - dxl, wpx);
            if (dyl < t / 2 + wpx && hWall(Math.floor(gx), rj)) c = Math.max(c, cover(t / 2 - dyl, wpx));
            if (dxl < t / 2 + wpx && dyl < t / 2 + wpx && ri >= 0 && rj >= 0 && ri <= cols && rj <= rows) c = Math.max(c, Math.min(cover(t / 2 - dxl, wpx), cover(t / 2 - dyl, wpx)));
            if (c > 0) mixInto(out, out, wall, c);
        });
    });


    Object.assign(Lib, {
        GENERATORS,
        GENERATOR_CATEGORIES: CATEGORIES,
        getGenerator: id => BY_ID[id] || null,
        generatorDefaults: defaultParams,
        normalizeGeneratorParams: (id, raw) => (BY_ID[id] ? normalizeParams(BY_ID[id], raw) : null),
        renderGenerator: render,
        genKit: { hash2, seedOf, makeBase, makeFbm, worleyNoise, buildLUT, CAL, noiseLayout, herringboneBrick }
    });
})(typeof self !== 'undefined' ? self : (typeof globalThis !== 'undefined' ? globalThis : this));
