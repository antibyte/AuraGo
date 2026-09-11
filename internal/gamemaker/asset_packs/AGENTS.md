# Purpose

Offline, original sprite and 3D model library consumed by Game Maker Studio and its agent.

# Ownership

This folder owns eighteen runtime PNG/JSON pairs, `catalog.json`, and retained
Imagegen source artwork plus production instructions under `production/`.
`aurago-low-poly/` additionally owns 220 generated runtime model records, GLBs,
shared animation clips, WEBP previews and MIT license. Its editable Blender
sources and production scripts live in `assets/game-maker-low-poly/`.
The Go service, authenticated HTTP routes and Studio UI remain with their
parent owners. Only the runtime pairs/catalog enter the external resource set;
production artwork is excluded from both the executable and that set. The model
runtime files enter the same resource set; no .blend or authoring scripts do.

# Local Contracts

- Catalog `kind` distinguishes `sprite2d` and `model3d`. The 3D catalog is
  `aurago-low-poly@1.0.0` and its complete uncompressed runtime is at most 100 MiB.
  GLBs use metres, +Y up/+Z forward, exact local dependency hashes and declared
  clips/sockets. Shared humanoid animation data is stored once.
- `import_pack` requires 1–64 explicit `asset_ids` for 3D. Publish the verified
  dependency closure transactionally; reuse identical files, reject modified
  copies, preserve existing project limits and permissions. Model metadata is
  per asset in project copies. Routes serve only manifest-allowlisted files.

- Eighteen sheets, each 640×640 RGBA with 100 cells of 64×64 pixels. Animation frames
  count as cells. Non-tile objects have real transparent margins; no painted
  checkerboards, labels, grid lines, empty cells or clipped bodies.
- Stable pack/asset IDs and schema/versioned JSON describe every cell and
  animation. Preserve ordered frames, direction, origins, timing and provenance.
  Metadata is English. Repeated source poses are explicit holds. Side views
  face right; top-down humans/creatures cover four directions.
- Version 2 records reviewed `entity`, `action`, and `transform` on every asset;
  animations also carry entity/action/direction. `forward_radians` is the actual
  nose direction (0 right, pi/2 down). Only explicit flip_x/flip_y permits flips;
  fixed buildings, cards and terrain do not inherit character rules. Assemblies
  have their own rules and direction, including the left-facing side ambulance.
  Preserve v1 project copies; the current pack imports in a separate v2 directory.
- Preserve originals and their hashes. Edit reviewed rectangles/pose selections
  in `production/manifest.json`; rebuild outputs through the pack script. Do not
  manually patch generated JSON/PNGs. No third-party game characters or packs.
- `assembly_part` sprites may touch cell edges. Each belongs to an `assemblies`
  recipe with width/height, normalized origin and ordered parts (`asset_id`,
  numeric `frame`, top-left pixel `x`/`y`, optional `animation_id`). Render a
  complete shared canvas before slicing; never resize parts independently.
  Empty outer assembly cells are omitted. Parts of a moving vehicle share
  timing and start together; an individual part may remain visually static.

# Work Guidance

- Follow `production/README.md`. Use Pillow 12.2 and nearest-neighbor scaling;
  do not infer transparency by deleting an RGB color. Check source alpha first.
- Review every changed animation on light/dark backgrounds for genuine movement,
  stable scale/ground line, neighboring fragments, directions and transitions.

# Verification

- `python scripts/pack_game_sprites.py --check`
- `go test ./internal/gamemaker ./internal/server -run 'TestSpritePack|TestGameMakerAssetPack'`
- Studio Assets previews and an exported game using locally packaged Phaser 4.2.1.
- `python assets/game-maker-low-poly/verify.py --boards`
- `node scripts/validate-game-maker-glbs.mjs`
- `GAMEMAKER_MODEL_BROWSER=1 go test ./internal/gamemaker -run 'TestModel(PackBrowser|ReferenceExports)' -timeout 8m`

# Child DOX Index

No child contracts. `production/README.md` owns the detailed production brief.

Effects/audio production: `assets/game-maker-presentation/AGENTS.md`.
`aurago-effects` and `aurago-sounds` use version 1.0.0, explicit selected IDs,
and the shared atomic publication transaction. Environments include effect/WAV
dependencies. Modified copies are never replaced. GamePlan schema 3 adds
optional presentation; schemas 1/2 remain compatible.
