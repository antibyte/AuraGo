// Batch D5: storyboard beats that show icons, diagrams, charts, headlines and lists, and the highlight of a diagram node.
import assert from 'node:assert/strict';
import { plain } from '../storymaker-test-support.mjs';
import { t } from '../storymaker-test-context.mjs';

const { check, near, catalog } = t;
const SM = t.core();

const flow = { layout: 'flow', flow: true, nodes: [{ id: 'req', label: 'Anfrage', icon: 'icon.send', cue: 1 }, { id: 'srv', label: 'Server', icon: 'icon.server', cue: 2 }] };
const BEATS = [
    { do: 'show', icon: 'icon.cpu', style: 'card', text: 'CPU', at: 'board', entry: 'draw' },
    { do: 'show', diagram: flow, at: 'full', entry: 'draw' },
    { do: 'show', chart: 'bar', data: { labels: ['Warten', 'Rechnen'], values: [90, 10], unit: '%', title: 'Zeit' }, at: 'full', entry: 'fade' },
    { do: 'show', text: 'Kleine Modelle sind *schnell*', style: 'headline', at: 'top', entry: 'fade' },
    { do: 'show', text: 'Laden\nRechnen\nAntworten', style: 'list', at: 'right', entry: 'fade' }
];
const compileScene = (beats, extra = {}) => SM.compile({ schema: 'storymaker/1', cast: [], scenes: [{ id: 'a', narration: 'Die Anfrage {1} geht zum Server {2} und zurueck.', ...extra, beats }] }, {}, catalog);
const blocksOf = out => out.timeline.scenes[0].entities.filter(e => e.type === 'block');
const issuesOf = (out, code) => plain(out.report.issues).filter(i => !code || i.code === code);

check('block beats: every show kind makes one block with its kind and area', () => {
    const out = compileScene(BEATS), blocks = blocksOf(out);
    assert.deepEqual(plain(blocks.map(b => b.block.kind)), ['icon', 'diagram', 'chart', 'headline', 'list']);
    assert.deepEqual(plain(blocks.map(b => b.id)), ['icon-cpu-1', 'diagram-1', 'chart-1', 'text-1', 'text-2']);
    const stage = SM.stage.create({ aspect: '16:9', env: null });
    ['board', 'full', 'full', 'top', 'right'].forEach((at, i) => {
        const a = SM.blocks.area(stage, null, 'day', at);
        near(blocks[i].w, a.w, 1e-6); near(blocks[i].h, a.h, 1e-6);
    });
    assert.deepEqual(issuesOf(out).filter(i => i.severity !== 'minor' && i.code !== 'narration_missing'), []);
    assert.ok(blocks.every(b => b.alpha.at(60) === 1), 'every block is visible at the end');
    assert.ok(blocks.every(b => b.alpha.init === 0), 'and hidden before its beat');
});

check('block beats: the node cues become the reveal times, without cues the player spreads the nodes', () => {
    const out = compileScene([BEATS[1]]), e = blocksOf(out)[0], cues = out.timeline.scenes[0].narration.cueList;
    assert.equal(e.reveals.length, 2);
    assert.ok(e.reveals[1] > e.reveals[0]);
    assert.equal(cues.length, 2);
    cues.forEach((c, k) => near(e.reveals[k], out.timeline.scenes[0].narration.start + c.at_ms / 1000, 1e-3, 'node ' + k + ' appears on its marker'));
    const plainFlow = { layout: 'flow', nodes: [{ id: 'a', label: 'A' }, { id: 'b', label: 'B' }] };
    const spread = blocksOf(compileScene([{ do: 'show', diagram: plainFlow, at: 'full', entry: 'draw' }]))[0];
    assert.equal(spread.reveals, null);
    near(spread.dur, SM.blocks.revealSeconds(spread.block), 1e-9);
    const none = blocksOf(compileScene([{ do: 'show', diagram: plainFlow, at: 'full', entry: 'none' }]))[0];
    assert.equal(none.dur, 0);
});

check('block beats: highlight with diagram.node marks the node, the legacy chart beat plays as a fade', () => {
    const out = compileScene([BEATS[1], { do: 'highlight', target: 'diagram-1.srv', style: 'pulse' },
        { do: 'chart', chart: 'bar', data: { labels: ['a'], values: [1] }, at: 'right' }, { do: 'highlight', target: 'diagram-1.zzz' }]);
    const [d, c] = blocksOf(out);
    assert.equal(d.marks.list.length, 1);
    assert.equal(d.marks.list[0].data.node, 'srv');
    assert.equal(c.block.kind, 'chart');
    assert.equal(c.entry, 'fade');
    const bad = issuesOf(out, 'unknown_name');
    assert.equal(bad.length, 1);
    assert.equal(bad[0].path, '/scenes/0/beats/3/target');
});

check('block beats: the needs hold the block fonts and a broken block is reported and skipped', () => {
    const fonts = [...catalog.fonts, { name: 'default', family: 'Storymaker Default', weight: 700, file: '/fonts/placeholder-bold.woff2', licence: 'OFL-1.1' }];
    const out = SM.compile({ schema: 'storymaker/1', cast: [], scenes: [{ id: 'a', beats: BEATS }] }, {}, { ...catalog, fonts });
    assert.ok(plain(out.needs).includes('font:default/700'), 'the 700 face');
    const broken = compileScene([{ do: 'show', icon: 'icon.nope', at: 'full', entry: 'draw' }, { do: 'show', chart: 'bar', at: 'full', entry: 'fade' },
        { do: 'chart', chart: 'bar', data: { labels: [], values: [] }, at: 'right' }, { do: 'show', icon: 'icon.cpu', at: 'middle', entry: 'pop' }]);
    assert.equal(blocksOf(broken).length, 1, 'only the icon at an unknown area is kept, at full');
    const codes = issuesOf(broken).filter(i => i.path.includes('/beats/')).map(i => i.path.replace('/scenes/0/beats/', '') + ' ' + i.code);
    assert.deepEqual(codes, ['0/icon unknown_asset', '1/data missing_field', '2/data invalid_value', '3/at unknown_spot']);
});
