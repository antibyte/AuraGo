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
- Cap the pixel ratio with `renderer.setPixelRatio(Math.min(devicePixelRatio, 2))`
  and keep the frame loop stable on high-DPI displays.
- Keep input state separate from movement; multiply motion by a clamped delta.
- Use simple bounding spheres or boxes for deterministic collision checks.
- Reuse geometries, materials, vectors, and effect objects. Do not allocate
  transient Three.js objects inside the frame loop.
- Pause the loop and timers on `visibilitychange` when the tab is hidden, and
  dispose geometries and materials when a scene or game is rebuilt.
- Keep the camera oriented toward the gameplay goal and prevent the player from
  leaving the readable play space.
- Use generated images only as textures, backgrounds, decals, or UI. Do not
  claim that AuraGo generated a 3D model.
- Add HTML UI for instructions and status when it is clearer than 3D text.
- Preserve `window.__AURAGO_GAME_DIAGNOSTICS__`; report canvas readiness, scene
  name, frame rate, resource errors, and runtime errors.

Use the shared mandatory planning round with template `three` and perspective
`3d`; preserve working code on edits. Call `game_maker_validate` scope `startup`.
This observes visible-canvas startup and runtime errors, not 3D gameplay, resize
or collision correctness. Explicitly leave gameplay unverified. Phaser templates,
sprite helpers and their 2D gameplay tests do not apply to Three.js. A sprite PNG
is a texture, not a 3D character model. Optional image review is advisory only.
