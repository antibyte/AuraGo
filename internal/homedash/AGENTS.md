# HomeDash core

## Purpose

- Core of the HomeDash app: entity hub, widget catalog, dashboard document schema, `homedash.db`
  store and command pipeline.
- `sources/hass`: Home Assistant domain/attribute mapping and websocket client.

## Ownership

- `internal/homedash` imports only the standard library, `internal/security`, `internal/dbutil` and
  `modernc.org/sqlite`; never `server`, `agent`, `tools` or integrations.
- Adapters under `sources/` may import their own integration (`sources/hass` imports `internal/tools`).
- Server wiring, HTTP routes, the config UI and the frontend are later milestones.
- Design: `docs/superpowers/specs/2026-11-23-homedash-design.md`. Its section 4 wins over the part
  documents in `docs/superpowers/specs/2026-11-23-homedash/`.
- The config block `homedash` lives in `internal/config/homedash.go`.
- The database file name and its backup/protection entry live in `internal/config/sqlite_paths.go`.
  The store takes a path; the file name literal stays out of this package.

## Local Contracts

- Entity ids are `<source>:<native id>`. They travel in query strings only, never in path segments.
- Hub locking: lock order `Hub.mu` then `Subscriber.mu`. No call into a `Source` while `Hub.mu` is
  held, except factories during config apply. Interest notes are delivered after unlock, in order
  per source. Sinks, timers and catalog results of an earlier run are ignored (generation and
  sequence guards).
- Runs of one source never overlap: a new run waits for the previous one to return. A rebuilt source
  drops its catalog and cached states. Entities that vanished are reported as removed with the
  first new catalog.
- Subscribers: sinks never block; latest state wins per entity; memory is bounded by the
  subscription size; the state cache keeps only entities in the catalog or with interest.
  Idle sources stop after 60 s.
- Command order: principal kind and key (agents, unknown kinds and empty keys are refused) ->
  request id -> hub closed / HomeDash disabled -> kiosk disabled -> kiosk: entity in the allowed set
  (before any lookup, so a display cannot probe for entities) -> entity and source lookup (the
  catalog is fetched only when stale) -> capability and argument validation -> integration gate ->
  HomeDash read-only, desktop read-only, source control switch -> kiosk allowed command -> view-only
  -> kiosk-denied capabilities -> confirmation for sensitive commands -> rate limits (the per-entity
  window is separate per principal kind) -> request-id dedupe -> at most 4 in flight per source ->
  latest-wins slot for continuous controls -> switches re-checked right before the source call ->
  audit (a replaced command is audited as `superseded`).
- The control gate fails closed: a source with `Control` false, or a registered source without a
  settings entry, refuses every command with `read_only`. The entity view shows the same block
  (`controlBlock`).
- Agents have no command path and no stream. A kiosk principal with a nil `Allowed` set gets
  nothing. Unlock and container actions are never allowed from a kiosk.
- The kiosk `AllowedSet` comes from the APPROVED revision's widget bindings combined with the
  catalog's `Commands` (`BuildAllowedSet`). The hub's read functions take no principal: kiosk routes
  must check the allowed set before calling them. When a dashboard's approved revision moves, the hub
  ends that dashboard's kiosk streams with `CloseApprovalChanged`; the display re-attaches with a
  fresh allowed set.
- Store (`homedash.db`): agent saves never move `approved_revision`; a user save sets it. Revisions
  keep the last 20 plus the approved one. Pairing codes and device tokens are stored only as SHA-256
  hashes. Every pairing failure returns the same error. Multi-statement changes run in one
  transaction. An update of a document written by a newer schema is refused with `schema_too_new`.
  Expired displays do not count against the device cap, and a cap of zero refuses.
- Documents carry no URLs. Only absent keys take defaults: an explicit out-of-range
  `look.radius_scale` or `grid.row_unit` is rejected. Layout items must lie inside the grid. Every
  save path, full document or patch, runs `Normalize`, `Autopack` and `Validate` before the store.
  `ApplyPatch` alone does not validate.
- Capability arguments: an entity's min/max narrow the registry's fixed range, never widen it.
  Attributes never carry tokens, URLs or coordinates (`BoundAttributes` plus the `hass` allowlist).
- The widget catalog is Go data plus the golden `testdata/widget_catalog.json` (24 types, 47
  variants), which the frontend contract test reads.
- `sources/hass` holds the only Home Assistant websocket client: one writer, token only in the auth
  message, 16 MiB read limit, one registry list batch in flight, backoff resets only after a stable
  session. It reads Home Assistant over REST only through `tools.HARequestJSON` / `tools.HAGetBytes`,
  which are GET-only. Commands go through `tools.HACallServiceContext`.

## Work Guidance

- Hub and command tests use `sourcetest.FakeSource` and short real timers. Make ordering
  deterministic with a test hook instead of longer sleeps.
- Change the catalog only together with the golden file
  (`go test ./internal/homedash -run TestWidgetCatalogGolden -update`), and tell the frontend owners
  the new hash.
- A new SQLite schema version backs up through `dbutil.BackupSQLiteForMigration` first.
- A `Dashboard` built in Go sets `Look.RadiusScale` and `Grid.RowUnit` explicitly.

## Verification

- `go test ./internal/homedash/... -count=1`
- Race detector on Linux (Windows has no gcc): copy `go.mod`, `go.sum` and the packages
  `internal/homedash` needs to a Linux host, then run `go test -race ./internal/homedash/`.
- `go test ./internal/config/ -run '(?i)(HomeDash|SQLite)'`
- `go test ./internal/audit/ -run TestSQLiteOpensAreBackedUpOrExcluded`
- `go test ./internal/tools/ -run '(?i)(TestHA)'`

## Child DOX Index

None.
