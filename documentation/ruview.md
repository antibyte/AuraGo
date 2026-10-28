# RuView Wi-Fi sensing

AuraGo integrates [RuView v3122](https://github.com/ruvnet/RuView/tree/a71634ae8a606bba5463489450c69e8b42de114e), revision `a71634ae8a606bba5463489450c69e8b42de114e`, including its local Dashboard, Observatory, Pose Fusion, calibration, recording and training views. The upstream interface retains its original language; AuraGo's management and board setup are translated into all 16 languages.

Wi-Fi sensing is experimental. Readiness does not prove measurements, presence or pose accuracy. RSSI is a connected host Wi-Fi signal; CSI requires supported sensor hardware. Demo frames remain labelled synthetic, stale frames lose their fresh status, and disconnections never enable client simulation. Missing models are reported by RuView. Model loading/unloading, SONA/LoRA activation and hard-coded pose statistics/demo-zone endpoints are unavailable in this revision; AuraGo blocks them.

The embedded Dashboard distinguishes service health from sensor data. A connected ESP32 source with no frames shows **Waiting for sensor data**, not simulation; failed source probes also make no simulation claim. Only an explicit synthetic source is labelled simulated. New frames restore the live indicator even when the source name is unchanged, and a stream without fresh frames becomes stale after ten seconds. Health, liveness and metrics endpoints remain read-only through AuraGo's authenticated proxy.

The embedded Dashboard's Hardware tab no longer shows simulated real-time CSI: amplitude and phase show "—", and the antenna and Wi-Fi values are labelled as example configuration that is not read from the hardware. The Live Demo's signal-strength and latency placeholders show "—". The hero, feature cards, performance and application sections carry the note "Upstream RuView project claims — not measured or verified by AuraGo." The offline pose Demo button appears only while AuraGo's RuView source is explicitly `simulated`. It stops when that changes, when the stream stops, or when the tab is left.

Live Demo uses the separate `/api/v1/stream/pose` WebSocket through AuraGo. The pinned Rust service accepts no zone, confidence or frame-rate query filters on this stream; the bundled client omits them. AuraGo supplies upstream authentication from the Vault. A connected sensor stream alone does not prove that the pose stream is connected or that pose frames have arrived.

The **Boards** view lists every board AuraGo knows. It is shown only for the ESP32 source. It reads the board list every three seconds while visible, without overlapping requests. **Last measurement** is a relative age and advances between replies. Measurements older than ten seconds and failed refreshes are marked stale. An offline board can keep its previous RSSI, labelled as the last value, for the same receiver only. A measurement time that the reply left empty shows a dash. **Set up this board** opens setup with that node ID. **Add board** opens setup for a new node ID. Previously received values remain visible. Closing the app cancels requests and timers.

The **Status** view answers one question first: is it working? A headline states the state in one sentence. One button offers the next step when there is one. A visible reason explains a blocked button. A setup list shows what is still missing: turned on, changes allowed, service running, receiver and UDP set, first board and data arriving. Each open step has its own fix button. The list disappears once real sensor data has arrived. **Details** is collapsed by default. It shows the service and revision, the source, storage used against the budget, the last recording action this AuraGo session confirmed, and open hints. **Copy diagnostics** copies the same facts as plain `key=value` lines. The copy has no token, URL, file path or legacy error sentence. Install is only offered for the native runtime. When RuView is enabled and writable but this session has not started a managed runtime, the headline says so and **Start service** is the next step. Opening the app does not start a service.

The status read is passive, so Status never contacts an external service. For an external instance, use **Check now** in the **⋯** menu or as the next step. The check result appears in the headline. Ten seconds later, unless the check failed, the headline says when the service was last checked and offers **Check now** again.

**Allow changes…** appears when read-only blocks a start. After you confirm, AuraGo saves only `ruview.readonly` set to off, through the normal administrator config save. Nothing else is sent. On a read-only desktop the button opens the settings instead.

The **Lab** view shows one card each for Observatory, Pose fusion and the original Dashboard. A page loads only when you choose **Open**. **Back** or **Close** removes it, and so does leaving the Lab view. Pose Fusion starts in **CSI Only**; camera and video modes are unavailable.

Known board IDs are stored under AuraGo's `data/ruview/`, separately for each receiver. Managed runtime health checks also discover boards while the app is closed. A successful empty reply, lost connection, upstream eviction or restart no longer removes known boards: they remain visible without claiming fresh measurements. Only IDs are persisted, not measurements or Wi-Fi credentials. Older installations populate this list on the next reception/discovery; boards that have never been observed or confirmed cannot be reconstructed from an empty upstream response. The administrative `GET /api/ruview/nodes` merges live and saved IDs and reports `live_state` (`ok`, `not_applicable` when the source has no board list, `not_running` when the managed runtime is not running in this process, `unreachable` or `invalid`). `available` is true exactly for `ok`; `POST` accepts only a `node_id` after confirmed board setup, with the normal admin, origin and write gates. Desktop uses the equivalent integration routes. Agent and upstream proxy APIs keep their live-data semantics.

Skipping or completing the Dashboard tour is remembered in this browser, including after reopening RuView or reloading AuraGo. To replay it, open the Dashboard's gear menu and choose **Reset onboarding → Reset**. Only this completion flag is persisted; other upstream preferences remain in memory. If browser storage is blocked or cleared, the tour may appear again.

Recording uses explicit **Start Recording** and **Stop Recording** actions in both Live View and Training. A recording continues until stopped, including after closing the view. This runtime has no recording-status endpoint. The upstream views therefore cannot know whether recording is active after a reload. AuraGo's overview records only start and stop responses that this process actually forwarded. A successful start or stop stays visible for the rest of the AuraGo session. A failed, incomplete or cancelled forwarded response keeps that last confirmed result and says the acknowledgement was not confirmed. A rejection that never left AuraGo, such as read-only mode or an invalid request, does not. Restarting AuraGo returns the marker to unknown. The file list does not prove whether recording is active. Timed recording is unsupported; the proxy rejects `duration_secs` with HTTP 400. Mutations are not automatically retried, and success requires the operation's explicit acknowledgement.

Training, Pretrain and LoRA expose only their supported parameters. LoRA uses a listed base model, a validated profile name and rank 8; ordinary training includes early stopping patience. Server-disabled training remains disabled. Failed or unsupported model, recording and training reads are distinguished from successfully empty lists, retain old values as stale, and offer refresh. Controls dependent on unavailable data remain disabled.

Read-only blocks starts, changes and deletions. Administrators may still stop a managed service, stop recording or cancel training; these remain authenticated, origin-checked writes. AuraGo never shuts down an external service, but administrators may stop its recording and training. Agent access remains limited to read operations. Native process exit clears readiness and revision confirmation and permits a manual restart. No automatic restart is attempted. A failed or timed-out stop retains ownership and reports that stopping was not confirmed.

## Setup

Open **Configuration → RuView**, enable the integration, select a runtime and source, then save. The integration and agent access are disabled by default; read-only starts enabled. At AuraGo startup, an enabled, writable managed runtime starts once in the background. Docker also requires its existing write permission; native packages must already be installed and verified. External services are never started automatically. Enabling RuView during an existing AuraGo session still requires **Start** or the next AuraGo startup.

Managed runtimes receive non-overlapping health checks about every three seconds while running, so the Status view follows readiness, connection failures and recovery without a manual test. Startup failures remain visible and are not retried automatically. A start reports its progress as `phase`: preparing, pulling the image (only when it is missing), creating the service and waiting for the first answer. **Stop service** works during a start and cancels it. A second start or install while one is running answers `busy`. A manual stop remains stopped until an explicit start or the next AuraGo startup. A managed service that ends on its own, as a native process or a Docker container, shows **RuView stopped unexpectedly** (`process_exited`) and is not restarted. A failed stop (`stop_failed`) and a service that exited (`process_exited`) stay visible across probes and configuration changes until an explicit Start or Stop replaces them. **Not checked**, starting and stopping each have their own headline. None of them is shown as unavailable. Opening or reopening the app never starts a service. The Boards view reads the board list every three seconds while it is visible. Status reads the board list only for a managed ESP32 service, and a managed runtime that is not running is never contacted for it. An external instance is contacted only by **Check now**, by the Boards view, or by an explicit data read. Connection tests still use the saved target and Vault token. A connection test reports success only for fresh sensor data. A connected service without its first frame reports `waiting_for_frames`. Status shows it as waiting (*RuView is running and waiting for data from the boards.*), not as success. Stale, source-mismatch and unexpected-simulation results are never shown as success.

The configuration shows external credentials only for external instances, and board networking only for ESP32 sensing. Switching modes preserves unsaved fields. Read-only stays visible under Operation, with a notice explaining the blocked actions; Permissions contains the agent switch. Advanced contains the managed HTTP port and storage budget. Save before testing or setting up a board. Disabled actions explain the missing step. Use **Open RuView** to open the app, where **Install** and **Start service** manage a managed runtime. Use **Set up board** for USB firmware and Wi-Fi provisioning.

Managed Docker connects to RuView automatically; the receiver IP is only for optional ESP32 boards sending UDP over the LAN. The form offers a private IPv4 address from the LAN binding, configured endpoint or browser URL when available. Confirm that it is the RuView host reachable from the board, particularly behind a proxy or tunnel. Accepting the suggestion fills empty receiver and managed LAN-bind fields in the draft; it does not overwrite existing values, enable UDP, grant write access, save or test a connection.

The board dialog opens from Config or Desktop even when no receiver IP has been saved. It loads the saved receiver and UDP port through the administrative, read-only `board-setup` endpoint. It also offers LAN candidates from the configured binding, a literal private endpoint, or the active local interfaces of a native/local-Docker host. It never resolves DNS or probes the network, and never presents AuraGo's container interfaces as the Docker host. Common container/VPN interfaces are excluded. Candidates require an explicit selection; they do not prove that the board can reach the receiver. Late results preserve typed values. If the host cannot be determined (for example an external hostname or container placement), the dialog explains how to find the RuView machine in the router's device list. It shows a separate warning while managed UDP reception is disabled; choosing an address does not enable reception or save configuration.

If starting Docker fails, Status keeps the failure as its headline until the next explicit Start, Stop or Install, or until a runtime setting changes. The headline names the cause in plain words, for example **RuView can't start: the registry denied the service image.** (`image_denied`). The Engine's own text is under **Technical details**, scrubbed of paths, URLs and addresses. A registry `denied` response means the pinned image is inaccessible; changing the receiver IP does not fix it. The reviewed image must be published and readable by AuraGo before managed Docker can start. A failed connection check shows its own cause, for example `unreachable`. The last-checked time records the last check, not proof of a received sensor frame.

| Runtime | Intended use |
| --- | --- |
| Managed Docker | Linux sensing server for ESP32 CSI or explicit demo; AuraGo creates and stops its own labelled container. |
| Native host | Optional verified Windows amd64 or macOS amd64/arm64 package; install explicitly, then start manually or with AuraGo when enabled and writable. Hosts need a working Wi-Fi adapter for RSSI. |
| External | Existing RuView service at the saved HTTP(S) origin. Private/loopback access requires `allow_private`. AuraGo never installs, starts or upgrades it. |

Containerized AuraGo must use an external native instance for host Wi-Fi. Linux has no native RSSI scanner in this integration: the pinned server does not connect its Linux adapter to that runtime path. Select `esp32` for real CSI or `simulated` explicitly for a demo; there is no automatic demo fallback.

Disable read-only to install/start a runtime, provision a board, calibrate, record or train. The agent is independently enabled with `agent_access` and always has only `status`, `nodes`, `sensing`, `model_info`, `recordings`, and `presence`. `nodes` stays the live list. `presence` also includes saved offline IDs for this receiver and reports the same `live_state`. `fresh: false` does not prove presence, and a simulation is not a sensor. RSSI is included only for a board that is live in that reply; an offline board has no RSSI. Responses include source, time and data freshness. A successful data read is kept when the following status probe fails, with `fresh: false` and a short `status_error`. A failed data read is an error and has no invented payload. Tokens use Vault keys `ruview_api_token` and `ruview_managed_token`; both are denied to Python/agent secret export and are scrubbed before network operations.

## Network and storage

The external URL must be an origin, without credentials, path, query or fragment. Credentialed requests never follow redirects. DNS is checked on every dial; environment proxies cannot bypass the private-network grant. Only reviewed methods and paths pass through `/api/ruview/proxy/`. Desktop uses `/api/desktop/integrations/ruview/` and retains its own policy gates. Mutations require administrator access, same-origin checks and writable RuView configuration. Pure data WebSockets remain readable in read-only mode. Only runtime-relevant configuration changes cancel active proxy requests, pulls and starts. Those are the RuView settings that define the runtime (enabled, mode, source, URL, ports, receiver and UDP settings), the data directory, and the Docker host and switches. Such a change also stops a running managed service, with the stop reason *Stopped after a settings change*. Agent access, read-only and the storage budget change nothing.

The Desktop frame is sandboxed without the page origin. It cannot open its own network connections, and it has no serial, camera, or microphone permission. The Desktop page relays reviewed API calls and sensing sockets for that frame. A fetch from the frame to any other path does not leave the browser. The frame loads its pinned scripts and fonts as static files. Those files do not use the session cookie. The proxy and status routes still require an administrator.

Managed HTTP binds to loopback, or the AuraGo application Docker network. The separate WebSocket port is not published; data WebSockets use the HTTP listener. Container limits are two CPUs, 2 GiB memory and 128 PIDs, a read-only root, dropped capabilities and no additional privileges. A writable Docker integration is necessary; Docker image builds are never performed by the user's AuraGo process. The standard socket proxy can retain `BUILD=0`.

Stop removes the container that AuraGo created, so a stopped service leaves nothing behind. A container named `aurago-ruview` that AuraGo did not create is never stopped or removed. Stop then succeeds and logs a warning, and Start reports `container_conflict` with the fix. While a managed Docker service runs, AuraGo asks the Engine about its container every five seconds. A container that exited, was removed or was replaced is shown as exited (`process_exited`). That check has its own five-second timeout and does not hold the lifecycle lock.

ESP32 reception needs explicit UDP publication: a concrete RFC1918 IPv4 LAN bind address and an allowlist of private IPv4 sources/CIDRs. The default port is 5005. The receiver address in board setup must reach the **RuView machine** from the board; it is not necessarily the AuraGo server or browser computer. Docker NAT and the host firewall must allow that path. Passive status reports this without contacting RuView: `udp_scope` is `managed`, `external` or `off`; `udp_enabled` follows the saved flag and is never set for an external service; `udp_ready` requires a running managed runtime, UDP on, a private bind and at least one allowlist entry. `receiver_configured` means the saved receiver is a private IPv4 address.

Managed recordings and models live below `data/ruview/data`; native executables live in `data/ruview/runtime/<revision>/<platform>`. An external service keeps its own storage. The storage check measures the managed data folder at most every 30 seconds. A recording start uses a measurement that is at most 30 seconds old, or measures again. When the budget (default 1024 MiB) is reached, recording starts are refused with HTTP 507, and the watcher requests a recording stop. This is a controlled recording stop, not a filesystem quota; data written between measurements can exceed the budget. A folder that cannot be measured is `storage_unreadable`: a permission error, a link, a special file or more than 200,000 entries. Recording starts are then refused with HTTP 503, and the budget is not treated as full. A missing folder counts as empty. AuraGo does not delete recordings automatically.

Recording creation validates any requested ID as a filename prefix and appends a random suffix. Use the returned ID for later operations: reusing a prefix cannot overwrite an existing recording. External model statistics render as text, and embedded documents allow inline scripts only with an AuraGo-generated nonce. Observatory and the legacy 3D visualization require explicit demo selection for generated frames; stalled live data expires after ten seconds.

## Status fields and error codes

`GET /api/ruview/status` (administrator) returns the fields below next to the existing ones. The desktop reads the same data through `/api/desktop/integrations/ruview/status`. The legacy `error` field stays for compatibility. It is a scrubbed English sentence, and the Status view does not show it.

| Field | Meaning |
| --- | --- |
| `runtime` | `external`, `not_started`, `starting`, `running`, `stopping`, `stopped`, `exited` or `stop_unconfirmed`. Always set. |
| `phase` | The step of a long start or install. Empty when idle. |
| `phase_since` | When `phase` was entered. `null` when `phase` is empty. |
| `error_code` | One stable code from the table below. Empty when there is none. |
| `error_detail` | Scrubbed detail, at most 240 bytes. No URL, path or address. |
| `last_fresh_at` | Last stored probe with fresh, non-simulated data. Only a runtime-relevant settings change resets it. |
| `stop_reason` | `user`, `config_changed`, `process_exit` or `aurago_shutdown`. Empty while the runtime runs. |
| `stopped_at` | When `stop_reason` was recorded. |
| `storage_used_bytes` | Size of the last successful walk of the managed data folder. `null` for an external service, before the first walk, or after a failed walk. |
| `storage_budget_bytes` | `storage_mb` in bytes for a managed service. `null` for an external service. |
| `recording_at` | When the last forwarded recording start or stop got a 2xx answer. |
| `hints` | Open hint names, sorted: `sensing_stale`, `storage_limit`, `stop_unconfirmed`. Always an array. |

Phase order:

- Docker start: `preparing`, then `pulling_image` only when the image is missing, then `creating`, then `waiting_ready`.
- Native start: `preparing`, `verifying`, `creating`, then `waiting_ready`.
- Install: `downloading`.

`waiting_ready` ends at the first stored probe that reports ready, or after 30 seconds.

`error_code` takes the first match from this list: lifecycle errors (`stop_failed`, `process_exited`), then a start or install failure, then probe results (`source_mismatch`, `unexpected_simulation` and the connection codes). `storage_unreadable` is set only when nothing else is.

Errors from the RuView routes use the HTTP status of their code and this body:

```json
{"error_code":"image_denied","error":"RuView image pull returned HTTP 403: registry denied","detail":"HTTP 403: registry denied"}
```

`error` is the scrubbed English sentence, and `detail` is the scrubbed technical text. Neither contains a token, URL, file path or address. The raw Engine text goes only to the server log. A refused recording start through `/api/ruview/proxy/` is plain text instead: 503 when the storage folder cannot be checked, and 507 when the budget is full.

| Code | HTTP | Meaning |
| --- | --- | --- |
| `disabled` | 403 | RuView is turned off. |
| `readonly` | 403 | Read-only blocks the change. |
| `external_runtime` | 403 | An external service is started and stopped by its operator. |
| `docker_readonly` | 403 | The Docker integration is off or read-only. |
| `already_running` | 409 | The service is already running. Stop it first. |
| `not_installed` | 409 | The native package is not installed or failed its check. |
| `busy` | 409 | Another start or install holds the lifecycle slot. |
| `cancelled` | 409 | A stop, a settings change or a shutdown cancelled the action. |
| `container_conflict` | 409 | A container named `aurago-ruview` belongs to another workload. |
| `platform_unsupported` | 409 | No verified native package for this platform in this release. |
| `invalid_node_id` | 400 | The board number is outside 1 to 255. |
| `invalid_config` | 400 | The settings do not allow the action. |
| `unreachable` | 502 | The RuView service could not be reached. |
| `upstream_auth` | 502 | RuView rejected the access token (401 or 403). |
| `upstream_error` | 502 | RuView answered with another error. |
| `docker_unavailable` | 502 | The Docker Engine could not be reached. |
| `image_denied` | 502 | The registry denied the service image. |
| `image_pull_failed` | 502 | The service image could not be downloaded. |
| `contract_unsupported` | 502 | RuView answered in a format this AuraGo version does not support. |
| `download_failed` | 502 | The native package could not be downloaded or failed its check. |
| `start_failed` | 502 | Any other start failure, for example the data folder, the Vault or container placement. |
| `stop_failed` | 502 | Stopping was not confirmed. |
| `process_exited` | 502 | The service ended on its own. Status only. |
| `source_mismatch` | 502 | RuView uses a different signal source than set in AuraGo. Status only. |
| `unexpected_simulation` | 502 | RuView sends simulated data although real sensing is set. Status only. |
| `timeout` | 504 | RuView or Docker did not answer in time. |
| `storage_unreadable` | 503 | The data folder cannot be measured. |
| `storage_full` | 507 | The storage budget is reached. |
| `artifact_missing` | 500 | This release has no verified digest or package for the service. |
| `unknown` | 500 | Any other failure. |

## ESP32 browser setup

Use HTTPS (or localhost) and a browser that supports Web Serial, with serial permissions allowed. Plug the board into the **browser computer**, close other tools using the port and use a USB data cable. BOOT while reconnecting can help enter the ROM loader. The RuView iframe has no serial permission; the separate AuraGo dialog owns the serial port.

The catalog includes S3 4/8/16 MiB and C6 4 MiB, each with UART bridge and native USB Serial/JTAG variants. Recognized bridges include catalogued CP210x, CH340/CH343 and FTDI USB IDs. Chip, exact flash capacity, console variant, SHA-256, sizes and offsets must all match. Unknown combinations are rejected. Firmware is built from the pinned **0.8.12** source, not the older 0.6.7 release bins or CYD partition layout.

The native USB (`cdc`) profiles also recognize `303a:4001`, shown by Windows as **Espressif Device / Espressif CDC Device** and documented in Espressif's [CDC example](https://github.com/espressif/esp-idf/blob/master/examples/peripherals/usb/device/tusb_serial_device/README.md). This USB ID identifies the transport only; the loader still verifies the actual chip and flash capacity before any erase or write. If the running firmware's CDC port does not respond to the loader, enter download mode by holding BOOT while pressing and releasing RESET, release BOOT, and select the newly enumerated Espressif port. The COM number may change. An uncertain write is never retried automatically.

**ESP32-S3-WROOM-1 N16R8 means 16 MB flash and 8 MB PSRAM**, as listed in the [Espressif module datasheet](https://documentation.espressif.com/esp32-s3-wroom-1_wroom-1u_datasheet_en.pdf). Select `esp32s3-16mb-cdc` for its native USB Serial/JTAG connection, or `esp32s3-16mb-uart` for a supported UART bridge. The generic firmware does not require or initialize PSRAM, displays or mmWave accessories. The 16 MiB builds have matching flash headers and retain the reviewed 8 MiB partition layout, leaving the upper flash unused. Capacity mismatches report the detected and selected sizes before any erase or write; they never silently select another profile.

1. Choose the exact profile and USB device. A board whose partition-table area is blank (all 0xFF), as on a new chip, is written without erasing. A board with a different partition layout is not written until you tick the explicit erase checkbox; the dialog says so and writes nothing.
2. Write and read back the verified firmware. Compatible updates leave the NVS sectors intact.
3. Reconnect and configure Wi-Fi, node ID (1–255), private receiver IPv4 and UDP port through `RUVIEW_HELLO_V1` / `RUVIEW_CONFIG_V1` at 115200 baud. On opening, the wizard proposes the lowest ID absent from both live and saved boards, including offline nodes. Opening it for one known board keeps that board's ID. It preserves manual input and keeps the ID unchanged during retries and confirmation. If discovery fails or all IDs are occupied, enter an ID explicitly; use an existing ID only when reconfiguring that board. The suggestion does not reserve an ID and cannot account for boards never observed or confirmed by this AuraGo installation.
4. Reconnect after restart and confirm the same board identity and non-secret settings. Matching readback registers only the node ID in AuraGo, independently of sensor reception. A save failure is shown separately and can be retried by repeating confirmation. Wi-Fi credentials remain only in browser memory and are sent directly to the board. Both fields stay filled while the dialog is open, including successful setup, explicit retries, cancellation and restart/sensor checks. Closing the dialog, logging out or losing access clears them; reopening starts with empty fields. They never enter AuraGo APIs or browser storage.
5. Check reception separately. Success requires a fresh real CSI source and a recently seen CSI sequence for the configured node. Firmware success and configuration success alone do not prove UDP delivery.

There is no automatic retry after an uncertain flash write. A cancelled operation may require re-entering the bootloader. Preserve Wi-Fi uses the firmware's explicit `KEEP` protocol; passwords are never read back. Transient status/network failures leave the draft open, while every action still requires a fresh successful permission check. Confirmed access revocation or disabling/read-only mode closes the dialog and clears credentials.

Each explicit flash attempt reads the current firmware catalog again, preserving the selected profile and Wi-Fi draft when setting up several boards in one dialog. Missing offsets and mismatched catalog revisions still stop before writing. Config and Desktop load the wizard, its modules, the serial loader and firmware files with the same resource version. After updating AuraGo, reload the page before starting the next board so an already open page cannot retain an older flasher.

The dialog labels its sections **Step 1: Write firmware**, **Step 2: Set up Wi-Fi and receiver**, and **Step 3: Check sensor data**. Step 2 includes the restart/readback confirmation: it is available after a successful configuration transfer, and checking sensor reception is available after matching readback. Existing compatible firmware can proceed directly to step 2. Each action reports progress and errors beside its controls. A browser port named **USB JTAG/serial debug unit** normally uses the native USB (`cdc`) profile; a USB-UART bridge uses `uart`. Chip and flash capacity must still match. Unknown flash failures retain their bounded diagnostic text, with separate bootloader guidance; they never trigger an automatic retry.

Verify the shared wizard with `node scripts/ruview/wizard-browser.test.mjs` (Chrome/Playwright), plus the existing protocol/upstream tests, `TestRuViewConfigBrowser` and backend `TestBoardSetup*` / `TestRuViewBoardSetup*`. Serial fixtures verify the protocol and UI flow, not physical board acceptance.

During flashing, the dialog shows file checks, connection, optional erase, writing,
verification and restart separately. Writing includes a percentage based on image
bytes; phases without a measured percentage show an activity indicator. The busy
indicator ends only after verification and reset. **Step 1 complete** then shows
**Continue to step 2**, which focuses the Wi-Fi form without opening a serial port
or sending settings. Firmware completion does not confirm Wi-Fi or sensor reception.

Repeating readback clears its previous confirmation and the dependent sensor success before opening a port. A cancelled attempt, failed readback or different board leaves sensor verification disabled. Repeating the sensor check clears its previous success even if the new check fails. Wi-Fi drafts and the resource-version binding are preserved.

## Artifact maintenance and release gate

`internal/ruview/artifacts.json` pins source, image digest and every firmware/runtime hash. `scripts/ruview/vendor-ui.mjs` builds the reviewed UI adapter and local dependencies; `npm run build:browser-vendor` bundles the existing esptool-js dependency shared with ESP Web Tools. The last patch stage of `vendor-ui.mjs`, `scripts/ruview/patch-honesty.mjs`, keeps invented upstream data from being shown as measured. The honesty, control and sensing gate patches use the checked replacement helpers, so a vendoring run with `--source` fails when an upstream anchor moves. `--check` verifies only the pinned output hashes. Preserve all vendored licenses and manifests. No external instance supplies executable UI content, login flows or service workers. AuraGo also serves the fonts: the adapter replaces the upstream Google Fonts import with bundled Inter and JetBrains Mono and drops the proprietary FK Grotesk Neue face, so the Dashboard uses the bundled Geist. Pose-Fusion includes its pinned WebAssembly modules, expires live CSI after ten seconds, and permits client-generated frames only with AuraGo's explicitly selected demo source. Its untrained upstream fusion/pose visualization remains experimental; displayed confidence is not a calibrated measurement.

The isolated UI uses the upstream-compatible `three-ruview` npm alias at 0.160.0 and `stats-ruview` at 0.17.0; these pins do not replace AuraGo's other Three.js versions. Review their bounds when upgrading RuView. The status `revision` identifies the adapter contract; `revision_verified` is true only for a runtime started from the managed pinned artifacts. An external server's revision is not asserted from its version string.

The local build contains Windows amd64 and all eight firmware profiles. The reviewed Linux amd64/arm64 image is publicly available as [`ghcr.io/antibyte/aurago-ruview:v3122`](https://github.com/users/antibyte/packages/container/package/aurago-ruview), with the unchanged catalog digest `sha256:573c0e15781b2c2f0450111f1ade85fb2b1e4b06d2b3f39dfceb6d7a0d41a41b`. The [RuView artifact prerelease](https://github.com/antibyte/AuraGo/releases/tag/ruview-v3122) contains the reviewed OCI archive and its checksum; it is separate from AuraGo application releases. **macOS packages still require successful native runner builds and catalog import before release.** The arm64 image is cross-compiled; execution on an arm64 host remains unverified. Unsupported/unpackaged native platforms fail closed. Local Windows compilation and a synthetic API smoke test do not establish Wi-Fi or board acceptance.

`.github/workflows/ruview-artifacts.yml` builds Windows, both macOS architectures (including the Swift `mac_wifi` helper), both Linux image architectures and eight firmware profiles. It uploads build artifacts only, with no publication permissions. Native runner labels follow the [GitHub runner reference](https://docs.github.com/en/actions/reference/runners/github-hosted-runners). Review the artifacts and source provenance before import:

```sh
node scripts/ruview/package-native.mjs --source=<pinned checkout> --platform=windows-amd64 --out=<package>
node scripts/ruview/import-native.mjs --package=<reviewed package>
node scripts/ruview/catalog-firmware.mjs --build=<firmware/out>
node scripts/ruview/check-artifacts.mjs
node scripts/ruview/check-artifacts.mjs --release
node scripts/ruview/vendor-ui.mjs --check
node scripts/build-browser-vendor.js --check
node scripts/build-ui-bundles.js --check
node --test scripts/ruview/model.test.mjs scripts/ruview/protocol.test.mjs scripts/ruview/upstream.test.mjs
```

The release check intentionally rejects an incomplete platform catalog. For a new artifact revision, publish the reviewed multi-architecture image as a separate authorized release step, record its real manifest digest and platforms, and rebuild AuraGo and its version-bound resource package. Publishing the existing pinned image does not require changing the catalog or rebuilding AuraGo. Never substitute a floating tag or invent a digest. The independent artifact names, target profiles, build metadata and hashes support repeat builds; bit-for-bit reproducibility across CI operating-system/toolchain images has not been established.

`.github/workflows/ruview-publish.yml` publishes an already reviewed OCI archive without rebuilding it. First upload the archive to a published RuView artifact prerelease in this repository, then run the workflow with its numeric release `asset_id` and `archive_sha256`. Draft release assets are inaccessible to the deliberately read-only repository token. The job checks the archive hash, pinned index digest and platform list, then copies all architectures and attestations to GHCR with digest preservation and verifies the published index. Make the package public and verify anonymous access to the exact digest before considering publication complete. The workflow grants only this manual job package-write permission; its repository access remains read-only.

Before release, test actual S3 and C6 first installation, NVS-preserving update, reconfiguration, restart and UDP reception. Test Windows/macOS RSSI using a real adapter and denied OS permissions; macOS helper availability is not permission acceptance. Check Standard/Fruity, wide/narrow layouts, all locales, offline states, all upstream views and stream/timer cleanup. Validate the matching `agotrain` pack and publish its commit before the AuraGo pin that depends on it. Publication, deployment, live measurement quality and hardware acceptance remain separate from local automated tests.
