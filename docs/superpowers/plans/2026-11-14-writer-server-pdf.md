# Writer — one-click PDF from the server: implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** File > Download as PDF saves a PDF with one click, File > Save as PDF in the workspace… writes it next to the user's files, and printed pages carry their pictures instead of a "Loading image" placeholder.

**Architecture:** Writer already paints the document into a hidden print frame. A new module, `writer-pdf.js`, waits for the pictures, moves them into the painted pages and packs those pages into one self-contained HTML page (engine stylesheet inline, pictures as `data:` URLs, only the font faces the pages use). The server gets that page at `POST /api/desktop/office/pdf` and renders it with `tools.RenderHTMLPDF`: Gotenberg when it is the document backend, the built-in Chromium (`security.LaunchEgressBrowser`) otherwise and as a fallback. Without a renderer the server answers `503 pdf_unavailable` and the download entry falls back to the print dialog.

**Tech stack:** Vanilla JS classic scripts loaded by `module-loader.js`, `@docx-editor.dev/core` 2.29.0 behind `lib`, node `vm` tests, Go (`net/http`, go-rod), Go + rod browser tests.

Roadmap: `docs/superpowers/specs/2026-11-12-writer-office-roadmap-design.md`, part 2b+.

---

## Read this first

**Decided by the user (2026-11-14):** two menu entries, one that downloads and one that saves into the workspace; the server renders with Gotenberg when it is set up as the document backend and with the built-in Chromium otherwise; when neither works, the download entry opens the print dialog as before. No new configuration switch.

**Scope.** In: the two File entries; the packer; the endpoint with both renderers; the fix for pictures in the print path (found while planning, see below). Out: PDF as an e-mail attachment; PDF/A; bookmarks in the PDF; a probe that greys the entries out in advance; Chromium inside the Docker image (Gotenberg serves Docker installs).

**Found while planning, 2026-11-14: today's print path prints pictures as placeholders.** `print()` in `writer-panels.js` paints the pages with `lib.paintSemanticLayout` right after the scratch editor has loaded. That standalone paint has no access to decoded pictures and writes a `.docx-drawing-placeholder` with the label "Loading image" (seen in `reports/autor/print.pdf` of `TestDesktopWriterAppBrowser`). Task 3 fixes it for print and PDF alike.

**Engine and browser behaviour, probed on 2026-11-14.** These are facts, not assumptions:

- A view-mode scratch editor paints its own pages into its container and decodes pictures there: about 180 ms after `save()` the drawing in its DOM is `.docx-drawing.docx-drawing-ready` with an `<img src="blob:…">`, and `surface.layout()` then holds the drawing with `resource.kind === 'ready'` (before that `'pending'`). Without a scroll container around it the scratch editor materialises every page: a 151-page document had all pages and its three pictures ready 660 ms after `save()`, and waiting, painting, grafting and inlining took 760 ms together.
- `lib.paintSemanticLayout(container, layout, …)` on its own paints a placeholder for every picture, also for a ready one, and painting it into the scratch editor's own container destroys the decoded pictures. So the pages are painted into a second container and the ready drawings are copied over by `data-drawing-node-id`.
- The `blob:` URLs of the pictures die with the scratch editor. `fetch(blobURL)` from the Writer window works (the page policy has `connect-src 'self' blob:` and `img-src data:`).
- Painted text runs carry the document's family first: `font-family: Calibri, Calibri, Carlito, Helvetica, Arial, sans-serif`. The fonts object (`documentFonts` in `writer.js`) has `sources: [{request: {family, weight, style}, bytes: Uint8Array, id, hash, faceIndex}]` (weights 400 and 700, styles `normal` and `italic`) and `substitutions: [{from: {family, weight, style}, to: {…}}]`, for example Calibri to Carlito and Arial to Liberation Sans. The engine lays text out with the substitute's bytes.
- A page with `@font-face{font-family:"calibri";…src:url(data:font/ttf;base64,…)}` built from those bytes, the engine stylesheet inline and pictures as `data:` URLs renders in a fresh `about:blank` page under the policy `default-src 'none'; img-src data:; style-src 'unsafe-inline'; font-src data:`. `Page.printToPDF` with `PreferCSSPageSize` gave page boxes 595 x 842 and 842 x 595 and the fonts `Carlito-Regular`, `Carlito-Bold`, `Carlito-Italic`, `LiberationSans`, `LiberationMono`. A document with three faces of Calibri packs to about 3.7 MB.
- `engine.css` contains no `url(…)`.
- The print frame (`.writer-print-frame`) is 1200 x 1000 px, off screen, so computed styles work in it.

**Server behaviour, read from the code on 2026-11-14:**

- `restrictRemoteContent` (`internal/tools/document_remote_content.go`) puts exactly that policy first into `<head>` and neutralises refresh and connection hints. `RenderReportHTMLPDF(ctx, *config.GotenbergConfig, html)` (`internal/tools/report_pdf.go`) applies it and renders through Gotenberg's Chromium route with `preferCssPageSize`.
- `security.LaunchEgressBrowser(ctx, EgressBrowserOptions{Sandbox, Bin})` is the only allowed way to a headless browser (`internal/security/AGENTS.md`). It returns `Browser *rod.Browser`, `Close()`, `Err()`, `Wrap(err)`, `ProxyDialed()`, `ProxyRefused()`; `SetEgressLaunchObserver` lets a test see the browser.
- `requireDesktopOperation(s, w, r, scope, operation)` checks the scope and, for `desktopWrite`, the read-only state. `svc.WriteFileBytesConditional(ctx, path, data, desktop.SourceUser, precondition)` writes inside the workspace under the mutation lock; `svc.ReadFileBytes` answers `os.ErrNotExist` for a missing file.
- The Docker image has no Chromium. rod downloads one on first use, as it does for `web_capture`.

**Dry run, 2026-11-14.** A script applied this plan to the worktree exactly as written: 8 files created, all 19 "replace … with" pairs matched once, the three fixture tags replaced, the language script added 5 keys and replaced 1 in each of the 16 files and changed nothing on a second run. Then:

- `npm run test:writer` (nine lines) and `npm run check:ui` passed; `go vet ./internal/tools ./internal/server ./ui` was clean.
- `go test ./internal/server -run DesktopWriterPDF` passed. `go test ./internal/tools -run "(?i)(RenderHTMLPDF|RestrictRemoteContent|DocumentCreator)"` passed with `AURAGO_RUN_BROWSER_SMOKE=1`, so the built-in browser really rendered: two sheet sizes, nothing loaded.
- `TestDesktop(Writer|QR)` passed in the browser. `reports/autor/server.pdf` showed the text, the picture and both formats; `reports/autor/print.pdf` showed the picture instead of the placeholder.
- `go test ./ui -count=1 -skip GoldfishAdventureHotspot` passed.
- `go test ./internal/server -run "(?i)(desktop|route|scope|security)"` failed in one test only, `TestGetDesktopServiceReusesCanonicalConfigWithRelativeDefaults`. That is the known failure of this machine (GOTMPDIR on D:, checkout on C:) and has nothing to do with this plan.

The dry run was reverted. Implement the plan as it stands; a block that does not match means the file changed since, so stop and report instead of adapting it. Three things the dry run found are already in the plan: the server reads the page before it refuses anything (a refusal to a half-sent upload reaches the browser as "Failed to fetch"); the file-ops fixture needs the print runtime; a test makes a desktop read-only through the server's own switch (`s.Cfg.VirtualDesktop.ReadOnly`), not through the service.

**Not verifiable here:** the Gotenberg renderer against a real sidecar (unit tests use a stand-in; `aurago-test` runs `aurago_gotenberg`, so the user's acceptance after a deploy covers it), and the built-in Chromium inside a Docker install.

**Repo rules:**

- Read the `AGENTS.md` chain for every path you touch before editing (root, `ui/AGENTS.md`, `ui/js/desktop/apps/AGENTS.md`, `scripts/AGENTS.md`, `internal/server/AGENTS.md`, `internal/tools/AGENTS.md`, `internal/security/AGENTS.md`); close out with a DOX pass (Task 6).
- Every user-visible string exists in all 16 files under `ui/lang/desktop/`. `scripts/test-writer-commands.mjs` scans every registry label and every literal `tr('key')` in `ui/js/desktop/apps/writer*.js`; write `tr('key')` with a literal key.
- Every file under `ui/js/desktop` stays below 1100 lines. No Internet assets, no new dependencies, no binary files.
- `writer.js`, `writer-panels.js`, `writer-commands.js`, `writer-menus.js` and `module-loader.js` may have CRLF line endings; new files and Go files have LF. Keep each file's line ending.
- A command is one entry in `DEFINITIONS` in `writer-commands.js`; `scripts/test-writer-menus.mjs` fails for a command no window menu lists. An app action also needs its `case` in `act` in `writer.js`.
- Temporary scripts go into `disposable/_writer-server-pdf/` and are deleted before the commit. Before each commit: `git diff --cached --check`, and look at `git diff --cached`.
- GitNexus: the index is hundreds of commits behind and cannot index worktrees. Controller's impact analysis for `print` in `writer-panels.js` (called from `act` in `writer.js` only) and `act` in `writer.js`: UNKNOWN, callers confirmed by text search. The Go code is new, except one route line. End each commit body with: `GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.`

**Environment (Windows):**

- Work in the worktree `.claude/worktrees/writer-server-pdf` (it has a real copy of `node_modules`).
- In a worktree session git commands must be plain and single (no `&&`, no `cd`).
- Browser tests: `AURAGO_RUN_BROWSER_SMOKE=1`, from PowerShell with the sandbox off. Delete `reports\autor` first. Implementer subagents cannot start Chrome; the controller runs these.
- `TestDesktopCodeStudioRunUsesTerminalSession` and `TestDesktopPolishPointerLightTargetsMatch` fail on `main` for unrelated reasons. Ignore them.

## File structure

| File | Change | Responsibility |
| --- | --- | --- |
| `ui/lang/desktop/*.json` | modify | 5 new keys in 16 languages; `pdf` becomes "Download as PDF" |
| `ui/js/desktop/apps/writer-pdf.js` | create | Wait for pictures, graft them, inline them, pack the pages; the PDF file name |
| `scripts/test-writer-pdf.mjs`, `package.json` | create, modify | Its test, part of `npm run test:writer` |
| `ui/js/desktop/core/module-loader.js`, `ui/desktop_office_assets_test.go`, three fixtures | modify | One new script |
| `ui/js/desktop/apps/writer-panels.js` | modify | `paint` (shared), `print`, `pdf` |
| `internal/tools/html_pdf.go`, `html_pdf_test.go` | create | `RenderHTMLPDF`: Gotenberg or the built-in browser |
| `internal/server/desktop_writer_pdf.go`, `desktop_writer_pdf_test.go`, `server_routes.go` | create, modify | `POST /api/desktop/office/pdf` |
| `ui/js/desktop/apps/writer-commands.js`, `writer-menus.js`, `writer.js` | modify | The `pdfSave` command; `pdf` and `pdfSave` actions |
| `scripts/test-writer-menus.mjs`, `test-writer-commands.mjs` | modify | New rules |
| `ui/desktop_writer_pdf_test.go`, `desktop_writer_browser_test.go`, `desktop_writer_templates_test.go` | create, modify | Browser checks |
| `ui/js/desktop/apps/AGENTS.md`, `internal/server/AGENTS.md`, `internal/tools/AGENTS.md`, `documentation/autor.md` | modify | Contracts and user documentation |

---

### Task 1: Language keys

**Files:**
- Modify: `ui/lang/desktop/*.json` (16 files)
- Create, then delete: `disposable/_writer-server-pdf/i18n.mjs`

- [ ] **Step 1: Write the script**

Create `disposable/_writer-server-pdf/i18n.mjs`:

```js
// Adds the server PDF keys to the sixteen desktop language files, each at its sorted place in the desktop.writer_
// block, and gives "pdf" its new text: the entry downloads now.
// Usage: node disposable/_writer-server-pdf/i18n.mjs <repo root>
import {readFileSync, writeFileSync} from 'node:fs';
import path from 'node:path';

const root = process.argv[2] || process.cwd();
const LANGS = ['cs', 'da', 'de', 'el', 'en', 'es', 'fr', 'hi', 'it', 'ja', 'nl', 'no', 'pl', 'pt', 'sv', 'zh'];
const REPLACED = ['pdf'];
const KEYS = {
    pdf: ['Stáhnout jako PDF', 'Download som PDF', 'Als PDF herunterladen', 'Λήψη ως PDF', 'Download as PDF', 'Descargar como PDF', 'Télécharger en PDF', 'PDF के रूप में डाउनलोड करें',
        'Scarica come PDF', 'PDFとしてダウンロード', 'Downloaden als pdf', 'Last ned som PDF', 'Pobierz jako PDF', 'Transferir como PDF', 'Ladda ned som PDF', '下载为 PDF'],
    pdf_save: ['Uložit jako PDF do pracovního prostoru…', 'Gem som PDF i arbejdsområdet…', 'Als PDF im Arbeitsbereich speichern…', 'Αποθήκευση ως PDF στον χώρο εργασίας…', 'Save as PDF in the workspace…',
        'Guardar como PDF en el espacio de trabajo…', 'Enregistrer en PDF dans l’espace de travail…', 'कार्यक्षेत्र में PDF के रूप में सहेजें…', 'Salva come PDF nell’area di lavoro…', 'ワークスペースにPDFとして保存…',
        'Opslaan als pdf in de werkruimte…', 'Lagre som PDF i arbeidsområdet…', 'Zapisz jako PDF w obszarze roboczym…', 'Guardar como PDF na área de trabalho…', 'Spara som PDF i arbetsytan…', '另存为 PDF 到工作区…'],
    pdf_working: ['Vytváří se PDF…', 'Opretter PDF…', 'PDF wird erstellt…', 'Δημιουργία PDF…', 'Creating the PDF…', 'Creando el PDF…', 'Création du PDF…', 'PDF बनाया जा रहा है…',
        'Creazione del PDF…', 'PDFを作成しています…', 'Pdf wordt gemaakt…', 'Oppretter PDF…', 'Tworzenie pliku PDF…', 'A criar o PDF…', 'Skapar PDF…', '正在创建 PDF…'],
    pdf_saved: ['PDF uloženo.', 'PDF gemt.', 'PDF gespeichert.', 'Το PDF αποθηκεύτηκε.', 'PDF saved.', 'PDF guardado.', 'PDF enregistré.', 'PDF सहेजा गया।',
        'PDF salvato.', 'PDFを保存しました。', 'Pdf opgeslagen.', 'PDF lagret.', 'Zapisano plik PDF.', 'PDF guardado.', 'PDF sparad.', 'PDF 已保存。'],
    pdf_failed: ['PDF se nepodařilo vytvořit.', 'PDF-filen kunne ikke oprettes.', 'Die PDF konnte nicht erstellt werden.', 'Δεν ήταν δυνατή η δημιουργία του PDF.', 'The PDF could not be created.',
        'No se pudo crear el PDF.', 'Le PDF n’a pas pu être créé.', 'PDF नहीं बनाया जा सका।', 'Impossibile creare il PDF.', 'PDFを作成できませんでした。', 'De pdf kon niet worden gemaakt.',
        'PDF-en kunne ikke opprettes.', 'Nie udało się utworzyć pliku PDF.', 'Não foi possível criar o PDF.', 'PDF-filen kunde inte skapas.', '无法创建 PDF。'],
    pdf_unavailable: ['Server teď nemůže vytvářet PDF. Použijte Tisk… a zvolte tam „Uložit jako PDF“.', 'Serveren kan ikke oprette PDF-filer lige nu. Brug Udskriv… og vælg „Gem som PDF“ der.',
        'Der Server kann gerade keine PDF erstellen. Nutze „Drucken…“ und wähle dort „Als PDF speichern“.', 'Ο διακομιστής δεν μπορεί να δημιουργήσει PDF αυτή τη στιγμή. Χρησιμοποιήστε την Εκτύπωση… και επιλέξτε εκεί «Αποθήκευση ως PDF».',
        'The server cannot create PDFs right now. Use Print… and choose “Save as PDF” there.', 'El servidor no puede crear PDF en este momento. Usa Imprimir… y elige allí «Guardar como PDF».',
        'Le serveur ne peut pas créer de PDF pour le moment. Utilisez Imprimer… et choisissez « Enregistrer au format PDF ».', 'सर्वर अभी PDF नहीं बना सकता। प्रिंट… का उपयोग करें और वहाँ “PDF के रूप में सहेजें” चुनें।',
        'Il server al momento non può creare PDF. Usa Stampa… e scegli lì «Salva come PDF».', 'サーバーは現在PDFを作成できません。「印刷…」を使い、そこで「PDFとして保存」を選んでください。',
        'De server kan nu geen pdf maken. Gebruik Afdrukken… en kies daar ‘Opslaan als pdf’.', 'Serveren kan ikke opprette PDF akkurat nå. Bruk Skriv ut… og velg «Lagre som PDF» der.',
        'Serwer nie może teraz tworzyć plików PDF. Użyj polecenia Drukuj… i wybierz tam „Zapisz jako PDF”.', 'O servidor não consegue criar PDF neste momento. Use Imprimir… e escolha aí «Guardar como PDF».',
        'Servern kan inte skapa PDF just nu. Använd Skriv ut… och välj ”Spara som PDF” där.', '服务器目前无法创建 PDF。请使用“打印…”，并在其中选择“另存为 PDF”。']
};
for (const [name, texts] of Object.entries(KEYS)) if (texts.length !== LANGS.length) throw new Error(name + ' has ' + texts.length + ' texts');
for (const [index, lang] of LANGS.entries()) {
    const file = path.join(root, 'ui/lang/desktop', lang + '.json');
    const lines = readFileSync(file, 'utf8').split('\n');
    const first = lines.find(line => /^\s*"desktop\.writer_/.test(line));
    const indent = /^(\s*)/.exec(first)[1], eol = first.endsWith('\r') ? '\r' : '';
    let added = 0, replaced = 0;
    for (const [name, texts] of Object.entries(KEYS)) {
        const key = 'desktop.writer_' + name, entry = indent + JSON.stringify(key) + ': ' + JSON.stringify(texts[index]);
        const block = lines.map((line, at) => [/^\s*"(desktop\.writer_[a-z0-9_]+)":/.exec(line)?.[1], at]).filter(([found]) => found);
        const existing = block.find(([found]) => found === key);
        if (existing) {
            if (!REPLACED.includes(name)) continue;
            const next = entry + /(,?\r?)$/.exec(lines[existing[1]])[1];
            if (next !== lines[existing[1]]) { lines[existing[1]] = next; replaced++; }
            continue;
        }
        if (REPLACED.includes(name)) throw new Error(lang + ' lacks ' + key);
        const after = block.find(([found]) => found > key);
        lines.splice(after ? after[1] : block.at(-1)[1] + 1, 0, entry + ',' + eol);
        added++;
    }
    const text = lines.join('\n');
    JSON.parse(text);
    writeFileSync(file, text);
    console.log(lang + ': ' + added + ' keys added, ' + replaced + ' replaced');
}
```

- [ ] **Step 2: Run it**

Run from the repo root: `node disposable/_writer-server-pdf/i18n.mjs .`

Expected: sixteen lines, each `<lang>: 5 keys added, 1 replaced`; `git diff --stat -- ui/lang/desktop` shows 16 files with 6 insertions and 1 deletion each. A second run prints `0 keys added, 0 replaced` sixteen times. Then delete `disposable/_writer-server-pdf/`.

- [ ] **Step 3: Check and commit**

Run: `npm run test:writer` — expected: eight lines, all passing. Run: `go test ./ui -run "(?i)i18n" -count=1` — expected: `ok`.

Commit `ui/lang/desktop/*.json` as `feat(writer): language keys for the server PDF`.

---

### Task 2: The packer module

**Files:**
- Create: `ui/js/desktop/apps/writer-pdf.js`, `scripts/test-writer-pdf.mjs`
- Modify: `package.json`, `ui/js/desktop/core/module-loader.js`, `ui/desktop_office_assets_test.go`, `ui/desktop_writer_browser_test.go`, `ui/desktop_writer_reload_browser_test.go`

- [ ] **Step 1: Write the failing test**

Create `scripts/test-writer-pdf.mjs`:

```js
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import vm from 'node:vm';

const context = vm.createContext({window: {}, btoa, setTimeout});
vm.runInContext(readFileSync('ui/js/desktop/apps/writer-pdf.js', 'utf8'), context);
const P = context.window.WriterPdf;
const same = (actual, expected, message) => assert.equal(JSON.stringify(actual), JSON.stringify(expected), message);

// Faces: one key per family, weight class and slant; the first family of a font list counts.
same(P.used([['Calibri, Carlito, sans-serif', '400', 'normal'], ['"Courier New", Calibri', '700', 'italic'], ['Calibri', 'bold', 'normal'], ['calibri', '400', 'normal'], ['Calibri', '600', 'oblique']]),
    ['calibri|r|n', 'courier new|b|i', 'calibri|b|n', 'calibri|b|i']);
const bytes = text => new TextEncoder().encode(text);
const fonts = {
    sources: [
        {request: {family: 'Carlito', weight: 400, style: 'normal'}, bytes: bytes('carlito-regular')},
        {request: {family: 'Carlito', weight: 700, style: 'normal'}, bytes: bytes('carlito-bold')},
        {request: {family: 'Liberation Mono', weight: 700, style: 'italic'}, bytes: bytes('mono-bold-italic')}],
    substitutions: [
        {from: {family: 'Calibri', weight: 400, style: 'normal'}, to: {family: 'Carlito', weight: 400, style: 'normal'}},
        {from: {family: 'Calibri', weight: 700, style: 'normal'}, to: {family: 'Carlito', weight: 700, style: 'normal'}},
        {from: {family: 'Courier New', weight: 700, style: 'italic'}, to: {family: 'Liberation Mono', weight: 700, style: 'italic'}},
        {from: {family: 'Symbol', weight: 400, style: 'normal'}, to: {family: 'Missing', weight: 400, style: 'normal'}}]
};
assert.equal(P.source(fonts, 'carlito|r|n').bytes, fonts.sources[0].bytes, 'a family with its own file');
assert.equal(P.source(fonts, 'calibri|b|n').bytes, fonts.sources[1].bytes, 'a substituted family takes the file it is laid out with');
assert.equal(P.source(fonts, 'calibri|b|i'), null, 'no file for a face nobody has');
assert.equal(P.source(fonts, 'symbol|r|n'), null, 'a substitute without a file');
assert.equal(P.faces(fonts, ['calibri|r|n', 'courier new|b|i', 'system-ui|r|n']),
    '@font-face{font-family:"calibri";font-weight:400;font-style:normal;src:url(data:font/ttf;base64,' + btoa('carlito-regular') + ')}'
    + '@font-face{font-family:"courier new";font-weight:700;font-style:italic;src:url(data:font/ttf;base64,' + btoa('mono-bold-italic') + ')}');
const big = {sources: [{request: {family: 'Big', weight: 400, style: 'normal'}, bytes: new Uint8Array(70000).fill(65)}], substitutions: []};
assert.equal(P.faces(big, ['big|r|n']), '@font-face{font-family:"big";font-weight:400;font-style:normal;src:url(data:font/ttf;base64,' + btoa('A'.repeat(70000)) + ')}', 'long files survive the chunks');

// The page: no script, no link, the title escaped.
const html = P.page({title: 'Q3 <Report> & more', engine: '.docx-page{color:black}', faces: '@font-face{}', sheets: '@page{size:1px 2px}', body: '<div id="pages"></div>'});
assert.equal(html, '<!doctype html><html><head><meta charset="utf-8"><title>Q3 &lt;Report&gt; &amp; more</title><style>.docx-page{color:black}</style><style>@font-face{}@page{size:1px 2px}</style></head><body><div id="pages"></div></body></html>');
assert.ok(!/<script|<link/i.test(html));

// File names.
assert.equal(P.fileName('Documents/Reports/Q3.docx'), 'Q3.pdf');
assert.equal(P.fileName('Q3.DOCX'), 'Q3.pdf');
assert.equal(P.fileName('Documents/notes'), 'notes.pdf');
assert.equal(P.fileName(''), 'document.pdf');

// Pictures that are still loading.
assert.equal(P.pending({pages: [{fragments: [{drawings: [{resource: {kind: 'ready'}}, {resource: {kind: 'pending'}}]}]}]}), true);
assert.equal(P.pending({pages: [{fragments: [{drawings: [{resource: {kind: 'ready'}}, {resource: {kind: 'invalid'}}]}]}]}), false);
assert.equal(P.pending(vm.runInContext('new Map([["a", {resource: {kind: "pending"}}]])', context)), true, 'maps are walked');
const loop = {resource: {kind: 'ready'}};
loop.self = loop;
assert.equal(P.pending(loop), false, 'a cycle ends');
assert.equal(P.pending({bytes: new Uint8Array(1000), resource: null}), false);
let calls = 0;
await P.pictures({surface: {layout: () => ({drawing: {resource: {kind: ++calls < 3 ? 'pending' : 'ready'}}})}}, 2000);
assert.equal(calls, 3, 'pictures waits until nothing is pending');
const started = Date.now();
await P.pictures({surface: {layout: () => ({drawing: {resource: {kind: 'pending'}}})}}, 120);
assert.ok(Date.now() - started < 1000, 'pictures gives up after its limit');

console.log('Writer PDF: faces, font sources, page, file names and the wait for pictures pass');
```

- [ ] **Step 2: Run it to see it fail**

Run: `node scripts/test-writer-pdf.mjs` — expected: it fails with `ENOENT` for `writer-pdf.js`.

- [ ] **Step 3: Write the module**

Create `ui/js/desktop/apps/writer-pdf.js`:

```js
(function () {
    'use strict';
    // The print pages of a document as one self-contained HTML page for the server's PDF renderer: the engine
    // stylesheet inline, pictures as data: URLs and only the font faces the pages use. The page carries no script
    // and no link; the server renders it under a policy that forbids both.
    const STYLE_URL = '/js/vendor/writer/engine.css';
    let stylePromise = null;

    const bold = weight => weight === 'bold' || Number(weight) >= 600;
    const slanted = style => !!style && style !== 'normal';
    const family = value => String(value || '').split(',')[0].trim().replace(/^["']|["']$/g, '').toLowerCase();
    // key names one face: the family, bold or regular, italic or normal.
    const key = (name, weight, style) => family(name) + '|' + (bold(weight) ? 'b' : 'r') + '|' + (slanted(style) ? 'i' : 'n');
    const requestKey = request => key(request.family, request.weight, request.style);

    // used turns [family list, weight, style] triples of painted text into face keys, each once.
    function used(triples) { return [...new Set(triples.map(([name, weight, style]) => key(name, weight, style)))]; }

    // source finds the font file of a face: the family's own, or the substitute the engine lays the family out with.
    function source(fonts, faceKey) {
        const own = fonts.sources.find(item => requestKey(item.request) === faceKey);
        if (own) return own;
        const swap = fonts.substitutions.find(item => requestKey(item.from) === faceKey);
        return swap ? fonts.sources.find(item => requestKey(item.request) === requestKey(swap.to)) || null : null;
    }

    function base64(bytes) {
        let text = '';
        for (let index = 0; index < bytes.length; index += 32768) text += String.fromCharCode.apply(null, bytes.subarray(index, index + 32768));
        return btoa(text);
    }

    // faces writes one @font-face rule per face that has a file, under the family name the pages ask for.
    function faces(fonts, keys) {
        return keys.map(faceKey => {
            const item = source(fonts, faceKey);
            if (!item) return '';
            const parts = faceKey.split('|'), style = parts.pop(), weight = parts.pop(), name = parts.join('|').replace(/["\\]/g, '');
            return '@font-face{font-family:"' + name + '";font-weight:' + (weight === 'b' ? 700 : 400) + ';font-style:' + (style === 'i' ? 'italic' : 'normal')
                + ';src:url(data:font/ttf;base64,' + base64(item.bytes) + ')}';
        }).join('');
    }

    const escapeText = value => String(value).replace(/[&<>]/g, sign => ({'&': '&amp;', '<': '&lt;', '>': '&gt;'}[sign]));

    // page joins the parts into the document the server renders.
    function page(parts) {
        return '<!doctype html><html><head><meta charset="utf-8"><title>' + escapeText(parts.title || '') + '</title><style>' + parts.engine + '</style><style>'
            + parts.faces + parts.sheets + '</style></head><body>' + parts.body + '</body></html>';
    }

    // fileName proposes the PDF's name from the document's path.
    function fileName(path) { return (String(path || '').split('/').pop().replace(/\.docx$/i, '') || 'document') + '.pdf'; }

    // pending says whether a layout still waits for a picture. The walk stops at the first one.
    function pending(layout) {
        const seen = new Set();
        const walk = (value, depth) => {
            if (!value || typeof value !== 'object' || depth > 16 || ArrayBuffer.isView(value) || seen.has(value)) return false;
            seen.add(value);
            if (value.resource && value.resource.kind === 'pending') return true;
            const items = value instanceof Map ? value.values() : Array.isArray(value) ? value : Object.values(value);
            for (const item of items) if (walk(item, depth + 1)) return true;
            return false;
        };
        return walk(layout, 0);
    }

    // pictures waits until a scratch editor has decoded its pictures, at most limit milliseconds.
    async function pictures(editor, limit = 8000) {
        const until = Date.now() + limit;
        while (pending(editor.surface.layout()) && Date.now() < until) await new Promise(resolve => setTimeout(resolve, 50));
    }

    // graft copies the decoded pictures of the scratch editor (source) into the painted pages. A page painted on
    // its own only gets a placeholder for a picture; the drawing's node id connects the two.
    function graft(source, pages) {
        const ready = new Map();
        for (const drawing of source.querySelectorAll('.docx-drawing-ready[data-drawing-node-id]')) ready.set(drawing.dataset.drawingNodeId, drawing);
        let count = 0;
        for (const hole of pages.querySelectorAll('.docx-drawing-placeholder[data-drawing-node-id]')) {
            const drawing = ready.get(hole.dataset.drawingNodeId);
            if (!drawing) continue;
            const copy = drawing.cloneNode(true);
            for (const side of ['left', 'top', 'width', 'height']) copy.style[side] = hole.style[side];
            hole.replaceWith(copy);
            count++;
        }
        return count;
    }

    function dataURL(blob) {
        return new Promise((resolve, reject) => {
            const reader = new FileReader();
            reader.onload = () => resolve(reader.result);
            reader.onerror = () => reject(reader.error);
            reader.readAsDataURL(blob);
        });
    }

    // inline turns the pictures' blob: addresses into data: ones. A blob: address dies with the editor that made it.
    async function inline(pages) {
        for (const image of pages.querySelectorAll('img')) {
            if (!/^blob:/.test(image.src)) continue;
            image.src = await dataURL(await (await fetch(image.src)).blob());
        }
    }

    // pack reads the painted pages of a print frame (see paint in writer-panels.js) and returns the page.
    async function pack(doc, fonts, title) {
        const pages = doc.getElementById('pages'), view = doc.defaultView, triples = [];
        for (const element of pages.querySelectorAll('*')) {
            if (![...element.childNodes].some(node => node.nodeType === 3 && node.textContent.trim())) continue;
            const style = view.getComputedStyle(element);
            triples.push([style.fontFamily, style.fontWeight, style.fontStyle]);
        }
        stylePromise ||= fetch(STYLE_URL).then(response => {
            if (!response.ok) throw new Error('HTTP ' + response.status);
            return response.text();
        });
        let engine;
        try { engine = await stylePromise; } catch (error) { stylePromise = null; throw error; }
        const sheets = [...doc.head.querySelectorAll('style')].map(style => style.textContent).join('');
        return page({title, engine, faces: faces(fonts, used(triples)), sheets, body: pages.outerHTML});
    }

    window.WriterPdf = {used, source, faces, page, fileName, pending, pictures, graft, inline, pack};
})();
```

- [ ] **Step 4: Run the test**

Run: `node scripts/test-writer-pdf.mjs` — expected: `Writer PDF: faces, font sources, page, file names and the wait for pictures pass`.

- [ ] **Step 5: Register the test and the script**

In `package.json`, replace

```json
 && node scripts/test-writer-templates.mjs",
```
with
```json
 && node scripts/test-writer-templates.mjs && node scripts/test-writer-pdf.mjs",
```

In `ui/js/desktop/core/module-loader.js`, replace

```js
'/js/desktop/apps/writer-templates.js', '/js/desktop/apps/writer-commands.js',
```
with
```js
'/js/desktop/apps/writer-templates.js', '/js/desktop/apps/writer-pdf.js', '/js/desktop/apps/writer-commands.js',
```

In `ui/desktop_office_assets_test.go`, replace

```go
		"/js/desktop/apps/writer-templates.js",
```
with
```go
		"/js/desktop/apps/writer-templates.js",
		"/js/desktop/apps/writer-pdf.js",
```

In `ui/desktop_writer_browser_test.go` (two fixtures) and `ui/desktop_writer_reload_browser_test.go` (one fixture), replace every occurrence of

```go
<script src="/js/desktop/apps/writer-templates.js"></script>
```
by
```go
<script src="/js/desktop/apps/writer-templates.js"></script><script src="/js/desktop/apps/writer-pdf.js"></script>
```

- [ ] **Step 6: Check and commit**

Run: `npm run test:writer` — expected: nine lines, the last one from the PDF test. Run: `npm run check:ui` — expected: it passes (`module-loader.js` is not part of the bundle). Run: `go test ./ui -run "(?i)(office|writer)" -count=1` — expected: `ok`.

Commit as `feat(writer): pack the print pages for a PDF renderer`.

---

### Task 3: Paint once, print or pack; pictures in the print

**Files:**
- Modify: `ui/js/desktop/apps/writer-panels.js`, `ui/desktop_writer_browser_test.go`

- [ ] **Step 1: Make the browser test ask for the picture**

In `ui/desktop_writer_browser_test.go`, replace

```go
        await app.act('print');observer.disconnect();
```
with
```go
        await app.act('print');observer.disconnect();
        // A page painted on its own only gets a placeholder for a picture; the print carries the decoded one.
        if(window.printProbe && (/docx-drawing-placeholder/.test(printProbe.html) || !/<img[^>]+src="data:image\/png/.test(printProbe.html)))throw Error('the printed pages lack the picture');
```

The controller runs `TestDesktopWriterAppBrowser` and sees it fail with `the printed pages lack the picture`.

- [ ] **Step 2: Split the print**

In `ui/js/desktop/apps/writer-panels.js`, replace

```js
        // title names the document, so that the print dialog can propose it as the file name of a PDF.
        async function print(bytes,lib,fonts,title) {
            // A separate snapshot keeps edits and review state out of the print transaction.
            printFrame?.dispose();
            const job=await window.AuraDesktopPrint.create({html:'<!doctype html><html><head><link rel="stylesheet" href="/js/vendor/writer/engine.css"></head><body><div class="docx-editor" id="pages"></div></body></html>',title:tr('print'),documentTitle:title || '',className:'writer-print-frame',signal:life.signal});
            printFrame=job;
            const doc=job.document;
            const printEditor=lib.createDocxEditor({container:doc.getElementById('pages'),document:bytes,fonts,mode:'view',zoom:1,modules:[lib.reviewModule]});
            try {
                await printEditor.save();
                for (const face of document.fonts) doc.fonts.add(face);
                await doc.fonts.ready;
                lib.paintSemanticLayout(doc.getElementById('pages'), printEditor.surface.layout(), {scale:96/72,ariaHidden:false,defaultFontFamily:'Calibri'});
                // Every painted page carries its own size. A named page per size lets a landscape section inside a
                // portrait document print as landscape; the section at the caret no longer decides for all pages.
                const setup=printEditor.snapshot().pageSetup,fallback=setup.pageWidthTwips/20+'pt '+setup.pageHeightTwips/20+'pt';
                const sheetPages=[...doc.querySelectorAll('#pages .docx-page')];
                const sheets=L.sheets(sheetPages.map(page=>page.style.width && page.style.height?page.style.width+' '+page.style.height:fallback));
                sheetPages.forEach((page,index)=>page.style.setProperty('page',sheets.names[index]));
                const style=doc.createElement('style');
                style.textContent=sheets.css+'html,body,#pages{margin:0!important;padding:0!important;height:auto!important;overflow:visible!important;background:white!important}.docx-page{position:relative!important;top:auto!important;left:auto!important;transform:none!important;break-after:page!important;break-inside:avoid!important;margin:0!important;box-shadow:none!important}.docx-page:last-child{break-after:auto!important}';
                doc.head.appendChild(style);
                await job.print();
            } catch(error) {job.dispose();throw error;}
            finally {printEditor.destroy();}
        }
        function reset(){aiRequest?.abort();suggestion=null;renderSuggestion();matches=[];reviewSignature=outlineSignature=searchSignature='';outlineActive='';}
        return {show,update,print,reset,dispose(){life.abort();reset();printFrame?.dispose();}};
```
with
```js
        // paint renders the document into a hidden print frame and returns the job: every page with its own size
        // and its pictures. title names the document, so that the print dialog can propose it as a PDF's file name.
        async function paint(bytes,lib,fonts,title) {
            // A separate snapshot keeps edits and review state out of the print transaction.
            printFrame?.dispose();
            const job=await window.AuraDesktopPrint.create({html:'<!doctype html><html><head><link rel="stylesheet" href="/js/vendor/writer/engine.css"></head><body><div class="docx-editor" id="source"></div><div class="docx-editor" id="pages"></div></body></html>',title:tr('print'),documentTitle:title || '',className:'writer-print-frame',signal:life.signal});
            printFrame=job;
            const doc=job.document,P=window.WriterPdf;
            // The scratch editor lives in #source and decodes the pictures there. A page painted on its own (#pages)
            // only gets placeholders for them, so the decoded ones are copied over and freed from the editor.
            const printEditor=lib.createDocxEditor({container:doc.getElementById('source'),document:bytes,fonts,mode:'view',zoom:1,modules:[lib.reviewModule]});
            try {
                await printEditor.save();
                for (const face of document.fonts) doc.fonts.add(face);
                await doc.fonts.ready;
                await P.pictures(printEditor);
                lib.paintSemanticLayout(doc.getElementById('pages'), printEditor.surface.layout(), {scale:96/72,ariaHidden:false,defaultFontFamily:'Calibri'});
                P.graft(doc.getElementById('source'),doc.getElementById('pages'));
                await P.inline(doc.getElementById('pages'));
                // Every painted page carries its own size. A named page per size lets a landscape section inside a
                // portrait document print as landscape; the section at the caret no longer decides for all pages.
                const setup=printEditor.snapshot().pageSetup,fallback=setup.pageWidthTwips/20+'pt '+setup.pageHeightTwips/20+'pt';
                const sheetPages=[...doc.querySelectorAll('#pages .docx-page')];
                const sheets=L.sheets(sheetPages.map(page=>page.style.width && page.style.height?page.style.width+' '+page.style.height:fallback));
                sheetPages.forEach((page,index)=>page.style.setProperty('page',sheets.names[index]));
                const style=doc.createElement('style');
                style.textContent=sheets.css+'#source{display:none!important}html,body,#pages{margin:0!important;padding:0!important;height:auto!important;overflow:visible!important;background:white!important}.docx-page{position:relative!important;top:auto!important;left:auto!important;transform:none!important;break-after:page!important;break-inside:avoid!important;margin:0!important;box-shadow:none!important}.docx-page:last-child{break-after:auto!important}';
                doc.head.appendChild(style);
                return job;
            } catch(error) {job.dispose();throw error;}
            finally {printEditor.destroy();}
        }
        async function print(bytes,lib,fonts,title) {
            const job=await paint(bytes,lib,fonts,title);
            try {await job.print();} catch(error) {job.dispose();throw error;}
        }
        // pdf returns the painted pages as one self-contained HTML page for the server's PDF renderer.
        async function pdf(bytes,lib,fonts,title) {
            const job=await paint(bytes,lib,fonts,title);
            try {return await window.WriterPdf.pack(job.document,fonts,title);} finally {job.dispose();}
        }
        function reset(){aiRequest?.abort();suggestion=null;renderSuggestion();matches=[];reviewSignature=outlineSignature=searchSignature='';outlineActive='';}
        return {show,update,print,pdf,reset,dispose(){life.abort();reset();printFrame?.dispose();}};
```

- [ ] **Step 3: Check and commit**

Run: `npm run test:writer` and `npm run check:ui` — expected: both pass. The controller runs `TestDesktopWriterAppBrowser` in the browser: it passes, and `reports/autor/print.pdf` shows the picture on page 2 instead of "Loading image".

Commit as `fix(writer): printed pages carry their pictures`.

---

### Task 4: The server renders the page

**Files:**
- Create: `internal/tools/html_pdf.go`, `internal/tools/html_pdf_test.go`, `internal/server/desktop_writer_pdf.go`, `internal/server/desktop_writer_pdf_test.go`
- Modify: `internal/server/server_routes.go`

- [ ] **Step 1: Write the failing renderer tests**

Create `internal/tools/html_pdf_test.go`:

```go
package tools

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/security"
	"aurago/internal/testutil"
)

func TestRenderHTMLPDFChoosesTheRenderer(t *testing.T) {
	oldGotenberg, oldChromium := htmlPDFGotenberg, htmlPDFChromium
	defer func() { htmlPDFGotenberg, htmlPDFChromium = oldGotenberg, oldChromium }()
	var calls []string
	var browserPage string
	var gotenbergErr, chromiumErr error
	htmlPDFGotenberg = func(ctx context.Context, cfg *config.GotenbergConfig, page string) ([]byte, error) {
		calls = append(calls, "gotenberg "+cfg.URL)
		return []byte("%PDF-gotenberg"), gotenbergErr
	}
	htmlPDFChromium = func(ctx context.Context, page string) ([]byte, error) {
		calls = append(calls, "browser")
		browserPage = page
		return []byte("%PDF-browser"), chromiumErr
	}
	page := `<!doctype html><html><head><title>x</title></head><body>pages</body></html>`
	withGotenberg := &config.Config{}
	withGotenberg.Tools.DocumentCreator.Enabled = true
	withGotenberg.Tools.DocumentCreator.Backend = "gotenberg"
	withGotenberg.Tools.DocumentCreator.Gotenberg.URL = "http://gotenberg:3000"
	withMaroto := &config.Config{}
	withMaroto.Tools.DocumentCreator.Enabled = true
	withMaroto.Tools.DocumentCreator.Backend = "maroto"
	disabled := &config.Config{}
	disabled.Tools.DocumentCreator.Backend = "gotenberg"

	for _, test := range []struct {
		name       string
		cfg        *config.Config
		gotenberg  error
		chromium   error
		want       string
		wantCalls  string
		wantErr    bool
		wantErrHas []string
	}{
		{name: "no config", want: "%PDF-browser", wantCalls: "browser"},
		{name: "another backend", cfg: withMaroto, want: "%PDF-browser", wantCalls: "browser"},
		{name: "document creator off", cfg: disabled, want: "%PDF-browser", wantCalls: "browser"},
		{name: "gotenberg", cfg: withGotenberg, want: "%PDF-gotenberg", wantCalls: "gotenberg http://gotenberg:3000"},
		{name: "gotenberg fails", cfg: withGotenberg, gotenberg: errors.New("refused"), want: "%PDF-browser", wantCalls: "gotenberg http://gotenberg:3000,browser"},
		{name: "both fail", cfg: withGotenberg, gotenberg: errors.New("refused"), chromium: errors.New("no binary"), wantCalls: "gotenberg http://gotenberg:3000,browser", wantErr: true, wantErrHas: []string{"gotenberg: refused", "browser: no binary"}},
		{name: "browser fails alone", chromium: errors.New("no binary"), wantCalls: "browser", wantErr: true, wantErrHas: []string{"browser: no binary"}},
	} {
		calls, gotenbergErr, chromiumErr = nil, test.gotenberg, test.chromium
		data, err := RenderHTMLPDF(context.Background(), test.cfg, page)
		if got := strings.Join(calls, ","); got != test.wantCalls {
			t.Fatalf("%s: renderers %q, want %q", test.name, got, test.wantCalls)
		}
		if test.wantErr {
			if !errors.Is(err, ErrHTMLPDFUnavailable) {
				t.Fatalf("%s: error %v is not ErrHTMLPDFUnavailable", test.name, err)
			}
			for _, part := range test.wantErrHas {
				if !strings.Contains(err.Error(), part) {
					t.Fatalf("%s: error %q lacks %q", test.name, err, part)
				}
			}
			continue
		}
		if err != nil || string(data) != test.want {
			t.Fatalf("%s: %q, %v", test.name, data, err)
		}
	}

	// The built-in browser gets the page behind the policy; Gotenberg's renderer applies it itself.
	if at, title := strings.Index(browserPage, remoteContentCSPMeta), strings.Index(browserPage, "<title>"); at < 0 || at > title {
		t.Fatalf("the browser page has no policy in front of its head content: %s", browserPage)
	}

	// A cancelled request is not "no renderer".
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	calls, gotenbergErr, chromiumErr = nil, errors.New("refused"), nil
	if _, err := RenderHTMLPDF(ctx, withGotenberg, page); !errors.Is(err, context.Canceled) || errors.Is(err, ErrHTMLPDFUnavailable) || len(calls) != 1 {
		t.Fatalf("cancelled render: %v after %v", err, calls)
	}
}

// The built-in browser keeps the page's own sheet sizes and loads nothing the page points at.
func TestRenderHTMLPDFBrowserKeepsPageSizesAndLoadsNothing(t *testing.T) {
	bin := testutil.RequireBrowserBin(t)
	previous := htmlPDFBrowserBin
	htmlPDFBrowserBin = bin
	defer func() { htmlPDFBrowserBin = previous }()
	var launched *security.EgressBrowser
	restore := security.SetEgressLaunchObserver(func(eb *security.EgressBrowser) { launched = eb })
	defer restore()
	var hits atomic.Int32
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits.Add(1) }))
	defer remote.Close()
	page := `<!doctype html><html><head><style>@page{size:595pt 842pt;margin:0}@page wide{size:842pt 595pt;margin:0}div{break-after:page}.wide{page:wide;break-after:auto}</style></head><body><div>Portrait</div><div class="wide">Landscape<img src="` +
		remote.URL + `/picture.png"><script>fetch("` + remote.URL + `/script")</script></div></body></html>`
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	data, err := RenderHTMLPDF(ctx, nil, page)
	if err != nil {
		t.Fatal(err)
	}
	portrait, landscape := 0, 0
	for _, box := range regexp.MustCompile(`/MediaBox \[0 0 ([0-9.]+) ([0-9.]+)\]`).FindAllSubmatch(data, -1) {
		width, _ := strconv.ParseFloat(string(box[1]), 64)
		height, _ := strconv.ParseFloat(string(box[2]), 64)
		if width > height {
			landscape++
		} else {
			portrait++
		}
	}
	if portrait != 1 || landscape != 1 {
		t.Fatalf("the PDF has %d portrait and %d landscape pages instead of 1 and 1", portrait, landscape)
	}
	if hits.Load() != 0 {
		t.Fatalf("the page reached its remote host %d times", hits.Load())
	}
	if launched == nil {
		t.Fatal("no egress browser was launched")
	}
	host := strings.TrimPrefix(remote.URL, "http://")
	for _, seen := range append(launched.ProxyDialed(), launched.ProxyRefused()...) {
		if seen == host {
			t.Fatalf("a request for %s got as far as the proxy", host)
		}
	}
}
```

Run: `go test ./internal/tools -run "RenderHTMLPDF" -count=1` — expected: a build failure, `undefined: htmlPDFGotenberg`.

- [ ] **Step 2: Write the renderer**

Create `internal/tools/html_pdf.go`:

```go
package tools

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"aurago/internal/config"
	"aurago/internal/security"

	"github.com/go-rod/rod/lib/proto"
)

// ErrHTMLPDFUnavailable means that no renderer produced a PDF: Gotenberg is not the document backend or failed,
// and the built-in browser could not render either.
var ErrHTMLPDFUnavailable = errors.New("no PDF renderer is available")

// htmlPDFMaxBytes bounds a rendered PDF.
const htmlPDFMaxBytes = 64 << 20

// The seams let unit tests choose the renderers. htmlPDFBrowserBin is for tests only; empty uses rod's lookup.
var (
	htmlPDFGotenberg  = RenderReportHTMLPDF
	htmlPDFChromium   = renderHTMLPDFChromium
	htmlPDFBrowserBin string
)

// RenderHTMLPDF renders one self-contained HTML page to a PDF whose sheet sizes come from the page's own @page
// rules. The page is data: both renderers get it behind restrictRemoteContent's policy, so it runs no script and
// loads nothing that is not inline. Gotenberg renders when it is the document backend; the built-in browser
// renders otherwise, and when Gotenberg fails.
func RenderHTMLPDF(ctx context.Context, cfg *config.Config, page string) ([]byte, error) {
	var failures []string
	if cfg != nil && cfg.Tools.DocumentCreator.Enabled && cfg.Tools.DocumentCreator.Backend == "gotenberg" {
		data, err := htmlPDFGotenberg(ctx, &cfg.Tools.DocumentCreator.Gotenberg, page)
		if err == nil {
			return data, nil
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		failures = append(failures, "gotenberg: "+err.Error())
	}
	data, err := htmlPDFChromium(ctx, restrictRemoteContent(page))
	if err == nil {
		return data, nil
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	failures = append(failures, "browser: "+err.Error())
	return nil, fmt.Errorf("%w (%s)", ErrHTMLPDFUnavailable, strings.Join(failures, "; "))
}

// renderHTMLPDFChromium prints the page in a fresh egress browser. The page comes in as document content, never as
// a URL, and scripts stay off even where the policy would not reach.
func renderHTMLPDFChromium(ctx context.Context, page string) ([]byte, error) {
	eb, err := security.LaunchEgressBrowser(ctx, security.EgressBrowserOptions{Sandbox: true, Bin: htmlPDFBrowserBin})
	if err != nil {
		return nil, fmt.Errorf("browser launch failed: %w", err)
	}
	defer func() { _ = eb.Close() }()
	target, err := eb.Browser.Page(proto.TargetCreateTarget{URL: "about:blank"})
	if err != nil {
		return nil, eb.Wrap(err)
	}
	target = target.Context(ctx)
	if err := (proto.EmulationSetScriptExecutionDisabled{Value: true}).Call(target); err != nil {
		return nil, eb.Wrap(err)
	}
	if err := target.SetDocumentContent(page); err != nil {
		return nil, eb.Wrap(err)
	}
	// Fonts and pictures are inline, but decoding them takes a moment.
	if _, err := target.Eval(`async () => { await document.fonts.ready; await Promise.all([...document.images].map(image => image.decode().catch(() => {}))); }`); err != nil {
		return nil, eb.Wrap(err)
	}
	stream, err := target.PDF(&proto.PagePrintToPDF{PrintBackground: true, PreferCSSPageSize: true})
	if err != nil {
		return nil, eb.Wrap(err)
	}
	data, err := io.ReadAll(io.LimitReader(stream, htmlPDFMaxBytes+1))
	if err != nil {
		return nil, eb.Wrap(err)
	}
	if cause := eb.Err(); cause != nil {
		return nil, cause
	}
	if len(data) > htmlPDFMaxBytes {
		return nil, fmt.Errorf("PDF exceeds %d MiB", htmlPDFMaxBytes>>20)
	}
	if !bytes.HasPrefix(data, []byte("%PDF-")) {
		return nil, errors.New("the browser returned a non-PDF response")
	}
	return data, nil
}
```

Run: `go test ./internal/tools -run "RenderHTMLPDF" -count=1 -v` — expected: `TestRenderHTMLPDFChoosesTheRenderer` passes; the browser test is skipped without `AURAGO_RUN_BROWSER_SMOKE=1`. The controller runs the browser test with the variable set.

- [ ] **Step 3: Write the failing endpoint test**

Create `internal/server/desktop_writer_pdf_test.go`:

```go
package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/tools"
)

func TestDesktopWriterPDFDownloadsAndSaves(t *testing.T) {
	s := newDesktopOfficeTestServer(t)
	previous := desktopWriterPDFRender
	defer func() { desktopWriterPDFRender = previous }()
	var pages []string
	var renderErr error
	desktopWriterPDFRender = func(ctx context.Context, cfg *config.Config, page string) ([]byte, error) {
		pages = append(pages, page)
		if renderErr != nil {
			return nil, renderErr
		}
		return []byte(fmt.Sprintf("%%PDF-1.7 render %d", len(pages))), nil
	}
	handler := handleDesktopWriterPDF(s)
	post := func(query, body string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		handler(rec, httptest.NewRequest(http.MethodPost, "/api/desktop/office/pdf"+query, strings.NewReader(body)))
		return rec
	}
	stored := func(path string) string {
		svc, _, err := s.getDesktopService(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		data, _, err := svc.ReadFileBytes(context.Background(), path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		return string(data)
	}

	// Without a path the response is the PDF.
	got := post("", "<html>pages</html>")
	if got.Code != http.StatusOK || got.Header().Get("Content-Type") != "application/pdf" || got.Body.String() != "%PDF-1.7 render 1" || pages[0] != "<html>pages</html>" {
		t.Fatalf("download: %d %q %q", got.Code, got.Header().Get("Content-Type"), got.Body)
	}
	if got.Header().Get("Cache-Control") != "no-store" || got.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("download headers: %v", got.Header())
	}
	rec := httptest.NewRecorder()
	handler(rec, httptest.NewRequest(http.MethodGet, "/api/desktop/office/pdf", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET: %d", rec.Code)
	}
	if got := post("", "  \n"); got.Code != http.StatusBadRequest || len(pages) != 1 {
		t.Fatalf("empty page: %d after %d renders", got.Code, len(pages))
	}

	// With a path the PDF becomes a file: create only, then with overwrite.
	target := "?path=Documents/Exports/report.pdf"
	if got := post(target, "<html>one</html>"); got.Code != http.StatusOK || !strings.Contains(got.Body.String(), `"path":"Documents/Exports/report.pdf"`) {
		t.Fatalf("save: %d %s", got.Code, got.Body)
	}
	if stored("Documents/Exports/report.pdf") != "%PDF-1.7 render 2" {
		t.Fatalf("saved bytes: %q", stored("Documents/Exports/report.pdf"))
	}
	if got := post(target, "<html>two</html>"); got.Code != http.StatusConflict || !strings.Contains(got.Body.String(), "pdf_exists") || len(pages) != 2 {
		t.Fatalf("existing file: %d %s after %d renders", got.Code, got.Body, len(pages))
	}
	if got := post(target+"&overwrite=1", "<html>three</html>"); got.Code != http.StatusOK || stored("Documents/Exports/report.pdf") != "%PDF-1.7 render 3" {
		t.Fatalf("overwrite: %d %s", got.Code, got.Body)
	}
	if got := post("?path=Documents/Exports/report.docx", "<html>four</html>"); got.Code != http.StatusBadRequest || len(pages) != 3 {
		t.Fatalf("wrong extension: %d after %d renders", got.Code, len(pages))
	}
	if got := post("?path=../outside.pdf", "<html>five</html>"); got.Code != http.StatusBadRequest {
		t.Fatalf("path outside the workspace: %d %s", got.Code, got.Body)
	}

	// No renderer is its own answer; another failure is not.
	renderErr = fmt.Errorf("%w (browser: no binary)", tools.ErrHTMLPDFUnavailable)
	if got := post("", "<html>six</html>"); got.Code != http.StatusServiceUnavailable || !strings.Contains(got.Body.String(), `"error":"pdf_unavailable"`) {
		t.Fatalf("no renderer: %d %s", got.Code, got.Body)
	}
	renderErr = errors.New("render broke")
	if got := post("", "<html>seven</html>"); got.Code != http.StatusBadGateway || !strings.Contains(got.Body.String(), `"error":"pdf_failed"`) || strings.Contains(got.Body.String(), "render broke") {
		t.Fatalf("failed render: %d %s", got.Code, got.Body)
	}
}

func TestDesktopWriterPDFReadOnlyDesktopStillDownloads(t *testing.T) {
	s := newDesktopOfficeTestServer(t)
	previous := desktopWriterPDFRender
	defer func() { desktopWriterPDFRender = previous }()
	renders := 0
	desktopWriterPDFRender = func(ctx context.Context, cfg *config.Config, page string) ([]byte, error) {
		renders++
		return []byte("%PDF-1.7"), nil
	}
	// The server's own switch decides; the desktop service follows it on the next request.
	s.CfgMu.Lock()
	s.Cfg.VirtualDesktop.ReadOnly = true
	s.CfgMu.Unlock()
	handler := handleDesktopWriterPDF(s)
	rec := httptest.NewRecorder()
	handler(rec, httptest.NewRequest(http.MethodPost, "/api/desktop/office/pdf", strings.NewReader("<html>pages</html>")))
	if rec.Code != http.StatusOK || rec.Body.String() != "%PDF-1.7" {
		t.Fatalf("download on a read-only desktop: %d %s", rec.Code, rec.Body)
	}
	rec = httptest.NewRecorder()
	handler(rec, httptest.NewRequest(http.MethodPost, "/api/desktop/office/pdf?path=Documents/report.pdf", strings.NewReader("<html>pages</html>")))
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "desktop_readonly") || renders != 1 {
		t.Fatalf("save on a read-only desktop: %d %s after %d renders", rec.Code, rec.Body, renders)
	}
}
```

Run: `go test ./internal/server -run "DesktopWriterPDF" -count=1` — expected: a build failure, `undefined: desktopWriterPDFRender`.

- [ ] **Step 4: Write the endpoint**

Create `internal/server/desktop_writer_pdf.go`:

```go
package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"aurago/internal/desktop"
	"aurago/internal/tools"
)

// desktopWriterPDFRender is the renderer; a variable so that tests run without a browser.
var desktopWriterPDFRender = tools.RenderHTMLPDF

const desktopWriterPDFTimeout = 2 * time.Minute

// handleDesktopWriterPDF renders the Writer's packed print pages (one self-contained HTML page) to a PDF. Without
// ?path the response is the PDF. With it the PDF becomes a file in the workspace: create only, or over an existing
// file with overwrite=1. An existing file is refused before rendering, so the client can ask and send again.
func handleDesktopWriterPDF(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		target := r.URL.Query().Get("path")
		scope, operation := desktopScopeRead, desktopRead
		if target != "" {
			scope, operation = desktopScopeWrite, desktopWrite
		}
		if !requireDesktopOperation(s, w, r, scope, operation) {
			return
		}
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		svc, hub, err := s.getDesktopService(r.Context())
		if err != nil {
			jsonError(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		// Read the page before any refusal: a browser that is still uploading megabytes reports a refusal that
		// closes the connection as a network failure, not as the answer.
		page, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 50<<20))
		if err != nil {
			jsonError(w, "Document exceeds export limit", http.StatusRequestEntityTooLarge)
			return
		}
		if len(bytes.TrimSpace(page)) == 0 {
			jsonError(w, "Print pages required", http.StatusBadRequest)
			return
		}
		overwrite := r.URL.Query().Get("overwrite") == "1"
		if target != "" {
			if !strings.EqualFold(filepath.Ext(target), ".pdf") {
				jsonError(w, "PDF path required", http.StatusBadRequest)
				return
			}
			if _, _, err := svc.ReadFileBytes(r.Context(), target); err == nil {
				if !overwrite {
					jsonError(w, "pdf_exists", http.StatusConflict)
					return
				}
			} else if !errors.Is(err, os.ErrNotExist) {
				jsonError(w, err.Error(), http.StatusBadRequest)
				return
			}
		}
		ctx, cancel := context.WithTimeout(r.Context(), desktopWriterPDFTimeout)
		defer cancel()
		data, err := desktopWriterPDFRender(ctx, s.ConfigSnapshot(), string(page))
		if err != nil {
			if s.Logger != nil {
				s.Logger.Warn("Writer PDF was not rendered", "error", err)
			}
			if errors.Is(err, tools.ErrHTMLPDFUnavailable) {
				jsonError(w, "pdf_unavailable", http.StatusServiceUnavailable)
				return
			}
			jsonError(w, "pdf_failed", http.StatusBadGateway)
			return
		}
		if target == "" {
			w.Header().Set("Content-Type", "application/pdf")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("Content-Disposition", `attachment; filename="document.pdf"`)
			w.Write(data)
			return
		}
		entry, err := svc.WriteFileBytesConditional(r.Context(), target, data, desktop.SourceUser, func(current desktop.FileWriteState) error {
			if current.Exists && !overwrite {
				return officeConflictError{message: "pdf_exists"}
			}
			return nil
		})
		if err != nil {
			code := http.StatusBadRequest
			if isOfficeConflictError(err) {
				code = http.StatusConflict
			}
			jsonError(w, err.Error(), code)
			return
		}
		broadcastDesktopEvent(s, hub, desktop.Event{Type: "desktop_changed", Payload: map[string]interface{}{"operation": "write_file", "path": entry.Path}, CreatedAt: time.Now().UTC()})
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok", "path": entry.Path})
	}
}
```

In `internal/server/server_routes.go`, replace

```go
	mux.HandleFunc("/api/desktop/office/export", handleDesktopOfficeExport(s))
```
with
```go
	mux.HandleFunc("/api/desktop/office/export", handleDesktopOfficeExport(s))
	mux.HandleFunc("/api/desktop/office/pdf", handleDesktopWriterPDF(s))
```

- [ ] **Step 5: Run the tests and commit**

Run: `go test ./internal/server -run "DesktopWriterPDF" -count=1 -v` — expected: both tests pass. Run: `go test ./internal/tools -run "(?i)(RenderHTMLPDF|RestrictRemoteContent)" -count=1` and `go vet ./internal/tools ./internal/server` — expected: `ok` and no findings. Run: `go test ./internal/server -run "(?i)(desktop|route|scope|security)" -count=1` — expected: `ok`.

The controller runs `AURAGO_RUN_BROWSER_SMOKE=1 go test ./internal/tools -run "RenderHTMLPDFBrowser" -count=1 -v`.

Commit as `feat(server): render the Writer's print pages to a PDF`.

---

### Task 5: The two menu entries

**Files:**
- Modify: `ui/js/desktop/apps/writer-commands.js`, `ui/js/desktop/apps/writer-menus.js`, `ui/js/desktop/apps/writer.js`, `scripts/test-writer-menus.mjs`, `scripts/test-writer-commands.mjs`, `ui/desktop_writer_browser_test.go`, `ui/desktop_writer_templates_test.go`
- Create: `ui/desktop_writer_pdf_test.go`

- [ ] **Step 1: Make the node tests ask for the entry**

In `scripts/test-writer-menus.mjs`, replace

```js
assert.ok(fileIds.includes('docx,pdf,html,md,txt'), 'the PDF stands with the other exports');
```
with
```js
assert.ok(fileIds.includes('docx,pdf,pdfSave,html,md,txt'), 'both PDF entries stand with the other exports');
```

In `scripts/test-writer-commands.mjs`, replace

```js
'zoom100', 'table', 'link', 'saveTemplate', 'pdf']) assert.equal(commands.state(id).enabled, false, id);
```
with
```js
'zoom100', 'table', 'link', 'saveTemplate', 'pdf', 'pdfSave']) assert.equal(commands.state(id).enabled, false, id);
```

Run: `npm run test:writer` — expected: the menus test fails with `both PDF entries stand with the other exports`.

- [ ] **Step 2: The command and the menu**

In `ui/js/desktop/apps/writer-commands.js`, replace

```js
        {id: 'pdf', label: 'pdf', icon: 'file-text', needsEditor: true},
```
with
```js
        {id: 'pdf', label: 'pdf', icon: 'file-text', needsEditor: true},
        {id: 'pdfSave', label: 'pdf_save', icon: 'file-text', write: true, needsEditor: true},
```

In `ui/js/desktop/apps/writer-menus.js`, replace

```js
                item('docx'),item('pdf'),item('html'),item('md'),item('txt'),SEP,item('pageSetup'),item('print')]},
```
with
```js
                item('docx'),item('pdf'),item('pdfSave'),item('html'),item('md'),item('txt'),SEP,item('pageSetup'),item('print')]},
```

- [ ] **Step 3: The actions**

In `ui/js/desktop/apps/writer.js`, replace

```js
        let editor, lib, queue, path = ctx.path || newPath(), etag = null, generation = 0, disposed = false, fileOperation = false;
```
with
```js
        let editor, lib, queue, path = ctx.path || newPath(), etag = null, generation = 0, disposed = false, fileOperation = false, pdfBusy = false;
```

Replace

```js
                    // The browser's print dialog writes the PDF; the notice says which destination to pick there.
                    case 'pdf':if(editor){notice(tr('pdf_hint'));await panels.print(await editor.save(),lib,documentFonts,basename(path).replace(/\.docx$/i,''));}break;
```
with
```js
                    case 'pdf':await pdfDownload();break;
                    case 'pdfSave':await pdfSave();break;
```

Replace

```js
            } catch(error){if(action==='print' || action==='pdf')notice(ctx.t('desktop.print_failed'),true);else fail(error);}
        }
        function download(bytes,type,name) {
            const url=URL.createObjectURL(new Blob([bytes],{type})),link=document.createElement('a');
            link.href=url;link.download=name;link.click();setTimeout(()=>URL.revokeObjectURL(url),1000);
        }
```
with
```js
            } catch(error){if(action==='print')notice(ctx.t('desktop.print_failed'),true);else fail(error);}
        }
        function download(bytes,type,name) {
            const url=URL.createObjectURL(new Blob([bytes],{type})),link=document.createElement('a');
            link.href=url;link.download=name;link.click();setTimeout(()=>URL.revokeObjectURL(url),1000);
        }
        // The server renders the document's print pages to a PDF. With a target the PDF becomes that file in the
        // workspace and an existing file is asked about first; without one the response carries the PDF. null means
        // the user kept the existing file.
        async function renderPdf(target) {
            const html=await panels.pdf(await editor.save(),lib,documentFonts,basename(path).replace(/\.docx$/i,''));
            const send=overwrite=>fetch('/api/desktop/office/pdf'+(target?'?path='+encodeURIComponent(target)+(overwrite?'&overwrite=1':''):''),{method:'POST',credentials:'same-origin',headers:{'Content-Type':'text/html; charset=utf-8'},body:html,signal:documentLife.signal});
            let response=await send(false);
            if(target && response.status===409) {
                await response.body?.cancel().catch(()=>{});
                const name=basename(target);
                if(!await ctx.confirmDialog(ctx.t?.('desktop.file_dialog_overwrite_title') || tr('pdf_save'), ctx.t?.('desktop.file_dialog_overwrite_message',{name}) || name))return null;
                response=await send(true);
            }
            return responseOK(response);
        }
        const pdfIdle=()=>{if(find('[data-notice-text]').textContent===tr('pdf_working'))notice('');};
        async function pdfDownload() {
            if(!editor || pdfBusy)return;
            pdfBusy=true;notice(tr('pdf_working'));
            try {
                const response=await renderPdf();
                download(await response.arrayBuffer(),'application/pdf',window.WriterPdf.fileName(path));
                pdfIdle();
            } catch(error) {
                if(error?.name==='AbortError' || disposed)return;
                if(error.message!=='pdf_unavailable'){notice(tr('pdf_failed'),true);return;}
                // No renderer on the server: the browser's print dialog writes the PDF, and the notice says how.
                notice(tr('pdf_hint'));
                try {await panels.print(await editor.save(),lib,documentFonts,basename(path).replace(/\.docx$/i,''));}
                catch(_) {notice(ctx.t('desktop.print_failed'),true);}
            } finally {pdfBusy=false;}
        }
        async function pdfSave() {
            if(!editor || ctx.readonly || pdfBusy)return;
            const sourceEditor=editor;
            const choice=await ctx.saveFileDialog({title:tr('pdf_save'),initialPath:path.split('/').slice(0,-1).join('/'),defaultName:window.WriterPdf.fileName(path),defaultExtension:'.pdf',filters:[{label:'PDF',extensions:['.pdf']}]});
            if(!choice?.path || choice.canceled || editor!==sourceEditor || disposed || pdfBusy)return;
            pdfBusy=true;notice(tr('pdf_working'));
            try {
                const response=await renderPdf(choice.path);
                if(!response){pdfIdle();return;}
                await response.body?.cancel().catch(()=>{});
                notice(tr('pdf_saved'));ctx.loadBootstrap?.();
            } catch(error) {
                if(error?.name==='AbortError' || disposed)return;
                notice(error.message==='pdf_unavailable'?tr('pdf_unavailable'):tr('pdf_failed'),true);
            } finally {pdfBusy=false;}
        }
```

Run: `npm run test:writer` — expected: nine lines, all passing.

- [ ] **Step 4: The browser fixtures**

In `ui/desktop_writer_browser_test.go`, replace

```go
	mux.HandleFunc("/writer-app-fixture", func(w http.ResponseWriter, r *http.Request) {
```
with
```go
	// This fixture has no PDF renderer: Download as PDF falls back to the print dialog. Like the server it reads
	// the page first; an answer to a half-sent upload reaches the browser as a network failure.
	mux.HandleFunc("/api/desktop/office/pdf", func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprint(w, `{"error":"pdf_unavailable"}`)
	})
	mux.HandleFunc("/writer-app-fixture", func(w http.ResponseWriter, r *http.Request) {
```

Replace

```go
        // Save as PDF is the same print job with a hint about the destination and the document's name.
        await app.act('pdf');
        if(!window.printProbe)throw Error('Save as PDF printed nothing: '+document.querySelector('[data-notice]').textContent);
        if(document.querySelector('[data-notice-text]').textContent!==labels['desktop.writer_pdf_hint'])throw Error('Save as PDF says '+document.querySelector('[data-notice-text]').textContent);
```
with
```go
        // Without a renderer on the server, Download as PDF is the same print job with a hint about the destination
        // and the document's name.
        await app.act('pdf');
        if(!window.printProbe)throw Error('Download as PDF printed nothing: '+document.querySelector('[data-notice]').textContent);
        if(document.querySelector('[data-notice-text]').textContent!==labels['desktop.writer_pdf_hint'])throw Error('Download as PDF says '+document.querySelector('[data-notice-text]').textContent);
```

Replace

```go
	mux.HandleFunc("/writer-file-ops-fixture", func(w http.ResponseWriter, r *http.Request) {
```
with
```go
	// The PDF endpoint of the fixture keeps the last page it was sent and answers like the server: 409 for an
	// existing file without overwrite, 503 while /test/pdf-down is on.
	var pdfDown bool
	var pdfPage string
	mux.HandleFunc("/api/desktop/office/pdf", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		target, overwrite := r.URL.Query().Get("path"), r.URL.Query().Get("overwrite") == "1"
		mu.Lock()
		defer mu.Unlock()
		status, code := http.StatusOK, ""
		if _, exists := files[target]; pdfDown {
			status, code = http.StatusServiceUnavailable, "pdf_unavailable"
		} else if target != "" && exists && !overwrite {
			status, code = http.StatusConflict, "pdf_exists"
		}
		traces = append(traces, map[string]any{"method": "PDF", "url": r.URL.String(), "status": status, "bytes": len(body)})
		if status != http.StatusOK {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(status)
			fmt.Fprintf(w, `{"error":%q}`, code)
			return
		}
		pdfPage = string(body)
		if target == "" {
			w.Header().Set("Content-Type", "application/pdf")
			w.Write([]byte("%PDF-1.7 fixture"))
			return
		}
		files[target] = []byte("%PDF-1.7 fixture")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status":"ok","path":%q}`, target)
	})
	mux.HandleFunc("/test/pdf-down", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		pdfDown = r.URL.Query().Get("on") == "1"
		mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/test/pdf-page", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprint(w, pdfPage)
	})
	mux.HandleFunc("/writer-file-ops-fixture", func(w http.ResponseWriter, r *http.Request) {
```

Replace

```go
	verifyWriterRecent(t, page)
	verifyWriterTemplates(t, page)
```
with
```go
	verifyWriterRecent(t, page)
	verifyWriterTemplates(t, page)
	verifyWriterPdf(t, page)
```

The file-ops fixture has no print runtime so far, and the PDF needs the print frame. In its page (the one whose context has `confirmDialog:async()=>window.confirmAnswer!==false`), replace

```go
<script>window.ready=(async()=>{const labels=await(await fetch('/lang/desktop/en.json')).json();window.writerContext={t:(key,params)=>String(labels[key]||key).replace(/\{\{(\w+)\}\}/g,(_,x)=>params?.[x]??''),promptDialog:async(title,value)=>window.promptAnswer??value,confirmDialog:async()=>window.confirmAnswer!==false,
```
with
```go
<script src="/js/desktop/core/print-runtime.js"></script>
<script>window.ready=(async()=>{const labels=await(await fetch('/lang/desktop/en.json')).json();window.writerContext={t:(key,params)=>String(labels[key]||key).replace(/\{\{(\w+)\}\}/g,(_,x)=>params?.[x]??''),promptDialog:async(title,value)=>window.promptAnswer??value,confirmDialog:async()=>window.confirmAnswer!==false,
```

In `ui/desktop_writer_templates_test.go`, replace

```go
'saveAs,saveTemplate,-','docx,pdf,html'])
```
with
```go
'saveAs,saveTemplate,-','docx,pdf,pdfSave,html'])
```

- [ ] **Step 5: The browser check**

Create `ui/desktop_writer_pdf_test.go`:

```go
package ui

import (
	"bytes"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

// verifyWriterPdf runs after verifyWriterTemplates in the file-ops fixture. The fixture's PDF endpoint keeps the
// page it was sent; this check renders that page the way the server does, in a blank page under the same policy.
func verifyWriterPdf(t *testing.T, page *rod.Page) {
	t.Helper()
	page.MustEval(`async()=>{
        const app=WriterApp.instances.get('test'),wait=ms=>new Promise(r=>setTimeout(r,ms));
        const until=async(test,what)=>{for(let i=0;i<160;i++){if(await test())return;await wait(50);}throw Error('timed out: '+what);};
        const labels=await(await fetch('/lang/desktop/en.json')).json(),text=key=>labels['desktop.writer_'+key];
        const banner=()=>document.querySelector('[data-notice]'),said=()=>document.querySelector('[data-notice-text]').textContent;
        const sent=async()=>(await (await fetch('/test/traces')).json()).filter(item=>item.method==='PDF');
        window.confirmAnswer=true;
        // A new document with a picture and a landscape section.
        await app.act('blank');
        await until(()=>!!app.editor&&document.querySelector('[data-loading]').hidden,'the blank document');
        const editor=app.editor,run=command=>{const result=editor.exec(command);if(!result.ok)throw Error(JSON.stringify({command:command.type,result}));};
        const toEnd=()=>{const last=editor.surface.session.paragraphIds().at(-1),end={paragraphId:last,offset:editor.query({type:'paragraphs'}).at(-1).text.length};run({type:'setSelection',range:{anchor:end,head:end}});};
        run({type:'paste',text:'Server PDF body'});
        const canvas=document.createElement('canvas');canvas.width=64;canvas.height=48;canvas.getContext('2d').fillRect(0,0,64,48);
        const blob=await new Promise(r=>canvas.toBlob(r,'image/png')),transfer=new DataTransfer();transfer.items.add(new File([blob],'pdf.png',{type:'image/png'}));
        const input=document.querySelector('[data-image]');input.files=transfer.files;input.dispatchEvent(new Event('change',{bubbles:true}));
        await until(()=>[...editor.surface.session.currentPackage().partBytes.keys()].some(part=>part.includes('/media/')),'the picture');
        toEnd();
        for(const command of [{type:'insertBreak',kind:'section'},{type:'paste',text:'Wide page'},{type:'setPageSetup',orientation:'landscape',scope:'section'}])run(command);
        const name=app.path.split('/').pop().replace(/\.docx$/,'')+'.pdf';

        // Download as PDF: one request without a path, the file saved under the document's name, nothing left behind.
        const clicks=[],click=HTMLAnchorElement.prototype.click;
        HTMLAnchorElement.prototype.click=function(){if(this.download)clicks.push({name:this.download,href:this.href});else click.call(this);};
        let before=(await sent()).length;
        await app.act('pdf');
        HTMLAnchorElement.prototype.click=click;
        let all=await sent();
        if(all.length!==before+1||all.at(-1).status!==200||all.at(-1).url.includes('path='))throw Error('download sent '+JSON.stringify(all.slice(before))+'; notice: '+said());
        if(clicks.length!==1||clicks[0].name!==name)throw Error('download saved '+JSON.stringify(clicks)+' instead of '+name);
        if(!(await (await fetch(clicks[0].href)).text()).startsWith('%PDF-'))throw Error('the downloaded file is not the PDF');
        if(!banner().hidden)throw Error('the notice stayed: '+said());
        if(document.querySelector('iframe.writer-print-frame'))throw Error('the print frame stayed');

        // Save as PDF in the workspace: a new file, then the question about the existing one.
        window.saveTarget='Documents/Exports/report.pdf';
        await app.act('pdfSave');
        all=await sent();
        if(all.at(-1).status!==200||!decodeURIComponent(all.at(-1).url).includes('path=Documents/Exports/report.pdf')||all.at(-1).url.includes('overwrite'))throw Error('save sent '+JSON.stringify(all.at(-1)));
        if(said()!==text('pdf_saved')||banner().dataset.error!=='false')throw Error('save says '+said());
        window.confirmAnswer=false;before=all.length;
        await app.act('pdfSave');
        all=await sent();
        if(all.length!==before+1||all.at(-1).status!==409)throw Error('a refused overwrite sent '+JSON.stringify(all.slice(before)));
        if(!banner().hidden)throw Error('a refused overwrite says '+said());
        window.confirmAnswer=true;before=all.length;
        await app.act('pdfSave');
        all=await sent();
        if(all.length!==before+2||all.at(-2).status!==409||all.at(-1).status!==200||!all.at(-1).url.includes('overwrite=1'))throw Error('an overwrite sent '+JSON.stringify(all.slice(before)));
        if(said()!==text('pdf_saved'))throw Error('an overwrite says '+said());

        // Without a renderer the workspace entry says so and the download entry opens the print dialog with its hint.
        await fetch('/test/pdf-down?on=1',{method:'POST'});
        await app.act('pdfSave');
        if(said()!==text('pdf_unavailable')||banner().dataset.error!=='true')throw Error('no renderer says '+said());
        let printed=0;
        const observer=new MutationObserver(records=>{for(const record of records)for(const frame of record.addedNodes)if(frame.tagName==='IFRAME'){
            const hook=()=>{frame.contentWindow.print=()=>{printed++;};};hook();frame.addEventListener('load',hook);
        }});
        observer.observe(document.body,{childList:true});
        await app.act('pdf');observer.disconnect();
        await fetch('/test/pdf-down?on=0',{method:'POST'});
        if(printed!==1||said()!==text('pdf_hint'))throw Error('the fallback printed '+printed+' times and says '+said());
    }`)

	packed := page.MustEval(`async()=>(await fetch('/test/pdf-page')).text()`).Str()
	for _, banned := range []string{"<script", "<link", "blob:", `class="docx-drawing docx-drawing-placeholder"`, `id="source"`} {
		if strings.Contains(packed, banned) {
			t.Fatalf("the packed page contains %q", banned)
		}
	}
	for _, needed := range []string{`@font-face{font-family:"calibri";font-weight:400;font-style:normal;src:url(data:font/ttf;base64,`, `src="data:image/png;base64,`, "@page writer-sheet-0{", "@page writer-sheet-1{", "<title>"} {
		if !strings.Contains(packed, needed) {
			t.Fatalf("the packed page lacks %q", needed)
		}
	}

	fresh := page.Browser().MustPage("about:blank")
	defer fresh.Close()
	policy := `<meta http-equiv="Content-Security-Policy" content="default-src 'none'; img-src data:; style-src 'unsafe-inline'; font-src data:">`
	if err := fresh.SetDocumentContent(strings.Replace(packed, "<head>", "<head>"+policy, 1)); err != nil {
		t.Fatal(err)
	}
	state := fresh.MustEval(`async()=>{
        await document.fonts.ready;
        await Promise.all([...document.images].map(image=>image.decode().catch(()=>{})));
        return {pages:document.querySelectorAll('.docx-page').length,faces:[...document.fonts].filter(face=>face.status==='loaded').length,images:[...document.images].map(image=>image.naturalWidth).join(),text:document.body.innerText};
    }`)
	if state.Get("pages").Int() != 2 || state.Get("faces").Int() < 1 || state.Get("images").Str() != "64" {
		t.Fatalf("the packed page renders as %s", state.JSON("", ""))
	}
	if text := state.Get("text").Str(); !strings.Contains(text, "Server PDF body") || !strings.Contains(text, "Wide page") {
		t.Fatalf("the packed page reads %q", text)
	}
	pdf, err := (proto.PagePrintToPDF{PrintBackground: true, PreferCSSPageSize: true}).Call(fresh)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.MkdirAll("../reports/autor", 0755); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile("../reports/autor/server.pdf", pdf.Data, 0644); err != nil {
		t.Fatal(err)
	}
	portrait, landscape := 0, 0
	for _, box := range regexp.MustCompile(`/MediaBox \[0 0 ([0-9.]+) ([0-9.]+)\]`).FindAllSubmatch(pdf.Data, -1) {
		width, _ := strconv.ParseFloat(string(box[1]), 64)
		height, _ := strconv.ParseFloat(string(box[2]), 64)
		if width > height {
			landscape++
		} else {
			portrait++
		}
	}
	if portrait != 1 || landscape != 1 {
		t.Fatalf("the rendered PDF has %d portrait and %d landscape pages instead of 1 and 1", portrait, landscape)
	}
	if !regexp.MustCompile(`/FontName\s*/[A-Z]+\+Carlito`).Match(pdf.Data) {
		t.Fatal("the rendered PDF does not embed the document's font")
	}
	if !bytes.Contains(pdf.Data, []byte("/Subtype /Image")) {
		t.Fatal("the rendered PDF has no picture")
	}
}
```

- [ ] **Step 6: Check and commit**

Run: `npm run test:writer`, `npm run check:ui`, `go vet ./ui` and `go test ./ui -run "(?i)(windowmenu|contextmenu|materials|office|writer|virtualdesktop|lifecycle|i18n|feeling|print)" -count=1` — expected: all pass.

The controller runs `AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run "TestDesktop(Writer|QR)" -count=1 -timeout 30m -v` and looks at `reports/autor/server.pdf` and `reports/autor/print.pdf`.

Commit as `feat(writer): Download as PDF and Save as PDF in the workspace`.

---

### Task 6: Documentation and DOX pass

**Files:**
- Modify: `ui/js/desktop/apps/AGENTS.md`, `internal/server/AGENTS.md`, `internal/tools/AGENTS.md`, `documentation/autor.md`

- [ ] **Step 1: `ui/js/desktop/apps/AGENTS.md`**

In the Writer section, after the sentence that ends with `so sections of different formats print right;` add:

```text
  `writer-pdf.js` packs those painted pages for the server's PDF renderer. `paint` in
  `writer-panels.js` is the one way to the print frame, for Print and for the PDF: the scratch
  editor lives in `#source` and decodes the pictures, the pages are painted into `#pages`, where a
  picture is only a placeholder, so `graft` copies the decoded drawings over by
  `data-drawing-node-id` and `inline` turns their `blob:` addresses into `data:` ones before the
  scratch editor goes. Never paint into the scratch editor's own container. `pack` returns one
  HTML page without script or link: the engine stylesheet inline and an `@font-face` only for the
  faces the pages use, under the family name the pages ask for, from the file the engine laid the
  text out with (`source`: own file or substitution). `pdf` (Download as PDF) posts that page to
  `/api/desktop/office/pdf` and falls back to the print dialog with `pdf_hint` when the server
  answers `pdf_unavailable`; `pdfSave` posts it with `?path=` and asks before `overwrite=1`;
```

- [ ] **Step 2: `internal/server/AGENTS.md` and `internal/tools/AGENTS.md`**

Add to the local contracts of `internal/server/AGENTS.md`:

```text
- `POST /api/desktop/office/pdf` renders the Writer's packed print pages (one self-contained HTML page, at most 50 MiB) through `tools.RenderHTMLPDF`. Without `path` it needs Desktop read and returns the PDF, also on a read-only desktop. With `path` (must end in `.pdf`) it needs Desktop write, refuses an existing file with 409 `pdf_exists` before rendering unless `overwrite=1`, and writes through `WriteFileBytesConditional`. No renderer is 503 `pdf_unavailable`, any other render failure 502 `pdf_failed`; the cause goes to the log, never to the client. Verify `TestDesktopWriterPDF*`.
```

Add to the local contracts of `internal/tools/AGENTS.md`:

```text
- `RenderHTMLPDF` renders caller-supplied HTML as data: Gotenberg (`RenderReportHTMLPDF`) when it is the enabled document backend, the built-in browser from `security.LaunchEgressBrowser` otherwise and when Gotenberg fails. Both get the page behind `restrictRemoteContent`; the browser path also turns script execution off, sets the page as document content (never a URL) and prints with `PreferCSSPageSize`. Failure of every renderer is `ErrHTMLPDFUnavailable`; a cancelled context is returned as such. Verify `TestRenderHTMLPDF*` (the browser test needs `AURAGO_RUN_BROWSER_SMOKE=1`).
```

- [ ] **Step 3: `documentation/autor.md`**

In the section "Templates and PDF", replace the paragraph that starts with `**File > Save as PDF…** opens the browser's print dialog` by:

```text
**File > Download as PDF** lets the server render the document and saves the
PDF under the document's name. **File > Save as PDF in the workspace…** writes
the PDF into a folder of the workspace and asks before it replaces a file.
Both use the pages of the print path, so the PDF has the fonts, pictures and
page formats of the print. The server renders with Gotenberg when it is the
document backend, and with its built-in Chromium otherwise. When neither is
available, Download as PDF opens the browser's print dialog; choose "Save as
PDF" as the destination there. **Print…** hands the document's name to the
print job. Every page prints in the format of its own section, so a landscape
section inside a portrait document stays landscape.
```

In "Build and verification", change `and the templates (`scripts/test-writer-templates.mjs`).` to `the templates (`scripts/test-writer-templates.mjs`) and the PDF packer (`scripts/test-writer-pdf.mjs`).`, and add a line for the server tests: `go test ./internal/server -run DesktopWriterPDF` and `go test ./internal/tools -run RenderHTMLPDF`.

- [ ] **Step 4: DOX pass and commit**

Re-read the `AGENTS.md` chain of every touched path and check that no statement about Writer printing or the PDF entry is stale (`git grep -n "Save as PDF" -- '*.md'`). Commit as `docs(writer): one-click PDF from the server`.
