// Clockwork Observatory layout tests (plan S5). Run: node scripts/test-pinball-layout-observatory.mjs
import assert from 'node:assert/strict';
import { TABLES } from '../ui/js/desktop/apps/pinball/tables/index.js';
import { validateTable } from '../ui/js/desktop/apps/pinball/tables/validate.js';
import { createWorld } from '../ui/js/desktop/apps/pinball/physics/world.js';
import { STEP } from '../ui/js/desktop/apps/pinball/physics/constants.js';
import { SLING } from '../ui/js/desktop/apps/pinball/tables/observatory/rules.js';
import { sweep, soak, crossShots } from './lib/pinball-shots.mjs';

const layout = TABLES.observatory.layout;
const dev = id => layout.devices.find(d => d.id === id);
const disc = dev('orrery'), bob = dev('pendulum');
let passed = 0;
function test(name, fn) { fn(); passed++; console.log('ok   ' + name); }
const fresh = (seed = 3) => createWorld(layout, { seed });
const has = (events, type, id) => events.some(e => e.type === type && (id === undefined || e.id === id));

// Steps `seconds` with no flipper input, collecting events; `each(world, tick)` runs after every step.
function play(world, seconds, each) {
    const events = [];
    for (let t = 0; t < Math.round(seconds / STEP); t++) {
        world.step({});
        for (const ev of world.events()) events.push({ ...ev, t: t * STEP });
        if (each) each(world, t);
    }
    return events;
}
// Parks the pendulum and waits until it has been lifted out of the way.
const parked = seed => { const w = fresh(seed); w.command({ type: 'pendulum', id: 'pendulum', mode: 'park' }); play(w, 1.5); return w; };
const turn = (b, cx, cy) => (b.x - cx) * b.vy - (b.y - cy) * b.vx; // + = counter-clockwise about the disc centre

test('the observatory layout validates and keeps the agreed ids', () => {
    assert.deepEqual(validateTable(TABLES.observatory).errors, []);
    assert.deepEqual(layout.shots.map(s => s.id), ['leftOrbit', 'comet', 'orrery', 'telescope', 'meridian', 'rightOrbit']);
    assert.deepEqual(layout.shots.find(s => s.id === 'telescope').setup, [{ type: 'pendulum', id: 'pendulum', mode: 'park' }]);
    assert.deepEqual(layout.flippers.map(f => f.id), ['left', 'right']);
    assert.equal(layout.flashers.length, 6);
    assert.ok(layout.lamps.length >= 70 && layout.lamps.length <= 110, 'lamp count ' + layout.lamps.length);
    assert.equal(disc.type, 'orrery'); assert.equal(bob.type, 'pendulum');
    for (const id of ['telescope', 'wellMars', 'wellSaturn', 'orreryGate', 'spinnerR', 'popA', 'popB', 'kickbackL', 'starS', 'starT', 'starA', 'starR', 'laneT1', 'laneT2', 'laneT3']) assert.ok(dev(id), id);
    assert.ok(layout.posts.some(p => p.id === 'orreryHub' && p.x === disc.x && p.y === disc.y), 'hub post in the middle of the disc');
    const tube = layout.paths.find(p => p.id === 'telescopeTube');
    assert.equal(tube.top, 0.037); assert.equal(tube.skin, 'telescope');
    const [ex, ey] = tube.points[tube.points.length - 1], d = Math.hypot(ex - disc.x, ey - disc.y);
    assert.ok(d >= 0.042 && d <= disc.r, 'the tube lands on the disc rim, outside the reach of the arms: ' + d.toFixed(4));
    assert.deepEqual(layout.toys.map(t => t.kind), ['orrery', 'telescope', 'gears', 'dome', 'clockDial']);
    for (const id of ['gearTrain', 'dome']) assert.equal(layout.toys.find(t => t.id === id).h, 0.043);
    const lampIds = new Set(layout.lamps.map(l => l.id));
    for (const id of ['mode_meteor', 'mode_transit', 'mode_conjunction', 'mode_supernova', 'wellMarsLit', 'wellSaturnLit', 'pendulumLit', 'spinnerLit', 'wizard', 'superJackpot', 'extraBall', 'modeStart', ...[1, 2, 12].map(n => 'clock' + n)]) assert.ok(lampIds.has(id), 'lamp ' + id);
    assert.deepEqual(layout.gi.map(g => [g.id, g.color]), [['lower', '#ffd9a0'], ['upper', '#9fbfff']]);
});

test('comet at 3.2 m/s reaches the right inlane through cometReturn; meridian at 3.4 m/s reaches the left inlane', () => {
    let w = fresh();
    w.placeBall(1, -0.156, 0.60, 0, 3.2);
    let ev = play(w, 4.5);
    assert.ok(has(ev, 'pathMade', 'cometRamp') && has(ev, 'pathMade', 'cometReturn'), 'comet made through its return');
    assert.ok(has(ev, 'rollover', 'inlaneR') && !has(ev, 'rollover', 'inlaneL'));
    w = fresh();
    w.placeBall(1, 0.128, 0.60, 0, 3.4);
    ev = play(w, 4.5);
    assert.ok(has(ev, 'pathMade', 'meridianRamp'), 'meridian made');
    assert.ok(has(ev, 'rollover', 'inlaneL') && !has(ev, 'rollover', 'inlaneR'));
});

test('a slow ball up the channel is captured by the telescope with the pendulum parked; the eject runs the tube and lands on the disc', () => {
    for (const speed of [1.5, 2.2, 3.0]) {
        const w = parked(3);
        w.placeBall(1, 0.045, 0.45, 0, speed);
        assert.ok(has(play(w, 3), 'hole', 'telescope'), 'captured at ' + speed + ' m/s');
        w.command({ type: 'eject', id: 'telescope' });
        const ev = play(w, 2);
        const made = ev.find(e => e.type === 'pathMade' && e.id === 'telescopeTube'), on = ev.find(e => e.type === 'orrery');
        assert.ok(made, 'the ball made the tube');
        assert.ok(on && on.t < 2, 'an orrery event within 2 s');
    }
});

test('the gravity slingshot: a well holds a ball and the throw is captured by the telescope within 2 s', () => {
    for (const id of ['wellMars', 'wellSaturn']) for (const da of [0, 0.03, -0.03]) {
        const well = dev(id), w = parked(3);
        w.placeBall(1, well.x - 0.012, well.y - 0.025, 0.1, 0.35);
        w.command({ type: 'magnet', id, mode: 'grab' });
        let held = false;
        for (let t = 0; t < Math.round(4 / STEP) && !held; t++) { w.step({}); for (const e of w.events()) if (e.type === 'magnet' && e.state === 'held') held = true; }
        assert.ok(held, id + ' grabbed the ball');
        play(w, 0.8);
        w.command({ type: 'magnet', id, mode: 'throw', angle: Math.atan2(SLING.at[1] - well.y, SLING.at[0] - well.x) + da, speed: SLING.speed });
        assert.ok(has(play(w, 2), 'hole', 'telescope'), id + ' sling captured by the telescope (angle error ' + da + ')');
    }
});

test('a swinging pendulum strikes balls for some timings and never traps one near the swing plane', () => {
    let struck = 0, captured = 0;
    for (let k = 0; k < 18; k++) {
        const w = fresh(5);
        play(w, k * 0.1);
        w.placeBall(1, 0.045, 0.47, 0, 2.2);
        const ev = play(w, 6);
        if (has(ev, 'pendulum')) struck++;
        if (has(ev, 'hole', 'telescope')) captured++;
    }
    assert.ok(struck >= 3, 'the bob struck the ball for ' + struck + ' of 18 timings');
    assert.ok(captured >= 1, 'some timings still reach the telescope: ' + captured);
    // A ball put down anywhere round the swing plane leaves it again (bug class: a ball pinned between the bob and a wall end or a target).
    for (let x = 0.005; x <= 0.095; x += 0.015) for (let y = 0.535; y <= 0.605; y += 0.01) {
        const w = fresh(9);
        w.placeBall(1, x, y, 0, 0);
        let rest = 0, longest = 0, last = null;
        play(w, 8, world => {
            const b = world.ctx.balls.get(1);
            if (!b || b.mode !== 'field') { rest = 0; return; }
            const still = last && Math.hypot(b.x - last[0], b.y - last[1]) < 0.0008;
            rest = still ? rest + STEP : 0; longest = Math.max(longest, rest); last = [b.x, b.y];
        });
        assert.ok(longest < 3, 'a ball at (' + x.toFixed(3) + ', ' + y.toFixed(3) + ') sat still for ' + longest.toFixed(1) + ' s');
    }
});

test('a ball on the disc picks up its direction and a negative speed command reverses it', () => {
    const spin = speed => {
        const w = fresh();
        if (speed !== undefined) w.command({ type: 'orrery', id: 'orrery', speed });
        play(w, 0.5);
        w.placeBall(1, disc.x + 0.03, disc.y, 0, 0);
        let sum = 0;
        play(w, 0.12, world => { const b = world.ctx.balls.get(1); if (b) sum += turn(b, disc.x, disc.y); });
        return sum;
    };
    assert.ok(spin() > 0.001, 'idle speed is counter-clockwise');
    assert.ok(spin(-8) < -0.001, 'a -8 rad/s command reverses it');
    assert.ok(disc.maxSpeed >= 24, 'the wizard speed fits under maxSpeed');
});

test('the disc visibly grabs a ball: it leaves with a clearly changed heading and never orbits for long', () => {
    for (const speed of [disc.speed, -disc.speed, 24, -24]) {
        const lay = { ...layout, devices: layout.devices.map(d => d.id === 'orrery' ? { ...d, speed } : d) };
        let total = 0, n = 0;
        for (const dx of [-0.03, -0.02, -0.01, 0, 0.01, 0.02, 0.03]) {
            const w = createWorld(lay, { seed: 3 });
            w.placeBall(1, disc.x + dx, disc.y - disc.r - 0.03, 0, 1.0);
            let t0 = -1, a0 = 0, a1 = null;
            for (let t = 0; t < Math.round(4 / STEP) && a1 === null; t++) {
                w.step({}); w.events();
                const b = w.ctx.balls.get(1);
                if (!b) break;
                const d = Math.hypot(b.x - disc.x, b.y - disc.y);
                if (t0 < 0 && d < disc.r) { t0 = t; a0 = Math.atan2(b.vy, b.vx); }
                if (t0 >= 0 && d > disc.r + 0.004) a1 = Math.atan2(b.vy, b.vx);
                if (t0 >= 0 && (t - t0) * STEP > 3) assert.fail('a ball orbited the disc for more than 3 s at ' + speed + ' rad/s (offset ' + dx + ')');
            }
            assert.ok(a1 !== null, 'the ball left the disc at ' + speed + ' rad/s (offset ' + dx + ')');
            let da = Math.abs(a1 - a0) * 180 / Math.PI;
            if (da > 180) da = 360 - da;
            total += da; n++;
        }
        if (speed === disc.speed) assert.ok(total / n >= 25, 'mean deviation at idle speed ' + (total / n).toFixed(1) + ' deg');
        // A ball dropped at rest anywhere on the disc is thrown off it within 3 s.
        for (let a = 0; a < 360; a += 60) for (const r of [0.012, 0.03, 0.044]) {
            const w = createWorld(lay, { seed: 3 });
            w.placeBall(1, disc.x + r * Math.cos(a * Math.PI / 180), disc.y + r * Math.sin(a * Math.PI / 180), 0, 0);
            let left = false;
            for (let t = 0; t < Math.round(3 / STEP) && !left; t++) {
                w.step({}); w.events();
                const b = w.ctx.balls.get(1);
                left = !b || Math.hypot(b.x - disc.x, b.y - disc.y) > disc.r + 0.004;
            }
            assert.ok(left, 'a resting ball stayed on the disc for 3 s at ' + speed + ' rad/s (' + a + ' deg, r ' + r + ')');
        }
    }
});

test('every shot is reachable; each lower flipper reaches at least four shots and a cross shot', () => {
    const r = sweep(layout);
    assert.deepEqual(r.missing, []);
    for (const fid of ['left', 'right']) {
        assert.ok(r.flippers[fid].length >= 4, fid + ' reaches ' + r.flippers[fid].join(','));
        assert.ok(crossShots(layout, fid, r.flippers[fid]).length >= 1, fid + ' has a cross shot');
    }
    console.log('       flippers: ' + Object.entries(r.flippers).map(([k, v]) => k + '=' + v.join('/')).join('  '));
});

test('a 120 s soak with four balls rests nowhere for 8 s and touches at least 30 switches', () => {
    for (const seed of [1, 2, 3]) {
        const s = soak(layout, { seed, seconds: 120 });
        assert.ok(s.ok, 'seed ' + seed + ': ' + s.problems.slice(0, 3).join('; '));
        assert.equal(s.searchFail, 0);
    }
});

console.log('layout-observatory ok (' + passed + ' tests)');
