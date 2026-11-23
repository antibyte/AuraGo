# Code Studio IDE Package 1C: Project-wide Replace Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task by task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add VS Code style project-wide replace to the Code Studio search panel. The parts are:
- a replace row that a chevron toggles;
- a preview for each match (old text struck through, new text after it);
- three actions: Replace (one exact match), Replace in File, and Replace All with the confirm "Replace N occurrences in M files?";
- open files change in their editor buffer as one undoable transaction;
- closed files are read with `GET /api/code-studio/file` and written with `PUT /api/code-studio/file`;
- read-only mode turns replace off;
- the search runs again after every replace.

There is no backend change.

**Architecture:** A new bundle fragment `ui/js/desktop/apps/code-studio/replace.js` has two parts.
1. **Pure helpers, no DOM and no studio state.** They build the RegExp, apply the JS replacement-template semantics, split a line into segments, build the preview parts, re-check stale lines, plan a whole file, and compute search results from a buffer. A Node `vm` test loads them. The preview and the write both use `replaceLineSegments`, so the preview always shows what will be written.
2. **The studio side.** It holds the replace-row markup, the grouped result markup, the actions, the per-file loop (one file's error never aborts the others), the Output-panel report and the search re-run.

`search.js` splits rendering into `renderSearchPanel` (head: chevron, form, replace row) and `renderSearchResults` (list only, so typing in the replace field keeps the caret). It also puts open-tab buffers in place of server results for those files. `core.js` gets one line in `updatePolicyControls` so that read-only changes reach the replace controls.

**Tech Stack:** Vanilla JS fragments in the shared Code Studio IIFE (concatenated by `scripts/build-ui-bundles.js`), CodeMirror 6 transactions, existing Code Studio file and search APIs, Node `node:vm` + `node:assert/strict`, Go + go-rod browser test against the in-memory `newCodeStudioBrowser` fixture, flat i18n JSON (16 desktop locales).

---

## Conventions for this plan

- **Plans A and B land first on the same tree.** They change `core.js` (command list, status bar, split panes, tab kinds, `isVirtualTab(tab)`), `editor.js` and `shortcuts.js`. They do not change `search.js`.
- **Shared regions this plan edits.** Each edit is anchored on text that A and B do not change:
  - `core.js` `updatePolicyControls`: anchored on its last two lines (the `knowledge` button).
  - `scripts/build-ui-bundles.js`: one-line anchor on the `search.js` part.
  - `ui/desktop_code_studio_browser_test.go` fixture: the struct header line and the one search `writeJSON` line.
  - `ui/css/code-studio.css`: append only.
  - Locale files: text insertion after `"codeStudio.replaceDone"`.
  - `package.json`: one-line anchor on `test:docker-manager`.
- **Anchors decide, line numbers do not.** Line numbers in the text are hints only. If an anchor is not found because A or B reformatted a line, put the new code at the same logical place (for example, as the last statement of `updatePolicyControls`) and say so in your report.
- **GitNexus (repo CLAUDE.md):**
  - Run `impact` before editing a symbol: MCP `impact({target, direction:"upstream"})`, or the CLI `node ../../../.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "<symbol>" --direction upstream --repo .` from the repo root.
  - Run `detect-changes --scope all` before every commit, with the same runner path.
  - A result of `risk: UNKNOWN` means "not resolved". Confirm the callers with Grep. This plan lists the callers it expects.
- **Shell and tools:**
  - Windows, PowerShell.
  - Browser tests start Chrome. Run them with the PowerShell tool and the sandbox off.
  - If `go test` fails with `no such tool "vet"`, prefix the command with `$env:GOTOOLCHAIN='go1.27.4';`.
  - If another session's half-written untracked Go file breaks the `ui` build, use `go test -overlay` with an empty replacement for that file (see the memory note). Do not edit that file.
- **No git commands in this plan.** Every task ends with "Commit:" and a file list. The controller commits through a temporary index.
- **`npm run build:ui` rebuilds every bundle.** Commit only `ui/js/desktop/bundles/code-studio.bundle.js` from the generated files. Other changed bundles come from other sessions' sources.
- **The Write tool decodes `\u` escapes.** The code below has none. Copy it as it stands. The `…` and `›` characters are meant to be literal UTF-8.

## Behaviour decisions (read before coding)

1. **The server returns one result per line.** Results are `{path, line, preview}`, capped at 200, with `truncated: true` beyond that. The search uses grep: `-F` or `-E`, `-i` and `-w`.
   - Replace changes **only the listed lines**.
   - "Replace one" changes the occurrence with index `k` on line `n`, counting matches of the JS RegExp on that line.
   - Replace in File and Replace All change every JS match on the listed lines of the file.
   - Together this keeps the preview equal to the write.
2. **Every target line is re-checked before anything is applied.** The check is `sameSearchLine(lineText, preview)`, a comparison after `trim()`. Trimming ignores indentation and a trailing CR; the test fixture also trims previews.
   - If any target line of a file differs, the whole file is "stale": it is skipped and reported.
   - In "one" mode the status bar says "The file changed since the search. Searching again.", and the search runs again.
3. **Open files: results come from their buffers.** After a search, `overlayOpenTabResults` takes every file the server listed that is open in a non-virtual tab and recomputes its result lines from the buffer with the JS RegExp.
   - Without this, a buffer with unsaved edits that shift lines would stay stale forever, because the search re-run reads the disk again.
   - Limitation: an open file whose disk copy has no match, but whose buffer does, is not listed. The server never names it.
4. **How a change reaches an open tab:**
   - **Live CodeMirror view:** one `view.dispatch({changes, userEvent:'input.replace'})`. That is one undo step; `input.replace` never joins typing history. The update listener marks the tab modified.
   - **Textarea fallback:** `view.setValue(text)`.
   - **Background tab with no live view:** the change is applied as one transaction to `tab.editorSnapshot.state`, and then `updateTabContent` runs. `renderEditor` later restores that state, so the change shows and can be undone. Without this step the old snapshot would bring the old text back.
   - Open tabs are never written to disk.
5. **Literal mode (regex off):** the replacement text is inserted verbatim, so `$1` and `$&` stay as typed, as in VS Code. **Regex mode:** `expandReplaceTemplate` implements ECMAScript GetSubstitution (`$$ $& $\` $' $1..$99 $<name>`). The Node test checks it against the native `String.prototype.replace`.
6. **Zero-length matches are skipped,** as in `find.js`.
7. **Closed files:**
   - A GET answer of 413 is reported as "larger than the file size limit, skipped". This is the server size cap.
   - A file whose content contains U+FFFD is skipped: it is not valid UTF-8, and writing it back would corrupt it.
   - A tab opened for the file while it was being read takes the change in its buffer instead.
   - A tab opened after the PUT is reloaded with `reloadCleanTabFromDisk`.
8. **Every per-file outcome lands in the Output panel.** Source label: the existing `codeStudio.replaceOne` "Replace".
   - The panel opens only when something was skipped or failed.
   - One file's error never stops the loop.
   - After the loop, the search runs again with the last search's options.
9. **Replace All is off when:** the list is truncated, the regex does not compile in JS, the result list has no JS match, a replace is running, or the studio is read-only.
   - A hint explains which applies.
   - Replace in File stays available for the listed files.
10. **Read-only:** the replace input and every replace button are disabled. Every action checks `canWrite` again before buffer edits and before each PUT.
11. **Existing keys are reused:**
    - `codeStudio.replacePlaceholder` ("Replace"), for the input
    - `codeStudio.replaceAll`
    - `codeStudio.replaceOne`
    - `codeStudio.replaceInFile` ("Replace in File"), for the per-file button
    - `codeStudio.findToggleReplace`, for the chevron

    Sixteen keys are new.

## File Structure

| File | Change | Responsibility |
| --- | --- | --- |
| `ui/js/desktop/apps/code-studio/replace.js` | Create (Task 1), append (Task 3) | Pure helpers (Task 1); studio-side replace UI and actions (Task 3) |
| `scripts/test-code-studio-replace.mjs` | Create | Node `vm` unit test of the pure helpers |
| `package.json` | Modify | `test:code-studio-replace` script |
| `C:\disposable\code-studio-pkg1\locales-c.mjs` | Create (outside the repo, not committed) | Inserts 16 keys into 16 locale files |
| `ui/lang/desktop/{cs,da,de,el,en,es,fr,hi,it,ja,nl,no,pl,pt,sv,zh}.json` | Modify | New `codeStudio.replace*` strings |
| `ui/js/desktop/apps/code-studio/search.js` | Overwrite in full | Head with chevron, form and replace row; `renderSearchResults`; buffer overlay in `runSearch` |
| `ui/js/desktop/apps/code-studio/core.js` | Modify (1 line) | `updatePolicyControls` calls `updateProjectReplaceControls()` |
| `scripts/build-ui-bundles.js` | Modify (1 line) | `replace.js` part after `search.js` |
| `ui/css/code-studio.css` | Append | `/* === IDE package 1C: replace === */` block |
| `ui/js/desktop/bundles/code-studio.bundle.js` | Generated | `npm run build:ui` |
| `ui/desktop_code_studio_browser_test.go` | Modify | Fixture flag `searchTruncated` |
| `ui/desktop_code_studio_replace_browser_test.go` | Create | One browser test for the replace area |

---

### Task 1: Pure replace helpers and their Node test

**Files:**
- Create: `ui/js/desktop/apps/code-studio/replace.js`
- Create: `scripts/test-code-studio-replace.mjs`
- Modify: `package.json`

- [ ] **Step 1: Create `ui/js/desktop/apps/code-studio/replace.js` with this complete content**

The file is a fragment inside the shared IIFE, indented 4 spaces like its siblings. It is not in the bundle yet; Task 3 adds it.

```js
    // Project-wide replace for the search panel. The server search (grep) lists the matching lines;
    // the replace itself runs here with a JavaScript RegExp built from the same options. The preview
    // and the write both go through replaceLineSegments, so the preview shows exactly what lands in a
    // file. Open files change in their editor buffer as one undoable transaction and are not written;
    // closed files are read with GET /api/code-studio/file and written back with PUT.

    // --- Project replace: pure helpers. No DOM and no studio state; scripts/test-code-studio-replace.mjs runs them in a VM. ---

    // Same options as the search: literal text is escaped (escapeRegExp from find.js), whole word wraps
    // the pattern in \b and the case switch decides the i flag. Returns null for an empty query or for a
    // pattern JavaScript cannot compile.
    function buildReplaceRegExp(query, options) {
        const needle = String(query || '');
        if (!needle) return null;
        const settings = options || {};
        let source = settings.regex ? needle : escapeRegExp(needle);
        if (settings.whole) source = settings.regex ? '\\b(?:' + source + ')\\b' : '\\b' + source + '\\b';
        try {
            return new RegExp(source, settings.case ? 'g' : 'gi');
        } catch (_) {
            return null;
        }
    }

    // GetSubstitution of String.prototype.replace: $$, $&, $`, $', $1 to $99 and $<name>.
    function expandReplaceTemplate(template, match, line) {
        const text = String(template == null ? '' : template);
        const source = String(line == null ? '' : line);
        const captures = match.length - 1;
        const groups = match.groups;
        const isDigit = ch => ch !== undefined && ch >= '0' && ch <= '9';
        let out = '';
        for (let i = 0; i < text.length; i += 1) {
            const ch = text[i];
            const next = text[i + 1];
            if (ch !== '$' || next === undefined) {
                out += ch;
            } else if (next === '$') {
                out += '$';
                i += 1;
            } else if (next === '&') {
                out += match[0];
                i += 1;
            } else if (next === '`') {
                out += source.slice(0, match.index);
                i += 1;
            } else if (next === "'") {
                out += source.slice(match.index + match[0].length);
                i += 1;
            } else if (isDigit(next)) {
                const two = isDigit(text[i + 2]) ? Number(next + text[i + 2]) : 0;
                const one = Number(next);
                if (two >= 1 && two <= captures) {
                    out += match[two] == null ? '' : match[two];
                    i += 2;
                } else if (one >= 1 && one <= captures) {
                    out += match[one] == null ? '' : match[one];
                    i += 1;
                } else {
                    out += '$';
                }
            } else if (next === '<' && groups && text.indexOf('>', i + 2) !== -1) {
                const close = text.indexOf('>', i + 2);
                const value = groups[text.slice(i + 2, close)];
                out += value == null ? '' : String(value);
                i = close;
            } else {
                out += '$';
            }
        }
        return out;
    }

    // Splits one line into plain text and matches; every match carries the text that replaces it.
    // job = { regex, replacement, regexMode }. Zero-length matches are skipped like the in-file find does.
    function replaceLineSegments(line, job) {
        const text = String(line == null ? '' : line);
        if (!job || !job.regex) return text ? [{ text }] : [];
        const flags = job.regex.flags.includes('g') ? job.regex.flags : job.regex.flags + 'g';
        const pattern = new RegExp(job.regex.source, flags);
        const replacement = String(job.replacement == null ? '' : job.replacement);
        const segments = [];
        let cursor = 0;
        let found = 0;
        let match;
        while (found < 5000 && (match = pattern.exec(text)) !== null) {
            if (match[0] === '') {
                pattern.lastIndex += 1;
                continue;
            }
            if (match.index > cursor) segments.push({ text: text.slice(cursor, match.index) });
            segments.push({
                match: true,
                from: match.index,
                to: match.index + match[0].length,
                text: match[0],
                insert: job.regexMode ? expandReplaceTemplate(replacement, match, text) : replacement
            });
            cursor = match.index + match[0].length;
            found += 1;
        }
        if (cursor < text.length) segments.push({ text: text.slice(cursor) });
        return segments;
    }

    function replaceSegmentsText(segments) {
        return (segments || []).map(segment => (segment.match ? segment.insert : segment.text)).join('');
    }

    // The parts one result row shows for one occurrence: the text before it (indentation dropped,
    // long prefixes shortened), the old text, the new text and the rest of the line. Other matches on
    // the same line stay plain.
    function replaceMatchPreview(segments, occurrence) {
        let before = '';
        let after = '';
        let found = null;
        let seen = 0;
        (segments || []).forEach(segment => {
            if (segment.match && !found && seen === occurrence) {
                found = segment;
                seen += 1;
                return;
            }
            if (segment.match) seen += 1;
            if (found) after += segment.text;
            else before += segment.text;
        });
        before = before.replace(/^\s+/, '');
        if (before.length > 40) before = '…' + before.slice(-39);
        return { before, old: found ? found.text : '', next: found ? found.insert : '', after };
    }

    // The server sends the line as grep printed it; indentation and a trailing CR do not count.
    function sameSearchLine(lineText, preview) {
        return String(lineText == null ? '' : lineText).trim() === String(preview == null ? '' : preview).trim();
    }

    // Plans the replacements for one file. targets: [{ line, preview, occurrence }]; occurrence null
    // means every match on that line. Each target line must still read like its search result, else
    // the whole file is stale. Returns { status: 'ok' | 'stale' | 'nomatch', changes, count, content };
    // changes are ascending offsets into content, ready for one CodeMirror transaction. CRLF stays.
    function planFileReplace(content, targets, job) {
        const text = String(content == null ? '' : content);
        const stale = { status: 'stale', changes: [], count: 0, content: text };
        const starts = [0];
        for (let i = 0; i < text.length; i += 1) {
            if (text.charCodeAt(i) === 10) starts.push(i + 1);
        }
        const changes = [];
        for (const target of targets || []) {
            const lineNumber = Number(target && target.line);
            if (!Number.isInteger(lineNumber) || lineNumber < 1 || lineNumber > starts.length) return stale;
            const from = starts[lineNumber - 1];
            let end = lineNumber < starts.length ? starts[lineNumber] - 1 : text.length;
            if (end > from && text.charCodeAt(end - 1) === 13) end -= 1;
            const lineText = text.slice(from, end);
            if (!sameSearchLine(lineText, target.preview)) return stale;
            const matches = replaceLineSegments(lineText, job).filter(segment => segment.match);
            let picked = matches;
            if (target.occurrence !== null && target.occurrence !== undefined) {
                const chosen = matches[Number(target.occurrence)];
                if (!chosen) return stale;
                picked = [chosen];
            }
            picked.forEach(segment => changes.push({ from: from + segment.from, to: from + segment.to, insert: segment.insert }));
        }
        changes.sort((a, b) => a.from - b.from || a.to - b.to);
        const applied = [];
        let out = '';
        let cursor = 0;
        changes.forEach(change => {
            if (change.from < cursor) return;
            out += text.slice(cursor, change.from) + change.insert;
            cursor = change.to;
            applied.push(change);
        });
        if (!applied.length) return { status: 'nomatch', changes: [], count: 0, content: text };
        return { status: 'ok', changes: applied, count: applied.length, content: out + text.slice(cursor) };
    }

    // Search results for an open buffer: the lines where the RegExp finds a non-empty match.
    function bufferSearchResults(path, content, regex, limit) {
        const found = [];
        if (!regex) return found;
        const max = Number(limit) > 0 ? Number(limit) : 200;
        const probe = { regex, replacement: '', regexMode: false };
        const lines = String(content == null ? '' : content).split('\n');
        for (let index = 0; index < lines.length && found.length < max; index += 1) {
            const line = lines[index].endsWith('\r') ? lines[index].slice(0, -1) : lines[index];
            if (replaceLineSegments(line, probe).some(segment => segment.match)) found.push({ path, line: index + 1, preview: line });
        }
        return found;
    }

    // --- End of the pure project replace helpers. ---
```

- [ ] **Step 2: Create `scripts/test-code-studio-replace.mjs` with this complete content**

```js
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
```

- [ ] **Step 3: Register the test in `package.json`**

Replace:
```json
    "test:docker-manager": "node scripts/test-docker-manager-ui.mjs",
```
With:
```json
    "test:docker-manager": "node scripts/test-docker-manager-ui.mjs",
    "test:code-studio-replace": "node scripts/test-code-studio-replace.mjs",
```

- [ ] **Step 4: Run the test**

Run: `npm run test:code-studio-replace`
Expected: last line `Code Studio replace helper tests passed`, exit code 0.

Run: `node scripts/build-ui-bundles.js --check`
Expected: every line starts with `Checked`, exit code 0. `replace.js` is not a bundle part yet, so the bundle is unchanged.

- [ ] **Step 5: Graph check.** Run `detect-changes --scope all` (see Conventions). Report the result. A `partial` or `truncated` result is not a clean check.

**Commit:** `feat(code-studio): pure helpers for project-wide replace`. Files:
- `ui/js/desktop/apps/code-studio/replace.js`
- `scripts/test-code-studio-replace.mjs`
- `package.json`

---

### Task 2: Locale strings for all 16 desktop languages

**Files:**
- Create (outside the repo, not committed): `C:\disposable\code-studio-pkg1\locales-c.mjs`
- Modify: `ui/lang/desktop/{cs,da,de,el,en,es,fr,hi,it,ja,nl,no,pl,pt,sv,zh}.json`

Shape of the locale files (checked):
- Each file is one flat JSON object with 2-space indentation and literal UTF-8 characters.
- Every file has `  "codeStudio.replaceDone": "...",` at the same position (line 12255 before Plans A and B).
- The script inserts the new lines directly after that line. Key order, indentation and line endings (CRLF or LF, detected per file) stay as they are.
- `tr()` fills `{{name}}` from vars.
- German uses informal address (no "Sie"); `ui/code_studio_status_search_test.go` checks this for other keys.

- [ ] **Step 1: Create `C:\disposable\code-studio-pkg1\locales-c.mjs` with this complete content**

```js
// Adds the Code Studio IDE package 1C (project-wide replace) strings to the 16 desktop locale files.
// The new lines go right after "codeStudio.replaceDone", so key order, indentation and line endings of
// every file stay as they are. Usage: node locales-c.mjs <repo root>
import { readFile, writeFile } from 'node:fs/promises';
import path from 'node:path';

const repo = process.argv[2] || 'C:/Users/Andi/Documents/repo/AuraGo';
const langs = ['cs', 'da', 'de', 'el', 'en', 'es', 'fr', 'hi', 'it', 'ja', 'nl', 'no', 'pl', 'pt', 'sv', 'zh'];
const anchorKey = '"codeStudio.replaceDone":';

const strings = {
    'codeStudio.replaceAllConfirm': {
        cs: "Nahradit {{count}} výskytů v {{files}} souborech?",
        da: "Erstat {{count}} forekomster i {{files}} filer?",
        de: "{{count}} Vorkommen in {{files}} Dateien ersetzen?",
        el: "Αντικατάσταση {{count}} εμφανίσεων σε {{files}} αρχεία;",
        en: "Replace {{count}} occurrences in {{files}} files?",
        es: "¿Reemplazar {{count}} coincidencias en {{files}} archivos?",
        fr: "Remplacer {{count}} occurrences dans {{files}} fichiers ?",
        hi: "{{files}} फ़ाइलों में {{count}} जगह बदलें?",
        it: "Sostituire {{count}} occorrenze in {{files}} file?",
        ja: "{{files}} 個のファイルで {{count}} 箇所を置換しますか？",
        nl: "{{count}} voorkomens in {{files}} bestanden vervangen?",
        no: "Erstatte {{count}} forekomster i {{files}} filer?",
        pl: "Zamienić {{count}} wystąpień w {{files}} plikach?",
        pt: "Substituir {{count}} ocorrências em {{files}} ficheiros?",
        sv: "Ersätta {{count}} förekomster i {{files}} filer?",
        zh: "要在 {{files}} 个文件中替换 {{count}} 处吗？"
    },
    'codeStudio.replaceTruncatedHint': {
        cs: "Více než 200 výsledků: zužte hledání, chcete-li použít Nahradit vše. Nahradit v souboru u uvedených souborů funguje dál.",
        da: "Mere end 200 resultater: indsnævr søgningen for at bruge Erstat alle. Erstat i fil virker stadig for de viste filer.",
        de: "Mehr als 200 Treffer: Grenze die Suche ein, um „Alle ersetzen“ zu nutzen. „In Datei ersetzen“ geht weiter für die gelisteten Dateien.",
        el: "Πάνω από 200 αποτελέσματα: περιορίστε την αναζήτηση για να χρησιμοποιήσετε την Αντικατάσταση όλων. Η Αντικατάσταση στο αρχείο λειτουργεί ακόμη για τα αρχεία της λίστας.",
        en: "More than 200 results: narrow the search to use Replace All. Replace in File still works for the listed files.",
        es: "Más de 200 resultados: acota la búsqueda para usar Reemplazar todo. Reemplazar en el archivo sigue funcionando para los archivos de la lista.",
        fr: "Plus de 200 résultats : affinez la recherche pour utiliser Tout remplacer. Remplacer dans le fichier fonctionne toujours pour les fichiers listés.",
        hi: "200 से ज़्यादा परिणाम: सभी बदलें इस्तेमाल करने के लिए खोज सीमित करें। सूची की फ़ाइलों के लिए फ़ाइल में बदलें अब भी काम करता है।",
        it: "Più di 200 risultati: restringi la ricerca per usare Sostituisci tutto. Sostituisci nel file funziona ancora per i file elencati.",
        ja: "結果が 200 件を超えています。すべて置換を使うには検索を絞り込んでください。一覧のファイルではファイル内置換を引き続き使えます。",
        nl: "Meer dan 200 resultaten: beperk de zoekopdracht om Alles vervangen te gebruiken. Vervangen in bestand werkt nog voor de getoonde bestanden.",
        no: "Mer enn 200 treff: snevre inn søket for å bruke Erstatt alle. Erstatt i fil virker fortsatt for de viste filene.",
        pl: "Ponad 200 wyników: zawęź wyszukiwanie, aby użyć Zamień wszystko. Zamień w pliku nadal działa dla wymienionych plików.",
        pt: "Mais de 200 resultados: restrinja a pesquisa para usar Substituir tudo. Substituir no ficheiro continua a funcionar para os ficheiros listados.",
        sv: "Fler än 200 träffar: begränsa sökningen för att använda Ersätt alla. Ersätt i fil fungerar fortfarande för de listade filerna.",
        zh: "结果超过 200 条：请缩小搜索范围以使用“全部替换”。列出的文件仍可使用“在文件中替换”。"
    },
    'codeStudio.replaceReadOnly': {
        cs: "Jen pro čtení: nahrazování je vypnuté.",
        da: "Skrivebeskyttet: erstatning er slået fra.",
        de: "Nur lesen: Ersetzen ist ausgeschaltet.",
        el: "Μόνο για ανάγνωση: η αντικατάσταση είναι απενεργοποιημένη.",
        en: "Read-only: replacing is turned off.",
        es: "Solo lectura: el reemplazo está desactivado.",
        fr: "Lecture seule : le remplacement est désactivé.",
        hi: "केवल पढ़ने के लिए: बदलना बंद है।",
        it: "Sola lettura: la sostituzione è disattivata.",
        ja: "読み取り専用: 置換はオフです。",
        nl: "Alleen lezen: vervangen staat uit.",
        no: "Skrivebeskyttet: erstatning er slått av.",
        pl: "Tylko odczyt: zamiana jest wyłączona.",
        pt: "Só leitura: a substituição está desligada.",
        sv: "Skrivskyddat: ersättning är avstängd.",
        zh: "只读：替换已关闭。"
    },
    'codeStudio.replaceInvalidRegex': {
        cs: "Tento výraz není platný regulární výraz JavaScriptu.",
        da: "Dette udtryk er ikke gyldig JavaScript-regex-syntaks.",
        de: "Dieser Ausdruck ist kein gültiger regulärer Ausdruck in JavaScript.",
        el: "Αυτή η έκφραση δεν είναι έγκυρη σύνταξη regex της JavaScript.",
        en: "This expression is not valid JavaScript regex syntax.",
        es: "Esta expresión no es una sintaxis de regex válida en JavaScript.",
        fr: "Cette expression n'est pas une syntaxe regex JavaScript valide.",
        hi: "यह व्यंजक मान्य JavaScript regex सिंटैक्स नहीं है।",
        it: "Questa espressione non è una sintassi regex JavaScript valida.",
        ja: "この式は JavaScript の正規表現として無効です。",
        nl: "Deze expressie is geen geldige JavaScript-regex-syntaxis.",
        no: "Dette uttrykket er ikke gyldig JavaScript-regex-syntaks.",
        pl: "To wyrażenie nie jest poprawną składnią regex JavaScript.",
        pt: "Esta expressão não é uma sintaxe de regex JavaScript válida.",
        sv: "Det här uttrycket är inte giltig JavaScript-regex-syntax.",
        zh: "此表达式不是有效的 JavaScript 正则语法。"
    },
    'codeStudio.replaceGroupsHint': {
        cs: "Skupiny vložíte pomocí $1, $2.",
        da: "Brug $1, $2 til grupper.",
        de: "Gruppen mit $1, $2 einsetzen.",
        el: "Χρησιμοποιήστε $1, $2 για ομάδες.",
        en: "Use $1, $2 for capture groups.",
        es: "Usa $1, $2 para los grupos.",
        fr: "Utilisez $1, $2 pour les groupes.",
        hi: "समूहों के लिए $1, $2 इस्तेमाल करें।",
        it: "Usa $1, $2 per i gruppi.",
        ja: "グループには $1、$2 を使います。",
        nl: "Gebruik $1, $2 voor groepen.",
        no: "Bruk $1, $2 for grupper.",
        pl: "Użyj $1, $2 dla grup.",
        pt: "Use $1, $2 para os grupos.",
        sv: "Använd $1, $2 för grupper.",
        zh: "使用 $1、$2 引用分组。"
    },
    'codeStudio.replaceNothing': {
        cs: "Není co nahradit.",
        da: "Intet at erstatte.",
        de: "Nichts zu ersetzen.",
        el: "Δεν υπάρχει τίποτα για αντικατάσταση.",
        en: "Nothing to replace.",
        es: "No hay nada que reemplazar.",
        fr: "Rien à remplacer.",
        hi: "बदलने के लिए कुछ नहीं है।",
        it: "Niente da sostituire.",
        ja: "置換するものがありません。",
        nl: "Niets te vervangen.",
        no: "Ingenting å erstatte.",
        pl: "Nie ma nic do zamiany.",
        pt: "Nada para substituir.",
        sv: "Inget att ersätta.",
        zh: "没有可替换的内容。"
    },
    'codeStudio.replaceNoMatchLine': {
        cs: "bez shody",
        da: "intet match",
        de: "kein Treffer",
        el: "καμία αντιστοιχία",
        en: "no match",
        es: "sin coincidencia",
        fr: "aucune correspondance",
        hi: "कोई मेल नहीं",
        it: "nessuna corrispondenza",
        ja: "一致なし",
        nl: "geen overeenkomst",
        no: "ingen treff",
        pl: "brak dopasowania",
        pt: "sem correspondência",
        sv: "ingen träff",
        zh: "无匹配"
    },
    'codeStudio.replaceStale': {
        cs: "Soubor se od hledání změnil. Hledá se znovu.",
        da: "Filen er ændret siden søgningen. Søger igen.",
        de: "Die Datei wurde seit der Suche geändert. Die Suche läuft erneut.",
        el: "Το αρχείο άλλαξε από την αναζήτηση. Νέα αναζήτηση.",
        en: "The file changed since the search. Searching again.",
        es: "El archivo cambió desde la búsqueda. Buscando de nuevo.",
        fr: "Le fichier a changé depuis la recherche. Nouvelle recherche.",
        hi: "खोज के बाद फ़ाइल बदल गई। फिर से खोजा जा रहा है।",
        it: "Il file è cambiato dopo la ricerca. Nuova ricerca in corso.",
        ja: "検索後にファイルが変更されました。再検索します。",
        nl: "Het bestand is sinds de zoekopdracht gewijzigd. Er wordt opnieuw gezocht.",
        no: "Filen er endret siden søket. Søker på nytt.",
        pl: "Plik zmienił się od wyszukiwania. Wyszukiwanie zostanie powtórzone.",
        pt: "O ficheiro mudou desde a pesquisa. A pesquisar de novo.",
        sv: "Filen har ändrats sedan sökningen. Söker igen.",
        zh: "搜索后文件已更改。正在重新搜索。"
    },
    'codeStudio.replaceSkippedStale': {
        cs: "{{path}}: od hledání změněn, přeskočeno",
        da: "{{path}}: ændret siden søgningen, sprunget over",
        de: "{{path}}: seit der Suche geändert, übersprungen",
        el: "{{path}}: άλλαξε από την αναζήτηση, παραλείφθηκε",
        en: "{{path}}: changed since the search, skipped",
        es: "{{path}}: cambió desde la búsqueda, omitido",
        fr: "{{path}} : modifié depuis la recherche, ignoré",
        hi: "{{path}}: खोज के बाद बदली गई, छोड़ी गई",
        it: "{{path}}: modificato dopo la ricerca, saltato",
        ja: "{{path}}: 検索後に変更されたため、スキップしました",
        nl: "{{path}}: gewijzigd sinds de zoekopdracht, overgeslagen",
        no: "{{path}}: endret siden søket, hoppet over",
        pl: "{{path}}: zmieniony od wyszukiwania, pominięto",
        pt: "{{path}}: alterado desde a pesquisa, ignorado",
        sv: "{{path}}: ändrad sedan sökningen, hoppades över",
        zh: "{{path}}：搜索后已更改，已跳过"
    },
    'codeStudio.replaceSkippedNoMatch': {
        cs: "{{path}}: výraz JavaScriptu nenašel shodu, přeskočeno",
        da: "{{path}}: JavaScript-udtrykket finder intet match, sprunget over",
        de: "{{path}}: der JavaScript-Ausdruck findet keinen Treffer, übersprungen",
        el: "{{path}}: η έκφραση JavaScript δεν βρίσκει αντιστοιχία, παραλείφθηκε",
        en: "{{path}}: the JavaScript expression finds no match, skipped",
        es: "{{path}}: la expresión de JavaScript no encuentra coincidencias, omitido",
        fr: "{{path}} : l'expression JavaScript ne trouve aucune correspondance, ignoré",
        hi: "{{path}}: JavaScript व्यंजक को कोई मेल नहीं मिला, छोड़ी गई",
        it: "{{path}}: l'espressione JavaScript non trova corrispondenze, saltato",
        ja: "{{path}}: JavaScript の式で一致が見つからないため、スキップしました",
        nl: "{{path}}: de JavaScript-expressie vindt geen overeenkomst, overgeslagen",
        no: "{{path}}: JavaScript-uttrykket finner ingen treff, hoppet over",
        pl: "{{path}}: wyrażenie JavaScript nie znajduje dopasowania, pominięto",
        pt: "{{path}}: a expressão JavaScript não encontra correspondências, ignorado",
        sv: "{{path}}: JavaScript-uttrycket hittar ingen träff, hoppades över",
        zh: "{{path}}：JavaScript 表达式找不到匹配，已跳过"
    },
    'codeStudio.replaceSkippedTooLarge': {
        cs: "{{path}}: větší než limit velikosti souboru, přeskočeno",
        da: "{{path}}: større end grænsen for filstørrelse, sprunget over",
        de: "{{path}}: größer als die Dateigrößengrenze, übersprungen",
        el: "{{path}}: μεγαλύτερο από το όριο μεγέθους αρχείου, παραλείφθηκε",
        en: "{{path}}: larger than the file size limit, skipped",
        es: "{{path}}: supera el límite de tamaño de archivo, omitido",
        fr: "{{path}} : dépasse la taille de fichier maximale, ignoré",
        hi: "{{path}}: फ़ाइल आकार सीमा से बड़ी, छोड़ी गई",
        it: "{{path}}: supera il limite di dimensione dei file, saltato",
        ja: "{{path}}: ファイルサイズの上限を超えているため、スキップしました",
        nl: "{{path}}: groter dan de limiet voor bestandsgrootte, overgeslagen",
        no: "{{path}}: større enn grensen for filstørrelse, hoppet over",
        pl: "{{path}}: większy niż limit rozmiaru pliku, pominięto",
        pt: "{{path}}: maior do que o limite de tamanho de ficheiro, ignorado",
        sv: "{{path}}: större än gränsen för filstorlek, hoppades över",
        zh: "{{path}}：超过文件大小上限，已跳过"
    },
    'codeStudio.replaceSkippedEncoding': {
        cs: "{{path}}: není čistý text UTF-8, přeskočeno",
        da: "{{path}}: ikke ren UTF-8-tekst, sprunget over",
        de: "{{path}}: kein reiner UTF-8-Text, übersprungen",
        el: "{{path}}: δεν είναι απλό κείμενο UTF-8, παραλείφθηκε",
        en: "{{path}}: not plain UTF-8 text, skipped",
        es: "{{path}}: no es texto UTF-8 limpio, omitido",
        fr: "{{path}} : pas du texte UTF-8 pur, ignoré",
        hi: "{{path}}: शुद्ध UTF-8 टेक्स्ट नहीं, छोड़ी गई",
        it: "{{path}}: non è testo UTF-8 puro, saltato",
        ja: "{{path}}: 純粋な UTF-8 テキストではないため、スキップしました",
        nl: "{{path}}: geen zuivere UTF-8-tekst, overgeslagen",
        no: "{{path}}: ikke ren UTF-8-tekst, hoppet over",
        pl: "{{path}}: to nie jest czysty tekst UTF-8, pominięto",
        pt: "{{path}}: não é texto UTF-8 simples, ignorado",
        sv: "{{path}}: inte ren UTF-8-text, hoppades över",
        zh: "{{path}}：不是纯 UTF-8 文本，已跳过"
    },
    'codeStudio.replaceSummary': {
        cs: "Nahrazeno {{count}} výskytů v {{files}} souborech.",
        da: "Erstattede {{count}} forekomster i {{files}} filer.",
        de: "{{count}} Vorkommen in {{files}} Dateien ersetzt.",
        el: "Αντικαταστάθηκαν {{count}} εμφανίσεις σε {{files}} αρχεία.",
        en: "Replaced {{count}} occurrences in {{files}} files.",
        es: "Se reemplazaron {{count}} coincidencias en {{files}} archivos.",
        fr: "{{count}} occurrences remplacées dans {{files}} fichiers.",
        hi: "{{files}} फ़ाइलों में {{count}} जगह बदलाव किए गए।",
        it: "Sostituite {{count}} occorrenze in {{files}} file.",
        ja: "{{files}} 個のファイルで {{count}} 箇所を置換しました。",
        nl: "{{count}} voorkomens vervangen in {{files}} bestanden.",
        no: "Erstattet {{count}} forekomster i {{files}} filer.",
        pl: "Zamieniono {{count}} wystąpień w {{files}} plikach.",
        pt: "Substituídas {{count}} ocorrências em {{files}} ficheiros.",
        sv: "Ersatte {{count}} förekomster i {{files}} filer.",
        zh: "已在 {{files}} 个文件中替换 {{count}} 处。"
    },
    'codeStudio.replaceUnsavedNote': {
        cs: "{{count}} otevřených souborů bylo změněno v editoru a zatím není uloženo.",
        da: "{{count}} åbne filer blev ændret i editoren og er ikke gemt endnu.",
        de: "{{count}} offene Dateien wurden im Editor geändert und sind noch nicht gespeichert.",
        el: "{{count}} ανοιχτά αρχεία άλλαξαν στον επεξεργαστή και δεν έχουν αποθηκευτεί ακόμη.",
        en: "{{count}} open files were changed in the editor and are not saved yet.",
        es: "{{count}} archivos abiertos se cambiaron en el editor y aún no se han guardado.",
        fr: "{{count}} fichiers ouverts ont été modifiés dans l'éditeur et ne sont pas encore enregistrés.",
        hi: "{{count}} खुली फ़ाइलें एडिटर में बदली गईं और अभी सहेजी नहीं गई हैं।",
        it: "{{count}} file aperti sono stati modificati nell'editor e non sono ancora salvati.",
        ja: "{{count}} 個の開いているファイルがエディターで変更され、まだ保存されていません。",
        nl: "{{count}} geopende bestanden zijn in de editor gewijzigd en nog niet opgeslagen.",
        no: "{{count}} åpne filer ble endret i editoren og er ikke lagret ennå.",
        pl: "{{count}} otwartych plików zmieniono w edytorze i nie są jeszcze zapisane.",
        pt: "{{count}} ficheiros abertos foram alterados no editor e ainda não foram guardados.",
        sv: "{{count}} öppna filer ändrades i redigeraren och är inte sparade än.",
        zh: "{{count}} 个打开的文件已在编辑器中更改，尚未保存。"
    },
    'codeStudio.replaceSomeSkipped': {
        cs: "{{count}} souborů přeskočeno, viz Výstup.",
        da: "{{count}} filer sprunget over, se Output.",
        de: "{{count}} Dateien übersprungen, siehe Ausgabe.",
        el: "{{count}} αρχεία παραλείφθηκαν, δείτε την Έξοδο.",
        en: "{{count}} files skipped, see Output.",
        es: "{{count}} archivos omitidos, consulta Salida.",
        fr: "{{count}} fichiers ignorés, voir Sortie.",
        hi: "{{count}} फ़ाइलें छोड़ी गईं, आउटपुट देखें।",
        it: "{{count}} file saltati, vedi Output.",
        ja: "{{count}} 個のファイルをスキップしました。出力を確認してください。",
        nl: "{{count}} bestanden overgeslagen, zie Uitvoer.",
        no: "{{count}} filer hoppet over, se Utdata.",
        pl: "Pominięto {{count}} plików, zobacz Wyjście.",
        pt: "{{count}} ficheiros ignorados, veja Saída.",
        sv: "{{count}} filer hoppades över, se Utdata.",
        zh: "已跳过 {{count}} 个文件，请查看输出。"
    },
    'codeStudio.replaceRunning': {
        cs: "Nahrazování...",
        da: "Erstatter...",
        de: "Wird ersetzt...",
        el: "Αντικατάσταση...",
        en: "Replacing...",
        es: "Reemplazando...",
        fr: "Remplacement...",
        hi: "बदला जा रहा है...",
        it: "Sostituzione in corso...",
        ja: "置換中...",
        nl: "Bezig met vervangen...",
        no: "Erstatter...",
        pl: "Zamienianie...",
        pt: "A substituir...",
        sv: "Ersätter...",
        zh: "正在替换..."
    }
};

const placeholders = value => (value.match(/\{\{\w+\}\}/g) || []).sort().join(',');

for (const lang of langs) {
    const file = path.join(repo, 'ui', 'lang', 'desktop', lang + '.json');
    const text = await readFile(file, 'utf8');
    const eol = text.includes('\r\n') ? '\r\n' : '\n';
    const existing = JSON.parse(text);
    const lines = text.split(eol);
    const index = lines.findIndex(line => line.trimStart().startsWith(anchorKey));
    if (index < 0) throw new Error(`${lang}: anchor ${anchorKey} not found`);
    if (!lines[index].trimEnd().endsWith(',')) throw new Error(`${lang}: the anchor is the last entry`);
    const indent = lines[index].match(/^\s*/)[0];
    const added = [];
    for (const [key, values] of Object.entries(strings)) {
        const value = values[lang];
        if (typeof value !== 'string' || !value.trim()) throw new Error(`${lang}: ${key} has no translation`);
        if (placeholders(value) !== placeholders(values.en)) throw new Error(`${lang}: ${key} placeholders differ from English`);
        if (key === 'codeStudio.replaceGroupsHint' && !value.includes('$1')) throw new Error(`${lang}: ${key} lost $1`);
        if (Object.prototype.hasOwnProperty.call(existing, key)) {
            if (existing[key] !== value) throw new Error(`${lang}: ${key} already exists with another text`);
            continue;
        }
        added.push(`${indent}${JSON.stringify(key)}: ${JSON.stringify(value)},`);
    }
    if (added.length) {
        lines.splice(index + 1, 0, ...added);
        const next = lines.join(eol);
        const parsed = JSON.parse(next);
        for (const [key, values] of Object.entries(strings)) {
            if (parsed[key] !== values[lang]) throw new Error(`${lang}: ${key} did not land`);
        }
        await writeFile(file, next, 'utf8');
    }
    console.log(`${lang}: added ${added.length} keys`);
}
```

- [ ] **Step 2: Run the script**

Run: `node C:\disposable\code-studio-pkg1\locales-c.mjs C:\Users\Andi\Documents\repo\AuraGo`
Expected: 16 lines `<lang>: added 16 keys`, exit code 0. A second run prints `added 0 keys` for every file.

- [ ] **Step 3: Check that the locale files still parse for the Go tests**

Run: `go test ./ui -run '(?i)^(TestCodeStudioStatusAndSearchCap)$' -count=1 -v`
Expected: `--- PASS: TestCodeStudioStatusAndSearchCap`.

- [ ] **Step 4: Graph check** (`detect-changes --scope all`).

**Commit:** `feat(code-studio): strings for project-wide replace`. Files:
- `ui/lang/desktop/cs.json`, `ui/lang/desktop/da.json`, `ui/lang/desktop/de.json`, `ui/lang/desktop/el.json`
- `ui/lang/desktop/en.json`, `ui/lang/desktop/es.json`, `ui/lang/desktop/fr.json`, `ui/lang/desktop/hi.json`
- `ui/lang/desktop/it.json`, `ui/lang/desktop/ja.json`, `ui/lang/desktop/nl.json`, `ui/lang/desktop/no.json`
- `ui/lang/desktop/pl.json`, `ui/lang/desktop/pt.json`, `ui/lang/desktop/sv.json`, `ui/lang/desktop/zh.json`

---

### Task 3: Replace UI in the search panel

**Files:**
- Modify: `ui/js/desktop/apps/code-studio/replace.js` (append)
- Modify: `ui/js/desktop/apps/code-studio/search.js` (overwrite)
- Modify: `ui/js/desktop/apps/code-studio/core.js` (shared region: `updatePolicyControls`)
- Modify: `scripts/build-ui-bundles.js`
- Modify: `ui/css/code-studio.css` (append)
- Generated: `ui/js/desktop/bundles/code-studio.bundle.js`

- [ ] **Step 1: Impact analysis before editing.** Run `impact` (upstream) for `renderSearchPanel`, `runSearch` and `updatePolicyControls`. Report callers, processes and risk; warn on HIGH or CRITICAL. If the result is UNKNOWN, confirm with Grep. Expected callers:
  - `renderSearchPanel`: `core.js` `renderShell`, `search.js` `toggleSearch`; after this task also `replace.js` `toggleProjectReplace` and `search.js` `renderSearchResults`.
  - `runSearch`: the search form submit in `search.js`; after this task also `replace.js` `rerunProjectSearch`.
  - `updatePolicyControls`: `core.js` `applyStudioPolicy` and `renderTabs`, `agent.js`, `git.js`, `sidebar.js`, `terminal.js`. The edit only appends one call that does nothing unless the replace row is open.

- [ ] **Step 2: Append the studio side to `ui/js/desktop/apps/code-studio/replace.js`**

Append to the end of the file, after the line `    // --- End of the pure project replace helpers. ---`:

```js

    // --- Project replace: studio side (state, DOM, editors and the file API). ---

    const PROJECT_REPLACE_INVALID_TEXT = String.fromCharCode(0xfffd);

    // Diff and commit tabs carry the path of a file too; they are never replace targets.
    function projectReplaceIsVirtual(tab) {
        if (typeof isVirtualTab === 'function') return isVirtualTab(tab);
        return !!tab && (tab.kind === 'diff' || tab.kind === 'commit');
    }

    function projectReplaceTab(path) {
        const wanted = normalizeCodeStudioPath(path);
        return state.openTabs.find(tab => tab && tab.path === wanted && !projectReplaceIsVirtual(tab)) || null;
    }

    function projectReplaceView(tab) {
        const views = [tab.view, tab.secondaryView, ...(Array.isArray(tab.views) ? tab.views : [])].filter(Boolean);
        return views.find(view => view.state && view.state.doc) || views.find(view => typeof view.setValue === 'function') || null;
    }

    // A tab without a live view keeps its CodeMirror state (with the undo history) in the snapshot
    // that renderEditor restores when the tab is shown again.
    function projectReplaceSnapshot(tab) {
        if (projectReplaceView(tab)) return null;
        const snapshot = tab.editorSnapshot;
        return snapshot && snapshot.state && snapshot.state.doc ? snapshot : null;
    }

    function projectReplaceTabText(tab) {
        const snapshot = projectReplaceSnapshot(tab);
        return snapshot ? snapshot.state.doc.toString() : editorValue(tab);
    }

    // Results for open files come from their buffers: the server only sees the files on disk, and a
    // replace needs the line numbers of the buffer it changes. Files the server did not list stay out.
    function overlayOpenTabResults(results) {
        const list = Array.isArray(results) ? results : [];
        const regex = buildReplaceRegExp(state.searchQuery, state.searchOptions);
        if (!regex) return list;
        const open = new Map();
        state.openTabs.forEach(tab => {
            if (tab && !projectReplaceIsVirtual(tab) && !open.has(tab.path)) open.set(tab.path, tab);
        });
        if (!open.size) return list;
        const out = [];
        const done = new Set();
        list.forEach(result => {
            const path = normalizeCodeStudioPath(result.path);
            const tab = open.get(path);
            if (!tab) {
                out.push(result);
            } else if (!done.has(path)) {
                done.add(path);
                out.push(...bufferSearchResults(result.path, projectReplaceTabText(tab), regex, 200));
            }
        });
        return out;
    }

    function projectReplaceJob() {
        const options = state.searchOptions || {};
        const regex = buildReplaceRegExp(state.searchQuery, options);
        return regex ? { regex, replacement: String(state.searchReplaceText || ''), regexMode: !!options.regex } : null;
    }

    // The result list grouped by file. Every row keeps its segments, so the preview and the occurrence
    // numbers come from the same computation as the write.
    function projectReplaceGroups(job) {
        const groups = [];
        const byPath = new Map();
        (state.searchResults || []).forEach(result => {
            const path = normalizeCodeStudioPath(result.path);
            let group = byPath.get(path);
            if (!group) {
                group = { path, label: searchResultLabel(path), rows: [], count: 0 };
                byPath.set(path, group);
                groups.push(group);
            }
            const line = Number(result.line) || 0;
            if (group.rows.some(row => row.line === line)) return;
            const preview = String(result.preview == null ? '' : result.preview).replace(/\r$/, '');
            const segments = replaceLineSegments(preview, job);
            const matches = segments.filter(segment => segment.match).length;
            group.rows.push({ line, preview, segments, matches });
            group.count += matches;
        });
        return groups;
    }

    function projectReplaceTotals(groups) {
        return groups.reduce((totals, group) => ({
            count: totals.count + group.count,
            files: totals.files + (group.count ? 1 : 0)
        }), { count: 0, files: 0 });
    }

    function projectReplaceRowMarkup() {
        const label = tr('codeStudio.replacePlaceholder', 'Replace');
        return `<div class="cs-search-replace-row" data-search-replace-row>
            <input name="replace" data-search-replace-input value="${esc(state.searchReplaceText || '')}" placeholder="${esc(label)}" aria-label="${esc(label)}" autocomplete="off" spellcheck="false">
            <button type="button" class="cs-button" data-search-replace-all>${buttonIcon('edit', 'R')}<span>${esc(tr('codeStudio.replaceAll', 'Replace All'))}</span></button>
            <span class="cs-search-replace-hint" data-search-replace-hint aria-live="polite"></span>
        </div>`;
    }

    function projectReplaceResultsMarkup() {
        const job = projectReplaceJob();
        const replaceLabel = tr('codeStudio.replaceOne', 'Replace');
        const fileLabel = tr('codeStudio.replaceInFile', 'Replace in File');
        return projectReplaceGroups(job).map(group => {
            const head = `<div class="cs-replace-file" data-replace-file="${esc(group.path)}">
                <span class="cs-replace-file-name" title="${esc(group.path)}"><strong>${esc(group.label)}</strong><em class="cs-replace-count">${esc(group.count)}</em></span>
                <button type="button" class="cs-replace-action" data-replace-file-action data-replace-path="${esc(group.path)}" data-replace-count="${esc(group.count)}" title="${esc(fileLabel)}">${esc(fileLabel)}</button>
            </div>`;
            const rows = group.rows.map(row => {
                const open = (code, tail) => `<button type="button" class="cs-replace-preview" data-search-path="${esc(group.path)}" data-search-line="${esc(row.line)}" title="${esc(group.label)}:${esc(row.line)}"><em class="cs-search-line">${esc(row.line)}</em><code>${code}</code>${tail || ''}</button>`;
                if (!row.matches) {
                    const tail = job ? `<span class="cs-replace-nomatch">${esc(tr('codeStudio.replaceNoMatchLine', 'no match'))}</span>` : '';
                    return `<div class="cs-replace-match is-nomatch">${open(esc(row.preview.trim()), tail)}</div>`;
                }
                let html = '';
                for (let occurrence = 0; occurrence < row.matches; occurrence += 1) {
                    const parts = replaceMatchPreview(row.segments, occurrence);
                    const code = `${esc(parts.before)}<del class="cs-replace-old">${esc(parts.old)}</del><ins class="cs-replace-new">${esc(parts.next)}</ins>${esc(parts.after)}`;
                    html += `<div class="cs-replace-match">${open(code)}<button type="button" class="cs-replace-action" data-replace-one data-replace-path="${esc(group.path)}" data-replace-line="${esc(row.line)}" data-replace-occurrence="${occurrence}" title="${esc(replaceLabel)}">${esc(replaceLabel)}</button></div>`;
                }
                return html;
            }).join('');
            return head + rows;
        }).join('');
    }

    function wireProjectReplaceControls(panel) {
        const toggle = panel.querySelector('[data-search-replace-toggle]');
        if (toggle) toggle.addEventListener('click', bind(() => toggleProjectReplace()));
        const input = panel.querySelector('[data-search-replace-input]');
        if (input) {
            input.addEventListener('input', bind(event => {
                state.searchReplaceText = event.target.value;
                renderSearchResults();
            }));
        }
        const all = panel.querySelector('[data-search-replace-all]');
        if (all) all.addEventListener('click', bind(() => projectReplaceAll()));
    }

    function wireProjectReplaceResults(box) {
        box.querySelectorAll('[data-replace-file-action]').forEach(button => {
            button.addEventListener('click', bind(() => projectReplaceFile(button.dataset.replacePath)));
        });
        box.querySelectorAll('[data-replace-one]').forEach(button => {
            button.addEventListener('click', bind(() => projectReplaceMatch(
                button.dataset.replacePath,
                Number(button.dataset.replaceLine),
                Number(button.dataset.replaceOccurrence)
            )));
        });
    }

    function toggleProjectReplace() {
        state.searchReplaceOpen = !state.searchReplaceOpen;
        renderSearchPanel();
        const field = shellPart(state.searchReplaceOpen ? '[data-search-replace-input]' : '[data-search] input[name="q"]');
        if (field) field.focus();
    }

    function projectReplaceHint(job, writable) {
        if (!writable) return tr('codeStudio.replaceReadOnly', 'Read-only: replacing is turned off.');
        if (state.searchQuery && !job) return tr('codeStudio.replaceInvalidRegex', 'This expression is not valid JavaScript regex syntax.');
        if (state.searchTruncated) return tr('codeStudio.replaceTruncatedHint', 'More than 200 results: narrow the search to use Replace All. Replace in File still works for the listed files.');
        if (job && job.regexMode) return tr('codeStudio.replaceGroupsHint', 'Use $1, $2 for capture groups.');
        return '';
    }

    // Runs after every result render and from updatePolicyControls, so read-only switches land at once.
    function updateProjectReplaceControls() {
        const root = studioRoot();
        const panel = root && root.querySelector('[data-search]');
        if (!panel || panel.hidden || !state.searchReplaceOpen) return;
        const writable = canWrite();
        const busy = !!state.projectReplaceBusy;
        const job = projectReplaceJob();
        const totals = job && state.searchPerformed ? projectReplaceTotals(projectReplaceGroups(job)) : { count: 0, files: 0 };
        const input = panel.querySelector('[data-search-replace-input]');
        if (input) input.disabled = !writable;
        const all = panel.querySelector('[data-search-replace-all]');
        if (all) {
            all.disabled = !writable || busy || !job || !!state.searchTruncated || totals.count === 0;
            all.title = state.searchTruncated
                ? tr('codeStudio.replaceTruncatedHint', 'More than 200 results: narrow the search to use Replace All. Replace in File still works for the listed files.')
                : tr('codeStudio.replaceAll', 'Replace All');
        }
        panel.querySelectorAll('[data-replace-file-action], [data-replace-one]').forEach(button => {
            button.disabled = !writable || busy || !job || button.dataset.replaceCount === '0';
        });
        const hint = panel.querySelector('[data-search-replace-hint]');
        if (hint) hint.textContent = projectReplaceHint(job, writable);
    }

    function projectReplaceJobFile(group) {
        return {
            path: group.path,
            label: group.label,
            targets: group.rows.filter(row => row.matches > 0).map(row => ({ line: row.line, preview: row.preview, occurrence: null }))
        };
    }

    function projectReplaceReady() {
        if (!canWrite() || state.projectReplaceBusy) return null;
        const job = projectReplaceJob();
        if (!job) flashStatus(tr('codeStudio.replaceInvalidRegex', 'This expression is not valid JavaScript regex syntax.'));
        return job;
    }

    async function projectReplaceAll() {
        const target = state;
        const job = projectReplaceReady();
        if (!job || target.searchTruncated) return;
        const groups = projectReplaceGroups(job);
        const totals = projectReplaceTotals(groups);
        if (!totals.count) {
            flashStatus(tr('codeStudio.replaceNothing', 'Nothing to replace.'));
            return;
        }
        const confirmed = await confirmValue(
            tr('codeStudio.replaceAllConfirm', 'Replace {{count}} occurrences in {{files}} files?', { count: totals.count, files: totals.files }),
            { confirmLabel: tr('codeStudio.replaceAll', 'Replace All'), confirmIcon: 'edit' }
        );
        if (!confirmed || !isLiveInstance(target)) return;
        await runProjectReplace(target, job, groups.filter(group => group.count > 0).map(projectReplaceJobFile), groups.filter(group => !group.count), 'all');
    }

    async function projectReplaceFile(path) {
        const target = state;
        const job = projectReplaceReady();
        if (!job) return;
        const group = projectReplaceGroups(job).find(item => item.path === normalizeCodeStudioPath(path));
        if (!group) return;
        if (!group.count) {
            flashStatus(tr('codeStudio.replaceNothing', 'Nothing to replace.'));
            return;
        }
        await runProjectReplace(target, job, [projectReplaceJobFile(group)], [], 'file');
    }

    // One match: the exact line and the occurrence index on that line, re-checked before anything changes.
    async function projectReplaceMatch(path, line, occurrence) {
        const target = state;
        const job = projectReplaceReady();
        if (!job) return;
        const group = projectReplaceGroups(job).find(item => item.path === normalizeCodeStudioPath(path));
        const row = group && group.rows.find(item => item.line === line);
        if (!row || !Number.isInteger(occurrence) || occurrence < 0 || occurrence >= row.matches) return;
        await runProjectReplace(target, job, [{ path: group.path, label: group.label, targets: [{ line, preview: row.preview, occurrence }] }], [], 'one');
    }

    // Files one after the other; a failing file is reported and the loop goes on.
    async function runProjectReplace(target, job, files, noMatchGroups, mode) {
        if (!canWrite(target) || target.projectReplaceBusy) return null;
        target.projectReplaceBusy = true;
        const report = { mode, count: 0, files: 0, buffers: 0, stale: 0, skipped: [] };
        (noMatchGroups || []).forEach(group => report.skipped.push(projectReplacePlanSkip({ status: 'nomatch' }, group).message));
        runWithInstance(target, () => {
            updateProjectReplaceControls();
            renderStatus(tr('codeStudio.replaceRunning', 'Replacing...'));
        });
        try {
            for (const file of files) {
                if (!isLiveInstance(target)) return null;
                let outcome;
                try {
                    outcome = await projectReplaceInFile(target, file, job);
                } catch (err) {
                    outcome = projectReplaceFailure(file, err);
                }
                if (outcome.status === 'written' || outcome.status === 'buffer') {
                    report.count += outcome.count;
                    report.files += 1;
                    if (outcome.status === 'buffer') report.buffers += 1;
                } else {
                    if (outcome.status === 'stale') report.stale += 1;
                    report.skipped.push(outcome.message);
                }
            }
        } finally {
            target.projectReplaceBusy = false;
        }
        if (!isLiveInstance(target)) return null;
        runWithInstance(target, () => finishProjectReplace(report));
        await runAsyncStep(target, rerunProjectSearch);
        return report;
    }

    async function projectReplaceInFile(target, file, job) {
        const open = runWithInstance(target, () => projectReplaceTab(file.path));
        if (open) return runWithInstance(target, () => projectReplaceInTab(open, file, job));
        let content;
        try {
            const result = await apiClient.file(file.path);
            content = result && typeof result.content === 'string' ? result.content : '';
        } catch (err) {
            return projectReplaceFailure(file, err);
        }
        if (!isLiveInstance(target)) return projectReplaceFailure(file, null);
        // A tab opened while the file was read takes the change in its buffer instead.
        const opened = runWithInstance(target, () => projectReplaceTab(file.path));
        if (opened) return runWithInstance(target, () => projectReplaceInTab(opened, file, job));
        if (content.includes(PROJECT_REPLACE_INVALID_TEXT)) {
            return projectReplaceSkip('error', 'codeStudio.replaceSkippedEncoding', '{{path}}: not plain UTF-8 text, skipped', file.label);
        }
        const plan = planFileReplace(content, file.targets, job);
        if (plan.status !== 'ok') return projectReplacePlanSkip(plan, file);
        if (!canWrite(target)) return { status: 'error', message: file.label + ': ' + tr('codeStudio.replaceReadOnly', 'Read-only: replacing is turned off.') };
        try {
            await apiClient.writeFile(file.path, plan.content);
        } catch (err) {
            return projectReplaceFailure(file, err);
        }
        const late = isLiveInstance(target) ? runWithInstance(target, () => projectReplaceTab(file.path)) : null;
        if (late) await reloadCleanTabFromDisk(target, late);
        return { status: 'written', count: plan.count };
    }

    function projectReplaceInTab(tab, file, job) {
        if (!canWrite()) return { status: 'error', message: file.label + ': ' + tr('codeStudio.replaceReadOnly', 'Read-only: replacing is turned off.') };
        if (tab.pathMutation) return projectReplacePlanSkip({ status: 'stale' }, file);
        const snapshot = projectReplaceSnapshot(tab);
        const plan = planFileReplace(projectReplaceTabText(tab), file.targets, job);
        if (plan.status !== 'ok') return projectReplacePlanSkip(plan, file);
        applyProjectReplaceToTab(tab, plan, snapshot);
        return { status: 'buffer', count: plan.count };
    }

    // One transaction is one undo step; the editor's update listener marks the tab modified.
    function applyProjectReplaceToTab(tab, plan, snapshot) {
        const view = projectReplaceView(tab);
        if (view && view.state && view.state.doc) {
            view.dispatch({ changes: plan.changes, userEvent: 'input.replace' });
            return;
        }
        if (view && typeof view.setValue === 'function') {
            view.setValue(plan.content);
            return;
        }
        if (snapshot) snapshot.state = snapshot.state.update({ changes: plan.changes, userEvent: 'input.replace' }).state;
        updateTabContent(tab, plan.content);
    }

    function projectReplaceSkip(status, key, fallback, label) {
        return { status, message: tr(key, fallback, { path: label }) };
    }

    function projectReplacePlanSkip(plan, file) {
        if (plan.status === 'stale') {
            return projectReplaceSkip('stale', 'codeStudio.replaceSkippedStale', '{{path}}: changed since the search, skipped', file.label);
        }
        return projectReplaceSkip('nomatch', 'codeStudio.replaceSkippedNoMatch', '{{path}}: the JavaScript expression finds no match, skipped', file.label);
    }

    // The server answers 413 for files above its size cap, on GET and on PUT.
    function projectReplaceFailure(file, err) {
        if (err && err.status === 413) {
            return projectReplaceSkip('error', 'codeStudio.replaceSkippedTooLarge', '{{path}}: larger than the file size limit, skipped', file.label);
        }
        return { status: 'error', message: file.label + ': ' + (err && err.message ? err.message : String(err || '')) };
    }

    function finishProjectReplace(report) {
        state.projectReplaceReport = report;
        const summary = tr('codeStudio.replaceSummary', 'Replaced {{count}} occurrences in {{files}} files.', { count: report.count, files: report.files });
        const lines = [summary];
        if (report.buffers) lines.push(tr('codeStudio.replaceUnsavedNote', '{{count}} open files were changed in the editor and are not saved yet.', { count: report.buffers }));
        appendOutput(lines.concat(report.skipped).join('\n'), tr('codeStudio.replaceOne', 'Replace'));
        if (report.files > report.buffers) scheduleGitRefresh();
        renderTabs();
        renderWindowMenus();
        renderStatus();
        updateProjectReplaceControls();
        if (report.mode === 'one' && report.stale) {
            flashStatus(tr('codeStudio.replaceStale', 'The file changed since the search. Searching again.'));
        } else if (report.skipped.length) {
            setActivePanel('output');
            flashStatus(tr('codeStudio.replaceSomeSkipped', '{{count}} files skipped, see Output.', { count: report.skipped.length }));
        } else {
            flashStatus(summary);
        }
    }

    // Runs the last search again with its own options, not with whatever the form shows now.
    function rerunProjectSearch() {
        if (!state.searchQuery) return undefined;
        const options = state.searchOptions || {};
        const data = new FormData();
        data.set('q', state.searchQuery);
        if (options.case) data.set('case', 'on');
        if (options.whole) data.set('whole', 'on');
        if (options.regex) data.set('regex', 'on');
        data.set('include', options.include || '');
        data.set('exclude', options.exclude || '');
        return runSearch(data);
    }
```

- [ ] **Step 3: Overwrite `ui/js/desktop/apps/code-studio/search.js`**

Plans A and B do not touch this file. First check that it is still the 145-line version this plan was written against: it starts with `    function searchResultLabel(path) {` and its `renderSearchPanel` contains ``panel.innerHTML = `<form class="cs-search-form" data-search-form>``. If someone changed it, port their change into the content below.

Replace the whole file content with:

```js
    function searchResultLabel(path) {
        const value = String(path || '');
        return value.startsWith(WORKSPACE_ROOT + '/') ? value.slice(WORKSPACE_ROOT.length + 1) : value;
    }

    // The search panel: a head with the VS Code style replace chevron, the search form and the replace
    // row (replace.js), then the result list. renderSearchResults only redraws the list, so typing in
    // the replace field keeps its caret.
    function renderSearchPanel() {
        const panel = shellPart('[data-search]');
        if (!panel) return;
        panel.hidden = !state.searchVisible;
        if (!state.searchVisible) return;
        const options = state.searchOptions || {};
        const replaceOpen = !!state.searchReplaceOpen;
        const toggleLabel = tr('codeStudio.findToggleReplace', 'Toggle replace');
        panel.innerHTML = `<div class="cs-search-head" data-replace-open="${replaceOpen ? 'true' : 'false'}">
            <button type="button" class="cs-search-replace-toggle" data-search-replace-toggle aria-expanded="${replaceOpen ? 'true' : 'false'}" title="${esc(toggleLabel)}" aria-label="${esc(toggleLabel)}"><span aria-hidden="true">›</span></button>
            <form class="cs-search-form" data-search-form>
                <input name="q" value="${esc(state.searchQuery || '')}" placeholder="${esc(tr('codeStudio.searchFiles', 'Search in Files'))}" autocomplete="off" spellcheck="false" inputmode="search" enterkeyhint="search">
                <input name="include" value="${esc(options.include || '')}" placeholder="*.go" autocomplete="off" spellcheck="false">
                <input name="exclude" value="${esc(options.exclude || '')}" placeholder="vendor/" autocomplete="off" spellcheck="false">
                <label><input type="checkbox" name="case"${options.case ? ' checked' : ''}> Aa</label>
                <label><input type="checkbox" name="whole"${options.whole ? ' checked' : ''}> Ab</label>
                <label><input type="checkbox" name="regex"${options.regex ? ' checked' : ''}> .*</label>
                <button type="submit" class="cs-button primary">${buttonIcon('search', 'S')}<span>${esc(tr('codeStudio.search', 'Search'))}</span></button>
                <button type="button" class="cs-icon-button" data-search-close title="${esc(tr('desktop.close', 'Close'))}">${iconMarkup('x', 'X', 'cs-icon-button-icon', 14)}</button>
            </form>${replaceOpen ? projectReplaceRowMarkup() : ''}
        </div><div class="cs-search-results" data-search-results></div>`;
        panel.querySelector('[data-search-form]').addEventListener('submit', bind(event => {
            event.preventDefault();
            runSearch(new FormData(event.currentTarget));
        }));
        panel.querySelector('[data-search-close]').addEventListener('click', bind(() => toggleSearch()));
        wireProjectReplaceControls(panel);
        renderSearchResults();
        const input = panel.querySelector('input[name="q"]');
        if (input) {
            input.addEventListener('keydown', bind(event => {
                if (event.key === 'Escape') {
                    event.preventDefault();
                    toggleSearch();
                }
            }));
            if (!state.searchPerformed || !input.value) {
                input.focus();
                input.select();
            }
        }
    }

    function renderSearchResults() {
        const panel = shellPart('[data-search]');
        const box = panel && panel.querySelector('[data-search-results]');
        if (!box) {
            if (panel && state.searchVisible) renderSearchPanel();
            return;
        }
        let results;
        if (!state.searchPerformed) {
            results = `<div class="cs-empty">${esc(tr('codeStudio.searchPrompt', 'Enter a search term and press Enter'))}</div>`;
        } else if (!state.searchResults.length) {
            results = `<div class="cs-empty">${esc(tr('codeStudio.noResults', 'No results found'))}</div>`;
        } else {
            results = `<div class="cs-search-summary">${esc(tr('codeStudio.resultsCount', '{{count}} results', { count: state.searchResults.length }))}</div>` +
                (state.searchTruncated ? `<div class="cs-search-summary">${esc(tr('codeStudio.resultsTruncated', 'Showing the first 200 results.'))}</div>` : '') +
                (state.searchReplaceOpen ? projectReplaceResultsMarkup() : state.searchResults.map(result => `
                <button type="button" class="cs-search-result" data-search-path="${esc(result.path)}" data-search-line="${esc(result.line)}" title="${esc(result.path)}">
                    <span><strong>${esc(searchResultLabel(result.path))}</strong><em class="cs-search-line">${esc(result.line)}</em></span>
                    <code>${esc(result.preview)}</code>
                </button>`).join(''));
        }
        box.innerHTML = results;
        box.querySelectorAll('[data-search-path]').forEach(btn => {
            btn.addEventListener('click', bind(() => openSearchResult(btn.dataset.searchPath, Number(btn.dataset.searchLine || 1))));
        });
        if (state.searchReplaceOpen) wireProjectReplaceResults(box);
        updateProjectReplaceControls();
    }

    function toggleSearch() {
        state.searchVisible = !state.searchVisible;
        renderSearchPanel();
        renderActivityBar();
        renderWindowMenus();
        if (state.searchVisible) {
            const input = shellPart('[data-search] input[name="q"]');
            if (input) {
                input.focus();
                input.select();
            }
        }
    }

    async function runSearch(formData) {
        const target = state;
        if (!isLiveInstance(target)) return;
        const query = String(formData.get('q') || '').trim();
        if (!query) return;
        const request = target.searchRequest = Symbol();
        state.searchQuery = query;
        state.searchOptions = {
            case: !!formData.get('case'),
            whole: !!formData.get('whole'),
            regex: !!formData.get('regex'),
            include: String(formData.get('include') || ''),
            exclude: String(formData.get('exclude') || '')
        };
        renderStatus(tr('codeStudio.search', 'Search') + '...');
        const currentPath = target.currentPath || WORKSPACE_ROOT;
        try {
            const result = await apiClient.search({
                q: query,
                path: currentPath,
                case: state.searchOptions.case ? 'true' : 'false',
                whole: state.searchOptions.whole ? 'true' : 'false',
                regex: state.searchOptions.regex ? 'true' : 'false',
                include: state.searchOptions.include,
                exclude: state.searchOptions.exclude
            });
            if (!isLiveInstance(target) || target.searchRequest !== request) return;
            runWithInstance(target, () => {
                // Open files are listed from their buffers (replace.js); the others as the server found them.
                state.searchResults = overlayOpenTabResults(result.results || []);
                state.searchTruncated = !!result.truncated || state.searchResults.length > 200;
                state.searchPerformed = true;
                renderSearchResults();
                flashStatus(tr('codeStudio.resultsCount', '{{count}} results', { count: state.searchResults.length }));
            });
        } catch (err) {
            if (isLiveInstance(target) && target.searchRequest === request) {
                runWithInstance(target, () => {
                    state.searchResults = [];
                    state.searchTruncated = false;
                    state.searchPerformed = true;
                    renderSearchResults();
                    showOperationError(err);
                });
            }
        }
    }

    async function openSearchResult(path, line) {
        const target = state;
        if (!isLiveInstance(target)) return;
        await runAsyncStep(target, () => openFile(path));
        if (!isLiveInstance(target)) return;
        runWithInstance(target, () => {
            const tab = activeTab();
            if (!tab || tab.path !== normalizeCodeStudioPath(path) || !tab.view) return;
            if (tab.view.state && tab.view.state.doc && state.cmModule && state.cmModule.EditorView) {
                const lineNumber = Math.min(Math.max(1, line || 1), tab.view.state.doc.lines);
                const docLine = tab.view.state.doc.line(lineNumber);
                tab.view.dispatch({
                    selection: { anchor: docLine.from },
                    effects: state.cmModule.EditorView.scrollIntoView(docLine.from, { y: 'center' })
                });
                tab.view.focus();
            } else if (tab.view.textarea) {
                const lines = tab.view.textarea.value.split('\n');
                const offset = lines.slice(0, Math.max(0, (line || 1) - 1)).join('\n').length;
                tab.view.textarea.focus();
                tab.view.textarea.setSelectionRange(offset, offset);
            }
            renderStatus();
        });
    }
```

When replace is closed, the result list looks exactly as before: same `.cs-search-result` markup and the same `searchTruncated` and `codeStudio.resultsTruncated` markers. `ui/code_studio_status_search_test.go` checks those markers.

- [ ] **Step 4: Hook read-only switches into the replace controls (`ui/js/desktop/apps/code-studio/core.js`, shared region `updatePolicyControls`)**

Replace:
```js
        const knowledge = state.root.querySelector('[data-action="knowledge"]');
        if (knowledge) knowledge.disabled = !window.AuraGoLearnings || typeof window.AuraGoLearnings.open !== 'function';
    }
```
With:
```js
        const knowledge = state.root.querySelector('[data-action="knowledge"]');
        if (knowledge) knowledge.disabled = !window.AuraGoLearnings || typeof window.AuraGoLearnings.open !== 'function';
        updateProjectReplaceControls();
    }
```
Plan B may have changed the git selector line above this anchor; the anchor itself does not change. If Plan B moved the `knowledge` lines, put `updateProjectReplaceControls();` as the last statement of `updatePolicyControls`.

- [ ] **Step 5: Add the fragment to the Code Studio bundle (`scripts/build-ui-bundles.js`)**

Replace:
```js
      'ui/js/desktop/apps/code-studio/search.js',
```
With:
```js
      'ui/js/desktop/apps/code-studio/search.js',
      'ui/js/desktop/apps/code-studio/replace.js',
```
The new part sits between `core.js`, which opens the shared IIFE, and `shortcuts.js`, which closes it.

- [ ] **Step 6: Append the styles to `ui/css/code-studio.css`**

Append to the end of the file:

```css

/* === IDE package 1C: replace === */
/* The search panel head: a VS Code style chevron toggles the replace row under the search form. */
.cs-search-head {
    display: grid;
    grid-template-columns: auto minmax(0, 1fr);
    gap: 6px;
    align-items: center;
    min-width: 0;
}

.cs-search-head > .cs-search-form {
    grid-column: 2;
    grid-row: 1;
}

.cs-search-replace-toggle {
    grid-column: 1;
    grid-row: 1;
    align-self: stretch;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 20px;
    min-height: 30px;
    padding: 0;
    border: 1px solid transparent;
    border-radius: var(--cs-radius-sm);
    background: transparent;
    color: var(--cs-muted);
    font: inherit;
    font-size: 15px;
    line-height: 1;
    cursor: pointer;
}

.cs-search-head[data-replace-open="true"] .cs-search-replace-toggle {
    grid-row: 1 / span 2;
}

.cs-search-replace-toggle:hover {
    background: var(--cs-hover);
    color: var(--cs-text);
}

.cs-search-replace-toggle:focus-visible,
.cs-replace-action:focus-visible,
.cs-replace-preview:focus-visible {
    outline: 2px solid var(--cs-accent-border);
    outline-offset: 1px;
}

.cs-search-replace-toggle span {
    display: inline-block;
    transition: transform var(--cs-transition-fast);
}

.cs-search-replace-toggle[aria-expanded="true"] span {
    transform: rotate(90deg);
}

.cs-search-replace-row {
    grid-column: 2;
    grid-row: 2;
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 8px;
    min-width: 0;
}

.cs-search-replace-row input[name="replace"] {
    flex: 1 1 220px;
    min-width: 0;
    max-width: 520px;
    height: 30px;
    padding: 0 9px;
    border: 1px solid var(--vd-mat-field-border, var(--cs-border));
    border-radius: var(--vd-mat-field-radius, var(--ds-radius-sm));
    color: var(--cs-text);
    background: var(--vd-mat-field, var(--cs-panel-soft));
    box-shadow: var(--vd-mat-field-shadow, none);
    font: inherit;
    font-size: 12px;
}

.cs-search-replace-row input[name="replace"]:focus {
    outline: none;
    border-color: var(--cs-accent-border);
}

.cs-search-replace-hint {
    flex: 1 1 200px;
    min-width: 0;
    color: var(--cs-muted);
    font-size: 11px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
}

.cs-search-replace-hint:empty {
    display: none;
}

.code-studio-search .cs-button:disabled,
.cs-search-replace-row input:disabled,
.cs-replace-action:disabled {
    opacity: 0.5;
    cursor: not-allowed;
}

/* Results in replace mode: one header per file, one row per match. */
.cs-replace-file {
    display: flex;
    align-items: center;
    gap: 8px;
    min-height: 26px;
    margin-top: 4px;
    padding: 2px 6px;
    border-radius: var(--ds-radius-sm);
    background: var(--cs-accent-faint);
}

.cs-replace-file-name {
    flex: 1 1 auto;
    display: flex;
    align-items: baseline;
    gap: 6px;
    min-width: 0;
    overflow: hidden;
    color: var(--cs-muted);
    font-size: 12px;
    white-space: nowrap;
}

.cs-replace-file-name strong {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    color: var(--cs-text);
    font-weight: 600;
}

.cs-replace-count {
    flex: 0 0 auto;
    min-width: 18px;
    padding: 0 6px;
    border-radius: 9px;
    background: var(--cs-accent-soft);
    color: var(--cs-text);
    font-size: 10px;
    font-style: normal;
    line-height: 16px;
    text-align: center;
    font-variant-numeric: tabular-nums;
}

.cs-replace-match {
    display: flex;
    align-items: center;
    gap: 6px;
    min-width: 0;
    padding-left: 12px;
}

.cs-replace-preview {
    flex: 1 1 auto;
    display: flex;
    align-items: baseline;
    gap: 8px;
    min-width: 0;
    min-height: 24px;
    padding: 2px 6px;
    border: 1px solid transparent;
    border-radius: var(--ds-radius-sm);
    background: transparent;
    color: var(--cs-text);
    font: inherit;
    text-align: left;
    cursor: pointer;
}

.cs-replace-preview:hover {
    border-color: var(--cs-accent-border);
    background: var(--cs-hover);
}

.cs-replace-preview code {
    flex: 1 1 auto;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: pre;
    color: var(--cs-text);
    font: 12px/1.4 var(--cs-mono-font);
}

/* Old text struck through, new text after it. The ink follows the diff colours, readable in light and dark themes. */
.cs-replace-old {
    border-radius: 2px;
    background: var(--cs-danger-soft);
    color: light-dark(color-mix(in srgb, var(--cs-danger) 18%, var(--vd-text)), var(--cs-danger));
    text-decoration: line-through;
}

.cs-replace-new {
    border-radius: 2px;
    background: var(--cs-accent-soft);
    color: light-dark(color-mix(in srgb, var(--cs-accent) 18%, var(--vd-text)), var(--cs-accent));
    text-decoration: none;
}

.cs-replace-nomatch {
    flex: 0 0 auto;
    color: var(--cs-muted);
    font-size: 10px;
    font-style: italic;
}

.cs-replace-match.is-nomatch code {
    color: var(--cs-muted);
}

.cs-replace-action {
    flex: 0 0 auto;
    height: 22px;
    padding: 0 8px;
    border: 1px solid var(--vd-mat-btn-border, var(--cs-border));
    border-radius: var(--cs-radius-sm);
    background: var(--vd-mat-btn, var(--cs-panel-soft));
    color: var(--vd-mat-btn-text, var(--cs-text));
    font: inherit;
    font-size: 11px;
    white-space: nowrap;
    cursor: pointer;
}

.cs-replace-action:hover:not(:disabled) {
    border-color: var(--cs-accent-border);
    background: var(--vd-mat-btn-hover, var(--cs-accent-soft));
}

/* The button for one match shows on hover and keyboard focus, like VS Code; touch screens always show it. */
.cs-replace-match .cs-replace-action {
    opacity: 0;
}

.cs-replace-match:hover .cs-replace-action,
.cs-replace-match:focus-within .cs-replace-action {
    opacity: 1;
}

.cs-replace-match:hover .cs-replace-action:disabled,
.cs-replace-match:focus-within .cs-replace-action:disabled {
    opacity: 0.5;
}

@media (hover: none) {
    .cs-replace-match .cs-replace-action {
        opacity: 1;
    }
}
```

- [ ] **Step 7: Build and check**

Run: `npm run build:ui`
Expected: a line `Built ui/js/desktop/bundles/code-studio.bundle.js (15 parts)` (more parts if Plans A or B added fragments), exit code 0.

Run: `node scripts/build-ui-bundles.js --check`
Expected: every line starts with `Checked`, exit code 0.

Run: `node --check ui/js/desktop/bundles/code-studio.bundle.js`
Expected: no output, exit code 0.

Run: `npm run test:code-studio-replace`
Expected: `Code Studio replace helper tests passed`.

Run: `go test ./ui -run '(?i)^(TestCodeStudioStatusAndSearchCap)$' -count=1 -v`
Expected: `--- PASS: TestCodeStudioStatusAndSearchCap`.

- [ ] **Step 8: Graph check** (`detect-changes --scope all`). Report the affected processes. Expected: search panel rendering, policy controls, bundle.

**Commit:** `feat(code-studio): project-wide replace in the search panel`. Files:
- `ui/js/desktop/apps/code-studio/replace.js`
- `ui/js/desktop/apps/code-studio/search.js`
- `ui/js/desktop/apps/code-studio/core.js`
- `scripts/build-ui-bundles.js`
- `ui/css/code-studio.css`
- `ui/js/desktop/bundles/code-studio.bundle.js`

---

### Task 4: Browser test for the replace area

**Files:**
- Modify: `ui/desktop_code_studio_browser_test.go` (fixture: truncated flag)
- Create: `ui/desktop_code_studio_replace_browser_test.go`

The fixture today:
- The fake search lowercases `q` and matches substrings, ignoring case, whole and regex.
- Previews are trimmed.
- It never sends `truncated`.
- File GET and PUT work on `backend.files`, so a PUT for a file shows up in that map.

The test uses lines without indentation, so trimming does not matter. The fixture only needs a switch for `truncated`.

- [ ] **Step 1: Fixture field (`ui/desktop_code_studio_browser_test.go`)**

Replace:
```go
type codeStudioFixtureBackend struct {
```
With the block below. Keep the empty line after the new field: it starts a separate gofmt alignment section, so the existing fields stay untouched.
```go
type codeStudioFixtureBackend struct {
	// searchTruncated makes the fake search answer like the server at its 200-result cap.
	searchTruncated bool

```

- [ ] **Step 2: Fixture search answer (same file; the handler already holds `b.mu`)**

Replace:
```go
		writeJSON(w, map[string]interface{}{"status": "ok", "results": results})
```
With:
```go
		payload := map[string]interface{}{"status": "ok", "results": results}
		if b.searchTruncated {
			payload["truncated"] = true
		}
		writeJSON(w, payload)
```

- [ ] **Step 3: Create `ui/desktop_code_studio_replace_browser_test.go` with this complete content**

```go
package ui

import "testing"

// Project-wide replace in the Code Studio search panel: the preview, Replace All writing a closed file
// through PUT, an open tab with unsaved edits taking the change in its buffer (one undo step, no PUT),
// Replace All switched off for a truncated result list, and read-only mode switching replace off.
// The fixture UI runs in German, so every check uses data attributes and state, not labels.
func TestDesktopCodeStudioReplaceBrowser(t *testing.T) {
	const alpha = "/workspace/replace-alpha.txt"
	const beta = "/workspace/replace-beta.txt"
	page, backend := newCodeStudioBrowser(t)
	backend.mu.Lock()
	backend.files[alpha] = "quokka one\nquokka quokka two\nplain\n"
	backend.files[beta] = "keep quokka here\n"
	backend.mu.Unlock()
	browserErrors := func() string {
		return page.MustEval(`()=>JSON.stringify(fixtureErrors)`).Str()
	}
	scenario := func(name, js string, args ...interface{}) {
		t.Helper()
		result, err := page.Eval(js, args...)
		if err != nil {
			t.Fatalf("%s: eval failed: %v; browser errors: %s", name, err, browserErrors())
		}
		if !result.Value.Get("ok").Bool() {
			t.Fatalf("%s: %s; browser errors: %s", name, result.Value.JSON("", ""), browserErrors())
		}
	}
	if _, err := page.Eval(`async()=>{await csWait(()=>csRoot()&&csRoot().querySelectorAll('.cs-tree-item').length>0,20000);}`); err != nil {
		t.Fatalf("Code Studio did not open: %v; errors=%s", err, browserErrors())
	}

	// beta.txt gets an unsaved edit that shifts its lines; then main.go becomes the active tab, so beta
	// lives on as a background tab whose editor state sits in its snapshot.
	scenario("dirty background tab", `async(beta)=>{
		await CodeStudioApp.openFile(beta);
		const tab=await csWait(()=>csState().openTabs.find(item=>item.path===beta&&item.view&&item.view.state),10000);
		tab.view.dispatch({changes:{from:0,insert:'// local\n'}});
		await csWait(()=>tab.modified,2000);
		await CodeStudioApp.openFile('/workspace/main.go');
		await csWait(()=>{const active=csState().openTabs[csState().activeTabIndex];return active&&active.path==='/workspace/main.go';},10000);
		return {ok:tab.modified&&!tab.view&&tab.content==='// local\nkeep quokka here\n',modified:tab.modified,hasView:!!tab.view,content:tab.content};
	}`, beta)

	// Search, open the replace row and type the replacement: every match shows old and new text, the
	// second match on a line has its own row, and beta's row uses the buffer's line number (2, not 1).
	scenario("preview", `async()=>{
		csRoot().querySelector('[data-activity="search"]').click();
		const form=csRoot().querySelector('[data-search-form]');
		form.elements.q.value='quokka';
		form.requestSubmit();
		await csWait(()=>csState().searchPerformed&&csState().searchQuery==='quokka'&&csRoot().querySelectorAll('.cs-search-result').length>0,10000);
		csRoot().querySelector('[data-search-replace-toggle]').click();
		const input=await csWait(()=>csRoot().querySelector('[data-search-replace-input]'),2000);
		input.value='wombat';
		input.dispatchEvent(new Event('input',{bubbles:true}));
		await csWait(()=>csRoot().querySelectorAll('.cs-replace-new').length===4,4000);
		const olds=[...csRoot().querySelectorAll('.cs-replace-old')].map(el=>el.textContent);
		const news=[...csRoot().querySelectorAll('.cs-replace-new')].map(el=>el.textContent);
		const second=csRoot().querySelector('[data-replace-one][data-replace-path="/workspace/replace-alpha.txt"][data-replace-line="2"][data-replace-occurrence="1"]');
		const row=second&&second.closest('.cs-replace-match');
		const rowText=row?row.querySelector('code').textContent:'';
		const betaRow=csRoot().querySelector('[data-replace-one][data-replace-path="/workspace/replace-beta.txt"]');
		const all=csRoot().querySelector('[data-search-replace-all]');
		return {
			ok:olds.length===4&&olds.every(text=>text==='quokka')&&news.every(text=>text==='wombat')&&rowText==='quokka quokkawombat two'&&!!betaRow&&betaRow.dataset.replaceLine==='2'&&csRoot().querySelectorAll('[data-replace-file-action]').length===2&&!!all&&!all.disabled,
			olds,news,rowText,betaLine:betaRow&&betaRow.dataset.replaceLine,allDisabled:all&&all.disabled
		};
	}`)

	// Replace All asks with the counts, writes the closed file and changes the open tab only in its buffer.
	scenario("replace all", `async()=>{
		csRoot().querySelector('[data-search-replace-all]').click();
		const confirm=await csWait(()=>document.querySelector('.cs-modal-backdrop [data-confirm]'),4000);
		const message=document.querySelector('.cs-modal-backdrop p').textContent;
		confirm.click();
		const report=await csWait(()=>csState().projectReplaceReport,10000);
		return {
			ok:/4/.test(message)&&/2/.test(message)&&report.count===4&&report.files===2&&report.buffers===1&&report.skipped.length===0,
			message,count:report.count,files:report.files,buffers:report.buffers,skipped:report.skipped
		};
	}`)
	backend.mu.Lock()
	gotAlpha, gotBeta := backend.files[alpha], backend.files[beta]
	backend.mu.Unlock()
	if want := "wombat one\nwombat wombat two\nplain\n"; gotAlpha != want {
		t.Errorf("Replace All wrote %q to the closed file, want %q", gotAlpha, want)
	}
	if gotBeta != "keep quokka here\n" {
		t.Errorf("the open tab's file was written to disk: %q", gotBeta)
	}

	// The background tab stays dirty, shows the change when it is activated, and one undo takes it back.
	scenario("dirty tab got the change in its buffer", `async(beta)=>{
		const tab=csState().openTabs.find(item=>item.path===beta);
		const want='// local\nkeep wombat here\n';
		const before={modified:tab.modified,content:tab.content};
		await CodeStudioApp.openFile(beta);
		await csWait(()=>tab.view&&tab.view.state,10000);
		const shown=tab.view.state.doc.toString();
		const undone=csState().cmModule.undo(tab.view);
		const afterUndo=tab.view.state.doc.toString();
		return {ok:before.modified&&before.content===want&&shown===want&&undone&&afterUndo==='// local\nkeep quokka here\n'&&tab.modified,before,shown,undone,afterUndo};
	}`, beta)

	// A truncated result list switches Replace All off with a hint; Replace in File stays usable.
	backend.mu.Lock()
	backend.searchTruncated = true
	backend.mu.Unlock()
	scenario("Replace All is off for a truncated list", `async()=>{
		const form=csRoot().querySelector('[data-search-form]');
		form.elements.q.value='plain';
		form.requestSubmit();
		await csWait(()=>csState().searchQuery==='plain'&&csState().searchTruncated&&!csState().projectReplaceBusy&&csRoot().querySelector('[data-replace-file="/workspace/replace-alpha.txt"]'),10000);
		const all=csRoot().querySelector('[data-search-replace-all]');
		const file=csRoot().querySelector('[data-replace-file-action][data-replace-path="/workspace/replace-alpha.txt"]');
		const hint=csRoot().querySelector('[data-search-replace-hint]').textContent.trim();
		return {ok:all.disabled&&!!file&&!file.disabled&&hint.length>0,allDisabled:all.disabled,fileDisabled:file&&file.disabled,hint};
	}`)

	// Read-only mode switches the replace input and every replace button off.
	scenario("read-only turns replace off", `async()=>{
		document.dispatchEvent(new CustomEvent('aurago:desktop-policy',{detail:{readonly:true}}));
		await csWait(()=>csState().readOnly,2000);
		const all=csRoot().querySelector('[data-search-replace-all]');
		const input=csRoot().querySelector('[data-search-replace-input]');
		const buttons=[...csRoot().querySelectorAll('[data-replace-file-action],[data-replace-one]')];
		return {ok:all.disabled&&input.disabled&&buttons.length>0&&buttons.every(button=>button.disabled),allDisabled:all.disabled,inputDisabled:input.disabled,buttons:buttons.map(button=>button.disabled)};
	}`)

	scenario("no browser errors", `()=>({ok:fixtureErrors.length===0,errors:fixtureErrors})`)
}
```

- [ ] **Step 4: gofmt.** Run `gofmt -l ui/desktop_code_studio_browser_test.go ui/desktop_code_studio_replace_browser_test.go`. Expected: no output. If a file is listed (for example because Plan A or B added fixture fields next to the edit), run `gofmt -w` on that file.

- [ ] **Step 5: Run the browser tests.** The env gate is `AURAGO_RUN_BROWSER_SMOKE=1`. Use the PowerShell tool with the sandbox off, because Chrome must start. The existing main test is included as the regression check for the rebuilt search panel.

```powershell
$env:AURAGO_RUN_BROWSER_SMOKE = '1'; go test ./ui -run '(?i)^(TestDesktopCodeStudioReplaceBrowser|TestDesktopCodeStudioBrowser)$' -count=1 -timeout 15m -v; Remove-Item Env:AURAGO_RUN_BROWSER_SMOKE
```
Expected: `--- PASS: TestDesktopCodeStudioReplaceBrowser` and `--- PASS: TestDesktopCodeStudioBrowser`. Count the PASS lines as the evidence. Without the env variable both tests skip, and that is not a pass.

If "dirty tab got the change in its buffer" fails with `shown` still containing `quokka`: Plan A changed where a background tab keeps its editor state. Adjust `projectReplaceSnapshot` in `replace.js` to Plan A's field, rebuild with `npm run build:ui`, and re-run.

- [ ] **Step 6: Graph check** (`detect-changes --scope all`).

**Commit:** `test(code-studio): browser test for project-wide replace`. Files:
- `ui/desktop_code_studio_browser_test.go`
- `ui/desktop_code_studio_replace_browser_test.go`

---

### Task 5: Final verification of package 1C

**Files:** none changed. Only fix and re-commit if a check fails.

- [ ] **Step 1: Generated files are current.** Run `node scripts/build-ui-bundles.js --check`. Expected: only `Checked ...` lines, exit code 0.
- [ ] **Step 2: Unit test.** Run `npm run test:code-studio-replace`. Expected: `Code Studio replace helper tests passed`.
- [ ] **Step 3: Code Studio Go tests, static and browser, with the sandbox off:**

```powershell
$env:AURAGO_RUN_BROWSER_SMOKE = '1'; go test ./ui -run '(?i)^(TestDesktopCodeStudio|TestCodeStudio)' -count=1 -timeout 30m -v; Remove-Item Env:AURAGO_RUN_BROWSER_SMOKE
```
Expected: every test reports `--- PASS`, including `TestDesktopCodeStudioReplaceBrowser`, `TestDesktopCodeStudioBrowser` and `TestCodeStudioStatusAndSearchCap`. This also covers the repair test, which switches read-only and so passes through the new `updatePolicyControls` hook.

- [ ] **Step 4: Graph check over everything** (`detect-changes --scope all`). It must not be `partial` or `truncated`. Hand the result to the controller with the commit list.

No commit unless a fix was needed. A fix commit lists exactly the files it changed.

---

## Spec coverage (self-review)

| Spec C requirement | Where |
| --- | --- |
| Replace input toggled like VS Code | `search.js` chevron `[data-search-replace-toggle]`, `toggleProjectReplace`, CSS rotate |
| Preview per match: old struck through, new after it | `projectReplaceResultsMarkup` (`<del class="cs-replace-old">` / `<ins class="cs-replace-new">`) built from `replaceMatchPreview` |
| Replace one / in file / all; confirm "Replace N occurrences in M files?" | `projectReplaceMatch`, `projectReplaceFile`, `projectReplaceAll` + `codeStudio.replaceAllConfirm` |
| JS RegExp from the search options: literal escaped, `\b`, case flag | `buildReplaceRegExp` (+ Node tests) |
| Regex mode `$1` groups with JS semantics; preview equal to the write | `expandReplaceTemplate` checked against native `replace`; both paths use `replaceLineSegments` |
| Replace All off for a truncated list, with a hint; per-file stays | `updateProjectReplaceControls`, `projectReplaceHint`, browser test "truncated" |
| Open tab: one undoable transaction, stays or becomes dirty, nothing written | `applyProjectReplaceToTab` (view dispatch / textarea `setValue` / snapshot transaction), browser test incl. `undo` |
| Not open: GET then PUT | `projectReplaceInFile` |
| Read-only disables replace | `updateProjectReplaceControls` via the `updatePolicyControls` hook + `canWrite` checks; browser test |
| Search runs again afterwards | `runProjectReplace` → `rerunProjectSearch` |
| No backend change | only the client and the test fixture change |
| Files without a JS match skipped and reported | `noMatchGroups` + `planFileReplace` `nomatch` → Output |
| Per-file errors in the Output panel, others continue | per-file `try/catch` in `runProjectReplace`, `finishProjectReplace` |
| One match: exact line + occurrence, re-verified; stale gives a notice and a re-run | `planFileReplace` occurrence + `sameSearchLine`; `codeStudio.replaceStale` |
| Large files (server cap) per file | 413 → `codeStudio.replaceSkippedTooLarge` |
| i18n in 16 files, `cs-*` CSS readable in light and dark, bundles rebuilt, one commit set for C | Tasks 2, 3 (`light-dark()` ink like the diff lines), 3 Step 7 |
| Tests: one browser test (preview, replace all writes, dirty tab stays dirty) + truncated | Task 4 |
