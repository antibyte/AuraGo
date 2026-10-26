import assert from 'node:assert/strict';
import fs from 'node:fs';
import { MAX_NAME_LENGTH, PALETTE, DEFAULT_COLORS, FLOOR_SURFACES, colorValue, assignColors, cleanName, uniqueNames, worldIds, formatToPar, fillText, powerHex, createQualityGovernor } from '../ui/js/desktop/apps/minigolf-shared.js';
import { formatPower, placeMeter } from '../ui/js/desktop/apps/minigolf-hud.js';
import { laneGeometry, obstacleOutlines } from '../ui/js/desktop/apps/minigolf-minimap.js';
import { rankPlayers, relativeToPar } from '../ui/js/desktop/apps/minigolf-scorecard.js';
import { tutorialStart, tutorialNext, tutorialView } from '../ui/js/desktop/apps/minigolf-tutorial.js';

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
assert.deepEqual(worldIds([{ world: 'temple' }, { world: 'pirate' }, { world: 'temple' }, { world: 'Bad' }]), ['temple', 'pirate']);
assert.ok(FLOOR_SURFACES.has('grass') && FLOOR_SURFACES.has('sand') && !FLOOR_SURFACES.has('wood'));
assert.equal(formatToPar(0, 'Par'), 'Par');
assert.equal(formatToPar(-2, 'Par'), String.fromCharCode(0x2212) + '2');
assert.equal(formatToPar(3, 'Par'), '+3');
assert.equal(fillText('{a} / {b}', { a: '$&', b: 2 }), '$& / 2', 'placeholders fill literally, even with $ in a name');
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
const typedLane = laneGeometry({ bounds: { min: [-5, -1, -5], max: [5, 2, 5] }, colliders: [{ type: 'mesh', surface: 'grass', vertices: Float32Array.of(-1, 0, -3, 1, 0, -3, 1, 0, 3, -1, 0, 3), indices: Uint32Array.of(0, 1, 2, 0, 3, 2) }] });
assert.deepEqual(typedLane.floor.grass, lane.floor.grass, 'typed collider arrays give the same lane triangles');
const area = (list, at) => ((list[at + 2] - list[at]) * (list[at + 5] - list[at + 1]) - (list[at + 4] - list[at]) * (list[at + 3] - list[at + 1])) / 2;
assert.equal(new Set([area(lane.floor.grass, 0), area(lane.floor.grass, 6), area(lane.floor.sand, 0)].map(Math.sign)).size, 1, 'floor triangles share one winding, so one fill never cancels overlaps');
const gate = { obstacles: [{ id: 'gate', position: [0, 0.3, 0], halfExtents: [1, 0.1, 1], colliders: [{ position: [0, 0, 0], halfExtents: [1, 0.1, 0.2] }, { position: [0, 0, 0], halfExtents: [0.2, 0.1, 1] }] }] };
const xs = outline => outline.filter((_, index) => index % 2 === 0), zs = outline => outline.filter((_, index) => index % 2 === 1);
const still = obstacleOutlines(gate);
assert.equal(still.length, 2, 'one outline per obstacle part');
close(Math.max(...xs(still[0])), 1, 1e-9, 'unrotated bar width'); close(Math.max(...zs(still[0])), 0.2, 1e-9, 'unrotated bar depth');
const round = obstacleOutlines({ obstacles: [{ id: 'post', position: [1, 0.3, 2], colliders: [{ shape: 'ball', position: [0, 0, 0], radius: 0.5 }, { shape: 'cylinder', position: [1, 0, 0], radius: 0.25, halfHeight: 0.3 }] }] });
assert.equal(round.length, 2, 'ball and cylinder parts get an outline');
close(Math.max(...xs(round[0])), 1.5, 1e-9, 'ball outline radius'); close(Math.max(...zs(round[0])), 2.5, 1e-9, 'ball outline depth'); close(Math.max(...xs(round[1])), 2.25, 1e-9, 'cylinder outline centre and radius');
const turned = obstacleOutlines(gate, [{ id: 'gate', position: [2, 0.3, 0], rotation: [0, Math.SQRT1_2, 0, Math.SQRT1_2] }]);
close(Math.max(...xs(turned[0])), 2.2, 1e-9, 'physics pose turns and moves the bar'); close(Math.max(...zs(turned[0])), 1, 1e-9, 'turned bar depth');
close(Math.min(...xs(obstacleOutlines({ obstacles: [{ id: 'box', position: [2, 0, 0], halfExtents: [0.5, 0.1, 0.5] }] })[0])), 1.5, 1e-9, 'single-box obstacle');

// --- Ranking and par (Task 8) --------------------------------------------------
const ranking = rankPlayers([{ scores: [3, 4], strokes: 0 }, { scores: [2, 4], strokes: 0 }, { scores: [3, 3], strokes: 0 }, { scores: [5, 5], strokes: 0 }]);
assert.deepEqual(ranking.map(entry => [entry.index, entry.place, entry.total]), [[1, 1, 6], [2, 1, 6], [0, 3, 7], [3, 4, 10]], 'ties share a place (1, 1, 3)');
const parRound = { courseIds: ['a', 'b'], holeIndex: 1, completed: false }, parCourses = [{ id: 'a', par: 2 }, { id: 'b', par: 3 }];
assert.equal(relativeToPar({ scores: [3], strokes: 2, done: true }, parRound, parCourses), 0, 'a finished current hole counts');
assert.equal(relativeToPar({ scores: [3], strokes: 2, done: false }, parRound, parCourses), 1, 'an open hole does not count');
assert.equal(relativeToPar({ scores: [3, 2], strokes: 0, done: true }, { courseIds: ['a', 'b'], holeIndex: 2, completed: true }, parCourses), 0, 'a completed round counts every hole once');

// --- Tutorial state machine (Task 9) ----------------------------------------
let step = tutorialStart();
assert.equal(step.step, 'aim');
assert.equal(tutorialNext(step, { type: 'cancel' }), step, 'a cancelled drag keeps the step');
step = tutorialNext(step, { type: 'drag', strength: 0.05 }); assert.equal(step.step, 'power');
assert.equal(tutorialNext(step, { type: 'drag', strength: 0.1 }).step, 'power');
step = tutorialNext(step, { type: 'strength', strength: 0.25 }); assert.equal(step.step, 'release');
step = tutorialNext(step, { type: 'shot' }); assert.equal(step.step, 'watch');
assert.equal(tutorialNext(step, { type: 'panel' }).step, 'watch');
step = tutorialNext(step, { type: 'resolved' }); assert.equal(step.step, 'done');
assert.deepEqual(tutorialNext(step, { type: 'confirm' }), { step: null, finished: true });
assert.deepEqual(tutorialNext(step, { type: 'shot' }), { step: null, finished: true });
assert.deepEqual(tutorialNext(step, { type: 'panel' }), { step: null, finished: true });
assert.equal(tutorialNext(tutorialNext(tutorialStart(), { type: 'turn' }), { type: 'shot' }).step, 'watch', 'an early shot jumps to watch');
assert.deepEqual(tutorialNext(tutorialStart(), { type: 'skip' }), { step: null, finished: true });
const finished = tutorialNext(tutorialStart(), { type: 'skip' });
assert.equal(tutorialNext(finished, { type: 'shot' }), finished, 'a finished tutorial stays finished');
const powerStep = { step: 'power', finished: false };
assert.equal(tutorialView(powerStep, { canAim: true }).text, 'tutorial_power');
assert.equal(tutorialView(powerStep, { canAim: true, camera: true }).text, 'tutorial_camera', 'camera mode hides the aiming steps');
assert.equal(tutorialView(powerStep, { canAim: true, panel: true }), null);
assert.equal(tutorialView(powerStep, { canAim: true, ballVisible: false }).anchor, 'top');
assert.equal(tutorialView({ step: 'done', finished: false }, {}).anchor, 'help');
// Every text the tutorial shows has an English string.
const english = JSON.parse(fs.readFileSync(new URL('../ui/lang/desktop/en.json', import.meta.url), 'utf8'));
for (const name of ['aim', 'power', 'release', 'watch', 'done']) {
    const view = tutorialView({ step: name, finished: false }, { canAim: true });
    for (const key of [view.text, view.keys].filter(Boolean)) assert.ok(english['minigolf.' + key], 'missing en string minigolf.' + key);
}
assert.ok(english['minigolf.' + tutorialView(powerStep, { canAim: true, camera: true }).text], 'missing en string for the camera hint');

console.log('minigolf ui: all checks passed');
