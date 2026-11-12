# Desktop 3D pets

## Purpose

Authoring sources and reproducible animation assets for optional real-time desktop pets.

## Ownership

- `punk/source.blend` and `punk/reference.png` retain the locally authored Rigify character and its reference image.
- `zombert/` retains Zombert's ImageGen reference, VARCO source/finger-rig GLBs,
  editable `source.blend` and `generation.json` provenance. `build_zombert.py`
  bakes the T-bind, maps the 53 joints to the shared motion names and exports
  its compact model and picker poster.
- `mesh2motion/` retains the pinned official CC0 human animations and the MIT retargeting math used by the offline build.
- Runtime files live in `ui/3d/pets/punk/` and `ui/3d/pets/zombert/`; `ui/js/desktop/core/pet-3d.js` owns rendering and interactive pose layers. The shared pet runtime owns placement, settings, notifications and drag physics.

## Local Contracts

- `aurago-punk-3d` and `aurago-zombie-3d` are separate bundled pets; preserve
  sprite Punk/Zombert and the active selection. Their ordinary OpenPets sheets
  are WebGL-failure fallbacks; keep them hidden and noninteractive during loading.
- Model and motion URLs are fixed, version-bound local resource URLs, never supplied by an imported pet manifest. No cloud service is required at runtime.
- Preserve normalized skinning, packed textures and the original editable source. Runtime model budget: 60,000 triangles, at most 71 joints, textures at most 1024 square.
- Zombert's rigid skull region must exclude the raised fingers of the T-bind.
  Keep hands attached during greeting, sitting and pickup; retain the VARCO
  finger weights and the single connected 53-joint tree.
- `PunkPersona_Eyes` is authored geometry with separate sclera, violet iris,
  pupil and catchlight materials, parented to `DEF-spine.006`. Export it with the
  portable head, retain its world placement and include it in geometry budgets.
  Check the eyes in a face close-up and at desktop scale, with head gaze/motion.
  Match the reference's large violet irises (about 60% of the opening width),
  clipped by the lids, with restrained warm sclera rather than broad white areas.
- Bake the A-pose mesh into a T-bind pose before retargeting. The runtime deform
  rig has one root: hips/pelvis under `DEF-spine`, shoulders under the chest,
  upper arms under shoulders, palms/first finger segments under hands. Do not
  copy Rigify's disconnected control/MCH parents into the portable skin.
- Keep upstream source revision, hashes, motion names and license notices with the generated outputs. Mesh2Motion art is CC0; its retargeting code is MIT. These licenses do not relabel AuraGo's character source artwork.

## Work Guidance

- Use Blender 5.2+ to build the compact model, then the installed Three.js/esbuild toolchain for offline retargeting. Reuse the locally packaged Aura Drift Three.js runtime in the browser.
- Check transitions, foot placement, mouse gaze, dragging, reduced motion, hidden tabs, switching pets during a load and GPU disposal in a real browser.

## Verification

Run Blender with `--background --factory-startup --python-exit-code 1 --python assets/desktop-pets/build_model.py` for Punk, or `build_zombert.py` for Zombert. Run `node scripts/build-punk-pet-motions.mjs` and `node scripts/test-punk-pet.mjs`, adding `--pet=zombert` to both for Zombert. Checks cover encoded textures, skin/motion hashes, skull/finger separation, real WebGL movement, switching between 3D pets and interaction/lifecycle. Existing `go test ./internal/desktop -run 'TestBundledPersonaSprites|TestInstallBundledDefaultPets|TestServiceBootstrapIncludesAllBundledDefaultPets|TestBundledPet'` and `go test ./ui -run 'DesktopPet|DesktopChat.*Pet'` cover catalog preservation and the shared runtime.

## Child DOX Index

None.
