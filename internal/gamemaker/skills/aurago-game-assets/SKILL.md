---
name: aurago-game-assets
description: Select local sprite packs or original 3D models and integrate game art, animation, music, and procedural audio.
license: MIT
compatibility: AuraGo offline sprite library; optional image and music integrations
metadata:
  managed_by: aurago
  source: AuraGo synthesis
  phaser_commit: 41be1e462bc600064e498cba370bfa8c5c055a22
  threejs_commit: 7221c1f4a6d2ae189a4d85d058d24f3228499d46
allowed-tools: game_maker_project, game_maker_file, game_maker_asset, game_maker_preview
---

# Optional local game assets & procedural audio

## Art direction & visual clarity

- **Contrast & silhouettes:** Keep foreground players, projectiles and pickups high-contrast against darker or desaturated backgrounds so gameplay reads instantly at a glance.
- **Cohesive palette:** Pick 1 dark backdrop tone, 1–2 muted environment tones, 1 hero accent color (e.g. cyan/emerald), 1 reward color (gold/amber) and 1 danger color (coral/crimson).
- **Depth:** Layer multi-speed parallax backgrounds or subtle ambient particles behind the playfield; keep colliders slightly forgiving compared to artwork bounds.

## Local asset packs

- Use `search_assets` (with `query`, `asset_kind`, `view`) and `describe_asset` to inspect exact pack/asset IDs, views (`side`, `top`, `board`, `3d`, `isometric`), orientation (`forward_radians`), animation clips and usage snippets.
- `import_pack` copies the selected assets and dependencies into `assets/builtin/...`; exact `asset_ids` are required for `model3d`, schema-2 atlas sprites, `effect` and `audio` packs. Use returned paths, never guessed filenames.
- Procedural Canvas/SVG/Three.js geometry is equally valid when custom visual styles fit better.

## Instant procedural SFX, ZzFX & optional media

- **Zero-permission SFX:** Call `game_maker_asset` `operation: "generate", kind: "sfx", prompt: "retro coin pickup", path: "assets/sfx/coin.wav"` anytime (even when external media generation is off). It writes a crisp `.wav` file and returns matching `zzfx_params` and `playSfx(...)` snippets.
- **Inline Web Audio synth:** Import `{ playSfx, zzfx, sfxPresets } from '../vendor/zzfx.js'` (via `add_library library="zzfx"` or `use_template`) for zero-latency procedural SFX unlocked on first user gesture.
- **AI image/music generation:** When `inspect` reports `image_generation_ready` or `music_generation_ready`, `generate` (`kind: "image"` or `"music"`) creates bespoke title art, textures or looping music under `assets/`.
