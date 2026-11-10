# System World asset authoring

## Purpose
Own the original Blender city kit, deterministic generator and source scene.

## Ownership
- Authoring: this directory.
- Runtime payload: `ui/3d/system-world/v1/`, `v2/`, `v3/`, plus `white-robot.glb/json` in its parent.
- Kit v3 output: `ui/3d/system-world/v3/` (written only by `scripts/build-system-world-v3.mjs`,
  including its `LICENSE.txt`); raw scratch `disposable/system-world/v3-raw/` (written by `build_world3.py`).
- Temporary tools: `disposable/system-world/`; visual checks: `reports/system-world-assets/`.
- Surface textures: `build_textures.py` writes `ui/3d/system-world/textures/v1/` (six seeded,
  periodic RGB WebP data maps plus manifest and MIT license).

## Local Contracts
- Blender 5.2.2 LTS; original geometry and materials under MIT.
- Each asset has three isolated GLBs, Y up, metre units and base-centred origins.
- The complete runtime model directory must stay below 40 MiB; first display
  stays below 20 MiB and the full app below 80 MiB.
- Surface maps are original procedural data (R albedo factor / 2, G roughness factor / 2,
  B height), seamless, byte-stable for the pinned numpy/Pillow and verified with `--check`.
  The GLBs stay texture-free; the renderer projects the maps (see `sysworld-surfaces.js`).
- Original city-kit designs have no image textures or external URIs. Preserve
  separate rotor and signal nodes. The explicitly requested ThreeDee robot
  derivative retains three embedded 512 px PBR textures and source provenance;
  do not relabel its existing artwork as newly authored MIT geometry.
- `build_robot.py` owns that derivative (24,000 triangles, below 2 MiB), never
  changes `ui/3d/robot.glb`, and exports tangents without runtime Draco.
- Only runtime GLBs, manifest and license belong in the resource package.
- Keep the planned cinematic city direction and street-level inspection quality.
- `build_expansion.py` owns 42 v2 designs and `production/aurago-world-2.blend`.
  The `quay-lantern` is the low warm waterfront lamp (bronze post, separate
  emissive `glow` part, no clip); `sysworld-experience.js` places six along the
  quay off the street grid and clear of the harbour hover dock.
  The `telescope` tube pivots at its yoke and faces glTF +Z at rest. The `sky-lift`
  cab travels `SKY_TRAVEL` (71.32 m) from the 0.88 m agent podium to the `sky-deck`
  (top at zero, placed at 72.2 m); keep both equal to `skyDeck` in
  `ui/js/desktop/apps/sysworld-layout.js`. The lift shaft is an open lattice (the
  kit's glass is opaque), its tower and boarding sides stay open at the landings.
  `living_assets.py` authors the repair bay, parcel sorter, relay mast, kinetic
  fountain, glass garden and sheltered charging/meeting point. Export their
  colliders, work slots, interaction and emitter markers at all three LODs.
  The courier's named `parcel` component follows its pickup/delivery cycle.
  `landmark_assets.py` authors the tidal turbine, data monument, neon gate and sky
  pod. Each owns one named animated part (rotor, rings and core, light band,
  scanner); the exported clip drives that part, and the pivot must stay at the
  part's own centre. The v2 manifest pins hashes of all six authoring sources.
  Robot `motion_bounds` enclose all sampled exported clips; collision dimensions
  include raised arms and remain identical across LODs.
  Tram end, side and sliding-door windows are framed apertures at every LOD;
  never place opaque kit glass or backing walls across passenger sight lines.
  Preserve matching LOD pivots and articulated clips. Modular floors have planar
  joining edges; the lift platform top is zero with four metres of travel and an
  open left side. Gallery guards use the `railing` module. Verify actual exported
  floor/shaft alignment with `node scripts/test-system-world-layout.mjs`.
- Kit v3 is authored by `kit3.py` (shared core: materials, baked AO, export) and `build_world3.py`
  (run entry point, `FAMILIES`, kit version 3.3.0). Registered families: `street_assets3.py`,
  `facade_assets3.py`, `district_assets3.py`, `furniture_assets3.py` and `interior_assets3.py` (31 designs
  together), plus `life_assets3.py` (phase 4, 15 designs; 46 in total). `interior_assets3.py` imports its
  `torus` and `face` helpers from `life_assets3.py`.
- Soft parts: a `kit3.py` factory returns `'soft': ['<part>']` in its info, and those parts export with
  smooth normals at every LOD. Tree canopies (`crown`), the planter and the flower bed (`foliage`) use
  this; new foliage goes in a soft part. The interior pot plants (`interior_assets3.py` `pot_plant`) do not
  yet (phase 6 backlog item 23).
- Street trees (`furniture_assets3.py` `tree`): `tree-round` is 6.4 m tall with a canopy about 4.6 m wide;
  `tree-column` is 7.6 m tall with a 2.4 m canopy. The canopy starts above head height.
- `life_assets3.py` (phase 4) authors harbour launch, cargo barge, sail skiff, channel buoy, mooring post,
  flower bed, cafe set, charge post, info pylon, utility cabinet, three neon signs, banner mast and gull.
  Boats have their origin on the waterline amidships, bow towards +X. Pivoted parts export as nodes with
  pivots in Blender axes: `rig` (sail skiff, `(.7, 0, 1.25)`), `lamp` (buoy, `(0, 0, 2.05)`), `banner`
  (banner mast, `(.14, 0, 6.55)`), and `wing_l`/`wing_r` (gull, hinged at the shoulders). The gull flies
  towards +X. The banner cloth uses the matt `city.marking` material.
- The charge post's workpoint `charge` sits on the pad at the origin. Its collider lies at glTF z -0.84 to
  -0.4, behind the pad on the pylon side.
- Furniture `navigation.colliders` mirror `SIZES` in `ui/js/desktop/apps/sysworld-furnishing.js`
  (`[x0, z0, x1, z1]` footprints); `scripts/test-system-world-furnishing.mjs` checks the packed manifest
  against it.
- Kit v3 geometry: every primitive carries `COLOR_0` baked ambient occlusion (greyscale,
  multiplied with the albedo by the renderer); node names equal component names; materials are `city.*`.
  New in kit v3: `city.kerb`, `city.setts`, `city.iron`, `city.marking` and `city.tactile`, each
  mapped in `sysworld-surfaces.js` `KINDS`. `city.window` is the window pane material (see Window contract).
- Kit v3 packing: `gltfpack@1.3.0 -cc -kn -km -ke -kv -vtf`, no simplification (the packed triangle
  count must equal the raw count). Every LOD keeps the carriageway, kerb and pavement surfaces.
  `-vtf` keeps TEXCOORD_0 as floats; quantised texture coordinates would be normalised to 0..1 and
  lose the window room coordinates.
- Window contract: `city.window` panes come from `pane()` in `facade_assets3.py`, used by `glass_tier`,
  `tower` and the district buildings. One flat quad per face per glass tier; a tower adds a ground-floor
  lobby pane per face. TEXCOORD_0 holds room coordinates: u is one unit per bay, left to right seen from
  outside, +16 per face; v is one unit per storey, 0 at the first upper floor, -1 for the lobby, and
  +20 per tier in district buildings. Corners sit on whole rooms. The exporter flips V and the renderer
  undoes it (`sysworld-windows.js`). `scripts/test-system-world-v3.mjs` checks every pane of the decoded
  GLBs: bays 0.8-6 m, storeys 1.2-6 m, u running left to right from outside, and `-vtf` in the manifest.
- No separate lit-window quads exist in kit v3. Vertex-coloured `city.ivory` and `city.warm` parts
  (light lines, diffusers) are light fixtures in the renderer and do not switch per pane.
- Enterable lobby contract (`interior_assets3.py`, category `interior`): `lobby-agent`, `lobby-memory`,
  `lobby-integrations` (two halls in one asset), `lobby-missions`, `lobby-infra` and `lobby-graph`.
  Each is authored in its building's own frame (same origin, front towards -Y in Blender, glTF +Z).
  The floor finish sits at `FLOOR` = `PODIUM` + 0.02 m. The glazing uses palette key `pane`
  (`city.pane`) at the positions of the stand-in panes; the renderer swaps in its own material.
- Door leaves are separate nodes `door_<n>_l` and `door_<n>_r`. Underscores only: `GLTFLoader` strips dots
  from node names. Each leaf is half a door wide, its pivot is at its bottom centre on the podium top, and
  it is authored closed. The renderer slides the leaves along X.
- `navigation.colliders` (fixture boxes) and `navigation.workpoints` are building-local glTF values. A
  collider is `[min x, min y, min z, max x, max y, max z]` with base `PODIUM`; workpoints stand at `FLOOR`.
  Authoring rules: each workpoint keeps at least 0.6 m from every fixture box, and aisles and passages beside
  counters are at least 1.4 m wide (a resident's routing body is about 1 m across).
- Anything that hangs in the doorway of the closed building (for example the mission terminal's dock rails)
  belongs to the building's `lobby` stand-in part, not to the interior.
- Exterior `lobby` part: the ground-floor panes (`city.window`, room coordinates) and the closed door glass
  (`city.glass`, 2.92 to 3.5 m high, sill at `PODIUM`). The renderer hides it while the interior is shown.
- Stairs (`facade_assets3.py` `stairs`): `STEPS` = 5 treads of `TREAD` = 0.32 m; six equal risers of
  `PODIUM / 6` under the `PODIUM` = 0.88 m top.
- Every number mirrors `ui/js/desktop/apps/sysworld-lobbies.js`. `scripts/test-system-world-lobby-contract.mjs`
  checks the packed GLBs against it.
- Kit v3 street modules follow `ui/js/desktop/apps/sysworld-ground.js` (`kerb`, `streetProfile`,
  `streetModules`) within 1.5 cm at LOD 0, 2 cm at LOD 1 and 3 cm at LOD 2. Corner arcs use
  10, 6 and 4 segments per 90 degrees at LOD 0, 1 and 2. `street-end` overlays the outer 0.64 m
  and its wall reaches −0.62 m.
- Kit v3 heights as authored: carriageway −0.14, pavement 0 (level with the plots), kerb rise
  0.14 over a 0.14 m splay, dropped kerbs at zebra crossings. The island body top is −0.2
  (`island.top` in `sysworld-ground.js`).
- The v3 manifest pins SHA-256 hashes of its authoring sources (`kit3.py`, `build_world3.py`,
  `street_assets3.py`), computed over LF-normalised bytes.
- The v1 `street-tile` and `street-crossing` have left the payload, and kit v3 supersedes twelve more
  v1 designs (`agent-spire`, `memory-archive`, `integration-gate`, `mission-terminal`,
  `compute-foundry`, `knowledge-atrium`, `operations-beacon`, `data-tower-a`, `data-tower-b`,
  `skybridge`, `street-lamp`, `planter`). v1 now holds three designs (`service-drone`, `data-tram`,
  `server-rack`), and `check_assets.py` asserts 3. `build_city.py` `ASSETS` lists only these three; the
  factories of the retired designs stay in the file as reference. Its hash is pinned in the v2 manifest
  `sources`, so any edit to `build_city.py` needs that entry updated.
- `robot_assets.py` and `hover_assets.py` are called from `build_expansion.py`. Each
  LOD of a part keeps one material list and stable mesh names (the material lists are
  asserted at build time), so the runtime swaps LODs by name. The `hover-pack` `pack`
  node, with its `straps`, `charge` and fan children, stays one removable node that the
  runtime hides while the visitor flies. The robot rubber `city.robot-rubber` is mapped
  in   `sysworld-surfaces.js` `KINDS`. Check dock placement with
  `node scripts/test-system-world-hover.mjs`.
- `spot-source.json` pins the third-party companion: a BSD-3-Clause Boston
  Dynamics Spot from `codepawl/menagerie-glb` (copyright 2021, Clearpath
  Robotics Inc.), with URL, commit, byte size, SHA-256, upstream
  `mujoco_menagerie` path and licence URL. The 9.5 MB source stays in ignored
  `disposable/system-world/` and is never shipped. `scripts/build-system-world-spot.mjs`
  verifies the hash, runs `gltfpack -cc -kn -km -si 0.55 -sa`, validates with
  the Khronos validator and writes the compact `ui/3d/system-world/spot.glb`,
  its manifest and the verbatim licence. Meshopt compression is lossless for the
  emitted data; only the triangle count is reduced for city scale. Keep all 13
  named joints, both materials, no textures and no remote URIs. `--check`
  reproduces the payload without writing. The licence text lives in
  `licences/texts/system-world-spot-BSD-3-Clause.txt` and travels with the asset.

- Sound recordings (`audio-sources.json`, `build_audio.py`, `ui/3d/system-world/audio/`): add or replace a
  recording only with the user's explicit approval for that download. Rebuild one recording with
  `build_audio.py --sources <dir> --only <id>,<id>`. Each source must be CC0-1.0; record
  its page, author, licence and source SHA-256 in `audio-sources.json`. The shipped `manifest.json` carries
  no URLs and stays that way. Never hand-edit the shipped `audio/` folder; rebuild it with `build_audio.py`.

## Verification
- `python assets/system-world/check_assets.py`.
- `node scripts/test-system-world-furnishing.mjs` (furniture rules, `SIZES` against the packed colliders,
  the charge-post contract).
- `node scripts/build-system-world-spot.mjs --check` and
  `node scripts/test-system-world-companion.mjs` verify the pinned source,
  the compact derivative, its provenance, licence texts and the model budget.
- `node scripts/test-system-world-expansion.mjs` (42 designs, 126 GLBs),
  `node scripts/test-system-world-foley.mjs` (city beds and the lantern hum).
- Sound: `node scripts/test-system-world-soundscape.mjs`, `-zones`, `-sound-meter`, `-sound-space`, `-sound-beds`,
  `-sound-movers`, `-sound-rooms`, `-sound-frame` and `-recordings` (in `scripts/`; the shared fake of the
  Web Audio graph is `scripts/system-world-audio-fake.mjs`), and `python assets/system-world/build_audio.py --check`.
- `node scripts/build-system-world-v3.mjs --check` (validates the packed kit v3 files and sources and
  reproduces them byte for byte when `disposable/system-world/v3-raw/` exists),
  `node scripts/test-system-world-v3.mjs` (manifest schema 3, LODs, hashes, budget),
  `node scripts/test-system-world-street-models.mjs` (street models against the profile),
  `node scripts/test-system-world-ground.mjs` (street profile, module assembly, pedestrian graph).
- `node scripts/test-system-world-layout.mjs` checks exported tram window sight lines
  from inside and outside, including closed doors at all three LODs.
- `python assets/system-world/build_textures.py --check` and `node scripts/test-system-world-surfaces.mjs`;
  `--preview` writes a lit contact sheet to `reports/system-world-assets/`.
- All exported GLBs must pass the Khronos validator and a real browser rendering check.
- Render previews from the exported GLBs. Repeated generation must preserve GLB hashes.

## Child DOX Index
None.
