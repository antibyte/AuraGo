# FindIt Domain

## Purpose

FindIt stores physical homelab inventory, nested locations, stock, and project BOMs in a standalone SQLite database. Items describe canonical part definitions; stock rows describe quantities at locations.

## Ownership

- Own `internal/findit/` domain types, catalog, SQLite schema, queries, commands, CSV helpers, and tests.
- HTTP routes, agent tool registration, desktop UI, image storage, and backup orchestration belong to their owning packages.
- Store only opaque `image_id` and `icon_id` references. FindIt does not own image bytes or fetch datasheets.

- Migration backups use `dbutil.BackupSQLite`: owner-only staging and file permissions/Windows ACL, WAL-inclusive copy, sync and atomic publication. Backup failure aborts migration. Keep existing copies; collisions insert a unique suffix before the extension. Restore guidance: `documentation/migration-backups.md`.

## Local Contracts

- Public entry points are `Open(dbPath)`, `Close`, `Query(ctx, QueryRequest)`, `Execute(ctx, CommandRequest)`, static `Catalog()` metadata, database-backed `(*Store).Catalog(ctx)`, and the CSV preview/import/export methods. `ErrorCode` exposes stable error classes.
- `(*Store).Catalog(ctx)` returns static category/unit/package metadata with built-in template definitions from one read-only database snapshot. It omits custom templates, which are discovered through paginated template search; persisted built-in definitions remain authoritative when refreshes preserve incompatible schemas for existing items.
- Keep the FindIt database separate from the SSH device registry in `internal/inventory`.
- Store quantities as integer micro-units. Expose quantities as decimal strings with at most six fractional places; `piece` quantities must be whole numbers. Serials are unique per item and require piece units.
- Availability includes retained zero-quantity stock positions so returns and inventory corrections do not depend on the recent movement window. Item `stock_locations` lists only locations with positive on-hand stock. Availability and shortage results carry units; stock positions include complete location paths.
- Item identity uses manufacturer plus part number. `package_family` means an electronic footprint or physical component package such as DIP, TQFP, or 0603; it does not describe stock packaging or unit conversion.
- Template properties are typed text, number, boolean, enum, or ISO `YYYY-MM-DD` date values. Date filters compare canonical date strings. Numeric property searches use SQLite numeric values and support engineering notation, including `4k7` and `4,7 kΩ`, with case-sensitive SI prefixes. Explicit units match the template field's declared unit; numeric searches exclude legacy property-text aliases. Filters spanning incompatible field types or numeric units require an explicit template. Referenced archived template definitions remain filterable, including references from archived items.
- Item/location search accepts `id:` followed by a bounded hexadecimal UUID prefix for printed label identifiers. Return all matching records; callers must resolve ambiguity explicitly. SQL wildcard characters are rejected.
- Stock changes, reservations, consumption, and project lifecycle transitions run transactionally. A project must be released before reserving stock. Cancellation releases remaining holds without consuming stock; completion never consumes stock implicitly. Consumption is explicit and reduces physical stock. Archived projects are immutable. Returns and positive adjustments require an active stock location; restoring an issued serial uses `return`. The total on-hand quantity for an item must fit the signed 64-bit micro-unit range; transfers preserve that total.
- Items referenced by an active BOM in a draft or released project cannot be archived. Releasing a project rejects any archived BOM item. Custom templates may be archived while items still reference them; those references and property definitions remain available for item history and edits that keep the archived template assignment unchanged. New items and reassignment cannot use archived templates. Any item reference, including archived items, keeps a template's field definitions and category in use; built-in catalog refreshes preserve incompatible used definitions and category.
- Keep all active reservations visible in project detail. The domain caps active reservations at 5,000 per project; project detail returns all active holds plus the latest 100 closed records and sets `reservations_has_more` when older history exists. Item detail returns the latest 100 movements.
- Commands with a `request_id` store the result receipt in the same SQLite transaction as the mutation. Route adapters should require request IDs for mutations. Every request transaction commit must use `fileutil.PublishContext(ctx, tx.Commit)` so Desktop publication revocation serializes with writes.
- Stock commands targeting an existing `stock_id` accept `stock_version`; a positive value must match the stored position version before mutation. The Desktop sends the observed version with its preview. Zero or omitted versions preserve legacy direct-client behavior. Receipt replay occurs before version checks, so an acknowledged write can be recovered after later changes.
- `PreviewCSV` accepts `items` or `project_lines`; `ImportCSV` recomputes validation and commits all rows atomically. Duplicate item identities require explicit `skip_rows`; project BOM rows resolve an exact item ID or unique manufacturer and part number, otherwise an explicit row resolution. Project-line CSV replaces the current draft BOM and requires `expected_version`. CSV imports never change stock. `ExportCSV` supports `items`, `stocks`, and `project_lines`.
- CSV accepts UTF-8, an optional BOM, comma or semicolon delimiters, and unique headers. Item arrays and properties use JSON cells. Imports are limited to 10 MiB, 256 columns and 25,000 rows; BOMs are limited to 1,000 lines. Enforce row/column limits before materializing an oversized record set.
- CSV exports prefix spreadsheet-formula-like text and original leading apostrophes with an apostrophe. The reserved `findit_csv_encoding` column declares `literal-v1`; imports with that marker decode one leading apostrophe and reject unknown versions. Unmarked external CSV is never heuristically decoded. Preserve the marker and original CSV for lossless FindIt reimport; spreadsheet re-saving can change literal encoding.

## Work Guidance

- Keep domain logic independent of HTTP status codes and UI wording. Return safe domain errors with stable codes.
- Keep project release snapshots immutable. Preserve inactive BOM lines when a revision changes so reservation and movement references remain valid.
- Keep database migration backups in the database directory. Desktop backup orchestration must snapshot this database together with referenced media while holding its runtime exclusive lock.
- Do not add purchasing workflows, external catalog fetches, credential storage, or image persistence here.

## Verification

- Run `go test ./internal/findit -count=1` for domain, stock, reservation, search, and CSV behavior.
- `BenchmarkSearchItems50k` builds a 50,000-item FTS and typed-property fixture and measures paginated text, engineering-number and unit searches. Resolve unit definitions once per template and use the property index; do not correlate JSON field enumeration with each item. Run with `go test ./internal/findit -run '^$' -bench BenchmarkSearchItems50k -benchtime=1x` when evaluating search changes.
- Format Go files with `gofmt` and run `git diff --check` before closeout.

## Child DOX Index

None.
