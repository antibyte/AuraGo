# CYD Creator — operator reference

CYD Creator is a builtin desktop app (`cyd-creator`, start-menu category Development) for building ESP32 /
ESP32-S3 touch-display apps: a designer, an agent that plans and writes the code, builds in a Docker container,
flashing over Web Serial and updates over Wi-Fi. User guide: [manual chapter 25](manuals/en/25-cyd-creator.md)
([German](manuals/de/25-cyd-creator.md)). Code-level contracts live in `internal/cydcreator/AGENTS.md`,
`internal/server/AGENTS.md` (CYD Creator) and `ui/js/desktop/apps/AGENTS.md` (CYD Creator, CYD Creator designer).

CYD Creator is not the older **Cheap Yellow Display** companion (`cyd:` config section, with its own flasher on the
config page); the two share only the hardware.

## Icon library

Select an image or button in the designer and open **Icon library** in its
inspector. The library contains 60 common motifs in playful, modern, comic
and sober styles, each in colour/monochrome and still/animated form (960
variants). Search by translated name or motif ID, filter by category, and
choose 24/32/48/64 px. The two previews show actual device pixels after RGB565
conversion; monochrome masks inherit the device theme text colour unless an
explicit icon colour is set. System reduced-motion preferences show posters.

Only selected images enter the project. Repeated imports and identical frames
count once. The picker displays image bytes and the current project budget;
an oversized import is refused atomically, leaving size and still-image
controls available. Flash budgets remain 256 KiB below 8 MB and 4 MiB on larger
profiles, including PSRAM boards. Frames, tint and origin metadata survive
undo/redo, revisions and project ZIP export/import. Ordinary images with no
icon metadata retain their previous behaviour.

Library API under `/api/desktop/cyd-creator/`:

- `GET icons`: read-only catalog (version, styles, tones, sizes, motifs and
  motion kinds). Uses the verified external UI resource set.
- `POST projects/{id}/icons`: protected write route, required numeric
  `If-Match` layout version, strict JSON
  `{"style":"modern","tone":"mono","motif":"wifi","size":32,"animated":true}`.
  Accepts catalog entries only, never arbitrary URLs or files. Returns the
  base `asset`, `assets`, ordered `frames`, `motion`, `version`, `added_bytes`
  and `total_bytes`. All frames and the asset-list change publish together;
  the designer then assigns the result in one undoable widget edit.

Artwork, exact prompts and deterministic export instructions:
[`assets/cyd-icons/README.md`](../assets/cyd-icons/README.md). ImageGen is used
only during asset production; library use is offline. Compile fixtures cover
all nine boards; physical animation performance needs device acceptance.

## Requirements

- **Virtual Desktop** enabled. Projects live in the desktop workspace under `CYD/<slug>/`.
- **Docker** enabled and not read-only (`docker.readonly: false`): the builder creates, starts and execs into its
  container. AuraGo reaches Docker through `docker.host`; the default socket proxy of the Compose setup is enough
  (it allows container create/start/stop/remove, archive PUT/GET, exec, image inspect and pull, volume create; image
  builds stay blocked with `BUILD=0`). Live-checked on 2026-10-11 with `tecnativa/docker-socket-proxy` v0.5.0.
  Because sources go in and artefacts come out through the Docker archive API, it also works when AuraGo itself
  runs in Docker or uses a remote Docker host.
- `cyd_creator.enabled: true` and an AuraGo restart. The capability `cyd_creator` (and with it the start-menu
  entry) is true when the service started, the section is enabled, and Docker is enabled and not read-only.
- **Disk**: the toolchain image is about 2.3 GB on disk (the app announces "about 3 GB"). Builds keep a PlatformIO
  build directory per project and board plus converted fonts in the volume `aurago-cyd-cache`; nothing prunes it
  automatically (see [Disk space](#disk-space)). Artefacts live under `<data_dir>/cyd_creator/artifacts/`
  (`directories.data_dir`, default `./data`).
- **Flashing and the serial monitor** run in the user's browser: Chrome or Edge with Web Serial, a secure context
  (HTTPS or `localhost`), CYD Creator open in the top-level `/desktop` document (the server denies Web Serial
  everywhere else through `Permissions-Policy`), and `virtual_desktop.serial_browser_enabled: true` (config page:
  **Virtual Desktop → Serial ports on this computer**, off by default). That switch reaches administrators only and
  is off on a read-only desktop; it is the same switch Quick Connect's browser serial uses. Without it the Flash
  dialog still offers the image (`merged.bin`) as a download.

## Configuration (`cyd_creator:`)

Config page: **Config → CYD Creator**. Zero or a missing number means the default (except `max_repair_attempts`,
where 0 means no repair). `config.Load` clamps out-of-range numbers and logs a warning; a config save refuses them.

| Key | Default | Range | Applies | Meaning |
|---|---|---|---|---|
| `enabled` | `false` | | switching on: restart | turns the feature on; switching off answers `CYD_DISABLED` at once |
| `builder_image` | `ghcr.io/antibyte/aurago-cyd-builder:1` | image reference, ≤ 255 bytes | restart | toolchain image, pulled from the app, never built by AuraGo; see [Builder image](#builder-image) |
| `build_timeout_seconds` | `300` | 60–1800 | live | build limit once a build of that project and board succeeded |
| `cold_build_timeout_seconds` | `600` | 60–3600, ≥ `build_timeout_seconds` | live | build limit until then |
| `memory_mb` | `2048` | 512–16384 | restart | builder memory (= memory + swap) |
| `cpu_cores` | `2` | 0.5–16 | restart | builder CPUs; the compiler job count also follows the memory (one per 512 MiB above 512 MiB) |
| `idle_stop_minutes` | `20` | 1–1440 | restart | the builder container stops after this idle time |
| `max_repair_attempts` | `3` | 0–10 | live | agent repair rounds after a failed build |
| `job_timeout_minutes` | `20` | 5–120 | live | agent job budget |
| `model` | `""` | provider ID | live | provider for agent jobs; empty = main LLM; an unknown ID makes jobs fail with `CYD_DISABLED` |
| `keep_builds_per_project` | `5` | 1–50 | live | build retention (see [Builds](#builds)) |
| `ota.enabled` | `true` | | live | device check-ins, minting device keys and OTA updates |
| `ota.heartbeat_seconds` | `60` | 30–600 | live | device check-in interval |
| `ota.public_url` | `""` | device URL rule | live | URL devices use to reach AuraGo; empty = derived from the server config (see [Device URL](#device-url-and-certificate-pin)) |

A change of `docker.host` needs a restart too. Without a running service (switched on since the start, or a refused
start) every save that keeps `enabled` on asks for a restart. The config page shows a restart note.

Storage: SQLite `sqlite.cyd_creator_path` (default `./data/cyd_creator.db`, part of backups). Next to it,
`cyd_builder_image.json` records the image ID of AuraGo's own pull. CYD Creator refuses to start (the API answers
`CYD_DISABLED` with a generic `details.problem`, AuraGo keeps running) when that file would lie inside the virtual
desktop workspace or the agent's file root: an agent-written record could make a locally built image pass as the
pulled one.

## Builder image

The default `ghcr.io/antibyte/aurago-cyd-builder:1` is built and published by `.github/workflows/cyd-builder.yml`
on pushes to `main` and on `v*` tags (amd64 and arm64, smoke-tested, cosign-signed; `:1` moves, `:<version>` such as
`:1.0.0` is created once and never changes). **Until that workflow has published it, the image does not exist**:
**Install build tools** then fails with `CYD_IMAGE_PULL_FAILED`. Build it on the Docker host AuraGo uses and point
the config at the local tag:

```bash
docker build -f deploy/docker/Dockerfile.cyd-builder \
  --build-context cydcreator=internal/cydcreator \
  -t aurago-cyd-builder:dev deploy/docker/cyd-builder
```

```yaml
cyd_creator:
    builder_image: aurago-cyd-builder:dev
```

The build takes about ten minutes (the prewarm compiles every board profile once); restart AuraGo afterwards. A
reference without `/` (such as `aurago-cyd-builder:dev`) is a local-only tag the administrator chose: AuraGo uses it
as it is and never pulls it.

Contents (version `1.0.4`, `CYD_BUILDER_VERSION`): Node 22 bookworm-slim base with `tini`, PlatformIO Core 6.2.0
(hash-pinned), platform `espressif32@6.13.0` with Arduino-ESP32 2.0.17 (framework-arduinoespressif32
3.20017.241212) and exactly pinned packages including SCons 4.11.1, LVGL 9.5.0, LovyanGFX 1.2.30,
WiFiManager 2.0.17, ArduinoJson 7.4.3 and the curated libraries a project can enable in its settings:
PubSubClient 2.8, OneWire 2.3.8, DallasTemperature
4.0.6, DHT sensor library 1.4.7, Adafruit BME280 2.3.0, Adafruit BMP280 3.0.0 and Adafruit NeoPixel 1.15.5 (with
Adafruit Unified Sensor 1.1.15 and BusIO 1.17.4). Builds run offline; a new library needs a new image version.

**Image trust.** Before every build AuraGo inspects `builder_image`. A registry reference must carry a repository
digest of its own repository, and an unpinned one (`:1`) must also match the image ID AuraGo recorded after its own
pull (`cyd_builder_image.json`); a digest-pinned reference (`…@sha256:…`) must carry exactly that digest. A locally
built or re-tagged image under the registry name is therefore treated as missing (`CYD_IMAGE_MISSING`, the server
log has the reason) and the app offers the pull again. The container is created from the verified image ID.

**Protection.** The live resources belong to CYD Creator only: container `aurago-cyd-builder`, volumes
`aurago-cyd-work` and `aurago-cyd-cache`, label `aurago.managed=cyd-creator`. The agent's docker tool refuses every
operation on them and hides them from its lists, agent Compose refuses them (including `container:` references and
building or tagging the builder image repository), and the administrator container API lists them under the
protected owner `cyd-creator`. A container named `aurago-cyd-builder` without the CYD Creator labels is refused
(`CYD_DOCKER_UNAVAILABLE`) and never touched. Tests and CI use other names (`aurago-cyd-*-it`, `cyd-ci-*`).

## Builder container

One long-running container per AuraGo (`sleep infinity` under `tini`): no network, all capabilities dropped,
`no-new-privileges`, read-only root, 1 GiB `/tmp` tmpfs, uid 1000, memory, CPU and 512-pid limits, restart policy
`no`. `/work` (`aurago-cyd-work`) and `/cache` (`aurago-cyd-cache`) are shared by all projects. Sources go in with
`PUT /archive`, artefacts come back with `GET /archive` (strictly checked: exact paths, size caps, no links), and
`cyd-build` runs as an exec. Cancel, timeout or a broken output stream stop and remove the container; the next build
creates a fresh one. The idle timer only stops it; AuraGo's shutdown removes it (the volumes stay).

`cyd-build` prints a keep-alive line at least every 240 s, because the socket proxy (HAProxy `timeout server 10m`)
cuts an exec stream that stays silent for ten minutes. It runs one build at a time per container and per cache key,
passes `--jobs` so compilers fit the memory limit, and exits with 0 (ok), 1 (compile or link error, the agent may
repair `src/app`), 2 (tooling or internal error, never sent to the agent) or 3 (flash budget exceeded).

All projects share one builder and its volumes, so app code of one project can read another project's cached build
output: fine for a single-owner install, no boundary between users.

## Builds

- One build runs at a time, FIFO; at most 8 wait (else `CYD_BUSY`). The cache key is `<project id>-<board>`; the
  first build of a key gets `cold_build_timeout_seconds`.
- A succeeded build keeps `merged.bin` (complete image for 0x0), `firmware.bin` (app slot, used by OTA and by the
  "Keep settings" flash), `firmware.elf` (only for backtrace decoding), `result.json`, `build.log` and the converted
  fonts (for the ZIP export) in `<data_dir>/cyd_creator/artifacts/<build id>/`. The full log is capped at 2 MiB.
- Retention after every finished build: the project's last build, the newest `keep_builds_per_project` − 1 other
  finished builds, the newest succeeded build and every build a device runs or is due to get.
- Typical size on the classic CYD: 70–82 % of its 1.875 MiB app slot (template plus a full layout).

## Boards

| id | Board | Chip / flash | Display / touch | Status |
|---|---|---|---|---|
| `esp32-2432s028r` | CYD 2.8" (micro-USB) | ESP32 / 4 MB | 320×240 ILI9341_2 / XPT2046 | verified |
| `esp32-2432s028-usbc` | CYD2USB 2.8" (USB-C) | ESP32 / 4 MB | 320×240 ST7789 / XPT2046 | community |
| `esp32-2432s024r` | CYD 2.4" resistive | ESP32 / 4 MB | 320×240 ILI9341 / XPT2046 | community |
| `esp32-2432s024c` | CYD 2.4" capacitive | ESP32 / 4 MB | 320×240 ILI9341 / CST820 | community |
| `esp32-3248s035r` | CYD 3.5" resistive | ESP32 / 4 MB | 480×320 ST7796 / XPT2046 | community |
| `esp32-3248s035c` | CYD 3.5" capacitive | ESP32 / 4 MB | 480×320 ST7796 / GT911 | community |
| `jc4827w543` | Guition 4.3" | ESP32-S3, PSRAM / 4 MB | 480×272 NV3041A (QSPI, landscape only) / GT911 | community |
| `esp32-8048s043` | Sunton 4.3" | ESP32-S3, PSRAM / 16 MB | 800×480 ST7262 (RGB) / GT911 | community |
| `esp32-8048s070` | Sunton 7" | ESP32-S3, PSRAM / 16 MB | 800×480 EK9716 (RGB) / GT911 | community |

Only `esp32-2432s028r` was tested on hardware (2026-10-10). Promote a profile by setting `"status": "verified"` in
`internal/cydcreator/boards/<id>.json` after a hardware test (orientation, colours, touch axes, backlight, Wi-Fi);
`go test ./internal/cydcreator/...` must stay green.

## Partitions

Every board has two OTA app slots, the 4 KiB config partition `cydcc` (device URL, device id, token, certificate
pin) and a LittleFS data partition. NVS (0x9000, 20 KiB) holds the touch calibration and the Wi-Fi settings;
`otadata` sits at 0xE000.

| Flash | app slots (0x10000 …) | `cydcc` | `littlefs` | image budget (decoded) |
|---|---|---|---|---|
| 4 MB | 2 × 1.875 MiB | 0x3D0000 | 0x3D1000, 188 KiB | 256 KiB |
| 8 MB | 2 × 3 MiB | 0x610000 | 0x611000, 1980 KiB | 4 MiB |
| 16 MB | 2 × 6.25 MiB | 0xC90000 | 0xC91000, 3516 KiB | 4 MiB |

No current board profile uses 8 MB.

## Flashing

The Flash dialog downloads the build's image, checks its SHA-256, connects with esptool-js (921600 baud, falling
back to 460800 and 115200), compares the chip with the board profile (`CYD_FLASH_WRONG_CHIP`), writes, verifies
by MD5 and resets the board. Install modes:

- **Full install** writes `merged.bin` at 0x0. This wipes NVS: touch calibration and Wi-Fi settings are gone.
- **Keep settings** writes `firmware.bin` at app0 (0x10000) and erases `otadata` (8 KiB of 0xFF at 0xE000, so app0
  boots even after an OTA update had switched to app1). Calibration and Wi-Fi stay.
- **Automatic** (default) picks "keep settings" only when this browser's record (`localStorage`
  `aurago.cydcreator.boards.v1`) says this project flashed the connected board (same MAC, chip and board profile)
  and the partition table on the board still matches the build; otherwise a full install. The server keeps no MAC
  addresses, so another browser always starts with a full install.

Every flash also rewrites `cydcc`: with a device blob when the build has the AuraGo link and the user may write,
else with 0xFF, so no stale token survives. A port another program holds (a Quick Connect terminal, another tab) is
`CYD_FLASH_PORT_BUSY`.

## Devices

A build with the AuraGo link (project setting, needs Wi-Fi) makes the Flash dialog mint a device
(`POST /api/desktop/cyd-creator/projects/{id}/devices`, write access and `ota.enabled` required). The server creates
a token `ccdt_<64 hex>` (scope `cyd_creator_device`, stored only as SHA-256, never logged, never shown to anyone)
and returns it once inside the 4 KiB config blob that the flasher writes to `cydcc`. There is no way to read a token
back; `POST /api/tokens` refuses the device scope.

- A project has at most 64 devices (`CYD_BUSY` beyond). A minted device stays *pending* until its first heartbeat
  and is removed after one hour without one.
- A re-flash from the same browser revokes the board's previous device of that project; a flash from another
  browser leaves the old entry until it is removed by hand.
- *Online* means the last heartbeat is at most three intervals old.
- **Remove** (`DELETE devices/{id}`) revokes the token at once. **Update to** (`PATCH devices/{id}` with
  `target_build_id`) sets the build the device fetches at its next heartbeat; **Rename** sets `name`.

## OTA protocol

Device endpoints under `/api/cyd-creator/device/` need no session. Every request without
`Authorization: Bearer ccdt_<64 hex>` is answered 401 before the body is read; the service then looks up the token
hash. With `ota.enabled: false` every device request is `CYD_OTA_DISABLED`; devices keep running their app.

| Endpoint | Purpose | Limit per device |
|---|---|---|
| `POST heartbeat` | status in (build, IP, RSSI, free heap, template version, OTA state, last OTA error); `{interval_s, update}` out | 12 per minute |
| `GET firmware/{build_id}` | `firmware.bin` of the device's current target build (own project, own board), never `merged.bin` | 3 per 10 minutes |
| `POST ota-result` | outcome of an update attempt | 10 per 10 minutes |

Failed authentication is limited per client address (IPv4 address, IPv6 /64): 20 distinct failing tokens within 10
minutes block that address with `CYD_RATE_LIMITED`; a revoked device that keeps sending its old token counts once.

The update offer is `{build_id, size, sha256, url}` with `url` = `/api/cyd-creator/device/firmware/<build_id>`, a
path the device appends to the URL from its config blob (the firmware accepts only an empty value, a `/path` or an
absolute URL below its own base). The device checks size and SHA-256 before it switches slots.

Rollback: a boot counter in RTC memory counts crash resets (panic and watchdogs) and is cleared on power-on, after
the first successful heartbeat or after 120 s of uptime. After three unstable boots the device returns to the other
slot, but only to the image it was running before an OTA update, never to a stale build left there by a USB flash.
The device then reports `rolled_back` with "rolled back from <build id>", and the server resets the target to the
running build so the failed build is not offered again. Picking that same build again during the same boot is reset
too: it takes a board restart or a new build to retry (the Devices dialog says so). A failed download or check is
retried by the firmware after 10 minutes.

## Device URL and certificate pin

Devices use `ota.public_url` when set, else a URL derived from the server config: the HTTPS domain or
`server.host` (a wildcard or loopback bind becomes the first LAN IPv4 address) with the HTTPS port, or `http://` with
`server.port` when HTTPS is off. Set `ota.public_url` when AuraGo runs in Docker (a wildcard bind derives the
container's address), behind a reverse proxy or NAT, or when the derived host is not reachable from the devices'
Wi-Fi.

The firmware joins URLs as strings and resolves the host itself, so the URL rule is strict (checked on config save,
blanked with a warning at load, checked again before minting): literal lowercase `http://` or `https://`, printable
ASCII, no credentials, query or fragment, no IPv6 literal, a host of letters, digits, dots and hyphens only (no
underscores, no `%`-escapes), port 1–65535. When a host name breaks the rule, use the IP address. A URL that fails
is `CYD_BAD_REQUEST` with `details.reason: device_url` and the hint `cyd_creator.ota.public_url`; the Flash dialog
shows it.

With `https://` the device pins AuraGo's leaf certificate: the SHA-256 of the DER leaf goes into the config blob,
and no token leaves the device before the pin matched. For a derived URL AuraGo reads its own certificate file
(self-signed, custom `cert_file` or the Let's Encrypt cache) and probes the URL only while Let's Encrypt has not
issued one yet; for `ota.public_url` it probes that URL (a reverse proxy may present its own certificate). An
unreadable pin is `CYD_BAD_REQUEST` with `details.reason: tls_pin`. **A certificate change (Let's Encrypt renews
about every 60 days, a regenerated self-signed certificate, a new proxy certificate) breaks the pin**: the device
shows "AuraGo certificate changed – please reflash via USB" and needs one USB flash. With `http://` there is no pin
and the token travels in clear text; firmware images are still checked by SHA-256.

## ZIP export and import

- `GET projects/{id}/export` returns `<slug>/…`: the complete PlatformIO project (generated `platformio.ini` with
  pinned libraries, `src/generated`, `include`, `lib/cyd_template`, `partitions.csv`), `src/app`, the converted
  fonts of the newest succeeded build whose `fonts.json` matches (else the newest one, flagged in the README),
  `cyd.project.json`, the original images and a README with build steps. When the generator refuses the project
  (layout or image errors) the generated files are left out and the README says so, so the export still works as a
  backup. Paths that would not unpack safely everywhere are left out and listed. Never included: Wi-Fi settings and
  device tokens. App code is exported as it is; the README tells users to check `src/app` for keys before sharing.
- Limits: at most 128 MiB unpacked (`CYD_FILE_LIMIT`, `details.limit: export_bytes`), two exports at a time
  (`CYD_BUSY`, `details.reason: export`), none while the project awaits a publication review.
- `POST import` (multipart field `file`) creates a new project, never touching an existing one: archive at most
  20 MiB, at most 500 entries, the kept entries (layout, `src/app` sources, listed PNG/JPEG images) at most 20 MiB
  unpacked. Generated and template files, the README and hidden folders are dropped unread and generated again. A
  newer template than installed is `CYD_TEMPLATE_TOO_NEW`. Other refusals are `CYD_IMPORT_INVALID` with
  `details.reason`: `not_zip`, `too_large`, `too_many_entries`, `unsafe_path` (zip slip, absolute paths, drive
  letters, backslashes, `.`/`..`, Windows device names, trailing dots or spaces, duplicates), `layout_missing`,
  `layout`, `file_type`, `board`, `app_files` (size, not UTF-8, case clash) or `asset` (damaged, too large, wrong
  type or name, more than 16 MiB together). The import records an "Import" version.

## Limits

| What | Limit |
|---|---|
| Layout | 1 MiB, 16 screens, 200 widgets per screen, nesting depth 4, 1024 bytes per text, 500 characters per logic hint |
| App files (`src/app`) | 64 files, 256 KiB each, 2 MiB together |
| Images | PNG/JPEG, 2 MiB per upload, 16 MiB of originals per project, decoded budget 256 KiB (4 MB flash) or 4 MiB |
| Builds | one at a time, 8 queued, log 2 MiB |
| Agent | one job at a time across all projects, prompt 8000 characters, up to 4 log attachments with 16 KiB together |
| Devices | 64 per project, pending for at most one hour |
| Versions | 50 per project |
| Import / export | 20 MiB and 500 entries / 128 MiB unpacked, two at a time |

## Disk space

`aurago-cyd-cache` grows with every project and board (PlatformIO build directories under
`/cache/build/<builder version>/`, converted fonts under `/cache/fonts/`); deleting a project does not shrink it, and
a new builder version starts a fresh build tree next to the old one. To reclaim the space, stop AuraGo, remove the
container `aurago-cyd-builder` and the volume `aurago-cyd-cache` with the Docker CLI on the host, and start AuraGo
again: the next build of each project is a cold build. Build artefacts on the AuraGo side follow
`keep_builds_per_project`.

## Troubleshooting

| Symptom / code | Fix |
|---|---|
| App missing from the start menu | Enable `cyd_creator`, enable Docker with `docker.readonly: false`, restart AuraGo; the server log shows why the service did not start. |
| `CYD_DISABLED` with `details.restart_required` | Restart AuraGo. With `details.problem`: the storage refusal above or a failed start, see the server log. |
| `CYD_DOCKER_UNAVAILABLE` | Docker not reachable: check `docker.host` and the socket proxy; or a foreign container named `aurago-cyd-builder` exists. |
| `CYD_IMAGE_MISSING` | Click **Install build tools**. For a registry image that was built or re-tagged locally, pull it again from the app. |
| `CYD_IMAGE_PULL_FAILED` | Registry unreachable, disk full, or the image is not published yet: build `aurago-cyd-builder:dev` ([Builder image](#builder-image)). |
| `CYD_BUILD_TIMEOUT` | Raise `build_timeout_seconds` / `cold_build_timeout_seconds` or give the builder more CPUs. |
| `CYD_BUILD_FAILED` with a toolchain note | Exit code 2 (out of memory, missing package, broken tree): raise `memory_mb`, check `build.log`. |
| `CYD_FLASH_BUDGET_EXCEEDED` | The app does not fit its slot: remove images or font sizes, or pick a 16 MB board. |
| `CYD_GENERATOR_ERROR` | A generator bug: export the project and report it; the agent never repairs generated code. |
| Export README says the fonts are missing or changed | Build once in CYD Creator (a succeeded build keeps the converted fonts), then export again. |
| Flash dialog: serial ports switched off / desktop read-only | Turn on `virtual_desktop.serial_browser_enabled` (Virtual Desktop → Serial ports on this computer); leave read-only mode. |
| Flash dialog: no Web Serial / not secure / embedded | Chrome or Edge, HTTPS or an SSH tunnel to `localhost`, CYD Creator inside `/desktop`; or **Download image**. |
| `CYD_FLASH_WRONG_CHIP` | The board does not match the project's board profile. |
| `CYD_FLASH_PORT_BUSY` | Close the program or tab that holds the port (Quick Connect terminal, another serial monitor). |
| `device_url` | Set `ota.public_url` to a URL the devices can reach that passes the rule; use the IPv4 address if the host name has an underscore. |
| `tls_pin` | The configured https URL did not present a certificate within 5 s: check it is reachable from AuraGo, or fix the certificate file. |
| `CYD_OTA_DISABLED` | `ota.enabled` is off: devices get no updates and no new device keys are minted. |
| Device never shows up under Devices | Check that `ota.public_url` (or the derived URL) is reachable from the device's network; the serial monitor shows the link attempts. A pending device disappears after one hour. |
| Device shows "AuraGo certificate changed – please reflash via USB" | AuraGo's TLS certificate changed (renewal or new self-signed certificate): flash the device once more over USB. |
| `CYD_RATE_LIMITED` on device requests | 20 distinct bad tokens from one address within 10 minutes; remove stale devices or wait. |
| `CYD_BUSY` with `publication_review` | A project directory swap was interrupted. Stop AuraGo; the server log ("CYD Creator receipt needs review") names the receipt in `<data_dir>/cyd_creator/transactions/`. Keep the tree you want as `CYD/<slug>` (the other one is `CYD/<slug>.cyd-backup-<receipt>` or the stage in `.cyd-creator-staging/`), move the other one out of the workspace, delete the receipt file and start AuraGo. Do not edit the database. Details: `internal/cydcreator/AGENTS.md` (Publication fence). |
