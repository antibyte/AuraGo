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

1. Inspect the project manifest and file list with `game_maker_project`.
2. During the internal planning round, call `get_plan` and inspect existing
   `src/main.ts`/`src/common.ts`. Submit the full structured plan with `set_plan`.
   Use `inspect.plan_example` for the exact schema; replace its example prose
   with concrete rules. Never send planning prose to the player. After acceptance,
   end the turn: the server imports planned packs and begins the building round.
3. Keep the first implementation the smallest loop that is actually playable;
   extend it only after it validates.
4. Write only through `game_maker_file`; never target `vendor/` or `dist/`.
   Supply `operation: "write"`, `path` and the complete `content`. Studio binds
   `job_id` server-side and accepts an omitted operation when content is present.
   A different explicit job ID is rejected. Check for `status: "ok"` before
   validating: a rejected write leaves the previous source in place.
   If a write does not have the expected effect, fix the parameters and retry
   once; do not switch to `execute_python`, `execute_shell`, `filesystem`, or
   any tool outside the allowed Game Maker scope.
5. Prefer offline sprite packs or 3D models and respect the user's selection.
   Describe relevant packs, import additional matches as needed, and use exact
   JSON frame/animation definitions. Custom generation requires the media
   capability; stay within roughly four generated images and one music track.
   A disabled generator does not disable built-in packs. Treat a generation
   fallback as a design constraint.
6. Call `game_maker_validate` with `scope: full` after the core loop and final
   coherent edits (3D: `startup`). The server owns the shared three-repair budget.
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

The plan is versioned at `.aurago/game-plan.json`; use `get_plan`/`set_plan`, not
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

Choose `shooter`, `platformer`, `topdown`, `blocks`, `board`, `minimal`, or `three`.
Use `blocks` for Breakout/Arkanoid, `platformer` for jump-and-run, `topdown` for
adventure, `shooter` for shooting games and `board` for cards/board games.
`minimal` is for games without a matching template, such as Snake; it is only
the schema example's default, not the recommended choice for every request.
The server installs a new 2D template once; edits keep their existing code.
Record objective, core_loop, 1–12 scope features, perspective, resolution (default
960×540), camera, controls, states (including playing), progress/failure/completion
rules, assumptions and fallback. Edits must list `preserve` behavior.
For 3D use schema_version 2, units `metres` and at most 64 roles. Each model role
has exact pack/version/asset_id, positive metric scale, declared animation IDs and
collider `catalog|box|sphere|capsule|mesh|none`. Omit sprite-only display_height
and origin. Include the user's selected models when relevant; search and describe
missing roles. Imports occur only after acceptance and contain selected models.
For 2D, retain schema_version 1. For each sprite role specify the exact pack version, asset OR assembly ID,
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

Use `scenarios: []` when the immutable template minimums cover the core loop.
They already check movement, primary action, a rule effect, timers, delayed
events, ESC end, assets and two restarts. Do not invent duplicate launch/movement/
hit tests just to fill the plan. Add at most eight scenarios only for additional
deterministic behavior that those checks cannot cover. A complete scenario:
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
Allowed steps: key, pointer (logical x/y), wait, observe. Keys: LEFT/RIGHT/UP/DOWN,
W/A/S/D/SPACE/R/ESC/ENTER. Maximum eight steps and six seconds per scenario,
25 seconds combined. Compare increased/decreased/changed/equals. Metrics:
player_x/player_y, actions, score, hits, spawns, turns, ticks, ended, object_count,
timer_count, listener_count, invalid_assets, assets_used, elapsed_ms.
Define observable results, not a `passed` flag. Keep the standard Arrow/Space/R
inputs usable for template tests, even when adding alternative player controls.
