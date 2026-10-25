# RuView P1 (shell + Status) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan batch by batch (one maker-agent dispatch per batch, one combined review per batch with Critical/Important findings only). Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the RuView overview/devices/iframe tab strip with a 44 px command bar, a calm Status view (one headline answers "is it working?", one next step, setup stepper, collapsed details), a Boards view (today's device list, restyled and keyed) and a minimal Lab landing. Add the backend fields and typed errors that drive it: §8.1 status fields, §8.4 error codes and HTTP statuses, and the fixes M27 (first), M29, M23, M24 and M28.

**Architecture:** The backend keeps `Status()` passive and adds derived fields: `runtime`, `phase`, `error_code`/`error_detail`, `stop_reason`, storage numbers, `hints` and timestamps. Two locks change roles: a non-blocking lifecycle slot (`m.op`) serializes long starts and installs, and `opMu` guards only short state changes, so a pull or download never holds it. Errors become `*ruview.Error{Code, Detail, HTTP}`, and one handler helper writes `{error_code, error, detail}`. The frontend is split into four classic scripts: pure `RuViewModel` (§7 `deriveState`, Node-testable), `RuViewStore` (fetch with timeouts, polling per view only while visible, epochs), `RuViewViews` (Status, Boards, Lab with keyed DOM patching) and `RuViewApp` (composition root, command bar, menus, live region).

**Tech Stack:** Go 1.2x (`internal/ruview`, `internal/server`), vanilla JS classic scripts plus the existing ESM modules (`ui/js/ruview/*.mjs`), CSS container queries and desktop tokens, Node `node:test` + `vm`, Playwright (`scripts/ruview/*.mjs`), go-rod (`ui/*_browser_test.go`, gated by `AURAGO_RUN_BROWSER_SMOKE=1`).

---

## 0. Ground rules for every batch

- Work only in `C:\Users\Andi\Documents\repo\AuraGo\.worktrees\ruview-p1` (branch `feature/ruview-p1`). Use absolute paths. PowerShell is the shell.
- **Implementers never commit, stash, reset or touch files outside their batch's file list.** The controller commits once per batch after review. Other sessions share the main checkout.
- **GitNexus before editing any function** (project `CLAUDE.md`). Command: `node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "<Symbol>" --direction upstream --repo AuraGo`. Report callers and risk in the batch report.
  - Hub symbols (`Status`, `Configure`, `request`, `renderAppContent`) will report HIGH/CRITICAL. Warn and proceed; the edits stay additive.
  - Treat `UNKNOWN` as unresolved and confirm with Grep.
  - The controller runs `detect-changes --scope all --repo AuraGo` before each commit.
- Temporary scripts go only in the session scratchpad (`C:\Users\Andi\AppData\Local\Temp\claude\...\scratchpad`), never in the repo. Write them with the Write tool, not shell heredocs.
- The Write/Edit tools turn backslash-u escape sequences into raw characters. In Go and JS source, write such literals with `\x5c` concatenation, or avoid them.
- If `go build` or `go test` fails with disk-space errors, trim old entries from GOCACHE/GOTMPDIR on `D:`.
- Tests are LEAN. Write only the tests listed here, plus fixes to existing assertions that the restructure breaks.
- Key names, codes, enum values and DOM hooks in §2 are **frozen contracts**. If a batch needs a new key or hook, it reports it instead of inventing silently.

## 1. File map

| File | Batch | Status | Responsibility |
|---|---|---|---|
| `internal/ruview/errors.go` | A | new | `Error` type, `Code*` constants, `HTTPStatus`, `ErrorCodes`, `AsError`, `fail`, `ctxError`, `scrubDetail` |
| `internal/ruview/lifecycle.go` | A | new | `Runtime*` / `Phase*` / `Stop*` constants and enumerators, `runtimeLocked`, phase helpers, `markExitedLocked`, `inspectExit` (M24) |
| `internal/ruview/storage.go` | A | new | throttled `sampleStorage` and forced `storageCheck` (M28); `directoryBytes` moved here (missing root = 0) |
| `internal/ruview/manager.go` | A | edit | §8.1 fields, new Manager fields, M27 `Configure`, `Status()` derivations, `test()` codes and `last_fresh_at`, `watch()` rewrite, `noteRecording` time, `Close` reason |
| `internal/ruview/runtime.go` | A | edit | M29 `start` flow, `ensureImage`/`createContainer` split, native verify/launch split, `stop(ctx, reason)`, M23 removal and conflict |
| `internal/ruview/artifacts.go` | A | edit | `Install` uses the op slot, `downloading` phase and codes |
| `internal/ruview/client.go` | A | edit | `request` returns typed errors (`errInvalidReply` stays matchable) |
| `internal/ruview/read.go` | A | edit | `readStatusError` delegates to `scrubDetail` |
| `internal/ruview/nodes.go` | A | edit | codes for disabled/readonly/invalid id |
| `internal/ruview/proxy.go` | A | edit | recording start uses `storageCheck`: unreadable → 503, full → 507 |
| `internal/ruview/hints.go` | A | edit | `openHintsLocked()` |
| `internal/server/ruview_handlers.go` | A | edit | `ruviewError(w, err)`; replace blanket 502 and string 403/400 |
| `internal/ruview/{lifecycle,ruview,runtime,hints,background}_test.go` | A | edit | broken assertions plus M27/M29/M23/M24 tests |
| `internal/ruview/status_fields_test.go` | A | new | M28, field table, `scrubDetail` |
| `internal/server/ruview_errors_test.go` | A | new | code→HTTP table, route refusal codes, D4 single-field save |
| `ui/js/desktop/apps/ruview-model.js` | B | new | `RuViewModel` (pure) |
| `scripts/ruview/model.test.mjs` | B (+C) | new | Node table tests (one per §7 row, plus helpers); C appends CSS/budget/Go-code checks |
| `ui/js/ruview/upstream-adapter.js` | B | edit | status fetch timeout; `simulation` reset on failure |
| `ui/js/vendor/ruview/aurago-adapter.js`, `ui/js/vendor/ruview/provenance.json` | B | edit | byte copy of the adapter plus its new sha256 |
| `scripts/ruview/upstream.test.mjs` | B (+C) | edit | B: hung-status test. C: `ruview.js` → `ruview-views.js` path at :194 |
| `ui/js/desktop/apps/ruview-store.js` | C | new | `RuViewStore` |
| `ui/js/desktop/apps/ruview-views.js` | C | new | `RuViewViews.status/boards/lab` |
| `ui/js/desktop/apps/ruview.js` | C | rewrite | `RuViewApp` composition root, command bar, menus, live region |
| `ui/css/ruview.css` | C | rewrite | tokens only, container queries, wizard part with `--pw-*` chains (no hex) |
| `ui/js/desktop/core/module-loader.js` | C | edit (:28) | four scripts in order |
| `ui/js/desktop/core/menus-and-routing.js` | C | edit (:1654) | Infra-style shell context |
| `ui/js/desktop/bundles/main.bundle.js` | C (E check) | generated | `npm run build:ui` |
| `ui/i18n_lint_test.go` | C | edit (:1075) | register four files in `desktopLocalI18nHelpers` |
| `ui/lang/ruview/en.json`, `de.json` | C | edit | new keys (§2.7); remove 3 unused keys |
| `scripts/ruview/browser.test.mjs`, `scripts/ruview/ui-browser.mjs` | C | edit | new harness/hooks plus the 7 new checks |
| `scripts/ruview/wizard-browser.test.mjs` | C | edit | harness loads four scripts; opens the wizard from Boards |
| `ui/lang/ruview/{cs,da,el,es,fr,hi,it,ja,nl,no,pl,pt,sv,zh}.json` | D | edit | translations of every new key, capitalisation fix, 3 keys removed |
| `internal/server/ruview_i18n_test.go` | D | new | `TestRuViewErrorCodesTranslated` (+ locale parity) |
| `documentation/ruview.md`, `documentation/manuals/{en,de}/30-ruview.md`, `internal/ruview/AGENTS.md` | E | edit | behaviour contract, manuals, package DOX |

`module-loader.js` is **not** a bundle part; `desktop.html:131` loads it directly. `menus-and-routing.js` is a part of `main.bundle.js` (`scripts/build-ui-bundles.js:62`), so editing it requires `npm run build:ui`.

## Batch order

```
Wave 1:  A (Go backend)            ∥  B (pure model + adapter fix)      disjoint files
Wave 2:  C (shell/store/views/CSS/en+de/browser tests)  ∥  D (14 locales + Go i18n test)   disjoint files
Wave 3:  E (docs, bundle check, vendor check, full test list)
```
- C needs B (the model) and the contracts. It mocks the backend, so it does not need A merged, but the controller should start wave 2 after both A and B are committed.
- D needs A's constants and translates the frozen en/de texts from §2.7.
- The cross-batch tests (`TestRuViewErrorCodesTranslated`, `TestDesktopUsedI18nKeysExistInAllLanguages`, the Go-code cross-check in `model.test.mjs`) pass only after A, C and D are all merged. The controller runs them before committing D (or C, whichever lands second).

---

## 2. Frozen contracts

### 2.1 `Status` additions (`internal/ruview/manager.go:20-44`)

Add these fields after `RecordingAck`. Keep every existing field, including `Error` (`json:"error,omitempty"`) for compatibility.
```go
	Runtime            string     `json:"runtime"`              // Runtime* (§2.3); always set
	Phase              string     `json:"phase"`                // Phase* or ""
	PhaseSince         *time.Time `json:"phase_since"`          // when Phase was entered; null when Phase == ""
	ErrorCode          string     `json:"error_code"`           // Code* or ""
	ErrorDetail        string     `json:"error_detail"`         // scrubbed, ≤240 bytes, no URL/path; "" if none
	LastFreshAt        *time.Time `json:"last_fresh_at"`        // last stored probe with fresh && !simulated; reset only by runtime-relevant Configure
	StopReason         string     `json:"stop_reason"`          // Stop* or ""
	StoppedAt          *time.Time `json:"stopped_at"`           // time of StopReason (needed for §7 row 6 "Stopped by you at 14:02")
	StorageUsedBytes   *int64     `json:"storage_used_bytes"`   // last successful walk; null for external, before the first walk, or after a failed walk
	StorageBudgetBytes *int64     `json:"storage_budget_bytes"` // StorageMB<<20 for docker/native; null for external
	RecordingAt        *time.Time `json:"recording_at"`         // time of the last confirmed (2xx) forwarded recording start/stop
	Hints              []string   `json:"hints"`                // open hint names, sorted; [] when none (never null)
```
Error precedence for `error_code` is:
1. lifecycle (`stop_failed`, `process_exited`)
2. start/install failure
3. probe (`source_mismatch`, `unexpected_simulation`, connection codes)
4. `storage_unreadable`, set only when nothing else is set.

### 2.2 Error codes and HTTP (`internal/ruview/errors.go`)

```go
const (
	CodeDisabled             = "disabled"              // 403
	CodeReadOnly             = "readonly"              // 403
	CodeExternalRuntime      = "external_runtime"      // 403
	CodeDockerReadOnly       = "docker_readonly"       // 403  Docker integration off or read-only
	CodeAlreadyRunning       = "already_running"       // 409
	CodeNotInstalled         = "not_installed"         // 409
	CodeBusy                 = "busy"                  // 409  another start/install holds the lifecycle slot
	CodeCancelled            = "cancelled"             // 409  generation cancelled by Stop/Configure/shutdown
	CodeContainerConflict    = "container_conflict"    // 409
	CodePlatformUnsupported  = "platform_unsupported"  // 409
	CodeInvalidNodeID        = "invalid_node_id"       // 400
	CodeInvalidConfig        = "invalid_config"        // 400
	CodeUnreachable          = "unreachable"           // 502  transport failure
	CodeUpstreamAuth         = "upstream_auth"         // 502  upstream 401/403
	CodeUpstreamError        = "upstream_error"        // 502  other non-2xx
	CodeDockerUnavailable    = "docker_unavailable"    // 502  Engine transport failure
	CodeImageDenied          = "image_denied"          // 502
	CodeImagePullFailed      = "image_pull_failed"     // 502
	CodeContractUnsupported  = "contract_unsupported"  // 502  invalid JSON / status != ready
	CodeDownloadFailed       = "download_failed"       // 502
	CodeStartFailed          = "start_failed"          // 502  any other start failure (dir, vault, placement, create/start)
	CodeStopFailed           = "stop_failed"           // 502
	CodeProcessExited        = "process_exited"        // 502  status-only in practice
	CodeSourceMismatch       = "source_mismatch"       // 502  status-only
	CodeUnexpectedSimulation = "unexpected_simulation" // 502  status-only
	CodeTimeout              = "timeout"               // 504
	CodeStorageUnreadable    = "storage_unreadable"    // 503
	CodeStorageFull          = "storage_full"          // 507
	CodeArtifactMissing      = "artifact_missing"      // 500  no verified digest/package in this release
	CodeUnknown              = "unknown"               // 500  fallback for untyped errors
)
```
`invalid_name` and `inventory_invalid` belong to P2 and are not declared yet.

```go
// Error is a RuView failure with a stable code. Detail is scrubbed: no token,
// URL or file path, at most 240 bytes. HTTP is the status internal/server writes.
type Error struct {
	Code   string
	Detail string
	HTTP   int
	msg    string // legacy English sentence: Status.Error, the "error" response field, logs (security.Scrub'ed)
	cause  error  // errors.Is/As only; never serialized
}
func (e *Error) Error() string      // msg, else codeText[Code] (+ ": " + Detail)
func (e *Error) Unwrap() error      // cause
func HTTPStatus(code string) int    // table above; unknown codes → 500
func ErrorCodes() []string          // every Code* above, sorted (for tests)
func AsError(err error) *Error      // nil→nil; errors.As *Error (HTTP filled if 0); context.Canceled→cancelled; DeadlineExceeded→timeout; else unknown with scrubbed detail
func fail(code, msg string, cause error) *Error   // Detail = scrubDetail(cause.Error(), 240) when cause != nil
func ctxError(err error) *Error                    // Canceled→cancelled, DeadlineExceeded→timeout
const maxDetailBytes = 240
func scrubDetail(text string, limit int) string
```
`scrubDetail` steps, in order:
1. `security.Scrub`
2. URLs `(?i)\b[a-z][a-z0-9+.-]*://\S+` → `[url]`
3. Windows/UNC paths `(?i)(?:\b[a-z]:[\\/]|\\\\)\S*` → `[path]`
4. absolute Unix paths `(^|[\s"'(=])/[^\s"')]*` → `${1}[path]`
5. `dockerutil.SanitizeOneLine(text, 0)`, then collapse whitespace with `strings.Join(strings.Fields(..)," ")`
6. cut to `limit` bytes at a rune boundary, then `TrimSpace`.

Response body written by `ruviewError` (internal/server):
```json
{"error_code":"image_denied","error":"RuView image pull returned HTTP 403: registry denied","detail":"HTTP 403: registry denied"}
```

### 2.3 Enumerations (`internal/ruview/lifecycle.go`)

```go
const ( RuntimeExternal = "external"; RuntimeNotStarted = "not_started"; RuntimeStarting = "starting"; RuntimeRunning = "running"
        RuntimeStopping = "stopping"; RuntimeStopped = "stopped"; RuntimeExited = "exited"; RuntimeStopUnconfirmed = "stop_unconfirmed" )
const ( PhasePreparing = "preparing"; PhasePullingImage = "pulling_image"; PhaseCreating = "creating"; PhaseWaitingReady = "waiting_ready"
        PhaseDownloading = "downloading"; PhaseVerifying = "verifying" )
const ( StopUser = "user"; StopConfigChanged = "config_changed"; StopProcessExit = "process_exit"; StopAuraGoShutdown = "aurago_shutdown" )
const ( opStarting = "starting"; opInstalling = "installing"; waitingReadyTimeout = 30 * time.Second )
func Runtimes() []string; func Phases() []string; func StopReasons() []string

// runtimeLocked derives Status.runtime; called with mu held. First match wins.
func (m *Manager) runtimeLocked(mode string, running bool) string {
	switch {
	case mode == "external":                                   return RuntimeExternal
	case m.status.DataState == "stopping":                     return RuntimeStopping
	case m.status.Error == CodeStopFailed:                     return RuntimeStopUnconfirmed
	case m.op == opStarting || m.phase == PhaseWaitingReady && running: return RuntimeStarting
	case running:                                              return RuntimeRunning
	case m.status.Error == CodeProcessExited:                  return RuntimeExited
	case m.stopReason != "":                                   return RuntimeStopped
	default:                                                   return RuntimeNotStarted
	}
}
```
Phase sequences:
- Docker start: `preparing → [pulling_image only if the image is missing] → creating → waiting_ready`
- Native start: `preparing → verifying → creating → waiting_ready`
- Install: `downloading`

`waiting_ready` ends at the first stored probe with `Ready`, or after 30 s (cleared by `watch`).

### 2.4 Lock protocol (M29)

- `mu` protects every field and is never held across I/O.
- `m.op` (under `mu`) is the single slot for long lifecycle work. It is claimed only while `opMu` is held, and is never waited on: a second Start/Install returns `busy`.
- `opMu` serializes short state changes: reap, container inspect/remove/create/start, process launch, `stopRuntime`. **It is never held across an image pull, a native download or file hashing.**
- `Stop` cancels the generation first (aborting a pull, download or probe), then takes `opMu`. **Decision: Stop during `starting` works.** The cancellation path already exists (`runtime.go:352-355`, proven by `TestStopAndShutdownCancelAutomaticStart`). With the pull outside `opMu`, Stop acquires the lock immediately, and refusing would leave the user waiting up to 10 minutes. In the UI, "Stop service" stays enabled while starting.
- Lock order is always `opMu → mu`. `emitHint`/`noteStop`/`noteStorage` are called with neither held.

### 2.5 `RuViewModel` (pure; `ui/js/desktop/apps/ruview-model.js`)

```js
window.RuViewModel = {
  VIEWS, STATE_IDS, KNOWN_ERROR_CODES,                 // KNOWN_ERROR_CODES mirrors Go ErrorCodes() + 'network','forbidden'
  runtimeOf(status),                                   // status.runtime, else legacy: external→'external', data_state stopping→'stopping',
                                                       //   error stop_failed→'stop_unconfirmed', data_state starting→'starting', ready→'running',
                                                       //   error process_exited→'exited', data_state stopped→'stopped', else 'not_started'
  errorCodeOf(status),                                 // status.error_code, else legacy error codes, else (status.error ? 'unknown' : '')
  availableViews(status),                              // ['status'] + 'boards' if enabled && source==='esp32' + 'lab' if enabled
  deriveState(input) → Derived,
  blockReason(action, {status, pending, desktopReadonly}) → '' | reason id,
  setupSteps(status, nodes) → null | [{id, state:'done'|'current'|'todo', action}],
  mergeNodes(prev, data) → {list, scope, liveState, error},        // today's applyNodes rule (ruview.js:188-203)
  boardFreshness(node, {source, simulated, nodesError, ageMs}) → 'live'|'stale'|'offline',  // mirrors presenceFresh (read.go:100)
  nodeAgeMs(node, receivedAtPerf, nowPerf) → number|null,
  liveBoardCount(nodes, status, nowPerf) → number|null,
  ageParts(ms) → null | {value, unit:'ms'|'s'|'min'|'h'},          // views format with t('age_<unit>') + Intl.NumberFormat
  formatBytes(bytes, lang) → string ('—' for null),                // Intl unit megabyte/gigabyte, binary division
  formatTime(ms, lang), formatDuration(ms) ('m:ss' / 'h:mm:ss'),
  fill(template, values),                                           // replaces {name}
  phaseStep(mode, phase) → {n,total}|null,
  diagnostics(status, nodes, now) → string,
};
```
Input and output of `deriveState`:
```js
// input:  { status, statusLoaded, statusFailed, statusAt /*Date.now() of last success*/, now,
//           nodes:{list, liveState, loaded, error, liveCount}, views, desktopReadonly, pending:Set }
// output: { id, row, tone:'neutral'|'info'|'warning'|'danger'|'success', glyph:'○'|'▲'|'◇'|'◐'|'✕'|'●', busy:boolean,
//           pill, pillAgeMs, ageMs, action, actionLabel, actionBlock, secondaryAction,
//           secondary:{kind, at?, reason?, n?, total?, sinceMs?}|null,
//           errorCode, errorDetail, phase, boards, overlays:[] }
// ageMs = status.last_frame_age_ms + (now - statusAt)   (client ageing between polls)
```
**Row table.** First match wins. `rt = runtimeOf(status)`, `code = errorCodeOf(status)` with `storage_unreadable` treated as "no code" for rows 6 and 11. Headline keys are in §2.7.

| row | id | condition | tone glyph | pill | action → actionLabel | secondary |
|---|---|---|---|---|---|---|
| 1 | `loading` | `!statusLoaded && !statusFailed` | neutral ○ | checking | — | — |
| 2 | `status_failed` | `statusFailed` | warning ▲ | unavailable | `refresh` → action_try_again | last_success (`statusAt`) if statusLoaded |
| 3 | `off` | `!status.enabled` | neutral ○ | off | `open-settings` → action_open_settings | about |
| 3a | `installing` | `status.phase === 'downloading'` | info ◐ busy | installing | — | phase_since |
| 4 | `readonly_blocked` | mode≠external && readonly && rt∈{not_started,stopped} | info ◇ | read_only | desktopReadonly ? `open-settings` : `allow-changes` → action_allow_changes; secondaryAction `open-settings` | readonly_blocks |
| 5 | `not_installed` | mode==='native' && code==='not_installed' | neutral ○ | not_installed | `runtime/install` → action_install | install |
| 6 | `stopped` | mode≠external && rt∈{not_started,stopped} && !code | neutral ○ | stopped (key `stopped_state`) | `runtime/start` → action_start | stop_reason (`stop_reason`,`stopped_at`) or not_started |
| 7 | `starting_pull` | rt==='starting' && phase==='pulling_image' | info ◐ busy | starting | `runtime/start` → action_starting, block `reason_wait_start` | phase_step (`phaseStep`) |
| 8 | `starting` | rt==='starting' | info ◐ busy | starting | same as 7 | phase_since |
| 9 | `stopping` | rt==='stopping' | info ◐ busy | stopping | — | — |
| 10 | `stop_failed` | code==='stop_failed' \|\| rt==='stop_unconfirmed' | danger ✕ | problem | `runtime/stop` → action_stop_again | stop_hint if hints∋stop_unconfirmed |
| 11 | `error` | code set | danger ✕ | problem | per code group (below) | technical (error_detail) + last_live (`last_fresh_at`) |
| 12 | `external_unchecked` | rt==='external' && !observed_at | neutral ○ | not_checked (key `unchecked`) | `test` → action_check_connection | — |
| 12a | `external_check_old` | rt==='external' && now−observed_at > 10 s | neutral ○ | not_checked + age | `test` → check_now | last_checked |
| 13 | `demo` | ready && simulated && source==='simulated' | info ◇ | demo | views∋live ? `open-live` : — ; secondaryAction `open-settings` | demo |
| 14 | `waiting` | ready && (waiting_for_frames \|\| (source==='esp32' && last_frame_age_ms==null && data_state≠'disconnected')) | info ◐ | waiting | nodes.loaded && list empty && views∋boards ? `flash` → action_add_board : views∋boards ? `show-boards` : — | last_checked |
| 15 | `disconnected` | ready && data_state==='disconnected' | warning ▲ | disconnected | esp32 ? `show-boards` : `open-settings` | last_live |
| 16 | `live` | ready && fresh && !simulated | success ● | live + pillAgeMs | views∋live ? `open-live` : views∋boards ? `show-boards` : — | live_meta (time · source · mode) |
| 17 | `stale` | ready && (data_state==='stale' \|\| (!fresh && last_frame_age_ms > 10000)) | warning ▲ | no_new_data | esp32 ? `show-boards` : — | last_live |
| F | `checking` | otherwise | neutral ○ | checking | `test` → check_now | — |

Row 11 action groups:
- **start**: `image_denied`, `image_pull_failed`, `docker_unavailable`, `start_failed`, `timeout`, `cancelled`, `busy`, `container_conflict` → `runtime/start` with action_retry_start, or `test` with action_check_again for external.
- **start again**: `process_exited` → `runtime/start` with action_start_again.
- **settings**: `docker_readonly`, `invalid_config`, `artifact_missing`, `platform_unsupported`, `source_mismatch`, `unexpected_simulation`, `upstream_auth`, `disabled`, `external_runtime` → `open-settings`.
- **check**: `unreachable`, `upstream_error`, `contract_unsupported`, `unknown` → `test` with action_check_again.
- **install**: `not_installed` → `runtime/install`.
- **allow**: `readonly` → `allow-changes`.

`errorCode` is normalized to `KNOWN_ERROR_CODES`, otherwise `'unknown'`.

Variants:
- Row 16 headline: wifi → `headline_live_wifi`; esp32 with `boards ≥ 1` → `headline_live_board`/`headline_live_boards` (via `Intl.PluralRules(lang).select(n)==='one'`); otherwise `headline_live`.
- Row 14 for wifi → `headline_waiting_wifi`.
- Row 8 without a phase → `headline_starting_plain`.

Overlays (chips; they never replace the headline):
- `read_only` when enabled && readonly && id≠readonly_blocked
- `storage_full` when `storage_limit`
- `storage_unreadable` when `error_code==='storage_unreadable'`

`blockReason`, first match:
1. `pending.has(action)` → `working`
2. action ∈ {runtime/start, runtime/install, flash, allow-changes} && desktopReadonly → `reason_desktop_readonly`
3. action ∈ {runtime/start, runtime/install, flash} && status.readonly → `reason_readonly`
4. action starts with `runtime/` && mode==='external' → `external_runtime`
5. action==='runtime/install' && mode≠'native' → `install_native`
6. action ∈ {runtime/start, runtime/install} && rt==='starting' → `reason_wait_start`
7. action ∉ {refresh, open-settings, copy-diagnostics, about, show-boards} && status && !status.enabled → `enable_first`
8. otherwise `''`.

`setupSteps` returns `null` when `!statusLoaded || status.last_fresh_at || status.fresh || source==='simulated'`. Steps:
- docker/native + esp32: `enabled, writable, service (rt==='running'), receiver (receiver_configured && udp_ready, M30), board (nodes.list.length>0), data`
- docker/native + wifi: `enabled, service, data`
- external + esp32: `enabled, checked (observed_at && ready), board, data`
- external + wifi: `enabled, checked, data`

The first undone step is `current`, the rest are `todo`. Fix actions per step: enabled→`open-settings`, writable→`allow-changes` (or `open-settings` when desktopReadonly), service→`runtime/start`, receiver→`open-settings`, checked→`test`, board→`flash`, data→`show-boards` for esp32, else null.

`diagnostics` output is plain `key=value` lines, language-neutral:
- header `RuView diagnostics <ISO time>`
- `revision`, `revision_verified`, `mode`, `source`, `enabled`, `readonly`, `agent_access`
- `runtime`, `phase`, `phase_since`, `ready`, `data_state`, `fresh`, `simulated`, `waiting_for_frames`, `last_frame_age_ms`
- `error_code`, `error_detail`
- `observed_at`, `last_fresh_at`, `stop_reason`, `stopped_at`
- `udp_scope`, `udp_enabled`, `udp_ready`, `receiver_configured`
- `storage_used_bytes`, `storage_budget_bytes`, `storage_limit`
- `recording_session`, `recording_ack`, `recording_at`
- `hints`, `boards live_state=… known=… live=…`

It **never** includes the legacy `error` field, so no paths or URLs leak.

### 2.6 DOM hooks (tests rely on these)

**Root.** `<div class="ruview-app" data-ruview-app>` contains:
- `<header class="ruview-commandbar" data-ruview-commandbar>`
- `<div class="ruview-body">`
- a single `<div class="ruview-sr-only" role="status" aria-live="polite" data-ruview-live>`, created once and never hidden.

**Command bar**, left to right:
- `.ruview-brand` (glyph `◉` aria-hidden, `.ruview-brand-name` "RuView", `<h1 class="ruview-sr-only">RuView</h1>`)
- `<div role="tablist" data-ruview-tablist aria-label=t(nav_label)>` with `<button role="tab" data-view="status|boards|lab" id="ruview-<win>-tab-<view>" aria-controls="ruview-<win>-panel-<view>" aria-selected tabindex="0|-1">`. Unavailable tabs get `hidden`.
- `<button data-ruview-view-menu aria-haspopup="menu" aria-expanded>` (shown below 720 px) with a popup `[data-ruview-view-list][role=menu]` of `[role=menuitemradio][data-view-option=<view>]`
- `<button class="ruview-pill" data-ruview-pill data-tone aria-label=t(pill_open_status)>` containing `[data-ruview-pill-glyph]` (aria-hidden), `[data-ruview-pill-label]`, `[data-ruview-pill-age][aria-live=off]`
- `[data-ruview-primary-slot]`: on Boards it holds `<button data-ruview-primary data-action="flash">` (label `action_add_board`, icon `＋`, and `aria-label` when icon-only). It is empty on Status and Lab.
- `<button data-ruview-more aria-haspopup="menu" aria-expanded aria-label=t(more_actions)>⋯</button>` with `[data-ruview-menu][role=menu]` holding `[role=menuitem]` items:
  - `data-action="test"` (check_now)
  - `data-action="runtime/stop"` (menu_stop_service; present only when mode≠external && runtime ∈ {running, starting, stop_unconfirmed, exited})
  - `data-action="runtime/install"` (native only)
  - `<a role=menuitem data-action="open-settings" href="/config#ruview" target="_blank" rel="noopener">` (key `settings`)
  - `data-action="copy-diagnostics"`
  - `data-action="about"`

**Status panel** `<section role="tabpanel" data-panel="status">`:
- `[data-ruview-headline]` with `data-state=<id>` and `data-tone`, containing `[data-ruview-glyph]` (aria-hidden, `data-busy`) and `[data-ruview-headline-text]` (sentence; `{age}` rendered as `<span data-ruview-age aria-live="off">`)
- `[data-ruview-secondary]`
- `<button data-ruview-next data-action=… aria-disabled aria-describedby>` + `[data-ruview-next-reason]` (visible reason)
- `[data-ruview-secondary-action]`
- `[data-ruview-overlays]` with `[data-chip=read_only|storage_full|storage_unreadable]`
- error block with `<details data-ruview-technical><summary>technical_details</summary><pre data-ruview-technical-text>`
- action error block `[data-ruview-action-error]` (translated cause + `<details data-ruview-action-technical>`)
- stepper `<ol class="ruview-setup" data-ruview-setup>` with `<li data-step=<id> data-step-state=done|current|todo>`; the current step has `<button data-step-fix data-action=…>`
- `<details data-ruview-details>` with rows `[data-detail=service|source|version|storage|recording|hints]` and a `data-action="copy-diagnostics"` button; a fallback `<textarea data-ruview-diagnostics readonly hidden>`
- `[data-ruview-about]` (quality_short + `<details>` "about_more" containing `quality`).

**Boards panel** `data-panel="boards"`:
- heading with `[data-ruview-board-count]`
- `devices_known` note
- `<p class="ruview-feedback" data-ruview-boards-feedback>` (not a live region)
- `[data-ruview-add-reason]`
- `<div class="ruview-board-grid" data-node-list>` of `<article class="ruview-node" data-node-id=<id>>`, each with `<h3>` (board_title), `[data-fresh]`, `[data-rssi]`, `[data-age][data-age-ms]`, `<button data-node=<id>>` (configure_node)

**Lab panel** `data-panel="lab"`:
- `[data-ruview-lab-landing]` with cards `[data-lab-card=index.html|observatory.html|pose-fusion.html]`. Each card has a heading, description, `.ruview-chip[data-chip=experimental]`, `.ruview-chip[data-chip=english]`, a requirement line and `<button data-lab-open=<page>>`.
- `[data-ruview-lab-page][hidden]` with a bar (`[data-lab-close=back]`, `[data-lab-title]`, chip, `[data-lab-close=close]`), the pose-fusion notice (existing `fusion_notice`) and `[data-ruview-lab-frame]` (iframe parent).

`ruview.css` uses **no ID selectors**, which keeps the hex-colour test unambiguous.

### 2.7 i18n keys (en / de). All keys are prefixed `ruview.`

Rules:
- German headlines are taken verbatim from design §7.
- New German sentences use capitalised "Du/Dir/Deine", like the existing `ui/lang/ruview` strings.
- Placeholders are `{name}`.
- Reused existing keys: `settings`, `close`, `service`, `source`, `storage_card`, `recording`, `observatory`, `pose_fusion`, `quality`, `fusion_notice`, `revision_verified`, `revision_unverified`, `recording_unknown`, `recording_unconfirmed`, `udp_operator`, `devices_known`, `devices_stale`, `devices_empty`, `devices_not_applicable`, `devices_not_running`, `device_offline`, `received`, `stale`, `rssi_retained`, `configure_node`, `signal_csi`, `signal_rssi`, `last_measurement`, `age_ms`, `age_s`, `age_min`, `unchecked`, `demo`, `disconnected`, `starting`, `stopping`, `stopped_state`, `check_ok`, `check_attention`, `check_unchecked`, `working`, `enable_first`, `external_runtime`, `install_native`, `docker`, `native`, `external`, `esp32`, `wifi`, `simulated`, `nav_label`, `disabled`.
- Remove `ruview.live_unverified`, `ruview.synthetic` and `ruview.stopped` (Grep confirmed they are unused in `ui/js`, `ui/cfg` and `ui/js/ruview`).

| key | en | de |
|---|---|---|
| view_status | Status | Status |
| view_boards | Boards | Boards |
| view_lab | Lab | Labor |
| view_menu_label | View: {view} | Ansicht: {view} |
| more_actions | More actions | Weitere Aktionen |
| check_now | Check now | Jetzt prüfen |
| menu_stop_service | Stop service | Dienst stoppen |
| menu_install | Install service | Dienst installieren |
| menu_copy_diagnostics | Copy diagnostics | Diagnose kopieren |
| menu_about | About Wi-Fi sensing | Über WLAN-Sensing |
| pill_open_status | Show status | Status anzeigen |
| pill_checking | Checking | Wird geprüft |
| pill_unavailable | Unavailable | Nicht abrufbar |
| pill_off | Off | Aus |
| pill_read_only | Read-only | Schreibgeschützt |
| pill_not_installed | Not installed | Nicht installiert |
| pill_installing | Installing | Wird installiert |
| pill_problem | Problem | Problem |
| pill_waiting | Waiting | Wartet |
| pill_live | Live | Live |
| pill_no_new_data | No new data | Keine neuen Daten |
| age_h | {n} h | {n} h |
| headline_loading | Checking RuView… | RuView wird geprüft … |
| headline_status_failed | Can't load the RuView status. | RuView-Status nicht abrufbar. |
| headline_off | RuView is turned off. | RuView ist ausgeschaltet. |
| headline_installing | Installing RuView: {phase}… | RuView wird installiert: {phase} … |
| headline_readonly_blocked | RuView is read-only, so the service can't start. | RuView ist schreibgeschützt, der Dienst kann nicht starten. |
| headline_not_installed | RuView isn't installed on this computer yet. | RuView ist auf diesem Rechner noch nicht installiert. |
| headline_stopped | The RuView service isn't running. | Der RuView-Dienst läuft nicht. |
| headline_starting_pull | Starting RuView: downloading the service image… | RuView startet: Dienst-Image wird geladen … |
| headline_starting | Starting RuView: {phase}… | RuView startet: {phase} … |
| headline_starting_plain | Starting RuView… | RuView startet … |
| headline_stopping | Stopping RuView… | RuView wird gestoppt … |
| headline_stop_failed | Stopping wasn't confirmed. RuView may still be running. | Stoppen nicht bestätigt. RuView läuft eventuell noch. |
| headline_external_unchecked | External RuView service, not checked yet. | Externer RuView-Dienst, noch nicht geprüft. |
| headline_external_check_old | External RuView service, last checked {age} ago. | Externer RuView-Dienst, zuletzt vor {age} geprüft. |
| headline_demo | Demo mode: simulated data, not measurements. | Demo-Modus: simulierte Daten, keine Messungen. |
| headline_waiting | RuView is running and waiting for data from the boards. | RuView läuft und wartet auf Daten der Boards. |
| headline_waiting_wifi | RuView is running and waiting for the first Wi-Fi data. | RuView läuft und wartet auf erste WLAN-Daten. |
| headline_disconnected | RuView is running, but its signal source is disconnected. | RuView läuft, aber die Signalquelle ist getrennt. |
| headline_live_board | Working: live data from {n} board, last {age} ago. | Läuft: Live-Daten von {n} Board, zuletzt vor {age}. |
| headline_live_boards | Working: live data from {n} boards, last {age} ago. | Läuft: Live-Daten von {n} Boards, zuletzt vor {age}. |
| headline_live | Working: live data, last {age} ago. | Läuft: Live-Daten, zuletzt vor {age}. |
| headline_live_wifi | Working: this computer's Wi-Fi signal, last {age} ago. | Läuft: WLAN-Signal dieses Rechners, zuletzt vor {age}. |
| headline_stale | No new data for {age}. | Seit {age} keine neuen Daten. |
| secondary_status_last_known | Showing the last known state from {time}. | Letzter bekannter Stand von {time} wird angezeigt. |
| secondary_off | RuView detects motion from Wi-Fi signals. Turn it on in the settings. | RuView erkennt Bewegung anhand von WLAN-Signalen. Schalte es in den Einstellungen ein. |
| secondary_readonly_blocks | Read-only blocks starting, board setup, recording, training and deleting. Stopping stays possible. | Der Schreibschutz sperrt Starten, Board-Einrichtung, Aufnahme, Training und Löschen. Stoppen bleibt möglich. |
| secondary_install | Install downloads and checks the pinned package. Nothing starts automatically. | Installieren lädt das festgelegte Paket und prüft es. Gestartet wird nichts automatisch. |
| secondary_not_started | Not started since AuraGo started. | Seit dem Start von AuraGo nicht gestartet. |
| secondary_phase_step | Step {n} of {total} · can take minutes the first time | Schritt {n} von {total} · kann beim ersten Mal Minuten dauern |
| secondary_phase_since | For {duration} | Seit {duration} |
| secondary_stop_hint | AuraGo recorded a hint in the planner. | AuraGo hat einen Hinweis im Planer angelegt. |
| secondary_demo | Choose ESP32 or Wi-Fi in settings for real sensing. | Wähle für echte Messungen ESP32 oder WLAN in den Einstellungen. |
| secondary_last_checked | Last checked {time} | Zuletzt geprüft {time} |
| secondary_last_live | Last live at {time}. | Zuletzt live um {time}. |
| secondary_live_meta | Last checked {time} · {source} · {mode} | Zuletzt geprüft {time} · {source} · {mode} |
| stop_reason_user | Stopped by you at {time}. | Von Dir um {time} gestoppt. |
| stop_reason_config_changed | Stopped after a settings change. | Nach einer Änderung der Einstellungen gestoppt. |
| stop_reason_process_exit | The service ended on its own. | Der Dienst wurde von selbst beendet. |
| stop_reason_aurago_shutdown | Stopped when AuraGo shut down. | Beim Beenden von AuraGo gestoppt. |
| state_external | External | Extern |
| state_not_started | Not started | Nicht gestartet |
| state_starting | Starting | Startet |
| state_running | Running | Läuft |
| state_stopping | Stopping | Stoppt |
| state_stopped | Stopped | Gestoppt |
| state_exited | Ended unexpectedly | Unerwartet beendet |
| state_stop_unconfirmed | Stop not confirmed | Stopp nicht bestätigt |
| phase_preparing | preparing | Vorbereitung |
| phase_pulling_image | downloading the service image | Dienst-Image wird geladen |
| phase_creating | creating the service | Dienst wird angelegt |
| phase_waiting_ready | waiting for the service to answer | warte auf Antwort des Dienstes |
| phase_downloading | downloading the package | Paket wird geladen |
| phase_verifying | checking the package | Paket wird geprüft |
| action_check_connection | Check connection | Verbindung prüfen |
| action_check_again | Check again | Erneut prüfen |
| action_try_again | Try again | Erneut versuchen |
| action_open_settings | Open settings | Einstellungen öffnen |
| action_allow_changes | Allow changes… | Änderungen erlauben … |
| action_install | Install | Installieren |
| action_start | Start service | Dienst starten |
| action_retry_start | Retry start | Start wiederholen |
| action_start_again | Start again | Erneut starten |
| action_stop_again | Try stopping again | Erneut stoppen |
| action_starting | Starting… | Startet … |
| action_show_boards | Show boards | Boards anzeigen |
| action_add_board | Add board | Board hinzufügen |
| reason_wait_start | Wait for the start to finish. | Warte, bis der Start abgeschlossen ist. |
| reason_readonly | Allow changes first (read-only is on). | Erlaube zuerst Änderungen (Schreibschutz ist an). |
| reason_desktop_readonly | The desktop is read-only. | Der Desktop ist schreibgeschützt. |
| err_disabled | RuView is turned off. | RuView ist ausgeschaltet. |
| err_readonly | RuView is read-only, so this change is blocked. | RuView ist schreibgeschützt, diese Änderung ist gesperrt. |
| err_external_runtime | An external RuView service is started and stopped on its own computer. | Ein externer RuView-Dienst wird an seinem eigenen Rechner gestartet und gestoppt. |
| err_docker_readonly | RuView can't start: the Docker integration is off or read-only. | RuView startet nicht: Die Docker-Integration ist aus oder schreibgeschützt. |
| err_already_running | RuView is already running. Stop it before starting again. | RuView läuft bereits. Stoppe den Dienst, bevor Du ihn neu startest. |
| err_not_installed | RuView isn't installed on this computer yet. | RuView ist auf diesem Rechner noch nicht installiert. |
| err_busy | RuView is busy with another start or installation. Wait for it to finish. | RuView ist mit einem anderen Start oder einer Installation beschäftigt. Warte, bis sie fertig ist. |
| err_cancelled | The action was cancelled by a stop or a settings change. | Die Aktion wurde durch einen Stopp oder eine Einstellungsänderung abgebrochen. |
| err_container_conflict | A container named aurago-ruview belongs to another AuraGo installation. Remove or rename it in Docker, then start again. | Ein Container namens aurago-ruview gehört zu einer anderen AuraGo-Installation. Entferne oder benenne ihn in Docker um und starte dann erneut. |
| err_platform_unsupported | There is no verified RuView package for this computer's system. | Für das System dieses Rechners gibt es kein geprüftes RuView-Paket. |
| err_invalid_node_id | The board number must be between 1 and 255. | Die Board-Nummer muss zwischen 1 und 255 liegen. |
| err_invalid_config | The RuView settings don't allow this. Check them in the settings. | Die RuView-Einstellungen erlauben das nicht. Prüfe sie in den Einstellungen. |
| err_unreachable | RuView can't be reached. | RuView ist nicht erreichbar. |
| err_upstream_auth | RuView rejected AuraGo's access token. | RuView hat das Zugriffstoken von AuraGo abgelehnt. |
| err_upstream_error | RuView answered with an error. | RuView hat mit einem Fehler geantwortet. |
| err_docker_unavailable | RuView can't start: Docker can't be reached. | RuView startet nicht: Docker ist nicht erreichbar. |
| err_image_denied | RuView can't start: the registry denied the service image. | RuView startet nicht: Die Registry verweigert das Dienst-Image. |
| err_image_pull_failed | RuView can't start: the service image could not be downloaded. | RuView startet nicht: Das Dienst-Image konnte nicht geladen werden. |
| err_contract_unsupported | RuView answered in a format this AuraGo version doesn't support. | RuView antwortet in einem Format, das diese AuraGo-Version nicht unterstützt. |
| err_download_failed | The RuView package could not be downloaded or failed its check. | Das RuView-Paket konnte nicht geladen werden oder hat die Prüfung nicht bestanden. |
| err_start_failed | RuView can't start. | RuView startet nicht. |
| err_stop_failed | Stopping wasn't confirmed. RuView may still be running. | Stoppen nicht bestätigt. RuView läuft eventuell noch. |
| err_process_exited | RuView stopped unexpectedly. | RuView wurde unerwartet beendet. |
| err_source_mismatch | RuView uses a different signal source than set in AuraGo. | RuView nutzt eine andere Signalquelle als in AuraGo eingestellt. |
| err_unexpected_simulation | RuView is sending simulated data although real sensing is set. | RuView sendet simulierte Daten, obwohl echte Messung eingestellt ist. |
| err_timeout | RuView didn't answer in time. | RuView hat nicht rechtzeitig geantwortet. |
| err_storage_unreadable | RuView's storage folder can't be checked, so the storage budget isn't enforced. | Der RuView-Speicherordner kann nicht geprüft werden, das Speicherbudget greift daher nicht. |
| err_storage_full | The RuView storage budget is full, so recording is stopped. | Das RuView-Speicherbudget ist voll, Aufnahmen sind gestoppt. |
| err_artifact_missing | This AuraGo release has no verified RuView package for this. | Diese AuraGo-Version enthält dafür kein geprüftes RuView-Paket. |
| err_unknown | Something went wrong in RuView. | In RuView ist etwas schiefgelaufen. |
| err_network | AuraGo can't be reached. Check the connection. | AuraGo ist nicht erreichbar. Prüfe die Verbindung. |
| err_forbidden | You don't have permission for this. | Dafür fehlt Dir die Berechtigung. |
| technical_details | Technical details | Technische Details |
| announce_checked | Check finished. | Prüfung abgeschlossen. |
| setup_heading | Setup | Einrichtung |
| step_enabled | Turned on | Eingeschaltet |
| step_writable | Changes allowed | Änderungen erlaubt |
| step_service | Service running | Dienst läuft |
| step_receiver | Receiver and UDP set | Empfänger und UDP eingerichtet |
| step_checked | Connection checked | Verbindung geprüft |
| step_board | First board | Erstes Board |
| step_data | Data arriving | Daten kommen an |
| details_summary | Details | Details |
| details_version | Version | Version |
| details_hints | Hints | Hinweise |
| hints_none | None open | Keine offen |
| hint_sensing_stale | Sensing became stale | Messung ist veraltet |
| hint_storage_limit | Storage budget full | Speicherbudget voll |
| hint_stop_unconfirmed | Stop not confirmed | Stopp nicht bestätigt |
| storage_of | {used} of {budget} | {used} von {budget} |
| storage_unknown | Not measured yet | Noch nicht gemessen |
| recording_last | Last confirmed action: {action} at {time} (this AuraGo session) | Letzte bestätigte Aktion: {action} um {time} (diese AuraGo-Sitzung) |
| recording_action_started | started | gestartet |
| recording_action_stopped | stopped | gestoppt |
| udp_on | UDP on | UDP an |
| udp_off | UDP off | UDP aus |
| receiver_set | receiver set | Empfänger eingerichtet |
| receiver_missing | no receiver set | kein Empfänger eingerichtet |
| diagnostics_copied | Diagnostics copied. | Diagnose kopiert. |
| diagnostics_failed | Copying failed. Select the text below and copy it. | Kopieren fehlgeschlagen. Markiere den Text unten und kopiere ihn. |
| quality_short | Wi-Fi sensing is experimental and does not prove presence. | WLAN-Sensing ist experimentell und beweist keine Anwesenheit. |
| about_more | More… | Mehr … |
| chip_storage_full | Storage full · recording stopped | Speicher voll · Aufnahme gestoppt |
| chip_storage_unreadable | Storage can't be checked | Speicher nicht prüfbar |
| allow_changes_title | Allow changes to RuView? | Änderungen an RuView erlauben? |
| allow_changes_body | Read-only currently blocks starting the service, setting up boards, recording, training and deleting. This saves “read-only: off” in the RuView settings. Stopping works either way. You can turn read-only back on in the settings. | Der Schreibschutz sperrt derzeit das Starten des Dienstes, die Board-Einrichtung, Aufnahmen, Training und Löschen. Damit wird „Schreibschutz: aus“ in den RuView-Einstellungen gespeichert. Stoppen geht in beiden Fällen. Du kannst den Schreibschutz in den Einstellungen wieder einschalten. |
| allow_changes_confirm | Allow changes | Änderungen erlauben |
| allow_changes_done | Changes are allowed now. | Änderungen sind jetzt erlaubt. |
| allow_changes_failed | The setting could not be saved. | Die Einstellung konnte nicht gespeichert werden. |
| board_title | Board #{id} | Board #{id} |
| lab_intro | Original RuView pages for experiments. They are in English, may show unverified claims and open only when you choose them. | Original-RuView-Seiten zum Ausprobieren. Sie sind englisch, können ungeprüfte Behauptungen zeigen und öffnen sich nur, wenn Du sie auswählst. |
| lab_observatory_desc | Upstream 3D view of the sensing field. | 3D-Ansicht des Messfelds aus dem RuView-Projekt. |
| lab_pose_fusion_desc | Experimental pose estimate from CSI with untrained example models. | Experimentelle Posenschätzung aus CSI mit untrainierten Beispielmodellen. |
| lab_dashboard | Original RuView dashboard | Original-RuView-Dashboard |
| lab_dashboard_desc | The upstream dashboard, including recording, training and the model library. | Das Dashboard des RuView-Projekts, mit Aufnahme, Training und Modellbibliothek. |
| lab_req_csi | Needs live CSI; demo only with the demo source. | Braucht Live-CSI; Demo nur mit der Demo-Quelle. |
| lab_req_any | Works with any source and shows waiting states without data. | Funktioniert mit jeder Quelle und zeigt ohne Daten Wartezustände. |
| chip_experimental | Experimental | Experimentell |
| chip_english | English only | Nur Englisch |
| lab_open | Open | Öffnen |
| lab_back | ← Lab | ← Labor |
| lab_frame_title_observatory | Observatory (experimental, English) | Observatorium (experimentell, englisch) |
| lab_frame_title_pose_fusion | Pose fusion (experimental, English) | Pose-Fusion (experimentell, englisch) |
| lab_frame_title_dashboard | Original RuView dashboard (experimental, English) | Original-RuView-Dashboard (experimentell, englisch) |

---

## 3. Batch A — Go backend (maker-agent; wave 1, parallel with B)

**Files:**
- `internal/ruview/{errors.go,lifecycle.go,storage.go}` (new)
- `internal/ruview/{manager.go,runtime.go,artifacts.go,client.go,read.go,nodes.go,proxy.go,hints.go}`
- `internal/server/ruview_handlers.go`
- tests named per task

Before editing, run GitNexus impact for: `Configure`, `Status`, `test`, `watch`, `reapNativeLocked`, `sameRuntime`, `storageLimited`, `directoryBytes`, `noteRecording`, `start`, `startDocker`, `startNative`, `ownedContainer`, `Stop`, `stopRuntime`, `Install`, `request`, `readStatusError`, `Nodes`, `RememberNode`, `Proxy`, `registerRuViewRoutes`.

### Task A1 — M27 first: Configure cancels only on runtime-relevant changes

- [ ] Add `func runtimeFields(c config.RuViewConfig) config.RuViewConfig` (zeroes `ReadOnly`, `AgentAccess`, `StorageMB`) in `manager.go`. Rewrite `sameRuntime` (`manager.go:462-468`) to use it; the Docker/DataDir comparisons stay.
- [ ] Add Manager fields (`manager.go:46-69`): `op, phase string; phaseSince, stoppedAt, lastFreshAt, recordingAt, storageAt, dockerAt time.Time; stopReason string; storageEvery, dockerEvery time.Duration; storageUsed int64; storageKnown, storageErr bool; walkStorage func(string) (int64, error)`. Initialize them in `New` (`:76-79`): `storageEvery: 30*time.Second, dockerEvery: 5*time.Second, walkStorage: directoryBytes`.
- [ ] Rewrite `Configure` (`:85-100`):
```go
func (m *Manager) Configure() {
	c := m.config().RuView
	m.mu.Lock()
	defer m.mu.Unlock()
	if reflect.DeepEqual(m.previous, c) { return }
	relevant := !reflect.DeepEqual(runtimeFields(m.previous), runtimeFields(c))
	m.previous = c
	if !relevant { return } // M27: agent access, read-only and storage budget keep streams, probes and a running pull.
	if m.op == opStarting || m.managed { m.stopReason, m.stoppedAt = StopConfigChanged, time.Now().UTC() }
	m.cancel()
	m.generation, m.cancel = context.WithCancel(context.Background())
	previous := m.status
	m.status = Status{}
	if lifecycleError(previous.Error) { // P0 rule: lifecycle errors survive Configure
		m.status.Error, m.status.ErrorCode, m.status.ErrorDetail = previous.Error, previous.ErrorCode, previous.ErrorDetail
	}
	m.observed, m.lastFreshAt = time.Time{}, time.Time{}
	if m.op == "" { m.phase, m.phaseSince = "", time.Time{} }
}
```
- [ ] In `watch` (rewritten in A5), the "config no longer matches" path calls `m.stopRuntime(ctx, StopConfigChanged)`.
- [ ] Update the comment at `manager.go:81-84` to describe this rule.

### Task A2 — typed errors (`errors.go`) and §8.4 handler helper

- [ ] Create `errors.go` per §2.2: constants, `codeText` map (short English sentences), `Error`, `HTTPStatus`, `ErrorCodes`, `AsError`, `fail`, `ctxError`, `scrubDetail`, `maxDetailBytes`.
- [ ] `read.go:108-120`: `readStatusError` returns `scrubDetail(err.Error(), 160)` or `"RuView status could not be read"` when empty. Keep `statusErrorURL` only if still used, otherwise delete it.
- [ ] `client.go:95-129` `request`:
  - `m.client` error → `fail(CodeInvalidConfig, err.Error(), err)`
  - `client.Do` error → `ctxError` if `ctx.Err() != nil` or `errors.Is` deadline/canceled; `var ne net.Error; errors.As(err,&ne) && ne.Timeout()` → `CodeTimeout`; else `fail(CodeUnreachable, "RuView service could not be reached", nil)`
  - status 401/403 → `fail(CodeUpstreamAuth, fmt.Sprintf("RuView returned HTTP %d", code), nil)`; other non-2xx → `CodeUpstreamError` with the same message
  - read error and invalid JSON/size → `&Error{Code: CodeContractUnsupported, HTTP: 502, msg: errInvalidReply.Error(), cause: errInvalidReply}`, so `errors.Is(err, errInvalidReply)` at `nodes.go:76` still matches
- [ ] `manager.go` `test()` (`:222-301`):
  - disabled → `fail(CodeDisabled, "RuView is disabled", nil)`
  - `raw.Status != "ready"` → `fail(CodeContractUnsupported, "RuView status contract is unsupported", nil)`
  - mismatch → `s.Error, s.ErrorCode = CodeSourceMismatch, CodeSourceMismatch`; same for `CodeUnexpectedSimulation`
  - `if err != nil { e := AsError(err); s.Error, s.ErrorCode, s.ErrorDetail = e.Error(), e.Code, e.Detail }`
  - the lifecycle-preservation block (`:286-288`) also copies `ErrorCode`/`ErrorDetail`
  - when stored (`current`): `if s.Fresh && !s.Simulated { m.lastFreshAt = now }`; `if s.Ready && m.phase == PhaseWaitingReady { m.phase, m.phaseSince = "", time.Time{} }`
- [ ] `nodes.go:47-49` → `fail(CodeDisabled, "RuView is disabled", nil)`. `RememberNode` `:118-123` → `CodeDisabled`/`CodeReadOnly` (separately), `CodeInvalidNodeID`.
- [ ] `lifecycleError` (`:305-307`) uses `CodeStopFailed`/`CodeProcessExited`.
- [ ] `internal/server/ruview_handlers.go`: add
```go
// ruviewError writes {error_code, error, detail} with the code's HTTP status (§8.4).
func ruviewError(w http.ResponseWriter, err error) {
	e := ruview.AsError(err)
	status := e.HTTP
	if status == 0 { status = ruview.HTTPStatus(e.Code) }
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error_code": e.Code, "error": e.Error(), "detail": e.Detail})
}
```
  Then replace these call sites:
  - `:108-110` → `ruviewError(w, &ruview.Error{Code: ruview.CodeDisabled})`
  - `:113-114` → `ruviewError(w, err)`
  - `:141-145` → split into `CodeDisabled` (`!cfg.Enabled`) and `CodeReadOnly`
  - `:157` → `CodeInvalidNodeID`
  - `:173-176` → `CodeExternalRuntime`
  - `:182-184` → `ruviewError(w, err)`

  Origin and method errors keep `jsonError`. The proxy routes are unchanged.
- [ ] **Test:** new `internal/server/ruview_errors_test.go`.
  - `TestRuViewErrorCodesMapToHTTP` holds a literal `want` map (every code in §2.2 → status). Assert `len(want)==len(ruview.ErrorCodes())`. For each code, check `HTTPStatus` and the body of `ruviewError` (`error_code`, non-empty `error`, `detail` round-trip). Also assert: `errors.New("boom at /etc/secret")` → 500, `unknown`, detail without `/etc`; `context.Canceled` → 409 `cancelled`.
  - `TestRuViewRouteRefusalsCarryCodes` uses `testDesktopPermissionServer` + `registerRuViewRoutes`, as in `ruview_handlers_test.go:15-29`, with an admin token and Origin `http://aurago.test`:
    - readonly docker: POST `runtime/start` → 403 `readonly`
    - disabled: POST `runtime/start` → 403 `disabled`; GET `nodes` → 403 `disabled`
    - external: POST `runtime/stop` → 403 `external_runtime`
    - writable: POST `nodes` `{"node_id":0}` → 400 `invalid_node_id`

### Task A3 — M29: lifecycle slot, phases, Stop during start

- [ ] `lifecycle.go` per §2.3: constants, enumerators, `runtimeLocked`, `setPhaseLocked(p)` (sets `phase` and `phaseSince=now`), `setPhase(p)` (locks `mu`).
- [ ] `runtime.go:43-128` `start(ctx, expected)` new flow:
  1. `m.opMu.Lock()`. Under `mu`: `reapNativeLocked()`, capture `generation`, `busy := m.op != ""`, `active := m.managed`.
  2. On `ctx.Err() || generation.Err() || expected mismatch` → unlock and return `ctxError(context.Canceled)`.
  3. Gates, in this order:
     - `!Enabled` → `CodeDisabled` ("RuView start requires enabled, writable integration")
     - `ReadOnly` → `CodeReadOnly`
     - external → `CodeExternalRuntime` ("external RuView is managed by its operator")
     - `busy` → `CodeBusy`
     - `active` → `expected != nil ? nil : fail(CodeAlreadyRunning, "RuView is already started; stop it before reconfiguration", nil)`

     Each gate unlocks `opMu` before returning.
  4. Claim under `mu`: `m.op=opStarting; m.setPhaseLocked(PhasePreparing); m.status=Status{DataState:"starting"}; m.observed=time.Time{}; m.stopReason, m.stoppedAt = "", time.Time{}`.
  5. Bind the work context **to the captured generation**: `ctx, cancel := context.WithCancel(ctx); stopBind := context.AfterFunc(generation, cancel)`, then defer both. Only then `m.opMu.Unlock()`.
  6. Deferred epilogue, under `mu`: `m.op = ""`. On error: clear `phase`/`phaseSince`; if `generation == m.generation && generation.Err()==nil`, then `e := AsError(startErr); m.status = Status{DataState:"disconnected", Error: security.Scrub(startErr.Error()), ErrorCode: e.Code, ErrorDetail: e.Detail}`.
  7. `configCheck` → `fail(CodeInvalidConfig, err.Error(), err)`. Data dir/Lstat/`managedToken` errors → `CodeStartFailed`.
  8. Docker:
     - `if err := m.ensureImage(ctx, c); err != nil { return err }` (no lock held)
     - `m.opMu.Lock(); defer m.opMu.Unlock()`
     - if `generation.Err() != nil || ctx.Err() != nil` → `ctxError(...)`
     - `m.setPhase(PhaseCreating)`; `if err := m.createContainer(ctx, c, dir, token); err != nil { return err }`
  9. Native:
     - `m.setPhase(PhaseVerifying); root, a, err := m.verifyNative(c)` (no lock)
     - lock `opMu` (deferred unlock), check the generation, `setPhase(PhaseCreating)`, `m.launchNative(c, root, dir, token)`
  10. Commit, under `mu`: `managed=true; runningConfig=c; current := ...`. If current: `phase, phaseSince = PhaseWaitingReady, now`.

      If `!current`, run the existing cleanup `stopRuntime(cleanup, "")` (opMu is held) and return `ctxError(context.Canceled)`. Then `noteStop(true, mode)`.
- [ ] Split `startDocker` (`:235-309`):
  - **`ensureImage(ctx, c) error`** handles: Docker disabled/readonly → `CodeDockerReadOnly`; no digest → `CodeArtifactMissing`; image inspect → 200 returns nil; code 0 → `dockerFail(ctx, err, 0)`. Otherwise it calls `m.setPhase(PhasePullingImage)` and runs the existing pull code.
    - Pull HTTP≠200 → `pullFail(status, detail)`. It keeps the message `"RuView image pull returned HTTP %d: %s"` (512-byte `SanitizeOneLine`, scrubbed). The cause is `fmt.Errorf("HTTP %d: %s", status, detail)`, and the code is `CodeImageDenied` when status ∈ {401,403,404} or the lower-cased detail contains `denied`, `unauthorized`, `forbidden` or `manifest unknown`; otherwise `CodeImagePullFailed`.
    - `DrainJSONMessages` errors are classified the same way, with message `"pull RuView image: …"`.
    - Any error with `ctx.Err()!=nil` → `ctxError`.
  - **`createContainer(ctx, c, dir, token) error`** covers ownership, old-container stop/remove, placement, create and start. Errors: `ownedContainer` conflict passes through; Engine transport errors (code 0) → `CodeDockerUnavailable`; other errors → `CodeStartFailed`; ctx errors → `ctxError`.
  - Helper `dockerFail(ctx, err error, code int) *Error`.
- [ ] Split `startNative` (`:172-207`) into:
  - `verifyNative(c) (root string, a NativeArtifact, err error)`: `nativeArtifact` error → `CodePlatformUnsupported`; `verifyFile` failure → `fail(CodeNotInstalled, "RuView native package is not installed or failed verification", err)`
  - `launchNative(c, root, dir, token) error`: launch error → `CodeStartFailed`; sets `m.process`
- [ ] `Stop` (`:352-359`) → `func (m *Manager) Stop(ctx) error { return m.stop(ctx, StopUser) }` with `stop(ctx, reason)` = cancel the generation, `opMu.Lock`, `stopRuntime(ctx, reason)`. `Close` (`manager.go:499-512`) calls `m.stop(ctx, StopAuraGoShutdown)`.
- [ ] `stopRuntime(ctx, reason string)` (`:362-429`):
  - Success epilogue: `m.status=Status{DataState:"stopped"}`; clear `phase`; `if reason != "" { m.stopReason, m.stoppedAt = reason, now }`.
  - Failure: `Status{DataState:"disconnected", Error: CodeStopFailed, ErrorCode: CodeStopFailed, ErrorDetail: scrubDetail(stopErr.Error(), maxDetailBytes)}`.
  - Return `fail(CodeStopFailed, stopErr.Error(), stopErr)` on failure.
  - Callers: `Stop`→user, `watch`→config_changed, start cleanup→"" (keeps the reason set by Configure/Stop).
- [ ] `artifacts.go:68-97` `Install`: `opMu.Lock`, then gates in order: disabled, readonly, busy, mode≠native → `CodeInvalidConfig` ("native installation is not selected"). Then claim `op=opInstalling`, `setPhaseLocked(PhaseDownloading)`, bind ctx to the captured generation, and `opMu.Unlock()`. Defer under `mu`: `op=""`, clear phase.
  - `configCheck` → invalid_config; `nativeArtifact` → platform_unsupported; mkdir → `CodeDownloadFailed`
  - each `download` error → `ctxError` if ctx is done, else `fail(CodeDownloadFailed, "RuView download failed", err)`
- [ ] **Test (background_test.go):** extend `backgroundFixture` (`:24-28`) with `imageMissing, exited, foreign atomic.Bool; pulls, deletes, stops atomic.Int32; pullEntered, pullRelease chan struct{}`.

  In the handler (`:37-90`), add these cases **before** the `strings.HasPrefix(path, "/images/")` case:
  - `/images/create`: count; close `pullEntered` once; wait for `pullRelease` or `r.Context().Done()`; write `{"status":"Downloaded"}\n`; clear `imageMissing`
  - `/images/…`: 404 when `imageMissing`
  - `/containers/aurago-ruview/json`: add `"State":{"Running":!exited,"Status":"running"|"exited","ExitCode":0|137}`; the owner label becomes `"someone-else"` when `foreign`
  - `/containers/boot-fixture/stop` increments `stops`; DELETE increments `deletes`

  Add **`TestRuViewStartPhasesBusyAndStopDuringPull`** (`imageMissing`, `pullRelease` never closed):
  - after `<-pullEntered`, Status has `Runtime==starting`, `Phase==pulling_image` and `PhaseSince!=nil`
  - `m.watch(ctx)` returns nil within 1 s
  - a second `m.Start` returns `AsError(err).Code==CodeBusy` within 1 s (HTTP 409)
  - `m.Install` → busy
  - `m.Stop` returns nil within 2 s
  - the pending Start returns `CodeCancelled`
  - final Status: `Runtime==stopped`, `StopReason==user`, `Phase==""`, `creates==0`

  A second subtest releases the pull: Start returns nil with `Runtime==starting` and `Phase==waiting_ready`; after `m.Test(ctx)` (ready fixture), `Phase==""` and `Runtime==running`.

  Use a small helper `within(t, d, fn)`.

### Task A4 — M23: remove the owned container after stop; conflict code

- [ ] `runtime.go:215-233`: add
```go
type containerState struct{ ID string; Running bool; Status string; ExitCode int; Known bool } // Known=false when the Engine omitted State
func inspectOwned(ctx context.Context, d *dockerutil.Client, c *config.Config) (st containerState, found bool, err error)
```
  It decodes `State *struct{Running bool; Status string; ExitCode int}` (a pointer; nil means unknown). Missing or wrong labels → `fail(CodeContainerConflict, "a container named aurago-ruview belongs to another workload", nil)`; the legacy text keeps "belongs to another workload". `ownedContainer` becomes a wrapper returning `(id, err)`.
- [ ] `stopRuntime` Docker branch (`:415-427`):
  - An ownership conflict → `m.logger.Warn("RuView container name belongs to another workload; nothing of ours to stop")` and return nil. It is not our container, and Start reports `container_conflict`.
  - After a confirmed stop (or 304/404): `DELETE containers/<id>`. A failure other than 404 is logged with Warn and stays non-fatal, because the stop is confirmed and the next start removes the container anyway.
- [ ] **Test:** in `runtime_test.go:22-100` (`TestRuViewDockerOwnershipAndBoundedResources`), replace `m.startDocker(...)` (`:74`) with `m.createContainer(...)` and add a DELETE case (`removed=true; owned=false`). After the first Stop assert `stopped && removed`. Before the second Stop set `owned=true; stopped, removed = false, false`, which models a container left by an earlier process, and assert stop+delete again.
- [ ] **Test:** `runtime_test.go:102-135`: call `m.ensureImage(context.Background(), c)` instead of `startDocker` and add `AsError(err).Code == CodeImageDenied`. Keep the message and length assertions.
- [ ] **Test (background_test.go):** `TestRuViewDockerConflictIsReportedAndStopIgnoresForeign` with `f.foreign`:
  - `m.Start` → `container_conflict`, `HTTPStatus 409`; `m.Status().ErrorCode == container_conflict`; `ErrorDetail` without `\`/`/` path tokens
  - `m.Stop` → nil, `stops==0 && deletes==0`

### Task A5 — M24 + M28: watch rewrite, exit detection, throttled storage

- [ ] New `storage.go`. Move `directoryBytes` from `manager.go:475-497` and treat a missing **root** as `(0, nil)`. Symlinks and special files stay errors. Add:
```go
// sampleStorage walks the managed data directory at most every m.storageEvery (M28).
func (m *Manager) sampleStorage(c *config.Config) (used int64, sampled bool, err error)
// storageCheck always walks (proxy recording start) and refreshes the cache.
func (m *Manager) storageCheck(c *config.Config) (limited bool, err error)
```
  Both update `storageAt/storageUsed/storageKnown/storageErr` under `mu`. Delete `storageLimited` (`manager.go:470-473`).
- [ ] `proxy.go:86-91`: `limited, err := m.storageCheck(c)`. `err` → `http.Error(w, "RuView storage could not be checked", http.StatusServiceUnavailable)`; `limited` → 507, as before.
- [ ] `lifecycle.go`:
  - `markExitedLocked(detail string)`: `process=nil; managed=false; runningConfig=nil; cancel` + new generation; `status=Status{DataState:"disconnected", Error: CodeProcessExited, ErrorCode: CodeProcessExited, ErrorDetail: detail}`; `observed` zero; `phase` cleared; `stopReason, stoppedAt = StopProcessExit, now`
  - `inspectExit(ctx, running *config.Config) (exited bool, detail string)`: only Docker; throttled by `dockerEvery` via `dockerAt` (under `mu`); 5 s timeout. Not found → `true, "container was removed"`; conflict → `true, "container was replaced by another workload"`; `Known && !Running` → `true, fmt.Sprintf("container exited with code %d", ExitCode)`; transport error or unknown state → false.
- [ ] `reapNativeLocked` (`manager.go:445-460`) → `m.markExitedLocked(exitDetail(p.err))`, where `exitDetail` is `scrubDetail(err.Error(),240)` or `"process ended"`.
- [ ] Rewrite `watch` (`manager.go:411-441`):
```go
func (m *Manager) watch(ctx context.Context) context.Context {
	m.Configure()
	m.opMu.Lock()
	m.mu.Lock(); m.reapNativeLocked(); managed, running, stopFailed := m.managed, m.runningConfig, m.status.Error == CodeStopFailed; m.mu.Unlock()
	if !managed || stopFailed { m.opMu.Unlock(); return nil }
	c := m.config()
	if !c.RuView.Enabled || running == nil || !sameRuntime(running, c) {
		_ = m.stopRuntime(ctx, StopConfigChanged) // M27 stop reason
		m.opMu.Unlock(); return nil
	}
	m.opMu.Unlock()
	if exited, detail := m.inspectExit(ctx, running); exited { // M24, outside opMu
		m.opMu.Lock(); m.mu.Lock()
		if m.managed && m.runningConfig == running { m.markExitedLocked(detail) }
		m.mu.Unlock(); m.opMu.Unlock(); return nil
	}
	m.mu.Lock()
	if m.phase == PhaseWaitingReady && time.Since(m.phaseSince) > waitingReadyTimeout { m.phase, m.phaseSince = "", time.Time{} }
	m.mu.Unlock()
	if used, sampled, err := m.sampleStorage(c); sampled { // M28: at most every 30 s, outside opMu
		limited := err == nil && used >= c.RuView.StorageMB<<20
		if limited { bounded, cancel := context.WithTimeout(ctx, 5*time.Second); _, _ = m.request(bounded, c, "POST", "/api/v1/recording/stop", nil); cancel() }
		m.mu.Lock(); m.status.StorageLimit = limited; m.mu.Unlock()
		if err == nil { m.noteStorage(c.RuView.Mode, limited) } // an unreadable directory neither opens nor closes the hint
	}
	m.mu.Lock(); generation := m.generation; m.mu.Unlock()
	return generation
}
```
- [ ] Update the comment at `manager.go:150-154` (M24 is now handled by `inspectExit`).
- [ ] **Test (background_test.go):** `TestRuViewDockerExitIsDetected`.
  - Setup: `c.RuView.UDPEnabled, UDPBind, UDPAllow = true, "192.168.1.2", []string{"192.168.1.4"}`; `m.dockerEvery = 0`; `m.Start` → nil; precondition `Status().UDPReady`.
  - Then `f.exited.Store(true)` → `m.watch` returns nil.
  - Status must have: `Runtime==exited`, `ErrorCode==process_exited`, `ErrorDetail` containing "137", `StopReason==process_exit`, `!UDPReady`, `!RevisionVerified`, `!Ready`.
  - `m.Nodes(ctx).LiveState == not_running`.
  - Then `f.exited.Store(false)`: `m.Start` → nil (no `already_running`).
- [ ] **Test (hints_test.go:153-180)** "storage" subtest. Set `m.storageEvery = 0`, `c.RuView.StorageMB = 1`, create `ruview/data`, write a file of `1<<20+1` bytes → 3× watch → hint open. A disabled watch keeps the hint. Removing the file closes it on the next watch. The "missing directory = full" assumption goes away.
- [ ] **Test (new status_fields_test.go):** `TestRuViewStorageSamplingIsThrottledAndErrorsAreNotFull` (`managedBudget(t,"native")`, `StorageMB=1`):
  1. first watch with the dir missing → `StorageUsedBytes==0`, `StorageBudgetBytes==1<<20`, `!StorageLimit`
  2. `storageEvery=time.Hour`, write a 2 MiB file, watch → `StorageUsedBytes` still 0 (throttled)
  3. `storageEvery=0`, watch → 2 MiB, `StorageLimit`, hint open
  4. `m.walkStorage = func(string)(int64,error){ return 0, errors.New("permission denied at /secret/x") }`, watch → `!StorageLimit`, `StorageUsedBytes==nil`, `ErrorCode==storage_unreadable`, hint still open (unchanged)
  5. `m.Proxy` POST `/api/v1/recording/start` → 503, not 507

### Task A6 — §8.1 fields in `Status()` and `noteRecording`

- [ ] `hints.go`: `func (m *Manager) openHintsLocked() []string` returns the sorted open hint names, or `[]string{}`.
- [ ] `manager.go:111-148` `Status()`. Under `mu`, after `s := m.status`:
  - `running := m.runningLocked(); s.Runtime = m.runtimeLocked(c.Mode, running)`
  - copy `phase`, `phaseSince`, `stopReason`, `stoppedAt`, `lastFreshAt`, `recordingAt` into pointer fields (nil when zero)
  - `s.Hints = m.openHintsLocked()`
  - snapshot `storageKnown/Used/Err`

  After unlocking, before the `!c.Enabled` early return:
  - for `c.Mode != "external"`: `budget := c.StorageMB<<20; s.StorageBudgetBytes = &budget`, and `if storageKnown && !storageErr { s.StorageUsedBytes = &used }`
  - normalize: `if s.ErrorCode == "" && s.Error != "" { s.ErrorCode = knownCodeOr(s.Error, CodeUnknown) }`
  - `if s.ErrorCode == "" && storageErr && c.Mode != "external" { s.ErrorCode = CodeStorageUnreadable }`
- [ ] `noteRecording` (`:198-213`): on 2xx also `m.recordingAt = time.Now().UTC()`.
- [ ] **Test (status_fields_test.go):** `TestRuViewStatusFields`.
  - `json.Marshal(Status{})` contains each key from §2.1 (with `"hints":null` allowed only for the zero struct; `m.Status()` must return `[]`).
  - External fixture (`ruview_test.go:23`): `Runtime==external`, `StorageBudgetBytes==nil`, `len(Hints)==0 && Hints!=nil`.
  - Fresh probe → `LastFreshAt` set; a stale probe keeps it; `Source` change + `Configure` → nil.
  - `noteRecording("/api/v1/recording/start",200)` → `RecordingAt` set; then `noteRecording(...,502)` → unchanged.
  - `emitHint(hintSensingStale,true)` → `Hints==["sensing_stale"]`.
  - Fixture handler answers 401 → `ErrorCode==upstream_auth`; 503 → `upstream_error`.
- [ ] **Test:** `TestRuViewScrubDetail` table: URL → `[url]`; `C:\Users\x\data` and `\\host\share` → `[path]`; `/var/run/docker.sock` → `[path]`; a token registered with `security.RegisterSensitive` → absent; 600 × "ä" → ≤240 bytes and valid UTF-8; a control character → space.

### Task A7 — fix existing Go assertions broken by A1–A6

- [ ] `lifecycle_test.go:200-224` `TestRuViewConfigureKeepsLifecycleError`. The second half now asserts that an AgentAccess toggle **keeps** `Ready`. Add **`TestRuViewConfigureCancelsOnlyRuntimeChanges`**, a table over `agent_access`, `readonly` and `storage_mb` (no cancel) versus `source=wifi` and `http_port+1` (cancel). Use `nativeFixture` with `p.kill = func() error { close(p.done); return nil }`. Install a **new** config object per case (`next := *c; next.RuView = c.RuView; change(&next.RuView); m.config = func() *config.Config { return &next }`), because `runningConfig` aliases `c`. Hold `ctx, done := m.bound(context.Background())` as the "pull/stream":
  - non-runtime change → `ctx.Err()==nil`, `Status().Ready` kept, `watch` returns non-nil
  - runtime change → `ctx.Err()!=nil`, status reset; `watch` returns nil; then `Runtime==stopped`, `StopReason==config_changed`, `StoppedAt!=nil`
- [ ] `ruview_test.go:248-251` `TestReconfigurationCancelsLiveRequests`: change `next.RuView.ReadOnly = false` to `next.RuView.Source = "wifi"`.
- [ ] Run `go test ./internal/ruview/ ./internal/server/ -count=1 -run 'TestRuView|TestKnownNodes|TestBackground|TestNative|TestStop|TestReadOnly|TestStatus|TestSource|TestDashboard|TestRecording|TestReconfiguration|TestNoRedirect|TestPoseStream|TestBoardSetup|TestStartup'`, then `go test ./internal/ruview/ -count=1` (whole package) and `go vet ./internal/ruview/ ./internal/server/`.

### Task A8 — D4 handler verification

- [ ] Verified while planning: `PUT /api/config` is `requireAdmin` (`server_routes_config.go:89-90`).
  - `handleUpdateConfig` (`config_handlers_main.go:449`) deep-merges the patch into the YAML map.
  - `deepMerge` (`:2205-2268`) writes a JSON `false` through the `default:` branch as `dst["readonly"]=false`.
  - `validateCompleteConfigListPatch` checks only list paths, and `ValidateRuViewConfig` runs on the merged result (`:754`).
  - The hot reload calls `s.RuView.Configure()` (`server.go:626`), which with A1 no longer cancels anything for `readonly`.
- [ ] **Test (`internal/server/ruview_errors_test.go`):** `TestRuViewReadOnlySingleFieldSave`, built on the pattern of `ai_gateway_reload_test.go:29-71`.
  - Initial YAML: `ruview:\n  enabled: true\n  readonly: true\n  agent_access: true\n  mode: docker\n  source: esp32\n  storage_mb: 512\n`
  - PUT `{"ruview":{"readonly":false}}` → 200
  - `config.Load(path)` → `!ReadOnly && Enabled && AgentAccess && StorageMB==512 && Mode=="docker" && Source=="esp32"`

---

## 4. Batch B — pure model + adapter fix (maker-agent; wave 1, parallel with A)

**Files:** `ui/js/desktop/apps/ruview-model.js` (new), `scripts/ruview/model.test.mjs` (new), `ui/js/ruview/upstream-adapter.js`, `ui/js/vendor/ruview/aurago-adapter.js`, `ui/js/vendor/ruview/provenance.json`, `scripts/ruview/upstream.test.mjs`.

### Task B1 — `ruview-model.js`

- [ ] Write a classic script IIFE that assigns `window.RuViewModel` with the API in §2.5. It has no DOM access, no `fetch` and **no `t(` calls**; it returns ids and key ids only. `Intl` is allowed. Keep it under 900 lines.
- [ ] Port today's rules:
  - `mergeNodes`: `ruview.js:188-203`, including the same-scope retained-RSSI rule and the `live_state` normalization
  - node freshness: `ruview.js:116,173`, identical to `presenceFresh` in `read.go:100-106`
- [ ] `phaseStep`: Docker `[preparing,pulling_image,creating,waiting_ready]`, native `[preparing,verifying,creating,waiting_ready]`, otherwise null.
- [ ] `ageParts`: null for non-finite or negative values; `<1000` → ms; `<60000` → s, with one decimal under 10 s when the decimal is non-zero (today's rule at `ruview.js:52-56`); `<3600000` → min; else h.

### Task B2 — `scripts/ruview/model.test.mjs`

- [ ] Load the file in `vm.runInNewContext(source, {window, Intl, console, Date, Math, JSON, Set, Map, Number, String, Array, Object})`, as `scripts/test-infra-ui.mjs:14-17` does.
- [ ] Write **one case per §7 row** (rows 1–17 + 3a + 12a + F) using a `base()` status helper (`{enabled:true, readonly:false, mode:'docker', source:'esp32', runtime:'running', ready:true, fresh:true, data_state:'live_unverified', last_frame_age_ms:800, observed_at:<now-1s>, …}`) plus the row's override. Assert `id`, `tone`, `glyph`, `action`, `actionLabel` and the secondary kind. Include at least:
  - row 11 with `image_denied` → action `runtime/start` + `action_retry_start`
  - row 11 with an unknown code → `errorCode==='unknown'`, action `test`
  - row 6 with `storage_unreadable` → still row 6, plus overlay `storage_unreadable`
  - row 16 wifi → headline variant flag, `boards===null`
  - row 4 with `desktopReadonly` → `open-settings`
- [ ] Add small tables for:
  - `blockReason`: readonly blocks start and flash but not stop/test; desktop readonly; pending
  - `setupSteps`: esp32 managed with UDP enabled but not running → receiver not done (M30); wifi has 3 steps; simulated → null; `last_fresh_at` → null
  - `boardFreshness`: 10000 ms live, 10001 stale, simulated stale, no sequence stale, offline offline
  - `ageParts(null)===null`
  - `diagnostics` contains no `error=` line, no `http://` and no token-like value
- [ ] Run: `node --test scripts/ruview/model.test.mjs`.

### Task B3 — adapter status timeout (P0 leftover)

- [ ] `ui/js/ruview/upstream-adapter.js:180-189` `policy()`:
  - per call: `const call=new AbortController(); const timer=setTimeout(()=>call.abort(),10000); const onEnd=()=>call.abort(); lifetime.signal.addEventListener('abort',onEnd,{once:true});`
  - pass `signal: call.signal` to both `load(...)` calls
  - in `catch`: `readonly=true;enabled=false;simulation=false;`
  - in `finally`: `clearTimeout(timer); lifetime.signal.removeEventListener('abort',onEnd); policyBusy=false;`

  A hung fetch can then no longer keep `policyBusy` set and `simulation` stale.
- [ ] Regenerate the vendored copy the way `vendor-ui.mjs:230` does: copy `ui/js/ruview/upstream-adapter.js` to `ui/js/vendor/ruview/aurago-adapter.js` with `\r\n`→`\n`. Set `provenance.json` → `files["aurago-adapter.js"]` (currently `:91`) to the sha256 of the written bytes.
  - Use a scratchpad script (`node <scratch>/rehash.mjs`) that reads, normalizes, writes, hashes and patches only that one entry.
  - Do **not** run `vendor-ui.mjs --source` (it needs the pinned upstream checkout).
  - Verify with `node scripts/ruview/vendor-ui.mjs --check`.
- [ ] **Test (`upstream.test.mjs`):** add `test('a hung status read drops the simulation gate', …)`.
  - Run the adapter source in `vm` with a fake `window.fetch`: the first call returns `{enabled:true, readonly:false, source:'simulated'}` (+ capabilities); later calls return a never-settling promise that rejects on `signal` abort.
  - Fake `setTimeout`/`setInterval` capture callbacks; `document={addEventListener(){}, querySelectorAll:()=>[], body:{}}`; `MutationObserver` is a stub class; `addEventListener` captures `DOMContentLoaded`.
  - Fire `DOMContentLoaded` and assert `window.auragoRuViewSimulationAllowed()===true`.
  - Fire the interval, fire the 10 s timeout and await microtasks; then assert `false`.
- [ ] Run: `node --test scripts/ruview/upstream.test.mjs scripts/ruview/model.test.mjs` and `node scripts/ruview/vendor-ui.mjs --check`.

---

## 5. Batch C — shell, store, views, CSS, en/de, registration, browser tests (maker-agent; wave 2, parallel with D)

**Files:**
- `ui/js/desktop/apps/{ruview-store.js,ruview-views.js}` (new), `ui/js/desktop/apps/ruview.js` (rewrite), `ui/css/ruview.css` (rewrite)
- `ui/js/desktop/core/module-loader.js`, `ui/js/desktop/core/menus-and-routing.js`, `ui/js/desktop/bundles/main.bundle.js` (generated)
- `ui/i18n_lint_test.go`, `ui/lang/ruview/en.json`, `ui/lang/ruview/de.json`
- `scripts/ruview/{browser.test.mjs,ui-browser.mjs,wizard-browser.test.mjs,upstream.test.mjs,model.test.mjs}`

Run GitNexus impact for `RuViewApp` and `renderAppContent` (menus-and-routing) first.

### Task C1 — registration and shell context

- [ ] `module-loader.js:28` → `'ruview': { styles: appStyles('/css/ruview.css'), scripts: ['/js/desktop/apps/ruview-model.js', '/js/desktop/apps/ruview-store.js', '/js/desktop/apps/ruview-views.js', '/js/desktop/apps/ruview.js'] },`
- [ ] `menus-and-routing.js:1654` → pass the Infra set (§8.7; copy the shape from `:1644-1652`):
```js
if (appId === 'ruview' && window.RuViewApp) {
    return window.RuViewApp.render(contentEl(id), id, Object.assign({}, context || {}, {
        esc, t, api, openApp, confirmDialog, setWindowMenus, clearWindowMenus, readonly: desktopReadonly(),
        setWindowBeforeClose: (winId, handler) => { const win = state.windows.get(winId); if (win) win.beforeClose = handler; },
        showConfirmModal: (title, message, confirmLabel) => confirmLabel
            ? modalDialog({ title, message, choices: [{ value: 'confirm', label: confirmLabel }] }).then(value => value === 'confirm')
            : confirmDialog(title, message)
    }));
}
```
- [ ] Run `npm run build:ui` (the bundle part changed), then `node scripts/build-ui-bundles.js --check`.
- [ ] `ui/i18n_lint_test.go:1075`: register `js/desktop/apps/ruview-model.js`, `ruview-store.js` and `ruview-views.js` with `{name: "t", prefix: "ruview."}`, next to the existing `ruview.js` entry.

### Task C2 — `ruview-store.js` (`window.RuViewStore = { create }`)

- [ ] `create({ base = '/api/desktop/integrations/ruview/', fetch, desktopReadonly })` returns `{ state, subscribe(fn), attach({root, win}), setView(view), refresh(), run(action), allowChanges(), dispose() }`.
  - `state`: `{status, statusLoaded, statusFailed, statusAt, nodes:{list, scope, liveState, loaded, error, receivedAt}, pending:Set, actionError, view, visible, disposed, desktopReadonly, lastCheckAt}`.
- [ ] `request(route, {method, body, timeout})`: an `AbortController` per request, linked to the store's dispose controller.
  - Timeouts: GET 10 s, POST `test` 20 s, POST `runtime/stop` 30 s, `PUT /api/config` 15 s; POST `runtime/start`/`runtime/install` have none (abort only on dispose).
  - Rejects with `{code, message, detail, status}`: `code` = `body.error_code`, else 403 → `forbidden`, network → `network`, abort-by-timeout → `timeout`, else `unknown`.
- [ ] Polling uses `setTimeout` chains at 3 s, with no overlap (one in-flight request per endpoint) and only while `visible`:
  - **status**: GET `status`, plus GET `nodes` only if `status.enabled && status.source==='esp32' && status.mode!=='external'` (**D6**: Status never dials an external service)
  - **boards**: GET `status` + GET `nodes` when enabled (explicit data read while visible, D6)
  - **lab**: GET `status` only, for the pill
- [ ] Status epochs: each status-producing request takes `seq=++counter`, and a poll result applies only if `seq > appliedSeq`. An action response always applies and sets `appliedSeq=++counter`, so in-flight older polls are dropped. A poll failure sets `statusFailed=true` and keeps the last status (H9). Nodes go through `RuViewModel.mergeNodes`.
- [ ] Visibility follows the Infra pattern (`infra-store.js:785-827`): `visibilitychange`; a `MutationObserver` on `win` (`style`,`class`,`hidden`,`data-space-hidden`); an `IntersectionObserver` on `root`.
  - `visible = !disposed && !document.hidden && inView && (!win || (!win.hidden && !win.classList.contains('vd-space-hidden') && win.style.display !== 'none' && win.getClientRects().length > 0))`.
  - Every tick also checks `root.checkVisibility?.()`, and skips but reschedules when false. This covers the harness without `.vd-window` and the minimised window (`window-interactions-runtime.js:17` sets `display:none`).
  - Hidden → clear timers and abort in-flight GETs (never actions). Visible → immediate poll + schedule.
  - Listen to `aurago:desktop-policy` to update `desktopReadonly`.
- [ ] `run(action)`:
  - `test` → POST `test`; for external esp32 also GET `nodes` once; sets `lastCheckAt`
  - `runtime/*` → POST
  - polling continues while actions are pending, so phases update during a long start
  - the `actionError` is kept unless `code==='cancelled'` or `code===status.error_code`
- [ ] `allowChanges()` → `PUT /api/config`, `Content-Type: application/json`, body exactly `JSON.stringify({ruview:{readonly:false}})`, then an immediate status refresh.

### Task C3 — `ruview-views.js` (`window.RuViewViews = { status, boards, lab }`)

- [ ] Each `mount(panel, deps)` returns `{update(snapshot), tick(), hide?(), dispose()}`, with `deps = {t, esc, lang, model, onAction(action, extra)}` and `snapshot = {state, derived, steps, views, now, nowPerf}`. Build the skeleton once with escaped static labels. `update` patches `textContent`/attributes/`hidden` only when a value changes, so elements are never replaced and **focus survives polls (H10)**.
- [ ] Dynamic copy comes **only** through literal maps so the lint sees every key: `STATE_COPY`, `ERROR_COPY` (every code in §2.2 + `network`, `forbidden`), `PHASE_COPY`, `RUNTIME_COPY` (`state_*`), `STOP_COPY`, `PILL_COPY`, `ACTION_COPY`, `REASON_COPY`, `STEP_COPY`, `HINT_COPY`, `AGE_COPY` (`age_ms/s/min/h`), `SOURCE`/`MODE`/`RECORDING`/`DATA_STATE` (moved from `ruview.js:16-31`).
  - Each map has the form `{x: () => t('…')}` and is read with today's `pick` helper (`ruview.js:15`).
  - No literal fallbacks (`TestFrontend_JS_NoLiteralTranslationFallbacks`).
- [ ] **Status view** (hooks per §2.6):
  - headline split around `{age}` into the age span; secondary; next-action button with aria-disabled + visible reason (`aria-describedby`); secondary action; overlay chips
  - error block with `<details data-ruview-technical>` (raw text only via `textContent`)
  - action error block; stepper (`<ol>` with sr-only state words `check_ok`/`check_attention`/`check_unchecked`)
  - Details: `<details>` whose open state is read from and written to `localStorage['ruview.ui.v1'] = {"details":true|false}` inside try/catch, **written only on the toggle event**
  - Details rows:
    - Service: `state_*` · mode · version (`revision_verified`/`revision_unverified`)
    - Source: source word · `receiver_set`/`receiver_missing` · `udp_on`/`udp_off`/`udp_operator`, plus `devices_not_applicable` when the source is not esp32
    - Storage: `storage_of` with `formatBytes`, or `storage_unknown` / `—` for external
    - Recording: `recording_last`, or `recording_unknown` + `recording_unconfirmed`
    - Hints: `hints_none` or the hint words
  - Copy diagnostics: `navigator.clipboard.writeText(model.diagnostics(...))`; on failure fill and show `[data-ruview-diagnostics]`
  - About note
- [ ] **Boards view** is today's `devices()` (`ruview.js:165-187`), restyled as a card grid and keyed by `node_id` (`Map<id, article>`: create missing, remove gone, reorder only when the order changes).
  - Same texts: `data-fresh` → `received` / `device_offline` / `stale`; RSSI is `String(value)` + `' dBm'`, with `rssi_retained` for an offline retained value.
  - Same feedback rule (`ruview.js:184`) in `[data-ruview-boards-feedback]`.
  - `tick()` updates `[data-age]`/`data-age-ms` and `[data-fresh]` via `textContent` only.
- [ ] **Lab view (minimal)**:
  - Landing of three cards: Observatory (`observatory`, `lab_observatory_desc`, `lab_req_csi`), Pose fusion (`pose_fusion`, `lab_pose_fusion_desc`, `lab_req_csi`), Original dashboard (`lab_dashboard`, `lab_dashboard_desc`, `lab_req_any`), each with both chips.
  - The iframe is mounted **only on `[data-lab-open]`**, exactly as today (`ruview.js:241-245`):
    - dynamic import of `relay.mjs` via `AuraLazyAssets.versionedURL`
    - keep the literal `frame.sandbox='allow-scripts'` and the same `allow`
    - `frame.name` from `aurago.ruview.onboarding.done` (try/catch)
    - `frame.title = t('lab_frame_title_…')` through a literal map (M5)
    - `relayDetach = attachRuViewRelay(frame, base)`, then `frame.src = base + 'ui/' + page`
    - pose fusion shows `fusion_notice` above the frame
  - `[data-lab-close]`, switching views and `dispose` unmount the frame and detach the relay (unchanged from today). Unmounting on hide/minimise stays P4.

### Task C4 — `ruview.js` composition root (rewrite of `ui/js/desktop/apps/ruview.js`)

- [ ] `render(host, id, ctx)`: `dispose(id)`. Use the `t(key)` helper with a warn-once missing-key fallback (keep `ruview.js:8-13`). `lang = document.documentElement.lang || 'en'`. Build the root, command bar, the three panels and the single live region. Create the store, mount the views, `store.attach({root, win: host.closest('.vd-window')})`, subscribe → derive → patch.
- [ ] Command bar:
  - tabs with roving tabindex: Left/Right/Home/End move focus and activate; Enter/Space activate; only available views are visible
  - view menu (below 720 px) and ⋯ menu: `role=menu`, ArrowUp/Down, Escape closes and returns focus to the trigger, outside click closes
  - pill click → Status
  - primary slot: Boards → Add board with `blockReason('flash')`; the reason text goes in `[data-ruview-add-reason]`
- [ ] Actions (click delegation, ignoring `aria-disabled="true"`):
  - `test` / `runtime/*` → `store.run`
  - `flash` / `[data-node]` → open the wizard as today (`ruview.js:252-261`), keeping `wizardClose` for dispose
  - `allow-changes` → `(ctx.showConfirmModal ? ctx.showConfirmModal(t('allow_changes_title'), t('allow_changes_body'), t('allow_changes_confirm')) : ctx.confirmDialog?.(t('allow_changes_title'), t('allow_changes_body')))`; on true `store.allowChanges()`; announce `allow_changes_done`, or show `allow_changes_failed` + the error. Hidden when `ctx.readonly`.
  - `open-settings` → `window.open('/config#ruview','_blank','noopener')`
  - `show-boards` → setView; `refresh` → `store.refresh()`; `copy-diagnostics`; `about` → Status + open/focus the about details
- [ ] Live region:
  - announce the headline text when `derived.id` changes after the first resolved state, and action results (`announce_checked`, errors, `diagnostics_copied`)
  - never announce on unchanged polls
  - ages tick in `aria-live=off` spans
- [ ] Ticker: `setInterval(1000)` only while `store.state.visible` and an age is on screen (live/stale/external_check_old headline, pill or Boards). Each tick re-derives (cheap) and patches, so 10 s expiry flips the headline without a poll.
- [ ] `dispose`: store dispose, view disposes, Lab unmount, `wizardClose?.()`, clear timers, remove listeners (AbortController), `instances.delete`. `window.RuViewApp = {render, dispose}`. Keep the file under 900 lines.

### Task C5 — `ruview.css` rewrite (tokens only)

- [ ] `.ruview-app`:
  - `container: ruview / inline-size; display:grid; grid-template-rows: 44px minmax(0,1fr); background: var(--vd-theme-app-bg); color: var(--vd-text)`
  - tone variables copied from Infra (`desktop-app-infra.css:2`): `--ruview-ok/warn/bad/info: var(--ds-color-*-fg)` and `--ruview-*-text: light-dark(color-mix(...), ...)`; neutral = `--vd-theme-muted`
  - `[data-tone=…]` sets `--ruview-tone` / `--ruview-tone-text`
- [ ] Command bar: `height:44px; display:flex; flex-wrap:nowrap; white-space:nowrap; overflow:hidden; border-bottom:1px solid var(--vd-theme-border); background: var(--vd-theme-panel-bg)`. The view switch is styled like `.infra-view-switch` (`aria-selected`); menus like `.infra-menu`.
- [ ] Controls: `min-height:36px`, `@media (pointer: coarse) { min-height:44px }`. Focus ring `outline: var(--ds-focus-ring-width) solid var(--ds-color-border-focus); outline-offset: var(--ds-focus-ring-offset)`. `[aria-disabled="true"]` gets `opacity:.5; cursor:not-allowed`.
- [ ] `@container ruview (max-width: 719px)`: hide `[data-ruview-tablist]`, show `[data-ruview-view-menu]`.
- [ ] `@container ruview (max-width: 559px)`: hide `[data-ruview-pill-age]` and `.ruview-brand-name`, and make the primary label `.ruview-sr-only` (icon + `aria-label`).
- [ ] The busy glyph spins only under `.desktop-body:not([data-animations="false"])` and `@media (prefers-reduced-motion: no-preference)`.
- [ ] Wizard part (`ruview.css:34-113`): keep the selectors, and make every fallback chain end in a system colour or `color-mix`. Examples:
  - `--ruview-surface: var(--pw-surface, var(--vd-theme-app-bg, Canvas))`
  - `--ruview-text: var(--pw-text, var(--vd-text, CanvasText))`
  - `--ruview-muted: var(--pw-muted, var(--vd-theme-muted, GrayText))`
  - `--ruview-border: var(--pw-line-strong, var(--vd-theme-border-strong, GrayText))`
  - `--ruview-accent: var(--pw-accent, var(--vd-accent, LinkText))`
  - backdrop `color-mix(in srgb, CanvasText 72%, transparent)`
  - shadow `0 20px 64px color-mix(in srgb, CanvasText 40%, transparent)`
  - success/danger `var(--pw-success, var(--ds-color-success-fg, currentColor))` / `var(--pw-danger, var(--ds-color-danger-fg, currentColor))`

  Remove `#35a8bf`, `#71809655`, `#57adbe` and every other hex value (M6). Delete the obsolete `.ruview-toolbar/.ruview-summary/.ruview-content/.ruview-check li` overview rules, but keep the wizard's `.ruview-wizard .ruview-check`. The stepper class `.ruview-setup` must not collide with the wizard's `.ruview-steps`.
- [ ] Use no ID selectors and no `cc-` classes.

### Task C6 — en/de strings

- [ ] Add every key in §2.7 to `ui/lang/ruview/en.json` and `de.json` with exactly the listed texts. Remove `ruview.live_unverified`, `ruview.synthetic` and `ruview.stopped` from both. Do not rename existing keys.

### Task C7 — test updates and the seven new browser checks

- [ ] `scripts/ruview/upstream.test.mjs:194` reads `ui/js/desktop/apps/ruview-views.js` (the iframe code moved there); the assertions stay.
- [ ] Append to `scripts/ruview/model.test.mjs`:
  - **no hex colours** in `ui/css/ruview.css`: strip comments, assert `!/#(?:[0-9a-f]{3,4}|[0-9a-f]{6}|[0-9a-f]{8})\b/i.test(css)`
  - each of the four RuView JS files has fewer than 900 lines
  - every `Code\w+\s*=\s*"(\w+)"` in `internal/ruview/errors.go` is in `RuViewModel.KNOWN_ERROR_CODES`
- [ ] `scripts/ruview/browser.test.mjs` harness (`:66-68`):
  - `<html lang="${lang}">`
  - load `ruview-model.js`, `ruview-store.js`, `ruview-views.js`, `ruview.js`
  - ctx `{t, esc, readonly:false, confirmDialog: async()=>window.__ruviewConfirm!==false, showConfirmModal: async()=>window.__ruviewConfirm!==false, openApp(){}, setWindowBeforeClose(){}, setWindowMenus(){}, clearWindowMenus(){}}`
- [ ] Status fixture (`:64`): emit the new shape (`runtime: mode==='external'?'external':(serviceState||'running')`, `error_code: statusError?'unreachable':''`, `error_detail: statusError`, `hints:[]`, `stop_reason:''`, …), still merged with `nodeFixture.statusExtra`. Add a `PUT /api/config` handler that records bodies in `configPuts`, sets `readonly=false` and returns `{"status":"ok"}`. The existing `configRequests` counter stays for the iframe assertion (`:152-154`).
- [ ] Map existing assertions to the new hooks:

| old | new |
|---|---|
| `[data-action=test]` waitFor (`:100,113,127,141`) | `[data-ruview-headline][data-state]:not([data-state=loading])` |
| `[data-tab=devices]` / `[data-tab=overview]` (`:105,122` + ui-browser) | helper `openView(page,'boards'\|'status')` in ui-browser.mjs (tab if visible, else view menu `[data-view-option]`; needed at 390 px) |
| `.ruview-summary article strong` labels (`:114-119`) | loop over `[state override, expected data-state]`: `{runtime:'not_started'}`→`stopped`, `{runtime:'starting',phase:'pulling_image'}`→`starting_pull`, default→`live`; keep `writes===0` |
| `checkReceptionStates` | `checkStatusStates` (below) |
| `[data-action=flash]` disabled (`:141`) | `getAttribute('aria-disabled')==='true'` (on Boards) |
| `[data-action="runtime/stop"]` disabled for external (`:142`) | open `[data-ruview-more]`; no `[data-ruview-menu] [data-action="runtime/stop"]` exists |
| native stop enabled while readonly (`:143-145`) | `statusExtra={runtime:'running'}`, mode native: the menu item exists and is not aria-disabled; click it (`writes` +1); no element `[data-action="runtime/start"]:not([aria-disabled="true"])` |
| failure loop `.ruview-feedback` (`:128-135`) | `[data-ruview-action-error]` contains the detail (in `<details>`), no `img`, and persists after 3.2 s |
| statusError `.ruview-feedback` (`:136-137`) | headline `data-state=error` with text `err_unreachable`; `[data-ruview-technical]` contains "could not be reached" |
| `[data-tab="${view}"]` iframe (`:147-148, 225, 242`) | `openView(page,'lab')` + `[data-lab-open="${view}"]`; also assert `iframe.title` equals the translated `lab_frame_title_*` |
| ui-browser `checkDevices` overview refresh assertion (`:51-54`) | D6: on Status with mode external, `fixture.reads` does not change over 4.2 s; with `statusExtra={mode:'docker',runtime:'running'}`, reads increase |

- [ ] New checks (in `ui-browser.mjs`, called from `browser.test.mjs`; viewport 1100×760, de):
  1. **Headline and next action visible without scrolling** for each fixture in `STATE_FIXTURES` (rows 2–17 + 3a + 12a, overrides as in §2.5). Row 2 is reached by setting `nodeFixture.statusFail=true` after the first load; row 12a by `observed_at` 120 s old. Assert `[data-state]` equals the expected id, `.ruview-body.scrollTop===0`, the headline rect is inside `[commandbar.bottom, innerHeight]`, and the `[data-ruview-next]` rect (when not hidden) has `bottom ≤ innerHeight`.
  2. **The command bar does not wrap at 560 px**: viewport 560×760; `.ruview-commandbar` `offsetHeight===44 && scrollWidth<=clientWidth`, and every visible child has the same `getBoundingClientRect().top` (±1).
  3. **Focus is kept across two polls**: focus `[data-ruview-next]` on row 16 and tag it with `dataset.focusProbe`; wait until `fixture.statusReads` grows by 2; `document.activeElement.dataset.focusProbe==='1'`. Repeat on Boards with `button[data-node="7"]`.
  4. **The live region is silent on routine polls**: attach a `MutationObserver` counter to `[data-ruview-live]`; two polls with unchanged state → 0 mutations. Then set `offline=true` → exactly one change, whose text equals the new headline.
  5. **Docker `image_denied` is the headline cause**: fixture row 11 (`error_detail:'HTTP 403: registry denied <img src=x onerror="window.ruviewInjected=1">'`). The headline text equals de `err_image_denied`; the headline does not contain "registry"; `[data-ruview-technical-text]` contains it as text; no `img`; `!window.ruviewInjected`; `[data-ruview-next][data-action="runtime/start"]`.
  6. **D4**: fixture `readonly=true`, mode docker, runtime `not_started` → row 4. With `window.__ruviewConfirm=false`, clicking `[data-ruview-next][data-action=allow-changes]` produces 0 PUTs. With `true`, it produces exactly one PUT to `/api/config` whose body deep-equals `{"ruview":{"readonly":false}}`; the headline then becomes `stopped`.
  7. **No hex colours**: covered by `model.test.mjs` (above).
- [ ] `checkStatusStates(page, fixture, set)`:
  - wifi source → Boards tab hidden; the details source row contains `devices_not_applicable` text; no step is `current` for UDP/receiver
  - managed docker `not_started` with `udp_enabled:true, udp_ready:false` → headline `stopped`, `[data-step=service]` current, `[data-step=receiver]` not done (M30)
  - disabled → headline `off`; Boards and Lab tabs hidden; `fixture.reads` unchanged over 4.2 s
  - first status failure → headline `status_failed`, never `off`
- [ ] `scripts/ruview/wizard-browser.test.mjs`: the harness at `:61` loads the four scripts and sets `<html lang>`; `open()` (`:110-114`) calls `openView(page,'boards')` before clicking `[data-action="flash"]` (import the helper from ui-browser.mjs or inline it).
- [ ] Runs, from the worktree root (PowerShell):
  - `node --test scripts/ruview/model.test.mjs scripts/ruview/upstream.test.mjs scripts/ruview/protocol.test.mjs`
  - `$env:AURAGO_PLAYWRIGHT='C:/Users/Andi/AppData/Local/npm-cache/_npx/420ff84f11983ee5/node_modules/playwright'; node scripts/ruview/browser.test.mjs; node scripts/ruview/wizard-browser.test.mjs`
  - Locale loops in the browser test pass for en/de only until D lands. Before D, run with a temporary local narrowing that is not saved, or accept the failures for the other 14 locales and report them.
  - `go test ./ui/ -count=1 -run 'TestFrontend_|TestVirtualDesktopFirstPartyJSFilesStayBelowLineBudget|TestVirtualDesktopJSUsesSemanticChunkNames|TestUIAvoidsCookieConsentClassPrefix|TestShippedWebAssetsLoadNothingExternal'`

---

## 6. Batch D — 14 locales + Go i18n test (maker-agent; wave 2, parallel with C)

**Files:** `ui/lang/ruview/{cs,da,el,es,fr,hi,it,ja,nl,no,pl,pt,sv,zh}.json`, `internal/server/ruview_i18n_test.go` (new). If one dispatch is too large, split it into two dispatches of 7 locales each.

- [ ] Translate every key in §2.7 from the English text, with real translations. Keep RuView, CSI, RSSI, UDP, dBm, ESP32, Docker and aurago-ruview unchanged, and keep `{placeholders}` and `#` intact. Match each locale's existing RuView tone. In Lab titles, label upstream pages as English.
- [ ] P0 leftover: capitalise `ruview.check_ok` / `ruview.check_attention` in cs (Hotovo/Otevřeno), da (Færdig/Åben), el (Έγινε/Ανοιχτό), es (Hecho/Pendiente), fr (Fait/Ouvert), it (Fatto/Aperto), nl (Klaar/Open), no (Ferdig/Åpent), pl (Gotowe/Otwarte), pt (Feito/Em aberto), sv (Klart/Öppet). hi, ja and zh stay as they are.
- [ ] Remove `ruview.live_unverified`, `ruview.synthetic` and `ruview.stopped`.
- [ ] **Test:** `internal/server/ruview_i18n_test.go` (package `server`; imports `aurago/ui`, `aurago/internal/ruview`, like `ui_i18n_manifest_test.go:3-14`):
```go
func TestRuViewErrorCodesTranslated(t *testing.T) {
	families := map[string][]string{"ruview.err_": ruview.ErrorCodes(), "ruview.state_": ruview.Runtimes(), "ruview.phase_": ruview.Phases(), "ruview.stop_reason_": ruview.StopReasons()}
	en := readRuViewLocale(t, "en") // fs.ReadFile(ui.Content, "lang/ruview/en.json")
	for _, lang := range strings.Fields("cs da de el en es fr hi it ja nl no pl pt sv zh") {
		values := readRuViewLocale(t, lang)
		for prefix, list := range families { for _, v := range list { if strings.TrimSpace(values[prefix+v]) == "" { t.Errorf("%s missing %s%s", lang, prefix, v) } } }
		for key := range en { if strings.TrimSpace(values[key]) == "" { t.Errorf("%s missing %s", lang, key) } }   // parity
		for key := range values { if _, ok := en[key]; !ok { t.Errorf("%s has extra %s", lang, key) } }
	}
}
```
- [ ] Run, after C and D are both present: `go test ./internal/server/ -count=1 -run TestRuViewErrorCodesTranslated` and `go test ./ui/ -count=1 -run 'TestDesktopUsedI18nKeysExistInAllLanguages|TestFrontend_StaticI18nKeysExistInEnglishBundle|TestDesktopDynamicI18nPrefixKeysExistInAllLanguages'`.

---

## 7. Batch E — docs, bundle and vendor checks, full verification (maker-agent or docs-agent; wave 3)

**Files:** `documentation/ruview.md`, `documentation/manuals/en/30-ruview.md`, `documentation/manuals/de/30-ruview.md`, `internal/ruview/AGENTS.md`. `main.bundle.js` only if `--check` reports drift.

### Task E1 — `documentation/ruview.md`

- [ ] Rewrite the paragraphs at `:13` (Devices → **Boards**) and `:15` (Overview → **Status**: headline/next step/stepper/Details/Copy diagnostics/Lab landing). Cover:
  - **D6**: Status never contacts an external service; **Check now** and "last checked …"; Boards reads the board list every 3 s while visible
  - **D4**: **Allow changes…** saves only `ruview.readonly=false` through the admin config save
- [ ] `:31` and `:53`:
  - only runtime-relevant changes cancel requests and pulls (M27); agent access, read-only and the storage budget do not
  - start progress `phase`; Stop during a start cancels it; a second start answers `busy`
- [ ] `:39`: error codes replace raw Engine text; the Engine text appears under **Technical details**.
- [ ] `:57`/`:61`: Stop removes the owned container (M23); a container that exits on its own is detected as `exited` (M24); the storage check runs every 30 s, and an unreadable directory is `storage_unreadable`, not full (M28).
- [ ] Add a section **"Status fields and error codes"** listing the §2.1 fields and the §2.2 code → HTTP table, plus `{error_code, error, detail}`.
- [ ] Add `node --test scripts/ruview/model.test.mjs` to the command list at `:117`.

### Task E2 — manuals and DOX

- [ ] Manuals en and de `30-ruview.md`: replace "Reading the Overview and the Devices" (en `:73`, de `:75`) with Status/Boards; "The Upstream Views" (en `:140`, de `:148`) becomes "The Lab"; extend "When Something Goes Wrong" (en `:220`, de `:231`) with the headline causes, Technical details, Copy diagnostics and Allow changes. The German manual keeps lowercase "du".
- [ ] `internal/ruview/AGENTS.md`:
  - Ownership names the four desktop files.
  - Local Contracts gains: the §2.1 fields; the M27 Configure rule; the op slot / `opMu` protocol; Stop during start; `busy`; `waiting_ready`; M23 removal and foreign containers; M24 detection; M28 sampling and the 503 recording refusal; `errors.go` codes and `ruviewError`; the D6 polling rule.
  - The Verification list gains `go test ./internal/server/ -run 'TestRuViewError|TestRuViewRoute|TestRuViewReadOnlySingleFieldSave|TestRuViewErrorCodesTranslated'` and `node --test scripts/ruview/model.test.mjs`.

### Task E3 — full verification (report PASS counts)

- [ ] `npm run build:ui; node scripts/build-ui-bundles.js --check`
- [ ] `node scripts/ruview/vendor-ui.mjs --check; node scripts/build-browser-vendor.js --check; node scripts/ruview/check-artifacts.mjs`
- [ ] `go test ./internal/ruview/ ./internal/agent/ -count=1 -run 'TestRuView|TestKnownNodes'`
- [ ] `go test ./internal/ruview/ -count=1 -v` (count PASS lines)
- [ ] `go test ./internal/ruview/ ./internal/planner/ ./internal/server/ -count=1 -run 'TestRuView|TestOperationalIssue'`
- [ ] `go test ./ui/ -count=1 -run 'TestFrontend_|TestDesktopUsedI18nKeysExistInAllLanguages|TestDesktopDynamicI18nPrefixKeysExistInAllLanguages|TestVirtualDesktopFirstPartyJSFilesStayBelowLineBudget|TestVirtualDesktopJSUsesSemanticChunkNames|TestShippedWebAssetsLoadNothingExternal|TestUIAvoidsCookieConsentClassPrefix'`
- [ ] `node --test scripts/ruview/model.test.mjs scripts/ruview/protocol.test.mjs scripts/ruview/upstream.test.mjs`
- [ ] `$env:AURAGO_PLAYWRIGHT='C:/Users/Andi/AppData/Local/npm-cache/_npx/420ff84f11983ee5/node_modules/playwright'; node scripts/ruview/browser.test.mjs; node scripts/ruview/wizard-browser.test.mjs`
- [ ] `$env:AURAGO_RUN_BROWSER_SMOKE='1'; go test ./ui/ -count=1 -run 'TestRuViewConfigBrowser|TestRuViewConfigReceiverHintsBrowser|TestRuViewConfigLayoutBrowser|TestRuViewSerializedFirmwareCatalogBrowser'`
- [ ] Optional for the controller: `go test -race ./internal/ruview/` on aurago-test (no gcc locally), using the existing recipe.

---

## 8. Self-review

### Spec coverage

| P1 item / audit ID | Task |
|---|---|
| 1 · M27 (Configure cancels only runtime changes; stop reason; lifecycle errors survive) | A1, A7 (`TestRuViewConfigureCancelsOnlyRuntimeChanges`) |
| 2 · M29 (no `opMu` over pull/download; `phase`/`phase_since`; Stop during starting works; busy) | A3 (`TestRuViewStartPhasesBusyAndStopDuringPull`) |
| 3 · M23 (remove owned container; `container_conflict` + remedy) | A4, i18n `err_container_conflict` |
| 3 · M24 (Docker self-exit → `exited`; udp_ready/live_state follow) | A5 (`TestRuViewDockerExitIsDetected`) |
| 4 · M28 (30 s throttle; `storage_unreadable` not full; used/budget bytes) | A5 (`TestRuViewStorageSampling…`), hints_test update |
| 5 · §8.1 fields (runtime … hints; `error` kept) | §2.1, A6 (`TestRuViewStatusFields`, `TestRuViewScrubDetail`) |
| 6 · §8.4 typed errors + `ruviewError` (M2) | A2 (`TestRuViewErrorCodesMapToHTTP`, `TestRuViewRouteRefusalsCarryCodes`) |
| 7 · §9 split (model/store/views/app), module-loader, shell context §8.7 | B1, C1–C4 |
| 8 · command bar, tablist, pill, primary slot, ⋯ menu, container queries 720/560 | C4, C5, browser check 2 |
| 9 · Status headline/live region/stepper/Details (`ruview.ui.v1`)/storage/recording/hints/Copy diagnostics; D6; D4 | C3, C4, C2, browser checks 1/3/4/6, A8 |
| 10 · Lab minimal landing; mount on Open; translated title (M5); relay unchanged | C3, browser Lab mapping |
| 11 · tokens only / no hex (M6); a11y §10; i18n §12; `desktopLocalI18nHelpers`; `TestRuViewErrorCodesTranslated` | C5, C1, C6, D |
| 12 · P0 leftovers: capitalisation, 3 unused keys, adapter timeout (stale `simulation`) | D, C6, B3 |
| H3 (no false attention) / M30 (UDP step needs a running runtime) | model `setupSteps`, `checkStatusStates` |
| H9 (failed status ≠ off) | row 2, `checkStatusStates` |
| H10 (focus kept) | keyed patching C3, browser check 3 |
| M1 (missing key shows the key) | C4 `t` helper |
| M3 (start progress) | phases A3 + rows 7/8 |
| M7 (no flashing for Wi-Fi/demo) | Boards visible only for esp32 |
| M8 (storage numbers, labelled times) | Details, `secondary_last_checked` |
| M25 (external "ready" never expires) | row 12a |
| Docs | E1, E2 |

### Open risks

- **The single-field config save validates the whole merged config.** If an unrelated section is currently invalid, the PUT returns 400. The UI shows `allow_changes_failed` plus Technical details and keeps **Open settings** available.
- **aurago-test.** Stop now removes the owned `aurago-ruview` container (M23); Start already recreated it. The test-server rule forbids deleting or recreating existing containers, so ask the user before any Start/Stop acceptance there.
- **More Engine calls.** M24 adds one Engine inspect every 5 s while a managed Docker runtime runs (the socket proxy already permits container inspect). It runs outside `opMu` with a 5 s timeout.
- **New lock protocol (A3).** The races to watch are start's commit versus Stop/Configure; the explicit captured-generation binding and the commit-time check cover them. A `-race` run on aurago-test is recommended.
- **Translation volume.** About 160 new keys × 14 locales in D. The user reviews German in acceptance. Cross-batch tests are red until A, C and D are all merged.
- **Browser runtime.** The 96-case locale loop plus the new state walk makes `browser.test.mjs` slower.
- **Dead keys.** Keys made unused by the rewrite (`overview`, `devices`, `artifact`, `storage_ok`, `session_not_started`, `show_devices`, `check_runtime`/`udp`/`receiver`/`board`/`csi`, …) are left in place, because only the three named keys were in scope. Cleanup belongs in P5.
- **Line budget.** `menus-and-routing.js` is already on the line-budget allowlist; the RuView files target fewer than 900 lines (checked in `model.test.mjs`).

### Decisions that differ from the design doc (with reason)

1. **`stopped_at` field added.** §7 row 6 shows "Stopped by you at 14:02", and §8.1 has no time for the stop reason.
2. **Additional codes beyond the §8.4 table**: `docker_readonly` (403), `invalid_config` (400), `platform_unsupported` (409), `upstream_error`, `download_failed`, `start_failed`, `stop_failed`, `process_exited`, `source_mismatch`, `unexpected_simulation` (502), `storage_unreadable` (503), `artifact_missing` and `unknown` (500). They give every existing failure path a precise headline instead of `unknown`. `invalid_name` and `inventory_invalid` are deferred to P2, where names and inventory reset arrive.
3. **`Error` keeps two unexported fields** (`msg`, `cause`) next to `{Code, Detail, HTTP}`. This keeps the legacy `error` text byte-compatible (Config page, wizard and existing tests) and keeps `errors.Is(err, errInvalidReply)` working (`nodes.go:76`).
4. **`storage_unreadable` has the lowest `error_code` precedence and is a warning chip, not a row-11 headline.** A storage-walk error must not hide a real probe or lifecycle error, and must not turn a working service into a danger headline.
5. **Two extra rows.** `installing` (3a) shows native install progress; §8.1 lists `downloading` but §7 has no row for it. `external_check_old` (12a) applies when an external check is older than 10 s: Status never probes external services (D6), so rows 16/17 would claim "live" or "no new data for 5 min" from a check we never repeated. This replaces the "Last checked" overlay chip.
6. **"Open Live" (rows 13/16) is not shown in P1.** The model takes `views`, and without `live` it falls back to **Show boards** or no action. P3 enables it by adding `live` to `availableViews`.
7. **Row 12 secondary (service host) is omitted** because Status carries no URL. **Row 5 secondary** is a generic install sentence because Status does not know the server's platform or package size.
8. **i18n prefixes.** Runtime words use `state_<runtime>`, phases use `phase_<phase>`, and stop reasons use `stop_reason_<r>`, all enumerated by the Go test. Data-state words keep their existing literal-map keys, so nothing is duplicated.
9. **Ages use the existing `age_ms/s/min` keys (+ `age_h`) with `Intl.NumberFormat`,** rather than `Intl.RelativeTimeFormat`. These strings are already translated and tested in 16 locales.
10. **Boards is visible only for the `esp32` source.** The design's alternative, "or known boards exist", would need `/nodes` reads on Status for non-esp32 sources; P2 can widen it.
11. **Stop on a foreign `aurago-ruview` container returns success with a warning ("nothing of ours to stop").** Start reports `container_conflict` with the remedy. This removes the "Stop fails permanently" trap from M23.
12. **`waiting_ready` ends at the first ready probe or after 30 s.** This avoids a brief "can't be reached" flash right after a container start.
13. **Lab keeps today's frame lifecycle except mount-on-Open.** The licence footer links and unmount on hide/minimise remain P4. The planner-hint link in Details is text-only, because no desktop app with a verified id for operational issues was found.
14. **Copy diagnostics is language-neutral `key=value` text** rather than translated, so support can read it in any locale. It excludes the legacy `error` field.
15. **German copy.** §7 headlines are used verbatim, including the space before "…". New German sentences use capitalised "Du/Dir", matching the existing `ui/lang/ruview` strings; the manuals keep lowercase "du".
16. **The bundle rebuild happens in C, right after editing `menus-and-routing.js`,** so wave-2 tests see a consistent bundle. E reruns `build:ui` and `--check`.
