# Plan decisions

The spec ([storymaker-quality-design](../../specs/2026-11-09-storymaker-quality-design.md)) fixed the goal and the two stages. Reading the code on 2026-11-09 showed gaps and conflicts in it. This file records how the plan resolves each one. Where this file and the spec differ, this file wins, and the last task of batch E (E7) corrects the spec.

Line numbers are as read on 2026-11-09. Verify a line before relying on it.

## Batch A: lints and direction

| # | Question | Decision | Why |
| --- | --- | --- | --- |
| A-1 | Where do the film-level lints live? | New function `LintFilmCraft(st *Story, template string, cat *Catalog) []Issue` in a new file `internal/storymaker/studio_craft.go`. `LintForJob` keeps its signature and keeps the per-scene lints, and gets the `entry_monotony` repair only. | `LintForJob` is also called by `packetIssues` (fill and revise packets), where a film-level finding means nothing. `LintForJob` has no template and no catalog. |
| A-2 | Which jobs get the craft lints? | Only `create` jobs (`StudioJob.Kind == AgentKindCreate`). A revise job never gets them. | A revise job must make the smallest change the user asked for. A film-level finding on an old story would send the model off to rewrite scenes nobody mentioned. |
| A-3 | Which class do the craft codes have? | New `IssueCraft(code string) bool` in `issues.go` for `environment_monotony`, `framing_monotony`, `gesture_monotony`, `cast_unused`. `IssueModelFixable` returns true for them, so they land in `report.Issues` and the check loop runs while rounds remain. `DecideOutcome` does not block on them (`blocked` ignores `IssueCraft`), but still lists them as open checks. Severity is `warning`. | Today every model-fixable item blocks (`agentjob.go:104`). The spec wants fix-while-rounds-remain and never `incomplete`. |
| A-4 | How does `cast_unused` get fixed in the check phase? | Through `patch_scene` only: put the member on stage in one scene, or make them `speaker` of one scene. `set_meta` stays out of the check phase. | `set_meta` is allowed only in cast and revise (`studio.go:53-88`). Widening the check phase grows the compact tool schema (900 token budget) for one lint. A member who is in the cast and never used is a story problem the model can fix by using them. |
| A-5 | The request text | The direction is ranked from the story title and the scene narrations, which the outline already saved. The request text is not plumbed. | `StudioJob` holds no request, and the title and narrations carry the topic words. |
| A-6 | Where do environment descriptions and tags live? | A new embedded file `internal/storymaker/data/env_direction.json`, read by `studio_direction.go`. `ui/img/storymaker/catalog.json` is generated and is not touched. A test ties the file to the catalog: every catalog environment has an entry, no entry names a missing environment. | The catalog is built by `scripts/build-storymaker-catalog.mjs` from legacy manifests, with tags in mixed languages. A separate curated file keeps the generator, its checks and the player out of this batch. |
| A-7 | Where does the direction go? | A `direction` object in the **fill** packet only (a packet key, not a stored scene field), plus an `about` line in the environment lists of the packets. The outline and cast packets keep `environments` as a list of names (an existing test asserts that) and get a new key `environment_guide`. | The `Scene` struct decodes strictly. A stored field would need model, canonical JSON and validator changes for no gain. |
| A-8 | Worked example | `packetFillExample(cast, dir)` builds the example from the scene's own direction (its environment, pose, gesture, actor spot, prop spot). | A fixed example is the cause of the identical scenes. An example that is the direction cannot contradict it. |
| A-9 | Prompt versions | Profile names change from `storymaker/v2/...` to `storymaker/v3/...` in one task. Text edits in the same batch are all part of v3. | `storymaker_agent_profile.go:11`: a published text is never edited in place. |
| A-10 | Fixture names | `testdata/stories/firewall.json` is read by many tests and stays. The real run becomes `testdata/stories/agent-firewall-run.json` (copied from `evidence/firewall-run.story.json` in this folder), and the varied hand-written story becomes `testdata/stories/agent-varied.json`. | No test globs the directory. |

The four lints, with the numbers the spec gave:

- A film needs at least 4 scenes (all scenes count, code scenes too).
- `environment_monotony` (not for the `dialogue_story` template): distinct environments `< min(3, ceil(n/3))`.
- `framing_monotony`: every scene has framing `wide`, none has a `camera.target`, and no scene has a `camera` beat.
- `gesture_monotony`: one gesture in more than `max(2, ceil(n/3))` scenes (a scene counts once per gesture).
- `cast_unused`: a cast member who is neither the `speaker` of a scene nor in any scene's `actors`. `speaker: "narrator"` is not a cast member.
- `entry_monotony`: the streak resets at every scene and fires for every streak of 3 or more (one issue per scene).

Direction roles and acts. With `n` scenes and `t = i/(n-1)`: `i == 0` is `hook`, `i == n-1` is `close`, `t < 0.25` is `problem`, `t < 0.5` is `solution`, `t < 0.75` is `how`, else `benefit`. Acts: hook and problem, solution and how, benefit and close. The ranked environments `r1, r2, r3` (distinct, `env.plain-stage` last) go to act 2, act 1, act 3.

## Batch B: composition v2

| # | Question | Decision | Why |
| --- | --- | --- | --- |
| B-1 | `Beat.scale` range | 0.5 to 2, the same as `Actor.Scale` and `Prop.Scale` (`validate.go:597-601`). The player may normalise the hero prop up to 3 internally. | One range everywhere. The stored value is what the author wrote. The spec said 0.5 to 3. |
| B-2 | Creating a v2 project | A new `ProjectStore.CreateAgent(ctx, title, aspect, voice)` that shares the body of `CreateWithVoice` and sets `Style.Compose = "v2"`. `storymakerAgentCreateProject` calls it. | `CreateWithVoice` is also the editor's path (`project.go:143`, `storymaker_handlers.go:384`). |
| B-3 | The caption band | v2 moves the feet line of the base spots from y 88 to y 74 on 16:9 and 1:1 **only when captions are on**, and keeps the depth scale of the old feet line, so actors keep their size. 9:16 needs no shift: its band is 69.3 to 82 % of the height and the feet are at 88 %. Spots that are not base spots (`desk`, `door`, `window`, `board`) and explicit `{x,y}` points stay as they are. | Measured: base spot feet at 88 % lie inside the 2-line band (79.4 to 92 %) on 16:9 and 1:1. 74 instead of the spec's 76: the push-in reaches zoom 1.08, which moves the feet down on screen by up to 2 %, and 74 keeps at least 3 % between the feet and the band at the end of a scene. |
| B-4 | Where the new player code lives | A new part `ui/js/storymaker/compose.js` (hero scale, push-in, band constants), added to `scripts/storymaker-bundle-parts.js`. `beats.js` (528 of 600 lines) gets three small hooks. | The static guard allows 600 lines per part. |
| B-5 | How `compose` reaches the stage | Through `timeline.style.compose` (`compile.js:216`) into `makeContext` and `stageFor` (`render.js:22-32`), and through the `SM.stage.create` option `compose` in `compile.js:93`. | Compile and render build their stages separately. A flag in only one of them makes them disagree. |
| B-6 | "No own camera" | `ApplyDefaults` always writes `camera: {framing: "wide"}`. A scene has no own camera when framing is `wide`, there is no `camera.target`, and no `camera` beat. | `defaults.go:103-109`. |
| B-7 | Hero prop scale | `scale = clamp(0.24 / (size.h * depth), 0.5, 3)`, applied only when the beat has no `scale`, only for the first cue-bound `show` of a prop in a scene, only for `compose: "v2"`. | `size.h` is in the catalog and the depth is known at build time (`beats.js:435-436`). |
| B-8 | Push-in | Zoom 1.0 to 1.08 over the scene (`SM.camera.addZoom`, `ease-in-out`) with a sideways pan to the speaker; the vertical focus stays, so the feet do not slide down toward the caption band. Rhythm `calm` uses 1.04. The pan target is evaluated once. | `focusOf` is evaluated once at build time. |

## Batch C: umlauts, browser, reasons

| # | Question | Decision | Why |
| --- | --- | --- | --- |
| C-1 | Where is the umlaut repair applied? | In `writeScene` after `sceneWriteFields` (so `put_scene` and `patch_scene` both pass, and `dry_run` does not repair), and in `studioOutline` before the word count. It also repairs `text` of `show` and `say` beats. | `show` and `say` text is drawn and spoken like narration. |
| C-2 | Which language | The job language (`StudioJob.Language`, base tag `de`), the same as `LintWords`. | `voice.language` can differ after `set_meta`. |
| C-3 | The word map | Embedded `internal/storymaker/data/de_umlauts.json`. Only losses whose ASCII form is not a German word. The existing fixtures write `ue` forms (`schuetzt`, `prueft`), which stay untouched, so the map has no `ue` keys. | `studio_story_scenes_test.go:118` pins `schuetzt`. |
| C-4 | Counting repairs | The count lives on the job's first Studio (`Studio.Repairs()`), and `publishOutcome` copies it into `AgentJobInfo.Repairs`. | `finalCheck` builds a second Studio only for the final check (`storymaker_agent.go:887`); `publishOutcome` already reads `j.studio.Progress`. |
| C-5 | Browser download "permission" | There is no config key. The gate is: `Storymaker.Enabled`, `VirtualDesktop.Enabled` and not `VirtualDesktop.ReadOnly` (a download writes to the data folder), and the status says `CanDownload`. The job calls `Browsers.Ensure` before each check round while the last compile was unavailable. | `browser.go:173-236`; `storymaker_handlers.go:680`. |
| C-6 | Download bound | The job bounds `Ensure` with its own context: `render.timeout_seconds`, the same bound the export uses. | The agent job otherwise has only the 30 minute job limit. |
| C-7 | Reason in the check event | `compile_reason` carries a closed code only (`browser_unavailable`, `compile_setup`, `compile_timeout`, `browser_crashed`, `no_compile`). The browser status text (it can hold paths) goes to the log only. | `launchIssue` text can include paths. |
| C-8 | Open check detail | `AgentCheck` gets `Detail string json:"detail,omitempty"`. The job card shows a translated reason per closed code. | Seen text must be translated in 16 locales (`ui/AGENTS.md`). |

## Batch D: layout report

| # | Question | Decision | Why |
| --- | --- | --- | --- |
| D-1 | Where is layout measured? | In a new worker message `layout`, sent by `host-render.js` after `prepare()` and before `POST /report`. The result is added to the report body under `layout`, per scene. `compile()` output stays unchanged. | `compile()` is synchronous, has no canvas and runs before fonts and art load. The compile golden (`04-compile.mjs:207-212`) compares its output. |
| D-2 | When is it on? | A boot flag `layout: true` that only `CompileCheck` sets. Exports do not set it. | An export must not pay for it. |
| D-3 | Hold frame | `max(last cue at_ms, last entity end) + 0.5 s`, clamped to the scene's last frame minus one and to the next scene's start. | `scene.js:167` `entityEnd`. |
| D-4 | v1 films | The caption rule applies to `compose: "v2"` stories only. A v1 film can still be `ready` if the other rules hold, and then the caption rule is simply not claimed. | Every v1 base spot is inside the band. |
| D-5 | What the layout issues are | New class `IssueLayout(code)`, same behaviour as `IssueCraft`: model-fixable, never blocking, listed as open checks. Codes `caption_overlap`, `prop_too_small`, `offscreen`, `actor_prop_overlap`, `scene_static`. | One mechanism. |
| D-6 | `ready` | `ready` needs: compile `ok`, layout measured for every storyboard scene, no code scene, no narration open check, no blocking item. `layout_unverified` is added only for a code scene or when the layout was not measured. | `agentjob.go:80` adds it today without condition. |
| D-7 | Signal that the layout ran | `CompileSummary` gets `Layout []LayoutScene`, `StoryboardScenes` and `CodeScenes`; `CheckReport` gets `StoryboardScenes`, `CodeScenes` and `LayoutScenes` (the number of measured blocks). The layout counts as verified when the compile ran, every storyboard scene has a block and no code scene exists. | `CompileScene` has no kind field today. |
| D-8 | Bubbles | Speech and thought bubbles are not measured. | Out of the spec. |

## Batch E: stills

Decisions of batch E are in its part file, after the code map of batch E was read.

## Execution notes that apply to all batches

- The editor's own lint list does not change. All new issue codes exist for agent jobs only.
- Agent job codes do not enter `ISSUE_CODES` in `ui/js/desktop/apps/storymaker-core.js` (the contract list of 28 codes). They enter `CHECK_KEYS` in `ui/js/desktop/apps/storymaker-agent.js` and the 16 locale files.
- `ui/js/desktop/apps/storymaker-agent.js` and `ui/css/desktop-app-storymaker.css` are CRLF. Go files are LF.
