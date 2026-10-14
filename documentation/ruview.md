# RuView Wi-Fi sensing

AuraGo integrates [RuView v3122](https://github.com/ruvnet/RuView/tree/a71634ae8a606bba5463489450c69e8b42de114e), revision `a71634ae8a606bba5463489450c69e8b42de114e`, including its local Dashboard, Observatory, Pose Fusion, calibration, recording and training views. The upstream interface retains its original language; AuraGo's management and board setup are translated into all 16 languages.

Wi-Fi sensing is experimental. Readiness does not prove measurements, presence or pose accuracy. RSSI is a connected host Wi-Fi signal; CSI requires supported sensor hardware. Demo frames remain labelled synthetic, stale frames lose their fresh status, and disconnections never enable client simulation. Missing models are reported by RuView. Model loading/unloading, SONA/LoRA activation and hard-coded pose statistics/demo-zone endpoints are unavailable in this revision; AuraGo blocks them.

The embedded Dashboard distinguishes service health from sensor data. A connected ESP32 source with no frames shows **Waiting for sensor data**, not simulation; failed source probes also make no simulation claim. Only an explicit synthetic source is labelled simulated. New frames restore the live indicator even when the source name is unchanged, and a stream without fresh frames becomes stale after ten seconds. Health, liveness and metrics endpoints remain read-only through AuraGo's authenticated proxy.

Live Demo uses the separate `/api/v1/stream/pose` WebSocket through AuraGo. The pinned Rust service accepts no zone, confidence or frame-rate query filters on this stream; the bundled client omits them. AuraGo supplies upstream authentication from the Vault. A connected sensor stream alone does not prove that the pose stream is connected or that pose frames have arrived.

The Devices view refreshes every three seconds while visible, without overlapping requests. **Last measurement** advances between replies; measurements older than ten seconds and failed refreshes are marked stale. Previously received values remain visible. Closing the app cancels requests and timers. Pose Fusion starts in **CSI Only**; camera and video modes are unavailable.

Skipping or completing the Dashboard tour is remembered in this browser, including after reopening RuView or reloading AuraGo. To replay it, open the Dashboard's gear menu and choose **Reset onboarding → Reset**. Only this completion flag is persisted; other upstream preferences remain in memory. If browser storage is blocked or cleared, the tour may appear again.

Recording uses explicit **Start Recording** and **Stop Recording** actions in both Live View and Training. A recording continues until stopped, including after closing the view. This runtime has no recording-status endpoint: after reload or an uncertain response the UI shows **Recording status unknown**, and an administrator can still stop it. The file list does not prove whether recording is active. Timed recording is unsupported; the proxy rejects `duration_secs` with HTTP 400. Mutations are not automatically retried, and success requires the operation's explicit acknowledgement.

Training, Pretrain and LoRA expose only their supported parameters. LoRA uses a listed base model, a validated profile name and rank 8; ordinary training includes early stopping patience. Server-disabled training remains disabled. Failed or unsupported model, recording and training reads are distinguished from successfully empty lists, retain old values as stale, and offer refresh. Controls dependent on unavailable data remain disabled.

Read-only blocks starts, changes and deletions. Administrators may still stop a managed service, stop recording or cancel training; these remain authenticated, origin-checked writes. AuraGo never shuts down an external service, but administrators may stop its recording and training. Agent access remains limited to the five existing read operations. Native process exit clears readiness and revision confirmation and permits a manual restart. No automatic restart is attempted. A failed or timed-out stop retains ownership and reports that stopping was not confirmed.

## Setup

Open **Configuration → RuView**, enable the integration, select a runtime and source, then save. Start and agent access are disabled by default; read-only starts enabled. Connection tests are explicit administrative POST actions using the saved target and Vault token. Merely opening AuraGo's overview never probes a service. Opening an upstream view or requesting an agent read retrieves data.

The configuration shows external credentials only for external instances, and board networking only for ESP32 sensing. Switching modes preserves unsaved fields. Read-only stays visible under Operation, with a notice explaining the blocked actions; Permissions contains the agent switch. Advanced contains the managed HTTP port and storage budget. Save before testing or setting up a board. Disabled actions explain the missing step. Use **Open RuView** to install or start a managed runtime, and **Set up board** for USB firmware and Wi-Fi provisioning.

Managed Docker connects to RuView automatically; the receiver IP is only for optional ESP32 boards sending UDP over the LAN. The form offers a private IPv4 address from the LAN binding, configured endpoint or browser URL when available. Confirm that it is the RuView host reachable from the board, particularly behind a proxy or tunnel. Accepting the suggestion fills empty receiver and managed LAN-bind fields in the draft; it does not overwrite existing values, enable UDP, grant write access, save or test a connection.

The board dialog opens from Config or Desktop even when no receiver IP has been saved. It loads the saved receiver and UDP port through the administrative, read-only `board-setup` endpoint. It also offers LAN candidates from the configured binding, a literal private endpoint, or the active local interfaces of a native/local-Docker host. It never resolves DNS or probes the network, and never presents AuraGo's container interfaces as the Docker host. Common container/VPN interfaces are excluded. Candidates require an explicit selection; they do not prove that the board can reach the receiver. Late results preserve typed values. If the host cannot be determined (for example an external hostname or container placement), the dialog explains how to find the RuView machine in the router's device list. It shows a separate warning while managed UDP reception is disabled; choosing an address does not enable reception or save configuration.

If starting Docker fails, the overview retains the concrete Engine error across status refreshes. A registry `denied` response means the pinned image is inaccessible; changing the receiver IP does not fix it. The reviewed image must be published and readable by AuraGo before managed Docker can start. Failed connection checks remain disconnected; only previously observed live measurements become stale. The timestamp records the last check, not proof of a received sensor frame.

| Runtime | Intended use |
| --- | --- |
| Managed Docker | Linux sensing server for ESP32 CSI or explicit demo; AuraGo creates and stops its own labelled container. |
| Native host | Optional verified Windows amd64 or macOS amd64/arm64 package; install and start explicitly. Hosts need a working Wi-Fi adapter for RSSI. |
| External | Existing RuView service at the saved HTTP(S) origin. Private/loopback access requires `allow_private`. AuraGo never installs, starts or upgrades it. |

Containerized AuraGo must use an external native instance for host Wi-Fi. Linux has no native RSSI scanner in this integration: the pinned server does not connect its Linux adapter to that runtime path. Select `esp32` for real CSI or `simulated` explicitly for a demo; there is no automatic demo fallback.

Disable read-only to install/start a runtime, provision a board, calibrate, record or train. The agent is independently enabled with `agent_access` and always has only `status`, `nodes`, `sensing`, `model_info` and `recordings`. Its responses include source, time and data freshness. Tokens use Vault keys `ruview_api_token` and `ruview_managed_token`; both are denied to Python/agent secret export and are scrubbed before network operations.

## Network and storage

The external URL must be an origin, without credentials, path, query or fragment. Credentialed requests never follow redirects. DNS is checked on every dial; environment proxies cannot bypass the private-network grant. Only reviewed methods and paths pass through `/api/ruview/proxy/`. Desktop uses `/api/desktop/integrations/ruview/` and retains its own policy gates. Mutations require administrator access, same-origin checks and writable RuView configuration. Pure data WebSockets remain readable in read-only mode; configuration changes cancel active proxy requests.

Managed HTTP binds to loopback, or the AuraGo application Docker network. The separate WebSocket port is not published; data WebSockets use the HTTP listener. Container limits are two CPUs, 2 GiB memory and 128 PIDs, a read-only root, dropped capabilities and no additional privileges. A writable Docker integration is necessary; Docker image builds are never performed by the user's AuraGo process. The standard socket proxy can retain `BUILD=0`.

ESP32 reception needs explicit UDP publication: a concrete RFC1918 IPv4 LAN bind address and an allowlist of private IPv4 sources/CIDRs. The default port is 5005. The receiver address in board setup must reach the **RuView machine** from the board; it is not necessarily the AuraGo server or browser computer. Docker NAT and the host firewall must allow that path.

Managed recordings and models live below `data/ruview/data`; native executables live in `data/ruview/runtime/<revision>/<platform>`. An external service keeps its own storage. The managed storage watcher checks once per second, refuses recording starts when the budget is exhausted and requests a recording stop when it reaches the configured budget (default 1024 MiB). This is a controlled recording stop, not a filesystem quota; data written between checks can exceed the budget. AuraGo does not delete recordings automatically.

Recording creation validates any requested ID as a filename prefix and appends a random suffix. Use the returned ID for later operations: reusing a prefix cannot overwrite an existing recording. External model statistics render as text, and embedded documents allow inline scripts only with an AuraGo-generated nonce. Observatory and the legacy 3D visualization require explicit demo selection for generated frames; stalled live data expires after ten seconds.

## ESP32 browser setup

Use HTTPS (or localhost) and a browser that supports Web Serial, with serial permissions allowed. Plug the board into the **browser computer**, close other tools using the port and use a USB data cable. BOOT while reconnecting can help enter the ROM loader. The RuView iframe has no serial permission; the separate AuraGo dialog owns the serial port.

The catalog includes S3 4/8/16 MiB and C6 4 MiB, each with UART bridge and native USB Serial/JTAG variants. Recognized bridges include catalogued CP210x, CH340/CH343 and FTDI USB IDs. Chip, exact flash capacity, console variant, SHA-256, sizes and offsets must all match. Unknown combinations are rejected. Firmware is built from the pinned **0.8.12** source, not the older 0.6.7 release bins or CYD partition layout.

The native USB (`cdc`) profiles also recognize `303a:4001`, shown by Windows as **Espressif Device / Espressif CDC Device** and documented in Espressif's [CDC example](https://github.com/espressif/esp-idf/blob/master/examples/peripherals/usb/device/tusb_serial_device/README.md). This USB ID identifies the transport only; the loader still verifies the actual chip and flash capacity before any erase or write. If the running firmware's CDC port does not respond to the loader, enter download mode by holding BOOT while pressing and releasing RESET, release BOOT, and select the newly enumerated Espressif port. The COM number may change. An uncertain write is never retried automatically.

**ESP32-S3-WROOM-1 N16R8 means 16 MB flash and 8 MB PSRAM**, as listed in the [Espressif module datasheet](https://documentation.espressif.com/esp32-s3-wroom-1_wroom-1u_datasheet_en.pdf). Select `esp32s3-16mb-cdc` for its native USB Serial/JTAG connection, or `esp32s3-16mb-uart` for a supported UART bridge. The generic firmware does not require or initialize PSRAM, displays or mmWave accessories. The 16 MiB builds have matching flash headers and retain the reviewed 8 MiB partition layout, leaving the upper flash unused. Capacity mismatches report the detected and selected sizes before any erase or write; they never silently select another profile.

1. Choose the exact profile and USB device. New installation or an incompatible partition table requires the explicit erase checkbox.
2. Write and read back the verified firmware. Compatible updates leave the NVS sectors intact.
3. Reconnect and configure Wi-Fi, node ID (1–255), private receiver IPv4 and UDP port through `RUVIEW_HELLO_V1` / `RUVIEW_CONFIG_V1` at 115200 baud. On opening, the wizard proposes the lowest ID absent from the service's known nodes, including stale nodes. It preserves manual input and keeps the ID unchanged during retries and confirmation. If discovery fails or all IDs are occupied, enter an ID explicitly; use an existing ID only when reconfiguring that board. The suggestion does not reserve an ID and cannot account for boards the service no longer reports.
4. Reconnect after restart and confirm the same board identity and non-secret settings. Wi-Fi credentials remain only in browser memory and are sent directly to the board. Both fields stay filled while the dialog is open, including successful setup, explicit retries, cancellation and restart/sensor checks. Closing the dialog, logging out or losing access clears them; reopening starts with empty fields. They never enter AuraGo APIs or browser storage.
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

`internal/ruview/artifacts.json` pins source, image digest and every firmware/runtime hash. `scripts/ruview/vendor-ui.mjs` builds the reviewed UI adapter and local dependencies; `npm run build:browser-vendor` bundles the existing esptool-js dependency shared with ESP Web Tools. Preserve all vendored licenses and manifests. No external instance supplies executable UI content, login flows or service workers. Pose-Fusion includes its pinned WebAssembly modules, expires live CSI after ten seconds, and permits client-generated frames only with AuraGo's explicitly selected demo source. Its untrained upstream fusion/pose visualization remains experimental; displayed confidence is not a calibrated measurement.

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
node --test scripts/ruview/protocol.test.mjs scripts/ruview/upstream.test.mjs
```

The release check intentionally rejects an incomplete platform catalog. For a new artifact revision, publish the reviewed multi-architecture image as a separate authorized release step, record its real manifest digest and platforms, and rebuild AuraGo and its version-bound resource package. Publishing the existing pinned image does not require changing the catalog or rebuilding AuraGo. Never substitute a floating tag or invent a digest. The independent artifact names, target profiles, build metadata and hashes support repeat builds; bit-for-bit reproducibility across CI operating-system/toolchain images has not been established.

`.github/workflows/ruview-publish.yml` publishes an already reviewed OCI archive without rebuilding it. First upload the archive to a published RuView artifact prerelease in this repository, then run the workflow with its numeric release `asset_id` and `archive_sha256`. Draft release assets are inaccessible to the deliberately read-only repository token. The job checks the archive hash, pinned index digest and platform list, then copies all architectures and attestations to GHCR with digest preservation and verifies the published index. Make the package public and verify anonymous access to the exact digest before considering publication complete. The workflow grants only this manual job package-write permission; its repository access remains read-only.

Before release, test actual S3 and C6 first installation, NVS-preserving update, reconfiguration, restart and UDP reception. Test Windows/macOS RSSI using a real adapter and denied OS permissions; macOS helper availability is not permission acceptance. Check Standard/Fruity, wide/narrow layouts, all locales, offline states, all upstream views and stream/timer cleanup. Validate the matching `agotrain` pack and publish its commit before the AuraGo pin that depends on it. Publication, deployment, live measurement quality and hardware acceptance remain separate from local automated tests.
