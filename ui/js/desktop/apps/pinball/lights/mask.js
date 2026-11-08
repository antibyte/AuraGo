// Lamp mask texture data: R insert index, G halo index, B halo strength, A 255 (plan section 3.10). DOM-free.
import { FIELD_W, FIELD_H, FIELD_X0, lampPolygon } from './shapes.js';

export const HALO_M = 0.006;

// Edge table [ax, ay, dx, dy, 1/len2] per edge, flat for speed.
function edgesOf(poly) {
    const e = new Float64Array(poly.length * 5);
    for (let i = 0, j = poly.length - 1; i < poly.length; j = i++) {
        const ax = poly[j][0], ay = poly[j][1], dx = poly[i][0] - ax, dy = poly[i][1] - ay, l = dx * dx + dy * dy;
        e[i * 5] = ax; e[i * 5 + 1] = ay; e[i * 5 + 2] = dx; e[i * 5 + 3] = dy; e[i * 5 + 4] = l ? 1 / l : 0;
    }
    return e;
}

function inside(e, x, y) {
    let c = false;
    for (let k = 0; k < e.length; k += 5) {
        const ay = e[k + 1], dy = e[k + 3], by = ay + dy;
        if ((by > y) !== (ay > y) && x < e[k] + e[k + 2] * ((y - ay) / dy)) c = !c;
    }
    return c;
}

function dist2(e, x, y, limit) {
    let best = limit;
    for (let k = 0; k < e.length; k += 5) {
        const ax = e[k], ay = e[k + 1], dx = e[k + 2], dy = e[k + 3];
        let t = ((x - ax) * dx + (y - ay) * dy) * e[k + 4];
        t = t < 0 ? 0 : t > 1 ? 1 : t;
        const ex = x - ax - dx * t, ey = y - ay - dy * t, d = ex * ex + ey * ey;
        if (d < best) best = d;
    }
    return best;
}

// Only insert-mounted lamps are painted (bumper/toy/backbox lamps live in 3D). Each lamp rasterises its own bounding box.
export function buildLampMask(layout, opts = {}) {
    const width = opts.width || 1024, height = opts.height || 2048;
    const data = new Uint8Array(width * height * 4);
    for (let i = 3; i < data.length; i += 4) data[i] = 255;
    const lamps = (layout && layout.lamps) || [];
    const sx = width / FIELD_W, sy = height / FIELD_H, halo2 = HALO_M * HALO_M;
    for (let li = 0; li < lamps.length; li++) {
        const lamp = lamps[li];
        if (lamp.mount && lamp.mount !== 'insert') continue;
        const index = li + 1, poly = lampPolygon(lamp), edges = edgesOf(poly);
        let x0 = Infinity, x1 = -Infinity, y0 = Infinity, y1 = -Infinity;
        for (const p of poly) { x0 = Math.min(x0, p[0]); x1 = Math.max(x1, p[0]); y0 = Math.min(y0, p[1]); y1 = Math.max(y1, p[1]); }
        const px0 = Math.max(0, Math.floor((x0 - HALO_M - FIELD_X0) * sx) - 1), px1 = Math.min(width - 1, Math.ceil((x1 + HALO_M - FIELD_X0) * sx) + 1);
        const py0 = Math.max(0, Math.floor((y0 - HALO_M) * sy) - 1), py1 = Math.min(height - 1, Math.ceil((y1 + HALO_M) * sy) + 1);
        for (let py = py0; py <= py1; py++) {
            const y = ((py + 0.5) / height) * FIELD_H;
            for (let px = px0; px <= px1; px++) {
                const o = (py * width + px) * 4;
                if (data[o] === index) continue;
                const x = ((px + 0.5) / width) * FIELD_W + FIELD_X0;
                if (inside(edges, x, y)) { data[o] = index; data[o + 1] = 0; data[o + 2] = 0; continue; }
                if (data[o] !== 0) continue;
                const d2 = dist2(edges, x, y, halo2);
                if (d2 >= halo2) continue;
                const k = 1 - Math.sqrt(d2) / HALO_M, s = Math.round(255 * k * k);
                if (s > data[o + 2]) { data[o + 1] = index; data[o + 2] = s; }
            }
        }
    }
    return { width, height, data };
}
