import assert from 'node:assert/strict';
import { createPhysics, MAX_ROLL, launchSpeedFor, rollDistance, strengthToPower } from '../ui/js/desktop/apps/minigolf-physics.js';
import { multiply, obstaclePose, validateObstacle } from '../ui/js/desktop/apps/minigolf-mechanics.js';
import { readColliders } from '../ui/js/desktop/apps/minigolf-collision.js';
import { buildColliderGlb } from './lib/minigolf-glb.mjs';

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
    assert.ok(actual[0] - snapshot.position[0] <= launchSpeedFor(0.5) * fixedStep + 1e-4, `${fps} Hz render pose stays within one fixed step`);
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
    assert.ok(renderPositions[i] - renderPositions[i - 1] < launchSpeedFor(0.5) / 144 + 0.01, '144 Hz interpolation avoids fixed-step position jumps');
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
assert.ok(firstLaunchStep.position[1] <= 0.16 + 0.005 + 1e-4, `flat putt launched without a hop beyond the contact skin: y=${firstLaunchStep.position[1]}`);
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

// --- Arcade rolling model (P1) ----------------------------------------------
const wideBounds = { min: [-60, -5, -60], max: [60, 10, 60] };
const wideFloor = (surface = 'grass') => ({ type: 'box', position: [0, -0.1, 0], halfExtents: [50, 0.1, 50], surface });
const wide = (extra = {}) => course({ bounds: wideBounds, cup: [55, 0, 55], colliders: [wideFloor()], ...extra });

async function rollOut(definition, direction, power, maxSteps = 2400) {
    const events = [];
    const physics = await createPhysics(definition, { onEvent: event => events.push(event) });
    const start = physics.getBall();
    assert.equal(physics.shoot(direction, power), true);
    const launch = Math.hypot(...physics.getSnapshot().velocity);
    let after100ms = null, maxHop = 0, steps = 0;
    while (steps < maxSteps && !events.some(event => event.type !== 'impact')) {
        physics.step(fixedStep);
        steps++;
        const velocity = physics.getSnapshot().velocity;
        if (steps === 12) after100ms = Math.hypot(velocity[0], velocity[2]);
        maxHop = Math.max(maxHop, Math.abs(physics.getBall()[1] - start[1]));
    }
    const end = physics.getBall();
    physics.dispose();
    return { launch, after100ms, maxHop, time: steps * fixedStep, distance: Math.hypot(end[0] - start[0], end[2] - start[2]), terminal: events.find(event => event.type !== 'impact')?.type };
}

function gridMesh(size = 60, cell = 0.25) {
    const cells = Math.round(size / cell), vertices = [], indices = [];
    for (let i = 0; i <= cells; i++) for (let j = 0; j <= cells; j++) vertices.push(-size / 2 + i * cell, 0, -size / 2 + j * cell);
    for (let i = 0; i < cells; i++) for (let j = 0; j < cells; j++) {
        const a = i * (cells + 1) + j, b = a + 1, c = a + cells + 1, d = c + 1;
        if ((i + j) % 2) indices.push(a, b, d, a, d, c); else indices.push(a, b, c, b, d, c);
    }
    return { type: 'mesh', vertices, indices, surface: 'grass' };
}

let previousSpeed = 0;
for (let i = 1; i <= 20; i++) {
    const launchSpeed = launchSpeedFor(i / 20);
    assert.ok(launchSpeed > previousSpeed, `launch speed rises with power ${i / 20}`);
    previousSpeed = launchSpeed;
}
close(launchSpeedFor(1), 16.77, 0.05, 'full-power launch speed');
close(rollDistance(launchSpeedFor(0.5)), 0.5 * MAX_ROLL, 1e-6, 'launch speed inverts the grass roll distance');
close(strengthToPower(0.5), 0.5 ** 1.5, 1e-12, 'strength curve');
assert.equal(strengthToPower(1), 1);

const full = await rollOut(wide(), [1, 0], 1);
assert.equal(full.terminal, 'rest', 'a full-power flat shot comes to rest');
assert.ok(Math.abs(full.distance - MAX_ROLL) <= MAX_ROLL * 0.1, `full power rolls 28 m ± 10 % (${full.distance.toFixed(2)} m)`);
assert.ok(full.time <= 6, `full-power shot rests within 6 s (${full.time.toFixed(2)} s)`);
assert.ok(full.after100ms >= full.launch * 0.95, `no slide-to-roll loss (${(full.after100ms / full.launch * 100).toFixed(1)} % after 0.1 s)`);
for (const power of [0.1, 0.25, 0.5]) {
    const shot = await rollOut(wide(), [1, 0], power);
    assert.ok(Math.abs(shot.distance - power * MAX_ROLL) <= power * MAX_ROLL * 0.05, `power ${power} rolls ${(power * MAX_ROLL).toFixed(2)} m ± 5 % (${shot.distance.toFixed(2)} m)`);
}
const putt = await rollOut(wide(), [1, 0], 0.01);
assert.ok(putt.distance > 0.1 && putt.distance <= 0.3, `shortest putt rolls 0.1–0.3 m (${putt.distance.toFixed(3)} m)`);

const tessellated = await rollOut(wide({ tee: [-28, 0.16, 0.13], colliders: [gridMesh()] }), [1, 0], 1);
assert.ok(Math.abs(tessellated.distance - full.distance) <= full.distance * 0.05, `trimesh seams keep the roll distance (${tessellated.distance.toFixed(2)} m vs ${full.distance.toFixed(2)} m)`);
assert.ok(tessellated.maxHop <= 0.01, `no seam hops above 1 cm (${(tessellated.maxHop * 100).toFixed(2)} cm)`);

const typedGrid = gridMesh();
const typedRoll = await rollOut(wide({ tee: [-28, 0.16, 0.13], colliders: [{ ...typedGrid, vertices: Float32Array.from(typedGrid.vertices), indices: Uint32Array.from(typedGrid.indices) }] }), [1, 0], 1);
close(typedRoll.distance, tessellated.distance, 1e-9, 'typed-array meshes behave exactly like plain arrays');
await assert.rejects(createPhysics(course({ colliders: [{ type: 'mesh', surface: 'grass', vertices: Float32Array.of(0, 0, 0, 1, 0, 0, 0, 0, 1), indices: Uint32Array.of(0, 1, 5) }] })), /Invalid minigolf mesh collider/, 'typed meshes keep the index range check');

const bankRotation = [0, Math.sin(Math.PI / 8), 0, Math.cos(Math.PI / 8)];
for (const surface of ['wood', 'stone']) {
    const events = [];
    const physics = await createPhysics(wide({ colliders: [wideFloor(), { type: 'box', position: [4, 0.3, 0], halfExtents: [0.1, 0.3, 3], rotation: bankRotation, surface }] }), { onEvent: event => events.push(event) });
    assert.equal(physics.shoot([1, 0], 0.4), true);
    let before = null, contactStep = -1, after = null;
    for (let step = 0; step < 600 && after === null; step++) {
        const velocity = physics.getSnapshot().velocity;
        physics.step(fixedStep);
        if (contactStep < 0 && events.some(event => event.type === 'impact' && event.surface === surface)) {
            before = Math.hypot(velocity[0], velocity[2]);
            contactStep = step;
        } else if (contactStep >= 0 && step >= contactStep + 6) {
            const bounced = physics.getSnapshot().velocity;
            after = Math.hypot(bounced[0], bounced[2]);
        }
    }
    physics.dispose();
    assert.ok(before > 0 && after / before >= 0.65, `45° ${surface} bank keeps ≥ 65 % speed (${(after / before * 100).toFixed(0)} %)`);
}

const gentleEvents = [];
const gentleSlope = await createPhysics(course({
    tee: [1, 0.12, 0],
    colliders: [{ type: 'box', position: [0, -0.05, 0], halfExtents: [5, 0.05, 5], rotation: [0, 0, -Math.sin(0.022), Math.cos(0.022)], surface: 'grass' }]
}), { onEvent: event => gentleEvents.push(event) });
assert.equal(gentleSlope.shoot([0, 1], 0.01), true);
for (let i = 0; i < 360 && !gentleEvents.some(event => event.type === 'rest'); i++) gentleSlope.step(fixedStep);
assert.ok(gentleEvents.some(event => event.type === 'rest'), 'a ball on a 2.5° slope comes to rest');
assert.ok(Math.abs(gentleSlope.getBall()[0] - 1) < 0.2, `2.5° slope holds the ball near x=1 (${gentleSlope.getBall()[0].toFixed(3)})`);
gentleSlope.dispose();

const grassRoll = await rollOut(wide(), [1, 0], 0.5);
const sandRoll = await rollOut(wide({ colliders: [wideFloor('sand')] }), [1, 0], 0.5);
assert.ok(sandRoll.distance < grassRoll.distance * 0.65, `sand materially shortens rolling distance (${sandRoll.distance.toFixed(2)} m vs ${grassRoll.distance.toFixed(2)} m)`);

// --- Trajectory preview -----------------------------------------------------
const pathLength = (points, from = 0) => points.slice(from + 1).reduce((sum, point, index) => sum + Math.hypot(point[0] - points[from + index][0], point[2] - points[from + index][2]), 0);
const wallAhead = () => wide({ colliders: [wideFloor(), { type: 'box', position: [3, 0.3, 0], halfExtents: [0.1, 0.3, 3], surface: 'wood' }] });

const predictEvents = [];
const predicting = await createPhysics(wallAhead(), { onEvent: event => predictEvents.push(event) });
const ballBefore = predicting.getBall(), timeBefore = predicting.getSnapshot().time;
const preview = predicting.predict([1, 0], 0.5);
assert.ok(preview && preview.points.length >= 2, 'preview returns a path');
assert.ok(preview.contactIndex >= 0, 'preview marks the first wall contact');
assert.ok(pathLength(preview.points, preview.contactIndex) <= 1.0, `preview ends ≤ 1 m after the wall (${pathLength(preview.points, preview.contactIndex).toFixed(2)} m)`);
assert.equal(preview.hazard, null);
assert.deepEqual(predicting.getBall(), ballBefore, 'preview leaves the live ball in place');
assert.equal(predicting.getSnapshot().time, timeBefore, 'preview leaves live time untouched');
assert.equal(predictEvents.length, 0, 'preview emits no live events');
const strongPreview = predicting.predict([1, 0], 1);
assert.ok(strongPreview.contactIndex >= 0 && pathLength(strongPreview.points, strongPreview.contactIndex) <= 1.0 + 1e-6, `full-power preview ends ≤ 1 m after the wall (${pathLength(strongPreview.points, strongPreview.contactIndex).toFixed(3)} m)`);

async function restAfterShot(physics, events, checkPreview = false) {
    assert.equal(physics.shoot([1, 0], 0.5), true);
    if (checkPreview) assert.equal(physics.predict([1, 0], 0.5), null, 'no preview while a shot is resolving');
    for (let i = 0; i < 2400 && !events.some(event => event.type !== 'impact'); i++) physics.step(fixedStep);
    return physics.getBall();
}
const predictedRest = await restAfterShot(predicting, predictEvents, true);
const freshEvents = [];
const fresh = await createPhysics(wallAhead(), { onEvent: event => freshEvents.push(event) });
const freshRest = await restAfterShot(fresh, freshEvents);
freshRest.forEach((value, axis) => close(value, predictedRest[axis], 1e-9, 'a preview does not change the following live shot'));
predicting.dispose();
fresh.dispose();

const openField = await createPhysics(wide());
const longPreview = openField.predict([1, 0], 1);
assert.equal(longPreview.contactIndex, -1, 'open field has no wall contact');
assert.ok(pathLength(longPreview.points) <= 0.45 * MAX_ROLL + 0.2, `long preview stops at 45 % of the roll (${pathLength(longPreview.points).toFixed(2)} m)`);
openField.dispose();

// The preview follows the live shot, including a moving gate.
const gateCourse = () => wide({ obstacles: [{ id: 'slider', kind: 'slide', position: [2.5, 0.3, 0], halfExtents: [0.1, 0.3, 0.5], axis: [0, 0, 1], amplitude: 0.3, speed: 1, phase: 0 }] });
const gated = await createPhysics(gateCourse());
for (let i = 0; i < 37; i++) gated.step(fixedStep);
const gatePreview = gated.predict([1, 0], 0.3);
assert.ok(gatePreview.contactIndex >= 0, 'the gate preview reaches the moving gate');
const livePath = [gated.getBall()];
assert.equal(gated.shoot([1, 0], 0.3), true);
for (let i = 0; i < 360; i++) { gated.step(fixedStep); livePath.push(gated.getBall()); }
const offPath = Math.max(...gatePreview.points.map(point => Math.min(...livePath.map(live => Math.hypot(live[0] - point[0], live[2] - point[2])))));
assert.ok(offPath <= 0.03, `preview follows the live shot past a moving gate (max ${(offPath * 100).toFixed(1)} cm off)`);
gated.dispose();

// --- Mechanics, obstacle kinds, gravity and loop (P3) ------------------------
const TERMINAL = ['rest', 'cup', 'water', 'out'];
const terminalOf = events => events.find(event => TERMINAL.includes(event.type));
const horizontal = velocity => Math.hypot(velocity[0], velocity[2]);
function runUntil(physics, done, limit = 1800) {
    for (let step = 1; step <= limit; step++) { physics.step(fixedStep); if (done()) return step; }
    return -1;
}

// 1. Per-course gravity.
for (const bad of [0, -1, 25, NaN]) await assert.rejects(createPhysics(wide({ gravity: bad })), TypeError, `gravity ${bad} is rejected`);
{
    const events = [];
    const moon = await createPhysics(wide({ gravity: 4 }), { onEvent: event => events.push(event) });
    assert.equal(moon.setBall([0, 2, 0]), true);
    assert.equal(moon.shoot([1, 0], 0.01), true);
    for (let i = 0; i < 108; i++) moon.step(fixedStep);
    assert.equal(events.some(event => event.type === 'impact'), false, `still falling after 0.9 s at 4 m/s² (y=${moon.getBall()[1].toFixed(2)})`);
    for (let i = 0; i < 36; i++) moon.step(fixedStep);
    assert.equal(events.some(event => event.type === 'impact'), true, 'landed by 1.2 s (fall time 0.96 s)');
    moon.dispose();
}

// 2. A boost pad reports without ending the shot and sets the horizontal speed.
const boostPad = { id: 'boost-a', kind: 'boost', position: [1.5, 0.16, 0], half: [0.3, 0.4, 3], yaw: 0, direction: [1, 0], speed: 6 };
{
    const events = [];
    const physics = await createPhysics(wide({ mechanics: [boostPad] }), { onEvent: event => events.push(event) });
    assert.equal(physics.shoot([1, 0], 0.1), true);
    let afterBoost = null;
    for (let i = 0; i < 1800 && !terminalOf(events); i++) {
        const snapshot = physics.step(fixedStep);
        if (!afterBoost && events.some(event => event.type === 'boost')) afterBoost = snapshot.velocity;
    }
    assert.equal(events.filter(event => event.type === 'boost').length, 1, 'exactly one boost event');
    assert.equal(events.find(event => event.type === 'boost').id, 'boost-a');
    assert.ok(horizontal(afterBoost) >= boostPad.speed - 0.3 && Math.abs(afterBoost[2]) < 1e-6, `boosted to ${horizontal(afterBoost).toFixed(2)} m/s along the pad`);
    assert.ok(['rest', 'out'].includes(terminalOf(events)?.type), 'the shot ends with rest or out, not with the boost');
    assert.ok(physics.getBall()[0] > 5, `the ball kept rolling after the pad (x=${physics.getBall()[0].toFixed(2)})`);
    physics.dispose();
}

// 3. Teleporter: moves the ball once, keeps the shot alive, no second trigger at the exit.
const teleporter = { id: 'tp-a', kind: 'teleport', position: [2, 0, 0], radius: 0.4, exit: [10, 0.17, 0], direction: [1, 0], minSpeed: 2 };
{
    const events = [];
    const physics = await createPhysics(wide({ mechanics: [teleporter] }), { onEvent: event => events.push(event) });
    assert.equal(physics.shoot([1, 0], 0.3), true);
    let jump = null;
    for (let i = 0; i < 1800 && !terminalOf(events); i++) {
        const snapshot = physics.step(fixedStep);
        if (!jump && events.some(event => event.type === 'teleport')) jump = snapshot;
    }
    assert.ok(jump, 'teleport event arrived');
    assert.ok(jump.position[0] >= 10, `render pose jumps with the ball, no tween (x=${jump.position[0]})`);
    assert.ok(horizontal(jump.velocity) >= 2 && jump.velocity[0] > 0, 'exit speed follows direction and minSpeed');
    assert.equal(events.filter(event => event.type === 'teleport').length, 1, 'no second trigger at the exit');
    assert.deepEqual(events.find(event => event.type === 'teleport').exit, [10, 0.17, 0]);
    assert.ok(physics.getBall()[0] >= 10 && terminalOf(events)?.type === 'rest', 'the shot continues from the exit');
    physics.dispose();
}

// 4. Cannon: captures, holds at the muzzle, fires once, deterministically.
const cannon = { id: 'cannon-a', kind: 'cannon', position: [2, 0, 0], radius: 0.4, muzzle: [2, 1.2, 0], velocity: [4, 3, 0], delay: 0.6 };
async function cannonRun() {
    const events = [];
    const physics = await createPhysics(wide({ mechanics: [cannon] }), { onEvent: event => events.push(event) });
    assert.equal(physics.shoot([1, 0], 0.2), true);
    const captured = runUntil(physics, () => physics.getBall()[1] > 1.1);
    assert.ok(captured > 0, 'the cannon caught the ball');
    for (let i = 0; i < 60; i++) physics.step(fixedStep);
    cannon.muzzle.forEach((value, axis) => close(physics.getBall()[axis], value, 0.001, 'ball waits at the muzzle'));
    assert.equal(physics.getSnapshot().moving, true, 'a held ball still counts as moving');
    assert.equal(events.some(event => event.type === 'cannon'), false, 'no event before the shot is fired');
    runUntil(physics, () => terminalOf(events));
    const fired = events.filter(event => event.type === 'cannon');
    assert.equal(fired.length, 1, 'exactly one cannon event');
    assert.deepEqual(fired[0].position, cannon.muzzle);
    assert.equal(terminalOf(events)?.type, 'rest');
    const rest = physics.getBall();
    physics.dispose();
    return rest;
}
const cannonFirst = await cannonRun();
assert.ok(cannonFirst[0] > 4, `the shell flies on (x=${cannonFirst[0].toFixed(2)})`);
assert.deepEqual(await cannonRun(), cannonFirst, 'two cannon runs rest on bit-identical positions');

// 5. Bumper: kicks the ball away at least with `kick`.
{
    const bumper = { id: 'pin-a', kind: 'bumper', position: [2, 0, 0], radius: 0.3, height: 0.5, surface: 'wood', kick: 5 };
    const events = [];
    const physics = await createPhysics(wide({ mechanics: [bumper] }), { onEvent: event => events.push(event) });
    assert.equal(physics.shoot([1, 0], 0.08), true);
    let kicked = null;
    for (let i = 0; i < 1800 && !terminalOf(events); i++) {
        const snapshot = physics.step(fixedStep);
        if (!kicked && events.some(event => event.type === 'bumper')) kicked = snapshot;
    }
    assert.ok(kicked, 'bumper event arrived');
    assert.ok(events.some(event => event.type === 'impact' && event.surface === 'wood'), 'the contact still reports an impact');
    assert.ok(horizontal(kicked.velocity) >= 0.95 * bumper.kick && kicked.velocity[0] < 0, `kicked away at ${horizontal(kicked.velocity).toFixed(2)} m/s`);
    assert.ok(physics.getBall()[0] < 1.54, 'the ball ends up behind the bumper');
    physics.dispose();
}

// 6. Obstacle kinds follow their formulas; ball and cylinder parts collide.
{
    const q = (axis, angle) => [axis[0] * Math.sin(angle / 2), axis[1] * Math.sin(angle / 2), axis[2] * Math.sin(angle / 2), Math.cos(angle / 2)];
    const nearly = (actual, expected, message, tolerance = 1e-12) => actual.forEach((value, i) => close(value, expected[i], tolerance, message));
    const spin = { id: 'spin-a', kind: 'spin', position: [1, 2, 3], axis: [0, 1, 0], speed: 2, phase: 0.5, amplitude: 0 };
    nearly(obstaclePose(spin, 0.7).rotation, q([0, 1, 0], 0.7 * 2 + 0.5), 'spin angle = speed * t + phase');
    assert.deepEqual(obstaclePose(spin, 0.7).position, [1, 2, 3]);
    const { amplitude: _unused, ...steadySpin } = spin;
    assert.doesNotThrow(() => validateObstacle({ ...steadySpin, halfExtents: [1, 0.2, 0.2] }), 'a spin needs no amplitude');
    assert.throws(() => validateObstacle({ ...steadySpin, kind: 'slide', halfExtents: [1, 0.2, 0.2] }), TypeError, 'other kinds still need an amplitude');
    const gate = { id: 'gate-a', kind: 'gate', position: [1, 0, 0], axis: [1, 0, 0], amplitude: 1, speed: 1, phase: 0, sharp: 3 };
    const sineAt = value => Math.asin(value);
    close(obstaclePose(gate, sineAt(0.2)).position[0], 1 + 0.6, 1e-12, 'gate moves with sharp * sin below the plateau');
    close(obstaclePose(gate, sineAt(0.9)).position[0], 2, 1e-12, 'gate holds at +amplitude');
    close(obstaclePose(gate, Math.PI + sineAt(0.9)).position[0], 0, 1e-12, 'gate holds at -amplitude');
    const boulder = { id: 'rock-a', kind: 'boulder', position: [0, 0.4, 0], axis: [1, 0, 0], amplitude: 2, speed: 1, phase: 0, radius: 0.4 };
    const travelled = 2 * Math.sin(1.1);
    close(obstaclePose(boulder, 1.1).position[0], travelled, 1e-12, 'boulder slides like slide');
    nearly(obstaclePose(boulder, 1.1).rotation, q([0, 0, -1], travelled / 0.4), 'boulder rolls by distance / radius about up x axis');
    const swing = { id: 'swing-a', kind: 'rotate', position: [0, 1, 0], axis: [0, 0, 1], amplitude: 0.5, speed: 1, phase: 0, rotation: q([0, 1, 0], 0.3) };
    nearly(obstaclePose(swing, 0.8).rotation, multiply(q([0, 0, 1], 0.5 * Math.sin(0.8)), swing.rotation), 'rotate composes the swing with the base rotation');

    const live = await createPhysics(wide({ obstacles: [{ ...gate, position: [4, 0.3, 3], halfExtents: [0.1, 0.3, 0.5] }, { ...boulder, position: [4, 0.4, -3], colliders: [{ shape: 'ball', position: [0, 0, 0], radius: 0.4 }] }] }));
    let snapshot;
    for (let i = 0; i < 90; i++) snapshot = live.step(fixedStep);
    const poses = live.getObstacles();
    const expected = [obstaclePose({ ...gate, position: [4, 0.3, 3] }, snapshot.time - fixedStep), obstaclePose({ ...boulder, position: [4, 0.4, -3] }, snapshot.time - fixedStep)];
    expected.forEach((pose, i) => { nearly(poses[i].position, pose.position, 'live position follows the formula', 1e-6); nearly(poses[i].rotation, pose.rotation, 'live rotation follows the formula', 1e-6); });
    live.dispose();

    for (const part of [{ shape: 'ball', position: [0, 0, 0], radius: 0.3 }, { shape: 'cylinder', position: [0, 0, 0], radius: 0.3, halfHeight: 0.3 }]) {
        const events = [];
        const physics = await createPhysics(wide({ obstacles: [{ id: 'post-a', kind: 'spin', position: [2, 0.3, 0], axis: [0, 1, 0], speed: 1, amplitude: 0, colliders: [part] }] }), { onEvent: event => events.push(event) });
        assert.equal(physics.shoot([1, 0], 0.2), true);
        runUntil(physics, () => terminalOf(events));
        assert.ok(events.some(event => event.type === 'impact' && event.surface === 'wood'), `${part.shape} part reports a wood impact`);
        assert.ok(physics.getBall()[0] < 1.6, `${part.shape} part stops the ball (x=${physics.getBall()[0].toFixed(2)})`);
        physics.dispose();
    }
    for (const broken of [{ ...spin, colliders: [{ shape: 'cone', radius: 1 }] }, { ...spin, colliders: [{ shape: 'ball', radius: 0 }] }, { ...boulder, radius: 0 }, { ...gate, sharp: 9, halfExtents: [1, 1, 1] }, { ...spin, id: 'Bad Id', halfExtents: [1, 1, 1] }]) {
        await assert.rejects(createPhysics(wide({ obstacles: [broken] })), /Invalid minigolf obstacle/, 'invalid obstacle is rejected');
    }
    for (const broken of [{ ...boostPad, speed: 0 }, { ...boostPad, direction: [2, 0] }, { ...teleporter, exit: [1, 2] }, { ...cannon, delay: -1 }, { id: 'pin-b', kind: 'bumper', position: [0, 0, 0], radius: 0.3, height: 0.5, kick: 0 }, { ...boostPad, kind: 'warp' }]) {
        await assert.rejects(createPhysics(wide({ mechanics: [broken] })), /Invalid minigolf mechanic/, 'invalid mechanic is rejected');
    }
    await assert.rejects(createPhysics(wide({ mechanics: [boostPad, { ...teleporter, id: boostPad.id }] })), /Duplicate minigolf mechanic id/);
}

// 7. The preview applies boost pads and bumpers and stops at teleporters and cannons.
{
    const open = await createPhysics(wide());
    const padded = await createPhysics(wide({ mechanics: [{ ...boostPad, position: [0.6, 0.16, 0], half: [0.2, 0.4, 3] }] }));
    const withoutPad = open.predict([1, 0], 0.04), withPad = padded.predict([1, 0], 0.04);
    assert.ok(pathLength(withPad.points) > pathLength(withoutPad.points) + 0.2, `boost lengthens the preview (${pathLength(withPad.points).toFixed(2)} m vs ${pathLength(withoutPad.points).toFixed(2)} m)`);
    assert.equal(withPad.zone, null);
    const livePath = [padded.getBall()];
    assert.equal(padded.shoot([1, 0], 0.04), true);
    for (let i = 0; i < 180; i++) { padded.step(fixedStep); livePath.push(padded.getBall()); }
    const off = Math.max(...withPad.points.map(point => Math.min(...livePath.map(live => Math.hypot(live[0] - point[0], live[2] - point[2])))));
    assert.ok(off <= 0.03, `the boosted preview follows the live shot (max ${(off * 100).toFixed(1)} cm off)`);
    open.dispose(); padded.dispose();

    for (const [mechanic, zone] of [[{ ...teleporter, position: [1.5, 0, 0] }, 'teleport'], [{ ...cannon, position: [1.5, 0, 0] }, 'cannon']]) {
        const physics = await createPhysics(wide({ mechanics: [mechanic] }));
        const preview = physics.predict([1, 0], 0.3);
        assert.equal(preview.zone, zone, `the preview ends at the ${zone} zone`);
        const end = preview.points[preview.points.length - 1];
        assert.ok(end[0] > 1 && end[0] < 2, `the ${zone} preview stops inside the zone (x=${end[0].toFixed(2)})`);
        physics.dispose();
    }
    const pin = await createPhysics(wide({ mechanics: [{ id: 'pin-a', kind: 'bumper', position: [2, 0, 0], radius: 0.3, height: 0.5, surface: 'wood', kick: 5 }] }));
    const bounced = pin.predict([1, 0], 0.3);
    assert.ok(bounced.contactIndex >= 0 && bounced.points[bounced.points.length - 1][0] < 2, 'the preview bounces off a bumper');
    pin.dispose();
}

// 8. Loop prototype (feasibility): 8 m run-up, loop radius 1.3 m with a 2.2 m side drift,
// 6 m run-out, gravity 4. Runtime coordinates, Y up, 48 cross sections around the loop.
function loopCourse({ boost = true, radius = 1.3, offset = 2.2, runUp = 8, runOut = 6, steps = 48 } = {}) {
    const WIDTH = 1.72, SPACING = WIDTH / 12, add = (a, b) => a.map((v, i) => v + b[i]), scale = (a, k) => a.map(v => v * k), minus = (a, b) => a.map((v, i) => v - b[i]);
    const dot = (a, b) => a[0] * b[0] + a[1] * b[1] + a[2] * b[2], unit = a => scale(a, 1 / Math.hypot(...a));
    const cross = (a, b) => [a[1] * b[2] - a[2] * b[1], a[2] * b[0] - a[0] * b[2], a[0] * b[1] - a[1] * b[0]];
    const sections = [];
    for (let i = 0; i <= runUp / 0.25; i++) sections.push({ center: [i * 0.25, 0, 0], across: [0, 0, 1], normal: [0, 1, 0] });
    for (let k = 1; k <= steps; k++) {
        const theta = 2 * Math.PI * k / steps, s = k / steps, side = [0, 0, 1];
        const center = [runUp + radius * Math.sin(theta), radius * (1 - Math.cos(theta)), offset * (3 * s * s - 2 * s * s * s)];
        const tangent = unit([radius * Math.cos(theta), radius * Math.sin(theta), offset * (6 * s - 6 * s * s) / (2 * Math.PI)]);
        const across = unit(minus(side, scale(tangent, dot(side, tangent))));
        sections.push({ center, across, normal: cross(across, tangent) });
    }
    for (let i = 1; i <= runOut / 0.25; i++) sections.push({ center: [runUp + i * 0.25, 0, offset], across: [0, 0, 1], normal: [0, 1, 0] });
    const mesh = () => ({ vertices: [], indices: [] });
    const floor = mesh(), rails = mesh();
    const vertex = (target, p) => target.vertices.push(...p) / 3 - 1;
    // Quads are wound so that their normal faces `facing` (the side the ball is on).
    const quad = (target, ids, facing) => {
        const [a, b, c] = ids.map(id => target.vertices.slice(id * 3, id * 3 + 3));
        const [i0, i1, i2, i3] = ids;
        if (dot(cross(minus(b, a), minus(c, a)), facing) >= 0) target.indices.push(i0, i1, i2, i0, i2, i3); else target.indices.push(i0, i2, i1, i0, i3, i2);
    };
    const rows = sections.map(({ center, across }) => Array.from({ length: 13 }, (_, j) => vertex(floor, add(center, scale(across, WIDTH / 2 - j * SPACING)))));
    for (let i = 0; i + 1 < sections.length; i++) for (let j = 0; j < 12; j++) quad(floor, [rows[i][j], rows[i][j + 1], rows[i + 1][j + 1], rows[i + 1][j]], sections[i].normal);
    for (const sign of [1, -1]) {
        const profile = sections.map(({ center, across, normal }) => {
            const edge = add(center, scale(across, sign * WIDTH / 2)), out = scale(across, sign * 0.14);
            const bottom = minus(edge, scale(normal, 0.045)), top = add(edge, scale(normal, 0.24));
            return [vertex(rails, bottom), vertex(rails, top), vertex(rails, add(top, out)), vertex(rails, add(bottom, out))];
        });
        for (let i = 0; i + 1 < sections.length; i++) for (const [p, q, facing] of [[0, 1, scale(sections[i].across, -sign)], [1, 2, sections[i].normal], [2, 3, scale(sections[i].across, sign)]]) {
            quad(rails, [profile[i][p], profile[i][q], profile[i + 1][q], profile[i + 1][p]], facing);
        }
    }
    const endX = runUp + runOut, lo = offset - WIDTH / 2 - 0.14, hi = offset + WIDTH / 2 + 0.14;
    const face = (corners, facing) => quad(rails, corners.map(p => vertex(rails, p)), facing);
    face([[endX, -0.045, lo], [endX, 0.24, lo], [endX, 0.24, hi], [endX, -0.045, hi]], [-1, 0, 0]);
    face([[endX + 0.14, -0.045, lo], [endX + 0.14, -0.045, hi], [endX + 0.14, 0.24, hi], [endX + 0.14, 0.24, lo]], [1, 0, 0]);
    face([[endX, 0.24, lo], [endX + 0.14, 0.24, lo], [endX + 0.14, 0.24, hi], [endX, 0.24, hi]], [0, 1, 0]);
    return wide({
        id: 'loop', tee: [0.5, 0.16, 0], cup: [endX - 1, 0, offset], gravity: 4, bounds: { min: [-4, -4, -4], max: [endX + 6, 8, offset + 4] },
        // The pad sits 1.5 m ahead of the loop; farther away the run-up friction eats the boost.
        mechanics: boost ? [{ id: 'boost-loop', kind: 'boost', position: [runUp - 1.5, 0.16, 0], half: [0.6, 0.4, WIDTH / 2], yaw: 0, direction: [1, 0], speed: 9 }] : [],
        colliders: [{ type: 'mesh', surface: 'grass', vertices: Float32Array.from(floor.vertices), indices: Uint32Array.from(floor.indices) }, { type: 'mesh', surface: 'wood', vertices: Float32Array.from(rails.vertices), indices: Uint32Array.from(rails.indices) }]
    });
}
async function loopShot(definition, power) {
    const events = [];
    const physics = await createPhysics(definition, { onEvent: event => events.push(event) });
    assert.equal(physics.shoot([1, 0], power), true);
    let apex = 0, exited = false;
    for (let step = 0; step < 1800 && !terminalOf(events); step++) {
        physics.step(fixedStep);
        const [x, y, z] = physics.getBall();
        apex = Math.max(apex, y);
        if (x > 8.4 && Math.abs(z - 2.2) < 1 && y < 0.5) exited = true;
    }
    const end = physics.getBall();
    physics.dispose();
    return { apex, exited, end, boosts: events.filter(event => event.type === 'boost').length, terminal: terminalOf(events)?.type };
}
{
    const boosted = loopCourse();
    // The ball must hug the track over the apex (centre height 2 * 1.3 - 0.16 = 2.44 m) and arrive in the run-out.
    for (const power of [0.2, 0.3]) {
        const shot = await loopShot(boosted, power);
        assert.ok(shot.boosts === 1 && shot.apex >= 2.38 && shot.exited && shot.terminal === 'rest', `power ${power} with boost passes the loop (apex ${shot.apex.toFixed(2)} m, ends x=${shot.end[0].toFixed(1)})`);
    }
    const plain = await loopShot(loopCourse({ boost: false }), 0.15);
    assert.ok(plain.boosts === 0 && plain.apex < 0.3 && !plain.exited && plain.terminal === 'rest' && plain.end[0] < 8, `without a boost power 0.15 stays in the run-up (x=${plain.end[0].toFixed(1)})`);
}

// 9. A seam between two separately built strips is welded away: no hop above 1 cm.
{
    const strip = x0 => {
        const vertices = [], indices = [];
        for (let i = 0; i <= 16; i++) for (let j = 0; j <= 12; j++) vertices.push(x0 + i * 0.25, 0.3, -0.86 + j * (1.72 / 12));
        for (let i = 0; i < 16; i++) for (let j = 0; j < 12; j++) { const a = i * 13 + j; indices.push(a, a + 1, a + 14, a, a + 14, a + 13); }
        return { surface: 'grass', vertices, indices };
    };
    const welded = readColliders(buildColliderGlb([strip(0), strip(4)]));
    assert.equal(welded.length, 1, 'both strips become one grass mesh');
    assert.equal(welded[0].vertices.length / 3, 2 * 17 * 13 - 13, 'the 13 seam vertices are shared');
    const seam = await createPhysics(wide({ tee: [0.5, 0.46, 0], colliders: welded }));
    assert.equal(seam.shoot([1, 0], 0.4), true);
    let hop = 0;
    for (let i = 0; i < 400 && seam.getBall()[0] < 7.5; i++) { seam.step(fixedStep); hop = Math.max(hop, Math.abs(seam.getBall()[1] - 0.46)); }
    assert.ok(seam.getBall()[0] > 7, 'the ball crossed the seam');
    assert.ok(hop <= 0.01, `no hop above 1 cm over the seam (${(hop * 100).toFixed(2)} cm)`);
    seam.dispose();
}

console.log('minigolf physics: all checks passed');
