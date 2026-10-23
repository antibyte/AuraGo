# Detective — remediation and optimization plan

Status: **draft**, planned 2026-10-23, not executed.
Source baseline: `reports/detective-audit-2026-10-23.md` (findings 4.1–4.22), verified 2026-10-23
against `main` at `e72af4dc2`.
Scope of this plan: `internal/detective`, `internal/server/detective_*`,
`internal/config/detective.go`, `ui/cfg/detective.js`, `ui/js/desktop/apps/detective*.js`,
`ui/css/desktop-app-detective.css`, `documentation/detective.md`.

> **For agentic workers:** execute the waves in order, one task per commit unless a task says
> otherwise. Every task follows the same shape: write the failing test first, watch it fail,
> implement, watch it pass, run the listed verification, do the DOX pass. Do not skip
> "Confirm callers" — the Detective contracts are consumed by the Desktop app, the agent loop
> (`ExecutionHooks`), the SQLite store and the export artifacts at the same time.

## Goal

Close all 22 audit findings without weakening the behavior the audit rated as strong: cumulative
budgets across continuation, server-anchored evidence quotes, immutable revision-bound exports,
automatic updates that never publish a partial replacement, and the desktop admission/revocation
boundary.

After this plan:

1. no read or write path scales with the evidence body (measured targets in §"Measurement targets");
2. the case API is bounded and evidence text loads on demand, so polling a large case costs
   milliseconds instead of a quarter second and megabytes;
3. a stale selected revision can never break the app, and Start/Stop/Continue behave under every
   permission state;
4. research does not pay for every page twice, and the prompt keeps exact provenance;
5. automatic updates announce themselves, errors are localized, and a provisional revision never
   silently becomes the default export;
6. administrators can configure the read-operation allowlist in the UI, and the Detective database
   path is independent of Game Maker;
7. the schema migration from v2/v3 and every new behavior has a focused test.

## Scope

In scope: the store (schema v4, summary projection, working-set/archive split), the live/list
queries, the HTTP projections in `detective_handlers.go`, the capture path in `detective_policy.go`,
the runner hooks in `detective_service.go`, report/revision semantics, the Desktop app (state,
transport, exports, a11y), the Config section, the Desktop notification path, the embedded skill
text where it describes the tool-result envelope, and the tests listed per task.

Out of scope: rewriting the research policy or the `detective_report` operation model, provider
routing/failover, replacing the SSE infrastructure, additional export formats, multi-user or
multi-tenant cases, cross-case search, moving the Detective store into the shared application
database.

## Current integration points

| Concern | Source |
| --- | --- |
| Route registration | `internal/server/detective_handlers.go:20` `registerDetectiveRoutes`, mounted at `internal/server/server_routes.go:650` |
| HTTP surface | `internal/server/detective_handlers.go:52` `handleDetective` (capabilities, cases, archive, report, source, events, live, run, export, autor) |
| Error envelope | `internal/server/detective_handlers.go:30` `detectiveError` — plain `err.Error()` in `{"error": …}` |
| Tool policy | `internal/server/detective_policy.go:16` `detectivePublicTools`, `:21` `detectiveToolKnown`, `:28` `detectiveAuthorize`, `:84` `detectiveCapture` |
| Runner and hooks | `internal/server/detective_service.go:120` `detectiveRunner.Run`; hooks `OnAcquire`/`BeforeIteration`/`BeforeRequest`/`BeforeTool`/`AfterTool`/`AfterResponse` at `:207-281` |
| Research resources | `internal/server/detective_resources.go:31` browser/workspace ownership |
| Store | `internal/detective/store.go:215` `saveLocked`, `:232` `Get`, `:242` `ListSummaries`, `:273` `Live`, `:326` `list`; `internal/detective/archive.go:66` `saveCaseTx`, `:106` `archiveLocked`, `:223` `resolvedReport` |
| Jobs and budgets | `internal/detective/jobs.go:167` `worker`, `:314` `update`, `:439` `complete` |
| Evidence capture | `internal/detective/evidence.go:39` `RecordSource`, `:49` `RecordReceipt`, `:108` `AddFinding` |
| Revisions | `internal/detective/evidence.go:233` `Submit`, `internal/detective/archive.go:300` `reportEvidenceLocked`, `internal/detective/types.go:104` `Report` |
| Automatic updates | `internal/detective/auto_update.go:36` `SetAutoUpdate`, `:82` `nextUpdate`, `:91` `queueDueLocked` |
| Exports | `internal/detective/export.go:31` `ExportContext`, `internal/detective/artifacts.go:15` `ExportRevision` |
| Migration | `internal/detective/store.go:121` `migrateDatabase`, `internal/detective/migration.go:11` `backupBeforeArchive`, `:41` `migrateArchive` |
| Config type | `internal/config/detective.go:11` `DetectiveConfig`, `internal/config/config_types.go:1201` SQLite paths, defaults `internal/config/config.go:1651` |
| Config UI | `ui/cfg/detective.js:2` `renderDetectiveSection`, registered in `ui/js/config/main.js:99`, `:3423`, icon slot `:619`, sprite `:717`; strings `ui/lang/config/detective/*.json` (16) and `ui/lang/config/*.json` |
| Config template | `config_template.yaml:1203-1211` |
| Desktop app | `ui/js/desktop/apps/detective.js`, `-views.js`, `ui/css/desktop-app-detective.css`, `ui/js/desktop/core/module-loader.js:478`, `desktop-foundation.js:626`, `menus-and-routing.js:1883` |
| App manifest | `internal/desktop/types.go:843` (icon `search`, category `ai`, permission `notifications`) |
| Notification pattern | `internal/server/person_detect_alerts.go:296` `showPersonAlertToast` → `broadcastDesktopEvent(s, hub, desktop.Event{Type: "notification", …})` |
| Docs | `documentation/detective.md` (usage, automatic update, evidence, API, migration) |
| Tests | `internal/detective/*_test.go`, `internal/server/detective_*_test.go`, `ui/desktop_detective_test.go`, `scripts/test-ui-regressions.mjs` |

## Design decisions

### DD1 — The working set carries no evidence bodies; the archive is the only excerpt store

`detective_sources` already holds every excerpt immutably; `AddFinding` validates quotes against
that row (`evidence.go:144`), not against the working set. The body therefore keeps source
*metadata* only, and `GET /cases/{id}` returns metadata plus counts. Excerpts load per source via
the existing `GET /cases/{id}/source?id=` (`detective_handlers.go:282`).

Rejected: keeping excerpts but bounded per case (still megabytes in every write), and moving the
working set into its own table (a second source of truth).

Compatibility: the archive rows are unchanged, so the migration is lossless and the UI keeps the
sources tab. `Source.Excerpt` stays in the struct because `RecordSource` returns it to the model
and `/source` serves it.

### DD2 — A `summary` column feeds the live view and the case list

`Live()` currently runs ~27 `json_extract` calls against the full body, and `ListSummaries()`
runs `json_remove` over all 100 bodies (`store.go:298-301`, `:331`). Both will read one small
`summary` JSON column maintained by `saveCaseTx` from a single Go projection (`caseSummary`).

The projection is the single implementation: the migration backfill calls the same function, and a
test asserts `caseSummary(loaded) == stored summary` for migrated rows. `body` stays the source of
truth for the run; `summary` is a cache with a fallback (empty summary → old query) so a
half-migrated database still serves.

### DD3 — Do not add a second SQLite connection; make the hot reads cheap instead

The store runs `SetMaxOpenConns(1)` plus one global mutex (`store.go:57`), which serializes polls,
exports and the runner. Adding a reader connection would introduce `SQLITE_BUSY`/checkpoint
interplay for little gain, because DD1+DD2 already remove the expensive reads.

Decision: keep one connection; after DD1/DD2, re-run `BenchmarkCaseHotPaths`. Only if a read still
exceeds 5 ms at the target body size do we take `Service.mu` to `sync.RWMutex` (read lock for
`Get`/`Live`/`List`/`Archive`/`Source`/`ReportRevision`, write lock elsewhere). `PRAGMA
synchronous=NORMAL` is only acceptable after DD1/DD2 and only together with a comment about the
WAL durability window; migrations and backups keep `FULL`.

### DD4 — Provisional revisions never become the default edition

A run that fails after a completed report (typical: a failed Deepen) currently publishes a partial
revision and makes it `LatestRevision`, so it becomes the default export and blocks auto-update
scheduling (`jobs.go:475-483`, `auto_update.go:70`).

New shape: `Report.Provisional bool` marks the automatic partial of a non-initial run;
`Case.LatestCompleteRevision int` is the newest non-provisional revision. `LatestRevision` keeps its
current meaning (monotonic, newest published) so existing clients and the export chain are
unaffected. Append updates and `SetAutoUpdate`/`nextUpdate` use `LatestCompleteRevision`.
The UI marks provisional revisions and defaults the selector and Autor handoff to the newest
complete one.

### DD5 — The admin read-operation catalog is served by a new read-only endpoint

`extra_read_operations` is a security-relevant allowlist that today only YAML can express. The
Config section needs the real tool/operation names, so the server exposes
`GET /api/desktop/detective/read-operations` (Desktop admin scope, GET only), derived from
`agent.ConfiguredToolSchemas(cfg, logger)` plus `detectiveToolKnown`: tool name and, where the
schema exposes one, the `operation` enum. MCP (`server/tool`) and Composio (slug) entries cannot be
enumerated and stay free-text rows. The endpoint returns names only — no secrets, no manifests.

### DD6 — The database path gets an explicit key, the legacy derivation stays the default

`detective.db` is derived from `sqlite.game_maker_path` today (`detective_service.go:42-45`), so
moving Game Maker "loses" cases. Add `sqlite.detective_path` (empty = current derivation), resolve
it like every other SQLite path (`config.go:1651` pattern), and log the resolved path once at
startup. No automatic data move: a migration of user files is out of scope and would be surprising.

## Behaviour changes to announce

1. `GET /cases/{id}` no longer returns `sources[].excerpt` and the case-level `findings`; both stay
   available through `GET /cases/{id}/source?id=` and `GET /cases/{id}/findings`. Documented in
   `documentation/detective.md` §"Reports and exports"/API block.
2. `Stop` is available for any active case whenever Detective is enabled (no longer gated on
   `ready`); a run whose permission was revoked stays `queued` with reason `permission_revoked`
   instead of failing.
3. Partial revisions of an existing case are `provisional`; the default export and append base use
   the newest complete revision.
4. PDF export names the supported scripts before the click and offers DOCX as the alternative.
5. `extra_read_operations` becomes editable in Config; unknown entries are dropped on save and
   flagged in the UI.
6. Detective errors reach the UI as stable codes with localized text; the raw English message
   remains the fallback and stays in the log.
7. `sqlite.detective_path` exists (default unchanged), and startup logs the resolved path.

## Measurement targets

Measured on this machine with the audit probe (200 sources × 20 KB excerpt, 4 MB body):

| Path | Today | Target |
| --- | ---: | ---: |
| `Live()` | 242 ms | < 2 ms |
| `ListSummaries()` (1 case, 100 cases real) | 28 ms | < 1 ms |
| one state write (`BeforeTool`) | 114 ms | < 15 ms |
| `GET /cases/{id}` | ~4 MB | < 150 KB |
| working-set body with 400 sources × 32 KB evidence | 4 MB+ | < 256 KB |
| UI requests per 3 s poll, window visible | 3 (+ full case fetch on change) | 2 (0 while hidden) |

These are bench targets, not CI wall-clock assertions. The committed tests assert the structural
property (body size, projection content, no excerpt in the body) and provide
`BenchmarkCaseHotPaths` for humans.

## Task overview

| ID | Task | Findings | Size | Depends on |
| --- | --- | --- | --- | --- |
| D1 | Revision state can never desync | 4.2 | S | — |
| D2 | Idempotency keys without `crypto.randomUUID` | 4.10 | S | — |
| D3 | Stop/Finish availability follows capability, not readiness | 4.6, part of 4.19 | S | — |
| D4 | Cheap startup query, worker backoff, surfaced `complete` errors | 4.7, 4.11 | S | — |
| D5 | Permission revocation keeps a queued case queued | 4.5 | M | D4 |
| D6 | Schema v4: `summary` column and projection | 4.1, 4.12 | L | — |
| D7 | Working set without excerpts, bounded case projection | 4.1, 4.3 | L | D6 |
| D8 | Hot-path guards and locking decision | 4.1 | M | D6, D7 |
| D9 | UI transport, visibility and idle polling | 4.18, 4.1 | M | D7 |
| D10 | Compact capture receipts | 4.4 | S | — |
| D11 | Localized error codes and PDF capability | 4.8, 4.13 | M | — |
| D12 | Provisional revisions and append base | 4.14 | M | D6 |
| D13 | Answers and token/cost visibility | 4.15, 4.17 | S | — |
| D14 | `cfb93191f` (server) + `c34548d33` (click-through) | `TestDetectiveNotificationOnPublish`, `TestDetectiveNotificationSuppressedWhenDisabled`, `TestDetectiveNotificationRateLimited`, `TestDesktopDetectiveOpensTheNotifiedCase` | The click-through landed in the next round: the bundle rebuild was safe once the bundle check was green and the foreign workstream had not touched bundle inputs |
| D15 | `0ba71a7d8` (server/config) + `7cd417eb0` (Config UI) | `TestNormalizeDetectiveReadOperations`, `TestDetectiveReadOperationsEndpoint`, `TestDetectiveConfigAllowlistEditor` | The Config control is a `tool: operation, operation` textarea instead of a multi-select per tool: it needs no new widget machinery, the server normalizes it again, and unknown lines stay visible and removable. Three labels in all 16 config locales |
| Review | `PENDING` | `TestDetectivePathKeepsLegacyAfterUpgrade`, `TestNormalizeDetectiveYAMLSection`, `TestDesktopDetectiveAccessibilityMarkers` (erweitert) | Fünf Befunde behoben: (1) der Altbestand neben der Game-Maker-DB wird beim Upgrade nur noch dann genutzt, wenn der konfigurierte Pfad der Default ist, noch nicht existiert und die Altdatei vorhanden ist (`DetectivePathDefaulted`); (2) die Allowlist wird auch beim Config-Save normalisiert; (3) Tab-/Panel-IDs sind jetzt pro Fenster eindeutig; (4) fehlt `ctx.confirmDialog`, meldet Löschen einen Fehler statt still nichts zu tun; (5) der Notify-Limiter räumt abgelaufene Fälle auf. Akzeptierte Risiken: einmalige `pre-v4`-Backups bleiben liegen, `detective_keys` behält 50 Zeilen je Fall |
| D16 | `590bdad41` | `TestDetectivePathDefaultAndResolve`, `TestDetectivePathDefaultsAndOverride`, `TestDetectivePathKeepsLegacyLocation`, `TestDetectiveInitializationErrorCodes` | D15 follows separately; the capability key is `initialization_error` with four codes (`schema_too_new`, `permission_denied`, `path_missing`, `store_failed`), and the app renders one generic `desktop.detective_init_failed` label plus the code instead of four texts |
| D17 | `7b68682e1` | `TestDetectiveKeyRetention`, `TestDetectiveSourceURLErrorMessage`, `TestDesktopDetectiveAccessibilityMarkers` | Icon decision: the shared `search` key stays, documented in the app AGENTS (a distinct glyph needs new artwork in both icon families for a cosmetic change). `TestDesktopDetectiveAccessibilityMarkers` cannot run yet: the parallel workstreams uncommitted `internal/tools` changes break `go test ./ui` (unused import, `loadProxyRoutes` arity) |
| D18 | `b0a73da35` | `TestMigrateV2ToV4KeepsCasesAndBacksUp`, `TestSetAutoUpdateMatrix`, `TestLiveUnderAutomaticUpdate` | The browser-fixture part stays as it is: Chrome cannot launch here (`OpenProcess: Zugriff verweigert`) and the suite is CI-only; the D1/D7/D12 behaviours are covered by node and source-contract tests. D15 remains open |
| Verification | — | `go test ./internal/detective` (3.9 s), `./internal/config` (5.2 s), `./ui` (75.7 s, full package), `./internal/server -run Detective` (5.7 s), `node scripts/test-ui-regressions.mjs` (8/8 Detective green; the two Game-Maker failures pre-date this work), `BenchmarkCaseHotPaths` (Live 38–118 µs, list 45–129 µs, write 0.8–1.9 ms) | Run after D18 while the parallel workstreams `internal/tools` tree compiled. D15 remained open at this point |
| Verification 2 | — | `go test ./internal/detective` (14,4 s), `./internal/config` (13,2 s), `./ui` (78,3 s), `./internal/server -run Detective` (6,3 s) alle **ok**; Node-Suite weiterhin nur die 2 vorbestehenden Game-Maker-Fehler | Wiederholt nach D15 Teil 1, als der fremde `internal/tools`-Baum wieder kompilierte. Offen bleibt allein die D15-Config-Oberfläche mit ihren 16 Locales |
| D14 | Desktop notifications for update, question and failure | 4.16 | M | — |
| D15 | `extra_read_operations` in Config | 4.9 | L | — |
| D16 | `sqlite.detective_path` and init visibility | 4.19 | M | — |
| D17 | A11y, confirm dialog, retention, URL wording, icon | 4.20, 4.21 | M | — |
| D18 | Test hardening: migration, automation matrix, browser | 4.22 | M | D6–D12 |

Size: S = one commit, M = two to four commits, L = one PR-sized change with its own review.

---

## Wave 1 — correctness and UI safety

### D1 — Revision state can never desync

**Problem.** `state.revision` is reset on case switch (`detective.js:476`) but not on "New" (`:483`)
or after creating a case (`:377-389`). With a stale revision number, `chosenReport()` (`:32-35`)
returns `undefined` and `exportHTML` (`:107-111`) throws; `paint()` and `draw()` then fail on every
poll and the report tab stops refreshing.

**Changes.**
1. `resetArchive()` owns the selection: set `state.revision = 0` and `selectedReport = null` there,
   and call it from both create paths (it already runs on case switch).
2. Move `chosenReport`, `knownReports`, `exportRevisionSet`/`reportRevisionSet` into
   `detective-views.js` as pure functions on `(state.current, revision, selectedReport, history)`
   so they are testable without a DOM.
3. Guard the export builder: `const chosen = chosenReport() || revisions.at(-1)`, and return `''`
   when no revision is known.
4. On create, also reset `state.tab = 'report'`.

**Tests (first).** `scripts/test-ui-regressions.mjs`: a case with revisions 1–3, select 3, then a
new case with revision 1 → `chosenReport` resolves to 1 and `reportHTML` renders; a case without
reports renders the empty state without throwing. `ui/desktop_detective_test.go` keeps the browser
flow (needs `AURAGO_RUN_BROWSER_SMOKE=1`).

**Verify.** `node scripts/test-ui-regressions.mjs`, `node --check ui/js/desktop/apps/detective.js`,
`go test ./ui -run TestDesktopDetective`.

**DOX.** `ui/js/desktop/apps/AGENTS.md` §Detective: state reset and the view-owned revision helpers.

### D2 — Idempotency keys without `crypto.randomUUID`

**Problem.** `key()` (`detective.js:6`) uses `crypto.randomUUID`, which is secure-context only, so
Start/Continue throws on plain-HTTP LAN installs. `infra.js:808` and `camera.js:707-709` already
show the fallback pattern; `notes.js:89` shares the gap (out of scope, mention in the commit).

**Changes.** Local `newKey()`: `window.crypto?.randomUUID?.()` else
`` `${Date.now().toString(36)}-${…getRandomValues hex…}` `` with a `Math.random` last resort. Keep
the value ≤ 128 characters (`jobs.go:31` limit).

**Tests.** Node: a fixture with `crypto.randomUUID = undefined` yields a non-empty, unique key, and
two calls differ; the string length stays under the server limit.

**Verify.** `node scripts/test-ui-regressions.mjs`, browser smoke.

**DOX.** Same section as D1 (one sentence on the key source).

### D3 — Stop/Finish availability follows capability, not readiness

**Problem.** `canStop` requires `caps.ready` (`detective.js:86`), while Stop is a pure state write
(`jobs.go:140-165`) admitted as `desktopStop` even in read-only mode. `ready` only becomes false when
the runner was never installed (a restart converts running/queued cases to `interrupted`, and `Start`
requires a runner), so the coupling is a latent hole rather than a live defect — but it is one
runner failure away from stranding an active case. Additionally `permission_blocked` is derived from
`auto_update.outcome` (`detective.js:19`), which cannot distinguish "read-only" from "disabled".

**Changes.**
1. `canStop = !!caps.enabled` (Stop is never gated on `ready`); Finish keeps the write gates.
2. `/capabilities` gains `block_reason` ("disabled" | "read_only" | "initializing" | "permission")
   and the UI renders it in the unavailable notice instead of the generic text.
3. Keep `ready` for Start/Continue/Deepen only.

**Tests.** Server: `TestDetectiveCapabilitiesBlockReason` (disabled vs read-only vs
initialization failure). UI: node test proving Stop renders enabled with `ready: false` and
`enabled: true`, disabled with `enabled: false`.

**Verify.** `go test ./internal/server -run TestDetectiveCapabilities`, node regressions.

**DOX.** `ui/js/desktop/apps/AGENTS.md` §Detective (Stop paragraph), `documentation/detective.md`
§"Using the app" (Stop bullet already says this; align it with `block_reason`).

### D4 — Cheap startup query, worker backoff, surfaced `complete` errors

**Problem.** `New()` calls `List()` (full bodies) to find running/queued cases (`store.go:93-115`)
although the table has a `status` column. The worker re-selects the same queued case in a tight loop
when `complete()` cannot write, and `complete()` discards its own error (`jobs.go:440`).

**Changes.**
1. `SELECT id FROM detective_cases WHERE status IN ('running','queued')` for the restart pass.
2. `worker()`: on a failed `complete()`/`saveLocked`, leave the inner loop and wait for the 30 s
   tick; add a per-case attempt counter (in memory) that logs a warning after three failures.
3. `complete()` returns its error; the worker logs it with the case ID (scrubbed).
4. Recovery test: simulated write failure does not spin.

**Tests.** `TestRestartQueryDoesNotLoadBodies` (store: insert 3 cases with large bodies, assert
`New` does not decode them — measure via a body that would fail unmarshalling, e.g. invalid JSON in
a non-running case), `TestWorkerBacksOffAfterCompleteFailure`.

**Verify.** `go test ./internal/detective -run 'Restart|Worker'`.

**DOX.** `internal/detective/AGENTS.md` (restart paragraph, worker contract).

### D5 — Permission revocation keeps a queued case queued

**Problem.** `worker()` consults `allowed()` only for scheduling (`jobs.go:178-193`); a queued case
starts anyway, the runner aborts (`detective_service.go:132-134`), and `complete()` files it as
`failed`/`provider_or_tool_failure` (`jobs.go:461-466`), which the UI shows as a provider error.

**Changes.**
1. In the worker loop, skip dequeuing while `!allowed()`; the case stays `queued` and the next tick
   re-evaluates. Do not touch its `next_run_at`.
2. Introduce `reasonPermissionRevoked = "permission_revoked"` for the runner's own abort (defence in
   depth when rights vanish after dequeue) and map it in `complete()`; automatic runs log it
   without counting it as a research failure.
3. `desktop.detective_permission_revoked` in all 16 `ui/lang/desktop/*.json`.

**Tests.** `TestQueuedCaseWaitsForPermission` (allowed=false keeps status queued and no run starts,
allowed=true starts it), `TestPermissionRevokedReasonIsLocalized` (translation completeness),
`TestCompleteMapsPermissionRevocation`.

**Verify.** `go test ./internal/detective -run 'Permission'`, `go test ./ui -run TestDetectiveTranslations`.

**DOX.** `internal/detective/AGENTS.md` (scheduler contract), `documentation/detective.md`
§"Automatic Update" (permission restrictions).

---

## Wave 2 — data model and transport

### D6 — Schema v4: `summary` column and projection

**Problem.** `Live()` parses the full body ~27 times per call; `ListSummaries()` parses all 100
bodies. Measured 242 ms / 28 ms at a 4 MB case.

**Changes.**
1. New file `internal/detective/summary.go` with `caseSummary(c *Case) CaseSummary` (status, phase,
   reason, effort, topic, usage, profile, counts, latest revision, latest source ID, auto-update) and
   `CaseSummary` JSON tags matching today's `LiveView` fields.
2. `saveCaseTx` writes `summary` (new `TEXT NOT NULL DEFAULT ''` column) from that projection.
3. `migrateDatabase`: accept `user_version <= 4`; v4 branch adds the column (idempotent
   `pragma_table_info` check) and backfills every case in one transaction through `caseSummary`
   (Go-side, one implementation).
4. `backupBeforeArchive` becomes `backupBeforeMigration(db, path, version, target)`; v4 creates
   `detective.db.pre-v4-<ts>.bak` with the same `VACUUM INTO` + chmod + fsync sequence.
5. `Live()` reads `summary` only (single row read, no body); `list(summary=true)` reads `summary`
   and falls back to the current `json_remove` query when the column is empty for a row.
6. `LiveView.Reports`: remove the duplicated `latest_revision` selection and report the real count
   from the summary (or drop the field; the UI never reads it).

**Tests (first).**
- `TestMigrateV3ToV4BackfillsSummary`: build a v3 database fixture with 2 cases (one large, one
  empty), open with `New`, assert `user_version = 4`, a `.pre-v4-*.bak` exists, and
  `caseSummary(loaded)` equals the stored `summary` semantically.
- `TestCaseSummaryMatchesProjection` for every save path (`saveLocked`, `startLocked`, `setAutoUpdate`).
- `TestLiveReadsSummaryOnly`: corrupt the body JSON of a case (raw SQL) while keeping a valid
  summary; `Live()` must still answer.
- Benchmark `BenchmarkCaseHotPaths` (Live, list, state write) with `-run '^$'`.

**Verify.** `go test ./internal/detective -count=1`, `go test ./internal/detective -bench CaseHotPaths -run '^$'`.

**Risk.** The migration runs at startup; 100 large cases cost a few seconds once. Log the duration.
An older binary refuses `user_version 4` — rollback means restoring the backup (document in
`documentation/detective.md`).

**DOX.** `internal/detective/AGENTS.md` (schema version, summary column, backup contract),
`documentation/detective.md` §"Reports and exports" (database paragraph and migration note).

### D7 — Working set without excerpts, bounded case projection

**Problem.** Every state write marshals and writes all excerpts; `GET /cases/{id}` returns them;
the UI re-downloads the whole case on each new source (`detective.js:344-352`).

**Changes.**
1. `recordSource` (`evidence.go:53-106`) appends the source to `c.Sources` **without** `Excerpt`
   (the returned `Source` keeps it for the model and for the archive insert).
2. `AddFinding`/`Submit`/`sealReport` keep working unchanged: quotes are validated against
   `detective_sources`, and `reportSources` only needs the metadata.
3. Case-level projections: `Get` returns the body with `Sources[].Excerpt = ""` and `Findings`
   omitted (the UI reads `report.findings` only, `detective-views.js:10`), plus `source_count` /
   `finding_count`; new `CaseView` type in `internal/detective/types.go` or a projection function in
   the handler (prefer the handler, so the store stays transport-free).
4. `migrateArchive`/v4 migration also strips excerpts from migrated bodies (they are already in
   `detective_sources`; assert equality with the archive row in the migration test).
5. Optional sub-item: cap working-set finding `Text` at 600 and `Quote` at 300 runes (the prompt
   limits are 500/240, `detective_service.go:412-420`); full values stay in `detective_findings` and
   in every report snapshot.
6. UI: the sources tab renders title/status/date from metadata and fetches the excerpt on first
   expand via `/source?id=` (the `data-ref` path at `detective.js:439-455` already does this); cache
   per case in memory for the window lifetime.

**Tests (first).**
- `TestCaseBodyStaysBounded`: 400 sources × 32 KB evidence through `RecordSource`, then assert the
  stored body is < 256 KB and contains no evidence text.
- `TestCaseResponseOmitsExcerpts`: HTTP test asserting `GET /cases/{id}` has `excerpt: ""`/absent and
  no `findings`, while `GET …/source?id=` still returns the full excerpt and `/findings` pages.
- Adapt `TestActiveTimeSurvivesRestart` (`detective_test.go:325-337`): the body no longer contains
  the excerpt; assert instead that `/source`/`detective_sources` still holds it byte-for-byte.
- Adapt `TestHeartbeatDoesNotRewriteEvidence` (body-size assertion stays valid).

**Verify.** `go test ./internal/detective ./internal/server -run 'Detective|Case|Source'`, UI browser
smoke for the sources tab.

**DOX.** `internal/detective/AGENTS.md` (working set/archive contract), `ui/js/desktop/apps/AGENTS.md`
§Detective (source rendering), `documentation/detective.md` §"Reports and exports" + API block.

### D8 — Hot-path guards and locking decision

**Problem.** The audit's headline cost has no regression guard, and `Service.mu` serializes polls,
exports and the runner.

**Changes.**
1. `internal/detective/performance_test.go`: structural guards from D6/D7 plus
   `BenchmarkCaseHotPaths` (small/medium/max fixtures, documented in a comment).
2. Re-measure after D6/D7. If `Live`/`List` stay under 2 ms and the state write under 15 ms at
   target sizes, keep `sync.Mutex` and record the measurement in the task's commit/PR text.
   Otherwise switch `Service.mu` to `sync.RWMutex` (read lock: `Get`, `Live`, `List`,
   `Archive`, `Source`, `ReportRevision`, `resolvedReport`; write lock everywhere else) and add
   `TestConcurrentLiveDuringWrite` (a slow write must not block a `Live` beyond the measure).
3. Evaluate `PRAGMA synchronous=NORMAL` only with a comment on the WAL durability window and only
   when the state write stays above target; migrations keep `FULL`.

**Tests.** The guards above; `go test -race ./internal/detective`.

**Verify.** `go test -race ./internal/detective -count=1`, bench output attached to the commit.

**DOX.** `internal/detective/AGENTS.md` (concurrency note + measured budget).

### D9 — UI transport, visibility and idle polling

**Problem.** The app polls `/cases`, `/live` and `/events` every 3 s per window and additionally the
full case whenever sources or the report change; it keeps polling while the window is hidden and
while the "new case" form is open.

**Changes.**
1. New additive endpoint `GET /cases/{id}/state?after=<event_id>` returning `{live, events, next}`
   (one round trip, same data as `/live` + `/events`); keep the old endpoints for compatibility.
2. Poll budget: 3 s while the selected case is queued/running/waiting, 10 s otherwise, no polling
   while `document.hidden` or the Desktop window is minimized (listen to the existing visibility
   signal), and no list poll while the form is open.
3. After D7, the "full case" refresh only happens on a revision change, not on every new source.
4. No ETag on the case list: the shared `api()` helper (`desktop-foundation.js:1065-1072`) treats
   any non-2xx status as an error, so a `304` would need a separate fetch path for little gain now
   that the list response is small (D6). Revisit only if the list still shows up in a profile.

**Tests.** Server: `TestDetectiveStateEndpointMatchesLiveAndEvents` (same payload, cursor advance,
404 for unknown case, unchanged old endpoints). UI: node test for the interval/visibility decision
(`nextPollDelay(...)`, `shouldPoll(...)` as pure functions), browser smoke for the flow.

**Verify.** `go test ./internal/server -run TestDetectiveState`, node regressions, browser smoke.

**DOX.** `internal/detective/AGENTS.md` (live/state contract), `ui/js/desktop/apps/AGENTS.md`
§Detective (polling paragraph), `documentation/detective.md` §"Using the app".

---

## Wave 3 — run cost and result semantics

### D10 — Compact capture receipts

**Problem.** `detectiveCapture` appends the full excerpt (up to 32 KB) to every tool result even
though `web_scraper`/`api_request` already return that text, doubling prompt tokens per page.

**Changes.**
1. `detective_policy.go:213` builds the receipt with `id`, `url`/`locator`, `title`, `status`,
   `sha256`, `retrieved_at` and `excerpt` capped at 400 runes.
2. The skill text (`internal/detective/skills/aurago-detective/SKILL.md:19-23`) and
   `documentation/detective.md` §"Research and evidence" state that the receipt is a excerpt preview
   and that quotes are validated against the stored source, not the preview.
3. `AfterTool` keeps returning the original tool output untouched.

**Tests.** `TestDetectiveCaptureAppendsBoundedReceipt` (a 32 KB page yields a receipt whose appended
JSON is < 2 KB, contains the ID and hash, and the stored excerpt is still 32 KB), plus the existing
`TestDetectiveCaptureBindsOnlyRetrievedDocuments`.

**Verify.** `go test ./internal/server -run TestDetectiveCapture`.

**DOX.** `internal/detective/AGENTS.md` (prompt copy limits), skill text, docs.

### D11 — Localized error codes and PDF capability

**Problem.** `detectiveError` forwards English text; the UI shows it verbatim. PDF export fails for
hi/zh/ja only at click time.

**Changes.**
1. `detectiveError` emits `{"error": <english>, "code": <stable>, "params"?: {…}}`. Map the
   sentinels (`ErrNotFound` → `not_found`, `ErrConflict` → `conflict`, `ErrBudget` → `budget_exhausted`,
   `ErrClosed` → `unavailable`) and the export/validation errors via typed errors (`export.go`
   returns `*ExportError{Code: "pdf_font_unsupported"|"revision_too_large"|…}`).
2. UI: `desktop.detective_error_<code>` with `{…}` interpolation where needed, fallback to the
   server text; all 16 `ui/lang/desktop/*.json`.
3. `/capabilities` gains `pdf_backend` (`builtin`|`gotenberg`) and `pdf_scripts`
   (`latin,greek,cyrillic`); the export bar shows a hint and offers DOCX when the case language is
   outside those scripts (best effort: compare `request.language` against a small allowlist, and
   always keep the DOCX link).

**Tests.** `TestDetectiveErrorCodes` (table per sentinel/export error), `TestDetectivePDFCapability`,
translation completeness (existing `TestDetectiveTranslations` covers non-empty values; add a check
that every code used in `detectiveError` has a key).

**Verify.** `go test ./internal/server -run 'TestDetectiveError|TestDetectivePDF'`,
`go test ./ui -run TestDetectiveTranslations`.

**DOX.** `internal/detective/AGENTS.md` (error envelope), `documentation/detective.md`
§"Reports and exports".

### D12 — Provisional revisions and append base

**Problem.** A failed Deepen/Continue publishes a partial revision that becomes `LatestRevision`,
the default export and the auto-update base.

**Changes.**
1. `Report.Provisional bool` (`json:"provisional,omitempty"`); `complete()` sets it for automatic
   partials of non-initial runs and reports `provisional: true` in the event text.
2. `Case.LatestCompleteRevision int` maintained in `Submit`/`sealReport`/`complete`; the v4 summary
   carries both.
3. `startLocked("update")` bases `Run.BaseRevision` on `LatestCompleteRevision`;
   `SetAutoUpdate`/`nextUpdate`/`queueDueLocked` use it instead of `LatestRevision`.
4. UI: the revision selector marks provisional entries, defaults to the newest complete revision,
   Autor uses the visible revision, and the report tab shows a notice for a provisional revision.

**Tests.** `TestFailedDeepenKeepsCompleteAsDefault` (complete → deepen fails with findings → newest
revision provisional, `LatestCompleteRevision` unchanged, auto-update still schedulable),
`TestAppendUpdateBasesOnLatestCompleteRevision`, `TestExportsUseTheSelectedProvisionalRevision`
(exports stay immutable and addressable).

**Verify.** `go test ./internal/detective -run 'Provisional|Complete'`.

**DOX.** `internal/detective/AGENTS.md` (revision contract), `documentation/detective.md`
§"Reports and exports" + §"Automatic Update".

### D13 — Answers and token/cost visibility

**Problem.** `Case.Answers` is never rendered; `Usage` tokens are recorded but invisible.

**Changes.**
1. Activity tab: answer entries (`answer` event or paired question/answer block) from `c.answers`
   and the `question` events; keep 16 locales.
2. Status line: `tokens in/out` (and `cached`) from `Usage`; the case list shows the token total per
   case; the automatic panel shows the update's last token usage.
3. No pricing logic — the budget tracker already owns costs elsewhere; only show token counts here.

**Tests.** Node view test for the answer/cost renderers, `TestDetectiveTranslations`.

**Verify.** `node scripts/test-ui-regressions.mjs`, browser smoke.

**DOX.** `ui/js/desktop/apps/AGENTS.md` §Detective.

### D14 — Desktop notifications for update, question and failure

**Problem.** The app declares the `notifications` permission but never emits; a published automatic
update or a pending question goes unnoticed with the window closed.

**Changes.**
1. `internal/server/detective_notify.go`: `notifyDetective(s, event, caseID, title, message)` using
   the `showPersonAlertToast` pattern (`broadcastDesktopEvent` with
   `Type: "notification"`, `Payload{appId: "detective", context: {case_id}}`), gated on
   `VirtualDesktop.Enabled` and called only on publish, `no_changes`, question and terminal failure.
2. `DetectiveApp.open(windowId, context)` selects `context.case_id` (same shape as
   `network-cameras`); register it in `menus-and-routing.js` next to the existing early branch.
3. Rate limit: at most one notification per case per minute (in-memory, like the person-alert slots).

**Tests.** `TestDetectiveNotificationOnPublish` and `TestDetectiveNotificationSuppressedWhenDisabled`,
`TestDetectiveNotificationRateLimited`; UI lifecycle test for `open` with a context.

**Verify.** `go test ./internal/server -run TestDetectiveNotification`, `go test ./ui -run TestDesktopDetective`.

**DOX.** `internal/server/AGENTS.md` (if the notification contract is added there),
`ui/js/desktop/apps/AGENTS.md` §Detective, `documentation/detective.md`.

---

## Wave 4 — administration and durability

### D15 — `extra_read_operations` in Config

**Problem.** The security-relevant allowlist exists only in YAML, without validation or preview.

**Changes.**
1. `GET /api/desktop/detective/read-operations` (admin, GET): `{tools: [{name, label?, operations:
   [string]}]}` from `agent.ConfiguredToolSchemas` + `detectiveToolKnown`; operations from the
   schema's `operation` enum where present; plus `free_form: ["mcp_call","composio_call"]`.
2. `ui/cfg/detective.js`: one row per tool with a multi-select of operations and a free-text field
   for MCP (`server/tool`) and Composio (slug) entries; unknown existing entries are shown as
   "unknown" and removable; help text explains that case-level selection is still required.
3. `internal/config/detective.go`: `NormalizeDetective` (trim, drop empty strings, drop empty
   slices) called from `config.Load` and the config-save path.
4. `ui/lang/config/detective/*.json` in all 16 locales.

**Tests.** `TestDetectiveReadOperationsEndpoint` (names only, admin only, disabled integration),
`TestNormalizeDetectiveReadOperations`, config round-trip (`PUT /api/config` keeps the map),
`TestDetectiveConfigTranslations` (16 locales complete).

**Verify.** `go test ./internal/server -run TestDetectiveReadOperations`,
`go test ./internal/config -run TestNormalizeDetective`, `go test ./ui -run TestDetective`.

**DOX.** `documentation/detective.md` §"Research and evidence" (YAML example becomes "or Config"),
`config_template.yaml` comment, and the Config section documentation in `ui/js/config/main.js` if it
describes the section list.

### D16 — `sqlite.detective_path` and init visibility

**Problem.** The store path derives from `sqlite.game_maker_path`, and an initialization failure is
only logged while the UI shows a generic "unavailable".

**Changes.**
1. `internal/config/config_types.go`: `DetectivePath string \`yaml:"detective_path"\`` next to
   `GameMakerPath`; `config.go` default `./data/detective.db` + `resolvePath` (same pattern as
   `:1651`); `config-merger` keeps the legacy derivation when the key is absent.
2. `detective_service.go`: use the configured path; drop the Game Maker fallback but keep the
   legacy derivation when the key is empty and `detective.db` already exists next to Game Maker.
3. Log the resolved path once at startup (`slog.Info`, no secrets).
4. `/capabilities`: `initialization_error` (a stable code from a small whitelist, never the raw
   error) so the UI can show `desktop.detective_init_failed_<code>`.
5. `config_template.yaml` documents the key.

**Tests.** `TestDetectivePathDefaultsAndOverride`, `TestDetectivePathKeepsLegacyLocation`,
`TestDetectiveCapabilitiesInitializationError`.

**Verify.** `go test ./internal/config ./internal/server -run 'DetectivePath|DetectiveCapabilities'`.

**DOX.** `documentation/detective.md` §"Reports and exports" (the "Cases live in …" paragraph),
`config_template.yaml`.

### D17 — A11y, confirm dialog, retention, URL wording, icon

**Problem.** Heading order (h2 status → h1 report → h3 blocks), `aria-pressed` tabs, an over-broad
`aria-live` region, a `window.confirm` fallback, an ever-growing `detective_keys` table, a
misleading "public URL" error, a shared icon.

**Changes.**
1. `detective.js`/`-views.js`: status title `h1`, report title `h2`, block headings `h3`;
   `role="tablist"`/`role="tab"`/`aria-controls`/`aria-selected`; a narrow `aria-live` region for
   status and phase only; drop the `window.confirm` fallback (require `ctx.confirmDialog`).
2. `store.go`: keep the newest 50 `detective_keys` rows per case and delete older ones after a
   successful start (`DELETE … WHERE case_id=? AND rowid NOT IN (SELECT rowid … ORDER BY rowid DESC
   LIMIT 50)`). The table has no timestamp column, so a retention window would need a schema change;
   the rowid cap bounds growth without altering retry semantics for recent runs. `eventLocked`
   keeps its 500 cap.
3. `evidence.go:21`: reword to "source must be an HTTP(S) URL without credentials"; note that the
   tool layer decides reachability.
4. `internal/desktop/types.go`: own icon key (`detective`) plus the matching Papirus/WhiteSur
   manifests, or keep `search` and document the choice — decide in review; default: own icon.

**Tests.** `TestDetectiveKeyRetention`, `TestDetectiveSourceURLErrorMessage`, the existing
registration/icon tests, `TestDesktopDetectiveBrowser` (a11y attributes asserted in the fixture).

**Verify.** `go test ./internal/detective -run Retention`, `go test ./ui -run TestDesktopDetective`,
`go test ./internal/desktop -run TestBuiltinApp`, `node --check`.

**DOX.** `ui/js/desktop/apps/AGENTS.md` §Detective, `documentation/detective.md` if user-visible.

---

## Wave 5 — test hardening

### D18 — Migration, automation matrix, browser coverage

**Changes.**
1. `internal/detective/migration_test.go`: a v2 database fixture (raw SQL: `user_version=2`, one
   table with two cases, archive tables absent) that runs the full v2→v3→v4 path with a backup,
   asserting IDs, revision numbers, `published_revision` backfill, summary equality and that a second
   open does not migrate again.
2. `TestSetAutoUpdateMatrix`: enabled/disabled × draft/running/waiting/partial/failed, asserting
   `next_run_at`, outcome and that disabling cancels an active automatic run.
3. `TestLiveUnderAutomaticUpdate`: outcome transitions `queued → running → updated/no_changes`, and
   that `no_changes` changes neither revision nor report.
4. Browser fixture: revision switch → new case (the D1 regression), source expansion after D7 (lazy
   excerpt), provisional marking after D12. Runs with `AURAGO_RUN_BROWSER_SMOKE=1` in CI.

**Verify.** `go test ./internal/detective ./ui -count=1` plus the browser suite in CI.

**DOX.** `internal/detective/AGENTS.md` §Verification.

---

## Coverage matrix

| Audit finding | Task |
| --- | --- |
| 4.1 poll/write cost | D6, D7, D8, D9 |
| 4.2 stale revision | D1 |
| 4.3 full case payload | D7 |
| 4.4 duplicated excerpts in prompts | D10 |
| 4.5 revoked permission starts and fails | D5 |
| 4.6 Stop coupled to `ready` | D3 |
| 4.7 startup loads all bodies | D4 |
| 4.8 English errors | D11 |
| 4.9 allowlist only in YAML | D15 |
| 4.10 `crypto.randomUUID` | D2 |
| 4.11 worker busy loop | D4 |
| 4.12 `LiveView.Reports` | D6 |
| 4.13 PDF script limits | D11 |
| 4.14 provisional revision becomes latest | D12 |
| 4.15 answers invisible | D13 |
| 4.16 no notifications | D14 |
| 4.17 no token/cost view | D13 |
| 4.18 polling while hidden | D9 |
| 4.19 DB path, init error, `block_reason` | D3, D16 |
| 4.20 accessibility | D17 |
| 4.21 confirm/icon/keys/URL wording | D17 |
| 4.22 test gaps | D18 |

## Risk and rollback

- **Schema v4** is the only irreversible step. It writes `detective.db.pre-v4-<ts>.bak` (synced),
  runs in one transaction, and aborts opening on any failure. Older binaries refuse a v4 database
  (`schemaVersion > 4`), so a rollback restores the backup; the plan documents this in
  `documentation/detective.md` and the startup log names the backup path.
- **Projection change (D7)** is a read-shape change, not a data change: excerpts stay in
  `detective_sources`, so reverting the code restores the old responses without data loss.
- **Provisional revisions (D12)** are additive: `latest_revision` keeps its meaning, so an old
  client still sees the newest published revision.
- **Config keys (D15/D16)** are additive with empty defaults; `config-merger` preserves the legacy
  behavior when the keys are absent.
- **UI changes** (D1, D2, D3, D9, D13, D17) are local to the lazy app module and reversible per
  commit; no bundle rebuild is needed for `detective.js` (the main bundle only changes for
  `menus-and-routing.js` in D14 → run `npm run build:ui` there).
- **D14** touches the shared notification path; keep it additive (new event payload fields only) and
  verify the person-detection and flow-failure notifications still pass their tests.

## Verification

Per task: the focused tests listed above. Before the final commit of each wave:

```bash
go test ./internal/detective -count=1
go test ./internal/config -count=1
go test ./internal/server -run 'Detective' -count=1
go test ./ui -run 'Detective|DesktopDetective' -count=1
node scripts/test-ui-regressions.mjs
node --check ui/js/desktop/apps/detective.js
node --check ui/js/desktop/apps/detective-views.js
```

Plus, once per wave:

```bash
go test -race ./internal/detective -count=1
go test ./internal/detective -bench CaseHotPaths -run '^$' -benchmem
AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run TestDesktopDetective -count=1
```

Manual acceptance after wave 2 and wave 3: create a case with a long source list, confirm the
sources tab lazily expands, export all three formats for the selected revision, run an automatic
update to completion, force a failure by revoking read permission mid-run, and reload the desktop
after a server restart (case must show `interrupted` with the resume path intact).

## Execution record

Filled in while executing: one line per task with commit hash, tests run and any deviation from this
plan.

### Wave 1 — D1–D5 (2026-10-23)

Each task is one commit on `main`; the failing test came first.

| Task | Commit | Tests | Deviations |
| --- | --- | --- | --- |
| plan | `ee1a3508c` | — | committed before the first task |
| D1 | `50ff845c2` | `scripts/test-ui-regressions.mjs` (new: revision selection), `go test ./ui -run Detective` | — |
| D2 | `2674bb153` | `scripts/test-ui-regressions.mjs` (new: fake crypto object) | — |
| D3 | `3dbf6cb3b` | `TestDetectiveCapabilitiesBlockReason`, `TestDetectiveTranslations`, node test for `canStop` | `block_reason` has three values (`disabled`, `read_only`, `initializing`); "permission" stayed an automatic-update outcome instead of a capability reason |
| D4 | `5bda7c392` | `TestRestartQueryIgnoresCorruptInactiveCase`, `TestCompleteReturnsPersistenceErrors`, full package | the backoff has no direct test: a persistent write failure also breaks the queue SELECT, so the loop cannot be reproduced deterministically |
| D5 | `104e0edd9` | `TestQueuedCaseWaitsForPermission`, `TestPermissionRevokedRunStaysResumable`, `TestDetectiveTranslations` | — |

Notes for the following waves:

- Two UI regression tests fail at `HEAD` before this work (`Game Maker asset selection…:
  bindModelLimits is not defined`, `Game Maker forwards runtime diagnostics…: fail is not defined`).
  Reproduced against `git show HEAD:scripts/test-ui-regressions.mjs`; out of scope here.
- `go test -race` cannot run in this environment: the repository builds with `CGO_ENABLED=0`
  (modernc SQLite) and `-race` answers `requires cgo`. Concurrency evidence therefore comes from the
  focused tests, not the race detector.
- The sixteen `ui/lang/desktop/*.json` files carried uncommitted homepage-studio keys from a
  parallel workstream. The Detective locale lines were staged hunk-wise (`git apply --cached` with a
  filtered patch), so no foreign line entered these commits; the working tree kept the other lines.

### Wave 2 — D6–D8 (2026-10-23)

| Task | Commit | Tests | Deviations |
| --- | --- | --- | --- |
| D6 | `4f9a54c93` | `TestMigrateV3ToV4BackfillsSummary`, `TestLiveReadsSummaryOnly`, `TestCaseSummaryMatchesProjectionOnEverySave`, the existing v1/v2 migration tests | **DD2 changed**: the summary became its own table (`detective_summaries`) **and** the migration rebuilds the case row with `body` last. Reason: SQLite walks the overflow chain of a large value for every column stored after it, so a summary *column* behind the 4 MB body still cost 8.4 ms per read; with the split plus the new column order the same read costs 69 µs. `LiveView.reports` was removed (it held `latest_revision`, not a count) |
| D7 | `13ee44b65` | `TestRecordSourceKeepsExcerptInArchiveOnly`, `TestCaseBodyStaysBounded`, `TestDetectiveCaseResponseStaysBounded`, node `Detective source text loads on expand` | `TestActiveTimeSurvivesRestart` stayed unchanged: it seeds the working set directly, so it still proves that a save/load cycle keeps whatever the store is given. The UI fetches a source excerpt on first expand instead of the case payload carrying it |
| D8 | `18e2a6204` | `BenchmarkCaseHotPaths`, full package | DD3 decision recorded: the single mutex and `synchronous=FULL` stay, because reads are in the microseconds; a read/write lock only above ~5 ms per read |
| D9 | `ba4fa02e7` | `TestDetectiveStateEndpointMatchesLiveAndEvents`, node `Detective polling backs off and pauses when hidden` | `/live` and `/events` stay as separate routes; the app polls `/state?after=` plus the case list (two requests), never the list while the form is open |

Measurements after D6/D7 (200 sources, 20 KB excerpt each, ~4 MB of evidence):

| Path | Audit | Now | Target |
| --- | ---: | ---: | ---: |
| `Live()` | 242 ms | 65 µs | < 2 ms |
| `ListSummaries()` | 28 ms | 76 µs | < 1 ms |
| one state write | 114 ms | 2.8 ms (bench) | < 15 ms |
| working-set body | 4.0 MB | 56 KB | < 256 KB |
| case payload | ~4 MB | ~30 KB | < 150 KB |

`Live` and the list are now independent of the evidence size (0 vs 200 sources differ by
~10 µs). The remaining write cost is the SQLite commit, not the payload.

Notes for the following waves:

- `go test ./ui -run Detective` and the node suite stay green; the two foreign Game-Maker failures
  and the unavailable `-race` run are unchanged from wave 1.
- The plan's DD2 wording ("summary column") is superseded by the table in D6; the reason is the
  SQLite row layout, not a preference.

### Wave 3 — D10–D14 (2026-10-23)

| Task | Commit | Tests | Deviations |
| --- | --- | --- | --- |
| D10 | `c285c1b3b` | `TestDetectiveCaptureAppendsBoundedReceipt`, existing capture test | The receipt keeps a 400-rune preview instead of no excerpt at all: the model can quote from the tool output it already saw, and the server still validates against the full stored excerpt. Both append sites (MCP receipts and the final block) share `detectiveSourcesAppend` |
| D11 | `6c88f970a` | `TestDetectiveErrorCodes`, `TestDetectiveCapabilitiesAdvertiseThePDFWriter`, `TestExportRefusalsCarryStableSentinels`, node `Detective errors and export formats follow the API codes` | **Audit 4.13 corrected**: nothing calls Document Creator or Gotenberg, so the old advice to configure it could not work. The error text, docs and 16 locales now say "use Word or Markdown"; the capability hint replaces the configuration hint. Codes are `not_found`, `conflict`, `budget_exhausted`, `copy_exists`, `permission_revoked`, `unavailable`, `unsupported_format`, `export_too_large`, `pdf_unsupported_font`, `invalid_request` |
| D12 | `27d1b4569` | `TestFailedDeepenKeepsCompleteAsDefault`, `TestAppendUpdateBasesOnLatestCompleteRevision`, `TestExportsUseTheSelectedProvisionalRevision`, node `Detective provisional reports stay out of the default` | The status event text stays the plain reason code (the UI localizes reason codes, so appending English "(provisional)" would break that); the provisional state reaches the UI through `Report.Provisional` and `latest_complete_revision` instead. `completeRevision()` derives the value for rows written before the field existed |
| D13 | `84a751f3c` | `TestAnswerIsAnActivityEvent`, node `Detective token counts are visible` | The automatic panel shows `c.run.usage` (the last run), because no per-update usage snapshot exists; token counts only, no pricing |
