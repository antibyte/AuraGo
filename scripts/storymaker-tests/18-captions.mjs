// Task S4: caption splitting and timing, the film end, and the captions of the compile report.
import assert from 'node:assert/strict';
import { plain, readJSON } from '../storymaker-test-support.mjs';
import { t } from '../storymaker-test-context.mjs';

const { check, catalog } = t;
const SM = t.core();
const C = SM.captions;

const piece = (hash, at, used, chars) => ({ hash, ext: 'wav', at_ms: at, in_ms: 0, out_ms: used, chars });
const block = (id, durationMs, pieces) => ({ id, kind: 'narration', speaker: 'narrator', duration_ms: durationMs, spans: [], levels: [], speech_ok: true, pieces });
const times = list => plain(list).map(c => [c.start_ms, c.end_ms]);
// n words of 30 letters. A line holds one word, because 30 + 1 + 30 = 61 columns is over 42. Length: 30n + n - 1 units.
const words = n => Array.from({ length: n }, (_, i) => String.fromCharCode(97 + i).repeat(30)).join(' ');

check('captions: a line holds 42 columns, and a wide character counts 2', () => {
    // 21 Han characters are 42 columns, so the 22nd starts the second line.
    const han = String.fromCodePoint(0x6f22).repeat(22);
    assert.deepEqual(plain(C.split(han)[0].lines), [han.slice(0, 21), han.slice(21)]);
    // 20 + 1 + 21 = 42 columns fit on the first line, and 'c' does not.
    const first = 'a'.repeat(20) + ' ' + 'b'.repeat(21);
    assert.deepEqual(plain(C.split(first + ' c')[0].lines), [first, 'c']);
    // A token over 42 columns breaks where canBreakAfter allows.
    assert.deepEqual(plain(C.split('x'.repeat(50))[0].lines), ['x'.repeat(42), 'x'.repeat(8)]);
});

check('captions: at most 2 lines each, and the offsets hold the whitespace after the text', () => {
    // 5 words give lines of one word and captions of 2 lines. Each word with its space is 31 units, and the text has 154.
    const parts = plain(C.split(words(5)));
    assert.deepEqual(parts.map(p => p.lines.length), [2, 2, 1]);
    assert.deepEqual(parts.map(p => [p.a, p.b]), [[0, 62], [62, 124], [124, 154]]);
});

check('captions: each lasts at least 1.0 s, except at the film end', () => {
    // Estimated 1 s over 154 units: cueMap(62) = 403 ms is raised to 1000, cueMap(124) = 805 ms is raised to 2000, the last end is 3000.
    const list = C.forBlock(words(5), SM.cuemap.estimated('a:n', 1, 154), 0, 'a');
    assert.deepEqual(times(list), [[0, 1000], [1000, 2000], [2000, 3000]]);
    assert.deepEqual(times(C.settle(list, 2500)), [[0, 1000], [1000, 2000], [2000, 2500]], 'the minimum yields at the film end');
    assert.deepEqual(times(C.settle(list, 2000)), [[0, 1000], [1000, 2000]], 'a caption left with no length is dropped');
    // settle sorts across blocks and cuts at the next start.
    const later = { start_ms: 4000, end_ms: 6000, lines: ['b'], scene_id: 'b', block_id: 'b:n' }, earlier = { start_ms: 0, end_ms: 5000, lines: ['a'], scene_id: 'a', block_id: 'a:n' };
    const settled = C.settle([later, earlier], 9000);
    assert.deepEqual(times(settled), [[0, 4000], [4000, 6000]]);
    assert.equal(C.at(settled, 3999).block_id, 'a:n');
    assert.equal(C.at(settled, 4000).block_id, 'b:n');
    assert.equal(C.at(settled, 6000), null);
});

check('captions: a caption start is startMs + round(cueMap(a)) on a two-piece block', () => {
    // 6 words, 185 units. Piece 0 holds 3 words with their spaces (93 units), piece 1 the rest (92), used 3000 and 4000 ms, pause 350 ms.
    // cueMap(62) = 62/93 of 3000 = 2000. cueMap(124) = 3350 + 31/92 of 4000 = 4698. cueMap(185) = 3350 + 4000 = 7350. startMs is 500.
    const b = SM.cuemap.prepare(block('a:n', 7350, [piece('h0', 0, 3000, 93), piece('h1', 3350, 4000, 92)]), 185);
    assert.equal(b.placed, 'span');
    const list = C.forBlock(words(6), b, 500, 'a');
    assert.deepEqual(times(list), [[500, 2500], [2500, 5198], [5198, 7850]]);
    plain(C.split(words(6))).forEach((part, i) => assert.equal(list[i].start_ms, 500 + Math.round(SM.cuemap.cueMap(b, part.a))));
    assert.equal(list[0].scene_id, 'a');
    assert.equal(list[0].block_id, 'a:n');
});

check('compile report: captions are filled for an estimated block and with captions off; a silent scene has none', () => {
    const story = style => ({ style, scenes: [{ id: 'a', narration: 'Hallo Welt.' }] });
    // 2 words: the estimate is the 1 s minimum, and the scene lasts 1.4 s.
    const want = [{ start_ms: 0, end_ms: 1000, lines: ['Hallo Welt.'], scene_id: 'a', block_id: 'a:n' }];
    assert.deepEqual(plain(SM.compile(story({ captions: true }), {}, catalog).report.captions), want);
    assert.deepEqual(plain(SM.compile(story({ captions: false }), {}, catalog).report.captions), want);
    assert.deepEqual(plain(SM.compile({ scenes: [{ id: 'a', silent: true, narration: 'Hallo Welt.' }] }, {}, catalog).report.captions), []);
});

check('compile report: the film end cuts a one-word last line and drops a caption with no length', () => {
    // One word of 100 ms: the scene lasts 0.1 + 0.4 tail = 0.5 s = 15 frames = 500 ms, so the 1.0 s minimum yields.
    const short = plain(SM.compile({ scenes: [{ id: 'a', narration: 'Hallo' }] }, { narration: { a: { blocks: [block('a:n', 100, [piece('h0', 0, 100, 5)])] } } }, catalog).report);
    assert.equal(short.duration_ms, 500);
    assert.deepEqual(short.captions, [{ start_ms: 0, end_ms: 500, lines: ['Hallo'], scene_id: 'a', block_id: 'a:n' }]);
    // A 1 s film holds 5 words, which give 3 captions. The estimate is 5 / 2.3 = 2.17 s. The second start, 875 ms, is raised to 1000 and
    // the third, 1750 ms, to 2000. Both start at or past the film end, so only the first stays.
    const cut = plain(SM.compile({ scenes: [{ id: 'a', duration: 1, narration: words(5) }] }, {}, catalog).report);
    assert.deepEqual(times(cut.captions), [[0, 1000]]);
});

check('compile report: the captions equal the fixture that Go parses', () => {
    // Story of internal/storymaker/render/testdata/report_measured.json. s1 has 55 units: 'Das Paket kommt heute an. Die Firewall' is 38
    // columns, and adding ' prueft' would make 45, so 'prueft es genau.' is the second line of one caption. cueMap(0) = 0, and
    // cueMap(55) = 2250 + 2350 = 4600. s2 starts its block at 5000 ms and lasts 2000 ms. Neither end meets the next start or the film end.
    const story = { scenes: [
        { id: 's1', narration: 'Das Paket kommt heute an. {1}Die Firewall prueft es genau.' },
        { id: 's2', transition: 'fade', narration: 'Das ist das Ende vom Film.' }
    ] };
    const audio = { narration: {
        s1: { blocks: [block('s1:n', 4600, [piece('a'.repeat(64), 0, 1900, 26), piece('b'.repeat(64), 2250, 2350, 29)])] },
        s2: { blocks: [block('s2:n', 2000, [piece('c'.repeat(64), 0, 2000, 26)])] }
    } };
    const fixture = readJSON('internal/storymaker/render/testdata/report_measured.json');
    assert.deepEqual(plain(SM.compile(story, audio, catalog).report.captions), fixture.captions);
});
