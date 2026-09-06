# Purpose

Offline, original pixel-art library consumed by Game Maker Studio and its agent.

# Ownership

This folder owns eighteen runtime PNG/JSON pairs, `catalog.json`, and retained
Imagegen source artwork plus production instructions under `production/`.
The Go service, authenticated HTTP routes and Studio UI remain with their
parent owners. Only the runtime pairs/catalog are embedded in the binary.

# Local Contracts

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
- Studio Assets previews and an exported game using embedded Phaser 4.2.1.

# Child DOX Index

No child contracts. `production/README.md` owns the detailed production brief.
