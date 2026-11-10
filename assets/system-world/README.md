# System World city kit

## System World 2 expansion

`build_expansion.py` authors 42 additional modular designs and their articulated
animation clips in Blender. It saves `production/aurago-world-2.blend` and exports
126 self-contained GLBs (three separately loadable LODs per design) plus a versioned
manifest (2.4.0) and MIT license to `ui/3d/system-world/v2/`. Sources are not shipped.

The kit includes accessible floors, walls, ceilings, windows, sliding doors, lifts,
the agent tower's open sky lift and crown terrace, public telescopes,
stairs, ramps, bridges, gallery railings, arcades, gardens and quays; a tram, stop, service cart and
landing pad; courier, technician and archivist robots; consoles, hologram tables,
chargers, cargo, coolers, benches, archive shelves and the `hover-pack` charging dock.
Human-readable clip names, pivots, portals, colliders and walk surfaces are exported,
not guessed at runtime.
The existing white robot and its provenance below are unchanged.

`living_assets.py` supplies the repair bay, parcel sorter, relay antenna, kinetic
fountain, glass garden and sheltered charging/meeting point. Their exported
work slots, interaction points and collision boxes drive local robot routines.
Their small LOD loads initially; details load within 55/32 metres, capped by
the quality setting. The courier parcel is a separate visible component.

`landmark_assets.py` supplies the four civic landmarks: the offshore tidal
turbine, the kinetic plaza monument, the street gateway and the elevated
observation pod. Each owns one named animated part - the rotor, the three
counter-turning rings and the floating core, the travelling light band and the
sweeping scanner - so their motion is exported with the model instead of being
imitated at runtime. The turbine stands on the harbour water, the gateway stands
on the pavement so its arch clears the road, and the others are placed on city
plots by `sysworld-places.js`.

`robot_assets.py` authors the courier, technician and archivist residents as
detailed articulated humanoids with a real parent hierarchy: structure, torso and
head; arms and forearms; legs, shins and feet; plus the courier's `parcel` and the
archivist's `halo` and `slate`. Their six clips (idle, walk, turn, greet, work and
carry) are keyed on every frame and exported on every second frame. Legs and carried
hands are solved with two-bone IK, and the walk is calibrated so planted feet move
backwards at the residents' 1.65 m/s walking speed (1.15 scale). Every LOD of a part
has the same material list, so the runtime can swap LODs by mesh name; the rubber
parts use the robot-local `city.robot-rubber` material.

`hover_assets.py` authors the `hover-pack` charging dock: a removable `pack` node
(straps, charge bars and two fans) that hides while the visitor flies, an `idle` clip
that spins the fans, and one collider with a `take` interaction spot.

```powershell
& 'D:/Blender 5.2/blender.exe' --background --factory-startup --python assets/system-world/build_expansion.py
node scripts/test-system-world-expansion.mjs
node scripts/test-system-world-layout.mjs
node scripts/test-system-world-hover.mjs
node scripts/build-system-world.js
```

The 80 MiB budget covers both city kits, the original white robot, renderer,
classic app modules, stylesheet and System World labels in all sixteen locales.
The combined runtime model directory has a 40 MiB ceiling.
First display has a separate 20 MiB budget; interior furnishings and alternate
LODs are lazy. Exterior shells are present at first display. Modular floor seams
and lift landings use exact planar edges; the platform top matches its declared
zero-height surface and its left side opens onto the gallery.
Sound is synthesized locally through the opt-in mixer, with no external audio files.
Use `scripts/build-system-world-review.mjs` and the `AURAGO_SYSTEM_WORLD_MODELS=1`
browser test for contact sheets of every exported model, LOD and clip. Review
outputs stay under ignored `reports/aurora/`; they are not runtime resources.
Full integration and acceptance commands: `documentation/system-world-2.md`.

Original Blender assets for AuraGo's cinematic data metropolis. The kit uses dark
metal, tinted glass, bronze details and restrained warm interior lighting.
The original city-kit geometry and materials were authored for AuraGo under MIT.
The reused ThreeDee robot retains its existing artwork provenance (see below).

## Kit v3

`build_world3.py` authors the kit v3 designs in Blender 5.2 LTS. `kit3.py` is the shared core.
Five families are registered: `street_assets3.py` (street modules), `facade_assets3.py` (slab-and-glass
facades and the skyline data towers), `district_assets3.py` (the seven district buildings and the
skybridge), `furniture_assets3.py` (the street lamp and planter, trees, bench, bollard, litter bin,
crossing sign and kiosk) and `interior_assets3.py` (the six lobby interiors). Together they make 31
designs and 93 GLBs. `life_assets3.py` (phase 4) is not registered yet. Blender writes uncompressed GLBs
and `manifest.raw.json` to `disposable/system-world/v3-raw/` and saves
`production/aurago-world-3.blend`. The Node script then packs them with meshopt and writes
`ui/3d/system-world/v3/`.

```powershell
& 'D:/Blender 5.2/blender.exe' --background --factory-startup --python assets/system-world/build_world3.py
node scripts/build-system-world-v3.mjs
node scripts/build-system-world-v3.mjs --check
```

`--preview` also renders LOD 0 views to `reports/system-world-assets/v3/` (ignored, not shipped).
`--only <ids>` builds a subset and leaves the manifest and the source scene untouched.

The v3 `manifest.json` has `schema_version` 3 and these fields: `version`, `license`,
`generator`, `units`, `up_axis`, `sources` (SHA-256 of each authoring source), `assets`
(each with `id`, `name`, `category`, `components`, `navigation`, `animations` and
`lods`), `compression` (`EXT_meshopt_compression`), `pack`, `total_model_bytes` and
`validator_errors`. Each LOD entry has `level`, `file`, `triangles`, `bounds`, `bytes` and `sha256`.

The renderer loads these GLBs with `GLTFLoader.setMeshoptDecoder(MeshoptDecoder)`
(`sysworld-scene.js`).

## Enterable lobbies

Six district buildings (agent, memory, integrations, missions, infra, graph) have a real ground floor. Each
has a `lobby` stand-in part (ground-floor panes and closed door glass) in its exterior GLB. The
interior GLB replaces it on approach. The operations beacon has podium and stairs in the registry but no
interior.

- Runtime: `sysworld-lobbies.js` is the pure registry (podiums, stairs, halls, doors, the 0.35 m step rule,
  wall and solid boxes, door states, the residents' lobby graph). `sysworld-lobby-render.js` loads, shows,
  hides and disposes the interiors.
- Interiors load within 45 m of a door (3D, measured at eye height) and are released beyond 70 m. They are
  not part of the first display.
- Tiers: medium loads the LOD 1 interior, high and ultra load LOD 0, and low loads no interiors. On low the
  stand-ins stay in place.
- Visitors climb the stairs. No step above 0.35 m is allowed between samples, so the podium edges are closed
  except at the stairs. Door leaves open when a visitor or a resident comes near; the visitor's doors open
  only when the interior is shown and the tier is not low.
- Residents route over the pavement, the stairs, the porches, the doors and the lobby grid, and they work at
  the interior workpoints (`sysworld-places.js` `createWayfinder`).
- Collision: lobby walls with door gaps, corner piers, lintels, ceiling slabs and the atrium lid come from the
  registry (`lobbySolids`). Interior fixtures come from each interior's `navigation.colliders`.
- Lobby numbers mirror the authoring source; `scripts/test-system-world-lobby-contract.mjs` checks them against
  the packed GLBs.

## Surface textures

The kit's GLBs carry no UVs, so surface detail is a separate runtime set:
`build_textures.py` (numpy + Pillow) writes six seamless maps to
`ui/3d/system-world/textures/v1/`: asphalt, running-bond pavers, board-formed concrete,
metal panels, roof gravel and foliage. Each RGB WebP packs data, not colour:
R = albedo factor / 2, G = roughness factor / 2, B = height. The renderer
(`sysworld-surfaces.js`) multiplies the kit's own material colours, projects the maps
triplanar and derives bump, wetness and puddles from them. About 0.6 MB in total.

```powershell
python assets/system-world/build_textures.py --preview
python assets/system-world/build_textures.py --check
node scripts/test-system-world-surfaces.mjs
```

## Runtime payload

The versioned runtime files live in `ui/3d/system-world/v1/`. The existing web
resource manifest already includes that directory and the GLB/JSON/TXT formats.
Blender, MCP, Python scripts, source scenes and review renders are authoring tools;
none are required or shipped to run the app.

- 3 asset designs; three separately loadable LODs per design.
- 9 self-contained GLB files. The original v1 kit retains its **8 MiB** gate;
  the combined v1/v2/v3/robot runtime directory has a **40 MiB** ceiling.
- No image textures, external URIs, Draco decoder, skeletal animation or lights.
- Standard metallic/roughness materials and `KHR_materials_emissive_strength`.
- Shared named materials; static geometry is batched by material.
- Every manifest entry records SHA-256, file size, triangle count and Y-up bounds.

Assets: service drone, data tram and server rack. Kit v3 supersedes the other twelve
v1 designs (agent reactor spire, memory archive, integration gateway, mission terminal,
compute foundry, knowledge atrium, two data towers, operations beacon, skybridge, street
lamp and planter), and their files are no longer in v1.

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
- Kit v3 streets are assembled from modules that are never stretched: 4 m plain and drain
  modules, one 4 m zebra module on each intersection arm (centred 8 m from the intersection),
  1 m filler modules for whole-metre remainders, 12 by 12 m four-way intersections, and end
  caps at the island and quay edges. `streetModules()` in `sysworld-ground.js` gives the
  placement list. Street segments stop at the intersection edges, so no street module runs
  through a crossing.
- Kit v3 wins per id. `kitFor(id, catalog3)` in `sysworld-scene.js` loads the v3 design whenever
  the v3 manifest ships that id, and colliders read `kitCatalog(v1, v3)`.
- Window panes (`city.window`) are drawn by `ui/js/desktop/apps/sysworld-windows.js` as one shared
  glossy material with interior-mapped rooms. Low tier shows flat lit or unlit cells, medium adds
  parallax, and high adds robots and screens. Lit rooms follow the time of day. Surface maps never
  touch them.
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

Use **Blender 5.2.2 LTS**. From the repository root:

```powershell
& 'D:\Blender 5.2\blender.exe' --background --factory-startup --python assets/system-world/build_city.py
python assets/system-world/check_assets.py
```

The generator also runs through Blender MCP `execute_blender_code` in a dedicated
Blender instance. Load the script with `__file__` set to its absolute path.
It replaces only its named kit collection and saves a compressed source library to
`assets/system-world/production/aurago-city-kit.blend`.
Names are canonicalised so repeated GLB exports are byte-identical.

## Companion (third party, BSD-3-Clause)

`spot-source.json` pins the Boston Dynamics Spot from `codepawl/menagerie-glb`
(MuJoCo Menagerie, copyright 2021 Clearpath Robotics Inc.): URL, commit, byte size,
SHA-256 and licence URL. Download the 9.5 MB source into ignored scratch once, then
build the 291 KB runtime derivative:

```powershell
curl -L --fail -o disposable/system-world/boston_dynamics_spot-spot.glb "<pinned URL from spot-source.json>"
node scripts/build-system-world-spot.mjs
node scripts/build-system-world-spot.mjs --check
```

`gltfpack` compresses the geometry with meshopt and quantizes attributes; all 13
named joints, both materials and the texture-free payload survive. The exact source
is never shipped, and the verbatim BSD-3-Clause text travels in
`ui/3d/system-world/spot-LICENSE.txt` and `licences/texts/`. The renderer decodes
the GLB with the bundled `MeshoptDecoder`; a failed load only hides the decorative
walker. Verify with `node scripts/test-system-world-companion.mjs`.

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
