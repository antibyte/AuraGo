// Batch E: the blocks API of a code scene (api.icon, api.diagram, api.chart, api.headline, api.list) and the board tokens.
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import vm from 'node:vm';
import { root, plain, FakeCanvas, loadPlayer } from '../storymaker-test-support.mjs';
import { t } from '../storymaker-test-context.mjs';

const { check, near, catalog, files, firewall, bundleScene } = t;
const SM = t.core();

// The mini catalog has the default font at weight 400 only; the real one also has 600 and 700.
const fonts = [400, 600, 700].map(weight => ({ name: 'default', family: 'Storymaker Default', weight, file: '/fonts/placeholder.woff2', licence: 'OFL-1.1' }));
const fullCatalog = { ...catalog, fonts };

function runScene(source, extra) {
    const ctx = loadPlayer();
    vm.runInContext(bundleScene(source, 'x'), ctx);
    const C = ctx.StorymakerCore, story = extra && extra.story || { ...firewall, scenes: [{ id: 'm', narration: 'eins {1}zwei {2}drei' }] };
    const plan = C.planTiming(story, {}, fullCatalog), id = extra && extra.id || 'm';
    const opts = { sceneId: id, index: plan.scenes[id].index, catalog: fullCatalog, aspect: extra && extra.aspect || '16:9', width: 1920, height: 1080, fps: 30, cues: plan.scenes[id].cues, palette: plan.palette, families: plan.fontFamilies, styleFont: 'default', safe: catalog.stage.safe, environment: plan.scenes[id].environment, variant: plan.scenes[id].variant };
    return { C, ctx, build: () => C.codescene.build(ctx.self.__smScene, opts) };
}
const build = source => runScene(source).build();
const fail = source => { try { build(source); } catch (e) { return String(e.message); } return ''; };

check('api.icon: a block in the right area with handle methods and the 600 and 700 fonts', () => {
    const built = build(`export default function scene(api) {
      const i = api.icon('icon.cpu', { at: 'right', style: 'card', label: 'CPU', start: 1 });
      i.fadeOut({ start: 3, dur: 1 }).moveTo('left', { start: 0, dur: 1 }).scaleTo(2, { start: 0, dur: 1 });
    }`);
    const [e] = built.entities, a = SM.blocks.area(built.stage, null, 'day', 'right');
    assert.equal(built.entities.length, 1);
    assert.equal(e.type, 'block');
    assert.equal(e.block.kind, 'icon'); assert.equal(e.block.icon, 'icon.cpu'); assert.equal(e.block.style, 'card'); assert.equal(e.block.label, 'CPU');
    assert.equal(e.entry, 'draw'); assert.equal(e.t0, 1); near(e.dur, 0.8, 1e-9);
    near(e.w, a.w, 1e-9); assert.equal(e.surface, 'panel');
    near(e.alpha.at(4), 0, 1e-9); near(e.alpha.at(2), 1, 1e-9);
    near(e.x.at(1), built.stage.resolve('left').x, 1e-6); near(e.scale.at(1), 2, 1e-9);
    assert.ok(plain(built.needs).includes('font:default/600') && plain(built.needs).includes('font:default/700'), plain(built.needs).join());
    const dflt = build("export default function scene(api) { api.icon('icon.cpu'); }").entities[0];
    near(dflt.w, SM.blocks.area(built.stage, null, 'day', 'center').w, 1e-9);
    assert.equal(dflt.entry, 'draw'); assert.equal(dflt.t0, 0);
});

check('api.icon: unknown ids, areas and entries throw a RangeError that names the fix', () => {
    assert.match(fail("export default function scene(api) { api.icon('icon.nope'); }"), /icon\.nope.*storymaker_catalog/);
    assert.match(fail("export default function scene(api) { api.icon('icon.cpu', { at: 'nowhere' }); }"), /"nowhere".*full, left, right, top, center, board/);
    assert.match(fail("export default function scene(api) { api.icon('icon.cpu', { entry: 'zoom' }); }"), /"zoom".*draw, pop, fade, slide, none/);
    assert.match(fail("export default function scene(api) { api.icon('icon.cpu', { start: -1 }); }"), /start must be 0 or more/);
});

const nodes = n => '[' + Array.from({ length: n }, (_, i) => `{ id: '${String.fromCharCode(97 + i)}', label: 'Node ${i}' }`).join(', ') + ']';

check('api.diagram: cue reveals are stored, highlight adds a node mark, there is no not_supported_yet issue', () => {
    const { C, build: run } = runScene(`export default function scene(api) {
      const d = api.diagram({ layout: 'flow', flow: true, nodes: ${nodes(3)} }, { reveals: [api.cue(1), api.cue(2), 3] });
      const same = d.highlight('b', { start: 3 });
      api.text(String(same === d), 'title');
    }`);
    const built = run(), [e] = built.entities, cues = C.planTiming({ ...firewall, scenes: [{ id: 'm', narration: 'eins {1}zwei {2}drei' }] }, {}, fullCatalog).scenes.m.cues;
    assert.equal(e.type, 'block'); assert.equal(e.block.kind, 'diagram'); assert.equal(e.block.diagram.flow, true);
    assert.deepEqual(plain(e.reveals), [cues[1], cues[2], 3]);
    assert.equal(e.entry, 'draw'); near(e.dur, 0.5 + 0.6 * 3, 1e-9);
    assert.equal(built.entities[1].text, 'true', 'highlight returns the handle, so calls chain');
    assert.deepEqual(plain(e.marks.list), [{ t0: 3, t1: 4.2, data: { node: 'b', style: 'pulse' } }]);
    assert.deepEqual(plain(built.issues), []);
    const st = C.scene.evalEntity(e, 3.5);
    assert.deepEqual(plain(st.mark.node), 'b');
    assert.ok(C.scene.entityEnd(e) >= 4.2);
});

check('api.diagram: highlight and the layout limits name the fix', () => {
    assert.match(fail(`export default function scene(api) { api.diagram({ layout: 'flow', nodes: ${nodes(2)} }).highlight('zzz'); }`), /"zzz".*a, b/);
    assert.match(fail("export default function scene(api) { api.icon('icon.cpu').highlight('a'); }"), /highlight.*diagram/);
    assert.match(fail(`export default function scene(api) { api.diagram({ layout: 'flow', nodes: ${nodes(6)} }); }`), /flow diagram takes 2 to 5 nodes, got 6/);
    assert.match(fail(`export default function scene(api) { api.diagram({ layout: 'ring', nodes: ${nodes(3)} }); }`), /"ring".*flow, cycle, hub, stack, compare/);
    assert.match(fail(`export default function scene(api) { api.diagram({ layout: 'flow', nodes: ${nodes(2)} }, { reveals: [1] }); }`), /reveals needs one finite number per node/);
    assert.match(fail("export default function scene(api) { api.diagram({ layout: 'flow', nodes: [{ id: 'a', label: 'A', icon: 'icon.nope' }, { id: 'b', label: 'B' }] }); }"), /icon\.nope.*storymaker_catalog/);
    assert.equal(fail(`export default function scene(api) { api.diagram({ nodes: ${nodes(3)} }); }`), '', 'the layout defaults to flow');
});

check('api.chart: a counter chart is a block; bad values name the fix', () => {
    const e = build("export default function scene(api) { api.chart({ type: 'counter', labels: ['Users'], values: [42], unit: '%', title: 'Growth' }); }").entities[0];
    assert.equal(e.block.kind, 'chart'); assert.equal(e.block.chart.values[0], 42); assert.equal(e.entry, 'fade'); near(e.dur, 1.2, 1e-9);
    assert.match(fail("export default function scene(api) { api.chart({ type: 'bar', labels: ['a'], values: [1, 2] }); }"), /same length/);
    assert.match(fail("export default function scene(api) { api.chart({ type: 'donut', labels: [], values: [] }); }"), /"donut".*bar, line, pie, counter/);
    assert.match(fail("export default function scene(api) { api.chart('bar', { labels: ['a'], values: [1] }); }"), /chart \{type, labels, values\}/);
});

const frameLog = (built, C, time, extra = {}) => {
    const cv = new FakeCanvas(1920, 1080);
    const tl = { aspect: '16:9', fps: 30, palette: { bg: '#fff', bg2: '#eee', fg: '#123456', accent: '#f00', accent2: '#0f0', text: '#000' }, cat: C.scene.indexCatalog(fullCatalog), fontFamilies: { default: 'F' }, style: { font: 'default' }, safe: catalog.stage.safe };
    C.codescene.drawFrame(built, time, cv.ctx, C.render.makeContext(tl, { rig: files.rig, props: files.props, envs: files.envs }, 1920, 1080));
    return cv.ctx.log;
};

check('api.headline and api.list: blocks in the full area; lists take at most 4 lines; drawing is the same at the same time', () => {
    const { C, build: run } = runScene(`export default function scene(api) {
      api.headline('Kleine Modelle sind *schnell*');
      api.list(['eins', 'zwei', 'drei'], { at: 'right', reveals: [api.cue(1), api.cue(2), 3] });
    }`);
    const built = run(), [h, l] = built.entities, full = SM.blocks.area(built.stage, null, 'day', 'full');
    assert.equal(h.block.kind, 'headline'); assert.equal(h.block.text, 'Kleine Modelle sind *schnell*'); near(h.w, full.w, 1e-9);
    assert.equal(h.entry, 'fade'); near(h.dur, 0.4 + 0.15 * 4, 1e-9);
    assert.equal(l.block.kind, 'list'); assert.equal(l.block.text, 'eins\nzwei\ndrei'); assert.equal(l.reveals.length, 3);
    assert.match(fail("export default function scene(api) { api.list(['a', 'b', 'c', 'd', 'e']); }"), /1 to 4 lines, got 5/);
    assert.match(fail("export default function scene(api) { api.list('a'); }"), /api\.list needs an array/);
    assert.match(fail("export default function scene(api) { api.headline(''); }"), /headline has 1 to 80/);
    const a = frameLog(built, C, 1.0), b = frameLog(built, C, 1.0);
    assert.ok(a.length > 20 && a.join('\n') === b.join('\n'));
});

check('free-motion.js: the reference example builds in every aspect, its blocks do not overlap and the frames repeat', () => {
    const source = fs.readFileSync(path.join(root, 'internal/storymaker/reference/free-motion.js'), 'utf8');
    for (const aspect of ['16:9', '9:16', '1:1']) {
        const { C, build: run } = runScene(source, { aspect });
        const built = run();
        assert.equal(built.duration, 8);
        assert.deepEqual(plain(built.issues), []);
        assert.deepEqual(plain(built.entities.map(e => e.block.kind)), ['headline', 'diagram']);
        const rc = C.render.makeContext({ aspect, fps: 30, palette: { bg: '#fff', bg2: '#eee', fg: '#123456', accent: '#f00', accent2: '#0f0', text: '#000' }, cat: C.scene.indexCatalog(fullCatalog), fontChain: null, fontFamilies: { default: 'F' }, style: { font: 'default' }, safe: catalog.stage.safe }, { rig: files.rig, props: files.props, envs: files.envs }, 1920, 1080);
        const [h, d] = built.entities.map(e => C.blocks.measure(new FakeCanvas(1920, 1080).ctx, C.scene.evalEntity(e, 7), rc, built.stage, built.stage.baseWindow()).box);
        assert.ok(h.y + h.h <= d.y + 1e-6 || d.y + d.h <= h.y, aspect + ': headline ' + JSON.stringify(h) + ' overlaps the diagram ' + JSON.stringify(d));
        for (const time of [0.5, 3, 7]) assert.equal(frameLog(built, C, time).join('\n'), frameLog(run(), C, time).join('\n'));
    }
});

check('code scenes on env.board resolve $text and $bg from the board tokens, then from the palette', () => {
    const source = "export default function scene(api) { api.duration(2); api.shape('rect', { at: 'center', w: 20, h: 20, fill: '$bg' }); api.onFrame((t, g) => { g.fill('$text'); g.rect(1, 1, 2, 2); g.text('x', 5, 5, { color: '$text' }); g.fill('$accent'); g.rect(3, 3, 2, 2); }); }";
    const fills = (environment, variant) => {
        const story = { ...firewall, scenes: [{ id: 's', kind: 'code', environment, variant }] };
        const { C, build: run } = runScene(source, { story, id: 's' });
        return frameLog(run(), C, 0.5).filter(l => l.startsWith('set fillStyle='));
    };
    const night = fills('env.board', 'night'), day = fills('env.board', 'day'), other = fills('env.office', 'night');
    assert.ok(night.includes('set fillStyle=#f4f5eb'), 'night $text: ' + night.join());
    assert.ok(night.includes('set fillStyle=#1f2a44'), 'night $bg of the shape: ' + night.join());
    assert.ok(night.includes('set fillStyle=#f00'), 'a token the board does not carry comes from the palette');
    assert.ok(day.includes('set fillStyle=#1f2a44') && !day.includes('set fillStyle=#f4f5eb'), 'day $text: ' + day.join());
    assert.ok(day.includes('set fillStyle=#fff'), 'day $bg stays the palette bg: ' + day.join());
    assert.ok(other.includes('set fillStyle=#000') && !other.includes('set fillStyle=#f4f5eb'), 'without env.board $text is the palette text: ' + other.join());
});

// A code-scene block behaves like a storyboard block: nothing of it draws before start, then its entry plays.
function blockDrawOps(source, time) {
    const { C, build: run } = runScene(source), built = run(), e = built.entities[0], st = C.scene.evalEntity(e, time);
    const cv = new FakeCanvas(1920, 1080);
    const rc = C.render.makeContext({ width: 1920, height: 1080, aspect: '16:9', palette: { bg: '#fff', bg2: '#eee', fg: '#111', text: '#111', accent: '#f60', accent2: '#0cc' }, fontFamilies: { default: 'x' }, styleFont: 'default', style: {} }, {}, 1920, 1080);
    C.blocks.draw(cv.ctx, st, Object.assign({}, rc, { cat: C.scene.indexCatalog(fullCatalog) }), built.stage, built.stage.baseWindow());
    return { ops: (cv.ctx.log || []).filter(l => /^(fill|stroke|fillText|fillRect|arc)/.test(l)).length, alpha: st.alpha, entity: e };
}

check('code-scene blocks are hidden before start and play their entry from start', () => {
    const sources = {
        'icon fade': "export default function scene(api) { api.duration(8); api.icon('icon.cpu', { start: 5, entry: 'fade' }); }",
        'icon none': "export default function scene(api) { api.duration(8); api.icon('icon.cpu', { start: 5, entry: 'none' }); }",
        'icon pop': "export default function scene(api) { api.duration(8); api.icon('icon.cpu', { start: 5, entry: 'pop' }); }",
        'icon slide': "export default function scene(api) { api.duration(8); api.icon('icon.cpu', { start: 5, entry: 'slide' }); }",
        'icon draw': "export default function scene(api) { api.duration(8); api.icon('icon.cpu', { start: 5, entry: 'draw' }); }",
        'chart title': "export default function scene(api) { api.duration(8); api.chart({ type: 'bar', labels: ['a', 'b'], values: [1, 2], title: 'T' }, { start: 5 }); }",
        'counter': "export default function scene(api) { api.duration(8); api.chart({ type: 'counter', labels: ['users'], values: [42], unit: '%' }, { start: 5 }); }"
    };
    for (const [name, source] of Object.entries(sources)) {
        const before = blockDrawOps(source, 1);
        assert.equal(before.ops, 0, name + ' draws ' + before.ops + ' ops before start');
        assert.equal(before.alpha, 0, name + ' alpha before start');
        const e = before.entity, end = blockDrawOps(source, e.t0 + e.dur + 0.01);
        near(end.alpha, 1, 1e-9);
        assert.ok(end.ops > 0, name + ' draws nothing after its entry');
    }
    const mid = blockDrawOps(sources['icon fade'], 5.4);
    assert.ok(mid.alpha > 0 && mid.alpha < 1, 'the fade is in progress: ' + mid.alpha);
});
