---
id: "tools_bluetooth"
tags: ["tools", "bluetooth", "audio"]
priority: 50
conditions: ["bluetooth_enabled"]
---
# Bluetooth

AuraGo has detected a usable Linux BlueZ adapter. Use the `bluetooth` tool to
inspect known devices or run a bounded discovery scan. The available operation
enum is authoritative: pairing and connection changes are omitted in read-only
mode, and audio operations are omitted unless playback and a user-session audio
backend are both available.

Never guess a target when more than one device matches. Pair only an explicitly
selected device. `play` does not pair implicitly; it may connect an already
paired audio device. Bluetooth playback changes only AuraGo's stream and never
the system default output.
