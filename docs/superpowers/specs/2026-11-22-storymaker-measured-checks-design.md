# Storymaker measured checks and cue by phrase

Status: design approved by the user on 2026-11-22, spec awaiting review. No plan and no code yet.
Builds on `2026-11-01-storymaker-design.md` (Player runtime, the phase-3 pixel rules), `2026-11-06` agent core and
`2026-11-09` quality work. To be built before `2026-11-21-storymaker-creation-modes-design.md` (both change
`studio_check.go` and the agent profile; free mode then inherits `flash` and the code-scene frame checks).

## Goal

Give the agent's check and the export measurements that need no vision model, so that weak models get findings they can act
on, and make cue timing robust for models that miscount `{n}` markers:

1. Picture checks on what the check already renders: text off the frame, text in platform overlays, text contrast,
   flashing, frozen or empty code scenes, a slow first scene.
2. Cue by phrase: the agent names a phrase of the narration instead of numbering markers.
3. Audio checks: dropouts and clipping in synthesized speech, and the balance and health of the final mix.

## Source of the ideas

The ideas come from a comparison with raea (github.com/shinyobjectz/raea, snapshot 4202a8785, 2026-11-22): its
`raea check` quality floor (`apps/check`: contrast, platform safe zones, audio defects, bed balance), its phrase cues
(`sdk/video/src/phrase.ts`) and its rule that every check must fail a cheat before it is trusted (`docs/direction.feature`
DR9). raea is under FSL-1.1-ALv2, which does not fit AuraGo's MIT licence: no raea code is copied. The rules below are
written new from the described behaviour, with numbers from raea, WCAG 2.3.1 and the phase-3 pixel rules of the
2026-11-01 spec.

## Decisions the user made (do not re-ask)

1. Mixed strictness. `text_offframe` and `flash` are hard: the agent gets them as checks to fix, and if one is left the
   job ends `incomplete`. They keep severity `warning`, so a person's own export is never refused for them. Every other
   new picture finding is advice.
2. One safe zone per aspect, no target-platform setting.
3. Measure where the data already is: Go on the stills of the check, a frame probe in the check's browser page, the mix in
   the export. Not a post-export FFmpeg analysis (the agent could not fix hard findings any more).
4. Cue by phrase at the tool edge: the Studio turns a phrase into a `{n}` marker. The story format does not change.
5. The burned caption in 9:16 moves up, so it stays clear of platform overlays.
6. A phrase that occurs twice resolves to its first occurrence; the answer says so.
7. A synthesized piece with a dropout or clipping is synthesized once more; if the fault stays, a warning is kept. Audio
   findings never change the outcome and never refuse an export.

## Picture checks

### New codes

| Code | Measured | Rule | Class |
|---|---|---|---|
| `text_offframe` | Go, layout text boxes | A text (not the caption) has more than 1 % of its box outside the frame (`layoutTextOutside`). Split off from `offscreen`, which stays for actors and props. | hard |
| `flash` | page frame probe | More than 3 flashes in any 1 s window (rule below). | hard |
| `flash_unverified` | page frame probe | The probe did not reach every frame within its deadline. | open check |
| `text_contrast` | Go, the still of the scene | Contrast below 3:1 inside a text or caption box (rule below). | advice |
| `text_unsafe` | Go, layout text boxes | More than 2 % of a text or caption box lies in the aspect's zone (table below). Not reported for a text that has `text_offframe`. | advice |
| `dead_stretch` (code scene) | page frame probe | More than 3 s of frames whose mean absolute difference per colour channel to the previous frame stays below 1 % of full scale. | advice |
| `first_frame_empty` (code scene) | page frame probe | The first frame after the entry transition is one colour: every pixel's channels lie within 1 % of full scale of the frame mean. | advice |
| `slow_hook` | player lint | Scene 1 has no narration, or its first show beat lands later than 0.5 s after the film start. | advice |

`dead_stretch` and `first_frame_empty` exist today for storyboard scenes, from the timeline. The code-scene versions are
the phase-3 rules of the 2026-11-01 spec, now measured on every probed frame instead of a 0.5 s sample.

### Safe zones

Fractions of the frame, measured from each edge:

| Aspect | Top | Right | Bottom | Left |
|---|---|---|---|---|
| 9:16 | 0.115 | 0.130 | 0.219 | 0.056 |
| 16:9 | 0.05 | 0.05 | 0.05 | 0.05 |
| 1:1 | 0.05 | 0.05 | 0.05 | 0.05 |

The 9:16 row is the union of TikTok, Reels and Shorts overlays (raea's default 220/140/420/60 px at 1080 x 1920). The
16:9 and 1:1 rows equal the stage's existing safe area (`stage.js` `DEFAULT_SAFE`).

### The caption in 9:16

`render.js` `captionLayout` puts the caption box bottom at 82 % of the height in 9:16 today, inside the bottom zone. It
moves to 78 %, and in 9:16 the caption's maximum width comes from the zone (centred in the safe band between 5.6 % and
87 %) instead of the stage safe area. Existing 9:16 films show the caption a little higher on their next export. 16:9 and
1:1 keep 92 %, which is inside their zone. A unit test holds the 9:16 caption box inside the zone for a two-line caption.

### Text contrast

On the still of each storyboard scene (the hold frame the layout run measures, `compilestills.go`), for each text box and
the caption box shown at that frame: the 3rd and 97th percentile of relative luminance over a grid of at most about 20 000
samples inside the box give the ratio `(L97 + 0.05) / (L3 + 0.05)`. Below 3:1 is reported. Below 1.15:1 means the text is
masked or not drawn and is not reported. 3:1 applies to every text, because Storymaker text is never smaller than 2.6 % of
the short side (`lint.js` `FONT_FLOOR`, at least 28 px at 1080), which is WCAG's large text. The layout run gives no text
colours, and none are needed: the percentiles find text and background.

### The frame probe and the flash rule

In the check's browser page, after the layout run, the probe renders every frame of the film at 64 px width (the player's
existing `opts.width` path; code scenes through their own frames) and records per frame the mean relative luminance `Y` and
the mean absolute difference per colour channel to the previous frame. `checks.js`, empty since phase 1, applies the rules
and returns the findings in the compile report's `checks` list, as the contract planned. Go maps them to issues; it does
not recompute anything.

A transition is a monotone run of `Y` whose total change is at least 0.10 and whose darker end is below 0.80. A flash is two
consecutive transitions in opposite directions. More than 3 flashes starting inside any window of `fps` frames gives one
`flash` finding per scene, at the window's start time.

The probe has a deadline of 20 s inside the existing compile deadline. Frames it does not reach make `flash_unverified`,
listed like `layout_unverified`: an open check (`ready_unverified`), never `incomplete`. The first plan task measures the
probe's cost on a long film; if a 90 s film does not fit, storyboard scenes are probed only within 0.5 s of each cut, because
inside a storyboard scene beats move single pieces and fades are slow, so the whole-frame mean cannot alternate quickly.

### Coverage

The text checks run for storyboard scenes only. Inside a code scene the text boxes are unknown, so a code scene stays
`layout_unverified` as today. The probe covers storyboard and code scenes.

### Outcome and classification

- `text_offframe` and `flash` are model-fixable (`IssueModelFixable`), outside the craft and layout classes, with severity
  `warning`. The check puts them into the agent's `issues`, they spend check rounds like validation errors, and
  `DecideOutcome` treats a left one as blocking (`incomplete`).
- `text_contrast`, `text_unsafe`, `slow_hook` and the code-scene `dead_stretch` and `first_frame_empty` join
  `IssueAdvisory`: one optional fix loop, never listed as open checks, never blocking.
- `flash_unverified` is listed as an open check and never blocks.
- In the editor's compile report and the export warnings, all new codes are warnings. Each gets its locale key
  `storymaker.issue.<code>` in all 16 `ui/lang/storymaker/*.json` files (a real translation each) and a row in the
  contract's issue table.

## Cue by phrase

The agent may give a beat `cue_phrase` (1 to 60 characters) instead of `cue` in `put_scene` and `patch_scene`. Both on one
beat are refused. Before the scene is saved the Studio resolves the phrase:

1. Search the scene's narration with its markers stripped; if the same call changes the narration, search the new text.
   Matching ignores case (Go case folding), punctuation and repeated whitespace; umlauts and accents stay as they are.
2. In Latin, Greek and Cyrillic text the phrase must start and end at word boundaries. In CJK text a substring is enough.
3. The first occurrence wins. If there are more, the answer says so in one sentence.
4. Insert a `{n}` marker right before the match: the next free number, or the marker already standing there. Set
   `cue: n` on the beat and drop `cue_phrase`.

The saved story is plain `storymaker/1`. The player, the Go validator, the editor and the contract do not change.
Narration is synthesized from the text with its markers stripped (`StripMarkers`), so a new marker never causes a new
synthesis.

A phrase that is not found is refused with `cue_phrase_not_found`, in the existing `studioRefused` shape. The answer names
the closest real word sequence of the same word count from that narration (smallest edit distance over normalised words),
and the refusal counts against the per-scene and per-job budgets like any refused write.

The agent profile (`storymaker_agent_profile.go`), the reference guide `animation.md` and `prompts/tools_manuals/storymaker.md`
teach `cue_phrase` only. Markers in a narration are the Studio's; the agent keeps them when it resends a narration, as
today. The beat schema (`internal/agent/native_tools_storymaker.go`) keeps `cue` for existing stories and gains
`cue_phrase`. Shorter descriptions pay for it, so the 900-token schema test keeps passing.

Code scenes keep `api.cue(n)`.

## Audio checks

### Synthesized pieces (at narration)

`storymakerDecodePiece` (`internal/server/storymaker_narrate.go`) holds the decoded samples of each piece briefly. Two
checks run there:

- Dropout: inside a speech run of the piece's envelope, one 10 ms step falls by at least 35 dB, from at least -45 dBFS to
  below -80 dBFS, and the level stays below -80 dBFS for 50 ms to 3 s.
- Clipping: a run of at least 3 consecutive samples with |x| >= 0.999, or of at least 4 consecutive equal samples with
  |x| >= 0.05 (flat tops that were scaled down). A piece counts as clipped with 3 or more runs inside any 1 s.

A faulty piece is synthesized once more; the retry is charged to the daily character budget. If the new take still fails,
it is kept, and the narration result carries `narration_dropout` or `narration_clipped`, shown on the agent job card and in
the export warnings. Neither changes the outcome or refuses an export.

### The export mix

Measured in the mix pass on the per-stem blocks (`render/mix.go`), so no stem file is needed:

- `bed_too_loud`: in a 6 s window in which the narration stem is active for at least half the time, the music stem's level
  (RMS after all gains) is less than 12 dB below the narration stem's. One warning with the first window's time. The mix
  aims at about 18 dB under speech (bed 12 LU under narration plus 6 dB ducking); the warning catches beds with loud swells.
- `bed_gap`: the music stem holds steady for at least 1 s (spread of its 10 ms levels below 3 dB) and then falls by at
  least 12 dB in one 10 ms step, outside the bed's planned fade-out.
- `mix_clipped`: the mix clamped at least one sample (`MixResult.Clamps`, counted today but reported nowhere).

These are export job warnings like `music_silent` (`render/jobs.go`): written to the export record and listed by
`storymaker-export.js`. The bed level stays at 12 LU under narration; raea's quieter 15 to 20 LU is a listening question
for later.

## Tests (lean, as in the last batches)

- Cheats: every new code has a fixture that must raise that code and no other new code. One clean fixture raises none.
- Go `internal/storymaker`: contrast on stills built as images, zones and `text_offframe` on literal layout boxes, the
  phrase resolver (German, English, CJK; duplicate, missing with closest phrase, both fields refused, marker reuse),
  classification and `DecideOutcome` for the hard codes and `flash_unverified`.
- Go `internal/storymaker/render`: `bed_too_loud`, `bed_gap`, `mix_clipped` on synthetic PCM.
- Go `internal/server`: piece dropout and clipping on synthetic samples, the single retry, the warning; the profile names
  `cue_phrase`.
- Go `internal/agent`: the beat schema has `cue_phrase` and stays within the 900-token cap.
- JS: the flash rule on a luminance series, the code-scene `dead_stretch` and `first_frame_empty` rules, `slow_hook`, the
  9:16 caption box inside the zone.
- Real browser (`AURAGO_RUN_BROWSER_SMOKE=1`): one check run with a strobing code scene (raises `flash`) and a 9:16 caption
  inside the zone.
- Locale keys in all 16 languages (`check:ui`, the static i18n test, the Storymaker locale tests).
- Docs: `documentation/manuals/{en,de}/34-storymaker.md`, `documentation/storymaker.md`,
  `prompts/tools_manuals/storymaker.md`, the contract's issue table and compile-report `checks`.
- Acceptance with a real model on aurago-test is the user's job; agents never deploy to or write on that server.

## Out of scope

- Text checks inside code scenes (the code-scene API would have to report text boxes): a follow-up for free mode.
- `api.cue("phrase")` in code scenes.
- `slow_open`, 4:5, a target-platform field, other loudness targets, a quieter bed.
- Comments, a review loop, findings with a history across runs.

## Files expected to change

`internal/storymaker`: model.go, issues.go, layout.go, stills.go (or a new contrast.go), studio_check.go, agentjob.go,
studio_story_scenes.go (or a new cuephrase.go), narration.go, reference/animation.md and tests.
`internal/storymaker/render`: compilecheck.go, report.go, mix.go, jobs.go, render.go and tests.
`internal/server`: storymaker_agent_profile.go, storymaker_narrate.go and tests.
`internal/agent`: native_tools_storymaker.go and tests.
`ui`: js/storymaker/checks.js, compile.js, host-render.js, worker-player.js, render.js, lint.js, the generated bundles,
lang/storymaker/*.json, js/desktop/apps/storymaker-export.js if the warning list needs the new codes.
Docs as listed above.
