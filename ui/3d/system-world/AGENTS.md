# System World runtime models

## Purpose
Versioned compact GLB assets for the integrated System World data metropolis.

## Ownership
The `v1/` and `v2/` kits plus `white-robot.glb` and its provenance `white-robot.json` ship.
Authoring belongs to `assets/system-world/`; see its README and DOX contract.

## Local Contracts
- Preserve the 17 v1 designs, three LODs and MIT provenance. The complete
  model directory, including v2 and the original robot, has a 40 MiB ceiling.
- No Blender source, previews, dependency caches, loose images or remote assets here. The
  only images are the generated runtime surface maps in `textures/v1/` (MIT, manifest-pinned
  sizes/hashes, written by `assets/system-world/build_textures.py`); they count toward the
  40 MiB model directory and must stay below 1.5 MiB.
- The white robot is an optimized derivative of the existing `ui/3d/robot.glb`.
  It preserves that artwork's provenance and three embedded 512 px PBR textures;
  do not apply the original kit's MIT-authorship claim to the derivative.
  At most 25,000 triangles, below 2 MiB, explicit tangents, no runtime Draco.
  Five residents share its geometry/materials/textures. Total kit plus robot
  remains below 40 MiB; hashes and source hashes must pass `check_assets.py`.
- Manifest hashes, sizes, bounds and triangle counts must match actual GLBs.
- `v2/` contains 42 Blender-authored designs and 126 GLBs with animation and
  navigation metadata. Verify with `test-system-world-expansion.mjs`,
  `test-system-world-layout.mjs` and `test-system-world-hover.mjs`; upper floors must
  match actual rendered lift landings. The complete app (models, renderer, scripts, locales) has an 80 MiB
  budget and first display stays below 20 MiB. New installations initially use
  LOD2; nearby detail follows camera distance and the quality cap. Their collision
  envelopes and work/interaction markers are always independent of visual LOD.
  Robot `motion_bounds` must contain the actual GLB geometry throughout every clip.
- Rigged v2 assets (`robot-*`, `hover-pack`) keep one ordered material list per part and
  stable mesh names at every LOD, so the runtime swaps LODs by mesh name. The
  `hover-pack` `pack` node is hidden while the visitor flies; keep it one removable node.
- `spot.glb`, `spot.json` and `spot-LICENSE.txt` are the compact companion set.
  It is a third-party BSD-3-Clause derivative, not part of the MIT kits: the
  manifest pins the upstream URL, commit and source SHA-256, the GLB uses
  `EXT_meshopt_compression` and `KHR_mesh_quantization`, keeps all 13 named
  joints and both materials, and has no textures or external URIs. The renderer
  loads it through `GLTFLoader.setMeshoptDecoder`; a failed load leaves the city
  fully usable. The verbatim licence travels here and in `licences/texts/`.
  Authoring and reproduction belong to `assets/system-world/spot-source.json`
  and `scripts/build-system-world-spot.mjs`.
- Load and instance models through the isolated renderer; do not mutate shared legacy Three.js.

## Verification
`python assets/system-world/check_assets.py`, glTF validation and rendered asset review.

## Child DOX Index
None.
