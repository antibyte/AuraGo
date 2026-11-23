// Unit tests for the pure project replace helpers in ui/js/desktop/apps/code-studio/replace.js.
// The helpers run in a VM together with escapeRegExp from find.js, which they share in the bundle.
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import vm from 'node:vm';

function sourceBetween(source, start, end, label) {
    const first = source.indexOf(start);
    assert.notEqual(first, -1, `${label}: missing start marker`);
    const last = source.indexOf(end, first + start.length);
    assert.notEqual(last, -1, `${label}: missing end marker`);
    return source.slice(first, last);
}

const findSource = await readFile(new URL('../ui/js/desktop/apps/code-studio/find.js', import.meta.url), 'utf8');
const replaceSource = await readFile(new URL('../ui/js/desktop/apps/code-studio/replace.js', import.meta.url), 'utf8');
const harness = [
    sourceBetween(findSource, '    function escapeRegExp(value) {', '    function findMatches(text, query, options) {', 'find.js escapeRegExp'),
    sourceBetween(replaceSource, '    // --- Project replace: pure helpers.', '    // --- End of the pure project replace helpers. ---', 'replace.js pure helpers'),
    'window.__replace = { buildReplaceRegExp, expandReplaceTemplate, replaceLineSegments, replaceSegmentsText, replaceMatchPreview, sameSearchLine, planFileReplace, bufferSearchResults };'
].join('\n');
const context = { window: {} };
vm.runInNewContext(harness, context, { filename: 'code-studio-replace-helpers.js' });
const r = context.window.__replace;
const plain = value => JSON.parse(JSON.stringify(value));
const job = (query, options, replacement) => ({ regex: r.buildReplaceRegExp(query, options), replacement, regexMode: !!(options && options.regex) });
const apply = (line, query, options, replacement) => r.replaceSegmentsText(r.replaceLineSegments(line, job(query, options, replacement)));

// Building the RegExp: empty and broken patterns give null, the case switch decides the i flag.
assert.equal(r.buildReplaceRegExp('', {}), null);
assert.equal(r.buildReplaceRegExp('(', { regex: true }), null);
assert.notEqual(r.buildReplaceRegExp('(', {}), null, 'a literal ( is escaped, not compiled');
assert.equal(r.buildReplaceRegExp('a', { case: true }).flags, 'g');
assert.equal(r.buildReplaceRegExp('a', {}).flags, 'gi');

// Literal text: special characters are escaped; the default search ignores case.
assert.equal(apply('a.b axb A.B', 'a.b', {}, 'X'), 'X axb X');
assert.equal(apply('v = (x+1)*[y]$ + 2', '(x+1)*[y]$', {}, 'W'), 'v = W + 2');
assert.equal(apply('end {z}|\\q', '{z}|\\q', {}, 'ok'), 'end ok');
// Outside regex mode the replacement is literal text: $1 and $& stay as typed.
assert.equal(apply('cost: price', 'price', {}, '$1 $& $$'), 'cost: $1 $& $$');

// Match case.
assert.equal(apply('Foo foo FOO', 'Foo', { case: true }, 'Bar'), 'Bar foo FOO');
assert.equal(apply('Foo foo FOO', 'Foo', {}, 'Bar'), 'Bar Bar Bar');

// Whole word uses \b, for literal text and for a regex (wrapped in a group, so alternatives stay inside).
assert.equal(apply('cat concat cat_x cat.', 'cat', { whole: true }, 'dog'), 'dog concat cat_x dog.');
assert.equal(apply('car cart scar cat', 'ca[rt]', { regex: true, whole: true }, 'X'), 'X cart scar X');

// Regex mode with $1-style groups.
assert.equal(apply('me@host and you@there', '(\\w+)@(\\w+)', { regex: true }, '$2 at $1'), 'host at me and there at you');
// Every replacement template behaves exactly like String.prototype.replace.
for (const template of ['$1', '$2', '$&', '$$', '$`', "$'", '$0', '$00', '$01', '$10', '$9', '$<word>', '$<nope>', '$<', 'end$', '[$1|$2]']) {
    for (const line of ['abcabd', 'xbx', 'no match here']) {
        const source = '(?<word>b)(c)?';
        assert.equal(apply(line, source, { regex: true }, template), line.replace(new RegExp(source, 'gi'), template), `template ${template} on ${line}`);
    }
}
assert.equal(apply('ab', '(b)', { regex: true }, '$<x>'), 'ab'.replace(/(b)/gi, '$<x>'), '$<name> without named groups stays literal');

// Several matches on one line; the preview of one occurrence leaves the others plain.
const multi = r.replaceLineSegments('foo bar foo baz foo', job('foo', {}, 'qux'));
assert.equal(multi.filter(segment => segment.match).length, 3);
assert.equal(r.replaceSegmentsText(multi), 'qux bar qux baz qux');
assert.deepEqual(plain(r.replaceMatchPreview(multi, 1)), { before: 'foo bar ', old: 'foo', next: 'qux', after: ' baz foo' });
assert.deepEqual(plain(r.replaceMatchPreview(r.replaceLineSegments('        indented foo', job('foo', {}, 'x')), 0)), { before: 'indented ', old: 'foo', next: 'x', after: '' });

// Empty replacement deletes the matches.
assert.equal(apply('foo bar foo', 'foo', {}, ''), ' bar ');
assert.equal(apply('a1b22c', '\\d+', { regex: true }, ''), 'abc');
// Zero-length matches are skipped.
assert.equal(apply('abc', 'x*', { regex: true }, '-'), 'abc');
assert.equal(apply('baaa', 'a*', { regex: true }, '-'), 'b-');

// Stale check: indentation and CR do not count, other text does.
assert.equal(r.sameSearchLine('\tvalue\r', 'value'), true);
assert.equal(r.sameSearchLine('value 2', 'value'), false);

// Whole-file plans: one exact occurrence, every occurrence, CRLF kept, stale and no-match files.
const crlf = 'one foo\r\ntwo foo foo\r\nthree\r\n';
const fooJob = job('foo', {}, 'X');
const single = r.planFileReplace(crlf, [{ line: 2, preview: 'two foo foo', occurrence: 1 }], fooJob);
assert.equal(single.status, 'ok');
assert.equal(single.count, 1);
assert.equal(single.content, 'one foo\r\ntwo foo X\r\nthree\r\n', 'only the second occurrence of line 2 changes');
assert.deepEqual(plain(single.changes), [{ from: 17, to: 20, insert: 'X' }]);
const whole = r.planFileReplace(crlf, [{ line: 1, preview: 'one foo', occurrence: null }, { line: 2, preview: '  two foo foo  ', occurrence: null }], fooJob);
assert.equal(whole.content, 'one X\r\ntwo X X\r\nthree\r\n');
assert.equal(whole.count, 3);
assert.equal(r.planFileReplace(crlf, [{ line: 2, preview: 'two foo', occurrence: 0 }], fooJob).status, 'stale', 'changed line text');
assert.equal(r.planFileReplace(crlf, [{ line: 9, preview: 'two foo foo', occurrence: 0 }], fooJob).status, 'stale', 'line beyond the end');
assert.equal(r.planFileReplace(crlf, [{ line: 2, preview: 'two foo foo', occurrence: 5 }], fooJob).status, 'stale', 'occurrence gone');
assert.equal(r.planFileReplace(crlf, [{ line: 3, preview: 'three', occurrence: null }], fooJob).status, 'nomatch');
const swap = r.planFileReplace('let a = b;\n', [{ line: 1, preview: 'let a = b;', occurrence: null }], job('(\\w) = (\\w)', { regex: true }, '$2 = $1'));
assert.equal(swap.content, 'let b = a;\n');

// Buffer results: line numbers of the buffer, CR stripped from the preview.
assert.deepEqual(plain(r.bufferSearchResults('/workspace/a.txt', '// local\nkeep foo here\r\nno\n', r.buildReplaceRegExp('foo', {}))), [{ path: '/workspace/a.txt', line: 2, preview: 'keep foo here' }]);

console.log('Code Studio replace helper tests passed');
