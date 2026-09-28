# Screensaver runtime models

## Purpose
Versioned compact GLBs for the Tiefsee (`abyss`) desktop screensaver.

## Ownership
`abyss/v1/` (four creature GLBs, `manifest.json`, `LICENSE.txt`) ships.
Authoring belongs to `assets/screensaver-abyss/`; see its README and DOX contract.

## Local Contracts
- Original MIT geometry: moon jelly, lion's mane jelly, comb jelly and manta ray.
- No textures, images, external URIs or Draco. Every primitive carries POSITION,
  NORMAL, TEXCOORD_0 (along, phase) and TEXCOORD_1 (angle, side); the runtime
  shaders depend on these semantics and on the material names.
- At most 12,000 triangles per creature; the kit stays below 1.5 MiB.
- Manifest hashes, sizes and triangle counts must match the actual GLBs.
- No Blender sources, previews or caches here.

## Verification
`python assets/screensaver-abyss/check_assets.py` and
`node scripts/validate-screensaver-glbs.mjs`.

## Child DOX Index
None.
