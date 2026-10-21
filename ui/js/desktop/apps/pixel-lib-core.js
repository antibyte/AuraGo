// Pixel pure image library, core part. DOM-free: loads in a window, in the Pixel worker and in a node vm.
// Image format everywhere: { width, height, data: Uint8ClampedArray RGBA }. Masks: Uint8ClampedArray(width * height).
// Other pixel-lib-*.js files add ops with AuraPixelLib.registerOp(name, fn); fn(args, ctx) gets ctx.progress(fraction)
// and ctx.cancelled() and returns its result (plain object, typed arrays are transferred automatically).
(function (root) {
    'use strict';

    const Lib = root.AuraPixelLib = root.AuraPixelLib || {};
    const ops = Lib.ops = Lib.ops || Object.create(null);

    // --- numbers ---

    function clamp(v, lo, hi) {
        return v < lo ? lo : (v > hi ? hi : v);
    }

    function clamp255(v) {
        return v <= 0 ? 0 : (v >= 255 ? 255 : Math.round(v));
    }

    // Deterministic PRNG, returns floats in [0, 1).
    function mulberry32(seed) {
        let a = seed >>> 0;
        return function next() {
            a = (a + 0x6D2B79F5) >>> 0;
            let t = a;
            t = Math.imul(t ^ (t >>> 15), t | 1);
            t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
            return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
        };
    }

    // String (or number) -> unsigned 32-bit seed (FNV-1a with a final avalanche).
    function hashSeed(value) {
        const text = String(value);
        let h = 0x811C9DC5;
        for (let i = 0; i < text.length; i++) {
            h ^= text.charCodeAt(i);
            h = Math.imul(h, 0x01000193);
        }
        h ^= h >>> 16;
        h = Math.imul(h, 0x85EBCA6B);
        h ^= h >>> 13;
        h = Math.imul(h, 0xC2B2AE35);
        h ^= h >>> 16;
        return h >>> 0;
    }

    // Rec. 601 weights on gamma-encoded values.
    function luma(r, g, b) {
        return 0.299 * r + 0.587 * g + 0.114 * b;
    }

    // --- colour spaces: h in degrees [0, 360), s/l/v in [0, 1], rgb in [0, 255] (floats, not rounded) ---

    function rgbToHsl(r, g, b, out) {
        r /= 255; g /= 255; b /= 255;
        const max = Math.max(r, g, b), min = Math.min(r, g, b);
        const l = (max + min) / 2;
        let h = 0, s = 0;
        const d = max - min;
        if (d > 0) {
            s = l > 0.5 ? d / (2 - max - min) : d / (max + min);
            if (max === r) h = (g - b) / d + (g < b ? 6 : 0);
            else if (max === g) h = (b - r) / d + 2;
            else h = (r - g) / d + 4;
            h *= 60;
        }
        const res = out || [0, 0, 0];
        res[0] = h; res[1] = s; res[2] = l;
        return res;
    }

    function hueChannel(p, q, t) {
        if (t < 0) t += 1;
        if (t > 1) t -= 1;
        if (t < 1 / 6) return p + (q - p) * 6 * t;
        if (t < 1 / 2) return q;
        if (t < 2 / 3) return p + (q - p) * (2 / 3 - t) * 6;
        return p;
    }

    function hslToRgb(h, s, l, out) {
        const res = out || [0, 0, 0];
        h = ((h % 360) + 360) % 360 / 360;
        if (s <= 0) {
            res[0] = res[1] = res[2] = l * 255;
            return res;
        }
        const q = l < 0.5 ? l * (1 + s) : l + s - l * s;
        const p = 2 * l - q;
        res[0] = hueChannel(p, q, h + 1 / 3) * 255;
        res[1] = hueChannel(p, q, h) * 255;
        res[2] = hueChannel(p, q, h - 1 / 3) * 255;
        return res;
    }

    function rgbToHsv(r, g, b, out) {
        r /= 255; g /= 255; b /= 255;
        const max = Math.max(r, g, b), min = Math.min(r, g, b);
        const d = max - min;
        let h = 0;
        if (d > 0) {
            if (max === r) h = (g - b) / d + (g < b ? 6 : 0);
            else if (max === g) h = (b - r) / d + 2;
            else h = (r - g) / d + 4;
            h *= 60;
        }
        const res = out || [0, 0, 0];
        res[0] = h; res[1] = max === 0 ? 0 : d / max; res[2] = max;
        return res;
    }

    function hsvToRgb(h, s, v, out) {
        const res = out || [0, 0, 0];
        h = ((h % 360) + 360) % 360 / 60;
        const i = Math.floor(h), f = h - i;
        const p = v * (1 - s), q = v * (1 - s * f), t = v * (1 - s * (1 - f));
        let r, g, b;
        switch (i % 6) {
            case 0: r = v; g = t; b = p; break;
            case 1: r = q; g = v; b = p; break;
            case 2: r = p; g = v; b = t; break;
            case 3: r = p; g = q; b = v; break;
            case 4: r = t; g = p; b = v; break;
            default: r = v; g = p; b = q;
        }
        res[0] = r * 255; res[1] = g * 255; res[2] = b * 255;
        return res;
    }

    // '#rgb', '#rrggbb' or '#rrggbbaa' -> [r, g, b, a(0..255)]; null for anything else.
    function parseColor(text) {
        const m = /^#?([0-9a-f]{3}|[0-9a-f]{6}|[0-9a-f]{8})$/i.exec(String(text == null ? '' : text).trim());
        if (!m) return null;
        let hex = m[1];
        if (hex.length === 3) hex = hex[0] + hex[0] + hex[1] + hex[1] + hex[2] + hex[2];
        return [parseInt(hex.slice(0, 2), 16), parseInt(hex.slice(2, 4), 16), parseInt(hex.slice(4, 6), 16), hex.length === 8 ? parseInt(hex.slice(6, 8), 16) : 255];
    }

    function toHex(r, g, b) {
        return '#' + [r, g, b].map(v => clamp255(v).toString(16).padStart(2, '0')).join('');
    }

    // --- images ---

    function createImage(width, height) {
        const w = Math.max(0, Math.round(width) || 0), h = Math.max(0, Math.round(height) || 0);
        return { width: w, height: h, data: new Uint8ClampedArray(w * h * 4) };
    }

    // 2x2 area average, alpha weighted so transparent pixels do not darken the colour.
    function halve(img) {
        const sw = img.width, sh = img.height;
        const w = Math.max(1, sw >> 1), h = Math.max(1, sh >> 1);
        const out = createImage(w, h);
        const s = img.data, d = out.data;
        for (let y = 0; y < h; y++) {
            const y0 = Math.min(sh - 1, y * 2), y1 = Math.min(sh - 1, y * 2 + 1);
            for (let x = 0; x < w; x++) {
                const x0 = Math.min(sw - 1, x * 2), x1 = Math.min(sw - 1, x * 2 + 1);
                const i0 = (y0 * sw + x0) * 4, i1 = (y0 * sw + x1) * 4, i2 = (y1 * sw + x0) * 4, i3 = (y1 * sw + x1) * 4;
                const a0 = s[i0 + 3], a1 = s[i1 + 3], a2 = s[i2 + 3], a3 = s[i3 + 3];
                const sa = a0 + a1 + a2 + a3;
                const o = (y * w + x) * 4;
                if (sa > 0) {
                    d[o] = (s[i0] * a0 + s[i1] * a1 + s[i2] * a2 + s[i3] * a3) / sa;
                    d[o + 1] = (s[i0 + 1] * a0 + s[i1 + 1] * a1 + s[i2 + 1] * a2 + s[i3 + 1] * a3) / sa;
                    d[o + 2] = (s[i0 + 2] * a0 + s[i1 + 2] * a1 + s[i2 + 2] * a2 + s[i3 + 2] * a3) / sa;
                }
                d[o + 3] = sa / 4;
            }
        }
        return out;
    }

    // Pixel (i, j) is centred at (i, j); coordinates outside the image are clamped to its edge.
    function sampleBilinear(img, x, y, out) {
        const res = out || [0, 0, 0, 0];
        const w = img.width, h = img.height, d = img.data;
        if (!w || !h) { res[0] = res[1] = res[2] = res[3] = 0; return res; }
        x = clamp(x, 0, w - 1); y = clamp(y, 0, h - 1);
        const x0 = Math.floor(x), y0 = Math.floor(y);
        const x1 = Math.min(w - 1, x0 + 1), y1 = Math.min(h - 1, y0 + 1);
        const tx = x - x0, ty = y - y0;
        const i00 = (y0 * w + x0) * 4, i10 = (y0 * w + x1) * 4, i01 = (y1 * w + x0) * 4, i11 = (y1 * w + x1) * 4;
        const w00 = (1 - tx) * (1 - ty) * d[i00 + 3], w10 = tx * (1 - ty) * d[i10 + 3];
        const w01 = (1 - tx) * ty * d[i01 + 3], w11 = tx * ty * d[i11 + 3];
        const wa = w00 + w10 + w01 + w11;
        if (wa > 0) {
            res[0] = (d[i00] * w00 + d[i10] * w10 + d[i01] * w01 + d[i11] * w11) / wa;
            res[1] = (d[i00 + 1] * w00 + d[i10 + 1] * w10 + d[i01 + 1] * w01 + d[i11 + 1] * w11) / wa;
            res[2] = (d[i00 + 2] * w00 + d[i10 + 2] * w10 + d[i01 + 2] * w01 + d[i11 + 2] * w11) / wa;
        } else {
            res[0] = res[1] = res[2] = 0;
        }
        res[3] = wa;
        return res;
    }

    // Large reductions first halve (area average) so thin features do not alias, then interpolate bilinearly.
    function resizeBilinear(img, width, height) {
        const w = Math.max(1, Math.round(width)), h = Math.max(1, Math.round(height));
        if (!img.width || !img.height) return createImage(w, h);
        let src = img;
        while (src.width >= 2 * w && src.height >= 2 * h) src = halve(src);
        const out = createImage(w, h);
        if (src.width === w && src.height === h) {
            out.data.set(src.data);
            return out;
        }
        const sw = src.width, sh = src.height, s = src.data, d = out.data;
        const xi0 = new Int32Array(w), xi1 = new Int32Array(w), xt = new Float32Array(w);
        for (let x = 0; x < w; x++) {
            const fx = clamp((x + 0.5) * sw / w - 0.5, 0, sw - 1);
            const x0 = Math.floor(fx);
            xi0[x] = x0; xi1[x] = Math.min(sw - 1, x0 + 1); xt[x] = fx - x0;
        }
        for (let y = 0; y < h; y++) {
            const fy = clamp((y + 0.5) * sh / h - 0.5, 0, sh - 1);
            const y0 = Math.floor(fy), y1 = Math.min(sh - 1, y0 + 1), ty = fy - y0;
            const r0 = y0 * sw, r1 = y1 * sw;
            for (let x = 0; x < w; x++) {
                const tx = xt[x];
                const i00 = (r0 + xi0[x]) * 4, i10 = (r0 + xi1[x]) * 4, i01 = (r1 + xi0[x]) * 4, i11 = (r1 + xi1[x]) * 4;
                const w00 = (1 - tx) * (1 - ty) * s[i00 + 3], w10 = tx * (1 - ty) * s[i10 + 3];
                const w01 = (1 - tx) * ty * s[i01 + 3], w11 = tx * ty * s[i11 + 3];
                const wa = w00 + w10 + w01 + w11;
                const o = (y * w + x) * 4;
                if (wa > 0) {
                    d[o] = (s[i00] * w00 + s[i10] * w10 + s[i01] * w01 + s[i11] * w11) / wa;
                    d[o + 1] = (s[i00 + 1] * w00 + s[i10 + 1] * w10 + s[i01 + 1] * w01 + s[i11 + 1] * w11) / wa;
                    d[o + 2] = (s[i00 + 2] * w00 + s[i10 + 2] * w10 + s[i01 + 2] * w01 + s[i11 + 2] * w11) / wa;
                }
                d[o + 3] = wa;
            }
        }
        return out;
    }

    // --- blur ---

    // Normalised 1D gaussian, radius = ceil(3 * sigma).
    function gaussianKernel(sigma) {
        if (!(sigma > 0)) return Float32Array.of(1);
        const radius = Math.max(1, Math.ceil(sigma * 3));
        const kernel = new Float32Array(radius * 2 + 1);
        let sum = 0;
        for (let i = -radius; i <= radius; i++) {
            const v = Math.exp(-(i * i) / (2 * sigma * sigma));
            kernel[i + radius] = v;
            sum += v;
        }
        for (let i = 0; i < kernel.length; i++) kernel[i] /= sum;
        return kernel;
    }

    // Box widths (odd) whose repeated application approximates a gaussian of the given sigma.
    function boxSizesForGauss(sigma, passes) {
        const ideal = Math.sqrt(12 * sigma * sigma / passes + 1);
        let lower = Math.floor(ideal);
        if (lower % 2 === 0) lower--;
        const upper = lower + 2;
        const m = Math.round((12 * sigma * sigma - passes * lower * lower - 4 * passes * lower - 3 * passes) / (-4 * lower - 4));
        const sizes = [];
        for (let i = 0; i < passes; i++) sizes.push(i < m ? lower : upper);
        return sizes;
    }

    // One box blur pass over n samples at src[so + k * stride] -> dst[so + k * stride]; the edge pixel repeats outward.
    function boxLine(src, dst, so, stride, n, r) {
        const inv = 1 / (2 * r + 1);
        const last = n - 1;
        let sum = 0;
        for (let k = -r; k <= r; k++) sum += src[so + (k < 0 ? 0 : (k > last ? last : k)) * stride];
        for (let i = 0; i < n; i++) {
            dst[so + i * stride] = sum * inv;
            const add = i + r + 1, sub = i - r;
            sum += src[so + (add > last ? last : add) * stride] - src[so + (sub < 0 ? 0 : sub) * stride];
        }
    }

    function boxBlurMask(src, w, h, r) {
        if (r < 1) return Uint8ClampedArray.from(src);
        const tmp = new Uint8ClampedArray(src.length);
        const out = new Uint8ClampedArray(src.length);
        for (let y = 0; y < h; y++) boxLine(src, tmp, y * w, 1, w, r);
        for (let x = 0; x < w; x++) boxLine(tmp, out, x, w, h, r);
        return out;
    }

    // Separable gaussian on a mask. Exact kernel for small sigma, three box passes (O(n)) for large sigma.
    function gaussBlurMask(alpha, w, h, sigma) {
        const n = w * h;
        if (!(sigma > 0.05) || !n) return Uint8ClampedArray.from(alpha);
        if (sigma > 6) {
            let cur = alpha;
            boxSizesForGauss(sigma, 3).forEach(size => { cur = boxBlurMask(cur, w, h, (size - 1) >> 1); });
            return cur === alpha ? Uint8ClampedArray.from(alpha) : cur;
        }
        const kernel = gaussianKernel(sigma);
        const radius = (kernel.length - 1) >> 1;
        // 8.8 fixed point between the passes keeps soft edges free of banding at half the memory of floats.
        const tmp = new Uint16Array(n);
        const out = new Uint8ClampedArray(n);
        for (let y = 0; y < h; y++) {
            const row = y * w;
            for (let x = 0; x < w; x++) {
                let acc = 0;
                for (let k = -radius; k <= radius; k++) {
                    const xx = x + k;
                    acc += alpha[row + (xx < 0 ? 0 : (xx >= w ? w - 1 : xx))] * kernel[k + radius];
                }
                tmp[row + x] = acc * 256 + 0.5;
            }
        }
        for (let y = 0; y < h; y++) {
            for (let x = 0; x < w; x++) {
                let acc = 0;
                for (let k = -radius; k <= radius; k++) {
                    const yy = y + k;
                    acc += tmp[(yy < 0 ? 0 : (yy >= h ? h - 1 : yy)) * w + x] * kernel[k + radius];
                }
                out[y * w + x] = acc / 256;
            }
        }
        return out;
    }

    // Gaussian blur of a mask; sigma = radius / 2 (the blur reaches about 1.5 * radius pixels).
    function blurAlpha(alpha, w, h, radius) {
        return gaussBlurMask(alpha, w, h, (Number(radius) || 0) / 2);
    }

    // Soft selection edge: same filter as blurAlpha under the name selections use.
    function featherAlpha(alpha, w, h, radius) {
        return blurAlpha(alpha, w, h, radius);
    }

    // Blurs RGBA with the colour weighted by alpha, so transparent pixels do not bleed black into the edge.
    function boxRGBALine(src, dst, so, stride, n, r) {
        const last = n - 1;
        const inv = 1 / (2 * r + 1);
        let sa = 0, sr = 0, sg = 0, sb = 0;
        const take = (k, sign) => {
            const idx = so + (k < 0 ? 0 : (k > last ? last : k)) * stride;
            const a = src[idx + 3];
            sa += sign * a; sr += sign * src[idx] * a; sg += sign * src[idx + 1] * a; sb += sign * src[idx + 2] * a;
        };
        for (let k = -r; k <= r; k++) take(k, 1);
        for (let i = 0; i < n; i++) {
            const o = so + i * stride;
            if (sa > 0) {
                dst[o] = sr / sa; dst[o + 1] = sg / sa; dst[o + 2] = sb / sa;
            } else {
                dst[o] = dst[o + 1] = dst[o + 2] = 0;
            }
            dst[o + 3] = sa * inv;
            take(i + r + 1, 1);
            take(i - r, -1);
        }
    }

    // Returns a new Uint8ClampedArray; r is the box half-width in pixels (0 returns a copy).
    function boxBlurRGBA(data, w, h, r) {
        r = Math.round(r);
        if (!(r >= 1) || !w || !h) return Uint8ClampedArray.from(data);
        const tmp = new Uint8ClampedArray(data.length);
        const out = new Uint8ClampedArray(data.length);
        for (let y = 0; y < h; y++) boxRGBALine(data, tmp, y * w * 4, 4, w, r);
        for (let x = 0; x < w; x++) boxRGBALine(tmp, out, x * 4, w * 4, h, r);
        return out;
    }

    // --- morphology and mask measurements ---

    // 1D max/min over a window of 2 * radius + 1 samples with van Herk / Gil-Werman (three comparisons per sample).
    function vanHerkLine(src, dst, so, stride, n, radius, isMax, pad, g, hh) {
        const k = radius * 2 + 1;
        const total = n + radius * 2;
        const padded = Math.ceil(total / k) * k;
        // Padded line value at index p (p in [0, padded)): identity element outside the real samples.
        const at = p => {
            const i = p - radius;
            return i >= 0 && i < n ? src[so + i * stride] : pad;
        };
        for (let p = 0; p < padded; p++) {
            const v = at(p);
            g[p] = p % k === 0 ? v : (isMax ? (v > g[p - 1] ? v : g[p - 1]) : (v < g[p - 1] ? v : g[p - 1]));
        }
        for (let p = padded - 1; p >= 0; p--) {
            const v = at(p);
            hh[p] = p % k === k - 1 || p === padded - 1 ? v : (isMax ? (v > hh[p + 1] ? v : hh[p + 1]) : (v < hh[p + 1] ? v : hh[p + 1]));
        }
        for (let i = 0; i < n; i++) {
            const a = hh[i], b = g[i + k - 1];
            dst[so + i * stride] = isMax ? (a > b ? a : b) : (a < b ? a : b);
        }
    }

    // Grow ('dilate') or shrink ('erode') a mask by a square structuring element of the given pixel radius.
    // Outside the image counts as empty for dilation and as full for erosion, so a shape touching the border keeps it.
    function morphAlpha(alpha, w, h, radius, mode) {
        const r = Math.max(0, Math.round(radius) || 0);
        if (!r || !w || !h) return Uint8ClampedArray.from(alpha);
        const isMax = mode !== 'erode';
        const pad = isMax ? 0 : 255;
        const k = r * 2 + 1;
        const longest = Math.max(w, h);
        const padded = Math.ceil((longest + r * 2) / k) * k;
        const g = new Uint8Array(padded), hh = new Uint8Array(padded);
        const tmp = new Uint8ClampedArray(alpha.length);
        const out = new Uint8ClampedArray(alpha.length);
        for (let y = 0; y < h; y++) vanHerkLine(alpha, tmp, y * w, 1, w, r, isMax, pad, g, hh);
        for (let x = 0; x < w; x++) vanHerkLine(tmp, out, x, w, h, r, isMax, pad, g, hh);
        return out;
    }

    // Bounding box of the samples above threshold, or null when there are none.
    function alphaBounds(alpha, w, h, threshold) {
        const t = threshold == null ? 0 : threshold;
        let minX = w, minY = h, maxX = -1, maxY = -1;
        for (let y = 0; y < h; y++) {
            const row = y * w;
            for (let x = 0; x < w; x++) {
                if (alpha[row + x] > t) {
                    if (x < minX) minX = x;
                    if (x > maxX) maxX = x;
                    if (y < minY) minY = y;
                    maxY = y;
                }
            }
        }
        return maxX < 0 ? null : { x: minX, y: minY, w: maxX - minX + 1, h: maxY - minY + 1 };
    }

    // Indices of the inside pixels (alpha > 127) that touch an outside pixel or the image edge's outside neighbour.
    function alphaBorder(alpha, w, h) {
        const found = [];
        for (let y = 0; y < h; y++) {
            for (let x = 0; x < w; x++) {
                const p = y * w + x;
                if (alpha[p] <= 127) continue;
                if ((x > 0 && alpha[p - 1] <= 127) || (x < w - 1 && alpha[p + 1] <= 127) ||
                    (y > 0 && alpha[p - w] <= 127) || (y < h - 1 && alpha[p + w] <= 127)) found.push(p);
            }
        }
        return Int32Array.from(found);
    }

    // --- gradients ---

    function stops(...list) {
        return list.map((color, i) => ({ pos: list.length === 1 ? 0 : i / (list.length - 1), color }));
    }

    function at(list) {
        return list.map(([pos, color]) => ({ pos, color }));
    }

    const GRADIENT_PRESETS = [
        { id: 'grayscale', stops: stops('#000000', '#ffffff') },
        { id: 'sunset', stops: stops('#1a1040', '#5b2a86', '#c2417f', '#ff7a45', '#ffd166') },
        { id: 'ocean', stops: stops('#03045e', '#0077b6', '#00b4d8', '#90e0ef', '#caf0f8') },
        { id: 'fire', stops: at([[0, '#000000'], [0.25, '#5a0f00'], [0.5, '#c1121f'], [0.72, '#ff6d00'], [0.9, '#ffd60a'], [1, '#ffffff']]) },
        { id: 'ice', stops: stops('#0b132b', '#1c5d99', '#639fab', '#bbcde5', '#ffffff') },
        { id: 'forest', stops: stops('#0b2a14', '#1b5e20', '#2e7d32', '#7cb342', '#dcedc8') },
        { id: 'rainbow', stops: stops('#ff0000', '#ffff00', '#00ff00', '#00ffff', '#0000ff', '#ff00ff', '#ff0000') },
        { id: 'viridis', stops: stops('#440154', '#482878', '#3e4989', '#31688e', '#26828e', '#1f9e89', '#35b779', '#6ece58', '#b5de2b', '#fde725') },
        { id: 'magma', stops: stops('#000004', '#180f3d', '#440f76', '#721f81', '#9e2f7f', '#cd4071', '#f1605d', '#fd9567', '#fec98d', '#fcfdbf') },
        { id: 'inferno', stops: stops('#000004', '#1b0c41', '#4a0c6b', '#781c6d', '#a52c60', '#cf4446', '#ed6925', '#fb9b06', '#f7d13d', '#fcffa4') },
        { id: 'plasma', stops: stops('#0d0887', '#46039f', '#7201a8', '#9c179e', '#bd3786', '#d8576b', '#ed7953', '#fb9f3a', '#fdca26', '#f0f921') },
        { id: 'copper', stops: stops('#000000', '#4d3017', '#99602e', '#e68f45', '#ffc77f') },
        { id: 'neon', stops: stops('#ff00ff', '#7a00ff', '#00e5ff', '#00ff9d', '#faff00') },
        { id: 'pastel', stops: stops('#ffd1dc', '#ffe5b4', '#fdfd96', '#b5ead7', '#c7ceea') },
        { id: 'earth', stops: stops('#2b1d0e', '#5c4033', '#7b8b3d', '#c2b280', '#f3e9d2') },
        { id: 'terrain', stops: at([[0, '#333399'], [0.15, '#0099cc'], [0.25, '#00cc66'], [0.5, '#ffff66'], [0.75, '#996633'], [1, '#ffffff']]) },
        { id: 'gold', stops: stops('#3d2b00', '#8a6a00', '#d4af37', '#ffe27a', '#fffbe6') },
        { id: 'sepia', stops: stops('#2b1d0e', '#704214', '#b08d57', '#e6d3a3', '#fff8e7') }
    ];

    function gradientPreset(id) {
        return GRADIENT_PRESETS.find(preset => preset.id === id) || null;
    }

    function cloneStops(list) {
        return (list || []).map(stop => {
            const copy = { pos: stop.pos, color: stop.color };
            if (stop.alpha != null) copy.alpha = stop.alpha;
            return copy;
        });
    }

    // Multi-stop gradient -> RGBA lookup table (size entries, entry i is the colour at i / (size - 1)).
    // Stops: { pos 0..1, color '#rrggbb', alpha 0..1 (optional) }; unsorted input is fine.
    function gradientLUT(gradientStops, size) {
        const n = Math.max(2, Math.round(size) || 256);
        const lut = new Uint8ClampedArray(n * 4);
        const list = (gradientStops || [])
            .map(stop => {
                const rgba = parseColor(stop.color) || [0, 0, 0, 255];
                return { pos: clamp(Number(stop.pos) || 0, 0, 1), r: rgba[0], g: rgba[1], b: rgba[2], a: stop.alpha != null ? clamp(Number(stop.alpha), 0, 1) * 255 : rgba[3] };
            })
            .sort((p, q) => p.pos - q.pos);
        if (!list.length) {
            for (let i = 0; i < n; i++) {
                const v = i / (n - 1) * 255;
                lut[i * 4] = lut[i * 4 + 1] = lut[i * 4 + 2] = v;
                lut[i * 4 + 3] = 255;
            }
            return lut;
        }
        let seg = 0;
        for (let i = 0; i < n; i++) {
            const t = i / (n - 1);
            while (seg < list.length - 1 && t > list[seg + 1].pos) seg++;
            const a = list[seg], b = list[Math.min(list.length - 1, seg + 1)];
            let f = 0;
            if (t <= a.pos) f = 0;
            else if (b === a || t >= b.pos) f = 1;
            else f = (t - a.pos) / (b.pos - a.pos);
            const from = t < list[0].pos ? list[0] : a;
            const to = t < list[0].pos ? list[0] : b;
            const o = i * 4;
            lut[o] = from.r + (to.r - from.r) * f;
            lut[o + 1] = from.g + (to.g - from.g) * f;
            lut[o + 2] = from.b + (to.b - from.b) * f;
            lut[o + 3] = from.a + (to.a - from.a) * f;
        }
        return lut;
    }

    // --- ops and the message protocol shared by the worker and the main-thread fallback ---

    function registerOp(name, fn) {
        if (typeof name !== 'string' || !name || typeof fn !== 'function') throw new Error('pixel lib: registerOp(name, fn) needs a name and a function');
        ops[name] = fn;
        return fn;
    }

    const isArrayBuffer = value => Object.prototype.toString.call(value) === '[object ArrayBuffer]';

    // ArrayBuffers (or whole-buffer typed arrays) found in a result, so the worker can transfer instead of copy.
    function collectTransfer(value, found, depth) {
        const list = found || [];
        if (!value || typeof value !== 'object' || (depth || 0) > 4) return list;
        if (isArrayBuffer(value)) {
            if (!list.includes(value)) list.push(value);
        } else if (ArrayBuffer.isView(value)) {
            const buffer = value.buffer;
            if (isArrayBuffer(buffer) && value.byteOffset === 0 && value.byteLength === buffer.byteLength && !list.includes(buffer)) list.push(buffer);
        } else if (Array.isArray(value)) {
            value.forEach(item => collectTransfer(item, list, (depth || 0) + 1));
        } else {
            Object.keys(value).forEach(key => collectTransfer(value[key], list, (depth || 0) + 1));
        }
        return list;
    }

    function failure(id, err) {
        return { id, error: err && err.message ? String(err.message) : String(err) };
    }

    // msg: { id, op, args }. hooks (optional): { progress(fraction), cancelled() }.
    // Returns { id, result, transfer } or { id, error }; a Promise of that when the op is asynchronous.
    function handleMessage(msg, hooks) {
        const id = msg && msg.id;
        try {
            if (!msg || typeof msg.op !== 'string') throw new Error('pixel lib: message without op');
            const fn = ops[msg.op];
            if (typeof fn !== 'function') throw new Error('pixel lib: unknown op ' + msg.op);
            const h = hooks || {};
            const ctx = {
                progress(fraction) { if (typeof h.progress === 'function') h.progress(clamp(Number(fraction) || 0, 0, 1)); },
                cancelled() { return typeof h.cancelled === 'function' ? !!h.cancelled() : false; }
            };
            const out = fn(msg.args || {}, ctx);
            const pack = result => ({ id, result, transfer: collectTransfer(result) });
            if (out && typeof out.then === 'function') return out.then(pack, err => failure(id, err));
            return pack(out);
        } catch (err) {
            return failure(id, err);
        }
    }

    registerOp('core.ping', () => ({ pong: true, ops: Object.keys(ops).length }));
    registerOp('core.resize', args => ({ image: resizeBilinear(args.image, args.width, args.height) }));
    registerOp('core.blurAlpha', args => ({ alpha: blurAlpha(args.alpha, args.width, args.height, args.radius) }));
    registerOp('core.morph', args => ({ alpha: morphAlpha(args.alpha, args.width, args.height, args.radius, args.mode) }));

    Object.assign(Lib, {
        version: 1,
        registerOp,
        handleMessage,
        collectTransfer,
        clamp,
        clamp255,
        mulberry32,
        hashSeed,
        luma,
        gaussianKernel,
        blurAlpha,
        featherAlpha,
        boxBlurRGBA,
        rgbToHsl,
        hslToRgb,
        rgbToHsv,
        hsvToRgb,
        parseColor,
        toHex,
        createImage,
        resizeBilinear,
        sampleBilinear,
        morphAlpha,
        alphaBounds,
        alphaBorder,
        GRADIENT_PRESETS,
        gradientPreset,
        cloneStops,
        gradientLUT
    });
})(typeof self !== 'undefined' ? self : (typeof globalThis !== 'undefined' ? globalThis : this));
