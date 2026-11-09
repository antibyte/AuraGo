// AuraPinball mechanism geometry factories (plan 8 / T13). Everything is built in tableRoot space: X = x, Y = h, Z = -y.
// Moving parts are created around their own origin; the caller positions them. Static helpers return world-placed pieces.
import * as THREE from 'three';
import { merge } from './materials.js';
import { FONT_STACK } from '../config.js';

export const v3 = (x, y, h = 0) => new THREE.Vector3(x, h, -y);

const lathe = (pts, seg) => new THREE.LatheGeometry(pts.map(([r, y]) => new THREE.Vector2(r, y)), seg);
const cyl = (r0, r1, h, seg, y = 0) => new THREE.CylinderGeometry(r1, r0, h, seg).translate(0, y + h / 2, 0);

// ---- flipper: tapered capsule (matches physics rBase / rTip / length), extruded into a bat ----
function batShape(L, rB, rT, seg) {
    const sinB = (rB - rT) / L, th = Math.atan2(Math.sqrt(1 - sinB * sinB), sinB), cosB = Math.sin(th);
    const s = new THREE.Shape();
    s.moveTo(rB * sinB, rB * cosB);
    s.lineTo(L + rT * sinB, rT * cosB);
    s.absarc(L, 0, rT, th, -th, true);
    s.lineTo(rB * sinB, -rB * cosB);
    s.absarc(0, 0, rB, -th, th - Math.PI * 2, true);
    s.curveSegments = seg;
    return s;
}
const flat = g => g.rotateX(-Math.PI / 2);             // shape (u, v, depth) -> (x = u, y = depth, z = -v)

export function flipperGeometry(p, low) {
    const seg = low ? 8 : 20;
    const rubber = flat(new THREE.ExtrudeGeometry(batShape(p.length, p.rBase, p.rTip, seg), { depth: 0.0075, bevelEnabled: false, curveSegments: seg })).translate(0, 0.0015, 0);
    const inset = 0.0017;
    const top = flat(new THREE.ExtrudeGeometry(batShape(p.length, p.rBase - inset, p.rTip - inset, seg),
        { depth: 0.0026, bevelEnabled: !low, bevelSize: 0.0006, bevelThickness: 0.0007, bevelSegments: 2, curveSegments: seg })).translate(0, 0.0088, 0);
    return { rubber, top };
}
export function pivotCap(x, y, low) {
    const seg = low ? 10 : 20;
    const cap = merge([cyl(0.0058, 0.0058, 0.0035, seg, 0.0122), new THREE.TorusGeometry(0.0058, 0.0007, 5, seg).rotateX(Math.PI / 2).translate(0, 0.0158, 0)]);
    const dot = new THREE.CircleGeometry(0.0028, low ? 8 : 16).rotateX(-Math.PI / 2).translate(0, 0.0159, 0);
    cap.translate(x, 0, -y); dot.translate(x, 0, -y);
    return { cap, dot };
}

// ---- rubber band between two playfield points (elliptic cross-section, slightly squashed) ----
export function bandGeometry(a, b, low, shift = null) {
    const L = Math.hypot(b[0] - a[0], b[1] - a[1]), rho = 0.0032;
    const g = new THREE.CapsuleGeometry(rho, Math.max(0.0005, L), low ? 2 : 4, low ? 6 : 10);
    g.rotateZ(-Math.PI / 2).scale(1, 2.35, 1);
    g.rotateY(Math.atan2(b[1] - a[1], b[0] - a[0]));
    const m = [(a[0] + b[0]) / 2, (a[1] + b[1]) / 2];
    g.translate(m[0] + (shift ? shift[0] : 0), 0.0105, -(m[1] + (shift ? shift[1] : 0)));
    return g;
}

// Post pieces: rubber ring and chrome core + cap (or a bare chrome post), placed in world space.
export function postGeometry(p, low) {
    const seg = low ? 8 : 14, rubber = p.material === 'rubber';
    if (rubber) return {
        rubber: cyl(p.r, p.r, 0.0135, seg, 0.0012).translate(p.x, 0, -p.y),
        chrome: merge([cyl(p.r * 0.5, p.r * 0.5, 0.0185, seg), cyl(p.r * 0.78, p.r * 0.78, 0.0016, seg, 0.0185)]).translate(p.x, 0, -p.y)
    };
    return { chrome: merge([cyl(p.r, p.r, 0.020, seg), cyl(p.r * 1.3, p.r * 1.3, 0.0016, seg, 0.020), cyl(p.r * 1.45, p.r * 1.45, 0.0012, seg)]).translate(p.x, 0, -p.y) };
}

// ---- pop bumper (around its own origin) ----
export function bumperGeometry(r, low) {
    const seg = low ? 14 : 32;
    return {
        base: lathe([[0, 0], [r * 1.2, 0], [r * 1.2, 0.0018], [r * 1.1, 0.0034], [0, 0.0034]], seg),
        ring: lathe([[r * 0.98, 0], [r * 1.12, 0], [r * 1.12, 0.0022], [r * 1.05, 0.0033], [r * 0.98, 0.0033]], seg).translate(0, 0.0035, 0),
        skirt: lathe([[r * 0.96, 0], [r * 0.96, 0.004], [r * 0.88, 0.0095], [r * 0.7, 0.0123], [0, 0.0123]], seg).translate(0, 0.0069, 0),
        cap: lathe([[r * 0.66, 0], [r * 0.67, 0.0045], [r * 0.62, 0.0095], [r * 0.5, 0.0138], [r * 0.3, 0.0168], [0.0015, 0.0178]], seg).translate(0, 0.0192, 0),
        stud: merge([cyl(r * 0.2, r * 0.2, 0.0016, low ? 8 : 14, 0.0364), cyl(r * 0.67, r * 0.67, 0.0022, seg, 0.0185)])
    };
}

// ---- target face atlas: one canvas, one material for every drop target and standup ----
export function createAtlas(items) {
    const cols = 4, rows = Math.max(1, Math.ceil(items.length / cols)), W = cols * 128, H = rows * 128;
    const c = document.createElement('canvas'); c.width = W; c.height = H;
    const g = c.getContext('2d'), cells = new Map();
    items.forEach((it, i) => {
        const cx = (i % cols) * 128, cy = Math.floor(i / cols) * 128;
        cells.set(it.key, { rect: [cx / W, 1 - (cy + 112) / H, (cx + 112) / W, 1 - cy / H], edge: [(cx + 120) / W, 1 - (cy + 120) / H] });
        const col = new THREE.Color(it.color || '#ff4f5e');
        const base = '#' + col.clone().multiplyScalar(0.82).getHexString(), hi = '#' + col.clone().lerp(new THREE.Color('#ffffff'), 0.45).getHexString();
        const gr = g.createLinearGradient(cx, cy, cx, cy + 112);
        gr.addColorStop(0, hi); gr.addColorStop(0.18, base); gr.addColorStop(1, '#' + col.clone().multiplyScalar(0.5).getHexString());
        g.fillStyle = gr; g.fillRect(cx, cy, 112, 112);
        g.strokeStyle = 'rgba(255,255,255,.85)'; g.lineWidth = 4; g.strokeRect(cx + 7, cy + 7, 98, 98);
        g.fillStyle = 'rgba(0,0,0,.18)'; g.fillRect(cx + 7, cy + 80, 98, 25);
        if (it.label) {
            g.font = `800 ${String(it.label).length > 1 ? 40 : 66}px ${FONT_STACK}`; g.textAlign = 'center'; g.textBaseline = 'middle';
            g.lineWidth = 6; g.strokeStyle = 'rgba(0,0,0,.55)'; g.strokeText(String(it.label), cx + 56, cy + 58);
            g.fillStyle = '#fffdf2'; g.fillText(String(it.label), cx + 56, cy + 58);
        }
        g.fillStyle = '#e6e2d6'; g.fillRect(cx + 112, cy, 16, 128); g.fillRect(cx, cy + 112, 128, 16);
    });
    const t = new THREE.CanvasTexture(c); t.colorSpace = THREE.SRGBColorSpace; t.anisotropy = 4;
    return { texture: t, cells };
}

// Box whose +X face carries a face-atlas rect and every other side the plain edge colour; front plane sits at local x = 0.
export function faceBox(w, h, t, cell) {
    const g = new THREE.BoxGeometry(t, h, w).translate(-t / 2, h / 2, 0), uv = g.attributes.uv, per = uv.count / 6;
    for (let i = 0; i < uv.count; i++) {
        if (i < per) uv.setXY(i, cell.rect[0] + uv.getX(i) * (cell.rect[2] - cell.rect[0]), cell.rect[1] + uv.getY(i) * (cell.rect[3] - cell.rect[1]));
        else uv.setXY(i, cell.edge[0], cell.edge[1]);
    }
    return g;
}

// ---- spinner: rod + brackets (static, axis along local X) and the plate (rotates about X) ----
export function spinnerParts(len, low) {
    const seg = low ? 5 : 8, ry = 0.023;
    const frame = merge([
        new THREE.CylinderGeometry(0.0011, 0.0011, len, seg).rotateZ(Math.PI / 2).translate(0, ry, 0),
        cyl(0.0018, 0.0018, ry + 0.004, seg).translate(-len / 2, 0, 0), cyl(0.0018, 0.0018, ry + 0.004, seg).translate(len / 2, 0, 0),
        cyl(0.0034, 0.0034, 0.0012, seg).translate(-len / 2, 0, 0), cyl(0.0034, 0.0034, 0.0012, seg).translate(len / 2, 0, 0)
    ]);
    const plate = merge([new THREE.BoxGeometry(len * 0.74, 0.0195, 0.0011), new THREE.BoxGeometry(len * 0.74, 0.0014, 0.0034).translate(0, 0.0091, 0), new THREE.BoxGeometry(len * 0.74, 0.0014, 0.0034).translate(0, -0.0091, 0)]);
    return { frame, plate, ry };
}

// ---- rollover wire (flat oval loop), unit size, instanced by the caller ----
export function rolloverGeometry(r, low) {
    return new THREE.TorusGeometry(r * 0.62, 0.0007, 4, low ? 10 : 18).rotateX(-Math.PI / 2).scale(0.7, 1, 1.9);
}

// ---- holes ----
export function holeParts(def, low) {
    const r = def.r ?? 0.016, seg = low ? 14 : 28, kind = def.kind || 'saucer';
    const dark = new THREE.CircleGeometry(r * 0.98, seg).rotateX(-Math.PI / 2).translate(0, 0.0011, 0);
    const rim = new THREE.TorusGeometry(r + 0.0006, 0.0017, 6, seg).rotateX(Math.PI / 2).translate(0, 0.0017, 0);
    const extra = [];
    if (kind === 'scoop') {
        extra.push(new THREE.CylinderGeometry(r + 0.003, r + 0.003, 0.018, seg, 1, true, Math.PI / 2, Math.PI).translate(0, 0.009, 0));
        extra.push(new THREE.BoxGeometry(2 * (r + 0.003), 0.0012, 0.0045).translate(0, 0.018, -(r + 0.0015)));
    } else if (kind === 'vuk') extra.push(new THREE.CylinderGeometry(r + 0.0014, r + 0.0014, 0.007, seg, 1, true).translate(0, 0.0035, 0));
    else if (kind === 'kickhole') extra.push(new THREE.TorusGeometry(r + 0.0032, 0.0011, 5, seg).rotateX(Math.PI / 2).translate(0, 0.0013, 0));
    return { dark, rim, shell: extra.length ? merge(extra) : null };
}

// Triangle (sling plastic): inset polygon extruded flat.
export function polygonPlate(pts, depth, bevel, low) {
    const s = new THREE.Shape(pts.map(p => new THREE.Vector2(p[0], p[1])));
    const g = new THREE.ExtrudeGeometry(s, { depth, bevelEnabled: !low && bevel > 0, bevelSize: bevel, bevelThickness: bevel, bevelSegments: 2 });
    return g.rotateX(-Math.PI / 2);
}
export function insetPolygon(pts, k) {
    const cx = pts.reduce((a, p) => a + p[0], 0) / pts.length, cy = pts.reduce((a, p) => a + p[1], 0) / pts.length;
    return pts.map(p => { const dx = p[0] - cx, dy = p[1] - cy, l = Math.hypot(dx, dy) || 1; return [p[0] - dx / l * k, p[1] - dy / l * k]; });
}

// Plunger parts around the tip origin: rod toward +Z (down the table), rubber tip, coiled spring of unit length along +Z.
export function plungerParts(low) {
    const seg = low ? 8 : 14;
    const rod = new THREE.CylinderGeometry(0.0035, 0.0035, 0.09, seg).rotateX(Math.PI / 2).translate(0, 0, 0.045);
    const tip = new THREE.CylinderGeometry(0.0072, 0.0072, 0.007, seg).rotateX(Math.PI / 2).translate(0, 0, 0.0035);
    const turns = 11, pts = [];
    for (let i = 0; i <= turns * 12; i++) { const a = i / 12 * Math.PI * 2; pts.push(new THREE.Vector3(Math.cos(a) * 0.0068, Math.sin(a) * 0.0068, i / (turns * 12))); }
    const spring = new THREE.TubeGeometry(new THREE.CatmullRomCurve3(pts), turns * (low ? 6 : 10), 0.00085, 4, false);
    return { rod, tip, spring };
}

// Printed sling plastic: a colour gradient with a neon edge line and a light sheen, mapped planar over the plate bounds.
export function slingBounds(pts) {
    const xs = pts.map(p => p[0]), ys = pts.map(p => p[1]);
    return { x0: Math.min(...xs), y0: Math.min(...ys), w: Math.max(...xs) - Math.min(...xs) || 1, h: Math.max(...ys) - Math.min(...ys) || 1 };
}
export function planarUV(geo, b) {
    const p = geo.attributes.position, uv = new Float32Array(p.count * 2);
    for (let i = 0; i < p.count; i++) { uv[2 * i] = (p.getX(i) - b.x0) / b.w; uv[2 * i + 1] = (-p.getZ(i) - b.y0) / b.h; }
    geo.setAttribute('uv', new THREE.BufferAttribute(uv, 2));
    return geo;
}
export function slingArtTexture(tri, b, secondary, accent) {
    const S = 256, c = document.createElement('canvas'); c.width = c.height = S;
    const g = c.getContext('2d'), P = tri.map(q => [(q[0] - b.x0) / b.w * S, (1 - (q[1] - b.y0) / b.h) * S]);
    const path = () => { g.beginPath(); P.forEach((q, i) => (i ? g.lineTo(q[0], q[1]) : g.moveTo(q[0], q[1]))); g.closePath(); };
    const gr = g.createLinearGradient(0, 0, S, S);
    gr.addColorStop(0, '#12051d'); gr.addColorStop(0.55, secondary); gr.addColorStop(1, '#ffd0ef');
    g.fillStyle = gr; path(); g.fill();
    g.save(); path(); g.clip(); g.globalAlpha = 0.28; g.strokeStyle = '#ffffff'; g.lineWidth = 7;
    for (let i = -2; i < 6; i++) { g.beginPath(); g.moveTo(i * 60, S); g.lineTo(i * 60 + S, 0); g.stroke(); }
    g.restore();
    const cx = P.reduce((a, q) => a + q[0], 0) / 3, cy = P.reduce((a, q) => a + q[1], 0) / 3;
    g.shadowColor = accent; g.shadowBlur = 10; g.strokeStyle = accent; g.lineWidth = 5; g.lineJoin = 'round';
    g.beginPath(); P.forEach((q, i) => { const x = cx + (q[0] - cx) * 0.8, y = cy + (q[1] - cy) * 0.8; if (i) g.lineTo(x, y); else g.moveTo(x, y); }); g.closePath(); g.stroke();
    g.shadowBlur = 0; g.strokeStyle = '#ffffff'; g.lineWidth = 1.6; g.globalAlpha = 0.85; g.stroke();
    const t = new THREE.CanvasTexture(c); t.colorSpace = THREE.SRGBColorSpace; t.anisotropy = 4;
    return t;
}
