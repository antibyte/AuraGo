# Desktop 3D pets

## Purpose

Authoring sources and reproducible animation assets for optional real-time desktop pets.

## Ownership

- `punk/source.blend` and `punk/reference.png` retain the locally authored Rigify character and its reference image.
- `catalog.json` names the eight supported model pets, sprite counterparts and
  joint counts. Keep it aligned with `service_pets.go` and the fixed browser map.
- `zombert/`, `green/`, `hoodie/`, `professor/` and `silver/` retain ImageGen
  references/prompts, original VARCO finger rigs, editable `source.blend` and
  `generation.json` provenance. `build_zombert.py -- --pet=<folder>` bakes the
  T-bind and maps the 53 joints to shared motion names (default: Zombert).
- `rexi/` and `purple-tentacle/` retain references/prompts, VARCO source meshes,
  editable Blender sources and anatomical `rig.json` landmarks.
  `build_creature_pet.py -- --pet=<folder>` authors the 21/13-joint rigs.
- `model_export.py` preserves packed full-resolution Blender sources, then
  normalizes four skin weights, exports the compact LOD and renders its poster.
- `mesh2motion/` retains the pinned official CC0 human animations and the MIT retargeting math used by the offline build.
- Runtime files live in `ui/3d/pets/<folder>/`; `ui/js/desktop/core/pet-3d.js` owns rendering and interactive pose layers. The shared pet runtime owns placement, settings, notifications and drag physics.

## Local Contracts

- The eight model pets are separate bundled entries; preserve their original
  sprites and the active selection. Their ordinary OpenPets sheets
  are WebGL-failure fallbacks; keep them hidden and noninteractive during loading.
- Model and motion URLs are fixed, version-bound local resource URLs, never supplied by an imported pet manifest. No cloud service is required at runtime.
- `PetRuntime.modelAssetFolder(id)` is the shared allowlist for the renderer and
  picker posters; unknown/prototype IDs must never resolve to model URLs.
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
- Rexi and Purple Tentacle use thirty original procedural movements, generated
  by `scripts/build-creature-pet-motions.mjs`, rather than a human motion retarget.
  Keep grounded feet/base, attached cups, tail/tip flexibility and the shared
  semantic gesture IDs. Hash the motion script and `rig.json` into both outputs;
  the procedural motion code/data is GPL-3.0-only, not CC0 Mesh2Motion artwork.

## Work Guidance

- Use Blender 5.2+ to build the compact model, then the installed Three.js/esbuild toolchain for offline retargeting. Reuse the locally packaged Aura Drift Three.js runtime in the browser.
- Check transitions, foot placement, mouse gaze, dragging, reduced motion, hidden tabs, switching pets during a load and GPU disposal in a real browser.

## Verification

Run Blender with `--background --factory-startup --python-exit-code 1 --python assets/desktop-pets/build_model.py` for Punk. Use `build_zombert.py -- --pet=<folder>` for the five VARCO humanoids, or `build_creature_pet.py -- --pet=<folder>` for the two creatures. Then run `node scripts/build-punk-pet-motions.mjs --pet=<folder>` for humanoids, or `node scripts/build-creature-pet-motions.mjs --pet=<folder>` for creatures; `scripts/pet-skeleton.mjs` loads their offline skeletons. Run `node scripts/test-punk-pet.mjs --pet=<folder>` for every changed model (default: Punk). Checks cover encoded textures, normalized skinning, motion hashes, skull/finger separation, real WebGL movement, all picker posters and interaction/lifecycle. Existing `go test ./internal/desktop -run 'TestBundledPersonaSprites|TestInstallBundledDefaultPets|TestServiceBootstrapIncludesAllBundledDefaultPets|TestBundledPet'` and `go test ./ui -run 'DesktopPet|DesktopChat.*Pet'` cover catalog preservation and the shared runtime.

## Child DOX Index

None.
