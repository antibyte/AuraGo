// Moving playfield devices: the orrery disc (a motor-driven turntable set flush in the playfield) and the pendulum
// (a bob on a rod swinging in the vertical plane across a lane). DOM-free, deterministic, JSON state only.
//
// Both factories take (def, ctx, clk) and return a device object (see devices.js / devices-holes.js):
//   { sub(h, balls, n), post(), cmd(cmd) -> bool, reset(), remove(ballId), clear(), anim(a), dbg() }
//
// ORRERY  def {id, type: 'orrery', x, y, r = 0.05, speed = 4, maxSpeed = 18, accel = 20, mu = SPIN.discMu}
//   speed is an angular speed in rad/s, + = counter-clockwise seen from above. The disc is a moving floor (ctx.addFloor,
//   world.js): a ball whose centre is over it feels floor friction towards the surface velocity
//   (-w (y - cy), w (x - cx)), so it is dragged, curls and spirals out. A ball on the disc is exempt from the passive
//   energy ceiling (the motor may add energy); the surface speed bounds it, the largest being w r = 0.9 m/s.
//   Coils off (tilt) spins the disc down to 0. post() runs once per tick: w moves towards the target by accel * STEP.
//   Events: 'orrery' {dir: 1} once per entry (leaving needs 3 mm beyond the rim, so rim wobble does not re-fire).
//   Command {type: 'orrery', id, speed} sets the target speed, clamped to +-maxSpeed. Anim slot: disc angle (rad,
//   wrapped to (-pi, pi], listed in the snapshot `spin` array for shortest-delta interpolation).
//
// PENDULUM  def {id, type: 'pendulum', pivot: [x, y], pivotH = 0.0885, length = 0.075, bobR = 0.011, axis: [1, 0],
//                period = 1.8, fastPeriod = 1.15, amp = 0.42, park = 1.10, mode = 'swing', material = 'metal'}
//   Swing angle theta (rad, 0 = hanging straight down, + towards `axis`):  theta = O + A sin(phase)  with the amplitude A
//   and the offset O ramping to the mode's target at 1.5 rad/s. swing and fast -> (amp, 0), park -> (0, park),
//   coils off -> (0, 0) so it hangs. The phase advances 2 pi / period per second from the substep length h (period is
//   fastPeriod in fast mode), never from a wall clock, so a mode change is continuous.
//   Bob maths. The bob centre is at height  hBob = pivotH - L cos(theta)  above the playfield and at the plan position
//   pivot + axis L sin(theta); its plan speed is  axis L cos(theta) dtheta/dt. A ball centre is BALL_R high. Two
//   spheres touch when their 3D distance is below rr = BALL_R + bobR; with the height difference dh = hBob - BALL_R the
//   plan distance at touch is  reff = sqrt(rr^2 - dh^2)  and there is no contact when |dh| >= rr.
//   With the defaults: hanging, hBob = 13.5 mm = BALL_R, dh = 0, reff = 24.5 mm; a ball is touched while
//   cos(theta) > (pivotH - BALL_R - rr) / L = 0.673, i.e. |theta| < 0.83 rad; parked at 1.10 rad, hBob = 54.5 mm,
//   dh = 41 mm and the bob clears every ball by 16.5 mm. A contact pushes the ball out to reff along the plan normal
//   (never left inside the bob) and runs the impulse through resolveVelocity against the bob's surface velocity.
//   Events: 'pendulum' {speed} (approach speed, > 0.15 m/s, at most one per 0.12 s). Command {type: 'pendulum', id,
//   mode: 'park' | 'swing' | 'fast'}. Anim slot: theta. If the bob hangs still (coils off) a ball squeezed between it
//   and a wall stays until the ball search kicks it, so layouts keep the bob plane clear of wall ends.
import { STEP, BALL_R, SPIN } from './constants.js';
import { resolveVelocity } from './collide.js';

const TWO_PI = Math.PI * 2;
const EXIT_HYST = 0.003;            // m beyond the rim before a ball counts as having left the disc
const ARM_RATE = 1.5;               // rad/s: how fast the pendulum amplitude and offset ramp
const HIT_SPEED = 0.15, HIT_GAP = 0.12;
const MODES = ['swing', 'fast', 'park'];

const toward = (value, target, step) => {
    const d = target - value;
    return d > step ? step : (d < -step ? -step : d);
};

// ---------------------------------------------------------------------------------------------------------- orrery
export function createOrrery(def, ctx, clk) {
    const id = def.id, cx = def.x, cy = def.y, r = def.r ?? 0.05, r2 = r * r, rOut = r + EXIT_HYST, rOut2 = rOut * rOut;
    const maxSpeed = def.maxSpeed ?? 18, accel = def.accel ?? 20, mu = def.mu ?? SPIN.discMu;
    const clampSpeed = s => { const v = Number.isFinite(+s) ? +s : 0; return v > maxSpeed ? maxSpeed : (v < -maxSpeed ? -maxSpeed : v); };
    let target = clampSpeed(def.speed ?? 4), omega = target, angle = 0;
    const inside = [], seen = [];

    ctx.addFloor({
        contains: (x, y) => (x - cx) * (x - cx) + (y - cy) * (y - cy) < r2,
        velocity(x, y, out) {
            out.svx = -omega * (y - cy); out.svy = omega * (x - cx); out.mu = mu; out.moving = omega !== 0;
            return out;
        }
    });

    return {
        sub(h, balls, n) {
            seen.length = 0;
            for (let i = 0; i < n; i++) {
                const b = balls[i], dx = b.x - cx, dy = b.y - cy, d2 = dx * dx + dy * dy, k = inside.indexOf(b.id);
                if (k < 0) {
                    if (d2 < r2) { inside.push(b.id); seen.push(b.id); ctx.emit('orrery', id, b, { dir: 1 }); }
                } else if (d2 < rOut2) seen.push(b.id);
            }
            for (let k = inside.length - 1; k >= 0; k--) if (seen.indexOf(inside[k]) < 0) inside.splice(k, 1);
        },
        post() {
            const want = ctx.coils ? target : 0;
            omega += toward(omega, want, accel * STEP);
            angle += omega * STEP;
            if (angle > Math.PI) angle -= TWO_PI; else if (angle <= -Math.PI) angle += TWO_PI;
        },
        cmd(c) {
            if (c.type !== 'orrery') return false;
            target = clampSpeed(c.speed);
            return true;
        },
        reset() { target = clampSpeed(def.speed ?? 4); omega = target; angle = 0; inside.length = 0; },
        remove(ballId) { const k = inside.indexOf(ballId); if (k >= 0) inside.splice(k, 1); },
        clear() { inside.length = 0; },
        anim(a) { a[this.slot] = angle; },
        dbg: () => ({ id, type: 'orrery', x: cx, y: cy, r, omega, target })
    };
}

// ------------------------------------------------------------------------------------------------------- pendulum
export function createPendulum(def, ctx, clk) {
    const id = def.id, px = def.pivot[0], py = def.pivot[1], pivotH = def.pivotH ?? 0.0885, L = def.length ?? 0.075, bobR = def.bobR ?? 0.011;
    const axis = def.axis || [1, 0], al = Math.hypot(axis[0], axis[1]) || 1, ax = axis[0] / al, ay = axis[1] / al;
    const period = def.period ?? 1.8, fastPeriod = def.fastPeriod ?? 1.15, amp = def.amp ?? 0.42, park = def.park ?? 1.10;
    const material = def.material ?? 'metal', rr = BALL_R + bobR;
    const modeOf = () => MODES.includes(def.mode) ? def.mode : 'swing';
    let mode = modeOf(), phase = 0, A = 0, O = 0, theta = 0, thetaDot = 0, lastHit = -10;

    const goalA = () => ctx.coils && mode !== 'park' ? amp : 0;
    const goalO = () => ctx.coils && mode === 'park' ? park : 0;
    const omegaOf = () => TWO_PI / (mode === 'fast' ? fastPeriod : period);
    function settle() {
        phase = 0; A = goalA(); O = goalO();
        theta = O; thetaDot = A * omegaOf();
        lastHit = -10;
    }
    settle();

    function advance(h) {
        const rate = ARM_RATE * h, dA = toward(A, goalA(), rate), dO = toward(O, goalO(), rate), w = omegaOf();
        A += dA; O += dO;
        phase += w * h;
        if (phase >= TWO_PI) phase -= TWO_PI;
        const s = Math.sin(phase);
        theta = O + A * s;
        thetaDot = (dO + dA * s) / h + A * Math.cos(phase) * w;
    }
    const hBob = () => pivotH - L * Math.cos(theta);

    return {
        sub(h, balls, n) {
            advance(h);
            const hb = hBob(), dh = hb - BALL_R;
            if (dh >= rr || dh <= -rr) return;              // the bob is clear of every ball
            const reff = Math.sqrt(rr * rr - dh * dh), reff2 = reff * reff, s = Math.sin(theta);
            const bx = px + ax * L * s, by = py + ay * L * s, vb = L * Math.cos(theta) * thetaDot, svx = ax * vb, svy = ay * vb;
            for (let i = 0; i < n; i++) {
                const b = balls[i], dx = b.x - bx, dy = b.y - by, d2 = dx * dx + dy * dy;
                if (d2 >= reff2) continue;
                const d = Math.sqrt(d2);
                const nx = d > 1e-9 ? dx / d : -ay, ny = d > 1e-9 ? dy / d : ax;
                b.x += nx * (reff - d); b.y += ny * (reff - d);
                const vn = resolveVelocity(b, nx, ny, material, svx, svy);
                b.exempt = true;
                if (vn > HIT_SPEED && clk.t - lastHit > HIT_GAP) { lastHit = clk.t; ctx.emit('pendulum', id, b, { speed: vn }); }
            }
        },
        cmd(c) {
            if (c.type !== 'pendulum' || !MODES.includes(c.mode)) return false;
            mode = c.mode;
            return true;
        },
        reset() { mode = modeOf(); settle(); },
        anim(a) { a[this.slot] = theta; },
        dbg() {
            const s = Math.sin(theta), hb = hBob(), dh = hb - BALL_R;
            return {
                id, type: 'pendulum', theta, mode, bob: [px + ax * L * s, py + ay * L * s], hBob: hb,
                bobR, reff: Math.abs(dh) < rr ? Math.sqrt(rr * rr - dh * dh) : 0, pivot: [px, py]
            };
        }
    };
}
