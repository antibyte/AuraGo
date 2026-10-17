# RuView sensing

## Purpose

Pinned RuView v3122 integration: managed or external runtime, passive status, authenticated proxy, and board setup.

## Ownership

This package owns the runtime client, proxy policy, node inventory and passive status. `internal/server` owns the HTTP routes and admin gates. `ui/js/desktop/apps/ruview.js` and `ui/js/ruview/wizard.mjs` own the Desktop app and board wizard. `documentation/ruview.md` owns operator guidance. The vendored upstream UI stays untranslated.

## Local Contracts

- `Status()` is passive. It does not dial, resolve DNS, or read the node file. Opening the app does not probe or start a runtime.
- Enabling RuView does not start a runtime in an already running session. A manual stop, process exit, or uncertain stop is not restarted automatically.
- Passive fields, copied from saved configuration and this process:
  - `udp_enabled` follows the saved flag and is false for external mode.
  - `udp_ready` is true only for a managed runtime with UDP enabled, a private bind, and at least one allowlist entry.
  - `udp_scope` is `managed`, `external`, or `off`. External means the operator runs reception; it is not a failure. Disabled configuration is `off`.
  - `receiver_configured` is true when the saved receiver is a private IPv4 address.
  - `recording_session` is `unknown`, `started`, or `stopped` for this process only. It starts as `unknown`.
  - `recording_ack` is empty or `unconfirmed`.
- The recording marker lives on the manager. `Configure()` clears cached probe status and must not clear a confirmed session.
- `noteRecording` runs only after `/api/v1/recording/start` or `/api/v1/recording/stop` is forwarded. A 2xx start sets `started` and clears the acknowledgement. A 2xx stop sets `stopped` and clears it. Any other forwarded result, including a transport failure or an invalid body, sets `unconfirmed` and keeps the last confirmed session. A local rejection that never leaves AuraGo does not call it.
- The wizard never enables UDP and never stores Wi-Fi credentials. An uncertain flash is never retried. An explicit node ID from 1 to 255 is marked edited before discovery, so a suggestion cannot replace it.
- Agent `nodes` and the proxy stay live-only. `presence` is the only agent read that also returns saved offline IDs for this receiver. Its node `state` is `live`, `stale`, or `offline`. A node is `fresh` only when the source is `esp32`, the result is not simulated, `csi_status` is `active`, a sequence is present, and the age is at most 10 seconds. RSSI is included only for a node that is `live` in that reply. Offline nodes have no RSSI and `rssi_retained: false`. The summary has no Wi-Fi data, file path, or scope path. `fresh: false` does not prove presence. Simulation is not a sensor.
- `Read` returns a successful data payload when the following status probe fails: `fresh: false`, `simulated` from the last known status, `data_state: disconnected`, and a short `status_error` without a URL or token. The call itself is not an error. A failed data read stays an error and has no invented payload. `status` still only calls `Test`.
- `SetOperationalHint` receives `sensing_stale`, `storage_limit`, or `stop_unconfirmed` with `open` set only when that hint changes. The same state does not call again. This package does not import the planner, choose issue text, or send a notice. Disabled configuration does not close an open hint.
- `sensing_stale` opens when the integration is enabled, the effective source is `esp32`, the probe is not simulated, the previous probe was fresh or live, and the new probe is `stale`, `disconnected`, or `live_unverified` with an age above 10 seconds. It closes only when a later probe is fresh, `esp32`, and not simulated. It does not open while disabled, simulated, sourced from `wifi`, or before a fresh or live probe.
- `storage_limit` opens when a managed budget observation changes from false to true and closes when a later observation is false. External mode does not open it. A stop or disable that replaces status does not count as the budget returning to false.
- `stop_unconfirmed` opens when a non-external stop returns without releasing runtime ownership. It closes on a later stop that returns successfully or on a start that finishes. External mode does not open it.
- Pose, vital signs, model load, LoRa, camera, and timed recording stay blocked.
- The upstream iframe keeps `allow-scripts allow-same-origin` until a separate isolation change.

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
