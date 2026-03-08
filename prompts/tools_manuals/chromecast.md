---
id: "tool_chromecast"
tags: ["tool"]
priority: 50
---
# Chromecast

Control Chromecast speakers on the local network. Discover devices, play audio, speak via TTS, control volume.

## Operations

### Discover devices
```json
{"action": "chromecast", "operation": "discover"}
```

### Play audio URL
```json
{"action": "chromecast", "operation": "play", "device_addr": "192.168.1.50", "url": "https://example.com/song.mp3"}
```

### Speak text (TTS → Chromecast)
```json
{"action": "chromecast", "operation": "speak", "device_addr": "192.168.1.50", "text": "Dinner is ready"}
```
⚠️ Max 200 characters for text.

### Stop playback
```json
{"action": "chromecast", "operation": "stop", "device_addr": "192.168.1.50"}
```

### Set volume (0.0–1.0)
```json
{"action": "chromecast", "operation": "volume", "device_addr": "192.168.1.50", "volume": 0.5}
```

### Get status
```json
{"action": "chromecast", "operation": "status", "device_addr": "192.168.1.50"}
```

## Parameters
| Field | Required | Description |
|-------|----------|-------------|
| `operation` | ✅ | discover, play, speak, stop, volume, status |
| `device_addr` | For all except discover | IP address from discover |
| `device_port` | ❌ | Default: 8009 |
| `url` | For play | Media URL |
| `text` | For speak | Text to speak (max 200 chars) |
| `volume` | For volume | 0.0 to 1.0 |
| `content_type` | ❌ | MIME type (default: audio/mpeg) |
| `language` | ❌ | TTS language override |

## Workflow
1. `discover` → find devices
2. Use `device_addr` from results
3. `speak` / `play` / `volume` / `stop`
