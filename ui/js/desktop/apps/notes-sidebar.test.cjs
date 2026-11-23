const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const test = require('node:test');

// package.json has "type": "module": run the UMD source in a vm with a fake module.
const mod = { exports: {} };
vm.runInNewContext(fs.readFileSync(path.join(__dirname, 'notes-sidebar.js'), 'utf8'), {
    module: mod, window: {}, Promise, setTimeout, clearTimeout
});
const S = mod.exports;
const json = (v) => JSON.parse(JSON.stringify(v));
const R = 'Documents/Notes';
const note = (p, title, modified) => ({ path: R + '/' + p, title, modified });

test('buildTree creates implied folders and nests deeply', () => {
    const t = json(S.buildTree([note('A/B/C/x.md', 'x', 1), note('top.md', 'top', 2)], [R + '/Empty'], { root: R }));
    assert.equal(t.path, R);
    assert.deepEqual(t.folders.map((f) => f.name), ['A', 'Empty']);
    assert.deepEqual(t.notes.map((n) => n.title), ['top']);
    const c = t.folders[0].folders[0].folders[0];
    assert.equal(c.path, R + '/A/B/C');
    assert.equal(c.name, 'C');
    assert.deepEqual(c.notes.map((n) => n.title), ['x']);
    assert.deepEqual(t.folders[1].notes, []);
});

test('buildTree sorts by name with numeric order', () => {
    const t = json(S.buildTree(
        [note('n10.md', 'Note 10', 1), note('n2.md', 'Note 2', 2), note('n1.md', 'note 1', 3)],
        [R + '/Ordner 10', R + '/Ordner 2', R + '/ärger'], { sort: 'name', root: R }));
    assert.deepEqual(t.notes.map((n) => n.title), ['note 1', 'Note 2', 'Note 10']);
    assert.deepEqual(t.folders.map((f) => f.name), ['ärger', 'Ordner 2', 'Ordner 10']);
});

test('buildTree sorts notes by modified descending', () => {
    const t = json(S.buildTree([note('a.md', 'a', 1), note('b.md', 'b', 30), note('c.md', 'c', 20)], [], { sort: 'modified', root: R }));
    assert.deepEqual(t.notes.map((n) => n.title), ['b', 'c', 'a']);
});

test('buildTree excludes the root from names and handles a root folder entry', () => {
    const t = json(S.buildTree([], [R, R + '/X'], { root: R }));
    assert.deepEqual(t.folders.map((f) => f.name), ['X']);
});

test('parseQuery', () => {
    assert.deepEqual(json(S.parseQuery('tag:#Projekt/alpha foo bar')), { q: 'foo bar', tag: 'Projekt/alpha', folder: '' });
    assert.deepEqual(json(S.parseQuery('tag:x')), { q: '', tag: 'x', folder: '' });
    assert.deepEqual(json(S.parseQuery('path:"Meine Ordner/2026" text')), { q: 'text', tag: '', folder: R + '/Meine Ordner/2026' });
    assert.deepEqual(json(S.parseQuery('path:A/B')), { q: '', tag: '', folder: R + '/A/B' });
    assert.deepEqual(json(S.parseQuery('  one   tag:t  two  path:P ')), { q: 'one two', tag: 't', folder: R + '/P' });
    assert.deepEqual(json(S.parseQuery('')), { q: '', tag: '', folder: '' });
});

test('markTerms', () => {
    assert.equal(S.markTerms('a <b>bold</b> move', ['b']), 'a &lt;<mark>b</mark>&gt;<mark>b</mark>old&lt;/<mark>b</mark>&gt; move');
    assert.equal(S.markTerms('Über uns', ['uber']), '<mark>Über</mark> uns');
    assert.equal(S.markTerms('Foo bar foo', ['foo', 'bar']), '<mark>Foo</mark> <mark>bar</mark> <mark>foo</mark>');
    assert.equal(S.markTerms('a & b', []), 'a &amp; b');
    assert.equal(S.markTerms('a & b', ['']), 'a &amp; b');
    assert.equal(S.markTerms('aaa', ['aa', 'aaa']), '<mark>aaa</mark>');
});
