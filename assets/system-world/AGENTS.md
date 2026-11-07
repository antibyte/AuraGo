# System World asset authoring

## Purpose
Own the original Blender city kit, deterministic generator and source scene.

## Ownership
- Authoring: this directory.
- Runtime payload: `ui/3d/system-world/v1/`, `v2/`, plus `white-robot.glb/json` in its parent.
- Temporary tools: `disposable/system-world/`; visual checks: `reports/system-world-assets/`.
- Surface textures: `build_textures.py` writes `ui/3d/system-world/textures/v1/` (six seeded,
  periodic RGB WebP data maps plus manifest and MIT license).

## Local Contracts
- Blender 5.2.2 LTS; original geometry and materials under MIT.
- Each asset has three isolated GLBs, Y up, metre units and base-centred origins.
- The complete runtime model directory must stay below 12 MiB; first display
  stays below 12 MiB and the full app below 48 MiB.
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

## Verification
- `python assets/system-world/check_assets.py`.
- `node scripts/build-system-world-spot.mjs --check` and
  `node scripts/test-system-world-companion.mjs` verify the pinned source,
  the compact derivative, its provenance, licence texts and the model budget.
- `node scripts/test-system-world-expansion.mjs` (42 designs, 126 GLBs),
  `node scripts/test-system-world-foley.mjs` (city beds and the lantern hum).
- `node scripts/test-system-world-layout.mjs` checks exported tram window sight lines
  from inside and outside, including closed doors at all three LODs.
- `python assets/system-world/build_textures.py --check` and `node scripts/test-system-world-surfaces.mjs`;
  `--preview` writes a lit contact sheet to `reports/system-world-assets/`.
- All exported GLBs must pass the Khronos validator and a real browser rendering check.
- Render previews from the exported GLBs. Repeated generation must preserve GLB hashes.

## Child DOX Index
None.
