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
- Returns `{"status": "success", "file": "hash.ext", "url": "http://..."}`
- Audio files are cached by content hash
- Audio is automatically sent as native attachment in Telegram/Discord
- Combine with `chromecast` action `speak` to play on speakers

## MiniMax Speech Control Tags (speech-2.8-hd and speech-2.8-turbo only)
Use these inline in the `text` parameter to control speech dynamics:

| Tag | Effect |
|------|--------|
| `<#1.5#>` | Pause for 1.5 seconds (replace number with desired duration) |
| `(laughs)` | Laughter |
| `(chuckle)` | Chuckle |
| `(coughs)` | Coughing |
| `(clear-throat)` | Clearing throat |
| `(groans)` | Groaning |
| `(breath)` | Breathing sound |
| `(pant)` | Panting |
| `(inhale)` | Inhale |
| `(exhale)` | Exhale |
| `(gasps)` | Gasp |
| `(sniffs)` | Sniff |
| `(sighs)` | Sigh |
| `(snorts)` | Snort |
| `(burps)` | Burp |
| `(lip-smacking)` | Lip smacking |
| `(humming)` | Humming |
| `(hissing)` | Hissing |
| `(emm)` | Thinking sound ("emm") |
| `(sneezes)` | Sneeze |

Example: `"Hello! <#0.5#> (laughs) That's great news."`
