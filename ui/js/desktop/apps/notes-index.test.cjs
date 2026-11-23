const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const test = require('node:test');

// package.json has "type": "module": run the UMD source in a vm with a fake module.
const mod = { exports: {} };
vm.runInNewContext(fs.readFileSync(path.join(__dirname, 'notes-index.js'), 'utf8'), {
    module: mod, window: {}, Promise, setTimeout, clearTimeout
});
const N = mod.exports;
const json = (v) => JSON.parse(JSON.stringify(v));
const fixture = JSON.parse(fs.readFileSync(path.join(__dirname, '..', '..', '..', 'testdata', 'notes-links-fixture.json'), 'utf8'));

for (const c of fixture.extract) {
    test('extract: ' + c.name, () => {
        const r = json(N.extract(c.content));
        assert.equal(r.title, c.title);
        assert.deepEqual(r.aliases, c.aliases);
        assert.deepEqual(r.tags, c.tags);
        assert.deepEqual(r.links, c.links);
    });
}

const lookup = N.createLookup(fixture.resolve);
fixture.resolve.cases.forEach((c, i) => {
    test('resolve #' + i + ': ' + c.value, () => {
        assert.equal(lookup.resolve(c.from, c.kind || 'wiki', c.value, !!c.embed), c.path);
    });
});

test('parseWikiValue', () => {
    assert.deepEqual(json(N.parseWikiValue('Note#Sub|Alias')), { target: 'Note', subpath: 'Sub', alias: 'Alias' });
    assert.deepEqual(json(N.parseWikiValue('Note\\|Alias')), { target: 'Note', subpath: '', alias: 'Alias' });
    assert.deepEqual(json(N.parseWikiValue('#Local')), { target: '', subpath: 'Local', alias: '' });
});

test('fuzzy', () => {
    assert.ok(N.fuzzy('kid', 'Kleine Ideen') > 0);
    assert.equal(N.fuzzy('xyz', 'Kleine Ideen'), 0);
    assert.ok(N.fuzzy('ide', 'Ideen') > N.fuzzy('ide', 'Kleine Ideen'));
    assert.ok(N.fuzzy('uber', 'Über uns') > 0);
});

const P = 'Documents/Notes/';
const INDEX = {
    notes: [
        { path: P + 'Plan.md', title: 'Plan', aliases: [], tags: ['work'], links: [{ target: 'Ideen', path: P + 'Ideen.md', embed: false, kind: 'wiki' }, { target: 'Ideen', path: P + 'Ideen.md', embed: false, kind: 'wiki' }] },
        { path: P + 'Ideen.md', title: 'Kleine Ideen', aliases: ['Einfaelle'], tags: ['work', 'misc'], links: [{ target: 'Plan', path: P + 'Plan.md', embed: false, kind: 'wiki' }] },
        { path: P + 'Archiv/Plan.md', title: 'Alter Plan', aliases: [], tags: [], links: [{ target: 'Plan', path: P + 'Plan.md', embed: false, kind: 'wiki' }] },
    ],
    folders: [P + 'Archiv'],
    attachments: []
};

function makeStore() {
    const calls = [];
    const changes = [];
    const store = N.create({
        request: (url, opts) => { calls.push(url); return Promise.resolve(INDEX); },
        onChange: (d) => changes.push(d)
    });
    return { store, calls, changes };
}

test('store queries', async () => {
    const { store, calls, changes } = makeStore();
    await store.load();
    assert.equal(calls[0], '/api/desktop/notes?index=1');
    assert.equal(changes.length, 1);
    assert.deepEqual(json(store.backlinks(P + 'Ideen.md')), [{ path: P + 'Plan.md', title: 'Plan', count: 2 }]);
    assert.deepEqual(json(store.backlinks(P + 'Plan.md')), [
        { path: P + 'Archiv/Plan.md', title: 'Alter Plan', count: 1 },
        { path: P + 'Ideen.md', title: 'Kleine Ideen', count: 1 }
    ]);
    assert.deepEqual(json(store.tags()), [{ tag: 'misc', count: 1 }, { tag: 'work', count: 2 }]);
    const hits = store.search('pla');
    assert.equal(hits[0].path, P + 'Plan.md');
    assert.equal(hits.length, 2);
    assert.deepEqual(json(store.search('')), []);
    assert.equal(store.linkText(P + 'Plan.md'), 'Plan');
    assert.equal(store.linkText(P + 'Ideen.md'), 'Ideen');
    assert.equal(store.note(P + 'Ideen.md').title, 'Kleine Ideen');
    assert.equal(store.note('nope'), null);
    assert.equal(store.notes().length, 3);
    assert.equal(store.resolve(P + 'Ideen.md', 'Einfaelle'), P + 'Ideen.md');
});

test('store linkText duplicated basename and outgoing', async () => {
    const { store } = makeStore();
    await store.load();
    assert.equal(store.linkText(P + 'Archiv/Plan.md'), 'Archiv/Plan');
    assert.equal(store.linkText(P + 'Plan.md'), 'Plan');
    const out = json(store.outgoing(P + 'Ideen.md', 'See [[Plan]] and [[Fehlt]]'));
    assert.deepEqual(out, [
        { value: 'Plan', path: P + 'Plan.md', embed: false, kind: 'wiki' },
        { value: 'Fehlt', path: '', embed: false, kind: 'wiki' }
    ]);
});

test('refresh debounces and failures keep data', async () => {
    const { store, calls } = makeStore();
    await Promise.all([store.refresh(), store.refresh()]);
    assert.equal(calls.length, 1);
    assert.equal(store.notes().length, 3);

    let fail = true;
    const s2 = N.create({ request: () => (fail ? Promise.reject(new Error('x')) : Promise.resolve(INDEX)) });
    await assert.rejects(s2.load());
    assert.equal(s2.notes().length, 0);
    fail = false;
    await s2.load();
    fail = true;
    await assert.rejects(s2.load());
    assert.equal(s2.notes().length, 3);
});
