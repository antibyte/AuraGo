# Tiefsee screensaver asset authoring

## Purpose
Own the deterministic Blender generator and source scene for the Tiefsee
desktop screensaver creatures.

## Ownership
- Authoring: `build_abyss.py`, `check_assets.py`, `production/aurago-abyss.blend`.
- Runtime payload: `ui/3d/screensaver/abyss/v1/`.
- Temporary tools: `disposable/`; visual checks: `reports/`.

## Local Contracts
- Blender 5.2 LTS; original geometry and materials under MIT.
- Creatures are generated from parametric surfaces only; animation is not baked
  into clips but driven by the runtime shaders from TEXCOORD_0/TEXCOORD_1.
- Keep material names (`bell`, `gonad`, `arm`, `tentacle`, `comb_body`, `comb_row`,
  `manta_top`, `manta_belly`) stable; the scene assigns shaders by name.
- Re-running the generator must reproduce identical GLB hashes. The manifest pins
  the generator hash (LF-normalized); regenerate after any generator edit.
- Only runtime GLBs, manifest and license belong in the resource package.

## Verification
- `& 'D:/Blender 5.2/blender.exe' --background --factory-startup --python assets/screensaver-abyss/build_abyss.py`
- `python assets/screensaver-abyss/check_assets.py`
- `node scripts/validate-screensaver-glbs.mjs`
- Rendered review: `AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_SCREENSAVER_THEMES=abyss go test ./ui -run TestDesktopScreensaverScenesBrowser`

## Child DOX Index
None.
