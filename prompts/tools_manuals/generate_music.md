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
| `lyrics` | string | no | Song lyrics with section tags in square brackets, each on its own line. The tags depend on the provider; see "Lyrics tags by provider". |
| `instrumental` | boolean | no | Set to `true` for instrumental music without vocals (default: false) |
| `title` | string | no | Title for the generated track |
| `duration_seconds` | number | no | ACE-Step: default 120, minimum 10, maximum the active profile limit (at most 600) |
| `bpm` | integer | no | ACE-Step: 30–300; omit for automatic selection |
| `vocal_language` | string | no | ACE-Step: language code such as `de`, `en`, or `ja`; omit for automatic selection |
| `seed` | integer | no | ACE-Step: 0–2147483647; omit for random, zero is valid |

## Lyrics tags by provider
Put each section tag on its own line in square brackets and separate sections with a blank line. Never write repeat notes such as `(x2)`; write repeated lines out.

### ACE-Step 1.5 (local)
- Section tags: `[Intro]`, `[Verse]` (numbered: `[Verse 1]`), `[Pre-Chorus]`, `[Chorus]`, `[Bridge]`, `[Outro]`, `[Build]`, `[Drop]`, `[Breakdown]`, `[Instrumental]`, `[Guitar Solo]`, `[Piano Interlude]`, `[Fade Out]`, `[Silence]`. Short free-form names such as `[Final Chorus]` also work.
- One short modifier after ` - ` is allowed: `[Chorus - anthemic]`, `[Verse - whispered]`. More than two modifiers get sung.
- Vocal and energy tags on their own line: `[raspy vocal]`, `[whispered]`, `[falsetto]`, `[powerful belting]`, `[spoken word]`, `[harmonies]`, `[call and response]`, `[ad-lib]`, `[high energy]`, `[low energy]`, `[building energy]`, `[explosive]`, `[melancholic]`, `[euphoric]`, `[dreamy]`, `[aggressive]`.
- `(parentheses)` are sung as backing vocals, UPPERCASE means more intensity, and 6-10 syllables per line work best.
- Put the tempo in `bpm`, not in `prompt`, and keep the tags consistent with the prompt.

### MiniMax
- Only these tags, spelled exactly: `[Intro]`, `[Verse]`, `[Pre Chorus]`, `[Chorus]`, `[Interlude]`, `[Bridge]`, `[Outro]`, `[Post Chorus]`, `[Transition]`, `[Break]`, `[Hook]`, `[Build Up]`, `[Inst]`, `[Solo]`. No numbers, no modifiers.
- At most 3500 characters of lyrics; longer lyrics are rejected before the provider call (`invalid_lyrics_length`).

### Google Lyria
- No separate lyrics field: lyrics are passed inside the text prompt, so keep them short.

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
- The administrator can select CUDA, ROCm, Intel XPU, Vulkan or explicit CPU. Vulkan uses the managed acestep.cpp/GGUF runtime on Linux; use the same tool parameters and the active runtime's duration/lyrics limits.
- Automatic hardware discovery uses CUDA/Vulkan on the Docker host. ROCm/XPU require an explicitly selected backend or concrete device, avoiding speculative multi-gigabyte image downloads. CPU still requires explicit selection.
- Local lyrics generation requires a loaded local language model. Otherwise provide lyrics or choose instrumental. Never silently use a cloud LLM to work around `lyrics_required`.
- Local music costs zero and retains the daily quantity limit. Cancellation/timeout recycles only its dedicated container; model downloads remain cached.
- A daily generation limit can be configured (0 = unlimited)
- Audio files are saved in MP3 format
- Generated music is automatically registered in the media registry with `media_type: "music"` and `source_tool: "generate_music"`
- The media registry entry can be used by other tools (e.g., homepage tool)
- MiniMax and ACE-Step use different lyric tags (see "Lyrics tags by provider"); Google Lyria generates from the text prompt
