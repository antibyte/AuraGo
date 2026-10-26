(function () {
    'use strict';

    const GROUPS = ['solid', 'foliage', 'water', 'glow'];
    const WATER = 2;
    const GLOW = 3;
    // Face order: +x, -x, +y, -y, +z, -z. Quad corners are origin + u*U + v*V with U x V = normal
    // (counter-clockwise from outside); V points up on side faces.
    const FN = new Int8Array([1, 0, 0, -1, 0, 0, 0, 1, 0, 0, -1, 0, 0, 0, 1, 0, 0, -1]);
    const FO = new Int8Array([1, 0, 1, 0, 0, 0, 0, 1, 1, 0, 0, 0, 0, 0, 1, 1, 0, 0]);
    const FU = new Int8Array([0, 0, -1, 0, 0, 1, 1, 0, 0, 1, 0, 0, 1, 0, 0, -1, 0, 0]);
    const FV = new Int8Array([0, 1, 0, 0, 1, 0, 0, 0, -1, 0, 0, 1, 0, 1, 0, 0, 1, 0]);
    const CU = [0, 1, 1, 0];
    const CV = [0, 0, 1, 1];

    function rgb(hex) {
        const n = parseInt(hex.slice(1), 16);
        return [(n >> 16 & 255) / 255, (n >> 8 & 255) / 255, (n & 255) / 255];
    }

    function hash3(x, y, z) {
        let h = Math.imul(x, 374761393) + Math.imul(y, 668265263) + Math.imul(z, 1274126177);
        h = Math.imul(h ^ (h >>> 13), 1103515245);
        return ((h ^ (h >>> 16)) >>> 0) / 4294967296;
    }

    function allocate(faces, withEmissive) {
        return {
            faces: 0,
            positions: new Float32Array(faces * 12),
            normals: new Float32Array(faces * 12),
            colors: new Float32Array(faces * 12),
            ao: new Float32Array(faces * 4),
            uvs: new Float32Array(faces * 8),
            seeds: new Float32Array(faces * 4),
            indices: new Uint32Array(faces * 6),
            emissive: withEmissive ? new Float32Array(faces * 12) : null
        };
    }

    // Face-culled mesher with per-vertex ambient occlusion; water is a separate group so it can be translucent.
    function mesh(grid, info, options) {
        const opts = options || {};
        const { w, d, h, yBase, voxels } = grid;
        const origin = opts.origin || [w / 2, yBase, d / 2];
        const ox = origin[0];
        const oy = origin[1];
        const oz = origin[2];
        const plane = w * d;

        const groupOf = new Int8Array(256).fill(-1);
        const occluder = new Uint8Array(256);
        const base = new Float32Array(256 * 3);
        const jitter = new Float32Array(256);
        const glow = new Float32Array(256 * 3);
        let grassId = -1;
        let dirtId = -1;
        for (let m = 1; m < info.length && m < 256; m++) {
            const e = info[m];
            if (!e) continue;
            groupOf[m] = GROUPS.indexOf(e.group);
            occluder[m] = e.group === 'water' ? 0 : 1;
            base.set(rgb(e.color), m * 3);
            jitter[m] = e.jitter || 0;
            if (e.glow) glow.set(rgb(e.glow), m * 3);
            if (e.name === 'grass') grassId = m;
            if (e.name === 'dirt') dirtId = m;
        }

        // Pass 1: which faces are visible (bit f of mask), counted per group so buffers are allocated exactly.
        const masks = new Uint8Array(voxels.length);
        const listed = [];
        const counts = [0, 0, 0, 0];
        for (let y = 0, i = 0; y < h; y++) {
            for (let z = 0; z < d; z++) {
                for (let x = 0; x < w; x++, i++) {
                    const m = voxels[i];
                    if (m === 0) continue;
                    const gi = groupOf[m];
                    if (gi < 0) continue;
                    // Water only faces air; others face air, water, or a different group.
                    const solidLike = gi !== WATER;
                    let mask = 0;
                    let n = x < w - 1 ? voxels[i + 1] : 0;
                    if (n === 0 || (solidLike && groupOf[n] !== gi)) mask |= 1;
                    n = x > 0 ? voxels[i - 1] : 0;
                    if (n === 0 || (solidLike && groupOf[n] !== gi)) mask |= 2;
                    n = y < h - 1 ? voxels[i + plane] : 0;
                    if (n === 0 || (solidLike && groupOf[n] !== gi)) mask |= 4;
                    n = y > 0 ? voxels[i - plane] : 0;
                    if (n === 0 || (solidLike && groupOf[n] !== gi)) mask |= 8;
                    n = z < d - 1 ? voxels[i + w] : 0;
                    if (n === 0 || (solidLike && groupOf[n] !== gi)) mask |= 16;
                    n = z > 0 ? voxels[i - w] : 0;
                    if (n === 0 || (solidLike && groupOf[n] !== gi)) mask |= 32;
                    if (mask === 0) continue;
                    masks[i] = mask;
                    listed.push(i);
                    for (let bits = mask; bits; bits &= bits - 1) counts[gi]++;
                }
            }
        }
        const builders = GROUPS.map((name, gi) => allocate(counts[gi], gi === GLOW));

        const occ = (x, y, z) => (x < 0 || y < 0 || z < 0 || x >= w || y >= h || z >= d ? 0 : occluder[voxels[x + z * w + y * plane]]);
        const around = new Uint8Array(9);
        const ao4 = [0, 0, 0, 0];
        const color = [0, 0, 0];

        // Pass 2: emit quads.
        for (let li = 0; li < listed.length; li++) {
            const i = listed[li];
            const m = voxels[i];
            const mask = masks[i];
            const b = builders[groupOf[m]];
            const y = (i / plane) | 0;
            const rest = i - y * plane;
            const z = (rest / w) | 0;
            const x = rest - z * w;
            const seed = hash3(x, y, z);
            const factor = 1 + (seed - .5) * 2 * jitter[m];
            const bm = m * 3;
            for (let f = 0; f < 6; f++) {
                if (!(mask & (1 << f))) continue;
                const f3 = f * 3;
                const fx = x + FN[f3];
                const fy = y + FN[f3 + 1];
                const fz = z + FN[f3 + 2];
                if (m === grassId && dirtId > 0 && f !== 2) {
                    const dm = dirtId * 3;
                    const t = f === 3 ? 1 : .65;
                    for (let c = 0; c < 3; c++) color[c] = Math.min(1, (base[bm + c] + (base[dm + c] - base[bm + c]) * t) * factor);
                } else {
                    for (let c = 0; c < 3; c++) color[c] = Math.min(1, base[bm + c] * factor);
                }

                const ux = FU[f3];
                const uy = FU[f3 + 1];
                const uz = FU[f3 + 2];
                const vx = FV[f3];
                const vy = FV[f3 + 1];
                const vz = FV[f3 + 2];
                for (let a = -1; a <= 1; a++) {
                    for (let c = -1; c <= 1; c++) {
                        around[(a + 1) * 3 + c + 1] = a === 0 && c === 0 ? 0 : occ(fx + a * ux + c * vx, fy + a * uy + c * vy, fz + a * uz + c * vz);
                    }
                }

                const fi = b.faces;
                for (let k = 0; k < 4; k++) {
                    const cu = CU[k];
                    const cv = CV[k];
                    const su = cu ? 2 : 0;
                    const sv = cv ? 2 : 0;
                    const s1 = around[su * 3 + 1];
                    const s2 = around[3 + sv];
                    const cc = around[su * 3 + sv];
                    const ao = 1 - (s1 && s2 ? 3 : s1 + s2 + cc) * .25;
                    ao4[k] = ao;

                    const vi = fi * 4 + k;
                    const p = vi * 3;
                    b.positions[p] = x + FO[f3] + cu * ux + cv * vx - ox;
                    b.positions[p + 1] = y + FO[f3 + 1] + cu * uy + cv * vy - oy;
                    b.positions[p + 2] = z + FO[f3 + 2] + cu * uz + cv * vz - oz;
                    b.normals[p] = FN[f3];
                    b.normals[p + 1] = FN[f3 + 1];
                    b.normals[p + 2] = FN[f3 + 2];
                    b.colors[p] = color[0];
                    b.colors[p + 1] = color[1];
                    b.colors[p + 2] = color[2];
                    b.ao[vi] = ao;
                    b.uvs[vi * 2] = cu;
                    b.uvs[vi * 2 + 1] = cv;
                    b.seeds[vi] = seed;
                    if (b.emissive) {
                        b.emissive[p] = glow[bm];
                        b.emissive[p + 1] = glow[bm + 1];
                        b.emissive[p + 2] = glow[bm + 2];
                    }
                }
                const vb = fi * 4;
                const ii = fi * 6;
                if (ao4[0] + ao4[2] < ao4[1] + ao4[3]) {
                    b.indices[ii] = vb; b.indices[ii + 1] = vb + 1; b.indices[ii + 2] = vb + 3;
                    b.indices[ii + 3] = vb + 1; b.indices[ii + 4] = vb + 2; b.indices[ii + 5] = vb + 3;
                } else {
                    b.indices[ii] = vb; b.indices[ii + 1] = vb + 1; b.indices[ii + 2] = vb + 2;
                    b.indices[ii + 3] = vb; b.indices[ii + 4] = vb + 2; b.indices[ii + 5] = vb + 3;
                }
                b.faces++;
            }
        }

        const groups = {};
        let faceCount = 0;
        GROUPS.forEach((name, gi) => {
            const b = builders[gi];
            groups[name] = {
                positions: b.positions, normals: b.normals, colors: b.colors, ao: b.ao, uvs: b.uvs,
                seeds: b.seeds, indices: b.indices, faceCount: b.faces
            };
            if (b.emissive) groups[name].emissive = b.emissive;
            faceCount += b.faces;
        });
        return { groups, faceCount, vertexCount: faceCount * 4 };
    }

    window.AuraVoxelMesher = Object.freeze({ mesh });
})();
