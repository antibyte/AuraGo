// Batch E: the blocks API of a code scene (api.icon, api.diagram, api.chart, api.headline, api.list) and the board tokens.
import assert from 'node:assert/strict';
import vm from 'node:vm';
import { plain, FakeCanvas, loadPlayer } from '../storymaker-test-support.mjs';
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
