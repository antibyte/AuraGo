// 2D geometry helpers for table layout (metres, y up). DOM-free.

// n points on a circle from a0Deg to a1Deg (inclusive), angles CCW from +x.
export function arcPoints(cx, cy, r, a0Deg, a1Deg, n) {
    const out = [];
    for (let i = 0; i < n; i++) {
        const a = (a0Deg + (a1Deg - a0Deg) * (n === 1 ? 0 : i / (n - 1))) * Math.PI / 180;
        out.push([cx + r * Math.cos(a), cy + r * Math.sin(a)]);
    }
    return out;
}

export const mirrorX = (points, cx = 0) => points.map(([x, y, ...rest]) => [2 * cx - x, y, ...rest]);

export function rotate([x, y], angle, [ox, oy] = [0, 0]) {
    const c = Math.cos(angle), s = Math.sin(angle), dx = x - ox, dy = y - oy;
    return [ox + dx * c - dy * s, oy + dx * s + dy * c];
}

export function distPointSeg(px, py, ax, ay, bx, by) {
    const dx = bx - ax, dy = by - ay, len2 = dx * dx + dy * dy;
    const t = len2 > 0 ? Math.max(0, Math.min(1, ((px - ax) * dx + (py - ay) * dy) / len2)) : 0;
    return Math.hypot(px - (ax + t * dx), py - (ay + t * dy));
}

const cross = (ax, ay, bx, by) => ax * by - ay * bx;

// Proper or touching intersection of segments (a,b) and (c,d). Returns the point or null.
export function segIntersect(a, b, c, d) {
    const rx = b[0] - a[0], ry = b[1] - a[1], sx = d[0] - c[0], sy = d[1] - c[1];
    const den = cross(rx, ry, sx, sy);
    if (Math.abs(den) < 1e-15) return null;
    const t = cross(c[0] - a[0], c[1] - a[1], sx, sy) / den;
    const u = cross(c[0] - a[0], c[1] - a[1], rx, ry) / den;
    if (t < 0 || t > 1 || u < 0 || u > 1) return null;
    return [a[0] + t * rx, a[1] + t * ry];
}

export function distSegSeg(a, b, c, d) {
    if (segIntersect(a, b, c, d)) return 0;
    return Math.min(
        distPointSeg(a[0], a[1], c[0], c[1], d[0], d[1]), distPointSeg(b[0], b[1], c[0], c[1], d[0], d[1]),
        distPointSeg(c[0], c[1], a[0], a[1], b[0], b[1]), distPointSeg(d[0], d[1], a[0], a[1], b[0], b[1]));
}

export function pointInPolygon([x, y], poly) {
    let inside = false;
    for (let i = 0, j = poly.length - 1; i < poly.length; j = i++) {
        const [xi, yi] = poly[i], [xj, yj] = poly[j];
        if ((yi > y) !== (yj > y) && x < (xj - xi) * (y - yi) / (yj - yi) + xi) inside = !inside;
    }
    return inside;
}

export function signedArea(points) {
    let a = 0;
    for (let i = 0; i < points.length; i++) {
        const [x1, y1] = points[i], [x2, y2] = points[(i + 1) % points.length];
        a += x1 * y2 - x2 * y1;
    }
    return a / 2;
}

// Rubber band around a closed polygon of posts: every edge is pushed outward by r and the
// corners are mitered, so neighbouring bands share their vertices. Returns [[p, q], ...] with
// edge i running from points[i] to points[i + 1] (wrapping).
export function bandSegments(points, r) {
    const n = points.length, sign = signedArea(points) >= 0 ? 1 : -1;
    const lines = points.map((p, i) => {
        const q = points[(i + 1) % n], dx = q[0] - p[0], dy = q[1] - p[1], len = Math.hypot(dx, dy);
        const nx = sign * dy / len, ny = -sign * dx / len;
        return { p: [p[0] + nx * r, p[1] + ny * r], d: [dx, dy] };
    });
    const corner = (l1, l2) => {
        const den = cross(l1.d[0], l1.d[1], l2.d[0], l2.d[1]);
        const t = cross(l2.p[0] - l1.p[0], l2.p[1] - l1.p[1], l2.d[0], l2.d[1]) / den;
        return [l1.p[0] + t * l1.d[0], l1.p[1] + t * l1.d[1]];
    };
    const verts = points.map((_, i) => corner(lines[(i + n - 1) % n], lines[i]));
    return points.map((_, i) => [verts[i], verts[(i + 1) % n]]);
}

// Layout object helpers with the defaults from the layout contract.
export const wall = (id, material, points, opts = {}) => ({
    id, material, points, closed: false, render: material === 'wood' || material === 'metal' || material === 'rubber' || material === 'plastic' ? material : 'none', height: 0.03, ...opts
});
export const post = (id, x, y, r, material = 'metal', render = 'post') => ({ id, x, y, r, material, render });
