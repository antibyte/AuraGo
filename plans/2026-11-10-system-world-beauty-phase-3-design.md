# System World beauty pass — Phase 3: enterable buildings (design)

Date: 2026-11-10 · Status: design by the main session, based on the approved roadmap
(`plans/2026-11-09-system-world-beauty-design.md`, phase 3 row). The implementation plan is written
after phase 2 has landed, because phase 2 changes the same runtime files.

## Goal

Six district buildings get a real ground floor: walk up lit steps, through sliding glass doors, into a
themed lobby. Residents work inside and are visible from the street through real glass. The small
operations beacon stays exterior-only.

| District id | Asset | Lobby |
| --- | --- | --- |
| `agent` | `agent-spire` | Agent core: reception ring around the reactor column |
| `memory` | `memory-archive` | Archive hall: glowing data stacks, reading tables |
| `integrations` | `integration-gate` | Two small gate halls (west: switchboard wall, east: service counter) |
| `missions` | `mission-terminal` | Departure hall: board, four counters, parcel belt |
| `infra` | `compute-foundry` | Machine hall: server racks, coolant pipes, control desk |
| `graph` | `knowledge-atrium` | Open-air garden court under the dome around the knowledge tree |

## What exists today (verified by reading, 2026-11-10)

- **Pavilion interiors.** `interiors` in `sysworld-layout.js` is a fixed list of three 12 × 12 m pavilions
  (floor `surfaces.room` = 0.16). `roomAt`, `groundHeight` and `canWalk` in `sysworld-exploration.js` know
  only these. `sysworld-experience.js` loads their v2 shell within 32 m (`loadShell`), places a v2 door GLB
  with an `open` clip, rebuilds door panel colliders each frame and lets the visitor through once the door
  value is above 0.85 and the room is ready.
- **Inside flag.** `roomAt` → `onEnvironment(!!room)` drives the audio environment, the effects filter,
  foley and hides rain indoors.
- **Districts.** `sysworld-scene.js` places each district model at `(d.x, 0, d.z)` without rotation:
  agent (0, −12), infra (−43, −53), integrations (43, −10), missions (43, 36), memory (−43, −10),
  graph (−43, 36), operations (0, 36). Model fronts face glTF +Z (south).
- **Solid buildings.** `footprints` in `sysworld-exploration.js` and `installCityColliders` in
  `sysworld-colliders.js` block every district from the ground up to 8 m; above that the full v1 bounds
  (agent: `upperSolids`). Every door lies inside its own footprint box.
- **Podiums.** Podium tops are at 0.88 m (`podiums` in `sysworld-exploration.js`). `buildingFloor` returns
  0.88 there, but only hover landings use it: a walking visitor stays at ground height and walks through
  the podium volume.
- **Colliders of kit models.** `traffic.solid(id, box)` takes oriented boxes; several boxes already form
  passages (integration gate). Placed kit assets get no colliders from their manifest: v3
  `navigation.colliders` is not read by the runtime.
- **Residents.** `livingPlaces` (`sysworld-places.js`) are six outdoor places. `addPlace` turns manifest
  `navigation.workpoints` into slots. The wayfinder is street-level only.
- **Tests that pin interiors.** `test-system-world-layout.mjs` (pavilion HUD, telescopes =
  deck + `interiors.length`, "3 galleries") and `test-system-world-expansion.mjs` (`canWalk` at the spire
  footprint, the plaza, the integration passage).

## Decisions

1. **Lobbies are their own registry.** A new pure module `sysworld-lobbies.js` is the single source of
   truth for podiums, stair flights, lobby rooms and doors (table below). `interiors` stays the pavilion
   list, so its tests keep their meaning.
2. **The podium becomes real ground.** `groundHeight` returns 0.88 on a podium and a stepped height on a
   stair flight: five treads of 0.32 m, six equal risers of 0.88 / 6 m, running 1.6 m out from the podium
   edge. This is exactly how kit v3 authors the steps (`facade_assets3.py` `STEPS`, `TREAD`, `PODIUM`).
   Residents climb them with the phase 1 footing (damped body height, feet on the treads).
3. **Nobody takes a step higher than 0.35 m.** `canWalk` (visitor) and the wayfinder's link test
   (residents) refuse a move whose ground height changes by more than 0.35 m between samples at most
   0.25 m apart. Podium edges are thereby closed everywhere except at the stairs, without extra walls.
   Kerbs (0.14), platforms (0.30) and steps (0.147) stay passable.
4. **Walls with door gaps replace the footprint boxes** of the six enterable buildings: 0.6 m thick boxes
   along the lobby perimeter, split at each door, plus a lintel box above each door and a ceiling box, all
   generated from the registry. Fixtures inside come from the interior asset's `navigation.colliders`.
   A door gap is closed by a panel collider until the interior is ready and the door is open.
5. **The interior is a separate kit v3 asset** per building (`lobby-<district id>`, category `interior`),
   authored in the building's own frame and placed at the district origin. It holds floor finish, ceiling,
   inner wall faces, furniture, the see-through glazing (`city.pane`) and the sliding door leaves.
   It loads when the camera is within 45 m of a door and is not part of the first display.
6. **Stand-in swap.** Each exterior building carries its ground-floor window panes and closed door leaves
   as the separate part `lobby` (already authored). While the interior is shown, the renderer hides that
   part; beyond 70 m it shows the stand-in again and hides the interior. On the low tier interiors never
   load: the stand-in stays and the doors stay shut.
7. **Glass.** `city.pane` is one shared transparent material (dark teal, opacity about 0.2, roughness 0.05,
   strong environment reflection, no depth write, double-sided). `sysworld-surfaces.js` and
   `sysworld-windows.js` leave it alone.
8. **Doors.** Each door has two leaves as separate nodes `door_<n>_l` and `door_<n>_r` (no dots: three.js strips them from node names), authored closed.
   They slide apart along the facade by their own width when a visitor or resident is within 3.2 m and
   the interior is ready (0.6 s ease, reduced motion: instant).
9. **Inside flag.** `lobbyAt(x, z)` feeds the same chain as `roomAt`. The knowledge atrium is flagged
   `open` (no roof): it counts as a place, but not as indoors for rain and audio.
10. **Residents work inside.** Each interior declares `navigation.workpoints`. Each lobby is added as a
    living place. The wayfinder gains three kinds of nodes per door: stair foot, threshold, and one inside
    node per workpoint; doors open for residents as for the visitor.
11. **Hover pack.** Lobbies, stairs and podiums are not landing spots. Lintel and ceiling boxes keep
    flights out of the rooms.

## Registry (world coordinates, metres)

All lobby floors are at 0.88. Fronts face south (+Z). Stairs run from the podium front edge towards +Z.

| Id | Podium (x0..x1, z0..z1) | Lobby outer rect (x0..x1, z0..z1) | Doors: x, width, height (front z) | Stairs: x, width | Ceiling |
| --- | --- | --- | --- | --- | --- |
| agent | −11.5..11.5, −23.5..−0.5 | −7.5..7.5, −19.5..−4.5 | 0, 4.0, 3.5 (z −4.5) | 0, 7.5 | 5.0 |
| memory | −56..−30, −21..1 | −54..−32, −20..−2 | −48 and −38, 2.8, 3.5 (z −2) | −48 and −38, 4.2 | 6.1 |
| integrations west | 30..56, −18..−2 | 32..38, −14.5..−5.5 | 35, 2.0, 2.92 (z −5.5) | 35, 3.4 | 4.3 |
| integrations east | (same podium) | 48..54, −14.5..−5.5 | 51, 2.0, 2.92 (z −5.5) | 51, 3.4 | 4.3 |
| missions | 28..58, 25.5..46.5 | 30..56, 27.5..42.5 | 34, 40, 46, 52, 4.0, 3.5 (z 42.5) | 43, 23 | 6.1 |
| infra | −55.5..−30.5, −64.5..−41.5 | −53..−33, −62..−50 | −46.75 and −39.25, 2.4, 3.12 (z −50) | −46.75 and −39.25, 3.0 | 4.5 |
| graph | −55..−31, 24..48 | circle, centre (−43, 36), radius 9.45 (12 sides) | −43, 3.2, 3.1 (z 45.13) | −43, 4.4 | open |

- The clear room is the outer rect shrunk by 0.6 m on every side (corner piers take 1.3 × 1.3 m).
- Infra: three cooling towers stand on the podium in front of the facade at (−50.5, −48), (−43, −48) and
  (−35.5, −48) with a 2.7 m plinth radius; the doors lie between them.
- Graph: the trunk of the knowledge tree (radius 1.5) stands in the centre.
- Operations beacon: podium −4.5..4.5, 31.5..40.5 with a 2.2 m flight at x 0; not enterable.
- The plan's first task checks every number in this table against the packed GLBs (bounds of the `lobby`
  part, door frames, stairs) and fails loudly on a mismatch, as the street contract test does.

## Blender work (main session)

Done on 2026-11-10 in the still unregistered sources (`facade_assets3.py`, `district_assets3.py`):
solid lit stairs with the profile above, the `lobby` stand-in part, 5.4 m ground floor and 2.4 m doors for
the foundry with the towers 7.5 m apart, a 4.7 m rotunda wall and a 3.1 m portal for the atrium. Every
door is at least 2.9 m high, so the visitor's eye (2.4 m above the floor) passes below the lintel.

To author for this phase (`interior_assets3.py`):

- Six interior assets as listed under "Goal", three LODs each (LOD 2 is the bare shell), baked occlusion,
  target 6–12 k triangles at LOD 0 and about 1.5 MiB packed in total.
- `city.pane` glazing at the positions of the stand-in panes; door leaves with pivots; a lit ceiling;
  floor inlays; furniture with colliders; workpoints for residents; screens and light lines in the
  emissive kit materials.
- New palette key `pane` in `kit3.py` (this changes a hashed source: rebuild, repack and commit the v3
  manifest together).

## Runtime work packages (for the implementation plan)

1. `sysworld-lobbies.js` with tests: registry, `lobbyAt`, podium and stair height, wall, lintel and
   ceiling boxes, door panels, registry-versus-GLB contract test.
2. Ground: `groundHeight` consults the registry; the 0.35 m step rule in `canWalk` and in the wayfinder.
3. Colliders: walls with gaps instead of footprint boxes for the six buildings; fixtures from the interior
   manifests; door panels; hover rules.
4. Interiors: loader on approach, stand-in swap, `city.pane` material, tier rules, dispose.
5. Doors: sliding leaves for visitor and residents, reduced motion.
6. Inside flag: audio, effects, foley, rain (atrium open).
7. Residents: lobby places, workpoints, wayfinder nodes, living test thresholds.
8. HUD prompt at the doors, survey scenes (each lobby from the street by day and night, and from inside),
   browser test, markers, bundle, budgets, docs.

## Testing and acceptance

- New: registry contract test against the packed GLBs; step-rule unit tests (kerb, platform, stairs pass;
  podium edge blocks); a walk test that enters and leaves every lobby through every door; wayfinder routes
  from a pavement node to an inside workpoint; hover cannot enter a lobby.
- Existing suites stay green; `interiors`-pinned assertions are untouched.
- Survey: for each lobby one street view by day and by night with residents visible through the glass, one
  view from inside towards the street, and one sequence of a resident climbing the stairs.
- Budgets: report model directory, first display (must not grow: interiors load on approach) and whole app.

## Risks

1. **Transparent glass sorting.** Several glass layers (lobby front, door leaves, interior screens) can
   sort wrongly. One shared material without depth write, and no transparent surfaces inside, keeps it simple.
2. **Stairs and footing.** The feet solve against a stepped height; a foot can land on a riser edge. If it
   looks wrong, the walker's foot target snaps to the tread centre.
3. **Path width between the foundry's cooling towers** is 2.1 m at the narrowest point; residents with
   parcels may queue there.
4. **Pop when the interior replaces the stand-in.** The swap happens at 45 m or more; if it is visible,
   cross-fade the glass.
5. **Wayfinder cost.** More nodes and the step-rule sampling make the one-time graph build slower.
