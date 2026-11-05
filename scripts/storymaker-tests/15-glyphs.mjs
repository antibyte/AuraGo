// Task G6: SM.glyphs, the coverage tables in JS, against the real ui/img/storymaker/glyphs.json.
import assert from 'node:assert/strict';
import { plain, readJSON } from '../storymaker-test-support.mjs';
import { t } from '../storymaker-test-context.mjs';

const { check } = t;
const data = readJSON('ui/img/storymaker/glyphs.json');
const ids = faces => plain(faces.map(f => f.id));

check('glyphs: a code point goes to the first covering face, and Greek comes from the Greek face, never from Geist', () => {
    const G = t.core().glyphs, idx = G.index(data);
    const faceId = cp => { const i = G.faceOf(idx, cp); return i < 0 ? null : idx.faces[i].id; };
    assert.equal(faceId(0x03C9), 'sm-greek');
    assert.equal(faceId(0x1F00), 'sm-greek-ext');
    assert.equal(faceId(0x0041), 'geist');
    assert.equal(faceId(0x0915), 'sm-deva');
    assert.equal(faceId(0x0627), 'sm-arab');
    assert.equal(faceId(0x6F22), 'sm-cjk');
    assert.equal(faceId(0x0E01), null, 'Thai has no face');
    assert.equal(G.index(null), null);
    assert.equal(G.faceOf(null, 0x41), -1);
});

check('glyphs: needsFor skips marks, spaces and ignorables, and lists each face once', () => {
    const G = t.core().glyphs, idx = G.index(data);
    const cp = (...codes) => String.fromCodePoint(...codes);
    assert.deepEqual(ids(G.needsFor(idx, 'A' + cp(0x03C9) + 'B' + cp(0x03C9))), ['geist', 'sm-greek']);
    assert.deepEqual(ids(G.needsFor(idx, cp(0x0301, 0x200D, 0x3000, 0x2028, 0x0020))), []);
    assert.deepEqual(ids(G.needsFor(idx, cp(0x0E01))), [], 'a character without a face is the job of Validate, not a need');
});

check('glyphs: the family chain, the faces of a language and the key', () => {
    const G = t.core().glyphs, idx = G.index(data);
    assert.deepEqual(plain(G.chain(idx, '$style')), ['SMGreek', '$style', 'SMDeva', 'SMArab', 'SMCJK']);
    for (const lang of ['el', 'EL_gr']) assert.deepEqual(ids(G.facesForLanguage(idx, lang)), ['sm-greek', 'sm-greek-ext'], lang);
    assert.deepEqual(ids(G.facesForLanguage(idx, 'hi-IN')), ['sm-deva']);
    assert.deepEqual(ids(G.facesForLanguage(idx, 'ar')), ['sm-arab']);
    assert.deepEqual(ids(G.facesForLanguage(idx, 'zh-CN')), ['sm-cjk']);
    assert.deepEqual(ids(G.facesForLanguage(idx, 'ja')), ['sm-cjk']);
    assert.deepEqual(ids(G.facesForLanguage(idx, 'de')), []);
    assert.deepEqual(ids(G.facesForLanguage(idx, 'constructor')), [], 'any language string is safe');
    assert.equal(G.key({ id: 'sm-cjk' }), 'font:sm-cjk/400');
});
// ---- Task S1: font needs and the font chain.
{
    const { default: assert } = await import('node:assert/strict');
    const { readJSON, plain, FakeCanvas, InProcessWorker, loadHost, TEST_LIMITS } = await import('../storymaker-test-support.mjs');
    const { t } = await import('../storymaker-test-context.mjs');
    const { check, catalog, bundleScene } = t;
    const SM = t.core();

    const GLYPHS = readJSON('ui/img/storymaker/glyphs.json');
    const CHAIN = ['SMGreek', '$style', 'SMDeva', 'SMArab', 'SMCJK'];
    const HAN = '中文标题';
    // The mini catalog with the real default family, so the geist face counts as a catalog font, as in the real catalog.
    const withGlyphs = data => ({ ...catalog, fonts: [{ name: 'default', family: 'Geist', weight: 400, file: '/fonts/Geist-Regular.woff2', licence: 'OFL-1.1' }], glyph_data: data });
    // The font keys beyond the catalog font of the fixture.
    const fallback = (story, data = GLYPHS) => plain(SM.compile(story, {}, withGlyphs(data)).needs).filter(k => k.startsWith('font:') && k !== 'font:default/400');
    const sceneOpts = language => ({
        sceneId: 'c', index: 0, catalog: withGlyphs(GLYPHS), aspect: '16:9', width: 640, height: 360, fps: 30, cues: {},
        palette: plain(SM.planTiming({ scenes: [{ id: 'c', kind: 'code' }] }, {}, catalog)).palette, families: { default: 'Geist' }, styleFont: 'default',
        safe: catalog.stage.safe, language, fontChain: CHAIN
    });

    check('S1 needs: Han text needs the CJK face, Latin adds no fallback', () => {
        const show = text => ({ scenes: [{ id: 'a', beats: [{ do: 'show', text }] }] });
        assert.deepEqual(fallback(show(HAN)), ['font:sm-cjk/400']);
        // Latin and U+2026 sit in the geist face, whose family is a catalog font.
        assert.deepEqual(fallback(show('Hallo Welt')), []);
        assert.deepEqual(fallback({ scenes: [{ id: 'a', beats: [{ do: 'say', who: 'anna', text: HAN }] }] }), ['font:sm-cjk/400'], 'say text counts');
    });

    check('S1 needs: U+2026 always counts', () => {
        // Here only sm-cjk covers U+2026, and the story never writes it: fitEllipsis can still draw it.
        const data = { schema: 'storymaker-glyphs/1', faces: [
            { id: 'geist', family: 'Geist', file: '/fonts/Geist-Regular.woff2', weight: '400', unicode_range: '', d: [32, 94] },
            { id: 'sm-cjk', family: 'SMCJK', file: '/fonts/qr/NotoSansCJKsc.otf', weight: '400', unicode_range: '', max_bytes: 17825792, d: [0x2026, 0] }
        ] };
        assert.deepEqual(fallback({ scenes: [{ id: 'a', beats: [{ do: 'show', text: 'Hallo' }] }] }, data), ['font:sm-cjk/400']);
    });

    check('S1 needs: narration counts only when it is drawn', () => {
        assert.deepEqual(fallback({ scenes: [{ id: 'a', narration: HAN }] }), ['font:sm-cjk/400'], 'captions default to on');
        assert.deepEqual(fallback({ style: { captions: false }, scenes: [{ id: 'a', narration: HAN }] }), [], 'captions off');
        assert.deepEqual(fallback({ scenes: [{ id: 'a', silent: true, narration: HAN }] }), [], 'a silent scene never draws it');
        assert.deepEqual(fallback({ style: { captions: false }, scenes: [{ id: 'a', narration: HAN, beats: [{ do: 'say', who: 'anna' }] }] }), ['font:sm-cjk/400'], 'a say without text draws it');
    });

    check('S1 code scenes: the language faces join the needs', () => {
        const needs = lang => plain(SM.codescene.build(api => { api.duration(1); }, sceneOpts(lang)).needs);
        // Sorted as strings: '-' (0x2D) comes before '/' (0x2F).
        assert.deepEqual(needs('el'), ['font:sm-greek-ext/400', 'font:sm-greek/400']);
        assert.deepEqual(needs('zh-CN'), ['font:sm-cjk/400']);
        assert.deepEqual(needs('de'), []);
    });

    check('S1 code scenes: the host passes language and chain to the scene worker', async () => {
        const story = { voice: { provider: 'phonon', voice: 'x', language: 'ja' }, scenes: [{ id: 'c', kind: 'code' }] };
        const cat = withGlyphs(GLYPHS), plan = plain(SM.planTiming(story, {}, cat));
        assert.equal(plan.language, 'ja');
        assert.deepEqual(plan.fontChain, CHAIN);
        assert.equal(plain(SM.planTiming(story, {}, catalog)).fontChain, undefined, 'no chain without glyph data');
        const H = loadHost().StorymakerHost, sent = [];
        const engine = H.createEngine(H.defaultEnv('', {
            hasWorkers: true,
            createWorker: (role, text) => {
                const w = new InProcessWorker(role, text), post = w.postMessage.bind(w);
                w.postMessage = msg => { sent.push(msg); post(msg); };
                return w;
            },
            setTimer: (fn, ms) => setTimeout(fn, ms), clearTimer: h => clearTimeout(h), now: () => 0, limits: TEST_LIMITS
        }));
        const bundle = bundleScene('export default function scene(api) {\n  api.duration(1);\n}\n', 'c');
        const built = await engine.buildScene('c', { msg: { code_bundles: { c: bundle }, catalog: cat }, plan });
        assert.deepEqual(plain(built.needs), ['font:sm-cjk/400']);
        const opts = sent.find(m => m.type === 'build').opts;
        assert.equal(opts.language, 'ja');
        assert.deepEqual(plain(opts.fontChain), CHAIN);
        engine.dispose();
    });

    check('S1 chain: a storyboard draws through the chain, and without glyph data through the family', () => {
        const out = SM.compile({ scenes: [{ id: 'a', beats: [{ do: 'show', text: 'Hallo' }] }] }, {}, withGlyphs(GLYPHS));
        const rc = SM.render.makeContext(out.timeline, {}, 1920, 1080);
        assert.deepEqual(plain(SM.render.familyOf(rc, { font: 'default' })), ['SMGreek', 'Geist', 'SMDeva', 'SMArab', 'SMCJK']);
        const bare = SM.compile({ scenes: [{ id: 'a' }] }, {}, catalog);
        assert.equal(SM.render.familyOf(SM.render.makeContext(bare.timeline, {}, 1920, 1080), { font: 'default' }), 'Storymaker Default');
    });

    check('S1 chain: g.text of a code scene sets ctx.font with the CJK face', async () => {
        const canvases = [];
        const env = { makeCanvas: (w, h) => { const c = new FakeCanvas(w, h); canvases.push(c); return c; } };
        const scene = api => {
            api.duration(1);
            api.onFrame((time, g) => { g.text('日本語', 10, 10, { size: 40 }); });
        };
        const runtime = new SM.workerScene.Runtime(env, { __smScene: scene });
        await runtime.handle({ type: 'build', opts: sceneOpts('ja') });
        await runtime.handle({ type: 'frame', t: 0.5, width: 0 });
        assert.match(canvases[0].ctx.font, /"SMCJK"/);
        assert.match(canvases[0].ctx.font, /"Geist"/);
        assert.doesNotMatch(canvases[0].ctx.font, /\$style/);
    });
}
