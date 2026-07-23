---
name: aurago-threejs-gameplay
description: Build efficient playable 3D browser games with the pinned Three.js runtime.
license: MIT
compatibility: Three.js 0.185.1
metadata:
  managed_by: aurago
  source: majidmanzarpour/threejs-game-skills
  commit: 7221c1f4a6d2ae189a4d85d058d24f3228499d46
allowed-tools: game_maker_project, game_maker_file, game_maker_asset, game_maker_validate
---

# Three.js Gameplay

Import Three.js from `../vendor/three-0.185.1.module.min.js`. Build a real game,
not a passive scene.

- Establish renderer, scene, camera, resize handling, lighting, and a bounded
  animation loop before adding content.
- Keep input state separate from movement; multiply motion by a clamped delta.
- Use simple bounding spheres or boxes for deterministic collision checks.
- Reuse geometries, materials, vectors, and effect objects. Do not allocate
  transient Three.js objects inside the frame loop.
- Keep the camera oriented toward the gameplay goal and prevent the player from
  leaving the readable play space.
- Use generated images only as textures, backgrounds, decals, or UI. Do not
  claim that AuraGo generated a 3D model.
- Add HTML UI for instructions and status when it is clearer than 3D text.
- Preserve `window.__AURAGO_GAME_DIAGNOSTICS__`; report canvas readiness, scene
  name, frame rate, resource errors, and runtime errors.

Validate WebGL startup, resize behavior, controls, collision feedback, and a
stable frame loop before polishing materials and effects.
