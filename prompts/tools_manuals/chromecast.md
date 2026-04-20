---
id: "tool_chromecast"
tags: ["tool"]
priority: 50
---
# Chromecast

Control Chromecast speakers on the local network. Discover devices, play audio, speak via TTS, control volume.
Chromecast devices can be registered in the device registry with friendly names (e.g. "Living Room", "Kitchen"). Use `device_name` to address devices by name instead of IP address.

## Operations

### Discover devices
```json
{"action": "chromecast", "operation": "discover"}
```

### Play audio URL
```json
{"action": "chromecast", "operation": "play", "device_name": "Living Room", "url": "https://example.com/song.mp3"}
```

### Play local audio file
```json
{"action": "chromecast", "operation": "play", "device_name": "Living Room", "local_path": "workdir/song.mp3"}
```

### Speak text (TTS → Chromecast)
```json
{"action": "chromecast", "operation": "speak", "device_name": "Living Room", "text": "Dinner is ready"}
```
⚠️ Max 200 characters for text.

### Stop playback
```json
{"action": "chromecast", "operation": "stop", "device_name": "Living Room"}
```

### Set volume (0.0–1.0)
```json
{"action": "chromecast", "operation": "volume", "device_name": "Living Room", "volume": 0.5}
```

### Get status
```json
{"action": "chromecast", "operation": "status", "device_name": "Living Room"}
```

## Parameters
| Field | Required | Description |
|-------|----------|-------------|
| `operation` | ✅ | discover, play, speak, stop, volume, status |
| `device_name` | For all except discover | Friendly device name from device registry (e.g. "Living Room") — resolved to IP automatically |
| `device_addr` | Alternative to device_name | Direct IP address (use device_name when possible) |
| `device_port` | ❌ | Default: 8009 |
| `url` | For play | Media URL |
| `local_path` | For play | Local workspace audio file; AuraGo publishes it on the LAN automatically |
| `text` | For speak | Text to speak (max 200 chars) |
| `volume` | For volume | 0.0 to 1.0 |
| `content_type` | ❌ | MIME type (default: audio/mpeg) |
| `language` | ❌ | TTS language override |

## Workflow
1. Registered devices can be addressed by name directly (e.g. `device_name: "Living Room"`)
2. If no devices are registered, use `discover` → find devices → use `device_addr` from results
3. `speak` / `play` / `volume` / `stop`
