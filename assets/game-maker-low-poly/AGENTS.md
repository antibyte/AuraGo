# Purpose
Original MIT low-poly Game Maker model collection, authored in Blender 5.2.1 LTS.

# Ownership
This directory owns the deterministic Blender sources, model catalog and retained
compressed production scenes. Runtime outputs belong to
`internal/gamemaker/asset_packs/aurago-low-poly/`.

# Local Contracts
- Exactly 220 designs in the ten categories fixed in `catalog.py`. Variants,
  animation clips and LODs are not additional designs.
- Runtime payload, including previews and animations, is at most 100 MiB.
- GLBs use metres, +Y up, +Z forward. Ground objects have base-centred origins.
- Shared humanoid skeleton and animation library; independent animated instances.
- No third-party artwork, runtime Blender dependency or remote asset URLs.
- Retain Blender sources, reproducible generation inputs and MIT license.

# Work Guidance
- Review exported geometry and animations in the actual Three.js renderer.
- Foot contact, grip alignment, moving pivots and modular connections are part
  of acceptance. Passing a parser alone is insufficient.

# Verification
- `python assets/game-maker-low-poly/catalog.py`
- `python assets/game-maker-low-poly/verify.py --boards` checks hashes, bounds,
  rig compatibility, clips, LODs and the full uncompressed 100 MiB budget.
- `node scripts/validate-game-maker-glbs.mjs` runs the pinned Khronos validator.
- `node scripts/build-game-maker-3d.js --check` verifies the local runtime bundle.
- `GAMEMAKER_MODEL_BROWSER=1 go test ./internal/gamemaker -run 'TestModel(PackBrowser|ReferenceExports)' -timeout 8m`
  renders every GLB, exercises animations/independent rigs, the Studio theme/size
  matrix, selective import, exported reference games, restart and resource cleanup.
- See `README.md` for authoring commands and `documentation/game-maker-low-poly.md`
  for game integration. Reports/screenshots/ZIP references belong in ignored
  `reports/low-poly/`; never treat parser success as visual acceptance.

# Child DOX Index
None.
