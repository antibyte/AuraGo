# Storymaker creation modes: from templates or free

Status: design approved by the user on 2026-11-21, spec awaiting review. No plan and no code yet.
Builds on `2026-11-01-storymaker-design.md` (code scenes, Player runtime), `2026-11-06` agent core and `2026-11-09` quality work.

## Goal

When a Storymaker project is created, the user picks how its pictures come about:

- **From templates** (mode `catalog`): the way it works today. People, props and environments come from the catalog, scenes are storyboard JSON.
- **Create freely** (mode `free`): the agent draws and animates everything itself through the code-scene engine. No catalog people, props or environments.

## Decisions the user made (do not re-ask)

1. Free mode means **code scenes only**. No catalog pieces, not even as a supplement.
2. The choice appears **everywhere a project is created**: the agent form on the start screen, the "New project" form, and the chat tool.
3. Approach **mode on the project plus its own agent path**: the Studio enforces the mode; a prompt-only switch and a separate tool family were rejected.

## Vocabulary

"Template" already means the story template in this code base (`concept_explainer`, `how_to`, ... in `Project.Template`, `studioTemplates`). The new field is therefore called `mode`, with the values `catalog` and `free`. The UI labels are "From templates" and "Create freely" (translated into all 16 Storymaker languages).

## Data model

- `Project.Mode string` with `json:"mode,omitempty"`. Empty means `catalog`, so every existing project keeps working with no migration. Writers store `catalog` as the empty string and `free` as `"free"`.
- The mode is set when the project is created and never changes afterwards (no switch in this slice).
- `CreateOptions` (voices.go:170) gets `Mode`; `CreateWithOptions` validates it (`catalog`, `free` or empty, else `ErrInvalidArgument`) and writes it into project.json. `CreateWithVoice` stays a thin wrapper.
- API:
  - `POST /projects` (`storymakerCreateRequest`): new optional `mode`. The body stays strict (`DisallowUnknownFields`), the error text names the new field.
  - `POST /agent/jobs` (`storymakerAgentStartRequest`): new optional `mode`, used only when the request creates the project (no `project_id`). With a `project_id` the project's mode wins; a non-empty `mode` that differs is refused with 422 `mode_mismatch`.
  - Chat tool `storymaker create` (`internal/agent/storymaker_tool.go`, `native_tools_storymaker_chat.go`): new parameter `mode` (`catalog` or `free`, default `catalog`) carried through `AgentCreateRequest.Mode`.
- `StudioJob` gets `Mode`. The server fills it from the project record in `newStorymakerAgentJob`; the model never names it. A revise job on a `free` project is a `free` job.
- Project cards and the project API expose `mode`, so the UI can mark free projects.

## UI

- The start screen shows one **segmented choice** "From templates | Create freely" with one explaining sentence each. It is a single piece of state shared by the agent form and the "New project" form. The default is "From templates".
- `createProjects` and `createAgentStart` read the state and send `mode` in their spec.
- The agent panel of an open blank project shows the project's mode as a read-only chip. A blank project is started with the mode it has.
- Free projects carry a small marker in the project list.
- The editor does not change: the mode steers the agent only. Manual code scenes keep going through the Code tab.
- New keys `storymaker.mode.*` go into all 16 `ui/lang/storymaker/*.json` files (the Storymaker app strings live there, as flat key-to-string maps, not in `ui/lang/desktop/`). Each language gets a real translation: the Storymaker locale tests reject a copy of the English text. `check:ui` and the static i18n test must stay green.

## Agent path in free mode

### Phases

`outline` (as today: story template, target seconds, one narration per scene, word budget) then `fill` (one scene per call, always a code scene) then `check`. The `cast` phase is skipped: there is no catalog cast. `set_meta` in free mode accepts `title` and `style` only. The off-screen narrator uses the project voice as today.

### Tool gating in the Studio

- `studioPhaseTools` and `studioPhaseOperations` get a free variant, picked by `StudioJob.Mode` (new `PhaseToolsFor(mode, phase)` and `PhaseOperationsFor(mode, phase, tool)`; the old functions stay as the `catalog` case). `OperationOwners` stays disjoint.
- Free `fill` offers `storymaker_story`: `put_scene`, `patch_scene`, `dry_run`, `get`, `get_code`, `put_code`, and `storymaker_catalog`: `reference` only. `search` and `describe` are refused in free mode.
- `put_scene` in free mode accepts only `kind: code`. Actors, props, beats and an environment in the data are refused with a `next_action` that says to draw them in code. The existing order stays: `put_code` bundles the source first, then `put_scene` with `kind: code` activates it.
- Revise in free mode keeps `get`, `put_scene`, `patch_scene`, `remove_scene`, `move_scene`, `set_meta`, `dry_run`, `get_code`, `put_code`, `validate` and `reference` only.
- Refusals use the existing `studioRefused` shape and count toward the per-scene and per-job budgets like any other refused write.

### Prompts

`storymakerAgentProfile` takes the mode. Free mode swaps in its own constants and leaves the catalog constants untouched:

- ground rules as today;
- **free house rules and style**: one motif per scene, one palette per film, readable silhouettes, text kept as real text and short, the camera and motion explained through cues, aspect-safe layout for 16:9, 9:16 and 1:1, a hook in the first frame, no stacked bullets;
- **free fill workflow**: read `reference` topic `code` once, write the source with `put_code` within 8 KiB, activate with `put_scene`, run `dry_run` when unsure;
- **free check workflow**: fix `code_error`, compile findings and narration findings, then one sentence for the user.

The tool guide for text-mode models lists the free operations and examples. The profile must stay inside the compact budget; the 900-token schema cap test keeps applying.

## Checks and outcome

- `LintForJob` skips code scenes after `scene_unfilled` (studio_check.go:96). In free mode the narration budget (`over_budget`) must apply to code scenes too, because every scene is one. The stage lints (`spot_overlap`, `entry_monotony`) and the film lints that need a cast or environments (`cast_unused`, `environment_monotony`, `fade_between_worlds`) do not apply.
- The compile check (`storymakerAgentCompile`, `CompileSummary`) and the code-scene frame checks (`dead_stretch`, `first_frame_empty`) run as today and drive the check phase.
- A free film reaches at best `ready_unverified`, because layout checks inside code scenes stay unverified (spec, Player runtime). The outcome card says so in plain words and does not look like a failure.
- Open point for the plan: whether the stills and look-alike checks (quality batch E) already cover code scenes. If they do, free mode uses them; if not, the plan notes the gap and does not widen this slice.

## Limits the user should know

- Code scenes hold at most 8 KiB of source (`StudioCodeBytes`).
- Narration is off-screen only: code scenes have no speaker binding and no lip-sync.
- `chart` and `diagram` helpers are unsupported; drawing is shapes, text and `g.path` SVG path data.
- Quality depends on the model more than in catalog mode. Weak models fail more at free code (spec, Authoring decision), which is why the Studio enforces the path.

## Tests (lean, as in the last batches)

- Go, `internal/storymaker`: mode validation and the store round trip; free phase tables; Studio refusals (catalog search, storyboard `put_scene`, `mode_mismatch`); free lint set including the narration budget on code scenes.
- Go, `internal/server`: create through `POST /projects`, `POST /agent/jobs` (with and without `project_id`), mode on the project card; free profile prompt and tool guide; one scripted agent run with a mock model in free mode ending in `ready_unverified`.
- Go, `internal/agent`: chat tool `mode` parameter, default and invalid value.
- JS: choice state and the spec sent by both forms; the new i18n keys exist in all languages (`ui` static i18n test, `check:ui`); one browser test of the start screen tour for the mode switch.
- Docs: `documentation/manuals/{en,de}/34-storymaker.md`, `documentation/storymaker.md`, `prompts/tools_manuals/storymaker.md`.
- Acceptance with a real model on aurago-test is the user's job; agents never deploy to or write on that server.

## Out of scope

- Changing the mode of an existing project.
- Editor changes (a free default for "add scene", hiding the catalog drawer).
- Raising the 8 KiB limit, `chart` and `diagram` helpers, layout scenes (phase 4), vision review.
- Mixed mode (catalog pieces inside free scenes).

## Files expected to change

`internal/storymaker`: project.go, voices.go, studio.go, studio_check.go, studio_story_scenes.go, studio_catalog.go, agentfacade.go and tests.
`internal/server`: storymaker_handlers.go, storymaker_agent_handlers.go, storymaker_agent.go, storymaker_agent_profile.go and tests.
`internal/agent`: storymaker_tool.go, native_tools_storymaker_chat.go and tests.
`ui`: js/desktop/apps/storymaker-projects.js, storymaker-agent.js, storymaker-api.js, storymaker.js, css/desktop-app-storymaker.css, lang/storymaker/*.json, generated main.bundle.js.
Docs as listed above.
