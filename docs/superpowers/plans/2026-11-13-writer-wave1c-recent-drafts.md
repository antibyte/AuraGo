# Writer Wave 1c — recent documents and unsaved drafts: implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** The File menu lists the documents last opened in Writer and opens one with a click, and a dialog lists the drafts the browser still keeps, so one can be restored or deleted.

**Architecture:** No new module. The desktop shell already keeps a recent-files list; it hands Writer three functions to read, record and forget entries. `writer.js` records what the shell cannot see and builds the submenu from a pure function in `writer-menus.js`. The draft store in `writer-session.js` learns to list its drafts without their bytes; the dialog in `writer-dialogs.js` shows them through the shared form dialog, which gains a list field.

**Tech stack:** Vanilla JS classic scripts (no bundler for the app files; `menus-and-routing.js` is part of `main.bundle.js`), `@docx-editor.dev/core` 2.29.0 behind `lib`, IndexedDB, node `vm` tests, Go + rod browser tests.

Roadmap: `docs/superpowers/specs/2026-11-12-writer-office-roadmap-design.md`, section "Wave 1 — Interaction". This is part 1c of three; 1a and 1b are on `main`.

---

## Read this first

**Decided by the user (2026-11-13):**

- A window opened without a file keeps starting in a blank document. There is no start page. The templates stay in the Insert panel.
- "Recent documents" is a submenu of File.
- The drafts dialog is part of 1c.

**Scope.** In: File > Recent documents; recording of files opened, renamed or saved under a new name inside Writer; File > Unsaved drafts with restore and delete. Out: a start page; templates in a menu; "clear list"; pinning; a larger shell list (it holds twelve entries across all apps); changing the question Writer asks when it opens a document that has a draft.

**How things work today** (read from the code on 2026-11-13):

- The shell keeps recent files in localStorage (`ui/js/desktop/core/session-runtime.js`): `readRecentFiles()` returns `[{path, name, appId, kind, openedAt}]`, newest first, at most twelve; `recordRecentFile(path, appId)` puts a path on top; `removeRecentFilesAtPath(path)` drops it. The shell records a file when a window opens with a path. Writer gets none of these functions, so files opened through its own Open dialog, Save as and Rename never reach the list. Notes already receives `recordRecentFile` (`menus-and-routing.js`, the `NotesApp.render` call).
- A window without a file loads the engine's blank document under `Documents/untitled-<8 hex>.docx`. The load itself sets the page to A4, which is a document change. Rule for this plan: a new document enters the recent list only when a save finds text in it, so blank launches never fill the list.
- `load(target, template)` in `writer.js` fetches the file, mounts the editor, and then asks "Recover draft?" when IndexedDB holds a draft under `location.origin + ':' + path`. A draft is `{key, id, bytes, revision, etag, updated, path}`. It is written before every save and deleted after a successful one, so a draft survives only when a save failed, the browser closed first, or the user declined the question.
- `WriterSession.draft(operation, key, value)` supports `get`, `put`, `delete` and `deleteIf` (deletes only when the stored `id` matches). Nothing lists drafts.
- `AuraFormDialog.open` (`ui/js/desktop/shared/form-dialog.js`) resolves `{extra: true}` for the extra button without the field values, and its `select` is always a drop-down.
- Window menus are rebuilt only when `menuSignature()` changes (`writer-menus.js`), because the shell reads `disabled` when menus are set and setting them closes an open menu.

**Not verified, the browser check in Task 4 is the proof.** If it fails there, stop and report the thrown message:

- A blank new document is not recorded even after its first automatic save (the rule above).
- `fetch` of a missing `.docx` answers 404 in the fixture and in the real handler alike, so a draft of a vanished document restores as a new document.
- A `<select size="n">` with the shell's `vd-modal-input` class shows as a list.

**Repo rules:**

- Read the `AGENTS.md` chain for every path you touch before editing (root, `ui/AGENTS.md`, `ui/js/desktop/apps/AGENTS.md`, `scripts/AGENTS.md`); close out with a DOX pass (Task 5).
- Every user-visible string exists in all 16 files under `ui/lang/desktop/`. `scripts/test-writer-commands.mjs` scans every registry label and every literal `tr('key')` in `ui/js/desktop/apps/writer*.js`; write `tr('key')` with a literal key.
- Every file under `ui/js/desktop` stays below 1100 lines. No Internet assets, no new dependencies.
- `ui/desktop_office_app_markers_test.go` pins these strings in `writer.js`: `window.WriterApp={render,dispose,instances}`, `queue?.dispose()`, `dispose(windowId);`, `editor?.destroy()`, `/api/desktop/office/document`; and in `writer-menus.js`: `item('new')`, `item('saveAs')`, `id:'agent'`, `labelKey:'desktop.menu_agent'`, `item('assist')`, `setWindowMenus`. `ui/desktop_desktop_feeling_test.go` pins `function recordRecentFile(` in `session-runtime.js`; that file is not edited.
- A command is one entry in `DEFINITIONS` in `writer-commands.js`; `scripts/test-writer-menus.mjs` fails for a command no window menu lists.
- `writer-session.js` is shared: Sheets, Notes, Synth Studio and Layerling use `OfficeSession`. Only add to it.
- Temporary scripts go into `disposable/_writer-wave1c/` and are deleted before the commit. Before each commit: `git diff --cached --check`, and look at `git diff --cached`.
- GitNexus: the index is hundreds of commits behind and cannot index worktrees. Controller's impact analysis for `writer.js`: UNKNOWN, callers confirmed by text search (`window.WriterApp` is used by the shell loader and the Go tests; the state object by the Writer modules). End each commit body with: `GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.`

**Environment (Windows):**

- Work in the worktree `.claude/worktrees/writer-wave1c` with a real copy of `node_modules` (`robocopy <main>\node_modules <worktree>\node_modules /E /MT:16 /NFL /NDL /NJH /NP`).
- In a worktree session git commands must be plain and single (no `&&`, no `cd`).
- Browser tests: `AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run "TestDesktop(Writer|QR)" -count=1 -timeout 30m -v`, from PowerShell with the sandbox off. Delete `reports\autor` first. Implementer subagents cannot start Chrome; the controller runs these.
- Static tests: `npm run test:writer`, `npm run check:ui`, `go test ./ui -run "(?i)(windowmenu|contextmenu|materials|office|writer|virtualdesktop|lifecycle|i18n|feeling)" -count=1`.
- `TestDesktopCodeStudioRunUsesTerminalSession` fails on `main` for an unrelated reason. Ignore it.

## File structure

| File | Change | Responsibility |
| --- | --- | --- |
| `ui/js/desktop/apps/writer-session.js` | modify | `drafts(namespace)`: every draft without its bytes |
| `ui/js/desktop/shared/form-dialog.js` | modify | A `select` with `size` is a list; the extra button answers with the values |
| `ui/css/desktop-form-dialog.css` | modify | The list field |
| `ui/js/desktop/apps/writer-menus.js` | modify | `recent(entries, current)`; the submenu and the drafts item in File |
| `ui/js/desktop/apps/writer-commands.js` | modify | The `drafts` command |
| `ui/js/desktop/apps/writer-dialogs.js` | modify | `drafts()` |
| `ui/js/desktop/apps/writer.js` | modify | Recording, the submenu items, restoring a draft, menu signature |
| `ui/js/desktop/core/menus-and-routing.js`, `ui/js/desktop/bundles/main.bundle.js` | modify | Writer's context gets the three recent-list functions; rebuilt bundle |
| `ui/lang/desktop/*.json` | modify | 7 new keys in 16 languages |
| `scripts/test-writer-session.mjs`, `scripts/test-writer-menus.mjs` | modify | Draft list, recent list, menu model |
| `ui/desktop_writer_recent_test.go` | create | Browser check |
| `ui/desktop_writer_browser_test.go` | modify | The file-ops fixture keeps a recent list and the menus; the call |
| `ui/js/desktop/apps/AGENTS.md`, `documentation/autor.md`, the roadmap spec | modify | Documentation |

---

### Task 1: Draft list and list field

**Files:**
- Modify: `ui/js/desktop/apps/writer-session.js`, `ui/js/desktop/shared/form-dialog.js`, `ui/css/desktop-form-dialog.css`
- Modify: `scripts/test-writer-session.mjs`

- [ ] **Step 1: Write the failing test**

Append this block to the end of `scripts/test-writer-session.mjs`:

```js
// The draft list leaves the document bytes out and puts the newest first.
const stored = [{key: 'o:Documents/a.docx', id: '1', path: 'Documents/a.docx', updated: 10, bytes: new Uint8Array(3)},
    {key: 'o:Documents/b.docx', id: '2', path: 'Documents/b.docx', updated: 20, bytes: new Uint8Array(5)}];
context.indexedDB = {open() {
    const request = {};
    const store = {openCursor() {
        const walk = {};
        let index = 0;
        const step = () => {
            walk.result = index < stored.length ? {value: stored[index], continue() { index++; Promise.resolve().then(step); }} : null;
            walk.onsuccess();
        };
        Promise.resolve().then(step);
        return walk;
    }};
    Promise.resolve().then(() => { request.result = {transaction: () => ({objectStore: () => store})}; request.onsuccess(); });
    return request;
}};
const listed = await context.window.WriterSession.drafts();
assert.equal(listed.map(entry => entry.path).join(), 'Documents/b.docx,Documents/a.docx');
assert.equal(listed[0].size, 5);
assert.equal(listed[0].id, '2');
assert.equal('bytes' in listed[0], false);
console.log('Writer drafts: the list leaves the bytes out and is newest first');
```

- [ ] **Step 2: Run it to see it fail**

Run: `node scripts/test-writer-session.mjs`
Expected: the two existing lines pass, then `TypeError: context.window.WriterSession.drafts is not a function`.

- [ ] **Step 3: List drafts in `writer-session.js`**

Replace

```js
    window.OfficeSession = { create, draft };
```
with
```js
    // Every draft of a namespace, newest first, without its document bytes:
    // {key, id, path, updated, revision, etag, size}.
    async function drafts(namespace = 'writer') {
        const db = await openDrafts(namespace);
        return new Promise((resolve, reject) => {
            const found = [], request = db.transaction('documents', 'readonly').objectStore('documents').openCursor();
            request.onsuccess = () => {
                const cursor = request.result;
                if (!cursor) { resolve(found.sort((a, b) => (b.updated || 0) - (a.updated || 0))); return; }
                const {bytes, ...rest} = cursor.value;
                found.push({...rest, size: bytes?.byteLength || 0});
                cursor.continue();
            };
            request.onerror = () => reject(request.error);
        });
    }
    window.OfficeSession = { create, draft, drafts };
```

- [ ] **Step 4: Run the test to see it pass**

Run: `node scripts/test-writer-session.mjs`
Expected: three lines, the last one `Writer drafts: the list leaves the bytes out and is newest first`.

- [ ] **Step 5: A list field and an extra button that answers with the values**

In `ui/js/desktop/shared/form-dialog.js` replace

```js
    // min, max, step, maxlength, placeholder, options: [{value, label}]}.
```
with
```js
    // min, max, step, maxlength, placeholder, options: [{value, label}], size}.
    //   select  a drop-down; with size it is a list that shows that many options at once.
```

Replace

```js
        if (field.type === 'select') return '<label>' + label + '<select class="vd-modal-input" ' + name + '>' + (field.options || []).map(option =>
```
with
```js
        if (field.type === 'select') return '<label>' + label + '<select class="vd-modal-input" ' + name + (field.size ? ' size="' + Number(field.size) + '"' : '') + '>' + (field.options || []).map(option =>
```

Replace

```js
    // to {extra: true} when the extra button was pressed, or to null when the dialog is cancelled.
```
with
```js
    // to {extra: true} plus the unvalidated values when the extra button was pressed, or to null when
    // the dialog is cancelled.
```

Replace

```js
            node.querySelector('[data-form-extra]')?.addEventListener('click', () => finish({extra: true}));
```
with
```js
            const collect = () => Object.fromEntries(fields.filter(field => !silent(field)).map(field => [field.id, read(field, input(field.id))]));
            node.querySelector('[data-form-extra]')?.addEventListener('click', () => finish(Object.assign(collect(), {extra: true})));
```

Replace

```js
                const values = {};
                for (const field of fields) if (!silent(field)) values[field.id] = read(field, input(field.id));
                const message = options.validate?.(values) || '';
```
with
```js
                const values = collect();
                const message = options.validate?.(values) || '';
```

In `ui/css/desktop-form-dialog.css` replace

```css
.vd-form-dialog .vd-modal-input{margin:0}
```
with
```css
.vd-form-dialog .vd-modal-input{margin:0}
.vd-form-dialog select[size].vd-modal-input{height:auto;min-height:0;padding:4px;overflow-y:auto}
.vd-form-dialog select[size] option{padding:5px 8px;border-radius:4px}
```

- [ ] **Step 6: Check and commit**

Run: `node --check ui/js/desktop/shared/form-dialog.js`, `node --check ui/js/desktop/apps/writer-session.js`, `npm run test:writer`.
Expected: no output from the two checks; every line of `test:writer` ends in `pass` or names its subject, none fails.

```
git add ui/js/desktop/apps/writer-session.js ui/js/desktop/shared/form-dialog.js ui/css/desktop-form-dialog.css scripts/test-writer-session.mjs
git diff --cached --check
git commit
```

Message:

```
feat(writer): list kept drafts; the form dialog gets a list field

OfficeSession.drafts() walks the draft store and leaves the document
bytes out. A select with a size is a list, and the extra button
answers with the field values, so a dialog can act on the picked row.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: Claude Fable 5.2 <noreply@anthropic.com>
```

---

### Task 2: Language keys and the recent list

**Files:**
- Modify: `ui/lang/desktop/*.json` (script)
- Modify: `ui/js/desktop/apps/writer-menus.js`, `scripts/test-writer-menus.mjs`

- [ ] **Step 1: Add the language keys**

Create `disposable/_writer-wave1c/i18n.mjs`:

```js
// Adds the Wave 1c keys to the sixteen desktop language files, each at its sorted place in the desktop.writer_ block.
// Usage: node disposable/_writer-wave1c/i18n.mjs <repo root>
import {readFileSync, writeFileSync} from 'node:fs';
import path from 'node:path';

const root = process.argv[2] || process.cwd();
const LANGS = ['cs', 'da', 'de', 'el', 'en', 'es', 'fr', 'hi', 'it', 'ja', 'nl', 'no', 'pl', 'pt', 'sv', 'zh'];
const KEYS = {
    recent: ['Naposledy otevřené', 'Senest åbnede', 'Zuletzt geöffnet', 'Πρόσφατα έγγραφα', 'Recent documents', 'Documentos recientes', 'Documents récents', 'हाल के दस्तावेज़', 'Documenti recenti', '最近使ったドキュメント', 'Recente documenten', 'Nylige dokumenter', 'Ostatnie dokumenty', 'Documentos recentes', 'Senaste dokument', '最近的文档'],
    recent_none: ['Žádné naposledy otevřené dokumenty', 'Ingen senest åbnede dokumenter', 'Keine zuletzt geöffneten Dokumente', 'Δεν υπάρχουν πρόσφατα έγγραφα', 'No recent documents', 'No hay documentos recientes', 'Aucun document récent', 'कोई हाल का दस्तावेज़ नहीं', 'Nessun documento recente', '最近使ったドキュメントはありません', 'Geen recente documenten', 'Ingen nylige dokumenter', 'Brak ostatnich dokumentów', 'Nenhum documento recente', 'Inga senaste dokument', '没有最近的文档'],
    drafts: ['Neuložené koncepty', 'Ikke-gemte kladder', 'Ungespeicherte Entwürfe', 'Μη αποθηκευμένα πρόχειρα', 'Unsaved drafts', 'Borradores sin guardar', 'Brouillons non enregistrés', 'बिना सहेजे गए ड्राफ़्ट', 'Bozze non salvate', '未保存の下書き', 'Niet-opgeslagen concepten', 'Ulagrede utkast', 'Niezapisane wersje robocze', 'Rascunhos não guardados', 'Osparade utkast', '未保存的草稿'],
    drafts_none: ['V tomto prohlížeči nejsou žádné neuložené koncepty.', 'Denne browser har ingen ikke-gemte kladder.', 'In diesem Browser liegen keine ungespeicherten Entwürfe.', 'Αυτό το πρόγραμμα περιήγησης δεν έχει μη αποθηκευμένα πρόχειρα.', 'This browser holds no unsaved drafts.', 'Este navegador no tiene borradores sin guardar.', 'Ce navigateur ne contient aucun brouillon non enregistré.', 'इस ब्राउज़र में कोई बिना सहेजा गया ड्राफ़्ट नहीं है।', 'Questo browser non contiene bozze non salvate.', 'このブラウザーに未保存の下書きはありません。', 'Deze browser bevat geen niet-opgeslagen concepten.', 'Denne nettleseren har ingen ulagrede utkast.', 'Ta przeglądarka nie przechowuje niezapisanych wersji roboczych.', 'Este navegador não tem rascunhos não guardados.', 'Den här webbläsaren har inga osparade utkast.', '此浏览器中没有未保存的草稿。'],
    draft_pick: ['Koncept', 'Kladde', 'Entwurf', 'Πρόχειρο', 'Draft', 'Borrador', 'Brouillon', 'ड्राफ़्ट', 'Bozza', '下書き', 'Concept', 'Utkast', 'Wersja robocza', 'Rascunho', 'Utkast', '草稿'],
    draft_restore: ['Obnovit', 'Gendan', 'Wiederherstellen', 'Επαναφορά', 'Restore', 'Restaurar', 'Restaurer', 'पुनर्स्थापित करें', 'Ripristina', '復元', 'Herstellen', 'Gjenopprett', 'Przywróć', 'Restaurar', 'Återställ', '恢复'],
    draft_deleted: ['Koncept byl odstraněn.', 'Kladden er slettet.', 'Entwurf gelöscht.', 'Το πρόχειρο διαγράφηκε.', 'Draft deleted.', 'Borrador eliminado.', 'Brouillon supprimé.', 'ड्राफ़्ट हटा दिया गया।', 'Bozza eliminata.', '下書きを削除しました。', 'Concept verwijderd.', 'Utkastet er slettet.', 'Wersja robocza została usunięta.', 'Rascunho eliminado.', 'Utkastet har tagits bort.', '草稿已删除。']
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

Run from the repo root: `node disposable/_writer-wave1c/i18n.mjs .`

Expected: sixteen lines, each `<lang>: 7 keys added`; `git diff --stat -- ui/lang/desktop` shows 16 files, 7 insertions each, no deletions. Then delete `disposable/_writer-wave1c/`.

- [ ] **Step 2: Write the failing test for the recent list**

In `scripts/test-writer-menus.mjs` replace

```js
console.log('Writer menus: model, coverage and rebuild rule pass');
```
with
```js
// The recent list: Writer's own files, newest first, without the open one; twins show their folder.
const recent = WriterMenus.recent([
    {path: 'Documents/a.docx', appId: 'writer'}, {path: 'Pictures/x.png', appId: 'gallery'}, {path: 'Documents/open.docx', appId: 'writer'},
    {path: 'Letters/a.docx', appId: 'writer'}, {path: 'b.docx', appId: 'writer'}], 'Documents/open.docx');
assert.equal(recent.map(entry => entry.path).join(), 'Documents/a.docx,Letters/a.docx,b.docx');
assert.equal(recent.map(entry => entry.label).join('|'), 'a.docx · Documents|a.docx · Letters|b.docx');
assert.equal(WriterMenus.recent(Array.from({length: 20}, (_, index) => ({path: 'f' + index + '.docx', appId: 'writer'})), '').length, 8);
assert.equal(WriterMenus.recent(undefined, '').length, 0);
console.log('Writer menus: model, coverage, rebuild rule and recent list pass');
```

Run: `node scripts/test-writer-menus.mjs`
Expected: `TypeError: WriterMenus.recent is not a function`.

- [ ] **Step 3: The pure function**

In `ui/js/desktop/apps/writer-menus.js` replace

```js
    window.WriterMenus = {create, build};
```
with
```js
    // The documents Writer opened last, newest first, without the one that is open. entries is the
    // shell's recent list ({path, appId}); two files of one name are told apart by their folder.
    function recent(entries, current, limit = 8) {
        const list = (entries || []).filter(entry => entry && entry.appId === 'writer' && entry.path && entry.path !== current).slice(0, limit);
        const names = list.map(entry => entry.path.split('/').pop());
        return list.map((entry, index) => {
            const folder = entry.path.split('/').slice(0, -1).join('/');
            const twin = names.indexOf(names[index]) !== names.lastIndexOf(names[index]);
            return {path: entry.path, label: twin && folder ? names[index] + ' · ' + folder : names[index]};
        });
    }
    window.WriterMenus = {create, build, recent};
```

- [ ] **Step 4: Run the tests and commit**

Run: `npm run test:writer`
Expected: the menus line reads `Writer menus: model, coverage, rebuild rule and recent list pass`; the commands line still passes (the seven keys are not used yet, which is allowed).

```
git add ui/lang/desktop ui/js/desktop/apps/writer-menus.js scripts/test-writer-menus.mjs
git diff --cached --check
git commit
```

Message:

```
feat(writer): texts and the list model for recent documents

Seven keys in sixteen languages. WriterMenus.recent picks Writer's own
entries from the shell's recent list, leaves the open document out and
tells files of one name apart by their folder.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: Claude Fable 5.2 <noreply@anthropic.com>
```

---

### Task 3: Wiring: the menu, recording, and the drafts dialog

**Files:**
- Modify: `ui/js/desktop/apps/writer-commands.js`, `ui/js/desktop/apps/writer-menus.js`, `ui/js/desktop/apps/writer-dialogs.js`, `ui/js/desktop/apps/writer.js`
- Modify: `ui/js/desktop/core/menus-and-routing.js`, `ui/js/desktop/bundles/main.bundle.js` (rebuilt)
- Modify: `scripts/test-writer-menus.mjs`

- [ ] **Step 1: Extend the menu test**

In `scripts/test-writer-menus.mjs` replace

```js
const menus = WriterMenus.build(id => { used.push(id); return {id}; }, key => 'tr:' + key, {file: [{id: 'cloud'}]});
```
with
```js
const menus = WriterMenus.build(id => { used.push(id); return {id}; }, key => 'tr:' + key, {file: [{id: 'cloud'}], recent: [{id: 'recent-0'}]});
```

Replace

```js
assert.ok(flat(menus[0].items).some(item => item.id === 'cloud'), 'the app-supplied file items are placed');
```
with
```js
assert.ok(flat(menus[0].items).some(item => item.id === 'cloud'), 'the app-supplied file items are placed');
assert.equal(menus[0].items.map(item => item.id || '-').slice(0, 5).join(), 'new,open,recent,drafts,-', 'File starts with New, Open, the recent documents and the drafts');
assert.equal(menus[0].items[2].items[0].id, 'recent-0', 'the recent documents are a submenu');
```

Replace

```js
    menuSignature: () => signature, fileMenuItems: () => []
```
with
```js
    menuSignature: () => signature, fileMenuItems: () => [], recentMenuItems: () => []
```

Run: `node scripts/test-writer-menus.mjs`
Expected: an assertion failure whose message starts with `File starts with New, Open, the recent documents and the drafts`.

- [ ] **Step 2: The command and the menu model**

In `ui/js/desktop/apps/writer-commands.js` replace

```js
        {id: 'open', label: 'open', icon: 'folder-open', shortcut: 'Ctrl+O', keys: ['o']},
```
with
```js
        {id: 'open', label: 'open', icon: 'folder-open', shortcut: 'Ctrl+O', keys: ['o']},
        {id: 'drafts', label: 'drafts', write: true},
```

In `ui/js/desktop/apps/writer-menus.js` replace

```js
    // The window menus. item(id) turns a registry command into a menu item; extra carries the items
    // only writer.js can build (cloud import and export). A command no surface offers fails
    // scripts/test-writer-menus.mjs.
```
with
```js
    // The window menus. item(id) turns a registry command into a menu item; extra carries the items
    // only writer.js can build (cloud import and export in `file`, the recent documents in `recent`).
    // A command no surface offers fails scripts/test-writer-menus.mjs.
```

Replace

```js
            {id:'file',labelKey:'desktop.menu_file',items:[item('new'),item('open'),item('save'),item('saveAs'),SEP,...extra.file,SEP,
```
with
```js
            {id:'file',labelKey:'desktop.menu_file',items:[item('new'),item('open'),{id:'recent',label:tr('recent'),items:extra.recent},item('drafts'),SEP,
                item('save'),item('saveAs'),SEP,...extra.file,SEP,
```

Replace

```js
    // app: the state facade of writer.js ({ctx, tr, commands, windowId, editor, menuSignature(), fileMenuItems()}).
```
with
```js
    // app: the state facade of writer.js ({ctx, tr, commands, windowId, editor, menuSignature(), fileMenuItems(), recentMenuItems()}).
```

Replace

```js
            ctx.setWindowMenus?.(app.windowId, build(item, tr, {file: app.fileMenuItems()}));
```
with
```js
            ctx.setWindowMenus?.(app.windowId, build(item, tr, {file: app.fileMenuItems(), recent: app.recentMenuItems()}));
```

Run: `node scripts/test-writer-menus.mjs`
Expected: `Writer menus: model, coverage, rebuild rule and recent list pass`.

- [ ] **Step 3: The drafts dialog**

In `ui/js/desktop/apps/writer-dialogs.js` replace

```js
        return {table, link, goToPage, wordCount, shortcuts, comment};
```
with
```js
        const draftLabel = entry => {
            const parts = String(entry.path).split('/'), name = parts.pop();
            const when = entry.updated ? new Date(entry.updated).toLocaleString(document.documentElement.lang || 'en', {dateStyle: 'medium', timeStyle: 'short'}) : '';
            return [name, parts.join('/'), when].filter(Boolean).join(' · ');
        };
        // The drafts this browser kept because a save failed or the browser closed first. The list
        // never loads the documents themselves; one draft at a time is restored or deleted.
        async function drafts() {
            let list;
            try { list = (await window.WriterSession.drafts()).filter(entry => entry.path); }
            catch (_) { app.notice(tr('draft_unavailable'), true); return; }
            if (!list.length) { await tell(tr('drafts'), [[tr('drafts_none'), '']]); return; }
            const answer = await ask(tr('drafts'), [{id: 'draft', type: 'select', label: tr('draft_pick'), size: Math.min(8, Math.max(2, list.length)),
                value: list[0].key, options: list.map(entry => ({value: entry.key, label: draftLabel(entry)}))}],
                {submitLabel: tr('draft_restore'), extraLabel: tr('delete')});
            const picked = answer && list.find(entry => entry.key === answer.draft);
            if (!picked) return;
            if (!answer.extra) return app.restoreDraft(picked);
            // deleteIf leaves a draft alone that was written again since the list was read.
            await window.WriterSession.draft('deleteIf', picked.key, {id: picked.id}).catch(() => {});
            app.notice(tr('draft_deleted'));
            return drafts();
        }
        return {table, link, goToPage, wordCount, shortcuts, comment, drafts};
```

- [ ] **Step 4: `writer.js`**

All replacements are in `ui/js/desktop/apps/writer.js`. Each old block occurs exactly once.

The state facade. Replace

```js
            windowId,signal:()=>documentLife.signal,countWords:text=>countWords(text),menuSignature,fileMenuItems,
```
with
```js
            windowId,signal:()=>documentLife.signal,countWords:text=>countWords(text),menuSignature,fileMenuItems,recentMenuItems,restoreDraft,
```

Whether the open document is in the recent list. Replace

```js
        let noticeKind = '', retrySaveAs = null;
```
with
```js
        let noticeKind = '', retrySaveAs = null, recorded = false;
```

Recording. Replace

```js
        function draftKey(){return location.origin+':'+path;}
```
with
```js
        // The shell keeps the recent list. Writer adds what the shell cannot see: a file opened,
        // renamed or saved under a new name inside the window, and a new document once it holds text.
        function remember(){recorded=true;ctx.recordRecentFile?.(path,'writer');menus.sync();}
        function draftKey(){return location.origin+':'+path;}
```

A new document is recorded by the first save that finds text. Replace

```js
            if(target===path)etag=response.headers.get('ETag');
```
with
```js
            if(target===path) {
                etag=response.headers.get('ETag');
                if(!recorded && editor?.surface.session.bodyText().trim())remember();
            }
```

`load` takes a third argument: restore the draft without asking. Replace

```js
        async function load(target,template) {
```
with
```js
        // take: put the kept draft in without asking, and start a new document when the file is gone.
        async function load(target,template,take) {
```

Replace

```js
            draftTimer=draftDeadline=null;lastDraft=null;statsRevision=-1;rulerSignature='';
```
with
```js
            draftTimer=draftDeadline=null;lastDraft=null;statsRevision=-1;rulerSignature='';recorded=false;
```

Replace

```js
                        const response=await responseOK(await fetch(nativeURL(target),{signal:requestLife.signal,cache:'no-store'}));
                        loadedETag=response.headers.get('ETag');bytes=await response.arrayBuffer();
                    } else {
```
with
```js
                        const found=await fetch(nativeURL(target),{signal:requestLife.signal,cache:'no-store'});
                        if(take && found.status===404){await found.body?.cancel().catch(()=>{});template='blank';}
                        else{const response=await responseOK(found);loadedETag=response.headers.get('ETag');bytes=await response.arrayBuffer();}
                    } else {
```

Replace

```js
                ctx.updateWindowContext?.(windowId,{path});
                toolbar.fill();menus.sync(true);refresh();
```
with
```js
                ctx.updateWindowContext?.(windowId,{path});
                if(etag)remember();
                toolbar.fill();menus.sync(true);refresh();
```

Replace

```js
                const restore=draft?.bytes && !ctx.readonly && await ctx.confirmDialog(tr('recover'),tr('recover_detail'));
```
with
```js
                const restore=draft?.bytes && !ctx.readonly && (take || await ctx.confirmDialog(tr('recover'),tr('recover_detail')));
```

A draft of a document that does not exist is saved as a new file. Replace

```js
                    loading=true;editor.load(draft.bytes);loading=false;etag=draft.etag || null;queue.changed();notice(tr('recovered'));refresh();
```
with
```js
                    loading=true;editor.load(draft.bytes);loading=false;etag=template?null:draft.etag || null;queue.changed();notice(tr('recovered'));refresh();
```

A file that is gone leaves the list. Replace

```js
                if(error.status===404)ctx.updateWindowContext?.(windowId,{path:''});
```
with
```js
                if(error.status===404){ctx.updateWindowContext?.(windowId,{path:''});ctx.forgetRecentFile?.(target);}
```

Save as. Replace

```js
                retrySaveAs=null;notice(tr('saved'));ctx.loadBootstrap?.();
```
with
```js
                retrySaveAs=null;notice(tr('saved'));ctx.loadBootstrap?.();remember();
```

Rename. Replace

```js
                await adopted.body?.cancel().catch(()=>{});
                ctx.loadBootstrap?.();
```
with
```js
                await adopted.body?.cancel().catch(()=>{});
                ctx.loadBootstrap?.();ctx.forgetRecentFile?.(oldPath);remember();
```

The action. Replace

```js
                    case 'new':if(!ctx.readonly && await guard()){await load(newPath(),'blank');showPanel('right','insert');}break;
```
with
```js
                    case 'drafts':await dialogs.drafts();break;
                    case 'new':if(!ctx.readonly && await guard()){await load(newPath(),'blank');showPanel('right','insert');}break;
```

The submenu items and the restore. Replace

```js
        function fileMenuItems() {
```
with
```js
        // A document from the recent list opens like one picked in the Open dialog.
        async function openRecent(target) {
            try{if(await guard())await load(target);}catch(error){fail(error);}
        }
        function recentMenuItems() {
            const list=window.WriterMenus.recent(ctx.recentFiles?.(),path);
            if(!list.length)return [{id:'recent-none',label:tr('recent_none'),disabled:true}];
            return list.map((entry,index)=>({id:'recent-'+index,label:entry.label,action:()=>openRecent(entry.path)}));
        }
        // A kept draft goes into its document without the question load() asks otherwise.
        async function restoreDraft(entry) {
            if(ctx.readonly || !await guard())return;
            await load(entry.path,null,true);
        }
        function fileMenuItems() {
```

The menu is rebuilt when the list changes. Replace

```js
                !!s?.table,!!s?.image,s?.scope?.kind,s?.formatting?.lineSpacing?.value,leftPanel,rightPanel,root.classList.contains('writer-focus'),
```
with
```js
                !!s?.table,!!s?.image,s?.scope?.kind,s?.formatting?.lineSpacing?.value,leftPanel,rightPanel,root.classList.contains('writer-focus'),
                window.WriterMenus.recent(ctx.recentFiles?.(),path).map(entry=>entry.path).join('|'),
```

- [ ] **Step 5: The shell hands Writer the list**

In `ui/js/desktop/core/menus-and-routing.js` replace

```js
            return window.WriterApp.render(contentEl(id), id, officeAppContext(context));
```
with
```js
            // Writer lists its recent documents itself and records files it opens or saves from inside the window.
            return window.WriterApp.render(contentEl(id), id, { ...officeAppContext(context), recentFiles: readRecentFiles, recordRecentFile, forgetRecentFile: removeRecentFilesAtPath });
```

Run: `npm run build:ui`
Expected: `git status --short` lists `ui/js/desktop/bundles/main.bundle.js` as modified, and `git diff --stat -- ui/js/desktop/bundles/main.bundle.js` shows 2 insertions and 1 deletion. If the bundle shows more than that change, stop and report: someone else's source is out of step with the committed bundle.

- [ ] **Step 6: Static checks and commit**

Run:

- `node --check` on `writer.js`, `writer-dialogs.js`, `writer-menus.js`, `writer-commands.js`, `menus-and-routing.js`
- `npm run test:writer`
- `npm run check:ui`
- `go test ./ui -run "(?i)(windowmenu|contextmenu|materials|office|writer|virtualdesktop|lifecycle|i18n|feeling)" -count=1`

Expected: all pass. `writer.js` stays below 1100 lines (it has about 700).

```
git add ui/js/desktop/apps/writer.js ui/js/desktop/apps/writer-commands.js ui/js/desktop/apps/writer-menus.js ui/js/desktop/apps/writer-dialogs.js ui/js/desktop/core/menus-and-routing.js ui/js/desktop/bundles/main.bundle.js scripts/test-writer-menus.mjs
git diff --cached --check
git commit
```

Message:

```
feat(writer): recent documents in the File menu and a dialog for kept drafts

The shell hands Writer its recent list. Writer records a file it
opens, renames or saves under a new name, and a new document once a
save finds text in it; a file that is gone leaves the list. File >
Unsaved drafts lists the drafts the browser still holds and restores
one into its document without a second question, or deletes it.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: Claude Fable 5.2 <noreply@anthropic.com>
```

---

### Task 4: Browser check

**Files:**
- Create: `ui/desktop_writer_recent_test.go`
- Modify: `ui/desktop_writer_browser_test.go`

- [ ] **Step 1: The fixture keeps a recent list and the menus**

In `ui/desktop_writer_browser_test.go`, in the `/writer-file-ops-fixture` handler, replace

```go
setWindowBeforeClose:()=>{},setWindowMenus:()=>{},saveFileDialog:async()=>({path:window.saveTarget}),
```
with
```go
setWindowBeforeClose:()=>{},setWindowMenus:(id,menus)=>window.menus=menus,recentFiles:()=>window.recent||[],recordRecentFile:(path,appId)=>{window.recent=[{path,appId},...(window.recent||[]).filter(entry=>entry.path!==path)];},forgetRecentFile:path=>{window.recent=(window.recent||[]).filter(entry=>entry.path!==path);},saveFileDialog:async()=>({path:window.saveTarget}),
```

At the end of `TestDesktopWriterAppBrowserFileOps` replace

```go
        if(app.path===original)throw Error('file ops stayed on the generated path');
    }`)
}
```
with
```go
        if(app.path===original)throw Error('file ops stayed on the generated path');
    }`)
	verifyWriterRecent(t, page)
}
```

- [ ] **Step 2: The check**

Create `ui/desktop_writer_recent_test.go`. When it starts, the file operations before it have renamed the document to `Documents/renamed.docx`, saved copies as `Documents/taken.docx` and `Documents/race.docx`, and the open document is `Documents/race.docx`.

```go
package ui

import (
	"testing"

	"github.com/go-rod/rod"
)

// verifyWriterRecent runs after the file operations of the file-ops fixture. The fixture keeps the
// shell's recent list in window.recent and the window menus in window.menus.
func verifyWriterRecent(t *testing.T, page *rod.Page) {
	t.Helper()
	page.MustEval(`async()=>{
        const app=WriterApp.instances.get('test'),wait=ms=>new Promise(r=>setTimeout(r,ms));
        const until=async(test,what)=>{for(let i=0;i<120;i++){if(test())return;await wait(50);}throw Error('timed out: '+what);};
        const shown=target=>app.path===target&&!!app.editor&&document.querySelector('[data-loading]').hidden;
        const paths=()=>(window.recent||[]).map(entry=>entry.path).join();
        const recent=()=>window.menus.find(menu=>menu.id==='file').items.find(item=>item.id==='recent').items;
        const labels=()=>recent().map(item=>item.label).join();
        const pick=label=>{const item=recent().find(entry=>entry.label===label);if(!item)throw Error('the recent menu lacks '+label+': '+labels());return item.action();};
        const body=()=>app.editor.surface.session.bodyText();
        const dialog=()=>document.querySelector('dialog.vd-form-dialog');
        const select=()=>dialog()?.querySelector('[data-form-field="draft"]');
        // A scroll event makes Writer refresh, which rebuilds the menus when the list changed.
        const nudge=async()=>{document.querySelector('[data-scroll]').dispatchEvent(new Event('scroll'));await wait(250);};
        window.confirmAnswer=true;
        if(app.session.dirty)await app.session.save();
        // Rename, Save as and the retried copy were recorded; the name renamed away was forgotten.
        if(paths()!=='Documents/race.docx,Documents/taken.docx,Documents/renamed.docx')throw Error('recorded '+paths());
        if(window.recent.some(entry=>entry.appId!=='writer'))throw Error('recorded for another app');
        if(labels()!=='taken.docx,renamed.docx')throw Error('the menu lists '+labels());
        // An entry opens its document and moves to the top of the list.
        await pick('renamed.docx');
        await until(()=>shown('Documents/renamed.docx'),'the recent document did not open');
        if(!body().includes('renamed'))throw Error('the recent document reads '+JSON.stringify(body()));
        if(!paths().startsWith('Documents/renamed.docx,Documents/race.docx,'))throw Error('opening did not record: '+paths());
        if(labels()!=='race.docx,taken.docx')throw Error('after opening the menu lists '+labels());
        // A file that is gone leaves the list, and the menu still leads back.
        window.recent.unshift({path:'Documents/gone.docx',appId:'writer'});
        await nudge();
        await pick('gone.docx');
        await until(()=>!document.querySelector('[data-load-actions]').hidden,'the missing document did not fail');
        if(paths().includes('gone.docx'))throw Error('the missing document stayed in the list: '+paths());
        await pick('race.docx');
        await until(()=>shown('Documents/race.docx'),'the way back did not open');
        // A new document is listed once it holds text and was saved, not before.
        await app.act('new');
        await until(()=>app.path.includes('untitled-')&&shown(app.path),'the new document did not open');
        const fresh=app.path;
        // Saving it empty writes the file and still leaves it out of the list.
        await app.act('save');
        if(app.session.dirty)throw Error('the empty new document was not saved');
        if(paths().includes(fresh))throw Error('an empty new document was recorded');
        app.editor.exec({type:'paste',text:'First words'});
        await app.session.save();
        if(!paths().startsWith(fresh+','))throw Error('the new document with text was not recorded: '+paths());
        // Kept drafts: one of a document that no longer exists, and an older one.
        for(const entry of await WriterSession.drafts())await WriterSession.draft('delete',entry.key);
        app.editor.exec({type:'paste',text:' lost words'});
        const bytes=await app.editor.save();
        await app.session.save();
        const key=target=>location.origin+':'+target;
        await WriterSession.draft('put',key('Documents/lost.docx'),{id:'lost',bytes,revision:1,etag:null,updated:Date.now(),path:'Documents/lost.docx'});
        await WriterSession.draft('put',key('Documents/old.docx'),{id:'old',bytes,revision:1,etag:null,updated:Date.now()-60000,path:'Documents/old.docx'});
        const listed=await WriterSession.drafts();
        if(listed.map(entry=>entry.id).join()!=='lost,old'||'bytes' in listed[0]||!listed[0].size)throw Error('draft list '+JSON.stringify(listed));
        void app.act('drafts');
        await until(()=>!!select(),'the drafts dialog did not open');
        if(select().options.length!==2||!select().options[0].textContent.startsWith('lost.docx'))throw Error('drafts shown: '+select().textContent);
        if(select().size!==2||select().multiple)throw Error('the drafts are not shown as a list of two rows');
        // Delete acts on the picked draft and the dialog comes back with the rest.
        select().value=key('Documents/old.docx');
        dialog().querySelector('[data-form-extra]').click();
        await until(()=>select()?.options.length===1,'deleting a draft did not leave one');
        if((await WriterSession.drafts()).map(entry=>entry.id).join()!=='lost')throw Error('the wrong draft was deleted');
        // Restore starts a new document at the draft's path, with the draft in it.
        dialog().querySelector('[type=submit]').click();
        await until(()=>shown('Documents/lost.docx'),'the draft did not open');
        await until(()=>body().includes('lost words'),'the draft text is missing');
        await app.session.save();
        if((await WriterSession.drafts()).length)throw Error('the restored draft stayed after saving');
        if(!paths().startsWith('Documents/lost.docx,'))throw Error('the restored document was not recorded: '+paths());
        // Without drafts the dialog says so and offers no choice.
        void app.act('drafts');
        await until(()=>!!dialog(),'the empty drafts dialog did not open');
        if(select())throw Error('an empty draft list offers a choice');
        dialog().querySelector('[type=submit]').click();await wait(150);
        if(dialog())throw Error('the empty drafts dialog stayed open');
    }`)
}
```

- [ ] **Step 3: Compile, then the controller runs the browser tests**

Run: `gofmt -l ui/desktop_writer_recent_test.go ui/desktop_writer_browser_test.go` (prints nothing) and `go vet ./ui`.

The controller then deletes `reports\autor` and runs `AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run "TestDesktop(Writer|QR)" -count=1 -timeout 30m -v`.
Expected: every `TestDesktopWriter*` passes. A failure in `verifyWriterRecent` names the step in its message; fix the cause, not the check. The controller also takes one screenshot of the drafts dialog with two entries to see the list field in the dark theme.

- [ ] **Step 4: Commit**

```
git add ui/desktop_writer_recent_test.go ui/desktop_writer_browser_test.go
git diff --cached --check
git commit
```

Message:

```
test(writer): recent documents and kept drafts in the browser

The file-ops fixture keeps the shell's recent list. The check opens a
document from the menu, loses one, starts a new one, and restores and
deletes drafts.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: Claude Fable 5.2 <noreply@anthropic.com>
```

---

### Task 5: Documentation

**Files:**
- Modify: `ui/js/desktop/apps/AGENTS.md`, `documentation/autor.md`, `docs/superpowers/specs/2026-11-12-writer-office-roadmap-design.md`

- [ ] **Step 1: `ui/js/desktop/apps/AGENTS.md`**

In the Writer section, directly after the line that ends with ``shortcut overview), all through `shared/form-dialog.js`;`` and the `writer-context.js` sentence that follows it, add:

```text
  Recent documents come from the shell's list: the shell hands Writer `recentFiles`,
  `recordRecentFile` and `forgetRecentFile` in its context. `remember()` in `writer.js` records a
  file that was loaded, renamed or saved under a new name, and a new document once a save finds
  text in it, so an empty new document never enters the list; a file that answers 404 is
  forgotten. `WriterMenus.recent` picks Writer's own entries without the open document for
  File > Recent documents, and the list is part of the menu signature.
  `OfficeSession.drafts()` lists kept drafts without their bytes. File > Unsaved drafts
  (`drafts()` in `writer-dialogs.js`) restores one through `load(path, null, true)`, which skips
  the recovery question and starts a new document at the draft's path when the file is gone, or
  deletes it with `deleteIf`. In the form dialog a `select` with `size` is a list, and the extra
  button answers with the field values;
```

Then search the file and `ui/AGENTS.md` for `recordRecentFile`. Where the Notes context is described with it, add that Writer receives `recentFiles`, `recordRecentFile` and `forgetRecentFile` the same way. If neither file mentions it, add nothing.

- [ ] **Step 2: `documentation/autor.md`**

In the section "Files and recovery", replace the paragraph that starts with `A local IndexedDB draft supports explicit recovery after failures.` by:

```text
Writer keeps a local draft in the browser while a document has unsaved changes
and removes it after a successful save. Opening a document that still has a
draft asks whether to restore it. Closing or opening another document first
attempts to save. If saving fails, the user may keep the draft and close;
unavailable recovery storage prevents discarding the only unsaved copy.
**File > Unsaved drafts** lists the drafts this browser still holds, with name,
folder and time. One draft at a time can be restored into its document or
deleted. A draft whose document no longer exists is restored as a new document
at the same path.

**File > Recent documents** lists up to eight documents last opened in Writer,
newest first, without the open one. Files opened, renamed or saved under a new
name inside Writer are recorded; a new document is recorded once it has been
saved with text. The list is the desktop's shared recent list, twelve entries
across all apps, kept in this browser, so the start menu shows the same files.
A document that no longer exists leaves the list when it is picked.
```

- [ ] **Step 3: The roadmap**

In `docs/superpowers/specs/2026-11-12-writer-office-roadmap-design.md` replace the line `- **1c** — start state and recent documents. Planned after 1b.` by:

```text
- **1c** — recent documents and unsaved drafts. Plan: `docs/superpowers/plans/2026-11-13-writer-wave1c-recent-drafts.md`. The user kept the blank document as the start state (2026-11-13), so there is no start page and the templates stay in the Insert panel. Recent documents are a File submenu fed by the shell's list. The drafts dialog makes kept drafts reachable when their document no longer opens by itself.
```

- [ ] **Step 4: DOX pass and commit**

Re-read the changed paths against the `AGENTS.md` chain. `ui/AGENTS.md` and the root guide need no change unless Step 1 found `recordRecentFile` there; say so in the report. `docs/` is git-ignored, so the roadmap needs `git add -f`.

```
git add ui/js/desktop/apps/AGENTS.md documentation/autor.md
git add -f docs/superpowers/specs/2026-11-12-writer-office-roadmap-design.md
git diff --cached --check
git commit
```

Message:

```
docs(writer): recent documents and unsaved drafts

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: Claude Fable 5.2 <noreply@anthropic.com>
```

---

## Final verification (controller)

- `npm run test:writer` and `npm run check:ui` pass.
- Browser: `TestDesktop(Writer|QR)` passes twice in a row (delete `reports\autor` before each run).
- `go test ./ui -count=1` fails only in `TestDesktopCodeStudioRunUsesTerminalSession`.
- `git status --short` is empty; `disposable/_writer-wave1c/` does not exist.
- One branch review by the reader-agent (Critical and Important only), one fix round.
