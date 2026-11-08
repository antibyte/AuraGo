// Pinball physics world: fixed STEP ticks, adaptive substeps, grid contacts, flippers, plunger, trough. DOM-free.
//
// Public API (plan 3.7): createWorld(layout, {seed}) -> world
//   step({left, right, plunger}) advances one STEP; events() drains the queue; command(cmd); snapshot(out?);
//   reset(); placeBall(id, x, y, vx, vy); stats(); debug(); dispose(); animIndex; trough (number of balls in the trough).
//
// ctx contract handed to createDevices(defs, ctx) / createPaths(defs, ctx)  (T03/T04 build on exactly this):
//   layout            the table layout
//   rng               {seed}: use rngNext(ctx.rng) from ../config.js, never Math.random
//   tick              getter, current tick (events of the running step carry it)
//   coils             getter, false while tilted (flippers drop, kickers act passive)
//   balls             Map<id, Ball>; Ball = {id, x, y, vx, vy, px, py, h, mode, hidden, captive, holder, path, lastSwitch, ...}
//                     px/py is the position at the start of the substep. mode: 'field'|'path'|'held'|'shooter'|'captive'.
//                     Only 'field' and 'shooter' balls are integrated and collided by the world; devices and paths own the rest.
//   shapes            {add(def) -> Shape, byId(id) -> first Shape|null, group(id) -> Shape[], setActive(idOrShape, on), all() -> Shape[]}
//                     Shape = {id, kind: 'seg'|'circle', ax, ay, bx, by | cx, cy, r, material, active, oneWay: null|[px, py],
//                     owner, onHit(ball, {nx, ny, vn, px, py}) | null, group}. A wall with several segments is one id with a group;
//                     setActive(id, on) switches the whole group. onHit fires after the response with the pre-impact approach speed vn
//                     (the info object is reused: copy what you keep). A ball touching a shape with onHit is exempt from the
//                     passive-energy ceiling for that substep, so a kicker may add energy.
//   emit(type, id, ball, extra)   queue an event {type, id, ball: ball.id|null, tick, x, y, ...extra}
//   addBallAt(id, x, y, vx, vy, mode) -> Ball, removeBall(id) (no trough), returnToTrough(id) (trough + 1)
//   Order per substep: integrate, flippers, contacts (2 passes + ball-ball), devices.substep(h), paths.substep(h), drain, plunger seat.
//   world.command() handles addBall/coils/nudge/removeBall/clearBalls itself and forwards every other command to
//   devices.command() then paths.command(); removeBall/clearBalls are also forwarded so holders can drop references.
import { STEP, BALL_R, G_SLOPE, MAX_SPEED, SUBSTEP_TRAVEL, SUBSTEP_MIN, SUBSTEP_MAX, ROLL_DECEL } from './constants.js';
import { rngNext } from '../config.js';
import { createGrid } from './grid.js';
import { segContact, circleContact, resolveVelocity, capEnergy } from './collide.js';
import { createFlipper, advanceFlipper, flipperContact, flipperTipSpeed, resetFlipper } from './flipper.js';
import { createDevices } from './devices.js';
import { createPaths } from './paths.js';
import { buildAnimIndex, writeSnapshot } from './snapshot.js';

const PLUNGER = { x: 0.2405, yRest: 0.0535, travel: 0.06, pullRate: 1.6, vMin: 0.55, vMax: 3.4, vAuto: 3.0, laneX: [0.2235, 0.2575], zoneYMax: 0.12 };
const IMPACT_GAP = 10;                  // ticks (about 20 ms) between impact events per ball
const AUTO_DELAY = Math.round(0.6 / STEP);
const SEARCH_IDLE = Math.round(10 / STEP), SEARCH_EVERY = Math.round(2 / STEP), SEARCH_CYCLES = 5;
const PASSIVE = new Set(['impact', 'clack', 'search', 'searchFail', 'shooter']);
const SHOWN_DECAY = 14;                 // plunger rod spring-back speed in the snapshot (1/s)
const EMPTY = Object.freeze([]);

export function createWorld(layout, { seed = 1 } = {}) {
    const rng = { seed: seed >>> 0 };
    const pl = { ...PLUNGER, ...(layout.plunger || {}) };
    pl.laneX = (layout.plunger && layout.plunger.laneX) || PLUNGER.laneX;
    const troughSize = layout.meta && Number.isFinite(layout.meta.troughSize) ? layout.meta.troughSize : 4;
    const drainY = Number.isFinite(layout.drainY) ? layout.drainY : 0.075;
    const animIndex = buildAnimIndex(layout);

    let tick = 0, coils = true, trough = troughSize, substeps = 0, searchCount = 0, queue = [];
    let pull = 0, shown = 0, nextSearch = SEARCH_IDLE, searchCycle = 0, inSearch = false;
    const balls = new Map(), list = [], pending = [];
    const flippers = (layout.flippers || []).map(createFlipper);

    // ---- shapes -------------------------------------------------------------------------------------------------
    const grid = createGrid(), shapeList = [], groups = new Map();
    function addShape(def) {
        const s = {
            id: def.id, kind: def.kind === 'circle' ? 'circle' : 'seg', ax: def.ax || 0, ay: def.ay || 0, bx: def.bx || 0, by: def.by || 0,
            cx: def.cx || 0, cy: def.cy || 0, r: def.r || 0, material: def.material || 'metal', active: def.active !== false,
            oneWay: def.oneWay || null, owner: def.owner === undefined ? null : def.owner, onHit: def.onHit || null, group: null, _q: 0
        };
        grid.insert(s);
        shapeList.push(s);
        if (s.id !== undefined) {
            let g = groups.get(s.id);
            if (!g) { g = []; groups.set(s.id, g); }
            g.push(s); s.group = g;
        }
        return s;
    }
    const shapes = {
        add: addShape,
        byId: id => { const g = groups.get(id); return g ? g[0] : null; },
        group: id => groups.get(id) || EMPTY,
        setActive(target, on) {
            if (target && typeof target === 'object') target.active = !!on;
            else for (const s of groups.get(target) || EMPTY) s.active = !!on;
        },
        all: () => shapeList
    };
    for (const w of layout.walls || []) {
        const pts = w.points || [], n = w.closed ? pts.length : pts.length - 1;
        for (let i = 0; i < n; i++) {
            const a = pts[i], b = pts[(i + 1) % pts.length];
            addShape({ id: w.id, kind: 'seg', ax: a[0], ay: a[1], bx: b[0], by: b[1], material: w.material, owner: 'wall' });
        }
    }
    for (const p of layout.posts || []) addShape({ id: p.id, kind: 'circle', cx: p.x, cy: p.y, r: p.r, material: p.material, owner: 'post' });

    // ---- events / balls -----------------------------------------------------------------------------------------
    function emit(type, id, ball, extra) {
        const ev = { type, id, ball: ball ? ball.id : null, tick };
        if (ball) { ev.x = ball.x; ev.y = ball.y; }
        if (extra) for (const k in extra) ev[k] = extra[k];
        queue.push(ev);
        if (!PASSIVE.has(type) && !inSearch) { searchCycle = 0; nextSearch = tick + SEARCH_IDLE; }
        return ev;
    }
    function makeBall(id, x, y, vx, vy, mode) {
        return {
            id, x, y, vx, vy, px: x, py: y, h: 0, mode: mode || 'field', hidden: false, captive: false, holder: null, path: null,
            lastSwitch: null, impactTick: -IMPACT_GAP, clackTick: -IMPACT_GAP, e0: 0, exempt: false, pushed: false, auto: false, serveTick: 0
        };
    }
    function addBallAt(id, x, y, vx, vy, mode) {
        let b = balls.get(id);
        if (b) { b.x = b.px = x; b.y = b.py = y; b.vx = vx; b.vy = vy; b.mode = mode || 'field'; b.h = 0; b.holder = null; b.path = null; b.captive = false; b.hidden = false; return b; }
        b = makeBall(id, x, y, vx, vy, mode);
        balls.set(id, b); list.push(b);
        return b;
    }
    function removeBall(id) {
        const b = balls.get(id);
        if (!b) return false;
        balls.delete(id);
        list.splice(list.indexOf(b), 1);
        return true;
    }
    function returnToTrough(id) { if (removeBall(id)) trough = Math.min(troughSize, trough + 1); }

    const ctx = {
        layout, rng, balls, shapes, emit, addBallAt, removeBall, returnToTrough,
        get tick() { return tick; }, get coils() { return coils; }
    };
    const devices = createDevices(layout.devices || [], ctx);
    const paths = createPaths(layout.paths || [], ctx);

    // ---- contacts -----------------------------------------------------------------------------------------------
    const cinfo = { nx: 0, ny: 0, pen: 0, cx: 0, cy: 0, svx: 0, svy: 0 };
    const hit = { nx: 0, ny: 0, vn: 0, px: 0, py: 0 };
    const cand = [];

    function respond(ball, id, material, onHit, moving) {
        ball.x += cinfo.nx * cinfo.pen; ball.y += cinfo.ny * cinfo.pen;
        const vn = resolveVelocity(ball, cinfo.nx, cinfo.ny, material, cinfo.svx, cinfo.svy);
        if (moving) ball.exempt = true;
        if (vn > 0) {
            if (vn > 0.15 && tick - ball.impactTick >= IMPACT_GAP) { ball.impactTick = tick; emit('impact', id, ball, { material, speed: vn }); }
            if (onHit) {
                ball.exempt = true;
                hit.nx = cinfo.nx; hit.ny = cinfo.ny; hit.vn = vn; hit.px = cinfo.cx; hit.py = cinfo.cy;
                onHit(ball, hit);
            }
        }
    }
    function contactPass(ball) {
        for (let i = 0; i < cand.length; i++) {
            const s = cand[i];
            if (!s.active) continue;
            if (s.oneWay !== null && ball.vx * s.oneWay[0] + ball.vy * s.oneWay[1] >= 0) continue;
            const touching = s.kind === 'seg'
                ? segContact(ball.x, ball.y, BALL_R, s.ax, s.ay, s.bx, s.by, ball.px, ball.py, cinfo)
                : circleContact(ball.x, ball.y, BALL_R, s.cx, s.cy, s.r, cinfo);
            if (!touching) continue;
            cinfo.svx = 0; cinfo.svy = 0;
            respond(ball, s.id, s.material, s.onHit, false);
        }
        for (let i = 0; i < flippers.length; i++) {
            const f = flippers[i];
            if (!flipperContact(f, ball.x, ball.y, BALL_R, cinfo)) continue;
            cinfo.cx = ball.x - cinfo.nx * BALL_R; cinfo.cy = ball.y - cinfo.ny * BALL_R;
            respond(ball, f.id, 'flipper', null, cinfo.svx !== 0 || cinfo.svy !== 0);
        }
    }
    function queryShapes(ball) {
        const m = BALL_R + 0.008;
        return grid.query(Math.min(ball.px, ball.x) - m, Math.min(ball.py, ball.y) - m, Math.max(ball.px, ball.x) + m, Math.max(ball.py, ball.y) + m, cand);
    }
    const live = b => b.mode === 'field' || b.mode === 'shooter';

    // Equal-mass collision between two balls (e from the 'ball' material, 0.93).
    function ballPair(a, b) {
        const dx = a.x - b.x, dy = a.y - b.y, d2 = dx * dx + dy * dy, rr = 2 * BALL_R;
        if (d2 >= rr * rr) return false;
        const d = Math.sqrt(d2);
        const nx = d > 1e-9 ? dx / d : 0, ny = d > 1e-9 ? dy / d : 1, pen = rr - d;
        a.x += nx * pen * 0.5; a.y += ny * pen * 0.5; b.x -= nx * pen * 0.5; b.y -= ny * pen * 0.5;
        const vn = -((a.vx - b.vx) * nx + (a.vy - b.vy) * ny);
        a.exempt = true; b.exempt = true;
        if (vn > 0) {
            const j = (1 + 0.93) * vn * 0.5;
            a.vx += nx * j; a.vy += ny * j; b.vx -= nx * j; b.vy -= ny * j;
            if (vn > 0.2 && tick - a.clackTick >= IMPACT_GAP && tick - b.clackTick >= IMPACT_GAP) {
                a.clackTick = b.clackTick = tick;
                emit('clack', 'ball', a, { other: b.id, speed: vn });
            }
        }
        return true;
    }

    // ---- plunger ------------------------------------------------------------------------------------------------
    const seatY = () => pl.yRest - pl.travel * pull;
    const inLane = b => b.x > pl.laneX[0] && b.y < pl.zoneYMax;
    function laneFree() {
        for (let i = 0; i < list.length; i++) if (live(list[i]) && list[i].x > pl.laneX[0] && list[i].y < pl.zoneYMax + 0.03) return false;
        return true;
    }
    function seatedBall() {
        let best = null;
        for (let i = 0; i < list.length; i++) {
            const b = list[i];
            if (b.mode === 'shooter' && b.y <= seatY() + 0.003 && b.vy <= 0.05 && (!best || b.y < best.y)) best = b;
        }
        return best;
    }
    function launch(ball, speed, auto) {
        ball.vx = 0; ball.vy = speed; ball.mode = 'field'; ball.auto = false;
        emit('launch', 'plunger', ball, { speed, auto });
    }
    function plungerTick(cmd) {
        if (pending.length && laneFree()) {
            const q = pending.shift(), b = addBallAt(q.id, pl.x, seatY(), 0, 0, 'shooter');
            b.auto = q.auto; b.serveTick = tick;
            trough = Math.max(0, trough - 1);
            emit('shooter', 'plunger', b);
        }
        if (pull > 0.05 && cmd < 0.02) {
            const b = seatedBall();
            if (b) launch(b, pl.vMin + (pl.vMax - pl.vMin) * pull, false);
            shown = Math.max(shown, pull); pull = 0;
        } else {
            const d = pl.pullRate * STEP;
            pull = cmd > pull ? Math.min(cmd, pull + d) : Math.max(cmd, pull - d);
        }
        shown = pull > shown ? pull : Math.max(pull, shown - SHOWN_DECAY * STEP);
        for (let i = 0; i < list.length; i++) {
            const b = list[i];
            if (b.auto && b.mode === 'shooter' && tick - b.serveTick >= AUTO_DELAY && b.y <= seatY() + 0.003) launch(b, pl.vAuto, true);
        }
    }

    // ---- tick ---------------------------------------------------------------------------------------------------
    function substep(h) {
        const nb = list.length;
        for (let i = 0; i < nb; i++) {
            const b = list[i];
            b.px = b.x; b.py = b.y; b.exempt = false;
            if (!live(b)) continue;
            b.e0 = 0.5 * (b.vx * b.vx + b.vy * b.vy) + G_SLOPE * b.y;
            b.vy -= G_SLOPE * h;
            const s2 = b.vx * b.vx + b.vy * b.vy;
            if (s2 > 0) {
                const s = Math.sqrt(s2), ns = Math.min(MAX_SPEED, s - ROLL_DECEL.field * h);
                if (ns <= 0) { b.vx = 0; b.vy = 0; } else { const k = ns / s; b.vx *= k; b.vy *= k; }
            }
            b.x += b.vx * h; b.y += b.vy * h;
        }
        for (let i = 0; i < flippers.length; i++) advanceFlipper(flippers[i], h);
        let paired = false;
        for (let i = 0; i < nb; i++) {
            const b = list[i];
            if (!live(b)) continue;
            queryShapes(b);
            contactPass(b); contactPass(b);
        }
        for (let i = 0; i < nb; i++) {
            const a = list[i];
            if (!live(a)) continue;
            for (let j = i + 1; j < nb; j++) {
                const b = list[j];
                if (live(b) && ballPair(a, b)) { paired = true; a.pushed = b.pushed = true; }
            }
        }
        for (let i = 0; i < nb; i++) {
            const b = list[i];
            if (!live(b)) continue;
            if (paired && b.pushed) { b.pushed = false; queryShapes(b); contactPass(b); }
            if (!b.exempt) capEnergy(b, b.e0, G_SLOPE);
        }
        devices.substep(h);
        paths.substep(h);
        for (let i = list.length - 1; i >= 0; i--) {
            const b = list[i];
            if (b.mode === 'field' && b.y < drainY && b.x < pl.laneX[0]) { emit('drain', 'drain', b); returnToTrough(b.id); }
        }
        const sy = seatY();
        for (let i = 0; i < list.length; i++) {
            const b = list[i];
            if (!live(b)) continue;
            const lane = inLane(b) && b.vy <= 0.05;
            b.mode = lane ? 'shooter' : 'field';
            if (lane && b.y <= sy) { b.y = sy; if (b.vy < 0) b.vy = 0; }
        }
    }

    function substepCount() {
        let vmax = 0, tip = 0;
        for (let i = 0; i < list.length; i++) {
            const b = list[i];
            if (!live(b)) continue;
            const v = Math.sqrt(b.vx * b.vx + b.vy * b.vy) + G_SLOPE * STEP;
            if (v > vmax) vmax = v;
        }
        for (let i = 0; i < flippers.length; i++) tip = Math.max(tip, flipperTipSpeed(flippers[i]));
        const n = Math.ceil((vmax + tip) * STEP / SUBSTEP_TRAVEL);
        return n < SUBSTEP_MIN ? SUBSTEP_MIN : (n > SUBSTEP_MAX ? SUBSTEP_MAX : n);
    }

    function ballSearch() {
        let field = false, held = false;
        for (let i = 0; i < list.length; i++) if (list[i].mode === 'field') field = true;
        for (let i = 0; i < flippers.length; i++) if (flippers[i].on) held = true;
        if (!field || held) { searchCycle = 0; nextSearch = tick + SEARCH_IDLE; return; }
        if (tick < nextSearch) return;
        if (searchCycle >= SEARCH_CYCLES) {
            for (let i = list.length - 1; i >= 0; i--) {
                const b = list[i];
                if (b.mode === 'field' && Math.hypot(b.vx, b.vy) < 0.1) { emit('searchFail', 'search', b); returnToTrough(b.id); }
            }
            searchCycle = 0; nextSearch = tick + SEARCH_IDLE;
            return;
        }
        searchCycle++; searchCount++;
        inSearch = true;
        devices.search(searchCycle);
        for (let i = 0; i < list.length; i++) {
            const b = list[i];
            if (b.mode === 'field' && Math.hypot(b.vx, b.vy) < 0.02) {
                const a = (rngNext(rng) - 0.5) * Math.PI * 0.6;
                b.vx += 0.3 * Math.sin(a); b.vy += 0.3 * Math.cos(a);
            }
        }
        emit('search', 'search', null, { cycle: searchCycle });
        inSearch = false;
        nextSearch = tick + SEARCH_EVERY;
    }

    function step(controls) {
        const c = controls || {};
        tick++;
        devices.preTick();
        for (let i = 0; i < flippers.length; i++) {
            const f = flippers[i], on = coils && !!c[f.key];
            if (on !== f.on) { f.on = on; emit('flipper', f.id, null, { up: on }); }
        }
        plungerTick(Math.max(0, Math.min(1, +c.plunger || 0)));
        const n = substepCount(), h = STEP / n;
        substeps = n;
        for (let k = 0; k < n; k++) substep(h);
        devices.postTick();
        ballSearch();
    }

    // ---- commands -----------------------------------------------------------------------------------------------
    function command(cmd) {
        if (!cmd) return false;
        switch (cmd.type) {
            case 'addBall': if (!balls.has(cmd.id) && !pending.some(q => q.id === cmd.id)) pending.push({ id: cmd.id, auto: !!cmd.auto }); return true;
            case 'coils': coils = !!cmd.on; return true;
            case 'nudge':
                for (const b of list) if (live(b)) { b.vx += +cmd.dx || 0; b.vy += +cmd.dy || 0; }
                return true;
            case 'removeBall': returnToTrough(cmd.id); devices.command(cmd); paths.command(cmd); return true;
            case 'clearBalls':
                balls.clear(); list.length = 0; pending.length = 0; trough = troughSize;
                devices.command(cmd); paths.command(cmd);
                return true;
            default: return devices.command(cmd) || paths.command(cmd);
        }
    }

    function reset() {
        balls.clear(); list.length = 0; pending.length = 0; queue = [];
        trough = troughSize; pull = 0; shown = 0; coils = true; rng.seed = seed >>> 0;
        searchCycle = 0; nextSearch = tick + SEARCH_IDLE;
        for (const f of flippers) resetFlipper(f);
        for (const s of shapeList) if (s.owner === 'wall' || s.owner === 'post') s.active = true;
        devices.reset(); paths.reset();
    }

    return {
        step, command, reset, animIndex, ctx,
        get trough() { return trough; },
        events() { const q = queue; queue = []; return q; },
        snapshot(out) {
            return writeSnapshot(out, { tick, balls: list, flippers, plunger: shown, animIndex, writeAnim: a => devices.writeAnim(a) });
        },
        placeBall(id, x, y, vx = 0, vy = 0) { return addBallAt(id, x, y, vx, vy, 'field'); },
        stats() {
            let held = 0;
            for (const b of list) if (b.mode === 'held') held++;
            return { balls: list.length, held, pending: pending.length, tick, substeps, searchCount };
        },
        debug() {
            return {
                segments: shapeList.filter(s => s.kind === 'seg').map(s => ({ id: s.id, ax: s.ax, ay: s.ay, bx: s.bx, by: s.by, material: s.material, active: s.active })),
                circles: shapeList.filter(s => s.kind === 'circle').map(s => ({ id: s.id, cx: s.cx, cy: s.cy, r: s.r, material: s.material, active: s.active })),
                flippers: flippers.map(f => ({ id: f.id, x: f.x, y: f.y, side: f.side, a: f.a, w: f.w, on: f.on, length: f.length, rBase: f.rBase, rTip: f.rTip, rest: f.rest, up: f.up })),
                devices: devices.debug(), paths: paths.debug()
            };
        },
        dispose() { balls.clear(); list.length = 0; pending.length = 0; queue = []; }
    };
}

export default createWorld;
