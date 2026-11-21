---
name: aurago-game-maker-director
description: Direct an AuraGo Game Maker job from design through a playable verified revision.
license: MIT
compatibility: AuraGo Game Maker Studio; Phaser 4.2.1 or Three.js 0.186.1
metadata:
  managed_by: aurago
  source: AuraGo synthesis
  phaser_commit: 41be1e462bc600064e498cba370bfa8c5c055a22
  threejs_commit: 7221c1f4a6d2ae189a4d85d058d24f3228499d46
allowed-tools: game_maker_project, game_maker_file, game_maker_asset, game_maker_preview
---

# Game Maker development

Implement the user's request freely with the bound project tools. Read existing files
before editing. New projects start with a framework-free `index.html`. Choose any local
HTML/CSS/JavaScript structure; TypeScript, starter templates and the bundled frameworks
are optional. There is one continuous agent run with no required design, scenarios or
repair phases.

## Core tools & shortcuts

- `game_maker_project`:
  - `inspect`: inspect `game.json`, project metadata and live `capabilities` (which templates, local libraries, recipe topics and media generators are ready).
  - `add_library`: copy local bundles (`phaser`, `three`, `voxel`, `effects-2d`, `effects-3d`, `zzfx`, `all`) into `vendor/` without overwriting existing `index.html` or `src/` files.
  - `use_template`: install an optional starter (`canvas` for framework-free HTML5 Canvas 2D + ZzFX, or Phaser/Three/Voxel starters). Always read existing work first because `use_template` replaces entry files.
  - `reference`: fetch framework docs (`phaser`, `three`, `voxel`) or copy-pasteable craft recipes (`topic`: `juice`, `particles`, `procedural-audio`, `procgen`, `pathfinding`, `save`, `tweening`, `ui-hud`).
  - `build` / `finish`: builds save/bundle `src/main.ts` -> `dist/game.js`; `finish` publishes the exact built snapshot.
- `game_maker_asset`: search/describe/import local 2D/3D/audio/effect packs, or `generate` images, music or instant procedural SFX (`kind: "sfx"`, producing `.wav` + ZzFX parameters).
- `game_maker_preview`: `open` / `reload` launch isolated Chromium; `input` accepts keyboard, mouse, `touch_*`/`tap`, `wait` and inline `capture` actions (2–6 captures return a stitched contact sheet); `probe` reads `window.__gameDebug` and HUD/canvas state; `logs` returns runtime/resource errors.

## Game design craft (optional inspiration)

- **Core loop:** Input -> Consequence -> Tactile Feedback -> Progression. Make every button press or collision produce immediate visual and sound confirmation (screen shake, flash, particles, ZzFX sound).
- **First 10 seconds:** Communicate controls and objective visually right on the title/start view and in the first screen of play without walls of text.
- **Clean flow:** Title/Start screen -> Active Gameplay (with `P`/button pause) -> Clear Win/Loss Result with Score & instant Restart (`R` / Space / Click).
- **Pacing & difficulty:** Introduce one mechanic safely, then layer twists, combo multipliers or escalating enemy patterns.
- **Creative twist ideas (pick one when fitting):** risk-reward combo timer, dash that reflects projectiles, light/shadow stealth meter, shrinking/shifting arena, weapon overheat vs cool-down reload, magnet pickup radius that grows with streak, or environmental hazards enemies can be lured into.
