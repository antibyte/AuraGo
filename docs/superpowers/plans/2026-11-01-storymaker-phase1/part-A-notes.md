# Part A notes: Go core (`internal/storymaker` + config)

Worktree (uncommitted, nothing committed): `C:/Users/Andi/AppData/Local/Temp/claude/C--Users-Andi-Documents-repo-AuraGo/f2c97a7f-addd-4aa7-bd49-85d6c9b3f2a1/scratchpad/sm-wt-a`
Plan part: `part-A.md` (8 tasks A1 to A8, 7180 lines; generated from the verified files, 25 labelled code blocks checked byte for byte).

## Status and evidence

- `go test ./internal/storymaker/... ./internal/config/... -count=1` green: storymaker 58 tests + 170 subtests in about 1.2 s, config
  21 s (whole package, includes the three new `Storymaker` tests). `go vet` and `gofmt -l` clean. `go build ./...` of the whole repo is green.
- Also green with the new template section: `go test ./cmd/config-merger ./internal/setup`, and
  `go test ./ui -run TestConfigDerivedValidationRulesCoverTemplate` (checks every template scalar against the derived rules).
- `go test ./internal/audit` has one failure, `TestUpdateConfigFallbackOnlyTrustedTransactionBackups` (bash `chmod 770` semantics on an
  NTFS temp dir, "WRITABLE-ROOT-ACCEPTED"). It tests the update recovery script, not config, and does not touch anything of part A; I did
  not re-run it on the base commit (no second worktree), so treat it as unrelated but unproven.
- Plan replay: `sm-tmp-a/sim.py` copies the tasks A2 to A7 one by one into a throw-away package; each task fails first (undefined symbols),
  passes after its implementation, and keeps `go vet` and the whole package green. The generated golden bundle equals the committed one.
- The golden `testdata/scenes/s3.bundle.js` (1365 bytes, sha256 `5a7e31916dcaa48c3aa30145cb6ea9496a489cc3e023d64999ef38bef83918d4`) is
  reproduced byte for byte by Node esbuild 0.28.2 (`transformSync` with `format:'iife', globalName:'__smSceneModule', target:'es2020',
  legalComments:'none', loader:'js', sourcefile:'scenes/s3.js'` plus the appended `"\nself.__smScene=__smSceneModule.default;\n"`).
  The `sourcefile` matters: esbuild names the exports object after it (`s3_exports`), so C must pass `scenes/<id>.js`.
- GitNexus: `impact "Load"` (internal/config/config.go:377) on the main checkout index: 73 impacted symbols, MEDIUM; the edit is additive.
  `detect_changes` belongs to the commit step of the plan (A1, A8), not run here because nothing is committed.
- Measured (this box): bundling `s3.js` 1.6 ms; a 60 KiB code scene 52 ms (of which the pre-check, two tokenizer passes, is 29 ms);
  decode + defaults + validate of `firewall.json` 0.23 ms.

## Files (all new unless marked)

`internal/storymaker/`: `model.go` (types, polymorphic `Position`/`Duration`, `CanonicalJSON`, limits, closed name sets, phase 1 matrix),
`decode.go` (`DecodeStory`, `DecodeError`, unknown-field walk + hint table), `validate.go` (`Validate`), `issues.go` (`CapIssues`,
`HasErrors`, alternatives helpers), `defaults.go` (`ApplyDefaults`, show-beat ids, `jp`), `catalog.go`, `codebundle.go`, `project.go`,
`memstore.go` (exported `MemStore`, test helper for B and E), `slug.go`, `AGENTS.md`, tests, `testdata/{stories,scenes,catalog-mini}`.
`internal/config/storymaker.go` + test; modified `config_types.go`, `config.go`, `config_template.yaml`.
decode.go, issues.go and memstore.go are not in the ownership table of contract section 10; they are plain splits of A's own area.
Fixtures `minimal.json`, `firewall.json`, `s3.js` and `catalog-mini/catalog.json` are byte-identical to the contract text (checked by script).

## Public API (final)

```go
const Schema = "storymaker/1"; CatalogSchema = "storymaker-catalog/1"; MustNotExist = "-"; MaxModelIssues = 8; MaxHTTPIssues = 200
const DefaultEnvironment = "env.plain-stage"; DefaultAspect = "16:9"; DefaultFPS = 30; DefaultPalette = "sunny"; DefaultFont = "default"
const SeverityError, SeverityWarning, SeverityMinor        // "error" "warning" "minor"
const CodeInvalidJSON ... CodeLayoutUnverified               // the 19 codes of contract section 2 (Code<CamelCase>)
var ErrConflict, ErrInvalidID, ErrInvalidArgument, ErrTooLarge
var PoseNames, GestureNames, ExpressionNames, MouthNames, TransitionNames, EaseNames, BeatKinds

type Story, Format, Style, Voice, Music, CastMember, Scene, Camera, Actor, Prop, Beat, ChartData   // contract section 2, snake_case tags
type Position struct{ Spot string; X, Y float64; Point bool; Invalid json.RawMessage }   // JSON: "desk" | {"x":..,"y":..}
func SpotAt(name string) *Position; func PointAt(x, y float64) *Position
type Duration struct{ Auto bool; Seconds float64; Invalid json.RawMessage }               // JSON: "auto" | number
func AutoDuration() *Duration; func SecondsDuration(s float64) *Duration
type Issue struct{ Severity, SceneID, Path, Code, Message string; AtSeconds *float64; Fix string }
type DefaultApplied struct{ Path string; Value any }
type Limits struct{ MaxScenes, MaxItemsPerScene, MaxBeatsPerScene, MaxNarrationChars, MaxStoryBytes, MaxCodeBytes int; MaxDurationSeconds float64 }
func DefaultLimits() Limits
type DecodeError struct{ Code, Message string; Issues []Issue }
func DecodeStory(data []byte, lim Limits) (Story, []Issue, error)   // error is always *DecodeError, issues >= 1 on failure, nil on success
func Validate(s *Story, cat *Catalog, lim Limits) []Issue           // canonicalises names in place; cat == nil skips catalog checks
func ApplyDefaults(s *Story) []DefaultApplied
func CanonicalJSON(s Story) ([]byte, error)                         // 2-space indent, struct key order, no HTML escaping, trailing "\n"
func CapIssues(issues []Issue, n int) []Issue; func HasErrors(issues []Issue) bool
func CodeSceneIssues(s *Story, status []CodeSceneStatus) []Issue    // code_error for code scenes without source or with a failed bundle

type Catalog struct{ Schema string; Stage Stage; Rig RigRef; Fonts []Font; Palettes []Palette; ... }
type Entry struct{ ID, Kind, Name, NameKey string; Tags []string; Thumb string; UsageExample json.RawMessage; File string;
                   Params map[string]string; Spots map[string]Spot; Floor *Floor; Variants []string; Aspects map[string]AspectOverride }
func (e Entry) MarshalJSON() ([]byte, error)     // the entry exactly as the catalog has it
func (e Entry) Raw() json.RawMessage
func LoadCatalog(fsys fs.FS, path string) (*Catalog, error)
func (c *Catalog) VerifyFiles(fsys fs.FS, dir string) error   // fsys = ui tree root, dir = "img/storymaker"; fonts resolve to fonts/<file> in fsys
func (c *Catalog) Lookup(id string) (Entry, bool); Names(kind string) []string; HasName(kind, name string) bool
func (c *Catalog) Search(query, kind string, limit int) []Entry   // default 6, cap 12
func (c *Catalog) Version() string; Entries() []Entry
func (c *Catalog) Spots(envID, aspect string) (map[string]Spot, bool); SpotNames(envID string) []string; FocusX(envID, aspect string) float64; Variants(envID string) []string
func NormalizeKind(kind string) string            // "char" -> "character", "trans" -> "transition", ...

type BundleError struct{ Line, Column int; Message string }   // json: line, column, message; 1-based, columns count characters
type BundleResult struct{ Code []byte; Errors []BundleError }
func BundleCodeScene(source []byte, sceneID string, maxBytes int) BundleResult   // maxBytes <= 0 means 64 KiB

type Store interface{ ReadFile; WriteFile; Remove; List; MkdirAll }   // as contract; semantics below
func ETag(data []byte) string                                         // desktop.NoteVersion
type Project struct{ ... }; type CodeSceneStatus struct{ SceneID string; OK bool; Errors []BundleError }   // as contract
type ProjectStore; func NewProjectStore(st Store, root string) *ProjectStore   // methods as contract section 4
type MemStore; func NewMemStore() *MemStore   // Store + Paths() and Dirs() for assertions
func ValidProjectID(id string) bool; func Slugify(title, fallback string) string
```

## Issue codes (what part A emits)

| code | severity | when |
| --- | --- | --- |
| `invalid_json` | error | syntax error (with line and column), empty body, trailing data, wrong JSON type of a field (path from the byte offset) |
| `unknown_field` | error | key not in the format; one issue per key with path, hint for common mistakes, matching is exact (case-sensitive) |
| `schema_mismatch` | error | `schema` missing or not `storymaker/1` (also from `Validate` for a hand-built story) |
| `too_large` | error | story over `MaxStoryBytes` (decode) |
| `limit_exceeded` | error | scenes, actors+props, beats, narration chars, scene duration, summed explicit durations, chart entries |
| `missing_field` | error | title, scenes, scene/cast ids, per-beat required fields (fix carries the beat example), voice provider/language, music file |
| `invalid_value` | error / warning | enums, ranges, slug patterns, positions of the wrong shape, voice id pattern, music file path, wrong asset kind; warning for a variant the environment lacks, `with:"prev"` on the first beat, actors/props/beats in a code scene |
| `duplicate_id` | error | scene ids, cast ids, instance ids (props and show beats), the same cast member twice in a scene |
| `unknown_name` | error | pose, gesture, expression, transition, ease, palette, font, text style, beat kind, target; at most 5 alternatives in the form the field takes |
| `unknown_asset` | error | asset id not in the catalog or not an asset id (alternatives via tags and edit distance) |
| `unknown_spot` | error | spot not in the effective environment (chain: previous scene, `env.plain-stage`; 9:16 overrides applied) |
| `unknown_cast` | error / warning | cast id not in the story's cast (error); beat of an actor who is not on stage (warning) |
| `unknown_cue` | error / warning | `cue` without a `{n}` marker in the narration (error, fix lists the markers); `cue` in a silent scene (warning) |
| `canonicalised` | minor | `pose.typing` -> `typing` etc. (also expr., gesture., trans., palette., ease., text.) |
| `not_supported_yet` | minor | say/highlight/chart/sfx beats, show with icon or chart, transitions other than cut/fade/slide-left/wipe, music other than none, cast speaker, known pose/expression/gesture missing from the catalog |
| `code_error` | error | `CodeSceneIssues` only: code scene without source or with a failed bundle |

Not emitted by part A: `narration_missing`, `deadline`, `layout_unverified` (player/server). Messages are English and meant for the model and
as detail text; the app localises by `code` (`storymaker.issue.<code>`).

## Decisions

- **Pipeline**: `DecodeStory` -> `ApplyDefaults` -> `Validate` -> `CanonicalJSON` -> `SaveStory`. `Validate` also works before defaults, but then
  reports `missing_field` for an omitted `format`; `ApplyDefaults` first avoids that.
- **Decode** is a four-step check: size, JSON syntax, schema, unknown fields (reflection walk over the generic tree, so every unknown key is
  reported with its path in document order), then the typed strict decode as safety net. Type mismatches carry `invalid_json` (the contract
  list has no code for them); their path comes from `UnmarshalTypeError.Offset`.
- **Polymorphic values** (`Position`, scene `Duration`): a value of the wrong shape is kept as `Invalid` raw JSON, so the story saves unchanged
  and `Validate` reports `invalid_value` at the exact path instead of a path-less decode failure.
- **Style**: `Style.UnmarshalJSON` defaults `captions` to true when a style object omits it; an absent style stays zero and `ApplyDefaults`
  fills `style` whole.
- **ApplyDefaults** is the spec table plus the show-beat instance id (`<item-slug>-<n>`, per slug and scene, skipping taken ids; text/icon/chart
  shows use `text`/`<icon>`/`chart`). It also completes document-level fields a bare story may lack (schema, `format` 16:9/30, `style` sunny/default/captions
  on), listed in `defaults_applied` like the others. It does not default prop ids, prop scale or any timing. Several actors take left, right,
  far-left, far-right by their rank in `story.cast` (not scene order; wraps after four); one actor stands at center.
- **Numbers**: beat `amount`, `seconds`, `delay`, `duration`, `count`, `cue` and actor/prop `scale` are plain numbers with `omitempty`, so 0 means absent.
- **Support matrix**: `phase1Beats` and `phase1Transitions` in `model.go`; `validate.go` emits the minor notes. Phase 2 only edits these sets.
- **Catalog**: manifest checks only (schema, version, stage, rig, fonts incl. a font named `default`, palettes with the seven hex tokens, entry kinds,
  duplicate ids, prefix per kind, bare name = id without prefix for enum-like kinds, `name_key`, safe relative `thumb`/`file`, `usage_example` object, kind
  fields). Palettes live only in `palettes` (an entry of kind `palette` is refused) and are indexed as entries `palette.<name>`; fonts appear as
  `Names("font")`. `Search`: exact id 10, exact name 8, exact tag 4, name 3, partial tag/id 2, summed over terms; plural `s` is tried; ties keep catalog order.
- **Bundling**: esbuild first (syntax errors with line/column), the pre-check always (tokenizer over the author's source), errors merged, sorted, deduplicated,
  at most 20. The tokenizer reads a slash after `)`, `}`, `++`, `--` both as division and as regex start and merges the findings, so a quote
  inside a regex literal cannot hide code behind it. It decodes `\uXXXX` identifier escapes and `\xNN`/`\u` escapes in the string `["constructor"]`.
  It also requires an `export default` (`export { x as default }` counts). Concatenated strings (`"con"+"structor"`) are not caught; the CSP and the worker are the barriers.
- **ProjectStore**: `Create` writes `story.json` (title, format, default style, one scene `s1` on `env.plain-stage`, defaults applied) and `project.json`,
  creates `assets/`, `audio/narration/`, `audio/music/`, `exports/` (`scenes/` appears with the first code scene). Ids come from `github.com/google/uuid`.
  `SaveStory` stores the bytes as given, checks `MaxStoryBytes`, requires the project to exist, then mirrors title/aspect/fps into `project.json` and bumps
  `updated_at` (best effort, three etag retries). `SaveCodeScene` is serialised by a mutex, checks the size before writing, and succeeds with a failed bundle
  (errors in the result and in `scenes/<id>.errors.json` as a JSON array). `CodeSceneStatus` lists scenes that have a `.js` source, sorted.
- `etag` = `desktop.NoteVersion` imported from `internal/desktop` (no cycle today: desktop does not import storymaker). If a cycle appears later, copy the three lines
  (quoted hex sha256) into `ETag`.
- `Slugify`: Latin letters transliterated through a table (umlauts, accents, a few Polish/Czech/Nordic letters); other scripts (CJK) produce the fallback.

## What the other parts must do or know

**B (server)**
- Map errors: `*DecodeError` -> 422 `invalid_story` with `issues`; `ErrConflict` -> 412 (428 without `If-Match` is B's own check); `fs.ErrNotExist` -> 404;
  `ErrInvalidID`/`ErrInvalidArgument` -> 400; `ErrTooLarge` -> 413 (also check the body size before reading).
- Implement `Store` over `desktop.Service` with these semantics: `WriteFile` creates parents and honours `ifMatch` "" / `MustNotExist` / etag atomically
  (a stale etag and an existing file under `MustNotExist` both return `ErrConflict`); **`Remove` must delete a directory tree** (`os.RemoveAll`
  semantics; missing path is not an error); `List` returns base names of the direct children; `ReadFile` returns `ETag(data)`.
- Pass `lim.MaxDurationSeconds = float64(cfg.Storymaker.Render.MaxDurationSeconds)` (and the byte limits) so the validator agrees with the config; `DefaultLimits()`
  uses 900 s.
- Config PUT validation: `NormalizeStorymakerConfig` is called in `config.Load` only. The server validates config patches in `internal/server/config_patch.go:103` and
  `internal/server/config_handlers_main.go:688` (next to `NormalizeVideoStudioConfig`); add the Storymaker call there, or invalid UI values are only caught on the next load.
  `Config.Save` (config.go:3337 list of patches) does not write the storymaker section, like `retro_games`; the config UI patches the YAML directly.
- `GET /projects/{id}` issues: `Validate(story after defaults, catalog, lim)` plus `CodeSceneIssues(story, CodeSceneStatus)`; any `error` blocks a render. HTTP answers cap at
  `MaxHTTPIssues`, model-facing ones at `MaxModelIssues` via `CapIssues`.
- `Validate` and `ApplyDefaults` mutate the `*Story`; give each request its own copy. `Catalog` and `ProjectStore` are safe for concurrent use.
- Catalog: `LoadCatalog(webassets.Namespace("ui"), "img/storymaker/catalog.json")`; `catalog_version` = `cat.Version()`. Load once and cache; `Catalog` is immutable.
- Render slug: `Slugify(title, projectID)` (<= 80 chars); the output name is `<slug>-<first 8 of job id>.mp4`.

**C (player)**
- Read fixtures with line endings normalised (`\r\n` -> `\n`); a Windows checkout converts `testdata/scenes/*.js` to CRLF. The Go tests do the same.
- Story JSON after `ApplyDefaults` has: `format`, `style`, every scene's `environment`, `variant`, `speaker`, `transition`, `duration` ("auto" or number), `camera.framing`;
  every actor's `at`, `pose`, `facing`, `expression`, `scale`; every prop's `layer`; every show beat's `id`; every cast `name`. Not filled (the player defaults them): prop `id`,
  prop `scale` (absent = 1), all timing, `Position` can still be `{x,y}`. A story with `error` issues is not rendered, but the player should not crash on one (positions of a wrong
  shape are saved as written).
- Support matrix the player must match: beats enter exit walk move pose gesture emote look show(item/text) hide camera wait; transitions cut fade slide-left wipe. `say`,
  `highlight`, `chart`, `sfx`, show with icon/chart, the other six transitions, music and a cast speaker are accepted by the validator and must be ignored.
- Bundle contract: `BundleCodeScene` output = `esbuild.transform(src, {format:'iife', globalName:'__smSceneModule', target:'es2020', legalComments:'none', loader:'js', sourcefile:'scenes/<id>.js'}).code + "\nself.__smScene=__smSceneModule.default;\n"` (a blank line sits between the IIFE and the registration because esbuild ends with a newline).

**D (catalog)** must satisfy `LoadCatalog` (a failing manifest names every problem, at most 12 listed): schema, `version` (1 to 64 chars), `stage` sizes and safe area, `rig.id` and relative
`rig.file`, a font named `default` (all fonts need name/family/weight/licence and a `/fonts/` path), palettes with `bg bg2 fg accent accent2 text outline` as `#hex` and `name_key`, entries with
the right prefix, `name` equal to the id without prefix for pose/gesture/expression/mouth/cycle/text_style/transition, `name_key = "storymaker.catalog." + id with . -> _`, a relative `.svg` `thumb`,
an object `usage_example`, and per kind: character `params` (all ten keys non-empty); prop `file`, `size.h` > 0, `anchor.x/y`; environment `file`, the five base spots (0..100),
`floor` scales > 0, `variants` day/night, `aspects` keys only `9:16`/`1:1`; pose `bones` object; gesture `keys` >= 2 and `duration` > 0; expression `eyes`, `brows`, `mouth`; mouth `shape` in the seven names;
cycle `period` > 0 and `keys` >= 2; text_style `font size weight color align max_lines`; transition `duration` >= 0; sfx `file`. Run `LoadCatalog` and `VerifyFiles(ui, "img/storymaker")` in D's tests.

**E (app)**: the `code` of an issue selects the localised heading (`storymaker.issue.<code>`, 16 codes from A, see the table); `message` and `fix` are English detail text, shown as text only. `path` is a JSON Pointer
(`/scenes/1/beats/0/who`) to select the field in the inspector. Project and library assets (`my.*`, `lib.*`) are accepted by pattern only and not resolved by A.

## Deviations from the contract

None that change a contract name or signature. Clarifications and additions: `Store.Remove` is `RemoveAll` (the contract only said "missing -> nil"); `DecodeStory`'s error is a `*DecodeError`; type
mismatches use `invalid_json`; `ApplyDefaults` also fills schema/format/style; extra exported helpers (`ETag`, `CapIssues`, `HasErrors`, `CodeSceneIssues`, `NormalizeKind`, `SpotAt`, `PointAt`,
`AutoDuration`, `SecondsDuration`, catalog accessors, `MemStore`); files `decode.go`, `issues.go`, `memstore.go`; plan task A8 (a document) has no failing-test step.

## Open items

- Config PUT validation hook in `internal/server` (B or E, see above). No change to `Config.Save`.
- Pre-existing audit test failure on this Windows box (unrelated, unverified on base).
- `-race` is unavailable here (no cgo); the concurrent `SaveCodeScene` test ran without the race detector.
- Optional: a `.gitattributes` line `internal/storymaker/testdata/scenes/*.js text eol=lf` would keep the golden bundle LF on Windows checkouts; the tests do not depend on it.
- `Validate` knows nothing about `my.*`/`lib.*` assets (phase 3) and not about narration audio (phase 2).
