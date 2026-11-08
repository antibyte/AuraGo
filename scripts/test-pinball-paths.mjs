// T04 path tests with synthetic layouts. Run: node scripts/test-pinball-paths.mjs
import assert from 'node:assert/strict';
import { createWorld } from '../ui/js/desktop/apps/pinball/physics/world.js';
import { createPaths } from '../ui/js/desktop/apps/pinball/physics/paths.js';
import { BALL_R } from '../ui/js/desktop/apps/pinball/physics/constants.js';

const SEP = 2 * BALL_R;
let passed = 0;
function test(name, fn) { fn(); passed++; console.log('ok   ' + name); }

const RAMP = { id: 'ramp', kind: 'plastic', entry: { a: [-0.02, 0.4], b: [0.02, 0.4] }, points: [[0, 0.4, 0], [0, 0.7, 0.04]], width: 0.05,
    exit: { kind: 'field', dir: [0, 1], speedScale: 0.9 } };
const mk = paths => createWorld({ walls: [], posts: [], flippers: [], devices: [], paths, meta: { troughSize: 4 } }, { seed: 1 });

// Steps until `type` shows up (or maxTicks); returns {ev, all, ticks}.
function until(world, type, maxTicks, each) {
    const all = [];
    for (let t = 0; t < maxTicks; t++) {
        world.step({});
        let hit = null;
        for (const e of world.events()) { all.push(e); if (!hit && e.type === type) hit = e; }
        if (hit) return { ev: hit, all, ticks: t };
        if (each) each();
    }
    return { ev: null, all, ticks: maxTicks };
}
const ballOf = (w, id) => w.ctx.balls.get(id);

test('arc length within 1% of the polyline length', () => {
    const ctx = { emit() {}, balls: new Map() };
    const straight = createPaths([RAMP], ctx).debug()[0];
    assert.ok(Math.abs(straight.length - Math.hypot(0.3, 0.04)) / Math.hypot(0.3, 0.04) < 0.01);
    assert.equal(straight.samples.length, 201);
    const r = 0.2, pts = [];
    for (let i = 0; i <= 4; i++) { const a = i / 4 * Math.PI / 2; pts.push([r * Math.cos(a), 0.3 + r * Math.sin(a), 0]); }
    const curve = createPaths([{ id: 'c', kind: 'wire', entry: { a: [0.18, 0.3], b: [0.22, 0.3] }, points: pts }], ctx).debug()[0];
    assert.ok(Math.abs(curve.length - r * Math.PI / 2) / (r * Math.PI / 2) < 0.01, 'curve length ' + curve.length);
});

test('0.9 m/s entry fails and the ball returns moving away from the ramp', () => {
    const w = mk([RAMP]);
    w.placeBall(1, 0, 0.395, 0, 0.9);
    const r = until(w, 'pathFail', 3000);
    assert.ok(r.all.some(e => e.type === 'pathEnter'), 'entered');
    assert.ok(r.ev, 'pathFail fired');
    const b = ballOf(w, 1);
    assert.equal(b.mode, 'field');
    assert.ok(b.vy < 0 && b.y <= 0.4 + 1e-3);
    assert.ok(!r.all.some(e => e.type === 'pathMade'));
    for (let i = 0; i < 20; i++) w.step({});
    assert.ok(!w.events().some(e => e.type === 'pathEnter'), 'moving away does not re-enter');
});

test('3.5 m/s makes the ramp: exit position, direction and speed = v_end * 0.9', () => {
    const w = mk([RAMP]);
    w.placeBall(1, 0, 0.395, 0, 3.5);
    const r = until(w, 'pathMade', 600);
    assert.ok(r.ev && r.ev.exit === 'field');
    const b = ballOf(w, 1);
    assert.equal(b.mode, 'field');
    assert.ok(Math.abs(b.x) < 1e-6 && Math.abs(b.y - 0.7) < 0.02, 'exit position ' + b.y);
    assert.ok(Math.abs(b.vx) < 1e-6 && b.vy > 0);
    assert.ok(Math.abs(b.vy - 0.9 * r.ev.speed) < 0.03, b.vy + ' vs ' + 0.9 * r.ev.speed);
    assert.ok(r.ev.speed < 3.5 && r.ev.speed > 2.5);
});

test('switch exit: diverter off -> field, on -> hole event via path', () => {
    const def = { ...RAMP, exit: { kind: 'switch', diverter: 'd1', on: { kind: 'hole', hole: 'h1' }, off: { kind: 'field', dir: [0, 1], speedScale: 0.9 } } };
    for (const on of [false, true]) {
        const w = mk([def]);
        const delivered = [];
        w.ctx.deliverToHole = (ball, id) => { delivered.push(id); ball.mode = 'held'; w.ctx.emit('hole', id, ball, { via: 'path', count: 1 }); return true; };
        w.command({ type: 'diverter', id: 'd1', on });
        w.placeBall(1, 0, 0.395, 0, 3.5);
        const r = until(w, 'pathMade', 600);
        assert.equal(r.ev.exit, on ? 'hole' : 'field');
        const evs = r.all.concat(w.events());
        const hole = evs.find(e => e.type === 'hole');
        if (on) { assert.equal(hole.via, 'path'); assert.deepEqual(delivered, ['h1']); } else assert.ok(!hole && !delivered.length);
    }
});

test('VUK eject travels its path and a backward crossing does not enter', () => {
    const tube = { id: 'vuk', kind: 'tube', entry: { a: [-0.02, 0.5], b: [0.02, 0.5] }, points: [[0, 0.5, 0.02], [0, 0.56, 0.02], [0.1, 0.6, 0.02]],
        exit: { kind: 'field', dir: [1, 0], speedScale: 0.9 }, switches: [{ id: 'sw1', at: 0.5 }] };
    const w = mk([tube]);
    w.placeBall(1, 0, 0.8, 0, 0);
    assert.equal(w.ctx.launchPath(ballOf(w, 1), 'vuk', 1.5), true);
    assert.equal(ballOf(w, 1).mode, 'path');
    const r = until(w, 'pathMade', 400);
    assert.ok(r.ev);
    assert.ok(r.all.some(e => e.type === 'pathEnter') && r.all.some(e => e.type === 'pathSwitch' && e.id === 'sw1' && e.path === 'vuk'));
    assert.ok(Math.abs(ballOf(w, 1).x - 0.1) < 0.02);
    const w2 = mk([{ ...RAMP, points: [[0, 0.4, 0], [0, 0.7, 0]] }]);
    w2.placeBall(1, 0, 0.405, 0, -1);
    for (let i = 0; i < 20; i++) w2.step({});
    assert.ok(!w2.events().some(e => e.type === 'pathEnter'));
});

test('two balls on one path do not pass each other', () => {
    const zig = { id: 'z', kind: 'wire', entry: { a: [-0.22, 0.48], b: [-0.18, 0.52] }, points: [[-0.2, 0.5, 0.02], [0.2, 0.5, 0.02], [0.2, 0.55, 0.02], [-0.2, 0.55, 0.02]],
        exit: { kind: 'field', dir: [-1, 0], speedScale: 0.9 } };
    const w = mk([zig]);
    w.placeBall(1, 0, 0.8, 0, 0); w.placeBall(2, 0.1, 0.8, 0, 0);
    w.ctx.launchPath(ballOf(w, 1), 'z', 0.6);
    let min = Infinity, swapped = false;
    for (let t = 0; t < 600; t++) {
        if (t === 100) w.ctx.launchPath(ballOf(w, 2), 'z', 3);
        w.step({});
        const a = ballOf(w, 1), b = ballOf(w, 2);
        if (a.path && b.path) {
            min = Math.min(min, a.path.s - b.path.s);
            if (b.path.v < 2.8) swapped = true;
        } else if (t > 100) break;
    }
    assert.ok(min >= SEP - 1e-6, 'min gap ' + min);
    assert.ok(swapped, 'velocities were exchanged');
});

test('snapshot h equals the spline h and mode is path', () => {
    const w = mk([RAMP]);
    w.placeBall(1, 0, 0.395, 0, 3.5);
    let seen = 0;
    for (let t = 0; t < 200 && !seen; t++) {
        w.step({});
        const b = ballOf(w, 1);
        if (b.mode !== 'path') continue;
        const e = w.snapshot().balls[0];
        assert.equal(e.mode, 'path');
        assert.ok(e.h > 0 && Math.abs(e.h - 0.04 * (e.y - 0.4) / 0.3) < 1e-6);
        seen = 1;
    }
    assert.ok(seen);
});

console.log('pinball paths: ' + passed + ' tests ok');
