# Aura Drift model kit

## Purpose
Own the Blender 5.2 generator of the Aura Drift model pack and its pure-Python kit (`kit/`).
`generate.py` is a thin CLI; the humans' guide with the helper reference is [`README.md`](README.md).

## Ownership
- Authoring: this directory. `runtime-entry.mjs` belongs to the three.js vendor bundle, not to the kit.
- Runtime payload: `ui/3d/aura-drift/v1/` (GLBs, `manifest.json`, `LICENSE.txt`). Written only by
  `generate.py`, the manifest only by `--only manifest`.
- Check: `scripts/test-aura-drift-assets.mjs`.
- Preview renders and scratch builds go outside the repository (`D:\tmp\agdrift-shots`, `D:\tmp\agdrift-build`).
- Generated scratch folders under `assets/aura-drift/build/` are git-ignored.

## Local Contracts
- Contract source: section 3 (GLB asset contract) of `plans/2026-11-06-aura-drift-3d-overhaul.md`.
- Authored in the glTF frame (Y up, metres, nose +X, right +Z); no textures, no images, UVs only on `face` meshes.
- Deterministic: seeded `core.rng_for`, no time, sorted iteration. Two runs must give equal SHA-256.
- Every module is MIT licensed, starts with a docstring and stays under about 400 lines.
- Group modules (`car_<id>.py`, `items.py`, `track_parts.py`, `animals.py`, `shared_props.py`, `scenery_<biome>.py`)
  expose `build() -> core.Model` and are discovered by `generate.py`; parallel authors never edit `generate.py`
  or the manifest.
- Car body materials are exactly `paint`, `paint_accent`, `trim`, `glass`, `chrome`, `head_light`, `brake_light`;
  wheels use `tyre`.
- Budgets live in the validator (cars 14000 triangles and 480 KB, pack 18 MiB). Check after each build.

## Verification
- `"D:\Blender 5.2\blender.exe" --background --factory-startup --python <abs>/assets/aura-drift/generate.py -- --only cars,manifest --out <abs>`
- `AURAGO_AURA_DRIFT_ASSETS_PARTIAL=1 node scripts/test-aura-drift-assets.mjs` (partial packs) or without the variable (full pack).
- Visual review: `--preview <abs scratch dir>`, then read the PNGs.

## Child DOX Index
None.
