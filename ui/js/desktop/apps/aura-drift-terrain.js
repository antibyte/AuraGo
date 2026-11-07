/* Aura Drift - 3D terrain: value noise, distance field, height rules, vertex colours, chunked mesh.
 *
 * Pure part (no THREE, runs in node): AD.world (noise helpers), AD.buildDistanceField, AD.terrainHeight,
 * AD.terrainPalette, AD.terrainColor. Presentation part: AD.createTerrain, AD.makeTerrainDetail.
 * Sim metres map to three x,z with y up. The corridor floor is below the road (-0.4), the hills
 * rise from hw+10 m outwards. AD.TERRAIN (look.js) holds the per-biome rules. */
(function () {
    'use strict';
    const AD = window.AuraDriftCore;
    const clamp = AD.clamp;
    /* Local aliases: global lookups are slow inside a vm context (the node tests). */
    const floor = Math.floor, sqrt = Math.sqrt, abs = Math.abs, min = Math.min, max = Math.max, pow = Math.pow, round = Math.round, hypot = Math.hypot;
    const lerp = (a, b, t) => a + (b - a) * t;
    const sstep = (a, b, x) => { const t = x <= a ? 0 : (x >= b ? 1 : (x - a) / (b - a)); return t * t * (3 - 2 * t); };

    /* ---- value noise (non tiling, 2D) ------------------------------------------- */
    const PERM = new Uint8Array(512), VAL = new Float32Array(256);
    (function initNoise() {
        const r = AD.rng(0x5eed1234);
        for (let i = 0; i < 256; i++) { VAL[i] = r(); PERM[i] = i; }
        for (let i = 255; i > 0; i--) { const j = floor(r() * (i + 1)); const t = PERM[i]; PERM[i] = PERM[j]; PERM[j] = t; }
        for (let i = 0; i < 256; i++) PERM[i + 256] = PERM[i];
    })();
    function noise2(x, y) {
        const xi = floor(x), yi = floor(y), xf = x - xi, yf = y - yi;
        const u = xf * xf * xf * (xf * (xf * 6 - 15) + 10), v = yf * yf * yf * (yf * (yf * 6 - 15) + 10);
        const X = xi & 255, Y = yi & 255;
        const a = VAL[PERM[PERM[X] + Y]], b = VAL[PERM[PERM[X + 1] + Y]];
        const c = VAL[PERM[PERM[X] + Y + 1]], d = VAL[PERM[PERM[X + 1] + Y + 1]];
        return a + (b - a) * u + (c - a) * v + (a - b - c + d) * u * v;
    }
    function fbm(x, y, oct) {
        let sum = 0, amp = 0.5, norm = 0;
        for (let o = 0; o < oct; o++) {
            sum += amp * noise2(x, y); norm += amp;
            const nx = (x * 0.8 - y * 0.6) * 2.03 + 17.3, ny = (x * 0.6 + y * 0.8) * 2.03 - 9.1;
            x = nx; y = ny; amp *= 0.5;
        }
        return sum / norm;
    }
    function ridged(x, y, oct) {
        let sum = 0, amp = 0.5, norm = 0;
        for (let o = 0; o < oct; o++) {
            const r = 1 - abs(noise2(x, y) * 2 - 1);
            sum += amp * r * r; norm += amp;
            const nx = (x * 0.8 - y * 0.6) * 2.1 + 5.7, ny = (x * 0.6 + y * 0.8) * 2.1 + 31.9;
            x = nx; y = ny; amp *= 0.5;
        }
        return sum / norm;
    }
    const srgbLin = v => v <= 0.04045 ? v / 12.92 : pow((v + 0.055) / 1.055, 2.4);
    function hexLin(hex) {
        const n = parseInt(String(hex).replace('#', ''), 16) || 0;
        return [srgbLin((n >> 16 & 255) / 255), srgbLin((n >> 8 & 255) / 255), srgbLin((n & 255) / 255)];
    }
    function hexRgb(hex) {
        const n = parseInt(String(hex).replace('#', ''), 16) || 0;
        return [n >> 16 & 255, n >> 8 & 255, n & 255];
    }

    /* Tileable 512 noise planes shared by every canvas texture (built once per page). */
    let TILE = null;
    function tileNoise() {
        if (TILE) return TILE;
        const N = 512, lo = new Float32Array(N * N), hi = new Float32Array(N * N);
        const LAT = new Float32Array(65536), r = AD.rng(0xC0FFEE);
        for (let i = 0; i < 65536; i++) LAT[i] = r();
        function octave(arr, f, amp, salt) {
            const inv = f / N;
            for (let j = 0; j < N; j++) {
                const y = j * inv, yi = floor(y), yf = y - yi, v = yf * yf * (3 - 2 * yf);
                const y0 = (yi % f) * 131 + salt, y1 = ((yi + 1) % f) * 131 + salt;
                for (let i = 0; i < N; i++) {
                    const x = i * inv, xi = floor(x), xf = x - xi, u = xf * xf * (3 - 2 * xf);
                    const x0 = (xi % f) * 257, x1 = ((xi + 1) % f) * 257;
                    const a = LAT[(x0 + y0) & 65535], b = LAT[(x1 + y0) & 65535], c = LAT[(x0 + y1) & 65535], d = LAT[(x1 + y1) & 65535];
                    const top = a + (b - a) * u;
                    arr[j * N + i] += amp * (top + (c + (d - c) * u - top) * v);
                }
            }
        }
        octave(lo, 4, 0.46, 11); octave(lo, 8, 0.28, 23); octave(lo, 16, 0.16, 37); octave(lo, 32, 0.1, 51);
        octave(hi, 96, 0.4, 67); octave(hi, 192, 0.3, 83);
        for (let i = 0; i < N * N; i++) hi[i] += 0.3 * r();
        TILE = { N: N, lo: lo, hi: hi };
        return TILE;
    }
    AD.world = { noise2: noise2, fbm: fbm, ridged: ridged, sstep: sstep, hexLin: hexLin, hexRgb: hexRgb, tileNoise: tileNoise, srgbLin: srgbLin };

    /* ---- distance field ------------------------------------------------------------- */
    const NEAR_R = 160, FAR_R = 1280, FAR_CELL = 32;
    AD.buildDistanceField = function (track, cell) {
        cell = cell || 8;
        const R = NEAR_R, b = track.bounds, S = track.samples;
        const minX = b.minX - R, minY = b.minY - R;
        const nx = Math.ceil((b.maxX + R - minX) / cell) + 1, ny = Math.ceil((b.maxY + R - minY) / cell) + 1;
        const near = new Float32Array(nx * ny).fill(R), idx = new Int32Array(nx * ny).fill(-1);
        const span = Math.ceil(R / cell);
        for (let i = 0; i < S.length; i++) {
            const s = S[i];
            const gx = round((s.x - minX) / cell), gy = round((s.y - minY) / cell);
            const x0 = max(0, gx - span), x1 = min(nx - 1, gx + span), y0 = max(0, gy - span), y1 = min(ny - 1, gy + span);
            for (let cy = y0; cy <= y1; cy++) {
                const py = minY + cy * cell - s.y, row = cy * nx;
                for (let cx = x0; cx <= x1; cx++) {
                    const px = minX + cx * cell - s.x, d = sqrt(px * px + py * py);
                    if (d < near[row + cx]) { near[row + cx] = d; idx[row + cx] = i; }
                }
            }
        }
        /* Coarse field for the large scale terms: a chamfer transform out to FAR_R. */
        const fminX = b.minX - FAR_R, fminY = b.minY - FAR_R;
        const fnx = Math.ceil((b.maxX + FAR_R - fminX) / FAR_CELL) + 1, fny = Math.ceil((b.maxY + FAR_R - fminY) / FAR_CELL) + 1;
        const far = new Float32Array(fnx * fny).fill(1e9);
        for (let i = 0; i < S.length; i += 2) {
            const s = S[i], gx = round((s.x - fminX) / FAR_CELL), gy = round((s.y - fminY) / FAR_CELL);
            const d = hypot(fminX + gx * FAR_CELL - s.x, fminY + gy * FAR_CELL - s.y);
            if (d < far[gy * fnx + gx]) far[gy * fnx + gx] = d;
        }
        const d1 = FAR_CELL, d2 = FAR_CELL * 1.4142;
        for (let y = 0; y < fny; y++) for (let x = 0; x < fnx; x++) {
            let v = far[y * fnx + x];
            if (x > 0) v = min(v, far[y * fnx + x - 1] + d1);
            if (y > 0) {
                v = min(v, far[(y - 1) * fnx + x] + d1);
                if (x > 0) v = min(v, far[(y - 1) * fnx + x - 1] + d2);
                if (x < fnx - 1) v = min(v, far[(y - 1) * fnx + x + 1] + d2);
            }
            far[y * fnx + x] = v;
        }
        for (let y = fny - 1; y >= 0; y--) for (let x = fnx - 1; x >= 0; x--) {
            let v = far[y * fnx + x];
            if (x < fnx - 1) v = min(v, far[y * fnx + x + 1] + d1);
            if (y < fny - 1) {
                v = min(v, far[(y + 1) * fnx + x] + d1);
                if (x < fnx - 1) v = min(v, far[(y + 1) * fnx + x + 1] + d2);
                if (x > 0) v = min(v, far[(y + 1) * fnx + x - 1] + d2);
            }
            far[y * fnx + x] = v;
        }
        /* Loop orientation: +normal is the inner side for a counter-clockwise loop. */
        let area = 0;
        for (let i = 0; i < S.length; i++) { const p = S[i], q = S[(i + 1) % S.length]; area += p.x * q.y - q.x * p.y; }
        const orient = area >= 0 ? 1 : -1;
        const poly = [];
        for (let i = 0; i < S.length; i += 8) poly.push(S[i]);

        function bilinear(grid, w, h, ox, oy, c, x, y, fallback) {
            const fx = (x - ox) / c, fy = (y - oy) / c;
            if (fx < 0 || fy < 0 || fx >= w - 1 || fy >= h - 1) {
                /* outside the grid: clamp to the edge and add what lies beyond it */
                const cx = clamp(fx, 0, w - 1.001), cy = clamp(fy, 0, h - 1.001);
                const out = hypot(fx - cx, fy - cy) * c;
                return sampleGrid(grid, w, cx, cy) + (fallback ? 0 : out);
            }
            return sampleGrid(grid, w, fx, fy);
        }
        function sampleGrid(grid, w, fx, fy) {
            const ix = floor(fx), iy = floor(fy), tx = fx - ix, ty = fy - iy, o = iy * w + ix;
            const a = grid[o], bb = grid[o + 1], cc = grid[o + w], dd = grid[o + w + 1];
            return a + (bb - a) * tx + (cc - a) * ty + (a - bb - cc + dd) * tx * ty;
        }
        const field = {
            cell: cell, radius: R, orient: orient, width: nx, height: ny,
            at(x, y) {
                const fx = (x - minX) / cell, fy = (y - minY) / cell;
                let d = R;
                if (fx >= 0 && fy >= 0 && fx < nx - 1 && fy < ny - 1) d = sampleGrid(near, nx, fx, fy);
                if (d < R - 16) return d;
                const f = bilinear(far, fnx, fny, fminX, fminY, FAR_CELL, x, y, false);
                return lerp(d, max(R - 1, f), sstep(R - 16, R, d));
            },
            info(x, y, out) {
                out = out || {};
                const gx = round((x - minX) / cell), gy = round((y - minY) / cell);
                let i = -1;
                if (gx >= 0 && gy >= 0 && gx < nx && gy < ny) i = idx[gy * nx + gx];
                if (i < 0) { out.halfWidth = track.width * 0.5; out.side = 0; out.inner = null; out.s = 0; out.field = field; return out; }
                const s = S[i], off = (x - s.x) * s.nx + (y - s.y) * s.ny;
                out.halfWidth = s.halfWidth; out.side = off >= 0 ? 1 : -1; out.inner = out.side * orient > 0; out.s = s.s; out.field = field;
                return out;
            },
            inside(x, y) {
                let c = false;
                for (let i = 0, j = poly.length - 1; i < poly.length; j = i++) {
                    const a = poly[i], q = poly[j];
                    if ((a.y > y) !== (q.y > y) && x < (q.x - a.x) * (y - a.y) / (q.y - a.y) + a.x) c = !c;
                }
                return c;
            }
        };
        return field;
    };

    /* ---- height rules ------------------------------------------------------------------ */
    const RAMP_END = { alpine: 85, desert: 46, forest: 56, glacier: 72 };
    function terrace(h, q, k) { const t = h / q, f = floor(t); return (f + sstep(0, k, t - f)) * q; }

    AD.terrainHeight = function (biomeId, dist, x, y, sample) {
        const rule = AD.TERRAIN[biomeId] || AD.TERRAIN.alpine;
        const hw = sample && sample.halfWidth ? sample.halfWidth : 12;
        const e = dist - hw;
        const base = rule.base !== undefined ? rule.base : -0.4;
        if (e <= 10) return base;
        const r = sstep(10, RAMP_END[biomeId] || 60, e);
        const fine = ((noise2(x / 9, y / 9) - 0.5) + (noise2(x / 27 + 11, y / 27) - 0.5) * 6.5) * sstep(12, 50, e);
        switch (rule.kind) {
            case 'hills': {
                const n = fbm(x / rule.wave, y / rule.wave, 4);
                let h = rule.amp * pow(clamp((n - 0.18) * 1.5, 0, 1), 1.25);
                h += (noise2(x / 52, y / 52) - 0.5) * 9 + (noise2(x / 17, y / 17) - 0.5) * 2.5;
                let peaks = 0;
                if (rule.ridged) peaks = rule.ridged.amp * pow(ridged(x / 620, y / 620, 4), 1.7) * sstep(rule.ridged.from * 0.4, rule.ridged.from * 1.1, dist);
                return base + r * h + fine + peaks;
            }
            case 'dunes': {
                const wx = x + 70 * (noise2(x / 190 + 7, y / 190) - 0.5), wy = y + 70 * (noise2(x / 190, y / 190 + 3) - 0.5);
                const crest = 1 - abs(2 * fbm(wx / rule.wave, wy / rule.wave, 2) - 1);
                let h = rule.amp * pow(crest, 1.5) + rule.amp * 0.3 * noise2(x / 60, y / 60);
                let mesa = 0;
                if (rule.mesas) {
                    const m = sstep(rule.mesas.from * 0.7, rule.mesas.from * 1.3, dist);
                    if (m > 0) {
                        const v = clamp((fbm(x / 540 + 3.3, y / 540, 3) - 0.47) * 5, 0, 1);
                        const levels = rule.mesas.amp / rule.mesas.quant, t = v * levels, f = floor(t);
                        mesa = m * (f + sstep(0, 0.16, t - f)) * rule.mesas.quant;
                    }
                }
                return base + r * h + fine + mesa;
            }
            case 'flat': {
                let h = base;
                if (rule.water) {
                    const inner = sample && sample.inner !== null && sample.inner !== undefined ? sample.inner : (sample && sample.field ? sample.field.inside(x, y) : true);
                    if (!inner) h = lerp(base, rule.water.depth, sstep(14, 26, e));
                    else h += 55 * pow(clamp((fbm(x / 800 + 1.7, y / 800, 3) - 0.5) * 3, 0, 1), 1.5) * sstep(500, 800, dist);
                }
                if (rule.ring) h += rule.ring.amp * pow(ridged(x / 700, y / 700, 4), 1.3) * sstep(rule.ring.from * 0.78, rule.ring.from * 1.25, dist);
                return h;
            }
            case 'rolling': {
                const n = fbm(x / rule.wave, y / rule.wave, 4);
                const h = rule.amp * pow(clamp((n - 0.2) * 1.55, 0, 1), 1.15) + (noise2(x / 34, y / 34) - 0.5) * 5;
                return base + r * h + fine;
            }
            case 'embankments': {
                const bank = rule.amp * sstep(10, 28, e) * (1 - 0.8 * sstep(70, 140, e)) * (0.65 + 0.7 * noise2(x / 90, y / 90));
                const lumps = (fbm(x / 160, y / 160, 3) - 0.5) * 5 * sstep(30, 90, e);
                const cone = clamp((fbm(x / 260 + 9, y / 260, 3) - 0.5) * 5, 0, 1);
                const heap = rule.heaps ? rule.heaps.amp * pow(cone, 1.3) * sstep(rule.heaps.from * 0.7, rule.heaps.from * 1.3, dist) : 0;
                return base + bank + lumps + fine + heap;
            }
            case 'ridged': {
                const h = terrace(rule.amp * pow(ridged(x / rule.wave, y / rule.wave, 3), 1.45), 14, 0.34) + 24 * fbm(x / 1500, y / 1500, 2);
                return base + r * h + fine;
            }
            default: return base;
        }
    };

    /* ---- colour ---------------------------------------------------------------------------- */
    const CP = {
        alpine: { k: 1.1, hs: 130, rock: 0.5, snow: 150, alt: '#93a655', snowCol: '#f4f0ea' },
        desert: { k: 1, hs: 70, rock: 0.42, bands: 7, alt: '#e8bd7a' },
        harbour: { k: 2.4, hs: 6, rock: 9, alt: '#34415a' },
        salt: { k: 1, hs: 120, rock: 0.5, alt: '#efe2da' },
        forest: { k: 1.45, hs: 40, rock: 0.62, alt: '#80602f' },
        industry: { k: 2.7, hs: 30, rock: 0.7, alt: '#6d4b3a' },
        glacier: { k: 2.5, hs: 80, rock: 0.62, snow: 38, alt: '#6f8ba6', snowCol: '#f1f8ff' },
        metro: { k: 2.9, hs: 3, rock: 9, alt: '#3e4150' }
    };
    AD.terrainPalette = function (biomeId, look) {
        const t = look.terrain, p = CP[biomeId] || CP.alpine, biome = (AD.BIOMES && AD.BIOMES[biomeId]) || { edge: '#6b7a5a' };
        const k = p.k || 1, lit = hex => hexLin(hex).map(v => min(0.92, v * k));
        return {
            low: lit(t.low), mid: lit(t.mid), high: lit(t.high), rock: lit(t.rock), edge: lit(biome.edge),
            alt: lit(p.alt), dark: lit(t.low).map(v => v * 0.55), snow: p.snowCol ? hexLin(p.snowCol) : null, p: p
        };
    };
    function mix3(out, a, b, t) { out[0] = a[0] + (b[0] - a[0]) * t; out[1] = a[1] + (b[1] - a[1]) * t; out[2] = a[2] + (b[2] - a[2]) * t; }
    const TMP = [0, 0, 0];
    AD.terrainColor = function (pal, h, slope, x, y, e, out) {
        const p = pal.p;
        const t = clamp(h / p.hs, 0, 1);
        const n1 = noise2(x / 70 + 3, y / 70), n2 = noise2(x / 21 + 5, y / 21), n3 = noise2(x / 6.5, y / 6.5), n4 = noise2(x / 190 + 40, y / 190), n5 = noise2(x / 11 + 80, y / 11 - 20);
        mix3(out, pal.low, pal.mid, sstep(0.12, 0.8, n1 * 0.85 + t * 0.5 + (n2 - 0.5) * 0.35));
        mix3(out, out, pal.high, sstep(0.4, 0.95, t + (n2 - 0.5) * 0.3));
        mix3(out, out, pal.alt, sstep(0.42, 0.7, n4 * 0.65 + n5 * 0.35) * 0.8);
        mix3(out, out, pal.dark, sstep(0.6, 0.85, 1 - n1) * 0.5 * (1 - t));
        const rk = sstep(p.rock, p.rock + 0.3, slope + (n3 - 0.5) * 0.3);
        if (p.bands) {
            const band = 0.5 + 0.5 * Math.sin(h / p.bands * 6.283 + n1 * 3.2);
            mix3(TMP, pal.rock, pal.high, band);
            mix3(out, out, TMP, rk);
        } else mix3(out, out, pal.rock, rk);
        if (pal.snow) {
            const s = sstep(p.snow, p.snow + 36, h + (n1 - 0.5) * 34) * (1 - sstep(0.75, 1.15, slope + (n3 - 0.5) * 0.2));
            mix3(out, out, pal.snow, s);
        }
        mix3(out, out, pal.edge, (1 - sstep(10, 36, e)) * 0.7);
        const v = 0.8 + 0.4 * n3 + 0.12 * (n5 - 0.5);
        out[0] *= v; out[1] *= v; out[2] *= v;
        return out;
    };

    /* ---- detail texture (neutral grey, multiplies the vertex colours) ------------------------- */
    AD.makeTerrainDetail = function (T, biomeId, look) {
        const tn = tileNoise(), N = tn.N;
        const c = document.createElement('canvas');
        c.width = c.height = N;
        const g = c.getContext('2d');
        const img = g.createImageData(N, N), d = img.data;
        const dark = look.night;
        const voro = biomeId === 'salt' ? voronoi(N, 18, 7) : null;
        for (let j = 0; j < N; j++) {
            for (let i = 0; i < N; i++) {
                const o = j * N + i, lo = tn.lo[o], hi = tn.hi[o];
                let v = 0.8 + 0.5 * (lo - 0.5) + 0.26 * (hi - 0.5);
                if (biomeId === 'desert') v += 0.035 * Math.sin((i / N * 24 + lo * 9) * 6.283);
                else if (biomeId === 'salt') v += voro[o];
                else if (biomeId === 'metro' || biomeId === 'harbour') {
                    const gx = (i % 64) < 1.5 || (j % 64) < 1.5;
                    if (gx) v *= 0.7;
                } else if (biomeId === 'glacier') v += 0.12 * (lo - 0.5);
                v = clamp(v, 0, 1.1) * (dark ? 1 : 1.05);
                const k = round(clamp(v, 0, 1) * 255);
                const tint = (tn.lo[(o + 197123) % (N * N)] - 0.5) * 0.14;
                d[o * 4] = clamp(k * (1 + tint), 0, 255); d[o * 4 + 1] = k; d[o * 4 + 2] = clamp(k * (1 - tint), 0, 255); d[o * 4 + 3] = 255;
            }
        }
        g.putImageData(img, 0, 0);
        const tex = new T.CanvasTexture(c);
        tex.colorSpace = T.SRGBColorSpace;
        tex.wrapS = tex.wrapT = T.RepeatWrapping;
        tex.anisotropy = 8;
        return tex;
    };
    /* Cracked crust: darker lines where two jittered cell centres are equally near. Periodic. */
    function voronoi(N, cells, seed) {
        const r = AD.rng(seed), px = new Float32Array(cells * cells), py = new Float32Array(cells * cells);
        for (let i = 0; i < cells * cells; i++) { px[i] = r(); py[i] = r(); }
        const out = new Float32Array(N * N), cs = N / cells, tint = new Float32Array(cells * cells);
        for (let i = 0; i < tint.length; i++) tint[i] = r() * 2 - 1;
        for (let j = 0; j < N; j++) for (let i = 0; i < N; i++) {
            const cx = floor(i / cs), cy = floor(j / cs);
            let f1 = 1e9, f2 = 1e9, best = 0;
            for (let oy = -1; oy <= 1; oy++) for (let ox = -1; ox <= 1; ox++) {
                const gx = (cx + ox + cells) % cells, gy = (cy + oy + cells) % cells, k = gy * cells + gx;
                const dx = (cx + ox + px[k]) * cs - i, dy = (cy + oy + py[k]) * cs - j, dd = sqrt(dx * dx + dy * dy);
                if (dd < f1) { f2 = f1; f1 = dd; best = k; } else if (dd < f2) f2 = dd;
            }
            const edge = f2 - f1;
            out[j * N + i] = edge < 1.3 ? -0.17 : (edge < 3 ? -0.05 : 0.045 * min(1, f1 / cs) + tint[best] * 0.05);
        }
        return out;
    }
    AD.world.voronoi = voronoi;

    /* ---- chunk geometry ----------------------------------------------------------------------- */
    const imul = Math.imul;
    function faceHash(i, j, t, seed) {
        let h = (imul(i + 1, 73856093) ^ imul(j + 7, 19349663) ^ imul(t + 3, 83492791) ^ seed) >>> 0;
        h = imul(h ^ (h >>> 15), 2246822519) >>> 0;
        return ((h ^ (h >>> 13)) >>> 0) / 4294967296;
    }

    /* A (n+1)^2 height grid expanded to separate triangles (a faceted low-poly look: every face gets
       its own small brightness and hue jitter) plus a skirt that hides LOD cracks between chunks. */
    function gridGeometry(T, ctx, x0, z0, size, cell, skirt) {
        const n = round(size / cell), m = n + 1, pm = n + 3;
        const hs = new Float32Array(pm * pm), es = new Float32Array(pm * pm);
        const smp = {};
        for (let j = 0; j < pm; j++) for (let i = 0; i < pm; i++) {
            const x = x0 + (i - 1) * cell, z = z0 + (j - 1) * cell;
            const dist = ctx.field.at(x, z);
            ctx.field.info(x, z, smp);
            hs[j * pm + i] = AD.terrainHeight(ctx.biomeId, dist, x, z, smp);
            es[j * pm + i] = dist - smp.halfWidth;
        }
        const vp = new Float32Array(m * m * 3), vc = new Float32Array(m * m * 3), c3 = [0, 0, 0];
        for (let j = 0; j < m; j++) for (let i = 0; i < m; i++) {
            const o = j * m + i, po = (j + 1) * pm + i + 1;
            const x = x0 + i * cell, z = z0 + j * cell, h = hs[po];
            const hx = (hs[po + 1] - hs[po - 1]) / (2 * cell), hz = (hs[po + pm] - hs[po - pm]) / (2 * cell);
            AD.terrainColor(ctx.pal, h, sqrt(hx * hx + hz * hz), x, z, es[po], c3);
            vp[o * 3] = x; vp[o * 3 + 1] = h; vp[o * 3 + 2] = z;
            vc[o * 3] = c3[0]; vc[o * 3 + 1] = c3[1]; vc[o * 3 + 2] = c3[2];
        }
        const tris = n * n * 2 + n * 4 * 2, total = tris * 3;
        const pos = new Float32Array(total * 3), col = new Float32Array(total * 3), uv = new Float32Array(total * 2);
        let w = 0;
        const seed = (round(x0) * 31 + round(z0)) | 0;
        function put(v, f, dark) {
            pos[w * 3] = vp[v * 3]; pos[w * 3 + 1] = vp[v * 3 + 1]; pos[w * 3 + 2] = vp[v * 3 + 2];
            col[w * 3] = vc[v * 3] * f[0] * dark; col[w * 3 + 1] = vc[v * 3 + 1] * f[1] * dark; col[w * 3 + 2] = vc[v * 3 + 2] * f[2] * dark;
            uv[w * 2] = vp[v * 3] / 32; uv[w * 2 + 1] = vp[v * 3 + 2] / 32;
            w++;
        }
        function putLow(v, f, dark) {
            put(v, f, dark);
            pos[(w - 1) * 3 + 1] -= skirt;
        }
        const fj = [1, 1, 1];
        function jitter(i, j, t) {
            const a = faceHash(i, j, t, seed), b = faceHash(i, j, t + 5, seed);
            const br = 0.88 + 0.24 * a, hue = (b - 0.5) * 0.08;
            fj[0] = br * (1 + hue); fj[1] = br; fj[2] = br * (1 - hue);
            return fj;
        }
        for (let j = 0; j < n; j++) for (let i = 0; i < n; i++) {
            const a = j * m + i, b = a + 1, c = a + m, d = c + 1;
            if ((i + j) & 1) {
                put(a, jitter(i, j, 0), 1); put(c, fj, 1); put(d, fj, 1);
                put(a, jitter(i, j, 1), 1); put(d, fj, 1); put(b, fj, 1);
            } else {
                put(a, jitter(i, j, 0), 1); put(c, fj, 1); put(b, fj, 1);
                put(b, jitter(i, j, 1), 1); put(c, fj, 1); put(d, fj, 1);
            }
        }
        /* skirt around the perimeter */
        const ring = [];
        for (let i = 0; i <= n; i++) ring.push(i);
        for (let j = 1; j <= n; j++) ring.push(j * m + n);
        for (let i = n - 1; i >= 0; i--) ring.push(n * m + i);
        for (let j = n - 1; j >= 1; j--) ring.push(j * m);
        const unit = [1, 1, 1];
        for (let k = 0; k < ring.length; k++) {
            const a = ring[k], b = ring[(k + 1) % ring.length];
            put(a, unit, 0.6); put(b, unit, 0.6); putLow(a, unit, 0.5);
            put(b, unit, 0.6); putLow(b, unit, 0.5); putLow(a, unit, 0.5);
        }
        const geo = new T.BufferGeometry();
        geo.setAttribute('position', new T.BufferAttribute(pos, 3));
        geo.setAttribute('color', new T.BufferAttribute(col, 3));
        geo.setAttribute('uv', new T.BufferAttribute(uv, 2));
        geo.computeBoundingSphere();
        return geo;
    }

    /* Coarse ring out to the horizon (one mesh, 64 m cells), aligned to the chunk rectangle. */
    function ringGeometry(T, ctx, rect, extent, cell) {
        const k = Math.ceil(extent / cell);
        const x0 = rect.x0 - k * cell, z0 = rect.z0 - k * cell;
        const nx = round((rect.x1 - rect.x0) / cell) + 2 * k, nz = round((rect.z1 - rect.z0) / cell) + 2 * k;
        const mx = nx + 1, mz = nz + 1, pm = mx + 2;
        const hs = new Float32Array(mx * mz), es = new Float32Array(mx * mz);
        const smp = {}, c3 = [0, 0, 0];
        const pos = new Float32Array(mx * mz * 3), col = new Float32Array(mx * mz * 3), uv = new Float32Array(mx * mz * 2);
        for (let j = 0; j < mz; j++) for (let i = 0; i < mx; i++) {
            const x = x0 + i * cell, z = z0 + j * cell, dist = ctx.field.at(x, z);
            ctx.field.info(x, z, smp);
            hs[j * mx + i] = AD.terrainHeight(ctx.biomeId, dist, x, z, smp);
            es[j * mx + i] = dist - smp.halfWidth;
        }
        for (let j = 0; j < mz; j++) for (let i = 0; i < mx; i++) {
            const o = j * mx + i, h = hs[o];
            const hx = (hs[j * mx + min(mx - 1, i + 1)] - hs[j * mx + max(0, i - 1)]) / (2 * cell);
            const hz = (hs[min(mz - 1, j + 1) * mx + i] - hs[max(0, j - 1) * mx + i]) / (2 * cell);
            const x = x0 + i * cell, z = z0 + j * cell;
            AD.terrainColor(ctx.pal, h, sqrt(hx * hx + hz * hz), x, z, es[o], c3);
            pos[o * 3] = x; pos[o * 3 + 1] = h; pos[o * 3 + 2] = z;
            col[o * 3] = c3[0]; col[o * 3 + 1] = c3[1]; col[o * 3 + 2] = c3[2];
            uv[o * 2] = x / 32; uv[o * 2 + 1] = z / 32;
        }
        const idx = [];
        for (let j = 0; j < nz; j++) for (let i = 0; i < nx; i++) {
            const cx = x0 + (i + 0.5) * cell, cz = z0 + (j + 0.5) * cell;
            if (cx > rect.x0 && cx < rect.x1 && cz > rect.z0 && cz < rect.z1) continue;
            const a = j * mx + i, b = a + 1, c = a + mx, d = c + 1;
            idx.push(a, c, b, b, c, d);
        }
        const geo = new T.BufferGeometry();
        geo.setAttribute('position', new T.BufferAttribute(pos, 3));
        geo.setAttribute('color', new T.BufferAttribute(col, 3));
        geo.setAttribute('uv', new T.BufferAttribute(uv, 2));
        geo.setIndex(new T.BufferAttribute(new Uint32Array(idx), 1));
        geo.computeBoundingSphere();
        void pm;
        return geo;
    }

    /* ---- the terrain object ---------------------------------------------------------------------- */
    AD.createTerrain = function (T, track, look, q) {
        const t0 = typeof performance !== 'undefined' ? performance.now() : 0;
        const biomeId = track.biomeId;
        const field = AD.buildDistanceField(track, 8);
        const ctx = { field: field, biomeId: biomeId, pal: AD.terrainPalette(biomeId, look) };
        const detail = AD.makeTerrainDetail(T, biomeId, look);
        const mat = new T.MeshStandardMaterial({
            vertexColors: true, map: detail, flatShading: true, roughness: 0.93, metalness: 0, side: T.DoubleSide,
            polygonOffset: true, polygonOffsetFactor: 1, polygonOffsetUnits: 2
        });
        const group = new T.Group();
        group.name = 'terrain';
        const geos = [];
        const rule = AD.TERRAIN[biomeId] || {};
        /* flat ground has no relief to resolve: half the resolution is enough */
        const nearCell = (q && q.terrainCell ? q.terrainCell : 8) * (rule.kind === 'flat' && !rule.water ? 2 : 1);
        const farCell = min(32, nearCell * 2);
        // GTAO draws the terrain twice; larger tiles retain the sample spacing with fewer calls.
        const SIZE = q && q.gtao ? 1024 : 256, M = 600, b = track.bounds;
        const cx0 = floor((b.minX - M) / SIZE), cy0 = floor((b.minY - M) / SIZE);
        let cx1 = floor((b.maxX + M) / SIZE), cy1 = floor((b.maxY + M) / SIZE);
        if ((cx1 - cx0 + 1) & 1) cx1++;
        if ((cy1 - cy0 + 1) & 1) cy1++;
        const stats = { chunks: 0, near: 0, far: 0, triangles: 0, ms: 0 };
        const isNear = (cx, cy) => field.at((cx + 0.5) * SIZE, (cy + 0.5) * SIZE) - SIZE * Math.SQRT1_2 < 300;
        function add(x0, z0, size, cell, near) {
            const geo = gridGeometry(T, ctx, x0, z0, size, cell, 14);
            geos.push(geo);
            const mesh = new T.Mesh(geo, mat);
            mesh.receiveShadow = true;
            mesh.castShadow = near && !(rule.kind === 'flat' && q && q.casters === 1);
            mesh.matrixAutoUpdate = false;
            group.add(mesh);
            stats.chunks++;
            if (near) stats.near++; else stats.far++;
            stats.triangles += geo.getAttribute('position').count / 3;
        }
        for (let cy = cy0; cy <= cy1; cy += 2) {
            for (let cx = cx0; cx <= cx1; cx += 2) {
                const nr = [isNear(cx, cy), isNear(cx + 1, cy), isNear(cx, cy + 1), isNear(cx + 1, cy + 1)];
                const anyNear = nr[0] || nr[1] || nr[2] || nr[3];
                if (!anyNear || (q && q.terrainBlock === 2)) { add(cx * SIZE, cy * SIZE, SIZE * 2, anyNear ? nearCell : farCell, anyNear); continue; }
                for (let k = 0; k < 4; k++) add((cx + (k & 1)) * SIZE, (cy + (k >> 1)) * SIZE, SIZE, nr[k] ? nearCell : farCell, nr[k]);
            }
        }
        const rect = { x0: cx0 * SIZE, z0: cy0 * SIZE, x1: (cx1 + 1) * SIZE, z1: (cy1 + 1) * SIZE };
        const rgeo = ringGeometry(T, ctx, rect, 1792, 64);
        geos.push(rgeo);
        const rmesh = new T.Mesh(rgeo, mat);
        rmesh.matrixAutoUpdate = false;
        group.add(rmesh);
        stats.chunks++; stats.triangles += rgeo.index.count / 3;
        stats.ms = typeof performance !== 'undefined' ? performance.now() - t0 : 0;

        return {
            group: group, field: field,
            setWetness(w) {
                mat.roughness = lerp(0.93, 0.55, w);
                mat.color.setScalar(lerp(1, 0.78, w));
            },
            stats() { return stats; },
            dispose() {
                for (const g of geos) g.dispose();
                mat.dispose(); detail.dispose();
                group.removeFromParent();
                geos.length = 0;
            }
        };
    };
})();
