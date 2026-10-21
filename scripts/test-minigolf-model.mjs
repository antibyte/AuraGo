import assert from 'node:assert/strict';
import { advanceHole, beginShot, cloneSave, createRound, normalizeBest, recordHole, recordRound, resolveShot, scoreTotal, validateSave } from '../ui/js/desktop/apps/minigolf-model.js';

const courses = [
    { id: 'temple-1', tee: [-2, 0.16, 0], cup: [2, 0, 0] },
    { id: 'temple-2', tee: [0, 1.16, 0], cup: [4, 1, 0] }
];
const round = createRound(courses, { players: ['Ada', 'Lin'], mode: 'temple' });
assert.equal(validateSave(round, courses), true);
assert.deepEqual(beginShot(round), { playerIndex: 0, player: 'Ada', position: [-2, 0.16, 0], shots: 0, strokes: 0 });
assert.equal(beginShot(round), null);
const checkpoint = cloneSave(round);
assert.deepEqual(checkpoint, round);
assert.equal(validateSave(JSON.parse(JSON.stringify(checkpoint)), courses), true);

assert.equal(resolveShot(round, { type: 'rest', position: [-1, 0.16, 0] }, courses[0]), true);
assert.equal(round.turnIndex, 1);
assert.equal(resolveShot(round, { type: 'water', position: [0, -1, 0] }, courses[0]), false);
assert.deepEqual(beginShot(round), { playerIndex: 1, player: 'Lin', position: [-2, 0.16, 0], shots: 0, strokes: 0 });
assert.equal(resolveShot(round, { type: 'water', position: [0, -1, 0] }, courses[0]), true);
assert.equal(round.players[1].strokes, 2);
assert.deepEqual(round.players[1].position, [-2, 0.16, 0]);
assert.deepEqual(beginShot(round), { playerIndex: 0, player: 'Ada', position: [-1, 0.16, 0], shots: 1, strokes: 1 });
assert.equal(resolveShot(round, { type: 'cup', position: [2, -0.2, 0] }, courses[0]), true);
assert.equal(round.players[0].done, true);
assert.equal(beginShot(round).player, 'Lin');
assert.equal(resolveShot(round, { type: 'cup', position: [2, -0.2, 0] }, courses[0]), true);
assert.equal(round.players.every(player => player.done), true);
assert.equal(advanceHole(round, courses), true);
assert.equal(round.holeIndex, 1);
assert.deepEqual(round.players.map(player => player.scores), [[2], [3]]);
assert.deepEqual(round.players.map(player => player.position), [courses[1].tee, courses[1].tee]);
assert.deepEqual(round.players.map(scoreTotal), [2, 3]);
assert.equal(validateSave(cloneSave(round), courses), true);

assert.equal(beginShot(round).player, 'Ada');
assert.equal(resolveShot(round, { type: 'cup', position: [4, 0.8, 0] }, courses[1]), true);
assert.equal(beginShot(round).player, 'Lin');
assert.equal(resolveShot(round, { type: 'cup', position: [4, 0.8, 0] }, courses[1]), true);
assert.equal(advanceHole(round, courses), true);
assert.equal(round.completed, true);
assert.equal(round.holeIndex, 2);
assert.deepEqual(round.players.map(scoreTotal), [3, 4]);
assert.equal(validateSave(cloneSave(round), courses), true);
assert.equal(advanceHole(round, courses), false);

const practice = createRound(courses, { players: ['Max'], mode: 'practice', holeId: 'temple-2' });
assert.deepEqual(practice.courseIds, ['temple-2']);
for (let shot = 0; shot < 12; shot++) {
    assert.ok(beginShot(practice));
    assert.equal(resolveShot(practice, { type: 'rest', position: practice.players[0].position }, courses[1]), true);
}
assert.equal(practice.players[0].done, true);
assert.equal(practice.players[0].strokes, 13);
assert.equal(beginShot(practice), null);
assert.equal(advanceHole(practice, courses), true);
assert.equal(practice.completed, true);
assert.equal(scoreTotal(practice.players[0]), 13);

const invalidPrototype = JSON.parse('{"version":1,"mode":"temple","courseIds":["temple-1"],"holeIndex":0,"players":[],"turnIndex":0,"completed":false,"__proto__":{"polluted":true}}');
assert.equal(validateSave(invalidPrototype, courses), false);
assert.equal(validateSave({ ...cloneSave(createRound(courses)), courseIds: ['missing'] }, courses), false);
assert.throws(() => createRound(courses, { players: [] }), /1 to 4/);

// --- Round format v2, modes and bests (P4) ----------------------------------
assert.equal(round.version, 2);
assert.deepEqual(createRound(courses, { players: ['Ada', 'Lin'] }).players.map(p => p.color), ['cream', 'coral'], 'string players get default colours');
const colored = createRound(courses, { players: [{ name: 'Ada', color: 'teal' }, { name: 'Lin', color: 'gold' }] });
assert.deepEqual(cloneSave(colored).players.map(p => p.color), ['teal', 'gold']);
const coloredSave = JSON.parse(JSON.stringify(cloneSave(colored)));
assert.equal(validateSave(coloredSave, courses), true);
assert.throws(() => createRound(courses, { players: [{ name: 'Ada', color: 'teal' }, { name: 'Lin', color: 'teal' }] }), /color/);
assert.throws(() => createRound(courses, { players: [{ name: 'Ada', color: 'magenta' }] }), /color/);
assert.ok(createRound(courses, { players: ['x'.repeat(24)] }));
assert.ok(createRound(courses, { players: [String.fromCodePoint(0x1f600).repeat(24)] }), 'names count code points');
assert.throws(() => createRound(courses, { players: ['x'.repeat(25)] }), /name/);
for (const mode of ['full', 'practice', 'temple', 'pirate', 'moon-2']) assert.equal(createRound(courses, { mode, holeId: mode === 'practice' ? 'temple-1' : undefined }).mode, mode);
for (const mode of ['Garden', '1world', '', 'a'.repeat(25), 'x_y']) assert.throws(() => createRound(courses, { mode }), /mode/);
assert.equal(validateSave({ ...coloredSave, version: 1 }, courses), false);
assert.equal(validateSave({ ...coloredSave, players: coloredSave.players.map(({ color, ...rest }) => rest) }, courses), false, 'colour is required');
assert.equal(validateSave({ ...coloredSave, players: coloredSave.players.map(p => ({ ...p, color: 'teal' })) }, courses), false, 'colours are unique');
assert.equal(validateSave({ ...coloredSave, players: coloredSave.players.map((p, i) => ({ ...p, name: 'n'.repeat(25) + i })) }, courses), false);

const bestCourses = [
    { id: 'temple-1', world: 'temple', tee: [0, 0, 0], cup: [1, 0, 0] },
    { id: 'temple-2', world: 'temple', tee: [0, 0, 0], cup: [1, 0, 0] },
    { id: 'pirate-1', world: 'pirate', tee: [0, 0, 0], cup: [1, 0, 0] }
];
const set = 'pirate-1,temple-1,temple-2';
assert.deepEqual(normalizeBest(null, bestCourses), { version: 2, set, rounds: {}, holes: {} });
assert.deepEqual(normalizeBest({ version: 2, set, rounds: { full: 9, temple: 5, moon: 4, pirate: 0 }, holes: { 'temple-1': 2, 'gone-1': 1, 'pirate-1': 14 } }, bestCourses),
    { version: 2, set, rounds: { full: 9, temple: 5 }, holes: { 'temple-1': 2 } });
assert.deepEqual(normalizeBest({ version: 2, set: 'temple-1,temple-2', rounds: { full: 9 }, holes: { 'temple-1': 2 } }, bestCourses),
    { version: 2, set, rounds: {}, holes: { 'temple-1': 2 } }, 'a changed course set drops only round bests');
const best = normalizeBest(null, bestCourses);
const sink = (r, course) => { assert.ok(beginShot(r)); assert.equal(resolveShot(r, { type: 'cup', position: course.cup }, course), true); };
const hole = createRound(bestCourses, { players: ['Ada', 'Lin'], mode: 'practice', holeId: 'temple-1' });
assert.equal(recordHole(best, hole), false, 'an unfinished hole records nothing');
assert.ok(beginShot(hole)); assert.equal(resolveShot(hole, { type: 'rest', position: [0.5, 0, 0] }, bestCourses[0]), true);
sink(hole, bestCourses[0]); sink(hole, bestCourses[0]);
assert.equal(recordHole(best, hole), true);
assert.equal(best.holes['temple-1'], 1);
assert.equal(recordHole(best, hole), false, 'an equal score is no new best');
assert.equal(advanceHole(hole, bestCourses), true);
assert.equal(recordRound(best, hole), false, 'practice never sets round bests');
const templeRound = createRound(bestCourses.slice(0, 2), { players: ['Ada'], mode: 'temple' });
sink(templeRound, bestCourses[0]); advanceHole(templeRound, bestCourses);
sink(templeRound, bestCourses[1]); advanceHole(templeRound, bestCourses);
assert.equal(recordRound(best, templeRound), true);
assert.equal(best.rounds.temple, 2);
assert.equal(recordRound(best, templeRound), false);

console.log('minigolf model: all checks passed');
