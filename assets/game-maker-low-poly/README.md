# AuraGo Low Poly production

Original MIT designs, Blender 5.2.1 LTS. No third-party meshes, texture artwork,
rigs or animations are included. The common palette and deliberately faceted
silhouettes connect transport, aviation, space, architecture, nature and FPS.
`catalog.py` is the authoritative 220-model inventory; variants and LODs are not
counted as separate designs.

## Rebuild

From the repository root, using the installed Blender executable:

```powershell
& 'D:\Blender 5.2\blender.exe' -b --python assets/game-maker-low-poly/build.py
& 'D:\Blender 5.2\blender.exe' -b --python assets/game-maker-low-poly/render.py
python assets/game-maker-low-poly/verify.py --boards
node scripts/build-game-maker-3d.js
node scripts/validate-game-maker-glbs.mjs
```

The generator retains ten compressed, editable category `.blend` files in
`production/`. Geometry, vertex colors, bones, sockets, NLA clips and LOD inputs
are authored through Blender Python. `--only id,id` is useful while iterating,
but overwrites that category's source scene: perform a full final build before
shipping. The preview renderer imports the exported GLB, not the source scene.
It excludes Blender's hidden bone-display Icosphere from camera bounds.

The runtime directory is
`internal/gamemaker/asset_packs/aurago-low-poly/`. It contains only GLBs, small
WEBP previews, metadata and MIT license. The shared humanoid animation library
is stored once. No source Blender files or Python code enter the web resource set.
All runtime files, including every LOD/preview/clip/license, count toward 100 MiB.
The separate Three.js engine/helper is an existing Game Maker runtime dependency.

## Coordinates and materials

- Authoring helpers accept metres in X/+Y-up/+Z-forward coordinates and convert
  to Blender coordinates at the modeling boundary. glTF export restores +Y up.
- Ground objects are base-centered. Aircraft/space objects use their centers;
  FPS rigs use their documented view/grip frame.
- Architecture uses a 2 m module grid, 3 m storeys and actual openings.
  Roads are 8 m wide. Connection positions and directions are in metadata.
- Four material classes at most: surface, metal, glass, emissive. Vertex colors
  supply the palette. Duplicate reversed faces are rejected by the shared mesh
  builder; double-sided materials handle thin leaves/cloth without doubled faces.
- Larger meshes include 100/50/20% target LODs. Small objects retain one LOD.
  Ratios are approximate because connectors and small separate meshes survive.
- Root dimensions, colliders, moving part axes, sockets, rigs, actual clips,
  timing events, speeds and file hashes are generated from the production data.

## Animation

Humanoids share one bone layout and 34 in-place actions. Animals use independent
species rigs/gaits; birds have wing and flight actions. Articulated objects have
baked open/close clips plus named game-driven pivots.

FPS arms and weapons use identical timing and view motion. Weapons are placed
at `[0.12,0.01,0.145]` relative to their shared view parent; the arms remain at zero.
Pistol arm actions have a `pistol_` prefix. Magazine and shot events identify
game timing, not automatic gameplay effects. Never apply weapon recoil twice
by parenting a baked moving weapon beneath a moving hand.

## Acceptance

```powershell
$env:GAMEMAKER_MODEL_BROWSER='1'
go test ./internal/gamemaker -run 'TestModel(PackBrowser|ReferenceExports)' -count=1 -v -timeout 8m
```

The tests use installed Chrome and the real local GLBs/runtime. Reports include:

- category contact sheets rendered from all 220 GLBs;
- per-file Khronos validation, including explicit warnings;
- independent skeletons, clip loading, LOD rendering and resource release;
- humanoid/animal/FPS motion frames and measured grip offsets;
- Studio Standard/Fruity light/dark at 1920, 1366 and 390 px, both densities;
- five small playable references exported through the real ZIP writer;
- 200 static objects, 20 animated rigs and eight vehicles, with GPU identification,
  load/render measurements and frame-time percentiles.

The validator's `NODE_SKINNED_MESH_NON_ROOT` warning describes Blender's retained
identity parent hierarchy; empty named socket nodes are informational. Neither
is suppressed. Runtime transforms and independent skeleton behavior are checked
in Three.js. New errors must fail the validator command.

`reference-game.ts` is original executable fixture source, not a runtime service.
The Go fixture runs the normal planning/import/build/export path and supplies only
selected per-model metadata. It does not invoke an LLM or fabricate a normal
agent job's gameplay acceptance.
