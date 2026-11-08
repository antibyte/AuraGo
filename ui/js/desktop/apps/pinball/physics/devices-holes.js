// Ball-holding devices: hole (saucer/scoop/vuk/kickhole), magnet and captive ball channel. DOM-free.
//
// Every factory takes (def, ctx, clk) and returns a device object (see devices.js):
//   { id, type, sub(h, balls, n), pre(), post(), cmd(cmd) -> bool, search(cycle) -> bool, reset(), remove(ballId), clear(),
//     anim(a), dbg() }
// `clk.t` is the simulation time in seconds, refreshed by devices.preTick. `balls` is the cached array of field balls.
// Held balls are mode 'held' (holder = device id); the world neither integrates nor collides them.
import { STEP, BALL_R, G_SLOPE } from './constants.js';
import { rngNext } from '../config.js';

const SPACING = 0.6;            // seconds between balls of an "eject all"
const RECAPTURE = 0.35;         // seconds a freshly ejected ball cannot be captured again
const TILT_EJECT = 0.5;         // coils off: holes give their balls back after this long
const CAPTURE_SPEED = { saucer: 0.9, scoop: 2.8, vuk: 2.8, kickhole: 1.2 };
const decay = (t, t0, dur) => { const k = 1 - (t - t0) / dur; return k > 0 ? k : 0; };

// ---------------------------------------------------------------------------------------------------------- hole
// def: {kind, x, y, r, captureSpeed, capacity, hidden, eject: {angleDeg, speed} | {path, speed}, autoEject, enabled}
// angleDeg is a math angle: 0 = +x, 90 = +y (up the table).
export function createHole(def, ctx, clk) {
    const id = def.id, kind = def.kind || 'saucer', x = def.x, y = def.y, r = def.r ?? 0.016, r2 = r * r;
    const capacity = Math.max(1, def.capacity ?? 1), captureSpeed = def.captureSpeed ?? CAPTURE_SPEED[kind] ?? 1.2;
    const eject = def.eject || { angleDeg: 90, speed: 2 }, autoEject = def.autoEject ?? 6;
    const slots = [];                       // {ball, ejectAt, locked}, oldest first
    let enabled = def.enabled !== false, coolId = null, coolUntil = 0, lipId = null, tilt = false, kickT = -10;

    function take(ball, via) {
        ball.mode = 'held'; ball.holder = id; ball.hidden = !!def.hidden; ball.h = 0; ball.path = null;
        ball.vx = 0; ball.vy = 0; ball.x = ball.px = x; ball.y = ball.py = y;
        const late = ctx.coils ? clk.t + autoEject : clk.t + TILT_EJECT + SPACING * slots.length;
        slots.push({ ball, ejectAt: late, locked: false });
        ctx.emit('hole', id, ball, { via, count: slots.length });
    }
    function release(slot) {
        slots.splice(slots.indexOf(slot), 1);
        const ball = slot.ball;
        ball.hidden = false; ball.holder = null; ball.captive = false; ball.mode = 'field'; ball.h = 0; ball.path = null;
        ball.x = ball.px = x; ball.y = ball.py = y;
        const speed = (eject.speed ?? 2) * (1 + (rngNext(ctx.rng) * 2 - 1) * 0.02);
        const jitter = (rngNext(ctx.rng) * 2 - 1) * 0.02;
        let via = 'field';
        if (eject.path && typeof ctx.launchPath === 'function' && ctx.launchPath(ball, eject.path, speed)) via = 'path';
        else {
            const ang = (eject.path ? Math.PI / 2 : (eject.angleDeg ?? 90) * Math.PI / 180) + jitter;
            ball.vx = speed * Math.cos(ang); ball.vy = speed * Math.sin(ang);
        }
        coolId = ball.id; coolUntil = clk.t + RECAPTURE; kickT = clk.t;
        ctx.emit('eject', id, ball, { via, speed, count: slots.length });
    }
    function schedule(count, delay) {
        const n = count === 'all' ? slots.length : Math.min(slots.length, Math.max(1, count | 0));
        for (let i = 0; i < n; i++) { slots[i].locked = false; slots[i].ejectAt = clk.t + delay + i * SPACING; }
        return n > 0;
    }

    return {
        id, type: 'hole', slot: -1,
        // Path exits deliver here whatever the speed. False when disabled or full (paths fall back to a field exit).
        accept(ball) {
            if (!enabled || slots.length >= capacity) return false;
            take(ball, 'path');
            return true;
        },
        sub(h, balls, n) {
            if (!enabled) return;
            for (let i = 0; i < n; i++) {
                const b = balls[i];
                if (b.mode !== 'field') continue;
                const dx = b.x - x, dy = b.y - y, d2 = dx * dx + dy * dy;
                if (d2 >= r2) { if (lipId === b.id && d2 > 2.5 * r2) lipId = null; continue; }
                if (slots.length >= capacity || (b.id === coolId && clk.t < coolUntil)) continue;
                const sp = Math.sqrt(b.vx * b.vx + b.vy * b.vy);
                if (sp < captureSpeed) take(b, 'field');
                else if (kind === 'saucer' && lipId !== b.id) {
                    lipId = b.id;
                    const a = (rngNext(ctx.rng) * 2 - 1) * 0.15, c = Math.cos(a) * 0.85, s = Math.sin(a) * 0.85;
                    const vx = b.vx; b.vx = vx * c - b.vy * s; b.vy = vx * s + b.vy * c;
                }
            }
        },
        pre() {
            if (!ctx.coils) {
                if (!tilt) {
                    tilt = true;
                    for (let i = 0; i < slots.length; i++) slots[i].ejectAt = Math.min(slots[i].ejectAt, clk.t + TILT_EJECT + SPACING * i);
                }
            } else tilt = false;
            for (let i = 0; i < slots.length; i++) if (clk.t >= slots[i].ejectAt) { release(slots[i]); break; }
        },
        cmd(c) {
            switch (c.type) {
                case 'eject': schedule(c.count === 'all' ? 'all' : (c.count || 1), Math.max(0, +c.delay || 0)); return true;
                case 'kick': schedule(1, 0); return true;
                case 'lockBall':
                    for (const s of slots) { s.locked = true; s.ejectAt = ctx.coils ? Infinity : Math.min(s.ejectAt, clk.t + TILT_EJECT); }
                    return true;
                case 'releaseToTrough': {
                    const all = slots.splice(0, slots.length);
                    for (const s of all) { s.ball.holder = null; ctx.returnToTrough(s.ball.id); }
                    return true;
                }
                case 'holeEnable': enabled = !!c.on; return true;
                default: return false;
            }
        },
        search() {
            let any = false;
            for (let i = 0; i < slots.length; i++) if (!slots[i].locked) { slots[i].ejectAt = clk.t + SPACING * i; any = true; }
            return any;
        },
        reset() { slots.length = 0; enabled = def.enabled !== false; coolId = null; lipId = null; tilt = false; kickT = -10; },
        remove(ballId) { for (let i = slots.length - 1; i >= 0; i--) if (slots[i].ball.id === ballId) slots.splice(i, 1); },
        clear() { slots.length = 0; },
        anim(a) { a[this.slot] = decay(clk.t, kickT, 0.2); },
        dbg() { return { id, type: 'hole', kind, x, y, r, enabled, held: slots.length, capacity, locked: slots.filter(s => s.locked).length }; }
    };
}

// -------------------------------------------------------------------------------------------------------- magnet
// def: {x, y, r, pull, damp, maxAccel}. Commands: magnet {mode: 'off'|'grab'|'release'|'throw', angle (rad), speed}.
// Deviation from plan 4: gravity is cancelled over the whole pull radius (not only 0.5 r), otherwise the equilibrium
// offset g/pull (18.5 mm) lies outside the cancel zone and the ball never settles on the centre.
export function createMagnet(def, ctx, clk) {
    const id = def.id, x = def.x, y = def.y, r = def.r ?? 0.035, r2 = r * r;
    const pull = def.pull ?? 60, damp = def.damp ?? 8, maxAcc = def.maxAccel ?? 6;
    let mode = 'off', held = null, engaged = null, power = 0;

    function free(vx, vy, state) {
        const b = held; held = null;
        b.mode = 'field'; b.holder = null; b.hidden = false; b.vx = vx; b.vy = vy; b.h = 0;
        ctx.emit('magnet', id, b, { state });
    }
    return {
        id, type: 'magnet', slot: -1,
        sub(h, balls, n) {
            if (mode !== 'grab' || held || !ctx.coils) return;
            let seen = false;
            for (let i = 0; i < n; i++) {
                const b = balls[i];
                if (b.mode !== 'field') continue;
                const dx = x - b.x, dy = y - b.y, d2 = dx * dx + dy * dy;
                if (d2 >= r2) continue;
                seen = true;
                if (engaged !== b.id) { engaged = b.id; ctx.emit('magnet', id, b, { state: 'grab' }); }
                let ax = pull * dx - damp * b.vx, ay = pull * dy - damp * b.vy;
                const am = Math.sqrt(ax * ax + ay * ay);
                if (am > maxAcc) { ax *= maxAcc / am; ay *= maxAcc / am; }
                b.vx += ax * h; b.vy += (ay + G_SLOPE) * h;
                if (d2 < 9e-6 && b.vx * b.vx + b.vy * b.vy < 0.0016) {
                    b.mode = 'held'; b.holder = id; b.vx = 0; b.vy = 0; b.x = b.px = x; b.y = b.py = y;
                    held = b;
                    ctx.emit('magnet', id, b, { state: 'held' });
                    break;
                }
            }
            if (!seen) engaged = null;
        },
        pre() { if (!ctx.coils && (held || mode === 'grab')) { if (held) free(0, 0, 'release'); mode = 'off'; engaged = null; } },
        post() {
            const target = mode === 'grab' && ctx.coils ? 1 : 0;
            power += Math.max(-1, Math.min(1, target - power)) * Math.min(1, STEP / 0.15);
        },
        cmd(c) {
            if (c.type !== 'magnet') return false;
            switch (c.mode) {
                case 'grab': if (ctx.coils) mode = 'grab'; break;
                case 'off': case 'release': if (held) free(0, 0, 'release'); mode = 'off'; engaged = null; break;
                case 'throw': {
                    const sp = Number.isFinite(c.speed) ? c.speed : 3, an = Number.isFinite(c.angle) ? c.angle : Math.PI / 2;
                    if (held) free(sp * Math.cos(an), sp * Math.sin(an), 'throw');
                    mode = 'off'; engaged = null;
                    break;
                }
                default: return false;
            }
            return true;
        },
        reset() { mode = 'off'; held = null; engaged = null; power = 0; },
        remove(ballId) { if (held && held.id === ballId) held = null; if (engaged === ballId) engaged = null; },
        clear() { held = null; engaged = null; },
        anim(a) { a[this.slot] = power; },
        dbg() { return { id, type: 'magnet', x, y, r, mode, held: held ? held.id : null }; }
    };
}

// ------------------------------------------------------------------------------------------------------- captive
// def: {channel: {a, b, width}, rest: [x, y], endSwitch}. Builds the channel walls, two mouth posts and the end trigger,
// and owns the captive ball (mode 'captive', id `${id}_ball`) which rolls along the channel axis a -> b. The channel must
// rise from a (mouth, resting end) to b so gravity returns the ball. Strikes by field balls are solved as a contact with
// a ball constrained to the axis; the `captive` event carries the striking ball. Reaching the far end emits a `trigger`
// event with id = endSwitch.
export function createCaptive(def, ctx, clk) {
    const id = def.id, ch = def.channel, [ax, ay] = ch.a, [bx, by] = ch.b, width = ch.width ?? 0.034, half = width / 2;
    const dx = bx - ax, dy = by - ay, L = Math.hypot(dx, dy), ux = dx / L, uy = dy / L, nx = -uy, ny = ux;
    const rest = def.rest || [ax + ux * 0.02, ay + uy * 0.02];
    const sMin = Math.max(0.0102, (rest[0] - ax) * ux + (rest[1] - ay) * uy), sMax = L - BALL_R, ballId = id + '_ball';
    const wall = (suffix, x0, y0, x1, y1) => ctx.shapes.add({ id: id + suffix, kind: 'seg', ax: x0, ay: y0, bx: x1, by: y1, material: 'metal', owner: 'device' });
    wall('_wallL', ax + nx * half, ay + ny * half, bx + nx * half, by + ny * half);
    wall('_wallR', ax - nx * half, ay - ny * half, bx - nx * half, by - ny * half);
    wall('_end', bx + nx * half, by + ny * half, bx - nx * half, by - ny * half);
    for (const [suffix, k] of [['_postL', 1], ['_postR', -1]]) {
        ctx.shapes.add({ id: id + suffix, kind: 'circle', cx: ax + nx * 0.013 * k, cy: ay + ny * 0.013 * k, r: 0.003, material: 'metal', owner: 'device' });
    }
    let s = sMin, w = 0, ball = null, lastStrike = -10, atEnd = false;

    function place() {
        if (!ball) return;
        ball.x = ball.px = ax + ux * s; ball.y = ball.py = ay + uy * s; ball.vx = w * ux; ball.vy = w * uy;
    }
    function spawn() {
        s = sMin; w = 0; atEnd = false;
        ball = ctx.addBallAt(ballId, ax + ux * s, ay + uy * s, 0, 0, 'captive');
        ball.captive = true; place();
    }
    spawn();

    return {
        id, type: 'captive', slot: -1,
        sub(h, balls, n) {
            if (!ball) return;
            w += -G_SLOPE * uy * h;
            if (w !== 0) { const d = 0.055 * h; w = Math.abs(w) <= d ? 0 : w - Math.sign(w) * d; }
            s += w * h;
            if (s < sMin) { s = sMin; w = w < -0.05 ? -w * 0.3 : 0; }
            else if (s >= sMax) {
                s = sMax;
                if (w > 0.05 && !atEnd) { atEnd = true; place(); ctx.emit('trigger', def.endSwitch, ball, { dir: 1 }); }
                w = w > 0 ? -w * 0.4 : w;
            }
            if (s < sMax - 0.004) atEnd = false;
            place();
            for (let i = 0; i < n; i++) {
                const b = balls[i];
                if (b.mode !== 'field') continue;
                const ex = b.x - ball.x, ey = b.y - ball.y, d2 = ex * ex + ey * ey, rr = 2 * BALL_R;
                if (d2 >= rr * rr) continue;
                const d = Math.sqrt(d2), cx = d > 1e-9 ? ex / d : nx, cy = d > 1e-9 ? ey / d : ny, pen = rr - d;
                b.x += cx * pen; b.y += cy * pen;
                const vn = -((b.vx - w * ux) * cx + (b.vy - w * uy) * cy);
                if (vn <= 0) continue;
                const nu = cx * ux + cy * uy, j = 1.93 * vn / (1 + nu * nu);
                b.vx += cx * j; b.vy += cy * j; w -= nu * j;
                if (vn > 0.15 && clk.t - lastStrike > 0.1) { lastStrike = clk.t; ctx.emit('captive', id, b, { speed: vn }); }
            }
        },
        cmd() { return false },
        reset() { spawn(); },
        remove() {}, clear() { spawn(); },
        anim(a) { a[this.slot] = (s - sMin) / (sMax - sMin); },
        dbg() { return { id, type: 'captive', s, w, sMin, sMax, a: [ax, ay], b: [bx, by], width }; }
    };
}
