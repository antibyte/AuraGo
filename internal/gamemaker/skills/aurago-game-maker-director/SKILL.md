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
5. Prefer offline sprite packs and respect the user's preimported selection.
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
acceptance. The initial submission has at most two corrections; fix the precise
`plan.<field>` error. No stronger model or hidden reasoning is required.
When the tool schema requests a string for `plan`, JSON-encode the complete plan
object once as that parameter. AuraGo decodes it before validation. Keep all fields
when correcting a plan; do not work around rejection by writing the plan file.
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
For each asset role specify the exact pack version, asset OR assembly ID,
related animation IDs, direction, display_height, normalized origin and collider
(none/rectangle/circle/feet). Use a procedural role with fallback when appropriate.
Never claim an attack animation exists because a character can attack logically.

Add 1–8 `scenarios`, beyond immutable template minimums. A complete scenario:
```json
{"id":"shooting","steps":[{"action":"key","key":"SPACE","ms":500}],"metric":"actions","compare":"increased","value":0}
```
Movement uses position, not the primary-action counter:
```json
{"id":"paddle_move","steps":[{"action":"key","key":"RIGHT","ms":350}],"metric":"player_x","compare":"changed","value":0}
```
`actions` counts actual primary actions such as launching/shooting, `hits` counts
actual collisions/rule effects, and `player_x/player_y` observe movement directly.
Never increment `actions` every frame to satisfy a movement scenario. Each
scenario starts from a restarted game; launch a waiting ball before checking hits.
Allowed steps: key, pointer (logical x/y), wait, observe. Keys: LEFT/RIGHT/UP/DOWN,
W/A/S/D/SPACE/R/ESC/ENTER. Maximum eight steps and six seconds per scenario,
25 seconds combined. Compare increased/decreased/changed/equals. Metrics:
player_x/player_y, actions, score, hits, spawns, turns, ticks, ended, object_count,
timer_count, listener_count, invalid_assets, assets_used, elapsed_ms.
Define observable results, not a `passed` flag. Keep the standard Arrow/Space/R
inputs usable for template tests, even when adding alternative player controls.
