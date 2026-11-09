# Storymaker output quality — design

Date: 2026-11-09. Status: approved by the user on 2026-11-09 (design presented in sections; the user chose "both stages in one plan", then "write the spec"). Executed on 2026-11-09 in batches A to E; see Built.

Related documents: `docs/superpowers/specs/2026-11-01-storymaker-design.md`, `docs/superpowers/specs/2026-11-01-storymaker-contract.md`, `docs/superpowers/plans/2026-11-06-storymaker-agent-core.md` (the agent core this work improves).

## Goal

A film the in-app agent makes should look and sound like a deliberate explainer, also when the author model is weak: varied scenes, readable composition, motion with a reason, and checks the agent can act on. The user named three weak areas: scene look, motion/timing/sound, and reliability. Story and text quality is out of scope, except the umlaut defect that breaks captions and speech.

## Evidence

Read-only inspection of aurago-test on 2026-11-09 (nothing changed there):

- Five agent jobs (2026-11-07 to 2026-11-08), all with provider agnesai, model `agnes-3.0-flash`, 7 scenes, 55 to 60 s planned, 145k to 246k tokens each. All ended `ready_unverified` with open checks `layout_unverified` and `compile_unverified`. Every `check` event in the job logs says `compile: "unavailable"`. In 4 of 5 jobs the check reported `advice: 0`. Each scene was written as `put_scene` (answer "draft") followed by `patch_scene`; 2 to 3 refusals per job.
- Only one project survived: "was ist eine firewall" (7 scenes, exported 42.2 s, 1920x1080, 30 fps). Its story: all 7 scenes `env.plain-stage`, camera `wide` (the default), one actor at `left` with pose `present`/`stand`, gesture `point` in 5 of 7 scenes, one prop at `right` per scene, two cast members but the second never appears, every cue is 1, one floating text card "ABGEWIESEN".
- Measured in a full-size export frame: the actor is about 43 % of the frame height. The caption box (top at about 79.5 % of the height) covers the actor's feet and floor-level props. Props are 9 to 16 % of the height by catalog size and are never enlarged.
- The narration has umlauts missing: "pruft", "durfen", "personlichen", "wahrend", "stromen". Captions and speech carry the errors. `LintWords` and validation do not flag them.
- Planned 55 s, exported 42.2 s (the 2.3 words per second estimate is too slow for the chosen voice).
- Sound (earshot, fast critique): 18 of 21 metrics in range. Integrated -17.2 LUFS (web target about -14), salient hit density 2.06 per second (range up to 1.86), nearly mono (side -28 dB). Not judged by ear.
- The five jobs ran before commit `e6ff84295` (author packets, stage lints, prompt profiles; 2026-11-08 16:46). aurago-test has run commit `145c4ad05` since 2026-11-09 00:26, which contains it, but no agent job has run since. Whether that commit already helps is unknown.

## Root causes

| Symptom | Cause (file:line as read on 2026-11-09, verify in the plan) |
| --- | --- |
| Every scene `wide` | `ApplyDefaults` sets framing `wide` when `camera` is absent (`internal/storymaker/defaults.go:103-109`). The player has no automatic camera move (`ui/js/storymaker/beats.js:141-159`, `camera.js`). |
| Every scene `plain-stage` | The default environment is `plain-stage` (`defaults.go:12`); an omitted environment copies the previous scene (`defaults.go:71-77`); the first scene starts on it (`project.go:140-141`). The fill prompt asks for continuity (`internal/server/storymaker_agent_profile.go:62`). The packet lists all 12 environments in catalog order with spot names only (`studio_packet.go:333-340`). The prompt says "stageable environments" but no such filter exists. |
| Same pose, gesture and prop spot | The worked example in the packet and in `reference/animation.md` is `office`, `present`, `point`, prop at `right` (`studio_packet.go:104-119`). |
| Monotony passes the check | `LintForJob` knows only `scene_unfilled`, `fade_between_worlds`, `entry_monotony` (warning; fires only when the streak is exactly 3 and is not reset per scene), `over_budget`, `spot_overlap` (`studio_check.go:69-208`). No lint for environment, framing, gesture, unused cast or prop count. Warnings become advice, which gets at most one loop per job. |
| Captions on the stage | Captions: bottom at 0.92 H, font 0.042 H, up to 2 lines (`ui/js/storymaker/render.js:315-327`). Floor spots: feet line y 88 (`assets/storymaker/lib/environments.mjs:13-55`). No caption safe area exists (`stage.js:12` has only the generic safe area). |
| Props small | Catalog `size.h` 0.09 to 0.16 of the frame height (`ui/img/storymaker/catalog.json`). `Beat` has no `scale` field (`internal/storymaker/model.go:235-263`), so a `show` beat cannot enlarge a prop; only scene-level `Prop.Scale` exists (`model.go:215,224`). |
| `compile: unavailable` | `CompileCheck` needs a ready browser (`render/compilecheck.go:57-76`). The pinned Chromium is downloaded only by the export (`render/browser.go:173-177`, called at `render.go:437`); the agent job never calls it. Reasons are discarded: only a constant `compile_unverified` reaches the outcome (`agentjob.go:81-83`), the event carries counts only (`storymaker_agent.go:644-646`), and only two paths log. |
| `layout_unverified` always | `DecideOutcome` always adds it (`agentjob.go:71-119`). The compile report carries durations and cues only, no geometry (`render/report.go`, `compile.js:208-213`). |
| Umlauts | No word list or repair exists (`studio_words.go`, `validate.go`). |

## Design

Two stages. Stage 1 needs no browser to test; stage 2 builds on the compile run.

### Stage 1: craft in the system

**1.1 Direction packet.** After the outline is accepted, the server computes a `direction` for each scene and adds it to the fill context: the scene's role (hook, problem, solution, how, benefit, close; from the outline arc), a suggested environment, a suggested framing, and a hero prop hint (from the existing `suggested_props` search).

- Each of the 12 catalog environments gets a one-line description and a few tags (curated by hand, in `internal/storymaker/data/env_direction.json`). The server ranks environments by tag overlap with the request and the outline narration, picks up to three distinct ones for the film and assigns them by act (hook and problem, solution and how, benefit and close). Ties rotate by a hash of the title, and `plain-stage` ranks last. The packet lists only spots that exist for the chosen environment.
- The packet's `environments` list shows the description. The model may deviate; the lints in 1.2 guarantee the minimum.
- The worked examples rotate between two or three different shapes (different environment, pose, gesture, spot). The sentence "maintain visual continuity with previous_scene" becomes "keep the same environment only while the scene's role stays the same". Prompt texts get new version names (`storymaker/v3/...`), as the profile code demands.

**1.2 Lints that the model must fix.** They run for create jobs only, from `craftIssues` in the check (`studio_craft.go`), not from `LintForJob`; the editor is unchanged. They apply to films of at least 4 scenes; the `dialogue_story` template is exempt from the environment rule.

- `environment_monotony`: fewer than `min(3, ceil(n/3))` distinct environments.
- `framing_monotony`: every scene `wide` and no `camera` beat in the film (after 1.3 the push-in is automatic, so this lint catches a film that never chooses any framing).
- `gesture_monotony`: one gesture used in more than `max(2, ceil(n/3))` scenes.
- `cast_unused`: a cast member who never appears in `actors` or as `speaker`. The fix text names two options: use the member or remove it from the cast. The fix goes through `patch_scene` only: the member gets a part in one scene. No `set_meta` call is needed.
- `entry_monotony` is repaired: reset per scene and fire for every streak of 3 or more.

These codes are model-fixable while check rounds remain (`StudioCheckRounds` = 6), but they never turn the outcome into `incomplete`: what is left at the end becomes an open check on a `ready_unverified` job. The mechanism is the class `IssueCraft` beside `IssueModelFixable` (`issues.go`), with `issueNeverBlocks`, which keeps these codes out of `incomplete`.

**1.3 Composition v2 in the player.** Active only for stories with `style.compose = "v2"` (see Compatibility).

- Caption-safe floor: with captions on, base spot feet lines move above the caption band (feet line y 74, `V2_FEET_Y` in `stage.js`, for 16:9 and 1:1, tuned against measurement; 9:16 gets no lift). The goal is stated as a rule: no actor foot and no prop base inside the caption band. The depth scale of the floor is adjusted so actor size stays about as it is now.
- Hero prop: the first cue-bound `show` of a scene is normalised to about 24 % of the frame height through the catalog `size.h` (a scale between 0.5 and 3, applied by the player, not stored). An explicit `scale` wins. `Beat` gets `scale` (validated 0.5 to 2), is part of the tool schema, and is documented in `reference/animation.md`.
- Automatic push-in: a scene without its own `camera` and without a non-wide framing zooms slowly (about 1.0 to 1.08 over the scene, ease-in-out) onto the speaker, or onto the hero prop at its cue. An explicit camera beat or framing switches it off for that scene. Rhythm `calm` halves the amount.
- Medium framing, when the caption-safe floor is on (compose `v2`, captions on, not 9:16): the player places the first actor with `SM.compose.mediumFocusY` (`MEDIUM_FEET_ON_SCREEN = 0.78`), so the feet stay above the caption. A camera target switches this off.
- Packet and prompts mention `scale` and the camera so the model can use them deliberately.

**1.4 Umlaut repair.** For language `de`, `put_scene`, `patch_scene` and `outline` replace whole words from a curated map when the narration, or the text of a `show` or `say` beat, is stored ("pruft" becomes "prüft", "durfen" becomes "dürfen", "wahrend" becomes "während", "personlichen" becomes "persönlichen", "stromen" becomes "strömen"). Rules: whole words only, case kept, only entries whose ASCII form is not itself a German word (so "schon" stays out). The map is a small embedded file with a test that checks every entry, and it is easy to extend. Other languages are untouched. A repair is reported in the tool answer so the model learns, and counted in the job report. The dash replacement the model already does by itself stays a lint warning.

**1.5 Compile check that runs.**

- Before the first check round the agent job makes the browser ready (the same `Ensure` the export uses), when the desktop is not read-only (`VirtualDesktop.ReadOnly` is the only gate; no config key exists for the download). It asks at most twice per job. A failed setup backs off for 15 minutes (`storymakerBrowserBackoff`), and the job continues without the browser.
- The unavailable reason is logged (including the browser status issue text), carried in the `check` event as `compile_reason`, and stored in the job's open check as a detail next to `compile_unverified`. The job card shows it.
- Optional, to be confirmed with the user in the plan: raise the export loudness target from about -16 to about -14 LUFS and soften the density of automatic SFX hits. Not part of the approved design.

### Stage 2: see and correct

**2.1 Layout report from the player.** `checks.run` in the player (`ui/js/storymaker/checks.js`, currently returns `[]`) is filled in. During the existing compile run, for each storyboard scene it samples one hold frame (after the last cue arrival plus half a second, at most the scene's last frame) and reports normalised rectangles of actors, props, text cards and the caption box (`entityBox` and `textBox` in `render.js` are exported for this; the caption box is measured from the canvas text metrics). The compile check reads a `layout` array (`render/compilecheck.go`) into `CompileSummary.Layout`, one entry per storyboard scene; blocks for unknown scenes are dropped and each kind is capped at 64 boxes. Go turns it into warnings (`LayoutIssues` in `layout.go`), model-fixable and never blocking:

- `caption_overlap`: an actor foot or prop base inside the caption band.
- `prop_too_small`: the hero prop under 12 % of the frame height.
- `offscreen`: a box outside the frame.
- `actor_prop_overlap`: an actor and a prop or card overlap by more than a small share.
- `scene_static`: nothing shows, moves or changes between a scene's first and hold frame, apart from the idle cycle.

The player runs the layout after `prepare()` in a worker message `layout`, behind a boot flag that only the compile check sets. The caption rule applies to compose `v2` with captions on, and not in 9:16; it skips scenes with a close framing, a camera target or a camera beat. Code scenes are not checked (they keep `layout_unverified`). `ready` means the open-check list is empty (the constant exists: `studio.go:29`); `layout_unverified` is added only when a code scene exists or the check did not run.

**2.2 Stills and contact sheet.** In the same browser run as the compile check, one still per measured scene is captured (the hold frame, `render/compilestills.go`, skipped when the check has an error issue), assembled into a contact sheet PNG (4 columns, 480 px thumbnails) and stored with the project as review evidence; the job card links it. The same stills give `scenes_look_alike`: a difference hash (64 bits) whose neighbour distance is 16 bits or less for three or more consecutive scenes is a warning of the layout class, model-fixable and never blocking. 16 was tuned on the real firewall run, whose neighbour distances are 0 to 11 bits, and on the varied story, whose are 24 to 39 bits.

**2.3 Vision review (dropped).** Not built. The AI policy of the Agnes provider refuses local images (`internal/tools/vision.go`, lines 144 to 153), so the contact sheet cannot go to the model; the contact sheet is the evidence for the user instead. The design below is not built: when the provider reports image input and the setting `storymaker.agent.vision_review` is on (default off), the contact sheet goes to the model once, in the check phase, with a fixed review question in the style of the six-question jury from the cinecode review ("at most three fixes"). If the agent loop cannot carry an image part, this item is dropped and the rest of stage 2 stands. The plan checks this first.

## Compatibility

- `style.compose` is a new optional story field (`"v2"`; absent means the current behaviour). `Style` is at `model.go:136`; validator, defaults, canonical JSON and the contract get the field. New projects created by the agent get `"v2"`; projects made in the editor and all existing stories stay on the current look, so finished films do not change when re-rendered. A toggle in the editor is out of scope.
- `Beat.scale`, `style.compose`, per-scene `layout` in the compile report and `compile_reason` in the check event are additive. The additive-values rule of the contract applies; the contract and the interface docs are updated in batch B and D.
- The check lints and layout issues are advice-or-fixable only for agent jobs; the editor's own lint list does not change.

## Testing and acceptance

Lean tests (user's rule): per task its own tests, one combined run per batch end.

- **Fixtures.** The real firewall `story.json` goes into `internal/storymaker/testdata` (taken from aurago-test, kept as is). The four new lints must flag it; this is the first red test. A hand-written story with varied environments, two cast members, medium framing and different gestures must pass all lints. Both are checked into the repo.
- **Unit.** Lints, direction packet ranking, umlaut map (every entry; ambiguous words untouched; other languages untouched), outcome rules, `Beat.scale` validation, report parsing with and without `layout`.
- **Player (node).** `npm run test:storymaker`: composition v2 positions, hero normalisation, push-in on/off, v1 stories unchanged (same stage coordinates as before), layout report geometry.
- **Browser.** `TestCompileCheckBrowser` extended for the layout report and the reason code; one render of the firewall story with `compose: v2`, which must pass the geometry check, and whose contact sheet I look at.
- **Acceptance by the user on aurago-test** (the plan writes the recipe like the F4 recipe of the agent core): one new run with their model on the same request "was ist eine firewall". Success is judged on the contact sheet: at least three different environments, both cast members used where the story has two, a readable hero prop in each scene, no caption over a foot, no `compile: unavailable` unless the reason is named. No agent deploys to aurago-test; an agent may read the result when asked.

## Batches

Each batch runs in its own worktree (`.worktrees/storymaker-quality-<x>`), lands on the user's word, and starts with the GitNexus impact check the project requires.

- **A** Lints, direction packet, environment descriptions and tags, prompt texts, fixtures, `entry_monotony` repair.
- **B** Composition v2 in the player, `Beat.scale`, `style.compose`, documentation of both.
- **C** Umlaut repair, browser readiness for the check, reason logging and job card detail.
- **D** Layout report and the new layout issues, outcome `ready`. Needs B.
- **E** Stills, contact sheet, `scenes_look_alike`, optional vision review. Needs D.

A, B and C are independent. The plan decides whether A and C run as one batch.

## Not in this work

New characters, environments or props (art); the quality of the story and its words beyond umlauts; the asset generator; checking code scenes; editor UI beyond the job card; calibrating words per second per voice (planned 55 s versus real 42.2 s) and the music or loudness mix, unless the user adds them in the plan stage.

## Risks

- More errors in the check loop cost tokens and rounds. The cap of 6 rounds stays, and the new lints never block the outcome.
- Moving the floor line changes every v2 scene's look; the depth scale and the hero normalisation are tuned together and must be viewed, not only unit-tested.
- A weak model may not fix a geometry error with `patch_scene`. The fix texts must name the exact field and value range, in the style of the existing `Fix` strings.
- The browser readiness step adds a download on a first agent run when the pinned Chromium is missing; it is skipped on a read-only desktop and shows progress in the job events.
- Other sessions work in the same checkout; commits go through a temporary index, landings through `merge --ff-only` after a rebase.

## Built

Batches A to E were built on the branches `storymaker-quality-a` to `storymaker-quality-e`, each in its own worktree. Nothing has landed on main, and nothing is pushed or deployed. The user decides each landing. The acceptance on aurago-test (see Testing and acceptance) is open. The optional loudness and sound density item of 1.5 was not built.

Commits per batch, oldest first:

**Batch A**

- 9103f1286 test(storymaker): add the real firewall run and a varied story as fixtures
- 94984383d feat(storymaker): craft issue class that never blocks the outcome
- cf86c6cab feat(storymaker): curated environment guide for the direction packet
- 6ebd58afd feat(storymaker): direction plan (role, environment, framing, gesture, cast, prop spot)
- 7c14472ef feat(storymaker): film-level craft lint (environments, framing, gestures, cast)
- 9be6d5013 fix(storymaker): reset the entry_monotony streak per scene and name only valid entries
- 5ab7e8990 feat(storymaker): the create job's check runs the craft lint
- ca49c55ec feat(storymaker): direction packet and a worked example built from it
- fc7ecbc15 feat(storymaker): agent prompts v3 (direction, film items, valid entry styles)
- de8549d98 feat(storymaker): job card line for open craft checks, document the craft lint
- a1a0ca577 fix(storymaker): keep every gesture finding and leave code scenes out of the craft lint
- 35689b2b4 fix(storymaker): no extra check loop for findings that never block, a different fill example in the prompt

**Batch B**

- 45f04e807 feat(storymaker): beat scale and style.compose in the model and the validator
- 70e4e4060 feat(storymaker): agent projects are created with composition v2
- 5e955388a feat(storymaker): beat scale in the agent tool schema and the animation reference
- 0aba65f3b feat(storymaker): caption-safe floor for composition v2
- 36c460358 feat(storymaker): hero prop and slow push-in for composition v2
- fc52bf323 docs(storymaker): composition v2 and beat scale
- 793fa36ce fix(storymaker): composition v2 keeps the depth of a point target and of walks

**Batch C**

- a17491e9a feat(storymaker): curated German umlaut repair
- acbdb7c03 feat(storymaker): restore missing umlauts when scene text is stored
- 42b25edfa fix(storymaker): drop stale umlaut repairs before a scene write
- e68ff4ef3 feat(storymaker): carry the reason of an unavailable compile check
- 9c9228979 feat(storymaker): make the browser ready before the agent's compile check
- 74d00d3c3 feat(storymaker): job report counts repairs, job card names why the compile check did not run
- 8daa478b1 docs(storymaker): browser readiness, compile reasons and umlaut repair
- b0855523c fix(storymaker): umlaut map keeps real German words, repairs counted only when stored
- 2cef710fa fix(storymaker): closed compile reasons, forget a cached unavailable compile when the browser is ready, safe reason lookup
- 2a2343009 fix(storymaker): back off after a failed browser download and let the download lock obey the context

**Batch D**

- fdb1e5abe feat(storymaker): the compile check carries the measured layout
- e1aedbbea feat(storymaker): layout run in the player (boxes of actors, props, texts and the caption)
- e4a74bc99 feat(storymaker): findings from the measured layout
- 034e7c195 feat(storymaker): layout findings never block, and a film with nothing open is ready
- c15a01e24 fix(storymaker): scrub and bound the strings of the layout blocks
- b255b4255 feat(storymaker): job card for a checked film and for layout findings, document the layout run
- 77e896e36 fix(storymaker): ready in the chat facade, no caption rule in 9:16, honest prop-size advice
- d62015a2e fix(storymaker): job card text that matches what was checked, ready accent, manual wording

**Batch E**

- 79be5150c feat(storymaker): difference hashes, contact sheet and the look-alike finding
- a9f438d8c feat(storymaker): the check reports scenes that look alike
- 563ea833a feat(storymaker): the compile check takes a still per scene in the same browser run
- 95b4294d0 feat(storymaker): save and serve the contact sheet of the agent's check
- 82fcfaca1 test(storymaker): real-browser acceptance of stills, caption rule and look-alike scenes
- bd4726828 fix(storymaker): tune lookAlikeMaxDistance on the real run
- 2cea7f28d fix(storymaker): composition v2 keeps the feet above the caption in medium framing
- 75cdbe678 fix(storymaker): the caption rule skips scenes with a close framing, a camera target or a camera beat
- bb258ecc5 docs(storymaker): measured actor height in the medium-framing comment
- f75443ec1 feat(storymaker): job card links to the contact sheet

The commit ids above are those of the stack rebased onto main `b475791cf`. Every later rebase changes them; the subjects stay.

### Review fixes of batch E

A reader review and a security review of batch E found five and three real problems. They were fixed in one round, after the commits above:

- 5e21b4a65 fix(storymaker): the job's review link follows the latest check, and a deleted project gets no sheet
- 3f16b8546 fix(storymaker): a look-alike run ends at a gap in the story order, and the layout index is the story's
- 1d0089b87 fix(storymaker): the contact sheet decodes one still at a time and stays within 16 million pixels
- 1e1fa53f5 fix(storymaker): a failed still leaves no stills, and the real-browser test runs the agent's layout rule

What changed: a check that yields no sheet clears `agent.review` instead of leaving a link to an older sheet. A look-alike run no longer reaches across a code scene or any other scene without a still. The layout index comes from the story order, not from the page. The contact sheet decodes one still at a time, shrinks its thumbnails (480 px down to 120 px) to stay within 16 million pixels and gives no sheet when even that does not fit. A still larger than 8 MB, or one that declares more than 16 million pixels, is refused before it is decoded. Saving a sheet for a deleted project fails instead of recreating its folder. The tests for these cases are new, among them a failed capture and `withReview` inside a real agent job.

### Known limits

1. Entity depth is one static value per scene. A move to an explicit point clears the depth for the whole scene, and in compose v2 the actor is slightly small before the move.
2. `exit` assigns no depth.
3. The asset `prop.clock` has an opaque square background (pre-existing).
4. 15 of the 16 locale texts added by the agent are not reviewed by native speakers.
5. The reasons `compile_setup` and `no_compile` show no text on the job card.
6. The 15 minute browser backoff also covers failures that a config change would fix.
7. `ready` end to end is covered by unit rows and the real-browser test, not by a scripted server test.
8. The contact sheet is not tied to a story version. After an edit it shows the last check until the next check replaces it. Two jobs on one project overwrite each other's sheet.
9. No vision review. Reinstating it needs a provider that accepts local images.
10. While the stills are taken, all screenshots of a film are held until the contact sheet is built. Each is capped at 8 MB, but a film of 200 scenes could in theory hold 1.6 GB; real 1080p screenshots are a few hundred KB.
11. Long films get smaller thumbnails on the contact sheet (at 200 scenes of 9:16 only 180 px wide), so the sheet is harder to read.
