# Autor desktop word processor

Autor edits complete DOCX documents using the Apache-2.0
@docx-editor.dev/core 2.29.0 engine. AuraGo's UI and extensions remain MIT.
The editor, fonts and shaping WASM ship locally with the versioned web resources.

## Writing

The document name opens rename and Save As. File menus use the desktop file
dialogs. New documents use A4 portrait paper with 25 mm margins. The toolbar
contains paragraph styles, fonts, numeric sizes, common formatting and lists.
When the window is too narrow for the whole bar, groups move into a More tools
popover instead of scrolling: lists first, then alignment, colours, insert,
text marks and the font controls; undo and redo always stay. The bar is one
tab stop; Left and Right move between its controls. In the size field they move the caret, and Tab steps to the next control.
The bar also holds clear formatting, line spacing, link, table and image.
The window menus cover every command: File, Edit (including cut, copy, paste,
paste without formatting, select all, find and go to page), View (panels,
formatting marks, how tracked changes are shown, zoom, focus mode), Insert
(table, image, QR code, link, page and section breaks, footnote, endnote,
table of contents, text box, header, footer, page numbers, and back to the
document from a header or footer), Format (text, paragraph, line spacing, copy
and paste formatting, clear formatting), Table (insert, rows, columns, merge,
split, delete), Tools (word count, track changes, the review panel, previous and
next change, keyboard shortcuts) and Agent (the AI assistant). Superscript and
subscript are in Format > Text. Ctrl+K opens the link dialog, which inserts,
changes or removes a link; Ctrl+G goes to a page. Paste from the menu needs the
browser's clipboard permission; Ctrl+V always works.
The zoom button in the status bar offers fit to width, whole page and fixed
steps; Ctrl with the mouse wheel zooms as well.
A right-click in the document opens a menu for what was clicked: cut, copy,
paste and paste without formatting, the link under the caret (change or remove
it), a comment on the selection, the table commands inside a table, rotate and
delete for an image, accept or reject for a tracked change under the pointer,
and the AI assistant. A right-click on an image that is not selected selects
it first. A selection stays as it is when you right-click; without a selection
the caret moves to the click first. Shift+F10 opens the same menu from the
keyboard.

While the caret is in a table, an image is selected, or a header or footer is
open, a bar with the matching commands covers the ruler. Only one bar shows at
a time: an image first, then a header or footer, then a table. The table bar
has insert and delete for rows and columns, delete table, merge, split, and
borders on and off; the image bar has rotate left, rotate right, delete and the
Format panel; the header or footer bar has page numbers, the first-page option,
the odd-and-even option and the way back to the document. The document does not
move when the bar appears.
Format provides paragraph spacing, page setup, table and image controls.
Page setup recognizes A4 and Letter in both orientations. Other sizes stay as
they are until a listed size is chosen. Indent, color and image controls follow
the current selection.
Insert provides tables, images, page breaks, footnotes, headers, footers,
page numbers and document templates.
A table is inserted from a dialog with a size grid and fields for rows
(1 to 100) and columns (1 to 30).

Outline and Search appear on the left. Format, Review and AI share the right
panel. Panels overlay the document in narrow windows. Fit-to-width, manual zoom
and focus mode are available in the status bar. Fruity uses the desktop's
active-window menu model.

Table columns resize using the engine's column handles. Merge a rectangular
cell selection; Split divides a cell into two columns and separates vertically
merged rows. Structural edits with unsupported tracking are disabled until
editing mode is restored.

## Review and AI

Review shows anchored comments, replies, resolved threads and tracked changes
with their authors and dates. Individual changes or all supported changes can
be accepted or rejected. Unsupported imported revisions remain in the DOCX.

AI works on selected text or an explicitly selected paragraph. It returns a
proposal without tools, file access or general chat history. Review the original
and proposed text before applying. Application is undoable and respects tracking;
any intervening document edit invalidates the proposal.

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

## Files and recovery

Autosave starts after 800 ms of inactivity, or after five seconds of continuous
editing. Only one save runs per document. Conditional writes prevent another
window's changes from being overwritten. A conflict offers Save As and reopen.
Reopen asks before discarding the local edits. Retry remains for a failed load
and for a transient save of the open file. A successful save clears that
conflict or save notice. Save As asks before replacing another file and then
writes it with that file's ETag. Renaming only creates the new name
(`If-None-Match: *`) and then uses the ETag returned for that file. An existing
name stays unchanged.

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

DOCX opens and saves directly, including package parts the editor cannot edit.
Unknown authored fonts keep their names and receive a named local substitute.
HTML, Markdown and text import into a new DOCX; source files are not overwritten.
Simpler-format exports warn about lost review and layout information. Browser
printing uses a separate snapshot, all document pages and the same local fonts.

Legacy Office API and agent writes refuse existing DOCX files they cannot
preserve. Use native Autor editing for those files, or explicitly export a
separate simplified copy. DOCX-to-DOCX downloads preserve the full package.

## Build and verification

- `npm run build:writer-vendor` regenerates local engine/font assets and licenses.
- `node scripts/build-writer-vendor.js --check` checks exact generated bytes.
- `npm run test:writer` checks save queue timing, revisions, failures and disposal
  (`scripts/test-writer-session.mjs`), the command table with its tracking rule,
  shortcuts and language keys (`scripts/test-writer-commands.mjs`) and the toolbar
  groups and overflow rule (`scripts/test-writer-toolbar.mjs`), the window
  menu model (`scripts/test-writer-menus.mjs`), the context surfaces
  (`scripts/test-writer-context.mjs`), the layout model and print sheets
  (`scripts/test-writer-layout.mjs`) and the templates
  (`scripts/test-writer-templates.mjs`).
- `AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run 'TestDesktopWriter(App|Engine)Browser'`
  exercises the actual editor, review, round-trips, conflicts, AI, imports,
  images, printing and a 100-page document.
- `AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_WRITER_MATRIX=1 go test ./ui -run '^TestDesktopAuroraBrowser$'`
  checks the real shell at desktop and touch sizes across all themes/densities.

Build the web resource set and executable together as described in
[web-assets.md](web-assets.md). Do not distribute an unpinned executable.
