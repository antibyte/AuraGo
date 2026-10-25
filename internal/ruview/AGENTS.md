# RuView sensing

## Purpose

Pinned RuView v3122 integration: managed or external runtime, passive status, authenticated proxy, and board setup.

## Ownership

This package owns the runtime client, proxy policy, node inventory and passive status. `internal/server` owns the HTTP routes and admin gates. `ui/js/desktop/apps/ruview.js` and `ui/js/ruview/wizard.mjs` own the Desktop app and board wizard. `documentation/ruview.md` owns operator guidance. The vendored upstream UI stays untranslated.

## Local Contracts

- `Status()` is passive. It does not dial, resolve DNS, or read the node file. Opening the app does not start a runtime. Overview and Devices read `Nodes()` every three seconds while visible; for a managed runtime that is not running, `Nodes()` returns `not_running` without dialing.
- Enabling RuView does not start a runtime in an already running session. A manual stop, process exit, or uncertain stop is not restarted automatically. `stop_failed` and `process_exited` survive `Configure()` and probes until an explicit `start` or `stop` replaces them.
- Passive fields, copied from saved configuration and this process:
  - `udp_enabled` follows the saved flag and is false for external mode.
  - `udp_ready` is true only for a running managed runtime with UDP enabled, a private bind, and at least one allowlist entry.
  - `udp_scope` is `managed`, `external`, or `off`. External means the operator runs reception; it is not a failure. Disabled configuration is `off`.
  - `receiver_configured` is true when the saved receiver is a private IPv4 address.
  - `recording_session` is `unknown`, `started`, or `stopped` for this process only. It starts as `unknown`.
  - `recording_ack` is empty or `unconfirmed`.
- The recording marker lives on the manager. `Configure()` clears cached probe status and must not clear a confirmed session.
- `noteRecording` runs only after `/api/v1/recording/start` or `/api/v1/recording/stop` is forwarded. A 2xx start sets `started` and clears the acknowledgement. A 2xx stop sets `stopped` and clears it. Any other forwarded result, including a transport failure or an invalid body, sets `unconfirmed` and keeps the last confirmed session. A local rejection that never leaves AuraGo does not call it.
- The wizard never enables UDP and never stores Wi-Fi credentials. An uncertain flash is never retried. An explicit node ID from 1 to 255 is marked edited before discovery, so a suggestion cannot replace it. A blank partition-table area (all 0xFF) is written without erase; `erase_required` is returned only for an existing, different partition table.
- Agent `nodes` and the proxy stay live-only. `presence` is the only agent read that also returns saved offline IDs for this receiver. Its node `state` is `live`, `stale`, or `offline`. A node is `fresh` only when the source is `esp32`, the result is not simulated, `csi_status` is `active`, a sequence is present, and the age is at most 10 seconds. RSSI is included only for a node that is `live` in that reply. Offline nodes have no RSSI and `rssi_retained: false`. The summary has no Wi-Fi data, file path, or scope path. `fresh: false` does not prove presence. Simulation is not a sensor.
- `Nodes()` sets `live_state` to `ok`, `not_applicable` (source is not `esp32`), `not_running` (managed runtime not running in this process; upstream is not dialed), `unreachable`, or `invalid`. `available` is true exactly for `ok`. `presence` reports the same `live_state`. The Desktop overview treats `not_applicable` as not needed and `not_running` as not checked, and Devices explains both.
- `Read` returns a successful data payload when the following status probe fails: `fresh: false`, `simulated` from the last known status, `data_state: disconnected`, and a short `status_error` without a URL or token. The call itself is not an error. A failed data read stays an error and has no invented payload. `status` still only calls `Test`.
- `waiting_for_frames` is true when a probe reports a connected source that has not delivered its first frame. Such a probe is never `fresh`. The config connection test and the Desktop overview show it as waiting, not as success.
- `SetOperationalHint` receives `sensing_stale`, `storage_limit`, or `stop_unconfirmed` with `open` set only when that hint changes. The same state does not call again. This package does not import the planner, choose issue text, or send a notice. Disabled configuration does not close an open hint.
- `sensing_stale` opens when the integration is enabled, the effective source is `esp32`, the probe is not simulated, the previous probe was fresh or live, and the new probe is `stale`, `disconnected`, or `live_unverified` with an age above 10 seconds. It closes only when a later probe is fresh, `esp32`, and not simulated. It does not open while disabled, simulated, sourced from `wifi`, or before a fresh or live probe.
- `storage_limit` opens when a managed budget observation changes from false to true and closes when a later observation is false. External mode does not open it. A stop or disable that replaces status does not count as the budget returning to false.
- `stop_unconfirmed` opens when a non-external stop returns without releasing runtime ownership. It closes on a later stop that returns successfully or on a start that finishes. External mode does not open it.
- Pose, vital signs, model load, LoRa, camera, and timed recording stay blocked.
- The gate patches to the vendored upstream UI use checked anchors: `scripts/ruview/patch-controls.mjs`, the sensing and pose gates inline in `vendor-ui.mjs`, and `scripts/ruview/patch-honesty.mjs`. `patch-honesty.mjs` is the last stage of `vendor-ui.mjs`. A vendoring run with `--source` fails if an anchor moves; `--check` verifies only the pinned hashes. Honesty patches keep the Demo controls hidden unless the source is explicitly simulated, show `—` for invented CSI values, and label example data and upstream claims.
- The upstream iframe sandbox is `allow-scripts` only. UI responses use `connect-src 'none'`. The frame sends API calls and WebSockets by `postMessage`. The Desktop parent relays a call only when `Endpoint` allows that method and path, and only through the authenticated proxy. It also relays its own `status` and `capabilities` reads, and GET of a `.wasm` file under the RuView UI prefix, because the opaque frame cannot fetch those itself. Paths with a query or fragment are refused before any network call. Replies without a matching id are ignored. The parent accepts a message only when `event.origin` is `null` and the source is that frame. It replies to that frame with target origin `*`, because the browser rejects target origin `null`; the frame accepts a reply only from its parent window. Serial, camera, and microphone stay off the frame. Tour completion still uses only `aurago.ruview.onboarding.done`, seeded from the frame name.

## Work Guidance

German UI copy uses Du and real umlauts. All 16 `ui/lang/ruview/` locales get real translations. The names RuView, CSI, RSSI, UDP, and dBm may stay unchanged.

## Verification

- `go test ./internal/ruview/ ./internal/agent/ -count=1 -run 'TestRuView|TestKnownNodes'`
- `go test ./internal/ruview/ ./internal/planner/ ./internal/server/ -count=1 -run 'TestRuView|TestOperationalIssue'`
- `go test ./internal/ruview/ -count=1 -run 'TestStatusUDPFieldsStayPassive|TestRecordingAcknowledgementFollowsForwardedResults|TestStatusIsPassive|TestReadOnlyStops|TestDashboardHealth'`
- `node scripts/ruview/browser.test.mjs`
- `node scripts/ruview/wizard-browser.test.mjs`

## Child DOX Index

None.
