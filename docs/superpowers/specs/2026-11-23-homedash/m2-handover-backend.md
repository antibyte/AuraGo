# HomeDash M2 backend: hand-over

Branch `claude/great-napier-86e554` in worktree `.claude/worktrees/great-napier-86e554`, based on main `40bafdfe8`. 17 commits (16 below plus this hand-over commit). Not merged, not pushed, not deployed.

## Commits (oldest first)

| Commit | Content |
|---|---|
| 027330bde | Task 1 core: PrefetchCatalogs, LookupEntities, atomic dashboard cap |
| e8bb91f9c | Task 2 `system` source adapter (+ A1 history) |
| 07c79402b | Task 3 metrics sample: network counters, host boot time, A1 history ring |
| 3c69534d2 | Task 4 source table |
| 6be8d4ead | fix (batch 1 review): NaN for unreadable metrics in history; caller cancel is no catalog failure |
| 83f0d3d96 | Task 5 service wiring, hot reload (A2), desktop capability |
| 9cc92eae4 | Task 6 route table, envelope, bootstrap, widget catalog, audit contract (A3) |
| 1fddf72c7 | Task 7a save pipeline, starter template, CRUD |
| 8a9761bb4 | Task 7b duplicate, revisions, restore, approve, validate |
| 43a85242a | fix (batch 2 review): service published under CfgMu (read) |
| 92f75d072 | Task 8 entities, entity, history, snapshot |
| b5a194fc2 | Task 9 commands route |
| 46163404a | Task 10 SSE stream and subscriptions |
| c1a453d86 | Task 11 end-to-end test (A1 history assertion) |
| 2452738f3 | fix (batch 3 review): one snapshot per subscription PUT, silent duplicates, login-off principal |
| 98f39a056 | Task 12 docs (`internal/homedash/AGENTS.md`, `internal/server/AGENTS.md`) |

## Verification (final tree 98f39a056, Windows, go1.27.4)

- `go build ./...` ok; `go vet` on homedash, server, audit ok; `go test` on `internal/homedash/...` (incl. `sources/system`), config, httpstream, `audit/...` ok.
- Server subset (`HomeDash|Metrics|ChibiSensei|DockerManagerStream|DesktopReadonly|BluetoothLive`): 65 PASS / 0 FAIL; full `internal/server` (`-skip Browser`) ok in 408 s, no failures. Task 0 baseline was fully green (subset 39 PASS).
- Race (aurago-test, go1.27.4 via `GOTOOLCHAIN=auto`): `internal/homedash/... -race` ok; `internal/server -race -count=3 HomeDash` ok; no data race. Stream tests passed 6 consecutive runs.

## Lead amendments applied

- A1: `system` history comes from a server ring since start (`origin:"source"`, no `since`).
- A2: reconfigure after publish; later under CfgMu read.
- A3: stream protocol constant declared once, in `homedash_routes.go`.

## Deviations from the plan / contract

- Store path key is `config.SQLiteArchivePrefix+"homedash.db"`; the plan's `["homedash"]` key does not exist.
- Contract-conforming addition: `[]` never `null` for any array (`homeDashNonNil`).
- Contract-conforming addition: malformed entity id in a command body returns `400 bad_request`.
- Contract-conforming addition: principal is the session nonce only when `auth.enabled`, otherwise `local-desktop` (the plan took any cookie).
- Contract-conforming addition: `Subscriber.Set` (M0 core) drops duplicates silently, also when rejected, and publishes one snapshot batch per call.
- Gap (a), NOT fixed (lead decision): on shutdown the HTTP drain cancels request contexts before `closeHomeDash`, so open streams end as dropped connections without `revoked {"reason":"shutdown"}`. Client reconnect rules treat both the same.
- Gap (b), NOT fixed: the per-frame 30 s write deadline is ineffective in production: `statusRecorder` and the gzip wrapper hide `SetWriteDeadline` (ErrNotSupported is tolerated), and the outer `httpstream.WithWriteTimeout` renews a 40-minute budget per streaming write. A client that stops reading is dropped only after up to 40 min. Same for the Docker Manager stream (security review, Medium).
- Gap (c), NOT fixed: `entities`/`entity` may answer partial catalogs when the 9 s request wait ends; the hub's 8 s per-source timeout reports `catalog_failed` in `sources` first. A history source timeout is `502 source_error`, not 504.
- Minor review findings, not fixed: the system adapter reads the sample outside its mutex (a concurrent Catalog may hold a newer state; self-heals next poll); history shows a one-point gap after a counter reset while the live rate keeps its last value.
- Test-only: the revoked-on-disable test calls `replaceConfigSnapshot` under CfgMu (write); the end-to-end test collects snapshot frames until six entities arrived.

## Contract differences from design-backend section 6 (12)

1. `POST dashboards` also takes `{"template":"system","name"?}`; `bootstrap` adds `templates`, `limits.widgets`, `limits.history_points`, `limits.history_range_seconds`.
2. Heartbeat is an `hb` event, not a `: hb` comment (client watchdog).
3. `snapshot` covers only newly added ids and carries `sources`.
4. `dashboard_changed` collapses within a flush window; a delete sends revision 0.
5. Missing snapshot capability is `400 unsupported_command`.
6. New HTTP codes: `not_found`, `method_not_allowed`, `body_too_large`, `storage_unavailable`, `timeout`, `internal_error`; core codes exposed: `slug_taken`, `revision_not_found`, `hub_closed`.
7. Shared-layer errors keep their bodies without `status`.
8. `GET dashboard` of a newer schema answers 200 with `schema_too_new:true`.
9. `request_id` is required on commands; `PUT dashboard` needs `slug` in the document.
10. Rejection reasons are M0's (`invalid_id|unknown_source|source_disabled|forbidden|limit`).
11. Desktop `revoked` reasons are only `shutdown`, `disabled`, `auth_lost`.
12. Read classification of `PUT stream/subscriptions` and `POST dashboards/validate` lives in the route table, not in `desktopRequestOperation`.

## Contradictions resolved by the plan (16)

Self-review resolutions, in short: system sample extended with network counters and boot time; per-route read classification (CYD precedent); `hb` event; `PrefetchCatalogs` before snapshots; atomic dashboard cap; service keeps its own coalesce interval; `dashboard_changed` reconciled by list; `unsupported_command` for system snapshots; smaller `MaxBytesReader` for JSON; DELETE added to a dedicated contract; `Requires` via `FilterAvailableApps`; capability vs API gate (egg mode 503); shared-layer error bodies documented; `source_unavailable` reserved for M3; design example ids made valid base32; agent facade deferred to M7.

## Open items for M3 and later

- HA adapter registers a row in `homedash_sources.go`; `source_unavailable` with `source` (first needed in M3).
- Kiosk routes and kiosk API; agent facade (M7).
- Server-wide stream write deadline: `Unwrap` on `statusRecorder`/`gzipResponseWriter`, and an `httpstream` rule that keeps a stricter handler deadline.
- Optional `revoked {"reason":"shutdown"}` before the drain.

## Not verified

- Browser/UI wiring (frontend track); live run against a real server instance.
- GitNexus index predates the new files: detect-changes partial (19 expected touched symbols, 26 new files unmapped); impact for new symbols checked by grep.
