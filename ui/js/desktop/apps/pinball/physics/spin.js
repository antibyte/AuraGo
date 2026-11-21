// Ball spin: angular velocity (wx, wy, wz) in the playfield frame (x right, y up the table, z out of the playfield),
// floor friction (sliding -> rolling) and wall contact friction. DOM-free, allocation-free.
//
// Conventions. I = K m R^2 (K = SPIN.inertia = 0.4). The ball centre is R above the floor, so the floor contact point
// has the velocity  v + w x (0, 0, -R) = (vx - R wy, vy + R wx). Rolling on a floor that moves with s means
// vx - R wy = sx and vy + R wx = sy, i.e. wy = (vx - sx)/R and wx = -(vy - sy)/R.
//
// Floor friction (a force F = -mu N u/|u| at the contact point, u = contact slip):
//   dv = F/m,  dw = r x F / I with r = (0, 0, -R)  ->  dwx = dv.y/(K R),  dwy = -dv.x/(K R)
//   du = dv + (-R dwy, R dwx) = (1 + 1/K) dv, so one step of friction changes the slip by (1 + 1/K) mu g_n h and
//   zeroing the slip takes dv = -u K/(1+K) (2/7 of u).
//   Invariant: vx + K R wy does not change under floor friction alone (dvx + K R dwy = dvx - dvx = 0). A sliding ball
//   without spin therefore ends rolling with vx (1 + K) = v0, i.e. at 1/(1+K) = 5/7 of its speed.
//
// Wall contact (normal n pointing at the ball, t = (-ny, nx), contact point -R n). Its velocity is
//   along t:  v.t - R wz        vertical (z):  R wt,  wt = w.t = -wx ny + wy nx
// A friction impulse P = Pt t + Pz z (per unit mass jt, jz) at r = -R n gives r x P = -R Pt z + R Pz t, so
//   dv_t = jt-term, dwz = -Pt/(K m R), dw_t = Pz/(K m R); the ball centre does not move in z.
//   Slip along t changes by Pt (1 + 1/K)/m: it is zero for |Pt|/m = |vt| K/(1+K)  (jtStop).
//   Vertical slip changes by Pz/(K m): it is zero for |Pz|/m = K R |wt|  (jzStop).
// Both are scaled by one Coulomb factor min(1, mu j / |(jtStop, jzStop)|), j = the normal impulse per unit mass.
import { BALL_R, G_NORMAL, SPIN } from './constants.js';

const R = BALL_R, K = SPIN.inertia, INV_K = 1 / K, STOP = K / (1 + K);

// Specific energy (J/kg): translation + rotation + slope potential.
export function specificEnergy(b, gSlope) {
    return 0.5 * (b.vx * b.vx + b.vy * b.vy) + 0.5 * K * R * R * (b.wx * b.wx + b.wy * b.wy + b.wz * b.wz) + gSlope * b.y;
}

export function clearSpin(b) { b.wx = 0; b.wy = 0; b.wz = 0; }

// Rolling spin for the velocity (vx, vy) on a static floor; wz is left alone.
export function setRolling(b, vx, vy) { b.wx = -vy / R; b.wy = vx / R; }

// Floor friction for one substep, after gravity and before the position update.
// floor = null (static field) or { svx, svy, mu } (a moving disc, S3). decel is the rolling resistance (m/s^2).
export function floorStep(b, h, decel, floor) {
    const svx = floor ? floor.svx : 0, svy = floor ? floor.svy : 0, mu = floor ? floor.mu : SPIN.floorMu;
    const ux = b.vx - R * b.wy - svx, uy = b.vy + R * b.wx - svy;
    const ul = Math.sqrt(ux * ux + uy * uy), fd = mu * G_NORMAL * h;
    if (ul <= (1 + INV_K) * fd) {
        b.vx -= STOP * ux; b.vy -= STOP * uy;
        b.wx = -(b.vy - svy) / R; b.wy = (b.vx - svx) / R;
    } else {
        const dvx = -fd * ux / ul, dvy = -fd * uy / ul;
        b.vx += dvx; b.vy += dvy; b.wx += INV_K * dvy / R; b.wy -= INV_K * dvx / R;
    }
    const rx = b.vx - svx, ry = b.vy - svy, rl = Math.sqrt(rx * rx + ry * ry);
    if (rl > 0) {
        const k = Math.max(0, rl - decel * h) / rl;
        b.vx = svx + rx * k; b.vy = svy + ry * k; b.wx *= k; b.wy *= k;
    }
    const dz = SPIN.zDecel * h;
    b.wz = b.wz > dz ? b.wz - dz : (b.wz < -dz ? b.wz + dz : 0);
    const m = SPIN.maxW;
    if (b.wx > m) b.wx = m; else if (b.wx < -m) b.wx = -m;
    if (b.wy > m) b.wy = m; else if (b.wy < -m) b.wy = -m;
    if (b.wz > m) b.wz = m; else if (b.wz < -m) b.wz = -m;
}

// Friction impulse of an impact. j is the normal impulse per unit mass, (rvx, rvy) the pre-impact velocity of the ball
// relative to the surface.
export function contactFriction(b, nx, ny, mu, j, rvx, rvy) {
    const tx = -ny, ty = nx;
    const vt = rvx * tx + rvy * ty - R * b.wz, wt = -b.wx * ny + b.wy * nx;
    const jtStop = (vt < 0 ? -vt : vt) * STOP, jzStop = K * R * (wt < 0 ? -wt : wt);
    const mag = Math.sqrt(jtStop * jtStop + jzStop * jzStop);
    if (mag < 1e-12) return;
    const scale = Math.min(1, mu * j / mag);
    const jt = jtStop * scale * (vt < 0 ? -1 : 1), jz = jzStop * scale * (wt < 0 ? -1 : 1), kw = jz / (K * R);
    b.vx -= jt * tx; b.vy -= jt * ty; b.wz += jt / (K * R);
    b.wx -= kw * tx; b.wy -= kw * ty;
}
