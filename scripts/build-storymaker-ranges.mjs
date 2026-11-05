// Generates the Storymaker code point tables from one committed data file, so Chrome and Node ICU never decide at run
// time. One source, two outputs: ui/js/storymaker/ranges.js (StorymakerCore.ranges) and internal/storymaker/ranges_gen.go.
//
//   node scripts/build-storymaker-ranges.mjs                  write both outputs
//   node scripts/build-storymaker-ranges.mjs --check          exit 1 when an output differs (CRLF checkouts compare as LF)
//   node scripts/build-storymaker-ranges.mjs --out <dir>      use <dir> as the root of the outputs (the tests do)
//   node scripts/build-storymaker-ranges.mjs --extract <dir>  rewrite the data file from the three Unicode files in <dir>
//
// No regex property escapes here: ECMAScript has no Bidi_Class, and category escapes would tie --check to the Node version.
import fs from 'node:fs';
import path from 'node:path';

const VERSION = '16.0.0';
const repo = path.resolve(import.meta.dirname, '..');
const DATA_FILE = path.join(repo, 'scripts', 'storymaker-ucd-' + VERSION + '.txt');
const JS_OUT = 'ui/js/storymaker/ranges.js';
const GO_OUT = 'internal/storymaker/ranges_gen.go';

// The JS whitespace set. Go repeats it by hand in IsJSSpace, and the shared narration fixture compares the two.
const SPACE = [[0x9, 0xD], [0x20, 0x20], [0xA0, 0xA0], [0x1680, 0x1680], [0x2000, 0x200A], [0x2028, 0x2029], [0x202F, 0x202F], [0x205F, 0x205F], [0x3000, 0x3000], [0xFEFF, 0xFEFF]];
const EAST_ASIAN_WIDE = [[0x1100, 0x115F], [0x2E80, 0x303E], [0x3041, 0x33FF], [0x3400, 0x4DBF], [0x4E00, 0x9FFF], [0xA000, 0xA4CF], [0xAC00, 0xD7A3], [0xF900, 0xFAFF], [0xFE30, 0xFE6F], [0xFF00, 0xFF60], [0xFFE0, 0xFFE6], [0x20000, 0x3FFFD]];
const CJK_WORD = [[0x3000, 0x30FF], [0x3400, 0x4DBF], [0x4E00, 0x9FFF], [0xF900, 0xFAFF], [0xFF00, 0xFF65], [0x20000, 0x2FFFF]];
// Kinsoku. No line start: ideographic full stop and comma, fullwidth comma, full stop, exclamation and question mark,
// fullwidth right parenthesis, right corner, white corner, black lenticular, angle and double angle brackets, fullwidth
// colon and semicolon, katakana middle dot, prolonged sound mark, iteration mark, horizontal ellipsis, two dot leader.
const NO_LINE_START = [0x3002, 0x3001, 0xFF0C, 0xFF0E, 0xFF01, 0xFF1F, 0xFF09, 0x300D, 0x300F, 0x3011, 0x3009, 0x300B, 0xFF1A, 0xFF1B, 0x30FB, 0x30FC, 0x3005, 0x2026, 0x2025];
// No line end: the left forms of corner, white corner, fullwidth parenthesis, black lenticular, angle and double angle brackets.
const NO_LINE_END = [0x300C, 0x300E, 0xFF08, 0x3010, 0x3008, 0x300A];

const JS_LOGIC = `
    // inTable(table, cp): table is a flat sorted list lo, hi, lo, hi, ... of inclusive ranges.
    function inTable(table, cp) {
        var lo = 0, hi = table.length / 2 - 1;
        while (lo <= hi) {
            var mid = (lo + hi) >> 1;
            if (cp < table[2 * mid]) hi = mid - 1;
            else if (cp > table[2 * mid + 1]) lo = mid + 1;
            else return true;
        }
        return false;
    }

    function isSpace(cp) { return inTable(SPACE, cp); }
    function isWide(cp) { return inTable(EAST_ASIAN_WIDE, cp); }
    function isCjkWord(cp) { return inTable(CJK_WORD, cp); }
    function isMark(cp) { return inTable(MARK, cp); }
    function isDefaultIgnorable(cp) { return inTable(DEFAULT_IGNORABLE, cp); }

    // width(text): same as Go TextWidth. A wide code point counts 2, any other 1.
    function width(text) {
        var n = 0;
        for (var i = 0; i < text.length; i++) {
            var cp = text.codePointAt(i);
            if (cp > 0xFFFF) i++;
            n += isWide(cp) ? 2 : 1;
        }
        return n;
    }

    // wordCount(text): same as Go WordCount. Whitespace separates; a run of n CJK word characters counts ceil(n/2); a run of
    // anything else counts 1.
    function wordCount(text) {
        var words = 0, cjk = 0, other = false;
        for (var i = 0; i < text.length; i++) {
            var cp = text.codePointAt(i);
            if (cp > 0xFFFF) i++;
            if (isSpace(cp)) {
                words += Math.ceil(cjk / 2);
                cjk = 0;
                other = false;
            } else if (isCjkWord(cp)) {
                cjk++;
                other = false;
            } else {
                words += Math.ceil(cjk / 2);
                cjk = 0;
                if (!other) words++;
                other = true;
            }
        }
        return words + Math.ceil(cjk / 2);
    }

    // dirOf(text): the first strong character decides; no strong character is ltr.
    function dirOf(text) {
        for (var i = 0; i < text.length; i++) {
            var cp = text.codePointAt(i);
            if (cp > 0xFFFF) i++;
            if (inTable(STRONG_RTL, cp)) return 'rtl';
            if (inTable(STRONG_LTR, cp)) return 'ltr';
        }
        return 'ltr';
    }

    // canBreakAfter(prevCp, cp): false inside a surrogate pair, before a mark, after U+200D or U+094D, and by kinsoku.
    function canBreakAfter(prevCp, cp) {
        if (prevCp >= 0xD800 && prevCp <= 0xDBFF && cp >= 0xDC00 && cp <= 0xDFFF) return false;
        if (isMark(cp) || prevCp === 0x200D || prevCp === 0x094D) return false;
        return !inTable(NO_LINE_START, cp) && !inTable(NO_LINE_END, prevCp);
    }

    function cjkBreak(prevCp, cp) { return isCjkWord(prevCp) && isCjkWord(cp) && canBreakAfter(prevCp, cp); }

    SM.ranges = {
        isSpace: isSpace, isWide: isWide, isCjkWord: isCjkWord, isMark: isMark, isDefaultIgnorable: isDefaultIgnorable,
        width: width, wordCount: wordCount, dirOf: dirOf, canBreakAfter: canBreakAfter, cjkBreak: cjkBreak
    };
`;

const hex = n => n.toString(16).toUpperCase();

function merge(ranges) {
    const out = [];
    for (const [lo, hi] of ranges.slice().sort((a, b) => a[0] - b[0])) {
        const last = out[out.length - 1];
        if (last && lo <= last[1] + 1) last[1] = Math.max(last[1], hi);
        else out.push([lo, hi]);
    }
    return out;
}

const singles = cps => merge(cps.map(cp => [cp, cp]));

// wrapItems(items, indent, width): items joined by one space, a new line (starting with indent) before width is passed.
function wrapItems(items, indent, width) {
    const lines = [];
    let line = indent;
    for (const item of items) {
        if (line.length + item.length > width && line !== indent) {
            lines.push(line.trimEnd());
            line = indent;
        }
        line += item + ' ';
    }
    if (line !== indent) lines.push(line.trimEnd());
    return lines;
}

// extract(dir): reads DerivedGeneralCategory.txt, DerivedCoreProperties.txt and DerivedBidiClass.txt and rewrites the data file.
function extract(dir) {
    const rows = name => {
        const text = fs.readFileSync(path.join(dir, name), 'utf8');
        const first = text.slice(0, text.indexOf('\n'));
        if (!first.includes(name.replace('.txt', '-' + VERSION + '.txt'))) throw new Error(name + ' is not the Unicode ' + VERSION + ' file: ' + first);
        return text.split(/\r?\n/).map(l => l.split('#')[0].trim()).filter(Boolean).map(l => l.split(';').map(f => f.trim()));
    };
    const range = spec => {
        const [a, b] = spec.split('..');
        return [parseInt(a, 16), parseInt(b || a, 16)];
    };
    const pick = (name, test) => merge(rows(name).filter(test).map(r => range(r[0])));
    const packed = (key, ranges) => wrapItems(ranges.map(([lo, hi]) => (lo === hi ? hex(lo) : hex(lo) + '-' + hex(hi))), key + ' ', 110);
    const text = [
        '# Storymaker code point data. Written once by scripts/build-storymaker-ranges.mjs --extract from the Unicode ' + VERSION,
        '# files DerivedGeneralCategory.txt, DerivedCoreProperties.txt and extracted/DerivedBidiClass.txt. Hex, inclusive.',
        '# M: general category Mn, Mc, Me. DI: Default_Ignorable_Code_Point. R, AL, L: Bidi_Class.',
        'version ' + VERSION,
        ...packed('M', pick('DerivedGeneralCategory.txt', r => r[1] === 'Mn' || r[1] === 'Mc' || r[1] === 'Me')),
        ...packed('DI', pick('DerivedCoreProperties.txt', r => r[1] === 'Default_Ignorable_Code_Point')),
        ...packed('R', pick('DerivedBidiClass.txt', r => r[1] === 'R')),
        ...packed('AL', pick('DerivedBidiClass.txt', r => r[1] === 'AL')),
        ...packed('L', pick('DerivedBidiClass.txt', r => r[1] === 'L')),
        ''
    ].join('\n');
    fs.writeFileSync(DATA_FILE, text);
    console.log('Wrote ' + path.relative(repo, DATA_FILE));
}

function parseData(text) {
    const tables = { M: [], DI: [], R: [], AL: [], L: [] };
    let version = '';
    for (const raw of text.split(/\r?\n/)) {
        if (!raw || raw.startsWith('#')) continue;
        const parts = raw.split(' ');
        if (parts[0] === 'version') {
            version = parts[1];
            continue;
        }
        const table = tables[parts[0]];
        if (!table) throw new Error('Unknown table in the data file: ' + parts[0]);
        for (const item of parts.slice(1)) {
            const [a, b] = item.split('-');
            table.push([parseInt(a, 16), parseInt(b || a, 16)]);
        }
    }
    if (version !== VERSION) throw new Error('The data file has version ' + version + ', the generator wants ' + VERSION);
    for (const [name, table] of Object.entries(tables)) {
        if (!table.length) throw new Error('The data file has no ' + name + ' table');
    }
    return tables;
}

function jsTable(name, ranges) {
    const items = ranges.map(([lo, hi]) => '0x' + hex(lo) + ',0x' + hex(hi) + ',');
    return ['    var ' + name + ' = [', ...wrapItems(items, '        ', 118), '    ];'];
}

function renderJS(data) {
    return [
        '// Storymaker player: code point tables and the text rules on them. Generated by scripts/build-storymaker-ranges.mjs from',
        '// scripts/storymaker-ucd-' + VERSION + '.txt (Unicode ' + VERSION + '). Do not edit. Classic script; publishes StorymakerCore.ranges.',
        '(function (root) {',
        "    'use strict';",
        '    var SM = root.StorymakerCore || (root.StorymakerCore = {});',
        '',
        ...jsTable('SPACE', SPACE),
        ...jsTable('EAST_ASIAN_WIDE', EAST_ASIAN_WIDE),
        ...jsTable('CJK_WORD', CJK_WORD),
        ...jsTable('MARK', data.M),
        ...jsTable('DEFAULT_IGNORABLE', data.DI),
        ...jsTable('STRONG_RTL', merge([...data.R, ...data.AL])),
        ...jsTable('STRONG_LTR', data.L),
        ...jsTable('NO_LINE_START', singles(NO_LINE_START)),
        ...jsTable('NO_LINE_END', singles(NO_LINE_END)),
        '',
        JS_LOGIC.replace(/^\n+|\n+$/g, ''),
        "})(typeof globalThis !== 'undefined' ? globalThis : self);",
        ''
    ].join('\n');
}

function goTable(name, doc, ranges) {
    const items = ranges.map(([lo, hi]) => '{0x' + hex(lo) + ', 0x' + hex(hi) + '},');
    return ['// ' + doc, 'var ' + name + ' = []ucdRange{', ...wrapItems(items, '\t', 100), '}', ''];
}

function renderGo(data) {
    return [
        '// Code generated by scripts/build-storymaker-ranges.mjs from scripts/storymaker-ucd-' + VERSION + '.txt (Unicode ' + VERSION + '). DO NOT EDIT.',
        '',
        'package storymaker',
        '',
        '// ucdVersion is the Unicode version of the data behind these tables.',
        'const ucdVersion = "' + VERSION + '"',
        '',
        '// ucdRange is an inclusive code point range. A table is sorted and its ranges do not overlap.',
        'type ucdRange struct{ lo, hi rune }',
        '',
        ...goTable('ucdEastAsianWide', 'ucdEastAsianWide is EAST_ASIAN_WIDE: a code point here counts 2 in TextWidth.', EAST_ASIAN_WIDE),
        ...goTable('ucdCJKWord', 'ucdCJKWord is CJK_WORD: a run of n of these counts ceil(n/2) in WordCount.', CJK_WORD),
        ...goTable('ucdDefaultIgnorable', 'ucdDefaultIgnorable is Default_Ignorable_Code_Point.', data.DI),
        '// inUCDTable reports whether r lies in one of the ranges of table.',
        'func inUCDTable(table []ucdRange, r rune) bool {',
        '\tlo, hi := 0, len(table)-1',
        '\tfor lo <= hi {',
        '\t\tmid := (lo + hi) / 2',
        '\t\tswitch {',
        '\t\tcase r < table[mid].lo:',
        '\t\t\thi = mid - 1',
        '\t\tcase r > table[mid].hi:',
        '\t\t\tlo = mid + 1',
        '\t\tdefault:',
        '\t\t\treturn true',
        '\t\t}',
        '\t}',
        '\treturn false',
        '}',
        ''
    ].join('\n');
}

const args = process.argv.slice(2);
const valueOf = flag => args[args.indexOf(flag) + 1];

if (args.includes('--extract')) {
    extract(path.resolve(valueOf('--extract')));
} else {
    const outRoot = args.includes('--out') ? path.resolve(valueOf('--out')) : repo;
    const data = parseData(fs.readFileSync(DATA_FILE, 'utf8'));
    const outputs = [[JS_OUT, renderJS(data)], [GO_OUT, renderGo(data)]];
    if (args.includes('--check')) {
        const stale = outputs.filter(([rel, text]) => {
            const file = path.join(outRoot, rel);
            return !fs.existsSync(file) || fs.readFileSync(file, 'utf8').replace(/\r\n/g, '\n') !== text;
        });
        for (const [rel] of stale) console.error('Outdated ' + rel + '; run node scripts/build-storymaker-ranges.mjs');
        if (stale.length) process.exit(1);
        console.log('storymaker ranges (Unicode ' + VERSION + ') are up to date');
    } else {
        for (const [rel, text] of outputs) {
            const file = path.join(outRoot, rel);
            fs.mkdirSync(path.dirname(file), { recursive: true });
            fs.writeFileSync(file, text);
            console.log('Built ' + rel);
        }
    }
}
