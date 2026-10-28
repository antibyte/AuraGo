# Ocean chat sea life asset authoring

## Purpose
Own the deterministic Blender generator and source scene for the sea creatures
(reef fish, sharks, turtle, whale) of the Ocean chat theme.

## Ownership
- Authoring: `build_ocean_life.py`, `check_assets.py`, `production/aurago-ocean-life.blend`.
- Runtime payload: `ui/3d/chat-ocean-life/v1/` (GLBs, `manifest.json`, `LICENSE.txt`).
- Runtime consumers: `ui/js/chat/ocean-life-*.js`, `ui/js/chat/ocean-life.js`
  (they read the files by name and assign shaders by material name).
- Temporary tools and preview renders stay outside the repository.

## Local Contracts
- Blender 5.2 LTS; original geometry and materials under MIT.
- Animals are generated from parametric surfaces only; animation is not baked into clips but driven by the
  runtime shaders from TEXCOORD_0 = (along, part) and TEXCOORD_1 = (height, side).
- Frame: glTF Y-up, +Z is the nose, +X is the animal's left, origin near the centre of mass, metres. The
  generator authors points in that frame and converts with `to_blender`; do not rotate the objects.
- Keep material names (`body`, `fin`, `eye`, `shell`, `flipper`, `fluke`) and the `part` values
  (0 body/shell, 0.25 fin, 0.5 eye, 0.75 flipper, 1 fluke/caudal) stable; the scene assigns shaders by name
  and `check_assets.py` pins both.
- Asset ids and their order (`fish_tang`, `fish_clown`, `fish_butterfly`, `fish_sardine`, `shark_reef`,
  `shark_hammer`, `turtle`, `whale`), file names and the manifest `length` per asset are part of the
  contract with the runtime species table.
- Budgets: kit at most 1.5 MiB, fish at most 3 000 triangles, sharks/turtle/whale at most 8 000.
- Re-running the generator must reproduce identical GLB and manifest hashes: seeded `random.Random` per
  species only, sorted-key JSON chunk, stable node and mesh names. The manifest pins the generator hash
  (LF-normalized); regenerate after any generator edit.
- Only runtime GLBs, manifest and license belong in the resource package.

## Verification
- `& 'D:/Blender 5.2/blender.exe' --background --factory-startup --python assets/chat-ocean-life/build_ocean_life.py`
- `python assets/chat-ocean-life/check_assets.py`
- `node scripts/validate-screensaver-glbs.mjs ui/3d/chat-ocean-life/v1/`
- Shape review: add `-- --preview <dir outside the repo>` and look at `<id>-side.png` / `<id>-quarter.png` / `<id>-front.png`.
- Rendered review: `AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run TestChatOceanSeaLifeBrowser`

## Child DOX Index
None.
