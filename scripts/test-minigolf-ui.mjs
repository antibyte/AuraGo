import assert from 'node:assert/strict';
import { MAX_NAME_LENGTH, PALETTE, DEFAULT_COLORS, FLOOR_SURFACES, colorValue, assignColors, cleanName, uniqueNames, worldIds, formatToPar, powerHex, createQualityGovernor } from '../ui/js/desktop/apps/minigolf-shared.js';

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

console.log('minigolf ui: all checks passed');
