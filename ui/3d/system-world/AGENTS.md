# System World runtime models

## Purpose
Versioned compact GLB assets for the integrated System World data metropolis.

## Ownership
The `v1/` kit plus `white-robot.glb` and its provenance `white-robot.json` ship.
Authoring belongs to `assets/system-world/`; see its README and DOX contract.

## Local Contracts
- Preserve 17 designs, three LODs, MIT provenance and the 8 MiB total size gate.
- No Blender source, previews, dependency caches, loose images or remote assets here.
- The white robot is an optimized derivative of the existing `ui/3d/robot.glb`.
  It preserves that artwork's provenance and three embedded 512 px PBR textures;
  do not apply the original kit's MIT-authorship claim to the derivative.
  At most 25,000 triangles, below 2 MiB, explicit tangents, no runtime Draco.
  Five residents share its geometry/materials/textures. Total kit plus robot
  remains below 8 MiB; hashes and source hashes must pass `check_assets.py`.
- Manifest hashes, sizes, bounds and triangle counts must match actual GLBs.
- Load and instance models through the isolated renderer; do not mutate shared legacy Three.js.

## Verification
`python assets/system-world/check_assets.py`, glTF validation and rendered asset review.

## Child DOX Index
None.
