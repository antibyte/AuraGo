# Noisemaker: AI helpers with provider-aware lyric tags

Date: 2026-11-23. Status: approved by the user in chat ("Passt, weiter").

## Goal

Give the Noisemaker desktop app real AI songwriting helpers: suggest a whole song, write and revise
lyrics, check lyrics against what the active music provider understands, and propose variations of
library songs. Every LLM that writes lyrics for a provider (Noisemaker helpers and the AuraGo agent's
`generate_music` tool) must know that provider's lyric tags and limits.

User decisions (2026-11-23):

- Local ACE-Step mode: AI helpers are offered, but run only on an explicit click. Song generation in
  local mode still never calls the chat LLM.
- Features: all four — whole song, lyrics revision, tag check, variation from library.
- Interaction: menus at the fields (no assistant chat sidebar). AI results replace field content and can
  be undone.
- Agent: same tag knowledge from one shared source.

## Current state (2026-11-23)

- `internal/server/noisemaker_handlers.go`: `POST /api/desktop/noisemaker/enhance` with kinds
  `idea | style | lyrics | title` (one-shot LLM, `noisemakerEnhancePrompts`, `noisemakerRunEnhance`).
  The lyrics prompt names only `[Verse 1], [Chorus], [Bridge], [Outro]`.
- `handleNoisemakerState` reports `llm_available: s.LLMClient != nil && !local`; `handleNoisemakerEnhance`
  rejects every request in local mode. So local ACE-Step users see no AI buttons at all.
- `noisemakerMaxLyricsLength = 8000`, but MiniMax rejects non-instrumental lyrics over 3500 characters.
- Providers (`internal/tools/music_generation.go`): `acestep` (local), `minimax` (default model
  `music-2.5+`), `google`/`google_lyria` (no lyrics field; lyrics are appended to the prompt; the UI hides
  lyrics via `supports_lyrics`).
- UI: `ui/js/desktop/apps/noisemaker-create.js` renders single AI buttons per field
  (`data-nm-enhance`); `noisemaker.js` is the shell (`useTemplate`, context menus);
  `noisemaker-menus.js` builds window and track context menus (desktop menus support submenus via
  `items`).
- Agent: `prompts/tools_manuals/generate_music.md` and the tool schema in
  `internal/agent/native_tools_integrations.go` mention only `[Verse]`, `[Chorus]`, `[Bridge]`.

## Provider tag facts (sources)

ACE-Step 1.5, pinned commit `ca1e85fe9430179831e6bc6be790c332190a3866`, `docs/en/Tutorial.md` and
`docs/en/ace_step_musicians_guide.md`:

- Structure tags ("common", not exhaustive): `[Intro]`, `[Verse]`/`[Verse 1]`, `[Pre-Chorus]`, `[Chorus]`,
  `[Bridge]`, `[Outro]`, `[Build]`, `[Drop]`, `[Breakdown]`, `[Instrumental]`, `[Guitar Solo]`,
  `[Piano Interlude]`, `[Fade Out]`, `[Silence]`; free-form names such as `[Final Chorus]` appear in examples.
- Modifiers with ` - `: `[Chorus - anthemic]`, `[Bridge - whispered]`. Do not stack many: the model may sing
  tag content. Complex style belongs in the caption.
- Vocal tags: `[raspy vocal]`, `[whispered]`, `[falsetto]`, `[powerful belting]`, `[spoken word]`,
  `[harmonies]`, `[call and response]`, `[ad-lib]`. Energy tags: `[high energy]`, `[low energy]`,
  `[building energy]`, `[explosive]`, `[melancholic]`, `[euphoric]`, `[dreamy]`, `[aggressive]`.
- Text: 6-10 syllables per line; UPPERCASE = stronger intensity; `(parentheses)` are sung as backing
  vocals; sections separated by blank lines; `[Instrumental]` for pure instrumentals.
- Caption: prose or comma tags; dimensions genre, mood, instruments, timbre, era, production, vocals,
  rhythm; no BPM/key/time signature in the caption (separate parameters). Caption and lyric tags must not
  conflict.

MiniMax `POST /v1/music_generation` (platform.minimax.io API reference and guide):

- Exactly these structure tags: `[Intro]`, `[Verse]`, `[Pre Chorus]`, `[Chorus]`, `[Interlude]`, `[Bridge]`,
  `[Outro]`, `[Post Chorus]`, `[Transition]`, `[Break]`, `[Hook]`, `[Build Up]`, `[Inst]`, `[Solo]`.
  No modifiers documented. Lines separated by `\n`.
- Lyrics 1-3500 characters for non-instrumental songs. Prompt up to 2000 characters.

Google Lyria: no lyrics field; no tags.

## Design

### 1. Shared lyric guide (`internal/tools/music_lyrics_guide.go`)

```go
type LyricsGuide struct {
    Provider       string   // "acestep" | "minimax" | "google"
    SupportsLyrics bool
    MaxLyricsChars int      // minimax 3500, acestep 8000 (Noisemaker cap), google 0
    SectionTags    []string // documented structure tags, canonical spelling
    VoiceTags      []string // ACE-Step vocal + energy tags; empty for minimax
    Modifiers      bool     // "[Chorus - anthemic]" allowed (acestep)
    Numbered       bool     // "[Verse 1]" allowed (acestep)
    FreeformTags   bool     // unknown tags are info, not warnings (acestep)
    BPMInStyle     bool     // false for acestep: tempo goes to the BPM control
}

func LyricsGuideFor(providerType string) LyricsGuide
func (g LyricsGuide) PromptRules() string // compact English rules for LLM system prompts
func LintSong(g LyricsGuide, style, lyrics string) []LyricsIssue
func (g LyricsGuide) FormatAllowedTags() string // one line for error messages

type LyricsIssue struct {
    Code     string            `json:"code"`
    Severity string            `json:"severity"` // error | warning | info
    Line     int               `json:"line,omitempty"` // 1-based, 0 = whole text
    Params   map[string]string `json:"params,omitempty"`
}
```

`google`/`google_lyria` and unknown provider types get `SupportsLyrics=false` and no tags (Noisemaker
already hides lyrics for them). Tag matching is case-insensitive and treats `-`, `_` and spaces alike,
so MiniMax `[Pre-Chorus]` and `[Pre Chorus]` both pass.

Lint codes (deterministic, no LLM):

| Code | Severity | Rule |
|---|---|---|
| `lyrics_too_long` | error | `len > MaxLyricsChars` (params `length`, `max`) |
| `unknown_tag` | warning (info when `FreeformTags`) | tag not in `SectionTags`/`VoiceTags`; param `suggestion` when an alias maps (e.g. MiniMax `Pre-Chorus`→`Pre Chorus`, `Instrumental`→`Inst`, `Build`→`Build Up`, `Guitar Solo`→`Solo`, `Breakdown`→`Break`; ACE-Step `Pre Chorus`→`Pre-Chorus`, `Inst`→`Instrumental`) |
| `tag_numbered` | warning | `[Verse 2]` when `!Numbered` (suggestion `Verse`) |
| `tag_modifier_unsupported` | warning | `[Chorus - x]` when `!Modifiers` |
| `tag_too_many_modifiers` | warning | more than 2 ` - ` modifiers |
| `heading_not_tag` | warning | a line like `Chorus:`, `**Chorus**`, `## Verse 1`, `(Chorus)` that names a known section (suggestion `[Chorus]`) |
| `repeat_marker` | warning | `(x2)`, `x2`, `(2x)`, `Repeat chorus`, `Wiederholung`, `(repeat)`: these get sung |
| `no_section_tags` | info | non-empty lyrics without any tag |
| `bpm_in_style` | info | `!BPMInStyle` and style contains a tempo like `120 bpm` |

### 2. Server (`internal/server/noisemaker_handlers.go`, routes in `server_routes.go`)

- `GET /state`: `llm_available = s.LLMClient != nil` (local mode no longer hides helpers). New field
  `lyrics_guide: {provider, supports_lyrics, max_chars, section_tags, modifiers}` for the counter and the UI.
- `POST /enhance`: local-mode rejection removed. All kinds append `LyricsGuideFor(provider).PromptRules()`
  where lyrics or style are written. The `style` kind tells ACE-Step not to put BPM/key in the style.
  New request fields: `action`, `instruction` (max 500 chars), `instrumental`, `mode`, `base`
  (`{title, idea, style, lyrics}`, each capped like the generate limits).
  - `revise`: `text` (lyrics) required. `action` ∈ `catchier_chorus | shorten | rhymes | add_bridge |
    fix_tags | custom`; `custom` requires `instruction`. `fix_tags` runs `LintSong` on the server and
    sends the issues (not client text) to the LLM. Response `{status, text}`.
  - `song`: optional `text` (idea seed) and `context` (style seed), `instrumental`. The LLM returns one
    JSON object `{idea, style, title, lyrics, bpm, vocal_language}`. Response `{status, song}`.
  - `variation`: `base` required, `mode` ∈ `style | verses | sequel`. Same output as `song`.
- Song parsing: strip code fences, take the outermost `{...}`, unmarshal, cap every field to the generate
  limits, clamp `bpm` to 30-300 (else omit), validate `vocal_language` against
  `^[a-z]{2,3}(-[A-Za-z]{2,4})?$`, drop `lyrics` when instrumental or `!SupportsLyrics`, drop `bpm` and
  `vocal_language` unless local ACE-Step. An unreadable answer returns 502 with code `ai_unreadable`.
- Lyrics output cleanup: strip a surrounding code fence before returning.
- `POST /api/desktop/noisemaker/lyrics/check` (desktop read scope): body `{lyrics, style}` →
  `{status, issues, length, max_chars, provider}`. No LLM.
- Generate path unchanged: MiniMax auto-lyrics reuse the improved `lyrics` prompt; local mode never calls
  the LLM.

### 3. UI (`noisemaker-create.js`, `noisemaker.js`, `noisemaker-menus.js`, CSS, i18n)

- **Song vorschlagen** button in the create head (both modes, hidden without `llm_available`, disabled
  in read-only). Switches to Custom mode, sends `song` with the current idea/style/instrumental as seeds,
  fills idea, style, title, lyrics (and BPM/vocal language when `supports_controls`).
- Lyrics field: the single lyrics button becomes **✨ KI ▾**, opening a desktop context menu
  (via a `menu(x, y, items)` dependency passed from the shell):
  Schreiben/Fortsetzen · Refrain eingängiger · Kürzen · Reime verbessern · Bridge ergänzen ·
  Tags prüfen · Eigener Wunsch… (opens an inline instruction row: input + Anwenden; Enter applies, Esc
  closes). The AI entries are absent without `llm_available`; Tags prüfen is always present when the
  provider supports lyrics.
- Check panel under the lyrics field: localized issue list (severity glyph, message from
  `desktop.noisemaker_issue_<code>` with params, line number button that selects the line in the textarea),
  **Mit KI korrigieren** (`revise` + `fix_tags`) when the LLM is available and an error or warning exists,
  close button; "passt zu <provider>" line when there are no issues. The panel closes when the lyrics change.
- Undo bar at the top of the form after any AI change: "KI-Vorschlag übernommen (fields) · Rückgängig".
  Restores the snapshot of the changed fields. Cleared when the user edits a changed field, switches
  mode, starts a new song, or a newer AI change replaces it.
- Counter and textarea `maxlength` use `lyrics_guide.max_chars`; the counter turns into the error colour
  over the limit.
- Library: track context menu gets **Variante vorschlagen ▸** (submenu: Neuer Stil, Neue Strophen,
  Fortsetzung) for single tracks when `llm_available` and not read-only. The shell opens the create pane
  (`focusCreate`) and calls `create.suggestVariation(track, mode)`.
- Busy and stale-result handling follows the existing `enhancementRequests` / `fieldRevisions` pattern;
  `song`/`variation` lock all four fields.
- New strings in all 16 `ui/lang/desktop/*.json` files.

### 4. Agent

- `prompts/tools_manuals/generate_music.md` gains a "Lyrics tags by provider" section listing the
  ACE-Step and MiniMax tags, modifiers, parentheses meaning and the 3500-character MiniMax limit.
  A Go test asserts that every `SectionTags` entry of both guides appears in the manual.
- The tool schema's `lyrics` description points to provider-specific tags in the manual.
- `tools.GenerateMusicResult` (shared by the agent's `generate_music`, Noisemaker and Game Maker): for
  MiniMax, non-instrumental lyrics over 3500 characters return `invalid_lyrics_length` before the paid
  call and before the daily quota is reserved, with a message naming the limit and the allowed tags.
  Noisemaker's generate handler shows `result.Message` when it is set.

### 5. Documentation

- `documentation/ace-step-local.md`: helpers run only on explicit click; generation never calls the LLM.
- `documentation/manuals/{en,de}/04-webui.md` Noisemaker section: the new helpers in a few lines.

## Error handling

- LLM missing: AI entries hidden; tag check still works.
- LLM failure or timeout (45 s): toast, fields unchanged.
- Unreadable song JSON: toast `ai_unreadable`, fields unchanged.
- Read-only desktop: AI buttons disabled (existing pattern); check still works.
- A result that arrives after the user edited the field or disposed the window is dropped.

## Testing

- Go (`internal/tools`): guide per provider, `PromptRules` contents, every lint code incl. aliases and
  case-insensitive matching, MiniMax length error.
- Go (`internal/server`): state `llm_available` true in local mode; enhance works in local mode with a fake
  LLM; generate in local mode never calls the LLM (adjusted `TestNoisemakerLocalNoCloudLyrics`); prompts
  carry the MiniMax tag list and ACE-Step modifier rule; `revise` action allow-list, `custom` needs an
  instruction, `fix_tags` sends server lint results; song parsing (fenced, prose around, invalid → 502,
  caps, instrumental drops lyrics, bpm clamp); `lyrics/check` endpoint.
- Go (`internal/agent`): MiniMax over-limit lyrics rejected before the provider call; manual drift test.
- UI: i18n keys present in all languages (existing lint); browser check of the create panel through the
  desktop preview harness (menu, check panel, undo, variation), with a stubbed LLM.

## Out of scope

Deterministic auto-fix without an LLM, live lint while typing, a dedicated translate action (use
Eigener Wunsch), lint warnings in the agent tool output (only the hard MiniMax limit), assistant chat sidebar.
