# Desktop Gallery

The Gallery is the Virtual Desktop viewer for the images and videos in the
Desktop media folders `Photos` and `Videos` (including the media the agent
generates there). It follows the desktop theme (`standard` and `fruity`,
light and dark), reduced-motion settings and the Desktop sound and
confirmation settings. All controls are localized in the 16 Desktop
languages.

## Browsing

- **Tabs** switch between Photos and Videos. Each tab shows its item count;
  the status bar shows the loaded item count and the total size.
- **Grouping by date** puts sticky section headers (Today, Yesterday, This
  week, month, month and year) above the grid. Grouping is available while
  sorting by date and can be switched off for a single flow.
- **Sorting** by newest, oldest, name or size is available from the View
  menu, the toolbar sort button and the background context menu. The active
  sort and grouping are shown with a check mark.
- **Tile size** is adjusted with the zoom slider in the status bar, its
  `+`/`−` buttons, the View menu or `Ctrl`/`Cmd`+mouse wheel over the grid.
  Six sizes from 104 to 320 px are available.
- **Search** filters loaded items by name, folder, description, tags, generation
  prompt and source. The clear
  button and `Escape` reset the search. A localized empty state explains when
  nothing matches.
- Large libraries load page by page (80 items per page). Scrolling to the end
  loads the next page automatically; a **Load more** button is offered as a
  fallback.
- Tile size, sort, grouping, tab and info-panel state are remembered per
  browser (`aurago.desktop.gallery.v1` in local storage).

## Selecting and managing files

- A **plain click** opens the item in the lightbox. `Ctrl`/`Cmd`-click,
  `Shift`-click for ranges, the tile checkbox or the **Select** mode toggle
  build a selection.
- Arrow keys, `Home` and `End` move the keyboard focus through the grid,
  `Space` toggles the focused item, `Enter` opens it, `Delete` deletes the
  selection, `Ctrl`/`Cmd`+`A` selects everything and `Escape` clears the
  selection.
- The **selection bar** shows the count and size and offers Download, Rename
  (single item) and Delete. Deleting asks for confirmation when
  `files.confirm_delete` is enabled and plays the Desktop delete sound. Failed
  deletions are summarized in one localized notification.
- The **info panel** (`I` or the toolbar button) shows a preview, editable
  name, folder, type, size, modified date and, for images and videos, the
  measured dimensions or duration. Multiple selected items show an aggregated
  summary.
- Item context menus offer Open, Download, Edit in Pixel (images), Show in
  Files, Copy link, Rename and Delete. The window menus (File, Edit, View)
  expose the same actions with keyboard shortcuts.
- When the Desktop is **read-only**, rename and delete controls are omitted
  and the info panel shows a read-only hint. Uploading is not offered because
  the media mounts are read-only for the browser; new media arrives through
  the agent, the File Manager or the host file system.

## Image metadata and categories

Open the Gallery info panel and select a photo to edit its description and
comma-separated tags. Original generation prompts and source identifiers remain
visible. Generating apps populate image descriptions from their existing prompt
when no explicit description is supplied; Noisemaker covers retain their
`cover` and `noisemaker` tags. These initial descriptions express generation
intent, while an explicit vision analysis describes the visible image.

**Manage categories** creates, renames or removes categories with one or more
keywords. Each category automatically includes images whose description, tags,
prompt or source contains any of its keywords, without case sensitivity. For
example, a Music covers category can use `cover, noisemaker`. The category
dropdown above the grid combines with the text search. Removing a category does
not remove images or metadata. Categories are shared between browsers; stale
saves are refused instead of overwriting another window's changes.

**Analyze with AI** analyzes the selected photo. **Analyze all photos** analyzes
the complete loaded Photos library, regardless of the category or text filter.
Both ask before sending images to the configured Vision provider. More than
20 photos trigger an additional warning about increased costs. Descriptions
are replaced, existing tags are retained, and the original prompt is preserved.
The queue processes one image at a time and shows successes and failures. Cancel,
closing the app or Desktop permission revocation stops remaining work; already
completed results stay saved. A provider request already sent may still incur
charges. Failed requests are not automatically retried.

Vision requires a configured provider supporting inline images; PNG, JPEG, GIF
and WebP inputs are limited to 20 MB each. Vision budget enforcement and usage
accounting apply. Analysis requires Desktop administrator access; read-only
Desktop mode blocks metadata/category changes and analysis. Failed metadata
loads leave photos viewable with an explicit warning. Images themselves are
never modified by metadata editing.

Metadata uses the existing media registry database, with legacy image-gallery
entries promoted on the first edit. Categories use the validated
`gallery.categories` Desktop setting, so no schema migration is needed.
The agent can read and search descriptions and tags with the existing
`media_registry` `list`, `search` and `get` operations. Category rules are
dynamic filters, not duplicate copies of the images.

## Lightbox

The lightbox is shared by the Gallery, the File Manager and other apps that
open images, videos or audio (`openMediaPreview`). It provides:

- Previous/next navigation with a filmstrip of neighbouring items and a
  position counter (arrow keys, `PageUp`/`PageDown`, `Home`, `End`).
- Zoom with the mouse wheel, pinch, double-click, `+`/`−`, `0` (fit) and `1`
  (actual size); panning by dragging when zoomed in.
- A slideshow (`Space`) with a progress indicator, native video and audio
  playback, and an info drawer (`I`) with the same metadata as the info panel.
- Download (`D`), Edit in Pixel, Show in Files, rename (`F2`) and delete
  (`Delete`) when the Desktop allows them.
- Chrome that hides automatically after a few seconds without pointer
  movement and returns on any interaction. `Escape` or a click on the
  backdrop closes the viewer.

## Live updates

The Gallery listens to the Desktop `desktop_changed` events, polls every
45 seconds and reloads when the tab becomes visible again, so files added,
renamed or deleted by the agent or the File Manager appear without a manual
refresh. The backend invalidates its recursive listing cache on every Desktop
mutation so the refreshed listing is current.

## Implementation notes

The app is loaded lazily from `ui/js/desktop/apps/gallery*.js` with
`ui/css/desktop-app-gallery.css`; the shell contract lives in
`ui/js/desktop/apps/AGENTS.md` (Gallery contract). Verify with
`go test ./ui -run 'Gallery|WindowMenu|ContextMenu'` and
`go test ./internal/desktop -run RecursiveCacheInvalidated`.
