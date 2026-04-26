---
id: video_download
tags: [media, integration]
priority: 75
conditions: ["tools.video_download.enabled"]
---

# Video Download (`video_download`)

Search, inspect, download, and transcribe videos using yt-dlp.

Search and metadata lookup are read-only. Download and transcription are separate opt-in permissions.

## Backend

- Default: Docker mode runs `ghcr.io/jauderho/yt-dlp:latest` in an ephemeral container.
- Native mode is optional and requires `yt-dlp` to be installed on the host or configured via `tools.video_download.yt_dlp_path`.
- `tools.video_download.enabled: true` exposes `search` and `info`.
- `download` requires `tools.video_download.allow_download: true`.
- `transcribe` requires `tools.video_download.allow_transcribe: true`.
- `tools.video_download.readonly: true` blocks `download` and `transcribe` even when their opt-in flags are enabled.

## Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `operation` | string | yes | `search`, `info`, and optionally `download` / `transcribe` when enabled |
| `query` | string | for `search` | Search query |
| `url` | string | for `info`, `download`, `transcribe` | Video URL |
| `format` | string | no | `video`, `audio`, `best`, `bestaudio`, or custom yt-dlp format |
| `quality` | string | no | `best`, `medium`, or `low` for video downloads |

## Examples

Search for videos:

```json
{"action":"video_download","operation":"search","query":"home assistant proxmox tutorial"}
```

Inspect a video:

```json
{"action":"video_download","operation":"info","url":"https://www.youtube.com/watch?v=..."}
```

Download audio only:

```json
{"action":"video_download","operation":"download","url":"https://www.youtube.com/watch?v=...","format":"audio"}
```

Download and transcribe:

```json
{"action":"video_download","operation":"transcribe","url":"https://www.youtube.com/watch?v=...","format":"audio"}
```

## Notes

- Downloaded files are stored under `tools.video_download.download_dir` and exposed as `/files/downloads/...`.
- `readonly: true` allows only `search` and `info`.
- Transcription uses the existing AuraGo speech-to-text configuration.
