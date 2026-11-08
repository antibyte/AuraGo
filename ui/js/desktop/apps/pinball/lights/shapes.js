// Lamp outlines and the one shared playfield texture mapping (plan sections 1 and 3.10). DOM-free.
export const FIELD_W = 0.515;
export const FIELD_H = 1.07;
export const FIELD_X0 = -0.2575;

// Playfield metres -> [u, v]; u = (x + 0.2575) / 0.515, v = y / 1.07.
export function toUV(x, y) { return [(x - FIELD_X0) / FIELD_W, y / FIELD_H]; }
// DataTexture texel coordinates (flipY=false, row 0 at v=0), continuous.
export function texelOf(x, y, w, h) { const t = toUV(x, y); return [t[0] * w, t[1] * h]; }
// Art canvas pixel (row 0 = top, flipY=true), continuous.
export function canvasPx(x, y, W, H) { const t = toUV(x, y); return [t[0] * W, (1 - t[1]) * H]; }
// Inverse of toUV.
export function fromUV(u, v) { return [u * FIELD_W + FIELD_X0, v * FIELD_H]; }

function ellipse(a, b, seg) {
    const out = [];
    for (let i = 0; i < seg; i++) { const t = (i / seg) * Math.PI * 2; out.push([Math.cos(t) * a, Math.sin(t) * b]); }
    return out;
}

function roundRect(a, b, seg) {
    const r = Math.min(a, b) * 0.5, n = Math.max(2, Math.round(seg / 4)), out = [];
    const corners = [[a - r, b - r, 0], [-a + r, b - r, 1], [-a + r, -b + r, 2], [a - r, -b + r, 3]];
    for (const [cx, cy, q] of corners) for (let i = 0; i <= n; i++) {
        const t = (q + i / n) * Math.PI / 2;
        out.push([cx + Math.cos(t) * r, cy + Math.sin(t) * r]);
    }
    return out;
}

function star(a, b) {
    const out = [];
    for (let i = 0; i < 10; i++) {
        const t = Math.PI / 2 + (i * Math.PI) / 5, k = i % 2 ? 0.45 : 1;
        out.push([Math.cos(t) * a * k, Math.sin(t) * b * k]);
    }
    return out;
}

// Unrotated outline in a w x h box, pointing +y.
function localShape(shape, a, b, seg) {
    switch (shape) {
    case 'oval': case 'circle': return ellipse(a, b, seg);
    case 'triangle': return [[0, b], [-a, -b], [a, -b]];
    case 'arrow': return [[0, b], [a, 0], [0.4 * a, 0], [0.4 * a, -b], [-0.4 * a, -b], [-0.4 * a, 0], [-a, 0]];
    case 'rect': return [[a, b], [-a, b], [-a, -b], [a, -b]];
    case 'roundRect': return roundRect(a, b, seg);
    case 'diamond': return [[0, b], [-a, 0], [0, -b], [a, 0]];
    case 'star': return star(a, b);
    case 'shield': return [[-a, b], [a, b], [a, 0], [0.6 * a, -0.6 * b], [0, -b], [-0.6 * a, -0.6 * b], [-a, 0]];
    case 'chevron': return [[0, b], [-a, 0], [-a, -b], [0, 0], [a, -b], [a, 0]];
    default: return ellipse(a, b, seg);
    }
}

export function polygonArea(pts) {
    let s = 0;
    for (let i = 0; i < pts.length; i++) { const p = pts[i], q = pts[(i + 1) % pts.length]; s += p[0] * q[1] - q[0] * p[1]; }
    return s / 2;
}

export function polygonCentroid(pts) {
    let cx = 0, cy = 0, s = 0;
    for (let i = 0; i < pts.length; i++) {
        const p = pts[i], q = pts[(i + 1) % pts.length], c = p[0] * q[1] - q[0] * p[1];
        s += c; cx += (p[0] + q[0]) * c; cy += (p[1] + q[1]) * c;
    }
    return s ? [cx / (3 * s), cy / (3 * s)] : [0, 0];
}

// Lamp outline in playfield metres: array of [x, y] vertices (counter-clockwise, implicitly closed, the first vertex is
// not repeated). The area centroid sits on (lamp.x, lamp.y). angle 0 points +y, positive angles turn counter-clockwise.
export function lampPolygon(lamp, seg = 24) {
    const a = (lamp.w || 0.016) / 2, b = (lamp.h || lamp.w || 0.016) / 2;
    let pts = localShape(lamp.shape, a, b, Math.max(8, seg | 0));
    if (polygonArea(pts) < 0) pts = pts.slice().reverse();
    const c = polygonCentroid(pts), ca = Math.cos(lamp.angle || 0), sa = Math.sin(lamp.angle || 0);
    return pts.map(p => {
        const x = p[0] - c[0], y = p[1] - c[1];
        return [lamp.x + x * ca - y * sa, lamp.y + x * sa + y * ca];
    });
}
