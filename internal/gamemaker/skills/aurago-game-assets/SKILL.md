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

# Optional local game assets

Use search_assets and describe_asset to discover exact pack/asset IDs, views,
orientation, animations and local helper examples. Keep the user's selected pairs.
import_pack copies the requested assets and dependencies into the project; exact
asset_ids are required for model, atlas, effect and audio packs. Use returned paths,
not guessed filenames. Existing local libraries and procedural graphics are valid.

Image/music generation is optional and requires the configured media permission.
Generated files stay under assets/ with their provenance. Imports and writes save
only; they never trigger a build, design approval or gameplay check. Request project
reference for optional framework APIs and preview screenshots when useful.
