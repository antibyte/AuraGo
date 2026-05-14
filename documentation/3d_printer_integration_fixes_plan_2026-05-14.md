# 3D Printer Integration Fixes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Resolve the relevant findings from `reports/3d_printer_integration_audit_2026-05-14.md` without widening the 3D printer integration beyond the current Elegoo SDCP and Klipper/Moonraker scope.

**Architecture:** Keep printer transport and request handling in `internal/tools`, keep agent-only concerns such as chat stream rendering, budget tracking, preferred MCP vision fallback, and logging in `internal/agent`, and keep HTTP-specific proxy behavior in `internal/server`. Shared configuration mapping and media storage helpers should live in `internal/tools` so agent and server paths cannot drift.

**Tech Stack:** Go standard library, `modernc.org/sqlite` via existing media registry helpers, `github.com/gorilla/websocket`, vanilla JS config UI, existing Go unit tests.

---

## Audit Relevance

| Audit item | Relevance | Notes |
|---|---:|---|
| Duplicate runtime config mapping | High | Confirmed in `internal/agent/dispatch_platform.go` and `internal/server/three_d_printer_handlers.go`. |
| Test endpoint ignores JSON decode errors | High | Confirmed in `handleThreeDPrinterTest`; malformed JSON currently falls through. |
| `analyze_camera` handled outside tool core | Medium | Confirmed, but moving all vision logic into `ExecuteThreeDPrinter` would pull agent concerns into the tool layer. Fix by extracting an explicit agent helper and documenting the boundary. |
| Hardcoded vision model | Medium | The actual API request uses `tools.AnalyzeImageWithPrompt`, which defaults to `google/gemini-2.0-flash-001`; the hardcoded `google/gemini-2.5-flash-lite-preview-09-2025` affects budget accounting fallback, not model selection. Still worth fixing via a shared default. |
| Unused `InternalStreamURL` / `ShowInChat` | Medium | `InternalStreamURL` is unused. `ShowInChat` is parsed but stream rendering currently happens whenever a broker exists. |
| Frigate-specific filename sanitizer | Medium | Confirmed naming leak; no functional bug. |
| Missing structured logs | Medium | Confirmed for `three_d_printer` dispatch. |
| Missing media registry registration | High | Confirmed: 3D printer snapshots write to disk but do not register like Frigate media. |
| Stream proxy context / timeout | Medium | The report's "use the 15s ctx for proxy" recommendation would break long-lived streams. Fix with a header/connect timeout and request-context cancellation, not a hard 15s stream lifetime. |
| DataDir validation | Low | `DataDir` is config-controlled, and the generated relative path is fixed. Add normalization and tests, but this is not a direct path traversal issue. |
| Missing operation tests | High | Confirmed for mutation operations and list/attributes/history coverage. |
| `findHTTPURL` too generic | Medium | Confirmed; replace with schema-first extraction and recursive fallback only if needed. |
| Infinite Elegoo response loop | Medium | Deadline prevents true infinite blocking, but a max unrelated-message count avoids noisy broadcast waits. |
| `boolAsInt` too local | Low | Local helper is fine unless reused. Prefer leaving it unless touching start-print constants. |
| Magic SDCP command numbers | Medium | Confirmed; constants improve readability and tests. |
| UI URL validation | Low | Useful user-facing polish, requires all 15 translation files if new strings are added. |
| Klipper API key in DOM | Low | Existing config UI pattern renders password values client-side. A real fix needs broader vault-backed config editing, not a narrow 3D printer patch. |

## Files

- Modify: `internal/tools/three_d_printer.go`
- Modify: `internal/tools/frigate.go`
- Modify: `internal/tools/three_d_printer_test.go`
- Modify: `internal/tools/frigate_test.go`
- Modify: `internal/server/three_d_printer_handlers.go`
- Modify: `internal/server/three_d_printer_handlers_test.go`
- Modify: `internal/agent/dispatch_platform.go`
- Modify: `internal/agent/dispatch_platform_test.go`
- Modify: `internal/agent/agent_dispatch_services.go`
- Modify: `internal/tools/vision.go`
- Modify: `ui/cfg/three_d_printers.js`
- Modify: `ui/lang/config/three_d_printers/*.json` only if UI validation introduces new visible strings.
- Modify: `prompts/tools_manuals/three_d_printer.md`

## Task 1: Centralize Runtime Config Mapping

**Files:**
- Modify: `internal/tools/three_d_printer.go`
- Modify: `internal/agent/dispatch_platform.go`
- Modify: `internal/server/three_d_printer_handlers.go`
- Test: existing package tests in `internal/agent`, `internal/server`, `internal/tools`

- [ ] Add `func BuildThreeDPrinterRuntimeConfig(cfg *config.Config) ThreeDPrinterConfig` to `internal/tools/three_d_printer.go`. The implementation should copy the exact mapping currently duplicated in agent and server, including `DataDir: cfg.Directories.DataDir`.
- [ ] Remove `buildRuntimeThreeDPrinterConfig` from `internal/agent/dispatch_platform.go`.
- [ ] Remove `buildThreeDPrinterRuntimeConfig` from `internal/server/three_d_printer_handlers.go`.
- [ ] Replace call sites with `tools.BuildThreeDPrinterRuntimeConfig(cfg)` and `tools.BuildThreeDPrinterRuntimeConfig(s.Cfg)`.
- [ ] Run: `go test ./internal/tools ./internal/agent ./internal/server -run "ThreeDPrinter|3D" -count=1`.
- [ ] Commit: `refactor: centralize 3d printer runtime config mapping`.

## Task 2: Harden Test Endpoint JSON Handling

**Files:**
- Modify: `internal/server/three_d_printer_handlers.go`
- Modify: `internal/server/three_d_printer_handlers_test.go`

- [ ] Add a failing test named `TestHandleThreeDPrinterTestRejectsMalformedJSON` that posts `{` to `/api/3d-printers/test` and expects HTTP 400 plus a JSON error message.
- [ ] In `handleThreeDPrinterTest`, replace `_ = json.NewDecoder(r.Body).Decode(&req)` with explicit error handling:

```go
if r.Body != nil {
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err != io.EOF {
		jsonError(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}
}
```

- [ ] Run: `go test ./internal/server -run ThreeDPrinter -count=1`.
- [ ] Commit: `fix: reject malformed 3d printer test payloads`.

## Task 3: Add Media Registry Support for Snapshots

**Files:**
- Modify: `internal/tools/three_d_printer.go`
- Modify: `internal/server/three_d_printer_handlers.go`
- Modify: `internal/agent/dispatch_platform.go`
- Modify: `internal/tools/three_d_printer_test.go`
- Modify: `internal/server/three_d_printer_handlers_test.go`

- [ ] Extend `ThreeDPrinterMediaResult` with `MediaID int64 json:"media_id,omitempty"`.
- [ ] Change `StoreThreeDPrinterMedia` signature to accept `mediaDB *sql.DB`, mirroring `StoreFrigateMedia`.
- [ ] Register stored snapshots with `RegisterMedia` when `mediaDB != nil`, using:

```go
MediaItem{
	MediaType:   "image",
	SourceTool:  "three_d_printer",
	Filename:    filename,
	FilePath:    localPath,
	WebPath:     webPath,
	FileSize:    int64(len(data)),
	Format:      strings.TrimPrefix(ext, "."),
	Provider:    "three_d_printer",
	Description: fmt.Sprintf("3D printer snapshot for %s", printerID),
	Tags:        []string{"3d_printer", "snapshot", printerID},
	Hash:        hash,
}
```

- [ ] Pass `dc.MediaRegistryDB` from agent snapshot and analyze paths.
- [ ] Pass `s.MediaRegistryDB` from server snapshot handler.
- [ ] Add a tool unit test using `InitMediaRegistryDB` and assert `MediaID > 0`.
- [ ] Run: `go test ./internal/tools ./internal/server ./internal/agent -run "ThreeDPrinter|Media" -count=1`.
- [ ] Commit: `feat: register 3d printer snapshots in media registry`.

## Task 4: Make Snapshot Storage Safer and Cleaner

**Files:**
- Modify: `internal/tools/frigate.go`
- Modify: `internal/tools/three_d_printer.go`
- Modify: `internal/tools/three_d_printer_test.go`
- Modify: `internal/tools/frigate_test.go`

- [ ] Rename `sanitizeFrigateFileToken` to `sanitizeMediaFileToken` in `internal/tools/frigate.go`.
- [ ] Update Frigate and 3D printer call sites to use the generic name.
- [ ] In `StoreThreeDPrinterMedia`, normalize `dataDir` with `filepath.Abs` and `filepath.Clean` before joining the fixed relative media directory.
- [ ] Add tests for empty data, empty data dir, and unsafe printer IDs such as `door/../../event 1`.
- [ ] Run: `go test ./internal/tools -run "ThreeDPrinter|Frigate" -count=1`.
- [ ] Commit: `chore: share media filename sanitizer`.

## Task 5: Clarify `analyze_camera` Boundary and Vision Budget Model

**Files:**
- Modify: `internal/agent/dispatch_platform.go`
- Modify: `internal/agent/dispatch_platform_test.go`
- Modify: `internal/agent/agent_dispatch_services.go`
- Modify: `internal/tools/vision.go`
- Modify: `prompts/tools_manuals/three_d_printer.md`

- [ ] Add an exported constant in `internal/tools/vision.go`:

```go
const DefaultVisionModel = "google/gemini-2.0-flash-001"
```

- [ ] Use `DefaultVisionModel` in `AnalyzeImageWithPrompt`.
- [ ] Use `tools.DefaultVisionModel` for budget fallback in both `agent_dispatch_services.go` and `dispatch_platform.go`.
- [ ] Extract `handleThreeDPrinterAnalyzeCamera(...)` in `dispatch_platform.go` so the dispatch switch no longer contains the full snapshot-analysis flow inline.
- [ ] Add a test that stubs `dispatchAnalyzeImageWithPrompt`, calls the 3D printer dispatch path with `analyze_camera`, and verifies the budget model fallback matches `tools.DefaultVisionModel`.
- [ ] Update `prompts/tools_manuals/three_d_printer.md` to state that `analyze_camera` captures a snapshot, stores/registers it, and then runs the configured Vision provider.
- [ ] Run: `go test ./internal/agent ./internal/tools -run "Vision|ThreeDPrinter" -count=1`.
- [ ] Commit: `fix: align 3d printer camera analysis with vision defaults`.

## Task 6: Inline Live Stream Rendering

**Files:**
- Modify: `internal/tools/three_d_printer.go`
- Modify: `internal/agent/dispatch_platform.go`
- Modify: `internal/agent/dispatch_platform_test.go`

- [x] Remove `InternalStreamURL` from `ThreeDPrinterRequest`.
- [x] Treat `show_live_stream` as the explicit inline-rendering operation when a chat broker is available; keep `camera_url` for URL-only requests.
- [x] Add a dispatch test that `show_live_stream` sends the same-origin proxied stream.
- [x] Run: `go test ./internal/agent ./internal/tools -run ThreeDPrinter -count=1`.
- [x] Commit: `fix: render 3d printer live streams inline`.

## Task 7: Add Structured Logging for 3D Printer Operations

**Files:**
- Modify: `internal/agent/dispatch_platform.go`

- [ ] Add `logger.Info("LLM requested 3D printer operation", "operation", req.Operation, "printer_id", req.PrinterID)` before operation execution.
- [ ] For mutation operations, include `"filename"` for `start_print` and avoid logging API keys or raw URLs.
- [ ] Run: `go test ./internal/agent -run ThreeDPrinter -count=1`.
- [ ] Commit: `chore: log 3d printer tool operations`.

## Task 8: Replace Magic SDCP Command Numbers

**Files:**
- Modify: `internal/tools/three_d_printer.go`
- Modify: `internal/tools/three_d_printer_test.go`
- Modify: `internal/server/three_d_printer_handlers_test.go`

- [ ] Add constants near the top of `three_d_printer.go`:

```go
const (
	sdcpCmdStatus         = 0
	sdcpCmdAttributes     = 1
	sdcpCmdStartPrint     = 128
	sdcpCmdPausePrint     = 129
	sdcpCmdCancelPrint    = 130
	sdcpCmdResumePrint    = 131
	sdcpCmdFiles          = 258
	sdcpCmdHistory        = 320
	sdcpCmdCameraURL      = 386
	sdcpCmdCameraLight    = 403
)
```

- [ ] Replace numeric command literals in implementation and tests.
- [ ] Run: `go test ./internal/tools ./internal/server -run ThreeDPrinter -count=1`.
- [ ] Commit: `refactor: name sdcp 3d printer commands`.

## Task 9: Broaden Operation Test Coverage

**Files:**
- Modify: `internal/tools/three_d_printer_test.go`

- [ ] Add table-driven Klipper tests for `pause_print`, `resume_print`, `cancel_print`, `attributes`, `history`, and `list_printers`.
- [ ] Add table-driven Elegoo tests for `pause_print`, `resume_print`, `cancel_print`, `set_camera_light`, `attributes`, `history`, `files` with custom directory, and `list_printers`.
- [ ] Each mutation test must assert the exact HTTP path or SDCP command.
- [ ] Run: `go test ./internal/tools -run ThreeDPrinter -count=1`.
- [ ] Commit: `test: cover 3d printer command operations`.

## Task 10: Tighten Camera URL Extraction and Stream Proxy Behavior

**Files:**
- Modify: `internal/tools/three_d_printer.go`
- Modify: `internal/tools/three_d_printer_test.go`
- Modify: `internal/server/three_d_printer_handlers.go`
- Modify: `internal/server/three_d_printer_handlers_test.go`

- [ ] Replace `findHTTPURL` use in `ElegooCentauriCarbonCameraURL` with schema-first extraction from `Data.Data.Url`, `Data.Url`, and `Url`; keep recursive fallback only after those fail.
- [ ] Add tests where an unrelated HTTP URL appears before the camera URL and assert the camera URL wins.
- [ ] Add a max unrelated-message counter to the Elegoo read loop, returning an error after 50 non-matching responses.
- [ ] For stream proxy, keep `r.Context()` for cancellation but replace `&http.Client{Timeout: 0}` with a package-level client using a transport with `ResponseHeaderTimeout` and dial/TLS timeouts. Do not use a total timeout that terminates healthy live streams.
- [ ] Run: `go test ./internal/tools ./internal/server -run ThreeDPrinter -count=1`.
- [ ] Commit: `fix: tighten 3d printer camera URL and stream handling`.

## Task 11: Add UI URL Validation Without New Translation Debt

**Files:**
- Modify: `ui/cfg/three_d_printers.js`

- [ ] Before `threeDPrinterTest` posts to the backend, validate Elegoo URLs with `ws:` or `wss:` and Klipper URLs with `http:` or `https:` using `new URL(value)`.
- [ ] Reuse existing `config.three_d_printers.test_failed` text for invalid input to avoid adding 15 translation keys in this small fix slice.
- [ ] Run a lightweight syntax check if available, otherwise inspect manually and run the existing frontend build/test command if the repo has one.
- [ ] Commit: `fix: validate 3d printer test URLs in config UI`.

## Final Verification

- [ ] Run: `go test ./internal/tools ./internal/server ./internal/agent -run "ThreeDPrinter|Vision|Frigate" -count=1`.
- [ ] Run: `go test ./internal/config/... ./internal/tools/... ./internal/server/... ./internal/agent/... -count=1`.
- [ ] Run: `git diff --cached` before the final commit to verify no secrets or runtime files are staged.
- [ ] Commit any remaining documentation/manual updates with `docs: update 3d printer tool documentation`.

## Deferred Items

- Klipper API key rendering in the config UI should be handled as part of a broader vault-backed config editor pass, not inside this 3D printer patch.
- Snapshot retention/rate limiting needs a product decision: age-based cleanup, per-printer rate limits, or both. Plan separately if media volume becomes a real problem.
- Moving `analyze_camera` fully into `ExecuteThreeDPrinter` is intentionally deferred because it would mix tool transport with agent-only budget/MCP/streaming concerns.
