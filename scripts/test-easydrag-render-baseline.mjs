#!/usr/bin/env node
// S0.0 render baseline (plans/2026-11-10-easydrag-s0-spec.md section 3). Seeded flows S (8 cards,
// 7 wires), M (60, 70, 4 groups) and XL (300, 360, 20 groups) each run three scenarios on the c1d07
// sandbox that the b5 checks use: drag one card for 20 frames, one step-done run event on a step in
// the middle of the flow, and one card rename. ed.renderStats counts the render work of each
// scenario; the counts must equal scripts/easydrag-render-baseline.json exactly. A lower count is
// committed only together with a baseline update whose commit message says why.
//
// Run: node scripts/test-easydrag-render-baseline.mjs            compare with the baseline
//      node scripts/test-easydrag-render-baseline.mjs --update   write the baseline
// A direct run starts test-easydrag.mjs with AURAGO_EASYDRAG_TEST_FILTER=render baseline (that runner
// builds the sandbox and calls run(env) below); every full test-easydrag.mjs run compares as well.
import fs from 'node:fs';
import path from 'node:path';
import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const BASELINE = path.join(here, 'easydrag-render-baseline.json');
const KEYS = ['cards', 'paints', 'wires', 'wireScans', 'groups', 'allRects', 'minimap', 'edgeNear', 'align'];
const FIXTURES = [
    { name: 'S', nodes: 8, edges: 7, groups: 0, seed: 0x5e0d5001 },
    { name: 'M', nodes: 60, edges: 70, groups: 4, seed: 0x5e0d5002 },
    { name: 'XL', nodes: 300, edges: 360, groups: 20, seed: 0x5e0d5003 }
];
const GROUP_COLORS = ['neutral', 'blue', 'green', 'amber', 'rose', 'violet'];
const BASE32 = 'abcdefghijklmnopqrstuvwxyz234567';

// mulberry32 is a small seeded PRNG: the same seed gives the same fixture on every machine.
function mulberry32(seed) {
    let a = seed >>> 0;
    return () => {
        a = (a + 0x6D2B79F5) >>> 0;
        let x = a;
        x = Math.imul(x ^ (x >>> 15), x | 1);
        x ^= x + Math.imul(x ^ (x >>> 7), x | 61);
        return ((x ^ (x >>> 14)) >>> 0) / 4294967296;
    };
}

// fixtureDoc builds a flow of spec.nodes cards on a jittered grid: a manual trigger, web searches
// wired as a random tree plus random extra wires (always from an earlier to a later card, never the
// same pair twice), and one last card without wires: the drag scenario moves it, so the
// drop-on-wire search (edgeNear) runs. Groups take disjoint runs of wired cards.
function fixtureDoc(spec) {
    const rand = mulberry32(spec.seed);
    const pick = n => Math.floor(rand() * n);
    const id = i => 'n_f' + String(i).padStart(7, '0');
    const nodes = [];
    for (let i = 0; i < spec.nodes; i++) {
        const position = { x: (i % 12) * 300 + pick(40), y: Math.floor(i / 12) * 160 + pick(40) };
        nodes.push(i === 0
            ? { id: id(0), key: 'start', type: 'trigger.manual', label: 'Start', position, params: {}, settings: {} }
            : { id: id(i), key: 'step_' + i, type: 'web.search', label: 'Step ' + i, position, params: { query: 'q' + i }, settings: {} });
    }
    const wired = spec.nodes - 1;
    const edges = [];
    const pairs = new Set();
    const wire = (a, b) => {
        pairs.add(a + '>' + b);
        edges.push({ id: 'e_f' + String(edges.length + 1).padStart(5, '0'), source: { node: id(a), port: 'out' }, target: { node: id(b), port: 'in' } });
    };
    for (let b = 1; b < wired; b++) wire(pick(b), b);
    for (let tries = 0; edges.length < spec.edges && tries < 100000; tries++) {
        const a = pick(wired - 1);
        const b = a + 1 + pick(wired - 1 - a);
        if (!pairs.has(a + '>' + b)) wire(a, b);
    }
    const groups = [];
    const span = Math.floor((wired - 1) / Math.max(spec.groups, 1));
    for (let g = 0; g < spec.groups; g++) {
        const first = 1 + g * span;
        const size = 3 + pick(4);
        groups.push({ id: 'g_aaaaaaa' + BASE32[g], title: 'Group ' + (g + 1), color: GROUP_COLORS[g % GROUP_COLORS.length], members: Array.from({ length: size }, (_, k) => id(first + k)) });
    }
    const doc = { schema: 1, name: 'Baseline ' + spec.name, nodes, edges };
    if (groups.length) doc.groups = groups;
    return doc;
}

export async function run(env) {
    const { eq, check, guardAsync, settle, sandbox, openEditor } = env;
    // frames runs the queued animation frames; a frame may queue the next, 50 rounds bound it.
    const frames = h => { for (let i = 0; i < 50 && h.frames.size; i++) h.flushFrames(); };
    const settled = async h => { frames(h); await settle(); frames(h); };
    const snap = ed => Object.fromEntries(KEYS.map(key => [key, ed.renderStats[key]]));
    const delta = (a, b) => Object.fromEntries(KEYS.map(key => [key, b[key] - a[key]]));
    const middle = doc => doc.nodes[Math.floor((doc.nodes.length - 1) / 2)].id;

    // open starts the editor for doc on a fresh sandbox: draft and live revision, enabled, so that
    // Run now follows a live run (as in the b5 F6 check). Timers never fire. The view is pinned to
    // zoom 1, so that the automatic-fit floor (READABLE_ZOOM, changed in S0.4) cannot change what a
    // drag reaches or snaps to.
    async function open(doc) {
        const h = sandbox(req => (req.method === 'POST' && req.url === '/api/desktop/flows/f1/run' ? { run_id: 'r1' } : undefined));
        const editor = openEditor(h, { enabled: true, flow: { draft: doc, live: JSON.parse(JSON.stringify(doc)), live_revision: 2, published_draft_revision: 3 } });
        await settled(h);
        editor.ed.view = { x: 0, y: 0, zoom: 1 };
        return { h, editor, ed: editor.ed, canvasEl: editor.el.querySelector('.ed-canvas'), card: id => editor.el.querySelector('[data-node-id="' + id + '"]') };
    }

    // Each scenario opens its own editor and returns the counter deltas of its one action and
    // whether the action took effect, so that a scenario that did nothing cannot pass as cheap.
    const SCENARIOS = {
        async drag(doc) {
            const s = await open(doc);
            const loose = doc.nodes[doc.nodes.length - 1].id;
            const from = s.ed.model.node(loose).position.x;
            const before = snap(s.ed);
            s.card(loose).fire('pointerdown', s.h.pe(1, 100, 100));
            for (let i = 1; i <= 20; i++) {
                s.canvasEl.fire('pointermove', s.h.pe(1, 100 + i * 12, 100 + i * 5));
                frames(s.h);
            }
            s.canvasEl.fire('pointerup', s.h.pe(1, 340, 200));
            await settled(s.h);
            const out = { counts: delta(before, snap(s.ed)), acted: s.ed.model.node(loose).position.x !== from };
            s.editor.dispose();
            return out;
        },
        async runEvent(doc) {
            const s = await open(doc);
            s.editor.el.querySelector('[data-ed-cmd="run-now"]').fire('click');
            await settled(s.h);
            const source = s.h.sources[s.h.sources.length - 1];
            const id = middle(doc);
            const before = snap(s.ed);
            if (source) source.emit('event', { seq: 1, type: 'step_finished', node_id: id, step: { node_id: id, status: 'success', duration_ms: 3, item_count: 1 } });
            await settled(s.h);
            const out = { counts: delta(before, snap(s.ed)), acted: !!source && s.card(id).className.includes('status-success') };
            s.editor.dispose();
            return out;
        },
        async rename(doc) {
            const s = await open(doc);
            const id = middle(doc);
            const before = snap(s.ed);
            s.ed.model.setLabel(id, 'Renamed step');
            await settled(s.h);
            const out = { counts: delta(before, snap(s.ed)), acted: s.ed.model.node(id).label === 'Renamed step' };
            s.editor.dispose();
            return out;
        }
    };

    await guardAsync('render baseline S/M/XL counters equal scripts/easydrag-render-baseline.json', async () => {
        const update = process.env.AURAGO_EASYDRAG_RENDER_BASELINE === 'update';
        const counts = {};
        let sane = true;
        for (const spec of FIXTURES) {
            const probe = await open(fixtureDoc(spec));
            const shape = [probe.ed.model.doc.nodes.length, probe.ed.model.doc.edges.length, (probe.ed.model.doc.groups || []).length];
            probe.editor.dispose();
            counts[spec.name] = {};
            const acted = [];
            for (const [name, scenario] of Object.entries(SCENARIOS)) {
                const out = await scenario(fixtureDoc(spec));
                counts[spec.name][name] = out.counts;
                acted.push(out.acted);
            }
            const want = [[spec.nodes, spec.edges, spec.groups], [true, true, true]];
            if (JSON.stringify([shape, acted]) !== JSON.stringify(want)) sane = false;
            eq('render baseline ' + spec.name + ' opens ' + spec.nodes + ' cards, ' + spec.edges + ' wires and ' + spec.groups + ' groups, and drag, run event and rename take effect', [shape, acted], want);
        }
        if (update) {
            if (!sane) { check('render baseline not written: a fixture or a scenario failed', false); return; }
            fs.writeFileSync(BASELINE, JSON.stringify({ spec: 'plans/2026-11-10-easydrag-s0-spec.md section 3', keys: KEYS, counts }, null, 2) + '\n');
            check('render baseline written to ' + path.basename(BASELINE), true);
            return;
        }
        const stored = fs.existsSync(BASELINE) ? JSON.parse(fs.readFileSync(BASELINE, 'utf8')) : null;
        check('render baseline ' + path.basename(BASELINE) + ' exists (node scripts/test-easydrag-render-baseline.mjs --update writes it)', !!stored);
        if (!stored) return;
        for (const spec of FIXTURES) eq('render baseline ' + spec.name + ' counters equal the baseline', counts[spec.name], stored.counts[spec.name]);
    });
}

if (path.basename(process.argv[1] || '') === 'test-easydrag-render-baseline.mjs') {
    const update = process.argv.includes('--update');
    const child = spawnSync(process.execPath, [path.join(here, 'test-easydrag.mjs')], {
        env: Object.assign({}, process.env, { AURAGO_EASYDRAG_TEST_FILTER: 'render baseline', AURAGO_EASYDRAG_RENDER_BASELINE: update ? 'update' : 'check' }),
        encoding: 'utf8',
        maxBuffer: 256 * 1024 * 1024
    });
    const lines = String(child.stdout || '').split(/\r?\n/);
    lines.filter(line => line.includes('render baseline') || line.startsWith('FAIL')).forEach(line => console.log(line));
    const passed = lines.filter(line => line.startsWith('ok   render baseline')).length;
    const want = update ? 4 : 7;
    if (child.status !== 0 || lines.some(line => line.startsWith('FAIL')) || passed !== want) {
        console.log(String(child.stderr || '').split(/\r?\n/).slice(-20).join('\n'));
        console.log('FAIL render baseline: exit ' + child.status + ', ' + passed + ' of ' + want + ' checks passed' + (child.error ? ' (' + child.error.message + ')' : ''));
        process.exit(1);
    }
    console.log('PASS render baseline (' + (update ? 'written to ' + path.basename(BASELINE) : 'unchanged') + ')');
}
