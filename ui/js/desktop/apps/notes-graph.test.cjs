const assert = require('node:assert/strict');
const test = require('node:test');

// The repo's package.json is "type": "module", so a plain require() of a .js file
// would load it as ESM. Evaluate it CommonJS-style in a sandbox that has neither
// document nor window (like the other Notes tests do with vm).
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const sandbox = { module: { exports: {} }, console, performance };
// No document/window in the sandbox: any DOM access at load time would throw.
vm.runInNewContext(fs.readFileSync(path.join(__dirname, 'notes-graph.js'), 'utf8'), sandbox);
const NotesGraph = sandbox.module.exports;

function rng(seed) {
    let s = seed >>> 0;
    return () => {
        s = (s + 0x6D2B79F5) >>> 0;
        let t = s;
        t = Math.imul(t ^ (t >>> 15), t | 1);
        t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
        return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
    };
}

function ring(n) {
    const nodes = [];
    const links = [];
    for (let i = 0; i < n; i++) {
        nodes.push({ id: 'n' + i });
        if (i > 0) links.push({ source: 'n' + i, target: 'n' + (i - 1) });
    }
    return { nodes, links };
}

test('same seed gives identical positions after 50 ticks', () => {
    const run = () => {
        const g = ring(25);
        const sim = NotesGraph.createSimulation({ nodes: g.nodes, links: g.links, seed: 7 });
        for (let i = 0; i < 50; i++) sim.tick();
        return sim.nodes.map(n => [n.x, n.y]);
    };
    assert.deepEqual(run(), run());
});

test('unlinked nodes separate', () => {
    const nodes = [];
    for (let i = 0; i < 30; i++) nodes.push({ id: 'u' + i });
    const sim = NotesGraph.createSimulation({ nodes, links: [], seed: 3 });
    sim.run(600);
    let min = Infinity;
    for (let i = 0; i < nodes.length; i++) {
        for (let j = i + 1; j < nodes.length; j++) {
            min = Math.min(min, Math.hypot(nodes[i].x - nodes[j].x, nodes[i].y - nodes[j].y));
        }
    }
    assert.ok(min > 8, 'min distance ' + min);
});

test('springs keep clusters together', () => {
    const nodes = [];
    const links = [];
    for (const c of ['A', 'B']) {
        for (let i = 1; i <= 5; i++) nodes.push({ id: c + i });
        for (let i = 1; i <= 5; i++) for (let j = i + 1; j <= 5; j++) links.push({ source: c + i, target: c + j });
    }
    links.push({ source: 'A1', target: 'B1' });
    const sim = NotesGraph.createSimulation({ nodes, links, seed: 11 });
    sim.run(600);
    let intra = 0, ni = 0, inter = 0, nx = 0;
    for (let i = 0; i < nodes.length; i++) {
        for (let j = i + 1; j < nodes.length; j++) {
            const d = Math.hypot(nodes[i].x - nodes[j].x, nodes[i].y - nodes[j].y);
            if (nodes[i].id[0] === nodes[j].id[0]) { intra += d; ni++; } else { inter += d; nx++; }
        }
    }
    assert.ok(intra / ni < inter / nx, `intra ${intra / ni} inter ${inter / nx}`);
});

test('sparse graph: orphans stay close to the linked chain', () => {
    const nodes = [];
    const links = [];
    for (let i = 0; i < 5; i++) {
        nodes.push({ id: 'c' + i });
        if (i > 0) links.push({ source: 'c' + i, target: 'c' + (i - 1) });
    }
    for (let i = 0; i < 5; i++) nodes.push({ id: 'o' + i });
    const sim = NotesGraph.createSimulation({ nodes, links, seed: 4 });
    sim.run();
    const chain = nodes.slice(0, 5);
    const cx = chain.reduce((a, n) => a + n.x, 0) / 5;
    const cy = chain.reduce((a, n) => a + n.y, 0) / 5;
    const chainR = Math.max(...chain.map(n => Math.hypot(n.x - cx, n.y - cy)));
    const far = Math.max(...nodes.slice(5).map(n => Math.hypot(n.x - cx, n.y - cy)));
    console.log('chain radius ' + chainR.toFixed(1) + ', farthest orphan ' + far.toFixed(1));
    // Bound: orphans form a loose ring; 3x the chain radius keeps the fitted cluster large.
    assert.ok(far < 3 * chainR, `orphan ${far} vs chain radius ${chainR}`);
});

test('cooling stops and reheat restarts', () => {
    const g = ring(20);
    const sim = NotesGraph.createSimulation({ nodes: g.nodes, links: g.links, seed: 1 });
    assert.equal(sim.stopped, false);
    let ticks = 0;
    while (!sim.stopped && ticks < 400) { sim.tick(); ticks++; }
    assert.ok(sim.stopped, 'stopped within 400 ticks');
    assert.ok(sim.alpha < 0.002);
    sim.reheat();
    assert.equal(sim.stopped, false);
    assert.ok(sim.alpha >= 0.5 - 1e-9);
    sim.run(400);
    assert.equal(sim.stopped, true);
});

test('pinned nodes keep their position', () => {
    const g = ring(10);
    g.nodes[3].fx = 42;
    g.nodes[3].fy = -17;
    const sim = NotesGraph.createSimulation({ nodes: g.nodes, links: g.links, seed: 5 });
    for (let i = 0; i < 40; i++) sim.tick();
    assert.equal(g.nodes[3].x, 42);
    assert.equal(g.nodes[3].y, -17);
});

test('performance: 2000 nodes / 3000 links', () => {
    const r = rng(99);
    const nodes = [];
    const links = [];
    for (let i = 0; i < 2000; i++) nodes.push({ id: 'p' + i });
    for (let i = 0; i < 3000; i++) {
        const a = Math.floor(r() * 2000);
        let b = Math.floor(r() * 2000);
        if (b === a) b = (b + 1) % 2000;
        links.push({ source: 'p' + a, target: 'p' + b });
    }
    const sim = NotesGraph.createSimulation({ nodes, links, seed: 2 });
    sim.tick();
    const t0 = performance.now();
    for (let i = 0; i < 20; i++) sim.tick();
    const avg = (performance.now() - t0) / 20;
    console.log('average tick ms: ' + avg.toFixed(2));
    assert.ok(avg < 30, 'avg tick ' + avg);
});

test('requiring in node exposes the API without DOM', () => {
    assert.equal(typeof NotesGraph.createSimulation, 'function');
    assert.equal(typeof NotesGraph.create, 'function');
    assert.equal(typeof sandbox.document, 'undefined');
    assert.equal(typeof sandbox.window, 'undefined');
    assert.equal(typeof document, 'undefined');
});

// ---- graph data from the notes index

const D = 'Documents/Notes/';
const fixtureNotes = () => [
    { path: D + 'a.md', title: 'Alpha', tags: ['x', 'y'], links: [
        { target: 'b', kind: 'wiki', path: D + 'b.md' },
        { target: 'b#Plan|again', kind: 'wiki', path: D + 'b.md' },
        { target: 'a', kind: 'wiki', path: D + 'a.md' },
        { target: 'skizze.png', kind: 'wiki', embed: true, path: D + '.attachments/skizze.png' },
        { target: 'Fehlt Noch#Teil|Alias', kind: 'wiki', path: '' },
        { target: 'foto.png', kind: 'wiki', path: '' },
        { target: '../ext.md', kind: 'markdown', path: '' }
    ] },
    { path: D + 'b.md', title: 'Beta', tags: ['x'], links: [{ target: 'a', kind: 'wiki', path: '' }, { target: 'fehlt noch', kind: 'wiki', path: '' }] },
    { path: D + 'c.md', title: 'Gamma', tags: [], links: [] }
];
const byLookup = (n, l) => l.path || (l.target === 'a' ? D + 'a.md' : '');
const ids = g => Array.from(g.nodes.map(n => n.id)).sort();

test('fromIndex: edges are undirected, deduplicated, without self loops or attachments', () => {
    const g = NotesGraph.fromIndex(fixtureNotes(), { linkPath: byLookup, current: D + 'b.md' });
    assert.deepEqual(ids(g), [D + 'a.md', D + 'b.md', D + 'c.md']);
    assert.equal(g.links.length, 1);
    assert.equal(g.nodes.find(n => n.id === D + 'b.md').current, true);
    assert.equal(g.nodes.find(n => n.id === D + 'a.md').current, false);
    assert.equal(g.nodes.find(n => n.id === D + 'a.md').label, 'Alpha');
});

test('fromIndex: unresolved targets merge by lowercase and skip attachments and markdown links', () => {
    const g = NotesGraph.fromIndex(fixtureNotes(), { linkPath: byLookup, unresolved: true });
    const un = g.nodes.filter(n => n.kind === 'unresolved');
    assert.deepEqual(Array.from(un.map(n => n.id)), ['unresolved:fehlt noch']);
    assert.equal(un[0].label, 'Fehlt Noch');
    assert.equal(g.links.filter(l => l.target === 'unresolved:fehlt noch').length, 2);
});

test('fromIndex: tags become nodes with note edges', () => {
    const g = NotesGraph.fromIndex(fixtureNotes(), { linkPath: byLookup, tags: true });
    assert.deepEqual(Array.from(g.nodes.filter(n => n.kind === 'tag').map(n => n.id)).sort(), ['tag:x', 'tag:y']);
    assert.equal(g.nodes.find(n => n.id === 'tag:x').label, '#x');
    assert.equal(g.links.length, 1 + 3);
});

test('fromIndex: orphans can be hidden, tag nodes keep their notes connected', () => {
    const plain = NotesGraph.fromIndex(fixtureNotes(), { linkPath: byLookup, orphans: false });
    assert.deepEqual(ids(plain), [D + 'a.md', D + 'b.md']);
    const tagged = NotesGraph.fromIndex(fixtureNotes(), { linkPath: byLookup, orphans: false, tags: true });
    assert.ok(!ids(tagged).includes(D + 'c.md'));
    assert.ok(ids(tagged).includes('tag:x'));
});

test('fromIndex: the filter keeps matching notes and edges among kept nodes', () => {
    const g = NotesGraph.fromIndex(fixtureNotes(), { linkPath: byLookup, filter: 'alp', tags: true, fuzzy: (q, t) => t.toLowerCase().includes(q) ? 1 : 0 });
    assert.deepEqual(ids(g), [D + 'a.md', 'tag:x', 'tag:y']);
    assert.equal(g.links.length, 2);
    const none = NotesGraph.fromIndex(fixtureNotes(), { linkPath: byLookup, filter: 'zzz' });
    assert.equal(none.nodes.length, 0);
});

test('neighborhood: depth limits the hops and keeps induced links', () => {
    const chain = { nodes: ['a', 'b', 'c', 'd'].map(id => ({ id })), links: [{ source: 'a', target: 'b' }, { source: 'b', target: 'c' }, { source: 'c', target: 'd' }] };
    assert.deepEqual(Array.from(NotesGraph.neighborhood(chain, 'b', 1).nodes.map(n => n.id)), ['a', 'b', 'c']);
    assert.deepEqual(Array.from(NotesGraph.neighborhood(chain, 'a', 2).nodes.map(n => n.id)), ['a', 'b', 'c']);
    assert.equal(NotesGraph.neighborhood(chain, 'a', 2).links.length, 2);
    assert.equal(JSON.stringify(NotesGraph.neighborhood(chain, 'zz', 1)), '{"nodes":[],"links":[]}');
});
