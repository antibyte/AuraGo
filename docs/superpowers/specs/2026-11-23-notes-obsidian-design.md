# Notes app: Obsidian-style linking, navigation, graph and writing

Date: 2026-11-23. Status: approved by the user in chat ("Ja, umsetzen").

## Goal

Bring the desktop Notes app (`ui/js/desktop/apps/notes*.js`, `internal/desktop/service_notes.go`,
`internal/server/desktop_notes_handlers.go`) closer to Obsidian and improve its UI/UX, while keeping the
Milkdown/Crepe rich editor (user decision: extend Milkdown, no CodeMirror live-preview rewrite).

Scope chosen by the user: all four groups — Linking, Graph, Navigation, Writing — plus UI/UX polish.

## Current state (2026-11-23)

- Editor: Milkdown Crepe 7.22.3 bundled into `ui/js/vendor/notes/engine.js` by `scripts/build-notes-vendor.js`
  from `scripts/notes-engine-entry.js`; CodeMirror source mode. `NotesEditor.needsSource()` forces source mode
  for `[[wikilinks]]`, `![[embeds]]`, footnotes, `$$`, HTML and notes > 200k chars.
- Fidelity bug: after any rich edit the whole body is re-serialized by mdast-util-to-markdown, which escapes
  `[` at a break and in phrasing (`> [!note]` → `> \[!note]`), `#` at a break (`#tag` → `\#tag`) and `=` at a
  break (`==x==` → `\==x==`).
- Library: flat card list (97px rows), folder `<select>`, tag text field, pins, trash. No tree, backlinks, graph,
  quick switcher, daily notes, templates folder.
- Backend: `SearchNotes` walks and reads every note on every query; tags come only from frontmatter.
- The engine bundle is also imported by Mailer compose (`mailer-compose.js`); changes there must be additive.

## Design

### 1. Obsidian syntax in the rich editor (engine)

New source module `scripts/notes-obsidian.js`, re-exported from `scripts/notes-engine-entry.js`, bundled into
`ui/js/vendor/notes/engine.js` (rebuild with `npm run build:notes-vendor`; manifest and LICENSES regenerate).

Markdown layer (pure, node-testable): micromark syntax extension + mdast-util-from-markdown extension +
mdast-util-to-markdown extension.

| Syntax | mdast node | Serialized |
|---|---|---|
| `[[target#sub\|alias]]` | `wikiLink {value, embed:false}` (value = raw inner text) | `[[` + value + `]]` verbatim |
| `![[file.png]]`, `![[Note]]` | `wikiLink {value, embed:true}` | `![[` + value + `]]` |
| `#tag`, `#a/b` (previous char start/whitespace/`(`; tag chars `\p{L}\p{N}_-/`; not digits only) | `hashtag {value}` | `#` + value |
| `[!type]` / `[!type]+` / `[!type]-` at the start of a paragraph, followed by whitespace/EOL | `calloutMarker {value, fold}` | raw marker |
| `==text==` (exactly two `=`, nested phrasing allowed; port of gfm-strikethrough attention) | `highlight {children}` | `==` + phrasing + `==` |

Wikilink inner text: no newline, no `[[`/`]]`, ≤ 500 chars. `\[[x]]` stays literal. Code spans/blocks unaffected.

ProseMirror layer (Milkdown `$nodeSchema`/`$markSchema`/`$remark`/`$inputRule`/`$prose`):
- `wiki_link` inline atom `{value, embed}`; `hashtag` inline atom `{value}`; `callout_marker` inline atom
  `{value, fold}`; `highlight` mark (`<mark>`). Atoms define `leafText` so `editor.text`/word counts include them.
- Node views via callbacks passed from `notes-editor.js` (`obsidian(options)`):
  `resolve(value) → {path, exists}`, `open(value, event)`, `embed(value, dom)`, `preview(value, anchorEl)`,
  `tag(value)`, `icon(name) → html`. Wikilink chip shows alias, else target (+ `› sub`); unresolved chips are
  styled dashed/muted. Image embeds render `<img>`; note embeds render a card (title + rendered excerpt).
- Decoration plugin: a blockquote whose first paragraph starts with `callout_marker` gets class `notes-callout`
  and `data-callout=<type>`; the marker renders icon + label.
- Input rules: `[[x]]` → wiki_link, `![[x]]` → embed, `#tag␠` → hashtag, `==x==` → highlight,
  `[!type]␠` at a quote paragraph start → callout_marker.
- Commands: `toggleHighlight`, `insertWikiLink(value, embed)`, `insertCallout(type, title)`.
- `needsSource()` no longer triggers for wikilinks/embeds. HTML, footnotes, `$$`, invalid frontmatter and
  > 200k chars still force source mode.

### 2. Backend: notes index, inline tags, folders, link updates

`internal/desktop`:
- `notes_markdown.go`: `ExtractNoteRefs(content)` → title, aliases (frontmatter `aliases`/`alias`, string or list),
  tags (frontmatter + inline `#tags`, lowercased, de-duplicated), links with byte spans
  (`wiki` raw value + embed flag, `markdown` relative URL), skipping frontmatter (for inline scanning), fenced
  code (``` and ~~~) and inline code spans.
- Link resolution `resolveNoteTarget` (same rules in JS, shared fixture `ui/testdata/notes-links-fixture.json`):
  1. Split `target#sub|alias`; empty target = the source note itself.
  2. Target with `/`: exact path under `Documents/Notes/` (case-insensitive, `.md` optional), then relative to the
     source folder, then shortest path ending in `/target.md`.
  3. Plain name: basename match (case-insensitive); several → same folder as source, then shortest path, then
     alphabetical.
  4. Alias match (case-insensitive), shortest path first.
  5. Embeds/links with a non-`.md` extension resolve against attachment files the same way.
  6. Markdown links resolve relative to the source folder (URL-decoded) and must stay inside `Documents/Notes/`.
- `service_notes_index.go`: `NotesIndex(ctx)` returns `{notes:[{path,title,aliases,tags,links:[{target,path,embed,
  kind}],modified,size}], folders, attachments, skipped}`. Per-file parse cache keyed by path + mtime + size
  (mutex on Service), entries for deleted files dropped each walk.
- `noteSummary` uses the extractor so inline tags count for list chips and the tag filter.
- `RewriteNoteLinks(content, rewrite func(link) (string, bool))` span-based rewriting used by move.

`internal/server/desktop_notes_handlers.go`:
- `GET /api/desktop/notes?index=1` → index.
- `POST {"operation":"mkdir","folder":"Documents/Notes/X"}` → create folder (inside Notes, not `.attachments`,
  no `..`), publish `desktop_changed`.
- `PATCH {"operation":"move", "update_links":true}`: referencing notes are computed before the move; after it
  each referencing note is re-read and rewritten with a version precondition (wikilinks keep `#sub`, `|alias`
  and `.md` style; new target = basename if unique else Notes-relative path without `.md`; markdown links get a
  new relative URL keeping `#frag`). Response = note JSON + `updated_links` (count) + `link_update_failed`
  (paths). Agent sources unchanged: agents still cannot mutate existing notes.

### 3. Linking UI

- Client index `notes-index.js` (`window.NotesIndex`): loads `?index=1`, refreshes (debounced) on
  `desktop_changed`, offers `resolve`, `backlinks`, `outgoing`, `tags`, `fuzzy` search, `linkText(path)`
  (shortest unambiguous wikilink text), and pure helpers `extract`, `parseWikiValue` (node-tested).
- Click on a wikilink opens the note; an unresolved link creates `<current folder>/<target>.md` with `# target`.
- Hover (450 ms) shows a preview card (marked + DOMPurify, frontmatter stripped, first 4000 chars).
- `[[` autocomplete in rich mode (own popup at caret, ↑↓/Enter/Tab/Esc) and source mode (CodeMirror
  `autocompletion`). Items: notes (title, path), headings of the chosen note after `#`, aliases.
- Back/forward history (buttons + Alt+←/→).
- Right sidebar becomes one panel with tabs: Backlinks (linked + unlinked mentions via search), Outgoing links
  (resolved/unresolved), Outline, Local graph, Info, Assistant. One toggle button opens/closes it.
- Rename/move sends `update_links:true` and reports how many notes were updated.
- Status bar shows the backlink count (click opens the Backlinks tab).

### 4. Navigation

- Left sidebar tabs: Files (folder tree, notes by title, folders first, sort by name/modified, drag & drop to
  move, new folder, new note in folder, trash toggle at the bottom), Search (full text with `tag:`/`path:`
  prefixes mapped to API params, highlighted terms), Tags (nested tag tree with counts), Bookmarks (pins).
  Right-click on a tree row opens a small menu (open, rename, move, bookmark, trash).
- Quick switcher (Ctrl+O): fuzzy over title/path/aliases, empty query lists recent notes (meta `recent`, max 20),
  Enter opens, Shift+Enter or "create" row creates the note.
- Command palette (Ctrl+P): every menu action with shortcut hints. Print and "Open file…" stay in the menu and
  the palette (Obsidian keymap).
- Breadcrumb `Folder › Note` in the document bar. Inline find/replace bar (Ctrl+F) instead of a side panel.
- Shortcuts: Ctrl+O, Ctrl+P, Ctrl+N, Ctrl+S, Ctrl+F, Ctrl+G (graph), Ctrl+E (source mode), Ctrl+Shift+H
  (highlight), Alt+←/→.
- UI state (sidebar tab, right tab/open, expanded folders) in localStorage (try/catch); bookmarks, recent,
  last note and sort in `notes.meta.json` (`pinned` key kept for compatibility).

### 5. Graph

- `notes-graph.js` (`window.NotesGraph`): canvas renderer + Barnes-Hut force simulation, no external library.
  Zoom (wheel), pan (drag background), drag nodes, hover highlights neighbours and shows the label, click opens.
  Node size by degree; colours from theme CSS variables; simulation stops when it cools down; ResizeObserver.
- Global graph replaces the editor area (button + Ctrl+G, Esc closes). Controls: text filter, toggles for tags as
  nodes, unresolved links, orphans; "fit".
- Local graph tab in the right sidebar: depth 1 or 2 around the current note.

### 6. Writing

- Daily notes: `Documents/Notes/Daily/YYYY-MM-DD.md` (local date). "Today" button/command opens or creates it
  (template `Documents/Notes/Templates/Daily.md` if present, else `# {{date:dddd, D. MMMM YYYY}}`). On a daily
  note, ‹ › go to the adjacent existing daily notes.
- Templates: `Documents/Notes/Templates/*.md` appear in "New note from template" (with the built-ins) and
  "Insert template". Variables `{{title}}`, `{{date}}`, `{{time}}`, `{{date:FMT}}`, `{{time:FMT}}` with tokens
  YYYY YY MMMM MMM MM M DD D dddd ddd HH H mm ss (names via `Intl` in the document language). `notes-templates.js`
  (`window.NotesTemplates`), node-tested.
- Insert menu: Callout (note, info, tip, success, question, warning, danger, bug, example, quote); toolbar:
  highlight and wikilink buttons.
- Properties bar above the note body: tags and aliases as editable chips, other frontmatter keys read-only.
  `NotesFrontmatter.updateList(content, key, values)` generalises `updateTags` (tags keep the old behaviour).
  Replaces the comma field in the Info panel.

### 7. UI/UX

- Compact tree rows (~28px) instead of 97px cards; snippet cards only in search results.
- One right-sidebar toggle instead of four header buttons; bookmark star in the document bar.
- All desktop themes, compact density, narrow windows (< 900px: sidebars overlay) and touch sizes.
- New strings in all 16 `ui/lang/desktop/*.json` files.
- `notes.js` stays the orchestrator; new modules: `notes-index.js`, `notes-sidebar.js`, `notes-palette.js`,
  `notes-graph.js`, `notes-templates.js`, `notes-panels.js`. Added to `module-loader.js` and the browser fixture.

## Out of scope

Tabs/split panes, folder rename/delete, one-click linking of unlinked mentions, a CodeMirror live preview,
canvas/whiteboards, plugins. Rich edits still normalise cosmetic Markdown (e.g. `*` → `-` list markers).

## Testing

- Node: `scripts/notes-obsidian.test.mjs` (mdast round trip: parse → stringify == input for every syntax case,
  plus escapes, code, URLs with `#`, digits-only tags, line-start tags/highlights/callouts).
- Node: `ui/js/desktop/apps/notes-index.test.cjs` and `notes-templates.test.cjs` with the shared fixture.
- Go: extractor + resolver against the shared fixture, index cache, inline tags in search, mkdir, move with
  `update_links` (wikilink/alias/heading/markdown link rewritten, code untouched, conflict reported).
- Browser (rod, `TestDesktopNotesAppBrowser` + new `testdata/notes-obsidian-functional.js`): rich round trip of a
  note with all syntax after an unrelated edit, autocomplete insert, click + back, backlinks, quick switcher
  open/create, palette command, graph nodes + click, daily note path, property chip, callout class, highlight.
- Screenshots in default, fruity dark/light, compact, narrow (existing loop) plus graph and switcher.

## Delivery

Seven phases, each committed on local `main` (other sessions share the checkout: commit only own files):
1 engine syntax, 2 backend + client index, 3 linking UI, 4 navigation, 5 graph, 6 writing, 7 i18n/polish/tests.
