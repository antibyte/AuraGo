(function () {
    'use strict';

    // Feature x/z are cell indices (fractional for decorations); every y in features, decorations and waterfall.bottomY is a WORLD y (grid y index = world y + yBase).
    const SIZE = Object.freeze({ w: 64, d: 56, h: 64, yBase: 30 });
    const MATERIAL = Object.freeze({
        AIR: 0, GRASS: 1, DIRT: 2, STONE: 3, DEEP_STONE: 4, SNOW: 5, SAND: 6, WATER: 7, OAK_LOG: 8,
        OAK_LEAVES: 9, PINE_LEAVES: 10, BIRCH_LOG: 11, BIRCH_LEAVES: 12, PLANKS: 13, PLASTER: 14, ROOF_RED: 15,
        ROOF_BLUE: 16, COBBLE: 17, GRAVEL: 18, WHEAT: 19, WINDOW: 20, LANTERN: 21, CRYSTAL: 22, ROOT: 23,
        GOLD_ORE: 24, MOSS: 25
    });
    const M = MATERIAL;

    const INFO = [];
    function def(id, name, color, jitter, group, glow) {
        INFO[id] = Object.freeze(glow ? { name, color, jitter, group, glow } : { name, color, jitter, group });
    }
    def(M.AIR, 'air', '#000000', 0, 'solid');
    def(M.GRASS, 'grass', '#6fae44', .07, 'solid');
    def(M.DIRT, 'dirt', '#8a5e3c', .06, 'solid');
    def(M.STONE, 'stone', '#8e9192', .07, 'solid');
    def(M.DEEP_STONE, 'deep_stone', '#5f6470', .06, 'solid');
    def(M.SNOW, 'snow', '#f2f5f7', .03, 'solid');
    def(M.SAND, 'sand', '#e2d39a', .05, 'solid');
    def(M.WATER, 'water', '#3f8fc4', 0, 'water');
    def(M.OAK_LOG, 'oak_log', '#6e5232', .05, 'solid');
    def(M.OAK_LEAVES, 'oak_leaves', '#4f8f34', .1, 'foliage');
    def(M.PINE_LEAVES, 'pine_leaves', '#2f6b3e', .08, 'foliage');
    def(M.BIRCH_LOG, 'birch_log', '#e8e4d8', .06, 'solid');
    def(M.BIRCH_LEAVES, 'birch_leaves', '#82b84a', .1, 'foliage');
    def(M.PLANKS, 'planks', '#b38a55', .05, 'solid');
    def(M.PLASTER, 'plaster', '#efe7d6', .03, 'solid');
    def(M.ROOF_RED, 'roof_red', '#b5443a', .06, 'solid');
    def(M.ROOF_BLUE, 'roof_blue', '#4f6fa8', .06, 'solid');
    def(M.COBBLE, 'cobble', '#85878a', .08, 'solid');
    def(M.GRAVEL, 'gravel', '#a39d93', .08, 'solid');
    def(M.WHEAT, 'wheat', '#e0bd4e', .1, 'foliage');
    def(M.WINDOW, 'window', '#39506e', 0, 'glow', '#ffc76a');
    def(M.LANTERN, 'lantern', '#f6d27a', 0, 'glow', '#ffb84a');
    def(M.CRYSTAL, 'crystal', '#9b6be0', .08, 'glow', '#c48dff');
    def(M.ROOT, 'root', '#6b4a2c', .05, 'solid');
    def(M.GOLD_ORE, 'gold_ore', '#d9b23a', .05, 'solid');
    def(M.MOSS, 'moss', '#5f8a46', .08, 'solid');
    const MATERIAL_INFO = Object.freeze(INFO);

    const CX = 32;
    const CZ = 28;
    const WATER_LEVEL = 3;
    const LAKE = Object.freeze({ x: 44, z: 34, r: 4.5 });
    const PLATEAU = Object.freeze({ x0: 29, z0: 23, x1: 41, z1: 33, top: 4 });
    const HOUSES = [
        { x: 30, z: 24, w: 6, d: 5, roof: M.ROOF_RED },
        { x: 36, z: 25, w: 5, d: 5, roof: M.ROOF_BLUE },
        { x: 31, z: 30, w: 5, d: 4, roof: M.ROOF_RED }
    ];
    const WINDMILL = Object.freeze({ x0: 42, z0: 15, hubZ: 18 });
    // Windmill -> village -> dock (the land end is appended once the dock is known), village -> mountain foot.
    // The plan's village point (35, 28) lies inside house 0, so both routes bend through the free cells around it.
    const PATH_TO_DOCK = [[43, 18], [42, 24], [41, 30], [37, 30]];
    const PATH_TO_MOUNTAIN = [[37, 30], [36, 30], [35, 29], [29, 29], [26, 24]];
    const FLOWER_COLORS = ['#e8453c', '#f5d33f', '#f4f1ea', '#b66be0'];

    // Per-cell planning flags used to keep trees, flowers and lanterns off structures.
    const F_LAKE = 1;
    const F_SHORE = 2;
    const F_RIVER = 4;
    const F_PATH = 8;
    const F_FIELD = 16;
    const F_BUILD = 32;
    const F_PLATEAU = 64;
    const F_POST = 128;

    function clamp(value, min, max) { return Math.max(min, Math.min(max, value)); }

    function mulberry32(seed) {
        let a = seed >>> 0;
        return function () {
            a = (a + 0x6d2b79f5) >>> 0;
            let t = a;
            t = Math.imul(t ^ (t >>> 15), t | 1);
            t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
            return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
        };
    }

    // Signed value noise (-1..1) on a seeded permutation table.
    function makeNoise(rng) {
        const perm = new Uint8Array(512);
        const vals = new Float32Array(256);
        for (let i = 0; i < 256; i++) { perm[i] = i; vals[i] = rng() * 2 - 1; }
        for (let i = 255; i > 0; i--) {
            const j = Math.floor(rng() * (i + 1));
            const t = perm[i];
            perm[i] = perm[j];
            perm[j] = t;
        }
        for (let i = 0; i < 256; i++) perm[i + 256] = perm[i];
        const lattice = (ix, iy) => vals[perm[perm[ix & 255] + (iy & 255)]];
        return function noise(x, y) {
            const ix = Math.floor(x);
            const iy = Math.floor(y);
            const fx = x - ix;
            const fy = y - iy;
            const u = fx * fx * (3 - 2 * fx);
            const v = fy * fy * (3 - 2 * fy);
            const top = lattice(ix, iy) + (lattice(ix + 1, iy) - lattice(ix, iy)) * u;
            const bottom = lattice(ix, iy + 1) + (lattice(ix + 1, iy + 1) - lattice(ix, iy + 1)) * u;
            return top + (bottom - top) * v;
        };
    }

    function makeGrid(w, d, h, yBase) {
        const voxels = new Uint8Array(w * d * h);
        const plane = w * d;
        return {
            w, d, h, yBase, voxels,
            // Writes are clamped to the grid; y is a world y.
            put(x, y, z, m) {
                const gy = y + yBase;
                if (x < 0 || z < 0 || gy < 0 || x >= w || z >= d || gy >= h) return false;
                voxels[x + z * w + gy * plane] = m;
                return true;
            },
            putIfAir(x, y, z, m) {
                const gy = y + yBase;
                if (x < 0 || z < 0 || gy < 0 || x >= w || z >= d || gy >= h) return false;
                const i = x + z * w + gy * plane;
                if (voxels[i] !== M.AIR) return false;
                voxels[i] = m;
                return true;
            },
            get(x, y, z) {
                const gy = y + yBase;
                if (x < 0 || z < 0 || gy < 0 || x >= w || z >= d || gy >= h) return M.AIR;
                return voxels[x + z * w + gy * plane];
            }
        };
    }

    function countSolid(voxels) {
        let n = 0;
        for (let i = 0; i < voxels.length; i++) if (voxels[i] !== M.AIR && voxels[i] !== M.WATER) n++;
        return n;
    }

    function shuffled(list, rng) {
        const out = list.slice();
        for (let i = out.length - 1; i > 0; i--) {
            const j = Math.floor(rng() * (i + 1));
            const t = out[i];
            out[i] = out[j];
            out[j] = t;
        }
        return out;
    }

    function inRect(x, z, x0, z0, x1, z1) { return x >= x0 && x <= x1 && z >= z0 && z <= z1; }

    // Cells the lake must leave alone: the houses with a one cell margin.
    function reservedForHouse(x, z) {
        for (const hs of HOUSES) if (inRect(x, z, hs.x - 2, hs.z - 2, hs.x + hs.w + 1, hs.z + hs.d + 1)) return true;
        return false;
    }

    function newState(seed) {
        const rng = mulberry32(seed);
        const noise = makeNoise(rng);
        const w = SIZE.w;
        const d = SIZE.d;
        const cells = w * d;
        return {
            seed, rng, noise, w, d, cells,
            fbm: (x, y) => (noise(x, y) + .5 * noise(x * 2 + 17, y * 2 + 9) + .25 * noise(x * 4 + 31, y * 4 + 5)) / 1.75,
            g: makeGrid(SIZE.w, SIZE.d, SIZE.h, SIZE.yBase),
            inside: new Uint8Array(cells),
            rr: new Float32Array(cells),
            H: new Int16Array(cells),
            mountain: new Float32Array(cells),
            flags: new Uint8Array(cells),
            water: new Uint8Array(cells),
            topSet: new Uint8Array(cells),
            topMat: new Uint8Array(cells),
            bottom: new Int16Array(cells),
            underDepth: new Int16Array(cells),
            features: {
                center: [CX, CZ], waterLevel: WATER_LEVEL, lake: { x: LAKE.x, z: LAKE.z, r: LAKE.r },
                waterfall: null, chimneys: [], windmill: null, lanterns: [], windows: [], crystals: [],
                dock: null, boat: { x: 46, y: 3.2, z: 35, heading: .4 }, meadow: null, trees: []
            },
            decorations: [],
            pathCells: []
        };
    }

    // Steps 1-5, 9, 12: footprint, heights, plateau, lake, river, paths and field marks.
    function planTerrain(s) {
        const { w, d, noise, fbm, inside, rr, H, mountain, flags, water, topSet, features } = s;
        for (let z = 0; z < d; z++) {
            for (let x = 0; x < w; x++) {
                const i = x + z * w;
                const dx = x - CX;
                const dz = z - CZ;
                const dist = Math.hypot(dx, dz);
                const cs = dist > 0 ? dx / dist : 1;
                const sn = dist > 0 ? dz / dist : 0;
                const R = 22 + 3.2 * noise(cs * 1.7, sn * 1.7) + 1.6 * noise(cs * 4.3 + 9, sn * 4.3 + 9);
                if (dist >= R) continue;
                const r = dist / R;
                inside[i] = 1;
                rr[i] = r;
                const gm = Math.exp(-((x - 21) ** 2 + (z - 17) ** 2) / (2 * 6.5 * 6.5));
                const mt = 16 * gm + (gm > .002 ? 2 * noise(x * .3 + 40, z * .3 + 40) * clamp(4 * gm * (1 - gm), 0, 1) : 0);
                const hill = 4 * Math.exp(-((x - 43) ** 2 + (z - 16) ** 2) / (2 * 3.5 * 3.5));
                mountain[i] = mt;
                let h = Math.round(3 + Math.round(1.6 * fbm(x * .09, z * .09)) + mt + hill);
                if (r > .9) h -= 1;
                if (r > .97) h -= 1;
                H[i] = Math.max(1, h);
            }
        }
        const isIn = (x, z) => x >= 0 && z >= 0 && x < w && z < d && inside[x + z * w] === 1;

        for (let z = PLATEAU.z0; z <= PLATEAU.z1; z++) {
            for (let x = PLATEAU.x0; x <= PLATEAU.x1; x++) {
                if (!isIn(x, z)) continue;
                H[x + z * w] = PLATEAU.top;
                flags[x + z * w] |= F_PLATEAU;
            }
        }

        // Lake with a sand shore ring; the village keeps its footprint.
        for (let z = 0; z < d; z++) {
            for (let x = 0; x < w; x++) {
                const i = x + z * w;
                if (!inside[i] || Math.abs(x - LAKE.x) > 8 || Math.abs(z - LAKE.z) > 8 || reservedForHouse(x, z)) continue;
                const dx = x - LAKE.x;
                const dz = z - LAKE.z;
                const dist = Math.hypot(dx, dz);
                const lakeR = LAKE.r + .8 * noise((dist > 0 ? dx / dist : 1) * 2 + 20, (dist > 0 ? dz / dist : 0) * 2 + 20);
                if (dist < lakeR) {
                    H[i] = dist < lakeR * .55 ? 1 : 2;
                    water[i] = 1;
                    topSet[i] = M.SAND;
                    flags[i] |= F_LAKE;
                } else if (dist < lakeR + 1.4) {
                    H[i] = WATER_LEVEL;
                    topSet[i] = M.SAND;
                    flags[i] |= F_SHORE;
                }
            }
        }

        // River from the lake eastwards until the footprint ends; the last full cell is the lip.
        let lipX = LAKE.x;
        let lipZ = LAKE.z;
        const riverCells = [];
        for (let x = LAKE.x; x < w; x++) {
            const z0 = LAKE.z + Math.round(Math.sin(x * .4));
            if (!isIn(x, z0) || !isIn(x, z0 + 1)) break;
            lipX = x;
            lipZ = z0;
            for (let k = 0; k < 2; k++) {
                const i = x + (z0 + k) * w;
                if (flags[i] & F_LAKE) continue;
                H[i] = 2;
                water[i] = 1;
                topSet[i] = M.GRAVEL;
                flags[i] |= F_RIVER;
                riverCells.push(i);
            }
        }
        // Banks stay at least at water level so the river never hangs over low ground.
        for (const i of riverCells) {
            const x = i % w;
            const z = (i - x) / w;
            for (let nz = -1; nz <= 1; nz++) {
                for (let nx = -1; nx <= 1; nx++) {
                    if (!isIn(x + nx, z + nz)) continue;
                    const j = x + nx + (z + nz) * w;
                    if (water[j]) continue;
                    H[j] = Math.max(H[j], WATER_LEVEL);
                    flags[j] |= F_RIVER;
                }
            }
        }
        features.waterfall = { x: lipX + 1, y: WATER_LEVEL, z: lipZ, dx: 1, dz: 0, width: 2, bottomY: -SIZE.yBase };

        // Dock: first lake cell west of the lake centre on the dock row, deck three cells into the lake.
        let dockX = LAKE.x;
        while (dockX > 0 && water[(dockX - 1) + 34 * w]) dockX--;
        features.dock = { x: dockX, y: WATER_LEVEL, z: 34 };

        // Gravel paths (Bresenham, width 1) avoid water and buildings.
        const routes = [PATH_TO_DOCK.concat([[dockX - 1, 34]]), PATH_TO_MOUNTAIN];
        for (const route of routes) {
            for (let k = 0; k + 1 < route.length; k++) {
                let [x, z] = route[k];
                const [tx, tz] = route[k + 1];
                const sx = x < tx ? 1 : -1;
                const sz = z < tz ? 1 : -1;
                const ddx = Math.abs(tx - x);
                const ddz = -Math.abs(tz - z);
                let err = ddx + ddz;
                for (;;) {
                    markPath(s, x, z, isIn);
                    if (x === tx && z === tz) break;
                    const e2 = 2 * err;
                    if (e2 >= ddz) { err += ddz; x += sx; }
                    if (e2 <= ddx) { err += ddx; z += sz; }
                }
            }
        }

        // Wheat field marks (applied to grass tops when the columns are built).
        for (let z = 19; z <= 23; z++) {
            for (let x = 45; x <= 50; x++) if (isIn(x, z)) s.flags[x + z * w] |= F_FIELD;
        }
        return { isIn };
    }

    function markPath(s, x, z, isIn) {
        if (!isIn(x, z)) return;
        const i = x + z * s.w;
        if (s.water[i] || (s.flags[i] & (F_LAKE | F_RIVER | F_BUILD)) || s.H[i] >= 12) return;
        for (const hs of HOUSES) if (inRect(x, z, hs.x, hs.z, hs.x + hs.w - 1, hs.z + hs.d - 1)) return;
        if (inRect(x, z, WINDMILL.x0, WINDMILL.z0, WINDMILL.x0 + 2, WINDMILL.z0 + 2)) return;
        if (!(s.flags[i] & F_PATH)) s.pathCells.push([x, z]);
        s.flags[i] |= F_PATH;
        if (s.topSet[i] === 0) s.topSet[i] = M.GRAVEL;
    }

    // Steps 6-8: columns, underside, stalactites, crystals and roots.
    function buildColumns(s, plan) {
        const { w, d, g, rng, noise, inside, rr, H, flags, water, topSet, topMat, bottom, underDepth, features } = s;
        const yBase = SIZE.yBase;
        const vox = g.voxels;
        const plane = w * d;
        // Heights stay far below the grid top and the underside is clamped to the grid bottom, so column writes skip bounds checks.
        const idx = (col, y) => col + (y + yBase) * plane;
        const drop = (x, z, hh) => {
            let best = 0;
            if (plan.isIn(x + 1, z)) best = Math.max(best, hh - H[x + 1 + z * w]);
            if (plan.isIn(x - 1, z)) best = Math.max(best, hh - H[x - 1 + z * w]);
            if (plan.isIn(x, z + 1)) best = Math.max(best, hh - H[x + (z + 1) * w]);
            if (plan.isIn(x, z - 1)) best = Math.max(best, hh - H[x + (z - 1) * w]);
            return best;
        };
        for (let z = 0; z < d; z++) {
            for (let x = 0; x < w; x++) {
                const i = x + z * w;
                if (!inside[i]) continue;
                const hh = H[i];
                let top = topSet[i];
                if (!top) {
                    if (hh >= 16) top = M.SNOW;
                    else if (hh >= 12 || drop(x, z, hh) >= 3) top = hh >= 8 && hh < 12 && rng() < .3 ? M.MOSS : M.STONE;
                    else top = M.GRASS;
                }
                if ((flags[i] & F_FIELD) && top === M.GRASS) top = M.WHEAT;
                topMat[i] = top;
                const subsoil = top === M.STONE || top === M.SNOW || top === M.MOSS ? M.STONE : M.DIRT;
                vox[idx(i, hh)] = top;
                for (let y = hh - 1; y >= 0; y--) vox[idx(i, y)] = hh - y <= 3 ? subsoil : M.STONE;
                if (water[i]) for (let y = hh + 1; y <= WATER_LEVEL; y++) vox[idx(i, y)] = M.WATER;

                const u = clamp(Math.round(Math.pow(1 - rr[i], .75) * 24 + 3 * noise(x * .2 + 70, z * .2 + 70)), 2, 26);
                underDepth[i] = u;
                let low = -u;
                for (let y = -1; y >= -u; y--) {
                    let m = y > -3 ? M.DIRT : y > -.55 * u ? M.STONE : M.DEEP_STONE;
                    if (m !== M.DIRT && rng() < .02) m = M.GOLD_ORE;
                    vox[idx(i, y)] = m;
                }
                if (rng() < .06) {
                    const extra = 1 + Math.floor(rng() * 4);
                    for (let k = 1; k <= extra && -u - k >= -yBase; k++) {
                        vox[idx(i, -u - k)] = M.DEEP_STONE;
                        low = -u - k;
                    }
                }
                bottom[i] = low;
            }
        }

        // The waterfall column stays open down to the bottom of the grid.
        const wf = features.waterfall;
        for (let k = 0; k < wf.width; k++) {
            for (let y = wf.y; y >= -yBase; y--) g.put(wf.x, y, wf.z + k, M.AIR);
            if (wf.x < w && wf.z + k < d) inside[wf.x + (wf.z + k) * w] = 0;
        }

        // Crystals hang from the deepest third of the underside.
        const deep = [];
        for (let z = 0; z < d; z++) {
            for (let x = 0; x < w; x++) {
                const i = x + z * w;
                if (inside[i] && underDepth[i] >= 17 && bottom[i] > -yBase + 2) deep.push([x, z]);
            }
        }
        for (const [x, z] of shuffled(deep, rng)) {
            if (features.crystals.length >= 14) break;
            if (features.crystals.some(c => Math.abs(c[0] - x) < 3 && Math.abs(c[2] - z) < 3)) continue;
            const i = x + z * w;
            let y = bottom[i] - 1;
            g.put(x, y, z, M.CRYSTAL);
            if (rng() < .5) { y -= 1; g.put(x, y, z, M.CRYSTAL); }
            features.crystals.push([x, y, z]);
            bottom[i] = y;
        }

        // Roots dangle below the rim.
        const rim = [];
        for (let i = 0; i < s.cells; i++) if (inside[i] && rr[i] > .85) rim.push(i);
        const rooted = [];
        for (const i of shuffled(rim, rng)) {
            if (rooted.length >= 18) break;
            const x = i % w;
            const z = (i - x) / w;
            if (rooted.some(c => Math.abs(c[0] - x) < 3 && Math.abs(c[1] - z) < 3)) continue;
            rooted.push([x, z]);
            const len = 2 + Math.floor(rng() * 5);
            for (let k = 1; k <= len; k++) g.putIfAir(x, bottom[i] - k, z, M.ROOT);
        }
    }

    function foundation(s, x0, z0, x1, z1, topY) {
        const { g, w, H } = s;
        for (let z = z0; z <= z1; z++) {
            for (let x = x0; x <= x1; x++) {
                if (x < 0 || z < 0 || x >= w || z >= s.d) continue;
                const i = x + z * w;
                const from = s.inside[i] ? H[i] + 1 : 0;
                for (let y = from; y <= topY; y++) g.put(x, y, z, M.COBBLE);
            }
        }
    }

    // Step 10: three gabled houses on the plateau.
    function buildHouses(s) {
        const { g, features, flags, w } = s;
        HOUSES.forEach((hs, n) => {
            const x1 = hs.x + hs.w - 1;
            const z1 = hs.z + hs.d - 1;
            foundation(s, hs.x, hs.z, x1, z1, PLATEAU.top);
            for (let z = hs.z - 1; z <= z1 + 1; z++) for (let x = hs.x - 1; x <= x1 + 1; x++) if (x >= 0 && z >= 0 && x < w) flags[x + z * w] |= F_BUILD;
            for (let y = 5; y <= 7; y++) {
                for (let z = hs.z; z <= z1; z++) {
                    for (let x = hs.x; x <= x1; x++) {
                        const edgeX = x === hs.x || x === x1;
                        const edgeZ = z === hs.z || z === z1;
                        if (!edgeX && !edgeZ) continue;
                        g.put(x, y, z, edgeX && edgeZ ? M.OAK_LOG : M.PLASTER);
                    }
                }
            }
            const doorX = hs.x + (hs.w >> 1);
            g.put(doorX, 5, z1, M.PLANKS);
            g.put(doorX, 6, z1, M.PLANKS);
            const sideX = hs.x + 1 === doorX ? hs.x + hs.w - 2 : hs.x + 1;
            const windows = [
                [hs.x + (hs.w >> 1), hs.z],
                [sideX, z1],
                [hs.x, hs.z + (hs.d >> 1)],
                [x1, hs.z + (hs.d >> 1)]
            ];
            for (const [x, z] of windows) {
                g.put(x, 6, z, M.WINDOW);
                features.windows.push([x, 6, z]);
            }
            // Roof layers step inwards from both z sides; the end walls carry a plank gable.
            let ridge = 8;
            for (let k = 0; ; k++) {
                const zl = hs.z - 1 + k;
                const zr = z1 + 1 - k;
                if (zl > zr) break;
                ridge = 8 + k;
                for (let z = zl; z <= zr; z++) {
                    for (let x = hs.x; x <= x1; x++) {
                        const gable = (x === hs.x || x === x1) && z >= hs.z + k && z <= z1 - k;
                        g.put(x, 8 + k, z, gable ? M.PLANKS : hs.roof);
                    }
                }
            }
            if (n === 0) {
                for (let y = 8; y <= ridge + 2; y++) g.put(x1, y, hs.z, M.COBBLE);
                features.chimneys.push([x1, ridge + 3, hs.z]);
            }
        });
    }

    // Steps 11, 13: windmill tower and dock deck.
    function buildLandmarks(s) {
        const { g, w, H, features, flags } = s;
        const x0 = WINDMILL.x0;
        const z0 = WINDMILL.z0;
        let base = 0;
        for (let z = z0; z < z0 + 3; z++) for (let x = x0; x < x0 + 3; x++) base = Math.max(base, H[x + z * w]);
        for (let z = z0; z < z0 + 3; z++) {
            for (let x = x0; x < x0 + 3; x++) {
                for (let y = H[x + z * w] + 1; y <= base + 7; y++) g.put(x, y, z, M.COBBLE);
                g.put(x, base + 8, z, M.PLANKS);
                flags[x + z * w] |= F_BUILD;
            }
        }
        g.put(x0 + 1, base + 9, z0 + 1, M.PLANKS);
        features.windmill = { hub: [x0 + 1, base + 8, WINDMILL.hubZ], axis: [0, 0, 1] };

        const dock = features.dock;
        for (let k = 0; k < 3; k++) {
            g.put(dock.x + k, WATER_LEVEL, dock.z, M.PLANKS);
            flags[dock.x + k + dock.z * w] |= F_BUILD;
        }
    }

    // Step 14: six lanterns, a cobble post beside the path with a lantern on top.
    function buildLanterns(s) {
        const { g, w, d, features, flags, H, topMat, inside } = s;
        const cells = s.pathCells;
        if (!cells.length) return;
        for (let n = 0; n < 6; n++) {
            const start = Math.floor((n + .5) * cells.length / 6);
            let placed = false;
            for (let off = 0; off < cells.length && !placed; off++) {
                const [px, pz] = cells[(start + off) % cells.length];
                for (const [nx, nz] of [[0, 1], [0, -1], [1, 0], [-1, 0]]) {
                    const x = px + nx;
                    const z = pz + nz;
                    if (x < 0 || z < 0 || x >= w || z >= d) continue;
                    const i = x + z * w;
                    if (!inside[i] || (flags[i] & (F_PATH | F_LAKE | F_RIVER | F_SHORE | F_BUILD | F_FIELD | F_POST))) continue;
                    if (topMat[i] !== M.GRASS || H[i] >= 12) continue;
                    if (features.lanterns.some(l => Math.abs(l[0] - x) < 3 && Math.abs(l[2] - z) < 3)) continue;
                    g.put(x, H[i] + 1, z, M.COBBLE);
                    g.put(x, H[i] + 2, z, M.LANTERN);
                    flags[i] |= F_POST;
                    features.lanterns.push([x, H[i] + 2, z]);
                    placed = true;
                    break;
                }
            }
        }
    }

    // Step 15: pines on the mountain, oaks on low grass, birches around the lake.
    function buildTrees(s) {
        const { g, rng, w, d, inside, rr, H, flags, topMat, mountain, features } = s;
        const all = [];
        for (let i = 0; i < s.cells; i++) if (inside[i] && rr[i] <= .88) all.push(i);
        const order = shuffled(all, rng);
        const blocked = (x, z) => {
            for (let nz = -1; nz <= 1; nz++) {
                for (let nx = -1; nx <= 1; nx++) {
                    const ax = x + nx;
                    const az = z + nz;
                    if (ax < 0 || az < 0 || ax >= w || az >= d) return true;
                    if (flags[ax + az * w] & (F_LAKE | F_SHORE | F_RIVER | F_PATH | F_FIELD | F_BUILD | F_PLATEAU | F_POST)) return true;
                }
            }
            return false;
        };
        // Greedy spacing-4 packing over shuffled candidates; a few reshuffles reach the target on tight slopes.
        const plant = (kind, count, accept, grow) => {
            const candidates = order.filter(i => {
                const x = i % w;
                return accept(i, x, (i - x) / w) && !blocked(x, (i - x) / w);
            });
            const placed = features.trees;
            let best = [];
            for (let attempt = 0; attempt < 16 && best.length < count; attempt++) {
                const picks = [];
                for (const i of attempt ? shuffled(candidates, rng) : candidates) {
                    if (picks.length >= count) break;
                    const x = i % w;
                    const z = (i - x) / w;
                    if (placed.some(t => Math.hypot(t.x - x, t.z - z) < 4)) continue;
                    if (picks.some(j => Math.hypot(j % w - x, (j - j % w) / w - z) < 4)) continue;
                    picks.push(i);
                }
                if (picks.length > best.length) best = picks;
            }
            for (const i of best) {
                const x = i % w;
                const z = (i - x) / w;
                placed.push({ kind, x, y: grow(x, z, H[i]), z });
                flags[i] |= F_POST;
            }
        };
        const rootable = i => topMat[i] === M.GRASS || topMat[i] === M.MOSS;
        plant('pine', 12, (i) => rootable(i) && H[i] >= 6 && H[i] <= 13 && mountain[i] >= 1, (x, z, h) => {
            const top = h + 4 + Math.floor(rng() * 3);
            for (let y = h + 1; y <= top; y++) g.put(x, y, z, M.OAK_LOG);
            const radii = [2, 2, 1, 1, 0];
            radii.forEach((r, k) => {
                const y = top - 3 + k;
                for (let dz = -r; dz <= r; dz++) {
                    for (let dx = -r; dx <= r; dx++) {
                        if (r > 0 && dx * dx + dz * dz > r * r + 1) continue;
                        g.putIfAir(x + dx, y, z + dz, M.PINE_LEAVES);
                    }
                }
            });
            return top;
        });
        plant('oak', 9, (i) => topMat[i] === M.GRASS && H[i] <= 6 && H[i] >= 2 && mountain[i] < 3, (x, z, h) => {
            const top = h + 4 + Math.floor(rng() * 2);
            for (let y = h + 1; y <= top; y++) g.put(x, y, z, M.OAK_LOG);
            sphere(g, rng, x, top + 1, z, 2.6, .4, M.OAK_LEAVES);
            return top;
        });
        plant('birch', 4, (i, x, z) => {
            const dist = Math.hypot(x - LAKE.x, z - LAKE.z);
            return topMat[i] === M.GRASS && dist >= 6 && dist <= 9 && H[i] <= 6;
        }, (x, z, h) => {
            const top = h + 5 + Math.floor(rng() * 2);
            for (let y = h + 1; y <= top; y++) g.put(x, y, z, M.BIRCH_LOG);
            sphere(g, rng, x, top + 1, z, 2, 0, M.BIRCH_LEAVES);
            return top;
        });
    }

    function sphere(g, rng, cx, cy, cz, radius, wobble, m) {
        const reach = Math.ceil(radius + wobble);
        for (let dy = -reach; dy <= reach; dy++) {
            for (let dz = -reach; dz <= reach; dz++) {
                for (let dx = -reach; dx <= reach; dx++) {
                    const r = radius + (wobble ? (rng() * 2 - 1) * wobble : 0);
                    if (dx * dx + dy * dy + dz * dz <= r * r) g.putIfAir(cx + dx, cy + dy, cz + dz, m);
                }
            }
        }
    }

    // Step 16: flowers and grass tufts on free grass; also picks the meadow rectangle for fireflies.
    function buildDecorations(s) {
        const { g, rng, w, inside, H, flags, topMat, decorations, features } = s;
        const free = [];
        for (let i = 0; i < s.cells; i++) {
            if (!inside[i] || topMat[i] !== M.GRASS || H[i] >= 12) continue;
            if (flags[i] & (F_LAKE | F_SHORE | F_RIVER | F_PATH | F_FIELD | F_BUILD | F_POST)) continue;
            const x = i % w;
            const z = (i - x) / w;
            if (g.get(x, H[i] + 1, z) !== M.AIR) continue;
            free.push(i);
        }
        const order = shuffled(free, rng);
        const place = (i, size, color) => {
            const x = i % w;
            const z = (i - x) / w;
            decorations.push({ x: x + (rng() - .5) * .5, y: H[i] + 1, z: z + (rng() - .5) * .5, size, color, sway: 1 });
        };
        for (let k = 0; k < order.length && k < 190; k++) {
            if (k < 70) place(order[k], .32, FLOWER_COLORS[k % FLOWER_COLORS.length]);
            else place(order[k], .22, '#5d9c3c');
        }
        // Meadow: the 14 x 10 window holding the most free grass cells.
        const grass = new Uint8Array(s.cells);
        for (const i of free) grass[i] = 1;
        let best = -1;
        for (let z0 = 0; z0 + 9 < s.d; z0 += 2) {
            for (let x0 = 0; x0 + 13 < w; x0 += 2) {
                let n = 0;
                for (let z = z0; z < z0 + 10; z++) for (let x = x0; x < x0 + 14; x++) n += grass[x + z * w];
                if (n > best) { best = n; features.meadow = { x0, z0, x1: x0 + 13, z1: z0 + 9 }; }
            }
        }
    }

    function buildIslets(rng) {
        // A: grass disc with a cone underneath and one small oak.
        const a = makeGrid(11, 11, 14, 8);
        for (let z = 0; z < 11; z++) {
            for (let x = 0; x < 11; x++) {
                const dist = Math.hypot(x - 5, z - 5);
                if (dist > 4.2) continue;
                a.put(x, 0, z, M.GRASS);
                const depth = clamp(Math.round(6 * (1 - dist / 4.6)), 1, 6);
                for (let k = 1; k <= depth; k++) a.put(x, -k, z, k <= 2 ? M.DIRT : k >= depth - 1 && depth >= 4 ? M.DEEP_STONE : M.STONE);
            }
        }
        for (let y = 1; y <= 3; y++) a.put(5, y, 5, M.OAK_LOG);
        sphere(a, rng, 5, 4, 5, 1.6, .2, M.OAK_LEAVES);

        // B: stone disc with a crystal cluster and a small cobble gate.
        const b = makeGrid(9, 9, 12, 6);
        for (let z = 0; z < 9; z++) {
            for (let x = 0; x < 9; x++) {
                const dist = Math.hypot(x - 4, z - 4);
                if (dist > 3.2) continue;
                b.put(x, 0, z, M.STONE);
                const depth = clamp(Math.round(5 * (1 - dist / 3.6)), 1, 5);
                for (let k = 1; k <= depth; k++) b.put(x, -k, z, k >= depth - 1 && depth >= 3 ? M.DEEP_STONE : M.STONE);
            }
        }
        const cluster = [[4, 1, 4], [4, 2, 4], [3, 1, 4], [5, 1, 4]];
        for (const [x, y, z] of cluster) b.put(x, y, z, M.CRYSTAL);
        for (let y = 1; y <= 3; y++) { b.put(2, y, 6, M.COBBLE); b.put(6, y, 6, M.COBBLE); }
        for (let x = 2; x <= 6; x++) b.put(x, 4, 6, M.COBBLE);

        const finish = (grid, offset, phase, trees, extra) => Object.assign({
            w: grid.w, d: grid.d, h: grid.h, yBase: grid.yBase, voxels: grid.voxels,
            solidCount: countSolid(grid.voxels), offset, phase, trees
        }, extra);
        return [
            finish(a, [-30, 8, -14], 0, 1),
            finish(b, [27, -6, 18], 2.1, 0, { crystals: cluster.map(c => c.slice()) })
        ];
    }

    function generate(seed) {
        const s = newState(seed === undefined ? 0x5eed1e : seed >>> 0);
        const plan = planTerrain(s);
        buildColumns(s, plan);
        buildHouses(s);
        buildLandmarks(s);
        buildLanterns(s);
        buildTrees(s);
        buildDecorations(s);
        const islets = buildIslets(s.rng);
        const g = s.g;
        return {
            seed: s.seed, w: g.w, d: g.d, h: g.h, yBase: g.yBase, voxels: g.voxels, solidCount: countSolid(g.voxels),
            features: s.features, decorations: s.decorations, islets
        };
    }

    window.AuraVoxelIsland = Object.freeze({ generate, MATERIAL, MATERIAL_INFO, SIZE });
})();
