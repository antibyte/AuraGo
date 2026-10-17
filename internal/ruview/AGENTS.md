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
- Pose, vital signs, model load, LoRa, camera, and timed recording stay blocked.
- The upstream iframe keeps `allow-scripts allow-same-origin` until a separate isolation change.

## Work Guidance

German UI copy uses Du and real umlauts. All 16 `ui/lang/ruview/` locales get real translations. The names RuView, CSI, RSSI, UDP, and dBm may stay unchanged.

## Verification

- `go test ./internal/ruview/ ./internal/agent/ -count=1 -run 'TestRuView|TestKnownNodes'`
- `go test ./internal/ruview/ -count=1 -run 'TestStatusUDPFieldsStayPassive|TestRecordingAcknowledgementFollowsForwardedResults|TestStatusIsPassive|TestReadOnlyStops|TestDashboardHealth'`
- `node scripts/ruview/browser.test.mjs`
- `node scripts/ruview/wizard-browser.test.mjs`

## Child DOX Index

None.
