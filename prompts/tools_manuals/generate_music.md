# generate_music

Generate music from text prompts using AI music generation models.

## Supported Providers
- **ACE-Step 1.5 local** — managed Docker runtime; no cloud key, GPU preferred, CPU opt-in
- **MiniMax** (`music-2.5+`, `music-2.5`) — High-quality AI music generation with lyrics support
- **Google Lyria** (`lyria-3-clip-preview`, `lyria-3-pro-preview`) — Google's music generation via Gemini API

## Parameters
| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `prompt` | string | yes | Description of the music to generate (style, mood, genre, instruments) |
| `lyrics` | string | no | Song lyrics in tagged format: `[Verse]`, `[Chorus]`, `[Bridge]`, etc. |
| `instrumental` | boolean | no | Set to `true` for instrumental music without vocals (default: false) |
| `title` | string | no | Title for the generated track |
| `duration_seconds` | number | no | ACE-Step: default 120, minimum 10, maximum the active profile limit (at most 600) |
| `bpm` | integer | no | ACE-Step: 30–300; omit for automatic selection |
| `vocal_language` | string | no | ACE-Step: language code such as `de`, `en`, or `ja`; omit for automatic selection |
| `seed` | integer | no | ACE-Step: 0–2147483647; omit for random, zero is valid |

## Examples

### Generate instrumental background music
```json
{
  "prompt": "Calm lo-fi hip hop beat with soft piano and vinyl crackle",
  "instrumental": true,
  "title": "Study Vibes"
}
```

### Generate a song with lyrics
```json
{
  "prompt": "Upbeat pop rock song with electric guitar and drums",
  "lyrics": "[Verse]\nWalking down the street today\nSun is shining all the way\n[Chorus]\nFeel the rhythm, feel the beat\nMoving to the sound so sweet",
  "title": "Sunshine Walk"
}
```

## Output
The tool saves the generated audio as MP3 in `data/audio/` and registers it in the media registry. Returns:
- `file_path` — Local path to the audio file
- `web_path` — Web-accessible URL path (e.g. `/data/audio/music_xxx.mp3`)
- `duration_ms` — Duration in milliseconds (when available)
- `provider` / `model` — Which provider and model were used
- `media_id` — ID in the media registry

## Notes
- Local ACE-Step must be ready in Music Generation settings. One job runs at a time; `acestep_busy` means another job owns the worker. Do not change containers or retry in a tight loop.
- Local lyrics generation requires a loaded local language model. Otherwise provide lyrics or choose instrumental. Never silently use a cloud LLM to work around `lyrics_required`.
- Local music costs zero and retains the daily quantity limit. Cancellation/timeout recycles only its dedicated container; model downloads remain cached.
- A daily generation limit can be configured (0 = unlimited)
- Audio files are saved in MP3 format
- Generated music is automatically registered in the media registry with `media_type: "music"` and `source_tool: "generate_music"`
- The media registry entry can be used by other tools (e.g., homepage tool)
- MiniMax supports detailed lyrics with section tags; Google Lyria generates based on text prompts
