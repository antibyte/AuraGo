import assert from 'node:assert/strict';
import { MAX_NAME_LENGTH, PALETTE, DEFAULT_COLORS, FLOOR_SURFACES, colorValue, assignColors, cleanName, uniqueNames, worldIds, formatToPar, powerHex, createQualityGovernor } from '../ui/js/desktop/apps/minigolf-shared.js';
import { formatPower, placeMeter } from '../ui/js/desktop/apps/minigolf-hud.js';
import { laneGeometry, obstacleOutlines } from '../ui/js/desktop/apps/minigolf-minimap.js';

const close = (actual, expected, tolerance, message) => assert.ok(Math.abs(actual - expected) <= tolerance, `${message}: ${actual} vs ${expected}`);

// --- Shared palette, names and helpers (Task 1) -------------------------------
assert.equal(MAX_NAME_LENGTH, 24);
assert.equal(PALETTE.length, 8);
assert.equal(new Set(PALETTE.map(color => color.id)).size, 8, 'unique colour ids');
assert.equal(new Set(PALETTE.map(color => color.hex)).size, 8, 'unique colour values');
assert.deepEqual(DEFAULT_COLORS, ['cream', 'coral', 'sky', 'lilac']);
assert.equal(colorValue('cream'), '#fff0c5', 'player 1 keeps the HUD cream everywhere');
assert.equal(colorValue('teal'), PALETTE[7].hex);
assert.equal(colorValue('unknown'), '#fff0c5');
assert.deepEqual(assignColors(['coral', undefined]), ['coral', 'cream'], 'a new player whose default is taken gets the first free colour');
assert.deepEqual(assignColors(['teal', 'teal', 'x']), ['teal', 'coral', 'sky']);
assert.equal(cleanName('  Ada\x07 ', 'Player 1'), 'Ada');
assert.equal(cleanName('\x00\x1f  ', 'Player 2'), 'Player 2');
assert.equal(cleanName('x'.repeat(30)), 'x'.repeat(24));
assert.equal(Array.from(cleanName(String.fromCodePoint(0x1f600).repeat(30))).length, 24, 'cuts code points');
assert.equal(cleanName('Ada' + ' '.repeat(21) + 'Lovelace'), 'Ada', 'trims again after cutting');
assert.deepEqual(uniqueNames(['Ada 2', 'Ada', 'ada']), ['Ada 2', 'Ada', 'ada 3']);
assert.deepEqual(uniqueNames(['y'.repeat(24), 'y'.repeat(24)]), ['y'.repeat(24), 'y'.repeat(22) + ' 2'], 'suffix stays within 24 code points');
assert.deepEqual(worldIds([{ world: 'garden' }, { world: 'coast' }, { world: 'garden' }, { world: 'Bad' }]), ['garden', 'coast']);
assert.ok(FLOOR_SURFACES.has('grass') && FLOOR_SURFACES.has('sand') && !FLOOR_SURFACES.has('wood'));
assert.equal(formatToPar(0, 'Par'), 'Par');
assert.equal(formatToPar(-2, 'Par'), String.fromCharCode(0x2212) + '2');
assert.equal(formatToPar(3, 'Par'), '+3');
assert.equal(powerHex(0), '#7fe0a6');
assert.equal(powerHex(0.25), '#bfdc98');
assert.equal(powerHex(0.5), '#ffd789');
assert.equal(powerHex(1), '#ff8f77');

// Auto quality: medium first, low below 38 FPS after 6 s, back after ~10 s >= 55 FPS and 12 s.
const governor = createQualityGovernor();
let clock = 0;
const feed = (fps, seconds) => {
    const changes = [], end = clock + seconds;
    while (clock < end - 1e-9) { clock += 1 / fps; const change = governor.frame(1 / fps, clock); if (change) changes.push({ change, at: clock }); }
    return changes;
};
assert.equal(governor.tier, 'medium', 'auto starts at medium');
const slow = feed(30, 8);
assert.deepEqual(slow.map(entry => entry.change), ['down']);
assert.ok(slow[0].at > 6, `no downgrade within 6 s (${slow[0].at.toFixed(2)} s)`);
assert.deepEqual(feed(60, 10), [], 'no upgrade within 12 s of the downgrade');
const fast = feed(60, 4);
assert.deepEqual(fast.map(entry => entry.change), ['up']);
assert.ok(fast[0].at - slow[0].at >= 12, 'upgrade waits 12 s');
assert.equal(governor.tier, 'medium');

// --- HUD formats (Task 5) -----------------------------------------------------
const nb = '\xa0';
assert.equal(formatPower(0.54, 11.2, 'de').percent, '54' + nb + '%');
assert.equal(formatPower(0.54, 11.2, 'en').percent, '54%');
assert.equal(formatPower(0.54, 11.2, 'de').text, '54' + nb + '% · ≈' + nb + '11' + nb + 'm');
assert.equal(formatPower(0.3, 0.42, 'de').distance, '≈' + nb + '0,4' + nb + 'm', 'below 1 m: 0.1 m steps');
assert.equal(formatPower(0.3, 7.3, 'de').distance, '≈' + nb + '7,5' + nb + 'm', 'below 10 m: 0.5 m steps');
assert.equal(formatPower(0.3, 7.3, 'en').distance, '≈' + nb + '7.5' + nb + 'm');
assert.equal(formatPower(0.3, NaN, 'en').text, '30%');
assert.equal(formatPower(0.3, 2, 'not a tag!').percent, '30%', 'invalid language tags fall back to English');
assert.deepEqual(placeMeter({ x: 100, y: 200, visible: true }, 800, 600, 96), { x: 108, y: 88 });
assert.deepEqual(placeMeter({ x: 780, y: 200, visible: true }, 800, 600, 96), { x: 676, y: 88 }, 'mirrors left at the right edge');
assert.deepEqual(placeMeter({ x: 100, y: 40, visible: true }, 800, 600, 96), { x: 108, y: 56 }, 'mirrors below at the top edge');
assert.deepEqual(placeMeter({ visible: false }, 800, 600, 96), { x: 352, y: 492 }, 'hidden ball: bottom centre');

// --- Lane map geometry (Task 6) ----------------------------------------------
const lane = laneGeometry({
    bounds: { min: [-5, -1, -5], max: [5, 2, 5] },
    water: [{ min: [2, -1, -1], max: [3, 0, 1] }],
    colliders: [
        { type: 'mesh', surface: 'grass', vertices: [-1, 0, -3, 1, 0, -3, 1, 0, 3, -1, 0, 3], indices: [0, 1, 2, 0, 3, 2] },
        { type: 'mesh', surface: 'sand', vertices: [1, 0, -1, 2, 0, -1, 2, 0, 1], indices: [0, 2, 1] },
        { type: 'mesh', surface: 'wood', vertices: [-1.2, 0.3, -3, -1, 0.3, -3, -1, 0.3, 3, -1.2, 0.3, 3, -1.2, 0, -3], indices: [0, 1, 2, 0, 2, 3, 4, 0, 3] }
    ]
});
assert.equal(lane.floor.grass.length, 12, 'both grass triangles count regardless of winding');
assert.equal(lane.floor.sand.length, 6);
assert.equal(lane.walls.length, 12, 'rail tops count, the vertical rail side does not');
assert.deepEqual(lane.frame, { min: [-1.5, -3.5], max: [2.5, 3.5] }, 'floor bounds plus 0.5 m');
assert.deepEqual(lane.water, [{ min: [2, -1], max: [3, 1] }]);
assert.deepEqual(laneGeometry({ bounds: { min: [-5, -1, -4], max: [5, 2, 4] } }).frame, { min: [-5, -4], max: [5, 4] }, 'no floor falls back to bounds');
const area = (list, at) => ((list[at + 2] - list[at]) * (list[at + 5] - list[at + 1]) - (list[at + 4] - list[at]) * (list[at + 3] - list[at + 1])) / 2;
assert.equal(new Set([area(lane.floor.grass, 0), area(lane.floor.grass, 6), area(lane.floor.sand, 0)].map(Math.sign)).size, 1, 'floor triangles share one winding, so one fill never cancels overlaps');
const gate = { obstacles: [{ id: 'gate', position: [0, 0.3, 0], halfExtents: [1, 0.1, 1], colliders: [{ position: [0, 0, 0], halfExtents: [1, 0.1, 0.2] }, { position: [0, 0, 0], halfExtents: [0.2, 0.1, 1] }] }] };
const xs = outline => outline.filter((_, index) => index % 2 === 0), zs = outline => outline.filter((_, index) => index % 2 === 1);
const still = obstacleOutlines(gate);
assert.equal(still.length, 2, 'one outline per obstacle part');
close(Math.max(...xs(still[0])), 1, 1e-9, 'unrotated bar width'); close(Math.max(...zs(still[0])), 0.2, 1e-9, 'unrotated bar depth');
const turned = obstacleOutlines(gate, [{ id: 'gate', position: [2, 0.3, 0], rotation: [0, Math.SQRT1_2, 0, Math.SQRT1_2] }]);
close(Math.max(...xs(turned[0])), 2.2, 1e-9, 'physics pose turns and moves the bar'); close(Math.max(...zs(turned[0])), 1, 1e-9, 'turned bar depth');
close(Math.min(...xs(obstacleOutlines({ obstacles: [{ id: 'box', position: [2, 0, 0], halfExtents: [0.5, 0.1, 0.5] }] })[0])), 1.5, 1e-9, 'single-box obstacle');

console.log('minigolf ui: all checks passed');
