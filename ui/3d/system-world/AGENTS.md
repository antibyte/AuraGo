# System World runtime models

## Purpose
Versioned compact GLB assets for the integrated System World data metropolis.

## Ownership
The `v1/`, `v2/` and `v3/` kits plus `white-robot.glb` and its provenance `white-robot.json` ship.
`v3/` is written only by `scripts/build-system-world-v3.mjs`.
Authoring belongs to `assets/system-world/`; see its README and DOX contract.

City life (phase 4) runtime ownership, all in `ui/js/desktop/apps/`:

| Concern | Owner |
| --- | --- |
| Wind value (direction, strength, gusts) | `sysworld-atmosphere.js` (`windAt`, `wind()`) |
| Leaf sway (`city.leaf`, instanced draws above the low tier) | `sysworld-surfaces.js` (`SW_SWAY`, `setWind`) |
| Banner cloth, near-gate, lawn and paving slabs, wiring of every new module | `sysworld-scene.js` |
| Instanced pivoted parts | `sysworld-parts.js` |
| Boats and buoys | `sysworld-harbour.js` |
| Gulls | `sysworld-gulls.js` |
| Steam and fireflies | `sysworld-particles.js` |
| Kit workplaces (charging plaza, harbour cafe) | `sysworld-experience.js` (places), `sysworld-society.js` (no demonstrate prompt) |

- Kit placements are solids. `sysworld-scene.js` passes `kitPlacements` to `installCityColliders(…, kit)`,
  which reads each placed design's `navigation.colliders`, turned with the placement.
- The lawn and paving materials are scene-owned: they live in the scene's `matSet`, never in the shared
  materials map.

## Local Contracts
- Preserve the 3 v1 designs (`service-drone`, `data-tram`, `server-rack`), three LODs and MIT provenance. The complete
  model directory, including v2 and the original robot, has a 40 MiB ceiling.
- No Blender source, previews, dependency caches, loose images or remote assets here. The
  only images are the generated runtime surface maps in `textures/v1/` (MIT, manifest-pinned
  sizes/hashes, written by `assets/system-world/build_textures.py`); they count toward the
  40 MiB model directory and must stay below 1.5 MiB.
- The white robot is an optimized derivative of the existing `ui/3d/robot.glb`.
  It preserves that artwork's provenance and three embedded 512 px PBR textures;
  do not apply the original kit's MIT-authorship claim to the derivative.
  At most 25,000 triangles, below 2 MiB, explicit tangents, no runtime Draco.
  Five residents share its geometry/materials/textures. Total kit plus robot
  remains below 40 MiB; hashes and source hashes must pass `check_assets.py`.
- Manifest hashes, sizes, bounds and triangle counts must match actual GLBs.
- `v2/` contains 42 Blender-authored designs and 126 GLBs with animation and
  navigation metadata. Verify with `test-system-world-expansion.mjs`,
  `test-system-world-layout.mjs` and `test-system-world-hover.mjs`; upper floors must
  match actual rendered lift landings. The complete app (models, renderer, scripts, locales) has an 80 MiB
  budget and first display stays below 20 MiB. New installations initially use
  LOD2; nearby detail follows camera distance and the quality cap. Their collision
  envelopes and work/interaction markers are always independent of visual LOD.
  Robot `motion_bounds` must contain the actual GLB geometry throughout every clip.
- Rigged v2 assets (`robot-*`, `hover-pack`) keep one ordered material list per part and
  stable mesh names at every LOD, so the runtime swaps LODs by mesh name. The
  `hover-pack` `pack` node is hidden while the visitor flies; keep it one removable node.
- `spot.glb`, `spot.json` and `spot-LICENSE.txt` are the compact companion set.
  It is a third-party BSD-3-Clause derivative, not part of the MIT kits: the
  manifest pins the upstream URL, commit and source SHA-256, the GLB uses
  `EXT_meshopt_compression` and `KHR_mesh_quantization`, keeps all 13 named
  joints and both materials, and has no textures or external URIs. The renderer
  loads it through `GLTFLoader.setMeshoptDecoder`; a failed load leaves the city
  fully usable. The verbatim licence travels here and in `licences/texts/`.
  Authoring and reproduction belong to `assets/system-world/spot-source.json`
  and `scripts/build-system-world-spot.mjs`.
- `v3/` holds the 46 kit v3 designs (street modules, facades, district buildings, data towers, street
  furniture, the six lobby interiors and the phase 4 city life; 138 GLBs, `total_model_bytes` 4,391,656 in the
  manifest): meshopt-compressed GLBs (`EXT_meshopt_compression`
  required), texture-free, `city.*` materials and `COLOR_0` on every primitive. Load them with
  `GLTFLoader.setMeshoptDecoder(MeshoptDecoder)` and await `MeshoptDecoder.ready` first, as
  `sysworld-scene.js` does.
- v3 wins per id. `kitFor(id, catalog3)` in `sysworld-scene.js` loads the v3 design whenever the v3
  manifest ships the id. The twelve superseded ids (`supersededV1`) therefore load from v3, and their
  v1 files are gone. Colliders read `kitCatalog(v1, v3)` (`sysworld-colliders.js`).
- `city.window` (window panes) is owned by `ui/js/desktop/apps/sysworld-windows.js`: one shared material
  for every pane, adopted in `sysworld-scene.js` `model()` before material sharing. It never enters the
  shared-material map, and `sysworld-surfaces.js` never patches it. Vertex-coloured `city.ivory` and
  `city.warm` parts are light fixtures.
- `city.pane` (interior glazing of the lobbies) is owned by `ui/js/desktop/apps/sysworld-lobby-render.js`
  (`PANE_MATERIAL`): one shared transparent material, adopted in `sysworld-scene.js` `model()` next to the
  window adoption. It never enters the shared-material map, `sysworld-surfaces.js` and `sysworld-windows.js`
  never touch it, and only `lobbyRender.dispose()` releases it.
- Lobby interiors are loaded by `createLobbyRender` in `sysworld-lobby-render.js`. An interior loads within
  45 m (3D, at eye height) of one of its doors and is released beyond 70 m. Medium loads LOD 1, high and ultra
  LOD 0, and low loads none. While an interior shows, its building's `lobby` node is hidden. Interiors are not
  counted in the first display.
- `installCityColliders` (`sysworld-colliders.js`) adds, per enterable district, `lobbySolids(district)` (walls
  with door gaps, corner piers, lintels, ceiling slab, atrium lid, foundry towers and pipes, spire struts) and
  each interior's `navigation.colliders` from the catalog (installed with the city, not on load). Door leaves have no colliders; the visitor is
  held at the leaf line by `lobbyPassage` in `canWalk`.
- Street-lamp light cones are off by day. `lampLevel` in `sysworld-atmosphere.js` fades them in towards dusk.
- Materials with vertex colours are keyed by `sharedMaterialKey` with a `|vertex-colours`
  suffix, so v3 materials are not shared with same-named v1/v2 materials that lack vertex colours.
- v3 bytes count toward the 40 MiB model directory ceiling, which the v3 build and test enforce.
- Load and instance models through the isolated renderer; do not mutate shared legacy Three.js.
- `audio/` holds the six ambience loops of the sound layer: `water`, `leaves`, `birds`, `gulls`, `city` and
  `rain`, and the two one-shot recordings `tram` (one tram pass) and `steps` (single footfalls), each a mono
  Ogg Vorbis file (`<id>.ogg`), plus `manifest.json` (schema 1; per file its bytes,
  SHA-256, length, loudness, true peak, title, author and licence `CC0-1.0`; no URLs). Generated by
  `assets/system-world/build_audio.py` from `assets/system-world/audio-sources.json`; do not hand-edit.
  The recordings are 1,537,920 bytes of the 1,572,864-byte budget (1.5 MiB) that `build_audio.py --check`
  enforces. They count toward the 40 MiB model directory, which `scripts/test-system-world-v3.mjs` sizes
  over the whole folder. The app serves the files through the versioned `assetURL` (`sysworld.js`).
  `sysworld-sound-recordings.js` fetches `manifest.json` and the recordings on the first frame while the
  sound is on, loads only `water`, `city` and `rain` on the low tier (no one-shot recording), and leaves a
  file that fails to load or decode silent without retrying it. `tram` and `steps` are decoded buffers
  only: `sysworld-sound-movers.js` plays the tram pass once under a tram voice that comes close, and
  `sysworld-foley.js` plays one footfall slice under each footfall on a hard surface.

## Verification
`python assets/system-world/check_assets.py`, glTF validation and rendered asset review.
`node scripts/build-system-world-v3.mjs --check`, `node scripts/test-system-world-v3.mjs`,
`node scripts/test-system-world-street-models.mjs` and `node scripts/test-system-world-ground.mjs`.
`node scripts/test-system-world-windows.mjs` and `node scripts/test-system-world-lamps.mjs`.
Lobbies: `node scripts/test-system-world-lobbies.mjs` (registry), `node scripts/test-system-world-lobby-contract.mjs`
(registry against the packed GLBs), `node scripts/test-system-world-lobby-walk.mjs` (every door entered and left)
and `node scripts/test-system-world-lobby-render.mjs` (interior loading against the `lobby-agent` GLBs).
City life: `node scripts/test-system-world-furnishing.mjs`, `-wind`, `-harbour`, `-gulls` and `-particles`
(the runtime modules listed under Ownership).

## Child DOX Index
None.
