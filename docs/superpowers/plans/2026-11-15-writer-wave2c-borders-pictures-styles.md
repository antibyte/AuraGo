# Writer Wave 2c — table borders, pictures, styles gallery: implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Table borders get a line style, a width and a colour and can be set edge by edge for the selected cells; a picture gets a height, an aspect lock, a crop and "Replace image…"; the style list in the toolbar shows every paragraph style the way it looks.

**Architecture:** One new pure module, `writer-objects.js`, turns what the Format panel holds (the border pen, a picture's size and crop) into engine commands; the panel sections "Table", "Image" and the new "Crop" use it. `borders` in the command registry takes the pen through the host. Replacing a picture is an app action in `writer.js` with its own hidden file input. The style list stays the toolbar's `<select>`; where the browser has the customizable select (`appearance: base-select`, Chrome and Edge), each option carries the engine's preview data as an inline style.

**Tech stack:** Vanilla JS classic scripts loaded by `module-loader.js`, `@docx-editor.dev/core` 2.29.0 behind `lib`, node `vm` tests, Go + rod browser tests.

Roadmap: `docs/superpowers/specs/2026-11-12-writer-office-roadmap-design.md`, section "Wave 2 — Business documents", part 2c.

---

## Read this first

**Decided by the user (2026-11-15):** borders act on the selected cells (the cell at the caret without a selection), not on "the whole table"; repeat header row, table alignment and picture position are left out because the engine refuses them; the styles gallery is the toolbar's style list with previews, not a panel; the pen offers the four line styles the engine paints on every edge.

**Scope.** In: the border pen (style, width, colour) and eight edge buttons in the Format panel; `borders` in the context bar and the Table menu drawn with the pen; picture height, aspect lock, crop in percent, reset crop, "Replace image…" (panel, context bar, right-click, Format > Image); the wrap list disabled where the engine refuses it; previews in the style list. Out: borders for a whole table without a cell selection; dashed and dotted lines; repeat header row; table alignment; picture position; flip; natural size; vertical cell alignment; character and table styles; creating or changing styles.

**Engine behaviour, probed in the browser on 2026-11-15.** These are facts, not assumptions:

- `{type:'setTableBorders', scope, spec:{style, size, color:{kind:'hex', value:'RRGGBB'}}}` with `scope` one of `all`, `outside`, `inside`, `top`, `bottom`, `left`, `right`; `{type:'setTableBorders', scope:'none', target:'all'}` removes. `style` is one of `single`, `dashed`, `dotted`, `double`, `triple`, `thick` (anything else: "border style is not allowlisted"). `size` is in eighths of a point.
- The command acts on the cell selection. With a caret, or a text selection that merely spans cells, that is the one cell at the selection's head. Dragging the mouse across cells makes a real cell selection (`editor.getTableCellSelection()` gives `{tableId, rows:{from,to}, columns:{from,to}, cellIds}`); then `outside` draws the outline of the selected block, `inside` its inner lines and `none` clears all of it, each as one undo step.
- A cell selection cannot be made by program: `selectTableRegion` is refused ("not supported by the tree editor"), and `surface.setCellSelection` with the public shape leaves the editor in a broken state. Looping over the cells works (9 cells in 91 ms) but is one undo step per cell: `editor.beginHistoryGroup()` groups only `toggleMark`, `setMarkAttr`, `setAlignment`, `setParagraphDirection`, `clearFormatting`, `setLineSpacing`, `setParagraphSpacing`, `setParagraphStyle` and `setParagraphFormat`.
- A dashed or dotted line on a cell's left edge is painted only where that edge had a visible line before. After "No borders" it is missing, while the top, right, bottom and inner lines appear (probed for `outside`, `all` and `left` on a nine-cell selection). Solid, double, triple and thick lines are painted on every edge, so the pen offers those four.
- `toggleHeaderRow` and `setTableProperties` are refused by `exec` and by `lib.tableCommand`; no command sets a table's alignment. `setTableCellVerticalAlignment` works and is not part of this plan.
- Painted borders are elements, not CSS borders: `.docx-table-cell .docx-table-border-edge-stroke[data-edge]` for one line (a dashed or dotted line carries `border-top` or `border-left` with the style, a solid one a background colour) and `.docx-table-border-double-stroke` for double lines.
- `{type:'setImageProperties', widthEmu, heightEmu}` sets either or both, without keeping proportions. `crop:{left, top, right, bottom}` is in percent of the picture; a side of 100 or two opposite sides that sum to 100 or more are refused ("crop values are out of range"). `resetToNaturalSize` answers "natural size is unavailable for this image".
- `editor.snapshot().image` and `editor.getSelectedImage()` carry `widthEmu`, `heightEmu`, `crop`, `rotationDegrees`, `wrap`, `kind`, `canChangeWrap`, `canCrop`, `canResize`, `intrinsic`. A picture inserted by Writer is `kind:'inline'` with `canChangeWrap:false`: `setImageWrapType` and `setImagePosition` answer "locked". Only a picture that is already anchored (from an imported document) can change its wrap.
- `editor.executeImageCommand({type:'replaceImage', data})` replaces the selected picture's file and keeps its size; `exec` refuses the command as asynchronous. Bytes in another format than the picture has are refused with code `invalidArgs` ("invalid-image"): a PNG picture takes PNG bytes only.
- `editor.getDocumentStyles()` gives `{styleId, name, type, preview:{fontFamily, fontSizePt, bold, italic, color}}`; a blank document has 16 paragraph styles (Title 28 pt, Heading 1 16 pt in `2F5496`, Quote italic, …). Only paragraph styles can be applied.

**Dry run, 2026-11-15.** A script applied this plan to the worktree exactly as written: 4 files created, all 39 "replace … with" pairs matched once, the three fixture tags replaced, the language script added 16 keys in each of the 16 files and nothing on a second run. Then:

- `npm run test:writer` (ten lines) and `npm run check:ui` passed; `go vet ./ui` was clean.
- `TestDesktop(Writer|QR)` passed in the browser, with the new check: a dragged nine-cell selection, "No borders", "Outside" (12 lines, none in the middle cell), "Inside", one undo, a double line, the pen kept across a closed panel; a picture resized locked and unlocked, cropped, the refused crop, "Reset crop", the wrap list off, a square JPEG put in place of a 4:3 PNG; the style previews and a style applied from the list.
- `reports/autor/styles-gallery.png` showed the open list with every style in its own look on a light ground, `reports/autor/objects-panel.png` the pen, the edge buttons and the picture fields.
- `go test ./ui -count=1 -skip GoldfishAdventureHotspot` passed.

The dry run was reverted. Implement the plan as it stands; a block that does not match means the file changed since, so stop and report instead of adapting it. What the dry run and its probes changed is already in the plan: `Mouse.MustDown` and `MustUp` take the button only; the pen has four line styles, because a dashed or dotted outline came out without its left edge.

**Not verifiable here:** how Word shows the borders and the crop of a saved document, and the plain style list in Firefox and Safari (the tests run in Chrome).

**Repo rules:**

- Read the `AGENTS.md` chain for every path you touch before editing (root, `ui/AGENTS.md`, `ui/js/desktop/apps/AGENTS.md`, `scripts/AGENTS.md`); close out with a DOX pass (Task 7).
- Every user-visible string exists in all 16 files under `ui/lang/desktop/`. `scripts/test-writer-commands.mjs` scans every registry label and every literal `tr('key')` in `ui/js/desktop/apps/writer*.js`.
- Every file under `ui/js/desktop` stays below 1100 lines. No Internet assets, no new dependencies, no binary files.
- `writer.js`, `writer-panels.js`, `writer-commands.js`, `writer-menus.js`, `writer-context.js`, `writer-toolbar.js`, `module-loader.js` and the scripts under `scripts/` may have CRLF line endings; check with a byte count, keep each file's line ending, new files are LF.
- A command is one entry in `DEFINITIONS` in `writer-commands.js`; `scripts/test-writer-menus.mjs` fails for a command no window menu lists. An app action also needs its `case` in `act` in `writer.js`.
- Temporary scripts go into `disposable/_writer-wave2c/` and are deleted before the commit. Before each commit: `git diff --cached --check`, and look at `git diff --cached`.
- GitNexus: the index is hundreds of commits behind and cannot index worktrees. Controller's impact analysis for `command` and `state` in `writer-commands.js`, `change` and `update` in `writer-panels.js`, `fill` in `writer-toolbar.js` and `act` in `writer.js`: UNKNOWN, callers confirmed by text search (all inside the Writer modules). End each commit body with: `GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.`

**Environment (Windows):**

- Work in the worktree `.claude/worktrees/writer-wave2c` (it has a real copy of `node_modules`).
- In a worktree session git commands must be plain and single (no `&&`, no `cd`).
- Browser tests: `AURAGO_RUN_BROWSER_SMOKE=1`, from PowerShell with the sandbox off. Delete `reports\autor` first. Implementer subagents cannot start Chrome; the controller runs these.

## File structure

| File | Change | Responsibility |
| --- | --- | --- |
| `ui/lang/desktop/*.json` | modify | 16 new keys in 16 languages |
| `ui/js/desktop/apps/writer-objects.js` | create | The border pen and its commands, a picture's size and crop commands |
| `scripts/test-writer-objects.mjs`, `package.json` | create, modify | Its test, part of `npm run test:writer` |
| `ui/js/desktop/core/module-loader.js`, `ui/desktop_office_assets_test.go`, three fixtures | modify | One new script |
| `ui/js/desktop/apps/writer-commands.js` | modify | `borders` takes the pen; `imageReplace`; `needsImage` |
| `ui/js/desktop/apps/writer-panels.js` | modify | Pen and edge buttons, picture height, lock, crop |
| `ui/js/desktop/apps/writer-context.js`, `writer-menus.js` | modify | "Replace image…" where the picture commands are |
| `ui/js/desktop/apps/writer.js` | modify | The pen for the registry; `replaceImage` |
| `ui/js/desktop/apps/writer-toolbar.js`, `ui/css/desktop-app-writer.css` | modify | Style previews |
| `scripts/test-writer-commands.mjs`, `test-writer-context.mjs`, `test-writer-toolbar.mjs` | modify | New rules |
| `ui/desktop_writer_objects_test.go`, `ui/desktop_writer_browser_test.go` | create, modify | Browser checks |
| `ui/js/desktop/apps/AGENTS.md`, `documentation/autor.md` | modify | Contracts and user documentation |

---

### Task 1: Language keys

**Files:**
- Modify: `ui/lang/desktop/*.json` (16 files)
- Create, then delete: `disposable/_writer-wave2c/i18n.mjs`

- [ ] **Step 1: Write the script**

Create `disposable/_writer-wave2c/i18n.mjs`:

```js
// Adds the Wave 2c keys to the sixteen desktop language files, each at its sorted place in the desktop.writer_ block.
// Usage: node disposable/_writer-wave2c/i18n.mjs <repo root>
import {readFileSync, writeFileSync} from 'node:fs';
import path from 'node:path';

const root = process.argv[2] || process.cwd();
const LANGS = ['cs', 'da', 'de', 'el', 'en', 'es', 'fr', 'hi', 'it', 'ja', 'nl', 'no', 'pl', 'pt', 'sv', 'zh'];
const KEYS = {
    border_line: ['Čára', 'Linje', 'Linie', 'Γραμμή', 'Line', 'Línea', 'Trait', 'रेखा', 'Linea', '線', 'Lijn', 'Linje', 'Linia', 'Linha', 'Linje', '线条'],
    border_width: ['Šířka čáry', 'Linjebredde', 'Linienbreite', 'Πάχος γραμμής', 'Line width', 'Grosor de línea', 'Épaisseur du trait', 'रेखा की चौड़ाई', 'Spessore linea', '線の太さ', 'Lijndikte', 'Linjebredde',
        'Grubość linii', 'Espessura da linha', 'Linjebredd', '线条粗细'],
    border_color: ['Barva čáry', 'Linjefarve', 'Linienfarbe', 'Χρώμα γραμμής', 'Line color', 'Color de línea', 'Couleur du trait', 'रेखा का रंग', 'Colore linea', '線の色', 'Lijnkleur', 'Linjefarge', 'Kolor linii',
        'Cor da linha', 'Linjefärg', '线条颜色'],
    border_outside: ['Vnější', 'Ydre', 'Außen', 'Εξωτερικά', 'Outside', 'Exterior', 'Extérieur', 'बाहरी', 'Esterni', '外枠', 'Buiten', 'Ytre', 'Zewnętrzne', 'Exteriores', 'Yttre', '外框'],
    border_inside: ['Vnitřní', 'Indre', 'Innen', 'Εσωτερικά', 'Inside', 'Interior', 'Intérieur', 'भीतरी', 'Interni', '内側', 'Binnen', 'Indre', 'Wewnętrzne', 'Interiores', 'Inre', '内框'],
    border_single: ['Plná', 'Fuldt optrukket', 'Durchgezogen', 'Συνεχής', 'Solid', 'Continua', 'Continu', 'ठोस', 'Continua', '実線', 'Doorgetrokken', 'Heltrukket', 'Ciągła', 'Contínua', 'Heldragen', '实线'],
    border_double: ['Dvojitá', 'Dobbelt', 'Doppelt', 'Διπλή', 'Double', 'Doble', 'Double', 'दोहरी', 'Doppia', '二重線', 'Dubbel', 'Dobbel', 'Podwójna', 'Dupla', 'Dubbel', '双线'],
    border_triple: ['Trojitá', 'Tredobbelt', 'Dreifach', 'Τριπλή', 'Triple', 'Triple', 'Triple', 'तिहरी', 'Tripla', '三重線', 'Drievoudig', 'Trippel', 'Potrójna', 'Tripla', 'Trippel', '三线'],
    border_thick: ['Tlustá', 'Tyk', 'Dick', 'Παχιά', 'Thick', 'Gruesa', 'Épais', 'मोटी', 'Spessa', '太線', 'Dik', 'Tykk', 'Gruba', 'Grossa', 'Tjock', '粗线'],
    border_hint: ['Ohraničení platí pro vybrané buňky. Více buněk vyberete tažením myší přes ně.', 'Kanter gælder for de markerede celler. Træk hen over cellerne for at markere flere.',
        'Rahmen gelten für die markierten Zellen. Für mehrere Zellen mit der Maus über die Zellen ziehen.', 'Τα περιγράμματα ισχύουν για τα επιλεγμένα κελιά. Σύρετε πάνω από τα κελιά για να επιλέξετε περισσότερα.',
        'Borders apply to the selected cells. Drag across cells to select several.', 'Los bordes se aplican a las celdas seleccionadas. Arrastra sobre las celdas para seleccionar varias.',
        'Les bordures s’appliquent aux cellules sélectionnées. Faites glisser sur les cellules pour en sélectionner plusieurs.', 'बॉर्डर चुनी हुई सेल पर लागू होते हैं। कई सेल चुनने के लिए उन पर खींचें।',
        'I bordi si applicano alle celle selezionate. Trascina sulle celle per selezionarne più di una.', '罫線は選択したセルに適用されます。複数のセルを選ぶには、セルの上をドラッグします。',
        'Randen gelden voor de geselecteerde cellen. Sleep over cellen om er meerdere te selecteren.', 'Kantlinjer gjelder for de merkede cellene. Dra over cellene for å merke flere.',
        'Obramowanie dotyczy zaznaczonych komórek. Przeciągnij po komórkach, aby zaznaczyć kilka.', 'Os limites aplicam-se às células selecionadas. Arraste sobre as células para selecionar várias.',
        'Kantlinjer gäller för de markerade cellerna. Dra över cellerna för att markera flera.', '边框应用于所选单元格。拖过单元格可选择多个。'],
    image_height: ['Výška', 'Højde', 'Höhe', 'Ύψος', 'Height', 'Alto', 'Hauteur', 'ऊँचाई', 'Altezza', '高さ', 'Hoogte', 'Høyde', 'Wysokość', 'Altura', 'Höjd', '高度'],
    image_lock: ['Zachovat poměr stran', 'Bevar højde-bredde-forhold', 'Seitenverhältnis sperren', 'Διατήρηση αναλογιών', 'Keep aspect ratio', 'Mantener proporción', 'Conserver les proportions', 'अनुपात बनाए रखें',
        'Mantieni proporzioni', '縦横比を固定', 'Verhouding behouden', 'Behold størrelsesforhold', 'Zachowaj proporcje', 'Manter proporção', 'Behåll proportioner', '锁定纵横比'],
    crop: ['Oříznutí', 'Beskæring', 'Zuschnitt', 'Περικοπή', 'Crop', 'Recorte', 'Rognage', 'क्रॉप', 'Ritaglio', 'トリミング', 'Bijsnijden', 'Beskjæring', 'Przycinanie', 'Recorte', 'Beskärning', '裁剪'],
    crop_reset: ['Zrušit oříznutí', 'Nulstil beskæring', 'Zuschnitt zurücksetzen', 'Επαναφορά περικοπής', 'Reset crop', 'Restablecer recorte', 'Réinitialiser le rognage', 'क्रॉप रीसेट करें', 'Reimposta ritaglio',
        'トリミングをリセット', 'Bijsnijden herstellen', 'Tilbakestill beskjæring', 'Resetuj przycinanie', 'Repor recorte', 'Återställ beskärning', '重置裁剪'],
    crop_invalid: ['Hodnoty oříznutí musí být mezi 0 a 99 % a část obrázku musí zůstat.', 'Beskæringsværdier skal ligge mellem 0 og 99 % og lade en del af billedet stå.',
        'Zuschnittwerte müssen zwischen 0 und 99 % liegen und einen Teil des Bildes übrig lassen.', 'Οι τιμές περικοπής πρέπει να είναι από 0 έως 99 % και να αφήνουν μέρος της εικόνας.',
        'Crop values must be between 0 and 99 % and leave part of the picture.', 'Los valores de recorte deben estar entre 0 y 99 % y dejar parte de la imagen.',
        'Les valeurs de rognage doivent être comprises entre 0 et 99 % et laisser une partie de l’image.', 'क्रॉप मान 0 से 99 % के बीच होने चाहिए और चित्र का कुछ भाग बचना चाहिए।',
        'I valori di ritaglio devono essere compresi tra 0 e 99 % e lasciare parte dell’immagine.', 'トリミングの値は0〜99 %で、画像の一部が残る必要があります。',
        'Bijsnijdwaarden moeten tussen 0 en 99 % liggen en een deel van de afbeelding overlaten.', 'Beskjæringsverdier må være mellom 0 og 99 % og la en del av bildet stå igjen.',
        'Wartości przycinania muszą mieścić się w zakresie 0–99 % i pozostawiać część obrazu.', 'Os valores de recorte devem estar entre 0 e 99 % e deixar parte da imagem.',
        'Beskärningsvärden måste ligga mellan 0 och 99 % och lämna kvar en del av bilden.', '裁剪值必须在 0 到 99 % 之间，并保留图片的一部分。'],
    image_replace: ['Nahradit obrázek…', 'Erstat billede…', 'Bild ersetzen…', 'Αντικατάσταση εικόνας…', 'Replace image…', 'Reemplazar imagen…', 'Remplacer l’image…', 'चित्र बदलें…', 'Sostituisci immagine…', '画像を置き換え…',
        'Afbeelding vervangen…', 'Erstatt bilde…', 'Zamień obraz…', 'Substituir imagem…', 'Ersätt bild…', '替换图片…']
};
for (const [name, texts] of Object.entries(KEYS)) if (texts.length !== LANGS.length) throw new Error(name + ' has ' + texts.length + ' texts');
for (const [index, lang] of LANGS.entries()) {
    const file = path.join(root, 'ui/lang/desktop', lang + '.json');
    const lines = readFileSync(file, 'utf8').split('\n');
    const first = lines.find(line => /^\s*"desktop\.writer_/.test(line));
    const indent = /^(\s*)/.exec(first)[1], eol = first.endsWith('\r') ? '\r' : '';
    let added = 0;
    for (const [name, texts] of Object.entries(KEYS)) {
        const key = 'desktop.writer_' + name, entry = indent + JSON.stringify(key) + ': ' + JSON.stringify(texts[index]);
        const block = lines.map((line, at) => [/^\s*"(desktop\.writer_[a-z0-9_]+)":/.exec(line)?.[1], at]).filter(([found]) => found);
        if (block.find(([found]) => found === key)) continue;
        const after = block.find(([found]) => found > key);
        lines.splice(after ? after[1] : block.at(-1)[1] + 1, 0, entry + ',' + eol);
        added++;
    }
    const text = lines.join('\n');
    JSON.parse(text);
    writeFileSync(file, text);
    console.log(lang + ': ' + added + ' keys added');
}
```

- [ ] **Step 2: Run it**

Run from the repo root: `node disposable/_writer-wave2c/i18n.mjs .`

Expected: sixteen lines, each `<lang>: 16 keys added`; `git diff --stat -- ui/lang/desktop` shows 16 files with 16 insertions each. A second run prints `0 keys added` sixteen times. Then delete `disposable/_writer-wave2c/`.

- [ ] **Step 3: Check and commit**

Run: `npm run test:writer` — expected: nine lines, all passing. Run: `go test ./ui -run "(?i)i18n" -count=1` — expected: `ok`.

Commit `ui/lang/desktop/*.json` as `feat(writer): language keys for borders, pictures and crop`.

---

### Task 2: The objects module

**Files:**
- Create: `ui/js/desktop/apps/writer-objects.js`, `scripts/test-writer-objects.mjs`
- Modify: `package.json`, `ui/js/desktop/core/module-loader.js`, `ui/desktop_office_assets_test.go`, `ui/desktop_writer_browser_test.go`, `ui/desktop_writer_reload_browser_test.go`

- [ ] **Step 1: Write the failing test**

Create `scripts/test-writer-objects.mjs`:

```js
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import vm from 'node:vm';

const context = {window: {}};
vm.runInNewContext(readFileSync('ui/js/desktop/apps/writer-objects.js', 'utf8'), context);
const O = context.window.WriterObjects;
const same = (actual, expected, message) => assert.equal(JSON.stringify(actual), JSON.stringify(expected), message);

// The pen: what a new window draws with, and what stays of a value nobody can draw with.
same(O.pen(), {style: 'single', size: 4, color: '64748B'});
same(O.pen({style: 'double', size: 12, color: '#ff0000'}), {style: 'double', size: 12, color: 'FF0000'});
same(O.pen({style: 'wave', size: 5, color: 'red'}), {style: 'single', size: 4, color: '64748B'}, 'unknown values fall back');
same(O.pen({style: 'dashed'}).style, 'single', 'the engine does not paint a dashed left edge, so the pen has no dashed line');
same(O.pen({style: 'thick', size: '24', color: 'abcdef'}), {style: 'thick', size: 24, color: 'ABCDEF'});
same(O.spec(O.pen({color: '#0000ff'})), {style: 'single', size: 4, color: {kind: 'hex', value: '0000FF'}});
same(O.styles, ['single', 'double', 'triple', 'thick']);
same(O.widths, [4, 6, 8, 12, 18, 24]);
same(O.edges, ['all', 'outside', 'inside', 'top', 'bottom', 'left', 'right', 'none']);
assert.equal(O.widthLabel(4, value => String(value).replace('.', ',')), '0,5 pt');
assert.equal(O.widthLabel(18, String), '2.25 pt');
same(O.edges.map(O.edgeLabel), ['borders', 'border_outside', 'border_inside', 'margin_top', 'margin_bottom', 'margin_left', 'margin_right', 'no_borders']);

// Border commands: one per edge button; none needs no pen.
const pen = O.pen({style: 'triple', size: 8, color: '00AA00'});
same(O.borderCommand('outside', pen), {type: 'setTableBorders', scope: 'outside', spec: {style: 'triple', size: 8, color: {kind: 'hex', value: '00AA00'}}});
same(O.borderCommand('all', pen).scope, 'all');
same(O.borderCommand('none', pen), {type: 'setTableBorders', scope: 'none', target: 'all'});
assert.equal(O.borderCommand('diagonal', pen), null);

// A picture's size: locked keeps the proportions, unlocked changes one side.
const picture = {widthEmu: 1905000, heightEmu: 952500};
same(O.sizeCommand(picture, 'width', 952500, true), {type: 'setImageProperties', widthEmu: 952500, heightEmu: 476250});
same(O.sizeCommand(picture, 'height', 1905000, true), {type: 'setImageProperties', widthEmu: 3810000, heightEmu: 1905000});
same(O.sizeCommand(picture, 'width', 952500, false), {type: 'setImageProperties', widthEmu: 952500});
same(O.sizeCommand(picture, 'height', 635000.4, false), {type: 'setImageProperties', heightEmu: 635000});
assert.equal(O.sizeCommand(picture, 'width', 0, true), null);
assert.equal(O.sizeCommand(picture, 'width', NaN, true), null);
assert.equal(O.sizeCommand({widthEmu: 0, heightEmu: 10}, 'width', 100, true), null, 'no proportions without a size');
assert.equal(O.fitHeight(1905000, 120, 120), 1905000);
assert.equal(O.fitHeight(1905000, 200, 100), 952500);

// Crop: percent of the picture; something of it has to stay.
same(O.crop({left: 10, top: 20, right: 30, bottom: 0}), {command: {type: 'setImageProperties', crop: {left: 10, top: 20, right: 30, bottom: 0}}});
same(O.crop({left: 0, top: 0, right: 0, bottom: 0}).command.crop, {left: 0, top: 0, right: 0, bottom: 0});
for (const bad of [{left: -1, top: 0, right: 0, bottom: 0}, {left: 100, top: 0, right: 0, bottom: 0}, {left: 60, top: 0, right: 40, bottom: 0}, {left: 0, top: 50, right: 0, bottom: 50},
    {left: NaN, top: 0, right: 0, bottom: 0}, {left: 0, top: 0, right: 0}]) same(O.crop(bad), {error: 'crop_invalid'}, JSON.stringify(bad));

console.log('Writer objects: border pen, border commands, picture size and crop pass');
```

- [ ] **Step 2: Run it to see it fail**

Run: `node scripts/test-writer-objects.mjs` — expected: it fails with `ENOENT` for `writer-objects.js`.

- [ ] **Step 3: Write the module**

Create `ui/js/desktop/apps/writer-objects.js`:

```js
(function () {
    'use strict';
    // Tables and pictures: what the Format panel holds, as engine commands. Nothing here touches the editor.
    // The engine also knows dashed and dotted, but does not paint them on a left edge that had no line before.
    const STYLES = ['single', 'double', 'triple', 'thick'];
    // Line widths in eighths of a point, the engine's unit: 1/2, 3/4, 1, 1 1/2, 2 1/4 and 3 pt.
    const WIDTHS = [4, 6, 8, 12, 18, 24];
    const EDGES = ['all', 'outside', 'inside', 'top', 'bottom', 'left', 'right', 'none'];
    const EDGE_LABELS = {all: 'borders', outside: 'border_outside', inside: 'border_inside', top: 'margin_top', bottom: 'margin_bottom', left: 'margin_left', right: 'margin_right', none: 'no_borders'};
    const PEN = {style: 'single', size: 4, color: '64748B'};

    // pen is the line every border button draws with. A value nobody can draw with falls back to the default.
    function pen(value = {}) {
        const color = String(value.color || '').replace(/^#/, '').toUpperCase(), size = Number(value.size);
        return {style: STYLES.includes(value.style) ? value.style : PEN.style, size: WIDTHS.includes(size) ? size : PEN.size, color: /^[0-9A-F]{6}$/.test(color) ? color : PEN.color};
    }
    // spec is the pen in the shape the engine takes.
    const spec = value => ({style: value.style, size: value.size, color: {kind: 'hex', value: value.color}});
    const widthLabel = (size, format) => format(size / 8) + ' pt';
    const edgeLabel = edge => EDGE_LABELS[edge];

    // borderCommand is the command of one edge button. It acts on the selected cells, or on the cell at the caret.
    function borderCommand(edge, value) {
        if (edge === 'none') return {type: 'setTableBorders', scope: 'none', target: 'all'};
        return EDGES.includes(edge) ? {type: 'setTableBorders', scope: edge, spec: spec(value)} : null;
    }

    // sizeCommand sets a picture's width or height in EMU. locked changes the other side with it.
    function sizeCommand(image, side, emu, locked) {
        const size = Math.round(emu);
        if (!(size > 0)) return null;
        if (!locked) return {type: 'setImageProperties', [side === 'width' ? 'widthEmu' : 'heightEmu']: size};
        if (!(image.widthEmu > 0) || !(image.heightEmu > 0)) return null;
        return side === 'width' ? {type: 'setImageProperties', widthEmu: size, heightEmu: Math.round(size * image.heightEmu / image.widthEmu)}
            : {type: 'setImageProperties', widthEmu: Math.round(size * image.widthEmu / image.heightEmu), heightEmu: size};
    }
    // fitHeight is the height a picture of the given pixel size has at a width.
    const fitHeight = (widthEmu, pixelWidth, pixelHeight) => Math.round(widthEmu * pixelHeight / pixelWidth);

    // crop takes the four sides in percent of the picture. Something of the picture has to stay.
    function crop(values) {
        const sides = ['left', 'top', 'right', 'bottom'].map(side => Number(values[side]));
        if (sides.some(side => !Number.isFinite(side) || side < 0 || side >= 100) || sides[0] + sides[2] >= 100 || sides[1] + sides[3] >= 100) return {error: 'crop_invalid'};
        return {command: {type: 'setImageProperties', crop: {left: sides[0], top: sides[1], right: sides[2], bottom: sides[3]}}};
    }

    window.WriterObjects = {styles: STYLES, widths: WIDTHS, edges: EDGES, pen, spec, widthLabel, edgeLabel, borderCommand, sizeCommand, fitHeight, crop};
})();
```

- [ ] **Step 4: Run the test**

Run: `node scripts/test-writer-objects.mjs` — expected: `Writer objects: border pen, border commands, picture size and crop pass`.

- [ ] **Step 5: Register the test and the script**

In `package.json`, replace

```json
 && node scripts/test-writer-pdf.mjs",
```
with
```json
 && node scripts/test-writer-pdf.mjs && node scripts/test-writer-objects.mjs",
```

In `ui/js/desktop/core/module-loader.js`, replace

```js
'/js/desktop/apps/writer-pdf.js', '/js/desktop/apps/writer-commands.js',
```
with
```js
'/js/desktop/apps/writer-pdf.js', '/js/desktop/apps/writer-objects.js', '/js/desktop/apps/writer-commands.js',
```

In `ui/desktop_office_assets_test.go`, replace

```go
		"/js/desktop/apps/writer-pdf.js",
```
with
```go
		"/js/desktop/apps/writer-pdf.js",
		"/js/desktop/apps/writer-objects.js",
```

In `ui/desktop_writer_browser_test.go` (two fixtures) and `ui/desktop_writer_reload_browser_test.go` (one fixture), replace every occurrence of

```go
<script src="/js/desktop/apps/writer-pdf.js"></script>
```
by
```go
<script src="/js/desktop/apps/writer-pdf.js"></script><script src="/js/desktop/apps/writer-objects.js"></script>
```

- [ ] **Step 6: Check and commit**

Run: `npm run test:writer` — expected: ten lines, the last one from the objects test. Run: `npm run check:ui` and `go test ./ui -run "(?i)(office|writer)" -count=1` — expected: both pass.

Commit as `feat(writer): border pen, picture size and crop as commands`.

---

### Task 3: Borders with a pen

**Files:**
- Modify: `ui/js/desktop/apps/writer-commands.js`, `ui/js/desktop/apps/writer-panels.js`, `ui/js/desktop/apps/writer.js`, `scripts/test-writer-commands.mjs`

- [ ] **Step 1: Make the node test ask for the pen**

In `scripts/test-writer-commands.mjs`, replace

```js
        clipboard: kind => options.clipboard ? options.clipboard[kind] : true
    });
```
with
```js
        clipboard: kind => options.clipboard ? options.clipboard[kind] : true,
        pen: options.pen && (() => options.pen)
    });
```

Replace

```js
    assert.equal(log.exec[3].spec.style, 'single');
    assert.equal(log.exec[4].target, 'all');
```
with
```js
    assert.equal(log.exec[3].spec.style, 'single');
    assert.equal(log.exec[4].target, 'all');
    // Borders draws with the window's pen; without one the registry's own line stays.
    const drawn = harness({pen: {style: 'double', size: 12, color: {kind: 'hex', value: 'FF0000'}}});
    assert.equal(drawn.commands.invoke('borders'), true);
    assert.equal(JSON.stringify(drawn.log.exec[0]), JSON.stringify({type: 'setTableBorders', scope: 'all', spec: {style: 'double', size: 12, color: {kind: 'hex', value: 'FF0000'}}}));
    assert.equal(drawn.commands.invoke('noBorders'), true);
    assert.equal(drawn.log.exec[1].spec, undefined);
```

Run: `node scripts/test-writer-commands.mjs` — expected: it fails at the `double` comparison.

- [ ] **Step 2: The registry takes the pen**

In `ui/js/desktop/apps/writer-commands.js`, replace

```js
        {id: 'borders', command: {type: 'setTableBorders', scope: 'all', spec: {style: 'single', size: 4, color: {kind: 'hex', value: '64748B'}}}, label: 'borders', glyph: 'borders-all', fallback: '▦'},
```
with
```js
        {id: 'borders', command: {type: 'setTableBorders', scope: 'all', spec: {style: 'single', size: 4, color: {kind: 'hex', value: '64748B'}}}, pen: true, label: 'borders', glyph: 'borders-all', fallback: '▦'},
```

Replace

```js
    //        clipboard(kind)?: whether the system clipboard can be read or written}
```
with
```js
    //        clipboard(kind)?: whether the system clipboard can be read or written,
    //        pen()?: the border line of the window, for the entries marked `pen`}
```

Replace

```js
            if (definition?.command) return definition.command;
```
with
```js
            if (definition?.command) return definition.pen && host.pen ? {...definition.command, spec: host.pen()} : definition.command;
```

Run: `node scripts/test-writer-commands.mjs` — expected: it passes.

- [ ] **Step 3: The panel's table section**

In `ui/js/desktop/apps/writer-panels.js`, replace

```js
        const more=action=>'<div class="writer-buttonrow"><button type="button" data-action="'+action+'">'+esc(tr('more'))+'</button></div>';
```
with
```js
        const more=action=>'<div class="writer-buttonrow"><button type="button" data-action="'+action+'">'+esc(tr('more'))+'</button></div>';
        // Tables and pictures: the pen draws every border until it is changed; lock keeps a picture's proportions.
        const O=window.WriterObjects;
        let pen=O.pen(), lock=true;
        const percent=(label,key)=>'<label class="writer-field"><span>'+esc(tr(label)+' · %')+'</span>'+input(key,0,'number','min="0" max="99" step="1"')+'</label>';
        const options=(key,items)=>'<select data-field="'+key+'">'+items.map(([value,text])=>'<option value="'+esc(value)+'">'+esc(text)+'</option>').join('')+'</select>';
```

Replace

```js
                    '<div class="writer-buttonrow">'+commandButton({type:'setTableBorders',scope:'all',spec:{style:'single',size:4,color:{kind:'hex',value:'64748B'}}},'borders')+
                    commandButton({type:'setTableBorders',scope:'none',target:'all'},'no_borders')+'</div>'+
```
with
```js
                    '<p class="writer-hint">'+esc(tr('border_hint'))+'</p>'+
                    field('border_line',options('borderStyle',O.styles.map(style=>[style,tr('border_'+style)])))+
                    field('border_width',options('borderWidth',O.widths.map(size=>[size,O.widthLabel(size,app.commands.format)])))+
                    field('border_color',input('borderColor','#'+pen.color.toLowerCase(),'color'))+
                    '<div class="writer-buttonrow">'+O.edges.map(edge=>'<button type="button" data-border="'+edge+'">'+esc(tr(O.edgeLabel(edge)))+'</button>').join('')+'</div>'+
```

Replace

```js
                if(highlight)fields['text.highlight']=highlight;
```
with
```js
                if(highlight)fields['text.highlight']=highlight;
                fields.borderStyle=pen.style;fields.borderWidth=pen.size;fields.borderColor='#'+pen.color.toLowerCase();
```

Replace

```js
                    cellFill:'setCellFill',imageWidth:'setImageProperties',imageAlt:'setImageProperties',imageWrap:'setImageWrapType'};
```
with
```js
                    cellFill:'setCellFill',borderStyle:'setTableBorders',borderWidth:'setTableBorders',borderColor:'setTableBorders',
                    imageWidth:'setImageProperties',imageHeight:'setImageProperties',imageAlt:'setImageProperties',imageWrap:'setImageWrapType',
                    cropLeft:'setImageProperties',cropTop:'setImageProperties',cropRight:'setImageProperties',cropBottom:'setImageProperties'};
```

Replace

```js
                    el.title=el.disabled?tr('command_unavailable'):'';
                }
```
with
```js
                    el.title=el.disabled?tr('command_unavailable'):'';
                }
                for(const el of root.querySelectorAll('.writer-right [data-border]'))el.disabled=app.readonly || !!app.blocked('setTableBorders') || !snapshot.table;
```

Replace

```js
            if(key==='text.color' || key==='text.highlight'){app.run(app.markCommand(key,v));return;}
```
with
```js
            if(key==='text.color' || key==='text.highlight'){app.run(app.markCommand(key,v));return;}
            // The pen is a setting of the window, not a change to the document.
            if(key==='borderStyle' || key==='borderWidth' || key==='borderColor'){pen=O.pen({...pen,[key==='borderStyle'?'style':key==='borderWidth'?'size':'color']:v});return;}
            if(key==='imageLock'){lock=el.checked;return;}
```

Replace

```js
            const target=event.target.closest('[data-command],[data-panel-command],[data-review-op],[data-block]');if(!target || target.disabled)return;
            if(target.dataset.command)app.run(JSON.parse(target.dataset.command));
```
with
```js
            const target=event.target.closest('[data-command],[data-panel-command],[data-review-op],[data-block],[data-border]');if(!target || target.disabled)return;
            if(target.dataset.command)app.run(JSON.parse(target.dataset.command));
            if(target.dataset.border)app.run(O.borderCommand(target.dataset.border,pen));
```

Replace

```js
        return {show,update,print,pdf,reset,dispose(){life.abort();reset();printFrame?.dispose();}};
```
with
```js
        return {show,update,print,pdf,pen:()=>O.spec(pen),reset,dispose(){life.abort();reset();printFrame?.dispose();}};
```

- [ ] **Step 4: The window's pen reaches the registry**

In `ui/js/desktop/apps/writer.js`, replace

```js
            clipboard: kind => kind === 'write' ? !!navigator.clipboard?.writeText : !!navigator.clipboard?.readText});
```
with
```js
            clipboard: kind => kind === 'write' ? !!navigator.clipboard?.writeText : !!navigator.clipboard?.readText,
            pen: () => panels.pen()});
```

- [ ] **Step 5: Check and commit**

Run: `npm run test:writer` and `npm run check:ui` — expected: both pass. The picture fields named in `fieldCommands` come with Task 4; until then they match nothing.

Commit as `feat(writer): table borders with line style, width and colour`.

---

### Task 4: Picture size, crop and replace

**Files:**
- Modify: `ui/js/desktop/apps/writer-commands.js`, `ui/js/desktop/apps/writer-context.js`, `ui/js/desktop/apps/writer-menus.js`, `ui/js/desktop/apps/writer-panels.js`, `ui/js/desktop/apps/writer.js`, `scripts/test-writer-commands.mjs`, `scripts/test-writer-context.mjs`

- [ ] **Step 1: Make the node tests ask for the command**

In `scripts/test-writer-commands.mjs`, replace

```js
    assert.equal(drawn.log.exec[1].spec, undefined);
```
with
```js
    assert.equal(drawn.log.exec[1].spec, undefined);
    // Replace image is an app action that needs a selected picture.
    assert.equal(harness().commands.state('imageReplace').enabled, false);
    const chosen = harness({snapshot: {image: {id: 'p1'}}});
    assert.equal(chosen.commands.state('imageReplace').enabled, true);
    assert.equal(chosen.commands.invoke('imageReplace'), true);
    assert.equal(chosen.log.acted[0], 'imageReplace');
    assert.equal(harness({snapshot: {image: {id: 'p1'}}, readonly: true}).commands.state('imageReplace').enabled, false);
    assert.equal(harness({snapshot: {image: {id: 'p1'}}, mode: 'suggesting'}).commands.state('imageReplace').enabled, false);
```

In `scripts/test-writer-context.mjs`, replace

```js
assert.ok(flatten(everything).includes('deleteImage'));
```
with
```js
assert.ok(flatten(everything).includes('deleteImage'));
assert.ok(flatten(everything).includes('imageReplace') && !flatten(noImage).includes('imageReplace'), 'replace image appears for a selected image only');
assert.equal(WriterContext.bars.image.items.join(), 'rotateLeft,rotateRight,|,imageReplace,deleteImage,format');
```

Run: `npm run test:writer` — expected: the commands test fails (`imageReplace` is unknown).

- [ ] **Step 2: The command, the bar and the menus**

In `ui/js/desktop/apps/writer-commands.js`, replace

```js
        {id: 'deleteImage', command: {type: 'deleteImage'}, label: 'image_delete', icon: 'trash'},
```
with
```js
        {id: 'imageReplace', label: 'image_replace', icon: 'image', write: true, needsEditor: true, needsImage: true, untracked: 'setImageProperties'},
        {id: 'deleteImage', command: {type: 'deleteImage'}, label: 'image_delete', icon: 'trash'},
```

Replace

```js
            if (definition.scope && editor.snapshot().scope?.kind !== definition.scope) return off;
```
with
```js
            if (definition.scope && editor.snapshot().scope?.kind !== definition.scope) return off;
            if (definition.needsImage && !editor.snapshot().image) return off;
```

In `ui/js/desktop/apps/writer-context.js`, replace

```js
        image: {items: ['rotateLeft', 'rotateRight', '|', 'deleteImage', 'format'], text: ['deleteImage', 'format']},
```
with
```js
        image: {items: ['rotateLeft', 'rotateRight', '|', 'imageReplace', 'deleteImage', 'format'], text: ['imageReplace', 'deleteImage', 'format']},
```

Replace

```js
            target.image ? ['rotateLeft', 'rotateRight', 'deleteImage'] : [],
```
with
```js
            target.image ? ['rotateLeft', 'rotateRight', 'imageReplace', 'deleteImage'] : [],
```

In `ui/js/desktop/apps/writer-menus.js`, replace

```js
item('rotateLeft'),item('rotateRight'),SEP,item('deleteImage')]},SEP,
```
with
```js
item('rotateLeft'),item('rotateRight'),SEP,item('imageReplace'),item('deleteImage')]},SEP,
```

Run: `npm run test:writer` — expected: ten lines, all passing.

- [ ] **Step 3: The panel's picture sections**

In `ui/js/desktop/apps/writer-panels.js`, replace

```js
                    section('image',length('image_width','imageWidth')+field('alt_text',input('imageAlt','','text'))+
                    field('image_wrap',select('imageWrap',[['inline','inline'],['squareLeft','wrap_left'],['squareRight','wrap_right'],['topAndBottom','wrap_top']])));
```
with
```js
                    section('image',length('image_width','imageWidth')+length('image_height','imageHeight')+
                    '<div class="writer-checks"><label><input type="checkbox" data-field="imageLock"'+(lock?' checked':'')+'>'+esc(tr('image_lock'))+'</label></div>'+
                    field('alt_text',input('imageAlt','','text'))+
                    field('image_wrap',select('imageWrap',[['inline','inline'],['squareLeft','wrap_left'],['squareRight','wrap_right'],['topAndBottom','wrap_top']]))+
                    '<div class="writer-buttonrow"><button type="button" data-action="imageReplace">'+esc(tr('image_replace'))+'</button></div>')+
                    section('crop',percent('margin_left','cropLeft')+percent('margin_top','cropTop')+percent('margin_right','cropRight')+percent('margin_bottom','cropBottom')+
                    '<div class="writer-buttonrow">'+commandButton({type:'setImageProperties',crop:{left:0,top:0,right:0,bottom:0}},'crop_reset')+'</div>');
```

Replace

```js
                        if(Number.isFinite(selected.widthEmu) && selected.widthEmu>0)fields.imageWidth=u.show(selected.widthEmu/635);
```
with
```js
                        if(Number.isFinite(selected.widthEmu) && selected.widthEmu>0)fields.imageWidth=u.show(selected.widthEmu/635);
                        if(Number.isFinite(selected.heightEmu) && selected.heightEmu>0)fields.imageHeight=u.show(selected.heightEmu/635);
                        for(const side of ['Left','Top','Right','Bottom'])fields['crop'+side]=Math.round((selected.crop?.[side.toLowerCase()] || 0)*10)/10;
```

Replace

```js
(!!key?.startsWith('image') && !image);
```
with
```js
(/^(image|crop)/.test(key || '') && !image) || (key==='imageWrap' && !image?.canChangeWrap);
```

Replace

```js
            if((['leftIndent','firstIndent','imageWidth'].includes(key) || key?.startsWith('margin')) && !Number.isFinite(twips)){app.notice(tr('length_invalid'),true);return;}
```
with
```js
            if((['leftIndent','firstIndent','imageWidth','imageHeight'].includes(key) || key?.startsWith('margin')) && !Number.isFinite(twips)){app.notice(tr('length_invalid'),true);return;}
```

Replace

```js
            if(key==='imageWidth') {
                const image=editor.getSelectedImage();
                if(image && twips>0)app.run({type:'setImageProperties',widthEmu:twips*635,heightEmu:Math.round(twips*635*image.heightEmu/image.widthEmu)});
            }
```
with
```js
            if(key==='imageWidth' || key==='imageHeight') {
                const image=editor.getSelectedImage(),size=image && O.sizeCommand(image,key==='imageWidth'?'width':'height',twips*635,lock);
                if(size)app.run(size);
            }
            if(key?.startsWith('crop')) {
                const crop=O.crop(Object.fromEntries(['Left','Top','Right','Bottom'].map(side=>[side.toLowerCase(),Number(find('[data-field="crop'+side+'"]').value)])));
                if(crop.error){app.notice(tr('crop_invalid'),true);return;}
                app.run(crop.command);
            }
```

- [ ] **Step 4: Replace image in the app**

In `ui/js/desktop/apps/writer.js`, replace

```js
            '<input data-image type="file" accept="image/png,image/jpeg,image/webp" hidden></div>';
```
with
```js
            '<input data-image type="file" accept="image/png,image/jpeg,image/webp" hidden><input data-image-replace type="file" accept="image/png,image/jpeg,image/webp" hidden></div>';
```

Replace

```js
                    case 'image':find('[data-image]').click();break;
```
with
```js
                    case 'image':find('[data-image]').click();break;
                    case 'imageReplace':find('[data-image-replace]').click();break;
```

Replace

```js
        let qrOpening=false;
```
with
```js
        // Replaces the selected picture's file. The engine takes only the format the picture already has, so a file in
        // another format is tried as PNG and as JPEG. The width stays and the height follows the new picture.
        async function replaceImage(file) {
            if(!editor || ctx.readonly || !/^image\/(png|jpeg|webp)$/.test(file.type))throw new Error(tr('image_format'));
            const refusal=commands.blocked('setImageProperties');if(refusal)throw new Error(refusal);
            if(file.size>20*1024*1024)throw new Error(tr('image_large'));
            const active=editor,selected=editor.getSelectedImage();
            if(!selected)throw new Error(tr('command_unavailable'));
            const bitmap=await createImageBitmap(file),width=bitmap.width,height=bitmap.height;
            const encoded=async type=>{
                const canvas=document.createElement('canvas');canvas.width=width;canvas.height=height;
                const paint=canvas.getContext('2d');
                if(type==='image/jpeg'){paint.fillStyle='#fff';paint.fillRect(0,0,width,height);}
                paint.drawImage(bitmap,0,0);
                const blob=await new Promise(resolve=>canvas.toBlob(resolve,type,.92));
                return blob?new Uint8Array(await blob.arrayBuffer()):null;
            };
            let result=null;
            try {
                for(const bytes of [async()=>new Uint8Array(await file.arrayBuffer()),()=>encoded('image/png'),()=>encoded('image/jpeg')]) {
                    const data=await bytes();
                    if(disposed || editor!==active || editor.getSelectedImage()?.id!==selected.id)return;
                    if(!data)continue;
                    result=await editor.executeImageCommand({type:'replaceImage',data});
                    if(result.ok || result.code!=='invalidArgs')break;
                }
            } finally {bitmap.close();}
            if(!result?.ok)throw new Error(result?.reason || tr('command_unavailable'));
            const now=editor.getSelectedImage();
            if(now?.widthEmu>0 && width>0 && height>0)run({type:'setImageProperties',heightEmu:window.WriterObjects.fitHeight(now.widthEmu,width,height)});
            else refresh();
        }
        let qrOpening=false;
```

Replace

```js
        find('[data-image]').addEventListener('change',event=>{const file=event.target.files[0];if(file)insertImage(file).catch(fail);event.target.value='';},{signal:lifecycle.signal});
```
with
```js
        find('[data-image]').addEventListener('change',event=>{const file=event.target.files[0];if(file)insertImage(file).catch(fail);event.target.value='';},{signal:lifecycle.signal});
        find('[data-image-replace]').addEventListener('change',event=>{const file=event.target.files[0];if(file)replaceImage(file).catch(fail);event.target.value='';},{signal:lifecycle.signal});
```

- [ ] **Step 5: Check and commit**

Run: `npm run test:writer`, `npm run check:ui` and `go test ./ui -run "(?i)(windowmenu|contextmenu|office|writer|i18n)" -count=1` — expected: all pass.

Commit as `feat(writer): picture height, aspect lock, crop and Replace image`.

---

### Task 5: Style previews in the toolbar

**Files:**
- Modify: `ui/js/desktop/apps/writer-toolbar.js`, `ui/css/desktop-app-writer.css`, `scripts/test-writer-toolbar.mjs`

- [ ] **Step 1: Make the node test ask for the preview**

In `scripts/test-writer-toolbar.mjs`, replace

```js
console.log('Writer toolbar: groups, glyphs and overflow fit pass');
```
with
```js
// A style's preview as the inline style of its option: the size capped, nothing but declarations.
assert.equal(WriterToolbar.stylePreview({fontFamily: 'Calibri', fontSizePt: 28, bold: false, italic: false, color: null}), 'font-family:"Calibri",sans-serif;font-size:22px;');
assert.equal(WriterToolbar.stylePreview({fontFamily: 'Calibri', fontSizePt: 16, bold: true, italic: true, color: '2F5496'}), 'font-family:"Calibri",sans-serif;font-size:21px;font-weight:700;font-style:italic;color:#2F5496;');
assert.equal(WriterToolbar.stylePreview({fontFamily: 'Calibri', fontSizePt: 9, bold: false, italic: true, color: '404040'}), 'font-family:"Calibri",sans-serif;font-size:13px;font-style:italic;color:#404040;');
assert.equal(WriterToolbar.stylePreview({fontFamily: 'A";color:red;}<b>', fontSizePt: 'x', color: 'red;'}), 'font-family:"Acolorredb",sans-serif;font-size:15px;');
assert.equal(WriterToolbar.stylePreview(null), '');
console.log('Writer toolbar: groups, glyphs, overflow fit and style previews pass');
```

Run: `node scripts/test-writer-toolbar.mjs` — expected: it fails with `stylePreview is not a function`.

- [ ] **Step 2: The preview and the list**

In `ui/js/desktop/apps/writer-toolbar.js`, replace

```js
    const STYLE_LABELS ={normal: 'style_normal', title: 'style_title', subtitle: 'style_subtitle', quote: 'style_quote', caption: 'style_caption', nospacing: 'style_no_spacing', listparagraph: 'style_list_paragraph'};
```
with
```js
    const STYLE_LABELS ={normal: 'style_normal', title: 'style_title', subtitle: 'style_subtitle', quote: 'style_quote', caption: 'style_caption', nospacing: 'style_no_spacing', listparagraph: 'style_list_paragraph'};
    // The engine's preview of a paragraph style as the inline style of its list entry. The size is in px, between
    // 13 and 22, so that a 28 pt title still fits a list; a name or colour that is not plain is dropped.
    function stylePreview(preview) {
        if (!preview) return '';
        const family = String(preview.fontFamily || '').replace(/[^\w .-]/g, ''), size = Math.min(22, Math.max(13, Math.round((Number(preview.fontSizePt) || 11) * 4 / 3)));
        return (family ? 'font-family:"' + family + '",sans-serif;' : '') + 'font-size:' + size + 'px;' + (preview.bold ? 'font-weight:700;' : '') + (preview.italic ? 'font-style:italic;' : '')
            + (/^[0-9a-f]{6}$/i.test(preview.color || '') ? 'color:#' + preview.color + ';' : '');
    }
```

Replace

```js
            const editor = app.editor, option = (value, text) => '<option value="' + esc(value) + '">' + esc(text) + '</option>';
            wrap.querySelector('[data-slot="styles.style"]').innerHTML = editor.getDocumentStyles().filter(style => style.type === 'paragraph').map(style => option(style.styleId, styleLabel(style))).join('');
```
with
```js
            const editor = app.editor, option = (value, text, look) => '<option value="' + esc(value) + '"' + (look ? ' style="' + esc(look) + '"' : '') + '>' + esc(text) + '</option>';
            // Where the browser draws a customizable select, every style shows the way it looks; elsewhere the list stays plain.
            const rich = !!window.CSS?.supports?.('appearance', 'base-select');
            wrap.querySelector('[data-slot="styles.style"]').innerHTML = editor.getDocumentStyles().filter(style => style.type === 'paragraph')
                .map(style => option(style.styleId, styleLabel(style), rich ? stylePreview(style.preview) : '')).join('');
```

Replace

```js
    window.WriterToolbar = {create, icons, groups: GROUPS, fit};
```
with
```js
    window.WriterToolbar = {create, icons, groups: GROUPS, fit, stylePreview};
```

- [ ] **Step 3: The list's look**

In `ui/css/desktop-app-writer.css`, replace

```css
.writer-fonts select[data-slot="font.family"]{width:125px}
```
with
```css
.writer-fonts select[data-slot="font.family"]{width:125px}
/* The style list as a gallery: a customizable select where the browser has one. The entries stand on paper, in the styles' own colours, whatever the theme. */
@supports (appearance:base-select){
.writer-app select[data-slot="styles.style"],.writer-app select[data-slot="styles.style"]::picker(select){appearance:base-select}
.writer-app select[data-slot="styles.style"]{display:inline-flex;align-items:center;justify-content:space-between;gap:6px;white-space:nowrap;overflow:hidden}
.writer-app select[data-slot="styles.style"]::picker(select){margin-top:4px;padding:4px;min-width:240px;max-height:min(70vh,520px);overflow:auto;border:1px solid #cbd5e1;border-radius:8px;background:#fff;color:#172033;box-shadow:0 12px 32px rgba(15,23,42,.28)}
.writer-app select[data-slot="styles.style"] option{display:block;padding:5px 10px;border-radius:5px;background:transparent;color:#172033;line-height:1.3;white-space:nowrap}
.writer-app select[data-slot="styles.style"] option::checkmark{display:none}
.writer-app select[data-slot="styles.style"] option:hover,.writer-app select[data-slot="styles.style"] option:focus-visible{background:#e2e8f0;outline:none}
.writer-app select[data-slot="styles.style"] option:checked{background:#dbeafe}
}
```

- [ ] **Step 4: Check and commit**

Run: `npm run test:writer`, `npm run check:ui` and `go test ./ui -run "(?i)(office|writer|materials|feeling)" -count=1` — expected: all pass.

Commit as `feat(writer): the style list shows each style the way it looks`.

---

### Task 6: Browser checks

**Files:**
- Create: `ui/desktop_writer_objects_test.go`
- Modify: `ui/desktop_writer_browser_test.go`

- [ ] **Step 1: Call the check**

In `ui/desktop_writer_browser_test.go`, replace

```go
	verifyWriterPdf(t, page)
```
with
```go
	verifyWriterPdf(t, page)
	verifyWriterObjects(t, page)
```

- [ ] **Step 2: The check**

Create `ui/desktop_writer_objects_test.go`:

```go
package ui

import (
	"os"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"
)

// verifyWriterObjects runs after verifyWriterPdf in the file-ops fixture: table borders drawn with the pen on a
// dragged cell selection, a picture's size, crop and replacement, and the style list's previews.
func verifyWriterObjects(t *testing.T, page *rod.Page) {
	t.Helper()
	helpers := `const app=WriterApp.instances.get('test'),editor=app.editor,wait=ms=>new Promise(r=>setTimeout(r,ms));
        const until=async(test,what)=>{for(let i=0;i<160;i++){if(await test())return;await wait(50);}throw Error('timed out: '+what);};
        const labels=await(await fetch('/lang/desktop/en.json')).json(),text=key=>labels['desktop.writer_'+key];
        const said=()=>document.querySelector('[data-notice-text]').textContent;
        const field=key=>document.querySelector('[data-right] [data-field="'+key+'"]');
        const set=async(key,value)=>{const el=field(key);if(el.type==='checkbox')el.checked=value;else el.value=String(value);el.dispatchEvent(new Event('change',{bubbles:true}));await wait(250);};
        const run=command=>{const result=editor.exec(command);if(!result.ok)throw Error(JSON.stringify({command:command.type,result}));};
        const u=WriterLayout.units(document.documentElement.lang,navigator.language);`
	script := func(body string) string { return `async()=>{` + helpers + body + `}` }

	// A blank document with a 3 x 3 table and the Format panel open.
	centers := page.MustEval(`async()=>{
        const app=WriterApp.instances.get('test'),wait=ms=>new Promise(r=>setTimeout(r,ms));
        window.confirmAnswer=true;
        await app.act('blank');
        for(let i=0;i<160&&!(app.editor&&document.querySelector('[data-loading]').hidden);i++)await wait(50);
        const result=app.editor.exec({type:'insertTable',rows:3,cols:3});
        if(!result.ok)throw Error('no table: '+JSON.stringify(result));
        if(document.querySelector('[data-right]').hidden)await app.act('format');
        await wait(400);
        if(!document.querySelector('[data-right] [data-field="borderStyle"]'))throw Error('the Format panel has no border pen: '+document.querySelector('[data-right]').textContent.slice(0,200));
        document.querySelector('[data-scroll]').scrollTop=0;
        await wait(150);
        const cells=[...document.querySelectorAll('.docx-page .docx-table-cell')].map(cell=>cell.getBoundingClientRect());
        if(cells.length!==9)throw Error('the table shows '+cells.length+' cells');
        return {ax:cells[0].left+20,ay:cells[0].top+cells[0].height/2,mx:cells[4].left+cells[4].width/2,my:cells[4].top+cells[4].height/2,bx:cells[8].left+cells[8].width/2,by:cells[8].top+cells[8].height/2};
    }`)
	page.Mouse.MustMoveTo(centers.Get("ax").Num(), centers.Get("ay").Num())
	page.Mouse.MustDown(proto.InputMouseButtonLeft)
	if err := page.Mouse.MoveLinear(proto.NewPoint(centers.Get("mx").Num(), centers.Get("my").Num()), 8); err != nil {
		t.Fatal(err)
	}
	if err := page.Mouse.MoveLinear(proto.NewPoint(centers.Get("bx").Num(), centers.Get("by").Num()), 8); err != nil {
		t.Fatal(err)
	}
	page.Mouse.MustUp(proto.InputMouseButtonLeft)
	time.Sleep(300 * time.Millisecond)
	page.MustEval(script(`
        if(editor.getTableCellSelection()?.cellIds.length!==9)throw Error('dragging selected '+JSON.stringify(editor.getTableCellSelection()));
        // Red thick 1.5 pt lines; the edge buttons draw with that pen, each as one undo step.
        const red=()=>[...document.querySelectorAll('.docx-page .docx-table-cell')].map(cell=>[...cell.querySelectorAll('.docx-table-border-edge-stroke')].filter(line=>/rgb\(255, 0, 0\)/.test(line.getAttribute('style'))).length);
        const all=()=>[...document.querySelectorAll('.docx-page .docx-table-cell')].map(cell=>cell.querySelectorAll('.docx-table-border-edge-stroke,.docx-table-border-double-stroke').length).reduce((a,b)=>a+b,0);
        const press=async edge=>{document.querySelector('[data-right] [data-border="'+edge+'"]').click();await wait(300);};
        const edges=[...document.querySelectorAll('[data-right] [data-border]')].map(button=>button.dataset.border+':'+button.textContent).join();
        if(edges!=='all:Borders,outside:Outside,inside:Inside,top:Top,bottom:Bottom,left:Left,right:Right,none:No borders')throw Error('the edge buttons read '+edges);
        if([...field('borderStyle').options].map(option=>option.textContent).join()!=='Solid,Double,Triple,Thick')throw Error('the line styles read '+field('borderStyle').textContent);
        if(field('borderWidth').options.length!==6||!/pt$/.test(field('borderWidth').options[0].textContent))throw Error('the line widths read '+field('borderWidth').textContent);
        await press('none');
        if(all()!==0)throw Error('No borders left '+all()+' lines');
        await set('borderStyle','thick');await set('borderWidth',12);await set('borderColor','#ff0000');
        if(all()!==0)throw Error('choosing a pen changed the document');
        await press('outside');
        const outline=red();
        if(outline[4]!==0||outline.reduce((a,b)=>a+b,0)!==12)throw Error('Outside drew '+JSON.stringify(outline));
        await press('inside');
        if(red().reduce((a,b)=>a+b,0)!==24)throw Error('Inside drew '+JSON.stringify(red()));
        run({type:'undo'});await wait(300);
        if(red().reduce((a,b)=>a+b,0)!==12)throw Error('one undo did not take Inside back: '+JSON.stringify(red()));
        // Double lines are drawn too, and the pen stays for the next click and for a reopened panel.
        await set('borderStyle','double');
        await press('all');
        if(!document.querySelector('.docx-page .docx-table-cell .docx-table-border-double-stroke'))throw Error('All drew no double line');
        await app.act('format');await wait(200);await app.act('format');await wait(300);
        if(field('borderStyle').value!=='double'||field('borderWidth').value!=='12'||field('borderColor').value!=='#ff0000')throw Error('the pen was forgotten: '+field('borderStyle').value+' '+field('borderWidth').value+' '+field('borderColor').value);
        // Outside a table the edge buttons are off.
        const last=editor.surface.session.paragraphIds().at(-1);
        run({type:'setSelection',range:{anchor:{paragraphId:last,offset:0},head:{paragraphId:last,offset:0}}});
        await wait(400);
        if(editor.snapshot().table||!document.querySelector('[data-right] [data-border="all"]').disabled)throw Error('the edge buttons stay on outside a table');
        // A picture after the table.
        const canvas=document.createElement('canvas');canvas.width=64;canvas.height=48;canvas.getContext('2d').fillRect(0,0,64,48);
        const blob=await new Promise(r=>canvas.toBlob(r,'image/png')),transfer=new DataTransfer();transfer.items.add(new File([blob],'objects.png',{type:'image/png'}));
        const input=document.querySelector('[data-image]');input.files=transfer.files;input.dispatchEvent(new Event('change',{bubbles:true}));
        await until(()=>[...editor.surface.session.currentPackage().partBytes.keys()].some(part=>part.includes('/media/')),'the picture');
        await until(()=>document.querySelector('.docx-page .docx-drawing-ready'),'the decoded picture');
        document.querySelector('.docx-page .docx-drawing').scrollIntoView({block:'center'});
        await wait(200);
    `))
	box := page.MustEval(`()=>{const b=document.querySelector('.docx-page .docx-drawing').getBoundingClientRect();return {x:b.left+b.width/2,y:b.top+b.height/2};}`)
	page.Mouse.MustMoveTo(box.Get("x").Num(), box.Get("y").Num())
	page.Mouse.MustClick(proto.InputMouseButtonLeft)
	time.Sleep(400 * time.Millisecond)
	page.MustEval(script(`
        const image=()=>editor.snapshot().image;
        if(!image())throw Error('a click did not select the picture');
        await wait(300);
        const w0=image().widthEmu,h0=image().heightEmu;
        if(field('imageWidth').value!==u.show(w0/635)||field('imageHeight').value!==u.show(h0/635))throw Error('the size fields read '+field('imageWidth').value+' x '+field('imageHeight').value);
        if(!field('imageLock').checked)throw Error('the aspect lock starts off');
        // Locked: the height follows the width. Unlocked: one side only.
        await set('imageWidth',u.show(w0*2/635));
        if(Math.abs(image().widthEmu-w0*2)>5000||Math.abs(image().heightEmu-h0*2)>5000)throw Error('locked width gave '+image().widthEmu+' x '+image().heightEmu+' from '+w0+' x '+h0);
        await set('imageLock',false);
        const w1=image().widthEmu;
        await set('imageHeight',u.show(h0/635));
        if(image().widthEmu!==w1||Math.abs(image().heightEmu-h0)>5000)throw Error('unlocked height gave '+image().widthEmu+' x '+image().heightEmu);
        if(field('imageHeight').value!==u.show(image().heightEmu/635))throw Error('the height field reads '+field('imageHeight').value);
        // Crop in percent; what leaves nothing of the picture is refused with a reason.
        await set('cropLeft',10);await set('cropTop',20);
        if(image().crop.left!==10||image().crop.top!==20||image().crop.right!==0)throw Error('crop gave '+JSON.stringify(image().crop));
        field('cropRight').value='95';await set('cropLeft',10);
        if(said()!==text('crop_invalid')||image().crop.right!==0)throw Error('a crop of everything says '+said()+' and gave '+JSON.stringify(image().crop));
        document.querySelector('[data-right] [data-command*="crop"]').click();await wait(300);
        if(image().crop.left!==0||image().crop.top!==0||field('cropLeft').value!=='0')throw Error('Reset crop left '+JSON.stringify(image().crop)+' and the field at '+field('cropLeft').value);
        // A picture in the text cannot change its wrap, so the list is off instead of failing.
        if(image().canChangeWrap||!field('imageWrap').disabled)throw Error('the wrap list is on for a picture in the text');
        // Replace image: a square JPEG for a 4:3 PNG. The engine takes PNG only, so Writer converts; the height follows.
        const media=()=>[...editor.surface.session.currentPackage().partBytes.entries()].filter(([part])=>part.includes('/media/')).map(([part,bytes])=>part+':'+bytes.byteLength).join();
        const before=media(),w2=image().widthEmu;
        const square=document.createElement('canvas');square.width=120;square.height=120;const paint=square.getContext('2d');paint.fillStyle='#0a0';paint.fillRect(0,0,120,120);
        const jpeg=await new Promise(r=>square.toBlob(r,'image/jpeg')),chosen=new DataTransfer();chosen.items.add(new File([jpeg],'square.jpg',{type:'image/jpeg'}));
        const bar=[...document.querySelectorAll('[data-contextbar] button')].map(button=>button.textContent.trim()).join('|');
        if(!bar.includes(text('image_replace')))throw Error('the picture bar reads '+bar);
        const replace=document.querySelector('[data-image-replace]');replace.files=chosen.files;replace.dispatchEvent(new Event('change',{bubbles:true}));
        await until(()=>media()!==before,'the replaced picture; notice: '+said());
        await until(()=>image()&&Math.abs(image().heightEmu-image().widthEmu)<=1,'the height of the replaced picture: '+JSON.stringify({w:image()?.widthEmu,h:image()?.heightEmu,notice:said()}));
        if(image().widthEmu!==w2)throw Error('replacing changed the width from '+w2+' to '+image().widthEmu);
        if(!/\.png:/.test(media()))throw Error('the replaced file is '+media());
        // The style list: every paragraph style, drawn the way it looks where the browser can.
        const styles=document.querySelector('[data-slot="styles.style"]'),options=[...styles.options];
        if(options.length<16)throw Error('the style list has '+options.length+' entries');
        if(CSS.supports('appearance','base-select')) {
            const look=id=>options.find(option=>option.value===id).getAttribute('style')||'';
            if(!/font-size:22px/.test(look('Title'))||!/color:#2F5496/.test(look('Heading1'))||!/font-style:italic/.test(look('Quote')))throw Error('the previews read '+look('Title')+' | '+look('Heading1')+' | '+look('Quote'));
            if(getComputedStyle(styles).appearance!=='base-select')throw Error('the style list is drawn as '+getComputedStyle(styles).appearance);
        } else if(options.some(option=>option.hasAttribute('style')))throw Error('a plain list carries previews');
        const first=editor.surface.session.paragraphIds().at(-1);
        run({type:'setSelection',range:{anchor:{paragraphId:first,offset:0},head:{paragraphId:first,offset:0}}});
        styles.value='Heading1';styles.dispatchEvent(new Event('change',{bubbles:true}));await wait(300);
        if(editor.snapshot().formatting.styleId!=='Heading1')throw Error('choosing a style gave '+editor.snapshot().formatting.styleId);
        document.querySelector('[data-scroll]').scrollTop=0;
        field('cellFill').scrollIntoView({block:'start'});
    `))

	// The panel's table and picture sections and the open list, for the eye.
	if err := os.MkdirAll("../reports/autor", 0755); err != nil {
		t.Fatal(err)
	}
	page.MustScreenshot("../reports/autor/objects-panel.png")
	list := page.MustEval(`()=>{const b=document.querySelector('[data-slot="styles.style"]').getBoundingClientRect();return {x:b.left+b.width/2,y:b.top+b.height/2};}`)
	page.Mouse.MustMoveTo(list.Get("x").Num(), list.Get("y").Num())
	page.Mouse.MustClick(proto.InputMouseButtonLeft)
	time.Sleep(400 * time.Millisecond)
	page.MustScreenshot("../reports/autor/styles-gallery.png")
	page.Keyboard.MustType(input.Escape)
}
```

- [ ] **Step 3: Check and commit**

Run: `gofmt -l ui/desktop_writer_objects_test.go` (nothing) and `go vet ./ui` — expected: clean.

The controller runs `AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run "TestDesktop(Writer|QR)" -count=1 -timeout 30m -v` and looks at `reports/autor/styles-gallery.png` and `reports/autor/objects-panel.png`.

Commit as `test(writer): borders, pictures and the style list in the browser`.

---

### Task 7: Documentation and DOX pass

**Files:**
- Modify: `ui/js/desktop/apps/AGENTS.md`, `documentation/autor.md`

- [ ] **Step 1: `ui/js/desktop/apps/AGENTS.md`**

In the Writer section, after the paragraph about `writer-pdf.js` (it ends with `asks before `overwrite=1`;`) add:

```text
  `writer-objects.js` turns the Format panel's table and picture fields into engine commands and
  touches no editor. Borders are drawn with a pen (one of four line styles, width in eighths of a point, colour;
  dashed and dotted are left out because the engine does not paint them on a left edge that had
  no line before)
  that the panel keeps per window; `borders` in the registry is marked `pen` and takes it through
  the host. A border command acts on the engine's cell selection, which only a mouse drag across
  cells makes, and otherwise on the one cell at the caret: never loop over a table's cells to
  border "the whole table", because each cell would be its own undo step (the engine's history
  groups cover text and paragraph formatting only). A picture's size goes through `sizeCommand`
  (locked keeps the proportions), its crop is in percent of the picture. `replaceImage` in
  `writer.js` keeps the width, lets the height follow the new picture and tries the file as it
  is, then as PNG, then as JPEG, because the engine takes only the format the picture has. The
  wrap list is off where `snapshot().image.canChangeWrap` is false: a picture inserted here is
  inline and the engine cannot anchor it. Repeat header row, table alignment and picture position
  are not offered for the same reason: the engine refuses the commands. The toolbar's style list
  stays a `<select>`; with `appearance: base-select` its options carry `stylePreview` as inline
  styles, and without it the list is plain;
```

- [ ] **Step 2: `documentation/autor.md`**

In the section "Page and paragraph" (or the section that describes tables and pictures, if there is one; read the file first), add a subsection "Tables and pictures" with this text:

```text
The Format panel draws table borders with a pen: choose the line (solid,
double, triple or thick), its width and its colour, then press
an edge button (Borders, Outside, Inside, Top, Bottom, Left, Right or No
borders). The buttons act on the selected cells; drag across cells to select
several, otherwise the cell with the caret is meant. **Borders** in the table
bar and in the Table menu draws all edges with the same pen.

A selected picture has a width and a height; **Keep aspect ratio** changes
both together. **Crop** hides a share of the picture on each side, in percent,
and **Reset crop** shows all of it again. **Replace image…** puts another file
in the picture's place, keeps the width and adjusts the height. Text wrapping
can be changed only for a picture that an imported document has anchored; a
picture inserted in Writer stays in the line of text.

The style list in the toolbar shows every paragraph style in its own font,
size and colour in Chrome and Edge; other browsers show the names.
```

In "Build and verification", name the new node test next to the others: `the objects module (`scripts/test-writer-objects.mjs`)`.

- [ ] **Step 3: DOX pass and commit**

Re-read the `AGENTS.md` chain of every touched path and check that no statement about table borders, the picture panel or the style list is stale (`git grep -n "no_borders\|image_width\|styles.style" -- '*.md'`). Commit as `docs(writer): borders, pictures and the style list`.

---

## Changes after the branch review (2026-11-15)

One review of the whole branch found nothing critical. These findings were fixed in one round; the code blocks above show the state before it:

- **"Replace image…" in the Format panel follows the command's state.** `update()` in `writer-panels.js` disables the button without a selected picture, in a read-only window and while changes are tracked, and `case 'imageReplace'` in `writer.js` checks `commands.state('imageReplace')` before it opens the file picker. Before, the picker opened and the refusal came after the file was chosen.
- **The height after a replacement goes to the replaced picture only.** `replaceImage` checks once more, after the engine has answered, that the window is alive, the editor is the same and the same picture is still selected.
- **The height respects a crop.** `fitHeight` takes the picture's crop: the frame keeps the proportions of the part that is shown.
- **Refusals read as sentences.** A file the engine cannot take in any of the three formats is answered with `image_format` instead of the engine's code, and a picture beyond the browser's canvas limits no longer ends in a script error.
- **The browser check covers the panel button**: off without a picture, on for a selected one.

Seen and left as they are:

- A rotated picture keeps the plain width-to-height rule after a replacement; how the engine reads width and height of a rotated picture was not probed.
- The four crop fields are sent together from the panel. They are refreshed with every panel update; nothing else in Writer changes a crop.
