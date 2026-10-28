---
id: "tool_chromecast"
tags: ["tool"]
priority: 50
---
# Chromecast

Control Chromecast and Google Cast devices that are in the inventory or the latest discovery. Discover devices, play a direct media file, speak via TTS, pause, resume, seek, mute, stop, set volume, and read status.
A device address must be a private address from the inventory or from discovery in the last 30 seconds. An arbitrary IP is rejected. Groups are labeled and are not controllable. YouTube pages are unsupported. `stream_type` `live` is only a hint; the receiver load stays buffered.
`chromecast.read_only` blocks play, speak, stop, volume, pause, resume, seek, and mute. Discover and status stay available.
`chromecast.advertise_host` is the LAN host Cast devices use for AuraGo media. Leave it empty and AuraGo picks one. `127.0.0.1` is never published.

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
Text longer than 500 runes is truncated.

### Pause, resume, and seek
```json
{"action": "chromecast", "operation": "pause", "device_name": "Living Room"}
```

```json
{"action": "chromecast", "operation": "resume", "device_name": "Living Room"}
```

```json
{"action": "chromecast", "operation": "seek", "device_name": "Living Room", "position": 30}
```

### Stop playback
```json
{"action": "chromecast", "operation": "stop", "device_name": "Living Room"}
```

### Set volume (0.0–1.0)
`volume` is required. `0` is a real level.
```json
{"action": "chromecast", "operation": "volume", "device_name": "Living Room", "volume": 0.5}
```

### Mute
`muted` is required.
```json
{"action": "chromecast", "operation": "mute", "device_name": "Living Room", "muted": true}
```

### Get status
```json
{"action": "chromecast", "operation": "status", "device_name": "Living Room"}
```

## Parameters
| Field | Required | Description |
|-------|----------|-------------|
| `operation` | yes | discover, play, speak, stop, volume, pause, resume, seek, mute, status |
| `device_name` | For all except discover | Friendly device name from the inventory or latest discovery |
| `device_addr` | Alternative to device_name | Private IP of a known device. An unknown or public address is rejected |
| `device_port` | no | Default 8009. Only 8009 or the stored inventory port is accepted |
| `url` | For play | Direct HTTP(S) media file. YouTube pages are rejected. Private LAN URLs require `chromecast.media_host_allowlist` unless they are AuraGo `/tts/` or `/cast-media/` URLs |
| `local_path` | For play | Local workspace audio/video/image file, published under `/cast-media/` for 3 hours. A file over 1 GiB is rejected |
| `stream_type` | no | `live` is echoed as a hint and does not switch the receiver to a live load |
| `text` | For speak | Text to speak. Longer than 500 runes is truncated |
| `volume` | For volume | 0.0 to 1.0. Omitting it is an error. 0 does not mean "missing" |
| `position` | For seek | Target position in seconds |
| `muted` | For mute | true or false |
| `content_type` | no | MIME type (default audio/mpeg). Speak uses the audio file extension, so WAV is `audio/wav` |
| `language` | no | TTS language override |

## Workflow
1. Registered devices can be addressed by name.
2. If no devices are registered, use `discover`, then `device_addr` from that result within 30 seconds.
3. Playback returns an error when the device does not confirm PLAYING, BUFFERING, PAUSED, or a finished clip.
4. Groups can be discovered and queried. They cannot be changed.
