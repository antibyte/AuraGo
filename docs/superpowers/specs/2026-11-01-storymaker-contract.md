# Storymaker: interface contract

**Purpose.** This document fixes the names, formats, routes, limits and protocols that the Storymaker parts share: the story document, the catalog, the Go core, the HTTP API, the render server, the player runtime, the messages between the desktop app and the player, and the desktop app registration. Code comments and plan tasks cite it as "contract section N". The section numbers are the ones of the drafting stage and are kept on purpose.

**Status.** Phases 1 and 2. Matches the phase 1 and phase 2 plans. The parent document is the design spec `docs/superpowers/specs/2026-11-01-storymaker-design.md`. Where the spec and this document differ for phases 1 and 2, this document is the reference.

**Basis.** The text describes the integrated code (base `561685cde` with the review fixes of the integration step). Names, enums, limits and routes are taken from the Go and JavaScript sources, not from the drafting text.

---

## 0. Drafting rules

Drafting-only; omitted.

## 1. Plan part format

Drafting-only; omitted.

---

## 2. Story document `storymaker/1`

Go types: `internal/storymaker/model.go`. Keys are snake_case. A key that is not part of the format is refused (`unknown_field`). Defaults are listed under "Defaults" below. Timing (delay, duration, ease) is never defaulted; the player owns it.

```
Story       {schema:"storymaker/1", title:string 1..120, format:Format, style:Style, voice:Voice|null,
             music:Music|null, cast:[CastMember], scenes:[Scene]}
Format      {aspect:"16:9"|"9:16"|"1:1", fps:24|25|30}
Style       {palette:string, font:string, captions:bool, rhythm?:"calm"|"standard"|"lively", auto_sfx?:bool}
Voice       {provider:VoiceProvider, voice:string, language:string}   // provider and voice id are checked, see the notes
Music       {mode:"none"|"generated"|"file", mood?, prompt?, file?, track?, volume?:0..1}
CastMember  {id:slug, name?:string, character:assetId, voice?:Voice|null}
Scene       {id:slug, kind?:"storyboard"|"code", environment?:assetId, variant?:"day"|"night",
             narration?:string, silent?:bool, speaker?:"narrator"|castId, actors?:[Actor],
             props?:[Prop], beats?:[Beat], transition?:Transition, duration?:"auto"|number,
             camera?:Camera}
Camera      {framing:"wide"|"medium"|"close", target?:string}
Actor       {cast:castId, at?:Position, pose?:string, facing?:"left"|"right"|"auto",
             expression?:string, scale?:number}                         // scale 0.5..2, absent = 1
Prop        {id?:slug, item:assetId, at?:Position, layer?:"back"|"front", scale?:number}
                                                                       // scale 0.5..2, absent = 1
Beat        {do:BeatKind, who?, target?, from?:Position, to?:Position, pose?, gesture?,
             count?:1..5, expression?, text?, style?, item?, icon?, chart?,
             data?:{labels:[string], values:[number]}, at?:Position, entry?, id?, op?,
             amount?:number, sound?, seconds?:number, with?:"prev", cue?:int,
             delay?:0..600, duration?:0..600, ease?:Ease}
Position    string (spot name) | {x:number 0..100, y:number 0..100}   // percent of the safe area; y = feet line
Duration    "auto" | number (seconds, greater than 0)
BeatKind    enter exit walk move pose gesture emote look say show hide highlight camera chart sfx wait
Transition  cut fade slide-left slide-right slide-up slide-down wipe zoom iris whip
Ease        linear ease-in ease-out ease-in-out back-out bounce-out
VoiceProvider elevenlabs google minimax mistral openai phonon piper resemble sanotts speech_lab stepfun supertonic xai
slug        ^[a-z0-9][a-z0-9-]{0,47}$
assetId     ^(char|prop|env|sfx)\.[a-z0-9][a-z0-9.-]{0,63}$  |  ^(my|lib)\.(char|prop|bg)\.[a-z0-9][a-z0-9-]{0,63}$
```

Notes on the shape:

- `voice` and `music` may be `null`. `POST /narrate` synthesises; validation still checks only the shape of `voice` and of the provider id.
- `voice.provider` (and `cast[].voice.provider`) is a concrete TTS provider id, never `"active"`. The ids are the dispatcher ids of `internal/tools/tts.go` `synthesizeProvider`, kept as a sorted list in `validate.go` (`voiceProviders`; the package does not import `internal/tools`). The comparison ignores case and surrounding space, and `Validate` does not rewrite the value. `"active"` is refused as `invalid_value` at `<voice path>/provider` (a saved story names a concrete provider). Any other id outside the list, including the dispatcher aliases `s2s` and `s2s-lab` (a story saves only `speech_lab`), is `unknown_name` at `<voice path>/provider` with at most 5 closest alternatives and a `{"provider":"..."}` example. An empty provider is `missing_field`. The voice id must match `^[A-Za-z0-9._-]{1,128}$` (`invalid_value` at `<voice path>/voice`); the language is only required.
- `style` may be given without `captions`; the decoder then sets captions to true.
- Numbers `amount`, `seconds`, `delay`, `duration`, `count`, `cue` and the `scale` of actors and props are plain numbers with `omitempty`. A value of 0 means "absent".
- A `Position` or `Duration` of the wrong JSON shape is kept as written. The story therefore saves unchanged, and `Validate` reports `invalid_value` at the exact path.
- `kind` is not defaulted. An absent kind is a storyboard scene.
- `music.track` is a positive decimal media id and excludes `music.file`. `style.auto_sfx`, like `style.rhythm`, is not written when absent.
- `style.rhythm` is not written when absent.
- `Prop.id` is not defaulted. `show` beats get an instance id (see "Defaults").

### Closed name sets

| Set | Values |
| --- | --- |
| poses | stand sit typing point-left point-right wave arms-crossed hands-on-hips shrug think present thumbs-up celebrate facepalm phone hold lean |
| gestures | point wave shrug think present thumbs-up celebrate facepalm count |
| expressions | neutral happy laugh sad angry surprised confused thinking worried determined |
| mouth shapes (catalog and player only) | rest ai e o u mbp fv |
| transitions | cut fade slide-left slide-right slide-up slide-down wipe zoom iris whip |
| eases | linear ease-in ease-out ease-in-out back-out bounce-out |
| beat kinds | enter exit walk move pose gesture emote look say show hide highlight camera chart sfx wait |

Pose, expression, gesture, palette, text style, transition and ease values are bare names. The validator also accepts the prefixed form (`pose.typing`, `expr.`, `gesture.`, `trans.`, `palette.`, `ease.`, `text.`), rewrites it to the bare name and reports a `canonicalised` minor issue.

### Phase 2 support

| Item | Behaviour |
| --- | --- |
| Beats the player plays | enter, exit, walk, move, pose, gesture, emote, look, say, show (with `item` or `text` only), hide, highlight, camera, wait, sfx |
| Beats accepted and ignored | chart, and show with `icon` or `chart`. Each gets a minor `not_supported_yet` issue. |
| Transitions the player plays | cut, fade, slide-left, wipe |
| Transitions accepted, played as fade | slide-right, slide-up, slide-down, zoom, iris, whip. Each gets a minor `not_supported_yet` issue. |
| A known pose, expression or gesture that the catalog lacks | Minor `not_supported_yet`. The player uses stand, neutral, or no gesture. |
| An unknown name | Error `unknown_name`, with up to five alternatives. |
| Narration audio | From `/narrate`; until the gate landed a narrated story exports silently; a scene without audio gets `narration_missing`. |

Behaviour of the first scene: the player never plays an entry transition on scene 1, whatever `transition` says.

### Defaults

`ApplyDefaults` fills these paths only when the author omitted them. It lists each filled path in `defaults_applied` (JSON Pointer).

| Path | Default |
| --- | --- |
| `/schema` | `"storymaker/1"` |
| `/format/aspect` | `"16:9"` |
| `/format/fps` | `30` |
| `/style` | `{palette:"sunny", font:"default", captions:true}` when the object is absent; palette and font are filled one by one when empty |
| `/cast/<i>/name` | the cast `id` |
| `/scenes/<i>/environment` | the environment of the previous scene, else `env.plain-stage` |
| `/scenes/<i>/variant` | `"day"` |
| `/scenes/<i>/speaker` | `"narrator"` |
| `/scenes/<i>/transition` | `cut` on scene 1; `wipe` when the environment differs from the previous scene's; else `fade` |
| `/scenes/<i>/duration` | `"auto"` |
| `/scenes/<i>/camera` | `{framing:"wide"}` (also when `framing` is empty) |
| actors `at` | one actor: `center`. Several actors: `left`, `right`, `far-left`, `far-right` by the actor's rank in `story.cast` (wraps after four). |
| actors `pose` | `"stand"` |
| actors `facing` | `"auto"` |
| actors `expression` | `"neutral"` |
| actors `scale` | `1` |
| props `layer` | `"front"` |
| `show` beats `id` | `<item slug>-<n>`, counted per slug and scene, skipping ids already taken. Text shows use `text`, icons `<icon>`, charts `chart`. |

Not defaulted: `kind`, `silent`, `narration`, timing fields, prop `id`, prop `scale` (the player treats absent as 1).

### Limits

`DefaultLimits()` (Go) and the configuration decide these. Validate uses the limits it is given. A zero `Limits` value means the defaults.

| Limit | Default | Error |
| --- | --- | --- |
| scenes | 200 | `limit_exceeded` |
| actors plus props per scene | 40 | `limit_exceeded` |
| beats per scene | 60 | `limit_exceeded` |
| narration characters per scene | 1500 | `limit_exceeded` |
| story document size | 1 MiB | `too_large` (decode) |
| code scene size | 64 KiB | `too_large` |
| scene duration (sum of explicit durations too) | `storymaker.render.max_duration_seconds`, default 900 | `limit_exceeded` |

### Issues

Shape (JSON): `{severity, scene_id?, path, code, message, at_seconds?, fix?}`.

- `severity`: `error` blocks a render; `warning` does not; `minor` is information (`not_supported_yet`, `canonicalised`, `layout_unverified`).
- `path`: a JSON Pointer (RFC 6901) from the story root, for example `/scenes/1/beats/0/expression`.
- `message` and `fix` are English text meant for the model and for detail. The desktop app localises the heading by `code` (`storymaker.issue.<code>`).
- Model-facing answers carry at most 8 issues (`MaxModelIssues`). HTTP save answers carry at most 200 (`MaxHTTPIssues`).

Issue codes (fixed; the desktop app has a localised heading for each):

| Code | Severity | Emitted by |
| --- | --- | --- |
| `invalid_json` | error | Go decode: syntax, empty body, trailing data, wrong JSON type of a field |
| `unknown_field` | error | Go decode: one issue per unknown key, matching is case-sensitive |
| `schema_mismatch` | error | Go decode and Validate: `schema` missing or not `storymaker/1` |
| `too_large` | error | Go decode: story over the size limit |
| `limit_exceeded` | error | Go Validate: scenes, items, beats, narration, say text, scene duration, summed durations, chart entries |
| `missing_field` | error | Go Validate: title, scenes, ids, per-beat required fields, voice provider and language, music file, `/music/mood` |
| `invalid_value` | error or warning | Go Validate: enums, ranges, slugs, positions, asset kind, voice and music shape, the voice provider `"active"`. Warning: a code scene with actors, props or beats (ignored); `with: "prev"` on the first beat; a variant the environment lacks. Player: a duration shorter than the speech, narration or a say block (warning). |
| `duplicate_id` | error | Go Validate: scene ids, cast ids, instance ids of props and show beats, one cast member twice in a scene |
| `unknown_name` | error | Go Validate: pose, gesture, expression, transition, ease, palette, font, text style, beat kind, target, voice provider (not one of the `VoiceProvider` ids). Player: warning, with a tolerant fallback. |
| `unknown_asset` | error | Go Validate: asset id not in the catalog (alternatives by tag and edit distance) |
| `unknown_spot` | error | Go Validate and player: spot not in the effective environment |
| `unknown_cast` | error or warning | Go Validate: cast id not in the cast (error); a beat of an actor who is not on stage (warning) |
| `unknown_cue` | error or warning | Go Validate and player: `cue` without a `{n}` marker in the narration (error); a cue in a silent scene (warning) |
| `canonicalised` | minor | Go Validate: a prefixed name rewritten to its bare form |
| `not_supported_yet` | minor | Go Validate and player: see "Phase 2 support" |
| `narration_missing` | warning | Player compile report only (`plan.js`). Go does not emit it. |
| `code_error` | error | Go `CodeSceneIssues`: a code scene without source or with a failed bundle. Player: a code scene that fails to build or to draw. |
| `deadline` | error | Player compile report: a code scene build or a frame missed its deadline |
| `layout_unverified` | minor | Player compile report: one per code scene (layout inside code is not checked) |
| `glyph_missing` | error | Go Validate: a character no face covers, at the text path |
| `line_unread`, `dead_stretch`, `first_frame_empty`, `text_too_small`, `fade_between_worlds`, `cue_late` | warning | Player compile report, storyboard scenes |
| `entry_monotony`, `transition_variety` | minor | Player compile report |

Go constants are `Code<CamelCase>` (for example `CodeInvalidValue`). `DecodeStory` failures always return at least one issue, so a 422 response carries `issues`.

### Shared fixtures

Owned by part A and read by part C's Node tests. The text of the fixtures is the text of the drafting contract; parts A and C checked their copies byte for byte.

| File | Content |
| --- | --- |
| `internal/storymaker/testdata/stories/minimal.json` | one scene, "Hallo Welt." |
| `internal/storymaker/testdata/stories/firewall.json` | three scenes: a storyboard scene with `env.office` and a prop, a storyboard scene with `env.abstract-tech` and a camera zoom, and the code scene `s3` |
| `internal/storymaker/testdata/scenes/s3.js` | the source of the code scene of `firewall.json` |
| `internal/storymaker/testdata/scenes/s3.bundle.js` | golden bundle of `s3.js` (1365 bytes), produced by `BundleCodeScene` and reproduced byte for byte by Node esbuild 0.28.2 |
| `internal/storymaker/testdata/narration/cross_language.json` | rows `{input, plain, cues, chars_total, width, words, pieces}`, read by a Go test and a Node test |
| `internal/storymaker/testdata/stories/speech.json` | a story with CJK, RTL, Greek and Hindi text and bubbles |

---

## 3. Catalog format `storymaker-catalog/1`

The catalog is produced by `scripts/build-storymaker-catalog.mjs` from `assets/storymaker/`. Its output lives in `ui/img/storymaker/` and is served at `/img/storymaker/`. Production Go code reads it through `webassets.Namespace("ui")` at `img/storymaker/catalog.json`; tests inject `os.DirFS("ui")`.

### Manifest

```
catalog.json  {schema:"storymaker-catalog/1", version, stage, rig, glyphs, fonts, palettes, entries}
version       12 hex characters: sha256 over the bytes of every file below ui/img/storymaker/ except catalog.json,
              in sorted path order; text files LF-normalised, WAV files raw.
stage         {width:1920, height:1080, safe:{x:5, y:5, w:90, h:90}}   // design frame, percent safe area
rig           {id:"rig.human/1", file:"rig/human.json"}
fonts         [{name:"default", family:"Geist", weight:400|500|600|700, file:"/fonts/Geist-<face>.woff2",
                licence:"OFL-1.1"}]                                    // one entry per face
glyphs        {file:"glyphs.json"}
palettes      [{id:"palette.sunny"|"palette.calm", name, name_key, tokens:{bg, bg2, fg, accent, accent2, text, outline}}]
entries       [Entry]
```

- Fallback faces live only in `glyphs.json`. They are not `fonts` entries and `style.font` cannot name them.
- The default font is Geist (four faces: Regular 400, Medium 500, SemiBold 600, Bold 700; files `/fonts/Geist-Regular.woff2`, `Geist-Medium.woff2`, `Geist-SemiBold.woff2`, `Geist-Bold.woff2`). Montserrat was rejected because `ui/fonts` has only weight 500 in unicode-range files.
- Palette tokens are hex colours. Palette `sunny` is the palette of the mini fixture; `calm` is the second starter palette.
- `LoadCatalog` checks the manifest and the entry fields. `LoadCatalog` reads `glyphs.file` too. It does not read the other files named by entries. `VerifyFiles` checks that every referenced file exists (used by tests).

### Entries

Common fields: `{id, kind, name, name_key, tags:[string], thumb, usage_example:object}`.

- `name` is the bare name (no prefix) for pose, gesture, expression, mouth, cycle, text_style and transition entries. For character, prop and environment it equals the id.
- `name_key` is `storymaker.catalog.` followed by the id with `.` replaced by `_`, for example `storymaker.catalog.char_office-woman-1`.
- `thumb` is `thumbs/<id>.svg`. Every entry except `sfx` has a thumb. Every thumbnail is a deterministic SVG composition (no PNG).

| Kind | Fields |
| --- | --- |
| `character` | `params`: `body`, `skin`, `hair`, `hair_color`, `eyes`, `brows`, `facial_hair`, `glasses`, `outfit`, `accessory` (all non-empty) |
| `prop` | `file` (`props/<name>.json`), `size.h` (fraction of the stage height at depth 1, greater than 0), `anchor` `{x, y}` (0 to 1 of the bounding box), `hold` (optional bool) |
| `environment` | `file` (`environments/<name>.json`), `spots` `{<name>: {x, y, depth?, face?}}` (the five base spots `far-left`, `left`, `center`, `right`, `far-right` are required; x and y are 0 to 100 in the design frame), `floor` `{y_near, y_far, scale_near, scale_far}`, `variants` `["day", "night"]`, `aspects?` (keys `9:16` and `1:1` only: `{focus_x, spots?}`) |
| `pose` | `bones` `{<bone>: degrees}`, `hands?` `{l?, r?}` (`open`, `fist`, `point`, `hold`), `view?` (`front`, `three_quarter`, `profile`: the best view for the pose), `seated?` (bool), `offset_y?` (rig units, positive is down) |
| `gesture` | `keys` `[{t: 0..1, bones, hands?}]` (at least two; the last key has t = 1), `duration` (seconds, greater than 0) |
| `expression` | `eyes`, `brows`, `mouth`: names of the variants in `rig/human.json` |
| `mouth` | `shape`: one of `rest ai e o u mbp fv` |
| `cycle` | `period` (seconds), `keys` `[{t, bones, offset_y?}]` (at least two), `speed?` (walk only: percent of the design width per second at depth scale 1) |
| `text_style` | `font` (`default` or a font name), `size` (fraction of the stage height), `weight`, `color` (`$text` or hex), `box?` `{fill, radius, pad}` (`radius` and `pad` are fractions of the stage height), `align` (`left`, `center`, `right`), `max_lines` |
| `transition` | `duration` (seconds, 0 or more) |
| `sfx` | `file` (take 1), `takes[{file, dur_ms, onset_ms, peak_ms, impact_ms, l50_lufs, true_peak_dbtp}]`, `impact_ms`, `align` (`peak`, `onset`), `hit?{over_db, dip_db}`, `pattern?{spacing_ms:[min,max], default_duration_ms}`, `source{kind, basis, url}` |

Palettes are listed only in `palettes` and are indexed as entries `palette.<name>`. Fonts are listed only in `fonts`; `Names("font")` returns their names.

### Starter IDs (phase 1, exact)

The tests and the mini fixture rely on these IDs.

| Kind | Count | IDs |
| --- | --- | --- |
| characters | 4 | `char.office-woman-1` `char.office-man-1` `char.it-tech-1` `char.senior-man-1` |
| props | 23 | `prop.laptop` `prop.phone` `prop.monitor` `prop.server` `prop.router` `prop.cloud` `prop.database` `prop.lock` `prop.key` `prop.shield` `prop.mail` `prop.document` `prop.coffee-cup` `prop.desk` `prop.chair` `prop.plant` `prop.check` `prop.cross` `prop.warning` `prop.idea` `prop.question` `prop.arrow-right` `prop.gear` |
| environments | 4 | `env.plain-stage` (spots `left center right far-left far-right board`), `env.office` (+ `desk door board window`), `env.home-office` (+ `desk door window`), `env.abstract-tech` (+ `board`) |
| poses | 10 | `pose.stand` `pose.sit` `pose.typing` `pose.point-left` `pose.point-right` `pose.wave` `pose.think` `pose.present` `pose.arms-crossed` `pose.hands-on-hips` |
| gestures | 9 | all names of the gesture set |
| expressions | 10 | all names of the expression set |
| mouth shapes | 7 | all names of the mouth set |
| cycles | 2 | `cycle.walk` `cycle.idle` |
| text styles | 6 | `text.title` `text.caption` `text.label` `text.speech` `text.thought` `text.subtitle` |
| transitions | 4 | `trans.cut` `trans.fade` `trans.slide-left` `trans.wipe` |
| palettes | 2 | `palette.sunny` `palette.calm` |
| sfx | 7 | `sfx.whoosh` `sfx.pop` `sfx.click` `sfx.ding` `sfx.success` `sfx.error` `sfx.typing` |

The catalog has 86 entries (4 + 23 + 4 + 10 + 9 + 10 + 7 + 2 + 6 + 4 + 7) and 2 palettes.

Text styles use these usage examples: `title` as in the mini fixture; `caption` and `label` add `"style"` to the `show` beat, so the validator accepts `style` on `show` beats with `text`.

### Locale keys

Names come from `ui/lang/storymaker_catalog/<lang>.json` (16 files). The key of an entry is its `name_key`. The prefix `storymaker.catalog.` is reserved for these files; the UI strings live in `ui/lang/storymaker/<lang>.json` and must never define a `storymaker.catalog.*` key.

### Thumbnails

`thumbs/<id>.svg`, one per entry except `sfx`. Character thumbnails show the rig posed in `stand`, three-quarter view, cropped to head and torso. Pose and gesture thumbnails show the woman, expression and mouth thumbnails her head in front view. Thumbnails are `<svg>` documents without script, images or links.

### Glyphs

`ui/img/storymaker/glyphs.json` holds the coverage of each face. The manifest names it as `"glyphs":{"file":"glyphs.json"}`, and the file joins the `version` hash and `--check`.

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

- The `d` values above show the format only; the build writes the real ones. Decode: the first `d` value is the first range start, then `gap = start - previous end`, `len = end - start`.
- A codepoint is covered if it is in the face's cmap and inside `unicode_range`. `geist` data excludes U+0370-03FF.
- Family order: `SMGreek`, the style family in place of `Geist`, `SMDeva`, `SMArab`, `SMCJK`.
- A face whose `family` equals a catalog font family adds no need. Its faces are loaded through `catalog.fonts`.
- The host attaches the parsed file as `catalog.glyph_data`. That key exists only in memory, never in `catalog.json`.

---

## 3a. Rig semantics

Normative for the player and for the catalog build. The rig file is `rig/human.json` (`rig.id` `rig.human/1`, `units_height`, `height` 0.55 of the stage height at depth 1, `seat_drop` 215).

1. **Units and placement.** Rig units. `units_height` is the rig height of an adult (feet line to top of the head). The origin is the midpoint of the feet line, and y points down. Shipped characters are 1000 rig units tall.
2. **Screen scale.** `rig.height × 1080 × depth × actor.scale × body.scale / units_height`. The root is placed at the feet position of the actor on screen.
3. **Depth.** Linear between `floor.y_far → scale_far` and `floor.y_near → scale_near`, clamped. A spot may give `depth` (0 far, 1 near) instead of a y lookup.
4. **Bone transform.** `world(b) = world(parent) · T(rest[view].x, rest[view].y) · R(rest[view].angle + pose.bones[b] + Σ deltas[b])`. Angles are in degrees, clockwise positive. `rest.x` and `rest.y` are offsets in the parent's frame (the root: rig space). `bodies[].bones[b].length` and `.rest` override the rig values for a body type.
5. **Bones (13).** `root hips torso neck head upper_arm_l lower_arm_l upper_arm_r lower_arm_r upper_leg_l lower_leg_l upper_leg_r lower_leg_r`. The end parts `hand_l hand_r foot_l foot_r` are slots attached to the lower bones.
6. **End parts.** Hands hang at local y = 135 of the lower arms; feet (shoes) at local y = 205 of the lower legs. Every body type keeps these lengths; body types change rest offsets and shapes only.
7. **Views.** Default `three_quarter`, towards the facing direction. A pose may give `view` as its best view. Walking uses `profile`. `look camera` uses `front`. A view missing from a slot's shapes falls back to `three_quarter`, then `front`.
8. **Facing.** Facing left mirrors the actor about the root x after the view is chosen.
9. **Limb sides.** Views face right. `_r` is the character's right limb. In `front` view `_r` is on the screen right (x > 0) and `_l` on the left. In `three_quarter` and `profile` `_r` is the near limb (drawn above the torso, on the back side at x < 0) and `_l` is the far limb. A negative angle swings a limb towards +x.
10. **Slots and variant keys.** A slot takes its variant key from the character's value for `slot.param`. The player resolves the params `body`, `skin`, `hair`, `eyes`, `brows`, `mouth`, `hand`, `top`, `bottom`, `shoes`, `glasses`, `facial_hair` and `accessory`. The shipped rig uses no slot with the param `skin`. The back hair slot `hair_back` has the param `hair`. `top`, `bottom` and `shoes` come from `params.outfit[<outfit>]`. `hand_l` and `hand_r` (param `hand`) use `pose.hands.l` and `pose.hands.r`, default `open`.
11. **Eyes and brows.** Keys are `<character value>/<expression variant>`, falling back to `<character value>/neutral`. The optional key `<value>/blink` drives a blink (0.12 s every 3.6 s, desynchronised per actor). A value `none` draws nothing; a missing variant draws nothing.
12. **Mouth.** The current speech shape (`rest ai e o u mbp fv`), else the mouth of the expression, else `rest`. Expression mouths (for example `smile`, `frown`) are variants of the same slot. Speech shapes come from the player's lip-sync track.
13. **Seated poses and the root drop.** A pose lowers the root by its `offset_y` (rig units, positive is down). `pose.sit` and `pose.typing` have `offset_y` 215. A `seated` pose without `offset_y` uses `rig.seat_drop`, then `rig.seated_offset`, then the length of `lower_leg_l`, then 0.24 × `units_height`. The drop blends with the pose blend.
14. **Point poses.** For an actor who faces left, `point-left` and `point-right` swap their bones, hands and view, so that the name keeps the screen direction. The swap happens only when both entries exist.
15. **Gesture and cycle keys.** Keys are additive bone deltas on top of the pose, eased ease-in-out per segment. `offset_y` (rig units) shifts the root. The hands of a gesture key hold for each side until a later key sets that side again. At t = 1 a gesture returns to the base pose and the pose hands return.
16. **Walk speed.** `cycle.walk.speed` is percent of the design width per second at depth scale 1, scaled by the mean depth scale along the path.
17. **Colour tokens.** Actor shapes resolve `$skin $hair $top $bottom $shoes` from the character params and `$outline $eye_white $iris $lip` from `params.fixed`, then from the palette. Environment shapes resolve from `env.tokens[variant]`, then the palette. Props resolve from the palette. Gradient stops may be tokens (for example `[[0, "$bg"], [1, "$bg2"]]`). Text colours and the label box fill resolve from `env.tokens[variant]`, then the palette. An unresolved token draws `#ff00ff` and is reported in tests.
18. **Shapes.** A shape has `fill` or `gradient`, and may have `stroke`. A shape with neither is rejected by the catalog build. Strokes use round caps and joins. Colours are `#rrggbb`, `#rrggbbaa` or `$token`.
19. **Environment layers.** Layers draw in their listed order (back to front). `parallax` `p` moves a layer by `(1 - p)` of the camera pan; it does not set the draw order.
20. **Environment tokens.** The day variant omits `bg` and `bg2` (the palette decides). Night overrides them. Dark variants carry a light `text` token, so titles stay readable. `abstract-tech` also has a dark `bg2`.
21. **Aspect overrides.** A 9:16 or 1:1 output shows a vertical slice of the 16:9 design frame, 31.6 or 56.3 percent of the width at design height. `aspects.<ratio>.focus_x` sets the centre of the slice. A spot with no override keeps its design position and is moved into the slice only when it would fall outside (3 percent margin). An override may carry `face`.
22. **Beat positions.** Spot positions are percent of the design frame. `{x, y}` positions are percent of the visible safe area of the output aspect. `y` is always the feet line.
23. **Text styles.** `size` is a fraction of the canvas height. `box.pad` and `box.radius` are fractions of the canvas height. A box without numbers uses 0.4 em padding and 0.3 em corners. `font: "default"` means the story's `style.font`; the catalog face with the nearest weight is used.
24. **Props.** `size.h` is a fraction of the design height at depth 1, multiplied by the depth scale and `Prop.scale`. The `anchor` is a fraction of the viewbox. The anchor lands on the spot.

---

## 3b. Mini catalog fixture

`internal/storymaker/testdata/catalog-mini/catalog.json` is a small catalog for the tests. Its version is `mini00000000` and it has one palette (`palette.sunny`). Its 26 entries are `sfx.whoosh`, `sfx.pop`, `sfx.typing`, `char.office-woman-1`, `prop.lock`, `prop.shield`, `env.plain-stage`, `env.office`, `env.abstract-tech`, `pose.stand`, `pose.typing`, `gesture.point`, `expr.neutral`, `expr.surprised`, `expr.happy`, `mouth.rest`, `cycle.walk`, `cycle.idle`, `text.title`, `text.speech`, `text.thought`, `text.subtitle`, `trans.cut`, `trans.fade`, `trans.slide-left`, `trans.wipe`. Twenty of them are the mini fixture text recorded by parts A and C; phase 2 added the three speech styles and the three `sfx` entries.

Ownership: part A owns the catalog file and `LoadCatalog` reads only its manifest. The stub files it names (`rig/human.json`, `props/lock.json`, `props/shield.json`, `environments/*.json`) are owned by part C and follow section 3a. Fixtures never live under `ui/`, because assetpack ships `testdata` directories.

---

## 4. Go package `internal/storymaker`

The package has no `internal/server` dependency. The render subpackage is `internal/storymaker/render`.

### Story and catalog API

```go
const Schema = "storymaker/1"
const CatalogSchema = "storymaker-catalog/1"
const MustNotExist = "-"                       // Store.WriteFile: create only
const MaxModelIssues = 8                      // model-facing answers
const MaxHTTPIssues = 200                     // HTTP save answers
const DefaultEnvironment = "env.plain-stage"  // DefaultAspect "16:9", DefaultFPS 30, DefaultPalette "sunny", DefaultFont "default"
const SeverityError, SeverityWarning, SeverityMinor   // "error" "warning" "minor"
// Code<CamelCase> constants for the 28 issue codes of section 2

var ErrConflict = errors.New(...)          // failed If-Match or MustNotExist on an existing file
var ErrInvalidID, ErrInvalidArgument, ErrTooLarge error
var PoseNames, GestureNames, ExpressionNames, MouthNames, TransitionNames, EaseNames, BeatKinds []string

type Story, Format, Style, Voice, Music, CastMember, Scene, Camera, Actor, Prop, Beat, ChartData   // section 2
type Position struct{ Spot string; X, Y float64; Point bool; Invalid json.RawMessage }
func SpotAt(name string) *Position; func PointAt(x, y float64) *Position
type Duration struct{ Auto bool; Seconds float64; Invalid json.RawMessage }
func AutoDuration() *Duration; func SecondsDuration(s float64) *Duration
type Issue struct{ Severity, SceneID, Path, Code, Message string; AtSeconds *float64; Fix string }
type DefaultApplied struct{ Path string; Value any }
type Limits struct{ MaxScenes, MaxItemsPerScene, MaxBeatsPerScene, MaxNarrationChars, MaxStoryBytes, MaxCodeBytes int; MaxDurationSeconds float64 }
func DefaultLimits() Limits
type DecodeError struct{ Code, Message string; Issues []Issue }   // the error of DecodeStory
func DecodeStory(data []byte, lim Limits) (Story, []Issue, error)  // error is *DecodeError when present
func Validate(s *Story, cat *Catalog, lim Limits) []Issue          // canonicalises names in place; cat == nil skips catalog checks
func ApplyDefaults(s *Story) []DefaultApplied                      // static defaults only; never timing
func CanonicalJSON(s Story) ([]byte, error)                        // 2-space indent, struct key order, no HTML escaping, trailing newline
func CapIssues(issues []Issue, n int) []Issue; func HasErrors(issues []Issue) bool
func CodeSceneIssues(s *Story, status []CodeSceneStatus) []Issue  // code_error for code scenes without source or with a failed bundle
func ETag(data []byte) string                                      // desktop.NoteVersion: quoted lowercase sha256

type Catalog struct{...}
type Entry struct{ ID, Kind, Name, NameKey string; Tags []string; Thumb string; UsageExample json.RawMessage; File string;
                   Params map[string]string; Spots map[string]Spot; Floor *Floor; Variants []string; Aspects map[string]AspectOverride;
                   Takes []SFXTake; ImpactMS float64; Align string; Hit *SFXHit; Pattern *SFXPattern; Source *SFXSource }
func (e Entry) MarshalJSON() ([]byte, error); func (e Entry) Raw() json.RawMessage   // the entry as the catalog has it
func LoadCatalog(fsys fs.FS, path string) (*Catalog, error)        // path "img/storymaker/catalog.json" in the ui namespace
func (c *Catalog) VerifyFiles(fsys fs.FS, dir string) error        // dir "img/storymaker"; fonts resolve to fonts/<file> in fsys
func (c *Catalog) Lookup(id string) (Entry, bool)
func (c *Catalog) Names(kind string) []string                       // bare names of enum-like kinds; fonts under "font"
func (c *Catalog) HasName(kind, name string) bool
func (c *Catalog) Search(query, kind string, limit int) []Entry     // default 6, cap 12
func (c *Catalog) Version() string; func (c *Catalog) Entries() []Entry
func (c *Catalog) Spots(envID, aspect string) (map[string]Spot, bool)
func (c *Catalog) SpotNames(envID string) []string; func (c *Catalog) FocusX(envID, aspect string) float64
func (c *Catalog) Variants(envID string) []string
func NormalizeKind(kind string) string                              // "char" -> "character", "trans" -> "transition", ...
```

`Search` ranks an exact id 10, an exact name 8, an exact tag 4, a name 3, a partial tag or id 2. Scores of several terms are summed, the plural "s" is tried, ties keep the catalog order.

`LoadCatalog` checks the manifest: schema, `version` (1 to 64 characters), `stage`, `rig`, `glyphs` (read through `LoadGlyphs` when the manifest names it), the font named `default`, palettes with the seven hex tokens, entry prefixes, bare names, `name_key`, relative thumbs, `usage_example` objects and the fields of each kind.

`LoadCatalog` also checks the sfx fields: `file` equals `takes[0].file`, at least 2 takes, `align` set, `source` set, and the thumb check is skipped for this kind. `VerifyFiles` checks every take and skips an empty thumb.

### Music API

```go
type SFXTake struct{ File string; DurMS, OnsetMS, PeakMS, ImpactMS, L50LUFS, TruePeakDBTP float64 }
type SFXHit struct{ OverDB, DipDB float64 }
type SFXPattern struct{ SpacingMS [2]float64; DefaultDurationMS float64 }
type SFXSource struct{ Kind, Basis, URL string }

var MusicMoods = []string{"upbeat-corporate", "calm-ambient", "inspiring", "playful", "tech-pulse", "dramatic", "lo-fi", "acoustic"}
func MusicPrompt(mood, prompt string) string
func MusicKeyGenerated(mood, prompt string) string
func MusicKeyFile(path string, size int64, mtime time.Time) string
func MusicKeyTrack(id string, size int64, mtime time.Time) string
func MusicVolume(m *Music) float64                // 0.25 when absent
type MusicMeta struct{ V int; Ext string; DurationMS int; LoudnessI *float64 }   // JSON: v, ext, duration_ms 1..3600000, loudness_i? in LUFS, -100..20
func (p *ProjectStore) WriteMusic(ctx context.Context, id, key, ext string, data []byte) error
func (p *ProjectStore) WriteMusicMeta(ctx context.Context, id, key string, m MusicMeta) error
func (p *ProjectStore) MusicInfo(ctx context.Context, id, key string) (meta MusicMeta, audioExt string, err error)
```

Keys are lower-case hex sha256 of `generated|mood|prompt`, `file|path|size|mtime` or `track|id|size|mtime`, with `mtime` as decimal `UnixNano`. Length is excluded. `meta.json` holds the duration, so `GET /narration` never decodes a track. The helper package `internal/storymaker/storymakertest` seeds narration and music for tests (`SeedNarration`, `SeedMusic`).

### Code scenes

```go
type BundleResult struct{ Code []byte; Errors []BundleError }      // Errors non-empty means Code is nil
type BundleError struct{ Line, Column int; Message string }        // JSON: line, column, message; 1-based; columns count characters
func BundleCodeScene(source []byte, sceneID string, maxBytes int) BundleResult   // maxBytes <= 0 means 64 KiB
```

The bundle is esbuild output with format `iife`, global name `__smSceneModule`, target `es2020`, no legal comments, loader `js`, source file `scenes/<id>.js`. The Go code appends `"\nself.__smScene=__smSceneModule.default;\n"`. The source is size-checked first, esbuild checks the syntax, and then a tokenizer over the author's source (always run) rejects these: `import` or `export from` other modules, dynamic `import()`, and the identifiers `eval`, `Function`, `globalThis`, `self`, `window`, `document`, `importScripts`, `fetch`, `XMLHttpRequest`, `WebSocket`, and `.constructor`. It also requires an `export default`. Positions in its messages refer to the author's source. The version of esbuild is pinned to 0.28.2 in `go.mod` and in `package.json`.

### Store and project storage

```go
type Store interface {
    ReadFile(ctx context.Context, path string) ([]byte, string, error)            // data, etag; missing: errors.Is(err, fs.ErrNotExist)
    WriteFile(ctx context.Context, path string, data []byte, ifMatch string) (string, error)
    Remove(ctx context.Context, path string) error
    List(ctx context.Context, dir string) ([]string, error)
    MkdirAll(ctx context.Context, dir string) error
    Open(ctx context.Context, path string) (io.ReadSeekCloser, FileInfo, error)   // missing: fs.ErrNotExist
}
```

Store semantics (the desktop implementation is in `internal/server/storymaker_store.go`):

- `WriteFile` creates the parent folders. `ifMatch` "" writes unconditionally; `MustNotExist` creates only; any other value must equal the current etag. A failed precondition returns `ErrConflict`.
- `Remove` works like `os.RemoveAll`: a folder tree is removed, and a missing path is not an error.
- `List` returns the base names of the direct children. A missing folder gives `nil, nil`.
- `ReadFile` returns `ETag(data)`.
- `Open` returns a seekable file for the audio routes, which serve files over 50 MB through it. `FileInfo` is `{Name string; Size int64; ModTime time.Time; MIME string}`. `MemStore` and `storymakerStore` are the only implementers.

```go
type Project struct {
    ID string `json:"id"`; Title string `json:"title"`; Aspect string `json:"aspect"`; FPS int `json:"fps"`
    CreatedAt time.Time `json:"created_at"`; UpdatedAt time.Time `json:"updated_at"`   // RFC 3339
    TargetSeconds int `json:"target_seconds,omitempty"`; Audience string `json:"audience,omitempty"`; Template string `json:"template,omitempty"`
}
type CodeSceneStatus struct { SceneID string `json:"scene_id"`; OK bool `json:"ok"`; Errors []BundleError `json:"errors"` }
type ProjectStore struct{...}
func NewProjectStore(st Store, root string) *ProjectStore          // root "Documents/Storymaker"
func (p *ProjectStore) Create(ctx context.Context, title, aspect string, fps int) (Project, Story, string, error)
func (p *ProjectStore) List(ctx context.Context) ([]Project, error)
func (p *ProjectStore) Get(ctx context.Context, id string) (Project, error)
func (p *ProjectStore) LoadStory(ctx context.Context, id string) ([]byte, string, error)
func (p *ProjectStore) SaveStory(ctx context.Context, id string, data []byte, ifMatch string) (string, error)
func (p *ProjectStore) SaveCodeScene(ctx context.Context, id, sceneID string, source []byte, ifMatch string) (string, BundleResult, error)
func (p *ProjectStore) LoadCodeSource(ctx context.Context, id, sceneID string) ([]byte, string, error)
func (p *ProjectStore) LoadCodeBundle(ctx context.Context, id, sceneID string) ([]byte, error)   // fs.ErrNotExist when none
func (p *ProjectStore) CodeSceneStatus(ctx context.Context, id string) ([]CodeSceneStatus, error)
func (p *ProjectStore) Delete(ctx context.Context, id string) error
func ValidProjectID(id string) bool                                // lowercase uuid v4
func Slugify(title, fallback string) string                       // ASCII letters and digits with dashes; Windows device names refused; 80 characters at most
```

- `Create` writes `story.json` (title, format, default style, one scene `s1` on `env.plain-stage`, defaults applied) and `project.json`. It creates `assets/`, `audio/narration/`, `audio/music/` and `exports/`. The folder `scenes/` appears with the first code scene. `CreateWithVoice` stores a resolved voice.
- `SaveStory` stores the bytes as given, refuses more than the story size limit (`ErrTooLarge`), requires the project to exist, mirrors title, aspect and fps into `project.json` and sets `updated_at`.
- `SaveCodeScene` refuses source over 64 KiB (`ErrTooLarge`) before anything is written. It checks the precondition first; a failed precondition changes nothing. On success it writes `scenes/<id>.js` and `scenes/<id>.bundle.js`, and removes `scenes/<id>.errors.json`. When the bundle fails it writes the source, removes the old bundle, and writes `scenes/<id>.errors.json` as a JSON array. A bundle failure is not a Go error: the result carries the errors.
- `Delete` removes the project folder (`Store.Remove`, recursive). The `DELETE` endpoint also cancels the project's render, narrate and music jobs. Code files have no delete of their own: a code scene removed from the story leaves `scenes/<id>.js` in place.
- Ids are lowercase uuid v4 (`github.com/google/uuid`). Scene ids are slugs.

Error mapping for the HTTP layer: `*DecodeError` gives 422 `invalid_story`; `ErrConflict` gives 412; `fs.ErrNotExist` gives 404; `ErrInvalidID` gives 400 `invalid_id`; `ErrInvalidArgument` gives 400 `invalid_argument`; `ErrTooLarge` gives 413 `too_large`.

### Narration, envelopes, audio info, glyphs and voices

```go
const BlockKindNarration, BlockKindDialogue    // "narration" "dialogue"
const JoinSentence, JoinSplit                  // "sentence" (350 ms pause after the piece), "split" (150 ms)

type NarrationBlock struct { ID, Kind, SceneID string; SceneIndex, Beat int; Speaker string; Voice Voice; Text string }
type Piece struct { Text string; Chars int; Join string }
func Blocks(s *Story) []NarrationBlock
func StripMarkers(text string) string
func SplitPieces(text string, limit int) []Piece
func NormalizeProvider(id string) string       // trimmed and lower-cased
func ChunkLimit(provider string) int           // runes: phonon 2000, sanotts 2000, google 200, others 400
func PieceKey(v Voice, pieceText string) string
func PiecesOf(b NarrationBlock) []Piece
func IsJSSpace(r rune) bool
func UTF16Len(s string) int
func TextWidth(s string) int                   // East Asian wide counts 2
func WordCount(s string) int

var ErrUnknownAudio error
type DecodedAudio struct { Rate, Channels int; Samples []float32 }
func SniffAudio(head []byte) string            // "wav", "mp3" or ""
func DecodeAudio(data []byte) (DecodedAudio, string, error)
func ProbeAudio(r io.ReadSeeker) (ext string, durationMS int, err error)

type Levels []uint8                            // marshals as a JSON array of integers 0..255
type Envelope struct { V, DurationMS, Rate int; Levels Levels }   // audio/narration/<hash>.env.json
func ComputeEnvelope(a DecodedAudio) Envelope
func (e Envelope) SpeechRuns() [][2]int
func ParseEnvelope(data []byte) (Envelope, error)
type AudioPieceInput struct { Hash, Ext string; Chars int; Join string; Env Envelope }
type AudioPiece struct { Hash, Ext string; AtMS, InMS, OutMS, Chars int }
type AudioBlock struct { ID, Kind string; Beat *int; Speaker string; DurationMS int; Spans [][2]int; Levels Levels; Pieces []AudioPiece; SpeechOK bool }
func AssembleBlock(b NarrationBlock, pieces []AudioPieceInput, fps int) AudioBlock

type SceneAudio struct { Blocks []AudioBlock }
type AudioInfo struct { Narration map[string]SceneAudio }
type MissingBlock struct { SceneID, BlockID string; Chars int }
type CharCounts struct { Total, Missing int }
type MusicRef struct { Key, Ext string; DurationMS int; LoudnessI *float64 }
type AudioAnswer struct { Narration map[string]SceneAudio; Missing []MissingBlock; Chars CharCounts; Music *MusicRef; MusicIssue string }
func (a AudioAnswer) Info() AudioInfo
var ErrMusicSourceMissing error
type MusicKeyResolver func(ctx context.Context, m *Music) (string, error)
func (p *ProjectStore) AudioAnswer(ctx context.Context, id string, s *Story, resolve MusicKeyResolver) (AudioAnswer, error)
type PieceFile struct { Ext string; HasEnv bool }
func (p *ProjectStore) PieceFiles(ctx context.Context, id string) (map[string]PieceFile, error)
func (p *ProjectStore) AudioPath(id, folder, file string) (string, error)
func (p *ProjectStore) OpenAudio(ctx context.Context, id, folder, file string) (io.ReadSeekCloser, FileInfo, error)
func (p *ProjectStore) WritePiece(ctx context.Context, id, hash, ext string, data []byte, env Envelope) error
func (p *ProjectStore) WriteEnvelope(ctx context.Context, id, hash string, env Envelope) error
func (p *ProjectStore) RemovePiece(ctx context.Context, id, hash, ext string) error

const GlyphsSchema = "storymaker-glyphs/1"
type GlyphFace struct { ID, Family, File, Weight, UnicodeRange string; MaxBytes int64 }
type Glyphs struct { Faces []GlyphFace }
func LoadGlyphs(fsys fs.FS, path string) (*Glyphs, error)   // at most 1 MiB, schema, monotonic ranges
func (g *Glyphs) FaceOf(r rune) int
func (g *Glyphs) Covered(r rune) bool
func (g *Glyphs) Missing(text string) []rune
func IsDefaultIgnorable(r rune) bool
func (c *Catalog) Glyphs() *Glyphs                          // nil when the manifest names no glyphs file
func (c *Catalog) GlyphsFile() string

func (p *ProjectStore) CreateWithVoice(ctx context.Context, title, aspect string, fps int, voice *Voice) (Project, Story, string, error)
type VoiceOption struct { ID, Name, Language, Gender string; Installed *bool; Note string }
type VoiceProvider struct { ID string; Active, Ready bool; Voices []VoiceOption; FreeText bool; Languages, TestedLanguages []string; NeedsInstall bool; DefaultVoice string; ChunkLimit int; Note string }
var ErrNoVoiceForLanguage error
func ResolveProjectVoice(providers []VoiceProvider, language string, defaultVoiceFor func(provider, language string) string) (Voice, []string, error)
func LanguageBase(tag string) string
func LocalVoiceProvider(id string) bool                     // phonon, piper, sanotts, supertonic, speech_lab
func ApplyCaptionOverride(s *Story, burnIn *bool)
```

- A block enters `Narration` only when every piece has audio and a valid envelope. Otherwise it appears in `Missing`, and the player estimates it. `Missing` and `Narration` are never null.
- `ErrMusicSourceMissing` is not an error of `AudioAnswer`: the narration stays, `Music` is null and `MusicIssue` is `source_missing`.
- Layout: `audio/narration/<hash>.<ext>`, `audio/narration/<hash>.env.json`, `audio/music/<key>.<ext>`.

### Configuration

`internal/config/storymaker.go`:

```go
type StorymakerConfig struct {
    Enabled bool                   `yaml:"enabled" json:"enabled"`        // loader default true (config.go:577)
    Render  StorymakerRenderConfig `yaml:"render" json:"render"`
    Narration StorymakerNarrationConfig `yaml:"narration" json:"narration"`
}
type StorymakerRenderConfig struct {
    TimeoutSeconds     int    `yaml:"timeout_seconds" json:"timeout_seconds"`          // default 3600, 60..14400
    MaxDurationSeconds int    `yaml:"max_duration_seconds" json:"max_duration_seconds"` // default 900, 10..3600
    BrowserPath        string `yaml:"browser_path" json:"browser_path"`                // optional absolute path, 1024 characters at most
    Transport          string `yaml:"transport" json:"transport"`                      // "", png, webp, raw, screenshot
}
type StorymakerNarrationConfig struct {
    MaxDailyChars int `yaml:"max_daily_chars" json:"max_daily_chars"` // default 100000, 1000..100000000
    MaxJobChars   int `yaml:"max_job_chars" json:"max_job_chars"`     // default 20000, 100..10000000
}

func NormalizeStorymakerConfig(c *StorymakerConfig) error   // defaults, ranges, path and transport; does not change Enabled
```

The loader calls `NormalizeStorymakerConfig` (`config.go:1091`). The configuration patch path (`config_patch.go:106`) and the configuration PUT validation (`config_handlers_main.go:693`) call it too, so an invalid value is refused before the file is written. `timeout_seconds` is the minimum render timeout; the render raises it for long videos (section 7).

---

## 5. HTTP API `/api/desktop/storymaker`

Handler: `internal/server/storymaker_handlers.go`. Routes are registered in `internal/server/server_routes.go`.

### Access rules

- GET endpoints need the desktop read scope (`desktop:read`). Mutations need `desktop:write`. Job cancel uses the `desktopStop` operation, so it works while the desktop is read-only.
- Read-only desktop: mutations get 403 `desktop_readonly` from the shared middleware.
- Storymaker disabled (`storymaker.enabled` false): every endpoint except `GET /status` answers 403 `storymaker_disabled`. `GET /status` answers 200 with `enabled:false` (see below).
- Virtual desktop disabled: 503 `desktop_disabled` (every endpoint except `GET /status`).
- Shared middleware: 401 `unauthorized`, 403 `desktop_scope_required`.
- Unknown path: 404 `not_found`. Wrong method: 405 `method_not_allowed`. A malformed project id: 404 `project_not_found`.

### Error body

Flat, as in the rest of the desktop API:

```json
{"error": "<code>", "code": "<code>", "message": "...", "issues": [Issue]}
```

`issues` appears only when there are issues to show.

### Routes

| Method and path | Scope | Success | Errors besides the shared ones |
| --- | --- | --- | --- |
| `GET /status` | read | 200 status object (below), `Cache-Control: no-store` | 500 `configuration_invalid`, 500 `storage_unavailable` |
| `GET /projects` | read | 200 `{projects:[Project]}`, newest first | |
| `POST /projects` `{title, aspect?, fps?, language?}` | write | 201 `{project, story, etag}`, `ETag` and `Location` headers. Defaults 16:9 and 30 fps. With `language` the story has the resolved `voice`; without it `voice:null`. | 400 `invalid_request` (body), 400 `invalid_argument` (title 1 to 120 characters, aspect, fps 24, 25 or 30), 422 `narration_unavailable` with `languages` |
| `GET /projects/{id}` | read | 200 `{project, story, etag, issues, code_scenes}`. `issues` = Validate of the story after defaults, plus `code_error` for each code scene without a bundle. `code_scenes` is never null. | 404 `project_not_found` |
| `DELETE /projects/{id}` | write | 204. Cancels the project's jobs. | 404 `project_not_found` |
| `PUT /projects/{id}/story` | write | 200 `{etag, issues, defaults_applied, story}`. `story` is the canonical form after defaults. The new `ETag` is in the body. Validation errors (unknown assets and so on) do not block the save; they come back in `issues`. | 428 `precondition_required` (no If-Match), 400 `precondition_invalid` (If-Match is not a quoted sha256), 412 `precondition_failed` (stale; the current tag is in the `ETag` header), 413 `too_large` (over 1 MiB), 422 `invalid_story` (undecodable; nothing is saved, `issues` says why) |
| `GET /projects/{id}/scenes/{scene}/code` | read | 200 `text/javascript` source with `ETag` | 404 `code_not_found` |
| `PUT /projects/{id}/scenes/{scene}/code` | write | 200 `{etag, ok, errors:[{line, column, message}]}`. A syntax error still saves the source and answers `ok:false`. | 428 `precondition_required` (send If-Match, or `If-None-Match: *` for a new scene), 400 `precondition_invalid`, 412 `precondition_failed`, 413 `too_large` (over 64 KiB), 400 `invalid_request` (unreadable body) |
| `GET /projects/{id}/scenes/{scene}/bundle` | read | 200 `text/javascript` IIFE with `ETag` | 404 `bundle_not_found` (the last bundle failed or does not exist) |
| `POST /projects/{id}/render` with optional body `{burn_in_captions?, srt, vtt, stems, quality}` | write | 202 `{job}` | 428, 400 `precondition_invalid`, 412 `precondition_failed`, 400 `invalid_idempotency_key`, 400 `invalid_request`, 409 `idempotency_conflict`, 422 `invalid_story` (with `issues`), 422 `code_error`, 409 `ffmpeg_unavailable`, 409 `browser_unavailable` (message is the status issue), 409 `narration_incomplete` (`missing`, `chars`), 409 `music_incomplete` (`key`, `music_issue?`), 429 `busy` |
| `POST /projects/{id}/music` `{duration_ms, generate?}` | write | 202 `{job}`; 200 `{job}` existing; 200 `{job:null}` when no music is needed, the key's file and meta exist, or mode generated has no stored file and the call lacks `generate:true`. `duration_ms` is required for mode generated, 1000 to `max_duration_seconds*1000`. `generate:true` is the user's click, and the job calls a provider only for the key this call read. Coalesced per project and kind. | 400 `invalid_request`, 400 `invalid_argument`, 404 `project_not_found`, 409 `music_unavailable` (mode generated while `cfg.MusicConfigured()` is false), 409 `budget_exceeded`, 422 `invalid_music_file` (source not wav or mp3 by its bytes, over 1 GiB, or missing, then with `music_issue:"source_missing"`), 429 `busy` |
| `GET /projects/{id}/jobs` | read | 200 `{jobs:[Job]}`, newest first, across the render and narrate queues | |
| `GET /projects/{id}/events` | read | `text/event-stream` (see below), across the render and narrate queues | |
| `GET /jobs/{id}` | read | 200 `{job}`, across the render and narrate queues | 404 `job_not_found` |
| `POST /jobs/{id}/cancel` | write, `desktopStop` | 200 `{job}`, across the render and narrate queues. Also answers for a finished job. | 404 `job_not_found` |
| `GET /player` | read, session cookie only | 200 HTML, the player host page (section 6) | 403 `storymaker_disabled` |
| `GET /voices` | read | 200 `{providers:[VoiceProvider]}`: the active provider first, then phonon, piper, sanotts, supertonic, speech_lab, then the cloud ids sorted. `no-store`, no secret. google is absent unless active. Works read-only. | none; a provider whose list fails has `ready:false` |
| `GET /projects/{id}/narration` | read | 200 `AudioAnswer`, `no-store`. Works read-only. | 404 `project_not_found`, 422 `invalid_story` |
| `POST /projects/{id}/narrate` (empty body) | write | 202 `{job}` new; 200 `{job}` existing queued or running job of this project; 200 `{job:null}` nothing missing. Coalesced per project and kind. | 404 `project_not_found`, 422 `invalid_story`, 409 `voice_required` (any block without a voice, message names the first), 409 `narration_unavailable` (provider not ready or not offered, message names it), 409 `voice_unknown` with `alternatives` (at most 5; a voice off the provider's list, or a Phonon voice that is not installed), 409 `budget_exceeded` with `{limit, missing}`, 429 `busy`, 403 `desktop_readonly` |
| `GET,HEAD /projects/{id}/audio/narration/{hash}.{ext}` | read | 200 or 206 through `http.ServeContent`, Range honoured. Works read-only. | 404 `audio_not_found`, 405 |
| `GET,HEAD /projects/{id}/audio/music/{key}.{ext}` | read | 200 or 206, Range honoured. Works read-only. | 404 `audio_not_found`, 405 |

Render details:

- `If-Match` on the render call is the story etag of the request. The render takes a snapshot of the story (after defaults) and of the code bundles when the job is queued. Edits made while the job waits change nothing.
- `Idempotency-Key` is optional: 1 to 128 printable characters. The same key, etag and options return the same job, again with 202; another etag or other options give 409 `idempotency_conflict`.
- Sixteen jobs may wait (`max_queued_renders`). A seventeenth gives 429 `busy`.
- 128 narrate and music jobs may wait, in their own queue. `/narrate` and `/music` coalesce per project and kind.

Audio routes:

- The folder segment must be exactly `narration` or `music`, else 404 `audio_not_found`. The file segment must match `^([0-9a-f]{64})\.(wav|mp3)$`.
- `.env.json`, `.meta.json` and `.pending.json` are never served.
- Headers: `Content-Type: audio/wav` or `audio/mpeg`, `X-Content-Type-Options: nosniff`, `Content-Security-Policy: sandbox; default-src 'none'`, `Cache-Control: private, max-age=86400, immutable`.

Narration answer:

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
  "music": {"key":"<64 hex>","ext":"mp3","duration_ms":61000,"loudness_i":-14.2}
}
```

- `music.loudness_i` is the music job's measurement of the stored track in LUFS, for the preview's bed gain. It is absent without FFmpeg, after a failed meter and for a silent track. The mix measures for itself. A story in mode `file` with no source yet has no `music_issue`: `music` is null, `POST /music` answers `{job:null}` and the validator reports `/music/file`.
- `beat` appears on dialogue blocks only. `pieces[].chars` is UTF-16 units including joining whitespace. `missing[].chars` and `chars.*` are code points of the piece text, which is what cloud caps count.
- `music_issue` appears only as `"source_missing"`, together with `music:null`.

### Status object

`GET /status` answers:

| Key | Type | Meaning |
| --- | --- | --- |
| `enabled` | bool | `storymaker.enabled` |
| `read_only` | bool | the desktop is read-only |
| `desktop_enabled` | bool | the virtual desktop is enabled |
| `ffmpeg` | object | `ready` (bool), `version?` (the version token of `ffmpeg -version`, for example `8.1.1-full_build-www.gyan.dev`, not the banner line), `issue?` (text when not ready). FFmpeg comes from `video_studio.ffmpeg_path` or PATH. |
| `browser` | object | `ready` (bool), `source` (`config`, `system`, `pinned` or `missing`), `issue?`, `can_download?` (bool: the pinned Chromium can be downloaded by the first export), `version?` |
| `limits` | object | `max_duration_seconds`, `max_scenes`, `max_items_per_scene`, `max_beats_per_scene`, `max_narration_chars`, `max_story_bytes`, `max_code_bytes`, `max_queued_renders` (16), `max_queued_narrations` (128), `max_daily_chars`, `max_job_chars` |
| `transport` | string | the transport in use (`png` by default) |
| `catalog_version` | string | the catalog version, for example `ab33e88a6b77` |
| `music_configured` | bool | `cfg.MusicConfigured()`: music generation can run. The music panel offers mode `generated` only then. |

When Storymaker or the desktop is disabled the answer has only `enabled`, `read_only` and `desktop_enabled`. Export is possible when `ffmpeg.ready` and (`browser.ready` or `browser.can_download`). The first status call can take a few seconds, because it probes the browser.

### Job object

```
Job {id, project_id, kind:"render"|"narrate"|"music", status, progress:0..1, phase?, frames_done, frames_total,
     pieces_done?, pieces_total?,
     output?:{path, size, stems?:{narration?, music?, sfx?}, srt?, vtt?, library?},
     warnings?:[{code, message}], error?:{code, message}, created_at, updated_at, started_at?, finished_at?}
status  queued | running | succeeded | failed | cancelled | interrupted
phase   starting | compiling | mixing | rendering | encoding | writing | synthesizing | generating
```

- `phase` is omitted while empty. `starting` covers the browser start and a first-time download. `mixing` runs between the report and the first frame.
- `output.path` is the desktop path `Documents/Storymaker/<project id>/exports/<slug>-<first 8 characters of the job id>.mp4`. `size` is in bytes.
- A job that was running when AuraGo stopped is `interrupted` with the error code `interrupted`.
- A narrate job has no `output` key. `pieces_done` and `pieces_total` show its progress.

Job error codes: `invalid_story`, `code_error`, `browser_unavailable`, `ffmpeg_unavailable`, `render_failed`, `render_timeout`, `cancelled`, `interrupted`, `storymaker_disabled`, `narration_failed`, `narration_unavailable`, `voice_required`, `budget_exceeded`, `narration_incomplete`, `music_failed`, `music_incomplete`.

Export files, written by the server into the desktop workspace through the desktop service (the MP4 as a conditional stream write, the record as a plain write):

- `Documents/Storymaker/<project id>/exports/<slug>-<job8>.mp4`
- `Documents/Storymaker/<project id>/exports/<job id>.json` with the job id, title, output path, frames, fps, size, encoder, transport, catalog version, render seconds, creation time, `stems`, `srt`, `vtt`, `library`, `warnings`, `loudness:{i, tp, master_gain_db}` and `cue_precision`.

The library copy is `Videos/Storymaker/<slug>.mp4`, with `-2`, `-3` on collision. It goes through `desktop.Service.WriteStorymakerVideo`. Stems and subtitles live in `exports/<slug>-<job8>/`. A failed extra leaves the MP4 and adds a warning.

### Events

`GET /projects/{id}/events` is a server-sent event stream:

1. A comment line `: storymaker events`.
2. Every unfinished job across both queues and the latest five jobs across both queues, oldest first, as `event: job` with `data: {"job": {...}}`. A window opened during a render sees its progress at once.
3. Every later state or progress change as `event: job`. Progress notices come at most every 150 ms.
4. A comment `: ping` every 15 seconds.

The stream is excluded from gzip and has no write deadline.

### Status and error keys used by the app

The app reads `limits.max_narration_chars` for the narration length limit, `issues` and `code` of each error, and `ETag`. A `story` the server cannot decode (a hand-edited `story.json`) is answered as `story: null` with `story_text` (the raw text) and the issues of the raw text.

---

## 6. Player runtime

Source: `ui/js/storymaker/`. Classic scripts, each in its own IIFE, shared through `StorymakerCore` (the pure modules) and `StorymakerHost` (the document realm). Parts are listed in `scripts/storymaker-bundle-parts.js`, which the bundle build and the Node tests both read.

- `ui/js/storymaker/host.bundle.js` (6 parts, as listed in `scripts/storymaker-bundle-parts.js`: `schedule`, `host-core`, `host-app`, `host-pool`, `host-render`, `host`) runs in the document realm: the page, the host, the render driver and the parallel scheduler.
- `ui/js/storymaker/player.bundle.js` (30 parts, in the order of `scripts/storymaker-bundle-parts.js`; phase 2 added `seed`, `ranges`, `motion`, `lint`, `glyphs`, `cuemap`, `captions`, `lipsync`, `bubble`) runs in workers (one blob per worker, prelude `self.__SM_ROLE__ = "player" | "scene"; importScripts(<base>/js/storymaker/player.bundle.js);`, plus the scene bundle text for a scene worker).
- `.gitattributes` keeps both bundles LF (`ui/js/storymaker/*.bundle.js text eol=lf`).
- `package.json` scripts: `test:storymaker` (`node scripts/test-storymaker-player.mjs`), `build:storymaker-catalog`, `check:storymaker-catalog` (with `--check`). `node scripts/build-storymaker-ranges.mjs` generates `ui/js/storymaker/ranges.js` and `internal/storymaker/ranges_gen.go`; with `--check` it fails on a stale output. Node runs the tests with a vm-based runner.
- `ui/css/storymaker-player.css` makes the canvas fill the page with `object-fit: contain` on black.

### Page markup

Served by `GET /api/desktop/storymaker/player` (the app iframe) and by the render listener (`<base>/`). Neither page runs inline script.

```html
<!-- app player host: GET /api/desktop/storymaker/player; ORIGIN = the absolute AuraGo origin of the request -->
<!doctype html><html data-sm-mode="app" data-sm-base="ORIGIN"><head><meta charset="utf-8">
<link rel="stylesheet" href="ORIGIN/css/storymaker-player.css"></head>
<body><canvas id="sm-stage"></canvas><script src="ORIGIN/js/storymaker/host.bundle.js"></script></body></html>

<!-- render page: GET <render base>/ ; BASE = http://127.0.0.1:<port>/<prefix> -->
<!doctype html><html data-sm-mode="render" data-sm-base="BASE"><head><meta charset="utf-8"></head>
<body><canvas id="sm-stage" width="W" height="H"></canvas><script src="BASE/js/storymaker/host.bundle.js"></script></body></html>
```

The player page answers with these headers:

- `Content-Security-Policy`: `sandbox allow-scripts; default-src 'none'; script-src <origin>/js/storymaker/ blob:; worker-src blob:; img-src blob: data:; font-src blob: data:; connect-src 'none'; style-src <origin>/css/storymaker-player.css; form-action 'none'; base-uri 'none'; object-src 'none'; frame-ancestors 'self'`
- `Referrer-Policy: no-referrer`, `Cache-Control: no-store`, `X-Content-Type-Options: nosniff`, `X-Frame-Options: SAMEORIGIN`.

The origin comes from the request host (validated) and the scheme (`https` when TLS or a trusted proxy says so). The sandbox has no `allow-same-origin`, so the page cannot read the desktop's cookies, storage or API. The page needs only the session cookie.

### Globals

- `host.bundle.js` mounts itself on load from `document.documentElement.dataset` (`smMode`, `smBase`) and `#sm-stage`. `window.StorymakerHost.mount({mode, canvas, base, boot?})` stays available for tests.
- App mode accepts messages only from `event.source === window.parent` and posts to the origin of `data-sm-base`. The parent posts with targetOrigin `'*'`, because the iframe origin is opaque.
- Render mode fetches everything from `base` (section 7).
- Pure modules are exported on `self.StorymakerCore` (worker realm) and `globalThis.StorymakerCore` (Node):
  - `compile(story, audioInfo, catalog, opts) → {timeline, report, schedule, needs}`
  - `planTiming(story, audioInfo, catalog, opts) → {fps, scenes: {<id>: {index, transition_frames, narration_start, …}}}` (used before code scenes are built)
  - `renderFrame(timeline, n, ctx, assets, opts)`
  - `ease(name, t)`
- `audioInfo` is `{}` or `{narration:{<scene id>:{blocks:[Block]}}}`, the `narration` value of `GET /projects/{id}/narration`. A block's absence means the scene or say text is estimated (words ÷ 2.3 s, at least 1.0 s), and a scene's narration or a say's text then gets `narration_missing`.

### Source rules

No template string with data, no `innerHTML` and no `eval` in the player source. A guard test greps the sources for these.

### Compile report

```
{frame_count, fps, width, height, duration_ms, catalog_version,
 scenes:[{id, start_frame, end_frame, kind, cues:[{marker, at_ms, placed}]}],
 audio_events:[{kind:"narration"|"dialogue", ref, start_ms, scene_id, block_id, piece} | {kind:"sfx", ref, start_ms, scene_id, take}],
 captions:[{start_ms, end_ms, lines, scene_id, block_id}], cue_precision, checks:[],
 unverified:[Issue], issues:[Issue]}
```

- `end_frame` is exclusive. `frame_count` is the largest `end_frame`. `duration_ms = round(frame_count × 1000 / fps)`.
- Transitions overlap: `start_frame[i] < end_frame[i-1]` is allowed.
- `checks` is empty until phase 3. `unverified` holds one minor `layout_unverified` per code scene. `captions` is always filled, measured or estimated. Compile drops audio events at or past `duration_ms` and cuts captions at it.
- `issues` holds the player's issues (`narration_missing`, `not_supported_yet`, `unknown_cue`, `unknown_spot`, `unknown_cast`, `unknown_asset`, `unknown_name`, `missing_field`, `duplicate_id`, `invalid_value` (warnings, tolerant fallbacks), and `code_error` or `deadline` (errors)). An error issue blocks the render job.
- `audio_events` has one entry per piece and one per sound; Go owns the gain, so there is no `gain` field. For `sfx`, `ref` is the catalog id and `take` is a 0-based index into its `takes`. `start_ms` is always film-global: for `sfx` it is `Math.round((sceneBase + eventTime)*1000)`, where `eventTime` is scene-local, the `start` of scheduling step 7 for a plain sfx and the key's event time for a typing key. `cue_precision` is the share of cues placed by a span, or null without cues. `scenes[].cues[].at_ms` is scene-local.

### Timing rules (summary)

- Scene time 0 is the start of the entry transition. Narration starts when the transition ends. A beat without a cue starts there too; a cued arrival may start earlier, down to scene time 0.
- A cue time comes from `cueMap(block, offset)`: inside the piece holding the offset, proportional over that piece's used audio, starting at its `at_ms`; `placed:proportion` over the block duration when pieces are unmeasured, their `chars` do not sum to the text length, or a piece had no detectable speech.
- Automatic scene length is the largest of: narration end plus 0.4 s, last dialogue block end plus 0.4 s, the end of the last beat, the end of the last text hold, and 1.0 s for a scene without speech. A code scene without `api.duration` takes its last motion plus 0.5 s; with narration, it takes the larger of that and this automatic length.
- An explicit `duration` is the total local length, entry transition included. Narration or a say block that runs past an explicit duration is one `warning` at `/scenes/<i>/duration`: the speech overlaps the next scene or is cut at the film end.
- Scene boundaries are whole frames: `start_frame[i] = max(start_frame[i-1] + 1, end_frame[i-1] - tf[i])`, where `tf = round(duration × fps)`.
- Beat defaults, scaled by the rhythm factor unless explicit: enter and exit 0.6 s (they walk); walk distance at `cycle.walk.speed`, at least 0.3 s; move 0.8 s; pose 0.3 s; gesture = catalog duration × `count`; emote 0.25 s; look 0.3 s; show 0.5 s (0 with `entry: "none"`); hide 0.35 s; camera 0.8 s (shake 0.5 s); highlight 0.8 s; say the block length; sfx 0 (a pattern `default_duration_ms`). `wait` is `seconds` (default 1), never scaled.
- Scheduling, in beat order. State: `cursor = T`, `prev`, `anchor`, the last end per target instance, `beatsEnd`. `f` is the rhythm factor.
  1. A non-object is skipped. An unknown kind, `chart`, or a show with `icon` or `chart` gets its minor issue and leaves the state untouched.
  2. A beat's target is the resolved instance of `who`, else `target`. A show's target is the instance it creates. `wait`, `sfx`, `chart`, a camera without a target, and a string that resolves to no instance (a camera on a spot) have none. Targets compare as instances, never as strings.
  3. `floor` is the end of the last beat on the same target, else 0.
  4. `dur`: an explicit `duration` wins. `wait` is `seconds`, else 1, never scaled. Otherwise it is the default above times `f`. `walk` is `max(0.3, distance ÷ speed) × f`, with the distance from the target's position at `max(floor, 0)`.
  5. An arrival (`enter`, `walk`, `move`, a show with an entry other than `none`, camera `focus`, camera `zoom` with a target) has `lead = leadFraction(ease) × dur`: the share of the duration until the ease first reaches 0.9, or for the pop spring until it first crosses 1. Every other beat has lead 0.
  6. Start time `t`: (a) a cue that resolves gives `t = cue time + delay - lead`, and a cue beats `with`; (b) else `with: "prev"` with a previous beat gives `t = prev.start + delay`; (c) else, when the previous beat and this beat both have a target and the targets differ, `t = anchor.start + 0.4 × anchor.dur + delay`; (d) else `t = cursor + delay`. An unresolved cue warns `unknown_cue` and falls through to b, c or d.
  7. Rules a, c, d: `start = max(0, floor, t)`. Rule b: `start = max(0, t)`, with no floor, so a `with: "prev"` beat stays together with the previous beat, also on the same target. A cued beat held back by its target gets a `cue_late` warning at `<beat>/cue` with `at_seconds = start + lead`. A clamp at 0 never warns.
  8. `landing = start + lead` for an arrival, else none. `end = start + dur`. `cursor`: rule b gives `max(cursor, end)`, the others give `end`. `anchor`: under rule b it moves only when `end` reaches the anchor's end, otherwise it is this beat. `prev` is always this beat.
  9. After the loop, `holdsEnd` is the largest `landing + scaled hold` of a text that no `hide` removes. A show with entry `none` has no landing and holds from its `start`. A hold is `0.5 + 0.3 × words`, at least 1.2 s for a caption or label in a scene that shows a title, and `max(1, f)` times that when scaled.
- Impact (`sfx`): `impact = chosen take impact_ms/1000`, 0 when the catalog has no such entry. The take comes from `seed.takeIndex` with `lastTake.get(sound)`, and `lastTake` is updated. A cued sfx uses `t = cueTime + delay - lead - impact`. An sfx with `with:"prev"` uses `(prev.landing ?? prev.start) + delay - impact`.
- Typing: `prng = seed.random(sceneId+':'+j+':'+sound)`; `x` starts at the beat's start plus the first take's impact; key events sit at event time `x - impact` of that key's take, with `x` advancing by `90+50*prng()` ms while `x < start+dur`; the take comes from `seed.takeIndex`.
- Auto-SFX, when `style.auto_sfx`: each show with entry `pop` gets a `sfx.pop` event with its impact on the landing, unless an sfx beat has the same `cue` or runs `with:"prev"` on it.
- `say` with text starts at the later of its scheduled start and the previous block's end, sets the new block end, and emits one `dialogue` event per piece. An estimated say with text gets a warning `narration_missing` at `/scenes/<i>/beats/<j>/text`. It adds the bubble and, with measured levels, a `speech` interval on `who`. When the actor is off stage the events stay, and there is no bubble and no interval. `say` without text keeps its start, lasts 0 and leaves the block end, the cursor and the target's last end unchanged. Its bubble stays until the end of the scene narration, with the 0.12 s fades, and its lines come from `SM.captions.at` at each time. A start at or after that end draws no bubble, and a silent scene draws none.
- Camera framings: wide 1.0, medium 1.35, close 1.9. `zoom` multiplies the current zoom (default 1.25). `focus` zooms to `amount` (default 1.6) on a target. `pan` moves to a target or by `amount` percent of the width. `shake` is `amount` design pixels (default 14).
- `show` text uses the `title` style at the upper third. Text is drawn in screen space; camera movement does not move it. Bubbles and burned captions are drawn by the player; bubbles follow the camera.
- A `show` instance id is `<item slug>-<n>` (per item and scene; props first) unless `id` is given. `hide` with `target` names an instance id; without one it hides the latest instance of the item.

### Deadlines

| Step | Limit | Notes |
| --- | --- | --- |
| worker `hello` | 5 s | a worker that does not say hello is replaced by the storyboard fallback |
| compile (plan and compile) | 10 s | `code_error` or `deadline` |
| code scene build | 2 s | `deadline` |
| assets (readiness barrier) | 20 s | |
| first 3 frames after a load | 4 s | both modes (JIT warm-up) |
| frame, app mode | 250 ms | a deadline terminates the engine |
| frame, render mode | `boot.frame_deadline_ms` (5000 from Go), capped at 60 s | |

A code-scene frame error ends that scene's worker once; the scene shows a placeholder from then on. A player-worker overrun ends the engine.

### Code scene API

A code scene is a function `export default function scene(api) { ... }` in JavaScript. The main calls:

- `api.duration(seconds)`: the local length (an explicit story `duration` wins). The scene keeps it, and a longer narration is a warning. A scene that does not call it takes its last motion plus 0.5 s; with narration, the larger of that and the automatic scene length.
- `api.add(assetId, {at, pose?, ...})`: an actor (`char.*`) or a prop; returns a handle with `gesture(name, {start})` and `emote(name, {start})`.
- `api.text(string, style, {at})`: a text handle (style `title`, `caption` or `label`).
- `api.shape('rect' | 'circle' | 'line', {at, w, h, fill, stroke})`.
- `api.tween(handle, props, {start, dur, ease, from?})`: `props` may include `opacity x y scale rotation width height` (x and y in percent, width and height in design pixels). A handle is visible from its start; `fadeIn` hides it until its start.
- `api.camera.zoom(...)`, `api.camera.pan(...)`, `api.camera.shake(...)`.
- `api.cue(n)`: the time of the `{n}` marker in the narration.
- `onFrame(fn)`: `fn(t, g)` draws each frame through a frozen facade `{save, restore, translate, rotate, scale, alpha, fill, stroke, rect, strokeRect, circle, line, path, text}` in design coordinates.
- `api.chart` and `api.diagram` are not supported in phase 1: they draw nothing and report `not_supported_yet`.

An error thrown by the API names the fix in its message. A thrown error carries `line` and `column` of the worker script; esbuild reformats the source, so these are approximate, while the Go bundle errors carry the author's positions.

### Worker messages (internal)

Host and workers exchange `{type, req, ...}` messages. Replies are `{type:'reply', req, ok:true, ...}` or `{type:'reply', req, ok:false, error:{code, message, line?, column?}}`. Each worker posts `{type:'hello', role}` once it runs.

- Player worker: `plan`, `compile`, `assets`, `render {n, width?, codeFrames, codeErrors, png?}` (returns `{bitmap}` or `{blob}`), `dispose`.
- Scene worker: `build {opts}` (returns `duration`, `needs`, `issues`), `assets`, `frame {t, width?}`, `dispose`.

---

## 7. Render listener and page

Go side: `internal/storymaker/render/` (`listener.go`, `ingest.go`, `render.go`, `browser.go`, `session.go`, `egress.go`, `ffmpeg.go`, `jobs.go`, `mix.go`, `mixinput.go`, `meter.go`, `gate.go`, `subtitles.go`, `options.go`).

### Listener

- Per job, on `http://127.0.0.1:<port>` (loopback only). The render base is `<origin>/<prefix>`, where `prefix` is 32 hex characters from `crypto/rand`.
- A request whose Host header is not the listener address gets 404 (protection against DNS rebinding). A path outside `<base>/` gets 404.
- Timeouts: read-header 10 s, read, write and idle 2 minutes each.
- Every response under `<base>` has `Access-Control-Allow-Origin: *`, `Cache-Control: no-store`, `X-Content-Type-Options: nosniff`, `Referrer-Policy: no-referrer`. The render page has an opaque origin, so every request from it is cross-origin with `Origin: null`; the prefix is the credential for reads, and the token of `boot.json` is the credential for `/report`, `/frame` and `/error`.
- `OPTIONS` answers 204 with `Access-Control-Allow-Methods: GET, POST`, `Access-Control-Allow-Headers: Content-Type, X-SM-Frame` and `Access-Control-Max-Age: 600`.

| Method and path | Purpose |
| --- | --- |
| `GET <base>/` | the render page (markup below) |
| `GET <base>/boot.json` | boot document (below) |
| `GET <base>/story.json` | the story snapshot (after defaults) |
| `GET <base>/catalog/…` | files of `ui/img/storymaker/` from the snapshot (`catalog.json` and every file it names, except the sfx WAVs) |
| `GET <base>/fonts/…` | the font files named in boot.json |
| `GET <base>/scenes/<id>.js` | code scene bundles (`text/javascript`) |
| `GET <base>/js/storymaker/host.bundle.js`, `player.bundle.js` | the bundles |
| `POST <base>/report?t=<token>` | compile report (JSON, 8 MiB at most) |
| `GET <base>/start?t=<token>` | long poll after the report: held at most 20 s, answers `{"pending":true}`, `{"start":true}` or `{"error", "message"}`; 403 `bad_token`, 404 before an accepted report |
| `POST <base>/frame?n=<n>&t=<token>` | one frame (see below) |
| `POST <base>/error?t=<token>` | a page failure (see below) |

`boot.json` carries `token` on its first GET only. `/report`, `/frame`, `/error` and `/start` require it as `t`; otherwise they answer 403 `bad_token` without any effect.

The render page is served with this CSP:

`sandbox allow-scripts; default-src 'none'; script-src <base>/js/ blob:; worker-src blob:; img-src blob: data:; font-src blob: data:; connect-src <base>/; form-action 'none'; base-uri 'none'; object-src 'none'; frame-ancestors 'none'`

It has no `style-src`. The render page markup:

```html
<!doctype html><html data-sm-mode="render" data-sm-base="BASE"><head><meta charset="utf-8"></head>
<body><canvas id="sm-stage" width="W" height="H"></canvas><script src="BASE/js/storymaker/host.bundle.js"></script></body></html>
```

### boot.json

```
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

- `fps` is the story's fps. `width` and `height` are the canvas of the aspect (1920×1080, 1080×1920 or 1080×1080). The page takes `fps`, `width` and `height` from boot.json, not from the story.
- `frame_limit` is the number of frames Go accepts, computed by Go.
- `frame_deadline_ms` is 5000: the per-frame deadline of render mode. The page uses it instead of 250 ms.
- `workers`, `chunk` and `pre_roll` are left out when they have no effect. Absent means 1, no chunking, 0. Go computes them; see "Workers" below.
- `story_url` is `<base>/story.json`; `catalog_url` is `<base>/catalog/catalog.json`; `assets_base` is `<base>/catalog/`.
- `code_bundles` maps each code scene id to `<base>/scenes/<id>.js`.
- `fonts` lists the faces of the story's font and every fallback face (`fallback`, `unicode_range`, `weight_range`, `max_bytes`). The page fetches a face only when compile needs it, and takes font URLs only from this list.

### Flow

1. The page loads boot.json, story.json, the catalog, `glyphs.json` and the bundles, then compiles, and fetches the fonts compile needs during `prepare`.
2. `POST /report?t=` validates and answers `{"start":false,"pending":true}` at once, or `{"start":false, "error":"<code>", "message":"..."}`. Go then mixes in the background. 2a. The page long-polls `GET /start?t=`; each call holds at most 20 s and answers `{"pending":true}`, `{"start":true}` or `{"error", "message"}`, and 404 before an accepted report. The 10 s no-frame watchdog starts with FFmpeg, after the mix.
3. The page sends frames in order, one at a time, and waits for each answer.
4. The last frame is answered with `done:true`.

### Frames

`POST <base>/frame?n=<n>&t=<token>` with the body of one frame. The page sends `Content-Type: text/plain`, which is CORS-safelisted, so Chrome sends no preflight. The listener also accepts the number as the header `X-SM-Frame: <n>`; the query is used when the header is absent.

The page sends `text/plain` for every transport. The listener also accepts the transport's own type, so the table gives the type of each body as well.

| Transport | Body type (also accepted) | Body |
| --- | --- | --- |
| `png` (default) | `image/png` | PNG of the canvas at its size |
| `webp` | `image/webp` | lossless WebP (`toBlob` quality 1) |
| `raw` | `application/octet-stream` | RGBA bytes, `width × height × 4` |
| `screenshot` | (Go captures the page; the page posts no frame) | captured by Go as PNG |

The body is limited to the largest frame the transport can send for the canvas (`MaxFrameBytes`). Answers:

| HTTP status | Body | Meaning |
| --- | --- | --- |
| 200 | `{"ok":true}` | frame taken, send the next |
| 200 | `{"ok":true,"done":true}` | last frame taken; the page stops with phase `done` |
| 200 | `{"restart":true}` | the encoder was replaced (hardware encoder fallback): render again from frame 0 without a new report |
| 200 or 4xx/5xx | `{"stop":true, "error"?, "message"?}` | refused; the page stops |

Refusal codes: `frame_header` (400, no number), `frame_too_large` (413), `frame_read` (400), `frame_limit` (413, past the frame count), `frame_order` (409, a frame other than the expected one), `not_running` (409), `encoder_failed` (500), `bad_token` (403).

### Page errors

`POST <base>/error?t=<token>` with `{code, message, scene_id?}`. The code is clipped to 40 characters, the message to 600 and the scene id to 60; the body is at most 16 KiB. Codes sent by the page: `readiness`, `deadline`, `code_error`, `render_failed`. The answer is `{"ok":true}`; a wrong token answers 403 and does not end the job. Any accepted `/error` ends the job: a page that reports an error does not continue.

### Report refusals

`report_too_large` (413, over 8 MiB), `report_repeated`, `bad_token` (403).

### Transports and render timing

- `storymaker.render.transport` `""` means `png`. Unknown values fall back to `png`.
- Measured frame rates on the development machine (16 CPUs, software raster, production path with the proxy and the CDP guard): png about 10 fps, webp about 4.9, screenshot about 4.1, raw about 1.6. Raw is excluded by memory and speed. The measurements are in the phase 1 spike notes; aurago-test is not measured yet.
- Render timeout: `max(render.timeout_seconds, 3 × frames ÷ measured fps)`, with measured fps png 6, webp 2.5, screenshot 2.5 and raw 1. A 15-minute video (27000 frames) gets 13500 s with png, inside the 14400 s ceiling.
- The report must arrive within 3 minutes of navigation.
- The job's overall timeout is reset to the computed value when the frame count is known.
- FFmpeg: a hardware encoder that fails is replaced once by libx264, and the page is told to restart (`restart:true`). A second failure ends the job.

### Browser

Browser resolution order: `storymaker.render.browser_path` (if set and usable), then the installed Chrome or Chromium found by `launcher.LookPath()`, then the pinned Chromium download (verified by SHA-256). A configured path that does not work is reported; it is never replaced silently.

- `ready` in the status is true only after a successful headless launch probe. A successful probe is cached for 10 minutes, a failed one for 30 seconds. The status call that starts the probe can take a few seconds.
- The launch flags: `--disable-gpu`, `--font-render-hinting=none`, `--hide-scrollbars`, and the pipe flags below. The sandbox flag `--no-sandbox` that rod adds inside containers is removed. The flag is set only by a test hook (`SetNoSandboxForTests`) on the Windows test box.
- No DevTools port. The browser is started with `--remote-debugging-pipe`. On Windows the pipe is given as `--remote-debugging-io-pipes=<read>,<write>` with inherited handles. On Unix the pipe uses the descriptors that `--remote-debugging-pipe` reads. rod talks to the browser through a pipe client (`devtools_pipe*.go`). The reason: a loopback DevTools port is reachable by every local account, and `GET /json/list` would name the render URL with its secret prefix.
- A launch that stays silent for 60 s is killed. Start-up output (the last 4 KiB) goes into the error text.

Pinned Chromium (used when nothing else is usable, and downloaded by the first export, never by a status call):

- Revision `1321438` (Chromium 128.0.6568.0), from `https://storage.googleapis.com/chromium-browser-snapshots/<prefix>/1321438/<archive>`.
- Archives and SHA-256 (from part B; the four hashes are in `browser.go`):

| Platform | Prefix and archive | Size (bytes) | SHA-256 |
| --- | --- | --- | --- |
| windows_amd64 | Win_x64 / chrome-win.zip | 267483258 | f8ddee43d6865a974fc8acfcfcb9f6a42e84aadc427482ae2fa2e6a36a413454 |
| linux_amd64 | Linux_x64 / chrome-linux.zip | 190157449 | b5fb87b55fd81eaba9325d67220585cc12c12df529bd8ae46906d43dd5699e4e |
| darwin_amd64 | Mac / chrome-mac.zip | 149328540 | b25cd79dc73ebf423b05a4a0d65f772a78e3b87d394e9a2e7d0a4cc9201bda7b |
| darwin_arm64 | Mac_Arm / chrome-mac.zip | 135605548 | 4a478b542f1f6eaa160e64aab76100501e061d3bba46f6a480897ef1103c70c5 |

- A download that receives no byte for 60 s is abandoned. The whole browser setup is bounded by `render.timeout_seconds`.
- The archive is extracted with checks: zip-slip, absolute and backslash names, entries outside the top folder, symlink escapes, and caps of 5000 files and 2 GiB.

### Egress control

The render page cannot reach the network by three independent layers. Each one alone keeps the page on the render base (tests in `egress_smoke_test.go`):

1. The page CSP (above). `connect-src <base>/`, no `navigate-to`.
2. A strict capture proxy (`StartStrictCaptureProxy` in `internal/security/capture_proxy.go`): it dials only the render origin and serves no other destination. Its header timeout is at least the report wait.
3. The CDP guard (`egress.go`), in hybrid mode:
   - Fetch interception on the page only for resource type `Document` (navigations; a handful per job). Navigations must lie below `<base>/`.
   - `Network.setBlockedURLs` with block patterns for every other request, on the page and on every worker that auto-attaches (`Target.setAutoAttach` with `waitForDebuggerOnStart`; a worker that cannot be guarded is closed).
   - `Target.setDiscoverTargets`: an extra page target (a popup) is closed.
   - When the browser has no block patterns (Chrome before 126), or when `GuardFull` is set, the guard falls back to catch-all Fetch interception, which is slower.

Notes from the measurements: a Fetch pause on a request that carries a frame costs about 35 ms, so the catch-all mode halves the rendering rate. A dedicated worker's own CDP session does not accept `Fetch.enable`; the page session covers the requests that workers make. WebSocket handshakes are outside Fetch and the block list; the CSP and the proxy stop them.

Limit: the prefix is not a secret against scene code, but a scene cannot obtain the token, because the host takes it before any worker exists. Go validates every report field it uses against its own plan.

### FFmpeg

- Frames go to FFmpeg on stdin: `-protocol_whitelist file,pipe` and `-format_whitelist` for each input.
- The audio is the mix WAV as input 1 with `-format_whitelist wav`. A film with no audio events and no music keeps `anullsrc` with `-t`. AAC 192k.
- Video: `scale=out_color_matrix=bt709:out_range=tv,format=yuv420p` plus the encoder's filter tail, `-movflags +faststart`.
- The encoder comes from `video_studio.hardware_encoding` and `hardware_device`, else libx264. FFmpeg comes from `video_studio.ffmpeg_path` or PATH (`videostudio.CheckRuntime`).

### Frame gate

After FFmpeg ends and before anything is published, `finish` probes the output with ffprobe. The probes of one check share one timeout, the larger of 120 s and the file size at 10 MiB/s, and each has an 8 MiB output cap.

- `ffprobe -v error -protocol_whitelist file -format_whitelist mov -count_packets -select_streams v:0 -show_entries stream=nb_read_packets -of csv=p=0 -i FILE` must equal the frame count.
- `ffprobe -v error -protocol_whitelist file -format_whitelist mov -select_streams <s> -show_entries stream=duration -of csv=p=0 -i FILE`, once for `v:0` and once for `a:0`: the two durations must differ by under 25 ms.

A failure is `render_failed`, and nothing is published. The order in `finish` is `ff.Wait`, stat, size, `ProbeOutput` and `CheckOutput`, phase writing, MP4, `publishExtras`, record. An empty `FFprobePath` fails the job with `ffmpeg_unavailable`.

### Workers

Go chooses the worker count and the page runs it. `workers` 1 or no `workers` keeps the serial path.

- `ChooseWorkers(transport, logicalCPUs, freeMB, width, height, chunk, cjk, codeScenes)` lives in `internal/storymaker/render/workers.go`. A transport other than `png` gets 1, because worker encoding exists only for PNG. Otherwise `clamp(floor((cpus-2)/3), 1, 6)`, further bounded by `floor((freeMB - codeMB) / (60 + 2*chunk*frameMB + (cjk?120:0)))` when `freeMB > 0`, with `frameMB = 0.6*width*height/(1920*1080)` and `codeMB = codeScenes*(cjk?120:0)`, and at least 1.
- `cjk` is true when any drawn text in the story resolves to face `sm-cjk`, or any code scene has the language base `zh` or `ja`.
- `chunk` is 8. `pre_roll` is 0 by default and 2 when on.
- Before the report the page creates `workers-1` helper engines with `shared_code_scenes` and no bundles. Each loads and prepares. A different `frame_count` or schedule is `render_failed`. This happens before the report, so it cannot trip the 10 s frame watchdog.
- `H.runPool({engines, limit, chunk, preRoll, post, status}) -> outcome` lives in `ui/js/storymaker/host-pool.js`:
  - chunk j goes to worker `j % N`, and a chunk with code-scene frames goes to worker 0
  - a worker waits while its chunk starts `>= N*C*2` frames past the next frame to post
  - one in-order poster, one POST in flight
  - PNG encoded in the worker
  - on `restart`, stop the poster, abandon the generation, clear the buffer, re-run the same engines from frame 0
  - a crash or deadline rejects, and the page posts `/error`
- `TestParallelParityBrowser` pins identical RGBA for N workers against 1 on `firewall.json` and `speech.json`, with pre-roll on and off. `TestParallelEncoderFailureBrowser` forces one encoder failure, and the job restarts from frame 0.

---

## 8. App and player iframe protocol

Owner: the player host (`ui/js/storymaker/host-app.js`) and the desktop app (`ui/js/desktop/apps/storymaker*.js`).

Every message is an object with `sm: 1` and a `type`. The parent accepts messages from the host only when `event.source === iframe.contentWindow`. The host accepts messages only from `window.parent`. Each side drops messages with a bad shape and answers a schema violation from the parent with `error {code:"bad_message"}`.

### Host to parent

| Type | Fields | Notes |
| --- | --- | --- |
| `ready` | `caps: {workers, offscreen}` | sent when the host starts; after every iframe reload, send `load` again |
| `need_assets` | `req`, `keys` (sorted) | assets the host still needs |
| `compiled` | `report` | `report` (section 6), cleaned by the parent; `audio_events` at most 40000, `captions` at most 8000 |
| `time` | `frame`, `playing` | the playhead. Sent every 90 ms while playing and on seek, pause, play and end. The first `time` after a compile means "ready to play". |
| `thumb` | `req`, `scene_id`, `bitmap` (ImageBitmap, transferred) | answer to `thumb` |
| `error` | `code`, `message` (600 characters at most), `scene_id?`, `line?`, `column?` | codes: `code_error`, `deadline`, `readiness`, `bad_message`, `unknown_name` |

### Parent to host

| Type | Fields | Notes |
| --- | --- | --- |
| `load` | `story`, `catalog` (parsed `catalog.json`; may carry `glyph_data`), `audio_info?` (section 6), `assets?` (`catalog:<path>` to parsed JSON), `fonts?` (`{name, family, weight, data: ArrayBuffer, weight_range?, unicode_range?}`), `code_bundles?` (`{<scene_id>: text}`), `format?` `{aspect, fps, width?}` | a new `load` or `reset` ends all workers |
| `assets` | `req`, `assets` (keys to values), `missing?` | answer to `need_assets` |
| `seek` | `frame` | one seek at a time; the newest wanted frame wins |
| `play`, `pause` | none | |
| `thumb` | `req`, `scene_id`, `frame?`, `width` | answered with `bitmap`; before the first draw the host answers `error {code:"readiness"}` |
| `reset` | none | |

The parent posts with targetOrigin `'*'` (the iframe origin is opaque). The host posts to the origin of `data-sm-base`.

### Asset keys and caps

- Keys: `catalog:<relative path>` (a path listed in `catalog.json`: `rig.file` or `entries[].file`), `font:<name>/<weight>`, `bundle:<scene_id>`. A key matches `^(catalog|font|bundle):[^\s]{1,230}$`.
- The parent serves only paths listed in `catalog.json` and `bundle:<code scene id>`. Unknown keys are listed in `missing`. The parent never ignores them silently.

Host caps (`StorymakerHost.CAPS`):

| Cap | Value |
| --- | --- |
| scenes in `load.story` | 400 |
| catalog entries | 6000 |
| asset keys per message | 2000 |
| fonts | 16, each at most 8 MiB unless a named glyph face allows more, 32 MiB in total, checked on `load.fonts` and on `font:` values of `assets`, with the faces and the byte count of the last accepted load |
| code bundles | 400, each at most 256 KiB of text |
| key length (in `missing`) | 240 |
| thumb width | 16 to 1920 |
| format `width` | 64 to 3840 |
| format `aspect` | `16:9`, `9:16`, `1:1` |
| format `fps` | 24, 25, 30 |
| audio scenes | 400 |
| audio blocks per scene | 61 |
| audio pieces per block | 800 |
| levels per block | 27000 |
| levels per message | 200000 |
| glyph faces | 16 |
| glyph range values | 60000 |
| fallback font | at most its `max_bytes` and at most 18 MiB |

Go enforces the two level caps first (`ProjectStore.AudioAnswer`, `MaxBlockLevels` and `MaxTotalLevels`): a block over the per-block cap, or one that would push the running total over the message cap, is sent with `levels: []` and keeps its pieces, duration and spans, so only its lip-sync and waveform are lost. A block with more than 2000 spans gets one span per used piece range.

`format.width` is the width of the preview canvas (the height follows the aspect). Without it the canvas has the export size. The desktop app uses 960 px for 16:9, 720 px for 1:1 and 540 px for 9:16.

### Readiness

The host posts `compiled` as soon as it has the report, asks for the assets it still lacks with `need_assets`, draws frame 0 and then posts `time {frame: 0}`. Thumbnails asked before that get `error {code:"readiness"}`.

### Parity recipe

`thumb` with `width` equal to the export width and an optional `frame` returns the bitmap that frame n draws on the canvas. A parent page can compare its decoded RGBA with the PNG of the render page.

---

## 9. Desktop app

Name: Storymaker. Global name `StorymakerApp`.

### Registration

- Manifest in `internal/desktop/types.go`: entry `builtin://storymaker`, category `creative`, `Requires: ["storymaker"]`, icon `storymaker`, metadata `logo_path` `/img/desktop-icons/storymaker.svg` and `open_maximized` `true`. Permissions: `files:read`, `files:write`, `notifications`. The manifest test is `internal/desktop/storymaker_app_test.go`.
- Capability `storymaker` in `internal/server/bluetooth_live.go` (`storymakerAvailable`).
- Module loader (`ui/js/desktop/core/module-loader.js`): the style `desktop-app-storymaker.css` and sixteen scripts in order: `storymaker-core.js`, `storymaker-model.js`, `storymaker-api.js`, `storymaker-assets.js`, `storymaker-player.js`, `storymaker-projects.js`, `storymaker-strip.js`, `storymaker-inspector.js`, `storymaker-code.js`, `storymaker-export.js`, `storymaker-audio.js`, `storymaker-voice.js`, `storymaker-music.js`, `storymaker-dialog.js`, `storymaker-handoff.js`, `storymaker.js`. Each file is under 1100 lines.
- Window size: default 1320×840; minimum 640×480 (`window-shell-runtime.js`).
- Dispatch: `ui/js/desktop/core/menus-and-routing.js` loads the scripts and renders through `window.StorymakerApp.render(...)`.
- Icon: `ui/img/desktop-icons/storymaker.svg`.

### Strings

- UI strings: `ui/lang/storymaker/<lang>.json`, 16 files (cs, da, de, el, en, es, fr, hi, it, ja, nl, no, pl, pt, sv, zh). Section prefix `storymaker`. They never define `storymaker.catalog.*` keys.
- Catalog names: `ui/lang/storymaker_catalog/<lang>.json`, 16 files (section 3).
- The app's own key: `desktop.app_storymaker` in `ui/lang/desktop/<lang>.json`, all 16 files.
- Config block strings: `config.storymaker.*` in `ui/lang/config/common/<lang>.json`.

### Configuration block

`ui/cfg/virtual_desktop.js` (`vdCfgStorymaker`): the toggle for `storymaker.enabled`; `storymaker.render.timeout_seconds` (60 to 14400), `storymaker.render.max_duration_seconds` (10 to 3600), `storymaker.render.browser_path`, `storymaker.render.transport` (automatic, png, webp, raw, screenshot), `storymaker.narration.max_daily_chars` (1000 to 100000000), and `storymaker.narration.max_job_chars` (100 to 10000000). The "Check export setup" action reads `GET /api/desktop/storymaker/status` for the saved settings only.

### Phase 1 screens

- Start screen: the project list (newest first), a new project (title, format 16:9, 9:16 or 1:1), delete through the desktop confirm dialog.
- Editor: toolbar with the Storyboard and Code tabs, the issues badge and Export; banners for read-only, conflict, preview unavailable and export unavailable; scene strip with thumbnails (add scene, add code scene); the stage (player iframe) with play, pause, a scrubber with one block per scene (hatched for code scenes), Home and End; a side panel with Scene and Library tabs; the issues panel.
- Scene inspector: environment, time of day, narration, entry transition (not for scene 1), duration (automatic or seconds), people and props of the scene, move up and down, delete.
- Library: People, Props and Places with search. A card adds an actor or a prop to the selected scene, or sets the environment.
- Code tab: `story.json` and every code scene in CodeMirror (the vendored bundle; a text area stays when it cannot load), with a problems list and gutter marks.
- Export: a panel with FFmpeg and browser state, the blockers, progress (phase, percent, frames), cancel, and the result path with "Show in Files".

Not in phase 1: project rename, scene duplicate, undo and redo, drag and drop, the catalog tabs Text, Transitions, Stickers and Mine, beat editing, and the agent panel.

### Phase 2 screens

- Voice panel: provider select, voice select or free-text field, language select, audition of the first stored piece (it never synthesises), the `not tested` mark, the facts of `note`, one action that writes the voice into the story, the captions toggle.
- Narration status: `chars`, `missing`, progress from `pieces_*`, the Generate button, the Stop button `data-stm="narration-cancel"`, and the panel's own text for `interrupted` and `cancelled`. Stop shows only while the narration job is queued or running, is hidden when read-only, and calls `inst.api.cancelJob(job.id)`.
- Music panel: mode, mood, prompt, file picker, Noisemaker track, volume, and the Stop button `data-stm="music-cancel"`. Generated mode shows only with `status.music_configured` and runs only from an explicit button click. Stop shows only while the music job is queued or running, is hidden when read-only, and calls `inst.api.cancelJob(job.id)`.
- Scrubber waveform: a `.stm-scrub-wave` canvas inside `.stm-scrub`, painted from `S.audioModel.waveform`, `pointer-events:none`.
- Export dialog: a native `<dialog>` in `storymaker-dialog.js` with quality, burn-in captions (preset from `style.captions`), SRT, VTT, stems and Open in Video Studio. Blockers gain `narration` and `music`.
- Finished-job handoff: `storymaker-export.js` shows stems, SRT, VTT and library paths when present and each warning as text. `storymaker-handoff.js` offers the handoff on any finished job with stems (`S.handoff.canOffer`). The export panel reads the project's job list once when the editor opens and follows the `job` events, and its start view offers the newest finished render behind `data-stm="export-last"`, so a render that ended while no window tracked it keeps its result and its handoff. It never replaces the start view by itself.

### Tests

Unit and guard tests: `ui/desktop_storymaker_*_test.go` (names, model, API, i18n, CSS, registration, guards, fixture, browser) and `ui/config_storymaker_test.go`. Server-side app tests: `internal/server/storymaker_app_browser_test.go` and `storymaker_app_states_browser_test.go`. Browser tests are gated by `AURAGO_RUN_BROWSER_SMOKE=1`.

### Documentation

- `internal/storymaker/AGENTS.md` (Go core; the render section is `internal/storymaker/render/AGENTS.md`).
- `ui/js/storymaker/AGENTS.md` (player).
- `ui/js/desktop/apps/AGENTS.md`, Storymaker section.
- Root `AGENTS.md`: child DOX index row.
- `documentation/storymaker.md`, and rows in `documentation/manuals/{en,de}/04-webui.md`.

---

## 10. Part ownership

Drafting-only; omitted.

## 11. Drafting waves and inputs

Drafting-only; omitted.

## 12. Deviations from the spec

Phase 1 deviations from the spec are recorded in the spec's decisions and deviations, and are folded into `docs/superpowers/specs/2026-11-01-storymaker-design.md` when the plan is committed.
