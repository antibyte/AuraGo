# Klipper 3D Printer Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add Klipper/Moonraker support to AuraGo's existing 3D printer integration using only standard Moonraker actions.

**Architecture:** Extend the existing `three_d_printer` tool with protocol routing so Elegoo SDCP and Klipper Moonraker printers share the same agent API. Keep media capture/proxy code common, and add protocol-specific camera URL resolution.

**Tech Stack:** Go `net/http`, existing AuraGo config/server/tool layers, vanilla JS config UI, JSON translation files.

---

### Task 1: Config Shape And Save Paths

**Files:**
- Modify: `internal/config/config_types.go`
- Modify: `internal/config/config.go`
- Modify: `internal/server/config_handlers_main.go`
- Modify: `internal/server/config_write_protection_test.go`
- Modify: `config_template.yaml`

- [ ] **Step 1: Write failing config test**

Add `TestConfigDeepMergeConvertsNumericKeyedKlipperPrinterMapToArray` in `internal/server/config_write_protection_test.go`. It should patch `three_d_printers.klipper.printers.0` and assert one parsed Klipper printer with ID `voron`.

- [ ] **Step 2: Run red test**

Run: `go test ./internal/server -run TestConfigDeepMergeConvertsNumericKeyedKlipperPrinterMapToArray -count=1`

Expected: FAIL because `Klipper` config and array path support do not exist yet.

- [ ] **Step 3: Implement config types/defaults/save path**

Add `KlipperPrinterConfig` and `KlipperConfig` beside the Elegoo types. Add `Klipper KlipperConfig` to `ThreeDPrintersConfig`. Default `Klipper.Enabled` to false. Include `three_d_printers.klipper.enabled` and `three_d_printers.klipper.printers` in config map export, and add `three_d_printers.klipper.printers` to `isConfigArrayPath`.

- [ ] **Step 4: Update template**

Add a commented Klipper example in `config_template.yaml` under `three_d_printers`, with `url`, `api_key`, `timeout_seconds`, and `webcam_name`.

- [ ] **Step 5: Run green test**

Run: `go test ./internal/server -run "TestConfigDeepMergeConvertsNumericKeyed.*PrinterMapToArray" -count=1`

Expected: PASS.

### Task 2: Klipper Tool Client And Routing

**Files:**
- Modify: `internal/tools/three_d_printer.go`
- Modify: `internal/tools/three_d_printer_test.go`
- Modify: `internal/agent/dispatch_platform.go`
- Modify: `internal/agent/native_tools_integrations.go`

- [ ] **Step 1: Write failing Klipper tool tests**

Add tests for:

- `status` sends `POST /printer/objects/query`.
- `files` sends `GET /server/files/list?root=gcodes`.
- `start_print` sends `POST /printer/print/start?filename=calibration.gcode`.
- read-only blocks `pause_print` before contacting Moonraker.
- `camera_url` selects the configured webcam by name.

- [ ] **Step 2: Run red tests**

Run: `go test ./internal/tools -run "Klipper|ThreeDPrinterExecuteBlocksKlipper" -count=1`

Expected: FAIL because Klipper routing and client functions are missing.

- [ ] **Step 3: Implement Klipper runtime types and resolver**

Add `KlipperPrinter`, `KlipperConfig`, and a resolved printer wrapper containing protocol, ID, name, URL, and either Elegoo or Klipper data. Update `list_printers` to include both protocol lists while avoiding API key output.

- [ ] **Step 4: Implement Moonraker HTTP client**

Implement standard endpoints only:

- `GET /server/info` for test connection.
- `POST /printer/objects/query` for status.
- `GET /printer/objects/list` for attributes.
- `GET /server/files/list?root=gcodes` for files.
- `GET /server/history/list?limit=20` for history.
- `POST /printer/print/start?filename=...`, `/pause`, `/resume`, `/cancel`.
- `GET /server/webcams/list` for camera URLs.

Use configured timeout, set `X-Api-Key` when an API key exists, parse JSON responses, and wrap non-2xx responses with status and sanitized body.

- [ ] **Step 5: Update agent runtime config**

Map `config.ThreeDPrinters.Klipper.Printers` into `tools.ThreeDPrinterConfig` in `internal/agent/dispatch_platform.go`. Update the native tool description to mention Klipper/Moonraker.

- [ ] **Step 6: Run green tool tests**

Run: `go test ./internal/tools ./internal/agent -run "Klipper|ThreeDPrinter|NativeTool" -count=1`

Expected: PASS.

### Task 3: Server Camera And Test Endpoint

**Files:**
- Modify: `internal/server/three_d_printer_handlers.go`
- Modify: `internal/server/three_d_printer_handlers_test.go`

- [ ] **Step 1: Write failing server tests**

Add tests proving:

- `/api/3d-printers/test` can test an ad hoc Klipper printer when request `protocol` is `klipper`.
- snapshot handler stores an image from a Klipper `snapshot_url`.
- stream handler accepts same host with different port.
- stream handler rejects an unrelated host.

- [ ] **Step 2: Run red tests**

Run: `go test ./internal/server -run "ThreeDPrinter.*Klipper|HandleThreeDPrinter.*Camera" -count=1`

Expected: FAIL because server handlers still assume Elegoo.

- [ ] **Step 3: Implement protocol-neutral camera helpers**

Add tool helpers that resolve camera stream/snapshot URLs for any configured printer, validate URL host against the printer base URL, and fetch snapshots from snapshot URL first with stream fallback.

- [ ] **Step 4: Update server handlers**

Use the protocol-neutral resolver for camera snapshot and stream endpoints. Extend the test endpoint to accept `protocol`, `api_key`, and `webcam_name`.

- [ ] **Step 5: Run green server tests**

Run: `go test ./internal/server -run "ThreeDPrinter|ConfigDeepMerge" -count=1`

Expected: PASS.

### Task 4: UI, Translations, Manual

**Files:**
- Modify: `ui/cfg/three_d_printers.js`
- Modify: `ui/lang/config/three_d_printers/*.json`
- Modify: `prompts/tools_manuals/three_d_printer.md`

- [ ] **Step 1: Write failing translation coverage expectation**

Run the existing UI/config tests after adding one temporary expected key locally in the test if needed. The intended key set includes Klipper labels/help for enable, printers title, URL, API key, webcam name, and add/test/remove buttons.

- [ ] **Step 2: Implement UI section**

Render a Klipper subsection with add/remove/test support. Use password input for `api_key`. Send `protocol: "klipper"` in test requests. Keep existing Elegoo behavior intact.

- [ ] **Step 3: Add all translations**

Update all 15 language files under `ui/lang/config/three_d_printers/` with real localized Klipper strings, not English copies.

- [ ] **Step 4: Update tool manual**

Document Klipper support, standard-only actions, read-only behavior, filename requirement, and camera fallback.

- [ ] **Step 5: Run UI/server smoke tests**

Run: `go test ./ui ./internal/server -run "Config|ThreeDPrinter" -count=1`

Expected: PASS.

### Task 5: Full Verification And Commit

**Files:**
- All modified files.

- [ ] **Step 1: Format Go code**

Run: `gofmt -w internal/tools/three_d_printer.go internal/tools/three_d_printer_test.go internal/server/three_d_printer_handlers.go internal/server/three_d_printer_handlers_test.go internal/server/config_write_protection_test.go internal/config/config_types.go internal/config/config.go internal/agent/dispatch_platform.go internal/agent/native_tools_integrations.go`

- [ ] **Step 2: Run package tests**

Run: `go test ./internal/tools ./internal/server ./internal/config ./internal/agent ./ui -count=1`

Expected: PASS.

- [ ] **Step 3: Secret scan**

Run: `rg -n "sk-or-|AURAGO_MASTER_KEY|password:|api_key: \"[^\"]+\"" --glob "!data/**" --glob "!reports/**" .`

Expected: no committed secrets; examples with empty `api_key: ""` are acceptable.

- [ ] **Step 4: Commit**

Commit all implementation changes with message `Add Klipper Moonraker 3D printer support`.
