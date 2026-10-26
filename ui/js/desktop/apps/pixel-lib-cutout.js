// Pixel pure image library, cutout / background removal part (WP8a). DOM-free; loads in a page, in the Pixel worker and in
// a node vm. Ops (AuraPixelLib.ops): cutout.classic (GrabCut with graph cut, or a soft colour key for plain backgrounds),
// cutout.chroma (colour key with despill), cutout.refine (guided filter, edge shift, smooth, feather), cutout.decontaminate
// (foreground colour estimation) and cutout.blur (background blur that ignores the subject).
// Images are { width, height, data: Uint8ClampedArray RGBA }, mattes are Uint8ClampedArray(width * height) with 255 = subject.
(function (root) {
    'use strict';

    const Lib = root.AuraPixelLib = root.AuraPixelLib || {};
    const clamp = Lib.clamp;
    // Ops get progress / cancelled from the host; direct callers (tests) may pass nothing.
    const quiet = ctx => ctx || { progress() {}, cancelled() { return false; } };

    // --- float planes ---

    // Mean over the (2r+1)^2 window, clipped at the borders. src and the result are planes of w * h samples.
    function boxMean(src, w, h, r) {
        const n = w * h;
        const res = new Float32Array(n);
        if (r < 1 || !n) {
            res.set(src);
            return res;
        }
        const tmp = new Float32Array(n);
        for (let y = 0; y < h; y++) {
            const row = y * w;
            let sum = 0;
            const first = Math.min(w - 1, r);
            for (let x = 0; x <= first; x++) sum += src[row + x];
            for (let x = 0; x < w; x++) {
                const lo = x - r < 0 ? 0 : x - r;
                const hi = x + r >= w ? w - 1 : x + r;
                tmp[row + x] = sum / (hi - lo + 1);
                if (x + r + 1 < w) sum += src[row + x + r + 1];
                if (x - r >= 0) sum -= src[row + x - r];
            }
        }
        const acc = new Float64Array(w);
        const top = Math.min(h - 1, r);
        for (let y = 0; y <= top; y++) {
            const row = y * w;
            for (let x = 0; x < w; x++) acc[x] += tmp[row + x];
        }
        for (let y = 0; y < h; y++) {
            const lo = y - r < 0 ? 0 : y - r;
            const hi = y + r >= h ? h - 1 : y + r;
            const inv = 1 / (hi - lo + 1);
            const row = y * w;
            for (let x = 0; x < w; x++) res[row + x] = acc[x] * inv;
            if (y + r + 1 < h) {
                const add = (y + r + 1) * w;
                for (let x = 0; x < w; x++) acc[x] += tmp[add + x];
            }
            if (y - r >= 0) {
                const sub = (y - r) * w;
                for (let x = 0; x < w; x++) acc[x] -= tmp[sub + x];
            }
        }
        return res;
    }

    function halvePlane(src, w, h) {
        const nw = Math.max(1, w >> 1), nh = Math.max(1, h >> 1);
        const out = new Float32Array(nw * nh);
        for (let y = 0; y < nh; y++) {
            const y0 = Math.min(h - 1, y * 2) * w, y1 = Math.min(h - 1, y * 2 + 1) * w;
            for (let x = 0; x < nw; x++) {
                const x0 = Math.min(w - 1, x * 2), x1 = Math.min(w - 1, x * 2 + 1);
                out[y * nw + x] = (src[y0 + x0] + src[y0 + x1] + src[y1 + x0] + src[y1 + x1]) * 0.25;
            }
        }
        return { data: out, w: nw, h: nh };
    }

    // Bilinear resample of a plane; large reductions average first so thin features survive.
    function resizePlane(src, sw, sh, dw, dh) {
        let cur = src, cw = sw, ch = sh;
        while (cw >= 2 * dw && ch >= 2 * dh) {
            const half = halvePlane(cur, cw, ch);
            cur = half.data; cw = half.w; ch = half.h;
        }
        const out = new Float32Array(dw * dh);
        if (cw === dw && ch === dh) {
            out.set(cur);
            return out;
        }
        const x0s = new Int32Array(dw), x1s = new Int32Array(dw), xts = new Float32Array(dw);
        for (let x = 0; x < dw; x++) {
            const fx = clamp((x + 0.5) * cw / dw - 0.5, 0, cw - 1);
            const x0 = Math.floor(fx);
            x0s[x] = x0; x1s[x] = Math.min(cw - 1, x0 + 1); xts[x] = fx - x0;
        }
        for (let y = 0; y < dh; y++) {
            const fy = clamp((y + 0.5) * ch / dh - 0.5, 0, ch - 1);
            const y0 = Math.floor(fy), ty = fy - y0;
            const r0 = y0 * cw, r1 = Math.min(ch - 1, y0 + 1) * cw;
            for (let x = 0; x < dw; x++) {
                const tx = xts[x], a = x0s[x], b = x1s[x];
                const top = cur[r0 + a] + (cur[r0 + b] - cur[r0 + a]) * tx;
                const bottom = cur[r1 + a] + (cur[r1 + b] - cur[r1 + a]) * tx;
                out[y * dw + x] = top + (bottom - top) * ty;
            }
        }
        return out;
    }

    function resizeMask(src, sw, sh, dw, dh) {
        if (sw === dw && sh === dh) return Uint8ClampedArray.from(src);
        const plane = resizePlane(src, sw, sh, dw, dh);
        const out = new Uint8ClampedArray(dw * dh);
        for (let i = 0; i < out.length; i++) out[i] = plane[i];
        return out;
    }

    function smoothstep(t) {
        t = t < 0 ? 0 : (t > 1 ? 1 : t);
        return t * t * (3 - 2 * t);
    }

    // --- guided filter (He et al.) with a colour guide; the statistics may run on a reduced copy (fast guided filter) ---

    function productMean(a, b, w, h, r, tmp) {
        for (let i = 0; i < tmp.length; i++) tmp[i] = a[i] * b[i];
        return boxMean(tmp, w, h, r);
    }

    // guide: RGBA image, p: Float32 plane in 0..1 of the same size. Returns the window-averaged coefficients a (3) and b.
    function guidedCoefficients(guide, p, r, eps) {
        const w = guide.width, h = guide.height, n = w * h, g = guide.data;
        const i0 = new Float32Array(n), i1 = new Float32Array(n), i2 = new Float32Array(n);
        for (let i = 0, q = 0; i < n; i++, q += 4) {
            i0[i] = g[q] / 255; i1[i] = g[q + 1] / 255; i2[i] = g[q + 2] / 255;
        }
        const m0 = boxMean(i0, w, h, r), m1 = boxMean(i1, w, h, r), m2 = boxMean(i2, w, h, r), mp = boxMean(p, w, h, r);
        const tmp = new Float32Array(n);
        const x0 = productMean(i0, p, w, h, r, tmp), x1 = productMean(i1, p, w, h, r, tmp), x2 = productMean(i2, p, w, h, r, tmp);
        for (let i = 0; i < n; i++) { x0[i] -= m0[i] * mp[i]; x1[i] -= m1[i] * mp[i]; x2[i] -= m2[i] * mp[i]; }
        const v00 = productMean(i0, i0, w, h, r, tmp), v01 = productMean(i0, i1, w, h, r, tmp), v02 = productMean(i0, i2, w, h, r, tmp);
        const v11 = productMean(i1, i1, w, h, r, tmp), v12 = productMean(i1, i2, w, h, r, tmp), v22 = productMean(i2, i2, w, h, r, tmp);
        for (let i = 0; i < n; i++) {
            const a00 = v00[i] - m0[i] * m0[i] + eps, a01 = v01[i] - m0[i] * m1[i], a02 = v02[i] - m0[i] * m2[i];
            const a11 = v11[i] - m1[i] * m1[i] + eps, a12 = v12[i] - m1[i] * m2[i], a22 = v22[i] - m2[i] * m2[i] + eps;
            const c00 = a11 * a22 - a12 * a12, c01 = a02 * a12 - a01 * a22, c02 = a01 * a12 - a02 * a11;
            const c11 = a00 * a22 - a02 * a02, c12 = a01 * a02 - a00 * a12, c22 = a00 * a11 - a01 * a01;
            const inv = 1 / (a00 * c00 + a01 * c01 + a02 * c02);
            const k0 = x0[i], k1 = x1[i], k2 = x2[i];
            const s0 = (k0 * c00 + k1 * c01 + k2 * c02) * inv;
            const s1 = (k0 * c01 + k1 * c11 + k2 * c12) * inv;
            const s2 = (k0 * c02 + k1 * c12 + k2 * c22) * inv;
            x0[i] = s0; x1[i] = s1; x2[i] = s2;
            v00[i] = mp[i] - (s0 * m0[i] + s1 * m1[i] + s2 * m2[i]);
        }
        return { w, h, a0: boxMean(x0, w, h, r), a1: boxMean(x1, w, h, r), a2: boxMean(x2, w, h, r), b: boxMean(v00, w, h, r) };
    }

    // q = mean(a) . I + mean(b) with the coefficients sampled bilinearly at every pixel of the full-size image.
    function guidedApply(image, co) {
        const W = image.width, H = image.height, d = image.data, lw = co.w, lh = co.h;
        const out = new Uint8ClampedArray(W * H);
        const xa = new Int32Array(W), xb = new Int32Array(W), xt = new Float32Array(W);
        for (let x = 0; x < W; x++) {
            const fx = clamp((x + 0.5) * lw / W - 0.5, 0, lw - 1);
            const x0 = Math.floor(fx);
            xa[x] = x0; xb[x] = Math.min(lw - 1, x0 + 1); xt[x] = fx - x0;
        }
        const { a0, a1, a2, b } = co;
        for (let y = 0; y < H; y++) {
            const fy = clamp((y + 0.5) * lh / H - 0.5, 0, lh - 1);
            const y0 = Math.floor(fy), ty = fy - y0;
            const r0 = y0 * lw, r1 = Math.min(lh - 1, y0 + 1) * lw;
            for (let x = 0; x < W; x++) {
                const tx = xt[x];
                const i00 = r0 + xa[x], i10 = r0 + xb[x], i01 = r1 + xa[x], i11 = r1 + xb[x];
                const w00 = (1 - tx) * (1 - ty), w10 = tx * (1 - ty), w01 = (1 - tx) * ty, w11 = tx * ty;
                const q = (d[(y * W + x) * 4] * (a0[i00] * w00 + a0[i10] * w10 + a0[i01] * w01 + a0[i11] * w11) +
                    d[(y * W + x) * 4 + 1] * (a1[i00] * w00 + a1[i10] * w10 + a1[i01] * w01 + a1[i11] * w11) +
                    d[(y * W + x) * 4 + 2] * (a2[i00] * w00 + a2[i10] * w10 + a2[i01] * w01 + a2[i11] * w11)) / 255 +
                    (b[i00] * w00 + b[i10] * w10 + b[i01] * w01 + b[i11] * w11);
                // levels: the filter leaks a little noise into flat areas; below 4 % and above 96 % the matte is solid
                out[y * W + x] = (q - 0.04) / 0.92 * 255;
            }
        }
        return out;
    }

    // Guide-following upsample of a matte of any size to the size of the image. workEdge bounds the size of the statistics.
    function guidedMatte(image, alpha, aw, ah, radius, eps, workEdge) {
        const W = image.width, H = image.height;
        const scale = Math.max(1, Math.max(W, H) / (workEdge || 1024));
        const lw = Math.max(1, Math.round(W / scale)), lh = Math.max(1, Math.round(H / scale));
        const guide = scale > 1 ? Lib.resizeBilinear(image, lw, lh) : image;
        const p = resizePlane(alpha, aw, ah, lw, lh);
        for (let i = 0; i < p.length; i++) p[i] /= 255;
        const co = guidedCoefficients(guide, p, Math.max(1, Math.round(radius / scale)), eps);
        return guidedApply(image, co);
    }

    // --- edge shift, smoothing ---

    // One 3x3 grey-scale dilation (isMax) or erosion, as a full square or a plus.
    function morphStep(src, w, h, isMax, plus) {
        const out = new Uint8ClampedArray(src.length);
        for (let y = 0; y < h; y++) {
            const up = y > 0, down = y < h - 1;
            for (let x = 0; x < w; x++) {
                const p = y * w + x;
                const left = x > 0, right = x < w - 1;
                let v = src[p], t;
                if (left) { t = src[p - 1]; if (isMax ? t > v : t < v) v = t; }
                if (right) { t = src[p + 1]; if (isMax ? t > v : t < v) v = t; }
                if (up) { t = src[p - w]; if (isMax ? t > v : t < v) v = t; }
                if (down) { t = src[p + w]; if (isMax ? t > v : t < v) v = t; }
                if (!plus) {
                    if (up && left) { t = src[p - w - 1]; if (isMax ? t > v : t < v) v = t; }
                    if (up && right) { t = src[p - w + 1]; if (isMax ? t > v : t < v) v = t; }
                    if (down && left) { t = src[p + w - 1]; if (isMax ? t > v : t < v) v = t; }
                    if (down && right) { t = src[p + w + 1]; if (isMax ? t > v : t < v) v = t; }
                }
                out[p] = v;
            }
        }
        return out;
    }

    // Moves the matte edge by whole pixels (positive grows the subject): alternating square and plus steps give a round shape.
    function shiftEdge(alpha, w, h, amount) {
        const steps = Math.min(48, Math.abs(Math.round(amount) || 0));
        let cur = alpha;
        for (let i = 0; i < steps; i++) cur = morphStep(cur, w, h, amount > 0, i % 2 === 1);
        return cur;
    }

    // Rounds the contour: blur, then steepen the ramp again so the edge stays crisp.
    function smoothEdge(alpha, w, h, amount) {
        const s = clamp(Number(amount) || 0, 0, 10);
        if (s <= 0) return alpha;
        const blurred = Lib.blurAlpha(alpha, w, h, s * 1.2);
        const gain = 1 + s * 0.7;
        const out = new Uint8ClampedArray(alpha.length);
        for (let i = 0; i < out.length; i++) out[i] = 127.5 + (blurred[i] - 127.5) * gain;
        return out;
    }

    // --- connected components (4-neighbourhood) ---

    // on: Uint8Array of 0/1. Returns { labels (0 = off), sizes (index = label), border (label touches the edge), count }.
    function labelComponents(on, w, h) {
        const n = w * h;
        const labels = new Int32Array(n);
        const stack = new Int32Array(n);
        const sizes = [0], border = [0];
        let count = 0;
        for (let start = 0; start < n; start++) {
            if (!on[start] || labels[start]) continue;
            count++;
            let top = 0, size = 0, touches = 0;
            stack[top++] = start;
            labels[start] = count;
            while (top > 0) {
                const p = stack[--top];
                size++;
                const x = p % w, y = (p - x) / w;
                if (x === 0 || y === 0 || x === w - 1 || y === h - 1) touches = 1;
                if (x > 0 && on[p - 1] && !labels[p - 1]) { labels[p - 1] = count; stack[top++] = p - 1; }
                if (x < w - 1 && on[p + 1] && !labels[p + 1]) { labels[p + 1] = count; stack[top++] = p + 1; }
                if (y > 0 && on[p - w] && !labels[p - w]) { labels[p - w] = count; stack[top++] = p - w; }
                if (y < h - 1 && on[p + w] && !labels[p + w]) { labels[p + w] = count; stack[top++] = p + w; }
            }
            sizes.push(size);
            border.push(touches);
        }
        return { labels, sizes, border, count };
    }

    // Drops subject islands below 1 % of the image (the largest and anything holding a keep-hint always stay) and fills
    // small enclosed holes (unless they hold a remove-hint). alpha is changed in place; hints are 0/1 planes or null.
    function cleanMatte(alpha, w, h, threshold, keepHint, dropHint) {
        const n = w * h;
        const minArea = Math.max(4, Math.round(n * 0.01));
        const on = new Uint8Array(n);
        for (let i = 0; i < n; i++) on[i] = alpha[i] > threshold ? 1 : 0;
        const fg = labelComponents(on, w, h);
        if (!fg.count) return alpha;
        let largest = 1;
        for (let k = 2; k <= fg.count; k++) if (fg.sizes[k] > fg.sizes[largest]) largest = k;
        const keep = new Uint8Array(fg.count + 1);
        for (let k = 1; k <= fg.count; k++) keep[k] = k === largest || fg.sizes[k] >= minArea ? 1 : 0;
        if (keepHint) for (let i = 0; i < n; i++) if (keepHint[i] && fg.labels[i]) keep[fg.labels[i]] = 1;
        for (let i = 0; i < n; i++) if (fg.labels[i] && !keep[fg.labels[i]]) alpha[i] = 0;
        for (let i = 0; i < n; i++) on[i] = alpha[i] > threshold ? 0 : 1;
        const holes = labelComponents(on, w, h);
        const fill = new Uint8Array(holes.count + 1);
        for (let k = 1; k <= holes.count; k++) fill[k] = !holes.border[k] && holes.sizes[k] < minArea ? 1 : 0;
        if (dropHint) for (let i = 0; i < n; i++) if (dropHint[i] && holes.labels[i]) fill[holes.labels[i]] = 0;
        for (let i = 0; i < n; i++) if (holes.labels[i] && fill[holes.labels[i]]) alpha[i] = 255;
        return alpha;
    }

    // --- colour statistics, k-means, Gaussian mixtures ---

    // Interleaved float RGB (3 samples per pixel), lightly smoothed so sensor noise does not drive the model.
    function smoothedColors(data, w, h) {
        const n = w * h;
        const planes = [new Float32Array(n), new Float32Array(n), new Float32Array(n)];
        for (let i = 0, q = 0; i < n; i++, q += 4) { planes[0][i] = data[q]; planes[1][i] = data[q + 1]; planes[2][i] = data[q + 2]; }
        const sm = planes.map(plane => boxMean(plane, w, h, 1));
        const col = new Float32Array(n * 3);
        for (let i = 0; i < n; i++) { col[i * 3] = sm[0][i]; col[i * 3 + 1] = sm[1][i]; col[i * 3 + 2] = sm[2][i]; }
        return col;
    }

    // k-means++ seeding and Lloyd steps on interleaved RGB points. Returns the centres (k * 3).
    function kmeans(points, count, k, rng, iterations) {
        k = Math.max(1, Math.min(k, count));
        const centers = new Float64Array(k * 3);
        const first = Math.floor(rng() * count);
        centers.set([points[first * 3], points[first * 3 + 1], points[first * 3 + 2]], 0);
        const dist = new Float64Array(count).fill(Infinity);
        for (let c = 1; c < k; c++) {
            let total = 0;
            for (let i = 0; i < count; i++) {
                const dr = points[i * 3] - centers[(c - 1) * 3], dg = points[i * 3 + 1] - centers[(c - 1) * 3 + 1], db = points[i * 3 + 2] - centers[(c - 1) * 3 + 2];
                const d = dr * dr + dg * dg + db * db;
                if (d < dist[i]) dist[i] = d;
                total += dist[i];
            }
            let pick = count - 1;
            if (total > 0) {
                let target = rng() * total;
                for (let i = 0; i < count; i++) { target -= dist[i]; if (target <= 0) { pick = i; break; } }
            }
            centers.set([points[pick * 3], points[pick * 3 + 1], points[pick * 3 + 2]], c * 3);
        }
        const sums = new Float64Array(k * 3), counts = new Int32Array(k);
        for (let it = 0; it < iterations; it++) {
            sums.fill(0); counts.fill(0);
            for (let i = 0; i < count; i++) {
                const r = points[i * 3], g = points[i * 3 + 1], b = points[i * 3 + 2];
                let best = 0, bestD = Infinity;
                for (let c = 0; c < k; c++) {
                    const dr = r - centers[c * 3], dg = g - centers[c * 3 + 1], db = b - centers[c * 3 + 2];
                    const d = dr * dr + dg * dg + db * db;
                    if (d < bestD) { bestD = d; best = c; }
                }
                sums[best * 3] += r; sums[best * 3 + 1] += g; sums[best * 3 + 2] += b; counts[best]++;
            }
            for (let c = 0; c < k; c++) {
                if (counts[c]) { centers[c * 3] = sums[c * 3] / counts[c]; centers[c * 3 + 1] = sums[c * 3 + 1] / counts[c]; centers[c * 3 + 2] = sums[c * 3 + 2] / counts[c]; }
            }
        }
        return centers;
    }

    const GMM_K = 5;
    const LOG_2PI_15 = 1.5 * Math.log(2 * Math.PI);

    // Gaussian mixture over the pixels in idx (full covariance). Hard-assignment EM from a k-means start.
    function fitGmm(col, idx, count, rng) {
        const k = Math.max(1, Math.min(GMM_K, count));
        const step = Math.max(1, Math.floor(count / 3000));
        const sample = new Float32Array(Math.ceil(count / step) * 3);
        let ns = 0;
        for (let i = 0; i < count; i += step, ns++) {
            const p = idx[i] * 3;
            sample[ns * 3] = col[p]; sample[ns * 3 + 1] = col[p + 1]; sample[ns * 3 + 2] = col[p + 2];
        }
        const centers = kmeans(sample, ns, k, rng, 6);
        const gmm = { k, logc: new Float64Array(k), mu: new Float64Array(k * 3), icov: new Float64Array(k * 6) };
        const assign = new Uint8Array(count);
        for (let i = 0; i < count; i++) {
            const p = idx[i] * 3;
            let best = 0, bestD = Infinity;
            for (let c = 0; c < k; c++) {
                const dr = col[p] - centers[c * 3], dg = col[p + 1] - centers[c * 3 + 1], db = col[p + 2] - centers[c * 3 + 2];
                const d = dr * dr + dg * dg + db * db;
                if (d < bestD) { bestD = d; best = c; }
            }
            assign[i] = best;
        }
        gmmFromAssignment(gmm, col, idx, count, assign);
        for (let i = 0; i < count; i++) assign[i] = gmmBest(gmm, col, idx[i] * 3);
        gmmFromAssignment(gmm, col, idx, count, assign);
        return gmm;
    }

    function gmmFromAssignment(gmm, col, idx, count, assign) {
        const k = gmm.k;
        const cnt = new Float64Array(k), sum = new Float64Array(k * 3), sq = new Float64Array(k * 6);
        for (let i = 0; i < count; i++) {
            const c = assign[i], p = idx[i] * 3;
            const r = col[p], g = col[p + 1], b = col[p + 2];
            cnt[c]++;
            sum[c * 3] += r; sum[c * 3 + 1] += g; sum[c * 3 + 2] += b;
            sq[c * 6] += r * r; sq[c * 6 + 1] += r * g; sq[c * 6 + 2] += r * b;
            sq[c * 6 + 3] += g * g; sq[c * 6 + 4] += g * b; sq[c * 6 + 5] += b * b;
        }
        for (let c = 0; c < k; c++) {
            if (cnt[c] < 1) { gmm.logc[c] = -Infinity; continue; }
            const n = cnt[c];
            const m0 = sum[c * 3] / n, m1 = sum[c * 3 + 1] / n, m2 = sum[c * 3 + 2] / n;
            // the floor keeps flat patches from collapsing into spikes (8-bit colour, about 2 levels of spread)
            const reg = 4;
            const a00 = sq[c * 6] / n - m0 * m0 + reg, a01 = sq[c * 6 + 1] / n - m0 * m1, a02 = sq[c * 6 + 2] / n - m0 * m2;
            const a11 = sq[c * 6 + 3] / n - m1 * m1 + reg, a12 = sq[c * 6 + 4] / n - m1 * m2, a22 = sq[c * 6 + 5] / n - m2 * m2 + reg;
            const c00 = a11 * a22 - a12 * a12, c01 = a02 * a12 - a01 * a22, c02 = a01 * a12 - a02 * a11;
            const c11 = a00 * a22 - a02 * a02, c12 = a01 * a02 - a00 * a12, c22 = a00 * a11 - a01 * a01;
            const det = Math.max(1e-3, a00 * c00 + a01 * c01 + a02 * c02);
            const inv = 1 / det;
            gmm.mu[c * 3] = m0; gmm.mu[c * 3 + 1] = m1; gmm.mu[c * 3 + 2] = m2;
            gmm.icov[c * 6] = c00 * inv; gmm.icov[c * 6 + 1] = c01 * inv; gmm.icov[c * 6 + 2] = c02 * inv;
            gmm.icov[c * 6 + 3] = c11 * inv; gmm.icov[c * 6 + 4] = c12 * inv; gmm.icov[c * 6 + 5] = c22 * inv;
            gmm.logc[c] = Math.log(n / count) - 0.5 * Math.log(det) - LOG_2PI_15;
        }
    }

    // log(pi_c * N(x | component c)) for the components of the mixture, written to terms; returns the largest one.
    function gmmTerms(gmm, r, g, b, terms) {
        const mu = gmm.mu, ic = gmm.icov, logc = gmm.logc;
        let top = -Infinity;
        for (let c = 0; c < gmm.k; c++) {
            let v = -Infinity;
            if (logc[c] !== -Infinity) {
                const x0 = r - mu[c * 3], x1 = g - mu[c * 3 + 1], x2 = b - mu[c * 3 + 2];
                const q = c * 6;
                v = logc[c] - 0.5 * (ic[q] * x0 * x0 + ic[q + 3] * x1 * x1 + ic[q + 5] * x2 * x2 +
                    2 * (ic[q + 1] * x0 * x1 + ic[q + 2] * x0 * x2 + ic[q + 4] * x1 * x2));
            }
            terms[c] = v;
            if (v > top) top = v;
        }
        return top;
    }

    function gmmBest(gmm, col, p) {
        const terms = new Float64Array(GMM_K);
        gmmTerms(gmm, col[p], col[p + 1], col[p + 2], terms);
        let best = 0;
        for (let c = 1; c < gmm.k; c++) if (terms[c] > terms[best]) best = c;
        return best;
    }

    // -log of the mixture density (log-sum-exp over the components) for all n pixels. Components more than 14 nats below
    // the best one add less than 1e-6 of the density and are skipped.
    function gmmCosts(gmm, col, n, out) {
        const terms = new Float64Array(GMM_K);
        for (let p = 0; p < n; p++) {
            const top = gmmTerms(gmm, col[p * 3], col[p * 3 + 1], col[p * 3 + 2], terms);
            let sum = 1;
            for (let c = 0; c < gmm.k; c++) {
                const d = terms[c] - top;
                if (d > -14 && d < 0) sum += Math.exp(d);
            }
            out[p] = -(top + (sum > 1 ? Math.log(sum) : 0));
        }
    }

    // --- max-flow / min-cut on a 8-neighbour pixel grid (Boykov-Kolmogorov search trees) ---

    // rcap: residual capacity of the 8 arcs of every pixel (E, W, S, N, SE, NW, SW, NE; opposite arcs differ in bit 0),
    // trcap: terminal capacity (> 0 from the source, < 0 to the sink). Both are consumed. Returns 1 per pixel on the source side.
    function minCutGrid(w, h, rcap, trcap, valid) {
        const n = w * h;
        const OFF = [1, -1, w, -w, w + 1, -w - 1, w - 1, 1 - w];
        const NONE = -1, TERMINAL = -2, ORPHAN = -3, FAR = 0x3fffffff;
        const parent = new Int32Array(n).fill(NONE);
        const ts = new Int32Array(n), dist = new Int32Array(n);
        const sink = new Uint8Array(n);
        const link = new Int32Array(n).fill(-1);
        const orphans = new Int32Array(n + 1);
        const ocap = n + 1;
        let oh = 0, ot = 0;
        let q0f = -1, q0l = -1, q1f = -1, q1l = -1;
        let time = 0;

        const head = a => (a >> 3) + OFF[a & 7];
        const sister = a => head(a) * 8 + ((a & 7) ^ 1);
        const setActive = i => {
            if (link[i] >= 0) return;
            if (q1l >= 0) link[q1l] = i; else q1f = i;
            q1l = i;
            link[i] = i;
        };
        const nextActive = () => {
            for (;;) {
                let i = q0f;
                if (i < 0) {
                    q0f = i = q1f; q0l = q1l; q1f = -1; q1l = -1;
                    if (i < 0) return -1;
                }
                if (link[i] === i) { q0f = q0l = -1; } else q0f = link[i];
                link[i] = -1;
                if (parent[i] !== NONE) return i;
            }
        };
        const orphanFront = i => { parent[i] = ORPHAN; oh = (oh + ocap - 1) % ocap; orphans[oh] = i; };
        const orphanRear = i => { parent[i] = ORPHAN; orphans[ot] = i; ot = (ot + 1) % ocap; };

        for (let i = 0; i < n; i++) {
            if (trcap[i] !== 0) {
                sink[i] = trcap[i] < 0 ? 1 : 0;
                parent[i] = TERMINAL;
                dist[i] = 1;
                setActive(i);
            }
        }

        const augment = middle => {
            const srcNode = middle >> 3;
            const snkNode = head(middle);
            let bottleneck = rcap[middle];
            let i = srcNode, a;
            for (;;) {
                a = parent[i];
                if (a === TERMINAL) break;
                const c = rcap[sister(a)];
                if (c < bottleneck) bottleneck = c;
                i = head(a);
            }
            if (trcap[i] < bottleneck) bottleneck = trcap[i];
            const srcRoot = i;
            i = snkNode;
            for (;;) {
                a = parent[i];
                if (a === TERMINAL) break;
                const c = rcap[a];
                if (c < bottleneck) bottleneck = c;
                i = head(a);
            }
            if (-trcap[i] < bottleneck) bottleneck = -trcap[i];
            const snkRoot = i;

            rcap[sister(middle)] += bottleneck;
            rcap[middle] -= bottleneck;
            for (i = srcNode; ; ) {
                a = parent[i];
                if (a === TERMINAL) break;
                const s = sister(a);
                rcap[a] += bottleneck;
                rcap[s] -= bottleneck;
                const next = head(a);
                if (rcap[s] === 0) orphanFront(i);
                i = next;
            }
            trcap[srcRoot] -= bottleneck;
            if (trcap[srcRoot] === 0) orphanFront(srcRoot);
            for (i = snkNode; ; ) {
                a = parent[i];
                if (a === TERMINAL) break;
                const s = sister(a);
                rcap[s] += bottleneck;
                rcap[a] -= bottleneck;
                const next = head(a);
                if (rcap[a] === 0) orphanFront(i);
                i = next;
            }
            trcap[snkRoot] += bottleneck;
            if (trcap[snkRoot] === 0) orphanFront(snkRoot);
        };

        // Distance of j to its tree root along the parent links, or FAR when the chain ends in an orphan.
        const originDistance = j => {
            let d = 0;
            for (;;) {
                if (ts[j] === time) { d += dist[j]; break; }
                const a = parent[j];
                d++;
                if (a === TERMINAL) { ts[j] = time; dist[j] = 1; break; }
                if (a === ORPHAN) { d = FAR; break; }
                j = head(a);
            }
            return d;
        };

        const adopt = (i, wantSink) => {
            let best = -1, bestD = FAR;
            const mask = valid[i];
            for (let d = 0; d < 8; d++) {
                if (!(mask & (1 << d))) continue;
                const j = i + OFF[d];
                const usable = wantSink ? rcap[i * 8 + d] > 0 : rcap[j * 8 + (d ^ 1)] > 0;
                if (!usable || (sink[j] === 1) !== wantSink || parent[j] === NONE) continue;
                let dd = originDistance(j);
                if (dd < FAR) {
                    if (dd < bestD) { best = i * 8 + d; bestD = dd; }
                    for (let k = j; ts[k] !== time; k = head(parent[k])) { ts[k] = time; dist[k] = dd--; }
                }
            }
            if (best >= 0) {
                parent[i] = best;
                ts[i] = time;
                dist[i] = bestD + 1;
                return;
            }
            parent[i] = NONE;
            for (let d = 0; d < 8; d++) {
                if (!(mask & (1 << d))) continue;
                const j = i + OFF[d];
                const a = parent[j];
                if ((sink[j] === 1) !== wantSink || a === NONE) continue;
                if (wantSink ? rcap[i * 8 + d] > 0 : rcap[j * 8 + (d ^ 1)] > 0) setActive(j);
                if (a !== TERMINAL && a !== ORPHAN && head(a) === i) orphanRear(j);
            }
        };

        let current = -1;
        for (;;) {
            let i = current;
            if (i >= 0) {
                link[i] = -1;
                if (parent[i] === NONE) i = -1;
            }
            if (i < 0) {
                i = nextActive();
                if (i < 0) break;
            }
            let middle = -1;
            const mask = valid[i];
            if (sink[i] === 0) {
                for (let d = 0; d < 8; d++) {
                    if (!(mask & (1 << d)) || rcap[i * 8 + d] <= 0) continue;
                    const j = i + OFF[d];
                    if (parent[j] === NONE) {
                        sink[j] = 0; parent[j] = j * 8 + (d ^ 1); ts[j] = ts[i]; dist[j] = dist[i] + 1;
                        setActive(j);
                    } else if (sink[j] === 1) {
                        middle = i * 8 + d;
                        break;
                    } else if (ts[j] <= ts[i] && dist[j] > dist[i]) {
                        parent[j] = j * 8 + (d ^ 1); ts[j] = ts[i]; dist[j] = dist[i] + 1;
                    }
                }
            } else {
                for (let d = 0; d < 8; d++) {
                    if (!(mask & (1 << d))) continue;
                    const j = i + OFF[d];
                    if (rcap[j * 8 + (d ^ 1)] <= 0) continue;
                    if (parent[j] === NONE) {
                        sink[j] = 1; parent[j] = j * 8 + (d ^ 1); ts[j] = ts[i]; dist[j] = dist[i] + 1;
                        setActive(j);
                    } else if (sink[j] === 0) {
                        middle = j * 8 + (d ^ 1);
                        break;
                    } else if (ts[j] <= ts[i] && dist[j] > dist[i]) {
                        parent[j] = j * 8 + (d ^ 1); ts[j] = ts[i]; dist[j] = dist[i] + 1;
                    }
                }
            }
            time++;
            if (middle >= 0) {
                link[i] = i;
                current = i;
                augment(middle);
                while (oh !== ot) {
                    const o = orphans[oh];
                    oh = (oh + 1) % ocap;
                    adopt(o, sink[o] === 1);
                }
            } else {
                current = -1;
            }
        }

        const label = new Uint8Array(n);
        for (let i = 0; i < n; i++) label[i] = parent[i] !== NONE && sink[i] === 0 ? 1 : 0;
        return label;
    }

    // --- classic engine ---

    const SCALE = 16;       // capacities are integers in 1/16 nat
    const GAMMA = 50;       // smoothness weight, as in GrabCut
    const HARD = Math.round(9 * GAMMA * SCALE);

    function borderMask(w, h) {
        const band = Math.max(2, Math.round(Math.min(w, h) * 0.05));
        const mask = new Uint8Array(w * h);
        for (let y = 0; y < h; y++) {
            for (let x = 0; x < w; x++) {
                if (x < band || y < band || x >= w - band || y >= h - band) mask[y * w + x] = 1;
            }
        }
        return mask;
    }

    // Plain-background test: the border pixels sit close to one colour. Returns the model or null.
    function uniformBackground(col, ring, skip, n) {
        let cnt = 0, m0 = 0, m1 = 0, m2 = 0;
        for (let i = 0; i < n; i++) {
            if (!ring[i] || skip[i]) continue;
            cnt++; m0 += col[i * 3]; m1 += col[i * 3 + 1]; m2 += col[i * 3 + 2];
        }
        if (cnt < 8) return null;
        m0 /= cnt; m1 /= cnt; m2 /= cnt;
        let variance = 0;
        for (let i = 0; i < n; i++) {
            if (!ring[i] || skip[i]) continue;
            const dr = col[i * 3] - m0, dg = col[i * 3 + 1] - m1, db = col[i * 3 + 2] - m2;
            variance += dr * dr + dg * dg + db * db;
        }
        const dev = Math.sqrt(variance / cnt);
        let near = 0;
        for (let i = 0; i < n; i++) {
            if (!ring[i] || skip[i]) continue;
            const dr = col[i * 3] - m0, dg = col[i * 3 + 1] - m1, db = col[i * 3 + 2] - m2;
            if (Math.sqrt(dr * dr + dg * dg + db * db) < 2.5 * dev + 6) near++;
        }
        if (dev > 26 || near / cnt < 0.93) return null;
        return { mean: [m0, m1, m2], dev };
    }

    // Colour distance to the background mapped to a matte: 0 inside the noise of the background, 0.5 half way to the typical
    // subject colour (so a sharp edge lands on the right pixel), 1 at the typical subject distance.
    function keyMatte(col, model, n) {
        const t0 = Math.max(12, 2.4 * model.dev);
        const gate = t0 + Math.max(20, 1.3 * model.dev);
        const dist = new Float32Array(n);
        let far = 0;
        for (let i = 0; i < n; i++) {
            const dr = col[i * 3] - model.mean[0], dg = col[i * 3 + 1] - model.mean[1], db = col[i * 3 + 2] - model.mean[2];
            dist[i] = Math.sqrt(dr * dr + dg * dg + db * db);
            if (dist[i] > gate) far++;
        }
        let reach = gate * 2;
        if (far) {
            const sample = new Float32Array(far);
            for (let i = 0, k = 0; i < n; i++) if (dist[i] > gate) sample[k++] = dist[i];
            sample.sort();
            reach = sample[far >> 1];
        }
        const mid = Math.max(t0 + 8, reach * 0.5);
        const alpha = new Uint8ClampedArray(n);
        for (let i = 0; i < n; i++) alpha[i] = smoothstep((dist[i] - t0) / (2 * (mid - t0))) * 255;
        return alpha;
    }

    // Otsu threshold of values in [0, cap] using 64 bins; returns a value in the same unit.
    function otsu(values, count, cap) {
        const bins = 64;
        const hist = new Float64Array(bins);
        for (let i = 0; i < count; i++) hist[Math.min(bins - 1, Math.floor(Math.min(values[i], cap) / cap * bins))]++;
        let total = 0, sumAll = 0;
        for (let b = 0; b < bins; b++) { total += hist[b]; sumAll += b * hist[b]; }
        let wB = 0, sumB = 0, best = 0, bestVar = -1;
        for (let b = 0; b < bins; b++) {
            wB += hist[b];
            if (!wB) continue;
            const wF = total - wB;
            if (!wF) break;
            sumB += b * hist[b];
            const mB = sumB / wB, mF = (sumAll - sumB) / wF;
            const between = wB * wF * (mB - mF) * (mB - mF);
            if (between > bestVar) { bestVar = between; best = b; }
        }
        return (best + 1) / bins * cap;
    }

    // Starting guess without any user input: the border is background, what stands out from the border colours and sits
    // near the middle is the subject. Returns { fg, sureBg } (0/1 planes); sureBg is the border plus what clearly looks like it.
    function initialSubject(col, w, h, ring, skip, rng) {
        const n = w * h;
        let ringCount = 0;
        for (let i = 0; i < n; i++) if (ring[i] && !skip[i]) ringCount++;
        const pts = new Float32Array(Math.max(1, ringCount) * 3);
        let np = 0;
        for (let i = 0; i < n; i++) {
            if (!ring[i] || skip[i]) continue;
            pts[np * 3] = col[i * 3]; pts[np * 3 + 1] = col[i * 3 + 1]; pts[np * 3 + 2] = col[i * 3 + 2]; np++;
        }
        const k = Math.min(4, Math.max(1, np));
        const centers = np ? kmeans(pts, np, k, rng, 6) : new Float64Array([128, 128, 128]);
        const spread = new Float64Array(k), mass = new Int32Array(k);
        for (let i = 0; i < np; i++) {
            let best = 0, bestD = Infinity;
            for (let c = 0; c < k; c++) {
                const dr = pts[i * 3] - centers[c * 3], dg = pts[i * 3 + 1] - centers[c * 3 + 1], db = pts[i * 3 + 2] - centers[c * 3 + 2];
                const d = dr * dr + dg * dg + db * db;
                if (d < bestD) { bestD = d; best = c; }
            }
            spread[best] += bestD; mass[best]++;
        }
        // A subject that crosses the border adds a small cluster of its own; only the dominant colours count as background.
        const usable = [];
        let biggest = 0;
        for (let c = 0; c < k; c++) {
            if (mass[c] > mass[biggest]) biggest = c;
            if (mass[c] >= Math.max(3, np * 0.12)) usable.push({ r: centers[c * 3], g: centers[c * 3 + 1], b: centers[c * 3 + 2], s: Math.max(7, Math.sqrt(spread[c] / mass[c])) });
        }
        if (!usable.length) usable.push({ r: centers[biggest * 3], g: centers[biggest * 3 + 1], b: centers[biggest * 3 + 2], s: Math.max(7, mass[biggest] ? Math.sqrt(spread[biggest] / mass[biggest]) : 12) });
        const score = new Float32Array(n);
        const cx = (w - 1) / 2, cy = (h - 1) / 2, sx = w * 0.32, sy = h * 0.32;
        let m = 0;
        const interior = new Float32Array(n);
        for (let p = 0; p < n; p++) {
            let sal = Infinity;
            for (let u = 0; u < usable.length; u++) {
                const dr = col[p * 3] - usable[u].r, dg = col[p * 3 + 1] - usable[u].g, db = col[p * 3 + 2] - usable[u].b;
                const d = Math.sqrt(dr * dr + dg * dg + db * db) / usable[u].s;
                if (d < sal) sal = d;
            }
            const x = p % w, y = (p - x) / w;
            const prior = Math.exp(-(((x - cx) * (x - cx)) / (2 * sx * sx) + ((y - cy) * (y - cy)) / (2 * sy * sy)));
            score[p] = sal * (0.4 + 0.6 * prior);
            if (!ring[p]) interior[m++] = score[p];
        }
        let threshold = m ? otsu(interior, m, 24) : 6;
        threshold = Math.max(threshold, 2.5);
        const fg = new Uint8Array(n);
        let count = 0;
        const apply = t => { count = 0; for (let p = 0; p < n; p++) { fg[p] = !ring[p] && !skip[p] && score[p] > t ? 1 : 0; count += fg[p]; } };
        apply(threshold);
        if (count < n * 0.02 || count > n * 0.85) {
            // Otsu split the wrong way (tiny or huge): fall back to the central blob of the most salient pixels.
            const sorted = Float32Array.from(interior.subarray(0, m)).sort();
            const share = count < n * 0.02 ? 0.82 : 0.45;
            apply(sorted[Math.min(m - 1, Math.floor(m * share))]);
        }
        const sureBg = new Uint8Array(n);
        for (let p = 0; p < n; p++) sureBg[p] = (ring[p] || score[p] < 0.45 * threshold) && !fg[p] ? 1 : 0;
        return { fg, sureBg };
    }

    // GrabCut on the proxy: GMM colour models + min-cut with contrast-sensitive smoothness. States: 0 bg, 1 fg, 2 probably
    // bg, 3 probably fg. Returns a Uint8Array of 0/1 (1 = subject).
    // seedBg (optional): the pixels the first background model is learned from, instead of everything not labelled subject.
    function grabCut(col, w, h, states, hard, iterations, ctx, seedBg) {
        const n = w * h;
        const rng = Lib.mulberry32(20241021);
        // contrast-sensitive neighbour weights (E, S, SE, SW); beta from the mean squared colour difference
        let diffSum = 0, diffCount = 0;
        const pairs = [[1, 0], [0, 1], [1, 1], [-1, 1]];
        for (let y = 0; y < h; y++) {
            for (let x = 0; x < w; x++) {
                const p = y * w + x;
                for (let e = 0; e < 4; e++) {
                    const nx = x + pairs[e][0], ny = y + pairs[e][1];
                    if (nx < 0 || ny < 0 || nx >= w || ny >= h) continue;
                    const q = ny * w + nx;
                    const dr = col[p * 3] - col[q * 3], dg = col[p * 3 + 1] - col[q * 3 + 1], db = col[p * 3 + 2] - col[q * 3 + 2];
                    diffSum += dr * dr + dg * dg + db * db; diffCount++;
                }
            }
        }
        const beta = diffCount && diffSum > 1e-6 ? 1 / (2 * diffSum / diffCount) : 1;
        const baseCap = new Int32Array(n * 8);
        const valid = new Uint8Array(n);
        const dirOf = [[0, 1], [2, 3], [4, 5], [6, 7]];
        for (let y = 0; y < h; y++) {
            for (let x = 0; x < w; x++) {
                const p = y * w + x;
                for (let d = 0; d < 8; d++) {
                    const dx = d === 0 || d === 4 || d === 7 ? 1 : (d === 1 || d === 5 || d === 6 ? -1 : 0);
                    const dy = d === 2 || d === 4 || d === 6 ? 1 : (d === 3 || d === 5 || d === 7 ? -1 : 0);
                    const nx = x + dx, ny = y + dy;
                    if (nx >= 0 && ny >= 0 && nx < w && ny < h) valid[p] |= 1 << d;
                }
                for (let e = 0; e < 4; e++) {
                    const nx = x + pairs[e][0], ny = y + pairs[e][1];
                    if (nx < 0 || ny < 0 || nx >= w || ny >= h) continue;
                    const q = ny * w + nx;
                    const dr = col[p * 3] - col[q * 3], dg = col[p * 3 + 1] - col[q * 3 + 1], db = col[p * 3 + 2] - col[q * 3 + 2];
                    const weight = GAMMA * Math.exp(-beta * (dr * dr + dg * dg + db * db)) / (e >= 2 ? Math.SQRT2 : 1);
                    const cap = Math.round(weight * SCALE);
                    baseCap[p * 8 + dirOf[e][0]] = cap;
                    baseCap[q * 8 + dirOf[e][1]] = cap;
                }
            }
        }
        const rcap = new Int32Array(n * 8), trcap = new Int32Array(n);
        const costFg = new Float32Array(n), costBg = new Float32Array(n);
        let label = new Uint8Array(n);
        for (let it = 0; it < iterations; it++) {
            if (ctx && ctx.cancelled()) throw new Error('cancelled');
            const fgIdx = [];
            let bgIdx = [];
            for (let p = 0; p < n; p++) (states[p] & 1 ? fgIdx : bgIdx).push(p);
            if (it === 0 && seedBg) {
                const seeded = bgIdx.filter(p => seedBg[p]);
                if (seeded.length >= 64) bgIdx = seeded;
            }
            if (fgIdx.length < 8 || bgIdx.length < 8) break;
            const gFg = fitGmm(col, Int32Array.from(fgIdx), fgIdx.length, rng);
            const gBg = fitGmm(col, Int32Array.from(bgIdx), bgIdx.length, rng);
            gmmCosts(gFg, col, n, costFg);
            gmmCosts(gBg, col, n, costBg);
            rcap.set(baseCap);
            const limit = HARD - SCALE;
            for (let p = 0; p < n; p++) {
                if (hard[p] === 1) trcap[p] = HARD;
                else if (hard[p] === 2) trcap[p] = -HARD;
                else trcap[p] = clamp(Math.round((costBg[p] - costFg[p]) * SCALE), -limit, limit);
            }
            label = minCutGrid(w, h, rcap, trcap, valid);
            let changed = 0;
            for (let p = 0; p < n; p++) {
                if (hard[p]) continue;
                const next = label[p] ? 3 : 2;
                if (next !== states[p]) { changed++; states[p] = next; }
            }
            if (ctx) ctx.progress(0.25 + 0.6 * (it + 1) / iterations);
            if (it > 0 && changed < n * 0.0005) break;
        }
        const out = new Uint8Array(n);
        for (let p = 0; p < n; p++) out[p] = hard[p] === 1 ? 1 : (hard[p] === 2 ? 0 : (states[p] & 1));
        return out;
    }

    function planeFlags(plane, n) {
        if (!plane) return null;
        const out = new Uint8Array(n);
        for (let i = 0; i < n; i++) out[i] = plane[i] > 127 ? 1 : 0;
        return out;
    }

    // args: { image (a proxy, 512 px long edge is plenty), scribbles?: { fg, bg } (planes, > 127 = hint),
    //         init?: matte used as the starting labels, method?: 'auto' | 'key' | 'grabcut', iterations? }
    // Returns { alpha, method } at the size of the image.
    function classic(args, ctx) {
        const img = args.image;
        const w = img.width, h = img.height, n = w * h;
        if (!(w > 0 && h > 0)) throw new Error('cutout.classic: empty image');
        const progress = f => ctx.progress(f);
        const fgHint = planeFlags(args.scribbles && args.scribbles.fg, n);
        const bgHint = planeFlags(args.scribbles && args.scribbles.bg, n);
        const init = args.init ? planeFlags(args.init, n) : null;
        const rng = Lib.mulberry32(7919);
        const transparent = new Uint8Array(n);
        for (let i = 0; i < n; i++) transparent[i] = img.data[i * 4 + 3] < 16 ? 1 : 0;
        const col = smoothedColors(img.data, w, h);
        const ring = borderMask(w, h);
        progress(0.05);

        const wanted = args.method || 'auto';
        const hinted = !!(fgHint || bgHint || init);
        const model = wanted === 'grabcut' || (hinted && wanted === 'auto') ? null : uniformBackground(col, ring, transparent, n);
        if (model || wanted === 'key') {
            const usable = model || uniformBackground(col, ring, transparent, n) || { mean: [col[0], col[1], col[2]], dev: 6 };
            const alpha = keyMatte(col, usable, n);
            for (let i = 0; i < n; i++) if (transparent[i]) alpha[i] = 0;
            cleanMatte(alpha, w, h, 16, fgHint, bgHint);
            // only the edge keeps its softness; a light patch inside the subject must not stay see-through
            const solid = new Uint8ClampedArray(n);
            for (let i = 0; i < n; i++) solid[i] = alpha[i] > 127 ? 255 : 0;
            const inner = shiftEdge(solid, w, h, -3);
            for (let i = 0; i < n; i++) if (inner[i]) alpha[i] = 255;
            progress(1);
            return { alpha, method: 'key' };
        }

        const hard = new Uint8Array(n);
        for (let i = 0; i < n; i++) {
            if (fgHint && fgHint[i]) hard[i] = 1;
            else if ((bgHint && bgHint[i]) || transparent[i]) hard[i] = 2;
        }
        const states = new Uint8Array(n);
        let seedBg = null;
        if (init) {
            for (let i = 0; i < n; i++) states[i] = hard[i] === 1 ? 1 : (hard[i] === 2 ? 0 : (init[i] ? 3 : 2));
        } else {
            const guess = initialSubject(col, w, h, ring, transparent, rng);
            for (let i = 0; i < n; i++) states[i] = hard[i] === 1 ? 1 : (hard[i] === 2 ? 0 : (guess.fg[i] ? 3 : 2));
            seedBg = guess.sureBg;
        }
        progress(0.2);
        const iterations = clamp(Math.round(args.iterations || 4), 1, 8);
        const binary = grabCut(col, w, h, states, hard, iterations, ctx, seedBg);
        let on = 0;
        for (let i = 0; i < n; i++) on += binary[i];
        if (on < n * 0.003 || on > n * 0.985) {
            // The model collapsed: fall back to the guess so the user still gets something to refine.
            const guess = initialSubject(col, w, h, ring, transparent, rng);
            for (let i = 0; i < n; i++) binary[i] = hard[i] === 1 ? 1 : (hard[i] === 2 ? 0 : guess.fg[i]);
        }
        const hard255 = new Uint8ClampedArray(n);
        for (let i = 0; i < n; i++) hard255[i] = binary[i] ? 255 : 0;
        cleanMatte(hard255, w, h, 127, fgHint, bgHint);
        // soft edge that follows the image edges
        const p = new Float32Array(n);
        for (let i = 0; i < n; i++) p[i] = hard255[i] / 255;
        const alpha = guidedApply(img, guidedCoefficients(img, p, 2, 0.002));
        for (let i = 0; i < n; i++) {
            if (fgHint && fgHint[i]) alpha[i] = 255;
            else if ((bgHint && bgHint[i]) || transparent[i]) alpha[i] = 0;
        }
        progress(1);
        return { alpha, method: 'grabcut' };
    }

    // --- colour key ---

    function rgbToYcc(r, g, b) {
        return [0.299 * r + 0.587 * g + 0.114 * b, 128 - 0.168736 * r - 0.331264 * g + 0.5 * b, 128 + 0.5 * r - 0.418688 * g - 0.081312 * b];
    }

    // args: { image, color: '#rrggbb' | [r, g, b] | 'auto' (mean of the border), tolerance 0..100, softness 0..100, despill 0..100 }
    function chroma(args, ctx) {
        const img = args.image;
        const w = img.width, h = img.height, n = w * h, d = img.data;
        let key = null;
        if (Array.isArray(args.color)) key = args.color.slice(0, 3);
        else if (typeof args.color === 'string' && args.color !== 'auto') {
            const parsed = Lib.parseColor(args.color);
            if (parsed) key = parsed.slice(0, 3);
        }
        if (!key) {
            const ring = borderMask(w, h);
            let cnt = 0, r = 0, g = 0, b = 0;
            for (let i = 0; i < n; i++) {
                if (!ring[i] || d[i * 4 + 3] < 16) continue;
                cnt++; r += d[i * 4]; g += d[i * 4 + 1]; b += d[i * 4 + 2];
            }
            key = cnt ? [r / cnt, g / cnt, b / cnt] : [0, 255, 0];
        }
        const tolerance = clamp(args.tolerance == null ? 40 : Number(args.tolerance), 0, 100) / 100;
        const softness = clamp(args.softness == null ? 20 : Number(args.softness), 0, 100) / 100;
        const despill = clamp(args.despill == null ? 50 : Number(args.despill), 0, 100) / 100;
        const [ky, kcb, kcr] = rgbToYcc(key[0], key[1], key[2]);
        const saturation = Math.hypot(kcb - 128, kcr - 128);
        const lumaWeight = clamp(1 - saturation / 70, 0.15, 1);
        const d0 = tolerance * 140, ramp = 4 + softness * 100;
        // spill: the channel the key colour is made of is pulled down to the mean of the other two
        const channels = [key[0], key[1], key[2]];
        let dom = 0;
        for (let c = 1; c < 3; c++) if (channels[c] > channels[dom]) dom = c;
        const others = [0, 1, 2].filter(c => c !== dom);
        const spill = despill > 0 && channels[dom] - Math.max(channels[others[0]], channels[others[1]]) > 24;
        const alpha = new Uint8ClampedArray(n);
        const out = new Uint8ClampedArray(n * 4);
        for (let i = 0; i < n; i++) {
            const q = i * 4;
            const r = d[q], g = d[q + 1], b = d[q + 2];
            const ycc = rgbToYcc(r, g, b);
            const dist = Math.hypot(Math.hypot(ycc[1] - kcb, ycc[2] - kcr), (ycc[0] - ky) * lumaWeight);
            const a = smoothstep((dist - d0) / ramp);
            alpha[i] = a * 255 * d[q + 3] / 255;
            out[q] = r; out[q + 1] = g; out[q + 2] = b;
            if (spill) {
                const limit = (out[q + others[0]] + out[q + others[1]]) / 2;
                if (out[q + dom] > limit) out[q + dom] = out[q + dom] - (out[q + dom] - limit) * despill;
            }
            out[q + 3] = alpha[i];
        }
        ctx.progress(1);
        return { alpha, image: { width: w, height: h, data: out } };
    }

    // --- refine ---

    // args: { image?: RGBA of the final size (guide), alpha: matte of any size (alphaWidth / alphaHeight, default the image
    //         size), width / height when there is no image, radius (guide window, px), eps, shift (-10..10 px), smooth (0..10),
    //         feather (0..20 px) }. Returns { alpha } at the final size.
    function refine(args, ctx) {
        const image = args.image || null;
        const W = image ? image.width : (args.width || args.alphaWidth);
        const H = image ? image.height : (args.height || args.alphaHeight);
        if (!(W > 0 && H > 0)) throw new Error('cutout.refine: no size');
        const aw = args.alphaWidth || W, ah = args.alphaHeight || H;
        if (!args.alpha || args.alpha.length !== aw * ah) throw new Error('cutout.refine: matte size does not match');
        const ratio = Math.max(1, W / aw, H / ah);
        const radius = args.radius == null ? clamp(Math.round(2 * ratio + 2), 1, 64) : Math.max(0, Number(args.radius) || 0);
        const eps = args.eps == null ? 0.002 : Math.max(1e-6, Number(args.eps) || 0);
        ctx.progress(0.05);
        let alpha;
        if (image && radius > 0) alpha = guidedMatte(image, args.alpha, aw, ah, radius, eps, args.workEdge);
        else alpha = resizeMask(args.alpha, aw, ah, W, H);
        ctx.progress(0.6);
        alpha = shiftEdge(alpha, W, H, Number(args.shift) || 0);
        alpha = smoothEdge(alpha, W, H, args.smooth);
        const feather = clamp(Number(args.feather) || 0, 0, 64);
        if (feather > 0) alpha = Lib.featherAlpha(alpha, W, H, feather);
        ctx.progress(1);
        return { alpha: alpha === args.alpha ? Uint8ClampedArray.from(alpha) : alpha };
    }

    // --- decontaminate (blur-fusion foreground estimation, Forte and Pitie) ---

    // One fusion pass over a tile: F and B are estimated from window averages of the subject / background weighted colours.
    function fusePass(img3, f3, b3, alpha, w, h, r) {
        const n = w * h;
        const blurA = boxMean(alpha, w, h, r);
        const inv = new Float32Array(n);
        for (let i = 0; i < n; i++) inv[i] = 1 - alpha[i];
        const tmp = new Float32Array(n);
        const outF = [new Float32Array(n), new Float32Array(n), new Float32Array(n)];
        const outB = [new Float32Array(n), new Float32Array(n), new Float32Array(n)];
        for (let c = 0; c < 3; c++) {
            for (let i = 0; i < n; i++) tmp[i] = f3[c][i] * alpha[i];
            const bf = boxMean(tmp, w, h, r);
            for (let i = 0; i < n; i++) tmp[i] = b3[c][i] * inv[i];
            const bb = boxMean(tmp, w, h, r);
            for (let i = 0; i < n; i++) {
                const fm = bf[i] / (blurA[i] + 1e-5);
                const bm = bb[i] / ((1 - blurA[i]) + 1e-5);
                const a = alpha[i];
                const f = fm + a * (img3[c][i] - a * fm - (1 - a) * bm);
                outF[c][i] = f < 0 ? 0 : (f > 1 ? 1 : f);
                outB[c][i] = bm;
            }
        }
        return { f: outF, b: outB };
    }

    // args: { image, alpha (same size), radius (window of the fine pass, default 6) }. Returns { image }: colours with the
    // background spill removed in the transition band, alpha = the matte.
    function decontaminate(args, ctx) {
        const img = args.image, alpha = args.alpha;
        const w = img.width, h = img.height, d = img.data;
        if (!alpha || alpha.length !== w * h) throw new Error('cutout.decontaminate: matte size does not match');
        const fine = Math.max(1, Math.round((Number(args.radius) || 6) / 2));
        const wide = Math.max(2, Math.round(fine * 15));
        const halo = wide + fine + 2;
        const tile = 256;
        const out = new Uint8ClampedArray(d);
        for (let i = 0; i < w * h; i++) out[i * 4 + 3] = alpha[i];
        const tilesX = Math.ceil(w / tile), tilesY = Math.ceil(h / tile);
        for (let ty = 0; ty < tilesY; ty++) {
            for (let tx = 0; tx < tilesX; tx++) {
                const x0 = tx * tile, y0 = ty * tile, x1 = Math.min(w, x0 + tile), y1 = Math.min(h, y0 + tile);
                let band = false;
                for (let y = y0; y < y1 && !band; y++) {
                    for (let x = x0; x < x1; x++) {
                        const a = alpha[y * w + x];
                        if (a > 0 && a < 255) { band = true; break; }
                    }
                }
                if (!band) continue;
                if (ctx.cancelled()) throw new Error('cancelled');
                const ex0 = Math.max(0, x0 - halo), ey0 = Math.max(0, y0 - halo), ex1 = Math.min(w, x1 + halo), ey1 = Math.min(h, y1 + halo);
                const tw = ex1 - ex0, th = ey1 - ey0, tn = tw * th;
                const img3 = [new Float32Array(tn), new Float32Array(tn), new Float32Array(tn)];
                const a = new Float32Array(tn);
                for (let y = 0; y < th; y++) {
                    for (let x = 0; x < tw; x++) {
                        const p = (ey0 + y) * w + ex0 + x, q = p * 4, o = y * tw + x;
                        img3[0][o] = d[q] / 255; img3[1][o] = d[q + 1] / 255; img3[2][o] = d[q + 2] / 255;
                        a[o] = alpha[p] / 255;
                    }
                }
                const first = fusePass(img3, img3, img3, a, tw, th, wide);
                const second = fusePass(img3, first.f, first.b, a, tw, th, fine);
                for (let y = y0; y < y1; y++) {
                    for (let x = x0; x < x1; x++) {
                        const p = y * w + x;
                        if (!(alpha[p] > 0 && alpha[p] < 255)) continue;
                        const o = (y - ey0) * tw + (x - ex0), q = p * 4;
                        out[q] = second.f[0][o] * 255; out[q + 1] = second.f[1][o] * 255; out[q + 2] = second.f[2][o] * 255;
                    }
                }
            }
            ctx.progress((ty + 1) / tilesY);
        }
        return { image: { width: w, height: h, data: out } };
    }

    // --- background blur that leaves the subject out ---

    // args: { image, alpha (same size, 255 = subject), radius (gaussian sigma = radius / 2, px) }. Returns { image }.
    function blurBackground(args, ctx) {
        const img = args.image, alpha = args.alpha;
        const W = img.width, H = img.height;
        if (!alpha || alpha.length !== W * H) throw new Error('cutout.blur: matte size does not match');
        const radius = Math.max(0, Number(args.radius) || 0);
        if (radius < 0.5) return { image: { width: W, height: H, data: Uint8ClampedArray.from(img.data) } };
        // heavy blurs are smooth, so they run on a reduced copy and are scaled back up
        let scale = Math.max(1, Math.floor(radius / 4));
        while ((W / scale) * (H / scale) > 4000000) scale++;
        const lw = Math.max(1, Math.round(W / scale)), lh = Math.max(1, Math.round(H / scale)), ln = lw * lh;
        const small = scale > 1 ? Lib.resizeBilinear(img, lw, lh) : img;
        const am = scale > 1 ? resizePlane(alpha, W, H, lw, lh) : Float32Array.from(alpha);
        const sigma = radius / 2 / scale;
        const half = Math.max(1, Math.round((Math.sqrt(4 * sigma * sigma + 1) - 1) / 2));
        const weight = new Float32Array(ln);
        const planes = [new Float32Array(ln), new Float32Array(ln), new Float32Array(ln)];
        for (let i = 0; i < ln; i++) {
            // background pixels carry full weight; subject pixels a sliver, so the blur inside the subject is only a fallback
            const wgt = ((255 - am[i]) / 255 * 0.999 + 0.001) * (small.data[i * 4 + 3] / 255);
            weight[i] = wgt;
            planes[0][i] = small.data[i * 4] * wgt; planes[1][i] = small.data[i * 4 + 1] * wgt; planes[2][i] = small.data[i * 4 + 2] * wgt;
        }
        let bw = weight, bp = planes;
        for (let pass = 0; pass < 3; pass++) {
            bw = boxMean(bw, lw, lh, half);
            bp = bp.map(plane => boxMean(plane, lw, lh, half));
            ctx.progress((pass + 1) / 4);
        }
        const colour = [new Float32Array(ln), new Float32Array(ln), new Float32Array(ln)];
        for (let i = 0; i < ln; i++) {
            const inv = 1 / Math.max(bw[i], 1e-6);
            colour[0][i] = bp[0][i] * inv; colour[1][i] = bp[1][i] * inv; colour[2][i] = bp[2][i] * inv;
        }
        const up = scale > 1 ? colour.map(plane => resizePlane(plane, lw, lh, W, H)) : colour;
        const data = new Uint8ClampedArray(W * H * 4);
        for (let i = 0; i < W * H; i++) {
            data[i * 4] = up[0][i]; data[i * 4 + 1] = up[1][i]; data[i * 4 + 2] = up[2][i]; data[i * 4 + 3] = img.data[i * 4 + 3];
        }
        ctx.progress(1);
        return { image: { width: W, height: H, data } };
    }

    Lib.registerOp('cutout.classic', (args, ctx) => classic(args, quiet(ctx)));
    Lib.registerOp('cutout.chroma', (args, ctx) => chroma(args, quiet(ctx)));
    Lib.registerOp('cutout.refine', (args, ctx) => refine(args, quiet(ctx)));
    Lib.registerOp('cutout.decontaminate', (args, ctx) => decontaminate(args, quiet(ctx)));
    Lib.registerOp('cutout.blur', (args, ctx) => blurBackground(args, quiet(ctx)));

    // The same functions for direct calls (tests, other libs); the ops above are what the Pixel worker runs.
    Lib.cutout = {
        classic: (args, ctx) => classic(args, quiet(ctx)),
        chroma: (args, ctx) => chroma(args, quiet(ctx)),
        refine: (args, ctx) => refine(args, quiet(ctx)),
        decontaminate: (args, ctx) => decontaminate(args, quiet(ctx)),
        blur: (args, ctx) => blurBackground(args, quiet(ctx)),
        shiftEdge, smoothEdge, cleanMatte, minCutGrid, resizeMask, labelComponents
    };
})(typeof self !== 'undefined' ? self : (typeof globalThis !== 'undefined' ? globalThis : this));
