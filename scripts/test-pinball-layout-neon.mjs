// Cyber Neon Metropolis layout tests (plan T14). Run: node scripts/test-pinball-layout-neon.mjs
import assert from 'node:assert/strict';
import { TABLES } from '../ui/js/desktop/apps/pinball/tables/index.js';
import { validateTable } from '../ui/js/desktop/apps/pinball/tables/validate.js';
import { createWorld } from '../ui/js/desktop/apps/pinball/physics/world.js';
import { STEP } from '../ui/js/desktop/apps/pinball/physics/constants.js';

const layout = TABLES.neon.layout;
let passed = 0;
function test(name, fn) { fn(); passed++; console.log('ok   ' + name); }

// Steps `seconds` with no flipper input; returns the events, the lowest y after a path exit and the x where the ball first got below y 0.25.
function play(world, seconds, each) {
    const events = [];
    let minY = Infinity, lowX = null;
    for (let t = 0; t < Math.round(seconds / STEP); t++) {
        world.step({});
        for (const ev of world.events()) events.push(ev);
        const b = world.ctx.balls.get(1);
        if (b && each) each(b, events);
        if (b && b.mode === 'field' && b.y < minY && events.some(e => e.type === 'pathMade')) { minY = b.y; if (b.y < 0.25 && lowX === null) lowX = b.x; }
    }
    return { events, minY, lowX, ball: world.ctx.balls.get(1) };
}
const fresh = () => createWorld(layout, { seed: 3 });
const has = (events, type, id) => events.some(e => e.type === type && (id === undefined || e.id === id));

test('the neon layout validates and keeps the agreed ids', () => {
    assert.deepEqual(validateTable(TABLES.neon).errors, []);
    assert.deepEqual(layout.shots.map(s => s.id), ['leftOrbit', 'leftRamp', 'vault', 'core', 'rightRamp', 'rightOrbit']);
    for (const id of ['flCore', 'flVault', 'flLeftRamp', 'flRightRamp', 'flPops']) assert.ok(layout.flashers.some(f => f.id === id), id);
    assert.ok(layout.lamps.length >= 70 && layout.lamps.length <= 100, 'lamp count ' + layout.lamps.length);
    assert.equal(layout.flippers.length, 3);
});

test('leftRamp at 3.5 m/s reaches the right flipper area within 2 s', () => {
    const w = fresh();
    w.placeBall(1, -0.156, 0.60, 0, 3.5);
    const r = play(w, 2);
    assert.ok(has(r.events, 'pathMade', 'leftRamp'), 'ramp made');
    assert.ok(has(r.events, 'pathMade', 'leftReturn'));
    assert.ok(r.minY < 0.25 && r.lowX > layout.meta.cx, 'ball crossed y 0.25 at x ' + r.lowX + ' (lowest y ' + r.minY.toFixed(3) + ')');
    assert.ok(has(r.events, 'rollover', 'inlaneR'), 'through the right inlane');
});

test('rightRamp: diverter off feeds the left inlane, on feeds the vault', () => {
    let w = fresh();
    w.placeBall(1, 0.128, 0.60, 0, 3.6);
    let r = play(w, 3);
    assert.ok(has(r.events, 'pathMade', 'rightRamp') && has(r.events, 'pathMade', 'rightReturn'));
    assert.ok(has(r.events, 'rollover', 'inlaneL'), 'left inlane');
    assert.ok(!has(r.events, 'hole', 'vault'));
    w = fresh();
    w.command({ type: 'diverter', id: 'lockDiverter', on: true });
    w.placeBall(1, 0.128, 0.60, 0, 3.6);
    r = play(w, 3);
    assert.ok(has(r.events, 'pathMade', 'vaultFeed'));
    const hole = r.events.find(e => e.type === 'hole' && e.id === 'vault');
    assert.ok(hole && hole.via === 'path', 'ball locked in the vault by the subway');
    assert.ok(!has(r.events, 'rollover', 'inlaneL'));
});

test('the Data Core tube ends in the pop bumpers within 2 s', () => {
    const w = fresh();
    w.placeBall(1, 0, 0.628, 0, 0.6);
    let ticks = 0;
    for (; ticks < 960 && !w.ctx.balls.get(1).holder; ticks++) w.step({});
    assert.equal(w.ctx.balls.get(1).holder, 'dataCore', 'captured by the VUK');
    w.events();
    w.command({ type: 'eject', id: 'dataCore', count: 1, delay: 0 });
    const r = play(w, 2);
    assert.ok(has(r.events, 'eject', 'dataCore') && has(r.events, 'pathMade', 'coreTube'));
    assert.ok(r.events.some(e => e.type === 'bumper'), 'bumper event');
});

test('the vault captures a ball through the dropped bank, and not through the raised one', () => {
    const n = [0.423, -0.906], base = [-0.104 + n[0] * 0.09, 0.711 + n[1] * 0.09];
    let w = fresh();
    w.command({ type: 'dropBank', id: 'firewall' });
    w.placeBall(1, base[0], base[1], -n[0] * 1.8, -n[1] * 1.8);
    let r = play(w, 2);
    assert.ok(has(r.events, 'hole', 'vault'), 'locked behind the dropped bank');
    w = fresh();
    w.placeBall(1, base[0], base[1], -n[0] * 1.8, -n[1] * 1.8);
    r = play(w, 1.2);
    assert.ok(!has(r.events, 'hole', 'vault') && has(r.events, 'drop'), 'bank hit instead');
});

console.log('layout-neon ok (' + passed + ' tests)');
