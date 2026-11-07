# Reviewed Aura Drift geometry

## Purpose
Retain reviewed NC AI reconstructions of original Aura Drift ImageGen concepts as compact offline build inputs.

## Ownership
- `<group>.json.gz`: native glTF-frame mesh replacements consumed by `kit/reviewed_assets.py` after the procedural builder.
- `sources.json`: concept and reconstruction hashes, request identifiers, selected objects and preparation decisions.
- `check.py`: standard-library coverage, provenance, geometry, wheel and rigid-part self-check.
- `runtime-check.js`: async browser-survey fixture for all eight car rigs, wheels, rigid parts and glazing/cloak depth handling.
- Original PNGs, textured GLBs and API logs remain in ignored `reports/aura-drift-ai-refresh-20261107/`.

## Local Contracts
- Deterministic gzip (`mtime=0`), sorted JSON keys, local vertices/faces/linear vertex colours, no textures, UVs or network dependencies.
- Replace only listed mesh data and reviewed pivots; preserve native node names, parents, attachment anchors and audience animation.
- Cars retain seven body materials and four neutral wheel nodes; native circular tyres provide steering and spin.
- Moving parts are rigid children with local joint origins, animated by the existing runtime.
- The grandstand retains procedural seating and spectators inside its reconstructed outer frame.

## Work Guidance
- Review all views before replacing shipped models. Keep original reconstruction files intact.
- Fit scenery to its existing placement bounds and verify pack and per-object budgets.

## Verification
- Regenerate with `generate.py`, run `node scripts/test-aura-drift-assets.mjs`, compare two exports by SHA-256.
- `python assets/aura-drift/reviewed/check.py` verifies all 27 retained inputs without Blender or network access.
- Browser survey verifies the eight cars, eight biomes and moving-part transforms.
- Set `AURAGO_AURA_DRIFT_CUSTOM` to the absolute `runtime-check.js` path and include `custom`
  in `AURAGO_AURA_DRIFT_SCENES` when running `TestAuraDriftSurveyBrowser`.

## Child DOX Index
None.
