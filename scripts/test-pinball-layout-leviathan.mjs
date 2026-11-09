// Deep Sea Leviathan layout tests (plan T20). Run: node scripts/test-pinball-layout-leviathan.mjs
import assert from 'node:assert/strict';
import { TABLES } from '../ui/js/desktop/apps/pinball/tables/index.js';
import { validateTable } from '../ui/js/desktop/apps/pinball/tables/validate.js';
import { createWorld } from '../ui/js/desktop/apps/pinball/physics/world.js';
import { STEP, FLIPPER } from '../ui/js/desktop/apps/pinball/physics/constants.js';
import { matchEvent } from '../ui/js/desktop/apps/pinball/contracts.js';
import { sweep, soak, crossShots } from './lib/pinball-shots.mjs';

const layout = TABLES.leviathan.layout;
const dev = id => layout.devices.find(d => d.id === id);
let passed = 0;
function test(name, fn) { fn(); passed++; console.log('ok   ' + name); }
const fresh = () => createWorld(layout, { seed: 3 });
const has = (events, type, id) => events.some(e => e.type === type && (id === undefined || e.id === id));

// Steps `seconds` with no flipper input (or `input(tick)`), collecting events; `each(world, tick)` runs after every step.
function play(world, seconds, each, input) {
    const events = [];
    for (let t = 0; t < Math.round(seconds / STEP); t++) {
        world.step(input ? input(t) : {});
        for (const ev of world.events()) events.push(ev);
        if (each) each(world, t);
    }
    return events;
}

test('the leviathan layout validates and keeps the agreed ids', () => {
    assert.deepEqual(validateTable(TABLES.leviathan).errors, []);
    assert.deepEqual(layout.shots.map(s => s.id), ['leftOrbit', 'current', 'pearl', 'ship', 'tentacle', 'rightOrbit']);
    for (const id of ['flShip', 'flPearl', 'flTentacle', 'flCurrent', 'flCoral']) assert.ok(layout.flashers.some(f => f.id === id), id);
    assert.deepEqual(layout.flippers.map(f => f.id), ['left', 'right', 'upperLeft']);
    assert.equal(layout.flippers[2].key, 'left');
    for (const id of ['shipScoop', 'abyss', 'pearl', 'sonar1', 'sonar2', 'sonar3', 'sonar4', 'sonar5', 'spinnerR', 'coral1', 'coral2', 'coral3', 'laneT1', 'laneT2', 'laneT3', 'kickbackR']) assert.ok(dev(id), id);
    assert.equal(dev('pearl').endSwitch, 'pearlTop');
    assert.equal(dev('abyss').type, 'magnet');
    const paths = Object.fromEntries(layout.paths.map(p => [p.id, p]));
    assert.equal(paths.currentRamp.kind, 'wire'); assert.equal(paths.tentacleRamp.kind, 'plastic');
    assert.ok(paths.tentacleRamp.switches.some(s => s.id === 'tentacleTop' && s.at === 0.5));
    const lampIds = new Set(layout.lamps.map(l => l.id));
    const need = ['shootAgain', 'bonus2x', 'bonus3x', 'bonus4x', 'bonus5x', 'kickbackR', 'extraBall', 'lockLit', 'lock1', 'lock2', 'lock3', 'modeStart', 'wizard', 'superJackpot', 'grab', 'spinnerLit', 'eyeL', 'eyeR',
        ...['tsunami', 'sonar', 'treasure', 'kraken'].map(m => 'mode_' + m), ...[1, 2, 3, 4, 5].map(n => 'sonar' + n), ...[1, 2, 3].map(n => 'pearl' + n), ...[1, 2, 3].map(n => 'laneT' + n),
        ...layout.shots.flatMap(s => ['arrow_', 'jp_', 'combo_'].map(p => p + s.id))];
    for (const id of need) assert.ok(lampIds.has(id), 'lamp ' + id);
    assert.ok(layout.lamps.length >= 70 && layout.lamps.length <= 100, 'lamp count ' + layout.lamps.length);
    for (const id of ['eyeL', 'eyeR']) assert.equal(layout.lamps.find(l => l.id === id).mount, 'toy');
    assert.deepEqual(layout.gi.map(g => [g.id, g.color]), [['lower', '#bff6ff'], ['upper', '#7fd3ff']]);
    assert.ok(layout.gi.every(g => g.bulbs.length >= 8));
    for (const kind of ['leviathan', 'ship', 'kelp']) assert.ok(layout.toys.some(t => t.kind === kind), 'toy ' + kind);
    assert.ok(layout.meta.attract.length >= 4);
});

test('a pearl strike fires the captive event and a strong hit sends the pearl to pearlTop, then it returns', () => {
    const [px, py] = [dev('pearl').channel.a[0], dev('pearl').channel.a[1]];
    for (const speed of [2.5, 5]) {
        const w = fresh();
        w.placeBall(1, px, py - 0.08, 0, speed);
        let inside = false;
        const events = play(w, 5, world => {
            const s = world.ctx.balls.get(1);
            if (s && Math.abs(s.x - px) < 0.015 && s.y > py + 0.01 && s.y < dev('pearl').channel.b[1]) inside = true;
        });
        const strike = events.find(e => e.type === 'captive' && e.id === 'pearl');
        assert.ok(strike && strike.ball === 1, 'captive event carries the striking ball at ' + speed);
        assert.ok(has(events, 'trigger', 'pearlTop'), 'pearl reached pearlTop at ' + speed);
        assert.ok(!inside, 'the striker never enters the channel');
        const ball = w.snapshot().balls.find(b => b.id === 'pearl_ball');
        assert.ok(Math.abs(ball.y - 0.6102) < 0.002 && Math.abs(ball.x - px) < 1e-6, 'the pearl came back to its mouth: ' + ball.y);
    }
});

test('Deep Grab: the magnet holds a ball and the throw toward the ship is captured by the scoop within 2 s', () => {
    const mg = dev('abyss'), sh = dev('shipScoop'), base = Math.atan2(sh.y - mg.y, sh.x - mg.x);
    for (const [da, speed] of [[0, 1.4], [0.05, 1.4], [-0.05, 1.4], [0, 1.8]]) {
        const w = fresh();
        w.placeBall(1, mg.x - 0.015, mg.y - 0.04, 0.3, 1.0);
        w.command({ type: 'magnet', id: 'abyss', mode: 'grab' });
        let heldAt = -1;
        const events = [];
        for (let t = 0; t < 4 / STEP && heldAt < 0; t++) {
            w.step({});
            for (const ev of w.events()) { events.push(ev); if (ev.type === 'magnet' && ev.state === 'held') heldAt = t; }
        }
        assert.ok(heldAt >= 0, 'magnet grabbed the ball');
        play(w, 1.5);
        w.command({ type: 'magnet', id: 'abyss', mode: 'throw', angle: base + da, speed });
        const after = play(w, 2);
        assert.ok(has(after, 'magnet', 'abyss') && after.find(e => e.type === 'magnet').state === 'throw', 'throw event');
        assert.ok(has(after, 'hole', 'shipScoop'), 'scoop captured the thrown ball (angle ' + da + ', ' + speed + ' m/s)');
    }
});

test('tentacleRamp over the helix feeds the left inlane; currentRamp feeds the right inlane', () => {
    let w = fresh();
    w.placeBall(1, 0.128, 0.60, 0, 3.5);
    let ev = play(w, 4);
    assert.ok(has(ev, 'pathMade', 'tentacleRamp') && has(ev, 'pathSwitch', 'tentacleTop'), 'tentacle made through its top switch');
    assert.ok(has(ev, 'rollover', 'inlaneL'), 'left inlane');
    assert.ok(!has(ev, 'rollover', 'inlaneR'));
    w = fresh();
    w.placeBall(1, -0.156, 0.60, 0, 3.5);
    ev = play(w, 4);
    assert.ok(has(ev, 'pathMade', 'currentRamp'), 'current made');
    assert.ok(has(ev, 'rollover', 'inlaneR'), 'right inlane');
    assert.ok(!has(ev, 'rollover', 'inlaneL'));
});

test('ramps are makeable with a good shot and fail with a weak one', () => {
    for (const [id, x, y, ramp] of [['tentacle', 0.128, 0.62, 'tentacleRamp'], ['current', -0.156, 0.60, 'currentRamp']]) {
        let w = fresh();
        w.placeBall(1, x, y, 0, 3.0);
        assert.ok(has(play(w, 4), 'pathMade', ramp), id + ' made at 3 m/s');
        w = fresh();
        w.placeBall(1, x, y, 0, 1.2);
        const weak = play(w, 4);
        assert.ok(has(weak, 'pathFail', ramp) && !has(weak, 'pathMade', ramp), id + ' fails at 1.2 m/s and rolls back');
    }
});

test('balls cannot perch: the sonar bank, the pearl roof, the head base and the upper ceiling shed a ball', () => {
    const roofApex = [dev('pearl').channel.b[0], dev('pearl').channel.b[1] + 0.028];
    const spots = [];
    for (const dx of [-0.02, -0.012, -0.004, 0.004, 0.012, 0.02]) spots.push(['pearl roof', roofApex[0] + dx, roofApex[1] + 0.02, 0.35]);
    for (const id of ['sonar1', 'sonar2', 'sonar3']) for (const dx of [-0.01, 0, 0.01]) spots.push([id, dev(id).x + dx, dev(id).y + 0.02, 0.58]);
    for (const dx of [-0.03, -0.003, 0.003, 0.03]) spots.push(['head base', 0.105 + dx, 0.925, 0.5]);
    for (const dx of [-0.02, 0.01, 0.03]) spots.push(['ceiling', -0.145 + dx, 0.88, 0.3]);
    for (const [what, x, y, below] of spots) {
        const w = fresh();
        w.placeBall(1, x, y, 0, 0);
        play(w, 10);
        const b = w.ctx.balls.get(1);
        if (!b || b.mode !== 'field') continue; // drained or held by a device: not perched
        assert.ok(b.y < below, what + ' held a ball at (' + x.toFixed(3) + ', ' + y.toFixed(3) + ') -> (' + b.x.toFixed(3) + ', ' + b.y.toFixed(3) + ')');
    }
});

test('every shot is reachable by a lower flipper and the upper left flipper reaches counted shots too', () => {
    const r = sweep(layout);
    assert.deepEqual(r.missing, []);
    for (const fid of ['left', 'right']) {
        assert.ok(r.flippers[fid].length >= 4, fid + ' reaches ' + r.flippers[fid].join(','));
        assert.ok(crossShots(layout, fid, r.flippers[fid]).length >= 1, fid + ' has a cross shot');
    }
    // The upper flipper: a ball dropped onto the resting bat, flipped at every delay (the generic sweep stops at delay 120, a drop takes ~160 ticks to land).
    const f = layout.flippers.find(x => x.id === 'upperLeft'), mx = f.x + f.length / 2 * Math.cos(FLIPPER.rest), my = f.y + f.length / 2 * Math.sin(FLIPPER.rest);
    const reached = new Map();
    for (const [dx, dy] of [[0, 0.06], [0, 0.04], [0.02, 0.06], [0.015, 0.03]]) for (let delay = 0; delay <= 300; delay += 2) {
        const w = fresh();
        w.placeBall(1, mx + dx, my + dy, 0, 0);
        for (let t = 0; t < Math.round(3 / STEP) + delay; t++) {
            w.step({ left: t >= delay });
            let drained = false;
            for (const ev of w.events()) {
                if (ev.type === 'drain') drained = true;
                for (const s of layout.shots) if (matchEvent(s.on, ev)) reached.set(s.id, (reached.get(s.id) || 0) + 1);
            }
            if (drained) break;
        }
    }
    assert.ok(reached.size >= 2 && reached.has('ship'), 'upper left flipper reaches ' + [...reached].map(([k, v]) => k + 'x' + v).join(', '));
    console.log('       flippers: ' + Object.entries(r.flippers).filter(([k]) => k !== 'upperLeft').map(([k, v]) => k + '=' + v.join('/')).join('  ') + '  upperLeft=' + [...reached.keys()].join('/'));
});

test('a 120 s soak with four balls rests nowhere for 8 s and touches at least 30 switches', () => {
    for (const seed of [1, 2, 3]) {
        const s = soak(layout, { seed, seconds: 120 });
        assert.ok(s.ok, 'seed ' + seed + ': ' + s.problems.slice(0, 3).join('; '));
        assert.equal(s.searchFail, 0);
    }
});

console.log('layout-leviathan ok (' + passed + ' tests)');
