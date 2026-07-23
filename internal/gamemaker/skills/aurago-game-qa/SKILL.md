---
name: aurago-game-qa
description: Verify a generated game through deterministic build and runtime diagnostics.
license: AuraGo original clean-room guidance
compatibility: AuraGo Game Maker Studio diagnostics
metadata:
  managed_by: aurago
  source: AuraGo clean-room synthesis; no TinySwords text, scripts, or assets
  concepts_commit: f59f1dca8bf461227c9b5d856764e1e90d8b8e90
allowed-tools: game_maker_project, game_maker_file, game_maker_validate
---

# Game QA

Use deterministic, scene-first checks. This package contains original AuraGo
guidance and copies no TinySwords code, text, scripts, or assets.

1. Build and inspect every concrete compiler diagnostic.
2. Confirm the manifest, entry point, local runtime, and diagnostic interface.
3. Confirm a canvas becomes ready and the expected scene reports itself.
4. Check that the documented controls change game state.
5. Check the core loop: goal, feedback, pressure or failure, progression, and
   restart or continued play.
6. Check resource and runtime errors, viewport resize, legible UI, and bounded
   frame-rate reporting.
7. Verify that external requests are unnecessary and that a ZIP export remains
   playable offline.

Make at most three focused repair passes. Do not hide a failed validation or
replace the last working preview with broken output.
