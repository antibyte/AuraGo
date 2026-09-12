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

Prefer a guided base (`fps`, `exploration`, `transport`, `flight`, `space`) via
`set_design`. Edit its short main.ts config and game-specific hooks, retaining
common.ts model loading, animation and lifecycle. All selected roles are bound.
The following engine reference applies when extending that base or using free
code (`three`). Import the pinned local Three.js runtime; build a playable game.
Scene-backed `three` is opt-in. An accepted plan with a scene emits the
canonical `src/scene.json`; `startGame(config)` loads it and the shared loop
runs the builder alongside `config.action` and `config.step`. A null scene keeps
the legacy `config.objects` path. Scene mechanics are optional, so `minimal` and
free-code `three` can be continuous without a forced combat rule or timer. A
free-code `three` startup pass still leaves gameplay unverified until live
observations cover it.

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
- Use original built-in GLBs through the local `aurago-three-assets-1.js` helper.
  Image generation still produces textures, backgrounds, decals or UI, not meshes.
  Describe and selectively import models; never substitute catalog URLs in games.
- Add HTML UI for instructions and status when it is clearer than 3D text.
- Preserve `window.__AURAGO_GAME_DIAGNOSTICS__`; report canvas readiness, scene
  name, frame rate, resource errors, and runtime errors.

Use the shared mandatory planning round with template `three` and perspective
`3d`, schema_version 3 and units `metres`; preserve working code on edits.
Model roles use metric `scale` and `collider: catalog|box|sphere|capsule|mesh|none`,
without sprite display_height or pixel origins. At most 64 concrete model roles.
Use one game clock for `updateInstance`, independent instances for animated rigs
and `createInstances` for repeated static props. Release instances, asset handles,
lights/shadows, controls and the renderer on teardown; abort pending model loads.
Call `game_maker_validate` scope `full` for guided bases. Required live-input
checks include movement, primary action, objective progress, assets, end and two
restarts; FPS also checks aim and reload. Free-code `three` uses scope `startup`
only and must leave gameplay unverified. Additional requirements need observation. Phaser templates,
sprite helpers and their 2D gameplay tests do not apply to Three.js. A sprite PNG
is a texture, not a 3D character model. Optional image review is advisory only.

For presentation, use the local aurago-effects-3d-1.js controller documented in the job context. The five guided bases already update/render/dispose it. Register real ground/roof meshes, send actual hit points and world normals, and use the same camera as the audio listener. Point emitters require an explicit position. Keep HUD in DOM outside the scene renderer. No duplicate animation loop or AudioContext.
