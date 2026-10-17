import assert from 'node:assert/strict';
import { createPhysics } from '../ui/js/desktop/apps/minigolf-physics.js';

const bounds = { min: [-20, -5, -20], max: [20, 10, 20] };
const fixedStep = 1 / 120;
const floor = { type: 'box', position: [0, -0.1, 0], halfExtents: [10, 0.1, 10], surface: 'grass' };
const course = (extra = {}) => ({ id: 'test', number: 1, world: 'test', par: 3, model: '', tee: [0, 0.16, 0], cup: [15, 0, 0], cupRadius: 0.32, bounds, colliders: [floor], obstacles: [], water: [], ...extra });
const close = (a, b, tolerance, message) => assert.ok(Math.abs(a - b) <= tolerance, `${message}: ${a} vs ${b}`);

async function travelledAt(fps) {
    const physics = await createPhysics(course());
    assert.equal(physics.shoot([1, 0], 0.5), true);
    let snapshot;
    for (let i = 0; i < fps; i++) snapshot = physics.step(1 / fps);
    const actual = physics.getBall();
    assert.ok(snapshot.position[0] <= actual[0] + 1e-9, `${fps} Hz render pose does not lead the authoritative pose`);
    assert.ok(actual[0] - snapshot.position[0] <= 12 * fixedStep + 1e-4, `${fps} Hz render pose stays within one fixed step`);
    physics.dispose();
    return actual[0];
}

const distances = await Promise.all([30, 60, 144].map(travelledAt));
close(distances[0], distances[1], 0.035, '30/60 Hz fixed-step distance');
close(distances[1], distances[2], 0.035, '60/144 Hz fixed-step distance');

const interpolated = await createPhysics(course());
assert.equal(interpolated.shoot([1, 0], 0.5), true);
const renderPositions = [];
let betweenFixedSteps = false;
for (let i = 0; i < 24; i++) {
    const snapshot = interpolated.step(1 / 144);
    const actual = interpolated.getBall()[0];
    renderPositions.push(snapshot.position[0]);
    if (actual - snapshot.position[0] > 0.005) betweenFixedSteps = true;
}
assert.ok(betweenFixedSteps, '144 Hz render samples interpolate between authoritative fixed-step poses');
for (let i = 1; i < renderPositions.length; i++) {
    assert.ok(renderPositions[i] >= renderPositions[i - 1] - 1e-6, 'interpolated positions remain continuous and monotonic');
    assert.ok(renderPositions[i] - renderPositions[i - 1] < 0.055, '144 Hz interpolation avoids fixed-step position jumps');
}
interpolated.dispose();

const wallEvents = [];
const wall = await createPhysics(course({
    tee: [0, 0.16, 0],
    cup: [10, 0, 0],
    colliders: [floor, { type: 'box', position: [1, 0.5, 0], halfExtents: [0.02, 0.5, 2], surface: 'stone' }]
}), { onEvent: event => wallEvents.push(event) });
assert.equal(wall.shoot([1, 0], 1), true);
for (let i = 0; i < 60; i++) wall.step(1 / 60);
assert.ok(wall.getBall()[0] < 0.86, `thin wall blocked CCD ball at x=${wall.getBall()[0]}`);
assert.ok(wallEvents.some(event => event.type === 'impact' && event.surface === 'stone'));
wall.dispose();

const slope = await createPhysics(course({
    tee: [1, 0.04, 0],
    colliders: [{ type: 'box', position: [0, -0.05, 0], halfExtents: [5, 0.05, 5], rotation: [0, 0, -Math.sin(0.06), Math.cos(0.06)], surface: 'grass' }]
}));
assert.equal(slope.shoot([0, 1], 0.01), true);
for (let i = 0; i < 240; i++) slope.step(1 / 120);
assert.ok(slope.getBall()[0] > 1.06, `ball rolled down slope to x=${slope.getBall()[0]}`);
slope.dispose();

const flatLaunch = await createPhysics(course());
assert.equal(flatLaunch.shoot([1, 0], 0.05), true);
const firstLaunchStep = flatLaunch.step(1 / 120);
assert.ok(firstLaunchStep.position[1] <= 0.1601, `flat putt launched without a hop: y=${firstLaunchStep.position[1]}`);
flatLaunch.dispose();

const idle = await createPhysics(course({
    tee: [1, 0.04, 0],
    colliders: [{ type: 'box', position: [0, -0.05, 0], halfExtents: [5, 0.05, 5], rotation: [0, 0, -Math.sin(0.06), Math.cos(0.06)], surface: 'grass' }],
    obstacles: [{ id: 'idle-gate', kind: 'slide', position: [0, 1, 1], halfExtents: [0.1, 0.5, 0.5], axis: [1, 0, 0], amplitude: 1, speed: 2, phase: 0 }]
}));
const idlePosition = idle.getBall();
for (let i = 0; i < 30; i++) idle.step(1 / 60);
assert.deepEqual(idle.getBall(), idlePosition, 'idle ball is held in place on a slope');
assert.equal(idle.getSnapshot().moving, false);
assert.ok(idle.getObstacles()[0].position[0] > 0.7, 'obstacles keep moving while the ball is idle');
idle.dispose();

let terminal;
let rearmed = false;
terminal = await createPhysics(course({
    tee: [0, 0.16, 0],
    cup: [10, 0, 0],
    bounds: { min: [-20, -5, -20], max: [0.35, 10, 20] }
}), { onEvent: event => {
    if (event.type === 'out' && !rearmed) {
        rearmed = true;
        terminal.setBall([0, 0.16, 0]);
        terminal.shoot([0, 1], 0.1);
    }
} });
assert.equal(terminal.shoot([1, 0], 0.5), true);
const terminalFrame = terminal.step(0.1);
assert.equal(rearmed, true, 'out event can synchronously prepare the next shot');
for (let axis = 0; axis < 3; axis++) close(terminalFrame.position[axis], [0, 0.16, 0][axis], 1e-6, 'remaining frame substeps do not move the next ball');
assert.ok(terminalFrame.time < 0.1, `terminal event stopped catch-up substeps at ${terminalFrame.time}s`);
assert.equal(terminalFrame.moving, true, 'synchronous next shot remains armed for the next frame');
terminal.dispose();

const waterEvents = [];
const water = await createPhysics(course({ water: [{ min: [0.3, -1, -1], max: [0.8, 1, 1] }] }), { onEvent: event => waterEvents.push(event) });
assert.equal(water.shoot([1, 0], 0.12), true);
for (let i = 0; i < 120 && !waterEvents.some(event => event.type === 'water'); i++) water.step(1 / 60);
assert.equal(waterEvents.filter(event => event.type === 'water').length, 1);
assert.equal(water.shoot([1, 0], 0.1), false);
assert.equal(water.setBall([0, 0.16, 0]), true);
assert.equal(water.shoot([1, 0], 0.1), true);
water.dispose();

function ringMesh() {
    const vertices = [];
    const indices = [];
    const segments = 32;
    for (let i = 0; i < segments; i++) {
        const a = i * Math.PI * 2 / segments;
        const b = (i + 1) * Math.PI * 2 / segments;
        const point = (radius, angle) => [radius * Math.cos(angle), 0, radius * Math.sin(angle)];
        const base = vertices.length / 3;
        vertices.push(...point(3, a), ...point(0.32, a), ...point(0.32, b), ...point(3, b));
        indices.push(base, base + 1, base + 2, base, base + 2, base + 3);
    }
    return { type: 'mesh', vertices, indices, surface: 'grass' };
}
const ring = ringMesh();
const cupEvents = [];
const slowCup = await createPhysics(course({ tee: [0, 0.4, 0], cup: [0, 0, 0], colliders: [ring] }), { onEvent: event => cupEvents.push(event) });
assert.equal(slowCup.shoot([1, 0], 0.01), true);
for (let i = 0; i < 120 && !cupEvents.some(event => event.type === 'cup'); i++) slowCup.step(1 / 60);
assert.equal(cupEvents.filter(event => event.type === 'cup').length, 1);
slowCup.dispose();

const fastEvents = [];
const fastCup = await createPhysics(course({ tee: [-1, 0.16, 0], cup: [0, 0, 0], colliders: [ring] }), { onEvent: event => fastEvents.push(event) });
assert.equal(fastCup.shoot([1, 0], 1), true);
for (let i = 0; i < 30; i++) fastCup.step(1 / 60);
assert.equal(fastEvents.some(event => event.type === 'cup'), false);
assert.ok(fastCup.getBall()[0] > 0.6, `fast ball passes the cup opening at x=${fastCup.getBall()[0]}`);
fastCup.dispose();

const moving = await createPhysics(course({ obstacles: [{ id: 'gate', kind: 'slide', position: [0, 1, 1], halfExtents: [0.1, 0.5, 0.5], axis: [1, 0, 0], amplitude: 1, speed: 2, phase: 0 }] }));
const first = moving.getObstacles()[0];
let snapshot;
for (let i = 0; i < 30; i++) snapshot = moving.step(1 / 60);
const gate = moving.getObstacles()[0];
assert.ok(gate.position[0] > 0.7, JSON.stringify(gate));
assert.deepEqual(snapshot.obstacles, [gate]);
close(snapshot.time, 0.5, 1e-9, 'obstacle simulation time');
assert.deepEqual(first.position, [0, 1, 1]);
moving.dispose();

const compoundEvents = [];
const compoundGate = await createPhysics(course({
    obstacles: [{
        id: 'compound-gate', kind: 'rotate', position: [1, 0.2, 0], halfExtents: [0.7, 0.2, 0.5],
        colliders: [
            { position: [0, 0, 0], halfExtents: [0.7, 0.2, 0.08] },
            { position: [0, 0, 0], halfExtents: [0.08, 0.2, 0.5] }
        ],
        axis: [0, 1, 0], amplitude: 0, speed: 1, phase: 0
    }]
}), { onEvent: event => compoundEvents.push(event) });
assert.equal(compoundGate.shoot([1, 0], 0.35), true);
for (let i = 0; i < 90 && !compoundEvents.some(event => event.type === 'rest'); i++) compoundGate.step(1 / 60);
assert.ok(compoundEvents.some(event => event.type === 'impact' && event.surface === 'wood'), 'compound obstacle boxes collide at the kinematic body pose');
assert.ok(compoundGate.getBall()[0] < 0.95, `kinematic box is positioned relative to its parent, x=${compoundGate.getBall()[0]}`);
compoundGate.dispose();

const lowPowerEvents = [];
const lowPower = await createPhysics(course(), { onEvent: event => lowPowerEvents.push(event) });
assert.equal(lowPower.shoot([1, 0], 0.05), true);
for (let i = 0; i < 600 && !lowPowerEvents.some(event => event.type === 'rest'); i++) lowPower.step(1 / 60);
assert.equal(lowPowerEvents.filter(event => event.type === 'rest').length, 1, 'a gentle putt settles instead of lingering indefinitely');
assert.ok(lowPower.getSnapshot().time < 5, `gentle putt settles within five seconds (${lowPower.getSnapshot().time.toFixed(2)}s)`);
lowPower.dispose();

const distanceEvents = [];
const maximumDistance = await createPhysics(course({
    cup: [15, 0, 0], colliders: [{ type: 'box', position: [0, -0.1, 0], halfExtents: [19, 0.1, 19], surface: 'grass' }]
}), { onEvent: event => distanceEvents.push(event) });
assert.equal(maximumDistance.shoot([1, 0], 1), true);
for (let i = 0; i < 1200 && !distanceEvents.some(event => event.type === 'rest'); i++) maximumDistance.step(1 / 120);
assert.equal(distanceEvents.filter(event => event.type === 'rest').length, 1, 'a maximum-power flat shot eventually settles');
assert.ok(maximumDistance.getBall()[0] > 10, `full-power distance remains playable (${maximumDistance.getBall()[0].toFixed(2)}m)`);
assert.ok(maximumDistance.getSnapshot().time < 10, `maximum-power shot settles within ten seconds (${maximumDistance.getSnapshot().time.toFixed(2)}s)`);
maximumDistance.dispose();

async function distanceOn(surface) {
    const events = [];
    const physics = await createPhysics(course({
        tee: [0, 0.16, 0],
        cup: [15, 0, 0],
        colliders: [{ type: 'box', position: [0, -0.1, 0], halfExtents: [10, 0.1, 10], surface }]
    }), { onEvent: event => events.push(event) });
    assert.equal(physics.shoot([1, 0], 0.5), true);
    for (let i = 0; i < 1800 && !events.some(event => event.type === 'rest'); i++) physics.step(1 / 120);
    assert.equal(events.filter(event => event.type === 'rest').length, 1, `${surface} shot settles`);
    const distance = physics.getBall()[0];
    physics.dispose();
    return distance;
}
const grassDistance = await distanceOn('grass');
const sandDistance = await distanceOn('sand');
assert.ok(sandDistance < grassDistance * 0.65, `sand materially shortens rolling distance (${sandDistance.toFixed(2)}m vs ${grassDistance.toFixed(2)}m)`);

console.log('minigolf physics: all checks passed');
