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
