#!/usr/bin/env node
// Sticky notes (UI-11): normalize() cuts long notes at code points, strips refused format
// characters from notes and group texts, and reports the change in a flag that is never saved.
import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';
import { webcrypto } from 'node:crypto';

const box = vm.createContext({ window: {}, crypto: webcrypto, console, Intl, Date, Map, Set, JSON });
for (const name of ['easydrag-template.js', 'easydrag-model.js']) {
    const file = new URL('../ui/js/desktop/apps/' + name, import.meta.url);
    vm.runInContext(fs.readFileSync(file, 'utf8'), box, { filename: file.pathname });
}
const ED = box.window.EasyDrag;
const catalog = { types: new Map() };
const cp = (...points) => String.fromCodePoint(...points);
const family = cp(0x1F469, 0x200D, 0x1F467);
const rlo = cp(0x202E);
const start = () => ({ id: 'n_aaaaaaaa', key: 'start', type: 'trigger.manual', position: { x: 0, y: 0 }, params: {}, settings: {} });

const clean = ED.model.create({ schema: 1, nodes: [start()], edges: [],
    notes: [{ id: 's_ok', position: { x: 1, y: 2 }, width: 200, height: 120, text: 'Hallo ' + family, color: 'amber' }] }, catalog);
assert.equal(clean.repairs().notesTrimmed, false, 'a valid note is left alone');
assert.equal(clean.doc.notes[0].text, 'Hallo ' + family, 'ZWJ emoji stay');

const notes = [
    { id: 's_long', position: { x: 0, y: 0 }, width: 50, height: 5000, text: cp(0x1F600).repeat(2049), color: 'red' },
    { id: 's_long', position: { x: 0, y: 0 }, width: 200, height: 120, text: 'pay ' + rlo + 'evil', color: 'blue' }
];
for (let i = 0; i < 100; i++) notes.push({ id: 's_n' + i, position: { x: 0, y: 0 }, width: 200, height: 120, text: '' });
const model = ED.model.create({ schema: 1, nodes: [start()], edges: [], notes,
    groups: [{ id: 'g_abcdefgh', title: 'Ab' + rlo + 'c', color: 'blue', members: ['n_aaaaaaaa'] }] }, catalog);
const doc = model.doc;
assert.equal(model.repairs().notesTrimmed, true, 'the change is reported');
assert.equal(doc.notes.length, 100, 'only the first 100 notes stay');
assert.equal([...doc.notes[0].text].length, 2048, 'cut at 2,048 code points');
assert.ok(new TextEncoder().encode(doc.notes[0].text).length <= 8192, 'and at most 8,192 bytes');
assert.equal(doc.notes[0].text, cp(0x1F600).repeat(2048), 'no code point is split');
assert.equal(doc.notes[0].width, 80, 'width is clamped');
assert.equal(doc.notes[0].height, 1200, 'height is clamped');
assert.equal(doc.notes[0].color, '', 'an unknown colour becomes empty');
assert.equal(doc.notes[1].text, 'pay evil', 'U+202E is stripped');
assert.notEqual(doc.notes[1].id, 's_long', 'a duplicate id gets a new one');
assert.match(doc.notes[1].id, /^s_[a-z2-7]{8}$/);
assert.equal(doc.groups[0].title, 'Abc', 'group titles lose refused format characters too');
assert.equal(JSON.stringify(model.toJSON()).includes('notesTrimmed'), false, 'the flag is never saved');
console.log('easydrag notes normaliser: ok');
