---
name: aurago-game-assets
description: Create and integrate bounded game art, music, textures, UI, and procedural audio.
license: MIT
compatibility: AuraGo image and music generation integrations
metadata:
  managed_by: aurago
  source: AuraGo synthesis
  phaser_commit: 41be1e462bc600064e498cba370bfa8c5c055a22
  threejs_commit: 7221c1f4a6d2ae189a4d85d058d24f3228499d46
allowed-tools: game_maker_project, game_maker_file, game_maker_asset, game_maker_validate
---

# Game Assets

Ask for only assets that materially improve the current game.

- Images may serve as sprites, sprite sheets, textures, backgrounds, decals, or
  UI art. Request transparent backgrounds when useful and keep dimensions
  modest.
- Music should be instrumental, loop-friendly, and aligned with the intended
  pace. Keep playback opt-in after a user gesture and expose mute or volume.
- Sound effects use procedural Web Audio unless a configured generator is
  explicitly available. Create the AudioContext after player interaction.
- A disabled generator, provider error, or budget limit is a normal fallback.
  Use shapes, gradients, particles, noise, or synthesized tones and keep the
  game playable.
- Reference only the project-local path returned by `game_maker_asset`.
- Never delete or mutate the global AuraGo media registry asset. Project
  deletion removes only the project copy and ledger provenance.
