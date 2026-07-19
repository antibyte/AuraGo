---
id: "tool_bluetooth"
tags: ["tool", "bluetooth", "audio"]
priority: 50
---
# Bluetooth

Discover and control Bluetooth devices through the usable Linux BlueZ adapter.
The schema is built dynamically from the current runtime and permissions.

## Operations

- `status`: Return adapter, audio backend, and AuraGo playback state.
- `list`: List devices already known to BlueZ without scanning.
- `discover`: Run a bounded scan. `timeout_seconds` is limited to 60 seconds.
- `pair`: Pair the explicitly selected `device` using BlueZ “Just Works”.
- `connect` / `disconnect`: Change connection state for the selected device.
- `play`: Play exactly one `local_path` or audio/music `media_id`.
- `speak`: Generate speech with the configured TTS provider and play it.
- `playback_status`: Inspect AuraGo's asynchronous Bluetooth playback.
- `stop`: Stop only the playback stream started by AuraGo.

The operation enum contains only operations currently allowed. Pair/connect/
disconnect require `bluetooth.readonly: false`; audio operations require
`bluetooth.allow_playback: true` and a usable PipeWire or PulseAudio backend.

## Examples

```json
{"action":"bluetooth","operation":"discover","timeout_seconds":10}
```

```json
{"action":"bluetooth","operation":"pair","device":"AA:BB:CC:DD:EE:FF"}
```

```json
{"action":"bluetooth","operation":"play","device":"Living Room","local_path":"workdir/music/song.mp3"}
```

```json
{"action":"bluetooth","operation":"speak","device":"AA:BB:CC:DD:EE:FF","text":"Das Essen ist fertig.","language":"de"}
```

URLs and streaming-service identifiers are not accepted. The agent-facing tool
never accepts a PIN; an optional transient PIN is available only in the admin
UI and is neither persisted nor logged.
