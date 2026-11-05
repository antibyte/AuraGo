# Frozen interfaces

Storymaker phase 2. Decision ledger v2 beats the spec. Line numbers were read on 2026-11-04 in the main checkout, and other sessions keep committing, so match by the quoted symbol if a line moved.

Batch tags: `[2A]` Motion, `[2B]` Speech, `[2C]` Sound, `[2D]` Export. An item without a tag lands with the first batch that uses it. "Decided here" marks a choice the ledger left open.

## 1 Go

### 1.1 Package `internal/storymaker`

New files: `narration.go`, `audio_decode.go`, `envelope.go`, `glyphs.go`, `voices.go`, `music.go`, `ranges_gen.go`, and the helper package `internal/storymaker/storymakertest`.

#### Story model `[2A, 2C]`

```go
type Style struct {
    Palette  string `json:"palette"`
    Font     string `json:"font"`
    Captions bool   `json:"captions"`
    Rhythm   string `json:"rhythm,omitempty"`   // [2A] "" | calm | standard | lively; Go never writes a default
    AutoSFX  bool   `json:"auto_sfx,omitempty"` // [2C]; unknown_field until then
}
```

- `Style` stays comparable (`defaults.go:48` uses `==`).
- `validator.style()` refuses any other rhythm with `invalid_value` at `/style/rhythm`.
- `ApplyDefaults` `[2A]`: first scene `cut`. Any other scene with an empty transition gets `wipe` when `sc.Environment` differs from the previous scene's value, else `fade`. Compare before `prevEnv` is overwritten at `defaults.go:78`.
- `phase1Beats` (`model.go:423`): `[2A]` adds `highlight`, `[2B]` adds `say`, `[2C]` adds `sfx`. `[2B]` removes the speaker `notYet` at `validate.go:456`. `[2C]` removes the music `notYet` at `validate.go:232-234`.
- Music validation `[2C]`, inside `validator.music()`:
  - mode `file` needs exactly one of `file` and `track`. Both set is `invalid_value` at `/music/track`.
  - `track` must parse as a positive int64, else `invalid_value`.
  - mode `generated` with neither `mood` nor `prompt` is `missing_field` at `/music/mood`.
  - a `mood` outside `MusicMoods` is `invalid_value`.
- Say validation `[2B]`:
  - A `say` without `text` in a scene that is not silent, whose `who` differs from the scene speaker, is an error `invalid_value` at `<beat>/who`. Decided here: spec line 503 says the speaker "must be that cast ID", so it blocks a render.
  - A `say` without `text` in a silent scene is a warning `invalid_value` at `<beat>/text`. Decided here: it has no narration to show, so no bubble is drawn.
  - A `say` with `runeLen(text) > lim.MaxNarrationChars` is an error `limit_exceeded` at `<beat>/text`, the same limit as narration (`validate.go:440-445`). Then 61 blocks and 800 pieces per block hold by construction.
  - A `say` whose actor is off stage keeps the warning of `validate.go:810-812`, with the message changed to say that the speech still plays, with no bubble and no lip-sync. Decided here.

#### New issue codes

```go
const (
    CodeGlyphMissing      = "glyph_missing"       // [2B] error, Go Validate
    CodeLineUnread        = "line_unread"         // [2A] warning, player
    CodeDeadStretch       = "dead_stretch"        // [2A] warning, player
    CodeFirstFrameEmpty   = "first_frame_empty"   // [2A] warning, player
    CodeTextTooSmall      = "text_too_small"      // [2A] warning, player
    CodeFadeBetweenWorlds = "fade_between_worlds" // [2A] warning, player
    CodeEntryMonotony     = "entry_monotony"      // [2A] minor, player
    CodeTransitionVariety = "transition_variety"  // [2A] minor, player
    CodeCueLate           = "cue_late"            // [2A] warning, player
)
```

`ISSUE_CODES` in `storymaker-core.js:319`, the pinned list at `ui/desktop_storymaker_guards_test.go:161` and `storymaker.issue.<code>` in 16 locales change with each batch.

#### Narration blocks and pieces `[2B]`

```go
const (
    BlockKindNarration = "narration"
    BlockKindDialogue  = "dialogue"
    JoinSentence       = "sentence" // 350 ms pause after this piece
    JoinSplit          = "split"    // 150 ms pause after this piece
)

type NarrationBlock struct {
    ID         string // "<scene>:n" or "<scene>:b<beatIndex>"
    Kind       string
    SceneID    string
    SceneIndex int
    Beat       int    // beat index; -1 for kind narration
    Speaker    string // "narrator" or a cast id
    Voice      Voice  // resolved; Provider == "" when nothing resolves; Provider is in NormalizeProvider form
    Text       string // StripMarkers of the source text
}

type Piece struct {
    Text  string // what is synthesised and hashed; no leading or trailing whitespace
    Chars int    // UTF-16 units of the source span including the whitespace after it
    Join  string // how it joins the next piece; "" on the last
}

func Blocks(s *Story) []NarrationBlock
func StripMarkers(text string) string
func SplitPieces(text string, limit int) []Piece
func NormalizeProvider(id string) string // trimmed and lower-cased
func ChunkLimit(provider string) int
func PieceKey(v Voice, pieceText string) string
func PiecesOf(b NarrationBlock) []Piece // SplitPieces(b.Text, ChunkLimit(b.Voice.Provider))
func IsJSSpace(r rune) bool
func UTF16Len(s string) int
func TextWidth(s string) int            // East Asian wide counts 2
func WordCount(s string) int
```

**`Blocks`**
- Emits, in scene order and inside a scene with narration first and say blocks in beat order:
  - one narration block per storyboard or code scene that is not silent and whose stripped narration is not empty
  - one dialogue block per `say` beat in a storyboard scene whose `StripMarkers(text)` is not empty. Code scenes have no beats (`compile.js:61-63`). JS uses `parsed.words > 0` for the same test.
- Decided here: `silent` silences only the scene narration. A say with text in a silent scene is still spoken and still needs audio, because spec line 178 only names the narration.
- Decided here: a say whose actor is off stage still makes a block. Its audio plays, without a bubble or lip-sync.
- Voice for a narration block: `cast[].voice` when `speaker` is a cast id and that voice is not nil, else `story.voice`.
- Voice for a dialogue block: the same rule with `beat.who`.
- `Voice.Provider` is stored through `NormalizeProvider`, because the validator accepts mixed case without rewriting it (contract line 60). `ChunkLimit` and `LocalVoiceProvider` normalise their argument too, and every provider lookup uses the normalised id.

**Cross-language whitespace and marker rule**
- `StripMarkers` is byte-for-byte `SM.narration.parse(text).plain` (`narration.js:15-34`):
  - markers are the regex `\{(\d+)\}` with ASCII digits
  - each chunk between markers has runs of JS whitespace collapsed to one U+0020
  - a chunk's leading space is dropped when the output is empty or already ends in a space
  - one trailing space is removed at the end
- `IsJSSpace` is exactly: U+0009-000D, U+0020, U+00A0, U+1680, U+2000-200A, U+2028, U+2029, U+202F, U+205F, U+3000, U+FEFF.
- Fixture `internal/storymaker/testdata/narration/cross_language.json`, read by a Go test and a Node test: `[{input, plain, cues:{n:offset}, chars_total, width, words, pieces:[{text, chars, join}]}]`. It holds U+00A0, U+3000, a CJK sentence, a Hindi sentence, a Greek sentence and a surrogate pair.

**`SplitPieces`, one rule list**
1. Sentence end. An ASCII `.`, `!` or `?` whose following run of closers is followed by whitespace or the end of the text. Or any of `…`, `。`, `！`, `？`, `۔`, `؟`, `।`, `॥`, U+037E, anywhere. Further terminators and the closers `" ' ” ’ » › ) ] } ） 」 』 】 〉 》` stay on the sentence. The whitespace after a sentence belongs to its `Chars`, not its `Text`.
2. A sentence over `limit` runes is split:
   - at the comma nearest its middle, comma being one of `, ، 、 ， ; ；`, kept on the left part, with each part at least 40 runes, recursively
   - else at the last whitespace before the limit
   - else at a code-point boundary at the limit

   Parts join with `JoinSplit`.
3. Merge. A part with `TextWidth < 20` or `WordCount <= 3` merges into the next part, or into the previous one if it is last, when the merged span stays within `limit` runes. A merged part is tested again. It keeps the source substring and the `Join` of its last member.
4. `Chars` sums to `UTF16Len(text)` by construction. Empty text gives nil.

Decided here: `WordCount` uses the CJK rule of D24. Without it every Han or kana sentence counts as one word and would merge up to the limit.

`WordCount` walks code points, skipping `IsJSSpace`. A run of n `CJK_WORD` characters counts `ceil(n/2)`. A run of anything else counts 1.

`TextWidth`: `EAST_ASIAN_WIDE` counts 2, everything else 1.

`ChunkLimit`, in runes: phonon 2000, sanotts 2000, google 200, every other id 400.

`PieceKey`: lower-case hex sha256 of the bytes `provider + "|" + voice + "|" + lang + "|" + pieceText`. Decided here: `provider` and `lang` are trimmed and lower-cased first, because the validator accepts mixed case without rewriting it (contract line 60). `voice` and `pieceText` stay as written.

#### Audio decoder `[2B]`, `audio_decode.go`

```go
var ErrUnknownAudio = errors.New("storymaker: unknown audio format")

const maxDecodedSeconds = 600

type DecodedAudio struct {
    Rate, Channels int
    Samples        []float32 // interleaved, -1..1
}

// SniffAudio detects the format from the first bytes: "wav", "mp3" or "".
func SniffAudio(head []byte) string

// DecodeAudio detects the format from the bytes, never from a claimed extension. ext is "wav" or "mp3".
func DecodeAudio(data []byte) (DecodedAudio, string, error)

// ProbeAudio reads the format and the duration without keeping any samples: the WAV header, or
// go-mp3's Length (go-mp3 decode.go:195). Music tracks use it, so /narration and the job never decode a track.
func ProbeAudio(r io.ReadSeeker) (ext string, durationMS int, err error)
```

- WAV: PCM16, PCM24, float32, mono or stereo, WAVE_FORMAT_EXTENSIBLE (0xFFFE), data size 0 or 0xFFFFFFFF treated as streamed. Data length is capped at the file size.
- WAV refusals: a rate outside 4000..192000, channels outside 1..2, a block align that does not equal channels times bytes per sample. The old decoder refused rates under 4 kHz, and the envelope window needs a rate of at least 100.
- MP3 through go-mp3 (`go.mod:40`): stereo s16 at the source rate, panics recovered into an error. It reads in chunks and stops with an error above `maxDecodedSeconds`, so the cap holds before the allocation.
- WAV above `maxDecodedSeconds` is an error too.
- Anything else returns `ErrUnknownAudio`. The narrate job turns every decoder error into `narration_failed` naming the provider.
- Decided here: the stored extension is the detected one, not `ttsAudioExtension`.
- Decided here: a stored piece that no longer decodes is removed with `RemovePiece` and synthesised again, so a damaged file cannot fail a project forever.

`voice.DecodeWAVPCM16Source` is not used. Supertonic is forced to WAV with `TTSConfig.Supertonic.ResponseFormat = "wav"`.

#### Envelope and block assembly `[2B]`, `envelope.go`

```go
// Levels marshals as a JSON array of integers 0..255. A plain []uint8 would become base64.
type Levels []uint8
func (l Levels) MarshalJSON() ([]byte, error)
func (l *Levels) UnmarshalJSON(b []byte) error

type Envelope struct {
    V          int    `json:"v"`           // 1
    DurationMS int    `json:"duration_ms"`
    Rate       int    `json:"rate"`        // 100
    Levels     Levels `json:"levels"`
}
func ComputeEnvelope(a DecodedAudio) Envelope
func (e Envelope) SpeechRuns() [][2]int // ms in the file, bridged, ascending

// ParseEnvelope reads a stored env.json. These are ordinary workspace files, so nothing in them is trusted:
// v == 1, rate == 100, 0 < duration_ms <= 600000, and len(levels) equals ceil(duration_ms/10) within 1.
func ParseEnvelope(data []byte) (Envelope, error)

type AudioPieceInput struct {
    Hash, Ext string
    Chars     int
    Join      string
    Env       Envelope
}

type AudioPiece struct {
    Hash  string `json:"hash"`
    Ext   string `json:"ext"`
    AtMS  int    `json:"at_ms"`
    InMS  int    `json:"in_ms"`
    OutMS int    `json:"out_ms"`
    Chars int    `json:"chars"`
}

type AudioBlock struct {
    ID         string       `json:"id"`
    Kind       string       `json:"kind"`
    Beat       *int         `json:"beat,omitempty"`
    Speaker    string       `json:"speaker"`
    DurationMS int          `json:"duration_ms"`
    Spans      [][2]int     `json:"spans"`    // never null
    Levels     Levels       `json:"levels"`
    Pieces     []AudioPiece `json:"pieces"`   // never null
    SpeechOK   bool         `json:"speech_ok"`
}

// AssembleBlock is the one function behind /narration, Plan.AudioInfo and the mix.
func AssembleBlock(b NarrationBlock, pieces []AudioPieceInput, fps int) AudioBlock
```

Decided here: `speech_ok` is added to the D2 block shape. It is true when every piece had detectable speech. JS needs it to choose `placed`.

Envelope rules:
- Window i covers the samples `floor(i*rate/100)` to `floor((i+1)*rate/100)-1`, as mean power across channels. A final partial window counts. Decided here: a fixed `floor(rate*0.01)` window would be 220 samples at the 22050 Hz of sanotts, 100.2 windows per second, and would drift about 45 ms over 20 s.
- Level byte: `round((clamp(dBFS,-60,0)+60)*255/60)`, silence 0.
- A window is speech when `level >= 85` (-40 dBFS).
- Runs separated by a gap under 100 ms are bridged.
- `duration_ms = round(frames*1000/rate)`.

`AssembleBlock`:
1. Per piece, speech start and end are the first run's start and the last run's end.
2. `in_ms = max(0, start-40)`, `out_ms = min(duration, end+100)`.
3. A piece with no run: `in_ms = 0`, `out_ms = duration`, and `SpeechOK` is false for the block.
4. `at_ms` is cumulative: previous `at_ms` plus previous used length plus the pause of the previous `Join`.
5. `DurationMS` is the last piece's `at_ms + (out_ms-in_ms)`. `Blocks` never emits a block with no piece.
6. `Spans`: the bridged runs of each piece clipped to its used range, shifted to block time.
7. `Levels`: `ceil(DurationMS*fps/1000)` entries.
   - The block is a 10 ms grid. Bin j holds the piece's level byte at the matching file position, or 0 in a pause.
   - Frame k averages power over bins `k*100/fps` to `(k+1)*100/fps - 1` (integer division).
   - Power from a byte b: 0 when b is 0, else `10^((b*60/255-60)/10)`.

#### Audio info `[2B]`, `narration.go`

```go
type SceneAudio struct { Blocks []AudioBlock `json:"blocks"` }
type AudioInfo struct { Narration map[string]SceneAudio `json:"narration"` }

type MissingBlock struct {
    SceneID string `json:"scene_id"`
    BlockID string `json:"block_id"`
    Chars   int    `json:"chars"` // code points of the missing pieces' text
}
type CharCounts struct { Total, Missing int } // json: total, missing; code points
type MusicRef struct {
    Key        string `json:"key"`
    Ext        string `json:"ext"`
    DurationMS int    `json:"duration_ms"`
}
type AudioAnswer struct {
    Narration  map[string]SceneAudio `json:"narration"`
    Missing    []MissingBlock        `json:"missing"`
    Chars      CharCounts            `json:"chars"`
    Music      *MusicRef             `json:"music"`
    MusicIssue string                `json:"music_issue,omitempty"` // "source_missing"
}
func (a AudioAnswer) Info() AudioInfo

var ErrMusicSourceMissing = errors.New("storymaker: the music source is missing")

// MusicKeyResolver returns the key of the story's music.
// "" means nothing is to be played: mode none, no music, or volume 0. A nil resolver means "".
// A file or track source that no longer exists returns ("", ErrMusicSourceMissing).
type MusicKeyResolver func(ctx context.Context, m *Music) (string, error)

func (p *ProjectStore) AudioAnswer(ctx context.Context, id string, s *Story, resolve MusicKeyResolver) (AudioAnswer, error)

// PieceFiles lists audio/narration and accepts only names matching ^[0-9a-f]{64}\.(wav|mp3|env\.json)$,
// which also skips leftover .aurago-write-* files. HasEnv says the file exists; ParseEnvelope decides validity.
func (p *ProjectStore) PieceFiles(ctx context.Context, id string) (map[string]PieceFile, error)
type PieceFile struct { Ext string; HasEnv bool }

// AudioPath is the desktop path of an audio file, for Publisher.WriteStream. folder is narration or music.
func (p *ProjectStore) AudioPath(id, folder, file string) (string, error)
func (p *ProjectStore) OpenAudio(ctx context.Context, id, folder, file string) (io.ReadSeekCloser, FileInfo, error)
func (p *ProjectStore) WritePiece(ctx context.Context, id, hash, ext string, data []byte, env Envelope) error
func (p *ProjectStore) WriteEnvelope(ctx context.Context, id, hash string, env Envelope) error
func (p *ProjectStore) RemovePiece(ctx context.Context, id, hash, ext string) error
```

- Decided here: a block enters `Narration` only when every piece has audio and a valid envelope. Otherwise it appears in `Missing`, and JS estimates it.
- `WritePiece` writes the audio with `MustNotExist` (an existing file is not an error), then the envelope. An audio file without a valid envelope counts as missing; a rerun decodes it and overwrites the envelope with `WriteEnvelope`, without a provider call.
- `Missing` and `Narration` are never null.
- `Music` is null unless the key's audio file and a valid meta file both exist.
- `ErrMusicSourceMissing` is not an error of `AudioAnswer`: narration data stays intact, `Music` is null and `MusicIssue` is `source_missing`. A moved file therefore cannot stop the voice panel or the preview. Any other resolver error is returned, and the handler answers 500 `storage_failed`.

Layout: `audio/narration/<hash>.<ext>`, `audio/narration/<hash>.env.json`, `audio/music/<key>.<ext>`, `audio/music/<key>.meta.json`, `audio/music/<key>.pending.json` (section 1.4).

#### Music `[2C]`, `music.go`

```go
var MusicMoods = []string{"upbeat-corporate", "calm-ambient", "inspiring", "playful", "tech-pulse", "dramatic", "lo-fi", "acoustic"}
func MusicPrompt(mood, prompt string) string
func MusicKeyGenerated(mood, prompt string) string
func MusicKeyFile(path string, size int64, mtime time.Time) string
func MusicKeyTrack(id string, size int64, mtime time.Time) string
func MusicVolume(m *Music) float64 // 0.25 when absent
type MusicMeta struct {
    V          int    `json:"v"`           // 1
    Ext        string `json:"ext"`         // wav or mp3
    DurationMS int    `json:"duration_ms"` // 1..3600000
}
func (p *ProjectStore) WriteMusic(ctx context.Context, id, key, ext string, data []byte) error // Store.WriteFile with MustNotExist; SeedMusic and small tracks
func (p *ProjectStore) WriteMusicMeta(ctx context.Context, id, key string, m MusicMeta) error
// MusicInfo reads the meta file and finds the audio file. meta.V is 0 when the meta is absent or invalid; audioExt is "" without audio.
func (p *ProjectStore) MusicInfo(ctx context.Context, id, key string) (meta MusicMeta, audioExt string, err error)
```

- Keys are lower-case hex sha256 of `generated|mood|prompt`, `file|path|size|mtime` or `track|id|size|mtime`. `mtime` is decimal `UnixNano`. Length is excluded.
- Decided here: `meta.json` holds the duration, so `/narration` never decodes a track.
- `MusicPrompt`: the mood's template, then `". "` and the user's prompt when both exist. A prompt without a mood stays as written. `Instrumental:true` is a parameter, not prompt text. First guesses, because they feed a paid call and the key:

| Mood | Template |
| --- | --- |
| `upbeat-corporate` | Upbeat corporate background music, bright piano and clean guitar, steady light drums, positive and professional |
| `calm-ambient` | Calm ambient background music, soft pads and slow textures, no drums, quiet and spacious |
| `inspiring` | Inspiring cinematic background music, warm strings and piano, gentle build, hopeful |
| `playful` | Playful light background music, pizzicato strings, marimba and soft percussion, cheerful and bouncy |
| `tech-pulse` | Modern technology background music, pulsing synth bass, clean electronic drums, focused and forward moving |
| `dramatic` | Dramatic cinematic background music, low strings and deep percussion, tense and serious |
| `lo-fi` | Lo-fi background music, mellow electric piano, soft vinyl texture, relaxed slow beat |
| `acoustic` | Warm acoustic background music, fingerpicked guitar and light percussion, friendly and natural |

#### Glyphs `[2B]`, `glyphs.go`

```go
const GlyphsSchema = "storymaker-glyphs/1"

type GlyphFace struct {
    ID, Family, File string
    Weight           string // FontFace descriptor: "400", "300 700", "100 900"
    UnicodeRange     string // "" means no restriction
    MaxBytes         int64  // 0 means the 8 MiB cap
}
type Glyphs struct { Faces []GlyphFace /* decoded ranges unexported */ }

func LoadGlyphs(fsys fs.FS, path string) (*Glyphs, error) // LimitReader 1 MiB, schema, monotonic ranges
func (g *Glyphs) FaceOf(r rune) int                       // first covering face in order, -1 when none
func (g *Glyphs) Covered(r rune) bool                     // FaceOf >= 0, IsDefaultIgnorable, IsJSSpace or a C0 control
func (g *Glyphs) Missing(text string) []rune              // over the NFC form, distinct, in order
func IsDefaultIgnorable(r rune) bool                      // generated table; U+200C and U+200D are in it
func (c *Catalog) Glyphs() *Glyphs                        // nil when the manifest names no glyphs file
func (c *Catalog) GlyphsFile() string
```

- `LoadCatalog` reads `glyphs.file` from the same folder as `catalog.json`, through `LoadGlyphs`. The mini fixture has no `glyphs` key, and then the check is skipped.
- NFC uses `golang.org/x/text/unicode/norm` (`go.mod:82`).
- `IsJSSpace` counts as covered because `glyphs.needsFor` skips it and narration collapses it to U+0020 before drawing. Go and JS must agree.
- `Validate` adds after `v.scenes()`:
  - one `glyph_missing` per text path, naming up to 3 characters as `U+XXXX "x"`
  - sources: show text, say text, the scene narration when the scene is not silent and either `style.captions` is on or the scene has a say without text, in storyboard and code scenes alike. A silent scene's narration is never drawn (`plan.js:97-98`).
  - code-scene strings stay `layout_unverified`
  - U+2026 must always be covered; the build fails otherwise.

Generated tables `ranges_gen.go` come from `scripts/build-storymaker-ranges.mjs` (section 4.2).

#### Catalog `Entry` additions `[2C]`

```go
type SFXTake struct {
    File         string  `json:"file"`
    DurMS        float64 `json:"dur_ms"`
    OnsetMS      float64 `json:"onset_ms"`
    PeakMS       float64 `json:"peak_ms"`
    ImpactMS     float64 `json:"impact_ms"`
    L50LUFS      float64 `json:"l50_lufs"`
    TruePeakDBTP float64 `json:"true_peak_dbtp"`
}
type SFXHit struct {
    OverDB float64 `json:"over_db"`
    DipDB  float64 `json:"dip_db"`
}
type SFXPattern struct {
    SpacingMS         [2]float64 `json:"spacing_ms"`
    DefaultDurationMS float64    `json:"default_duration_ms"`
}
type SFXSource struct {
    Kind  string `json:"kind"`
    Basis string `json:"basis"`
    URL   string `json:"url,omitempty"`
}
// added to Entry:
Takes    []SFXTake   `json:"takes,omitempty"`
ImpactMS float64     `json:"impact_ms,omitempty"` // nominal; the player and the mix read the take's
Align    string      `json:"align,omitempty"`     // "peak" | "onset"
Hit      *SFXHit     `json:"hit,omitempty"`
Pattern  *SFXPattern `json:"pattern,omitempty"`
Source   *SFXSource  `json:"source,omitempty"`
```

- `checkEntry` for `sfx`: `file` equals `takes[0].file`, at least 2 takes, `align` set, `source` set. The thumb check is skipped for this kind and `thumb` is empty.
- `VerifyFiles` checks every take and skips an empty thumb (`catalog.go:522`).
- `render.snapshot` skips sfx entries, because the page never reads WAVs (`render.go:558-562`).

#### Store addition `[2C]`, `project.go:44-50`

```go
type FileInfo struct {
    Name    string
    Size    int64
    ModTime time.Time
    MIME    string
}
// added to Store:
Open(ctx context.Context, path string) (io.ReadSeekCloser, FileInfo, error) // missing: fs.ErrNotExist
```

- Implemented by `MemStore` (`bytes.Reader`) and `storymakerStore` (over `desktop.Service.OpenPreviewFile`, `service_files.go:373`). Those are the only two implementers.
- Decided here: it lands in `[2B]`, because the audio routes use it for files over 50 MB.
- The `st` field of `ProjectStore` stays unexported (`project.go:85`). The render package and `storymakertest` reach audio through `OpenAudio`, `AudioPath`, `WriteMusic`, `WriteMusicMeta` and `MusicInfo`.

#### Project creation and voice resolution `[2B]`, `voices.go`

```go
func (p *ProjectStore) CreateWithVoice(ctx context.Context, title, aspect string, fps int, voice *Voice) (Project, Story, string, error)
// Create calls CreateWithVoice with nil, so existing callers stay as they are.

type VoiceOption struct {
    ID        string `json:"id"`
    Name      string `json:"name,omitempty"`
    Language  string `json:"language,omitempty"`
    Gender    string `json:"gender,omitempty"`
    Installed *bool  `json:"installed,omitempty"`
    Note      string `json:"note,omitempty"`
}
type VoiceProvider struct {
    ID              string        `json:"id"`
    Active          bool          `json:"active"`
    Ready           bool          `json:"ready"`
    Voices          []VoiceOption `json:"voices,omitempty"`
    FreeText        bool          `json:"free_text,omitempty"`
    Languages       []string      `json:"languages"`             // "*" means not restricted
    TestedLanguages []string      `json:"tested_languages"`      // never null; empty in phase 2
    NeedsInstall    bool          `json:"needs_install"`
    DefaultVoice    string        `json:"default_voice,omitempty"`
    ChunkLimit      int           `json:"chunk_limit"`
    Note            string        `json:"note,omitempty"`
}
var ErrNoVoiceForLanguage = errors.New("storymaker: no voice for this language")

// ResolveProjectVoice follows spec line 505. providers has the order of storymakerVoiceProviders.
// A language base is the first segment of the lower-cased tag.
func ResolveProjectVoice(providers []VoiceProvider, language string, defaultVoiceFor func(provider, language string) string) (Voice, []string, error)
func LanguageBase(tag string) string
func LocalVoiceProvider(id string) bool // phonon, piper, sanotts, supertonic, speech_lab; normalises its argument
func ApplyCaptionOverride(s *Story, burnIn *bool)
```

- Resolution, in order:
  1. the active provider, if ready and it supports the language, with its configured voice when that voice speaks the language
  2. else that provider's `defaultVoiceFor` result
  3. else the first other ready provider in list order that supports the language, with its default voice. The list has local providers ahead of cloud ones, so a paid provider never wins over a free one.
- When nothing fits it returns `ErrNoVoiceForLanguage` and the union of supported languages, excluding `"*"`.
- `ApplyCaptionOverride` sets `s.Style.Captions` when `burnIn` is not nil. `Prepare` calls it before `Validate` (`render.go:157-158`), so burned captions get the glyph check.
- The `not tested` mark of D20 lives on the provider and language, per spec line 492, because free-text providers, sanotts and google have no voice entries to carry it. The panel shows it for every provider when `LanguageBase(language)` is not in `TestedLanguages`.

#### Seeded-audio helper `[2C]`, `internal/storymaker/storymakertest/seed.go`

```go
// SeedNarration writes a generated 24 kHz mono s16 WAV and its envelope for every missing piece,
// with speech at -20 dBFS from 100 ms to the end minus 100 ms. Used by the fixtures that the
// narration_incomplete gate would otherwise break.
func SeedNarration(ctx context.Context, p *storymaker.ProjectStore, id string, s *storymaker.Story, seconds float64) error
// SeedMusic writes a generated WAV and its meta through WriteMusic and WriteMusicMeta.
func SeedMusic(ctx context.Context, p *storymaker.ProjectStore, id string, key string, seconds float64) error
```

### 1.2 Package `internal/storymaker/render`

#### Jobs, `jobs.go`

```go
const (
    KindRender  = "render"
    KindNarrate = "narrate" // [2B]
    KindMusic   = "music"   // [2C]

    PhaseSynthesizing = "synthesizing" // [2B]
    PhaseGenerating   = "generating"   // [2C]

    CodeNarrationFailed      = "narration_failed"      // [2B]
    CodeNarrationUnavailable = "narration_unavailable" // [2B]
    CodeVoiceRequired        = "voice_required"        // [2B]
    CodeBudgetExceeded       = "budget_exceeded"       // [2B]
    CodeNarrationIncomplete  = "narration_incomplete"  // [2C]
    CodeMusicFailed          = "music_failed"          // [2C]
    CodeMusicIncomplete      = "music_incomplete"      // [2C]

    MinFreeDiskBytes = minFreeStagingByte // [2B] exported for the narrate and music jobs
)

type ManagerOptions struct {
    Dir       string
    Run       RunFunc
    MaxQueued int
    Retention int
    Logger    *slog.Logger
    Kinds     []string // [2B] empty means {KindRender}; load() keeps these kinds only (jobs.go:266)
    Workers   int      // [2B] 0 or 1; NewManager refuses anything else
    NewID     func() string
    Now       func() time.Time
}

type EnqueueRequest struct {
    ProjectID   string
    Kind        string // [2B] "" means Kinds[0]; a kind outside Kinds is an error
    Coalesce    bool   // [2B] a queued or running job of this project and kind is returned with replayed true
    Key         string
    Fingerprint string
    Payload     any
}

type JobWarning struct {
    Code    string `json:"code"`
    Message string `json:"message"`
}

// added to Job:
Warnings    []JobWarning `json:"warnings,omitempty"`     // [2C]
PiecesDone  int          `json:"pieces_done,omitempty"`  // [2B] kind narrate
PiecesTotal int          `json:"pieces_total,omitempty"` // [2B] kind narrate

func (h *Handle) SetPieces(done, total int)     // [2B]
func (h *Handle) AddWarning(code, message string) // [2C]
```

- `Enqueue` does the coalesce check under `m.mu` in the same step as the insert, so two calls cannot create two jobs.
- `runJob` passes a nil output when `Output.Path == ""`, so a narrate job has no `output` key.
- Decided here: warnings are `{code, message}` objects. Codes: `stem_failed`, `subtitle_failed`, `library_failed`, `music_silent`.
- `[2B]` The messages at `jobs.go:272`, `:542`, `:604`, `:673` and `:675` say "this job" instead of "export" and "render", because a narrate job ends through them too.

#### Output `[2C]`

```go
type OutputStems struct {
    Narration string `json:"narration,omitempty"`
    Music     string `json:"music,omitempty"`
    SFX       string `json:"sfx,omitempty"`
}
type Output struct {
    Path    string       `json:"path"`
    Size    int64        `json:"size"`
    Stems   *OutputStems `json:"stems,omitempty"`   // present ("{}") whenever stems were asked for; a silent stem is omitted
    SRT     string       `json:"srt,omitempty"`
    VTT     string       `json:"vtt,omitempty"`
    Library string       `json:"library,omitempty"`
}
```

#### Render options `[2C]`

```go
type RenderOptions struct {
    BurnInCaptions *bool  `json:"burn_in_captions,omitempty"` // nil means style.captions
    SRT            bool   `json:"srt"`
    VTT            bool   `json:"vtt"`
    Stems          bool   `json:"stems"`
    Quality        string `json:"quality"` // standard | high
}
func ParseRenderOptions(body []byte) (RenderOptions, error) // empty body: defaults; unknown field: error
func (o RenderOptions) Fingerprint() string
```

- Defaults: all false, `quality:"standard"`.
- `Fingerprint`: lower-case hex sha256 of `burn=<null|true|false>;srt=<b>;vtt=<b>;stems=<b>;quality=<q>`.
- `handleStorymakerRender` fingerprint: `id + "\x00" + match + "\x00" + opts.Fingerprint()`.
- Decided here: `high` sets `Encoder.Quality = 18`. `standard` keeps the selected encoder's value (`videostudio/encode.go:27`).

#### Plan and Prepare

```go
type Plan struct {
    ProjectID string
    Story     storymaker.Story
    StoryJSON []byte
    Bundles   map[string][]byte
    StoryETag string
    Blocks    []storymaker.NarrationBlock // [2B]
    AudioInfo storymaker.AudioInfo        // [2B]
    AudioJSON []byte                      // [2B] marshalled once, served in boot.audio_info
    Options   RenderOptions               // [2C]
    Music     *storymaker.MusicRef        // [2C]
}
// [2B] unchanged signature:
func (r *Runner) Prepare(ctx context.Context, projectID, ifMatch string, st Settings) (*Plan, error)
// [2C] signature changes; callers are storymaker_handlers.go:551 and the render tests:
func (r *Runner) Prepare(ctx context.Context, projectID, ifMatch string, st Settings, opts RenderOptions) (*Plan, error)
```

`[2C]` `Prepare` refuses with `*PrepareError` carrying these codes, after validation and before the bundles (`render.go:164-174`):
- `narration_incomplete`, with `Missing []storymaker.MissingBlock` and `Chars storymaker.CharCounts` on the error
- `music_incomplete`, with `MusicKey string` and `MusicIssue string` on the error, when music is on, its volume is above 0 and the key's file or meta is missing. A missing source gives `MusicKey ""` and `MusicIssue "source_missing"`.

Volume 0 has one owner: the resolver returns `""`, so `Plan.Music` is nil and the gate passes.

`Deps` additions:

```go
MusicKey      storymaker.MusicKeyResolver // [2B]; nil means key ""
Library       func(ctx context.Context, slug string, r io.Reader, size int64) (string, error) // [2C]; nil skips the copy
WorkspaceFree func() (int64, error)       // [2C] free bytes of the desktop workspace; nil skips the check
```

`Publisher` stays as it is, so its test stand-ins keep compiling.

#### Settings additions, `render.go:64-73`

```go
FFprobePath  string // [2C] empty fails the job with ffmpeg_unavailable
LogicalCPUs  int    // [2D] runtime.NumCPU() in production
FreeMemoryMB int64  // [2D] gopsutil mem.VirtualMemory().Available >> 20; 0 means unknown, no memory bound
```

New in `internal/videostudio/encoder_export.go` `[2C]`: `func ResolveFFprobe(ffmpegPath string) (string, error)` over `resolveProbeBinary` (`runtime.go:126`).

#### Listener, boot and report

```go
const maxReportBytes = 8 << 20 // [2B] listener.go:24; the text at listener.go:383 changes with it

type PageParams struct {
    Width, Height   int
    FPS             int
    Transport       string
    FrameLimit      int
    FrameDeadlineMS int
    Token           string          // [2B] 32 hex characters; StartListener refuses anything shorter
    AudioInfo       json.RawMessage // [2B] nil serves {}
    Workers, Chunk, PreRoll int     // [2D]
}

type SnapshotFont struct {
    Name, Family string
    Weight       int
    File         string
    Data         []byte
    Fallback     bool   // [2B]
    UnicodeRange string // [2B]
    WeightRange  string // [2B] FontFace descriptor when it differs from Weight
    MaxBytes     int64  // [2B]
}

type bootFont struct {
    Name         string `json:"name"`
    Family       string `json:"family"`
    Weight       int    `json:"weight"`
    URL          string `json:"url"`
    Fallback     bool   `json:"fallback,omitempty"`
    UnicodeRange string `json:"unicode_range,omitempty"`
    WeightRange  string `json:"weight_range,omitempty"`
    MaxBytes     int64  `json:"max_bytes,omitempty"`
}

// added to bootDoc; AudioInfo changes type to json.RawMessage:
Token     string `json:"token,omitempty"`    // [2B] present on the first GET only
Workers   int    `json:"workers,omitempty"`  // [2D]
Chunk     int    `json:"chunk,omitempty"`    // [2D]
PreRoll   int    `json:"pre_roll,omitempty"` // [2D]

type ReportAnswer struct {
    Start   bool   `json:"start"`
    Pending bool   `json:"pending,omitempty"` // [2C]
    Error   string `json:"error,omitempty"`
    Message string `json:"message,omitempty"`
}
// FromFrame is removed [2C].

type StartAnswer struct {
    Pending bool   `json:"pending,omitempty"`
    Start   bool   `json:"start,omitempty"`
    Error   string `json:"error,omitempty"`
    Message string `json:"message,omitempty"`
}

type Hooks struct {
    Report func(body []byte) ReportAnswer
    Frame  func(n int, contentType string, data []byte) (FrameAnswer, int)
    Error  func(e PageError)
    // Start blocks at most startHold. status is the HTTP status: 404 before an accepted report, else 200. [2C]
    Start  func(ctx context.Context) (answer StartAnswer, status int)
}
```

Token, in `Listener`:
- `boot()` serves `Token` only on the first GET, through an `atomic.Bool` swap. HEAD never consumes it.
- `report`, `frame`, `pageError` and `start` first compare `r.URL.Query().Get("t")` with `subtle.ConstantTimeCompare`.
- A mismatch answers 403 with `{"error":"bad_token"}` (`{"stop":true,"error":"bad_token"}` on `/frame`), calls no hook, and consumes nothing.
- The prefix stays `<origin>/<prefix>`. `count()` learns the route `start`.

`font()` picks the Content-Type by extension: `.woff2` `font/woff2`, `.ttf` `font/ttf`, `.otf` `font/otf` (`listener.go:373` hard-codes woff2).

Report struct, `report.go`:

```go
type reportAudioEvent struct {
    Kind    string `json:"kind"`     // narration | dialogue | sfx
    Ref     string `json:"ref"`      // piece hash or sfx id
    StartMS int64  `json:"start_ms"`
    SceneID string `json:"scene_id"`
    BlockID string `json:"block_id,omitempty"`
    Piece   int    `json:"piece,omitempty"`
    Take    int    `json:"take,omitempty"`
}
type reportCaption struct {
    StartMS int64    `json:"start_ms"`
    EndMS   int64    `json:"end_ms"`
    Lines   []string `json:"lines"`
    SceneID string   `json:"scene_id"`
    BlockID string   `json:"block_id"`
}
// added to compileReport:
AudioEvents  []reportAudioEvent `json:"audio_events"`
Captions     []reportCaption    `json:"captions"`
CuePrecision *float64           `json:"cue_precision"`

// added to reportExpect:
Blocks    map[string]reportBlock // measured blocks only: id to {SceneID, Kind, Hashes []string, Text string (captionKey of the block text)}
Catalog   *storymaker.Catalog    // [2C] sfx refs; nil before
MaxEvents int                    // derived, see below

const maxRhythmFactor = 1.25 // pinned against RHYTHM in motion.js by a test
const maxReportCaptions = 8000

// captionKey removes JS whitespace and control characters. Both sides of the caption check use it.
func captionKey(s string) string
```

S2 validation in `parseReport`, refusal `render_failed`:
- Film length: `filmMS = round(frame_count*1000/fps)`, and `|duration_ms - filmMS| <= 1`. All bounds below use `filmMS`, never the report's own value (`report.go` checks no duration today).
- Count: `len(AudioEvents) <= MaxEvents`. `MaxEvents` is the sum of the pieces of all measured blocks, plus per sfx beat 1 or for a pattern `floor(max(beat.duration*1000, default_duration_ms*maxRhythmFactor)/spacing_ms[0])+1`, plus one per show with entry `pop` when `auto_sfx`. Absolute ceiling 32768.
- Kind: `narration`, `dialogue` or `sfx`. Anything else is refused.
- Narration and dialogue event: `block_id` known, kind and scene match, `piece` in range, `Hashes[piece] == ref`, the pair `(block_id, piece)` unique.
- Completeness: for each measured block the event pieces form a prefix `0..k` with no gap. A narration block must have at least piece 0, because a scene always starts inside the film. A dialogue block may have none, because compile drops an event past the film end. This is what stops an empty `audio_events` from defeating the gate, and it also catches an honest bug that estimates a measured block.
- Sfx event: `ref` is a catalog sfx, `0 <= take < len(takes)`.
- Every event: `0 <= start_ms < filmMS`.
- Caption:
  - at most `maxReportCaptions` captions
  - `block_id` known; `0 <= start_ms < end_ms <= filmMS`; starts ascending
  - 1 or 2 lines, each not empty after stripping and at most 256 UTF-16 units
  - `captionKey` of the joined lines must be a substring of `reportBlock.Text`. A narration with U+0001 passes `Validate` and must not fail here, so both sides strip the same way.
- A JS `gain` field no longer exists. Any gain in the report is ignored because the struct has no such field.

Decided here: `piece` and `block_id` are on the event, because one block can hold the same sentence twice with the same hash. The caption carries `block_id`, so the substring check is linear.

SRT and VTT writer, `subtitles.go` `[2C]`:
- `func WriteSRT(caps []reportCaption) []byte`, `func WriteVTT(caps []reportCaption) []byte`.
- Controls stripped, runs of JS whitespace collapsed to U+0020, `&`, `<`, `>` written as `&amp;`, `&lt;`, `&gt;`. That also breaks `-->`.

#### FFmpeg, `ffmpeg.go` `[2C]`

```go
type FFmpegConfig struct {
    Path      string
    Encoder   videostudio.Encoder
    Transport string
    Width, Height, FPS, Frames int
    Output    string
    AudioPath string // "" keeps anullsrc with -t; else the mix WAV as input 1
}
const (
    audioRate    = 48000
    audioBitrate = "192k" // replaces silentAudioBitrate 64k at ffmpeg.go:25; applies to anullsrc too
)
```

- `-shortest` is removed (`ffmpeg.go:74`).
- Mix input: `-protocol_whitelist file,pipe -format_whitelist wav -i <AudioPath>`.
- `FFmpeg.Message` scrubs the staging directory from its text. Every FFmpeg and ffprobe error passes through that before it reaches a `JobError`, so no absolute path reaches the user.

#### Mix API, `mix.go` `[2C]`

```go
const (
    MixRate          = 48000
    mixChannels      = 2
    mixBlockFrames   = 4800
    mixTargetLUFS    = -16.0
    mixCeilingDBTP   = -1.0
    mixStemGuardDBFS = -0.1
    bedBelowNarrLU   = -12.0
    bedNoNarrLUFS    = -18.0
    bedMaxGainDB     = 24.0
    voiceMatchCapDB  = 6.0
    duckDepthDB      = 6.0
    duckAttackMS     = 50.0
    duckReleaseMS    = 400.0
    duckLookaheadMS  = 50.0
    duckThresholdDB  = -45.0
    duckPeakDecayMS  = 20.0
    carveHz          = 2200.0
    carveQ           = 1.0
    carveDB          = -5.0
    carveRampMS      = 20.0
    dipAttackFrames  = 480
    dipHoldFrames    = 3840
    dipReturnFrames  = 14400
    loopXfadeS       = 2.0
    musicFadeInS     = 0.5
    musicFadeOutS    = 1.5
    clipFadeMS       = 5
)

type Span struct{ From, To int64 } // film frames, half open
type Impact struct {
    At      int64
    DepthDB float64
}
type MixClip struct {
    Key           string
    Path          string   // raw s16le 48 kHz stereo file in staging; "" when PCM is set
    PCM           []int16  // interleaved stereo, SFX read once into memory
    In            int64    // used range start in frames
    Len           int64    // used length in frames
    Start         int64    // film position in frames
    GainDB        float64  // narration: written by the voice match; sfx: empty on input, written in pass 2
    LevelOffsetDB float64  // sfx only: over_db - take.l50_lufs, over_db 0 without a hit
    Fade          int64    // frames faded at both edges when the piece was trimmed
    Voice         string   // provider|voice, narration only
}
type MixBed struct {
    Path   string
    Frames int64
    Volume float64 // above 0; volume 0 never reaches the mixer
}
type MixInput struct {
    Frames    int64 // round(frame_count*48000/fps)
    Narration []MixClip
    SFX       []MixClip
    Bed       *MixBed
    Spans     []Span
    Impacts   []Impact
}
type MixOutputs struct{ Mix, Narration, Music, SFX string } // Mix required; "" skips a stem
type MixResult struct {
    NarrationI, BedI, PreI, PreTP, BedGainDB, MasterGainDB float64
    VoiceGainDB map[string]float64
    StemPeak    [3]float64 // narration, music, sfx after the master gain, linear
    Clamps      int64
    BedDropped  bool       // the bed read silent
}
func Mix(ctx context.Context, in *MixInput, newMeter func(context.Context) (*Meter, error), out MixOutputs, progress func(float64)) (MixResult, error)
func EstimateMixBytes(decodedBytes, inputBytes, filmFrames int64, stems bool) int64
func HasAudio(plan *Plan, rep compileReport) bool
```

`HasAudio`: the plan has a measured block or `Plan.Music`, or the report has an sfx event. It never decides narration or music from the report, so a report cannot switch the audio off. Sfx exist only in the report, because an sfx-only film is valid (D28).

Passes:
1. Voice match: over the speech spans per voice, from decoded samples. `gain = loudestRMSdB - voiceRMSdB`, capped at +6 dB, written into each clip's `GainDB`. Then stream the narration stem and the raw looped bed into two meters.
2. Compute the bed gain and the sfx gains, then stream the pre-gain mix into a meter and record stem peaks.
3. Write stems and mix after `G = min(-16-I, -1-TP, -0.1-maxStemPeakDBFS)`.

Rules:
- `bedLinearDB = min(reference - bedI, bedMaxGainDB)`.
- A bed that reads `Silent` is dropped: no gain, no music stem, `BedDropped` true, and the job adds warning `music_silent`. A silent file would otherwise give an infinite gain.
- The loop crossfade is `min(loopXfadeS, bed length / 2)`. `POST /music` refuses a source under 1 s.
- Reference: narration `I-12+20*log10(volume/0.25)`, or `-18+20*log10(volume/0.25)` with no narration.
- Sfx gain from D9: in pass 2 `GainDB = reference + LevelOffsetDB`, where the reference is the actual bed level when music is on, narration `I-12` when no music, and `-18` when no narration.
- Stems are quantised after the gain. The mix is the clamp of the integer sum.
- A clip is truncated at `MixInput.Frames`. A clip that starts at or past it is dropped.
- Narration `I = -inf` with narration events fails with `narration_failed`. Otherwise `-inf` gives `G=0`.
- The mix stops on `ctx` at block boundaries.

Input build, `mixinput.go`:

```go
// planMixInput computes clip frames and file sizes and copies nothing.
func (run *renderRun) planMixInput(ctx context.Context) (*mixPlan, error)
func (run *renderRun) buildMixInput(ctx context.Context, plan *mixPlan, dir string) (*MixInput, error)
func decodeToS16(ctx context.Context, ffmpeg, src, dst string, maxSeconds float64) error
```

- Piece and music bytes are first copied by `ProjectStore.OpenAudio` to `staging/<job>/dec/<hash>.<ext>`. FFmpeg sees only the copy.
- Arguments: `-hide_banner -nostdin -loglevel error -protocol_whitelist file -format_whitelist wav,mp3 -i SRC -map 0:a:0 -ar 48000 -ac 2 -f s16le -t <maxSeconds> -fs <bytes> -y DST`, with `bytes = maxSeconds*192000*1.1`.
- `maxSeconds` is `out_ms/1000+1` for a piece and `film+2` for music. A hand-replaced low-bitrate MP3 under `audio/narration` must not decode to many GiB.
- Each hash decodes once per job. SFX WAVs come from `deps.UI` at `img/storymaker/<take.file>`, are read directly and expanded to stereo. `buildMixInput` sets `LevelOffsetDB` and reads the take from `event.take`.
- Envelopes come from go-mp3 and the mix decodes with FFmpeg. go-mp3 skips tags only and has no Xing or LAME handling, so the two timelines may differ. A test decodes `testdata/narration/one.mp3` both ways and compares the first speech run. When the difference is above 10 ms, `buildMixInput` applies that offset per MP3 file before trimming, and the measured value is recorded in the plan.

Disk check, on the `mixPlan`, before any copy: `free < EstimateMixBytes(...) + minFreeStagingByte` fails `render_failed` with the needed size in the message. `EstimateMixBytes` is `inputBytes + decodedBytes + filmFrames*4`, plus `filmFrames*4*3*2` when stems are on, because stems are staged and then published. `decodedBytes` is the sum of `(out_ms+1000)*192` per piece plus music `(film+2)*192000*1.1`.

#### Meter API, `meter.go` `[2C]`

```go
type MeterReading struct {
    I, TP  float64
    Silent bool // loudnorm reported -inf
}
type Meter struct{ /* running ffmpeg */ }
func StartMeter(ctx context.Context, ffmpegPath string) (*Meter, error)
func (m *Meter) Write(interleaved []float32) error
func (m *Meter) Close() (MeterReading, error)
func MeterArgs() []string
func parseMeterOutput(stderr string) (MeterReading, error)
```

Arguments: `-hide_banner -nostdin -nostats -loglevel info -protocol_whitelist file,pipe -format_whitelist f32le -f f32le -ar 48000 -ac 2 -i pipe:0 -af ebur128=framelog=quiet,loudnorm=I=-16:TP=-1:LRA=11:print_format=json -f null -`.

- `I` comes from the last `I:` line of ebur128's summary.
- `TP` comes from `input_tp` in the JSON between the last `{` and the last `}`; values are strings, and `-inf` means silent.
- `-loglevel info` is needed: both outputs vanish at `error`.
- The first mix test confirms this combined graph on FFmpeg 5.1.9 and 8.1.1. If `framelog=quiet` is refused on 5.1.9, use `framelog=verbose`, which prints nothing at `info`.

#### Deferred start, `render.go` `[2C]`

Added to `renderRun`:

```go
page      browserPage        // set under run.mu before Navigate; driveScreenshots needs it
audioPath string             // set under run.mu before the first FFmpeg start; startFFmpeg reads it, so recoverEncoder keeps the mix
mixCtx    context.Context    // derived from the job ctx, created in execute before the page opens
cancelMix context.CancelFunc
mixWG     sync.WaitGroup     // onReport adds under run.mu before it starts the goroutine
startCh   chan struct{}      // closed once FFmpeg runs or the start failed
startErr  *JobError          // read after startCh is closed

func (run *renderRun) onStart(ctx context.Context) (StartAnswer, int)
func (run *renderRun) startAfterReport()
func (run *renderRun) publishExtras(ctx context.Context, slug, exportDir string, out *Output) // stems, subtitles, library; failures become warnings
```

`onReport` under `run.mu`: the one-shot check, `parseReport`, store the report and frame count, `mixWG.Add(1)`, `close(reportCh)`, start the goroutine, return `{start:false, pending:true}`.

`startAfterReport`, without `run.mu` except where named:
1. `SetPhase(PhaseMixing)`.
2. Mix on `mixCtx` when `HasAudio`, into `staging/mix.wav` and `staging/stem-*.wav`. Then set `run.audioPath` under `run.mu`.
3. Under `run.mu`: when `mixCtx.Err() != nil`, end without starting FFmpeg. Otherwise start FFmpeg, set `ff`, start the watchdog.
4. `ingest.Start`.
5. `SetPhase(PhaseRendering)`, `SetFrames(0,n)`.
6. `close(startCh)`.
7. For the screenshot transport, `go driveScreenshots(run.ctx, run.page)` (moved from `render.go:452-454`).

A failure sets `startErr`, calls `run.fail`, then closes `startCh`. `mixWG.Done` runs on every exit of the goroutine.

`startFFmpeg` (`render.go:630-635`) reads `AudioPath: run.audioPath`, so the replacement encode of `recoverEncoder` (`render.go:650`) keeps the mix. Without this a narrated film would ship silent and still pass the frame gate.

`execute` registers `defer func(){ run.cancelMix(); run.mixWG.Wait() }()` directly after the FFmpeg discard defer at `render.go:401-408`. Go runs defers last-in-first-out, so the order at exit is: stop the mix and wait, discard `run.ff`, close the page, proxy and listener, stop the watchdog, `os.RemoveAll(staging)`.

Why this form:
- A job that never got an accepted report added nothing to `mixWG`, so the wait returns at once. A channel closed by the goroutine would block forever and stop the one-worker queue (`jobs.go:614-637`).
- `wait` also returns on `failCh` and on a tab crash (`render.go:437-440`) with the job ctx still live, because `runJob` cancels only after `safeRun` returns (`jobs.go:661-662`). `cancelMix` stops the mix at the next block.
- The check in step 3 is under `run.mu`, so FFmpeg either never starts or is in `run.ff` when the discard defer reads it. On Windows, `RemoveAll` fails while FFmpeg holds `mix.wav` or `out.mp4`.

`onStart`: under `run.mu`, answer `({}, 404)` when no report was accepted. Otherwise select on `startCh`, `time.After(startHold)` (20 s), `ctx.Done()`, and answer with status 200.

#### Frame gate, `gate.go` `[2C]`

```go
type ProbeResult struct {
    VideoPackets              int
    VideoSeconds, AudioSeconds float64
}
func ProbeOutput(ctx context.Context, ffprobe, file string) (ProbeResult, error) // 120 s timeout, 8 MiB output cap
func CheckOutput(r ProbeResult, frames int) error // packets equal frames, |audio-video| < 25 ms
```

Commands: `ffprobe -v error -protocol_whitelist file -format_whitelist mov -count_packets -select_streams v:0 -show_entries stream=nb_read_packets -of csv=p=0 -i FILE`, and `ffprobe -v error -protocol_whitelist file -format_whitelist mov -select_streams v:0|a:0 -show_entries stream=duration -of csv=p=0 -i FILE`. The timeout and output cap match `videostudio/probe.go:76-84`. `finish` runs after `wait` with no overall timer, so an unbounded ffprobe would hold the single worker.

`finish()` order: `ff.Wait`, stat, size, `ProbeOutput` and `CheckOutput` (failure is `render_failed`), phase writing, MP4, `publishExtras`, record.

`publishExtras` checks `WorkspaceFree() >= file size + minFreeStagingByte` before each extra, and otherwise adds `stem_failed`, `subtitle_failed` or `library_failed`. The stream path has no free-space check of its own (`service_file_stream.go:19-113`).

#### Workers `[2D]`

```go
func ChooseWorkers(transport string, logicalCPUs int, freeMB int64, width, height, chunk int, cjk bool, codeScenes int) int
```

- Not `png`: 1.
- Else `clamp(floor((cpus-2)/3), 1, 6)`, further bounded by `floor((freeMB - codeMB) / (60 + 2*chunk*frameMB + (cjk?120:0)))` when `freeMB > 0`, with `frameMB = 0.6*width*height/(1920*1080)` and `codeMB = codeScenes*(cjk?120:0)`, and at least 1.
- `cjk`: any drawn text in the story resolves to face `sm-cjk`, or any code scene has the language base `zh` or `ja`.
- Each code scene has its own worker (`host-core.js:222`), and D13 gives it the faces of its language, so `N` cannot bound that memory. The formula only keeps it out of the budget for player workers. Accepted in phase 2, see section 8 "Still open".
- Decided here: worker encoding exists only for PNG, so other transports stay serial.

### 1.3 Package `internal/desktop` `[2C]`, `service_media.go`

```go
// WriteStorymakerVideo copies r into Videos/Storymaker/<slug>.mp4, or <slug>-2.mp4, <slug>-3.mp4 and so on.
// slug must match ^[A-Za-z0-9-]{1,80}$; the path is built here. It returns the desktop path.
func (s *Service) WriteStorymakerVideo(ctx context.Context, slug string, r io.Reader, size int64, source string) (string, error)
```

- Steps: `ensureReady`; `Config().ReadOnly` refuses, and is checked again under the lock; `resolveMediaMount("Videos/Storymaker")`; `fileutil.FreeDiskBytes(mount.Dir)` must exceed `size + 64 MiB`; `os.OpenRoot(mount.Dir)`; `root.MkdirAll("Storymaker", 0o700)`, because only the mount root exists at start and the first copy would fail otherwise; stage `.aurago-write-*` in that folder with `io.LimitReader(size+1)`; under `desktopMutationMu`, find the free name with `Lstat`, `fileutil.RenameRootContext`, `invalidateListCache`, `Audit`.
- `RegisterMedia` is skipped.
- `Videos` is `DataDir/generated_videos` (`service.go:663`).

### 1.4 Package `internal/server`

Runtime wiring, `storymaker_service.go`:

```go
type storymakerRuntime struct {
    dataDir  string
    store    *storymakerStore
    projects *storymaker.ProjectStore
    catalog  *storymaker.Catalog
    browsers *render.Browsers
    ffmpeg   *storymakerFFmpegCache
    runner   *render.Runner
    jobs     *render.Manager          // render
    narrate  *render.Manager          // [2B] narrate and music
    voices   *storymakerVoiceCache    // [2B]
    usage    *storymakerNarrationUsage // [2B]
}

func (rt *storymakerRuntime) jobByID(id string) (render.Job, *render.Manager, bool)
func (rt *storymakerRuntime) listJobs(projectID string) []render.Job // merged, newest first
func (rt *storymakerRuntime) cancelProject(projectID string)
func (rt *storymakerRuntime) close()                                 // both managers
```

- `narrate`: `render.NewManager(render.ManagerOptions{Dir: filepath.Join(dir, "narrate"), Kinds: []string{render.KindNarrate, render.KindMusic}, MaxQueued: 128, Workers: 1, Run: run, Logger: logger})`, with `run` a closure built in `newStorymakerRuntime` that captures `rt`: `func(ctx, job, payload, h) { return s.storymakerAudioRun(ctx, rt, job, payload, h) }`. Decided here: the job must not call `s.storymakerRuntime()`, which holds `s.storymakerMu` while `old.jobs.Close()` waits for the running job (`storymaker_service.go:51-52`, `:65-67`, `jobs.go:611`).
- `storymakerRuntime()` at `:65-67` and `closeStorymaker` at `:127-129` call `rt.close()`.
- `newStorymakerRuntime` closes the render manager when the second `NewManager` fails, or its worker would stay alive with no owner.
- Decided here: `storymakerRunOverride` applies to the render manager only. A new `storymakerAudioRunOverride render.RunFunc` serves tests.

New handlers, `storymaker_audio_handlers.go`:

```go
func handleStorymakerVoices(s *Server, w http.ResponseWriter, r *http.Request)
func handleStorymakerNarration(s *Server, w http.ResponseWriter, r *http.Request, id string)
func handleStorymakerNarrate(s *Server, w http.ResponseWriter, r *http.Request, id string)
func handleStorymakerMusic(s *Server, w http.ResponseWriter, r *http.Request, id string)     // [2C]
func handleStorymakerAudio(s *Server, w http.ResponseWriter, r *http.Request, id, folder, file string)
func writeStorymakerErrorWith(w http.ResponseWriter, status int, code, message string, extra map[string]any)
func (s *Server) storymakerVoiceProviders(ctx context.Context) []storymaker.VoiceProvider
func (s *Server) storymakerMusicKey(ctx context.Context, m *storymaker.Music) (string, error) // [2B] returns ""; [2C] the full rule
```

`storymakerVoiceProviders`:
- Order: the active provider first, then phonon, piper, sanotts, supertonic, speech_lab, then the cloud ids sorted.
- google is omitted unless `cfg.TTS.Provider` is google (spec line 487). `tools.TTSReady` returns true for it with no key (`tts_config.go:165-167`) and its languages are `*`, so resolution would otherwise pick an unofficial endpoint on a box that uses a local provider.
- `/voices`, `POST /projects` and `POST /narrate` all use this one list. A story naming a provider outside it answers 409 `narration_unavailable`.

Dispatch at `storymaker_handlers.go:112-146`:
- `parts == ["voices"]`
- `len(parts)==3 && parts[2]=="narration"`, `"narrate"`, `"music"`
- `len(parts)==5 && parts[2]=="audio"`

Narrate runner, `storymaker_narrate.go`:

```go
type storymakerNarratePayload struct{ ProjectID string }
type storymakerMusicPayload struct {
    ProjectID  string
    DurationMS int
}
func (s *Server) storymakerAudioRun(ctx context.Context, rt *storymakerRuntime, job render.Job, payload any, h *render.Handle) (render.Output, error)
func storymakerTTSConfig(cfg *config.Config, v storymaker.Voice) tools.TTSConfig
```

`storymakerTTSConfig`: copy `cfg`, set `cp.TTS.Provider = v.Provider`, call `tools.BuildTTSConfig(&cp, tools.TTSRouteTool, v.Language)`. Then set `Language = v.Language` again, because `tts_config.go:62-68` replaces it for phonon and sanotts. Set the provider's voice field from spec table lines 478-488, and `NoProviderFallback = true`. Supertonic gets `ResponseFormat = "wav"`.

`storymakerAudioRun` starts with `ctx, release, err := s.beginDesktopRun(ctx)` and defers `release()`. A config change that sets read-only or switches the desktop off then revokes the run and cancels a provider call already in flight (`desktop_operation.go:108-140`, `server.go:657-659`). Every error it returns is a `*render.JobError`, because a plain `context.Canceled` would surface as `render_failed` with the text "context canceled" (`jobs.go:676-681`).

Decided here, the job loop. Before every provider call:
1. Re-read `story.json` and the piece list, so an edit during a run is picked up and deleted text is not synthesised.
2. Read `s.ConfigSnapshot()`. `!Storymaker.Enabled` or `!VirtualDesktop.Enabled` fails with `storymaker_disabled`. `VirtualDesktop.ReadOnly` (S5) fails with `narration_unavailable` and a message that names the read-only state, because `render_failed` would tell the user that an export failed.
3. A missing block without a provider fails with `voice_required`. A save during a run can set the voice to null, and `BuildTTSConfig` would turn an empty provider into piper (`tts_config.go:38-41`).
4. Free space of `cfg.VirtualDesktop.WorkspaceDir` below `render.MinFreeDiskBytes` fails with `narration_failed` naming the free size.
5. For a cloud provider, check the caps and reserve. The job cap counts characters already sent in this job plus this piece. The daily count is written to `usage.json` with this piece's characters before the call. Exceeding either fails with `budget_exceeded`.
6. Synthesise one piece. A provider error with no bytes subtracts the reservation and fails with `narration_failed`. Once the provider has answered it has billed, so a decode failure, a write failure or a crash after that keeps the reservation.
7. Decode, write audio and envelope.

The loop ends when nothing is missing. A ctx error without a cancel request fails with `narration_unavailable`, or `music_failed` for a music job, with a message that names the read-only state or the config change.

`[2C]` music job:
1. Source.
   - Generated: read `cfg`, `IsBlocked("music_generation")` when `!cfg.UsesLocalMusic()`, read-only re-check, then `tools.GenerateMusicResult(ctx, cfg, s.MediaRegistryDB, s.Logger, params)` with `Instrumental:true` and `DurationSeconds: clamp(ceil(duration_ms/1000)+2, 10, max)`, then `RecordCostForCategory`. `max` is 600, or when `cfg.UsesLocalMusic()` and the status has a profile, `min(600, acestep.Default().Status().Profile.MaxDuration)` (600 when that is 0). A value above the profile limit fails instead of shortening (`music_generation.go:171-182`, `acestep/types.go:66-71`), and the loop covers the rest of a longer film. After the call, write `audio/music/<key>.pending.json` holding the result's path relative to `DataDir/audio`. A rerun copies from it without a provider call, because a failed copy or a restart would otherwise pay again while the file sits in `DataDir/audio`.
   - File: `Store.Open(music.file)`.
   - Track: `tools.GetMedia`; `item.MediaType` must be `music` (as `noisemaker_handlers.go:712`); `rel`, relative to `DataDir/audio`, must pass `filepath.IsLocal`; the job opens it only through `Store.Open("Music/"+rel)`. `resolveMediaMount` refuses symlink components (`service_media.go:50`). Decided here: this is the meaning of "under DataDir/audio and not protected" in ledger S4. The only protected list in the code is `RuntimePermissions.ProtectedNotesRoots` (`internal/tools/permissions.go:248`), which names Desktop Notes, and the `Music` mount cannot leave `DataDir/audio` (`service.go:661`).
2. Limits, read-only: size at most `maxMusicSourceBytes` (1 GiB), `SniffAudio` of the first 12 bytes is `wav` or `mp3`, free disk at least size plus `MinFreeDiskBytes`. `POST /music` makes these checks too and answers 422 `invalid_music_file` before any job for a file or track source.
3. Copy with `Publisher.WriteStream(ctx, p.AudioPath(id,"music",key+"."+ext), r, maxMusicSourceBytes)`. A `*desktop.PathConflict` (`service_file_stream.go:102-104`) counts as success.
4. `ProbeAudio` on the stored copy, on every path, because Lyria sets no `DurationMs` (`music_generation.go:218` sets it for ACE-Step only). A duration outside 1000..3600000 ms removes the copy and fails with `music_failed`.
5. `WriteMusicMeta` after the file, then remove the pending file. A file without valid meta counts as missing for `/narration` and the gate. A rerun probes it and writes meta without a provider call.

Decided here: the source format must be `wav` or `mp3`, matching the route whitelist. Anything else is 422 `invalid_music_file`.

`POST /narrate` refusals, in order: `voice_required`, `narration_unavailable`, `voice_unknown`, `budget_exceeded`. `voice_unknown`: a block's voice whose provider returned a list (`Voices` not empty, not `FreeText`) and whose id is not on it answers 409 with `alternatives`, at most 5, and a message naming the first block. This is the list check of spec lines 494 and 506. `Validate` cannot do it, because the package does not import `internal/tools` (contract line 60).

Usage file: `storymaker/narrate/usage.json`, `{"v":1,"date":"YYYY-MM-DD","chars":N}`, local date, written with `writeAtomic` semantics, reset on a new date. An unreadable file is renamed to `usage.bad.json` and replaced by one with `chars = max_daily_chars`, so cloud calls stop for that date with a message naming the file. A budget guard that fails open would be a bypass.

Settings: `storymakerSettings` adds `FFprobePath` from `videostudio.ResolveFFprobe(ffmpeg)` and, `[2D]`, `LogicalCPUs` and `FreeMemoryMB`.

Config, `internal/config/storymaker.go` `[2B]`:

```go
type StorymakerNarrationConfig struct {
    MaxDailyChars int `yaml:"max_daily_chars" json:"max_daily_chars"` // default 100000, 1000..100000000
    MaxJobChars   int `yaml:"max_job_chars" json:"max_job_chars"`     // default 20000, 100..10000000
}
// StorymakerConfig gains: Narration StorymakerNarrationConfig `yaml:"narration" json:"narration"`
```

Handlers that change:
- `handleStorymakerProject` DELETE uses `rt.cancelProject`.
- `handleStorymakerJob`, `...ProjectJobs` and `...JobCancel` use the router.
- `handleStorymakerEvents` subscribes to both managers, keeps `revRender` and `revAudio`, and sends first every unfinished job of both managers plus the merged latest five. Five newer narrate jobs must not push a running export out of the list a reloaded app reads (`storymaker_handlers.go:683-686` has the plain cut today).

### 1.5 Package `internal/tools` `[2B]`

- `ttsElevenLabs` (`tts.go:485-488`): the URL uses `url.PathEscape(voiceID)`; the body is `json.Marshal` of `{text, model_id, voice_settings:{stability:0.5, similarity_boost:0.75}}`. Tests cover a voice id with `/` and a text with a quote and a non-ASCII rune.
- New in `tts_cloud.go`: `func OpenAITTSVoices() []string` returning `alloy, ash, ballad, coral, echo, fable, nova, onyx, sage, shimmer, verse, marin, cedar`.

Voices endpoint inputs, all existing: `tools.TTSReady`, `tools.PiperListVoices(port)`, `tools.SupertonicListStyles(baseURL)`, `phonon.Root()`, `phonon.Ready(root)`, `phonon.Voices(root, dataDir)`, `phonon.VoicePath`, `phonon.Language`, `phonon.DefaultVoice(language, persona)`, `(*realtimespeech.Client).FetchXAIVoices(ctx, apiKey)` (cached 5 min), `speechlab` `/ready`.

Provider languages:

| Provider | `languages` |
| --- | --- |
| phonon | `de en es fr pt` |
| sanotts | `ar cs de en es fr id it pt ro ru tr vi` |
| piper | the bases of the listed voice locales, `["*"]` when the list fails |
| speech_lab | `SpeechLab.Language` base, else `["*"]` |
| others | `["*"]` |

sanotts lists no hi and no zh, because `sanotts.Voice` falls back to English there (`sanotts.go:46-65`).

## 2 HTTP

Base `/api/desktop/storymaker`. Error body and shared errors as in contract section 5.

### Public routes

| Method and path | Scope | Batch | Request | Success | Errors besides the shared ones | Read-only | Idempotency |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `GET /voices` | read | 2B | none | 200 `{providers:[VoiceProvider]}`, in the order of section 1.4, `no-store`, no secret. google is absent unless active | none; a provider whose list fails has `ready:false` | works | safe |
| `POST /projects` (changed) | write | 2B | `{title, aspect?, fps?, language?}` | 201 as before; with `language` the story has the resolved `voice`; without it `voice:null` | added: 422 `narration_unavailable` with `languages:[string]` | 403 `desktop_readonly` | not idempotent |
| `GET /projects/{id}/narration` | read | 2B | none | 200 `AudioAnswer` (section 3), `no-store` | 404 `project_not_found`, 422 `invalid_story` | works | safe |
| `POST /projects/{id}/narrate` | write | 2B | empty | 202 `{job}` new; 200 `{job}` existing queued or running job of this project; 200 `{job:null}` nothing missing | 404 `project_not_found`, 422 `invalid_story`, 409 `voice_required` (any block without a voice, message names the first), 409 `narration_unavailable` (provider not ready or not offered, message names it), 409 `voice_unknown` with `alternatives` (at most 5), 409 `budget_exceeded` with `{limit, missing}`, 429 `busy` | 403 `desktop_readonly` | coalesced per project and kind |
| `POST /projects/{id}/music` | write | 2C | `{duration_ms}`, required for mode generated, 1000 to `max_duration_seconds*1000` | 202 `{job}`; 200 `{job}` existing; 200 `{job:null}` when no music is needed or the key's file and meta exist | 400 `invalid_request`, 400 `invalid_argument`, 404, 409 `music_unavailable` (mode generated while `cfg.MusicConfigured()` is false), 409 `budget_exceeded`, 422 `invalid_music_file` (source not wav or mp3 by its bytes, over 1 GiB, or missing, then with `music_issue:"source_missing"`), 429 `busy` | 403 `desktop_readonly` | coalesced per project and kind |
| `GET,HEAD /projects/{id}/audio/narration/{hash}.{ext}` | read | 2B | Range honoured | 200 or 206 through `http.ServeContent` | 404 `audio_not_found`, 405 | works | safe |
| `GET,HEAD /projects/{id}/audio/music/{key}.{ext}` | read | 2B | Range honoured | 200 or 206 | 404 `audio_not_found`, 405 | works | safe |
| `POST /projects/{id}/render` (changed) | write | 2C | empty or `RenderOptions` JSON, at most 16 KiB, strict | 202 `{job}` | added: 400 `invalid_request`, 409 `narration_incomplete` with `missing`, `chars`, 409 `music_incomplete` with `key`, `music_issue?` | 403 `desktop_readonly` | same key, etag and options return the job; other etag or options 409 `idempotency_conflict` |
| `GET /projects/{id}/jobs` (changed) | read | 2B | none | 200 `{jobs}` merged across both queues, newest first | none | works | safe |
| `GET /jobs/{id}`, `POST /jobs/{id}/cancel` (changed) | read; write with `desktopStop` | 2B | none | 200 `{job}` from whichever queue holds it | 404 `job_not_found` | cancel works | cancel repeatable |
| `GET /projects/{id}/events` (changed) | read | 2B | none | event `job` for both queues; the initial list is every unfinished job plus the merged latest five | none | works | stream |
| `GET /status` (changed) | read | 2B, 2C | none | adds `limits.max_queued_narrations` 128, `limits.max_daily_chars`, `limits.max_job_chars`, `[2C] music_configured` bool | as before | works | safe |

Audio route rules (S3):
- The folder segment must be exactly `narration` or `music`, else 404 `audio_not_found`.
- The file segment must match `^([0-9a-f]{64})\.(wav|mp3)$`.
- `ValidProjectID` already runs at `storymaker_handlers.go:120`.
- `.env.json`, `.meta.json` and `.pending.json` are never served.
- Headers: `Content-Type: audio/wav` or `audio/mpeg`, `X-Content-Type-Options: nosniff`, `Content-Security-Policy: sandbox; default-src 'none'`, `Cache-Control: private, max-age=86400, immutable`.

SFX stay at `/img/storymaker/sfx/`.

### Render listener routes, under `<render base>`

| Route | Batch | Request | Answers |
| --- | --- | --- | --- |
| `GET /boot.json` | 2B | none | boot document; `token` on the first GET only |
| `POST /report?t=<token>` | 2B | JSON, at most 8 MiB | 2B: `{"start":true}`. 2C: `{"start":false,"pending":true}`. Refused: `{"start":false,"error":code,"message":text}`. 413 `report_too_large`. 403 `bad_token` |
| `GET /start?t=<token>` | 2C | none | Held at most 20 s, then `{"pending":true}`, `{"start":true}` or `{"error":code,"message":text}`. 403 `bad_token`, 404 before an accepted report. Repeatable |
| `POST /frame?n=<n>&t=<token>` | 2B | one frame | as today; `from_frame` never existed in the contract; 403 `{"stop":true,"error":"bad_token"}` |
| `POST /error?t=<token>` | 2B | as today | `{"ok":true}`; 403 `bad_token` without ending the job |

## 3 JSON shapes

### Compile report additions `[2B]`, sfx `[2C]`

```json
{
  "scenes": [{"id":"s1","start_frame":0,"end_frame":90,"kind":"storyboard",
              "cues":[{"marker":1,"at_ms":1840,"placed":"span"}]}],
  "audio_events": [
    {"kind":"narration","ref":"<64 hex>","start_ms":500,"scene_id":"s1","block_id":"s1:n","piece":0},
    {"kind":"dialogue","ref":"<64 hex>","start_ms":4200,"scene_id":"s1","block_id":"s1:b3","piece":0},
    {"kind":"sfx","ref":"sfx.pop","start_ms":1836,"scene_id":"s1","take":2}
  ],
  "captions": [{"start_ms":500,"end_ms":2600,"lines":["erste Zeile","zweite Zeile"],"scene_id":"s1","block_id":"s1:n"}],
  "cue_precision": 1
}
```

- `cues[].at_ms` is scene-local, rounded, from the scene's time 0. Decided here.
- `placed`: `span` or `proportion`.
- `cue_precision` is the share of cues with `placed:span`, or `null` when the film has no cue. Decided here.
- `start_ms` is `Math.round((sceneBase + blockStart)*1000) + at_ms` for a piece, and `Math.round((sceneBase + eventTime)*1000)` for sfx. `eventTime` is scene-local: `start` from step 7 of section 4.6 for a plain sfx, and the key's event time for a typing key. The value is always film-global.
- `take` is a 0-based index into `takes`. Decided here.
- `captions` are always filled, measured or estimated, regardless of `style.captions`, so SRT works with burn-in off.
- `style.captions` decides drawing only.
- Film-end rule, decided here. After the frame count is known (`compile.js:118-119`), compile drops every audio event with `start_ms >= duration_ms`, lowers each caption end to `duration_ms`, and drops a caption left with no length. Each scene that lost an event gets one warning `invalid_value` at `/scenes/<i>/duration`. The cases: a narration longer than an explicit duration, which is only a warning today (`compile.js:72-77`); a short last line whose 1.0 s minimum outruns the 0.4 s tail (`compile.js:70`); a last code scene shorter than its narration. The Go bounds stay strict for a report that breaks this.
- `needs` stays outside the report in the `compile` return. New key form: `font:<face id>/400`, for example `font:sm-cjk/400`.

### `GET /projects/{id}/narration`

```json
{
  "narration": {"s1": {"blocks": [{
      "id":"s1:n", "kind":"narration", "speaker":"narrator",
      "duration_ms":4210, "spans":[[40,1820],[2170,4110]], "speech_ok":true,
      "levels":[0,0,91,140,171],
      "pieces":[{"hash":"<64 hex>","ext":"wav","at_ms":0,"in_ms":52,"out_ms":1932,"chars":34}]
  }]}},
  "missing": [{"scene_id":"s2","block_id":"s2:n","chars":120}],
  "chars": {"total":640,"missing":120},
  "music": {"key":"<64 hex>","ext":"mp3","duration_ms":61000}
}
```

- `beat` appears on dialogue blocks only.
- `pieces[].chars` is UTF-16 units including joining whitespace.
- `missing[].chars` and `chars.*` are code points of the piece text, which is what cloud caps count.
- `music_issue` appears only as `"source_missing"`, together with `music:null`.

### Audio info (compile input, `load.audio_info`, `boot.audio_info`)

Decided here: `audioInfo = {narration: <the answer's narration>}`. `plan.js:99` reads `audioInfo.narration`, so the wrapper stays. An empty value is `{}`. Music is never in it.

### Envelope file `audio/narration/<hash>.env.json`

```json
{"v":1,"duration_ms":1932,"rate":100,"levels":[0,0,12,91]}
```

### `ui/img/storymaker/glyphs.json`

```json
{"schema":"storymaker-glyphs/1","faces":[
  {"id":"sm-greek","family":"SMGreek","file":"/fonts/UcC73FwrK3iLTeHuS_nVMrMxCp50SjIa1pL7SUc.woff2","weight":"300 700","unicode_range":"U+0370-03FF","d":[880,3]},
  {"id":"sm-greek-ext","family":"SMGreek","file":"/fonts/UcC73FwrK3iLTeHuS_nVMrMxCp50SjIa2ZL7SUc.woff2","weight":"300 700","unicode_range":"U+1F00-1FFF","d":[7936,21]},
  {"id":"geist","family":"Geist","file":"/fonts/Geist-Regular.woff2","weight":"400","unicode_range":"","d":[32,94]},
  {"id":"sm-deva","family":"SMDeva","file":"/fonts/qr/NotoSansDevanagari.ttf","weight":"100 900","unicode_range":"","d":[2304,127]},
  {"id":"sm-arab","family":"SMArab","file":"/fonts/qr/NotoSansArabic.ttf","weight":"100 900","unicode_range":"","d":[1536,255]},
  {"id":"sm-cjk","family":"SMCJK","file":"/fonts/qr/NotoSansCJKsc.otf","weight":"400","unicode_range":"","max_bytes":17825792,"d":[32,94]}
]}
```

The `d` values above show the format only; the build writes the real ones.
- Decode: first `d` value is the first range start, then `gap = start - previous end`, `len = end - start`.
- `geist` data excludes U+0370-03FF.
- A face whose `family` equals a catalog font family adds no need. Its faces are loaded through `catalog.fonts`.
- The manifest adds `"glyphs":{"file":"glyphs.json"}`.
- The file joins the `version` hash and `--check`.

Decided here: the faces stay out of `catalog.fonts` (`validate.go:166`). The host attaches the parsed file as `catalog.glyph_data`. That key exists only in memory, never in `catalog.json`.

### Catalog sfx entry `[2C]`

```json
{"id":"sfx.pop","kind":"sfx","name":"sfx.pop","name_key":"storymaker.catalog.sfx_pop","tags":["pop"],"thumb":"",
 "usage_example":{"do":"sfx","sound":"sfx.pop"},"file":"sfx/pop_1.wav","impact_ms":3.75,"align":"peak",
 "hit":{"over_db":4.5,"dip_db":3},
 "source":{"kind":"forge","basis":"earshot 0.4.0 pop/0","url":""},
 "takes":[{"file":"sfx/pop_1.wav","dur_ms":200,"onset_ms":0,"peak_ms":3.5,"impact_ms":3.5,"l50_lufs":-18,"true_peak_dbtp":-1.5}]}
```

- `sfx.typing` adds `"pattern":{"spacing_ms":[90,140],"default_duration_ms":1200}` and no `hit`.
- `hit`: pop 4.5 and 3, ding 4.5 and 3, success 4.5 and 4, error 4.5 and 4. whoosh, click, typing have none.
- Takes: whoosh 4, pop 4, click 4, ding 3, success 2, error 2, typing 6.
- `align`: `onset` for success and error, else `peak`.

### `boot.json`

```json
{"mode":"render","fps":30,"width":1920,"height":1080,"transport":"png","frame_limit":27000,"frame_deadline_ms":5000,
 "token":"<32 hex, first GET only>",
 "story_url":"<base>/story.json","catalog_url":"<base>/catalog/catalog.json","assets_base":"<base>/catalog/",
 "code_bundles":{"s3":"<base>/scenes/s3.js"},
 "fonts":[{"name":"default","family":"Geist","weight":400,"url":"<base>/fonts/Geist-Regular.woff2"},
          {"name":"sm-cjk","family":"SMCJK","weight":400,"url":"<base>/fonts/qr/NotoSansCJKsc.otf","fallback":true,"weight_range":"400","max_bytes":17825792},
          {"name":"sm-greek","family":"SMGreek","weight":400,"url":"<base>/fonts/UcC73...woff2","fallback":true,"weight_range":"300 700","unicode_range":"U+0370-03FF"}],
 "audio_info":{"narration":{}},
 "workers":4,"chunk":8,"pre_roll":0}
```

`workers`, `chunk`, `pre_roll` land in `[2D]`. Absent means 1, no chunking, 0.

The story at `story_url` carries the burn-in override. `catalog.json` plus `glyphs.json` are in the snapshot, sfx files are not.

### Job object

```
Job {id, project_id, kind:"render"|"narrate"|"music", status, progress:0..1, phase?, frames_done, frames_total,
     pieces_done?, pieces_total?,
     output?:{path, size, stems?:{narration?, music?, sfx?}, srt?, vtt?, library?},
     warnings?:[{code, message}], error?:{code, message}, created_at, updated_at, started_at?, finished_at?}
phase  starting | compiling | mixing | rendering | encoding | writing | synthesizing | generating
```

### Export options

`POST /render` body: `{"burn_in_captions":true,"srt":true,"vtt":false,"stems":true,"quality":"standard"}`. An empty body means `{srt:false, vtt:false, stems:false, quality:"standard"}` and `burn_in_captions` absent, which means `style.captions`.

### Exports layout

```
Documents/Storymaker/<project id>/exports/
  <slug>-<job8>.mp4
  <slug>-<job8>/narration.wav  music.wav  sfx.wav  captions.srt  captions.vtt
  <job id>.json
Videos/Storymaker/<slug>.mp4, <slug>-2.mp4, ...
```

Decided here: the MP4 keeps its phase 1 path, and only the extras live in the folder of the same name.

- Each extra is written after the MP4 with `WriteStream` (stems, cap 2 GiB each) and `WriteBytes` (subtitles).
- A failed extra leaves the MP4 and adds a warning.
- A silent stem is omitted.
- `<job id>.json` gains `stems`, `srt`, `vtt`, `library`, `warnings`, `loudness:{i, tp, master_gain_db}`, `cue_precision`.

## 4 Player JS

Static rules stay: below 600 lines per part, first line a comment, no template strings, no timers or clocks in player parts except `determinism.js` (`scripts/storymaker-tests/10-static-guards.mjs:18-31`).

### 4.1 Bundle part order after all batches

`storymakerPlayerParts`:
`ease, seed [2A], ranges [2A], timeline, schedule, stage, parts, rig, glyphs [2B], text, camera, scene, narration, cuemap [2B], captions [2B], lipsync [2B], plan, motion [2A], bubble [2B], beats, lint [2A], checks, compile, transitions, render, codescene, determinism, worker-player, worker-scene, worker-main`.

`storymakerHostParts`:
`schedule, host-core, host-app, host-pool [2D], host-render, host`.

### 4.2 New modules

**`seed.js` `[2A]`, `SM.seed`**
- `hash(str) -> uint32`: FNV-1a over UTF-16 units, the body of `hashSeed` at `codescene.js:15-19`.
- `random(seedStr) -> fn() -> [0,1)`: mulberry32 from `codescene.js:22-30`. `codescene.js` delegates to these two, so existing scene output is unchanged.
- `takeIndex(sceneId, beatIndex, soundId, key, count, last) -> int`: `i = hash(sceneId+':'+beatIndex+':'+soundId+':'+key) % count`; when `count>1 && i===last`, `i=(i+1)%count`.

No project seed exists. Lip-sync seeds from the block id.

**`ranges.js` `[2A]`, generated, `SM.ranges`.** Generated by `scripts/build-storymaker-ranges.mjs` with `--check`, together with `internal/storymaker/ranges_gen.go`. The tables are committed output, so Chrome and Node ICU never decide at run time. No `Intl.Segmenter`, no `\p{...}`, no `normalize` in compile or in the wrap.

The generator reads a committed data file, `scripts/storymaker-ucd-16.0.0.txt`, as compact range text extracted once from the Unicode 16.0.0 `DerivedGeneralCategory.txt`, `DerivedCoreProperties.txt` and `DerivedBidiClass.txt`. The version goes into the output header. The generator uses no `\p{...}`: ECMAScript regex has no Bidi_Class, and `\p{M}` and `\p{DI}` would tie `--check`, which every matrix runs, to the Node version.

| Function | Result |
| --- | --- |
| `isSpace(cp)` | the JS set of section 1.1 |
| `isWide(cp)` | U+1100-115F, U+2E80-303E, U+3041-33FF, U+3400-4DBF, U+4E00-9FFF, U+A000-A4CF, U+AC00-D7A3, U+F900-FAFF, U+FE30-FE6F, U+FF00-FF60, U+FFE0-FFE6, U+20000-3FFFD |
| `isCjkWord(cp)` | U+3000-30FF, U+3400-4DBF, U+4E00-9FFF, U+F900-FAFF, U+FF00-FF65, U+20000-2FFFF |
| `isMark(cp)` | general category M |
| `isDefaultIgnorable(cp)` | Default_Ignorable_Code_Point |
| `width(text)` | same as Go `TextWidth` |
| `wordCount(text)` | same as Go `WordCount` |
| `dirOf(text)` | `rtl` or `ltr` from the first strong character (R or AL against L); no strong character gives `ltr` |
| `canBreakAfter(prevCp, cp)` | false inside a surrogate pair, before a mark, after U+200D, or after U+094D. It also applies kinsoku. No line start: `。、，．！？）」』】〉》：；・ー々…‥`. No line end: `「『（【〈《` |
| `cjkBreak(prevCp, cp)` | true between two `isCjkWord` characters when `canBreakAfter` allows |

**`glyphs.js` `[2B]`, `SM.glyphs`**
- `index(glyphData) -> idx | null`
- `faceOf(idx, cp) -> face index | -1`
- `needsFor(idx, text) -> [face]`: per code point, skip `isDefaultIgnorable`, `isMark`, `isSpace`, and take the first covering face. Decided here: marks add no need, because compile may not call `normalize`.
- `chain(idx, styleFamily) -> [family...]`: `SMGreek`, the style family in place of `Geist`, `SMDeva`, `SMArab`, `SMCJK`
- `facesForLanguage(idx, lang) -> [face]`: lower-cases the tag and takes its first segment split at `-` or `_`, the same rule as Go's `LanguageBase`, because the validator accepts any language string. Then el gives both Greek faces, hi `sm-deva`, ar `sm-arab`, zh and ja `sm-cjk`, others none
- `key(face) -> 'font:<id>/400'`

**`cuemap.js` `[2B]`, `SM.cuemap`**
- `prepare(rawBlock, length) -> block`. Adds `length`, `starts[]` (cumulative UTF-16 offsets), and `placed`. `placed` is `span` when `speech_ok` and the `chars` sum equals `length`, else `proportion`.
- `estimated(id, seconds, length) -> block`, with `placed: proportion`, `duration_ms`, `pieces: []`.
- `cueMap(block, utf16Offset) -> ms from block start`:
  - `placed: span`: find piece k with `starts[k] <= offset < starts[k]+chars[k]` (an offset equal to `length` is the end of the last piece); `time = at_ms[k] + (offset-starts[k])/chars[k]*(out_ms[k]-in_ms[k])`
  - `placed: proportion`: `offset/max(1,length)*duration_ms`

No vowel weights. Cue and caption times both use this function.

**`captions.js` `[2B]`, `SM.captions`**
- `MAX_LINES = 2`, `MAX_WIDTH = 42`, `MIN_MS = 1000`.
- `split(plain) -> [{a, b, lines}]`: offsets in UTF-16, `b` exclusive, trailing whitespace inside the range. Lines fill greedily by `ranges.width`, breaking at whitespace or at `cjkBreak`. A token over 42 breaks at `canBreakAfter`.
- `forBlock(plain, block, startMs, sceneId) -> [caption]`: `start_i = startMs+round(cueMap(a_i))`, raised to `start_{i-1}+1000` when earlier. `end_i = max(startMs+round(cueMap(b_i)), start_i+1000)`, lowered to `start_{i+1}`.
- `settle(all, filmMs) -> sorted`: across blocks, each `end` is lowered to `min(next start, filmMs)`, and a caption with `start >= end` is dropped. The 1.0 s minimum yields at the film end. It runs after `compile.js:118-119`.
- `at(list, ms) -> caption | null`: binary search.

**`lipsync.js` `[2B]`, `SM.lipsync`**
- `track(block) -> [mouth name per frame]`, seeded with `seed.random(block.id)`.
- `level < 85` is `rest`; `85-127` low; `128-170` mid; `>= 171` high.
- Low picks `e`, or `fv` at probability 0.25. Mid picks `ai`, or `e` at 0.25. High picks `o`, or `u` at 0.25.
- A frame whose level is at least 40 below both neighbours is `mbp`.

A block without levels has no track, and the mouth rests. `scene.evalEntity` adds `s.mouth`, `render.js:116` passes `st.mouth||null`, and `rig.js:173` already resolves it.

**`motion.js` `[2A]`, `SM.motion`**
- `RHYTHM = {calm: 1.25, standard: 1, lively: 0.8}`; `factor(style) -> number`, absent means 1.
- `DEFAULTS`: enter 0.6, exit 0.6, move 0.8, pose 0.3, emote 0.25, look 0.3, show 0.5, hide 0.35, camera 0.8, shake 0.5, wait 1 (never scaled), highlight 0.8.
- `leadFraction(easeName) -> number`, table in 4.5.
- `holdSeconds(text, opts) -> {base, scaled}`: `base = 0.5+0.3*ranges.wordCount(text)`, and for a secondary line (style `caption` or `label` in a scene that shows a `title`) `max(base, 1.2)`; `scaled = base*max(1,factor)`.
- `OVERLAP = 0.4`.

**`lint.js` `[2A]`, `SM.lint`**
- `scene(c) -> [Issue]`, `c = {scene, index, plan, built, stage, cat, fmt, envChanged}`.
- `film(c) -> [Issue]`, `c = {scenesIn, plans}`.
- Storyboard scenes only, from `built.log`.

| Code | Rule |
| --- | --- |
| `line_unread` | text visible from landing to its hide or the scene end for less than `base`; `at_seconds` is the landing |
| `dead_stretch` | more than 3 s of `[T, D]` with no beat in progress (a `wait` does not count), no narration or dialogue playing, no camera move, a text counting from landing to hold end |
| `first_frame_empty` | at `t=T` no text or prop with alpha above 0 and no actor with alpha above 0 inside `stage.vis` |
| `text_too_small` | static: `style.size*height/min(width,height) < 0.026`, for every style drawn in the scene |
| `fade_between_worlds` | `plan.transition==='fade'`, index above 0, environment differs from the previous scene's. Variant alone does not count |
| `entry_monotony` | film has at least 3 show beats, all with the same entry; minor, path `/scenes` |
| `transition_variety` | more than 5 requested kinds in the film; minor, path `/scenes` |

`cue_late` comes from `beats.js`.

**`bubble.js` `[2B]`, `SM.bubble`**
- `layout(ctx, lines, style, opts) -> lay` and `draw(ctx, lay, anchor, kind, resolve, alpha)`, where `kind` is `speech` or `thought`.
- `anchor` is the actor's head point in the camera window (`stage.toCanvas` with `win`, feet minus `ACTOR_REACH*depth`, `beats.js:13`), clamped so the box stays in the safe area.
- Speech draws a rounded box with a triangle tail. Thought draws a rounded box with three circles.
- Text goes through `SM.text`.

Decided here: new catalog text styles `text.speech`, `text.thought`, `text.subtitle`, each size 0.042. A bubble fades in and out over 0.12 s, not rhythm scaled. `text.subtitle` draws burned captions in `renderFrame`, after the transition composite, in every render including thumbs, so the parity recipe holds. Caption box bottom edge: 92 % of the height in 16:9 and 1:1, 82 % in 9:16.

### 4.3 Changes to existing modules

**`ease.js` `[2A]`**
- Internal curves are added to `FNS`: `quart-out`, `expo-out`, `spring`.
- `SM.isEase` keeps returning true only for the six public names, so a story or a code scene cannot select them.
- `spring(u)`, with `zeta=0.75`, `w0=7.6`, `wd=w0*sqrt(1-zeta^2)`: `1 - exp(-zeta*w0*u)*(cos(wd*u)+(zeta*w0/wd)*sin(wd*u))`, and 1 at `u>=1`.

**`timeline.js` and `camera.js` `[2A]`**
- `Track.prototype.add(t0, t1, to, ease, from, log)`.
- With `log`, `evalSeg` returns `from*Math.pow(to/from, s)`.
- `camera.addZoom` always passes `log:true`, code scenes included.
- `StepTrack.blendAt` returns `u` eased by `ease-in-out`.

**`narration.js` `[2B]`**: `parse.words` uses `ranges.wordCount`. Decided here: it feeds holds and the estimate, and is unchanged for text without CJK.

**`plan.js` `[2B]`**
- `planScene` finds `info.blocks` by id, never by position.
- `narration` gains `block` (a prepared block or an estimated one), `cueList:[{marker, at_ms, placed}]`, `dialogue:{<beat index>: prepared block}`.
- `cues[n] = T + cueMap(block, parsed.cues[n])/1000`.
- `planTiming` adds `fontChain`, the result of `glyphs.chain(idx, '$style')`, absent without glyph data.

**`text.js`**

| Function | Change |
| --- | --- |
| `fontString(weight, px, family)` | `[2B]` `family` may be an array; each name is quoted, `sans-serif` last |
| `fitEllipsis` | `[2B]` trim set adds `،؛؟۔…。、，！？：；` |
| `breakWord` | `[2B]` breaks only where `ranges.canBreakAfter` allows, iterating code points |
| `wrap(measure, text, maxWidth, maxLines)` | `[2B]` a token also breaks at `cjkBreak` |
| `balance(measure, lines, maxWidth, maxLines) -> lines` | new `[2A]` narrows the width by bisection until the line count would grow |
| `layout(ctx, text, style, opts)` | `[2A]` shrink loop, see below. `[2B]` adds `dir` |
| `draw` | `[2B]` sets `ctx.direction = lay.dir` before measuring and drawing |

Layout returns the existing fields plus `dir`, `shrunk` and `overflow`.

Shrink: `px` steps by 0.94 while the wrap truncates, floor `max(0.45*px0, 0.026*min(W,H))`, then `balance`. Pass `opts.short` for the floor.

Decided here: `style.align` stays physical for RTL. Compile never measures. Layout results may be memoised, keyed on text, style name, canvas W and H, family chain and `maxWidth`, and the memo is cleared at 512 entries. `maxWidth` belongs in the key, because `render.js:171` derives it from `pos.x`, and `pos.x` moves during a `slide` entry (`beats.js:162`) or a move. Without it two workers that start at different chunks could cache different wraps, and frame n would stop being a pure function of n. A test renders a sliding text at its frames in both orders.

**`scene.js`**
- `[2A]` `e.highlights = new Intervals()` on actors, props and texts.
- `[2B]` entity type `bubble {owner, kind, lines|captions, alpha}`, `e.speech = new Intervals()` on actors, and `createBubble`. For a say without text, `captions` holds the scene narration's caption lines in scene-local ms.
- `indexCatalog` keeps `sfx` entries through `byId`.

**`beats.js`**: `buildStoryboard` returns the existing fields plus `holdsEnd`, `blocksEnd`, `log:[{j, kind, tgt, start, end, landing, cued}]`, `audioEvents:[...]`.

Highlight `[2A]`:
- `pulse`: scale times `1+0.08*sin(2*PI*2*u)`. `drawText` reads no scale today (`render.js:166-173`), so it applies a canvas transform around the anchor while a pulse runs.
- `outline`: 6 design pixel stroke in `$accent` around the entity box.
- `arrow`: a `$accent` triangle 40 design pixels above the entity, bobbing 8 pixels.

**`compile.js`** adds to the report as in section 3, runs `SM.lint`, and keeps `compile(story, audioInfo, catalog, opts)`. It applies the film-end rule of section 3, and passes `lastTake`, a `Map(soundId -> take)` that spans the film, through `c` into `buildStoryboard`.

`autoD = max(T+n.seconds+tail, built.blocksEnd > 0 ? built.blocksEnd+TAIL_SECONDS : 0, built.beatsEnd, built.holdsEnd, T+(n.seconds > 0 ? 0 : MIN_SCENE_SECONDS))`. The default `built` literal at `compile.js:53` gains `blocksEnd: 0, holdsEnd: 0`, or `Math.max` gives NaN for a code scene. `tail` at `compile.js:69` is 0 for a scene without narration, so a dialogue-only scene adds `TAIL_SECONDS` itself.

Needs `[2B]`:
- Glyph faces: every show text, say text, and bubble text; narration when the scene is not silent and captions are on or a say without text uses it; U+2026 always.
- Base font: `compile.js:107-113` adds a `fontKey` for `text` entities only. It also adds the key of `text.subtitle` when captions are drawn, and of `text.speech` or `text.thought` per bubble. Without this a Latin story with narration and no on-screen text draws its captions in a host font.
- Code scenes pass `language` and `fontChain` through `buildScene` opts (`host-core.js:223-226`), and `codescene.build` adds `facesForLanguage` keys to `needs`.

**`render.js` `[2B]`**: `makeContext` (`render.js:17-23`) copies `timeline.fontChain` into `rc.fontChain`. `familyOf` (`render.js:32-35`) returns `rc.fontChain` with `$style` replaced by the family it finds today, when present. `drawCaptions(ctx, timeline, n, rc)` runs last in `renderFrame`, whichever branch drew the frame, when `timeline.style.captions` is true.

**`worker-scene.js` `[2B]`**: the timeline object at `worker-scene.js:81` gains `fontChain: o.fontChain`, because `g.text` draws through `familyOf` (`codescene.js:270`). Without it the CJK face loads and is never named in `ctx.font`, so Japanese draws from host fonts.

**`worker-player.js`, `worker-scene.js` `[2B]`**: `new env.FontFace(f.family, f.data, {weight: f.weight_range || String(f.weight), unicodeRange: f.unicode_range})`, omitting `unicodeRange` when empty.

**`worker-player.js` `[2D]`**: `render` with `msg.png` returns `{blob}` from `main.canvas.convertToBlob({type:'image/png'})`.

**`host-core.js`**
- `[2B]` `fontDescriptor` also reads `catalog.glyph_data.faces` and adds `weight_range`, `unicode_range`.
- `[2D]` `Engine.load` accepts `msg.shared_code_scenes`, and then builds no scene workers.
- `[2D]` `Engine.render(n, {png:true})` returns a Blob.

### 4.4 Role ease table `[2A]`

An explicit `ease` always wins. `ease-out` stays the existing cubic.

| Beat | Tracks | Default ease |
| --- | --- | --- |
| `enter`, `exit`, `walk` | x, y | `linear` |
| `enter` | alpha step at start | `linear` |
| `exit` | alpha over the last 0.2 s | `ease-in` |
| `move` | x, y | `quart-out` |
| `show` entry `fade` | alpha | `quart-out` |
| `show` entry `slide` | alpha, x | `quart-out` |
| `show` entry `pop`, not text | alpha `quart-out`; scale from 0.6 | `spring` |
| `show` entry `pop`, text | alpha | `quart-out` |
| `hide` | alpha | `ease-in` |
| `camera` `focus` | zoom, cx, cy | `expo-out` |
| `camera` `zoom`, `pan` | zoom, cx, cy | `ease-in-out` |
| `pose` | blend | `ease-in-out` |
| `gesture` | keys | `ease-in-out` per segment (contract 3a item 15) |

Decided here: `exit` always walks (`beats.js:197`, and `validate.go:833` requires `to`), so the "exit that does not walk" of the spec does not exist yet. The 0.35 s applies to `hide`.

Decided here: `emote` has no ease, because the expression is a step (`scene.js:105`) until the emote takes of phase 4.

### 4.5 Lead fractions

Fraction of the duration until the value first reaches 0.9, or for `spring` until it first crosses 1.

| Ease | Fraction | Ease | Fraction |
| --- | --- | --- | --- |
| `linear` | 0.9 | `quart-out` | 0.4377 |
| `ease-in` | 0.9655 | `expo-out` | 0.3322 |
| `ease-out` | 0.5358 | `spring` | 0.4812 |
| `ease-in-out` | 0.7076 | `back-out` | 0.2955 |
| `bounce-out` | 0.3450 | | |

`spring` is computed from the formula. A test pins each value to 0.001.

### 4.6 Scheduling algorithm `[2A]`, say `[2B]`, sfx impact `[2C]`

State: `cursor=T`, `prev=null`, `anchor=null`, `blockEnd=T+narration.seconds`, `lastEnd=Map(instance -> end)`, `beatsEnd=0`. `f = motion.factor(story.style)`. `lastTake` comes from `compile.js` and spans the film.

1. Skip non-objects. An unknown kind, `chart`, or a show with `icon` or `chart` gets its minor issue and leaves the state untouched.
2. `tgt` is the resolved instance: `who`, else `target`, through `find`. A show's target is the instance it creates. `wait`, `sfx`, `chart`, a camera without target have none. A string that resolves to no instance (a camera on a spot) has none. Compare instances, never strings.
3. `floor = tgt ? (lastEnd.get(tgt) || 0) : 0`.
4. `dur`:
   - an explicit `duration >= 0` wins
   - `wait`: `seconds` when `>= 0`, else 1, never multiplied by `f` (`beats.js:315`; Go requires a value above 0, `validate.go:944-946`)
   - else `DEFAULTS[kind]*f`; gesture is catalog duration times count times `f`; camera shake is `DEFAULTS.shake*f`; show entry `none` is 0
   - `walk`: `max(0.3, distance/speed)*f`, with the distance from the target's position at `max(floor,0)` (D10); no circular dependency, because only same-target beats change that track
   - `say` with text: block seconds. `say` without text: 0
   - `sfx`: 0, or for a pattern `default_duration_ms/1000*f`
5. Class and offsets:
   - Arrival (`enter`, `walk`, `move`, a show with entry other than `none`, camera `focus`, camera `zoom` with a target): `lead = leadFraction(ease used)*dur`. The chart lead lands with the chart beat in phase 4.
   - Impact (`sfx`): `impact = chosen take impact_ms/1000`, 0 when the catalog has no such entry (`[2A]` and `[2B]`, which also keep a minor `not_supported_yet`). The take comes from `seed.takeIndex` with `lastTake.get(sound)`, and `lastTake` is updated.
   - Start (everything else, `say` included): both 0.
6. `t`:
   - a. `cue` resolves: `t = cueTime + delay - lead - impact`, `cued = true`. A cue beats `with`.
   - b. else `with:"prev"` and `prev` exists: `sfx` gives `(prev.landing ?? prev.start) + delay - impact`; others give `prev.start + delay`.
   - c. else `prev && prev.tgt && tgt && prev.tgt !== tgt`: `t = anchor.start + 0.4*anchor.dur + delay`.
   - d. else `t = cursor + delay`.

   An unresolved cue warns `unknown_cue` and falls through to b, c or d.
7. Rules a, c, d: `start = max(0, floor, t)`. Rule b: `start = max(0, t)`, with no floor, because spec line 346 keeps a `with:"prev"` beat without a cue running together with the previous beat, also on the same target (walking and pointing, pinned at `pt/04-compile.mjs:68`). When `cued` and `floor > t + 1e-6`, emit `cue_late` at `<beat>/cue` with `at_seconds = start+lead`. A clamp at 0 never warns.
8. `say` with text: `start = max(start, blockEnd)`. It then sets `blockEnd = start + blockSeconds`, marks `blocksEnd`, and emits one `dialogue` event per piece. An estimated say with text gets a warning `narration_missing` at `/scenes/<i>/beats/<j>/text`.

   `say` without text keeps its step 7 start and leaves `blockEnd` unchanged. Its bubble stays until `T+narration.seconds`, with the 0.12 s fades, and its lines come from `SM.captions.at` at each time. A start at or after that end draws no bubble. In a silent scene no bubble is drawn. Its `dur` stays 0, so `cursor` and `lastEnd` do not move.
9. Run the kind at `[start, start+dur]` with the eases of 4.4. A say with text adds the bubble and, with measured levels, a `speech` interval on `who`. When the actor is off stage, `actorOf` returns null with its warning: the events stay, and there is no bubble and no interval.
10. `landing = arrival ? start+lead : null`, `end = start+dur`. `lastEnd.set(tgt, max(existing,end))`. Log the beat.
11. `cursor`: rule b gives `max(cursor,end)`, rules a, c, d give `end`.
12. `anchor`: under rule b, replace `anchor` only when `end >= anchor.end`; otherwise `anchor` is this beat. `prev` is always this beat.
13. `beatsEnd = max(beatsEnd, end)`.

After the loop:
- `holdsEnd`: the largest `landing + scaled hold` of text that no `hide` removes.
- Typing `[2C]`: `prng = seed.random(sceneId+':'+j+':'+sound)`; `x` starts at the beat's start plus the first take's impact; emit key events at event time `x - impact` of that key's take, with `x` advancing by `90+50*prng()` ms while `x < start+dur`; `key` counts up; take from `seed.takeIndex`.
- Auto-SFX `[2C]`, when `style.auto_sfx`: each show with entry `pop` gets a `sfx.pop` event with its impact on the landing, unless an sfx beat has the same `cue` or runs `with:"prev"` on it.

### 4.7 Rhythm

- Scaled by `f`: default beat durations, the walk duration, the typing default, the scaled hold (never below base).
- Never scaled: explicit `duration`, `delay`, `wait.seconds` and the default wait, cue times, transition durations, narration and its estimate, the 0.4 s tail, the 1.0 s minimum, eases.

Decided here: code-scene API defaults are not scaled, because they are the author's own timing rather than compile defaults (`codescene.js:119-121` stays).

### 4.8 What the render page does

1. `GET boot.json`. The token stays in a closure variable. `post` and `getJSON` take a route name or a label and add the base and the token themselves, and a failure text names only that route or label. So neither the token nor the prefix enters `__smStatus`, the `/error` body or the job record (`host-render.js:33`, `:42`, `:173-174` put them there today). The page sends `t` only when it has a token, and an absent `boot.token` passes validation, because the stub servers send none. Validate the document, set the canvas and the deadline.
2. Fetch the story and the catalog. When `catalog.glyphs` is set, fetch `assets_base + catalog.glyphs.file` into `catalog.glyph_data`. Fetch the code bundles.
3. Fonts are lazy `[2B]`: `load.fonts` is empty and `env.requestAssets` fetches `font:` keys through a map of `boot.fonts` by `name/weight`, caching the buffer per key. Any other font key is missing. A response over `max_bytes`, default 8 MiB, is a readiness failure.
4. `engine.load`, then `prepare()`.
5. `[2D]` Create `workers-1` helper engines with `shared_code_scenes` and no bundles. Each loads and prepares. A different `frame_count` or schedule is `render_failed`. Decided here: this happens before the report, so it cannot trip the 10 s frame watchdog.
6. `POST /report?t=`. An `error`, or neither `start` nor `pending`, ends in phase `refused`.
7. `[2C]` On `pending`, phase `waiting`: loop `GET /start?t=` with no page timer. `pending` loops, `error` ends in `refused`, `start` continues.
8. Screenshot transport: install `__smRenderFrame`, phase `ready`.
9. Serial path (`workers` 1): draw, encode, `POST /frame?n=<n>&t=<token>`. On `restart`, `n=0`. On `stop`, stop. `done` ends. `from_frame` is gone.
10. `[2D]` Parallel path in `host-pool.js`, `H.runPool({engines, limit, chunk, preRoll, post, status}) -> outcome`:
    - chunk j goes to worker `j % N`, and a chunk with code-scene frames goes to worker 0
    - a worker waits while its chunk starts `>= N*C*2` frames past the next frame to post
    - one in-order poster, one POST in flight
    - PNG encoded in the worker
    - on `restart`, stop the poster, abandon the generation, clear the buffer, re-run the same engines from frame 0
    - a crash or deadline rejects, and the page posts `/error`
    - the parallel smoke forces one encoder failure
11. Any failure: `POST /error?t=`. A failure before the token is known, such as a failed `boot.json` GET, answers 403 and does not end the job, which then ends at the 3-minute report wait (`render.go:455-460`). Accepted.

## 5 App JS

Every module follows the guards in `ui/desktop_storymaker_guards_test.go:44-87`: `'use strict'`, first line a comment, no template strings, no `alert`, `confirm`, `prompt`, `innerHTML`, storage; classes `stm-`; each key literal in `lang/storymaker/en.json`.

Loader order `[2B]`-`[2D]`: `core, model, api, assets, player, projects, strip, inspector, code, export, audio, voice, music, dialog, handoff`, then `storymaker.js`. `storymakerModules` in `ui/desktop_storymaker_registration_test.go:11` and the `module-loader.js` entry follow. `storymaker.js` has 162 lines left, so each panel is a module and the root gets only wiring.

### `storymaker-core.js` errors `[2B]`

- `errorFromResponse` (`:271-282`) gains `data: b`, and `normalizeError` (`:257-268`) gains `data: body`. Panels read `err.data.languages`, `err.data.missing`, `err.data.chars`, `err.data.limit`, `err.data.key`, `err.data.music_issue` and `err.data.alternatives`. These fields are lost today.
- `describeError` gets explicit branches ahead of the 422 line at `:297` for `narration_unavailable` and `[2C]` `invalid_music_file`, and branches for `voice_unknown` and `[2C]` `music_unavailable`, with keys `storymaker.error.<code>`.

### `storymaker-api.js` additions

```
voices() -> {providers}
narration(id) -> AudioAnswer
narrate(id) -> {job}
music(id, durationMs) -> {job}
noisemakerTracks() -> {items}               // [2C] GET /api/desktop/noisemaker/tracks with spec.raw (storymaker-api.js:27)
listJobs(id) -> [Job]
startRender(id, etag, key, options?)        // [2C] options sends a JSON body
narrationUrl(id, hash, ext) -> string
musicUrl(id, key, ext) -> string
fontBuffer(file, maxBytes?)                 // default 8 MiB (storymaker-api.js:130)
```

Only `narrationUrl` and `musicUrl` build audio URLs, from values in the parent's own `/narration` answer. They refuse a hash or key outside `^[0-9a-f]{64}$` and an ext outside wav, mp3. SFX URLs come from the catalog as `/img/storymaker/<take.file>`. Nothing is built from player-message text.

### `storymaker-assets.js` changes `[2B]`

- `loadCatalog` attaches `raw.glyph_data`.
- `font:<id>/<weight>` resolves in `glyph_data.faces` too, with `api.fontBuffer(file, face.max_bytes)`.
- The file regex at `storymaker-assets.js:48` becomes `^\/fonts\/(qr\/)?[A-Za-z0-9_.-]+$`.

### `storymaker-audio.js` `[2B]`, music and SFX `[2C]`

```
S.createAudio({api, projectId}) -> {load(info, events), play(frame), pause(), seek(frame), sync(frame), dispose()}
  info   = {fps, frame_count, narration: answer.narration, music: answer.music, volume, catalog}
  events = report.audio_events
S.audioModel.waveform(report, narration) -> Uint8Array(frame_count)   // pure, no DOM
S.audioModel.windowFor(events, ms) -> events                          // pure, no DOM
S.bindAudio(inst) -> {refresh(), dispose()}
```

- `AudioContext` is created on the first `play`.
- Narration and SFX: `decodeAudioData` of pieces in a sliding window, 2 s behind to 20 s ahead of the playhead.
- Bed: one `<audio>` through `MediaElementSource`.
- Decided here: `sync(frame)` is added to the ledger's method list. The owner calls it on each `time` message while playing, and drift above 120 ms reschedules.
- Ducking is a WebAudio gain from stored `levels`, with no carve.
- The iframe stays without audio or connect access.
- `bindAudio` sets `inst.audioInfo`, `inst.audio`, `inst.refreshAudio()`, and emits bus event `audio`.
- Refresh triggers: project open, a successful save, a finished narrate or music job, on every terminal status.
- `reloadPlayer` (`storymaker.js:391`) sends `audio_info: {narration: inst.audioInfo ? inst.audioInfo.narration : {}}`.
- `bindAudio` calls `reloadPlayer` when the narration JSON of a fresh answer differs from the one last sent, and `inst.loadSeq` makes the newest win. `storymaker.js:218` loads the player before any answer exists, and compile input would otherwise stay old until the next edit. The following `compiled` message feeds `createAudio.load`. An answer for another project is ignored.

### `storymaker-voice.js` `[2B]`

`S.createVoicePanel(inst) -> {el, paint(), dispose()}`:
- provider select, voice select or free-text field, audition, `not tested` mark, facts from `note`
- language select: the value comes from `voice.language`, else the desktop language (`window.SYSTEM_LANG`, as `module-loader.js:703` reads it), limited to the selected provider's `languages`. A provider with `["*"]` shows a free-text field with that value. A phase 1 story has no language to read, and the language decides the piece key and the code-scene faces.
- audition, decided here: plays `narrationUrl` of the first piece of the first block in the current `/narration` answer through an `<audio>` element, and is disabled when no piece exists. It never synthesises, so no paid route exists outside the job loop and its caps.
- `not tested`: shown when `LanguageBase` of the language is not in the provider's `tested_languages`, which is every provider in phase 2.
- one action writes the voice `{provider, voice, language}` into the story, for phase 1 stories with `voice:null`
- captions toggle writing `style.captions`, because captions default to on and phase 1 stories start drawing them (section 8)
- narration status (`chars`, `missing`, progress from `pieces_*`), Generate button
- terminal states: its own text for `interrupted` and `cancelled`, because `describeJobError` gives the unknown text for those codes (`storymaker-core.js:313-317`)

Scrubber waveform: `createTransport` gains a `.stm-scrub-wave` canvas inside `.stm-scrub`, painted from `S.audioModel.waveform`, `pointer-events:none`.

New project: `storymaker-projects.js:69` sends `language` (desktop language). On `narration_unavailable`, it offers create without a voice.

`[2B]` Narrate job events: the listener at `storymaker-export.js:267` filters to `kind === 'render'`, and the voice panel takes `narrate`, the music panel `music`.

### `storymaker-music.js` `[2C]`

`S.createMusicPanel(inst) -> {el, paint(), dispose()}`: mode, mood, prompt, file picker through `window.AuraDesktopFileDialogs.open` with a filter for wav and mp3 (`file-dialog-runtime.js:569`), Noisemaker track through `api.noisemakerTracks()`, volume. Generated mode is shown only when `status.music_configured`.

Jobs:
- Mode generated runs `api.music(id, report.duration_ms)` only from an explicit button click that shows the mood and the prompt. Autosave after each pause in typing changes the key and refreshes `audioInfo` (`storymaker.js:717-725`), so an automatic call would pay for every pause, and the default daily quota is unlimited (`music_generation.go:187-188`).
- Modes file and track cost nothing, so when music is on and `audioInfo.music` is null the panel makes one `api.music` call per key without a click.
- `music_issue: "source_missing"` shows the missing source and offers another file.
- Terminal states: its own text for `interrupted` and `cancelled`, as in the voice panel.

### `storymaker-export.js` silent note `[2C]`

The key `storymaker.export.silent_note` at `storymaker-export.js:145` ("This version exports silent videos", `en.json:178`) is removed in all 16 locales. A new `storymaker.export.silent_film` shows only when `audioInfo` has no narration and no music and `report.audio_events` is empty.

### `storymaker-dialog.js` `[2D]`

`S.createExportDialog(inst) -> {open(), close(), dispose()}`: a native `<dialog>` built with `el()` and `showModal()`, which the guards allow.
- Fields: quality, burn-in captions (preset from `style.captions`), SRT, VTT, stems, Open in Video Studio.
- Open in Video Studio forces `stems:true` and adds the job id to `inst.handoffWish` (a `Set`, session only).
- `createExport` keeps the button and finished-job actions, and calls `inst.api.startRender(id, etag, key, options)`.
- Blockers gain `narration` and `music`, from `audioInfo`.

### `storymaker-handoff.js` `[2D]`

```
S.handoff.canOffer(job, vsStatus) -> bool
S.handoff.run(inst, job) -> Promise<{projectId}>
S.handoff.buildTracks(project, assets, frameCount, names) -> tracks    // pure; Node test
  assets = {video: id, narration?: id, music?: id, sfx?: id}
```

`canOffer`: `job.status === 'succeeded'`, `job.output.stems` present, `vsStatus.enabled && desktop_enabled && ffmpeg_ready && !read_only`.

`run`:
1. `POST /api/desktop/video-studio/projects {name, width, height, fps, ntsc:false}`.
2. For the MP4 and each existing stem, `POST .../projects/{id}/media {source_path, folder:""}` with `Idempotency-Key: storymaker-<job id>-<name>`. Decided here: `folder` is empty, so no folder list is needed.
3. Poll `GET .../jobs/{id}` every 500 ms, at most 120 s, until `succeeded`.
4. `GET` the project for the fresh ETag and assets.
5. `buildTracks`, and keep each asset's `id`, non-blank `name`, `folder`.
6. `PUT` the whole project with `If-Match`, version 3.
7. `inst.ctx.openApp('video-studio', {projectId})`.

`buildTracks` output:
- Video track `track-<uuid>`, kind `video`, with one clip `clip-<uuid>`, `audio_detached:true`.
- One audio track per existing stem in the order narration, music, sfx.
- Every clip: `{id, asset_id, start:0, offset:0, duration:min(asset.duration_frames, frameCount), x:0, y:0, width:1, height:1, rotation:0, opacity:1, volume:1, fade_in:0, fade_out:0, fit:'contain'}` (`video-studio-timeline.js:62-63`).
- Tracks have `{id, kind, name, muted:false, hidden:false, locked:false, clips}`.

### Player message whitelist

**`storymaker-player.js` `checkIncoming`, `compiled`.** `LIMITS` adds `events: 40000`, `captions: 8000`, `cues: 400`, `lineChars: 400`.

The cleaned report adds:
- `audio_events`: each `{kind in narration|dialogue|sfx, ref (at most 80 characters), start_ms, scene_id, block_id?, piece?, take?}`
- `captions`: each `{start_ms, end_ms, lines (at most 2 strings, clipped), scene_id, block_id}`
- `cue_precision`: a number or null
- `scenes[].cues`: each `{marker, at_ms, placed}`

A malformed entry is dropped, not the message. The parent uses these for display and preview only, and never forwards them.

**`host-app.js` `checkMessage`, `load`.** The signature becomes `checkMessage(msg, loaded)`, because an `assets` message carries no catalog (`host-app.js:62-71`) and the CJK face always arrives through `assets`. `loaded` is `{faces, fontBytes}`, kept in `mountApp` state, set when a load is accepted, cleared in `reset()`, and passed at `host-app.js:248`. A missing `loaded` means no faces and 0 bytes.

`CAPS` adds `maxAudioScenes: 400`, `maxAudioBlocks: 61`, `maxAudioPieces: 800`, `maxLevels: 27000`, `maxTotalLevels: 200000`, `maxGlyphFaces: 16`, `maxGlyphRanges: 60000`, `maxFallbackFontBytes: 18 MiB`.
- `audio_info.narration` is checked against those caps. `maxLevels` is per block, and `maxTotalLevels` is per message.
- `catalog.glyph_data` needs the schema string and caps.
- A font over 8 MiB passes only if `name` matches a face in `loaded.faces`, or for a `load` in that message's own `catalog.glyph_data`, whose `max_bytes` holds it. The same check runs on `font:` values in `assets` messages, which today have none (`host-app.js:42` checks `load` only). Without faces an over-8 MiB font is refused.
- `loaded.fontBytes` accumulates across `load` and `assets`, so the 32 MiB total holds over both.
- `load.fonts[]` may carry `weight_range` (at most 16 characters) and `unicode_range` (at most 256).

### `VideoStudioApp` additions `[2D]`

- `window.VideoStudioApp.openProject(windowId, projectId)` (`video-studio.js:884`): when the instance is not `studioReady`, it stores `s.ctx.projectId` and returns. Otherwise `await loadProjects(s)`, then `switchProject(s, projectId)` when listed, so the dirty guard at `:456-463` runs.
- `initialize` at `:375`: prefers `s.ctx.projectId` when it is in `s.projects`, else `projects[0]`.
- `window-shell-runtime.js` reuse branch after `:1615-1619`: when `context.projectId` is a non-empty string and not `sessionRestore`, call `openProject` for a live instance, else store `existing.pendingProjectId`.
- `menus-and-routing.js:1649`: `launch` also merges `{projectId: win.pendingProjectId}` and deletes it.

These three shell files and `main.bundle.js` are dirty in the main checkout, so rebuild the bundle at merge. `window-shell-runtime.js` and `menus-and-routing.js` are parts of `main.bundle.js` (`scripts/build-ui-bundles.js:25`, `:64`). `module-loader.js` is not.

### Lists the guards read

- `JOB_ERROR_CODES` (`storymaker-core.js:311`) adds `narration_failed`, `narration_unavailable`, `voice_required`, `budget_exceeded` `[2B]`, then `narration_incomplete`, `music_failed`, `music_incomplete` `[2C]`.
- `PHASES` (`storymaker-export.js:12`) stays render-only.
- Locale files `ui/lang/storymaker/*.json`: one final task per batch. Config labels (`ui/lang/config/common`) and generated catalog names (`ui/lang/storymaker_catalog`) are outside that rule.

## 6 Constants

| Name | Value | Source | Lives in |
| --- | --- | --- | --- |
| Fragment width | under 20 | first guess (cost) | `narration.go` |
| Fragment words | 3 or fewer | first guess (cost) | `narration.go` |
| Split part minimum | 40 runes | first guess | `narration.go` |
| Chunk limits | phonon 2000, sanotts 2000, google 200, others 400 | code, spec | `narration.go` `ChunkLimit` |
| Say text limit | 1500 characters, `MaxNarrationChars` | spec limit for narration | `validate.go` |
| CJK words | 1 per 2 characters | first guess | `ranges` generator |
| Unicode data | 16.0.0, committed file | decided here | `scripts/storymaker-ucd-16.0.0.txt` |
| Decoded piece | at most 600 s | first guess | `audio_decode.go` |
| WAV rate | 4000 to 192000 Hz | code, `evidence/tts.md` | `audio_decode.go` |
| Envelope rate | 100 Hz | spec D5 | `envelope.go` |
| Level scale | -60 to 0 dBFS, 0..255 | D5 | `envelope.go` |
| Speech threshold | -40 dBFS, byte 85 | measured | `envelope.go` |
| Gap bridge | under 100 ms | measured | `envelope.go` |
| In margin | 40 ms | first guess | `envelope.go` |
| Out margin | 100 ms | first guess | `envelope.go` |
| Clip fade | 5 ms | measured (click risk) | `mix.go` |
| Sentence pause | 350 ms | first guess | `envelope.go` |
| Split pause | 150 ms | first guess | `envelope.go` |
| Words per second, tail, minimum | 2.3, 0.4 s, 1.0 s | spec | `narration.js`, `plan.js` |
| Narrate queue | 128 | spec | `storymaker_service.go` |
| Render queue | 16 | spec | `jobs.go` |
| Max job chars | 20000 | spec | `config/storymaker.go` |
| Max daily chars | 100000 | spec | `config/storymaker.go` |
| Voice alternatives | at most 5 | contract line 60 | `storymaker_audio_handlers.go` |
| Report cap | 8 MiB | measured need | `listener.go` |
| Report event ceiling | 32768 | first guess | `report.go` |
| Report captions | at most 8000 | parent cap | `report.go` |
| Caption line cap | 256 UTF-16 units | first guess | `report.go` |
| Largest rhythm factor in Go | 1.25, pinned by a test | motion.js | `report.go` |
| Token | 32 hex characters, shorter refused | decided here | `listener.go` |
| Rhythm factors | 1.25, 1, 0.8 | first guess | `motion.js` |
| Overlap | 0.4 | spec | `motion.js` |
| Beat defaults | enter 0.6, exit 0.6, move 0.8, pose 0.3, emote 0.25, look 0.3, show 0.5, hide 0.35, camera 0.8, shake 0.5, wait 1 (never scaled) | spec, phase 1 | `motion.js` |
| Highlight | 0.8 s, pulse 0.08 at 2 cycles, outline 6 px, arrow 40 px, bob 8 px | first guess | `motion.js`, `render.js` |
| Lead point | 0.9 | spec | `motion.js` |
| Spring | damping 0.75, w0 7.6, start scale 0.6 | spec, first guess, phase 1 | `ease.js`, `beats.js` |
| Hold | 0.5 + 0.3 per word; secondary 1.2 | spec | `motion.js` |
| Shrink | step 0.94, floor 0.45, font floor 0.026 of the short side | spec | `text.js` |
| Layout memo | 512 entries | first guess | `text.js` |
| Dead stretch | 3 s | spec | `lint.js` |
| Entry monotony, transition variety | 3, 5 | spec | `lint.js` |
| Captions | 2 lines, width 42, at least 1.0 s, cut at the film end | spec | `captions.js` |
| Caption bottom | 92 %, 9:16 82 % | spec (9:16), first guess | `render.js` |
| Bubble fade | 0.12 s | first guess | `bubble.js` |
| Subtitle and bubble style size | 0.042 | first guess | `assets/storymaker` catalog |
| Lip-sync | 85, 128, 171; variation 0.25; dip 40 | first guess | `lipsync.js` |
| SFX level | -18 LUFS loudest 50 ms, ceiling -1.5 dBFS | measured | `assets/storymaker/README.md` |
| Hits | pop 4.5/3, ding 4.5/3, success 4.5/4, error 4.5/4 | D27 | catalog |
| Typing | spacing 90-140 ms, 1200 ms | D27 | catalog |
| Mood prompts | the eight templates of section 1.1 | first guess | `music.go` |
| Music source | at most 1 GiB; 1 s to 3600 s | first guess | `storymaker_narrate.go` |
| Mix rate, block | 48000, 4800 frames | spec, measured | `mix.go` |
| Target, ceiling, stem guard | -16 LUFS, -1 dBTP, -0.1 dBFS | spec, measured | `mix.go` |
| Bed reference | narration minus 12 LU; -18 LUFS without | spec | `mix.go` |
| Bed gain cap | +24 dB | first guess | `mix.go` |
| Voice match cap | +6 dB | measured need, first guess | `mix.go` |
| Ducking | depth 6 dB, attack 50 ms, release 400 ms, look-ahead 50 ms, threshold -45 dBFS, peak decay 20 ms | measured | `mix.go` |
| Carve | 2200 Hz, Q 1.0, -5 dB, ramp 20 ms | spec, spike | `mix.go` |
| Impact dip | attack 10 ms, hold 80 ms, return 0.3 s | spec, spike | `mix.go` |
| Loop crossfade | 2 s, at most half the bed | spike, decided here | `mix.go` |
| Music fades | in 0.5 s, out 1.5 s | first guess | `mix.go` |
| MP3 timeline tolerance | 10 ms | first guess | `mixinput.go` |
| AAC | 192k, 48 kHz | measured | `ffmpeg.go` |
| Frame gate audio delta | under 25 ms | measured | `gate.go` |
| Gate probe | 120 s, 8 MiB output | `videostudio/probe.go` pattern, first guess | `gate.go` |
| Start hold | 20 s | D29 | `listener.go` |
| Report wait, frame watchdog | 3 min, 10 s | phase 1 | `render.go` |
| Frame deadline | 5000 ms | phase 1 | `render.go` |
| Render cap | 2 GiB per file | code | `render.go:50` |
| Disk floor | 256 MiB plus estimate; stems count twice | code, D S5 | `render.go`, `mix.go` |
| Render timeout rate | 6 fps PNG | measured | `render.go` |
| Workers | `clamp(floor((cpu-2)/3),1,6)` | measured | `render` `ChooseWorkers` |
| Chunk, pre-roll | 8; 0 by default, 2 when on | measured | `render` |
| Memory bound | 60 + 2*C*frameMB, frameMB 0.6, CJK +120, code scenes reserved | measured | `render` |
| Reorder buffer | N*C*2 | spec | `host-pool.js` |
| Font caps | 8 MiB each, 32 MiB total, CJK 17 MiB | spec, measured | `host-app.js`, `glyphs.json` |
| Levels per load | 27000 per block, 200000 per message | D2, first guess | `host-app.js` |
| Glyph file cap | 1 MiB | D12 | `glyphs.go` |
| Preview drift | 120 ms | D14 | `storymaker-audio.js` |
| Decode window | 2 s back, 20 s ahead | first guess | `storymaker-audio.js` |
| Handoff poll | 500 ms, 120 s | first guess | `storymaker-handoff.js` |
| Library free-space margin | 64 MiB | first guess | `service_media.go` |
| High quality | CRF 18 | first guess | `render.go` |
| xAI voice cache | 5 min | D6 | `storymaker_audio_handlers.go` |

## 7 Contract amendment

Apply to `docs/superpowers/specs/2026-11-01-storymaker-contract.md`. Line numbers are today's. Each batch applies its slice in turn and earlier slices shift lines, so a slice matches by the quoted text, not by the number.

**Header.**
- Line 1: "# Storymaker: interface contract".
- Line 5, replace the whole Status sentence: "Phases 1 and 2. Matches the phase 1 and phase 2 plans. The parent document is the design spec `docs/superpowers/specs/2026-11-01-storymaker-design.md`. Where the spec and this document differ for phases 1 and 2, this document is the reference."

**Section 2.**
- Line 29: `Style {palette:string, font:string, captions:bool, rhythm?:"calm"|"standard"|"lively", auto_sfx?:bool}`.
- Line 31 stays.
- Line 59 `[2B]`: "`voice` and `music` may be `null`. `POST /narrate` synthesises; validation still checks only the shape of `voice` and of the provider id."
- Add after line 64: "`music.track` is a positive decimal media id and excludes `music.file`. `style.rhythm` and `style.auto_sfx` are not written when absent."
- Lines 81-93: rename "Phase 2 support".
  - Beats the player plays add say, highlight, sfx.
  - Beats accepted and ignored: chart, show with icon or chart.
  - Remove the music and speaker rows.
  - Narration audio row: "From `/narrate`; until the gate landed a narrated story exports silently; a scene without audio gets `narration_missing`."
- Line 111: `/scenes/<i>/transition`: "`cut` on scene 1; `wipe` when the environment differs from the previous scene's; else `fade`".
- Line 155 `[2B]`: `limit_exceeded` adds say text.
- Line 156 `[2C]`: `missing_field` adds `/music/mood`.
- Line 165 `[2A]`: "see \"Phase 2 support\"".
- Lines 177-182: add rows `internal/storymaker/testdata/narration/cross_language.json` `[2B]` and `internal/storymaker/testdata/stories/speech.json` `[2B]`.
- Issue table, add:

| Code | Severity | Emitted by |
| --- | --- | --- |
| `glyph_missing` | error | Go Validate: a character no face covers, at the text path |
| `line_unread`, `dead_stretch`, `first_frame_empty`, `text_too_small`, `fade_between_worlds`, `cue_late` | warning | Player compile report, storyboard scenes |
| `entry_monotony`, `transition_variety` | minor | Player compile report |

**Section 3.**
- Line 193: add `glyphs` after `rig`.
- Line 194: "`version`: sha256 over the bytes of every file below `ui/img/storymaker/` except `catalog.json`, in sorted path order; text files LF-normalised, WAV files raw."
- Line 195: drop "Current value: ab33e88a6b77", so the line stops going stale.
- Add after line 199:
  - `glyphs {file:"glyphs.json"}`
  - "Fallback faces live only in `glyphs.json`. They are not `fonts` entries and `style.font` cannot name them."
- Line 206 `[2B]`: "`LoadCatalog` reads `glyphs.file` too. It does not read the other files named by entries."
- Line 214: "Every entry except `sfx` has a thumb."
- Line 228, replace with: "`sfx` | `file` (take 1), `takes[{file, dur_ms, onset_ms, peak_ms, impact_ms, l50_lufs, true_peak_dbtp}]`, `impact_ms`, `align` (`peak`, `onset`), `hit?{over_db, dip_db}`, `pattern?{spacing_ms:[min,max], default_duration_ms}`, `source{kind, basis, url}`".
- Line 246: text styles 6, adding `text.speech`, `text.thought`, `text.subtitle`. Add row: `sfx | 7 | sfx.whoosh sfx.pop sfx.click sfx.ding sfx.success sfx.error sfx.typing`.
- Line 250: "The catalog has 86 entries." The 2B slice writes 79, and the 2C slice writes 86.
- Line 260 `[2C]`: "one per entry except `sfx`".
- Add subsection "Glyphs": the shape of section 3 of the frozen interfaces, the family order, and "a codepoint is covered if it is in the face's cmap and inside `unicode_range`".

**Section 3a.** Line 279: add "Speech shapes come from the player's lip-sync track."

**Section 3b.** Line 297: the entry count and list follow the fixture, which gains the styles of `[2B]` and the sfx entries of `[2C]`.

**Section 4.**
- Line 317: "the 28 issue codes". The 2A slice writes 27.
- Lines 342-343 `[2C]`: `Entry` adds `Takes`, `ImpactMS`, `Align`, `Hit`, `Pattern`, `Source`.
- Line 360 `[2B, 2C]`: `LoadCatalog` also checks `glyphs` and the sfx fields.
- Add to the Store block: `Open(ctx, path) (io.ReadSeekCloser, FileInfo, error)`.
- Add the narration, envelope, audio info, glyph and voice API of frozen section 1.1 `[2B]`, and the music API and `Entry` fields `[2C]`.
- Line 414 addition: "`CreateWithVoice` stores a resolved voice."
- Line 417 `[2B]`: "The `DELETE` endpoint also cancels the project's render, narrate and music jobs."
- Line 427: add `Narration StorymakerNarrationConfig`, with its defaults.

**Section 5.**
- Line 473, replace with: `POST /projects` `{title, aspect?, fps?, language?}`, success as before, errors adding 422 `narration_unavailable` with `languages`.
- Line 480, replace with: `POST /projects/{id}/render` with optional body `{burn_in_captions?, srt, vtt, stems, quality}`, errors adding 400 `invalid_request`, 409 `narration_incomplete` (`missing`, `chars`), 409 `music_incomplete` (`key`, `music_issue?`).
- Lines 481-484: "across the render and narrate queues".
- Add rows for `GET /voices`, `GET /projects/{id}/narration`, `POST /projects/{id}/narrate`, `POST /projects/{id}/music`, `GET /projects/{id}/audio/narration/{hash}.{ext}`, `GET /projects/{id}/audio/music/{key}.{ext}` exactly as in frozen section 2.
- Line 490: "The same key, etag and options return the same job; another etag or other options give 409 `idempotency_conflict`."
- Add: "128 narrate and music jobs may wait, in their own queue. `/narrate` and `/music` coalesce per project and kind."
- Line 504: add `max_queued_narrations`, `max_daily_chars`, `max_job_chars`. Add row `music_configured`.
- Lines 513-516: replace with the Job block of frozen section 3.
- Line 519, replace with: "`phase` is omitted while empty. `starting` covers the browser start and a first-time download. `mixing` runs between the report and the first frame."
- Line 523: add `narration_failed`, `narration_unavailable`, `narration_incomplete`, `voice_required`, `budget_exceeded`, `music_failed`, `music_incomplete`.
- Line 528: record fields add stems, srt, vtt, library, warnings, loudness, cue_precision.
- Line 530, replace with: "The library copy is `Videos/Storymaker/<slug>.mp4`, with `-2`, `-3` on collision. It goes through `desktop.Service.WriteStorymakerVideo`. Stems and subtitles live in `exports/<slug>-<job8>/`. A failed extra leaves the MP4 and adds a warning."
- Line 537: "every unfinished job across both queues and the latest five jobs across both queues".

**Section 6.**
- Lines 553-554: part counts follow `scripts/storymaker-bundle-parts.js`; list the new parts. The 2D slice adds `host-pool.js`.
- Line 556 `[2A]`: add `node scripts/build-storymaker-ranges.mjs` with `--check`.
- Line 591, replace with: "`audioInfo` is `{}` or `{narration:{<scene id>:{blocks:[Block]}}}`, the `narration` value of `GET /projects/{id}/narration`. A block's absence means the scene or say text is estimated (words ÷ 2.3 s, at least 1.0 s), and a scene's narration or a say's text then gets `narration_missing`."
- Lines 600-602: add `audio_events`, `captions`, `cue_precision` and `scenes[].cues` to the block.
- Lines 605, 606 and 608 stay.
- Line 607, replace with: "`checks` is empty until phase 3. `unverified` holds one minor `layout_unverified` per code scene. `captions` is always filled, measured or estimated. Compile drops audio events at or past `duration_ms` and cuts captions at it."
- Line 609, replace with: "`audio_events` has one entry per piece; Go owns the gain, so there is no `gain` field. `cue_precision` is the share of cues placed by a span, or null without cues. `scenes[].cues[].at_ms` is scene-local."
- Line 613 `[2A]`: "Narration starts when the transition ends. A beat without a cue starts there too; a cued arrival may start earlier, down to scene time 0."
- Line 614, replace with: "A cue time comes from `cueMap(block, offset)`: inside the piece holding the offset, proportional over that piece's used audio, starting at its `at_ms`; `placed:proportion` over the block duration when pieces are unmeasured, their `chars` do not sum to the text length, or a piece had no detectable speech."
- Line 615: "Automatic scene length is the largest of: narration end plus 0.4 s, last dialogue block end plus 0.4 s, the end of the last beat, the end of the last text hold, and 1.0 s for a scene without speech."
- Line 618, replace with: "Beat defaults, scaled by the rhythm factor unless explicit: enter and exit 0.6 s (they walk); walk distance at `cycle.walk.speed`, at least 0.3 s; move 0.8 s; pose 0.3 s; gesture = catalog duration × `count`; emote 0.25 s; look 0.3 s; show 0.5 s (0 with `entry: "none"`); hide 0.35 s; camera 0.8 s (shake 0.5 s); highlight 0.8 s; sfx 0 (a pattern `default_duration_ms`); say the block length. `wait` is `seconds` (default 1), never scaled."
- Line 619, replace with: the scheduling rule list of frozen section 4.6.
- Line 621: add "Bubbles and burned captions are drawn by the player; bubbles follow the camera."
- Line 658: `render {n, width?, codeFrames, codeErrors, png?}` returns `{bitmap}` or `{blob}`. This text lands only if W landed.

**Section 7.**
- Line 665 `[2C]`: the file list adds `mix.go`, `mixinput.go`, `meter.go`, `gate.go`, `subtitles.go`, `options.go`.
- Line 672 `[2B]`: "...`Origin: null`; the prefix is the credential for reads, and the token of `boot.json` is the credential for `/report`, `/frame`, `/error` and `/start`."
- Line 680 `[2C]`: "`catalog.json` and every file it names, except the sfx WAVs".
- Line 684: "compile report (JSON, 8 MiB at most)".
- Lines 684-686: add `?t=<token>`. Add row `GET <base>/start?t=<token>`.
- Add under the table: "`boot.json` carries `token` on its first GET only. `/report`, `/frame`, `/error` and `/start` require it as `t`; otherwise they answer 403 `bad_token` without any effect."
- Lines 702-704: replace with the boot document of frozen section 3. `workers`, `chunk` and `pre_roll` land only if W landed.
- Line 712: "`fonts` lists the faces of the story's font and every fallback face (`fallback`, `unicode_range`, `weight_range`, `max_bytes`). The page fetches a face only when compile needs it."
- Line 716 `[2B]`: "The page loads boot.json, story.json, the catalog, `glyphs.json` and the bundles, then compiles, and fetches the fonts compile needs during `prepare`."
- Line 717, replace with: "2. `POST /report?t=` validates and answers `{"start":false,"pending":true}` at once, or `{"start":false, "error":"<code>", "message":"..."}`. Go then mixes in the background. 2a. The page long-polls `GET /start?t=`; each call holds at most 20 s and answers `{"pending":true}`, `{"start":true}` or `{"error", "message"}`, and 404 before an accepted report. The 10 s no-frame watchdog starts with FFmpeg, after the mix."
- Line 723: add `&t=<token>`.
- Line 743 `[2B]`: refusal codes add `bad_token` (403).
- Line 747 `[2B]`: "`POST <base>/error?t=<token>`"; a wrong token answers 403 and does not end the job.
- Line 751: "`report_too_large` (413, over 8 MiB), `report_repeated`, `bad_token` (403)."
- Line 760: unchanged.
- Line 800, replace with: "Limit: the prefix is not a secret against scene code, but a scene cannot obtain the token, because the host takes it before any worker exists. Go validates every report field it uses against its own plan."
- Line 805, replace with: "The audio is the mix WAV as input 1 with `-format_whitelist wav`. A film with no audio events and no music keeps `anullsrc` with `-t`. AAC 192k."
- Line 806: remove `-shortest`.
- Add "### Frame gate": command and rule from D30, with the whitelists and limits of frozen section 1.2.
- Add "### Workers": from frozen section 4.8 step 10, if W landed.

**Section 8.**
- Line 823: `compiled`: "`report` (section 6), cleaned by the parent; `audio_events` at most 40000, `captions` at most 8000".
- Line 832: `load`: `audio_info?` per section 6; `catalog` may carry `glyph_data`; fonts add `weight_range?`, `unicode_range?`.
- Lines 848-859, add caps: audio scenes 400, blocks per scene 61, pieces per block 800, levels per block 27000, levels per message 200000, glyph faces 16, glyph range values 60000, fallback font at most its `max_bytes` and at most 18 MiB.
- Line 853: "fonts: 16, each at most 8 MiB unless a named glyph face allows more, 32 MiB in total, checked on `load.fonts` and on `font:` values of `assets`, with the faces and the byte count of the last accepted load".

**Section 9.**
- Line 881: "the style `desktop-app-storymaker.css` and sixteen scripts in order: `storymaker-core.js`, `storymaker-model.js`, `storymaker-api.js`, `storymaker-assets.js`, `storymaker-player.js`, `storymaker-projects.js`, `storymaker-strip.js`, `storymaker-inspector.js`, `storymaker-code.js`, `storymaker-export.js`, `storymaker-audio.js`, `storymaker-voice.js`, `storymaker-music.js`, `storymaker-dialog.js`, `storymaker-handoff.js`, `storymaker.js`."
- Line 895: add `storymaker.narration.max_daily_chars`, `storymaker.narration.max_job_chars`.
- Line 906: remove export options, quality, captions, stems and the Video Studio handoff from the "Not in phase 1" list and add a "Phase 2 screens" paragraph: voice panel, narration status, music panel, scrubber waveform, export dialog, finished-job handoff.

## 8 Spec corrections

Text for a section "Changes from the phase 2 plan" at the end of `docs/superpowers/specs/2026-11-01-storymaker-design.md`.

**Narration**
- Line 135, 499: the envelope is `audio/narration/<hash>.env.json`, a 10 ms RMS series free of the frame rate. A per-fps file per piece drifts when pieces are joined. Go builds per-frame block levels and spans at answer time.
- Line 84: envelopes are not transferred to workers. Levels travel inline in `audio_info`.
- Line 97: voice resolution is in `voices.go`, not `narration.go`.
- Line 495: one sentence is one piece, not sentences grouped up to the chunk limit. An edit then re-synthesises one sentence and sentence starts are exact. Fragments under 20 width or of 3 words or fewer merge into a neighbour.
- Line 496: sanoTTS is already serialised (`internal/tools/sanotts.go:18-27`). The narrate queue adds one call at a time for every provider.
- Line 497: the key is computed per piece, with provider and language trimmed and lower-cased.
- Line 498: decoding uses a new `audio_decode.go`, because `voice.DecodeWAVPCM16Source` accepts only mono s16. ffprobe is not a fallback. Music durations come from the WAV header or go-mp3's length, with no full decode.
- Line 490: no code enforces a 400-character limit. It stays our own conservative size.
- Line 492: per-language notes hold facts only. Listening notes wait for the user's tests, because nothing here can judge speech. The `not tested` mark ships, on the provider and language, shown for every provider in phase 2.
- Line 494: the ElevenLabs body also needs real JSON (`tts.go:487-488`), not only the path escape.
- Lines 494, 506: `Validate` checks the shape only, because the package does not import `internal/tools`. The check against the list runs in `POST /narrate`, which answers 409 `voice_unknown` with at most 5 alternatives. The agent's `kind=voice` discovery is phase 3.
- Lines 484, 837: the OpenAI list is the 13 voices named in the plan; `gpt-4o-mini-tts` is hard-coded (`tts_cloud.go:21`) and shuts down 2027-01-06.
- Line 483: `FetchXAIVoices` is a live client method, cached 5 minutes.
- Line 487: google is offered only when active, and `POST /narrate` refuses it otherwise. Providers are listed active first, then local, then cloud, so resolution never picks a paid provider ahead of a free one.
- Line 488: Speech Lab uses the `/ready` snapshot; no per-call voice exists.
- Line 501: the vowel-weight rule is dropped. It erred 581-730 ms mean against 341 ms for proportional (n=30). `cueMap` places a marker at a sentence start on that sentence's onset, and proportionally over the sentence's speech inside it. Each block has `placed: span` or `proportion`, and the fallback no longer depends on the script.
- Line 502: narration clips of different voices are matched before the stem is measured, capped at +6 dB, so a film with two speakers lands near target without a meter per clip. Clips reach the mix rate through FFmpeg in job staging.
- Line 504: the `narration_incomplete` gate lands in batch 2C with a seeded-audio test helper. Until then a narrated story exports silently.
- Line 505: `POST /projects` takes an optional `language`. Without it the project has `voice:null`, as in phase 1, and the voice panel writes one. `POST /narrate` answers 409 `voice_required` without a voice.
- Line 503: a silent scene silences its narration only; say text stays spoken. A say without text draws the active caption lines of the scene narration until that narration ends, has no block and does not move the schedule. In a silent scene it draws nothing and gets a warning. A say whose actor is off stage still plays its audio, with no bubble and no lip-sync. Say text has the 1500-character limit of narration.
- Audio is untrusted on disk: `env.json` and `meta.json` are checked when read, an invalid one counts as missing, and a piece that no longer decodes is synthesised again.
- Daily and job caps reserve characters before the call, because a provider that answered has billed.

**Motion**
- Lines 189, 345: `exit` always walks today, so 0.35 s applies to `hide`.
- Line 229, 364, 850: the default transition is `wipe` when the environment changes, else `fade`, for new scenes only. A stored fade between worlds still gets the lint.
- Line 342: `emote` has no ease. The expression stays a step until phase 4.
- Line 346: overlap measures from the anchor, the longest beat of a `with:"prev"` group, not strictly the previous beat. A short beat that runs with a long one must not start the next target's beat early.
- Lines 348, 817: the chart lead lands with the chart beat in phase 4. Phase 2 leaves `chart` out of the schedule.
- Line 353: for holds a run of CJK characters counts 1 word per 2 characters. The same count feeds the estimate.
- Line 355: rhythm factors are first guesses. Code-scene API defaults are not scaled.
- Line 357: shrink to fit runs in `text.layout`, because compile has no canvas and runs before fonts. Line 409: the phase 2 `text_too_small` reads the style size only.
- Determinism: every seed comes from strings, since no project seed exists among the compile inputs. `Intl.Segmenter`, `\p{Script}` and `normalize` are not used in compile or the wrap, because they vary with ICU. Generated range tables from a committed Unicode 16.0.0 data file replace them.

**Glyphs**
- Lines 119, 326: Go does not read WOFF2. `glyphs.go` reads a build-generated `glyphs.json` that carries each face's coverage.
- Line 327: Geist has only 6 Greek letters. Inter Greek and Greek Extended come first with their `unicodeRange`, or Chrome draws ω as Ω. Emoji, Thai and Hebrew are `glyph_missing`. Default-ignorables and JS whitespace are exempt, and the NFC form is checked. A silent scene's narration is not checked, because it is never drawn.
- Line 330: no subset. `NotoSansCJKsc.otf` gets a named 17 MiB exception, because a subset breaks the per-character check. Measured: 80-120 MiB per worker, 292-330 ms load.
- Fallback faces load on need. Render mode fetches fonts lazily.

**Sound**
- Lines 33, 312, 522, 817: the SFX set is not a CC0 download. 25 WAVs are rendered once with earshot: forge sounds are code-made, and whoosh, click, ding and success derive from CC0 Freesound recordings named in `assets/storymaker/README.md`. The output-rights reading stays open.
- Line 99: `mix.go` lives in `render/` and is pure Go, not an FFmpeg audio graph.
- Line 514, 589: looping uses an equal-power crossfade in `mix.go`, not `acrossfade`.
- Lines 516-519: ducking uses a feed-forward stage in `mix.go`, not `sidechaincompress`. Both filters exist on both FFmpeg versions, so the reasons are exact stems, a 1000-clip graph, look-ahead and unit tests.
- Lines 521, 605, 836: the meter is one FFmpeg process reading a float pipe with `ebur128` and `loudnorm` in one graph, I from ebur128 (loudnorm under-reads an 8 s film by 1.6 LU), TP from loudnorm. It is not a two-pass `loudnorm`. The gain is `min(-16-I, -1-TP, -0.1-max stem peak)`.
- Line 522: take rotation is seeded from scene id, beat index and sound id, and the last take spans the film. Hit levels: pop 4.5/3, ding 4.5/3, success 4.5/4, error 4.5/4. Auto-SFX also skips a pop whose landing already has an sfx beat running `with:"prev"` on it.
- Line 523: procedural effects are not built. A dev-time render with earshot gives the same repeatability. 25 WAVs ship, copied by the build.
- Line 524, 854: no limiter. Normal shapes land at -16.0 to -17.6 LUFS. An sfx-only film ends near -22 LUFS, accepted.
- Line 525: the preview bed plays through `<audio>`, and narration decodes in a sliding window, because a 15-minute decoded bed is 345 MB. No carve in preview.
- Line 589: generated music length comes from the app's last compile until phase 3's check job. A stale length is absorbed by the loop or the trim. A local profile below 600 s gets the profile's limit as its duration, and the loop covers the rest.
- Line 511 stays: modes are `generated` and `file`, with `file` XOR `track`.
- Music jobs share the narrate queue. `POST /music` also copies file and track sources to a hash name. Decided here: wav and mp3 sources only, at most 1 GiB, 1 s to 3600 s. A moved source reports `music_issue: source_missing` and does not stop narration.
- Generated music starts only from an explicit click, because a key changes at each pause in typing.

**Rendering and export**
- Line 83: the 10 s no-frame watchdog starts with FFmpeg, after the mix, not when the report is accepted.
- Line 544: PNG is encoded in the worker. Worker encoding exists only for PNG, so other transports stay serial. Pre-roll is off by default and stays switchable, because every frame was byte-identical with and without it. `N = clamp(floor((cpus-2)/3),1,6)`, bounded by memory, and Go passes it in `boot.json`.
- Line 553: the frame gate also checks audio against video within 25 ms, because the packet count does not notice a wrong audio length. `-shortest` is removed.
- Lines 555, 773: new inputs use narrower format lists than `videostudio/probe.go:16-17`: `wav,mp3` for decoding, `f32le` for the meter, `wav` for the mix, `mov` for the gate.
- Line 556: the restart stays an answer to `/frame`.
- Line 560: the camera-import reason is wrong for a nested path (`video-studio-projects.js:433` accepts top-level `Videos/<file>` only). The library copy is for discoverability, and the handoff imports from `exports/`. Writing to `Videos` needs new code, `WriteStorymakerVideo`.
- Line 562: stems and subtitles go into `exports/<slug>-<job8>/`. There is no `handoff` render option: handoff is a client action on any finished job with stems, so a closed window cannot lose it.
- Line 563: the per-file cap in phase 2 is 2 GiB (`render.go:50`).
- Line 577: `audio_events` has one entry per piece, adds `scene_id`, `block_id`, `piece`, `take`, and has no `gain`. Go computes every gain.
- Line 578: `captions[]` adds `scene_id` and `block_id`.
- Line 582, 768: a one-shot token guards `/report`, `/frame`, `/error`, `/start`, and Go validates refs, counts, times and caption text. Go also requires each measured block's pieces in order, so an empty `audio_events` cannot defeat the gate. `maxReportBytes` is 8 MiB.
- Line 588: mixing no longer runs inside the report request, because three timers would cut it. The page long-polls `/start`.
- Line 771: the read-only flag is re-checked before every paid call, and a revocable desktop run also cancels a call in flight. Addition: disk is estimated before any copy or decode, because the 256 MiB floor is too low for a 900 s film with stems. Every narration piece has a decode limit too, not only music.
- Line 783: `voice_required` and `music_incomplete` join the stable code list.
- `alimiter` is in use in Video Studio's mux at `videostudio/render.go:394`, unrelated to this mix.

**Handoff and app**
- Lines 594, 601: the handoff and the track builder live in `storymaker-handoff.js`.
- Line 605: stems sum to the mix exactly before encoding only. After AAC the residual is about 18 dB below the mix at 64k and 23 dB at 192k. Audio is AAC 192k.
- Line 607: the launch parameter also touches `menus-and-routing.js`, because the existing window is always reused.
- Line 737: the export dialog is a native `<dialog>` in its own module.

**After the dry run and the review**
- A dot is no sentence end before a lower-case word or a digit, or after 1 to 3 digits, a single letter or a listed abbreviation, so "19." and "z. B." stay inside a sentence. German closing quotes count as closers.
- A code scene without `api.duration` covers its narration. Narration or a say that runs past an explicit scene duration warns.
- Generated music calls a provider only from the Generate click, pinned by a key in the job. A source never chosen is not a missing source.
- The preview follows the export's bed and SFX levels from stored measurements. It is close, not equal, and the panel says so.
- SRT writes `&` literally; only VTT uses entities.
- The export panel offers the newest finished render after a reopen, which keeps the promise that a closed window cannot lose the handoff.
- Both paid panels have a stop control. A scene can be set silent in the inspector. Deleting a project waits up to 15 s for its jobs.
- `interfaces.md` section 10 in the plan folder lists the changed names and the items decided against.

**Phase 1 projects after the update**
- `style.captions` defaults to true (`contract line 61`), so every narrated phase 1 story starts drawing captions in preview and export from 2B. The voice panel has a toggle.
- A say without text whose `who` is not the scene speaker moves from a minor issue to a blocking error.
- A `mood` outside the presets moves from a minor issue to a blocking error in 2C.

**To verify, closed:** Clip semantics, audio rates (local), OpenAI list, Ceiling and stems, Default fade, Glyph fallback, CJK in workers, Parallel memory, Procedural effects (not built), Speech Lab per-call voice (none used), Lyria length (the loop and trim cover any length), Mix limiter (none), Cue placement in speech (replaced by `cueMap`), Music shape checks (spec line 855, not adopted in phase 2).

**Still open for the user's ears or hands:** voice notes and rhythm factors; sentence pause and trim margins; taste of the SFX set, especially `error` and `success`; whether per-sentence synthesis keeps good prosody, on cloud voices above all; the output-rights reading of earshot renders; whether emote and gesture beats with a cue should lead as well (spec line 852); the audible sentence gap, about 490 ms against the 350 ms the rule names; the level of non-hit SFX, which follows the music slider; the mood prompt texts. aurago-test was unreachable on 2026-11-04, so its acceptance stays open.

**Still open as decisions:**
- Whether code-scene workers should get language faces only when the scene registers `onFrame`. It would save about 100 MiB per worker, but D13 says they get them, so phase 2 keeps D13 and counts the memory in `ChooseWorkers`.
- `audio/narration` grows with every edit and voice change, and nothing prunes it. A prune needs a rule for queued renders, whose `Plan` names hashes fixed at queue time. Deleting a project frees it.

## 9 Review ledger

Each critical and important finding, by review lens. "Outline row" means the fix is to a task row or matrix of the execution outline, which another agent revises. No frozen text changes for it, so it is rejected for this document, and the evidence named is what I opened.

**Lens code**
- critical, deferred `<-mixDone` blocks and misorders: applied. `mixWG`, `mixCtx`, a defer after `render.go:401-408`, and a check under `run.mu` before FFmpeg.
- critical, S2 bounds refuse honest reports at the film end: applied. Film-end rule in section 3 and `settle(all, filmMs)`. Go stays strict.
- important, render package and `storymakertest` cannot reach audio: applied. `OpenAudio`, `AudioPath`, `WriteMusic`, `WriteMusicMeta`, `MusicInfo`.
- important, `max_bytes` check on `assets` cannot run in `checkMessage(msg)`: applied. `checkMessage(msg, loaded)`, confirmed at `host-app.js:24`, `:62-71`, `:248`.
- important, layout memo key omits `maxWidth`: applied. Key extended, confirmed at `render.js:171` and `beats.js:162`.
- important, `MaxEvents` ignores rhythm: applied, with constant `maxRhythmFactor` 1.25 instead of a Go rhythm table.
- important, music duration clamp ignores the ACE-Step profile: applied. Confirmed at `music_generation.go:180` and `acestep/types.go:66-71`.
- important, M5 breaks `pt/02`: rejected for this document, outline row. Evidence holds at `02-stage-parts-text.mjs:100`, `:104`.
- important, A17 and A21 miss `render_test.go`: rejected for this document, outline row. Evidence holds at `render_test.go:265` and its ten `Prepare` calls.
- important, N18 line lists: rejected for this document, outline row. Evidence holds at `09-host-render.mjs:35`, `:42`, `:48`.
- important, S1 and S4 miss `pt/04` pins: rejected for this document, outline row. Evidence holds at `04-compile.mjs:24`, `:150`.
- important, G1, A2, A3 miss the catalog guard: rejected for this document, outline row. Evidence holds at `ui/storymaker_catalog_test.go:767`, `:769`, `:777`.
- important, audition has no route: applied. Plays a stored piece, never synthesises.

**Lens spec**
- important, say without text has no schedule: applied in section 4.6 steps 4, 8 and 9.
- important, contract lines 599-609 replacement deletes normative text: applied. Only lines 600-602, 607 and 609 change.
- important, google offered only when active, and provider order: applied in section 1.4.
- important, voice list validation has no interface: applied. 409 `voice_unknown` in `POST /narrate`, recorded in section 8.
- important, step 4 ignores `wait.seconds` and scales it: applied. Confirmed at `beats.js:315`, `validate.go:944-946`.
- important, `not tested` mark cannot be carried: applied. `VoiceProvider.TestedLanguages`.
- important, audition undefined: applied, as above.
- important, silent scenes not exempted from glyphs and say: applied in sections 1.1 and 4.6.
- important, CC0 wording not corrected: applied in section 8.
- important, contract lines missing from section 7: applied, all named lines added with their batch.
- important, spec differences missing from section 8: applied. Lines 555 and 773 are now recorded as narrower lists, because the gate gained whitelists.

**Lens wire**
- critical, captions and events past the film: applied, as above.
- critical, `mixDone` faults: applied, as above.
- important, `errorFromResponse` drops extra fields: applied, `data` field. Confirmed at `storymaker-core.js:257-282`.
- important, code scene never gets the font chain: applied. `worker-scene.js:81`, `render.js:17-23`, `:32-35`.
- important, `MixClip` has no sfx level input: applied, `LevelOffsetDB`.
- important, sfx `start_ms` is scene-local: applied in section 3.
- important, ACE-Step profile: applied, as above.
- important, music storage has no API or restart rule: applied, with one change: production streams through `Publisher.WriteStream` as D31 says, and `WriteMusic` takes bytes, because `Store` has no stream write.
- important, moved music source stops the answer: applied. `ErrMusicSourceMissing`, `music_issue`.
- important, player does not recompile on a new answer: applied in `bindAudio`. Confirmed at `storymaker.js:218`.
- important, voice panel lacks a language control: applied, language select.
- important, `MaxEvents`: applied, as above.
- important, worker memory by code scene: applied in part. `codeScenes` and the zh or ja rule are in `ChooseWorkers`. Sending language faces only with `onFrame` is rejected, because D13 binds; recorded as an open decision.
- important, say text has no limit: applied, `limit_exceeded`.
- important, bed gain undefined when silent or short: applied. Drop with `music_silent`, cap +24 dB, crossfade at most half, source at least 1 s.

**Lens security**
- critical, captions and events past the film: applied, as above.
- critical, deferred start: applied, as above.
- critical, music panel starts paid generation: applied. Explicit click only for mode generated. Confirmed at `music_generation.go:187-188`; I did not open `storymaker.js:717-725` beyond the code map.
- important, read-only re-check does not stop a call in flight: applied. `beginDesktopRun` and all three states, confirmed at `desktop_operation.go:108-140`, `server.go:657-659`. The failure code follows the spec-lens minor: `narration_unavailable` or `music_failed`, not `render_failed`.
- important, budget bypass by failure: applied. Reservation before the call, job cap on sent characters, unreadable usage file blocks, pending file for music.
- important, decode bomb and header trust: applied. Limits in `audio_decode.go`. The fixed window also changed, because it would otherwise fail the envelope length check at 22050 Hz.
- important, decode cache written before the disk check: applied. `planMixInput`, new `EstimateMixBytes` signature, limits on every piece.
- important, stems and cache can fill the disk: applied in part. `WorkspaceFree`, narrate floor and stems counted twice are in. The 512 MiB prune is rejected, because it could delete hashes a queued render's `Plan` names; recorded as an open decision.
- important, unbounded `POST /music` source: applied, with `ProbeAudio` instead of `videostudio.Probe`, so a music job needs no FFmpeg and the editor keeps working without it.
- important, `env.json` and `meta.json` trusted: applied. `ParseEnvelope`, meta rules, `PieceFiles` filter, `maxTotalLevels`.
- important, gate can be defeated by the report: applied with two corrections. A dialogue block may have no event in any scene, because a far delay can pass the film end outside the last scene. `HasAudio` also counts sfx events, because an sfx-only film exists only in the report.
- important, `recoverEncoder` drops the audio: applied, `run.audioPath`. Confirmed at `render.go:630-635`, `:650`.
- important, provider case: applied, `NormalizeProvider`.
- important, say text limit: applied, as above.
- important, google: applied, as above.
- important, audition: applied, as above.

**Lens tasks**
- critical, WAVs only in a session temp folder: rejected for this document, outline row. Evidence holds: 25 WAVs under `spikes/sfx/set`.
- critical, A3 commits entries that A4's rule accepts: rejected for this document, outline row. Evidence holds at `catalog.go:289`, `:334`, `:522`.
- important, A2 guards: rejected for this document, outline row. Evidence holds at `ui/storymaker_catalog_test.go:769`.
- important, G1 and S5 file-count pin: rejected for this document, outline row. Evidence holds at `catalog.test.mjs:24`.
- important, S5 and A6 mini catalog pins: rejected for this document, outline row. Evidence holds at `00-fixtures.mjs:29`.
- important, A17 fixture: rejected for this document, outline row. Evidence holds at `render_test.go:265`.
- important, A21 test file: rejected for this document, outline row. Evidence holds as above.
- important, A24 misses the API test: rejected for this document, outline row. Evidence holds at `storymaker_api_test.go:528`.
- important, M5 pin: rejected for this document, outline row, as above.
- important, M11 misses pins: rejected for this document, outline row. Evidence holds at `08-host-app.mjs:64` and `storymaker_player_gate_browser_test.go:278-282`; I did not recompute the 341-frame hand calculation.
- important, S2, S3, S4 reds in `pt/04`: rejected for this document, outline row. Line 24 confirmed; lines 57-59 not reopened.
- important, S7 and S9 font keys for captions and bubbles: applied in section 4.3 "Needs", confirmed at `compile.js:107-113`. The test pins stay with the outline.
- important, N21 and S4 film end: applied, as above.
- important, E3d before E3e loader entry: rejected for this document, outline row. Evidence holds at `ui/desktop_storymaker_guards_test.go:216`.
- important, N10, N12, N15, A14 use later symbols: rejected for this document, outline row, except that section 1.4 now states `storymakerMusicKey` returns `""` in 2B.
- important, E1e and E2b features undefined: applied. Audition, `noisemakerTracks`, `AuraDesktopFileDialogs`. Confirmed at `server_routes.go:590`, `noisemaker_handlers.go:662-670`, `file-dialog-runtime.js:569`.
- important, E3f and I2 bundle rebuild: rejected for this document, outline row. Evidence holds at `scripts/build-ui-bundles.js:25`, `:64`. Section 5 records which shell files are bundle parts.

## 10 Decisions after the freeze

The sections above were frozen before the tasks were written. Writing them in parallel, a dry run and an independent review then changed some of it. Where this section, `reconcile.md` or a task in a part differs from sections 1 to 8, those win. `reconcile.md` holds the numbered decisions R1 to R42 and the adopted minor findings with their reasons.

**Go**
- `ranges_gen.go`: `ucdVersion`, `type ucdRange struct{lo, hi rune}`, `ucdEastAsianWide`, `ucdCJKWord`, `ucdDefaultIgnorable`, `inUCDTable(table []ucdRange, r rune) bool`. `IsDefaultIgnorable` is `inUCDTable(ucdDefaultIgnorable, r)`.
- Rule 1 of `SplitPieces` (line 118): a bare `.` directly followed by whitespace is no sentence end when the next non-whitespace rune is lower case or an ASCII digit, or when the run of letters, marks and digits directly before it is 1 to 3 ASCII digits, a single letter or, lower-cased, one of dr, prof, nr, bzw, ca, usw, etc, vs, mr, mrs, st, bspw, ggf, evtl, inkl, mio, mrd, vgl. The closers also hold U+201C, U+2018 and U+00AB, and a hard terminator followed by a quote that opens the next sentence ends the sentence before that quote.
- `MusicKeyResolver`: `""` also means mode file with no source chosen yet. A source that was set and no longer exists returns `ErrMusicSourceMissing`.
- `MusicRef` and `MusicMeta` gain `LoudnessI *float64 \`json:"loudness_i,omitempty"\``, measured by `render.MeterStored`, one best-effort FFmpeg run in the music job.
- `storymakerMusicPayload` gains `GenerateKey string`. `POST /music` takes `{duration_ms, generate?}`. Generated music calls a provider only when the job's `GenerateKey` equals the key at run time, and a route call without `generate:true` answers `{job:null}` for mode generated with no stored file. The app's Generate button is the only caller that sets it. A pending result is copied with no provider call.
- `storymakerNarrateStopped` and `storymakerMusicStopped` return one fixed sentence that names both causes, because `replaceConfigSnapshot` revokes runs before it publishes the snapshot.
- `POST /narrate` also answers 409 `voice_unknown` for a Phonon voice that is listed but not installed. The 409 extras are `missing` (integer of code points), `limit` (the cap that was hit) and `alternatives` (voice id strings, the block's language first).
- `checkAudioGates(answer, musicKey)` reads nothing itself. `Prepare` passes the answer and key of its one resolve.
- `ProbeOutput` and `FFmpeg.Wait` take their timeout as an argument. The post-encode limit is the larger of 120 s and the file size at 10 MiB/s.
- SRT writes `&` literally and replaces `<` and `>`; VTT keeps its entities.
- Go removes the report token and the URL prefix from every message that comes from the page before it reaches the job record.
- `startAfterReport` recovers a panic, logs it and fails the job.
- The delete route cancels the project's jobs in both queues and waits for them with a 15 s bound before it removes the folder (`Manager.WaitProject`).
- `FreeMemoryMB` is capped by `render.BoundMemoryToCgroup`, which reads the top level of `/sys/fs/cgroup`.
- `storymakerSynth` refuses a result over `storymakerMaxPieceBytes` before `DecodeAudio`.

**Player**
- A code scene without `api.duration` and with narration covers its narration: the larger of its last motion plus 0.5 s and the automatic scene length. A silent one keeps its last motion plus 0.5 s. With `api.duration` it keeps that length, and a longer narration is a warning.
- A say block that runs past an explicit scene duration warns, in the same one warning per scene as narration.
- `holdSeconds(text, {secondary, factor})`. The floor check of 4.6 step 7 is `cued && floor > 0 && floor > t + 1e-6`. `e.speech` holds `{block: preparedBlock}`.
- `g.text` in a code scene sets `ctx.direction` per call.

**App**
- New names on `inst`: `editStory(fn, options)` (the one story-level edit), `reloadPlayer`, `showPanel(name)`, `startExport(options)`, `handoffWish` (a session-only Set, created in `createInstance`), `audioError` (the last failed `GET /narration`). `panel.marker` from the voice panel. `S.handoff.status()` reads `GET /api/desktop/video-studio/status`.
- Audition and Generate use the stored voice. Both wait until a changed choice is applied (`storymaker.voice.apply_first`). A voice that is not installed is not offered, unless it is already the stored one.
- Refusal of `narrationUrl` and `musicUrl` is the empty string.
- `visibleIssues` no longer hides `narration_missing`.
- The scene inspector has a Silent switch (`data-stm="scene-silent"`), the way out when a story must export without a voice.
- The export start view offers the newest finished render behind `data-stm="export-last"`, read once from `inst.api.listJobs`. A replayed finished job stays out of the panel unless its id is in `inst.handoffWish`.
- `S.handoff.canOffer` is false with a `stem_failed` warning. The dialog disables Open in Video Studio with a reason when the status part of `canOffer` fails. A failed handoff keeps its project id in the panel and reuses it on retry.
- Stop controls: `narration-cancel` and `music-cancel`. Read errors: `narration-read-retry`, `music-read-error`, `music-read-retry`.
- The preview mirrors `computeMixLevels` for the bed and the SFX. It is close to the export, not equal to it, and the panel says so.
- `.stm-tabs-small` wraps, because four side tabs do not fit in one row.

**Open items decided against for phase 2**
- Take rotation when auto-SFX or typing events mix with sfx beats. A full MP3 scan before the duration limit in `POST /music`. A digest for the three Unicode files. Sweeping temp files of a crashed publish. The handoff reads canvas and frame rate from the live story, not the exported one. A memory bound for the MP3 offset check. A retry past the budget checks when a pending file exists. The MP3 offset in the preview. Scene code cannot read its narration length.
- A real sentence end right after 1 to 3 digits, a single letter or a listed abbreviation stays in one piece, for example "Es waren 20. Dann kamen 30.".
- Sentence pause and the level of non-hit SFX wait for listening: the audible gap is about 490 ms against the 350 ms the rule names, and non-hit SFX follow the music slider.
