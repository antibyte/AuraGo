// AuraPinball apron: shaped dark brushed-metal plate (h 0.012) in front of the playfield with an accent pinstripe, chrome edge
// strips, six screw heads, a bevelled rear lip and the plunger-lane notch. The instruction cards (cabinet.js, h 0.0125) and
// the printed badge (plastics) lie on it. Geometry is in tableRoot space (X = x, Y = h, Z = -y).
import * as THREE from 'three';
import { rngNext } from '../config.js';
import { CABINET } from '../tables/kit/cabinet-dims.js';
import { box, merge, quad, canvasTexture } from './materials.js';
import { rgbCss, hexRgb } from './art-kit.js';

const X0 = -CABINET.sideIn, X1 = 0.2235, YF = CABINET.frontY, YR = -0.012, H = 0.012, H0 = 0.004;

function metalCanvas(accent, low) {
    const w = low ? 512 : 1024, h = Math.round(w * (YR - YF) / (X1 - X0)), c = document.createElement('canvas');
    c.width = w; c.height = h;
    const g = c.getContext('2d'), holder = { seed: 0xA9120 };
    const bg = g.createLinearGradient(0, 0, 0, h);
    bg.addColorStop(0, '#1a2029'); bg.addColorStop(0.5, '#222a35'); bg.addColorStop(1, '#141a22');
    g.fillStyle = bg; g.fillRect(0, 0, w, h);
    for (let i = 0; i < h * 1.6; i++) {            // brushed streaks along x
        const y = rngNext(holder) * h, a = 0.03 + rngNext(holder) * 0.09, x0 = rngNext(holder) * w * 0.4, x1 = x0 + w * (0.3 + rngNext(holder) * 0.7);
        g.strokeStyle = rngNext(holder) > 0.5 ? `rgba(210,222,240,${a})` : `rgba(0,0,0,${a * 1.4})`; g.lineWidth = 0.6 + rngNext(holder) * 1.4;
        g.beginPath(); g.moveTo(x0, y); g.lineTo(x1, y + (rngNext(holder) - 0.5) * 1.5); g.stroke();
    }
    const sheen = g.createLinearGradient(0, 0, w, 0);
    sheen.addColorStop(0.1, 'rgba(255,255,255,0)'); sheen.addColorStop(0.3, 'rgba(255,255,255,.05)'); sheen.addColorStop(0.5, 'rgba(255,255,255,0)');
    sheen.addColorStop(0.78, 'rgba(255,255,255,.04)'); sheen.addColorStop(1, 'rgba(255,255,255,0)');
    g.fillStyle = sheen; g.fillRect(0, 0, w, h);
    // accent pinstripe near the rear edge (canvas top = rear) and a fainter one near the front
    g.save(); g.shadowColor = rgbCss(accent, 1, 1); g.shadowBlur = h * 0.04;
    g.fillStyle = rgbCss(accent, 1.1); g.fillRect(w * 0.012, h * 0.095, w * 0.976, Math.max(2, h * 0.022));
    g.globalAlpha = 0.45; g.fillRect(w * 0.012, h * 0.9, w * 0.976, Math.max(1.5, h * 0.012));
    g.restore();
    return c;
}

export function buildApron({ group, mats, accent = '#37d9ff', low = false, aniso = 1 }) {
    const acc = Array.isArray(accent) ? accent : hexRgb(accent, [55, 217, 255]);
    const meshes = [], textures = [], own = [];
    const add = (geo, mat, name) => { const m = new THREE.Mesh(geo, mat); m.name = name; m.castShadow = false; m.receiveShadow = false; group.add(m); meshes.push(m); return m; };
    const tex = canvasTexture(metalCanvas(acc, low), { aniso: Math.min(4, aniso) }); textures.push(tex);
    const topMat = new THREE.MeshStandardMaterial({ map: tex, roughness: 0.42, metalness: 0.62, envMap: mats.chrome.envMap, envMapIntensity: 0.4 }); own.push(topMat);

    // Top plate: plunger-lane notch is simply the missing strip x in [X1, sideIn - 0.01] (the cabinet slab stays at h 0.004 there).
    const shape = new THREE.Shape([new THREE.Vector2(X0, YF), new THREE.Vector2(X1, YF), new THREE.Vector2(X1, YR), new THREE.Vector2(X0, YR)]);
    const top = new THREE.ShapeGeometry(shape).rotateX(-Math.PI / 2).translate(0, H, 0);
    const p = top.attributes.position, uv = top.attributes.uv;
    for (let i = 0; i < p.count; i++) uv.setXY(i, (p.getX(i) - X0) / (X1 - X0), (-p.getZ(i) - YF) / (YR - YF));
    add(top, topMat, 'apronPlate');

    // Walls (front, lane notch) and the bevelled rear lip: h 0.004 at y 0 up to 0.012 at y -0.012.
    const side = new THREE.MeshStandardMaterial({ color: '#10141c', metalness: 0.5, roughness: 0.5, envMap: mats.chrome.envMap, envMapIntensity: 0.3, side: THREE.DoubleSide }); own.push(side);
    add(merge([
        quad([X0, H0, -YF], [X1, H0, -YF], [X1, H, -YF], [X0, H, -YF]),
        quad([X1, H0, -YF], [X1, H0, 0], [X1, H, -YR], [X1, H, -YF]),
        quad([X1, H0, 0], [X0, H0, 0], [X0, H, -YR], [X1, H, -YR])
    ]), side, 'apronWalls');

    // Chrome edge strips (front, lane side, rear) and six screw heads.
    const ey = H + 0.0008, strips = [
        box((X0 + X1) / 2, ey, -(YF + 0.002), X1 - X0, 0.0016, 0.004), box(X1 - 0.002, ey, -(YF + YR) / 2, 0.004, 0.0016, YR - YF),
        box((X0 + X1) / 2, ey, -(YR - 0.002), X1 - X0, 0.0016, 0.004)
    ];
    for (const x of [-0.252, -0.012, 0.209]) for (const y of [-0.112, -0.019]) strips.push(new THREE.CylinderGeometry(0.003, 0.003, 0.0018, 12).translate(x, H + 0.0009, -y));
    add(merge(strips), mats.chrome, 'apronTrim');

    return {
        meshes, textures,
        dispose() {
            textures.forEach(t => t.dispose()); own.forEach(m => m.dispose());
            for (const m of meshes) { m.geometry.dispose(); group.remove(m); }
        }
    };
}
