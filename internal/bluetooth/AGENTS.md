# Bluetooth

## Purpose

Native Bluetooth discovery, permissions, and playback.

## Ownership

`internal/bluetooth` owns this domain. The contracts below also bind related server, UI, config, asset, and test work through the root routing table.

## Local Contracts

### Bluetooth Integration Contract
- Linux Bluetooth uses BlueZ over the system D-Bus. Startup detection stays passive. The live session (`live.go`, `bus_linux.go`) only watches ObjectManager, PropertiesChanged and NameOwnerChanged signals and never starts discovery, pairs, connects or changes audio defaults on its own.
- `readonly` and `allow_playback` restrict only `ActorAgent`. Authenticated operators (`ActorOperator`: admin API, desktop app, config page, MeshCore pairing) are not bound by them. `enabled: false`, Docker and non-Linux disable everything for everyone.
- The native `bluetooth` tool exists only with a powered usable adapter and pairs Just Works only, through a short-lived per-call agent (`NoInputNoOutput`, or `KeyboardOnly` for an admin-supplied PIN). Agents can never request interactive pairing or discoverability.
- The interactive `KeyboardDisplay` agent is registered only while an operator's interactive pairing runs or a discoverable window (60–600 s) is open; only the discoverable window claims the default-agent role. It uses its own D-Bus connection.
- Pairing questions go through `interactionBroker`: one open question, 20 s deadline, then rejection. Passkeys and PINs are never logged, persisted, or put into events or LLM tool schemas; they are returned only by the admin-protected interaction endpoint and dropped on close.
- Desktop events (`bluetooth_changed {revision}`, `bluetooth_interaction {id}`, `desktop_changed app_availability`) carry no device data. The desktop app is gated by `Requires: ["bluetooth"]`, which is true while an adapter exists that is not hard-blocked.
- Bluetooth playback accepts workspace-local files or audio/music Media Registry IDs only, never URLs. It may connect an already paired target but must never pair implicitly. Route only AuraGo's stream to the matched sink, keep the system default output unchanged, and allow at most one AuraGo-owned playback.
- Standard Docker installations must report Bluetooth unavailable unless a future explicit and security-reviewed host D-Bus/audio passthrough contract is added.
- Live Speech headsets (`headset_*.go`): a `HeadsetLink` exists only while a Live Speech session has its bridge open (`internal/server/realtime_speech_headset.go`). It switches to an HFP profile, records with `pw-record`, plays with `pw-play`, always restores the previous profile on Close, serves one browser per headset, and never stores or logs audio. PipeWire only.
- Host preparation happens only through the `AURAGO-BLUETOOTH-KIT` block (`scripts/aurago-bluetooth.sh`, copied into `install.sh`, `update.sh` and `install_service_linux.sh` by `scripts/sync-bluetooth-kit.sh`). It never removes packages, linger or WirePlumber files, changes `config.yaml` only when the stored decision in `data/bluetooth-setup` changes, and never blocks installation, update or service start. Covered by `internal/audit/bluetooth_kit_test.go`.

## Verification

- Run `go test ./internal/bluetooth` and the named cross-component checks in the contracts above when those paths change.
- Headset changes: `go test ./internal/bluetooth -run Headset`, `go test ./internal/server -run RealtimeSpeech`, and with `AURAGO_RUN_BROWSER_SMOKE=1` `go test ./ui -run 'RealtimeSpeech.*Browser'`.
- Installer kit changes: `bash scripts/sync-bluetooth-kit.sh`, then `go test ./internal/audit -run 'BluetoothKit|PreparesBluetooth|AppliesBluetoothChoice' -count=1`.

## Child DOX Index

None.
