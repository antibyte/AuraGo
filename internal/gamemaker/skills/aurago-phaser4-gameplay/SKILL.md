---
name: aurago-phaser4-gameplay
description: Build responsive 2D gameplay with the pinned Phaser 4 runtime.
license: MIT
compatibility: Phaser 4.2.1
metadata:
  managed_by: aurago
  source: phaserjs/phaser skills
  commit: 41be1e462bc600064e498cba370bfa8c5c055a22
allowed-tools: game_maker_project, game_maker_file, game_maker_asset, game_maker_preview
---

# Optional Phaser guidance

Use the project's local Phaser runtime when it suits the request. project reference
returns the installed helper/API documentation; project use_template can explicitly
install a 2D starter. Custom HTML/CSS/JS and direct Phaser use are equally supported.

Read existing helpers before editing. Phaser scene restart reuses the scene object,
so initialize per-run state in create/setup. Use dynamic Arcade bodies for movement;
StaticBody has no setVelocity. Register colliders on actual GameObjects/groups.
Load exact local asset paths and animation IDs returned by the asset tools.
Game rules, controls, UI, lifecycle and file layout are the agent's choice.

File writes only save. Use build for compiler diagnostics and preview for actual
rendering, errors, screenshots and input. No AuraGo hook or gameplay counter is
required for publication. finish publishes after the current build succeeds.
