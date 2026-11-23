const assert = require('node:assert/strict');
const test = require('node:test');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
// package.json has "type": "module", so plain require() would treat the .js as ESM; run the UMD source in a vm.
const mod = { exports: {} };
vm.runInNewContext(fs.readFileSync(path.join(__dirname, 'notes-frontmatter.js'), 'utf8'), { module: mod, window: {} });
const raw = mod.exports;
// results from the vm realm have foreign prototypes; normalise for deepEqual
const FM = Object.fromEntries(Object.entries(raw).map(([k, fn]) => [k, (...a) => { const r = fn(...a); return typeof r === 'string' ? r : JSON.parse(JSON.stringify(r)); }]));

test('updateTags adds a block to a note without frontmatter', () => {
    assert.equal(FM.updateTags('Hello', ['A', 'b']), '---\ntags: a, b\n---\n\nHello');
    assert.equal(FM.updateTags('Hello', []), 'Hello');
});
test('updateTags replaces tags keeping owner and bracket style', () => {
    const src = '---\nowner: Preserve this field\ntags: [x, y]\n---\nBody';
    assert.equal(FM.updateTags(src, ['New']), '---\nowner: Preserve this field\ntags: [new]\n---\nBody');
    const plain = '---\ntags: x\nowner: Preserve this field\n---\nBody';
    assert.equal(FM.updateTags(plain, ['n1', 'n2']), '---\ntags: n1, n2\nowner: Preserve this field\n---\nBody');
});
test('updateTags preserves CRLF and drops an emptied block', () => {
    const src = '---\r\ntags: [x]\r\nowner: o\r\n---\r\nBody';
    assert.equal(FM.updateTags(src, ['z']), '---\r\ntags: [z]\r\nowner: o\r\n---\r\nBody');
    assert.equal(FM.updateTags('---\ntags: x\n---\nBody', []), 'Body');
});
test('updateTags replaces block-list tags', () => {
    const src = '---\ntags:\n  - a\n  - b\nowner: o\n---\nBody';
    assert.equal(FM.updateTags(src, ['c']), '---\ntags: c\nowner: o\n---\nBody');
    assert.equal(FM.updateTags('---\ntags:\n  - a\n---\nBody', []), 'Body');
});
test('deriveTitle and strip', () => {
    assert.equal(FM.deriveTitle('---\ntitle: "T"\n---\n# H'), 'T');
    assert.equal(FM.deriveTitle('---\ntags: a\n---\n\n# Heading\ntext'), 'Heading');
    assert.equal(FM.deriveTitle('', 'file.md'), 'file');
    assert.equal(FM.strip('---\ntags: a\n---\nBody'), 'Body');
    assert.equal(FM.strip('no fm'), 'no fm');
});
test('listValues handles bracket, comma, block, quoted and missing', () => {
    assert.deepEqual(FM.listValues('---\naliases: [Alpha, "b c"]\n---\n', 'aliases'), ['Alpha', 'b c']);
    assert.deepEqual(FM.listValues('---\naliases: Alpha, Beta\n---\n', 'aliases'), ['Alpha', 'Beta']);
    assert.deepEqual(FM.listValues('---\naliases:\n  - Alpha\n  - "B c"\nx: y\n---\n', 'aliases'), ['Alpha', 'B c']);
    assert.deepEqual(FM.listValues("---\naliases: ['One', 'Two']\n---\n", 'aliases'), ['One', 'Two']);
    assert.deepEqual(FM.listValues('---\nx: y\n---\n', 'aliases'), []);
    assert.deepEqual(FM.listValues('no frontmatter', 'aliases'), []);
    assert.deepEqual(FM.listValues('---\naliases: a\nno end', 'aliases'), []);
});
test('updateList adds and replaces without touching other lines', () => {
    const src = '---\ntags: [x]\nowner: Preserve this field\n---\nBody';
    const added = FM.updateList(src, 'aliases', ['Alpha', 'P A']);
    assert.equal(added, '---\naliases: Alpha, "P A"\ntags: [x]\nowner: Preserve this field\n---\nBody');
    const replaced = FM.updateList(added, 'aliases', ['Zed']);
    assert.equal(replaced, '---\naliases: Zed\ntags: [x]\nowner: Preserve this field\n---\nBody');
    assert.equal(FM.updateList(added, 'aliases', []), src);
    assert.equal(FM.updateList('Body', 'aliases', ['A']), '---\naliases: A\n---\n\nBody');
    assert.equal(FM.updateList('---\naliases: [A]\n---\nB', 'aliases', ['C']), '---\naliases: [C]\n---\nB');
    assert.equal(FM.updateList('---\r\naliases: A\r\n---\r\nB', 'aliases', ['C', 'D']), '---\r\naliases: C, D\r\n---\r\nB');
    assert.equal(FM.updateList('---\naliases: A\n---\nB', 'aliases', []), 'B');
});
test('fields lists top-level keys in order', () => {
    const src = '---\ntitle: T\ntags: [a, b]\naliases:\n  - X\n  - Y\nowner: me\nempty:\n---\nBody';
    assert.deepEqual(FM.fields(src), [
        { key: 'title', value: 'T', list: false },
        { key: 'tags', value: 'a, b', list: true },
        { key: 'aliases', value: 'X, Y', list: true },
        { key: 'owner', value: 'me', list: false },
        { key: 'empty', value: '', list: false },
    ]);
    assert.deepEqual(FM.fields('Body'), []);
});
test('parse returns aliases', () => {
    assert.deepEqual(FM.parse('---\naliases: [A, B]\n---\n').aliases, ['A', 'B']);
    assert.deepEqual(FM.parse('---\nalias: Solo\n---\n').aliases, ['Solo']);
    assert.deepEqual(FM.parse('x').aliases, []);
});
