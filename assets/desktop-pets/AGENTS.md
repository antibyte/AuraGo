# Desktop 3D pets

## Purpose

Authoring sources and reproducible animation assets for the optional real-time Punk desktop pet.

## Ownership

- `punk/source.blend` and `punk/reference.png` retain the locally authored Rigify character and its reference image.
- `mesh2motion/` retains the pinned official CC0 human animations and the MIT retargeting math used by the offline build.
- Runtime files live in `ui/3d/pets/punk/`; `ui/js/desktop/core/pet-3d.js` owns rendering and interactive pose layers. The shared pet runtime owns placement, settings, notifications and drag physics.

## Local Contracts

- Add `aurago-punk-3d` as a separate bundled pet; preserve the existing sprite Punk and active selection. Its ordinary OpenPets sheet is the picker/WebGL-failure fallback; keep it hidden and noninteractive while the 3D model is loading.
- Model and motion URLs are fixed, version-bound local resource URLs, never supplied by an imported pet manifest. No cloud service is required at runtime.
- Preserve normalized skinning, packed textures and the original editable source. Runtime model budget: 60,000 triangles, 71 deform joints, textures at most 1024 square.
- Bake the A-pose mesh into a T-bind pose before retargeting. The runtime deform
  rig has one root: hips/pelvis under `DEF-spine`, shoulders under the chest,
  upper arms under shoulders, palms/first finger segments under hands. Do not
  copy Rigify's disconnected control/MCH parents into the portable skin.
- Keep upstream source revision, hashes, motion names and license notices with the generated outputs. Mesh2Motion art is CC0; its retargeting code is MIT. These licenses do not relabel AuraGo's character source artwork.

## Work Guidance

- Use Blender 5.2+ to build the compact model, then the installed Three.js/esbuild toolchain for offline retargeting. Reuse the locally packaged Aura Drift Three.js runtime in the browser.
- Check transitions, foot placement, mouse gaze, dragging, reduced motion, hidden tabs, switching pets during a load and GPU disposal in a real browser.

## Verification

Run Blender with `--background --factory-startup --python assets/desktop-pets/build_model.py`, then `node scripts/build-punk-pet-motions.mjs`. `node scripts/test-punk-pet.mjs` checks real encoded texture sizes, motion hashes, skinned WebGL movement and interaction/lifecycle. Existing `go test ./internal/desktop -run 'TestBundledPersonaSprites|TestInstallBundledDefaultPets|TestServiceBootstrapIncludesAllBundledDefaultPets|TestBundledPet'` and `go test ./ui -run 'DesktopPet|DesktopChat.*Pet'` cover catalog preservation and the shared runtime.

## Child DOX Index

None.
