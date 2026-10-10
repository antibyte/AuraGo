# CYD Creator domain

## Purpose

`internal/cydcreator` owns the CYD Creator domain: board profiles, device themes, the widget catalog, the
`cyd.project.json` model with validation and layout patch ops, partition tables, the `cydcc` config blob and the
build diagnostics parser. Later CYD Creator plans add the template and toolchain, the generator, storage and the
service to this same package.

## Ownership

- `errors.go` (`Error`, one `Code*` constant per CYD_* code, `ErrorCodes()`), `ids.go`, `limits.go`.
- `boards.go` + `boards/<id>.json`, `themes.go` + `themes.json`, `catalog.go` (widgets, fonts, symbols, curated
  libraries).
- `project.go`, `validate.go`, `patch.go`: layout model, validation issues and `ApplyOps`.
- `partitions.go`, `configblob.go`, `diagnostics.go` (`ParseDiagnostics`, `IsGeneratedPath`, `IsEditablePath`).
- `testdata/diagnostics/*.txt`: PlatformIO/gcc/ld log fixtures (`.txt`, because `*.log` is git-ignored).
  `.gitattributes` keeps `testdata/**` and the template/builder paths LF on every platform.

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
  orientation; the template maps project orientation to a rotation per driver.
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

## Work Guidance

- Table-driven tests live beside the source; golden files belong to the generator under `testdata/golden/`.
- A new board profile records its source URL and must pass `TestEveryEmbeddedBoardLoads`.

## Verification

- `go test ./internal/cydcreator/...`
- `go vet ./internal/cydcreator/...`
- `gofmt -l internal/cydcreator` prints nothing; `-race` needs gcc (run it on aurago-test).

## Child DOX Index

None.
