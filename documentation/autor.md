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

A local IndexedDB draft supports explicit recovery after failures. Closing or
opening another document first attempts to save. If saving fails, the user may
keep a local recovery draft and close; unavailable recovery storage prevents
discarding the only unsaved copy.

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
  groups and overflow rule (`scripts/test-writer-toolbar.mjs`) and the window
  menu model (`scripts/test-writer-menus.mjs`).
- `AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run 'TestDesktopWriter(App|Engine)Browser'`
  exercises the actual editor, review, round-trips, conflicts, AI, imports,
  images, printing and a 100-page document.
- `AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_WRITER_MATRIX=1 go test ./ui -run '^TestDesktopAuroraBrowser$'`
  checks the real shell at desktop and touch sizes across all themes/densities.

Build the web resource set and executable together as described in
[web-assets.md](web-assets.md). Do not distribute an unpinned executable.
