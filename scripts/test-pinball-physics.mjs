// Physics core suite (T02): contacts, energy, flipper feel, plunger, trough, ball-ball, nudge, determinism.
import assert from 'node:assert/strict';
import { createWorld } from '../ui/js/desktop/apps/pinball/physics/world.js';
import { STEP, BALL_R, FLIPPER } from '../ui/js/desktop/apps/pinball/physics/constants.js';
import { lerpSnapshot } from '../ui/js/desktop/apps/pinball/physics/snapshot.js';
import { rngNext } from '../ui/js/desktop/apps/pinball/config.js';
import { TABLES } from '../ui/js/desktop/apps/pinball/tables/index.js';
import { run, runFrames, energy, outline, crossesWall } from './lib/pinball-sim.mjs';

const neon = TABLES.neon.layout, cx = neon.meta.cx;
const flip = id => neon.flippers.find(f => f.id === id);
const near = (a, b, tol, what) => assert.ok(Math.abs(a - b) <= tol, what + ': ' + a + ' vs ' + b + ' (tol ' + tol + ')');

// A closed box with thin interior walls and posts; no plunger zone, no drain.
function boxLayout(half = 0.15, withObstacles = true) {
    const walls = [{ id: 'box', material: 'metal', closed: true, points: [[-half, 0.2], [half, 0.2], [half, 0.8], [-half, 0.8]] }];
    const posts = [];
    if (withObstacles) {
        walls.push({ id: 'bar1', material: 'wood', closed: false, points: [[-0.12, 0.35], [-0.02, 0.45]] });
        walls.push({ id: 'bar2', material: 'rubber', closed: false, points: [[0.03, 0.62], [0.12, 0.55], [0.12, 0.7]] });
        for (const [x, y] of [[-0.06, 0.6], [0.05, 0.4], [0.0, 0.52], [0.09, 0.3], [-0.1, 0.7]]) posts.push({ id: 'p' + posts.length, x, y, r: 0.005, material: 'metal' });
    }
    return { id: 'box', version: 1, meta: { troughSize: 4 }, flippers: [], plunger: { laneX: [9, 10], zoneYMax: 0 }, drainY: -1, walls, posts, devices: [], paths: [] };
}

// 1. Tunnelling: 200 seeded 10 m/s shots, 0.4 s each.
{
    const layout = boxLayout(), out = outline(layout, 'box'), rng = { seed: 7 };
    let shots = 0, bounces = 0;
    for (let n = 0; n < 200; n++) {
        const w = createWorld(layout, { seed: n + 1 });
        let x, y, ok = false;
        for (let tries = 0; tries < 50 && !ok; tries++) {
            x = -0.12 + rngNext(rng) * 0.24; y = 0.25 + rngNext(rng) * 0.5;
            ok = layout.posts.every(p => Math.hypot(x - p.x, y - p.y) > 0.04) && layout.walls.slice(1).every(wl => out.segments.filter(s => s.id === wl.id).every(s => distSeg(x, y, s) > 0.04));
        }
        const a = rngNext(rng) * Math.PI * 2;
        w.placeBall(1, x, y, 10 * Math.cos(a), 10 * Math.sin(a));
        let prev = { x, y };
        const events = run(w, 192, null, (t, ww) => {
            const b = ww.snapshot().balls[0];
            assert.equal(crossesWall(prev.x, prev.y, b.x, b.y, out), null, 'shot ' + n + ' tick ' + t + ' crossed a wall');
            assert.ok(out.contains(b.x, b.y), 'shot ' + n + ' left the outline');
            for (const p of layout.posts) assert.ok(Math.hypot(b.x - p.x, b.y - p.y) > p.r, 'shot ' + n + ' entered a post');
            prev = { x: b.x, y: b.y };
        });
        bounces += events.filter(e => e.type === 'impact').length; shots++;
    }
    assert.equal(shots, 200); assert.ok(bounces > 400, 'the shots must actually bounce around: ' + bounces);
}
function distSeg(px, py, s) {
    const dx = s.b[0] - s.a[0], dy = s.b[1] - s.a[1], t = Math.max(0, Math.min(1, ((px - s.a[0]) * dx + (py - s.a[1]) * dy) / (dx * dx + dy * dy)));
    return Math.hypot(px - s.a[0] - t * dx, py - s.a[1] - t * dy);
}

// 2. Passive energy never increases (50 seeded balls, 2 s, Neon with unpressed flippers).
{
    const rng = { seed: 99 };
    let ticks = 0;
    for (let n = 0; n < 50; n++) {
        const w = createWorld(neon, { seed: n + 1 });
        const a = rngNext(rng) * Math.PI * 2, v = 0.5 + rngNext(rng) * 3;
        w.placeBall(1, -0.15 + rngNext(rng) * 0.3, 0.25 + rngNext(rng) * 0.45, v * Math.cos(a), v * Math.sin(a));
        let last = null;
        run(w, 960, null, (t, ww) => {
            const b = ww.snapshot().balls[0];
            if (!b || b.mode === 'shooter') { last = null; return; }
            const e = energy(b);
            if (last !== null) assert.ok(e <= last + 1e-9, 'ball ' + n + ' tick ' + t + ' gained ' + (e - last) + ' J');
            last = e; ticks++;
        });
    }
    assert.ok(ticks > 20000, 'enough tracked ticks: ' + ticks);
}

// 3. Stroke timing and angular speed limit.
const strokes = {};
{
    const w = createWorld(neon), omegaMax = FLIPPER.omegaMax;
    let upAt = -1, backAt = -1, maxW = 0;
    const evs = [];
    for (let t = 1; t <= 90; t++) {
        w.step({ left: true });
        evs.push(...w.events());
        const f = w.snapshot().flippers;
        maxW = Math.max(maxW, Math.abs(f[1]));
        if (upAt < 0 && f[0] >= FLIPPER.up - 0.02) upAt = t;
    }
    for (let t = 1; t <= 90; t++) {
        w.step({ left: false });
        evs.push(...w.events());
        const f = w.snapshot().flippers;
        maxW = Math.max(maxW, Math.abs(f[1]));
        if (backAt < 0 && f[0] <= FLIPPER.rest + 0.02) backAt = t;
    }
    strokes.up = upAt * STEP; strokes.back = backAt * STEP;
    assert.ok(strokes.up >= 0.026 && strokes.up <= 0.040, 'stroke up ' + strokes.up);
    assert.ok(strokes.back > 0 && strokes.back <= 0.060, 'stroke back ' + strokes.back);
    assert.ok(maxW <= omegaMax + 1e-9, 'omega ' + maxW);
    assert.deepEqual(evs.filter(e => e.type === 'flipper').map(e => e.up), [true, false]);
    // Coils off keeps the bat down.
    const w2 = createWorld(neon);
    w2.command({ type: 'coils', on: false });
    for (let t = 0; t < 60; t++) w2.step({ left: true, right: true });
    assert.equal(w2.snapshot().flippers[0], FLIPPER.rest);
}

// 4. Tip shots. A ball that leaves the inlane guide and falls onto the swinging bat tip is struck at full speed.
const tip = { best: 0, bestRest: 0, rebound: 0 };
{
    const L = flip('left'), tipX = L.x + FLIPPER.length * Math.cos(FLIPPER.rest), tipY = L.y + FLIPPER.length * Math.sin(FLIPPER.rest);
    const shot = (delay, dx, h, pressed) => {
        const w = createWorld(neon);
        w.placeBall(1, tipX + dx, tipY + h, 0, -0.3);
        let vmax = 0;
        run(w, delay + 110, t => ({ left: pressed && t >= delay }), (t, ww) => {
            const b = ww.snapshot().balls[0];
            if (b && t >= delay) vmax = Math.max(vmax, Math.hypot(b.vx, b.vy));
        });
        return vmax;
    };
    for (const dx of [-0.01, 0, 0.005, 0.01]) {
        for (const h of [0.05, 0.08, 0.12]) {
            for (let d = 0; d <= 80; d += 2) tip.best = Math.max(tip.best, shot(d, dx, h, true));
            tip.rebound = Math.max(tip.rebound, shot(0, dx, h, false));
        }
    }
    assert.ok(tip.best >= 5.0 && tip.best <= 8.0, 'tip shot exit speed ' + tip.best);
    assert.ok(tip.rebound < 2.0, 'unpressed rebound ' + tip.rebound);
    // The inlane feeds the resting bat: a ball rolling down the guide reaches the tip region instead of stalling at the pivot.
    const w = createWorld(neon);
    w.placeBall(1, cx - 0.1855, 0.38, 0, -0.3);
    const evs = run(w, 700);
    assert.ok(evs.some(e => e.type === 'drain'), 'a ball in the left inlane rolls over the resting bat and drains');
}

// 5. Cradle: held flipper, ball dropped on the bat.
const cradle = {};
{
    const L = flip('left'), w = createWorld(neon);
    w.placeBall(1, L.x + 0.04, L.y + 0.07, 0, 0);
    const pos = [];
    run(w, 3 * 480, () => ({ left: true }), (t, ww) => { const b = ww.snapshot().balls[0]; pos.push([b.x, b.y, Math.hypot(b.vx, b.vy)]); });
    const last = pos[pos.length - 1], ref = pos[480];
    let drift = 0;
    for (let i = 480; i < pos.length; i++) drift = Math.max(drift, Math.hypot(pos[i][0] - ref[0], pos[i][1] - ref[1]));
    Object.assign(cradle, { speed: last[2], drift, pivot: Math.hypot(last[0] - L.x, last[1] - L.y) });
    assert.ok(last[2] < 0.02, 'cradle speed ' + last[2]);
    assert.ok(drift < 0.003, 'cradle drift ' + drift);
    assert.ok(cradle.pivot < 0.05, 'cradle distance to pivot ' + cradle.pivot);
    const before = pos[pos.length - 1];
    run(w, 240, () => ({ left: false }));
    const b = w.snapshot().balls[0];
    assert.ok(!b || Math.hypot(b.x - before[0], b.y - before[1]) > 0.02, 'releasing the flipper frees the ball');
}

// 6. Plunger.
const plungerSpeeds = {};
{
    const w = createWorld(neon);
    w.command({ type: 'addBall', id: 1 });
    let evs = run(w, 4);
    assert.ok(evs.some(e => e.type === 'shooter'), 'ball served');
    assert.equal(w.trough, 3);
    assert.equal(w.snapshot().balls[0].mode, 'shooter');
    let released = -1;
    evs = run(w, 600, t => ({ plunger: t < 330 ? 1 : 0 }), (t, ww) => { if (released < 0 && ww.snapshot().balls[0].y > 0.80) released = t; });
    const launch = evs.find(e => e.type === 'launch');
    near(launch.speed, 3.4, 0.05, 'full pull launch');
    assert.equal(launch.auto, false);
    assert.ok(released >= 0 && (released - 330) / 480 < 1, 'ball passes y 0.80 within 1 s: ' + released);
    plungerSpeeds.full = launch.speed;

    const w2 = createWorld(neon);
    w2.command({ type: 'addBall', id: 1 });
    evs = run(w2, 150, t => ({ plunger: t < 120 ? 0.3 : 0 }));
    const half = evs.find(e => e.type === 'launch');
    near(half.speed, 1.405, 0.02, 'partial pull launch');
    plungerSpeeds.part = half.speed;

    const w3 = createWorld(neon);
    w3.command({ type: 'addBall', id: 1, auto: true });
    evs = run(w3, 384);
    const auto = evs.find(e => e.type === 'launch');
    assert.ok(auto && auto.auto === true, 'auto launch within 0.8 s');
    near(auto.speed, 3.0, 1e-9, 'auto speed');
}

// 7. Drain and trough.
{
    const w = createWorld(neon);
    w.command({ type: 'addBall', id: 1 });
    run(w, 4);
    assert.equal(w.trough, 3);
    w.placeBall(1, cx, 0.2, 0, -0.5);
    const evs = run(w, 400);
    const drain = evs.filter(e => e.type === 'drain');
    assert.equal(drain.length, 1);
    assert.equal(drain[0].ball, 1);
    assert.equal(w.trough, 4);
    assert.equal(w.stats().balls, 0);
    assert.equal(w.snapshot().balls.length, 0);
}

// 8. Head-on ball-ball: momentum and restitution.
const pair = {};
{
    const w = createWorld(boxLayout(0.25, false));
    w.placeBall(1, -0.1, 0.5, 1.5, 0);
    w.placeBall(2, 0.1, 0.5, -1.5, 0);
    let before = null, after = null, clack = null;
    for (let t = 0; t < 200 && !after; t++) {
        const s0 = w.snapshot().balls.map(b => b.vx);
        w.step({});
        const ev = w.events().find(e => e.type === 'clack');
        if (ev) { clack = ev; before = s0; after = w.snapshot().balls.map(b => b.vx); }
    }
    assert.ok(clack, 'balls collide');
    near(after[0] + after[1], before[0] + before[1], 1e-9, 'momentum');
    pair.e = (after[1] - after[0]) / (before[0] - before[1]);
    near(pair.e, 0.93, 0.01, 'restitution');
}

// 9. Nudge.
{
    const w = createWorld(neon);
    w.placeBall(1, 0, 0.5, 0, 0);
    w.command({ type: 'nudge', dx: 0.2, dy: 0 });
    assert.equal(w.snapshot().balls[0].vx, 0.2);
    w.command({ type: 'nudge', dx: -0.2, dy: 0.14 });
    assert.equal(w.snapshot().balls[0].vx, 0);
    assert.equal(w.snapshot().balls[0].vy, 0.14);
}

// 10. Determinism across frame accumulators.
{
    const script = (t, w) => {
        if (t === 700) w.command({ type: 'nudge', dx: 0.2, dy: 0.14 });
        return { left: (t % 240) > 100 && (t % 240) < 160, right: (t % 300) > 150 && (t % 300) < 200, plunger: t > 1000 && t < 1100 ? 1 : 0 };
    };
    const play = hz => {
        const w = createWorld(neon, { seed: 5 });
        for (let id = 1; id <= 3; id++) w.command({ type: 'addBall', id, auto: id !== 3 });
        const events = hz ? runFrames(w, hz, 1440, script) : run(w, 1440, script);
        return { snap: w.snapshot(), events, stats: w.stats() };
    };
    const base = play(0), ref = play(60);
    assert.ok(base.events.some(e => e.type === 'launch'), 'balls launched');
    for (const hz of [30, 144]) {
        const got = play(hz);
        assert.deepEqual(got.snap, ref.snap, 'snapshots equal at ' + hz + ' Hz');
        assert.deepEqual(got.events, ref.events, 'events equal at ' + hz + ' Hz');
    }
    assert.deepEqual(base.snap, ref.snap);
    assert.deepEqual(base.events, ref.events);
    // Interpolation keeps ids and stays between the snapshots.
    const a = createWorld(neon), b = createWorld(neon);
    a.placeBall(1, 0, 0.5, 0, 0); b.placeBall(1, 0.01, 0.5, 0, 0);
    const mid = lerpSnapshot(a.snapshot(), b.snapshot(), 0.5);
    near(mid.balls[0].x, 0.005, 1e-12, 'lerp');
}

// Cost of a 4-ball step: Neon with flippers working, drained balls are put back so four are always in play.
{
    const w = createWorld(neon), rng = { seed: 3 };
    const refill = () => {
        const have = new Set(w.snapshot().balls.map(b => b.id));
        for (let id = 1; id <= 4; id++) if (!have.has(id)) w.placeBall(id, -0.12 + rngNext(rng) * 0.2, 0.55 + rngNext(rng) * 0.1, (rngNext(rng) - 0.5) * 2, -0.5);
    };
    for (let t = 0; t < 480; t++) { refill(); w.step({ left: (t >> 5) & 1, right: (t >> 6) & 1 }); w.events(); }
    const ticks = 4800, t0 = process.hrtime.bigint();
    let substeps = 0;
    for (let t = 0; t < ticks; t++) { refill(); w.step({ left: (t >> 5) & 1, right: (t >> 6) & 1 }); w.events(); substeps += w.stats().substeps; }
    const ms = Number(process.hrtime.bigint() - t0) / 1e6 / ticks * 8;
    console.log('  4-ball cost: ' + ms.toFixed(3) + ' ms per 8-tick frame, ' + (substeps / ticks).toFixed(1) + ' substeps per tick');
    assert.ok(ms < 2, '4-ball frame cost ' + ms);
}

console.log('physics ok: tip ' + tip.best.toFixed(2) + ' m/s, rebound ' + tip.rebound.toFixed(2) + ', stroke up ' + (strokes.up * 1000).toFixed(1) + ' ms / back ' + (strokes.back * 1000).toFixed(1) + ' ms, cradle drift ' + cradle.drift.toExponential(2) + ' speed ' + cradle.speed.toExponential(2) + ', plunger ' + plungerSpeeds.full + '/' + plungerSpeeds.part.toFixed(3) + ', ball-ball e ' + pair.e.toFixed(4));
