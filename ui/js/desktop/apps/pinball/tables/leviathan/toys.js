// Deep Sea Leviathan 3D toys (plan 6.2 / T22): the sea monster rising out of a coiled mound in the middle of the tentacle helix (scaled
// neck, horned skull with frills and teeth, hinged jaw, glowing eyes whose pupils follow the nearest ball, breathing), the broken
// wooden hull round the ship scoop with ribs, a mast stump and a lantern, swaying kelp, rising bubbles and drifting plankton.
// buildToys(THREE, kit, layout, {tier, low}) -> {group, update(dt, t, snap, toyState, lamps, events?), dispose}. Local frame is the
// table root: X = x, Y = h, Z = -y (the player looks from +Z, so "forward" for the head is +Z).
// Draw calls: high 12 (neck, skull, jaw, eyes, pupils, head glow, hull, lights, kelp, glow, bubbles, plankton), low 10.
import { rngNext } from '../../config.js';

const HEAD = [0.105, 0.880], SHIP = [0.100, 0.780], PEARL = [-0.082, 0.6102];
const HEAD_H = 0.184;
const TEAL = '#43e3d5', GOLD = '#dcad63';

function canvas(w, h) { const c = document.createElement('canvas'); c.width = w; c.height = h; return c; }
function colorOf(THREE, v, fallback) { try { return new THREE.Color(v || fallback); } catch { return new THREE.Color(fallback); } }
const isLowTier = (kit, opts) => {
    if (opts && typeof opts.low === 'boolean') return opts.low;
    if (typeof opts === 'string') return opts === 'low';
    return !!(kit && kit.W && kit.W <= 1024);
};
const clamp = (v, a, b) => Math.max(a, Math.min(b, v));
const finite = (v, d) => (Number.isFinite(v) ? v : d);

// ---- procedural textures ----
function scaleTexture(THREE) {
    const S = 128, c = canvas(S, S), g = c.getContext('2d'), n = 8, w = S / n;
    g.fillStyle = '#6a6a6a'; g.fillRect(0, 0, S, S);
    for (let row = -1; row <= n; row++) {
        for (let col = -1; col <= n; col++) {
            const x = (col + (row & 1 ? 0.5 : 0)) * w, y = row * w * 0.72;
            const gr = g.createRadialGradient(x, y - w * 0.2, 0, x, y, w * 0.62);
            gr.addColorStop(0, '#f2f2f2'); gr.addColorStop(0.65, '#bdbdbd'); gr.addColorStop(1, '#8a8a8a');
            g.fillStyle = gr; g.beginPath(); g.moveTo(x - w * 0.5, y - w * 0.1); g.quadraticCurveTo(x - w * 0.5, y + w * 0.62, x, y + w * 0.68);
            g.quadraticCurveTo(x + w * 0.5, y + w * 0.62, x + w * 0.5, y - w * 0.1); g.quadraticCurveTo(x, y - w * 0.32, x - w * 0.5, y - w * 0.1); g.fill();
            g.strokeStyle = 'rgba(20,20,20,.65)'; g.lineWidth = 1.6; g.stroke();
            g.strokeStyle = 'rgba(255,255,255,.35)'; g.lineWidth = 1; g.beginPath(); g.moveTo(x - w * 0.34, y + w * 0.05); g.quadraticCurveTo(x, y - w * 0.18, x + w * 0.34, y + w * 0.05); g.stroke();
        }
    }
    const t = new THREE.CanvasTexture(c); t.colorSpace = THREE.SRGBColorSpace; t.wrapS = t.wrapT = THREE.RepeatWrapping; t.anisotropy = 4;
    return t;
}

function woodTexture(THREE) {
    const c = canvas(128, 128), g = c.getContext('2d'), rng = { seed: 0x600D };
    g.fillStyle = '#a89577'; g.fillRect(0, 0, 128, 128);
    for (let i = 0; i < 46; i++) {
        const y = rngNext(rng) * 128, v = 90 + rngNext(rng) * 90;
        g.strokeStyle = `rgba(${v * 0.55},${v * 0.5},${v * 0.42},${0.18 + rngNext(rng) * 0.25})`; g.lineWidth = 0.6 + rngNext(rng) * 1.6;
        g.beginPath(); g.moveTo(0, y); g.bezierCurveTo(40, y + (rngNext(rng) - 0.5) * 8, 90, y + (rngNext(rng) - 0.5) * 8, 128, y + (rngNext(rng) - 0.5) * 6); g.stroke();
    }
    for (let i = 0; i < 40; i++) { g.fillStyle = `rgba(80,190,170,${0.05 + rngNext(rng) * 0.12})`; g.beginPath(); g.arc(rngNext(rng) * 128, rngNext(rng) * 128, 2 + rngNext(rng) * 7, 0, 7); g.fill(); }
    g.fillStyle = 'rgba(0,0,0,.3)'; for (let i = 0; i < 6; i++) g.fillRect(rngNext(rng) * 128, 0, 1.4, 128);
    const t = new THREE.CanvasTexture(c); t.colorSpace = THREE.SRGBColorSpace; t.wrapS = t.wrapT = THREE.RepeatWrapping; t.anisotropy = 4;
    return t;
}

function glowTexture(THREE) {
    const c = canvas(64, 64), g = c.getContext('2d');
    const gr = g.createRadialGradient(32, 32, 0, 32, 32, 32);
    gr.addColorStop(0, 'rgba(255,255,255,1)'); gr.addColorStop(0.22, 'rgba(255,255,255,.6)'); gr.addColorStop(0.55, 'rgba(255,255,255,.14)'); gr.addColorStop(1, 'rgba(255,255,255,0)');
    g.fillStyle = gr; g.fillRect(0, 0, 64, 64);
    const t = new THREE.CanvasTexture(c); t.colorSpace = THREE.SRGBColorSpace;
    return t;
}

function bubbleTexture(THREE) {
    const c = canvas(64, 64), g = c.getContext('2d');
    const gr = g.createRadialGradient(32, 32, 14, 32, 32, 30);
    gr.addColorStop(0, 'rgba(150,230,255,.06)'); gr.addColorStop(0.8, 'rgba(170,240,255,.45)'); gr.addColorStop(1, 'rgba(200,250,255,0)');
    g.fillStyle = gr; g.beginPath(); g.arc(32, 32, 30, 0, 7); g.fill();
    g.strokeStyle = 'rgba(230,255,255,.95)'; g.lineWidth = 2.2; g.beginPath(); g.arc(32, 32, 26, 0, 7); g.stroke();
    g.fillStyle = 'rgba(255,255,255,.95)'; g.beginPath(); g.ellipse(23, 22, 6, 3.4, -0.7, 0, 7); g.fill();
    const t = new THREE.CanvasTexture(c); t.colorSpace = THREE.SRGBColorSpace;
    return t;
}

// ---- geometry helpers (all indexed or plain, attributes position/normal/uv/color) ----
function tint(THREE, g, hex, uvScale = 1) {
    const col = new THREE.Color(hex), n = g.attributes.position.count, a = new Float32Array(n * 3);
    for (let i = 0; i < n; i++) { a[i * 3] = col.r; a[i * 3 + 1] = col.g; a[i * 3 + 2] = col.b; }
    g.setAttribute('color', new THREE.BufferAttribute(a, 3));
    if (uvScale !== 1 && g.attributes.uv) { const uv = g.attributes.uv; for (let i = 0; i < uv.count; i++) uv.setXY(i, uv.getX(i) * uvScale, uv.getY(i) * uvScale); }
    return g;
}
function mergeGeos(THREE, list) {
    const pos = [], nor = [], uvs = [], col = [], idx = [];
    let off = 0;
    for (const g of list) {
        const p = g.attributes.position, n = g.attributes.normal, u = g.attributes.uv, c = g.attributes.color, ix = g.index;
        for (let k = 0; k < p.count; k++) {
            pos.push(p.getX(k), p.getY(k), p.getZ(k)); nor.push(n.getX(k), n.getY(k), n.getZ(k)); uvs.push(u ? u.getX(k) : 0, u ? u.getY(k) : 0);
            col.push(c ? c.getX(k) : 1, c ? c.getY(k) : 1, c ? c.getZ(k) : 1);
        }
        if (ix) for (let k = 0; k < ix.count; k++) idx.push(ix.getX(k) + off); else for (let k = 0; k < p.count; k++) idx.push(k + off);
        off += p.count; g.dispose();
    }
    const m = new THREE.BufferGeometry();
    m.setAttribute('position', new THREE.Float32BufferAttribute(pos, 3)); m.setAttribute('normal', new THREE.Float32BufferAttribute(nor, 3));
    m.setAttribute('uv', new THREE.Float32BufferAttribute(uvs, 2)); m.setAttribute('color', new THREE.Float32BufferAttribute(col, 3));
    m.setIndex(idx);
    return m;
}

// Piecewise-linear lookup in a [[s, value], ...] table.
function table(tab, s) {
    if (s <= tab[0][0]) return tab[0][1];
    for (let i = 1; i < tab.length; i++) if (s <= tab[i][0]) { const a = tab[i - 1], b = tab[i], k = (s - a[0]) / (b[0] - a[0] || 1); return a[1] + (b[1] - a[1]) * k; }
    return tab[tab.length - 1][1];
}

// Sweep a closed ring along a point path. fn(s, cos, sin, i) -> [dx, dy] offset in the ring plane (B = T x ref, N = B x T).
function sweep(THREE, pts, n, fn, ref = [0, 0, 1], uRep = 1, vRep = 1) {
    const N = pts.length, pos = [], uv = [], idx = [];
    const T = new THREE.Vector3(), B = new THREE.Vector3(), Nn = new THREE.Vector3(), R = new THREE.Vector3(ref[0], ref[1], ref[2]);
    for (let i = 0; i < N; i++) {
        const a = pts[Math.max(0, i - 1)], b = pts[Math.min(N - 1, i + 1)];
        T.set(b[0] - a[0], b[1] - a[1], b[2] - a[2]).normalize();
        B.crossVectors(T, R); if (B.lengthSq() < 1e-6) B.crossVectors(T, new THREE.Vector3(1, 0, 0));
        B.normalize(); Nn.crossVectors(B, T).normalize();
        for (let j = 0; j <= n; j++) {
            const ang = j / n * Math.PI * 2, c = Math.cos(ang), s = Math.sin(ang), d = fn(i / (N - 1), c, s, i);
            pos.push(pts[i][0] + B.x * d[0] + Nn.x * d[1], pts[i][1] + B.y * d[0] + Nn.y * d[1], pts[i][2] + B.z * d[0] + Nn.z * d[1]);
            uv.push(j / n * uRep, i / (N - 1) * vRep);
        }
    }
    for (let i = 0; i < N - 1; i++) for (let j = 0; j < n; j++) {
        const a = i * (n + 1) + j, b = (i + 1) * (n + 1) + j;
        idx.push(a, b, a + 1, a + 1, b, b + 1);
    }
    const g = new THREE.BufferGeometry();
    g.setAttribute('position', new THREE.Float32BufferAttribute(pos, 3)); g.setAttribute('uv', new THREE.Float32BufferAttribute(uv, 2));
    g.setIndex(idx); g.computeVertexNormals();
    return g;
}

// Smooth spline through control points, resampled to `count` points.
function spline(THREE, ctrl, count) {
    return new THREE.CatmullRomCurve3(ctrl.map(p => new THREE.Vector3(p[0], p[1], p[2]))).getPoints(count - 1).map(v => [v.x, v.y, v.z]);
}

// Cone from a base point along a direction.
function spike(THREE, base, dir, r, len, seg = 5) {
    const g = new THREE.ConeGeometry(r, len, seg, 1); g.translate(0, len / 2, 0);
    const q = new THREE.Quaternion().setFromUnitVectors(new THREE.Vector3(0, 1, 0), new THREE.Vector3(dir[0], dir[1], dir[2]).normalize());
    g.applyQuaternion(q); g.translate(base[0], base[1], base[2]);
    return g;
}

// Per-vertex shading by normal and position: fn(normal, position, i) -> THREE.Color-compatible hex or Color.
function shade(THREE, g, fn) {
    const n = g.attributes.position.count, nrm = g.attributes.normal, pos = g.attributes.position, a = new Float32Array(n * 3), v = new THREE.Vector3(), p = new THREE.Vector3(), c = new THREE.Color();
    for (let i = 0; i < n; i++) {
        v.set(nrm.getX(i), nrm.getY(i), nrm.getZ(i)); p.set(pos.getX(i), pos.getY(i), pos.getZ(i));
        const r = fn(v, p, i); c.set(r); a[i * 3] = c.r; a[i * 3 + 1] = c.g; a[i * 3 + 2] = c.b;
    }
    g.setAttribute('color', new THREE.BufferAttribute(a, 3));
    return g;
}
const mix = (THREE, a, b, k) => new THREE.Color(a).lerp(new THREE.Color(b), clamp(k, 0, 1));

export function buildToys(THREE, kit, layout, opts) {
    const low = isLowTier(kit, opts);
    const group = new THREE.Group(); group.name = 'leviathanToys';
    const disposables = [];
    const own = o => { disposables.push(o); return o; };
    const find = id => ((layout && layout.toys) || []).find(t => t.id === id);
    const headToy = find('head') || { x: HEAD[0], y: HEAD[1] }, shipToy = find('hull') || { x: SHIP[0], y: SHIP[1], rot: 0.6 };
    const hx = headToy.x, hy = headToy.y, sx = shipToy.x, sy = shipToy.y;
    const rng = { seed: 0x1E71A7 };
    const seg = low ? 8 : 14;

    const scaleTex = own(scaleTexture(THREE)), woodTex = own(woodTexture(THREE)), glowTex = own(glowTexture(THREE));
    const creatureMat = own(new THREE.MeshStandardMaterial({ map: scaleTex, vertexColors: true, roughness: 0.42, metalness: 0.28, emissive: '#0a4a56', emissiveIntensity: 0.55, side: THREE.DoubleSide }));
    const boneMat = own(new THREE.MeshStandardMaterial({ vertexColors: true, roughness: 0.35, metalness: 0.1, emissive: '#2a2418', emissiveIntensity: 0.5, side: THREE.DoubleSide }));
    void boneMat;

    // ---- neck, coils and rocky mound (static, breathes by scaling) ----
    const neckParts = [];
    const neckPath = spline(THREE, [[0, 0.004, 0.002], [0, 0.050, -0.008], [0, 0.100, -0.010], [0, 0.140, -0.002], [0, 0.166, 0.008], [0, 0.182, 0.010]], low ? 22 : 40);
    const neckR = s => 0.0255 - 0.0085 * Math.min(1, s * 1.15) + (s > 0.9 ? 0.0035 * (s - 0.9) / 0.1 : 0);
    const neck = sweep(THREE, neckPath, seg, (s, c, sn, i) => {
        const ring = 1 + 0.085 * Math.max(0, Math.sin(i * 1.55)) ** 2;
        const r = neckR(s) * ring; return [c * r, sn * r * 0.96];
    }, [0, 0, 1], 2, Math.max(2, Math.round(0.19 / 0.0085 / 8)));
    shade(THREE, neck, (nm, p, i) => {
        const front = clamp(nm.z * 1.4 + 0.1, 0, 1), n = ((i * 2654435761) >>> 0) / 4294967296;
        return mix(THREE, mix(THREE, '#134f66', '#1f8c97', n * 0.8), '#a6ece0', front * 0.85);
    });
    neckParts.push(neck);
    const mound = new THREE.LatheGeometry([[0.001, 0.0], [0.038, 0.0], [0.039, 0.005], [0.033, 0.013], [0.027, 0.024], [0.0235, 0.040], [0.021, 0.055]].map(p => new THREE.Vector2(p[0], p[1])), low ? 12 : 22);
    { const p = mound.attributes.position; for (let i = 0; i < p.count; i++) { const k = 1 + (rngNext(rng) - 0.5) * 0.10 * (p.getY(i) < 0.03 ? 1 : 0.4); p.setX(i, p.getX(i) * k); p.setZ(i, p.getZ(i) * k); } mound.computeVertexNormals(); }
    shade(THREE, mound, (nm, p) => mix(THREE, '#1a2f38', '#4a6a70', rngNext(rng) * 0.8 + p.y * 3));
    neckParts.push(mound);
    for (const [r, h, tr, col] of [[0.0305, 0.011, 0.0068, '#1c7080'], [0.0255, 0.028, 0.0058, '#23909a'], [0.0235, 0.043, 0.0050, '#1c7080']]) {
        const t = new THREE.TorusGeometry(r, tr, 8, low ? 20 : 34); t.rotateX(Math.PI / 2); t.translate(0, h, 0);
        const uv = t.attributes.uv; for (let i = 0; i < uv.count; i++) uv.setXY(i, uv.getX(i) * 6, uv.getY(i) * 2);
        shade(THREE, t, (nm, p) => mix(THREE, col, '#a6ece0', clamp(nm.y * 0.5 + 0.2, 0, 1) * 0.6));
        neckParts.push(t);
    }
    // jagged rocks and coral spikes round the mound
    for (let i = 0; i < (low ? 8 : 14); i++) {
        const a = i / (low ? 8 : 14) * Math.PI * 2 + rngNext(rng) * 0.3, r = 0.034 + rngNext(rng) * 0.004;
        neckParts.push(shade(THREE, spike(THREE, [Math.cos(a) * r, 0, Math.sin(a) * r], [Math.cos(a) * 0.5, 1, Math.sin(a) * 0.5], 0.0035 + rngNext(rng) * 0.003, 0.012 + rngNext(rng) * 0.016, 5), (nm, p) => mix(THREE, '#27414a', '#6f9aa0', p.y * 28)));
    }
    // dorsal spines down the back of the neck, side frills lower down (small: the helix ramp wraps the neck)
    const backAt = s => { const i = Math.round(s * (neckPath.length - 1)); return neckPath[i]; };
    for (let i = 0; i < (low ? 7 : 11); i++) {
        const s = 0.12 + i * 0.082, p = backAt(Math.min(0.99, s)), r = neckR(s) * 0.98, len = 0.016 - i * 0.0009;
        neckParts.push(shade(THREE, spike(THREE, [p[0], p[1], p[2] - r], [0, 0.55, -1], 0.0036, len, 4), () => (i % 2 ? GOLD : '#f0d393')));
    }
    const neckGeo = own(mergeGeos(THREE, neckParts.map(g => (g.attributes.color ? g : tint(THREE, g, '#ffffff')))));
    const neckMesh = new THREE.Mesh(neckGeo, creatureMat); neckMesh.name = 'leviathanNeck'; neckMesh.frustumCulled = false; neckMesh.position.set(hx, 0, -hy);
    group.add(neckMesh);

    // ---- skull (yaws, pitches and tracks; local frame: +Z forward, +Y up, origin mid-skull) ----
    const headTop = new THREE.Group(); headTop.position.set(hx, HEAD_H, -hy); headTop.name = 'leviathanHead'; group.add(headTop);
    const RX = [[-0.032, 0.006], [-0.026, 0.015], [-0.013, 0.0215], [0.0, 0.0215], [0.012, 0.0175], [0.022, 0.0125], [0.031, 0.0090], [0.036, 0.0050]];
    const RY = [[-0.032, 0.005], [-0.026, 0.012], [-0.013, 0.0175], [0.0, 0.0175], [0.012, 0.0130], [0.022, 0.0090], [0.031, 0.0068], [0.036, 0.0035]];
    const skullPath = []; for (let i = 0; i < (low ? 16 : 26); i++) { const z = -0.032 + 0.068 * i / ((low ? 16 : 26) - 1); skullPath.push([0, -0.0006 * Math.sin(i * 0.4), z]); }
    const skull = sweep(THREE, skullPath, seg, (s, c, sn, i) => {
        const z = -0.032 + 0.068 * s, rx = table(RX, z), ry = table(RY, z), up = sn > 0;
        const bump = up ? 1 + 0.12 * Math.exp(-((z + 0.010) ** 2) / 0.00006) * Math.abs(c) : 1;     // brow ridges above the eyes
        return [c * rx, sn * ry * (up ? 1 : 0.34) * bump];
    }, [0, 1, 0], 2, 3);
    shade(THREE, skull, (nm, p, i) => {
        const n = ((i * 2246822519) >>> 0) / 4294967296;
        return mix(THREE, mix(THREE, '#0f4a6a', '#1d8e9a', n * 0.6 + clamp(nm.x * nm.x * 0.5, 0, 0.4)), '#8fe0d8', clamp(-nm.y * 0.9 + 0.15, 0, 1) * 0.55 + (p.y > 0.012 ? 0.0 : 0));
    });
    const headParts = [skull];
    // horns (swept back), cheek spikes, frills
    for (const sdx of [-1, 1]) {
        const hornPath = spline(THREE, [[sdx * 0.011, 0.013, -0.020], [sdx * 0.019, 0.022, -0.032], [sdx * 0.026, 0.030, -0.046], [sdx * 0.025, 0.033, -0.060]], 12);
        const horn = sweep(THREE, hornPath, 6, (s, c, sn) => [c * 0.0048 * (1 - s) ** 0.8 + 0.0002, sn * 0.0048 * (1 - s) ** 0.8 + 0.0002], [0, 0, 1], 1, 1);
        headParts.push(shade(THREE, horn, (nm, p) => mix(THREE, '#d9b36a', '#fff0c8', clamp(p.z * -16, 0, 1) * 0.6)));
        const brow = sweep(THREE, spline(THREE, [[sdx * 0.009, 0.0165, 0.004], [sdx * 0.0145, 0.0185, -0.010], [sdx * 0.0215, 0.0125, -0.021]], 9), 6,
            (s, c, sn) => [c * 0.0030 * (1 - s * 0.7), sn * 0.0034 * (1 - s * 0.7)], [0, 0, 1]);
        headParts.push(shade(THREE, brow, () => '#d9b36a'));
        for (let k = 0; k < 4; k++) {
            const z = -0.004 - k * 0.0085, rx = table(RX, z);
            headParts.push(shade(THREE, spike(THREE, [sdx * rx * 0.97, -0.0035 - k * 0.0003, z], [sdx * 1.0, -0.15, -0.9], 0.0032 - k * 0.0003, 0.012 + (k === 1 ? 0.004 : 0) - k * 0.0012, 5), () => '#f0d393'));
        }
        // frill: a thin spiked membrane fanned out behind the eye
        const sh = new THREE.Shape();
        sh.moveTo(0, 0); sh.lineTo(0.020, 0.012); sh.lineTo(0.017, 0.007); sh.lineTo(0.034, 0.004); sh.lineTo(0.026, -0.001); sh.lineTo(0.040, -0.011);
        sh.lineTo(0.026, -0.0075); sh.lineTo(0.030, -0.020); sh.lineTo(0.012, -0.012); sh.lineTo(0, -0.012); sh.closePath();
        const fin = new THREE.ExtrudeGeometry(sh, { depth: 0.0012, bevelEnabled: false });
        fin.rotateY(Math.PI / 2); fin.rotateZ(sdx * 0.62); fin.translate(sdx * 0.0185, 0.003, -0.0125);
        headParts.push(shade(THREE, fin, (nm, p) => mix(THREE, '#15507c', '#39d3dc', clamp(Math.abs(p.z + 0.012) * 24, 0, 1))));
        // gold ribs along the frill
        for (const [ang, len] of [[0.5, 0.034], [0.05, 0.036], [-0.4, 0.030]]) {
            const dir = [sdx * Math.sin(0.62) * 0.7, Math.sin(ang) * 0.9, -Math.cos(ang)];
            headParts.push(shade(THREE, spike(THREE, [sdx * 0.0185, 0.003, -0.0125], dir, 0.0011, len, 4), () => GOLD));
        }
    }
    // crest of spines running up the top of the skull
    for (let i = 0; i < (low ? 5 : 8); i++) {
        const z = 0.020 - i * 0.0085, ry = table(RY, z);
        headParts.push(shade(THREE, spike(THREE, [0, ry * 0.96, z], [0, 1, -0.55], 0.0034, 0.011 + (i % 2) * 0.004 - i * 0.0004, 4), () => (i % 2 ? '#f0d393' : GOLD)));
    }
    // nostrils and teeth (upper row)
    for (const sdx of [-1, 1]) headParts.push(tint(THREE, new THREE.SphereGeometry(0.0021, 6, 5).scale(1, 0.7, 1.3).translate(sdx * 0.0033, 0.0045, 0.0345), '#02080c'));
    const toothCol = '#f4efdc';
    for (const sdx of [-1, 1]) for (let k = 0; k < (low ? 5 : 8); k++) {
        const z = 0.032 - k * 0.0060 - (k === 0 ? 0 : 0.002), rx = table(RX, z) * 0.9, big = k === 0 || k === 3;
        headParts.push(tint(THREE, spike(THREE, [sdx * rx, -table(RY, z) * 0.34 + 0.0006, z], [sdx * 0.08, -1, 0.12], 0.0017 + (big ? 0.0005 : 0), (big ? 0.0125 : 0.0085) * (1 - k * 0.03), 4), toothCol));
    }
    for (const sdx of [-1, 1]) headParts.push(tint(THREE, spike(THREE, [sdx * 0.0030, -table(RY, 0.034) * 0.34, 0.0345], [sdx * 0.05, -1, 0.1], 0.0014, 0.0095, 4), toothCol));
    const headGeo = own(mergeGeos(THREE, headParts.map(g => (g.attributes.color ? g : tint(THREE, g, '#ffffff')))));
    const headMesh = new THREE.Mesh(headGeo, creatureMat); headMesh.name = 'leviathanSkull'; headMesh.frustumCulled = false; headTop.add(headMesh);

    // ---- jaw (hinged at the back of the cheeks) ----
    const jawPivot = new THREE.Group(); jawPivot.position.set(0, -0.0085, -0.0135); headTop.add(jawPivot);
    const JRX = [[-0.012, 0.0125], [-0.004, 0.0135], [0.008, 0.0120], [0.020, 0.0090], [0.030, 0.0062], [0.034, 0.0035]];
    const jawPath = []; const JN = low ? 12 : 20; for (let i = 0; i < JN; i++) { const z = -0.012 + 0.046 * i / (JN - 1); jawPath.push([0, -0.0006 + 0.0008 * (i / JN), z]); }
    const jawGeo0 = sweep(THREE, jawPath, seg, (s, c, sn) => {
        const z = -0.012 + 0.046 * s, rx = table(JRX, z);
        return [c * rx, sn * rx * 0.52 * (sn > 0 ? 0.8 : 1)];
    }, [0, 1, 0], 2, 2);
    shade(THREE, jawGeo0, (nm, p, i) => mix(THREE, mix(THREE, '#0f4a6a', '#1c8794', (((i * 2246822519) >>> 0) / 4294967296) * 0.5), '#a6ece0', clamp(-nm.y * 0.9 + 0.1, 0, 1) * 0.7));
    const jawParts = [jawGeo0];
    for (const sdx of [-1, 1]) for (let k = 0; k < (low ? 4 : 7); k++) {
        const z = 0.030 - k * 0.0062, rx = table(JRX, z) * 0.82, big = k === 1;
        jawParts.push(tint(THREE, spike(THREE, [sdx * rx, 0.0032, z], [sdx * -0.06, 1, 0.1], 0.0016 + (big ? 0.0006 : 0), big ? 0.012 : 0.0085, 4), toothCol));
    }
    jawParts.push(shade(THREE, spike(THREE, [0, -0.0035, 0.034], [0, -0.7, 1], 0.0034, 0.011, 5), () => '#d9b36a'));      // chin spike
    for (const sdx of [-1, 1]) jawParts.push(shade(THREE, spike(THREE, [sdx * 0.011, -0.0035, -0.004], [sdx * 0.8, -0.3, -0.9], 0.0030, 0.012, 5), () => '#d9b36a'));
    const jawMesh = new THREE.Mesh(own(mergeGeos(THREE, jawParts.map(g => (g.attributes.color ? g : tint(THREE, g, '#ffffff'))))), creatureMat);
    jawMesh.name = 'leviathanJaw'; jawMesh.frustumCulled = false;
    jawMesh.geometry.translate(0, 0, 0.0135);       // built in the skull frame; the pivot sits at z -0.0135
    jawPivot.add(jawMesh);

    // ---- eyes + pupils (instanced; pupils track the nearest ball) ----
    const EYE_R = 0.0068, EYE_POS = [[-0.0155, 0.0075, -0.0125], [0.0155, 0.0075, -0.0125]], D0 = [new THREE.Vector3(-0.55, 0.10, 0.83).normalize(), new THREE.Vector3(0.55, 0.10, 0.83).normalize()];
    const eyeMat = own(new THREE.MeshBasicMaterial({ color: '#ffffff', toneMapped: false }));
    const eyes = new THREE.InstancedMesh(own(new THREE.SphereGeometry(EYE_R, low ? 10 : 16, low ? 8 : 12)), eyeMat, 2); eyes.frustumCulled = false; eyes.name = 'leviathanEyes';
    const pupilMat = own(new THREE.MeshBasicMaterial({ color: '#02070b', toneMapped: false }));
    const pupils = new THREE.InstancedMesh(own(new THREE.SphereGeometry(0.0034, 8, 6)), pupilMat, 2); pupils.frustumCulled = false; pupils.name = 'leviathanPupils';
    eyes.setColorAt(0, new THREE.Color('#ffffff')); eyes.setColorAt(1, new THREE.Color('#ffffff'));
    headTop.add(eyes, pupils);
    // soft halos: camera-facing glow points (eyes + the glowing throat)
    const hg = own(new THREE.BufferGeometry());
    hg.setAttribute('position', new THREE.BufferAttribute(new Float32Array([EYE_POS[0][0], EYE_POS[0][1], EYE_POS[0][2] + 0.003, EYE_POS[1][0], EYE_POS[1][1], EYE_POS[1][2] + 0.003, 0, -0.004, 0.006]), 3));
    hg.setAttribute('color', new THREE.BufferAttribute(new Float32Array(9), 3));
    const hgMat = own(new THREE.PointsMaterial({ map: glowTex, size: 0.048, vertexColors: true, transparent: true, blending: THREE.AdditiveBlending, depthWrite: false, toneMapped: false, sizeAttenuation: true }));
    const headGlow = new THREE.Points(hg, hgMat); headGlow.frustumCulled = false; headGlow.renderOrder = 8; headTop.add(headGlow);

    // ---- sunken ship ----
    const woodMat = own(new THREE.MeshStandardMaterial({ map: woodTex, vertexColors: true, roughness: 0.9, metalness: 0.02, emissive: '#2a4044', emissiveIntensity: 0.9, side: THREE.DoubleSide }));
    const hullParts = [], bioPos = [], bioCol = [];
    const A = d => d * Math.PI / 180;
    const onArc = (r, deg, h) => [sx + r * Math.cos(A(deg)), h, -(sy + r * Math.sin(A(deg)))];
    const wood = (tone) => mix(THREE, '#7a6347', '#c4a578', tone);
    // ribs: curved timbers rising from the ring, broken at different heights
    const ribAngles = []; for (let a = -42; a <= 132; a += low ? 18 : 11) ribAngles.push(a);
    ribAngles.forEach((a, i) => {
        const hh = 0.022 + rngNext(rng) * 0.030 + (a > 15 && a < 75 ? 0.020 : 0), r0 = 0.0345, lean = 0.006 + rngNext(rng) * 0.005;
        const pts = spline(THREE, [onArc(r0, a, 0.0), onArc(r0 + lean * 0.5, a + 2, hh * 0.45), onArc(r0 + lean * 1.2, a + 4, hh)], 8);
        const rib = sweep(THREE, pts, 5, (s, c, sn) => { const w = 0.0021 * (1 - 0.55 * s); return [c * w, sn * w * 1.5]; }, [0, 1, 0]);
        hullParts.push(shade(THREE, rib, () => wood(rngNext(rng))));
        if (rngNext(rng) < 0.5) hullParts.push(shade(THREE, spike(THREE, pts[pts.length - 1], [0.2, 1, 0.1], 0.0014, 0.006, 4), () => '#c8b186'));
    });
    // planking: arc spans at three levels, with gaps where the hull is broken open
    const plank = (a0, a1, h, r, thick = 0.0020) => {
        const pts = []; const n = Math.max(3, Math.round((a1 - a0) / 6)); for (let i = 0; i <= n; i++) pts.push(onArc(r, a0 + (a1 - a0) * i / n, h));
        const g = sweep(THREE, pts, 4, (s, c, sn) => [c * thick, sn * 0.0042], [0, 1, 0]);
        hullParts.push(shade(THREE, g, () => wood(rngNext(rng) * 0.9)));
    };
    for (const [h, spans] of [[0.006, [[-40, 130]]], [0.015, [[-40, 40], [55, 130]]], [0.025, [[-30, 12], [30, 95], [108, 128]]], [0.036, [[0, 35], [48, 78]]], [0.047, [[10, 30]]]]) {
        for (const [a0, a1] of spans) plank(a0, a1, h, 0.0385 + h * 0.07);
    }
    // stern post: a tall curled timber at the back of the wreck
    hullParts.push(shade(THREE, sweep(THREE, spline(THREE, [onArc(0.040, 52, 0), onArc(0.043, 54, 0.04), onArc(0.046, 56, 0.07), onArc(0.040, 58, 0.088)], 12), 5,
        (s, c, sn) => [c * 0.0030 * (1 - 0.5 * s), sn * 0.0042 * (1 - 0.5 * s)], [0, 1, 0]), () => wood(0.5)));
    // mast stump with a splintered top, a yard fragment and a torn sail
    const mastBase = onArc(0.036, 118, 0), mastTop = [mastBase[0] - 0.006, 0.068, mastBase[2] + 0.003];
    hullParts.push(shade(THREE, sweep(THREE, spline(THREE, [mastBase, [mastBase[0] - 0.002, 0.03, mastBase[2] + 0.001], mastTop], 8), 7, (s, c, sn) => [c * 0.0032 * (1 - 0.15 * s), sn * 0.0032 * (1 - 0.15 * s)], [0, 0, 1]), () => wood(0.4)));
    for (let k = 0; k < 5; k++) hullParts.push(shade(THREE, spike(THREE, [mastTop[0] + (k - 2) * 0.0011, mastTop[1], mastTop[2] + ((k * 7) % 3 - 1) * 0.0010], [(k - 2) * 0.25, 1, 0.1], 0.0011, 0.004 + (k % 3) * 0.0025, 3), () => '#b9a27a'));
    const yardY = 0.052, yard = new THREE.CylinderGeometry(0.0013, 0.0013, 0.036, 5); yard.rotateZ(Math.PI / 2 + 0.18); yard.translate(mastBase[0] - 0.005, yardY, mastBase[2] + 0.002);
    hullParts.push(shade(THREE, yard, () => wood(0.3)));
    const sail = new THREE.PlaneGeometry(0.026, 0.024, 4, 4);
    { const p = sail.attributes.position; for (let i = 0; i < p.count; i++) { const x = p.getX(i), y = p.getY(i); p.setZ(i, Math.sin(x * 220 + y * 90) * 0.0025 + (y < 0 ? -y * 0.05 : 0)); p.setX(i, x + (y < 0 ? -y * 0.18 : 0)); } sail.computeVertexNormals(); }
    sail.translate(mastBase[0] - 0.006, yardY - 0.0145, mastBase[2] + 0.004);
    hullParts.push(shade(THREE, sail, (nm, p, i) => mix(THREE, '#51606a', '#a9bfc0', 0.35 + 0.35 * Math.sin(i * 1.3))));
    // rope sagging from the yard to a rib
    hullParts.push(shade(THREE, sweep(THREE, spline(THREE, [[mastBase[0] - 0.021, yardY - 0.004, mastBase[2] + 0.005], onArc(0.047, 160, 0.034), onArc(0.040, 142, 0.030)], 10), 4, (s, c, sn) => [c * 0.0007, sn * 0.0007], [0, 1, 0]), () => '#a38a5e'));
    // lantern post (the lantern itself is a glowing mesh)
    const lanPos = onArc(0.040, 84, 0), lanH = 0.052;
    hullParts.push(shade(THREE, sweep(THREE, spline(THREE, [lanPos, [lanPos[0], 0.030, lanPos[2]], [lanPos[0] + 0.001, lanH - 0.004, lanPos[2]]], 6), 5, (s, c, sn) => [c * 0.0017, sn * 0.0017], [0, 0, 1]), () => wood(0.45)));
    const cage = new THREE.CylinderGeometry(0.0046, 0.0046, 0.0105, 6, 1, true); cage.translate(lanPos[0] + 0.001, lanH + 0.0045, lanPos[2]);
    hullParts.push(tint(THREE, cage, '#2c2a26'));
    hullParts.push(tint(THREE, new THREE.ConeGeometry(0.0058, 0.004, 6).translate(lanPos[0] + 0.001, lanH + 0.012, lanPos[2]), '#2c2a26'));
    // silt mound along the hull foot with encrusting barnacles
    const moundPts = []; for (let a = -46; a <= 140; a += 8) moundPts.push(onArc(0.0415, a, 0.0025));
    const siltG = sweep(THREE, moundPts, 6, (s, c, sn, i) => { const k = 0.0075 + 0.0030 * Math.sin(i * 1.7) ** 2; return [c * k * 1.5, sn * k * 0.8]; }, [0, 1, 0], 6, 1);
    hullParts.push(shade(THREE, siltG, (nm, p) => mix(THREE, '#1b2d33', '#46646a', rngNext(rng))));
    for (let i = 0; i < (low ? 10 : 22); i++) {
        const a = -40 + rngNext(rng) * 170, r = 0.035 + rngNext(rng) * 0.012, p = onArc(r, a, 0.002 + rngNext(rng) * 0.03);
        const rad = 0.0013 + rngNext(rng) * 0.0016;
        hullParts.push(tint(THREE, new THREE.SphereGeometry(rad, 5, 4).translate(p[0], p[1], p[2]), '#c9d9d3'));
        if (rngNext(rng) < 0.45) { bioPos.push(p[0], p[1] + rad, p[2]); const c = new THREE.Color(rngNext(rng) < 0.7 ? '#43e3d5' : '#58f59a'); bioCol.push(c.r, c.g, c.b); }
    }
    const hullGeo = own(mergeGeos(THREE, hullParts));
    { const uv = hullGeo.attributes.uv; for (let i = 0; i < uv.count; i++) uv.setXY(i, uv.getX(i) * 1.6, uv.getY(i) * 1.6); }
    const hullMesh = new THREE.Mesh(hullGeo, woodMat); hullMesh.name = 'leviathanHull'; hullMesh.frustumCulled = false; group.add(hullMesh);

    // lantern glass core + bioluminescent beads (one unlit mesh; lantern vertices pulse)
    const lights = [];
    const coreG = new THREE.SphereGeometry(0.0032, 8, 6); coreG.scale(1, 1.4, 1); coreG.translate(lanPos[0] + 0.001, lanH + 0.0045, lanPos[2]); tint(THREE, coreG, '#ffffff'); lights.push(coreG);
    const lanVerts = coreG.attributes.position.count;
    for (let i = 0; i < bioPos.length; i += 3) lights.push(tint(THREE, new THREE.SphereGeometry(0.0010, 5, 4).translate(bioPos[i], bioPos[i + 1], bioPos[i + 2]), '#ffffff'));
    // glowing sea-life beads along the creature's neck and coils
    for (let i = 0; i < (low ? 8 : 16); i++) {
        const s = 0.08 + (i % 8) * 0.095, p = backAt(Math.min(0.97, s)), side = i < 8 ? -1 : 1, r = neckR(s) * 0.96;
        lights.push(tint(THREE, new THREE.SphereGeometry(0.0012, 5, 4).translate(hx + p[0] + side * r, p[1], -hy + p[2] + r * 0.2), '#ffffff'));
    }
    const lightsGeo = own(mergeGeos(THREE, lights));
    const lightCol = lightsGeo.attributes.color;
    const lightsMesh = new THREE.Mesh(lightsGeo, own(new THREE.MeshBasicMaterial({ vertexColors: true, toneMapped: false }))); lightsMesh.name = 'leviathanLights'; lightsMesh.frustumCulled = false; group.add(lightsMesh);
    const beadsEnd = lightsGeo.attributes.position.count;

    // world glow: lantern halo and soft pools around the head base and ship
    const wg = own(new THREE.BufferGeometry());
    wg.setAttribute('position', new THREE.BufferAttribute(new Float32Array([lanPos[0] + 0.001, lanH + 0.0045, lanPos[2], sx, 0.012, -sy, hx, 0.012, -hy]), 3));
    wg.setAttribute('color', new THREE.BufferAttribute(new Float32Array(9), 3));
    const wgMat = own(new THREE.PointsMaterial({ map: glowTex, size: 0.055, vertexColors: true, transparent: true, blending: THREE.AdditiveBlending, depthWrite: false, toneMapped: false, sizeAttenuation: true }));
    const worldGlow = new THREE.Points(wg, wgMat); worldGlow.frustumCulled = false; worldGlow.renderOrder = 6; group.add(worldGlow);

    // ---- kelp: instanced blades with a vertex-shader sway ----
    const kelpToys = ((layout && layout.toys) || []).filter(t => t.kind === 'kelp');
    const kelpList = kelpToys.length ? kelpToys : [{ x: -0.246, y: 0.565, rot: 0 }, { x: -0.238, y: 0.588, rot: 1 }, { x: 0.213, y: 0.565, rot: 2 }, { x: 0.206, y: 0.588, rot: 3 }];
    const BL_H = 0.070, blade = new THREE.PlaneGeometry(0.0075, BL_H, 1, 7); blade.translate(0, BL_H / 2, 0);
    {
        const p = blade.attributes.position, col = new Float32Array(p.count * 3), c = new THREE.Color();
        for (let i = 0; i < p.count; i++) {
            const y = clamp(p.getY(i) / BL_H, 0, 1), w = 1 - 0.62 * y * y;
            p.setX(i, p.getX(i) * w * (1 + 0.5 * Math.sin(y * 9)));
            c.set(mix(THREE, '#0e3a2c', '#3fe3a0', y ** 1.5)); col[i * 3] = c.r; col[i * 3 + 1] = c.g; col[i * 3 + 2] = c.b;
        }
        blade.setAttribute('color', new THREE.BufferAttribute(col, 3)); blade.computeVertexNormals();
    }
    own(blade);
    const kelpMat = own(new THREE.MeshStandardMaterial({ vertexColors: true, side: THREE.DoubleSide, roughness: 0.6, metalness: 0, emissive: '#0c5a44', emissiveIntensity: 0.9 }));
    const perToy = low ? 3 : 6, KN = kelpList.length * perToy;
    const kelp = new THREE.InstancedMesh(blade, kelpMat, KN); kelp.frustumCulled = false; kelp.name = 'leviathanKelp';
    const tmp = new THREE.Object3D(), kelpData = [];
    for (const kt of kelpList) {
        for (let k = 0; k < perToy; k++) {
            kelpData.push({ x: kt.x + (rngNext(rng) - 0.5) * 0.012, y: kt.y + (rngNext(rng) - 0.5) * 0.016, yaw: rngNext(rng) * Math.PI, lean: (rngNext(rng) - 0.5) * 0.25,
                sx: 0.9 + rngNext(rng) * 0.5, sc: 0.7 + rngNext(rng) * 0.8, ph: rngNext(rng) * 6.28, sp: 1.1 + rngNext(rng) * 0.8 });
        }
    }
    const writeKelp = (t, amp) => {
        for (let i = 0; i < KN; i++) {
            const d = kelpData[i];
            tmp.position.set(d.x, 0.001, -d.y);
            tmp.rotation.set(Math.cos(t * d.sp * 0.8 + d.ph) * 0.10 * amp, d.yaw, d.lean + Math.sin(t * d.sp + d.ph) * 0.16 * amp);
            tmp.scale.set(d.sx, d.sc, 1); tmp.updateMatrix(); kelp.setMatrixAt(i, tmp.matrix);
        }
        kelp.instanceMatrix.needsUpdate = true;
    };
    writeKelp(0, 1); group.add(kelp);

    // ---- bubbles and plankton (high tier only) ----
    let bubbles = null, bub = null, motes = null, moteBase = null, moteCol = null;
    const BN = 56;
    if (!low) {
        bub = { x: new Float32Array(BN), h: new Float32Array(BN), z: new Float32Array(BN), v: new Float32Array(BN), ph: new Float32Array(BN), life: new Float32Array(BN), next: 0 };
        for (let i = 0; i < BN; i++) { bub.h[i] = -1; bub.life[i] = 0; }
        const bg = own(new THREE.BufferGeometry()); bg.setAttribute('position', new THREE.BufferAttribute(new Float32Array(BN * 3), 3));
        const bm = own(new THREE.PointsMaterial({ map: own(bubbleTexture(THREE)), size: 0.0085, color: '#cfffff', transparent: true, opacity: 0.9, blending: THREE.AdditiveBlending, depthWrite: false, toneMapped: false }));
        bubbles = new THREE.Points(bg, bm); bubbles.frustumCulled = false; bubbles.renderOrder = 7; group.add(bubbles);
        const MN = 48; moteBase = new Float32Array(MN * 4); moteCol = new Float32Array(MN * 3);
        for (let i = 0; i < MN; i++) { moteBase[i * 4] = -0.24 + rngNext(rng) * 0.48; moteBase[i * 4 + 1] = 0.12 + rngNext(rng) * 0.85; moteBase[i * 4 + 2] = rngNext(rng) * 6.28; moteBase[i * 4 + 3] = 0.2 + rngNext(rng) * 0.4; }
        const mg = own(new THREE.BufferGeometry()); mg.setAttribute('position', new THREE.BufferAttribute(new Float32Array(MN * 3), 3)); mg.setAttribute('color', new THREE.BufferAttribute(moteCol, 3));
        const mm = own(new THREE.PointsMaterial({ map: glowTex, size: 0.0085, vertexColors: true, transparent: true, blending: THREE.AdditiveBlending, depthWrite: false, toneMapped: false }));
        motes = new THREE.Points(mg, mm); motes.frustumCulled = false; motes.renderOrder = 7; group.add(motes);
    }
    const emit = (x, y, h, n, spread = 0.006) => {
        if (!bub) return;
        for (let k = 0; k < n; k++) {
            const i = bub.next; bub.next = (bub.next + 1) % BN;
            bub.x[i] = x + (rngNext(rng) - 0.5) * spread; bub.z[i] = -(y + (rngNext(rng) - 0.5) * spread); bub.h[i] = h + rngNext(rng) * 0.006;
            bub.v[i] = 0.035 + rngNext(rng) * 0.05; bub.ph[i] = rngNext(rng) * 6.28; bub.life[i] = 1.6 + rngNext(rng) * 1.4;
        }
    };

    // ---- per-frame state ----
    const eyeColor = new THREE.Color('#43e3d5'), eyeTarget = new THREE.Color(), tmpC = new THREE.Color(), white = new THREE.Color('#ffffff');
    const gaze = new THREE.Vector3(hx, 0.03, -hy + 0.2), gazeGoal = new THREE.Vector3(), eyeW = new THREE.Vector3(), dLoc = new THREE.Vector3(), dir = new THREE.Vector3();
    const qInv = new THREE.Quaternion(), qLim = new THREE.Quaternion(), qAim = new THREE.Quaternion(), zAxis = new THREE.Vector3(0, 0, 1);
    const eyeObj = new THREE.Object3D(), pupObj = new THREE.Object3D();
    let flare = 0, snap = 0, lantern = 0, jawA = 0, jawV = 0, yaw = 0, pitch = 0.2, open = 0.3, eyeLvl = [0.3, 0.3], blink = 0, nextBlink = 3.2, bubbleT = 0, idleT = 0;
    const lampLevel = (lamps, id, fallback) => {
        const i = lamps && lamps.index && lamps.index.get ? lamps.index.get(id) : undefined;
        return i !== undefined && lamps.intensity ? lamps.intensity[i] : fallback;
    };
    function updateBubbles(dt, t) {
        if (!bub) return;
        const a = bubbles.geometry.attributes.position;
        for (let i = 0; i < BN; i++) {
            if (bub.life[i] > 0) {
                bub.life[i] -= dt; bub.h[i] += bub.v[i] * dt; bub.ph[i] += dt * 4;
                if (bub.life[i] <= 0 || bub.h[i] > 0.24) { bub.life[i] = 0; bub.h[i] = -1; }
                a.setXYZ(i, bub.x[i] + Math.sin(bub.ph[i]) * 0.0022, bub.h[i], bub.z[i] + Math.cos(bub.ph[i] * 0.8) * 0.0018);
            } else a.setXYZ(i, 0, -1, 0);
        }
        a.needsUpdate = true;
        void t;
    }
    function updateMotes(t) {
        if (!motes) return;
        const a = motes.geometry.attributes.position, c = motes.geometry.attributes.color, n = a.count;
        for (let i = 0; i < n; i++) {
            const b = i * 4, ph = moteBase[b + 2] + t * moteBase[b + 3];
            a.setXYZ(i, moteBase[b] + Math.sin(ph * 1.1) * 0.012, 0.012 + 0.04 * (1 + Math.sin(ph * 0.7 + i)), -(moteBase[b + 1] + Math.cos(ph * 0.9) * 0.015));
            const tw = 0.25 + 0.75 * Math.max(0, Math.sin(t * (0.8 + moteBase[b + 3]) + i * 2.1)) ** 2;
            c.setXYZ(i, 0.25 * tw, 0.95 * tw, 0.85 * tw);
        }
        a.needsUpdate = true; c.needsUpdate = true;
    }

    return {
        group,
        update(dt, t, snapshot, toyState, lamps, events) {
            dt = Number.isFinite(dt) ? Math.min(dt, 0.1) : 0.016; t = Number.isFinite(t) ? t : 0;
            const ts = toyState || {}, te = ts.eye || {}, k = a => Math.min(1, dt * a);
            if (events && events.length) for (const e of events) {
                const id = String((e && (e.id || e.device)) || ''), type = String((e && e.type) || '');
                if (type === 'pathMade' && /tentacleRamp/.test(id)) { flare = 1; snap = 1; }
                else if (type === 'hole' && /shipScoop/.test(id)) { flare = Math.max(flare, 0.8); snap = 1; lantern = 1; }
                else if (/ship|flShip/i.test(id) && type !== 'bumper') lantern = Math.max(lantern, 0.8);
                else if (type === 'captive' || /pearl/i.test(id)) emit(PEARL[0], PEARL[1], 0.012, 11, 0.012);
            }
            flare = Math.max(0, flare - dt * 1.5); snap = Math.max(0, snap - dt * 3.2); lantern = Math.max(0, lantern - dt * 1.4);
            const lit = finite(ts.jaw, 0);

            // gaze: nearest ball (table frame), else a slow wander toward the player
            let best = null, bd = 1e9;
            const balls = (snapshot && snapshot.balls) || [];
            for (const b of balls) { if (!b || !Number.isFinite(b.x) || !Number.isFinite(b.y)) continue; const d = (b.x - hx) ** 2 + (b.y - hy) ** 2; if (d < bd) { bd = d; best = b; } }
            if (best) gazeGoal.set(best.x, finite(best.h, 0.014) + 0.012, -best.y);
            else gazeGoal.set(hx + Math.sin(t * 0.5) * 0.08, 0.03, -hy + 0.22 + Math.sin(t * 0.37) * 0.05);
            gaze.lerp(gazeGoal, k(best ? 9 : 2));
            const gdx = gaze.x - hx, gdz = gaze.z + hy;           // gdz > 0 means the ball is in front (toward the player)
            const near = Math.hypot(gdx, gdz);
            const yawGoal = clamp(Math.atan2(gdx, gdz + 0.0001) * 0.45, -0.5, 0.5) * (near < 0.012 ? 0 : 1);
            const pitchGoal = 0.20 + clamp(0.04 / Math.max(0.03, near), 0, 0.2) + 0.06 * Math.sin(t * 1.3) + snap * 0.10 - lit * 0.18;
            yaw += (yawGoal - yaw) * k(5); pitch += (pitchGoal - pitch) * k(6);
            const breath = Math.sin(t * 1.35), breath2 = Math.sin(t * 1.35 - 0.6);
            headTop.position.set(hx, HEAD_H + breath * 0.0014, -hy);
            headTop.rotation.set(pitch, yaw, Math.sin(t * 0.7) * 0.015);
            neckMesh.scale.set(1 + 0.022 * breath2, 1 + breath * 0.0014 / HEAD_H, 1 + 0.022 * breath2);
            headTop.updateMatrix();

            // jaw: spring toward the open amount (multiball / wizard) plus a snap on scoop and ramp events
            const jawGoal = clamp(lit * 0.62 + snap * 0.42 + 0.025 + 0.02 * breath, 0, 0.8);
            jawV += ((jawGoal - jawA) * 220 - jawV * 18) * dt; jawA += jawV * dt;
            jawPivot.rotation.x = jawA;

            // eye colour, openness, blink
            eyeTarget.copy(colorOf(THREE, te.color, TEAL)); eyeColor.lerp(eyeTarget, k(6));
            open += (clamp(finite(te.open, 0.3), 0, 1) - open) * k(5);
            nextBlink -= dt; if (nextBlink <= 0) { blink = 0.14; nextBlink = 2.6 + rngNext(rng) * 4; } blink = Math.max(0, blink - dt);
            const lid = (0.22 + 0.78 * clamp(open + flare * 0.5, 0, 1)) * (blink > 0 ? 0.12 : 1);
                        ['eyeL', 'eyeR'].forEach((id, e) => {
                const lv = clamp(Math.max(lampLevel(lamps, id, 0), open * 0.8) + flare * 0.7, 0, 1.5), fl = 0.94 + 0.06 * Math.sin(t * 8 + e * 1.9);
                eyeLvl[e] = (0.35 + lv * 1.9) * fl;
                tmpC.copy(eyeColor).lerp(white, flare * 0.4).multiplyScalar(eyeLvl[e]); eyes.setColorAt(e, tmpC);
                const E = EYE_POS[e];
                eyeObj.position.set(E[0], E[1], E[2]); eyeObj.rotation.set(0, 0, 0); eyeObj.scale.set(1, lid, 1); eyeObj.updateMatrix(); eyes.setMatrixAt(e, eyeObj.matrix);
                // pupil: aim from the eye toward the gaze point (in the skull frame), limited to the eye's field of view
                eyeW.set(E[0], E[1], E[2]).applyMatrix4(headTop.matrix);
                dLoc.copy(gaze).sub(eyeW); if (dLoc.lengthSq() < 1e-8) dLoc.set(0, 0, 1);
                dLoc.normalize().applyQuaternion(qInv.copy(headTop.quaternion).invert());
                const ang = D0[e].angleTo(dLoc), maxA = 0.8;
                if (ang > maxA) { qLim.setFromUnitVectors(D0[e], dLoc); qLim.slerp(qAim.identity(), 1 - maxA / ang); dir.copy(D0[e]).applyQuaternion(qLim); } else dir.copy(dLoc);
                pupObj.position.set(E[0] + dir.x * EYE_R * 0.9, E[1] + dir.y * EYE_R * 0.9 * lid, E[2] + dir.z * EYE_R * 0.9);
                pupObj.quaternion.setFromUnitVectors(zAxis, dir);
                pupObj.scale.set(0.42, 1.05 * Math.max(0.3, lid), 0.45); pupObj.updateMatrix(); pupils.setMatrixAt(e, pupObj.matrix);
                const gc = hg.attributes.color; const hv = eyeLvl[e] * 0.22; gc.setXYZ(e, eyeColor.r * hv, eyeColor.g * hv, eyeColor.b * hv);
            });
            eyes.instanceMatrix.needsUpdate = true; eyes.instanceColor.needsUpdate = true; pupils.instanceMatrix.needsUpdate = true;
            { const mv = clamp(jawA * 1.6 + flare * 0.25, 0, 1) * 0.55; hg.attributes.color.setXYZ(2, eyeColor.r * mv, eyeColor.g * mv, eyeColor.b * mv); hg.attributes.color.needsUpdate = true; }

            // lantern: soft flicker, flashes on ship hits
            const flick = 0.85 + 0.1 * Math.sin(t * 7.3) + 0.06 * Math.sin(t * 13.1 + 1.2), lv = (0.55 + lantern * 2.4) * flick;
            for (let v = 0; v < lanVerts; v++) lightCol.setXYZ(v, 1.0 * lv, 0.72 * lv, 0.30 * lv);
            for (let v = lanVerts; v < beadsEnd; v++) { const ph = 0.55 + 0.45 * Math.sin(t * 1.7 + v * 0.37); lightCol.setXYZ(v, 0.25 * ph * (0.8 + flare), 0.95 * ph * (0.8 + flare), 0.85 * ph * (0.8 + flare)); }
            lightCol.needsUpdate = true;
            const wc = wg.attributes.color, lh = 0.20 + lantern * 0.55;
            wc.setXYZ(0, 1.0 * lh, 0.72 * lh, 0.30 * lh);
            wc.setXYZ(1, 0.9 * (0.07 + lantern * 0.12), 0.65 * (0.07 + lantern * 0.12), 0.25 * (0.07 + lantern * 0.12));
            const hp = 0.10 + flare * 0.25 + lit * 0.15; wc.setXYZ(2, eyeColor.r * hp * 0.4, eyeColor.g * hp * 0.4, eyeColor.b * hp * 0.4); wc.needsUpdate = true;

            // kelp, bubbles, plankton
            writeKelp(t, 1 + flare * 0.8);
            idleT += dt;
            if (bub) {
                bubbleT -= dt;
                if (bubbleT <= 0) {
                    bubbleT = 0.5 + rngNext(rng) * 0.9;
                    const r = rngNext(rng);
                    if (r < 0.45) { const kt = kelpList[Math.floor(rngNext(rng) * kelpList.length)]; emit(kt.x, kt.y, 0.05, 1, 0.01); }
                    else if (r < 0.7) emit(hx + 0.006 * Math.sin(idleT), hy - 0.030, HEAD_H - 0.012, 1 + (lit > 0.3 ? 2 : 0), 0.008);
                    else emit(sx - 0.02, sy + 0.01, 0.03, 1, 0.02);
                }
                if (snap > 0.9) emit(hx, hy - 0.032, HEAD_H - 0.012, 5, 0.012);
                updateBubbles(dt, t); updateMotes(t);
            }
        },
        dispose() {
            for (const d of disposables) if (d && d.dispose) d.dispose();
            kelp.dispose(); eyes.dispose(); pupils.dispose();
            if (group.parent) group.parent.remove(group);
        }
    };
}

export default buildToys;
