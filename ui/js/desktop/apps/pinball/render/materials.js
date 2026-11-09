// AuraPinball render materials, quality tiers and small geometry helpers (plan section 7).
import * as THREE from 'three';
import { mergeGeometries } from 'three/addons/utils/BufferGeometryUtils.js';
import { rngNext } from '../config.js';

// DPR cap, bloom, shadow map size (0 = none), art size, point lights, MSAA samples, anisotropy.
export const TIERS = Object.freeze({
    low: { dpr: 1, bloom: null, shadow: 0, art: [1024, 2048], lights: 2, samples: 2, aniso: 1, lampGain: 1.9 },
    medium: { dpr: 1.25, bloom: { strength: 0.42, radius: 0.38, threshold: 1.3, half: true }, shadow: 1024, art: [2048, 4096], lights: 6, samples: 4, aniso: 4, lampGain: 1.4 },
    high: { dpr: 1.5, bloom: { strength: 0.5, radius: 0.4, threshold: 1.3, half: false }, shadow: 2048, art: [2048, 4096], lights: 6, samples: 4, aniso: 8, lampGain: 1.45 }
});
export const tierOf = q => TIERS[q] || TIERS.high;

// ---- geometry helpers (three coordinates: X = x, Y = h, Z = -y) ----
export function box(x, y, z, sx, sy, sz) { return new THREE.BoxGeometry(sx, sy, sz).translate(x, y, z); }
export function merge(list) {
    const parts = list.filter(Boolean);
    if (!parts.length) return null;
    const out = parts.length === 1 ? parts[0] : mergeGeometries(parts, false);
    if (parts.length > 1) parts.forEach(g => g.dispose());
    return out;
}
// Axis-aligned quad from four corners (a,b,c,d counter-clockwise seen from the front) with uv (0,0)..(1,1).
export function quad(a, b, c, d, uv = [0, 0, 1, 1]) {
    const g = new THREE.BufferGeometry();
    g.setAttribute('position', new THREE.Float32BufferAttribute([...a, ...b, ...c, ...a, ...c, ...d], 3));
    g.setAttribute('uv', new THREE.Float32BufferAttribute([uv[0], uv[1], uv[2], uv[1], uv[2], uv[3], uv[0], uv[1], uv[2], uv[3], uv[0], uv[3]], 2));
    g.computeVertexNormals();
    return g;
}

// Extruded strip along a polyline (playfield metres [x, y]); height h1 above h0. side: 0 centred, +1 outward from `away`
// (offset to the side pointing away from that point), thickness in metres. Returns an indexed-free BufferGeometry.
export function ribbonGeometry(points, { thickness = 0.008, h0 = 0, h1 = 0.03, away = null, uvLen = 0.12 } = {}) {
    const n = points.length;
    if (n < 2) return null;
    const pos = [], uvs = [];
    const seg = [];
    for (let i = 0; i < n - 1; i++) { const dx = points[i + 1][0] - points[i][0], dy = points[i + 1][1] - points[i][1], l = Math.hypot(dx, dy) || 1e-9; seg.push([-dy / l, dx / l, l]); }
    let sgn = 0;
    if (away) {
        let acc = 0;
        for (let i = 0; i < n - 1; i++) { const mx = (points[i][0] + points[i + 1][0]) / 2 - away[0], my = (points[i][1] + points[i + 1][1]) / 2 - away[1]; acc += (seg[i][0] * mx + seg[i][1] * my) * seg[i][2]; }
        sgn = acc >= 0 ? 1 : -1;
    }
    const L = [], R = [], dist = [0];
    for (let i = 1; i < n; i++) dist[i] = dist[i - 1] + seg[i - 1][2];
    for (let i = 0; i < n; i++) {
        const a = seg[Math.max(0, i - 1)], b = seg[Math.min(n - 2, i)];
        let nx = a[0] + b[0], ny = a[1] + b[1];
        const l = Math.hypot(nx, ny);
        if (l < 1e-6) { nx = b[0]; ny = b[1]; } else { nx /= l; ny /= l; }
        const k = Math.min(2.5, 1 / Math.max(0.4, nx * b[0] + ny * b[1]));
        const lo = sgn > 0 ? thickness : sgn < 0 ? 0 : thickness / 2, ro = sgn > 0 ? 0 : sgn < 0 ? thickness : thickness / 2;
        L.push([points[i][0] + nx * lo * k, points[i][1] + ny * lo * k]);
        R.push([points[i][0] - nx * ro * k, points[i][1] - ny * ro * k]);
    }
    const P = (p, h) => [p[0], h, -p[1]];
    const add = (a, b, c, d, hint, u0, u1, v0, v1) => {
        const ab = [b[0] - a[0], b[1] - a[1], b[2] - a[2]], ac = [c[0] - a[0], c[1] - a[1], c[2] - a[2]];
        const nrm = [ab[1] * ac[2] - ab[2] * ac[1], ab[2] * ac[0] - ab[0] * ac[2], ab[0] * ac[1] - ab[1] * ac[0]];
        const flip = nrm[0] * hint[0] + nrm[1] * hint[1] + nrm[2] * hint[2] < 0;
        const A = [u0, v0], B = [u1, v0], C = [u1, v1], D = [u0, v1];
        const v = flip ? [a, d, c, a, c, b] : [a, b, c, a, c, d];
        const t = flip ? [A, D, C, A, C, B] : [A, B, C, A, C, D];
        for (const p of v) pos.push(p[0], p[1], p[2]);
        for (const q of t) uvs.push(q[0], q[1]);
    };
    for (let i = 0; i < n - 1; i++) {
        const u0 = dist[i] / uvLen, u1 = dist[i + 1] / uvLen;
        add(P(L[i], h1), P(R[i], h1), P(R[i + 1], h1), P(L[i + 1], h1), [0, 1, 0], u0, u1, 0, 1);
        add(P(L[i], h0), P(L[i], h1), P(L[i + 1], h1), P(L[i + 1], h0), [seg[i][0], 0, -seg[i][1]], u0, u1, 0, 1);
        add(P(R[i], h0), P(R[i], h1), P(R[i + 1], h1), P(R[i + 1], h0), [-seg[i][0], 0, seg[i][1]], u0, u1, 0, 1);
    }
    const cap = (i, s) => add(P(L[i], h0), P(L[i], h1), P(R[i], h1), P(R[i], h0), [s * seg[Math.min(i, n - 2)][1], 0, s * seg[Math.min(i, n - 2)][0]], 0, 1, 0, 1);
    cap(0, -1); cap(n - 1, 1);
    const g = new THREE.BufferGeometry();
    g.setAttribute('position', new THREE.Float32BufferAttribute(pos, 3));
    g.setAttribute('uv', new THREE.Float32BufferAttribute(uvs, 2));
    g.computeVertexNormals();
    return g;
}

// ---- textures ----
export function canvasTexture(canvas, { srgb = true, aniso = 1, repeat = false, mip = true } = {}) {
    const t = new THREE.CanvasTexture(canvas);
    t.colorSpace = srgb ? THREE.SRGBColorSpace : THREE.NoColorSpace;
    t.anisotropy = aniso; t.generateMipmaps = mip; t.minFilter = mip ? THREE.LinearMipmapLinearFilter : THREE.LinearFilter;
    if (repeat) t.wrapS = t.wrapT = THREE.RepeatWrapping;
    return t;
}

function woodCanvas() {
    const c = document.createElement('canvas'); c.width = 512; c.height = 128;
    const g = c.getContext('2d'), holder = { seed: 0xB00D };
    g.fillStyle = '#8a5a34'; g.fillRect(0, 0, 512, 128);
    for (let i = 0; i < 90; i++) {
        const y = rngNext(holder) * 128, a = 0.05 + rngNext(holder) * 0.14, dark = rngNext(holder) > 0.45;
        g.strokeStyle = dark ? `rgba(40,20,8,${a})` : `rgba(255,214,160,${a * 0.6})`; g.lineWidth = 0.6 + rngNext(holder) * 2.2;
        g.beginPath(); g.moveTo(0, y);
        for (let x = 0; x <= 512; x += 64) g.lineTo(x, y + (rngNext(holder) - 0.5) * 5);
        g.stroke();
    }
    return c;
}

// Material set; envMap is assigned explicitly because scene.environment ignores per-material intensity in r186.
export function createMaterials({ envMap, palette, aniso = 1 }) {
    const std = (o, env = 1) => new THREE.MeshStandardMaterial({ envMap, envMapIntensity: env, ...o });
    const woodTex = canvasTexture(woodCanvas(), { aniso, repeat: true });
    const mats = {
        chrome: std({ color: '#c9d3df', metalness: 0.85, roughness: 0.42 }, 0.9),
        steel: std({ color: '#76818f', metalness: 1, roughness: 0.4 }, 0.85),
        darkMetal: std({ color: '#232832', metalness: 0.9, roughness: 0.42 }, 0.9),
        wood: new THREE.MeshPhysicalMaterial({ color: '#a8754c', map: woodTex, metalness: 0, roughness: 0.46, clearcoat: 0.7, clearcoatRoughness: 0.16, envMap, envMapIntensity: 0.7 }),
        rail: std({ color: '#ffffff', vertexColors: true, metalness: 0.88, roughness: 0.55 }, 0.34),
        plasticWall: new THREE.MeshPhysicalMaterial({ color: '#ffffff', vertexColors: true, transparent: true, opacity: 0.55, roughness: 0.12, metalness: 0, clearcoat: 1, clearcoatRoughness: 0.08, envMap, envMapIntensity: 0.7, depthWrite: false }),
        laminate: std({ color: '#0c0f16', metalness: 0.15, roughness: 0.38 }, 0.5),
        apron: std({ color: '#10141d', metalness: 0.1, roughness: 0.3 }, 0.9),
        rubberBlack: std({ color: '#0a0a0c', metalness: 0, roughness: 0.8 }, 0.1),
        glow: new THREE.MeshBasicMaterial({ color: new THREE.Color(palette.accent || '#37d9ff').multiplyScalar(1.7) }),
        glowSecondary: new THREE.MeshBasicMaterial({ color: new THREE.Color(palette.secondary || '#ff48c4').multiplyScalar(1.7) })
    };
    mats.woodTex = woodTex;
    return mats;
}

export function disposeMaterials(mats) {
    for (const v of Object.values(mats)) { if (v && v.dispose) v.dispose(); }
}
