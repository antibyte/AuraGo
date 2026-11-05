// Task C7: the code-scene API on the shared fixture s3.js, the restricted onFrame facade, the determinism shadows.
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import vm from 'node:vm';
import { root, read, plain, FakeCanvas, loadPlayer, makeContext } from '../storymaker-test-support.mjs';
import { t } from '../storymaker-test-context.mjs';

const { check, near, catalog, files, firewall, bundleScene, s3Source } = t;
const SM = t.core();

function runScene(source, name, extra) {
    const ctx = loadPlayer();
    vm.runInContext(bundleScene(source, name || 'x'), ctx);
    const C = ctx.StorymakerCore;
    const plan = C.planTiming(extra && extra.story || firewall, {}, catalog);
    const sceneId = extra && extra.id || 's3';
    const opts = { sceneId, index: plan.scenes[sceneId].index, catalog, aspect: '16:9', width: 1920, height: 1080, fps: 30, cues: plan.scenes[sceneId].cues, palette: plan.palette, families: plan.fontFamilies, styleFont: 'default', safe: catalog.stage.safe };
    return { ctx, C, build: () => C.codescene.build(ctx.self.__smScene, opts) };
}

check('code scene API on the shared fixture s3.js', () => {
    const { C, build } = runScene(s3Source, 's3');
    const built = build();
    assert.equal(built.duration, 5);
    assert.deepEqual(plain(built.needs), ['catalog:rig/human.json', 'font:default/400']);
    assert.deepEqual(plain(built.issues), []);
    const [anna, label] = built.entities;
    assert.equal(anna.type, 'actor'); assert.equal(label.type, 'text');
    near(anna.x.at(0), 0.30 * 1920, 1e-9, 'at: left');
    assert.equal(C.scene.evalEntity(anna, 0.5).gesture, null);
    const g = C.scene.evalEntity(anna, 1.6);
    assert.equal(g.gesture.name, 'point'); near(g.gesture.frac, 0.5, 1e-9);
    assert.equal(C.scene.evalEntity(anna, 2.3).gesture, null, 'the gesture is over after its 1.2 s');
    assert.equal(C.scene.evalEntity(anna, 2.9).expression, 'neutral');
    assert.equal(C.scene.evalEntity(anna, 3.1).expression, 'happy');
    near(C.scene.evalEntity(label, 0).alpha, 1, 1e-9);
    // the tween on opacity runs from the current value (1) to 1: no change; a fade-in from 0 is a tween with `from`
    const more = runScene(`export default function scene(api) {
      const t = api.text('Hi', 'title', { at: 'right' });
      t.fadeIn({ start: 1, dur: 1 });
      const box = api.shape('rect', { at: { x: 50, y: 50 }, w: 200, h: 100 });
      api.tween(box, { x: 80, scale: 2 }, { start: 0, dur: 2, ease: 'linear' });
      const lock = api.add('prop.lock', { at: 'center', layer: 'back' });
      lock.moveTo('left', { start: 1, dur: 1 }).scaleTo(2, { start: 1, dur: 1 }).rotateTo(90, { start: 0, dur: 1, ease: 'linear' });
      const c = api.chart('bar', { labels: [], values: [] }, { at: 'right' });
      api.tween(c, { grow: 1 }, { start: api.cue(1), dur: 2 });
      api.camera.zoom(1.5, { start: 0, dur: 1 });
    }`, 'more', { story: { ...firewall, scenes: [{ id: 'm', narration: 'eins {1}zwei' }] }, id: 'm' }).build();
    const [text, box, lock] = more.entities;
    assert.equal(more.entities.length, 3, 'the inert chart is not an entity');
    near(text.alpha.at(0.5), 0, 1e-9); near(text.alpha.at(2), 1, 1e-9);
    near(box.x.at(2), 0.8 * 1920, 1e-9); near(box.x.at(1), (0.5 + 0.8) / 2 * 1920, 1e-6); near(box.scale.at(2), 2, 1e-9);
    near(lock.rot.at(0.5), 45, 1e-9); near(lock.scale.at(2), 2, 1e-9);
    assert.equal(lock.layer, 'back');
    assert.deepEqual(plain(more.issues.map(i => i.code)), ['not_supported_yet', 'not_supported_yet'], 'api.chart and the grow tween report not_supported_yet');
    near(more.camera.zoom.at(1), 1.5, 1e-9);
    assert.equal(more.duration, Math.max(2, 1.3) + 0.5, 'without api.duration: the end of the last motion + 0.5 s');
});

check('code scene API: api.random is seeded, api.cue maps to cue times, errors name the fix', () => {
    const { C } = runScene('export default function scene() {}', 'e');
    const r1 = C.codescene.random(42), r2 = C.codescene.random(42), r3 = C.codescene.random('42');
    const a = [r1(), r1(), r1()], b = [r2(), r2(), r2()];
    assert.deepEqual(a, b); assert.ok(a.every(v => v >= 0 && v < 1));
    assert.notDeepEqual(a, [C.codescene.random(43)(), 0, 0].slice(0, 1).concat(a.slice(1)));
    assert.equal(r3(), C.codescene.random('42')());
    const err = src => { try { runScene(src, 'err', { story: { ...firewall, scenes: [{ id: 'm', narration: 'eins {1}zwei' }] }, id: 'm' }).build(); } catch (e) { return String(e.message); } return ''; };
    assert.match(err("export default function s(api) { api.add('char.nobody'); }"), /Unknown catalog id "char.nobody"/);
    assert.match(err("export default function s(api) { api.add('char.office-woman-1').pose('flying'); }"), /Unknown pose "flying"\. Available: .*stand/);
    assert.match(err("export default function s(api) { api.cue(9); }"), /no cue marker \{9\}/);
    assert.match(err("export default function s(api) { api.duration(9999); }"), /between 0 and 600/);
    assert.match(err("export default function s(api) { api.tween({}, { opacity: 1 }); }"), /needs a handle/);
    assert.match(err("export default function s(api) { const t = api.text('x'); api.tween(t, { weird: 1 }); }"), /Unknown tween property "weird"/);
    assert.match(err("export default function s(api) { const t = api.text('x'); t.fadeIn({ start: 'soon' }); }"), /start must be a number/);
    assert.match(err("export default function s(api) { api.text('x', 'huge'); }"), /Unknown text style "huge"/);
    assert.match(err("export default function s(api) { api.add('env.office'); }"), /api.add takes a character or a prop/);
    assert.equal(err("export default function s(api) { api.cue(1); }"), '');
});

check('seed: the generator moved to SM.seed without changing a code scene, and takeIndex never repeats the last take', () => {
    // The generator exactly as it stood in codescene.js before the move. It stays here as the reference.
    const before = seed => {
        const s = String(seed === undefined ? 0 : seed);
        let h = 2166136261;
        for (let i = 0; i < s.length; i++) { h ^= s.charCodeAt(i); h = Math.imul(h, 16777619); }
        let a = h >>> 0;
        return () => {
            a = (a + 0x6D2B79F5) | 0;
            let x = Math.imul(a ^ (a >>> 15), 1 | a);
            x = (x + Math.imul(x ^ (x >>> 7), 61 | x)) ^ x;
            return ((x ^ (x >>> 14)) >>> 0) / 4294967296;
        };
    };
    for (const seed of [undefined, 0, 42, '42', 's1:0:sfx.pop', 'x'.repeat(40)]) {
        const ref = before(seed), viaSeed = SM.seed.random(seed), viaScene = SM.codescene.random(seed);
        for (let i = 0; i < 16; i++) {
            const want = ref();
            assert.equal(viaSeed(), want);
            assert.equal(viaScene(), want);
        }
    }
    // Published FNV-1a 32-bit test vectors.
    assert.equal(SM.seed.hash(''), 0x811C9DC5);
    assert.equal(SM.seed.hash('a'), 0xE40C292C);
    // A scene that draws from api.random still gets the same picture: x is percent / 100 * 1920.
    const { build } = runScene(`export default function scene(api) {
      const box = api.shape('rect', { at: { x: 50, y: 50 }, w: 200, h: 100 });
      api.tween(box, { x: api.random(7)() * 100 }, { start: 0, dur: 1, ease: 'linear' });
    }`, 'seeded');
    near(build().entities[0].x.at(1), before(7)() * 1920, 1e-6, 'x from api.random');
    // takeIndex: the rule of frozen section 4.2, as a property over many keys.
    for (const count of [2, 3, 4]) {
        for (let key = 0; key < 50; key++) {
            const fresh = SM.seed.takeIndex('s1', 3, 'sfx.pop', key, count, -1);
            assert.ok(fresh >= 0 && fresh < count, 'in range');
            assert.equal(SM.seed.takeIndex('s1', 3, 'sfx.pop', key, count, fresh), (fresh + 1) % count, 'never the last take');
        }
    }
    assert.equal(SM.seed.takeIndex('s1', 3, 'sfx.pop', 0, 1, 0), 0, 'a sound with one take repeats it');
});

check('code scene frames: the onFrame hook draws through a restricted facade', () => {
    const { C, build } = runScene(`export default function scene(api) {
      api.duration(2);
      api.onFrame((t, g) => { g.fill('$accent'); g.rect(10 + t * 100, 20, 30, 40); g.circle(5, 5, 3); g.text('t=' + t.toFixed(1), 100, 100, { size: 20 }); g.path('M0,0 L5,5', 'stroke'); });
    }`, 'hook');
    const built = build();
    const cv = new FakeCanvas(1920, 1080);
    const tl = { aspect: '16:9', fps: 30, palette: { bg: '#fff', bg2: '#eee', accent: '#f00', text: '#000' }, cat: C.scene.indexCatalog(catalog), fontFamilies: { default: 'F' }, style: { font: 'default' }, safe: catalog.stage.safe };
    const rc = C.render.makeContext(tl, { rig: files.rig, props: files.props, envs: files.envs }, 1920, 1080);
    C.codescene.drawFrame(built, 0.5, cv.ctx, rc);
    const log = cv.ctx.log.join('\n');
    assert.match(log, /set fillStyle=#f00/); assert.match(log, /fillRect\(60,20,30,40\)/); assert.match(log, /fillText\(t=0.5,100,100\)/);
    assert.match(log, /arc\(5,5,3,0,6.283/); assert.match(log, /stroke\(path\(M0,0 L5,5\)\)/);
    // the facade is a frozen set of drawing verbs: no canvas, no context, no way to reach the page
    const probe = runScene('export default function scene(api) { api.onFrame((t, g) => { globalThis.__facade = g; g.rect(NaN, 0, 1, 1); }); }', 'probe');
    assert.throws(() => C.codescene.drawFrame(probe.build(), 0, cv.ctx, rc), /x must be a finite number/);
    const g = probe.ctx.__facade;
    assert.ok(Object.isFrozen(g));
    assert.deepEqual(Object.keys(g).sort(), ['alpha', 'circle', 'fill', 'line', 'path', 'rect', 'restore', 'rotate', 'save', 'scale', 'stroke', 'strokeRect', 'text', 'translate']);
    assert.equal(g.canvas, undefined); assert.equal(g.ctx, undefined); assert.equal(g.fillStyle, undefined);
});

check('determinism shadows: Date, Math.random, performance.now, crypto, timers throw a helpful error', () => {
    const calls = [];
    const g = {
        Date, Math: { random: () => 0.5 }, performance: { now: () => 1 }, crypto: { getRandomValues: a => a, randomUUID: () => 'x' },
        setTimeout: () => calls.push('t'), setInterval: () => calls.push('i'), requestAnimationFrame: () => calls.push('r')
    };
    const done = SM.determinism.install(g);
    assert.deepEqual(plain(done).sort(), ['Date', 'Math.random', 'crypto.getRandomValues', 'crypto.randomUUID', 'performance.now', 'requestAnimationFrame', 'setInterval', 'setTimeout'].sort());
    assert.throws(() => g.Math.random(), /Math\.random\(\) is not available.*api\.random\(seed\)/);
    assert.throws(() => g.Date.now(), /Date\.now\(\)/);
    assert.throws(() => new g.Date(), /new Date\(\)/);
    assert.throws(() => g.performance.now(), /performance\.now\(\)/);
    assert.throws(() => g.crypto.getRandomValues(new Uint8Array(2)), /getRandomValues/);
    assert.throws(() => g.setTimeout(() => {}), /no timers/);
    assert.throws(() => g.setInterval(() => {}), /setInterval/);
    assert.throws(() => g.requestAnimationFrame(() => {}), /requestAnimationFrame/);
    assert.equal(new g.Date(0).getTime(), 0, 'a Date from an explicit value still works');
    assert.equal(g.Date.UTC(2020, 0, 1), Date.UTC(2020, 0, 1));
    assert.ok(new g.Date(5) instanceof Date);
    assert.deepEqual(calls, []);
});

check('code scene bundle form: Node esbuild reproduces the golden bundle of part A byte for byte (when that file is in the tree)', () => {
    const golden = 'internal/storymaker/testdata/scenes/s3.bundle.js';
    if (!fs.existsSync(path.join(root, golden))) return;   // part A's golden file has not landed in this tree yet
    assert.equal(t.s3Bundle, read(golden).replace(/\r\n/g, '\n'));
});

check('code scene frames (D26): each g.text sets the direction of its own text', () => {
    const arabic = String.fromCodePoint(0x0645, 0x0631, 0x062D, 0x0628, 0x0627) + '!';
    const { C, build } = runScene(`export default function scene(api) {
      api.duration(1);
      api.onFrame((t, g) => { g.text(${JSON.stringify(arabic)}, 100, 100); g.text('Hello!', 100, 200); });
    }`, 'dirs');
    const cv = new FakeCanvas(1920, 1080);
    const tl = { aspect: '16:9', fps: 30, palette: { bg: '#fff', bg2: '#eee', accent: '#f00', text: '#000' }, cat: C.scene.indexCatalog(catalog), fontFamilies: { default: 'F' }, style: { font: 'default' }, safe: catalog.stage.safe };
    const rc = C.render.makeContext(tl, { rig: files.rig, props: files.props, envs: files.envs }, 1920, 1080);
    C.codescene.drawFrame(build(), 0, cv.ctx, rc);
    const drawn = cv.ctx.log.filter(l => l.startsWith('set direction=') || l.startsWith('fillText('));
    assert.deepEqual(drawn, ['set direction=rtl', 'fillText(' + arabic + ',100,100)', 'set direction=ltr', 'fillText(Hello!,100,200)']);
});
