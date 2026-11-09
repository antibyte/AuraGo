import assert from 'node:assert/strict';
import { CAMERA_REV, normalizeSettings, normalizeScores, DEFAULT_KEYS, DEFAULT_PAD, rngNext, TABLE_IDS } from '../ui/js/desktop/apps/pinball/config.js';

// Old settings object (no music/callouts/camera/start) migrates to the new defaults.
const old = { quality: 'medium', effects: 0.4, ambience: 0.1, reduced: true, keys: { left: 'KeyZ', right: 'KeyX' }, pad: { left: 4 } };
const s = normalizeSettings(old);
assert.equal(s.music, 0.5); assert.equal(s.callouts, true); assert.equal(s.camera, 'follow');
assert.equal(s.keys.start, 'Digit1'); assert.equal(s.pad.start, 8);
assert.equal(s.quality, 'medium'); assert.equal(s.effects, 0.4); assert.equal(s.reduced, true);
assert.equal(s.keys.left, 'KeyZ'); assert.equal(s.keys.right, 'KeyX');
assert.deepEqual(normalizeSettings(null).keys, DEFAULT_KEYS);
assert.deepEqual(normalizeSettings(undefined).pad, DEFAULT_PAD);

// An old Digit1 binding moves start to the next free key.
assert.equal(normalizeSettings({ keys: { plunger: 'Digit1' } }).keys.start, 'Digit2');
assert.equal(normalizeSettings({ keys: { plunger: 'Digit1', camera: 'Digit2' } }).keys.start, 'KeyS');
assert.equal(normalizeSettings({ pad: { camera: 8 } }).pad.start, 6);
assert.equal(normalizeSettings({ keys: { start: 'KeyG' } }).keys.start, 'KeyG');

// Duplicate bindings reset all keys; invalid values fall back.
assert.deepEqual(normalizeSettings({ keys: { left: 'KeyA', right: 'KeyA' } }).keys, DEFAULT_KEYS);
assert.deepEqual(normalizeSettings({ keys: { left: 'KeyA', start: 'KeyA' } }).keys, DEFAULT_KEYS);
assert.deepEqual(normalizeSettings({ pad: { left: 5 } }).pad, DEFAULT_PAD);
assert.equal(normalizeSettings({ keys: { left: 'bad key!' } }).keys.left, 'ShiftLeft');
assert.equal(normalizeSettings({ camera: 'top' }).camera, 'top');
// Ball cam is the default; a pre-ball-cam 'play' moves to it once, a later explicit 'play' (cameraRev set by normalize) sticks.
assert.equal(normalizeSettings(null).camera, 'follow'); assert.equal(normalizeSettings({ camera: 'bogus' }).camera, 'follow');
assert.equal(normalizeSettings({ camera: 'play' }).camera, 'follow'); assert.equal(normalizeSettings({ camera: 'follow' }).camera, 'follow');
const chosen = normalizeSettings({ camera: 'play', cameraRev: CAMERA_REV });
assert.equal(chosen.camera, 'play'); assert.equal(normalizeSettings(JSON.parse(JSON.stringify(chosen))).camera, 'play');
assert.equal(normalizeSettings({ camera: 'top', cameraRev: CAMERA_REV }).camera, 'top');
assert.equal(normalizeSettings({ callouts: false }).callouts, false);
assert.equal(normalizeSettings({ music: 2 }).music, 1);

// Scores: top 10 per table, sorted, invalid rows dropped.
const rows = Array.from({ length: 14 }, (_, i) => ({ name: 'P' + i, score: i * 100, date: 1 }));
rows.push({ name: 5, score: 1, date: 1 }, { name: 'x', score: -1, date: 1 }, { name: 'x', score: 1.5, date: 1 }, { name: 'x', score: 1, date: NaN }, null);
const sc = normalizeScores({ neon: rows, bogus: rows });
assert.equal(sc.neon.length, 10); assert.equal(sc.neon[0].score, 1300); assert.equal(sc.neon[9].score, 400);
assert.deepEqual(Object.keys(sc), [...TABLE_IDS]); assert.deepEqual(sc.temple, []);

// rngNext is deterministic and keeps its state in holder.seed.
const a = { seed: 7 }, b = { seed: 7 };
const seq = [rngNext(a), rngNext(a), rngNext(a)];
assert.deepEqual(seq, [rngNext(b), rngNext(b), rngNext(b)]);
assert.ok(seq.every(v => v >= 0 && v < 1) && new Set(seq).size === 3);
console.log('pinball config ok');
