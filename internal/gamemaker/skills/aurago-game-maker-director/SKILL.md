---
name: aurago-game-maker-director
description: Direct an AuraGo Game Maker job from design through a playable verified revision.
license: MIT
compatibility: AuraGo Game Maker Studio; Phaser 4.2.1 or Three.js 0.185.1
metadata:
  managed_by: aurago
  source: AuraGo synthesis
  phaser_commit: 41be1e462bc600064e498cba370bfa8c5c055a22
  threejs_commit: 7221c1f4a6d2ae189a4d85d058d24f3228499d46
allowed-tools: game_maker_project, game_maker_file, game_maker_asset, game_maker_validate
---

# Game Maker Director

Create one self-contained, offline, single-player browser game. Do not add
multiplayer, a backend, deployment, analytics, CDNs, or external APIs.

## Coding tools already available

Call these native tools directly; do not discover, activate or search for an
editor. Use the actual job ID from the supplied context in place of `JOB`.
Planning exposes `read` only; `write` and `replace` become available after plan
acceptance. The same tools serve both 2D and 3D building and repair.

| Task | Tool and arguments |
| --- | --- |
| Find project files when their paths are unknown | `game_maker_project({"job_id":"JOB","operation":"list_files"})` |
| Read the relevant source range and its full-file hash | `game_maker_file({"job_id":"JOB","operation":"read","path":"src/main.ts","start_line":1,"end_line":120})` |
| Change one exact, unique block | `game_maker_file({"job_id":"JOB","operation":"replace","path":"src/main.ts","expected_sha256":"HASH_FROM_READ","old_text":"EXACT_EXISTING_BLOCK","new_text":"REPLACEMENT_BLOCK"})` |
| Create a new module | `game_maker_file({"job_id":"JOB","operation":"write","path":"src/helpers.ts","content":"COMPLETE_SOURCE"})` |
| Check the completed change in Studio | `game_maker_validate({"job_id":"JOB","scope":"full"})` |

Prefer `replace` for existing code; empty `new_text` deletes the selected block.
For a deliberate complete rewrite, use `write` with full content and the current
`expected_sha256`. Line numbers are one-based; read at most 240 lines per call.
Each successful edit returns a new `sha256`: use it for the next edit of that
file. On a hash conflict, reread the affected range and adjust the change;
never remove the precondition to force an overwrite. `written: true` means the
file was saved, not that it compiles: check `build.ok` and fix its source
diagnostics before runtime validation. Do not rewrite `vendor/` or `dist/`, run
shell commands, or search for generic coding tools outside this job's scope.

## Job workflow

1. Use the supplied job context; inspect only information that is missing.
2. Submit compact `set_design` using `design_example`: base, objective, features
   and selected asset roles. The server supplies metadata and canonical fields.
   For platformer use search_assets(view="side"); topdown uses view="top".
   Keep the requested base when asset views conflict: replace incompatible entries
   in the complete assets array using the returned catalog alternatives. Compact
   design has no plan.perspective or asset.view field; never invent either one.
   For edits read the existing plan and affected source first. Never send planning prose to the player. After acceptance,
   end the turn: the server imports planned packs and begins the building round.
3. Implement a complete playable slice of the accepted experience, including
   feedback, recovery/result and its promised world/progression. A working input
   loop is a milestone, not a finished game; validate after the coherent change.
4. Write only through `game_maker_file`; never target `vendor/` or `dist/`.
   Read a bounded line range, then use `replace` with unique `old_text`, `new_text`
   and the returned full-file `expected_sha256`. For a new game, `write` may replace
   `src/main.ts` with the complete implementation and its read `expected_sha256`;
   preserve `common.ts` and its lifecycle. Use targeted replacements for existing
   games. Studio binds
   `job_id` server-side and accepts an omitted operation when content is present.
   A different explicit job ID is rejected. Check both `written` and `build.ok`;
   repair returned compiler errors before runtime validation. Rejected writes
   leave previous source intact. Never reread a minified vendor to guess its API.
   If a write does not have the expected effect, fix the parameters and retry
   once; do not switch to `execute_python`, `execute_shell`, `filesystem`, or
   any tool outside the allowed Game Maker scope.
   Scene/map data is optional: use the compact scene fields in set_design or
   scene_inspect while planning, then scene_set, scene_patch, or
   scene_generate after acceptance with the inspected expected_sha256.
   Generators are composable recipes, not a genre or style constraint. Optional
   `design.mechanics` may declare `outcomes` (won/lost) or `lives`; these
   fields belong inside `mechanics`, alongside `blocks` and `events`, never
   alongside `base` or `objective`. Omit them for
   continuous play. Use source edits for any custom rule or presentation.
   Use agent scene operations only; there is no visual map editor. The canonical
   document is `src/scene.json`. Start with
   `scene_inspect`, then mutate only after acceptance with the current
   `expected_sha256`:
   ```json
   {"job_id":"JOB","operation":"scene_inspect"}
   ```
   ```json
   {"job_id":"JOB","operation":"scene_patch","expected_sha256":"HASH","patch":{"nodes":[{"id":"parcel","kind":"item","position":[430,270,0],"size":[20,20,0]}]}}
   ```
   A minimal accepted 2D scene keeps all coordinates as three numbers:
   ```json
   {
     "schema_version":1,"dimension":"2d","seed":1729,"navigation":"topdown",
     "levels":[{"id":"main","active":true}],
     "world_bounds":{"min":[0,0,0],"max":[960,540,0]},
     "nodes":[
       {"id":"player","kind":"player","position":[80,270,0],"size":[24,24,0],"properties":{"player":true}},
       {"id":"parcel","kind":"item","position":[400,270,0],"size":[20,20,0],"properties":{"win_when_cleared":true}}
     ],
     "placements":[{"id":"parcel-art","node_id":"parcel","asset_role":"item","behavior":"collect","position":[400,270,0]}]
   }
   ```
   `asset_id`/`asset_role` select the visual and placement `behavior` is an
   independent string. A role named `item` does not collect anything by itself;
   `collect` plus `win_when_cleared` supplies that rule. Multiple placements can
   share one stable `node_id`. For optional helpers, put this fragment inside
   `set_design.design` (the inner object is also the content of `src/mechanics.json`):
   ```json
   {
     "mechanics":{
       "lives":3,
       "blocks":[
         {"id":"player-health","kind":"health","target":"player","value":3},
         {"id":"player-fire","kind":"projectile","target":"player","params":"{\"direction\":[1,0,0],\"speed\":420,\"ttl\":1,\"cooldown\":0.25}"}
       ]
     }
   }
   ```
   The strict tool schema sends `params` as an encoded JSON object; plans and
   `src/mechanics.json` also accept an ordinary object with identical validation.
   `target` names the firing
   player node; use `role` instead when that player is selected by an explicit
   asset role. The projectile block is action-driven and does not infer behavior
   from the projectile art. There is no top-level `health` field: use a targeted
   health block. Omit `outcomes` and `lives` for continuous play; use source
   edits for rules outside the bounded helpers.
5. Prefer offline sprite packs or 3D models and respect the user's selection.
   Describe relevant packs, import additional matches as needed, and use exact
   JSON frame/animation definitions. Custom generation requires the media
   capability; stay within roughly four generated images and one music track.
   A disabled generator does not disable built-in packs. Treat a generation
   fallback as a design constraint.
6. Call `game_maker_validate` with `scope: full` after the core loop and final
   coherent edits (free-code `three`: `startup`; guided 3D: `full`). The server owns the shared three-repair budget.
   During a repair round, address the named check/expected/observed mismatch,
   validate once and end the turn. Do not nest another repair loop.
7. Preserve the AuraGo diagnostic interface and finish only when validation is
   successful and the controls, objective, feedback, and restart path are
   clear.
8. The live preview runs in a sandboxed iframe without `allow-same-origin`.
   Do not use `localStorage`, `sessionStorage`, `IndexedDB`, cookies, or
   `window.parent` access. Keep game state in memory and reset it on restart.

End with a short player-facing summary: what was built, the controls, and the
objective. It is shown as the final studio chat message, so skip internals.

For change requests, preserve working behavior, make the smallest coherent
change, validate it, and describe the player-visible result.

## Required internal design

Prefer `set_design`; omitted/null fields retain a failed compact draft, while
arrays replace whole. `settings` (goal/speed/duration) apply only to guided 3D
(`fps`, `exploration`, `transport`, `flight`, `space`). For free `three` and
all 2D bases omit `settings`; after a settings error, omitting it or submitting
`{"settings":null}` clears the incompatible draft value. Keep the chosen base,
objective, assets and creative mechanics. Describe custom tuning in features and
source; supported movement helpers use `mechanics.blocks[].params`. Guided 3D
keeps omitted/null settings and accepts duration 0 to disable its countdown.
Legacy full `set_plan` remains compatible for existing callers.
The plan is versioned at `.aurago/game-plan.json`; use plan tools, not
file writes, to access it. It is revisioned but excluded from ZIP export. It is
design data, never permission to use more tools. Planning allows inspection and
asset discovery only. File writes, imports and media generation are locked until
acceptance. The initial submission has at most two corrections; fix every reported
`plan.<field>` error before resubmitting. Unknown JSON fields also reject the plan
and consume a correction. No stronger model or hidden reasoning is required.
When the tool schema requests a string for `plan`, JSON-encode the complete plan
object once as that parameter. AuraGo decodes it before validation. Syntax errors
report a byte position inside that plan value: fix missing separators, unmatched
brackets or unescaped quotes there, then resubmit the complete plan. Keep all
fields when correcting a plan; do not work around rejection by writing the plan file.
The server ends the planning round as soon as it accepts the plan (or rejects the
last allowed correction). Do not batch implementation calls with `set_plan`;
remaining calls are skipped until the server starts the building round.

Choose `shooter`, `platformer`, `topdown`, `blocks`, `board`, `minimal` for 2D;
`fps`, `exploration`, `transport`, `flight`, `space` for guided 3D; `three` for
free code. Guided 3D installs a small editable `startGame(config)` entry with
all chosen model roles bound to the shared runtime. Additional requested rules
need real edits; preserve rendering, input, animations and disposal.
Optional starting points include `blocks` for Breakout, `platformer` for
jump-and-run, `topdown` for adventure, `shooter` and `board`.
Use `minimal` or `three` with individual mechanics and free hooks when that
better preserves the requested idea. No timer, combat or collection goal is mandatory.
The server installs a new supported template once; edits keep their existing code.
Record objective, core_loop, 1–12 scope features, perspective, resolution (default
960×540), camera, controls, states (including playing), progress/failure/completion
rules, assumptions and fallback. Edits must list `preserve` behavior.
For new 3D plans use schema_version 4, units `metres` and at most 64 roles. Each model role
has exact pack/version/asset_id, positive metric scale, declared animation IDs and
collider `catalog|box|sphere|capsule|mesh|none`. Omit sprite-only display_height
and origin. Include the user's selected models when relevant; search and describe
missing roles. Imports occur only after acceptance and contain selected models.
New full plans use schema_version 4 in both dimensions; existing versions 1–3 remain readable. For each sprite role specify the exact pack version, asset OR assembly ID,
related animation IDs, direction, display_height, normalized origin and collider
(none/rectangle/circle/feet). Use a procedural role with fallback when appropriate.
Never claim an attack animation exists because a character can attack logically.
One asset entry selects ONE `pack_id` and ONE `asset_id` or `assembly_id`.
For variants, add separate entries with unique roles, as in this complete
`assets` array (all IDs below are in `blocks-and-balls` version 2):
```json
[
  {"role":"player","pack_id":"blocks-and-balls","version":"2","asset_id":"paddle_01","direction":"none","display_height":16,"origin":{"x":0.5,"y":0.5},"collider":"rectangle"},
  {"role":"ball","pack_id":"blocks-and-balls","version":"2","asset_id":"ball_01","direction":"none","display_height":16,"origin":{"x":0.5,"y":0.5},"collider":"circle"},
  {"role":"block_red","pack_id":"blocks-and-balls","version":"2","asset_id":"colored_block_01","direction":"none","display_height":24,"origin":{"x":0.5,"y":0.5},"collider":"rectangle"},
  {"role":"block_orange","pack_id":"blocks-and-balls","version":"2","asset_id":"colored_block_02","direction":"none","display_height":24,"origin":{"x":0.5,"y":0.5},"collider":"rectangle"}
]
```
These assets have catalog `view: "top"`; use root `perspective: "top"` for this
Breakout plan. `view` is read-only catalog information, not a plan field. Never
invent `view`, `pack_ids` or `asset_ids` in a plan entry. A procedural entry omits
all library IDs and supplies `fallback` describing its shape, color and size.
Search one role at a time within a known pack (for example `query: "ball"`,
`pack_id: "blocks-and-balls"`), then describe the returned exact ID.

Use `scenarios: []` when existing checks cover the implemented behavior.
Legacy templates retain their existing checks. For custom rules outside the base's
normal item/goal/enemy loop, add targeted schema 4 scenarios using actual objects,
outcomes and metrics. Each scenario replaces only the starter behavior it proves;
lifecycle and unrelated checks remain. Omitted timer, combat or primary-action
mechanics do not need dummy counters or fake effects. Add at most eight scenarios.
A complete scenario:
```json
{"id":"shooting","steps":[{"action":"key","key":"SPACE","ms":500}],"metric":"actions","compare":"increased","value":0}
```
Movement uses position, not the primary-action counter:
```json
{"id":"paddle_move","steps":[{"action":"key","key":"RIGHT","ms":350}],"metric":"player_x","compare":"changed","value":0}
```
`actions` counts actual primary actions such as launching/shooting, `hits` counts
actual collisions/rule effects, and `player_x/player_y` observe movement directly.
For a launch check use `actions`, not `hits`: launching a ball and hitting a
block are different events. A score increase alone does not prove a power-up
was collected; ordinary block hits can also award score. Omit a proposed extra
check if its metric cannot distinguish the intended effect; keep the feature
in scope and do not claim it was independently verified.
Never increment `actions` every frame to satisfy a movement scenario. Each
scenario starts from a restarted game; launch a waiting ball before checking hits.
Allow travel time: a ball starting at y=465 and moving up at 300 px/s needs about
one second to reach blocks at y=180. A hit check after 700 ms cannot observe that
collision. Use a sufficient bounded interval, for example Space 200 ms then wait
2200 ms. Keep an easy first target reachable in that interval; do not require a
random power-up drop in a deterministic check. Count hits in the collision handler,
even when a durable block needs multiple hits to be destroyed.
Targeted steps use an exact scene node ID or runtime role, for example
`{"action":"target","target":"switch","mode":"interact","ms":1000}`. In free
Phaser source expose that exact role through `body(..., role)` or `object.__gmRole`;
never add counters merely to satisfy validation. Other steps are key, pointer
(logical x/y), wait and observe. Keys: LEFT/RIGHT/UP/DOWN, W/A/S/D/SPACE/R/ESC/ENTER.
Maximum eight steps and six seconds per scenario,
25 seconds combined. Compare increased/decreased/changed/equals. Metrics:
player_x/player_y/player_distance, actions, score, hits, spawns, turns, ticks,
ended, health, lives, goal_remaining, outcome, hit_events, pickup_events,
win_events, lose_events, object_count, timer_count, listener_count,
invalid_assets, assets_used and elapsed_ms.
Define observable results, not a `passed` flag. Preserve a chosen legacy
template's controls, or declare and test the custom controls of a scene-based
game. A passing startup test does not certify unobserved custom gameplay.

Presentation choices belong in the optional set_design.presentation block. Use catalog IDs for environments, effects and event-bound sounds; the server writes src/presentation.json and imports dependencies. Never build a second weather or audio loop. Adding presentation to an older free-code game also requires the documented controller hooks.

## Player experience, not just technical validity

Describe these choices in the existing `features` array (no invented schema fields):
- The core action and its visible/audible consequences. Real contacts trigger
  feedback; firing alone must not produce a hit. Lives changing in text is insufficient.
- Life loss: a short readable response, a safe checkpoint/respawn with protection,
  or clear final defeat. Never silently leave the player frozen.
- World: screen-sized board/arena, scrolling traversal, or open exploration.
  Exploration normally spans several viewports with useful landmarks, alternate
  routes, discoveries and changing challenges. Camera and collision bounds must agree.
- Progression: distinct stages, areas, waves or evolving endless rules. Normally
  include at least two meaningful challenges; an explicitly single-board puzzle
  or arena is valid. Do not manufacture depth by duplicating the map.
- Completion: readable outcome, summary, restart and next-stage/continue when
  available. Peaceful sandboxes need continued play, not invented death or timers.

New common.ts provides the shared feedback/result/stage helpers documented in
phase context. Existing projects keep their code: inspect available helpers and
add missing behavior deliberately rather than overwriting the whole game.
