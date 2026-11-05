// Task C9: the app host: the section 8 protocol with a fake parent window, in-process workers running the real player
// bundle parts: source check, size caps, reset on load, playback with frame dropping, thumbnails, deadlines, fallback.
import assert from 'node:assert/strict';
import {
    plain, loadHost, until, settle, loadPlayer, FakeCanvas, InProcessWorker, realmGlobals, TEST_LIMITS
} from '../storymaker-test-support.mjs';
import { t } from '../storymaker-test-context.mjs';

const { check, near, catalog, files, firewall, minimal, s3Bundle, bundleScene } = t;

const BASE = 'https://aura.test';
const assetsOf = f => {
    const a = { 'catalog:rig/human.json': f.rig };
    for (const k of Object.keys(f.props)) a['catalog:' + k] = f.props[k];
    for (const k of Object.keys(f.envs)) a['catalog:' + k] = f.envs[k];
    return a;
};
const fontOf = () => ({ name: 'default', family: 'Storymaker Default', weight: 400, data: new ArrayBuffer(16) });

// An HTMLCanvasElement has no transferToImageBitmap: the fallback draws the canvas itself.
const domCanvas = (w, h) => { const c = new FakeCanvas(w, h); c.transferToImageBitmap = undefined; return c; };

// ---- app mode ---------------------------------------------------------------------------------------------------
function makeApp(opts = {}) {
    const hostCtx = loadHost(), H = hostCtx.StorymakerHost;
    const parent = { messages: [], postMessage(msg, origin, transfer) { this.messages.push({ msg, origin, transfer }); } };
    const listeners = [], rafQueue = [];
    let rafId = 0;
    const win = {
        parent, addEventListener(type, fn) { listeners.push(fn); }, removeEventListener(type, fn) { const i = listeners.indexOf(fn); if (i >= 0) listeners.splice(i, 1); },
        requestAnimationFrame(fn) { rafQueue.push({ id: ++rafId, fn }); return rafId; },
        cancelAnimationFrame(id) { const i = rafQueue.findIndex(r => r.id === id); if (i >= 0) rafQueue.splice(i, 1); },
        createImageBitmap: async c => c
    };
    const canvas = new FakeCanvas(300, 150), workers = [], clock = { t: 0 };
    const env = {
        hasWorkers: opts.hasWorkers !== false,
        createWorker(role, text) {
            const w = new InProcessWorker(role, text, { hang: role === 'player' ? opts.hangPlayer : [] });
            workers.push(w);
            return w;
        },
        setTimer: (fn, ms) => setTimeout(fn, ms), clearTimer: h => clearTimeout(h), now: () => clock.t,
        limits: { ...TEST_LIMITS, ...(opts.limits || {}) }, ...(opts.env || {})
    };
    const app = H.mountApp({ canvas, base: BASE, window: win, env });
    const send = (msg, source = parent) => listeners.slice().forEach(fn => fn({ source, data: { sm: 1, ...msg } }));
    const sent = type => parent.messages.filter(m => m.msg.type === type).map(m => m.msg);
    const step = ts => { const r = rafQueue.shift(); if (r) r.fn(ts); return !!r; };
    const loadMsg = (extra = {}) => ({
        type: 'load', story: firewall, catalog, audio_info: {}, assets: assetsOf(files), fonts: [fontOf()], code_bundles: { s3: s3Bundle },
        format: { aspect: '16:9', fps: 30, width: 480 }, ...extra
    });
    return { H, hostCtx, app, parent, win, canvas, workers, clock, send, sent, step, rafQueue, listeners, loadMsg };
}
const loaded = async a => { a.send(a.loadMsg()); await until(() => a.sent('time').length >= 1, 'the first time message'); };

check('host app: ready, load, compile report, first frame drawn, target origin', async () => {
    const a = makeApp();
    assert.deepEqual(plain(a.sent('ready')), [{ sm: 1, type: 'ready', caps: { workers: true, offscreen: false } }].map(m => ({ ...m, caps: { workers: true, offscreen: false } })));
    assert.equal(a.parent.messages[0].origin, BASE, 'posts go to the origin of data-sm-base');
    await loaded(a);
    const rep = plain(a.sent('compiled')[0].report);
    // 341 frames: s1 0..129, s2 114..206 (0.5 s + 5 words / 2.3 + 0.4 s tail = 92 frames), s3 191..341 (206 - 15, then api.duration(5) = 150 frames)
    assert.equal(rep.frame_count, 341); assert.equal(rep.width, 480); assert.equal(rep.height, 270);
    assert.deepEqual(rep.scenes.map(s => s.kind), ['storyboard', 'storyboard', 'code']);
    assert.equal(rep.issues.filter(i => i.severity === 'error').length, 0);
    assert.deepEqual(plain(a.sent('time')[0]), { sm: 1, type: 'time', frame: 0, playing: false });
    assert.equal(a.canvas.width, 480); assert.equal(a.canvas.height, 270);
    assert.ok(a.canvas.ctx.log.some(l => /^drawImage\(bitmap\d+,0,0\)$/.test(l)), 'the first frame is drawn on the visible canvas');
    assert.equal(a.sent('error').length, 0);
    assert.ok(a.workers.length === 2 && a.workers[0].role === 'player' && a.workers[1].role === 'scene');
});

check('host app: assets the load did not carry are requested with need_assets (compiled first)', async () => {
    const a = makeApp();
    a.send(a.loadMsg({ assets: {}, fonts: [] }));
    await until(() => a.sent('need_assets').length === 1, 'need_assets');
    const types = a.parent.messages.map(m => m.msg.type);
    assert.ok(types.indexOf('compiled') < types.indexOf('need_assets'), 'the report is posted before the assets are fetched');
    const req = a.sent('need_assets')[0];
    assert.deepEqual(plain(req.keys), ['catalog:environments/abstract-tech.json', 'catalog:environments/office.json', 'catalog:props/lock.json', 'catalog:props/shield.json', 'catalog:rig/human.json', 'font:default/400']);
    const all = assetsOf(files); all['font:default/400'] = new ArrayBuffer(16);
    a.send({ type: 'assets', req: req.req, assets: all, missing: [] });
    await until(() => a.sent('time').length >= 1, 'the first frame after the assets');
    assert.equal(a.sent('error').length, 0);
});

check('host app: missing assets are reported, never silently ignored', async () => {
    const a = makeApp();
    a.send(a.loadMsg({ assets: {}, fonts: [] }));
    await until(() => a.sent('need_assets').length === 1, 'need_assets');
    const req = a.sent('need_assets')[0];
    a.send({ type: 'assets', req: req.req, assets: { 'catalog:rig/human.json': files.rig }, missing: ['catalog:props/lock.json'] });
    await until(() => a.sent('error').length >= 1, 'an error');
    const err = a.sent('error')[0];
    assert.equal(err.code, 'readiness'); assert.match(err.message, /Missing assets: .*props\/lock\.json/);
    assert.equal(a.sent('time').length, 0, 'no frame is shown without its assets');
});

check('host app: the source check, the typed schema and the size caps', async () => {
    const a = makeApp();
    const before = a.parent.messages.length;
    a.send({ type: 'seek', frame: 3 }, {});
    a.send({ type: 'load', story: firewall, catalog }, null);
    assert.equal(a.parent.messages.length, before, 'messages from any window but the parent are ignored');
    a.send({ type: 'seek', frame: -1 });
    a.send({ type: 'nope' });
    a.send({ type: 'load', story: { scenes: new Array(1000).fill({}) }, catalog });
    a.send({ type: 'load', story: firewall, catalog: { entries: 5 } });
    a.send({ type: 'load', story: firewall, catalog, fonts: [{ name: 'x', family: 'x', weight: 400, data: new ArrayBuffer(9 * 1048576) }] });
    a.send({ type: 'load', story: firewall, catalog, code_bundles: { 'bad id': 'x' } });
    a.send({ type: 'load', story: firewall, catalog, code_bundles: { s3: 'x'.repeat(300000) } });
    a.send({ type: 'thumb', req: 1, scene_id: 's1', width: 5000 });
    a.send({ type: 'assets', req: 1, assets: { 'bad key': 1 } });
    a.listeners.forEach(fn => fn({ source: a.parent, data: { type: 'seek', frame: 1 } }));
    const errs = a.sent('error');
    assert.equal(errs.length, 10);
    assert.ok(errs.every(e => e.code === 'bad_message'));
    const H = a.H, ok = H.checkMessage;
    assert.equal(ok({ sm: 1, type: 'play' }), null); assert.equal(ok({ sm: 1, type: 'seek', frame: 7 }), null);
    assert.equal(ok({ sm: 1, type: 'thumb', req: 'r', scene_id: 's1', width: 160 }), null);
    assert.equal(ok({ sm: 1, type: 'load', story: firewall, catalog, assets: assetsOf(files), fonts: [fontOf()], code_bundles: { s3: s3Bundle }, format: { aspect: '9:16', fps: 25, width: 540 } }), null);
    assert.match(ok({ sm: 1, type: 'load', story: firewall, catalog, format: { fps: 60 } }), /fps/);
    assert.ok(ok(null) && ok('x') && ok({ sm: 2, type: 'play' }));
    assert.equal(a.sent('compiled').length, 0, 'nothing was loaded');
});

check('host app: seek, play with frame dropping, pause and the end of the video', async () => {
    const a = makeApp();
    await loaded(a);
    const renders = () => a.workers[0].received.filter(t => t === 'render').length;
    const r0 = renders();
    a.send({ type: 'seek', frame: 100 });
    await until(() => a.sent('time').some(m => m.frame === 100), 'seek');
    assert.equal(renders(), r0 + 1);
    a.send({ type: 'seek', frame: 99999 });
    await until(() => a.sent('time').some(m => m.frame === 340), 'seek clamps to the last frame, 341 - 1');
    a.send({ type: 'seek', frame: 0 });
    await until(() => a.sent('time').filter(m => m.frame === 0).length >= 2, 'seek back');
    a.send({ type: 'play' });
    assert.equal(a.sent('time').at(-1).playing, true);
    const before = renders();
    a.clock.t = 1000; a.step(1000); await settle(8);
    a.clock.t = 1100; a.step(1100); await settle(8);     // 3 frames later
    a.clock.t = 1200; a.step(1700); await settle(8);     // 21 frames later: the in-between frames are dropped
    a.clock.t = 1300; a.step(1000 + 5000); await settle(8);
    const rendered = renders() - before;
    assert.ok(rendered <= 4, 'only the frames at the sampled times are rendered, got ' + rendered);
    assert.equal(a.sent('time').filter(m => m.playing).at(-1).frame, 150);
    a.send({ type: 'pause' });
    assert.equal(a.sent('time').at(-1).playing, false); assert.equal(a.rafQueue.length, 0, 'no frame callback stays queued');
    a.send({ type: 'play' });
    a.clock.t = 2000; a.step(50000); await settle(8);
    a.clock.t = 2100; a.step(90000); await settle(8);
    assert.equal(a.app.state.playing, false, 'playback ends at the last frame');
    assert.equal(a.sent('time').at(-1).frame, 340); assert.equal(a.sent('time').at(-1).playing, false);
});

check('host app: thumbnails come back as bitmaps of the requested width', async () => {
    const a = makeApp();
    await loaded(a);
    a.send({ type: 'thumb', req: 'th1', scene_id: 's2', width: 160 });
    await until(() => a.sent('thumb').length === 1, 'the thumbnail');
    const th = a.sent('thumb')[0];
    assert.equal(th.req, 'th1'); assert.equal(th.scene_id, 's2');
    assert.equal(th.bitmap.width, 160); assert.equal(th.bitmap.height, 90);
    assert.equal(a.parent.messages.find(m => m.msg.type === 'thumb').transfer.length, 1, 'the bitmap is transferred');
    a.send({ type: 'thumb', req: 'th2', scene_id: 'nope', width: 160 });
    await until(() => a.sent('error').length === 1, 'unknown scene');
    assert.equal(a.sent('error')[0].scene_id, 'nope');
    a.send({ type: 'thumb', req: 'th3', scene_id: 's3', width: 96, frame: 300 });
    await until(() => a.sent('thumb').length === 2, 'the code scene thumbnail');
    assert.equal(a.sent('thumb')[1].bitmap.width, 96);
    assert.equal(a.sent('error').length, 1, 'no error for the code scene');
});

check('host app: every load and reset starts clean (workers stopped, canvas cleared)', async () => {
    const a = makeApp();
    await loaded(a);
    const first = a.workers.slice();
    a.send(a.loadMsg({ format: { aspect: '16:9', fps: 30, width: 320 } }));
    await until(() => a.sent('compiled').length === 2, 'the second report');
    assert.ok(first.every(w => w.dead), 'the first load\'s workers are terminated');
    await until(() => a.sent('time').length >= 2, 'the second first frame');
    assert.equal(a.sent('compiled')[1].report.width, 320);
    const alive = a.workers.filter(w => !w.dead).length;
    a.send({ type: 'reset' });
    assert.ok(a.workers.every(w => w.dead) && alive === 2);
    const posts = a.parent.messages.length;
    a.send({ type: 'seek', frame: 3 }); a.send({ type: 'play' });
    await settle(5);
    assert.equal(a.parent.messages.length, posts, 'nothing plays after a reset');
    assert.ok(a.canvas.ctx.log.some(l => l.startsWith('clearRect')));
    a.app.dispose();
    a.send({ type: 'load', ...a.loadMsg() });
    await settle(5);
    assert.equal(a.sent('compiled').length, 2, 'a disposed host answers nothing');
});

check('host app: a worker that overruns its deadline is terminated and reported', async () => {
    const a = makeApp({ hangPlayer: ['render'], limits: { warmupDeadlineMs: 60, frameDeadlineMs: 60 } });
    a.send(a.loadMsg());
    await until(() => a.sent('error').length >= 1, 'a deadline error');
    const err = a.sent('error')[0];
    assert.equal(err.code, 'deadline'); assert.match(err.message, /took longer than 60 ms/);
    assert.ok(a.workers[0].dead, 'the player worker is terminated');
    assert.ok(a.workers.every(w => w.dead), 'the code-scene workers stop with it');
    assert.equal(a.sent('time').length, 0);
});

const failing = src => bundleScene(src, 'bad');
check('host app: a code scene that fails to build is reported, its scene shows a placeholder', async () => {
    for (const [src, pattern] of [
        ["export default function scene(api) { api.add('char.nobody'); }", /Unknown catalog id "char\.nobody"/],
        ['export default function scene(api) { api.duration(3); const x = Math.random(); }', /Math\.random\(\) is not available/],
        ['export default function scene(api) { api.duration(3); setTimeout(() => {}, 1); }', /setTimeout\(\) is not available/],
        ['export default function scene(api) { throw new Error("boom"); }', /boom/]
    ]) {
        const a = makeApp();
        a.send(a.loadMsg({ code_bundles: { s3: failing(src) } }));
        await until(() => a.sent('time').length >= 1, 'the first frame');
        const errs = a.sent('error');
        assert.equal(errs.length, 1, src);
        assert.equal(errs[0].code, 'code_error'); assert.equal(errs[0].scene_id, 's3'); assert.match(errs[0].message, pattern);
        const rep = plain(a.sent('compiled')[0].report);
        assert.ok(rep.issues.some(i => i.code === 'code_error' && i.severity === 'error' && i.scene_id === 's3'));
        // s3 is 191..296 here: the failed scene has no built duration, so it takes its narration (see Edit 4)
        const inside = rep.scenes[2].start_frame + 20;
        a.send({ type: 'seek', frame: inside });
        await until(() => a.sent('time').some(m => m.frame === inside), 'a frame inside the failed code scene');
        assert.equal(a.sent('error').length, 1, 'the placeholder frame renders without another error');
    }
    const a = makeApp();
    a.send(a.loadMsg({ code_bundles: {} }));
    await until(() => a.sent('error').length >= 1, 'missing bundle');
    assert.match(a.sent('error')[0].message, /No bundle for code scene "s3"/);
});

check('host app: a code scene that fails at frame time is reported once and replaced by a placeholder', async () => {
    const a = makeApp();
    const src = 'export default function scene(api) { api.duration(4); api.onFrame((t, g) => { if (t > 1) throw new Error("late failure"); g.rect(0, 0, 10, 10); }); }';
    a.send(a.loadMsg({ code_bundles: { s3: bundleScene(src, 'late') } }));
    await until(() => a.sent('time').length >= 1, 'the first frame');
    // s3 starts at 191 (206 - 15, s2 ends at 206) and lasts api.duration(4) = 120 frames, so it ends at 311
    a.send({ type: 'seek', frame: 191 + 15 });
    await until(() => a.sent('time').some(m => m.frame === 206), 'the early code scene frame');
    assert.equal(a.sent('error').length, 0);
    a.send({ type: 'seek', frame: 191 + 60 });
    await until(() => a.sent('error').length === 1, 'the late failure');
    const e = a.sent('error')[0];
    assert.equal(e.code, 'code_error'); assert.equal(e.scene_id, 's3'); assert.match(e.message, /late failure/); assert.equal(typeof e.line, 'number');
    a.send({ type: 'seek', frame: 191 + 70 });
    await until(() => a.sent('time').some(m => m.frame === 261), 'the next frame still renders');
    assert.equal(a.sent('error').length, 1);
});

check('host app: without workers the storyboard renders in the host realm and code scenes show a placeholder', async () => {
    const core = loadPlayer(), g = realmGlobals();
    const a = makeApp({ hasWorkers: false, env: {
        loadCore: async () => core.StorymakerCore, makeCanvas: domCanvas, FontFace: g.FontFace, fonts: g.fonts
    } });
    assert.equal(a.sent('ready')[0].caps.workers, false);
    a.send(a.loadMsg());
    await until(() => a.sent('time').length >= 1, 'the first frame');
    assert.equal(a.workers.length, 0);
    const rep = plain(a.sent('compiled')[0].report);
    // 296 frames: s1 0..129, s2 114..206 (0.5 s + 5 words / 2.3 + 0.4 s tail = 92 frames), s3 191..296. Without workers the code scene
    // is not built, so it takes its narration: 0.5 s + 6 words / 2.3 + 0.4 s tail = 3.509 s = 105 frames
    assert.equal(rep.scenes.length, 3); assert.equal(rep.frame_count, 296);
    assert.equal(rep.issues.filter(i => i.severity === 'error').length, 0);
    assert.ok(a.canvas.ctx.log.some(l => /^drawImage\(canvas\d+,0,0\)$/.test(l)));
    a.send({ type: 'seek', frame: 120 });
    await until(() => a.sent('time').some(m => m.frame === 120), 'a storyboard frame');
    a.send({ type: 'seek', frame: rep.scenes[2].start_frame + 20 });
    await until(() => a.sent('time').some(m => m.frame === rep.scenes[2].start_frame + 20), 'a code scene frame');
    a.send({ type: 'thumb', req: 't', scene_id: 's1', width: 120 });
    await until(() => a.sent('thumb').length === 1, 'a thumbnail');
    assert.equal(a.sent('error').length, 0);
});

check('host app: a worker that never says hello falls back to the host realm', async () => {
    const core = loadPlayer(), g = realmGlobals();
    const a = makeApp({ limits: { helloDeadlineMs: 30 }, env: {
        createWorker: () => ({ postMessage() {}, terminate() {}, addEventListener() {} }),
        loadCore: async () => core.StorymakerCore, makeCanvas: domCanvas, FontFace: g.FontFace, fonts: g.fonts
    } });
    a.send(a.loadMsg({ code_bundles: {} }));
    await until(() => a.sent('time').length >= 1, 'the fallback frame');
    assert.equal(a.sent('compiled').length, 1);
});

check('host app: an oversized font passes only with a face of the last accepted load, and the total spans load and assets', async () => {
    const MIB = 1048576;
    const cjkFace = { id: 'sm-cjk', family: 'SMCJK', file: '/fonts/qr/NotoSansCJKsc.otf', weight: '400', unicode_range: '', max_bytes: 17 * MIB, d: [32, 94] };
    const withFaces = { ...catalog, glyph_data: { schema: 'storymaker-glyphs/1', faces: [cjkFace] } };
    const nine = new ArrayBuffer(9 * MIB);   // over the 8 MiB cap, under the 17 MiB of the face
    const a = makeApp(), ok = a.H.checkMessage;
    const loadOf = (cat, name, family) => ({ sm: 1, type: 'load', story: firewall, catalog: cat, fonts: [{ name, family, weight: 400, data: nine }] });
    const fontAnswer = (key, data = nine) => ({ sm: 1, type: 'assets', req: 'r', assets: { [key]: data } });

    // load: the message's own glyph data decides
    assert.match(ok(loadOf(withFaces, 'default', 'Storymaker Default')), /oversized/);
    assert.equal(ok(loadOf(withFaces, 'sm-cjk', 'SMCJK')), null);
    assert.match(ok(loadOf(catalog, 'sm-cjk', 'SMCJK')), /oversized/, 'without faces an oversized font is refused');

    // assets: the faces of the last accepted load decide
    const held = { faces: [cjkFace], fontBytes: 0 };
    assert.match(ok(fontAnswer('font:default/400'), held), /oversized/);
    assert.equal(ok(fontAnswer('font:sm-cjk/400'), held), null);
    assert.match(ok(fontAnswer('font:sm-cjk/400')), /oversized/, 'a missing loaded means no faces');
    // 8 MiB + 1 byte is the limit of this face, so 9 MiB does not fit
    assert.match(ok(fontAnswer('font:sm-cjk/400'), { faces: [{ ...cjkFace, max_bytes: 8 * MIB + 1 }], fontBytes: 0 }), /oversized/);
    // max_bytes never lifts the limit above the 18 MiB of maxFallbackFontBytes
    assert.match(ok(fontAnswer('font:sm-cjk/400', new ArrayBuffer(19 * MIB)), { faces: [{ ...cjkFace, max_bytes: 40 * MIB }], fontBytes: 0 }), /oversized/);
    // the 32 MiB total: 23 + 9 = 32 fits, 24 + 9 = 33 does not
    assert.equal(ok(fontAnswer('font:sm-cjk/400'), { faces: [cjkFace], fontBytes: 23 * MIB }), null);
    assert.match(ok(fontAnswer('font:sm-cjk/400'), { faces: [cjkFace], fontBytes: 24 * MIB }), /too large/);

    // the state: set by a load, grown by an assets answer, cleared by a reset
    a.send(a.loadMsg({ catalog: withFaces, assets: {}, fonts: [{ name: 'sm-cjk', family: 'SMCJK', weight: 400, data: nine }] }));
    assert.deepEqual(plain(a.app.state.loaded.faces.map(f => f.id)), ['sm-cjk']);
    assert.equal(a.app.state.loaded.fontBytes, 9 * MIB, 'the load counts its fonts');
    await until(() => a.sent('need_assets').length === 1, 'need_assets');
    const all = assetsOf(files); all['font:default/400'] = new ArrayBuffer(16);
    a.send({ type: 'assets', req: a.sent('need_assets')[0].req, assets: all, missing: [] });
    assert.equal(a.app.state.loaded.fontBytes, 9 * MIB + 16, 'the assets answer adds the 16 bytes of its font');
    await until(() => a.sent('time').length >= 1, 'the first frame');

    // onMessage passes the state: the same answer is fine before the reset and refused after it
    a.send({ type: 'assets', req: 'late', assets: { 'font:sm-cjk/400': nine } });
    assert.equal(a.sent('error').length, 0);
    a.send({ type: 'reset' });
    assert.deepEqual(plain(a.app.state.loaded), { faces: [], fontBytes: 0 });
    a.send({ type: 'assets', req: 'late', assets: { 'font:sm-cjk/400': nine } });
    assert.equal(a.sent('error').at(-1).code, 'bad_message');
    assert.match(a.sent('error').at(-1).message, /oversized/);

    // the descriptor carries the ranges of a face and leaves a catalog font alone
    const desc = a.H.Engine.prototype.fontDescriptor(withFaces, 'font:sm-cjk/400', nine);
    assert.deepEqual([desc.name, desc.family, desc.weight, desc.weight_range, desc.unicode_range], ['sm-cjk', 'SMCJK', 400, '400', '']);
    const base = a.H.Engine.prototype.fontDescriptor(withFaces, 'font:default/400', nine);
    assert.deepEqual([base.family, base.weight_range, base.unicode_range], ['Storymaker Default', undefined, undefined]);
});
