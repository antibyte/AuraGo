import assert from 'node:assert/strict';
import vm from 'node:vm';
import { read, loadPlayer, FakeCanvas, legacyCatalog, legacyFiles, plain } from '../storymaker-test-support.mjs';
import { t } from '../storymaker-test-context.mjs';

t.check('embedded agent effects example runs with GSAP and seeks deterministically in every aspect', () => {
    const catalog = legacyCatalog(), files = legacyFiles();
    const source = read('internal/storymaker/reference/effects.js');
    for (const aspect of ['16:9', '9:16', '1:1']) {
        const boot = () => {
            const ctx = loadPlayer({ navigator: { userAgent: 'Storymaker reference test' } });
            vm.runInContext(read('ui/js/vendor/storymaker/runtime.js'), ctx);
            vm.runInContext(t.bundleScene(source, 'agent-example'), ctx);
            const C = ctx.StorymakerCore;
            const plan = C.planTiming({ schema: 'storymaker/1', format: { aspect, fps: 30 }, cast: [], scenes: [{ id: 's1', kind: 'code' }] }, {}, catalog);
            const built = C.codescene.build(ctx.self.__smScene, {
                sceneId: 's1', index: 0, catalog, aspect, width: 1920, height: 1080, fps: 30,
                cues: {}, palette: plan.palette, families: plan.fontFamilies, styleFont: 'default', safe: catalog.stage.safe
            });
            assert.equal(built.duration, 8);
            assert.deepEqual(plain(built.issues), []);
            assert.equal(built.entities.length, 3);
            const tl = { aspect, fps: 30, palette: plan.palette, cat: C.scene.indexCatalog(catalog),
                fontFamilies: plan.fontFamilies, style: { font: 'default' }, safe: catalog.stage.safe };
            const rc = C.render.makeContext(tl, files, 1920, 1080);
            return seconds => {
                const canvas = new FakeCanvas(1920, 1080);
                C.codescene.drawFrame(built, seconds, canvas.ctx, rc);
                return canvas.ctx.log.join('\n').replace(/canvas\d+/g, 'canvas');
            };
        };
        const draw = boot(), fresh = boot();
        const times = [0, 0.5, 1.7, 3, 4.5, 7.9];
        const expected = times.map(draw);
        assert.notEqual(expected[0], expected[3], 'the effect must move');
        for (const index of [5, 4, 3, 2, 1, 0, 3, 0, 5, 2, 4, 1]) {
            assert.equal(draw(times[index]), expected[index], aspect + ' reverse/random seek');
            assert.equal(fresh(times[index]), expected[index], aspect + ' restarted scene');
        }
    }
});
