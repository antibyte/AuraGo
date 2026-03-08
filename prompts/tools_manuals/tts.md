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
- Provider is configured in `config.yaml` → `tts.provider` ("google" or "elevenlabs")
- Returns `{"status": "success", "file": "hash.mp3", "url": "http://..."}` 
- Audio files are cached by content hash
- Combine with `chromecast` action `speak` to play on speakers
