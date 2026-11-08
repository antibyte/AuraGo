// Shot reachability sweep and soak for pinball layouts (plan T14). Node only.
//   sweep(layout, opts) -> { shots: {id: {byFlipper: {fid: {start, delay, peak}}}}, flippers: {fid: [shotIds]}, missing, runs }
//   soak(layout, opts)  -> { ok, problems, nan, escaped, restSeconds, searchFail, switches, ... }
import { createWorld } from '../../ui/js/desktop/apps/pinball/physics/world.js';
import { STEP, BALL_R, FLIPPER } from '../../ui/js/desktop/apps/pinball/physics/constants.js';
import { matchEvent } from '../../ui/js/desktop/apps/pinball/contracts.js';
import { outline } from './pinball-sim.mjs';
import { createBot } from '../../ui/testdata/pinball-bot.js';

const SECONDS = 3;
const SETTLE_TICKS = 800;

function guidePlacement(layout, flipper) {
    const w = (layout.walls || []).find(x => x.id === 'inlaneGuide' + (flipper.id === 'left' ? 'L' : 'R'));
    if (!w) return null;
    const [a, b] = w.points, t = 0.8;
    const x = a[0] + (b[0] - a[0]) * t, y = a[1] + (b[1] - a[1]) * t;
    let nx = -(b[1] - a[1]), ny = b[0] - a[0];
    if (ny < 0) { nx = -nx; ny = -ny; }
    const l = Math.hypot(nx, ny);
    return [x + nx / l * (BALL_R + 0.0003), y + ny / l * (BALL_R + 0.0003)];
}

function midBat(f) {
    const len = f.length ?? FLIPPER.length, rest = f.rest ?? FLIPPER.rest;
    return [f.x + f.side * len / 2 * Math.cos(rest), f.y + len / 2 * Math.sin(rest)];
}

function setup(layout, seed) {
    const world = createWorld(layout, { seed });
    for (const shot of layout.shots || []) for (const cmd of shot.setup || []) world.command(cmd);
    return world;
}

// One run: returns { hits: Map(shotId -> peak speed at the time), } for the given flipper/start/delay.
function runCase(layout, flipper, start, delay, seed) {
    const world = setup(layout, seed);
    const key = flipper.key, hits = new Map();
    const input = {};
    let begin;
    if (start === 'drop') {
        const [mx, my] = midBat(flipper);
        world.placeBall(1, mx, my + 0.06, 0, 0);
        begin = 0;
    } else {
        const p = guidePlacement(layout, flipper);
        if (!p) return hits;
        world.placeBall(1, p[0], p[1], 0, 0);
        input[key] = true;
        for (let t = 0; t < SETTLE_TICKS; t++) { world.step(input); world.events(); }
        input[key] = false;
        begin = world.snapshot().tick;
    }
    const total = Math.round(SECONDS / STEP) + delay;
    let peak = 0;
    for (let t = 0; t < total; t++) {
        input[key] = t >= delay;
        world.step(input);
        const ball = world.ctx.balls.get(1);
        if (ball) peak = Math.max(peak, Math.hypot(ball.vx, ball.vy));
        let drained = false;
        for (const ev of world.events()) {
            if (ev.type === 'drain') drained = true;
            for (const shot of layout.shots) if (!hits.has(shot.id) && matchEvent(shot.on, ev)) hits.set(shot.id, peak);
        }
        if (drained) break;
    }
    void begin;
    return hits;
}

// Delays are ticks (1/480 s). `drop` fires 0..120 ticks after the release; a `cradle` ball needs about 220 ticks to roll
// from the guide to the bat tip, so its delays run to 270 to cover the whole bat.
const DELAYS = { drop: Array.from({ length: 41 }, (_, i) => i * 3), cradle: Array.from({ length: 91 }, (_, i) => i * 3) };

export function sweep(layout, { delays = null, seed = 1 } = {}) {
    const shots = {}, flippers = {};
    for (const s of layout.shots) shots[s.id] = { byFlipper: {} };
    let runs = 0;
    for (const f of layout.flippers) {
        flippers[f.id] = new Set();
        const starts = f.id === 'upperRight' || f.id.startsWith('upper') ? ['drop'] : ['cradle', 'drop'];
        for (const start of starts) for (const delay of delays || DELAYS[start]) {
            runs++;
            for (const [id, peak] of runCase(layout, f, start, delay, seed)) {
                flippers[f.id].add(id);
                const cur = shots[id].byFlipper[f.id];
                if (!cur || peak > cur.peak) shots[id].byFlipper[f.id] = { start, delay, peak: +peak.toFixed(2) };
            }
        }
    }
    const missing = layout.shots.filter(s => !Object.keys(shots[s.id].byFlipper).length).map(s => s.id);
    return { shots, flippers: Object.fromEntries(Object.entries(flippers).map(([k, v]) => [k, [...v]])), missing, runs };
}

// Shots whose arrow lies more than 3 cm beyond the centre line, on the side away from the flipper's pivot, are cross shots.
export function crossShots(layout, flipperId, reached) {
    const cx = layout.meta.cx, f = layout.flippers.find(x => x.id === flipperId);
    return reached.filter(id => {
        const lamp = layout.lamps.find(l => l.id === 'arrow_' + id);
        return !!lamp && f.side * (lamp.x - cx) > 0.03;
    });
}

const SWITCH_TYPES = new Set(['bumper', 'sling', 'drop', 'bankDown', 'standup', 'spinner', 'rollover', 'trigger', 'gate', 'hole', 'pathMade', 'pathSwitch', 'kickback', 'captive', 'pathEnter', 'bankReset', 'eject']);

export function soak(layout, { seed = 7, balls = 4, seconds = 60, restLimit = 8, restRadius = 0.02 } = {}) {
    const world = createWorld(layout, { seed }), bot = createBot(layout, seed * 31 + 5), box = outline(layout);
    const problems = [], switches = new Set(), anchors = new Map(), counts = {};
    let nan = 0, escaped = 0, searchFail = 0, maxRest = 0;
    for (let i = 1; i <= balls; i++) world.command({ type: 'addBall', id: i });
    const ticks = Math.round(seconds / STEP), restTicks = Math.round(restLimit / STEP);
    // Exercise the switchable devices the way rules would: arm kickbacks, raise banks, flip diverters.
    const timed = [];
    for (const d of layout.devices || []) {
        if (d.type === 'kickback') timed.push({ every: 5, cmd: () => ({ type: 'kickback', id: d.id, armed: true }) });
        else if (d.type === 'dropBank') timed.push({ every: 9, cmd: () => ({ type: 'resetBank', id: d.id }) });
        else if (d.type === 'diverter') { let on = false; timed.push({ every: 7, cmd: () => ({ type: 'diverter', id: d.id, on: (on = !on) }) }); }
    }
    for (const q of timed) if (q.every >= 5) world.command(q.cmd());
    for (let t = 0; t < ticks; t++) {
        for (const q of timed) if (t > 0 && t % Math.round(q.every / STEP) === 0) world.command(q.cmd());
        const all = [...world.ctx.balls.values()];
        world.step(bot.controls(all, t));
        for (const ev of world.events()) {
            counts[ev.type] = (counts[ev.type] || 0) + 1;
            if (SWITCH_TYPES.has(ev.type)) switches.add(ev.type + ':' + ev.id);
            if (ev.type === 'searchFail') { searchFail++; problems.push('searchFail at ' + (t * STEP).toFixed(1) + ' s near (' + ev.x?.toFixed(3) + ', ' + ev.y?.toFixed(3) + ')'); }
            if (ev.type === 'drain' || ev.type === 'searchFail') world.command({ type: 'addBall', id: ev.ball });
        }
        for (const b of world.ctx.balls.values()) {
            if (![b.x, b.y, b.vx, b.vy].every(Number.isFinite)) { nan++; continue; }
            if (b.mode !== 'field' && b.mode !== 'shooter') { anchors.delete(b.id); continue; }
            if (b.mode === 'field' && !box.contains(b.x, b.y)) { escaped++; if (escaped < 4) problems.push('ball ' + b.id + ' escaped at (' + b.x.toFixed(3) + ', ' + b.y.toFixed(3) + ') t=' + (t * STEP).toFixed(1)); }
            if (b.mode !== 'field') { anchors.delete(b.id); continue; }
            const a = anchors.get(b.id);
            if (!a || Math.hypot(b.x - a.x, b.y - a.y) > restRadius) { anchors.set(b.id, { x: b.x, y: b.y, t }); continue; }
            maxRest = Math.max(maxRest, (t - a.t) * STEP);
            if (t - a.t >= restTicks) { problems.push('ball ' + b.id + ' rested at (' + b.x.toFixed(3) + ', ' + b.y.toFixed(3) + ') for ' + restLimit + ' s'); a.t = t; }
        }
    }
    if (nan) problems.push(nan + ' NaN samples');
    if (switches.size < 30) problems.push('only ' + switches.size + ' distinct switches');
    return { ok: problems.length === 0, problems, nan, escaped, searchFail, maxRest: +maxRest.toFixed(1), switches: switches.size, switchList: [...switches].sort(), counts };
}
