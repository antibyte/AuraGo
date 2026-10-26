# Game Maker voxel games

## Purpose

Validated Voxel definitions, finite worlds, transactions, evidence and isolated saves.

## Ownership

This block belongs to the Game Maker contract. The Studio contract, the shared
game lifecycle and the sprite/model library stay in `internal/gamemaker/AGENTS.md`.

## Local Contracts

- Voxel projects use immutable `dimension: 3d, variant: voxel`; legacy projects
  retain the empty variant. Schema 5/base `voxel` owns `src/voxel.json` and optional
  `main.ts` hooks. Plan acceptance and file writes share definition validation.
  Build evidence requires a used definition and the installed Voxel runtime;
  executable definitions count as implementations. Keep generic `config.objects`,
  scene/mechanics and unchanged-code requirements on their existing game paths.
- Version 1 is a finite, single-player FPS sandbox: 96×48×96 by default,
  at most 128×64×128, 16³ chunks, 64 blocks/items/recipes and 24 enemies.
  Survival/creative is fixed by the definition. Definition errors name the
  element index/ID, the field and its accepted values. Grid mutation owns collision,
  geometry invalidation and saves; inventory/crafting transactions are atomic.
  Hooks use the public API and dispose their scene additions on reload/disposal.
- Both modes require the seven `voxelTerrainMaterials` used by terrain generation.
  Planning/editing guidance names them; creative supply does not remove them.
  Definition validation reports bounded independent corrections together before
  checking reachability. Native JSON-string and object designs retain the same
  failed draft for partial corrections; acceptance and file-write gates stay strict.
- Voxel prepared profiles keep three planning/four editing tools and fixed
  schemas; repair shares editing. Only compact definitions/API and bounded file
  observations enter prompts. Saves and chunk arrays never enter agent context.
  Exact helper comparison remains necessary for compact source references.
- The additive migration takes a SQLite backup before changing existing project
  tables. Autosaves are CAS-versioned by project and compatible world identity;
  incompatible revisions retain older saves. Reset advances a tombstone version.
  Validate all 4 MiB payloads, coordinates, identities and numerical bounds.
- Published saves require a separate parent-only project/revision grant plus
  HTTP authorization and existing write/delete policy. Asset grants, draft and
  validation frames never read or write saved progress. Published previews and
  exports use immutable revision files; no automatic source/runtime migrations.
  Exported games use IndexedDB; studio save rows are never exported.
- Voxel validation uses real sampled block/inventory/player/enemy consequences.
  Counter-only reports cannot pass mining, placing, crafting or combat. Preserve
  the 16-check/60-second ceilings and the ordinary input-only driver boundary.
  Pause, inventory, focus loss, touch cancellation and disposal release input.
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
