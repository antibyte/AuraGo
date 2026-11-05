# Execution outline

Phase 2 is one plan in parts, landed in four batches: 2A, 2B, 2C, 2D. Each batch has its own worktree and branch, starts from the main of that moment, lands alone and leaves main green.

The plan is already committed on main, before execution starts. That commit holds:
- this plan, the frozen interfaces, `decisions.md` and `code-map.md`, in `<plan>`
- the section "Changes from the phase 2 plan" (frozen section 8) in the design spec
- the evidence and the spike sources listed below

Batch 2A starts with task M0 on a branch from that main. If a listed file is missing from `<plan>`, stop and report it.

Tasks run strictly in the order of section 3. "Depends on" names the direct prerequisites only, so never reorder by it. Where a row in section 3 and the task text in a part differ, the part wins: the parts went through a dry run and a review after the rows were written. `interfaces.md` section 10 lists the decisions made after the freeze, and they win over sections 1 to 8.

`docs/` is git-ignored, so those paths need `git add -f`.

Path prefixes used in sections 2 and 3:

| Prefix | Path |
| --- | --- |
| `sm/` | `internal/storymaker/` |
| `rn/` | `internal/storymaker/render/` |
| `srv/` | `internal/server/` |
| `pl/` | `ui/js/storymaker/` |
| `app/` | `ui/js/desktop/apps/` |
| `pt/` | `scripts/storymaker-tests/` |
| `cat/` | `assets/storymaker/` |
| `parts` | `scripts/storymaker-bundle-parts.js` |
| `<plan>` | `docs/superpowers/plans/2026-11-04-storymaker-phase2/` |
| `<spikes>` | `<plan>/spikes/` |
| `<evidence>` | `<plan>/evidence/` |

These files came out of the planning session's scratchpad, so they exist only in the plan folder:

| Target | Content |
| --- | --- |
| `<evidence>/` | the 10 reports `mix.md`, `mixReal.md`, `perSentence.md`, `fonts.md`, `sfx.md`, `tts.md`, `handoff.md`, `voices.md`, `parallelWorkers.md`, `critic.md` |
| `<spikes>/sfx/set/` | the 25 WAVs, `recipe.json`, `sheet.json` |
| `<spikes>/sfx/scripts/` | `build_set.py` |
| `<spikes>/fonts/` | `cmap.mjs`, `glyphs.mjs`, `greek.mjs` |
| `<spikes>/mix/mixer/`, `<spikes>/mixreal/mixer/` | `go.mod`, `main.go`, `mixer.go`, `wav.go` |
| `<spikes>/parallel/` | `prototype.diff` |

Both mixers carry their own `go.mod`, so `go build ./...` and `go vet` skip them. Spike media and generated output stay out.

## 1 Batches

### 2A Motion (part M)

**Works after landing:**
- role eases
- hide at 0.35 s
- cue arrivals that land on the marker, with `cue_late`
- overlap at 40 %
- text holds, including Han and kana word counts
- shrink to fit with balanced lines
- log-space zoom
- `style.rhythm` in story JSON
- `wipe` as the default transition on an environment change
- the highlight beat
- the static motion lint in the issues panel

Export stays silent.

**Not done:**
- `say` and `sfx` stay minor `not_supported_yet`
- `style.auto_sfx` stays `unknown_field`
- no rhythm control in the inspector (the Code tab edits it)
- no fallback fonts, so CJK draws from host fonts without an error
- the audio block shape stays the phase 1 one

**Proof:**

```powershell
npm run test:storymaker
$env:AURAGO_RUN_BROWSER_SMOKE='1'; go test ./internal/server -run '^TestStorymakerAppRealServerBrowser$' -count=1 -timeout 15m
```

### 2B Speech (parts G, N, S, E1)

**Works after landing:**
- `glyph_missing` and the Noto and Inter fallback, loaded on need
- the voice panel, with `GET /voices`, a language select and an audition of stored pieces
- narrate jobs with their cache, envelopes and character caps
- `GET /narration` and the audio routes
- project voice snapshot and language resolution
- cue placement with `placed` and `cue_precision`
- `say` with bubbles, burned captions, lip-sync
- narration-only preview and the scrubber waveform
- the report token and report validation

**Not done:**
- export is still silent, even for a narrated story
- no `narration_incomplete` gate
- no SRT, VTT or stems
- music and `sfx` stay minor issues
- no render options

**Proof:**

```powershell
go test ./internal/storymaker/... ./internal/tools -run '(?i)(narration|envelope|glyph|elevenlabs|report)' -count=1
$env:AURAGO_RUN_BROWSER_SMOKE='1'; go test ./internal/server -run '^TestStorymakerAppRealServerBrowser$' -count=1 -timeout 15m
```

### 2C Sound (parts A, E2)

**Works after landing:**
- SFX set with takes, `sfx.typing` and `auto_sfx`
- music jobs with generated, file and track sources, generated only from an explicit click
- the Go mixer with voice match, ducking, carve, dips and the master gain
- AAC 192k, the frame gate and deferred start
- the `narration_incomplete` and `music_incomplete` gates
- stems, SRT and VTT, through the API
- the library copy
- preview with music and SFX
- the parity test with audio

**Not done:**
- no export dialog, because options are API only
- no finished-job actions for stems or subtitles
- no handoff
- render stays serial

**Proof:**

```powershell
go test ./internal/storymaker/render -run '(?i)(mix|meter|gate|deferred|subtitle)' -count=1
$env:AURAGO_RUN_BROWSER_SMOKE='1'; go test ./internal/server -run '^TestStorymakerParityAppPlayerAndRenderPage$' -count=1 -timeout 15m
```

### 2D Export (parts E3, W, I)

**Works after landing:**
- the export dialog
- finished-job actions
- Video Studio handoff and the `projectId` launch
- parallel workers with parity on CJK, RTL and bubbles, plus the forced encoder failure
- docs and the manual
- the 5.1.9 swatch check
- asset-pack acceptance

W is droppable. Dropping it:
- skips G-W-1 and keeps the serial path
- drops from I4 the "### Workers" contract text, contract line 658 and the `workers`, `chunk` and `pre_roll` boot fields
- records the drop in the spec

**Not done, and open for the user:**
- aurago-test acceptance (B13), because the box was unreachable on 2026-11-04
- listening notes and rhythm factors
- taste of the SFX set
- prosody with per-sentence synthesis
- agent voice discovery (`storymaker_catalog kind=voice`), which is phase 3

**Proof:**

```powershell
$env:AURAGO_RUN_BROWSER_SMOKE='1'; go test ./internal/server -run '^TestStorymakerAppRealServerBrowser$' -count=1 -timeout 15m
$env:AURAGO_RUN_BROWSER_SMOKE='1'; go test ./internal/storymaker/render -run '^TestParallelParityBrowser$' -count=1 -timeout 20m
```

## 2 File map

The owner columns are generated from the Files column of section 3. A file named there without an owner here is a test that goes with its task.

### Budgets

| File | Today | Rule | Note |
| --- | --- | --- | --- |
| Any player part | | under 600 lines, first line a comment (`pt/10-static-guards.mjs:24-25`) | no timers or clocks except `determinism.js` |
| `pl/beats.js` | 377 lines | under 600 | at risk. The added code runs to about 250 lines, so pure placement helpers (beat class, typing keys, auto-SFX events) go into `pl/motion.js` |
| `pl/ranges.js` | new | under 600 | generated. The generator packs many ranges per line |
| `app/storymaker.js` | 938 lines | under 1100 (`ui/desktop_storymaker_guards_test.go:75-77`) | wiring only, every panel is a module |

### Files owned per batch

`validate.go`, `plan.js` and `host-render.js` have exactly one owner part per batch:

| File | 2A | 2B | 2C | 2D |
| --- | --- | --- | --- | --- |
| `sm/validate.go` | M | G | A | none |
| `pl/plan.js` | M | S | none | none |
| `pl/host-render.js` | none | N | A | W |

The app strings in `ui/lang/storymaker/*.json` belong to the last strings task of each batch: M15, E1i, E2e, E3g. Two locale sets are outside that rule and travel with their own tasks: the generated catalog names in `ui/lang/storymaker_catalog/*.json` (S5, A3) and the config labels in `ui/lang/config/common/*.json` (N8).

### Go

| File | Responsibility | Owner per batch |
| --- | --- | --- |
| `sm/model.go` | `Style` fields, issue codes, beat sets | 2A M, 2B G, 2C A |
| `sm/decode.go` | field hints for `rhythm`, `auto_sfx` | 2A M, 2C A |
| `sm/defaults.go` | default transition | 2A M |
| `sm/validate.go` | style, music, say, speaker, glyph checks | 2A M, 2B G, 2C A |
| `sm/catalog.go` | glyph hook, sfx fields, `VerifyFiles` | 2B G, 2C A |
| `sm/project.go`, `sm/memstore.go` | `Store.Open`, `CreateWithVoice` | 2B N |
| `sm/narration.go` (new) | `IsJSSpace` (G3), then blocks, pieces, keys, audio answer | 2B G and N, 2C A |
| `sm/audio_decode.go` (new) | WAV and MP3 decoding, sniff, probe | 2B N |
| `sm/envelope.go` (new) | envelope, `ParseEnvelope`, `AssembleBlock` | 2B N |
| `sm/glyphs.go` (new) | coverage reader | 2B G |
| `sm/voices.go` (new) | voice resolution, caption override | 2B N |
| `sm/music.go` (new) | moods, prompt, keys, volume, `MusicMeta`, `WriteMusic`, `WriteMusicMeta`, `MusicInfo` | 2C A |
| `sm/ranges_gen.go` (new, generated) | Go tables | 2A M |
| `sm/storymakertest/seed.go` (new) | seeded audio for fixtures | 2C A |
| `sm/testdata/narration/cross_language.json` (new) | Go and Node fixture | 2B N |
| `sm/testdata/narration/one.mp3` (new, small) | MP3 decoder fixture | 2B N |
| `sm/testdata/stories/speech.json` (new) | CJK, RTL, Greek, Hindi, bubbles story | 2B S, 2D W |
| `sm/testdata/catalog-mini/catalog.json` | fixture styles and sfx entries | 2B S, 2C A |
| `sm/*_test.go` | new: `narration_test.go`, `audio_decode_test.go`, `envelope_test.go`, `glyphs_test.go`, `voices_test.go`, `music_test.go` | with their tasks |
| `rn/jobs.go` | kinds, coalesce, pieces, warnings, output | 2B N, 2C A |
| `rn/listener.go` | token, cap, fonts, boot, `/start` | 2B N, 2C A, 2D W |
| `rn/report.go` | report struct, S2 validation | 2B N, 2C A |
| `rn/render.go` | `Plan`, `Prepare`, `Deps`, `Settings`, snapshot, deferred start, `finish` | 2B N, 2C A, 2D W |
| `rn/ffmpeg.go` | audio input, AAC 192k, no `-shortest`, message scrub | 2C A |
| `rn/mix.go`, `rn/mixinput.go`, `rn/meter.go`, `rn/gate.go`, `rn/subtitles.go` (new) | mix, inputs, meter, frame gate, SRT and VTT | 2C A |
| `rn/options.go` (new; file name decided here) | `RenderOptions` | 2C A |
| `rn/workers.go` (new; file name decided here) | `ChooseWorkers` | 2D W |
| `rn/testdata/report_measured.json` (new) | report fixture shared by Node and Go | 2B S |
| `rn/*_test.go` | existing tests plus new ones per file | with their tasks |
| `srv/storymaker_service.go` | second manager, router, usage, settings, `WorkspaceFree`, library wiring | 2B N, 2C A, 2D W |
| `srv/storymaker_handlers.go` | dispatch, jobs, events, projects, render, status | 2B N, 2C A |
| `srv/storymaker_audio_handlers.go` (new) | voices, narration, narrate, music, audio routes, music key | 2B N, 2C A |
| `srv/storymaker_narrate.go` (new) | narrate and music job runner | 2B N, 2C A |
| `srv/storymaker_store.go` | `Open`, library wiring | 2B N, 2C A |
| `srv/server.go` | test seams beside `storymakerRunOverride` (`srv/server.go:376`) | 2B N, 2C A |
| `srv/storymaker_*_test.go` | API, config, e2e, parity, tour | with their tasks |
| `internal/config/storymaker.go`, `internal/config/storymaker_test.go`, `config_template.yaml` | narration caps | 2B N |
| `internal/tools/tts.go`, `internal/tools/tts_cloud.go`, tests | ElevenLabs fix, OpenAI list | 2B N |
| `internal/desktop/service_media.go`, new test | `WriteStorymakerVideo` | 2C A |
| `internal/videostudio/encoder_export.go` | `ResolveFFprobe` | 2C A |

### Player

| File | Responsibility | Owner per batch |
| --- | --- | --- |
| `parts` | bundle part order of frozen 4.1 | 2A M, 2B G and S, 2D W |
| `pl/ease.js`, `pl/timeline.js`, `pl/camera.js` | internal curves, log zoom, blend | 2A M |
| `pl/seed.js`, `pl/ranges.js` (new) | seeds, tables | 2A M |
| `pl/motion.js` (new) | motion rules, placement helpers | 2A M, 2C A |
| `pl/lint.js` (new) | lint | 2A M, 2B S |
| `pl/glyphs.js` (new) | coverage in JS | 2B G |
| `pl/cuemap.js`, `pl/captions.js`, `pl/lipsync.js`, `pl/bubble.js` (new) | cue map, captions, mouths, bubbles | 2B S |
| `pl/host-pool.js` (new) | parallel scheduler | 2D W |
| `pl/beats.js` | kinds, scheduler, log, say, sfx | 2A M, 2B S, 2C A |
| `pl/compile.js` | auto length, lint, report fields, film end, needs, `lastTake` | 2A M, 2B S, 2C A |
| `pl/plan.js` | blocks by id, font chain | 2A M, 2B S |
| `pl/narration.js` | `parse.words` through `ranges` | 2B N |
| `pl/text.js` | balance, shrink, memo; family list, CJK wrap, direction | 2A M, 2B G |
| `pl/scene.js` | highlight intervals; bubble, speech, mouth; sfx index | 2A M, 2B S, 2C A |
| `pl/render.js` | highlight, memo input; captions, bubble, mouth, chain | 2A M, 2B S |
| `pl/codescene.js` | seed delegate; `facesForLanguage` | 2A M, 2B S |
| `pl/worker-player.js` | FontFace descriptors; PNG blob | 2B G, 2D W |
| `pl/worker-scene.js` | FontFace descriptors; `fontChain` | 2B G and S |
| `pl/host-core.js` | font descriptor; helper engines | 2B G and S, 2D W |
| `pl/host-app.js` | `checkMessage(msg, loaded)`, caps for fonts and audio info | 2B G and E1 |
| `pl/host-render.js` | token, lazy fonts; `/start`; parallel path | 2B N, 2C A, 2D W |
| `ui/js/storymaker/host.bundle.js`, `ui/js/storymaker/player.bundle.js` | generated, rebuilt in every task that edits a part | every player task |
| `scripts/build-storymaker-ranges.mjs` (new), `scripts/storymaker-ucd-16.0.0.txt` (new), `package.json` | generator, committed Unicode data, npm scripts | 2A M |
| `pt/*.mjs` | existing plus new 12 to 22 (the runner reads the folder) | with their tasks |

### App

| File | Responsibility | Owner per batch |
| --- | --- | --- |
| `app/storymaker-api.js` | new calls, URL builders, font buffer limit | 2B G and E1, 2C E2 |
| `app/storymaker-assets.js` | `glyph_data`, fallback face fonts | 2B G |
| `app/storymaker-player.js` | `compiled` whitelist | 2B E1 |
| `app/storymaker-audio.js` (new) | preview audio, waveform model | 2B E1, 2C E2 |
| `app/storymaker-voice.js` (new) | voice panel, narration status, captions toggle | 2B E1 |
| `app/storymaker-music.js` (new) | music panel | 2C E2 |
| `app/storymaker-dialog.js` (new) | export dialog | 2D E3 |
| `app/storymaker-handoff.js` (new) | tracks and handoff | 2D E3 |
| `app/storymaker-export.js` | job kind filter; errors, silent note; dialog hook, actions | 2B E1, 2C E2, 2D E3 |
| `app/storymaker-strip.js` | scrubber waveform | 2B E1 |
| `app/storymaker-projects.js` | language on create | 2B E1 |
| `app/storymaker-core.js` | `ISSUE_CODES`, `JOB_ERROR_CODES`, error `data`, `describeError` | 2A M, 2B E1, 2C E2 |
| `app/storymaker.js` | wiring only | 2B E1, 2C E2, 2D E3 |
| `app/video-studio.js` | `openProject`, `ctx.projectId` | 2D E3 |
| `ui/css/desktop-app-storymaker.css` | panel styles | 2B E1, 2C E2, 2D E3 |
| `ui/cfg/virtual_desktop.js` | narration cap fields | 2B N |
| `ui/lang/storymaker/*.json` (16) | app strings | 2A M, 2B E1, 2C E2, 2D E3 |
| `ui/lang/storymaker_catalog/*.json` (16) | generated catalog names | 2B S, 2C A |
| `ui/lang/config/common/*.json` (16) | config labels | 2B N |
| `ui/desktop_storymaker_*_test.go`, `ui/config_storymaker_test.go`, `ui/storymaker_catalog_test.go`, `ui/storymaker_player_gate_browser_test.go` | guards and fixtures; new `ui/desktop_storymaker_audio_test.go`, `ui/desktop_storymaker_handoff_test.go` | with their tasks |

### Catalog and fonts

| File | Responsibility | Owner per batch |
| --- | --- | --- |
| `cat/lib/catalog.mjs` | glyph manifest key; speech styles; raw WAV hash; sfx entries | 2B G and S, 2C A |
| `cat/lib/names.mjs` | names of speech styles and sfx | 2B S, 2C A |
| `cat/lib/validate.mjs` | glyph file; sfx entries | 2B G, 2C A |
| `cat/lib/glyphs.mjs`, `cat/test/glyphs.test.mjs` (new) | cmap reader and test | 2B G |
| `cat/test/catalog.test.mjs` | file count (line 24 pins 105), Buffer values (lines 20, 23), entry count (line 78 pins 76) | 2B G and S, 2C A |
| `cat/test/sfx.test.mjs` (new) | WAV format and sha256 | 2C A |
| `cat/sfx/*.wav` (25, new), `cat/README.md` | source WAVs and provenance | 2C A |
| `scripts/build-storymaker-catalog.mjs` | glyph file, raw WAV copy | 2B G, 2C A |
| `ui/img/storymaker/**` | generated: `glyphs.json`, `catalog.json`, thumbnails, `sfx/*.wav` | 2B G and S, 2C A |
| `ui/fonts/Inter-LICENSE.txt` (new), `ui/fonts/qr/AGENTS.md` | OFL text, Storymaker note | 2B G |
| `licences/THIRD_PARTY_LIBRARIES.md` | Noto rows; sample sources | 2B G, 2C A |
| `.gitattributes` | `*.wav binary` | 2C A |

### Docs

Each contract slice matches its lines by the quoted text, not by the number, because earlier slices shift lines.

| File | Responsibility | Owner per batch |
| --- | --- | --- |
| `docs/superpowers/specs/2026-11-01-storymaker-contract.md` | frozen section 7, sliced per batch | 2A M16, 2B E1j, 2C E2f, 2D I4 |
| `docs/superpowers/specs/2026-11-01-storymaker-design.md` | frozen section 8; To verify results | planning commit, 2D I4 |
| `sm/AGENTS.md` | child doc | 2A M16, 2B G2 and E1j, 2C E2f, 2D I5 |
| `pl/AGENTS.md` | child doc | 2A M16, 2B E1j, 2D I5 |
| `rn/AGENTS.md`, `app/AGENTS.md`, `srv/AGENTS.md` | child docs | 2B E1j, 2C E2f, 2D I5 |
| `internal/desktop/AGENTS.md` | child doc | 2C E2f, 2D I5 |
| `AGENTS.md` (root) | Storymaker row at line 212 | 2D I5 |
| `documentation/storymaker.md` | user page, including the phase 1 project changes of frozen section 8 | 2A M16, 2B E1j, 2C E2f, 2D I5 |
| `documentation/manuals/en/04-webui.md`, `documentation/manuals/de/04-webui.md` | manual rows (phase 1 touched both) | 2D I5 |

### Dirty files in the main checkout

Other sessions hold uncommitted edits in these files. Git status at the start of planning also shows root `AGENTS.md` and `ui/lang/desktop/*.json` dirty.

| File | 2A | 2B | 2C | 2D |
| --- | --- | --- | --- | --- |
| `ui/js/desktop/core/menus-and-routing.js` | not edited | not edited | not edited | E3f edits the Video Studio launch (line 1649) |
| `ui/js/desktop/core/module-loader.js` | not edited | E1c, E1e add `audio`, `voice` (line 35) | E2b adds `music` | E3a, E3d add `dialog`, `handoff` |
| `ui/js/desktop/core/window-shell-runtime.js` | not edited | not edited | not edited | E3f edits the reuse branch (line 1615) |
| `ui/js/desktop/core/desktop-foundation.js` | not edited | not edited | not edited | not edited |
| `ui/js/desktop/bundles/main.bundle.js` | no rebuild | no rebuild | no rebuild | rebuilt at merge |
| `internal/desktop/types.go` | not edited | not edited | not edited | not edited |
| `AGENTS.md` (root) | not edited | not edited | not edited | I5 edits line 212 |
| `ui/lang/desktop/*.json` | not edited | not edited | not edited | not edited |

`module-loader.js` is not a part of `main.bundle.js` (`ui/desktop.html:131` loads it on its own), so 2B and 2C leave that bundle unchanged. Their only overlap with the main checkout is `module-loader.js`.

**Merge handling:**
- 2A overlaps none of these files, so it lands by the plain rule in section 5.
- In 2D the branch does not commit `main.bundle.js` task by task. E3f rebuilds it and leaves it uncommitted in the worktree, because I2 needs the rebuilt copy, and every 2D commit stages named paths only. The last commit of the batch, made after the rebase on main, is the output of `npm run build:ui`.
- If the overlap check shows any file of the branch dirty in the main checkout, not only those in this table, `merge --ff-only` refuses. Do this instead:
  1. Wait for the other session with a poll on main's SHA and the overlap count.
  2. Rebase after each move of main and rebuild.
  3. If the wait runs long, ask the user. Hand landings around another session's edits need the user's OK.
- Never commit, stash or restore another session's edits.

## 3 Task table

Each row is one commit.

- "Wire" in a title means the task changes both sides of a Go and JS boundary and tests both sides.
- `npm run build:ui` rebuilds the two player bundles in every task that edits a player part.
- A new app module joins `module-loader.js` and `storymakerModules` (`ui/desktop_storymaker_registration_test.go:11`) in the same commit, because `TestStorymakerLoaderListsEveryScript` compares them to the disk.
- Numbers in parentheses after a test file are its lines that the task edits.
- Every commit stages named paths only.

**Allowed red states on the branch:**
- From M7 to M10, `npm run test:storymaker` is red on the pins that M11 lists.
- Until the batch's strings task, the i18n guards are red on the new keys.
- From E3f to the last commit of 2D, the committed `main.bundle.js` is older than its parts. `npm run check:ui` passes only on the worktree's rebuilt copy, and a clean checkout of those commits fails it.
- A batch never lands red.

### Settled: `with:"prev"` has no floor

Frozen 4.6 step 7 exempts rule b from the floor, because spec line 346 says a `with:"prev"` beat "still runs together with the previous beat", and `pt/04-compile.mjs:68` pins that. In `firewall.json` s2 the point gesture runs `with:"prev"` on the walking actor, so this fixes every firewall pin. Hand calculation with the mini catalog (walk speed 12, walk 1.712 s from 0.5 s): gesture start 0.5 s, s2 92 frames, s2 range 114..206, s3 range 191..341, `frame_count` (`api.duration(5)`) 341, gate test (`api.duration(4)`) 311. M11 does not paste these numbers without their derivation.

### Batch 2A: part M

#### G-M-1 Story fields and shared tables

| ID | Title | Batch | Files | Depends on | Failing test |
| --- | --- | --- | --- | --- | --- |
| M0 | Make the refusal test reliable. It fails on main already, so every matrix would start red | 2A | `rn/render_test.go` | none | `TestRunRefusesReportsThatDisagreeWithTheJob` fails on most runs |
| M1 | `style.rhythm` in Go | 2A | `sm/model.go`, `sm/validate.go`, `sm/decode.go`, `sm/validate_test.go`, `sm/decode_test.go` | none | `rhythm:"fast"` gives `invalid_value` at `/style/rhythm`, and a canonical save without it writes no `rhythm` key |
| M2 | Default transition by environment (D11), comparing before `sm/defaults.go:78` overwrites `prevEnv` | 2A | `sm/defaults.go`, `sm/defaults_test.go` (30 becomes `wipe`; 35 stays `fade`, because s3 inherits `env.office`) | none | a new environment gets `wipe` in `defaults_applied`, the same environment `fade`, the first scene `cut`, a stored `fade` stays |
| M3 | Range tables from one generator. Wire: Go+JS | 2A | `scripts/build-storymaker-ranges.mjs`, `scripts/storymaker-ucd-16.0.0.txt`, `pl/ranges.js`, `sm/ranges_gen.go`, `parts`, `package.json`, `pt/12-ranges.mjs` | none | `canBreakAfter` is false inside a surrogate pair, before a mark and after U+094D; `wordCount` of 4 Han is 2; `dirOf` of an Arabic text is `rtl`; both outputs name Unicode 16.0.0; `--check` fails on one changed byte |
| M4 | `seed.js`, code scenes delegate to it | 2A | `pl/seed.js`, `pl/codescene.js`, `parts`, `pt/06-codescene.mjs` | none | `takeIndex` never returns `last` when `count>1`, and code-scene output stays byte-identical |
| M5 | Internal eases, log zoom, eased pose blend | 2A | `pl/ease.js`, `pl/timeline.js`, `pl/camera.js`, `pt/01-ease-timeline.mjs`, `pt/02-stage-parts-text.mjs` (104: `near(mid.zoom, Math.SQRT2, 1e-9)`) | none | `isEase('spring')` is false, `spring(1)` is 1, zoom 1 to 4 at s=0.5 is 2, `blendAt` returns the eased u |

#### G-M-2 Scheduling

| ID | Title | Batch | Files | Depends on | Failing test |
| --- | --- | --- | --- | --- | --- |
| M6 | `motion.js`: factors, defaults, lead fractions, holds | 2A | `pl/motion.js`, `parts`, `pt/13-motion.mjs` | M3, M5 | each lead fraction of frozen 4.5 matches a numeric search to 0.001, and a 4-character Han text has base hold 1.1 s |
| M7 | Role eases and scaled defaults (frozen 4.4, 4.7) | 2A | `pl/beats.js`, `pl/motion.js`, `pt/13-motion.mjs` | M6 | table check of every default ease; explicit `ease` wins; hide 0.35 s; lively walk times 0.8; explicit `duration` unscaled; a calm `wait` of 2 s stays 2 s and a default `wait` stays 1 s; pop opacity never above 1 |
| M8 | Scheduler: targets, floors, overlap, cursor, anchor (frozen 4.6 steps 1-4, 6b-6d, 7, 10-13) | 2A | `pl/beats.js`, `pt/13-motion.mjs` | M7 | a beat on another target starts at 0.4 of the anchor, a same-target beat follows, a `wait` breaks overlap, targets compare as instances, a short `with:"prev"` beat on another target leaves the anchor on the longer beat, and the firewall s2 gesture starts at 0.5 s, together with the walk on the same actor |
| M9 | Cue lead and `cue_late` (steps 5, 6a, 7), sfx impact 0 | 2A | `pl/beats.js`, `pl/motion.js`, `sm/model.go`, `pt/13-motion.mjs` | M8 | table of enter, walk, move, show fade, show slide, show pop, camera focus and zoom with a target, each with and without delay: the beat lands on marker plus delay within 1 ms; a clamp at 0 does not warn; a busy target gives `cue_late` at `<beat>/cue` |
| M10 | Holds, beat log, scene length | 2A | `pl/beats.js`, `pl/compile.js` (the default `built` at line 53 gains `blocksEnd: 0, holdsEnd: 0`), `pt/13-motion.mjs` | M9 | an auto scene with a 4-word title lasts landing plus 1.7 s; a secondary line holds 1.2 s; an explicit duration is never extended; lively keeps the base hold; a code scene without a usable duration gets a number, not NaN |
| M11 | Re-pin the firewall timing. Derive each number in a comment from the rules, never paste a run's output | 2A | `pt/04-compile.mjs` (73), `pt/07-workers.mjs` (43), `pt/08-host-app.mjs` (64, 137, 156), `pt/09-host-render.mjs` (94, 97), `ui/storymaker_player_gate_browser_test.go` (278-282, 634-635, 650) | M7-M10 | `npm run test:storymaker` is red only on the firewall-derived pins before and fully green after; the four tests of `ui/storymaker_player_gate_browser_test.go` pass once with `AURAGO_RUN_BROWSER_SMOKE=1`, because no matrix runs them |

Pins that stay and need no edit in M11:
- `pt/04-compile.mjs` 15, 17, 19-20, 25, 29-42, 47, 49, 51, 55, 58, 67, 68, 69, 71, 74-88, 94, 128, 155-163. Line 128 lists transition names, not timing. Line 68 stays because rule b has no floor.
- `pt/05-render.mjs` 27, 43-50, 62. The file has 85 lines, and the code map's line 100 does not exist.
- `pt/07-workers.mjs` 50, 104.
- `pt/08-host-app.mjs` 269-270. Lines 227, 242-250, 262 and 264 change in M11: they pinned frames from the old s2 length, and a no-workers or failed-build code scene has no built duration, so that story runs 296 frames.
- `pt/09-host-render.mjs` 113-127, 145-153. These run `minimal.json`, which stays 42 frames.
- `pt/11-real-catalog.mjs` 335-337, 339-341.
- `ui/storymaker_player_gate_browser_test.go` 373, which also runs `minimal.json`.

#### G-M-3 Highlight, shrink, lint, close

| ID | Title | Batch | Files | Depends on | Failing test |
| --- | --- | --- | --- | --- | --- |
| M12 | Highlight beat. Wire: Go beat set and player | 2A | `sm/model.go` (`phase1Beats`, line 423), `sm/validate_test.go`, `pl/beats.js`, `pl/scene.js`, `pl/render.js`, `pl/plan.js` (the dead `SUPPORTED_BEATS` list), `pt/04-compile.mjs` (105, 112), `pt/05-render.mjs` | M8 | pulse scale peaks at 1.08, and on a text `drawText` applies it as a canvas transform around the anchor; outline strokes 6 design pixels in `$accent`; arrow draws above the entity; Go reports no `not_supported_yet` |
| M13 | Shrink to fit, balance and the layout memo (D23) | 2A | `pl/text.js`, `pl/render.js`, `pt/02-stage-parts-text.mjs`, `pt/05-render.mjs` | M3 | an over-long title steps by 0.94 to its floor, reports `shrunk`, and `overflow` only when still truncated; balance never adds a line; a sliding text renders the same at its frames in both orders |
| M14 | Static lint and its Go codes | 2A | `pl/lint.js`, `pl/compile.js`, `parts`, `sm/model.go`, `pt/14-lint.mjs`, `pt/11-real-catalog.mjs` | M10, M12, M13 | one fixture fires each of `line_unread`, `dead_stretch`, `first_frame_empty`, `text_too_small`, `fade_between_worlds`, `entry_monotony`, `transition_variety`; a clean story fires none; title, caption and label clear 2.6 % (spec line 858) |
| M15 | 2A strings: `ISSUE_CODES`, pinned list, 16 locales | 2A | `app/storymaker-core.js` (line 319), `ui/desktop_storymaker_guards_test.go` (line 161), `ui/lang/storymaker/*.json` | M14 | `TestStorymakerAppUsesOnlyKeysItHas` with the new pin fails until the keys exist; the i18n guards pass in 16 locales |
| M16 | Contract slice, child docs, lint rates | 2A | contract header (lines 1, 5), section 2 (line 29 rhythm only, after line 64 the rhythm sentence, lines 81-93 heading and highlight, line 111, line 165, issue rows for the lint codes and `cue_late`), section 4 (line 317 with 27 codes), section 6 (lines 553-554 with `seed`, `ranges`, `motion`, `lint`; line 556; line 613; line 615 without dialogue; line 618 without say and the sfx pattern; line 619 without say and impact); `pl/AGENTS.md`, `sm/AGENTS.md`, `documentation/storymaker.md`; rate of each lint code and of `cue_late` on the fixtures recorded in the plan's Measured list (spec lines 845, 852) | M15 | no failing test (docs); a `node -e` check that every cited path exists |

### Batch 2B: parts G, N, S, E1

#### G-G-1 Coverage data

| ID | Title | Batch | Files | Depends on | Failing test |
| --- | --- | --- | --- | --- | --- |
| G1 | Coverage build, ported from `<spikes>/fonts/cmap.mjs` and `glyphs.mjs` | 2B | `cat/lib/glyphs.mjs`, `cat/lib/catalog.mjs`, `cat/lib/validate.mjs`, `scripts/build-storymaker-catalog.mjs` (`listExisting` reads `glyphs.json`), `ui/img/storymaker/glyphs.json`, `ui/img/storymaker/catalog.json`, `cat/test/glyphs.test.mjs`, `cat/test/catalog.test.mjs` (24: 106), `ui/storymaker_catalog_test.go` (`referenced` at line 777 gains the manifest's `glyphs.file`) | M3 | decoded `sm-greek` covers U+03C9, `geist` excludes U+0370-03FF, U+2026 is covered, `--check` fails when `glyphs.json` differs |
| G2 | Font notices and QR note | 2B | `ui/fonts/Inter-LICENSE.txt`, `licences/THIRD_PARTY_LIBRARIES.md` (Inter at line 257, new Noto rows), `ui/fonts/qr/AGENTS.md`, `sm/AGENTS.md`, `ui/storymaker_catalog_test.go` | G1 | a catalog test fails until the Inter licence exists and each glyph face file has a notice row |
| G3 | Go coverage reader | 2B | `sm/glyphs.go`, `sm/narration.go` (`IsJSSpace` only; N1 fills the file), `sm/catalog.go`, `sm/glyphs_test.go`, `sm/catalog_real_test.go` | G1, M3 | `LoadGlyphs` refuses non-monotonic ranges and over 1 MiB; `FaceOf(U+03C9)` is `sm-greek`; `Covered(U+2028)` is true; the mini catalog gives nil and skips the check |

#### G-G-2 Validation and text flow

| ID | Title | Batch | Files | Depends on | Failing test |
| --- | --- | --- | --- | --- | --- |
| G4 | `glyph_missing` in `Validate` | 2B | `sm/validate.go`, `sm/model.go`, `sm/glyphs.go`, `sm/validate_test.go` | G3 | Thai show text gives `glyph_missing` at its path naming `U+0E01`; U+200D and U+2028 are exempt; narration is checked only with captions on or a say without text, and never in a silent scene; code-scene strings stay unchecked |
| G5 | Say and speaker validation | 2B | `sm/validate.go` (line 456), `sm/model.go`, `sm/validate_test.go` | G4 | a say without text whose `who` differs from the speaker gives `invalid_value` at `<beat>/who`; in a silent scene it is a warning at `<beat>/text`; a say over 1500 characters gives `limit_exceeded` at `<beat>/text`; an off-stage actor's warning says the speech still plays; a cast speaker gives no `not_supported_yet` |
| G6 | `glyphs.js` and text flow (D26) | 2B | `pl/glyphs.js`, `pl/text.js`, `parts`, `pt/15-glyphs.mjs`, `pt/02-stage-parts-text.mjs` | M3, G1 | `breakWord` never splits a surrogate pair or after U+094D; no line starts with `。`; `ctx.direction` is set before measuring; `fontString` quotes each family |
| G7 | Fallback faces in host and app. Wire: app to host message | 2B | `pl/host-core.js`, `pl/worker-player.js`, `pl/worker-scene.js`, `pl/host-app.js` (`checkMessage(msg, loaded)`), `app/storymaker-assets.js` (line 48), `app/storymaker-api.js` (`fontBuffer` limit), `pt/07-workers.mjs`, `pt/08-host-app.mjs`, `ui/desktop_storymaker_api_test.go` | G1 | `checkMessage` refuses a 9 MiB font named `default` and accepts `sm-cjk` with `max_bytes`, in `load` and in `assets`, with the faces of the last accepted load; `loaded.fontBytes` holds the 32 MiB total across both; `reset` clears it; FontFace receives `unicodeRange` and the weight range |

#### G-N-1 Narration core

| ID | Title | Batch | Files | Depends on | Failing test |
| --- | --- | --- | --- | --- | --- |
| N1 | Blocks and markers. Wire: shared fixture read by Go and Node | 2B | `sm/narration.go`, `pl/narration.js`, `sm/testdata/narration/cross_language.json`, `sm/narration_test.go`, `pt/16-narration.mjs` | M3, G3 | rows with U+00A0 and U+3000 give the same `plain`, cues, width and words in both languages; a say in a silent scene and a say whose actor is off stage still make a block; a say of only spaces or only a marker makes none; `Voice.Provider` is lower-cased in the block |
| N2 | Pieces, chunk limits, keys | 2B | `sm/narration.go`, `sm/testdata/narration/cross_language.json`, `sm/narration_test.go` | N1 | fixture `pieces` match; `Chars` sums to `UTF16Len`; "Genau." merges; a Han sentence does not merge up to the limit; `Google` splits at 200; `PieceKey` ignores provider and language case |
| N3 | Audio decoder | 2B | `sm/audio_decode.go`, `sm/audio_decode_test.go`, `sm/testdata/narration/one.mp3` | none | table: s16, s24, f32, stereo, 0xFFFE, data size 0 and 0xFFFFFFFF, oversized data length; a rate of 3000 Hz, 3 channels and a wrong block align are refused; above 600 s is an error on WAV and MP3; garbage MP3 returns an error instead of a panic; the format comes from bytes; `ProbeAudio` reads a duration without keeping samples |
| N4 | Envelope and block assembly | 2B | `sm/envelope.go`, `sm/envelope_test.go` | N2, N3 | -20 dBFS gives the pinned byte; a 90 ms gap bridges and 110 ms does not; `at_ms` adds pauses; frame count is the ceil; a silent piece sets `speech_ok` false; `Levels` marshals as an array; 20 s at 22050 Hz gives the window count of the floor rule; `ParseEnvelope` refuses `duration_ms` 2000000000 and a wrong level count |

#### G-N-2 Storage, voices, config

| ID | Title | Batch | Files | Depends on | Failing test |
| --- | --- | --- | --- | --- | --- |
| N5 | `Store.Open` and the audio store | 2B | `sm/project.go` (lines 44-50), `sm/memstore.go`, `srv/storymaker_store.go`, `sm/narration.go`, `sm/project_test.go` | N4 | an audio file without an envelope and one with an invalid `env.json` count as missing; `WriteEnvelope` fills it with no provider call; `PieceFiles` skips `.aurago-write-*`; `Missing` and `Narration` are never null; a nil resolver and a key of `""` leave `Music` null; `ErrMusicSourceMissing` gives `music_issue` and keeps the narration |
| N6 | Voice resolution and `CreateWithVoice` | 2B | `sm/voices.go`, `sm/project.go`, `sm/voices_test.go` | N5 | the three resolution steps follow the list order, so a paid provider never wins over a free one; `ErrNoVoiceForLanguage` with the union without `*`; sanotts refused for `hi`; `LocalVoiceProvider("Phonon")` is true; `ApplyCaptionOverride` sets `style.captions` only when `burnIn` is not nil; `Create` still gives `voice:null` |
| N7 | ElevenLabs fix and OpenAI voice list | 2B | `internal/tools/tts.go` (lines 485-488), `internal/tools/tts_cloud.go`, a new test beside them | none | a voice id with `/` is escaped in the path; a text with `"` and `ü` round-trips as JSON; the list has 13 voices |
| N8 | Narration caps in config, with config page labels in 16 locales | 2B | `internal/config/storymaker.go`, `internal/config/storymaker_test.go`, `config_template.yaml` (block at line 2369), `ui/cfg/virtual_desktop.js`, `ui/lang/config/common/*.json`, `ui/config_storymaker_test.go` (lines 15-19), `srv/storymaker_config_test.go` | none | defaults 100000 and 20000; 999 daily chars refused; `TestConfigStorymakerTranslations` fails until the keys exist in 16 locales |

#### G-N-3 Jobs

| ID | Title | Batch | Files | Depends on | Failing test |
| --- | --- | --- | --- | --- | --- |
| N9 | Manager kinds, coalesce, piece progress | 2B | `rn/jobs.go` (messages at lines 272, 542, 604, 673, 675), `rn/jobs_test.go` | none | two `Coalesce` enqueues give one job and replayed true; `load` keeps the manager's kinds only; `Workers:2` is refused; a narrate job has no `output` key; the five messages say "this job" |
| N10 | Two managers and the router. `storymakerAudioRun` is a stub that fails the job until N11 | 2B | `srv/storymaker_service.go`, `srv/storymaker_handlers.go`, `srv/storymaker_narrate.go`, `srv/server.go` (`storymakerAudioRunOverride`), `srv/storymaker_api_test.go` | N9 | `GET /jobs/{id}` finds a narrate job; the project list merges newest first; SSE delivers both queues; project DELETE cancels both; one running render with six newer narrate jobs stays in the first SSE list; a failing second `NewManager` closes the first |
| N11 | TTS config and the narrate loop. Test seam: `storymakerSynthOverride` on `Server` | 2B | `srv/storymaker_narrate.go`, `srv/storymaker_service.go`, `srv/server.go`, `srv/storymaker_narrate_test.go` | N5, N10 | a sentence deleted during a run is not synthesised; read-only set mid-job cancels a stub blocked on ctx, writes no file and fails with `narration_unavailable`; Storymaker switched off fails with `storymaker_disabled`; a voice set to null mid-run fails with `voice_required`; undecodable audio gives `narration_failed` naming the provider; low disk gives `narration_failed` with the free size; a stored piece that no longer decodes is removed and synthesised again; a rerun skips stored pieces |
| N12 | Character caps and daily usage | 2B | `srv/storymaker_narrate.go`, `srv/storymaker_service.go` (`usage`), `srv/storymaker_narrate_test.go` | N8, N11 | a cloud job over the cap fails with `budget_exceeded`; `Phonon` in any case is exempt; usage is written before the call and subtracted only on a provider error with no bytes; the job cap counts characters already sent; an unreadable `usage.json` becomes `usage.bad.json` and stops cloud calls for that date; usage resets on a new local date |

#### G-N-4 Routes

| ID | Title | Batch | Files | Depends on | Failing test |
| --- | --- | --- | --- | --- | --- |
| N13 | Voice providers: phonon, piper, supertonic, sanotts, speech_lab | 2B | `srv/storymaker_audio_handlers.go`, `srv/storymaker_audio_test.go` | N6 | phonon needs `phonon.Ready` and `VoicePath`; a failing piper list gives `ready:false` and `["*"]`; sanotts lists no `hi` or `zh`; notes hold facts only |
| N14 | Cloud providers, xAI cache, `GET /voices` | 2B | `srv/storymaker_audio_handlers.go`, `srv/storymaker_handlers.go`, `srv/storymaker_service.go` (`voices`), `srv/storymaker_audio_test.go` | N7, N13 | active provider first, then the local ids, then the cloud ids sorted; inactive google is absent; `tested_languages` is never null; `no-store`; no secret in the body; OpenAI lists 13; xAI fetches once in 5 min; works read-only |
| N15 | `GET /narration`, `POST /narrate`, `writeStorymakerErrorWith`, `storymakerMusicKey` returning `""` until A15 | 2B | `srv/storymaker_audio_handlers.go`, `srv/storymaker_handlers.go`, `srv/storymaker_audio_test.go` | N11, N12, N14 | 202 new, 200 existing, 200 `{job:null}`; 409 `voice_required` names the first block; 409 `narration_unavailable` for a provider that is not ready and for inactive google; 409 `voice_unknown` with at most 5 `alternatives`; 409 `budget_exceeded` with `limit` and `missing`; 403 read-only |
| N16 | Audio file routes | 2B | `srv/storymaker_audio_handlers.go`, `srv/storymaker_handlers.go`, `srv/storymaker_audio_test.go` | N5 | Range gives 206; a file over 50 MB serves through `Open`; folder `exports`, `.env.json`, `.meta.json`, `.pending.json`, an upper-case hash and `ogg` give 404; nosniff and sandbox CSP are set |
| N17 | `POST /projects` language, status limits | 2B | `srv/storymaker_handlers.go`, `srv/storymaker_api_test.go` | N6, N14, N15 | language gives a resolved `voice`; none gives `voice:null`; 422 `narration_unavailable` with `languages`; `/status` shows the three new limits |

#### G-N-5 Render wire

| ID | Title | Batch | Files | Depends on | Failing test |
| --- | --- | --- | --- | --- | --- |
| N18 | Report token and 8 MiB cap (S1). Wire: Go+JS | 2B | `rn/listener.go` (lines 24, 383), `rn/render.go` (token from `crypto/rand`), `pl/host-render.js` (lines 33, 42, 55, 173-174), `rn/listener_test.go` (fixture at line 37, every POST at 247-289), `rn/render_test.go` (fake page at 136, 172, 179, 203, 208), `rn/browser_smoke_test.go` (458-468: the literal stays equal to `host-render.js:163`, or `TestGuardAndRequestOverheadSpike` silently stops preflighting), `pt/09-host-render.mjs` (`makeServer` strips `t` before its matches at 35, 41-42, 48; a new run with `boot.token`) | none | the second GET of `boot.json` has no token; HEAD consumes nothing; a report without the token gets 403, calls no hook and leaves the one-shot free; a token under 32 characters is refused; a forced 500 on `/report` leaves no token or prefix in `__smStatus` or the job error; without `boot.token` the page sends no `t` |
| N19 | Fallback faces on the render page. Wire: Go+JS | 2B | `rn/listener.go` (`font()` at line 373, boot), `rn/render.go` (`snapshot`), `pl/host-render.js` (lines 72-76), `rn/listener_test.go`, `pt/09-host-render.mjs` (104-105) | G3, G7, N18 | a Latin story fetches no CJK face; `font:sm-cjk/400` resolves through `boot.fonts`; a response over `max_bytes` is a readiness failure; `.otf` serves as `font/otf`; `glyphs.json` is in the snapshot |
| N20 | `Plan.AudioInfo` in boot | 2B | `rn/render.go`, `rn/listener.go` (lines 330, 342), `srv/storymaker_service.go`, `rn/render_test.go` | N5, N15, N18 | `boot.audio_info.narration` equals the `narration` of `GET /narration` byte for byte; nil serves `{}`; a nil `MusicKey` means key `""` |
| N21 | Report validation (S2): film length, events, captions | 2B | `rn/report.go`, `rn/render.go`, `rn/report_test.go` | N20 | table refuses an unknown block, wrong hash, duplicate piece, a gap in a block's piece prefix, a narration block with no event, an unknown kind, a time past `filmMS`, a `duration_ms` off by more than 1, over `MaxEvents`, over 8000 captions, a caption with 0 lines, an empty line, a line over 256 UTF-16 units and a caption not in its block; an empty `audio_events` with measured blocks is refused; a dialogue block with no event passes; a narration with U+0001 passes; a JS `gain` is ignored |

#### G-S-1 Fonts, cues, report

| ID | Title | Batch | Files | Depends on | Failing test |
| --- | --- | --- | --- | --- | --- |
| S1 | Font needs and chain | 2B | `pl/plan.js`, `pl/compile.js`, `pl/render.js`, `pl/codescene.js`, `pl/host-core.js`, `pl/worker-scene.js` (`fontChain`), `pt/15-glyphs.mjs` | G6 | Han text needs `font:sm-cjk/400`; Latin adds no fallback; U+2026 always counts; narration adds needs only with captions on and never in a silent scene; a code scene in `el` gets both Greek faces and one in `zh-CN` gets `sm-cjk`; a code scene's `g.text` sets `ctx.font` with `"SMCJK"` in it |
| S2 | Cue map, blocks by id, one event per piece | 2B | `pl/cuemap.js`, `pl/plan.js`, `pl/compile.js` (lines 97-100), `parts`, `pt/17-cuemap.mjs`, `pt/04-compile.mjs` (57-59, 97-99) | N4 | a marker at a sentence start lands on that piece's `at_ms`; a `chars` mismatch or `speech_ok:false` gives `proportion`; de, el, hi and ja fixtures place cues; each piece gives one event with `block_id` and `piece`, no `gain` |
| S3 | Cue fields, `cue_precision` and the film end in the report. Wire: JS output to `parseReport` | 2B | `pl/compile.js`, `pl/plan.js`, `rn/testdata/report_measured.json`, `pt/04-compile.mjs`, `rn/report_test.go` | S2, N21 | `cue_precision` is null without cues; no two narration events overlap; an event at or past `duration_ms` is dropped with one warning at `/scenes/<i>/duration`, for a last scene with duration 3 on a 10 s narration and for a last code scene shorter than its narration; Node's report equals the fixture and Go's `parseReport` accepts the fixture |
| S4 | Captions in the report | 2B | `pl/captions.js`, `pl/compile.js`, `parts`, `pt/18-captions.mjs`, `pt/04-compile.mjs` (24) | S2 | width 42 with wide characters counting 2; at most 2 lines; each at least 1.0 s except at the film end; filled for estimated blocks and with captions off; a caption start equals `startMs + round(cueMap(a))` on a two-piece block; a one-word last line is cut at `duration_ms`; a caption left with no length is dropped |

#### G-S-2 Dialogue and bubbles

| ID | Title | Batch | Files | Depends on | Failing test |
| --- | --- | --- | --- | --- | --- |
| S5 | Catalog styles `text.speech`, `text.thought`, `text.subtitle` | 2B | `cat/lib/catalog.mjs`, `cat/lib/names.mjs`, `ui/img/storymaker/**`, `ui/lang/storymaker_catalog/*.json`, `cat/test/catalog.test.mjs` (24: 109; 78: 79), `sm/testdata/catalog-mini/catalog.json`, `pt/00-fixtures.mjs` (29: 23), `sm/catalog_test.go` (233: 30 missing files) | G1 | the entry count pin moves to 79; each new style has size 0.042 and a name in 16 locales |
| S6 | `bubble.js` and the bubble entity | 2B | `pl/bubble.js`, `pl/scene.js`, `pl/render.js`, `parts`, `pt/19-bubble.mjs` | S5 | speech draws a tail, thought three circles; the box stays in the safe area for an actor at the edge; alpha 0.5 at 0.06 s |
| S7 | Say with text (frozen 4.6 steps 8-9), base font key per bubble style | 2B | `pl/beats.js`, `pl/compile.js`, `pl/plan.js`, `pt/04-compile.mjs` (105, 112), `pt/19-bubble.mjs` | S2, S3, S6 | a say starts at the later of its cue and the previous block's end; blocks never overlap; auto covers the last block plus 0.4 s; one `dialogue` event per piece; an estimated say warns `narration_missing` at `/scenes/<i>/beats/<j>/text`; an off-stage actor keeps the events and gets no bubble or speech interval; a bubble adds the key of its style |
| S8 | Say without text (bubble with the narration's caption lines) | 2B | `pl/beats.js`, `pl/scene.js`, `pl/compile.js`, `pt/19-bubble.mjs` | S4, S7 | the bubble shows the active caption line at each time until `T+narration.seconds` with the 0.12 s fades; it creates no block and leaves `blockEnd`, `cursor` and `lastEnd` alone; it adds its font needs; a silent scene draws none |

#### G-S-3 Captions drawn, lip-sync

| ID | Title | Batch | Files | Depends on | Failing test |
| --- | --- | --- | --- | --- | --- |
| S9 | Burned captions, base font key of `text.subtitle` | 2B | `pl/render.js` (`renderFrame`, lines 219-236), `pl/compile.js`, `pt/18-captions.mjs`, `pt/05-render.mjs`, `pt/04-compile.mjs` (150 gains `font:default/400`) | S4, S5 | captions draw after the transition composite in both branches and in thumbs; none with `style.captions` false; bottom edge at 82 % in 9:16; a Latin story with narration and no on-screen text needs the base font; `pt/09-host-render.mjs:104-105` still sees one font GET |
| S10 | Lip-sync | 2B | `pl/lipsync.js`, `pl/scene.js`, `pl/render.js` (line 116), `parts`, `pt/20-lipsync.mjs` | S2, S7 | the level table maps to mouths; two runs are identical; a dip of 40 gives `mbp`; no levels give `rest`; a cast speaker and a say `who` both get the track |
| S11 | Speech fixture and real-catalog check, lint sees dialogue | 2B | `sm/testdata/stories/speech.json`, `pl/lint.js`, `pt/11-real-catalog.mjs`, `pt/14-lint.mjs` | S1-S10 | the fixture compiles against the real catalog with every face in `needs` and zero errors; dialogue suppresses `dead_stretch` |

#### G-E1-1 App data and preview

| ID | Title | Batch | Files | Depends on | Failing test |
| --- | --- | --- | --- | --- | --- |
| E1a | API client additions and error `data` | 2B | `app/storymaker-api.js`, `app/storymaker-core.js` (`normalizeError` lines 257-268, `errorFromResponse` lines 271-282, `describeError` ahead of line 297), `ui/desktop_storymaker_api_test.go` | N15, N16 | `narrationUrl` refuses an upper-case hash and `ogg`; `listJobs` and `narrate` call the right routes; `err.data.languages` survives both error functions; `narration_unavailable` and `voice_unknown` get their own sentence |
| E1b | Message whitelists. Wire: parent and host | 2B | `app/storymaker-player.js` (line 13), `pl/host-app.js`, `ui/desktop_storymaker_model_test.go`, `pt/08-host-app.mjs` | S3, S4 | a malformed event is dropped and the message stays; `audio_info` over its caps, including 200000 levels per message, is refused; `glyph_data` needs the schema string |
| E1c | `storymaker-audio.js`: narration, pure model | 2B | `app/storymaker-audio.js`, `ui/js/desktop/core/module-loader.js`, `ui/desktop_storymaker_registration_test.go`, `ui/desktop_storymaker_audio_test.go` | E1a | `windowFor` keeps 2 s back to 20 s ahead; `waveform` gives one value per frame; `sync` above 120 ms drift reschedules on a fake `AudioContext` |
| E1d | `bindAudio` and player reload | 2B | `app/storymaker-audio.js`, `app/storymaker.js` (line 391), `ui/desktop_storymaker_fixture_test.go`, `ui/desktop_storymaker_audio_test.go` | E1b, E1c | open, save and a narrate job on every terminal status each refresh once; a stale answer for another project is ignored; `load` carries `audio_info.narration`; open with existing audio gives a second `load` with the blocks, and the following `compiled` feeds `createAudio.load` |

#### G-E1-2 Panels

| ID | Title | Batch | Files | Depends on | Failing test |
| --- | --- | --- | --- | --- | --- |
| E1e | Voice panel | 2B | `app/storymaker-voice.js`, `app/storymaker.js`, `ui/js/desktop/core/module-loader.js`, `ui/desktop_storymaker_registration_test.go`, `ui/css/desktop-app-storymaker.css`, `ui/desktop_storymaker_audio_test.go` | E1d, N14 | a `voice:null` story preselects the active provider; `not tested` shows for a list provider, a free-text provider and sanotts; the language select starts from `window.SYSTEM_LANG` and is limited to the provider's `languages`, with a free-text field for `["*"]`; one action writes `{provider, voice, language}`; audition plays the first stored piece and is disabled with none; no ready provider shows the "no TTS ready" state |
| E1f | Narration status, Generate, "no audio" marker, job filter, captions toggle | 2B | `app/storymaker-voice.js`, `app/storymaker-export.js` (line 267), `app/storymaker.js`, `ui/desktop_storymaker_audio_test.go` | E1e | a `narrate` job event leaves the export popover idle and advances `pieces_done`; a missing block shows the marker; read-only disables Generate; `interrupted` and `cancelled` show the panel's own text; the toggle writes `style.captions` |
| E1g | Scrubber waveform | 2B | `app/storymaker-strip.js`, `app/storymaker-audio.js`, `ui/css/desktop-app-storymaker.css`, `ui/desktop_storymaker_css_test.go`, `ui/desktop_storymaker_audio_test.go` | E1c | the waveform has block levels at block frames and 0 in pauses; `.stm-scrub-wave` has `pointer-events:none` |
| E1h | New project sends the language | 2B | `app/storymaker-projects.js` (line 69), `app/storymaker.js`, `ui/desktop_storymaker_model_test.go` | E1a, N17 | 422 `narration_unavailable` shows `err.data.languages` and an offer; the second call sends no `language` |

#### G-E1-3 Close 2B

| ID | Title | Batch | Files | Depends on | Failing test |
| --- | --- | --- | --- | --- | --- |
| E1i | 2B strings | 2B | `app/storymaker-core.js` (lines 311, 319), `ui/desktop_storymaker_guards_test.go` (line 161), `ui/desktop_storymaker_i18n_test.go` (exact list at lines 81-85), `ui/lang/storymaker/*.json` | E1a-E1h | the pin with `glyph_missing` and the four new job errors fails until all 16 locales have them |
| E1j | Contract slice, child docs, cue share | 2B | contract section 2 (line 59; lines 81-93 say, speaker, narration audio row; line 155; lines 177-182; `glyph_missing` row), section 3 (193; 194 text part; 195; after 199; 206; 246 text styles; 250 with 79; Glyphs subsection), section 3a (279), section 3b (297 with the speech styles), section 4 (Store `Open`; narration, envelope, audio info, glyph and voice API; 317 with 28 codes; 360 glyphs; 414; 417; 427), section 5 (473; 481-484; rows `GET /voices`, `GET /projects/{id}/narration`, `POST /projects/{id}/narrate`, both audio routes; queue line for narrate; 504 limits; 513-516 kinds and pieces; 523 narrate codes; 537), section 6 (553-554; 591; 600-602; 607; 609; 614; 615; 618 say; 619 say step; 621), section 7 (672; 684-686 `?t=` and 8 MiB; token note without `/start`; 702-704 without `workers`, `chunk`, `pre_roll`; 712; 716; 723; 743; 747; 751; 800), section 8 (823; 832; 848-859; 853), section 9 (881 with `audio`, `voice`; 895); `sm/AGENTS.md`, `rn/AGENTS.md`, `pl/AGENTS.md`, `app/AGENTS.md`, `srv/AGENTS.md`, `documentation/storymaker.md`; share of cues placed by spans recorded (spec line 853) | E1i | no failing test (docs); path check |
| E1k | Tour with voice and narration | 2B | `srv/storymaker_app_browser_test.go` | E1e-E1h, N11 | `TestStorymakerAppRealServerBrowser` with a stub synth picks a voice, generates narration, sees the waveform and still ends in a real export |

### Batch 2C: parts A, E2

Part A starts at A1: nothing is measured again, the real-material result is in `<evidence>/mixReal.md`.

#### G-A-1 SFX set and catalog

The rows run in the order shown: A4 lands before A3, because `sm/catalog.go:334` and `:522` refuse an empty thumb, and a real catalog with sfx entries would stop every real-catalog test and the runtime.

| ID | Title | Batch | Files | Depends on | Failing test |
| --- | --- | --- | --- | --- | --- |
| A1 | SFX files and provenance | 2C | `cat/sfx/*.wav` (25, from `<spikes>/sfx/set`), `cat/README.md`, `.gitattributes`, `licences/THIRD_PARTY_LIBRARIES.md`, `cat/test/sfx.test.mjs` | none | 25 WAVs at 48 kHz mono PCM16 whose sha256 equals the README table |
| A4 | Go catalog fields, on a test catalog | 2C | `sm/catalog.go` (lines 334, 412, 522), `rn/render.go` (lines 558-562), `sm/catalog_test.go` | none | on a `fstest.MapFS` catalog: an sfx entry with an empty thumb and 2 takes loads; one take, no `align`, no `source` and `file` other than `takes[0].file` are refused; `VerifyFiles` names a missing take and skips the empty thumb |
| A2 | Binary files in the catalog tooling, no output change | 2C | `cat/lib/catalog.mjs` (`versionOf`, line 257), `scripts/build-storymaker-catalog.mjs` (line 22, lines 35-49, write and check), `cat/test/catalog.test.mjs` (20, 23: Buffer values) | A1 | `versionOf` over a Map with a Buffer hashes the raw bytes without `lf()`; the file loops accept a Buffer; the generated output and `version` do not change, so `--check` stays green |
| A3 | Catalog sfx entries and the WAV copy | 2C | `cat/lib/catalog.mjs`, `cat/lib/names.mjs`, `cat/lib/validate.mjs`, `ui/img/storymaker/catalog.json`, `ui/img/storymaker/sfx/*.wav`, `ui/lang/storymaker_catalog/*.json`, `cat/test/catalog.test.mjs` (24: 134; 78: 86), `ui/storymaker_catalog_test.go` (327 `sfx` prefix; 333-337 no thumb for sfx; 370-373 sfx fields; 767 raw hash for `.wav`; 769-771 allow `.wav`; 777-787 reference `takes[].file`), `sm/catalog_real_test.go`, `sm/validate_test.go`, `rn/render_test.go` | A1, A2, A4 | 86 entries; `sfx.typing` has `pattern` and no `hit`; `error` and `success` have `align:onset`; `--check` fails on one changed WAV byte; `sound:"sfx.nope"` on the real catalog becomes `unknown_asset` (`sm/validate.go:638-640` now runs); the render snapshot holds no WAV |

#### G-A-2 Story fields and compile

| ID | Title | Batch | Files | Depends on | Failing test |
| --- | --- | --- | --- | --- | --- |
| A5 | Music fields, keys, music store, `auto_sfx`, `sfx` beat accepted | 2C | `sm/music.go` (with `MusicMeta`, `WriteMusic`, `WriteMusicMeta`, `MusicInfo`), `sm/validate.go` (lines 232-234), `sm/model.go`, `sm/decode.go`, `sm/music_test.go`, `sm/validate_test.go` (177) | none | both `file` and `track` is `invalid_value` at `/music/track`; track `abc` is refused; generated without mood or prompt is `missing_field` at `/music/mood`; a mood outside the presets is `invalid_value`; the key excludes length; `MusicPrompt` joins template and prompt with `". "`; a meta round trip works and an invalid meta reads as `V` 0; `auto_sfx` is accepted |
| A6 | Sfx beat: impact and takes | 2C | `pl/beats.js`, `pl/scene.js`, `pl/motion.js`, `pl/compile.js` (`lastTake`), `sm/testdata/catalog-mini/catalog.json`, `pt/21-sfx.mjs`, `pt/04-compile.mjs` (105, 112), `pt/00-fixtures.mjs` (29 new count; 31 skips `.wav`), `sm/catalog_test.go` (233 new count; 236-244 create the takes, skip an empty thumb) | A3, A4 | a cued whoosh starts at marker minus the take's impact; `with:"prev"` sits on the landing; a repeat never takes the same take, across scenes too; a clamp at 0 does not warn; a cued sfx in scene 2 has `start_ms` equal to scene 2's start plus its local time |
| A7 | `sfx.typing` expansion | 2C | `pl/motion.js`, `pl/beats.js`, `pt/21-sfx.mjs` | A6 | keys sit 90 to 140 ms apart inside the duration; takes are seeded; two compiles match; the default duration scales with rhythm |
| A8 | Auto-SFX | 2C | `pl/motion.js`, `pl/beats.js`, `pt/21-sfx.mjs` | A6 | each pop entry gets `sfx.pop` on its landing, unless an sfx beat has that cue or runs `with:"prev"` on it; absent means off |
| A9 | Report validation for sfx | 2C | `rn/report.go`, `rn/render.go`, `rn/report_test.go` | A4, A7, A8 | an out-of-range take is refused; `MaxEvents` holds a real typing and auto-SFX compile, including a calm story at 1500 ms; `maxRhythmFactor` equals the largest value of `RHYTHM` in `pl/motion.js`; 32769 events are refused |

#### G-A-3 Mixer

| ID | Title | Batch | Files | Depends on | Failing test |
| --- | --- | --- | --- | --- | --- |
| A10 | Meter | 2C | `rn/meter.go`, `rn/meter_test.go` | none | `parseMeterOutput` on stored stderr gives I from the `I:` line and TP from JSON; `-inf` means silent; the real graph runs on 8.1.1 and on 5.1.9 (one `debian:12 --rm`), with `framelog=verbose` as the fallback |
| A11 | Mix core: clips, bed, loop, stems | 2C | `rn/mix.go`, `rn/mix_test.go` (from `<spikes>/mix/mixer`, `<spikes>/mixreal/mixer`) | none | mix minus the three stems is 0 at every sample; the loop crossfade is equal power and at most half the bed; trimmed clips fade 5 ms; a clip past `Frames` is dropped; cancel stops at a block boundary |
| A12 | Ducking, carve, impact dips | 2C | `rn/mix.go`, `rn/mix_test.go` | A11 | the duck reaches -6 dB 50 ms before speech and releases over 400 ms; the carve cuts 2.2 kHz only inside spans; the dip follows 10 ms, 80 ms, 0.3 s |
| A13 | Passes and gains | 2C | `rn/mix.go`, `rn/mix_test.go` | A10, A12 | two voices 4.5 dB apart land within 1 dB; `G` is the min of the three bounds; volume 0.5 is +6 dB; sfx gain is reference plus `LevelOffsetDB`; a silent bed is dropped with `BedDropped`; a quiet bed stops at +24 dB; `-inf` narration with events fails with `narration_failed`, otherwise `G=0` |

#### G-A-4 Inputs and encode

| ID | Title | Batch | Files | Depends on | Failing test |
| --- | --- | --- | --- | --- | --- |
| A14 | Seeded-audio helper | 2C | `sm/storymakertest/seed.go`, a new test beside it | N5, A5 | after `SeedNarration` the answer has no missing block and speech runs from 100 ms to end minus 100 ms; `voice:null` fixtures seed under their empty voice; `SeedMusic` leaves a file and a valid meta |
| A15 | Music key resolver and `Plan.Music` | 2C | `srv/storymaker_audio_handlers.go`, `sm/narration.go`, `rn/render.go`, `srv/storymaker_audio_test.go` | A5, A14 | a changed mood changes the key and a changed length does not; volume 0 gives key `""`; `/narration` reads `meta.json` and decodes nothing; a key without file or valid meta gives `music:null`; a moved source gives `music_issue` and keeps the narration |
| A16 | Mix plan, input build and disk check | 2C | `rn/mixinput.go`, `rn/mix.go` (`EstimateMixBytes`, `HasAudio`), `rn/mix_test.go` | A13, A15 | exact FFmpeg arguments with both whitelists; `-t` and `-fs` on every piece and on music; each hash decodes once; sfx reads `event.take`; too little disk fails with the needed size and leaves no file in `dec/`; `testdata/narration/one.mp3` decoded both ways compares the first speech run within 10 ms, or the plan records the offset |
| A17 | FFmpeg arguments, AAC 192k, `ResolveFFprobe` | 2C | `rn/ffmpeg.go` (lines 25, 74), `internal/videostudio/encoder_export.go`, `rn/render.go`, `srv/storymaker_service.go`, `rn/ffmpeg_test.go`, `rn/render_test.go` (265: `FFprobePath: testBinary(t, "ffprobe")`) | none | `-shortest` is gone; 192k; the mix is input 1 with `-format_whitelist wav`; `anullsrc` keeps `-t`; an empty `FFprobePath` fails with `ffmpeg_unavailable`; `FFmpeg.Message` holds no staging path |
| A18 | Frame gate in `finish()` | 2C | `rn/gate.go`, `rn/render.go` (lines 470-524), `rn/gate_test.go` | A17 | `CheckOutput` refuses a wrong packet count and 1 s of extra audio; `ProbeOutput` has both whitelists, the 120 s timeout and the 8 MiB cap; a refused file publishes nothing |

#### G-A-5 Deferred start, options, outputs

| ID | Title | Batch | Files | Depends on | Failing test |
| --- | --- | --- | --- | --- | --- |
| A19 | Deferred start. Wire: Go+JS | 2C | `rn/render.go` (lines 452-454, 589-628), `rn/listener.go`, `pl/host-render.js` (lines 105-110, 154), `rn/render_test.go` (fake page), `rn/listener_test.go`, `pt/09-host-render.mjs` (150-151) | N18, A17 | `/report` answers `pending` while the start blocks; `/start` answers 404 before a report, holds and then answers; a report timeout and a cancel before the report return at once; a page crash during a held mix ends the job within one block and starts no FFmpeg; the 10 s watchdog starts with FFmpeg; a stand-in's `{start:true}` still works; the four tests of `ui/storymaker_player_gate_browser_test.go` pass once with `AURAGO_RUN_BROWSER_SMOKE=1` |
| A20 | Mix wired into the job | 2C | `rn/render.go` (`startAfterReport`), `rn/mixinput.go`, `rn/jobs.go` (`Warnings`, `AddWarning`), `rn/render_test.go` | A16, A18, A19 | a seeded story's MP4 has non-silent AAC, landing between -18.5 and -15.5 LUFS at -1 dBTP or below; a stand-in hardware encoder failing on frame 1 still ends with that AAC, so `recoverEncoder` keeps the mix; a silent bed adds `music_silent`; a silent story keeps `anullsrc`; cancel during the mix ends the job |
| A21 | Render options | 2C | `rn/options.go`, `rn/render.go` (`Prepare` signature; `ApplyCaptionOverride` before lines 157-158), `srv/storymaker_handlers.go` (lines 551, 571), `rn/options_test.go`, `rn/render_test.go` (Prepare calls at 306, 535, 660, 665, 671, 679, 688, 691, 706, 718), `srv/storymaker_api_test.go` | N6, A20 | an unknown field is 400; an empty body gives defaults; the same key with other options is 409; `burn_in_captions:true` with Thai narration gives `glyph_missing`; `high` sets CRF 18 |
| A22 | Subtitles and export extras | 2C | `rn/subtitles.go`, `rn/render.go` (`publishExtras`, `Deps.WorkspaceFree`), `rn/jobs.go` (`Output`, `OutputStems`), `srv/storymaker_service.go`, `rn/subtitles_test.go`, `rn/render_test.go` | A20, A21 | `-->` is broken and `<&>` escaped; a failed stem keeps the MP4 and adds `stem_failed`; low workspace space adds the warning of that extra; a silent stem is omitted; the record has loudness |
| A23 | Library copy | 2C | `internal/desktop/service_media.go`, a new test, `rn/render.go`, `srv/storymaker_store.go`, `srv/storymaker_service.go` | A22 | the first copy on a fresh data dir creates `Storymaker`; a collision gives `-2`; a bad slug and read-only are refused; low disk is refused; a failed copy gives `library_failed` and keeps the MP4 |

#### G-A-6 Gates, music, parity

| ID | Title | Batch | Files | Depends on | Failing test |
| --- | --- | --- | --- | --- | --- |
| A24 | `narration_incomplete` and `music_incomplete` gates | 2C | `rn/render.go`, `srv/storymaker_handlers.go` (branches after line 561), `srv/storymaker_e2e_test.go`, `srv/storymaker_parity_browser_test.go`, `srv/storymaker_app_browser_test.go`, `srv/storymaker_api_test.go` (`SeedNarration` before line 546; the capability checks at 534 and 539 stay ahead of the gate), `rn/browser_smoke_test.go`, `rn/render_test.go` | A14, A15, A21 | a narrated story without audio is 409 with `missing` and `chars`; seeded it is 202; a silent scene passes; music on without its file is 409 with `key`; a moved source is 409 with key `""` and `music_issue`; volume 0 passes |
| A25a | Music route and `music_configured`. The job is a stub that fails with `music_failed` until A25b | 2C | `srv/storymaker_audio_handlers.go`, `srv/storymaker_handlers.go`, `srv/storymaker_narrate.go`, `srv/storymaker_audio_test.go` | A15, N15 | `.ogg`, a source over 1 GiB and a 0.5 s WAV are 422 `invalid_music_file`; a missing source adds `music_issue`; generated while not configured is 409 `music_unavailable`; a `duration_ms` out of range is 400 `invalid_argument`; an existing key with meta gives `{job:null}`; status reports `music_configured` |
| A25b | Music job: file and track sources | 2C | `srv/storymaker_narrate.go`, `srv/storymaker_audio_test.go` | A25a | a track with another media type or a path outside `DataDir/audio` is refused; the source opens only through `Store.Open`; a `PathConflict` counts as success; `ProbeAudio` writes the meta; a file without valid meta gets it on a rerun; a duration outside 1000..3600000 ms removes the copy and fails with `music_failed`; a read-only stop fails with `music_failed` naming the state |
| A25c | Music job: generated source. Test seam: `storymakerMusicOverride` on `Server` | 2C | `srv/storymaker_narrate.go`, `srv/server.go`, `srv/storymaker_audio_test.go` | A25b | `IsBlocked` refuses a generated track; read-only is re-checked before the call; a film longer than the ACE-Step profile requests the profile's limit and succeeds; a result without `DurationMs` gets its meta from `ProbeAudio`; a rerun after a failed copy reads `pending.json` and makes no provider call; `RecordCostForCategory` runs once |
| A26 | Parity and export with audio | 2C | `srv/storymaker_parity_browser_test.go`, `srv/storymaker_e2e_test.go` | A20, A24 | app player and render page give identical RGBA with the same seeded `audio_info`; the export has AAC with a measured level and the frame gate passes |

#### G-E2-1 App for sound, close 2C

| ID | Title | Batch | Files | Depends on | Failing test |
| --- | --- | --- | --- | --- | --- |
| E2a | API: `music`, `noisemakerTracks`, `startRender` options, music errors | 2C | `app/storymaker-api.js` (line 91), `app/storymaker-core.js` (`describeError`), `ui/desktop_storymaker_api_test.go` | A21, A25a | options send a JSON body and no options send none; `musicUrl` refuses a bad key; `noisemakerTracks` calls `/api/desktop/noisemaker/tracks`; `invalid_music_file` and `music_unavailable` get their own sentence |
| E2b | Music panel | 2C | `app/storymaker-music.js`, `app/storymaker.js`, `ui/js/desktop/core/module-loader.js`, `ui/desktop_storymaker_registration_test.go`, `ui/desktop_storymaker_fixture_test.go`, `ui/css/desktop-app-storymaker.css`, `ui/desktop_storymaker_audio_test.go` | E2a | generated mode hides without `music_configured`; generated with `audioInfo.music` null shows a button with mood and prompt and sends nothing until the click, also after a prompt edit; file mode sends one `api.music` per key with the report's duration; the picker uses `window.AuraDesktopFileDialogs.open`; `file` and `track` stay exclusive; `music_issue` shows the missing source and offers another file; `interrupted` and `cancelled` show the panel's own text |
| E2c | Preview with music and SFX | 2C | `app/storymaker-audio.js`, `ui/desktop_storymaker_audio_test.go` | E2b, A6 | the bed is one `<audio>` through `MediaElementSource`; the duck follows stored levels; SFX schedule from `audio_events`; no URL comes from message text |
| E2d | Export errors, missing audio actions, silent note | 2C | `app/storymaker-export.js` (lines 145, 213), `app/storymaker-core.js`, `ui/desktop_storymaker_model_test.go` | A24 | 409 `narration_incomplete` shows `err.data.missing` and an action to the voice panel; `music_incomplete` leads to the music panel; `silent_film` shows only with no narration, no music and empty `audio_events` |
| E2e | 2C strings | 2C | `app/storymaker-core.js` (line 311), `ui/desktop_storymaker_i18n_test.go` (exact list at lines 81-85), `ui/lang/storymaker/*.json` (removes `storymaker.export.silent_note`) | E2a-E2d | the guards fail until `narration_incomplete`, `music_failed`, `music_incomplete`, the error sentences, `silent_film` and the panel keys exist in 16 locales |
| E2f | Contract slice and docs | 2C | contract section 2 (line 29 `auto_sfx`; after 64 the music sentence; lines 81-93 sfx and music row; 156), section 3 (194 WAV part; 214; 228; 246 sfx row; 250 with 86; 260), section 3b (297 sfx entries), section 4 (342-343; 360 sfx fields; music API and `Entry` fields), section 5 (480; 490; `POST /projects/{id}/music` row; queue line for music; 504 `music_configured`; 513-516 output, warnings, phases; 519; 523 sound codes; 528; 530), section 6 (618 sfx; 619 impact rule), section 7 (665; 680; `/start` row at 684-686; token note adds `/start`; 717; 805; 806; "### Frame gate"), section 9 (881 with `music`); `rn/AGENTS.md`, `sm/AGENTS.md`, `app/AGENTS.md`, `srv/AGENTS.md`, `internal/desktop/AGENTS.md`, `documentation/storymaker.md` | E2e, A26 | no failing test (docs); path check |
| E2g | Tour with sound | 2C | `srv/storymaker_app_browser_test.go` | E2e, A26 | the tour fails until it generates narration, passes the gate and exports with an audible track |

### Batch 2D: parts E3, W, I

#### G-E3-1 Export dialog

| ID | Title | Batch | Files | Depends on | Failing test |
| --- | --- | --- | --- | --- | --- |
| E3a | Dialog module | 2D | `app/storymaker-dialog.js`, `ui/js/desktop/core/module-loader.js`, `ui/desktop_storymaker_registration_test.go`, `ui/css/desktop-app-storymaker.css`, `ui/desktop_storymaker_model_test.go` | E2a | burn-in presets from `style.captions`; "Open in Video Studio" forces `stems:true` and adds the job id to `inst.handoffWish`; the guards accept the module |
| E3b | Export start through the dialog, new blockers | 2D | `app/storymaker-export.js`, `app/storymaker.js`, `ui/desktop_storymaker_fixture_test.go`, `ui/desktop_storymaker_model_test.go` | E3a | missing narration or music blocks the start with its reason; `startRender` receives the chosen options |
| E3c | Finished-job actions | 2D | `app/storymaker-export.js`, `ui/desktop_storymaker_model_test.go` | E3b | stems, SRT, VTT and library paths show when present; each warning shows as text; a job without extras shows only the MP4 |

#### G-E3-2 Handoff

| ID | Title | Batch | Files | Depends on | Failing test |
| --- | --- | --- | --- | --- | --- |
| E3d | `buildTracks`. Wire: JS output against the Go validator | 2D | `app/storymaker-handoff.js`, `ui/js/desktop/core/module-loader.js`, `ui/desktop_storymaker_registration_test.go`, `ui/desktop_storymaker_handoff_test.go` | none | Node output decodes strictly into `videostudio.Project` and passes `videostudio.Validate` (`internal/videostudio/validate.go:41`); durations cap at frame count; a silent stem is skipped |
| E3e | `handoff.run` and `canOffer` | 2D | `app/storymaker-handoff.js`, `app/storymaker-export.js`, `ui/desktop_storymaker_handoff_test.go` | E3c, E3d | a fake fetch sees project, media with `storymaker-<job>-<name>` keys, polling every 500 ms up to 120 s, GET, PUT with `If-Match`, then `openApp` |
| E3f | `projectId` launch (dirty shell files). Runs `npm run build:ui` and leaves `main.bundle.js` rebuilt and uncommitted for I2 | 2D | `app/video-studio.js` (lines 375, 456, 884), `ui/js/desktop/core/window-shell-runtime.js` (line 1615), `ui/js/desktop/core/menus-and-routing.js` (line 1649), new `ui/desktop_video_studio_open_project_test.go` | none | `openProject` before ready stores the id; when ready it runs `switchProject` with the dirty guard; `initialize` prefers a listed id; the reuse branch calls it |
| E3g | 2D strings | 2D | `ui/desktop_storymaker_i18n_test.go` (exact list at lines 81-85, SRT and VTT labels), `ui/lang/storymaker/*.json` | E3a-E3f | the i18n guards fail until the dialog and handoff keys exist in 16 locales |

#### G-W-1 Parallel workers (droppable)

| ID | Title | Batch | Files | Depends on | Failing test |
| --- | --- | --- | --- | --- | --- |
| W1 | `ChooseWorkers`, settings, boot. Wire: Go to boot document | 2D | `rn/workers.go`, `rn/render.go`, `rn/listener.go`, `srv/storymaker_service.go`, `rn/workers_test.go`, `rn/listener_test.go` | none | a table of CPU, memory, frame size, CJK and code scenes; a code scene in `zh` or `ja` sets `cjk`; non-png gives 1; boot omits absent values |
| W2 | PNG in the worker, helper engines | 2D | `pl/worker-player.js`, `pl/host-core.js`, `pt/07-workers.mjs` | none | `render` with `png` returns a blob; `shared_code_scenes` builds no scene workers |
| W3 | `host-pool.js` | 2D | `pl/host-pool.js`, `parts`, `pt/22-host-pool.mjs`, `pt/09-host-render.mjs` (183: value 6) | W2 | frames post in order with one POST in flight; the window is `N*C*2`; code-scene chunks go to worker 0; a restart re-runs from 0; a crash rejects |
| W4 | Parallel path in the render page. Wire: Go+JS | 2D | `pl/host-render.js`, `pt/09-host-render.mjs` | W1, W3 | helpers prepare before the report; a differing `frame_count` is `render_failed`; `workers:1` keeps the serial path |
| W5 | Parallel parity and forced encoder failure | 2D | `rn/browser_smoke_test.go`, `sm/testdata/stories/speech.json` | W4, S11 | `TestParallelParityBrowser`: N workers against 1 give identical RGBA on `firewall.json` and `speech.json`, pre-roll on and off; `TestParallelEncoderFailureBrowser`: one forced failure restarts from frame 0 |

#### G-I-1 Integration

| ID | Title | Batch | Files | Depends on | Failing test |
| --- | --- | --- | --- | --- | --- |
| I1 | Narrated export end to end with all options | 2D | `srv/storymaker_e2e_test.go` | E3c, A25c | create with `language`, narrate with the stub synth, render with stems, SRT and VTT; every output exists, the SRT matches the text and ffprobe sees the frame count |
| I2 | Final app tour. Precondition: the worktree holds the `main.bundle.js` that E3f rebuilt | 2D | `srv/storymaker_app_browser_test.go` | E3e, E3f | the tour fails until dialog, stems, handoff and the opened Video Studio project all work |
| I3 | FFmpeg 5.1.9: swatch and gate (spec line 836) | 2D | `rn/ffmpeg_test.go`, spec To verify | A18 | a new swatch test decodes a known colour within 2 levels on 8.1.1 and in one `debian:12 --rm` container |

#### G-I-2 Close phase 2

| ID | Title | Batch | Files | Depends on | Failing test |
| --- | --- | --- | --- | --- | --- |
| I4 | Contract final slice, spec To verify results | 2D | contract section 6 (553-554 `host-pool.js` and 658, if W landed), section 7 (702-704 `workers`, `chunk`, `pre_roll` and "### Workers", if W landed), section 9 (881 final list with `dialog`, `handoff`; 906); spec To verify items with results and any deviation | I1-I3 | no failing test (docs); path check |
| I5 | Root and child docs, manuals | 2D | `AGENTS.md` (line 212), `pl/AGENTS.md`, `sm/AGENTS.md`, `rn/AGENTS.md`, `app/AGENTS.md`, `srv/AGENTS.md`, `internal/desktop/AGENTS.md`, `documentation/storymaker.md`, `documentation/manuals/en/04-webui.md`, `documentation/manuals/de/04-webui.md` | I4 | no failing test (docs); path check |
| I6 | Asset-pack scratch instance, by the recipe below | 2D | scratch run dir only, result in the plan | I2 | login, voice with sanotts, narrate, export with stems, handoff on an asset-pack build. Phonon's Windows `ptts.exe` cannot write here |
| I7 | Final matrix and landing | 2D | none | I1-I6 | the 2D matrix of section 4 is green, except failures confirmed on clean main |

**I6 recipe.** Run all of it with the shell sandbox off, under `D:\tmp\go-build\sm-i6`, a short path, because a repo-built executable cannot start Chrome on this box and a long run dir breaks the bundled ONNX runtime.
1. Snapshot the branch: `git archive <branch>` unpacked into `src` with `C:\Windows\System32\tar.exe`.
2. In `src`: `go run ./cmd/assetpack -out deploy -stage assets/web`, then `go build -trimpath -ldflags "-s -w $(Get-Content -Raw deploy/web-assets.ldflags)" -o D:\tmp\go-build\sm-i6\aurago.exe ./cmd/aurago`.
3. `aurago.exe -install-assets deploy\aurago-web-assets-<id>.tar.gz`, then `-check-assets`.
4. Config: text-replace `config_template.yaml`, read with `Get-Content -Raw -Encoding UTF8`. Set `server.port`, `https.enabled: false` next to it, `server.ui_language: en`, and `storymaker.render.browser_path` to the installed Chrome.
5. Child environment only: `AURAGO_MASTER_KEY` (random 64 hex) and `DOCKER_HOST=tcp://127.0.0.1:1`. First start with `-password`.
6. Stop by PID after checking the exe path.

## 4 Test policy

The user's rule applies to every batch.

- Each task writes its named failing test, plus one test for each bug fix that matters. No extra property, mutation or stress suites.
- One combined review per batch, by reader-agent for spec and quality. It reports Critical and Important findings only.
- security-agent reviews 2B (token, audio routes, decoders, read-only re-check), 2C (music inputs, FFmpeg inputs, library path, report validation) and 2D (handoff).
- Fixes go to maker-agent. There is no re-review round: the controller checks the diff and runs the tests touched.
- At most one browser smoke per batch: the real-server tour. The parity tests that the spec names (2C, 2D) and W5 are their own proofs. M11 and A19 also run the four tests of `ui/storymaker_player_gate_browser_test.go` once, because they change what those tests pin and no matrix runs them.
- The stand-in tours `TestStorymakerAppBrowser` and `TestStorymakerAppStatesBrowser` stay compiled and are run only when a failure points at them.

Set this first in every shell, from the batch worktree. Keep the backslash form of `GOTMPDIR`.

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'
$env:GOTMPDIR = 'D:\tmp\go-build'
```

Browser lines run through `go test` only, so test binaries live under `D:\tmp\go-build`, with the shell sandbox off. Repo-built executables cannot start Chrome on this box. Set `AURAGO_RUN_BROWSER_SMOKE` only on browser lines, then remove it.

For `-run`, group alternatives as `(?i)(a|b)` and count PASS lines when a run is evidence.

### 2A matrix

```powershell
npm run build:ui
npm run check:ui
npm run test:storymaker
node scripts/build-storymaker-ranges.mjs --check
npm run check:storymaker-catalog
go test ./internal/storymaker/... ./internal/config ./internal/audit -count=1
go test ./internal/server -run Storymaker -count=1 -timeout 20m
go test ./ui -run Storymaker -count=1 -timeout 20m
go test ./ui -run '^TestShippedWebAssetsLoadNothingExternal$' -count=1
gofmt -l internal/storymaker internal/server internal/config
go vet ./internal/storymaker/... ./internal/server
$env:GOOS='linux'; go build ./...; Remove-Item Env:GOOS
$env:AURAGO_RUN_BROWSER_SMOKE='1'; go test ./internal/server -run '^TestStorymakerAppRealServerBrowser$' -count=1 -timeout 15m; Remove-Item Env:AURAGO_RUN_BROWSER_SMOKE
```

### 2B matrix

The 2A lines, with `internal/tools` added to the `gofmt` line, plus these before the browser line:

```powershell
node --test assets/storymaker/test/glyphs.test.mjs
node --test assets/storymaker/test/catalog.test.mjs
go test ./internal/tools -run '(?i)(elevenlabs|openaitts)' -count=1
go test ./cmd/config-merger ./internal/setup -count=1
```

### 2C matrix

The 2B lines, with `internal/desktop internal/videostudio` added to the `gofmt` line and `./internal/desktop ./internal/videostudio` added to the `go vet` line, plus:

```powershell
node --test assets/storymaker/test/sfx.test.mjs
go test ./internal/videostudio -run ResolveFFprobe -count=1
go test ./internal/desktop -run WriteStorymakerVideo -count=1
$env:AURAGO_RUN_BROWSER_SMOKE='1'; go test ./internal/server -run '^TestStorymakerParityAppPlayerAndRenderPage$' -count=1 -timeout 15m; Remove-Item Env:AURAGO_RUN_BROWSER_SMOKE
```

### 2D matrix (the whole phase)

```powershell
npm run build:ui
npm run check:ui
npm run test:storymaker
node scripts/build-storymaker-ranges.mjs --check
npm run check:storymaker-catalog
node --test assets/storymaker/test/glyphs.test.mjs
node --test assets/storymaker/test/catalog.test.mjs
node --test assets/storymaker/test/sfx.test.mjs
go test ./internal/storymaker/... ./internal/config ./internal/desktop ./internal/security ./internal/videostudio ./internal/audit ./cmd/config-merger ./internal/setup -count=1
go test ./internal/tools -run '(?i)(elevenlabs|openaitts)' -count=1
go test ./internal/server -run Storymaker -count=1 -timeout 20m
go test ./ui -run '(?i)(storymaker|videostudio)' -count=1 -timeout 20m
go test ./ui -run '^TestShippedWebAssetsLoadNothingExternal$' -count=1
gofmt -l internal/storymaker internal/server internal/config internal/desktop internal/tools internal/videostudio
go vet ./internal/storymaker/... ./internal/server ./internal/desktop
$env:GOOS='linux'; go build ./...; Remove-Item Env:GOOS
$env:AURAGO_RUN_BROWSER_SMOKE='1'
go test ./internal/server -run '^TestStorymakerAppRealServerBrowser$' -count=1 -timeout 15m
go test ./internal/server -run '^TestStorymakerParityAppPlayerAndRenderPage$' -count=1 -timeout 15m
go test ./internal/storymaker/render -run '^TestParallelParityBrowser$' -count=1 -timeout 20m
go test ./internal/storymaker/render -run '^TestParallelEncoderFailureBrowser$' -count=1 -timeout 15m
Remove-Item Env:AURAGO_RUN_BROWSER_SMOKE
```

Drop the two parallel lines if W was dropped.

Known failures on main, recorded 2026-11-05: `internal/audit` `TestUpdateConfigFallbackOnlyTrustedTransactionBackups` and `TestCredentialedHTTPClientsBindRedirectsToOrigin`, whose pending list names `internal/agentmail/client.go`, a file this plan does not touch. Confirm any failure on a clean main before calling it pre-existing, and list it in the batch report. A browser test that times out while other work loads the machine is rerun alone before it counts as a failure; `TestStorymakerAppReviewFixesBrowser` did this once in the dry run and then passed three times.

## 5 Execution rules

**Agents (CLAUDE.md roster, no model overrides):**
- maker-agent writes and edits code.
- test-agent runs tests and validation.
- docs-agent writes docs, including contract, spec and plan.
- security-agent reviews security.
- reader-agent reads and does the combined review.
- debug-agent investigates failures.
- Fixes always go back to maker-agent. Small verbatim tasks may be batched in one dispatch.

**GitNexus:**
- Run `impact` before editing an existing symbol, from the main checkout, because `.worktrees/*` is not registered: `node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "<symbol>" --direction upstream --repo .`
- Report callers, processes and risk. Warn on HIGH or CRITICAL.
- `risk: UNKNOWN` is unresolved: confirm with a text search.
- Run `detect_changes` before every commit, from the worktree root: `node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo C:/Users/Andi/Documents/repo/AuraGo`. The absolute path selects the main checkout's index, and the tool diffs the linked worktree it is started in (checked on 2026-11-05 from a linked worktree). `--repo .` answers `not found` there. A run from the main checkout diffs main's working tree, with other sessions' files.
- Run that line alone and read its answer before `git add`. Do not gate on `$LASTEXITCODE`: a `partial: true` answer exits 1 on purpose. `--scope all` is `git diff HEAD`, so untracked new files do not appear, and a task that only adds files gets `No changes detected`; treat that like a PARTIAL answer on new files.
- The index is stale. A PARTIAL answer on new files is accepted with a grep confirmation stated in the commit body. No re-index.
- If the tool crashes, closes the connection, answers `not found`, or prints only its banner, use `graft` callers plus grep and say so in the commit body.
- Rename symbols only with the `rename` tool.

**Worktrees:**
- One per batch: `.worktrees/storymaker-p2a`, `-p2b`, `-p2c`, `-p2d`, each on a branch of the same name from current main.
- Check `git -C <main checkout> status --short -- <paths>` at task start, because another session may be doing the same work.
- Remove a `node_modules` junction as a link only, before `git worktree remove`. Check `(Get-Item -LiteralPath $p -Force).LinkType -eq 'Junction'`, then run `[System.IO.Directory]::Delete($p,$false)`.

**Commits:**
- One commit per task, inside the worktree, with the worktree's own index. The planning commit is already on main.
- `docs/` paths need `git add -f`. A new nested `AGENTS.md` needs `-f` too.
- Trailer on every commit: `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.
- Never push unless the user asks.

**Opening a batch:**
1. Check the previous batch landed: `git merge-base --is-ancestor storymaker-p2<x> main`. Batch 2A has no previous batch.
2. Create the worktree from main: `git worktree add .worktrees/storymaker-p2<y> -b storymaker-p2<y> main`.
3. Link `node_modules` as a junction to the main checkout's folder, if the batch runs `npm` or `node --test`.

**Closing a batch, after landing:**
1. Remove the `node_modules` junction as a link only, as the Worktrees rule says.
2. Run `git worktree remove .worktrees/storymaker-p2<y>`, then `git branch -d storymaker-p2<y>`.

**Landing a batch:**
1. Rebase on main inside the worktree. In 2D first run `git restore ui/js/desktop/bundles/main.bundle.js`, because E3f left it rebuilt and uncommitted.
2. Run `npm run build:ui`. Commit the regenerated bundles as the last commit only if `git status` shows a bundle diff.
3. Run the batch matrix.
4. Confirm the main checkout is on `main` with `git -C <main checkout> branch --show-current`.
5. Check `git merge-base --is-ancestor main <branch>`. If main moved during the matrix, rebase and rebuild, then rerun: the whole matrix when main's new commits overlap the batch's files, otherwise only the lines that cover the packages main changed.
6. Compare `git -C <main checkout> diff HEAD --name-only` with `git diff main...<branch> --name-only`. Also check `git ls-files -v AGENTS.md`: a lowercase `h` means an assume-unchanged flag hides its edits.
7. Check that `.git/index.lock` is absent.
8. Run `git -C <main checkout> merge --ff-only <branch>` as its own step, never chained after the checks with `;`.

Never stage in the shared index and never autostash. A stale index lock is removed only with the user's OK.

**Runtime and tooling limits:**
- No external assets at runtime. Fonts and SFX ship locally. `TestShippedWebAssetsLoadNothingExternal` and `TestStorymakerAppLoadsNothingExternalAndAvoidsFilteredClasses` must stay green.
- Temporary scripts go into the session scratchpad, not the project.
- Write and Edit decode backslash-u sequences. Write fixtures with U+00A0, U+3000 or surrogate pairs through a node script, and use `\x5c` in Go sources.
- Bash heredocs mangle backslashes: write scripts with the Write tool, then run them.
- Stop only your own PIDs, never by image name.
- The 5.1.9 runs use one `debian:12 --rm` container of your own. Do not touch other containers.

## 6 Review ledger

Each finding that touches a row, matrix or rule of this outline, with what I opened. Findings about frozen text alone were decided in frozen section 9. Their outline effect is the changed test or file in the named row.

**Applied as reported**
- Critical, WAVs only in a session folder: planning commit copies them. Checked the scratchpad list and `<evidence>/sfx.md`. Both mixers have their own `go.mod`, so `./...` skips them.
- Important, M5 misses a pin: `pt/02-stage-parts-text.mjs:100`, `:104`.
- Important, A17 and A21 miss `rn/render_test.go`: lines 265, 306, 535, 660, 665, 671, 679, 688, 691, 706, 718.
- Important, A24 misses the API test: `srv/storymaker_api_test.go:528`, `:534`, `:539`, `:546-551`. The checks at `srv/storymaker_handlers.go:543-550` stay ahead of `Prepare`.
- Important, E3d before E3e loader entry: `ui/desktop_storymaker_guards_test.go:213-217`.
- Important, N10, N12, N15 and A14 use later symbols: stub in N10, music key stub in N15, `MusicMeta` to A5, service file in N12. I added the same fix to N14 (`voices` cache) and N17 (`writeStorymakerErrorWith`).
- Important, E3f and I2 bundle rebuild: `scripts/build-ui-bundles.js:25`, `:64`, `:114`. Landing step 1 adds a restore, because a rebase refuses a dirty tracked file.
- Minor, W3 pin: `pt/09-host-render.mjs:183`.
- Minor, dirty-file table: `module-loader.js` is not in `scripts/build-ui-bundles.js`, and `ui/desktop.html:131` loads it alone.
- Minor, locale scope and exact list: `ui/desktop_storymaker_i18n_test.go:81-85`, `:92`.
- Minor, matrices: `ui/external_assets_test.go:24`, config-merger and setup, gofmt and vet lines.
- Minor, W droppable: E1j writes the boot document without the three fields; line 658 and the fields are conditional in I4.
- Minor, owner column: regenerated from the Files column.
- Minor, contract slices: every entry of frozen section 7 now has a slice. I moved header lines 1 and 5 from I4 to M16, because the precedence sentence must hold from the first phase 2 text. Line 519 moved from E1j to E2f, because `mixing` first runs in 2C.
- Minor, I6 steps: written from the local scratch instance and Chrome launch notes.
- Minor, planning commit and moved main: first commit of `storymaker-p2a`; landing step 5.
- Minor, M9 and S4 test coverage against spec line 800: tables widened.
- Minor, E2f and E1j section 4 split: applied through the slice list.
- Row effects of the frozen fixes: M7 (wait), M10 (`built` literal), M12 (pulse on text), M13 (memo key), G3-G5, G7, N1-N6, N9-N16, N18-N21, S1, S3, S4, S7, S8, A5, A6, A9, A13, A15-A18, A20, A22, A23, A25a-c, E1a, E1b, E1d-E1f, E2a, E2b, E2d, W1.

**Applied with a change**
- Critical, A3 commits entries that A4's rule accepts: confirmed at `sm/catalog.go:289-290`, `:334`, `:522`. Rows now run A4, A2, A3.
- Important, A2 and G1 guards: confirmed at `ui/storymaker_catalog_test.go:327`, `:767-787` and `assets/storymaker/test/catalog.test.mjs:20`, `:23`, `:24`. A2 no longer adds output. A3 adds WAVs, entries and test changes in one commit, so no red state appears between them.
- Important, S5 and A6 mini pins: confirmed at `pt/00-fixtures.mjs:29`, `:31` and `sm/catalog_test.go:233`, `:236-244`. A6's new counts follow the fixture and are derived in the task. The 30 follows from the 3 added thumbs.
- Important, M11 pins and minor M11, M2 values: the list is split into changing and staying pins. Added `pt/08-host-app.mjs:137`, which no finding named. Line 373 and `defaults_test.go:35` removed.
- The hand calculation behind 341 frames rests on rule b having no floor, which the controller settled in frozen 4.6 step 7. See "Settled: `with:"prev"` has no floor". I checked the speed in `catalog-mini/catalog.json` and the rules in `pl/beats.js`.
- Important, S2, S3, S4 reds: `pt/04-compile.mjs:24`, `:57-59`, `:97-99`. Lines 59 and 99 moved into S2, so S2 emits the new event shape and adds `pl/compile.js`.
- Important, S1 pin and S7, S9 font keys: `pt/04-compile.mjs:150` changes when compile adds the key of `text.subtitle`, so it sits on S9, not S1. The mini catalog has no glyph data, so S1 adds no key there. `pt/09-host-render.mjs:104-105` stays, because the code scene already needs that face (`pt/07-workers.mjs:101`) and fonts cache per key.
- Minor, N18 line lists: `pt/09-host-render.mjs:35`, `:41-42`, `:48`, `:103`, `:157`, `:173`, `:175`; `rn/render_test.go:136`, `:203`, `:208`; `rn/browser_smoke_test.go:458-468`. Frozen 4.8 step 1 sends `t` only with a token, so the stub paths stay valid and only `makeServer` learns to strip `t`. The fake page in `rn/render_test.go` needs the token, because it talks to the real listener.
- Added beyond the findings: G3 needs `IsJSSpace` before N1, so G3 opens `sm/narration.go`. A20 needs `AddWarning` for `music_silent` before A22, so that moved. `ApplyCaptionOverride` follows its frozen `[2B]` tag into N6, and A21 only calls it.

**Rejected**
- Minor, regrouping of A25, E2f and E3g into new closing groups: rejected, because every task keeps its writing group. The splits stay: A25a, A25b, A25c in G-A-6 and E2f, E2g in G-E2-1. E3g keeps its stated dependency on E3a-E3f.
- Important, audition as a new route: not needed. Frozen section 5 plays a stored piece, so E1e tests that and no paid route exists.
- Important, S1 and S4 "G2 lists the test only for licences": G2 keeps its own licence check, and the reference change sits on G1.