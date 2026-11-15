# Game Maker voxel games

## Purpose

Validated Voxel definitions, finite worlds, transactions, evidence and isolated saves.

## Ownership

This block belongs to the Game Maker contract. The Studio contract, the shared
game lifecycle and the sprite/model library stay in `internal/gamemaker/AGENTS.md`.

## Local Contracts

- Existing Voxel project identity and published saves remain compatible.
  The optional Voxel template installs src/voxel.json and its local runtime.
  Runtime definition checks still protect world/save transactions; free source
  edits and publication do not enforce plan or gameplay evidence.
- Version 1 is a finite, single-player FPS sandbox: 96×48×96 by default,
  at most 128×64×128, 16³ chunks, 64 blocks/items/recipes and 24 enemies.
  Survival/creative is fixed by the definition. Definition errors name the
  element index/ID, the field and its accepted values. Grid mutation owns collision,
  geometry invalidation and saves; inventory/crafting transactions are atomic.
  Hooks use the public API and dispose their scene additions on reload/disposal.
- Both modes use the seven voxelTerrainMaterials during terrain generation.
  Definition errors remain concrete runtime/helper diagnostics. Request framework
  documentation when needed; no dedicated planning/editing profile is required.
  Saves and chunk arrays never enter agent context.
- The additive migration takes a SQLite backup before changing existing project
  tables. Autosaves are CAS-versioned by project and compatible world identity;
  incompatible revisions retain older saves. Reset advances a tombstone version.
  Validate all 4 MiB payloads, coordinates, identities and numerical bounds.
- Published saves require a separate parent-only project/revision grant plus
  HTTP authorization and existing write/delete policy. Asset grants, draft and
  validation frames never read or write saved progress. Published previews and
  exports use immutable revision files; no automatic source/runtime migrations.
  Exported games use IndexedDB; studio save rows are never exported.
- Pause, inventory, focus loss, touch cancellation and disposal release input.
  Held input is owned by its keyboard, UI or canvas-pointer source: releasing
  camera look cannot cancel a separately held Mine action. Verify
  `TestVoxelTouchMineSurvivesCanvasLookRelease` with `GAMEMAKER_VOXEL_BROWSER=1`.
- Canvas input must not request pointer capture while pointer lock owns the mouse.
  Verify real right-click placement under pointer lock as well as unlocked mouse,
  keyboard and touch input; keyboard-only placement checks miss browser failures.
- Creative mining may discard drops when inventory is full, but still removes
  the block and marks the save dirty. Count collection progress only for credited
  items. Survival mining remains atomic and requires room for the drop.

## Work Guidance

Keep these rules binding for every Game Maker change in this area. A rule that
applies to the whole Studio belongs in the parent contract.

## Verification

`go test ./internal/gamemaker -run Voxel -count=1` and the opt-in
`GAMEMAKER_VOXEL_BROWSER=1` browser checks named in the parent contract.

## Child DOX Index

None.
