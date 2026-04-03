## Tool: Send Audio (`send_audio`)

Send an audio file to the user. In the Web UI it appears as an inline audio player with play/pause, progress bar, speed control, and a download button. Provide a local workspace path or a direct HTTPS URL to an audio file.

### Schema

| Parameter | Type | Required | Description |
|---|---|---|---|
| `path` | string | yes | Local workspace path (e.g. `output.mp3`) **or** a full HTTPS URL to an audio file |
| `title` | string | no | Optional title shown above the audio player |

### Supported Formats

MP3, WAV, OGG, FLAC, M4A, AAC, Opus, WebM

### Workflow

1. Generate or obtain an audio file (via `execute_python`, TTS tool, or skill).
2. Call `send_audio` with the workspace-relative path or a URL.
3. Include the returned `web_path` in your final response text so references persist in chat history.

### Examples

```json
{"action": "send_audio", "path": "speech_output.mp3", "title": "Generated speech"}
```

```json
{"action": "send_audio", "path": "https://example.com/sound.mp3", "title": "Reference audio"}
```

### Notes

- Audio files are copied to `data/audio/` and served at `/files/audio/...`
- URL audio is downloaded automatically — no pre-download needed
- The audio player supports all modern browsers natively (HTML5 `<audio>`)
- **On error:** try at most **one alternative path**, then inform the user. Do **not** loop.
- **For TTS audio:** prefer the `local_path` from the `tts` tool result over the URL — it is always reachable regardless of TTS cache state. If only the URL is known and it returns 404, call `tts` again to regenerate, then use the new `local_path`.
