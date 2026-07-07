---
id: "tool_chromecast"
tags: ["tool"]
priority: 50
---
# Chromecast

Control Chromecast and Google Cast devices on the local network. Discover devices, play audio/video/image media, speak via TTS, control volume.
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

### Play direct video URL
```json
{"action": "chromecast", "operation": "play", "device_name": "Living Room", "url": "https://example.com/movie.mp4", "content_type": "video/mp4"}
```

### Play local media file
```json
{"action": "chromecast", "operation": "play", "device_name": "Living Room", "local_path": "workdir/song.mp3"}
```

```json
{"action": "chromecast", "operation": "play", "device_name": "Living Room", "local_path": "workdir/clip.webm"}
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
| `url` | For play | Direct HTTP(S) media URL. Public URLs are SSRF-protected; private LAN URLs require `chromecast.media_host_allowlist` unless they are AuraGo-generated `/tts/` or `/cast-media/` URLs. |
| `local_path` | For play | Local workspace audio/video/image file; AuraGo publishes it under `/cast-media/` on the LAN automatically |
| `text` | For speak | Text to speak (max 200 chars) |
| `volume` | For volume | 0.0 to 1.0 |
| `content_type` | ❌ | MIME type (default: audio/mpeg). Direct video URLs should specify `video/mp4` or `video/webm`. Local `.mp4`/`.webm` files are detected automatically. Unsupported local extensions require this field. |
| `language` | ❌ | TTS language override |

## Workflow
1. Registered devices can be addressed by name directly (e.g. `device_name: "Living Room"`)
2. If no devices are registered, use `discover` → find devices → use `device_addr` from results
3. `speak` / `play` / `volume` / `stop`
