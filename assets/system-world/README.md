# System World city kit

Original Blender assets for AuraGo's cinematic data metropolis. The kit uses dark
metal, tinted glass, bronze details and restrained warm interior lighting.
The original city-kit geometry and materials were authored for AuraGo under MIT.
The reused ThreeDee robot retains its existing artwork provenance (see below).

## Runtime payload

The versioned runtime files live in `ui/3d/system-world/v1/`. The existing web
resource manifest already includes that directory and the GLB/JSON/TXT formats.
Blender, MCP, Python scripts, source scenes and review renders are authoring tools;
none are required or shipped to run the app.

- 17 asset designs; three separately loadable LODs per design.
- 51 self-contained GLB files. A hard **8 MiB** ceiling covers the complete
  runtime directory, including the manifest and license.
- No image textures, external URIs, Draco decoder, skeletal animation or lights.
- Standard metallic/roughness materials and `KHR_materials_emissive_strength`.
- Shared named materials; static geometry is batched by material.
- Every manifest entry records SHA-256, file size, triangle count and Y-up bounds.

Assets: agent reactor spire, memory archive, integration gateway, mission terminal,
compute foundry, knowledge atrium, two data towers, operations beacon, skybridge,
street segment, crossing, service drone, data tram, street lamp, planter and server rack.

## Integration contract

The assets are integrated through the isolated city renderer in System World.

- GLB coordinates use **metres, Y up**. Origins are centred horizontally at ground
  level. A bridge origin is its deck base; the host chooses the elevation.
- Each file contains exactly one asset root with `userData.asset_id` and
  `userData.lod`. Child nodes expose stable `userData.component` values.
- Drone rotors are individual nodes with centred local pivots; rotate them about
  local Y. The operations beacon has a separate `signal` component.
- Geometry is static. Window light patterns are decorative, not telemetry.
  Actual statuses, traffic, colours and movement come from the application's
  verified data model.
- LOD 0 is for close inspection, LOD 1 for district views, LOD 2 for distant
  buildings. Load one level per instance and use screen-size thresholds with
  hysteresis. Do not keep all three levels resident for every city instance.
- Cache source geometry by asset/LOD. Instance repeated mesh primitives and
  deduplicate materials by their `city.*` names. Clone a material only when a
  specific entity needs its own appearance; preserve the shared base material.
- The street segment is 16 by 12 metres and the crossing 12 by 12 metres.
  Stretch the segment along its local X axis to fill the space between crossings.
  Do not overlay complete streets through crossings.
- Keep collision/navigation geometry separate. Use the manifest bounds for
  coarse picking/culling and simplified building obstacles, never detailed facade
  triangles for walking collision.
- The matching isolated Three.js 0.185.1 renderer can load the files directly.
  Do not replace the desktop's shared legacy Three.js global just to load this kit.

## White ThreeDee robot

`build_robot.py` creates `ui/3d/system-world/white-robot.glb` from the existing
`ui/3d/robot.glb`. This is a derivative of that artwork, not a new MIT-authored
city-kit design; `white-robot.json` retains source/output hashes and provenance.
The original and ThreeDee theme remain untouched. Blender decimates 1,446,104
triangles to 24,000 and retains its three embedded PBR maps at 512 px, including
explicit tangents. No runtime Draco decoder is needed. The robot is capped at
2 MiB; it and the original city kit together remain below 8 MiB. Five residents
share the loaded geometry, materials and textures, with separate street poses.

Rebuild with:
`& 'D:\Blender 5.2\blender.exe' --background --factory-startup --python assets/system-world/build_robot.py`

## Rebuild

Use **Blender 5.2.1 LTS**. From the repository root:

```powershell
& 'D:\Blender 5.2\blender.exe' --background --factory-startup --python assets/system-world/build_city.py
python assets/system-world/check_assets.py
```

The generator also runs through Blender MCP `execute_blender_code` in a dedicated
Blender instance. Load the script with `__file__` set to its absolute path.
It replaces only its named kit collection and saves a compressed source library to
`assets/system-world/production/aurago-city-kit.blend`.
Names are canonicalised so repeated GLB exports are byte-identical.

`render_preview.py` imports the exported GLBs into a separate scene and renders
a city arrangement plus a close view. Run it in Blender's scripting workspace or
through the same MCP tool. It writes only to ignored `reports/system-world-assets/`.
This presentation scene is not the interactive application's telemetry layout.

## Checks

`check_assets.py` is a dependency-free check for hashes, size limits, triangle
counts, self-contained buffers, a single isolated asset per file, expected
components, finite bounds, LOD reduction and missing/stale catalog entries.

The official Khronos validator can additionally be installed as a temporary
authoring dependency:

```powershell
npm install --prefix disposable/system-world/browser --no-audit --no-fund --save-exact gltf-validator@2.0.0-dev.3.10 three@0.185.1
```

Validate every GLB with `gltf-validator.validateBytes`, then inspect the actual
models with Three.js at all detail levels, desktop and touch sizes. Do not accept
a beauty render of higher-detail source geometry in place of the shipped files.

## Local MCP setup

The development machine uses `ahujasid/blender-mcp` **1.9.1**, registered as
`blender` in Codex, with the matching Blender addon (protocol 5).
The connection is local loopback, port 9876. Telemetry is disabled both in the
MCP process environment and the addon's saved preferences. No cloud asset or
paid model generation services are needed.

Blender must be running for scene operations. A new Codex session loads the new
MCP tool registration; an already-running session can use the same server through
a normal MCP stdio client. The local installation is not an AuraGo runtime dependency.
