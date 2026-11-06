// Task C8: the worker realm: role dispatch, the message protocol of the player worker and of a code-scene worker, the
// determinism shadows installed on load. The workers run the real player bundle parts in a vm (InProcessWorker).
import assert from 'node:assert/strict';
import vm from 'node:vm';
import { plain, InProcessWorker, until } from '../storymaker-test-support.mjs';
import { loadHost, TEST_LIMITS } from '../storymaker-test-support.mjs';
import { t } from '../storymaker-test-context.mjs';

const { check, catalog, files, firewall, minimal, s3Bundle } = t;

// A tiny RPC client over a worker: replies are matched by req.
function client(worker) {
    const waiting = new Map(), c = { hello: false, order: [] };
    let req = 0;
    worker.onmessage = ev => {
        const d = ev.data;
        if (d.type === 'hello') { c.hello = d.role; return; }
        c.order.push(d.req);
        const w = waiting.get(d.req);
        if (w) { waiting.delete(d.req); w(d); }
    };
    c.call = msg => new Promise(resolve => { msg.req = ++req; waiting.set(req, resolve); worker.postMessage(msg); });
    return c;
}
const font = { name: 'default', family: 'Storymaker Default', weight: 400, data: new ArrayBuffer(8) };
const allAssets = () => {
    const a = { 'catalog:rig/human.json': files.rig };
    for (const k of Object.keys(files.props)) a['catalog:' + k] = files.props[k];
    for (const k of Object.keys(files.envs)) a['catalog:' + k] = files.envs[k];
    return a;
};

check('player worker: hello, plan, compile, assets, render, in that protocol', async () => {
    const w = new InProcessWorker('player', ''), c = client(w);
    await until(() => c.hello === 'player', 'hello');
    const planned = await c.call({ type: 'plan', story: firewall, catalog, audioInfo: {}, opts: {} });
    assert.equal(planned.ok, true);
    assert.deepEqual(plain(Object.keys(planned.plan.scenes)), ['s1', 's2', 's3']);
    assert.equal(planned.plan.scenes.s3.kind, 'code');
    assert.equal(planned.plan.palette.bg, '#fff7e6'); assert.equal(planned.plan.fontFamilies.default, 'Storymaker Default');
    const before = await c.call({ type: 'render', n: 0, codeFrames: {}, codeErrors: {} });
    assert.equal(before.ok, false); assert.equal(before.error.code, 'code_error', 'render before compile is an error reply');
    const comp = await c.call({ type: 'compile', story: firewall, catalog, audioInfo: {}, opts: { codeScenes: { s3: { duration: 5 } } } });
    // s1 is 129 frames: 9 words / 2.3 + 0.4 s tail = 4.313 s. s2 starts at 129 - 15 = 114. Its narration decides its length:
    // 0.5 s + 5 words / 2.3 + 0.4 s tail = 3.074 s = 92 frames, so it ends at 206. Its beats end earlier: walk 1.712 s from 0.5 s,
    // shield at 0.5 + 0.4 * 1.712 = 1.185 s for 0.5 s, camera 0.8 s until 2.485 s. s3 starts at 206 - 15 = 191 and lasts api.duration(5) = 150 frames: 341.
    assert.equal(comp.ok, true); assert.equal(comp.report.frame_count, 341);
    assert.equal(comp.schedule.scenes.length, 3);
    assert.ok(comp.missing.includes('catalog:rig/human.json'), 'nothing is loaded yet');
    const loaded = await c.call({ type: 'assets', assets: allAssets(), fonts: [font] });
    assert.deepEqual(plain(loaded.missing), [], 'the readiness barrier: every need is satisfied');
    assert.equal(w.context.fonts.faces.length, 1, 'the font went through FontFace and fonts.add in the worker');
    assert.equal(w.context.fonts.faces[0].status, 'loaded');
    const frame = await c.call({ type: 'render', n: 40, codeFrames: {}, codeErrors: {} });
    assert.equal(frame.ok, true); assert.equal(frame.bitmap.width, 1920); assert.equal(frame.bitmap.height, 1080); assert.match(frame.bitmap.digest, /^[0-9a-f]{16}$/);
    const again = await c.call({ type: 'render', n: 40, codeFrames: {}, codeErrors: {} });
    assert.equal(again.bitmap.digest, frame.bitmap.digest, 'the same frame renders the same pixels (call log digest)');
    const other = await c.call({ type: 'render', n: 41, codeFrames: {}, codeErrors: {} });
    assert.notEqual(other.bitmap.digest, frame.bitmap.digest);
    const thumb = await c.call({ type: 'render', n: 40, width: 320, codeFrames: {}, codeErrors: {} });
    assert.deepEqual([thumb.bitmap.width, thumb.bitmap.height], [320, 180]);
    const fullWidth = await c.call({ type: 'render', n: 40, width: 1920, codeFrames: {}, codeErrors: {} });
    assert.equal(fullWidth.bitmap.digest, frame.bitmap.digest, 'a thumbnail at the full width is the frame itself (the parity recipe of the notes)');
    const bad = await c.call({ type: 'mystery' });
    assert.equal(bad.ok, false); assert.match(bad.error.message, /unknown message type/);
    w.terminate();
});

check('player worker: messages are handled one at a time, in order; a worker font that fails reports readiness', async () => {
    const w = new InProcessWorker('player', ''), c = client(w);
    await until(() => c.hello === 'player', 'hello');
    const calls = [
        c.call({ type: 'compile', story: minimal, catalog, audioInfo: {}, opts: {} }),
        c.call({ type: 'assets', assets: allAssets(), fonts: [font] }),
        c.call({ type: 'render', n: 3, codeFrames: {}, codeErrors: {} })
    ];
    const replies = await Promise.all(calls);
    assert.ok(replies.every(r => r.ok), JSON.stringify(replies.map(r => r.error)));
    assert.deepEqual(c.order, [1, 2, 3]);
    const failed = await c.call({ type: 'assets', assets: {}, fonts: [{ name: 'other', family: 'Other', weight: 700, data: new ArrayBuffer(0) }] });
    assert.equal(failed.ok, false); assert.equal(failed.error.code, 'readiness'); assert.match(failed.error.message, /Font "Other" \(700\) could not be loaded: A network error occurred./);
    w.terminate();
});

check('player worker: render with png answers a blob from convertToBlob, and no bitmap', async () => {
    const w = new InProcessWorker('player', ''), c = client(w), types = [];
    // env.makeCanvas reads OffscreenCanvas when the first frame draws, so this stand-in sees the encode call.
    w.context.OffscreenCanvas = class extends w.context.OffscreenCanvas {
        async convertToBlob(options) { types.push(options.type); return super.convertToBlob(options); }
    };
    await until(() => c.hello === 'player', 'hello');
    await c.call({ type: 'compile', story: minimal, catalog, audioInfo: {}, opts: {} });
    await c.call({ type: 'assets', assets: allAssets(), fonts: [font] });
    const frame = await c.call({ type: 'render', n: 3, codeFrames: {}, codeErrors: {}, png: true });
    assert.equal(frame.ok, true, JSON.stringify(frame.error));
    assert.equal(frame.bitmap, undefined, 'a png render hands over no bitmap');
    assert.ok(frame.blob.size > 0, 'the fake blob counts the draw calls of the frame');
    assert.deepEqual(types, ['image/png']);
    w.terminate();
});

check('engine: shared_code_scenes builds no scene workers and compiles the same film; render with png returns the blob', async () => {
    const roles = [], errors = [];
    const H = loadHost().StorymakerHost;
    const env = H.defaultEnv('http://unused', {
        hasWorkers: true,
        createWorker(role, text) { roles.push(role); return new InProcessWorker(role, text, {}); },
        setTimer: (fn, ms) => setTimeout(fn, ms), clearTimer: h => clearTimeout(h), now: () => Date.now(), limits: TEST_LIMITS,
        onError: e => errors.push(e.code)
    });
    const load = extra => ({ story: firewall, catalog, audio_info: {}, assets: allAssets(), fonts: [font], format: { fps: 30, width: 192, height: 108 }, ...extra });
    const main = H.createEngine(env), mainLoaded = await main.load(load({ code_bundles: { s3: s3Bundle } }));
    assert.deepEqual(roles, ['player', 'scene']);
    roles.length = 0;
    const helper = H.createEngine(env), helperLoaded = await helper.load(load({ code_bundles: {}, shared_code_scenes: main.codeScenesInfo }));
    assert.deepEqual(roles, ['player'], 'a helper engine builds no scene worker');
    assert.deepEqual(errors, [], 'a helper engine reports no missing bundle');
    assert.equal(helperLoaded.report.frame_count, mainLoaded.report.frame_count);
    assert.equal(JSON.stringify(helper.schedule), JSON.stringify(main.schedule));
    await helperLoaded.prepare();
    const blob = await helper.render(0, { png: true });
    assert.ok(blob && blob.size > 0, 'render with png returns the blob');
    const bitmap = await helper.render(1);
    assert.ok(bitmap.digest, 'without png the answer stays a bitmap');
    main.dispose();
    helper.dispose();
});

check('worker realm: the determinism shadows are installed before any scene code runs', async () => {
    const w = new InProcessWorker('player', ''), c = client(w);
    await until(() => c.hello === 'player', 'hello');
    for (const code of ['Math.random()', 'Date.now()', 'new Date()', 'performance.now()', 'crypto.getRandomValues(new Uint8Array(1))', 'setTimeout(function () {}, 1)', 'setInterval(function () {}, 1)', 'requestAnimationFrame(function () {})']) {
        assert.throws(() => vm.runInContext(code, w.context), /not available in a Storymaker scene/, code);
    }
    w.terminate();
    const doc = new InProcessWorker('none', '');
    assert.doesNotThrow(() => vm.runInContext('Math.random(); Date.now()', doc.context), 'a document-realm load (no role) installs nothing');
});

check('scene worker: build reports duration, needs and issues; frame returns a bitmap; errors carry a line', async () => {
    const w = new InProcessWorker('scene', s3Bundle), c = client(w);
    await until(() => c.hello === 'scene', 'hello');
    const plan = t.core().planTiming(firewall, {}, catalog);
    const opts = { sceneId: 's3', index: 2, catalog, aspect: '16:9', width: 640, height: 360, fps: 30, cues: plan.scenes.s3.cues, palette: plan.palette, families: plan.fontFamilies, styleFont: 'default', safe: catalog.stage.safe };
    const frameEarly = await c.call({ type: 'frame', t: 0.5, width: 0 });
    assert.equal(frameEarly.ok, false, 'a frame before build is an error');
    const built = await c.call({ type: 'build', opts });
    assert.equal(built.ok, true); assert.equal(built.duration, 5);
    assert.deepEqual(plain(built.needs), ['catalog:rig/human.json', 'font:default/400']);
    const loaded = await c.call({ type: 'assets', assets: { 'catalog:rig/human.json': files.rig }, fonts: [font] });
    assert.equal(loaded.ok, true);
    const a = await c.call({ type: 'frame', t: 1.5, width: 0 }), b = await c.call({ type: 'frame', t: 1.5, width: 0 }), later = await c.call({ type: 'frame', t: 3.5, width: 0 });
    assert.equal(a.ok, true); assert.deepEqual([a.bitmap.width, a.bitmap.height], [640, 360]);
    assert.equal(a.bitmap.digest, b.bitmap.digest); assert.notEqual(a.bitmap.digest, later.bitmap.digest, 'the scene animates (gesture at 1 s, emote at 3 s)');
    const small = await c.call({ type: 'frame', t: 1.5, width: 160 });
    assert.deepEqual([small.bitmap.width, small.bitmap.height], [160, 90]);
    w.terminate();
    const broken = new InProcessWorker('scene', t.bundleScene('export default function scene(api) {\n  api.duration(2);\n  api.add("char.nobody");\n}\n', 'broken')), cb = client(broken);
    await until(() => cb.hello === 'scene', 'hello');
    const err = await cb.call({ type: 'build', opts });
    assert.equal(err.ok, false); assert.equal(err.error.code, 'code_error'); assert.match(err.error.message, /Scene "s3": Unknown catalog id "char\.nobody"/);
    assert.equal(typeof err.error.line, 'number', 'the stack names a line of the scene script');
    broken.terminate();
    const none = new InProcessWorker('scene', ''), cn = client(none);
    await until(() => cn.hello === 'scene', 'hello');
    const missing = await cn.call({ type: 'build', opts });
    assert.match(missing.error.message, /did not register a scene function/);
    none.terminate();
});

check('workers: FontFace gets the weight range and the unicode range, and an empty range is omitted', async () => {
    const greek = { name: 'sm-greek', family: 'SMGreek', weight: 400, weight_range: '300 700', unicode_range: 'U+0370-03FF', data: new ArrayBuffer(8) };
    const cjk = { name: 'sm-cjk', family: 'SMCJK', weight: 400, weight_range: '400', unicode_range: '', data: new ArrayBuffer(8) };
    for (const role of ['player', 'scene']) {
        const w = new InProcessWorker(role, role === 'scene' ? s3Bundle : ''), c = client(w);
        await until(() => c.hello === role, 'hello');
        const reply = await c.call({ type: 'assets', assets: {}, fonts: [greek, cjk, font] });
        assert.equal(reply.ok, true, role);
        assert.deepEqual(plain(w.context.fonts.faces.map(f => f.desc)), [
            { weight: '300 700', unicodeRange: 'U+0370-03FF' }, { weight: '400' }, { weight: '400' }
        ], role);
        w.terminate();
    }
});
