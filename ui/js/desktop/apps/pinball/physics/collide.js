// Ball-versus-shape contact maths: closest point, push-out, impulse response, energy ceiling. DOM-free, allocation-free.
import { MATERIALS, BALL_R, SPIN } from './constants.js';
import { contactFriction } from './spin.js';

// Ball (x, y, r) against segment a-b. On contact fills out {nx, ny, pen, cx, cy} (normal points at the ball,
// cx/cy is the closest point on the shape) and returns true. (prx, pry) is the previous ball position; it only
// picks the side when the centre lies exactly on the segment.
export function segContact(x, y, r, ax, ay, bx, by, prx, pry, out) {
    const dx = bx - ax, dy = by - ay, l2 = dx * dx + dy * dy;
    let t = l2 > 0 ? ((x - ax) * dx + (y - ay) * dy) / l2 : 0;
    if (t < 0) t = 0; else if (t > 1) t = 1;
    const qx = ax + t * dx, qy = ay + t * dy, ex = x - qx, ey = y - qy, d2 = ex * ex + ey * ey;
    if (d2 >= r * r) return false;
    const d = Math.sqrt(d2);
    if (d > 1e-9) { out.nx = ex / d; out.ny = ey / d; } else {
        const len = Math.sqrt(l2) || 1;
        let nx = -dy / len, ny = dx / len;
        if (nx * (prx - qx) + ny * (pry - qy) < 0) { nx = -nx; ny = -ny; }
        out.nx = nx; out.ny = ny;
    }
    out.pen = r - d; out.cx = qx; out.cy = qy;
    return true;
}

// Ball (x, y, r) against a circle (cx, cy, cr). Same output as segContact.
export function circleContact(x, y, r, cx, cy, cr, out) {
    const ex = x - cx, ey = y - cy, rr = r + cr, d2 = ex * ex + ey * ey;
    if (d2 >= rr * rr) return false;
    const d = Math.sqrt(d2);
    if (d > 1e-9) { out.nx = ex / d; out.ny = ey / d; } else { out.nx = 0; out.ny = 1; }
    out.pen = rr - d; out.cx = cx + out.nx * cr; out.cy = cy + out.ny * cr;
    return true;
}

// e = vn < 0.03 ? 0 : clamp(e0 - eSlope * vn, eMin, e0)
export function restitutionOf(mat, vn) {
    if (vn < 0.03) return 0;
    const e = mat.e0 - mat.eSlope * vn;
    return e < mat.eMin ? mat.eMin : (e > mat.e0 ? mat.e0 : e);
}

// Velocity response against a surface that moves with (svx, svy). Returns the approach speed vn
// (0 when the ball is separating) measured before the response.
// Friction (tangential and vertical-roll exchange, spin.js) only acts on impacts (vn >= 0.03); resting contacts roll free
// so a ball can run down a raised bat.
export function resolveVelocity(ball, nx, ny, material, svx, svy) {
    const rvx = ball.vx - svx, rvy = ball.vy - svy;
    const vn = -(rvx * nx + rvy * ny);
    if (vn <= 0) return 0;
    const mat = MATERIALS[material] || MATERIALS.metal;
    const e = restitutionOf(mat, vn), j = (1 + e) * vn;
    ball.vx += nx * j; ball.vy += ny * j;
    if (vn >= 0.03) contactFriction(ball, nx, ny, mat.mu, j, rvx, rvy);
    return vn;
}

// Passive-energy ceiling: after a substep of contacts the specific energy (translation + rotation + slope) must not
// exceed e0. Push-out never adds energy this way: v and w are scaled down (to zero at worst).
const KR2 = 0.5 * SPIN.inertia * BALL_R * BALL_R;
export function capEnergy(ball, e0, gSlope) {
    const ke = e0 - gSlope * ball.y;
    const cur = 0.5 * (ball.vx * ball.vx + ball.vy * ball.vy) + KR2 * (ball.wx * ball.wx + ball.wy * ball.wy + ball.wz * ball.wz);
    if (cur <= ke) return;
    if (ke <= 0) { ball.vx = 0; ball.vy = 0; ball.wx = 0; ball.wy = 0; ball.wz = 0; return; }
    const s = Math.sqrt(ke / cur);
    ball.vx *= s; ball.vy *= s; ball.wx *= s; ball.wy *= s; ball.wz *= s;
}
