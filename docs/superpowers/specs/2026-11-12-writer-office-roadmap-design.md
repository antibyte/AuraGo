# Writer: office-grade roadmap and UI/UX polish — design

Date: 2026-11-12. Status: draft, waiting for the user's review. The user chose the priority ("everyday office first") and the interaction concept ("menu bar + context") on 2026-11-12; the wave split below was presented but not yet confirmed.

Related documents: `documentation/autor.md` (current behaviour), `ui/js/desktop/apps/AGENTS.md` (Writer notes).

## Goal

Writer should feel like a professional word processor for everyday office work: letters, reports, minutes, offers. Two things carry that: the interaction model people expect from Word, LibreOffice Writer and Google Docs (complete menus, right-click menu, real dialogs, a toolbar that never scrolls sideways), and the business-document features that are missing or half exposed today (page setup, headers and footers, PDF, templates, paragraph and table properties).

This is a roadmap. Each wave gets its own implementation plan; Wave 3 items get their own specs.

## Decisions

| Decision | Choice | Source |
| --- | --- | --- |
| Priority | Interaction quality first, then business documents. Long-document and collaboration depth later. | User, 2026-11-12 |
| Interaction concept | Menu bar + context, like Google Docs: complete window menus, one toolbar with an overflow menu, a contextual bar for table / image / header-footer, a right-click menu, form dialogs. No ribbon, no second toolbar row. | User, 2026-11-12 |
| Approach | Foundation first (a command registry that feeds every surface), then feature waves. Bolting features onto today's structure and a UI rewrite were both rejected. | Recommended, not yet confirmed |
| PDF | Through the existing print path (browser "Save as PDF"), after fixing per-section page sizes. A server-side renderer is optional and later. | Recommended, not yet confirmed |
| Spell check | Parked. Not feasible on the current engine. | Evidence below |

## Evidence

Read-only inventory on 2026-11-12. Line numbers are as read that day; verify them in each plan.

### What the UI offers today

- One toolbar row, hand-built as an HTML string, scrolling horizontally when it does not fit (`writer.js:61-68`). Undo/redo, paragraph style select (apply only), font, size, B/I/U/S, alignment, lists, indent, text and highlight colour, an insert button that opens a side panel.
- Window menus File, Edit, Insert, Agent, View (`writer.js:643-663`). No Cut, Copy, Paste, Select all. No Format, Table or Tools menu.
- Side panels: Outline, Search (left); Format, Insert, Review, Assist (right). Most formatting beyond the toolbar lives in the Format panel.
- No right-click menu. Writer calls `ctx.wireContextMenuBoundary(host)` without a callback (`writer.js:107`), so the browser's native menu appears on the document.
- Input through the shared prompt dialog: table size as "3 × 3", hyperlink URL, go to page.
- Present but shallow: page setup (A4 and Letter only, margins), header/footer edit with one field (page X of Y), footnote insert, table of contents insert (no update), hyperlink insert (no edit or remove), table row/column/merge/split/fill/borders, image width/alt/wrap, line spacing as a multiple only, left and first-line indent only, four built-in templates, zoom in 10 % steps.
- Solid and to be kept: save queue with conditional writes and conflict handling, IndexedDB draft recovery, comments and tracked changes, find and replace, the AI assist panel, focus mode.
- Absent: PDF export, spell check, recent documents, superscript/subscript, format painter, formatting marks, styles gallery, section breaks, endnotes, text boxes, shortcut overview, word count dialog.

### Code structure

- `writer.js` is one 712-line `render()` closure with about 70 nested functions. `act()` is one switch mixing panels, file operations, zoom, print and export (`writer.js:585-621`).
- Adding a toolbar button touches the markup, three icon maps, `toggleSlots` and the CSS. Adding a panel touches four places.
- The rule "blocked while suggesting changes" is a regex in `can()` (`writer.js:140`) and is repeated by hand in about eight places in `writer.js` and `writer-panels.js`.
- About 50 command objects are written inline. There is no registry.
- `desktop-app-writer.css` uses raw pixel values throughout and about 20 hard-coded colours.

### What the engine supports

Engine: `@docx-editor.dev/core` 2.29.0 (Apache-2.0), a prebuilt bundle at `ui/js/vendor/writer/engine.js`, built by `scripts/build-writer-vendor.js` from `scripts/writer-engine-entry.js`. Checked against the bundle and the matching type definitions in `node_modules/@docx-editor.dev/core/dist/`.

"Command" means a typed engine command exists. "Model only" means the edit is possible only by writing to the document tree directly, the way `scripts/writer-table-ops.js` already does for merge and split; that depends on engine internals and must be re-checked on every engine upgrade.

| Capability | Verdict |
| --- | --- |
| Superscript, subscript | Command (`toggleMark`) |
| Line spacing exact / at least, right indent | Command (`setLineSpacing`, `setIndent`) |
| Keep with next, keep lines, widow control, page break before, tab stops | Command (`setParagraphFormat`) |
| Section break (next page, continuous) | Command (`insertBreak`) |
| Endnote, convert note | Command (`insertNote`, `convertNote`) |
| Text box | Command (`insertTextBox`) |
| Update table of contents | Command (`refreshToc`) |
| Remove hyperlink; edit = query `hyperlinkAt`, then insert again | Command |
| Different first page, odd/even headers | Command (`setHeaderFooterOptions`, `editHeaderFooter` variants) |
| Page fields PAGE, NUMPAGES, SECTIONPAGES, PAGE_X_OF_Y | Command |
| Custom page size | Command (`setPageSetup`, twips) |
| Insert row above, column left, delete table | Command |
| Table border style, width, colour | Command (`setTableBorders`) |
| Image height, crop, position, rotate, flip, replace | Command |
| Format painter | Command (`copyFormatting`, `pasteFormatting`) |
| Tracked-change display modes, next/previous change | Command (`setReviewDisplayMode`) |
| Formatting marks | Command (`toggleParagraphMarks`) |
| Select all, cut, copy, paste | Command; `paste` needs the app to read the clipboard |
| Repeat header row, table alignment | Declared, probably refused at runtime. Unverified; test before building |
| Small caps, character spacing, paragraph shading, columns, row height, page-number formats (roman, letters), list numbering format | Model only |
| Paragraph borders, create or modify a style, bookmarks, cross-references, date field, watermark | No command |
| Document properties (title, author) | Only in an automation API the bundle does not include |
| Spell check | No. The surface sets `spellcheck=false`, pages are painted DOM, there is no decoration API |
| PDF | No. Upstream needs a separate Node.js package |

The engine also has toolbar slots Writer does not use yet, among them `text.link`, `table.insert`, `list.lineSpacing`, `insert.textBox`, `insert.endnote`, the section-break slots and the `review.*` set.

### What the desktop shell gives Writer

- Window menus support submenus, separators, ticked items, disabled items with a reason, shortcut labels and icons; no radio items. A change means calling `setWindowMenus` again (`core/menus-and-routing.js:1393-1478`).
- Menu shortcuts are skipped while focus is in editable content, except Ctrl+S, F5 and the zoom keys (`menus-and-routing.js:1517-1523`). Writer's own keydown handler (`writer.js:689`) must therefore remain the shortcut source.
- `wireContextMenuBoundary(root, {onContextMenu})` suppresses the browser menu when the callback returns true. `ctx.showContextMenu(x, y, items)` is already passed to Writer and supports submenus, separators, shortcut labels and disabled items.
- Dialogs: only prompt and confirm reach Writer. There is no shared multi-field form dialog; `shared/qr-dialog.js` is the only custom-dialog example.
- A global recent-files list exists in localStorage (`core/session-runtime.js:233-268`). It records only when a window opens with a path; files opened from inside Writer are not recorded, and apps cannot read the list.
- Constraints on splitting files: each file under `ui/js/desktop` stays under 1100 lines and `-part-` names are forbidden (`ui/desktop_js_line_budget_test.go`). New Writer files go into `core/module-loader.js:125`, `desktop_office_assets_test.go:27-31` and the script lists in `desktop_writer_browser_test.go` and `desktop_writer_reload_browser_test.go`. `desktop_office_app_markers_test.go:13-17` requires four strings to stay in `writer.js`. `writer-session.js` is shared with Notes, Sheets, Layerling and Synth Studio and is not changed.

### How the repo makes PDFs

- Headless Chromium through go-rod (`internal/tools/web_capture.go:134`). rod downloads Chromium when none is found, so a browser is not guaranteed on a deployment.
- gofpdf for the Infra atlas, the newspaper and Detective exports. It cannot reproduce the engine's layout.
- The Writer print path (`writer-panels.js:303-322`) already paints the engine's own pages as real text with an exact `@page` size. Saving that as PDF from the browser print dialog matches the screen page for page. One defect: the `@page` size comes from the section at the cursor only, so a document whose sections differ in size or orientation prints wrong.

## Design

### Architecture: one command registry feeds every surface

A new module holds one table of commands. Each entry names its id, i18n label key, icon, shortcut, the engine slot or command it runs, whether it is a toggle, and whether it is blocked while suggesting changes or in read-only mode. It builds on the engine's own slots (`commandForSlot`, `toolbarCommandState`) and adds hand-written entries only where no slot exists (file actions, dialogs, merge and split through `tableCommand`).

Every surface reads from it: window menus, toolbar, overflow menu, right-click menu, contextual bar, the keydown handler and tooltips (label plus shortcut). The suggesting-mode rule lives there once and replaces the regex and its copies.

Modules, in load order:

| File | Responsibility | Depends on |
| --- | --- | --- |
| `writer-session.js` | Save queue and drafts. Unchanged. | — |
| `writer-commands.js` | Registry, `can`/`run` gate, shortcut table | — |
| `shared/form-dialog.js` | Generic multi-field dialog: focus trap, focus return, cancel signal. Shared so Sheets can use it later. | — |
| `writer-dialogs.js` | Table grid picker, hyperlink, go to page, word count, page setup, paragraph | form-dialog, commands |
| `writer-toolbar.js` | Data-driven toolbar, priority overflow, arrow-key navigation, state sync | commands |
| `writer-menus.js` | Window menu model and right-click items | commands |
| `writer-context.js` | Contextual bar for table / image / header-footer; right-click wiring | commands, menus |
| `writer-panels.js` | Side panels. Shrinks as dialogs and the contextual bar take over. | commands |
| `writer.js` | Lifecycle, file operations, wiring. Keeps the strings the marker test requires. | all |

`writer.js` is split gradually, one module per feature, not in one move. Each split updates the module loader and the test script lists in the same change.

### Wave 0 — Foundation

No visible feature; it makes the rest cheap.

| Item | Size | Notes |
| --- | --- | --- |
| Command registry | M | Risk: enabled/active state out of sync with the engine. Covered by a node test. |
| Shared form dialog | M | A native `<dialog>` loaded with Writer's scripts; no shell change. First used for Insert table. |
| Data-driven toolbar with overflow | M | Groups carry a priority; what does not fit moves into a "more" menu. Roving tabindex. Replaces horizontal scrolling. |
| Design tokens | S | Replace the hard-coded chrome colours with theme tokens. One spacing and size scale follows in Wave 1 with the chrome it changes. |

Plan: `docs/superpowers/plans/2026-11-12-writer-wave0-foundation.md`.

### Wave 1 — Interaction

| Item | Size |
| --- | --- |
| Complete menus: File, Edit (cut, copy, paste, paste as plain text, select all, find), View (panels, formatting marks, zoom, focus), Insert, Format (text, paragraph, lists, clear), Table, Tools (word count, review, assist, shortcuts) | M |
| Right-click menu: clipboard, link edit/remove, table and image actions by context, comment, assist | M |
| Contextual bar that appears with the cursor in a table, on an image, or in a header/footer | M |
| Dialogs replacing prompts: table insert with grid picker, hyperlink (insert, edit, remove), go to page, word count | M |
| Toolbar additions from existing engine slots: line spacing, link, table, image, superscript/subscript, format painter | S |
| Small engine commands surfaced in menus: formatting marks, tracked-change display modes, next/previous change, endnote, section breaks, text box | S each |
| Zoom presets (fit width, whole page, fixed steps) and Ctrl+wheel | S |
| Shortcuts: Ctrl+K link, Ctrl+G go to page; hints in tooltips; a shortcut overview | S |
| Start state for an empty window: New, Open, Recent, Templates | M |
| Recent documents: expose the shell list to apps and record files opened from inside Writer | S |

Paste from a menu needs `navigator.clipboard`, which browsers allow only over HTTPS or localhost. When it is unavailable the menu item explains that Ctrl+V works.

Wave 1 is planned in three parts, each shippable on its own (user, 2026-11-12):

- **1a** — registry extension, complete menus, clipboard commands, dialogs, zoom, toolbar additions, shortcuts. Plan: `docs/superpowers/plans/2026-11-12-writer-wave1a-commands-menus.md`.
- **1b** — right-click menu and contextual bar. Planned after 1a has landed.
- **1c** — start state and recent documents. Planned after 1b.

Decided while planning 1a: superscript and subscript are menu items, not toolbar buttons; the format painter is "Copy formatting" and "Paste formatting" in the Format menu, because the engine does not expose the one-click painter's arming; "simple markup" is not offered, because the review module declares three display modes; "whole page" zoom is computed by Writer, because the engine fits the page width only.

### Wave 2 — Business documents

| Item | Size |
| --- | --- |
| Page setup dialog: more paper sizes, custom size, orientation, margin presets | S |
| Headers and footers: different first page, odd/even, page-number position, the four page fields, today's date as text | S |
| PDF export through the print path, with one named page per section | S |
| Paragraph dialog: spacing rules, all indents, keep options, page break before, tab stops | S |
| Table: insert above/left, delete table, border style/width/colour | S |
| Image: height with aspect lock, crop, rotate, replace, position | S |
| Update table of contents | S |
| Styles gallery with previews (apply only) | M |
| Templates as real `.docx` files so they carry page setup, headers and styles: DIN 5008 letter, invoice, minutes; "save as template". CV, offer and report follow. | M |
| Repeat header row and table alignment | S if the engine accepts the commands, M with a model-level workaround. Runtime test first. |
| Server-side PDF (same HTML rendered by rod, saved into the workspace, offered only when a Chromium is present) | M, optional |

Dropped from the first draft: a print preview dialog (the browser print dialog already has preview and page range) and a document-properties dialog (the bundle has no command for it).

### Wave 3 — Depth (own specs)

Feasible on the current engine: interactive ruler (drag margins, indents, tab stops), columns, small caps and character spacing, page-number formats, row height. The last four are model-only workarounds.

Parked until an engine upgrade or a fork, and not to be promised: spell check, creating or modifying styles, numbering-format picker, paragraph borders, bookmarks and cross-references, document compare, watermark, date fields, ODT and RTF.

### Cross-cutting polish

Done inside each wave, not as a separate pass.

- Units: one display unit per locale for lengths (cm, or inches for en-US), points for type; twips stay internal. Today the ruler shows cm and the panels mm.
- Accessibility: arrow keys in the toolbar, roles and focus return for popovers and dialogs, focus moved into a panel when it opens.
- Narrow windows: overflow menu instead of scrolling; the contextual bar collapses into the same menu.
- States: a progress indication for large documents, clear empty states, the invalid-table-size message that currently shows the prompt label (`writer-panels.js:230`).
- i18n: every new string in all 16 languages; remove hard-coded fallbacks such as the "Deutsch" placeholder (`writer-panels.js:67`).
- Tooltips: set once with the shortcut, not rewritten on every refresh.

## Testing

- Node test for the registry (in the style of `scripts/test-writer-session.mjs`): every toolbar, menu and context entry resolves to a registry command; the suggesting-mode and read-only rules hold; shortcuts are unique.
- A test that every `desktop.writer_*` key used in code exists in all 16 language files.
- Browser smoke (`TestDesktopWriter(App|Engine)Browser`) extended per wave for the right-click menu, toolbar overflow at a narrow width, each new dialog, and the PDF print layout with mixed section sizes.
- A runtime probe of the unverified engine commands (header row, table alignment) before the items that depend on them are planned.

## Risks

- Model-level workarounds rely on undocumented engine internals. Keep them in one place next to `writer-table-ops.js` and re-test them on every engine bump.
- Splitting `writer.js` breaks the browser test harnesses unless their script lists change in the same commit.
- The command registry is the single point every surface trusts. A wrong enabled state shows up everywhere at once, which is why it gets its own test first.
- Clipboard access depends on how the instance is served.

## Open questions for the user

1. Is the wave split right, or should something move forward (for example templates into Wave 1)?
2. Is PDF through the browser print dialog enough for now, or is a one-click server-side PDF wanted from the start?
3. Which business templates matter most beyond letter, invoice and minutes?
