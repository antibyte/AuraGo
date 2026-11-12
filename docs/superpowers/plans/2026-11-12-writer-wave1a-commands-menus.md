# Writer Wave 1a — commands, menus, dialogs, zoom: implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give Writer complete window menus, the missing everyday commands, real dialogs for link, go to page, word count and table size, zoom presets with Ctrl+wheel, and four toolbar additions, all fed by the command registry from Wave 0.

**Architecture:** The registry (`writer-commands.js`) learns two things: commands that are plain engine commands without a toolbar slot, and a checked state for menus. `writer-menus.js` builds the window menus from it and rebuilds them only when something a menu shows has changed. `writer-dialogs.js` holds the dialogs; they all go through the shared form dialog, which gains read-only rows, a size grid and a third button. `writer.js` keeps lifecycle, file operations and the app actions.

**Tech stack:** Vanilla JS classic scripts (no bundler), `@docx-editor.dev/core` 2.29.0 behind `lib`, native `<dialog>`, node `vm` tests, Go + rod browser tests.

Roadmap: `docs/superpowers/specs/2026-11-12-writer-office-roadmap-design.md`, section "Wave 1 — Interaction". This plan is part 1a of three. Part 1b (right-click menu, contextual bar) and part 1c (start state, recent documents) get their own plans.

---

## Read this first

**Scope of 1a.** In: registry extension, complete menus (File, Edit, View, Insert, Format, Table, Tools, Agent), clipboard commands in the Edit menu, dialogs (link insert/edit/remove, go to page, word count, shortcut overview, table size with grid), zoom presets and Ctrl+wheel, toolbar additions (clear formatting, line spacing, link, table, image), shortcuts Ctrl+K and Ctrl+G. Out: right-click menu, contextual bar, start state, recent documents, templates.

**Deliberate differences from the roadmap table, decided while planning:**

- Superscript and subscript are menu items (Format > Text), not toolbar buttons. The chosen concept is the Google-Docs layout, which keeps them in the menu, and the bar has no room to spare.
- The format painter is two menu commands, "Copy formatting" and "Paste formatting". The engine's one-click painter is armed through chrome state that its public API does not expose (`commandForSlot('format.painter')` is `null`).
- "Simple markup" is not offered. The review module Writer loads declares only `all-markup`, `proposed` and `original`.
- "Whole page" zoom is computed by Writer. The engine's only fit target is `pageWidth`.

**Engine facts this plan relies on** (verified against `node_modules/@docx-editor.dev/core/dist/*.d.ts` and by calling `commandForSlot` in node):

| Slot | Command |
| --- | --- |
| `script.super` / `script.sub` | `{type:'toggleMark',mark:'superscript'}` / `'subscript'` |
| `format.clear` | `{type:'clearFormatting'}` |
| `review.paragraphMarks` | `{type:'toggleParagraphMarks'}`; state in `snapshot().showParagraphMarks` |
| `review.allMarkup` / `review.noMarkup` / `review.original` | `{type:'setReviewDisplayMode',mode:'all-markup'}` / `'proposed'` / `'original'`; state in `snapshot().reviewDisplayMode` |
| `review.previousChange` / `review.nextChange` | `{type:'navigateReviewChange',direction:'previous'}` / `'next'` |
| `insert.pageBreak`, `insert.sectionBreakNextPage`, `insert.sectionBreakContinuous` | `{type:'insertBreak',kind:'page'}` / `'section'` / `'sectionContinuous'` |
| `insert.footnote` / `insert.endnote` | `{type:'insertNote',noteKind:'footnote'}` / `'endnote'` |
| `insert.toc`, `insert.textBox`, `insert.pageXofY` | `{type:'insertToc'}`, `{type:'insertTextBox'}`, `{type:'insertPageField',field:'PAGE_X_OF_Y'}` |
| `list.lineSpacing` (value slot) | `commandForSlotValue(slot, 1.5)` gives `{type:'setLineSpacing',rule:'multiple',value:1.5}` |

Commands without a slot, with their exact shapes: `{type:'selectAll'}`, `{type:'copy'}`, `{type:'cut'}`, `{type:'paste',text,html?}`, `{type:'pasteWithoutFormatting',text}`, `{type:'copyFormatting'}`, `{type:'pasteFormatting'}`, `{type:'insertRow',where:'above'|'below'}`, `{type:'insertColumn',where:'left'|'right'}`, `{type:'deleteRow'}`, `{type:'deleteColumn'}`, `{type:'deleteTable'}`, `{type:'insertHyperlink',href,text?}`, `{type:'removeHyperlink'}`, `{type:'setEditingMode',mode}`. `editor.query({type:'hyperlinkAt'})` returns `{href, range}` or `null`. Its `range` is the whole paragraph, and neither hyperlink command takes a `target`: with the caret in a link, `insertHyperlink` replaces the address in place and `removeHyperlink` unlinks, both keeping the text (probed in the browser, 2026-11-12).

**Not verified at runtime, so the browser checks in this plan are the proof.** If one of these fails, stop and report the thrown message; do not weaken the check:

- `copy` and `cut` writing the system clipboard from a menu click.
- `editor.focus()` returning the caret after a menu command.
- The toolbar still fitting a 1366 px window after Task 6. If it does not, Task 6 says what to drop.

**Shell facts** (`ui/js/desktop/core/menus-and-routing.js`):

- A menu item is `{id, label|labelKey, icon, shortcut, shortcutHint, disabled, checked, action}`, `{type:'separator'}`, or a submenu `{id, label, items}`. `disabled` and `checked` are read when `setWindowMenus` runs, not when a menu opens. Calling `setWindowMenus` again rebuilds that window's bar and closes an open menu.
- `shortcut` is drawn and also dispatched by the shell when the key event's target is not editable. `shortcutHint` is only drawn. Clipboard shortcuts must be hints: a dispatched Ctrl+C would steal the key from text selected in a side panel.
- `ctx.showContextMenu(x, y, items)` takes the same item shape. An item without `icon` gets a default tool icon there, so pass one.

**Repo rules:**

- Every user-visible string exists in all 16 files under `ui/lang/desktop/` (`cs da de el en es fr hi it ja nl no pl pt sv zh`). `scripts/test-writer-commands.mjs` scans every registry label and every literal `tr('key')` in `ui/js/desktop/apps/writer*.js`; write `tr('key')` with a literal key so the scan sees it.
- Every file under `ui/js/desktop` stays below 1100 lines. No Internet assets, no new dependencies.
- `ui/desktop_office_app_markers_test.go` pins these strings in `writer.js`: `window.WriterApp={render,dispose,instances}`, `queue?.dispose()`, `dispose(windowId);`, `editor?.destroy()`, `/api/desktop/office/document`.
- Toolbar controls carry `data-tool`; `[data-command]` belongs to the panels.
- Never test `editingMode==='suggesting'` to gate a command; ask `commands.blocked(type)`, `commands.can(command)` or `commands.tracking()`.
- `ui/js/desktop/apps/writer-session.js` is shared with other apps. Do not touch it.
- `docs/` is git-ignored; plans and specs are added with `git add -f`. Temporary scripts live outside the repo.
- GitNexus: the index is hundreds of commits behind and cannot index worktrees. Controller's impact analysis for `writer.js` and `writer-panels.js`: UNKNOWN, callers confirmed by text search (`window.WriterApp` is used by the shell loader and the Go tests; the state object by the Writer modules). End each commit body with: `GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.`

**Environment (Windows):**

- Work in a worktree. It needs a real copy of `node_modules` (`robocopy <main>\node_modules <worktree>\node_modules /E /MT:16 /NFL /NDL /NJH /NP`); a junction makes `npm run check:ui` fail.
- Browser tests: `AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run "TestDesktop(Writer|QR)" -count=1 -timeout 30m -v`, from PowerShell with the sandbox off. Delete `reports\autor` first, or the test panics with "Access is denied" on a screenshot of the previous run. Implementer subagents cannot start Chrome; the controller runs these.
- Static tests: `npm run test:writer`, `npm run check:ui`, `go test ./ui -run "(?i)(materials|office|writer|virtualdesktop|lifecycle|i18n)" -count=1`.
- `TestDesktopCodeStudioRunUsesTerminalSession` fails on `main` for an unrelated reason. Ignore it.

## File structure

| File | Change | Responsibility |
| --- | --- | --- |
| `ui/js/desktop/apps/writer-commands.js` | modify | Registry: `command` entries, `glyph`, `hint`, `text`/`number` labels, `active`, `options`; new definitions; `menuItem` with real state |
| `ui/js/desktop/shared/form-dialog.js` | modify | Field types `static` and `grid`, `extraLabel` button, optional Cancel |
| `ui/css/desktop-form-dialog.css` | modify | Styles for the two field types, scroll for long forms |
| `ui/js/desktop/apps/writer-dialogs.js` | create | Table size, link, go to page, word count, shortcut overview |
| `ui/js/desktop/apps/writer-menus.js` | create | Window menu model and its state-driven rebuild |
| `ui/js/desktop/apps/writer-toolbar.js` | modify | Glyphs, groups, line-spacing select, state of action buttons |
| `ui/js/desktop/apps/writer.js` | modify | App actions (clipboard, zoom, tracking, dialogs), wiring, Ctrl+wheel |
| `ui/js/desktop/apps/writer-panels.js` | modify | Table and link buttons open the dialogs |
| `ui/css/desktop-app-writer.css` | modify | Width of the line-spacing select |
| `ui/js/desktop/core/module-loader.js` | modify | Two new scripts in the `writer` entry |
| `ui/lang/desktop/*.json` | modify | 38 new keys in 16 languages |
| `scripts/test-writer-commands.mjs` | replace | Registry rules |
| `scripts/test-writer-menus.mjs` | create | Menu model and rebuild rule |
| `scripts/test-writer-toolbar.mjs` | modify | New groups and glyphs |
| `ui/desktop_writer_menus_test.go` | create | Browser checks for dialogs, actions and menus |
| `ui/desktop_office_apps_test.go`, `ui/desktop_office_assets_test.go`, three fixtures | modify | Script lists and marker paths |
| `ui/js/desktop/apps/AGENTS.md`, `documentation/autor.md` | modify | Documentation |

---

### Task 1: Registry: plain engine commands, menu state, new definitions, language keys

**Files:**
- Replace: `ui/js/desktop/apps/writer-commands.js`
- Replace: `scripts/test-writer-commands.mjs`
- Modify: `ui/js/desktop/apps/writer-toolbar.js` (two lines), `scripts/test-writer-toolbar.mjs` (one line)
- Modify: `ui/lang/desktop/*.json` (script)

- [ ] **Step 1: Add the language keys**

Create this script outside the repo (the session scratchpad or the disposable folder) as `writer-wave1a-i18n.mjs`. It inserts each key at its sorted place in the `desktop.writer_` block and leaves every other line untouched.

```js
// Adds the Wave 1a keys to the sixteen desktop language files, each at its sorted place in the desktop.writer_ block.
// Usage: node writer-wave1a-i18n.mjs <repo root>
import {readFileSync, writeFileSync} from 'node:fs';
import path from 'node:path';

const root = process.argv[2] || process.cwd();
const LANGS = ['cs', 'da', 'de', 'el', 'en', 'es', 'fr', 'hi', 'it', 'ja', 'nl', 'no', 'pl', 'pt', 'sv', 'zh'];
const KEYS = {
    cut: ['Vyjmout', 'Klip', 'Ausschneiden', 'Αποκοπή', 'Cut', 'Cortar', 'Couper', 'काटें', 'Taglia', '切り取り', 'Knippen', 'Klipp ut', 'Wytnij', 'Cortar', 'Klipp ut', '剪切'],
    copy: ['Kopírovat', 'Kopiér', 'Kopieren', 'Αντιγραφή', 'Copy', 'Copiar', 'Copier', 'कॉपी करें', 'Copia', 'コピー', 'Kopiëren', 'Kopier', 'Kopiuj', 'Copiar', 'Kopiera', '复制'],
    paste: ['Vložit', 'Indsæt', 'Einfügen', 'Επικόλληση', 'Paste', 'Pegar', 'Coller', 'पेस्ट करें', 'Incolla', '貼り付け', 'Plakken', 'Lim inn', 'Wklej', 'Colar', 'Klistra in', '粘贴'],
    paste_plain: ['Vložit bez formátování', 'Indsæt uden formatering', 'Ohne Formatierung einfügen', 'Επικόλληση χωρίς μορφοποίηση', 'Paste without formatting', 'Pegar sin formato', 'Coller sans formatage', 'बिना फ़ॉर्मैटिंग के पेस्ट करें', 'Incolla senza formattazione', '書式なしで貼り付け', 'Plakken zonder opmaak', 'Lim inn uten formatering', 'Wklej bez formatowania', 'Colar sem formatação', 'Klistra in utan formatering', '粘贴为纯文本'],
    select_all: ['Vybrat vše', 'Markér alt', 'Alles auswählen', 'Επιλογή όλων', 'Select all', 'Seleccionar todo', 'Tout sélectionner', 'सभी चुनें', 'Seleziona tutto', 'すべて選択', 'Alles selecteren', 'Merk alt', 'Zaznacz wszystko', 'Selecionar tudo', 'Markera allt', '全选'],
    marks: ['Formátovací značky', 'Formateringstegn', 'Formatierungszeichen', 'Σύμβολα μορφοποίησης', 'Formatting marks', 'Marcas de formato', 'Marques de formatage', 'फ़ॉर्मैटिंग चिह्न', 'Segni di formattazione', '編集記号', 'Opmaaktekens', 'Formateringstegn', 'Znaki formatowania', 'Marcas de formatação', 'Formateringstecken', '格式标记'],
    markup: ['Zobrazení změn', 'Visning af ændringer', 'Änderungen anzeigen', 'Εμφάνιση αλλαγών', 'Show changes', 'Mostrar cambios', 'Afficher les modifications', 'बदलाव दिखाएँ', 'Mostra modifiche', '変更の表示', 'Wijzigingen weergeven', 'Vis endringer', 'Pokaż zmiany', 'Mostrar alterações', 'Visa ändringar', '显示修订'],
    markup_all: ['Všechny změny', 'Alle ændringer', 'Alle Änderungen', 'Όλες οι αλλαγές', 'All changes', 'Todos los cambios', 'Toutes les modifications', 'सभी बदलाव', 'Tutte le modifiche', 'すべての変更', 'Alle wijzigingen', 'Alle endringer', 'Wszystkie zmiany', 'Todas as alterações', 'Alla ändringar', '所有修订'],
    markup_final: ['Konečný text', 'Endelig tekst', 'Endgültiger Text', 'Τελικό κείμενο', 'Final text', 'Texto final', 'Texte final', 'अंतिम पाठ', 'Testo finale', '最終版', 'Definitieve tekst', 'Endelig tekst', 'Tekst końcowy', 'Texto final', 'Slutlig text', '最终文本'],
    zoom: ['Lupa', 'Zoom', 'Zoom', 'Ζουμ', 'Zoom', 'Zoom', 'Zoom', 'ज़ूम', 'Zoom', 'ズーム', 'Zoom', 'Zoom', 'Powiększenie', 'Zoom', 'Zoom', '缩放'],
    zoom_page: ['Celá stránka', 'Hele siden', 'Ganze Seite', 'Ολόκληρη σελίδα', 'Whole page', 'Página completa', 'Page entière', 'पूरा पृष्ठ', 'Pagina intera', 'ページ全体', 'Hele pagina', 'Hele siden', 'Cała strona', 'Página inteira', 'Hela sidan', '整页'],
    section_next: ['Konec oddílu (další stránka)', 'Sektionsskift (næste side)', 'Abschnittsumbruch (nächste Seite)', 'Αλλαγή ενότητας (επόμενη σελίδα)', 'Section break (next page)', 'Salto de sección (página siguiente)', 'Saut de section (page suivante)', 'सेक्शन ब्रेक (अगला पृष्ठ)', 'Interruzione di sezione (pagina successiva)', 'セクション区切り（次のページ）', 'Sectie-einde (volgende pagina)', 'Inndelingsskift (neste side)', 'Podział sekcji (następna strona)', 'Quebra de secção (página seguinte)', 'Avsnittsbrytning (nästa sida)', '分节符（下一页）'],
    section_continuous: ['Konec oddílu (průběžný)', 'Sektionsskift (fortløbende)', 'Abschnittsumbruch (fortlaufend)', 'Αλλαγή ενότητας (συνεχής)', 'Section break (continuous)', 'Salto de sección (continuo)', 'Saut de section (continu)', 'सेक्शन ब्रेक (निरंतर)', 'Interruzione di sezione (continua)', 'セクション区切り（現在の位置から）', 'Sectie-einde (doorlopend)', 'Inndelingsskift (løpende)', 'Podział sekcji (ciągły)', 'Quebra de secção (contínua)', 'Avsnittsbrytning (löpande)', '分节符（连续）'],
    endnote: ['Vysvětlivka', 'Slutnote', 'Endnote', 'Σημείωση τέλους', 'Endnote', 'Nota al final', 'Note de fin', 'एंडनोट', 'Nota di chiusura', '文末脚注', 'Eindnoot', 'Sluttnote', 'Przypis końcowy', 'Nota de fim', 'Slutnot', '尾注'],
    text_box: ['Textové pole', 'Tekstboks', 'Textfeld', 'Πλαίσιο κειμένου', 'Text box', 'Cuadro de texto', 'Zone de texte', 'टेक्स्ट बॉक्स', 'Casella di testo', 'テキストボックス', 'Tekstvak', 'Tekstboks', 'Pole tekstowe', 'Caixa de texto', 'Textruta', '文本框'],
    superscript: ['Horní index', 'Hævet skrift', 'Hochgestellt', 'Εκθέτης', 'Superscript', 'Superíndice', 'Exposant', 'सुपरस्क्रिप्ट', 'Apice', '上付き', 'Superscript', 'Hevet skrift', 'Indeks górny', 'Sobrescrito', 'Upphöjd', '上标'],
    subscript: ['Dolní index', 'Sænket skrift', 'Tiefgestellt', 'Δείκτης', 'Subscript', 'Subíndice', 'Indice', 'सबस्क्रिप्ट', 'Pedice', '下付き', 'Subscript', 'Senket skrift', 'Indeks dolny', 'Subscrito', 'Nedsänkt', '下标'],
    copy_format: ['Kopírovat formát', 'Kopiér formatering', 'Format kopieren', 'Αντιγραφή μορφοποίησης', 'Copy formatting', 'Copiar formato', 'Copier le formatage', 'फ़ॉर्मैटिंग कॉपी करें', 'Copia formattazione', '書式をコピー', 'Opmaak kopiëren', 'Kopier formatering', 'Kopiuj formatowanie', 'Copiar formatação', 'Kopiera formatering', '复制格式'],
    paste_format: ['Vložit formát', 'Indsæt formatering', 'Format übertragen', 'Επικόλληση μορφοποίησης', 'Paste formatting', 'Pegar formato', 'Coller le formatage', 'फ़ॉर्मैटिंग पेस्ट करें', 'Incolla formattazione', '書式を貼り付け', 'Opmaak plakken', 'Lim inn formatering', 'Wklej formatowanie', 'Colar formatação', 'Klistra in formatering', '粘贴格式'],
    row_above: ['Vložit řádek nad', 'Indsæt række over', 'Zeile oberhalb einfügen', 'Εισαγωγή γραμμής από πάνω', 'Insert row above', 'Insertar fila encima', 'Insérer une ligne au-dessus', 'ऊपर पंक्ति जोड़ें', 'Inserisci riga sopra', '上に行を挿入', 'Rij boven invoegen', 'Sett inn rad over', 'Wstaw wiersz powyżej', 'Inserir linha acima', 'Infoga rad ovanför', '在上方插入行'],
    row_below: ['Vložit řádek pod', 'Indsæt række under', 'Zeile unterhalb einfügen', 'Εισαγωγή γραμμής από κάτω', 'Insert row below', 'Insertar fila debajo', 'Insérer une ligne en dessous', 'नीचे पंक्ति जोड़ें', 'Inserisci riga sotto', '下に行を挿入', 'Rij onder invoegen', 'Sett inn rad under', 'Wstaw wiersz poniżej', 'Inserir linha abaixo', 'Infoga rad nedanför', '在下方插入行'],
    column_left: ['Vložit sloupec vlevo', 'Indsæt kolonne til venstre', 'Spalte links einfügen', 'Εισαγωγή στήλης αριστερά', 'Insert column left', 'Insertar columna a la izquierda', 'Insérer une colonne à gauche', 'बाईं ओर कॉलम जोड़ें', 'Inserisci colonna a sinistra', '左に列を挿入', 'Kolom links invoegen', 'Sett inn kolonne til venstre', 'Wstaw kolumnę z lewej', 'Inserir coluna à esquerda', 'Infoga kolumn till vänster', '在左侧插入列'],
    column_right: ['Vložit sloupec vpravo', 'Indsæt kolonne til højre', 'Spalte rechts einfügen', 'Εισαγωγή στήλης δεξιά', 'Insert column right', 'Insertar columna a la derecha', 'Insérer une colonne à droite', 'दाईं ओर कॉलम जोड़ें', 'Inserisci colonna a destra', '右に列を挿入', 'Kolom rechts invoegen', 'Sett inn kolonne til høyre', 'Wstaw kolumnę z prawej', 'Inserir coluna à direita', 'Infoga kolumn till höger', '在右侧插入列'],
    delete_row: ['Odstranit řádek', 'Slet række', 'Zeile löschen', 'Διαγραφή γραμμής', 'Delete row', 'Eliminar fila', 'Supprimer la ligne', 'पंक्ति हटाएँ', 'Elimina riga', '行を削除', 'Rij verwijderen', 'Slett rad', 'Usuń wiersz', 'Eliminar linha', 'Ta bort rad', '删除行'],
    delete_column: ['Odstranit sloupec', 'Slet kolonne', 'Spalte löschen', 'Διαγραφή στήλης', 'Delete column', 'Eliminar columna', 'Supprimer la colonne', 'कॉलम हटाएँ', 'Elimina colonna', '列を削除', 'Kolom verwijderen', 'Slett kolonne', 'Usuń kolumnę', 'Eliminar coluna', 'Ta bort kolumn', '删除列'],
    table_delete: ['Odstranit tabulku', 'Slet tabel', 'Tabelle löschen', 'Διαγραφή πίνακα', 'Delete table', 'Eliminar tabla', 'Supprimer le tableau', 'तालिका हटाएँ', 'Elimina tabella', '表を削除', 'Tabel verwijderen', 'Slett tabell', 'Usuń tabelę', 'Eliminar tabela', 'Ta bort tabell', '删除表格'],
    tools: ['Nástroje', 'Værktøjer', 'Extras', 'Εργαλεία', 'Tools', 'Herramientas', 'Outils', 'टूल', 'Strumenti', 'ツール', 'Extra', 'Verktøy', 'Narzędzia', 'Ferramentas', 'Verktyg', '工具'],
    word_count_title: ['Počet slov', 'Ordoptælling', 'Wörter zählen', 'Καταμέτρηση λέξεων', 'Word count', 'Contar palabras', 'Statistiques', 'शब्द गणना', 'Conteggio parole', '文字カウント', 'Woorden tellen', 'Ordtelling', 'Statystyka wyrazów', 'Contagem de palavras', 'Ordräkning', '字数统计'],
    chars_no_spaces: ['Znaky bez mezer', 'Tegn uden mellemrum', 'Zeichen ohne Leerzeichen', 'Χαρακτήρες χωρίς κενά', 'Characters without spaces', 'Caracteres sin espacios', 'Caractères (espaces non compris)', 'बिना स्पेस के वर्ण', 'Caratteri senza spazi', '文字数（スペースを含めない）', 'Tekens zonder spaties', 'Tegn uten mellomrom', 'Znaki bez spacji', 'Caracteres sem espaços', 'Tecken utan blanksteg', '字符数（不计空格）'],
    selected_words: ['Vybraná slova', 'Markerede ord', 'Markierte Wörter', 'Επιλεγμένες λέξεις', 'Selected words', 'Palabras seleccionadas', 'Mots sélectionnés', 'चयनित शब्द', 'Parole selezionate', '選択した単語', 'Geselecteerde woorden', 'Merkede ord', 'Zaznaczone wyrazy', 'Palavras selecionadas', 'Markerade ord', '所选字词'],
    shortcuts: ['Klávesové zkratky', 'Tastaturgenveje', 'Tastenkürzel', 'Συντομεύσεις πληκτρολογίου', 'Keyboard shortcuts', 'Atajos de teclado', 'Raccourcis clavier', 'कीबोर्ड शॉर्टकट', 'Scorciatoie da tastiera', 'キーボード ショートカット', 'Sneltoetsen', 'Hurtigtaster', 'Skróty klawiaturowe', 'Atalhos de teclado', 'Kortkommandon', '键盘快捷键'],
    link_text: ['Zobrazovaný text', 'Tekst, der skal vises', 'Anzuzeigender Text', 'Κείμενο προς εμφάνιση', 'Text to display', 'Texto para mostrar', 'Texte à afficher', 'दिखाया जाने वाला टेक्स्ट', 'Testo da visualizzare', '表示するテキスト', 'Weer te geven tekst', 'Tekst som skal vises', 'Tekst do wyświetlenia', 'Texto a apresentar', 'Text som ska visas', '要显示的文字'],
    link_address: ['Adresa', 'Adresse', 'Adresse', 'Διεύθυνση', 'Address', 'Dirección', 'Adresse', 'पता', 'Indirizzo', 'アドレス', 'Adres', 'Adresse', 'Adres', 'Endereço', 'Adress', '地址'],
    link_remove: ['Odebrat odkaz', 'Fjern link', 'Link entfernen', 'Κατάργηση συνδέσμου', 'Remove link', 'Quitar enlace', 'Supprimer le lien', 'लिंक हटाएँ', 'Rimuovi collegamento', 'リンクを削除', 'Link verwijderen', 'Fjern lenke', 'Usuń link', 'Remover hiperligação', 'Ta bort länk', '移除链接'],
    link_invalid: ['Zadejte webovou, e-mailovou nebo telefonní adresu.', 'Angiv en web-, mail- eller telefonadresse.', 'Bitte eine Web-, E-Mail- oder Telefonadresse eingeben.', 'Εισαγάγετε διεύθυνση ιστού, email ή τηλεφώνου.', 'Enter a web, email or phone address.', 'Introduce una dirección web, de correo o de teléfono.', 'Saisissez une adresse web, e-mail ou de téléphone.', 'वेब, ईमेल या फ़ोन पता दर्ज करें।', 'Inserisci un indirizzo web, email o telefonico.', 'ウェブ、メール、または電話のアドレスを入力してください。', 'Voer een web-, e-mail- of telefoonadres in.', 'Skriv inn en nett-, e-post- eller telefonadresse.', 'Wpisz adres internetowy, e-mail lub telefon.', 'Introduza um endereço web, de e-mail ou de telefone.', 'Ange en webb-, e-post- eller telefonadress.', '请输入网址、邮箱或电话地址。'],
    previous_change: ['Předchozí změna', 'Forrige ændring', 'Vorherige Änderung', 'Προηγούμενη αλλαγή', 'Previous change', 'Cambio anterior', 'Modification précédente', 'पिछला बदलाव', 'Modifica precedente', '前の変更', 'Vorige wijziging', 'Forrige endring', 'Poprzednia zmiana', 'Alteração anterior', 'Föregående ändring', '上一处修订'],
    next_change: ['Další změna', 'Næste ændring', 'Nächste Änderung', 'Επόμενη αλλαγή', 'Next change', 'Cambio siguiente', 'Modification suivante', 'अगला बदलाव', 'Modifica successiva', '次の変更', 'Volgende wijziging', 'Neste endring', 'Następna zmiana', 'Alteração seguinte', 'Nästa ändring', '下一处修订']
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

Run it from the repo root: `node <path>/writer-wave1a-i18n.mjs .`

Expected: sixteen lines, each `<lang>: 38 keys added`. `git diff --stat -- ui/lang/desktop` shows 16 files with 38 insertions each and no deletions.

- [ ] **Step 2: Replace the registry test**

Replace `scripts/test-writer-commands.mjs` with:

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
    'insertTextBox', 'setEditingMode', 'editHeaderFooter', 'exitHeaderFooter', 'replaceMatch', 'selectAll', 'copy', 'cut', 'removeHyperlink'];
for (const type of types) assert.equal(WriterCommands.untrackable(type), legacy.test(type), type);

function harness(options = {}) {
    const log = {exec: [], table: [], notices: [], refreshed: 0, acted: []};
    const editor = {
        mode: options.mode || 'editing', refused: new Set(options.refused || []), active: new Set(options.active || []),
        snapshot() { return Object.assign({editingMode: this.mode}, options.snapshot); },
        can(command) { return this.refused.has(command.type) ? {ok: false, reason: 'engine'} : {ok: true}; },
        exec(command) { log.exec.push(command); return {ok: true}; },
        isActive(command) { return this.active.has(command.type); }
    };
    const slotTypes = {'history.undo': 'undo', 'history.redo': 'redo', 'text.bold': 'toggleBold', 'text.italic': 'toggleItalic',
        'insert.pageBreak': 'insertBreak', 'insert.endnote': 'insertNote', 'insert.toc': 'insertToc'};
    const lib = {
        commandForSlot: slot => ({type: slotTypes[slot] || slot}),
        commandForSlotValue: (slot, value) => ({type: 'value', slot, value}),
        tableCommand(target, command, dry) { log.table.push(command.type + ':' + !!dry); return {ok: true}; }
    };
    const state = {editor: options.noEditor ? null : editor, loading: !!options.loading};
    const commands = WriterCommands.create({
        tr: key => key, t: key => key, readonly: !!options.readonly, format: value => 'n' + value,
        editor: () => state.editor, lib: () => lib, loading: () => state.loading,
        act: id => { log.acted.push(id); return true; }, active: id => (options.open || []).includes(id),
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
    assert.equal(commands.tracking(), true);
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
    assert.equal(commands.invoke('mergeCells'), true);
    assert.equal(log.table.join(), 'mergeCells:true,splitCell:true,splitCell:false,mergeCells:true,mergeCells:false');
    assert.equal(log.exec.length, 0);
    assert.equal(log.refreshed, 2);
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
    assert.equal(commands.tracking(), false);
    for (const id of ['save', 'bold', 'copy', 'paste', 'goToPage', 'wordCount', 'zoom100', 'table', 'link']) assert.equal(commands.state(id).enabled, false, id);
    for (const id of ['open', 'shortcuts', 'outline']) assert.equal(commands.state(id).enabled, true, id);
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
{ // An entry can name its engine command directly.
    const {commands, log} = harness({refused: ['cut']});
    assert.equal(commands.state('copy').enabled, true);
    assert.equal(commands.state('cut').enabled, false);
    assert.equal(commands.state('cut').reason, 'engine');
    assert.equal(commands.invoke('selectAll'), true);
    assert.equal(log.exec[0].type, 'selectAll');
    assert.equal(commands.invoke('rowAbove'), true);
    assert.equal(log.exec[1].type + ':' + log.exec[1].where, 'insertRow:above');
    assert.equal(commands.invoke('spacing150'), true);
    assert.equal(log.exec[2].rule + ':' + log.exec[2].value, 'multiple:1.5');
    assert.equal(log.acted.length, 0);
}
{ // Structural entries are refused while changes are tracked.
    const {commands} = harness({mode: 'suggesting'});
    for (const id of ['rowAbove', 'deleteTable', 'pageBreak', 'endnote', 'toc']) assert.equal(commands.state(id).reason, 'tracking_unsupported', id);
    for (const id of ['copy', 'selectAll', 'superscript', 'marks', 'spacing115']) assert.equal(commands.state(id).enabled, true, id);
}
{ // A checked state comes from the snapshot, or from the app for its own actions.
    const {commands} = harness({snapshot: {showParagraphMarks: true, reviewDisplayMode: 'proposed', formatting: {lineSpacing: {rule: 'multiple', value: 1.5}}}, open: ['outline']});
    assert.equal(commands.state('marks').active, true);
    assert.equal(commands.state('markupFinal').active, true);
    assert.equal(commands.state('markupAll').active, false);
    assert.equal(commands.state('spacing150').active, true);
    assert.equal(commands.state('spacing100').active, false);
    assert.equal(commands.state('outline').active, true);
    assert.equal(commands.state('review').active, false);
    assert.equal(commands.menuItem('marks').checked, true);
    assert.equal(commands.menuItem('outline').checked, true);
    assert.equal(commands.menuItem('markupAll').checked, undefined);
}
{ // Without a display mode in the snapshot, all changes are shown.
    const {commands} = harness();
    assert.equal(commands.state('markupAll').active, true);
    assert.equal(commands.state('marks').active, false);
}
{ // Read-only windows keep reading actions.
    const {commands} = harness({readonly: true});
    for (const id of ['new', 'save', 'saveAs', 'image', 'qr', 'paste', 'pastePlain', 'table', 'link', 'tracking']) assert.equal(commands.state(id).enabled, false, id);
    for (const id of ['open', 'print', 'docx', 'search', 'goToPage', 'wordCount', 'zoomPage', 'shortcuts']) assert.equal(commands.state(id).enabled, true, id);
}
{ // An action can name the command type that tracking refuses.
    const {commands} = harness({mode: 'suggesting'});
    for (const id of ['qr', 'table', 'link']) {
        assert.equal(commands.state(id).enabled, false, id);
        assert.equal(commands.state(id).reason, 'tracking_unsupported', id);
    }
    assert.equal(commands.state('image').enabled, true);
    assert.equal(commands.state('paste').enabled, true);
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
    assert.equal(key('k'), 'link');
    assert.equal(key('g'), 'goToPage');
    assert.equal(key('s', {ctrlKey: false, metaKey: true}), 'save', 'Cmd counts as Ctrl');
    assert.equal(key('s', {altKey: true}), '', 'AltGr letters are text, not shortcuts');
    assert.equal(key('o', {shiftKey: true}), '');
    assert.equal(key('s', {ctrlKey: false}), '');
    for (const name of ['z', 'c', 'v', 'x', 'a']) assert.equal(key(name), '', 'the engine owns Ctrl+' + name);
}
{ // Labels, tooltips and menu items.
    const {commands, log} = harness({active: ['toggleBold']});
    assert.equal(commands.label('saveAs'), 'save_as');
    assert.equal(commands.label('qr'), 'desktop.qr_insert');
    assert.equal(commands.label('zoom125'), '125%');
    assert.equal(commands.label('spacing115'), 'n1.15');
    assert.equal(commands.format(1.5), 'n1.5');
    assert.equal(commands.tooltip('save'), 'save (Ctrl+S)');
    assert.equal(commands.tooltip('copy'), 'copy (Ctrl+C)');
    assert.equal(commands.tooltip('bold'), 'bold (Ctrl+B)');
    assert.equal(commands.tooltip('strike'), 'strike');
    const item = commands.menuItem('print');
    assert.equal(item.id, 'print');
    assert.equal(item.label, 'print');
    assert.equal(item.icon, 'printer');
    assert.equal(item.shortcut, 'Ctrl+P');
    assert.equal(item.shortcutHint, undefined);
    assert.equal(item.disabled, false);
    item.action();
    assert.equal(log.acted[0], 'print');
    assert.equal(commands.menuItem('undo').disabled, false);
    assert.equal(commands.menuItem('search').icon, 'search');
    // The shell dispatches `shortcut` itself; clipboard keys are only shown.
    assert.equal(commands.menuItem('copy').shortcut, undefined);
    assert.equal(commands.menuItem('copy').shortcutHint, 'Ctrl+C');
    // A glyph is drawn by the toolbar only, and a pressed toggle is not a check mark.
    assert.equal(commands.menuItem('alignLeft').icon, '');
    assert.equal(commands.menuItem('bold').checked, undefined);
}
{ // The engine wants a bare hex, a palette name, half-points and a number of lines.
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
    assert.equal(commands.valueCommand('list.lineSpacing', '1.5').value, 1.5);
    assert.equal(commands.valueCommand('list.lineSpacing', '').value, 1, 'an unknown spacing still probes with a valid value');
}

// The table itself.
const definitions = WriterCommands.definitions, ids = definitions.map(definition => definition.id);
assert.equal(new Set(ids).size, ids.length, 'ids are unique');
const bound = definitions.flatMap(definition => definition.keys || []);
assert.equal(new Set(bound).size, bound.length, 'no key is bound twice');
for (const definition of definitions) {
    assert.ok(definition.label || definition.fullLabel || definition.text || definition.number !== undefined, definition.id + ' has a label');
    assert.ok(!(definition.slot && definition.command), definition.id + ' is a slot or a command, not both');
    assert.ok(definition.slot || definition.command || !definition.toggle, definition.id + ': only engine commands toggle');
    assert.ok(!(definition.shortcut && definition.hint), definition.id + ' shows one shortcut');
    if (definition.keys) assert.ok(definition.shortcut, definition.id + ' shows the shortcut it binds');
    if (definition.options) assert.equal(definition.control, 'select', definition.id + ': only a select has options');
}

// Every label of the table and every literal tr('key') of the Writer files exists in all sixteen languages.
const used = new Set(definitions.filter(definition => definition.fullLabel || definition.label).map(definition => definition.fullLabel || 'desktop.writer_' + definition.label));
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

- [ ] **Step 3: Run the test to verify it fails**

Run: `node scripts/test-writer-commands.mjs`
Expected: FAIL with `TypeError: commands.tracking is not a function`.

- [ ] **Step 4: Replace the registry**

Replace `ui/js/desktop/apps/writer-commands.js` with:

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
    const shows = name => snapshot => (snapshot.reviewDisplayMode || 'all-markup') === name;
    const zoom = percent => ({id: 'zoom' + percent, text: percent + '%', needsEditor: true});
    const lines = value => ({id: 'spacing' + Math.round(value * 100), command: {type: 'setLineSpacing', rule: 'multiple', value}, number: value,
        active: snapshot => snapshot.formatting?.lineSpacing?.rule === 'multiple' && snapshot.formatting.lineSpacing.value === value});
    // One entry per command a surface can offer. An entry is one of three kinds:
    //   slot       an engine toolbar slot
    //   command    an engine command that has no slot
    //   (neither)  an app action; its `case` lives in act() in writer.js
    // Fields:
    //   label      desktop.writer_* key; fullLabel names a key outside that prefix; text is a literal; number is formatted for the locale
    //   icon       shared icon key, for menus and the toolbar; glyph is drawn by writer-toolbar.js and used there only
    //   shortcut   shown in menus and tooltips, and dispatched by the shell outside editable content; keys is what Writer binds itself
    //   hint       a shortcut that is only shown, because the engine or the browser owns the key
    //   toggle     pressed state from the engine, for the toolbar; active(snapshot) is a state menus show as a check mark
    //   control    'select', 'number' or 'color' instead of a button; options are the values of a select; value is a colour default
    //   write      refused in a read-only window; needsEditor: refused without a document
    //   untracked  command type whose tracking refusal disables the action
    const DEFINITIONS = [
        {id: 'undo', slot: 'history.undo', label: 'undo', icon: 'undo', fallback: '↶', shortcut: 'Ctrl+Z'},
        {id: 'redo', slot: 'history.redo', label: 'redo', icon: 'redo', fallback: '↷', shortcut: 'Ctrl+Y'},
        {id: 'cut', command: {type: 'cut'}, label: 'cut', icon: 'cut', hint: 'Ctrl+X'},
        {id: 'copy', command: {type: 'copy'}, label: 'copy', icon: 'copy', hint: 'Ctrl+C'},
        {id: 'paste', label: 'paste', icon: 'paste', hint: 'Ctrl+V', write: true, needsEditor: true},
        {id: 'pastePlain', label: 'paste_plain', hint: 'Ctrl+Shift+V', write: true, needsEditor: true},
        {id: 'selectAll', command: {type: 'selectAll'}, label: 'select_all', hint: 'Ctrl+A'},
        {id: 'style', slot: 'styles.style', label: 'style', control: 'select'},
        {id: 'fontFamily', slot: 'font.family', label: 'font', control: 'select'},
        {id: 'fontSize', slot: 'font.size', label: 'size', control: 'number'},
        {id: 'bold', slot: 'text.bold', label: 'bold', icon: 'format-bold', fallback: 'B', toggle: true, hint: 'Ctrl+B'},
        {id: 'italic', slot: 'text.italic', label: 'italic', icon: 'format-italic', fallback: 'I', toggle: true, hint: 'Ctrl+I'},
        {id: 'underline', slot: 'text.underline', label: 'underline', glyph: 'format-underline', fallback: 'U', toggle: true, hint: 'Ctrl+U'},
        {id: 'strike', slot: 'text.strike', label: 'strike', icon: 'format-strike', fallback: 'S', toggle: true},
        {id: 'superscript', slot: 'script.super', label: 'superscript', toggle: true},
        {id: 'subscript', slot: 'script.sub', label: 'subscript', toggle: true},
        {id: 'clearFormat', slot: 'format.clear', label: 'clear_format', glyph: 'clear-format', fallback: 'T'},
        {id: 'copyFormat', command: {type: 'copyFormatting'}, label: 'copy_format'},
        {id: 'pasteFormat', command: {type: 'pasteFormatting'}, label: 'paste_format'},
        {id: 'alignLeft', slot: 'alignment.left', label: 'align_left', glyph: 'align-left', fallback: '≡', toggle: true},
        {id: 'alignCenter', slot: 'alignment.center', label: 'align_center', glyph: 'align-center', fallback: '≡', toggle: true},
        {id: 'alignRight', slot: 'alignment.right', label: 'align_right', glyph: 'align-right', fallback: '≡', toggle: true},
        {id: 'alignJustify', slot: 'alignment.justify', label: 'justify', glyph: 'align-justify', fallback: '≡', toggle: true},
        {id: 'lineSpacing', slot: 'list.lineSpacing', label: 'line_spacing', control: 'select', options: [1, 1.15, 1.5, 2]},
        lines(1), lines(1.15), lines(1.5), lines(2),
        {id: 'bullets', slot: 'list.bullet', label: 'bullets', icon: 'list', fallback: '•', toggle: true},
        {id: 'numbering', slot: 'list.numbered', label: 'numbering', icon: 'format-numbered', fallback: '1.', toggle: true},
        {id: 'outdent', slot: 'list.outdent', label: 'outdent', glyph: 'outdent', fallback: '⇤'},
        {id: 'indent', slot: 'list.indent', label: 'indent', glyph: 'indent', fallback: '⇥'},
        {id: 'textColor', slot: 'text.color', label: 'color', glyph: 'text-color', control: 'color', value: '#172033'},
        {id: 'highlight', slot: 'text.highlight', label: 'highlight', glyph: 'highlight', control: 'color', value: '#ffff00'},
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
        {id: 'goToPage', label: 'go_page', shortcut: 'Ctrl+G', keys: ['g'], needsEditor: true},
        {id: 'outline', label: 'outline', icon: 'list', fallback: '☷'},
        {id: 'format', label: 'format', icon: 'sliders', fallback: 'A'},
        {id: 'review', label: 'review', icon: 'chat', fallback: '☷'},
        {id: 'assist', label: 'assist', icon: 'star', fallback: '✦'},
        {id: 'insert', label: 'insert', icon: 'plus', fallback: '＋'},
        {id: 'marks', slot: 'review.paragraphMarks', label: 'marks', active: snapshot => !!snapshot.showParagraphMarks},
        {id: 'markupAll', slot: 'review.allMarkup', label: 'markup_all', active: shows('all-markup')},
        {id: 'markupFinal', slot: 'review.noMarkup', label: 'markup_final', active: shows('proposed')},
        {id: 'markupOriginal', slot: 'review.original', label: 'original', active: shows('original')},
        {id: 'focus', label: 'focus_mode', icon: 'maximize', fallback: '⛶'},
        {id: 'fit', label: 'fit', icon: 'monitor'},
        {id: 'zoomPage', label: 'zoom_page', needsEditor: true},
        zoom(50), zoom(75), zoom(100), zoom(125), zoom(150), zoom(200),
        {id: 'zoomIn', label: 'zoom_in', icon: 'zoom-in', needsEditor: true},
        {id: 'zoomOut', label: 'zoom_out', icon: 'zoom-out', needsEditor: true},
        {id: 'table', label: 'table', glyph: 'table', fallback: '▦', write: true, needsEditor: true, untracked: 'insertTable'},
        {id: 'image', label: 'image', icon: 'image', fallback: '▣', write: true, needsEditor: true},
        {id: 'qr', fullLabel: 'desktop.qr_insert', icon: 'image', write: true, needsEditor: true, untracked: 'insertImage'},
        {id: 'link', label: 'hyperlink', icon: 'link', fallback: '∞', shortcut: 'Ctrl+K', keys: ['k'], write: true, needsEditor: true, untracked: 'insertHyperlink'},
        {id: 'pageBreak', slot: 'insert.pageBreak', label: 'page_break'},
        {id: 'sectionNext', slot: 'insert.sectionBreakNextPage', label: 'section_next'},
        {id: 'sectionContinuous', slot: 'insert.sectionBreakContinuous', label: 'section_continuous'},
        {id: 'footnote', slot: 'insert.footnote', label: 'footnote'},
        {id: 'endnote', slot: 'insert.endnote', label: 'endnote'},
        {id: 'toc', slot: 'insert.toc', label: 'toc'},
        {id: 'textBox', slot: 'insert.textBox', label: 'text_box'},
        {id: 'header', command: {type: 'editHeaderFooter', position: 'header'}, label: 'header'},
        {id: 'footer', command: {type: 'editHeaderFooter', position: 'footer'}, label: 'footer'},
        {id: 'pageNumber', slot: 'insert.pageXofY', label: 'page_numbers'},
        {id: 'backDocument', command: {type: 'exitHeaderFooter'}, label: 'back_document'},
        {id: 'rowAbove', command: {type: 'insertRow', where: 'above'}, label: 'row_above'},
        {id: 'rowBelow', command: {type: 'insertRow', where: 'below'}, label: 'row_below'},
        {id: 'columnLeft', command: {type: 'insertColumn', where: 'left'}, label: 'column_left'},
        {id: 'columnRight', command: {type: 'insertColumn', where: 'right'}, label: 'column_right'},
        {id: 'deleteRow', command: {type: 'deleteRow'}, label: 'delete_row'},
        {id: 'deleteColumn', command: {type: 'deleteColumn'}, label: 'delete_column'},
        {id: 'deleteTable', command: {type: 'deleteTable'}, label: 'table_delete'},
        {id: 'mergeCells', command: {type: 'mergeCells'}, label: 'merge_cells'},
        {id: 'splitCell', command: {type: 'splitCell', rows: 1, cols: 2}, label: 'split_cell'},
        {id: 'wordCount', label: 'word_count_title', needsEditor: true},
        {id: 'tracking', label: 'tracking', write: true, needsEditor: true},
        {id: 'previousChange', slot: 'review.previousChange', label: 'previous_change'},
        {id: 'nextChange', slot: 'review.nextChange', label: 'next_change'},
        {id: 'shortcuts', label: 'shortcuts', icon: 'keyboard'}
    ];
    // host: {tr, t, readonly, editor(), lib(), loading(), act(id), notice(message, error), refresh(),
    //        active(id)?: whether an app action is switched on, format(number)?: a number in the user's locale}
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
        const format = value => (host.format || String)(value);
        function label(id) {
            const definition = get(id);
            if (definition.text) return definition.text;
            if (definition.number !== undefined) return format(definition.number);
            return definition.fullLabel ? host.t(definition.fullLabel) : host.tr(definition.label);
        }
        const tooltip = id => label(id) + (get(id).shortcut || get(id).hint ? ' (' + (get(id).shortcut || get(id).hint) + ')' : '');
        // The engine command behind an entry; null for value slots and app actions.
        function command(id) {
            const definition = get(id);
            if (definition?.command) return definition.command;
            return definition?.slot && host.lib() ? host.lib().commandForSlot(definition.slot) : null;
        }
        function state(id) {
            const definition = get(id), editor = host.editor(), off = {enabled: false, active: false, reason: ''};
            if (!definition) return off;
            if (definition.slot || definition.command) {
                const engineCommand = editor ? command(id) : null;
                if (!engineCommand) return off;
                const allowed = can(engineCommand);
                const active = definition.active ? !!definition.active(editor.snapshot()) : !!definition.toggle && !!editor.isActive(engineCommand);
                return {enabled: !!allowed.ok, active, reason: allowed.reason || ''};
            }
            if ((definition.write && host.readonly) || (definition.needsEditor && !editor)) return off;
            const reason = definition.untracked ? blocked(definition.untracked) : '';
            return {enabled: !reason, active: !!host.active?.(id), reason};
        }
        const invoke = id => get(id)?.slot || get(id)?.command ? run(command(id)) : host.act(id);
        // The engine wants a bare hex for colours, a palette name for highlights, half-points for sizes and lines for spacing.
        function valueCommand(slot, value) {
            const lib = host.lib();
            if (slot === 'text.color') return lib.commandForSlotValue(slot, colorHex(value).replace('#', ''));
            if (slot === 'text.highlight') return lib.commandForSlotValue(slot, highlightName(value));
            if (slot === 'font.size') return lib.commandForSlotValue(slot, Number(value) * 2);
            if (slot === 'list.lineSpacing') return lib.commandForSlotValue(slot, Number(value) || 1);
            return lib.commandForSlotValue(slot, value);
        }
        // The command a key press asks for, or '' when Writer does not bind it. Alt is excluded
        // because AltGr reports Ctrl+Alt, and those letters are text.
        function actionForKey(event) {
            if (!(event.ctrlKey || event.metaKey) || event.altKey) return '';
            const key = (event.shiftKey ? 'shift+' : '') + String(event.key).toLowerCase();
            return DEFINITIONS.find(definition => definition.keys?.includes(key))?.id || '';
        }
        // A menu item with the state of this moment. The shell reads `disabled` and `checked` when the
        // menus are set, so writer-menus.js sets them again when that state changes.
        function menuItem(id) {
            const definition = get(id), current = state(id);
            const item = {id, label: label(id), icon: definition.icon || '', disabled: !current.enabled, action: () => invoke(id)};
            if (definition.shortcut) item.shortcut = definition.shortcut;
            else if (definition.hint) item.shortcutHint = definition.hint;
            // A pressed toolbar toggle follows the selection; menus only mark modes and open panels.
            if (current.active && !definition.toggle) item.checked = true;
            return item;
        }
        return {get, can, run, blocked, tracking, label, tooltip, format, command, state, invoke, valueCommand, actionForKey, menuItem};
    }
    window.WriterCommands = {create, definitions: DEFINITIONS, untrackable: type => UNTRACKABLE.has(type), colors: {hex: colorHex, highlightName, highlightHex}};
})();
```

- [ ] **Step 5: Let the toolbar draw glyphs by their new field**

The definitions with a locally drawn glyph now carry it as `glyph`, not `icon`. In `ui/js/desktop/apps/writer-toolbar.js`, inside `control(id)`:

Replace
```js
            if (definition.control === 'color') return '<label class="writer-color" title="' + name + '">' + icon(definition.icon, '') + '<input type="color" ' + attrs(id) + ' value="' + definition.value + '" aria-label="' + name + '"></label>';
            return '<button type="button" ' + attrs(id) + ' title="' + esc(commands.tooltip(id)) + '" aria-label="' + name + '">' + icon(definition.icon, definition.fallback) + '</button>';
```
with
```js
            if (definition.control === 'color') return '<label class="writer-color" title="' + name + '">' + icon(definition.glyph, '') + '<input type="color" ' + attrs(id) + ' value="' + definition.value + '" aria-label="' + name + '"></label>';
            return '<button type="button" ' + attrs(id) + ' title="' + esc(commands.tooltip(id)) + '" aria-label="' + name + '">' + icon(definition.glyph || definition.icon, definition.fallback) + '</button>';
```

In `scripts/test-writer-toolbar.mjs` replace
```js
for (const id of placed) assert.ok(known.get(id).control || known.get(id).icon, id + ' needs an icon');
```
with
```js
for (const id of placed) assert.ok(known.get(id).control || known.get(id).icon || known.get(id).glyph, id + ' needs an icon');
```

- [ ] **Step 6: Run the tests**

Run: `npm run test:writer`
Expected: the last two lines are
```
Writer commands: tracking rule, gate, state, shortcuts, menu items, colours and 164 language keys pass
Writer toolbar: groups, glyphs and overflow fit pass
```
(The count grows to 178 once the dialogs and menus files exist.)

Run: `go test ./ui -run "(?i)(materials|office|writer|virtualdesktop|lifecycle|i18n)" -count=1`
Expected: `ok  	aurago/ui`

- [ ] **Step 7: Commit**

```bash
git add ui/js/desktop/apps/writer-commands.js ui/js/desktop/apps/writer-toolbar.js scripts/test-writer-commands.mjs scripts/test-writer-toolbar.mjs ui/lang/desktop
git commit -m "feat(writer): registry carries plain engine commands, menu state and the Wave 1a commands"
```

---

### Task 2: Form dialog: read-only rows, a size grid, a third button

**Files:**
- Replace: `ui/js/desktop/shared/form-dialog.js`
- Modify: `ui/css/desktop-form-dialog.css`

- [ ] **Step 1: Replace the dialog**

Replace `ui/js/desktop/shared/form-dialog.js` with:

```js
(function () {
    'use strict';
    const escapeHTML = value => String(value ?? '').replace(/[&<>"']/g, c => ({'&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;'}[c]));
    let serial = 0;
    // Field: {id, label, type: 'text'|'url'|'number'|'select'|'checkbox'|'static'|'grid', value, required,
    // min, max, step, maxlength, placeholder, options: [{value, label}]}.
    //   static  a read-only row: label and value. It answers nothing.
    //   grid    rows x cols cells; pointing at a cell writes its row and column into the two number
    //           fields named by targets: [rowsId, colsId], and a click submits. It answers nothing.
    const silent = field => field.type === 'static' || field.type === 'grid';
    function fieldMarkup(field) {
        const name = 'data-form-field="' + escapeHTML(field.id) + '"', label = '<span>' + escapeHTML(field.label) + '</span>';
        if (field.type === 'static') return '<p class="vd-form-static">' + label + '<strong>' + escapeHTML(field.value ?? '') + '</strong></p>';
        if (field.type === 'grid') {
            let cells = '';
            for (let row = 1; row <= field.rows; row++) for (let col = 1; col <= field.cols; col++)
                cells += '<button type="button" tabindex="-1" data-form-cell="' + row + ',' + col + '" aria-label="' + row + ' × ' + col + '"></button>';
            return '<div class="vd-form-grid" data-form-grid="' + escapeHTML(field.id) + '" role="group" aria-label="' + escapeHTML(field.label) + '">' + cells + '</div>';
        }
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
    // open({title, fields, submitLabel, cancelLabel, extraLabel, validate, signal}) resolves to {id: value},
    // to {extra: true} when the extra button was pressed, or to null when the dialog is cancelled.
    // validate(values) returns a message to refuse them. Without cancelLabel there is no Cancel button.
    function open(options) {
        if (options.signal?.aborted) return Promise.resolve(null);
        const fields = options.fields || [], id = 'vd-form-dialog-' + (++serial);
        const node = document.createElement('dialog');
        node.className = 'vd-form-dialog';
        node.setAttribute('aria-labelledby', id);
        node.innerHTML = '<form method="dialog" novalidate><h2 id="' + id + '">' + escapeHTML(options.title) + '</h2>' + fields.map(fieldMarkup).join('') +
            '<p role="alert" data-form-error hidden></p><div class="vd-modal-actions">' +
            (options.extraLabel ? '<button type="button" class="vd-button" data-form-extra>' + escapeHTML(options.extraLabel) + '</button>' : '') +
            (options.cancelLabel ? '<button type="button" class="vd-button" data-form-cancel>' + escapeHTML(options.cancelLabel) + '</button>' : '') +
            '<button type="submit" class="vd-button vd-button-primary">' + escapeHTML(options.submitLabel) + '</button></div></form>';
        // The dialog lives on the body, so an app's own key handlers never see its keys.
        document.body.appendChild(node);
        return new Promise(resolve => {
            const form = node.querySelector('form'), error = node.querySelector('[data-form-error]');
            const input = fieldId => form.querySelector('[data-form-field="' + CSS.escape(fieldId) + '"]');
            let answer = null;
            const finish = value => { answer = value; node.close(); };
            const onAbort = () => finish(null);
            node.addEventListener('close', () => { options.signal?.removeEventListener('abort', onAbort); node.remove(); resolve(answer); }, {once: true});
            node.querySelector('[data-form-cancel]')?.addEventListener('click', () => finish(null));
            node.querySelector('[data-form-extra]')?.addEventListener('click', () => finish({extra: true}));
            for (const field of fields.filter(candidate => candidate.type === 'grid')) {
                const grid = form.querySelector('[data-form-grid="' + CSS.escape(field.id) + '"]'), [rows, cols] = field.targets.map(input);
                grid.style.setProperty('--vd-form-grid-cols', String(field.cols));
                const paint = () => {
                    for (const cell of grid.children) {
                        const [row, col] = cell.dataset.formCell.split(',').map(Number);
                        cell.toggleAttribute('data-on', row <= rows.valueAsNumber && col <= cols.valueAsNumber);
                    }
                };
                const point = event => {
                    const cell = event.target.closest('[data-form-cell]');
                    if (!cell) return false;
                    [rows.value, cols.value] = cell.dataset.formCell.split(',');
                    paint();
                    return true;
                };
                grid.addEventListener('pointerover', point);
                grid.addEventListener('click', event => { if (point(event)) form.requestSubmit(); });
                rows.addEventListener('input', paint);
                cols.addEventListener('input', paint);
                paint();
            }
            form.addEventListener('submit', event => {
                event.preventDefault();
                if (!form.reportValidity()) return;
                const values = {};
                for (const field of fields) if (!silent(field)) values[field.id] = read(field, input(field.id));
                const message = options.validate?.(values) || '';
                error.textContent = message;
                error.hidden = !message;
                if (!message) finish(values);
            });
            options.signal?.addEventListener('abort', onAbort, {once: true});
            // showModal traps the focus, closes on Escape and returns the focus on close.
            node.showModal();
            const first = node.querySelector('input[data-form-field],select[data-form-field]') || node.querySelector('[type=submit]');
            first.focus();
            first.select?.();
        });
    }
    window.AuraFormDialog = {open};
})();
```

- [ ] **Step 2: Style the new parts**

In `ui/css/desktop-form-dialog.css` replace the line
```css
.vd-form-dialog form{display:grid;gap:12px;margin:0}
```
with
```css
.vd-form-dialog form{display:grid;gap:12px;margin:0;max-height:calc(100vh - 96px);overflow:auto}
.vd-form-dialog .vd-form-static{display:flex;justify-content:space-between;gap:24px;margin:0;color:var(--vd-muted,var(--vd-theme-muted,#a7b1c3))}
.vd-form-dialog .vd-form-static strong{color:var(--vd-text,#e4e8f0);font-weight:600;font-variant-numeric:tabular-nums;text-align:right}
.vd-form-dialog .vd-form-grid{display:grid;grid-template-columns:repeat(var(--vd-form-grid-cols,10),18px);gap:3px;justify-content:start}
.vd-form-dialog .vd-form-grid button{width:18px;height:18px;min-height:0;padding:0;border:1px solid var(--vd-mat-field-border,var(--ds-color-border-default,var(--vd-border,#ffffff26)));border-radius:3px;background:var(--vd-mat-field,transparent);cursor:pointer}
.vd-form-dialog .vd-form-grid button[data-on]{background:var(--vd-accent,#9abfff);border-color:var(--vd-accent,#9abfff)}
```

- [ ] **Step 3: Verify**

Run: `node --check ui/js/desktop/shared/form-dialog.js`
Expected: no output.

Run: `go test ./ui -run "(?i)(materials|office|writer|virtualdesktop|lifecycle|i18n)" -count=1` and `npm run check:ui`
Expected: `ok  	aurago/ui`; `check:ui` ends with `Ocean chat sea life renderer verified`. If `check:ui` reports drift, run `npm run build:ui` and stage the regenerated bundle only if `git status` shows it changed.

The dialog itself is exercised in the browser by the existing `verifyWriterTableDialog` (unchanged behaviour for `rows`/`cols` and Cancel) and by Task 3.

- [ ] **Step 4: Commit**

```bash
git add ui/js/desktop/shared/form-dialog.js ui/css/desktop-form-dialog.css
git commit -m "feat(desktop): form dialog with read-only rows, a size grid and a third button"
```

---

### Task 3: Dialogs: table size, link, go to page, word count, shortcut overview

**Files:**
- Create: `ui/js/desktop/apps/writer-dialogs.js`
- Create: `ui/desktop_writer_menus_test.go`
- Modify: `ui/js/desktop/apps/writer.js`, `ui/js/desktop/apps/writer-panels.js`
- Modify: `ui/js/desktop/core/module-loader.js`, `ui/desktop_office_assets_test.go`, `ui/desktop_writer_browser_test.go` (two fixtures and one call), `ui/desktop_writer_reload_browser_test.go` (one fixture)

- [ ] **Step 1: Write the browser check**

Create `ui/desktop_writer_menus_test.go`:

```go
package ui

import (
	"testing"

	"github.com/go-rod/rod"
)

// writerWave1Prelude is shared by the Wave 1a checks: the app, a wait, the open form dialog and
// a way back to the untouched blank document.
const writerWave1Prelude = `
        const app=WriterApp.instances.get('test'),editor=app.editor,wait=ms=>new Promise(r=>setTimeout(r,ms));
        const dialog=()=>document.querySelector('dialog.vd-form-dialog');
        const field=id=>dialog().querySelector('[data-form-field="'+id+'"]');
        const submit=()=>dialog().querySelector('[type=submit]').click();
        const body=()=>editor.surface.session.bodyText();
        const failed=()=>{const n=document.querySelector('[data-notice]');return !n.hidden&&n.dataset.error==='true'?document.querySelector('[data-notice-text]').textContent:'';};
        const rewind=()=>{for(let i=0;i<80&&editor.snapshot().canUndo;i++)editor.exec({type:'undo'});};
`

// verifyWriterDialogs drives the Wave 1a dialogs on the blank document and leaves it blank.
func verifyWriterDialogs(t *testing.T, page *rod.Page) {
	t.Helper()
	page.MustEval(`async()=>{` + writerWave1Prelude + `
        try {
            // A link at a collapsed caret asks for text and address, and refuses a script address.
            void app.act('link');await wait(200);
            if(!dialog())throw Error('link dialog did not open');
            if(dialog().querySelector('[data-form-extra]'))throw Error('a new link offers no removal');
            field('text').value='AuraGo';field('href').value='javascript:alert(1)';submit();await wait(80);
            if(!dialog())throw Error('a script address was accepted');
            field('href').value='example.com';submit();await wait(300);
            if(dialog())throw Error('link dialog stayed open');
            if(failed())throw Error('link insert: '+failed());
            if(!body().includes('AuraGo'))throw Error('the link text was not inserted: '+JSON.stringify(body()));
            // The caret sits in the new link: editing shows its address and offers removal.
            editor.exec({type:'selectAll'});
            const made=editor.query({type:'hyperlinkAt'});
            if(made?.href!=='https://example.com')throw Error('hyperlinkAt after insert: '+JSON.stringify(made));
            void app.act('link');await wait(200);
            if(field('text'))throw Error('editing a link must not ask for its text');
            if(field('href').value!=='https://example.com')throw Error('edit shows '+field('href').value);
            field('href').value='https://aurago.example/docs';submit();await wait(300);
            if(failed())throw Error('link edit: '+failed());
            editor.exec({type:'selectAll'});
            const edited=editor.query({type:'hyperlinkAt'});
            if(edited?.href!=='https://aurago.example/docs')throw Error('hyperlinkAt after edit: '+JSON.stringify(edited));
            void app.act('link');await wait(200);
            dialog().querySelector('[data-form-extra]').click();await wait(300);
            if(failed())throw Error('link removal: '+failed());
            editor.exec({type:'selectAll'});
            if(editor.query({type:'hyperlinkAt'}))throw Error('the link was not removed');
            if(!body().includes('AuraGo'))throw Error('removing the link removed its text');
            // Go to page accepts a page of the document.
            void app.act('goToPage');await wait(200);
            if(field('page').max!==String(editor.getTotalPages()))throw Error('go to page allows up to '+field('page').max);
            field('page').value='1';submit();await wait(150);
            if(dialog())throw Error('go to page stayed open');
            // Word count is read-only: rows, one button.
            void app.act('wordCount');await wait(200);
            const rows=[...dialog().querySelectorAll('.vd-form-static')];
            if(rows.length<4)throw Error('word count shows '+rows.length+' rows');
            if(rows[1].querySelector('strong').textContent!=='1')throw Error('one word expected, got '+rows[1].textContent);
            if(dialog().querySelector('[data-form-cancel]'))throw Error('an information dialog has no Cancel');
            submit();await wait(150);
            // The shortcut overview lists what the registry binds.
            void app.act('shortcuts');await wait(200);
            const keys=[...dialog().querySelectorAll('.vd-form-static strong')].map(x=>x.textContent);
            for(const key of ['Ctrl+K','Ctrl+G','Ctrl+S','Ctrl+C'])if(!keys.includes(key))throw Error('shortcut overview lacks '+key);
            submit();await wait(150);
            // The table dialog: pointing at a grid cell fills rows and columns.
            void app.act('table');await wait(200);
            dialog().querySelector('[data-form-cell="2,3"]').dispatchEvent(new PointerEvent('pointerover',{bubbles:true}));
            if(field('rows').value!=='2'||field('cols').value!=='3')throw Error('grid wrote '+field('rows').value+' x '+field('cols').value);
            if(dialog().querySelectorAll('[data-form-cell][data-on]').length!==6)throw Error('grid highlight is wrong');
            dialog().querySelector('[data-form-cancel]').click();await wait(150);
            if(dialog())throw Error('table dialog stayed open');
        } finally { dialog()?.close(); rewind(); }
        if(body().trim())throw Error('the document was not left blank: '+JSON.stringify(body()));
    }`)
}
```

The link part of this helper was rewritten during implementation to edit and remove the link from a collapsed caret inside a longer paragraph; see `ui/desktop_writer_menus_test.go`.

In `ui/desktop_writer_browser_test.go`, in `TestDesktopWriterAppBrowser`, add the call right after `verifyWriterTableDialog(t, page)`:

```go
	verifyWriterDialogs(t, page)
```

- [ ] **Step 2: Write the dialogs**

Create `ui/js/desktop/apps/writer-dialogs.js`:

```js
(function () {
    'use strict';
    // app: the state facade of writer.js. Every dialog asks through the shared form dialog and
    // acts through the registry gate, so tracking and read-only refusals are announced the same way.
    function create(app) {
        const {tr, ctx, commands} = app;
        const ask = (title, fields, more) => window.AuraFormDialog.open(Object.assign({title, fields, submitLabel: ctx.t('desktop.ok'),
            cancelLabel: ctx.t('desktop.cancel'), signal: app.signal()}, more));
        // A read-only dialog: rows of [label, value] and one button.
        const tell = (title, rows) => window.AuraFormDialog.open({title, submitLabel: ctx.t('desktop.ok'), signal: app.signal(),
            fields: rows.map(([label, value], index) => ({id: 'row' + index, type: 'static', label, value}))});
        async function table() {
            const size = await ask(tr('table'), [
                {id: 'grid', type: 'grid', label: tr('table'), rows: 8, cols: 10, targets: ['rows', 'cols']},
                {id: 'rows', label: tr('table_rows'), type: 'number', value: 3, min: 1, max: 100, step: 1, required: true},
                {id: 'cols', label: tr('table_columns'), type: 'number', value: 3, min: 1, max: 30, step: 1, required: true}]);
            if (size) commands.run({type: 'insertTable', rows: size.rows, cols: size.cols});
        }
        // An address without a scheme is a web address; only web, mail, phone and in-document targets are links.
        const address = value => /^[a-z][a-z0-9+.-]*:/i.test(value) || value.startsWith('#') ? value : 'https://' + value;
        async function link() {
            const editor = app.editor;
            if (!editor) return;
            const current = editor.query({type: 'hyperlinkAt'});
            const selected = editor.snapshot().selectionCollapsed ? '' : editor.query({type: 'selectedText'});
            const fields = [{id: 'href', label: tr('link_address'), value: current?.href || '', placeholder: 'https://', required: true, maxlength: 2000}];
            if (!current && !selected) fields.unshift({id: 'text', label: tr('link_text'), value: '', maxlength: 500});
            const answer = await ask(tr('hyperlink'), fields, {extraLabel: current ? tr('link_remove') : '',
                validate: values => /^(https?:|mailto:|tel:|#)/i.test(address(values.href)) ? '' : tr('link_invalid')});
            if (!answer || app.editor !== editor) return;
            // The engine edits a link where the selection is: inserting over a link replaces its
            // address and keeps its text, and unlinking keeps the text. hyperlinkAt's range is the
            // whole paragraph, so it is never used as a target.
            if (answer.extra) { commands.run({type: 'removeHyperlink'}); return; }
            const command = {type: 'insertHyperlink', href: address(answer.href)};
            if (!current && answer.text) command.text = answer.text;
            commands.run(command);
        }
        async function goToPage() {
            const editor = app.editor;
            if (!editor) return;
            const total = editor.getTotalPages(), current = editor.getCurrentPage('viewport') || 1;
            const answer = await ask(tr('go_page'), [{id: 'page', label: tr('page_count', {current, total}), type: 'number', value: current, min: 1, max: total, step: 1, required: true}]);
            if (answer && app.editor === editor) editor.scrollToPage(Math.max(1, Math.min(total, answer.page)));
        }
        function wordCount() {
            const editor = app.editor;
            if (!editor) return Promise.resolve(null);
            const text = editor.surface.session.bodyText();
            const selected = editor.snapshot().selectionCollapsed ? '' : editor.query({type: 'selectedText'}) || '';
            const rows = [[tr('pages'), editor.getTotalPages()], [tr('words'), app.countWords(text)], [tr('chars'), [...text].length],
                [tr('chars_no_spaces'), [...text.replace(/\s/g, '')].length]];
            if (selected.trim()) rows.push([tr('selected_words'), app.countWords(selected)]);
            return tell(tr('word_count_title'), rows.map(([label, value]) => [label, commands.format(value)]));
        }
        const shortcuts = () => tell(tr('shortcuts'), window.WriterCommands.definitions.filter(definition => definition.shortcut || definition.hint)
            .map(definition => [commands.label(definition.id), definition.shortcut || definition.hint]));
        return {table, link, goToPage, wordCount, shortcuts};
    }
    window.WriterDialogs = {create};
})();
```

- [ ] **Step 3: Load the module**

In `ui/js/desktop/core/module-loader.js`, in the `'writer'` entry, add `'/js/desktop/apps/writer-dialogs.js'` right after `'/js/desktop/apps/writer-toolbar.js'`.

In `ui/desktop_office_assets_test.go` add `js/desktop/apps/writer-dialogs.js` next to the entry for `writer-toolbar.js`, in the same form as that entry.

In the three fixtures (two in `ui/desktop_writer_browser_test.go`, one in `ui/desktop_writer_reload_browser_test.go`) add `<script src="/js/desktop/apps/writer-dialogs.js"></script>` right after the `writer-toolbar.js` script tag. Find every place with a text search for `writer-toolbar.js` under `ui/` (excluding `bundles/`); each list of Writer scripts must gain the new file.

Run `npm run build:ui`. Stage a bundle file only if `git status` shows it changed.

- [ ] **Step 4: Wire the dialogs into `writer.js`**

1. In the `commands` host object (the `window.WriterCommands.create({...})` call at the top of `render`), add a number format so labels and counts follow the user's locale. Replace
```js
            editor: () => editor, lib: () => lib, loading: () => loading, act: id => act(id), notice, refresh: () => refresh()});
```
with
```js
            editor: () => editor, lib: () => lib, loading: () => loading, act: id => act(id), notice, refresh: () => refresh(),
            format: value => Number(value).toLocaleString(locale)});
```

2. In the `state` object, replace the line that starts with `form:(title,fields)=>window.AuraFormDialog.open(` (the whole line) with
```js
            windowId,signal:()=>documentLife.signal,countWords:text=>countWords(text),
```

3. Right after `const toolbar = window.WriterToolbar.create(state);` add
```js
        const dialogs = window.WriterDialogs.create(state);
```

4. In `act`, replace the whole `case 'page':{...}` line (it calls `ctx.promptDialog(tr('go_page'), ...)`) with
```js
                    case 'page':case 'goToPage':await dialogs.goToPage();break;
                    case 'table':await dialogs.table();break;
                    case 'link':await dialogs.link();break;
                    case 'wordCount':await dialogs.wordCount();break;
                    case 'shortcuts':await dialogs.shortcuts();break;
```

- [ ] **Step 5: Let the panel buttons open the dialogs**

In `ui/js/desktop/apps/writer-panels.js`, replace the `case 'table_insert':{ ... }` block (seven lines, it calls `app.form('table',[`) with
```js
                case 'table_insert':await app.act('table');break;
```
and replace the line
```js
                case 'hyperlink':{const url=await app.prompt('hyperlink','https://');if(url)app.run({type:'insertHyperlink',href:url});break;}
```
with
```js
                case 'hyperlink':await app.act('link');break;
```

Check with a text search that nothing else reads `app.form` and that `app.prompt` still has readers (the comment prompt uses it).

- [ ] **Step 6: Run the tests**

Run: `npm run test:writer`, `npm run check:ui`, `go test ./ui -run "(?i)(materials|office|writer|virtualdesktop|lifecycle|i18n)" -count=1`
Expected: all pass; the Go run also compiles the new helper.

Controller: run the browser tests (see "Environment"). Expected: `--- PASS: TestDesktopWriterAppBrowser`. The link part of `verifyWriterDialogs` is the first runtime proof of `hyperlinkAt`, `removeHyperlink` and `insertHyperlink` with a `target`; report its message verbatim if it fails.

- [ ] **Step 7: Commit**

```bash
git add ui/js/desktop/apps/writer-dialogs.js ui/desktop_writer_menus_test.go ui/js/desktop/apps/writer.js ui/js/desktop/apps/writer-panels.js ui/js/desktop/core/module-loader.js ui/desktop_office_assets_test.go ui/desktop_writer_browser_test.go ui/desktop_writer_reload_browser_test.go
git commit -m "feat(writer): dialogs for table size, link, go to page, word count and shortcuts"
```

---

### Task 4: App actions: clipboard, zoom presets, Ctrl+wheel, tracking switch

**Files:**
- Modify: `ui/js/desktop/apps/writer.js`
- Modify: `ui/desktop_writer_menus_test.go`, `ui/desktop_writer_browser_test.go` (one call)

- [ ] **Step 1: Write the browser check**

Append to `ui/desktop_writer_menus_test.go`:

```go
// verifyWriterActions covers the app actions behind the Edit and View menus: paste from the
// system clipboard (stubbed), the tracking switch, zoom presets and Ctrl+wheel.
func verifyWriterActions(t *testing.T, page *rod.Page) {
	t.Helper()
	page.MustEval(`async()=>{` + writerWave1Prelude + `
        const own=Object.getOwnPropertyDescriptor(navigator,'clipboard');
        Object.defineProperty(navigator,'clipboard',{configurable:true,value:{
            read:async()=>[{types:['text/html','text/plain'],getType:async type=>new Blob([type==='text/html'?'<p><b>Rich paste</b></p>':'Rich paste'],{type})}],
            readText:async()=>'Plain paste'}});
        try {
            await app.act('paste');await wait(200);
            if(failed())throw Error('paste: '+failed());
            if(!body().includes('Rich paste'))throw Error('paste inserted '+JSON.stringify(body()));
            await app.act('pastePlain');await wait(200);
            if(failed())throw Error('paste without formatting: '+failed());
            if(!body().includes('Plain paste'))throw Error('plain paste inserted '+JSON.stringify(body()));
            editor.exec({type:'selectAll'});
            for(const type of ['copy','cut'])if(!editor.can({type}).ok)throw Error(type+' is refused on a selection: '+editor.can({type}).reason);
            await app.act('tracking');
            if(editor.snapshot().editingMode!=='suggesting')throw Error('tracking did not switch on');
            await app.act('tracking');
            if(editor.snapshot().editingMode!=='editing')throw Error('tracking did not switch off');
            await app.act('zoom150');
            if(Math.abs(editor.getZoom()-1.5)>.001)throw Error('zoom150 gave '+editor.getZoom());
            if(document.querySelector('[data-zoom]').textContent!=='150%')throw Error('the status bar shows '+document.querySelector('[data-zoom]').textContent);
            await app.act('zoomPage');await wait(300);
            const whole=editor.getZoom(),sheet=document.querySelector('.docx-page').getBoundingClientRect(),view=document.querySelector('[data-scroll]');
            if(sheet.height>view.clientHeight||sheet.width>view.clientWidth)throw Error('whole page does not fit: '+Math.round(sheet.width)+'x'+Math.round(sheet.height)+' in '+view.clientWidth+'x'+view.clientHeight);
            const turn=new WheelEvent('wheel',{deltaY:-100,ctrlKey:true,bubbles:true,cancelable:true});
            view.dispatchEvent(turn);
            if(!turn.defaultPrevented||Math.abs(editor.getZoom()-(whole+.1))>.011)throw Error('Ctrl+wheel: '+editor.getZoom()+' after '+whole);
            const scroll=new WheelEvent('wheel',{deltaY:-100,bubbles:true,cancelable:true});
            view.dispatchEvent(scroll);
            if(scroll.defaultPrevented)throw Error('a plain wheel must scroll');
            await app.act('fit');
        } finally {
            if(own)Object.defineProperty(navigator,'clipboard',own);else delete navigator.clipboard;
            rewind();
        }
        if(body().trim())throw Error('the document was not left blank: '+JSON.stringify(body()));
    }`)
}
```

In `TestDesktopWriterAppBrowser` add `verifyWriterActions(t, page)` right after `verifyWriterDialogs(t, page)`.

- [ ] **Step 2: Tell the registry which app actions are switched on**

In the `commands` host object in `writer.js`, replace
```js
            format: value => Number(value).toLocaleString(locale)});
```
with
```js
            format: value => Number(value).toLocaleString(locale), active: id => activeAction(id)});
```

- [ ] **Step 3: Add the helpers**

In `writer.js`, directly after the `setFocus` function, add:

```js
        // Whether an app action is switched on; menus show it as a check mark.
        function activeAction(id) {
            if(id==='outline' || id==='search')return leftPanel===id;
            if(['format','review','assist','insert'].includes(id))return rightPanel===id;
            if(id==='focus')return root.classList.contains('writer-focus');
            if(id==='tracking')return commands.tracking();
            return false;
        }
        // The engine rejects a zoom outside its range instead of clamping it.
        function zoomTo(value) {
            if(!editor)return;
            editor.setZoom(Math.max(.25,Math.min(3,Math.round(value*100)/100)));
            refresh();
        }
        // The engine fits the page width only; a whole page is the smaller of the two ratios.
        function wholePageZoom() {
            const sheet=root.querySelector('.docx-page'),view=find('[data-scroll]');
            if(!sheet || !view)return editor.getZoom();
            const box=sheet.getBoundingClientRect();
            return editor.getZoom()*Math.min((view.clientWidth-32)/box.width,(view.clientHeight-32)/box.height);
        }
        // A menu click carries no clipboard event, so the clipboard is read here. Tracking takes
        // text only, like the paste handler below.
        async function pasteFromClipboard(plain) {
            if(!editor)return;
            if(!navigator.clipboard?.readText)throw new Error(ctx.t('desktop.clipboard_read_unavailable'));
            let text='',html='';
            if(!plain && !commands.tracking() && navigator.clipboard.read) {
                try {
                    for(const entry of await navigator.clipboard.read()) {
                        if(entry.types.includes('text/html'))html=await (await entry.getType('text/html')).text();
                        if(entry.types.includes('text/plain'))text=await (await entry.getType('text/plain')).text();
                    }
                } catch(_) { html=''; }
            }
            if(!html && !text)text=await navigator.clipboard.readText();
            run(plain?{type:'pasteWithoutFormatting',text}:html?{type:'paste',text,html:sanitize(html)}:{type:'paste',text});
        }
```

- [ ] **Step 4: Add the actions**

In `act`, replace the line
```js
                    case 'zoomIn':case 'zoomOut':editor?.setZoom(Math.max(.25,Math.min(3,editor.getZoom()+(action==='zoomIn'?.1:-.1))));refresh();break;
```
with
```js
                    case 'zoomIn':case 'zoomOut':if(editor)zoomTo(editor.getZoom()+(action==='zoomIn'?.1:-.1));break;
                    case 'zoomPage':if(editor)zoomTo(wholePageZoom());break;
                    case 'zoomMenu':{
                        const box=find('[data-zoom]').getBoundingClientRect(),ids=['fit','zoomPage','zoom50','zoom75','zoom100','zoom125','zoom150','zoom200'];
                        if(ctx.showContextMenu)ctx.showContextMenu(box.left,box.top,ids.map(id=>Object.assign(commands.menuItem(id),{icon:'zoom-in'})));
                        else await act('fit');
                        break;
                    }
                    case 'tracking':run({type:'setEditingMode',mode:commands.tracking()?'editing':'suggesting'});break;
                    case 'paste':case 'pastePlain':await pasteFromClipboard(action==='pastePlain');break;
```
and, after the line `case 'html':case 'md':case 'txt':await exportSimple(action);break;`, add as the last clause of the `switch`:
```js
                    default:{const preset=/^zoom(\d+)$/.exec(action);if(preset && editor)zoomTo(Number(preset[1])/100);}
```

- [ ] **Step 5: Zoom button and Ctrl+wheel**

In the status-bar markup of `writer.js` (inside the long string after `button('focus')`), replace
```js
<button data-action="fit" data-zoom title="' + esc(tr('fit')) + '">100%</button>
```
with
```js
<button data-action="zoomMenu" data-zoom aria-haspopup="menu" title="' + esc(tr('zoom')) + '">100%</button>
```

After the `mount.addEventListener('dragover', ...)` line add:
```js
        find('[data-scroll]').addEventListener('wheel',event=>{
            if(!(event.ctrlKey || event.metaKey) || !editor)return;
            event.preventDefault();zoomTo(editor.getZoom()+(event.deltaY<0?.1:-.1));
        },{passive:false,signal:lifecycle.signal});
```

- [ ] **Step 6: Run the tests**

Run: `npm run test:writer`, `npm run check:ui`, `go test ./ui -run "(?i)(materials|office|writer|virtualdesktop|lifecycle|i18n)" -count=1`
Expected: all pass.

Controller: run the browser tests. Expected: `--- PASS: TestDesktopWriterAppBrowser`.

- [ ] **Step 7: Commit**

```bash
git add ui/js/desktop/apps/writer.js ui/desktop_writer_menus_test.go ui/desktop_writer_browser_test.go
git commit -m "feat(writer): clipboard actions, zoom presets with Ctrl+wheel and a tracking switch"
```

---

### Task 5: Complete window menus that follow the document state

**Files:**
- Create: `ui/js/desktop/apps/writer-menus.js`, `scripts/test-writer-menus.mjs`
- Modify: `ui/js/desktop/apps/writer.js`, `package.json`
- Modify: `ui/js/desktop/core/module-loader.js`, `ui/desktop_office_assets_test.go`, three fixtures, `ui/desktop_office_apps_test.go`
- Modify: `ui/desktop_writer_menus_test.go`, `ui/desktop_writer_browser_test.go` (one call)

- [ ] **Step 1: Write the failing node test**

Create `scripts/test-writer-menus.mjs`:

```js
import assert from 'node:assert/strict';
import vm from 'node:vm';
import {readFileSync} from 'node:fs';

const context = {window: {}};
vm.runInNewContext(readFileSync('ui/js/desktop/apps/writer-commands.js', 'utf8'), context);
vm.runInNewContext(readFileSync('ui/js/desktop/apps/writer-menus.js', 'utf8'), context);
const {WriterCommands, WriterMenus} = context.window;
const flat = items => items.flatMap(item => item.items ? flat(item.items) : [item]);

// The model names registry commands only, and every command is reachable from a menu or the toolbar.
const used = [];
const menus = WriterMenus.build(id => { used.push(id); return {id}; }, key => 'tr:' + key, {file: [{id: 'cloud'}]});
assert.equal(menus.map(menu => menu.id).join(), 'file,edit,view,insert,format,table,tools,agent');
const known = new Set(WriterCommands.definitions.map(definition => definition.id));
for (const id of used) assert.ok(known.has(id), 'menu item ' + id + ' is not a registry command');
const absent = WriterCommands.definitions.map(definition => definition.id).filter(id => !used.includes(id));
assert.equal(absent.join(), 'style,fontFamily,fontSize,lineSpacing,textColor,highlight,insert', 'only toolbar controls and the insert panel stay out of the menus');
assert.ok(flat(menus[0].items).some(item => item.id === 'cloud'), 'the app-supplied file items are placed');
for (const menu of menus) {
    assert.ok(menu.labelKey || String(menu.label).startsWith('tr:'), menu.id + ' has a title');
    const ids = flat(menu.items).filter(item => item.id).map(item => item.id);
    assert.equal(new Set(ids).size, ids.length, menu.id + ' lists no command twice');
    for (const item of menu.items) if (item.items) assert.ok(String(item.label).startsWith('tr:'), item.id + ' submenu has a title');
}
assert.equal(menus.at(-1).items[0].id, 'assist');

// The bar is rebuilt only when the signature changes, and an engine command hands the focus back.
const log = {set: 0, focused: 0, ran: []};
let signature = 'a';
const app = {
    windowId: 'w1', tr: key => key, editor: {focus() { log.focused++; }},
    ctx: {setWindowMenus(id, model) { log.set++; log.window = id; log.model = model; }},
    commands: {
        get: id => WriterCommands.definitions.find(definition => definition.id === id),
        menuItem: id => ({id, action: () => log.ran.push(id)})
    },
    menuSignature: () => signature, fileMenuItems: () => []
};
const bar = WriterMenus.create(app);
bar.sync();
bar.sync();
assert.equal(log.set, 1);
assert.equal(log.window, 'w1');
signature = 'b';
bar.sync();
assert.equal(log.set, 2);
bar.sync(true);
assert.equal(log.set, 3, 'a forced sync always rebuilds');
const find = id => flat(log.model.flatMap(menu => menu.items)).find(item => item.id === id);
find('bold').action();
assert.equal(log.ran.join(), 'bold');
assert.equal(log.focused, 1, 'an engine command returns the focus to the document');
find('copy').action();
assert.equal(log.focused, 2);
find('wordCount').action();
assert.equal(log.focused, 2, 'an app action keeps the focus where its dialog puts it');
console.log('Writer menus: model, coverage and rebuild rule pass');
```

In `package.json`, append ` && node scripts/test-writer-menus.mjs` to the `test:writer` script.

Run: `node scripts/test-writer-menus.mjs`
Expected: FAIL with `ENOENT ... writer-menus.js`.

- [ ] **Step 2: Write the menus**

Create `ui/js/desktop/apps/writer-menus.js`. The compact object style is deliberate: two Go tests look for the literal strings `item('new')`, `item('saveAs')`, `id:'agent'`, `labelKey:'desktop.menu_agent'` and `item('assist')`.

```js
(function () {
    'use strict';
    const SEP = {type:'separator'};
    // The window menus. item(id) turns a registry command into a menu item; extra carries the items
    // only writer.js can build (cloud import and export). A command no surface offers fails
    // scripts/test-writer-menus.mjs.
    function build(item, tr, extra) {
        return [
            {id:'file',labelKey:'desktop.menu_file',items:[item('new'),item('open'),item('save'),item('saveAs'),SEP,...extra.file,SEP,
                item('docx'),item('html'),item('md'),item('txt'),SEP,item('print')]},
            {id:'edit',labelKey:'desktop.menu_edit',items:[item('undo'),item('redo'),SEP,item('cut'),item('copy'),item('paste'),item('pastePlain'),SEP,
                item('selectAll'),SEP,item('search'),item('goToPage')]},
            {id:'view',label:tr('view'),items:[item('outline'),item('format'),item('review'),SEP,item('marks'),
                {id:'markup',label:tr('markup'),items:[item('markupAll'),item('markupFinal'),item('markupOriginal')]},SEP,
                {id:'zoom',label:tr('zoom'),items:[item('fit'),item('zoomPage'),SEP,item('zoom50'),item('zoom75'),item('zoom100'),item('zoom125'),item('zoom150'),item('zoom200'),SEP,
                    item('zoomIn'),item('zoomOut')]},
                item('focus')]},
            {id:'insert',label:tr('insert'),items:[item('table'),item('image'),item('qr'),item('link'),SEP,
                item('pageBreak'),item('sectionNext'),item('sectionContinuous'),SEP,item('footnote'),item('endnote'),SEP,item('toc'),item('textBox'),SEP,
                item('header'),item('footer'),item('pageNumber'),item('backDocument')]},
            {id:'format',label:tr('format'),items:[
                {id:'text',label:tr('text'),items:[item('bold'),item('italic'),item('underline'),item('strike'),SEP,item('superscript'),item('subscript')]},
                {id:'paragraph',label:tr('paragraph'),items:[item('alignLeft'),item('alignCenter'),item('alignRight'),item('alignJustify'),SEP,
                    item('bullets'),item('numbering'),SEP,item('indent'),item('outdent')]},
                {id:'spacing',label:tr('line_spacing'),items:[item('spacing100'),item('spacing115'),item('spacing150'),item('spacing200')]},SEP,
                item('copyFormat'),item('pasteFormat'),item('clearFormat')]},
            {id:'table',label:tr('table'),items:[item('table'),SEP,item('rowAbove'),item('rowBelow'),item('columnLeft'),item('columnRight'),SEP,
                item('deleteRow'),item('deleteColumn'),item('deleteTable'),SEP,item('mergeCells'),item('splitCell')]},
            {id:'tools',label:tr('tools'),items:[item('wordCount'),SEP,item('tracking'),item('review'),item('previousChange'),item('nextChange'),SEP,item('shortcuts')]},
            {id:'agent',labelKey:'desktop.menu_agent',items:[item('assist')]}
        ];
    }
    // app: the state facade of writer.js ({ctx, tr, commands, windowId, editor, menuSignature(), fileMenuItems()}).
    function create(app) {
        const {ctx, tr, commands} = app;
        let signature = null;
        // A menu click leaves the focus in the menu bar; after an engine command it belongs to the document again.
        function item(id) {
            const entry = commands.menuItem(id), definition = commands.get(id), action = entry.action;
            if (definition.slot || definition.command) entry.action = () => { action(); app.editor?.focus(); };
            return entry;
        }
        // The shell reads disabled and checked when the menus are set, and setting them closes an
        // open menu, so the bar is rebuilt only when something it shows has changed.
        function sync(force) {
            const next = app.menuSignature();
            if (!force && next === signature) return;
            signature = next;
            ctx.setWindowMenus?.(app.windowId, build(item, tr, {file: app.fileMenuItems()}));
        }
        return {sync};
    }
    window.WriterMenus = {create, build};
})();
```

Run: `node scripts/test-writer-menus.mjs`
Expected: `Writer menus: model, coverage and rebuild rule pass`

- [ ] **Step 3: Load the module**

Add `'/js/desktop/apps/writer-menus.js'` to the `'writer'` entry of `ui/js/desktop/core/module-loader.js` right after `writer-dialogs.js`, the matching entry to `ui/desktop_office_assets_test.go`, and `<script src="/js/desktop/apps/writer-menus.js"></script>` after the `writer-dialogs.js` tag in the three fixtures. Run `npm run build:ui`; stage a bundle only if `git status` shows it changed.

- [ ] **Step 4: Move the menus out of `writer.js`**

1. In the `state` object, extend the line added in Task 3 to
```js
            windowId,signal:()=>documentLife.signal,countWords:text=>countWords(text),menuSignature,fileMenuItems,
```

2. After `const dialogs = window.WriterDialogs.create(state);` add
```js
        const menus = window.WriterMenus.create(state);
```

3. Replace the whole `function setMenus() { ... }` (from `function setMenus() {` to its closing brace, 16 lines) with
```js
        function fileMenuItems() {
            return [{id:'gog-import',label:ctx.t('desktop.gog.import'),disabled:ctx.readonly,action:()=>gogOffice('import')},
                {id:'gog-export',label:ctx.t('desktop.gog.export'),disabled:ctx.readonly||!editor,action:()=>gogOffice('export')}];
        }
        // Everything a menu shows as enabled or checked. The bar is rebuilt when this changes.
        function menuSignature() {
            const s=editor && !loading?editor.snapshot():null;
            return JSON.stringify([!!s,!!ctx.readonly,s?.editingMode,s?.reviewDisplayMode,!!s?.showParagraphMarks,!!s?.canUndo,!!s?.canRedo,!!s?.selectionCollapsed,
                !!s?.table,!!s?.image,s?.scope?.kind,s?.formatting?.lineSpacing?.value,leftPanel,rightPanel,root.classList.contains('writer-focus'),
                !!s?.table && can({type:'mergeCells'}).ok,!!s && can({type:'pasteFormatting'}).ok]);
        }
```

4. Replace the two remaining calls. `toolbar.fill();setMenus();refresh();` becomes `toolbar.fill();menus.sync(true);refresh();`, and the last statement of `render`, `setMenus();void load(path,ctx.path?null:'blank');`, becomes `menus.sync(true);void load(path,ctx.path?null:'blank');`.

5. In `refresh()`, right after `toolbar.refresh();`, add `menus.sync();`.

6. In `setFocus`, add `menus.sync();` as its last statement (`showPanel` already ends in `refresh()`).

A text search for `setMenus` in `writer.js` must find nothing afterwards.

- [ ] **Step 5: Point the marker tests at the new file**

In `ui/desktop_office_apps_test.go`:

- `TestOfficeAppsExposeNewAndSaveAsMenus`: remove `"item('new')",` and `"item('saveAs')",` from the `writer` case and add a case after it:
```go
		{
			name:    "writer menus",
			file:    "writer-menus.js",
			markers: []string{"item('new')", "item('saveAs')"},
		},
```
- `TestEditorWriterAndSheetsExposeAgentMenus`: change the `writer` case's `path` to `"js/desktop/apps/writer-menus.js"`.

- [ ] **Step 6: Browser check**

Append to `ui/desktop_writer_menus_test.go`:

```go
// verifyWriterMenus reads the menus the fixture's setWindowMenus received: their structure,
// which shortcuts are only shown, and that checked and disabled follow the document.
func verifyWriterMenus(t *testing.T, page *rod.Page) {
	t.Helper()
	page.MustEval(`async()=>{` + writerWave1Prelude + `
        const flat=items=>items.flatMap(entry=>entry.items?flat(entry.items):[entry]);
        const item=id=>flat(window.menus.flatMap(menu=>menu.items)).find(entry=>entry.id===id);
        const titles=window.menus.map(menu=>menu.id).join();
        if(titles!=='file,edit,view,insert,format,table,tools,agent')throw Error('menus: '+titles);
        for(const id of ['cut','copy','paste','pastePlain','selectAll','goToPage','marks','markupAll','zoomPage','zoom125','link','endnote','sectionNext','textBox','superscript','spacing150','copyFormat','rowAbove','deleteTable','wordCount','tracking','nextChange','shortcuts','assist','gog-import'])
            if(!item(id))throw Error('menu lacks '+id);
        if(item('copy').shortcut||item('copy').shortcutHint!=='Ctrl+C')throw Error('copy must only show its shortcut');
        if(item('link').shortcut!=='Ctrl+K')throw Error('link shortcut: '+item('link').shortcut);
        if(!item('rowAbove').disabled)throw Error('row commands need a table');
        if(!item('markupAll').checked||item('marks').checked||item('tracking').checked)throw Error('initial view state is wrong');
        try {
            item('marks').action();await wait(250);
            if(!item('marks').checked)throw Error('formatting marks are not checked after switching them on');
            item('marks').action();await wait(250);
            if(item('marks').checked)throw Error('formatting marks stay checked');
            item('tracking').action();await wait(250);
            if(!item('tracking').checked||!item('table').disabled)throw Error('tracking must check its item and disable structural inserts');
            item('tracking').action();await wait(250);
            if(item('tracking').checked||item('table').disabled)throw Error('tracking did not switch off in the menus');
            item('outline').action();await wait(250);
            if(!item('outline').checked)throw Error('an open panel is not checked');
            item('outline').action();await wait(250);
            editor.exec({type:'insertTable',rows:2,cols:2});await wait(300);
            if(item('rowAbove').disabled)throw Error('row commands stay disabled inside a table');
            item('rowBelow').action();await wait(250);
            if(failed())throw Error('row below: '+failed());
            if(editor.getSelectedTable()?.rowCount!==3)throw Error('row below gave '+JSON.stringify(editor.getSelectedTable()));
        } finally { rewind(); }
        if(body().trim())throw Error('the document was not left blank: '+JSON.stringify(body()));
    }`)
}
```

In `TestDesktopWriterAppBrowser` add `verifyWriterMenus(t, page)` right after `verifyWriterActions(t, page)`.

- [ ] **Step 7: Run the tests**

Run: `npm run test:writer`
Expected: four `... pass` lines, the last one `Writer menus: model, coverage and rebuild rule pass`.

Run: `npm run check:ui` and `go test ./ui -run "(?i)(materials|office|writer|virtualdesktop|lifecycle|i18n)" -count=1`
Expected: pass.

Controller: run the browser tests. Expected: `--- PASS: TestDesktopWriterAppBrowser`.

- [ ] **Step 8: Commit**

```bash
git add ui/js/desktop/apps/writer-menus.js scripts/test-writer-menus.mjs package.json ui/js/desktop/apps/writer.js ui/js/desktop/core/module-loader.js ui/desktop_office_assets_test.go ui/desktop_office_apps_test.go ui/desktop_writer_menus_test.go ui/desktop_writer_browser_test.go ui/desktop_writer_reload_browser_test.go
git commit -m "feat(writer): complete window menus that follow the document state"
```

---

### Task 6: Toolbar additions: clear formatting, line spacing, link, table, image

**Files:**
- Modify: `ui/js/desktop/apps/writer-toolbar.js`, `ui/css/desktop-app-writer.css`, `scripts/test-writer-toolbar.mjs`

- [ ] **Step 1: Extend the node test**

In `scripts/test-writer-toolbar.mjs`, after the line `assert.equal(WriterToolbar.icons({}, value => String(value))('undo', '↶'), '↶');` add:

```js
// Wave 1a additions sit in the bar, and their glyphs are drawn locally.
for (const id of ['clearFormat', 'lineSpacing', 'link', 'table', 'image']) assert.ok(placed.includes(id), id + ' is in the toolbar');
for (const key of ['table', 'clear-format']) assert.ok(icon(key, '?').startsWith('<svg class="writer-glyph"'), key + ' is a local glyph');
```

Run: `node scripts/test-writer-toolbar.mjs`
Expected: FAIL with `clearFormat is in the toolbar`.

- [ ] **Step 2: Glyphs and groups**

In `ui/js/desktop/apps/writer-toolbar.js`:

Replace the `'more'` line of `GLYPHS` with
```js
        'more': '<circle cx="5" cy="12" r="1.3"/><circle cx="12" cy="12" r="1.3"/><circle cx="19" cy="12" r="1.3"/>',
        'table': '<rect x="4" y="5" width="16" height="14" rx="1.5"/><path d="M4 10h16M4 14.5h16M10 5v14"/>',
        'clear-format': '<path d="M5 5h11M10.5 5l-2.5 12"/><path d="M14.5 14.5l5 5M19.5 14.5l-5 5"/>'
```

Replace the `marks`, `alignment` and `insert` lines of `GROUPS` with
```js
        {id: 'marks', priority: 7, items: ['bold', 'italic', 'underline', 'strike', 'clearFormat']},
        {id: 'alignment', priority: 4, items: ['alignLeft', 'alignCenter', 'alignRight', 'alignJustify', 'lineSpacing']},
```
```js
        {id: 'insert', priority: 6, className: 'writer-tool-tail', items: ['link', 'table', 'image', 'insert']}
```
(the `lists` and `colors` lines between them stay as they are).

- [ ] **Step 3: A select with fixed options**

In `control(id)`, replace
```js
            if (definition.control === 'select') return '<select ' + attrs(id) + ' aria-label="' + name + '"></select>';
```
with
```js
            // A select with options is complete here; the style and font lists are filled per document.
            // The hidden empty option is what the select shows when the paragraph has another spacing.
            if (definition.control === 'select') return '<select ' + attrs(id) + ' aria-label="' + name + '"' + (definition.options ? ' title="' + name + '"' : '') + '>' +
                (definition.options ? '<option value="" hidden></option>' + definition.options.map(value => '<option value="' + value + '">' + esc(commands.format(value)) + '</option>').join('') : '') + '</select>';
```

- [ ] **Step 4: State of the new controls**

In `refresh()`, replace
```js
                    el.value = slot === 'styles.style' ? fmt.styleId || 'Normal' : slot === 'font.family' ? fmt.fontFamily || 'Calibri' : fmt.fontSizePt || 11;
```
with
```js
                    el.value = slot === 'styles.style' ? fmt.styleId || 'Normal' : slot === 'font.family' ? fmt.fontFamily || 'Calibri'
                        : slot === 'list.lineSpacing' ? (fmt.lineSpacing?.rule === 'multiple' ? String(fmt.lineSpacing.value) : '') : fmt.fontSizePt || 11;
```
and replace
```js
                setTitle(el, allowed.ok ? '' : unavailable(allowed.reason));
            }
            rove();
```
with
```js
                setTitle(el, allowed.ok ? (commands.get(id).options ? commands.label(id) : '') : unavailable(allowed.reason));
            }
            // App actions in the bar (link, table, image) follow the registry state too.
            for (const el of wrap.querySelectorAll('button[data-action]')) {
                const state = commands.state(el.dataset.tool);
                el.disabled = !state.enabled;
                setTitle(el, state.enabled ? commands.tooltip(el.dataset.tool) : commands.tooltip(el.dataset.tool) + ' · ' + unavailable(state.reason));
            }
            rove();
```

- [ ] **Step 5: CSS**

In `ui/css/desktop-app-writer.css`, after the line that starts with `.writer-fonts input{`, add:
```css
.writer-toolbar-wrap select[data-slot="list.lineSpacing"]{width:64px;height:30px;padding:3px 6px;font-variant-numeric:tabular-nums}
```

- [ ] **Step 6: Run the tests**

Run: `npm run test:writer`, `npm run check:ui`, `go test ./ui -run "(?i)(materials|office|writer|virtualdesktop|lifecycle|i18n)" -count=1`
Expected: all pass.

Controller: run the browser tests. `verifyWriterToolbarOverflow` requires the whole bar to fit a 1366 px window (More button hidden, popover empty). If it fails there, the bar has become too wide: take `'clearFormat'` out of the `marks` group again (it stays in Format > Clear formatting), remove it from the list in Step 1's first assertion, and re-run. Report that this was needed.

Then look at `reports/autor/default-dark-format.png` and `fruity-light-format.png`: the five new controls are visible, the line-spacing select shows a value, nothing wraps or overlaps.

- [ ] **Step 7: Commit**

```bash
git add ui/js/desktop/apps/writer-toolbar.js ui/css/desktop-app-writer.css scripts/test-writer-toolbar.mjs
git commit -m "feat(writer): toolbar gains clear formatting, line spacing, link, table and image"
```

---

### Task 7: Documentation and final verification

**Files:**
- Modify: `ui/js/desktop/apps/AGENTS.md`, `documentation/autor.md`

- [ ] **Step 1: `ui/js/desktop/apps/AGENTS.md`**

In the bullet that starts with "`writer.js` owns Autor's native DOCX document":

- After "`writer-toolbar.js` renders the toolbar from that table and owns its overflow popover and roving tabindex;" insert:
  "`writer-menus.js` builds the window menus from it (`build`) and sets them again only when `menuSignature()` in `writer.js` changes, because the shell reads `disabled` and `checked` when menus are set and setting them closes an open menu; `writer-dialogs.js` holds the dialogs (table size, link, go to page, word count, shortcut overview), all through `shared/form-dialog.js`;"
- Replace the sentence that starts with "A new command is one entry in `DEFINITIONS`" and ends with "a toolbar control is its id in a `GROUPS` entry." with:
  "A new command is one entry in `DEFINITIONS`: a `slot`, a `command` (an engine command without a slot), or neither for an app action, which also needs its `case` in `act` in `writer.js`. A window menu lists it in `build` in `writer-menus.js`; a toolbar control is its id in a `GROUPS` entry; `scripts/test-writer-menus.mjs` fails for a command no surface offers. `icon` is a shared key for menus and the toolbar, `glyph` is drawn by `writer-toolbar.js` only. Clipboard and engine-owned shortcuts are `hint`, never `shortcut`: the shell dispatches a menu `shortcut` itself."
- Replace "`shared/form-dialog.js` (`window.AuraFormDialog`) is the multi-field dialog;" with:
  "`shared/form-dialog.js` (`window.AuraFormDialog`) is the multi-field dialog, with `static` rows for read-only information, a `grid` that fills two number fields, and `extraLabel` for a third button;"

If a quoted sentence is worded differently in the file, edit the sentence that says the same thing and keep the rest of the bullet.

- [ ] **Step 2: `documentation/autor.md`**

In the section "Writing", after the sentence that ends with "In the size field they move the caret, and Tab steps to the next control." add:

```
The bar also holds clear formatting, line spacing, link, table and image.
The window menus cover every command: File, Edit (including cut, copy, paste,
paste without formatting, select all, find and go to page), View (panels,
formatting marks, how tracked changes are shown, zoom, focus mode), Insert
(table, image, QR code, link, page and section breaks, footnote, endnote,
table of contents, text box, header, footer, page numbers), Format (text,
paragraph, line spacing, copy and paste formatting, clear formatting), Table
(rows, columns, merge, split, delete) and Tools (word count, track changes,
previous and next change, keyboard shortcuts). Superscript and subscript are
in Format > Text. Ctrl+K opens the link dialog, which inserts, changes or
removes a link; Ctrl+G goes to a page. Paste from the menu needs the
browser's clipboard permission; Ctrl+V always works.
The zoom button in the status bar offers fit to width, whole page and fixed
steps; Ctrl with the mouse wheel zooms as well.
```

In the same section replace the sentence "A table is inserted from a dialog that asks for rows (1 to 100) and columns (1 to 30)." with:

```
A table is inserted from a dialog with a size grid and fields for rows
(1 to 100) and columns (1 to 30).
```

In "Build and verification", in the bullet about `npm run test:writer`, add `the window menu model (`scripts/test-writer-menus.mjs`)` to the list of what it checks.

- [ ] **Step 3: Full verification**

Run, in this order:

1. `npm run test:writer`: four `... pass` lines.
2. `npm run check:ui`: ends with `Ocean chat sea life renderer verified`.
3. `go test ./ui -count=1` (no browser): the only failure allowed is `TestDesktopCodeStudioRunUsesTerminalSession`.
4. Controller: delete `reports\autor`, then `AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run "TestDesktop(Writer|QR)" -count=1 -timeout 30m -v`: every `TestDesktopWriter*` passes.
5. `git status --short`: only intended files.
6. Line counts: every file under `ui/js/desktop/apps/writer*.js` and `ui/js/desktop/shared/form-dialog.js` is below 1100 lines.

- [ ] **Step 4: Commit**

```bash
git add ui/js/desktop/apps/AGENTS.md documentation/autor.md
git commit -m "docs(writer): menus, dialogs, zoom and the registry kinds"
```

---

## What changes for the user

- Eight window menus instead of five; every Writer command is in one of them. Check marks show open panels, formatting marks, the change display, tracking and the line spacing.
- Edit has cut, copy, paste, paste without formatting and select all. Menu paste needs the browser's clipboard permission.
- Insert table opens a dialog with a size grid. Link opens a dialog that inserts, changes or removes a link and accepts web, mail, phone and in-document addresses only.
- "Go to page" is a dialog (also Ctrl+G, and the page button in the status bar). Tools has a word count and a shortcut overview.
- The zoom button opens presets instead of jumping to "fit to width"; Ctrl+wheel zooms.
- The toolbar gains clear formatting, line spacing, link, table and image.
- The Edit menu no longer lists Review, Format and Insert (they are in View, Tools and their own menus).

## Out of scope for this plan

- Right-click menu and contextual bar (part 1b); start state and recent documents (part 1c).
- The one-click format painter, "simple markup", arrow-key navigation inside the toolbar's overflow popover.
- Custom line spacing values in the toolbar (the Format panel has the number field).
- Page setup, paragraph and header/footer dialogs (Wave 2).
