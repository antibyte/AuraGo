# System World asset authoring

## Purpose
Own the original Blender city kit, deterministic generator and source scene.

## Ownership
- Authoring: this directory.
- Runtime payload: `ui/3d/system-world/v1/` plus `white-robot.glb/json` in its parent.
- Temporary tools: `disposable/system-world/`; visual checks: `reports/system-world-assets/`.

## Local Contracts
- Blender 5.2.1 LTS; original geometry and materials under MIT.
- Each asset has three isolated GLBs, Y up, metre units and base-centred origins.
- The complete runtime directory must stay below 8 MiB.
- Original city-kit designs have no image textures or external URIs. Preserve
  separate rotor and signal nodes. The explicitly requested ThreeDee robot
  derivative retains three embedded 512 px PBR textures and source provenance;
  do not relabel its existing artwork as newly authored MIT geometry.
- `build_robot.py` owns that derivative (24,000 triangles, below 2 MiB), never
  changes `ui/3d/robot.glb`, and exports tangents without runtime Draco.
- Only runtime GLBs, manifest and license belong in the resource package.
- Keep the planned cinematic city direction and street-level inspection quality.

## Verification
- `python assets/system-world/check_assets.py`.
- All exported GLBs must pass the Khronos validator and a real browser rendering check.
- Render previews from the exported GLBs. Repeated generation must preserve GLB hashes.

## Child DOX Index
None.
