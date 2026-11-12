# Writer Wave 0 — Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give Writer one command registry that feeds menus, shortcuts and a data-driven toolbar with an overflow popover, plus a reusable form dialog, so that the feature waves that follow cost one registry entry per command.

**Architecture:** A pure module `writer-commands.js` owns the command table, the one "blocked while suggesting changes" rule, the `can`/`run` gate, colour conversion and the shortcut table. `writer-toolbar.js` renders the toolbar from that table and owns overflow and arrow-key navigation. `shared/form-dialog.js` is a native `<dialog>` with typed fields. `writer.js` keeps lifecycle and file operations and shrinks.

**Tech Stack:** Vanilla JS classic scripts (no bundler for app files), `@docx-editor.dev/core` 2.29.0 as the vendored engine, node `vm` tests, Go + rod browser smoke tests.

**Spec:** `docs/superpowers/specs/2026-11-12-writer-office-roadmap-design.md` (Wave 0).

---

## Read this first

**What Wave 0 changes for the user.** Almost nothing on purpose: the toolbar no longer scrolls sideways (groups move into a "More tools" popover), Left/Right move between toolbar controls, tooltips and menus show shortcuts consistently, inserting a table asks for rows and columns in a real dialog, and typing AltGr letters (ś, ó, ń on a Polish layout) no longer triggers Save/Open/New.

**Deliberate behaviour changes** (everything else must behave as before):

1. Shortcuts match the exact modifier set. `Ctrl+Alt+<key>` (AltGr) and `Ctrl+Shift+O/N/F/H/P` no longer fire. `Ctrl+Shift+S` stays Save As.
2. The toolbar bar is `overflow:hidden`; groups leave in this order as the window narrows: lists, alignment, colours, insert, text marks, fonts. Undo/redo always stay.
3. The table prompt ("3 × 3") is replaced by a two-field dialog. The key `desktop.writer_table_dimensions` is removed.
4. A disabled toolbar button's tooltip reads `Label (Shortcut) · reason` (was `Label · reason: detail`).

**Repo rules that apply to every task**

- `CLAUDE.md`: run GitNexus `impact` (upstream) on a symbol before editing it and report the result. `render` in `writer.js` and `create` in `writer-panels.js` are closures the index may answer with `UNKNOWN`; that is not an all-clear, confirm callers with a text search (`WriterApp.render`, `WriterPanels.create`). Run `detect_changes --scope all` before every commit. Other sessions leave uncommitted files in this checkout; a `PARTIAL RESULT` that names only their files is expected. Stage only the paths listed in the task.
- Agent roster from the user's global `CLAUDE.md`: maker-agent writes code, test-agent runs tests. No model overrides.
- No external assets. All copy in all sixteen `ui/lang/desktop/*.json` files.
- Temporary scripts go into the designated disposable folder (memory note `disposable-folder`), never into the repo.
- Every file under `ui/js/desktop` stays below 1100 lines; file names never contain `-part-`.
- `desktop_office_app_markers_test.go` requires these strings to remain in `writer.js`: `window.WriterApp={render,dispose,instances}`, `queue?.dispose()`, `dispose(windowId);`, `editor?.destroy()`, `/api/desktop/office/document`.
- `writer-session.js` is shared with Notes, Sheets, Layerling and Synth Studio. Do not touch it.
- `[data-command]` is taken: `writer-panels.js` JSON-parses every element with that attribute. Toolbar controls use `data-tool`.
- End every commit message with the session's attribution trailer.

**Test commands**

| What | Command |
| --- | --- |
| Node tests | `npm run test:writer` |
| Static Go tests | `go test ./ui -run "(?i)(office|writer|virtualdesktop|lifecycle|i18n)" -count=1` |
| Bundle check | `npm run check:ui` |
| Browser smoke | `AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run "TestDesktopWriter(App|Engine)Browser" -count=1 -timeout 30m` |

On this Windows machine the browser smoke needs the setup in the memory notes `reference_chrome_launch_from_agent`, `reference_go_cache_disk_d` and `reference_windows_motion_browser_tests`. Read them before the first browser run.

## File structure

| File | Status | Responsibility |
| --- | --- | --- |
| `ui/js/desktop/apps/writer-commands.js` | new | Command table, tracking rule, `can`/`run`, colour conversion, shortcut table, menu items. No DOM. |
| `ui/js/desktop/apps/writer-toolbar.js` | new | Toolbar markup from the table, glyphs, state sync, overflow popover, roving tabindex. |
| `ui/js/desktop/shared/form-dialog.js` | new | `window.AuraFormDialog.open(...)`: modal form with typed fields. |
| `ui/css/desktop-form-dialog.css` | new | Styles for the form dialog. |
| `scripts/test-writer-commands.mjs` | new | Node tests for the registry and the language keys. |
| `scripts/test-writer-toolbar.mjs` | new | Node tests for toolbar groups and the overflow fit rule. |
| `ui/js/desktop/apps/writer.js` | modify | Lifecycle, file operations, wiring. Loses the toolbar, the icon maps, `can`/`run`, colour helpers. |
| `ui/js/desktop/apps/writer-panels.js` | modify | Uses `app.blocked` and `app.form`. |
| `ui/css/desktop-app-writer.css` | modify | Toolbar wrap, overflow popover, colour tokens. |
| `ui/js/desktop/core/module-loader.js` | modify | Writer script and style list. Rebuild `main.bundle.js` afterwards. |
| `ui/desktop_office_assets_test.go`, `ui/desktop_writer_browser_test.go`, `ui/desktop_writer_reload_browser_test.go`, `ui/desktop_writer_shell_test.go` | modify | Script lists and two new browser checks. |
| `ui/lang/desktop/*.json` (16) | modify | Three new keys, one removed. |
| `package.json` | modify | `test:writer` runs the new node tests. |
| `ui/js/desktop/apps/AGENTS.md`, `documentation/autor.md` | modify | Ownership and behaviour notes. |

Final script order for Writer: `purify`, `marked`, `writer-session.js`, `writer-commands.js`, `shared/form-dialog.js`, `writer-toolbar.js`, `writer-panels.js`, `writer.js`.

---

### Task 1: Command registry (pure module, test first)

**Files:**
- Create: `scripts/test-writer-commands.mjs`
- Create: `ui/js/desktop/apps/writer-commands.js`
- Modify: `package.json` (script `test:writer`)

- [ ] **Step 1: Write the failing test**

Create `scripts/test-writer-commands.mjs`. Objects that come out of the `vm` context belong to another realm, so the test compares scalars only (no `deepEqual` on returned objects).

```js
import assert from 'node:assert/strict';
import vm from 'node:vm';
import {readFileSync, readdirSync} from 'node:fs';

const context = {window: {}};
vm.runInNewContext(readFileSync('ui/js/desktop/apps/writer-commands.js', 'utf8'), context);
const WriterCommands = context.window.WriterCommands;

// The untrackable set is the rule writer.js used to spell as a regular expression.
const legacy = /^(insert(Table|Row|Column|Image|Note|Toc|Break|PageField|Hyperlink)|delete(Table|Row|Column|Note)|mergeCells|splitCell|set(PageSetup|TableBorders|CellFill|ImageProperties|ImageWrapType|NoteProperties)|convertNote)$/;
const types = ['insertTable', 'insertRow', 'insertColumn', 'insertImage', 'insertNote', 'insertToc', 'insertBreak', 'insertPageField', 'insertHyperlink',
    'deleteTable', 'deleteRow', 'deleteColumn', 'deleteNote', 'mergeCells', 'splitCell', 'setPageSetup', 'setTableBorders', 'setCellFill',
    'setImageProperties', 'setImageWrapType', 'setNoteProperties', 'convertNote',
    'undo', 'redo', 'paste', 'toggleMark', 'setAlignment', 'setIndent', 'setLineSpacing', 'setParagraphSpacing', 'setSelection', 'clearFormatting',
    'insertTextBox', 'setEditingMode', 'editHeaderFooter', 'exitHeaderFooter', 'replaceMatch'];
for (const type of types) assert.equal(WriterCommands.untrackable(type), legacy.test(type), type);

function harness(options = {}) {
    const log = {exec: [], table: [], notices: [], refreshed: 0, acted: []};
    const editor = {
        mode: options.mode || 'editing', refused: new Set(options.refused || []), active: new Set(options.active || []),
        snapshot() { return {editingMode: this.mode}; },
        can(command) { return this.refused.has(command.type) ? {ok: false, reason: 'engine'} : {ok: true}; },
        exec(command) { log.exec.push(command); return {ok: true}; },
        isActive(command) { return this.active.has(command.type); }
    };
    const slotTypes = {'history.undo': 'undo', 'history.redo': 'redo', 'text.bold': 'toggleBold', 'text.italic': 'toggleItalic'};
    const lib = {
        commandForSlot: slot => ({type: slotTypes[slot] || slot}),
        commandForSlotValue: (slot, value) => ({type: 'value', slot, value}),
        tableCommand(target, command, dry) { log.table.push(command.type + ':' + !!dry); return {ok: true}; }
    };
    const state = {editor: options.noEditor ? null : editor, loading: !!options.loading};
    const commands = WriterCommands.create({
        tr: key => key, t: key => key, readonly: !!options.readonly,
        editor: () => state.editor, lib: () => lib, loading: () => state.loading,
        act: id => { log.acted.push(id); return true; },
        notice: (message, error) => log.notices.push({message, error}),
        refresh: () => { log.refreshed++; }
    });
    return {commands, editor, log, state};
}

{ // Suggesting mode refuses structural commands before the engine is asked.
    const {commands, log} = harness({mode: 'suggesting'});
    const refused = commands.can({type: 'insertTable'});
    assert.equal(refused.ok, false);
    assert.equal(refused.reason, 'tracking_unsupported');
    assert.equal(commands.can({type: 'undo'}).ok, true);
    assert.equal(commands.blocked('setPageSetup'), 'tracking_unsupported');
    assert.equal(commands.blocked('paste'), '');
    assert.equal(commands.run({type: 'insertTable'}), false);
    assert.equal(log.exec.length, 0);
    assert.equal(log.notices[0].message, 'command_unavailable · tracking_unsupported');
    assert.equal(log.notices[0].error, true);
    assert.equal(log.refreshed, 0);
}
{ // Merge and split go through the table operations, dry for can and live for run.
    const {commands, log} = harness();
    assert.equal(commands.can({type: 'mergeCells'}).ok, true);
    assert.equal(commands.run({type: 'splitCell', rows: 1, cols: 2}), true);
    assert.equal(log.table.join(), 'mergeCells:true,splitCell:true,splitCell:false');
    assert.equal(log.exec.length, 0);
    assert.equal(log.refreshed, 1);
}
{ // Nothing runs while a document loads, and nothing is announced.
    const {commands, log} = harness({loading: true});
    assert.equal(commands.run({type: 'undo'}), false);
    assert.equal(log.exec.length, 0);
    assert.equal(log.notices.length, 0);
}
{ // Without a document only document-free actions are enabled.
    const {commands} = harness({noEditor: true});
    assert.equal(commands.can({type: 'undo'}).ok, false);
    assert.equal(commands.state('save').enabled, false);
    assert.equal(commands.state('bold').enabled, false);
    assert.equal(commands.state('open').enabled, true);
    assert.equal(commands.menuItem('undo').disabled, true);
    assert.equal(commands.menuItem('print').disabled, true);
    assert.equal(commands.menuItem('open').disabled, false);
}
{ // State comes from the engine; only toggles report a pressed state.
    const {commands, log} = harness({refused: ['undo'], active: ['toggleBold', 'redo']});
    assert.equal(commands.state('undo').enabled, false);
    assert.equal(commands.state('undo').reason, 'engine');
    assert.equal(commands.state('bold').active, true);
    assert.equal(commands.state('italic').active, false);
    assert.equal(commands.state('redo').active, false);
    assert.equal(commands.invoke('bold'), true);
    assert.equal(log.exec[0].type, 'toggleBold');
    assert.equal(commands.invoke('outline'), true);
    assert.equal(log.acted[0], 'outline');
}
{ // Read-only windows keep reading actions.
    const {commands} = harness({readonly: true});
    for (const id of ['new', 'save', 'saveAs', 'image', 'qr']) assert.equal(commands.state(id).enabled, false, id);
    for (const id of ['open', 'print', 'docx', 'search']) assert.equal(commands.state(id).enabled, true, id);
}
{ // An action can name the command type that tracking refuses.
    const {commands} = harness({mode: 'suggesting'});
    assert.equal(commands.state('qr').enabled, false);
    assert.equal(commands.state('qr').reason, 'tracking_unsupported');
    assert.equal(commands.state('image').enabled, true);
}
{ // Shortcuts match the exact modifier set.
    const {commands} = harness();
    const key = (name, mods = {}) => commands.actionForKey({key: name, ctrlKey: true, metaKey: false, shiftKey: false, altKey: false, ...mods});
    assert.equal(key('s'), 'save');
    assert.equal(key('S', {shiftKey: true}), 'saveAs');
    assert.equal(key('f'), 'search');
    assert.equal(key('h'), 'search');
    assert.equal(key('n'), 'new');
    assert.equal(key('o'), 'open');
    assert.equal(key('p'), 'print');
    assert.equal(key('s', {ctrlKey: false, metaKey: true}), 'save', 'Cmd counts as Ctrl');
    assert.equal(key('s', {altKey: true}), '', 'AltGr letters are text, not shortcuts');
    assert.equal(key('o', {shiftKey: true}), '');
    assert.equal(key('s', {ctrlKey: false}), '');
    assert.equal(key('z'), '', 'the engine owns undo');
}
{ // Labels, tooltips and menu items.
    const {commands, log} = harness();
    assert.equal(commands.label('saveAs'), 'save_as');
    assert.equal(commands.label('qr'), 'desktop.qr_insert');
    assert.equal(commands.tooltip('save'), 'save (Ctrl+S)');
    assert.equal(commands.tooltip('bold'), 'bold');
    const item = commands.menuItem('print');
    assert.equal(item.id, 'print');
    assert.equal(item.label, 'print');
    assert.equal(item.icon, 'printer');
    assert.equal(item.shortcut, 'Ctrl+P');
    assert.equal(item.disabled, false);
    item.action();
    assert.equal(log.acted[0], 'print');
    assert.equal(commands.menuItem('undo').disabled, false, 'menus are built once, so a slot item only needs a document');
    assert.equal(commands.menuItem('search').icon, 'search');
}
{ // The engine wants a bare hex, a palette name and half-points.
    const colors = WriterCommands.colors;
    assert.equal(colors.hex('#AABBCC'), '#aabbcc');
    assert.equal(colors.hex('aabbcc'), '#aabbcc');
    assert.equal(colors.hex({value: 'FF0000'}), '#ff0000');
    assert.equal(colors.hex({resolvedHex: '00ff00'}), '#00ff00');
    assert.equal(colors.hex('red'), '');
    assert.equal(colors.hex(null), '');
    assert.equal(colors.highlightName('#fffe00'), 'yellow');
    assert.equal(colors.highlightName('#000081'), 'darkBlue');
    assert.equal(colors.highlightName('nope'), '');
    assert.equal(colors.highlightHex('yellow'), '#ffff00');
    assert.equal(colors.highlightHex('#123456'), '#123456');
    const {commands} = harness();
    assert.equal(commands.valueCommand('text.color', '#AABBCC').value, 'aabbcc');
    assert.equal(commands.valueCommand('text.highlight', '#fffe00').value, 'yellow');
    assert.equal(commands.valueCommand('font.size', 11.5).value, 23);
    assert.equal(commands.valueCommand('font.family', 'Carlito').value, 'Carlito');
}

// The table itself.
const definitions = WriterCommands.definitions, ids = definitions.map(definition => definition.id);
assert.equal(new Set(ids).size, ids.length, 'ids are unique');
const bound = definitions.flatMap(definition => definition.keys || []);
assert.equal(new Set(bound).size, bound.length, 'no key is bound twice');
for (const definition of definitions) {
    assert.ok(definition.label || definition.fullLabel, definition.id + ' has a label');
    assert.ok(definition.slot || !definition.toggle, definition.id + ': only engine slots toggle');
    if (definition.keys) assert.ok(definition.shortcut, definition.id + ' shows the shortcut it binds');
}

// Every label of the table and every literal tr('key') of the Writer files exists in all sixteen languages.
const used = new Set(definitions.map(definition => definition.fullLabel || 'desktop.writer_' + definition.label));
for (const file of readdirSync('ui/js/desktop/apps').filter(name => /^writer(-[a-z]+)?\.js$/.test(name)))
    for (const match of readFileSync('ui/js/desktop/apps/' + file, 'utf8').matchAll(/\btr\('([a-z0-9_]+)'[,)]/g)) used.add('desktop.writer_' + match[1]);
const languages = readdirSync('ui/lang/desktop').filter(name => name.endsWith('.json'));
assert.equal(languages.length, 16);
for (const file of languages) {
    const labels = JSON.parse(readFileSync('ui/lang/desktop/' + file, 'utf8'));
    for (const key of used) assert.ok(labels[key], file + ' lacks ' + key);
}
console.log('Writer commands: tracking rule, gate, state, shortcuts, menu items, colours and ' + used.size + ' language keys pass');
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `node scripts/test-writer-commands.mjs`
Expected: FAIL with `ENOENT: no such file or directory, open 'ui/js/desktop/apps/writer-commands.js'`.

- [ ] **Step 3: Write the module**

Create `ui/js/desktop/apps/writer-commands.js`:

```js
(function () {
    'use strict';
    // Structural commands the engine cannot record as a tracked change.
    const UNTRACKABLE = new Set([
        'insertTable', 'insertRow', 'insertColumn', 'insertImage', 'insertNote', 'insertToc', 'insertBreak', 'insertPageField', 'insertHyperlink',
        'deleteTable', 'deleteRow', 'deleteColumn', 'deleteNote', 'mergeCells', 'splitCell',
        'setPageSetup', 'setTableBorders', 'setCellFill', 'setImageProperties', 'setImageWrapType', 'setNoteProperties', 'convertNote'
    ]);
    // The engine refuses these two; scripts/writer-table-ops.js edits the document tree instead.
    const TABLE_OPS = new Set(['mergeCells', 'splitCell']);
    // The OOXML highlight palette holds named colours only.
    const HIGHLIGHTS = {black: '000000', blue: '0000ff', cyan: '00ffff', darkBlue: '000080', darkCyan: '008080', darkGray: '808080', darkGreen: '008000',
        darkMagenta: '800080', darkRed: '800000', darkYellow: '808000', green: '00ff00', lightGray: 'c0c0c0', magenta: 'ff00ff', red: 'ff0000', white: 'ffffff', yellow: 'ffff00'};
    // '#rrggbb' for a hex string or an engine colour object, '' for anything else.
    function colorHex(value) {
        const raw = value && typeof value === 'object' ? (value.value ?? value.resolvedHex ?? '') : value;
        const body = String(raw ?? '').trim().replace(/^#/, '');
        return /^[0-9a-fA-F]{6}$/.test(body) ? '#' + body.toLowerCase() : '';
    }
    // The palette name closest to a picked colour.
    function highlightName(value) {
        const hex = colorHex(value);
        if (!hex) return '';
        const red = parseInt(hex.slice(1, 3), 16), green = parseInt(hex.slice(3, 5), 16), blue = parseInt(hex.slice(5, 7), 16);
        let nearest = '', distance = Infinity;
        for (const [name, candidate] of Object.entries(HIGHLIGHTS)) {
            const delta = (red - parseInt(candidate.slice(0, 2), 16)) ** 2 + (green - parseInt(candidate.slice(2, 4), 16)) ** 2 + (blue - parseInt(candidate.slice(4, 6), 16)) ** 2;
            if (delta < distance) { distance = delta; nearest = name; }
        }
        return nearest;
    }
    function highlightHex(value) {
        const name = String(value ?? '').trim();
        return HIGHLIGHTS[name] ? '#' + HIGHLIGHTS[name] : colorHex(value);
    }
    // One entry per command a surface can offer.
    //   slot       engine toolbar slot; without it the id is an app action handled by writer.js
    //   label      desktop.writer_* key; fullLabel names a key outside that prefix
    //   icon       shared icon key or a glyph drawn by writer-toolbar.js; fallback is the text stand-in
    //   shortcut   what menus and tooltips show; keys is what Writer itself binds (the engine binds the rest)
    //   write      refused in a read-only window; needsEditor: refused without a document
    //   untracked  command type whose tracking refusal disables the action
    const DEFINITIONS = [
        {id: 'undo', slot: 'history.undo', label: 'undo', icon: 'undo', fallback: '↶', shortcut: 'Ctrl+Z'},
        {id: 'redo', slot: 'history.redo', label: 'redo', icon: 'redo', fallback: '↷', shortcut: 'Ctrl+Y'},
        {id: 'style', slot: 'styles.style', label: 'style', control: 'select'},
        {id: 'fontFamily', slot: 'font.family', label: 'font', control: 'select'},
        {id: 'fontSize', slot: 'font.size', label: 'size', control: 'number'},
        {id: 'bold', slot: 'text.bold', label: 'bold', icon: 'format-bold', fallback: 'B', toggle: true},
        {id: 'italic', slot: 'text.italic', label: 'italic', icon: 'format-italic', fallback: 'I', toggle: true},
        {id: 'underline', slot: 'text.underline', label: 'underline', icon: 'format-underline', fallback: 'U', toggle: true},
        {id: 'strike', slot: 'text.strike', label: 'strike', icon: 'format-strike', fallback: 'S', toggle: true},
        {id: 'alignLeft', slot: 'alignment.left', label: 'align_left', icon: 'align-left', fallback: '≡', toggle: true},
        {id: 'alignCenter', slot: 'alignment.center', label: 'align_center', icon: 'align-center', fallback: '≡', toggle: true},
        {id: 'alignRight', slot: 'alignment.right', label: 'align_right', icon: 'align-right', fallback: '≡', toggle: true},
        {id: 'alignJustify', slot: 'alignment.justify', label: 'justify', icon: 'align-justify', fallback: '≡', toggle: true},
        {id: 'bullets', slot: 'list.bullet', label: 'bullets', icon: 'list', fallback: '•', toggle: true},
        {id: 'numbering', slot: 'list.numbered', label: 'numbering', icon: 'format-numbered', fallback: '1.', toggle: true},
        {id: 'outdent', slot: 'list.outdent', label: 'outdent', icon: 'outdent', fallback: '⇤'},
        {id: 'indent', slot: 'list.indent', label: 'indent', icon: 'indent', fallback: '⇥'},
        {id: 'textColor', slot: 'text.color', label: 'color', icon: 'text-color', control: 'color', value: '#172033'},
        {id: 'highlight', slot: 'text.highlight', label: 'highlight', icon: 'highlight', control: 'color', value: '#ffff00'},
        {id: 'new', label: 'new', icon: 'file-plus', shortcut: 'Ctrl+N', keys: ['n'], write: true},
        {id: 'open', label: 'open', icon: 'folder-open', shortcut: 'Ctrl+O', keys: ['o']},
        {id: 'save', label: 'save', icon: 'save', shortcut: 'Ctrl+S', keys: ['s'], write: true, needsEditor: true},
        {id: 'saveAs', label: 'save_as', icon: 'copy', shortcut: 'Ctrl+Shift+S', keys: ['shift+s'], write: true, needsEditor: true},
        {id: 'docx', label: 'download_docx', icon: 'file-text', needsEditor: true},
        {id: 'html', label: 'export_html', icon: 'file-code', needsEditor: true},
        {id: 'md', label: 'export_md', icon: 'notes', needsEditor: true},
        {id: 'txt', label: 'export_txt', icon: 'file-text', needsEditor: true},
        {id: 'print', label: 'print', icon: 'printer', shortcut: 'Ctrl+P', keys: ['p'], needsEditor: true},
        {id: 'search', label: 'find', icon: 'search', fallback: '⌕', shortcut: 'Ctrl+F', keys: ['f', 'h']},
        {id: 'outline', label: 'outline', icon: 'list', fallback: '☷'},
        {id: 'format', label: 'format', icon: 'sliders', fallback: 'A'},
        {id: 'review', label: 'review', icon: 'chat', fallback: '☷'},
        {id: 'assist', label: 'assist', icon: 'star', fallback: '✦'},
        {id: 'insert', label: 'insert', icon: 'plus', fallback: '＋'},
        {id: 'image', label: 'image', icon: 'image', write: true, needsEditor: true},
        {id: 'qr', fullLabel: 'desktop.qr_insert', icon: 'image', write: true, needsEditor: true, untracked: 'insertImage'},
        {id: 'focus', label: 'focus_mode', icon: 'maximize', fallback: '⛶'},
        {id: 'fit', label: 'fit', icon: 'monitor'}
    ];
    // host: {tr, t, readonly, editor(), lib(), loading(), act(id), notice(message, error), refresh()}
    function create(host) {
        const byId = new Map(DEFINITIONS.map(definition => [definition.id, definition]));
        const get = id => byId.get(id);
        const tracking = () => host.editor()?.snapshot().editingMode === 'suggesting';
        // Why a command type is refused before the engine is asked; '' when it is not.
        const blocked = type => tracking() && UNTRACKABLE.has(type) ? host.tr('tracking_unsupported') : '';
        function can(command) {
            const editor = host.editor();
            if (!editor || !command) return {ok: false};
            const reason = blocked(command.type);
            if (reason) return {ok: false, reason};
            return TABLE_OPS.has(command.type) ? host.lib().tableCommand(editor, command, true) : editor.can(command);
        }
        function run(command) {
            const editor = host.editor();
            if (!editor || !command || host.loading()) return false;
            const refuse = result => host.notice(host.tr('command_unavailable') + (result.reason ? ' · ' + result.reason : ''), true);
            const allowed = can(command);
            if (!allowed.ok) { refuse(allowed); return false; }
            const result = TABLE_OPS.has(command.type) ? host.lib().tableCommand(editor, command) : editor.exec(command);
            if (!result.ok) refuse(result);
            host.refresh();
            return result.ok;
        }
        const label = id => get(id).fullLabel ? host.t(get(id).fullLabel) : host.tr(get(id).label);
        const tooltip = id => label(id) + (get(id).shortcut ? ' (' + get(id).shortcut + ')' : '');
        // The engine command behind a slot entry; null for value slots and app actions.
        const command = id => get(id)?.slot && host.lib() ? host.lib().commandForSlot(get(id).slot) : null;
        function state(id) {
            const definition = get(id), editor = host.editor(), off = {enabled: false, active: false, reason: ''};
            if (!definition) return off;
            if (definition.slot) {
                const engineCommand = editor ? command(id) : null;
                if (!engineCommand) return off;
                const allowed = can(engineCommand);
                return {enabled: !!allowed.ok, active: !!definition.toggle && !!editor.isActive(engineCommand), reason: allowed.reason || ''};
            }
            if ((definition.write && host.readonly) || (definition.needsEditor && !editor)) return off;
            const reason = definition.untracked ? blocked(definition.untracked) : '';
            return {enabled: !reason, active: false, reason};
        }
        const invoke = id => get(id)?.slot ? run(command(id)) : host.act(id);
        // The engine wants a bare hex for colours, a palette name for highlights and half-points for sizes.
        function valueCommand(slot, value) {
            const lib = host.lib();
            if (slot === 'text.color') return lib.commandForSlotValue(slot, colorHex(value).replace('#', ''));
            if (slot === 'text.highlight') return lib.commandForSlotValue(slot, highlightName(value));
            if (slot === 'font.size') return lib.commandForSlotValue(slot, Number(value) * 2);
            return lib.commandForSlotValue(slot, value);
        }
        // The command a key press asks for, or '' when Writer does not bind it. Alt is excluded
        // because AltGr reports Ctrl+Alt, and those letters are text.
        function actionForKey(event) {
            if (!(event.ctrlKey || event.metaKey) || event.altKey) return '';
            const key = (event.shiftKey ? 'shift+' : '') + String(event.key).toLowerCase();
            return DEFINITIONS.find(definition => definition.keys?.includes(key))?.id || '';
        }
        // Window menus are built once per document, so a slot item is disabled only without a document.
        const menuItem = id => ({id, label: label(id), icon: get(id).icon || id, shortcut: get(id).shortcut,
            disabled: get(id).slot ? !host.editor() : !state(id).enabled, action: () => invoke(id)});
        return {get, can, run, blocked, label, tooltip, command, state, invoke, valueCommand, actionForKey, menuItem};
    }
    window.WriterCommands = {create, definitions: DEFINITIONS, untrackable: type => UNTRACKABLE.has(type), colors: {hex: colorHex, highlightName, highlightHex}};
})();
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `node scripts/test-writer-commands.mjs`
Expected: PASS, last line `Writer commands: tracking rule, gate, state, shortcuts, menu items, colours and ... language keys pass`.

- [ ] **Step 5: Add it to the npm script**

In `package.json` replace

```json
    "test:writer": "node scripts/test-writer-session.mjs",
```

with

```json
    "test:writer": "node scripts/test-writer-session.mjs && node scripts/test-writer-commands.mjs",
```

Run: `npm run test:writer`
Expected: both scripts print their pass line.

- [ ] **Step 6: Commit**

```bash
git add scripts/test-writer-commands.mjs ui/js/desktop/apps/writer-commands.js package.json
git commit -m "feat(writer): command registry with the tracking rule, gate and shortcut table"
```

---

### Task 2: Writer uses the registry for the gate, colours and the tracking rule

**Files:**
- Modify: `ui/js/desktop/apps/writer.js`
- Modify: `ui/js/desktop/apps/writer-panels.js:98-100,143-149`
- Modify: `ui/js/desktop/core/module-loader.js:125`, then rebuild `ui/js/desktop/bundles/main.bundle.js`
- Modify: `ui/desktop_office_assets_test.go:29`, `ui/desktop_writer_browser_test.go:75,591`, `ui/desktop_writer_reload_browser_test.go:70`

- [ ] **Step 1: Make the static test fail first**

In `ui/desktop_office_assets_test.go`, in `requiredLazyAssets`, add a line after `"/js/desktop/apps/writer-session.js",`:

```go
		"/js/desktop/apps/writer-commands.js",
```

Run: `go test ./ui -run "TestDesktopOfficeAssetsAreEmbeddedAndRouted" -count=1`
Expected: FAIL with `desktop lazy asset registry missing "/js/desktop/apps/writer-commands.js"`.

- [ ] **Step 2: Load the module**

In `ui/js/desktop/core/module-loader.js` (the `'writer'` entry) insert `'/js/desktop/apps/writer-commands.js', ` directly after `'/js/desktop/apps/writer-session.js', `. Only in the `'writer'` entry; the other four apps that list `writer-session.js` stay as they are.

In `ui/desktop_writer_browser_test.go` (two fixtures) and `ui/desktop_writer_reload_browser_test.go` (one fixture) replace each occurrence of

```html
<script src="/js/desktop/apps/writer-session.js"></script><script src="/js/desktop/apps/writer-panels.js"></script>
```

with

```html
<script src="/js/desktop/apps/writer-session.js"></script><script src="/js/desktop/apps/writer-commands.js"></script><script src="/js/desktop/apps/writer-panels.js"></script>
```

Run: `npm run build:ui`
Expected: `main.bundle.js` is rewritten (it embeds `module-loader.js`).

- [ ] **Step 3: Create the registry in `writer.js` and drop the local gate**

Directly after the line `const tr = (key, params) => ctx.t?.('desktop.writer_' + key, params) || key;` add:

```js
        const commands = window.WriterCommands.create({tr, t: (key, params) => ctx.t?.(key, params) || key, readonly: !!ctx.readonly,
            editor: () => editor, lib: () => lib, loading: () => loading, act: id => act(id), notice, refresh: () => refresh()});
        const {can, run} = commands, colors = window.WriterCommands.colors;
```

`notice`, `act` and `refresh` are function declarations further down in `render`, so they are hoisted; `editor`, `lib` and `loading` are read only when a command runs.

Delete the two function declarations `function can(command) {...}` and `function run(command) {...}` (the regex lives in the registry now). Leaving them would be a duplicate declaration and a syntax error.

In the `state` object replace

```js
            run,can,notice,prompt:(key,value)=>ctx.promptDialog(tr(key),value || ''),act,refresh:()=>refresh(),pinSelection,
            markCommand:(slot,value)=>slot==='text.color'?lib.commandForSlotValue(slot,colorValue(value).replace('#','')):
                slot==='text.highlight'?lib.commandForSlotValue(slot,highlightName(value)):lib.commandForSlotValue(slot,value),
            colorHex:value=>colorValue(value),highlightHex:value=>highlightHex(value),
```

with

```js
            commands,run,can,blocked:commands.blocked,notice,prompt:(key,value)=>ctx.promptDialog(tr(key),value || ''),act,refresh:()=>refresh(),pinSelection,
            markCommand:commands.valueCommand,colorHex:colors.hex,highlightHex:colors.highlightHex,
```

- [ ] **Step 4: Replace the colour helpers and `valueCommand` in `writer.js`**

Replace the whole `function valueCommand(control) {...}` with:

```js
        const valueCommand = control => commands.valueCommand(control.dataset.slot, control.dataset.slot === 'font.size' ? control.valueAsNumber : control.value);
```

Delete `const colorValue = ...`, the comment above `highlightColors`, `const highlightColors = ...`, `function highlightName(value) {...}` and `const highlightHex = ...`. Keep `countWords`.

In `refresh()` replace

```js
                    const hex=slot==='text.color'?colorValue(fmt.color):highlightHex(fmt.highlight);
```

with

```js
                    const hex=slot==='text.color'?colors.hex(fmt.color):colors.highlightHex(fmt.highlight);
```

- [ ] **Step 5: Use the tracking rule for images in `writer.js`**

In `insertImage` replace

```js
            if(editor.snapshot().editingMode==='suggesting')throw new Error(tr('tracking_unsupported'));
```

with

```js
            const refusal=commands.blocked('insertImage');if(refusal)throw new Error(refusal);
```

and replace

```js
                if(ctx.readonly || editor.snapshot().editingMode==='suggesting' || editor.getDocumentHandle().revision!==revision)throw new Error(ctx.t('desktop.qr_stale'));
```

with

```js
                if(ctx.readonly || commands.blocked('insertImage') || editor.getDocumentHandle().revision!==revision)throw new Error(ctx.t('desktop.qr_stale'));
```

In `insertQR` replace

```js
            if(editor.snapshot().editingMode==='suggesting')throw new Error(tr('tracking_unsupported'));
```

with

```js
            const refusal=commands.blocked('insertImage');if(refusal)throw new Error(refusal);
```

and in its `current` arrow replace `&& source.snapshot().editingMode!=='suggesting'` with `&& !commands.blocked('insertImage')` (the arrow has already checked `editor===source`).

Leave the paste handler's `editingMode==='suggesting'` test alone: it converts HTML to plain text, it does not gate a command.

- [ ] **Step 6: Use the tracking rule in `writer-panels.js`**

In `update()` replace

```js
            for(const el of root.querySelectorAll('[data-panel-command="table_insert"],[data-panel-command="hyperlink"],[data-action="image"]')){
                el.disabled=app.readonly || editor.snapshot().editingMode==='suggesting';el.title=el.disabled?tr('tracking_unsupported'):'';
            }
```

with

```js
            for(const [selector,type] of [['[data-panel-command="table_insert"]','insertTable'],['[data-panel-command="hyperlink"]','insertHyperlink'],['.writer-right [data-action="image"]','insertImage']]) {
                const reason=app.blocked(type);
                for(const el of root.querySelectorAll(selector)){el.disabled=app.readonly || !!reason;el.title=reason;}
            }
```

In the `if(right==='format')` block replace the three loops

```js
                for(const el of root.querySelectorAll('.writer-right input,.writer-right select')){
                    const key=el.dataset.field, tracking=snapshot.editingMode==='suggesting';
                    el.disabled=app.readonly || (tracking && ['paper','orientation','marginTop','marginBottom','marginLeft','marginRight','cellFill','imageWidth','imageAlt','imageWrap'].includes(key));
                    el.title=el.disabled?tr('command_unavailable'):'';
                }
                for(const key of ['cellFill']){const el=find('[data-field="'+key+'"]');if(el)el.disabled=app.readonly || snapshot.editingMode==='suggesting' || !editor.getSelectedTable();}
                for(const key of ['imageWidth','imageAlt','imageWrap']){const el=find('[data-field="'+key+'"]');if(el)el.disabled=app.readonly || snapshot.editingMode==='suggesting' || !image;}
```

with

```js
                // The engine command each field sends decides whether tracking refuses it.
                const fieldCommands={paper:'setPageSetup',orientation:'setPageSetup',marginTop:'setPageSetup',marginBottom:'setPageSetup',marginLeft:'setPageSetup',marginRight:'setPageSetup',
                    cellFill:'setCellFill',imageWidth:'setImageProperties',imageAlt:'setImageProperties',imageWrap:'setImageWrapType'};
                for(const el of root.querySelectorAll('.writer-right input,.writer-right select')){
                    const key=el.dataset.field,type=fieldCommands[key];
                    el.disabled=app.readonly || !!(type && app.blocked(type)) || (key==='cellFill' && !editor.getSelectedTable()) || (!!key?.startsWith('image') && !image);
                    el.title=el.disabled?tr('command_unavailable'):'';
                }
```

- [ ] **Step 7: Run the tests**

Run: `npm run test:writer`
Expected: PASS (the language-key scan now also reads the edited files).

Run: `go test ./ui -run "(?i)(office|writer|virtualdesktop|lifecycle|i18n)" -count=1`
Expected: `ok`.

Run: `npm run check:ui`
Expected: no bundle drift reported.

Run the browser smoke (see "Test commands").
Expected: PASS. It exercises formatting, tracking, tables, rename and conflicts, all of which go through the moved gate.

- [ ] **Step 8: Commit**

```bash
git add ui/js/desktop/apps/writer.js ui/js/desktop/apps/writer-panels.js ui/js/desktop/core/module-loader.js ui/js/desktop/bundles/main.bundle.js ui/desktop_office_assets_test.go ui/desktop_writer_browser_test.go ui/desktop_writer_reload_browser_test.go
git commit -m "refactor(writer): one tracking rule and command gate from the registry"
```

---

### Task 3: Menus, shortcuts and view buttons from the registry

**Files:**
- Modify: `ui/js/desktop/apps/writer.js`

- [ ] **Step 1: Build the document-bar buttons from the registry**

Delete the `actionIcons` constant and the `actionIcon` arrow. Replace the `button` arrow

```js
        const button = (action, label, symbol) => '<button type="button" data-action="' + action + '" title="' + esc(tr(label)) + '" aria-label="' + esc(tr(label)) + '">' + actionIcon(action, symbol) + '</button>';
```

with

```js
        const button = id => '<button type="button" data-action="' + id + '" title="' + esc(commands.tooltip(id)) + '" aria-label="' + esc(commands.label(id)) + '">' + icon(commands.get(id).icon, commands.get(id).fallback) + '</button>';
```

Update the seven call sites in the markup:

| Old | New |
| --- | --- |
| `button('outline','outline','☷')` | `button('outline')` |
| `button('search','find','⌕')` | `button('search')` |
| `button('format','format','A')` | `button('format')` |
| `button('review','review','☷')` | `button('review')` |
| `button('assist','assist','✦')` | `button('assist')` |
| `button('insert','insert','＋')` | `button('insert')` |
| `button('focus','focus_mode','⛶')` | `button('focus')` |

The Find button's tooltip now reads "Find (Ctrl+F)".

- [ ] **Step 2: Build the menus from the registry**

Replace the whole `function setMenus() {...}` with:

```js
        function setMenus() {
            const item=commands.menuItem;
            ctx.setWindowMenus?.(windowId,[
                {id:'file',labelKey:'desktop.menu_file',items:[
                    item('new'),item('open'),item('save'),item('saveAs'),
                    {type:'separator'},
                    {id:'gog-import',label:ctx.t('desktop.gog.import'),disabled:ctx.readonly,action:()=>gogOffice('import')},
                    {id:'gog-export',label:ctx.t('desktop.gog.export'),disabled:ctx.readonly||!editor,action:()=>gogOffice('export')},
                    {type:'separator'},item('docx'),item('html'),item('md'),item('txt'),
                    {type:'separator'},item('print')]},
                {id:'edit',labelKey:'desktop.menu_edit',items:[item('undo'),item('redo'),item('search'),item('review'),item('format'),item('insert')]},
                {id:'insert',label:tr('insert'),items:[item('image'),item('qr')]},
                {id:'agent',labelKey:'desktop.menu_agent',items:[item('assist')]},
                {id:'view',label:tr('view'),items:[item('outline'),item('focus'),item('fit')]}
            ]);
        }
```

Ids, labels, icons and disabled states are the same as before; Save As gains its shortcut label.

- [ ] **Step 3: Bind keys from the registry**

In the `root.addEventListener('keydown', ...)` handler replace

```js
            const command=event.ctrlKey || event.metaKey,action=command && {s:event.shiftKey?'saveAs':'save',o:'open',n:'new',f:'search',h:'search',p:'print'}[event.key.toLowerCase()];
            if(action){event.preventDefault();event.stopPropagation();act(action);}
```

with

```js
            const action=commands.actionForKey(event);
            if(action){event.preventDefault();event.stopPropagation();commands.invoke(action);}
```

- [ ] **Step 4: Run the tests**

Run: `npm run test:writer` and `go test ./ui -run "(?i)(office|writer|virtualdesktop|lifecycle|i18n)" -count=1`
Expected: PASS / `ok`.

Run the browser smoke. `verifyWriterShell` fails if a menu icon falls back to a text placeholder, so it proves the icon keys survived.
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add ui/js/desktop/apps/writer.js
git commit -m "refactor(writer): menus, shortcuts and view buttons read the command registry"
```

---

### Task 4: Data-driven toolbar module

The toolbar keeps its look and its `data-slot` attributes (browser tests select `[data-slot="font.size"]`). It moves into its own module and gets a wrapper element that Task 5 uses for the overflow popover.

**Files:**
- Create: `scripts/test-writer-toolbar.mjs`
- Create: `ui/js/desktop/apps/writer-toolbar.js`
- Modify: `ui/js/desktop/apps/writer.js`, `ui/css/desktop-app-writer.css`
- Modify: `ui/js/desktop/core/module-loader.js`, bundle, the three fixtures, `ui/desktop_office_assets_test.go`, `package.json`

- [ ] **Step 1: Write the failing test**

Create `scripts/test-writer-toolbar.mjs`:

```js
import assert from 'node:assert/strict';
import vm from 'node:vm';
import {readFileSync} from 'node:fs';

const context = {window: {}};
vm.runInNewContext(readFileSync('ui/js/desktop/apps/writer-commands.js', 'utf8'), context);
vm.runInNewContext(readFileSync('ui/js/desktop/apps/writer-toolbar.js', 'utf8'), context);
const {WriterCommands, WriterToolbar} = context.window;

// Every toolbar item is a registry command, placed once.
const known = new Map(WriterCommands.definitions.map(definition => [definition.id, definition]));
const placed = WriterToolbar.groups.flatMap(group => group.items);
assert.equal(new Set(placed).size, placed.length, 'no command sits in two groups');
for (const id of placed) assert.ok(known.has(id), 'toolbar item ' + id + ' is not a registry command');
for (const id of placed) assert.ok(known.get(id).control || known.get(id).icon, id + ' needs an icon');
const groupIds = WriterToolbar.groups.map(group => group.id);
assert.equal(new Set(groupIds).size, groupIds.length, 'group ids are unique');
assert.equal(new Set(WriterToolbar.groups.map(group => group.priority)).size, groupIds.length, 'priorities are distinct, so the overflow order is defined');

// Glyphs the shared atlases lack are drawn locally; everything else asks the shell.
const icon = WriterToolbar.icons({iconMarkup: (key, fallback) => '<i>' + key + '|' + fallback + '</i>'}, value => String(value));
assert.ok(icon('align-left', '≡').startsWith('<svg class="writer-glyph"'));
assert.equal(icon('undo', '↶'), '<i>undo|↶</i>');
assert.equal(WriterToolbar.icons({}, value => String(value))('undo', '↶'), '↶');
console.log('Writer toolbar: groups and glyphs pass');
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `node scripts/test-writer-toolbar.mjs`
Expected: FAIL with `ENOENT ... writer-toolbar.js`.

- [ ] **Step 3: Write the module**

Create `ui/js/desktop/apps/writer-toolbar.js`:

```js
(function () {
    'use strict';
    // The shared icon atlases carry no keys for text alignment, underline, indent or the
    // colour controls, so Autor draws those few glyphs itself, in the toolbar's line style.
    const GLYPHS = {
        'align-left': '<path d="M4 6h16M4 12h9M4 18h16"/>',
        'align-center': '<path d="M4 6h16M7.5 12h9M4 18h16"/>',
        'align-right': '<path d="M4 6h16M11 12h9M4 18h16"/>',
        'align-justify': '<path d="M4 6h16M4 12h16M4 18h16"/>',
        'format-underline': '<path d="M7 4v8a5 5 0 0 0 10 0V4"/><path d="M6 20h12"/>',
        'indent': '<path d="M4 5h16M4 12h8M4 19h16"/><path d="M12 8l4 4-4 4"/>',
        'outdent': '<path d="M4 5h16M4 12h8M4 19h16"/><path d="M16 8l-4 4 4 4"/>',
        'text-color': '<path d="M12 4L5.5 15M12 4l6.5 11"/><path d="M8.6 10.6h6.8"/><path d="M6 19.5h12" stroke-width="3"/>',
        'highlight': '<path d="M14 4l6 6-9 9H5v-6z"/><path d="M11 7l6 6"/><path d="M6 20.5h12" stroke-width="3"/>'
    };
    // icon(key, fallback): a local glyph, else the shell's icon, else the text stand-in.
    function icons(ctx, esc) {
        return (key, fallback) => {
            const shape = GLYPHS[key];
            if (shape) return '<svg class="writer-glyph" viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="1.9" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true" focusable="false">' + shape + '</svg>';
            return ctx.iconMarkup?.(key, fallback, 'writer-icon', 18, 'action') || esc(fallback ?? '');
        };
    }
    // Bar order. A group with a higher priority stays in the bar longer when the window narrows.
    const GROUPS = [
        {id: 'history', priority: 9, items: ['undo', 'redo']},
        {id: 'fonts', priority: 8, className: 'writer-fonts', items: ['style', 'fontFamily', 'fontSize']},
        {id: 'marks', priority: 7, items: ['bold', 'italic', 'underline', 'strike']},
        {id: 'alignment', priority: 4, items: ['alignLeft', 'alignCenter', 'alignRight', 'alignJustify']},
        {id: 'lists', priority: 3, items: ['bullets', 'numbering', 'outdent', 'indent']},
        {id: 'colors', priority: 5, className: 'writer-colors', items: ['textColor', 'highlight']},
        {id: 'insert', priority: 6, className: 'writer-tool-tail', items: ['insert']}
    ];
    const STYLE_LABELS = {normal: 'style_normal', title: 'style_title', subtitle: 'style_subtitle', quote: 'style_quote', caption: 'style_caption', nospacing: 'style_no_spacing', listparagraph: 'style_list_paragraph'};
    // app: the state facade of writer.js ({root, tr, esc, ctx, commands, editor, ...}).
    function create(app) {
        const {root, tr, esc, ctx, commands} = app, icon = icons(ctx, esc), colors = window.WriterCommands.colors, life = new AbortController();
        const wrap = root.querySelector('[data-toolbar]');
        // A slot control carries data-slot; an app action carries data-action, which writer.js handles.
        const attrs = id => 'data-tool="' + id + '" ' + (commands.get(id).slot ? 'data-slot="' + commands.get(id).slot + '"' : 'data-action="' + id + '"');
        function control(id) {
            const definition = commands.get(id), name = esc(commands.label(id));
            if (definition.control === 'select') return '<select ' + attrs(id) + ' aria-label="' + name + '"></select>';
            // The one number control is the font size, in points.
            if (definition.control === 'number') return '<input type="number" ' + attrs(id) + ' required min="1" max="200" step=".5" value="11" aria-label="' + name + '">';
            if (definition.control === 'color') return '<label class="writer-color" title="' + name + '">' + icon(definition.icon, '') + '<input type="color" ' + attrs(id) + ' value="' + definition.value + '" aria-label="' + name + '"></label>';
            return '<button type="button" ' + attrs(id) + ' title="' + esc(commands.tooltip(id)) + '" aria-label="' + name + '">' + icon(definition.icon, definition.fallback) + '</button>';
        }
        wrap.innerHTML = '<div class="writer-toolbar" role="toolbar" aria-label="' + esc(tr('format_bar')) + '">' +
            GROUPS.map(group => '<div class="writer-toolgroup' + (group.className ? ' ' + group.className : '') + '" data-group="' + group.id + '">' + group.items.map(control).join('') + '</div>').join('') + '</div>';
        const valueCommand = el => commands.valueCommand(el.dataset.slot, el.dataset.slot === 'font.size' ? el.valueAsNumber : el.value);
        const unavailable = reason => tr('command_unavailable') + (reason ? ' · ' + reason : '');
        // Tooltips change rarely; writing an unchanged one restarts the browser's tooltip timer.
        const setTitle = (el, title) => { if (el.title !== title) el.title = title; };
        function styleLabel(style) {
            const id = String(style.styleId || '').toLowerCase(), heading = id.match(/^heading([1-9])$/);
            if (heading) return tr('style_heading', {level: heading[1]});
            return STYLE_LABELS[id] ? tr(STYLE_LABELS[id]) : style.name;
        }
        // The style and font lists come from the loaded document.
        function fill() {
            const editor = app.editor, option = (value, text) => '<option value="' + esc(value) + '">' + esc(text) + '</option>';
            wrap.querySelector('[data-slot="styles.style"]').innerHTML = editor.getDocumentStyles().filter(style => style.type === 'paragraph').map(style => option(style.styleId, styleLabel(style))).join('');
            wrap.querySelector('[data-slot="font.family"]').innerHTML = editor.getAvailableFonts().map(font => option(font, font)).join('');
        }
        function refresh() {
            const editor = app.editor;
            if (!editor) return;
            const fmt = editor.snapshot().formatting || {};
            for (const el of wrap.querySelectorAll('[data-slot]')) {
                const id = el.dataset.tool, slot = el.dataset.slot;
                if (el.matches('button')) {
                    const state = commands.state(id);
                    el.disabled = !state.enabled;
                    setTitle(el, state.enabled ? commands.tooltip(id) : commands.tooltip(id) + ' · ' + unavailable(state.reason));
                    if (commands.get(id).toggle) el.setAttribute('aria-pressed', String(state.active));
                    continue;
                }
                if (el.type === 'color') {
                    const hex = slot === 'text.color' ? colors.hex(fmt.color) : colors.highlightHex(fmt.highlight);
                    if (hex && el !== document.activeElement) el.value = hex;
                } else if (el !== document.activeElement) {
                    el.value = slot === 'styles.style' ? fmt.styleId || 'Normal' : slot === 'font.family' ? fmt.fontFamily || 'Calibri' : fmt.fontSizePt || 11;
                } else continue;
                const allowed = commands.can(valueCommand(el));
                el.disabled = !allowed.ok;
                setTitle(el, allowed.ok ? '' : unavailable(allowed.reason));
            }
        }
        wrap.addEventListener('click', event => {
            const el = event.target.closest('button[data-slot]');
            if (el && !el.disabled) commands.invoke(el.dataset.tool);
        }, {signal: life.signal});
        wrap.addEventListener('change', event => {
            const el = event.target;
            if (!el.dataset.slot) return;
            if (!el.checkValidity()) { el.reportValidity(); return; }
            commands.run(valueCommand(el));
        }, {signal: life.signal});
        return {fill, refresh, dispose() { life.abort(); }};
    }
    window.WriterToolbar = {create, icons, groups: GROUPS};
})();
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `node scripts/test-writer-toolbar.mjs`
Expected: PASS, `Writer toolbar: groups and glyphs pass`.

In `package.json` extend `test:writer`:

```json
    "test:writer": "node scripts/test-writer-session.mjs && node scripts/test-writer-commands.mjs && node scripts/test-writer-toolbar.mjs",
```

- [ ] **Step 5: Load the module**

`ui/js/desktop/core/module-loader.js`, `'writer'` entry: insert `'/js/desktop/apps/writer-toolbar.js', ` directly before `'/js/desktop/apps/writer-panels.js'`.

In the three fixtures replace each occurrence of

```html
<script src="/js/desktop/apps/writer-commands.js"></script><script src="/js/desktop/apps/writer-panels.js"></script>
```

with

```html
<script src="/js/desktop/apps/writer-commands.js"></script><script src="/js/desktop/apps/writer-toolbar.js"></script><script src="/js/desktop/apps/writer-panels.js"></script>
```

In `ui/desktop_office_assets_test.go` add `"/js/desktop/apps/writer-toolbar.js",` after the `writer-commands.js` line.

Run: `npm run build:ui`

- [ ] **Step 6: Remove the toolbar from `writer.js`**

1. Delete the comment and constant `glyphs`, the `icon` arrow, the comment and constant `slotIcons`, `toggleSlots`, `slotButton` and `colorField`. In their place add one line (after the `commands` lines from Task 2):

```js
        const icon = window.WriterToolbar.icons(ctx, esc);
```

2. In the markup replace everything from `'<div class="writer-toolbar" role="toolbar" aria-label="' + esc(tr('format_bar')) + '">' +` up to and including `button('insert') + '</div>' +` (the toolbar and its seven groups) with:

```js
            '<div class="writer-toolbar-wrap" data-toolbar></div>' +
```

3. After `const panels = window.WriterPanels.create(state);` add:

```js
        const toolbar = window.WriterToolbar.create(state);
```

4. Delete `styleLabels`, `function styleLabel`, `function fillToolbar` and the `valueCommand` constant from Task 2.

5. In `load()` replace `fillToolbar();setMenus();refresh();` with `toolbar.fill();setMenus();refresh();`.

6. In `refresh()` replace the first line and the whole `for(const el of root.querySelectorAll('.writer-toolbar [data-slot]')) {...}` loop

```js
            const snapshot=editor.snapshot(),fmt=snapshot.formatting || {};
            for(const el of root.querySelectorAll('.writer-toolbar [data-slot]')) {
                ...
            }
```

with

```js
            const snapshot=editor.snapshot();
            toolbar.refresh();
```

7. Replace the pointerdown listener's selector `'.writer-toolbar,.writer-right,.writer-views'` with `'.writer-toolbar-wrap,.writer-right,.writer-views'`.

8. In the root `click` listener replace

```js
            const control=event.target.closest('[data-action],[data-slot]');if(!control || control.disabled)return;
            if(control.dataset.action)void act(control.dataset.action);else if(control.matches('button'))run(lib?.commandForSlot(control.dataset.slot));
```

with

```js
            const control=event.target.closest('[data-action]');if(!control || control.disabled)return;
            void act(control.dataset.action);
```

The toolbar module handles slot clicks now; leaving the old branch would toggle every mark twice.

9. Delete the whole root `change` listener (`root.addEventListener('change',event=>{const el=event.target;if(!el.dataset.slot)return; ... },{signal:lifecycle.signal});`). Panel fields use `data-field` and are handled in `writer-panels.js`.

10. In `cleanup()` add `toolbar.dispose();` directly before `panels.dispose();`.

- [ ] **Step 7: CSS for the wrapper**

In `ui/css/desktop-app-writer.css`:

Add before the `.writer-toolbar{` rule:

```css
.writer-toolbar-wrap{position:relative;flex-shrink:0}
```

Rescope the control rules from the bar to the wrapper, so they also style groups that sit in the overflow popover later. Replace the selectors:

| Old selector | New selector |
| --- | --- |
| `.writer-toolbar button{` | `.writer-toolbar-wrap button{` |
| `.writer-toolbar [data-slot="text.bold"]{` | `.writer-toolbar-wrap [data-slot="text.bold"]{` |
| `.writer-toolbar [data-slot="text.italic"]{` | `.writer-toolbar-wrap [data-slot="text.italic"]{` |
| `.writer-toolbar [data-slot="text.underline"]{` | `.writer-toolbar-wrap [data-slot="text.underline"]{` |
| `.writer-toolbar .writer-glyph{` | `.writer-toolbar-wrap .writer-glyph{` |
| `.writer-toolbar .writer-colors{` | `.writer-toolbar-wrap .writer-colors{` |
| `.writer-toolbar .writer-color{` | `.writer-toolbar-wrap .writer-color{` |
| `.writer-toolbar .writer-color:hover{` | `.writer-toolbar-wrap .writer-color:hover{` |
| `.writer-toolbar .writer-color input[type=color]{` (two places, one inside `@media(pointer:coarse)`) | `.writer-toolbar-wrap .writer-color input[type=color]{` |
| `.writer-toolbar button{width:44px;height:44px}` inside `@media(pointer:coarse)` | `.writer-toolbar-wrap button{width:44px;height:44px}` |

Add after the `.writer-toolbar-wrap .writer-colors{...}` rule:

```css
.writer-toolbar-wrap .writer-tool-tail{border-right:0;padding-right:0}
```

In the focus-mode rule replace `.writer-focus .writer-toolbar,` with `.writer-focus .writer-toolbar-wrap,`.

- [ ] **Step 8: Run the tests**

Run: `npm run test:writer`, `go test ./ui -run "(?i)(office|writer|virtualdesktop|lifecycle|i18n)" -count=1`, `npm run check:ui`
Expected: PASS / `ok` / no drift.

Run the browser smoke.
Expected: PASS. It sets the font size through `[data-slot="font.size"]`, clicks it with a real pointer and takes theme screenshots; compare `writer-standard-comfortable-1920x1080.png` with a run from before this task: the toolbar must look the same.

- [ ] **Step 9: Commit**

```bash
git add scripts/test-writer-toolbar.mjs ui/js/desktop/apps/writer-toolbar.js ui/js/desktop/apps/writer.js ui/css/desktop-app-writer.css ui/js/desktop/core/module-loader.js ui/js/desktop/bundles/main.bundle.js ui/desktop_office_assets_test.go ui/desktop_writer_browser_test.go ui/desktop_writer_reload_browser_test.go package.json
git commit -m "refactor(writer): toolbar rendered from the command registry in its own module"
```

---

### Task 5: Overflow popover and arrow-key navigation

**Files:**
- Modify: `scripts/test-writer-toolbar.mjs`, `ui/js/desktop/apps/writer-toolbar.js`, `ui/js/desktop/apps/writer.js`, `ui/css/desktop-app-writer.css`
- Modify: `ui/desktop_writer_shell_test.go`, `ui/desktop_writer_browser_test.go`
- Modify: `ui/lang/desktop/*.json` (key `desktop.writer_toolbar_more`)

- [ ] **Step 1: Write the failing test for the fit rule**

In `scripts/test-writer-toolbar.mjs` insert before the final `console.log`:

```js
// The overflow rule: the lowest priority leaves first, the bar always keeps one group,
// and the overflow button's width counts once something has left.
const bar = [{id: 'history', priority: 9, width: 70}, {id: 'fonts', priority: 8, width: 300}, {id: 'marks', priority: 7, width: 130},
    {id: 'alignment', priority: 4, width: 130}, {id: 'lists', priority: 3, width: 130}, {id: 'colors', priority: 5, width: 120}, {id: 'insert', priority: 6, width: 30}];
const fit = available => WriterToolbar.fit(bar, available, 30, 7).join();
assert.equal(fit(1000), '');
assert.equal(fit(952), '', 'an exact fit keeps everything');
assert.equal(fit(951), 'lists');
assert.equal(fit(700), 'lists,alignment,colors');
assert.equal(fit(420), 'lists,alignment,colors,insert,marks');
assert.equal(fit(100), 'lists,alignment,colors,insert,marks,fonts', 'one group always stays');
```

and change the final line to `console.log('Writer toolbar: groups, glyphs and overflow fit pass');`.

Run: `node scripts/test-writer-toolbar.mjs`
Expected: FAIL with `WriterToolbar.fit is not a function`.

- [ ] **Step 2: Add the language key**

Create this script in the disposable folder as `writer-wave0-i18n.mjs` (Task 6 runs it again with another argument):

```js
// Usage, from the repo root: node <path>/writer-wave0-i18n.mjs toolbar|table
import {readFileSync, writeFileSync} from 'node:fs';
const LANGS = ['cs', 'da', 'de', 'el', 'en', 'es', 'fr', 'hi', 'it', 'ja', 'nl', 'no', 'pl', 'pt', 'sv', 'zh'];
const text = {
    toolbar_more: ['Další nástroje', 'Flere værktøjer', 'Weitere Werkzeuge', 'Περισσότερα εργαλεία', 'More tools', 'Más herramientas', "Plus d'outils", 'और टूल', 'Altri strumenti', 'その他のツール', 'Meer hulpmiddelen', 'Flere verktøy', 'Więcej narzędzi', 'Mais ferramentas', 'Fler verktyg', '更多工具'],
    table_columns: ['Sloupce', 'Kolonner', 'Spalten', 'Στήλες', 'Columns', 'Columnas', 'Colonnes', 'कॉलम', 'Colonne', '列', 'Kolommen', 'Kolonner', 'Kolumny', 'Colunas', 'Kolumner', '列'],
    table_rows: ['Řádky', 'Rækker', 'Zeilen', 'Γραμμές', 'Rows', 'Filas', 'Lignes', 'पंक्तियाँ', 'Righe', '行', 'Rijen', 'Rader', 'Wiersze', 'Linhas', 'Rader', '行']
};
const prefix = key => '  "desktop.writer_' + key + '": ';
const line = (key, value) => prefix(key) + JSON.stringify(value) + ',';
function at(lines, key, file) {
    const index = lines.findIndex(candidate => candidate.startsWith(prefix(key)));
    if (index < 0 || !lines[index].endsWith(',')) throw new Error(file + ': anchor desktop.writer_' + key + ' is missing or is the last key');
    return index;
}
const steps = {
    toolbar(lines, i, file) { lines.splice(at(lines, 'toc', file) + 1, 0, line('toolbar_more', text.toolbar_more[i])); },
    table(lines, i, file) {
        lines[at(lines, 'table_dimensions', file)] = line('table_columns', text.table_columns[i]);
        lines.splice(at(lines, 'table_resize_hint', file) + 1, 0, line('table_rows', text.table_rows[i]));
    }
};
const step = steps[process.argv[2]];
if (!step) throw new Error('argument: toolbar or table');
LANGS.forEach((lang, i) => {
    const file = 'ui/lang/desktop/' + lang + '.json', lines = readFileSync(file, 'utf8').split('\n');
    step(lines, i, file);
    const result = lines.join('\n');
    JSON.parse(result);
    writeFileSync(file, result);
});
console.log('updated ' + LANGS.length + ' language files (' + process.argv[2] + ')');
```

Run from the repo root: `node <disposable folder>/writer-wave0-i18n.mjs toolbar`
Expected: `updated 16 language files (toolbar)`. `git diff --stat ui/lang/desktop` shows 16 files with one inserted line each.

- [ ] **Step 3: Implement fit, the popover and roving tabindex**

In `ui/js/desktop/apps/writer-toolbar.js`:

1. Add to `GLYPHS` (after `'highlight'`, with a comma on the line before):

```js
        'more': '<circle cx="5" cy="12" r="1.3"/><circle cx="12" cy="12" r="1.3"/><circle cx="19" cy="12" r="1.3"/>'
```

and extend the comment above `GLYPHS` to `... indent, the colour controls or the overflow button, so Autor ...`.

2. Add after the `GROUPS` constant:

```js
    // The ids of the groups that must leave a bar of `available` pixels. groups: [{id, priority, width}]
    // in bar order. The lowest priority leaves first; the bar always keeps one group.
    function fit(groups, available, moreWidth, gap) {
        const width = list => list.reduce((sum, group) => sum + group.width, 0) + Math.max(0, list.length - 1) * gap;
        if (width(groups) <= available) return [];
        const hidden = [];
        let visible = groups.slice();
        for (const group of groups.slice().sort((a, b) => a.priority - b.priority)) {
            if (visible.length <= 1 || width(visible) + gap + moreWidth <= available) break;
            visible = visible.filter(candidate => candidate !== group);
            hidden.push(group.id);
        }
        return hidden;
    }
```

3. In `create`, replace the `wrap.innerHTML = ...` statement with:

```js
        const moreLabel = esc(tr('toolbar_more'));
        wrap.innerHTML = '<div class="writer-toolbar" role="toolbar" aria-label="' + esc(tr('format_bar')) + '">' +
            GROUPS.map(group => '<div class="writer-toolgroup' + (group.className ? ' ' + group.className : '') + '" data-group="' + group.id + '">' + group.items.map(control).join('') + '</div>').join('') +
            '<button type="button" class="writer-toolbar-more" data-toolbar-more aria-haspopup="true" aria-expanded="false" title="' + moreLabel + '" aria-label="' + moreLabel + '" hidden>' + icon('more', '⋯') + '</button></div>' +
            '<div class="writer-toolbar-overflow" data-toolbar-overflow role="group" aria-label="' + moreLabel + '" hidden></div>';
        const bar = wrap.querySelector('.writer-toolbar'), more = wrap.querySelector('[data-toolbar-more]'), overflow = wrap.querySelector('[data-toolbar-overflow]');
        const nodes = GROUPS.map(group => ({id: group.id, priority: group.priority, node: wrap.querySelector('[data-group="' + group.id + '"]')}));
        const FIELDS = 'button,select,input';
        function closeOverflow() {
            if (overflow.hidden) return false;
            overflow.hidden = true;
            more.setAttribute('aria-expanded', 'false');
            return true;
        }
        // The bar is one tab stop: the control that last had focus, else the first enabled one.
        function rove(target) {
            const list = [...bar.querySelectorAll(FIELDS)].filter(el => !el.disabled && !el.hidden);
            const current = list.includes(target) ? target : list.find(el => el.tabIndex === 0) || list[0];
            for (const el of bar.querySelectorAll(FIELDS)) el.tabIndex = el === current ? 0 : -1;
        }
        // Put every group back, measure, then move what does not fit into the popover.
        function layout() {
            if (!bar.clientWidth) return;   // hidden in focus mode: keep the last arrangement
            const focused = wrap.contains(document.activeElement) ? document.activeElement : null;
            more.hidden = false;
            for (const group of nodes) bar.insertBefore(group.node, more);
            const style = getComputedStyle(bar), gap = parseFloat(style.columnGap) || 0;
            const available = bar.clientWidth - parseFloat(style.paddingLeft) - parseFloat(style.paddingRight);
            const hidden = new Set(fit(nodes.map(group => ({id: group.id, priority: group.priority, width: group.node.offsetWidth})), available, more.offsetWidth, gap));
            for (const group of nodes) if (hidden.has(group.id)) overflow.appendChild(group.node);
            for (const el of overflow.querySelectorAll(FIELDS)) el.tabIndex = 0;
            more.hidden = !hidden.size;
            if (!hidden.size) closeOverflow();
            rove();
            // Moving a node blurs it; give the focus back where it was.
            if (focused && focused.isConnected && focused !== document.activeElement) focused.focus({preventScroll: true});
        }
        let frame = 0;
        const observer = new ResizeObserver(() => { cancelAnimationFrame(frame); frame = requestAnimationFrame(layout); });
        observer.observe(wrap);
```

4. At the end of `refresh()` (after the `for` loop) add:

```js
            rove();
```

5. After the existing `change` listener add:

```js
        more.addEventListener('click', () => {
            overflow.hidden = !overflow.hidden;
            more.setAttribute('aria-expanded', String(!overflow.hidden));
        }, {signal: life.signal});
        document.addEventListener('pointerdown', event => { if (!wrap.contains(event.target)) closeOverflow(); }, {signal: life.signal});
        bar.addEventListener('focusin', event => { if (event.target.matches(FIELDS)) rove(event.target); }, {signal: life.signal});
        // Left and Right move through the bar. Up and Down stay with the select and the size field.
        bar.addEventListener('keydown', event => {
            if (!['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(event.key) || event.altKey || event.ctrlKey || event.metaKey) return;
            const list = [...bar.querySelectorAll(FIELDS)].filter(el => !el.disabled && !el.hidden), index = list.indexOf(event.target);
            const edge = event.key === 'Home' || event.key === 'End';
            if (index < 0 || (edge && !event.target.matches('button'))) return;
            event.preventDefault();
            const next = event.key === 'Home' ? list[0] : event.key === 'End' ? list.at(-1) : list[(index + (event.key === 'ArrowRight' ? 1 : -1) + list.length) % list.length];
            rove(next);
            next.focus();
        }, {signal: life.signal});
```

Home and End act only on buttons, because inside the select and the size field they belong to the field.

6. Replace the `return {fill, refresh, dispose() { life.abort(); }};` line with:

```js
        // Escape closes the popover and returns to its button; false when it was not open.
        function escape() {
            if (!closeOverflow()) return false;
            more.focus();
            return true;
        }
        return {fill, refresh, escape, dispose() { life.abort(); observer.disconnect(); cancelAnimationFrame(frame); }};
```

7. Replace the export line with `window.WriterToolbar = {create, icons, groups: GROUPS, fit};`.

- [ ] **Step 4: Escape order in `writer.js`**

In the keydown handler's Escape block add as the first check (before the document-menu check):

```js
                if(toolbar.escape())return;
```

and extend the comment above it: `// The overflow popover and the menu are the topmost layers; ...`.

- [ ] **Step 5: CSS**

In `ui/css/desktop-app-writer.css`, in the `.writer-toolbar{` rule replace `overflow-x:auto;overflow-y:hidden;` with `overflow:hidden;` and remove `;scrollbar-width:thin` from the end of that rule.

Add after the `.writer-toolbar-wrap .writer-tool-tail{...}` rule:

```css
.writer-toolbar-more{margin-left:auto;flex-shrink:0}
.writer-toolbar-overflow{position:absolute;z-index:11;top:calc(100% - 3px);right:10px;display:flex;flex-wrap:wrap;align-items:center;gap:7px;max-width:calc(100% - 20px);padding:8px;background:var(--writer-card);border:1px solid var(--vd-mat-card-border,var(--writer-border));border-radius:var(--vd-mat-card-radius,10px);box-shadow:var(--vd-mat-card-shadow,0 0 #0000),0 18px 40px #0006}
.writer-toolbar-overflow .writer-toolgroup{border-right:0;padding-right:0}
```

- [ ] **Step 6: Run the node tests**

Run: `npm run test:writer`
Expected: PASS, including `Writer toolbar: groups, glyphs and overflow fit pass` and the language scan with the new `toolbar_more` key.

- [ ] **Step 7: Browser check**

Add to `ui/desktop_writer_shell_test.go` (after `verifyWriterShell`):

```go
// verifyWriterToolbarOverflow narrows the standalone Writer fixture and checks that
// low-priority groups move into the overflow popover instead of scrolling the bar.
func verifyWriterToolbarOverflow(t *testing.T, page *rod.Page) {
	t.Helper()
	page.MustEval(`async()=>{
        const host=document.getElementById('host'),wait=()=>new Promise(r=>setTimeout(r,300));
        const bar=document.querySelector('.writer-toolbar'),more=document.querySelector('[data-toolbar-more]'),overflow=document.querySelector('[data-toolbar-overflow]');
        await wait();
        if(!more.hidden||overflow.children.length)throw Error('overflow in use although the bar fits');
        if(bar.querySelectorAll('[tabindex="0"]').length!==1)throw Error('the bar needs exactly one tab stop');
        host.style.width='520px';await wait();
        if(bar.scrollWidth>bar.clientWidth+1)throw Error('narrow toolbar still overflows: '+bar.scrollWidth+'>'+bar.clientWidth);
        if(more.hidden)throw Error('narrow toolbar has no overflow button');
        if(!bar.querySelector('[data-slot="font.size"]'))throw Error('the font group left the bar');
        if(!overflow.querySelector('[data-slot="text.color"]'))throw Error('the colour group did not move to the overflow');
        more.click();
        if(overflow.hidden||more.getAttribute('aria-expanded')!=='true')throw Error('overflow popover did not open');
        bar.dispatchEvent(new KeyboardEvent('keydown',{key:'Escape',bubbles:true}));
        if(!overflow.hidden||document.activeElement!==more)throw Error('Escape did not close the overflow popover');
        const first=bar.querySelector('[data-tool="undo"]'),second=bar.querySelector('[data-tool="redo"]');
        for(const el of [first,second])el.disabled=false;
        first.focus();first.dispatchEvent(new KeyboardEvent('keydown',{key:'ArrowRight',bubbles:true}));
        if(document.activeElement!==second||second.tabIndex!==0||first.tabIndex!==-1)throw Error('ArrowRight did not move the tab stop');
        host.style.width='';await wait();
        if(!more.hidden||overflow.children.length)throw Error('overflow not restored at full width');
    }`)
}
```

In `ui/desktop_writer_browser_test.go`, in `TestDesktopWriterAppBrowser`, add directly after the line `verifyWriterPointerFocus(t, page, "test", "")`:

```go
	verifyWriterToolbarOverflow(t, page)
```

Run the browser smoke.
Expected: PASS. Then look at `writer-standard-comfortable-390x844.png` and `writer-standard-comfortable-1366x768.png` from the run: the bar does not scroll, the More button sits at the right edge when groups left.

- [ ] **Step 8: Commit**

```bash
git add scripts/test-writer-toolbar.mjs ui/js/desktop/apps/writer-toolbar.js ui/js/desktop/apps/writer.js ui/css/desktop-app-writer.css ui/desktop_writer_shell_test.go ui/desktop_writer_browser_test.go ui/lang/desktop
git commit -m "feat(writer): toolbar overflow popover and arrow-key navigation"
```

---

### Task 6: Shared form dialog, first used for Insert table

**Files:**
- Create: `ui/js/desktop/shared/form-dialog.js`, `ui/css/desktop-form-dialog.css`
- Modify: `ui/js/desktop/apps/writer.js` (state facade), `ui/js/desktop/apps/writer-panels.js:230`
- Modify: `ui/js/desktop/core/module-loader.js`, bundle, the three fixtures, `ui/desktop_office_assets_test.go`
- Modify: `ui/desktop_writer_shell_test.go`, `ui/desktop_writer_browser_test.go`
- Modify: `ui/lang/desktop/*.json` (add `table_columns`, `table_rows`; remove `table_dimensions`)

- [ ] **Step 1: Write the failing browser check**

Add to `ui/desktop_writer_shell_test.go`:

```go
// verifyWriterTableDialog inserts a table through the form dialog: the form refuses an
// invalid size, a valid one is answered as numbers and changes the document, and cancel
// answers null. The engine object may be frozen, so the check listens at the dialog.
func verifyWriterTableDialog(t *testing.T, page *rod.Page) {
	t.Helper()
	page.MustEval(`async()=>{
        const app=WriterApp.instances.get('test'),wait=ms=>new Promise(r=>setTimeout(r,ms));
        if(!document.querySelector('[data-panel-command="table_insert"]'))await app.act('insert');
        const open=AuraFormDialog.open;let answer;
        AuraFormDialog.open=options=>open(options).then(values=>(answer=values));
        try {
            const before=app.session.revision;
            document.querySelector('[data-panel-command="table_insert"]').click();await wait(150);
            const dialog=document.querySelector('dialog.vd-form-dialog');
            if(!dialog?.open)throw Error('table dialog did not open');
            const set=(id,value)=>{dialog.querySelector('[data-form-field="'+id+'"]').value=value;};
            set('rows','0');dialog.querySelector('[type=submit]').click();await wait(50);
            if(!dialog.open||answer!==undefined)throw Error('an invalid row count was accepted');
            set('rows','2');set('cols','4');dialog.querySelector('[type=submit]').click();await wait(300);
            if(document.querySelector('dialog.vd-form-dialog'))throw Error('table dialog stayed open');
            if(answer?.rows!==2||answer?.cols!==4)throw Error('table dialog answered '+JSON.stringify(answer));
            const notice=document.querySelector('[data-notice]');
            if(!notice.hidden&&notice.dataset.error==='true')throw Error(document.querySelector('[data-notice-text]').textContent);
            if(app.session.revision===before)throw Error('no table was inserted');
            document.querySelector('[data-panel-command="table_insert"]').click();await wait(150);
            document.querySelector('dialog.vd-form-dialog [data-form-cancel]').click();await wait(100);
            if(document.querySelector('dialog.vd-form-dialog')||answer!==null)throw Error('cancel did not close the dialog with null');
        } finally { AuraFormDialog.open=open; }
    }`)
}
```

In `TestDesktopWriterAppBrowser` add after `verifyWriterToolbarOverflow(t, page)`:

```go
	verifyWriterTableDialog(t, page)
```

Run the browser smoke for `TestDesktopWriterAppBrowser`.
Expected: FAIL with `table dialog did not open` (the click still opens the prompt stub).

- [ ] **Step 2: Write the dialog**

Create `ui/js/desktop/shared/form-dialog.js`:

```js
(function () {
    'use strict';
    const escapeHTML = value => String(value ?? '').replace(/[&<>"']/g, c => ({'&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;'}[c]));
    let serial = 0;
    // Field: {id, label, type: 'text'|'url'|'number'|'select'|'checkbox', value, required,
    // min, max, step, maxlength, placeholder, options: [{value, label}]}.
    function fieldMarkup(field) {
        const name = 'data-form-field="' + escapeHTML(field.id) + '"', label = '<span>' + escapeHTML(field.label) + '</span>';
        if (field.type === 'checkbox') return '<label class="vd-form-check"><input type="checkbox" ' + name + (field.value ? ' checked' : '') + '>' + label + '</label>';
        if (field.type === 'select') return '<label>' + label + '<select class="vd-modal-input" ' + name + '>' + (field.options || []).map(option =>
            '<option value="' + escapeHTML(option.value) + '"' + (String(option.value) === String(field.value ?? '') ? ' selected' : '') + '>' + escapeHTML(option.label) + '</option>').join('') + '</select></label>';
        const limits = ['min', 'max', 'step', 'maxlength', 'placeholder'].filter(key => field[key] !== undefined && field[key] !== null)
            .map(key => ' ' + key + '="' + escapeHTML(field[key]) + '"').join('');
        return '<label>' + label + '<input class="vd-modal-input" type="' + (['number', 'url'].includes(field.type) ? field.type : 'text') + '" ' + name +
            ' value="' + escapeHTML(field.value ?? '') + '"' + limits + (field.required ? ' required' : '') + ' autocomplete="off"></label>';
    }
    // A number answers as a Number, a checkbox as a boolean, everything else as a trimmed string.
    function read(field, el) {
        if (field.type === 'checkbox') return el.checked;
        if (field.type === 'number') return el.valueAsNumber;
        return el.value.trim();
    }
    // open({title, fields, submitLabel, cancelLabel, validate, signal}) resolves to {id: value}
    // or to null when the dialog is cancelled. validate(values) returns a message to refuse them.
    function open(options) {
        if (options.signal?.aborted) return Promise.resolve(null);
        const fields = options.fields || [], id = 'vd-form-dialog-' + (++serial);
        const node = document.createElement('dialog');
        node.className = 'vd-form-dialog';
        node.setAttribute('aria-labelledby', id);
        node.innerHTML = '<form method="dialog" novalidate><h2 id="' + id + '">' + escapeHTML(options.title) + '</h2>' + fields.map(fieldMarkup).join('') +
            '<p role="alert" data-form-error hidden></p><div class="vd-modal-actions">' +
            '<button type="button" class="vd-button" data-form-cancel>' + escapeHTML(options.cancelLabel) + '</button>' +
            '<button type="submit" class="vd-button vd-button-primary">' + escapeHTML(options.submitLabel) + '</button></div></form>';
        // The dialog lives on the body, so an app's own key handlers never see its keys.
        document.body.appendChild(node);
        return new Promise(resolve => {
            const form = node.querySelector('form'), error = node.querySelector('[data-form-error]');
            let answer = null;
            const finish = value => { answer = value; node.close(); };
            const onAbort = () => finish(null);
            node.addEventListener('close', () => { options.signal?.removeEventListener('abort', onAbort); node.remove(); resolve(answer); }, {once: true});
            node.querySelector('[data-form-cancel]').addEventListener('click', () => finish(null));
            form.addEventListener('submit', event => {
                event.preventDefault();
                if (!form.reportValidity()) return;
                const values = {};
                for (const field of fields) values[field.id] = read(field, form.querySelector('[data-form-field="' + CSS.escape(field.id) + '"]'));
                const message = options.validate?.(values) || '';
                error.textContent = message;
                error.hidden = !message;
                if (!message) finish(values);
            });
            options.signal?.addEventListener('abort', onAbort, {once: true});
            // showModal traps the focus, closes on Escape and returns the focus on close.
            node.showModal();
            const first = node.querySelector('[data-form-field]');
            first?.focus();
            first?.select?.();
        });
    }
    window.AuraFormDialog = {open};
})();
```

Create `ui/css/desktop-form-dialog.css`:

```css
/* Shared form dialog (window.AuraFormDialog). Reuses the shell's modal input and button classes. */
.vd-form-dialog{width:min(420px,calc(100vw - 32px));padding:var(--ds-space-5,20px);border:1px solid var(--ds-color-border-default,var(--vd-border,#ffffff26));border-radius:var(--ds-radius-xl,14px);background:var(--ds-color-surface-3,var(--vd-theme-panel-bg,#1b202a));color:var(--vd-text,#e4e8f0);box-shadow:var(--ds-elevation-5,0 18px 40px #0006);font:inherit}
.vd-form-dialog::backdrop{background:rgba(8,12,22,.6)}
.vd-form-dialog form{display:grid;gap:12px;margin:0}
.vd-form-dialog h2{margin:0 0 2px;font-size:inherit;font-weight:900}
.vd-form-dialog label{display:grid;gap:6px;color:var(--vd-muted,var(--vd-theme-muted,#a7b1c3))}
.vd-form-dialog label.vd-form-check{display:flex;align-items:center;gap:8px}
.vd-form-dialog .vd-modal-input{margin:0}
.vd-form-dialog [role=alert]{margin:0;color:var(--ds-color-danger-fg,#f29a91)}
.vd-form-dialog .vd-modal-actions{display:flex;justify-content:flex-end;gap:8px;margin-top:4px}
@media(prefers-reduced-motion:reduce){.vd-form-dialog{transition:none!important;animation:none!important}}
```

- [ ] **Step 3: Load both files**

`ui/js/desktop/core/module-loader.js`, `'writer'` entry:

- styles: `appStyles('/js/vendor/writer/engine.css', '/css/desktop-form-dialog.css', '/css/desktop-app-writer.css')`
- scripts: insert `'/js/desktop/shared/form-dialog.js', ` directly before `'/js/desktop/apps/writer-toolbar.js'`.

In the three fixtures replace each occurrence of

```html
<script src="/js/desktop/apps/writer-commands.js"></script><script src="/js/desktop/apps/writer-toolbar.js"></script>
```

with

```html
<script src="/js/desktop/apps/writer-commands.js"></script><script src="/js/desktop/shared/form-dialog.js"></script><script src="/js/desktop/apps/writer-toolbar.js"></script>
```

In `ui/desktop_office_assets_test.go` add `"/js/desktop/shared/form-dialog.js",` and `"/css/desktop-form-dialog.css",` to `requiredLazyAssets`.

Run: `npm run build:ui`

- [ ] **Step 4: Language keys**

Confirm nothing else uses the old key:

Run: `git grep -n "table_dimensions" -- ui scripts internal`
Expected: only `ui/js/desktop/apps/writer-panels.js` and the sixteen `ui/lang/desktop/*.json` files.

Run from the repo root: `node <disposable folder>/writer-wave0-i18n.mjs table`
Expected: `updated 16 language files (table)`; each file loses `desktop.writer_table_dimensions` and gains `desktop.writer_table_columns` and `desktop.writer_table_rows`.

- [ ] **Step 5: Offer the dialog to the panels and use it**

In `ui/js/desktop/apps/writer.js`, in the `state` object, add after `prompt:(key,value)=>ctx.promptDialog(tr(key),value || ''),`:

```js
            form:(title,fields)=>window.AuraFormDialog.open({title:tr(title),fields,submitLabel:ctx.t('desktop.ok'),cancelLabel:ctx.t('desktop.cancel'),signal:documentLife.signal}),
```

`documentLife` is replaced on every load; the arrow reads the current one when it is called, so a dialog closes when its document goes away.

In `ui/js/desktop/apps/writer-panels.js` replace the `case 'table_insert'` line

```js
                case 'table_insert':{const answer=await app.prompt('table_dimensions','3 × 3');if(!answer)return;const [rows,cols]=answer.split(/[x×,;\s]+/).map(Number);if(rows>=1 && rows<=100 && cols>=1 && cols<=30)app.run({type:'insertTable',rows,cols});else app.notice(tr('table_dimensions'),true);break;}
```

with

```js
                case 'table_insert':{
                    const size=await app.form('table',[
                        {id:'rows',label:tr('table_rows'),type:'number',value:3,min:1,max:100,step:1,required:true},
                        {id:'cols',label:tr('table_columns'),type:'number',value:3,min:1,max:30,step:1,required:true}]);
                    if(size)app.run({type:'insertTable',rows:size.rows,cols:size.cols});
                    break;
                }
```

The form's own limits replace the hand-written range check and the error notice that showed the prompt label.

- [ ] **Step 6: Run the tests**

Run: `npm run test:writer`
Expected: PASS; the language scan finds `table_rows` and `table_columns` in all sixteen files.

Run: `go test ./ui -run "(?i)(office|writer|virtualdesktop|lifecycle|i18n)" -count=1` and `npm run check:ui`
Expected: `ok` / no drift.

Run the browser smoke.
Expected: PASS, including `verifyWriterTableDialog`.

- [ ] **Step 7: Commit**

```bash
git add ui/js/desktop/shared/form-dialog.js ui/css/desktop-form-dialog.css ui/js/desktop/apps/writer.js ui/js/desktop/apps/writer-panels.js ui/js/desktop/core/module-loader.js ui/js/desktop/bundles/main.bundle.js ui/desktop_office_assets_test.go ui/desktop_writer_browser_test.go ui/desktop_writer_reload_browser_test.go ui/desktop_writer_shell_test.go ui/lang/desktop
git commit -m "feat(writer): shared form dialog, Insert table asks for rows and columns"
```

---

### Task 7: Colour tokens in the Writer chrome

Only chrome colours change. The page (`#fff`, `#192234`, the caret) and the template thumbnail stay as they are: paper is light in every theme by design.

**Files:**
- Modify: `ui/css/desktop-app-writer.css`

- [ ] **Step 1: Replace the hard-coded chrome colours**

| Rule | Old | New |
| --- | --- | --- |
| `.writer-document [data-state=dirty]::before` | `background:#dfac69` | `background:var(--ds-color-warning-fg,#dfac69)` |
| `.writer-document [data-state=error]` | `color:#f29a91` | `color:var(--ds-color-danger-fg,#f29a91)` |
| `.writer-notice[data-error=true]` | `#e89942 12%` | `var(--ds-color-danger-fg,#e89942) 12%` |
| `.writer-review-card del` | `color:#e59892` | `color:var(--ds-color-danger-fg,#e59892)` |
| `.writer-suggestion-original` | `border-left:2px solid #c97979` | `border-left:2px solid var(--ds-color-danger-fg,#c97979)` |
| `.writer-suggestion-text` | `border-left:2px solid #62b397` | `border-left:2px solid var(--ds-color-success-fg,#62b397)` |
| `.writer-ruler` | `color:#aab3c3` | `color:var(--writer-muted)` |
| `.writer-ruler` | both `#ffffff14` | `color-mix(in srgb,var(--writer-muted) 22%,transparent)` |
| `.writer-viewport` | `scrollbar-color:#65708480 transparent` | `scrollbar-color:color-mix(in srgb,var(--writer-muted) 50%,transparent) transparent` |
| `.writer-editor` | `--doc-scrollbar-thumb:#65708480` | `--doc-scrollbar-thumb:color-mix(in srgb,var(--writer-muted) 50%,transparent)` |

- [ ] **Step 2: Remove the override the tokens made redundant**

Delete the rule that only recoloured the ruler for light schemes:

```css
.desktop-body[data-theme=fruity][data-fruity-mode=light] .writer-ruler,.desktop-body[data-scheme="light"] .writer-ruler{color:#536175;background-image:repeating-linear-gradient(90deg,transparent,transparent calc(1.25% - 1px),#38496025 calc(1.25% - 1px),#38496025 1.25%)}
```

- [ ] **Step 3: Verify in the themes**

Run the browser smoke. Open the screenshots it writes for `standard`, `fruity-dark` and `fruity-light` (comfortable, 1366x768) and the three `-review.png` files.
Expected: ruler numbers and ticks are readable on the canvas in all three; the unsaved dot, a deleted-text line in a review card and the error notice keep a warning/danger colour that reads on its background. If the ruler ticks vanish in `fruity-light`, raise the mix from 22 % to 30 % and re-run.

- [ ] **Step 4: Commit**

```bash
git add ui/css/desktop-app-writer.css
git commit -m "style(writer): status and ruler colours come from theme tokens"
```

---

### Task 8: Documentation and final verification

**Files:**
- Modify: `ui/js/desktop/apps/AGENTS.md`, `documentation/autor.md`

- [ ] **Step 1: `ui/js/desktop/apps/AGENTS.md`**

Replace the three ownership lines

```
- `writer.js` owns Autor's native DOCX document, menus and lifecycle;
  `writer-session.js` owns revision-bound autosave and IndexedDB recovery;
  `writer-panels.js` owns formatting, search, review, AI and snapshot printing.
```

with

```
- `writer.js` owns Autor's native DOCX document, file operations and lifecycle;
  `writer-commands.js` owns the command table every surface reads (menus,
  toolbar, shortcuts, tooltips), the one "blocked while suggesting" rule, the
  `can`/`run` gate and the colour conversion; `writer-toolbar.js` renders the
  toolbar from that table and owns its overflow popover and roving tabindex;
  `writer-session.js` owns revision-bound autosave and IndexedDB recovery;
  `writer-panels.js` owns formatting, search, review, AI and snapshot printing.
  A new command is one entry in `DEFINITIONS`; a toolbar control is its id in a
  `GROUPS` entry. Never test `editingMode==='suggesting'` to gate a command:
  ask `commands.blocked(type)` or `commands.can(command)`. Toolbar controls
  carry `data-tool`, because the panels JSON-parse every `[data-command]`.
  `shared/form-dialog.js` (`window.AuraFormDialog`) is the multi-field dialog;
  it mounts on the body so an app's key handlers never see its keys. Verify
  `npm run test:writer`.
```

In the following bullet replace `Autor's toolbar names the shared icon key per slot (`slotIcons`) and per action (`actionIcons`); an action id is never an icon name.` with `Autor's command table names the shared icon key of every command (`icon`); a command id is never an icon name.`, replace `draws those few glyphs itself in `writer.js` (`.writer-glyph`)` with `draws those few glyphs itself in `writer-toolbar.js` (`.writer-glyph`)`, and replace `both convert the picked colour through `markCommand`` with `both convert the picked colour through `commands.valueCommand``.

- [ ] **Step 2: `documentation/autor.md`**

In the section "Writing", after the sentence `The toolbar contains paragraph styles, fonts, numeric sizes, common formatting and lists.` add:

```
When the window is too narrow for the whole bar, groups move into a More tools
popover instead of scrolling: lists first, then alignment, colours, insert,
text marks and the font controls; undo and redo always stay. The bar is one
tab stop; Left and Right move between its controls.
```

After the sentence that ends `page numbers and document templates.` add:

```
A table is inserted from a dialog that asks for rows (1 to 100) and columns
(1 to 30).
```

In the verification section at the end of the file, add `npm run test:writer` covers `scripts/test-writer-session.mjs`, `scripts/test-writer-commands.mjs` and `scripts/test-writer-toolbar.mjs`.

- [ ] **Step 3: Full verification**

Run, in this order:

1. `npm run test:writer` — expected: three pass lines.
2. `go test ./ui -run "(?i)(office|writer|virtualdesktop|lifecycle|i18n)" -count=1` — expected: `ok`.
3. `npm run check:ui` — expected: no drift.
4. `node scripts/build-writer-vendor.js --check` — expected: pass (the vendored engine was not touched).
5. Browser smoke including `TestDesktopWriterReloadsChangedFileOnReopen`:
   `AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run "TestDesktopWriter(App|Engine)Browser|TestDesktopWriterReloadsChangedFileOnReopen" -count=1 -timeout 30m` — expected: PASS.
6. `wc -l ui/js/desktop/apps/writer*.js ui/js/desktop/shared/form-dialog.js` — expected: every file far below 1100; `writer.js` shorter than its 712 lines before this plan.
7. `git grep -n "editingMode==='suggesting'" -- ui/js/desktop/apps/writer.js ui/js/desktop/apps/writer-panels.js` — expected: exactly one hit, the paste handler in `writer.js`.

- [ ] **Step 4: Commit**

```bash
git add ui/js/desktop/apps/AGENTS.md documentation/autor.md
git commit -m "docs(writer): command registry, toolbar module and form dialog"
```

---

## Out of scope for this plan

- New commands, the right-click menu, the contextual bar, complete menus and the remaining dialogs: Wave 1, its own plan.
- A spacing and size scale for the Writer CSS: done in Wave 1 together with the chrome it changes.
- Any change to the vendored engine bundle or to `writer-session.js`.
