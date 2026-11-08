# Reviewed Aura Drift geometry

## Purpose
Retain reviewed NC AI reconstructions of original Aura Drift ImageGen concepts as compact offline build inputs.

## Ownership
- `<group>.json.gz`: native glTF-frame mesh replacements consumed by `kit/reviewed_assets.py` after the procedural builder; all eight cars include per-loop UVs and texture hashes.
- `car-*.webp`: one embedded 1024-square atlas per car. RGB retains artwork; alpha 0.5 is untinted artwork and alpha 1 is livery paint. Alpha never controls opacity.
- `prepare_cars.py`: Blender/FFmpeg preparation from hash-checked originals (`--sources <folder>`); closed Boolean wheel/joint cuts, capped boundaries, detached axle-fragment cleanup and surface-snapped light anchors. Existing original interior boundaries may remain; new cuts cannot increase their count.
- `sources.json`: concept and reconstruction hashes, request identifiers, selected objects and preparation decisions.
- `check.py`: standard-library coverage, provenance, geometry, wheel and rigid-part self-check.
- `runtime-check.js`: async browser-survey fixture for all eight car rigs, wheels, rigid parts and glazing/cloak depth handling.
- Original PNGs, textured GLBs and API logs remain in ignored `reports/aura-drift-ai-refresh-20261107/`.

## Local Contracts
- Deterministic gzip (`mtime=0`), sorted JSON keys, local vertices/faces/linear vertex colours and car UV loops. Rebuilding needs no network; re-preparing cars also needs Blender and FFmpeg.
- Replace only listed mesh data and reviewed pivots; preserve native node names, parents, attachment anchors and audience animation.
- Textured car bodies use `paint`, `trim` (cut caps), `head_light`, `brake_light`; the procedural scaffold retains the seven-role contract. Four neutral wheel nodes use native circular tyres for steering and spin.
- Moving parts are rigid children with local joint origins, animated by the existing runtime.
- Wheel wells include the old tyre interiors; continuous native axles/struts connect the new tyres. The sofa uses its measured narrower front track, not the overall side-table bounds. The mower clears the static reconstructed blade before retaining the native spinner.
- The grandstand retains procedural seating and spectators inside its reconstructed outer frame.

## Work Guidance
- Review all views before replacing shipped models. Keep original reconstruction files intact.
- Fit scenery to its existing placement bounds and verify pack and per-object budgets.

## Verification
- Regenerate with `generate.py`, run `node scripts/test-aura-drift-assets.mjs`, compare two exports by SHA-256.
- `python assets/aura-drift/reviewed/check.py` verifies all 28 retained inputs, UVs, atlas hashes and source/cut boundary counts without Blender or network access.
- Browser survey verifies paint-mask pixels, opaque artwork, ghost opacity, quality-switch texture retention, eight cars, eight biomes and moving-part transforms.
- Visually inspect all eight cars from front, side and rear, plus moving rigid parts; structural checks cannot approve appearance.
- Set `AURAGO_AURA_DRIFT_CUSTOM` to the absolute `runtime-check.js` path and include `custom`
  in `AURAGO_AURA_DRIFT_SCENES` when running `TestAuraDriftSurveyBrowser`.

## Child DOX Index
None.
