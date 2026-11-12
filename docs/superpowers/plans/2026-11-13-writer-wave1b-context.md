# Writer Wave 1b — right-click menu and contextual bar: implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A right-click in the document opens a menu that fits what was clicked, and a contextual bar appears over the ruler while the caret is in a table, an image is selected, or a header or footer is being edited.

**Architecture:** One new module, `writer-context.js`, owns both surfaces. Its pure part says which bar belongs to a snapshot and which sections a right-click menu has; its DOM part renders the bar from the command registry and builds the menu through the desktop shell's `showContextMenu`. The registry gains the few commands the two surfaces need. The window menus list the same commands, so nothing exists only in a context surface.

**Tech stack:** Vanilla JS classic scripts (no bundler), `@docx-editor.dev/core` 2.29.0 behind `lib`, node `vm` tests, Go + rod browser tests.

Roadmap: `docs/superpowers/specs/2026-11-12-writer-office-roadmap-design.md`, section "Wave 1 — Interaction". This is part 1b of three; 1a (commands, menus, dialogs, zoom) is on `main`, 1c (start state, recent documents) follows.

---

## Read this first

**Decided by the user (2026-11-13):** the contextual bar sits directly under the toolbar and covers the ruler while it is shown. The document must not move when the bar appears or disappears.

**Scope.** In: right-click menu inside the document; contextual bar for table, image and header/footer; the commands both need (table borders, rotate and delete an image, "different first page", "different odd and even pages", add a comment); the same commands in the window menus. Out: start state and recent documents (1c); image wrap, size and alt text in the bar; cell fill in the bar; a bar for footnotes; page setup and paragraph dialogs (Wave 2).

**Engine behaviour, probed in the browser on 2026-11-13.** These are facts, not assumptions:

- A right-click moves the caret to the click point, exactly like a left click, and so collapses an existing selection. The engine does not call `preventDefault` on `contextmenu` and opens no menu of its own.
- Stopping a right-button `pointerdown` in the capture phase on the editor mount (`preventDefault` + `stopPropagation`) keeps the selection; `contextmenu` still fires and the focus stays in the document.
- The engine paints the selection without child elements, so the app cannot tell whether a point lies inside the selection. Rule for this plan: a right-click never changes a non-collapsed selection or a selected image; with a collapsed caret the click moves the caret.
- An image is `.docx-drawing` with `data-drawing-node-id`. A left click selects it (`snapshot().image` is set, with `rotationDegrees`, `wrap`, `kind`). A right-click on an unselected image does not select it.
- `{type:'transformImage',action:'rotateCW'}` and `{type:'deleteImage'}` work through `editor.exec` on the selected image. `setImageWrapType` is refused for an image Writer inserted ("wrap cannot be changed on this drawing"), which is why wrap is not in the bar.
- `{type:'setHeaderFooterOptions',titlePage:true}` and `{evenAndOddHeaders:true}` work. `editor.getHeaderFooterState()` returns `{editing:'header'|'footer', titlePage, evenAndOddHeaders, ...}` while a header or footer is open and `null` in the body.
- `snapshot().table` is `{rows, columns, rowIndex, columnIndex}` or `null`; `snapshot().scope.kind` is `'body'`, `'headerFooter'`, `'note'` or `'frame'`.
- After `transformImage`, a programmatic `setSelection` to a text caret leaves `snapshot().image` set; a real click into the text clears it.
- `editor.can({type:'removeHyperlink'})` is `ok` everywhere, so "is the caret in a link" is asked with `editor.query({type:'hyperlinkAt'})`.

**From the type definitions, not probed.** The browser check in Task 3 is their proof; if it fails there, stop and report the thrown message:

- `{type:'deleteImage', drawingNodeId}` deletes an image that is not selected.
- `editor.getReviewItemsAt(clientX, clientY)` returns `[{placement:{key, kind:'revision'|'comment', readOnly}, rect}]`, empty where there is nothing.
- `snapshot().tocContext` is `{id}` after a right-click on a table of contents (the engine sets it in its own `contextmenu` listener, which runs before the app's), and `{type:'refreshToc', tocId}` updates it.
- `insertHyperlink {href}` over a plain selection turns the selection into a link.
- Shift+F10 makes the engine dispatch a `contextmenu` event with `button: -1` and no coordinates.

**Shell facts** (`ui/js/desktop/core/menus-and-routing.js`):

- `ctx.showContextMenu(x, y, items)`: an item is `{id, label, icon, disabled, disabledHint, danger, action}`, `{separator:true}`, or a submenu `{id, label, icon, items}`. There is no `checked`. An item without `icon` gets a default tool icon, and an unknown icon key is drawn as its name in text, so every item names an icon that exists. Known to exist: `cut`, `copy`, `paste`, `link`, `chat`, `trash`, `check`, `columns`, `star`, `undo`, `redo`, `list`, `arrow-up`, `arrow-down`, `image`, `edit`.
- `writer.js` already calls `ctx.wireContextMenuBoundary?.(host)`, which suppresses the browser's menu inside the window unless the event is already `defaultPrevented`. Writer's own listener sits deeper (on the editor mount), runs first and prevents the default, so the boundary call stays as it is.

**Repo rules:**

- Read the `AGENTS.md` chain for every path you touch before editing (root, `ui/AGENTS.md`, `ui/js/desktop/apps/AGENTS.md`); close out with a DOX pass (Task 5).
- Every user-visible string exists in all 16 files under `ui/lang/desktop/`. `scripts/test-writer-commands.mjs` scans every registry label and every literal `tr('key')` in `ui/js/desktop/apps/writer*.js`; write `tr('key')` with a literal key.
- Every file under `ui/js/desktop` stays below 1100 lines. No Internet assets, no new dependencies.
- `ui/desktop_office_app_markers_test.go` pins these strings in `writer.js`: `window.WriterApp={render,dispose,instances}`, `queue?.dispose()`, `dispose(windowId);`, `editor?.destroy()`, `/api/desktop/office/document`.
- Toolbar and bar controls carry `data-tool`; `[data-command]` belongs to the panels. A command is one entry in `DEFINITIONS` in `writer-commands.js`; `scripts/test-writer-menus.mjs` fails for a command no window menu lists.
- Never test `editingMode==='suggesting'` to gate a command; ask `commands.blocked(type)`, `commands.can(command)` or `commands.tracking()`.
- Do not touch `ui/js/desktop/apps/writer-session.js`.
- Temporary scripts go into `disposable/_writer-wave1b/` and are deleted before the commit. Before each commit: `git diff --cached --check`, and look at `git diff --cached`.
- GitNexus: the index is hundreds of commits behind and cannot index worktrees. Controller's impact analysis for `writer.js`: UNKNOWN, callers confirmed by text search (`window.WriterApp` is used by the shell loader and the Go tests; the state object by the Writer modules). End each commit body with: `GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.`

**Environment (Windows):**

- Work in a worktree with a real copy of `node_modules` (`robocopy <main>\node_modules <worktree>\node_modules /E /MT:16 /NFL /NDL /NJH /NP`).
- Browser tests: `AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run "TestDesktop(Writer|QR)" -count=1 -timeout 30m -v`, from PowerShell with the sandbox off. Delete `reports\autor` first. Implementer subagents cannot start Chrome; the controller runs these.
- Static tests: `npm run test:writer`, `npm run check:ui`, `go test ./ui -run "(?i)(windowmenu|contextmenu|materials|office|writer|virtualdesktop|lifecycle|i18n)" -count=1`.
- `TestDesktopCodeStudioRunUsesTerminalSession` fails on `main` for an unrelated reason. Ignore it.

## File structure

| File | Change | Responsibility |
| --- | --- | --- |
| `ui/js/desktop/apps/writer-commands.js` | modify | New definitions, glyphs on the table commands, a `scope` field |
| `ui/js/desktop/apps/writer-toolbar.js` | modify | Thirteen new glyphs |
| `ui/js/desktop/apps/writer-context.js` | create | Bar and right-click model (pure); bar rendering, selection guard and menu (DOM) |
| `ui/js/desktop/apps/writer-dialogs.js` | modify | `comment()` |
| `ui/js/desktop/apps/writer-panels.js` | modify | The panel's comment button uses the same action |
| `ui/js/desktop/apps/writer-menus.js` | modify | The new commands in Insert, Format and Table |
| `ui/js/desktop/apps/writer.js` | modify | Bar element, wiring, two header options, `comment`, menu signature |
| `ui/css/desktop-app-writer.css` | modify | The bar |
| `ui/js/desktop/core/module-loader.js` | modify | One new script |
| `ui/lang/desktop/*.json` | modify | 6 new keys in 16 languages |
| `scripts/test-writer-context.mjs` | create | Bar and menu model |
| `scripts/test-writer-commands.mjs`, `scripts/test-writer-toolbar.mjs` | modify | New rules |
| `ui/desktop_writer_context_test.go` | create | Browser check |
| `ui/desktop_office_assets_test.go`, three fixtures | modify | Script lists; the fixture's `showContextMenu` |
| `ui/js/desktop/apps/AGENTS.md`, `documentation/autor.md` | modify | Documentation |

---

### Task 1: Registry: the commands the two surfaces need

**Files:**
- Modify: `ui/js/desktop/apps/writer-commands.js`, `ui/js/desktop/apps/writer-toolbar.js`
- Modify: `scripts/test-writer-commands.mjs`, `scripts/test-writer-toolbar.mjs`
- Modify: `ui/lang/desktop/*.json` (script)

- [ ] **Step 1: Add the language keys**

Create `disposable/_writer-wave1b/i18n.mjs`:

```js
// Adds the Wave 1b keys to the sixteen desktop language files, each at its sorted place in the desktop.writer_ block.
// Usage: node disposable/_writer-wave1b/i18n.mjs <repo root>
import {readFileSync, writeFileSync} from 'node:fs';
import path from 'node:path';

const root = process.argv[2] || process.cwd();
const LANGS = ['cs', 'da', 'de', 'el', 'en', 'es', 'fr', 'hi', 'it', 'ja', 'nl', 'no', 'pl', 'pt', 'sv', 'zh'];
const KEYS = {
    rotate_left: ['Otočit doleva', 'Roter mod venstre', 'Nach links drehen', 'Περιστροφή αριστερά', 'Rotate left', 'Girar a la izquierda', 'Faire pivoter à gauche', 'बाईं ओर घुमाएँ', 'Ruota a sinistra', '左に回転', 'Linksom draaien', 'Roter mot venstre', 'Obróć w lewo', 'Rodar para a esquerda', 'Rotera åt vänster', '向左旋转'],
    rotate_right: ['Otočit doprava', 'Roter mod højre', 'Nach rechts drehen', 'Περιστροφή δεξιά', 'Rotate right', 'Girar a la derecha', 'Faire pivoter à droite', 'दाईं ओर घुमाएँ', 'Ruota a destra', '右に回転', 'Rechtsom draaien', 'Roter mot høyre', 'Obróć w prawo', 'Rodar para a direita', 'Rotera åt höger', '向右旋转'],
    image_delete: ['Odstranit obrázek', 'Slet billede', 'Bild löschen', 'Διαγραφή εικόνας', 'Delete image', 'Eliminar imagen', 'Supprimer l’image', 'छवि हटाएँ', 'Elimina immagine', '画像を削除', 'Afbeelding verwijderen', 'Slett bilde', 'Usuń obraz', 'Eliminar imagem', 'Ta bort bild', '删除图片'],
    header_first_page: ['Jiná první stránka', 'Anden første side', 'Erste Seite anders', 'Διαφορετική πρώτη σελίδα', 'Different first page', 'Primera página diferente', 'Première page différente', 'पहला पृष्ठ अलग', 'Prima pagina diversa', '先頭ページのみ別指定', 'Eerste pagina afwijkend', 'Annerledes førsteside', 'Inna pierwsza strona', 'Primeira página diferente', 'Annorlunda förstasida', '首页不同'],
    header_odd_even: ['Jiné liché a sudé stránky', 'Forskellige lige og ulige sider', 'Gerade und ungerade Seiten anders', 'Διαφορετικές μονές και ζυγές σελίδες', 'Different odd and even pages', 'Páginas pares e impares diferentes', 'Pages paires et impaires différentes', 'विषम और सम पृष्ठ अलग', 'Pagine pari e dispari diverse', '奇数/偶数ページ別指定', "Even en oneven pagina's afwijkend", 'Ulike oddetalls- og partallssider', 'Inne strony parzyste i nieparzyste', 'Páginas pares e ímpares diferentes', 'Olika udda och jämna sidor', '奇偶页不同'],
    toc_update: ['Aktualizovat obsah', 'Opdater indholdsfortegnelse', 'Inhaltsverzeichnis aktualisieren', 'Ενημέρωση πίνακα περιεχομένων', 'Update table of contents', 'Actualizar tabla de contenido', 'Mettre à jour la table des matières', 'विषय-सूची अपडेट करें', 'Aggiorna sommario', '目次を更新', 'Inhoudsopgave bijwerken', 'Oppdater innholdsfortegnelse', 'Zaktualizuj spis treści', 'Atualizar índice', 'Uppdatera innehållsförteckning', '更新目录']
};
for (const [name, texts] of Object.entries(KEYS)) if (texts.length !== LANGS.length) throw new Error(name + ' has ' + texts.length + ' texts');
for (const [index, lang] of LANGS.entries()) {
    const file = path.join(root, 'ui/lang/desktop', lang + '.json');
    const lines = readFileSync(file, 'utf8').split('\n');
    const indent = /^(\s*)"desktop\.writer_/.exec(lines.find(line => /^\s*"desktop\.writer_/.test(line)))[1];
    let added = 0;
    for (const [name, texts] of Object.entries(KEYS)) {
        const key = 'desktop.writer_' + name;
        const block = lines.map((line, at) => [/^\s*"(desktop\.writer_[a-z0-9_]+)":/.exec(line)?.[1], at]).filter(([found]) => found);
        if (block.some(([found]) => found === key)) continue;
        const next = block.find(([found]) => found > key);
        lines.splice(next ? next[1] : block.at(-1)[1] + 1, 0, indent + JSON.stringify(key) + ': ' + JSON.stringify(texts[index]) + ',');
        added++;
    }
    const text = lines.join('\n');
    JSON.parse(text);
    writeFileSync(file, text);
    console.log(lang + ': ' + added + ' keys added');
}
```

Run from the repo root: `node disposable/_writer-wave1b/i18n.mjs .`

Expected: sixteen lines, each `<lang>: 6 keys added`; `git diff --stat -- ui/lang/desktop` shows 16 files, 6 insertions each, no deletions. Then delete `disposable/_writer-wave1b/`.

- [ ] **Step 2: Extend the registry test**

In `scripts/test-writer-commands.mjs`, insert this block directly before the line `// The table itself.`:

```js
{ // A command can belong to one editing scope; outside it the command is off.
    const body = harness();
    assert.equal(body.commands.state('headerFirstPage').enabled, false);
    assert.equal(body.commands.invoke('headerFirstPage'), false);
    assert.equal(body.log.acted.length, 0);
    const header = harness({snapshot: {scope: {kind: 'headerFooter'}}, open: ['headerFirstPage']});
    assert.equal(header.commands.state('headerFirstPage').enabled, true);
    assert.equal(header.commands.state('headerFirstPage').active, true);
    assert.equal(header.commands.state('headerOddEven').active, false);
    assert.equal(header.commands.invoke('headerOddEven'), true);
    assert.equal(header.log.acted[0], 'headerOddEven');
    assert.equal(harness({snapshot: {scope: {kind: 'headerFooter'}}, readonly: true}).commands.state('headerFirstPage').enabled, false);
}
{ // The Wave 1b engine commands carry their exact shapes.
    const {commands, log} = harness();
    for (const id of ['rotateLeft', 'rotateRight', 'deleteImage', 'borders', 'noBorders']) assert.equal(commands.invoke(id), true, id);
    assert.equal(log.exec.map(command => command.type + ':' + (command.action || command.scope || '')).join(), 'transformImage:rotateCCW,transformImage:rotateCW,deleteImage:,setTableBorders:all,setTableBorders:none');
    assert.equal(log.exec[3].spec.style, 'single');
    assert.equal(log.exec[4].target, 'all');
    assert.equal(commands.state('comment').enabled, true);
    assert.equal(harness({readonly: true}).commands.state('comment').enabled, false);
    for (const id of ['rowAbove', 'rowBelow', 'columnLeft', 'columnRight', 'deleteRow', 'deleteColumn', 'deleteTable', 'mergeCells', 'splitCell', 'borders', 'noBorders', 'rotateLeft', 'rotateRight'])
        assert.ok(commands.get(id).glyph, id + ' has a glyph for the contextual bar');
}
```

In the block of table rules near the end of the file, after the line that starts with `    if (definition.options) assert.equal(`, add:

```js
    if (definition.scope) assert.ok(!definition.slot && !definition.command, definition.id + ': only app actions are scoped');
```

In `scripts/test-writer-toolbar.mjs`, after the line that starts with `for (const key of ['table', 'clear-format'])`, add:

```js
for (const definition of WriterCommands.definitions) if (definition.glyph) assert.ok(icon(definition.glyph, '?').startsWith('<svg class="writer-glyph"'), definition.glyph + ' is drawn');
```

Run: `npm run test:writer`
Expected: FAIL in `test-writer-commands.mjs` with `AssertionError ... false !== true` (the command `headerFirstPage` does not exist yet, so it is never enabled).

- [ ] **Step 3: Add the definitions**

In `ui/js/desktop/apps/writer-commands.js`:

1. In the field comment above `DEFINITIONS`, after the line that starts with `    //   write      refused in a read-only window`, add:
```js
    //   scope      editing scope ('headerFooter') an app action belongs to; refused anywhere else
```

2. Replace the nine table lines
```js
        {id: 'rowAbove', command: {type: 'insertRow', where: 'above'}, label: 'row_above'},
        {id: 'rowBelow', command: {type: 'insertRow', where: 'below'}, label: 'row_below'},
        {id: 'columnLeft', command: {type: 'insertColumn', where: 'left'}, label: 'column_left'},
        {id: 'columnRight', command: {type: 'insertColumn', where: 'right'}, label: 'column_right'},
        {id: 'deleteRow', command: {type: 'deleteRow'}, label: 'delete_row'},
        {id: 'deleteColumn', command: {type: 'deleteColumn'}, label: 'delete_column'},
        {id: 'deleteTable', command: {type: 'deleteTable'}, label: 'table_delete'},
        {id: 'mergeCells', command: {type: 'mergeCells'}, label: 'merge_cells'},
        {id: 'splitCell', command: {type: 'splitCell', rows: 1, cols: 2}, label: 'split_cell'},
```
with
```js
        {id: 'rowAbove', command: {type: 'insertRow', where: 'above'}, label: 'row_above', glyph: 'row-above', fallback: '↥'},
        {id: 'rowBelow', command: {type: 'insertRow', where: 'below'}, label: 'row_below', glyph: 'row-below', fallback: '↧'},
        {id: 'columnLeft', command: {type: 'insertColumn', where: 'left'}, label: 'column_left', glyph: 'col-left', fallback: '↤'},
        {id: 'columnRight', command: {type: 'insertColumn', where: 'right'}, label: 'column_right', glyph: 'col-right', fallback: '↦'},
        {id: 'deleteRow', command: {type: 'deleteRow'}, label: 'delete_row', glyph: 'row-delete', fallback: '⊟'},
        {id: 'deleteColumn', command: {type: 'deleteColumn'}, label: 'delete_column', glyph: 'col-delete', fallback: '⊟'},
        {id: 'deleteTable', command: {type: 'deleteTable'}, label: 'table_delete', glyph: 'table-delete', fallback: '⊠'},
        {id: 'mergeCells', command: {type: 'mergeCells'}, label: 'merge_cells', glyph: 'merge-cells', fallback: '⧉'},
        {id: 'splitCell', command: {type: 'splitCell', rows: 1, cols: 2}, label: 'split_cell', glyph: 'split-cell', fallback: '◫'},
        {id: 'borders', command: {type: 'setTableBorders', scope: 'all', spec: {style: 'single', size: 4, color: {kind: 'hex', value: '64748B'}}}, label: 'borders', glyph: 'borders-all', fallback: '▦'},
        {id: 'noBorders', command: {type: 'setTableBorders', scope: 'none', target: 'all'}, label: 'no_borders', glyph: 'borders-none', fallback: '▢'},
        {id: 'rotateLeft', command: {type: 'transformImage', action: 'rotateCCW'}, label: 'rotate_left', glyph: 'rotate-left', fallback: '↺'},
        {id: 'rotateRight', command: {type: 'transformImage', action: 'rotateCW'}, label: 'rotate_right', glyph: 'rotate-right', fallback: '↻'},
        {id: 'deleteImage', command: {type: 'deleteImage'}, label: 'image_delete', icon: 'trash'},
        {id: 'comment', label: 'comment', icon: 'chat', write: true, needsEditor: true},
        {id: 'headerFirstPage', label: 'header_first_page', write: true, needsEditor: true, scope: 'headerFooter'},
        {id: 'headerOddEven', label: 'header_odd_even', write: true, needsEditor: true, scope: 'headerFooter'},
```

3. In `state(id)`, replace
```js
            if ((definition.write && host.readonly) || (definition.needsEditor && !editor)) return off;
```
with
```js
            if ((definition.write && host.readonly) || (definition.needsEditor && !editor)) return off;
            if (definition.scope && editor.snapshot().scope?.kind !== definition.scope) return off;
```

- [ ] **Step 4: Draw the glyphs**

In `ui/js/desktop/apps/writer-toolbar.js`, replace the last entry of `GLYPHS`
```js
        'clear-format': '<path d="M5 5h11M10.5 5l-2.5 12"/><path d="M14.5 14.5l5 5M19.5 14.5l-5 5"/>'
```
with
```js
        'clear-format': '<path d="M5 5h11M10.5 5l-2.5 12"/><path d="M14.5 14.5l5 5M19.5 14.5l-5 5"/>',
        'row-above': '<rect x="4" y="11" width="16" height="9" rx="1"/><path d="M4 15.5h16M12 11v9"/><path d="M12 3v5M9.5 5.5h5"/>',
        'row-below': '<rect x="4" y="4" width="16" height="9" rx="1"/><path d="M4 8.5h16M12 4v9"/><path d="M12 16v5M9.5 18.5h5"/>',
        'col-left': '<rect x="11" y="4" width="9" height="16" rx="1"/><path d="M15.5 4v16M11 12h9"/><path d="M3 12h5M5.5 9.5v5"/>',
        'col-right': '<rect x="4" y="4" width="9" height="16" rx="1"/><path d="M8.5 4v16M4 12h9"/><path d="M16 12h5M18.5 9.5v5"/>',
        'row-delete': '<rect x="4" y="4" width="16" height="16" rx="1"/><path d="M4 9.5h16M4 14.5h16"/><path d="M10 10.5l4 3M14 10.5l-4 3"/>',
        'col-delete': '<rect x="4" y="4" width="16" height="16" rx="1"/><path d="M9.5 4v16M14.5 4v16"/><path d="M10.5 10l3 4M13.5 10l-3 4"/>',
        'table-delete': '<rect x="4" y="5" width="16" height="14" rx="1.5"/><path d="M4 10h16M10 5v14"/><path d="M13.5 12.5l4 4M17.5 12.5l-4 4"/>',
        'merge-cells': '<rect x="4" y="6" width="16" height="12" rx="1"/><path d="M12 6v2.5M12 15.5V18"/><path d="M8 12h8M10 10l-2 2 2 2M14 10l2 2-2 2"/>',
        'split-cell': '<rect x="4" y="6" width="16" height="12" rx="1"/><path d="M12 6v12"/><path d="M6.5 12H9M15 12h2.5"/>',
        'borders-all': '<rect x="4" y="4" width="16" height="16" rx="1"/><path d="M12 4v16M4 12h16"/>',
        'borders-none': '<rect x="4" y="4" width="16" height="16" rx="1" stroke-dasharray="2 2.5"/><path d="M12 4v16M4 12h16" stroke-dasharray="2 2.5"/>',
        'rotate-left': '<path d="M5 9a8 8 0 1 1-1 6"/><path d="M5 4v5h5"/>',
        'rotate-right': '<path d="M19 9a8 8 0 1 0 1 6"/><path d="M19 4v5h-5"/>'
```

- [ ] **Step 5: Run the tests**

Run: `npm run test:writer`
Expected: `test-writer-commands.mjs` and `test-writer-toolbar.mjs` pass. `test-writer-menus.mjs` now FAILS with `only toolbar controls and the insert panel stay out of the menus`, because the new commands are not in a window menu yet. That is expected until Task 4; do not change that test.

Run: `go test ./ui -run "(?i)(windowmenu|contextmenu|materials|office|writer|virtualdesktop|lifecycle|i18n)" -count=1`
Expected: `ok  	aurago/ui`

- [ ] **Step 6: Commit**

```bash
git add ui/js/desktop/apps/writer-commands.js ui/js/desktop/apps/writer-toolbar.js scripts/test-writer-commands.mjs scripts/test-writer-toolbar.mjs ui/lang/desktop
git diff --cached --check
git commit -m "feat(writer): commands for table borders, image rotation and header options"
```

---

### Task 2: The context module: which bar, which menu

**Files:**
- Create: `ui/js/desktop/apps/writer-context.js`, `scripts/test-writer-context.mjs`
- Modify: `package.json`

- [ ] **Step 1: Write the failing test**

Create `scripts/test-writer-context.mjs`:

```js
import assert from 'node:assert/strict';
import vm from 'node:vm';
import {readFileSync} from 'node:fs';

const context = {window: {}};
vm.runInNewContext(readFileSync('ui/js/desktop/apps/writer-commands.js', 'utf8'), context);
vm.runInNewContext(readFileSync('ui/js/desktop/apps/writer-context.js', 'utf8'), context);
const {WriterCommands, WriterContext} = context.window;
const known = new Set(WriterCommands.definitions.map(definition => definition.id));

// What the caret is in decides the bar: an image before a header or footer, before a table.
const {kind} = WriterContext;
assert.equal(kind(null), '');
assert.equal(kind({scope: {kind: 'body'}}), '');
assert.equal(kind({scope: {kind: 'body'}, table: {rows: 2}}), 'table');
assert.equal(kind({scope: {kind: 'headerFooter'}, table: {rows: 2}}), 'headerFooter');
assert.equal(kind({scope: {kind: 'headerFooter'}, table: {rows: 2}, image: {id: 'i'}}), 'image');
assert.equal(kind({scope: {kind: 'note'}}), '');

// Every bar names registry commands; the ones drawn as text are part of the bar.
assert.equal(Object.keys(WriterContext.bars).join(), 'table,image,headerFooter');
for (const [name, bar] of Object.entries(WriterContext.bars)) {
    const ids = bar.items.filter(id => id !== '|');
    assert.equal(new Set(ids).size, ids.length, name + ' lists no command twice');
    for (const id of ids) assert.ok(known.has(id), name + ' bar item ' + id + ' is not a registry command');
    for (const id of bar.text || []) assert.ok(ids.includes(id), name + ' draws ' + id + ' as text but does not hold it');
    assert.notEqual(bar.items[0], '|');
    assert.notEqual(bar.items.at(-1), '|');
}
for (const id of WriterContext.bars.table.items.filter(id => id !== '|')) assert.ok(WriterCommands.definitions.find(definition => definition.id === id).glyph, 'table bar item ' + id + ' needs a glyph');

// The right-click menu: sections of registry ids, '@' names for items only the app can build, one submenu.
const flatten = sections => sections.flat().flatMap(entry => typeof entry === 'string' ? [entry] : entry.items.filter(id => id !== '|'));
const custom = new Set(['@accept', '@reject', '@unlink', '@deleteDrawing', '@updateToc']);
const plain = WriterContext.sections({review: '', link: false, table: false, image: false, drawing: '', toc: '', scope: 'body'});
assert.equal(plain.filter(section => section.length).map(section => section.join()).join(' / '), 'cut,copy,paste,pastePlain / link / comment / assist');
const everything = WriterContext.sections({review: 'r1', link: true, table: true, image: true, drawing: '', toc: 't1', scope: 'headerFooter'});
for (const entry of flatten(everything)) assert.ok(entry.startsWith('@') ? custom.has(entry) : known.has(entry), 'menu entry ' + entry + ' is unknown');
assert.equal(everything[0].join(), '@accept,@reject', 'a change under the pointer comes first');
assert.ok(flatten(everything).includes('@unlink'));
assert.ok(flatten(everything).includes('@updateToc'));
assert.ok(flatten(everything).includes('backDocument'));
assert.ok(flatten(everything).includes('rotateRight'));
assert.ok(!flatten(everything).includes('@deleteDrawing'), 'a selected image uses the registry commands');
const submenu = everything.flat().find(entry => typeof entry !== 'string');
assert.equal(submenu.submenu, 'table');
assert.equal(submenu.items.join(), WriterContext.bars.table.items.join(), 'the table submenu is the table bar');
const unselected = WriterContext.sections({review: '', link: false, table: false, image: false, drawing: 'd1', toc: '', scope: 'body'});
assert.ok(flatten(unselected).includes('@deleteDrawing'));
assert.ok(!flatten(unselected).includes('rotateRight'), 'rotation needs the image selected');
console.log('Writer context: bar kinds, bar contents and right-click sections pass');
```

In `package.json`, append ` && node scripts/test-writer-context.mjs` to the `test:writer` script.

Run: `node scripts/test-writer-context.mjs`
Expected: FAIL with `ENOENT ... writer-context.js`.

- [ ] **Step 2: Write the module**

Create `ui/js/desktop/apps/writer-context.js`:

```js
(function () {
    'use strict';
    // What the caret is in decides the contextual bar: a selected image wins over a header or
    // footer, which wins over a table.
    function kind(snapshot) {
        if (!snapshot) return '';
        if (snapshot.image) return 'image';
        if (snapshot.scope?.kind === 'headerFooter') return 'headerFooter';
        if (snapshot.table) return 'table';
        return '';
    }
    // Bar contents by kind: command ids, '|' for a divider. `text` lists the ids drawn with
    // their label instead of an icon. A header or footer bar is titled by what is being edited.
    const BARS = {
        table: {items: ['rowAbove', 'rowBelow', 'columnLeft', 'columnRight', '|', 'deleteRow', 'deleteColumn', 'deleteTable', '|', 'mergeCells', 'splitCell', '|', 'borders', 'noBorders']},
        image: {items: ['rotateLeft', 'rotateRight', '|', 'deleteImage', 'format'], text: ['deleteImage', 'format']},
        headerFooter: {items: ['pageNumber', 'headerFirstPage', 'headerOddEven', '|', 'backDocument'], text: ['pageNumber', 'headerFirstPage', 'headerOddEven', 'backDocument']}
    };
    // The right-click menu as sections. An entry is a registry id, '@name' for an item only the
    // app can build, or {submenu, items}. target: {review, link, table, image, drawing, toc, scope}.
    function sections(target) {
        return [
            target.review ? ['@accept', '@reject'] : [],
            ['cut', 'copy', 'paste', 'pastePlain'],
            target.link ? ['link', '@unlink'] : ['link'],
            ['comment'],
            target.table ? [{submenu: 'table', items: BARS.table.items}] : [],
            target.image ? ['rotateLeft', 'rotateRight', 'deleteImage'] : target.drawing ? ['@deleteDrawing'] : [],
            target.toc ? ['@updateToc'] : [],
            target.scope === 'headerFooter' ? ['backDocument'] : [],
            ['assist']
        ];
    }
    // The shell draws an unknown icon key as text, so every menu item names an icon that exists.
    const ICONS = {pastePlain: 'paste', rowAbove: 'arrow-up', rowBelow: 'arrow-down', columnLeft: 'columns', columnRight: 'columns', deleteRow: 'trash',
        deleteColumn: 'trash', deleteTable: 'trash', mergeCells: 'columns', splitCell: 'columns', borders: 'columns', noBorders: 'columns',
        rotateLeft: 'undo', rotateRight: 'redo', backDocument: 'undo'};
    const PRESSABLE = new Set(['headerFirstPage', 'headerOddEven', 'format']);
    // app: the state facade of writer.js ({root, tr, esc, ctx, commands, editor, readonly, notice, refresh}).
    function create(app) {
        const {root, tr, esc, ctx, commands} = app, icon = window.WriterToolbar.icons(ctx, esc), life = new AbortController();
        const bar = root.querySelector('[data-contextbar]'), mount = root.querySelector('[data-editor]');
        let shown = '';
        function button(id, text) {
            const definition = commands.get(id), name = esc(commands.label(id));
            return '<button type="button" data-tool="' + id + '" title="' + esc(commands.tooltip(id)) + '" aria-label="' + name + '"' + (text ? ' class="writer-context-text"' : '') + '>' +
                (text ? name : icon(definition.glyph || definition.icon, definition.fallback || '')) + '</button>';
        }
        function heading(current) {
            if (current === 'headerFooter') return app.editor.getHeaderFooterState()?.editing === 'footer' ? tr('footer') : tr('header');
            return current === 'image' ? tr('image') : tr('table');
        }
        // Builds the bar when the context changes and keeps its buttons in step with the registry.
        function refresh() {
            const editor = app.editor, current = editor ? kind(editor.snapshot()) : '', title = current ? heading(current) : '';
            if (current + ':' + title !== shown) {
                shown = current + ':' + title;
                bar.hidden = !current;
                bar.setAttribute('aria-label', title);
                bar.innerHTML = !current ? '' : '<strong>' + esc(title) + '</strong>' + BARS[current].items.map(id => id === '|'
                    ? '<span class="writer-context-divider" aria-hidden="true"></span>' : button(id, (BARS[current].text || []).includes(id))).join('');
            }
            for (const el of bar.querySelectorAll('button[data-tool]')) {
                const id = el.dataset.tool, state = commands.state(id);
                el.disabled = !state.enabled;
                if (PRESSABLE.has(id)) el.setAttribute('aria-pressed', String(state.active));
                const tip = commands.tooltip(id) + (!state.enabled && state.reason ? ' · ' + state.reason : '');
                if (el.title !== tip) el.title = tip;
            }
        }
        bar.addEventListener('click', event => {
            const el = event.target.closest('button[data-tool]');
            if (el && !el.disabled) commands.invoke(el.dataset.tool);
        }, {signal: life.signal});
        // The engine moves its caret on a right-click like on a left one, which would drop the
        // selection the menu is about. With a selection or a selected image the press is kept
        // from the engine; with a plain caret the caret follows the click.
        mount.addEventListener('pointerdown', event => {
            const snapshot = event.button === 2 && app.editor ? app.editor.snapshot() : null;
            if (!snapshot || (snapshot.selectionCollapsed && !snapshot.image)) return;
            event.preventDefault();
            event.stopPropagation();
        }, {capture: true, signal: life.signal});
        // What the right-click is about. The engine has already noted a table of contents under the pointer.
        function target(event, point) {
            const editor = app.editor, snapshot = editor.snapshot(), drawing = event.target.closest?.('.docx-drawing');
            const hit = point ? editor.getReviewItemsAt?.(point[0], point[1])[0]?.placement : null;
            return {
                review: hit && hit.kind === 'revision' && !hit.readOnly && !app.readonly ? hit.key : '',
                link: !!editor.query({type: 'hyperlinkAt'}), table: !!snapshot.table, image: !!snapshot.image,
                drawing: !snapshot.image && drawing && !app.readonly ? drawing.dataset.drawingNodeId || '' : '',
                toc: !app.readonly && snapshot.tocContext?.id || '', scope: snapshot.scope?.kind || 'body'
            };
        }
        function review(key, accept) {
            const result = accept ? app.editor.acceptReviewItem(key) : app.editor.rejectReviewItem(key);
            if (!result.ok) app.notice(tr('command_unavailable') + (result.reason ? ' · ' + result.reason : ''), true);
            app.refresh();
        }
        const custom = {
            '@accept': at => ({id: 'accept', label: tr('accept'), icon: 'check', action: () => review(at.review, true)}),
            '@reject': at => ({id: 'reject', label: tr('reject'), icon: 'undo', action: () => review(at.review, false)}),
            '@unlink': () => ({id: 'unlink', label: tr('link_remove'), icon: 'link', disabled: app.readonly, action: () => commands.run({type: 'removeHyperlink'})}),
            '@deleteDrawing': at => ({id: 'deleteDrawing', label: tr('image_delete'), icon: 'trash', danger: true, action: () => commands.run({type: 'deleteImage', drawingNodeId: at.drawing})}),
            '@updateToc': at => ({id: 'updateToc', label: tr('toc_update'), icon: 'list', action: () => commands.run({type: 'refreshToc', tocId: at.toc})})
        };
        // A registry command as a context-menu item. The focus returns to the document after an engine command.
        function command(id) {
            const entry = commands.menuItem(id), definition = commands.get(id), action = entry.action;
            entry.icon = entry.icon || ICONS[id] || 'edit';
            if (definition.slot || definition.command) entry.action = () => { action(); app.editor?.focus(); };
            return entry;
        }
        function item(entry, at) {
            if (typeof entry !== 'string') return {id: entry.submenu, label: tr('table'), icon: 'columns', items: entry.items.map(id => id === '|' ? {separator: true} : command(id))};
            if (!entry.startsWith('@')) return command(entry);
            const made = custom[entry](at), action = made.action;
            made.action = () => { action(); app.editor?.focus(); };
            return made;
        }
        // Opens the menu for a contextmenu event inside the document; false when it is not Writer's to answer.
        function open(event) {
            if (!app.editor || !ctx.showContextMenu || !mount.contains(event.target)) return false;
            // Shift+F10 and the menu key arrive without a pointer position.
            const keyboard = event.button === -1 || (!event.clientX && !event.clientY), box = mount.getBoundingClientRect();
            const point = keyboard ? null : [event.clientX, event.clientY], at = target(event, point), items = [];
            for (const section of sections(at)) {
                if (!section.length) continue;
                if (items.length) items.push({separator: true});
                items.push(...section.map(entry => item(entry, at)));
            }
            ctx.showContextMenu(point ? point[0] : box.left + Math.min(120, box.width / 2), point ? point[1] : box.top + 60, items);
            return true;
        }
        mount.addEventListener('contextmenu', event => {
            if (!open(event)) return;
            event.preventDefault();
            event.stopPropagation();
        }, {signal: life.signal});
        return {refresh, open, dispose() { life.abort(); }};
    }
    window.WriterContext = {create, kind, bars: BARS, sections};
})();
```

- [ ] **Step 3: Run the tests**

Run: `node scripts/test-writer-context.mjs`
Expected: `Writer context: bar kinds, bar contents and right-click sections pass`

Run: `node --check ui/js/desktop/apps/writer-context.js`
Expected: no output.

Run: `node scripts/test-writer-commands.mjs`
Expected: pass. The language-key scan now also reads `writer-context.js`; every `tr('...')` in it names an existing key (`accept`, `reject`, `link_remove`, `image_delete`, `toc_update`, `table`, `image`, `header`, `footer`, `command_unavailable`).

- [ ] **Step 4: Commit**

```bash
git add ui/js/desktop/apps/writer-context.js scripts/test-writer-context.mjs package.json
git diff --cached --check
git commit -m "feat(writer): context module with the bar and right-click models"
```

---

### Task 3: Wire the bar and the menu into Writer

**Files:**
- Modify: `ui/js/desktop/apps/writer.js`, `ui/js/desktop/apps/writer-dialogs.js`, `ui/js/desktop/apps/writer-panels.js`, `ui/css/desktop-app-writer.css`
- Modify: `ui/js/desktop/core/module-loader.js`, `ui/desktop_office_assets_test.go`, `ui/desktop_writer_browser_test.go` (two fixtures, the fixture context, one call), `ui/desktop_writer_reload_browser_test.go` (one fixture)
- Create: `ui/desktop_writer_context_test.go`

- [ ] **Step 1: Write the browser check**

Create `ui/desktop_writer_context_test.go`. It uses real mouse events where the engine's own pointer handling matters (keeping a selection, moving a caret, selecting an image) and synthetic `contextmenu` events elsewhere. The JavaScript sits in Go raw strings and must not contain a backtick.

```go
package ui

import (
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

// writerContextPrelude adds what the context checks share: the bar, the menu the fixture's
// showContextMenu received, a caret setter and a right-click on an element.
const writerContextPrelude = `
        const bar=document.querySelector('[data-contextbar]'),tool=id=>bar.querySelector('[data-tool="'+id+'"]');
        const ids=items=>items.filter(x=>!x.separator).map(x=>x.id),entry=id=>window.contextMenu.items.find(x=>x.id===id);
        const caret=(from,to)=>{editor.exec({type:'selectAll'});const s=editor.surface.state().selection.anchor;
            editor.exec({type:'setSelection',range:{anchor:{...s,offset:from},head:{...s,offset:to===undefined?from:to}}});};
        const rightClick=el=>{const b=el.getBoundingClientRect();window.contextMenu=null;
            const e=new MouseEvent('contextmenu',{bubbles:true,cancelable:true,button:2,clientX:b.left+b.width/2,clientY:b.top+b.height/2});el.dispatchEvent(e);return e;};
        const run=text=>[...document.querySelectorAll('.docx-page .layout-run-text')].find(x=>x.textContent.includes(text));
`

// verifyWriterContext covers the right-click menu and the contextual bar on the blank document
// and leaves it blank: a selection survives a right-click, a plain caret follows it, the menu
// fits a link, a table and an image, and the bar appears for a table, a header and an image.
func verifyWriterContext(t *testing.T, page *rod.Page) {
	t.Helper()
	script := func(body string) string {
		return `async()=>{` + writerWave1Prelude + writerContextPrelude + body + `}`
	}
	settle := func() { page.MustEval(`()=>new Promise(r=>setTimeout(r,300))`) }
	rightClickAt := func(x, y float64) {
		page.Mouse.MustMoveTo(x, y)
		page.Mouse.MustClick(proto.InputMouseButtonRight)
		settle()
	}
	point := page.MustEval(script(`
        editor.exec({type:'paste',text:'Alpha beta gamma delta epsilon'});await wait(200);
        caret(0,10);window.contextMenu=null;
        const b=run('gamma').getBoundingClientRect();
        return {x:b.left+b.width*0.8,y:b.top+b.height/2};
    `))
	x, y := point.Get("x").Num(), point.Get("y").Num()
	rightClickAt(x, y)
	page.MustEval(script(`
        if(editor.query({type:'selectedText'})!=='Alpha beta')throw Error('a right-click dropped the selection: '+JSON.stringify(editor.query({type:'selectedText'})));
        if(!window.contextMenu)throw Error('no menu opened on a right-click');
        const got=ids(window.contextMenu.items).join();
        if(got!=='cut,copy,paste,pastePlain,link,comment,assist')throw Error('menu for a text selection: '+got);
        if(entry('copy').disabled||entry('cut').disabled)throw Error('copy and cut must be enabled for a selection');
        caret(0);window.contextMenu=null;
    `))
	rightClickAt(x, y)
	page.MustEval(script(`
        const s=editor.surface.state().selection;
        if(!editor.snapshot().selectionCollapsed||s.anchor.offset===0)throw Error('a right-click with a plain caret must move it: '+JSON.stringify(s));
        if(!window.contextMenu)throw Error('no menu opened for a plain caret');
        // A link under the caret can be removed from the menu.
        caret(0,5);
        if(!editor.exec({type:'insertHyperlink',href:'https://example.com'}).ok)throw Error('could not link the selection');
        caret(2);
        const event=rightClick(run('Alpha'));
        if(!event.defaultPrevented)throw Error('the browser menu was not suppressed');
        if(!ids(window.contextMenu.items).includes('unlink'))throw Error('no unlink in '+ids(window.contextMenu.items).join());
        entry('unlink').action();await wait(250);
        caret(2);
        if(editor.query({type:'hyperlinkAt'}))throw Error('the link was not removed');
        // Shift+F10 arrives without a pointer position; the menu opens inside the document.
        window.contextMenu=null;
        document.querySelector('.docx-pages').dispatchEvent(new MouseEvent('contextmenu',{bubbles:true,cancelable:true,button:-1}));
        const box=document.querySelector('[data-editor]').getBoundingClientRect(),menu=window.contextMenu;
        if(!menu||!(menu.x>=box.left&&menu.x<=box.right&&menu.y>=box.top&&menu.y<=box.bottom))throw Error('keyboard menu position: '+JSON.stringify(menu&&[menu.x,menu.y]));
        // The table bar appears with the caret in a table and acts on it.
        if(!bar.hidden)throw Error('the contextual bar shows outside any context');
        editor.exec({type:'insertTable',rows:2,cols:2});await wait(350);
        if(bar.hidden||bar.querySelector('strong').textContent!=='Table')throw Error('no table bar: '+bar.outerHTML.slice(0,200));
        tool('rowBelow').click();await wait(300);
        if(failed())throw Error('row below: '+failed());
        if(editor.getSelectedTable()?.rowCount!==3)throw Error('row below gave '+JSON.stringify(editor.getSelectedTable()));
        rightClick(document.querySelector('.docx-table-cell'));
        const table=entry('table');
        if(!table||!ids(table.items).includes('deleteTable'))throw Error('no table submenu in '+ids(window.contextMenu.items).join());
        tool('deleteTable').click();await wait(350);
        if(document.querySelector('.docx-table-fragment')||!bar.hidden)throw Error('the table or its bar stayed after deleting the table');
        // The header bar switches the header options and leads back to the document.
        editor.exec({type:'editHeaderFooter',position:'header'});await wait(350);
        if(bar.hidden||bar.querySelector('strong').textContent!=='Header')throw Error('no header bar: '+bar.outerHTML.slice(0,200));
        if(tool('headerFirstPage').getAttribute('aria-pressed')!=='false')throw Error('the first page option starts pressed');
        tool('headerFirstPage').click();await wait(300);
        if(editor.getHeaderFooterState()?.titlePage!==true||tool('headerFirstPage').getAttribute('aria-pressed')!=='true')throw Error('first page option: '+JSON.stringify(editor.getHeaderFooterState()));
        tool('backDocument').click();await wait(350);
        if(editor.snapshot().scope.kind!=='body'||!bar.hidden)throw Error('back to the document left scope '+editor.snapshot().scope.kind);
        // An image for the next step.
        const canvas=document.createElement('canvas');canvas.width=64;canvas.height=48;canvas.getContext('2d').fillRect(0,0,64,48);
        const blob=await new Promise(r=>canvas.toBlob(r,'image/png')),transfer=new DataTransfer();transfer.items.add(new File([blob],'context.png',{type:'image/png'}));
        const input=document.querySelector('[data-image]');input.files=transfer.files;input.dispatchEvent(new Event('change',{bubbles:true}));
    `))
	page.Timeout(10 * time.Second).MustWait(`()=>[...WriterApp.instances.get('test').editor.surface.session.currentPackage().partBytes.keys()].some(x=>x.includes('/media/'))`)
	settle()
	drawing := page.MustEval(`()=>{const b=document.querySelector('.docx-page .docx-drawing').getBoundingClientRect();return {x:b.left+b.width/2,y:b.top+b.height/2};}`)
	page.Mouse.MustMoveTo(drawing.Get("x").Num(), drawing.Get("y").Num())
	page.Mouse.MustClick(proto.InputMouseButtonLeft)
	settle()
	page.MustEval(script(`
        if(!editor.snapshot().image)throw Error('a click did not select the image');
        if(bar.hidden||bar.querySelector('strong').textContent!=='Image')throw Error('no image bar: '+bar.outerHTML.slice(0,200));
        tool('rotateRight').click();await wait(300);
        if(editor.snapshot().image?.rotationDegrees!==90)throw Error('rotate right gave '+JSON.stringify(editor.snapshot().image?.rotationDegrees));
        caret(1);await wait(250);
        if(editor.snapshot().image||!bar.hidden)throw Error('the image bar stayed after the caret left the image');
        rightClick(document.querySelector('.docx-page .docx-drawing'));
        if(!ids(window.contextMenu.items).includes('deleteDrawing'))throw Error('no image deletion in '+ids(window.contextMenu.items).join());
        entry('deleteDrawing').action();await wait(400);
        if(failed())throw Error('delete image: '+failed());
        if(document.querySelector('.docx-page .docx-drawing'))throw Error('the image was not deleted');
        rewind();
        if(body().trim())throw Error('the document was not left blank: '+JSON.stringify(body()));
        if(JSON.stringify(editor.snapshot().pageSetup)!==paper)throw Error('the page setup was not restored: '+JSON.stringify(editor.snapshot().pageSetup));
    `))
}
```

During implementation the last part of this helper was split: the image is left with a real click into the text, because a programmatic caret keeps the engine's image selection after a rotation; see `ui/desktop_writer_context_test.go`.

In `ui/desktop_writer_browser_test.go`:

1. In `TestDesktopWriterAppBrowser` add `verifyWriterContext(t, page)` right after `verifyWriterMenus(t, page)`.
2. The fixture of that test builds `window.writerContext`. Give it a context menu that records what it was asked to show. Replace, in the fixture used by `TestDesktopWriterAppBrowser` (the first one in the file),
```js
setWindowMenus:(id,menus)=>window.menus=menus,
```
with
```js
setWindowMenus:(id,menus)=>window.menus=menus,showContextMenu:(x,y,items)=>{window.contextMenu={x,y,items};},
```
If the same text occurs in the second fixture, leave that one alone.

- [ ] **Step 2: Load the module**

Add `'/js/desktop/apps/writer-context.js'` to the `'writer'` entry of `ui/js/desktop/core/module-loader.js` right after `writer-menus.js`, the matching entry to `ui/desktop_office_assets_test.go`, and `<script src="/js/desktop/apps/writer-context.js"></script>` after the `writer-menus.js` tag in the three fixtures (two in `ui/desktop_writer_browser_test.go`, one in `ui/desktop_writer_reload_browser_test.go`). Find every list with a text search for `writer-menus.js` under `ui/` (excluding `bundles/`). Run `npm run build:ui`; stage a bundle only if `git status` shows it changed.

- [ ] **Step 3: The bar element and the wiring in `writer.js`**

1. In the markup, replace
```js
<main class="writer-canvas"><div class="writer-ruler" aria-hidden="true">
```
with
```js
<main class="writer-canvas"><div class="writer-contextbar" data-contextbar role="toolbar" hidden></div><div class="writer-ruler" aria-hidden="true">
```

2. Create the module. The variable is not called `context`, because that is the name of a parameter of `render`. Replace
```js
        const menus = window.WriterMenus.create(state);
```
with
```js
        const menus = window.WriterMenus.create(state);
        const contextBar = window.WriterContext.create(state);
```

3. In `refresh()`, replace
```js
            toolbar.refresh();menus.sync();
```
with
```js
            toolbar.refresh();contextBar.refresh();menus.sync();
```

4. A press on the bar must not drop the document selection. Replace
```js
if(event.target.closest('.writer-toolbar-wrap,.writer-right,.writer-views'))pinSelection();
```
with
```js
if(event.target.closest('.writer-toolbar-wrap,.writer-contextbar,.writer-right,.writer-views'))pinSelection();
```

5. In `cleanup()`, replace
```js
toolbar.dispose();panels.dispose();
```
with
```js
toolbar.dispose();contextBar.dispose();panels.dispose();
```

- [ ] **Step 4: Header options and the comment action**

In `activeAction` in `writer.js`, replace
```js
            if(id==='tracking')return commands.tracking();
```
with
```js
            if(id==='tracking')return commands.tracking();
            if(id==='headerFirstPage')return !!editor?.getHeaderFooterState()?.titlePage;
            if(id==='headerOddEven')return !!editor?.getHeaderFooterState()?.evenAndOddHeaders;
```

In `act`, replace
```js
                    case 'tracking':run({type:'setEditingMode',mode:commands.tracking()?'editing':'suggesting'});break;
```
with
```js
                    case 'tracking':run({type:'setEditingMode',mode:commands.tracking()?'editing':'suggesting'});break;
                    case 'headerFirstPage':case 'headerOddEven':{
                        // The options are readable only while a header or footer is open; the registry scopes both actions to it.
                        const option=action==='headerFirstPage'?'titlePage':'evenAndOddHeaders';
                        run({type:'setHeaderFooterOptions',[option]:!editor?.getHeaderFooterState()?.[option]});break;
                    }
                    case 'comment':await dialogs.comment();break;
```

In `ui/js/desktop/apps/writer-dialogs.js`, replace
```js
        return {table, link, goToPage, wordCount, shortcuts};
```
with
```js
        // A comment belongs to the selection; the engine says so when there is none.
        async function comment() {
            const editor = app.editor;
            if (!editor) return;
            const text = await app.prompt('comment');
            if (!text?.trim() || app.editor !== editor) return;
            const result = editor.addComment(text, app.author);
            if (!result.ok) app.notice(result.reason || tr('select_text'), true);
        }
        return {table, link, goToPage, wordCount, shortcuts, comment};
```

In `ui/js/desktop/apps/writer-panels.js`, replace
```js
                case 'comment_add':{const text=await app.prompt('comment');if(text?.trim()){const result=editor.addComment(text,app.author);if(!result.ok)app.notice(result.reason || tr('select_text'),true);}break;}
```
with
```js
                case 'comment_add':await app.act('comment');break;
```

- [ ] **Step 5: CSS**

In `ui/css/desktop-app-writer.css`, after the line that starts with `.writer-ruler{`, add:

```css
.writer-contextbar{position:absolute;z-index:6;top:0;left:0;right:0;display:flex;align-items:center;gap:4px;min-height:34px;padding:2px 14px;overflow-x:auto;scrollbar-width:none;background:var(--writer-bar);border-bottom:1px solid var(--writer-border);box-shadow:var(--vd-mat-bar-edge,0 0 #0000),0 6px 14px #0003}
.writer-contextbar strong{flex-shrink:0;margin-right:6px;font-size:11px;letter-spacing:.06em;text-transform:uppercase;color:var(--writer-muted);white-space:nowrap}
.writer-contextbar button{flex-shrink:0;min-width:28px;height:28px;display:grid;place-items:center;padding:3px 5px;white-space:nowrap}
.writer-contextbar .writer-context-text{padding-inline:9px}
.writer-contextbar .writer-context-divider{flex-shrink:0;width:1px;height:18px;margin:0 4px;background:var(--writer-border)}
.writer-contextbar .writer-glyph{display:block;pointer-events:none}
```

and replace
```css
.writer-focus .writer-ruler{display:none!important}
```
with
```css
.writer-focus .writer-ruler,.writer-focus .writer-contextbar{display:none!important}
```

The bar is positioned over the ruler inside `.writer-canvas` (which is `position:relative`), so nothing below it moves. It is taller than the ruler by about twelve pixels and covers that much of the page's top margin while the document is scrolled to the very top.

- [ ] **Step 6: Run the tests**

Run: `node --check` for `writer.js`, `writer-dialogs.js`, `writer-panels.js`; `npm run check:ui`; `go test ./ui -run "(?i)(windowmenu|contextmenu|materials|office|writer|virtualdesktop|lifecycle|i18n)" -count=1`
Expected: all pass (the Go run compiles the new helper). `npm run test:writer` still fails only in `test-writer-menus.mjs`, as after Task 1.

Controller: run the browser tests. Expected: `--- PASS: TestDesktopWriterAppBrowser`. `verifyWriterContext` is the first runtime proof of the items listed under "From the type definitions, not probed"; report its message verbatim if it fails. Then look at `reports/autor/default-dark-format.png`: it is taken with a text selection, so no bar is visible there; that is correct.

- [ ] **Step 7: Commit**

```bash
git add ui/js/desktop/apps/writer.js ui/js/desktop/apps/writer-dialogs.js ui/js/desktop/apps/writer-panels.js ui/css/desktop-app-writer.css ui/js/desktop/core/module-loader.js ui/desktop_office_assets_test.go ui/desktop_writer_browser_test.go ui/desktop_writer_reload_browser_test.go ui/desktop_writer_context_test.go
git diff --cached --check
git commit -m "feat(writer): right-click menu and a contextual bar for tables, images and headers"
```

---

### Task 4: The same commands in the window menus

**Files:**
- Modify: `ui/js/desktop/apps/writer-menus.js`, `ui/js/desktop/apps/writer.js`, `ui/desktop_writer_menus_test.go`

- [ ] **Step 1: List the commands**

In `build` in `ui/js/desktop/apps/writer-menus.js`:

Replace
```js
item('table'),item('image'),item('qr'),item('link'),SEP,
```
with
```js
item('table'),item('image'),item('qr'),item('link'),item('comment'),SEP,
```

Replace
```js
item('header'),item('footer'),item('pageNumber'),item('backDocument')]},
```
with
```js
item('header'),item('footer'),item('pageNumber'),item('headerFirstPage'),item('headerOddEven'),item('backDocument')]},
```

Replace
```js
                {id:'spacing',label:tr('line_spacing'),items:[item('spacing100'),item('spacing115'),item('spacing150'),item('spacing200')]},SEP,
```
with
```js
                {id:'spacing',label:tr('line_spacing'),items:[item('spacing100'),item('spacing115'),item('spacing150'),item('spacing200')]},
                {id:'image',label:tr('image'),items:[item('rotateLeft'),item('rotateRight'),SEP,item('deleteImage')]},SEP,
```

Replace
```js
item('deleteRow'),item('deleteColumn'),item('deleteTable'),SEP,item('mergeCells'),item('splitCell')]},
```
with
```js
item('deleteRow'),item('deleteColumn'),item('deleteTable'),SEP,item('mergeCells'),item('splitCell'),SEP,item('borders'),item('noBorders')]},
```

Run: `npm run test:writer`
Expected: all five test lines pass again, the last two `Writer menus: model, coverage and rebuild rule pass` and `Writer context: bar kinds, bar contents and right-click sections pass`.

- [ ] **Step 2: The menus follow the header options**

In `menuSignature()` in `writer.js`, replace
```js
                ...(s?['mergeCells','splitCell','pasteFormat','previousChange','nextChange','backDocument'].map(id=>commands.state(id).enabled):[])]);
```
with
```js
                ...(s?['mergeCells','splitCell','pasteFormat','previousChange','nextChange','backDocument'].map(id=>commands.state(id).enabled):[]),
                ...(s?.scope?.kind==='headerFooter'?[activeAction('headerFirstPage'),activeAction('headerOddEven')]:[])]);
```

- [ ] **Step 3: Browser check**

In `verifyWriterMenus` in `ui/desktop_writer_menus_test.go`, replace
```
'tracking','nextChange','shortcuts','assist','gog-import'])
```
with
```
'tracking','nextChange','shortcuts','assist','gog-import','comment','rotateLeft','deleteImage','borders','headerFirstPage'])
```
and replace
```
        if(!item('rowAbove').disabled)throw Error('row commands need a table');
```
with
```
        if(!item('rowAbove').disabled)throw Error('row commands need a table');
        if(!item('headerFirstPage').disabled||!item('rotateLeft').disabled)throw Error('header and image commands need their context');
```

- [ ] **Step 4: Run the tests**

Run: `npm run test:writer`, `npm run check:ui`, `go test ./ui -run "(?i)(windowmenu|contextmenu|materials|office|writer|virtualdesktop|lifecycle|i18n)" -count=1`
Expected: all pass.

Controller: run the browser tests. Expected: `--- PASS: TestDesktopWriterAppBrowser`. If the new assertion fails for `rotateLeft` (the engine may accept `transformImage` without a selected image), report it; the fix is then to gate the three image commands in the registry on `snapshot().image`, not to drop the assertion.

- [ ] **Step 5: Commit**

```bash
git add ui/js/desktop/apps/writer-menus.js ui/js/desktop/apps/writer.js ui/desktop_writer_menus_test.go
git diff --cached --check
git commit -m "feat(writer): window menus list the context commands"
```

---

### Task 5: Documentation and final verification

**Files:**
- Modify: `ui/js/desktop/apps/AGENTS.md`, `documentation/autor.md`

- [ ] **Step 1: `ui/js/desktop/apps/AGENTS.md`**

In the bullet that starts with "`writer.js` owns Autor's native DOCX document", after the sentence part about `writer-dialogs.js` (it ends with "all through `shared/form-dialog.js`;"), insert:

"`writer-context.js` owns the two context surfaces: the bar over the ruler (`kind(snapshot)` picks image, header/footer or table; `bars` lists its commands) and the right-click menu (`sections(target)`), both built from the registry. The engine moves its caret on a right-click, so the module keeps a right-button press from the engine while there is a selection or a selected image, and lets a plain caret follow the click. Items only the app can build (accept or reject the change under the pointer, unlink, delete an unselected image, update a table of contents) are the `@` entries; every context-menu item names an icon the shell has, because the shell draws an unknown key as text;"

In the same bullet, extend the sentence about a new command: after "a toolbar control is its id in a `GROUPS` entry;" add "a contextual-bar control is its id in `bars` in `writer-context.js`; `scope` limits an app action to one editing scope;".

If a quoted phrase is worded differently in the file, edit the sentence that says the same thing and keep the rest of the bullet.

- [ ] **Step 2: `documentation/autor.md`**

In the section "Writing", after the paragraph about the window menus and the zoom button (it ends with "Ctrl with the mouse wheel zooms as well."), add:

```
A right-click in the document opens a menu for what was clicked: cut, copy and
paste, the link under the caret (change or remove it), a comment on the
selection, the table commands inside a table, rotate and delete for an image,
accept or reject for a tracked change under the pointer, and the AI
assistant. A selection stays as it is when you right-click; without a
selection the caret moves to the click first. Shift+F10 opens the same menu
from the keyboard.
While the caret is in a table, an image is selected, or a header or footer is
open, a bar with the matching commands covers the ruler: rows, columns,
merge, split and borders for a table; rotate, delete and the Format panel for
an image; page numbers, "different first page", "different odd and even
pages" and the way back to the document for a header or footer. The document
does not move when the bar appears.
```

- [ ] **Step 3: Full verification**

Run, in this order:

1. `npm run test:writer`: five `... pass` lines after the two session lines.
2. `npm run check:ui`: ends with `Ocean chat sea life renderer verified`.
3. `go test ./ui -count=1` (no browser): the only failure allowed is `TestDesktopCodeStudioRunUsesTerminalSession`.
4. Controller: delete `reports\autor`, then `AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run "TestDesktop(Writer|QR)" -count=1 -timeout 30m -v`: every `TestDesktopWriter*` passes.
5. `git status --short`: only intended files; `disposable/_writer-wave1b/` is gone.
6. Line counts: every `ui/js/desktop/apps/writer*.js` is below 1100 lines.
7. DOX pass: the two guides above are the owners of what changed; `ui/AGENTS.md` and the root guide need no change (no new contract above the app level). Say so in the report.

- [ ] **Step 4: Commit**

```bash
git add ui/js/desktop/apps/AGENTS.md documentation/autor.md
git diff --cached --check
git commit -m "docs(writer): right-click menu and contextual bar"
```

---

## What changes for the user

- A right-click in the document opens Writer's own menu instead of nothing. A selection survives the right-click.
- With the caret in a table, on a selected image, or in a header or footer, a bar with the matching commands covers the ruler. The document does not jump.
- New commands, also in the window menus: table borders on and off, rotate and delete an image, "different first page" and "different odd and even pages" for headers and footers, add a comment (Insert menu).
- The Review panel's comment button and the new menu entries share one action.

## Out of scope for this plan

- Start state and recent documents (part 1c).
- Image wrap, size and alt text, and cell fill, in the bar; they stay in the Format panel. The engine refuses a wrap change for images Writer inserted.
- A bar for footnotes and text boxes.
- Telling whether a right-click landed inside or outside a selection; the engine does not expose where the selection is painted.
