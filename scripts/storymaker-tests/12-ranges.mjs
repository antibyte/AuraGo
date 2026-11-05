// Task M3: the generated code point tables and the text rules on them, in JS and in Go.
import assert from 'node:assert/strict';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { spawnSync } from 'node:child_process';
import { root, read } from '../storymaker-test-support.mjs';
import { t } from '../storymaker-test-context.mjs';

const { check } = t;
const SM = t.core();
const GENERATOR = path.join(root, 'scripts', 'build-storymaker-ranges.mjs');
const JS_OUT = 'ui/js/storymaker/ranges.js', GO_OUT = 'internal/storymaker/ranges_gen.go';
const text = (...cps) => String.fromCodePoint(...cps);
const HAN = 0x6F22, ZI = 0x5B57, KA = 0x915, VIRAMA = 0x94D, SSA = 0x937;

function goTable(src, name) {
    const at = src.indexOf('var ' + name + ' = []ucdRange{');
    assert.ok(at >= 0, name + ' is in ' + GO_OUT);
    const body = src.slice(at, src.indexOf('\n}', at));
    return [...body.matchAll(/\{0x([0-9A-F]+), 0x([0-9A-F]+)\}/g)].map(m => [parseInt(m[1], 16), parseInt(m[2], 16)]);
}

check('ranges: canBreakAfter keeps surrogate pairs, marks, conjuncts and kinsoku together', () => {
    const R = SM.ranges;
    assert.equal(R.canBreakAfter(0xD83D, 0xDE00), false, 'inside a surrogate pair');
    assert.equal(R.canBreakAfter(0x41, 0x300), false, 'before a combining mark');
    assert.equal(R.canBreakAfter(KA, VIRAMA), false, 'before the virama, which is a mark');
    assert.equal(R.canBreakAfter(VIRAMA, SSA), false, 'after U+094D');
    assert.equal(R.canBreakAfter(0x200D, 0x1F600), false, 'after U+200D');
    assert.equal(R.canBreakAfter(HAN, 0x3002), false, 'no line starts with an ideographic full stop');
    assert.equal(R.canBreakAfter(0x300C, HAN), false, 'no line ends with an opening corner bracket');
    assert.equal(R.canBreakAfter(HAN, ZI), true);
    assert.equal(R.cjkBreak(HAN, ZI), true);
    assert.equal(R.cjkBreak(0x41, HAN), false, 'only between two CJK word characters');
    assert.equal(R.cjkBreak(HAN, 0x3002), false, 'kinsoku also holds for cjkBreak');
});

check('ranges: wordCount and width', () => {
    const R = SM.ranges;
    assert.equal(R.wordCount(text(HAN, ZI, HAN, ZI)), 2, '4 Han: ceil(4/2)');
    assert.equal(R.wordCount(text(HAN, ZI, HAN)), 2, '3 Han: ceil(3/2)');
    assert.equal(R.wordCount('two words'), 2, 'each run of other characters counts 1');
    assert.equal(R.wordCount('abc' + text(HAN, ZI)), 2, 'abc is one run, the Han pair is ceil(2/2)');
    assert.equal(R.wordCount(text(HAN, ZI, 0x3000, HAN, ZI)), 2, 'U+3000 is whitespace, not a CJK word character');
    assert.equal(R.wordCount(text(0x20, 0xA0)), 0, 'whitespace alone');
    assert.equal(R.width('a' + text(HAN)), 3, '1 + 2');
    assert.equal(R.width(text(0x20000)), 2, 'a wide character outside the BMP counts once');
    assert.equal(R.width(text(0x1F600)), 1, 'not in the wide list');
});

check('ranges: dirOf takes the first strong character', () => {
    const R = SM.ranges, arabic = text(0x645, 0x631, 0x62D, 0x628, 0x627), hebrew = text(0x5E9, 0x5DC, 0x5D5, 0x5DD);
    assert.equal(R.dirOf(arabic), 'rtl');
    assert.equal(R.dirOf(hebrew), 'rtl');
    assert.equal(R.dirOf('123 ' + arabic), 'rtl', 'digits are not strong');
    assert.equal(R.dirOf('Hi ' + arabic), 'ltr');
    assert.equal(R.dirOf('123'), 'ltr', 'no strong character');
    assert.equal(R.dirOf(''), 'ltr');
});

check('ranges: the sets of frozen section 4.2', () => {
    const R = SM.ranges;
    assert.deepEqual(Object.keys(R).sort(), ['canBreakAfter', 'cjkBreak', 'dirOf', 'isCjkWord', 'isDefaultIgnorable', 'isMark', 'isSpace', 'isWide', 'width', 'wordCount']);
    for (const cp of [0x9, 0xD, 0x20, 0xA0, 0x1680, 0x2000, 0x200A, 0x2028, 0x2029, 0x202F, 0x205F, 0x3000, 0xFEFF]) assert.equal(R.isSpace(cp), true, cp.toString(16));
    assert.equal(R.isSpace(0x200B), false, 'U+200B is not JS whitespace');
    assert.equal(R.isMark(0x300), true);
    assert.equal(R.isMark(VIRAMA), true);
    assert.equal(R.isMark(0x41), false);
    assert.equal(R.isDefaultIgnorable(0x200C), true);
    assert.equal(R.isDefaultIgnorable(0x200D), true);
    assert.equal(R.isDefaultIgnorable(0x41), false);
    assert.equal(R.isWide(0xAC00), true, 'Hangul is wide');
    assert.equal(R.isCjkWord(0xAC00), false, 'but not a CJK word character');
    assert.equal(R.isCjkWord(0x3042), true, 'kana');
});

check('outputs: both name Unicode 16.0.0, and the Go tables hold the same code points as the JS functions', () => {
    assert.match(read(JS_OUT).split('\n', 2).join('\n'), /Unicode 16\.0\.0/);
    const go = read(GO_OUT);
    assert.match(go.split('\n', 1)[0], /Unicode 16\.0\.0/);
    for (const [name, fn] of [['ucdEastAsianWide', SM.ranges.isWide], ['ucdCJKWord', SM.ranges.isCjkWord], ['ucdDefaultIgnorable', SM.ranges.isDefaultIgnorable]]) {
        const table = goTable(go, name), inGo = new Uint8Array(0x110000);
        assert.ok(table.length > 0, name + ' has ranges');
        for (const [lo, hi] of table) inGo.fill(1, lo, hi + 1);
        for (let cp = 0; cp < 0x110000; cp++) {
            if (fn(cp) !== (inGo[cp] === 1)) assert.fail(name + ' differs from the JS table at ' + cp.toString(16));
        }
    }
});

check('generator: --check accepts its own output and refuses one changed byte in either file', () => {
    const run = (...args) => spawnSync(process.execPath, [GENERATOR, ...args], { encoding: 'utf8' });
    assert.equal(run('--check').status, 0, 'the committed outputs match the data file');
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'sm-ranges-'));
    try {
        assert.equal(run('--out', dir).status, 0);
        assert.equal(run('--check', '--out', dir).status, 0);
        for (const rel of [JS_OUT, GO_OUT]) {
            const file = path.join(dir, rel), good = fs.readFileSync(file), bad = Buffer.from(good);
            bad[bad.length >> 1] ^= 1;
            fs.writeFileSync(file, bad);
            assert.equal(run('--check', '--out', dir).status, 1, rel + ' with one changed byte');
            fs.writeFileSync(file, good);
            assert.equal(run('--check', '--out', dir).status, 0, rel + ' restored');
        }
    } finally {
        fs.rmSync(dir, { recursive: true, force: true });
    }
});
