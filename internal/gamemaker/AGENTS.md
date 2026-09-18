# Game Maker

## Purpose

Own offline game planning, source editing, asset import, browser validation,
revision publication and standalone export for Phaser and Three.js games.

## Ownership

- `guided_design.go`, `skills.go`, `skills/`: compact agent contracts and examples.
- `templates/`: new-project source bases; preserve authored existing source on edits.
- `runtime/game-flow.js`: player feedback and result/stage UI, driven by engine clocks.
- `runtime/aurago-game-1.js`: Phaser input/assets and read-only test binding.
- `assets/game-maker-presentation/` at repository root: editable effects/audio source;
  its build script produces the two bundled effects runtimes here.
- `runtime.go`: project-local runtime installation, also used by exports.

## Local Contracts

- Terminal job states become observable only after working-copy cleanup, with the
  writer released under the same lock. Immediate retries and revision restores
  must not race a job that has already reported completion.
- Complete experiences include consequences, recovery or results, a suitable world
  and meaningful progression. Single-screen boards and peaceful sandboxes remain
  valid; do not require combat, lives, timers or linear levels for every idea.
- Phase guidance stays within 2500 characters and uses existing design fields.
  Keep its prefix deterministic; put detailed examples in curated skills.
- Feedback never changes gameplay counters, health or outcomes. Real contacts and
  game rules own those changes. Missing observations remain unverified.
- New common templates opt into baseline visual/audio feedback. Explicit imported
  sound bindings take precedence; one mixer, gesture gate and 32-voice budget apply
  to fallback cues too. Mute/volume survive stage changes within the same game root.
- Game flow owns no loop or timers. Pause/inactive state freezes it. Scene shutdown
  or renderer disposal removes feedback, UI and audio resources. Ending play still
  renders the result UI; authored stages rebuild through the engine lifecycle.
- Phaser life-based authored rules use `damagePlayer()` and checkpoints; Three.js
  authored rules use `api.damagePlayer(amount)`. Scene-builder health/contact rules
  remain authoritative for scene-driven games. Do not double-apply damage.
- Level contents are authored source or scene data. Helpers select actual levels;
  never claim progression by cloning an empty map or incrementing a label.
- Phaser HUD roots use screen-fixed coordinates and explicit `auragoHUD` data.
  World depth may equal world Y; crossing depth 1000 must never hide actors.
  Asset fitting preserves aspect ratio without resizing authored colliders.
  Keep collision footprints separate from artwork and transparent padding.
- Preview startup judges canvas layout only in an active, sized viewport after
  continuous layout settling. Hidden Studio windows/tabs are not game errors.
  Genuine invisible canvases still fail with measured layout diagnostics.
- Movement-only target scenarios measure positions, not primary-action counts.
  Explain mismatched legacy checks without marking them passed or fabricating
  counters. A displacement check alone never proves steering smoothness.
- Visual review stays advisory and uses the selected multimodal route directly.
  Request JSON mode only for supported providers; tolerate normal JSON wrappers
  and allow at most one format correction within the original review deadline.

## Work Guidance

Keep existing `common.ts` working on revisions. Advertise new APIs to older games
only after reading their source and deliberately incorporating needed helpers.
Do not patch a published game merely because a new starter changed.

## Verification

- `go test ./internal/gamemaker` and focused agent/server tests.
- `GAMEMAKER_EXPERIENCE_BROWSER=1`: normal-input contact, feedback, checkpoint,
  pause, stage/result/restart and narrow-screen controls in exported 2D/3D fixtures.
- `GAMEMAKER_TARGET_BROWSER=1`: positive and negative read-only target evidence.
- `GAMEMAKER_GUIDED_BROWSER=1`: starter engine/lifecycle browser checks.
- `GAMEMAKER_PREVIEW_BROWSER=1`: hidden/resumed previews, delayed layout, invalid
  canvases and loading-HUD recovery in Chrome.
- `node scripts/build-game-maker-presentation.js --check` and
  `node scripts/test-game-maker-presentation-evidence.mjs`.
- Package changed runtimes with the matching resource flags and verify `--check-assets`.

## Child DOX Index

- `asset_packs/AGENTS.md`: original local content, metadata and reproducible packing.
