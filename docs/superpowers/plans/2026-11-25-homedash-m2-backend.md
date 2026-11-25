# HomeDash M2 Backend Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Turn the M0 HomeDash core into a live desktop backend: server wiring with hot reload, the dashboards API with server-side save pipeline and starter template, the entity, history, snapshot and command routes, the SSE stream with `PUT …/stream/subscriptions`, and the read-only `system` source, so that the M2 frontend (F1/F2) can list, create and show a live dashboard.

**Architecture:** `internal/server` builds one `homeDashService` (hub + `homedash.db` store + catalog) at start on the server lifetime context, publishes it through an atomic pointer, reconfigures it from `replaceConfigSnapshot` without blocking, and closes it after the HTTP drain. One handler serves `/api/desktop/homedash/` through a route table that classifies every endpoint as read (allowed on a read-only desktop) or write (`checkDesktopOperation(desktopWrite)`). The stream follows the Docker Manager pattern (hello frame, per-write deadline, heartbeat) with coalesced frames drained from `homedash.Subscriber`. The `system` adapter lives in `internal/homedash/sources/system` and reads an extended copy of the server's existing 10 s metrics sample, so there is no second sampler.

**Tech Stack:** Go 1.27 (module `aurago`), standard library `net/http`, `httptest`, `modernc.org/sqlite` through the existing store, `internal/homedash` and `internal/homedash/sourcetest` from M0. No new module.

The code in this plan was written without compiling. Implementers may fix compile errors (imports, typos, small signature slips), but must not change the API contract below or any exported name without reporting it to the lead.

---

## Lead amendments (2026-11-25)

These override the task bodies wherever they say otherwise. Every dispatch of the named tasks includes them.

**A1. `system` history from server start (Tasks 2, 3, 11).** A dashboard opened for the first time must show
filled sparklines, not empty ones that grow one point per minute.
- Task 3 additionally keeps a bounded in-memory ring of the collector's samples in the server
  (`dashboard_metrics_snapshot.go` or a new `homedash_metrics_ring.go`): one entry per collector tick (10 s) with
  time, cpu %, memory %, disk %, net receive and send rates in Mbit/s (rates only once two samples exist); capacity
  8640 entries (24 h), oldest dropped first, guarded by a mutex, no allocation per read beyond the result slice.
  It exposes `func(from, to time.Time) []<sample>` (copy of the entries in range, oldest first).
- Task 2: the `system` adapter implements `homedash.HistorySource` (`internal/homedash/source.go:32`):
  `History(ctx, native, from, to, points)` reads that function, keeps the entries of the requested metric inside
  `[from, to]`, averages them into at most `points` equal time buckets (empty buckets are skipped, never zero-filled),
  oldest first. `system:uptime` returns an empty slice. The adapter receives the function through its constructor,
  like the snapshot function; tests use a fake ring.
- Tests: ring capacity and order, rate computation across two samples, adapter bucketing (range filter, averaging,
  `points` cap, empty input), and in Task 11 the starter dashboard's `history?id=system:net_rx` answering
  `origin:"source"` with points after two collector ticks of the fake.
- The contract section "`system` entities" states this; the route example of `history` stays generic.
- Exact surface (so the tasks fit together): in `internal/homedash/sources/system`, `type HistoryFunc func(from, to
  time.Time) []HistoryPoint` with `HistoryPoint{T time.Time; CPU, Memory, Disk float64; NetRx, NetTx *float64}`
  (rates nil until two samples exist); `New(snap SnapshotFunc, history HistoryFunc, pollSeconds int, now func()
  time.Time)` and `Factory(snap SnapshotFunc, history HistoryFunc)` gain the `history` parameter (every existing call
  in Tasks 2, 4 and 11 passes it) (nil means "no history": `History`
  returns an empty slice); `func (s *Source) History(ctx context.Context, native string, from, to time.Time, points
  int) ([]homedash.Sample, error)`. In `internal/server`, the ring is `homeDashMetricsRing` (new file
  `homedash_metrics_ring.go`) with `append(HistoryPoint)` called from the collector right after
  `storeSystemMetricsSnapshot` (rates computed there, so the adapter never duplicates the rate logic of
  `advanceLocked`), and the accessor `s.homeDashSystemHistory(from, to time.Time) []system.HistoryPoint`; Task 4 passes
  it as `system.Factory(s.homeDashSystemSnapshot, s.homeDashSystemHistory)`.
- Task 11's end-to-end test: build the adapter with a fake snapshot AND a fake history function fed by the test;
  after two ticks assert `history?id=system:net_rx` answers `origin:"source"`, no `since`, and `points` non-empty. The
  current assertion `origin == "memory"` is wrong and is replaced.

**A2. Task 5, no lost reconfigure at start.** After `s.homeDash.Store(svc)` in `initHomeDash`, call
`svc.reconfigure(s.ConfigSnapshot())` once, so a config change that arrived between reading the snapshot and
publishing the service is not dropped.

**A3. Task 6, stream protocol constant.** Declare `homeDashStreamProtocol = 1` in Task 6's const block (bootstrap and
its test use it) and do not declare it again in Task 10.

## File structure

Created:
- `internal/homedash/hub_lookup.go` — `Hub.PrefetchCatalogs`, `Hub.LookupEntities`.
- `internal/homedash/hub_lookup_test.go` — tests of both.
- `internal/homedash/store_limit_test.go` — dashboard cap is safe to change while saving.
- `internal/homedash/sources/system/system.go` — the `system` source adapter.
- `internal/homedash/sources/system/system_test.go` — adapter tests against a fake snapshot function.
- `internal/server/homedash_metrics_test.go` — extended metrics sample and host boot time.
- `internal/server/homedash_sources.go` — source spec table, `system` row, server snapshot mapping.
- `internal/server/homedash_sources_test.go`
- `internal/server/homedash_service.go` — service, deps, RuntimeConfig builder, init/close, audit binding.
- `internal/server/homedash_service_test.go`
- `internal/server/homedash_fixture_test.go` — shared test fixture for all HomeDash server tests.
- `internal/server/homedash_routes.go` — route table, dispatch, auth classification, envelope, errors, decode, principal, bootstrap, widget catalog.
- `internal/server/homedash_routes_test.go`
- `internal/server/homedash_template.go` — starter template builder (`system`).
- `internal/server/homedash_api_dashboards.go` — save pipeline and every dashboard route.
- `internal/server/homedash_api_dashboards_test.go`
- `internal/server/homedash_api_entities.go` — entities, entity, history, snapshot.
- `internal/server/homedash_api_entities_test.go`
- `internal/server/homedash_api_commands.go` — commands route.
- `internal/server/homedash_api_commands_test.go`
- `internal/server/homedash_stream.go` — SSE stream and subscriptions.
- `internal/server/homedash_stream_test.go`
- `internal/server/homedash_system_e2e_test.go` — starter dashboard live from the real adapter.
- `internal/audit/homedash_manifest_test.go`
- `docs/superpowers/specs/2026-11-23-homedash/m2-handover-backend.md` — hand-over notes (force-add).

Modified:
- `internal/homedash/store.go` — `maxDashboards` becomes `atomic.Int64`.
- `internal/homedash/store_dashboards.go` — `CreateDashboard` reads the atomic cap.
- `internal/server/dashboard_metrics_snapshot.go` — network counters, host boot time store/load.
- `internal/server/server_routes.go` — collector stores the boot time; mux registers `/api/desktop/homedash/`.
- `internal/server/server.go` — two `Server` fields, `replaceConfigSnapshot` hook, `Start` init/close.
- `internal/server/bluetooth_live.go` — capability `homedash`.
- `internal/audit/audit_manifests.go` — route contract `/api/desktop/homedash/`.
- `internal/homedash/AGENTS.md` — server wiring, `sources/system`, new hub methods.
- `internal/server/AGENTS.md` — new section "HomeDash API".

Not touched (owned by the frontend plan): root `AGENTS.md` HomeDash row (it already points to `internal/server/AGENTS.md`), `internal/desktop` `BuiltinApps` row, everything under `ui/`.

---

## API contract (frozen for M2)

Base path: `/api/desktop/homedash/` (the frontend's `hd-api.js` takes it from the host). Every route below is relative to it. Routes of later milestones (`kiosk/*`, `mqtt-entities`, the kiosk API under `/api/homedash/kiosk/`) do not exist in M2 and answer `404 not_found`.

### General rules

- **Auth.** A browser session, or `Authorization: Bearer <token>` with scope `desktop:admin`. When login is disabled every local request is the admin. A bearer token with any other scope is refused.
- **Classes.** `read` routes work on a read-only desktop (`virtual_desktop.readonly`). `write` routes answer `403 desktop_readonly` on a read-only desktop. Session requests other than GET need a same-origin `Origin` header (checked by the shared auth middleware, `403 csrf_check_failed`).
- **Availability.** Every route answers `503 homedash_disabled` when `homedash.enabled` is false, egg mode is on or `virtual_desktop.enabled` is false; `503 storage_unavailable` when `homedash.db` could not be opened at start.
- **Order of checks.** auth → route (`404 not_found`) → method (`405 method_not_allowed`, `Allow` header) → availability (503) → desktop read-only for `write` routes (403) → handler.
- **Envelope.** Success: `{"status":"ok", …fields}`. HomeDash error: `{"status":"error","code":"<code>","message":"<English>", …extra}`. Errors produced by shared layers have no `status` field: the auth middleware answers `401 {"error":"unauthorized","redirect":"/auth/login"}`, `403 {"error":"invalid_bearer_scope"}` and `403 {"error":"csrf_check_failed","message":"…"}`; the desktop scope check answers `403 {"error":"desktop_scope_required","message":"…"}`; the desktop operation gate answers `403 {"error":"desktop_readonly","code":"desktop_readonly","message":"…"}`. **Clients key on HTTP status plus `code`, falling back to `error` when `code` is absent.**
- **Headers.** Every answer carries `Cache-Control: no-store`. JSON answers are `application/json; charset=utf-8`. Key order inside JSON objects is not significant.
- **Times** are RFC 3339 strings in UTC (`Z`), possibly with fractional seconds. Query times are parsed with RFC 3339 (fractional seconds allowed).
- **Entity ids** are `<source>:<native>` and travel only in query strings (`?id=`) or JSON bodies, never in path segments. URL-encode them.
- **Body limits.** `dashboards` (POST), `dashboard` (PUT), `dashboards/validate`: 512 KiB. `stream/subscriptions`: 256 KiB. Every other body: 64 KiB. Over the limit: `413 body_too_large`. Malformed JSON, an empty required body or a wrong JSON shape: `400 bad_request`. Unknown top-level request fields are ignored; the dashboard `document` itself is decoded strictly (unknown fields are a `422 invalid_document` issue with code `bad_json`).

### Routes

| Method | Path | Class | Purpose |
|---|---|---|---|
| GET | `bootstrap` | read | versions, flags, limits, templates, source statuses |
| GET | `widget-catalog` | read | widget catalog (ETag) |
| GET | `entities?source=&kind=&area=&q=&limit=&offset=` | read | entity catalog with current state |
| GET | `entity?id=` | read | one entity |
| GET | `history?id=&from=&to=&points=` | read | numeric series |
| GET | `snapshot?id=&w=&h=` | read | camera image (no M2 source has one) |
| POST | `commands` | write | device command (no M2 source accepts one) |
| GET | `stream` | read | SSE stream |
| PUT | `stream/subscriptions` | read | replace the stream's subscription |
| GET | `dashboards` | read | dashboard summaries |
| POST | `dashboards` | write | create from a document or a template |
| POST | `dashboards/validate` | read | validation report, nothing saved |
| GET | `dashboard?id=` | read | one dashboard |
| PUT | `dashboard?id=` | write | replace (optimistic revision) |
| DELETE | `dashboard?id=&revision=` | write | delete |
| POST | `dashboard/duplicate?id=` | write | copy as a new dashboard |
| GET | `dashboard/revisions?id=` | read | kept revisions, newest first |
| POST | `dashboard/restore?id=&revision=` | write | save an old revision as the next one |
| POST | `dashboard/approve?id=&revision=` | write | set `approved_revision` |

Query `id` of dashboard routes must match `^d_[a-z2-7]{12}$`, `revision` must be an integer ≥ 1; otherwise `400 bad_request`. Query `id` of entity routes must be a valid entity id; otherwise `400 bad_request`.

### Shared JSON types

**Entity** (in `entities`, `entity`, stream `snapshot` and `entity` frames):

```json
{"id":"system:cpu","source":"system","kind":"sensor","name":"CPU","unit":"%","icon_hint":"cpu",
 "capabilities":[{"name":"numeric","allowed":true},{"name":"history","allowed":true}],
 "readonly":true,
 "state":{"value":12.3,"available":true,"updated_at":"2026-11-25T10:00:00Z"}}
```

Fields: `id`, `source`, `kind` (one of the catalog `kinds`), `name` (external text, ≤ 120 runes, render with `textContent`), optional `area`, `area_id`, `device_class`, `unit`, `icon_hint` (a hint only; unknown icons fall back to the kind icon), `capabilities` (always an array; command capabilities carry `allowed`, optional `blocked_by`, `sensitive`, `min`, `max`, `step`, `options`; read capabilities `stream|snapshot|history|numeric` always have `allowed:true`), `readonly` (no allowed command), optional `readonly_reason` (a config key such as `virtual_desktop.readonly`), optional `media`, `state`.

A subscribed id whose source is enabled but whose catalog does not (yet) contain it appears as a placeholder: `{"id":"system:foo","source":"system","kind":"generic","name":"foo","capabilities":[],"readonly":true,"state":{"value":null,"available":false,"updated_at":"…"}}`. A later `entity` frame replaces it when the catalog has it.

**State**: `{"value": number|string|boolean|null, "text"?: string, "attributes"?: object, "available": bool, "stale"?: true, "updated_at": time}`. `available:false` = offline (show the offline look; there is no meaningful value). `stale:true` = the value is old (show the stale look, keep the value). The host computes alert tones from `value` and widget options (decision D5); the server sends no tone.

**SourceStatus**: `{"source":"system","state":"idle|starting|live|polling|degraded|unavailable|disabled","reason"?:"<code>","at":time}`. `system` reasons: `no_metrics` (with `unavailable`), `stale_metrics` (with `degraded`); hub reasons `crashed`, `stopped`, `catalog_failed`.

**DashboardSummary**: `{"id","slug","name","icon"?,"revision","approved_revision","updated_at","saved_by":"user"|"agent"}`.

**Dashboard**: the document of design section 4.2 with server-owned `id`, `revision`, `approved_revision`, `updated_at`, `saved_by`. Complete example in the template answer below. Rules the client must know: `slug` is required on PUT (keep the one you got); on create an empty `slug` is derived from the name and made unique (`living-room`, `living-room-2`, …); absent `look`, `grid`, `kiosk`, widget `actions`, page `compact` take defaults; `layouts.lg` items missing for a widget are placed by the server (Autopack); `md`/`sm` are never filled by the server.

**Issue** (in `issues` and `warnings`): `{"path":"/pages/0/widgets/2/bindings/primary","code":"binding_required","message":"…"}`. Issues block a save; warnings (`entity_unknown`, `kind_mismatch`, `capability_missing`) never do.

### `system` entities (decision D7)

| id | name | kind | unit | device_class | value | notes |
|---|---|---|---|---|---|---|
| `system:cpu` | CPU | sensor | `%` | – | 0–100, 0.1 steps | |
| `system:memory` | Memory | sensor | `%` | – | 0–100, 0.1 steps | |
| `system:disk` | Disk | sensor | `%` | – | 0–100, 0.1 steps | disk of the data directory's volume as the collector measures it |
| `system:net_rx` | Network receive | sensor | `Mbit/s` | `data_rate` | ≥ 0, 0.001 steps | unavailable until two samples were seen |
| `system:net_tx` | Network send | sensor | `Mbit/s` | `data_rate` | ≥ 0, 0.001 steps | same |
| `system:uptime` | Uptime | sensor | `s` | `duration` | whole seconds | host uptime; format as a duration in the UI; never stale |

All six: capabilities `numeric` and `history`, `readonly:true`, no command. Values change every 10 s (collector period); the source polls every `homedash.sources.system.poll_seconds` (default 5 s) while subscribed and marks values `stale` when the sample is older than max(3 × poll, 30 s). History (lead amendment A1): the `system` source serves its own history (`origin:"source"`) from a server-side ring of the collector's samples, one point per collector tick (10 s), the last 24 h since server start, independent of subscriptions; `system:uptime` has no history (`points: []`). The hub's in-memory ring is not used for `system`.

### Route examples

**GET `bootstrap`** → 200

```json
{"status":"ok","api_version":1,"stream_protocol":1,"schema_version":1,
 "catalog_hash":"ee1d3b09a2c21f7f0096bbac5c51ca48b7952358b2b4264b055d23351f6066d0",
 "readonly":false,"desktop_readonly":false,"kiosk_enabled":false,
 "limits":{"dashboards":50,"subscriptions":300,"pages":12,"widgets_per_page":120,"widgets":400,
           "document_bytes":262144,"history_points":500,"history_range_seconds":604800},
 "templates":["system"],
 "sources":[{"source":"system","state":"idle","at":"2026-11-25T09:58:00Z"}]}
```

`readonly` = `homedash.readonly` (commands refused); `desktop_readonly` = writes refused (hide create/edit/delete). `catalog_hash` equals the `hash` of `widget-catalog`.

**GET `widget-catalog`** → 200 with header `ETag: "<hash>"`; a request with `If-None-Match` containing the hash → `304` without body.

```json
{"status":"ok","catalog":{"version":1,"hash":"ee1d3b09…","schema_version":1,"grid":{…},"limits":{…},
 "families":["glass","tactile","minimal","instrument"],"kinds":[…],"capabilities":[…],
 "palette_tokens":[…],"common_options":[…],"widgets":[…]}}
```

`catalog` is exactly the content of `internal/homedash/testdata/widget_catalog.json` (the golden file the frontend contract test already reads).

**GET `entities?kind=sensor&limit=50`** → 200

```json
{"status":"ok","entities":[{"id":"system:cpu","source":"system","kind":"sensor","name":"CPU","unit":"%","icon_hint":"cpu","capabilities":[{"name":"numeric","allowed":true},{"name":"history","allowed":true}],"readonly":true,"state":{"value":12.3,"available":true,"updated_at":"2026-11-25T10:00:00Z"}}],
 "total":6,"limit":50,"offset":0,
 "sources":[{"source":"system","state":"polling","at":"2026-11-25T10:00:01Z"}]}
```

Query: `source` (a source id), `kind` (a catalog kind), `area` (case-insensitive match on area or area_id, ≤ 80 runes), `q` (case-insensitive substring of name, id or area, ≤ 100 runes), `limit` 1–500 (default 100), `offset` ≥ 0 (default 0). Out of range → `400 bad_request`. Sorted by name (case-insensitive), then id. The call keeps the listed sources running for 30 s (catalog warm-up).

**GET `entity?id=system:cpu`** → 200 `{"status":"ok","entity":{…Entity…}}`; unknown → `404 entity_not_found`.

**GET `history?id=system:cpu&from=2026-11-25T09:00:00Z&to=2026-11-25T10:00:00Z&points=60`** → 200

```json
{"status":"ok","id":"system:cpu","origin":"memory","since":"2026-11-25T08:12:03Z",
 "points":[{"t":"2026-11-25T09:58:40Z","v":11.8},{"t":"2026-11-25T09:59:50Z","v":12.3}]}
```

Defaults: `to` = now, `from` = `to` − 24 h, `points` = 120. Rules: `to` after `from`, range ≤ 7 days, `points` 1–500, else `400 bad_request`. `since` (process start of the ring) is present only for `origin:"memory"`; `origin:"source"` series come from the source's own history (in M2 only `system`, lead amendment A1; no `since`). `points` is always an array, oldest first, averaged down to at most `points`.

**GET `snapshot?id=…&w=640&h=360`** → 200 binary image (`Content-Type` `image/jpeg|png|webp`, `X-Content-Type-Options: nosniff`). `w` 1–1920 (default 640), `h` 1–1080 (default 360). In M2 every entity answers `400 unsupported_command` (`system` has no snapshot).

**POST `commands`** → 202

Request: `{"entity":"ha:light.kitchen","command":"brightness","args":{"value":40},"request_id":"5f0c2a1e-…","dashboard_id":"d_m3x7q2k5a4bc","widget_id":"w3","confirmed":false}`. `entity`, `command` and `request_id` (1–64 characters, a new one per user action; a retry with the same id within 30 s returns the first result) are required; `args` defaults to `{}`; `dashboard_id`/`widget_id` are optional but must be valid ids when present. Argument names per capability: design-backend section 6 table.

Answer: `{"status":"ok","accepted":true,"superseded":false,"expected":{"value":true,"text":"on","attributes":{"brightness_pct":40},"available":true,"updated_at":"…"}}` (`expected` omitted when the source predicts nothing; `superseded:true` when a newer value for the same control replaced this one before it was sent). The real state follows on the stream. In M2 the `system` entities answer `400 unsupported_command`; on a read-only desktop every command answers `403 desktop_readonly`.

**GET `stream`** → `200 text/event-stream` (see Stream protocol). Before the stream starts it can answer `429 too_many_streams` or `503`.

**PUT `stream/subscriptions`** → 200

Request: `{"stream_id":"9f2c4b7e1a3d5f60718293a4b5c6d7e8","entities":["system:cpu","system:memory","bad id","nope:x"]}` (full replacement; `[]` or a missing list unsubscribes everything).
Answer: `{"status":"ok","accepted":["system:cpu","system:memory"],"rejected":[{"id":"bad id","reason":"invalid_id"},{"id":"nope:x","reason":"unknown_source"}]}`.
Rejection reasons: `invalid_id`, `unknown_source`, `source_disabled`, `forbidden`, `limit` (more than `limits.subscriptions` ids). Duplicates are dropped silently. Unknown or closed stream (or a stream opened by another principal): `404 stream_not_found`.

**GET `dashboards`** → 200 `{"status":"ok","dashboards":[{"id":"d_m3x7q2k5a4bc","slug":"system","name":"System","revision":1,"approved_revision":1,"updated_at":"2026-11-25T10:00:00.123456789Z","saved_by":"user"}]}` (sorted by name, then id; `[]` when empty).

**POST `dashboards`** — exactly one of two forms, else `400 bad_request`:

1. Document: `{"document":{"name":"My dashboard","pages":[{"id":"main","name":"Main"}]}}` (the "Empty dashboard" button sends exactly this with a localized name). → 201 `{"status":"ok","dashboard":{…saved document, slug "my-dashboard"…},"warnings":[]}`.
2. Template (decision D3): `{"template":"system","name":"System"}` (`name` optional, default `"System"`; any other template → `400 bad_request`). → 201, answer below.

Errors: `422 invalid_document` (+`issues`), `422 limit_exceeded` (dashboard count at `limits.dashboards`, or document > 256 KiB), `409 slug_taken` (an explicit `slug` already used), `409 schema_too_new` (`schema_version` above 1).

**Starter template answer** (POST `{"template":"system"}` → 201):

```json
{"status":"ok","warnings":[],
 "dashboard":{
  "schema_version":1,"id":"d_m3x7q2k5a4bc","slug":"system","name":"System",
  "revision":1,"approved_revision":1,"updated_at":"2026-11-25T10:00:00.123456789Z","saved_by":"user",
  "look":{"family":"glass","palette":{"mode":"theme"},"backdrop":{"kind":"theme"},"density":"comfortable","radius_scale":1},
  "grid":{"row_unit":"auto","gap":null,"fit":"scroll"},
  "pages":[{"id":"main","name":"Main","compact":true,
   "widgets":[
    {"id":"cpu","type":"gauge","variant":"arc","bindings":{"primary":"system:cpu"},
     "options":{"alert":95,"decimals":0,"max":100,"min":0,"warn":80},
     "actions":{"tap":{"do":"default"},"hold":{"do":"more_info"}},"confirm":false},
    {"id":"memory","type":"gauge","variant":"needle","bindings":{"primary":"system:memory"},
     "options":{"alert":95,"decimals":0,"max":100,"min":0,"warn":85},
     "actions":{"tap":{"do":"default"},"hold":{"do":"more_info"}},"confirm":false},
    {"id":"disk","type":"gauge","variant":"arc","bindings":{"primary":"system:disk"},
     "options":{"alert":95,"decimals":0,"max":100,"min":0,"warn":85},
     "actions":{"tap":{"do":"default"},"hold":{"do":"more_info"}},"confirm":false},
    {"id":"net-rx","type":"sensor","variant":"sparkline","bindings":{"primary":"system:net_rx"},
     "options":{"decimals":1,"hours":1},
     "actions":{"tap":{"do":"default"},"hold":{"do":"more_info"}},"confirm":false},
    {"id":"net-tx","type":"sensor","variant":"sparkline","bindings":{"primary":"system:net_tx"},
     "options":{"decimals":1,"hours":1},
     "actions":{"tap":{"do":"default"},"hold":{"do":"more_info"}},"confirm":false},
    {"id":"uptime","type":"entity","variant":"tile","bindings":{"primary":"system:uptime"},
     "options":{},
     "actions":{"tap":{"do":"default"},"hold":{"do":"more_info"}},"confirm":false}],
   "layouts":{"lg":[
    {"id":"cpu","x":0,"y":0,"w":4,"h":4},{"id":"memory","x":4,"y":0,"w":4,"h":4},{"id":"disk","x":8,"y":0,"w":4,"h":4},
    {"id":"net-rx","x":0,"y":4,"w":5,"h":2},{"id":"net-tx","x":5,"y":4,"w":5,"h":2},{"id":"uptime","x":10,"y":4,"w":2,"h":2}]}}],
  "kiosk":{"start_page":"main","idle_dim_seconds":120,"idle_dim_level":0.3,"page_rotation_seconds":0,"screen_saver":"clock","show_clock":true}}}
```

A second template creation gets slug `system-2`. `warnings` lists `entity_unknown` per binding when the `system` source is switched off.

**GET `dashboard?id=`** → 200 `{"status":"ok","dashboard":{…},"schema_too_new":false}`. A document written by a newer schema still answers 200 with `"schema_too_new":true`; show it read-only (its PUT answers `409 schema_too_new`). Unknown → `404 dashboard_not_found`.

**PUT `dashboard?id=`** → 200. Request `{"revision":7,"document":{…full document, slug included…}}`. Answer `{"status":"ok","dashboard":{…,"revision":8,"approved_revision":8},"warnings":[]}` (a user save approves). Errors: `409 revision_conflict` with `"current_revision":9`, `422 invalid_document` with `issues`, `409 slug_taken`, `409 schema_too_new`, `404 dashboard_not_found`, `422 limit_exceeded`.

**DELETE `dashboard?id=&revision=7`** → 200 `{"status":"ok","id":"d_m3x7q2k5a4bc"}`; stale → `409 revision_conflict` (+`current_revision`); unknown → `404`.

**POST `dashboard/duplicate?id=`** with optional body `{"name":"Living room 2"}` → 201 `{"status":"ok","dashboard":{…new id, revision 1, slug "<slug>-copy" made unique, name = body name or "<name> (copy)"…},"warnings":[]}`.

**GET `dashboard/revisions?id=`** → 200 `{"status":"ok","revisions":[{"revision":3,"saved_at":"2026-11-25T10:05:00Z","saved_by":"user"},{"revision":2,"saved_at":"…","saved_by":"user"}]}` (the last 20 plus the approved one).

**POST `dashboard/restore?id=&revision=2`** → 200 `{"status":"ok","dashboard":{…content of revision 2 saved as the next revision…},"warnings":[]}`; unknown revision → `404 revision_not_found`; the restored content runs the full save pipeline (`422`, `409 slug_taken` possible).

**POST `dashboard/approve?id=&revision=2`** → 200 `{"status":"ok","dashboard":{…,"approved_revision":2}}`; unknown revision → `404 revision_not_found`.

**POST `dashboards/validate`** → 200. Request `{"document":{…}}`. Answer `{"status":"ok","valid":false,"issues":[{"path":"/pages","code":"required","message":"at least one page"}],"warnings":[],"document":{…normalized and auto-packed document…}}`. An empty `slug` is derived from the name (not checked for uniqueness). Undecodable document → `422 invalid_document` (issue code `bad_json`); `schema_version` above 1 → `409 schema_too_new`. Works on a read-only desktop.

### Error codes

| HTTP | code | where |
|---|---|---|
| 400 | `bad_request` | any route: malformed query, body or id |
| 400 | `invalid_args` | `commands` |
| 400 | `unsupported_command` | `commands` (entity lacks the capability); `snapshot` (no snapshot) |
| 401 | `unauthorized` (in `error`) | auth middleware, no session |
| 403 | `invalid_bearer_scope` (in `error`) | auth middleware |
| 403 | `desktop_scope_required` (in `error`) | bearer without `desktop:admin` |
| 403 | `csrf_check_failed` (in `error`) | session write with a foreign `Origin` |
| 403 | `desktop_readonly` | every `write` route on a read-only desktop |
| 403 | `read_only` (+`blocked_by`) | `commands` (`homedash.readonly`, `virtual_desktop.readonly`, `homedash.sources.<key>.control`, integration gates) |
| 403 | `forbidden_entity` (+`blocked_by`) | `commands` (agent/kiosk principals; not reachable from desktop sessions) |
| 404 | `not_found` | unknown route |
| 404 | `dashboard_not_found`, `revision_not_found` | dashboard routes |
| 404 | `entity_not_found` | `entity`, `history`, `snapshot`, `commands` |
| 404 | `stream_not_found` | `stream/subscriptions` |
| 405 | `method_not_allowed` (+`Allow` header) | any route |
| 409 | `revision_conflict` (+`current_revision`) | `dashboard` PUT/DELETE |
| 409 | `slug_taken`, `schema_too_new` | dashboard writes; `schema_too_new` also on `validate` |
| 409 | `confirmation_required` | `commands` (sensitive capability without `confirmed:true`) |
| 413 | `body_too_large` | routes with a body |
| 422 | `invalid_document` (+`issues`) | dashboard writes, `validate` (undecodable only) |
| 422 | `limit_exceeded` | dashboard writes |
| 429 | `rate_limited` (+`retry_after_ms`, `Retry-After` header) | `commands` |
| 429 | `too_many_streams` | `stream` |
| 500 | `internal_error` | unexpected failure (logged, no detail) |
| 502 | `command_failed`, `source_error` | `commands`, `history`, `snapshot` |
| 503 | `homedash_disabled`, `storage_unavailable`, `hub_closed` | any route |
| 504 | `timeout` | the request context ended before the work finished |
| 503 | `source_unavailable` (+`source`) | reserved for M3+, never sent in M2 |

### Stream protocol

`GET stream` answers `200` with `Content-Type: text/event-stream`, `Cache-Control: no-store`, `X-Accel-Buffering: no`. Each frame is `event: <name>\ndata: <one line of compact JSON>\n\n`; frames carry no `id:`; every payload passes the server's secret scrubber. The server arms a 30 s write deadline before every frame; a client that stops reading is dropped.

Frames, in the order a client sees them:

1. `hello` (once, first):
   `{"stream_id":"9f2c4b7e1a3d5f60718293a4b5c6d7e8","protocol":1,"heartbeat_ms":15000,"limits":{"entities":300},"server_time":"2026-11-25T10:00:00.000000123Z","principal":"session"}`
2. The client sends `PUT stream/subscriptions` with that `stream_id` (the union of all HomeDash windows of the JS realm).
3. `snapshot` — after every accepted PUT that **adds** ids, for the added ids only (ids already subscribed get none; a PUT that only removes ids produces no snapshot). It may arrive before the PUT answer.
   `{"entities":[{"id":"system:cpu","source":"system","kind":"sensor","name":"CPU","unit":"%","icon_hint":"cpu","capabilities":[{"name":"numeric","allowed":true},{"name":"history","allowed":true}],"readonly":true,"state":{"value":12.3,"available":true,"updated_at":"2026-11-25T10:00:00Z"}}],"sources":[{"source":"system","state":"starting","at":"2026-11-25T10:00:01Z"}]}`
   `sources` is always an array (possibly empty) of statuses changed since the last flush.
4. `entity` — meta, capability or gate changes of subscribed ids (also the first real catalog entry of a placeholder):
   `{"entities":[{…Entity…}]}`
5. `removed` — subscribed ids that left their source's catalog (still subscribed; show them as missing):
   `{"ids":["system:foo"]}`
6. `state` — coalesced, latest value per entity, at most one flush per 100 ms:
   `{"changes":{"system:cpu":{"value":14.1,"available":true,"updated_at":"2026-11-25T10:00:10Z"},"system:uptime":{"value":7215,"available":true,"updated_at":"2026-11-25T10:00:11Z"}}}`
7. `source` — one frame per changed status of a source the stream holds ids of (only in flushes without a `snapshot`):
   `{"source":"system","state":"polling","at":"2026-11-25T10:00:01Z"}`
8. `dashboard_changed` — any dashboard was created, saved, restored, approved or deleted (all desktop streams, whatever they subscribe):
   `{"id":"d_m3x7q2k5a4bc","revision":2,"approved_revision":2}`; a delete sends `"revision":0,"approved_revision":0`. Several changes inside one 100 ms window collapse into the last one, so on every `dashboard_changed` the client refetches `GET dashboards` and reconciles every open dashboard by `revision`.
9. `hb` — every 15 s: `{"server_time":"2026-11-25T10:00:15Z"}`.
10. `revoked` — last frame, then the server ends the response: `{"reason":"shutdown"|"disabled"|"auth_lost"}`.

Within one flush the order is `snapshot`, `entity`, `removed`, `state`, `source`, `dashboard_changed`, `revoked`. The server re-checks authorization and availability every 5 s (`auth_lost`, `disabled`); a read-only desktop does not end the stream.

**Reconnect rules (client):**
- No frame of any kind (including `hb`) for 45 s, or an EventSource `error`: call `close()` on the EventSource (never let it auto-reconnect: the stream id changes), show the stale banner, keep the last values, and reconnect with backoff 1 s, 2 s, 4 s, 8 s, 16 s, 30 s (cap), ±20 % jitter; reset the backoff after a stream lived 60 s.
- After each new `hello`, PUT the full subscription again and treat the `snapshot` as the truth for those ids.
- `revoked` `shutdown`: reconnect with backoff. `disabled`: stop and call `bootstrap` (503 `homedash_disabled` → show the disabled state). `auth_lost`: stop; the desktop shell handles the login.
- PUT answering `404 stream_not_found`: the stream is gone; close and reconnect. `GET stream` answering `429 too_many_streams`: back off at least 5 s.
- There is no `Last-Event-ID`; the protocol resumes from snapshots.

### Limits

Streams per server: `homedash.limits.max_streams` (default 32). Ids per stream: `homedash.limits.max_subscriptions` (default 300, sent in `hello.limits.entities`). Dashboards: `homedash.limits.max_dashboards` (default 50). Document ≤ 256 KiB, 12 pages, 120 widgets per page, 400 widgets (catalog `limits`). Entity list `limit` ≤ 500. History range ≤ 7 days, ≤ 500 points. `request_id` ≤ 64 characters. Commands: 60 per minute and 10 per 10 s per principal, 5 per 5 s per entity. Snapshot ≤ 8 MiB.

### Differences from design-backend section 6 (forced by M0 code, hand-over or lead decisions)

1. `POST dashboards` also accepts `{"template":"system","name"?}` (D3); `bootstrap` adds `templates`, `limits.widgets`, `limits.history_points`, `limits.history_range_seconds`.
2. Heartbeat is an `hb` event instead of the `: hb` comment, so the client can run a watchdog (EventSource hides comments).
3. `snapshot` covers only newly added ids and carries `sources` (M0 `Subscriber.Set`, `internal/homedash/subscriber.go:179-202`).
4. `dashboard_changed` collapses inside a flush window (single `Batch.Dashboard`, `subscriber.go:56,306-313`); a delete sends revisions 0.
5. A missing snapshot capability is `400 unsupported_command` (M0 `Hub.Snapshot`, `internal/homedash/history.go:233-240`), not a new code.
6. New HTTP-layer codes: `not_found`, `method_not_allowed`, `body_too_large`, `storage_unavailable`, `timeout`, `internal_error`; core codes exposed that the section did not list: `slug_taken` (409), `revision_not_found` (404), `hub_closed` (503).
7. Shared-layer errors (`unauthorized`, `invalid_bearer_scope`, `desktop_scope_required`, `csrf_check_failed`, `desktop_readonly`) keep their existing bodies without `status`.
8. `GET dashboard` of a newer schema answers 200 with `schema_too_new:true`.
9. `request_id` is required on commands; `PUT dashboard` needs `slug` in the document.
10. Rejection reasons are M0's (`invalid_id|unknown_source|source_disabled|forbidden|limit`).
11. Desktop `revoked` reasons are only `shutdown`, `disabled`, `auth_lost`.
12. Read classification of `PUT stream/subscriptions` and `POST dashboards/validate` lives in the HomeDash route table, not in `desktopRequestOperation` (CYD Creator precedent).

---

## Project rules (every task)

- **R1 Worktree.** Work only in the session worktree the app created; run every command there. The main checkout carries other sessions' uncommitted and git-ignored scratch files (`go build ./...` and `./internal/audit/...` fail there).
- **R2 GitNexus.** Before editing an EXISTING symbol run impact (MCP `impact({target:"<symbol>", direction:"upstream", repo:"AuraGo"})`, or CLI `node C:/Users/Andi/.gitnexus/maintenance/AuraGo/graph-alternate/run.cjs impact "<symbol>" --direction upstream --repo C:/Users/Andi/Documents/repo/AuraGo`), report callers and risk, warn on HIGH/CRITICAL and treat `UNKNOWN` as unresolved (confirm callers with Grep). Before each commit run detect-changes (MCP `detect_changes({scope:"all", repo:"AuraGo", worktree:"<worktree path>"})` or the CLI equivalent). The index is often stale or broken on this machine: when impact or detect-changes cannot answer, grep the callers and add the line `GitNexus unavailable; callers checked with grep: <symbols>` to the commit body.
- **R3 Commits.** One commit per task with only the task's paths: `git add <paths>` then `git commit -m "<message>"`. New `AGENTS.md` files and anything under `docs/` are git-ignored: `git add -f <path>` and confirm with `git ls-files <path>`. Never `git add -A`.
- **R4 Files.** LF line endings, no BOM. No new go.mod module. In Go source never write a literal backslash-u escape (the Write tool decodes it); use `\x5c` if ever needed.
- **R5 Tests.** Use the exact commands given. Group `-run` alternatives as `'(?i)(A|B)'` and count PASS lines with `-v` when a run is evidence. Environment: keep `GOTMPDIR` in backslash form on a short path (for example `C:\tmp\go-build`); if Go reports `no such tool "vet"`, prefix commands with `GOTOOLCHAIN=go1.27.4`; if D: is full, trim `D:\dev-cache\go-build` entries older than 3 days (1 day if that frees nothing).
- **R6 Scope.** Never touch `ui/`, the desktop app manifest, config UI or kiosk code; those are other plans.

---

## Task 0: Workspace

- [ ] Use the branch the app gave you; do not run `git worktree add`. Record `git rev-parse HEAD` and `git status --short` in the session scratchpad file `m2-backend-baseline.txt`.
- [ ] Confirm the base contains M0 and M1: `git log --oneline -1 be4f4cf9e` succeeds and `internal/homedash/hub.go` exists.
- [ ] Baseline (append output summaries to the same scratchpad file):
  - `go build ./...`
  - `go test ./internal/homedash/... ./internal/config/ ./internal/httpstream/ -count=1`
  - `go test ./internal/audit/... -count=1`
  - `go test ./internal/server -count=1 -run '(?i)(ChibiSensei|DockerManagerStream|DashboardMetrics|DesktopReadonly)'`
- [ ] Any failure: record test name and first error line; later tasks compare against this list. Do not fix unrelated failures.

**Done when:** the scratchpad file lists HEAD, status and the four baseline results.

---

### Task 1: Core additions for the HTTP layer

**Files:** create `internal/homedash/hub_lookup.go`, `internal/homedash/hub_lookup_test.go`, `internal/homedash/store_limit_test.go`; modify `internal/homedash/store.go`, `internal/homedash/store_dashboards.go`.

**Read first:** `internal/homedash/AGENTS.md`; `internal/homedash/hub.go:298-421` (`catalogStale`, `Entities`, `Entity`); `internal/homedash/hub_sources.go:58-62,279-348` (`catalogJobLocked`, `fetchCatalogs`); `internal/homedash/subscriber.go:124-210` (`Set` snapshots placeholders for uncataloged ids at `:189-193`); `internal/homedash/store.go:44-80`; `internal/homedash/store_dashboards.go:63-105`; `internal/homedash/hub_test.go:19-42` (`newTestHub`); `internal/homedash/store_test.go:15-46`.

Why: `Subscriber.Set` snapshots a placeholder when the catalog was never fetched, which would flash every widget "unavailable" on open; the server prefetches catalogs before `Set`. Validation needs the bound entities without warming sources. `Store.SetMaxDashboards` writes a plain int that `CreateDashboard` reads concurrently, so it cannot be called from config hot reload as it is.

- [ ] Write `internal/homedash/hub_lookup_test.go`:

```go
package homedash_test

import (
	"context"
	"testing"

	"aurago/internal/homedash"
	"aurago/internal/homedash/sourcetest"
)

func TestPrefetchCatalogsFetchesOncePerSourceWithoutStarting(t *testing.T) {
	a := sourcetest.New("fake", sourcetest.Entity("fake:a", homedash.KindSensor))
	other := sourcetest.New("other", sourcetest.Entity("other:x", homedash.KindSwitch))
	h := newTestHub(t, nil, a, other)
	ctx := context.Background()

	h.PrefetchCatalogs(ctx, []homedash.EntityID{"fake:a", "fake:b", "bad id", "nope:x"})
	if a.CatalogCalls() != 1 || other.CatalogCalls() != 0 {
		t.Fatalf("catalog calls fake=%d other=%d, want 1 and 0", a.CatalogCalls(), other.CatalogCalls())
	}
	if a.Runs() != 0 {
		t.Fatal("PrefetchCatalogs started the source")
	}
	h.PrefetchCatalogs(ctx, []homedash.EntityID{"fake:a"})
	if a.CatalogCalls() != 1 {
		t.Fatal("a fresh catalog was fetched again")
	}

	sub, err := h.Attach(homedash.Principal{Kind: homedash.PrincipalSession, Key: "k"})
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Close()
	if _, _, err := sub.Set([]homedash.EntityID{"fake:a"}); err != nil {
		t.Fatal(err)
	}
	b := sub.Drain()
	if len(b.Snapshot) != 1 || b.Snapshot[0].Kind != homedash.KindSensor || !b.Snapshot[0].State.Available {
		t.Fatalf("snapshot after prefetch = %+v, want the cataloged sensor", b.Snapshot)
	}
}

func TestLookupEntitiesReturnsCatalogedEntitiesWithGates(t *testing.T) {
	a := sourcetest.New("fake",
		sourcetest.Entity("fake:a", homedash.KindSwitch, homedash.CapabilitySpec{Name: homedash.CapToggle, Allowed: true}))
	h := newTestHub(t, func(c *homedash.RuntimeConfig) { c.ReadOnly = true }, a)

	got := h.LookupEntities(context.Background(), []homedash.EntityID{"fake:a", "fake:b", "bad id", "nope:x"})
	if len(got) != 1 {
		t.Fatalf("lookup = %+v, want only fake:a", got)
	}
	e := got["fake:a"]
	if e.Kind != homedash.KindSwitch || !e.ReadOnly || e.ReadOnlyReason != "homedash.readonly" {
		t.Fatalf("fake:a = %+v, want the switch with the homedash.readonly gate applied", e)
	}
	if a.Runs() != 0 {
		t.Fatal("LookupEntities started the source")
	}
}
```

- [ ] Write `internal/homedash/store_limit_test.go`:

```go
package homedash

import (
	"context"
	"fmt"
	"testing"
)

func TestStoreSetMaxDashboardsWhileCreating(t *testing.T) {
	st, _, _ := openTestStore(t)
	ctx := context.Background()
	st.SetMaxDashboards(1)
	if _, err := st.CreateDashboard(ctx, storeDoc("one"), SavedByUser); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateDashboard(ctx, storeDoc("two"), SavedByUser); code(err) != CodeLimitExceeded {
		t.Fatalf("second dashboard over a cap of 1: %v", err)
	}
	st.SetMaxDashboards(0) // ignored
	if _, err := st.CreateDashboard(ctx, storeDoc("three"), SavedByUser); code(err) != CodeLimitExceeded {
		t.Fatalf("cap 0 must be ignored: %v", err)
	}
	// Hot reload changes the cap while requests save (meaningful under -race).
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 200; i++ {
			st.SetMaxDashboards(10 + i%5)
		}
	}()
	for i := 0; i < 5; i++ {
		_, _ = st.CreateDashboard(ctx, storeDoc(fmt.Sprintf("race-%d", i)), SavedByUser)
	}
	<-done
}
```

- [ ] Run `go test ./internal/homedash -count=1 -run '(?i)(TestPrefetchCatalogs|TestLookupEntities|TestStoreSetMaxDashboards)'`. Expected: build failure `h.PrefetchCatalogs undefined` / `h.LookupEntities undefined`.
- [ ] Create `internal/homedash/hub_lookup.go`:

```go
package homedash

import "context"

// PrefetchCatalogs fetches the stale catalogs of the active sources of ids concurrently
// (CatalogTimeout each, outside the lock), so that a following Subscriber.Set snapshots real
// entities instead of placeholders. It adds no interest and starts no source. Invalid ids and
// unknown or inactive sources are skipped.
func (h *Hub) PrefetchCatalogs(ctx context.Context, ids []EntityID) {
	now := h.opts.Now()
	seen := map[SourceID]bool{}
	var jobs []catalogJob
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		return
	}
	for _, id := range ids {
		src, _, err := ParseEntityID(string(id))
		if err != nil || seen[src] {
			continue
		}
		seen[src] = true
		rt := h.rts[src]
		if rt == nil || !rt.active || rt.src == nil || !h.catalogStale(rt, now) {
			continue
		}
		jobs = append(jobs, h.catalogJobLocked(rt))
	}
	h.mu.Unlock()
	h.fetchCatalogs(ctx, jobs)
}

// LookupEntities returns the cataloged entities among ids with their current state and the hub
// gates applied (as Entity does). Stale catalogs are fetched first (PrefetchCatalogs); unknown ids
// are absent from the map. It never warms or starts a source.
func (h *Hub) LookupEntities(ctx context.Context, ids []EntityID) map[EntityID]Entity {
	h.PrefetchCatalogs(ctx, ids)
	out := make(map[EntityID]Entity, len(ids))
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return out
	}
	for _, id := range ids {
		src, _, err := ParseEntityID(string(id))
		if err != nil {
			continue
		}
		rt := h.rts[src]
		if rt == nil || !rt.active {
			continue
		}
		e, ok := rt.catalog[id]
		if !ok {
			continue
		}
		if st, ok := h.states[id]; ok {
			e.State = st
		}
		out[id] = h.viewLocked(e)
	}
	return out
}
```

- [ ] GitNexus impact (R2) for `Store`, `OpenStore`, `Store.SetMaxDashboards`, `Store.CreateDashboard`.
- [ ] In `internal/homedash/store.go`: add `"sync/atomic"` to the imports; change the field to `maxDashboards atomic.Int64` (comment: "set by SetMaxDashboards from config hot reload while requests save"); in `OpenStore` replace the return with:

```go
	st := &Store{db: db, now: func() time.Time { return time.Now().UTC() }}
	st.maxDashboards.Store(50)
	return st, nil
```

  and `SetMaxDashboards` with `if n > 0 { s.maxDashboards.Store(int64(n)) }`.
- [ ] In `internal/homedash/store_dashboards.go` `CreateDashboard`: `if int64(n) >= s.maxDashboards.Load() {`.
- [ ] Run `go test ./internal/homedash/... -count=1`. Expected: PASS (new tests included).
- [ ] detect-changes (R2); commit `internal/homedash/hub_lookup.go internal/homedash/hub_lookup_test.go internal/homedash/store_limit_test.go internal/homedash/store.go internal/homedash/store_dashboards.go` with message `feat(homedash): catalog prefetch, entity lookup and a reload-safe dashboard cap`.

**Done when:** `go test ./internal/homedash/... -count=1` passes and `Hub.PrefetchCatalogs`, `Hub.LookupEntities` exist with the documented semantics.

---

### Task 2: `system` source adapter

**Files:** create `internal/homedash/sources/system/system.go`, `internal/homedash/sources/system/system_test.go`.

**Read first:** `internal/homedash/source.go` (Source, Sink, Factory); `internal/homedash/entity.go` (Kind, State, SourceStatus, status constants); `internal/homedash/hub_sources.go:423-489` (`hubSink.Status` accepts `starting|live|polling|degraded|unavailable`; `Update` drops updates older than the stored state, so `UpdatedAt` must never go backwards); `internal/homedash/sourcetest/fake.go:176-212` (`RecordingSink`); contract section "`system` entities".

- [ ] Write `internal/homedash/sources/system/system_test.go`:

```go
package system

import (
	"context"
	"errors"
	"testing"
	"time"

	"aurago/internal/homedash"
	"aurago/internal/homedash/sourcetest"
)

func lastUpdate(t *testing.T, sink *sourcetest.RecordingSink, native string) homedash.State {
	t.Helper()
	for i := len(sink.Updates) - 1; i >= 0; i-- {
		if sink.Updates[i].Native == native {
			return sink.Updates[i].State
		}
	}
	t.Fatalf("no update for %s", native)
	return homedash.State{}
}

func lastStatus(t *testing.T, sink *sourcetest.RecordingSink) homedash.SourceStatus {
	t.Helper()
	if len(sink.Statuses) == 0 {
		t.Fatal("no status")
	}
	return sink.Statuses[len(sink.Statuses)-1]
}

func TestSystemAdapterMapsSnapshot(t *testing.T) {
	t0 := time.Date(2026, 11, 25, 10, 0, 0, 0, time.UTC)
	now, snap, have := t0, Snapshot{}, false
	src := New(func() (Snapshot, bool) { return snap, have }, 5, func() time.Time { return now })
	sink := &sourcetest.RecordingSink{}

	// Before the collector's first sample: six read-only sensors, all unavailable.
	list, err := src.Catalog(context.Background())
	if err != nil || len(list) != 6 {
		t.Fatalf("catalog = %d entities, %v", len(list), err)
	}
	want := []homedash.EntityID{"system:cpu", "system:memory", "system:disk", "system:net_rx", "system:net_tx", "system:uptime"}
	for i, e := range list {
		if e.ID != want[i] || e.Kind != homedash.KindSensor || !e.ReadOnly || e.State.Available || len(e.Capabilities) != 2 {
			t.Fatalf("entity %d = %+v", i, e)
		}
	}
	src.pollOnce(sink)
	if st := lastStatus(t, sink); st.State != homedash.StatusUnavailable || st.Reason != "no_metrics" {
		t.Fatalf("status before a sample = %+v", st)
	}

	// First sample: percentages and uptime; no rate after one sample.
	snap, have = Snapshot{SampledAt: t0, CPUPercent: 12.34, MemoryPercent: 40, DiskPercent: 91.06,
		CPUOK: true, MemoryOK: true, DiskOK: true, NetworkOK: true,
		NetBytesRecv: 1_000_000, NetBytesSent: 500_000, BootTime: t0.Add(-2 * time.Hour)}, true
	now = t0.Add(time.Second)
	src.pollOnce(sink)
	if st := lastUpdate(t, sink, NativeCPU); st.Value != 12.3 || !st.Available || st.Stale || !st.UpdatedAt.Equal(t0) {
		t.Fatalf("cpu = %+v", st)
	}
	if st := lastUpdate(t, sink, NativeDisk); st.Value != 91.1 {
		t.Fatalf("disk = %+v", st)
	}
	if st := lastUpdate(t, sink, NativeNetRX); st.Available {
		t.Fatalf("net_rx after one sample = %+v, want unavailable", st)
	}
	if st := lastUpdate(t, sink, NativeUptime); st.Value != 7201.0 || !st.Available {
		t.Fatalf("uptime = %+v", st)
	}
	if st := lastStatus(t, sink); st.State != homedash.StatusPolling {
		t.Fatalf("status = %+v", st)
	}

	// Second sample 10 s later: rates in Mbit/s.
	snap.SampledAt, snap.NetBytesRecv, snap.NetBytesSent = t0.Add(10*time.Second), 13_500_000, 3_000_000
	now = t0.Add(11 * time.Second)
	src.pollOnce(sink)
	if st := lastUpdate(t, sink, NativeNetRX); st.Value != 10.0 || !st.Available {
		t.Fatalf("net_rx = %+v, want 10 Mbit/s", st)
	}
	if st := lastUpdate(t, sink, NativeNetTX); st.Value != 2.0 {
		t.Fatalf("net_tx = %+v, want 2 Mbit/s", st)
	}

	// Same sample again: only the uptime changes.
	n := len(sink.Updates)
	now = t0.Add(15 * time.Second)
	src.pollOnce(sink)
	if len(sink.Updates) != n+1 || sink.Updates[n].Native != NativeUptime {
		t.Fatalf("unchanged sample re-sent: %+v", sink.Updates[n:])
	}

	// 31 s after the last sample: stale (threshold max(3*5 s, 30 s)), status degraded.
	now = t0.Add(41 * time.Second)
	src.pollOnce(sink)
	if st := lastUpdate(t, sink, NativeCPU); !st.Stale || !st.Available {
		t.Fatalf("cpu after 31 s = %+v, want stale", st)
	}
	if st := lastStatus(t, sink); st.State != homedash.StatusDegraded || st.Reason != "stale_metrics" {
		t.Fatalf("status = %+v", st)
	}

	// Counter reset: the rate keeps its last value and the baseline moves.
	snap.SampledAt, snap.NetBytesRecv = t0.Add(50*time.Second), 100
	now = t0.Add(51 * time.Second)
	src.pollOnce(sink)
	if st := lastUpdate(t, sink, NativeNetRX); st.Value != 10.0 || st.Stale {
		t.Fatalf("net_rx after a counter reset = %+v", st)
	}
}

func TestSystemFactoryAndCommand(t *testing.T) {
	snap := func() (Snapshot, bool) { return Snapshot{}, false }
	f := Factory(snap)
	if _, ok := f(homedash.RuntimeConfig{}); ok {
		t.Fatal("no settings entry must disable the source")
	}
	off := homedash.RuntimeConfig{Sources: map[homedash.SourceID]homedash.SourceSettings{SourceID: {Enabled: false}}}
	if _, ok := f(off); ok {
		t.Fatal("a switched-off source must stay disabled")
	}
	on := homedash.RuntimeConfig{Sources: map[homedash.SourceID]homedash.SourceSettings{SourceID: {Enabled: true, PollSeconds: 2}}}
	src, ok := f(on)
	if !ok || src.ID() != SourceID || src.(*Source).poll != 2*time.Second {
		t.Fatalf("factory = %v %v", src, ok)
	}
	if _, ok := Factory(nil)(on); ok {
		t.Fatal("no snapshot function must disable the source")
	}
	_, err := src.Command(context.Background(), homedash.Command{})
	if !errors.Is(err, &homedash.Error{Code: homedash.CodeUnsupportedCommand}) {
		t.Fatalf("command = %v, want unsupported_command", err)
	}
}
```

- [ ] Run `go test ./internal/homedash/sources/system/ -count=1`. Expected: build failure (package has no non-test files / `New` undefined).
- [ ] Create `internal/homedash/sources/system/system.go`:

```go
// Package system is the HomeDash source for the AuraGo host: CPU, memory and disk use, network
// throughput and uptime. It reads the sample the server's metrics collector stores (no sampler of
// its own) and has no command.
package system

import (
	"context"
	"math"
	"sync"
	"time"

	"aurago/internal/homedash"
)

// SourceID is the id of the source and the prefix of its entity ids.
const SourceID homedash.SourceID = "system"

// Native ids of the six entities.
const (
	NativeCPU    = "cpu"
	NativeMemory = "memory"
	NativeDisk   = "disk"
	NativeNetRX  = "net_rx"
	NativeNetTX  = "net_tx"
	NativeUptime = "uptime"
)

// collectorInterval is the period of the server's metrics collector (internal/server/server_routes.go).
const collectorInterval = 10 * time.Second

// Snapshot is the server's latest metrics sample.
type Snapshot struct {
	SampledAt                              time.Time
	CPUPercent, MemoryPercent, DiskPercent float64
	CPUOK, MemoryOK, DiskOK, NetworkOK     bool
	NetBytesRecv, NetBytesSent             uint64    // cumulative counters of the host
	BootTime                               time.Time // zero when unknown
}

// SnapshotFunc returns the latest sample; ok is false before the collector's first tick.
type SnapshotFunc func() (Snapshot, bool)

type entityDef struct{ native, name, unit, deviceClass, icon string }

var entityDefs = []entityDef{
	{NativeCPU, "CPU", "%", "", "cpu"},
	{NativeMemory, "Memory", "%", "", "memory"},
	{NativeDisk, "Disk", "%", "", "disk"},
	{NativeNetRX, "Network receive", "Mbit/s", "data_rate", "download"},
	{NativeNetTX, "Network send", "Mbit/s", "data_rate", "upload"},
	{NativeUptime, "Uptime", "s", "duration", "timer"},
}

func (d entityDef) entity() homedash.Entity {
	return homedash.Entity{
		ID: homedash.MakeEntityID(SourceID, d.native), Source: SourceID, Kind: homedash.KindSensor,
		Name: d.name, Unit: d.unit, DeviceClass: d.deviceClass, IconHint: d.icon,
		Capabilities: []homedash.CapabilitySpec{{Name: homedash.CapNumeric, Allowed: true}, {Name: homedash.CapHistory, Allowed: true}},
		ReadOnly:     true,
	}
}

// Source is the system adapter.
type Source struct {
	snap SnapshotFunc
	poll time.Duration
	now  func() time.Time

	mu       sync.Mutex
	base     *Snapshot // the sample the last rate was measured from
	rx, tx   *float64  // last throughput in Mbit/s; nil until two samples were seen
	lastSent map[string]homedash.State
}

// New builds the adapter; pollSeconds outside 2..60 becomes 5, a nil now becomes time.Now.
func New(snap SnapshotFunc, pollSeconds int, now func() time.Time) *Source {
	if pollSeconds < 2 || pollSeconds > 60 {
		pollSeconds = 5
	}
	if now == nil {
		now = time.Now
	}
	return &Source{snap: snap, poll: time.Duration(pollSeconds) * time.Second, now: now, lastSent: map[string]homedash.State{}}
}

// Factory builds the adapter from the runtime configuration. It is disabled without a settings
// entry, when the source is switched off, or without a snapshot function. It never blocks.
func Factory(snap SnapshotFunc) homedash.Factory {
	return func(cfg homedash.RuntimeConfig) (homedash.Source, bool) {
		set, ok := cfg.Sources[SourceID]
		if !ok || !set.Enabled || snap == nil {
			return nil, false
		}
		return New(snap, set.PollSeconds, nil), true
	}
}

func (s *Source) ID() homedash.SourceID { return SourceID }

// SetInterest is ignored: one sample serves all six entities.
func (s *Source) SetInterest([]string) {}

// Command always refuses: the source is read-only.
func (s *Source) Command(context.Context, homedash.Command) (homedash.CommandResult, error) {
	return homedash.CommandResult{}, &homedash.Error{Code: homedash.CodeUnsupportedCommand, Message: "the system source is read-only"}
}

// Catalog returns the six entities with their current state; it does not move the rate baseline.
func (s *Source) Catalog(context.Context) ([]homedash.Entity, error) {
	snap, ok := s.snap()
	states := s.compute(snap, ok, s.now(), false)
	out := make([]homedash.Entity, 0, len(entityDefs))
	for _, d := range entityDefs {
		e := d.entity()
		e.State = states[d.native]
		out = append(out, e)
	}
	return out, nil
}

// Run sends the catalog once, then the status and the changed states every poll interval.
func (s *Source) Run(ctx context.Context, sink homedash.Sink) error {
	list, _ := s.Catalog(ctx)
	sink.Catalog(list)
	ticker := time.NewTicker(s.poll)
	defer ticker.Stop()
	for {
		s.pollOnce(sink)
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func (s *Source) staleAfter() time.Duration { return max(3*s.poll, 3*collectorInterval) }

// pollOnce moves the rate baseline, reports the status and sends the states that changed.
func (s *Source) pollOnce(sink homedash.Sink) {
	snap, ok := s.snap()
	now := s.now()
	status := homedash.SourceStatus{Source: SourceID, State: homedash.StatusPolling, At: now}
	switch {
	case !ok:
		status.State, status.Reason = homedash.StatusUnavailable, "no_metrics"
	case now.Sub(snap.SampledAt) > s.staleAfter():
		status.State, status.Reason = homedash.StatusDegraded, "stale_metrics"
	}
	sink.Status(status)
	states := s.compute(snap, ok, now, true)
	var ups []homedash.StateUpdate
	s.mu.Lock()
	for _, d := range entityDefs {
		next := states[d.native]
		if prev, sent := s.lastSent[d.native]; sent && sameState(prev, next) {
			continue
		}
		s.lastSent[d.native] = next
		ups = append(ups, homedash.StateUpdate{Native: d.native, State: next})
	}
	s.mu.Unlock()
	if len(ups) > 0 {
		sink.Update(ups...)
	}
}

// compute maps a sample to the six states. UpdatedAt is the sample time for sample values (it never
// goes backwards, so the hub keeps every update) and now for the uptime and for "no sample yet".
func (s *Source) compute(snap Snapshot, ok bool, now time.Time, advance bool) map[string]homedash.State {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ok && advance {
		s.advanceLocked(snap)
	}
	off := homedash.State{Value: nil, Available: false, UpdatedAt: now}
	if ok {
		off.UpdatedAt = snap.SampledAt
	}
	stale := ok && now.Sub(snap.SampledAt) > s.staleAfter()
	sample := func(v float64, avail bool) homedash.State {
		if !ok || !avail {
			return off
		}
		return homedash.State{Value: v, Available: true, Stale: stale, UpdatedAt: snap.SampledAt}
	}
	out := map[string]homedash.State{
		NativeCPU:    sample(round(snap.CPUPercent, 1), snap.CPUOK),
		NativeMemory: sample(round(snap.MemoryPercent, 1), snap.MemoryOK),
		NativeDisk:   sample(round(snap.DiskPercent, 1), snap.DiskOK),
		NativeNetRX:  off,
		NativeNetTX:  off,
		NativeUptime: {Value: nil, Available: false, UpdatedAt: now},
	}
	if s.rx != nil {
		out[NativeNetRX] = sample(*s.rx, snap.NetworkOK)
	}
	if s.tx != nil {
		out[NativeNetTX] = sample(*s.tx, snap.NetworkOK)
	}
	if ok && !snap.BootTime.IsZero() && now.After(snap.BootTime) {
		out[NativeUptime] = homedash.State{Value: math.Floor(now.Sub(snap.BootTime).Seconds()), Available: true, UpdatedAt: now}
	}
	return out
}

// advanceLocked measures the throughput between the baseline and a newer sample. A counter that
// went down (host reboot, interface reset) moves the baseline and keeps the last rate.
func (s *Source) advanceLocked(snap Snapshot) {
	if !snap.NetworkOK {
		return
	}
	b := s.base
	if b != nil && !snap.SampledAt.After(b.SampledAt) {
		return
	}
	if b != nil && snap.NetBytesRecv >= b.NetBytesRecv && snap.NetBytesSent >= b.NetBytesSent {
		if dt := snap.SampledAt.Sub(b.SampledAt).Seconds(); dt > 0 {
			rx := round(float64(snap.NetBytesRecv-b.NetBytesRecv)*8/1e6/dt, 3)
			tx := round(float64(snap.NetBytesSent-b.NetBytesSent)*8/1e6/dt, 3)
			s.rx, s.tx = &rx, &tx
		}
	}
	cp := snap
	s.base = &cp
}

func round(v float64, places int) float64 {
	p := math.Pow(10, float64(places))
	return math.Round(v*p) / p
}

// sameState reports a state that need not be sent again (unavailable states ignore UpdatedAt).
func sameState(a, b homedash.State) bool {
	return a.Available == b.Available && a.Stale == b.Stale && a.Value == b.Value &&
		(!a.Available || a.UpdatedAt.Equal(b.UpdatedAt))
}
```

- [ ] Run `go test ./internal/homedash/sources/system/ -count=1 -v`. Expected: 2 PASS lines.
- [ ] detect-changes (R2; new package only); commit both files with message `feat(homedash): read-only system source from the server metrics sample`.

**Done when:** both tests pass and the package imports only the standard library and `internal/homedash`.

---

### Task 3: Extend the server metrics sample

**Files:** modify `internal/server/dashboard_metrics_snapshot.go`, `internal/server/server_routes.go`, `internal/server/server.go`; create `internal/server/homedash_metrics_test.go`.

**Read first:** `internal/server/dashboard_metrics_snapshot.go` (whole file); `internal/server/server_routes.go:123-160` (collector: stores the sample at `:137`, already calls `tools.GetHostInfo()` at `:147`); `internal/tools/system_metrics.go:25-56` (`SystemMetrics`, `NetworkMetrics{BytesSent,BytesRecv}`, `Available["network"]` set at `:196`); `internal/server/dashboard_metrics_snapshot_test.go` (must stay green: new fields are zero there); `internal/server/server.go:428-430`.

- [ ] Write `internal/server/homedash_metrics_test.go`:

```go
package server

import (
	"os"
	"strings"
	"testing"
	"time"

	"aurago/internal/tools"
)

func TestMetricsSnapshotCarriesNetworkAndBootTime(t *testing.T) {
	s := &Server{}
	at := time.Date(2026, 11, 25, 10, 0, 0, 0, time.UTC)
	s.storeSystemMetricsSnapshot(tools.SystemMetrics{
		Available: map[string]bool{"network": true},
		Network:   tools.NetworkMetrics{BytesSent: 10, BytesRecv: 20},
	}, at)
	got, ok := s.loadSystemMetricsSnapshot()
	if !ok || !got.NetworkAvailable || got.NetBytesSent != 10 || got.NetBytesRecv != 20 {
		t.Fatalf("snapshot = %+v %v", got, ok)
	}
	if _, ok := s.loadHostBootTime(); ok {
		t.Fatal("boot time known before the collector stored one")
	}
	s.storeHostBootTime(at.Add(-time.Hour))
	boot, ok := s.loadHostBootTime()
	if !ok || !boot.Equal(at.Add(-time.Hour)) || boot.Location() != time.UTC {
		t.Fatalf("boot time = %v %v", boot, ok)
	}
	var none *Server
	none.storeHostBootTime(at)
	if _, ok := none.loadHostBootTime(); ok {
		t.Fatal("nil server: ok = true")
	}
}

func TestMetricsCollectorStoresHostBootTime(t *testing.T) {
	source, err := os.ReadFile("server_routes.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(source), "s.storeHostBootTime(") {
		t.Fatal("the 10 s collector must store the host boot time for the HomeDash system source")
	}
}
```

- [ ] Run `go test ./internal/server -count=1 -run '(?i)(TestMetricsSnapshotCarries|TestMetricsCollectorStores)'`. Expected: build failure (`NetworkAvailable`, `storeHostBootTime` undefined).
- [ ] GitNexus impact (R2) for `systemMetricsSnapshot`, `Server.storeSystemMetricsSnapshot`, `Server.run`, `Server` (struct).
- [ ] `dashboard_metrics_snapshot.go`: add fields `NetBytesSent uint64`, `NetBytesRecv uint64`, `NetworkAvailable bool` to `systemMetricsSnapshot`; fill them in `storeSystemMetricsSnapshot` from `m.Network.BytesSent`, `m.Network.BytesRecv`, `m.Available["network"]`; append:

```go
// storeHostBootTime publishes the host boot time the collector derived from the host uptime.
func (s *Server) storeHostBootTime(boot time.Time) {
	if s == nil || boot.IsZero() {
		return
	}
	s.hostBootUnix.Store(boot.Unix())
}

// loadHostBootTime returns the host boot time (UTC, whole seconds); ok is false before the first sample.
func (s *Server) loadHostBootTime() (time.Time, bool) {
	if s == nil {
		return time.Time{}, false
	}
	sec := s.hostBootUnix.Load()
	if sec <= 0 {
		return time.Time{}, false
	}
	return time.Unix(sec, 0).UTC(), true
}
```

- [ ] `server.go`: next to `latestSystemMetrics` add `hostBootUnix atomic.Int64 // host boot time (Unix seconds) the metrics collector derived; 0 = unknown`.
- [ ] `server_routes.go` collector (`:147-149`): inside the existing `if hostInfo := tools.GetHostInfo(); hostInfo.Uptime > 0 {` block add `s.storeHostBootTime(time.Now().Add(-time.Duration(hostInfo.Uptime) * time.Second))`. Do not change the `storeSystemMetricsSnapshot(metricsResult.Data, time.Now())` line (pinned by `TestDashboardMetricsCollectorStoresSnapshot`).
- [ ] Run `go test ./internal/server -count=1 -run '(?i)(Metrics)' -v`. Expected: PASS for the two new tests and both existing `TestDashboardMetrics*`.
- [ ] detect-changes; commit the four paths with message `feat(server): network counters and host boot time in the metrics sample`.

**Done when:** the four `*Metrics*` tests pass.

---

### Task 4: HomeDash source table

**Files:** create `internal/server/homedash_sources.go`, `internal/server/homedash_sources_test.go`.

**Read first:** Task 2 file; Task 3 accessors; `internal/homedash/runtime_config.go` (SourceSettings: `Enabled` must already be "homedash switch && integration switch"; `ConfigKey`); `internal/config/homedash.go:47-68`.

- [ ] Write `internal/server/homedash_sources_test.go`:

```go
package server

import (
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/homedash/sources/system"
	"aurago/internal/tools"
)

func TestHomeDashSystemSnapshotFromServerMetrics(t *testing.T) {
	s := &Server{}
	if _, ok := s.homeDashSystemSnapshot(); ok {
		t.Fatal("snapshot before the first collector sample")
	}
	at := time.Date(2026, 11, 25, 10, 0, 0, 0, time.UTC)
	s.storeSystemMetricsSnapshot(tools.SystemMetrics{
		Available: map[string]bool{"cpu": true, "memory": true, "disk": true, "network": true},
		CPU:       tools.CPUMetrics{UsagePercent: 12.5},
		Memory:    tools.MemoryMetrics{UsedPercent: 40},
		Disk:      tools.DiskMetrics{UsedPercent: 91},
		Network:   tools.NetworkMetrics{BytesSent: 10, BytesRecv: 20},
	}, at)
	got, ok := s.homeDashSystemSnapshot()
	if !ok || !got.SampledAt.Equal(at) || got.CPUPercent != 12.5 || got.MemoryPercent != 40 || got.DiskPercent != 91 ||
		!got.CPUOK || !got.MemoryOK || !got.DiskOK || !got.NetworkOK || got.NetBytesRecv != 20 || got.NetBytesSent != 10 ||
		!got.BootTime.IsZero() {
		t.Fatalf("snapshot = %+v %v", got, ok)
	}
	s.storeHostBootTime(at.Add(-time.Hour))
	if got, _ := s.homeDashSystemSnapshot(); !got.BootTime.Equal(at.Add(-time.Hour)) {
		t.Fatalf("boot time = %v", got.BootTime)
	}
}

func TestHomeDashSourceSpecsListSystem(t *testing.T) {
	specs := (&Server{}).homeDashSourceSpecs()
	if len(specs) != 1 || specs[0].ID != system.SourceID || specs[0].ConfigKey != "system" || specs[0].Factory == nil || specs[0].Settings == nil {
		t.Fatalf("specs = %+v", specs)
	}
	cfg := &config.Config{}
	cfg.HomeDash.Sources.System = config.HomeDashSourceConfig{Enabled: true, Control: true, PollSeconds: 7}
	if set := specs[0].Settings(cfg); !set.Enabled || set.Control || set.PollSeconds != 7 {
		t.Fatalf("settings = %+v (control must stay off: the source is read-only)", set)
	}
}
```

- [ ] Run `go test ./internal/server -count=1 -run '(?i)(TestHomeDashSystemSnapshot|TestHomeDashSourceSpecs)'`. Expected: build failure.
- [ ] Create `internal/server/homedash_sources.go`:

```go
package server

import (
	"aurago/internal/config"
	"aurago/internal/homedash"
	"aurago/internal/homedash/sources/system"
)

// homeDashSourceSpec is one HomeDash source with its server dependencies. ConfigKey is the key under
// homedash.sources and names the blocking switch in blocked_by. Settings must not block or lock
// (it runs under CfgMu during replaceConfigSnapshot); its Enabled already combines the homedash
// switch with the integration's own enabled flag. Every adapter package adds one row here.
type homeDashSourceSpec struct {
	ID        homedash.SourceID
	ConfigKey string
	Settings  func(cfg *config.Config) homedash.SourceSettings
	Factory   homedash.Factory
}

// homeDashSourceSpecs lists the sources of this build. M2: system only.
func (s *Server) homeDashSourceSpecs() []homeDashSourceSpec {
	return []homeDashSourceSpec{
		{ID: system.SourceID, ConfigKey: "system", Settings: homeDashSystemSettings, Factory: system.Factory(s.homeDashSystemSnapshot)},
	}
}

// homeDashSystemSettings: the host source has no integration switch and no control.
func homeDashSystemSettings(cfg *config.Config) homedash.SourceSettings {
	src := cfg.HomeDash.Sources.System
	return homedash.SourceSettings{Enabled: src.Enabled, Control: false, PollSeconds: src.PollSeconds}
}

// homeDashSystemSnapshot maps the collector's latest sample (no second sampler) for the system source.
func (s *Server) homeDashSystemSnapshot() (system.Snapshot, bool) {
	m, ok := s.loadSystemMetricsSnapshot()
	if !ok {
		return system.Snapshot{}, false
	}
	out := system.Snapshot{
		SampledAt: m.SampledAt, CPUPercent: m.CPUPercent, MemoryPercent: m.MemoryPercent, DiskPercent: m.DiskPercent,
		CPUOK: m.CPUAvailable, MemoryOK: m.MemoryAvailable, DiskOK: m.DiskAvailable, NetworkOK: m.NetworkAvailable,
		NetBytesRecv: m.NetBytesRecv, NetBytesSent: m.NetBytesSent,
	}
	if boot, ok := s.loadHostBootTime(); ok {
		out.BootTime = boot
	}
	return out, true
}
```

- [ ] Run the same command. Expected: 2 PASS.
- [ ] detect-changes; commit both files with message `feat(server): HomeDash source table with the system source`.

**Done when:** both tests pass.

---

### Task 5: HomeDash service, wiring, hot reload, capability

**Files:** create `internal/server/homedash_service.go`, `internal/server/homedash_service_test.go`, `internal/server/homedash_fixture_test.go`; modify `internal/server/server.go`, `internal/server/bluetooth_live.go`.

**Read first:** `internal/server/server.go:588-727` (`replaceConfigSnapshot`; runs under `CfgMu`; non-blocking hooks like `CYDCreator.UpdateLimits` at `:698-700`); `internal/server/server.go:955-966` (`integrationCtx = serverCtx`, Chibi init/defer), `:1046-1134` (deferred cleanup that closes Docker hosts, go2rtc, Bluetooth after the HTTP drain), `:1224-1232` (feature inits), `:1929` (`return s.run(shutdownCh)`), `:2186-2327` (`serveWithShutdown` waits for the drain before `Start` returns); `internal/server/docker_manager_service.go:38-99` (open/close pattern); `internal/server/bluetooth_live.go:11-42`; `internal/desktop/capabilities.go:23-41` (how `Requires` is consumed); `internal/config/sqlite_paths.go:82-101` (`SQLiteArchivePaths(cfg)["homedash"]`); `internal/server/desktop_security_test.go:140-165` (`testDesktopPermissionServer`); `internal/server/desktop_remote_security.go:232-265`; `internal/desktop/service_audit.go`; `internal/homedash/hub.go:13-47` (Options).

- [ ] Write `internal/server/homedash_fixture_test.go` (shared by every later HomeDash server test):

```go
package server

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/homedash"
	"aurago/internal/homedash/sourcetest"
)

// homeDashTestEnv is a server with auth on, a desktop:admin and a desktop:read token, and a
// HomeDash service on a temporary homedash.db.
type homeDashTestEnv struct {
	s       *Server
	svc     *homeDashService
	admin   string
	adminID string
	read    string
	audits  *homeDashAuditLog
}

type homeDashAuditLog struct {
	mu      sync.Mutex
	entries []string // "entity|command|outcome"
}

func (l *homeDashAuditLog) record(_ context.Context, cmd homedash.Command, outcome string) {
	l.mu.Lock()
	l.entries = append(l.entries, string(cmd.Entity)+"|"+string(cmd.Name)+"|"+outcome)
	l.mu.Unlock()
}

func (l *homeDashAuditLog) has(entry string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, e := range l.entries {
		if e == entry {
			return true
		}
	}
	return false
}

// fakeHomeDashSpec registers a FakeSource that is enabled and controllable.
func fakeHomeDashSpec(f *sourcetest.FakeSource) homeDashSourceSpec {
	return homeDashSourceSpec{ID: f.ID(), ConfigKey: string(f.ID()),
		Settings: func(*config.Config) homedash.SourceSettings { return homedash.SourceSettings{Enabled: true, Control: true} },
		Factory:  func(homedash.RuntimeConfig) (homedash.Source, bool) { return f, true }}
}

func newHomeDashTestEnv(t *testing.T, mutate func(*config.Config), specs ...homeDashSourceSpec) *homeDashTestEnv {
	t.Helper()
	s, readToken, _ := testDesktopPermissionServer(t)
	admin, meta, err := s.TokenManager.Create("homedash admin", []string{desktopScopeAdmin}, nil)
	if err != nil {
		t.Fatal(err)
	}
	s.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := s.Cfg
	cfg.Directories.DataDir = t.TempDir()
	cfg.VirtualDesktop.Enabled = true
	cfg.HomeDash.Enabled = true
	cfg.HomeDash.Sources.System.Enabled = true
	if err := config.NormalizeHomeDashConfig(&cfg.HomeDash); err != nil {
		t.Fatal(err)
	}
	if mutate != nil {
		mutate(cfg)
	}
	audits := &homeDashAuditLog{}
	svc, err := newHomeDashService(context.Background(), filepath.Join(cfg.Directories.DataDir, config.HomeDashDBFilename), cfg, homeDashDeps{
		Logger: s.Logger, Sources: specs, Audit: audits.record,
		HubOptions: homedash.Options{CoalesceInterval: 10 * time.Millisecond, CatalogTTL: time.Second, IdleStop: 200 * time.Millisecond},
	})
	if err != nil {
		t.Fatal(err)
	}
	s.homeDash.Store(svc)
	t.Cleanup(s.closeHomeDash)
	if cfg.HomeDash.Enabled && len(specs) > 0 {
		sourcetest.WaitFor(t, 2*time.Second, func() bool {
			for _, st := range svc.hub.Sources() {
				if st.State == homedash.StatusDisabled {
					return false
				}
			}
			return true
		}, "HomeDash configuration applied")
	}
	return &homeDashTestEnv{s: s, svc: svc, admin: admin, adminID: meta.ID, read: readToken, audits: audits}
}
```

- [ ] Write `internal/server/homedash_service_test.go`:

```go
package server

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/homedash"
	"aurago/internal/homedash/sourcetest"
)

func TestHomeDashRuntimeConfigFromConfig(t *testing.T) {
	base := func() *config.Config {
		c := &config.Config{}
		c.HomeDash.Enabled = true
		c.HomeDash.Sources.System.Enabled = true
		if err := config.NormalizeHomeDashConfig(&c.HomeDash); err != nil {
			t.Fatal(err)
		}
		return c
	}
	specs := []homeDashSourceSpec{{ID: "system", ConfigKey: "system", Settings: homeDashSystemSettings}}
	tests := []struct {
		name   string
		mutate func(*config.Config)
		ok     func(homedash.RuntimeConfig) bool
	}{
		{"defaults", nil, func(rc homedash.RuntimeConfig) bool {
			s := rc.Sources["system"]
			return rc.Enabled && !rc.EggMode && !rc.ReadOnly && !rc.DesktopReadOnly && rc.Limits.MaxStreams == 32 &&
				rc.Limits.MaxSubscriptions == 300 && rc.Limits.MaxDashboards == 50 && rc.History.SampleSeconds == 60 &&
				s.Enabled && !s.Control && s.PollSeconds == 5 && s.ConfigKey == "system"
		}},
		{"disabled", func(c *config.Config) { c.HomeDash.Enabled = false }, func(rc homedash.RuntimeConfig) bool { return !rc.Enabled }},
		{"egg mode", func(c *config.Config) { c.EggMode.Enabled = true }, func(rc homedash.RuntimeConfig) bool { return rc.EggMode }},
		{"desktop read-only", func(c *config.Config) { c.VirtualDesktop.ReadOnly = true }, func(rc homedash.RuntimeConfig) bool { return rc.DesktopReadOnly }},
		{"homedash read-only", func(c *config.Config) { c.HomeDash.ReadOnly = true }, func(rc homedash.RuntimeConfig) bool { return rc.ReadOnly }},
		{"kiosk", func(c *config.Config) { c.HomeDash.Kiosk.Enabled = true; c.HomeDash.Kiosk.AllowControl = false },
			func(rc homedash.RuntimeConfig) bool { return rc.KioskEnabled && !rc.KioskAllowControl }},
		{"system off", func(c *config.Config) { c.HomeDash.Sources.System.Enabled = false },
			func(rc homedash.RuntimeConfig) bool { return !rc.Sources["system"].Enabled }},
	}
	for _, tc := range tests {
		c := base()
		if tc.mutate != nil {
			tc.mutate(c)
		}
		if rc := homeDashRuntimeConfig(c, specs); !tc.ok(rc) {
			t.Errorf("%s: runtime config = %+v", tc.name, rc)
		}
	}
}

func TestHomeDashInitAndCloseOpenTheStore(t *testing.T) {
	cfg := &config.Config{}
	cfg.Directories.DataDir = t.TempDir()
	cfg.HomeDash.Enabled = true
	if err := config.NormalizeHomeDashConfig(&cfg.HomeDash); err != nil {
		t.Fatal(err)
	}
	s := &Server{Cfg: cfg, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	s.initHomeDash(context.Background())
	svc := s.homeDash.Load()
	if svc == nil {
		t.Fatal("initHomeDash published no service")
	}
	if _, err := os.Stat(filepath.Join(cfg.Directories.DataDir, config.HomeDashDBFilename)); err != nil {
		t.Fatalf("homedash.db: %v", err)
	}
	if got := svc.hub.Sources(); len(got) != 1 || got[0].Source != "system" {
		t.Fatalf("sources = %+v", got)
	}
	s.closeHomeDash()
	s.closeHomeDash()
	if s.homeDash.Load() != nil {
		t.Fatal("service still published after closeHomeDash")
	}
	none := &Server{Cfg: &config.Config{}}
	none.initHomeDash(context.Background())
	if none.homeDash.Load() != nil {
		t.Fatal("without a data directory HomeDash must stay unavailable")
	}
}

func TestHomeDashReconfiguresOnConfigSnapshotAndGatesCapability(t *testing.T) {
	fake := sourcetest.New("fake", sourcetest.Entity("fake:a", homedash.KindSensor))
	env := newHomeDashTestEnv(t, nil, fakeHomeDashSpec(fake))
	caps := desktopCapabilities{env.s}
	status := func() string { return env.svc.hub.Sources()[0].State }
	if status() != homedash.StatusIdle || !caps.HasCapability("homedash") {
		t.Fatalf("start: status %q, capability %v", status(), caps.HasCapability("homedash"))
	}
	cfg := *env.s.ConfigSnapshot()
	cfg.HomeDash.Enabled = false
	env.s.replaceConfigSnapshot(&cfg)
	sourcetest.WaitFor(t, 2*time.Second, func() bool { return status() == homedash.StatusDisabled }, "source disabled by hot reload")
	if caps.HasCapability("homedash") {
		t.Fatal("capability on although homedash.enabled is false")
	}
	cfg.HomeDash.Enabled = true
	env.s.replaceConfigSnapshot(&cfg)
	sourcetest.WaitFor(t, 2*time.Second, func() bool { return status() == homedash.StatusIdle }, "source enabled again")
	cfg.VirtualDesktop.Enabled = false
	env.s.replaceConfigSnapshot(&cfg)
	if caps.HasCapability("homedash") {
		t.Fatal("capability on without the virtual desktop")
	}
}
```

- [ ] Run `go test ./internal/server -count=1 -run '(?i)(TestHomeDashRuntimeConfig|TestHomeDashInitAndClose|TestHomeDashReconfigures)'`. Expected: build failure.
- [ ] Create `internal/server/homedash_service.go`:

```go
package server

import (
	"context"
	"log/slog"
	"time"

	"aurago/internal/config"
	"aurago/internal/desktop"
	"aurago/internal/homedash"
)

// homeDashService is the server's HomeDash runtime: hub, homedash.db store and widget catalog. It is
// built once at start (initHomeDash) on the server lifetime context and closed after the HTTP drain
// (closeHomeDash). The store path is fixed at start; a data_dir change needs a restart.
type homeDashService struct {
	hub      *homedash.Hub
	store    *homedash.Store
	cat      homedash.WidgetCatalog
	specs    []homeDashSourceSpec
	coalesce time.Duration // the stream writer flushes at most once per interval
	log      *slog.Logger
}

// homeDashDeps are the parts newHomeDashService does not build itself.
type homeDashDeps struct {
	Logger     *slog.Logger
	Sources    []homeDashSourceSpec
	Audit      func(ctx context.Context, cmd homedash.Command, outcome string)
	HubOptions homedash.Options // Logger and Audit are taken from the fields above
}

func newHomeDashService(parent context.Context, dbPath string, cfg *config.Config, deps homeDashDeps) (*homeDashService, error) {
	store, err := homedash.OpenStore(dbPath)
	if err != nil {
		return nil, err
	}
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	opts := deps.HubOptions
	opts.Logger, opts.Audit = logger, deps.Audit
	coalesce := opts.CoalesceInterval
	if coalesce <= 0 {
		coalesce = 100 * time.Millisecond
	}
	hub := homedash.NewHub(parent, opts)
	for _, spec := range deps.Sources {
		hub.Register(spec.ID, spec.Factory)
	}
	svc := &homeDashService{hub: hub, store: store, cat: homedash.Catalog(), specs: deps.Sources, coalesce: coalesce, log: logger}
	svc.reconfigure(cfg)
	return svc, nil
}

// reconfigure publishes a configuration. It never blocks, so replaceConfigSnapshot may call it
// under CfgMu: the hub only stores the snapshot and wakes its own goroutine.
func (svc *homeDashService) reconfigure(cfg *config.Config) {
	svc.store.SetMaxDashboards(cfg.HomeDash.Limits.MaxDashboards)
	svc.hub.Reconfigure(homeDashRuntimeConfig(cfg, svc.specs))
}

// close ends every stream ("shutdown"), stops the sources (waits up to 10 s) and closes the store.
func (svc *homeDashService) close() {
	svc.hub.Close()
	if err := svc.store.Close(); err != nil {
		svc.log.Warn("[HomeDash] Store did not close cleanly", "error", err)
	}
}

// homeDashRuntimeConfig builds the hub's immutable snapshot. It reads cfg only (no lock).
func homeDashRuntimeConfig(cfg *config.Config, specs []homeDashSourceSpec) homedash.RuntimeConfig {
	hd := cfg.HomeDash
	rc := homedash.RuntimeConfig{
		Enabled:           hd.Enabled,
		ReadOnly:          hd.ReadOnly,
		DesktopReadOnly:   cfg.VirtualDesktop.ReadOnly,
		EggMode:           cfg.EggMode.Enabled,
		KioskEnabled:      hd.Kiosk.Enabled,
		KioskAllowControl: hd.Kiosk.AllowControl,
		Limits: homedash.Limits{
			MaxDashboards: hd.Limits.MaxDashboards, MaxStreams: hd.Limits.MaxStreams, MaxKioskStreams: hd.Limits.MaxKioskStreams,
			MaxSubscriptions: hd.Limits.MaxSubscriptions, CommandsPerMinute: hd.Limits.CommandsPerMinute,
		},
		History: homedash.HistoryConfig{RingPoints: hd.History.RingPoints, RingEntities: hd.History.RingEntities, SampleSeconds: hd.History.SampleSeconds},
		Sources: make(map[homedash.SourceID]homedash.SourceSettings, len(specs)),
	}
	for _, spec := range specs {
		set := spec.Settings(cfg)
		set.ConfigKey = spec.ConfigKey
		rc.Sources[spec.ID] = set
	}
	return rc
}

// initHomeDash opens homedash.db and starts the hub on the server lifetime context. A failure is
// logged and leaves HomeDash unavailable (routes answer 503 storage_unavailable); the server runs on.
func (s *Server) initHomeDash(ctx context.Context) {
	if s == nil || ctx.Err() != nil {
		return
	}
	cfg := s.ConfigSnapshot()
	if cfg == nil {
		return
	}
	path := config.SQLiteArchivePaths(cfg)["homedash"]
	if path == "" {
		s.logHomeDash("HomeDash has no data directory and stays unavailable", nil)
		return
	}
	svc, err := newHomeDashService(ctx, path, cfg, homeDashDeps{Logger: s.Logger, Sources: s.homeDashSourceSpecs(), Audit: s.homeDashAudit})
	if err != nil {
		s.logHomeDash("HomeDash store could not be opened", err)
		return
	}
	s.homeDash.Store(svc)
}

// closeHomeDash withdraws and closes the service. Idempotent; Start defers it right after
// initHomeDash, so it runs after the HTTP drain and before the integrations close.
func (s *Server) closeHomeDash() {
	if s == nil {
		return
	}
	if svc := s.homeDash.Swap(nil); svc != nil {
		svc.close()
	}
}

func (s *Server) logHomeDash(message string, err error) {
	if s != nil && s.Logger != nil {
		s.Logger.Warn("[HomeDash] "+message, "error", err)
	}
}

// homeDashAuditKey carries the HTTP attribution of a command into the hub's audit callback.
type homeDashAuditKey struct{}

// homeDashAudit writes one desktop audit row per command outcome (ok, superseded, refused:<code>,
// failed). Best effort: a failure is logged and never changes the command's answer.
func (s *Server) homeDashAudit(ctx context.Context, cmd homedash.Command, outcome string) {
	info, _ := ctx.Value(homeDashAuditKey{}).(desktop.AuditRequestInfo)
	actx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	svc, _, err := s.getDesktopService(actx)
	if err == nil {
		err = svc.AuditWithRequest(actx, "homedash.command", string(cmd.Entity), map[string]string{
			"command": string(cmd.Name), "outcome": outcome, "dashboard_id": cmd.DashboardID,
			"widget_id": cmd.WidgetID, "principal": string(cmd.Principal.Kind),
		}, desktop.SourceUser, info)
	}
	if err != nil {
		s.logHomeDash("Command audit could not be written", err)
	}
}
```

- [ ] GitNexus impact (R2) for `Server` (struct), `Server.replaceConfigSnapshot`, `Start`, `desktopCapabilities.HasCapability`. Expect HIGH/CRITICAL for the hub symbols: report it, the edits are additive.
- [ ] `server.go` struct, next to `hostBootUnix`: `homeDash atomic.Pointer[homeDashService] // HomeDash runtime (homedash_service.go); nil before initHomeDash, after closeHomeDash or when the store failed`.
- [ ] `server.go` `replaceConfigSnapshot`, right after the `if s.CYDCreator != nil { … }` block (`:698-700`):

```go
	if svc := s.homeDash.Load(); svc != nil {
		svc.reconfigure(cfg)
	}
```

- [ ] `server.go` `Start`, right after `s.initBluetoothLive(serverCtx)` (`:1232`):

```go
	s.initHomeDash(serverCtx)
	defer s.closeHomeDash()
```

- [ ] `bluetooth_live.go` `HasCapability`, before `default:`:

```go
	case "homedash":
		// The desktop app catalog shows HomeDash (manifest Requires ["homedash"]) only with both switches on.
		cfg := c.s.ConfigSnapshot()
		return cfg != nil && cfg.HomeDash.Enabled && cfg.VirtualDesktop.Enabled
```

- [ ] Run `go test ./internal/server -count=1 -run '(?i)(TestHomeDashRuntimeConfig|TestHomeDashInitAndClose|TestHomeDashReconfigures|TestDashboardMetrics|BluetoothLive)' -v`. Expected: all PASS.
- [ ] detect-changes; commit `internal/server/homedash_service.go internal/server/homedash_service_test.go internal/server/homedash_fixture_test.go internal/server/server.go internal/server/bluetooth_live.go` with message `feat(server): HomeDash service wiring, hot reload and desktop capability`.

**Done when:** the three new tests pass and `go build ./...` succeeds in the worktree.

---

### Task 6: Route table, dispatch, envelope, bootstrap, catalog, audit contract

**Files:** create `internal/server/homedash_routes.go`, `internal/server/homedash_routes_test.go`, `internal/audit/homedash_manifest_test.go`; modify `internal/server/server_routes.go`, `internal/audit/audit_manifests.go`, `internal/server/homedash_fixture_test.go`.

**Read first:** contract sections "General rules", "Error codes"; `internal/server/desktop_auth.go:23-65` (`authenticateDesktopPermission`), `internal/server/desktop_operation.go:75-93,192-196`; `internal/server/auth.go:1018-1051` (middleware errors), `:143-156` (`sessionNonce`, stable across renewal), `:1173` (`bearerCredential`); `internal/server/json_request.go` (no route-cap argument: wrap the body in a smaller `http.MaxBytesReader` first, `internal/server/AGENTS.md:150`); `internal/server/server_routes.go:528-532`; `internal/audit/audit_manifests.go:123-171`; `internal/audit/audit_manifests_test.go:92-153,2007-2030` (a dedicated contract with DELETE is accepted: only public mutating routes are refused, coverage is by prefix).

- [ ] Append to `internal/server/homedash_fixture_test.go` (add imports `bytes`, `encoding/json`, `net/http/httptest`, `strings`):

```go
// do calls the HomeDash handler directly. body: nil, a raw JSON string, or a value to marshal.
func (e *homeDashTestEnv) do(t *testing.T, token, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var rd io.Reader
	switch b := body.(type) {
	case nil:
	case string:
		rd = strings.NewReader(b)
	default:
		raw, err := json.Marshal(b)
		if err != nil {
			t.Fatal(err)
		}
		rd = bytes.NewReader(raw)
	}
	r := httptest.NewRequest(method, homeDashAPIPrefix+path, rd)
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	handleHomeDash(e.s).ServeHTTP(w, r)
	return w
}

func hdDecode(t *testing.T, w *httptest.ResponseRecorder, dst any) {
	t.Helper()
	if err := json.Unmarshal(w.Body.Bytes(), dst); err != nil {
		t.Fatalf("answer is not JSON: %v\n%s", err, w.Body.String())
	}
}

// hdCode returns "code", or "error" for answers of the shared auth layers.
func hdCode(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct{ Code, Error string }
	hdDecode(t, w, &body)
	if body.Code != "" {
		return body.Code
	}
	return body.Error
}
```

- [ ] Write `internal/server/homedash_routes_test.go`:

```go
package server

import (
	"net/http"
	"testing"

	"aurago/internal/config"
	"aurago/internal/homedash"
)

func TestHomeDashRoutesRequireDesktopAdmin(t *testing.T) {
	env := newHomeDashTestEnv(t, nil)
	if w := env.do(t, "", http.MethodGet, "bootstrap", nil); w.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous = %d %s", w.Code, w.Body.String())
	}
	if w := env.do(t, env.read, http.MethodGet, "bootstrap", nil); w.Code != http.StatusForbidden || hdCode(t, w) != "desktop_scope_required" {
		t.Fatalf("desktop:read = %d %s", w.Code, w.Body.String())
	}
	w := env.do(t, env.admin, http.MethodGet, "bootstrap", nil)
	if w.Code != http.StatusOK || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("admin = %d %v %s", w.Code, w.Header(), w.Body.String())
	}
	var boot struct {
		Status         string   `json:"status"`
		APIVersion     int      `json:"api_version"`
		StreamProtocol int      `json:"stream_protocol"`
		CatalogHash    string   `json:"catalog_hash"`
		Templates      []string `json:"templates"`
		Limits         struct {
			Subscriptions int `json:"subscriptions"`
		} `json:"limits"`
	}
	hdDecode(t, w, &boot)
	if boot.Status != "ok" || boot.APIVersion != 1 || boot.StreamProtocol != 1 || boot.CatalogHash != homedash.Catalog().Hash ||
		len(boot.Templates) != 1 || boot.Templates[0] != "system" || boot.Limits.Subscriptions != 300 {
		t.Fatalf("bootstrap = %+v", boot)
	}
}

func TestHomeDashRouteTableAnswers404And405(t *testing.T) {
	env := newHomeDashTestEnv(t, nil)
	if w := env.do(t, env.admin, http.MethodGet, "kiosk/devices", nil); w.Code != http.StatusNotFound || hdCode(t, w) != "not_found" {
		t.Fatalf("unknown route = %d %s", w.Code, w.Body.String())
	}
	w := env.do(t, env.admin, http.MethodDelete, "bootstrap", nil)
	if w.Code != http.StatusMethodNotAllowed || hdCode(t, w) != "method_not_allowed" || w.Header().Get("Allow") != "GET" {
		t.Fatalf("405 = %d %v %s", w.Code, w.Header(), w.Body.String())
	}
}

func TestHomeDashUnavailableAnswers503(t *testing.T) {
	for name, mutate := range map[string]func(*config.Config){
		"homedash off": func(c *config.Config) { c.HomeDash.Enabled = false },
		"egg mode":     func(c *config.Config) { c.EggMode.Enabled = true },
		"no desktop":   func(c *config.Config) { c.VirtualDesktop.Enabled = false },
	} {
		env := newHomeDashTestEnv(t, mutate)
		if w := env.do(t, env.admin, http.MethodGet, "bootstrap", nil); w.Code != http.StatusServiceUnavailable || hdCode(t, w) != "homedash_disabled" {
			t.Fatalf("%s: %d %s", name, w.Code, w.Body.String())
		}
	}
	env := newHomeDashTestEnv(t, nil)
	env.s.closeHomeDash()
	if w := env.do(t, env.admin, http.MethodGet, "bootstrap", nil); w.Code != http.StatusServiceUnavailable || hdCode(t, w) != "storage_unavailable" {
		t.Fatalf("no service: %d %s", w.Code, w.Body.String())
	}
}

func TestHomeDashWidgetCatalogETag(t *testing.T) {
	env := newHomeDashTestEnv(t, nil)
	w := env.do(t, env.admin, http.MethodGet, "widget-catalog", nil)
	etag := `"` + homedash.Catalog().Hash + `"`
	if w.Code != http.StatusOK || w.Header().Get("ETag") != etag {
		t.Fatalf("catalog = %d %v", w.Code, w.Header())
	}
	var body struct {
		Catalog homedash.WidgetCatalog `json:"catalog"`
	}
	hdDecode(t, w, &body)
	if body.Catalog.Hash != homedash.Catalog().Hash || len(body.Catalog.Widgets) != 24 {
		t.Fatalf("catalog hash %q, %d widgets", body.Catalog.Hash, len(body.Catalog.Widgets))
	}
	r := env.do(t, env.admin, http.MethodGet, "widget-catalog", nil)
	_ = r
	req := httptestRequestWithHeader(t, env, "widget-catalog", "If-None-Match", etag)
	if req.Code != http.StatusNotModified || req.Body.Len() != 0 {
		t.Fatalf("If-None-Match = %d %q", req.Code, req.Body.String())
	}
}

func TestHomeDashStatusTableCoversCoreCodes(t *testing.T) {
	want := map[string]int{
		homedash.CodeBadRequest: 400, homedash.CodeInvalidArgs: 400, homedash.CodeUnsupportedCommand: 400,
		homedash.CodeReadOnly: 403, homedash.CodeForbiddenEntity: 403, homedash.CodeViewOnly: 403,
		homedash.CodeDashboardNotFound: 404, homedash.CodeEntityNotFound: 404, homedash.CodeStreamNotFound: 404,
		homedash.CodeRevisionNotFound: 404, homeDashCodeNotFound: 404, homeDashCodeMethodNotAllowed: 405,
		homedash.CodeRevisionConflict: 409, homedash.CodeConfirmationRequired: 409, homedash.CodeSchemaTooNew: 409,
		homedash.CodeSlugTaken: 409, homeDashCodeBodyTooLarge: 413, homedash.CodeInvalidDocument: 422,
		homedash.CodeLimitExceeded: 422, homedash.CodeRateLimited: 429, homedash.CodeTooManyStreams: 429,
		homedash.CodeCommandFailed: 502, homedash.CodeSourceError: 502, homedash.CodeHomeDashDisabled: 503,
		homedash.CodeSourceUnavailable: 503, homedash.CodeHubClosed: 503, homeDashCodeStorageUnavailable: 503,
		homeDashCodeTimeout: 504, homeDashCodeInternal: 500,
	}
	for code, status := range want {
		if got := homeDashStatus(code); got != status {
			t.Errorf("%s = %d, want %d", code, got, status)
		}
	}
}
```

  and add this helper to the same test file:

```go
func httptestRequestWithHeader(t *testing.T, env *homeDashTestEnv, path, key, value string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(http.MethodGet, homeDashAPIPrefix+path, nil)
	r.Header.Set("Authorization", "Bearer "+env.admin)
	r.Header.Set(key, value)
	w := httptest.NewRecorder()
	handleHomeDash(env.s).ServeHTTP(w, r)
	return w
}
```

  (imports: `net/http/httptest`; delete the two placeholder lines `r := env.do(...)` / `_ = r` if the linter complains.)
- [ ] Write `internal/audit/homedash_manifest_test.go`:

```go
package audit

import (
	"slices"
	"testing"
)

// The HomeDash desktop API has one dedicated contract that allows DELETE (design section 11 item 1)
// and names the event stream.
func TestHomeDashDesktopRouteContract(t *testing.T) {
	t.Parallel()
	for _, c := range RouteContractManifest() {
		if c.Pattern != "/api/desktop/homedash/" {
			continue
		}
		if c.Auth != "session-or-desktop-admin-token" || !slices.Contains(c.Methods, "DELETE") || !slices.Contains(c.ContentTypes, "text/event-stream") {
			t.Fatalf("/api/desktop/homedash/ contract = %+v", c)
		}
		return
	}
	t.Fatal("/api/desktop/homedash/ must have an explicit route contract")
}
```

- [ ] Run `go test ./internal/server -count=1 -run '(?i)(TestHomeDashRoutes|TestHomeDashRouteTable|TestHomeDashUnavailable|TestHomeDashWidgetCatalog|TestHomeDashStatusTable)'` and `go test ./internal/audit -count=1 -run TestHomeDashDesktopRouteContract`. Expected: build failure / FAIL.
- [ ] Create `internal/server/homedash_routes.go`:

```go
package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/homedash"
)

const (
	homeDashAPIPrefix      = "/api/desktop/homedash/"
	homeDashAPIVersion     = 1
	homeDashTemplateSystem = "system"

	homeDashSmallBody        int64 = 64 << 10
	homeDashSubscriptionBody int64 = 256 << 10
	homeDashDocumentBody     int64 = 512 << 10

	// homeDashCatalogWait bounds a request that may fetch catalogs (8 s per source plus slack).
	homeDashCatalogWait = 9 * time.Second
)

// HTTP-layer codes the core does not define (design section 12.3).
const (
	homeDashCodeNotFound           = "not_found"
	homeDashCodeMethodNotAllowed   = "method_not_allowed"
	homeDashCodeBodyTooLarge       = "body_too_large"
	homeDashCodeStorageUnavailable = "storage_unavailable"
	homeDashCodeTimeout            = "timeout"
	homeDashCodeInternal           = "internal_error"
)

// hdRequest is one routed HomeDash request with the configuration read once.
type hdRequest struct {
	s   *Server
	svc *homeDashService
	cfg *config.Config
	w   http.ResponseWriter
	r   *http.Request
}

// hdEndpoint is one method of a route. op desktopRead works on a read-only desktop; any other op
// passes checkDesktopOperation first (403 desktop_readonly on a read-only desktop).
type hdEndpoint struct {
	op desktopOperation
	fn func(*hdRequest)
}

// homeDashRoutes maps the path below homeDashAPIPrefix to its methods. Later tasks add rows.
var homeDashRoutes = map[string]map[string]hdEndpoint{
	"bootstrap":      {http.MethodGet: {desktopRead, (*hdRequest).bootstrap}},
	"widget-catalog": {http.MethodGet: {desktopRead, (*hdRequest).widgetCatalog}},
}

// handleHomeDash serves /api/desktop/homedash/. Order: desktop:admin (session or token) → route
// (404) → method (405) → availability (503) → desktop operation for writes (403) → handler.
func handleHomeDash(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if !authenticateDesktopPermission(s, w, r, desktopScopeAdmin) {
			return
		}
		methods, ok := homeDashRoutes[strings.TrimPrefix(r.URL.Path, homeDashAPIPrefix)]
		if !ok || !strings.HasPrefix(r.URL.Path, homeDashAPIPrefix) {
			homeDashWriteError(w, http.StatusNotFound, homeDashCodeNotFound, "unknown HomeDash route", nil)
			return
		}
		ep, ok := methods[r.Method]
		if !ok {
			w.Header().Set("Allow", homeDashAllow(methods))
			homeDashWriteError(w, http.StatusMethodNotAllowed, homeDashCodeMethodNotAllowed, "method not allowed", nil)
			return
		}
		cfg := s.ConfigSnapshot()
		if !homeDashEnabled(cfg) {
			homeDashWriteError(w, http.StatusServiceUnavailable, homedash.CodeHomeDashDisabled, "HomeDash is disabled", nil)
			return
		}
		svc := s.homeDash.Load()
		if svc == nil {
			homeDashWriteError(w, http.StatusServiceUnavailable, homeDashCodeStorageUnavailable, "HomeDash storage is unavailable", nil)
			return
		}
		if ep.op != desktopRead && !checkDesktopOperation(s, w, r, ep.op) {
			return
		}
		ep.fn(&hdRequest{s: s, svc: svc, cfg: cfg, w: w, r: r})
	}
}

// homeDashEnabled: the API runs with homedash and the virtual desktop on and outside egg mode.
func homeDashEnabled(cfg *config.Config) bool {
	return cfg != nil && cfg.HomeDash.Enabled && !cfg.EggMode.Enabled && cfg.VirtualDesktop.Enabled
}

func homeDashAllow(methods map[string]hdEndpoint) string {
	list := make([]string, 0, len(methods))
	for m := range methods {
		list = append(list, m)
	}
	sort.Strings(list)
	return strings.Join(list, ", ")
}

func homeDashWriteJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func homeDashWriteOK(w http.ResponseWriter, status int, fields map[string]any) {
	body := map[string]any{"status": "ok"}
	for k, v := range fields {
		body[k] = v
	}
	homeDashWriteJSON(w, status, body)
}

func homeDashWriteError(w http.ResponseWriter, status int, code, message string, extra map[string]any) {
	body := map[string]any{"status": "error", "code": code, "message": message}
	for k, v := range extra {
		if k != "status" && k != "code" && k != "message" {
			body[k] = v
		}
	}
	homeDashWriteJSON(w, status, body)
}

// homeDashStatus maps a code to its HTTP status (contract table "Error codes").
func homeDashStatus(code string) int {
	switch code {
	case homedash.CodeBadRequest, homedash.CodeInvalidID, homedash.CodeInvalidArgs, homedash.CodeUnsupportedCommand, homedash.CodePairingInvalid:
		return http.StatusBadRequest
	case homedash.CodeKioskUnpaired, homedash.CodeKioskRevoked:
		return http.StatusUnauthorized
	case homedash.CodeReadOnly, homedash.CodeForbiddenEntity, homedash.CodeViewOnly:
		return http.StatusForbidden
	case homedash.CodeDashboardNotFound, homedash.CodeEntityNotFound, homedash.CodeStreamNotFound,
		homedash.CodeRevisionNotFound, homedash.CodeKioskDisabled, homeDashCodeNotFound:
		return http.StatusNotFound
	case homeDashCodeMethodNotAllowed:
		return http.StatusMethodNotAllowed
	case homedash.CodeRevisionConflict, homedash.CodeConfirmationRequired, homedash.CodeSchemaTooNew, homedash.CodeSlugTaken:
		return http.StatusConflict
	case homeDashCodeBodyTooLarge:
		return http.StatusRequestEntityTooLarge
	case homedash.CodeInvalidDocument, homedash.CodeInvalidPatch, homedash.CodeLimitExceeded:
		return http.StatusUnprocessableEntity
	case homedash.CodeRateLimited, homedash.CodeTooManyStreams:
		return http.StatusTooManyRequests
	case homedash.CodeCommandFailed, homedash.CodeSourceError:
		return http.StatusBadGateway
	case homedash.CodeHomeDashDisabled, homedash.CodeSourceUnavailable, homedash.CodeHubClosed, homeDashCodeStorageUnavailable:
		return http.StatusServiceUnavailable
	case homeDashCodeTimeout:
		return http.StatusGatewayTimeout
	default:
		return http.StatusInternalServerError
	}
}

func (h *hdRequest) bad(message string) {
	homeDashWriteError(h.w, http.StatusBadRequest, homedash.CodeBadRequest, message, nil)
}

// fail answers an error: *homedash.Error by its code (with blocked_by, retry_after_ms + Retry-After,
// current_revision, issues), a finished request context as 504 timeout, anything else as a logged
// 500 internal_error that never carries the error text.
func (h *hdRequest) fail(err error) {
	var he *homedash.Error
	switch {
	case errors.As(err, &he):
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		he = &homedash.Error{Code: homeDashCodeTimeout, Message: "the request did not finish in time"}
	default:
		h.s.logHomeDash("Request failed: "+h.r.URL.Path, err)
		he = &homedash.Error{Code: homeDashCodeInternal, Message: "HomeDash could not complete the request"}
	}
	extra := map[string]any{}
	if he.BlockedBy != "" {
		extra["blocked_by"] = he.BlockedBy
	}
	if he.RetryAfterMS > 0 {
		extra["retry_after_ms"] = he.RetryAfterMS
		h.w.Header().Set("Retry-After", strconv.FormatInt((he.RetryAfterMS+999)/1000, 10))
	}
	if he.CurrentRevision > 0 {
		extra["current_revision"] = he.CurrentRevision
	}
	if len(he.Issues) > 0 {
		extra["issues"] = he.Issues
	}
	homeDashWriteError(h.w, homeDashStatus(he.Code), he.Code, he.Message, extra)
}

func (h *hdRequest) decode(dst any, limit int64) bool         { return h.decodeBody(dst, limit, false) }
func (h *hdRequest) decodeOptional(dst any, limit int64) bool { return h.decodeBody(dst, limit, true) }

// decodeBody reads one JSON body through the server's decodeJSONRequest; the route limit wins
// because the smaller MaxBytesReader wraps the body first (internal/server/AGENTS.md).
func (h *hdRequest) decodeBody(dst any, limit int64, optional bool) bool {
	if h.r.Body != nil {
		h.r.Body = http.MaxBytesReader(h.w, h.r.Body, limit)
	}
	err := decodeJSONRequest(h.w, h.r, dst, h.s)
	switch {
	case err == nil:
		return true
	case errors.Is(err, io.EOF) && optional:
		return true
	case errors.Is(err, io.EOF):
		h.bad("a JSON body is required")
	case jsonDecodeStatus(err) == http.StatusRequestEntityTooLarge:
		homeDashWriteError(h.w, http.StatusRequestEntityTooLarge, homeDashCodeBodyTooLarge, "the request body is too large", nil)
	default:
		h.bad("the body is not valid JSON for this endpoint")
	}
	return false
}

func (h *hdRequest) dashboardID() (string, bool) {
	id := h.r.URL.Query().Get("id")
	if !homedash.ValidDashboardID(id) {
		h.bad("id must be a dashboard id")
		return "", false
	}
	return id, true
}

func (h *hdRequest) revisionParam() (int64, bool) {
	rev, err := strconv.ParseInt(h.r.URL.Query().Get("revision"), 10, 64)
	if err != nil || rev < 1 {
		h.bad("revision must be a positive integer")
		return 0, false
	}
	return rev, true
}

func (h *hdRequest) entityID() (homedash.EntityID, bool) {
	raw := h.r.URL.Query().Get("id")
	if _, _, err := homedash.ParseEntityID(raw); err != nil {
		h.bad("id must be an entity id")
		return "", false
	}
	return homedash.EntityID(raw), true
}

// intParam reads an optional integer query parameter within lo..hi.
func (h *hdRequest) intParam(name string, def, lo, hi int) (int, bool) {
	raw := h.r.URL.Query().Get(name)
	if raw == "" {
		return def, true
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < lo || n > hi {
		h.bad(name + " is out of range")
		return 0, false
	}
	return n, true
}

// homeDashPrincipal identifies the caller for rate limits, dedupe and stream ownership: a token
// digest, the session nonce digest (stable across session renewal), or the local desktop when login
// is off. The GET stream and its PUT subscriptions resolve to the same key in one browser.
func homeDashPrincipal(r *http.Request) homedash.Principal {
	key := "local-desktop"
	if token, bearer := bearerCredential(r.Header.Get("Authorization")); bearer && token != "" {
		key = "t:" + homeDashDigest(token)
	} else if c, err := r.Cookie(sessionCookieName); err == nil && c.Value != "" {
		key = "s:" + homeDashDigest(sessionNonce(c.Value))
	}
	return homedash.Principal{Kind: homedash.PrincipalSession, Key: key}
}

func homeDashDigest(v string) string {
	sum := sha256.Sum256([]byte(v))
	return hex.EncodeToString(sum[:16])
}

func (h *hdRequest) bootstrap() {
	hd, cat := h.cfg.HomeDash, h.svc.cat
	homeDashWriteOK(h.w, http.StatusOK, map[string]any{
		"api_version": homeDashAPIVersion, "stream_protocol": homeDashStreamProtocol,
		"schema_version": homedash.SchemaVersion, "catalog_hash": cat.Hash,
		"readonly": hd.ReadOnly, "desktop_readonly": h.cfg.VirtualDesktop.ReadOnly, "kiosk_enabled": hd.Kiosk.Enabled,
		"limits": map[string]any{
			"dashboards": hd.Limits.MaxDashboards, "subscriptions": hd.Limits.MaxSubscriptions,
			"pages": cat.Limits.Pages, "widgets_per_page": cat.Limits.WidgetsPerPage, "widgets": cat.Limits.Widgets,
			"document_bytes": cat.Limits.DocumentBytes, "history_points": homedash.MaxHistoryPoints,
			"history_range_seconds": int(homedash.MaxHistoryRange / time.Second),
		},
		"templates": []string{homeDashTemplateSystem},
		"sources":   h.svc.hub.Sources(),
	})
}

func (h *hdRequest) widgetCatalog() {
	cat := h.svc.cat
	h.w.Header().Set("ETag", `"`+cat.Hash+`"`)
	if inm := h.r.Header.Get("If-None-Match"); inm != "" && strings.Contains(inm, cat.Hash) {
		h.w.WriteHeader(http.StatusNotModified)
		return
	}
	homeDashWriteOK(h.w, http.StatusOK, map[string]any{"catalog": cat})
}
```

  `homeDashStreamProtocol` is declared in Task 10's file; until then declare it here as `homeDashStreamProtocol = 1` inside the first const block and **move** it in Task 10 only if you prefer (keep exactly one declaration).
- [ ] GitNexus impact (R2) for `Server.run` (route registration) and `RouteContractManifest`.
- [ ] `server_routes.go`: after `mux.HandleFunc("/api/desktop/chibi-sensei/", handleChibiSensei(s))` add `mux.HandleFunc("/api/desktop/homedash/", handleHomeDash(s))`.
- [ ] `audit_manifests.go`: directly before the `/api/desktop/local-wikipedia/` row add

```go
		// HomeDash desktop API: dashboards CRUD (DELETE), commands, SSE stream, camera snapshots (internal/server/homedash_routes.go).
		{Pattern: "/api/desktop/homedash/", Methods: []string{"GET", "POST", "PUT", "DELETE"}, Auth: "session-or-desktop-admin-token", Category: "homedash", ContentTypes: []string{"application/json", "text/event-stream", "image/jpeg"}},
```

- [ ] Run both test commands. Expected: PASS. Then `go test ./internal/audit/... -count=1` (expected PASS; compare with the Task 0 baseline).
- [ ] detect-changes; commit the six paths with message `feat(server): HomeDash route table, envelope, bootstrap and widget catalog`.

**Done when:** the five route tests, the manifest test and the audit package pass.

---

### Task 7a: Save pipeline, starter template, create/list/get/update/delete

**Files:** create `internal/server/homedash_template.go`, `internal/server/homedash_api_dashboards.go`, `internal/server/homedash_api_dashboards_test.go`; modify `internal/server/homedash_routes.go` (rows).

**Read first:** contract "Shared JSON types", "Starter template answer", dashboard route examples; `internal/homedash/dashboard.go:194-282` (`DecodeDashboard`, `Normalize`); `internal/homedash/dashboard_layout.go:89-129` (`Autopack`); `internal/homedash/dashboard_validate.go:95-159,271-378`; `internal/homedash/dashboard_migrate.go`; `internal/homedash/store_dashboards.go` (Create/Update/Delete/Get/List, `slug_taken` at `:49-54`); `internal/homedash/catalog_widgets.go:110-120,207-211` (gauge/sensor/entity variants, options, bindings).

- [ ] Write `internal/server/homedash_api_dashboards_test.go`:

```go
package server

import (
	"fmt"
	"net/http"
	"slices"
	"testing"

	"aurago/internal/config"
	"aurago/internal/homedash"
)

type hdSaved struct {
	Dashboard homedash.Dashboard `json:"dashboard"`
	Warnings  []homedash.Issue   `json:"warnings"`
}

func minimalDoc(name string) map[string]any {
	return map[string]any{"name": name, "pages": []any{map[string]any{"id": "main", "name": "Main"}}}
}

func TestHomeDashDashboardCRUDAndRevisionConflict(t *testing.T) {
	env := newHomeDashTestEnv(t, nil)
	w := env.do(t, env.admin, http.MethodPost, "dashboards", map[string]any{"document": minimalDoc("Living Room")})
	if w.Code != http.StatusCreated {
		t.Fatalf("create = %d %s", w.Code, w.Body.String())
	}
	var c hdSaved
	hdDecode(t, w, &c)
	d := c.Dashboard
	if d.Slug != "living-room" || d.Revision != 1 || d.ApprovedRevision != 1 || d.SavedBy != "user" || d.Look.Family != "glass" || d.Kiosk == nil {
		t.Fatalf("created = %+v", d)
	}
	path := "dashboard?id=" + d.ID

	if w := env.do(t, env.admin, http.MethodGet, "dashboards", nil); w.Code != 200 {
		t.Fatalf("list = %d", w.Code)
	} else {
		var l struct{ Dashboards []homedash.DashboardSummary }
		hdDecode(t, w, &l)
		if len(l.Dashboards) != 1 || l.Dashboards[0].ID != d.ID {
			t.Fatalf("list = %+v", l)
		}
	}
	if w := env.do(t, env.admin, http.MethodGet, path, nil); w.Code != 200 {
		t.Fatalf("get = %d %s", w.Code, w.Body.String())
	}

	d.Name = "Lounge"
	w = env.do(t, env.admin, http.MethodPut, path, map[string]any{"revision": 1, "document": d})
	var u hdSaved
	hdDecode(t, w, &u)
	if w.Code != 200 || u.Dashboard.Revision != 2 || u.Dashboard.Name != "Lounge" || u.Dashboard.ApprovedRevision != 2 {
		t.Fatalf("update = %d %s", w.Code, w.Body.String())
	}
	w = env.do(t, env.admin, http.MethodPut, path, map[string]any{"revision": 1, "document": d})
	var conflict struct {
		Code            string `json:"code"`
		CurrentRevision int64  `json:"current_revision"`
	}
	hdDecode(t, w, &conflict)
	if w.Code != http.StatusConflict || conflict.Code != "revision_conflict" || conflict.CurrentRevision != 2 {
		t.Fatalf("stale update = %d %s", w.Code, w.Body.String())
	}
	d.Pages = nil
	w = env.do(t, env.admin, http.MethodPut, path, map[string]any{"revision": 2, "document": d})
	var invalid struct {
		Code   string           `json:"code"`
		Issues []homedash.Issue `json:"issues"`
	}
	hdDecode(t, w, &invalid)
	if w.Code != http.StatusUnprocessableEntity || invalid.Code != "invalid_document" ||
		!slices.ContainsFunc(invalid.Issues, func(i homedash.Issue) bool { return i.Path == "/pages" }) {
		t.Fatalf("invalid update = %d %s", w.Code, w.Body.String())
	}

	w = env.do(t, env.admin, http.MethodPost, "dashboards", map[string]any{"document": minimalDoc("Living Room")})
	var second hdSaved
	hdDecode(t, w, &second)
	if second.Dashboard.Slug != "living-room-2" {
		t.Fatalf("second slug = %q", second.Dashboard.Slug)
	}
	taken := minimalDoc("Other")
	taken["slug"] = "living-room"
	if w := env.do(t, env.admin, http.MethodPost, "dashboards", map[string]any{"document": taken}); w.Code != http.StatusConflict || hdCode(t, w) != "slug_taken" {
		t.Fatalf("taken slug = %d %s", w.Code, w.Body.String())
	}
	if w := env.do(t, env.admin, http.MethodPost, "dashboards", `{"document":{"name":"X","bogus":1}}`); w.Code != http.StatusUnprocessableEntity || hdCode(t, w) != "invalid_document" {
		t.Fatalf("unknown field = %d %s", w.Code, w.Body.String())
	}

	if w := env.do(t, env.admin, http.MethodDelete, path+"&revision=1", nil); w.Code != http.StatusConflict {
		t.Fatalf("stale delete = %d", w.Code)
	}
	if w := env.do(t, env.admin, http.MethodDelete, path+"&revision=2", nil); w.Code != 200 {
		t.Fatalf("delete = %d %s", w.Code, w.Body.String())
	}
	if w := env.do(t, env.admin, http.MethodGet, path, nil); w.Code != http.StatusNotFound || hdCode(t, w) != "dashboard_not_found" {
		t.Fatalf("get deleted = %d %s", w.Code, w.Body.String())
	}
	if w := env.do(t, env.admin, http.MethodGet, "dashboard?id=nope", nil); w.Code != http.StatusBadRequest {
		t.Fatalf("bad id = %d", w.Code)
	}
}

func TestHomeDashStarterTemplate(t *testing.T) {
	env := newHomeDashTestEnv(t, nil) // no source registered: every binding warns entity_unknown
	w := env.do(t, env.admin, http.MethodPost, "dashboards", map[string]any{"template": "system"})
	if w.Code != http.StatusCreated {
		t.Fatalf("template = %d %s", w.Code, w.Body.String())
	}
	var c hdSaved
	hdDecode(t, w, &c)
	d := c.Dashboard
	if d.Name != "System" || d.Slug != "system" || d.Look.Family != "glass" || d.Look.Palette.Mode != "theme" ||
		d.Look.RadiusScale != 1 || !d.Grid.RowUnit.Auto || len(d.Pages) != 1 || d.ApprovedRevision != 1 {
		t.Fatalf("template dashboard = %+v", d)
	}
	got := []string{}
	for _, wd := range d.Pages[0].Widgets {
		got = append(got, fmt.Sprintf("%s:%s/%s=%s", wd.ID, wd.Type, wd.Variant, wd.Bindings["primary"]))
	}
	want := []string{"cpu:gauge/arc=system:cpu", "memory:gauge/needle=system:memory", "disk:gauge/arc=system:disk",
		"net-rx:sensor/sparkline=system:net_rx", "net-tx:sensor/sparkline=system:net_tx", "uptime:entity/tile=system:uptime"}
	if !slices.Equal(got, want) {
		t.Fatalf("widgets = %v", got)
	}
	wantLG := []homedash.LayoutItem{{ID: "cpu", X: 0, Y: 0, W: 4, H: 4}, {ID: "memory", X: 4, Y: 0, W: 4, H: 4}, {ID: "disk", X: 8, Y: 0, W: 4, H: 4},
		{ID: "net-rx", X: 0, Y: 4, W: 5, H: 2}, {ID: "net-tx", X: 5, Y: 4, W: 5, H: 2}, {ID: "uptime", X: 10, Y: 4, W: 2, H: 2}}
	if !slices.Equal(d.Pages[0].Layouts.LG, wantLG) {
		t.Fatalf("layout = %+v", d.Pages[0].Layouts.LG)
	}
	if len(c.Warnings) != 6 || c.Warnings[0].Code != "entity_unknown" {
		t.Fatalf("warnings = %+v", c.Warnings)
	}
	w = env.do(t, env.admin, http.MethodPost, "dashboards", map[string]any{"template": "system", "name": "Server"})
	hdDecode(t, w, &c)
	if c.Dashboard.Slug != "system-2" || c.Dashboard.Name != "Server" {
		t.Fatalf("second template = %+v", c.Dashboard)
	}
	for _, body := range []any{map[string]any{"template": "nope"}, map[string]any{"template": "system", "document": minimalDoc("A")}, map[string]any{}} {
		if w := env.do(t, env.admin, http.MethodPost, "dashboards", body); w.Code != http.StatusBadRequest {
			t.Fatalf("%v = %d %s", body, w.Code, w.Body.String())
		}
	}
}

func TestHomeDashDashboardWritesRefusedOnReadOnlyDesktop(t *testing.T) {
	env := newHomeDashTestEnv(t, func(c *config.Config) { c.VirtualDesktop.ReadOnly = true })
	if w := env.do(t, env.admin, http.MethodPost, "dashboards", map[string]any{"template": "system"}); w.Code != http.StatusForbidden || hdCode(t, w) != "desktop_readonly" {
		t.Fatalf("create on a read-only desktop = %d %s", w.Code, w.Body.String())
	}
	if w := env.do(t, env.admin, http.MethodGet, "dashboards", nil); w.Code != http.StatusOK {
		t.Fatalf("list on a read-only desktop = %d", w.Code)
	}
}
```

- [ ] Run `go test ./internal/server -count=1 -run '(?i)(TestHomeDashDashboardCRUD|TestHomeDashStarterTemplate|TestHomeDashDashboardWritesRefused)'`. Expected: FAIL (`404 not_found` from the route table).
- [ ] Create `internal/server/homedash_template.go`:

```go
package server

import "aurago/internal/homedash"

// homeDashStarterSystem builds the "system" starter dashboard (decision D3): hero variants only,
// family glass, theme palette, square 4×4 gauges over a 2-row band. Look.RadiusScale and
// Grid.RowUnit are set explicitly (Go-built documents, design section 12.3). The caller runs the
// save pipeline.
func homeDashStarterSystem(name string) homedash.Dashboard {
	if name == "" {
		name = "System"
	}
	gauge := func(id, variant, entity string, warn float64) homedash.Widget {
		return homedash.Widget{ID: id, Type: "gauge", Variant: variant, Bindings: map[string]string{"primary": entity},
			Options: map[string]any{"min": 0.0, "max": 100.0, "warn": warn, "alert": 95.0, "decimals": 0.0}}
	}
	spark := func(id, entity string) homedash.Widget {
		return homedash.Widget{ID: id, Type: "sensor", Variant: "sparkline", Bindings: map[string]string{"primary": entity},
			Options: map[string]any{"decimals": 1.0, "hours": 1.0}}
	}
	compact := true
	return homedash.Dashboard{
		Name: name,
		Look: homedash.Look{Family: "glass", Palette: homedash.Palette{Mode: "theme"}, Backdrop: homedash.Backdrop{Kind: "theme"},
			Density: "comfortable", RadiusScale: 1},
		Grid: homedash.Grid{RowUnit: homedash.RowUnit{Auto: true}, Fit: "scroll"},
		Pages: []homedash.Page{{
			ID: "main", Name: "Main", Compact: &compact,
			Widgets: []homedash.Widget{
				gauge("cpu", "arc", "system:cpu", 80),
				gauge("memory", "needle", "system:memory", 85),
				gauge("disk", "arc", "system:disk", 85),
				spark("net-rx", "system:net_rx"),
				spark("net-tx", "system:net_tx"),
				{ID: "uptime", Type: "entity", Variant: "tile", Bindings: map[string]string{"primary": "system:uptime"}, Options: map[string]any{}},
			},
			Layouts: homedash.Layouts{LG: []homedash.LayoutItem{
				{ID: "cpu", X: 0, Y: 0, W: 4, H: 4}, {ID: "memory", X: 4, Y: 0, W: 4, H: 4}, {ID: "disk", X: 8, Y: 0, W: 4, H: 4},
				{ID: "net-rx", X: 0, Y: 4, W: 5, H: 2}, {ID: "net-tx", X: 5, Y: 4, W: 5, H: 2}, {ID: "uptime", X: 10, Y: 4, W: 2, H: 2},
			}},
		}},
	}
}
```

- [ ] Create `internal/server/homedash_api_dashboards.go` (Task 7b appends to it):

```go
package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"aurago/internal/homedash"
)

// prepare is the save pipeline every save path shares (decision D4): Migrate, Normalize, Autopack,
// Validate. Issues refuse the save as invalid_document; warnings pass. ApplyPatch never saves alone.
func (svc *homeDashService) prepare(ctx context.Context, doc *homedash.Dashboard) (homedash.Report, error) {
	if err := homedash.Migrate(doc); err != nil {
		return homedash.Report{}, err
	}
	homedash.Normalize(doc)
	homedash.Autopack(doc, svc.cat)
	known := svc.known(ctx, *doc)
	rep := homedash.Validate(*doc, svc.cat, func(id homedash.EntityID) (homedash.Entity, bool) {
		e, ok := known[id]
		return e, ok
	})
	if !rep.OK() {
		return rep, &homedash.Error{Code: homedash.CodeInvalidDocument, Message: "the dashboard is not valid", Issues: rep.Issues}
	}
	return rep, nil
}

// known looks up the bound entities (catalogs fetched when stale, no source warmed) so Validate can
// warn about unknown or unfitting entities.
func (svc *homeDashService) known(ctx context.Context, doc homedash.Dashboard) map[homedash.EntityID]homedash.Entity {
	ids := homeDashBoundIDs(doc)
	if len(ids) == 0 {
		return map[homedash.EntityID]homedash.Entity{}
	}
	lctx, cancel := context.WithTimeout(ctx, homeDashCatalogWait)
	defer cancel()
	return svc.hub.LookupEntities(lctx, ids)
}

// homeDashBoundIDs lists the distinct syntactically valid entity ids a document binds.
func homeDashBoundIDs(doc homedash.Dashboard) []homedash.EntityID {
	seen := map[homedash.EntityID]bool{}
	var ids []homedash.EntityID
	for _, p := range doc.Pages {
		for _, w := range p.Widgets {
			for _, raw := range w.Bindings {
				if _, _, err := homedash.ParseEntityID(raw); err != nil {
					continue
				}
				if id := homedash.EntityID(raw); !seen[id] {
					seen[id] = true
					ids = append(ids, id)
				}
			}
		}
	}
	return ids
}

// saveNew stores doc as a new dashboard (a user save, approved). An empty slug is derived from
// slugBase (the name when empty) and made unique; an explicit slug is kept as given.
func (svc *homeDashService) saveNew(ctx context.Context, doc homedash.Dashboard, slugBase string) (homedash.Dashboard, homedash.Report, error) {
	doc.ID, doc.Revision, doc.ApprovedRevision, doc.UpdatedAt, doc.SavedBy = "", 0, 0, time.Time{}, ""
	if doc.Slug == "" {
		if slugBase == "" {
			slugBase = doc.Name
		}
		slug, err := svc.freeSlug(ctx, homeDashSlug(slugBase))
		if err != nil {
			return homedash.Dashboard{}, homedash.Report{}, err
		}
		doc.Slug = slug
	}
	rep, err := svc.prepare(ctx, &doc)
	if err != nil {
		return homedash.Dashboard{}, rep, err
	}
	saved, err := svc.store.CreateDashboard(ctx, doc, homedash.SavedByUser)
	if err != nil {
		return homedash.Dashboard{}, rep, err
	}
	svc.hub.DashboardChanged(saved.ID, saved.Revision, saved.ApprovedRevision)
	return saved, rep, nil
}

// saveExisting stores doc as the next revision of id when revision is still current (a user save,
// approved). Server-owned fields of doc are ignored.
func (svc *homeDashService) saveExisting(ctx context.Context, id string, revision int64, doc homedash.Dashboard) (homedash.Dashboard, homedash.Report, error) {
	doc.ID, doc.Revision, doc.ApprovedRevision, doc.UpdatedAt, doc.SavedBy = id, 0, 0, time.Time{}, ""
	rep, err := svc.prepare(ctx, &doc)
	if err != nil {
		return homedash.Dashboard{}, rep, err
	}
	saved, err := svc.store.UpdateDashboard(ctx, doc, revision, homedash.SavedByUser)
	if err != nil {
		return homedash.Dashboard{}, rep, err
	}
	svc.hub.DashboardChanged(saved.ID, saved.Revision, saved.ApprovedRevision)
	return saved, rep, nil
}

// freeSlug returns base, or base-2 … base-99, whichever no dashboard uses yet. The store's unique
// index still refuses a race with 409 slug_taken.
func (svc *homeDashService) freeSlug(ctx context.Context, base string) (string, error) {
	list, err := svc.store.ListDashboards(ctx)
	if err != nil {
		return "", err
	}
	taken := make(map[string]bool, len(list))
	for _, d := range list {
		taken[d.Slug] = true
	}
	if !taken[base] {
		return base, nil
	}
	for n := 2; n < 100; n++ {
		if candidate := base + "-" + strconv.Itoa(n); !taken[candidate] {
			return candidate, nil
		}
	}
	return "", &homedash.Error{Code: homedash.CodeSlugTaken, Message: "no free slug for this name"}
}

// homeDashSlug derives a slug base: lower-case ASCII letters and digits, every other run of
// characters one dash, at most 44 characters (room for "-99"), "dashboard" when nothing is left.
func homeDashSlug(text string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(text) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			if dash && b.Len() > 0 {
				b.WriteByte('-')
			}
			dash = false
			b.WriteRune(r)
		} else {
			dash = true
		}
		if b.Len() >= 44 {
			break
		}
	}
	s := b.String()
	if len(s) > 44 {
		s = s[:44]
	}
	if s = strings.Trim(s, "-"); s == "" {
		return "dashboard"
	}
	return s
}

func (h *hdRequest) listDashboards() {
	list, err := h.svc.store.ListDashboards(h.r.Context())
	if err != nil {
		h.fail(err)
		return
	}
	homeDashWriteOK(h.w, http.StatusOK, map[string]any{"dashboards": list})
}

type homeDashCreateRequest struct {
	Document json.RawMessage `json:"document"`
	Template string          `json:"template"`
	Name     string          `json:"name"`
}

func (h *hdRequest) createDashboard() {
	var req homeDashCreateRequest
	if !h.decode(&req, homeDashDocumentBody) {
		return
	}
	var doc homedash.Dashboard
	slugBase := ""
	switch {
	case len(req.Document) > 0 && req.Template == "" && req.Name == "":
		d, err := homedash.DecodeDashboard(req.Document)
		if err != nil {
			h.fail(err)
			return
		}
		doc = d
	case len(req.Document) == 0 && req.Template == homeDashTemplateSystem:
		doc, slugBase = homeDashStarterSystem(req.Name), homeDashTemplateSystem
	case len(req.Document) == 0 && req.Template != "":
		h.bad("unknown template")
		return
	default:
		h.bad("send either a document or a template")
		return
	}
	saved, rep, err := h.svc.saveNew(h.r.Context(), doc, slugBase)
	if err != nil {
		h.fail(err)
		return
	}
	homeDashWriteOK(h.w, http.StatusCreated, map[string]any{"dashboard": saved, "warnings": rep.Warnings})
}

func (h *hdRequest) getDashboard() {
	id, ok := h.dashboardID()
	if !ok {
		return
	}
	d, err := h.svc.store.GetDashboard(h.r.Context(), id)
	var he *homedash.Error
	if errors.As(err, &he) && he.Code == homedash.CodeSchemaTooNew {
		homeDashWriteOK(h.w, http.StatusOK, map[string]any{"dashboard": d, "schema_too_new": true})
		return
	}
	if err != nil {
		h.fail(err)
		return
	}
	homeDashWriteOK(h.w, http.StatusOK, map[string]any{"dashboard": d, "schema_too_new": false})
}

type homeDashUpdateRequest struct {
	Revision int64           `json:"revision"`
	Document json.RawMessage `json:"document"`
}

func (h *hdRequest) updateDashboard() {
	id, ok := h.dashboardID()
	if !ok {
		return
	}
	var req homeDashUpdateRequest
	if !h.decode(&req, homeDashDocumentBody) {
		return
	}
	if req.Revision < 1 || len(req.Document) == 0 {
		h.bad("revision and document are required")
		return
	}
	doc, err := homedash.DecodeDashboard(req.Document)
	if err != nil {
		h.fail(err)
		return
	}
	saved, rep, err := h.svc.saveExisting(h.r.Context(), id, req.Revision, doc)
	if err != nil {
		h.fail(err)
		return
	}
	homeDashWriteOK(h.w, http.StatusOK, map[string]any{"dashboard": saved, "warnings": rep.Warnings})
}

func (h *hdRequest) deleteDashboard() {
	id, ok := h.dashboardID()
	if !ok {
		return
	}
	rev, ok := h.revisionParam()
	if !ok {
		return
	}
	devices, err := h.svc.store.DeleteDashboard(h.r.Context(), id, rev)
	if err != nil {
		h.fail(err)
		return
	}
	for _, dev := range devices {
		h.svc.hub.RevokeDevice(dev, homedash.CloseDeviceRevoked)
	}
	h.svc.hub.DashboardChanged(id, 0, 0)
	homeDashWriteOK(h.w, http.StatusOK, map[string]any{"id": id})
}
```

- [ ] Add rows to `homeDashRoutes` in `homedash_routes.go`:

```go
	"dashboards": {http.MethodGet: {desktopRead, (*hdRequest).listDashboards}, http.MethodPost: {desktopWrite, (*hdRequest).createDashboard}},
	"dashboard": {http.MethodGet: {desktopRead, (*hdRequest).getDashboard}, http.MethodPut: {desktopWrite, (*hdRequest).updateDashboard},
		http.MethodDelete: {desktopWrite, (*hdRequest).deleteDashboard}},
```

- [ ] Run the Task 7a test command. Expected: 3 PASS.
- [ ] detect-changes; commit the four paths with message `feat(server): HomeDash save pipeline, starter template and dashboard CRUD`.

**Done when:** the three tests pass.

---

### Task 7b: Duplicate, revisions, restore, approve, validate

**Files:** modify `internal/server/homedash_api_dashboards.go`, `internal/server/homedash_routes.go`, `internal/server/homedash_api_dashboards_test.go`.

**Read first:** Task 7a code; `internal/homedash/store_dashboards.go:161-299` (`Approve`, `Revisions`, `GetRevision`, `revision_not_found`).

- [ ] Append to `internal/server/homedash_api_dashboards_test.go`:

```go
func TestHomeDashDuplicateRestoreApproveRevisionsValidate(t *testing.T) {
	env := newHomeDashTestEnv(t, nil)
	var c hdSaved
	hdDecode(t, env.do(t, env.admin, http.MethodPost, "dashboards", map[string]any{"document": minimalDoc("A")}), &c)
	d := c.Dashboard
	q := "?id=" + d.ID
	d.Name = "B"
	if w := env.do(t, env.admin, http.MethodPut, "dashboard"+q, map[string]any{"revision": 1, "document": d}); w.Code != 200 {
		t.Fatalf("update = %d %s", w.Code, w.Body.String())
	}
	var revs struct{ Revisions []homedash.RevisionInfo }
	hdDecode(t, env.do(t, env.admin, http.MethodGet, "dashboard/revisions"+q, nil), &revs)
	if len(revs.Revisions) != 2 || revs.Revisions[0].Revision != 2 || revs.Revisions[1].Revision != 1 {
		t.Fatalf("revisions = %+v", revs)
	}
	w := env.do(t, env.admin, http.MethodPost, "dashboard/restore"+q+"&revision=1", nil)
	var r hdSaved
	hdDecode(t, w, &r)
	if w.Code != 200 || r.Dashboard.Revision != 3 || r.Dashboard.Name != "A" || r.Dashboard.ApprovedRevision != 3 {
		t.Fatalf("restore = %d %s", w.Code, w.Body.String())
	}
	w = env.do(t, env.admin, http.MethodPost, "dashboard/approve"+q+"&revision=2", nil)
	var a hdSaved
	hdDecode(t, w, &a)
	if w.Code != 200 || a.Dashboard.ApprovedRevision != 2 || a.Dashboard.Revision != 3 {
		t.Fatalf("approve = %d %s", w.Code, w.Body.String())
	}
	for _, p := range []string{"dashboard/approve", "dashboard/restore"} {
		if w := env.do(t, env.admin, http.MethodPost, p+q+"&revision=99", nil); w.Code != http.StatusNotFound || hdCode(t, w) != "revision_not_found" {
			t.Fatalf("%s 99 = %d %s", p, w.Code, w.Body.String())
		}
	}
	w = env.do(t, env.admin, http.MethodPost, "dashboard/duplicate"+q, nil)
	var dup hdSaved
	hdDecode(t, w, &dup)
	if w.Code != http.StatusCreated || dup.Dashboard.Name != "A (copy)" || dup.Dashboard.Slug != "a-copy" || dup.Dashboard.Revision != 1 || dup.Dashboard.ID == d.ID {
		t.Fatalf("duplicate = %d %s", w.Code, w.Body.String())
	}
	hdDecode(t, env.do(t, env.admin, http.MethodPost, "dashboard/duplicate"+q, map[string]any{"name": "Second"}), &dup)
	if dup.Dashboard.Name != "Second" || dup.Dashboard.Slug != "a-copy-2" {
		t.Fatalf("named duplicate = %+v", dup.Dashboard)
	}
	var v struct {
		Valid    bool               `json:"valid"`
		Issues   []homedash.Issue   `json:"issues"`
		Document homedash.Dashboard `json:"document"`
	}
	hdDecode(t, env.do(t, env.admin, http.MethodPost, "dashboards/validate", map[string]any{"document": minimalDoc("V")}), &v)
	if !v.Valid || v.Document.Slug != "v" || v.Document.Kiosk == nil || v.Document.Kiosk.StartPage != "main" {
		t.Fatalf("validate valid = %+v", v)
	}
	hdDecode(t, env.do(t, env.admin, http.MethodPost, "dashboards/validate", map[string]any{"document": map[string]any{"name": "V"}}), &v)
	if v.Valid || !slices.ContainsFunc(v.Issues, func(i homedash.Issue) bool { return i.Path == "/pages" }) {
		t.Fatalf("validate invalid = %+v", v)
	}
}

func TestHomeDashValidateWorksOnReadOnlyDesktop(t *testing.T) {
	env := newHomeDashTestEnv(t, func(c *config.Config) { c.VirtualDesktop.ReadOnly = true })
	if w := env.do(t, env.admin, http.MethodPost, "dashboards/validate", map[string]any{"document": minimalDoc("V")}); w.Code != http.StatusOK {
		t.Fatalf("validate on a read-only desktop = %d %s", w.Code, w.Body.String())
	}
	if w := env.do(t, env.admin, http.MethodPost, "dashboard/duplicate?id=d_aaaaaaaaaaaa", nil); w.Code != http.StatusForbidden || hdCode(t, w) != "desktop_readonly" {
		t.Fatalf("duplicate on a read-only desktop = %d %s", w.Code, w.Body.String())
	}
}
```

- [ ] Run `go test ./internal/server -count=1 -run '(?i)(TestHomeDashDuplicateRestore|TestHomeDashValidateWorks)'`. Expected: FAIL (404 routes).
- [ ] Append to `internal/server/homedash_api_dashboards.go` (add `"unicode/utf8"` only if you use it; the code below uses runes):

```go
// homeDashCopyName appends " (copy)" and keeps the result within 80 runes.
func homeDashCopyName(name string) string {
	const suffix = " (copy)"
	runes := []rune(name)
	if limit := 80 - len([]rune(suffix)); len(runes) > limit {
		runes = runes[:limit]
	}
	return string(runes) + suffix
}

type homeDashDuplicateRequest struct {
	Name string `json:"name"`
}

func (h *hdRequest) duplicateDashboard() {
	id, ok := h.dashboardID()
	if !ok {
		return
	}
	var req homeDashDuplicateRequest
	if !h.decodeOptional(&req, homeDashSmallBody) {
		return
	}
	src, err := h.svc.store.GetDashboard(h.r.Context(), id)
	if err != nil {
		h.fail(err) // schema_too_new is refused (409): a copy would drop unknown fields
		return
	}
	base := src.Slug + "-copy"
	if req.Name != "" {
		src.Name = req.Name
	} else {
		src.Name = homeDashCopyName(src.Name)
	}
	src.Slug = ""
	saved, rep, err := h.svc.saveNew(h.r.Context(), src, base)
	if err != nil {
		h.fail(err)
		return
	}
	homeDashWriteOK(h.w, http.StatusCreated, map[string]any{"dashboard": saved, "warnings": rep.Warnings})
}

func (h *hdRequest) listRevisions() {
	id, ok := h.dashboardID()
	if !ok {
		return
	}
	list, err := h.svc.store.Revisions(h.r.Context(), id)
	if err != nil {
		h.fail(err)
		return
	}
	homeDashWriteOK(h.w, http.StatusOK, map[string]any{"revisions": list})
}

func (h *hdRequest) restoreDashboard() {
	id, ok := h.dashboardID()
	if !ok {
		return
	}
	rev, ok := h.revisionParam()
	if !ok {
		return
	}
	ctx := h.r.Context()
	old, err := h.svc.store.GetRevision(ctx, id, rev)
	if err != nil {
		h.fail(err)
		return
	}
	cur, err := h.svc.store.GetDashboard(ctx, id)
	if err != nil {
		h.fail(err)
		return
	}
	saved, rep, err := h.svc.saveExisting(ctx, id, cur.Revision, old)
	if err != nil {
		h.fail(err)
		return
	}
	homeDashWriteOK(h.w, http.StatusOK, map[string]any{"dashboard": saved, "warnings": rep.Warnings})
}

func (h *hdRequest) approveDashboard() {
	id, ok := h.dashboardID()
	if !ok {
		return
	}
	rev, ok := h.revisionParam()
	if !ok {
		return
	}
	ctx := h.r.Context()
	if _, err := h.svc.store.GetDashboard(ctx, id); err != nil {
		h.fail(err) // unknown dashboard or a newer schema: nothing approved
		return
	}
	d, err := h.svc.store.Approve(ctx, id, rev)
	if err != nil {
		h.fail(err)
		return
	}
	h.svc.hub.DashboardChanged(d.ID, d.Revision, d.ApprovedRevision)
	homeDashWriteOK(h.w, http.StatusOK, map[string]any{"dashboard": d})
}

type homeDashValidateRequest struct {
	Document json.RawMessage `json:"document"`
}

// validateDashboard runs the save pipeline without saving. An empty slug is derived from the name
// (uniqueness is checked only on create).
func (h *hdRequest) validateDashboard() {
	var req homeDashValidateRequest
	if !h.decode(&req, homeDashDocumentBody) {
		return
	}
	if len(req.Document) == 0 {
		h.bad("document is required")
		return
	}
	doc, err := homedash.DecodeDashboard(req.Document)
	if err != nil {
		h.fail(err)
		return
	}
	if doc.Slug == "" {
		doc.Slug = homeDashSlug(doc.Name)
	}
	rep, err := h.svc.prepare(h.r.Context(), &doc)
	var he *homedash.Error
	if err != nil && !(errors.As(err, &he) && he.Code == homedash.CodeInvalidDocument) {
		h.fail(err)
		return
	}
	homeDashWriteOK(h.w, http.StatusOK, map[string]any{"valid": rep.OK(), "issues": rep.Issues, "warnings": rep.Warnings, "document": doc})
}
```

- [ ] Add rows:

```go
	"dashboards/validate":  {http.MethodPost: {desktopRead, (*hdRequest).validateDashboard}},
	"dashboard/duplicate":  {http.MethodPost: {desktopWrite, (*hdRequest).duplicateDashboard}},
	"dashboard/revisions":  {http.MethodGet: {desktopRead, (*hdRequest).listRevisions}},
	"dashboard/restore":    {http.MethodPost: {desktopWrite, (*hdRequest).restoreDashboard}},
	"dashboard/approve":    {http.MethodPost: {desktopWrite, (*hdRequest).approveDashboard}},
```

- [ ] Run `go test ./internal/server -count=1 -run '(?i)(TestHomeDash(Dashboard|Starter|Duplicate|Validate))' -v`. Expected: 5 PASS.
- [ ] detect-changes; commit the three paths with message `feat(server): HomeDash duplicate, revisions, restore, approve and validate`.

**Done when:** the five dashboard tests pass.

---

### Task 8: Entities, entity, history, snapshot

**Files:** create `internal/server/homedash_api_entities.go`, `internal/server/homedash_api_entities_test.go`; modify `internal/server/homedash_routes.go`.

**Read first:** contract entity/history/snapshot examples; `internal/homedash/hub.go:303-421`; `internal/homedash/history.go:150-257` (ring records only subscribed numeric entities, keyed by `State.UpdatedAt`; `Series`); `internal/homedash/hub_sources.go:453-488` (updates older than the stored state are dropped).

- [ ] Write `internal/server/homedash_api_entities_test.go`:

```go
package server

import (
	"net/http"
	"net/url"
	"testing"
	"time"

	"aurago/internal/homedash"
	"aurago/internal/homedash/sourcetest"
)

func TestHomeDashEntitiesEntityHistorySnapshot(t *testing.T) {
	fake := sourcetest.New("fake",
		sourcetest.Entity("fake:temp", homedash.KindSensor, homedash.CapabilitySpec{Name: homedash.CapNumeric}),
		sourcetest.Entity("fake:lamp", homedash.KindLight))
	env := newHomeDashTestEnv(t, nil, fakeHomeDashSpec(fake))

	var list struct {
		Entities []homedash.Entity       `json:"entities"`
		Total    int                     `json:"total"`
		Sources  []homedash.SourceStatus `json:"sources"`
	}
	hdDecode(t, env.do(t, env.admin, http.MethodGet, "entities", nil), &list)
	if list.Total != 2 || list.Entities[0].ID != "fake:lamp" || list.Entities[1].ID != "fake:temp" || len(list.Sources) != 1 {
		t.Fatalf("entities = %+v", list)
	}
	hdDecode(t, env.do(t, env.admin, http.MethodGet, "entities?kind=light", nil), &list)
	if list.Total != 1 {
		t.Fatalf("kind filter = %+v", list)
	}
	for _, q := range []string{"entities?limit=0", "entities?limit=501", "entities?kind=nope", "entities?source=Bad"} {
		if w := env.do(t, env.admin, http.MethodGet, q, nil); w.Code != http.StatusBadRequest {
			t.Fatalf("%s = %d", q, w.Code)
		}
	}
	var one struct{ Entity homedash.Entity }
	hdDecode(t, env.do(t, env.admin, http.MethodGet, "entity?id=fake:temp", nil), &one)
	if one.Entity.Kind != homedash.KindSensor {
		t.Fatalf("entity = %+v", one)
	}
	if w := env.do(t, env.admin, http.MethodGet, "entity?id=fake:none", nil); w.Code != http.StatusNotFound || hdCode(t, w) != "entity_not_found" {
		t.Fatalf("unknown entity = %d %s", w.Code, w.Body.String())
	}
	if w := env.do(t, env.admin, http.MethodGet, "entity?id=", nil); w.Code != http.StatusBadRequest {
		t.Fatalf("empty id = %d", w.Code)
	}

	// History: the ring records only subscribed entities.
	sub, err := env.svc.hub.Attach(homedash.Principal{Kind: homedash.PrincipalSession, Key: "test"})
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Close()
	if _, _, err := sub.Set([]homedash.EntityID{"fake:temp"}); err != nil {
		t.Fatal(err)
	}
	sourcetest.WaitFor(t, 2*time.Second, func() bool { return fake.Running() == 1 }, "source running")
	base := time.Now().UTC().Add(2 * time.Minute).Truncate(time.Minute)
	for i, v := range []float64{20, 21, 22} {
		fake.Push(homedash.StateUpdate{Native: "temp", State: homedash.State{Value: v, Available: true, UpdatedAt: base.Add(time.Duration(2*i) * time.Minute)}})
	}
	from, to := base.Add(-time.Hour).Format(time.RFC3339), base.Add(time.Hour).Format(time.RFC3339)
	var hist struct {
		Origin string            `json:"origin"`
		Since  *time.Time        `json:"since"`
		Points []homedash.Sample `json:"points"`
	}
	hdDecode(t, env.do(t, env.admin, http.MethodGet, "history?id=fake:temp&from="+url.QueryEscape(from)+"&to="+url.QueryEscape(to), nil), &hist)
	if hist.Origin != "memory" || hist.Since == nil || len(hist.Points) != 3 || hist.Points[2].V != 22 {
		t.Fatalf("history = %+v", hist)
	}
	if w := env.do(t, env.admin, http.MethodGet, "history?id=fake:temp&points=501", nil); w.Code != http.StatusBadRequest {
		t.Fatalf("points 501 = %d", w.Code)
	}
	if w := env.do(t, env.admin, http.MethodGet, "snapshot?id=fake:temp", nil); w.Code != http.StatusBadRequest || hdCode(t, w) != "unsupported_command" {
		t.Fatalf("snapshot = %d %s", w.Code, w.Body.String())
	}
}
```

- [ ] Run `go test ./internal/server -count=1 -run TestHomeDashEntitiesEntityHistorySnapshot`. Expected: FAIL (404 routes).
- [ ] Create `internal/server/homedash_api_entities.go`:

```go
package server

import (
	"context"
	"net/http"
	"slices"
	"strconv"
	"time"
	"unicode/utf8"

	"aurago/internal/homedash"
)

func (h *hdRequest) listEntities() {
	q := h.r.URL.Query()
	f := homedash.Filter{Source: homedash.SourceID(q.Get("source")), Kind: homedash.Kind(q.Get("kind")), Area: q.Get("area"), Query: q.Get("q")}
	switch {
	case f.Source != "" && !homedash.ValidSourceID(string(f.Source)):
		h.bad("source must be a source id")
		return
	case f.Kind != "" && !slices.Contains(homedash.AllKinds(), f.Kind):
		h.bad("kind must be a catalog kind")
		return
	case utf8.RuneCountInString(f.Query) > 100 || utf8.RuneCountInString(f.Area) > 80:
		h.bad("q or area is too long")
		return
	}
	var ok bool
	if f.Limit, ok = h.intParam("limit", 100, 1, 500); !ok {
		return
	}
	if f.Offset, ok = h.intParam("offset", 0, 0, 1<<30); !ok {
		return
	}
	ctx, cancel := context.WithTimeout(h.r.Context(), homeDashCatalogWait)
	defer cancel()
	items, total, sources := h.svc.hub.Entities(ctx, f)
	homeDashWriteOK(h.w, http.StatusOK, map[string]any{"entities": items, "total": total, "limit": f.Limit, "offset": f.Offset, "sources": sources})
}

func (h *hdRequest) getEntity() {
	id, ok := h.entityID()
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(h.r.Context(), homeDashCatalogWait)
	defer cancel()
	e, err := h.svc.hub.Entity(ctx, id)
	if err != nil {
		h.fail(err)
		return
	}
	homeDashWriteOK(h.w, http.StatusOK, map[string]any{"entity": e})
}

func (h *hdRequest) history() {
	id, ok := h.entityID()
	if !ok {
		return
	}
	q := h.r.URL.Query()
	to := time.Now().UTC()
	if raw := q.Get("to"); raw != "" {
		t, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			h.bad("to must be an RFC 3339 time")
			return
		}
		to = t
	}
	from := to.Add(-24 * time.Hour)
	if raw := q.Get("from"); raw != "" {
		t, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			h.bad("from must be an RFC 3339 time")
			return
		}
		from = t
	}
	points, ok := h.intParam("points", 120, 1, homedash.MaxHistoryPoints)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(h.r.Context(), homeDashCatalogWait)
	defer cancel()
	series, err := h.svc.hub.History(ctx, id, from, to, points)
	if err != nil {
		h.fail(err)
		return
	}
	fields := map[string]any{"id": id, "origin": series.Origin, "points": series.Points}
	if series.Origin == "memory" {
		fields["since"] = series.Since.UTC()
	}
	homeDashWriteOK(h.w, http.StatusOK, fields)
}

func (h *hdRequest) snapshot() {
	id, ok := h.entityID()
	if !ok {
		return
	}
	width, ok := h.intParam("w", 640, 1, 1920)
	if !ok {
		return
	}
	height, ok := h.intParam("h", 360, 1, 1080)
	if !ok {
		return
	}
	img, ct, err := h.svc.hub.Snapshot(h.r.Context(), id, width, height)
	if err != nil {
		h.fail(err)
		return
	}
	switch ct {
	case "image/jpeg", "image/png", "image/webp":
	default:
		homeDashWriteError(h.w, http.StatusBadGateway, homedash.CodeSourceError, "the source sent no image", nil)
		return
	}
	hdr := h.w.Header()
	hdr.Set("Content-Type", ct)
	hdr.Set("Content-Length", strconv.Itoa(len(img)))
	hdr.Set("X-Content-Type-Options", "nosniff")
	hdr.Set("Cache-Control", "no-store")
	h.w.WriteHeader(http.StatusOK)
	_, _ = h.w.Write(img)
}
```

- [ ] Add rows:

```go
	"entities": {http.MethodGet: {desktopRead, (*hdRequest).listEntities}},
	"entity":   {http.MethodGet: {desktopRead, (*hdRequest).getEntity}},
	"history":  {http.MethodGet: {desktopRead, (*hdRequest).history}},
	"snapshot": {http.MethodGet: {desktopRead, (*hdRequest).snapshot}},
```

- [ ] Run the test command. Expected: PASS.
- [ ] detect-changes; commit the three paths with message `feat(server): HomeDash entity, history and snapshot routes`.

**Done when:** the test passes.

---

### Task 9: Commands route

**Files:** create `internal/server/homedash_api_commands.go`, `internal/server/homedash_api_commands_test.go`; modify `internal/server/homedash_routes.go`.

**Read first:** contract `POST commands`; `internal/homedash/command.go:116-185,283-396` (gate order, dedupe, audit outcomes); Task 5 `homeDashAudit`, `homeDashAuditKey`; `internal/server/desktop_remote_security.go:232-245`.

- [ ] Write `internal/server/homedash_api_commands_test.go`:

```go
package server

import (
	"net/http"
	"testing"

	"aurago/internal/config"
	"aurago/internal/homedash"
	"aurago/internal/homedash/sourcetest"
)

func homeDashCommandFake() *sourcetest.FakeSource {
	return sourcetest.New("fake",
		sourcetest.Entity("fake:a", homedash.KindSwitch, homedash.CapabilitySpec{Name: homedash.CapToggle, Allowed: true}),
		sourcetest.Entity("fake:s", homedash.KindSensor))
}

func TestHomeDashCommandsAcceptAuditAndRefuse(t *testing.T) {
	fake := homeDashCommandFake()
	env := newHomeDashTestEnv(t, nil, fakeHomeDashSpec(fake))
	cmd := func(body map[string]any) (int, string) {
		w := env.do(t, env.admin, http.MethodPost, "commands", body)
		if w.Code == http.StatusAccepted {
			return w.Code, ""
		}
		return w.Code, hdCode(t, w)
	}
	if code, _ := cmd(map[string]any{"entity": "fake:a", "command": "toggle", "request_id": "r1"}); code != http.StatusAccepted || len(fake.Commands()) != 1 {
		t.Fatalf("toggle = %d, %d source calls", code, len(fake.Commands()))
	}
	if !env.audits.has("fake:a|toggle|ok") {
		t.Fatalf("audit = %+v", env.audits.entries)
	}
	if code, _ := cmd(map[string]any{"entity": "fake:a", "command": "toggle", "request_id": "r1"}); code != http.StatusAccepted || len(fake.Commands()) != 1 {
		t.Fatalf("same request id = %d, %d source calls (dedupe)", code, len(fake.Commands()))
	}
	tests := []struct {
		body       map[string]any
		status     int
		code       string
		auditEntry string
	}{
		{map[string]any{"entity": "fake:a", "command": "toggle"}, 400, "bad_request", ""},
		{map[string]any{"entity": "fake:a", "command": "toggle", "request_id": "r2", "dashboard_id": "nope"}, 400, "bad_request", ""},
		{map[string]any{"entity": "fake:s", "command": "toggle", "request_id": "r3"}, 400, "unsupported_command", "fake:s|toggle|refused:unsupported_command"},
		{map[string]any{"entity": "fake:zz", "command": "toggle", "request_id": "r4"}, 404, "entity_not_found", "fake:zz|toggle|refused:entity_not_found"},
	}
	for _, tc := range tests {
		if status, code := cmd(tc.body); status != tc.status || code != tc.code {
			t.Errorf("%v = %d %s, want %d %s", tc.body, status, code, tc.status, tc.code)
		}
		if tc.auditEntry != "" && !env.audits.has(tc.auditEntry) {
			t.Errorf("missing audit %q in %+v", tc.auditEntry, env.audits.entries)
		}
	}
}

func TestHomeDashCommandsRefusedOnReadOnly(t *testing.T) {
	fake := homeDashCommandFake()
	env := newHomeDashTestEnv(t, func(c *config.Config) { c.VirtualDesktop.ReadOnly = true }, fakeHomeDashSpec(fake))
	w := env.do(t, env.admin, http.MethodPost, "commands", map[string]any{"entity": "fake:a", "command": "toggle", "request_id": "r1"})
	if w.Code != http.StatusForbidden || hdCode(t, w) != "desktop_readonly" || len(fake.Commands()) != 0 {
		t.Fatalf("read-only desktop = %d %s", w.Code, w.Body.String())
	}

	fake2 := homeDashCommandFake()
	env2 := newHomeDashTestEnv(t, func(c *config.Config) { c.HomeDash.ReadOnly = true }, fakeHomeDashSpec(fake2))
	w = env2.do(t, env2.admin, http.MethodPost, "commands", map[string]any{"entity": "fake:a", "command": "toggle", "request_id": "r1"})
	var body struct {
		Code      string `json:"code"`
		BlockedBy string `json:"blocked_by"`
	}
	hdDecode(t, w, &body)
	if w.Code != http.StatusForbidden || body.Code != "read_only" || body.BlockedBy != "homedash.readonly" || len(fake2.Commands()) != 0 {
		t.Fatalf("homedash.readonly = %d %s", w.Code, w.Body.String())
	}
}
```

- [ ] Run `go test ./internal/server -count=1 -run '(?i)(TestHomeDashCommands)'`. Expected: FAIL (404 route).
- [ ] Create `internal/server/homedash_api_commands.go`:

```go
package server

import (
	"context"
	"net/http"

	"aurago/internal/homedash"
)

type homeDashCommandRequest struct {
	Entity      string         `json:"entity"`
	Command     string         `json:"command"`
	Args        map[string]any `json:"args"`
	RequestID   string         `json:"request_id"`
	DashboardID string         `json:"dashboard_id"`
	WidgetID    string         `json:"widget_id"`
	Confirmed   bool           `json:"confirmed"`
}

// command runs the M0 pipeline (validate, gates, confirmation, rate limits, request-id dedupe,
// in-flight control, Source.Command, audit) for the session principal. A read-only desktop never
// gets here (the route is desktopWrite). 202 means the source accepted; the state follows on the stream.
func (h *hdRequest) command() {
	var req homeDashCommandRequest
	if !h.decode(&req, homeDashSmallBody) {
		return
	}
	switch {
	case req.Entity == "" || req.Command == "":
		h.bad("entity and command are required")
		return
	case req.RequestID == "" || len(req.RequestID) > 64:
		h.bad("request_id must be 1 to 64 characters")
		return
	case req.DashboardID != "" && !homedash.ValidDashboardID(req.DashboardID):
		h.bad("dashboard_id must be a dashboard id")
		return
	case req.WidgetID != "" && !homedash.ValidWidgetID(req.WidgetID):
		h.bad("widget_id must be a widget id")
		return
	}
	if req.Args == nil {
		req.Args = map[string]any{}
	}
	ctx := context.WithValue(h.r.Context(), homeDashAuditKey{}, desktopAuditRequestInfo(h.s, h.r))
	res, err := h.svc.hub.Command(ctx, homedash.Command{
		Entity: homedash.EntityID(req.Entity), Name: homedash.Capability(req.Command), Args: req.Args,
		Principal: homeDashPrincipal(h.r), RequestID: req.RequestID, Confirmed: req.Confirmed,
		DashboardID: req.DashboardID, WidgetID: req.WidgetID,
	})
	if err != nil {
		h.fail(err)
		return
	}
	fields := map[string]any{"accepted": true, "superseded": res.Superseded}
	if res.Expected != nil {
		fields["expected"] = res.Expected
	}
	homeDashWriteOK(h.w, http.StatusAccepted, fields)
}
```

- [ ] Add row `"commands": {http.MethodPost: {desktopWrite, (*hdRequest).command}},`.
- [ ] Run the test command with `-v`. Expected: 2 PASS.
- [ ] detect-changes; commit the three paths with message `feat(server): HomeDash commands route with desktop audit`.

**Done when:** both tests pass.

---

### Task 10: SSE stream and subscriptions

**Files:** create `internal/server/homedash_stream.go`, `internal/server/homedash_stream_test.go`; modify `internal/server/homedash_routes.go`.

**Read first:** contract "Stream protocol", "Reconnect rules"; `internal/server/docker_manager_stream.go:35-165`; `internal/server/docker_manager_operations.go:19-29,98-142` (`dmEventStream`: arm deadline, send, comment); `internal/httpstream/timeout.go` (outer wrapper renews budgets per write; `ResponseController.SetWriteDeadline` returns `ErrNotSupported` on recorders); `internal/server/sse.go:164-190` (`scrubSSEJSON`); `internal/server/desktop_handlers.go:289-304` (`desktopWSAuthorizationValid`); `internal/homedash/subscriber.go` (whole file: `Attach` errors, `Set`, `Ready`, `Drain`, `Batch`); `internal/homedash/hub.go:64-75` (close reasons).

- [ ] Write `internal/server/homedash_stream_test.go`:

```go
package server

import (
	"bufio"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/homedash"
	"aurago/internal/homedash/sourcetest"
)

type hdEvent struct{ name, data string }

// openHomeDashStream opens GET stream on a real HTTP server, waits for hello and returns its id.
func openHomeDashStream(t *testing.T, env *homeDashTestEnv, token string) (string, <-chan hdEvent, func()) {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc(homeDashAPIPrefix, handleHomeDash(env.s))
	srv := httptest.NewServer(mux)
	req, _ := http.NewRequest(http.MethodGet, srv.URL+homeDashAPIPrefix+"stream", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := srv.Client().Do(req)
	if err != nil || resp.StatusCode != http.StatusOK || resp.Header.Get("Content-Type") != "text/event-stream" {
		srv.Close()
		t.Fatalf("stream = %v %v", resp, err)
	}
	events := make(chan hdEvent, 64)
	go func() {
		defer close(events)
		sc := bufio.NewScanner(resp.Body)
		sc.Buffer(make([]byte, 0, 64<<10), 1<<20)
		var ev hdEvent
		for sc.Scan() {
			line := sc.Text()
			switch {
			case line == "":
				if ev.name != "" {
					events <- ev
				}
				ev = hdEvent{}
			case strings.HasPrefix(line, "event: "):
				ev.name = strings.TrimPrefix(line, "event: ")
			case strings.HasPrefix(line, "data: "):
				ev.data = strings.TrimPrefix(line, "data: ")
			}
		}
	}()
	hello := waitHomeDashEvent(t, events, "hello")
	var h struct {
		StreamID    string `json:"stream_id"`
		Protocol    int    `json:"protocol"`
		HeartbeatMS int64  `json:"heartbeat_ms"`
	}
	if err := json.Unmarshal([]byte(hello.data), &h); err != nil || len(h.StreamID) != 32 || h.Protocol != 1 || h.HeartbeatMS <= 0 {
		t.Fatalf("hello = %s", hello.data)
	}
	return h.StreamID, events, func() { resp.Body.Close(); srv.Close() }
}

func waitHomeDashEvent(t *testing.T, events <-chan hdEvent, name string) hdEvent {
	t.Helper()
	deadline := time.After(3 * time.Second)
	for {
		select {
		case ev, ok := <-events:
			if !ok {
				t.Fatalf("stream ended before %q", name)
			}
			if ev.name == name {
				return ev
			}
		case <-deadline:
			t.Fatalf("no %q event", name)
		}
	}
}

func shortenHomeDashStreamTimers(t *testing.T) {
	hb, re := homeDashHeartbeat, homeDashReauthEvery
	homeDashHeartbeat, homeDashReauthEvery = 100*time.Millisecond, 20*time.Millisecond
	t.Cleanup(func() { homeDashHeartbeat, homeDashReauthEvery = hb, re })
}

func TestHomeDashStreamHelloSnapshotStateHeartbeat(t *testing.T) {
	shortenHomeDashStreamTimers(t)
	fake := sourcetest.New("fake", sourcetest.Entity("fake:a", homedash.KindSensor, homedash.CapabilitySpec{Name: homedash.CapNumeric}))
	env := newHomeDashTestEnv(t, nil, fakeHomeDashSpec(fake))
	id, events, stop := openHomeDashStream(t, env, env.admin)
	defer stop()

	w := env.do(t, env.admin, http.MethodPut, "stream/subscriptions", map[string]any{"stream_id": id, "entities": []string{"fake:a", "bad id", "nope:x"}})
	var put struct {
		Accepted []string             `json:"accepted"`
		Rejected []homedash.Rejection `json:"rejected"`
	}
	hdDecode(t, w, &put)
	if w.Code != http.StatusOK || len(put.Accepted) != 1 || put.Accepted[0] != "fake:a" || len(put.Rejected) != 2 ||
		put.Rejected[0].Reason != "invalid_id" || put.Rejected[1].Reason != "unknown_source" {
		t.Fatalf("PUT = %d %s", w.Code, w.Body.String())
	}
	var snap struct {
		Entities []homedash.Entity       `json:"entities"`
		Sources  []homedash.SourceStatus `json:"sources"`
	}
	if err := json.Unmarshal([]byte(waitHomeDashEvent(t, events, "snapshot").data), &snap); err != nil {
		t.Fatal(err)
	}
	if len(snap.Entities) != 1 || snap.Entities[0].ID != "fake:a" || snap.Entities[0].Kind != homedash.KindSensor || snap.Sources == nil {
		t.Fatalf("snapshot = %+v (the catalog must be prefetched: no generic placeholder)", snap)
	}
	sourcetest.WaitFor(t, 2*time.Second, func() bool { return fake.Running() == 1 }, "source running")
	for _, v := range []float64{1, 2, 3} {
		fake.Push(homedash.StateUpdate{Native: "a", State: homedash.State{Value: v, Available: true}})
	}
	for got := 0.0; got != 3; {
		var st struct {
			Changes map[string]homedash.State `json:"changes"`
		}
		if err := json.Unmarshal([]byte(waitHomeDashEvent(t, events, "state").data), &st); err != nil {
			t.Fatal(err)
		}
		if v, ok := st.Changes["fake:a"].Value.(float64); ok {
			got = v
		}
	}
	waitHomeDashEvent(t, events, "hb")

	w = env.do(t, env.admin, http.MethodPost, "dashboards", map[string]any{"template": "system"})
	var created hdSaved
	hdDecode(t, w, &created)
	var changed homedash.DashboardEvent
	if err := json.Unmarshal([]byte(waitHomeDashEvent(t, events, "dashboard_changed").data), &changed); err != nil ||
		changed.ID != created.Dashboard.ID || changed.Revision != 1 || changed.ApprovedRevision != 1 {
		t.Fatalf("dashboard_changed = %+v %v", changed, err)
	}
}

func TestHomeDashStreamEndsWithRevoked(t *testing.T) {
	shortenHomeDashStreamTimers(t)
	fake := sourcetest.New("fake", sourcetest.Entity("fake:a", homedash.KindSensor))
	env := newHomeDashTestEnv(t, nil, fakeHomeDashSpec(fake))

	token, meta, err := env.s.TokenManager.Create("stream admin", []string{desktopScopeAdmin}, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, events, stop := openHomeDashStream(t, env, token)
	defer stop()
	if err := env.s.TokenManager.Delete(meta.ID); err != nil {
		t.Fatal(err)
	}
	if ev := waitHomeDashEvent(t, events, "revoked"); ev.data != `{"reason":"auth_lost"}` {
		t.Fatalf("revoked = %s", ev.data)
	}

	_, events2, stop2 := openHomeDashStream(t, env, env.admin)
	defer stop2()
	cfg := *env.s.ConfigSnapshot()
	cfg.HomeDash.Enabled = false
	env.s.replaceConfigSnapshot(&cfg)
	if ev := waitHomeDashEvent(t, events2, "revoked"); ev.data != `{"reason":"disabled"}` {
		t.Fatalf("revoked = %s", ev.data)
	}
}

func TestHomeDashSubscriptionsAndStreamLimits(t *testing.T) {
	fake := sourcetest.New("fake", sourcetest.Entity("fake:a", homedash.KindSensor))
	env := newHomeDashTestEnv(t, func(c *config.Config) {
		c.VirtualDesktop.ReadOnly = true // streams and subscriptions are reads
		c.HomeDash.Limits.MaxStreams = 4
	}, fakeHomeDashSpec(fake))
	if w := env.do(t, env.admin, http.MethodPut, "stream/subscriptions", map[string]any{"stream_id": "00000000000000000000000000000000", "entities": []string{}}); w.Code != http.StatusNotFound || hdCode(t, w) != "stream_not_found" {
		t.Fatalf("unknown stream = %d %s", w.Code, w.Body.String())
	}
	var stops []func()
	defer func() {
		for _, s := range stops {
			s()
		}
	}()
	var first string
	for i := 0; i < 4; i++ {
		id, _, stop := openHomeDashStream(t, env, env.admin)
		stops = append(stops, stop)
		if i == 0 {
			first = id
		}
	}
	if w := env.do(t, env.admin, http.MethodPut, "stream/subscriptions", map[string]any{"stream_id": first, "entities": []string{"fake:a"}}); w.Code != http.StatusOK {
		t.Fatalf("PUT on a read-only desktop = %d %s", w.Code, w.Body.String())
	}
	if w := env.do(t, env.read, http.MethodPut, "stream/subscriptions", map[string]any{"stream_id": first, "entities": []string{}}); w.Code != http.StatusForbidden {
		t.Fatalf("desktop:read PUT = %d", w.Code)
	}
	if w := env.do(t, env.admin, http.MethodGet, "stream", nil); w.Code != http.StatusTooManyRequests || hdCode(t, w) != "too_many_streams" {
		t.Fatalf("fifth stream = %d %s", w.Code, w.Body.String())
	}
}
```

- [ ] Run `go test ./internal/server -count=1 -run '(?i)(TestHomeDashStream|TestHomeDashSubscriptions)'`. Expected: FAIL (404 routes / `homeDashHeartbeat` undefined).
- [ ] Create `internal/server/homedash_stream.go` (if Task 6 declared `homeDashStreamProtocol` in `homedash_routes.go`, keep that one and drop it here):

```go
package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"aurago/internal/homedash"
)

// Stream timing; variables so that tests can shorten them.
var (
	homeDashHeartbeat   = 15 * time.Second
	homeDashReauthEvery = 5 * time.Second
)

const (
	// homeDashWriteTimeout bounds each frame; httpstream.WithWriteTimeout renews its own budget on the
	// API listeners. The deadline is never cleared (a stuck client must time out).
	homeDashWriteTimeout = 30 * time.Second
	// homeDashStreamProtocol is sent in hello and bootstrap; v1 changes are additive only.
	homeDashStreamProtocol = 1
)

type homeDashHelloLimits struct {
	Entities int `json:"entities"`
}

type homeDashHello struct {
	StreamID    string              `json:"stream_id"`
	Protocol    int                 `json:"protocol"`
	HeartbeatMS int64               `json:"heartbeat_ms"`
	Limits      homeDashHelloLimits `json:"limits"`
	ServerTime  time.Time           `json:"server_time"`
	Principal   string              `json:"principal"`
}

// homeDashEventStream writes server-sent events: one compact, scrubbed JSON line per frame, a write
// deadline armed before each frame.
type homeDashEventStream struct {
	w       http.ResponseWriter
	flusher http.Flusher
	rc      *http.ResponseController
}

func (st *homeDashEventStream) arm() error {
	if err := st.rc.SetWriteDeadline(time.Now().Add(homeDashWriteTimeout)); err != nil && !errors.Is(err, http.ErrNotSupported) {
		return err
	}
	return nil
}

func (st *homeDashEventStream) send(event string, data any) error {
	raw, err := json.Marshal(data)
	if err != nil {
		return err
	}
	payload := scrubSSEJSON(string(raw))
	if err := st.arm(); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(st.w, "event: %s\ndata: %s\n\n", event, payload); err != nil {
		return err
	}
	st.flusher.Flush()
	return nil
}

// writeBatch sends one drained batch in protocol order: snapshot (with the pending source statuses),
// entity, removed, state, source (only without a snapshot), dashboard_changed, revoked. done is true
// after revoked.
func (st *homeDashEventStream) writeBatch(b homedash.Batch) (done bool, err error) {
	if len(b.Snapshot) > 0 {
		sources := b.Sources
		if sources == nil {
			sources = []homedash.SourceStatus{}
		}
		if err := st.send("snapshot", map[string]any{"entities": b.Snapshot, "sources": sources}); err != nil {
			return false, err
		}
		b.Sources = nil
	}
	if len(b.Entities) > 0 {
		if err := st.send("entity", map[string]any{"entities": b.Entities}); err != nil {
			return false, err
		}
	}
	if len(b.Removed) > 0 {
		if err := st.send("removed", map[string]any{"ids": b.Removed}); err != nil {
			return false, err
		}
	}
	if len(b.States) > 0 {
		if err := st.send("state", map[string]any{"changes": b.States}); err != nil {
			return false, err
		}
	}
	for _, src := range b.Sources {
		if err := st.send("source", src); err != nil {
			return false, err
		}
	}
	if b.Dashboard != nil {
		if err := st.send("dashboard_changed", b.Dashboard); err != nil {
			return false, err
		}
	}
	if b.Closed != "" {
		_ = st.send("revoked", map[string]string{"reason": b.Closed})
		return true, nil
	}
	return false, nil
}

// homeDashStreamRevoked re-validates an open stream: the caller's desktop:admin authorization (as the
// desktop websocket does) and HomeDash availability. A read-only desktop does not end a stream.
func homeDashStreamRevoked(s *Server, r *http.Request) string {
	if !desktopWSAuthorizationValid(s, r, desktopScopeAdmin) {
		return homedash.CloseAuthLost
	}
	if !homeDashEnabled(s.ConfigSnapshot()) {
		return homedash.CloseDisabled
	}
	return ""
}

// stream serves GET stream. One goroutine writes; it returns when the request ends, a write fails,
// re-validation fails (revoked) or the hub closes the subscriber (revoked with the hub's reason).
// The subscriber is always closed. Flushes are coalesced to at most one per svc.coalesce.
func (h *hdRequest) stream() {
	flusher, ok := h.w.(http.Flusher)
	if !ok {
		homeDashWriteError(h.w, http.StatusInternalServerError, homeDashCodeInternal, "streaming is not supported", nil)
		return
	}
	sub, err := h.svc.hub.Attach(homeDashPrincipal(h.r))
	if err != nil {
		h.fail(err)
		return
	}
	defer sub.Close()

	header := h.w.Header()
	header.Set("Content-Type", "text/event-stream")
	header.Set("Cache-Control", "no-store")
	header.Set("X-Accel-Buffering", "no")
	h.w.WriteHeader(http.StatusOK)
	st := &homeDashEventStream{w: h.w, flusher: flusher, rc: http.NewResponseController(h.w)}

	limit := h.cfg.HomeDash.Limits.MaxSubscriptions
	if limit <= 0 {
		limit = 300
	}
	if st.send("hello", homeDashHello{
		StreamID: sub.ID(), Protocol: homeDashStreamProtocol, HeartbeatMS: homeDashHeartbeat.Milliseconds(),
		Limits: homeDashHelloLimits{Entities: limit}, ServerTime: time.Now().UTC(), Principal: string(homedash.PrincipalSession),
	}) != nil {
		return
	}
	beat := time.NewTicker(homeDashHeartbeat)
	defer beat.Stop()
	reauth := time.NewTicker(homeDashReauthEvery)
	defer reauth.Stop()
	ctx := h.r.Context()
	var lastFlush time.Time
	for {
		select {
		case <-ctx.Done():
			return
		case <-beat.C:
			if st.send("hb", map[string]any{"server_time": time.Now().UTC()}) != nil {
				return
			}
		case <-reauth.C:
			if reason := homeDashStreamRevoked(h.s, h.r); reason != "" {
				_ = st.send("revoked", map[string]string{"reason": reason})
				return
			}
		case <-sub.Ready():
			if wait := h.svc.coalesce - time.Since(lastFlush); wait > 0 {
				timer := time.NewTimer(wait)
				select {
				case <-ctx.Done():
					timer.Stop()
					return
				case <-timer.C:
				}
			}
			lastFlush = time.Now()
			b := sub.Drain()
			if b.Empty() {
				continue
			}
			if done, err := st.writeBatch(b); done || err != nil {
				return
			}
		}
	}
}

type homeDashSubscriptionRequest struct {
	StreamID string   `json:"stream_id"`
	Entities []string `json:"entities"`
}

// subscriptions serves PUT stream/subscriptions: full replacement. Catalogs of the named sources are
// fetched first when stale, so the snapshot carries real entities. The stream must belong to the same
// principal (404 otherwise).
func (h *hdRequest) subscriptions() {
	var req homeDashSubscriptionRequest
	if !h.decode(&req, homeDashSubscriptionBody) {
		return
	}
	sub, ok := h.svc.hub.Subscriber(req.StreamID, homeDashPrincipal(h.r))
	if !ok {
		homeDashWriteError(h.w, http.StatusNotFound, homedash.CodeStreamNotFound, "there is no open stream with this id", nil)
		return
	}
	ids := make([]homedash.EntityID, 0, len(req.Entities))
	for _, id := range req.Entities {
		ids = append(ids, homedash.EntityID(id))
	}
	ctx, cancel := context.WithTimeout(h.r.Context(), homeDashCatalogWait)
	h.svc.hub.PrefetchCatalogs(ctx, ids)
	cancel()
	accepted, rejected, err := sub.Set(ids)
	if err != nil {
		h.fail(err)
		return
	}
	if accepted == nil {
		accepted = []homedash.EntityID{}
	}
	if rejected == nil {
		rejected = []homedash.Rejection{}
	}
	homeDashWriteOK(h.w, http.StatusOK, map[string]any{"accepted": accepted, "rejected": rejected})
}
```

- [ ] Add rows:

```go
	"stream":               {http.MethodGet: {desktopRead, (*hdRequest).stream}},
	"stream/subscriptions": {http.MethodPut: {desktopRead, (*hdRequest).subscriptions}},
```

- [ ] Run the test command with `-v`. Expected: 3 PASS. Run it 5 more times (`-count=5`) to catch timing flakes; all must pass.
- [ ] detect-changes; commit the three paths with message `feat(server): HomeDash SSE stream and subscriptions`.

**Done when:** the three stream tests pass in 6 consecutive runs.

---

### Task 11: Starter dashboard live from the real `system` adapter

**Files:** create `internal/server/homedash_system_e2e_test.go`.

**Read first:** Tasks 2, 4, 7a, 10; contract "`system` entities".

- [ ] Write `internal/server/homedash_system_e2e_test.go`:

```go
package server

import (
	"encoding/json"
	"net/http"
	"net/url"
	"sync"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/homedash"
	"aurago/internal/homedash/sources/system"
)

// The M2 definition of done end to end: the starter dashboard binds the six system entities, a
// stream shows them with real meta and values, a new metrics sample arrives as a state frame and the
// history ring fills.
func TestHomeDashSystemStarterEndToEnd(t *testing.T) {
	var mu sync.Mutex
	t0 := time.Now().UTC().Truncate(time.Second)
	sample := system.Snapshot{SampledAt: t0, CPUPercent: 21, MemoryPercent: 40, DiskPercent: 50,
		CPUOK: true, MemoryOK: true, DiskOK: true, NetworkOK: true, NetBytesRecv: 1000, NetBytesSent: 1000, BootTime: t0.Add(-time.Hour)}
	read := func() (system.Snapshot, bool) { mu.Lock(); defer mu.Unlock(); return sample, true }
	spec := homeDashSourceSpec{ID: system.SourceID, ConfigKey: "system", Settings: homeDashSystemSettings, Factory: system.Factory(read)}
	env := newHomeDashTestEnv(t, func(c *config.Config) { c.HomeDash.Sources.System.PollSeconds = 2 }, spec)

	w := env.do(t, env.admin, http.MethodPost, "dashboards", map[string]any{"template": "system"})
	var created hdSaved
	hdDecode(t, w, &created)
	if w.Code != http.StatusCreated || len(created.Warnings) != 0 {
		t.Fatalf("template = %d %s (system entities must be known: no warnings)", w.Code, w.Body.String())
	}
	ids := homeDashBoundIDs(created.Dashboard)
	if len(ids) != 6 {
		t.Fatalf("bound ids = %v", ids)
	}

	id, events, stop := openHomeDashStream(t, env, env.admin)
	defer stop()
	names := make([]string, len(ids))
	for i, e := range ids {
		names[i] = string(e)
	}
	if w := env.do(t, env.admin, http.MethodPut, "stream/subscriptions", map[string]any{"stream_id": id, "entities": names}); w.Code != http.StatusOK {
		t.Fatalf("PUT = %d %s", w.Code, w.Body.String())
	}
	var snap struct {
		Entities []homedash.Entity `json:"entities"`
	}
	if err := json.Unmarshal([]byte(waitHomeDashEvent(t, events, "snapshot").data), &snap); err != nil || len(snap.Entities) != 6 {
		t.Fatalf("snapshot = %+v %v", snap, err)
	}
	var cpu homedash.Entity
	for _, e := range snap.Entities {
		if e.ID == "system:cpu" {
			cpu = e
		}
	}
	if cpu.Name != "CPU" || cpu.Unit != "%" || !cpu.State.Available || cpu.State.Value != 21.0 {
		t.Fatalf("system:cpu = %+v", cpu)
	}

	mu.Lock()
	sample.SampledAt, sample.CPUPercent = t0.Add(10*time.Second), 55
	mu.Unlock()
	for got, frames := 0.0, 0; got != 55; frames++ {
		if frames > 6 {
			t.Fatal("no state frame with the new CPU value")
		}
		var st struct {
			Changes map[string]homedash.State `json:"changes"`
		}
		if err := json.Unmarshal([]byte(waitHomeDashEvent(t, events, "state").data), &st); err != nil {
			t.Fatal(err)
		}
		if v, ok := st.Changes["system:cpu"].Value.(float64); ok {
			got = v
		}
	}

	from, to := url.QueryEscape(t0.Add(-time.Hour).Format(time.RFC3339)), url.QueryEscape(t0.Add(time.Hour).Format(time.RFC3339))
	var hist struct {
		Origin string            `json:"origin"`
		Points []homedash.Sample `json:"points"`
	}
	hdDecode(t, env.do(t, env.admin, http.MethodGet, "history?id=system:cpu&from="+from+"&to="+to, nil), &hist)
	if hist.Origin != "memory" || len(hist.Points) == 0 {
		t.Fatalf("history = %+v", hist)
	}
}
```

- [ ] Run `go test ./internal/server -count=1 -run TestHomeDashSystemStarterEndToEnd -v`. Expected: PASS (no production change in this task; if it fails, fix the defect in the task that owns the code and note it in this commit body).
- [ ] Commit the file with message `test(server): HomeDash starter dashboard live from the system source`.

**Done when:** the test passes three times in a row (`-count=3`).

---

### Task 12: Documentation (docs-agent)

**Files:** modify `internal/homedash/AGENTS.md`, `internal/server/AGENTS.md`.

**Read first:** root `AGENTS.md` DOX rules (`:7`), both target files, this plan's contract and Tasks 1–10.

- [ ] `internal/homedash/AGENTS.md`:
  - Ownership: replace "Server wiring, HTTP routes, the config UI and the frontend are later milestones" with: server wiring and the desktop API live in `internal/server/homedash_*.go` (see `internal/server/AGENTS.md`, "HomeDash API"); the config UI and kiosk are later milestones. Add `sources/system`: host metrics from the server's collector sample, read-only, imports only `internal/homedash`.
  - Local Contracts: add `PrefetchCatalogs` (fetches stale catalogs of the named sources without interest; call it before `Subscriber.Set` so snapshots carry real entities) and `LookupEntities` (validation lookup without warm-up). Add: the dashboard cap may change at runtime (`SetMaxDashboards` is atomic).
  - Verification: add `go test ./internal/homedash/sources/system/ -count=1`.
- [ ] `internal/server/AGENTS.md`: new section `### HomeDash API` after `### Docker Manager API`, at most ~25 lines, covering: files and responsibilities; `initHomeDash`/`closeHomeDash` placement (after `initBluetoothLive` in `Start`, deferred, so it closes after the HTTP drain and before Docker hosts, go2rtc and Bluetooth); `replaceConfigSnapshot` calls `homeDashService.reconfigure` (non-blocking under `CfgMu`); store path from `config.SQLiteArchivePaths(cfg)["homedash"]`, fixed at start; capability `homedash` (`homedash.enabled && virtual_desktop.enabled`); auth (`desktop:admin` session or token) and route-table classification (`desktopRead` vs `checkDesktopOperation(desktopWrite)`; validate and subscriptions are reads); availability 503 codes; envelope and shared-layer error bodies; body limits; principal key (token digest, session nonce digest, `local-desktop`); stream rules (hello, `hb` event, 30 s write deadline per frame, scrubbed frames, 5 s re-validation, `revoked` reasons, coalescing, prefetch before `Set`); save pipeline (Migrate → Normalize → Autopack → Validate on every save path; template `system`; `DashboardChanged` after every save/delete/approve); `system` source fed by the extended metrics sample and `storeHostBootTime`; source table rule (one row per adapter in `homedash_sources.go`); verification command `go test ./internal/server -run '(?i)(HomeDash)' -count=1` and `go test ./internal/audit -run TestHomeDashDesktopRouteContract`; the authoritative contract is the M2 backend plan in `docs/superpowers/plans/`.
- [ ] Run `go test ./internal/audit/... -count=1` (doc link tests). Expected: PASS.
- [ ] detect-changes; commit both files (`git add` suffices: both are tracked) with message `docs(homedash): server wiring, desktop API and system source`.

**Done when:** both guides describe the shipped behaviour and the audit package passes.

---

### Task 13: Final verification, race run, hand-over

**Files:** create `docs/superpowers/specs/2026-11-23-homedash/m2-handover-backend.md` (docs-agent).

- [ ] test-agent, in the worktree:
  - `go build ./...`
  - `go vet ./internal/homedash/... ./internal/server/ ./internal/audit/`
  - `go test ./internal/homedash/... ./internal/config/ ./internal/httpstream/ ./internal/audit/... -count=1`
  - `go test ./internal/server -count=1 -run '(?i)(HomeDash|Metrics|ChibiSensei|DockerManagerStream|DesktopReadonly|BluetoothLive)' -v` (count PASS lines)
  - `go test ./internal/server -count=1 -timeout 40m -skip 'Browser'` (full package once; compare failures against the Task 0 baseline; any new failure blocks)
- [ ] Race run once (no local gcc). On aurago-test (test-server rules: scratch directory only, never touch containers, apps or data):

```bash
git archive --format=tar HEAD | ssh aurago-test 'rm -rf /tmp/hd-m2-race && mkdir -p /tmp/hd-m2-race && tar -x -C /tmp/hd-m2-race'
ssh aurago-test 'cd /tmp/hd-m2-race && go test -race -count=1 ./internal/homedash/... && go test -race -count=3 -run "(?i)(HomeDash)" ./internal/server/'
ssh aurago-test 'rm -rf /tmp/hd-m2-race'
```

  If aurago-test has no Go toolchain or cannot fetch modules, use the WSL approach of memory `reference_linux_go_test_container.md` and record which one ran.
- [ ] detect-changes over the whole branch (`scope: compare`, base `main`); judge by the listed symbols.
- [ ] docs-agent writes `m2-handover-backend.md` (≤ 80 lines): commits table, verification results with PASS counts, race result, every deviation from design-backend section 6 (copy the contract's "Differences" list), contradictions resolved (from this plan's self-review), open items for M3 (HA adapter registers a row in `homedash_sources.go`; `source_unavailable` with `source`; kiosk routes; agent facade), and that nothing is pushed or deployed. `git add -f` it, confirm with `git ls-files`, commit with message `docs(homedash): M2 backend hand-over`.

**Done when:** every command above is green (or matches the baseline), the race run is clean, and the hand-over is committed.

---

## Batches and reviews

| Batch | Tasks | Review |
|---|---|---|
| 1 Core | 1, 2, 3 | reader-agent once (Critical/Important only): lock order in `hub_lookup.go`, adapter `UpdatedAt` monotonicity, atomic cap. |
| 2 Wiring and dashboards (B5) | 4, 5, 6, 7a, 7b | reader-agent once: hot-reload path never blocks under `CfgMu`, shutdown order, save pipeline on every save path, route classification table. |
| 3 Live data (B6) | 8, 9, 10, 11 | reader-agent once; **security-agent once** over `homedash_routes.go` (auth, classification, principal, error mapping), `homedash_api_commands.go`, `homedash_stream.go` (re-validation, write deadline, scrubbing, stream ownership, limits) and `homedash_service.go` (`homeDashAudit`). |
| 4 Docs and closure | 12, 13 | none beyond the race run. |

Tasks inside a batch run sequentially (later tasks extend the route table and the fixture). Fix findings in one follow-up commit per batch; no re-review rounds for minor items (lean workflow). The frontend track may start immediately against the frozen contract with its fixture API (D9).

---

## Self-review

### Coverage of D1–D9

| Decision | Where |
|---|---|
| D1 B5: init/close | Task 5 (`initHomeDash`, `closeHomeDash`, `Start` placement) |
| D1 B5: RuntimeConfig, hot reload, egg mode, `homedash.enabled=false` | Task 5 (`homeDashRuntimeConfig`, `replaceConfigSnapshot` hook, reconfigure test); Task 6 (503 test); Task 10 (`revoked disabled`) |
| D1 B5: bootstrap, widget catalog | Task 6 |
| D1 B5: CRUD with revisions, restore, duplicate, validate, approve | Tasks 7a, 7b |
| D1 B5: desktop operation classification | Task 6 route table; tests in 7a, 7b, 9, 10 |
| D1 B5: capability `homedash` | Task 5 (`bluetooth_live.go`, test) |
| D1 B5: audit route contracts | Task 6 (`audit_manifests.go`, `homedash_manifest_test.go`) |
| D1 B5: agent facade stub "only if needed" | Not needed in M2 (no caller); M7 adds it. Recorded in the hand-over. |
| D1 B6: entities, entity, history, snapshot (`unsupported` for system) | Task 8 (snapshot answers `400 unsupported_command`) |
| D1 B6: commands route | Task 9 |
| D1 B6: SSE + PUT subscriptions | Task 10 |
| D1 B6: `system` adapter | Tasks 2, 3, 4, 11 |
| D2 contract first | "API contract (frozen for M2)" incl. differences list |
| D3 starter template | Task 7a (`homedash_template.go`, test), Task 11 (no warnings with the real source) |
| D4 save pipeline, explicit RadiusScale/RowUnit, new HTTP codes | Task 7a `prepare` used by saveNew/saveExisting (create, update, duplicate, restore, template) and validate; template sets both fields; codes in Task 6 |
| D5 alert tone | Frontend only (engine helper); the contract states the server sends no tone and documents the template's `warn`/`alert` options. The direction rule is the frontend plan's to decide and document. |
| D6 transport | Task 10 (hello, `hb` every 15 s, snapshot per accepted PUT for added ids, coalesced `state`, Docker Manager write-deadline pattern) |
| D7 system source | Tasks 2–4 (no second sampler; 5 s poll; six entities; numeric + history; read-only; real `SystemMetrics` fields verified at `internal/tools/system_metrics.go:25-56`) |
| D8 testing | httptest handler tests with FakeSource (6–10), adapter vs fake snapshot (2), one `-race` run (13). The wiring test and browser test are frontend (F2). |
| D9 order | Backend plan first; Batch 3 can run while the frontend uses its fixture API. |

Design sections cited: main 3/4/8/12 (12.3 carry-overs in Tasks 6, 7a); backend 5 (hub lifecycle, interest, shutdown → Task 5), 6 (contract), 7 (document/store rules → Task 7), 8.5 (Tasks 2–4), 9 (auth, CSRF via middleware, scrubbing, stream auth loss → Tasks 6, 10), 11 (hot reload, capability → Task 5), 12 (server tests `TestHomeDashRoutesRequireDesktopAdmin`, `…DashboardCRUDAndRevisionConflict`, `…SubscriptionsAndStreamLimits` (read-only desktop), `…CommandsRefusedOnReadOnly`, `…StreamHelloSnapshotStateHeartbeat`, `…StreamEndsWithRevoked`, `…CommandsAcceptAuditAndRefuse`; `TestSystemAdapterMapsSnapshot`), 13 (B5, B6). Kiosk, MQTT and HA tests of section 12 belong to M3/M6.

### Contradictions found and resolutions

1. **System metrics shape.** Design 8.5 assumes a `func() (tools.SystemMetrics, time.Time)` reader; the stored sample has only CPU/memory/disk percent (`internal/server/dashboard_metrics_snapshot.go:13-21`), no network counters, no uptime. Resolution: Task 3 extends the sample (network counters) and stores the host boot time from the collector's existing `tools.GetHostInfo()` call (`server_routes.go:147`); the adapter defines its own `Snapshot` so it does not import `tools`.
2. **Read classification.** Design 6 says to add D-admin-read paths to `desktopRequestOperation`; HomeDash never calls `requireDesktopPermission` (the only general consumer, `desktop_auth.go:23-28`). Resolution: per-route classification in `homeDashRoutes` with `checkDesktopOperation`, the CYD Creator precedent (`internal/server/AGENTS.md:1059-1066`).
3. **Heartbeat.** Design 6: `: hb` comment. EventSource does not surface comments, so the client cannot detect a silent dead stream. Resolution: `hb` event (additive, recorded in the differences list).
4. **Placeholders in snapshots.** `Subscriber.Set` snapshots a generic unavailable placeholder when the catalog was never fetched (`internal/homedash/subscriber.go:189-193`); nothing fetches it on interest. Resolution: `Hub.PrefetchCatalogs` before `Set` (Task 1, Task 10).
5. **Dashboard cap hot reload.** `Store.SetMaxDashboards` writes a plain int read concurrently by `CreateDashboard` (`store.go:76-80`, `store_dashboards.go:78`). Resolution: atomic (Task 1).
6. **Coalesce interval.** `Options.CoalesceInterval` is "read by the stream writer" (`hub.go:20`) but the hub has no getter. Resolution: the service keeps its own copy.
7. **`dashboard_changed` collapse.** `Batch.Dashboard` holds one pointer (`subscriber.go:56,306-313`). Resolution: contract requires list reconciliation; no core change in M2.
8. **Snapshot of `system`.** D1 says "unsupported"; the hub answers `unsupported_command` (`history.go:233-240`). Resolution: keep the core code, 400.
9. **Route cap for JSON.** `decodeJSONRequest` has no route-limit parameter (`json_request.go:32-51`; design 14 left it unverified). Resolution: wrap the body in a smaller `MaxBytesReader` first ("a smaller route limit still wins", `internal/server/AGENTS.md:150`).
10. **DELETE route contract** (design 11 item 1). The manifest test refuses only public mutating routes and covers registered routes by prefix (`audit_manifests_test.go:99-105,2017-2030`). Resolution: dedicated `/api/desktop/homedash/` contract with DELETE is accepted (Task 6 + test).
11. **How `Requires` is consumed** (design 11 item 6): `desktop.FilterAvailableApps` with `desktopCapabilities` (`internal/desktop/capabilities.go:23-41`, `desktop_handlers.go:102,145`). The frontend's `BuiltinApps` row adds `Requires: ["homedash"]`.
12. **Capability vs API gate.** Capability is `homedash.enabled && virtual_desktop.enabled` exactly as instructed; the API additionally refuses in egg mode (503), matching the hub, which disables every source in egg mode (`runtime_config.go:38`).
13. **Error envelope of shared layers.** `writeDesktopPolicyError` (`desktop_operation.go:192-196`) and the auth middleware (`auth.go:1018-1051`) write bodies without `status`. Resolution: contract tells clients to key on `code`, falling back to `error`.
14. **`source_unavailable` with `source`** (design 12.3): no M2 code path produces it (the hub returns `entity_not_found` for inactive sources). Resolution: listed as reserved; added when an adapter needs it (M3).
15. **Design example ids** (`d_k3x9q2m7a1bc`) contain characters outside base32 `[a-z2-7]` and would fail `ValidDashboardID` (`ids.go:21`). The contract uses valid ids.
16. **Agent facade at init** (design 5): deferred to M7 per D1.

### Depth notes

All code that is protocol-, concurrency- or security-relevant is written out: route dispatch and auth classification, error mapping and principal (Task 6), save pipeline and starter template (Task 7a), stream, frame writer, re-validation and subscription handler (Task 10), command handler (Task 9), RuntimeConfig builder, service lifecycle and audit binding (Task 5), system adapter (Task 2), core prefetch/lookup (Task 1). Every test is complete code. No output limit forced omissions.
