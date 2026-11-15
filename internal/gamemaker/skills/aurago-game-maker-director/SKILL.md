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

Implement the user's request with the bound project tools. Read existing files
before editing. New projects have a framework-free index.html. Choose any local
HTML/CSS/JavaScript structure; TypeScript and the bundled game frameworks are optional.
There is one continuous agent run, with no required design, scenarios or repair phases.

Writes save without building. Use project build or preview open/reload when useful.
Preview logs, inputs and screenshots are available independently of the Studio window.
Screenshots preserve game state and have no per-job count quota. Call finish when
the work is complete; a successful current build then publishes a consistent revision.
Cancellation or failure preserves the draft and the previous revision.

Use project reference only when framework documentation is needed. use_template
explicitly replaces entry files; preserve existing authored work unless replacement
fits the request. Local libraries are supported; CDN and npm installation are not.
