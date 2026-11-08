// Flipper: solenoid-driven bat modelled as a tapered capsule with surface velocity. DOM-free.
// Local angle a (rad) runs rest -> up; the world angle is a (side > 0, bat points +x) or PI - a (side < 0).
import { FLIPPER, STEP } from './constants.js';

const STOP_MIN = 4; // rad/s: slower arrivals at a stop do not rebound, the bat just settles

export function createFlipper(def) {
    const p = { ...FLIPPER, ...def };
    const f = {
        id: def.id, key: def.key || def.id, x: def.x, y: def.y, side: def.side < 0 ? -1 : 1,
        length: p.length, rBase: p.rBase, rTip: p.rTip, rest: p.rest, up: p.up,
        alphaUp: p.alphaUp, omegaMax: p.omegaMax, alphaDown: p.alphaDown, omegaDownMax: p.omegaDownMax,
        stopRebound: p.stopRebound, restRebound: p.restRebound,
        a: p.rest, w: 0, on: false, ex: 1, ey: 0, sinB: 0, cosB: 1
    };
    f.sinB = (f.rBase - f.rTip) / f.length;
    f.cosB = Math.sqrt(1 - f.sinB * f.sinB);
    orient(f);
    return f;
}

export function orient(f) {
    const phi = f.side > 0 ? f.a : Math.PI - f.a;
    f.ex = Math.cos(phi); f.ey = Math.sin(phi);
}

export function resetFlipper(f) { f.a = f.rest; f.w = 0; f.on = false; orient(f); }

// Maximum surface speed (m/s) this bat can reach during the coming tick; used to size the substeps.
export function flipperTipSpeed(f) {
    const reach = f.length + f.rTip;
    if (f.on) return f.a >= f.up && f.w === 0 ? 0 : Math.min(f.omegaMax, Math.abs(f.w) + f.alphaUp * STEP) * reach;
    return f.a <= f.rest && f.w === 0 ? 0 : Math.min(f.omegaDownMax, Math.abs(f.w) + f.alphaDown * STEP) * reach;
}

// Advance one substep. Returns true when the bat moved.
export function advanceFlipper(f, h) {
    if (f.on) {
        if (f.a >= f.up && f.w === 0) return false;
        f.w = Math.min(f.w + f.alphaUp * h, f.omegaMax);
        f.a += f.w * h;
        if (f.a >= f.up) { f.a = f.up; f.w = f.w > STOP_MIN ? -f.stopRebound * f.w : 0; }
    } else {
        if (f.a <= f.rest && f.w === 0) return false;
        f.w = Math.max(f.w - f.alphaDown * h, -f.omegaDownMax);
        f.a += f.w * h;
        if (f.a <= f.rest) { f.a = f.rest; f.w = f.w < -STOP_MIN ? -f.restRebound * f.w : 0; }
    }
    orient(f);
    return true;
}

// Ball (bx, by, r) against the bat. On contact fills out {nx, ny, pen, svx, svy} and returns true.
// (svx, svy) is the bat's surface velocity at the contact point.
export function flipperContact(f, bx, by, r, out) {
    const dx = bx - f.x, dy = by - f.y, reach = f.length + f.rTip + r;
    if (dx * dx + dy * dy > reach * reach) return false;
    const u = dx * f.ex + dy * f.ey, vr = -dx * f.ey + dy * f.ex;
    const s = vr >= 0 ? 1 : -1, av = vr * s;
    const lam = u * f.cosB - av * f.sinB;
    let nu, nv, pen;
    if (lam >= 0 && lam <= f.length * f.cosB) {
        pen = r - (u * f.sinB + av * f.cosB - f.rBase);
        if (pen <= 0) return false;
        nu = f.sinB; nv = f.cosB;
    } else {
        const cu = lam < 0 ? 0 : f.length, cr = lam < 0 ? f.rBase : f.rTip;
        const du = u - cu, d = Math.sqrt(du * du + av * av);
        pen = r + cr - d;
        if (pen <= 0 || d < 1e-9) return false;
        nu = du / d; nv = av / d;
    }
    nv *= s;
    const nx = nu * f.ex - nv * f.ey, ny = nu * f.ey + nv * f.ex;
    out.nx = nx; out.ny = ny; out.pen = pen;
    const om = f.side * f.w;
    const qx = bx - nx * (r - pen) - f.x, qy = by - ny * (r - pen) - f.y;
    out.svx = -om * qy; out.svy = om * qx;
    return true;
}
