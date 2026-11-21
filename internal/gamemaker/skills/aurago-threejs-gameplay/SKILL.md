---
name: aurago-threejs-gameplay
description: Build efficient playable 3D browser games with the pinned Three.js runtime.
license: MIT
compatibility: Three.js 0.186.1
metadata:
  managed_by: aurago
  source: majidmanzarpour/threejs-game-skills
  commit: 7221c1f4a6d2ae189a4d85d058d24f3228499d46
allowed-tools: game_maker_project, game_maker_file, game_maker_asset, game_maker_preview
---

# Optional Three.js and Voxel guidance

Use local Three.js modules (`vendor/three-0.186.1.module.min.js`) when building 3D games.
`game_maker_project` `add_library` (`library: "three"` or `"voxel"`) installs local
modules into `vendor/` without overwriting source; `reference` (`framework: "three"` or
`"voxel"`) returns helper documentation, and `use_template` installs optional starters
(`three`, `fps`, `exploration`, `transport`, `flight`, `space`, `voxel`).

## 3D atmosphere, camera & feel

- **Lighting & depth:** Combine a `HemisphereLight` (sky/ground fill) with a shadow-casting `DirectionalLight`, plus `THREE.FogExp2` or `THREE.Fog` matching `scene.background` for atmospheric depth and clear silhouettes.
- **Camera feel:** Use exponential damping (`1 - Math.exp(-lambda * dt)`) for smooth third-person follow; briefly widen `camera.fov` during dashes/boosts and apply subtle shake offsets on impacts.
- **Controls:** Keep movement camera-relative, clamp `dt` to `0.05s`, and support both mouse drag / pointer-lock and keyboard turning so preview and casual play both work cleanly.
- **Readable feedback:** Pulse collectible emissive intensity, spawn expanding rings or particle bursts on pickups/shots, and trigger `vendor/zzfx.js` SFX or `generate kind="sfx"` audio.

## Low-poly model lifecycle & performance

- Inspect `describe_asset` for exact forward axes (`+Z`), scales (metres), animation clip IDs, sockets and colliders before writing animation code.
- Create independent instances per animated actor with `createInstance(asset)` so each has its own skeleton and animation state; advance once per frame via `updateInstance(record, dt, camera)`.
- Use `createInstances(asset, matrices)` (instanced rendering) for static trees, rocks or props.
- Dispose geometries, materials, textures, listeners, animation loops, and call `disposeInstance` + `releaseAsset` on scene teardown/restart. Keep existing Voxel save compatibility when editing Voxel worlds.

Own rendering, controls, camera, rules and HTML UI freely. Writes save only; `build`
reports compiler/import errors, and `preview` provides real WebGL screenshots,
contact sheets, `probe` (`window.__gameDebug`) and input. `finish` publishes a
successfully built snapshot.
