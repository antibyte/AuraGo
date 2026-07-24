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
- Pick a fixed logical resolution and `Phaser.Scale.FIT` with auto-centering
  so the canvas adapts to the preview without stretching gameplay.
- Make keyboard controls explicit and include touch or pointer controls when
  the game concept is likely to be used on mobile.
- Reference project assets with relative paths (`assets/...`); load them in
  `preload` and confirm the exact paths returned by `game_maker_asset`.
- Unlock audio only after a player gesture; keep music opt-in with a mute.
- Pool frequently spawned objects and never allocate objects or arrays in the
  per-frame `update` path.
- Communicate objectives, score, health, cooldowns, and game-over state through
  readable in-game UI with a fixed HUD (`setScrollFactor(0)`).
- Release scene resources on shutdown so restarts stay leak-free.
- Preserve `window.__AURAGO_GAME_DIAGNOSTICS__` and emit scene readiness.

Validate after scene wiring, after gameplay rules, and after final polish.
