# System World runtime models

## Purpose
Versioned compact GLB assets for the planned System World data metropolis.

## Ownership
Only `v1/*.glb`, `v1/manifest.json` and `v1/LICENSE.txt` ship.
Authoring belongs to `assets/system-world/`; see its README and DOX contract.

## Local Contracts
- Preserve 17 designs, three LODs, MIT provenance and the 8 MiB total size gate.
- No Blender source, previews, dependency caches, images or remote assets here.
- Manifest hashes, sizes, bounds and triangle counts must match actual GLBs.
- Load and instance models through the isolated renderer; do not mutate shared legacy Three.js.

## Verification
`python assets/system-world/check_assets.py`, glTF validation and rendered asset review.

## Child DOX Index
None.
