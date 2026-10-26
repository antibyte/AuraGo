// Pixel pure image library, filters part (WP5). DOM-free; loads in a window, in the Pixel worker and in a node vm.
// Ops: fx.apply { image, id, params, scale } -> { image }; fx.render (apply + fade + selection mask), fx.compose, fx.histogram,
// fx.autoTone. Every filter is a pure function (image, params, ctx) -> image of the same size; ids live in Lib.fx.filters.
// ctx.scale is the size of the working image relative to the document (previews run on a proxy): pixel-sized parameters
// (radius, distance, cell size, ...) are multiplied by it so a preview looks like the full-size result.
// Images: { width, height, data: Uint8ClampedArray RGBA, straight alpha }.
(function (root) {
    'use strict';

    const Lib = root.AuraPixelLib = root.AuraPixelLib || {};
    const clamp = Lib.clamp, mulberry32 = Lib.mulberry32, hashSeed = Lib.hashSeed;
    const PI = Math.PI, TAU = PI * 2;

    // --- small helpers ---

    const num = (v, d) => { const n = Number(v); return Number.isFinite(n) ? n : d; };
    const c255 = v => (v <= 0 ? 0 : (v >= 255 ? 255 : v));
    const lerp = (a, b, t) => a + (b - a) * t;
    const smooth = (e0, e1, x) => { const t = clamp((x - e0) / (e1 - e0 || 1e-9), 0, 1); return t * t * (3 - 2 * t); };
    const lumaOf = (r, g, b) => 0.299 * r + 0.587 * g + 0.114 * b;
    const hyp = (a, b) => Math.sqrt(a * a + b * b);
    const cloneImage = img => ({ width: img.width, height: img.height, data: new Uint8ClampedArray(img.data) });
    const blankImage = (w, h) => ({ width: w, height: h, data: new Uint8ClampedArray(w * h * 4) });
    const hexRGB = (hex, fallback) => { const c = Lib.parseColor(hex); return c ? [c[0], c[1], c[2]] : (fallback || [0, 0, 0]); };

    function isOpaque(d) {
        for (let i = 3; i < d.length; i += 4) if (d[i] !== 255) return false;
        return true;
    }

    // Float32 luma plane of an image.
    function lumaPlane(img) {
        const d = img.data, n = img.width * img.height, out = new Float32Array(n);
        for (let i = 0, p = 0; p < n; i += 4, p++) out[p] = 0.299 * d[i] + 0.587 * d[i + 1] + 0.114 * d[i + 2];
        return out;
    }

    // Value 0..255 -> 256 entry table of a function on 0..1.
    function makeLut(fn) {
        const lut = new Uint8ClampedArray(256);
        for (let i = 0; i < 256; i++) lut[i] = fn(i / 255) * 255;
        return lut;
    }

    function applyLut(img, lutR, lutG, lutB) {
        const d = img.data;
        for (let i = 0; i < d.length; i += 4) { d[i] = lutR[d[i]]; d[i + 1] = lutG[d[i + 1]]; d[i + 2] = lutB[d[i + 2]]; }
        return img;
    }

    // Applies the per-pixel function f(r, g, b, out) -> writes out[0..2] (floats) ; alpha untouched.
    function mapRGB(img, f) {
        const d = img.data, out = [0, 0, 0];
        for (let i = 0; i < d.length; i += 4) {
            f(d[i], d[i + 1], d[i + 2], out);
            d[i] = out[0]; d[i + 1] = out[1]; d[i + 2] = out[2];
        }
        return img;
    }

    // --- blur machinery ---

    function boxSizesForGauss(sigma, passes) {
        const ideal = Math.sqrt(12 * sigma * sigma / passes + 1);
        let lower = Math.floor(ideal);
        if (lower % 2 === 0) lower--;
        if (lower < 1) lower = 1;
        const upper = lower + 2;
        const m = Math.round((12 * sigma * sigma - passes * lower * lower - 4 * passes * lower - 3 * passes) / (-4 * lower - 4));
        const sizes = [];
        for (let i = 0; i < passes; i++) sizes.push(i < m ? lower : upper);
        return sizes;
    }

    // One box pass over n floats with the window 2r+1 centred on each sample; the edge sample repeats outward.
    function boxPass(src, dst, n, r) {
        const inv = 1 / (2 * r + 1);
        let sum = src[0] * (r + 1);
        for (let k = 1; k <= r; k++) sum += src[k < n ? k : n - 1];
        for (let i = 0; i < n; i++) {
            dst[i] = sum * inv;
            const add = i + r + 1, sub = i - r;
            sum += src[add < n ? add : n - 1] - src[sub > 0 ? sub : 0];
        }
    }

    // Gaussian (three box passes) over a float line; the result is written back into `a`.
    function gaussLine(a, t, n, radii) {
        boxPass(a, t, n, radii[0]);
        boxPass(t, a, n, radii[1]);
        boxPass(a, t, n, radii[2]);
        for (let i = 0; i < n; i++) a[i] = t[i];
    }

    const radiiFor = sigma => boxSizesForGauss(sigma, 3).map(s => (s - 1) >> 1);

    // Gaussian blur of a Float32 plane (new array).
    function blurPlane(plane, w, h, sigma) {
        if (!(sigma > 0.25) || !w || !h) return Float32Array.from(plane);
        const radii = radiiFor(sigma);
        const out = Float32Array.from(plane);
        const line = new Float32Array(Math.max(w, h)), tmp = new Float32Array(Math.max(w, h));
        for (let y = 0; y < h; y++) {
            const o = y * w;
            for (let x = 0; x < w; x++) line[x] = out[o + x];
            gaussLine(line, tmp, w, radii);
            for (let x = 0; x < w; x++) out[o + x] = line[x];
        }
        for (let x = 0; x < w; x++) {
            for (let y = 0; y < h; y++) line[y] = out[y * w + x];
            gaussLine(line, tmp, h, radii);
            for (let y = 0; y < h; y++) out[y * w + x] = line[y];
        }
        return out;
    }

    // Gaussian blur of an RGBA image with the colour weighted by alpha (no dark halos at transparent edges).
    function gaussBlurImage(img, sx, sy) {
        const w = img.width, h = img.height;
        if (sy === undefined) sy = sx;
        const out = cloneImage(img);
        if (!w || !h || (!(sx > 0.25) && !(sy > 0.25))) return out;
        const opaque = isOpaque(img.data);
        const len = Math.max(w, h);
        const ch = [new Float32Array(len), new Float32Array(len), new Float32Array(len), new Float32Array(len)];
        const tmp = new Float32Array(len);
        const pass = (src, dst, n, count, base, step, radii) => {
            for (let line = 0; line < count; line++) {
                const o = base(line);
                for (let k = 0; k < n; k++) {
                    const i = o + k * step;
                    const a = src[i + 3];
                    if (opaque) { ch[0][k] = src[i]; ch[1][k] = src[i + 1]; ch[2][k] = src[i + 2]; } else { const f = a / 255; ch[0][k] = src[i] * f; ch[1][k] = src[i + 1] * f; ch[2][k] = src[i + 2] * f; ch[3][k] = a; }
                }
                const channels = opaque ? 3 : 4;
                for (let c = 0; c < channels; c++) gaussLine(ch[c], tmp, n, radii);
                for (let k = 0; k < n; k++) {
                    const i = o + k * step;
                    if (opaque) { dst[i] = ch[0][k]; dst[i + 1] = ch[1][k]; dst[i + 2] = ch[2][k]; dst[i + 3] = 255; } else {
                        const a = ch[3][k];
                        if (a > 0.01) { const f = 255 / a; dst[i] = ch[0][k] * f; dst[i + 1] = ch[1][k] * f; dst[i + 2] = ch[2][k] * f; } else { dst[i] = dst[i + 1] = dst[i + 2] = 0; }
                        dst[i + 3] = a;
                    }
                }
            }
        };
        let cur = img.data;
        if (sx > 0.25) { pass(cur, out.data, w, h, y => y * w * 4, 4, radiiFor(sx)); cur = out.data; }
        if (sy > 0.25) {
            const tmpData = new Uint8ClampedArray(cur);
            pass(tmpData, out.data, h, w, x => x * 4, w * 4, radiiFor(sy));
        }
        return out;
    }

    // Box blur of one Float32 plane with half-width r (new array).
    function boxBlurPlane(plane, w, h, r) {
        r = Math.round(r);
        if (r < 1) return Float32Array.from(plane);
        const out = Float32Array.from(plane);
        const line = new Float32Array(Math.max(w, h)), tmp = new Float32Array(Math.max(w, h));
        for (let y = 0; y < h; y++) {
            const o = y * w;
            for (let x = 0; x < w; x++) line[x] = out[o + x];
            boxPass(line, tmp, w, r);
            for (let x = 0; x < w; x++) out[o + x] = tmp[x];
        }
        for (let x = 0; x < w; x++) {
            for (let y = 0; y < h; y++) line[y] = out[y * w + x];
            boxPass(line, tmp, h, r);
            for (let y = 0; y < h; y++) out[y * w + x] = tmp[y];
        }
        return out;
    }

    // Box blur along a direction (angle in degrees, 0 = right, clockwise) over `length` pixels. The image is walked in
    // digital lines, so the cost does not depend on the length. gain > 0 weights bright samples (bokeh): the weight of a
    // sample is alpha * (1 + gain * smoothstep(thr, 255, max channel)).
    function dirBlur(src, w, h, angle, length, gain, thr) {
        const out = new Uint8ClampedArray(src.length);
        const rad = angle * PI / 180, cx = Math.cos(rad), sy = Math.sin(rad);
        const xMajor = Math.abs(cx) >= Math.abs(sy);
        const major = xMajor ? Math.abs(cx) : Math.abs(sy);
        const half = Math.round(length * major / 2);
        if (half < 1) { out.set(src); return out; }
        const n = xMajor ? w : h, across = xMajor ? h : w;
        const slope = xMajor ? sy / cx : cx / sy;
        const shift = new Int32Array(n);
        let minS = 0, maxS = 0;
        for (let k = 0; k < n; k++) { shift[k] = Math.round(slope * k); if (shift[k] < minS) minS = shift[k]; if (shift[k] > maxS) maxS = shift[k]; }
        const lr = new Float32Array(n), lg = new Float32Array(n), lb = new Float32Array(n), lw = new Float32Array(n), la = new Float32Array(n);
        const tmp = new Float32Array(n);
        const weighted = gain > 0;
        for (let c = -maxS; c <= across - 1 - minS; c++) {
            let any = false;
            for (let k = 0; k < n; k++) {
                const pos = c + shift[k];
                if (pos >= 0 && pos < across) any = true;
                const q = pos < 0 ? 0 : (pos >= across ? across - 1 : pos);
                const i = (xMajor ? q * w + k : k * w + q) * 4;
                const a = src[i + 3] / 255;
                let wt = a;
                if (weighted) { const m = Math.max(src[i], src[i + 1], src[i + 2]); wt = a * (1 + gain * smooth(thr, 255, m)); }
                lr[k] = src[i] * wt; lg[k] = src[i + 1] * wt; lb[k] = src[i + 2] * wt; lw[k] = wt; la[k] = a;
            }
            if (!any) continue;
            boxPass(lr, tmp, n, half); for (let k = 0; k < n; k++) lr[k] = tmp[k];
            boxPass(lg, tmp, n, half); for (let k = 0; k < n; k++) lg[k] = tmp[k];
            boxPass(lb, tmp, n, half); for (let k = 0; k < n; k++) lb[k] = tmp[k];
            boxPass(lw, tmp, n, half); for (let k = 0; k < n; k++) lw[k] = tmp[k];
            boxPass(la, tmp, n, half); for (let k = 0; k < n; k++) la[k] = tmp[k];
            for (let k = 0; k < n; k++) {
                const pos = c + shift[k];
                if (pos < 0 || pos >= across) continue;
                const i = (xMajor ? pos * w + k : k * w + pos) * 4;
                const wt = lw[k];
                if (wt > 1e-6) { const f = 1 / wt; out[i] = lr[k] * f; out[i + 1] = lg[k] * f; out[i + 2] = lb[k] * f; }
                out[i + 3] = la[k] * 255;
            }
        }
        return out;
    }

    // --- sampling ---

    // Bilinear sample of straight-alpha RGBA at float pixel coordinates (edges clamp); writes to out[o..o+3] (floats).
    function sampleRGBA(d, w, h, x, y, out, o) {
        if (x < 0) x = 0; else if (x > w - 1) x = w - 1;
        if (y < 0) y = 0; else if (y > h - 1) y = h - 1;
        const x0 = x | 0, y0 = y | 0;
        const x1 = x0 + 1 < w ? x0 + 1 : x0, y1 = y0 + 1 < h ? y0 + 1 : y0;
        const tx = x - x0, ty = y - y0;
        const i00 = (y0 * w + x0) * 4, i10 = (y0 * w + x1) * 4, i01 = (y1 * w + x0) * 4, i11 = (y1 * w + x1) * 4;
        const w00 = (1 - tx) * (1 - ty), w10 = tx * (1 - ty), w01 = (1 - tx) * ty, w11 = tx * ty;
        const a00 = d[i00 + 3] * w00, a10 = d[i10 + 3] * w10, a01 = d[i01 + 3] * w01, a11 = d[i11 + 3] * w11;
        const a = a00 + a10 + a01 + a11;
        if (a > 0) {
            const f = 1 / a;
            out[o] = (d[i00] * a00 + d[i10] * a10 + d[i01] * a01 + d[i11] * a11) * f;
            out[o + 1] = (d[i00 + 1] * a00 + d[i10 + 1] * a10 + d[i01 + 1] * a01 + d[i11 + 1] * a11) * f;
            out[o + 2] = (d[i00 + 2] * a00 + d[i10 + 2] * a10 + d[i01 + 2] * a01 + d[i11 + 2] * a11) * f;
        } else { out[o] = out[o + 1] = out[o + 2] = 0; }
        out[o + 3] = a;
    }

    // Backward mapping: map(x, y, out) writes the source coordinates for output pixel (x, y) into out[0], out[1].
    function warpImage(img, map) {
        const w = img.width, h = img.height, src = img.data;
        const out = blankImage(w, h), d = out.data;
        const pt = [0, 0], px = [0, 0, 0, 0];
        for (let y = 0; y < h; y++) {
            for (let x = 0; x < w; x++) {
                pt[0] = x; pt[1] = y;
                map(x, y, pt);
                sampleRGBA(src, w, h, pt[0], pt[1], px, 0);
                const i = (y * w + x) * 4;
                d[i] = px[0]; d[i + 1] = px[1]; d[i + 2] = px[2]; d[i + 3] = px[3];
            }
        }
        return out;
    }

    // --- value noise (smooth, seeded) ---

    function noiseTable(seed) {
        const rnd = mulberry32(seed >>> 0), perm = new Uint8Array(512), vals = new Float32Array(256);
        const base = new Uint8Array(256);
        for (let i = 0; i < 256; i++) { base[i] = i; vals[i] = rnd() * 2 - 1; }
        for (let i = 255; i > 0; i--) { const j = (rnd() * (i + 1)) | 0; const t = base[i]; base[i] = base[j]; base[j] = t; }
        for (let i = 0; i < 512; i++) perm[i] = base[i & 255];
        return { perm, vals };
    }

    // Smooth value noise in [-1, 1].
    function valueNoise(t, x, y) {
        const xi = Math.floor(x), yi = Math.floor(y);
        const fx = x - xi, fy = y - yi;
        const u = fx * fx * (3 - 2 * fx), v = fy * fy * (3 - 2 * fy);
        const X = xi & 255, Y = yi & 255;
        const p = t.perm, vl = t.vals;
        const a = vl[p[p[X] + Y]], b = vl[p[p[X + 1] + Y]], c = vl[p[p[X] + Y + 1]], d = vl[p[p[X + 1] + Y + 1]];
        return lerp(lerp(a, b, u), lerp(c, d, u), v);
    }

    // Two octaves of value noise.
    function fbm(t, x, y) { return valueNoise(t, x, y) * 0.65 + valueNoise(t, x * 2.03 + 17.1, y * 2.03 - 9.7) * 0.35; }

    // Hash of integer cell coordinates -> [0, 1).
    function cellHash(x, y, seed) {
        let h = Math.imul(x | 0, 0x27d4eb2d) ^ Math.imul(y | 0, 0x165667b1) ^ Math.imul((seed | 0) + 0x9e3779b9, 0x85ebca6b);
        h ^= h >>> 15; h = Math.imul(h, 0x2c1b3c6d); h ^= h >>> 12; h = Math.imul(h, 0x297a2d39); h ^= h >>> 15;
        return (h >>> 0) / 4294967296;
    }

    // Standard normal sample from a uniform generator.
    function gauss(rnd) {
        let u = 0;
        while (u === 0) u = rnd();
        return Math.sqrt(-2 * Math.log(u)) * Math.cos(TAU * rnd());
    }

    // Splits a long image into horizontal bands (with `overlap` rows of context on both sides) and runs fn(sub) on each,
    // so filters that need big float planes stay within a bounded amount of memory.
    function inBands(img, overlap, fn, maxPixels) {
        const w = img.width, h = img.height;
        const limit = maxPixels || 1500000;
        const rows = Math.max(overlap * 2 + 8, Math.floor(limit / Math.max(1, w)));
        if (h <= rows + overlap * 2) return fn(img);
        const out = blankImage(w, h);
        for (let y0 = 0; y0 < h; y0 += rows) {
            const y1 = Math.min(h, y0 + rows);
            const s0 = Math.max(0, y0 - overlap), s1 = Math.min(h, y1 + overlap);
            const sub = { width: w, height: s1 - s0, data: img.data.subarray(s0 * w * 4, s1 * w * 4) };
            const res = fn(sub);
            out.data.set(res.data.subarray((y0 - s0) * w * 4, (y1 - s0) * w * 4), y0 * w * 4);
        }
        return out;
    }

    // --- filter registry ---

    const FILTERS = Object.create(null);
    const cloneValue = v => (v && typeof v === 'object' ? JSON.parse(JSON.stringify(v)) : v);

    // def(id, defaults, fn): fn(img, p, c) works on a private copy of the image and returns an image of the same size.
    function def(id, defaults, fn) { FILTERS[id] = { id, defaults, fn }; }

    function defaultsFor(id) {
        const entry = FILTERS[id];
        if (!entry) return null;
        const out = {};
        Object.keys(entry.defaults).forEach(key => { out[key] = cloneValue(entry.defaults[key]); });
        return out;
    }

    function applyFilter(image, id, params, hooks) {
        const entry = FILTERS[id];
        if (!entry) throw new Error('fx: unknown filter ' + id);
        const h = hooks || {};
        const p = Object.assign(defaultsFor(id), params || {});
        const c = {
            scale: num(h.scale, 1) > 0 ? num(h.scale, 1) : 1,
            progress: typeof h.progress === 'function' ? h.progress : () => {},
            cancelled: typeof h.cancelled === 'function' ? h.cancelled : () => false
        };
        const img = cloneImage(image);
        const res = entry.fn(img, p, c);
        return res || img;
    }

    // Straight tone curve through control points [[x, y], ...] in 0..255 (monotone cubic, Fritsch-Carlson) -> 256 entry LUT.
    function splineLut(points) {
        const lut = new Uint8ClampedArray(256);
        const pts = (points || []).map(q => [clamp(num(q[0], 0), 0, 255), clamp(num(q[1], 0), 0, 255)]).sort((a, b) => a[0] - b[0]);
        const xs = [], ys = [];
        pts.forEach(q => {
            if (xs.length && q[0] - xs[xs.length - 1] < 1e-6) ys[ys.length - 1] = q[1];
            else { xs.push(q[0]); ys.push(q[1]); }
        });
        const n = xs.length;
        if (n < 2) { for (let i = 0; i < 256; i++) lut[i] = i; return lut; }
        const d = new Array(n - 1), m = new Array(n);
        for (let k = 0; k < n - 1; k++) d[k] = (ys[k + 1] - ys[k]) / (xs[k + 1] - xs[k]);
        m[0] = d[0]; m[n - 1] = d[n - 2];
        for (let k = 1; k < n - 1; k++) m[k] = d[k - 1] * d[k] <= 0 ? 0 : (d[k - 1] + d[k]) / 2;
        for (let k = 0; k < n - 1; k++) {
            if (d[k] === 0) { m[k] = m[k + 1] = 0; continue; }
            const a = m[k] / d[k], b = m[k + 1] / d[k], s = a * a + b * b;
            if (s > 9) { const t = 3 / Math.sqrt(s); m[k] = t * a * d[k]; m[k + 1] = t * b * d[k]; }
        }
        let k = 0;
        for (let x = 0; x < 256; x++) {
            if (x <= xs[0]) { lut[x] = ys[0]; continue; }
            if (x >= xs[n - 1]) { lut[x] = ys[n - 1]; continue; }
            while (k < n - 2 && x >= xs[k + 1]) k++;
            const hh = xs[k + 1] - xs[k], t = (x - xs[k]) / hh, t2 = t * t, t3 = t2 * t;
            lut[x] = (2 * t3 - 3 * t2 + 1) * ys[k] + (t3 - 2 * t2 + t) * hh * m[k] + (-2 * t3 + 3 * t2) * ys[k + 1] + (t3 - t2) * hh * m[k + 1];
        }
        return lut;
    }

    const identityLut = () => { const l = new Uint8ClampedArray(256); for (let i = 0; i < 256; i++) l[i] = i; return l; };
    const composeLut = (first, second) => { const l = new Uint8ClampedArray(256); for (let i = 0; i < 256; i++) l[i] = second[first[i]]; return l; };
    const contrastLut = (factor, brightness) => makeLut(t => (t - 0.5) * factor + 0.5 + (brightness || 0) / 255);

    function lumaHistogram(img) {
        const d = img.data, hist = new Float64Array(256);
        let count = 0;
        for (let i = 0; i < d.length; i += 4) {
            if (d[i + 3] === 0) continue;
            hist[Math.round(lumaOf(d[i], d[i + 1], d[i + 2]))]++;
            count++;
        }
        return { hist, count };
    }

    function channelHistogram(img, c) {
        const d = img.data, hist = new Float64Array(256);
        let count = 0;
        for (let i = 0; i < d.length; i += 4) {
            if (d[i + 3] === 0) continue;
            hist[d[i + c]]++;
            count++;
        }
        return { hist, count };
    }

    // Lowest and highest value that remain after dropping `clip` percent of the pixels at each end.
    function percentileRange(h, clip) {
        const limit = h.count * clip / 100;
        let lo = 0, hi = 255, acc = 0;
        for (let i = 0; i < 256; i++) { acc += h.hist[i]; if (acc > limit) { lo = i; break; } }
        acc = 0;
        for (let i = 255; i >= 0; i--) { acc += h.hist[i]; if (acc > limit) { hi = i; break; } }
        return { lo, hi: Math.max(hi, lo + 1) };
    }

    const stretchLut = (lo, hi) => makeLut(t => (t * 255 - lo) / (hi - lo));

    // --- adjust: levels, curves, colour ---

    def('levels', { channel: 'rgb', in_black: 0, gamma: 1, in_white: 255, out_black: 0, out_white: 255 }, (img, p) => {
        const ib = clamp(num(p.in_black, 0), 0, 254), iw = clamp(num(p.in_white, 255), ib + 1, 255);
        const g = clamp(num(p.gamma, 1), 0.05, 10), ob = clamp(num(p.out_black, 0), 0, 255), ow = clamp(num(p.out_white, 255), 0, 255);
        const lut = new Uint8ClampedArray(256);
        for (let i = 0; i < 256; i++) lut[i] = ob + (ow - ob) * Math.pow(clamp((i - ib) / (iw - ib), 0, 1), 1 / g);
        const id = identityLut(), ch = p.channel;
        return applyLut(img, ch === 'rgb' || ch === 'r' ? lut : id, ch === 'rgb' || ch === 'g' ? lut : id, ch === 'rgb' || ch === 'b' ? lut : id);
    });

    const IDENTITY_CURVE = [[0, 0], [255, 255]];
    def('curves', { curves: { rgb: IDENTITY_CURVE, r: IDENTITY_CURVE, g: IDENTITY_CURVE, b: IDENTITY_CURVE } }, (img, p) => {
        const cv = p.curves || {};
        const master = splineLut(cv.rgb || IDENTITY_CURVE);
        return applyLut(img, composeLut(master, splineLut(cv.r || IDENTITY_CURVE)), composeLut(master, splineLut(cv.g || IDENTITY_CURVE)), composeLut(master, splineLut(cv.b || IDENTITY_CURVE)));
    });

    def('hue-saturation', { hue: 0, saturation: 0, lightness: 0, colorize: false, tone_hue: 30, tone_sat: 40 }, (img, p) => {
        const hs = num(p.hue, 0), sat = num(p.saturation, 0) / 100, li = num(p.lightness, 0) / 100, col = !!p.colorize;
        const th = num(p.tone_hue, 30), ts = clamp(num(p.tone_sat, 40) / 100, 0, 1);
        const hsl = [0, 0, 0], rgb = [0, 0, 0], d = img.data;
        for (let i = 0; i < d.length; i += 4) {
            Lib.rgbToHsl(d[i], d[i + 1], d[i + 2], hsl);
            let h = hsl[0], s = hsl[1], l = hsl[2];
            if (col) { h = th; s = ts; } else { h += hs; s *= 1 + sat; }
            l = li >= 0 ? l + (1 - l) * li : l + l * li;
            Lib.hslToRgb(h, clamp(s, 0, 1), clamp(l, 0, 1), rgb);
            d[i] = rgb[0]; d[i + 1] = rgb[1]; d[i + 2] = rgb[2];
        }
        return img;
    });

    def('color-balance', { shadows_cr: 0, shadows_mg: 0, shadows_yb: 0, mid_cr: 0, mid_mg: 0, mid_yb: 0, high_cr: 0, high_mg: 0, high_yb: 0, preserve_luma: true }, (img, p) => {
        const ws = new Float32Array(256), wm = new Float32Array(256), wh = new Float32Array(256);
        for (let i = 0; i < 256; i++) {
            const l = i / 255;
            ws[i] = clamp((l - 0.333) / -0.25 + 0.5, 0, 1) * 0.7;
            wm[i] = clamp((l - 0.333) / 0.25 + 0.5, 0, 1) * clamp((l + 0.333 - 1) / -0.25 + 0.5, 0, 1) * 0.7;
            wh[i] = clamp((l + 0.333 - 1) / 0.25 + 0.5, 0, 1) * 0.7;
        }
        const k = 0.9;
        const s = [num(p.shadows_cr, 0) * k, num(p.shadows_mg, 0) * k, num(p.shadows_yb, 0) * k];
        const m = [num(p.mid_cr, 0) * k, num(p.mid_mg, 0) * k, num(p.mid_yb, 0) * k];
        const h = [num(p.high_cr, 0) * k, num(p.high_mg, 0) * k, num(p.high_yb, 0) * k];
        const keep = p.preserve_luma !== false, d = img.data;
        for (let i = 0; i < d.length; i += 4) {
            const r = d[i], g = d[i + 1], b = d[i + 2];
            const l0 = lumaOf(r, g, b), li = Math.round(l0);
            const cr = s[0] * ws[li] + m[0] * wm[li] + h[0] * wh[li];
            const mg = s[1] * ws[li] + m[1] * wm[li] + h[1] * wh[li];
            const yb = s[2] * ws[li] + m[2] * wm[li] + h[2] * wh[li];
            let nr = r + cr - (mg + yb) * 0.3, ng = g + mg - (cr + yb) * 0.3, nb = b + yb - (cr + mg) * 0.3;
            if (keep) { const shift = l0 - lumaOf(nr, ng, nb); nr += shift; ng += shift; nb += shift; }
            d[i] = nr; d[i + 1] = ng; d[i + 2] = nb;
        }
        return img;
    });

    // Shared by vibrance and the adjust panel: boosts low-saturation colours more, spares skin tones.
    function vibranceRGB(r, g, b, amount, sat, out) {
        const mx = Math.max(r, g, b), mn = Math.min(r, g, b);
        if (mx <= 0) { out[0] = out[1] = out[2] = 0; return; }
        const s = (mx - mn) / mx;
        let f = 1 + sat;
        if (amount > 0) {
            // hue near orange/red (skin) gets less
            let skin = 1;
            if (mx !== mn) {
                let h;
                if (mx === r) h = ((g - b) / (mx - mn) + (g < b ? 6 : 0)) * 60; else if (mx === g) h = ((b - r) / (mx - mn) + 2) * 60; else h = ((r - g) / (mx - mn) + 4) * 60;
                if (h < 50 || h > 345) skin = 0.6;
            }
            const q = 1 - s;
            f *= 1 + amount * q * q * skin;
        } else if (amount < 0) {
            f *= 1 + amount * s;
        }
        if (f < 0) f = 0;
        const y = lumaOf(r, g, b);
        out[0] = y + (r - y) * f; out[1] = y + (g - y) * f; out[2] = y + (b - y) * f;
        // keep the colour inside the gamut by pulling it back towards grey
        const hi = Math.max(out[0], out[1], out[2]), lo = Math.min(out[0], out[1], out[2]);
        let t = 1;
        if (hi > 255 && hi - y > 0) t = Math.min(t, (255 - y) / (hi - y));
        if (lo < 0 && y - lo > 0) t = Math.min(t, y / (y - lo));
        if (t < 1) { out[0] = y + (out[0] - y) * t; out[1] = y + (out[1] - y) * t; out[2] = y + (out[2] - y) * t; }
    }

    def('vibrance', { vibrance: 50, saturation: 0 }, (img, p) => {
        const v = clamp(num(p.vibrance, 0) / 100, -1, 1), sat = num(p.saturation, 0) / 100, out = [0, 0, 0], d = img.data;
        for (let i = 0; i < d.length; i += 4) {
            vibranceRGB(d[i], d[i + 1], d[i + 2], v, sat, out);
            d[i] = out[0]; d[i + 1] = out[1]; d[i + 2] = out[2];
        }
        return img;
    });

    def('channel-mixer', { mix_rr: 100, mix_rg: 0, mix_rb: 0, mix_gr: 0, mix_gg: 100, mix_gb: 0, mix_br: 0, mix_bg: 0, mix_bb: 100, mono: false, mono_r: 30, mono_g: 59, mono_b: 11 }, (img, p) => {
        const d = img.data;
        if (p.mono) {
            const a = num(p.mono_r, 30) / 100, b = num(p.mono_g, 59) / 100, c = num(p.mono_b, 11) / 100;
            for (let i = 0; i < d.length; i += 4) { const v = d[i] * a + d[i + 1] * b + d[i + 2] * c; d[i] = d[i + 1] = d[i + 2] = v; }
            return img;
        }
        const m = ['rr', 'rg', 'rb', 'gr', 'gg', 'gb', 'br', 'bg', 'bb'].map(k => num(p['mix_' + k], 0) / 100);
        for (let i = 0; i < d.length; i += 4) {
            const r = d[i], g = d[i + 1], b = d[i + 2];
            d[i] = r * m[0] + g * m[1] + b * m[2];
            d[i + 1] = r * m[3] + g * m[4] + b * m[5];
            d[i + 2] = r * m[6] + g * m[7] + b * m[8];
        }
        return img;
    });

    def('black-white', { reds: 40, yellows: 60, greens: 40, cyans: 60, blues: 20, magentas: 80, tint: false, tint_color: '#d9b27c', tint_amount: 50 }, (img, p) => {
        const W = [p.reds, p.yellows, p.greens, p.cyans, p.blues, p.magentas].map((v, i) => num(v, [40, 60, 40, 60, 20, 80][i]) / 100);
        const tint = !!p.tint, tc = hexRGB(p.tint_color, [217, 178, 124]), tsat = Lib.rgbToHsl(tc[0], tc[1], tc[2]);
        const amount = clamp(num(p.tint_amount, 50) / 100, 0, 1), rgb = [0, 0, 0], d = img.data;
        for (let i = 0; i < d.length; i += 4) {
            const r = d[i], g = d[i + 1], b = d[i + 2];
            const mx = Math.max(r, g, b), mn = Math.min(r, g, b);
            let gray = mn;
            if (mx > mn) {
                let h;
                if (mx === r) h = ((g - b) / (mx - mn) + (g < b ? 6 : 0)); else if (mx === g) h = (b - r) / (mx - mn) + 2; else h = (r - g) / (mx - mn) + 4;
                const k = Math.floor(h) % 6, f = h - Math.floor(h);
                gray = mn + (mx - mn) * lerp(W[k], W[(k + 1) % 6], f);
            }
            gray = c255(gray);
            if (tint) { Lib.hslToRgb(tsat[0], tsat[1] * amount, gray / 255, rgb); d[i] = rgb[0]; d[i + 1] = rgb[1]; d[i + 2] = rgb[2]; } else { d[i] = d[i + 1] = d[i + 2] = gray; }
        }
        return img;
    });

    def('photo-filter', { color: '#ec8a00', density: 25, preserve_luma: true }, (img, p) => {
        const col = hexRGB(p.color, [236, 138, 0]), dens = clamp(num(p.density, 25) / 100, 0, 1), keep = p.preserve_luma !== false;
        const norm = Math.max(0.2, lumaOf(col[0], col[1], col[2]) / 255), d = img.data;
        const mr = col[0] / 255 / norm, mg = col[1] / 255 / norm, mb = col[2] / 255 / norm;
        for (let i = 0; i < d.length; i += 4) {
            const r = d[i], g = d[i + 1], b = d[i + 2];
            let nr = lerp(r, c255(r * mr), dens), ng = lerp(g, c255(g * mg), dens), nb = lerp(b, c255(b * mb), dens);
            if (keep) { const l1 = lumaOf(nr, ng, nb); if (l1 > 1) { const f = lumaOf(r, g, b) / l1; nr *= f; ng *= f; nb *= f; } }
            d[i] = nr; d[i + 1] = ng; d[i + 2] = nb;
        }
        return img;
    });

    const defaultStops = id => Lib.cloneStops((Lib.gradientPreset(id) || Lib.gradientPreset('grayscale')).stops);
    def('gradient-map', { gradient: defaultStops('sunset'), reverse: false }, (img, p) => {
        const lut = Lib.gradientLUT(p.gradient, 256), rev = !!p.reverse, d = img.data;
        for (let i = 0; i < d.length; i += 4) {
            let y = Math.round(lumaOf(d[i], d[i + 1], d[i + 2]));
            if (rev) y = 255 - y;
            d[i] = lut[y * 4]; d[i + 1] = lut[y * 4 + 1]; d[i + 2] = lut[y * 4 + 2];
        }
        return img;
    });

    def('threshold', { level: 128 }, (img, p) => {
        const t = num(p.level, 128), d = img.data;
        for (let i = 0; i < d.length; i += 4) { const v = lumaOf(d[i], d[i + 1], d[i + 2]) >= t ? 255 : 0; d[i] = d[i + 1] = d[i + 2] = v; }
        return img;
    });

    def('posterize', { levels: 5 }, (img, p) => {
        const levels = clamp(Math.round(num(p.levels, 5)), 2, 64), step = 255 / (levels - 1);
        const lut = new Uint8ClampedArray(256);
        for (let i = 0; i < 256; i++) lut[i] = Math.round(i / step) * step;
        return applyLut(img, lut, lut, lut);
    });

    def('invert', {}, img => {
        const d = img.data;
        for (let i = 0; i < d.length; i += 4) { d[i] = 255 - d[i]; d[i + 1] = 255 - d[i + 1]; d[i + 2] = 255 - d[i + 2]; }
        return img;
    });

    def('equalize', { amount: 100 }, (img, p) => {
        const a = clamp(num(p.amount, 100) / 100, 0, 1), h = lumaHistogram(img), cdf = new Float64Array(256);
        let acc = 0;
        for (let i = 0; i < 256; i++) { acc += h.hist[i]; cdf[i] = h.count ? acc / h.count : i / 255; }
        const d = img.data;
        for (let i = 0; i < d.length; i += 4) {
            const y = lumaOf(d[i], d[i + 1], d[i + 2]);
            if (y < 0.5) continue;
            const target = lerp(y, cdf[Math.round(y)] * 255, a), f = target / y;
            d[i] = d[i] * f; d[i + 1] = d[i + 1] * f; d[i + 2] = d[i + 2] * f;
        }
        return img;
    });

    def('auto-levels', { clip: 0.5 }, (img, p) => {
        const clip = clamp(num(p.clip, 0.5), 0, 10), luts = [0, 1, 2].map(c => { const r = percentileRange(channelHistogram(img, c), clip); return stretchLut(r.lo, r.hi); });
        return applyLut(img, luts[0], luts[1], luts[2]);
    });

    def('auto-contrast', { clip: 0.5 }, (img, p) => {
        const r = percentileRange(lumaHistogram(img), clamp(num(p.clip, 0.5), 0, 10)), lut = stretchLut(r.lo, r.hi);
        return applyLut(img, lut, lut, lut);
    });

    // Gain that turns the average colour of the mid-tones neutral (gray world).
    function grayWorldGains(img) {
        const d = img.data;
        let sr = 0, sg = 0, sb = 0, n = 0;
        for (let i = 0; i < d.length; i += 4) {
            if (d[i + 3] < 128) continue;
            const y = lumaOf(d[i], d[i + 1], d[i + 2]);
            if (y < 25 || y > 230) continue;
            sr += d[i]; sg += d[i + 1]; sb += d[i + 2]; n++;
        }
        if (!n || sr < 1 || sg < 1 || sb < 1) return [1, 1, 1];
        const avg = (sr + sg + sb) / 3;
        return [clamp(avg / sr, 0.5, 2), clamp(avg / sg, 0.5, 2), clamp(avg / sb, 0.5, 2)];
    }

    def('auto-white-balance', { strength: 100 }, (img, p) => {
        const s = clamp(num(p.strength, 100) / 100, 0, 1), g = grayWorldGains(img).map(v => lerp(1, v, s)), d = img.data;
        for (let i = 0; i < d.length; i += 4) { d[i] = d[i] * g[0]; d[i + 1] = d[i + 1] * g[1]; d[i + 2] = d[i + 2] * g[2]; }
        return img;
    });

    // --- adjust: classic one-click looks (old ids) ---

    def('grayscale', { method: 'luma' }, (img, p) => {
        const m = p.method, d = img.data;
        for (let i = 0; i < d.length; i += 4) {
            const r = d[i], g = d[i + 1], b = d[i + 2];
            const v = m === 'average' ? (r + g + b) / 3 : (m === 'lightness' ? (Math.max(r, g, b) + Math.min(r, g, b)) / 2 : lumaOf(r, g, b));
            d[i] = d[i + 1] = d[i + 2] = v;
        }
        return img;
    });

    def('monochrome', { contrast: 15 }, (img, p) => {
        const f = 1 + num(p.contrast, 15) / 100, d = img.data;
        for (let i = 0; i < d.length; i += 4) { const v = (lumaOf(d[i], d[i + 1], d[i + 2]) - 127.5) * f + 127.5; d[i] = d[i + 1] = d[i + 2] = v; }
        return img;
    });

    // Temperature shift + saturation + brightness in one pass (warm / cool / tint share it).
    function toneShift(img, tr, tg, tb, sat, bright) {
        const d = img.data;
        for (let i = 0; i < d.length; i += 4) {
            let r = d[i] * tr, g = d[i + 1] * tg, b = d[i + 2] * tb;
            const y = lumaOf(r, g, b);
            r = (y + (r - y) * sat) * bright; g = (y + (g - y) * sat) * bright; b = (y + (b - y) * sat) * bright;
            d[i] = r; d[i + 1] = g; d[i + 2] = b;
        }
        return img;
    }

    def('warm', { warmth: 50, saturation: 20 }, (img, p) => {
        const t = clamp(num(p.warmth, 50) / 100, 0, 1);
        return toneShift(img, 1 + 0.18 * t, 1 + 0.03 * t, 1 - 0.18 * t, 1 + num(p.saturation, 20) / 100, 1 + 0.05 * t);
    });

    def('cool', { coolness: 50, saturation: -10 }, (img, p) => {
        const t = clamp(num(p.coolness, 50) / 100, 0, 1);
        return toneShift(img, 1 - 0.18 * t, 1 - 0.01 * t, 1 + 0.18 * t, 1 + num(p.saturation, -10) / 100, 1);
    });

    def('tint', { hue: 90, saturation: 15 }, (img, p) => {
        const hs = num(p.hue, 90), sat = 1 + num(p.saturation, 15) / 100, hsl = [0, 0, 0], rgb = [0, 0, 0], d = img.data;
        for (let i = 0; i < d.length; i += 4) {
            Lib.rgbToHsl(d[i], d[i + 1], d[i + 2], hsl);
            Lib.hslToRgb(hsl[0] + hs, clamp(hsl[1] * sat, 0, 1), hsl[2], rgb);
            d[i] = rgb[0]; d[i + 1] = rgb[1]; d[i + 2] = rgb[2];
        }
        return img;
    });

    def('high-contrast', { contrast: 50, brightness: 0 }, (img, p) => {
        const lut = contrastLut(1 + num(p.contrast, 50) / 100, num(p.brightness, 0) * 1.28);
        return applyLut(img, lut, lut, lut);
    });

    // --- blur ---

    def('blur', { radius: 3 }, (img, p, c) => gaussBlurImage(img, num(p.radius, 3) * c.scale));

    def('box-blur', { radius: 5 }, (img, p, c) => {
        const len = Math.round(num(p.radius, 5) * c.scale) * 2;
        const h1 = dirBlur(img.data, img.width, img.height, 0, len, 0, 0);
        img.data.set(dirBlur(h1, img.width, img.height, 90, len, 0, 0));
        return img;
    });

    def('motion-blur', { angle: 0, distance: 24 }, (img, p, c) => {
        img.data.set(dirBlur(img.data, img.width, img.height, num(p.angle, 0), num(p.distance, 24) * c.scale, 0, 0));
        return img;
    });

    // Averages samples on an arc (spin) or along the ray to the centre (zoom); the sample count follows the distance.
    function radialBlur(img, cx, cy, amount, mode, maxSamples) {
        const w = img.width, h = img.height, src = img.data, out = new Uint8ClampedArray(src.length);
        const spin = mode === 'spin', span = spin ? amount * PI / 180 : amount;
        for (let y = 0; y < h; y++) {
            for (let x = 0; x < w; x++) {
                const dx = x - cx, dy = y - cy, n = clamp(Math.ceil(hyp(dx, dy) * span / 1.2), 1, maxSamples), o = (y * w + x) * 4;
                if (n === 1) { out[o] = src[o]; out[o + 1] = src[o + 1]; out[o + 2] = src[o + 2]; out[o + 3] = src[o + 3]; continue; }
                let sr = 0, sg = 0, sb = 0, sa = 0;
                // spin: rotate the offset step by step (one sin/cos per pixel); zoom: the offset shrinks linearly
                let vx = dx, vy = dy, rc = 1, rs = 0, f = 1, df = 0;
                if (spin) {
                    const a0 = -span / 2, step = span / (n - 1), c0 = Math.cos(a0), s0 = Math.sin(a0);
                    vx = dx * c0 - dy * s0; vy = dx * s0 + dy * c0;
                    rc = Math.cos(step); rs = Math.sin(step);
                } else { f = 1 - span; df = span / (n - 1); }
                for (let k = 0; k < n; k++) {
                    const sx = spin ? cx + vx : cx + dx * f, sy = spin ? cy + vy : cy + dy * f;
                    const px = sx < 0 ? 0 : (sx > w - 1 ? w - 1 : (sx + 0.5) | 0), py = sy < 0 ? 0 : (sy > h - 1 ? h - 1 : (sy + 0.5) | 0);
                    const j = (py * w + px) * 4, a = src[j + 3];
                    sr += src[j] * a; sg += src[j + 1] * a; sb += src[j + 2] * a; sa += a;
                    if (spin) { const nx = vx * rc - vy * rs; vy = vx * rs + vy * rc; vx = nx; } else f += df;
                }
                if (sa > 0) { out[o] = sr / sa; out[o + 1] = sg / sa; out[o + 2] = sb / sa; }
                out[o + 3] = sa / n;
            }
        }
        img.data.set(out);
        return img;
    }

    const pointOf = (v, dx, dy) => (v && typeof v === 'object' ? { x: num(v.x, dx), y: num(v.y, dy) } : { x: dx, y: dy });

    def('radial-blur', { amount: 12, center: { x: 0.5, y: 0.5 } }, (img, p) => {
        const c = pointOf(p.center, 0.5, 0.5), big = img.width * img.height > 16e6;
        return radialBlur(img, c.x * (img.width - 1), c.y * (img.height - 1), clamp(num(p.amount, 12), 0, 180), 'spin', big ? 12 : 24);
    });

    def('zoom-blur', { amount: 30, center: { x: 0.5, y: 0.5 } }, (img, p) => {
        const c = pointOf(p.center, 0.5, 0.5), big = img.width * img.height > 16e6;
        return radialBlur(img, c.x * (img.width - 1), c.y * (img.height - 1), clamp(num(p.amount, 30), 0, 100) / 100 * 0.5, 'zoom', big ? 12 : 24);
    });

    // Disc-like bokeh from three directional box blurs 60 degrees apart; bright samples weigh more.
    def('lens-blur', { radius: 8, brightness: 50, threshold: 190 }, (img, p, c) => {
        const r = num(p.radius, 8) * c.scale;
        if (r < 1) return img;
        const gain = clamp(num(p.brightness, 50), 0, 100) / 100 * 6, thr = clamp(num(p.threshold, 190), 0, 254), w = img.width, h = img.height;
        let cur = img.data;
        [0, 60, 120].forEach(angle => { cur = dirBlur(cur, w, h, angle, r, gain, thr); });
        img.data.set(cur);
        return img;
    });

    // Separable bilateral blur: spatial triangle weights times a colour-distance falloff. threshold 0..100.
    function surfaceBlur(img, radius, threshold) {
        const w = img.width, h = img.height, src = img.data;
        const r = clamp(Math.round(radius), 1, 30), step = Math.max(1, Math.ceil(r / 8));
        const sigma = Math.max(2, threshold * 1.2);
        const range = new Float32Array(766);
        for (let i = 0; i < 766; i++) { const v = Math.exp(-(i * i) / (2 * sigma * sigma)); range[i] = v < 1e-4 ? 0 : v; }
        const spatial = new Float32Array(r + 1);
        for (let k = 0; k <= r; k++) spatial[k] = 1 - k / (r + 1);
        let a = src, out = new Uint8ClampedArray(src.length);
        for (let pass = 0; pass < 2; pass++) {
            const horizontal = pass === 0;
            for (let y = 0; y < h; y++) {
                for (let x = 0; x < w; x++) {
                    const i = (y * w + x) * 4;
                    let sr = 0, sg = 0, sb = 0, sw = 0;
                    const cr = a[i], cg = a[i + 1], cb = a[i + 2];
                    for (let k = -r; k <= r; k += step) {
                        let xx = x, yy = y;
                        if (horizontal) { xx = x + k; if (xx < 0) xx = 0; else if (xx >= w) xx = w - 1; } else { yy = y + k; if (yy < 0) yy = 0; else if (yy >= h) yy = h - 1; }
                        const j = (yy * w + xx) * 4;
                        const diff = Math.abs(a[j] - cr) + Math.abs(a[j + 1] - cg) + Math.abs(a[j + 2] - cb);
                        const wt = spatial[k < 0 ? -k : k] * range[diff] * (a[j + 3] / 255);
                        sr += a[j] * wt; sg += a[j + 1] * wt; sb += a[j + 2] * wt; sw += wt;
                    }
                    if (sw > 1e-6) { out[i] = sr / sw; out[i + 1] = sg / sw; out[i + 2] = sb / sw; } else { out[i] = cr; out[i + 1] = cg; out[i + 2] = cb; }
                    out[i + 3] = a[i + 3];
                }
            }
            if (horizontal) { a = out; out = new Uint8ClampedArray(src.length); }
        }
        img.data.set(out);
        return img;
    }

    def('surface-blur', { radius: 5, threshold: 20 }, (img, p, c) => surfaceBlur(img, num(p.radius, 5) * c.scale, clamp(num(p.threshold, 20), 1, 100)));

    // Median per colour channel with a sliding histogram (Huang): O(n * radius).
    function medianImage(img, radius) {
        const r = clamp(Math.round(radius), 1, 40), w = img.width, h = img.height, src = img.data, out = new Uint8ClampedArray(src.length);
        const total = (2 * r + 1) * (2 * r + 1), half = (total - 1) >> 1;
        const hist = new Int32Array(256);
        const cx = x => (x < 0 ? 0 : (x >= w ? w - 1 : x)), cy = y => (y < 0 ? 0 : (y >= h ? h - 1 : y));
        for (let c = 0; c < 3; c++) {
            for (let y = 0; y < h; y++) {
                hist.fill(0);
                for (let yy = y - r; yy <= y + r; yy++) {
                    const row = cy(yy) * w;
                    for (let xx = -r; xx <= r; xx++) hist[src[(row + cx(xx)) * 4 + c]]++;
                }
                let med = 0, less = 0;
                while (less + hist[med] <= half) { less += hist[med]; med++; }
                for (let x = 0; x < w; x++) {
                    if (x > 0) {
                        const xr = cx(x + r), xl = cx(x - r - 1);
                        for (let yy = y - r; yy <= y + r; yy++) {
                            const row = cy(yy) * w;
                            const add = src[(row + xr) * 4 + c], sub = src[(row + xl) * 4 + c];
                            hist[add]++; if (add < med) less++;
                            hist[sub]--; if (sub < med) less--;
                        }
                        while (less > half) { med--; less -= hist[med]; }
                        while (less + hist[med] <= half) { less += hist[med]; med++; }
                    }
                    out[(y * w + x) * 4 + c] = med;
                }
            }
        }
        for (let i = 3; i < out.length; i += 4) out[i] = src[i];
        return { width: w, height: h, data: out };
    }

    def('median', { radius: 2 }, (img, p, c) => medianImage(img, Math.max(1, num(p.radius, 2) * c.scale)));

    // --- sharpen ---

    def('sharpen', { amount: 100 }, (img, p) => {
        const k = num(p.amount, 100) / 100, w = img.width, h = img.height, s = new Uint8ClampedArray(img.data), d = img.data;
        for (let y = 0; y < h; y++) {
            const y0 = y > 0 ? y - 1 : 0, y1 = y < h - 1 ? y + 1 : y;
            for (let x = 0; x < w; x++) {
                const x0 = x > 0 ? x - 1 : 0, x1 = x < w - 1 ? x + 1 : x;
                const i = (y * w + x) * 4, a = (y0 * w + x) * 4, b = (y1 * w + x) * 4, l = (y * w + x0) * 4, r = (y * w + x1) * 4;
                for (let c = 0; c < 3; c++) d[i + c] = s[i + c] + k * (4 * s[i + c] - s[a + c] - s[b + c] - s[l + c] - s[r + c]);
            }
        }
        return img;
    });

    // Adds back the difference to a blurred copy: per channel, only where it exceeds the threshold.
    function unsharp(img, sigma, amount, threshold, lumaOnly) {
        const blur = gaussBlurImage(img, sigma), d = img.data, b = blur.data;
        for (let i = 0; i < d.length; i += 4) {
            if (lumaOnly) {
                const dy = lumaOf(d[i], d[i + 1], d[i + 2]) - lumaOf(b[i], b[i + 1], b[i + 2]);
                if (Math.abs(dy) < threshold) continue;
                const add = dy * amount;
                d[i] += add; d[i + 1] += add; d[i + 2] += add;
            } else {
                for (let c = 0; c < 3; c++) {
                    const df = d[i + c] - b[i + c];
                    if (Math.abs(df) >= threshold) d[i + c] += df * amount;
                }
            }
        }
        return img;
    }

    def('unsharp-mask', { amount: 120, radius: 2, threshold: 0 }, (img, p, c) => unsharp(img, num(p.radius, 2) * c.scale, num(p.amount, 120) / 100, num(p.threshold, 0), false));

    def('high-pass', { radius: 6, gain: 1 }, (img, p, c) => {
        const b = gaussBlurImage(img, num(p.radius, 6) * c.scale).data, d = img.data, g = num(p.gain, 1);
        for (let i = 0; i < d.length; i += 4) { d[i] = 128 + (d[i] - b[i]) * g; d[i + 1] = 128 + (d[i + 1] - b[i + 1]) * g; d[i + 2] = 128 + (d[i + 2] - b[i + 2]) * g; }
        return img;
    });

    // Local contrast: unsharp mask with a big radius, strongest in the mid-tones.
    function localContrast(img, sigma, amount) {
        const b = gaussBlurImage(img, sigma).data, d = img.data;
        for (let i = 0; i < d.length; i += 4) {
            const y = lumaOf(d[i], d[i + 1], d[i + 2]), yb = lumaOf(b[i], b[i + 1], b[i + 2]);
            const t = y / 127.5 - 1, w = 1 - t * t * 0.8;
            const add = (y - yb) * amount * w;
            d[i] += add; d[i + 1] += add; d[i + 2] += add;
        }
        return img;
    }

    def('clarity', { amount: 60, radius: 30 }, (img, p, c) => localContrast(img, num(p.radius, 30) * c.scale, num(p.amount, 60) / 100));

    // --- noise ---

    def('add-noise', { amount: 25, distribution: 'gaussian', mono: true, seed: 1 }, (img, p) => {
        const a = clamp(num(p.amount, 25), 0, 100) / 100, rnd = mulberry32(num(p.seed, 1)), gaussian = p.distribution !== 'uniform', mono = p.mono !== false;
        const sigma = a * 80, range = a * 127, d = img.data;
        const draw = () => (gaussian ? gauss(rnd) * sigma : (rnd() * 2 - 1) * range);
        for (let i = 0; i < d.length; i += 4) {
            if (mono) { const n = draw(); d[i] += n; d[i + 1] += n; d[i + 2] += n; } else { d[i] += draw(); d[i + 1] += draw(); d[i + 2] += draw(); }
        }
        return img;
    });

    def('denoise', { method: 'bilateral', radius: 2, threshold: 30 }, (img, p) => {
        const r = Math.max(1, num(p.radius, 2));
        return p.method === 'median' ? medianImage(img, r) : surfaceBlur(img, r, clamp(num(p.threshold, 30), 1, 100));
    });

    def('dust-scratches', { radius: 2, threshold: 24 }, (img, p) => {
        const med = medianImage(img, Math.max(1, num(p.radius, 2))).data, th = num(p.threshold, 24), d = img.data;
        for (let i = 0; i < d.length; i += 4) {
            const diff = Math.max(Math.abs(d[i] - med[i]), Math.abs(d[i + 1] - med[i + 1]), Math.abs(d[i + 2] - med[i + 2]));
            if (diff > th) { d[i] = med[i]; d[i + 1] = med[i + 1]; d[i + 2] = med[i + 2]; }
        }
        return img;
    });

    // Unit-variance noise plane, blurred by `sigma` (grain size) and renormalised.
    function noisePlane(w, h, rnd, sigma) {
        const n = w * h;
        let plane = new Float32Array(n);
        for (let i = 0; i < n; i++) plane[i] = gauss(rnd);
        if (sigma > 0.3) {
            plane = blurPlane(plane, w, h, sigma);
            let sum = 0;
            for (let i = 0; i < n; i++) sum += plane[i] * plane[i];
            const k = 1 / Math.sqrt(sum / n || 1);
            for (let i = 0; i < n; i++) plane[i] *= k;
        }
        return plane;
    }

    def('grain', { amount: 30, size: 1.5, mono: true, seed: 1 }, (img, p, c) => {
        const w = img.width, h = img.height, amt = clamp(num(p.amount, 30), 0, 100) * 0.7, rnd = mulberry32(num(p.seed, 1)), sigma = Math.max(0, (num(p.size, 1.5) - 0.7) * c.scale);
        const mono = p.mono !== false, d = img.data;
        const planes = mono ? [noisePlane(w, h, rnd, sigma)] : [noisePlane(w, h, rnd, sigma), noisePlane(w, h, rnd, sigma), noisePlane(w, h, rnd, sigma)];
        for (let i = 0, q = 0; i < d.length; i += 4, q++) {
            const t = lumaOf(d[i], d[i + 1], d[i + 2]) / 127.5 - 1, tone = 1 - 0.7 * t * t;
            for (let k = 0; k < 3; k++) d[i + k] += planes[mono ? 0 : k][q] * amt * tone;
        }
        return img;
    });

    // --- distort ---

    const centerOf = (img, v) => { const c = pointOf(v, 0.5, 0.5); return { x: c.x * (img.width - 1), y: c.y * (img.height - 1) }; };
    const radiusOf = (img, percent) => clamp(num(percent, 100), 5, 400) / 100 * Math.min(img.width, img.height) / 2;

    def('twirl', { twist: 150, radius: 100, center: { x: 0.5, y: 0.5 } }, (img, p) => {
        const c = centerOf(img, p.center), R = radiusOf(img, p.radius), twist = num(p.twist, 150) * PI / 180;
        return warpImage(img, (x, y, out) => {
            const dx = x - c.x, dy = y - c.y, r = hyp(dx, dy);
            if (r >= R) return;
            const t = 1 - r / R, a = twist * t * t, cs = Math.cos(a), sn = Math.sin(a);
            out[0] = c.x + dx * cs - dy * sn; out[1] = c.y + dx * sn + dy * cs;
        });
    });

    def('pinch', { amount: 50, radius: 100, center: { x: 0.5, y: 0.5 } }, (img, p) => {
        const c = centerOf(img, p.center), R = radiusOf(img, p.radius), pw = Math.pow(2, -clamp(num(p.amount, 50), -100, 100) / 100);
        return warpImage(img, (x, y, out) => {
            const dx = x - c.x, dy = y - c.y, r = hyp(dx, dy);
            if (r >= R || r < 1e-6) return;
            const f = R * Math.pow(r / R, pw) / r;
            out[0] = c.x + dx * f; out[1] = c.y + dy * f;
        });
    });

    def('wave', { amplitude: 12, wavelength: 70, angle: 0 }, (img, p, c) => {
        const A = num(p.amplitude, 12) * c.scale, L = Math.max(2, num(p.wavelength, 70) * c.scale), a = num(p.angle, 0) * PI / 180;
        const ux = Math.cos(a), uy = Math.sin(a);
        return warpImage(img, (x, y, out) => {
            const off = A * Math.sin(TAU * (x * ux + y * uy) / L);
            out[0] = x - off * uy; out[1] = y + off * ux;
        });
    });

    def('ripple', { amplitude: 8, wavelength: 40, phase: 0, center: { x: 0.5, y: 0.5 } }, (img, p, c) => {
        const ctr = centerOf(img, p.center), A = num(p.amplitude, 8) * c.scale, L = Math.max(2, num(p.wavelength, 40) * c.scale), ph = num(p.phase, 0) * PI / 180;
        return warpImage(img, (x, y, out) => {
            const dx = x - ctr.x, dy = y - ctr.y, r = hyp(dx, dy);
            if (r < 1e-6) return;
            const f = (r + A * Math.sin(TAU * r / L - ph)) / r;
            out[0] = ctr.x + dx * f; out[1] = ctr.y + dy * f;
        });
    });

    def('polar', { mode: 'to_polar' }, (img, p) => {
        const w = img.width, h = img.height, cx = (w - 1) / 2, cy = (h - 1) / 2, maxR = hyp(w, h) / 2, toPolar = p.mode !== 'to_rect';
        return warpImage(img, (x, y, out) => {
            if (toPolar) {
                const dx = x - cx, dy = y - cy;
                out[0] = (Math.atan2(dy, dx) + PI) / TAU * (w - 1); out[1] = hyp(dx, dy) / maxR * (h - 1);
            } else {
                const ang = x / Math.max(1, w - 1) * TAU - PI, r = y / Math.max(1, h - 1) * maxR;
                out[0] = cx + r * Math.cos(ang); out[1] = cy + r * Math.sin(ang);
            }
        });
    });

    def('spherize', { amount: 60, radius: 100, center: { x: 0.5, y: 0.5 } }, (img, p) => {
        const c = centerOf(img, p.center), R = radiusOf(img, p.radius), a = clamp(num(p.amount, 60), -100, 100) / 100;
        return warpImage(img, (x, y, out) => {
            const dx = x - c.x, dy = y - c.y, r = hyp(dx, dy);
            if (r >= R || r < 1e-6) return;
            const d = r / R;
            const target = a >= 0 ? 1 - Math.sqrt(1 - d * d) : Math.sqrt(1 - (1 - d) * (1 - d));
            const f = R * lerp(d, target, Math.abs(a)) / r;
            out[0] = c.x + dx * f; out[1] = c.y + dy * f;
        });
    });

    def('kaleidoscope', { segments: 6, rotation: 0, zoom: 100, center: { x: 0.5, y: 0.5 } }, (img, p) => {
        const c = centerOf(img, p.center), n = clamp(Math.round(num(p.segments, 6)), 2, 48), rot = num(p.rotation, 0) * PI / 180;
        const seg = TAU / n, zoom = clamp(num(p.zoom, 100), 20, 400) / 100;
        return warpImage(img, (x, y, out) => {
            const dx = x - c.x, dy = y - c.y, r = hyp(dx, dy) / zoom;
            let a = (Math.atan2(dy, dx) - rot) % seg;
            if (a < 0) a += seg;
            if (a > seg / 2) a = seg - a;
            const ang = a + rot;
            out[0] = c.x + r * Math.cos(ang); out[1] = c.y + r * Math.sin(ang);
        });
    });

    def('displace', { amount: 24, scale: 70, seed: 1 }, (img, p, c) => {
        const A = num(p.amount, 24) * c.scale, s = Math.max(2, num(p.scale, 70) * c.scale), t = noiseTable(num(p.seed, 1));
        return warpImage(img, (x, y, out) => {
            out[0] = x + fbm(t, x / s, y / s) * A; out[1] = y + fbm(t, x / s + 31.7, y / s - 12.3) * A;
        });
    });

    def('glass', { texture: 'frosted', amount: 4, scale: 12, seed: 1 }, (img, p, c) => {
        const A = num(p.amount, 4) * c.scale, s = Math.max(2, num(p.scale, 12) * c.scale), seed = num(p.seed, 1) | 0, kind = p.texture;
        return warpImage(img, (x, y, out) => {
            if (kind === 'ribs') {
                const f = x / s - Math.floor(x / s);
                out[0] = x + (f - 0.5) * 2 * A;
            } else if (kind === 'blocks') {
                const bx = Math.floor(x / s), by = Math.floor(y / s);
                out[0] = x + (cellHash(bx, by, seed) - 0.5) * 2 * A; out[1] = y + (cellHash(bx, by, seed + 7) - 0.5) * 2 * A;
            } else {
                out[0] = x + (cellHash(x, y, seed) - 0.5) * 2 * A; out[1] = y + (cellHash(x, y, seed + 7) - 0.5) * 2 * A;
            }
        });
    });

    def('lens-distortion', { amount: 40, zoom: 100 }, (img, p) => {
        const w = img.width, h = img.height, cx = (w - 1) / 2, cy = (h - 1) / 2, half = hyp(cx, cy) || 1;
        const k = clamp(num(p.amount, 40), -100, 100) / 100 * 0.5, zoom = clamp(num(p.zoom, 100), 50, 200) / 100;
        return warpImage(img, (x, y, out) => {
            const dx = (x - cx) / half, dy = (y - cy) / half, f = (1 + k * (dx * dx + dy * dy)) / zoom;
            out[0] = cx + (x - cx) * f; out[1] = cy + (y - cy) * f;
        });
    });

    // --- stylize: shared pieces ---

    function channelPlane(img, c) {
        const d = img.data, n = img.width * img.height, out = new Float32Array(n);
        for (let i = 0, p = 0; p < n; i += 4, p++) out[p] = d[i + c];
        return out;
    }

    // Sobel gradient of a plane (edge clamp): gx, gy are the raw 3x3 responses.
    function sobelGrad(plane, w, h) {
        const gx = new Float32Array(w * h), gy = new Float32Array(w * h);
        for (let y = 0; y < h; y++) {
            const y0 = (y > 0 ? y - 1 : 0) * w, y1 = y * w, y2 = (y < h - 1 ? y + 1 : y) * w;
            for (let x = 0; x < w; x++) {
                const x0 = x > 0 ? x - 1 : 0, x2 = x < w - 1 ? x + 1 : x;
                const a = plane[y0 + x0], b = plane[y0 + x], c = plane[y0 + x2];
                const d = plane[y1 + x0], f = plane[y1 + x2];
                const g = plane[y2 + x0], hh = plane[y2 + x], i = plane[y2 + x2];
                gx[y1 + x] = (c + 2 * f + i) - (a + 2 * d + g);
                gy[y1 + x] = (g + 2 * hh + i) - (a + 2 * b + c);
            }
        }
        return { gx, gy };
    }

    function sobelMag(plane, w, h) {
        const g = sobelGrad(plane, w, h), out = new Float32Array(w * h);
        for (let i = 0; i < out.length; i++) out[i] = hyp(g.gx[i], g.gy[i]);
        return out;
    }

    // Average colour (alpha weighted) of the pixels in [x0, x1) x [y0, y1) -> [r, g, b, a].
    function blockColor(d, w, x0, y0, x1, y1) {
        let sr = 0, sg = 0, sb = 0, sa = 0;
        for (let y = y0; y < y1; y++) {
            for (let x = x0; x < x1; x++) {
                const i = (y * w + x) * 4, a = d[i + 3];
                sr += d[i] * a; sg += d[i + 1] * a; sb += d[i + 2] * a; sa += a;
            }
        }
        const n = (x1 - x0) * (y1 - y0);
        return sa > 0 ? [sr / sa, sg / sa, sb / sa, sa / n] : [0, 0, 0, 0];
    }

    function fillBlock(d, w, x0, y0, x1, y1, col) {
        for (let y = y0; y < y1; y++) {
            for (let x = x0; x < x1; x++) {
                const i = (y * w + x) * 4;
                d[i] = col[0]; d[i + 1] = col[1]; d[i + 2] = col[2]; d[i + 3] = col[3];
            }
        }
    }

    // --- palettes, quantising and dithering ---

    function websafe() {
        const out = [];
        for (let r = 0; r < 6; r++) for (let g = 0; g < 6; g++) for (let b = 0; b < 6; b++) out.push([r * 51, g * 51, b * 51]);
        return out;
    }

    const PALETTE_HEX = {
        '1bit': ['#000000', '#ffffff'],
        gameboy: ['#0f380f', '#306230', '#8bac0f', '#9bbc0f'],
        cga: ['#000000', '#55ffff', '#ff55ff', '#ffffff'],
        c64: ['#000000', '#ffffff', '#68372b', '#70a4b2', '#6f3d86', '#588d43', '#352879', '#b8c76f', '#6f4f25', '#433900', '#9a6759', '#444444', '#6c6c6c', '#9ad284', '#6c5eb5', '#959595'],
        pico8: ['#000000', '#1d2b53', '#7e2553', '#008751', '#ab5236', '#5f574f', '#c2c3c7', '#fff1e8', '#ff004d', '#ffa300', '#ffec27', '#00e436', '#29adff', '#83769c', '#ff77a8', '#ffccaa'],
        ega: ['#000000', '#0000aa', '#00aa00', '#00aaaa', '#aa0000', '#aa00aa', '#aa5500', '#aaaaaa', '#555555', '#5555ff', '#55ff55', '#55ffff', '#ff5555', '#ff55ff', '#ffff55', '#ffffff']
    };
    const PALETTE_IDS = ['1bit', 'gameboy', 'cga', 'c64', 'pico8', 'ega', 'websafe', 'custom'];

    // Median cut over a deterministic sample of the opaque pixels; returns up to `count` average colours.
    function medianCut(img, count) {
        const d = img.data, n = img.width * img.height, step = Math.max(1, Math.floor(n / 60000));
        let pix = [];
        for (let p = 0; p < n; p += step) { const i = p * 4; if (d[i + 3] > 0) pix.push([d[i], d[i + 1], d[i + 2]]); }
        if (!pix.length) return [[0, 0, 0], [255, 255, 255]];
        let boxes = [pix];
        const target = clamp(Math.round(count), 2, 256);
        while (boxes.length < target) {
            let bi = -1, best = -1, axis = 0;
            boxes.forEach((box, k) => {
                if (box.length < 2) return;
                for (let c = 0; c < 3; c++) {
                    let lo = 255, hi = 0;
                    for (let j = 0; j < box.length; j++) { const v = box[j][c]; if (v < lo) lo = v; if (v > hi) hi = v; }
                    const score = (hi - lo) * Math.sqrt(box.length);
                    if (score > best) { best = score; bi = k; axis = c; }
                }
            });
            if (bi < 0 || best <= 0) break;
            const box = boxes[bi].slice().sort((a, b) => a[axis] - b[axis]), mid = box.length >> 1;
            boxes.splice(bi, 1, box.slice(0, mid), box.slice(mid));
        }
        return boxes.map(box => {
            let r = 0, g = 0, b = 0;
            box.forEach(q => { r += q[0]; g += q[1]; b += q[2]; });
            return [r / box.length, g / box.length, b / box.length].map(Math.round);
        });
    }

    function paletteFor(id, img, count) {
        if (id === 'websafe') return websafe();
        if (PALETTE_HEX[id]) return PALETTE_HEX[id].map(hex => hexRGB(hex));
        return medianCut(img, count);
    }

    // Nearest palette entry; small palettes are searched directly, big ones through a 5-bit cache.
    function makeQuantizer(pal) {
        const n = pal.length, pr = new Float32Array(n), pg = new Float32Array(n), pb = new Float32Array(n);
        pal.forEach((c, i) => { pr[i] = c[0]; pg[i] = c[1]; pb[i] = c[2]; });
        const search = (r, g, b) => {
            let best = 0, bd = Infinity;
            for (let i = 0; i < n; i++) {
                const dr = r - pr[i], dg = g - pg[i], db = b - pb[i];
                const dist = 2 * dr * dr + 4 * dg * dg + 3 * db * db;
                if (dist < bd) { bd = dist; best = i; }
            }
            return best;
        };
        if (n <= 24) return search;
        const cache = new Int16Array(32768).fill(-1);
        return (r, g, b) => {
            const rr = r < 0 ? 0 : (r > 255 ? 255 : r), gg = g < 0 ? 0 : (g > 255 ? 255 : g), bb = b < 0 ? 0 : (b > 255 ? 255 : b);
            const key = ((rr >> 3) << 10) | ((gg >> 3) << 5) | (bb >> 3);
            let idx = cache[key];
            if (idx < 0) idx = cache[key] = search(((rr >> 3) << 3) + 4, ((gg >> 3) << 3) + 4, ((bb >> 3) << 3) + 4);
            return idx;
        };
    }

    function bayerMatrix(order) {
        let m = [[0, 2], [3, 1]];
        for (let size = 2; size < order; size *= 2) {
            const next = [];
            for (let y = 0; y < size * 2; y++) next.push(new Array(size * 2));
            for (let y = 0; y < size; y++) {
                for (let x = 0; x < size; x++) {
                    const v = m[y][x] * 4;
                    next[y][x] = v; next[y][x + size] = v + 2; next[y + size][x] = v + 3; next[y + size][x + size] = v + 1;
                }
            }
            m = next;
        }
        return m;
    }

    function ditherImage(img, method, pal, strength) {
        const w = img.width, h = img.height, d = img.data, q = makeQuantizer(pal);
        if (method.indexOf('bayer') === 0) {
            const order = method === 'bayer2' ? 2 : (method === 'bayer8' ? 8 : 4), m = bayerMatrix(order), nn = order * order;
            const spread = (pal.length <= 4 ? 255 : 330 / Math.cbrt(pal.length)) * clamp(strength, 0, 100) / 100;
            for (let y = 0; y < h; y++) {
                for (let x = 0; x < w; x++) {
                    const i = (y * w + x) * 4, t = ((m[y % order][x % order] + 0.5) / nn - 0.5) * spread;
                    const c = pal[q(d[i] + t, d[i + 1] + t, d[i + 2] + t)];
                    d[i] = c[0]; d[i + 1] = c[1]; d[i + 2] = c[2];
                }
            }
            return img;
        }
        const atkinson = method === 'atkinson', rowLen = (w + 4) * 3;
        let rows = [new Float32Array(rowLen), new Float32Array(rowLen), new Float32Array(rowLen)];
        for (let y = 0; y < h; y++) {
            const cur = rows[0], n1 = rows[1], n2 = rows[2];
            const ltr = (y & 1) === 0;
            for (let k = 0; k < w; k++) {
                const x = ltr ? k : w - 1 - k, i = (y * w + x) * 4, e = (x + 2) * 3;
                const r = d[i] + cur[e], g = d[i + 1] + cur[e + 1], b = d[i + 2] + cur[e + 2];
                const c = pal[q(r, g, b)];
                d[i] = c[0]; d[i + 1] = c[1]; d[i + 2] = c[2];
                const er = r - c[0], eg = g - c[1], eb = b - c[2], dir = ltr ? 3 : -3;
                if (atkinson) {
                    const f = 1 / 8;
                    cur[e + dir] += er * f; cur[e + dir + 1] += eg * f; cur[e + dir + 2] += eb * f;
                    cur[e + 2 * dir] += er * f; cur[e + 2 * dir + 1] += eg * f; cur[e + 2 * dir + 2] += eb * f;
                    n1[e - dir] += er * f; n1[e - dir + 1] += eg * f; n1[e - dir + 2] += eb * f;
                    n1[e] += er * f; n1[e + 1] += eg * f; n1[e + 2] += eb * f;
                    n1[e + dir] += er * f; n1[e + dir + 1] += eg * f; n1[e + dir + 2] += eb * f;
                    n2[e] += er * f; n2[e + 1] += eg * f; n2[e + 2] += eb * f;
                } else {
                    cur[e + dir] += er * 0.4375; cur[e + dir + 1] += eg * 0.4375; cur[e + dir + 2] += eb * 0.4375;
                    n1[e - dir] += er * 0.1875; n1[e - dir + 1] += eg * 0.1875; n1[e - dir + 2] += eb * 0.1875;
                    n1[e] += er * 0.3125; n1[e + 1] += eg * 0.3125; n1[e + 2] += eb * 0.3125;
                    n1[e + dir] += er * 0.0625; n1[e + dir + 1] += eg * 0.0625; n1[e + dir + 2] += eb * 0.0625;
                }
            }
            cur.fill(0);
            rows = [n1, n2, cur];
        }
        return img;
    }

    def('dither', { method: 'floyd-steinberg', palette: '1bit', colors: 8, strength: 100 }, (img, p) => {
        const pal = paletteFor(p.palette, img, num(p.colors, 8));
        return ditherImage(img, p.method || 'floyd-steinberg', pal, num(p.strength, 100));
    });

    def('pixel-art', { block: 6, palette: 'none', colors: 16 }, (img, p, c) => {
        const w = img.width, h = img.height, d = img.data, s = Math.max(1, Math.round(num(p.block, 6) * c.scale));
        const pal = p.palette && p.palette !== 'none' ? paletteFor(p.palette, img, num(p.colors, 16)) : null, q = pal ? makeQuantizer(pal) : null;
        for (let by = 0; by < h; by += s) {
            for (let bx = 0; bx < w; bx += s) {
                const x1 = Math.min(w, bx + s), y1 = Math.min(h, by + s), col = blockColor(d, w, bx, by, x1, y1);
                if (q && col[3] > 0) { const e = pal[q(col[0], col[1], col[2])]; col[0] = e[0]; col[1] = e[1]; col[2] = e[2]; }
                fillBlock(d, w, bx, by, x1, y1, col);
            }
        }
        return img;
    });

    def('pixelate', { size: 12 }, (img, p, c) => {
        const w = img.width, h = img.height, d = img.data, s = Math.max(1, Math.round(num(p.size, 12) * c.scale));
        for (let by = 0; by < h; by += s) for (let bx = 0; bx < w; bx += s) { const x1 = Math.min(w, bx + s), y1 = Math.min(h, by + s); fillBlock(d, w, bx, by, x1, y1, blockColor(d, w, bx, by, x1, y1)); }
        return img;
    });

    def('mosaic', { size: 24, grout: 1.5, grout_color: '#1c1c1c', variation: 12, seed: 1 }, (img, p, c) => {
        const w = img.width, h = img.height, d = img.data, s = Math.max(2, Math.round(num(p.size, 24) * c.scale)), seed = num(p.seed, 1) | 0;
        const g = Math.max(0, num(p.grout, 1.5) * c.scale), gc = hexRGB(p.grout_color, [28, 28, 28]), vary = clamp(num(p.variation, 12), 0, 100) / 100 * 0.6;
        const out = new Uint8ClampedArray(d.length);
        for (let by = 0; by < h; by += s) {
            for (let bx = 0; bx < w; bx += s) {
                const x1 = Math.min(w, bx + s), y1 = Math.min(h, by + s), col = blockColor(d, w, bx, by, x1, y1);
                const f = 1 + (cellHash(bx / s, by / s, seed) - 0.5) * 2 * vary;
                for (let y = by; y < y1; y++) {
                    for (let x = bx; x < x1; x++) {
                        const i = (y * w + x) * 4, edge = x - bx < g || y - by < g;
                        out[i] = edge ? gc[0] : col[0] * f; out[i + 1] = edge ? gc[1] : col[1] * f; out[i + 2] = edge ? gc[2] : col[2] * f; out[i + 3] = col[3];
                    }
                }
            }
        }
        img.data.set(out);
        return img;
    });

    // --- stylize: painterly and print looks ---

    // Kuwahara: each pixel takes the mean colour of the least varied of its four overlapping quadrants.
    function kuwahara(img, r) {
        const w = img.width, h = img.height, d = img.data, W = w + 1;
        const ir = new Float64Array(W * (h + 1)), ig = new Float64Array(W * (h + 1)), ib = new Float64Array(W * (h + 1)), il = new Float64Array(W * (h + 1)), iq = new Float64Array(W * (h + 1));
        for (let y = 0; y < h; y++) {
            let sr = 0, sg = 0, sb = 0, sl = 0, sq = 0;
            for (let x = 0; x < w; x++) {
                const i = (y * w + x) * 4, l = lumaOf(d[i], d[i + 1], d[i + 2]);
                sr += d[i]; sg += d[i + 1]; sb += d[i + 2]; sl += l; sq += l * l;
                const k = (y + 1) * W + x + 1, up = y * W + x + 1;
                ir[k] = ir[up] + sr; ig[k] = ig[up] + sg; ib[k] = ib[up] + sb; il[k] = il[up] + sl; iq[k] = iq[up] + sq;
            }
        }
        const out = new Uint8ClampedArray(d.length);
        const box = (arr, x0, y0, x1, y1) => arr[(y1 + 1) * W + x1 + 1] - arr[y0 * W + x1 + 1] - arr[(y1 + 1) * W + x0] + arr[y0 * W + x0];
        for (let y = 0; y < h; y++) {
            for (let x = 0; x < w; x++) {
                let bestVar = Infinity, br = 0, bg = 0, bb = 0;
                for (let q = 0; q < 4; q++) {
                    const x0 = Math.max(0, q & 1 ? x : x - r), x1 = Math.min(w - 1, q & 1 ? x + r : x);
                    const y0 = Math.max(0, q & 2 ? y : y - r), y1 = Math.min(h - 1, q & 2 ? y + r : y);
                    const n = (x1 - x0 + 1) * (y1 - y0 + 1), ml = box(il, x0, y0, x1, y1) / n;
                    const variance = box(iq, x0, y0, x1, y1) / n - ml * ml;
                    if (variance < bestVar) { bestVar = variance; br = box(ir, x0, y0, x1, y1) / n; bg = box(ig, x0, y0, x1, y1) / n; bb = box(ib, x0, y0, x1, y1) / n; }
                }
                const o = (y * w + x) * 4;
                out[o] = br; out[o + 1] = bg; out[o + 2] = bb; out[o + 3] = d[o + 3];
            }
        }
        return { width: w, height: h, data: out };
    }

    def('oil-paint', { radius: 4, detail: 20 }, (img, p, c) => {
        const r = Math.max(1, Math.round(num(p.radius, 4) * c.scale)), keep = clamp(num(p.detail, 20), 0, 100) / 100 * 0.6;
        const res = inBands(img, r + 1, sub => kuwahara(sub, r));
        if (keep > 0) { const d = res.data, s = img.data; for (let i = 0; i < d.length; i += 4) { d[i] = lerp(d[i], s[i], keep); d[i + 1] = lerp(d[i + 1], s[i + 1], keep); d[i + 2] = lerp(d[i + 2], s[i + 2], keep); } }
        return res;
    });

    // Rotated dot screen: fills o with the distance to the centre of the screen cell of (x, y) and that centre in image coordinates.
    function dotCell(x, y, size, cs, sn, o) {
        const u = x * cs + y * sn, v = -x * sn + y * cs;
        const uc = (Math.floor(u / size) + 0.5) * size, vc = (Math.floor(v / size) + 0.5) * size;
        o[0] = hyp(u - uc, v - vc); o[1] = uc * cs - vc * sn; o[2] = uc * sn + vc * cs;
    }

    // Coverage (0..1) of a dot whose area follows the ink amount `tone` (0..1).
    function dotCoverage(dist, tone, size) {
        return clamp(size * Math.sqrt(clamp(tone, 0, 1) / PI) * 1.05 - dist + 0.5, 0, 1);
    }

    def('halftone', { size: 8, angle: 45, mode: 'mono', ink: '#000000', paper: '#ffffff' }, (img, p, c) => {
        const w = img.width, h = img.height, size = Math.max(2, num(p.size, 8) * c.scale), base = num(p.angle, 45) * PI / 180;
        const src = gaussBlurImage(img, size * 0.35).data, d = img.data, px = [0, 0, 0, 0], cell = [0, 0, 0];
        const color = p.mode === 'color';
        const ink = hexRGB(p.ink, [0, 0, 0]), paper = hexRGB(p.paper, [255, 255, 255]);
        const angles = color ? [base - PI / 6, base + PI / 6, base - PI / 4] : [base];
        const cos = angles.map(Math.cos), sin = angles.map(Math.sin);
        for (let y = 0; y < h; y++) {
            for (let x = 0; x < w; x++) {
                const i = (y * w + x) * 4;
                if (color) {
                    for (let ch = 0; ch < 3; ch++) {
                        dotCell(x, y, size, cos[ch], sin[ch], cell);
                        sampleRGBA(src, w, h, cell[1], cell[2], px, 0);
                        d[i + ch] = 255 * (1 - dotCoverage(cell[0], 1 - px[ch] / 255, size));
                    }
                } else {
                    dotCell(x, y, size, cos[0], sin[0], cell);
                    sampleRGBA(src, w, h, cell[1], cell[2], px, 0);
                    const cov = dotCoverage(cell[0], 1 - lumaOf(px[0], px[1], px[2]) / 255, size);
                    for (let ch = 0; ch < 3; ch++) d[i + ch] = lerp(paper[ch], ink[ch], cov);
                }
            }
        }
        return img;
    });

    // Jittered grid of sites covering the image with one ring of margin cells: arrays sx, sy of gw * gh entries.
    function siteGrid(w, h, size, seed, jitter) {
        const gw = Math.ceil(w / size) + 2, gh = Math.ceil(h / size) + 2, sx = new Float32Array(gw * gh), sy = new Float32Array(gw * gh);
        for (let j = 0; j < gh; j++) {
            for (let i = 0; i < gw; i++) {
                sx[j * gw + i] = ((i - 1) + 0.5 + (cellHash(i - 1, j - 1, seed) - 0.5) * jitter) * size;
                sy[j * gw + i] = ((j - 1) + 0.5 + (cellHash(i - 1, j - 1, seed + 11) - 0.5) * jitter) * size;
            }
        }
        return { gw, gh, sx, sy };
    }

    // Index of the site nearest to (x, y) among the nine cells around it.
    function nearestCell(g, x, y, size) {
        const gx = Math.floor(x / size) + 1, gy = Math.floor(y / size) + 1;
        let best = Infinity, idx = 0;
        for (let j = gy - 1; j <= gy + 1; j++) {
            if (j < 0 || j >= g.gh) continue;
            for (let i = gx - 1; i <= gx + 1; i++) {
                if (i < 0 || i >= g.gw) continue;
                const q = j * g.gw + i, dx = g.sx[q] - x, dy = g.sy[q] - y, dd = dx * dx + dy * dy;
                if (dd < best) { best = dd; idx = q; }
            }
        }
        return idx;
    }

    def('crystallize', { cell: 24, seed: 1 }, (img, p, c) => {
        const w = img.width, h = img.height, d = img.data, size = Math.max(3, num(p.cell, 24) * c.scale), g = siteGrid(w, h, size, num(p.seed, 1) | 0, 0.7);
        const n = g.gw * g.gh, acc = new Float64Array(n * 4), count = new Float64Array(n);
        for (let y = 0; y < h; y++) {
            for (let x = 0; x < w; x++) {
                const q = nearestCell(g, x, y, size), i = (y * w + x) * 4, a = d[i + 3];
                acc[q * 4] += d[i] * a; acc[q * 4 + 1] += d[i + 1] * a; acc[q * 4 + 2] += d[i + 2] * a; acc[q * 4 + 3] += a; count[q]++;
            }
        }
        for (let y = 0; y < h; y++) {
            for (let x = 0; x < w; x++) {
                const q = nearestCell(g, x, y, size), i = (y * w + x) * 4, sa = acc[q * 4 + 3];
                if (sa > 0) { d[i] = acc[q * 4] / sa; d[i + 1] = acc[q * 4 + 1] / sa; d[i + 2] = acc[q * 4 + 2] / sa; }
                d[i + 3] = sa / count[q];
            }
        }
        return img;
    });

    def('pointillize', { size: 10, seed: 1, paper: '#ffffff' }, (img, p, c) => {
        const w = img.width, h = img.height, d = img.data, size = Math.max(3, num(p.size, 10) * c.scale), seed = num(p.seed, 1) | 0, paper = hexRGB(p.paper, [255, 255, 255]);
        const g = siteGrid(w, h, size, seed, 0.5), n = g.gw * g.gh, radius = new Float32Array(n), pri = new Float32Array(n), col = new Float32Array(n * 3), px = [0, 0, 0, 0];
        for (let j = 0; j < g.gh; j++) {
            for (let i = 0; i < g.gw; i++) {
                const q = j * g.gw + i;
                radius[q] = size * (0.42 + 0.3 * cellHash(i - 1, j - 1, seed + 23));
                pri[q] = cellHash(i - 1, j - 1, seed + 37);
                sampleRGBA(img.data, w, h, g.sx[q], g.sy[q], px, 0);
                col[q * 3] = px[0]; col[q * 3 + 1] = px[1]; col[q * 3 + 2] = px[2];
            }
        }
        const out = new Uint8ClampedArray(d.length);
        for (let y = 0; y < h; y++) {
            const gy = Math.floor(y / size) + 1;
            for (let x = 0; x < w; x++) {
                const gx = Math.floor(x / size) + 1;
                let best = -1, bestCov = 0;
                for (let j = gy - 1; j <= gy + 1; j++) {
                    if (j < 0 || j >= g.gh) continue;
                    for (let i = gx - 1; i <= gx + 1; i++) {
                        if (i < 0 || i >= g.gw) continue;
                        const q = j * g.gw + i, dx = g.sx[q] - x, dy = g.sy[q] - y;
                        const cov = radius[q] - Math.sqrt(dx * dx + dy * dy) + 0.5;
                        if (cov > 0 && (best < 0 || pri[q] > pri[best])) { best = q; bestCov = cov > 1 ? 1 : cov; }
                    }
                }
                const o = (y * w + x) * 4;
                if (best >= 0) { out[o] = paper[0] + (col[best * 3] - paper[0]) * bestCov; out[o + 1] = paper[1] + (col[best * 3 + 1] - paper[1]) * bestCov; out[o + 2] = paper[2] + (col[best * 3 + 2] - paper[2]) * bestCov; } else { out[o] = paper[0]; out[o + 1] = paper[1]; out[o + 2] = paper[2]; }
                out[o + 3] = d[o + 3];
            }
        }
        d.set(out);
        return img;
    });

    // Lighting-direction relief: gradient along the light direction added to mid gray.
    def('emboss', { angle: 135, height: 2, amount: 100, keep_color: false }, (img, p, c) => {
        const w = img.width, h = img.height, d = img.data, a = num(p.angle, 135) * PI / 180, dx = Math.cos(a), dy = Math.sin(a);
        const k = num(p.height, 2) * c.scale * clamp(num(p.amount, 100), 0, 300) / 100;
        const chans = p.keep_color ? [0, 1, 2].map(ch => channelPlane(img, ch)) : [lumaPlane(img)];
        const grads = chans.map(pl => sobelGrad(pl, w, h));
        for (let q = 0, i = 0; q < w * h; q++, i += 4) {
            for (let ch = 0; ch < 3; ch++) {
                const g = grads[grads.length === 1 ? 0 : ch];
                d[i + ch] = 128 + (g.gx[q] * dx + g.gy[q] * dy) * 0.5 * k;
            }
        }
        return img;
    });

    def('edge', { method: 'sobel', gain: 1, invert: false, color: true }, (img, p) => {
        const w = img.width, h = img.height, d = img.data, gain = num(p.gain, 1), inv = !!p.invert, color = p.color !== false;
        const chans = color ? [0, 1, 2].map(ch => channelPlane(img, ch)) : [lumaPlane(img)];
        const mags = chans.map(pl => {
            if (p.method === 'laplacian') {
                const out = new Float32Array(w * h);
                for (let y = 0; y < h; y++) {
                    for (let x = 0; x < w; x++) {
                        let sum = 0;
                        for (let j = -1; j <= 1; j++) for (let i = -1; i <= 1; i++) sum += pl[clamp(y + j, 0, h - 1) * w + clamp(x + i, 0, w - 1)];
                        out[y * w + x] = Math.abs(9 * pl[y * w + x] - sum);
                    }
                }
                return out;
            }
            return sobelMag(pl, w, h);
        });
        const lap = p.method === 'laplacian';
        for (let q = 0, i = 0; q < w * h; q++, i += 4) {
            for (let ch = 0; ch < 3; ch++) {
                const v = mags[mags.length === 1 ? 0 : ch][q] * gain * (lap ? 1 : 0.5);
                d[i + ch] = inv ? 255 - v : v;
            }
        }
        return img;
    });

    def('glowing-edges', { width: 2, brightness: 60, smoothness: 5 }, (img, p, c) => {
        const w = img.width, h = img.height, d = img.data, pre = gaussBlurImage(img, Math.max(0, num(p.smoothness, 5) * 0.15 * c.scale));
        const gain = num(p.brightness, 60) / 50 * 0.9;
        const edges = [0, 1, 2].map(ch => sobelMag(channelPlane(pre, ch), w, h));
        const out = { width: w, height: h, data: new Uint8ClampedArray(d.length) };
        for (let q = 0, i = 0; q < w * h; q++, i += 4) { for (let ch = 0; ch < 3; ch++) out.data[i + ch] = Math.pow(c255(edges[ch][q] * 0.5 * gain) / 255, 0.8) * 255; out.data[i + 3] = d[i + 3]; }
        const glow = gaussBlurImage(out, Math.max(0, num(p.width, 2) * c.scale)).data;
        for (let i = 0; i < d.length; i += 4) for (let ch = 0; ch < 3; ch++) d[i + ch] = out.data[i + ch] * 0.7 + glow[i + ch] * 1.1;
        return img;
    });

    def('sketch', { radius: 6, darkness: 65, keep_color: false }, (img, p, c) => {
        const w = img.width, h = img.height, d = img.data, gray = lumaPlane(img), inv = new Float32Array(gray.length);
        for (let i = 0; i < inv.length; i++) inv[i] = 255 - gray[i];
        const blur = blurPlane(inv, w, h, Math.max(0.5, num(p.radius, 6) * c.scale)), gamma = Math.pow(2, (num(p.darkness, 65) - 50) / 25), keep = !!p.keep_color;
        for (let q = 0, i = 0; q < w * h; q++, i += 4) {
            const dodge = Math.min(255, gray[q] * 255 / Math.max(1, 255 - blur[q])), v = Math.pow(dodge / 255, gamma) * 255;
            if (keep) { const f = v / 255; d[i] = d[i] * f; d[i + 1] = d[i + 1] * f; d[i + 2] = d[i + 2] * f; } else { d[i] = d[i + 1] = d[i + 2] = v; }
        }
        return img;
    });

    // Flat colour areas (smoothed, luma posterised) with dark outlines from a Sobel edge map.
    function toonImage(img, levels, edgeAmount, blurRadius) {
        const w = img.width, h = img.height;
        const base = blurRadius > 0.5 ? surfaceBlur(cloneImage(img), blurRadius, 30) : cloneImage(img), d = base.data;
        const edges = sobelMag(lumaPlane(blurRadius > 0.5 ? base : img), w, h), L = Math.max(2, levels);
        const lo = lerp(220, 60, edgeAmount / 100), hi = lo + 80, hsl = [0, 0, 0], rgb = [0, 0, 0];
        for (let q = 0, i = 0; q < w * h; q++, i += 4) {
            const y = lumaOf(d[i], d[i + 1], d[i + 2]);
            const yq = Math.round(y / 255 * (L - 1)) / (L - 1) * 255;
            Lib.rgbToHsl(d[i], d[i + 1], d[i + 2], hsl);
            Lib.hslToRgb(hsl[0], clamp(hsl[1] * 1.15, 0, 1), yq / 255 * 0.85 + 0.075, rgb);
            const line = edgeAmount > 0 ? smooth(lo, hi, edges[q]) * (edgeAmount / 100) : 0;
            const f = 1 - line;
            d[i] = rgb[0] * f; d[i + 1] = rgb[1] * f; d[i + 2] = rgb[2] * f;
        }
        return base;
    }

    def('toon', { levels: 5, edge: 60, smooth: 3 }, (img, p, c) => toonImage(img, Math.round(num(p.levels, 5)), clamp(num(p.edge, 60), 0, 100), num(p.smooth, 3) * c.scale));

    def('comic', { levels: 4, outline: 70, dots: 6 }, (img, p, c) => {
        const base = toonImage(img, Math.round(num(p.levels, 4)), clamp(num(p.outline, 70), 0, 100), 2 * c.scale);
        const dots = num(p.dots, 6);
        if (dots >= 2) {
            const ht = applyFilter(img, 'halftone', { size: dots, angle: 45, mode: 'mono' }, { scale: c.scale }).data, d = base.data;
            for (let i = 0; i < d.length; i += 4) { const f = lerp(1, ht[i] / 255, 0.5); d[i] *= f; d[i + 1] *= f; d[i + 2] *= f; }
        }
        return base;
    });

    def('neon', { glow: 6, intensity: 100, darken: 70 }, (img, p, c) => {
        const w = img.width, h = img.height, d = img.data, pre = gaussBlurImage(img, 0.8 * c.scale);
        const edges = [0, 1, 2].map(ch => sobelMag(channelPlane(pre, ch), w, h)), k = num(p.intensity, 100) / 100, dark = 1 - clamp(num(p.darken, 70), 0, 100) / 100 * 0.9;
        const layer = { width: w, height: h, data: new Uint8ClampedArray(d.length) };
        for (let q = 0, i = 0; q < w * h; q++, i += 4) {
            const e = [edges[0][q], edges[1][q], edges[2][q]], m = Math.max(e[0], e[1], e[2], 1), boost = Math.min(255, m * 0.7) / m;
            for (let ch = 0; ch < 3; ch++) layer.data[i + ch] = e[ch] * boost;
            layer.data[i + 3] = d[i + 3];
        }
        const glow = gaussBlurImage(layer, Math.max(0.5, num(p.glow, 6) * c.scale)).data;
        for (let i = 0; i < d.length; i += 4) for (let ch = 0; ch < 3; ch++) d[i + ch] = d[i + ch] * dark * 0.5 + (layer.data[i + ch] + glow[i + ch] * 0.9) * k;
        return img;
    });

    // --- light ---

    // amount > 0 darkens the edges, < 0 lightens them. size/feather 0..100, roundness -100 (boxy) .. 100 (circle).
    function vignetteImage(img, amount, size, roundness, feather) {
        const w = img.width, h = img.height, d = img.data, amt = clamp(amount / 100, -1, 1);
        if (!amt) return img;
        const cx = (w - 1) / 2, cy = (h - 1) / 2, big = Math.max(w, h) / 2, t = clamp(roundness, 0, 100) / 100;
        const rx = lerp(w / 2, big, t), ry = lerp(h / 2, big, t), n = roundness < 0 ? 2 + (-roundness / 100) * 4 : 2;
        const inner = clamp(size, 0, 100) / 100 * 1.2, span = 0.15 + clamp(feather, 0, 100) / 100 * 1.2;
        for (let y = 0; y < h; y++) {
            const ny = Math.abs((y - cy) / ry);
            for (let x = 0; x < w; x++) {
                const nx = Math.abs((x - cx) / rx);
                const dist = n === 2 ? Math.sqrt(nx * nx + ny * ny) : Math.pow(Math.pow(nx, n) + Math.pow(ny, n), 1 / n);
                const f = smooth(inner, inner + span, dist) * Math.abs(amt), i = (y * w + x) * 4;
                if (amt > 0) { const m = 1 - f; d[i] *= m; d[i + 1] *= m; d[i + 2] *= m; } else { d[i] += (255 - d[i]) * f; d[i + 1] += (255 - d[i + 1]) * f; d[i + 2] += (255 - d[i + 2]) * f; }
            }
        }
        return img;
    }

    def('vignette', { amount: 60, size: 50, roundness: 0, feather: 60 }, (img, p) => vignetteImage(img, num(p.amount, 60), num(p.size, 50), num(p.roundness, 0), num(p.feather, 60)));

    // result = screen(base, layer) where layer is another RGBA array scaled by k.
    function screenAdd(d, layer, k) {
        for (let i = 0; i < d.length; i += 4) {
            for (let ch = 0; ch < 3; ch++) { const a = c255(layer[i + ch] * k); d[i + ch] = 255 - (255 - d[i + ch]) * (255 - a) / 255; }
        }
    }

    def('bloom', { threshold: 140, radius: 14, intensity: 60 }, (img, p, c) => {
        const thr = clamp(num(p.threshold, 140), 0, 254), bright = cloneImage(img), b = bright.data;
        for (let i = 0; i < b.length; i += 4) { const wgt = smooth(thr, 255, lumaOf(b[i], b[i + 1], b[i + 2])); b[i] *= wgt; b[i + 1] *= wgt; b[i + 2] *= wgt; }
        screenAdd(img.data, gaussBlurImage(bright, num(p.radius, 14) * c.scale).data, num(p.intensity, 60) / 50);
        return img;
    });

    def('glow', { radius: 10, intensity: 55 }, (img, p, c) => {
        screenAdd(img.data, gaussBlurImage(img, num(p.radius, 10) * c.scale).data, num(p.intensity, 55) / 100 * 1.15);
        return img;
    });

    def('light-leak', { color: '#ff7a1a', position: { x: 0.92, y: 0.12 }, size: 60, intensity: 60, angle: 35, stretch: 2 }, (img, p) => {
        const w = img.width, h = img.height, d = img.data, col = hexRGB(p.color, [255, 122, 26]), pos = pointOf(p.position, 0.92, 0.12);
        const px = pos.x * (w - 1), py = pos.y * (h - 1), sigma = Math.max(2, clamp(num(p.size, 60), 5, 200) / 100 * Math.max(w, h) * 0.45);
        const a = num(p.angle, 35) * PI / 180, cs = Math.cos(a), sn = Math.sin(a), st = Math.max(1, num(p.stretch, 2)), k = clamp(num(p.intensity, 60), 0, 100) / 100 * 1.25;
        for (let y = 0; y < h; y++) {
            for (let x = 0; x < w; x++) {
                const dx = x - px, dy = y - py, u = (dx * cs + dy * sn) / (sigma * st), v = (-dx * sn + dy * cs) / sigma;
                const m = Math.exp(-(u * u + v * v) * 1.4);
                if (m < 0.002) continue;
                const hot = m * m * m, i = (y * w + x) * 4;
                for (let ch = 0; ch < 3; ch++) {
                    const add = c255((col[ch] * (1 - hot) + 255 * hot) * m * k);
                    d[i + ch] = 255 - (255 - d[i + ch]) * (255 - add) / 255;
                }
            }
        }
        return img;
    });

    def('lens-flare', { position: { x: 0.3, y: 0.3 }, brightness: 100, ghosts: 5, rays: true }, (img, p) => {
        const w = img.width, h = img.height, d = img.data, pos = pointOf(p.position, 0.3, 0.3);
        const lx = pos.x * (w - 1), ly = pos.y * (h - 1), cx = (w - 1) / 2, cy = (h - 1) / 2, diag = hyp(w, h), k = clamp(num(p.brightness, 100), 10, 300) / 100;
        const T = [0.4, 0.75, 1.15, 1.5, 1.9, 2.4, -0.3, 0.2], R = [0.035, 0.06, 0.03, 0.08, 0.045, 0.1, 0.025, 0.05];
        const TINT = [[255, 190, 120], [140, 200, 255], [255, 150, 200], [170, 255, 190], [255, 220, 150], [150, 170, 255], [255, 255, 255], [255, 170, 100]];
        const ghosts = [];
        for (let i = 0; i < clamp(Math.round(num(p.ghosts, 5)), 0, 8); i++) ghosts.push({ x: lx + (cx - lx) * T[i], y: ly + (cy - ly) * T[i], r: diag * R[i], tint: TINT[i] });
        const rays = p.rays !== false, add = [0, 0, 0];
        for (let y = 0; y < h; y++) {
            for (let x = 0; x < w; x++) {
                const dx = x - lx, dy = y - ly, dist = hyp(dx, dy);
                let core = Math.exp(-Math.pow(dist / (diag * 0.04), 2)) + 0.45 * Math.exp(-Math.pow(dist / (diag * 0.16), 2)) + 0.16 * Math.exp(-dist / (diag * 0.45));
                if (rays) {
                    const a = Math.atan2(dy, dx);
                    core += (Math.pow(Math.abs(Math.cos(a * 5 + 0.3)), 24) * 0.6 + Math.pow(Math.abs(Math.cos(a * 9 + 1)), 40) * 0.4) * Math.exp(-dist / (diag * 0.22)) * 0.8;
                }
                add[0] = 255 * core; add[1] = 238 * core; add[2] = 214 * core;
                for (let g = 0; g < ghosts.length; g++) {
                    const gh = ghosts[g], dg = hyp(x - gh.x, y - gh.y);
                    if (dg > gh.r * 1.2) continue;
                    const ring = smooth(gh.r * 0.7, gh.r, dg) * (1 - smooth(gh.r, gh.r * 1.12, dg)), fill = (1 - smooth(0, gh.r, dg)) * 0.18, v = (ring * 0.55 + fill) * 0.7;
                    add[0] += gh.tint[0] * v; add[1] += gh.tint[1] * v; add[2] += gh.tint[2] * v;
                }
                const i = (y * w + x) * 4;
                for (let ch = 0; ch < 3; ch++) d[i + ch] = 255 - (255 - d[i + ch]) * (255 - c255(add[ch] * k)) / 255;
            }
        }
        return img;
    });

    def('aberration', { mode: 'lateral', amount: 3, angle: 0 }, (img, p, c) => {
        const w = img.width, h = img.height, src = img.data, out = new Uint8ClampedArray(src.length), amt = num(p.amount, 3) * c.scale;
        const radial = p.mode === 'radial', a = num(p.angle, 0) * PI / 180, ax = Math.cos(a) * amt, ay = Math.sin(a) * amt;
        const cx = (w - 1) / 2, cy = (h - 1) / 2, k = amt / (hyp(cx, cy) || 1), px = [0, 0, 0, 0];
        for (let y = 0; y < h; y++) {
            for (let x = 0; x < w; x++) {
                const i = (y * w + x) * 4;
                let rx = x + ax, ry = y + ay, bx = x - ax, by = y - ay;
                if (radial) { rx = cx + (x - cx) * (1 + k); ry = cy + (y - cy) * (1 + k); bx = cx + (x - cx) * (1 - k); by = cy + (y - cy) * (1 - k); }
                sampleRGBA(src, w, h, rx, ry, px, 0); out[i] = px[0];
                sampleRGBA(src, w, h, bx, by, px, 0); out[i + 2] = px[2];
                out[i + 1] = src[i + 1]; out[i + 3] = src[i + 3];
            }
        }
        img.data.set(out);
        return img;
    });

    def('tilt-shift', { position: 50, size: 20, blur: 8, angle: 0, saturation: 25 }, (img, p, c) => {
        const w = img.width, h = img.height, d = img.data, sigma = num(p.blur, 8) * c.scale;
        const b1 = sigma > 0.25 ? gaussBlurImage(img, sigma).data : d, b2 = sigma > 0.25 ? gaussBlurImage(img, sigma * 0.4).data : d, sat = 1 + num(p.saturation, 25) / 100;
        const a = num(p.angle, 0) * PI / 180, nx = -Math.sin(a), ny = Math.cos(a), cx = (w - 1) / 2, cy = (h - 1) / 2;
        const half = 0.5 * (Math.abs(w * nx) + Math.abs(h * ny)) || 1, mid = (clamp(num(p.position, 50), 0, 100) / 100 - 0.5) * 2 * half;
        const band = clamp(num(p.size, 20), 0, 100) / 100 * half, feather = half * 0.55;
        const out = new Uint8ClampedArray(d.length);
        for (let y = 0; y < h; y++) {
            for (let x = 0; x < w; x++) {
                const i = (y * w + x) * 4, dist = Math.abs((x - cx) * nx + (y - cy) * ny - mid), m = clamp((dist - band) / feather, 0, 1);
                for (let ch = 0; ch < 3; ch++) out[i + ch] = m < 0.5 ? lerp(d[i + ch], b2[i + ch], m * 2) : lerp(b2[i + ch], b1[i + ch], (m - 0.5) * 2);
                out[i + 3] = d[i + 3];
                if (sat !== 1) { const yy = lumaOf(out[i], out[i + 1], out[i + 2]); out[i] = yy + (out[i] - yy) * sat; out[i + 1] = yy + (out[i + 1] - yy) * sat; out[i + 2] = yy + (out[i + 2] - yy) * sat; }
            }
        }
        img.data.set(out);
        return img;
    });

    // --- retro ---

    function sepiaPixels(d, amount) {
        for (let i = 0; i < d.length; i += 4) {
            const r = d[i], g = d[i + 1], b = d[i + 2];
            d[i] = lerp(r, 0.393 * r + 0.769 * g + 0.189 * b, amount);
            d[i + 1] = lerp(g, 0.349 * r + 0.686 * g + 0.168 * b, amount);
            d[i + 2] = lerp(b, 0.272 * r + 0.534 * g + 0.131 * b, amount);
        }
    }

    def('sepia', { amount: 100 }, (img, p) => { sepiaPixels(img.data, clamp(num(p.amount, 100) / 100, 0, 1)); return img; });

    def('vintage', { sepia: 60, fade: 20, brightness: -10, vignette: 25, grain: 12, seed: 1 }, (img, p, c) => {
        const d = img.data, cf = 1 - clamp(num(p.fade, 20), 0, 100) / 100, bf = 1 + num(p.brightness, -10) / 100;
        sepiaPixels(d, clamp(num(p.sepia, 60) / 100, 0, 1));
        for (let i = 0; i < d.length; i += 4) { d[i] = ((d[i] - 127.5) * cf + 127.5) * bf; d[i + 1] = ((d[i + 1] - 127.5) * cf + 127.5) * bf; d[i + 2] = ((d[i + 2] - 127.5) * cf + 127.5) * bf; }
        vignetteImage(img, num(p.vignette, 25), 45, 0, 70);
        const g = num(p.grain, 12);
        return g > 0 ? applyFilter(img, 'grain', { amount: g, size: 1.4, mono: true, seed: num(p.seed, 1) }, { scale: c.scale }) : img;
    });

    def('fade', { desaturate: 35, lift: 15, brightness: 12 }, (img, p) => {
        const sat = 1 - clamp(num(p.desaturate, 35), 0, 100) / 100, lift = clamp(num(p.lift, 15), 0, 100) / 100 * 70, bf = 1 + num(p.brightness, 12) / 100, d = img.data;
        for (let i = 0; i < d.length; i += 4) {
            const y = lumaOf(d[i], d[i + 1], d[i + 2]);
            for (let ch = 0; ch < 3; ch++) d[i + ch] = (lift + (y + (d[i + ch] - y) * sat) * (255 - lift) / 255) * bf;
        }
        return img;
    });

    def('cyanotype', { contrast: 0, brightness: 0 }, (img, p) => {
        const cf = 1 + num(p.contrast, 0) / 100, bo = num(p.brightness, 0) / 100, d = img.data;
        for (let i = 0; i < d.length; i += 4) {
            const l = clamp(((lumaOf(d[i], d[i + 1], d[i + 2]) / 255 - 0.5) * cf + 0.5) + bo, 0, 1);
            d[i] = 20 + l * 60; d[i + 1] = 40 + l * 100; d[i + 2] = 80 + l * 140;
        }
        return img;
    });

    def('duotone', { shadow: '#14143c', highlight: '#ffd27f', balance: 0 }, (img, p) => {
        const s = hexRGB(p.shadow, [20, 20, 60]), hl = hexRGB(p.highlight, [255, 210, 127]), bal = num(p.balance, 0) / 200, d = img.data;
        for (let i = 0; i < d.length; i += 4) {
            const l = clamp(lumaOf(d[i], d[i + 1], d[i + 2]) / 255 + bal, 0, 1);
            d[i] = s[0] + (hl[0] - s[0]) * l; d[i + 1] = s[1] + (hl[1] - s[1]) * l; d[i + 2] = s[2] + (hl[2] - s[2]) * l;
        }
        return img;
    });

    def('solarize', { threshold: 128, mode: 'shadows' }, (img, p) => {
        const t = num(p.threshold, 128), high = p.mode === 'highlights', lut = new Uint8ClampedArray(256);
        for (let i = 0; i < 256; i++) lut[i] = (high ? i > t : i < t) ? 255 - i : i;
        return applyLut(img, lut, lut, lut);
    });

    const CROSS_CURVES = {
        classic: [[[0, 0], [64, 40], [128, 128], [192, 226], [255, 255]], [[0, 0], [64, 72], [128, 140], [192, 205], [255, 255]], [[0, 38], [64, 82], [128, 122], [192, 168], [255, 212]]],
        cool: [[[0, 0], [64, 52], [128, 124], [192, 205], [255, 245]], [[0, 10], [64, 78], [128, 136], [192, 200], [255, 250]], [[0, 30], [64, 92], [128, 150], [192, 212], [255, 255]]],
        warm: [[[0, 12], [64, 76], [128, 146], [192, 214], [255, 255]], [[0, 0], [64, 62], [128, 124], [192, 192], [255, 245]], [[0, 0], [64, 48], [128, 104], [192, 160], [255, 200]]]
    };
    def('cross-process', { amount: 70, style: 'classic' }, (img, p) => {
        const a = clamp(num(p.amount, 70), 0, 100) / 100, curves = CROSS_CURVES[p.style] || CROSS_CURVES.classic, id = identityLut();
        const luts = curves.map(points => { const l = splineLut(points), mixed = new Uint8ClampedArray(256); for (let i = 0; i < 256; i++) mixed[i] = lerp(id[i], l[i], a); return mixed; });
        return applyLut(img, luts[0], luts[1], luts[2]);
    });

    def('vhs', { scanlines: 40, rgb_shift: 4, noise: 25, smear: 2, tracking: 20, seed: 1 }, (img, p, c) => {
        const w = img.width, h = img.height, rnd = mulberry32(num(p.seed, 1)), tracking = clamp(num(p.tracking, 20), 0, 100) / 100;
        let d = img.data;
        const smear = num(p.smear, 2) * c.scale;
        if (smear > 0.3) {
            const b = gaussBlurImage(img, smear, 0).data;
            for (let i = 0; i < d.length; i += 4) {
                const y = lumaOf(d[i], d[i + 1], d[i + 2]), yb = lumaOf(b[i], b[i + 1], b[i + 2]);
                d[i] = y + (b[i] - yb); d[i + 1] = y + (b[i + 1] - yb); d[i + 2] = y + (b[i + 2] - yb);
            }
        }
        const shift = Math.round(num(p.rgb_shift, 4) * c.scale), src = new Uint8ClampedArray(d), out = d;
        const bands = [];
        for (let k = 0, n = Math.round(tracking * 5); k < n; k++) bands.push({ y0: Math.floor(rnd() * h), bh: Math.max(2, Math.floor((0.01 + rnd() * 0.06) * h)), off: (rnd() * 2 - 1) * tracking * w * 0.06 });
        const noise = clamp(num(p.noise, 25), 0, 100) / 100 * 60, period = Math.max(2, Math.round(3 * c.scale)), dark = 1 - clamp(num(p.scanlines, 40), 0, 100) / 100 * 0.4;
        for (let y = 0; y < h; y++) {
            let off = 0, streak = 0;
            for (let k = 0; k < bands.length; k++) { const b = bands[k]; if (y >= b.y0 && y < b.y0 + b.bh) { off = Math.round(b.off * (1 + 0.3 * Math.sin(y * 0.9))); streak = noise * 1.6; } }
            const rowDark = (y % period) < period / 2 ? 1 : dark;
            for (let x = 0; x < w; x++) {
                const xs = clamp(x - off, 0, w - 1), i = (y * w + x) * 4;
                const ir = (y * w + clamp(xs - shift, 0, w - 1)) * 4, ib = (y * w + clamp(xs + shift, 0, w - 1)) * 4, ig = (y * w + xs) * 4;
                const n = (rnd() - 0.5) * 2 * (noise + streak * rnd());
                out[i] = (src[ir] + n) * rowDark; out[i + 1] = (src[ig + 1] + n) * rowDark; out[i + 2] = (src[ib + 2] + n) * rowDark; out[i + 3] = src[ig + 3];
            }
        }
        return img;
    });

    def('crt', { curvature: 30, scanlines: 40, line_size: 3, mask: 'aperture', mask_amount: 40, glow: 30 }, (img, p, c) => {
        const w = img.width, h = img.height, src = img.data, out = new Uint8ClampedArray(src.length), cx = (w - 1) / 2, cy = (h - 1) / 2, px = [0, 0, 0, 0];
        const k = clamp(num(p.curvature, 30), 0, 100) / 100 * 0.3;
        for (let y = 0; y < h; y++) {
            for (let x = 0; x < w; x++) {
                const nx = (x - cx) / (cx || 1), ny = (y - cy) / (cy || 1), f = 1 + k * (nx * nx + ny * ny);
                const sx = cx + (x - cx) * f, sy = cy + (y - cy) * f, i = (y * w + x) * 4;
                out[i + 3] = 255;
                if (sx < 0 || sx > w - 1 || sy < 0 || sy > h - 1) continue;
                sampleRGBA(src, w, h, sx, sy, px, 0);
                const ex = Math.abs((sx - cx) / (cx || 1)), ey = Math.abs((sy - cy) / (cy || 1)), edge = 1 - smooth(0.92, 1, Math.max(ex, ey)), vig = 1 - 0.22 * (ex * ex + ey * ey);
                out[i] = px[0] * edge * vig; out[i + 1] = px[1] * edge * vig; out[i + 2] = px[2] * edge * vig;
            }
        }
        const glow = clamp(num(p.glow, 30), 0, 100) / 100;
        if (glow > 0) screenAdd(out, gaussBlurImage({ width: w, height: h, data: new Uint8ClampedArray(out) }, Math.max(1, Math.min(w, h) * 0.005)).data, glow * 0.7);
        const period = Math.max(2, Math.round(num(p.line_size, 3) * c.scale)), sl = clamp(num(p.scanlines, 40), 0, 100) / 100 * 0.6;
        const mask = p.mask === 'none' ? 0 : clamp(num(p.mask_amount, 40), 0, 100) / 100 * 0.6, shadow = p.mask === 'shadow';
        for (let y = 0; y < h; y++) {
            const line = 1 - sl * (0.5 - 0.5 * Math.cos(TAU * ((y % period) / period)));
            const row = shadow && (Math.floor(y / period) & 1) ? 1 : 0;
            for (let x = 0; x < w; x++) {
                const i = (y * w + x) * 4, tri = (x + row) % 3;
                for (let ch = 0; ch < 3; ch++) out[i + ch] = out[i + ch] * line * (mask && ch !== tri ? 1 - mask : 1 + mask * 0.35);
            }
        }
        img.data.set(out);
        return img;
    });

    def('glitch', { amount: 40, shift: 40, rgb_split: 6, seed: 1 }, (img, p, c) => {
        const w = img.width, h = img.height, rnd = mulberry32(num(p.seed, 1)), amount = clamp(num(p.amount, 40), 0, 100) / 100, shiftMax = num(p.shift, 40) * c.scale;
        const split = Math.round(num(p.rgb_split, 6) * c.scale), src = new Uint8ClampedArray(img.data), d = img.data;
        const wrap = x => ((x % w) + w) % w;
        // whole-image colour split first
        if (split) {
            for (let y = 0; y < h; y++) {
                for (let x = 0; x < w; x++) {
                    const i = (y * w + x) * 4;
                    d[i] = src[(y * w + wrap(x - split)) * 4]; d[i + 2] = src[(y * w + wrap(x + split)) * 4 + 2];
                }
            }
            src.set(d);
        }
        const bands = Math.round(2 + amount * 18);
        for (let b = 0; b < bands; b++) {
            const y0 = Math.floor(rnd() * h), bh = Math.max(1, Math.floor((0.004 + rnd() * 0.07) * h)), off = Math.round((rnd() * 2 - 1) * shiftMax), extra = rnd() < 0.5 ? Math.round((rnd() * 2 - 1) * split * 2) : 0;
            for (let y = y0; y < Math.min(h, y0 + bh); y++) {
                for (let x = 0; x < w; x++) {
                    const i = (y * w + x) * 4, s = (y * w + wrap(x - off)) * 4;
                    d[i] = src[(y * w + wrap(x - off - extra)) * 4]; d[i + 1] = src[s + 1]; d[i + 2] = src[(y * w + wrap(x - off + extra)) * 4 + 2]; d[i + 3] = src[s + 3];
                }
            }
        }
        const blocks = Math.round(amount * 6);
        for (let b = 0; b < blocks; b++) {
            const bw = Math.max(2, Math.floor((0.03 + rnd() * 0.2) * w)), bh = Math.max(2, Math.floor((0.01 + rnd() * 0.08) * h));
            const sx = Math.floor(rnd() * Math.max(1, w - bw)), sy = Math.floor(rnd() * Math.max(1, h - bh)), dx = Math.floor(rnd() * Math.max(1, w - bw)), dy = Math.floor(rnd() * Math.max(1, h - bh));
            for (let y = 0; y < bh; y++) for (let x = 0; x < bw; x++) {
                const s = ((sy + y) * w + sx + x) * 4, t = ((dy + y) * w + dx + x) * 4;
                d[t] = src[s]; d[t + 1] = src[s + 1]; d[t + 2] = src[s + 2]; d[t + 3] = src[s + 3];
            }
        }
        return img;
    });

    // --- the adjust panel's combined filter ---
    // All sliders are -100..100 (0 = untouched). Order: tone curve (blacks/whites, exposure in linear light, brightness,
    // contrast) -> white balance -> highlights/shadows -> saturation/vibrance -> dehaze -> clarity -> sharpness -> vignette.

    const ADJUST_KEYS = ['exposure', 'brightness', 'contrast', 'highlights', 'shadows', 'whites', 'blacks', 'temperature', 'tint', 'vibrance', 'saturation', 'clarity', 'dehaze', 'sharpness', 'vignette'];
    const ADJUST_DEFAULTS = {};
    ADJUST_KEYS.forEach(key => { ADJUST_DEFAULTS[key] = 0; });

    function adjustHasEffect(p) {
        return ADJUST_KEYS.some(key => num(p && p[key], 0) !== 0);
    }

    function dehaze(img, amount) {
        const w = img.width, h = img.height, d = img.data, A = 235;
        if (amount > 0) {
            const k = amount / 100 * 0.8, dark = new Float32Array(w * h);
            for (let q = 0, i = 0; q < dark.length; q++, i += 4) dark[q] = Math.min(d[i], d[i + 1], d[i + 2]);
            const blurred = blurPlane(dark, w, h, Math.max(1.5, 0.01 * Math.max(w, h)));
            for (let q = 0, i = 0; q < dark.length; q++, i += 4) {
                const t = Math.max(0.35, 1 - 0.95 * k * (blurred[q] / A));
                d[i] = (d[i] - A) / t + A; d[i + 1] = (d[i + 1] - A) / t + A; d[i + 2] = (d[i + 2] - A) / t + A;
            }
        } else if (amount < 0) {
            const k = -amount / 100 * 0.35;
            for (let i = 0; i < d.length; i += 4) { d[i] = d[i] * (1 - k) + A * k; d[i + 1] = d[i + 1] * (1 - k) + A * k; d[i + 2] = d[i + 2] * (1 - k) + A * k; }
        }
        return img;
    }

    def('adjust', ADJUST_DEFAULTS, (img, p) => {
        const v = key => clamp(num(p[key], 0), -100, 100);
        const e = v('exposure'), b = v('brightness'), ct = v('contrast'), hi = v('highlights'), sh = v('shadows'), wh = v('whites'), bl = v('blacks');
        const temp = v('temperature') / 100, tint = v('tint') / 100, vib = v('vibrance') / 100, sat = v('saturation') / 100;
        const d = img.data, w = img.width, h = img.height;
        let lut = null;
        if (e || b || ct || wh || bl) {
            const bp = -bl / 100 * 0.25, wp = 1 - wh / 100 * 0.25, ev = Math.pow(2, e / 50);
            const cf = ct >= 0 ? 1 + ct / 100 * 1.2 : 1 + ct / 100 * 0.95;
            lut = makeLut(t => {
                let x = (t - bp) / (wp - bp);
                if (e) x = Math.pow(Math.pow(clamp(x, 0, 1), 2.2) * ev, 1 / 2.2);
                if (b) { const c = clamp(x, 0, 1); x += b / 100 * 1.4 * c * (1 - c); }
                if (ct) x = (x - 0.5) * cf + 0.5;
                return x;
            });
        }
        const rm = 1 + 0.25 * temp + 0.06 * tint, gm = 1 - 0.2 * tint, bm = 1 - 0.25 * temp + 0.06 * tint, balance = temp !== 0 || tint !== 0;
        let toneAdd = null;
        if (hi || sh) {
            toneAdd = new Float32Array(256);
            for (let i = 0; i < 256; i++) { const t = i / 255; toneAdd[i] = hi / 100 * 64 * smooth(0.4, 1, t) + sh / 100 * 64 * (1 - smooth(0, 0.6, t)); }
        }
        const color = vib !== 0 || sat !== 0, out = [0, 0, 0];
        if (lut || balance || toneAdd || color) {
            for (let i = 0; i < d.length; i += 4) {
                let r = d[i], g = d[i + 1], bb = d[i + 2];
                if (lut) { r = lut[r]; g = lut[g]; bb = lut[bb]; }
                if (balance) { r *= rm; g *= gm; bb *= bm; }
                if (toneAdd) { const a = toneAdd[Math.round(c255(lumaOf(r, g, bb)))]; r += a; g += a; bb += a; }
                if (color) { vibranceRGB(c255(r), c255(g), c255(bb), vib, sat, out); r = out[0]; g = out[1]; bb = out[2]; }
                d[i] = r; d[i + 1] = g; d[i + 2] = bb;
            }
        }
        const long = Math.max(w, h);
        let res = img;
        if (v('dehaze')) res = dehaze(res, v('dehaze'));
        if (v('clarity')) res = localContrast(res, Math.max(2, 0.012 * long), v('clarity') / 100);
        if (v('sharpness')) res = unsharp(res, clamp(0.0007 * long, 0.6, 1.8), v('sharpness') / 100 * 1.5, 0, true);
        if (v('vignette')) res = vignetteImage(res, -v('vignette'), 45, 0, 70);
        return res;
    });

    // --- fade / compose ---
    // fade: { mode, strength 0..100 }. mask: optional Uint8ClampedArray(w * h), 255 = the filtered result fully visible.

    const FADE_MODES = ['normal', 'multiply', 'screen', 'overlay', 'soft-light', 'hard-light', 'darken', 'lighten', 'difference', 'color', 'luminosity'];

    function softLight(cb, cs) {
        if (cs <= 0.5) return cb - (1 - 2 * cs) * cb * (1 - cb);
        const dd = cb <= 0.25 ? ((16 * cb - 12) * cb + 4) * cb : Math.sqrt(cb);
        return cb + (2 * cs - 1) * (dd - cb);
    }

    const hardLight = (cb, cs) => (cs <= 0.5 ? cb * 2 * cs : cb + (2 * cs - 1) - cb * (2 * cs - 1));

    function blendSeparable(mode, cb, cs) {
        switch (mode) {
            case 'multiply': return cb * cs;
            case 'screen': return cb + cs - cb * cs;
            case 'overlay': return hardLight(cs, cb);
            case 'soft-light': return softLight(cb, cs);
            case 'hard-light': return hardLight(cb, cs);
            case 'darken': return cb < cs ? cb : cs;
            case 'lighten': return cb > cs ? cb : cs;
            case 'difference': return Math.abs(cb - cs);
            default: return cs;
        }
    }

    // W3C non-separable helpers on 0..1 triples.
    function setLum(c, l, out) {
        const dd = l - (0.3 * c[0] + 0.59 * c[1] + 0.11 * c[2]);
        let r = c[0] + dd, g = c[1] + dd, b = c[2] + dd;
        const lum = 0.3 * r + 0.59 * g + 0.11 * b, n = Math.min(r, g, b), x = Math.max(r, g, b);
        if (n < 0) { const k = lum / (lum - n); r = lum + (r - lum) * k; g = lum + (g - lum) * k; b = lum + (b - lum) * k; }
        if (x > 1) { const k = (1 - lum) / (x - lum); r = lum + (r - lum) * k; g = lum + (g - lum) * k; b = lum + (b - lum) * k; }
        out[0] = r; out[1] = g; out[2] = b;
    }

    function composeImages(base, top, fade, mask) {
        const mode = fade && FADE_MODES.indexOf(fade.mode) >= 0 ? fade.mode : 'normal';
        const s = fade ? clamp(num(fade.strength, 100) / 100, 0, 1) : 1;
        if (!mask && mode === 'normal' && s >= 1) return top;
        const w = base.width, h = base.height, b = base.data, t = top.data, out = blankImage(w, h), d = out.data;
        const cb = [0, 0, 0], cs = [0, 0, 0], tmp = [0, 0, 0];
        const nonSep = mode === 'color' || mode === 'luminosity';
        for (let q = 0, i = 0; q < w * h; q++, i += 4) {
            const m = mask ? mask[q] / 255 : 1;
            if (m <= 0) { d[i] = b[i]; d[i + 1] = b[i + 1]; d[i + 2] = b[i + 2]; d[i + 3] = b[i + 3]; continue; }
            const ab = b[i + 3] / 255, at = t[i + 3] / 255;
            let ar, rr, rg, rb;
            if (mode === 'normal') {
                // exact lerp in premultiplied space
                ar = lerp(ab, at, s);
                if (ar > 0) {
                    const f = 1 / ar;
                    rr = lerp(b[i] * ab, t[i] * at, s) * f; rg = lerp(b[i + 1] * ab, t[i + 1] * at, s) * f; rb = lerp(b[i + 2] * ab, t[i + 2] * at, s) * f;
                } else { rr = rg = rb = 0; }
            } else {
                const as = at * s;
                ar = as + ab * (1 - as);
                if (ar > 0) {
                    cb[0] = b[i] / 255; cb[1] = b[i + 1] / 255; cb[2] = b[i + 2] / 255; cs[0] = t[i] / 255; cs[1] = t[i + 1] / 255; cs[2] = t[i + 2] / 255;
                    let bl0, bl1, bl2;
                    if (nonSep) {
                        if (mode === 'color') setLum(cs, 0.3 * cb[0] + 0.59 * cb[1] + 0.11 * cb[2], tmp); else setLum(cb, 0.3 * cs[0] + 0.59 * cs[1] + 0.11 * cs[2], tmp);
                        bl0 = tmp[0]; bl1 = tmp[1]; bl2 = tmp[2];
                    } else { bl0 = blendSeparable(mode, cb[0], cs[0]); bl1 = blendSeparable(mode, cb[1], cs[1]); bl2 = blendSeparable(mode, cb[2], cs[2]); }
                    const k = as / ar;
                    rr = ((1 - k) * cb[0] + k * ((1 - ab) * cs[0] + ab * bl0)) * 255;
                    rg = ((1 - k) * cb[1] + k * ((1 - ab) * cs[1] + ab * bl1)) * 255;
                    rb = ((1 - k) * cb[2] + k * ((1 - ab) * cs[2] + ab * bl2)) * 255;
                } else { rr = rg = rb = 0; }
            }
            if (m >= 1) { d[i] = rr; d[i + 1] = rg; d[i + 2] = rb; d[i + 3] = ar * 255; continue; }
            const am = lerp(ab, ar, m);
            if (am > 0) {
                const f = 1 / am;
                d[i] = lerp(b[i] * ab, rr * ar, m) * f; d[i + 1] = lerp(b[i + 1] * ab, rg * ar, m) * f; d[i + 2] = lerp(b[i + 2] * ab, rb * ar, m) * f;
            }
            d[i + 3] = am * 255;
        }
        return out;
    }

    // --- analysis ---

    // 256 bin histograms of the pixels with alpha > 0: { r, g, b, l, count }.
    function histogramOf(img) {
        const d = img.data, r = new Uint32Array(256), g = new Uint32Array(256), b = new Uint32Array(256), l = new Uint32Array(256);
        let count = 0;
        for (let i = 0; i < d.length; i += 4) {
            if (d[i + 3] === 0) continue;
            r[d[i]]++; g[d[i + 1]]++; b[d[i + 2]]++; l[Math.round(lumaOf(d[i], d[i + 1], d[i + 2]))]++;
            count++;
        }
        return { r, g, b, l, count };
    }

    // Suggested adjust sliders: stretch the histogram (whites/blacks), centre the brightness (exposure), a little contrast,
    // neutralise the colour cast (temperature/tint) and add mild vibrance. Only the keys it changes are returned.
    function autoTone(img) {
        const lh = lumaHistogram(img);
        const out = {};
        if (!lh.count) return out;
        const range = percentileRange(lh, 0.4), lo = range.lo, hi = range.hi;
        const blacks = clamp(-lo / 64 * 100, -100, 0), whites = clamp((255 - hi) / 64 * 100, 0, 100);
        if (Math.round(blacks)) out.blacks = Math.round(blacks);
        if (Math.round(whites)) out.whites = Math.round(whites);
        let sum = 0;
        for (let i = 0; i < 256; i++) sum += lh.hist[i] * i;
        const mean = sum / lh.count, stretched = clamp((mean - lo) / Math.max(1, hi - lo), 0.02, 0.98), target = 0.46;
        const exposure = clamp(Math.round(110 * Math.log2(target / stretched) * 0.6), -60, 60);
        if (exposure) out.exposure = exposure;
        const spread = (hi - lo) / 255;
        out.contrast = spread > 0.92 ? 6 : 12;
        const gains = grayWorldGains(img);
        const k = (gains[0] - gains[2]) / (gains[0] + gains[2]);
        const temperature = clamp(Math.round(k * 400 * 0.6), -50, 50);
        if (temperature) out.temperature = temperature;
        const mid = (gains[0] + gains[2]) / 2, tintK = (mid - gains[1]) / Math.max(0.01, mid);
        const tint = clamp(Math.round(tintK * 500 * 0.5), -40, 40);
        if (tint) out.tint = tint;
        out.vibrance = 15;
        return out;
    }

    // --- ops ---

    function renderOp(args, ctx) {
        const base = args.image;
        const hooks = { scale: args.scale, progress: ctx && ctx.progress, cancelled: ctx && ctx.cancelled };
        const raw = applyFilter(base, args.id, args.params, hooks);
        const out = args.fade || args.mask ? composeImages(base, raw, args.fade, args.mask) : raw;
        return { image: out, raw: args.keepRaw ? raw : null };
    }

    Lib.registerOp('fx.apply', (args, ctx) => ({ image: applyFilter(args.image, args.id, args.params, { scale: args.scale, progress: ctx.progress, cancelled: ctx.cancelled }) }));
    Lib.registerOp('fx.render', renderOp);
    Lib.registerOp('fx.compose', args => ({ image: composeImages(args.base, args.raw, args.fade, args.mask) }));
    Lib.registerOp('fx.histogram', args => {
        const img = args.id ? applyFilter(args.image, args.id, args.params, { scale: args.scale }) : args.image;
        const hist = histogramOf(img);
        return { r: hist.r, g: hist.g, b: hist.b, l: hist.l, count: hist.count };
    });
    Lib.registerOp('fx.autoTone', args => ({ params: autoTone(args.image) }));

    Lib.fx = {
        filters: FILTERS,
        ids: () => Object.keys(FILTERS),
        defaults: defaultsFor,
        apply: applyFilter,
        compose: composeImages,
        histogram: histogramOf,
        autoTone,
        splineLut,
        medianCut,
        render: renderOp,
        adjustKeys: ADJUST_KEYS.slice(),
        adjustHasEffect,
        fadeModes: FADE_MODES.slice(),
        paletteIds: PALETTE_IDS.slice()
    };
})(typeof self !== 'undefined' ? self : (typeof globalThis !== 'undefined' ? globalThis : this));
