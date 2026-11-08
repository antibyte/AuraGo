// Lost Temple of the Sun layout tests (plan T23). Run: node scripts/test-pinball-layout-temple.mjs
import assert from 'node:assert/strict';
import { TABLES } from '../ui/js/desktop/apps/pinball/tables/index.js';
import { validateTable } from '../ui/js/desktop/apps/pinball/tables/validate.js';
import { createWorld } from '../ui/js/desktop/apps/pinball/physics/world.js';
import { STEP, FLIPPER, BALL_R } from '../ui/js/desktop/apps/pinball/physics/constants.js';
import { distPointSeg } from '../ui/js/desktop/apps/pinball/tables/kit/geom.js';

const layout = TABLES.temple.layout;
let passed = 0;
function test(name, fn) { fn(); passed++; console.log('ok   ' + name); }

const fresh = () => createWorld(layout, { seed: 3 });
const has = (events, type, id) => events.some(e => e.type === type && (id === undefined || e.id === id));
// Steps `seconds` with the given input; `each(ball, tick)` sees the ball after every step.
function play(world, seconds, input = {}, each) {
    const events = [];
    for (let t = 0; t < Math.round(seconds / STEP); t++) {
        world.step(typeof input === 'function' ? input(t) : input);
        for (const ev of world.events()) events.push(ev);
        const b = world.ctx.balls.get(1);
        if (b && each) each(b, t);
    }
    return { events, ball: world.ctx.balls.get(1) };
}

test('the temple layout validates and keeps the agreed ids', () => {
    assert.deepEqual(validateTable(TABLES.temple).errors, []);
    assert.deepEqual(layout.shots.map(s => s.id), ['leftOrbit', 'leftRamp', 'idol', 'rightRamp', 'rightOrbit', 'lock']);
    assert.deepEqual(layout.shots.map(s => s.on), ['trigger:leftOrbitUp@1', 'pathMade:leftRamp', 'hole:idolScoop', 'pathMade:rightRamp', 'trigger:rightOrbitUp@1', 'hole:orbitLock']);
    for (const id of ['flIdol', 'flSun', 'flLeftRamp', 'flRightRamp', 'flJungle']) assert.ok(layout.flashers.some(f => f.id === id), id);
    for (const id of ['sun', 'idolScoop', 'idolDoors', 'jewelL', 'jewelR', 'orbitLock', 'spinnerL', 'jungle1', 'jungle2', 'jungle3', 'laneT1', 'laneT2', 'laneT3', 'kickbackL'])
        assert.ok(layout.devices.some(d => d.id === id), id);
    assert.deepEqual(layout.devices.find(d => d.id === 'sun').targets.map(t => t.id), ['sunS', 'sunU', 'sunN', 'sunStar']);
    const lock = layout.devices.find(d => d.id === 'orbitLock');
    assert.ok(lock.kind === 'kickhole' && lock.capacity === 3 && lock.hidden && lock.enabled === false);
    assert.equal(layout.paths.find(p => p.id === 'leftRamp').kind, 'plastic');
    assert.equal(layout.paths.find(p => p.id === 'rightRamp').kind, 'wire');
    for (const id of ['doorsOpen', 'idolEyeL', 'idolEyeR', 'lockLit', 'lock1', 'lock2', 'lock3', 'modeStart', 'wizard', 'superJackpot', 'extraBall', 'mode_expedition', 'mode_serpent', 'mode_riddle', 'mode_eclipse', 'sunS', 'sunU', 'sunN', 'sunStar', 'jewelL', 'jewelR', 'spinnerLit'])
        assert.ok(layout.lamps.some(l => l.id === id), id);
    assert.equal(layout.lamps.find(l => l.id === 'idolEyeL').mount, 'toy');
    assert.equal(layout.flippers.length, 3);
    assert.ok(layout.lamps.length >= 70 && layout.lamps.length <= 100, 'lamp count ' + layout.lamps.length);
    assert.ok(layout.flashers.length <= 6 && layout.toys.length >= 5);
});

// A ball shot up the middle through the dropped SUN bank (it is lowered with the dropBank command).
const shootIdol = open => {
    const w = fresh();
    w.command({ type: 'dropBank', id: 'sun' });
    if (open) w.command({ type: 'door', id: 'idolDoors', open: true });
    for (let t = 0; t < 480; t++) w.step({}); // the door swings open in a fraction of a second
    w.placeBall(1, 0, 0.56, 0, 2.2);
    return { w, ...play(w, 2) };
};

test('the idol doors block the scoop until they open', () => {
    const closed = shootIdol(false);
    assert.ok(!has(closed.events, 'hole', 'idolScoop'), 'closed doors: no capture');
    assert.ok(closed.ball.y < 0.66, 'the ball stayed in front of the door (y ' + closed.ball.y.toFixed(3) + ')');
    const open = shootIdol(true);
    assert.ok(has(open.events, 'hole', 'idolScoop'), 'open doors: captured in the scoop');
    assert.equal(open.ball.holder, 'idolScoop');
});

test('a ball waiting on the closed door drops into the scoop, and the scoop exit cannot trap it', () => {
    const w = fresh();
    w.placeBall(1, 0, 0.675, 0, 0);
    let r = play(w, 1);
    assert.ok(has(r.events, 'hole', 'idolScoop'), 'a ball resting on the closed door is captured');
    w.command({ type: 'eject', id: 'idolScoop', count: 1, delay: 0 });
    r = play(w, 0.6);
    assert.ok(has(r.events, 'eject', 'idolScoop') && has(r.events, 'pathMade', 'idolExit'), 'kicked out through the exit tube');
    assert.ok(r.ball.mode === 'field' && r.ball.y < 0.58, 'back on the field below the SUN bank (y ' + r.ball.y.toFixed(3) + ')');
});

test('the orbit lock captures only while enabled, and ejects on command', () => {
    let w = fresh();
    w.placeBall(1, 0.2055, 0.62, 0, 3.2);
    let r = play(w, 1.5);
    assert.ok(has(r.events, 'trigger', 'rightOrbitUp') && !has(r.events, 'hole', 'orbitLock'), 'disabled: the ball rides past');
    w = fresh();
    w.command({ type: 'holeEnable', id: 'orbitLock', on: true });
    w.placeBall(1, 0.2055, 0.62, 0, 3.2);
    r = play(w, 1.5);
    assert.ok(has(r.events, 'hole', 'orbitLock') && w.ctx.balls.get(1).holder === 'orbitLock', 'enabled: captured');
    assert.equal(w.ctx.balls.get(1).hidden, true);
    w.command({ type: 'eject', id: 'orbitLock', count: 1, delay: 0 });
    r = play(w, 2);
    assert.ok(has(r.events, 'eject', 'orbitLock') && r.ball.mode === 'field' && r.ball.y < 0.74, 'ejected down the lane');
});

test('the right orbit exit feeds the upper flipper, which throws the ball back up', () => {
    const f = layout.flippers.find(x => x.id === 'upperRight'), rest = FLIPPER.rest;
    const tip = [f.x + f.side * FLIPPER.length * Math.cos(rest), f.y + FLIPPER.length * Math.sin(rest)];
    const w = fresh();
    w.placeBall(1, 0.2055, 0.80, 0, -0.8);
    let touched = false, hit = -1, maxAfter = 0;
    play(w, 4, t => ({ right: hit >= 0 && t >= hit }), (b, t) => {
        const d = distPointSeg(b.x, b.y, f.x, f.y, tip[0], tip[1]);
        if (hit < 0 && d < BALL_R + 0.012 && b.y < 0.56) { touched = true; hit = t + 60; }
        if (hit >= 0 && t > hit) maxAfter = Math.max(maxAfter, b.y);
    });
    assert.ok(touched, 'the ball rolled out of the orbit onto the upper flipper');
    assert.ok(maxAfter > 0.56, 'flipped back up through the orbit mouth (y ' + maxAfter.toFixed(3) + ')');
});

test('leftRamp returns to the right inlane, rightRamp to the left inlane', () => {
    let w = fresh();
    w.placeBall(1, -0.156, 0.60, 0, 3.5);
    let r = play(w, 3.5);
    assert.ok(has(r.events, 'pathMade', 'leftRamp') && has(r.events, 'pathMade', 'leftReturn') && has(r.events, 'rollover', 'inlaneR'), 'stone ramp');
    w = fresh();
    w.placeBall(1, 0.128, 0.60, 0, 3.6);
    r = play(w, 3.5);
    assert.ok(has(r.events, 'pathMade', 'rightRamp') && has(r.events, 'pathMade', 'rightReturn') && has(r.events, 'rollover', 'inlaneL'), 'gold ramp');
});

test('the SUN bank is tilted so nothing rests on top of it, and the jungle pops fire', () => {
    const w = fresh();
    w.placeBall(1, -0.040, 0.640, 0, 0);
    let rest = 0;
    play(w, 6, {}, b => { if (b.y > 0.58 && b.y < 0.65 && Math.hypot(b.vx, b.vy) < 0.02) rest++; });
    assert.ok(rest < 480, 'a ball dropped on the SUN row keeps moving (still for ' + (rest * STEP).toFixed(2) + ' s)');
    const b = fresh();
    b.placeBall(1, 0.06, 0.78, 0, 1.2);
    assert.ok(has(play(b, 3).events, 'bumper', 'jungle2'), 'a ball shot up the right side reaches a jungle pop');
});

console.log('layout-temple ok (' + passed + ' tests)');
