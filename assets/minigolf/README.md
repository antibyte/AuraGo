# AuraGo Mini Golf courses

The 18 original low-poly courses, the three world prop kits and the Blender
generator in this directory were created for AuraGo and are licensed under the
MIT License in this directory. They are authored in Blender 5.2.2 using metres.
Runtime GLBs and manifest coordinates use glTF's Y-up coordinate system.

The player-facing description of the worlds, the look and the budgets is in
[`documentation/minigolf.md`](../../documentation/minigolf.md).

## Outputs

The generator writes to `ui/3d/minigolf/`:

- `course-01.glb` to `course-18.glb`, one self-contained GLB per course,
- `props-pirate.glb`, `props-temple.glb` and `props-moon.glb`, one prop kit per world,
- `previews/<look>.webp`, one menu preview per world (separate build),
- `manifest.json`, the compact course, kit and world index.

The Blender scenes are not versioned. `--save-blend` writes
`assets/minigolf/build/aura-golf.blend`, which is git-ignored. The collision
meshes are nodes of the course GLB itself (`floor-*` and `rail-*` with the extra
`aurago_collider`): the runtime reads, welds and fingerprints them, and the
manifest only carries the digest, so physics follows Blender's exported topology
and coordinates exactly. Moving obstacles are named top-level nodes
(`obstacle-<id>`) and remain separate from the fixed colliders. The runtime
manifest (version 2) is compact JSON; pretty-print it for inspection with
`python -m json.tool ui/3d/minigolf/manifest.json`.

Sound effects and music are not part of this Blender pack. Their sources,
provenance and rebuild steps are in [`audio/README.md`](audio/README.md).

## Module map

`generate.py` is the CLI. All logic is in the `kit/` package:

| Module | Role |
|---|---|
| `dress.py`, `dress_island.py`, `island.py`, `trim.py` | Dressing a P3 course: the `Dresser`, the island strata and mesh, the shore strip, the rail trims (one node from the rail runs), the bake. |
| `dress_parts.py` | The world style of the functional parts: ball and putter templates, flag, hole digits, tee ring, cup rim. |
| `placement.py`, `placement_plan.py`, `occluders.py` | Deterministic prop placement (hero and fixed props of the course data, corridors of every route, random dressing scaled with the island) and its rule check; the kit objects used as bake occluders. |
| `track.py`, `pieces.py`, `pads.py`, `layout.py`, `rails.py` | The P3 track kit core (plain Python): sections and pads with shared seam points, lane pieces and raster pads, course data to `Track`, wall runs. |
| `cup_hole.py`, `cup_parts.py` | The exact cup: hole polygon and zipper floor around it (plain Python), the collision cup `floor-cup`, cup rim, tee ring and flag. |
| `courses.py`, `course_entry.py`, `course_data_pirate.py`, `course_data_temple.py`, `course_data_moon.py` | The course pipeline: `build_scene` builds and dresses a course, `build_course` exports and describes it (the manifest v2 entry: bounds, water, routes); the course data of each world (plain literals). |
| `footprint.py`, `terrain.py`, `bed_field.py` | Island outline fitted to the layout, island height (bed under grounded floors, cup hollow, ponds, trenches), the floor triangle hash behind it. |
| `structures.py`, `support_plan.py`, `support_parts.py`, `bridge_parts.py`, `loop_shell.py`, `tunnel_plan.py`, `tunnel_parts.py`, `portal_pirate.py`, `portal_temple.py`, `pond_parts.py`, `trench_parts.py`, `detail.py`, `part_utils.py` | The structures of a P3 course: supports under elevated floors and loops, bridge railings and beams, the outer shell of a loop (its floor has no outer side), tunnel roofs with the skull-rock and stone-head portals, pond water surfaces, trench and chasm walls, per-course detail overrides (`structure_selftest.py` checks the rules). |
| `mechanic_floor.py`, `mechanic_data.py`, `obstacle_data.py`, `cannon_flight.py` | Mechanics and obstacles of a course as data (plain Python): placement by position or anchor, floor height, manifest entries (runtime axes) with the collision parts of every obstacle, the cannon barrel and launch velocity with its flight checks, and the Python mirror of `validateMechanic` / `validateObstacle`. |
| `mech_util.py`, `mechanic_parts.py`, `mechanic_portals.py`, `mechanic_cannon.py`, `obstacle_parts.py`, `obstacle_gates.py` | The visible parts per world: bumpers (barrel, stone mushroom, neon pylon), the boost plate, teleporters (hatch and barrel, idol arch, transporter pad with a spinning ring), the pirate cannon, the moving obstacle nodes `obstacle-<id>` drawn from their collision parts (anchor weight, log, boulder, radar arm, gate panels) and their gallows and door frames. |
| `pieces_moon.py`, `pads_funnel.py`, `layout_fork.py`, `layout_ponds.py` | Moon half-pipe and loop, the polar funnel pad, forks (split pad, branches built by the layout, join pad) and the free ponds of the course data (`ponds`). Importing the layout registers them. |
| `build_track.py`, `floor_paint.py`, `basic_parts.py` | Blender floor and collision rail nodes of a `Track`, floor colours, shared mesh helpers (coordinate swizzles, materials, hole number, templates). |
| `track_checks.py`, `track_selftest.py` | The two probe courses with their pure-Python checks, and the headless Blender selftest that exports and reads them back. |
| `export.py` | Export, canonical node names, merging of scenery per material, GLB reading, `glb_collision` (collision meshes and digest of a track-kit GLB). |
| `kits.py` | Builds each world's prop kit and its manifest entry. |
| `props_pirate.py`, `props_temple.py`, `props_moon.py` | The props of each world (`PROPS`, `PROP_META`). Helpers: `props_pirate_*.py`, `ship_pirate.py`, `props_temple_*.py`, `head_temple*.py`, `props_moon_*.py`. |
| `materials.py`, `palette.py`, `colors.py`, `shapes.py`, `prop.py`, `bake.py` | Shared materials, palettes, vertex colours, shape builders, prop composition and the ambient-occlusion bake. |
| `previews.py` | Menu preview renders (Cycles, `--only previews`): the dressed showcase course of each world (`COURSES`: `pirate-04`, `temple-09`, `moon-18`), framed on its playable area. |
| `probe.py`, `scene_<world>.py`, `stage.py`, `lane.py`, `compose.py` | Style probes and their stage helpers (not used by the build). |

The contracts for these modules are in [`kit/AGENTS.md`](kit/AGENTS.md).

## Commands

Run from the repository root with Blender 5.2.2 (the `--` separates Blender's
arguments from the generator's):

```powershell
& 'D:\Blender 5.2\blender.exe' --background --factory-startup --python assets/minigolf/generate.py -- --only kits,courses
& 'D:\Blender 5.2\blender.exe' --background --factory-startup --python assets/minigolf/generate.py -- --only kits
& 'D:\Blender 5.2\blender.exe' --background --factory-startup --python assets/minigolf/generate.py -- --only courses --course 7
& 'D:\Blender 5.2\blender.exe' --background --factory-startup --python assets/minigolf/generate.py -- --only courses --course 7 --save-blend
& 'D:\Blender 5.2\blender.exe' --background --factory-startup --python assets/minigolf/generate.py -- --only previews
& 'D:\Blender 5.2\blender.exe' --background --factory-startup --python assets/minigolf/generate.py -- --only all
node scripts/test-minigolf-assets.mjs
node scripts/test-minigolf-courses.mjs
```

The generator builds the courses of `kit/course_data_<world>.py` (manifest version 2, collision read from the GLB)
and writes to `ui/3d/minigolf/` unless `--out DIR` names another folder. A scratch build goes to a folder of its
own (`assets/minigolf/build/` is ignored by git) and is checked with the same tests through the environment:

```powershell
& 'D:\Blender 5.2\blender.exe' --background --factory-startup --python assets/minigolf/generate.py -- --out assets/minigolf/build/scratch --only kits,courses
& 'D:\Blender 5.2\blender.exe' --background --factory-startup --python assets/minigolf/generate.py -- --out assets/minigolf/build/scratch --course 1 --save-blend
node scripts/minigolf-route-search.mjs --dir assets/minigolf/build/scratch --course pirate-01
$env:AURAGO_MINIGOLF_DIR = 'assets/minigolf/build/scratch'; $env:AURAGO_MINIGOLF_PARTIAL = '1'   # a subset of the 18 courses is fine
node scripts/test-minigolf-assets.mjs; node scripts/test-minigolf-courses.mjs
```

- `--set p3` is accepted for old command lines; it is the only course set.
- `--out DIR` sets the folder for the GLBs, the manifest and the previews
  (default `ui/3d/minigolf`).
- `--only` takes `kits`, `courses`, `previews` or `all` (comma-separated). The
  default is `kits,courses`; with `--course` it is `courses`.
- `--only kits` rebuilds the three prop kits and keeps the course entries; a
  courses build keeps the kit entries.
- `--course N` (1–18) rebuilds one course in a fresh process and patches its
  manifest entry. A full courses build must exist first.
- `--save-blend` is allowed only together with `--course N`. A full run deletes
  each course after its export, so the scene would not survive.
- `--only previews` is separate because Cycles renders are not bit-reproducible.
  It builds the showcase course of each world; `--worlds pirate,temple` limits it.

A full courses build takes about two minutes on 16 cores; the three kits take
about 15 seconds. On Windows a transient file lock can abort one export. Run the
command again.

## Determinism

Randomness comes only from seeded `random.Random` and Blender's noise. Bakes run
on the CPU with a fixed seed and sample count, and colours are rounded to 8 bits.
Each course is deleted after its export, so no node or mesh name carries a Blender
counter, and `--course N` in a fresh process writes the same file as the full run.

To check it, run `--only kits` twice and compare the `sha256` values of the kits in
`ui/3d/minigolf/manifest.json`. For the courses, run the full build twice and
compare the SHA-256 of every course GLB and of the manifest between the two runs.

## Sizes of the last full build (the P3 courses in `ui/3d/minigolf`)

| Item | Size |
|---|---|
| 18 course GLBs | 15,206,816 bytes (largest moon-18 1,203,448; 494,664 triangles in all, most temple-12 with 33,877) |
| `manifest.json` | 132,124 bytes (version 2, no collider arrays) |
| Three prop kits | 5,029,080 bytes (pirate 1,771,100, temple 1,768,908, moon 1,489,072) |
| Three previews | 89,892 bytes (pirate-04 23,932, temple-09 32,320, moon-18 33,640) |
| Whole `ui/3d/minigolf/` package, including audio and previews | 23,318,031 bytes |

The asset test enforces the budgets (course at most 1.25 MiB, 36,000 triangles and
32 mesh nodes; manifest at most 256 KB; kit at most 3 MiB and 60,000 triangles;
prop at most 15,000 triangles, hero 20,000; preview at most 80 KB; package at most
32 MiB). Regenerated sizes can vary with edits; the test is the authoritative
budget check. Two full `--only kits,courses` runs of this build gave identical SHA-256
for all 22 files (18 courses, three kits, manifest).

## What the asset test checks

`node scripts/test-minigolf-assets.mjs` checks (manifest version 2; `AURAGO_MINIGOLF_DIR`
names another asset folder, `AURAGO_MINIGOLF_PARTIAL=1` accepts a subset of the 18 courses):

- the budgets above, the GLB structure, embedded resources and the glTF validator;
- the manifest fields of every course, 18 courses (6 pirate, 6 temple, 6 moon) with the ids
  `pirate-01` to `moon-18`, the gravity of the world, no `colliders` and no `look`;
- the collision: visible `floor-*` and hidden `rail-*` nodes with `aurago_collider`, the
  digest of the welded GLB collision equal to the manifest, an open cup (no floor
  triangle over the hole except `floor-cup`) and a tee one ball radius above the floor;
- no numbered node or mesh names (`scenery-matte.001`);
- `path` (starts within 0.3 m of the tee, ends within 1 m of the cup) and `corridors` (the
  first is the main route, branches start and end on it), `length`, the `worlds` list;
- every placement: a known kit prop, a distance to the island rim (`island`), a clear corridor
  around every route (except `span` props) and a hero at scale 0.9 or more; every kit prop
  placed in at least one course (full set only);
- mechanics and obstacles: accepted by `validateMechanic` / `validateObstacle`, unique ids,
  inside `bounds`, each `obstacle-<id>` node in the pose of `obstaclePose(spec, 0)`, and a
  `cycle` that is a whole number of periods for every obstacle;
- each kit: top-level prop nodes, `COLOR_0` on every primitive, the prop and kit
  budgets, and the tags in the node extras;
- the previews: WebP signature and size;
- the ball and putter templates, the flag and an upward-facing tee ring.

`node scripts/test-minigolf-courses.mjs` plays every course against the Rapier physics with the
collision from the GLB: a recorded route per course (at most par + 1 shots, every shot but the
last ends resting, the last in the cup), obstacles that move with physics time and a gentle tee
shot over grass. `AURAGO_MINIGOLF_SEARCH=1` searches new routes and prints them,
`AURAGO_MINIGOLF_COURSE=<id>` limits the run.

The full runtime check is `npm run test:minigolf`, described in
[`documentation/minigolf.md`](../../documentation/minigolf.md).
