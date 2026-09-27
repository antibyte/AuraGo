# System World runtime models

## Purpose
Versioned compact GLB assets for the integrated System World data metropolis.

## Ownership
The `v1/` and `v2/` kits plus `white-robot.glb` and its provenance `white-robot.json` ship.
Authoring belongs to `assets/system-world/`; see its README and DOX contract.

## Local Contracts
- Preserve the 17 v1 designs, three LODs and MIT provenance. The complete
  model directory, including v2 and the original robot, has a 12 MiB ceiling.
- No Blender source, previews, dependency caches, loose images or remote assets here.
- The white robot is an optimized derivative of the existing `ui/3d/robot.glb`.
  It preserves that artwork's provenance and three embedded 512 px PBR textures;
  do not apply the original kit's MIT-authorship claim to the derivative.
  At most 25,000 triangles, below 2 MiB, explicit tangents, no runtime Draco.
  Five residents share its geometry/materials/textures. Total kit plus robot
  remains below 12 MiB; hashes and source hashes must pass `check_assets.py`.
- Manifest hashes, sizes, bounds and triangle counts must match actual GLBs.
- `v2/` contains 33 Blender-authored designs and 99 GLBs with animation and
  navigation metadata. Verify with `test-system-world-expansion.mjs` and
  `test-system-world-layout.mjs`; upper floors must match actual rendered lift
  landings. The complete app (models, renderer, scripts, locales) has a 48 MiB
  budget and first display stays below 12 MiB. New installations initially use
  LOD2; nearby detail follows camera distance and the quality cap. Their collision
  envelopes and work/interaction markers are always independent of visual LOD.
  Robot `motion_bounds` must contain the actual GLB geometry throughout every clip.
- Load and instance models through the isolated renderer; do not mutate shared legacy Three.js.

## Verification
`python assets/system-world/check_assets.py`, glTF validation and rendered asset review.

## Child DOX Index
None.
