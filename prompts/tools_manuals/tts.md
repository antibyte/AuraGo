---
id: "tool_tts"
tags: ["tool"]
priority: 50
---
# TTS (Text-to-Speech)

Generate speech audio from text. Max 500 characters per call.

## Usage
```json
{"action": "tts", "text": "Hello, how are you?", "language": "en"}
```

## Parameters
| Field | Required | Description |
|-------|----------|-------------|
| `text` | ✅ | Text to synthesize (max 500 chars) |
| `language` | ❌ | BCP-47 code (e.g. "de", "en"). Default: from config |

## Notes
- Provider is configured in Speech Output settings → `tts.provider` ("sanotts", "google", "elevenlabs", "minimax", "mistral", "xai", "openai", "stepfun", "resemble", "piper", or "supertonic"). API keys are stored only in the Vault.
- New installations use local CPU sanoTTS and `language: auto`. It follows the user language; explicit speech language settings take priority. Available languages: ar, cs, de, en, es, fr, id, it, pt, ro, ru, tr, vi. Other languages use an English voice; provide English speech text for those languages. Text is synthesized as supplied, never automatically translated by TTS.
- sanoTTS installs its Python runtime dependencies and downloads the selected tiny voice on first use, then runs locally without a key, Docker, or GPU. Output is `.wav`.
- If `tts.piper.enabled` is true and no provider is set, Piper is used automatically
- Piper TTS runs as a Docker container (auto-managed) and produces `.wav` files
- Supertonic TTS runs through a managed Docker sidecar when `tts.provider: supertonic` and `tts.supertonic.auto_start: true`; it supports `.wav`, `.flac`, and `.ogg`
- Google/ElevenLabs/MiniMax/Mistral/xAI/OpenAI/StepFun/Resemble produce `.mp3` files. xAI defaults to `eve`; OpenAI to `gpt-4o-mini-tts`/`coral`; StepFun to `step-tts-mini`/`jingdiannvsheng`. Resemble requires an account voice UUID.
- OpenAI Speech API models retire on 2027-01-06; use OpenAI Realtime for new long-term integrations. Resemble's former hosted voice platform is archived; its adapter requires access that still works. See `documentation/tts-providers.md`.
- Returns `{"status": "success", "file": "hash.ext", "url": "http://...", "local_path": "/abs/path/to/file"}`
- Audio files are cached by content hash — the cache may be evicted over time
- Audio is automatically sent as native attachment in Telegram/Discord
- Combine with `chromecast` action `speak` to play on speakers
- **⚠️ TTS audio is automatically posted to the WebUI chat when generated.** Do NOT call `send_audio` after `tts` — that would send it twice. Only use `send_audio` for audio files that are NOT from the `tts` tool.
