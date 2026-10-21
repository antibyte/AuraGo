// Pixel pure image library, WP7 "intelligence" part. DOM-free: loads in a page, in the Pixel worker and in a node vm.
// Contents: multi-scale PatchMatch inpainting (op `inpaint.fill`, content-aware fill / remove object), colour palette
// extraction (k-means in Lab), colour harmonies and content bounds (trim / auto-crop). Needs pixel-lib-core.js first.
// Images are { width, height, data: Uint8ClampedArray RGBA }, masks are Uint8ClampedArray(width * height).
(function (root) {
    'use strict';

    const Lib = root.AuraPixelLib = root.AuraPixelLib || {};
    if (typeof Lib.registerOp !== 'function' || typeof Lib.mulberry32 !== 'function') {
        throw new Error('pixel-lib-inpaint.js needs pixel-lib-core.js to be loaded first');
    }

    function abortError() {
        const err = new Error('Aborted');
        err.name = 'AbortError';
        return err;
    }

    function intOption(value, lo, hi, fallback) {
        const n = Math.round(Number(value));
        return Number.isFinite(n) ? Math.max(lo, Math.min(hi, n)) : fallback;
    }

    // --- inpainting (PatchMatch, Barnes et al. 2009, with the Wexler et al. 2007 coarse-to-fine reconstruction) ---
    //
    // The hole is filled from the pixels around it. Per pyramid level: random nearest-neighbour field (NNF) from every
    // target patch (a patch that touches the hole) to a source patch (a patch without hole pixels), then iterations of
    // propagation + random search, each followed by a vote that rebuilds the hole pixels from the overlapping patches.
    // The coarse NNF is upsampled to start the next finer level. Pixels are premultiplied floats so transparency works.

    const MIN_SOURCES = 16;
    const COARSEST_EDGE = 40;

    // Region of the image that takes part in a fill: the hole bounds plus a margin of source pixels around them.
    // options.margin: 'auto' (default), 'all' or a pixel count.
    function inpaintRegion(box, width, height, options) {
        const o = options || {};
        if (!box || !(width > 0) || !(height > 0)) return null;
        const patch = intOption(o.patch, 3, 15, 7);
        let margin;
        if (o.margin === 'all') margin = Math.max(width, height);
        else if (typeof o.margin === 'number' && o.margin >= 0) margin = Math.round(o.margin);
        else margin = Math.max(64, box.w, box.h);
        margin = Math.max(margin, patch * 2);
        const x0 = Math.max(0, box.x - margin), y0 = Math.max(0, box.y - margin);
        const x1 = Math.min(width, box.x + box.w + margin), y1 = Math.min(height, box.y + box.h + margin);
        return { x: x0, y: y0, w: x1 - x0, h: y1 - y0 };
    }

    // Floats (premultiplied RGBA 0..255) and hole flags of an RGBA crop.
    function buildWork(data, mask, w, h, threshold) {
        const img = new Float32Array(w * h * 4);
        const hole = new Uint8Array(w * h);
        let known = 0, useAlpha = false;
        for (let i = 0, n = w * h; i < n; i++) {
            const o = i * 4, a = data[o + 3], f = a / 255;
            img[o] = data[o] * f;
            img[o + 1] = data[o + 1] * f;
            img[o + 2] = data[o + 2] * f;
            img[o + 3] = a;
            if (mask[i] > threshold) {
                hole[i] = 1;
            } else {
                known++;
                if (a < 255) useAlpha = true;
            }
        }
        return { w, h, img, hole, known, useAlpha };
    }

    function makeLevel(w, h, img, hole) {
        return { w, h, img, hole, pr: 0, valid: null, validList: null, targets: null, nx: null, ny: null, nd: null, acc: null, ws: null };
    }

    // Half-size level: known pixels are averaged, a block that contains any hole pixel is a hole (the hole only grows).
    function downLevel(L) {
        const w = (L.w + 1) >> 1, h = (L.h + 1) >> 1;
        const img = new Float32Array(w * h * 4);
        const hole = new Uint8Array(w * h);
        const s = L.img;
        for (let y = 0; y < h; y++) {
            for (let x = 0; x < w; x++) {
                let n = 0, r = 0, g = 0, b = 0, a = 0, anyHole = 0;
                for (let dy = 0; dy < 2; dy++) {
                    const sy = Math.min(L.h - 1, y * 2 + dy);
                    for (let dx = 0; dx < 2; dx++) {
                        const si = sy * L.w + Math.min(L.w - 1, x * 2 + dx);
                        if (L.hole[si]) { anyHole = 1; continue; }
                        const o = si * 4;
                        n++;
                        r += s[o]; g += s[o + 1]; b += s[o + 2]; a += s[o + 3];
                    }
                }
                const i = y * w + x;
                hole[i] = anyHole;
                if (n) {
                    const o = i * 4;
                    img[o] = r / n; img[o + 1] = g / n; img[o + 2] = b / n; img[o + 3] = a / n;
                }
            }
        }
        return makeLevel(w, h, img, hole);
    }

    // 1 where the (2r+1)^2 window around the pixel contains a hole pixel.
    function dilateAny(hole, w, h, r) {
        const tmp = new Uint8Array(w * h), out = new Uint8Array(w * h);
        for (let y = 0; y < h; y++) {
            const row = y * w;
            let c = 0;
            for (let k = 0; k <= Math.min(r, w - 1); k++) c += hole[row + k];
            for (let x = 0; x < w; x++) {
                tmp[row + x] = c > 0 ? 1 : 0;
                const add = x + r + 1, sub = x - r;
                if (add < w) c += hole[row + add];
                if (sub >= 0) c -= hole[row + sub];
            }
        }
        for (let x = 0; x < w; x++) {
            let c = 0;
            for (let k = 0; k <= Math.min(r, h - 1); k++) c += tmp[k * w + x];
            for (let y = 0; y < h; y++) {
                out[y * w + x] = c > 0 ? 1 : 0;
                const add = y + r + 1, sub = y - r;
                if (add < h) c += tmp[add * w + x];
                if (sub >= 0) c -= tmp[sub * w + x];
            }
        }
        return out;
    }

    // Patch radius, source positions (patch fully known, fully inside) and target positions (patch touches the hole).
    function prepareLevel(L, patch) {
        const w = L.w, h = L.h;
        const pr = Math.min(patch >> 1, (Math.min(w, h) - 1) >> 1);
        L.pr = pr;
        L.valid = null; L.validList = null; L.targets = null;
        if (pr < 1) return;
        const near = dilateAny(L.hole, w, h, pr);
        const valid = new Uint8Array(w * h);
        let nv = 0, nt = 0;
        for (let y = pr; y < h - pr; y++) {
            for (let x = pr; x < w - pr; x++) {
                if (near[y * w + x]) nt++;
                else { valid[y * w + x] = 1; nv++; }
            }
        }
        const validList = new Int32Array(nv), targets = new Int32Array(nt);
        let iv = 0, it = 0;
        for (let y = pr; y < h - pr; y++) {
            for (let x = pr; x < w - pr; x++) {
                const i = y * w + x;
                if (valid[i]) validList[iv++] = i;
                else targets[it++] = i;
            }
        }
        L.valid = valid; L.validList = validList; L.targets = targets;
        L.nx = new Int32Array(w * h);
        L.ny = new Int32Array(w * h);
        L.nd = new Float32Array(w * h).fill(Infinity);
    }

    // Flat start for the coarsest level: the colours of the known neighbours flow inwards ring by ring.
    function diffuseFill(L) {
        const w = L.w, h = L.h, img = L.img;
        const known = new Uint8Array(w * h);
        let pending = [];
        for (let i = 0; i < w * h; i++) {
            if (L.hole[i]) pending.push(i);
            else known[i] = 1;
        }
        if (pending.length === w * h) return false;
        while (pending.length) {
            const next = [], done = [];
            for (let k = 0; k < pending.length; k++) {
                const i = pending[k];
                const y = (i / w) | 0, x = i - y * w;
                let c = 0, r = 0, g = 0, b = 0, a = 0;
                for (let dy = -1; dy <= 1; dy++) {
                    const ny = y + dy;
                    if (ny < 0 || ny >= h) continue;
                    for (let dx = -1; dx <= 1; dx++) {
                        const nx = x + dx;
                        if ((!dx && !dy) || nx < 0 || nx >= w) continue;
                        const j = ny * w + nx;
                        if (!known[j]) continue;
                        const o = j * 4;
                        c++;
                        r += img[o]; g += img[o + 1]; b += img[o + 2]; a += img[o + 3];
                    }
                }
                if (c) {
                    const o = i * 4;
                    img[o] = r / c; img[o + 1] = g / c; img[o + 2] = b / c; img[o + 3] = a / c;
                    done.push(i);
                } else {
                    next.push(i);
                }
            }
            if (!done.length) break;
            for (let k = 0; k < done.length; k++) known[done[k]] = 1;
            pending = next;
        }
        return true;
    }

    // Sum of squared differences of two patches; stops as soon as the sum reaches the cutoff.
    function patchDist(img, w, pr, useAlpha, px, py, qx, qy, cutoff) {
        let sum = 0;
        const span = pr * 2 + 1;
        for (let dy = -pr; dy <= pr; dy++) {
            let a = ((py + dy) * w + px - pr) * 4;
            let b = ((qy + dy) * w + qx - pr) * 4;
            const end = a + span * 4;
            if (useAlpha) {
                for (; a < end; a += 4, b += 4) {
                    const d0 = img[a] - img[b], d1 = img[a + 1] - img[b + 1], d2 = img[a + 2] - img[b + 2], d3 = img[a + 3] - img[b + 3];
                    sum += d0 * d0 + d1 * d1 + d2 * d2 + d3 * d3;
                }
            } else {
                for (; a < end; a += 4, b += 4) {
                    const d0 = img[a] - img[b], d1 = img[a + 1] - img[b + 1], d2 = img[a + 2] - img[b + 2];
                    sum += d0 * d0 + d1 * d1 + d2 * d2;
                }
            }
            if (sum >= cutoff) return sum;
        }
        return sum;
    }

    function randomNNF(L, rand) {
        const list = L.validList, n = list.length, w = L.w, t = L.targets;
        for (let k = 0; k < t.length; k++) {
            const q = list[(rand() * n) | 0];
            const qy = (q / w) | 0;
            L.ny[t[k]] = qy;
            L.nx[t[k]] = q - qy * w;
        }
    }

    // Distances of every target to its current source (the target pixels changed since the last pass).
    function computeDist(L, useAlpha) {
        const t = L.targets, w = L.w;
        for (let k = 0; k < t.length; k++) {
            const p = t[k], py = (p / w) | 0, px = p - py * w;
            L.nd[p] = patchDist(L.img, w, L.pr, useAlpha, px, py, L.nx[p], L.ny[p], Infinity);
        }
    }

    // One PatchMatch sweep over the targets: propagation from the already visited neighbours, then random search.
    function pmPass(L, useAlpha, reverse, rand, rmax) {
        const w = L.w, h = L.h, img = L.img, pr = L.pr, valid = L.valid, nx = L.nx, ny = L.ny, nd = L.nd, targets = L.targets;
        const total = targets.length, hiX = w - pr, hiY = h - pr;
        const step = reverse ? 1 : -1;
        for (let k = 0; k < total; k++) {
            const p = targets[reverse ? total - 1 - k : k];
            const py = (p / w) | 0, px = p - py * w;
            let bx = nx[p], by = ny[p], bd = nd[p];
            if (px + step >= 0 && px + step < w && nd[p + step] !== Infinity) {
                const cx = nx[p + step] - step, cy = ny[p + step];
                if (cx >= pr && cx < hiX && cy >= pr && cy < hiY && valid[cy * w + cx] && (cx !== bx || cy !== by)) {
                    const d = patchDist(img, w, pr, useAlpha, px, py, cx, cy, bd);
                    if (d < bd) { bd = d; bx = cx; by = cy; }
                }
            }
            if (py + step >= 0 && py + step < h && nd[p + step * w] !== Infinity) {
                const cx = nx[p + step * w], cy = ny[p + step * w] - step;
                if (cx >= pr && cx < hiX && cy >= pr && cy < hiY && valid[cy * w + cx] && (cx !== bx || cy !== by)) {
                    const d = patchDist(img, w, pr, useAlpha, px, py, cx, cy, bd);
                    if (d < bd) { bd = d; bx = cx; by = cy; }
                }
            }
            for (let r = rmax; r >= 1 && bd > 0; r >>= 1) {
                const cx = bx + ((rand() * (2 * r + 1)) | 0) - r;
                const cy = by + ((rand() * (2 * r + 1)) | 0) - r;
                if (cx < pr || cx >= hiX || cy < pr || cy >= hiY || !valid[cy * w + cx]) continue;
                const d = patchDist(img, w, pr, useAlpha, px, py, cx, cy, bd);
                if (d < bd) { bd = d; bx = cx; by = cy; }
            }
            nx[p] = bx; ny[p] = by; nd[p] = bd;
        }
    }

    function patchKernel(pr) {
        const span = pr * 2 + 1, sigma = Math.max(1, pr * 0.5);
        const kernel = new Float32Array(span * span);
        for (let y = -pr; y <= pr; y++) {
            for (let x = -pr; x <= pr; x++) kernel[(y + pr) * span + x + pr] = Math.exp(-(x * x + y * y) / (2 * sigma * sigma));
        }
        return kernel;
    }

    // Rebuilds the hole pixels as the weighted mean of what every overlapping target patch says about them.
    // weighted: patches that match well count more (sigma = 75th percentile of the match errors).
    function vote(L, useAlpha, weighted, kernel) {
        const w = L.w, img = L.img, hole = L.hole, pr = L.pr, nx = L.nx, ny = L.ny, nd = L.nd, targets = L.targets;
        const total = targets.length;
        if (!total) return;
        const span = pr * 2 + 1, norm = span * span * (useAlpha ? 4 : 3);
        if (!L.acc) { L.acc = new Float32Array(w * L.h * 4); L.ws = new Float32Array(w * L.h); }
        const acc = L.acc, ws = L.ws;
        acc.fill(0);
        ws.fill(0);
        let sigma2 = 1;
        if (weighted) {
            const stride = Math.max(1, Math.floor(total / 2048));
            const sample = [];
            for (let k = 0; k < total; k += stride) sample.push(nd[targets[k]] / norm);
            sample.sort((a, b) => a - b);
            sigma2 = Math.max(1, sample[Math.floor(sample.length * 0.75)] || 1);
        }
        for (let k = 0; k < total; k++) {
            const p = targets[k], py = (p / w) | 0, px = p - py * w;
            const qx = nx[p], qy = ny[p];
            const wp = weighted ? Math.exp(-(nd[p] / norm) / (2 * sigma2)) : 1;
            for (let dy = -pr; dy <= pr; dy++) {
                const ti = (py + dy) * w + px - pr, si = (qy + dy) * w + qx - pr, ki = (dy + pr) * span;
                for (let dx = 0; dx < span; dx++) {
                    const t = ti + dx;
                    if (!hole[t]) continue;
                    const wt = wp * kernel[ki + dx];
                    const s4 = (si + dx) * 4, t4 = t * 4;
                    acc[t4] += wt * img[s4];
                    acc[t4 + 1] += wt * img[s4 + 1];
                    acc[t4 + 2] += wt * img[s4 + 2];
                    acc[t4 + 3] += wt * img[s4 + 3];
                    ws[t] += wt;
                }
            }
        }
        for (let i = 0, n = w * L.h; i < n; i++) {
            if (!hole[i] || ws[i] <= 0) continue;
            const o = i * 4, f = 1 / ws[i];
            img[o] = acc[o] * f; img[o + 1] = acc[o + 1] * f; img[o + 2] = acc[o + 2] * f; img[o + 3] = acc[o + 3] * f;
        }
    }

    // Hole pixels of the finer level start from the bilinear upsample of the coarser result.
    function upsampleHole(L, C) {
        const w = L.w, h = L.h, cw = C.w, ch = C.h, s = C.img, d = L.img;
        for (let y = 0; y < h; y++) {
            const fy = Math.max(0, Math.min(ch - 1, (y + 0.5) / 2 - 0.5));
            const y0 = Math.floor(fy), y1 = Math.min(ch - 1, y0 + 1), ty = fy - y0;
            for (let x = 0; x < w; x++) {
                const i = y * w + x;
                if (!L.hole[i]) continue;
                const fx = Math.max(0, Math.min(cw - 1, (x + 0.5) / 2 - 0.5));
                const x0 = Math.floor(fx), x1 = Math.min(cw - 1, x0 + 1), tx = fx - x0;
                const i00 = (y0 * cw + x0) * 4, i10 = (y0 * cw + x1) * 4, i01 = (y1 * cw + x0) * 4, i11 = (y1 * cw + x1) * 4;
                for (let c = 0; c < 4; c++) {
                    d[i * 4 + c] = (s[i00 + c] * (1 - tx) + s[i10 + c] * tx) * (1 - ty) + (s[i01 + c] * (1 - tx) + s[i11 + c] * tx) * ty;
                }
            }
        }
    }

    // The coarse NNF doubled; targets without a usable coarse match get a random source.
    function upsampleNNF(L, C, rand) {
        const w = L.w, h = L.h, t = L.targets, list = L.validList;
        for (let k = 0; k < t.length; k++) {
            const p = t[k], py = (p / w) | 0, px = p - py * w;
            const pc = (py >> 1) * C.w + (px >> 1);
            let qx = -1, qy = -1;
            if (C.nd && C.nd[pc] !== Infinity) {
                qx = C.nx[pc] * 2 + (px & 1);
                qy = C.ny[pc] * 2 + (py & 1);
            }
            if (qx < L.pr || qx >= w - L.pr || qy < L.pr || qy >= h - L.pr || !L.valid[qy * w + qx]) {
                const q = list[(rand() * list.length) | 0];
                qy = (q / w) | 0;
                qx = q - qy * w;
            }
            L.nx[p] = qx;
            L.ny[p] = qy;
        }
    }

    function iterationsFor(index, count, base) {
        if (index === count - 1 && count > 1) return base + 2;
        if (index === 0) return Math.max(2, base - 2);
        return base;
    }

    // Fills the hole pixels of work.img (floats) in place. Returns statistics.
    function solve(work, patch, base, rand, hooks) {
        const levels = [makeLevel(work.w, work.h, work.img, work.hole)];
        const useAlpha = work.useAlpha;
        for (;;) {
            const top = levels[levels.length - 1];
            if (Math.max(top.w, top.h) <= COARSEST_EDGE || Math.min(top.w, top.h) <= patch * 2 + 2 || levels.length >= 10) break;
            levels.push(downLevel(top));
        }
        levels.forEach((L, i) => prepareLevel(L, i === 0 && L.w * L.h > 1000000 ? Math.min(patch, 5) : patch));
        while (levels.length > 1) {
            const c = levels[levels.length - 1];
            if (c.validList && c.validList.length >= MIN_SOURCES && c.targets.length > 0) break;
            levels.pop();
        }
        const count = levels.length;
        const fine = levels[0];
        const stats = { levels: count, iterations: 0, targets: fine.targets ? fine.targets.length : 0, sources: fine.validList ? fine.validList.length : 0 };
        const coarsest = levels[count - 1];
        diffuseFill(coarsest);
        if (!fine.validList || !fine.validList.length || !fine.targets.length) {
            // Nothing to copy from (or the hole sits in the border band): the flat diffusion has to do.
            for (let li = count - 2; li >= 0; li--) { diffuseFill(levels[li]); }
            stats.fallback = true;
            return stats;
        }
        let totalWork = 0, doneWork = 0;
        levels.forEach((L, i) => { if (L.targets && L.targets.length && L.validList.length) totalWork += L.targets.length * (iterationsFor(i, count, base) + 1); });
        const tick = amount => {
            doneWork += amount;
            if (hooks.progress) hooks.progress(Math.min(1, doneWork / Math.max(1, totalWork)));
            if (hooks.cancelled && hooks.cancelled()) throw abortError();
        };
        for (let li = count - 1; li >= 0; li--) {
            const L = levels[li];
            if (!L.targets || !L.targets.length || !L.validList.length) {
                if (li < count - 1) upsampleHole(L, levels[li + 1]);
                continue;
            }
            const kernel = patchKernel(L.pr);
            if (li === count - 1) {
                randomNNF(L, rand);
            } else {
                upsampleHole(L, levels[li + 1]);
                upsampleNNF(L, levels[li + 1], rand);
                vote(L, useAlpha, false, kernel);
                levels[li + 1] = null;
            }
            tick(L.targets.length);
            const rmax = li === 0 ? Math.min(Math.max(L.w, L.h), 24) : Math.min(Math.max(L.w, L.h), 64);
            const rounds = iterationsFor(li, count, base);
            for (let it = 0; it < rounds; it++) {
                computeDist(L, useAlpha);
                pmPass(L, useAlpha, (it & 1) === 1, rand, rmax);
                vote(L, useAlpha, true, kernel);
                stats.iterations++;
                tick(L.targets.length);
            }
            // The coarser level is no longer needed once its NNF has been handed down.
            L.acc = null; L.ws = null;
        }
        return stats;
    }

    // Premultiplied floats back to 8-bit RGBA, written for the hole pixels only.
    function writeHole(floats, hole, w, h, target, region, targetWidth) {
        const out = target.data;
        for (let y = 0; y < h; y++) {
            for (let x = 0; x < w; x++) {
                const i = y * w + x;
                if (!hole[i]) continue;
                const o = i * 4;
                let a = floats[o + 3];
                a = a < 0 ? 0 : (a > 255 ? 255 : a);
                const q = ((region.y + y) * targetWidth + region.x + x) * 4;
                if (a < 0.5) {
                    out[q] = out[q + 1] = out[q + 2] = out[q + 3] = 0;
                    continue;
                }
                const f = 255 / a;
                out[q] = floats[o] * f;
                out[q + 1] = floats[o + 1] * f;
                out[q + 2] = floats[o + 2] * f;
                out[q + 3] = a;
            }
        }
    }

    // image: { width, height, data }, mask: Uint8ClampedArray(width * height); pixels with mask > threshold are replaced.
    // options: patch (3..15, default 7), iterations (base count per level, default 4), margin ('auto' | 'all' | px),
    // seed, threshold (default 127), maxPixels (work area cap, default 2,000,000) and maxHole (hole size cap, default 250,000): bigger jobs are
    // solved smaller and scaled back, inPlace (write into image instead of a copy). hooks: { progress(fraction), cancelled() }.
    // Returns { image, region, stats }; image is the input size, only masked pixels differ. Throws Error('no-sources')
    // when there is nothing around the hole to copy from.
    function inpaint(image, mask, options, hooks) {
        const o = options || {}, hk = hooks || {};
        const W = image.width, H = image.height;
        if (!mask || mask.length !== W * H) throw new Error('inpaint: the mask must have one value per pixel');
        const threshold = o.threshold == null ? 127 : o.threshold;
        const out = o.inPlace ? image : { width: W, height: H, data: new Uint8ClampedArray(image.data) };
        const bounds = Lib.alphaBounds(mask, W, H, threshold);
        if (!bounds) return { image: out, region: null, stats: { levels: 0, iterations: 0, targets: 0, sources: 0 } };
        const patch = intOption(o.patch, 3, 15, 7);
        const base = intOption(o.iterations, 1, 20, 4);
        const maxPixels = intOption(o.maxPixels, 10000, 64000000, 2000000);
        const maxHole = intOption(o.maxHole, 1000, 64000000, 250000);
        const rand = Lib.mulberry32(o.seed == null ? 1 : o.seed);
        const region = inpaintRegion(bounds, W, H, { patch, margin: o.margin });
        const rw = region.w, rh = region.h;
        const crop = new Uint8ClampedArray(rw * rh * 4), cropMask = new Uint8ClampedArray(rw * rh);
        for (let y = 0; y < rh; y++) {
            const si = (region.y + y) * W + region.x;
            crop.set(image.data.subarray(si * 4, (si + rw) * 4), y * rw * 4);
            cropMask.set(mask.subarray(si, si + rw), y * rw);
        }
        let holeCount = 0;
        for (let i = 0; i < cropMask.length; i++) if (cropMask[i] > threshold) holeCount++;
        // Work area and hole are capped: a bigger job is solved at a smaller scale and scaled back up.
        const scale = Math.min(1, Math.sqrt(maxPixels / (rw * rh)), Math.sqrt(maxHole / Math.max(1, holeCount)));
        let stats;
        if (scale >= 1) {
            const work = buildWork(crop, cropMask, rw, rh, threshold);
            if (!work.known) throw new Error('no-sources');
            stats = solve(work, patch, base, rand, hk);
            writeHole(work.img, work.hole, rw, rh, out, region, W);
        } else {
            const sw = Math.max(8, Math.round(rw * scale)), sh = Math.max(8, Math.round(rh * scale));
            const small = Lib.resizeBilinear({ width: rw, height: rh, data: crop }, sw, sh);
            const smallMask = new Uint8ClampedArray(sw * sh);
            for (let y = 0; y < rh; y++) {
                const sy = Math.min(sh - 1, Math.floor(y * sh / rh));
                for (let x = 0; x < rw; x++) {
                    if (cropMask[y * rw + x] > threshold) smallMask[sy * sw + Math.min(sw - 1, Math.floor(x * sw / rw))] = 255;
                }
            }
            const work = buildWork(small.data, smallMask, sw, sh, 127);
            if (!work.known) throw new Error('no-sources');
            stats = solve(work, patch, base, rand, hk);
            const filled = { width: sw, height: sh, data: new Uint8ClampedArray(small.data) };
            writeHole(work.img, work.hole, sw, sh, filled, { x: 0, y: 0 }, sw);
            const up = Lib.resizeBilinear(filled, rw, rh);
            const holeFull = new Uint8Array(rw * rh);
            for (let i = 0; i < holeFull.length; i++) if (cropMask[i] > threshold) holeFull[i] = 1;
            const upFloat = new Float32Array(rw * rh * 4);
            for (let i = 0; i < rw * rh; i++) {
                const q = i * 4, f = up.data[q + 3] / 255;
                upFloat[q] = up.data[q] * f; upFloat[q + 1] = up.data[q + 1] * f; upFloat[q + 2] = up.data[q + 2] * f; upFloat[q + 3] = up.data[q + 3];
            }
            writeHole(upFloat, holeFull, rw, rh, out, region, W);
            stats.scale = scale;
        }
        if (hk.progress) hk.progress(1);
        return { image: out, region, stats };
    }

    Lib.registerOp('inpaint.fill', (args, ctx) => {
        const res = inpaint(args.image, args.mask, {
            patch: args.patch, iterations: args.iterations, margin: args.margin, seed: args.seed,
            threshold: args.threshold, maxPixels: args.maxPixels, maxHole: args.maxHole, inPlace: !!args.inPlace
        }, ctx);
        return { image: res.image, region: res.region, stats: res.stats };
    });

    // --- colour: Lab, palette extraction, harmonies ---

    const SRGB_LINEAR = new Float32Array(256);
    for (let i = 0; i < 256; i++) {
        const v = i / 255;
        SRGB_LINEAR[i] = v <= 0.04045 ? v / 12.92 : Math.pow((v + 0.055) / 1.055, 2.4);
    }

    function labF(t) {
        return t > 0.008856 ? Math.cbrt(t) : 7.787 * t + 16 / 116;
    }

    // sRGB (0..255) -> CIE Lab, D65. Returns [L, a, b].
    function rgbToLab(r, g, b, out) {
        const lr = SRGB_LINEAR[Math.max(0, Math.min(255, Math.round(r)))];
        const lg = SRGB_LINEAR[Math.max(0, Math.min(255, Math.round(g)))];
        const lb = SRGB_LINEAR[Math.max(0, Math.min(255, Math.round(b)))];
        const fx = labF((0.4124564 * lr + 0.3575761 * lg + 0.1804375 * lb) / 0.95047);
        const fy = labF(0.2126729 * lr + 0.7151522 * lg + 0.0721750 * lb);
        const fz = labF((0.0193339 * lr + 0.1191920 * lg + 0.9503041 * lb) / 1.08883);
        const res = out || [0, 0, 0];
        res[0] = 116 * fy - 16;
        res[1] = 500 * (fx - fy);
        res[2] = 200 * (fy - fz);
        return res;
    }

    function labInv(t) {
        const c = t * t * t;
        return c > 0.008856 ? c : (t - 16 / 116) / 7.787;
    }

    function linearToSrgb(v) {
        const c = v <= 0.0031308 ? v * 12.92 : 1.055 * Math.pow(v, 1 / 2.4) - 0.055;
        return Math.max(0, Math.min(255, c * 255));
    }

    // CIE Lab -> sRGB floats (0..255, clamped).
    function labToRgb(L, a, b, out) {
        const fy = (L + 16) / 116, fx = fy + a / 500, fz = fy - b / 200;
        const x = labInv(fx) * 0.95047, y = labInv(fy), z = labInv(fz) * 1.08883;
        const res = out || [0, 0, 0];
        res[0] = linearToSrgb(3.2404542 * x - 1.5371385 * y - 0.4985314 * z);
        res[1] = linearToSrgb(-0.9692660 * x + 1.8760108 * y + 0.0415560 * z);
        res[2] = linearToSrgb(0.0556434 * x - 0.2040259 * y + 1.0572252 * z);
        return res;
    }

    // Dominant colours: k-means in Lab on a pixel sample, k-means++ start, sorted by population.
    // options: seed, maxSamples (default 20000), minAlpha (default 128), iterations (default 16), merge (Lab distance below
    // which two result colours count as one, default 3). Returns [{ r, g, b, hex, count, share }]; fewer than `count`
    // entries when the image has fewer distinct colours.
    function extractPalette(image, count, options) {
        const o = options || {};
        const k0 = intOption(count, 1, 32, 8);
        const total = image.width * image.height, d = image.data;
        const minAlpha = o.minAlpha == null ? 128 : o.minAlpha;
        const stride = Math.max(1, Math.ceil(total / intOption(o.maxSamples, 100, 200000, 20000)));
        const lab = [], rgb = [];
        const tmp = [0, 0, 0];
        for (let i = 0; i < total; i += stride) {
            const q = i * 4;
            if (d[q + 3] < minAlpha) continue;
            rgbToLab(d[q], d[q + 1], d[q + 2], tmp);
            lab.push(tmp[0], tmp[1], tmp[2]);
            rgb.push(d[q], d[q + 1], d[q + 2]);
        }
        const n = lab.length / 3;
        if (!n) return [];
        const rand = Lib.mulberry32(o.seed == null ? 7 : o.seed);
        const k = Math.min(k0, n);
        const centres = new Float64Array(k * 3);
        const near = new Float64Array(n).fill(Infinity);
        const distTo = (i, c) => {
            const dl = lab[i * 3] - centres[c * 3], da = lab[i * 3 + 1] - centres[c * 3 + 1], db = lab[i * 3 + 2] - centres[c * 3 + 2];
            return dl * dl + da * da + db * db;
        };
        let first = Math.floor(rand() * n);
        for (let c = 0; c < k; c++) {
            if (c > 0) {
                let sum = 0;
                for (let i = 0; i < n; i++) sum += near[i];
                if (sum <= 0) { first = Math.floor(rand() * n); } else {
                    let pick = rand() * sum;
                    first = n - 1;
                    for (let i = 0; i < n; i++) { pick -= near[i]; if (pick <= 0) { first = i; break; } }
                }
            }
            centres[c * 3] = lab[first * 3]; centres[c * 3 + 1] = lab[first * 3 + 1]; centres[c * 3 + 2] = lab[first * 3 + 2];
            for (let i = 0; i < n; i++) { const dd = distTo(i, c); if (dd < near[i]) near[i] = dd; }
        }
        const label = new Int32Array(n).fill(-1);
        const sums = new Float64Array(k * 3), counts = new Float64Array(k);
        const maxIter = intOption(o.iterations, 1, 64, 16);
        for (let iter = 0; iter < maxIter; iter++) {
            let changed = 0;
            sums.fill(0); counts.fill(0);
            for (let i = 0; i < n; i++) {
                let best = 0, bd = Infinity;
                for (let c = 0; c < k; c++) { const dd = distTo(i, c); if (dd < bd) { bd = dd; best = c; } }
                if (label[i] !== best) { label[i] = best; changed++; }
                sums[best * 3] += lab[i * 3]; sums[best * 3 + 1] += lab[i * 3 + 1]; sums[best * 3 + 2] += lab[i * 3 + 2];
                counts[best]++;
            }
            for (let c = 0; c < k; c++) {
                if (counts[c] > 0) {
                    centres[c * 3] = sums[c * 3] / counts[c]; centres[c * 3 + 1] = sums[c * 3 + 1] / counts[c]; centres[c * 3 + 2] = sums[c * 3 + 2] / counts[c];
                }
            }
            if (!changed) break;
        }
        const mean = [];
        for (let c = 0; c < k; c++) mean.push({ r: 0, g: 0, b: 0, count: 0, lab: [centres[c * 3], centres[c * 3 + 1], centres[c * 3 + 2]] });
        for (let i = 0; i < n; i++) {
            const m = mean[label[i]];
            m.r += rgb[i * 3]; m.g += rgb[i * 3 + 1]; m.b += rgb[i * 3 + 2]; m.count++;
        }
        let list = mean.filter(m => m.count > 0);
        const mergeBelow = o.merge == null ? 3 : o.merge;
        list.sort((a, b) => b.count - a.count);
        const merged = [];
        list.forEach(m => {
            const twin = merged.find(x => Math.hypot(x.lab[0] - m.lab[0], x.lab[1] - m.lab[1], x.lab[2] - m.lab[2]) < mergeBelow);
            if (twin) { twin.r += m.r; twin.g += m.g; twin.b += m.b; twin.count += m.count; } else merged.push(m);
        });
        list = merged.sort((a, b) => b.count - a.count);
        return list.map(m => {
            const r = Math.round(m.r / m.count), g = Math.round(m.g / m.count), b = Math.round(m.b / m.count);
            return { r, g, b, hex: Lib.toHex(r, g, b), count: m.count, share: m.count / n };
        });
    }

    // Harmonies of a colour as hex lists that start with the colour itself:
    // [{ id: 'complementary' | 'analogous' | 'triadic' | 'split-complementary' | 'tetradic' | 'monochrome', colors: [...] }].
    function colorHarmonies(hex) {
        const rgba = Lib.parseColor(hex);
        if (!rgba) return [];
        const hsl = Lib.rgbToHsl(rgba[0], rgba[1], rgba[2]);
        const h = hsl[0], s = hsl[1], l = hsl[2];
        const at = (dh, dl) => {
            const rgb = Lib.hslToRgb(h + dh, s, Math.max(0, Math.min(1, l + (dl || 0))));
            return Lib.toHex(rgb[0], rgb[1], rgb[2]);
        };
        const base = Lib.toHex(rgba[0], rgba[1], rgba[2]);
        return [
            { id: 'complementary', colors: [base, at(180)] },
            { id: 'analogous', colors: [at(-30), base, at(30)] },
            { id: 'triadic', colors: [base, at(120), at(240)] },
            { id: 'split-complementary', colors: [base, at(150), at(210)] },
            { id: 'tetradic', colors: [base, at(60), at(180), at(240)] },
            { id: 'monochrome', colors: [at(0, -0.3), at(0, -0.15), base, at(0, 0.15), at(0, 0.3)] }
        ];
    }

    // --- content bounds (trim transparent pixels, auto-crop) ---

    // Background of an image from its corner pixels ([r, g, b, a] each): { transparent: true } when most corners are
    // transparent, else { transparent: false, color: [r, g, b] } of the corner colour that most corners agree with.
    function estimateBackground(corners, tolerance) {
        const tol = tolerance == null ? 12 : tolerance;
        const list = (corners || []).filter(Boolean);
        if (!list.length) return { transparent: true, color: null };
        const clear = list.filter(c => c[3] < 16).length;
        if (clear * 2 >= list.length) return { transparent: true, color: null };
        const solid = list.filter(c => c[3] >= 16);
        let best = solid[0], bestVotes = -1;
        solid.forEach(c => {
            const votes = solid.filter(x => Math.max(Math.abs(x[0] - c[0]), Math.abs(x[1] - c[1]), Math.abs(x[2] - c[2])) <= tol).length;
            if (votes > bestVotes) { best = c; bestVotes = votes; }
        });
        return { transparent: false, color: [best[0], best[1], best[2]] };
    }

    // Bounding box { x, y, w, h } of the pixels that count as content, or null. options: mode 'alpha' (default: alpha >
    // threshold) or 'color' (also differs from options.background = [r, g, b] by more than `tolerance` in some channel;
    // without a background colour only alpha counts), threshold (default 0), tolerance (default 12).
    function contentBounds(image, options) {
        const o = options || {};
        const w = image.width, h = image.height, d = image.data;
        const threshold = o.threshold == null ? 0 : o.threshold;
        const useColor = o.mode === 'color' && Array.isArray(o.background);
        const tol = o.tolerance == null ? 12 : o.tolerance;
        const bg = o.background;
        let minX = w, minY = h, maxX = -1, maxY = -1;
        for (let y = 0; y < h; y++) {
            let q = y * w * 4;
            for (let x = 0; x < w; x++, q += 4) {
                if (d[q + 3] <= threshold) continue;
                if (useColor && Math.abs(d[q] - bg[0]) <= tol && Math.abs(d[q + 1] - bg[1]) <= tol && Math.abs(d[q + 2] - bg[2]) <= tol) continue;
                if (x < minX) minX = x;
                if (x > maxX) maxX = x;
                if (y < minY) minY = y;
                maxY = y;
            }
        }
        return maxX < 0 ? null : { x: minX, y: minY, w: maxX - minX + 1, h: maxY - minY + 1 };
    }

    Object.assign(Lib, {
        inpaint,
        inpaintRegion,
        rgbToLab,
        labToRgb,
        extractPalette,
        colorHarmonies,
        estimateBackground,
        contentBounds
    });
})(typeof self !== 'undefined' ? self : (typeof globalThis !== 'undefined' ? globalThis : this));
