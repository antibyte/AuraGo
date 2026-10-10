# CYD Creator domain

## Purpose

`internal/cydcreator` owns the CYD Creator domain: board profiles, device themes, the widget catalog, the
`cyd.project.json` model with validation and layout patch ops, partition tables, the `cydcc` config blob and the
build diagnostics parser, plus (plan 1b) the embedded device template `cyd_template`, the reference generated
project and, outside the package, the builder image (`deploy/docker/Dockerfile.cyd-builder`,
`deploy/docker/cyd-builder/`, `.github/workflows/cyd-builder.yml`). Later plans add the generator, storage and the
service to this same package.

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
  generated symbols: the names `init goto back theme_colors event_index async` and the prefixes `scr_ show_ hide_
  font_` and suffix `_series` are reserved (`NAME_INVALID`). The designer's JS `RESERVED_NAMES` must mirror this
  list. Generator-private symbols use `ui__` or `cyd_`, which no valid name produces. Free text (project name,
  timezone, titles, props, hints) must be escaped wherever it reaches C code.
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
  `clipReason`, deduplicates and returns at most 200 entries in log order. The cap is linear: errors get the budget
  first, then warnings, then notes, and inside a severity the earliest entries win. `Generated` is decided from the
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
  none, an open one never asks. The portal's head element carries a small script (`kPasswordCheck` in
  `cyd_portal.cpp`, a raw string the Go tests extract and run under Node with a fake DOM) that blocks the save form
  and shows `WIFI_PASSWORD_LENGTH` when this rule fails; it reads the lock from WiFiManager's list markup (class
  `l` on the signal icon after `a[data-ssid]`), so re-check it on a WiFiManager bump. The device applies the same
  rule before joining (`password_ok()`, lock flags remembered from the scan); an open listed network joins right
  away without the keyboard. Page, device and driver count UTF-8 bytes; WEP keys (5 or 13 characters) are
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

### Builder image

- The builder image pins PlatformIO Core 6.1.19 (hash-pinned `requirements.txt`, generated from
  `requirements.in`), the platform `espressif32@6.13.0`, every platform package at an exact version
  (framework-arduinoespressif32 3.20017.241212 = Arduino-ESP32 2.0.17, tool-esptoolpy, tool-scons and the
  8.4.0+2021r2-patch5 toolchains), the template libraries (LVGL 9.5.0, LovyanGFX 1.2.30, WiFiManager 2.0.17,
  ArduinoJson 7.4.3) and every `Libraries()` spec. The packages are installed before the platform so PlatformIO never
  resolves a range against the registry of the day, and a drift check after the prewarm fails the image build when
  the installed platform/package set differs from the pinned one. A pin change touches the Dockerfile, `Libraries()`,
  the reference `platformio.ini` files and the version tag `CYD_BUILDER_VERSION`.
- `cyd-build` passes `CYD_BUILD_ID` through `include/cyd_build_id.h` (never a `-D` flag: flag changes wipe the
  PlatformIO build cache) and keeps the PlatformIO project checksum stable across work directories. It passes
  `--jobs` (the allowed CPUs, capped by the CPU quota and by one compiler per 512 MiB of the container memory limit
  after 512 MiB for PlatformIO and the linker): without it the compilers are OOM-killed in the 2 GiB container.
- Exit codes are the contract with the service: `0` ok; `1` compile error (a `file:line: error:` compiler diagnostic
  or a linker diagnostic (undefined reference, multiple definition, region overflow) and no killed compiler, so the
  agent can repair `src/app`); `2` tooling or internal error (a compiler
  killed by the OOM killer, PlatformIO failing without a code diagnostic, a missing package or `cannot find -l`, a
  cancel, a symlink or special file in the work tree, an invalid `fonts.json`, a bootloader whose chip differs from
  `board_config.h`); `3` flash budget exceeded. `2` is never agent-repairable. `out/result.json` is always written
  (also when the log reader hangs up), and `files`/`fonts` are empty on a non-zero exit.
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
  keep secrets out of the image, `/work` and `/cache`.
- Publishing: `.github/workflows/cyd-builder.yml` builds each architecture natively and smoke-tests it offline
  (amd64 all profiles, arm64 one). Pushes to `main` and `v*` tags (or a manual dispatch with `publish`) push by
  digest, then one job creates the multi-arch `:1` and, only when it does not exist yet, the immutable
  `:<CYD_BUILDER_VERSION>`, and signs the manifest with cosign. A changed image under an unchanged
  `CYD_BUILDER_VERSION` therefore moves only `:1`: bump the version for every pin, template or builder-script change.

## Work Guidance

- Table-driven tests live beside the source; golden files belong to the generator under `testdata/golden/`.
- A new board profile records its source URL and must pass `TestEveryEmbeddedBoardLoads`.

## Verification

- `go test ./internal/cydcreator/...`
- `go vet ./internal/cydcreator/...`
- Template or builder changes: build the image
  (`docker build -f deploy/docker/Dockerfile.cyd-builder --build-context cydcreator=internal/cydcreator -t aurago-cyd-builder:dev deploy/docker/cyd-builder`,
  about 10 minutes: the prewarm compiles all nine profiles) and run
  `docker run --rm --network none --read-only --tmpfs /tmp:rw,exec,size=1g -e HOME=/tmp --user 1000:1000 -v <work volume>:/work -v <cache volume>:/cache <image> cyd-selftest --all`.
- `gofmt -l internal/cydcreator` prints nothing; `-race` needs gcc (run it on aurago-test).

## Child DOX Index

None.
