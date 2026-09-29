# System World asset authoring

## Purpose
Own the original Blender city kit, deterministic generator and source scene.

## Ownership
- Authoring: this directory.
- Runtime payload: `ui/3d/system-world/v1/`, `v2/`, plus `white-robot.glb/json` in its parent.
- Temporary tools: `disposable/system-world/`; visual checks: `reports/system-world-assets/`.
- Surface textures: `build_textures.py` writes `ui/3d/system-world/textures/v1/` (six seeded,
  periodic RGB WebP data maps plus manifest and MIT license).

## Local Contracts
- Blender 5.2.1 LTS; original geometry and materials under MIT.
- Each asset has three isolated GLBs, Y up, metre units and base-centred origins.
- The complete runtime model directory must stay below 12 MiB; first display
  stays below 12 MiB and the full app below 48 MiB.
- Surface maps are original procedural data (R albedo factor / 2, G roughness factor / 2,
  B height), seamless, byte-stable for the pinned numpy/Pillow and verified with `--check`.
  The GLBs stay texture-free; the renderer projects the maps (see `sysworld-surfaces.js`).
- Original city-kit designs have no image textures or external URIs. Preserve
  separate rotor and signal nodes. The explicitly requested ThreeDee robot
  derivative retains three embedded 512 px PBR textures and source provenance;
  do not relabel its existing artwork as newly authored MIT geometry.
- `build_robot.py` owns that derivative (24,000 triangles, below 2 MiB), never
  changes `ui/3d/robot.glb`, and exports tangents without runtime Draco.
- Only runtime GLBs, manifest and license belong in the resource package.
- Keep the planned cinematic city direction and street-level inspection quality.
- `build_expansion.py` owns 36 v2 designs and `production/aurago-world-2.blend`.
  The `telescope` tube pivots at its yoke and faces glTF +Z at rest. The `sky-lift`
  cab travels `SKY_TRAVEL` (71.32 m) from the 0.88 m agent podium to the `sky-deck`
  (top at zero, placed at 72.2 m); keep both equal to `skyDeck` in
  `ui/js/desktop/apps/sysworld-layout.js`. The lift shaft is an open lattice (the
  kit's glass is opaque), its tower and boarding sides stay open at the landings.
  `living_assets.py` authors the repair bay, parcel sorter, relay mast, kinetic
  fountain, glass garden and sheltered charging/meeting point. Export their
  colliders, work slots, interaction and emitter markers at all three LODs.
  The courier's named `parcel` component follows its pickup/delivery cycle.
  The manifest pins hashes of all three authoring sources.
  Robot `motion_bounds` enclose all sampled exported clips; collision dimensions
  include raised arms and remain identical across LODs.
  Preserve matching LOD pivots and articulated clips. Modular floors have planar
  joining edges; the lift platform top is zero with four metres of travel and an
  open left side. Gallery guards use the `railing` module. Verify actual exported
  floor/shaft alignment with `node scripts/test-system-world-layout.mjs`.

## Verification
- `python assets/system-world/check_assets.py`.
- `python assets/system-world/build_textures.py --check` and `node scripts/test-system-world-surfaces.mjs`;
  `--preview` writes a lit contact sheet to `reports/system-world-assets/`.
- All exported GLBs must pass the Khronos validator and a real browser rendering check.
- Render previews from the exported GLBs. Repeated generation must preserve GLB hashes.

## Child DOX Index
None.
