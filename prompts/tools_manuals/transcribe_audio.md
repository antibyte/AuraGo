## Tool: Speech-to-Text / Audio Transcription (`transcribe_audio`)

Transcribe an audio file to text using the configured Whisper/STT service. Supports standard audio formats.

### Schema

| Parameter | Type | Required | Description |
|---|---|---|---|
| `file_path` | string | yes | Path to the audio file |

### Examples

**Transcribe a voice recording:**
```json
{"action": "transcribe_audio", "file_path": "agent_workspace/workdir/recording.mp3"}
```

**Transcribe a wav file:**
```json
{"action": "transcribe_audio", "file_path": "agent_workspace/workdir/meeting.wav"}
```

### Notes
- Supported formats: MP3, WAV, OGG, FLAC, M4A, WebM
- Uses the Whisper API configured in `config.yaml` (whisper section)
- If `whisper.provider` is set to `multimodal`, a multimodal LLM (e.g., Gemini) is used instead of the standard Whisper API
- OGG files from Telegram may need conversion to MP3/WAV first (use `execute_shell` with ffmpeg)
- The transcription language is auto-detected
