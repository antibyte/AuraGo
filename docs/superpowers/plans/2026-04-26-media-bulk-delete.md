# Media Bulk Delete Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let users select multiple visible files in the Media View and delete only those selected files in one confirmed action.

**Architecture:** Add backend bulk-delete endpoints that reuse existing single-delete semantics, then add a lightweight per-tab selection model in the Media View. The UI only submits explicit visible selections and reloads the active tab after deletion.

**Tech Stack:** Go HTTP handlers, SQLite-backed media registries, vanilla JavaScript SPA, existing shared modal helpers, existing media/gallery CSS.

---

## File Structure

- Modify `internal/server/media_handlers.go`: add reusable delete helper and `handleMediaBulkDelete`.
- Modify `internal/server/image_generation_handlers.go`: add reusable image delete helper and `handleImageGalleryBulkDelete`.
- Modify `internal/server/server_routes_config.go`: register the two bulk routes before dynamic ID routes.
- Add or modify server tests in `internal/server/media_handlers_test.go` and `internal/server/image_generation_handlers_test.go`.
- Modify `ui/media.html`: add the bulk toolbar.
- Modify `ui/js/media/main.js`: add selection state, toolbar behavior, bulk requests, and tab-specific selection rendering.
- Modify `ui/js/gallery/main.js`: add image card selection hooks that work inside Media View.
- Modify `ui/css/media.css` and possibly `ui/css/gallery.css`: style toolbar, selected cards, and checkbox positions.
- Modify `ui/lang/media/*.json`: add translations for all supported languages.
- Modify `ui/chat_regression_test.go`: add regression markers for the Media View bulk-delete flow.

---

## Task 1: Backend Bulk Delete for Registry Media

**Files:**

- Modify: `internal/server/media_handlers.go`
- Create or modify: `internal/server/media_handlers_test.go`

- [ ] **Step 1: Add failing tests for `/api/media/bulk-delete`**

Add tests that initialize a temporary media registry DB, register three media items, call `POST /api/media/bulk-delete`, and assert that selected IDs are soft-deleted while unselected IDs remain.

Also add:

- empty list returns `400`
- one missing ID returns `status: "partial"` and preserves successful deletions

- [ ] **Step 2: Extract single-item media deletion helper**

Create an unexported helper in `media_handlers.go`:

```go
func (s *Server) deleteMediaItemByID(id int64, dataDir string) error
```

Move the current `DELETE /api/media/{id}` behavior into that helper:

- `tools.GetMedia`
- `tools.DeleteMedia`
- best-effort physical delete from `FilePath` or type-based data subdirectory

- [ ] **Step 3: Add bulk request/response structs**

Use:

```go
type mediaBulkDeleteRequest struct {
    IDs []int64 `json:"ids"`
}

type mediaBulkDeleteFailure struct {
    ID      int64  `json:"id"`
    Message string `json:"message"`
}
```

- [ ] **Step 4: Implement `handleMediaBulkDelete`**

Parse JSON, reject empty `ids`, deduplicate IDs, call `deleteMediaItemByID` for each ID, and return:

- `status: "ok"` when all selected items delete
- `status: "partial"` when at least one selected item fails
- `deleted` count
- `failed` array

- [ ] **Step 5: Register route**

In `internal/server/server_routes_config.go`, register:

```go
mux.HandleFunc("/api/media/bulk-delete", handleMediaBulkDelete(s))
```

Place it before `mux.HandleFunc("/api/media/", handleMediaByID(s))`.

- [ ] **Step 6: Run focused tests**

Run:

```powershell
$env:GOCACHE = (Resolve-Path disposable\gocache).Path
go test ./internal/server -run "TestMediaBulkDelete|TestHandleMedia" -count=1
```

Expected: PASS.

---

## Task 2: Backend Bulk Delete for Images

**Files:**

- Modify: `internal/server/image_generation_handlers.go`
- Modify: `internal/server/image_generation_handlers_test.go`
- Modify: `internal/server/server_routes_config.go`

- [ ] **Step 1: Add failing tests for `/api/image-gallery/bulk-delete`**

Create a test with one media-registry image and one image-gallery image. Call:

```json
{
  "items": [
    {"id": 1, "source": "media_registry"},
    {"id": 2, "source": "image_gallery"}
  ]
}
```

Assert both active records are removed and companion records are cleared just like single delete.

- [ ] **Step 2: Extract image deletion helper**

Create:

```go
func (s *Server) deleteImageGalleryItemByID(id int64, source, dataDir string) error
```

Move the current image `DELETE` branch logic into this helper.

- [ ] **Step 3: Add bulk image structs**

Use:

```go
type imageGalleryBulkDeleteItem struct {
    ID     int64  `json:"id"`
    Source string `json:"source"`
}

type imageGalleryBulkDeleteRequest struct {
    Items []imageGalleryBulkDeleteItem `json:"items"`
}
```

- [ ] **Step 4: Implement `handleImageGalleryBulkDelete`**

Deduplicate by `source + ":" + id`, delete each item with the helper, and return `ok` or `partial` using the same response shape as the media bulk endpoint.

- [ ] **Step 5: Register route**

In `internal/server/server_routes_config.go`, register:

```go
mux.HandleFunc("/api/image-gallery/bulk-delete", handleImageGalleryBulkDelete(s))
```

Place it before `mux.HandleFunc("/api/image-gallery/", handleImageGalleryByID(s))`.

- [ ] **Step 6: Run focused tests**

Run:

```powershell
$env:GOCACHE = (Resolve-Path disposable\gocache).Path
go test ./internal/server -run "TestImageGalleryBulkDelete|TestImageGalleryDelete" -count=1
```

Expected: PASS.

---

## Task 3: Media View Selection State and Toolbar

**Files:**

- Modify: `ui/media.html`
- Modify: `ui/js/media/main.js`
- Modify: `ui/css/media.css`
- Modify: `ui/lang/media/*.json`

- [ ] **Step 1: Add toolbar markup**

Add a toolbar under `.media-tabs`:

```html
<div id="media-bulk-toolbar" class="media-bulk-toolbar" aria-live="polite">
    <button id="media-select-mode-btn" type="button" class="btn-gallery-nav" onclick="toggleMediaSelectionMode()" data-i18n="media.bulk_select">Select</button>
    <button id="media-select-visible-btn" type="button" class="btn-gallery-nav" onclick="selectVisibleMediaItems()" data-i18n="media.bulk_select_visible">Select visible</button>
    <button id="media-clear-selection-btn" type="button" class="btn-gallery-nav" onclick="clearCurrentMediaSelection()" data-i18n="media.bulk_clear_selection">Clear selection</button>
    <button id="media-delete-selected-btn" type="button" class="btn-gallery-nav btn-danger" onclick="deleteSelectedMediaItems()" disabled data-i18n="media.bulk_delete_selected">Delete selected</button>
    <span id="media-selected-count" class="media-selected-count" data-i18n="media.bulk_selected_none">0 selected</span>
</div>
```

- [ ] **Step 2: Add translations**

Add these keys to all `ui/lang/media/*.json` files:

- `bulk_select`
- `bulk_select_done`
- `bulk_select_visible`
- `bulk_clear_selection`
- `bulk_delete_selected`
- `bulk_selected_count`
- `bulk_confirm_delete`
- `bulk_deleted`
- `bulk_partial_deleted`

- [ ] **Step 3: Add selection state**

In `ui/js/media/main.js`, add:

```js
let mediaSelectionMode = false;
const mediaSelections = {
    images: new Map(),
    audio: new Map(),
    videos: new Map(),
    documents: new Map()
};
```

Each selected value should store the item needed by the bulk call:

- images: `{id, source}`
- other tabs: `{id}`

- [ ] **Step 4: Clear selection on navigation**

Call `clearCurrentMediaSelection()` or a full selection reset on:

- tab switch
- search input debounce before reload
- provider filter change
- pagination functions

- [ ] **Step 5: Add toolbar functions**

Implement:

- `toggleMediaSelectionMode()`
- `selectVisibleMediaItems()`
- `clearCurrentMediaSelection()`
- `updateMediaBulkToolbar()`
- `toggleMediaItemSelection(tab, key, payload, checked)`
- `getCurrentVisibleMediaItems()`

- [ ] **Step 6: Style toolbar and selected state**

Add styles:

- `.media-bulk-toolbar`
- `.media-selected-count`
- `.media-select-check`
- `.media-card-selected`
- mobile wrapping under 640px

---

## Task 4: Selection Rendering for All Tabs

**Files:**

- Modify: `ui/js/gallery/main.js`
- Modify: `ui/js/media/main.js`
- Modify: `ui/css/gallery.css`
- Modify: `ui/css/media.css`

- [ ] **Step 1: Image cards**

In `renderGrid(images)`, when `window.mediaSelectionMode` or global `mediaSelectionMode` is active, render a checkbox with:

```html
<input type="checkbox" class="media-select-check" data-tab="images" data-id="..." data-source="...">
```

Stop click propagation on the checkbox so the lightbox does not open.

- [ ] **Step 2: Audio cards**

In `renderAudioGrid`, prepend a checkbox to each card when selection mode is active. Store `{id: item.id}` in `mediaSelections.audio`.

- [ ] **Step 3: Video cards**

In `renderVideoGrid`, prepend a checkbox to each card when selection mode is active. Store `{id: item.id}` in `mediaSelections.videos`.

- [ ] **Step 4: Document rows**

In `renderDocList`, add a checkbox at the left edge of each row when selection mode is active. Store `{id: item.id}` in `mediaSelections.documents`.

- [ ] **Step 5: Preserve action behavior**

Verify:

- checkbox click does not open image lightbox
- checkbox click does not start video/audio
- document open/download/delete actions still work

---

## Task 5: Bulk Delete Browser Flow

**Files:**

- Modify: `ui/js/media/main.js`

- [ ] **Step 1: Implement `deleteSelectedMediaItems`**

Use `showConfirm` with the selected count. If cancelled, do nothing.

For images:

```js
await fetch('/api/image-gallery/bulk-delete', {
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    body: JSON.stringify({items})
});
```

For other tabs:

```js
await fetch('/api/media/bulk-delete', {
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    body: JSON.stringify({ids})
});
```

- [ ] **Step 2: Handle responses**

On `ok`, show success toast, clear selection, and reload the tab.

On `partial`, show warning toast with deleted and failed counts, clear successful selections, and reload the tab.

On `error`, show alert or error toast and keep selection.

- [ ] **Step 3: Pagination correction**

After deleting, if the reloaded page returns empty and the previous offset was greater than zero, decrement offset by `MEDIA_LIMIT` or `GALLERY_LIMIT` and reload once.

---

## Task 6: Regression Tests and Verification

**Files:**

- Modify: `ui/chat_regression_test.go`

- [ ] **Step 1: Add frontend marker test**

Add markers for:

- `media-bulk-toolbar`
- `toggleMediaSelectionMode`
- `selectVisibleMediaItems`
- `deleteSelectedMediaItems`
- `/api/media/bulk-delete`
- `/api/image-gallery/bulk-delete`
- `media-select-check`

- [ ] **Step 2: Run frontend test**

Run:

```powershell
$env:GOCACHE = (Resolve-Path disposable\gocache).Path
go test ./ui -run TestMediaFrontend -count=1
```

Expected: PASS.

- [ ] **Step 3: Run backend focused tests**

Run:

```powershell
$env:GOCACHE = (Resolve-Path disposable\gocache).Path
go test ./internal/server -run "TestMediaBulkDelete|TestImageGalleryBulkDelete|TestImageGalleryDelete" -count=1
```

Expected: PASS.

- [ ] **Step 4: Run whitespace check**

Run:

```powershell
git diff --check
```

Expected: no whitespace errors. CRLF warnings for UI files are acceptable if they match existing repository behavior.

- [ ] **Step 5: Commit**

Commit implementation:

```powershell
git add internal/server/media_handlers.go internal/server/image_generation_handlers.go internal/server/server_routes_config.go internal/server/*media*test.go internal/server/image_generation_handlers_test.go ui/media.html ui/js/media/main.js ui/js/gallery/main.js ui/css/media.css ui/css/gallery.css ui/lang/media/*.json ui/chat_regression_test.go
git commit -m "feat: bulk delete selected media items"
```
