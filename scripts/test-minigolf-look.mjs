import assert from 'node:assert/strict';
import { LOOKS, LOOK_IDS, TIERS, lookOf, placementMatrix, tierConfig } from '../ui/js/desktop/apps/minigolf-looks.js';
import { createIntro, introDuration, overviewFor, pathLength } from '../ui/js/desktop/apps/minigolf-camera.js';

const close = (actual, expected, message, eps = 1e-6) => expected.forEach((value, i) => assert.ok(Math.abs(actual[i] - value) <= eps, `${message}[${i}]: ${actual[i]} != ${value}`));

// Every look carries the data the renderer reads.
assert.deepEqual(LOOK_IDS, ['pirate', 'temple', 'moon']);
for (const id of LOOK_IDS) {
    const look = LOOKS[id];
    for (const key of ['exposure', 'background', 'sky', 'sun', 'environment', 'bloom', 'water', 'fx']) assert.ok(look[key] !== undefined, `${id}.${key}`);
    assert.equal(look.sun.direction.length, 3, `${id} sun direction`);
    assert.ok(look.fx.confetti.length >= 4, `${id} confetti colours`);
    assert.ok(['lagoon', 'jungle', 'void'].includes(look.water.mode), `${id} water mode`);
}
assert.equal(LOOKS.moon.water.mode, 'void');
assert.equal(LOOKS.moon.fog, null);

// Legacy worlds map onto the new looks until P3 renames them.
assert.equal(lookOf({ world: 'coast' }), 'pirate');
assert.equal(lookOf({ world: 'garden' }), 'temple');
assert.equal(lookOf({ world: 'night' }), 'moon');
assert.equal(lookOf({ world: 'coast', look: 'moon' }), 'moon');
assert.equal(lookOf({ world: 'pirate' }), 'pirate');
assert.equal(lookOf(null), 'pirate');

// Quality tiers.
assert.deepEqual(['low', 'medium', 'high'].map(tier => TIERS[tier].particles), [60, 160, 320]);
assert.equal(TIERS.low.scatter, false);
assert.equal(TIERS.low.shadows, 0);
assert.equal(TIERS.high.shadows, 2048);
assert.equal(tierConfig('nonsense'), TIERS.medium);

// Placement [name, x, y, z, yaw, scale] -> column-major matrix (yaw about +Y).
const m = placementMatrix(['barrel', 1, 2, 3, Math.PI / 2, 2]);
const apply = (p) => [0, 1, 2].map(r => m[r] * p[0] + m[4 + r] * p[1] + m[8 + r] * p[2] + m[12 + r]);
close(apply([1, 0, 0]), [1, 2, 1], 'yaw +90 deg maps +X to -Z');
close(apply([0, 1, 0]), [1, 4, 3], 'uniform scale on Y');

// Intro flight.
const straight = Array.from({ length: 11 }, (_, i) => [0, .16, -i]);
assert.equal(pathLength(straight), 10);
assert.equal(introDuration(straight), 3.5);
assert.equal(introDuration(straight.slice(0, 2)), 2.5);
assert.equal(introDuration(Array.from({ length: 31 }, (_, i) => [0, 0, -i])), 4.5);
assert.equal(createIntro([[0, 0, 0]], [0, 0, 0], [0, 0, 0]), null);
const end = [10, 13, 9], target = [0, 0, -5];
const intro = createIntro(straight, end, target);
const position = [], look = [];
assert.equal(intro.sample(0, position, look), false);
close(position, [0, 3.36, 2.2], 'start behind and above the tee');
close(look, [0, .16, -2.5], 'start looks 2.5 m ahead');
intro.sample(intro.duration * .3, position, look);
assert.ok(position[1] > 3 && position[2] < 0, 'phase A flies along the lane');
assert.equal(intro.sample(intro.duration, position, look), true);
close(position, end, 'ends in the overview pose');
close(look, target, 'ends on the overview target');
// Overview framing follows the island size; small islands keep the fixed P2 pose.
{
    const tee = [0, .16, 4.45], cup = [0, 0, -3.86];
    const small = overviewFor({ island: { center: [0, -.1], half: [4.9, 5.35] }, tee, cup });
    close(small.target, [0, .08, .295], 'small island looks at the tee/cup midpoint');
    close(small.position, [10, .08 + 13, .295 + 14], 'small island keeps the old (10, 13, 14) offset');
    assert.equal(small.shadowHalf, 9);
    assert.equal(small.waterRadius, 60);
    const none = overviewFor({ tee, cup });
    close(none.position, small.position, 'no island keeps the old pose');
    assert.equal(none.shadowHalf, 9);
    const wide = overviewFor({ island: { center: [2, -1], half: [16, 9] }, tee: [0, .16, 0], cup: [0, 1.16, 3] });
    close(wide.target, [2, .66, -1], 'large island looks at the island centre at mean tee/cup height');
    const offset = wide.position.map((value, i) => value - wide.target[i]), distance = Math.hypot(...offset);
    assert.ok(Math.abs(distance - 16 * 3.4) < 1e-9, `large island distance ${distance}`);
    close(offset.map(value => value / distance), [10, 13, 14].map(value => value / Math.hypot(10, 13, 14)), 'large island keeps the view direction');
    assert.ok(wide.shadowHalf >= 17.5 && wide.waterRadius === 80);
    assert.ok(Math.hypot(...overviewFor({ island: { center: [0, 0], half: [20, 20] }, tee, cup }).position.map((v, i) => v - [0, .08, 0][i])) <= 60 + 1e-9, 'distance is capped at 60 m');
}
// A large island is framed by its playable area (path and every corridor plus 2.5 m), not by the whole island.
{
    const distanceOf = pose => Math.hypot(...pose.position.map((value, i) => value - pose.target[i]));
    const island = { center: [10, 4], half: [17, 14.7], power: 3, wobble: .015 };
    const main = [[0, .16, 0], [12, .16, 0], [22, -.4, 12]], branch = [[12, .16, 0], [14, .16, 12], [22, -.4, 12]];
    const wide = overviewFor({ island, tee: main[0], cup: main[2], path: main, corridors: [main, branch] });
    // Box x 0..22, z 0..12 -> centre (11, 6), half extent 11 + 2.5.
    close(wide.target, [11, -.12, 6], 'target is the centre of the playable box at the mean tee/cup height');
    assert.ok(Math.abs(distanceOf(wide) - 13.5 * 3.4) < 1e-9, `framed by the playable box: ${distanceOf(wide)}`);
    close(wide.position.map((value, i) => value - wide.target[i]).map(value => value / distanceOf(wide)), [10, 13, 14].map(value => value / Math.hypot(10, 13, 14)), 'playable framing keeps the view direction');
    const whole = overviewFor({ island, tee: main[0], cup: main[2] });
    assert.ok(distanceOf(wide) < distanceOf(whole), 'the playable box is closer than the whole island');
    // Shadow camera and water still cover the island.
    assert.equal(wide.shadowHalf, whole.shadowHalf);
    assert.equal(wide.waterRadius, whole.waterRadius);
    assert.ok(wide.shadowHalf >= 17 + 1.5 && wide.waterRadius === 85);
    // A branch outside the main route widens the box.
    const longBranch = overviewFor({ island, tee: main[0], cup: main[2], path: main, corridors: [main, [[12, .16, 0], [12, .16, 25], [22, -.4, 12]]] });
    assert.ok(Math.abs(distanceOf(longBranch) - (12.5 + 2.5) * 3.4) < 1e-9, 'a corridor counts like the main route');
    // A tiny playable box never gets closer than the P2 distance; routes without usable points fall back to the island.
    const small = overviewFor({ island, tee: [0, 0, 0], cup: [3, 0, 0], path: [[0, 0, 0], [3, 0, 0]] });
    assert.ok(Math.abs(distanceOf(small) - 21.6) < 1e-9, 'minimum distance is 21.6 m (the P2 view distance)');
    close(overviewFor({ island, tee: main[0], cup: main[2], path: [[NaN, 0, NaN]] }).target, whole.target, 'unusable routes fall back to the island framing');
    // Small islands keep the P2 pose even with routes.
    const legacy = overviewFor({ island: { center: [0, -.1], half: [4.9, 5.35] }, tee: [0, .16, 4.45], cup: [0, 0, -3.86], path: [[0, .16, 4.45], [0, 0, -3.86]] });
    close(legacy.position, [10, .08 + 13, .295 + 14], 'small island keeps the P2 pose with a path');
}
// The fog thins out with the view distance of large courses and stays untouched for the P2 pose.
{
    const tee = [0, .16, 4.45], cup = [0, 0, -3.86];
    assert.equal(overviewFor({ island: { center: [0, 0], half: [4.9, 5.35] }, tee, cup }).fogScale, 1);
    assert.equal(overviewFor({ tee, cup }).fogScale, 1);
    const island = { center: [0, 0], half: [17, 15] }, route = [[0, .16, 0], [24, .16, 12]];
    const pose = overviewFor({ island, tee: route[0], cup: route[1], path: route });
    const distance = Math.hypot(...pose.position.map((value, i) => value - pose.target[i]));
    assert.ok(pose.fogScale < 1 && Math.abs(pose.fogScale * distance - Math.hypot(10, 13, 14)) < 1e-9, 'density x distance stays at the P2 value');
    for (const id of ['pirate', 'temple']) {
        const fogAtLane = scale => 1 - Math.exp(-((LOOKS[id].fog.density * scale * distance) ** 2));
        assert.ok(fogAtLane(pose.fogScale) <= fogAtLane(1) / 2, `${id}: fog over the lane at ${distance.toFixed(0)} m`);
        assert.ok(Math.abs(fogAtLane(pose.fogScale) - (1 - Math.exp(-((LOOKS[id].fog.density * Math.hypot(10, 13, 14)) ** 2)))) < 1e-9, `${id}: lane fog equals the P2 overview`);
    }
}

console.log('AuraGolf look tests: ok');
