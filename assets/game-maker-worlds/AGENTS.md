# Purpose
Original MIT maritime and isometric Game Maker assets, authored in Blender.

# Contracts
- `catalog.py` owns 120 maritime motifs and 240 isometric entries. Directions,
  recolours, frames and LODs never increase these counts.
- Runtime limits: pirates-3d 48 MiB, pirates-topdown 20 MiB, pirates-side 16 MiB,
  isometric 40 MiB; additional shared resources 4 MiB. Measure uncompressed files.
- Production sources and retained .blend scenes stay here; only reviewed runtime
  output enters `internal/gamemaker/asset_packs/` and the catalog.
- Reuse original AuraGo geometry/rig authoring utilities, not third-party art.
- Export metres, +Y up, +Z forward. Ships retain waterline and named sockets.
- Pixel atlases use schema 2, at most 2048 square, fixed per-asset anchors and
  explicit frame rectangles. Never resize individual animation frames to fit.
- Isometric projection is 2:1, 128x64 tiles, 32 px per elevation unit.
- A parser pass is not visual acceptance. Inspect exported GLBs, every animation,
  joins, sprite directions, independent instances and extracted game exports.

# Verification
Production reports belong in ignored `reports/game-maker-worlds/`.
