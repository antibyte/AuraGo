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
2. Reply first with a short build plan the player can read in the studio:
   the core loop, the fail or pressure condition, the progression signal,
   the controls, and which generated assets you will request. Then build.
3. Keep the first implementation the smallest loop that is actually playable;
   extend it only after it validates.
4. Write only through `game_maker_file`; never target `vendor/` or `dist/`.
   If a write does not have the expected effect, fix the parameters and retry
   once; do not switch to `execute_python`, `execute_shell`, `filesystem`, or
   any tool outside the allowed Game Maker scope.
5. Use `game_maker_asset` only for planned, gameplay-relevant media and only
   when the capability is enabled. Stay within roughly four images and one
   music track per job. Treat a fallback response as a design constraint,
   not a failed job.
6. Call `game_maker_validate` after coherent edits. Fix concrete diagnostics
   before adding polish, with at most three repair passes.
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
