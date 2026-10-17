import assert from 'node:assert/strict';
import { advanceHole, beginShot, cloneSave, createRound, resolveShot, scoreTotal, validateSave } from '../ui/js/desktop/apps/minigolf-model.js';

const courses = [
    { id: 'garden-1', tee: [-2, 0.16, 0], cup: [2, 0, 0] },
    { id: 'garden-2', tee: [0, 1.16, 0], cup: [4, 1, 0] }
];
const round = createRound(courses, { players: ['Ada', 'Lin'], mode: 'garden' });
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

const practice = createRound(courses, { players: ['Max'], mode: 'practice', holeId: 'garden-2' });
assert.deepEqual(practice.courseIds, ['garden-2']);
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

const invalidPrototype = JSON.parse('{"version":1,"mode":"garden","courseIds":["garden-1"],"holeIndex":0,"players":[],"turnIndex":0,"completed":false,"__proto__":{"polluted":true}}');
assert.equal(validateSave(invalidPrototype, courses), false);
assert.equal(validateSave({ ...cloneSave(createRound(courses)), courseIds: ['missing'] }, courses), false);
assert.throws(() => createRound(courses, { players: [] }), /1 to 4/);

console.log('minigolf model: all checks passed');
