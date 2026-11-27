# Storymaker blocks and reliable code scenes

Status: design approved by the user on 2026-11-27, spec awaiting review. No plan and no code yet.
Builds on `2026-11-01-storymaker-design.md` (Player runtime), `2026-11-09` quality work, `2026-11-21-storymaker-creation-modes-design.md`
(free mode is code scenes only), `2026-11-22-storymaker-measured-checks-design.md` (checks and the frame probe), and the
`storymaker_view` vision tool (commit 342fbd9ba).

## Goal

Make the agent produce better animations and films. The work has two stages:

1. Stage 1: free-mode (code-scene) films finish reliably, and code scenes become reviewable like storyboard scenes.
2. Stage 2: explainer building blocks (icons, diagrams, charts, kinetic text) through one renderer with two entry points.

## Evidence

The evidence comes from the real runs on aurago-test: 11 agent jobs between 2026-11-07 and 2026-11-26. The data is in
`data/storymaker/agent/jobs.json` and `events/*.jsonl`. The projects are under
`agent_workspace/virtual_desktop/Documents/Storymaker/`.

1. The two free-mode jobs ran 30.6 and 30.4 minutes. They used MiniMax-M3.1-Flash. The requests were "was ist eine inference
   engine" (2026-11-23) and "Was ist ein Agentic Harness" (2026-11-26). Each filled only 4 of 7 scenes. Each ended
   `incomplete` with open checks: `scene_unfilled` (3 times), `scene_static` (3 times), `model_error`, `layout_unverified`
   and `narration_pending`. 4 and 7 model calls respectively hit `completion_tokens` 20480 exactly.
2. Cause: `internal/server/storymaker_agent.go:410` sets `max_tokens` to `gameMakerOutputTokens(gameMakerRouteLimits(&cfg),
   llm.ConservativeOutputTokens)`. That is 4096 answer tokens plus a 16384 reasoning allowance, which is 20480. Code scenes
   need about 18 to 20 thousand tokens of reasoning.
3. A response that ends with `finish_reason` length is an `IncompleteCompletionError` (`internal/agent/agent_llm_call.go:440-446`).
   Its tool calls are dropped and its reasoning is discarded.
4. Each attempt starts with a fresh ephemeral history (`storymaker_agent.go:316-320`). So the model reads the reference and
   calls `get_code` again on every attempt. These two calls cost about 24 s per attempt.
5. `storymakerAgentAttempts` is 2 (`storymaker_agent.go:477`). Scenes are filled strictly in order. A slow scene uses the time
   that the later scenes need. The job timeout is `Storymaker.Agent.JobTimeoutSeconds`, default 1800
   (`internal/config/storymaker.go`).
6. An unfilled scene stays a storyboard scene with its narration. It uses the default `env.plain-stage` (the curtain stage)
   and has no cast. The film shows an empty stage.
7. The contact sheet showed only the 3 empty storyboard scenes and none of the 4 code scenes. `ui/js/storymaker/layout.js:84-86`
   skips code scenes (`if (timeline.scenes[i].kind === 'code') continue;`). Stills come only from layout blocks
   (`internal/storymaker/render/compilestills.go:17-40`). So a code scene gets no still, no contrast check, no look-alike
   (`internal/storymaker/stills.go:188-193`) and no vision screenshot on the sheet.
8. The free reference text (`internal/storymaker/reference/code-free.md:37` and `code.md:37`) tells the model to draw inside
   x=710..1210, y=110..800 for all formats. Nothing enforces this. In a 16:9 project the safe area is about x 96..1824
   (`stage.js` DEFAULT_SAFE 5/5/90/90).
9. Charts, icons and diagrams are placeholders. `beats.js:485` ignores `show` with `chart` or `icon`. `validate.go` marks
   them `not_supported_yet`. `codescene.js:190-191` `api.chart` and `api.diagram` draw nothing. The `Beat` struct already has
   `Icon`, `Chart` and `Data` fields (`internal/storymaker/model.go:263-297`).
10. Catalog-mode films (agnes-3.0-flash and step-3.7-flash) finish. But every scene is one prop with one gesture. The catalog
    has 80 prop ids, mostly everyday objects. So tech topics get unrelated props: `prop.shield` for "LLM",
    `prop.fingerprint` for "summarise texts", `prop.graduation-cap` for "next word".
11. A step-3.7-flash story kept closing markers `{/1}` in its narration. They showed in the captions, for example
    "Daten{/1}.".

## Decisions the user made (do not re-ask)

1. Scope: reliability and building blocks are both in this round.
2. Icons come from an open icon library shipped locally: Lucide (`lucide-static`, ISC licence). About 500 curated icons.
   The licence and NOTICE files ship with the data. Nothing is loaded from the Internet at runtime.
3. Blocks appear in two places: on stage next to the characters (the board area), and as full-frame board scenes.
4. Approach: one renderer with two entry points. Storyboard beats serve catalog scenes. `api.*` calls serve code scenes.
   Rejected alternatives:
   - Code-only helpers. Catalog films would then need a code scene for every diagram, which is the failure seen in the evidence.
   - Beats only. Free mode would lose its own drawing, and `api.chart` and `api.diagram` would stay placeholders.
5. Stage 1 and Stage 2 are approved as written below. The guidance, the tests and the batch order are approved too.
6. One-time download of `lucide-static@1.82.0` from the npm registry during batch 2 is approved. The package is about 58 MB
   unpacked. Only the generated data, about 200 KB, is committed.

## Stage 1: reliable and reviewable code scenes

### Output budget

In free mode, the phases fill, check and revise call `gameMakerOutputTokens` with `gameMakerBuildAnswerTokens` (16384).
They no longer use `llm.ConservativeOutputTokens`. The result:

- 32768 tokens at a 204800-token context.
- 65536 tokens when the context is at least 262144 tokens. This uses the wide allowance of 49152, rule
  `gameMakerReasoningAllowance` in `internal/server/game_maker_service.go:177-182`.

Catalog mode keeps 20480. No catalog run went above 12495 completion tokens.

### Fill order

- Pass 1 gives every unfilled scene one attempt.
- Pass 2 retries only the scenes that are still unfilled.
- The limit stays at 2 attempts per scene. The job timeout is unchanged.

### Shorter attempts

- The free fill packet (`internal/storymaker/studio_packet.go`, `packetFreeFill`) carries the scene's `code_rev`. It is `"-"`
  for a scene without source (`internal/storymaker/project.go:33`).
- The packet also carries a compact API card: signatures, palette and `safe_box`. The card is about 1.5 KB. The plan fixes
  the text of the card.
- The free profile (`internal/server/storymaker_agent_profile_free.go`) changes. It no longer says to read the reference
  "before the first scene". It says to read the reference only when the card is not enough.
- The model can call `put_code` directly, without first calling `get_code`.

### Format-correct drawing area

- The packet gets `safe_box` in design pixels for the project's own aspect ratio. It is the visible part of the 1920x1080
  design frame for that aspect (`stage.js` cover fit), inset by the platform margins of `internal/storymaker/layout.go:80-87`
  (9:16 uses its overlay margins, the other aspects 5 % per side), and its bottom edge ends above the caption band. Go
  computes it; the plan fixes where, and a test pins the three boxes.
- Code scenes get `api.safe()`, which returns the same box as `{x, y, w, h}`.
- The fixed range x=710..1210 is removed from `code.md` and `code-free.md`.

### Code scenes in the review

- `layout.js` emits a layout block for each code scene. The block has a `code` flag and a hold frame. The plan fixes the
  moment of the hold frame, for example 70 % into the scene or after the last cue.
- Stills, the contact sheet, the contrast check, the look-alike and `storymaker_view` then cover code scenes too.
- In Stage 1, a code block carries no element boxes. So it adds no new layout issues. Stage 2 adds boxes for blocks.

### Closing markers

- The Studio removes `{/n}` from the narration when it saves. This happens in the same place as the umlaut repair
  (`repairText`).
- The narration parser in Go (`internal/storymaker/narration.go`, `parseMarkers`) and in JS (`ui/js/storymaker/narration.js`)
  both ignore `{/n}`. So old stories show clean captions.
- Go and JS stay in step.

## Stage 2: building blocks

### Icons

- Source: `lucide-static@1.82.0` (ISC). The download is pinned with its npm integrity hash.
- A build script converts the SVG elements of each icon into path data on Lucide's 24x24 grid. The elements are `path`,
  `circle`, `rect` (including `rx`), `line`, `polyline`, `polygon` and `ellipse`. The script writes one JSON file. Each entry
  has `id`, `paths` and `tags` (Lucide tags and categories). The plan fixes the script name. It sits next to the existing
  Storymaker catalog build.
- The allow-list holds about 500 curated icons. The categories include devices, development, connectivity, security,
  science, medical, finance, charts, communication, people, nature, transport, weather, files, arrows, home and education.
  The plan fixes the exact list.
- The `LICENSE` and `NOTICE` files ship with the data.
- The data is one file. So the asset-count budget is not affected.
- Ids have the form `icon.<lucide-name>`, for example `icon.cpu` and `icon.server`.
- Drawing: strokes on `Path2D` with round caps and joins. Line width is 2 on the 24 grid, scaled to the target size.
- Styles:
  - `line`: palette ink.
  - `badge`: a filled circle in the palette accent, with a light line.
  - `card`: a rounded card with a label below.
- Entry effects: `draw` reveals the strokes with a line dash that depends only on t, so it is deterministic. `pop` and `fade`
  also work.
- Catalog search also returns icons (kind `icon`). Icons match on id and on Lucide tags. Each result has a `usage_example`.
- Free mode may search icons. The kind is forced to `icon`. Catalog entries stay off-limits in free mode.

### Diagrams

- Layouts:
  - `flow`: 2 to 5 nodes in a row, with arrows.
  - `cycle`: 3 to 6 nodes on a ring, with curved arrows.
  - `hub`: a centre and 2 to 6 spokes.
  - `stack`: 2 to 5 layers, built from the bottom.
  - `compare`: 2 columns. Each column has a title and up to 3 points.
- Edges are implied by the layout. The model does not list edges.
- A node has an `id`, a `label` of at most 3 words and an optional `icon`.
- Nodes reveal in order across the beat duration. Alternatively, each node reveals on a phrase of the narration. The field is
  `cue_phrase` per node. The Studio resolves it the same way as the beat `cue_phrase`.
- Arrows draw on.
- `flow: true` moves small dots along the arrows. The dots are a pure function of t.
- `highlight` may target one node, written as `<diagram id>.<node id>`.
- Labels shrink to fit their node. The plan fixes the minimum size.

### Charts

- `bar`: up to 8 bars.
- `line`: up to 24 points.
- `pie`: drawn as a donut, up to 6 parts.
- `counter`: one number that counts up, with an optional unit.
- Charts use the existing `ChartData` labels and values. They add an optional `unit` and an optional `title`. The plan fixes
  the length limits.
- Bars and lines grow on entry. Values are labelled.
- The existing beat kind `chart` plays like `show` with `chart`. The agent is taught `show` only.

### Kinetic text

- Text style `headline`: the words rise one after the other. One word written as `*word*` gets a marker highlight.
- Text style `list`: up to 4 lines, separated by newlines. Lines are revealed one by one, or on cues.
- The existing styles `title` and `label` stay as they are.

### Placement

On stage:

- The spot `board` becomes an area.
- If the environment's art has a board or a screen, the area is its inner face. The office whiteboard is one example.
- Elsewhere, the area is the right half of the frame, above the caption band.
- The plan fixes the rectangle for each environment.
- Blocks scale to fit the area.
- A character can point at the area with the gesture `point`, or look at it.

Full frame:

- A new environment `env.board` is drawn by the player from the palette. Its variants are day (light paper) and night (dark
  slate), with a subtle grain.
- `env.board` is available in every catalog build and in free mode.
- Its spots are `full`, `left`, `right` and `top`.
- An actor may stand small at the side as a presenter.

Defaults:

- Free-mode scenes default to `env.board` instead of `env.plain-stage`. Code scenes start on a calm board. They no longer paint
  a full-frame rectangle to hide the curtain stage. Scenes that already have a saved environment keep it.

### Entry points

Storyboard beats:

- `show` with `icon` (and an optional style).
- `chart` with `data`.
- A new `diagram` object: `{layout, nodes[{id, label, icon}], flow}`.
- Text with style `headline` or `list`.
- Entry value `draw` for icons and diagrams.
- `highlight` on a node.

Code scenes: `api.icon(id, opts)`, `api.diagram(spec, opts)`, `api.chart(spec, opts)`, `api.headline(text, opts)`,
`api.list(items, opts)` and `api.safe()`. Handles support `fadeIn`, `fadeOut`, `moveTo`, `scaleTo` and
`highlight(nodeId, timing)`. The placeholders and their `not_supported_yet` notes are removed. Everything draws on the
same Canvas2D path as the `g` facade, and is a pure function of t.

Shared player:

- Both entry points call one player module. The plan fixes its name.
- The plan also fixes how the storyboard path (raster overlay canvas) and the code-scene path share that module.

### Measuring

- Blocks report their boxes to the layout measurement, in code scenes too. A new block list is added in `layout.js`
  (`measureScene`) and in `internal/storymaker/layout.go` (`LayoutScene`).
- Rules: caption overlap, offscreen, too small (the plan fixes the threshold), overlap with an actor.
- Inside code scenes, only block boxes and the caption are measured. `api.text`, `api.shape` and raw `g` drawing stay unmeasured.
- New issue codes are added in three places: Go, the JS `ISSUE_CODES` list (`ui/js/desktop/apps/storymaker-core.js`), and the
  pinned guard test for that list.

### Fallback board for unfilled scenes

At the end of a job, a scene that is still unfilled gets a board automatically.

- The headline has at most 8 words. It is taken from the scene's narration.
- No icon is used. Lucide tags are English, and narration often is not.
- Catalog mode: the scene becomes a storyboard scene on `env.board`, with a `show` beat that has a text headline.
- Free mode: the scene becomes a small generated code scene that uses `api.headline`.
- A new open check `scene_fallback` names the scene. It replaces `scene_unfilled` for that scene.
- The outcome stays `incomplete`. But the film has no empty frame.
- A scene the user left empty in the app is not touched: the fallback runs only at the end of an agent job, only for scenes of
  that job's outline.

## Agent guidance

- The profiles (catalog and free), `animation.md`, `code.md`, `code-free.md` and the examples are rewritten around one visual
  idea per scene.
- Each rewrite includes a decision table:
  - steps or a process: flow
  - a loop: cycle
  - parts of a system: hub or stack
  - A versus B, or before and after: compare
  - numbers: chart or counter
  - a thing or a concept: icon card or prop
  - the key sentence: headline
- Variety rules:
  - No block type three times in a row.
  - In a 7-scene film, 1 to 3 full-frame boards.
  - In catalog mode, the character points at the board.
- The fill packet's direction gets `suggested_block`. It comes from simple word rules for German and English:
  - "zuerst", "dann", "danach"; "first", "then": flow
  - numbers and percent: chart
  - "Kreislauf", "Schleife"; "loop", "cycle": cycle
  - "besteht aus", "Schichten"; "consists of", "layers": stack
  - "statt", "im Gegensatz"; "instead", "versus": compare
  - Other languages get no suggestion.
- Fill examples and the free code example (`internal/storymaker/reference/free-motion.js`) use the new blocks. The free example
  becomes shorter.
- New advice findings. None of them blocks an export:
  - `no_blocks`: a film of 5 or more scenes without any block.
  - `block_repeat`: the same block layout three times in a row.
  - `label_long`: a node label over 3 words, or a headline over 8 words.

## Tests (lean, as in the last batches)

- Node tests for the blocks: the same t gives the same pixels; the boxes of every layout; reveal timing; the icon file is
  complete and every icon draws.
- Go tests: validation of the new `show` fields; layout rules; the fallback board; the output-budget function; the two-pass
  fill order with a scripted model; the packet fields.
- One browser test: a fixture film that uses every block, in 16:9 and 9:16, with stills.
- Existing guard tests are updated, not deleted: the `ISSUE_CODES` pin, the beat support matrix in
  `scripts/storymaker-tests/04-compile.mjs`, and the `chart` expectation in `validate_test.go`.
- One combined review per batch.
- Acceptance is by the user on aurago-test, with two requests: one free and one catalog. Agents never deploy.

## Batches

1. Stage 1 complete.
2. Icons: download, build script, data, licence files, catalog search.
3. Player blocks, `env.board`, board area.
4. Beats, Go and JS validation, layout measurement, node highlight.
5. Code API and free reference.
6. Agent guidance, packet hints, advice findings, fallback board, user manual.

## Out of scope

- Keeping the reasoning of a cut-off response across attempts (agent checkpoints).
- Raising the default job timeout.
- Measuring raw `g` drawing, `api.text` or `api.shape` inside code scenes.
- Icon suggestions for narration that is not German or English. German icon tags.
- New catalog props or characters.
- Music and audio checks.
- A vision model that judges the blocks. `storymaker_view` already exists, and it will simply see more.

## Files expected to change (the plan fixes the exact list)

- Go, server: `internal/server/storymaker_agent.go`, `storymaker_agent_profile.go`, `storymaker_agent_profile_free.go`.
- Go, storymaker: `internal/storymaker/` `studio_packet.go`, `studio_story*.go`, `studio_catalog.go`, `catalog.go`,
  `validate.go`, `model.go`, `layout.go`, `narration.go`, `defaults.go`, `agentjob.go`, `issues.go`, `stills.go`,
  `render/compilestills.go`, `reference/*`.
- UI player: `ui/js/storymaker/` `beats.js`, `codescene.js`, `layout.js`, `render.js`, `raster.js`, `stage.js`,
  `narration.js`, `text.js`, `checks.js`, and a new blocks module (the plan fixes the name).
- Assets: `assets/storymaker/lib` (environments and catalog build), the new icon data and its licence files.
- Desktop and language: `ui/js/desktop/apps/storymaker-core.js`, `ui/lang/storymaker/*.json` (new issue texts).
- Tests and docs: `scripts/storymaker-tests`, `documentation/manuals`.
