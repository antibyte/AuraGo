const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const test = require('node:test');

// package.json has "type": "module": run the UMD source in a vm with a fake module.
const mod = { exports: {} };
vm.runInNewContext(fs.readFileSync(path.join(__dirname, 'notes-palette.js'), 'utf8'), {
    module: mod, window: {}, Promise, setTimeout, clearTimeout
});
const P = mod.exports;
const json = (v) => JSON.parse(JSON.stringify(v));

test('matchPositions prefers word starts', () => {
    assert.deepEqual(json(P.matchPositions('kid', 'Kleine Ideen')), [0, 7, 8]);
    assert.deepEqual(json(P.matchPositions('ki', 'Kleine Ideen')), [0, 7]);
});

test('matchPositions prefers consecutive runs', () => {
    assert.deepEqual(json(P.matchPositions('abc', 'xxabcx')), [2, 3, 4]);
    assert.deepEqual(json(P.matchPositions('proj', 'my project plan')), [3, 4, 5, 6]);
});

test('matchPositions returns null without an in-order match', () => {
    assert.equal(P.matchPositions('zzz', 'Kleine Ideen'), null);
    assert.equal(P.matchPositions('ba', 'ab'), null);
    assert.equal(P.matchPositions('abcd', 'abc'), null);
});

test('matchPositions ignores case and diacritics', () => {
    assert.deepEqual(json(P.matchPositions('uber', 'Über uns')), [0, 1, 2, 3]);
    assert.deepEqual(json(P.matchPositions('CAFE', 'café')), [0, 1, 2, 3]);
});

test('matchPositions with empty query matches nothing highlighted', () => {
    assert.deepEqual(json(P.matchPositions('', 'abc')), []);
    assert.deepEqual(json(P.matchPositions('  ', 'abc')), []);
});
