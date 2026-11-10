# System World beauty pass — Phase 4: city life (design)

Date: 2026-11-10 · Status: design by the main session, based on the approved roadmap
(`plans/2026-11-09-system-world-beauty-design.md`, phase 4 row). The implementation plan is written after
phase 3, because placements must respect the lobby stairs and doors.

## Goal

The city looks inhabited and alive: trees and beds that move in the wind, parks and a market on the empty
plots, furniture along the streets, neon on the facades, boats and buoys on the water, gulls over the
harbour, steam from vents and fireflies in the green at night.

## What exists today (verified by reading, 2026-11-10)

- **Grid** (`sysworld-layout.js`, `sysworld-ground.js`). Streets along Z at x −67, −18, 18, 67 and along X
  at z −77, −32, 13, 59; half width 6 m; kerb line 3.30; pedestrian nodes at 4.75. Island x −85..85,
  z −94..80, quay strip z 74..80. 25 plot slabs: x bands −85..−73, −61..−24, −12..12, 24..61, 73..85 by
  z bands −94..−83, −71..−38, −26..7, 19..53, 65..74. 64 zebras, 192 pedestrian nodes.
- **Empty plots.** The whole northern row z −94..−83 (four of its five plots), the eastern plot x 73..85,
  z −71..−38, the south-east corner x 73..85, z 65..74 and the east end of the quay (x 76..85).
- **Already placed** (`sysworld-scene.js` `placements`, `sysworld-experience.js`, `sysworld-places.js`):
  16 lamps and 16 planters at x ±31/±55 and ±35/±51 on the north pavements (z = street − 4.5), three data
  towers, a skybridge, two server racks, seven stations, five hover docks, a drone pad at (78, 30), six
  living places (fountain at (−28, 47)), landmarks, v2 gardens, arcades, a bridge, quay pieces and lanterns,
  chargers and coolers, the companion loop on the plot east of x = 18.
- **Drawing.** Static kit models are instanced per mesh node from the module-level arrays `placements`
  (v1) and `kitPlacements` (v3); more copies of a design add matrices, not draw calls. There is no
  per-instance colour or animation. A tier change rebuilds all batches.
- **Colliders.** `installCityColliders` reads v1 bounds for `placements` only. v3 `navigation.colliders`
  are not read; `kitPlacements` get no solids. The robot navigator knows only trams, racks, lamps and
  planters as obstacles. The hover test demands 3 m between a dock and any solid placement and 2 m to
  every pedestrian node.
- **Water and air.** The sea is a camera-following plane at y −3.2 with three sine waves, fresnel and
  foam around the island (`sysworld-atmosphere.js`). The island body ends at −3.0. Only the tidal turbine
  at (0, 96) stands in the water. Free water: south z 80..140, east x 85..145, west x −145..−85, north
  z −130..−94. Six drones fly at 27–74 m. There is no wind value, no steam, no fireflies, no birds.
- **Loop and tiers.** Fixed 60 Hz `simulate`, per-frame `update`; reduced motion freezes simulation time.
  Tiers low / medium / high / ultra; `applyTier` forwards to surfaces, atmosphere, experience, weather,
  drones and flow. Subsystems are wired by hand into `createCity` (create, update, tier, reduced, dispose,
  stats).
- **Budgets.** Model directory 11.9 of 40 MiB, first display 6.1 of 20 MiB, whole app 13.1 of 80 MiB after
  phase 1. No test pins draw calls or triangles.

## Decisions

1. **One pure placement module** (`sysworld-furnishing.js`): all new placements come from rules over the
   grid, not from hand-typed lists, and are returned as `{asset, x, z, y, angle}`. A unit test checks the
   rules on the result.
2. **Furnishing strip.** Large pieces (trees, benches, beds, kiosks, cafe sets, pylons, cabinets) stand on
   the outer 0.4–2.4 m of the plots, never on the pavement itself, so walking routes stay free. Only
   bollards, litter bins and crossing signs stand on the pavement, in the 0.5 m behind the kerb.
3. **Clearances** (tested): 1.2 m to every pedestrian node and link, 3 m to hover docks, 2.5 m to rails
   and platforms, 2 m to stairs and doors of the phase 3 registry, 1 m between colliders, nothing on a
   carriageway, zebra, dropped kerb or tactile strip.
4. **Kit v3 colliders become real.** `installCityColliders` also takes `kitPlacements` and reads each
   design's `navigation.colliders` (rotated with the placement). The robot navigator derives its circles
   from the same boxes. The hover, robot and living tests run against these solids.
5. **The empty plots get a purpose.**
   - North-west strip (x −61..−24, z −94..−83): a linear park with two tree rows, beds and benches.
   - North-east strip (x 24..61, z −94..−83): a market with three kiosks, cafe sets and banner masts.
   - North corners (x ±79, z −88.5): small groves.
   - East plot (x 73..85, z −71..−38): a charging plaza with charge posts (workpoints for residents),
     an info pylon and trees.
   - South-east corner and quay east end: a harbour cafe terrace with beds; mooring posts along the quay.
   - District plots: trees at the corners, beds beside the stairs, benches facing the entrances, a street
     cabinet per block, an info pylon at each station.
   - Streets: a tree every 12 m on the plot edge where no lamp, planter, station or dock stands; a litter
     bin per block; crossing signs at every zebra; bollards at the dropped kerbs.
6. **Wind.** The atmosphere owns one wind value (direction, strength, slow gusts) and a uniform. Foliage
   sways in the vertex shader of the leaf material in proportion to the height above the model origin, with
   a phase from the instance position. Banners swing their `banner` node. Off on the low tier and with
   reduced motion. The same wind later drives the sound of leaves (phase 5).
7. **Harbour traffic** (`sysworld-harbour.js`): a launch patrols a loop off the quay, a barge crawls past
   100 m out, three skiffs sail on the east side with their `rig` swinging, six buoys mark the channel with
   a `lamp` that blinks at night. Every hull rides the sea's own wave function (height and tilt). Boats
   keep 12 m from the island, the turbine and each other.
8. **Gulls.** A small flock (12 on high, 6 on medium, none on low) circles over the harbour and follows the
   barge; body and wings are three instanced meshes with matrices computed on the CPU.
9. **Neon.** Blade signs on tower and kiosk corners, wide and round signs above lobby canopies and
   pavilions, at mount points taken from the facade registry. They use the kit's light materials, so they
   follow day and night like the light fixtures.
10. **Steam and fireflies.** Steam: one instanced soft-quad cloud per vent (cooling towers, roof units,
    a few manholes), animated in the shader, at most 60 quads. Fireflies: twinkling points over the green
    areas on clear nights, at most 200. Both off on low and with reduced motion.
11. **Tiers.** Low: no wind, gulls, steam or fireflies; boats stand still; furniture at LOD 2. Medium:
    half the gulls, wind on. High and ultra: everything.

## Blender work (main session)

Done (unregistered): `furniture_assets3.py` (lamp, planter, two trees, bench, bollard, litter bin, crossing
sign, kiosk) and `life_assets3.py` (launch, barge, skiff, buoy, mooring post, flower bed, cafe set, charge
post, info pylon, street cabinet, three neon signs, banner mast, gull). To do in this phase: register
`life_assets3`, add a third tree and a low hedge if the parks look uniform, and check every model in the
engine at dusk.

## Work packages (for the implementation plan)

1. `sysworld-furnishing.js` with the rule test; survey scenes for each new area.
2. Colliders for kit v3 placements; navigator obstacles; hover, robot and living tests on v3 solids.
3. Wind value, foliage sway, banners.
4. `sysworld-harbour.js`: boats, buoys, wave ride.
5. Gulls.
6. Neon mount points and placement.
7. Steam and fireflies.
8. Charging plaza and cafe as living places (workpoints).
9. Markers, bundle, browser test, budgets, docs.

## Testing and acceptance

- Rule test for all clearances; collider test with rotated boxes; boats never enter the keep-out zones over
  a ten-minute simulation; gull and particle counts per tier; no per-frame allocation in the new updates.
- Existing suites green; draw calls and frame time on the high tier reported before and after.
- Survey: park, market, charging plaza, harbour terrace, a tree-lined street by day and night, the harbour
  with boats at dusk, fireflies at night.

## Risks

1. Hundreds of new solids slow the wayfinder's one-time link test: use a spatial grid.
2. Foliage sway on shared materials must not move trunks or planters: the leaf material alone is patched,
   and amplitude is zero at the model origin.
3. The island stands 3.2 m above the water, so boats are seen from above: hull decks must read well from
   that angle (they were authored with full decks and deck gear).
4. Draw calls: every new design adds one batch per material; the target is at most 60 additional calls.
