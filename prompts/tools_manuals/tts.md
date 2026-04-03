---
id: "tool_tts"
tags: ["tool"]
priority: 50
---
# TTS (Text-to-Speech)

Generate speech audio from text. Max 200 characters per call.

## Usage
```json
{"action": "tts", "text": "Hello, how are you?", "language": "en"}
```

## Parameters
| Field | Required | Description |
|-------|----------|-------------|
| `text` | ✅ | Text to synthesize (max 200 chars) |
| `language` | ❌ | BCP-47 code (e.g. "de", "en"). Default: from config |

## Notes
- Provider is configured in `config.yaml` → `tts.provider` ("google", "elevenlabs", "minimax", or "piper")
- If `tts.piper.enabled` is true and no provider is set, Piper is used automatically
- Piper TTS runs as a Docker container (auto-managed) and produces `.wav` files
- Google/ElevenLabs/MiniMax produce `.mp3` files
- Returns `{"status": "success", "file": "hash.ext", "url": "http://...", "local_path": "/abs/path/to/file"}`
- Audio files are cached by content hash — the cache may be evicted over time
- Audio is automatically sent as native attachment in Telegram/Discord
- Combine with `chromecast` action `speak` to play on speakers
- **To post TTS audio to chat:** use `send_audio` with `path` set to the `local_path` from this result (not the URL). The local path is always available even if the TTS cache was evicted.


