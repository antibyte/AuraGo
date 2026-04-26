## Tool: Send YouTube Video (`send_youtube_video`)

Use `send_youtube_video` when the user asks to send or play a YouTube video in chat.

## Behavior

- Requires `tools.send_youtube_video.enabled: true`.
- Web Chat: emits a YouTube embed event so the UI renders an inline player.
- Telegram, Discord, and other text channels: delivers the canonical YouTube link.
- Does not download, mirror, or store the YouTube video file.
- Uses `youtube-nocookie.com` for the Web Chat embed.

## Arguments

- `url` (required): YouTube URL. Supports `youtube.com/watch?v=...`, `youtu.be/...`, `/shorts/...`, `/live/...`, and `/embed/...`.
- `title` (optional): title shown above the embedded player or before the text-channel link.
- `start_seconds` (optional): playback start offset in seconds. Overrides `t=` or `start=` from the URL.

## Examples

```json
{"action": "send_youtube_video", "url": "https://www.youtube.com/watch?v=dQw4w9WgXcQ", "title": "Reference video"}
```

```json
{"action": "send_youtube_video", "url": "https://youtu.be/dQw4w9WgXcQ?t=43", "title": "Start at chorus"}
```

## Notes

- Use `send_video` only for direct video files such as MP4 or WebM.
- Use `send_youtube_video` for YouTube pages and Shorts links.
- Never try to download a YouTube video just to show it in chat.
