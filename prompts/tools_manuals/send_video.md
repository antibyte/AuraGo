## Tool: Send Video (`send_video`)

Use `send_video` when the user asks to see an existing video file in the chat, or when you have created/downloaded a video manually and need to present it in the WebUI.

## Behavior

- Shows an inline video player in the WebUI chat.
- Copies local files into `data/generated_videos/` so they can be served safely.
- Downloads direct HTTP(S) video URLs into `data/generated_videos/`.
- Registers the video in the media registry.

## Arguments

- `path` (required): workspace-relative path, absolute local path, or direct HTTP(S) URL.
- `title` (optional): display title above the player.

Supported browser-friendly formats: MP4, WebM, MOV, M4V, OGV/OGG.

## Examples

```json
{"action": "send_video", "path": "clips/demo.mp4", "title": "Demo clip"}
```

```json
{"action": "send_video", "path": "https://example.com/video.mp4", "title": "Reference video"}
```
