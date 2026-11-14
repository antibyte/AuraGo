# Writer Wave 2a — page setup, paragraph dialog, page fields: implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A Page setup dialog and a Paragraph dialog, all four page fields and today's date for headers and footers, and one length unit (centimetres, or inches in a US browser) in the dialogs and the Format panel.

**Architecture:** One new pure module, `writer-layout.js`, holds the length unit, the paper and margin catalogue, and the model of both dialogs: fields from the engine's state, dependent fields, and one command that carries only what the user changed. The shared form dialog gains headings, half-width fields, a unit suffix, mixed checkboxes and a hook for dependent fields. `writer-dialogs.js` builds the two dialogs from the model; the registry, the menus, the right-click menu, the header/footer bar and the Format panel offer them.

**Tech stack:** Vanilla JS classic scripts (no bundler for the app files; `module-loader.js` is part of `main.bundle.js`), `@docx-editor.dev/core` 2.29.0 behind `lib`, node `vm` tests, Go + rod browser tests.

Roadmap: `docs/superpowers/specs/2026-11-12-writer-office-roadmap-design.md`, section "Wave 2 — Business documents". This is part 2a of three; 2b (PDF and templates) and 2c (table, image, styles gallery) follow.

---

## Read this first

**Decided by the user (2026-11-13):** Wave 2 starts with "page and paragraph". Tab stops are not part of the Paragraph dialog; they come with the interactive ruler in Wave 3.

**Scope.** In: Page setup dialog (six papers, custom size, orientation, margin presets, whole document or this section); Paragraph dialog (alignment, left and right indent, first-line or hanging indent, spacing before and after, line spacing as single, 1.15, 1.5, double, at least, exactly or multiple, keep with next, keep lines together, widow control, page break before); the page fields PAGE, NUMPAGES, SECTIONPAGES and PAGE X OF Y and today's date as text; lengths in the Format panel and both dialogs in one unit. Out: tab stops; header and footer distance from the page edge (the engine exposes it only while a header or footer is open); the ruler, which keeps its centimetre scale; paragraph direction; "no space between paragraphs of the same style"; columns.

**Engine behaviour, probed in the browser on 2026-11-13.** These are facts, not assumptions:

- `editor.getPageSetup()` returns `{pageWidthTwips, pageHeightTwips, orientation, marginsTwips: {top, right, bottom, left}, gutterTwips}` for the section the caret is in.
- `{type:'setPageSetup', pageWidth, pageHeight}` accepts a custom size (A5 was 8391 x 11906). `orientation` alone swaps width and height. Width, height and `orientation: 'landscape'` together end as landscape whichever way round the two numbers were sent.
- `scope: 'section'` changes only the caret's section; the default `'document'` changes every section.
- Margins that leave no room are refused when the command runs (`ok: false`, reason `invalid-property-value`); `editor.can` still says yes.
- `{type:'setParagraphFormat', ...}` sets alignment, `spaceBeforePt`, `spaceAfterPt`, `lineSpacing {rule, value}`, `indentLeftTwips`, `indentRightTwips`, `indentFirstLineTwips` (negative is a hanging indent), `keepNext`, `keepLines`, `widowControl`, `pageBreakBefore` in ONE undo step. `null` for a length, a spacing or the line spacing hands the value back to the paragraph style.
- `snapshot().formatting` reads them back: `alignment` (`'both'` is justify), `lineSpacing {rule: 'multiple'|'exact'|'atLeast', value}` (lines for `multiple`, points otherwise), `spaceBeforePt`, `spaceAfterPt` (absent when no level states one), `paragraphFlags {keepNext, keepLines, widowControl, pageBreakBefore, contextualSpacing}` (`null` when the selection disagrees), `indent {left, right, firstLine, mixed: {left, right, firstLine}}`, `disagrees {alignment, spaceBeforePt, spaceAfterPt, lineSpacing, ...}`.
- In suggesting mode the engine accepts `setParagraphFormat` and `setPageSetup`. Writer's own rule (`UNTRACKABLE` in `writer-commands.js`) already refuses `setPageSetup` there; the Paragraph dialog stays available, like the paragraph fields of the Format panel.
- `{type:'insertPageField', field}` works for `PAGE`, `NUMPAGES`, `SECTIONPAGES` and `PAGE_X_OF_Y`, only while a header or footer is open. `setAlignment`, `setParagraphFormat` and `setPageSetup` work inside a header or footer.

**Units.** The fixture pages have `<html lang="en">` and headless Chrome reports `navigator.language` `en-US`, so the browser tests see inches. The node tests cover centimetres. Never write a length into a browser check as a literal text: compute it with `WriterLayout.units(document.documentElement.lang, navigator.language)`.

**Repo rules:**

- Read the `AGENTS.md` chain for every path you touch before editing (root, `ui/AGENTS.md`, `ui/js/desktop/apps/AGENTS.md`, `scripts/AGENTS.md`); close out with a DOX pass (Task 6).
- Every user-visible string exists in all 16 files under `ui/lang/desktop/`. `scripts/test-writer-commands.mjs` scans every registry label and every literal `tr('key')` in `ui/js/desktop/apps/writer*.js`; write `tr('key')` with a literal key.
- Every file under `ui/js/desktop` stays below 1100 lines. No Internet assets, no new dependencies.
- `ui/desktop_office_app_markers_test.go` pins these strings in `writer.js`: `window.WriterApp={render,dispose,instances}`, `queue?.dispose()`, `dispose(windowId);`, `editor?.destroy()`, `/api/desktop/office/document`; and in `writer-menus.js`: `item('new')`, `item('saveAs')`, `id:'agent'`, `labelKey:'desktop.menu_agent'`, `item('assist')`, `setWindowMenus`.
- A command is one entry in `DEFINITIONS` in `writer-commands.js`; `scripts/test-writer-menus.mjs` fails for a command no window menu lists. An app action also needs its `case` in `act` in `writer.js`.
- `ui/js/desktop/shared/form-dialog.js` is shared; only add to it, and keep every existing dialog working (table, link, go to page, word count, shortcuts, drafts).
- Temporary scripts go into `disposable/_writer-wave2a/` and are deleted before the commit. Before each commit: `git diff --cached --check`, and look at `git diff --cached`.
- GitNexus: the index is hundreds of commits behind and cannot index worktrees. Controller's impact analysis for `writer.js` and `writer-panels.js`: UNKNOWN, callers confirmed by text search (`window.WriterApp` is used by the shell loader and the Go tests; `window.WriterPanels.paperSize` only by `scripts/test-writer-session.mjs`). End each commit body with: `GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.`

**Environment (Windows):**

- Work in the worktree `.claude/worktrees/writer-wave2a` with a real copy of `node_modules` (`robocopy <main>\node_modules <worktree>\node_modules /E /MT:16 /NFL /NDL /NJH /NP`).
- In a worktree session git commands must be plain and single (no `&&`, no `cd`).
- Browser tests: `AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run "TestDesktop(Writer|QR)" -count=1 -timeout 30m -v`, from PowerShell with the sandbox off. Delete `reports\autor` first. Implementer subagents cannot start Chrome; the controller runs these.
- Static tests: `npm run test:writer`, `npm run check:ui`, `go test ./ui -run "(?i)(windowmenu|contextmenu|materials|office|writer|virtualdesktop|lifecycle|i18n|feeling)" -count=1`.
- `TestDesktopCodeStudioRunUsesTerminalSession` fails on `main` for an unrelated reason. Ignore it.

## File structure

| File | Change | Responsibility |
| --- | --- | --- |
| `ui/js/desktop/apps/writer-layout.js` | create | Length unit, papers, margin presets, the model of both dialogs (pure) |
| `scripts/test-writer-layout.mjs`, `package.json` | create, modify | Its test, part of `npm run test:writer` |
| `ui/js/desktop/shared/form-dialog.js`, `ui/css/desktop-form-dialog.css` | modify | Headings, half-width fields, unit suffix, mixed checkbox, `onInput`, a wide variant |
| `ui/lang/desktop/*.json` | modify | 47 new keys in 16 languages; seven labels lose their "· mm" |
| `ui/js/desktop/apps/writer-commands.js` | modify | Six new commands, one label |
| `ui/js/desktop/apps/writer-menus.js`, `writer-context.js` | modify | Menus, right-click menu, header/footer bar |
| `ui/js/desktop/apps/writer-dialogs.js` | modify | `page()`, `paragraph()` |
| `ui/js/desktop/apps/writer-panels.js` | modify | Lengths in the unit, six papers, the two "More…" buttons; `paperSize` leaves |
| `ui/js/desktop/apps/writer.js` | modify | `units` on the state facade, three `act` cases |
| `ui/js/desktop/core/module-loader.js`, `ui/js/desktop/bundles/main.bundle.js` | modify | One new script; rebuilt bundle |
| `scripts/test-writer-menus.mjs`, `test-writer-context.mjs`, `test-writer-session.mjs` | modify | New rules; the paper test moves to the layout test |
| `ui/desktop_office_assets_test.go`, `ui/desktop_writer_browser_test.go`, `ui/desktop_writer_reload_browser_test.go` | modify | Script lists; one assertion in the unit |
| `ui/desktop_writer_layout_test.go` | create | Browser check |
| `ui/js/desktop/apps/AGENTS.md`, `documentation/autor.md`, the roadmap spec | modify | Documentation |

---

### Task 1: The layout model

**Files:**
- Create: `ui/js/desktop/apps/writer-layout.js`, `scripts/test-writer-layout.mjs`
- Modify: `package.json`

- [ ] **Step 1: Write the failing test**

Create `scripts/test-writer-layout.mjs`:

```js
import assert from 'node:assert/strict';
import vm from 'node:vm';
import {readFileSync} from 'node:fs';

const context = {window: {}};
vm.runInNewContext(readFileSync('ui/js/desktop/apps/writer-layout.js', 'utf8'), context);
const L = context.window.WriterLayout, same = (actual, expected, message) => assert.equal(JSON.stringify(actual), JSON.stringify(expected), message);

// Units: centimetres, or inches for an English interface in a US browser; comma or point on input.
const cm = L.units('de', 'de-DE'), inch = L.units('en', 'en-US');
assert.equal(cm.unit, 'cm');
assert.equal(inch.unit, 'in');
assert.equal(L.units('en', 'en-GB').unit, 'cm');
assert.equal(L.units('de', 'en-US').unit, 'cm');
assert.equal(cm.show(1417), '2,5');
assert.equal(cm.read('2,5'), 1417);
assert.equal(cm.read(' 2.5 cm '), 1417);
assert.equal(cm.read(''), null);
assert.ok(Number.isNaN(cm.read('wide')));
assert.ok(Number.isNaN(cm.read('25 mm')), 'another unit is not read as centimetres');
assert.equal(cm.read('−0,5'), -283, 'a typographic minus is a minus');
assert.equal(cm.show(null), '');
assert.equal(cm.show(''), '');
assert.equal(inch.show(1440), '1');
assert.equal(inch.read('8.5'), 12240);
assert.equal(inch.read('8.5 in'), 12240);
assert.equal(cm.number(1.15), '1,15');
assert.equal(cm.points('12,5 pt'), 12.5);
assert.equal(cm.points(''), null);

// Papers.
assert.equal(L.papers.map(paper => paper.id).join(), 'a4,a5,a3,b5,letter,legal');
assert.equal(L.paperOf(12240, 15840), 'letter');
assert.equal(L.paperOf(15840, 12240), 'letter');
assert.equal(L.paperOf(12259, 15840), 'letter');
assert.equal(L.paperOf(12260, 15840), 'custom');
assert.equal(L.paperOf(11906, 16838), 'a4');
assert.equal(L.paperOf(16838, 11906), 'a4');
assert.equal(L.paperOf(11925, 16838), 'a4');
assert.equal(L.paperOf(8391, 11906), 'a5');
assert.equal(L.paperOf(9000, 9000), 'custom');

// The Page setup dialog: fields, dependent fields, and a command that holds only what changed.
const a4 = {pageWidthTwips: 11906, pageHeightTwips: 16838, orientation: 'portrait', marginsTwips: {top: 1417, right: 1417, bottom: 1417, left: 1417}};
const page = L.pageFields(a4, cm);
same(page, {paper: 'a4', width: '21', height: '29,7', orientation: 'portrait', preset: 'normal', top: '2,5', right: '2,5', bottom: '2,5', left: '2,5', scope: 'document'});
same(L.pageInput('paper', {...page, paper: 'a5'}, cm), {width: '14,8', height: '21'});
same(L.pageInput('paper', {...page, paper: 'a5', orientation: 'landscape'}, cm), {width: '21', height: '14,8'});
same(L.pageInput('paper', {...page, paper: 'custom'}, cm), {});
same(L.pageInput('orientation', {...page, orientation: 'landscape'}, cm), {width: '29,7', height: '21'});
same(L.pageInput('orientation', {...page, orientation: 'portrait'}, cm), {});
same(L.pageInput('width', {...page, width: '29,7', height: '21'}, cm), {paper: 'a4', orientation: 'landscape'});
same(L.pageInput('height', {...page, height: '30'}, cm), {paper: 'custom', orientation: 'portrait'});
same(L.pageInput('width', {...page, width: 'x'}, cm), {});
same(L.pageInput('preset', {...page, preset: 'narrow'}, cm), {top: '1,27', right: '1,27', bottom: '1,27', left: '1,27'});
same(L.pageInput('preset', {...page, preset: 'custom'}, cm), {});
same(L.pageInput('left', {...page, left: '3'}, cm), {preset: 'custom'});
same(L.pageInput('left', {...page, left: '2,5'}, cm), {preset: 'normal'});
same(L.pageInput('scope', {...page, scope: 'section'}, cm), {});
same(L.pageCommand(page, page, cm), {command: null});
same(L.pageCommand({...page, paper: 'a5', width: '14,8', height: '21'}, page, cm), {command: {type: 'setPageSetup', pageWidth: 8391, pageHeight: 11906, orientation: 'portrait', scope: 'document'}});
same(L.pageCommand({...page, width: '29,7', height: '21', orientation: 'landscape'}, page, cm), {command: {type: 'setPageSetup', pageWidth: 16838, pageHeight: 11906, orientation: 'landscape', scope: 'document'}});
same(L.pageCommand({...page, left: '3', scope: 'section'}, page, cm), {command: {type: 'setPageSetup', marginLeft: 1701, scope: 'section'}});
same(L.pageCommand({...page, scope: 'section'}, page, cm), {command: null});
same(L.pageCommand({...page, top: 'x'}, page, cm), {error: 'length_invalid'});
same(L.pageCommand({...page, top: ''}, page, cm), {error: 'length_invalid'});
same(L.pageCommand({...page, top: '-1'}, page, cm), {error: 'length_invalid'});
same(L.pageCommand({...page, width: '1'}, page, cm), {error: 'page_size_range'});
same(L.pageCommand({...page, left: '10', right: '10'}, page, cm), {error: 'margins_too_large'});
// In inches a paper keeps its exact size although the text is rounded to two places.
const inches = L.pageFields(a4, inch);
assert.equal(inches.width, '8.27');
same(L.pageCommand({...inches, ...L.pageInput('paper', {...inches, paper: 'a5'}, inch), paper: 'a5'}, inches, inch).command,
    {type: 'setPageSetup', pageWidth: 8391, pageHeight: 11906, orientation: 'portrait', scope: 'document'});

// The Paragraph dialog.
const plain = {alignment: 'left', lineSpacing: {rule: 'multiple', value: 1.08}, spaceAfterPt: 8,
    paragraphFlags: {contextualSpacing: false, keepNext: false, keepLines: false, widowControl: true, pageBreakBefore: false},
    indent: {left: 0, right: 0, firstLine: 0, mixed: {left: false, right: false, firstLine: false}},
    disagrees: {alignment: false, direction: false, spaceBeforePt: false, spaceAfterPt: false, lineSpacing: false, tabStops: false}};
const paragraph = L.paragraphFields(plain, cm);
same(paragraph, {alignment: 'left', left: '0', right: '0', special: 'none', by: '', before: '0', after: '8', lineRule: 'multiple', lineValue: '1,08',
    keepNext: false, keepLines: false, widowControl: true, pageBreakBefore: false});
const shaped = L.paragraphFields({...plain, alignment: 'both', lineSpacing: {rule: 'multiple', value: 1.5}, indent: {...plain.indent, firstLine: -283}}, cm);
assert.equal([shaped.alignment, shaped.special, shaped.by, shaped.lineRule, shaped.lineValue].join('|'), 'justify|hanging|0,5|150|');
assert.equal(L.paragraphFields({...plain, lineSpacing: {rule: 'exact', value: 18}}, cm).lineRule, 'exact');
assert.equal(L.paragraphFields({...plain, lineSpacing: undefined}, cm).lineRule, 'single');
const mixed = L.paragraphFields({paragraphFlags: {contextualSpacing: null, keepNext: null, keepLines: false, widowControl: true, pageBreakBefore: false},
    indent: {left: 0, right: 0, firstLine: 0, mixed: {left: true, right: false, firstLine: true}},
    disagrees: {alignment: true, spaceBeforePt: true, spaceAfterPt: false, lineSpacing: true}}, cm);
same(mixed, {alignment: '', left: '', right: '0', special: '', by: '', before: '', after: '0', lineRule: '', lineValue: '',
    keepNext: null, keepLines: false, widowControl: true, pageBreakBefore: false});
assert.equal(L.paragraphFields(undefined, cm).keepNext, null);
same(L.paragraphInput('special', {...paragraph, special: 'none'}), {by: ''});
same(L.paragraphInput('special', {...paragraph, special: 'first'}), {});
same(L.paragraphInput('lineRule', {...paragraph, lineRule: 'double'}), {lineValue: ''});
same(L.paragraphInput('lineRule', {...paragraph, lineRule: 'exact'}), {});
same(L.paragraphCommand(paragraph, paragraph, cm), {command: null});
same(L.paragraphCommand({...paragraph, alignment: 'justify', left: '1', special: 'hanging', by: '0,5', before: '6', lineRule: 'exact', lineValue: '18', keepNext: true}, paragraph, cm),
    {command: {type: 'setParagraphFormat', alignment: 'justify', indentLeftTwips: 567, indentFirstLineTwips: -283, spaceBeforePt: 6, lineSpacing: {rule: 'exact', value: 18}, keepNext: true}});
same(L.paragraphCommand({...paragraph, left: '', after: ''}, paragraph, cm), {command: {type: 'setParagraphFormat', indentLeftTwips: null, spaceAfterPt: null}});
same(L.paragraphCommand({...paragraph, lineRule: '150', lineValue: ''}, paragraph, cm).command.lineSpacing, {rule: 'multiple', value: 1.5});
same(L.paragraphCommand({...paragraph, special: 'first', by: '1,25'}, paragraph, cm).command, {type: 'setParagraphFormat', indentFirstLineTwips: 709});
same(L.paragraphCommand({...paragraph, special: 'first', by: ''}, paragraph, cm), {error: 'length_invalid'});
same(L.paragraphCommand({...paragraph, right: 'x'}, paragraph, cm), {error: 'length_invalid'});
same(L.paragraphCommand({...paragraph, before: '-1'}, paragraph, cm), {error: 'spacing_invalid'});
same(L.paragraphCommand({...paragraph, lineRule: 'multiple', lineValue: '20'}, paragraph, cm), {error: 'line_spacing_invalid'});
same(L.paragraphCommand({...paragraph, lineRule: 'atLeast', lineValue: ''}, paragraph, cm), {error: 'line_spacing_invalid'});
same(L.paragraphCommand({...mixed, keepNext: true}, mixed, cm), {command: {type: 'setParagraphFormat', keepNext: true}});
console.log('Writer layout: units, papers, page setup and paragraph models pass');
```

- [ ] **Step 2: Run it to see it fail**

Run: `node scripts/test-writer-layout.mjs`
Expected: `ENOENT` for `ui/js/desktop/apps/writer-layout.js`.

- [ ] **Step 3: Write the module**

Create `ui/js/desktop/apps/writer-layout.js`:

```js
(function () {
    'use strict';
    // Page and paragraph layout as the dialogs and the Format panel show it. Pure: no DOM, no editor.
    const PER = {cm: 1440 / 2.54, in: 1440};
    // Lengths are shown in centimetres, or in inches for an English interface in a US browser.
    // locale is the interface language, region the browser's language tag.
    function units(locale, region) {
        const unit = /^en\b/i.test(String(locale || '')) && /^en-US$/i.test(String(region || '')) ? 'in' : 'cm';
        const options = {maximumFractionDigits: 2, useGrouping: false};
        let format;
        try { format = new Intl.NumberFormat(locale || 'en', options); } catch (_) { format = new Intl.NumberFormat('en', options); }
        const known = value => value !== null && value !== undefined && value !== '' && Number.isFinite(Number(value));
        // A number as the user reads it; '' for an unknown or mixed value.
        const number = value => known(value) ? format.format(Number(value)) : '';
        // A typed number, with a comma or a point: null for an empty field, NaN for anything else.
        function parse(text) {
            const raw = String(text ?? '').trim().replace('−', '-').replace(',', '.');
            if (!raw) return null;
            return /^-?(\d+\.?\d*|\.\d+)$/.test(raw) ? Number(raw) : NaN;
        }
        const suffix = unit === 'in' ? /\s*(in|"|″)\s*$/i : /\s*cm\s*$/i;
        const show = twips => known(twips) ? number(Number(twips) / PER[unit]) : '';
        // Twips for a typed length; the unit's own name may follow the number.
        function read(text) {
            const value = parse(String(text ?? '').replace(suffix, ''));
            return value === null || Number.isNaN(value) ? value : Math.round(value * PER[unit]);
        }
        const points = text => parse(String(text ?? '').replace(/\s*pt\s*$/i, ''));
        return {unit, number, parse, show, read, points};
    }
    const PAPERS = [{id: 'a4', name: 'A4', width: 11906, height: 16838}, {id: 'a5', name: 'A5', width: 8391, height: 11906}, {id: 'a3', name: 'A3', width: 16838, height: 23811},
        {id: 'b5', name: 'B5', width: 9978, height: 14173}, {id: 'letter', name: 'Letter', width: 12240, height: 15840}, {id: 'legal', name: 'Legal', width: 12240, height: 20160}];
    // The paper a size is, in either orientation; 'custom' for anything else.
    function paperOf(width, height) {
        const short = Math.min(width, height), long = Math.max(width, height);
        return PAPERS.find(paper => Math.abs(short - paper.width) < 20 && Math.abs(long - paper.height) < 20)?.id || 'custom';
    }
    // Margin presets as [top, right, bottom, left] in twips.
    const MARGINS = {normal: [1417, 1417, 1417, 1417], narrow: [720, 720, 720, 720], wide: [1440, 2880, 1440, 2880]};
    const SIDES = ['top', 'right', 'bottom', 'left'];
    const presetOf = margins => Object.keys(MARGINS).find(id => MARGINS[id].every((twips, index) => Math.abs(twips - margins[index]) < 6)) || 'custom';
    // A typed length shows two decimals. Where it shows exactly like a paper edge or a preset margin,
    // that exact value is meant, so choosing A5 in inches does not shave the page.
    const KNOWN = [...new Set([...PAPERS.flatMap(paper => [paper.width, paper.height]), ...Object.values(MARGINS).flat()])];
    function exact(text, u) {
        const twips = u.read(text);
        return Number.isFinite(twips) ? KNOWN.find(value => u.show(value) === u.show(twips)) ?? twips : twips;
    }
    // The Page setup dialog's fields for a PageSetup, as the texts the dialog shows.
    function pageFields(setup, u) {
        const margins = SIDES.map(side => setup.marginsTwips[side]);
        return {paper: paperOf(setup.pageWidthTwips, setup.pageHeightTwips), width: u.show(setup.pageWidthTwips), height: u.show(setup.pageHeightTwips),
            orientation: setup.pageWidthTwips > setup.pageHeightTwips ? 'landscape' : 'portrait', preset: presetOf(margins),
            top: u.show(margins[0]), right: u.show(margins[1]), bottom: u.show(margins[2]), left: u.show(margins[3]), scope: 'document'};
    }
    // What else changes in the Page setup dialog when one field was edited: {field: text}.
    function pageInput(changed, values, u) {
        const width = exact(values.width, u), height = exact(values.height, u), sized = Number.isFinite(width) && Number.isFinite(height);
        if (changed === 'paper') {
            const paper = PAPERS.find(candidate => candidate.id === values.paper);
            if (!paper) return {};
            const landscape = values.orientation === 'landscape';
            return {width: u.show(landscape ? paper.height : paper.width), height: u.show(landscape ? paper.width : paper.height)};
        }
        if (changed === 'orientation') return sized && (width > height) !== (values.orientation === 'landscape') ? {width: u.show(height), height: u.show(width)} : {};
        if (changed === 'width' || changed === 'height') return sized ? {paper: paperOf(width, height), orientation: width > height ? 'landscape' : 'portrait'} : {};
        if (changed === 'preset') return MARGINS[values.preset] ? Object.fromEntries(SIDES.map((side, index) => [side, u.show(MARGINS[values.preset][index])])) : {};
        if (SIDES.includes(changed)) {
            const margins = SIDES.map(side => exact(values[side], u));
            return margins.every(Number.isFinite) ? {preset: presetOf(margins)} : {};
        }
        return {};
    }
    // The setPageSetup command for the dialog's answer: only what differs from what the dialog opened
    // with. {command} (null when nothing changed) or {error} with a desktop.writer_ key.
    function pageCommand(values, initial, u) {
        const width = exact(values.width, u), height = exact(values.height, u), margins = SIDES.map(side => exact(values[side], u));
        if (![width, height, ...margins].every(Number.isFinite) || margins.some(twips => twips < 0)) return {error: 'length_invalid'};
        if (Math.min(width, height) < 1440 || Math.max(width, height) > 31680) return {error: 'page_size_range'};
        if (margins[1] + margins[3] > width - 1440 || margins[0] + margins[2] > height - 1440) return {error: 'margins_too_large'};
        const command = {type: 'setPageSetup'};
        if (values.width !== initial.width || values.height !== initial.height) Object.assign(command, {pageWidth: width, pageHeight: height, orientation: width > height ? 'landscape' : 'portrait'});
        SIDES.forEach((side, index) => { if (values[side] !== initial[side]) command['margin' + side[0].toUpperCase() + side.slice(1)] = margins[index]; });
        if (Object.keys(command).length === 1) return {command: null};
        command.scope = values.scope === 'section' ? 'section' : 'document';
        return {command};
    }
    const LINES = {single: 1, '115': 1.15, '150': 1.5, double: 2};
    const FLAGS = ['keepNext', 'keepLines', 'widowControl', 'pageBreakBefore'];
    // The Paragraph dialog's fields for snapshot().formatting. '' is a text the selection disagrees
    // on or cannot state, null a flag it disagrees on.
    function paragraphFields(formatting, u) {
        const f = formatting || {}, off = f.disagrees || {}, indent = f.indent, mixed = indent?.mixed || {};
        const first = indent && !mixed.firstLine ? indent.firstLine : null, line = f.lineSpacing;
        const preset = line?.rule === 'multiple' ? Object.keys(LINES).find(id => LINES[id] === line.value) : '';
        const fields = {
            alignment: off.alignment || !f.alignment ? '' : f.alignment === 'both' ? 'justify' : f.alignment,
            left: indent && !mixed.left ? u.show(indent.left) : '', right: indent && !mixed.right ? u.show(indent.right) : '',
            special: first === null ? '' : first > 0 ? 'first' : first < 0 ? 'hanging' : 'none', by: first ? u.show(Math.abs(first)) : '',
            before: off.spaceBeforePt ? '' : u.number(f.spaceBeforePt ?? 0), after: off.spaceAfterPt ? '' : u.number(f.spaceAfterPt ?? 0),
            lineRule: off.lineSpacing ? '' : !line ? 'single' : preset || line.rule, lineValue: off.lineSpacing || !line || preset ? '' : u.number(line.value)
        };
        for (const flag of FLAGS) fields[flag] = f.paragraphFlags ? f.paragraphFlags[flag] : null;
        return fields;
    }
    // What else changes in the Paragraph dialog when one field was edited.
    function paragraphInput(changed, values) {
        if (changed === 'special' && (values.special === 'none' || values.special === '')) return {by: ''};
        if (changed === 'lineRule' && (LINES[values.lineRule] || values.lineRule === '')) return {lineValue: ''};
        return {};
    }
    // The setParagraphFormat command for the dialog's answer: only what differs from what the dialog
    // opened with. An emptied length or spacing is null, which hands the value back to the style.
    function paragraphCommand(values, initial, u) {
        const command = {type: 'setParagraphFormat'};
        if (values.alignment !== initial.alignment && values.alignment) command.alignment = values.alignment;
        for (const [field, key] of [['left', 'indentLeftTwips'], ['right', 'indentRightTwips']]) {
            if (values[field] === initial[field]) continue;
            const twips = u.read(values[field]);
            if (Number.isNaN(twips)) return {error: 'length_invalid'};
            command[key] = twips;
        }
        if (values.special !== initial.special || values.by !== initial.by) {
            if (values.special === 'none') command.indentFirstLineTwips = 0;
            else if (values.special) {
                const by = u.read(values.by);
                if (!Number.isFinite(by) || by <= 0) return {error: 'length_invalid'};
                command.indentFirstLineTwips = values.special === 'hanging' ? -by : by;
            }
        }
        for (const [field, key] of [['before', 'spaceBeforePt'], ['after', 'spaceAfterPt']]) {
            if (values[field] === initial[field]) continue;
            const points = u.points(values[field]);
            if (Number.isNaN(points) || points < 0 || points > 1584) return {error: 'spacing_invalid'};
            command[key] = points;
        }
        if (values.lineRule !== initial.lineRule || values.lineValue !== initial.lineValue) {
            if (LINES[values.lineRule]) command.lineSpacing = {rule: 'multiple', value: LINES[values.lineRule]};
            else if (values.lineRule) {
                const value = u.points(values.lineValue), multiple = values.lineRule === 'multiple';
                if (!Number.isFinite(value) || value < (multiple ? 0.5 : 1) || value > (multiple ? 10 : 1584)) return {error: 'line_spacing_invalid'};
                command.lineSpacing = {rule: values.lineRule, value};
            }
        }
        for (const flag of FLAGS) if (values[flag] !== initial[flag] && typeof values[flag] === 'boolean') command[flag] = values[flag];
        return {command: Object.keys(command).length > 1 ? command : null};
    }
    window.WriterLayout = {units, papers: PAPERS, margins: MARGINS, lines: LINES, paperOf, pageFields, pageInput, pageCommand, paragraphFields, paragraphInput, paragraphCommand};
})();
```

- [ ] **Step 4: Run the test to see it pass, add it to the suite**

Run: `node scripts/test-writer-layout.mjs`
Expected: `Writer layout: units, papers, page setup and paragraph models pass`.

In `package.json` replace

```js
node scripts/test-writer-context.mjs",
```
with
```js
node scripts/test-writer-context.mjs && node scripts/test-writer-layout.mjs",
```

Run: `npm run test:writer`
Expected: the last line is the layout line; nothing fails.

- [ ] **Step 5: Commit**

```
git add ui/js/desktop/apps/writer-layout.js scripts/test-writer-layout.mjs package.json
git diff --cached --check
git commit
```

Message:

```
feat(writer): layout model for the page and paragraph dialogs

WriterLayout holds the length unit (centimetres, inches in a US
browser), the paper and margin catalogue and the model of both
dialogs: fields from the engine's state, dependent fields, and one
command with only what changed.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: Claude Fable 5.2 <noreply@anthropic.com>
```

---

### Task 2: The form dialog learns what the two dialogs need

**Files:**
- Modify: `ui/js/desktop/shared/form-dialog.js`, `ui/css/desktop-form-dialog.css`

There is no node test for the form dialog (it is DOM); the browser check in Task 5 covers it, and every existing dialog check must keep passing.

- [ ] **Step 1: `form-dialog.js`**

Replace

```js
    // Field: {id, label, type: 'text'|'url'|'number'|'select'|'checkbox'|'static'|'grid', value, required,
    // min, max, step, maxlength, placeholder, options: [{value, label}], size}.
    //   select  a drop-down; with size it is a list that shows that many options at once.
```
with
```js
    // Field: {id, label, type: 'text'|'url'|'number'|'select'|'checkbox'|'static'|'grid'|'heading', value, required,
    // min, max, step, maxlength, placeholder, inputmode, options: [{value, label}], size, half, suffix, mixed}.
    //   select  a drop-down; with size it is a list that shows that many options at once.
    //   heading a title over the fields that follow. It answers nothing.
    //   half    the field takes half the dialog's width, so two stand side by side.
    //   suffix  a unit shown after a text or number input.
    //   mixed   a checkbox that starts undecided and answers null until it is clicked.
```

Replace

```js
    const silent = field => field.type === 'static' || field.type === 'grid';
    function fieldMarkup(field) {
        const name = 'data-form-field="' + escapeHTML(field.id) + '"', label = '<span>' + escapeHTML(field.label) + '</span>';
        if (field.type === 'static') return '<p class="vd-form-static">' + label + '<strong>' + escapeHTML(field.value ?? '') + '</strong></p>';
```
with
```js
    const silent = field => ['static', 'grid', 'heading'].includes(field.type);
    function fieldMarkup(field) {
        const name = 'data-form-field="' + escapeHTML(field.id) + '"', label = '<span>' + escapeHTML(field.label) + '</span>', half = field.half ? ' vd-form-half' : '';
        if (field.type === 'static') return '<p class="vd-form-static">' + label + '<strong>' + escapeHTML(field.value ?? '') + '</strong></p>';
        if (field.type === 'heading') return '<h3 class="vd-form-heading">' + escapeHTML(field.label) + '</h3>';
```

Replace

```js
        if (field.type === 'checkbox') return '<label class="vd-form-check"><input type="checkbox" ' + name + (field.value ? ' checked' : '') + '>' + label + '</label>';
        if (field.type === 'select') return '<label>' + label + '<select class="vd-modal-input" ' + name + (field.size ? ' size="' + Number(field.size) + '"' : '') + '>' + (field.options || []).map(option =>
```
with
```js
        if (field.type === 'checkbox') return '<label class="vd-form-check' + half + '"><input type="checkbox" ' + name + (field.value ? ' checked' : '') + '>' + label + '</label>';
        if (field.type === 'select') return '<label class="vd-form-field' + half + '">' + label + '<select class="vd-modal-input" ' + name + (field.size ? ' size="' + Number(field.size) + '"' : '') + '>' + (field.options || []).map(option =>
```

Replace

```js
        const limits = ['min', 'max', 'step', 'maxlength', 'placeholder'].filter(key => field[key] !== undefined && field[key] !== null)
            .map(key => ' ' + key + '="' + escapeHTML(field[key]) + '"').join('');
        return '<label>' + label + '<input class="vd-modal-input" type="' + (['number', 'url'].includes(field.type) ? field.type : 'text') + '" ' + name +
            ' value="' + escapeHTML(field.value ?? '') + '"' + limits + (field.required ? ' required' : '') + ' autocomplete="off"></label>';
```
with
```js
        const limits = ['min', 'max', 'step', 'maxlength', 'placeholder', 'inputmode'].filter(key => field[key] !== undefined && field[key] !== null)
            .map(key => ' ' + key + '="' + escapeHTML(field[key]) + '"').join('');
        const box = '<input class="vd-modal-input" type="' + (['number', 'url'].includes(field.type) ? field.type : 'text') + '" ' + name +
            ' value="' + escapeHTML(field.value ?? '') + '"' + limits + (field.required ? ' required' : '') + ' autocomplete="off">';
        return '<label class="vd-form-field' + half + '">' + label + (field.suffix ? '<span class="vd-form-unit">' + box + '<small>' + escapeHTML(field.suffix) + '</small></span>' : box) + '</label>';
```

Replace

```js
    // A number answers as a Number, a checkbox as a boolean, everything else as a trimmed string.
    function read(field, el) {
        if (field.type === 'checkbox') return el.checked;
```
with
```js
    // A number answers as a Number, a checkbox as a boolean (null while it is undecided), everything else as a trimmed string.
    function read(field, el) {
        if (field.type === 'checkbox') return el.indeterminate ? null : el.checked;
```

Replace

```js
    // open({title, fields, submitLabel, cancelLabel, extraLabel, validate, signal}) resolves to {id: value},
```
with
```js
    // open({title, fields, submitLabel, cancelLabel, extraLabel, validate, onInput, wide, signal}) resolves to {id: value},
```

Replace

```js
    // validate(values) returns a message to refuse them. Without cancelLabel there is no Cancel button.
```
with
```js
    // validate(values) returns a message to refuse them. Without cancelLabel there is no Cancel button.
    // onInput(changed, values, set) runs after every edit, so a dialog can keep dependent fields in
    // step: changed is the edited field's id, set(id, value) writes another field. wide gives more room.
```

Replace

```js
        node.className = 'vd-form-dialog';
```
with
```js
        node.className = 'vd-form-dialog' + (options.wide ? ' vd-form-wide' : '');
```

Replace

```js
            const collect = () => Object.fromEntries(fields.filter(field => !silent(field)).map(field => [field.id, read(field, input(field.id))]));
```
with
```js
            const collect = () => Object.fromEntries(fields.filter(field => !silent(field)).map(field => [field.id, read(field, input(field.id))]));
            for (const field of fields) if (field.type === 'checkbox' && field.mixed) input(field.id).indeterminate = true;
            const set = (fieldId, value) => {
                const el = input(fieldId);
                if (!el) return;
                if (el.type === 'checkbox') { el.indeterminate = value === null; el.checked = !!value; } else el.value = value;
            };
            if (options.onInput) form.addEventListener('input', event => {
                const changed = event.target.dataset?.formField;
                if (changed) options.onInput(changed, collect(), set);
            });
```

- [ ] **Step 2: `desktop-form-dialog.css`**

Replace

```css
.vd-form-dialog form{display:grid;gap:12px;margin:0;max-height:calc(100vh - 96px);overflow:auto}
```
with
```css
.vd-form-dialog.vd-form-wide{width:min(520px,calc(100vw - 32px))}
.vd-form-dialog form{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:12px;margin:0;max-height:calc(100vh - 96px);overflow:auto}
.vd-form-dialog form>*{grid-column:1/-1}
.vd-form-dialog form>.vd-form-half{grid-column:auto}
.vd-form-dialog .vd-form-heading{margin:6px 0 -4px;font-size:11px;font-weight:700;letter-spacing:.04em;text-transform:uppercase;color:var(--vd-muted,var(--vd-theme-muted,#a7b1c3))}
.vd-form-dialog .vd-form-unit{display:flex;align-items:center;gap:6px}
.vd-form-dialog .vd-form-unit input{flex:1;min-width:0}
.vd-form-dialog .vd-form-unit small{flex:none;min-width:1.6em;color:var(--vd-muted,var(--vd-theme-muted,#a7b1c3))}
```

- [ ] **Step 3: Check and commit**

Run: `node --check ui/js/desktop/shared/form-dialog.js`, `npm run test:writer`, `go test ./ui -run "(?i)(materials|office)" -count=1`.
Expected: all pass.

```
git add ui/js/desktop/shared/form-dialog.js ui/css/desktop-form-dialog.css
git diff --cached --check
git commit
```

Message:

```
feat(desktop): form dialog headings, half-width fields, units and dependent fields

A heading groups fields, half puts two side by side, suffix shows a
unit, a mixed checkbox answers null until it is clicked, and onInput
lets a dialog keep dependent fields in step.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: Claude Fable 5.2 <noreply@anthropic.com>
```

---

### Task 3: Language keys

**Files:**
- Modify: `ui/lang/desktop/*.json` (script)

- [ ] **Step 1: Add the keys and take the unit out of seven labels**

Create `disposable/_writer-wave2a/i18n.mjs`:

```js
// Adds the Wave 2a keys to the sixteen desktop language files, each at its sorted place in the desktop.writer_ block,
// and takes " · mm" off seven labels: the Format panel now adds the unit of the user's locale itself.
// Usage: node disposable/_writer-wave2a/i18n.mjs <repo root>
import {readFileSync, writeFileSync} from 'node:fs';
import path from 'node:path';

const root = process.argv[2] || process.cwd();
const LANGS = ['cs', 'da', 'de', 'el', 'en', 'es', 'fr', 'hi', 'it', 'ja', 'nl', 'no', 'pl', 'pt', 'sv', 'zh'];
const UNITLESS = ['margin_top', 'margin_bottom', 'margin_left', 'margin_right', 'left_indent', 'first_indent', 'image_width'];
const KEYS = {
    page_setup_dialog: ['Vzhled stránky…', 'Sideopsætning…', 'Seite einrichten…', 'Διαμόρφωση σελίδας…', 'Page setup…', 'Configurar página…', 'Mise en page…', 'पृष्ठ सेटअप…', 'Imposta pagina…', 'ページ設定…', 'Pagina-instelling…', 'Utskriftsformat…', 'Ustawienia strony…', 'Configurar página…', 'Utskriftsformat…', '页面设置…'],
    paragraph_dialog: ['Odstavec…', 'Afsnit…', 'Absatz…', 'Παράγραφος…', 'Paragraph…', 'Párrafo…', 'Paragraphe…', 'अनुच्छेद…', 'Paragrafo…', '段落…', 'Alinea…', 'Avsnitt…', 'Akapit…', 'Parágrafo…', 'Stycke…', '段落…'],
    page_width: ['Šířka', 'Bredde', 'Breite', 'Πλάτος', 'Width', 'Ancho', 'Largeur', 'चौड़ाई', 'Larghezza', '幅', 'Breedte', 'Bredde', 'Szerokość', 'Largura', 'Bredd', '宽度'],
    page_height: ['Výška', 'Højde', 'Höhe', 'Ύψος', 'Height', 'Alto', 'Hauteur', 'ऊँचाई', 'Altezza', '高さ', 'Hoogte', 'Høyde', 'Wysokość', 'Altura', 'Höjd', '高度'],
    margins: ['Okraje', 'Margener', 'Ränder', 'Περιθώρια', 'Margins', 'Márgenes', 'Marges', 'हाशिए', 'Margini', '余白', 'Marges', 'Marger', 'Marginesy', 'Margens', 'Marginaler', '页边距'],
    margin_preset: ['Předvolba', 'Forudindstilling', 'Voreinstellung', 'Προρύθμιση', 'Preset', 'Preestablecido', 'Préréglage', 'प्रीसेट', 'Preimpostazione', 'プリセット', 'Voorinstelling', 'Forhåndsinnstilling', 'Ustawienie wstępne', 'Predefinição', 'Förinställning', '预设'],
    margins_normal: ['Normální', 'Normal', 'Normal', 'Κανονικό', 'Normal', 'Normal', 'Normales', 'सामान्य', 'Normali', '標準', 'Normaal', 'Normal', 'Normalne', 'Normal', 'Normal', '常规'],
    margins_narrow: ['Úzké', 'Smal', 'Schmal', 'Στενό', 'Narrow', 'Estrecho', 'Étroites', 'संकीर्ण', 'Stretti', '狭い', 'Smal', 'Smal', 'Wąskie', 'Estreita', 'Smal', '窄'],
    margins_wide: ['Široké', 'Bred', 'Breit', 'Πλατύ', 'Wide', 'Ancho', 'Larges', 'चौड़ा', 'Larghi', '広い', 'Breed', 'Bred', 'Szerokie', 'Larga', 'Bred', '宽'],
    margins_custom: ['Vlastní', 'Brugerdefineret', 'Benutzerdefiniert', 'Προσαρμοσμένο', 'Custom', 'Personalizado', 'Personnalisées', 'कस्टम', 'Personalizzati', 'ユーザー設定', 'Aangepast', 'Egendefinert', 'Niestandardowe', 'Personalizada', 'Anpassad', '自定义'],
    apply_to: ['Použít na', 'Anvend på', 'Anwenden auf', 'Εφαρμογή σε', 'Apply to', 'Aplicar a', 'Appliquer à', 'इस पर लागू करें', 'Applica a', '設定対象', 'Toepassen op', 'Bruk på', 'Zastosuj do', 'Aplicar a', 'Använd på', '应用于'],
    apply_document: ['Celý dokument', 'Hele dokumentet', 'Ganzes Dokument', 'Ολόκληρο το έγγραφο', 'Whole document', 'Todo el documento', 'À tout le document', 'संपूर्ण दस्तावेज़', 'Intero documento', '文書全体', 'Heel document', 'Hele dokumentet', 'Cały dokument', 'Todo o documento', 'Hela dokumentet', '整篇文档'],
    apply_section: ['Tento oddíl', 'Denne sektion', 'Diesen Abschnitt', 'Αυτή την ενότητα', 'This section', 'Esta sección', 'À cette section', 'यह अनुभाग', 'Questa sezione', 'このセクション', 'Deze sectie', 'Denne inndelingen', 'Ta sekcja', 'Esta secção', 'Det här avsnittet', '本节'],
    unit_cm: ['cm', 'cm', 'cm', 'εκ.', 'cm', 'cm', 'cm', 'सेमी', 'cm', 'cm', 'cm', 'cm', 'cm', 'cm', 'cm', '厘米'],
    unit_in: ['in', 'tommer', 'Zoll', 'ίντσες', 'in', 'pulg.', 'po', 'इंच', 'poll.', 'インチ', 'inch', 'tommer', 'in', 'pol.', 'tum', '英寸'],
    length_invalid: ['Zadejte délku jako číslo, například 2,5.', 'Angiv en længde som et tal, for eksempel 2,5.', 'Gib eine Länge als Zahl ein, zum Beispiel 2,5.', 'Εισαγάγετε ένα μήκος ως αριθμό, για παράδειγμα 2,5.', 'Enter a length as a number, for example 2.5.', 'Introduce una longitud como número, por ejemplo 2,5.', 'Saisissez une longueur sous forme de nombre, par exemple 2,5.', 'लंबाई को संख्या के रूप में दर्ज करें, उदाहरण के लिए 2.5।', 'Inserisci una lunghezza come numero, ad esempio 2,5.', '長さを数値で入力してください（例: 2.5）。', 'Voer een lengte in als getal, bijvoorbeeld 2,5.', 'Angi en lengde som et tall, for eksempel 2,5.', 'Wprowadź długość jako liczbę, na przykład 2,5.', 'Introduza um comprimento como número, por exemplo 2,5.', 'Ange en längd som ett tal, till exempel 2,5.', '请输入数字形式的长度，例如 2.5。'],
    page_size_range: ['Strana stránky musí měřit 2,54 až 55,88 cm (1 až 22 palců).', 'En sidekant skal være mellem 2,54 og 55,88 cm (1 til 22 tommer).', 'Eine Seitenkante muss zwischen 2,54 und 55,88 cm lang sein (1 bis 22 Zoll).', 'Κάθε πλευρά της σελίδας πρέπει να είναι μεταξύ 2,54 και 55,88 εκ. (1 έως 22 ίντσες).', 'A page side must be between 2.54 and 55.88 cm (1 to 22 in).', 'Cada lado de la página debe medir entre 2,54 y 55,88 cm (de 1 a 22 pulg.).', 'Un côté de la page doit mesurer entre 2,54 et 55,88 cm (1 à 22 po).', 'पृष्ठ की भुजा 2.54 और 55.88 सेमी (1 से 22 इंच) के बीच होनी चाहिए।', 'Un lato della pagina deve essere compreso tra 2,54 e 55,88 cm (da 1 a 22 poll.).', 'ページの辺は 2.54～55.88 cm（1～22 インチ）の範囲で指定してください。', 'Een paginazijde moet tussen 2,54 en 55,88 cm liggen (1 tot 22 inch).', 'En sidekant må være mellom 2,54 og 55,88 cm (1 til 22 tommer).', 'Bok strony musi mieć od 2,54 do 55,88 cm (od 1 do 22 cali).', 'Um lado da página tem de medir entre 2,54 e 55,88 cm (1 a 22 pol.).', 'En sidkant måste vara mellan 2,54 och 55,88 cm (1 till 22 tum).', '页面边长必须介于 2.54 和 55.88 厘米之间（1 到 22 英寸）。'],
    margins_too_large: ['Okraje nechávají příliš málo místa pro text.', 'Margenerne efterlader for lidt plads til teksten.', 'Die Ränder lassen zu wenig Platz für den Text.', 'Τα περιθώρια αφήνουν πολύ λίγο χώρο για το κείμενο.', 'The margins leave too little room for the text.', 'Los márgenes dejan muy poco espacio para el texto.', 'Les marges laissent trop peu de place au texte.', 'हाशिए पाठ के लिए बहुत कम जगह छोड़ते हैं।', 'I margini lasciano troppo poco spazio per il testo.', '余白が大きすぎて、本文の領域が足りません。', 'De marges laten te weinig ruimte over voor de tekst.', 'Margene gir for lite plass til teksten.', 'Marginesy pozostawiają za mało miejsca na tekst.', 'As margens deixam demasiado pouco espaço para o texto.', 'Marginalerna lämnar för lite utrymme för texten.', '页边距过大，留给正文的空间不足。'],
    more: ['Více…', 'Mere…', 'Mehr…', 'Περισσότερα…', 'More…', 'Más…', 'Plus…', 'अधिक…', 'Altro…', 'その他…', 'Meer…', 'Mer…', 'Więcej…', 'Mais…', 'Mer…', '更多…'],
    alignment: ['Zarovnání', 'Justering', 'Ausrichtung', 'Στοίχιση', 'Alignment', 'Alineación', 'Alignement', 'संरेखण', 'Allineamento', '配置', 'Uitlijning', 'Justering', 'Wyrównanie', 'Alinhamento', 'Justering', '对齐方式'],
    indent_heading: ['Odsazení', 'Indrykning', 'Einzug', 'Εσοχή', 'Indentation', 'Sangría', 'Retrait', 'इंडेंटेशन', 'Rientri', 'インデント', 'Inspringing', 'Innrykk', 'Wcięcia', 'Avanço', 'Indrag', '缩进'],
    right_indent: ['Vpravo', 'Højre', 'Rechts', 'Δεξιά', 'Right', 'Derecha', 'Droite', 'दायाँ', 'A destra', '右', 'Rechts', 'Høyre', 'Z prawej', 'Direita', 'Höger', '右侧'],
    indent_special: ['Speciální', 'Speciel', 'Sondereinzug', 'Ειδική', 'Special', 'Especial', 'Spécial', 'विशेष', 'Speciale', '最初の行', 'Speciaal', 'Spesielt', 'Specjalne', 'Especial', 'Special', '特殊格式'],
    indent_none: ['Žádné', 'Ingen', 'Kein', 'Καμία', 'None', 'Ninguna', 'Aucun', 'कोई नहीं', 'Nessuno', 'なし', 'Geen', 'Ingen', 'Brak', 'Nenhum', 'Inget', '无'],
    indent_first: ['První řádek', 'Første linje', 'Erste Zeile', 'Πρώτη γραμμή', 'First line', 'Primera línea', 'Première ligne', 'पहली पंक्ति', 'Prima riga', '字下げ', 'Eerste regel', 'Første linje', 'Pierwszy wiersz', 'Primeira linha', 'Första raden', '首行缩进'],
    indent_hanging: ['Předsazení', 'Hængende', 'Hängend', 'Προεξοχή', 'Hanging', 'Francesa', 'Suspendu', 'हैंगिंग', 'Sporgente', 'ぶら下げ', 'Verkeerd-om', 'Hengende', 'Wysunięcie', 'Pendente', 'Hängande', '悬挂缩进'],
    indent_by: ['O kolik', 'Med', 'Um', 'Κατά', 'By', 'En', 'De', 'द्वारा', 'Rientra di', '幅', 'Met', 'Med', 'Co', 'Por', 'Med', '缩进值'],
    spacing_heading: ['Mezery', 'Afstand', 'Abstand', 'Διάστημα', 'Spacing', 'Espaciado', 'Espacement', 'रिक्ति', 'Spaziatura', '間隔', 'Afstand', 'Avstand', 'Odstępy', 'Espaçamento', 'Avstånd', '间距'],
    line_single: ['Jednoduché', 'Enkelt', 'Einfach', 'Μονό', 'Single', 'Sencillo', 'Simple', 'एकल', 'Singola', '1 行', 'Enkel', 'Enkel', 'Pojedyncze', 'Simples', 'Enkelt', '单倍行距'],
    line_double: ['Dvojité', 'Dobbelt', 'Doppelt', 'Διπλό', 'Double', 'Doble', 'Double', 'दोहरा', 'Doppia', '2 行', 'Dubbel', 'Dobbel', 'Podwójne', 'Duplo', 'Dubbelt', '2 倍行距'],
    line_at_least: ['Nejméně', 'Mindst', 'Mindestens', 'Τουλάχιστον', 'At least', 'Mínimo', 'Au moins', 'कम से कम', 'Minima', '最小値', 'Ten minste', 'Minst', 'Co najmniej', 'Pelo menos', 'Minst', '最小值'],
    line_exact: ['Přesně', 'Præcis', 'Genau', 'Ακριβώς', 'Exactly', 'Exacto', 'Exactement', 'ठीक', 'Esatta', '固定値', 'Exact', 'Nøyaktig', 'Dokładnie', 'Exatamente', 'Exakt', '固定值'],
    line_multiple: ['Násobky', 'Flere', 'Mehrfach', 'Πολλαπλό', 'Multiple', 'Múltiple', 'Multiple', 'एकाधिक', 'Multipla', '倍数', 'Meerdere', 'Flere', 'Wielokrotne', 'Múltiplo', 'Flera', '多倍行距'],
    line_value: ['Výška', 'På', 'Wert', 'Στα', 'At', 'En', 'De', 'पर', 'Valore', '間隔', 'Op', 'Verdi', 'Co', 'Em', 'Värde', '设置值'],
    breaks_heading: ['Stránkování', 'Sideinddeling', 'Umbruch', 'Σελιδοποίηση', 'Pagination', 'Paginación', 'Pagination', 'पृष्ठांकन', 'Impaginazione', '改ページ位置の自動修正', 'Paginering', 'Paginering', 'Podział na strony', 'Paginação', 'Sidbrytning', '分页'],
    keep_next: ['Svázat s následujícím', 'Hold sammen med næste', 'Mit nächstem Absatz zusammenhalten', 'Διατήρηση με το επόμενο', 'Keep with next', 'Conservar con el siguiente', 'Paragraphes solidaires', 'अगले के साथ रखें', 'Mantieni con il successivo', '次の段落と分離しない', 'Bij volgende alinea houden', 'Hold sammen med neste', 'Razem z następnym', 'Manter com o seguinte', 'Håll ihop med nästa', '与下段同页'],
    keep_lines: ['Svázat řádky', 'Hold linjer sammen', 'Zeilen nicht trennen', 'Διατήρηση γραμμών μαζί', 'Keep lines together', 'Conservar líneas juntas', 'Lignes solidaires', 'पंक्तियों को साथ रखें', 'Mantieni assieme le righe', '段落を分割しない', 'Regels bijeenhouden', 'Hold linjer sammen', 'Zachowaj wiersze razem', 'Manter linhas juntas', 'Håll ihop rader', '段中不分页'],
    widow_control: ['Kontrola osamocených řádků', 'Kontrol af horeunger', 'Absatzkontrolle', 'Έλεγχος χηρών και ορφανών', 'Widow and orphan control', 'Control de líneas viudas y huérfanas', 'Éviter veuves et orphelines', 'विडो और ऑर्फ़न नियंत्रण', 'Controlla righe isolate', '改ページ時 1 行残して段落を区切らない', 'Zwevende regels voorkomen', 'Kontroll av enker og foreldreløse', 'Kontrola bękartów i wdów', 'Controlo de linhas órfãs e viúvas', 'Kontroll av enstaka rader', '孤行控制'],
    page_break_before: ['Vložit konec stránky před', 'Sideskift før', 'Seitenumbruch davor', 'Αλλαγή σελίδας πριν', 'Page break before', 'Salto de página anterior', 'Saut de page avant', 'पहले पृष्ठ विराम', 'Anteponi interruzione', '段落前で改ページする', 'Pagina-einde ervoor', 'Sideskift foran', 'Podział strony przed', 'Quebra de página antes', 'Sidbrytning före', '段前分页'],
    spacing_invalid: ['Mezera musí být v rozsahu 0 až 1584 pt.', 'Afstanden skal være mellem 0 og 1584 pt.', 'Der Abstand muss zwischen 0 und 1584 pt liegen.', 'Το διάστημα πρέπει να είναι μεταξύ 0 και 1584 pt.', 'Spacing must be between 0 and 1584 pt.', 'El espaciado debe estar entre 0 y 1584 pt.', 'L’espacement doit être compris entre 0 et 1584 pt.', 'रिक्ति 0 और 1584 pt के बीच होनी चाहिए।', 'La spaziatura deve essere compresa tra 0 e 1584 pt.', '間隔は 0～1584 pt の範囲で指定してください。', 'De afstand moet tussen 0 en 1584 pt liggen.', 'Avstanden må være mellom 0 og 1584 pt.', 'Odstęp musi mieścić się w zakresie od 0 do 1584 pt.', 'O espaçamento tem de estar entre 0 e 1584 pt.', 'Avståndet måste vara mellan 0 och 1584 pt.', '间距必须介于 0 和 1584 pt 之间。'],
    line_spacing_invalid: ['Zadejte řádkování: 0,5 až 10 řádků nebo 1 až 1584 pt.', 'Angiv en linjeafstand: 0,5 til 10 linjer eller 1 til 1584 pt.', 'Gib einen Zeilenabstand ein: 0,5 bis 10 Zeilen oder 1 bis 1584 pt.', 'Εισαγάγετε ένα διάστιχο: 0,5 έως 10 γραμμές ή 1 έως 1584 pt.', 'Enter a line spacing: 0.5 to 10 lines, or 1 to 1584 pt.', 'Introduce un interlineado: de 0,5 a 10 líneas o de 1 a 1584 pt.', 'Saisissez un interligne : 0,5 à 10 lignes ou 1 à 1584 pt.', 'पंक्ति रिक्ति दर्ज करें: 0.5 से 10 पंक्तियाँ, या 1 से 1584 pt।', 'Inserisci un’interlinea: da 0,5 a 10 righe oppure da 1 a 1584 pt.', '行間を入力してください: 0.5～10 行、または 1～1584 pt。', 'Voer een regelafstand in: 0,5 tot 10 regels of 1 tot 1584 pt.', 'Angi en linjeavstand: 0,5 til 10 linjer eller 1 til 1584 pt.', 'Wprowadź interlinię: od 0,5 do 10 wierszy lub od 1 do 1584 pt.', 'Introduza um espaçamento entre linhas: 0,5 a 10 linhas ou 1 a 1584 pt.', 'Ange ett radavstånd: 0,5 till 10 rader eller 1 till 1584 pt.', '请输入行距：0.5 到 10 行，或 1 到 1584 pt。'],
    field_page: ['Číslo stránky', 'Sidetal', 'Seitenzahl', 'Αριθμός σελίδας', 'Page number', 'Número de página', 'Numéro de page', 'पृष्ठ संख्या', 'Numero di pagina', 'ページ番号', 'Paginanummer', 'Sidetall', 'Numer strony', 'Número de página', 'Sidnummer', '页码'],
    field_pages: ['Počet stránek', 'Antal sider', 'Seitenanzahl', 'Αριθμός σελίδων', 'Number of pages', 'Número de páginas', 'Nombre de pages', 'पृष्ठों की संख्या', 'Numero di pagine', 'ページ数', 'Aantal pagina’s', 'Antall sider', 'Liczba stron', 'Número de páginas', 'Antal sidor', '页数'],
    field_section_pages: ['Stránky v oddílu', 'Sider i sektionen', 'Seiten im Abschnitt', 'Σελίδες στην ενότητα', 'Pages in section', 'Páginas de la sección', 'Pages de la section', 'अनुभाग के पृष्ठ', 'Pagine nella sezione', 'セクションのページ数', 'Pagina’s in sectie', 'Sider i inndelingen', 'Strony w sekcji', 'Páginas na secção', 'Sidor i avsnittet', '本节页数'],
    field_page_of: ['Stránka X z Y', 'Side X af Y', 'Seite X von Y', 'Σελίδα X από Y', 'Page X of Y', 'Página X de Y', 'Page X sur Y', 'पृष्ठ X का Y', 'Pagina X di Y', 'X / Y ページ', 'Pagina X van Y', 'Side X av Y', 'Strona X z Y', 'Página X de Y', 'Sida X av Y', '第 X 页，共 Y 页'],
    insert_date: ['Datum', 'Dato', 'Datum', 'Ημερομηνία', 'Date', 'Fecha', 'Date', 'दिनांक', 'Data', '日付', 'Datum', 'Dato', 'Data', 'Data', 'Datum', '日期'],
    fields: ['Pole', 'Felter', 'Felder', 'Πεδία', 'Fields', 'Campos', 'Champs', 'फ़ील्ड', 'Campi', 'フィールド', 'Velden', 'Felt', 'Pola', 'Campos', 'Fält', '域'],
};
for (const [name, texts] of Object.entries(KEYS)) if (texts.length !== LANGS.length) throw new Error(name + ' has ' + texts.length + ' texts');
for (const [index, lang] of LANGS.entries()) {
    const file = path.join(root, 'ui/lang/desktop', lang + '.json');
    const lines = readFileSync(file, 'utf8').split('\n');
    const indent = /^(\s*)"desktop\.writer_/.exec(lines.find(line => /^\s*"desktop\.writer_/.test(line)))[1];
    let added = 0, stripped = 0;
    for (const [name, texts] of Object.entries(KEYS)) {
        const key = 'desktop.writer_' + name;
        const block = lines.map((line, at) => [/^\s*"(desktop\.writer_[a-z0-9_]+)":/.exec(line)?.[1], at]).filter(([found]) => found);
        if (block.some(([found]) => found === key)) continue;
        const next = block.find(([found]) => found > key);
        lines.splice(next ? next[1] : block.at(-1)[1] + 1, 0, indent + JSON.stringify(key) + ': ' + JSON.stringify(texts[index]) + ',');
        added++;
    }
    for (const name of UNITLESS) {
        const at = lines.findIndex(line => line.startsWith(indent + '"desktop.writer_' + name + '":'));
        if (at < 0) throw new Error(lang + ' lacks ' + name);
        const plain = lines[at].replace(/ · mm(",?\r?)$/, '$1');
        if (plain === lines[at] && /mm/.test(lines[at])) throw new Error(lang + ': unexpected unit in ' + lines[at]);
        if (plain !== lines[at]) { lines[at] = plain; stripped++; }
    }
    const text = lines.join('\n');
    JSON.parse(text);
    writeFileSync(file, text);
    console.log(lang + ': ' + added + ' keys added, ' + stripped + ' labels without unit');
}
```

Run from the repo root: `node disposable/_writer-wave2a/i18n.mjs .`

Expected: sixteen lines, each `<lang>: 47 keys added, 7 labels without unit`; `git diff --stat -- ui/lang/desktop` shows 16 files with 54 insertions and 7 deletions each. A second run prints `0 keys added, 0 labels without unit` sixteen times. Then delete `disposable/_writer-wave2a/`.

- [ ] **Step 2: Check and commit**

Run: `npm run test:writer` (the commands line still passes; the new keys are not used yet) and `go test ./ui -run "(?i)i18n" -count=1`.

```
git add ui/lang/desktop
git diff --cached --check
git commit
```

Message:

```
feat(writer): texts for page setup, the paragraph dialog and page fields

47 keys in sixteen languages. Seven length labels lose their "· mm":
the Format panel names the unit of the user's locale itself.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: Claude Fable 5.2 <noreply@anthropic.com>
```

Between this commit and Task 4 the Format panel shows seven lengths without a unit. Tasks 3 and 4 land together.

---

### Task 4: Wiring

**Files:**
- Modify: `ui/js/desktop/apps/writer-commands.js`, `writer-menus.js`, `writer-context.js`, `writer-dialogs.js`, `writer-panels.js`, `writer.js`
- Modify: `ui/js/desktop/core/module-loader.js`, `ui/js/desktop/bundles/main.bundle.js` (rebuilt)
- Modify: `scripts/test-writer-menus.mjs`, `scripts/test-writer-context.mjs`, `scripts/test-writer-session.mjs`
- Modify: `ui/desktop_office_assets_test.go`, `ui/desktop_writer_browser_test.go`, `ui/desktop_writer_reload_browser_test.go`

- [ ] **Step 1: Extend the node tests**

In `scripts/test-writer-menus.mjs` replace

```js
assert.equal(menus[0].items[2].items[0].id, 'recent-0', 'the recent documents are a submenu');
```
with
```js
assert.equal(menus[0].items[2].items[0].id, 'recent-0', 'the recent documents are a submenu');
assert.ok(menus[0].items.some(item => item.id === 'pageSetup'), 'Page setup is in File');
const formatMenu = menus.find(menu => menu.id === 'format');
assert.equal(formatMenu.items.filter(item => item.id === 'paragraphFormat' || item.id === 'pageSetup').map(item => item.id).join(), 'paragraphFormat,pageSetup', 'the two dialogs are in Format');
const fields = menus.find(menu => menu.id === 'insert').items.find(item => item.id === 'fields');
assert.equal(fields.items.map(item => item.id || '-').join(), 'fieldPage,pageNumber,fieldPages,fieldSectionPages,-,insertDate', 'the page fields and the date are a submenu of Insert');
```

In `scripts/test-writer-context.mjs` replace

```js
assert.equal(plain.filter(section => section.length).map(section => section.join()).join(' / '), 'cut,copy,paste,pastePlain / link / comment / assist');
```
with
```js
assert.equal(plain.filter(section => section.length).map(section => section.join()).join(' / '), 'cut,copy,paste,pastePlain / link / comment / paragraphFormat / assist');
assert.equal(WriterContext.bars.headerFooter.items.join(), 'fieldPage,pageNumber,fieldPages,insertDate,|,headerFirstPage,headerOddEven,|,backDocument');
```

In `scripts/test-writer-session.mjs` the paper sizes are tested by the layout test now. Replace

```js
const panels={window:{}};
vm.runInNewContext(readFileSync('ui/js/desktop/apps/writer-panels.js','utf8'),panels);
const paperSize=panels.window.WriterPanels.paperSize;
assert.equal(paperSize(12240,15840),'letter');
assert.equal(paperSize(15840,12240),'letter');
assert.equal(paperSize(12259,15840),'letter');
assert.equal(paperSize(12260,15840),'');
assert.equal(paperSize(11906,16838),'a4');
assert.equal(paperSize(16838,11906),'a4');
assert.equal(paperSize(11925,16838),'a4');
assert.equal(paperSize(9000,9000),'');
console.log('Writer paper sizes: Letter and A4 match both orientations');
```
with
```js
// Paper sizes are tested with the layout model in scripts/test-writer-layout.mjs.
```

Run: `node scripts/test-writer-menus.mjs` and `node scripts/test-writer-context.mjs`
Expected: the first fails with `Page setup is in File`, the second with an assertion on the right-click sections.

- [ ] **Step 2: The registry**

In `ui/js/desktop/apps/writer-commands.js` replace

```js
        {id: 'pageNumber', slot: 'insert.pageXofY', label: 'page_numbers'},
```
with
```js
        {id: 'pageNumber', slot: 'insert.pageXofY', label: 'field_page_of'},
        {id: 'fieldPage', command: {type: 'insertPageField', field: 'PAGE'}, label: 'field_page'},
        {id: 'fieldPages', command: {type: 'insertPageField', field: 'NUMPAGES'}, label: 'field_pages'},
        {id: 'fieldSectionPages', command: {type: 'insertPageField', field: 'SECTIONPAGES'}, label: 'field_section_pages'},
        {id: 'insertDate', label: 'insert_date', write: true, needsEditor: true},
        {id: 'pageSetup', label: 'page_setup_dialog', write: true, needsEditor: true, untracked: 'setPageSetup'},
        {id: 'paragraphFormat', label: 'paragraph_dialog', write: true, needsEditor: true},
```

- [ ] **Step 3: Menus, right-click menu, header/footer bar**

In `ui/js/desktop/apps/writer-menus.js` replace

```js
                item('docx'),item('html'),item('md'),item('txt'),SEP,item('print')]},
```
with
```js
                item('docx'),item('html'),item('md'),item('txt'),SEP,item('pageSetup'),item('print')]},
```

Replace

```js
                item('header'),item('footer'),item('pageNumber'),item('headerFirstPage'),item('headerOddEven'),item('backDocument')]},
```
with
```js
                item('header'),item('footer'),{id:'fields',label:tr('fields'),items:[item('fieldPage'),item('pageNumber'),item('fieldPages'),item('fieldSectionPages'),SEP,item('insertDate')]},
                item('headerFirstPage'),item('headerOddEven'),item('backDocument')]},
```

Replace

```js
                {id:'image',label:tr('image'),items:[item('rotateLeft'),item('rotateRight'),SEP,item('deleteImage')]},SEP,
```
with
```js
                {id:'image',label:tr('image'),items:[item('rotateLeft'),item('rotateRight'),SEP,item('deleteImage')]},SEP,
                item('paragraphFormat'),item('pageSetup'),SEP,
```

In `ui/js/desktop/apps/writer-context.js` replace

```js
        headerFooter: {items: ['pageNumber', 'headerFirstPage', 'headerOddEven', '|', 'backDocument'], text: ['pageNumber', 'headerFirstPage', 'headerOddEven', 'backDocument']}
```
with
```js
        headerFooter: {items: ['fieldPage', 'pageNumber', 'fieldPages', 'insertDate', '|', 'headerFirstPage', 'headerOddEven', '|', 'backDocument'],
            text: ['fieldPage', 'pageNumber', 'fieldPages', 'insertDate', 'headerFirstPage', 'headerOddEven', 'backDocument']}
```

Replace

```js
            ['comment'],
            target.table ? [{submenu: 'table', items: BARS.table.items}] : [],
```
with
```js
            ['comment'],
            ['paragraphFormat'],
            target.table ? [{submenu: 'table', items: BARS.table.items}] : [],
```

Run: `node scripts/test-writer-menus.mjs` and `node scripts/test-writer-context.mjs`
Expected: both pass.

- [ ] **Step 4: The two dialogs**

In `ui/js/desktop/apps/writer-dialogs.js` replace

```js
        return {table, link, goToPage, wordCount, shortcuts, comment, drafts};
```
with
```js
        // The layout dialogs. WriterLayout turns the engine's state into the texts the fields show
        // and the answer into one command with only what changed; lengths carry the user's unit.
        const L = window.WriterLayout;
        const unit = () => tr(app.units.unit === 'in' ? 'unit_in' : 'unit_cm');
        const length = (id, label, values) => ({id, label, value: values[id], suffix: unit(), inputmode: 'decimal', maxlength: 8, half: true});
        // A select; a value the selection disagrees on shows as a dash that sends nothing.
        const choice = (id, label, values, pairs, half) => ({id, type: 'select', label, value: values[id], half,
            options: (values[id] === '' ? [['', '—']] : []).concat(pairs).map(([value, text]) => ({value, label: text}))});
        const check = (id, label, values) => ({id, type: 'checkbox', label, value: values[id] === true, mixed: values[id] === null, half: true});
        const layout = (title, fields, model, command) => ask(title, fields, {wide: true,
            onInput: (changed, values, set) => { for (const [id, value] of Object.entries(model(changed, values, app.units))) set(id, value); },
            validate: values => { const {error} = command(values); return error ? tr(error) : ''; }});
        async function page() {
            const editor = app.editor, setup = editor?.getPageSetup();
            if (!setup) return;
            const initial = L.pageFields(setup, app.units), command = values => L.pageCommand(values, initial, app.units);
            const answer = await layout(tr('page_setup'), [
                choice('paper', tr('paper'), initial, L.papers.map(paper => [paper.id, paper.name]).concat([['custom', tr('paper_custom')]])),
                length('width', tr('page_width'), initial), length('height', tr('page_height'), initial),
                choice('orientation', tr('orientation'), initial, [['portrait', tr('portrait')], ['landscape', tr('landscape')]]),
                {id: 'marginsHeading', type: 'heading', label: tr('margins')},
                choice('preset', tr('margin_preset'), initial, [['normal', tr('margins_normal')], ['narrow', tr('margins_narrow')], ['wide', tr('margins_wide')], ['custom', tr('margins_custom')]]),
                length('top', tr('margin_top'), initial), length('bottom', tr('margin_bottom'), initial),
                length('left', tr('margin_left'), initial), length('right', tr('margin_right'), initial),
                choice('scope', tr('apply_to'), initial, [['document', tr('apply_document')], ['section', tr('apply_section')]])
            ], L.pageInput, command);
            if (!answer || app.editor !== editor) return;
            const result = command(answer);
            if (result.command) commands.run(result.command);
        }
        async function paragraph() {
            const editor = app.editor;
            if (!editor) return;
            const u = app.units, initial = L.paragraphFields(editor.snapshot().formatting, u), command = values => L.paragraphCommand(values, initial, u);
            const answer = await layout(tr('paragraph'), [
                choice('alignment', tr('alignment'), initial, [['left', tr('align_left')], ['center', tr('align_center')], ['right', tr('align_right')], ['justify', tr('justify')]]),
                {id: 'indentHeading', type: 'heading', label: tr('indent_heading')},
                length('left', tr('left_indent'), initial), length('right', tr('right_indent'), initial),
                choice('special', tr('indent_special'), initial, [['none', tr('indent_none')], ['first', tr('indent_first')], ['hanging', tr('indent_hanging')]], true),
                length('by', tr('indent_by'), initial),
                {id: 'spacingHeading', type: 'heading', label: tr('spacing_heading')},
                {id: 'before', label: tr('before_spacing'), value: initial.before, inputmode: 'decimal', maxlength: 8, half: true},
                {id: 'after', label: tr('after_spacing'), value: initial.after, inputmode: 'decimal', maxlength: 8, half: true},
                choice('lineRule', tr('line_spacing'), initial, [['single', tr('line_single')], ['115', u.number(1.15)], ['150', u.number(1.5)], ['double', tr('line_double')],
                    ['atLeast', tr('line_at_least')], ['exact', tr('line_exact')], ['multiple', tr('line_multiple')]], true),
                {id: 'lineValue', label: tr('line_value'), value: initial.lineValue, inputmode: 'decimal', maxlength: 8, half: true},
                {id: 'breaksHeading', type: 'heading', label: tr('breaks_heading')},
                check('keepNext', tr('keep_next'), initial), check('keepLines', tr('keep_lines'), initial),
                check('widowControl', tr('widow_control'), initial), check('pageBreakBefore', tr('page_break_before'), initial)
            ], L.paragraphInput, command);
            if (!answer || app.editor !== editor) return;
            const result = command(answer);
            if (result.command) commands.run(result.command);
        }
        return {table, link, goToPage, wordCount, shortcuts, comment, drafts, page, paragraph};
```

- [ ] **Step 5: The Format panel**

All replacements are in `ui/js/desktop/apps/writer-panels.js`.

Replace

```js
    function paperSize(widthTwips, heightTwips) {
        const near = (width, height, expectedWidth, expectedHeight) => Math.abs(width - expectedWidth) < 20 && Math.abs(height - expectedHeight) < 20;
        if (near(widthTwips, heightTwips, 12240, 15840) || near(widthTwips, heightTwips, 15840, 12240)) return 'letter';
        if (near(widthTwips, heightTwips, 11906, 16838) || near(widthTwips, heightTwips, 16838, 11906)) return 'a4';
        return '';
    }
    function create(app) {
        const {root,find,tr,esc}=app, life=new AbortController();
```
with
```js
    function create(app) {
        const {root,find,tr,esc}=app, life=new AbortController();
        // Lengths carry the unit of the user's locale; the papers are the layout module's.
        const L=window.WriterLayout, u=app.units, unitName=tr(u.unit==='in'?'unit_in':'unit_cm');
```

Replace

```js
        const section=(label,body)=>'<section class="writer-panel-section"><h3>'+esc(tr(label))+'</h3>'+body+'</section>';
```
with
```js
        const section=(label,body)=>'<section class="writer-panel-section"><h3>'+esc(tr(label))+'</h3>'+body+'</section>';
        const length=(label,key)=>'<label class="writer-field"><span>'+esc(tr(label)+' · '+unitName)+'</span>'+input(key,'','text','inputmode="decimal" maxlength="8"')+'</label>';
        // Opens the dialog that holds everything a section's quick fields leave out.
        const more=action=>'<div class="writer-buttonrow"><button type="button" data-action="'+action+'">'+esc(tr('more'))+'</button></div>';
```

Replace

```js
                    field('left_indent',input('leftIndent',0,'number','min="0" max="100"'))+field('first_indent',input('firstIndent',0,'number','min="-100" max="100"')))+
                    section('page_setup',field('paper',select('paper',[['a4','a4'],['letter','letter_paper']],'<option value="custom" disabled>'+esc(tr('paper_custom'))+'</option>'))+
                    field('orientation',select('orientation',[['portrait','portrait'],['landscape','landscape']]))+
                    ['Top','Bottom','Left','Right'].map(x=>field('margin_'+x.toLowerCase(),input('margin'+x,25,'number','min="0" max="100"'))).join(''))+
```
with
```js
                    length('left_indent','leftIndent')+length('first_indent','firstIndent')+more('paragraphFormat'))+
                    section('page_setup',field('paper','<select data-field="paper">'+L.papers.map(paper=>'<option value="'+paper.id+'">'+esc(paper.name)+'</option>').join('')+'<option value="custom" disabled>'+esc(tr('paper_custom'))+'</option></select>')+
                    field('orientation',select('orientation',[['portrait','portrait'],['landscape','landscape']]))+
                    ['Top','Bottom','Left','Right'].map(x=>length('margin_'+x.toLowerCase(),'margin'+x)).join('')+more('pageSetup'))+
```

Replace

```js
                    section('image',field('image_width',input('imageWidth',80,'number','min="1" max="300"'))+field('alt_text',input('imageAlt','','text'))+
```
with
```js
                    section('image',length('image_width','imageWidth')+field('alt_text',input('imageAlt','','text'))+
```

Replace

```js
                const setup=snapshot.pageSetup, image=snapshot.image;
                const mm=twips=>Math.round(Number(twips)*25.4/1440);
                const indent=fmt.indent;
                const fields={lineSpacing:fmt.lineSpacing?.value || 1.15,beforePt:fmt.spaceBeforePt || 0,afterPt:fmt.spaceAfterPt || 0,
                    orientation:setup.orientation,paper:paperSize(setup.pageWidthTwips,setup.pageHeightTwips) || 'custom',
                    leftIndent:indent && !indent.mixed?.left ? mm(indent.left) : '',
                    firstIndent:indent && !indent.mixed?.firstLine ? mm(indent.firstLine) : ''};
```
with
```js
                const setup=snapshot.pageSetup, image=snapshot.image;
                const indent=fmt.indent;
                const fields={lineSpacing:fmt.lineSpacing?.value || 1.15,beforePt:fmt.spaceBeforePt || 0,afterPt:fmt.spaceAfterPt || 0,
                    orientation:setup.orientation,paper:L.paperOf(setup.pageWidthTwips,setup.pageHeightTwips),
                    leftIndent:indent && !indent.mixed?.left ? u.show(indent.left) : '',
                    firstIndent:indent && !indent.mixed?.firstLine ? u.show(indent.firstLine) : ''};
```

Replace

```js
                        if(Number.isFinite(selected.widthEmu) && selected.widthEmu>0)fields.imageWidth=Math.round(selected.widthEmu/36000);
```
with
```js
                        if(Number.isFinite(selected.widthEmu) && selected.widthEmu>0)fields.imageWidth=u.show(selected.widthEmu/635);
```

Replace

```js
                for(const direction of ['Top','Bottom','Left','Right'])fields['margin'+direction]=Math.round(setup.marginsTwips[direction.toLowerCase()]*25.4/1440);
```
with
```js
                for(const direction of ['Top','Bottom','Left','Right'])fields['margin'+direction]=u.show(setup.marginsTwips[direction.toLowerCase()]);
```

The "More…" buttons follow the registry. Replace

```js
            if(right==='review') {
                const mode=find('[data-field="tracking"]');if(mode)mode.value=snapshot.editingMode;
```
with
```js
            if(right==='format')for(const id of ['pageSetup','paragraphFormat']) {
                const el=root.querySelector('.writer-right [data-action="'+id+'"]'),state=app.commands.state(id);
                if(el){el.disabled=!state.enabled;el.title=state.reason || '';}
            }
            if(right==='review') {
                const mode=find('[data-field="tracking"]');if(mode)mode.value=snapshot.editingMode;
```

Replace

```js
            if(key==='leftIndent' || key==='firstIndent')app.run({type:'setIndent',[key==='leftIndent'?'left':'firstLine']:Math.round(n*1440/25.4)});
            if(key?.startsWith('margin'))app.run({type:'setPageSetup',[key]:Math.round(n*1440/25.4),scope:'document'});
```
with
```js
            const twips=u.read(v);
            if((['leftIndent','firstIndent','imageWidth'].includes(key) || key?.startsWith('margin')) && !Number.isFinite(twips)){app.notice(tr('length_invalid'),true);return;}
            if(key==='leftIndent' || key==='firstIndent')app.run({type:'setIndent',[key==='leftIndent'?'left':'firstLine']:twips});
            if(key?.startsWith('margin'))app.run({type:'setPageSetup',[key]:twips,scope:'document'});
```

Replace

```js
                if(v==='custom')return;
                let [pageWidth,pageHeight]=v==='letter'?[12240,15840]:[11906,16838];
```
with
```js
                const paper=L.papers.find(candidate=>candidate.id===v);
                if(!paper)return;
                let pageWidth=paper.width,pageHeight=paper.height;
```

Replace

```js
                if(image)app.run({type:'setImageProperties',widthEmu:n*36000,heightEmu:n*36000*image.heightEmu/image.widthEmu});
```
with
```js
                if(image && twips>0)app.run({type:'setImageProperties',widthEmu:twips*635,heightEmu:Math.round(twips*635*image.heightEmu/image.widthEmu)});
```

Replace

```js
    window.WriterPanels={create,paperSize};
```
with
```js
    window.WriterPanels={create};
```

- [ ] **Step 6: `writer.js`**

Replace

```js
            windowId,signal:()=>documentLife.signal,countWords:text=>countWords(text),menuSignature,fileMenuItems,recentMenuItems,restoreDraft,
```
with
```js
            windowId,signal:()=>documentLife.signal,countWords:text=>countWords(text),menuSignature,fileMenuItems,recentMenuItems,restoreDraft,
            units:window.WriterLayout.units(locale,navigator.language),
```

Replace

```js
                    case 'comment':await dialogs.comment();break;
```
with
```js
                    case 'comment':await dialogs.comment();break;
                    case 'pageSetup':await dialogs.page();break;
                    case 'paragraphFormat':await dialogs.paragraph();break;
                    // The engine has no date field; today's date goes in as text.
                    case 'insertDate':run({type:'paste',text:new Date().toLocaleDateString(locale,{dateStyle:'long'})});break;
```

- [ ] **Step 7: Register the new script**

`writer-layout.js` loads before `writer-commands.js` everywhere.

In `ui/js/desktop/core/module-loader.js` replace

```js
'/js/desktop/apps/writer-session.js', '/js/desktop/apps/writer-commands.js',
```
with
```js
'/js/desktop/apps/writer-session.js', '/js/desktop/apps/writer-layout.js', '/js/desktop/apps/writer-commands.js',
```

In `ui/desktop_office_assets_test.go` replace

```go
		"/js/desktop/apps/writer-commands.js",
```
with
```go
		"/js/desktop/apps/writer-layout.js",
		"/js/desktop/apps/writer-commands.js",
```

In `ui/desktop_writer_browser_test.go` (two fixtures) and `ui/desktop_writer_reload_browser_test.go` (one fixture), replace every occurrence of

```go
<script src="/js/desktop/apps/writer-session.js"></script><script src="/js/desktop/apps/writer-commands.js"></script>
```
by

```go
<script src="/js/desktop/apps/writer-session.js"></script><script src="/js/desktop/apps/writer-layout.js"></script><script src="/js/desktop/apps/writer-commands.js"></script>
```

Three occurrences in total.

The file-ops check reads the left indent from the Format panel. In `ui/desktop_writer_browser_test.go` replace

```go
        if(Number(indent)!==Math.round(left*25.4/1440))throw Error(
```
with
```go
        if(indent!==WriterLayout.units(document.documentElement.lang,navigator.language).show(left))throw Error(
```

Run: `npm run build:ui`
Expected: `git diff --stat -- ui/js/desktop/bundles` shows only `main.bundle.js`, 1 insertion and 1 deletion. If the bundle shows more, stop and report.

- [ ] **Step 8: Static checks and commit**

Run:

- `node --check` on `writer.js`, `writer-dialogs.js`, `writer-panels.js`, `writer-menus.js`, `writer-context.js`, `writer-commands.js`, `writer-layout.js`
- `npm run test:writer`
- `npm run check:ui`
- `gofmt -l ui/desktop_writer_browser_test.go ui/desktop_writer_reload_browser_test.go ui/desktop_office_assets_test.go` (prints nothing)
- `go test ./ui -run "(?i)(windowmenu|contextmenu|materials|office|writer|virtualdesktop|lifecycle|i18n|feeling)" -count=1`

Expected: all pass.

```
git add ui/js/desktop/apps/writer.js ui/js/desktop/apps/writer-commands.js ui/js/desktop/apps/writer-menus.js ui/js/desktop/apps/writer-context.js ui/js/desktop/apps/writer-dialogs.js ui/js/desktop/apps/writer-panels.js ui/js/desktop/core/module-loader.js ui/js/desktop/bundles/main.bundle.js scripts/test-writer-menus.mjs scripts/test-writer-context.mjs scripts/test-writer-session.mjs ui/desktop_office_assets_test.go ui/desktop_writer_browser_test.go ui/desktop_writer_reload_browser_test.go
git diff --cached --check
git commit
```

Message:

```
feat(writer): Page setup and Paragraph dialogs, page fields, one length unit

File and Format open Page setup (six papers, custom size, orientation,
margin presets, whole document or this section) and Paragraph
(alignment, indents, spacing, line spacing, keep options) as one undo
step each. Insert > Fields and the header/footer bar offer the page
number, page X of Y, the page count and today's date. The Format
panel shows lengths in the unit of the user's locale.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: Claude Fable 5.2 <noreply@anthropic.com>
```

---

### Task 5: Browser check

**Files:**
- Create: `ui/desktop_writer_layout_test.go`
- Modify: `ui/desktop_writer_browser_test.go`

- [ ] **Step 1: The check**

Create `ui/desktop_writer_layout_test.go`:

```go
package ui

import (
	"testing"

	"github.com/go-rod/rod"
)

// verifyWriterLayout drives the Page setup and Paragraph dialogs, the Format panel's lengths and the
// page fields on the blank document and leaves it blank. Lengths are computed in the unit the page
// itself uses, so the check holds in centimetres and in inches.
func verifyWriterLayout(t *testing.T, page *rod.Page) {
	t.Helper()
	page.MustEval(`async()=>{` + writerWave1Prelude + `
        const u=WriterLayout.units(document.documentElement.lang,navigator.language);
        const type=(id,value)=>{const el=field(id);el.value=value;el.dispatchEvent(new Event('input',{bubbles:true}));};
        const error=()=>{const node=dialog().querySelector('[data-form-error]');return node.hidden?'':node.textContent;};
        const opened=!!document.querySelector('.writer-right [data-field="marginTop"]');
        try {
            // Page setup: fields follow each other, an invalid length is refused, one command lands.
            void app.act('pageSetup');await wait(250);
            if(!dialog())throw Error('page setup did not open');
            if([field('paper').value,field('orientation').value,field('preset').value,field('scope').value].join()!=='a4,portrait,normal,document')throw Error('page setup shows '+[field('paper').value,field('orientation').value,field('preset').value,field('scope').value]);
            if(field('top').value!==u.show(1417)||!dialog().textContent.includes(u.unit==='in'?'in':'cm'))throw Error('page setup margins show '+field('top').value);
            type('paper','a5');
            if(field('width').value!==u.show(8391)||field('height').value!==u.show(11906))throw Error('A5 shows '+field('width').value+' x '+field('height').value);
            type('orientation','landscape');
            if(field('width').value!==u.show(11906)||field('height').value!==u.show(8391))throw Error('landscape shows '+field('width').value+' x '+field('height').value);
            type('preset','narrow');
            if(field('left').value!==u.show(720))throw Error('narrow margins show '+field('left').value);
            type('left','wide');submit();await wait(120);
            if(!dialog()||!error())throw Error('an invalid margin was accepted');
            type('left',u.show(1134));
            if(field('preset').value!=='custom')throw Error('a typed margin left the preset at '+field('preset').value);
            submit();await wait(350);
            if(dialog())throw Error('page setup stayed open: '+error());
            if(failed())throw Error('page setup: '+failed());
            const setup=editor.getPageSetup();
            if(setup.pageWidthTwips!==11906||setup.pageHeightTwips!==8391||setup.orientation!=='landscape'||setup.marginsTwips.top!==720||setup.marginsTwips.left!==u.read(u.show(1134)))throw Error('page setup gave '+JSON.stringify(setup));
            // The Format panel shows the same lengths in the same unit and takes them back.
            if(!document.querySelector('.writer-right [data-field="marginTop"]'))await app.act('format');
            await wait(200);
            const panel=key=>document.querySelector('.writer-right [data-field="'+key+'"]');
            if(panel('marginTop').value!==u.show(720)||panel('paper').value!=='a5')throw Error('the panel shows '+panel('marginTop').value+' '+panel('paper').value);
            if(!panel('marginTop').closest('label').textContent.includes(u.unit==='in'?'in':'cm'))throw Error('the panel names no unit: '+panel('marginTop').closest('label').textContent);
            panel('marginTop').value=u.show(1417);panel('marginTop').dispatchEvent(new Event('change',{bubbles:true}));await wait(250);
            if(editor.getPageSetup().marginsTwips.top!==u.read(u.show(1417)))throw Error('the panel margin gave '+editor.getPageSetup().marginsTwips.top);
            for(const id of ['pageSetup','paragraphFormat'])if(!document.querySelector('.writer-right [data-action="'+id+'"]'))throw Error('the panel has no button for '+id);
            // Paragraph: several fields in one undo step; an untouched dialog sends nothing.
            editor.exec({type:'paste',text:'Layout paragraph'});
            void app.act('paragraphFormat');await wait(250);
            if(!dialog())throw Error('the paragraph dialog did not open');
            if([field('alignment').value,field('special').value,field('keepNext').checked,field('widowControl').checked].join()!=='left,none,false,true')throw Error('the paragraph dialog shows '+[field('alignment').value,field('special').value,field('keepNext').checked,field('widowControl').checked]);
            type('alignment','justify');type('left',u.show(567));type('special','hanging');type('by',u.show(283));type('before','6');type('lineRule','exact');type('lineValue','18');
            field('keepNext').click();
            submit();await wait(350);
            if(dialog())throw Error('the paragraph dialog stayed open: '+error());
            if(failed())throw Error('paragraph: '+failed());
            const f=editor.snapshot().formatting;
            if(f.alignment!=='both'||f.indent.left!==u.read(u.show(567))||f.indent.firstLine!==-u.read(u.show(283))||f.spaceBeforePt!==6||f.lineSpacing.rule!=='exact'||f.lineSpacing.value!==18||f.paragraphFlags.keepNext!==true||f.paragraphFlags.keepLines!==false)
                throw Error('paragraph gave '+JSON.stringify({alignment:f.alignment,indent:f.indent,before:f.spaceBeforePt,line:f.lineSpacing,flags:f.paragraphFlags}));
            editor.exec({type:'undo'});
            const back=editor.snapshot().formatting;
            if(back.alignment!=='left'||back.paragraphFlags.keepNext!==false||back.indent.left!==0)throw Error('the paragraph dialog is more than one undo step');
            const revision=app.session.revision;
            void app.act('paragraphFormat');await wait(250);
            submit();await wait(250);
            if(dialog()||app.session.revision!==revision)throw Error('an untouched paragraph dialog changed the document');
            // Page fields and the date, from the footer bar.
            if(!editor.exec({type:'editHeaderFooter',position:'footer'}).ok)throw Error('the footer did not open');
            await wait(350);
            const bar=document.querySelector('[data-contextbar]'),tools=[...bar.querySelectorAll('button[data-tool]')].map(button=>button.dataset.tool).join();
            if(tools!=='fieldPage,pageNumber,fieldPages,insertDate,headerFirstPage,headerOddEven,backDocument')throw Error('the footer bar holds '+tools);
            for(const id of ['fieldPage','fieldPages','insertDate']) {
                const before=app.session.revision;
                bar.querySelector('[data-tool="'+id+'"]').click();await wait(250);
                if(failed())throw Error(id+': '+failed());
                if(app.session.revision===before)throw Error(id+' changed nothing');
            }
            bar.querySelector('[data-tool="backDocument"]').click();await wait(350);
            if(editor.snapshot().scope.kind!=='body')throw Error('the footer stayed open');
            const fields=window.menus.find(menu=>menu.id==='insert').items.find(item=>item.id==='fields');
            if(!fields||!fields.items.find(item=>item.id==='fieldPage').disabled||fields.items.find(item=>item.id==='insertDate').disabled)throw Error('in the body the page fields must be off and the date on');
        } finally {
            dialog()?.querySelector('[data-form-cancel]')?.click();
            if(editor.snapshot().scope.kind!=='body')editor.exec({type:'exitHeaderFooter'});
            rewind();
            if(!!document.querySelector('.writer-right [data-field="marginTop"]')!==opened)await app.act('format');
        }
        await wait(200);
        if(JSON.stringify(editor.getPageSetup())!==paper)throw Error('the layout check left the page as '+JSON.stringify(editor.getPageSetup()));
    }`)
}
```

In `ui/desktop_writer_browser_test.go` replace

```go
	verifyWriterContext(t, page)
	result := page.MustEval(`async()=>{
```
with
```go
	verifyWriterContext(t, page)
	verifyWriterLayout(t, page)
	result := page.MustEval(`async()=>{
```

- [ ] **Step 2: Compile, then the controller runs the browser tests**

Run: `gofmt -l ui/desktop_writer_layout_test.go ui/desktop_writer_browser_test.go` (prints nothing) and `go vet ./ui`.

The controller then deletes `reports\autor` and runs `AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run "TestDesktop(Writer|QR)" -count=1 -timeout 30m -v`.
Expected: every `TestDesktopWriter*` passes. A failure in `verifyWriterLayout` names the step; fix the cause, not the check. The controller also screenshots both dialogs with the shell's stylesheets in the dark theme and looks at them: two columns, the unit after every length, the headings, and no clipped text.

- [ ] **Step 3: Commit**

```
git add ui/desktop_writer_layout_test.go ui/desktop_writer_browser_test.go
git diff --cached --check
git commit
```

Message:

```
test(writer): page setup, paragraph dialog, panel lengths and page fields in the browser

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: Claude Fable 5.2 <noreply@anthropic.com>
```

---

### Task 6: Documentation

**Files:**
- Modify: `ui/js/desktop/apps/AGENTS.md`, `documentation/autor.md`, `docs/superpowers/specs/2026-11-12-writer-office-roadmap-design.md`

- [ ] **Step 1: `ui/js/desktop/apps/AGENTS.md`**

In the Writer section, directly before the line that starts with `` `writer-session.js` owns revision-bound autosave``, add:

```text
  `writer-layout.js` is the pure layout model: `units(locale, region)` gives centimetres, or
  inches for an English interface in a US browser, with comma or point on input; `papers` and
  `margins` are the catalogue; `pageFields`/`pageInput`/`pageCommand` and
  `paragraphFields`/`paragraphInput`/`paragraphCommand` turn the engine's state into the texts a
  dialog shows, keep dependent fields in step and build ONE command that carries only what the
  user changed, so an untouched dialog sends nothing and a mixed value is left alone. A length
  that shows like a paper edge or a preset margin is that exact value. `page()` and `paragraph()`
  in `writer-dialogs.js` build the dialogs from it; the Format panel reads and writes its lengths
  through the same unit and names it in the label. Tab stops and header/footer distance are not
  offered. In the form dialog `heading`, `half`, `suffix`, a `mixed` checkbox and
  `onInput(changed, values, set)` exist for these dialogs. Browser checks never hold a length as
  a literal text: the fixtures run in inches;
```

- [ ] **Step 2: `documentation/autor.md`**

Add this section directly before the heading `## Files and recovery`:

```text
## Page and paragraph

**Format > Page setup…** (also in the File menu and behind "More…" in the Format
panel) sets the paper (A3, A4, A5, B5, Letter, Legal or a custom size), the
orientation and the margins, with presets for normal, narrow and wide margins.
"Apply to" changes the whole document or only the section the caret is in.
Page setup is not available while changes are tracked.

**Format > Paragraph…** (also in the right-click menu and behind "More…" in the
Format panel) sets alignment, left and right indent, a first-line or hanging
indent, spacing before and after, the line spacing (single, 1.15, 1.5, double,
at least, exactly or multiple) and the options keep with next, keep lines
together, widow and orphan control and page break before. The dialog is one
undo step and changes only what was edited; a field the selection disagrees on
stays empty and is left alone. An emptied indent or spacing goes back to what
the paragraph style says. Tab stops are not part of the dialog.

Lengths are shown in centimetres, or in inches when the interface is English and
the browser's language is US English; a comma or a point is accepted. Font
sizes and spacing stay in points. The ruler keeps its centimetre scale.

While a header or footer is open, its bar and **Insert > Fields** insert the
page number, "page X of Y", the number of pages and the pages of the section.
**Date** inserts today's date as text; it does not update itself.
```

- [ ] **Step 3: The roadmap**

In `docs/superpowers/specs/2026-11-12-writer-office-roadmap-design.md`, directly after the Wave 2 table (the line that starts with `| Server-side PDF`), add an empty line and:

```text
Wave 2 is planned in three parts, each shippable on its own (user, 2026-11-13):

- **2a** — page setup dialog, paragraph dialog, page fields and date, one length unit. Plan: `docs/superpowers/plans/2026-11-14-writer-wave2a-page-paragraph.md`. Tab stops move to the interactive ruler in Wave 3 (user, 2026-11-13). Header and footer distance is left out: the engine exposes it only while a header or footer is open. The ruler keeps centimetres until Wave 3.
- **2b** — PDF through the print path, templates as real `.docx` files, "save as template".
- **2c** — table border style, image size, crop, replace and position, styles gallery, repeat header row and table alignment.
```

- [ ] **Step 4: DOX pass and commit**

Re-read the changed paths against the `AGENTS.md` chain. `ui/AGENTS.md` and the root guide need no change; say so in the report. `docs/` is git-ignored, so the roadmap needs `git add -f`.

```
git add ui/js/desktop/apps/AGENTS.md documentation/autor.md
git add -f docs/superpowers/specs/2026-11-12-writer-office-roadmap-design.md
git diff --cached --check
git commit
```

Message:

```
docs(writer): page setup, paragraph dialog, page fields and units

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: Claude Fable 5.2 <noreply@anthropic.com>
```

---

## Final verification (controller)

- `npm run test:writer` and `npm run check:ui` pass.
- Browser: `TestDesktop(Writer|QR)` passes twice in a row (delete `reports\autor` before each run).
- `go test ./ui -count=1 -skip GoldfishAdventureHotspot` fails only in `TestDesktopCodeStudioRunUsesTerminalSession`.
- `git status --short` is empty; `disposable/_writer-wave2a/` does not exist.
- One branch review by the reader-agent (Critical and Important only), one fix round.

## Changes after the branch review (2026-11-14)

The code blocks above are the plan as it was executed. The review led to these changes on top:

- `pageCommand(values, setup, u)` and `paragraphCommand(values, formatting, u)` take the engine's original state instead of the field texts and compare values: a retyped equal value ("2.50" for "2,5") sends nothing, an untouched side of a non-catalogue page keeps its exact size, and turning such a page swaps the unrounded sizes.
- The header/footer bar starts with "back to the document", so it cannot scroll out of view in a narrow window.
- In the form dialog an input fills its column (`box-sizing: border-box; width: 100%`); half-width inputs without a unit overflowed.
- The right-click check in `ui/desktop_writer_context_test.go` expects the Paragraph item, and the layout check compares the unit element instead of searching the dialog's text.
- `module-loader.js` is not part of `main.bundle.js`; `npm run build:ui` changed nothing.
