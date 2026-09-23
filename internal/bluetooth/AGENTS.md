# Bluetooth

## Purpose

Native Bluetooth discovery, permissions, and playback.

## Ownership

`internal/bluetooth` owns this domain. The contracts below also bind related server, UI, config, asset, and test work through the root routing table.

## Local Contracts

### Bluetooth Integration Contract
- Linux Bluetooth uses BlueZ over the system D-Bus; startup detection must stay passive and must not start discovery, pair, connect, or alter audio defaults.
- The native `bluetooth` tool exists only with a powered usable adapter. Pair/connect/disconnect require `bluetooth.readonly: false`; play/speak/status/stop require a usable PipeWire or PulseAudio backend and `bluetooth.allow_playback: true`.
- Agent-side pairing is Just Works only. An optional numeric PIN may be passed transiently by the admin UI, but must never be stored, logged, or exposed in LLM tool schemas.
- Bluetooth playback accepts workspace-local files or audio/music Media Registry IDs only, never URLs. It may connect an already paired target but must never pair implicitly.
- Route only AuraGo's stream to the matched Bluetooth sink, keep the system default output unchanged, and allow at most one AuraGo-owned Bluetooth playback at a time.
- Standard Docker installations must report Bluetooth unavailable unless a future explicit and security-reviewed host D-Bus/audio passthrough contract is added.

## Verification

- Run `go test ./internal/bluetooth` and the named cross-component checks in the contracts above when those paths change.

## Child DOX Index

None.
