---
name: aurago-phaser4-gameplay
description: Build responsive 2D gameplay with the pinned Phaser 4 runtime.
license: MIT
compatibility: Phaser 4.2.1
metadata:
  managed_by: aurago
  source: phaserjs/phaser skills
  commit: 41be1e462bc600064e498cba370bfa8c5c055a22
allowed-tools: game_maker_project, game_maker_file, game_maker_asset, game_maker_validate
---

# Phaser 4 Gameplay

Use the provided global `Phaser` runtime. Keep a scene-first architecture:
bootstrap configuration, one focused gameplay scene, and separate helpers only
when complexity justifies them.

- Create game objects and physics relationships in `create`; update continuous
  input and simulation in `update`.
- Use Arcade Physics for simple movement, overlap, collision, bounds, and
  velocities. Avoid Matter unless the design genuinely needs it.
- Make keyboard controls explicit and include touch or pointer controls when
  the game concept is likely to be used on mobile.
- Use Phaser scale modes so the canvas adapts without stretching gameplay.
- Pool frequently spawned objects and avoid allocation-heavy effects in
  per-frame paths.
- Communicate objectives, score, health, cooldowns, and game-over state through
  readable in-game UI.
- Use local assets only. Prefer generated art when available and procedural
  shapes when it is not.
- Preserve `window.__AURAGO_GAME_DIAGNOSTICS__` and emit scene readiness.

Validate after scene wiring, after gameplay rules, and after final polish.
