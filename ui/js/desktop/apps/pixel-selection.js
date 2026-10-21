// Pixel module "selection" (WP3): selection tools (rectangle, ellipse, lasso, polygon lasso, magic wand, quick selection),
// the "select" menu (all, deselect, reselect, inverse, subject, colour range, modify, layer conversions, fill, stroke),
// the marching-ants overlay and the selection helpers other packages can call (r.modifySelection, r.recordSelection, ...).
// The pure part (masks, distance transform, region growing, contour tracing) is DOM-free and exported as
// Pixel.selectionKit, so scripts/test-pixel-selection.mjs can run it on tiny images.
(function () {
    'use strict';

    const Pixel = window.AuraPixelApp = window.AuraPixelApp || {};
    const kit = Pixel.selectionKit = {};

    // ===============================================================================================================
    // Pure helpers. Masks are Uint8ClampedArray(w * h), 0..255; boxes are { x, y, w, h }.
    // ===============================================================================================================

    function clamp(v, lo, hi) {
        return v < lo ? lo : (v > hi ? hi : v);
    }

    function now() {
        return typeof performance !== 'undefined' && performance.now ? performance.now() : Date.now();
    }

    // Bounding box of the samples above threshold, or null.
    function boundsOf(alpha, w, h, threshold) {
        const t = threshold || 0;
        let minX = w, minY = h, maxX = -1, maxY = -1;
        for (let y = 0; y < h; y++) {
            const row = y * w;
            let first = -1;
            for (let x = 0; x < w; x++) {
                if (alpha[row + x] > t) { first = x; break; }
            }
            if (first < 0) continue;
            let last = first;
            for (let x = w - 1; x > first; x--) {
                if (alpha[row + x] > t) { last = x; break; }
            }
            if (first < minX) minX = first;
            if (last > maxX) maxX = last;
            if (y < minY) minY = y;
            maxY = y;
        }
        return maxX < 0 ? null : { x: minX, y: minY, w: maxX - minX + 1, h: maxY - minY + 1 };
    }

    function growBox(box, margin, w, h) {
        const x0 = Math.max(0, box.x - margin), y0 = Math.max(0, box.y - margin);
        const x1 = Math.min(w, box.x + box.w + margin), y1 = Math.min(h, box.y + box.h + margin);
        return { x: x0, y: y0, w: Math.max(0, x1 - x0), h: Math.max(0, y1 - y0) };
    }

    function extractBox(src, w, box) {
        const out = new Uint8ClampedArray(box.w * box.h);
        for (let y = 0; y < box.h; y++) {
            const from = (box.y + y) * w + box.x;
            out.set(src.subarray(from, from + box.w), y * box.w);
        }
        return out;
    }

    function pasteBox(dst, w, box, part) {
        for (let y = 0; y < box.h; y++) {
            dst.set(part.subarray(y * box.w, (y + 1) * box.w), (box.y + y) * w + box.x);
        }
        return dst;
    }

    // --- shapes ---

    function rectMask(w, h, x, y, rw, rh) {
        const out = new Uint8ClampedArray(w * h);
        const x0 = clamp(Math.round(x), 0, w), x1 = clamp(Math.round(x + rw), 0, w);
        const y0 = clamp(Math.round(y), 0, h), y1 = clamp(Math.round(y + rh), 0, h);
        for (let yy = y0; yy < y1; yy++) out.fill(255, yy * w + x0, yy * w + x1);
        return out;
    }

    // Coverage from the distance of the pixel centre to the outline (f / |grad f| of the implicit ellipse equation).
    function ellipseMask(w, h, x, y, rw, rh, antialias) {
        const out = new Uint8ClampedArray(w * h);
        const a = rw / 2, b = rh / 2;
        if (!(a > 0) || !(b > 0)) return out;
        const cx = x + a, cy = y + b;
        const x0 = Math.max(0, Math.floor(x) - 1), x1 = Math.min(w, Math.ceil(x + rw) + 1);
        const y0 = Math.max(0, Math.floor(y) - 1), y1 = Math.min(h, Math.ceil(y + rh) + 1);
        for (let yy = y0; yy < y1; yy++) {
            const ny = (yy + 0.5 - cy) / b;
            for (let xx = x0; xx < x1; xx++) {
                const nx = (xx + 0.5 - cx) / a;
                const f = nx * nx + ny * ny - 1;
                if (antialias === false) {
                    if (f <= 0) out[yy * w + xx] = 255;
                    continue;
                }
                const gx = nx / a, gy = ny / b;
                const g = 2 * Math.sqrt(gx * gx + gy * gy);
                const sd = g > 1e-9 ? f / g : -1;
                const cover = clamp(0.5 - sd, 0, 1);
                if (cover > 0) out[yy * w + xx] = Math.round(cover * 255);
            }
        }
        return out;
    }

    // Non-zero winding scanline fill. Four sub-scanlines per row and exact horizontal coverage give the anti-aliased edge.
    function polygonMask(w, h, points, antialias) {
        const n = points ? points.length : 0;
        if (n < 3) return null;
        let minX = Infinity, minY = Infinity, maxX = -Infinity, maxY = -Infinity;
        for (let i = 0; i < n; i++) {
            const p = points[i];
            if (p.x < minX) minX = p.x;
            if (p.x > maxX) maxX = p.x;
            if (p.y < minY) minY = p.y;
            if (p.y > maxY) maxY = p.y;
        }
        const x0 = Math.max(0, Math.floor(minX)), x1 = Math.min(w, Math.ceil(maxX));
        const y0 = Math.max(0, Math.floor(minY)), y1 = Math.min(h, Math.ceil(maxY));
        if (x1 <= x0 || y1 <= y0) return null;
        const bw = x1 - x0, bh = y1 - y0;
        const sub = antialias === false ? 1 : 4;
        const rows = bh * sub;
        const crossings = new Array(rows);
        for (let i = 0; i < n; i++) {
            const a = points[i], b = points[(i + 1) % n];
            if (a.y === b.y) continue;
            const down = b.y > a.y;
            const ya = down ? a.y : b.y, yb = down ? b.y : a.y;
            const xa = down ? a.x : b.x, xb = down ? b.x : a.x;
            const slope = (xb - xa) / (yb - ya);
            const from = Math.max(0, Math.ceil((ya - y0) * sub - 0.5));
            const to = Math.min(rows - 1, Math.ceil((yb - y0) * sub - 0.5) - 1);
            for (let k = from; k <= to; k++) {
                const sy = y0 + (k + 0.5) / sub;
                (crossings[k] || (crossings[k] = [])).push(xa + (sy - ya) * slope, down ? 1 : -1);
            }
        }
        const alpha = new Uint8ClampedArray(w * h);
        const cover = new Float32Array(bw);
        const addSpan = (sx0, sx1) => {
            const xa = clamp(sx0 - x0, 0, bw), xb = clamp(sx1 - x0, 0, bw);
            if (xb <= xa) return;
            const ia = Math.floor(xa), ib = Math.floor(xb);
            if (ia === ib) { cover[ia] += xb - xa; return; }
            cover[ia] += ia + 1 - xa;
            for (let i = ia + 1; i < ib; i++) cover[i] += 1;
            if (ib < bw) cover[ib] += xb - ib;
        };
        for (let row = 0; row < bh; row++) {
            cover.fill(0);
            for (let s = 0; s < sub; s++) {
                const list = crossings[row * sub + s];
                if (!list || list.length < 4) continue;
                for (let i = 2; i < list.length; i += 2) {
                    const x = list[i], d = list[i + 1];
                    let j = i - 2;
                    while (j >= 0 && list[j] > x) { list[j + 2] = list[j]; list[j + 3] = list[j + 1]; j -= 2; }
                    list[j + 2] = x; list[j + 3] = d;
                }
                let winding = 0;
                for (let i = 0; i + 2 < list.length; i += 2) {
                    winding += list[i + 1];
                    if (winding !== 0) addSpan(list[i], list[i + 2]);
                }
            }
            const base = (y0 + row) * w + x0;
            for (let i = 0; i < bw; i++) {
                const v = Math.min(1, cover[i] / sub);
                alpha[base + i] = sub === 1 ? (v >= 0.5 ? 255 : 0) : Math.round(v * 255);
            }
        }
        return alpha;
    }

    // Moves a mask; pixels pushed outside are dropped.
    function shiftMask(alpha, w, h, dx, dy) {
        const out = new Uint8ClampedArray(alpha.length);
        const x0 = Math.max(0, -dx), x1 = Math.min(w, w - dx);
        if (x1 <= x0) return out;
        for (let y = 0; y < h; y++) {
            const ny = y + dy;
            if (ny < 0 || ny >= h) continue;
            out.set(alpha.subarray(y * w + x0, y * w + x1), ny * w + x0 + dx);
        }
        return out;
    }

    // [1 2 1] smoothing of the edges (in place, only inside the mask bounds + 1): the anti-alias of flood-filled masks.
    function smoothEdges(alpha, w, h) {
        const box = boundsOf(alpha, w, h, 0);
        if (!box) return alpha;
        const b = growBox(box, 1, w, h);
        const part = extractBox(alpha, w, b);
        const bw = b.w, bh = b.h;
        const tmp = new Uint8ClampedArray(part.length);
        for (let y = 0; y < bh; y++) {
            const row = y * bw;
            for (let x = 0; x < bw; x++) {
                const l = part[row + (x > 0 ? x - 1 : x)], c = part[row + x], r = part[row + (x < bw - 1 ? x + 1 : x)];
                tmp[row + x] = (l + 2 * c + r + 2) >> 2;
            }
        }
        for (let y = 0; y < bh; y++) {
            const up = (y > 0 ? y - 1 : y) * bw, row = y * bw, down = (y < bh - 1 ? y + 1 : y) * bw;
            for (let x = 0; x < bw; x++) part[row + x] = (tmp[up + x] + 2 * tmp[row + x] + tmp[down + x] + 2) >> 2;
        }
        pasteBox(alpha, w, b, part);
        return alpha;
    }

    // --- distance transform (Felzenszwalb & Huttenlocher), signed distance, round grow / shrink / border / stroke ---

    const FAR = 1e12;

    function squaredDistance1d(g, off, stride, n, tmp, v, z) {
        let k = 0;
        v[0] = 0; z[0] = -FAR; z[1] = FAR;
        for (let q = 1; q < n; q++) {
            const fq = g[off + q * stride] + q * q;
            let s;
            for (;;) {
                const p = v[k];
                s = (fq - (g[off + p * stride] + p * p)) / (2 * q - 2 * p);
                if (s > z[k] || k === 0) break;
                k--;
            }
            k++; v[k] = q; z[k] = s; z[k + 1] = FAR;
        }
        k = 0;
        for (let q = 0; q < n; q++) {
            while (z[k + 1] < q) k++;
            const p = v[k];
            tmp[q] = (q - p) * (q - p) + g[off + p * stride];
        }
        for (let q = 0; q < n; q++) g[off + q * stride] = tmp[q];
    }

    // Euclidean distance of every sample to the nearest feature sample (feature[i] truthy); 0 on features.
    function distanceToFeature(feature, w, h) {
        const g = new Float32Array(w * h);
        for (let i = 0; i < g.length; i++) g[i] = feature[i] ? 0 : FAR;
        const m = Math.max(w, h);
        const tmp = new Float64Array(m), v = new Int32Array(m), z = new Float64Array(m + 1);
        for (let y = 0; y < h; y++) squaredDistance1d(g, y * w, 1, w, tmp, v, z);
        for (let x = 0; x < w; x++) squaredDistance1d(g, x, w, h, tmp, v, z);
        for (let i = 0; i < g.length; i++) g[i] = Math.sqrt(g[i]);
        return g;
    }

    // Positive inside, negative outside; the outline (threshold 128) lies at 0, a pixel next to it has +-0.5.
    function signedDistance(alpha, w, h) {
        const n = w * h;
        const inside = new Uint8Array(n), outside = new Uint8Array(n);
        for (let i = 0; i < n; i++) {
            if (alpha[i] >= 128) inside[i] = 1;
            else outside[i] = 1;
        }
        const toInside = distanceToFeature(inside, w, h);
        const toOutside = distanceToFeature(outside, w, h);
        const sd = new Float32Array(n);
        for (let i = 0; i < n; i++) sd[i] = inside[i] ? toOutside[i] - 0.5 : -(toInside[i] - 0.5);
        return sd;
    }

    // Area of the pixel footprint [sd - 0.5, sd + 0.5] that lies in [lo, hi].
    function bandCoverage(sd, lo, hi) {
        return clamp(Math.min(sd + 0.5, hi) - Math.max(sd - 0.5, lo), 0, 1);
    }

    function morphRound(alpha, w, h, radius, mode) {
        const r = Math.max(0, Number(radius) || 0);
        if (!r) return Uint8ClampedArray.from(alpha);
        const sd = signedDistance(alpha, w, h);
        const out = new Uint8ClampedArray(alpha.length);
        const grow = mode !== 'shrink';
        for (let i = 0; i < out.length; i++) {
            const v = Math.round(clamp(grow ? sd[i] + r + 0.5 : sd[i] - r + 0.5, 0, 1) * 255);
            out[i] = grow ? (v > alpha[i] ? v : alpha[i]) : (v < alpha[i] ? v : alpha[i]);
        }
        return out;
    }

    // Ring along the outline. location: 'center' | 'inside' | 'outside'; border() is the centred form.
    function ringMask(alpha, w, h, width, location) {
        const wd = Math.max(0, Number(width) || 0);
        const out = new Uint8ClampedArray(alpha.length);
        if (!wd) return out;
        let lo = -wd / 2, hi = wd / 2;
        if (location === 'inside') { lo = 0; hi = wd; }
        else if (location === 'outside') { lo = -wd; hi = 0; }
        const sd = signedDistance(alpha, w, h);
        for (let i = 0; i < out.length; i++) {
            if (sd[i] < lo - 1 || sd[i] > hi + 1) continue;
            out[i] = Math.round(bandCoverage(sd[i], lo, hi) * 255);
        }
        return out;
    }

    // Blur-and-sharpen: rounds corners and drops specks. blurred = gaussian blur of the mask with sigma = radius / 2.
    function sharpenBlurred(blurred, radius) {
        const k = Math.max(1, 1.25 * radius);
        const out = new Uint8ClampedArray(blurred.length);
        for (let i = 0; i < out.length; i++) out[i] = clamp((blurred[i] - 127.5) * k + 127.5, 0, 255);
        return out;
    }

    // --- colour ---

    const SRGB_LINEAR = new Float32Array(256);
    for (let i = 0; i < 256; i++) {
        const c = i / 255;
        SRGB_LINEAR[i] = c <= 0.04045 ? c / 12.92 : Math.pow((c + 0.055) / 1.055, 2.4);
    }

    function labCurve(t) {
        return t > 0.008856 ? Math.cbrt(t) : 7.787 * t + 16 / 116;
    }

    // sRGB (0..255) -> CIE Lab (D65), written to out[o..o+2].
    function rgbToLab(r, g, b, out, o) {
        const lr = SRGB_LINEAR[clamp(Math.round(r), 0, 255)], lg = SRGB_LINEAR[clamp(Math.round(g), 0, 255)], lb = SRGB_LINEAR[clamp(Math.round(b), 0, 255)];
        const x = labCurve((0.4124564 * lr + 0.3575761 * lg + 0.1804375 * lb) / 0.95047);
        const y = labCurve(0.2126729 * lr + 0.7151522 * lg + 0.0721750 * lb);
        const z = labCurve((0.0193339 * lr + 0.1191920 * lg + 0.9503041 * lb) / 1.08883);
        out[o] = 116 * y - 16;
        out[o + 1] = 500 * (x - y);
        out[o + 2] = 200 * (y - z);
        return out;
    }

    // --- flood selection (magic wand) ---

    // Channels within tol of the seed pixel (alpha included). Contiguous: 4-connected scanline fill from the seed.
    function floodSelect(rgba, w, h, sx, sy, tol, contiguous) {
        const out = new Uint8ClampedArray(w * h);
        if (sx < 0 || sy < 0 || sx >= w || sy >= h) return out;
        const so = (sy * w + sx) * 4;
        const r0 = rgba[so], g0 = rgba[so + 1], b0 = rgba[so + 2], a0 = rgba[so + 3];
        const t = Math.max(0, Number(tol) || 0);
        const matches = p => {
            const i = p * 4;
            const a = rgba[i + 3];
            if (a0 < 8 || a < 8) return a0 < 8 && a < 8;
            return Math.abs(rgba[i] - r0) <= t && Math.abs(rgba[i + 1] - g0) <= t && Math.abs(rgba[i + 2] - b0) <= t && Math.abs(a - a0) <= t;
        };
        if (!contiguous) {
            for (let p = 0; p < out.length; p++) if (matches(p)) out[p] = 255;
            return out;
        }
        let stack = new Int32Array(1024), sp = 0;
        const push = (x, y) => {
            if (sp + 2 > stack.length) { const bigger = new Int32Array(stack.length * 2); bigger.set(stack); stack = bigger; }
            stack[sp++] = x; stack[sp++] = y;
        };
        push(sx, sy);
        while (sp > 0) {
            const y = stack[--sp], x = stack[--sp];
            const row = y * w;
            if (out[row + x] || !matches(row + x)) continue;
            let xl = x, xr = x;
            while (xl > 0 && !out[row + xl - 1] && matches(row + xl - 1)) xl--;
            while (xr < w - 1 && !out[row + xr + 1] && matches(row + xr + 1)) xr++;
            out.fill(255, row + xl, row + xr + 1);
            for (let dir = -1; dir <= 1; dir += 2) {
                const ny = y + dir;
                if (ny < 0 || ny >= h) continue;
                const nrow = ny * w;
                let open = false;
                for (let i = xl; i <= xr; i++) {
                    const ok = !out[nrow + i] && matches(nrow + i);
                    if (ok && !open) push(i, ny);
                    open = ok;
                }
            }
        }
        return out;
    }

    // --- colour range ---

    // include / exclude: arrays of [r, g, b]. fuzz 0..200 (half of it is the Lab distance where the membership reaches 0).
    // A 6-bit-per-channel lookup keeps the cost proportional to the number of distinct colours, not pixels.
    function colorRangeAlpha(rgba, w, h, include, exclude, fuzz, invert) {
        const radius = Math.max(0, Number(fuzz) || 0) * 0.5;
        const lab = [0, 0, 0];
        const toLab = list => (list || []).map(c => Array.from(rgbToLab(c[0], c[1], c[2], [0, 0, 0], 0)));
        const inc = toLab(include), exc = toLab(exclude);
        const member = d => (radius <= 0 ? (d < 0.75 ? 1 : 0) : clamp((radius - d) / (radius * 0.5), 0, 1));
        const best = (list, l, a, b) => {
            let m = 0;
            for (let i = 0; i < list.length; i++) {
                const dl = l - list[i][0], da = a - list[i][1], db = b - list[i][2];
                const v = member(Math.sqrt(dl * dl + da * da + db * db));
                if (v > m) m = v;
            }
            return m;
        };
        const lut = new Int16Array(1 << 18).fill(-1);
        const out = new Uint8ClampedArray(w * h);
        for (let p = 0; p < out.length; p++) {
            const o = p * 4;
            const key = ((rgba[o] >> 2) << 12) | ((rgba[o + 1] >> 2) << 6) | (rgba[o + 2] >> 2);
            let v = lut[key];
            if (v < 0) {
                rgbToLab(((key >> 12) & 63) * 4 + 2, ((key >> 6) & 63) * 4 + 2, (key & 63) * 4 + 2, lab, 0);
                v = Math.round(best(inc, lab[0], lab[1], lab[2]) * (1 - best(exc, lab[0], lab[1], lab[2])) * 255);
                lut[key] = v;
            }
            let f = v / 255 * (rgba[o + 3] / 255);
            if (invert) f = 1 - f;
            out[p] = Math.round(f * 255);
        }
        return out;
    }

    // --- quick selection: region growing in Lab space with a gradient barrier ---

    const MAX_CENTERS = 8;

    // rgba: ImageData-like bytes of w * h pixels. options: { tolerance (Lab distance), barrier (Lab gradient over 2 px) }.
    // seedDisc() adds the pixels under the brush (always selected) and learns their colours; grow(ms) spreads the region
    // to 4-neighbours that are close to a learned colour and stops at strong edges. Returns true when the queue is empty.
    function createGrower(rgba, w, h, options) {
        const o = options || {};
        const tol = o.tolerance > 0 ? o.tolerance : 1;
        const barrier = o.barrier > 0 ? o.barrier : 12;
        const n = w * h;
        const lab = new Float32Array(n * 3);
        const have = new Uint8Array(n);
        const state = new Uint8Array(n); // 0 open, 1 grown, 2 rejected, 3 grown on an edge (does not spread)
        const centers = [];
        let queue = new Int32Array(4096), head = 0, tail = 0;
        let rejected = new Int32Array(1024), rejectedCount = 0;
        let count = 0;
        const dirty = { x0: w, y0: h, x1: -1, y1: -1 };

        function labOf(p) {
            if (have[p]) return;
            const i = p * 4, a = rgba[i + 3];
            let r = rgba[i], g = rgba[i + 1], b = rgba[i + 2];
            if (a < 255) {
                const k = a / 255;
                r = r * k + 128 * (1 - k); g = g * k + 128 * (1 - k); b = b * k + 128 * (1 - k);
            }
            rgbToLab(r, g, b, lab, p * 3);
            have[p] = 1;
        }

        function distToCenter(p, c) {
            const i = p * 3;
            const dl = lab[i] - c.l, da = lab[i + 1] - c.a, db = lab[i + 2] - c.b;
            return Math.sqrt(dl * dl + da * da + db * db);
        }

        function minDistance(p) {
            let m = Infinity;
            for (let i = 0; i < centers.length; i++) {
                const d = distToCenter(p, centers[i]);
                if (d < m) m = d;
            }
            return m;
        }

        function pairDistance(p, q) {
            const i = p * 3, j = q * 3;
            const dl = lab[i] - lab[j], da = lab[i + 1] - lab[j + 1], db = lab[i + 2] - lab[j + 2];
            return Math.sqrt(dl * dl + da * da + db * db);
        }

        function gradient(p) {
            const x = p % w, y = (p / w) | 0;
            const left = y * w + (x > 0 ? x - 1 : x), right = y * w + (x < w - 1 ? x + 1 : x);
            const up = (y > 0 ? y - 1 : y) * w + x, down = (y < h - 1 ? y + 1 : y) * w + x;
            labOf(left); labOf(right); labOf(up); labOf(down);
            return Math.max(pairDistance(left, right), pairDistance(up, down));
        }

        function push(p) {
            if (tail === queue.length) {
                if (head > 0) {
                    queue.copyWithin(0, head, tail);
                    tail -= head;
                    head = 0;
                } else {
                    const bigger = new Int32Array(queue.length * 2);
                    bigger.set(queue);
                    queue = bigger;
                }
            }
            queue[tail++] = p;
        }

        function mark(p, value) {
            state[p] = value;
            count++;
            const x = p % w, y = (p / w) | 0;
            if (x < dirty.x0) dirty.x0 = x;
            if (x > dirty.x1) dirty.x1 = x;
            if (y < dirty.y0) dirty.y0 = y;
            if (y > dirty.y1) dirty.y1 = y;
        }

        function remember(q) {
            if (rejectedCount === rejected.length) {
                if (rejected.length >= (1 << 22)) return;
                const bigger = new Int32Array(rejected.length * 2);
                bigger.set(rejected);
                rejected = bigger;
            }
            rejected[rejectedCount++] = q;
        }

        // A new colour may make earlier rejections acceptable: reopen them and wake the grown pixels beside them.
        function requeueRejected() {
            for (let i = 0; i < rejectedCount; i++) {
                const q = rejected[i];
                if (state[q] !== 2) continue;
                state[q] = 0;
                const x = q % w, y = (q / w) | 0;
                if (x > 0 && state[q - 1] === 1) push(q - 1);
                if (x < w - 1 && state[q + 1] === 1) push(q + 1);
                if (y > 0 && state[q - w] === 1) push(q - w);
                if (y < h - 1 && state[q + w] === 1) push(q + w);
            }
            rejectedCount = 0;
        }

        // Teaches the colour model: returns true when a new colour was added.
        function absorb(p) {
            let best = -1, bestDist = Infinity;
            for (let i = 0; i < centers.length; i++) {
                const d = distToCenter(p, centers[i]);
                if (d < bestDist) { bestDist = d; best = i; }
            }
            if (best < 0 || (bestDist > tol * 0.5 && centers.length < MAX_CENTERS)) {
                centers.push({ l: lab[p * 3], a: lab[p * 3 + 1], b: lab[p * 3 + 2], n: 1 });
                return true;
            }
            const c = centers[best];
            if (c.n < 64) c.n++;
            const k = 1 / c.n;
            c.l += (lab[p * 3] - c.l) * k; c.a += (lab[p * 3 + 1] - c.a) * k; c.b += (lab[p * 3 + 2] - c.b) * k;
            return false;
        }

        function visit(q) {
            if (state[q] !== 0) return;
            labOf(q);
            if (minDistance(q) > tol) { state[q] = 2; remember(q); return; }
            if (gradient(q) > barrier) { mark(q, 3); return; }
            mark(q, 1);
            push(q);
        }

        return {
            width: w,
            height: h,
            state,
            centers,
            count: () => count,
            seedDisc(cx, cy, radius) {
                const rad = Math.max(0.5, radius);
                const x0 = Math.max(0, Math.floor(cx - rad)), x1 = Math.min(w - 1, Math.ceil(cx + rad));
                const y0 = Math.max(0, Math.floor(cy - rad)), y1 = Math.min(h - 1, Math.ceil(cy + rad));
                const picked = [];
                for (let y = y0; y <= y1; y++) {
                    for (let x = x0; x <= x1; x++) {
                        const dx = x - cx, dy = y - cy;
                        if (dx * dx + dy * dy <= rad * rad) picked.push(y * w + x);
                    }
                }
                if (!picked.length) return 0;
                const stride = picked.length > 64 ? Math.ceil(picked.length / 64) : 1;
                let learned = false;
                for (let i = 0; i < picked.length; i += stride) {
                    labOf(picked[i]);
                    if (absorb(picked[i])) learned = true;
                }
                for (let i = 0; i < picked.length; i++) {
                    const p = picked[i];
                    if (state[p] === 1 || state[p] === 3) continue;
                    mark(p, 1);
                    push(p);
                }
                if (learned) requeueRejected();
                return picked.length;
            },
            grow(maxMs) {
                const start = now();
                let steps = 0;
                while (head < tail) {
                    const p = queue[head++];
                    if (state[p] !== 1) continue;
                    const x = p % w, y = (p / w) | 0;
                    if (x > 0) visit(p - 1);
                    if (x < w - 1) visit(p + 1);
                    if (y > 0) visit(p - w);
                    if (y < h - 1) visit(p + w);
                    if ((++steps & 255) === 0 && maxMs >= 0 && now() - start > maxMs) return head >= tail;
                }
                head = tail = 0;
                return true;
            },
            // Box of pixels grown since the last call (for incremental previews), or null.
            drainDirty() {
                if (dirty.x1 < 0) return null;
                const box = { x: dirty.x0, y: dirty.y0, w: dirty.x1 - dirty.x0 + 1, h: dirty.y1 - dirty.y0 + 1 };
                dirty.x0 = w; dirty.y0 = h; dirty.x1 = -1; dirty.y1 = -1;
                return box;
            },
            binary() {
                const out = new Uint8Array(n);
                for (let i = 0; i < n; i++) if (state[i] === 1 || state[i] === 3) out[i] = 1;
                return out;
            }
        };
    }

    // 3x3 majority vote: removes specks and fills one-pixel holes of a 0/1 mask.
    function majority(bin, w, h) {
        const out = new Uint8Array(bin.length);
        for (let y = 0; y < h; y++) {
            for (let x = 0; x < w; x++) {
                let sum = 0;
                for (let dy = -1; dy <= 1; dy++) {
                    const yy = y + dy;
                    if (yy < 0 || yy >= h) { sum += 3 * bin[y * w + x]; continue; }
                    for (let dx = -1; dx <= 1; dx++) {
                        const xx = x + dx;
                        sum += xx < 0 || xx >= w ? bin[y * w + x] : bin[yy * w + xx];
                    }
                }
                out[y * w + x] = sum >= 5 ? 1 : 0;
            }
        }
        return out;
    }

    // 0/1 mask of pw x ph -> document-sized alpha: bilinear sampling and a contrast ramp about one document pixel wide.
    function upscaleBinary(bin, pw, ph, w, h) {
        const out = new Uint8ClampedArray(w * h);
        let minX = pw, maxX = -1, minY = ph, maxY = -1;
        for (let y = 0; y < ph; y++) {
            for (let x = 0; x < pw; x++) {
                if (!bin[y * pw + x]) continue;
                if (x < minX) minX = x;
                if (x > maxX) maxX = x;
                if (y < minY) minY = y;
                if (y > maxY) maxY = y;
            }
        }
        if (maxX < 0) return out;
        const sx = pw / w, sy = ph / h;
        const dx0 = Math.max(0, Math.floor((minX - 1) / sx)), dx1 = Math.min(w, Math.ceil((maxX + 2) / sx));
        const dy0 = Math.max(0, Math.floor((minY - 1) / sy)), dy1 = Math.min(h, Math.ceil((maxY + 2) / sy));
        const ramp = Math.max(1, 1 / sx);
        for (let y = dy0; y < dy1; y++) {
            const fy = (y + 0.5) * sy - 0.5;
            const yf = Math.floor(fy), ty = fy - yf;
            const ya = clamp(yf, 0, ph - 1), yb = clamp(yf + 1, 0, ph - 1);
            for (let x = dx0; x < dx1; x++) {
                const fx = (x + 0.5) * sx - 0.5;
                const xf = Math.floor(fx), tx = fx - xf;
                const xa = clamp(xf, 0, pw - 1), xb = clamp(xf + 1, 0, pw - 1);
                const top = bin[ya * pw + xa] * (1 - tx) + bin[ya * pw + xb] * tx;
                const bottom = bin[yb * pw + xa] * (1 - tx) + bin[yb * pw + xb] * tx;
                const c = clamp(((top * (1 - ty) + bottom * ty) - 0.5) * ramp + 0.5, 0, 1);
                if (c > 0) out[y * w + x] = Math.round(c * 255);
            }
        }
        return out;
    }

    // --- marching ants: outline of the mask (alpha >= 128) as closed loops of pixel-edge vertices ---

    // Returns an array of Int32Array [x0, y0, x1, y1, ...] loops (collinear vertices dropped), or null when the outline
    // has more than maxEdges unit edges (noise masks): callers fall back to drawing border pixels.
    function traceContours(alpha, w, h, box, maxEdges) {
        const bw = box.w, bh = box.h, vw = bw + 1;
        const outs = new Uint8Array(vw * (bh + 1)); // bit d: an edge leaves the vertex in direction d (0 right, 1 down, 2 left, 3 up)
        const cap = maxEdges > 0 ? maxEdges : 400000;
        let edges = 0;
        for (let y = box.y; y < box.y + bh; y++) {
            for (let x = box.x; x < box.x + bw; x++) {
                const p = y * w + x;
                if (alpha[p] < 128) continue;
                const vx = x - box.x, vy = y - box.y;
                if (y === 0 || alpha[p - w] < 128) { outs[vy * vw + vx] |= 1; edges++; }
                if (x === w - 1 || alpha[p + 1] < 128) { outs[vy * vw + vx + 1] |= 2; edges++; }
                if (y === h - 1 || alpha[p + w] < 128) { outs[(vy + 1) * vw + vx + 1] |= 4; edges++; }
                if (x === 0 || alpha[p - 1] < 128) { outs[(vy + 1) * vw + vx] |= 8; edges++; }
            }
            if (edges > cap) return null;
        }
        const DX = [1, 0, -1, 0], DY = [0, 1, 0, -1];
        const loops = [];
        for (let start = 0; start < outs.length; start++) {
            while (outs[start]) {
                let d = 0;
                while (!(outs[start] & (1 << d))) d++;
                let cx = start % vw, cy = (start / vw) | 0;
                const pts = [];
                let prev = -1;
                for (;;) {
                    const v = cy * vw + cx;
                    if (d !== prev) { pts.push(cx + box.x, cy + box.y); prev = d; }
                    outs[v] &= ~(1 << d);
                    cx += DX[d]; cy += DY[d];
                    const next = cy * vw + cx;
                    const bits = outs[next];
                    if (!bits) break;
                    const right = (d + 1) & 3, left = (d + 3) & 3;
                    if (bits & (1 << right)) d = right;
                    else if (bits & (1 << d)) d = d;
                    else if (bits & (1 << left)) d = left;
                    else d = (d + 2) & 3;
                }
                if (pts.length >= 6) loops.push(Int32Array.from(pts));
            }
        }
        return loops;
    }

    Object.assign(kit, {
        clamp, boundsOf, growBox, extractBox, pasteBox, rectMask, ellipseMask, polygonMask, shiftMask, smoothEdges,
        distanceToFeature, signedDistance, bandCoverage, morphRound, ringMask, sharpenBlurred, rgbToLab, floodSelect,
        colorRangeAlpha, createGrower, majority, upscaleBinary, traceContours
    });

    // ===============================================================================================================
    // Window state and shared helpers
    // ===============================================================================================================

    const PROXY_EDGE = 1536;
    const MAX_ANIMATED_VERTICES = 60000;

    function ui(r) {
        return r.selectionUI;
    }

    const tick = () => new Promise(resolve => setTimeout(resolve, 0));

    // Selection commits run one after the other (feather jobs are asynchronous), so a quick second gesture cannot
    // overtake the first one.
    function enqueue(r, task) {
        const u = ui(r);
        u.chain = u.chain.then(task).catch(err => { if (window.console) console.error('pixel selection', err); });
        return u.chain;
    }

    // A selection change is a history step (undo restores it), but selecting alone does not make the file "unsaved".
    function recordSelection(r, label) {
        if (!r.canvas || !r.canvas.width) return;
        const dirty = r.isDirty;
        r.pushHistory(label);
        r.isDirty = dirty;
        r.updateStatus();
    }

    function modeFromPointer(p, base) {
        if (p.shift && p.alt) return 'intersect';
        if (p.shift) return 'add';
        if (p.alt) return 'subtract';
        return base || 'replace';
    }

    function selectionBox(r) {
        const m = r.getSelectionMask();
        return m ? { x: m.x, y: m.y, w: m.w, h: m.h } : null;
    }

    function insideSelection(r, x, y) {
        const m = r.getSelectionMask();
        if (!m) return false;
        const ix = Math.floor(x), iy = Math.floor(y), w = r.canvas.width, h = r.canvas.height;
        if (ix < 0 || iy < 0 || ix >= w || iy >= h) return false;
        return m.alpha[iy * w + ix] >= 128;
    }

    // Soft edge for a document-sized mask. Only the crop around the mask goes through the job (worker).
    async function featherMask(r, alpha, w, h, radius, box) {
        const crop = kit.growBox(box || kit.boundsOf(alpha, w, h, 0) || { x: 0, y: 0, w, h }, Math.ceil(radius * 2) + 2, w, h);
        const part = kit.extractBox(alpha, w, crop);
        let soft;
        try {
            const res = await r.runJob('core.blurAlpha', { alpha: part, width: crop.w, height: crop.h, radius }, { transfer: [part.buffer] });
            soft = res.alpha;
        } catch (err) {
            if (err && err.name === 'AbortError') throw err;
            const lib = window.AuraPixelLib;
            if (!lib || !lib.featherAlpha) throw err;
            soft = lib.featherAlpha(kit.extractBox(alpha, w, crop), crop.w, crop.h, radius);
        }
        return kit.pasteBox(new Uint8ClampedArray(w * h), w, crop, soft);
    }

    // Combines a finished document-sized mask with the selection (mode), feathers it and records the history step.
    async function finishMask(r, alpha, mode, label, feather) {
        if (!alpha || !r.canvas || !r.canvas.width) return false;
        const w = r.canvas.width, h = r.canvas.height;
        if (alpha.length !== w * h) return false;
        const box = kit.boundsOf(alpha, w, h, 0);
        if (!box) {
            if (mode === 'replace' && r.selection) {
                r.deselect();
                recordSelection(r, 'select:none');
            }
            return false;
        }
        if (!r.selection && (mode === 'subtract' || mode === 'intersect')) return false;
        let out = alpha;
        if (feather > 0) out = await featherMask(r, alpha, w, h, feather, box);
        const ok = r.setSelectionMask(out, { mode });
        recordSelection(r, ok ? label : 'select:none');
        return ok;
    }

    function deselectRecorded(r) {
        if (!r.selection) return;
        r.deselect();
        recordSelection(r, 'select:none');
    }

    // --- pixels to look at: the active layer (preview included) or what is on screen ---

    function sampleCanvas(r, all) {
        const w = r.canvas.width, h = r.canvas.height;
        if (all) {
            const canvas = r.makeCanvas(w, h);
            r.renderComposition(canvas.getContext('2d'), r.layers, { previews: true });
            return canvas;
        }
        const layer = r.activeLayer();
        if (r.editMaskMode && layer && layer.maskCanvas) return layer.maskCanvas;
        return (layer && (layer.preview || layer.canvas)) || r.canvas;
    }

    function imageDataOf(canvas) {
        return canvas.getContext('2d', { willReadFrequently: true }).getImageData(0, 0, canvas.width, canvas.height);
    }

    // ===============================================================================================================
    // Marching ants (overlay 'selection', replaces the built-in one)
    // ===============================================================================================================

    function antsStyle(r) {
        const z = r.zoom > 0 ? r.zoom : 1;
        const dash = Math.max(4, 3 / z);
        return { lw: Math.max(1, 1 / z), dash, phase: (now() / 1000 * 30 % 8) / 8 * dash * 2 };
    }

    // White dashes over black dashes shifted by one dash length; trace() builds the path and strokes it.
    function strokeAnts(ctx, r, trace) {
        const s = antsStyle(r);
        ctx.lineWidth = s.lw;
        ctx.setLineDash([s.dash, s.dash]);
        ctx.lineDashOffset = -s.phase;
        ctx.strokeStyle = '#ffffff';
        trace();
        ctx.lineDashOffset = -s.phase + s.dash;
        ctx.strokeStyle = '#000000';
        trace();
        ctx.setLineDash([]);
        ctx.lineDashOffset = 0;
    }

    // Outline loops of a mask selection, cached per mask array (selections are immutable, a move creates a new object
    // that shares the array while it floats). loops === null: too many edges, the built-in border pixels are used.
    function contourFor(r, sel) {
        const u = ui(r);
        const w = r.canvas.width, h = r.canvas.height;
        const cached = u.contour;
        if (cached && cached.alpha === sel.alpha && cached.w === w && cached.h === h) return cached;
        let loops = [];
        if (sel.alpha && sel.alpha.length === w * h) {
            const box = kit.boundsOf(sel.alpha, w, h, 127);
            loops = box ? kit.traceContours(sel.alpha, w, h, box) : [];
        }
        let vertices = 0, path = null;
        if (loops) {
            loops.forEach(loop => { vertices += loop.length / 2; });
            if (typeof Path2D === 'function') {
                path = new Path2D();
                loops.forEach(loop => {
                    path.moveTo(loop[0], loop[1]);
                    for (let i = 2; i < loop.length; i += 2) path.lineTo(loop[i], loop[i + 1]);
                    path.closePath();
                });
            }
        }
        u.contour = { alpha: sel.alpha, w, h, loops, path, vertices };
        return u.contour;
    }

    function traceLoops(ctx, loops) {
        ctx.beginPath();
        for (let k = 0; k < loops.length; k++) {
            const loop = loops[k];
            ctx.moveTo(loop[0], loop[1]);
            for (let i = 2; i < loop.length; i += 2) ctx.lineTo(loop[i], loop[i + 1]);
            ctx.closePath();
        }
        ctx.stroke();
    }

    function drawSelection(r, ctx) {
        const sel = r.selection;
        if (!sel || !ctx) return;
        // Rectangles and ellipses keep the built-in ants (they also show the pixels the move tool has lifted).
        if (sel.type !== 'mask' || !sel.alpha) { r.drawMarchingAnts(ctx); return; }
        const u = ui(r);
        let offX = 0, offY = 0;
        if (r.moveSelFloating) {
            // The built-in move tool lifted the pixels: show them where they are and the outline with them.
            if (r.selImageData) {
                const tmp = r.acquireTempCanvas(r.selImageData.width, r.selImageData.height);
                tmp.getContext('2d').putImageData(r.selImageData, 0, 0);
                ctx.drawImage(tmp, Math.round(sel.x), Math.round(sel.y));
                r.releaseTempCanvas(tmp);
            }
            offX = Math.round(sel.x - r.moveSelOrigX);
            offY = Math.round(sel.y - r.moveSelOrigY);
        } else if (u.moveOffset) {
            offX = u.moveOffset.dx;
            offY = u.moveOffset.dy;
        }
        const contour = contourFor(r, sel);
        if (!contour.loops) { r.drawMarchingAnts(ctx); return; }
        ctx.translate(offX, offY);
        strokeAnts(ctx, r, () => {
            if (contour.path) ctx.stroke(contour.path);
            else traceLoops(ctx, contour.loops);
        });
        if (contour.vertices <= MAX_ANIMATED_VERTICES) r.scheduleMarchingAnts();
    }

    // ===============================================================================================================
    // Options bar
    // ===============================================================================================================

    const MODES = {
        replace: ['pixel.sel_mode_replace', 'New', 'pixel.sel_mode_replace_hint', 'New selection'],
        add: ['pixel.sel_mode_add', 'Add', 'pixel.sel_mode_add_hint', 'Add to the selection (Shift)'],
        subtract: ['pixel.sel_mode_subtract', 'Subtract', 'pixel.sel_mode_subtract_hint', 'Subtract from the selection (Alt)'],
        intersect: ['pixel.sel_mode_intersect', 'Intersect', 'pixel.sel_mode_intersect_hint', 'Intersect with the selection (Shift+Alt)']
    };
    // Options whose change shows or hides other controls.
    const RERENDER = new Set(['style']);

    function segmentedHTML(r, key, list, current, labels) {
        return `<div class="pixel-param-segmented pixel-sel-seg" role="group" data-sel-seg="${r.esc(key)}">${list.map(value => {
            const info = labels[value];
            return `<button type="button" class="pixel-param-seg-btn${value === current ? ' active' : ''}" data-sel-val="${r.esc(value)}" aria-pressed="${value === current ? 'true' : 'false'}" title="${r.esc(r.t(info[2], info[3]))}">${r.esc(r.t(info[0], info[1]))}</button>`;
        }).join('')}</div>`;
    }

    function modeHTML(r, key) {
        const list = key === 'qsMode' ? ['replace', 'add', 'subtract'] : ['replace', 'add', 'subtract', 'intersect'];
        return `<div class="pixel-sel-group">${segmentedHTML(r, key, list, ui(r).opts[key], MODES)}</div>`;
    }

    function sliderHTML(r, key, labelKey, fallback, min, max, unit) {
        const value = ui(r).opts[key];
        return `<div class="pixel-slider-row pixel-sel-slider"><span class="pixel-slider-label">${r.esc(r.t(labelKey, fallback))}</span><input type="range" class="pixel-slider" data-sel-opt="${key}" min="${min}" max="${max}" step="1" value="${value}"><span class="pixel-slider-value" data-sel-out="${key}" data-sel-unit="${r.esc(unit || '')}">${value}${r.esc(unit || '')}</span></div>`;
    }

    function checkHTML(r, key, labelKey, fallback) {
        return `<label class="pixel-checkbox-row pixel-sel-check"><input type="checkbox" data-sel-opt="${key}"${ui(r).opts[key] ? ' checked' : ''}> ${r.esc(r.t(labelKey, fallback))}</label>`;
    }

    function numberHTML(r, key, label, min, max) {
        return `<label class="pixel-sel-num"><span class="pixel-label">${r.esc(label)}</span><input type="number" class="pixel-param-number" data-sel-opt="${key}" min="${min}" max="${max}" step="1" value="${ui(r).opts[key]}"></label>`;
    }

    function deselectButtonHTML(r) {
        return `<button class="pixel-btn pixel-sel-clear" type="button" data-sel-act="deselect">${r.esc(r.t('pixel.clear_selection', 'Deselect'))}</button>`;
    }

    function hintHTML(r, key, fallback) {
        return `<p class="pixel-draw-hint pixel-sel-hint">${r.esc(r.t(key, fallback))}</p>`;
    }

    function styleHTML(r) {
        const o = ui(r).opts;
        const styles = {
            normal: ['pixel.sel_style_normal', 'Normal', 'pixel.sel_style_normal_hint', 'Free size'],
            ratio: ['pixel.sel_style_ratio', 'Ratio', 'pixel.sel_style_ratio_hint', 'Fixed aspect ratio'],
            fixed: ['pixel.sel_style_fixed', 'Size', 'pixel.sel_style_fixed_hint', 'Fixed size in pixels']
        };
        let html = `<div class="pixel-sel-group">${segmentedHTML(r, 'style', ['normal', 'ratio', 'fixed'], o.style, styles)}`;
        if (o.style === 'ratio') html += numberHTML(r, 'ratioW', r.t('pixel.sel_width_short', 'W'), 1, 999) + numberHTML(r, 'ratioH', r.t('pixel.sel_height_short', 'H'), 1, 999);
        if (o.style === 'fixed') html += numberHTML(r, 'sizeW', r.t('pixel.sel_width_short', 'W'), 1, 16384) + numberHTML(r, 'sizeH', r.t('pixel.sel_height_short', 'H'), 1, 16384);
        return html + '</div>';
    }

    // Listeners go on the controls, not on the bar: the bar element survives tool changes, its content does not.
    function wireSelOptions(r, bar, onChange) {
        const o = ui(r).opts;
        bar.querySelectorAll('[data-sel-seg]').forEach(group => {
            const key = group.dataset.selSeg;
            const buttons = Array.from(group.querySelectorAll('[data-sel-val]'));
            buttons.forEach(btn => btn.addEventListener('click', () => {
                o[key] = btn.dataset.selVal;
                buttons.forEach(b => {
                    const on = b === btn;
                    b.classList.toggle('active', on);
                    b.setAttribute('aria-pressed', on ? 'true' : 'false');
                });
                if (RERENDER.has(key)) r.renderOptionsBar();
                if (onChange) onChange(key);
            }));
        });
        bar.querySelectorAll('[data-sel-opt]').forEach(input => {
            const key = input.dataset.selOpt;
            const isCheck = input.type === 'checkbox';
            const isRange = input.type === 'range';
            input.addEventListener(isRange ? 'input' : 'change', () => {
                if (isCheck) o[key] = !!input.checked;
                else if (input.type === 'number' || isRange) o[key] = clamp(Number(input.value) || 0, Number(input.min) || 0, Number(input.max) || 1e9);
                else o[key] = input.value;
                const out = bar.querySelector('[data-sel-out="' + key + '"]');
                if (out) out.textContent = o[key] + (out.dataset.selUnit || '');
                if (onChange) onChange(key);
            });
        });
        bar.querySelectorAll('[data-sel-act]').forEach(btn => btn.addEventListener('click', () => {
            if (btn.dataset.selAct === 'deselect') deselectRecorded(r);
        }));
    }

    // ===============================================================================================================
    // Moving the outline (drag inside the selection, arrow keys)
    // ===============================================================================================================

    function setHoverCursor(r, p) {
        const u = ui(r);
        const o = u.opts;
        const name = r.selection && o.mode === 'replace' && !p.shift && !p.alt && insideSelection(r, p.x, p.y) ? 'move' : '';
        if (u.cursorName === name) return;
        u.cursorName = name;
        const cursor = name || r.getToolCursor();
        if (r.overlayCanvas) r.overlayCanvas.style.cursor = cursor;
        if (r.canvas) r.canvas.style.cursor = cursor;
    }

    function beginMove(r, p) {
        return { kind: 'move', sx: p.x, sy: p.y, dx: 0, dy: 0, sel0: r.selection, moved: false };
    }

    function moveOutline(r, g, p) {
        const dx = Math.round(p.x - g.sx), dy = Math.round(p.y - g.sy);
        if (dx === g.dx && dy === g.dy) return;
        g.dx = dx; g.dy = dy; g.moved = true;
        const sel = g.sel0;
        if (sel.type === 'mask') {
            ui(r).moveOffset = { dx, dy };
            r.requestOverlay();
        } else {
            r.selection = Object.assign({}, sel, { x: sel.x + dx, y: sel.y + dy });
        }
    }

    function endMove(r, g) {
        const u = ui(r);
        u.moveOffset = null;
        if (!g.moved || (!g.dx && !g.dy)) { r.requestOverlay(); return; }
        const sel = g.sel0;
        if (sel.type === 'mask') {
            const w = r.canvas.width, h = r.canvas.height;
            r.setSelectionMask(kit.shiftMask(sel.alpha, w, h, g.dx, g.dy), { mode: 'replace' });
        }
        recordSelection(r, 'select:move');
    }

    // Moves the whole selection by (dx, dy) document pixels.
    function nudgeSelection(r, dx, dy) {
        const sel = r.selection;
        if (!sel) return false;
        if (sel.type === 'mask') {
            const w = r.canvas.width, h = r.canvas.height;
            if (!sel.alpha || sel.alpha.length !== w * h) return false;
            r.setSelectionMask(kit.shiftMask(sel.alpha, w, h, dx, dy), { mode: 'replace' });
        } else {
            r.selection = Object.assign({}, sel, { x: sel.x + dx, y: sel.y + dy });
        }
        const u = ui(r);
        clearTimeout(u.nudgeTimer);
        // Holding an arrow key is one history step, written when the key rests.
        u.nudgeTimer = setTimeout(() => {
            u.nudgeTimer = 0;
            if (!r.state.disposed && r.selection) recordSelection(r, 'select:nudge');
        }, 500);
        return true;
    }

    const ARROWS = { ArrowLeft: [-1, 0], ArrowRight: [1, 0], ArrowUp: [0, -1], ArrowDown: [0, 1] };

    function nudgeKey(r, e) {
        const dir = ARROWS[e.key];
        if (!dir || e.ctrlKey || e.metaKey || e.altKey || !r.selection || ui(r).g) return false;
        const step = e.shiftKey ? 10 : 1;
        return nudgeSelection(r, dir[0] * step, dir[1] * step);
    }

    // ===============================================================================================================
    // Rectangle and ellipse
    // ===============================================================================================================

    // Rectangle of a drag. Shift (pressed after the drag began) constrains to a square, Alt to drawing from the centre.
    function shapeRect(r, g, p) {
        const o = ui(r).opts;
        if (!p.shift) g.shiftUp = true;
        if (!p.alt) g.altUp = true;
        const constrain = p.shift && (g.shiftUp || !g.startShift);
        const centre = p.alt && (g.altUp || !g.startAlt);
        const px = Math.round(p.x), py = Math.round(p.y);
        let x, y, w, h;
        if (o.style === 'fixed') {
            w = Math.max(1, Math.round(o.sizeW));
            h = Math.max(1, Math.round(o.sizeH));
            x = centre ? px - Math.round(w / 2) : px;
            y = centre ? py - Math.round(h / 2) : py;
            return { x, y, w, h };
        }
        let dx = px - g.x0, dy = py - g.y0;
        const sx = dx < 0 ? -1 : 1, sy = dy < 0 ? -1 : 1;
        let aw = Math.abs(dx), ah = Math.abs(dy);
        let ratio = 0;
        if (o.style === 'ratio' && o.ratioW > 0 && o.ratioH > 0) ratio = o.ratioW / o.ratioH;
        else if (constrain) ratio = 1;
        if (ratio) {
            // The shape encloses the dragged box, so a purely horizontal drag still grows it.
            if (aw / ratio >= ah) ah = aw / ratio;
            else aw = ah * ratio;
            aw = Math.round(aw); ah = Math.round(ah);
        }
        if (centre) {
            x = g.x0 - aw; y = g.y0 - ah; w = aw * 2; h = ah * 2;
        } else {
            x = sx < 0 ? g.x0 - aw : g.x0;
            y = sy < 0 ? g.y0 - ah : g.y0;
            w = aw; h = ah;
        }
        return { x, y, w, h };
    }

    function showSelectionSize(r, q) {
        r.setStatus(r.t('pixel.sel_status', 'Selection') + ': ' + q.w + ' × ' + q.h + '  (' + q.x + ', ' + q.y + ')');
    }

    async function commitShape(r, kind, q, mode) {
        const o = ui(r).opts;
        const w = r.canvas.width, h = r.canvas.height;
        const label = kind === 'select-rect' ? 'select:rect' : 'select:ellipse';
        let alpha;
        if (kind === 'select-rect') {
            const x0 = clamp(q.x, 0, w), y0 = clamp(q.y, 0, h), x1 = clamp(q.x + q.w, 0, w), y1 = clamp(q.y + q.h, 0, h);
            if (x1 <= x0 || y1 <= y0) {
                if (mode === 'replace' && r.selection) deselectRecorded(r);
                return;
            }
            if (mode === 'replace' && !(o.feather > 0)) {
                r.selection = { type: 'rect', x: x0, y: y0, w: x1 - x0, h: y1 - y0 };
                r.startMarchingAnts();
                recordSelection(r, label);
                return;
            }
            alpha = kit.rectMask(w, h, q.x, q.y, q.w, q.h);
        } else {
            if (mode === 'replace' && !(o.feather > 0) && o.antialias) {
                if (q.x >= w || q.y >= h || q.x + q.w <= 0 || q.y + q.h <= 0) {
                    if (r.selection) deselectRecorded(r);
                    return;
                }
                r.selection = { type: 'ellipse', x: q.x, y: q.y, w: q.w, h: q.h };
                r.startMarchingAnts();
                recordSelection(r, label);
                return;
            }
            alpha = kit.ellipseMask(w, h, q.x, q.y, q.w, q.h, o.antialias);
        }
        await finishMask(r, alpha, mode, label, o.feather);
    }

    function shapeTool(kind) {
        const isRect = kind === 'select-rect';
        return {
            id: kind,
            options(r) {
                return modeHTML(r, 'mode') + styleHTML(r) + sliderHTML(r, 'feather', 'pixel.sel_feather', 'Feather', 0, 100, 'px') +
                    (isRect ? '' : checkHTML(r, 'antialias', 'pixel.sel_antialias', 'Anti-alias')) + deselectButtonHTML(r);
            },
            wireOptions(r, bar) { wireSelOptions(r, bar); },
            onActivate(r) { ui(r).cursorName = ''; },
            onDeactivate(r) { ui(r).g = null; r.requestOverlay(); },
            onHover(r, e, p) { setHoverCursor(r, p); },
            onDown(r, e, p) {
                const u = ui(r);
                u.g = null;
                if (e.button !== 0) return;
                if (r.selection && u.opts.mode === 'replace' && !p.shift && !p.alt && insideSelection(r, p.x, p.y)) {
                    u.g = beginMove(r, p);
                    return;
                }
                const g = { kind: 'shape', x0: Math.round(p.x), y0: Math.round(p.y), mode: modeFromPointer(p, u.opts.mode), startShift: p.shift, startAlt: p.alt, shiftUp: false, altUp: false, rect: null, moved: false };
                g.rect = shapeRect(r, g, p);
                u.g = g;
                showSelectionSize(r, g.rect);
                r.requestOverlay();
            },
            onMove(r, e, p) {
                const g = ui(r).g;
                if (!g) return;
                if (g.kind === 'move') { moveOutline(r, g, p); return; }
                g.rect = shapeRect(r, g, p);
                g.moved = true;
                showSelectionSize(r, g.rect);
                r.requestOverlay();
            },
            onUp(r, e, p) {
                const u = ui(r);
                const g = u.g;
                u.g = null;
                if (!g) return;
                if (g.kind === 'move') { endMove(r, g); return; }
                const q = shapeRect(r, g, p);
                r.requestOverlay();
                r.updateStatus();
                if (q.w < 1 || q.h < 1) {
                    // A plain click: the marquee tools deselect (add / subtract modes leave the selection alone).
                    if (g.mode === 'replace' && r.selection) deselectRecorded(r);
                    return;
                }
                enqueue(r, () => commitShape(r, kind, q, g.mode));
            },
            onKey(r, e) { return nudgeKey(r, e); },
            overlay(ctx, r) {
                const g = ui(r).g;
                if (!g || g.kind !== 'shape' || !g.rect) return;
                const q = g.rect;
                strokeAnts(ctx, r, () => {
                    ctx.beginPath();
                    if (isRect) ctx.rect(q.x, q.y, q.w, q.h);
                    else ctx.ellipse(q.x + q.w / 2, q.y + q.h / 2, Math.abs(q.w / 2), Math.abs(q.h / 2), 0, 0, Math.PI * 2);
                    ctx.stroke();
                });
            }
        };
    }

    // ===============================================================================================================
    // Lasso and polygon lasso
    // ===============================================================================================================

    async function commitPolygon(r, pts, mode, label) {
        const o = ui(r).opts;
        const alpha = kit.polygonMask(r.canvas.width, r.canvas.height, pts, o.antialias);
        if (!alpha) {
            if (mode === 'replace' && r.selection) deselectRecorded(r);
            return;
        }
        await finishMask(r, alpha, mode, label, o.feather);
    }

    function tracePolyline(ctx, pts, closeBack, extra) {
        ctx.beginPath();
        ctx.moveTo(pts[0].x, pts[0].y);
        for (let i = 1; i < pts.length; i++) ctx.lineTo(pts[i].x, pts[i].y);
        if (extra) ctx.lineTo(extra.x, extra.y);
        if (closeBack) ctx.closePath();
        ctx.stroke();
    }

    function lassoTool() {
        return {
            id: 'lasso',
            options(r) {
                return modeHTML(r, 'mode') + sliderHTML(r, 'feather', 'pixel.sel_feather', 'Feather', 0, 100, 'px') +
                    checkHTML(r, 'antialias', 'pixel.sel_antialias', 'Anti-alias') + deselectButtonHTML(r);
            },
            wireOptions(r, bar) { wireSelOptions(r, bar); },
            onActivate(r) { ui(r).cursorName = ''; },
            onDeactivate(r) { ui(r).g = null; r.requestOverlay(); },
            onHover(r, e, p) { setHoverCursor(r, p); },
            onDown(r, e, p) {
                const u = ui(r);
                u.g = null;
                if (e.button !== 0) return;
                if (r.selection && u.opts.mode === 'replace' && !p.shift && !p.alt && insideSelection(r, p.x, p.y)) {
                    u.g = beginMove(r, p);
                    return;
                }
                u.g = { kind: 'lasso', pts: [{ x: p.x, y: p.y }], mode: modeFromPointer(p, u.opts.mode), farthest: 0 };
                r.requestOverlay();
            },
            onMove(r, e, p) {
                const g = ui(r).g;
                if (!g) return;
                if (g.kind === 'move') { moveOutline(r, g, p); return; }
                const last = g.pts[g.pts.length - 1];
                if ((p.x - last.x) * (p.x - last.x) + (p.y - last.y) * (p.y - last.y) < 0.5) return;
                g.pts.push({ x: p.x, y: p.y });
                g.farthest = Math.max(g.farthest, Math.hypot(p.x - g.pts[0].x, p.y - g.pts[0].y));
                r.requestOverlay();
            },
            onUp(r, e, p) {
                const u = ui(r);
                const g = u.g;
                u.g = null;
                r.requestOverlay();
                if (!g) return;
                if (g.kind === 'move') { endMove(r, g); return; }
                if (g.pts.length < 3 || g.farthest < 3) {
                    if (g.mode === 'replace' && r.selection) deselectRecorded(r);
                    return;
                }
                enqueue(r, () => commitPolygon(r, g.pts, g.mode, 'select:lasso'));
            },
            onKey(r, e) { return nudgeKey(r, e); },
            overlay(ctx, r) {
                const g = ui(r).g;
                if (!g || g.kind !== 'lasso' || g.pts.length < 2) return;
                strokeAnts(ctx, r, () => tracePolyline(ctx, g.pts, true));
            }
        };
    }

    function constrainAngle(from, to) {
        const dx = to.x - from.x, dy = to.y - from.y;
        const len = Math.hypot(dx, dy);
        if (!len) return to;
        const step = Math.PI / 4;
        const ang = Math.round(Math.atan2(dy, dx) / step) * step;
        return { x: from.x + len * Math.cos(ang), y: from.y + len * Math.sin(ang) };
    }

    function polygonTool() {
        const closeRadius = r => 8 / Math.max(0.1, r.zoom || 1);
        const point = (r, p, poly) => {
            const pt = { x: p.x, y: p.y };
            return p.shift && poly && poly.pts.length ? constrainAngle(poly.pts[poly.pts.length - 1], pt) : pt;
        };
        const finish = (r, poly) => {
            const u = ui(r);
            if (u.poly !== poly) return;
            u.poly = null;
            r.requestOverlay();
            if (poly.pts.length >= 3) enqueue(r, () => commitPolygon(r, poly.pts, poly.mode, 'select:polygon'));
            r.updateStatus();
        };
        const cancel = r => {
            const u = ui(r);
            if (!u.poly) return;
            u.poly = null;
            r.requestOverlay();
            r.updateStatus();
        };
        return {
            id: 'polygon-lasso',
            options(r) {
                return modeHTML(r, 'mode') + sliderHTML(r, 'feather', 'pixel.sel_feather', 'Feather', 0, 100, 'px') +
                    checkHTML(r, 'antialias', 'pixel.sel_antialias', 'Anti-alias') + deselectButtonHTML(r) +
                    hintHTML(r, 'pixel.sel_polygon_hint', 'Click to add points. Double-click or Enter closes, Backspace removes the last point, Escape cancels, Shift snaps to 45 degrees.');
            },
            wireOptions(r, bar) { wireSelOptions(r, bar); },
            onActivate(r) { ui(r).cursorName = ''; },
            onDeactivate(r) {
                const u = ui(r);
                if (u.poly) finish(r, u.poly);
            },
            onDown(r, e, p) {
                const u = ui(r);
                if (e.button !== 0) return;
                if (!u.poly) u.poly = { pts: [], mode: modeFromPointer(p, u.opts.mode), hover: null, downAt: 0 };
                const poly = u.poly;
                const pt = point(r, p, poly);
                const stamp = now();
                const first = poly.pts[0], last = poly.pts[poly.pts.length - 1];
                const double = poly.pts.length >= 2 && stamp - poly.downAt < 400 && Math.hypot(pt.x - last.x, pt.y - last.y) < closeRadius(r);
                poly.downAt = stamp;
                if (poly.pts.length >= 3 && (double || Math.hypot(pt.x - first.x, pt.y - first.y) <= closeRadius(r))) {
                    finish(r, poly);
                    return;
                }
                if (double) return;
                poly.pts.push(pt);
                poly.hover = pt;
                r.requestOverlay();
            },
            onMove(r, e, p) {
                const poly = ui(r).poly;
                if (poly) { poly.hover = point(r, p, poly); r.requestOverlay(); }
            },
            onHover(r, e, p) {
                const poly = ui(r).poly;
                if (poly) { poly.hover = point(r, p, poly); r.requestOverlay(); return; }
                setHoverCursor(r, p);
            },
            onKey(r, e) {
                const u = ui(r);
                const poly = u.poly;
                if (!poly) return nudgeKey(r, e);
                if (e.key === 'Enter') { finish(r, poly); return true; }
                if (e.key === 'Escape') { cancel(r); return true; }
                if (e.key === 'Backspace' || e.key === 'Delete') {
                    poly.pts.pop();
                    if (!poly.pts.length) cancel(r);
                    else r.requestOverlay();
                    return true;
                }
                return false;
            },
            overlay(ctx, r) {
                const poly = ui(r).poly;
                if (!poly || !poly.pts.length) return;
                const pts = poly.pts;
                const tip = pts[pts.length - 1];
                const hover = poly.hover && (poly.hover.x !== tip.x || poly.hover.y !== tip.y) ? poly.hover : null;
                if (pts.length > 1 || hover) strokeAnts(ctx, r, () => tracePolyline(ctx, pts, false, hover));
                const size = Math.max(3, 4 / Math.max(0.1, r.zoom || 1));
                ctx.fillStyle = '#ffffff';
                ctx.strokeStyle = '#000000';
                ctx.lineWidth = Math.max(1, 1 / Math.max(0.1, r.zoom || 1));
                pts.forEach((pt, i) => {
                    const s = i === 0 ? size * 1.4 : size;
                    ctx.fillRect(pt.x - s / 2, pt.y - s / 2, s, s);
                    ctx.strokeRect(pt.x - s / 2, pt.y - s / 2, s, s);
                });
            }
        };
    }

    // ===============================================================================================================
    // Magic wand
    // ===============================================================================================================

    function wandTool() {
        return {
            id: 'magic-wand',
            options(r) {
                return modeHTML(r, 'mode') + sliderHTML(r, 'tolerance', 'pixel.tolerance', 'Tolerance', 0, 255, '') +
                    checkHTML(r, 'contiguous', 'pixel.sel_contiguous', 'Contiguous') + checkHTML(r, 'sampleAll', 'pixel.sel_sample_all', 'Sample all layers') +
                    checkHTML(r, 'antialias', 'pixel.sel_antialias', 'Anti-alias') + sliderHTML(r, 'feather', 'pixel.sel_feather', 'Feather', 0, 100, 'px') +
                    deselectButtonHTML(r);
            },
            wireOptions(r, bar) { wireSelOptions(r, bar); },
            onKey(r, e) { return nudgeKey(r, e); },
            onDown(r, e, p) {
                if (e.button !== 0) return;
                const u = ui(r), o = u.opts;
                const w = r.canvas.width, h = r.canvas.height;
                const x = Math.floor(p.x), y = Math.floor(p.y);
                if (x < 0 || y < 0 || x >= w || y >= h) return;
                const mode = modeFromPointer(p, o.mode);
                const settings = { tolerance: o.tolerance, contiguous: o.contiguous, sampleAll: o.sampleAll, antialias: o.antialias, feather: o.feather };
                enqueue(r, async () => {
                    const img = imageDataOf(sampleCanvas(r, settings.sampleAll));
                    const alpha = kit.floodSelect(img.data, img.width, img.height, x, y, settings.tolerance, settings.contiguous);
                    if (settings.antialias) kit.smoothEdges(alpha, img.width, img.height);
                    await finishMask(r, alpha, mode, 'select:wand', settings.feather);
                });
            }
        };
    }

    // ===============================================================================================================
    // Quick selection brush
    // ===============================================================================================================

    function qsBegin(r, p) {
        const o = ui(r).opts;
        const w = r.canvas.width, h = r.canvas.height;
        const src = sampleCanvas(r, o.qsSampleAll);
        const scale = Math.max(w, h) > PROXY_EDGE ? Math.max(w, h) / PROXY_EDGE : 1;
        const pw = Math.max(1, Math.round(w / scale)), ph = Math.max(1, Math.round(h / scale));
        let data;
        if (scale === 1) {
            data = imageDataOf(src).data;
        } else {
            const tmp = r.makeCanvas(pw, ph);
            const tctx = tmp.getContext('2d');
            tctx.imageSmoothingQuality = 'high';
            tctx.drawImage(src, 0, 0, pw, ph);
            data = imageDataOf(tmp).data;
        }
        const preview = r.makeCanvas(pw, ph);
        return {
            kind: 'qs', pw, ph, sx: pw / w, sy: ph / h, last: null, timer: 0, dead: false,
            mode: modeFromPointer(p, o.qsMode),
            grower: kit.createGrower(data, pw, ph, { tolerance: o.qsTolerance, barrier: 4 + (100 - o.qsEdge) * 0.2 }),
            preview, pctx: preview.getContext('2d')
        };
    }

    // Paints the newly grown pixels into the preview canvas (only the changed box).
    function qsFlush(g) {
        const box = g.grower.drainDirty();
        if (!box) return;
        const img = g.pctx.createImageData(box.w, box.h);
        const st = g.grower.state;
        const subtract = g.mode === 'subtract';
        for (let y = 0; y < box.h; y++) {
            const row = (box.y + y) * g.pw + box.x;
            for (let x = 0; x < box.w; x++) {
                const s = st[row + x];
                if (s !== 1 && s !== 3) continue;
                const i = (y * box.w + x) * 4;
                img.data[i] = subtract ? 255 : 60;
                img.data[i + 1] = subtract ? 70 : 170;
                img.data[i + 2] = subtract ? 70 : 255;
                img.data[i + 3] = 105;
            }
        }
        g.pctx.putImageData(img, box.x, box.y);
    }

    function qsSchedule(r, g) {
        if (g.timer || g.dead) return;
        g.timer = setTimeout(() => {
            g.timer = 0;
            if (g.dead || r.state.disposed) return;
            const done = g.grower.grow(12);
            qsFlush(g);
            r.requestOverlay();
            if (!done) qsSchedule(r, g);
        }, 0);
    }

    function qsStroke(r, g, p) {
        const o = ui(r).opts;
        const rad = Math.max(1, o.qsSize / 2);
        const from = g.last || p;
        const step = Math.max(1, rad * 0.4);
        const count = Math.max(1, Math.ceil(Math.hypot(p.x - from.x, p.y - from.y) / step));
        for (let i = 1; i <= count; i++) {
            const t = i / count;
            g.grower.seedDisc((from.x + (p.x - from.x) * t) * g.sx, (from.y + (p.y - from.y) * t) * g.sy, rad * g.sx);
        }
        g.last = { x: p.x, y: p.y };
        const done = g.grower.grow(8);
        qsFlush(g);
        r.requestOverlay();
        if (!done) qsSchedule(r, g);
    }

    async function qsFinish(r, g) {
        const w = r.canvas.width, h = r.canvas.height;
        g.dead = true;
        clearTimeout(g.timer);
        try {
            while (!g.grower.grow(20)) await tick();
            const bin = kit.majority(g.grower.binary(), g.pw, g.ph);
            const alpha = kit.upscaleBinary(bin, g.pw, g.ph, w, h);
            await finishMask(r, alpha, g.mode, 'select:quick', 0);
        } finally {
            const u = ui(r);
            if (u.pending === g) u.pending = null;
            r.requestOverlay();
        }
    }

    function quickSelectTool() {
        return {
            id: 'quick-select',
            group: 'select',
            order: 35,
            svg: '<svg width="16" height="16" viewBox="0 0 16 16"><circle cx="6.5" cy="9.5" r="4.2" fill="none" stroke="currentColor" stroke-width="1.4" stroke-dasharray="2 2"/><path d="M12.2 1.8l.7 1.8 1.8.7-1.8.7-.7 1.8-.7-1.8-1.8-.7 1.8-.7z" fill="currentColor"/></svg>',
            shortcut: 'Shift+W',
            labelKey: 'pixel.quick_select',
            fallback: 'Quick selection',
            raster: false,
            cursor: 'crosshair',
            options(r) {
                return modeHTML(r, 'qsMode') + sliderHTML(r, 'qsSize', 'pixel.sel_brush_size', 'Size', 3, 200, 'px') +
                    sliderHTML(r, 'qsTolerance', 'pixel.tolerance', 'Tolerance', 4, 80, '') +
                    sliderHTML(r, 'qsEdge', 'pixel.sel_edge_detection', 'Edge detection', 0, 100, '%') +
                    checkHTML(r, 'qsSampleAll', 'pixel.sel_sample_all', 'Sample all layers') + deselectButtonHTML(r) +
                    hintHTML(r, 'pixel.sel_quick_hint', 'Paint over the object. Alt paints away from the selection.');
            },
            wireOptions(r, bar) { wireSelOptions(r, bar); },
            onActivate(r) { ui(r).hover = null; },
            onDeactivate(r) { const u = ui(r); u.hover = null; r.requestOverlay(); },
            onHover(r, e, p) {
                ui(r).hover = { x: p.x, y: p.y };
                r.requestOverlay();
            },
            onDown(r, e, p) {
                const u = ui(r);
                if (e.button !== 0 || u.pending) return;
                u.g = qsBegin(r, p);
                qsStroke(r, u.g, p);
            },
            onMove(r, e, p) {
                const u = ui(r);
                u.hover = { x: p.x, y: p.y };
                if (u.g && u.g.kind === 'qs') qsStroke(r, u.g, p);
            },
            onUp(r, e, p) {
                const u = ui(r);
                const g = u.g;
                if (!g || g.kind !== 'qs') return;
                u.g = null;
                u.pending = g;
                qsStroke(r, g, p);
                enqueue(r, () => qsFinish(r, g));
            },
            onKey(r, e) { return nudgeKey(r, e); },
            overlay(ctx, r) {
                const u = ui(r);
                const g = u.g || u.pending;
                if (g && g.kind === 'qs') ctx.drawImage(g.preview, 0, 0, r.canvas.width, r.canvas.height);
                if (u.hover) {
                    const rad = Math.max(1, u.opts.qsSize / 2);
                    const lw = Math.max(1, 1 / Math.max(0.1, r.zoom || 1));
                    ctx.lineWidth = lw * 2;
                    ctx.strokeStyle = 'rgba(0,0,0,0.7)';
                    ctx.beginPath();
                    ctx.arc(u.hover.x, u.hover.y, rad + lw, 0, Math.PI * 2);
                    ctx.stroke();
                    ctx.lineWidth = lw;
                    ctx.strokeStyle = 'rgba(255,255,255,0.95)';
                    ctx.beginPath();
                    ctx.arc(u.hover.x, u.hover.y, rad, 0, Math.PI * 2);
                    ctx.stroke();
                }
            }
        };
    }

    // ===============================================================================================================
    // Small value dialogs (grow, shrink, border, smooth, feather, stroke)
    // ===============================================================================================================

    // spec: { title, ok, fields: [{ key, label, type: 'number' | 'select' | 'color', min, max, step, value, options }], onSubmit(values) }
    function askValues(r, spec) {
        const u = ui(r);
        if (u.dialog) { u.dialog.remove(); u.dialog = null; }
        const dlg = document.createElement('div');
        dlg.className = 'vd-modal-backdrop pixel-sel-dialog-backdrop';
        const fieldHTML = f => {
            const id = 'pixel-sel-field-' + f.key;
            const label = `<label class="pixel-label" for="${id}">${r.esc(f.label)}</label>`;
            if (f.type === 'select') {
                return label + `<select class="vd-modal-input" id="${id}" data-sel-field="${f.key}">${f.options.map(o => `<option value="${r.esc(o.value)}"${o.value === f.value ? ' selected' : ''}>${r.esc(o.label)}</option>`).join('')}</select>`;
            }
            if (f.type === 'color') return label + `<input class="vd-modal-input" type="color" id="${id}" data-sel-field="${f.key}" value="${r.esc(f.value)}">`;
            return label + `<input class="vd-modal-input" type="number" id="${id}" data-sel-field="${f.key}" min="${f.min}" max="${f.max}" step="${f.step || 1}" value="${r.esc(f.value)}">`;
        };
        dlg.innerHTML = `<form class="vd-modal pixel-sel-dialog" role="dialog">
            <div class="vd-modal-title">${r.esc(spec.title)}</div>
            <div class="pixel-resize-form">${spec.fields.map(fieldHTML).join('')}</div>
            <div class="vd-modal-actions">
                <button type="button" class="vd-button" data-sel-cancel>${r.esc(r.t('pixel.cancel', 'Cancel'))}</button>
                <button type="submit" class="vd-button vd-button-primary">${r.esc(spec.ok || r.t('pixel.apply', 'Apply'))}</button>
            </div>
        </form>`;
        document.body.appendChild(dlg);
        u.dialog = dlg;
        const close = () => {
            dlg.remove();
            if (u.dialog === dlg) u.dialog = null;
        };
        dlg.querySelector('form').addEventListener('submit', ev => {
            ev.preventDefault();
            const values = {};
            spec.fields.forEach(f => {
                const el = dlg.querySelector('[data-sel-field="' + f.key + '"]');
                let v = el ? el.value : f.value;
                if (!f.type || f.type === 'number') {
                    v = Number(v);
                    v = Number.isFinite(v) ? clamp(v, f.min, f.max) : f.value;
                }
                values[f.key] = v;
            });
            close();
            spec.onSubmit(values);
        });
        dlg.querySelector('[data-sel-cancel]').addEventListener('click', close);
        dlg.addEventListener('click', ev => { if (ev.target === dlg) close(); });
        dlg.addEventListener('keydown', ev => {
            if (ev.key === 'Escape') { ev.preventDefault(); close(); }
        });
        const first = dlg.querySelector('[data-sel-field]');
        if (first && first.focus) first.focus();
        return dlg;
    }

    // ===============================================================================================================
    // Modify: grow, shrink, border, feather, smooth
    // ===============================================================================================================

    const MORPH_PIXEL_LIMIT = 8000000;

    async function blurPart(r, part, w, h, radius) {
        try {
            const res = await r.runJob('core.blurAlpha', { alpha: part, width: w, height: h, radius }, { transfer: [part.buffer] });
            return res.alpha;
        } catch (err) {
            if ((err && err.name === 'AbortError') || !window.AuraPixelLib) throw err;
            return window.AuraPixelLib.blurAlpha(part, w, h, radius);
        }
    }

    async function morphPart(r, part, w, h, radius, mode) {
        const res = await r.runJob('core.morph', { alpha: part, width: w, height: h, radius: Math.max(1, Math.round(radius)), mode }, { transfer: [part.buffer] });
        return res.alpha;
    }

    // kind: grow | shrink | border | feather | smooth. Returns true when the selection changed.
    async function modifySelection(r, kind, amount) {
        const cur = r.getSelectionMask();
        const radius = Math.max(0, Number(amount) || 0);
        if (!cur || !radius) return false;
        const w = r.canvas.width, h = r.canvas.height;
        const rev = r.selectionRev;
        const alpha = cur.alpha;
        const box = kit.boundsOf(alpha, w, h, 0);
        if (!box) return false;
        r.setStatus(r.t('pixel.sel_working', 'Working…'));
        await tick();
        let out;
        if (kind === 'feather' || kind === 'smooth') {
            const crop = kit.growBox(box, Math.ceil(radius * 2) + 2, w, h);
            let soft = await blurPart(r, kit.extractBox(alpha, w, crop), crop.w, crop.h, radius);
            if (kind === 'smooth') soft = kit.sharpenBlurred(soft, radius);
            out = kit.pasteBox(new Uint8ClampedArray(w * h), w, crop, soft);
        } else {
            const crop = kit.growBox(box, Math.ceil(radius) + 3, w, h);
            const part = kit.extractBox(alpha, w, crop);
            let result;
            if (crop.w * crop.h <= MORPH_PIXEL_LIMIT) {
                result = kind === 'border' ? kit.ringMask(part, crop.w, crop.h, radius, 'center') : kit.morphRound(part, crop.w, crop.h, radius, kind);
            } else if (kind === 'border') {
                // Too big for the exact distance transform: square elements in the worker.
                const half = Math.max(1, Math.round(radius / 2));
                const wide = await morphPart(r, Uint8ClampedArray.from(part), crop.w, crop.h, half, 'dilate');
                const thin = await morphPart(r, Uint8ClampedArray.from(part), crop.w, crop.h, half, 'erode');
                result = new Uint8ClampedArray(part.length);
                for (let i = 0; i < result.length; i++) result[i] = Math.max(0, wide[i] - thin[i]);
            } else {
                result = await morphPart(r, part, crop.w, crop.h, radius, kind === 'grow' ? 'dilate' : 'erode');
            }
            out = kit.pasteBox(new Uint8ClampedArray(w * h), w, crop, result);
        }
        if (r.selectionRev !== rev) { r.updateStatus(); return false; }
        const ok = r.setSelectionMask(out, { mode: 'replace' });
        recordSelection(r, ok ? 'select:' + kind : 'select:none');
        if (!ok) r.notify({ type: 'info', message: r.t('pixel.sel_empty', 'The selection is empty now.') });
        return ok;
    }

    const MODIFY_SPECS = {
        grow: { key: 'pixel.sel_grow_title', title: 'Grow selection', labelKey: 'pixel.sel_grow_by', label: 'Grow by (px)', min: 1, max: 250, step: 1 },
        shrink: { key: 'pixel.sel_shrink_title', title: 'Shrink selection', labelKey: 'pixel.sel_shrink_by', label: 'Shrink by (px)', min: 1, max: 250, step: 1 },
        border: { key: 'pixel.sel_border_title', title: 'Selection border', labelKey: 'pixel.sel_border_width', label: 'Border width (px)', min: 1, max: 200, step: 1 },
        smooth: { key: 'pixel.sel_smooth_title', title: 'Smooth selection', labelKey: 'pixel.sel_smooth_radius', label: 'Smoothing radius (px)', min: 1, max: 100, step: 1 },
        feather: { key: 'pixel.sel_feather_title', title: 'Feather selection', labelKey: 'pixel.sel_feather_radius', label: 'Feather radius (px)', min: 0.5, max: 250, step: 0.5 }
    };

    function showModifyDialog(r, kind) {
        const spec = MODIFY_SPECS[kind];
        if (!spec || !r.selection) return;
        const u = ui(r);
        askValues(r, {
            title: r.t(spec.key, spec.title),
            fields: [{ key: 'amount', label: r.t(spec.labelKey, spec.label), min: spec.min, max: spec.max, step: spec.step, value: u.modifyValues[kind] }],
            onSubmit: values => {
                u.modifyValues[kind] = values.amount;
                enqueue(r, () => modifySelection(r, kind, values.amount));
            }
        });
    }

    // ===============================================================================================================
    // Select all / deselect / reselect / inverse, select subject, selection <-> layers, fill, stroke
    // ===============================================================================================================

    function selectAllRecorded(r) {
        if (!r.canvas || !r.canvas.width) return;
        r.selectAll();
        recordSelection(r, 'select:all');
    }

    function reselect(r) {
        const u = ui(r);
        const last = u.lastSel;
        const w = r.canvas.width, h = r.canvas.height;
        if (!last || !w) return false;
        if (last.type === 'mask' && (!last.alpha || last.alpha.length !== w * h)) return false;
        r.selection = last;
        r.startMarchingAnts();
        recordSelection(r, 'select:reselect');
        return true;
    }

    function invertRecorded(r) {
        if (!r.selection) return;
        if (r.invertSelection()) recordSelection(r, 'select:invert');
        else recordSelection(r, 'select:none');
    }

    async function selectSubject(r) {
        if (typeof r.computeSubjectMask !== 'function' || !r.canvas.width) return false;
        const w = r.canvas.width, h = r.canvas.height;
        r.setStatus(r.t('pixel.sel_subject_working', 'Detecting the subject…'));
        try {
            const res = await r.computeSubjectMask({ engine: 'auto' });
            const alpha = res && res.alpha;
            if (!alpha || alpha.length !== w * h) throw new Error(r.t('pixel.sel_subject_failed', 'The subject could not be detected.'));
            return await finishMask(r, alpha instanceof Uint8ClampedArray ? alpha : Uint8ClampedArray.from(alpha), 'replace', 'select:subject', 0);
        } catch (err) {
            if (!err || err.name !== 'AbortError') r.notify({ type: 'error', message: (err && err.message) || r.t('pixel.sel_subject_failed', 'The subject could not be detected.') });
            return false;
        } finally {
            r.updateStatus();
        }
    }

    // The layer's alpha (times its enabled mask) as a document-sized mask.
    function layerAlphaMask(r, layer) {
        const w = r.canvas.width, h = r.canvas.height;
        const alpha = new Uint8ClampedArray(w * h);
        if (!layer || !layer.canvas || !layer.canvas.width) return alpha;
        const data = imageDataOf(layer.canvas).data;
        for (let p = 0, i = 3; p < alpha.length; p++, i += 4) alpha[p] = data[i];
        if (layer.maskCanvas && layer.maskEnabled && layer.maskCanvas.width) {
            const m = imageDataOf(layer.maskCanvas).data;
            for (let p = 0, i = 3; p < alpha.length; p++, i += 4) alpha[p] = alpha[p] * m[i] / 255;
        }
        return alpha;
    }

    function selectFromLayerAlpha(r, layer, mode) {
        const target = layer || r.activeLayer();
        if (!target || !r.canvas.width) return Promise.resolve(false);
        return enqueue(r, () => finishMask(r, layerAlphaMask(r, target), mode || 'replace', 'select:layer-alpha', 0));
    }

    function selectionToLayerMask(r) {
        const sel = r.getSelectionMask();
        const layer = r.activeLayer();
        if (!sel || !layer) return false;
        r.commitPendingPreviews();
        const canvas = r.makeCanvas(r.canvas.width, r.canvas.height);
        const ctx = canvas.getContext('2d');
        const img = ctx.createImageData(sel.w, sel.h);
        const d = img.data, w = r.canvas.width;
        for (let y = 0; y < sel.h; y++) {
            const row = (sel.y + y) * w + sel.x;
            for (let x = 0; x < sel.w; x++) {
                const i = (y * sel.w + x) * 4;
                d[i] = d[i + 1] = d[i + 2] = 255;
                d[i + 3] = sel.alpha[row + x];
            }
        }
        ctx.putImageData(img, sel.x, sel.y);
        layer.maskCanvas = canvas;
        layer.maskEnabled = true;
        r.editMaskMode = false;
        r.markLayerDirty(layer, { mask: true });
        r.compositeLayers();
        r.refreshLayerPanel();
        r.pushHistory('select:to-mask');
        return true;
    }

    // New layer above the active one with the selected pixels (cut: they are removed from the source layer).
    function layerFromSelection(r, cut) {
        const sel = r.getSelectionMask();
        const src = r.activeLayer();
        if (!sel || !src || !r.requireRasterLayer(src)) return false;
        if (r.layers.length >= r.MAX_LAYERS) {
            r.notify({ type: 'error', message: r.t('pixel.max_layers', 'Maximum number of layers reached.') });
            return false;
        }
        r.commitPendingPreviews();
        const w = r.canvas.width, h = r.canvas.height;
        const sctx = src.canvas.getContext('2d', { willReadFrequently: true });
        const picked = sctx.getImageData(sel.x, sel.y, sel.w, sel.h);
        const d = picked.data;
        for (let y = 0; y < sel.h; y++) {
            const row = (sel.y + y) * w + sel.x;
            for (let x = 0; x < sel.w; x++) {
                const i = (y * sel.w + x) * 4 + 3;
                d[i] = d[i] * sel.alpha[row + x] / 255;
            }
        }
        const copy = r.makeCanvas(w, h);
        copy.getContext('2d').putImageData(picked, sel.x, sel.y);
        if (cut) {
            const cleared = r.copyCanvas(src.canvas);
            const cctx = cleared.getContext('2d', { willReadFrequently: true });
            const rest = cctx.getImageData(sel.x, sel.y, sel.w, sel.h);
            const rd = rest.data;
            for (let y = 0; y < sel.h; y++) {
                const row = (sel.y + y) * w + sel.x;
                for (let x = 0; x < sel.w; x++) {
                    const i = (y * sel.w + x) * 4 + 3;
                    rd[i] = rd[i] * (255 - sel.alpha[row + x]) / 255;
                }
            }
            cctx.putImageData(rest, sel.x, sel.y);
            if (!r.commitLayerPixels(src, cleared, { respectSelection: false, history: false })) return false;
        }
        const index = r.activeLayerIdx;
        const layer = r.createLayer({ name: r.t(cut ? 'pixel.sel_layer_cut' : 'pixel.sel_layer_copy', cut ? 'Selection cut' : 'Selection copy'), canvas: copy });
        r.layers.splice(index + 1, 0, layer);
        r.activeLayerIdx = index + 1;
        r.editMaskMode = false;
        r.layersStructureChanged();
        r.pushHistory(cut ? 'select:cut-to-layer' : 'select:copy-to-layer');
        return true;
    }

    // Ctrl+J belongs to the layers package: use its command when it is there.
    function copyToNewLayer(r) {
        const cmd = r.listCommands().find(c => c.shortcuts.indexOf('ctrl+j') >= 0 && c.source !== 'tool' && c.source !== 'panel');
        if (cmd && !(cmd.enabled && cmd.enabled(r) === false)) { cmd.run(r); return true; }
        return layerFromSelection(r, false);
    }

    function fillSelection(r, which) {
        if (!r.canvas || !r.canvas.width || !r.canPaintActiveLayer('fill')) return false;
        const canvas = r.makeCanvas(r.canvas.width, r.canvas.height);
        const ctx = canvas.getContext('2d');
        ctx.fillStyle = which === 'secondary' ? r.secondaryColor : r.primaryColor;
        ctx.fillRect(0, 0, canvas.width, canvas.height);
        return r.commitLayerPixels(r.activeLayer(), canvas, { label: 'fill:selection' });
    }

    function contentAwareFill(r) {
        if (typeof r.contentAwareFill !== 'function' || !r.selection) return;
        try {
            const res = r.contentAwareFill();
            if (res && typeof res.catch === 'function') res.catch(err => { if (window.console) console.error('pixel content-aware fill', err); });
        } catch (err) {
            if (window.console) console.error('pixel content-aware fill', err);
        }
    }

    function strokeSelection(r, values) {
        const sel = r.getSelectionMask();
        if (!sel || !r.canPaintActiveLayer('stroke')) return false;
        const w = r.canvas.width, h = r.canvas.height;
        const width = Math.max(0.5, Number(values.width) || 1);
        const crop = kit.growBox({ x: sel.x, y: sel.y, w: sel.w, h: sel.h }, Math.ceil(width) + 3, w, h);
        const ring = kit.ringMask(kit.extractBox(sel.alpha, w, crop), crop.w, crop.h, width, values.location);
        const mask = kit.pasteBox(new Uint8ClampedArray(w * h), w, crop, ring);
        const canvas = r.makeCanvas(w, h);
        const ctx = canvas.getContext('2d');
        ctx.fillStyle = values.color;
        ctx.fillRect(0, 0, w, h);
        return r.commitLayerPixels(r.activeLayer(), canvas, { label: 'stroke:selection', respectSelection: false, mask });
    }

    function showStrokeDialog(r) {
        if (!r.selection) return;
        const u = ui(r);
        askValues(r, {
            title: r.t('pixel.sel_stroke', 'Stroke selection'),
            fields: [
                { key: 'width', label: r.t('pixel.stroke_width', 'Stroke width'), min: 1, max: 250, step: 1, value: u.strokeValues.width },
                { key: 'color', type: 'color', label: r.t('pixel.color_primary', 'Color'), value: /^#[0-9a-f]{6}$/i.test(r.primaryColor) ? r.primaryColor : '#000000' },
                {
                    key: 'location', type: 'select', label: r.t('pixel.sel_stroke_location', 'Location'), value: u.strokeValues.location,
                    options: [
                        { value: 'inside', label: r.t('pixel.sel_stroke_inside', 'Inside') },
                        { value: 'center', label: r.t('pixel.sel_stroke_center', 'Center') },
                        { value: 'outside', label: r.t('pixel.sel_stroke_outside', 'Outside') }
                    ]
                }
            ],
            onSubmit: values => {
                u.strokeValues.width = values.width;
                u.strokeValues.location = values.location;
                strokeSelection(r, values);
            }
        });
    }

    // ===============================================================================================================
    // Colour range: a floating panel next to the canvas; clicks on the image pick the colours
    // ===============================================================================================================

    const CR_PROXY = 640;

    function loadColorRangeSource(r, cr) {
        const w = r.canvas.width, h = r.canvas.height;
        cr.src = sampleCanvas(r, ui(r).opts.crAll);
        const scale = Math.min(1, CR_PROXY / Math.max(w, h));
        const pw = Math.max(1, Math.round(w * scale)), ph = Math.max(1, Math.round(h * scale));
        const tmp = r.makeCanvas(pw, ph);
        tmp.getContext('2d').drawImage(cr.src, 0, 0, pw, ph);
        cr.proxy = imageDataOf(tmp);
        cr.pw = pw;
        cr.ph = ph;
        cr.preview = r.makeCanvas(pw, ph);
    }

    function paintColorRangePreview(r, cr) {
        if (!cr.proxy) return;
        const o = ui(r).opts;
        const alpha = kit.colorRangeAlpha(cr.proxy.data, cr.pw, cr.ph, cr.include, cr.exclude, o.crFuzz, o.crInvert);
        const pctx = cr.preview.getContext('2d');
        const img = pctx.createImageData(cr.pw, cr.ph);
        const d = img.data;
        for (let p = 0; p < alpha.length; p++) {
            const a = alpha[p];
            if (!a) continue;
            const i = p * 4;
            d[i] = 255; d[i + 1] = 32; d[i + 2] = 32; d[i + 3] = Math.round(a * 0.62);
        }
        pctx.putImageData(img, 0, 0);
        r.requestOverlay();
    }

    function scheduleColorRangePreview(r, cr) {
        if (cr.timer) return;
        cr.timer = setTimeout(() => {
            cr.timer = 0;
            if (ui(r).cr === cr && !r.state.disposed) paintColorRangePreview(r, cr);
        }, 30);
    }

    function renderColorRangeSwatches(r, cr) {
        if (!cr.swatches) return;
        const chip = (c, exclude) => `<span class="pixel-sel-swatch${exclude ? ' pixel-sel-swatch-ex' : ''}" style="background:rgb(${c[0]},${c[1]},${c[2]})" title="${r.esc(exclude ? r.t('pixel.sel_cr_removed', 'Removed color') : r.t('pixel.sel_cr_picked', 'Picked color'))}"></span>`;
        cr.swatches.innerHTML = cr.include.length || cr.exclude.length
            ? cr.include.map(c => chip(c, false)).join('') + cr.exclude.map(c => chip(c, true)).join('')
            : `<span class="pixel-sel-hint">${r.esc(r.t('pixel.sel_cr_empty', 'No color picked yet.'))}</span>`;
    }

    function pickColorRangeColor(r, cr, e) {
        const rect = r.canvas.getBoundingClientRect();
        if (!rect.width || !rect.height) return;
        const x = clamp(Math.floor((e.clientX - rect.left) * (r.canvas.width / rect.width)), 0, r.canvas.width - 1);
        const y = clamp(Math.floor((e.clientY - rect.top) * (r.canvas.height / rect.height)), 0, r.canvas.height - 1);
        const px = cr.src.getContext('2d', { willReadFrequently: true }).getImageData(x, y, 1, 1).data;
        const color = [px[0], px[1], px[2]];
        if (e.altKey) cr.exclude.push(color);
        else if (e.shiftKey) cr.include.push(color);
        else { cr.include = [color]; cr.exclude = []; }
        if (cr.include.length > 24) cr.include.shift();
        if (cr.exclude.length > 24) cr.exclude.shift();
        renderColorRangeSwatches(r, cr);
        scheduleColorRangePreview(r, cr);
    }

    function closeColorRange(r) {
        const u = ui(r);
        const cr = u.cr;
        if (!cr) return;
        u.cr = null;
        clearTimeout(cr.timer);
        cr.cleanups.forEach(fn => fn());
        if (cr.el) cr.el.remove();
        r.requestOverlay();
    }

    function applyColorRange(r, cr) {
        const o = ui(r).opts;
        const settings = { mode: o.crMode, fuzz: o.crFuzz, invert: o.crInvert, include: cr.include.slice(), exclude: cr.exclude.slice() };
        const src = cr.src;
        closeColorRange(r);
        if (!settings.include.length && !settings.invert) return;
        enqueue(r, async () => {
            const img = imageDataOf(src);
            const alpha = kit.colorRangeAlpha(img.data, img.width, img.height, settings.include, settings.exclude, settings.fuzz, settings.invert);
            await finishMask(r, alpha, settings.mode, 'select:color-range', 0);
        });
    }

    function openColorRange(r) {
        const u = ui(r);
        if (!r.canvas || !r.canvas.width) return;
        if (u.cr) return;
        const cr = { include: [], exclude: [], proxy: null, src: null, preview: null, pw: 0, ph: 0, timer: 0, el: null, swatches: null, cleanups: [] };
        u.cr = cr;
        loadColorRangeSource(r, cr);
        const el = document.createElement('div');
        el.className = 'pixel-sel-cr';
        el.setAttribute('role', 'dialog');
        el.setAttribute('aria-label', r.t('pixel.sel_color_range', 'Color range'));
        el.innerHTML = `<div class="pixel-sel-cr-title">${r.esc(r.t('pixel.sel_color_range', 'Color range'))}</div>
            <p class="pixel-sel-hint">${r.esc(r.t('pixel.sel_cr_hint', 'Click the image to pick a color. Shift+click adds a color, Alt+click removes one.'))}</p>
            <div class="pixel-sel-swatches" data-sel-swatches></div>
            ${sliderHTML(r, 'crFuzz', 'pixel.sel_fuzziness', 'Fuzziness', 0, 200, '')}
            ${checkHTML(r, 'crInvert', 'pixel.sel_invert', 'Invert')}
            ${checkHTML(r, 'crAll', 'pixel.sel_sample_all', 'Sample all layers')}
            ${modeHTML(r, 'crMode')}
            <div class="pixel-sel-cr-actions">
                <button type="button" class="pixel-btn" data-sel-cr="reset">${r.esc(r.t('pixel.reset', 'Reset'))}</button>
                <button type="button" class="pixel-btn" data-sel-cr="cancel">${r.esc(r.t('pixel.cancel', 'Cancel'))}</button>
                <button type="button" class="pixel-btn pixel-btn-primary" data-sel-cr="ok">${r.esc(r.t('pixel.sel_cr_ok', 'Select'))}</button>
            </div>`;
        cr.el = el;
        cr.swatches = el.querySelector('[data-sel-swatches]');
        cr.cleanups.push(r.registerOverlay('color-range', ctx => drawColorRange(r, ctx), 55));
        const parent = r.host.querySelector('.pixel-workspace') || r.host;
        parent.appendChild(el);
        const dock = r.panelContainer;
        if (dock && dock.getBoundingClientRect) {
            const width = dock.getBoundingClientRect().width;
            if (width > 0) el.style.right = Math.round(width + 14) + 'px';
        }
        wireSelOptions(r, el, key => {
            if (key === 'crAll') {
                loadColorRangeSource(r, cr);
            }
            scheduleColorRangePreview(r, cr);
        });
        el.querySelectorAll('[data-sel-cr]').forEach(btn => btn.addEventListener('click', () => {
            const act = btn.dataset.selCr;
            if (act === 'ok') applyColorRange(r, cr);
            else if (act === 'cancel') closeColorRange(r);
            else if (act === 'reset') {
                cr.include = []; cr.exclude = [];
                renderColorRangeSwatches(r, cr);
                scheduleColorRangePreview(r, cr);
            }
        }));
        // Picking: the press never reaches the active tool while the panel is open.
        const wrap = r.canvasWrap;
        if (wrap) {
            const onPick = e => {
                if (e.button !== 0) return;
                e.preventDefault();
                e.stopPropagation();
                pickColorRangeColor(r, cr, e);
            };
            wrap.addEventListener('pointerdown', onPick, true);
            cr.cleanups.push(() => wrap.removeEventListener('pointerdown', onPick, true));
        }
        const onKey = e => {
            if (r.state.disposed || ui(r).cr !== cr) { document.removeEventListener('keydown', onKey, true); return; }
            if (!r.isActiveWindow()) return;
            const target = e.target;
            const closest = selector => !!(target && target.closest && target.closest(selector));
            const typing = closest('input, textarea, select, [contenteditable=""], [contenteditable="true"]');
            if (typing && (!closest('.pixel-sel-cr') || e.key !== 'Escape')) return;
            if (e.key === 'Escape') { e.preventDefault(); e.stopPropagation(); closeColorRange(r); }
            else if (e.key === 'Enter') { e.preventDefault(); e.stopPropagation(); applyColorRange(r, cr); }
        };
        document.addEventListener('keydown', onKey, true);
        cr.cleanups.push(() => document.removeEventListener('keydown', onKey, true));
        renderColorRangeSwatches(r, cr);
        paintColorRangePreview(r, cr);
    }

    function drawColorRange(r, ctx) {
        const cr = ui(r).cr;
        if (cr && cr.preview) ctx.drawImage(cr.preview, 0, 0, r.canvas.width, r.canvas.height);
    }

    // ===============================================================================================================
    // Registrations: tools, menus, commands, the per-window module
    // ===============================================================================================================

    // Size of the selection descriptor without rasterising it (called on every selection change for the status bar).
    function selectionSize(sel) {
        if (!sel) return null;
        return { w: Math.round(Math.abs(sel.w)), h: Math.round(Math.abs(sel.h)) };
    }

    Pixel.registerTool(shapeTool('select-rect'));
    Pixel.registerTool(shapeTool('select-ellipse'));
    Pixel.registerTool(lassoTool());
    Pixel.registerTool(wandTool());
    Pixel.registerTool(Object.assign({
        group: 'select',
        order: 45,
        svg: '<svg width="16" height="16" viewBox="0 0 16 16"><path d="M3 12.5L4.6 4.2 10.8 2.8l2.4 6L8 13.6z" fill="none" stroke="currentColor" stroke-width="1.3" stroke-linejoin="round" stroke-dasharray="2 1.6"/><circle cx="3" cy="12.5" r="1.3" fill="currentColor"/><circle cx="4.6" cy="4.2" r="1.3" fill="currentColor"/><circle cx="10.8" cy="2.8" r="1.3" fill="currentColor"/><circle cx="13.2" cy="8.8" r="1.3" fill="currentColor"/><circle cx="8" cy="13.6" r="1.3" fill="currentColor"/></svg>',
        shortcut: 'Shift+K',
        labelKey: 'pixel.polygon_lasso',
        fallback: 'Polygon lasso',
        raster: false,
        cursor: 'crosshair'
    }, polygonTool()));
    Pixel.registerTool(quickSelectTool());

    const hasDocument = r => !!(r.canvas && r.canvas.width);
    const hasSelection = r => !!r.selection;

    function needSelection(r) {
        r.notify({ type: 'info', message: r.t('pixel.sel_need_selection', 'Make a selection first.') });
    }

    // The built-in Edit items run r.selectAll() / r.deselect() directly; these also write the history step.
    Pixel.registerMenuItems('edit', [
        { id: 'select-all', labelKey: 'pixel.select_all', icon: 'image', shortcut: 'Ctrl+A', action: r => selectAllRecorded(r) },
        { id: 'deselect', labelKey: 'pixel.deselect', icon: 'image', shortcut: 'Ctrl+D', action: r => deselectRecorded(r) }
    ], { order: 0 });

    Pixel.registerMenuItems('select', [
        { id: 'select-all', labelKey: 'pixel.select_all', label: 'Select all', icon: 'image', shortcutHint: 'Ctrl+A', enabled: hasDocument, action: r => selectAllRecorded(r) },
        { id: 'deselect', labelKey: 'pixel.deselect', label: 'Deselect', icon: 'image', shortcutHint: 'Ctrl+D', action: r => deselectRecorded(r) },
        { id: 'reselect', labelKey: 'pixel.sel_reselect', label: 'Reselect', icon: 'image', shortcut: 'Ctrl+Shift+D', action: r => { reselect(r); } },
        { id: 'inverse', labelKey: 'pixel.sel_inverse', label: 'Inverse', icon: 'image', shortcut: 'Ctrl+Shift+I', action: r => invertRecorded(r) },
        { type: 'separator' },
        { id: 'select-subject', labelKey: 'pixel.sel_subject', label: 'Select subject', icon: 'image', hidden: r => typeof r.computeSubjectMask !== 'function', enabled: r => hasDocument(r) && typeof r.computeSubjectMask === 'function', action: r => { enqueue(r, () => selectSubject(r)); } },
        { id: 'color-range', labelKey: 'pixel.sel_color_range_menu', label: 'Color range…', icon: 'image', enabled: hasDocument, action: r => openColorRange(r) },
        { type: 'separator' },
        {
            id: 'modify', labelKey: 'pixel.sel_modify', label: 'Modify', icon: 'image', enabled: hasSelection,
            items: [
                { id: 'grow', labelKey: 'pixel.sel_grow', label: 'Grow…', icon: 'image', action: r => showModifyDialog(r, 'grow') },
                { id: 'shrink', labelKey: 'pixel.sel_shrink', label: 'Shrink…', icon: 'image', action: r => showModifyDialog(r, 'shrink') },
                { id: 'border', labelKey: 'pixel.sel_border', label: 'Border…', icon: 'image', action: r => showModifyDialog(r, 'border') },
                { id: 'smooth', labelKey: 'pixel.sel_smooth', label: 'Smooth…', icon: 'image', action: r => showModifyDialog(r, 'smooth') },
                { id: 'feather', labelKey: 'pixel.sel_feather_menu', label: 'Feather…', icon: 'image', shortcut: 'Shift+F6', action: r => { if (r.selection) showModifyDialog(r, 'feather'); } }
            ]
        },
        { type: 'separator' },
        { id: 'from-layer-alpha', labelKey: 'pixel.sel_from_alpha', label: 'Selection from layer transparency', icon: 'image', enabled: hasDocument, action: r => { selectFromLayerAlpha(r, null, 'replace'); } },
        { id: 'to-layer-mask', labelKey: 'pixel.sel_to_mask', label: 'Save selection as layer mask', icon: 'image', enabled: hasSelection, action: r => { selectionToLayerMask(r); } },
        { id: 'copy-to-layer', labelKey: 'pixel.sel_copy_layer', label: 'Copy to new layer', icon: 'image', enabled: hasSelection, action: r => { copyToNewLayer(r); } },
        { id: 'cut-to-layer', labelKey: 'pixel.sel_cut_layer', label: 'Cut to new layer', icon: 'scissors', shortcut: 'Ctrl+Shift+J', action: r => { if (r.selection) layerFromSelection(r, true); else needSelection(r); } },
        { type: 'separator' },
        { id: 'fill-primary', labelKey: 'pixel.sel_fill_primary', label: 'Fill with primary color', icon: 'image', shortcut: 'Alt+Backspace', enabled: hasDocument, action: r => { fillSelection(r, 'primary'); } },
        { id: 'fill-secondary', labelKey: 'pixel.sel_fill_secondary', label: 'Fill with secondary color', icon: 'image', shortcut: 'Ctrl+Backspace', enabled: hasDocument, action: r => { fillSelection(r, 'secondary'); } },
        { id: 'fill-content-aware', labelKey: 'pixel.sel_fill_content', label: 'Content-aware fill', icon: 'image', hidden: r => typeof r.contentAwareFill !== 'function', enabled: r => hasSelection(r) && typeof r.contentAwareFill === 'function', action: r => contentAwareFill(r) },
        { id: 'stroke', labelKey: 'pixel.sel_stroke_menu', label: 'Stroke selection…', icon: 'image', enabled: hasSelection, action: r => showStrokeDialog(r) }
    ], { order: 0 });

    // Escape keeps its built-in meaning (cancel the crop, deselect) but leaves a history step.
    Pixel.registerCommand({
        id: 'edit.cancel', labelKey: 'pixel.cmd_cancel', fallback: 'Cancel crop / deselect', group: 'edit', shortcut: 'Escape',
        run: r => { if (r.cropState) r.cancelCrop(); if (r.selection) deselectRecorded(r); },
        enabled: r => !!(r.cropState || r.selection)
    });

    Pixel.registerModule('selection', function install(r) {
        const u = {
            opts: {
                mode: 'replace', feather: 0, antialias: true, style: 'normal', ratioW: 1, ratioH: 1, sizeW: 100, sizeH: 100,
                tolerance: 32, contiguous: true, sampleAll: false,
                qsMode: 'add', qsSize: 30, qsTolerance: 25, qsEdge: 50, qsSampleAll: false,
                crFuzz: 40, crInvert: false, crAll: true, crMode: 'replace'
            },
            g: null, pending: null, poly: null, hover: null, chain: Promise.resolve(), lastSel: null, contour: null,
            moveOffset: null, cursorName: '', nudgeTimer: 0, dialog: null, cr: null,
            modifyValues: { grow: 5, shrink: 5, border: 10, smooth: 5, feather: 5 },
            strokeValues: { width: 4, location: 'center' }
        };
        r.selectionUI = u;
        // Entry points of the dialogs for scripts and tests.
        u.colorRange = {
            open: () => openColorRange(r),
            apply: () => { if (u.cr) applyColorRange(r, u.cr); },
            close: () => closeColorRange(r)
        };
        u.stroke = values => strokeSelection(r, values);

        r.registerOverlay('selection', ctx => drawSelection(r, ctx), 50);

        // The status bar names the size of the selection next to the document size.
        const baseUpdateStatus = r.updateStatus;
        r.updateStatus = Pixel.bindRuntime(r, function updateStatus() {
            baseUpdateStatus();
            const size = selectionSize(this.selection);
            if (!size || !this.statusText || !this.imgWidth) return;
            this.setStatus(this.statusText.textContent + '  ·  ' + this.t('pixel.sel_status', 'Selection') + ' ' + size.w + ' × ' + size.h);
        });

        // Double-clicking closes a polygon; the shell would also read it as "zoom to fit".
        r.on('ready', () => {
            if (!r.overlayCanvas) return;
            r.overlayCanvas.addEventListener('dblclick', e => {
                if (r.activeTool === 'polygon-lasso') e.stopPropagation();
            });
        });

        // Reselect remembers what a deselect took away.
        let previous = r.selection;
        r.on('selection-changed', e => {
            const next = e ? e.selection : r.selection;
            if (previous && !next) u.lastSel = previous;
            if (!next) u.contour = null;
            previous = next;
            r.updateStatus();
        });
        r.on('document-changed', () => {
            u.lastSel = null;
            u.contour = null;
        });

        Object.assign(r, {
            // History step for a selection change (does not mark the document as unsaved).
            recordSelection: Pixel.bindRuntime(r, function recordSelectionMethod(label) { recordSelection(this, label || 'select'); }),
            // kind: grow | shrink | border | feather | smooth; amount in pixels. Resolves to true when the selection changed.
            modifySelection: Pixel.bindRuntime(r, function modifySelectionMethod(kind, amount) { return enqueue(this, () => modifySelection(this, kind, amount)); }),
            // Selection from the alpha of a layer (default: the active one). mode: replace | add | subtract | intersect.
            selectFromLayerAlpha: Pixel.bindRuntime(r, function selectFromLayerAlphaMethod(layer, mode) { return selectFromLayerAlpha(this, layer, mode); }),
            layerFromSelection: Pixel.bindRuntime(r, function layerFromSelectionMethod(cut) { return layerFromSelection(this, !!cut); }),
            selectionToLayerMask: Pixel.bindRuntime(r, function selectionToLayerMaskMethod() { return selectionToLayerMask(this); }),
            reselect: Pixel.bindRuntime(r, function reselectMethod() { return reselect(this); })
        });
    });
})();
