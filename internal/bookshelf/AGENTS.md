# Bookshelf Domain Contract

`internal/bookshelf` owns the local Bookshelf catalog, scans, metadata,
collections, covers, annotations, reading progress and prepared-book cache.
The server owns HTTP authorization and injects authorized source adapters.

- Bookshelf is enabled by default in the config loader, template and settings UI.
  Preserve explicit `bookshelf_enabled: false` values across save and reload;
  activation still requires Desktop administrator authorization.
- Source adapters expose only list/stat/open. Never add a source write, rename,
  move or delete operation. Source credentials stay in the Vault and never enter
  these records.
- Source `Path`, adapter directory arguments and `Entry.Path` are slash-separated
  relative paths under the source's configured root. Reject absolute paths,
  traversal, NUL and ambiguous separators at this boundary.
- The source file is authoritative and remains unchanged. Catalog metadata,
  overrides, collection order, cover assets, annotations and progress live under
  the explicitly supplied private data root. Removing a source keeps its book
  history and marks its entries unavailable.
- Scan failures, cancellation, denied directories and limits never mark unseen
  books unavailable. Only a complete scan may do so. Bound entry count, depth,
  text fields, locators, covers, transfers and cache bytes.
- Uploads use the effective `limits.max_book_bytes` capability, including the
  Desktop file-size cap. HTTP 413 responses include `max_bytes`; the dialog
  explains the limit, restores its controls and retains successful batch items
  across a retry of the same selection.
- Managed uploads retain SHA-256 content revisions across upload, listing,
  preparation and content serving. The server caches bounded fingerprints only
  while file identity, size and modification time agree; existing imports and
  their reading history need no revision migration. Verify
  `TestBookshelfImportedPDFPreparesAndServesRangesWithoutLosingProgress`.
- The browser API serializes JSON mutations before the shared Desktop fetch;
  uploads retain native multipart boundaries. Fixtures must not normalize
  request bodies on behalf of production code. Verify `scripts/test-bookshelf-api.mjs`.
- Catalog edits use expected revisions. Progress uses its own compare-and-swap
  revision and carries the rendition revision. Never choose or merge competing
  reader positions automatically.
- Prepared files are private disposable cache data. Recheck source metadata
  before and after transfer, require the full advertised length, publish from a
  synced temporary file through `internal/fileutil`, deduplicate concurrent
  preparation, and never evict active leases.
- Schema migrations are versioned and preserve a backup before upgrading an
  existing catalog. Cache reset must not remove catalog data or covers.
- Book metadata and content are not sent to an LLM or external metadata service.
  Cover assets accept bounded raster images only; reject SVG and other active
  content.

Run `go test ./internal/bookshelf` for package changes. Changes to HTTP routes,
configuration, Desktop registration, UI, sources or reader assets must also read
their owning `AGENTS.md` files and preserve this package's contracts.
