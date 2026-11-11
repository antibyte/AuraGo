// Pixel pure image library, "art" part 2: the wet layer (watercolour, ink). DOM-free.
// A coarse grid holds water and pigment (amount m plus m * the 8 pigment components). Water spreads and wicks into
// dry paper, pigment follows it, rims dry first and pull pigment outward (dark edges), paper tooth granulates the result.
// Needs pixel-lib-core.js and pixel-lib-paint.js first (the Pixel worker and the test loader order them that way).
(function (root) {
    'use strict';

    const Lib = root.AuraPixelLib = root.AuraPixelLib || {};
    const paint = Lib.paint;
    if (!paint || typeof paint.registerEngine !== 'function' || typeof paint.pigmentOf !== 'function') {
        throw new Error('pixel-lib-wet.js needs pixel-lib-paint.js to be loaded first');
    }
    const DW = 0.14, DP = 0.05, ADV = 0.9, EVAP = 0.006, EDGE = 2.5, ABS = 0.006, GRAN = 0.6;
    const WET = 0.02; // a cell is wet above this much water
    const NP = 8;     // pigment components per cell: q0..q4, e0..e2 (m, the amount, lives in its own array)

    function clamp(v, lo, hi) { return v < lo ? lo : (v > hi ? hi : v); }
    function smoothstep(a, b, x) {
        if (x <= a) return 0;
        if (x >= b) return 1;
        const t = (x - a) / (b - a);
        return t * t * (3 - 2 * t);
    }

    const sPig = new Float32Array(8), sGl = new Float32Array(8), sSamp = new Float32Array(NP);
    const sRgb = [0, 0, 0], WHITE = paint.pigmentOf(255, 255, 255);

    // Same maths as paint.pigmentToRgb, inlined for the render loop: it runs per pixel, and a sqrt through a local
    // binding is much cheaper than the global Math lookup inside vm sandboxes (test and bench harnesses).
    const sqrt = Math.sqrt, lin2srgb = paint.linearToSrgb, TM = paint.PIGMENT.T;
    const T00 = TM[0][0], T01 = TM[0][1], T02 = TM[0][2], T03 = TM[0][3], T04 = TM[0][4];
    const T10 = TM[1][0], T11 = TM[1][1], T12 = TM[1][2], T13 = TM[1][3], T14 = TM[1][4];
    const T20 = TM[2][0], T21 = TM[2][1], T22 = TM[2][2], T23 = TM[2][3], T24 = TM[2][4];
    function ksInv(k) {
        if (k < 0) k = 0;
        return 1 + k - sqrt(k * k + 2 * k);
    }
    function toRgb(p, out) {
        const R0 = ksInv(p[0]), R1 = ksInv(p[1]), R2 = ksInv(p[2]), R3 = ksInv(p[3]), R4 = ksInv(p[4]);
        out[0] = lin2srgb(T00 * R0 + T01 * R1 + T02 * R2 + T03 * R3 + T04 * R4 + p[5]);
        out[1] = lin2srgb(T10 * R0 + T11 * R1 + T12 * R2 + T13 * R3 + T14 * R4 + p[6]);
        out[2] = lin2srgb(T20 * R0 + T21 * R1 + T22 * R2 + T23 * R3 + T24 * R4 + p[7]);
    }

    function createWet(opts) {
        const o = opts || {};
        const docW = Math.max(1, o.docW | 0), docH = Math.max(1, o.docH | 0), maxCells = o.maxCells > 0 ? o.maxCells : 600000;
        const paper = o.paper || null, paperScale = o.paperScale > 0 ? o.paperScale : 1;
        const absorb = paper ? paper.absorb : 0.5, tooth = paper ? paper.tooth : 0.5, WICK = 0.04 + 0.22 * absorb;
        let s = 1;
        while (Math.ceil(docW / s) * Math.ceil(docH / s) > maxCells) s++;
        const gw = Math.ceil(docW / s), gh = Math.ceil(docH / s), n = gw * gh;
        // Interleaved pigment fields (cell i owns pg[i*8 .. i*8+7]) keep the inner loops on one cache line.
        const water = new Float32Array(n), dWater = new Float32Array(n);
        const m = new Float32Array(n), dM = new Float32Array(n);
        const pg = new Float32Array(n * NP), dpg = new Float32Array(n * NP);
        let bx0 = 0, by0 = 0, bx1 = -1, by1 = -1, active = false;      // paint box in cells (inclusive): grows only
        let sx0 = 0, sy0 = 0, sx1 = -1, sy1 = -1, simOn = false;       // simulation box: cells that hold water, padded; shrinks
        let dx0 = 0, dy0 = 0, dx1 = -1, dy1 = -1, dirty = false;       // changed since the last render
        let wetFlag = false, paintFlag = false;

        function markDirty(x0, y0, x1, y1) {
            if (!dirty) { dx0 = x0; dy0 = y0; dx1 = x1; dy1 = y1; dirty = true; return; }
            if (x0 < dx0) dx0 = x0;
            if (y0 < dy0) dy0 = y0;
            if (x1 > dx1) dx1 = x1;
            if (y1 > dy1) dy1 = y1;
        }

        function deposit(x, y, radius, waterAmt, amount, pigment) {
            const r = Math.max(radius, 0.8 * s);
            const cx0 = Math.max(0, Math.floor((x - r) / s) - 1), cx1 = Math.min(gw - 1, Math.ceil((x + r) / s) + 1);
            const cy0 = Math.max(0, Math.floor((y - r) / s) - 1), cy1 = Math.min(gh - 1, Math.ceil((y + r) / s) + 1);
            if (cx1 < cx0 || cy1 < cy0) return;
            for (let cy = cy0; cy <= cy1; cy++) {
                const ddy = (cy + 0.5) * s - y;
                for (let cx = cx0; cx <= cx1; cx++) {
                    const ddx = (cx + 0.5) * s - x, q = Math.sqrt(ddx * ddx + ddy * ddy) / r;
                    if (q >= 1) continue;
                    const f = 1 - smoothstep(0.35, 1, q), i = cy * gw + cx;
                    const w = water[i] + waterAmt * f;
                    water[i] = w > 2.5 ? 2.5 : w;
                    if (water[i] > WET) wetFlag = true;
                    const dm = amount * f;
                    if (dm > 0) {
                        m[i] += dm;
                        const a = i * NP;
                        for (let k = 0; k < NP; k++) pg[a + k] += dm * pigment[k];
                        paintFlag = true;
                    }
                }
            }
            const px0 = Math.max(0, cx0 - 2), py0 = Math.max(0, cy0 - 2), px1 = Math.min(gw - 1, cx1 + 2), py1 = Math.min(gh - 1, cy1 + 2);
            if (!active) { bx0 = px0; by0 = py0; bx1 = px1; by1 = py1; active = true; }
            else { bx0 = Math.min(bx0, px0); by0 = Math.min(by0, py0); bx1 = Math.max(bx1, px1); by1 = Math.max(by1, py1); }
            if (!simOn) { sx0 = px0; sy0 = py0; sx1 = px1; sy1 = py1; simOn = true; }
            else { sx0 = Math.min(sx0, px0); sy0 = Math.min(sy0, py0); sx1 = Math.max(sx1, px1); sy1 = Math.max(sy1, py1); }
            markDirty(cx0, cy0, cx1, cy1);
        }

        function step() {
            if (!simOn) { wetFlag = false; return false; }
            // water that reached the box border would leak out of it: grow the box first
            let leak = false;
            for (let x = sx0; x <= sx1 && !leak; x++) leak = water[sy0 * gw + x] > 1e-4 || water[sy1 * gw + x] > 1e-4;
            for (let y = sy0; y <= sy1 && !leak; y++) leak = water[y * gw + sx0] > 1e-4 || water[y * gw + sx1] > 1e-4;
            if (leak) {
                sx0 = Math.max(0, sx0 - 2); sy0 = Math.max(0, sy0 - 2); sx1 = Math.min(gw - 1, sx1 + 2); sy1 = Math.min(gh - 1, sy1 + 2);
                bx0 = Math.min(bx0, sx0); by0 = Math.min(by0, sy0); bx1 = Math.max(bx1, sx1); by1 = Math.max(by1, sy1); // paint follows the water
            }
            for (let y = sy0; y <= sy1; y++) {
                const r0 = y * gw + sx0, r1 = y * gw + sx1 + 1;
                dWater.fill(0, r0, r1); dM.fill(0, r0, r1); dpg.fill(0, r0 * NP, r1 * NP);
            }
            // Edges between neighbouring cells (right, down): water flows downhill, pigment rides along, wet cells
            // diffuse. An edge with no water on either side moves nothing, so dry paper costs next to nothing.
            for (let y = sy0; y <= sy1; y++) {
                for (let x = sx0; x <= sx1; x++) {
                    const i = y * gw + x, wi = water[i];
                    for (let d = 0; d < 2; d++) {
                        let j;
                        if (d === 0) { if (x >= sx1) continue; j = i + 1; } else { if (y >= sy1) continue; j = i + gw; }
                        const wj = water[j];
                        if (wi === 0 && wj === 0) continue;
                        const mi = m[i], mj = m[j];
                        if (wi > WET && wj > WET && (mi !== 0 || mj !== 0)) {
                            let c = wi < wj ? wi : wj;
                            if (c > 1) c = 1;
                            c *= DP;
                            const xm = c * (mi - mj);
                            dM[i] -= xm; dM[j] += xm;
                            const a = i * NP, b = j * NP;
                            for (let k = 0; k < NP; k++) {
                                const xk = c * (pg[a + k] - pg[b + k]);
                                dpg[a + k] -= xk; dpg[b + k] += xk;
                            }
                        }
                        if (wi === wj) continue;
                        const src = wi > wj ? i : j, dst = wi > wj ? j : i, ws = wi > wj ? wi : wj;
                        const fl = DW * (wi > wj ? wi - wj : wj - wi) * (water[dst] > WET ? 1 : WICK);
                        dWater[src] -= fl; dWater[dst] += fl;
                        const ms = m[src];
                        if (ms > 0) {
                            let frac = fl / (ws > 0.05 ? ws : 0.05);
                            if (frac > 0.22) frac = 0.22;
                            frac *= ADV;
                            const mv = ms * frac;
                            dM[src] -= mv; dM[dst] += mv;
                            const a = src * NP, b = dst * NP;
                            for (let k = 0; k < NP; k++) {
                                const v = pg[a + k] * frac;
                                dpg[a + k] -= v; dpg[b + k] += v;
                            }
                        }
                    }
                }
            }
            let anyWet = false, anyWater = false;
            let nx0 = gw, ny0 = gh, nx1 = -1, ny1 = -1; // cells that had water before or after this step
            for (let y = sy0; y <= sy1; y++) {
                for (let x = sx0; x <= sx1; x++) {
                    const i = y * gw + x, w = water[i], dw = dWater[i];
                    if (w === 0 && dw === 0) continue; // dry and untouched: nothing changes
                    let nw = w + dw;
                    if (w > 0) {
                        const dry = (x > 0 && water[i - 1] > WET ? 0 : 1) + (x < gw - 1 && water[i + 1] > WET ? 0 : 1) +
                            (y > 0 && water[i - gw] > WET ? 0 : 1) + (y < gh - 1 && water[i + gw] > WET ? 0 : 1);
                        nw -= EVAP * (1 + EDGE * dry / 4) + ABS * absorb; // dry rims lose water faster and pull pigment outward
                    }
                    if (nw < 0) nw = 0;
                    water[i] = nw;
                    if (w > 0 || nw > 0) {
                        if (x < nx0) nx0 = x;
                        if (x > nx1) nx1 = x;
                        if (y < ny0) ny0 = y;
                        if (y > ny1) ny1 = y;
                    }
                    if (nw > 0) anyWater = true;
                    if (nw > WET) anyWet = true;
                    m[i] += dM[i];
                    const a = i * NP;
                    for (let k = 0; k < NP; k++) pg[a + k] += dpg[a + k];
                }
            }
            if (!anyWet && anyWater) { // leftovers below the wet threshold are gone
                for (let y = sy0; y <= sy1; y++) water.fill(0, y * gw + sx0, y * gw + sx1 + 1);
            }
            // only the wet area changed, so the area to redraw shrinks while the paint dries
            if (nx1 >= nx0) markDirty(Math.max(0, nx0 - 1), Math.max(0, ny0 - 1), Math.min(gw - 1, nx1 + 1), Math.min(gh - 1, ny1 + 1));
            // the simulation box follows the water: later small strokes must not pay for an earlier big wash
            if (nx1 >= nx0) { sx0 = Math.max(0, nx0 - 2); sy0 = Math.max(0, ny0 - 2); sx1 = Math.min(gw - 1, nx1 + 2); sy1 = Math.min(gh - 1, ny1 + 2); }
            else simOn = false;
            wetFlag = anyWet;
            return anyWet;
        }

        function cellsToRect(x0, y0, x1, y1) {
            const rx0 = Math.max(0, (x0 - 1) * s), ry0 = Math.max(0, (y0 - 1) * s);
            const rx1 = Math.min(docW, (x1 + 2) * s), ry1 = Math.min(docH, (y1 + 2) * s);
            return rx1 > rx0 && ry1 > ry0 ? { x: rx0, y: ry0, w: rx1 - rx0, h: ry1 - ry0 } : null;
        }
        function dirtyRect() { return dirty ? cellsToRect(dx0, dy0, dx1, dy1) : null; }
        function paintRect() { return paintFlag && active ? cellsToRect(bx0, by0, bx1, by1) : null; }

        // Paper height at a doc pixel, nearest texel (tiled 256); scale = doc px per texel like paint.paperHeight.
        const pScale = paperScale >= 0.25 ? paperScale : 0.25, pH = paper ? paper.height : null;

        function render(out, base) {
            const od = out.data, bd = base.data, w = out.w, h = out.h, lock = !!out.lockAlpha, sel = out.sel, dw = out.docW;
            const ox = out.x, oy = out.y;
            if (!active) { od.set(bd.subarray(0, w * h * 4)); dirty = false; return; }
            // per output column: the two source cells, the lerp weight and the paper column (once per call)
            const colX0 = new Int32Array(w), colX1 = new Int32Array(w), colT = new Float32Array(w), colU = new Int32Array(w);
            let cmin = 0, cmax = 0;
            if (s > 1) {
                cmin = gw - 1; cmax = 0;
                for (let i = 0; i < w; i++) {
                    const fx = (ox + i + 0.5) / s - 0.5, ix = Math.floor(fx);
                    const x0 = ix < 0 ? 0 : (ix > gw - 1 ? gw - 1 : ix), x1 = ix + 1 < 0 ? 0 : (ix + 1 > gw - 1 ? gw - 1 : ix + 1);
                    colX0[i] = x0; colX1[i] = x1; colT[i] = fx - ix;
                    if (x0 < cmin) cmin = x0;
                    if (x1 > cmax) cmax = x1;
                }
                for (let i = 0; i < w; i++) { colX0[i] -= cmin; colX1[i] -= cmin; }
            }
            if (pH) for (let i = 0; i < w; i++) colU[i] = Math.floor((ox + i) / pScale + 0.5) & 255;
            const cw = cmax - cmin + 1;
            const rbM = new Float32Array(s > 1 ? cw : 1), rbP = new Float32Array(s > 1 ? cw * NP : 1);
            const gk = GRAN * tooth * 2;
            let lastKey = 0, haveKey = false;
            for (let j = 0; j < h; j++) {
                const py = oy + j, rowK = j * w * 4;
                od.set(bd.subarray(rowK, rowK + w * 4), rowK); // untouched pixels stay the base
                let rowOff = 0, ty = 0;
                let ra = 0, rb = 0; // source cell rows (s > 1) or the cell row (s === 1)
                if (s === 1) {
                    if (py >= gh) continue;
                    ra = py * gw;
                } else {
                    const fy = (py + 0.5) / s - 0.5, iy = Math.floor(fy);
                    ty = fy - iy;
                    ra = (iy < 0 ? 0 : (iy > gh - 1 ? gh - 1 : iy)) * gw;
                    rb = (iy + 1 < 0 ? 0 : (iy + 1 > gh - 1 ? gh - 1 : iy + 1)) * gw;
                    // vertical interpolation once per row
                    const sy = 1 - ty;
                    let rowMax = 0;
                    for (let c = 0; c < cw; c++) {
                        const ca = ra + cmin + c, cb = rb + cmin + c;
                        const mv = m[ca] * sy + m[cb] * ty;
                        rbM[c] = mv;
                        if (mv > rowMax) rowMax = mv;
                    }
                    if (rowMax < 0.002) continue; // no pigment on this row: the base copy is the answer
                    for (let c = 0; c < cw; c++) {
                        const a = (ra + cmin + c) * NP, b = (rb + cmin + c) * NP, d = c * NP;
                        for (let k = 0; k < NP; k++) rbP[d + k] = pg[a + k] * sy + pg[b + k] * ty;
                    }
                }
                if (pH) rowOff = (Math.floor(py / pScale + 0.5) & 255) << 8;
                for (let i = 0; i < w; i++) {
                    const px = ox + i, k = rowK + i * 4;
                    let mm;
                    if (s === 1) {
                        if (px >= gw) continue;
                        const c = ra + px;
                        mm = m[c];
                        if (mm < 0.002) continue;
                        const a = c * NP;
                        for (let f = 0; f < NP; f++) sSamp[f] = pg[a + f];
                    } else {
                        const x0 = colX0[i], x1 = colX1[i], tx = colT[i], sx = 1 - tx;
                        mm = rbM[x0] * sx + rbM[x1] * tx;
                        if (mm < 0.002) continue;
                        const a = x0 * NP, b = x1 * NP;
                        for (let f = 0; f < NP; f++) sSamp[f] = rbP[a + f] * sx + rbP[b + f] * tx;
                    }
                    const ab = bd[k + 3];
                    if (lock && ab === 0) continue;
                    let selW = 1;
                    if (sel) { const sv = sel[py * dw + px]; if (sv === 0) continue; selW = sv / 255; }
                    // dens * ink = (mm * g) * (q / mm) = g * q for the absorption bands
                    let g = pH ? 1 + gk * (0.5 - pH[rowOff + colU[i]]) : 1;
                    if (g < 0) g = 0;
                    const dens = mm * g;
                    // substrate seen over white (runs of equal base pixels are common, so memoise the conversion)
                    const key = bd[k] | (bd[k + 1] << 8) | (bd[k + 2] << 16) | (ab << 24);
                    if (!haveKey || key !== lastKey) {
                        if (ab === 255) paint.pigmentOf(bd[k], bd[k + 1], bd[k + 2], sPig);
                        else if (ab === 0) sPig.set(WHITE);
                        else { paint.pigmentOf(bd[k], bd[k + 1], bd[k + 2], sGl); paint.pigmentMix(sPig, WHITE, sGl, ab / 255); }
                        lastKey = key; haveKey = true;
                    }
                    // glaze: absorption adds, the residual is added scaled by min(1, density)
                    for (let q = 0; q < 5; q++) sGl[q] = sPig[q] + g * sSamp[q];
                    const rs = (dens < 1 ? dens : 1) / mm;
                    sGl[5] = sPig[5] + sSamp[5] * rs; sGl[6] = sPig[6] + sSamp[6] * rs; sGl[7] = sPig[7] + sSamp[7] * rs;
                    toRgb(sGl, sRgb);
                    let na = ab, r = sRgb[0], gg = sRgb[1], b = sRgb[2];
                    if (!lock && ab < 255) {
                        let lo = r < gg ? r : gg;
                        if (b < lo) lo = b;
                        const cand = 255 * (1 - lo / 255);
                        if (cand > na) na = cand;
                    }
                    if (na < 255 && na > 0) { // un-composite from white
                        const a = na / 255, inv = 255 * (1 - a);
                        r = (r - inv) / a; gg = (gg - inv) / a; b = (b - inv) / a;
                        r = r < 0 ? 0 : (r > 255 ? 255 : r); gg = gg < 0 ? 0 : (gg > 255 ? 255 : gg); b = b < 0 ? 0 : (b > 255 ? 255 : b);
                    }
                    od[k] = bd[k] + (r - bd[k]) * selW; od[k + 1] = bd[k + 1] + (gg - bd[k + 1]) * selW; od[k + 2] = bd[k + 2] + (b - bd[k + 2]) * selW;
                    od[k + 3] = ab + (na - ab) * selW;
                }
            }
            dirty = false;
        }

        function bake() {
            water.fill(0); dWater.fill(0); m.fill(0); dM.fill(0); pg.fill(0); dpg.fill(0);
            active = false; simOn = false; wetFlag = false; paintFlag = false; dirty = false;
        }

        // q0..q4, e0..e2 as plain arrays (copies, for inspection and tests; the hot paths use the interleaved storage)
        function field(f) {
            const a = new Float32Array(n);
            for (let i = 0; i < n; i++) a[i] = pg[i * NP + f];
            return a;
        }

        return {
            s, gw, gh, docW, docH, water, m,
            get q0() { return field(0); }, get q1() { return field(1); }, get q2() { return field(2); }, get q3() { return field(3); },
            get q4() { return field(4); }, get e0() { return field(5); }, get e1() { return field(6); }, get e2() { return field(7); },
            deposit, step, render, bake, dirtyRect, paintRect,
            isWet() { return wetFlag; },
            hasPaint() { return paintFlag; }
        };
    }

    function wetEngine(medium, o) {
        const p = medium.p, colorPig = paint.pigmentOf(o.color[0], o.color[1], o.color[2]);
        let strokeLoad = 1, rgb = [o.color[0], o.color[1], o.color[2]];
        return {
            setColor(c) { paint.pigmentOf(c[0], c[1], c[2], colorPig); rgb = [c[0], c[1], c[2]]; },
            begin() { strokeLoad = 1; },
            reach(size) { return size / 2 * 1.1 + 2; },
            spacing(size) { return Math.max(1, size * 0.12); },
            // `target` is a wet layer (createWet), not a paint buffer
            dab(wet, x, y, size, pressure, angle, flow) {
                pressure = clamp(Number.isFinite(pressure) ? pressure : 1, 0, 1);
                flow = clamp(Number.isFinite(flow) ? flow : 1, 0, 1);
                const radius = size / 2 * (0.8 + 0.3 * pressure);
                wet.deposit(x, y, radius, p.water * o.wetness * (0.6 + 0.4 * pressure), p.pigment * o.load * flow * strokeLoad, colorPig);
                strokeLoad = Math.max(0.25, strokeLoad - p.deplete);
                return { x0: Math.floor(x - radius), y0: Math.floor(y - radius), x1: Math.ceil(x + radius) + 1, y1: Math.ceil(y + radius) + 1 };
            },
            carried() { return [rgb[0], rgb[1], rgb[2]]; }
        };
    }

    paint.registerEngine('wet', wetEngine);
    paint.createWet = createWet;
})(typeof self !== 'undefined' ? self : (typeof globalThis !== 'undefined' ? globalThis : this));
