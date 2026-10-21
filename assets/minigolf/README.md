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
`assets/minigolf/build/aura-golf.blend`, which is git-ignored. Fixed collider
vertices and triangle indices are read back from each exported GLB, so physics
follows Blender's exported topology and coordinates exactly. Moving obstacles are
named top-level nodes and remain separate from fixed colliders. The runtime
manifest is compact JSON; pretty-print it for inspection with
`python -m json.tool ui/3d/minigolf/manifest.json`.

Sound effects and music are not part of this Blender pack. Their sources,
provenance and rebuild steps are in [`audio/README.md`](audio/README.md).

## Module map

`generate.py` is the CLI. All logic is in the `kit/` package:

| Module | Role |
|---|---|
| `courses_v1.py` | The 18 legacy course definitions and the lane, rail, hole and obstacle geometry, unchanged and guarded by the P1 fingerprints. `build_course` takes the `Dresser` that replaces the decor. |
| `dress.py`, `dress_island.py`, `island.py`, `trim.py` | Dressing a course: the island terrain and strata, the shore strip, the rail trims, the bake. |
| `dress_specials.py`, `special_parts.py`, `dress_parts.py` | Layout specials as fixed placements or hand-built parts; the functional parts (obstacles, templates, flag, hole digits, tee ring, cup). |
| `placement.py`, `occluders.py` | Deterministic prop placement and its rule check; the kit objects used as bake occluders. |
| `export.py` | Export, canonical node names, merging of scenery per material, GLB reading. |
| `kits.py` | Builds each world's prop kit and its manifest entry. |
| `props_pirate.py`, `props_temple.py`, `props_moon.py` | The props of each world (`PROPS`, `PROP_META`). Helpers: `props_pirate_*.py`, `ship_pirate.py`, `props_temple_*.py`, `head_temple*.py`, `props_moon_*.py`. |
| `materials.py`, `palette.py`, `colors.py`, `shapes.py`, `prop.py`, `bake.py` | Shared materials, palettes, vertex colours, shape builders, prop composition and the ambient-occlusion bake. |
| `previews.py` | Menu preview renders (Cycles, `--only previews`). |
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
```

- `--only` takes `kits`, `courses`, `previews` or `all` (comma-separated). The
  default is `kits,courses`; with `--course` it is `courses`.
- `--only kits` rebuilds the three prop kits and keeps the course entries; a
  courses build keeps the kit entries.
- `--course N` (1–18) rebuilds one course in a fresh process and patches its
  manifest entry. A full courses build must exist first.
- `--save-blend` is allowed only together with `--course N`. A full run deletes
  each course after its export, so the scene would not survive.
- `--only previews` is separate because Cycles renders are not bit-reproducible.

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

## Sizes of the last full build

| Item | Size |
|---|---|
| 18 course GLBs | 10,065,848 bytes (largest 754,904; 336,816 triangles in all, largest 21,149) |
| `manifest.json` | 3,312,219 bytes |
| Three prop kits | 5,029,080 bytes (pirate 1,771,100, temple 1,768,908, moon 1,489,072) |
| Whole `ui/3d/minigolf/` package, including audio and previews | 21,320,234 bytes |

The asset test enforces the budgets (course at most 768 KiB and 24,000 triangles
and 24 mesh nodes; kit at most 3 MiB and 60,000 triangles; prop at most 15,000
triangles, hero 20,000; package at most 32 MiB). Regenerated sizes can vary with
edits; the test is the authoritative budget check.

## What the asset test checks

`node scripts/test-minigolf-assets.mjs` checks:

- the budgets above, the GLB structure, embedded resources and the glTF validator;
- the P1 fingerprints of each course's colliders and of its physics fields
  (`obstacles, tee, cup, cupRadius, bounds, water, par`), read from
  `scripts/fixtures/minigolf-p1-colliders.json`;
- no numbered node or mesh names (`scenery-matte.001`);
- each course's `look`, the `worlds` list, and the `path` (starts within 0.3 m of
  the tee, ends within 1 m of the cup);
- every placement: a known kit prop, a distance to the island rim (`island`), and
  a clear lane corridor (except `span` props);
- each kit: top-level prop nodes, `COLOR_0` on every primitive, the prop and kit
  budgets, and the tags in the node extras;
- the previews: WebP signature and size;
- the P1 rules that still apply: an upward-facing tee ring and cup patch, named
  obstacle nodes, and open cup collision geometry.

The full runtime check is `npm run test:minigolf`, described in
[`documentation/minigolf.md`](../../documentation/minigolf.md).
