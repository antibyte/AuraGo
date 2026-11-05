// Task C10: the render driver: boot.json, report, frames in order over all transports, restart, stop, refusal and failures,
// against a fake fetch (a real Chrome run is in ui/storymaker_player_gate_browser_test.go).
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { root, read, plain, loadHost, FakeCanvas, InProcessWorker, MINI, TEST_LIMITS } from '../storymaker-test-support.mjs';
import { storymakerHostParts } from '../storymaker-bundle-parts.js';
import { t } from '../storymaker-test-context.mjs';

const { check, firewall, minimal, s3Bundle, bundleScene } = t;
const failing = src => bundleScene(src, 'bad');

const RBASE = 'http://127.0.0.1:5555/0123456789abcdef0123456789abcdef';
function makeServer(opts = {}) {
    const transport = opts.transport || 'png', W = 192, H = 108;
    const posts = [], gets = [];
    const storyDoc = opts.story || firewall;
    const boot = {
        mode: 'render', fps: 30, width: W, height: H, transport, frame_limit: opts.frameLimit || 5000, story_url: RBASE + '/story.json',
        catalog_url: RBASE + '/catalog/catalog.json', assets_base: RBASE + '/catalog/',
        code_bundles: opts.noCode ? {} : { s3: RBASE + '/scenes/s3.js' },
        fonts: [{ name: 'default', family: 'Storymaker Default', weight: 400, url: RBASE + '/fonts/placeholder.woff2' }], audio_info: {},
        ...(opts.boot || {})
    };
    const res = (body, status = 200) => ({
        ok: status >= 200 && status < 300, status, json: async () => body, text: async () => typeof body === 'string' ? body : JSON.stringify(body), arrayBuffer: async () => new ArrayBuffer(16)
    });
    const answerFor = opts.answer || (() => null);
    let frames = 0;
    async function fetch(url, init = {}) {
        if (init.method === 'POST') {
            const p = url.slice(RBASE.length);
            const rec = { path: p, headers: init.headers || {}, body: init.body };
            posts.push(rec);
            if (p === '/report') {
                const rep = JSON.parse(init.body);
                rec.report = rep;
                frames = rep.frame_count;
                return res(opts.report ? opts.report(rep) : { start: true, from_frame: 0 });
            }
            if (p.startsWith('/frame?n=')) {
                const n = Number(p.slice('/frame?n='.length));
                rec.n = n;
                const custom = answerFor(n, posts.filter(x => x.path.startsWith('/frame?n=')).length);
                if (custom) return res(custom);
                return res(n >= frames - 1 ? { ok: true, done: true } : { ok: true });
            }
            if (p === '/error') { rec.error = JSON.parse(init.body); return res({ ok: true }); }
            return res({}, 404);
        }
        gets.push(url);
        if (opts.bootStatus && url.endsWith('/boot.json')) return res({}, opts.bootStatus);
        if (url === RBASE + '/boot.json') return res(boot);
        if (url === RBASE + '/story.json') return res(storyDoc);
        if (url === RBASE + '/scenes/s3.js') return res(s3Bundle);
        if (url === RBASE + '/fonts/placeholder.woff2') return res('');
        if (url.startsWith(RBASE + '/catalog/')) {
            const rel = url.slice((RBASE + '/catalog/').length);
            const file = path.join(root, MINI, rel);
            return fs.existsSync(file) ? res(JSON.parse(fs.readFileSync(file, 'utf8'))) : res({}, 404);
        }
        return res({}, 404);
    }
    return { fetch, posts, gets, boot, W, H };
}
async function runRender(opts = {}) {
    const hostCtx = loadHost(), H = hostCtx.StorymakerHost, server = makeServer(opts);
    const canvas = new FakeCanvas(1, 1), base = canvas.ctx;
    canvas.getContext = () => new Proxy(base, { get(target, prop) {
        if (prop === 'getImageData') return (x, y, w, h) => ({ data: new Uint8ClampedArray(w * h * 4) });
        return target[prop];
    } });
    canvas.toBlob = (cb, type, quality) => cb({ __blob: true, type, quality });
    const win = { document: { body: { style: {} }, documentElement: { style: {} } }, requestAnimationFrame: fn => setTimeout(fn, 0) };
    canvas.style = {};
    const workers = [];
    const env = {
        hasWorkers: true,
        createWorker(role, text) { const w = new InProcessWorker(role, text, {}); workers.push(w); return w; },
        setTimer: (fn, ms) => { if (opts.timers) opts.timers.push(ms); return setTimeout(fn, ms); }, clearTimer: h => clearTimeout(h), now: () => Date.now(), limits: { ...TEST_LIMITS, ...(opts.limits || {}) }
    };
    const status = await H.mountRender({ canvas, base: RBASE, fetch: server.fetch, window: win, env });
    return { server, status: plain(status), win, canvas, workers, frames: server.posts.filter(p => p.path.startsWith('/frame?n=')) };
}

check('render mode: boot.json, report, then every frame in order (png)', async () => {
    const r = await runRender({ transport: 'png' });
    assert.equal(r.status.phase, 'done', JSON.stringify(r.status));
    assert.equal(r.win.document.body.style.margin, '0', 'the body margin is set through the CSSOM');
    assert.equal(r.win.document.documentElement.style.overflow, 'hidden', 'no scrollbars: the screenshot transport captures the viewport');
    assert.equal(r.canvas.style.display, 'block', 'no inline-canvas gap below the canvas');
    assert.deepEqual(r.server.posts.map(p => p.path).slice(0, 2), ['/report', '/frame?n=0']);
    const rep = r.server.posts[0].report;
    // 341 frames: s1 0..129, s2 114..206 (0.5 s + 5 words / 2.3 + 0.4 s tail = 92 frames), s3 191..341 (206 - 15, then api.duration(5) = 150 frames)
    assert.equal(rep.frame_count, 341); assert.equal(rep.width, 192); assert.equal(rep.height, 108);
    assert.equal(rep.issues.filter(i => i.severity === 'error').length, 0);
    assert.equal(r.server.posts[0].headers['Content-Type'], 'application/json');
    assert.equal(r.frames.length, 341);
    r.frames.forEach((f, i) => {
        assert.equal(f.path, '/frame?n=' + i); assert.equal(f.headers['Content-Type'], 'text/plain', 'a CORS-simple request: no preflight per frame');
        assert.equal(f.headers['X-SM-Frame'], undefined); assert.equal(f.body.type, 'image/png');
    });
    assert.equal(r.canvas.width, 192); assert.equal(r.canvas.height, 108);
    assert.equal(r.server.posts.filter(p => p.path === '/error').length, 0);
    const fontGets = r.server.gets.filter(u => u.includes('/fonts/'));
    assert.deepEqual(fontGets, [RBASE + '/fonts/placeholder.woff2'], 'font URLs come from boot.fonts only');
    assert.ok(r.server.gets.every(u => u.startsWith(RBASE + '/')), 'every request stays under the render base');
    assert.ok(r.server.gets.includes(RBASE + '/catalog/rig/human.json') && r.server.gets.includes(RBASE + '/catalog/environments/office.json'));
    assert.ok(r.workers.every(w => w.dead || true));
});

check('render mode: webp, raw RGBA and the screenshot transport', async () => {
    const webp = await runRender({ transport: 'webp', story: minimal, noCode: true });
    assert.equal(webp.status.phase, 'done'); assert.equal(webp.frames.length, 42);
    assert.equal(webp.frames[0].headers['Content-Type'], 'text/plain'); assert.equal(webp.frames[0].body.type, 'image/webp'); assert.equal(webp.frames[0].body.quality, 1, 'lossless webp');
    const raw = await runRender({ transport: 'raw', story: minimal, noCode: true });
    assert.equal(raw.status.phase, 'done'); assert.equal(raw.frames.length, 42);
    assert.equal(raw.frames[3].headers['Content-Type'], 'text/plain');
    assert.equal(raw.frames[3].body.byteLength, 192 * 108 * 4, 'raw RGBA is w*h*4 bytes');
    const shot = await runRender({ transport: 'screenshot', story: minimal, noCode: true });
    assert.equal(shot.status.phase, 'ready'); assert.equal(shot.status.frame_count, 42);
    assert.equal(shot.server.posts.filter(p => p.path.startsWith('/frame')).length, 0, 'the screenshot transport posts no frames');
    assert.equal(typeof shot.win.__smRenderFrame, 'function');
    const before = shot.canvas.ctx.log.filter(l => l.startsWith('drawImage')).length;
    assert.equal(await shot.win.__smRenderFrame(7), 7);
    assert.equal(shot.canvas.ctx.log.filter(l => l.startsWith('drawImage')).length, before + 1, 'the frame is drawn on the visible canvas');
    assert.equal(plain(shot.win.__smStatus).frame, 7);
    await assert.rejects(() => shot.win.__smRenderFrame(42), /outside 0\.\.41/);
});

check('render mode: boot.frame_deadline_ms replaces the 250 ms frame deadline of the preview', async () => {
    const limits = { warmupFrames: 0, frameDeadlineMs: 2000 };
    const control = [], offered = [], absurd = [];
    assert.equal((await runRender({ story: minimal, noCode: true, limits, timers: control })).status.phase, 'done');
    assert.ok(control.includes(2000), 'without frame_deadline_ms every frame call waits at most the preview deadline');
    assert.equal((await runRender({ story: minimal, noCode: true, limits, timers: offered, boot: { frame_deadline_ms: 5000 } })).status.phase, 'done');
    assert.ok(offered.includes(5000) && !offered.includes(2000), 'the deadline Go offers in boot.json is the frame deadline of the render');
    assert.equal((await runRender({ story: minimal, noCode: true, limits, timers: absurd, boot: { frame_deadline_ms: 3600000 } })).status.phase, 'done');
    assert.ok(absurd.includes(60000) && !absurd.includes(3600000), 'a render page never waits longer than a minute for one frame');
});

check('render mode: restart, stop, refusal, frame limit and failures', async () => {
    let restarted = false;
    const restart = await runRender({ story: minimal, noCode: true, answer: n => { if (n === 5 && !restarted) { restarted = true; return { restart: true }; } return null; } });
    assert.equal(restart.status.phase, 'done');
    assert.deepEqual(restart.frames.map(f => f.n), [...Array(6).keys(), ...Array(42).keys()], 'restart renders again from frame 0');
    const stop = await runRender({ story: minimal, noCode: true, answer: n => n === 3 ? { stop: true, error: 'cancelled', message: 'job cancelled' } : null });
    assert.equal(stop.status.phase, 'stopped'); assert.equal(stop.frames.length, 4); assert.equal(stop.status.error, 'cancelled');
    const refused = await runRender({ story: minimal, noCode: true, report: () => ({ start: false, error: 'ffmpeg_unavailable', message: 'no ffmpeg' }) });
    assert.equal(refused.status.phase, 'refused'); assert.equal(refused.frames.length, 0); assert.equal(refused.status.error, 'ffmpeg_unavailable');
    const resumed = await runRender({ story: minimal, noCode: true, report: () => ({ start: true, from_frame: 10 }) });
    assert.equal(resumed.frames[0].n, 10);
    const limited = await runRender({ story: minimal, noCode: true, frameLimit: 20, answer: () => ({ ok: true }) });
    assert.equal(limited.status.phase, 'error'); assert.equal(limited.frames.length, 20);
    assert.match(limited.server.posts.at(-1).error.message, /ran past the last frame/);
    const broken = await runRender({ bootStatus: 503 });
    assert.equal(broken.status.phase, 'error');
    const err = broken.server.posts.find(p => p.path === '/error').error;
    assert.equal(err.code, 'readiness'); assert.match(err.message, /boot\.json answered 503/);
    const rejected = await runRender({ story: minimal, noCode: true, answer: () => ({ stop: true, error: 'bad_frame', message: 'size' }) });
    assert.equal(rejected.status.phase, 'stopped');
});

check('render mode: a code scene that fails is reported in the report issues and by POST /error', async () => {
    const hostCtx = loadHost();
    const server = makeServer({});
    const origFetch = server.fetch;
    const bad = failing('export default function scene(api) { api.duration(3); api.add("prop.lock", { at: "nowhere" }); }');
    const fetch = async (url, init) => url === RBASE + '/scenes/s3.js' ? { ok: true, status: 200, text: async () => bad } : origFetch(url, init);
    const canvas = new FakeCanvas(1, 1);
    const win = { document: { body: { style: {} } } };
    const env = { hasWorkers: true, createWorker: (role, text) => new InProcessWorker(role, text, {}), setTimer: (fn, ms) => setTimeout(fn, ms), clearTimer: h => clearTimeout(h), now: () => Date.now() };
    const status = plain(await hostCtx.StorymakerHost.mountRender({ canvas, base: RBASE, fetch, window: win, env }));
    const rep = server.posts.find(p => p.path === '/report').report;
    assert.ok(rep.issues.some(i => i.code === 'code_error' && i.severity === 'error' && /Unknown position "nowhere"/.test(i.message)));
    const err = server.posts.find(p => p.path === '/error');
    assert.ok(err && err.error.code === 'code_error' && err.error.scene_id === 's3');
    void status;
});

check('host bundle is small and self-contained: no codescene, rig or compile in the document realm', () => {
    const host = storymakerHostParts.map(read).join('\n');
    assert.ok(!/SM\.rig =|SM\.compile =|SM\.codescene =|SM\.render =/.test(host));
    assert.ok(storymakerHostParts.length === 5 && storymakerHostParts[0].endsWith('schedule.js'));
    const ctx = loadHost();
    assert.ok(ctx.StorymakerHost.mount && ctx.StorymakerHost.mountApp && ctx.StorymakerHost.mountRender && ctx.StorymakerCore.schedule);
    assert.throws(() => ctx.StorymakerHost.mount({ mode: 'nope', canvas: {}, base: 'x' }), /Unknown Storymaker mode/);
    assert.throws(() => ctx.StorymakerHost.mount({ mode: 'app' }), /needs a canvas/);
});
