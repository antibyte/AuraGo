import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { createPhysics } from '../ui/js/desktop/apps/minigolf-physics.js';

const manifest = JSON.parse(await readFile(new URL('../ui/3d/minigolf/manifest.json', import.meta.url), 'utf8'));
assert.equal(manifest.version, 1);
assert.equal(manifest.courses.length, 18);
const close = (actual, expected, message) => assert.ok(Math.abs(actual - expected) <= 1e-5, `${message}: ${actual} vs ${expected}`);
const horizontalDistance = (position, target) => Math.hypot(position[0] - target[0], position[2] - target[2]);

// Deterministic playable routes found against the exported Rapier geometry.
// Set AURAGO_MINIGOLF_SEARCH=1 to rerun the slower coarse route discovery.
const ROUTES = {
    'garden-01': [{ angle: 3.141592653589793, power: 0.9 }],
    'garden-02': [{ angle: -3.1338970074971737, power: 0.95 }],
    'garden-03': [{ angle: -3.394485381500596, power: 1 }],
    'garden-04': [{ angle: 3.1302037161209193, power: 0.7 }],
    'garden-05': [{ angle: -3.124900089818196, power: 0.975 }, { angle: -4.8581913447888585, power: 0.55 }],
    'garden-06': [{ angle: -2.878714631232768, power: 0.85 }, { angle: 1.6125627320502407, power: 1, wait: 1.5 }],
    'coast-07': [{ angle: 3.07599060245278, power: 0.7 }, { angle: 2.1452008615359777, power: 1 }],
    'coast-08': [{ angle: -3.1157889013048896, power: 1 }],
    'coast-09': [{ angle: -3.1362450018090966, power: 0.9 }],
    'coast-10': [{ angle: -3.1349887067119764, power: 1 }, { angle: -4.797565855540604, power: 0.85 }],
    'coast-11': [{ angle: -3.991407202631565, power: 0.8 }, { angle: -3.111628975767671, power: 0.2, wait: 3.009 }],
    'coast-12': [{ angle: 3.033034331212618, power: 0.95 }, { angle: 2.601289829309998, power: 0.4 }],
    'night-13': [{ angle: 3.08726085196407, power: 0.55 }, { angle: 2.942502248109428, power: 0.55 }],
    'night-14': [{ angle: -3.107083897644142, power: 0.925 }, { angle: 1.8936867393410872, power: 0.5 }],
    'night-15': [{ angle: 3.1223143485473197, power: 1 }, { angle: -2.093073619776795, power: 0.12 }],
    'night-16': [{ angle: -3.6639796232205186, power: 1 }, { angle: -4.140663833934456, power: 0.55 }],
    'night-17': [{ angle: -3.1300028695125954, power: 1 }, { angle: -3.3271326577227267, power: 0.12 }],
    'night-18': [{ angle: -3.332961869066842, power: 1 }]
};

function runShot(physics, position, shot, events, { resetTime = true } = {}) {
    events.length = 0;
    physics.setBall(position);
    if (resetTime) physics.resetTime();
    assert.equal(physics.shoot([Math.sin(shot.angle), Math.cos(shot.angle)], shot.power), true);
    for (let frame = 0; frame < 900; frame++) {
        physics.step(1 / 60);
        const terminal = events.find(event => ['rest', 'cup', 'water', 'out'].includes(event.type));
        if (terminal) return terminal;
    }
    return null;
}

function findCupPath(physics, course, events) {
    const directPowers = [0.12, 0.2, 0.3, 0.4, 0.55, 0.7, 0.85, 0.9, 0.925, 0.95, 0.975, 1];
    const angleOffsets = [-150, -110, -80, -50, -30, -15, 0, 15, 30, 50, 80, 110, 150, 180];
    const powers = [0.12, 0.25, 0.4, 0.55, 0.7, 0.85, 0.925, 1];
    const shot = (position, angle, power) => {
        const result = runShot(physics, position, { angle, power }, events);
        return result && { type: result.type, position: result.position };
    };

    const targetAngle = position => Math.atan2(course.cup[0] - position[0], course.cup[2] - position[2]);
    const initialAngle = targetAngle(course.tee);
    let beam = [{ position: course.tee, path: [] }];
    const seen = new Set();
    const key = position => position.map(value => Math.round(value / 0.28)).join(',');
    seen.add(key(course.tee));

    for (const power of directPowers) {
        const result = shot(course.tee, initialAngle, power);
        if (result?.type === 'cup') return [{ angle: initialAngle, power }];
        if (result?.type === 'rest') beam.push({ position: result.position, path: [{ angle: initialAngle, power }] });
    }
    beam = [beam[0], ...beam.slice(1).sort((a, b) => horizontalDistance(a.position, course.cup) - horizontalDistance(b.position, course.cup)).slice(0, 2)];
    for (let depth = 0; depth < 8 && beam.length; depth++) {
        const next = [];
        for (const state of beam) {
            const base = targetAngle(state.position);
            for (const offset of angleOffsets) for (const power of powers) {
                const result = shot(state.position, base + offset * Math.PI / 180, power);
                if (!result) continue;
                const path = [...state.path, { angle: base + offset * Math.PI / 180, power }];
                if (result.type === 'cup') return path;
                if (result.type !== 'rest' || path.length >= 12) continue;
                const cell = key(result.position);
                if (seen.has(cell)) continue;
                seen.add(cell);
                next.push({ position: result.position, path });
            }
        }
        next.sort((a, b) => horizontalDistance(a.position, course.cup) + a.path.length * 0.08 - horizontalDistance(b.position, course.cup) - b.path.length * 0.08);
        beam = next.slice(0, 5);
    }
    return null;
}

for (const course of manifest.courses) {
    assert.ok(course.colliders.some(collider => collider.type === 'mesh' && collider.surface === 'grass'), `${course.id}: grass collision mesh`);
    const events = [];
    const physics = await createPhysics(course, { onEvent: event => events.push(event) });
    try {
        physics.getBall().forEach((value, axis) => close(value, course.tee[axis], `${course.id}: tee position`));
        const initialObstacles = physics.getObstacles();
        const direction = [course.cup[0] - course.tee[0], course.cup[2] - course.tee[2]];
        const length = Math.hypot(...direction);
        assert.ok(length > 0, `${course.id}: tee and cup differ`);
        assert.equal(physics.shoot([direction[0] / length, direction[1] / length], 0.12), true, `${course.id}: accepts putt`);
        for (let i = 0; i < 6; i++) physics.step(1 / 60);

        const position = physics.getBall();
        assert.ok(position.every(Number.isFinite), `${course.id}: finite ball pose`);
        assert.ok(position.every((value, axis) => value >= course.bounds.min[axis] && value <= course.bounds.max[axis]), `${course.id}: ball remains in bounds during tee shot`);
        assert.ok(Math.hypot(position[0] - course.tee[0], position[2] - course.tee[2]) > 0.025, `${course.id}: putt moves across the course mesh`);
        assert.ok(events.some(event => event.type === 'impact' && event.surface === 'grass'), `${course.id}: Rapier ball contacts the exported grass mesh`);
        assert.equal(events.some(event => ['cup', 'water', 'out', 'rest'].includes(event.type)), false, `${course.id}: gentle tee shot remains in play`);

        const currentObstacles = physics.getObstacles();
        assert.equal(currentObstacles.length, course.obstacles.length, `${course.id}: obstacle list`);
        currentObstacles.forEach((obstacle, index) => {
            const initial = initialObstacles[index];
            const moved = obstacle.position.some((value, axis) => Math.abs(value - initial.position[axis]) > 1e-4)
                || obstacle.rotation.some((value, axis) => Math.abs(value - initial.rotation[axis]) > 1e-4);
            assert.ok(moved, `${course.id}: obstacle ${obstacle.id} advances with physics time`);
        });

        const route = process.env.AURAGO_MINIGOLF_SEARCH === '1' ? findCupPath(physics, course, events) : ROUTES[course.id];
        const path = route;
        assert.ok(path, `${course.id}: coarse shot search finds a playable route to the cup`);
        assert.ok(path.length <= 12, `${course.id}: route respects the game's stroke cap`);
        let routePosition = course.tee;
        physics.resetTime();
        for (let index = 0; index < path.length; index++) {
            const wait = path[index].wait || 0;
            for (let tick = 0; tick < Math.round(wait * 120); tick++) physics.step(1 / 120);
            const terminal = runShot(physics, routePosition, path[index], events, { resetTime: false });
            assert.ok(terminal, `${course.id}: shot ${index + 1} reaches a terminal state within 15 seconds`);
            assert.equal(terminal.type, index === path.length - 1 ? 'cup' : 'rest', `${course.id}: shot ${index + 1} follows a playable route`);
            routePosition = terminal.position;
            if (wait > 0) console.log(`${course.id}: waited ${wait.toFixed(2)}s for a moving gate`);
        }
        if (process.env.AURAGO_MINIGOLF_SEARCH === '1') console.log(`${course.id}: reachable in ${path.length} shot${path.length === 1 ? '' : 's'} ${JSON.stringify(path)}`);
        else console.log(`${course.id}: recorded route reaches the cup in ${path.length} shot${path.length === 1 ? '' : 's'}`);
    } finally {
        physics.dispose();
    }
}

console.log('minigolf courses: all 18 manifest courses loaded and accepted grounded tee shots');
