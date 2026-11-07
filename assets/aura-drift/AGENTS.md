# Aura Drift model kit

## Purpose
Own the Blender 5.2 generator of the Aura Drift model pack and its pure-Python kit (`kit/`).
`generate.py` is a thin CLI; the humans' guide with the helper reference is [`README.md`](README.md).

## Ownership
- Authoring: this directory. `kit/polish_props.py` owns the six extra trackside props (shared
  balloon bunch and camera tower; alpine waymark, desert parasol, harbour signal mast, metro holo
  arrow); `kit/weather_props.py` owns the six weather props (shared wind sock and rain shelter;
  alpine snow gauge, desert anemometer, harbour storm lantern, glacier drift fence). The shared
  builders return `toy.Parts`, the scenery builders fill a `Piece`.
  `runtime-entry.mjs` belongs to the three.js vendor bundle, not to the kit.
- Runtime payload: `ui/3d/aura-drift/v1/` (20 GLBs, 9 179 600 bytes, `manifest.json`, `LICENSE.txt`).
  Written only by `generate.py`, the manifest only by `--only manifest` (a partial `--only` run
  emits a partial manifest, so regenerate the whole pack before shipping). Adding a prop means adding
  its node to `scripts/test-aura-drift-assets.mjs`, its tags to `kit/meta_*.py`, and its placement
  to `AD.SCATTER` or `AD.furnitureProps` so it is actually visible. A prop in `shared.glb` can only
  be reached through `AD.furnitureProps`: `AD.scatterProps` places into the biome's own kit.
- Audio inputs: `audio/sources.json` (13 hash-pinned CC0 sources, 17 sfx group overrides)
  and `audio/README.md`. The sound pack itself is `ui/3d/aura-drift/audio/` and is written only by
  `scripts/build-aura-drift-audio.mjs --write --sources D:\tmp\agdrift-cc0` (downloads in one folder per source id,
  never committed); its synthesis code is in `scripts/aura-drift-synth/`. Credits are in
  `ui/3d/aura-drift/audio/LICENSE.txt`; engines, menu/results music and all other groups are own synthesis.
  The eight race songs (`music/<biome>-song.ogg`) are YuE2 Studio generations (licence terms NOT verified, open item),
  recorded in `audio/yue2-songs.json` and re-encoded from FLAC masters with `build-aura-drift-audio.mjs --songs <dir>`.
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
- Budgets live in the validator (cars 14000 triangles and 480 KB, shared.glb 1.75 MiB, scenery
  kits 1.1 MiB each, pack 18 MiB; per-prop 450/1200/4500/14000 triangles by tag). Check after each build.

## Verification
- `"D:\Blender 5.2\blender.exe" --background --factory-startup --python <abs>/assets/aura-drift/generate.py -- --only cars,manifest --out <abs>`
- `AURAGO_AURA_DRIFT_ASSETS_PARTIAL=1 node scripts/test-aura-drift-assets.mjs` (partial packs) or without the variable (full pack).
- `node scripts/build-aura-drift-audio.mjs --check` (audio pack, read-only, no ffmpeg) and `node scripts/test-aura-drift-audio.mjs`.
- Visual review: `--preview <abs scratch dir>`, then read the PNGs.
- Model and sound changes are followed by a regenerate and a commit of the pack together with the kit.

## Child DOX Index
None.
