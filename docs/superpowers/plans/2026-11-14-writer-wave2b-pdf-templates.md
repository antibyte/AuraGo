# Writer Wave 2b — PDF through the print path, templates: implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Every printed page keeps the format of its own section, File > Save as PDF… leads through the print dialog, four built-in templates (letter, invoice, minutes, report) replace the three text snippets, and the user can save a document as a template and start a new one from it.

**Architecture:** One new module, `writer-templates.js`, holds the built-in templates as recipes (paper, margins, blocks, footer), the rules for the user's own templates, and `build`, which turns a recipe into DOCX bytes in a scratch editor. `load(target, template)` in `writer.js` then starts every templated document from finished bytes, whether they were built or read from the user's templates folder. Printing gives each painted page a named `@page` of its own size (`WriterLayout.sheets`), and the shared print runtime learns to name the job.

**Tech stack:** Vanilla JS classic scripts (`module-loader.js` is a standalone script; `print-runtime.js` is part of `main.bundle.js`), `@docx-editor.dev/core` 2.29.0 behind `lib`, node `vm` tests, Go + rod browser tests.

Roadmap: `docs/superpowers/specs/2026-11-12-writer-office-roadmap-design.md`, section "Wave 2 — Business documents". This is part 2b; a server-side one-click PDF follows as its own part, then 2c.

---

## Read this first

**Decided by the user (2026-11-14):** built-in templates are recipes that Writer builds when a document is created, not `.docx` files in the repo. The user's own templates are real DOCX files in the workspace. A one-click PDF from the server is wanted too and is NOT part of this plan.

**Scope.** In: per-page print format; File > Save as PDF… (the print dialog with a hint and the document's name); File > New from template… (built-in and own templates in one list); File > Save as template…; the templates letter (after DIN 5008, approximated), invoice, minutes and report, also as tiles in the Insert panel. Out: server-side PDF; preview pictures in the template dialog; renaming or deleting templates inside Writer (the Files app does it); own paragraph styles in built-in templates (the engine cannot create styles); a positioned address frame in the letter.

**Engine and browser behaviour, probed on 2026-11-14.** These are facts, not assumptions:

- Pasted HTML (`{type:'paste', text:'', html}`) keeps `text-align`, `font-size` in pt, `<strong>`, `margin-top` and `margin-bottom` in pt on a `<p>` (they become space before and after), `<ul>` and `<ol>`. A paragraph style cannot be pasted; `lib.commandForSlotValue('styles.style', id)` sets one at the caret. A blank document has the styles `Title`, `Subtitle` and `Heading1` to `Heading9`.
- In a pasted table the column width is read from the `width` attribute of a `<td>` only (as a proportion; the table is always 9360 twips wide), not from `<colgroup>` or a `width` style. Borders come from `border` in the cell's style. `text-align` on a `<td>` is ignored; a `<p style="text-align:right">` inside the cell works. `<th>` is centred and bold, so the recipes use `<td>`.
- `setTableBorders` with `scope:'all'` borders the selected cell only. The recipes therefore carry their borders in the HTML.
- A footer is built by commands, in this order, each answering `ok`: `editHeaderFooter {position:'footer'}`, `setAlignment`, the font size, `insertPageField {field:'PAGE'}`, `paste`, `insertPageField {field:'NUMPAGES'}`, `exitHeaderFooter`. `lib.commandForSlotValue('font.size', n)` takes half-points (18 is 9 pt).
- All of this works in a scratch editor created in an attached, empty element, the way the read-only import in `load()` already does it. `scratch.destroy()` leaves the element empty. Building one template takes 300 to 500 ms.
- A document that starts from built bytes has `snapshot().canUndo === false`. The pre-existing bug "undo right after New reverts the paper" does not touch templated documents.
- After `paintSemanticLayout` every `.docx-page` carries its own size as inline `width` and `height` in px. A landscape section is wider than tall there.
- Chrome prints mixed formats when each page element has the CSS property `page: <name>` and the sheet has `@page <name>{size:<w> <h>;margin:0}`. Proven with `Page.printToPDF`: the PDF's page boxes were 595 x 842 and 842 x 595. With today's single `@page` both pages came out portrait.

**Server behaviour, read from the code on 2026-11-14:**

- `PUT /api/desktop/office/document?representation=docx&path=Documents/Templates/x.docx` creates the missing folder (`internal/desktop/atomic_write.go`). No call to `/api/desktop/directory` is needed.
- `GET /api/desktop/files?path=Documents/Templates` answers 400 for a folder that does not exist, and `{"status":"ok","files":[{name, path, type:"file"|"directory", ...}]}` otherwise, directories first, then by name.
- `GET` of a DOCX that does not exist answers 404; a `PUT` needs `If-None-Match: *` for a new file or `If-Match` for an existing one.

**Dry run, 2026-11-14.** This plan was applied to a copy exactly as written: all 36 "replace … with" pairs matched once, the eight node tests passed with the planned failures on the way, `go vet ./ui` was clean, and `TestDesktop(Writer|QR)` passed in the browser, including the new templates check and the print with mixed formats. Implement it as it stands; a block that does not match means the file changed since, so stop and report instead of adapting it.

**Not verifiable here:** which title Chrome's print dialog proposes as the PDF file name. The plan sets the name on the print frame's document and, while the job is open, on the page itself. The user checks the real dialog.

**Repo rules:**

- Read the `AGENTS.md` chain for every path you touch before editing (root, `ui/AGENTS.md`, `ui/js/desktop/apps/AGENTS.md`, `scripts/AGENTS.md`); close out with a DOX pass (Task 6).
- Every user-visible string exists in all 16 files under `ui/lang/desktop/`. `scripts/test-writer-commands.mjs` scans every registry label and every literal `tr('key')` in `ui/js/desktop/apps/writer*.js`; write `tr('key')` with a literal key.
- Every file under `ui/js/desktop` stays below 1100 lines. No Internet assets, no new dependencies, no binary files.
- `ui/desktop_office_app_markers_test.go` pins these strings in `writer.js`: `window.WriterApp={render,dispose,instances}`, `queue?.dispose()`, `dispose(windowId);`, `editor?.destroy()`, `/api/desktop/office/document`; and in `writer-menus.js`: `item('new')`, `item('saveAs')`, `id:'agent'`, `labelKey:'desktop.menu_agent'`, `item('assist')`, `setWindowMenus`.
- A command is one entry in `DEFINITIONS` in `writer-commands.js`; `scripts/test-writer-menus.mjs` fails for a command no window menu lists. An app action also needs its `case` in `act` in `writer.js`.
- `ui/js/desktop/core/print-runtime.js` is shared by Notes, Writer, Sheets and Viewer; only add to it.
- Temporary scripts go into `disposable/_writer-wave2b/` and are deleted before the commit. Before each commit: `git diff --cached --check`, and look at `git diff --cached`.
- GitNexus: the index is hundreds of commits behind and cannot index worktrees. Controller's impact analysis for `load` in `writer.js`, `print` and `template` in `writer-panels.js` and `create` in `print-runtime.js`: UNKNOWN, callers confirmed by text search (`panels.template` is called from `load` only; `panels.print` from `act` only; `AuraDesktopPrint.create` is called with an options object by Notes, Writer, Sheets and Viewer, and a new optional field changes none of them). End each commit body with: `GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.`

**Environment (Windows):**

- Work in the worktree `.claude/worktrees/writer-wave2b` (it has a real copy of `node_modules`).
- In a worktree session git commands must be plain and single (no `&&`, no `cd`).
- Browser tests: `AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run "TestDesktop(Writer|QR)" -count=1 -timeout 30m -v`, from PowerShell with the sandbox off. Delete `reports\autor` first. Implementer subagents cannot start Chrome; the controller runs these.
- Static tests: `npm run test:writer`, `npm run check:ui`, `go test ./ui -run "(?i)(windowmenu|contextmenu|materials|office|writer|virtualdesktop|lifecycle|i18n|feeling|print)" -count=1`.
- `TestDesktopCodeStudioRunUsesTerminalSession` and `TestDesktopPolishPointerLightTargetsMatch` fail on `main` for unrelated reasons. Ignore them.

## File structure

| File | Change | Responsibility |
| --- | --- | --- |
| `ui/lang/desktop/*.json` | modify | 39 new keys in 16 languages; `print` becomes "Print…" |
| `ui/js/desktop/apps/writer-templates.js` | create | Recipes of the built-in templates, their HTML, `build`, the rules for the user's own templates |
| `scripts/test-writer-templates.mjs`, `package.json` | create, modify | Its test, part of `npm run test:writer` |
| `ui/js/desktop/apps/writer-layout.js`, `scripts/test-writer-layout.mjs` | modify | `sheets(sizes)`: one named page per page size |
| `ui/js/desktop/core/print-runtime.js`, `ui/js/desktop/bundles/main.bundle.js` | modify | `documentTitle`; rebuilt bundle |
| `ui/js/desktop/apps/writer-panels.js` | modify | Print with a page per size and a title; template tiles from the catalogue; `template()` leaves |
| `ui/js/desktop/apps/writer-commands.js`, `writer-menus.js` | modify | Three new commands in the File menu |
| `ui/js/desktop/apps/writer-dialogs.js` | modify | `templates()`, `saveTemplate()` |
| `ui/js/desktop/apps/writer.js` | modify | `load` takes a template; `ownTemplates`, `newFromTemplate`, `saveTemplate`; four `act` cases |
| `ui/js/desktop/core/module-loader.js` | modify | One new script |
| `scripts/test-writer-menus.mjs`, `test-writer-commands.mjs` | modify | New rules |
| `ui/desktop_office_assets_test.go`, `ui/desktop_writer_browser_test.go`, `ui/desktop_writer_reload_browser_test.go` | modify | Script lists; print with mixed formats; a folder listing in the file-ops fixture |
| `ui/desktop_writer_templates_test.go` | create | Browser check |
| `ui/js/desktop/apps/AGENTS.md`, `documentation/autor.md` | modify | Documentation |

---

### Task 1: Language keys

**Files:**
- Modify: `ui/lang/desktop/*.json` (script)

- [ ] **Step 1: Add the keys and give `print` its new text**

Create `disposable/_writer-wave2b/i18n.mjs`:

```js
// Adds the Wave 2b keys to the sixteen desktop language files, each at its sorted place in the desktop.writer_ block,
// and gives "print" its new text: the PDF has its own menu entry now.
// Usage: node disposable/_writer-wave2b/i18n.mjs <repo root>
import {readFileSync, writeFileSync} from 'node:fs';
import path from 'node:path';

const root = process.argv[2] || process.cwd();
const LANGS = ['cs', 'da', 'de', 'el', 'en', 'es', 'fr', 'hi', 'it', 'ja', 'nl', 'no', 'pl', 'pt', 'sv', 'zh'];
const REPLACED = ['print'];
const KEYS = {
    print: ['Tisknout…', 'Udskriv…', 'Drucken…', 'Εκτύπωση…', 'Print…', 'Imprimir…', 'Imprimer…', 'प्रिंट…', 'Stampa…', '印刷…', 'Afdrukken…', 'Skriv ut…', 'Drukuj…', 'Imprimir…', 'Skriv ut…', '打印…'],
    pdf: ['Uložit jako PDF…', 'Gem som PDF…', 'Als PDF speichern…', 'Αποθήκευση ως PDF…', 'Save as PDF…', 'Guardar como PDF…', 'Enregistrer au format PDF…', 'PDF के रूप में सहेजें…', 'Salva come PDF…', 'PDFとして保存…', 'Opslaan als PDF…', 'Lagre som PDF…', 'Zapisz jako PDF…', 'Guardar como PDF…', 'Spara som PDF…', '另存为 PDF…'],
    pdf_hint: ['V dialogu tisku zvol jako cíl „Uložit jako PDF“.', 'Vælg i udskriftsdialogen „Gem som PDF“ som destination.', 'Wähle im Druckdialog als Ziel „Als PDF speichern“.', 'Στο παράθυρο εκτύπωσης επίλεξε ως προορισμό το «Αποθήκευση ως PDF».', 'In the print dialog, choose “Save as PDF” as the destination.', 'En el cuadro de diálogo de impresión, elige «Guardar como PDF» como destino.', 'Dans la boîte de dialogue d’impression, choisissez « Enregistrer au format PDF » comme destination.', 'प्रिंट संवाद में गंतव्य के रूप में “PDF के रूप में सहेजें” चुनें।', 'Nella finestra di stampa scegli «Salva come PDF» come destinazione.', '印刷ダイアログで、保存先として「PDFとして保存」を選択してください。', 'Kies in het afdrukvenster als bestemming „Opslaan als PDF“.', 'Velg i utskriftsdialogen «Lagre som PDF» som mål.', 'W oknie drukowania wybierz jako cel „Zapisz jako PDF”.', 'Na caixa de diálogo de impressão, escolha «Guardar como PDF» como destino.', 'Välj i utskriftsdialogen ”Spara som PDF” som mål.', '在打印对话框中，将目标选择为“另存为 PDF”。'],
    new_from_template: ['Nový z šablony…', 'Ny fra skabelon…', 'Neu aus Vorlage…', 'Νέο από πρότυπο…', 'New from template…', 'Nuevo desde plantilla…', 'Nouveau à partir d’un modèle…', 'टेम्पलेट से नया…', 'Nuovo da modello…', 'テンプレートから新規作成…', 'Nieuw op basis van sjabloon…', 'Nytt fra mal…', 'Nowy z szablonu…', 'Novo a partir de modelo…', 'Nytt från mall…', '从模板新建…'],
    save_template: ['Uložit jako šablonu…', 'Gem som skabelon…', 'Als Vorlage speichern…', 'Αποθήκευση ως πρότυπο…', 'Save as template…', 'Guardar como plantilla…', 'Enregistrer comme modèle…', 'टेम्पलेट के रूप में सहेजें…', 'Salva come modello…', 'テンプレートとして保存…', 'Opslaan als sjabloon…', 'Lagre som mal…', 'Zapisz jako szablon…', 'Guardar como modelo…', 'Spara som mall…', '另存为模板…'],
    template_pick: ['Šablona', 'Skabelon', 'Vorlage', 'Πρότυπο', 'Template', 'Plantilla', 'Modèle', 'टेम्पलेट', 'Modello', 'テンプレート', 'Sjabloon', 'Mal', 'Szablon', 'Modelo', 'Mall', '模板'],
    template_own: ['Moje šablona', 'Min skabelon', 'Eigene Vorlage', 'Το δικό μου πρότυπο', 'My template', 'Mi plantilla', 'Mon modèle', 'मेरा टेम्पलेट', 'Il mio modello', 'マイテンプレート', 'Mijn sjabloon', 'Min mal', 'Mój szablon', 'O meu modelo', 'Min mall', '我的模板'],
    template_use: ['Vytvořit dokument', 'Opret dokument', 'Dokument anlegen', 'Δημιουργία εγγράφου', 'Create document', 'Crear documento', 'Créer le document', 'दस्तावेज़ बनाएँ', 'Crea documento', 'ドキュメントを作成', 'Document maken', 'Opprett dokument', 'Utwórz dokument', 'Criar documento', 'Skapa dokument', '创建文档'],
    template_folder: ['Složka pro vlastní šablony', 'Mappe til dine egne skabeloner', 'Ordner für eigene Vorlagen', 'Φάκελος για τα δικά σου πρότυπα', 'Folder for your own templates', 'Carpeta para tus propias plantillas', 'Dossier de vos propres modèles', 'आपके अपने टेम्पलेट के लिए फ़ोल्डर', 'Cartella per i tuoi modelli', '独自テンプレート用のフォルダー', 'Map voor je eigen sjablonen', 'Mappe for dine egne maler', 'Folder na własne szablony', 'Pasta para os seus próprios modelos', 'Mapp för dina egna mallar', '用于自定义模板的文件夹'],
    template_name: ['Název šablony', 'Skabelonens navn', 'Name der Vorlage', 'Όνομα προτύπου', 'Template name', 'Nombre de la plantilla', 'Nom du modèle', 'टेम्पलेट का नाम', 'Nome del modello', 'テンプレート名', 'Naam van het sjabloon', 'Malnavn', 'Nazwa szablonu', 'Nome do modelo', 'Mallnamn', '模板名称'],
    template_name_invalid: ['Použij název o nejvýše 80 znacích bez lomítek, dvojteček, uvozovek a znaků * ? < > |.', 'Brug et navn på højst 80 tegn uden skråstreger, kolontegn, anførselstegn og tegnene * ? < > |.', 'Verwende einen Namen mit höchstens 80 Zeichen, ohne Schrägstriche, Doppelpunkte, Anführungszeichen und ohne die Zeichen * ? < > |.', 'Χρησιμοποίησε όνομα έως 80 χαρακτήρων χωρίς κάθετους, άνω τελείες, εισαγωγικά ή τους χαρακτήρες * ? < > |.', 'Use a name of up to 80 characters without slashes, colons, quotation marks or the characters * ? < > |.', 'Usa un nombre de hasta 80 caracteres sin barras, dos puntos, comillas ni los caracteres * ? < > |.', 'Utilisez un nom de 80 caractères maximum, sans barres obliques, deux-points, guillemets ni les caractères * ? < > |.', '80 अक्षरों तक का नाम उपयोग करें, जिसमें स्लैश, कोलन, उद्धरण चिह्न या अक्षर * ? < > | न हों।', 'Usa un nome di massimo 80 caratteri, senza barre, due punti, virgolette o i caratteri * ? < > |.', '80文字以内で、スラッシュ、コロン、引用符、および文字 * ? < > | を含まない名前を使用してください。', 'Gebruik een naam van maximaal 80 tekens zonder schuine strepen, dubbele punten, aanhalingstekens of de tekens * ? < > |.', 'Bruk et navn på høyst 80 tegn uten skråstreker, kolon, anførselstegn eller tegnene * ? < > |.', 'Użyj nazwy do 80 znaków, bez ukośników, dwukropków, cudzysłowów i znaków * ? < > |.', 'Use um nome com até 80 caracteres, sem barras, dois pontos, aspas nem os caracteres * ? < > |.', 'Använd ett namn på högst 80 tecken utan snedstreck, kolon, citattecken eller tecknen * ? < > |.', '请使用不超过 80 个字符的名称，不含斜杠、冒号、引号以及字符 * ? < > |。'],
    template_saved: ['Šablona „{{name}}“ uložena.', 'Skabelonen „{{name}}“ er gemt.', 'Vorlage „{{name}}“ gespeichert.', 'Το πρότυπο «{{name}}» αποθηκεύτηκε.', 'Template “{{name}}” saved.', 'Plantilla «{{name}}» guardada.', 'Modèle « {{name}} » enregistré.', 'टेम्पलेट “{{name}}” सहेजा गया।', 'Modello «{{name}}» salvato.', 'テンプレート「{{name}}」を保存しました。', 'Sjabloon „{{name}}“ opgeslagen.', 'Malen «{{name}}» er lagret.', 'Szablon „{{name}}” zapisano.', 'Modelo «{{name}}» guardado.', 'Mallen ”{{name}}” sparad.', '模板“{{name}}”已保存。'],
    template_failed: ['Šablonu nelze otevřít.', 'Skabelonen kunne ikke åbnes.', 'Die Vorlage konnte nicht geöffnet werden.', 'Το πρότυπο δεν μπόρεσε να ανοιχτεί.', 'The template could not be opened.', 'No se pudo abrir la plantilla.', 'Le modèle n’a pas pu être ouvert.', 'टेम्पलेट खोला नहीं जा सका।', 'Impossibile aprire il modello.', 'テンプレートを開けませんでした。', 'Het sjabloon kon niet worden geopend.', 'Malen kunne ikke åpnes.', 'Nie udało się otworzyć szablonu.', 'Não foi possível abrir o modelo.', 'Mallen kunde inte öppnas.', '无法打开该模板。'],
    template_sender_name: ['Vaše jméno nebo firma', 'Dit navn eller firma', 'Dein Name oder deine Firma', 'Το όνομα ή η εταιρεία σας', 'Your name or company', 'Su nombre o empresa', 'Votre nom ou société', 'आपका नाम या कंपनी', 'Nome o ragione sociale', 'お名前または会社名', 'Uw naam of bedrijfsnaam', 'Ditt navn eller firma', 'Imię i nazwisko lub nazwa firmy', 'O seu nome ou empresa', 'Ditt namn eller företag', '您的姓名或公司名称'],
    template_street: ['Ulice a číslo popisné', 'Vejnavn og husnummer', 'Straße und Hausnummer', 'Οδός και αριθμός', 'Street and number', 'Calle y número', 'Rue et numéro', 'सड़क और मकान नंबर', 'Via e numero civico', '通り名と番地', 'Straat en huisnummer', 'Gate og husnummer', 'Ulica i numer domu', 'Rua e número', 'Gata och nummer', '街道和门牌号'],
    template_city: ['PSČ a město', 'Postnummer og by', 'PLZ und Ort', 'Ταχυδρομικός κώδικας και πόλη', 'Postcode and town', 'Código postal y localidad', 'Code postal et ville', 'पिन कोड और शहर', 'CAP e città', '郵便番号と市区町村', 'Postcode en plaats', 'Postnummer og sted', 'Kod pocztowy i miejscowość', 'Código postal e localidade', 'Postnummer och ort', '邮政编码和城市'],
    template_sender_contact: ['Telefon · E-mail', 'Telefon · E-mail', 'Telefon · E-Mail', 'Τηλέφωνο · Email', 'Phone · Email', 'Teléfono · Correo electrónico', 'Téléphone · E-mail', 'फ़ोन · ईमेल', 'Telefono · E-mail', '電話 · メール', 'Telefoon · E-mail', 'Telefon · E-post', 'Telefon · E-mail', 'Telefone · E-mail', 'Telefon · E-post', '电话 · 电子邮件'],
    template_recipient_name: ['Jméno příjemce', 'Modtagerens navn', 'Name des Empfängers', 'Όνομα παραλήπτη', 'Recipient’s name', 'Nombre del destinatario', 'Nom du destinataire', 'प्राप्तकर्ता का नाम', 'Nome del destinatario', '宛名', 'Naam van de ontvanger', 'Mottakerens navn', 'Imię i nazwisko adresata', 'Nome do destinatário', 'Mottagarens namn', '收件人姓名'],
    template_signature: ['Vaše jméno', 'Dit navn', 'Dein Name', 'Το όνομά σας', 'Your name', 'Su nombre', 'Votre nom', 'आपका नाम', 'Nome e cognome', 'お名前', 'Uw naam', 'Ditt navn', 'Twoje imię', 'O seu nome', 'Ditt namn', '您的姓名'],
    invoice: ['Faktura', 'Faktura', 'Rechnung', 'Τιμολόγιο', 'Invoice', 'Factura', 'Facture', 'इनवॉइस', 'Fattura', '請求書', 'Factuur', 'Faktura', 'Faktura', 'Fatura', 'Faktura', '发票'],
    invoice_number: ['Číslo faktury', 'Fakturanummer', 'Rechnungsnummer', 'Αριθμός τιμολογίου', 'Invoice number', 'Número de factura', 'Numéro de facture', 'इनवॉइस नंबर', 'Numero fattura', '請求書番号', 'Factuurnummer', 'Fakturanummer', 'Numer faktury', 'Número da fatura', 'Fakturanummer', '发票编号'],
    invoice_date: ['Datum vystavení', 'Fakturadato', 'Rechnungsdatum', 'Ημερομηνία τιμολογίου', 'Invoice date', 'Fecha de factura', 'Date de facture', 'इनवॉइस की तारीख', 'Data fattura', '請求日', 'Factuurdatum', 'Fakturadato', 'Data faktury', 'Data da fatura', 'Fakturadatum', '发票日期'],
    invoice_customer: ['Číslo zákazníka', 'Kundenummer', 'Kundennummer', 'Αριθμός πελάτη', 'Customer number', 'Número de cliente', 'Numéro client', 'ग्राहक नंबर', 'Numero cliente', '顧客番号', 'Klantnummer', 'Kundenummer', 'Numer klienta', 'Número de cliente', 'Kundnummer', '客户编号'],
    invoice_intro: ['Za následující služby vám účtujeme:', 'Vi fakturerer hermed følgende ydelser:', 'Wir berechnen Ihnen folgende Leistungen:', 'Σας χρεώνουμε τις ακόλουθες υπηρεσίες:', 'We charge you for the following services:', 'Le facturamos los siguientes servicios:', 'Nous vous facturons les prestations suivantes :', 'हम आपसे निम्नलिखित सेवाओं का शुल्क ले रहे हैं:', 'Le addebitiamo i seguenti servizi:', '以下のサービスについて請求させていただきます：', 'Wij factureren u de volgende diensten:', 'Vi fakturerer herved følgende tjenester:', 'Obciążamy Państwa za następujące usługi:', 'Cobramos-lhe os seguintes serviços:', 'Vi debiterar er för följande tjänster:', '我们将就以下服务向您收费：'],
    invoice_pos: ['Pol.', 'Pos.', 'Pos.', 'Α/Α', 'Item', 'Pos.', 'Pos.', 'क्र.', 'Pos.', 'No.', 'Pos.', 'Pos.', 'Poz.', 'Pos.', 'Pos.', '序号'],
    invoice_description: ['Popis', 'Beskrivelse', 'Beschreibung', 'Περιγραφή', 'Description', 'Descripción', 'Description', 'विवरण', 'Descrizione', '内容', 'Omschrijving', 'Beskrivelse', 'Opis', 'Descrição', 'Beskrivning', '描述'],
    invoice_quantity: ['Množství', 'Antal', 'Menge', 'Ποσότητα', 'Quantity', 'Cantidad', 'Quantité', 'मात्रा', 'Quantità', '数量', 'Aantal', 'Antall', 'Ilość', 'Quantidade', 'Antal', '数量'],
    invoice_unit_price: ['Cena za jednotku', 'Stykpris', 'Einzelpreis', 'Τιμή μονάδας', 'Unit price', 'Precio unitario', 'Prix unitaire', 'प्रति इकाई मूल्य', 'Prezzo unitario', '単価', 'Stukprijs', 'Enhetspris', 'Cena jednostkowa', 'Preço unitário', 'Styckpris', '单价'],
    invoice_amount: ['Částka', 'Beløb', 'Betrag', 'Ποσό', 'Amount', 'Importe', 'Montant', 'राशि', 'Importo', '金額', 'Bedrag', 'Beløp', 'Kwota', 'Montante', 'Belopp', '金额'],
    invoice_net: ['Celkem bez DPH', 'Nettobeløb', 'Nettobetrag', 'Καθαρή αξία', 'Net total', 'Total neto', 'Total HT', 'कुल शुद्ध राशि', 'Totale netto', '税抜合計', 'Totaal netto', 'Nettobeløp', 'Suma netto', 'Total sem IVA', 'Nettobelopp', '不含税合计'],
    invoice_tax: ['DPH', 'moms', 'Umsatzsteuer', 'ΦΠΑ', 'VAT', 'IVA', 'TVA', 'जीएसटी', 'IVA', '消費税', 'btw', 'mva', 'VAT', 'IVA', 'moms', '增值税'],
    invoice_total: ['Celkem', 'I alt', 'Gesamtbetrag', 'Σύνολο', 'Total', 'Total', 'Total TTC', 'कुल', 'Totale', '合計', 'Totaal', 'Totalt', 'Razem', 'Total', 'Totalt', '总计'],
    invoice_terms: ['Celkovou částku uhraďte do 14 dnů.', 'Betal venligst det samlede beløb inden for 14 dage.', 'Bitte überweisen Sie den Gesamtbetrag innerhalb von 14 Tagen.', 'Παρακαλούμε εξοφλήστε το σύνολο εντός 14 ημερών.', 'Please pay the total within 14 days.', 'Le rogamos que abone el importe total en un plazo de 14 días.', 'Veuillez régler le montant total dans un délai de 14 jours.', 'कृपया 14 दिनों के भीतर कुल राशि का भुगतान करें।', 'La preghiamo di saldare l’importo totale entro 14 giorni.', '14日以内に合計金額をお支払いください。', 'Wij verzoeken u het totaalbedrag binnen 14 dagen te betalen.', 'Vennligst betal totalbeløpet innen 14 dager.', 'Prosimy o uregulowanie całej kwoty w ciągu 14 dni.', 'Queira pagar o total no prazo de 14 dias.', 'Vänligen betala det totala beloppet inom 14 dagar.', '请在 14 天内支付总金额。'],
    invoice_bank: ['Bankovní spojení: IBAN · BIC', 'Bankoplysninger: IBAN · BIC', 'Bankverbindung: IBAN · BIC', 'Τραπεζικά στοιχεία: IBAN · BIC', 'Bank details: IBAN · BIC', 'Datos bancarios: IBAN · BIC', 'Coordonnées bancaires : IBAN · BIC', 'बैंक विवरण: IBAN · BIC', 'Coordinate bancarie: IBAN · BIC', '銀行口座：IBAN · BIC', 'Bankgegevens: IBAN · BIC', 'Bankopplysninger: IBAN · BIC', 'Dane bankowe: IBAN · BIC', 'Dados bancários: IBAN · BIC', 'Bankuppgifter: IBAN · BIC', '银行信息：IBAN · BIC'],
    minutes: ['Zápis', 'Referat', 'Protokoll', 'Πρακτικά', 'Minutes', 'Acta', 'Procès-verbal', 'कार्यवृत्त', 'Verbale', '議事録', 'Notulen', 'Referat', 'Protokół', 'Ata', 'Protokoll', '会议纪要'],
    minutes_place: ['Místo', 'Sted', 'Ort', 'Τόπος', 'Place', 'Lugar', 'Lieu', 'स्थान', 'Luogo', '場所', 'Plaats', 'Sted', 'Miejsce', 'Local', 'Plats', '地点'],
    minutes_chair: ['Předseda', 'Ordstyrer', 'Leitung', 'Πρόεδρος', 'Chair', 'Presidencia', 'Présidence', 'अध्यक्ष', 'Presidenza', '議長', 'Voorzitter', 'Møteleder', 'Przewodniczący', 'Presidente', 'Ordförande', '主持人'],
    minutes_keeper: ['Zapsal', 'Referent', 'Protokollführung', 'Συντάκτης πρακτικών', 'Minutes by', 'Redactado por', 'Rédigé par', 'कार्यवृत्त लेखक', 'Verbale a cura di', '記録者', 'Notulen door', 'Referent', 'Protokołował', 'Ata por', 'Protokollförare', '记录人'],
    minutes_topic: ['Téma', 'Emne', 'Thema', 'Θέμα', 'Topic', 'Tema', 'Sujet', 'विषय', 'Argomento', '議題', 'Onderwerp', 'Tema', 'Temat', 'Assunto', 'Ämne', '议题'],
    report_author: ['Autor', 'Forfatter', 'Verfasser', 'Συγγραφέας', 'Author', 'Autor', 'Auteur', 'लेखक', 'Autore', '作成者', 'Auteur', 'Forfatter', 'Autor', 'Autor', 'Författare', '作者'],
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

Run from the repo root: `node disposable/_writer-wave2b/i18n.mjs .`

Expected: sixteen lines, each `<lang>: 39 keys added, 1 replaced`; `git diff --stat -- ui/lang/desktop` shows 16 files with 40 insertions and 1 deletion each. A second run prints `0 keys added, 0 replaced` sixteen times. Then delete `disposable/_writer-wave2b/`.

The keys `template_sender`, `template_recipient` and `meeting` stay in the files although Task 4 stops using them.

- [ ] **Step 2: Check and commit**

Run: `npm run test:writer` (it still passes; the new keys are not used yet) and `go test ./ui -run "(?i)i18n" -count=1`.

```
git add ui/lang/desktop
git diff --cached --check
git commit
```

Message:

```
feat(writer): texts for templates and Save as PDF

39 keys in sixteen languages: the template dialogs, the placeholder
texts of letter, invoice, minutes and report, and the PDF entry.
"Print / PDF" becomes "Print…" because the PDF gets its own entry.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: Claude Fable 5.2 <noreply@anthropic.com>
```

Between this commit and Task 4 the File menu shows "Print…" without a PDF entry. Tasks 1 to 4 land together.

---

### Task 2: The templates module

**Files:**
- Create: `ui/js/desktop/apps/writer-templates.js`
- Create: `scripts/test-writer-templates.mjs`
- Modify: `package.json`

- [ ] **Step 1: Write the failing test**

Create `scripts/test-writer-templates.mjs`:

```js
import assert from 'node:assert/strict';
import vm from 'node:vm';
import {readFileSync} from 'node:fs';

const context = {window: {}};
vm.runInNewContext(readFileSync('ui/js/desktop/apps/writer-layout.js', 'utf8'), context);
vm.runInNewContext(readFileSync('ui/js/desktop/apps/writer-templates.js', 'utf8'), context);
const T = context.window.WriterTemplates, tr = key => '[' + key + ']';
const same = (actual, expected, message) => assert.equal(JSON.stringify(actual), JSON.stringify(expected), message);

// The catalogue: four recipes, each with a paper, margins, blocks and a footer.
assert.equal(T.ids.join(), 'letter,invoice,minutes,report');
assert.equal(T.folder, 'Documents/Templates');
assert.equal(T.recipe('nope', tr), null);
for (const id of T.ids) {
    assert.equal(T.label(id, tr), '[' + id + ']');
    const plan = T.recipe(id, tr, {date: 'TODAY'});
    assert.equal(plan.id, id);
    same(plan.paper, {width: 11906, height: 16838}, id + ' is A4');
    same(plan.margins, [1417, 1134, 1134, 1417]);
    assert.equal(plan.footer, true);
    assert.ok(plan.blocks.length >= 8, id + ' has content');
    for (const block of plan.blocks) {
        assert.equal([block.text !== undefined, !!block.list, !!block.table].filter(Boolean).length, 1, id + ': a block is a paragraph, a list or a table');
        if (block.table) {
            for (const row of block.table) assert.equal(row.length, block.widths.length, id + ': every row has a width per cell');
            assert.equal(block.widths.reduce((sum, width) => sum + width, 0), 600, id + ': the widths add up to 600');
        }
    }
    const texts = JSON.stringify(plan.blocks);
    assert.ok(texts.includes('TODAY'), id + ' carries the date');
    assert.ok(!texts.includes('undefined'), id + ' has no missing text');
}
same(T.recipe('letter', tr, {inches: true}).paper, {width: 12240, height: 15840}, 'a user of inches gets Letter');
assert.notEqual(T.recipe('letter', tr).margins, T.recipe('invoice', tr).margins, 'margins are a copy');

// The letter: letterhead at the right, return line, recipient, date, subject, signature.
const letter = T.recipe('letter', tr, {date: 'TODAY'}).blocks;
same(letter[0], {text: '[template_sender_name]', align: 'right', bold: true, size: 14, after: 0});
assert.equal(letter[4].text, '[template_sender_name] · [template_street] · [template_city]');
assert.equal(letter[4].size, 8);
same(letter[8], {text: 'TODAY', align: 'right', before: 60});
assert.equal(letter.at(-1).text, '[template_signature]');
// The invoice: a five-column table with the amounts at the right and the total in bold.
const invoice = T.recipe('invoice', tr, {date: 'TODAY'}).blocks.find(block => block.table);
assert.equal(invoice.table[0].join(), '[invoice_pos],[invoice_description],[invoice_quantity],[invoice_unit_price],[invoice_amount]');
same(invoice.right, [2, 3, 4]);
assert.equal(invoice.head, true);
same(invoice.table.at(-1).slice(3), [{text: '[invoice_total]', bold: true}, {text: '…', bold: true}]);

// HTML for the engine's import: only what it is known to keep.
assert.equal(T.html([{text: 'A & B', align: 'right', size: 8, before: 6, after: 0}]), '<p style="text-align:right;font-size:8pt;margin-top:6pt;margin-bottom:0pt">A &amp; B</p>');
assert.equal(T.html([{text: 'Plain'}, {text: 'Bold', bold: true}]), '<p>Plain</p><p><strong>Bold</strong></p>');
assert.equal(T.html([{text: 'Styled', style: 'Heading1'}]), '<p>Styled</p>', 'a style is set after the import');
assert.equal(T.html([{list: ['a', '<b>']}, {list: ['c'], ordered: true}]), '<ul><li>a</li><li>&lt;b&gt;</li></ul><ol><li>c</li></ol>');
assert.equal(T.html([{table: [['H', 'N'], ['x', {text: 'y', bold: true}]], widths: [100, 500], head: true, right: [1]}]),
    '<table border="1" style="border-collapse:collapse">' +
    '<tr><td width="100" style="border:1px solid #64748b"><p style="margin-top:3pt;margin-bottom:3pt"><strong>H</strong></p></td>' +
    '<td width="500" style="border:1px solid #64748b"><p style="text-align:right;margin-top:3pt;margin-bottom:3pt"><strong>N</strong></p></td></tr>' +
    '<tr><td width="100" style="border:1px solid #64748b"><p style="margin-top:3pt;margin-bottom:3pt">x</p></td>' +
    '<td width="500" style="border:1px solid #64748b"><p style="text-align:right;margin-top:3pt;margin-bottom:3pt"><strong>y</strong></p></td></tr></table>');
same(T.styled(T.recipe('report', tr, {date: 'D'}).blocks), [['[report]', 'Title'], ['[report_author] · D', 'Subtitle'], ['[summary]', 'Heading1'], ['[findings]', 'Heading1'], ['[next_steps]', 'Heading1']]);
same(T.styled(T.recipe('letter', tr).blocks), []);

// A template's file name.
same(T.fileName('  House style '), {name: 'House style.docx'});
same(T.fileName('Brief.DOCX'), {name: 'Brief.docx'});
same(T.fileName('Angebot 2026 (Entwurf)'), {name: 'Angebot 2026 (Entwurf).docx'});
same(T.fileName('x'.repeat(80)), {name: 'x'.repeat(80) + '.docx'});
for (const bad of ['', '   ', '.docx', 'a/b', 'a' + String.fromCharCode(92) + 'b', 'a:b', 'what?', 'a*b', '"q"', '<x>', 'a|b', '.hidden', 'dot.', 'CON', 'lpt1', 'x'.repeat(81), 'tab' + String.fromCharCode(9) + 'here'])
    same(T.fileName(bad), {error: 'template_name_invalid'}, JSON.stringify(bad));

// The user's own templates out of a folder listing.
same(T.own([{name: 'B.docx', path: 'Documents/Templates/B.docx', type: 'file'}, {name: 'Old', path: 'Documents/Templates/Old', type: 'directory'},
    {name: 'notes.txt', path: 'Documents/Templates/notes.txt', type: 'file'}, {name: '~$B.docx', path: 'Documents/Templates/~$B.docx', type: 'file'},
    {name: '.aurago-write-1f.docx', path: 'Documents/Templates/.aurago-write-1f.docx', type: 'file'}, {name: 'a.DOCX', path: 'Documents/Templates/a.DOCX', type: 'file'}, null]),
    [{path: 'Documents/Templates/B.docx', title: 'B'}, {path: 'Documents/Templates/a.DOCX', title: 'a'}]);
same(T.own(undefined), []);
same(T.own({error: 'x'}), []);
// The name a document offers for its template.
assert.equal(T.title('Documents/Angebot.docx'), 'Angebot');
assert.equal(T.title('Documents/untitled-1a2b3c4d.docx'), '');
assert.equal(T.title(''), '');
console.log('Writer templates: recipes, import HTML, styles, file names and the own list pass');
```

- [ ] **Step 2: Run it to see it fail**

Run: `node scripts/test-writer-templates.mjs`
Expected: FAIL with `ENOENT` for `ui/js/desktop/apps/writer-templates.js`.

- [ ] **Step 3: Write the module**

Create `ui/js/desktop/apps/writer-templates.js`:

```js
(function () {
    'use strict';
    // The built-in templates and the rules for the user's own. A built-in template is a recipe: a paper, margins,
    // a list of blocks and a footer. build() turns a recipe into DOCX bytes in a scratch editor, so a document
    // made from it starts like one made from a user's own template: from finished bytes, with no undo steps.
    // A block is one of
    //   {text, style?, align?, size?, bold?, before?, after?}   a paragraph; style is a paragraph style id, size and
    //                                                            spacing are points
    //   {list: [text], ordered?}                                 a bulleted or numbered list
    //   {table: [[cell]], widths: [n], head?, right?: [column]}  a bordered table; a cell is a text or {text, bold};
    //                                                            widths are proportions, head makes the first row
    //                                                            bold, right names the right-aligned columns
    const ids = ['letter', 'invoice', 'minutes', 'report'];
    const folder = 'Documents/Templates';
    // Top, right, bottom, left: 25 mm at the top and the left, 20 mm at the right and the bottom. These are the
    // side margins of a DIN 5008 letter, and they leave the 6.5 inch the engine gives a pasted table on A4.
    const MARGINS = [1417, 1134, 1134, 1417];
    const names = tr => ({letter: tr('letter'), invoice: tr('invoice'), minutes: tr('minutes'), report: tr('report')});
    const label = (id, tr) => names(tr)[id];
    // The sender as a letterhead at the right, the small return line and the recipient. This follows the address
    // field of a DIN 5008 letter with paragraph spacing; the engine has no positioned frames.
    const address = tr => [
        {text: tr('template_sender_name'), align: 'right', bold: true, size: 14, after: 0},
        {text: tr('template_street'), align: 'right', after: 0},
        {text: tr('template_city'), align: 'right', after: 0},
        {text: tr('template_sender_contact'), align: 'right', after: 6},
        {text: [tr('template_sender_name'), tr('template_street'), tr('template_city')].join(' · '), size: 8, after: 6},
        {text: tr('template_recipient_name'), after: 0},
        {text: tr('template_street'), after: 0},
        {text: tr('template_city'), after: 0}
    ];
    const closing = tr => [{text: tr('template_closing'), before: 12, after: 36}, {text: tr('template_signature')}];
    const strong = text => ({text, bold: true});
    const RECIPES = {
        letter: (tr, date) => address(tr).concat([
            {text: date, align: 'right', before: 60},
            {text: tr('template_subject'), bold: true, before: 12, after: 24},
            {text: tr('template_greeting')},
            {text: tr('template_body')}], closing(tr)),
        invoice: (tr, date) => address(tr).concat([
            {text: tr('invoice_number') + ': …', align: 'right', before: 36, after: 0},
            {text: tr('invoice_date') + ': ' + date, align: 'right', after: 0},
            {text: tr('invoice_customer') + ': …', align: 'right'},
            {text: tr('invoice'), bold: true, size: 16, before: 12},
            {text: tr('template_greeting')},
            {text: tr('invoice_intro')},
            {table: [[tr('invoice_pos'), tr('invoice_description'), tr('invoice_quantity'), tr('invoice_unit_price'), tr('invoice_amount')],
                ['1', '…', '…', '…', '…'], ['2', '…', '…', '…', '…'],
                ['', '', '', tr('invoice_net'), '…'], ['', '', '', tr('invoice_tax'), '…'], ['', '', '', strong(tr('invoice_total')), strong('…')]],
                widths: [50, 250, 80, 110, 110], head: true, right: [2, 3, 4]},
            {text: tr('invoice_terms'), before: 12},
            {text: tr('invoice_bank')}], closing(tr)),
        minutes: (tr, date) => [
            {text: tr('minutes'), style: 'Title'},
            {table: [[strong(tr('insert_date')), date], [strong(tr('minutes_place')), '…'], [strong(tr('minutes_chair')), '…'], [strong(tr('minutes_keeper')), '…'],
                [strong(tr('participants')), '…']], widths: [150, 450]},
            {text: tr('agenda'), style: 'Heading1'},
            {list: [tr('minutes_topic'), tr('minutes_topic')], ordered: true},
            {text: tr('decisions'), style: 'Heading1'},
            {list: ['…']},
            {text: tr('next_steps'), style: 'Heading1'},
            {table: [[tr('task'), tr('owner'), tr('due')], ['…', '…', '…'], ['…', '…', '…']], widths: [300, 150, 150], head: true}],
        report: (tr, date) => [
            {text: tr('report'), style: 'Title'},
            {text: tr('report_author') + ' · ' + date, style: 'Subtitle'},
            {text: tr('summary'), style: 'Heading1'},
            {text: tr('template_body')},
            {text: tr('findings'), style: 'Heading1'},
            {text: tr('template_body')},
            {text: tr('next_steps'), style: 'Heading1'},
            {list: [tr('template_task')]}]
    };
    // options: {date: today's date as text, inches: true where lengths are inches; such a user gets Letter, not A4}.
    function recipe(id, tr, options = {}) {
        const make = RECIPES[id];
        if (!make) return null;
        const paper = window.WriterLayout.papers.find(candidate => candidate.id === (options.inches ? 'letter' : 'a4'));
        return {id, paper: {width: paper.width, height: paper.height}, margins: MARGINS.slice(), blocks: make(tr, options.date || ''), footer: true};
    }
    const escape = value => String(value ?? '').replace(/[&<>"']/g, c => ({'&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;'}[c]));
    // The engine's HTML import keeps alignment, font size, bold and the space before and after a paragraph.
    function paragraph(block) {
        const css = [];
        if (block.align) css.push('text-align:' + block.align);
        if (block.size) css.push('font-size:' + block.size + 'pt');
        if (block.before !== undefined) css.push('margin-top:' + block.before + 'pt');
        if (block.after !== undefined) css.push('margin-bottom:' + block.after + 'pt');
        const text = escape(block.text);
        return '<p' + (css.length ? ' style="' + css.join(';') + '"' : '') + '>' + (block.bold ? '<strong>' + text + '</strong>' : text) + '</p>';
    }
    // In a table the import reads the column width from the width attribute only, the borders from the cell style,
    // and an alignment only from a paragraph inside the cell.
    function table(block) {
        return '<table border="1" style="border-collapse:collapse">' + block.table.map((row, r) => '<tr>' + row.map((cell, c) => {
            const item = typeof cell === 'string' ? {text: cell} : cell;
            return '<td width="' + block.widths[c] + '" style="border:1px solid #64748b">' + paragraph({text: item.text, before: 3, after: 3,
                bold: item.bold || (!!block.head && r === 0), align: (block.right || []).includes(c) ? 'right' : undefined}) + '</td>';
        }).join('') + '</tr>').join('') + '</table>';
    }
    function list(block) {
        const tag = block.ordered ? 'ol' : 'ul';
        return '<' + tag + '>' + block.list.map(text => '<li>' + escape(text) + '</li>').join('') + '</' + tag + '>';
    }
    const html = blocks => blocks.map(block => block.table ? table(block) : block.list ? list(block) : paragraph(block)).join('');
    // The paragraphs that carry a paragraph style, in document order: [[text, style id]]. The HTML import knows no
    // styles, so build() sets them afterwards.
    const styled = blocks => blocks.filter(block => block.style).map(block => [block.text, block.style]);
    // The file name of a template the user names: {name} with .docx, or {error}. The same name must be valid on
    // every system the workspace may live on.
    function fileName(value) {
        const base = String(value ?? '').trim().replace(/\.docx$/i, '').trim();
        if (!base || base.length > 80 || /[\\/:*?"<>|\x00-\x1f]/.test(base) || /^\.|[. ]$/.test(base) || /^(con|prn|aux|nul|com[1-9]|lpt[1-9])$/i.test(base))
            return {error: 'template_name_invalid'};
        return {name: base + '.docx'};
    }
    // The user's templates out of a folder listing of the workspace: [{path, title}], in the listing's order.
    const own = files => (Array.isArray(files) ? files : [])
        .filter(file => file && file.type === 'file' && /\.docx$/i.test(file.name || '') && !/^(\.|~\$)/.test(file.name))
        .map(file => ({path: file.path, title: file.name.replace(/\.docx$/i, '')}));
    // What a document is called without its folder and extension; nothing for a document that has no name yet.
    function title(path) {
        const base = String(path ?? '').split('/').pop().replace(/\.docx$/i, '');
        return /^untitled-[0-9a-f]{8}$/.test(base) ? '' : base;
    }
    // Builds a recipe in a scratch editor inside mount (an attached, empty element) and answers the DOCX bytes.
    async function build(plan, lib, mount, fonts) {
        const scratch = lib.createDocxEditor({container: mount, document: 'blank', fonts, modules: [lib.reviewModule]});
        try {
            if (scratch.snapshot().isOpening) await scratch.save();
            const run = command => {
                const result = scratch.exec(command);
                if (!result.ok) throw new Error('template ' + plan.id + ': ' + command.type + ' ' + (result.reason || ''));
            };
            const [top, right, bottom, left] = plan.margins;
            run({type: 'setPageSetup', pageWidth: plan.paper.width, pageHeight: plan.paper.height, marginTop: top, marginRight: right, marginBottom: bottom, marginLeft: left});
            run({type: 'paste', text: '', html: html(plan.blocks)});
            const paragraphs = scratch.query({type: 'paragraphs'}), paragraphIds = scratch.surface.session.paragraphIds();
            let cursor = 0;
            for (const [text, style] of styled(plan.blocks)) {
                const index = paragraphs.findIndex((candidate, at) => at >= cursor && candidate.text === text);
                if (index < 0) throw new Error('template ' + plan.id + ': no paragraph "' + text + '"');
                cursor = index + 1;
                const start = {paragraphId: paragraphIds[index], offset: 0};
                run({type: 'setSelection', range: {anchor: start, head: start}});
                run(lib.commandForSlotValue('styles.style', style));
            }
            // "1 / 3", centred and small. Page fields exist only inside a header or footer.
            if (plan.footer) for (const command of [{type: 'editHeaderFooter', position: 'footer'}, {type: 'setAlignment', align: 'center'},
                lib.commandForSlotValue('font.size', 18), {type: 'insertPageField', field: 'PAGE'}, {type: 'paste', text: ' / '},
                {type: 'insertPageField', field: 'NUMPAGES'}, {type: 'exitHeaderFooter'}]) run(command);
            return await scratch.save();
        } finally { scratch.destroy(); }
    }
    window.WriterTemplates = {ids, folder, label, recipe, html, styled, fileName, own, title, build};
})();
```

This file was built and looked at in the browser on 2026-11-14 (all four templates, A4). Type it as it stands; the widths, spacings and the order of the footer commands are measured, not guessed.

- [ ] **Step 4: Run the test to see it pass, add it to the suite**

Run: `node scripts/test-writer-templates.mjs`
Expected: `Writer templates: recipes, import HTML, styles, file names and the own list pass`

In `package.json` replace

```json
 && node scripts/test-writer-layout.mjs",
```
with
```json
 && node scripts/test-writer-layout.mjs && node scripts/test-writer-templates.mjs",
```

Run: `npm run test:writer`
Expected: eight lines that end in `pass`. The commands line counts more language keys than before, because it now reads the `tr('…')` calls of the new file.

- [ ] **Step 5: Commit**

```
git add ui/js/desktop/apps/writer-templates.js scripts/test-writer-templates.mjs package.json
git diff --cached --check
git commit
```

Message:

```
feat(writer): templates as recipes, built into DOCX bytes

writer-templates.js describes letter, invoice, minutes and report as
paper, margins, blocks and a footer. build() pastes a recipe as HTML
into a scratch editor, sets paragraph styles and the footer's page
fields by command and answers the DOCX bytes. The module also holds the
rules for the user's own templates: the folder, a valid file name, and
which files of a folder listing are templates.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: Claude Fable 5.2 <noreply@anthropic.com>
```

---

### Task 3: Print every page in its own format

**Files:**
- Modify: `ui/js/desktop/apps/writer-layout.js`, `scripts/test-writer-layout.mjs`
- Modify: `ui/js/desktop/core/print-runtime.js`, `ui/js/desktop/bundles/main.bundle.js` (rebuilt)
- Modify: `ui/js/desktop/apps/writer-panels.js`

- [ ] **Step 1: Extend the layout test**

In `scripts/test-writer-layout.mjs` replace

```js
console.log('Writer layout: units, papers, page setup and paragraph models pass');
```
with
```js
// Printing: one named page per page size, the first size as the default page.
same(L.sheets(['794px 1123px', '794px 1123px', '1123px 794px', '794px 1123px']), {
    css: '@page{size:794px 1123px;margin:0}@page writer-sheet-0{size:794px 1123px;margin:0}@page writer-sheet-1{size:1123px 794px;margin:0}',
    names: ['writer-sheet-0', 'writer-sheet-0', 'writer-sheet-1', 'writer-sheet-0']});
same(L.sheets([]), {css: '', names: []});
console.log('Writer layout: units, papers, page setup, paragraph models and print sheets pass');
```

Run: `node scripts/test-writer-layout.mjs`
Expected: FAIL with `L.sheets is not a function`.

- [ ] **Step 2: `writer-layout.js`**

Replace

```js
    window.WriterLayout = {units, papers: PAPERS, margins: MARGINS, lines: LINES, paperOf, pageFields, pageInput, pageCommand, paragraphFields, paragraphInput, paragraphCommand};
```
with
```js
    // Printing: one named page per page size, so every section prints in its own format. sizes holds one CSS
    // 'width height' per printed page. The first size is also the default page, for a browser without named pages.
    function sheets(sizes) {
        const names = new Map();
        for (const size of sizes) if (!names.has(size)) names.set(size, 'writer-sheet-' + names.size);
        const css = (sizes.length ? '@page{size:' + sizes[0] + ';margin:0}' : '') + [...names].map(([size, name]) => '@page ' + name + '{size:' + size + ';margin:0}').join('');
        return {css, names: sizes.map(size => names.get(size))};
    }
    window.WriterLayout = {units, papers: PAPERS, margins: MARGINS, lines: LINES, paperOf, pageFields, pageInput, pageCommand, paragraphFields, paragraphInput, paragraphCommand, sheets};
```

Run: `node scripts/test-writer-layout.mjs`
Expected: `Writer layout: units, papers, page setup, paragraph models and print sheets pass`

- [ ] **Step 3: `print-runtime.js` names the job**

Replace

```js
    async function create({ html, title = '', className = 'vd-print-frame', failureMessage = 'desktop.print_failed', signal } = {}) {
```
with
```js
    // documentTitle names the printed document. A browser proposes the title of the printing page as the file
    // name of "Save as PDF"; which page counts differs, so the frame's document carries it, and the page itself
    // does while the job is open.
    async function create({ html, title = '', documentTitle = '', className = 'vd-print-frame', failureMessage = 'desktop.print_failed', signal } = {}) {
```

Replace

```js
        let timer = 0, disposed = false;
        let rejectLoad;
        const dispose = () => {
            if (disposed) return;
            disposed = true;
```
with
```js
        let timer = 0, disposed = false, pageTitle = null;
        let rejectLoad;
        const dispose = () => {
            if (disposed) return;
            disposed = true;
            if (pageTitle !== null) document.title = pageTitle;
```

Replace

```js
                frame.contentWindow.focus();
                frame.contentWindow.print();
```
with
```js
                if (documentTitle) { doc.title = documentTitle; pageTitle = document.title; document.title = documentTitle; }
                frame.contentWindow.focus();
                frame.contentWindow.print();
```

Run: `npm run build:ui`
Expected: `git diff --stat -- ui/js/desktop/bundles` shows only `main.bundle.js`, 7 insertions and 2 deletions, the same as `print-runtime.js`. If the bundle shows more, stop and report.

- [ ] **Step 4: `writer-panels.js` prints a page per size**

Replace

```js
        async function print(bytes,lib,fonts) {
            // A separate snapshot keeps edits and review state out of the print transaction.
            printFrame?.dispose();
            const job=await window.AuraDesktopPrint.create({html:'<!doctype html><html><head><link rel="stylesheet" href="/js/vendor/writer/engine.css"></head><body><div class="docx-editor" id="pages"></div></body></html>',title:tr('print'),className:'writer-print-frame',signal:life.signal});
```
with
```js
        // title names the document, so that the print dialog can propose it as the file name of a PDF.
        async function print(bytes,lib,fonts,title) {
            // A separate snapshot keeps edits and review state out of the print transaction.
            printFrame?.dispose();
            const job=await window.AuraDesktopPrint.create({html:'<!doctype html><html><head><link rel="stylesheet" href="/js/vendor/writer/engine.css"></head><body><div class="docx-editor" id="pages"></div></body></html>',title:tr('print'),documentTitle:title || '',className:'writer-print-frame',signal:life.signal});
```

Replace

```js
                const setup=printEditor.snapshot().pageSetup;
                const style=doc.createElement('style');
                style.textContent='@page{size:'+setup.pageWidthTwips/20+'pt '+setup.pageHeightTwips/20+'pt;margin:0}html,body,#pages{
```
with
```js
                // Every painted page carries its own size. A named page per size lets a landscape section inside a
                // portrait document print as landscape; the section at the caret no longer decides for all pages.
                const setup=printEditor.snapshot().pageSetup,fallback=setup.pageWidthTwips/20+'pt '+setup.pageHeightTwips/20+'pt';
                const sheetPages=[...doc.querySelectorAll('#pages .docx-page')];
                const sheets=L.sheets(sheetPages.map(page=>page.style.width && page.style.height?page.style.width+' '+page.style.height:fallback));
                sheetPages.forEach((page,index)=>page.style.setProperty('page',sheets.names[index]));
                const style=doc.createElement('style');
                style.textContent=sheets.css+'html,body,#pages{
```

The rest of that line (from `margin:0!important;` on) stays as it is.

- [ ] **Step 5: Check and commit**

Run: `node --check ui/js/desktop/apps/writer-panels.js`, `node --check ui/js/desktop/core/print-runtime.js`, `npm run test:writer`, `npm run check:ui`, `go test ./ui -run "(?i)(print|office|writer)" -count=1`.
Expected: all pass.

```
git add ui/js/desktop/apps/writer-layout.js scripts/test-writer-layout.mjs ui/js/desktop/core/print-runtime.js ui/js/desktop/bundles/main.bundle.js ui/js/desktop/apps/writer-panels.js
git diff --cached --check
git commit
```

Message:

```
fix(writer): every printed page keeps the format of its section

The print sheet had one @page size, taken from the section at the
caret, so a landscape section inside a portrait document printed as
portrait. Each painted page now gets a named page of its own size.
The shared print runtime can name a job: the name goes on the frame's
document and, while the job is open, on the page, so that the print
dialog can propose it for a PDF.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: Claude Fable 5.2 <noreply@anthropic.com>
```

---

### Task 4: Wiring

**Files:**
- Modify: `scripts/test-writer-menus.mjs`, `scripts/test-writer-commands.mjs`
- Modify: `ui/js/desktop/apps/writer-commands.js`, `writer-menus.js`, `writer-dialogs.js`, `writer-panels.js`, `writer.js`
- Modify: `ui/js/desktop/core/module-loader.js`
- Modify: `ui/desktop_office_assets_test.go`, `ui/desktop_writer_browser_test.go`, `ui/desktop_writer_reload_browser_test.go`

- [ ] **Step 1: Extend the node tests**

In `scripts/test-writer-menus.mjs` replace

```js
assert.equal(menus[0].items.map(item => item.id || '-').slice(0, 5).join(), 'new,open,recent,drafts,-', 'File starts with New, Open, the recent documents and the drafts');
assert.equal(menus[0].items[2].items[0].id, 'recent-0', 'the recent documents are a submenu');
```
with
```js
assert.equal(menus[0].items.map(item => item.id || '-').slice(0, 6).join(), 'new,newFromTemplate,open,recent,drafts,-', 'File starts with New, New from template, Open, the recent documents and the drafts');
assert.equal(menus[0].items[3].items[0].id, 'recent-0', 'the recent documents are a submenu');
const fileIds = menus[0].items.map(item => item.id || '-').join();
assert.ok(fileIds.includes('save,saveAs,saveTemplate,-'), 'Save as template follows Save as');
assert.ok(fileIds.includes('docx,pdf,html,md,txt'), 'the PDF stands with the other exports');
```

In `scripts/test-writer-commands.mjs` replace

```js
    for (const id of ['save', 'bold', 'copy', 'paste', 'goToPage', 'wordCount', 'zoom100', 'table', 'link']) assert.equal(commands.state(id).enabled, false, id);
    for (const id of ['open', 'shortcuts', 'outline']) assert.equal(commands.state(id).enabled, true, id);
```
with
```js
    for (const id of ['save', 'bold', 'copy', 'paste', 'goToPage', 'wordCount', 'zoom100', 'table', 'link', 'saveTemplate', 'pdf']) assert.equal(commands.state(id).enabled, false, id);
    for (const id of ['open', 'shortcuts', 'outline', 'newFromTemplate']) assert.equal(commands.state(id).enabled, true, id);
```

and replace

```js
    for (const id of ['new', 'save', 'saveAs', 'image', 'qr', 'paste', 'pastePlain', 'table', 'link', 'tracking']) assert.equal(commands.state(id).enabled, false, id);
    for (const id of ['open', 'print', 'docx', 'search', 'goToPage', 'wordCount', 'zoomPage', 'shortcuts']) assert.equal(commands.state(id).enabled, true, id);
```
with
```js
    for (const id of ['new', 'newFromTemplate', 'save', 'saveAs', 'saveTemplate', 'image', 'qr', 'paste', 'pastePlain', 'table', 'link', 'tracking']) assert.equal(commands.state(id).enabled, false, id);
    for (const id of ['open', 'print', 'pdf', 'docx', 'search', 'goToPage', 'wordCount', 'zoomPage', 'shortcuts']) assert.equal(commands.state(id).enabled, true, id);
```

Run: `node scripts/test-writer-menus.mjs` and `node scripts/test-writer-commands.mjs`
Expected: both FAIL (the menu starts with `new,open,recent,…`; `saveTemplate` has no definition).

- [ ] **Step 2: The registry**

In `ui/js/desktop/apps/writer-commands.js` replace

```js
        {id: 'new', label: 'new', icon: 'file-plus', shortcut: 'Ctrl+N', keys: ['n'], write: true},
        {id: 'open', label: 'open', icon: 'folder-open', shortcut: 'Ctrl+O', keys: ['o']},
```
with
```js
        {id: 'new', label: 'new', icon: 'file-plus', shortcut: 'Ctrl+N', keys: ['n'], write: true},
        {id: 'newFromTemplate', label: 'new_from_template', icon: 'file-plus', write: true},
        {id: 'open', label: 'open', icon: 'folder-open', shortcut: 'Ctrl+O', keys: ['o']},
```

and replace

```js
        {id: 'saveAs', label: 'save_as', icon: 'copy', shortcut: 'Ctrl+Shift+S', keys: ['shift+s'], write: true, needsEditor: true},
        {id: 'docx', label: 'download_docx', icon: 'file-text', needsEditor: true},
```
with
```js
        {id: 'saveAs', label: 'save_as', icon: 'copy', shortcut: 'Ctrl+Shift+S', keys: ['shift+s'], write: true, needsEditor: true},
        {id: 'saveTemplate', label: 'save_template', write: true, needsEditor: true},
        {id: 'docx', label: 'download_docx', icon: 'file-text', needsEditor: true},
        {id: 'pdf', label: 'pdf', icon: 'file-text', needsEditor: true},
```

- [ ] **Step 3: The File menu**

In `ui/js/desktop/apps/writer-menus.js` replace

```js
            {id:'file',labelKey:'desktop.menu_file',items:[item('new'),item('open'),{id:'recent',label:tr('recent'),items:extra.recent},item('drafts'),SEP,
                item('save'),item('saveAs'),SEP,...extra.file,SEP,
                item('docx'),item('html'),item('md'),item('txt'),SEP,item('pageSetup'),item('print')]},
```
with
```js
            {id:'file',labelKey:'desktop.menu_file',items:[item('new'),item('newFromTemplate'),item('open'),{id:'recent',label:tr('recent'),items:extra.recent},item('drafts'),SEP,
                item('save'),item('saveAs'),item('saveTemplate'),SEP,...extra.file,SEP,
                item('docx'),item('pdf'),item('html'),item('md'),item('txt'),SEP,item('pageSetup'),item('print')]},
```

Run: `node scripts/test-writer-menus.mjs` and `node scripts/test-writer-commands.mjs`
Expected: both pass.

- [ ] **Step 4: The two dialogs**

In `ui/js/desktop/apps/writer-dialogs.js` replace

```js
        return {table, link, goToPage, wordCount, shortcuts, comment, drafts, page, paragraph};
```
with
```js
        // New from template: the built-in recipes first, then the user's own DOCX files from the templates folder.
        async function templates() {
            const T = window.WriterTemplates, own = await app.ownTemplates();
            const options = T.ids.map(id => ({value: 'builtin:' + id, label: T.label(id, tr)}))
                .concat(own.map(entry => ({value: 'own:' + entry.path, label: entry.title + ' · ' + tr('template_own')})));
            const answer = await ask(tr('new_from_template'), [
                {id: 'template', type: 'select', label: tr('template_pick'), size: Math.min(10, Math.max(4, options.length)), value: options[0].value, options},
                {id: 'folder', type: 'static', label: tr('template_folder'), value: T.folder}], {submitLabel: tr('template_use')});
            if (answer) await app.newFromTemplate(answer.template);
        }
        // Save as template asks for a name only. The folder is fixed, so the list above finds the file again.
        async function saveTemplate() {
            const editor = app.editor, T = window.WriterTemplates;
            if (!editor) return;
            const answer = await ask(tr('save_template'), [{id: 'name', label: tr('template_name'), value: T.title(app.path), required: true, maxlength: 80},
                {id: 'folder', type: 'static', label: tr('template_folder'), value: T.folder}],
                {submitLabel: tr('save'), validate: values => T.fileName(values.name).error ? tr('template_name_invalid') : ''});
            if (answer && app.editor === editor) await app.saveTemplate(T.fileName(answer.name).name);
        }
        return {table, link, goToPage, wordCount, shortcuts, comment, drafts, page, paragraph, templates, saveTemplate};
```

- [ ] **Step 5: The Insert panel**

In `ui/js/desktop/apps/writer-panels.js` replace

```js
                    section('templates','<div class="writer-template-grid">'+['blank','letter','report','meeting'].map(x=>'<button data-action="'+x+'"><span class="writer-template-paper" aria-hidden="true"><i></i><i></i><i></i></span>'+esc(tr(x))+'</button>').join('')+'</div>');
```
with
```js
                    section('templates','<div class="writer-template-grid">'+['blank'].concat(window.WriterTemplates.ids).map(x=>'<button data-action="'+x+'"><span class="writer-template-paper" aria-hidden="true"><i></i><i></i><i></i></span>'+esc(x==='blank'?tr('blank'):window.WriterTemplates.label(x,tr))+'</button>').join('')+'</div>'+more('newFromTemplate'));
```

Delete the function `template(kind)` from the same file: the six lines

```js
        function template(kind) {
            const date=new Date().toLocaleDateString(document.documentElement.lang || 'en');
            if(kind==='letter')return '<p>'+esc(tr('template_sender'))+'</p><p>'+esc(tr('template_recipient'))+'</p><p>'+esc(date)+'</p><h1>'+esc(tr('template_subject'))+'</h1><p>'+esc(tr('template_greeting'))+'</p><p>'+esc(tr('template_body'))+'</p><p>'+esc(tr('template_closing'))+'</p>';
            if(kind==='report')return '<h1>'+esc(tr('report'))+'</h1><p>'+esc(date)+'</p><h2>'+esc(tr('summary'))+'</h2><p>'+esc(tr('template_body'))+'</p><h2>'+esc(tr('findings'))+'</h2><p>'+esc(tr('template_body'))+'</p><h2>'+esc(tr('next_steps'))+'</h2><ul><li>'+esc(tr('template_task'))+'</li></ul>';
            return '<h1>'+esc(tr('meeting'))+'</h1><p>'+esc(date)+'</p><h2>'+esc(tr('participants'))+'</h2><p>…</p><h2>'+esc(tr('agenda'))+'</h2><ol><li>'+esc(tr('template_subject'))+'</li></ol><h2>'+esc(tr('decisions'))+'</h2><p>…</p><h2>'+esc(tr('next_steps'))+'</h2><table><tr><td>'+esc(tr('task'))+'</td><td>'+esc(tr('owner'))+'</td><td>'+esc(tr('due'))+'</td></tr><tr><td>…</td><td>…</td><td>…</td></tr></table>';
        }
```

and replace

```js
        return {show,update,template,print,reset,dispose(){life.abort();reset();printFrame?.dispose();}};
```
with
```js
        return {show,update,print,reset,dispose(){life.abort();reset();printFrame?.dispose();}};
```

- [ ] **Step 6: `writer.js`**

The state facade gains the document's path and the three template functions. Replace

```js
            windowId,signal:()=>documentLife.signal,countWords:text=>countWords(text),menuSignature,fileMenuItems,recentMenuItems,restoreDraft,
```
with
```js
            windowId,signal:()=>documentLife.signal,countWords:text=>countWords(text),menuSignature,fileMenuItems,recentMenuItems,restoreDraft,
            get path(){return path;},ownTemplates,newFromTemplate,saveTemplate,
```

`load` takes a template. Replace

```js
        // take: put the kept draft in without asking, and start a new document when the file is gone.
        async function load(target,template,take) {
```
with
```js
        // template: 'blank', the id of a built-in template, or {bytes} of one of the user's own. The document is new.
        // take: put the kept draft in without asking, and start a new document when the file is gone.
        async function load(target,template,take) {
```

Replace

```js
                let bytes='blank',imported=null,loadedETag=null,loadedPath=target;
                if(!template) {
```
with
```js
                let bytes='blank',imported=null,loadedETag=null,loadedPath=target;
                // A template hands over finished DOCX bytes: the user's own file as it is, a built-in one built in a
                // scratch editor under the loading layer. So the new document has no undo steps of its own.
                const started=!!template && template!=='blank';
                if(started) {
                    const T=window.WriterTemplates;
                    bytes=template.bytes || await T.build(T.recipe(template,tr,{date:new Date().toLocaleDateString(locale,{dateStyle:'long'}),inches:state.units.unit==='in'}),lib,mount,fonts);
                    if(disposed || token!==generation)return;
                } else if(!template) {
```

Replace

```js
                if(template || imported) {
                    if(!ctx.readonly) {
                        run({type:'setPageSetup',pageWidth:11906,pageHeight:16838,marginTop:1417,marginBottom:1417,marginLeft:1417,marginRight:1417});
                        const html=imported?sanitize(imported.html || ''):template!=='blank'?panels.template(template):'';
                        if(imported || html){run({type:'paste',text:imported?.text || '',html});restoreHTMLHeadings(editor,html);}
                    }
                    if(imported)notice(tr('import_copy'));
                }
```
with
```js
                if(!started && (template || imported)) {
                    if(!ctx.readonly) {
                        run({type:'setPageSetup',pageWidth:11906,pageHeight:16838,marginTop:1417,marginBottom:1417,marginLeft:1417,marginRight:1417});
                        if(imported){const html=sanitize(imported.html || '');run({type:'paste',text:imported.text || '',html});restoreHTMLHeadings(editor,html);}
                    }
                    if(imported)notice(tr('import_copy'));
                }
```

A document from a template is not a file until it is changed, so closing an untouched one leaves nothing behind and it does not enter the recent list. Its state still reads "unsaved". Replace

```js
                find('[data-save-state]').textContent=tr(queue.dirty?'unsaved':'saved');
```
with
```js
                // An untouched document from a template is not written; its first change or Save makes it a file.
                find('[data-save-state]').textContent=tr(queue.dirty || started?'unsaved':'saved');
```

The actions. Replace

```js
                    case 'blank':case 'letter':case 'report':case 'meeting':if(!ctx.readonly && await guard())await load(newPath(),action);break;
```
with
```js
                    case 'blank':case 'letter':case 'invoice':case 'minutes':case 'report':if(!ctx.readonly && await guard())await load(newPath(),action);break;
                    case 'newFromTemplate':await dialogs.templates();break;
                    case 'saveTemplate':await dialogs.saveTemplate();break;
```

Replace

```js
                    case 'print':if(editor)await panels.print(await editor.save(),lib,documentFonts);break;
```
with
```js
                    case 'print':if(editor)await panels.print(await editor.save(),lib,documentFonts,basename(path).replace(/\.docx$/i,''));break;
                    // The browser's print dialog writes the PDF; the notice says which destination to pick there.
                    case 'pdf':if(editor){notice(tr('pdf_hint'));await panels.print(await editor.save(),lib,documentFonts,basename(path).replace(/\.docx$/i,''));}break;
```

Replace

```js
            } catch(error){if(action==='print')notice(ctx.t('desktop.print_failed'),true);else fail(error);}
```
with
```js
            } catch(error){if(action==='print' || action==='pdf')notice(ctx.t('desktop.print_failed'),true);else fail(error);}
```

The three template functions. Replace

```js
        function fileMenuItems() {
```
with
```js
        // The user's own templates are the DOCX files of one workspace folder. The server refuses to list a folder
        // that does not exist yet, which means there are none.
        async function ownTemplates() {
            try {
                const response=await fetch('/api/desktop/files?path='+encodeURIComponent(window.WriterTemplates.folder),{signal:lifecycle.signal,cache:'no-store'});
                if(!response.ok){await response.body?.cancel().catch(()=>{});return [];}
                return window.WriterTemplates.own((await response.json()).files);
            } catch(_){return [];}
        }
        // choice: 'builtin:<id>' or 'own:<path>'. The new document gets a path of its own, so saving it never
        // writes the template.
        async function newFromTemplate(choice) {
            if(ctx.readonly)return;
            const at=String(choice).indexOf(':'),kind=String(choice).slice(0,at),value=String(choice).slice(at+1);
            try {
                let template=value;
                if(kind==='own') {
                    const response=await fetch(nativeURL(value),{signal:lifecycle.signal,cache:'no-store'});
                    if(!response.ok){await response.body?.cancel().catch(()=>{});notice(tr('template_failed'),true,'info');return;}
                    template={bytes:await response.arrayBuffer()};
                }
                if(await guard())await load(newPath(),template);
            } catch(error){if(error?.name!=='AbortError' && !disposed)notice(tr('template_failed'),true,'info');}
        }
        // Writes the open document into the templates folder; the server creates the folder with the first file.
        // The document itself keeps its path.
        async function saveTemplate(name) {
            if(!editor || ctx.readonly || fileOperation)return;
            const source=editor,target=window.WriterTemplates.folder+'/'+name;
            try {
                const probe=await fetch(nativeURL(target),{signal:documentLife.signal,cache:'no-store'});
                let expected=null;
                if(probe.ok) {
                    expected=probe.headers.get('ETag');
                    await probe.body?.cancel().catch(()=>{});
                    if(!expected){const error=new Error(tr('conflict'));error.status=412;throw error;}
                    if(!await ctx.confirmDialog(ctx.t?.('desktop.file_dialog_overwrite_title') || tr('save_template'),ctx.t?.('desktop.file_dialog_overwrite_message',{name}) || name))return;
                } else if(probe.status===404)await probe.body?.cancel().catch(()=>{});
                else await responseOK(probe);
                if(disposed || editor!==source)return;
                await write(await editor.save(),target,expected);
                notice(tr('template_saved',{name:name.replace(/\.docx$/i,'')}));ctx.loadBootstrap?.();
            } catch(error) {
                if(error?.name!=='AbortError' && !disposed)notice(error.status===412?tr('conflict'):(error.message || String(error)),true,'info');
            }
        }
        function fileMenuItems() {
```

- [ ] **Step 7: Register the new script**

`writer-templates.js` loads directly after `writer-layout.js` everywhere.

In `ui/js/desktop/core/module-loader.js` replace

```js
'/js/desktop/apps/writer-layout.js', '/js/desktop/apps/writer-commands.js',
```
with
```js
'/js/desktop/apps/writer-layout.js', '/js/desktop/apps/writer-templates.js', '/js/desktop/apps/writer-commands.js',
```

In `ui/desktop_office_assets_test.go` replace

```go
		"/js/desktop/apps/writer-layout.js",
		"/js/desktop/apps/writer-commands.js",
```
with
```go
		"/js/desktop/apps/writer-layout.js",
		"/js/desktop/apps/writer-templates.js",
		"/js/desktop/apps/writer-commands.js",
```

In `ui/desktop_writer_browser_test.go` (two fixtures) and `ui/desktop_writer_reload_browser_test.go` (one fixture), replace every occurrence of

```go
<script src="/js/desktop/apps/writer-layout.js"></script><script src="/js/desktop/apps/writer-commands.js"></script>
```
by

```go
<script src="/js/desktop/apps/writer-layout.js"></script><script src="/js/desktop/apps/writer-templates.js"></script><script src="/js/desktop/apps/writer-commands.js"></script>
```

Three occurrences in total. `module-loader.js` is not part of a bundle, so nothing is rebuilt.

- [ ] **Step 8: Static checks and commit**

Run:

- `node --check` on `writer.js`, `writer-dialogs.js`, `writer-panels.js`, `writer-menus.js`, `writer-commands.js`, `writer-templates.js`
- `npm run test:writer`
- `npm run check:ui`
- `gofmt -l ui/desktop_writer_browser_test.go ui/desktop_writer_reload_browser_test.go ui/desktop_office_assets_test.go` (prints nothing)
- `go test ./ui -run "(?i)(windowmenu|contextmenu|materials|office|writer|virtualdesktop|lifecycle|i18n|feeling|print)" -count=1`

Expected: all pass. `writer.js` has about 750 lines, `writer-panels.js` about 335.

```
git add ui/js/desktop/apps/writer.js ui/js/desktop/apps/writer-commands.js ui/js/desktop/apps/writer-menus.js ui/js/desktop/apps/writer-dialogs.js ui/js/desktop/apps/writer-panels.js ui/js/desktop/core/module-loader.js scripts/test-writer-menus.mjs scripts/test-writer-commands.mjs ui/desktop_office_assets_test.go ui/desktop_writer_browser_test.go ui/desktop_writer_reload_browser_test.go
git diff --cached --check
git commit
```

Message:

```
feat(writer): New from template, Save as template, Save as PDF

File > New from template lists the built-in templates (letter, invoice,
minutes, report) and the user's own DOCX files from Documents/Templates.
A templated document starts from finished bytes under a new untitled
path, with no undo steps, and is not written before its first change.
File > Save as template stores the open document in that folder under a
name. File > Save as PDF opens the print dialog with a hint and the
document's name. The Insert panel shows the built-in templates as tiles.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: Claude Fable 5.2 <noreply@anthropic.com>
```

---

### Task 5: Browser checks

**Files:**
- Create: `ui/desktop_writer_templates_test.go`
- Modify: `ui/desktop_writer_browser_test.go`

- [ ] **Step 1: The templates check**

Create `ui/desktop_writer_templates_test.go`:

```go
package ui

import (
	"testing"

	"github.com/go-rod/rod"
)

// verifyWriterTemplates runs after verifyWriterRecent in the file-ops fixture. The fixture keeps every written
// document in memory, lists a folder under /api/desktop/files and records each request under /test/traces.
func verifyWriterTemplates(t *testing.T, page *rod.Page) {
	t.Helper()
	page.MustEval(`async()=>{
        const app=WriterApp.instances.get('test'),T=WriterTemplates,wait=ms=>new Promise(r=>setTimeout(r,ms));
        const until=async(test,what)=>{for(let i=0;i<160;i++){if(await test())return;await wait(50);}throw Error('timed out: '+what);};
        const labels=await(await fetch('/lang/desktop/en.json')).json(),text=key=>labels['desktop.writer_'+key];
        const shown=()=>!!app.editor&&document.querySelector('[data-loading]').hidden;
        const body=()=>app.editor.surface.session.bodyText();
        const dialog=()=>document.querySelector('dialog.vd-form-dialog');
        const field=id=>dialog()?.querySelector('[data-form-field="'+id+'"]');
        const submit=()=>dialog().querySelector('[type=submit]').click();
        const notice=()=>document.querySelector('[data-notice]');
        const puts=async target=>(await (await fetch('/test/traces')).json()).filter(item=>item.method==='PUT'&&decodeURIComponent(item.url).includes(target)&&item.status===200);
        const u=WriterLayout.units(document.documentElement.lang,navigator.language);
        window.confirmAnswer=true;
        if(app.session.dirty)await app.session.save();
        // The File menu offers the three new entries where they belong.
        const fileIds=window.menus.find(menu=>menu.id==='file').items.map(item=>item.id||'-').join();
        for(const run of ['new,newFromTemplate,open','saveAs,saveTemplate,-','docx,pdf,html'])if(!fileIds.includes(run))throw Error('the File menu reads '+fileIds);
        // Every built-in template builds: its paper and margins, its texts, a footer, no undo steps.
        const paper=u.unit==='in'?[12240,15840]:[11906,16838],marks={letter:'template_subject',invoice:'invoice_intro',minutes:'agenda',report:'summary'};
        for(const id of T.ids) {
            const before=app.path;
            await app.act(id);
            await until(()=>shown()&&app.path!==before,id+' did not open');
            const setup=app.editor.getPageSetup(),margins=setup.marginsTwips;
            if(setup.pageWidthTwips!==paper[0]||setup.pageHeightTwips!==paper[1]||[margins.top,margins.right,margins.bottom,margins.left].join()!=='1417,1134,1134,1417')throw Error(id+' page: '+JSON.stringify(setup));
            if(!text(marks[id])||!body().includes(text(marks[id])))throw Error(id+' lacks "'+text(marks[id])+'": '+body().slice(0,200));
            if(body().includes('undefined')||body().includes('desktop.writer_'))throw Error(id+' shows a raw key: '+body().slice(0,300));
            if(app.editor.snapshot().canUndo)throw Error(id+' starts with undo steps');
            if(![...app.editor.surface.session.currentPackage().partBytes.keys()].some(key=>key.includes('footer')))throw Error(id+' has no footer');
            if(!/untitled-[0-9a-f]{8}\.docx$/.test(app.path))throw Error(id+' opened at '+app.path);
            if((id==='minutes'||id==='report')&&app.editor.getOutline().length!==3)throw Error(id+' outline: '+JSON.stringify(app.editor.getOutline()));
            if(id==='invoice') {
                const at=app.editor.query({type:'paragraphs'}).findIndex(paragraph=>paragraph.text===text('invoice_description'));
                const start={paragraphId:app.editor.surface.session.paragraphIds()[at],offset:0};
                app.editor.exec({type:'setSelection',range:{anchor:start,head:start}});
                const table=app.editor.getSelectedTable();
                if(!table||table.columnCount!==5||table.rowCount!==6)throw Error('the invoice table is '+JSON.stringify(table));
            }
        }
        // An untouched document from a template is not written.
        const untouched=app.path.split('/').pop();
        await wait(1500);
        if(app.session.dirty||(await puts(untouched)).length)throw Error('an untouched template document was written');
        // The Insert panel shows the built-in templates and leads to the list.
        if(!document.querySelector('.writer-right .writer-template-grid'))await app.act('insert');
        await wait(150);
        const tiles=[...document.querySelectorAll('.writer-right .writer-template-grid button')].map(button=>button.dataset.action).join();
        if(tiles!=='blank,letter,invoice,minutes,report')throw Error('the Insert panel offers '+tiles);
        if(!document.querySelector('.writer-right [data-action="newFromTemplate"]'))throw Error('the Insert panel has no way to the template list');
        await app.act('insert');
        // Without own templates the list holds the four built-in ones and names the folder.
        void app.act('newFromTemplate');
        await until(()=>!!field('template'),'the template list did not open');
        if(field('template').options.length!==4||field('template').size!==4||field('template').multiple)throw Error('the list offers '+field('template').options.length+' rows');
        if(!dialog().textContent.includes(T.folder))throw Error('the list does not name the folder');
        dialog().querySelector('[data-form-cancel]').click();await wait(150);
        // Save as template: an invalid name is refused in the dialog, a valid one writes into the folder.
        app.editor.exec({type:'paste',text:'House style marker '});
        const home=app.path,target=T.folder+'/House style.docx';
        void app.act('saveTemplate');
        await until(()=>!!field('name'),'the template dialog did not open');
        if(field('name').value!=='')throw Error('an unnamed document offers the name '+field('name').value);
        field('name').value='a/b';submit();await wait(150);
        if(!dialog()||dialog().querySelector('[data-form-error]').hidden)throw Error('an invalid template name was accepted');
        field('name').value='House style';submit();
        await until(async()=>(await puts(target)).length===1,'the template was not written');
        await until(()=>!notice().hidden&&notice().textContent.includes('House style'),'saving a template says nothing');
        if(dialog())throw Error('the template dialog stayed open');
        if(app.path!==home)throw Error('saving a template moved the document to '+app.path);
        const first=(await puts(target))[0];
        if(first.ifNone!=='*')throw Error('the first template write carried '+JSON.stringify(first));
        // The same name again asks, then replaces the file with its version.
        void app.act('saveTemplate');
        await until(()=>!!field('name'),'the template dialog did not open again');
        field('name').value='House style.docx';submit();
        await until(async()=>(await puts(target)).length===2,'the template was not replaced');
        if((await puts(target))[1].ifMatch!==first.etag)throw Error('the replacement did not name the version it replaces');
        // New from template now lists the own template and opens it as a new document.
        void app.act('newFromTemplate');
        await until(()=>!!field('template'),'the template list did not open again');
        const options=[...field('template').options].map(option=>option.value).join();
        if(options!=='builtin:letter,builtin:invoice,builtin:minutes,builtin:report,own:'+target)throw Error('the list offers '+options);
        if(field('template').size!==5||!field('template').options[4].textContent.startsWith('House style · '))throw Error('the own template is shown as '+field('template').options[4].textContent);
        field('template').value='own:'+target;submit();
        await until(()=>shown()&&app.path!==home&&body().includes('House style marker'),'the own template did not open');
        if(!/untitled-[0-9a-f]{8}\.docx$/.test(app.path))throw Error('a document from a template opened at '+app.path);
        if(app.editor.snapshot().canUndo)throw Error('a document from an own template starts with undo steps');
        await app.act('save');
        if(app.session.dirty)throw Error('the new document was not saved');
        if((await puts(target)).length!==2)throw Error('saving the new document wrote the template');
    }`)
}
```

- [ ] **Step 2: The file-ops fixture lists a folder and runs the check**

In `ui/desktop_writer_browser_test.go` replace

```go
	"os"
	"strings"
	"sync"
```
with
```go
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
```

Replace

```go
	mux.HandleFunc("/writer-file-ops-fixture", func(w http.ResponseWriter, r *http.Request) {
```
with
```go
	mux.HandleFunc("/api/desktop/files", func(w http.ResponseWriter, r *http.Request) {
		folder := r.URL.Query().Get("path") + "/"
		mu.Lock()
		var names []string
		for name := range files {
			if strings.HasPrefix(name, folder) && !strings.Contains(name[len(folder):], "/") {
				names = append(names, name)
			}
		}
		mu.Unlock()
		// Like the server, the fixture refuses a folder that does not exist.
		if len(names) == 0 {
			http.Error(w, `{"error":"list desktop files"}`, http.StatusBadRequest)
			return
		}
		sort.Strings(names)
		list := make([]map[string]string, 0, len(names))
		for _, name := range names {
			list = append(list, map[string]string{"name": name[len(folder):], "path": name, "type": "file"})
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"status": "ok", "files": list})
	})
	mux.HandleFunc("/writer-file-ops-fixture", func(w http.ResponseWriter, r *http.Request) {
```

Replace

```go
	verifyWriterRecent(t, page)
}
```
with
```go
	verifyWriterRecent(t, page)
	verifyWriterTemplates(t, page)
}
```

- [ ] **Step 3: Print with mixed formats, and Save as PDF**

Still in `ui/desktop_writer_browser_test.go`. The print hook also records the names of the job. Replace

```go
                window.printProbe={pages:frame.contentDocument.querySelectorAll('.docx-page').length,text:frame.contentDocument.body.innerText,html:frame.contentDocument.documentElement.outerHTML};window.printFonts=[...frame.contentDocument.fonts];
```
with
```go
                window.printProbe={pages:frame.contentDocument.querySelectorAll('.docx-page').length,text:frame.contentDocument.body.innerText,html:frame.contentDocument.documentElement.outerHTML,title:document.title,frameTitle:frame.contentDocument.title};window.printFonts=[...frame.contentDocument.fonts];
```

Replace

```go
        observer.observe(document.body,{childList:true});
        await WriterApp.instances.get('test').act('print');observer.disconnect();
        if(!window.printProbe || printProbe.pages!==WriterApp.instances.get('test').editor.getTotalPages() || !printProbe.text.includes('First window edit'))throw Error('Print did not capture all current pages: '+JSON.stringify(window.printProbe)+'; notice: '+document.querySelector('[data-notice]').textContent);
```
with
```go
        observer.observe(document.body,{childList:true});
        const app=WriterApp.instances.get('test'),editor=app.editor;
        // A landscape section after the portrait pages: every printed page must keep its own format.
        const last=editor.surface.session.paragraphIds().at(-1),end={paragraphId:last,offset:editor.query({type:'paragraphs'}).at(-1).text.length};
        for(const command of [{type:'setSelection',range:{anchor:end,head:end}},{type:'insertBreak',kind:'section'},{type:'paste',text:'Landscape section'},{type:'setPageSetup',orientation:'landscape',scope:'section'}]) {
            const result=editor.exec(command);if(!result.ok)throw Error(JSON.stringify({command,result}));
        }
        window.titleBeforePrint=document.title;
        const name=app.path.split('/').pop().replace(/\.docx$/,''),labels=await(await fetch('/lang/desktop/en.json')).json();
        // Save as PDF is the same print job with a hint about the destination and the document's name.
        await app.act('pdf');
        if(!window.printProbe)throw Error('Save as PDF printed nothing: '+document.querySelector('[data-notice]').textContent);
        if(document.querySelector('[data-notice-text]').textContent!==labels['desktop.writer_pdf_hint'])throw Error('Save as PDF says '+document.querySelector('[data-notice-text]').textContent);
        if(printProbe.title!==name||printProbe.frameTitle!==name)throw Error('the print job is named '+JSON.stringify({title:printProbe.title,frameTitle:printProbe.frameTitle,name}));
        window.printProbe=null;
        await app.act('print');observer.disconnect();
        if(!window.printProbe || printProbe.pages!==editor.getTotalPages() || printProbe.pages!==3 || !printProbe.text.includes('First window edit') || !printProbe.text.includes('Landscape section'))throw Error('Print did not capture all current pages: '+JSON.stringify({pages:window.printProbe?.pages,total:editor.getTotalPages()})+'; notice: '+document.querySelector('[data-notice]').textContent);
```

Replace

```go
        WriterApp.dispose('test');
        const html=printProbe.html,faces=printFonts;
```
with
```go
        WriterApp.dispose('test');
        if(document.title!==window.titleBeforePrint)throw Error('printing left the page title at '+document.title);
        const html=printProbe.html,faces=printFonts;
```

Replace

```go
	if pages := bytes.Count(pdf.Data, []byte("/Type /Page\n")); pages != 2 {
		t.Fatalf("Printed PDF has %d pages instead of 2", pages)
	}
```
with
```go
	if pages := bytes.Count(pdf.Data, []byte("/Type /Page\n")); pages != 3 {
		t.Fatalf("Printed PDF has %d pages instead of 3", pages)
	}
	// Two portrait pages and the landscape section, each in its own format.
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
	if portrait != 2 || landscape != 1 {
		t.Fatalf("Printed PDF has %d portrait and %d landscape pages instead of 2 and 1", portrait, landscape)
	}
```

- [ ] **Step 4: Compile, then the controller runs the browser tests**

Run: `gofmt -l ui/desktop_writer_templates_test.go ui/desktop_writer_browser_test.go` (prints nothing) and `go vet ./ui`.

The controller then deletes `reports\autor` and runs `AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run "TestDesktop(Writer|QR)" -count=1 -timeout 30m -v`.
Expected: every `TestDesktopWriter*` passes. A failure in `verifyWriterTemplates` or in the print block names the step; fix the cause, not the check. If the document in `TestDesktopWriterAppBrowser` does not have exactly three pages after the section break, report the numbers instead of changing the expectation.

The controller also looks at pictures: both new dialogs with the shell's stylesheets (`/css/desktop-shell.bundle.css`, `/css/desktop-form-dialog.css`) in the dark theme, and the first page of each of the four templates. The list shows its rows without clipping, the static row names the folder, the letter has its letterhead at the right and the invoice table has borders and right-aligned amounts.

- [ ] **Step 5: Commit**

```
git add ui/desktop_writer_templates_test.go ui/desktop_writer_browser_test.go
git diff --cached --check
git commit
```

Message:

```
test(writer): templates, own templates, mixed print formats and Save as PDF in the browser

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: Claude Fable 5.2 <noreply@anthropic.com>
```

---

### Task 6: Documentation

**Files:**
- Modify: `ui/js/desktop/apps/AGENTS.md`, `documentation/autor.md`

The roadmap already records the decisions of 2026-11-14 and names this plan; it needs no change.

- [ ] **Step 1: `ui/js/desktop/apps/AGENTS.md`**

In the Writer section, directly before the line that starts with `` `writer-session.js` owns revision-bound autosave``, add:

```text
  `writer-templates.js` holds the built-in templates and the rules for the user's own. A built-in
  template is a recipe (`recipe(id, tr, {date, inches})`: paper, margins, blocks, footer) and
  `build` turns it into DOCX bytes in a scratch editor inside the empty mount, so
  `load(target, template)` starts every templated document from finished bytes with no undo steps
  (`template` is `'blank'`, a built-in id or `{bytes}`). The recipe goes in as HTML because the
  engine's import is known to keep alignment, font size, bold and paragraph spacing; in a table it
  reads the column width from the `width` attribute only, the borders from the cell style and an
  alignment only from a paragraph inside the cell. Paragraph styles and the footer's page fields
  are set by commands afterwards. The user's own templates are the DOCX files in
  `Documents/Templates` (`folder`): `saveTemplate` in `writer.js` writes there and the server
  creates the folder with the first file; `ownTemplates` lists it and takes the refusal for a
  missing folder as "none". A document from a template gets a new `untitled-` path and is not
  written before its first change. Printing gives every painted page a named `@page` of its own
  size (`WriterLayout.sheets`), so sections of different formats print right;
```

In the same file, in the entry that starts with `- Notes, Writer, Sheets and Viewer share `AuraDesktopPrint` from`, add after the sentence that ends with `only the parent prints.`:

```text
  `documentTitle` names a job: it goes on the frame's document and, while the job is open, on
  the page itself, because browsers take the proposed PDF file name from one of the two.
```

- [ ] **Step 2: `documentation/autor.md`**

Add this section directly before the heading `## Files and recovery`:

```text
## Templates and PDF

**File > New from template…** lists the built-in templates (letter, invoice,
minutes, report) and, below them, your own. The Insert panel shows the built-in
ones as tiles. A built-in template brings its margins (25 mm at the top and the
left, 20 mm at the right and the bottom), its paragraph formats, tables and a
footer with "page / pages". Its texts follow the interface language, and it
uses A4, or Letter where lengths are shown in inches. The letter follows
DIN 5008 with paragraph spacing: letterhead at the right, return line,
recipient, date and subject. It has no positioned address frame, so check the
position before using window envelopes.

**File > Save as template…** stores the open document under a name of your
choice as a DOCX file in `Documents/Templates`; the folder is created with the
first template. An existing name is replaced only after a question. Rename or
delete templates in the Files app. Every DOCX file in that folder is offered as
a template.

A document made from a template is new. It has no undo steps, never writes
back to the template, and becomes a file in `Documents` under an `untitled-`
name with its first change or with Save.

**File > Save as PDF…** opens the browser's print dialog; choose "Save as PDF"
as the destination there. Writer hands the document's name to the print job so
that the dialog can propose it as the file name. **Print…** uses the same path.
Every page prints in the format of its own section, so a landscape section
inside a portrait document stays landscape.
```

In the section `## Build and verification`, in the entry that starts with `- `npm run test:writer` checks`, replace

```text
  groups and overflow rule (`scripts/test-writer-toolbar.mjs`) and the window
  menu model (`scripts/test-writer-menus.mjs`).
```
with
```text
  groups and overflow rule (`scripts/test-writer-toolbar.mjs`), the window
  menu model (`scripts/test-writer-menus.mjs`), the context surfaces
  (`scripts/test-writer-context.mjs`), the layout model and print sheets
  (`scripts/test-writer-layout.mjs`) and the templates
  (`scripts/test-writer-templates.mjs`).
```

- [ ] **Step 3: DOX pass and commit**

Re-read the changed paths against the `AGENTS.md` chain. `ui/AGENTS.md`, `scripts/AGENTS.md` and the root guide need no change; say so in the report.

```
git add ui/js/desktop/apps/AGENTS.md documentation/autor.md
git diff --cached --check
git commit
```

Message:

```
docs(writer): templates, own templates, Save as PDF and print formats

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: Claude Fable 5.2 <noreply@anthropic.com>
```

---

## Final verification (controller)

- `npm run test:writer` and `npm run check:ui` pass.
- Browser: `TestDesktop(Writer|QR)` passes twice in a row (delete `reports\autor` before each run).
- `go test ./ui -count=1 -skip GoldfishAdventureHotspot` fails only in `TestDesktopCodeStudioRunUsesTerminalSession` and `TestDesktopPolishPointerLightTargetsMatch`.
- `git status --short` is empty; `disposable/_writer-wave2b/` does not exist.
- One branch review by the reader-agent (Critical and Important only), one fix round.
- Left for the user's acceptance: the file name Chrome's print dialog proposes for the PDF, and the look of the four templates in the real shell.

## Changes after the branch review (2026-11-14)

Implemented as written: Tasks 1 to 5 by maker-agents, Task 6 by the docs-agent; every file matched the dry run. One fix round followed (902e2350a):

- `fileName` refuses a name that starts with `~$` and a Windows device name with an extension (`nul.bak`). `own()` hides `~$` files, so such a template was saved and never listed.
- A blank document and a built-in template no longer open with the notice about substitute fonts. The bullets of a built-in list name the Symbol font, which showed the notice on minutes and report.
- `verifyWriterTemplates` reads the footer part for both page fields, expects no notice after a built-in template opens, and watches all four untouched documents for writes, not only the last.

Looked at and left: `window.print()` is assumed to block until the dialog closes, as it does in Chrome, so the page title is restored afterwards; two windows printing at the same moment could restore a stale title. `saveTemplate` returns without a notice while another file operation of the window runs.
