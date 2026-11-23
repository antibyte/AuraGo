# Notes Obsidian Round Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give the desktop Notes app Obsidian-style syntax, linking, navigation, graph and writing features with a
cleaner UI, per `docs/superpowers/specs/2026-11-23-notes-obsidian-design.md`.

**Architecture:** Obsidian syntax is added to the bundled Milkdown engine as a pure micromark/mdast layer plus a
Milkdown/ProseMirror layer. The Go service gains a cached notes index (refs + link resolution) used by a new
client index module; new UI modules (sidebar, palette, graph, panels, templates) hang off the existing
`notes.js` orchestrator.

**Tech Stack:** Milkdown/Crepe 7.22.3 + ProseMirror + micromark/mdast (bundled with esbuild), CodeMirror 6
bundle, vanilla JS desktop app modules, Go 1.x service + net/http handlers, `node --test`, Go tests, rod
browser test.

**Plan granularity:** Phases 1 and 2 are written out in full. Phases 3–7 list files, interfaces, behaviour and
tests; each is expanded into full steps in this file right before it is executed, using what the earlier
phases actually produced (signatures, CSS hooks). Every phase ends in its own commit.

**Shared rules for every task**
- Other sessions commit in this checkout. Stage only the files of the task (`git add <paths>`), never `git add -A`;
  commit with `git commit -- <paths>` if other files are staged. End commit messages with
  `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.
- `docs/` is git-ignored: use `git add -f` for files under `docs/`.
- Scratch files go to the session scratchpad, never into the repo.
- GitNexus rule: before editing an existing Go function, run
  `node ../../../.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "<symbol>" --direction upstream --repo .`
  and report HIGH/CRITICAL risk.
- New UI strings: add the key to all 16 files in `ui/lang/desktop/` (en and de written by hand, the others
  translated), alphabetically near the other `desktop.notes_*` keys. The app reads them through `tr('<suffix>')`,
  which tries `desktop.notes_<suffix>`, then `desktop.writer_<suffix>`, then `desktop.<suffix>`.

---

## Phase 1 — Obsidian syntax in the editor engine

### File structure

- Create `scripts/notes-obsidian-markdown.js` — pure micromark syntax + mdast from/to-markdown extensions for
  wikilinks, embeds, hashtags, callout markers, highlights; `parseWikiValue`. No DOM, no Milkdown imports.
- Create `scripts/notes-obsidian-markdown.test.mjs` — node round-trip tests for the layer above.
- Create `scripts/notes-obsidian.js` — Milkdown plugins: `$remark`, node/mark schemas, node views, input rules,
  callout decoration plugin, commands; exports `obsidian(options)` and `parseWikiValue`.
- Modify `scripts/notes-engine-entry.js` — add `export { obsidian, parseWikiValue } from './notes-obsidian.js';`.
- Regenerate `ui/js/vendor/notes/engine.js`, `engine.css`, `manifest.json`, `LICENSES.md` via
  `npm run build:notes-vendor`.
- Modify `ui/js/desktop/apps/notes-editor.js` — register `lib.obsidian(...)`, drop wikilink/embed from
  `needsSource`, add `highlight`, `insertWiki`, `insertCallout` commands.
- Modify `ui/js/desktop/apps/notes.js` — pass the new callbacks (Phase-1 level: simple open by title/basename).
- Modify `ui/css/desktop-app-notes.css` — chip, tag, highlight, callout, embed styles.
- Modify `ui/testdata/notes-functional.js` — rich round trip of a note with all syntax.

### Task 1.1: Markdown layer with round-trip tests

**Files:**
- Create: `scripts/notes-obsidian-markdown.js`
- Test: `scripts/notes-obsidian-markdown.test.mjs`

- [ ] **Step 1: Write the failing test**

```js
// scripts/notes-obsidian-markdown.test.mjs
import test from 'node:test';
import assert from 'node:assert/strict';
import {fromMarkdown} from 'mdast-util-from-markdown';
import {toMarkdown} from 'mdast-util-to-markdown';
import {gfm} from 'micromark-extension-gfm';
import {gfmFromMarkdown, gfmToMarkdown} from 'mdast-util-gfm';
import {obsidianSyntax, obsidianFromMarkdown, obsidianToMarkdown, parseWikiValue} from './notes-obsidian-markdown.js';

const parse = text => fromMarkdown(text, {extensions: [gfm(), obsidianSyntax()], mdastExtensions: [gfmFromMarkdown(), obsidianFromMarkdown()]});
const stringify = tree => toMarkdown(tree, {bullet: '-', extensions: [gfmToMarkdown(), obsidianToMarkdown()]});
const all = (node, type, out = []) => { if (node.type === type) out.push(node); for (const child of node.children || []) all(child, type, out); return out; };
const same = text => assert.equal(stringify(parse(text)), text);

test('wikilinks parse with raw value and round-trip verbatim', () => {
    const tree = parse('See [[Folder/Note#Heading|Alias]] and [[Plain]].\n');
    assert.deepEqual(all(tree, 'wikiLink').map(n => [n.value, n.embed]), [['Folder/Note#Heading|Alias', false], ['Plain', false]]);
    same('See [[Folder/Note#Heading|Alias]] and [[Plain]].\n');
    same('[[Note]] starts the line\n');
    same('Embed ![[image.png]] inline\n');
    same('![[Other note#Part]]\n');
    assert.deepEqual(all(parse('![[image.png]]\n'), 'wikiLink').map(n => [n.value, n.embed]), [['image.png', true]]);
});

test('wikilinks inside table cells keep the escaped pipe', () => {
    const tree = parse('| Link |\n| - |\n| [[Note\\|Alias]] |\n');
    assert.deepEqual(all(tree, 'wikiLink').map(n => n.value), ['Note\\|Alias']);
    const out = stringify(tree);
    assert.ok(out.includes('[[Note\\|Alias]]'), out);
    assert.deepEqual(all(parse(out), 'wikiLink').map(n => n.value), ['Note\\|Alias']);
    assert.deepEqual(parseWikiValue('Note\\|Alias'), {target: 'Note', subpath: '', alias: 'Alias'});
});

test('parseWikiValue splits target, subpath and alias', () => {
    assert.deepEqual(parseWikiValue('Folder/Note#Heading|Alias'), {target: 'Folder/Note', subpath: 'Heading', alias: 'Alias'});
    assert.deepEqual(parseWikiValue('Note#^block-1'), {target: 'Note', subpath: '^block-1', alias: ''});
    assert.deepEqual(parseWikiValue('#Local heading'), {target: '', subpath: 'Local heading', alias: ''});
    assert.deepEqual(parseWikiValue(' Spaced | Shown '), {target: 'Spaced', subpath: '', alias: 'Shown'});
});

test('non-wikilinks stay literal', () => {
    for (const text of ['\\[[x]]\n', 'a [[b\nc]] d\n', '`[[code]]`\n', '```\n[[fenced]]\n```\n', '[[]]\n']) {
        const tree = parse(text);
        assert.equal(all(tree, 'wikiLink').length, 0, text);
        assert.equal(all(parse(stringify(tree)), 'wikiLink').length, 0, 'reparse ' + text);
    }
    same('```\n[[fenced]]\n```\n');
    // Like Obsidian: an unclosed outer [[ stays literal, the inner link still works.
    assert.deepEqual(all(parse('[[a [[b]]\n'), 'wikiLink').map(n => n.value), ['b']);
});

test('hashtags parse at word starts and never get escaped', () => {
    same('#tag at the start\n');
    const tree = parse('Text #projekt/sub-a_b and #über (#paren)\n');
    assert.deepEqual(all(tree, 'hashtag').map(n => n.value), ['projekt/sub-a_b', 'über', 'paren']);
    same('Text #projekt/sub-a_b and #über (#paren)\n');
    assert.deepEqual(all(parse('#12a ok\n'), 'hashtag').map(n => n.value), ['12a']);
    for (const text of ['Issue #123\n', 'a#b\n', '# Heading\n', 'http://example.com/#frag\n', '`#code`\n']) {
        assert.equal(all(parse(text), 'hashtag').length, 0, text);
    }
});

test('callout markers survive in blockquotes', () => {
    const tree = parse('> [!warning]- Careful\n> Body text\n');
    assert.deepEqual(all(tree, 'calloutMarker').map(n => [n.value, n.fold]), [['warning', '-']]);
    same('> [!warning]- Careful\n> Body text\n');
    same('> [!note]\n> Body\n');
    same('> [!tip]+ Open by default\n');
    assert.equal(all(parse('[!x](https://example.com)\n'), 'calloutMarker').length, 0);
    assert.equal(all(parse('text [!x] mid\n'), 'calloutMarker').length, 0);
});

test('highlights allow nested phrasing and line starts', () => {
    const tree = parse('==marked== and ==**bold** inside==\n');
    const marks = all(tree, 'highlight');
    assert.equal(marks.length, 2);
    assert.equal(all(marks[1], 'strong').length, 1);
    same('==marked== and ==**bold** inside==\n');
    same('==start== of a line\n');
    for (const text of ['a == b\n', '===\n', 'x ==not closed\n']) assert.equal(all(parse(text), 'highlight').length, 0, text);
});

test('a full Obsidian note round-trips unchanged', () => {
    same('# Projekt\n\nVerweis auf [[Ideen|meine Ideen]] und #projekt/alpha.\n\n> [!info] Hinweis\n> Siehe ==wichtig== und ![[skizze.png]]\n\n- [ ] Aufgabe mit [[Aufgaben#Heute]]\n');
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `node --test scripts/notes-obsidian-markdown.test.mjs`
Expected: FAIL — `Cannot find module ... notes-obsidian-markdown.js`.

- [ ] **Step 3: Implement `scripts/notes-obsidian-markdown.js`**

Required exports and behaviour (ESM, imports only `micromark-util-character`, `micromark-util-chunked`,
`micromark-util-classify-character`, `micromark-util-resolve-all`, `micromark-util-symbol` if needed — all in
`node_modules`):

```js
export function obsidianSyntax()        // micromark extension: {text: {33: [embedConstruct], 35: [hashtagConstruct], 61: [highlightConstruct], 91: [wikiLinkConstruct, calloutConstruct]}, insideSpan: {null: [highlightResolveAll]}, attentionMarkers: {null: [61]}}
export function obsidianFromMarkdown()   // {enter: {...}, exit: {...}} creating the mdast nodes below
export function obsidianToMarkdown()     // {handlers: {wikiLink, hashtag, calloutMarker, highlight}, unsafe: []}
export function parseWikiValue(value)    // {target, subpath, alias}
```

Constructs (put them `before` the CommonMark ones, i.e. return them in the arrays above; micromark tries
extension constructs first when added via `{text: {...}}` with the default `add: 'before'`... use
`combineExtensions` semantics: set `add: 'before'` on each construct object):

1. `wikiLink` (code 91 `[`): requires `[[`; then one or more chars that are not line endings, not `[`, not
   `]` except a `]` that is not followed by `]`… simpler rule: consume until the first `]]`; fail on line ending,
   EOF, `[[`, empty value or > 500 chars. A backslash consumes the next char (so `\|` and `\]` stay in the value).
   Tokens: `wikiLink` > `wikiLinkMarker` `[[`, `wikiLinkValue`, `wikiLinkMarker` `]]`. The from-markdown handler
   reads the value with `this.sliceSerialize(token)` on `wikiLinkValue` and creates
   `{type: 'wikiLink', value, embed: false}`.
2. `embed` (code 33 `!`): `!` followed by the wikilink grammar; node `{type: 'wikiLink', value, embed: true}`.
3. `hashtag` (code 35 `#`): `previous` must be `null`, a line ending, whitespace or `(`; following chars
   (one or more) match `/[\p{L}\p{N}_\-\/]/u` (test with `String.fromCharCode(code)`); the value must contain a
   char that is not a digit; stops before any other char. Node `{type: 'hashtag', value}`.
4. `calloutMarker` (code 91 `[`, tried after `wikiLink`): only when `this.previous === null` (paragraph start);
   `[!` + one or more `[A-Za-z0-9_-]` + `]` + optional `+`/`-`; the next code must be a line ending, space,
   tab or EOF. Node `{type: 'calloutMarker', value: <type lowercased as written>, fold: '' | '+' | '-'}`.
5. `highlight` (code 61 `=`): port `node_modules/micromark-extension-gfm-strikethrough/lib/syntax.js` with the
   marker `=` (61), `singleTilde` behaviour replaced by "exactly two markers" (a run of 1 or ≥3 `=` is not a
   highlight), token names `highlight`, `highlightSequence`, `highlightText`. From-markdown mirrors
   `node_modules/mdast-util-gfm-strikethrough/lib/index.js` (`{type: 'highlight', children: []}`).

To-markdown handlers:
- `wikiLink(node) => (node.embed ? '!' : '') + '[[' + node.value + ']]'` (value verbatim, never escaped).
- `hashtag(node) => '#' + node.value`.
- `calloutMarker(node) => '[!' + node.value + ']' + (node.fold || '')`.
- `highlight(node, _, state, info)` — port the strikethrough handler with `==` (`state.enter('highlight')`,
  `state.containerPhrasing(node, {...info, before: '=', after: '='})`), plus `peek` returning `'='`.
  Add `inConstruct` name `'highlight'` to the handler's phrasing so nested text is escaped correctly.

`parseWikiValue(value)`: split at the first `|` not preceded by `\` → `alias`; in the left part replace `\|`
with `|`; split at the first `#` → `target` and `subpath`; trim all three.

- [ ] **Step 4: Run tests to verify they pass**

Run: `node --test scripts/notes-obsidian-markdown.test.mjs`
Expected: all 8 tests PASS. If a case cannot pass with the grammar above, fix the grammar, not the test,
unless the test contradicts the spec (then note it in the report).

- [ ] **Step 5: Commit**

```bash
git add scripts/notes-obsidian-markdown.js scripts/notes-obsidian-markdown.test.mjs
git commit -m "feat(notes): Obsidian markdown syntax layer with round-trip tests" -- scripts/notes-obsidian-markdown.js scripts/notes-obsidian-markdown.test.mjs
```

### Task 1.2: Milkdown layer and engine rebuild

**Files:**
- Create: `scripts/notes-obsidian.js`
- Modify: `scripts/notes-engine-entry.js`
- Regenerate: `ui/js/vendor/notes/{engine.js,engine.css,manifest.json,LICENSES.md}`

- [ ] **Step 1: Implement `scripts/notes-obsidian.js`**

```js
import {$remark, $nodeSchema, $markSchema, $inputRule, $prose, $view} from '@milkdown/kit/utils';
import {InputRule} from '@milkdown/kit/prose/inputrules';
import {Plugin, PluginKey} from '@milkdown/kit/prose/state';
import {Decoration, DecorationSet} from '@milkdown/kit/prose/view';
import {toggleMark} from '@milkdown/kit/prose/commands';
import {obsidianSyntax, obsidianFromMarkdown, obsidianToMarkdown, parseWikiValue} from './notes-obsidian-markdown.js';
export {parseWikiValue};

// options: {resolve(value)->{exists:boolean}|null, open(value, event), embed(value, element, embedState),
//           preview(value|null, anchorElement), tag(value, event), icon(name)->html string, label(type)->string}
export function obsidian(options = {}) { /* returns a flat array of Milkdown plugins */ }
export const obsidianCommands = { toggleHighlight(state, dispatch) {...}, insertWiki(view, value, embed) {...}, insertCallout(view, type, title) {...} };
```

Contents of `obsidian(options)`:
- `$remark('obsidianSyntax', () => function () { const data = this.data(); (data.micromarkExtensions ||= []).push(obsidianSyntax()); (data.fromMarkdownExtensions ||= []).push(obsidianFromMarkdown()); (data.toMarkdownExtensions ||= []).push(obsidianToMarkdown()); })`
- `$nodeSchema('wiki_link', () => ({group: 'inline', inline: true, atom: true, selectable: true, attrs: {value: {default: ''}, embed: {default: false}}, leafText: n => (n.attrs.embed ? '!' : '') + '[[' + n.attrs.value + ']]', parseDOM: [{tag: 'span[data-wiki-link]', getAttrs: d => ({value: d.dataset.wikiLink, embed: d.hasAttribute('data-embed')})}], toDOM: n => ['span', {'data-wiki-link': n.attrs.value, ...(n.attrs.embed ? {'data-embed': ''} : {})}, n.attrs.value], parseMarkdown: {match: n => n.type === 'wikiLink', runner: (state, n, type) => state.addNode(type, {value: n.value, embed: !!n.embed})}, toMarkdown: {match: n => n.type.name === 'wiki_link', runner: (state, n) => state.addNode('wikiLink', undefined, undefined, {value: n.attrs.value, embed: n.attrs.embed})}}))`
- `hashtag` atom (`attrs {value}`, `leafText '#'+value`, `span[data-hashtag]`, mdast `hashtag`).
- `callout_marker` atom (`attrs {value, fold}`, `leafText '[!'+value+']'+fold`, `span[data-callout-marker]`,
  mdast `calloutMarker`).
- `$markSchema('highlight', () => ({parseDOM: [{tag: 'mark'}], toDOM: () => ['mark', {class: 'notes-highlight'}, 0], parseMarkdown: {match: n => n.type === 'highlight', runner: (state, n, type) => { state.openMark(type); state.next(n.children); state.closeMark(type); }}, toMarkdown: {match: m => m.type.name === 'highlight', runner: (state, m) => { state.withMark(m, 'highlight'); }}}))`
- `$view` for `wiki_link`: inline-block chip `<span class="notes-wikilink" data-wiki-link=...>` showing
  `alias || target + (subpath ? ' › ' + subpath : '')` (or the raw value when target and alias are empty);
  `data-unresolved` when `options.resolve?.(value)?.exists === false`; embeds render
  `<span class="notes-embed" data-embed>` and call `options.embed?.(value, element)` (fallback: chip text
  `![[value]]`). `mousedown` + `click`: `event.preventDefault()`, `options.open?.(value, event)`.
  `mouseenter` starts a 450 ms timer calling `options.preview?.(value, dom)`; `mouseleave` clears it and calls
  `options.preview?.(null, dom)`. `ignoreMutation: () => true`, `stopEvent: e => e.type.startsWith('mouse') || e.type === 'click'`,
  `update(next)` re-renders when the type matches.
- `$view` for `hashtag`: `<span class="notes-hashtag" data-hashtag=value>#value</span>`, click → `options.tag?.(value, event)`.
- `$view` for `callout_marker`: `<span class="notes-callout-marker" data-callout=value>` with
  `options.icon?.(iconForCallout(value)) || ''` + `<span>` label `options.label?.(value) || value` +
  fold sign if any. `iconForCallout` maps note/info/todo→info, tip/hint/important→star, success/check/done→check,
  question/help/faq→help-circle, warning/caution/attention→alert-triangle, failure/fail/missing/danger/error/bug→alert-octagon,
  example→list, quote/cite→format-quote, abstract/summary/tldr→file-text, default→info.
- Callout decoration plugin `$prose(() => new Plugin({key: new PluginKey('notesCallouts'), props: {decorations(state) {...}}}))`:
  for each `blockquote` node whose first child is a paragraph whose first child is `callout_marker`, add
  `Decoration.node(pos, pos + node.nodeSize, {class: 'notes-callout', 'data-callout': canonical(type)})` where
  `canonical` maps aliases to one of note, info, tip, success, question, warning, danger, bug, example, quote,
  abstract, todo, failure.
- Input rules:
  - `/(!?)\[\[([^\[\]\n]{1,500})\]\]$/` → replace match with `wiki_link {value: m[2], embed: m[1] === '!'}`.
  - `/(^|\s|\()#([\p{L}\p{N}_\-\/]*[\p{L}_\-\/][\p{L}\p{N}_\-\/]*)\s$/u` → keep the prefix char, insert a
    `hashtag` node and a trailing space.
  - `/(?:^|[^=])==([^=\s](?:[^=]*[^=\s])?)==$/` → replace with highlighted text (mark).
  - `/^\[!([A-Za-z0-9_-]+)\]([+-]?)\s$/` inside a blockquote's first paragraph → `callout_marker` + space.
- `obsidianCommands.toggleHighlight` = `toggleMark(state.schema.marks.highlight)`;
  `insertWiki(view, value, embed)` replaces the selection with the node and a following space;
  `insertCallout` is done by the host via markdown insert (see Task 1.3).

- [ ] **Step 2: Export from the engine entry**

Append to `scripts/notes-engine-entry.js`:

```js
export { obsidian, obsidianCommands, parseWikiValue } from './notes-obsidian.js';
```

- [ ] **Step 3: Rebuild the vendor bundle**

Run: `npm run build:notes-vendor`
Expected: `Notes vendor built: 4 assets, <n> permissive packages`, and `git diff --stat ui/js/vendor/notes`
shows engine.js + manifest.json changed. Then `node scripts/build-notes-vendor.js --check` prints
`Notes vendor verified`.

- [ ] **Step 4: Smoke the export in node**

Run: `node -e "import('./ui/js/vendor/notes/engine.js').then(m=>console.log(typeof m.obsidian, typeof m.parseWikiValue)).catch(e=>console.log('dom-needed', e.message))"`
Expected: `function function`, or `dom-needed …` if the bundle touches `document` at import time (acceptable; the
browser check in Task 1.3 covers it).

- [ ] **Step 5: Commit**

```bash
git add scripts/notes-obsidian.js scripts/notes-engine-entry.js ui/js/vendor/notes
git commit -m "feat(notes): bundle Obsidian syntax plugins into the editor engine" -- scripts/notes-obsidian.js scripts/notes-engine-entry.js ui/js/vendor/notes
```

### Task 1.3: Editor integration, styles and browser round trip

**Files:**
- Modify: `ui/js/desktop/apps/notes-editor.js:11-16` (`needsSource`), `:17-37` (builder), `:129-146` (`command`), `:201-205` (API)
- Modify: `ui/js/desktop/apps/notes.js:137-138` (editor options), `:243-248` (`openLink` neighbour)
- Modify: `ui/css/desktop-app-notes.css` (append)
- Modify: `ui/testdata/notes-functional.js` (new block before `check(errors.length===0,…)`)

- [ ] **Step 1: Extend the browser functional test (fails first)**

Insert before the final `check(errors.length===0,…)` in `ui/testdata/notes-functional.js`:

```js
    const obsidianNote='# Obsidian\n\nVerweis auf [[Ideen|meine Ideen]] und #projekt/alpha.\n\n> [!info] Hinweis\n> Siehe ==wichtig== und ![[skizze.png]]\n\n- [ ] Aufgabe mit [[Aufgaben#Heute]]\n';
    const obsHost=document.createElement('div');document.body.append(obsHost);
    const obsEditor=await NotesEditor.create(obsHost,{content:obsidianNote,readonly:false,tr:key=>key,changed:()=>{},resolveWiki:()=>({exists:false})});
    check(!obsEditor.sourceMode,'Obsidian syntax forced source mode');
    check(obsHost.querySelectorAll('.notes-wikilink').length===2&&obsHost.querySelector('.notes-hashtag')&&obsHost.querySelector('.notes-callout[data-callout=info]')&&obsHost.querySelector('mark.notes-highlight')&&obsHost.querySelector('.notes-embed'),'Obsidian syntax not rendered: '+obsHost.innerHTML.slice(0,400));
    obsEditor.view.dispatch(obsEditor.view.state.tr.insertText('!',obsEditor.view.state.doc.child(0).nodeSize-1));
    check(obsEditor.content()===obsidianNote.replace('# Obsidian','# Obsidian!'),'Obsidian round trip changed text: '+JSON.stringify(obsEditor.content()));
    obsEditor.command('highlight');obsEditor.insertWiki('Neu');
    check(obsEditor.content().includes('[[Neu]]'),'insertWiki failed');
    await obsEditor.destroy();obsHost.remove();
```

Run: `go test ./ui -run '^TestDesktopNotesAppBrowser$' -count=1 -v` with the env var that
`requirePrecisionBrowserSmoke` (in `ui/precision_workspace_browser_test.go`) requires.
Expected: FAIL with `Obsidian syntax forced source mode`.

If, after the integration, the round-trip check fails only because Milkdown canonicalises a non-Obsidian part
of `obsidianNote` (e.g. list marker or blank lines), change that part of the fixture to Milkdown's canonical
form and say so in the report. Any difference inside an Obsidian construct is a real bug.

- [ ] **Step 2: Integrate the plugins in `notes-editor.js`**

- `needsSource`: remove `|\[\[[^\]]+\]\]|!\[\[` from the regex (keep HTML, `:::`, footnotes, `$$`).
- Before `await crepe.create()`:

```js
        crepe.editor.use(lib.obsidian({
            resolve: value => options.resolveWiki?.(value) || null,
            open: (value, event) => options.openWiki?.(value, event),
            embed: (value, element) => options.embedWiki ? options.embedWiki(value, element) : (element.textContent = '![[' + value + ']]'),
            preview: (value, anchor) => options.previewWiki?.(value, anchor),
            tag: (value, event) => options.tagClick?.(value, event),
            icon: name => options.icon?.(name) || '',
            label: type => options.tr('callout_' + type) !== 'callout_' + type ? options.tr('callout_' + type) : type.charAt(0).toUpperCase() + type.slice(1)
        }));
```

- `command(name)`: add `else if(name==='highlight')action=lib.obsidianCommands.toggleHighlight;` (source mode:
  wrap the selection with `==`: `replaceSelection('=='+selected.text+'==')`).
- New API functions:

```js
        function insertWiki(value, embed=false){
            if(options.readonly)return;
            if(sourceMode){replaceSelection((embed?'!':'')+'[['+value+']]');return;}
            lib.obsidianCommands.insertWiki(view,value,embed);focus();
        }
        function insertCallout(type,title=''){insert('> [!'+type+']'+(title?' '+title:'')+'\n> \n');}
        function refreshDecorations(){view.dispatch(view.state.tr.setMeta('notesRefresh',true));}
```

  and return them: `{..., insertWiki, insertCallout, refreshDecorations, ...}`. (`refreshDecorations` lets the
  host re-render chips after the index changes; node views re-check `resolve` when their `update` runs, so
  also re-create chips by `view.setProps({})` if needed.)

- [ ] **Step 3: Phase-1 callbacks in `notes.js`**

In the `NotesEditor.create(...)` options add:

```js
                    resolveWiki:value=>resolveWikiSimple(value),openWiki:value=>openWikiSimple(value),
                    tagClick:tag=>{find('[data-tag]').value=tag;page=0;refreshList().catch(fail);},icon:name=>ctx.iconMarkup?.(name,'','notes-icon',16,'action')||'',
```

and next to `openLink`:

```js
        // Phase 1 only; Phase 2 replaces these with NotesIndex.
        function parseWikiValue(value){
            const raw=String(value||''),cut=raw.search(/(?<!\\)\|/),left=(cut<0?raw:raw.slice(0,cut)).replace(/\\\|/g,'|'),hash=left.indexOf('#');
            return {target:(hash<0?left:left.slice(0,hash)).trim(),subpath:hash<0?'':left.slice(hash+1).trim(),alias:cut<0?'':raw.slice(cut+1).trim()};
        }
        function wikiCandidates(value){
            const name=parseWikiValue(value).target.split('/').pop().replace(/\.md$/i,'').toLowerCase();
            return name?list.filter(note=>basename(note.path).replace(/\.md$/i,'').toLowerCase()===name||note.title.toLowerCase()===name):[];
        }
        function resolveWikiSimple(value){return {exists:wikiCandidates(value).length>0};}
        function openWikiSimple(value){const hit=wikiCandidates(value)[0];if(hit)loadNote(hit.path);}
```

- [ ] **Step 4: Styles (append to `ui/css/desktop-app-notes.css`)**

```css
.notes-wikilink{display:inline;padding:1px 3px;border-radius:4px;color:var(--notes-accent);cursor:pointer;text-decoration:underline;text-decoration-color:color-mix(in srgb,var(--notes-accent) 45%,transparent);text-underline-offset:3px}
.notes-wikilink:hover{background:color-mix(in srgb,var(--notes-accent) 12%,transparent)}
.notes-wikilink[data-unresolved]{color:var(--notes-muted);text-decoration-style:dashed}
.notes-hashtag{display:inline-block;padding:0 7px;border-radius:999px;font-size:.88em;line-height:1.6;background:color-mix(in srgb,var(--notes-accent) 13%,transparent);color:color-mix(in srgb,var(--notes-accent) 85%,var(--notes-ink));cursor:pointer}
mark.notes-highlight{background:color-mix(in srgb,#f5d547 42%,transparent);color:inherit;border-radius:3px;padding:0 2px}
.notes-rich .milkdown .ProseMirror blockquote.notes-callout{--callout:#5b8def;border-left:3px solid var(--callout);background:color-mix(in srgb,var(--callout) 10%,transparent);color:var(--notes-ink);padding:10px 16px}
.notes-callout[data-callout=tip],.notes-callout[data-callout=success]{--callout:#3fb37f}
.notes-callout[data-callout=question],.notes-callout[data-callout=todo]{--callout:#d9a520}
.notes-callout[data-callout=warning]{--callout:#e8892f}
.notes-callout[data-callout=danger],.notes-callout[data-callout=bug],.notes-callout[data-callout=failure]{--callout:#e05252}
.notes-callout[data-callout=example]{--callout:#9b6ee8}
.notes-callout[data-callout=quote]{--callout:#8a96a8}
.notes-callout[data-callout=abstract]{--callout:#2fb3c4}
.notes-callout-marker{display:inline-flex;align-items:center;gap:6px;font-weight:650;color:var(--callout,var(--notes-accent));margin-right:4px}
.notes-callout-marker .notes-icon{width:16px;height:16px}
.notes-embed{display:block;margin:10px 0;border:1px solid var(--notes-border);border-radius:8px;padding:10px 12px;background:var(--notes-surface-low)}
.notes-embed img{display:block;max-width:100%;border-radius:6px}
```

- [ ] **Step 5: Run the browser test to verify it passes**

Run: `go test ./ui -run '^TestDesktopNotesAppBrowser$' -count=1 -v -timeout 10m` (same env var).
Expected: PASS; log shows `Notes workflows: {...}` without errors. Also run
`node --test scripts/notes-obsidian-markdown.test.mjs ui/js/desktop/apps/notes-links.test.cjs` → PASS.

- [ ] **Step 6: Commit**

```bash
git commit -m "feat(notes): edit wikilinks, tags, callouts and highlights in the rich editor" -- ui/js/desktop/apps/notes-editor.js ui/js/desktop/apps/notes.js ui/css/desktop-app-notes.css ui/testdata/notes-functional.js
```

---

## Phase 2 — Notes index, inline tags, folders, link updates

### File structure

- Create `internal/desktop/notes_markdown.go` — `ExtractNoteRefs`, `NoteRefs`, `NoteRefLink`, `ParseWikiValue`,
  `RewriteNoteLinks`.
- Create `internal/desktop/notes_resolve.go` — `noteLookup`, `newNoteLookup`, `(*noteLookup).Resolve`.
- Create `internal/desktop/service_notes_index.go` — `NotesIndex`, index types, per-file cache, `MoveNoteWithLinks`.
- Create `internal/desktop/notes_markdown_test.go`, `notes_resolve_test.go`, `service_notes_index_test.go`.
- Create `ui/testdata/notes-links-fixture.json` — shared extraction/resolution cases (Go + JS).
- Modify `internal/desktop/service_notes.go` — `noteSummary` uses `ExtractNoteRefs` for tags.
- Modify `internal/server/desktop_notes_handlers.go` + `desktop_notes_handlers_test.go` — `index=1`, `mkdir`,
  `update_links`.
- Create `ui/js/desktop/apps/notes-index.js` + `notes-index.test.cjs` — client index (pure helpers node-tested
  against the shared fixture).
- Modify `ui/js/desktop/core/module-loader.js` (notes scripts) and the fixture HTML in
  `ui/desktop_notes_browser_test.go` to load `notes-index.js` before `notes.js`.

### Task 2.1: Shared fixture + Go extractor/resolver

**Files:**
- Create: `ui/testdata/notes-links-fixture.json`, `internal/desktop/notes_markdown.go`, `internal/desktop/notes_resolve.go`
- Test: `internal/desktop/notes_markdown_test.go`, `internal/desktop/notes_resolve_test.go`

- [ ] **Step 1: Write the shared fixture**

```json
{
  "extract": [
    {"name": "frontmatter and inline", "content": "---\ntitle: Projekt Alpha\naliases: [Alpha, \"P-A\"]\ntags: [Work]\n---\n# Heading\nText #todo and #work [[Ideen|meine]] ![[bild.png]] [md](sub/Other.md#x) [web](https://x.y/a.md)\n```\n[[code]] #code\n```\n`[[inline]]` #fine/sub\n",
     "title": "Projekt Alpha", "aliases": ["Alpha", "P-A"], "tags": ["work", "todo", "fine/sub"],
     "links": [{"kind": "wiki", "value": "Ideen|meine", "embed": false}, {"kind": "wiki", "value": "bild.png", "embed": true}, {"kind": "markdown", "value": "sub/Other.md#x", "embed": false}]},
    {"name": "heading title", "content": "Intro\n\n# Real Title\n", "title": "Real Title", "aliases": [], "tags": [], "links": []},
    {"name": "digits are not tags", "content": "Issue #123 and #12a, a#b\n", "title": "Issue #123 and #12a, a#b", "aliases": [], "tags": ["12a"], "links": []}
  ],
  "resolve": {
    "notes": ["Documents/Notes/Ideen.md", "Documents/Notes/Projekte/Ideen.md", "Documents/Notes/Projekte/Plan.md", "Documents/Notes/Archiv/2025/Plan.md", "Documents/Notes/Daily/2026-11-23.md", "Documents/Notes/sub/Other.md"],
    "aliases": {"Documents/Notes/Projekte/Plan.md": ["Masterplan"]},
    "attachments": ["Documents/Notes/.attachments/bild.png", "Documents/Notes/Projekte/skizze.png"],
    "cases": [
      {"from": "Documents/Notes/Projekte/Plan.md", "value": "Ideen", "path": "Documents/Notes/Projekte/Ideen.md"},
      {"from": "Documents/Notes/Daily/2026-11-23.md", "value": "Ideen", "path": "Documents/Notes/Ideen.md"},
      {"from": "Documents/Notes/Daily/2026-11-23.md", "value": "Plan", "path": "Documents/Notes/Projekte/Plan.md"},
      {"from": "Documents/Notes/Ideen.md", "value": "Archiv/2025/Plan#Ziele|alt", "path": "Documents/Notes/Archiv/2025/Plan.md"},
      {"from": "Documents/Notes/Ideen.md", "value": "2025/Plan", "path": "Documents/Notes/Archiv/2025/Plan.md"},
      {"from": "Documents/Notes/Ideen.md", "value": "masterplan", "path": "Documents/Notes/Projekte/Plan.md"},
      {"from": "Documents/Notes/Ideen.md", "value": "#Local", "path": "Documents/Notes/Ideen.md"},
      {"from": "Documents/Notes/Ideen.md", "value": "Fehlt", "path": ""},
      {"from": "Documents/Notes/Ideen.md", "value": "bild.png", "embed": true, "path": "Documents/Notes/.attachments/bild.png"},
      {"from": "Documents/Notes/Projekte/Plan.md", "value": "skizze.png", "embed": true, "path": "Documents/Notes/Projekte/skizze.png"},
      {"from": "Documents/Notes/Ideen.md", "kind": "markdown", "value": "sub/Other.md#x", "path": "Documents/Notes/sub/Other.md"},
      {"from": "Documents/Notes/Ideen.md", "kind": "markdown", "value": "../../outside.md", "path": ""},
      {"from": "Documents/Notes/Ideen.md", "kind": "markdown", "value": "https://x.y/a.md", "path": ""}
    ]
  }
}
```

- [ ] **Step 2: Write failing Go tests** — `notes_markdown_test.go` loads `../../ui/testdata/notes-links-fixture.json`,
  runs `ExtractNoteRefs(content)` per `extract` case and compares `Title`, `Aliases`, `Tags`, and
  `[(Kind, Value, Embed)]` of `Links`; plus a `RewriteNoteLinks` test:

```go
func TestRewriteNoteLinksKeepsAliasAndCode(t *testing.T) {
	in := "See [[Old|shown]] and [[Old#Part]] and [x](Old.md#y)\n```\n[[Old]]\n```\n"
	out := RewriteNoteLinks(in, func(link NoteRefLink) (string, bool) {
		if link.Kind == "wiki" {
			v := ParseWikiValue(link.Value)
			if v.Target == "Old" { return ReplaceWikiTarget(link.Value, "New"), true }
		}
		if link.Kind == "markdown" && strings.HasPrefix(link.Value, "Old.md") { return "New.md" + strings.TrimPrefix(link.Value, "Old.md"), true }
		return "", false
	})
	want := "See [[New|shown]] and [[New#Part]] and [x](New.md#y)\n```\n[[Old]]\n```\n"
	if out != want { t.Fatalf("got %q", out) }
}
```

  `notes_resolve_test.go` builds `newNoteLookup(notes, aliases, attachments)` from the fixture and checks
  `Resolve(from, kind, value, embed)` for every case.

  Run: `go test ./internal/desktop -run '(?i)(ExtractNoteRefs|RewriteNoteLinks|NoteLookup)' -count=1`
  Expected: FAIL (undefined symbols).

- [ ] **Step 3: Implement** (Go, `package desktop`):

```go
type NoteRefLink struct {
	Kind  string // "wiki" or "markdown"
	Value string // wiki inner text or markdown URL
	Embed bool
	Start int // byte offset of Value in content
	End   int
}
type NoteRefs struct {
	Title   string
	Aliases []string
	Tags    []string // lowercased, de-duplicated, frontmatter first then inline order
	Links   []NoteRefLink
}
type WikiValue struct{ Target, Subpath, Alias string }

func ExtractNoteRefs(content string) NoteRefs
func ParseWikiValue(value string) WikiValue
func ReplaceWikiTarget(value, target string) string // keeps "#sub" and "|alias" text exactly
func RewriteNoteLinks(content string, rewrite func(NoteRefLink) (string, bool)) string
```

  Rules: frontmatter parsed with `yaml.v3` (title, aliases/alias as string or list, tags as string or list);
  title fallback = first `# ` heading else first non-empty body line (trimmed, max 160 runes) — keep
  `noteSummary`'s existing behaviour for the file-name fallback; body scanning skips fenced code (``` / ~~~
  fences with matching length), inline code spans (backtick runs), and HTML comments; wikilinks `!?\[\[...\]\]`
  per the Phase-1 grammar; inline tags per the Phase-1 hashtag rule; markdown links `](url)` /
  `](<url>)` whose URL has no scheme, does not start with `/` or `#`, and ends in `.md` (optionally with
  `#frag`) or a known attachment extension (png, jpg, jpeg, gif, webp, svg, pdf).

  `notes_resolve.go`:

```go
type noteLookup struct { /* lowercased path -> path, basename -> []path, alias -> []path, attachments */ }
func newNoteLookup(notes []string, aliases map[string][]string, attachments []string) *noteLookup
func (l *noteLookup) Resolve(from, kind, value string, embed bool) string // "" when unresolved
func (l *noteLookup) LinkText(path string) string // basename w/o .md if unique, else Notes-relative path w/o .md
```

  Resolution exactly as in the spec, section 2 (empty target → `from`; with `/`: Notes-root exact, then
  relative to `from`'s folder, then shortest suffix match; plain: basename → same folder → shortest →
  alphabetical; alias; non-`.md` extension → attachments with the same steps; markdown: relative, decoded,
  must stay under `Documents/Notes/`).

- [ ] **Step 4: Run tests** — same command, expected PASS.

- [ ] **Step 5: Commit** — `git add` the five files, commit `feat(notes): extract and resolve Obsidian links on the server`.

### Task 2.2: Index service, inline tags, move with link updates

**Files:**
- Create: `internal/desktop/service_notes_index.go`, `internal/desktop/service_notes_index_test.go`
- Modify: `internal/desktop/service_notes.go:132-180` (`noteSummary`)

- [ ] **Step 1: Failing tests** (`service_notes_index_test.go`, reuse the service test helper used by
  `internal/desktop/service_notes_test.go` to create a temp workspace service):
  - write `Documents/Notes/a.md` (`# A\n[[b]] #tag`), `Documents/Notes/sub/b.md` (`# B`),
    `Documents/Notes/.attachments/x.png`; `NotesIndex(ctx)` returns both notes, `a` has link
    `{Target:"b", Path:"Documents/Notes/sub/b.md"}` and tag `tag`, attachments contain `x.png`, folders contain
    `Documents/Notes/sub`.
  - second call after touching nothing reuses cache (expose `s.notesIndexCacheHits` counter for tests).
  - `SearchNotes(NotesQuery{Tag:"tag"})` finds `a.md` (inline tag).
  - `MoveNoteWithLinks(ctx, "Documents/Notes/sub/b.md", "Documents/Notes/c.md", version, SourceUser)` moves
    the file and rewrites `a.md` to `[[c]]`, returns `updated == 1`; a referencing note changed concurrently
    (simulate by a precondition func that fails) lands in `failed`.
  Run: `go test ./internal/desktop -run '(?i)NotesIndex|MoveNoteWithLinks|InlineTag' -count=1` → FAIL.

- [ ] **Step 2: Implement**

```go
type NoteIndexLink struct {
	Target string `json:"target"` // raw value
	Path   string `json:"path"`   // resolved path or ""
	Embed  bool   `json:"embed,omitempty"`
	Kind   string `json:"kind"`
}
type NoteIndexEntry struct {
	Path     string          `json:"path"`
	Title    string          `json:"title"`
	Aliases  []string        `json:"aliases"`
	Tags     []string        `json:"tags"`
	Links    []NoteIndexLink `json:"links"`
	Modified time.Time       `json:"modified"`
	Size     int64           `json:"size"`
}
type NotesIndexResult struct {
	Notes       []NoteIndexEntry `json:"notes"`
	Folders     []string         `json:"folders"`
	Attachments []string         `json:"attachments"`
	Skipped     int              `json:"skipped"`
}
func (s *Service) NotesIndex(ctx context.Context) (NotesIndexResult, error)
func (s *Service) MoveNoteWithLinks(ctx context.Context, from, to, expected, source string) (updated int, failed []string, err error)
```

  Cache: `notesIndexMu sync.Mutex`, `notesIndexCache map[string]notesIndexCacheEntry{mod time.Time; size int64; refs NoteRefs}` on
  `Service` (add the fields to the `Service` struct definition; check impact first). The walk mirrors
  `SearchNotes` (same root, symlink skip, `.attachments` files go to `Attachments`, other non-`.md` files under
  Notes also count as attachments). Drop cache entries not seen in the walk.
  `MoveNoteWithLinks`: build the index, collect notes whose links resolve to `from` (excluding `from`), call
  `MovePathConditional(from, to, CheckNoteVersion(expected,false))`, rebuild a lookup with `to`, then for each
  referencing note read bytes + version, `RewriteNoteLinks` (wiki: `ReplaceWikiTarget(value, lookup.LinkText(to))`
  keeping a `.md` suffix if the old target had one; markdown: relative path from the referencing note to `to`
  with URL-escaped segments + old `#frag`), write with `WriteFileBytesConditional(..., CheckNoteVersion(version,false))`;
  errors append to `failed`.
  `noteSummary`: replace the hand-rolled frontmatter/tag parse with `ExtractNoteRefs` (keep `Title` fallback to
  the file name when refs give none, keep `Snippet` behaviour).

- [ ] **Step 3: Run tests** — expected PASS; also `go test ./internal/desktop -count=1` (whole package) PASS.

- [ ] **Step 4: Commit** — `feat(notes): cached notes index, inline tags and link-preserving moves`.

### Task 2.3: HTTP handler (index, mkdir, update_links)

**Files:** Modify `internal/server/desktop_notes_handlers.go`, `internal/server/desktop_notes_handlers_test.go`.

- [ ] **Step 1: Failing handler tests** (follow the existing helper in `desktop_notes_handlers_test.go`):
  `GET ?index=1` returns JSON with `notes`; `POST {"operation":"mkdir","folder":"Documents/Notes/Neu"}` → 200
  and the folder exists; `POST mkdir` with `Documents/Notes/../x`, `Documents/Other`,
  `Documents/Notes/.attachments/x` → 400; `PATCH {"operation":"move","new_path":...,"update_links":true}` with
  `If-Match` → 200 and body has `"updated_links":1`.
  Run: `go test ./internal/server -run '(?i)DesktopNotes' -count=1` → FAIL.
- [ ] **Step 2: Implement** — in the GET branch, `if r.URL.Query().Get("index") == "1"` → `svc.NotesIndex`;
  extend the body struct with `UpdateLinks bool \`json:"update_links"\``; POST with `body.Operation == "mkdir"`
  validates `folder` (clean, prefix `NotesDirectory+"/"`, no `.attachments` segment) → `svc.CreateDirectory(ctx, folder, desktop.SourceUser)`,
  publish `create_directory`, respond `{"path": folder}`; PATCH `move` with `UpdateLinks` calls
  `svc.MoveNoteWithLinks` instead of `MovePathConditional` and encodes
  `struct{desktop.Note; UpdatedLinks int \`json:"updated_links"\`; LinkUpdateFailed []string \`json:"link_update_failed,omitempty"\`}`,
  publishing `write_file` for each rewritten note path (return the paths from `MoveNoteWithLinks` if needed).
- [ ] **Step 3: Run tests** — PASS; `go vet ./internal/server ./internal/desktop` clean.
- [ ] **Step 4: Commit** — `feat(notes): index endpoint, folder creation and link updates on move`.

### Task 2.4: Client index module

**Files:** Create `ui/js/desktop/apps/notes-index.js`, `ui/js/desktop/apps/notes-index.test.cjs`; modify
`ui/js/desktop/core/module-loader.js` (notes `scripts`: insert `'/js/desktop/apps/notes-index.js'` before
`notes-editor.js`), `ui/desktop_notes_browser_test.go` (fixture `<script>` list and the mock `/api/desktop/notes`
handler: `index=1` returns `{notes:[{path,title,aliases:[],tags,links:[],modified,size}],folders:[],attachments:[]}`).

`window.NotesIndex` API (UMD-style so node can `require` it: `(function(root,factory){...})(this,...)` exporting
via `module.exports` when defined):

```js
NotesIndex.parseWikiValue(value) -> {target, subpath, alias}
NotesIndex.extract(content) -> {title, aliases, tags, links:[{kind, value, embed}]}   // same rules as Go
NotesIndex.createLookup({notes, aliases, attachments}) -> {resolve(from, kind, value, embed) -> path|'' , linkText(path) -> string}
NotesIndex.fuzzy(query, text) -> score (0 = no match; higher is better; consecutive + word-start bonuses)
NotesIndex.create({request, onChange}) -> store {
  load(), refresh() /* debounced 400ms */, data /* last index */, resolve(from, value, embed=false, kind='wiki'),
  backlinks(path) -> [{path, title, count}], outgoing(path, content) -> [{value, path, embed, kind}],
  tags() -> [{tag, count}], search(query, limit=50) -> [{path, title, aliases, score, matched}], linkText(path)
}
```

- [ ] **Step 1: Failing node test** — `notes-index.test.cjs` reads `ui/testdata/notes-links-fixture.json`,
  checks `extract` and `createLookup().resolve` for every case, plus
  `assert.ok(NotesIndex.fuzzy('kid','Kleine Ideen')>0)`, `assert.equal(NotesIndex.fuzzy('xyz','Kleine Ideen'),0)`,
  `assert.ok(NotesIndex.fuzzy('ide','Ideen')>NotesIndex.fuzzy('ide','Kleine Ideen'))`.
  Run: `node --test ui/js/desktop/apps/notes-index.test.cjs` → FAIL.
- [ ] **Step 2: Implement** the module mirroring the Go rules.
- [ ] **Step 3: Run** node test → PASS; run `go test ./ui -run '^TestDesktopNotesAppBrowser$'` → PASS (module
  loads, no errors).
- [ ] **Step 4: Replace Phase-1 helpers** in `notes.js`: `resolveWikiSimple`/`openWikiSimple`/local
  `parseWikiValue` now use a `NotesIndex.create({request:(url,o)=>request(url,o)})` store created in `render`,
  loaded in `instance.ready` and refreshed in `desktopEvent`.
- [ ] **Step 5: Commit** — `feat(notes): client notes index with shared link rules`.

---

## Phase 3 — Linking UI (expand before execution)

Files: create `ui/js/desktop/apps/notes-panels.js` (`window.NotesPanels`: `backlinks`, `outgoing`, `outline`,
`info` renderers returning HTML + event hooks); modify `notes.js`, `notes-editor.js`, CSS, functional test,
16 locale files.

Behaviour:
1. Wikilink click → `index.resolve(current.path, value)`; open, or create `<current folder>/<target>.md` with
   `# <target>\n\n` via `PUT If-None-Match:*` and open it. Embed rendering: images → `<img src=/api/desktop/download?inline=1&path=…>`;
   notes → card with title + `marked`+`DOMPurify` excerpt (first 4000 chars, frontmatter stripped) fetched
   lazily, cached per version; missing → muted chip.
2. Hover preview card (`[data-preview]`, absolute in the app root, max 380×300, closes on leave/Escape/scroll).
3. Rich `[[` autocomplete: in `notes-editor.js` track the text before the caret on every transaction
   (`/(!?)\[\[([^\[\]\n|#]*)(#[^\[\]\n|]*)?$/`); show `[data-suggest]` list at `view.coordsAtPos`; keys
   ↑/↓/Enter/Tab/Escape via `handleKeyDown`; Enter replaces `[[query` with a `wiki_link` node whose value is
   `index.linkText(path)` (+ `#heading` when chosen). Items from `options.suggest(query, heading)` supplied by
   `notes.js` (`index.search`, headings via a fetched note's `extract`). Source mode: CodeMirror
   `autocompletion({override:[source]})` with the same provider (completion inserts `value]]`).
4. History: `history=[], historyIndex` in `notes.js`; `loadNote(path,{fromHistory})`; buttons
   `[data-action=back|forward]` in the document bar; Alt+←/→.
5. Right sidebar: single toggle `[data-action=sidebarRight]`; tab strip `backlinks, outgoing, outline, localGraph
   (placeholder until Phase 5), info, assist`; selected tab in localStorage `aurago.notes.right`. Backlinks tab:
   linked (from `index.backlinks`, each with the line excerpt where the link occurs — fetched lazily) and
   "Unlinked mentions" (search API `q=<title>` minus linked ones). Outgoing tab: resolved + unresolved (click
   unresolved → create).
6. Rename/move uses `update_links:true`; notice `links_updated` with the count; `link_update_failed` → error notice.
7. Status bar `[data-backlinks]` button "n Rückverweise" opens the Backlinks tab.

Tests: functional script — click a wikilink opens target, back returns; unresolved click creates note;
backlinks tab lists the linking note; autocomplete inserts `[[Ideen]]` via keyboard; rename reports
`updated_links`. Mock backend in `desktop_notes_browser_test.go` gains `index=1` with real extraction via
`desktop.ExtractNoteRefs` and `PATCH move` with `update_links` (use `desktop.RewriteNoteLinks`).

## Phase 4 — Navigation (expand before execution)

Files: create `ui/js/desktop/apps/notes-sidebar.js` (`window.NotesSidebar.create(root, api)`),
`ui/js/desktop/apps/notes-palette.js` (`window.NotesPalette.open(root, {mode:'switcher'|'commands', items, onPick, onCreate, placeholder, tr})`);
modify `notes.js` (library markup replaced by the sidebar host), CSS, locales, module loader + fixture.

Behaviour:
1. Left sidebar tabs (Files, Search, Tags, Bookmarks), tab in localStorage `aurago.notes.left`.
2. Files: tree built from `index.data.notes` + `folders`; folders first, collapsible (expanded set in
   localStorage `aurago.notes.expanded`), rows 28px, active row highlighted, sort name/modified (meta `sort`);
   head buttons new note (in selected folder), new folder (`promptDialog` → POST mkdir); drag a note row onto a
   folder row or the root → PATCH move with `update_links`; right-click row → popup (open, rename, move,
   bookmark, trash); footer trash toggle shows the trash list (existing API with `trash=true`).
3. Search: input with `tag:x` / `path:Folder` tokens mapped to `tag` / `folder` params, remaining words as `q`;
   result cards with `<mark>` around query words in title/snippet; keyboard ↑/↓/Enter.
4. Tags: `index.tags()` as nested tree (`a/b`), counts; click → Search tab with `tag:a/b`.
5. Bookmarks: `meta.pinned` list; star button `[data-action=pin]` in the document bar toggles.
6. Quick switcher Ctrl+O: items = `index.search(query)`; empty query → `meta.recent` (maintained in
   `loadNote`, max 20); Enter open; Shift+Enter or the "create" row → create note named by the query in the
   current folder.
7. Command palette Ctrl+P: items from the same definitions as `menus()` (refactor `menus()` to build from a
   shared `commands()` list with id, label, shortcut, enabled); Enter runs `act(id)`.
8. Breadcrumb: `[data-crumbs]` showing folder segments (click → Files tab revealing the folder) › title button
   (existing document menu).
9. Inline find bar under the toolbar (`[data-findbar]`): find, replace, count, next, replace one/all, case,
   close; Ctrl+F opens and focuses; Escape closes. Remove the right `search` panel.
10. Shortcuts per the spec; Ctrl+P no longer prints; File menu keeps Print and Open file.

Tests: switcher opens a note by fuzzy query and creates one with Shift+Enter; palette runs "Neue Notiz"; tree
shows a folder and moving by simulated drop issues PATCH with `update_links`; tag click filters; find bar
replaces text.

## Phase 5 — Graph (expand before execution)

Files: create `ui/js/desktop/apps/notes-graph.js` (`window.NotesGraph.create(host, {getTheme, onOpen, tr})` →
`{setData({nodes:[{id,label,kind:'note'|'tag'|'unresolved',degree}], links:[{source,target}]}), focus(id), fit(), destroy()}`);
modify `notes.js` (graph view host `[data-graph]` in `.notes-main`, local graph tab), CSS, locales, loader + fixture.

Behaviour: Barnes-Hut quadtree repulsion (θ 0.9), link springs, centering, velocity decay 0.4, alpha decay to
stop; devicePixelRatio canvas; wheel zoom around the cursor, drag pan, node drag (fixes node while dragging),
hover highlights the node, its links and neighbours and dims others, labels shown when zoomed in or hovered;
click opens (`onOpen(id)`); theme colours read from computed CSS (`--notes-accent`, `--notes-muted`,
`--notes-ink`, `--notes-well`); reduced motion → run the simulation to rest before the first paint.
Global view controls: filter input, toggles (tags, unresolved, orphans), fit button, close (Esc). Local graph:
current note + neighbours to depth 1/2 (select), re-centres on note change.

Tests: graph opens with Ctrl+G, has N nodes for N notes (expose `instance.graph.nodeCount()` for tests),
clicking a node position from `instance.graph.screenPosition(id)` opens that note; local tab renders a canvas.

## Phase 6 — Writing (expand before execution)

Files: create `ui/js/desktop/apps/notes-templates.js` (`window.NotesTemplates`: `expand(text, {title, date, lang})`,
`formatDate(date, fmt, lang)`, `dailyPath(date)`, `DAILY_DIR`, `TEMPLATE_DIR`) + `notes-templates.test.cjs`;
modify `notes-frontmatter.js` (`updateList(content, key, values)`, `listValues(content, key)`, `fields(content)`),
`notes.js`, `notes-editor.js` (callout insert), CSS, locales, loader + fixture.

Behaviour: daily note button `[data-action=daily]` + command; daily navigation ‹ › between existing daily notes
(from the index, sorted by name); templates popup lists built-ins + `Templates/*.md`; "Insert template" inserts
the expanded body (frontmatter stripped) at the cursor; Insert menu → Callout submenu (types in the spec);
toolbar buttons highlight + wikilink (`insertWiki('')` opens the `[[` suggest); properties bar
`[data-properties]` above the editor: tag and alias chips (× removes, input adds on Enter/comma), other keys
read-only, collapsible; Info panel loses the tag input.

Tests: node test for `expand` (fixed date 2026-11-23 14:05, `de`: `{{date:dddd, D. MMMM YYYY}}` →
`Montag, 23. November 2026`, `{{date}}` → `2026-11-23`, `{{time}}` → `14:05`, `{{title}}`), `updateList` keeps
other keys; browser: daily button opens `Documents/Notes/Daily/<today>.md`, chip add writes `tags:`, callout
insert renders `.notes-callout`.

## Phase 7 — i18n, polish, tests, review (expand before execution)

- Fill any missing `desktop.notes_*` keys in all 16 locale files; `go test ./ui -run '(?i)i18n'` PASS.
- CSS pass for every theme (default, fruity dark/light, others in `ui/css/desktop-themes*`), compact density,
  narrow (< 900px overlays, < 600px container query), touch sizes, focus rings, reduced motion.
- Screenshot loop in `TestDesktopNotesAppBrowser`: add graph + switcher + sidebar-tab scenes; inspect the PNGs.
- `node --test scripts/notes-obsidian-markdown.test.mjs ui/js/desktop/apps/notes-*.test.cjs`,
  `go test ./internal/desktop ./internal/server -run '(?i)notes' -count=1`, `go test ./ui -run '^TestDesktopNotes' -count=1`.
- One combined code review of the whole round (Critical/Important only), fix, final commit; update memory.
