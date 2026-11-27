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
    const wrapDist = (a, b, len) => { const d = abs(a - b) % len; return min(d, len - d); };

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
        const rz = new Float32Array(nx * ny);
        const span = Math.ceil(R / cell);
        for (let i = 0; i < S.length; i++) {
            const s = S[i];
            const gx = round((s.x - minX) / cell), gy = round((s.y - minY) / cell);
            const x0 = max(0, gx - span), x1 = min(nx - 1, gx + span), y0 = max(0, gy - span), y1 = min(ny - 1, gy + span);
            for (let cy = y0; cy <= y1; cy++) {
                const py = minY + cy * cell - s.y, row = cy * nx;
                for (let cx = x0; cx <= x1; cx++) {
                    const px = minX + cx * cell - s.x, d = sqrt(px * px + py * py);
                    if (d < near[row + cx]) { near[row + cx] = d; idx[row + cx] = i; rz[row + cx] = s.z || 0; }
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
            cell: cell, radius: R, orient: orient, width: nx, height: ny, length: track.length,
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
                if (i < 0) { out.halfWidth = track.width * 0.5; out.side = 0; out.inner = null; out.s = 0; out.roadZ = 0; out.field = field; return out; }
                const s = S[i], dx = x - s.x, dy = y - s.y, off = dx * s.nx + dy * s.ny;
                let roadZ = 0;
                if (track.maxZ) {
                    const along = dx * s.tx + dy * s.ty;
                    const exactZ = AD.roadProfileAt ? AD.roadProfileAt(track, s.s + along).z : (s.z || 0);
                    const d = hypot(dx, dy);
                    if (d <= 36) roadZ = exactZ;
                    else {
                        const fx = clamp((x - minX) / cell, 0, nx - 1.001), fy = clamp((y - minY) / cell, 0, ny - 1.001);
                        const smoothZ = sampleGrid(rz, nx, fx, fy);
                        roadZ = lerp(exactZ, smoothZ, sstep(36, 72, d)) * (1 - sstep(R - 48, R - 8, d));
                    }
                }
                out.halfWidth = s.halfWidth; out.side = off >= 0 ? 1 : -1; out.inner = out.side * orient > 0; out.s = s.s; out.roadZ = roadZ; out.field = field;
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
    const RAMP_END = { alpine: 95, desert: 68, forest: 64, glacier: 84 };
    function terrace(h, q, k) { const t = h / q, f = floor(t); return (f + sstep(0, k, t - f)) * q; }

    AD.terrainHeight = function (biomeId, dist, x, y, sample) {
        const rule = AD.TERRAIN[biomeId] || AD.TERRAIN.alpine;
        const hw = sample && sample.halfWidth ? sample.halfWidth : 12;
        const e = dist - hw;
        const base = rule.base !== undefined ? rule.base : -0.4;
        const roadZ = (sample && sample.roadZ) || 0;
        if (e <= 10) return roadZ + base;
        const r = sstep(10, RAMP_END[biomeId] || 68, e);
        const roadLift = roadZ ? roadZ * (1 - r) : 0;
        /* Keep near-verge micro-noise calm so trackside props sit flush on gentle slopes. */
        const fine = ((noise2(x / 14, y / 14) - 0.5) * 0.7 + (noise2(x / 42 + 11, y / 42) - 0.5) * 2.2) * sstep(22, 78, e);
        /* Grade a level Start/Finish paddock terrace beside s = 0 so grandstands never bury into hills. */
        const len = sample && sample.field && sample.field.length;
        const paddock = len ? (1 - sstep(78, 128, wrapDist(sample.s || 0, 0, len))) * (1 - sstep(22, 46, e)) : 0;
        let off = 0;
        switch (rule.kind) {
            case 'hills': {
                const n = fbm(x / rule.wave, y / rule.wave, 4);
                let h = rule.amp * pow(clamp((n - 0.18) * 1.5, 0, 1), 1.25);
                h += (noise2(x / 52, y / 52) - 0.5) * 9 + (noise2(x / 17, y / 17) - 0.5) * 2.5;
                let peaks = 0;
                if (rule.ridged) peaks = rule.ridged.amp * pow(ridged(x / 620, y / 620, 4), 1.7) * sstep(rule.ridged.from * 0.4, rule.ridged.from * 1.1, dist);
                off = r * h + fine + peaks;
                break;
            }
            case 'dunes': {
                const wx = x + 70 * (noise2(x / 190 + 7, y / 190) - 0.5), wy = y + 70 * (noise2(x / 190, y / 190 + 3) - 0.5);
                const crest = 1 - abs(2 * fbm(wx / rule.wave, wy / rule.wave, 2) - 1);
                let h = rule.amp * pow(crest, 1.5) + rule.amp * 0.3 * noise2(x / 60, y / 60);
                let mesa = 0;
                if (rule.mesas) {
                    const m = sstep(rule.mesas.from * 0.85, rule.mesas.from * 1.45, dist);
                    if (m > 0) {
                        const v = clamp((fbm(x / 540 + 3.3, y / 540, 3) - 0.47) * 5, 0, 1);
                        const levels = rule.mesas.amp / rule.mesas.quant, t = v * levels, f = floor(t);
                        mesa = m * (f + sstep(0, 0.42, t - f)) * rule.mesas.quant;
                    }
                }
                off = r * h + fine + mesa;
                break;
            }
            case 'flat': {
                if (rule.water) {
                    const inner = sample && sample.inner !== null && sample.inner !== undefined ? sample.inner : (sample && sample.field ? sample.field.inside(x, y) : true);
                    const wFrom = rule.water.from !== undefined ? rule.water.from : 14;
                    const wTo = rule.water.to !== undefined ? rule.water.to : 26;
                    if (!inner) return base + roadLift + (rule.water.depth - base) * sstep(wFrom, wTo, e);
                    off = 55 * pow(clamp((fbm(x / 800 + 1.7, y / 800, 3) - 0.5) * 3, 0, 1), 1.5) * sstep(500, 800, dist);
                } else if (rule.ring) {
                    off = rule.ring.amp * pow(ridged(x / 700, y / 700, 4), 1.3) * sstep(rule.ring.from * 0.78, rule.ring.from * 1.25, dist);
                }
                break;
            }
            case 'rolling': {
                const n = fbm(x / rule.wave, y / rule.wave, 4);
                const h = rule.amp * pow(clamp((n - 0.2) * 1.55, 0, 1), 1.15) + (noise2(x / 34, y / 34) - 0.5) * 5;
                off = r * h + fine;
                break;
            }
            case 'embankments': {
                const bank = rule.amp * sstep(14, 42, e) * (1 - 0.8 * sstep(75, 145, e)) * (0.65 + 0.7 * noise2(x / 90, y / 90));
                const lumps = (fbm(x / 160, y / 160, 3) - 0.5) * 5 * sstep(30, 90, e);
                const cone = clamp((fbm(x / 260 + 9, y / 260, 3) - 0.5) * 5, 0, 1);
                const heap = rule.heaps ? rule.heaps.amp * pow(cone, 1.3) * sstep(rule.heaps.from * 0.7, rule.heaps.from * 1.3, dist) : 0;
                off = bank + lumps + fine + heap;
                break;
            }
            case 'ridged': {
                const h = terrace(rule.amp * pow(ridged(x / rule.wave, y / rule.wave, 3), 1.45), 16, 0.62) + 24 * fbm(x / 1500, y / 1500, 2);
                off = r * h + fine;
                break;
            }
            default: break;
        }
        return base + roadLift + off * (1 - paddock);
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
        const ao = 0.84 + 0.16 * sstep(0, 14, e);
        const city = (p.hs <= 6) ? (0.9 + 0.2 * noise2(x / 64, y / 64)) : 1;
        const v = (0.88 + 0.24 * n3 + 0.08 * (n5 - 0.5)) * ao * city;
        out[0] *= v; out[1] *= v; out[2] *= v;
        if (h < -0.55 && p.hs === 6) {
            const wet = sstep(-0.55, -5.0, h);
            out[0] = lerp(out[0], 0.05, wet); out[1] = lerp(out[1], 0.10, wet); out[2] = lerp(out[2], 0.17, wet);
        }
        return out;
    };

    /* ---- detail & bump textures (neutral grey, multiplies vertex colours + surface relief) ------ */
    function terrainSurfaceData(biomeId, look) {
        const tn = tileNoise(), N = tn.N, h = new Float32Array(N * N);
        const dark = look.night;
        const voro = biomeId === 'salt' ? voronoi(N, 18, 7) : null;
        const rgba = new Uint8Array(N * N * 4), nrm = new Uint8Array(N * N * 4);
        for (let j = 0; j < N; j++) {
            for (let i = 0; i < N; i++) {
                const o = j * N + i, lo = tn.lo[o], hi = tn.hi[o];
                let v = 0.84 + 0.42 * (lo - 0.5) + 0.22 * (hi - 0.5);
                if (biomeId === 'desert') v += 0.045 * Math.sin((i / N * 24 + lo * 9) * 6.283);
                else if (biomeId === 'salt') v += voro[o];
                else if (biomeId === 'metro' || biomeId === 'harbour') {
                    const seam = (i % 64) < 2 || (j % 64) < 2, block = (floor(i / 64) * 13 + floor(j / 64) * 7) % 5;
                    v *= 0.92 + block * 0.03;
                    if (seam) v *= 0.68;
                } else if (biomeId === 'glacier') v += 0.06 * Math.sin((i / N * 12 + j / N * 6 + lo * 5) * 6.283) + 0.1 * (lo - 0.5);
                h[o] = clamp(v, 0, 1.2);
                const k = round(clamp(h[o] * (dark ? 1 : 1.05), 0, 1) * 255);
                const tint = (tn.lo[(o + 197123) % (N * N)] - 0.5) * 0.1;
                rgba[o * 4] = clamp(k * (1 + tint), 0, 255); rgba[o * 4 + 1] = k; rgba[o * 4 + 2] = clamp(k * (1 - tint), 0, 255); rgba[o * 4 + 3] = 255;
                const bVal = round(clamp(h[o], 0, 1) * 255);
                nrm[o * 4] = nrm[o * 4 + 1] = nrm[o * 4 + 2] = bVal; nrm[o * 4 + 3] = 255;
            }
        }
        return { N, rgba, nrm };
    }
    AD.makeTerrainDetail = function (T, biomeId, look) {
        const s = terrainSurfaceData(biomeId, look);
        const tex = new T.DataTexture(s.rgba, s.N, s.N, T.RGBAFormat);
        tex.colorSpace = T.SRGBColorSpace; tex.wrapS = tex.wrapT = T.RepeatWrapping; tex.anisotropy = 8;
        tex.generateMipmaps = true; tex.magFilter = T.LinearFilter; tex.minFilter = T.LinearMipmapLinearFilter;
        tex.needsUpdate = true;
        return tex;
    };
    AD.makeTerrainBump = function (T, biomeId, look) {
        const s = terrainSurfaceData(biomeId, look);
        const tex = new T.DataTexture(s.nrm, s.N, s.N, T.RGBAFormat);
        tex.wrapS = tex.wrapT = T.RepeatWrapping; tex.anisotropy = 4;
        tex.generateMipmaps = true; tex.magFilter = T.LinearFilter; tex.minFilter = T.LinearMipmapLinearFilter;
        tex.needsUpdate = true;
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
    /* A (n+1)^2 height grid expanded to smooth-normal triangles plus a skirt that hides LOD cracks. */
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
        const vp = new Float32Array(m * m * 3), vc = new Float32Array(m * m * 3), vn = new Float32Array(m * m * 3), c3 = [0, 0, 0];
        for (let j = 0; j < m; j++) for (let i = 0; i < m; i++) {
            const o = j * m + i, po = (j + 1) * pm + i + 1;
            const x = x0 + i * cell, z = z0 + j * cell, h = hs[po];
            const hx = (hs[po + 1] - hs[po - 1]) / (2 * cell), hz = (hs[po + pm] - hs[po - pm]) / (2 * cell);
            const nLen = sqrt(hx * hx + 1 + hz * hz) || 1;
            AD.terrainColor(ctx.pal, h, sqrt(hx * hx + hz * hz), x, z, es[po], c3);
            vp[o * 3] = x; vp[o * 3 + 1] = h; vp[o * 3 + 2] = z;
            vc[o * 3] = c3[0]; vc[o * 3 + 1] = c3[1]; vc[o * 3 + 2] = c3[2];
            vn[o * 3] = -hx / nLen; vn[o * 3 + 1] = 1 / nLen; vn[o * 3 + 2] = -hz / nLen;
        }
        const foliage = [];
        for (const plant of ctx.plants) {
            if (plant.x < x0 || plant.x >= x0 + size || plant.y < z0 || plant.y >= z0 + size) continue;
            const shades = plant.palette.map(hexLin);
            // Anchor the roots on the rendered triangle, including on coarse quality tiers.
            const gx = (plant.x - x0) / cell, gz = (plant.y - z0) / cell;
            const i = floor(gx), j = floor(gz), u = gx - i, v = gz - j;
            const k = (j + 1) * pm + i + 1, a = hs[k], b = hs[k + 1], c = hs[k + pm], d = hs[k + pm + 1];
            const diagonal = (i + j) & 1 ? min(u, v) : max(0, u + v - 1);
            const z = a + (b - a) * u + (c - a) * v + (a - b - c + d) * diagonal;
            AD.vergeTriangles({ ...plant, z }, (a, b, c, shade) => foliage.push({ vertices: [a, b, c], colour: shades[shade] }));
        }
        const tris = n * n * 2 + n * 4 * 2 + foliage.length, total = tris * 3;
        const pos = new Float32Array(total * 3), col = new Float32Array(total * 3), nrm = new Float32Array(total * 3), uv = new Float32Array(total * 2);
        let w = 0;
        function put(v, dark) {
            pos[w * 3] = vp[v * 3]; pos[w * 3 + 1] = vp[v * 3 + 1]; pos[w * 3 + 2] = vp[v * 3 + 2];
            col[w * 3] = vc[v * 3] * dark; col[w * 3 + 1] = vc[v * 3 + 1] * dark; col[w * 3 + 2] = vc[v * 3 + 2] * dark;
            nrm[w * 3] = vn[v * 3]; nrm[w * 3 + 1] = vn[v * 3 + 1]; nrm[w * 3 + 2] = vn[v * 3 + 2];
            uv[w * 2] = vp[v * 3] / 32; uv[w * 2 + 1] = vp[v * 3 + 2] / 32;
            w++;
        }
        function putLow(v, dark) {
            put(v, dark);
            pos[(w - 1) * 3 + 1] -= skirt;
        }
        for (let j = 0; j < n; j++) for (let i = 0; i < n; i++) {
            const a = j * m + i, b = a + 1, c = a + m, d = c + 1;
            if ((i + j) & 1) {
                put(a, 1); put(c, 1); put(d, 1);
                put(a, 1); put(d, 1); put(b, 1);
            } else {
                put(a, 1); put(c, 1); put(b, 1);
                put(b, 1); put(c, 1); put(d, 1);
            }
        }
        /* skirt around the perimeter */
        const ring = [];
        for (let i = 0; i <= n; i++) ring.push(i);
        for (let j = 1; j <= n; j++) ring.push(j * m + n);
        for (let i = n - 1; i >= 0; i--) ring.push(n * m + i);
        for (let j = n - 1; j >= 1; j--) ring.push(j * m);
        for (let k = 0; k < ring.length; k++) {
            const a = ring[k], b = ring[(k + 1) % ring.length];
            put(a, 0.72); put(b, 0.72); putLow(a, 0.58);
            put(b, 0.72); putLow(b, 0.58); putLow(a, 0.58);
        }
        for (const face of foliage) {
            const A = face.vertices[0], B = face.vertices[1], C = face.vertices[2];
            const ux = B[0] - A[0], uy = B[1] - A[1], uz = B[2] - A[2];
            const vx = C[0] - A[0], vy = C[1] - A[1], vz = C[2] - A[2];
            let fx = uy * vz - uz * vy, fy = uz * vx - ux * vz + 0.65, fz = ux * vy - uy * vx;
            const fl = sqrt(fx * fx + fy * fy + fz * fz) || 1;
            fx /= fl; fy /= fl; fz /= fl;
            for (const v of face.vertices) {
                pos.set(v, w * 3); col.set(face.colour, w * 3);
                nrm[w * 3] = fx; nrm[w * 3 + 1] = fy; nrm[w * 3 + 2] = fz;
                uv[w * 2] = v[0] / 32; uv[w * 2 + 1] = v[2] / 32; w++;
            }
        }
        const geo = new T.BufferGeometry();
        geo.setAttribute('position', new T.BufferAttribute(pos, 3));
        geo.setAttribute('color', new T.BufferAttribute(col, 3));
        geo.setAttribute('normal', new T.BufferAttribute(nrm, 3));
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
        geo.computeVertexNormals();
        geo.computeBoundingSphere();
        void pm;
        return geo;
    }

    /* ---- ground height & rendered mesh query for world objects ----------------------------------- */
    /* One distance field per track: the terrain mesh, the props, the roadside scenery and the pet
       decor all have to stand on the same ground. The field also knows which side of the loop a
       point lies on, and the harbour needs that side: its ground sinks into the sea outside the
       quay. Without it every object there hovers a metre above the water. */
    const fieldCache = new WeakMap();
    const _msmp = {};
    AD.trackField = function (track, cell) {
        cell = cell || 8;
        let field = fieldCache.get(track);
        if (!field || field.cell !== cell) {
            field = AD.buildDistanceField(track, cell);
            fieldCache.set(track, field);
        }
        return field;
    };

    /* Ground height under a world point, from the same field the terrain mesh is built from. */
    AD.groundHeight = function (track, biomeId, x, y) {
        if (!track) return 0;
        const field = AD.trackField(track, 8);
        const dist = field.at(x, y);
        field.info(x, y, _msmp);
        const h = AD.terrainHeight(biomeId || track.biomeId, dist, x, y, _msmp);
        return isFinite(h) ? h : 0;
    };

    /* Exact height on the rendered terrain triangle at (x, y) for grid cell size `cell`. */
    AD.terrainMeshHeight = function (track, biomeId, x, y, cell) {
        if (!track) return 0;
        cell = cell || 8;
        const id = biomeId || track.biomeId;
        const field = AD.trackField(track, 8);
        const gi = floor(x / cell), gj = floor(y / cell);
        const u = x / cell - gi, v = y / cell - gj;
        const x0 = gi * cell, z0 = gj * cell;
        const at = (px, py) => {
            const dist = field.at(px, py);
            field.info(px, py, _msmp);
            return AD.terrainHeight(id, dist, px, py, _msmp);
        };
        const a = at(x0, z0), b = at(x0 + cell, z0), c = at(x0, z0 + cell), d = at(x0 + cell, z0 + cell);
        const diag = ((gi + gj) & 1) ? min(u, v) : max(0, u + v - 1);
        const h = a + (b - a) * u + (c - a) * v + (a - b - c + d) * diag;
        return isFinite(h) ? h : 0;
    };

    /* Terrain slope & height span over a footprint radius for slope-safe prop filtering. */
    AD.terrainSlopeAt = function (track, biomeId, x, y, radius) {
        const d = max(2, radius || 4);
        const id = biomeId || track.biomeId;
        const hl = AD.groundHeight(track, id, x - d, y), hr = AD.groundHeight(track, id, x + d, y);
        const hd = AD.groundHeight(track, id, x, y - d), hu = AD.groundHeight(track, id, x, y + d);
        const hc = AD.groundHeight(track, id, x, y);
        const hx = (hr - hl) / (2 * d), hz = (hu - hd) / (2 * d);
        const lo = min(hc, hl, hr, hd, hu), hi = max(hc, hl, hr, hd, hu);
        return { slope: hypot(hx, hz), hx: hx, hz: hz, minH: lo, maxH: hi, span: hi - lo };
    };

    /* ---- the terrain object ---------------------------------------------------------------------- */
    AD.createTerrain = function (T, track, look, q) {
        const t0 = typeof performance !== 'undefined' ? performance.now() : 0;
        const biomeId = track.biomeId;
        const field = AD.trackField(track, 8);
        const ctx = { field: field, biomeId: biomeId, pal: AD.terrainPalette(biomeId, look), plants: AD.vergePlants?.(track) || [] };
        const detail = AD.makeTerrainDetail(T, biomeId, look);
        const bump = AD.makeTerrainBump(T, biomeId, look);
        const rule = AD.TERRAIN[biomeId] || {};
        const mat = new T.MeshStandardMaterial({
            vertexColors: true, map: detail, bumpMap: bump,
            bumpScale: rule.kind === 'flat' && !rule.water ? 0.14 : (biomeId === 'harbour' ? 0.18 : 0.34),
            roughness: 0.9, metalness: 0, side: T.DoubleSide,
            polygonOffset: true, polygonOffsetFactor: 1, polygonOffsetUnits: 2
        });
        const group = new T.Group();
        group.name = 'terrain';
        const geos = [];
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
                mat.dispose(); detail.dispose(); bump.dispose();
                group.removeFromParent();
                geos.length = 0;
            }
        };
    };
})();
