import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import * as THREE from 'three';
import { createNavigator } from '../ui/js/desktop/apps/sysworld-navigation.js';
import { createTraffic } from '../ui/js/desktop/apps/sysworld-traffic.js';
import { createSociety } from '../ui/js/desktop/apps/sysworld-society.js';

// Residents must not jitter: a short blockage is waited out, a street that stays
// blocked still turns the robot around, a blocked resident keeps facing its goal, a
// resident that cannot pass waits instead of pacing, and a standing robot keeps
// still instead of walking on the spot.
{
    // A short blockage must not turn the resident around: it waits and drives on.
    const square = [[0, 0], [60, 0], [60, 60], [0, 60]];
    let blocked = true;
    const nav = createNavigator({ routes: [square], obstacles: [], blocked: () => blocked });
    for (let i = 0; i < 90; i++) nav.step(1 / 60);
    blocked = false;
    for (let i = 0; i < 1800; i++) nav.step(1 / 60);
    assert.equal(nav.stats().turns, 0, 'A brief blockage must not turn the resident around');
    assert.ok(nav.agents[0].speed > 1, 'The resident drives on once the way is free');
}
{
    // A street that stays blocked still deserves the U-turn.
    const square = [[0, 0], [60, 0], [60, 60], [0, 60]];
    const nav = createNavigator({ routes: [square], obstacles: [{ x: 30, z: 2.2, r: 7 }] });
    for (let i = 0; i < 3000; i++) nav.step(1 / 30);
    assert.ok(nav.stats().turns > 0, 'A blocked street forces a U-turn');
}
{
    // A resident whose way is blocked steps aside but keeps facing its goal: a
    // resident that swings its heading left and right walks on the spot.
    const world = createTraffic(), scene = new THREE.Scene(), camera = new THREE.PerspectiveCamera();
    const society = createSociety(scene, { traffic: world, camera, floor: () => 0, active: () => true });
    const resident = society.add({ node: new THREE.Group(), play() { } }, 0, { min: [-.3, 0, -.3], max: [.3, 2, .3] });
    const start = { x: resident.body.x, z: resident.body.z };
    camera.position.set(start.x, 2, start.z);
    society.inspect(resident.id);
    society.action('guide', { x: start.x, z: start.z + 14 });
    // A narrow post right in front: the way ahead is blocked, both sides are free.
    world.solid('post', { owner: 'post', x: start.x, z: start.z + 1.5, min: [-.3, 0, -.3], max: [.3, 3, .3] });
    let worst = 0, worstAt = null;
    for (let i = 0; i < 600; i++) {
        camera.position.set(resident.body.x, 2, resident.body.z);
        world.begin(); society.update(1 / 60, true); world.solve(1 / 60);
        const target = resident.path[0];
        if (!target) continue;
        const goal = Math.atan2(target.x - resident.body.x, target.z - resident.body.z);
        const deviation = Math.abs(Math.atan2(Math.sin(resident.body.heading - goal), Math.cos(resident.body.heading - goal)));
        if (deviation > worst) { worst = deviation; worstAt = { i, state: resident.state, heading: resident.body.heading, goal, pos: [resident.body.x, resident.body.z], way: [target.x, target.z] }; }
    }
    if (process.env.JITTER_DEBUG) console.log('worst at', JSON.stringify(worstAt));
    assert.ok(worst < .6, `A blocked resident keeps facing its goal (worst deviation ${worst.toFixed(2)} rad)`);
    society.dispose(); world.dispose();
}
{
    // A wide obstacle that no sidestep can clear must not turn the resident into a
    // circle: it stands still until the street opens again.
    const world = createTraffic(), scene = new THREE.Scene(), camera = new THREE.PerspectiveCamera();
    const society = createSociety(scene, { traffic: world, camera, floor: () => 0, active: () => true });
    const resident = society.add({ node: new THREE.Group(), play() { } }, 0, { min: [-.3, 0, -.3], max: [.3, 2, .3] });
    const start = { x: resident.body.x, z: resident.body.z };
    camera.position.set(start.x, 2, start.z);
    society.inspect(resident.id);
    society.action('guide', { x: start.x, z: start.z + 14 });
    // A wide wall across the street: stepping aside does not open the way.
    world.solid('wall', { owner: 'wall', x: start.x, z: start.z + 2, min: [-6, 0, -.4], max: [6, 3, .4] });
    let waiting = 0, reversals = 0, last = null, previous = null;
    for (let i = 0; i < 900; i++) {
        camera.position.set(resident.body.x, 2, resident.body.z);
        world.begin(); society.update(1 / 60, true); world.solve(1 / 60);
        if (previous) {
            const dx = resident.body.x - previous.x, dz = resident.body.z - previous.z, d = Math.hypot(dx, dz);
            // The first two and a half seconds the wall is impassable: the resident
            // waits instead of shuffling back and forth in front of it.
            if (i < 150) waiting += d;
            if (d > .01) {
                if (last && Math.abs(Math.atan2(last.x * dz - last.z * dx, last.x * dx + last.z * dz)) > 1.05) reversals++;
                last = { x: dx, z: dz };
            }
        }
        previous = { x: resident.body.x, z: resident.body.z };
    }
    assert.ok(waiting < .2, `A resident that cannot pass waits instead of jittering (${waiting.toFixed(2)} m)`);
    assert.ok(reversals < 20, `A resident that cannot pass waits instead of pacing (${reversals} direction changes)`);
    society.dispose(); world.dispose();
}
{
    // The white residents stand still instead of bobbing as if they walked.
    const bytes = await fs.readFile('ui/3d/system-world/white-robot.glb');
    const jsonLength = bytes.readUInt32LE(12);
    const doc = JSON.parse(bytes.subarray(20, 20 + jsonLength));
    for (const key of ['images', 'textures', 'materials', 'samplers']) delete doc[key];
    for (const mesh of doc.meshes) for (const primitive of mesh.primitives) delete primitive.material;
    const jsonBytes = Buffer.from(JSON.stringify(doc));
    const padded = Buffer.concat([jsonBytes, Buffer.alloc((4 - jsonBytes.length % 4) % 4, 32)]);
    const header = Buffer.from(bytes.subarray(0, 20));
    const geometry = Buffer.concat([header, padded, bytes.subarray(20 + jsonLength)]);
    geometry.writeUInt32LE(geometry.length, 8); geometry.writeUInt32LE(padded.length, 12);
    const nativeFetch = globalThis.fetch;
    globalThis.fetch = async () => new Response(geometry);
    const { createCityLife } = await import('../ui/js/desktop/apps/sysworld-life.js');
    const { obstacles } = await import('../ui/js/desktop/apps/sysworld-scene.js');
    // A parked tram on the first street holds the residents back instead of letting
    // them drive through: the standing resident is the one that must keep still.
    const world = createTraffic();
    const tram = world.register('tram-0', { circles: [{ x: 0, z: 0, r: 2.6 }], minY: 0, maxY: 3.4, reach: 2.6 }, { x: -40, y: 0, z: -77, heading: 0 }, 3);
    assert.ok(tram, 'The blocking tram registers');
    const scene = new THREE.Scene();
    const life = createCityLife(scene, [], { robotURL: 'white-robot.glb', active: () => true, obstacles, traffic: world });
    globalThis.fetch = nativeFetch;
    try {
        for (let i = 0; i < 200 && life.stats().robots !== 5 && !life.stats().robotError; i++) await new Promise(r => setTimeout(r, 10));
        assert.equal(life.stats().robots, 5, 'All five residents load');
        const residents = scene.getObjectByName('city-life').children.filter(n => n.name.startsWith('city-white-robot-'));
        let stillFrames = 0, bob = 0;
        const low = new Map(), high = new Map(), history = residents.map(() => []);
        for (let frame = 0; frame < 3600; frame++) {
            world.begin(); life.update(1 / 30, true); world.solve(1 / 60);
            residents.forEach((resident, i) => {
                const trail = history[i];
                trail.push({ x: resident.position.x, z: resident.position.z, y: resident.position.y });
                if (trail.length > 61) trail.shift();
                if (trail.length < 61) return;
                // A robot stands still when it covered less than ten centimetres in the
                // last two seconds. The long window keeps the frames in which a robot
                // rolls away out of the measurement, and the travel is horizontal, so
                // the bob itself is never mistaken for a step.
                const first = trail[0];
                const travel = Math.hypot(resident.position.x - first.x, resident.position.z - first.z);
                if (travel > .1) return;
                // A resident that stands still hovers quietly; the walk bob would double the range.
                stillFrames++;
                low.set(i, Math.min(low.get(i) ?? Infinity, resident.position.y));
                high.set(i, Math.max(high.get(i) ?? -Infinity, resident.position.y));
            });
        }
        for (const [i, top] of high) bob = Math.max(bob, top - (low.get(i) ?? top));
        assert.ok(stillFrames > 100, `The simulation actually holds residents back (${stillFrames} still frames)`);
        assert.ok(bob < .09, `A standing resident keeps still (worst bob ${bob.toFixed(3)} m)`);
    } finally { life.dispose(); world.dispose(); }
}
console.log('System World: residents wait out short blockages, keep facing their goal and stand still instead of jittering.');
