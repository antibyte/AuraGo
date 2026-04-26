# Media Bulk Delete Design

## Goal

Add a safe bulk-delete workflow to the Media View so users can mark multiple visible files and delete only those selected files in one action.

## Scope

V1 supports explicit selection only. It does not delete all search results, all files in a tab, or hidden files outside the currently rendered page unless the user selected those visible cards or rows.

Supported tabs:

- Images
- Audio
- Videos
- Documents

## UX

The Media View gets a compact bulk toolbar near the existing search controls:

- Selection mode toggle: `Select`
- Page selection action: `Select visible`
- Destructive action: `Delete selected`
- Clear action: `Clear selection`
- Count label: `N selected`

Cards and rows show a checkbox when selection mode is active. Selecting a checkbox must not open a lightbox, start playback, or open a document. The delete button is disabled when the current tab has no selected items.

Selections are per-tab and cleared when the user changes tabs, changes the search query, changes a filter, or changes pagination.

## Backend

Add bulk endpoints instead of issuing many single DELETE requests from the browser:

- `POST /api/media/bulk-delete`
- `POST /api/image-gallery/bulk-delete`

The image endpoint is needed because images can come from either the media registry or the legacy image gallery. The payload preserves the existing source distinction:

```json
{
  "items": [
    {"id": 12, "source": "media_registry"},
    {"id": 18, "source": "image_gallery"}
  ]
}
```

The generic media endpoint uses media IDs:

```json
{
  "ids": [3, 4, 5]
}
```

Both endpoints return:

```json
{
  "status": "ok",
  "deleted": 3,
  "failed": []
}
```

Partial failure returns `status: "partial"` with a `failed` array containing the item ID and message. Empty selections return `400`.

## Data Flow

1. The page loads a tab and renders its visible page.
2. The user enables selection mode.
3. The user marks visible items.
4. The user clicks `Delete selected`.
5. The UI shows one shared confirmation dialog with the selected count.
6. The browser calls the relevant bulk endpoint.
7. The backend reuses the same deletion behavior as single delete:
   - soft-delete registry records
   - clear companion image records when needed
   - best-effort remove physical files
8. The UI clears selection and reloads the active tab.

## Safety

- Only explicit selected IDs are sent.
- The backend validates every ID against the registry before deletion.
- Physical file deletion remains best-effort and scoped to existing item metadata.
- Partial failures are reported instead of hiding errors.
- The UI does not keep stale selected IDs across search, filter, pagination, or tab changes.

## Tests

Backend tests:

- bulk media delete succeeds for multiple registry items
- bulk media delete rejects empty ID list
- bulk media delete reports partial failures
- bulk image delete removes companion records like existing single delete

Frontend regression tests:

- Media View contains bulk toolbar markers
- Cards/rows expose selectable controls
- bulk delete calls the new endpoints
- selection clears on tab/search/page changes
