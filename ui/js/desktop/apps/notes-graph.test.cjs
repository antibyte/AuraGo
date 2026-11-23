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
