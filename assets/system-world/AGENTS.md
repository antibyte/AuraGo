# System World asset authoring

## Purpose
Own the original Blender city kit, deterministic generator and source scene.

## Ownership
- Authoring: this directory.
- Runtime payload: `ui/3d/system-world/v1/`, `v2/`, plus `white-robot.glb/json` in its parent.
- Temporary tools: `disposable/system-world/`; visual checks: `reports/system-world-assets/`.

## Local Contracts
- Blender 5.2.1 LTS; original geometry and materials under MIT.
- Each asset has three isolated GLBs, Y up, metre units and base-centred origins.
- The complete runtime model directory must stay below 12 MiB; first display
  stays below 12 MiB and the full app below 48 MiB.
- Original city-kit designs have no image textures or external URIs. Preserve
  separate rotor and signal nodes. The explicitly requested ThreeDee robot
  derivative retains three embedded 512 px PBR textures and source provenance;
  do not relabel its existing artwork as newly authored MIT geometry.
- `build_robot.py` owns that derivative (24,000 triangles, below 2 MiB), never
  changes `ui/3d/robot.glb`, and exports tangents without runtime Draco.
- Only runtime GLBs, manifest and license belong in the resource package.
- Keep the planned cinematic city direction and street-level inspection quality.
- `build_expansion.py` owns 33 v2 designs and `production/aurago-world-2.blend`.
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
- All exported GLBs must pass the Khronos validator and a real browser rendering check.
- Render previews from the exported GLBs. Repeated generation must preserve GLB hashes.

## Child DOX Index
None.
