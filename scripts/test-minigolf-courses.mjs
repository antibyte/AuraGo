// Plays every AuraGolf course (manifest v2) against the real Rapier physics, with the collision meshes read from the
// course GLBs.
//
//   node scripts/test-minigolf-courses.mjs
//
// Environment:
//   AURAGO_MINIGOLF_DIR=<dir>      asset folder with manifest.json and the course GLBs (default ui/3d/minigolf)
//   AURAGO_MINIGOLF_PARTIAL=1      accept a subset of the 18 courses (staging builds); a course without a recorded
//                                  route is then skipped with a note instead of failing
//   AURAGO_MINIGOLF_COURSE=<id>    only this course (id, number or <world>-<NN>)
//   AURAGO_MINIGOLF_SEARCH=1       search every route anew (slow) and print it as a ROUTES line
import assert from 'node:assert/strict';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { colliderDigest } from '../ui/js/desktop/apps/minigolf-collision.js';
import { createPhysics } from '../ui/js/desktop/apps/minigolf-physics.js';
import { findRoute, loadCourses, replayRoute, timedRoute } from './lib/minigolf-routes.mjs';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const dir = path.resolve(root, process.env.AURAGO_MINIGOLF_DIR || 'ui/3d/minigolf');
const partial = process.env.AURAGO_MINIGOLF_PARTIAL === '1';
const search = process.env.AURAGO_MINIGOLF_SEARCH === '1';
const only = process.env.AURAGO_MINIGOLF_COURSE || '';

const WORLDS = [['pirate', 1], ['temple', 7], ['moon', 13]];
const COURSE_COUNT = 18;
const idOf = number => `${WORLDS.filter(([, first]) => number >= first).pop()[0]}-${String(number).padStart(2, '0')}`;
const EXPECTED_IDS = Array.from({ length: COURSE_COUNT }, (_, i) => idOf(i + 1));
const FRAME = 1 / 60;

const close = (actual, expected, message) => assert.ok(Math.abs(actual - expected) <= 1e-5, `${message}: ${actual} vs ${expected}`);

// Recorded playable routes (one shot list per course; `wait` seconds of obstacle time before a shot). The shots of a
// route end resting (`rest`) except the last, which ends in the cup. A route needs at most par + 1 shots.
// `AURAGO_MINIGOLF_SEARCH=1` searches new ones against the exported geometry.
const ROUTES = {
    'pirate-01': [{"angle":1.570796,"power":0.55}],
    'pirate-02': [{"angle":1.10929,"power":1},{"angle":-1.180712,"power":0.15}],
    'pirate-03': [{"angle":0.523599,"power":1},{"angle":2.48157,"power":0.45}],
    'pirate-04': [{"angle":2.515388,"power":0.35},{"angle":0.992245,"power":0.15}],
    'pirate-05': [{"angle":2.20372,"power":1},{"angle":0.602041,"power":0.925,"wait":1.158333}],
    'pirate-06': [{"angle":2.617994,"power":1},{"angle":0.957123,"power":1}],
    'temple-07': [{"angle":1.5707963267948966,"power":0.85},{"angle":-1.6137465162646119,"power":0.15}],
    'temple-08': [{"angle":2.1650068427144227,"power":0.75},{"angle":-1.2897519591388917,"power":0.15}],
    'temple-09': [{"angle":-1.7540405989103272,"power":1},{"angle":-1.2026723566539541,"power":0.15}],
    'temple-10': [{"angle":1.832595714594046,"power":1},{"angle":-2.596476703040247,"power":0.15}],
    'temple-11': [{"angle":1.2109371888779847,"power":0.85},{"angle":1.1050123847470181,"power":0.15}],
    'temple-12': [{"angle":1.6929693744344996,"power":1},{"angle":2.4251973732384804,"power":0.15}],
    // moon routes: recorded in the switch
};

const { manifest, courses: all } = await loadCourses(dir);
assert.equal(manifest.version, 2, 'manifest version');

// Course set: all 18 (6 pirate, 6 temple, 6 moon) or, with AURAGO_MINIGOLF_PARTIAL=1, any ascending subset.
const ids = all.map(course => course.id);
if (partial) {
    assert.ok(all.length >= 1 && all.length <= COURSE_COUNT, `partial manifest has ${all.length} courses`);
    all.forEach((course, index) => assert.equal(course.id, idOf(course.number), `course ${index + 1}: id`));
    assert.deepEqual(all.map(course => course.number), [...new Set(all.map(course => course.number))].sort((a, b) => a - b), 'course numbers ascend');
} else {
    assert.equal(all.length, COURSE_COUNT, 'manifest has 18 courses');
    assert.deepEqual(ids, EXPECTED_IDS, 'course ids');
    assert.deepEqual(WORLDS.map(([world]) => all.filter(course => course.world === world).length), [6, 6, 6], 'six courses per world');
}
all.forEach(course => assert.equal(course.world, course.id.split('-')[0], `${course.id}: world`));
const selected = only ? all.filter(course => course.id === only || String(course.number) === only) : all;
assert.ok(selected.length > 0, `no course matches AURAGO_MINIGOLF_COURSE=${only} (${ids.join(', ')})`);

let routed = 0;
const skipped = [];
for (const course of selected) {
    // The collision comes from the GLB and equals the manifest fingerprint.
    assert.ok(course.colliders.some(collider => collider.type === 'mesh' && collider.surface === 'grass'), `${course.id}: grass collision mesh`);
    assert.equal(await colliderDigest(course.colliders), course.collision.digest, `${course.id}: collision digest of the GLB equals the manifest`);

    const events = [];
    const physics = await createPhysics(course, { onEvent: event => events.push(event) });
    try {
        physics.getBall().forEach((value, axis) => close(value, course.tee[axis], `${course.id}: tee position`));
        const initialObstacles = physics.getObstacles();

        // A gentle tee shot along the first metre of the route moves the ball across the grass and stays in play.
        const lead = course.path.find(point => Math.hypot(point[0] - course.tee[0], point[2] - course.tee[2]) >= 1) ?? course.cup;
        const direction = [lead[0] - course.tee[0], lead[2] - course.tee[2]];
        const length = Math.hypot(...direction);
        assert.ok(length > 0, `${course.id}: tee and route differ`);
        assert.equal(physics.shoot([direction[0] / length, direction[1] / length], 0.12), true, `${course.id}: accepts putt`);
        for (let i = 0; i < 6; i++) physics.step(FRAME);
        const position = physics.getBall();
        assert.ok(position.every(Number.isFinite), `${course.id}: finite ball pose`);
        assert.ok(position.every((value, axis) => value >= course.bounds.min[axis] && value <= course.bounds.max[axis]), `${course.id}: ball remains in bounds during tee shot`);
        assert.ok(Math.hypot(position[0] - course.tee[0], position[2] - course.tee[2]) > 0.025, `${course.id}: putt moves across the course mesh`);
        assert.ok(events.some(event => event.type === 'impact' && event.surface === 'grass'), `${course.id}: Rapier ball contacts the exported grass mesh`);
        assert.equal(events.some(event => ['cup', 'water', 'out', 'rest'].includes(event.type)), false, `${course.id}: gentle tee shot remains in play`);

        // Obstacles follow the physics time: a fraction of the cycle that no obstacle period divides evenly.
        const target = Math.max(0.1, (course.cycle || 1) * 0.137);
        while (physics.getSnapshot().time < target) physics.step(FRAME);
        const obstacles = physics.getObstacles();
        assert.equal(obstacles.length, course.obstacles.length, `${course.id}: obstacle list`);
        obstacles.forEach((obstacle, index) => {
            const initial = initialObstacles[index];
            const moved = obstacle.position.some((value, axis) => Math.abs(value - initial.position[axis]) > 1e-4)
                || obstacle.rotation.some((value, axis) => Math.abs(value - initial.rotation[axis]) > 1e-4);
            assert.ok(moved, `${course.id}: obstacle ${obstacle.id} advances with physics time`);
        });

        // The route: every shot ends resting except the last, which ends in the cup; at most par + 1 shots.
        let route = ROUTES[course.id];
        if (search) route = timedRoute(physics, course, findRoute(physics, course, events), events);
        else if (!route && partial) { skipped.push(course.id); continue; }
        assert.ok(route, `${course.id}: ${search ? 'the search finds a playable route to the cup' : 'no recorded route in ROUTES'}`);
        assert.ok(route.length >= 1 && route.length <= 12, `${course.id}: route respects the game's stroke cap`);
        assert.ok(route.length <= course.par + 1, `${course.id}: route needs ${route.length} shots, par ${course.par} allows at most ${course.par + 1}`);
        const replay = replayRoute(physics, course, route, events);
        assert.ok(replay.ok, `${course.id}: shot ${replay.failed ? replay.failed.index + 1 : '?'} should end '${replay.failed?.expected}' but ends '${replay.failed?.got}'`);
        assert.equal(replay.shots, route.length, `${course.id}: every shot of the route is played`);
        routed++;
        const waits = route.filter(shot => shot.wait).length;
        if (search) console.log(`    '${course.id}': ${JSON.stringify(route)},`);
        console.log(`${course.id}: ${search ? 'found' : 'recorded'} route reaches the cup in ${route.length} shot${route.length === 1 ? '' : 's'} (par ${course.par})${waits ? `, ${waits} wait${waits === 1 ? '' : 's'} for moving parts` : ''}`);
    } finally {
        physics.dispose();
    }
}
if (skipped.length) console.log(`note: no recorded route yet for ${skipped.join(', ')} (partial run, not played)`);
console.log(`minigolf courses: ${selected.length} of ${all.length} manifest courses loaded from the GLB collision, accepted tee shots and ${routed} routes played to the cup${partial ? ' (partial)' : ''}`);
