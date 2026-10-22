# Orbit desktop theme model pack

## Purpose
Own the original Blender model pack of the Orbit desktop theme (space station, shuttle,
satellite, asteroid rocks), its deterministic generator and the editable source scene.

## Ownership
- Authoring: this directory (`generate.py`, `orbit-models.blend`).
- Runtime payload: `ui/3d/desktop-theme-orbit/v1/` (four GLBs, `manifest.json`, `LICENSE.txt`).
- Consumer: `ui/js/desktop/theme-fx/orbit-scene.js` loads the GLBs by node name.
- Check: `scripts/test-desktop-theme-orbit-assets.mjs` (part of `npm run check:desktop-themes`).
- Preview renders go outside the repository (use a scratch directory), never into the tree.

## Local Contracts
- Blender 5.2 LTS; original geometry and materials under MIT, no third-party content.
- Everything is built from code in metres in the glTF frame (Y up, +Z forward) and exported
  with `export_yup`. No textures, no UVs, no images or samplers in any GLB: detail comes from
  geometry, bevels, inset faces and vertex colours (`COLOR_0`, linear, multiplied onto the
  material base colour). Materials are exported single-sided.
- Re-running the generator with the same Blender version must reproduce identical GLB
  hashes (seeded `random.Random`, `mathutils.noise` with fixed offsets, explicit exporter
  options). The manifest pins SHA-256, bytes and triangles per file and the Blender version;
  regenerate after any generator edit.
- Budgets: whole pack at most 1,572,864 bytes; triangles station 26,000, shuttle 7,000,
  satellite 5,000, every `RockN` mesh 800 (they are built at 752).
- Required node names (the runtime looks them up): station `RingRotor` (empty, the habitat
  ring, spokes, pods, ring windows and ring lights are its children and spin about Y),
  `BeaconRed`, `BeaconGreen`, `DockPort` (empty at `(0, -0.85, 0)`, docking axis -Y);
  shuttle `EngineGlow`, `Exhaust` (empty 0.02 behind the nozzle mouths); satellite `Dish`
  (empty at the gimbal, boresight +Z, holds `DishReflector` and `DishFeed`); asteroids
  `Rock0` .. `Rock4`.
- Material names are shared across the files and drive runtime behaviour: `HullWhite`,
  `HullDark`, `GoldFoil`, `SolarPanel`, `Glass`, `Rock` (white base, vertex colour carries
  the albedo) and the emissive `WindowsEmissive`, `DockLightsEmissive`,
  `BeaconRedEmissive`, `BeaconGreenEmissive`, `EngineEmissive`. Emissive materials export a
  non-zero `emissiveFactor` plus `KHR_materials_emissive_strength`; keep `Emissive` in the
  name of every glowing material.
- Frames: the station spin axis is Y, ring centreline radius 1.0, spine y from -0.85 to
  +0.85 (dish mast reaches about 1.09), solar wings along +-X at y = 0.55 out to |x| = 1.725.
  The shuttle faces +Z and its origin is the tip of the nose docking probe (it docks nose
  first; body toward -Z). The asteroid meshes are centred on their own origin (radius about
  1, extents up to 1.43) and their nodes sit at x = 0, 3, 6, 9, 12 only for layout.
- Meshes are merged per material (one draw call per material and group); keep the split
  between static hull parts and the rotor group, and keep emissive parts in their own meshes
  (`RingWindows`, `RingLights`, `SpineWindows`, `DockLights`, `AccentLights`, `EngineGlow`,
  `NoseLights`, the nav lights and beacons) so the runtime can drive them separately.
- The asteroid rocks place their vertices on purpose (crater floor/wall/rim rings plus an
  even background, triangulated by a spherical convex hull) rather than decimating an
  icosphere, so crater rims survive at 752 triangles.

## Verification
- `& 'D:/Blender 5.2/blender.exe' --background --factory-startup --python assets/desktop-theme-orbit/generate.py`
- `node scripts/test-desktop-theme-orbit-assets.mjs`
- Determinism: run the generator twice and compare the SHA-256 of the four GLBs.
- Visual review: append `-- --preview <scratch dir>` to render EEVEE 1024 x 768 previews of
  every model (hero view plus close-ups); the Khronos validator (`gltf-validator`) must
  report no errors on the GLBs.

## Child DOX Index
None.
