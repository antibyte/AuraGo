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

Use local Three.js modules when appropriate. project reference returns documentation
for installed helpers. project use_template offers three, fps, exploration,
transport, flight, space and voxel as optional starting points. Read existing
authored files before replacing them. No template or gameplay structure is required.

Imported models have exact local paths, scales, forward directions, clips and rig
metadata. Read describe_asset; independent animated figures need separate skeletons
and actions. Dispose owned resources, listeners and animation loops when replacing
a scene. Keep existing Voxel save compatibility when editing a saved world.

Own rendering, controls, camera, rules and HTML UI freely. Writes save only; build
reports compiler/import errors. The independent preview supports real screenshots,
logs and input without canvas hooks. finish publishes a successfully built snapshot.
