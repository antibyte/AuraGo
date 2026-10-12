# CYD Creator domain

## Purpose

`internal/cydcreator` owns the CYD Creator domain: board profiles, device themes, the widget catalog, the
`cyd.project.json` model with validation and layout patch ops, partition tables, the `cydcc` config blob and the
build diagnostics parser, plus (plan 1b) the embedded device template `cyd_template`, the reference generated
project and, outside the package, the builder image (`deploy/docker/Dockerfile.cyd-builder`,
`deploy/docker/cyd-builder/`, `.github/workflows/cyd-builder.yml`), plus (plan 1c) the deterministic generator that
turns a project into the complete PlatformIO tree, plus (plan 2) the SQLite store, the service (projects, layout,
app files, assets, events) and the build queue, plus (plan 3) agent jobs with working copies, drafts, the crash-safe
publication, revisions, the job chat and the agent tool implementations. Plan 5 adds devices to this same package.

## Ownership

- `errors.go` (`Error`, one `Code*` constant per CYD_* code, `ErrorCodes()`), `ids.go`, `limits.go`.
- `boards.go` + `boards/<id>.json`, `themes.go` + `themes.json`, `catalog.go` (widgets, fonts, symbols, curated
  libraries).
- `project.go`, `validate.go`, `patch.go`: layout model, validation issues and `ApplyOps`.
- `partitions.go`, `configblob.go`, `diagnostics.go` (`ParseDiagnostics`, `IsGeneratedPath`, `IsEditablePath`).
- `testdata/diagnostics/*.txt`: PlatformIO/gcc/ld log fixtures (`.txt`, because `*.log` is git-ignored).
  `.gitattributes` keeps `testdata/**` and the template/builder paths LF on every platform.
- `template.go` + `template/`: `TemplateVersion`, `DeviceLanguages`, `TemplateFS()`, `TemplateStrings(lang)`, device
  strings (`template/strings.json` is the source; `config/cyd_strings.h` is rendered from it), the C++ runtime
  `template/lib/cyd_template/**` and the app scaffold `template/scaffold/src/app/app_main.cpp`.
- `testdata/reference/basic/`: hand-written generator output for every board profile (golden target of the
  generator, prewarm and smoke project of the builder image).
- `deploy/docker/Dockerfile.cyd-builder`, `deploy/docker/cyd-builder/{cyd-build,cyd-addr2line,cyd-selftest,requirements.*}`
  and `.github/workflows/cyd-builder.yml`.
- `generate*.go` (plan 1c): `Generate`, `InspectAsset`, `ScaffoldAppFiles` and the renderers of
  `include/board_config.h`, `platformio.ini` and `src/generated/*` (`partitions.csv` comes from `PartitionCSV` in
  `partitions.go`). `testdata/golden/<case>/` holds generator inputs (`cyd.project.json`) with their expected
  output. `generate_builder_it_test.go` is the opt-in compile proof (Docker and the builder image).
- Plan 2: `storage.go` (SQLite schema v1 with every table, migrations, startup recovery), `builder.go` (the
  `Builder` interface and build record types), `service.go` (lifecycle, limits, locking rule, default service),
  `service_projects.go`, `service_layout.go`, `service_generate.go`, `service_capabilities.go`, `files_root.go`,
  `files.go`, `assets.go`, `events.go`, `builds.go`, `builds_log.go`. The server owns the wiring, the Docker adapter
  and the HTTP API (`internal/server/AGENTS.md`, "CYD Creator").
- Plan 3: `jobs.go` (job model, phases, plan validation, rows, lock setter, `job_step`), `jobs_start.go`
  (`StartJob`, resume, `ConfirmJob`, `CancelJob`, `DiscardDraft`), `jobs_run.go` (phase state machine, builds,
  publication, failure and draft handling), `jobs_recover.go` (startup recovery), `jobs_agent_context.go` (private
  conversation), `job_context.go`, `runner.go` (`Runner`, `JobRun`, job runtime, timeout), `messages.go`, `revisions.go` +
  `revisions_diff.go` (blob store, revisions, diff, restore), `staging.go` + `staging_publish.go` (working copies,
  receipts, fence), `tools_project.go`, `tools_files.go`, `tools_build.go`, `tools_reference.go` (agent tool
  implementations), `texts.go` (translation codes and English templates of job steps and system chat lines). The agent schemas and dispatcher live in `internal/agent` (`internal/agent/AGENTS.md`), the
  runner, prompt profiles and job routes in `internal/server`.

## Local Contracts

### Package rules

- The 1a files are pure: stdlib only, never import `internal/server`, `internal/agent`, `internal/tools` or Docker
  code (`TestPackageLayering` checks direct imports). Every CYD Creator plan shares this one package: search for an
  identifier before declaring a helper, use the `Code*` and `Issue*` constants instead of string literals and never
  redeclare them.
- `Boards()`, `Themes()`, `WidgetCatalog()`, `WidgetSpecFor()`, `Libraries()` and `ThemesJSON()` return copies.
  `FontFamilies`, `FontSizes` and `Symbols` are package variables the validator reads: treat them as read-only.
  The validator shares one cached `WidgetSpec` per type (`widgetSpecRef`); never write to its `Events`, `Options`
  or `[]string` defaults.
- `ThemesJSON()` returns nil when the embedded file is broken: an HTTP handler must answer 500, never an empty 200.
- Bounded text: `clipReason` (1024 B, rune boundary) is the one clip for error reasons and diagnostic messages,
  `clipEcho` (64 B) for user text repeated in issue messages. Issue `Path`, `Screen` and `Widget` are never clipped.

### Boards

- Board profiles are strict JSON (unknown keys and trailing data fail), the file name equals the id, and
  `Boards()` loads them once and returns deep copies. A broken profile fails `TestEveryEmbeddedBoardLoads`, never
  the running server. `display.width/height` describe the board's designed orientation, not the controller's RAM
  orientation; `display.offset_rotation` (LovyanGFX semantics, 0-7) turns the controller frame into that
  orientation (classic CYD: 5) and `touch.offset_rotation` corrects capacitive touch layers (XPT2046: always 0).
- Drivers and their bus: display `ILI9341`, `ILI9341_2`, `ST7789`, `ST7796` (spi), `NV3041A` (qspi), `ST7262`,
  `EK9716` (rgb); touch `XPT2046` (spi), `GT911`, `CST820` (i2c). `ILI9341_2` is the ILI9341 with the alternative
  init sequence (TFT_eSPI `ILI9341_2_DRIVER`, LovyanGFX `Panel_ILI9341_2`): same pins, same 240x320 controller RAM.
  The generator emits `CYD_PANEL_DRIVER_<driver>` verbatim, so a new driver needs a case in `cyd_board.cpp`
  (`TestRenderBoardConfigPanelDriverMacro` checks both).
- Pin keys per bus: display spi `sclk mosi cs dc` (optional `miso rst bl`), qspi `cs sclk d0 d1 d2 d3` (optional
  `rst bl`), rgb `de vsync hsync pclk r0..r4 g0..g5 b0..b4` (optional `bl disp`) plus `timing` (`pclk_hz`,
  `hsync_polarity hsync_front_porch hsync_pulse_width hsync_back_porch`, the same four for vsync,
  `pclk_active_neg`; optional `de_idle_high pclk_idle_high`); touch spi `sclk mosi miso cs` (optional `irq`), i2c
  `sda scl` (optional `int rst`). `-1` marks an unconnected optional pin.
- `validateBoard` rules a new profile must satisfy:
  - A GPIO may be used once. The only reuse is a touch controller on the display's SPI bus, and it must share
    `sclk`, `mosi` and `miso` with the display in the same role, all three or none of them.
  - Pins exist on the chip and avoid flash/PSRAM (ESP32 GPIO 6-11; S3 GPIO 26-32, and 33-37 when
    `arduino.memory_type` contains `opi`); input-only pins (ESP32 34-39) only for input roles; the LDR needs an ADC1
    pin (ESP32 32-39, S3 1-10) because ADC2 is unusable while Wi-Fi runs.
  - RGB panels need an esp32s3 with PSRAM, `display.hz` 0 and a complete `timing` (`pclk_hz` 1..40 MHz, flags 0/1,
    porch and pulse widths 0..127); spi/qspi need `hz` 1..80 MHz, qspi needs an esp32s3.
  - `pio.build_flags` are `-DNAME[=value]` or `-m<option>` without quotes or spaces. `pio.board_build` may only set
    `arduino.memory_type`, `flash_mode`, `f_flash` and `f_cpu` (the generator owns partitions, filesystem and the
    rest); an esp32s3 with PSRAM must set `arduino.memory_type`.
  - XPT2046 needs a calibration with `0 <= min < max <= 4095`; GT911/CST820 carry none and need a valid `i2c_addr`.
  - Boards without PSRAM keep LVGL draw buffers at or below 48 KiB.
- `esp32-2432s028r` is the only verified profile. Pins follow witnessmenow/ESP32-Cheap-Yellow-Display;
  `invert: true` and the XPT2046 calibration 290/3670/230/3860 (XPT2046_Touchscreen rotation 1, landscape) come
  from the hardware-tested `agocyd` firmware. 24 draw-buffer lines are 1/10 of the screen (15 KiB) and leave heap
  for Wi-Fi, TLS and WiFiManager. Promote a community profile to `verified` only after a hardware test.
  Hardware finding (2026-10-10, ESP32-2432S028R with ESP32-D0WD-V3): with `Panel_ILI9341` the panel updated only
  partly (stale content and garbage on the right); `ILI9341_2` (the init `agocyd` uses through TFT_eSPI
  `ILI9341_2_DRIVER`) with `offset_rotation: 5`, `invert: true` and 40 MHz gives a correct picture, so the profile
  uses `ILI9341_2`. The community `esp32-2432s024r/c` profiles stay on `ILI9341` until someone tests them.
- `themes.json` is the only source of theme values (colors uppercase `#RRGGBB`); `ThemesJSON()` is the compact
  API form. Library `PIOSpec` and every `Depends` entry pin exact versions; the builder image and exported
  `platformio.ini` use the identical strings.

### Project model and validation

- `ParseProject` is strict and fails with `CYD_INVALID_LAYOUT` plus `details.reason`: at most 1 MiB (a leading BOM
  is stripped), a shape pre-scan before decoding (nesting at most 32 levels, no array with more than 4096
  elements, at most 32768 objects), no unknown fields, no trailing data, format 1, well-formed screen/widget/asset ids.
  Semantic problems are `Validate` issues, never parse errors.
- `MarshalProject` is canonical: struct field order, sorted map keys, 2-space indent, no HTML escaping, trailing
  newline, nil slices as `[]`. It also fails with `CYD_INVALID_LAYOUT` (`details.limit`, `details.bytes`) when the
  canonical form exceeds `MaxLayoutBytes`, because such a layout could not be parsed back. Storage, jobs, imports
  and `ApplyOps` callers must handle that error and map it to the agent as a typed error, never to an internal one.
- `WalkWidgets` hands out pointers that alias the caller's slices; deep-copy before mutating (`ApplyOps` does).
- Widget names are C identifiers (`^[a-z][a-z0-9_]{0,31}$`), unique per project, and must not collide with
  generated symbols: the names `init goto back theme_colors event_index async screen_t transition_t`, the prefixes
  `scr_ show_ hide_ font_`, the suffix `_series` and `img_<asset id>` of an existing asset are reserved
  (`NAME_INVALID`). The designer's JS `RESERVED_NAMES` must mirror this list; the `Validate` issue stays
  authoritative. Asset ids may only use letters, digits and `_` (they become `ui_img_<id>`; `PROP_INVALID` at
  `assets/<n>/id`). Generator-private symbols use `ui__` or `cyd_`, which no valid name produces. Free text
  (project name, timezone, titles, props, hints) must be escaped wherever it reaches C code.
- `Validate` covers every code in `IssueCodes()` in deterministic order with English messages free of indexes.
  - A widget placed on a screen has depth 1 (maximum 4); tabview children are bounded by the content area (size
    minus `bar_size` on the bar side). Geometry is checked with comparisons, not `abs`, so huge values cannot
    overflow: width/height 1..2048, `|x|` and `|y|` at most 4096 (`SIZE_INVALID`).
  - The project `name` (1..64 runes, no control characters, valid UTF-8) and `timezone` (1..64 printable ASCII,
    never empty) are `PROP_INVALID` at paths `name` and `timezone`. `NewProject` defaults the timezone to `UTC0`;
    project settings and imports must keep it non-empty.
  - `orientation` is one of the four names (`ORIENTATION_INVALID`). A board with an NV3041A display
    (`jc4827w543`) cannot rotate by register, so it only accepts its native orientation family: portrait on a
    landscape-native board is `ORIENTATION_INVALID` ("this panel supports landscape only"). The template's
    `cyd_board.cpp` has the matching `#error` on the internal rotation.
  - `UNKNOWN_PROP` lists at most 20 props per widget plus one summary issue at `<widget path>/props`.
  - A `msgbox` needs at least one button (`PROP_INVALID` at `<widget path>/props/buttons`): LVGL creates the footer
    with the first button, so a box without one could never be closed. `ApplyOps` refuses an op that empties the
    list, like any other new error.
  - The asset budget counts every listed asset; `AssetSizes == nil` skips it.
- `ApplyOps` is atomic and never modifies its input (it works on a deep copy). It accepts at most 200 ops, each
  checked by the gate: an op fails when it is structurally impossible or introduces a validation error whose key
  the layout did not have before. The key is code + widget id (screen id for screen issues, nothing for project
  issues) + the path below that element, so reindexing, renames and moving a widget to another screen do not
  create new keys. Three families are keyed differently because their attribution is unstable:
  `NAME_DUPLICATE`, `ID_DUPLICATE` and `SCREEN_NAME_DUPLICATE` by code + the duplicated name or id,
  `UNKNOWN_PROP` by code + widget (the 20-issue cap shifts which props are listed).
  - After all ops the result must marshal within `MaxLayoutBytes` and pass the shape check, else the call fails
    with `CYD_INVALID_LAYOUT` at the last op (`details.limit`, `details.bytes`). A widget list added by one op that
    is nested deeper than 4 levels on its own or holds more than 200 widgets is rejected before it is cloned
    (structural, no `code`).
  - Failure details are bounded (reason and path at most 1 KiB, the op name at most 64 B); never echo more.
  - Added widgets and screens always get fresh ids and catalog default sizes (statusbar: parent width);
    `goto_screen` targets may be names and are stored as ids. `Set` merges top-level fields, `props` and `style` one
    level deep, `null` deletes, `events` are replaced. `move_widget` without screen and parent only changes x/y; with
    a screen and no parent it moves to that screen's root. The designer's JS model mirrors these semantics.

### Partitions and config blob

- Partition tables are exactly spec section 8. The data partition `littlefs` uses subtype `spiffs`, so the template
  passes the label `littlefs` to `LittleFS.begin`. `cydcc` is data subtype 0x41 at 0x3D0000/0x610000/0xC90000.
- The config blob is `AGCC`, version byte 1, reserved byte 0, little-endian uint16 length at bytes 6-7, compact
  JSON `{url, device_id, token, cert_sha256}` from byte 8, zero padding to 4096; server and firmware accept at
  most 4088 payload bytes.
- `DeviceConfig.validate` is firmware-compatible and strict for encode and decode alike: the URL starts with the
  literal lowercase `http://` or `https://`, is printable ASCII without quotes or backslashes, has no userinfo,
  query, fragment or IPv6 literal, a non-empty host name and a port of 1..65535; `https` requires a 64-hex
  lowercase `cert_sha256`, `http` must not carry one; the token matches `[A-Za-z0-9._~+/=-]+`. Decoding also
  needs exactly the four keys (case-sensitive, once each, all strings), no trailing data and zero padding.
  Violations are `CYD_BAD_REQUEST`; an IPv6 literal host is one of them, so a device URL needs a hostname or an
  IPv4 address.
- `DeviceConfig` redacts `Token` in `String`, `GoString` and `LogValue`; only the JSON blob carries it. Never log it.

### Diagnostics and paths

- `ParseDiagnostics` makes paths under the work root project-relative, drops warnings and notes outside the
  project or in `.pio/`, keeps errors anywhere, maps linker `undefined reference`/`multiple definition` lines to
  file-less errors, skips indented lines (gcc and ld start every diagnostic in column 0; excerpts and `from`
  continuations are indented), clips `File` (a `#line` directive can name any path) and `Message` with
  `clipReason`, deduplicates and returns at most 200 entries in log order. The cap is linear: the first error gets
  the budget first, then the first 16 cyd-build library notes (later ones rank as other notes; under the cap all
  stay), the other errors, warnings and other notes, and inside a class the earliest entries win.
  `capDiagnostics` and `OrderAgentDiagnostics` share these classes (`classifyDiagnostics`). A library note is
  only cyd-build's exact sentence (`libraryNotePattern`, pinned to `cyd-build` by
  `TestLibraryHintMatchesCydBuild`), never a `#pragma message` note. `Generated` is decided from the
  full path before clipping; `Generated` diagnostics are generator bugs (`CYD_GENERATOR_ERROR`) and never go to the
  agent for repair.
- `IsGeneratedPath` classifies `src/generated/**`, `lib/cyd_template/**`, `include/**`, `platformio.ini` and
  `partitions.csv`. `IsEditablePath` is the single write gate for `src/app/**` (file API and agent tools): the
  `src/app/` prefix is case-sensitive, the extension `.cpp`, `.h`, `.hpp` or `.c`, at most four directory levels
  below `src/app`, at most 160 bytes, no `..`, backslashes or `:`. Every segment is ASCII, 1..64 bytes of
  `[A-Za-z0-9_.-]`, starts with a letter, digit or `_`, does not end with `.` (Windows drops trailing dots, so `net.`
  would alias `net`) and is no Windows device name (`CON PRN AUX NUL COM0-9 LPT0-9`, compared case-insensitively on
  the part before the first dot, so `aux/x.cpp` and `con.h` are rejected while `console.cpp` and `com10.cpp` pass).
  Generated paths are never writable.

### Device template

- `cyd_template` is configured only by `include/board_config.h` (the macro contract is the reference
  `testdata/reference/basic/profiles/<board>/include/board_config.h`; every macro is always emitted) and the
  platformio.ini contract (`-Iinclude -Ilib/cyd_template/config -Isrc/generated`,
  `-DLV_CONF_INCLUDE_SIMPLE`, `-DWM_STRINGS_FILE=\"cyd_wm_strings.h\"`, `-DWM_NODEBUG`). LVGL is called only from
  the loop task; other tasks use `cyd::ui_async`. `library.json` keeps `libArchive: false` (strong
  `verifyRollbackLater()` and weak-hook overrides) and no dependencies (builds are offline).
- A change to any template file is a template version bump (`TemplateVersion`, `library.json`); projects are never
  migrated automatically. Device strings: edit `template/strings.json`, then
  `CYD_UPDATE_STRINGS=1 go test ./internal/cydcreator/ -run TestTemplateStringsHeaderUpToDate`.
- Setup screen (`cyd_wifi_ui.cpp`) layout, for every board, orientation, theme and language: only the network list
  scrolls (vertically); every label in a container has a width and a long mode (wrap for prose and button captions,
  dots for titles and SSIDs). The phone view shrinks the QR code (96..220 px) and then drops the hint until it
  fits; the list view has rescan as the header's refresh button and "hidden network" as the last list entry; the
  password view shows the SSID as the title, gives the keyboard at most half the screen (less when two fields need
  the room) and ends the content at its top edge; the status replaces the keyboard while connecting and after a
  failure, until a field is tapped; leaving the password view cancels a running attempt and restarts the portal
  first. `layout_check()` (`cyd_layout.cpp`, always compiled) logs `layout: <view> ...` (at most five lines plus
  `<n> more`) after every view change. Test builds with `#define CYD_LAYOUT_SELFTEST 1` (layout and reconnect;
  boots without joining the stored network), `2` (layout only) or `3` (layout and a 50-cycle soak with heap
  numbers) in `include/board_config.h` run the on-device self-test (`cyd_layout_selftest.cpp`, compiled only then)
  a few seconds after boot and print `layout-selftest: <view> PASS|FAIL <n>`, `layout-selftest: PASS|FAIL <n>`,
  `layout-selftest: reconnect PASS|FAIL <ms>` and `layout-selftest: soak ...`; the generator never sets it.
- Setup portal: the SoftAP `CYD-Setup-XXXX` is WPA2 with a fresh random 8-character password. WiFiManager's menu is
  `wifi` only, and its admin routes (`/update /u /erase /restart /info /param /paramsave /exit /close`) answer 404:
  `block_admin_routes` registers them from WiFiManager's web-server callback, before WiFiManager's own routes, and the
  first matching handler wins. Keep that list in step with the WiFiManager version pin.
- Passwords: a WPA network needs 8..63 characters or 64 hex digits; a network typed by hand (hidden) may also have
  none, an open one never asks. The portal's head element carries a small script (`kPortalScript` in
  `cyd_portal.cpp`, a raw string the Go tests extract and run under Node with a fake DOM; keep it below 1.75 KiB,
  WiFiManager sends it with every page; `portal_stop()` frees it) that blocks the save form and shows `WIFI_PASSWORD_LENGTH` when this rule
  fails; it reads the lock from WiFiManager's list markup (class `l` on the signal icon after `a[data-ssid]`), so
  re-check it on a WiFiManager bump. The same script shows a scanning overlay (`#cydsc`, `WIFI_SCANNING`, a CSS
  spinner from the theme, static with `prefers-reduced-motion`) as soon as a form goes to `/wifi` (menu, refresh),
  because WiFiManager answers only after a blocking scan (`role=status`); it never blocks the navigation, and a
  tap, 30 s or `pageshow` after a back-forward-cache restore hide it. WiFiManager's own 30 s scan cache stays as it is; its scan preload stays off,
  it would race the device's async list scans for the driver's single scan buffer. The device applies the same
  rule before joining (`password_ok()`, lock flags remembered from the scan); an open listed network joins right
  away without the keyboard. Page (TextEncoder, or a fallback that counts an unpaired surrogate as U+FFFD), device and driver count UTF-8 bytes; WEP keys (5 or 13 characters) are
  rejected on purpose (obsolete).
- Portal shutdown and joining (hardware-verified 2026-10-10): never call WiFiManager's `stopConfigPortal()`. Its
  `shutdownConfigPortal()` rewrites the SoftAP config (empty, channel 6), blocks for a second, forces STA mode and may
  call `WiFi.reconnect()`. `portal_stop()` uses `CydWiFiManager::stop_quietly()` (frees web and DNS server through
  protected members of the pinned 2.0.17; re-check on a version bump) and `WiFi.mode(WIFI_STA)`, which keeps an
  associated station associated; `setDisableConfigPortal(false)` keeps the phone path from running the shutdown
  inside `process()`. The on-device path stops the portal before `WiFi.begin()` (one radio channel: next to the
  SoftAP the first join timed out), skips the join when the station is already on that SSID with that password,
  counts success only for a `STA_GOT_IP` after its `begin()` (`wifi_ip_generation()`), gives up on
  `NO_AP_FOUND` (`wifi_last_fail_reason()`; twice for a network the scan listed, once for a typed SSID), after 3
  failed joins (`wifi_fail_generation()`), on `WL_CONNECT_FAILED` or after `CYD_WIFI_CONNECT_MS`. Ending or leaving an attempt disconnects with the driver's
  auto-reconnect off and restarts the portal, which turns auto-reconnect back on (WiFiManager's phone-path join
  tries once and relies on the driver's retry). Closing after a success never starts a new join
  (`wifi_await_link()`). A phone-path join runs inside the `process()` call that handled the save; when such a
  save (WiFiManager's pre-save callback, non-empty SSID) does not end connected, `portal_loop()` returns
  `PortalEvent::JoinFailed`, disconnects the station (the driver would keep retrying the saved credentials off
  the SoftAP's channel) and the phone view shows `WIFI_FAILED` until the next attempt; page requests that only
  scan never touch the station link. WiFiManager keeps the portal up and shows the reason on its status line. Scans are polled for up to
  15 s (2.0.17 `scanComplete()` reports a failure after 6 s, which scans next to the SoftAP exceed), and
  `start_scan()` never starts a second scan while one runs (a rebuilt list keeps waiting).
- Token origin: the AuraGo bearer token is sent only to `g_cfg.url + path` (the `cydcc` URL; https is pinned by
  `cert_sha256` before any token leaves the device). An update offer's `url` is honoured only when empty, a `/path`
  or absolute below the configured URL (at most 256 bytes, visible ASCII 0x21-0x7E, not starting with `//`); anything
  else falls back to `/api/cyd-creator/device/firmware/<build_id>`. The offered `sha256` is lowercased and must be 64
  hex, `build_id` must match `[A-Za-z0-9._-]{1,64}`, and heartbeat/ota-result answers are capped at 4 KiB. The server
  must therefore send `update.url` empty or as a `/path`. App code gets a separate worker (`cyd_http.cpp`) whose https requests are
  not certificate-verified: never route secrets through it.
- Rollback has two guards. The RTC boot counter counts only crash resets (`PANIC`, `INT_WDT`, `TASK_WDT`, `WDT`) and
  is cleared on power-on, after the first successful heartbeat or after 120 s of uptime. At 3 unstable boots the other
  slot is booted only if (a) it holds an app that is not marked INVALID/ABORTED, (b) its ELF SHA-256 equals `prev_elf`
  and (c) the running image's ELF SHA-256 equals `ota_new_elf`; both are written by `rollback_remember_update` right
  before an OTA switch. So a rollback only ever returns an image this device installed by OTA to the image it came
  from, never to a stale build of another project left in the slot after a USB flash. The bootloader rollback
  (`verifyRollbackLater()`) is the second guard for an OTA image that resets before it proves stable.
- Touch calibration (XPT2046) is stored with a `board_key` (FNV-1a of board id, panel offset rotation and native
  size), so a different board or profile offers a fresh calibration instead of using a foreign one. The flow
  gives up without storing after 60 s idle or a touch held 10 s, uses a relative pivot, and rejects a fit with a
  residual above 10 px (retry).
- NVS (`cyd_storage.cpp`): reads probe the namespace with the IDF `nvs_open(ns, NVS_READONLY)` before
  `Preferences::begin`, because Preferences logs a missing namespace (every lookup on a fresh device) as an error;
  `get_float` and key removal check `isKey` first for the same reason. Results are the same either way.
- Link and worker state: the worker task only talks to the server and hands results to the loop task (mutex slot,
  progress atomics, 15-minute busy watchdog); it never touches LVGL. Holding BOOT for 5 s during the splash erases
  the Wi-Fi credentials, including the IDF driver's own store.

### Generator

- `Generate` is deterministic (project order, sorted map keys, fixed formatting) and returns the whole PlatformIO
  tree: the embedded `lib/cyd_template/**`, `src/app/**` verbatim as given, `include/board_config.h`,
  `platformio.ini`, `partitions.csv` and `src/generated/{ui.h,ui.cpp,ui_theme.cpp,ui_assets.cpp,ui_hooks_weak.cpp,fonts.json}`.
  It never writes `include/cyd_build_id.h` (cyd-build does) and emits C++ only. It reproduces
  `testdata/reference/basic` byte for byte for every board (`TestGenerateReproducesReference`); a change of
  generated output updates the reference (when affected) and the goldens in the same commit.
- Refusals, in order: a `template_version` other than `TemplateVersion` is `CYD_INVALID_LAYOUT`
  (`details.reason`, `details.template_version`, `details.embedded`); asset problems are `CYD_BAD_REQUEST`;
  validation errors are `CYD_INVALID_LAYOUT` with `details.issues` (also in `GenerateOutput.Issues`; warnings never
  block); an app path that fails `IsEditablePath` is `CYD_BAD_REQUEST` (`details.path`); more than 4096 extra glyphs
  (characters outside printable ASCII, Latin-1 Supplement and Latin Extended-A, cyd-build's limit) is
  `CYD_INVALID_LAYOUT` (`details.reason`, `details.chars`, `details.limit`). Assets are decoded before `Validate`,
  so a bad asset together with layout errors gives a bad-request error without `Issues`.
- Images (`generate_assets.go`): PNG and JPEG only (the format comes from the header, so a decoder another package
  registers is not enough), 1..2048 px per side. Decoded pixels are far larger than the files, so
  `decodeProjectAssets` reads every header first and refuses before any pixel is decoded: a missing asset, a size
  that differs from the layout, an image larger than the largest display of any board (`maxBoardDisplay()` over
  `Boards()`) and a project whose images need more than `AssetBudgetLarge` even as opaque RGB565. Errors are
  `CYD_BAD_REQUEST` with `details.asset`; the last two add `details.reason` `dimensions` or `budget`. `Validate`
  then applies the exact budget of the board. `InspectAsset` decodes pixels, so an importer must make the same
  header checks before it calls it. Descriptors are LVGL 9: RGB565 little-endian (`lv_color_to_u16` truncation), or
  RGB565A8 (alpha plane after the colours, straight alpha) when any pixel is not opaque, stride `w*2`, data 4-byte
  aligned.
- Generated names: public `ui_<name>` (named widgets), `ui_scr_<screen>`, `UI_SCREEN_<SCREEN>`, `ui_img_<asset id>`,
  `ui_font_<family>_<size>`, `on_<name>_<event>`, `ui_show_<name>`/`ui_hide_<name>` (named msgboxes),
  `ui_<name>_series[n]` (named charts). Private: `ui__<key>_wifi`/`_clock` (key = name, or the decimal traversal
  index of an unnamed widget, which no name can be), `ui__ev_<name>_<event>`, `ui__scr_<screen>_create`/`_kb`,
  `ui__img_<asset id>_map` and fixed helpers (`ui__style_*`, `ui__statusbar_*`, `ui__msgbox_close` and others). A
  new private name must not end in `_wifi`/`_clock` and must not start with `ui__ev_`, `ui__scr_`, `ui__img_` or
  `ui__<digit>`, so it can never collide with a derived one.
- Free text reaches C only through `cString` (quote, backslash and newline escaped, other control bytes and DEL as
  octal `\ooo`, the second `?` of `??` escaped, UTF-8 raw; the same escaper as the template's strings header) or
  `cComment` (control characters, format characters such as bidi overrides and line or paragraph separators become
  spaces, `??` and `*/` are split, trailing spaces and backslashes are cut so a comment never continues). Colours go
  through `cHexColor` (anything but `#RRGGBB`, hex digits in either case, becomes black) and `codeWriter.line` takes a constant format, so a
  user value is never a format string.
- `ui_theme.cpp` is fixed text with the theme's tokens substituted (`renderTheme`); every `ui__style_*` helper is
  defined non-static, so `ui.cpp` may call any of them. A colour that is not `#RRGGBB` (hex digits in either case) renders black and
  `colorLuma` (the `DARK` flag, background luma below 128) counts it as black too, so flag and colour agree; an
  unknown button style renders as filled and the theme id passes through `cComment`.
- Device fidelity rules: panel children are offset by the panel's border width (padding 0), tab pages have no
  padding, gauges are square (`lv_scale` draws a needle only then), message boxes live on `lv_layer_top()` behind a
  hidden backdrop and always have at least one footer button, text areas share one keyboard per screen (a password
  text area sets the bullet `*`: LVGL's default U+2022 is in none of the converted fonts), keypad `LV_SYMBOL_*` keys
  exist only in the built-in `numeric` and `numeric_ok` layouts (custom keys are always quoted text, button symbols
  outside the catalog list are dropped), and canvas buffers come from PSRAM when the board has it.

### Builder image

- The builder image pins PlatformIO Core 6.1.19 (hash-pinned `requirements.txt`, generated from
  `requirements.in`), the platform `espressif32@6.13.0`, every platform package at an exact version
  (framework-arduinoespressif32 3.20017.241212 = Arduino-ESP32 2.0.17, tool-esptoolpy, tool-scons and the
  8.4.0+2021r2-patch5 toolchains), the template libraries (LVGL 9.5.0, LovyanGFX 1.2.30, WiFiManager 2.0.17,
  ArduinoJson 7.4.3) and every `Libraries()` spec. The packages are installed before the platform so PlatformIO never
  resolves a range against the registry of the day, and a drift check after the prewarm fails the image build when
  the installed platform/package set differs from the pinned one. A pin change touches the Dockerfile, `Libraries()`,
  `CURATED_LIBS` in `cyd-build` (and `DHT_LIB_DEPS` in `cyd-selftest` for the DHT/Unified Sensor pins), the
  reference `platformio.ini` files and the version tag `CYD_BUILDER_VERSION`.
- `cyd-build` makes the env's libdeps folder (`/tmp/cyd-pio/libdeps/<env>`) anew for every build, with links into
  `/opt/cyd/libs` for exactly the env's `lib_deps` (read with PlatformIO's own parser, each checked against the
  library's `.piopm`): an entry outside `CURATED_LIBS` is exit 2, and app code that includes a library the project
  does not enable fails to compile (exit 1, "No such file or directory"), as it would in the export. For a missing
  catalog header (`LIBRARY_HEADERS`) `cyd-build` adds a gcc-style `file:line:col: note:` line naming the library to
  enable, which `ParseDiagnostics` keeps as a note. `TestBuilderLibrariesMatchTheGenerator` pins `CURATED_LIBS`, the
  Dockerfile's `--library` list and the generator's specs to one set and `LIBRARY_HEADERS` to `libraryHeaders`;
  `cyd-selftest --all` (and `--libs`) checks the gate, stale objects and the note on the classic CYD.
- `cyd-build` passes `CYD_BUILD_ID` through `include/cyd_build_id.h` (never a `-D` flag: flag changes wipe the
  PlatformIO build cache) and keeps the PlatformIO project checksum stable across work directories. It passes
  `--jobs` (the allowed CPUs, capped by the CPU quota and by one compiler per 512 MiB of the container memory limit
  after 512 MiB for PlatformIO and the linker): without it the compilers are OOM-killed in the 2 GiB container.
- Exit codes are the contract with the service: `0` ok; `1` compile error (a `file:line: error:` compiler diagnostic
  or a linker diagnostic (undefined reference, multiple definition, region overflow) and no killed compiler, so the
  agent can repair `src/app`); `2` tooling or internal error (a compiler killed by the OOM killer, PlatformIO failing
  without a code diagnostic, a missing package or `cannot find -l`, a cancel, a symlink or special file in the work
  tree, an invalid `fonts.json`, a malformed `platformio.ini` or a `lib_deps` entry that is not a curated library, a
  bootloader whose chip differs from `board_config.h`); `3` flash budget exceeded. `2` is never agent-repairable.
  `out/result.json` is always written (also when the log reader hangs up), and `files`/`fonts` are empty on a
  non-zero exit.
- Cache layout: PlatformIO build directory `/cache/build/<builder version>/<cache key>` (the version comes from
  `/opt/cyd/builder-version`, so a new image never reuses objects of an older toolchain), converted fonts in
  `/cache/fonts/<sha256>.c` (key: converter id, the `fonts.json` entry and the SHA-256 of the font files; a hit
  refreshes the mtime, so the oldest entries are the ones to evict), locks in `/cache/locks/<cache key>.lock` and
  `/tmp/cyd-build.lock`. Builds are serialized per cache key across all containers sharing `/cache` and to one
  build at a time per container (a waiting build says so); the service must not start parallel builds in one
  container. Any eviction code works on this layout.
- Fonts are reproducible: a `fonts.json` entry is validated before conversion (symbol `ui_font_[a-z0-9_]{1,48}`,
  a family the image ships, size 8-96, at most 16 ranges, `chars` at most 4096 and nothing below U+007F or in
  control/surrogate classes) and the converter's `Opts` header line, which would leak the command line, is replaced
  by a fixed one (`converter id v2`), so the same entry converts to the same bytes. `firmware.bin` is not
  byte-reproducible.
- Trust boundary: the arguments and the image are trusted, the build tree is not. `platformio.ini`, `partitions.csv`,
  `include/`, `lib/` and `src/generated/` come from the generator only (`platformio.ini` could run Python through
  `extra_scripts`). `cyd-build` refuses symlinks and special files anywhere in the tree, gives PlatformIO, the compiler
  and `lv_font_conv` an allowlisted environment and runs offline on a read-only root file system as uid 1000
  (writes only `/work`, `/cache`, `/tmp`). Compiling untrusted C++ can still read every file the container can read:
  keep secrets out of the image, `/work` and `/cache`. All projects share the one builder container and both volumes,
  so app code can read other projects' cached build output (e.g. via `#include` or `.incbin`): fine for a
  single-owner install, no boundary between users.
- The live builder resources (container `aurago-cyd-builder`, volumes `aurago-cyd-work` and `aurago-cyd-cache`, label
  `aurago.managed=cyd-creator`; names in `internal/dockerutil`) belong to the CYD Creator app only: the agent docker
  tool, agent Compose and the admin Containers API protect them (`internal/tools/AGENTS.md`), so tests and CI use
  other names (`aurago-cyd-*-it`, `cyd-ci-*`).
- Image trust (`internal/server/cyd_creator_builder.go`, `imageTrustProblem`): before every acquire the builder
  inspects `builder_image`. A reference with a registry host needs a `RepoDigests` entry of its own repository
  (`cydImageProvenanceProblem`); a digest-pinned one needs exactly its digest and nothing else. An unpinned one must
  also have the image ID that AuraGo's own pull recorded for exactly this reference (`cydTrustedImage`, written
  atomically after a successful `Pull` to `cyd_builder_image.json` next to `cyd_creator.db`, `cydBuilderTrustFile`;
  the data directory is out of reach of agent file tools; when the pull stream names the registry digest, the image
  must carry exactly that repository digest to be recorded): on the containerd image store (Docker 29 default) a local
  build under the tag gets a repository digest too, so only the record tells it from the pulled image. No record, a
  different ID (re-tagged or replaced locally) or a changed `builder_image` is `CYD_IMAGE_MISSING` with a generic
  text (the server log has the reason) and Status reports the image as not present, so the app offers the pull,
  which re-establishes trust. A single-component reference without '/' (`aurago-cyd-builder:dev`/`:ci`,
  `cydImageLocalOnly`) is a local-only tag the administrator configured for development, the integration test and CI:
  it needs no digest and no record. Any reference with '/' (also Docker Hub `user/repo:tag`) is a registry reference.
  The container is created from the verified image ID, which is part of the fingerprint (a new pull recreates it).
  `initCYDCreator` sets `TrustFile` (`cydBuilderTrustFile`); empty keeps the record in memory only (tests).
- Publishing: `.github/workflows/cyd-builder.yml` builds each architecture natively and smoke-tests it offline
  (amd64 all profiles, arm64 one). The amd64 job then compiles ten builds of the nine golden cases (`all_widgets` on
  two boards, `all_widgets_portrait` and the seven `theme_*` cases) strictly with `TestGenerateCompilesInBuilder`
  (4 CPUs, volumes `cyd-ci-work` and `cyd-ci-cache`). The path filters include `generate*.go` and
  `testdata/golden/**`, and the build job's checkout keeps no credentials (`persist-credentials: false`; the publish
  job's checkout does not set it). Pushes to `main` and `v*` tags (or a manual dispatch with `publish`) push by
  digest, then one job creates the multi-arch `:1` and, only when it does not exist yet, the immutable
  `:<CYD_BUILDER_VERSION>`, and signs the manifest with cosign. A changed image under an unchanged
  `CYD_BUILDER_VERSION` therefore moves only `:1`: bump the version for every pin, template or builder-script change.
  A generator-only push to `main` also runs the workflow and republishes `:1` with unchanged inputs.

### Storage and service (plan 2)

- One SQLite database (`sqlite.cyd_creator_path`, default `./data/cyd_creator.db`, listed in
  `config.SQLiteDatabasePaths`; `dbutil.Open`: WAL, foreign keys, busy timeout) with the overview schema: nine `cc_*`
  tables, `cc_meta.schema_version` 1 and seven indexes, one of them `UNIQUE cc_devices_token(token_hash)` (device
  fixtures need distinct non-empty hashes); `cc_projects.publish_receipt` (plan 3, added before v1 was released) is the
  receipt id of the last committed directory swap; `cc_jobs.error_reason` (plan 4a review, also added to the unreleased
  v1) keeps a failed job's `details.reason`; `cc_messages.code` and `vars` (plan 4a review, also in the unreleased v1)
  keep a system line's translation code and its vars as JSON (`''` for none). Migrations append to `storeMigrations` (never edit a released step; each
  step commits with its version); an existing database is first copied with `VACUUM INTO <db>.v<N>.bak`, and a schema
  newer than the code refuses the start. Times are fixed-width RFC 3339 UTC (`timeLayout`), so SQL order is
  chronological.
- Startup (`NewService`): one transaction (`recoverStore`) turns queued/running builds into `interrupted`
  (`CYD_BUILD_INTERRUPTED`, a `build_status` event each) and queued/running jobs into `interrupted` and releases
  locks of jobs that are no longer active (only `awaiting_confirmation` stays). Then,
  before the worker starts, `discardPartialArtifacts` removes the interrupted builds' artefacts, `*.tmp` staging
  directories and directories named like a build without a `cc_builds` row (so a fresh or restored database sweeps
  old artefacts). `Close` stops the worker (the running and the queued builds end `interrupted`; it waits at most
  30 s), then closes the database; the caller owns and closes the `Builder`.
- Service methods return `*Error` (or a context error); unexpected failures become `CYD_INTERNAL` (`CodeInternal`,
  HTTP 500, generic message) with the cause logged (`internalError`). Writes and consistent reads take the
  per-project mutex (`lockProject`; the map never shrinks, accepted at homelab scale).
- Locking rule: `PutLayout`, `WriteFile`, `DeleteFile` and `StartBuild` refuse with `CYD_PROJECT_LOCKED` (details
  `job_id`) while `locked_by_job` is set, unless the origin is `JobOrigin(<that job>)`; `PutAsset`, `DeleteAsset`,
  `RenameProject` and `DeleteProject` are user-only and always refuse then. Origins: `user`, `job:<id>`, `restore`,
  `import`; `StartBuild` takes only `user` and `job:<id>`. `DeleteProject` also refuses with `CYD_BUSY` while a build
  of the project is queued or running; it deletes the project's rows in all nine tables in one transaction, then the
  artefacts, the directory and the subscriptions.

### Projects, layout, files and assets (plan 2)

- Projects live in `<desktop workspace>/CYD/<slug>/` (slug: ASCII-folded `[a-z0-9-]`, at most 40 bytes, `project`
  when empty, `-project` appended to a Windows device name such as `nul` (Windows `os.Mkdir("nul")` creates
  nothing), `-2` … `-999` when taken in the database or on disk) and are accessed only through `os.Root`. A missing
  project directory or `cyd.project.json`, an oversized or a non-regular one is `CYD_INVALID_LAYOUT` with
  `details.reason` `missing`, `too_large` or `not_regular`.
- Every layout write bumps `version` and mirrors name/board/theme into the row (a value `Validate` rejects keeps the
  previous one); file writes do not bump it. `PutLayout` needs the stored version (else `CYD_PROJECT_CONFLICT` with
  `details.version`), saves semantically invalid layouts and returns the issues; only a parse/structure failure, a
  format other than 1 or a refused asset list is `CYD_INVALID_LAYOUT`. A damaged stored layout lists no assets, so a
  save can replace it.
- Asset entries of a layout save (`checkLayoutAssets`, `details.asset`): every `file` matches `assetFileRegex`
  (`assets/[a-z0-9][a-z0-9_-]{0,62}.(png|jpg|jpeg)`), ids and files are listed once, stored entries never change
  (`file`, `w`, `h`, `alpha`) but may be dropped, and an entry the stored layout lacks (designer undo/redo, a second
  tab overwriting) is accepted only when its file belongs to no other stored entry; the file need not exist
  (`Validate`/`Generate` report a missing image).
- Files: only `IsEditablePath` paths are writable (`CYD_FILE_READONLY` for generated, template, layout and asset
  paths, `CYD_BAD_REQUEST` otherwise), UTF-8 text without NUL, `If-Match` sha256 / `*` (create) / "" (agent,
  unconditional; a delete also takes `*`), at most 64 files, 256 KiB each, 2 MiB in total (`CYD_FILE_LIMIT` with
  `details.limit` `file_bytes|files|total_bytes`). App paths are exact-case, because Windows and macOS open a case
  variant as the other entry: reading or deleting a variant is `CYD_NOT_FOUND`, writing one next to an existing file
  or folder is `CYD_FILE_CONFLICT` (`details.existing`). A hand-made file above 256 KiB is listed with an empty
  `sha256`, reading it is `CYD_FILE_LIMIT`, and only an unconditional write or delete replaces it; a directory, link
  or device at an app path is `CYD_BAD_REQUEST` (`details.reason` `not_regular`) and is never touched. Writes are
  atomic (`rootWriteFile`: synced `.cyd-write-*` sibling renamed over the target; one left by a crash is hidden by
  `IsEditablePath`). User writes set `app_changed_since_job`. Generated paths are listed and read from an on-demand
  `Generate`.
- Assets (`PutAsset`): PNG/JPEG by magic bytes, at most 2 MiB (`MaxAssetUploadBytes`). The header is read first
  (`checkUploadHeader`): an image larger than every board's display is `CYD_ASSET_TOO_LARGE` (`details.reason`
  `dimensions`) before any pixel is decoded; then it must fit the project's display (either orientation) and the
  decoded total `AssetBudget` (`budget`). Names are sanitised to the asset file rule with `_2` … suffixes, ids are
  fresh `a_` ids. Because a hand-edited or imported layout can carry any path, the asset file rule is checked again
  right before every open, read and remove (`OpenAsset`, `DeleteAsset`, `loadGenerateInput`); `DeleteAsset` keeps a
  file that a remaining entry still lists. An asset file above 2 MiB fails `Generate`/`StartBuild` with
  `CYD_ASSET_TOO_LARGE` (`details.asset`, `reason` `file_size`). `OpenAsset` reads the file (at most 2 MiB) into
  memory under the project lock and returns a reader the caller closes: no handle may stay open inside a project
  directory, because on Windows it blocks the directory swap of a publication or restore.

### Events (plan 2)

- `Emit` persists, then fans out under `eventsMu` (seq order); `EmitEphemeral` has seq 0 (the build log is
  ephemeral) and a live persisted event never carries seq 0. `seq` is global (AUTOINCREMENT over all projects), so
  gaps are normal. Subscribers get a 256-event buffer and are dropped (channel closed) when full, when their project
  is deleted and on `Close`. The newest 5000 persisted events per project are kept (`maxEventsPerProject`, pruned
  after every `eventPruneEvery` = 200 emits of that project); `LatestEventSeq` is the project's newest seq. Events of
  deleted projects (an `Emit` racing `DeleteProject`) are dropped at startup and by every retention pass.
- Consumers subscribe before reading `EventsAfter` and drop live events whose seq they already have; a resume point
  below the project's oldest kept event means events were pruned, one above its newest one cannot come from this
  database: the client reloads its state (the SSE handler's `stream_ready` `{"reload":true,"after":N}`, N the seq
  the stream continues after).
- Never call `Emit` or `EmitEphemeral` inside a database transaction or while iterating rows. `Emit` holds
  `eventsMu` (which `EmitEphemeral` also takes) while it inserts through its own connection of the pool
  (`dbutil.Open` default: 4 connections, `busy_timeout` 5 s): inside an open write transaction the insert waits for
  the busy timeout and the event is lost while the transaction cannot commit, and open rows hold connections, so an
  exhausted pool blocks every emitter. Commit or close the rows first, then emit.
- Lock order: `jobs.mu` (plan 3) → project lock → `buildsMu` → `eventsMu` → `subsMu`; `jobRun.mu` and `fenceMu`
  are leaves (never take another lock or emit while holding them). `StartBuild` emits the queued `build_status`
  while it holds `buildsMu` (so `queued` always precedes `running`), and that `Emit` needs a pool connection: never
  take `buildsMu` while holding a transaction or open rows.
- `Close` closes every subscription, and `Subscribe` after `Close` returns an already closed channel: consumers treat
  a closed channel as the end of the stream (the SSE client resumes with `Last-Event-ID`).

### Builds (plan 2)

- `StartBuild` checks the builder and the toolchain (`CYD_DOCKER_UNAVAILABLE`; `CYD_IMAGE_MISSING` while the image
  is missing or being pulled), generates the tree from the project directory or `BuildOptions.Dir` at the current
  version (typed tree and generator refusals such as `CYD_INVALID_LAYOUT` come back before a build exists) and
  queues it: at most 8 waiting builds, else `CYD_BUSY` (also during shutdown). One worker runs builds FIFO, one at a
  time (cyd-build also serializes per container); the cache key is `<project id>-<board>`, the timeout
  `cold_build_timeout_seconds` until a build of that (project, board) succeeded, then `build_timeout_seconds`
  (`CYD_BUILD_TIMEOUT`). `CancelBuild` cancels a queued build at once and stops a running one
  (`CYD_BUILD_CANCELLED`); finished builds are left alone.
- `build_log` events (ephemeral) carry at most 50 lines, each cut at 4096 bytes, at least 200 ms apart; a "lines
  skipped" marker replaces overflow. The full log (2 MiB cap, then a truncation marker) is `build.log`.
- Outcome (`classifyBuild`): exit 0 needs non-empty merged.bin, firmware.bin and firmware.elf whose hashes match
  result.json (a mismatch fails the build with a note); 1 is `CYD_BUILD_FAILED`, or `CYD_GENERATOR_ERROR` when only
  generated paths have errors, plus a note pointing at `build.log` when no diagnostic names a file (a region
  overflow); 2 (toolchain, OOM kill, tree shape) is `CYD_BUILD_FAILED` with every error diagnostic demoted to a
  note plus a toolchain note, an unknown code the same with the note "cyd-build ended with exit code N", and so is
  a failed `Builder.Build` (its typed code or `CYD_BUILD_FAILED`, a generic note; Docker text stays in the server
  log), so plan 3 never tries to repair them; 3 is `CYD_FLASH_BUDGET_EXCEEDED`.
- Artefacts live in `<data>/cyd_creator/artifacts/<build_id>/` (staged in `<id>.tmp`, then renamed) and are only
  touched through an `os.Root` of the artefact directory, after `validBuildID` (`ccb_` plus element id characters).
  A succeeded build keeps merged.bin, firmware.bin and firmware.elf, every finished build except an interrupted one
  (which stores no artefacts) `build.log` (and result.json when cyd-build wrote one). Downloadable are merged.bin, firmware.bin, result.json and build.log
  (`OpenArtifact` returns an open file the caller closes); `firmware.elf` stays internal for `DecodeBacktrace` (1–64
  `0x` addresses, succeeded builds only, two-minute limit). A succeeded build also keeps cyd-build's `out/fonts/`
  (converted fonts and `fonts.json`, hash-checked, each at most `maxFontArtifactBytes` = 7 × 0x640000, unlisted) in
  `fonts/`; `BuildFonts` reads them for the ZIP export (plan 5).
- Retention after every finished build keeps `last_build`, the newest `keep_builds_per_project` − 1 other finished
  builds, the newest succeeded build (failed builds never prune the only good firmware) and every build a device
  runs or targets. An artefact directory whose removal failed is swept at the next start.
- `PullToolchain` runs one pull in the background on the service context (`Close` waits for it; a second call while
  it runs is a no-op, `CYD_BUSY` during shutdown); `Toolchain`/`Capabilities` show the progress. `Capabilities`
  always says enabled and writable (the HTTP layer overlays both); `active_job` reads `cc_jobs` rows with status
  `queued|running|awaiting_confirmation` (plan 3 writes exactly these).

### Agent jobs (plan 3)

- Slot and lock: one job at a time holds the global agent slot (a `cc_jobs` row with status `queued`, `running` or
  `awaiting_confirmation`, which `activeJob` reads, plus `jobs.active` in memory) and its project's lock.
  `setProjectLock` writes `locked_by_job` and emits `lock_changed` under the project mutex (an unknown project is
  `CYD_NOT_FOUND` without an event; callers serialize through `jobs.mu`); the releases in `abortStart`, `finishJob`
  and the publication emit under the project mutex too, so lock events keep the order of the lock changes.
  `StartJob` is `CYD_DISABLED` without a runner and `CYD_BUSY` during shutdown, while another job is active
  (`details.job_id`, `project_id`), while the slot cannot be read (an `activeJob` error never opens a second slot;
  `Capabilities` then shows none), while the project is locked, and while it is fenced (`publication_review`,
  checked again after the lock is set).
- Requests: prompts and chat messages are at most 8000 runes of UTF-8 without NUL; attachments are kind `log` only,
  at most 4 and 16 KiB together, and reach the agent as untrusted data. An empty prompt is allowed for a resume,
  `generate_code` and log attachments (fixed English prompts).
- Start: the project (with `resume`: the draft) is copied into `<workspace>/.cyd-creator-staging/<job_id>/`
  (`copyTree`: regular files only, at most 1000 files and 64 MiB, each file synced, `.cyd-write-*` leftovers
  skipped), the `before_job` revision is recorded and `UserChanges` computed (layout changed, app files and assets
  added/modified/deleted since the newest `job` revision; nil before the first green job). A project tree that
  `snapshotTree` refuses (an app file above 256 KiB: `CYD_FILE_LIMIT`; an asset above 2 MiB: `CYD_ASSET_TOO_LARGE`)
  refuses the start with that typed error; any failed preparation ends the job `failed` and frees the lock. The
  phases run in a goroutine joined to `s.wg`; `Close` sets `jobs.closing` before it cancels and waits, so a running
  job ends `interrupted`.
- Phases: new = planning, awaiting confirmation, layout, coding, building, (repairing, building)*, done; change and
  generate_code start at coding (generate_code, unless resumed, refuses a layout with errors: `CYD_INVALID_LAYOUT`
  with `details.issues`). Planning ends on an accepted `SubmitPlan` (none: "The agent stopped: ..."), coding and
  repair rounds on `FinishJob` (runner `RunComplete` hooks `PlanAccepted`/`JobFinished`), layout when the model stops
  calling tools. An accepted plan stores status `awaiting_confirmation` (the phase stays `planning`) and posts the
  plan summary as the agent's chat message; the job keeps slot and lock but has no run. `ConfirmJob` (`checkLock`
  with the job's origin, working copy present) continues at layout; an edited prompt replaces the prompt, drops
  the plan, is stored as a user message and plans again.
- Building reuses the agent's last build while the working copy fingerprint (content hashes of layout, src/app and
  assets, never firmware hashes) is unchanged, else builds with `BuildOptions{Origin: JobOrigin(id), Dir: staging}`.
  Only `CYD_BUILD_FAILED` with non-generated error diagnostics and `CYD_FLASH_BUDGET_EXCEEDED` are repaired, at most
  `MaxRepairAttempts` times; any other build failure ends the job with the build's code.
- Publication (`publishJob`): the green working copy replaces the project through the receipt swap
  (`publishStage`, below); its asset entries pass `checkLayoutAssets` against the stored list. The commit point is
  the transaction that bumps the version, stores `publish_receipt`, mirrors name/board/theme (`layoutRowMeta`),
  clears lock, draft and `app_changed_since_job` and stores the job `succeeded`. After it, without the job's
  cancellation: the `job` revision (a failure is only logged), the project's private conversations are deleted, an
  older draft is removed, then `project_changed` (layout and files, origin `job:<id>`), `lock_changed`,
  `job_status` and the agent summary (the `FinishJob` text, else the final reply, else a fixed line). A rename
  refused by an open file (`CYD_BUSY`, `details.reason` `file_open`) is retried after 0.25, 0.5, 1, 2 and 4 s
  (`publishRetryDelays`); a cancel or timeout during the wait ends the job. No handle may stay open inside a project
  directory across a swap (Windows refuses the rename).
- Ending without publication (`finishJob`): the project stays unchanged. A working copy that differs from the
  project becomes its single draft (`draft_job`; an older draft's folder and conversation go), otherwise the copy
  and the job's conversation are removed; the lock is released and a system chat line written (the ending's `msg`
  code, with the `_draft` variant when a draft was kept). `classifyJobError`
  decides by the run context's cause, not by the error a waiting step returned: cancel → `cancelled`, timeout →
  `failed` ("exceeded its time limit"), shutdown → `interrupted`, a typed error → `failed` with its code, message
  and an identifier-shaped `details.reason` (`^[a-z][a-z0-9_]{0,63}$`, e.g. `publication_review`, `file_open`; free
  text such as `ParseJobPlan`'s is dropped) as `error_reason` (job JSON `error_reason`, omitted when empty; the
  client tells a fenced publication from another `CYD_BUSY` by it), a Runner error (`agentStopped`) → "The agent
  stopped: <reason>" (scrubbed with `ServiceConfig.Scrub`, at most 300 runes), anything else → `CYD_INTERNAL` with
  a generic text (the cause can name host paths; logged).
- Job rows: `saveJob` emits `job_status` only for a stored state (one retry; it returns the error, a missing row is
  `CYD_JOB_NOT_FOUND`). Failed, cancelled and interrupted endings (`finishJob`, `abortStart`) go through
  `saveTerminalJob` (further tries after 0.2, 0.5 and 1 s), because a lost terminal write keeps the slot taken until
  a cancel or restart; `succeeded` is written inside the publication's commit transaction.
- `CancelJob` answers `CYD_JOB_STATE` for a stored terminal status first (also while the finishing run still holds
  the slot); it cancels a running job together with its running build and finishes a job awaiting confirmation at
  once (under `jobs.mu`, so a concurrent `ConfirmJob` cannot start it).
- Resume: needs the draft folder and a project equal to the draft job's `before_job` revision (else
  `CYD_JOB_STATE`: publishing the draft would undo the later changes). The new job (`resume_of`) copies the draft
  and continues at its phase: queued or planning restart the mode's first phase, layout continues at layout with the
  draft's plan, later phases at coding with it; the runner loads the draft's conversation. `DiscardDraft` clears
  `draft_job` and the job's `staging_dir` in one transaction (see startup recovery below), then removes the folder
  and the conversation; without a draft it changes nothing.
- Timeout: `JobTimeout` per run segment as a timer (`jobs.newTimer`, nil = real timers; tests install fakes): the
  context carries no deadline, `JobRun.TimeRemaining` tells the runner what is left. After confirmation the segment
  gets the budget minus the planning time (at least a quarter; kept in memory, so after a restart nothing is
  deducted). One grace window `min(budget/2, 15 min)`, announced as a `job_step`, only when the working copy
  changed within `min(budget/3, 10 min)` during coding, building or repairing.
- Chat and lists: user lines (start prompt with attachments, edited confirmation), agent lines (plan summary,
  publication summary) and system lines (job endings, recovery; agent and system texts cut at 8000 runes), a
  `job_message` event per line. System lines go through `postSystemMessage` only and carry `code` and `vars`
  (message JSON, omitted on user and agent lines); `postMessage` writes agent lines. `ListMessages` returns the
  newest 200 oldest first (vars decoded with exact numbers; unreadable vars drop the code), `ListJobs` the newest 20;
  both order by time, then rowid. `cc_messages` and `cc_jobs` have no retention.
- `ParseJobPlan` is strict (unknown fields, trailing data, more than 64 KiB raw or 32 KiB decoded refused; a JSON
  string holding the object is accepted): summary 1..1000 runes (newlines allowed), 1..16 screens (120 runes each),
  up to 64 widgets (160) and 16 data sources (200), each on one line, no control, format or line-separator
  characters, `wifi` off|optional|required, `aurago_link` needs Wi-Fi, known libraries once each. Errors are
  `CYD_BAD_REQUEST` with a clipped `details.reason` the agent can act on.
- Events carry only server-written text: `job_step` kinds layout, file, build, repair, info (others become info),
  one line of at most 200 runes from a server template (with clipped widget and screen names, app paths and compiler
  messages), never model prose, reasoning or plan text (`SubmitPlan` reports counts).
- Translation codes (`texts.go`): every `job_step` carries `code` (a `stepTexts` key) and `vars`, every system line
  a `messageTexts` code and its vars; the English `text` is always rendered from the code's template
  (`renderText`: `{{name}}` filled in one pass; a placeholder without a var renders empty and is logged as a
  warning), so text and code cannot drift. Codes match `^[a-z][a-z0-9_]{0,63}$`; vars are identifier names with
  strings (one line, at most 160 runes for steps and 300 for messages) or numbers (`textVars`: integer kinds as
  int64, finite floats). Counts are phrased without plural forms ("screens: 2"), durations as integer vars with unit
  symbols, never Go duration strings: `{{seconds}} s`, from 60 s on the `_minutes` variant with
  `{{minutes}} min {{seconds}} s` (`textSeconds` rounds up), `{{milliseconds}} ms` for the publication retry. A
  failure with a CYD code carries `error_code` next to `error` (`job_failed`, `build_failed`), so the client shows its
  own sentence for the code. Every job ending (`jobEnding.msg`) has a `_draft` variant that adds exactly the draft
  sentence. A new step or system line adds its code to the table and its `cyd_creator.step.*`/`cyd_creator.msg.*` key
  to all 16 `ui/lang/cyd_creator` files (`en.json` equals the template and has no other such key, checked with
  `TextKeys` by the ui key test); `TestEveryStepAndSystemLineUsesATableCode` checks every call site (literal code, a
  var for each placeholder, draft variants), `TestEveryCodeRendersItsEnglishText` the English text of each code. An
  unknown code is logged and emits nothing. Still English inside translated lines (backlog): the layout op summaries
  of `layout_updated` and the runner's `agent_stopped` reason. The
  private conversation lives in `cc_agent_context`: `SaveAgentContext` only for the running job (at most 16 MiB of
  JSON), `LoadAgentContext` returns the job's, else the resumed draft's.

### Revisions (plan 3)

- Revisions hold layout, src/app files and assets as SHA-256 blobs in `<data>/cyd_creator/blobs/<aa>/<sha256>`
  (only through an `os.Root`, deduplicated, synced temporary file renamed into place, hash checked on every read), at
  most 50 per project (trimmed in the inserting transaction; order by time, then rowid). Unreferenced blobs are
  pruned only at startup.
- `RevisionDiff` compares a revision with the project's previous one (the first with nothing): app text files as
  unified diffs (3 context lines, `\ No newline at end of file`, each at most 64 KiB, about 1 MiB per answer, then
  "diff omitted"; LCS tables of at most 4 M cells per file and 16 M per answer, beyond that one delete+insert hunk),
  other app files and assets as a "Binary file, a → b bytes" note (assets from the stored sizes).
- `RestoreRevision` (origin `restore`: respects lock and fence) writes the revision into a stage (paths or folders
  that differ only in letter case are `CYD_FILE_CONFLICT`), checks the layout and its asset entries, saves the
  current project first as a `restore` revision "Before restoring: <label>" when it differs from the newest
  revision, swaps like a publication (setting `app_changed_since_job`), then adds a `restore` revision (a failure is
  only logged) and emits `project_changed`. `RecordRevision` records plan 5's `import` revision.

### Agent tools (plan 3)

- Tools change only the job's working copy, through an `os.Root` of it, with the rules of the project's layout and
  file API. `PhaseTools` is the one phase-to-tool table for the agent schemas and the service checks: every call
  needs the running job, not ending (cancelled or timed out), in a phase that offers the tool, else
  `CYD_JOB_STATE`. Planning: read_project, api_reference, submit_plan; layout: read_project, api_reference,
  update_layout; coding and repairing: all but submit_plan.
- `UpdateLayoutForJob`: `ApplyOps` (atomic, refuses new validation errors), `checkLayoutAssets`, then a canonical
  `MarshalProject`/`ParseProject` round trip whose `CYD_INVALID_LAYOUT` reaches the agent typed. Ops cannot change
  name, board, orientation or the asset list.
- `ReadJobFile` reads app files (exact case: a variant is `CYD_NOT_FOUND`), generated files rendered from the working
  copy (`generated: true`) and `cyd.project.json`; the last two are read-only. `WriteJobFile` has the limits of an
  unconditional `WriteFile` and refuses a case-insensitive clash with another file or folder (`CYD_FILE_CONFLICT`)
  before the existence check; `DeleteJobFile` removes regular files only and the folders it empties.
- `BuildForJob` builds the working copy as an agent build and returns at most 50 diagnostics without generated ones
  in `OrderAgentDiagnostics` order (first error, cyd-build's library notes, other errors, warnings, notes; the cap
  never drops the first error or a library note), `diagnostics_total` and a `next_action`; `SubmitPlan` stores the
  parsed plan; `FinishJob` takes a 1..2000-rune summary for the chat.
- `APIReference` topics (`ReferenceTopics`): overview, cyd (`cyd.h` verbatim from `TemplateFS()` plus runtime notes:
  no certificate checks, LittleFS not mounted, HTTP worker limits), lvgl (LVGL 9 names, C++11 without RTTI), widgets
  (the project's generated symbols), libraries, board. `TestAPIReferenceMatchesTheTemplate` pins its facts to the
  template sources, the generator's version pins, names and font ranges and the curated libraries (each needs a
  `libraryHeaders` entry): a change there updates `tools_reference.go` in the same commit.

### Publication fence and startup recovery (plan 3)

- A project directory swap (job publication, revision restore) writes a synced receipt
  `<data>/cyd_creator/transactions/<receipt id>.json` first (`project_id`, `slug`, `stage`, `previous_version`,
  `new_version`); the commit stores the receipt id in `cc_projects.publish_receipt`. While a swap of a project is
  unresolved the project is fenced in memory: every write, `StartJob` and `ConfirmJob` refuse with `CYD_BUSY`
  (`details.reason` `publication_review`) and `ProjectSummary.publish_review` is true, until a start resolves it.
- `NewService` order: `unfinishedJobIDs` → `recoverStore` → `discardPartialArtifacts` → `recoverJobs` → build
  worker; nothing can start a job or build before recovery ends, and a recovery failure never fails the start.
  `recoverJobs` (`jobs_recover.go`):
  - `recoverReceipts` lists the folder once: committed receipts finish (and a job publication whose cleanup the
    crash cut short gets its `job` revision, its private conversations cleared and its events, but only while the
    project is still at the receipt's `new_version`; a later state is only logged), uncommitted ones
    roll back while the version is still `previous_version`; anything else stays, is logged and fences its project.
    An unreadable folder, more than 1000 entries or a receipt naming no project fences every project.
  - An interrupted job's working copy becomes the draft only when the job got past its preparation (phase not
    `queued` and a `before_job` revision exists) and the copy differs from the project; the older draft is removed
    only after the new one is recorded. A step that fails keeps the copy and fences the project (`recover_<job>`);
    the next start settles that job again (newest job of its project by rowid, interrupted, not the draft, copy
    present, `staging_dir` set: `DiscardDraft` clears it, so a discarded draft whose folder could not be removed
    never comes back).
  - The sweep removes working copies except drafts, jobs awaiting confirmation, stages of receipts left for review
    and copies kept above; it removes nothing when a receipt or the job rows could not be read. A draft whose
    folder is gone is cleared. `pruneOrphanBlobs` runs only here (blob writes dedupe without a lock).
- A staging folder (`.cyd-creator-staging`) that is a link or junction is never followed: `StartJob`,
  `RestoreRevision`, `removeStaging` and the sweep refuse. `receiptPaths` (every swap and its recovery) refuses a
  link or junction at any existing component of `CYD/`, `CYD/<slug>`, the backup or the stage. `DeleteProject`
  removes its jobs' working copies.
- Resolving a fenced project by hand (server log: "CYD Creator receipt needs review" with the receipt and project,
  or "receipts could not all be checked"):
  1. Stop AuraGo. Read the receipt: the project tree is `<workspace>/CYD/<slug>`, the tree before the swap
     `<workspace>/CYD/<slug>.cyd-backup-<receipt id>`, the new tree (when not in place)
     `<workspace>/.cyd-creator-staging/<stage>`.
  2. Leave the tree you want as `CYD/<slug>` and move the other one out of the workspace (or delete it). Do not edit
     the database.
  3. Delete the receipt file, and any file in `transactions/` that is not a receipt (each one fences every
     project). Start AuraGo; a backup without a receipt is swept at the start.
  A `recover_<job>` fence needs no action: restart AuraGo and the job is settled again.

## Work Guidance

- Table-driven tests live beside the source; golden files belong to the generator under `testdata/golden/`.
- A new board profile records its source URL and must pass `TestEveryEmbeddedBoardLoads`.
- A new device theme needs its `testdata/golden/theme_<id>` case (`TestGenerateGolden` requires exactly one per
  `Themes()` entry) and an entry in two hard-coded lists: `wantCases` in `TestGenerateGolden` and the theme list in
  `itBuilds` (`generate_builder_it_test.go`), otherwise its golden is never compiled. A new widget type or action
  kind must appear in the `all_widgets` golden (`TestAllWidgetsGoldenCoversCatalog` derives both from the catalog
  and `actionTypes`); a new event kind is only checked once it is added to that test's hard-coded event list.

## Verification

- `go test ./internal/cydcreator/...`
- `go vet ./internal/cydcreator/...`
- Template or builder changes: build the image
  (`docker build -f deploy/docker/Dockerfile.cyd-builder --build-context cydcreator=internal/cydcreator -t aurago-cyd-builder:dev deploy/docker/cyd-builder`,
  about 10 minutes: the prewarm compiles all nine profiles and the library check) and run
  `docker run --rm --network none --read-only --tmpfs /tmp:rw,exec,size=1g -e HOME=/tmp --user 1000:1000 -v <work volume>:/work -v <cache volume>:/cache <image> cyd-selftest --all`.
  Success is exit 0 with nine `== <board> exit 0` lines and the two `== libs:` lines (DHT enabled exit 0, not
  enabled exit 1); the disabled-library build prints a `fatal error: DHT.h` line and a PlatformIO `[FAILED]` banner
  on purpose, so "no error lines" is not the criterion.
- Generator changes: `go test ./internal/cydcreator/ -run TestGenerate`; after an intended output change run
  `go test ./internal/cydcreator/ -run TestGenerateGolden -update`, review the diff and commit it with the code.
  `-update` is a test flag, unlike the `CYD_UPDATE_*` environment variables of the reference and strings tests; it
  also rewrites a golden `cyd.project.json` that is not in canonical form.
- Compile proof (Docker and the builder image; opt-in, 19 builds):
  `CYD_BUILDER_IT=1 CYD_BUILDER_IT_STRICT=1 CYD_BUILDER_IMAGE=<image> go test ./internal/cydcreator/ -run TestGenerateCompilesInBuilder -v -timeout 90m`.
  It compiles the goldens (`all_widgets` on the classic CYD and an ESP32-S3 board, `all_widgets_portrait`, one build
  per theme) and the reference on every board, in a container with plan 2's settings: read-only root, 1 GiB `/tmp`
  tmpfs (rw, exec, nosuid, nodev), uid 1000, no network, `--cap-drop ALL`, `--memory 2g --memory-swap 2g --init
  --pids-limit 512`. `CYD_BUILDER_IT_STRICT=1` adds `-Wall -Wextra` for `src/` and fails on any compiler warning;
  `CYD_BUILDER_IT_ONLY` keeps the builds whose `case@board` contains one of its comma-separated parts;
  `CYD_BUILDER_IT_CPUS` (default 2) sets `--cpus`; `CYD_BUILDER_WORK_VOLUME` and `CYD_BUILDER_CACHE_VOLUME` name the
  volumes (defaults `aurago-cyd-work-it` and `aurago-cyd-cache-it`; the live `aurago-cyd-work` and
  `aurago-cyd-cache` are refused because the test deletes build directories). One build gets 15 minutes, the
  container lives at most two hours, and everything stops a minute before the test deadline.
- Builder adapter against Docker (opt-in, `internal/server`, unique `aurago-cyd-*-it-<hex>` names removed after the
  test): build the stub once
  (`docker build --label aurago.cyd-plan=1 -t aurago-cyd-builder-stub:test internal/server/testdata/cyd_builder_stub`),
  then `CYD_BUILDER_STUB_IMAGE=aurago-cyd-builder-stub:test go test ./internal/server -run TestCYDBuilderStubMechanicsLive`;
  `CYD_BUILDER_IT=1 CYD_BUILDER_IT_IMAGE=aurago-cyd-builder:dev go test ./internal/server -run TestCYDBuilderIntegration -timeout 60m`
  compiles every board through the adapter (without `CYD_BUILDER_IT_IMAGE` it uses the published image and pulls it
  only when `Status` reports it missing or untrusted).
- `gofmt -l internal/cydcreator` prints nothing; `-race` needs gcc (run it on aurago-test).

## Child DOX Index

None.
