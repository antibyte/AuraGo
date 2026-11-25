# HomeDash core

## Purpose

- Core of the HomeDash app: entity hub, widget catalog, dashboard document schema, `homedash.db`
  store and command pipeline.
- `sources/hass`: the full Home Assistant adapter: catalog with areas, websocket state stream with
  reconnect, REST fallback, typed commands, history and camera snapshots.
- `sources/hass/hasstest`: importable fake Home Assistant for tests. It must not import `hass`.
- `sources/system`: host metrics (CPU, memory, disk, network rates, uptime), read-only.

## Ownership

- `internal/homedash` imports only the standard library, `internal/security`, `internal/dbutil` and
  `modernc.org/sqlite`; never `server`, `agent`, `tools` or integrations.
- Adapters under `sources/` may import their own integration (`sources/hass` imports `internal/tools`).
- `sources/system` reads the server's collector sample and history ring through injected functions;
  it imports only `internal/homedash`. The server owns the sampler; the adapter never samples.
- Server wiring and the desktop API live in `internal/server/homedash_*.go` (see
  `internal/server/AGENTS.md`, "HomeDash API"); the config UI and kiosk are later milestones.
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
- `Subscriber.Set` drops duplicate ids silently (rejected ids too). One call publishes its snapshot
  as one batch, so one PUT yields one `snapshot` frame.
- `Hub.PrefetchCatalogs(ctx, ids)` refreshes only stale catalogs of the named sources, without
  interest or start. Call it before `Subscriber.Set` so snapshots carry real entities.
- `Hub.LookupEntities(ctx, ids)` answers validation lookups through the normal gates, with no
  warm-up.
- A catalog fetch abandoned because the caller's context ended (client disconnect) does not mark
  the source `catalog_failed`. A real per-source timeout still does.
- The dashboard cap may change at runtime: `Store.SetMaxDashboards` is atomic and safe during saves.
- `sources/system`: live values come from the collector sample; the server's ring rates (`NetRxMbps`/`NetTxMbps`, Mbit/s) are preferred, and the adapter computes its own only when the snapshot has none; history comes from the server's ring, which computes its rates the same way. `History` buckets ring entries (last entry per bucket as T, empty and NaN values
  skipped, at most `points` buckets). Its series answer `origin:"source"` (no `since`); the hub's
  in-memory ring is not used. `system:uptime` has no history.
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
  which are GET-only. Commands go through `tools.HACallServiceStates` (typed, never returns Home Assistant's text).
- Adapter locking: `a.mu` guards adapter state and is held while calling the sink. It is never held across network I/O.
  Lock order: `a.mu` → sink → `Hub.mu`.
- Catalog reads without a run use the one-shot `WSClient.Fetch` (states and registries). Without registries,
  REST `/api/states` is the fallback; the last known registries still apply.
- Polling runs only with interest. After three failed websocket handshakes, REST polling runs while REST answers
  (5-minute window, then one websocket attempt). A failing poll returns to websocket backoff.
- Catalog: areas come from the entity registry, else the device. Hidden, disabled, `alarm_control_panel` and invalid
  ids are excluded. `readonly_reason` is the first `blocked_by`. The catalog is
  pushed only when its meta fingerprint changes, and always before updates. A degraded or unavailable source is
  marked stale once.
- Factory: disabled when settings are missing, the source switch is off, or the token or URL is empty. A non-empty
  but unusable URL builds an adapter that reports `unavailable`/`invalid_url` without network I/O.
- Hub status precedence: a running source's own `unavailable` reason (`auth_failed`, `invalid_url`) wins over a
  concurrent `catalog_failed`. A non-running source with a failing catalog still reports `catalog_failed`.
- `homedash.Error.Source` is set by `SourceUnavailable(src, msg)` and names the failing source. `source_unavailable`
  is audited as `failed`, not `refused:<code>`.
- `Hub.SourceCatalog` is a raw, non-gated read (HomeDash switches are not applied) for templates only. It never
  starts a source or adds interest, and it is never used for client entity views.
- Command errors: policy refusals → 403 `read_only` with `blocked_by`; 4xx and 500 → `command_failed`; no
  connection, auth, 502/503/504 → `source_unavailable` (source `ha`). A caller deadline returns the raw `ctx.Err()`.
  Entities outside a known full list → `entity_not_found`.
- Changed states from a command go through the normal ingest path (`mapState`, `FilterAttributes`) before the sink.
- History: sensor (numeric), number, input_number, switch, input_boolean and binary_sensor. Other domains answer
  `[]` without a request. Climate advertises `history` but answers `[]` until M5.
- Snapshot: cameras only; jpeg, png or webp; at most 8 MiB; the declared type must match the sniffed bytes; width and
  height are ignored. Snapshots are never stored.
- Widget `icon` is optional and validated (`bad_format` at `/pages/N/widgets/M/icon`). `update_widget` merges it like
  `title`; an empty value never clears it.
- Icon names are the fixed list `IconNames()` (`KnownIcon`). Add names only together with
  `ui/js/homedash/hd-icons.js` and the contract.

## Work Guidance

- Hub and command tests use `sourcetest.FakeSource` and short real timers. Make ordering
  deterministic with a test hook instead of longer sleeps.
- Change the catalog only together with the golden file
  (`go test ./internal/homedash -run TestWidgetCatalogGolden -update`), and tell the frontend owners
  the new hash.
- A new SQLite schema version backs up through `dbutil.BackupSQLiteForMigration` first.
- A `Dashboard` built in Go sets `Look.RadiusScale` and `Grid.RowUnit` explicitly.
- `sources/hass/hasstest` (`New(t, Options)`, `DemoOptions()`, `URL`, `Token`) is the fake Home Assistant. Failure
  options: `FailStates`, `FailRegistries`, `ChangedStatesWrapped`, `NoEvents`, `NoChangedStates`.
- The opt-in live test `TestHALiveReadOnly` needs `HOMEDASH_HA_LIVE_URL` and `HOMEDASH_HA_LIVE_TOKEN` (both
  required, otherwise it skips). `HOMEDASH_HA_LIVE_TOGGLE` optionally names one switch or light, which the test
  toggles twice to restore its state.

## Verification

- `go test ./internal/homedash/... -count=1`
- `go test ./internal/homedash/sources/system/ -count=1`
- `go test ./internal/homedash/sources/hass/... -count=1`
- Race detector on Linux (Windows has no gcc): copy `go.mod`, `go.sum` and the packages
  `internal/homedash` needs to a Linux host, then run `go test -race ./internal/homedash/`.
- `go test ./internal/config/ -run '(?i)(HomeDash|SQLite)'`
- `go test ./internal/audit/ -run TestSQLiteOpensAreBackedUpOrExcluded`
- `go test ./internal/tools/ -run '(?i)(TestHA)'`

## Child DOX Index

None.
