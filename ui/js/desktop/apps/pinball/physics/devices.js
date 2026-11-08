// Playfield devices (plan 3.2, 4): bumper, sling, drop bank, standup, spinner, rollover, trigger, gate, door, kickback,
// diverter; holes, magnet and the captive channel live in devices-holes.js. DOM-free, deterministic (rngNext only).
//
//   createDevices(defs, ctx) -> DeviceSystem {ids, preTick, substep(h), postTick, command(cmd), search(cycle), reset, writeAnim, debug}
//
// Hooks this module assigns on ctx (for paths.js, called at runtime only):
//   ctx.deliverToHole(ball, holeId) -> bool   capture a ball arriving from a path, whatever its speed; emits hole {via:'path'};
//                                             false when the hole is disabled or full (paths then use a field exit)
//   ctx.diverterOn(diverterId) -> bool        current diverter state
// Hook this module calls when present (provided by paths.js): ctx.launchPath(ball, pathId, speed) -> bool, used by hole
// ejects of the form eject: {path, speed}; without it the hole ejects straight up (+y).
//
// Event payloads (type, id = device or target id, ball, tick, x, y always present):
//   bumper {} · sling {speed} · drop {bank, speed} · bankDown {} · bankReset {} · standup {speed} · spinner {rate}
//   rollover {dir} · trigger {dir} · gate {dir} (dir = +1 along the device normal (-dy, dx), gate: along `pass`)
//   hole {via, count} · eject {via, speed, count} · kickback {} · magnet {state} · captive {speed}
// Coils off (tilt): bumpers and slings act passive (no kick, no event), the kickback does not fire, the magnet drops its
// ball and switches off, holes give their balls back after 0.5 s. Switch-type devices (standup, targets, spinner,
// rollover, trigger, gate) keep reporting.
import { STEP, BALL_R, G_SLOPE } from './constants.js';
import { capEnergy } from './collide.js';
import { buildAnimIndex } from './snapshot.js';
import { createHole, createMagnet, createCaptive } from './devices-holes.js';

const decay = (t, t0, dur) => { const k = 1 - (t - t0) / dur; return k > 0 ? k : 0; };
const TWO_PI = Math.PI * 2;

// A contact that may not add energy: clamp to the specific energy at the start of the substep.
const passive = ball => capEnergy(ball, ball.e0, G_SLOPE);

// Replace the ball's normal velocity by `vn` (along n) and scale the tangential part.
function kickAlong(ball, nx, ny, vn, tangent) {
    const cur = ball.vx * nx + ball.vy * ny, tx = ball.vx - cur * nx, ty = ball.vy - cur * ny;
    ball.vx = nx * vn + tx * tangent; ball.vy = ny * vn + ty * tangent;
}

// Directed segment a-b with normal (-dy, dx): +1 / -1 when the ball's last substep move crossed it, else 0.
function lineOf(a, b) {
    const dx = b[0] - a[0], dy = b[1] - a[1], len = Math.hypot(dx, dy) || 1;
    return { ax: a[0], ay: a[1], dx, dy, nx: -dy, ny: dx, ux: -dy / len, uy: dx / len, inv2: 1 / (dx * dx + dy * dy || 1) };
}
function crossing(l, b) {
    if (b.mode !== 'field') return 0;
    const s0 = (b.px - l.ax) * l.nx + (b.py - l.ay) * l.ny, s1 = (b.x - l.ax) * l.nx + (b.y - l.ay) * l.ny;
    let dir;
    if (s0 < 0 && s1 >= 0) dir = 1; else if (s0 >= 0 && s1 < 0) dir = -1; else return 0;
    const f = s0 / (s0 - s1), X = b.px + (b.x - b.px) * f, Y = b.py + (b.y - b.py) * f;
    const u = ((X - l.ax) * l.dx + (Y - l.ay) * l.dy) * l.inv2;
    return u >= 0 && u <= 1 ? dir : 0;
}
// Distance from a point to segment a-b; fills tmp = {nx, ny} with the unit normal pointing at the point.
function segDist(px, py, ax, ay, bx, by, tmp) {
    const dx = bx - ax, dy = by - ay, l2 = dx * dx + dy * dy;
    let t = l2 > 0 ? ((px - ax) * dx + (py - ay) * dy) / l2 : 0;
    t = t < 0 ? 0 : (t > 1 ? 1 : t);
    const ex = px - ax - t * dx, ey = py - ay - t * dy, d = Math.sqrt(ex * ex + ey * ey);
    tmp.nx = d > 1e-9 ? ex / d : 0; tmp.ny = d > 1e-9 ? ey / d : 1;
    return d;
}
const tmpN = { nx: 0, ny: 1 };
const faceEnds = (x, y, angle, width) => {
    const tx = -Math.sin(angle) * width / 2, ty = Math.cos(angle) * width / 2;
    return { ax: x - tx, ay: y - ty, bx: x + tx, by: y + ty };
};

export function createDevices(defs, ctx) {
    const clk = { t: 0 };
    const animIndex = buildAnimIndex(ctx.layout || { devices: defs });
    const devs = [], byId = new Map(), holes = new Map(), diverters = new Map();
    const pres = [], subs = [], posts = [];
    const field = [];                         // cached field balls, refreshed per substep
    let nField = 0;
    const collect = b => { if (b.mode === 'field') field[nField++] = b; };
    const refresh = () => { nField = 0; ctx.balls.forEach(collect); };

    // ---- bumper -------------------------------------------------------------------------------------------------
    function bumper(def) {
        const x = def.x, y = def.y, r = def.r ?? 0.0215, kick = def.kick ?? 2.0;
        let t0 = -10;
        const fire = (ball, nx, ny, vn, silent) => {
            kickAlong(ball, nx, ny, kick + 0.3 * vn, 0.9);
            t0 = clk.t;
            if (!silent) ctx.emit('bumper', def.id, ball);
        };
        ctx.shapes.add({
            id: def.id, kind: 'circle', cx: x, cy: y, r, material: 'rubber', owner: 'device',
            onHit(ball, info) {
                if (!ctx.coils || clk.t - t0 < 0.06) { passive(ball); return; }
                fire(ball, info.nx, info.ny, info.vn, false);
            }
        });
        const touch = (silent) => {
            let any = false;
            for (let i = 0; i < nField; i++) {
                const b = field[i], dx = b.x - x, dy = b.y - y, d = Math.sqrt(dx * dx + dy * dy);
                if (d < r + BALL_R + 0.004 && d > 1e-9) { fire(b, dx / d, dy / d, 0, silent); any = true; }
            }
            return any;
        };
        return {
            cmd(c) { if (c.type !== 'kick') return false; refresh(); touch(false); return true; },
            search() { refresh(); return touch(true); },
            reset() { t0 = -10; },
            anim(a) { a[this.slot] = decay(clk.t, t0, 0.06); },
            dbg: () => ({ id: def.id, type: 'bumper', x, y, r })
        };
    }

    // ---- sling: a kicker on an existing rubber wall -------------------------------------------------------------
    function sling(def) {
        const kick = def.kick ?? 1.9, threshold = def.threshold ?? 0.30;
        const segs = ctx.shapes.group(def.face);
        let t0 = -10;
        const fire = (ball, nx, ny, vn, silent) => {
            kickAlong(ball, nx, ny, kick + 0.3 * vn, 1);
            t0 = clk.t;
            if (!silent) ctx.emit('sling', def.id, ball, { speed: kick + 0.3 * vn });
        };
        const onHit = (ball, info) => {
            if (!ctx.coils || info.vn < threshold || clk.t - t0 < 0.05) { passive(ball); return; }
            fire(ball, info.nx, info.ny, info.vn, false);
        };
        for (let i = 0; i < segs.length; i++) segs[i].onHit = onHit;
        const touch = silent => {
            let any = false;
            for (let i = 0; i < nField; i++) for (let k = 0; k < segs.length; k++) {
                const s = segs[k], b = field[i];
                if (segDist(b.x, b.y, s.ax, s.ay, s.bx, s.by, tmpN) < BALL_R + 0.004) { fire(b, tmpN.nx, tmpN.ny, 0, silent); any = true; break; }
            }
            return any;
        };
        return {
            cmd(c) { if (c.type !== 'kick') return false; refresh(); touch(false); return true; },
            search() { refresh(); return touch(true); },
            reset() { t0 = -10; },
            anim(a) { a[this.slot] = decay(clk.t, t0, 0.05); },
            dbg: () => ({ id: def.id, type: 'sling', face: def.face })
        };
    }

    // ---- standup ------------------------------------------------------------------------------------------------
    function standup(def) {
        const f = faceEnds(def.x, def.y, def.angle || 0, def.width ?? 0.020), threshold = def.threshold ?? 0.15;
        let t0 = -10;
        ctx.shapes.add({
            id: def.id, kind: 'seg', ax: f.ax, ay: f.ay, bx: f.bx, by: f.by, material: 'target', owner: 'device',
            onHit(ball, info) {
                passive(ball);
                if (info.vn >= threshold) { t0 = clk.t; ctx.emit('standup', def.id, ball, { speed: info.vn }); }
            }
        });
        return {
            reset() { t0 = -10; },
            anim(a) { a[this.slot] = decay(clk.t, t0, 0.12); },
            dbg: () => ({ id: def.id, type: 'standup', ...f })
        };
    }

    // ---- drop bank ----------------------------------------------------------------------------------------------
    function dropBank(def) {
        const width = def.width ?? 0.024, threshold = def.threshold ?? 0.25;
        const targets = (def.targets || []).map(t => {
            const f = faceEnds(t.x, t.y, def.angle || 0, width);
            return { id: t.id, f, down: false, t0: -10, slot: animIndex.get(t.id), shape: null };
        });
        let downCount = 0, pending = false;
        const lower = (t, tClock) => { t.down = true; t.t0 = tClock; ctx.shapes.setActive(t.shape, false); downCount++; };
        for (const t of targets) {
            t.shape = ctx.shapes.add({
                id: t.id, kind: 'seg', ax: t.f.ax, ay: t.f.ay, bx: t.f.bx, by: t.f.by, material: 'target', owner: 'device',
                onHit(ball, info) {
                    passive(ball);
                    if (t.down || info.vn < threshold) return;
                    lower(t, clk.t);
                    ctx.emit('drop', t.id, ball, { bank: def.id, speed: info.vn });
                    if (downCount === targets.length) ctx.emit('bankDown', def.id, ball);
                }
            });
        }
        function blocked() {
            refresh();
            for (let i = 0; i < nField; i++) for (const t of targets) {
                if (segDist(field[i].x, field[i].y, t.f.ax, t.f.ay, t.f.bx, t.f.by, tmpN) < BALL_R + 0.002) return true;
            }
            return false;
        }
        function raise(silent) {
            for (const t of targets) if (t.down) { t.down = false; t.t0 = clk.t; ctx.shapes.setActive(t.shape, true); }
            downCount = 0; pending = false;
            if (!silent) ctx.emit('bankReset', def.id, null);
        }
        return {
            post() { if (pending && !blocked()) raise(false); },
            cmd(c) {
                if (c.type === 'resetBank') {
                    if (!downCount) return true;
                    if (blocked()) pending = true; else raise(false);
                    return true;
                }
                if (c.type === 'dropBank') { for (const t of targets) if (!t.down) lower(t, clk.t); pending = false; return true; }
                return false;
            },
            reset() { raise(true); for (const t of targets) t.t0 = -10; },
            anim(a) {
                for (const t of targets) {
                    const k = 1 - decay(clk.t, t.t0, 0.12);
                    a[t.slot] = t.down ? k : 1 - k;
                }
                a[this.slot] = targets.length ? downCount / targets.length : 0;
            },
            dbg: () => ({ id: def.id, type: 'dropBank', down: targets.filter(t => t.down).map(t => t.id), pending })
        };
    }

    // ---- spinner ------------------------------------------------------------------------------------------------
    function spinner(def) {
        const l = lineOf(def.a, def.b), gain = def.gain ?? 60, maxRate = def.maxRate ?? 360;
        let omega = 0, angle = 0, acc = 0;
        return {
            sub(h, balls, n) {
                for (let i = 0; i < n; i++) {
                    const b = balls[i], c = crossing(l, b);
                    if (!c) continue;
                    const vn = Math.abs(b.vx * l.ux + b.vy * l.uy);
                    omega = c * Math.min(maxRate, gain * vn);
                    b.vx *= 0.98; b.vy *= 0.98;
                }
            },
            post() {
                if (omega === 0) return;
                const a = Math.abs(omega), sg = omega > 0 ? 1 : -1;
                let na = a - (25 + 1.2 * a) * STEP;
                if (na < 0) na = 0;
                const mean = 0.5 * (a + na);
                angle += sg * mean * STEP; acc += mean * STEP;
                while (acc >= TWO_PI) { acc -= TWO_PI; ctx.emit('spinner', def.id, null, { rate: a }); }
                omega = na * sg;
                if (na === 0) omega = 0;
            },
            reset() { omega = 0; angle = 0; acc = 0; },
            anim(a) { a[this.slot] = angle; },
            dbg: () => ({ id: def.id, type: 'spinner', omega, angle })
        };
    }

    // ---- rollover -----------------------------------------------------------------------------------------------
    function rollover(def) {
        const r = def.r ?? 0.008, r2 = r * r, inside = [];
        let t0 = -10;
        return {
            sub(h, balls, n) {
                for (let i = 0; i < n; i++) {
                    const b = balls[i], dx = b.x - def.x, dy = b.y - def.y, inn = b.mode === 'field' && dx * dx + dy * dy < r2, k = inside.indexOf(b.id);
                    if (inn && k < 0) { inside.push(b.id); t0 = clk.t; ctx.emit('rollover', def.id, b, { dir: b.vy >= 0 ? 1 : -1 }); }
                    else if (!inn && k >= 0) inside.splice(k, 1);
                }
            },
            reset() { inside.length = 0; t0 = -10; },
            remove(ballId) { const k = inside.indexOf(ballId); if (k >= 0) inside.splice(k, 1); },
            clear() { inside.length = 0; },
            anim(a) { a[this.slot] = decay(clk.t, t0, 0.1); },
            dbg: () => ({ id: def.id, type: 'rollover', x: def.x, y: def.y, r })
        };
    }

    // ---- trigger ------------------------------------------------------------------------------------------------
    function trigger(def) {
        const l = lineOf(def.a, def.b);
        return {
            sub(h, balls, n) {
                for (let i = 0; i < n; i++) {
                    const c = crossing(l, balls[i]);
                    if (c) ctx.emit('trigger', def.id, balls[i], { dir: c });
                }
            },
            reset() {}, anim() {},
            dbg: () => ({ id: def.id, type: 'trigger', a: def.a, b: def.b })
        };
    }

    // ---- gate: oneWay blocks balls moving against `pass`; open and closed are plain ----------------------------
    function gate(def) {
        const l = lineOf(def.a, def.b), pass = def.pass || [l.ux, l.uy], sgn = (l.nx * pass[0] + l.ny * pass[1]) >= 0 ? 1 : -1;
        const shape = ctx.shapes.add({ id: def.id, kind: 'seg', ax: def.a[0], ay: def.a[1], bx: def.b[0], by: def.b[1], material: 'metal', owner: 'device' });
        let mode = 'oneWay', t0 = -10;
        const apply = m => {
            mode = m === 'open' || m === 'closed' ? m : 'oneWay';
            shape.oneWay = mode === 'oneWay' ? [pass[0], pass[1]] : null;
            ctx.shapes.setActive(shape, mode !== 'open');
        };
        apply(def.mode || 'oneWay');
        return {
            sub(h, balls, n) {
                for (let i = 0; i < n; i++) {
                    const c = crossing(l, balls[i]);
                    if (c) { t0 = clk.t; ctx.emit('gate', def.id, balls[i], { dir: c * sgn }); }
                }
            },
            cmd(c) { if (c.type !== 'gate') return false; apply(c.mode); return true; },
            reset() { apply(def.mode || 'oneWay'); t0 = -10; },
            anim(a) { a[this.slot] = Math.max(mode === 'open' ? 1.2 : 0, 1.2 * decay(clk.t, t0, 0.25)); },
            dbg: () => ({ id: def.id, type: 'gate', mode })
        };
    }

    // ---- door ---------------------------------------------------------------------------------------------------
    function door(def) {
        const shape = ctx.shapes.add({ id: def.id, kind: 'seg', ax: def.a[0], ay: def.a[1], bx: def.b[0], by: def.b[1], material: def.material || 'plastic', owner: 'device' });
        let open = !!def.open, pos = open ? 1 : 0;
        ctx.shapes.setActive(shape, pos < 0.5);
        return {
            post() {
                const target = open ? 1 : 0;
                if (pos === target) return;
                const d = STEP / 0.4;
                pos = pos < target ? Math.min(target, pos + d) : Math.max(target, pos - d);
                ctx.shapes.setActive(shape, pos < 0.5);
            },
            cmd(c) { if (c.type !== 'door') return false; open = !!c.open; return true; },
            reset() { open = !!def.open; pos = open ? 1 : 0; ctx.shapes.setActive(shape, pos < 0.5); },
            anim(a) { a[this.slot] = pos * pos * (3 - 2 * pos); },
            dbg: () => ({ id: def.id, type: 'door', open, pos })
        };
    }

    // ---- kickback -----------------------------------------------------------------------------------------------
    function kickback(def) {
        const r = def.r ?? 0.012, r2 = r * r, speed = def.speed ?? 3.6;
        let armed = !!def.armed, t0 = -10;
        const shoot = b => { b.vx *= 0.2; b.vy = speed; armed = false; t0 = clk.t; ctx.emit('kickback', def.id, b); };
        return {
            sub(h, balls, n) {
                if (!armed || !ctx.coils) return;
                for (let i = 0; i < n; i++) {
                    const b = balls[i], dx = b.x - def.x, dy = b.y - def.y;
                    if (b.mode === 'field' && dx * dx + dy * dy < r2) { shoot(b); return; }
                }
            },
            cmd(c) {
                if (c.type === 'kickback') { armed = !!c.armed; return true; }
                if (c.type === 'kick') {
                    refresh();
                    for (let i = 0; i < nField; i++) if ((field[i].x - def.x) ** 2 + (field[i].y - def.y) ** 2 < r2 * 4) { shoot(field[i]); break; }
                    return true;
                }
                return false;
            },
            reset() { armed = !!def.armed; t0 = -10; },
            anim(a) { a[this.slot] = decay(clk.t, t0, 0.15); },
            dbg: () => ({ id: def.id, type: 'kickback', armed })
        };
    }

    // ---- diverter (path-level switch) ---------------------------------------------------------------------------
    function diverter(def) {
        let on = !!def.on, pos = on ? 1 : 0;
        return {
            get on() { return on; },
            post() { const t = on ? 1 : 0; if (pos !== t) pos = pos < t ? Math.min(t, pos + STEP / 0.15) : Math.max(t, pos - STEP / 0.15); },
            cmd(c) { if (c.type !== 'diverter') return false; on = !!c.on; return true; },
            reset() { on = !!def.on; pos = on ? 1 : 0; },
            anim(a) { a[this.slot] = pos; },
            dbg: () => ({ id: def.id, type: 'diverter', on })
        };
    }

    const FACTORIES = {
        bumper, sling, standup, dropBank, spinner, rollover, trigger, gate, door, kickback, diverter,
        hole: d => createHole(d, ctx, clk), magnet: d => createMagnet(d, ctx, clk), captive: d => createCaptive(d, ctx, clk)
    };
    for (const def of defs || []) {
        const make = FACTORIES[def.type];
        if (!make) continue;
        const d = make(def);
        d.id = def.id; d.type = def.type; d.slot = animIndex.has(def.id) ? animIndex.get(def.id) : -1;
        if (!d.anim) d.anim = () => {};
        devs.push(d); byId.set(def.id, d);
        if (d.pre) pres.push(d);
        if (d.sub) subs.push(d);
        if (d.post) posts.push(d);
        if (def.type === 'hole') holes.set(def.id, d);
        if (def.type === 'diverter') diverters.set(def.id, d);
    }

    ctx.deliverToHole = (ball, holeId) => { const h = holes.get(holeId); return h ? h.accept(ball) : false; };
    ctx.diverterOn = id => { const d = diverters.get(id); return d ? d.on : undefined; };

    return {
        ids: (defs || []).map(d => d.id),
        preTick() {
            clk.t = ctx.tick * STEP;
            for (let i = 0; i < pres.length; i++) pres[i].pre();
        },
        substep(h) {
            if (!subs.length) return;
            refresh();
            for (let i = 0; i < subs.length; i++) subs[i].sub(h, field, nField);
        },
        postTick() { for (let i = 0; i < posts.length; i++) posts[i].post(); },
        command(cmd) {
            if (cmd.type === 'removeBall') { for (const d of devs) if (d.remove) d.remove(cmd.id); return true; }
            if (cmd.type === 'clearBalls') { for (const d of devs) if (d.clear) d.clear(); return true; }
            const d = byId.get(cmd.id);
            return d && d.cmd ? d.cmd(cmd) === true : false;
        },
        search(cycle) {
            let any = false;
            for (const d of devs) if (d.search && d.search(cycle)) any = true;
            return any;
        },
        reset() { for (const d of devs) d.reset(); },
        writeAnim(anim) { for (let i = 0; i < devs.length; i++) devs[i].anim(anim); },
        debug() { return devs.map(d => d.dbg()); }
    };
}

export default createDevices;
