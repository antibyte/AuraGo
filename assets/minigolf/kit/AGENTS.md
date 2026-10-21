# AuraGolf procedural kit

## Purpose

Blender 5.2 modules behind AuraGolf's diorama look. `../generate.py` is a thin CLI
over this package (`--only kits,courses,previews|all`, `--course N`,
`--save-blend`). The package builds:

- per-world prop kits (`props-<look>.glb`, one per `pirate`, `temple`, `moon`),
- the 18 dressed courses (`course-NN.glb`) from the unchanged legacy geometry in
  `courses_v1.py`,
- the manifest entries for `look`, `path`, `props`, `island`, `kits` and `worlds`,
- the menu previews (`--only previews`).

`probe.py` and the `scene_<world>.py` files are style probes for the look; they
do not feed the game build. The module map is in [`../README.md`](../README.md).
Nothing here writes `ui/` except `generate.py` through its runtime paths.

## Ownership

- MIT licensed like `../LICENSE`; every module starts with a docstring saying so.
- Blender 5.2.2 headless only. No third-party Python packages; the numpy bundled
  with Blender is used only by the optional `stage.clip_report` diagnostic.
- Probe PNGs go to the `--out` folder. Final WebP probe files go to
  `documentation/auragolf/img/p2-stilprobe-<world>.webp`, never from `--quick`.

## Local Contracts

### Determinism

- Randomness is `random.Random(seed)` or `mathutils.noise` (fixed permutation). Seeds
  in use: kits and props `seed=1`, placement `1000 + number`, island `4000 + number`,
  trims `7000 + 2 * number + side`.
- No time input and no unordered iteration.
- Bakes are CPU, fixed seed and sample count, no denoise, rounded to 8 bits.
- Each course is deleted after its export (`courses_v1.discard_scene`: objects,
  meshes, scene, then every orphan; `generate.clear_scene` does the same before the
  first course). Blender numbers a name that is still taken, so no exported node or
  mesh name may carry a counter. `--course N` in a fresh process must write the
  same file as the full run.
- `probe.py` prints a `scene_hash` (vertex positions plus baked 8-bit colours) that
  must be identical between runs. Cycles beauty renders are only visually identical
  (threads and OIDN).

### Materials

- Kinds: `matte`, `satin`, `metal`, `glass`, `water`, `emissive`, `glow`, `foliage`,
  `cloth`. Every material reads `Col` through a Color Attribute node, which the
  exporter writes as `COLOR_0`. `emissive` takes a dimmed copy as base colour, so
  its emission keeps the gradient. `glow` keeps full lighting plus soft emission.
- Extras on the material (`materials.get`):
  - `aurago_kind` on every material;
  - `aurago_strength` on `emissive` and `glow`, snapped to `EMISSION_STEPS`
    (1.0, 1.6, 2.5) or, for `glow`, `GLOW_STEPS` (0.3, 0.7, 1.0), so a prop never
    needs a primitive per brightness;
  - `aurago_flicker` (0–1) for flames, set on `emissive`/`glow`;
  - `aurago_sway` (metres) on `foliage` and `cloth`.
- The runtime reads `aurago_kind`, `aurago_flicker` and `aurago_sway`. It does not
  read `aurago_strength`: the emission it draws is the exported glTF emission,
  multiplied by `COLOR_0`. Keep the exported emission strength near 1–2.5, because
  Neutral tone mapping desaturates highlights above about 0.8.
- Only `foliage`, `cloth` and `glass` are double sided; every other kind culls back
  faces.
- Each variant is its own datablock (for example `kit-emissive-1.6-f0.35`,
  `kit-foliage-s0.06`).

### Prop contract (`PROPS`, `PROP_META`)

- A builder returns ONE joined mesh named `prop-<look>-<name>`, Z-up, metres, base at
  z = 0, footprint centred, accepts `seed`, and is registered in its module's
  `PROPS` dict. Helpers that are not props (the pirate foam ring) stay out of
  `PROPS`.
- `PROP_META[name]` holds `tags` (a subset of `hero`, `tall`, `scatter`, `flat`,
  `span`) and optional `clearance` (headroom in metres, for `span` only) and
  `spin: {axis, speed}`.
- `kits.build_kit` writes these as node extras `aurago_prop`, with `radius`,
  `height`, `tags` and any `spin` or `clearance`.
- Limits, checked while building and again on the exported GLB: at most 3
  primitives per prop, 4 for a `hero`; at most 8 materials per world kit. The
  asset test adds the triangle budgets (see `../README.md` and
  `documentation/minigolf.md`).
- A `hero` is moved so the centre of its smallest enclosing plan circle is the
  origin (`kits.centre_footprint`). The radius stays as small as possible and the
  placement may scale it up.
- The moon dish is two props, `dishbase` and `dishhead`, placed with one transform
  on the vertical axis x = y = 0 (the head is not based at z = 0). `dishhead`
  carries `spin`, so the runtime turns it about that axis.

### Kit export (`kits.build_kit`)

- Each world builds in a fresh scene. Props are built with `seed=1`, sorted by
  name, placed 40 m apart on bake plates, and baked per prop (contact AO at the
  foot, 160 samples). Plates are removed before the export.
- The origin is reset, the primitive and material limits are checked, and the
  kit is exported as `props-<look>.glb` with the props as top-level nodes.
- The manifest entry is `{model, bytes, sha256, triangles, props: {name: {radius,
  height, tags, triangles, spin?, clearance?}}}`.

### Dressing a course

- `courses_v1.build_course(number, course, dresser)` builds the lane, collision
  rails, obstacles, tee, cup and flag with the legacy code. The `Dresser` in
  `dress.py` replaces only the decor around them.
- Bit-identical geometry and data:
  - `surface-grass` and `surface-sand` get a POINT colour only (no bevel,
    smoothing or shading change);
  - `rail-left` and `rail-right` get `aurago_hidden` only, with no colour;
  - the colliders, `tee`, `cup`, `obstacles`, `bounds`, `water` and `par` match
    `scripts/fixtures/minigolf-p1-colliders.json` (keys `colliders` and `physics`);
    the asset test checks them.
  - `bounds` come from a throw-away legacy build, so the out-of-bounds test does
    not change.
- Island (`dress_island.Terrain`, `island.Outline`): the top height `top_height(x, y)`
  is the base ground, a bed 6 cm under the lane (smoothed over 1.1 m to 2.0 m),
  the cup hollow, and a trench across the island under gap courses. Three strata
  per world plus a submerged skirt (Pirate Bay, Jungle Temple) or a floating
  underside (Moon Station). The `water-shore` strip has `uv.x` from 0 (shore) to 1
  (outside). No shore on the moon or across a trench.
- Trims (`trim.py`): `rail-trim-left` and `rail-trim-right` share
  `courses_v1.rail_edge` with the rails. The inner face is flush with the rail;
  the top stays at most 2 cm above and the face at most 3 cm outside it. The outer
  face reaches down at most 0.5 m, to the ground.
- Functional parts (`dress_parts.restyle`): obstacles (bevel only, exact extents),
  ball (icosphere on exactly r = 0.16, plain white, tinted by the game), putter
  (steel shaft 0 to 0.82 m, bevelled head), flag (`cloth`, double sided), hole
  digits (`emissive`, 8 mm bevel), tee ring, cup rim (2 cm thick below its
  unchanged top, 8 mm bevel) and lining, and a felt-coloured `cup-patch` that hides
  the dark ground under the stepped cut. The legacy `tee-ring` and `cup-patch` wind
  their faces downwards; `restyle` flips them face up, and the asset test requires
  +Y normals.
- Templates and specials: `dress_specials.build` turns each layout special into
  fixed kit placements and/or hand-built parts (`special_parts`), in the place and
  size of the old part. Pier beams, bridge parapets, the sun wheel (blades
  `windmill-sail-0` to `-3`), moon pads and the gate hub are hand-built.
- Bake (`Dresser.bake`): only the island, trims, specials, cup parts and lane are
  baked. Each placed prop is linked into the scene as an occluder (unbaked kit
  objects of the look, built once per process by `occluders.py`) and removed before
  the export. Templates, obstacles, rails and the shore are hidden from the
  occluders; the shore and templates only get `Col`.
- Export and merge (`export.join_scenery`): every mesh that is not functional is
  joined per material into `scenery-<material>` nodes. Functional names stay
  single nodes: `surface-*`, `rail-*` (including `rail-trim-*`), `obstacle-*`,
  `windmill-sail*`, `flag`, `ball-template`, `putter-template`, `water-shore`.
  Builder properties are stripped. A course has at most 24 mesh nodes.
- The manifest `path` runs to the end of the lane, up to 1 m behind the cup (the
  cup sits at 91 % of the lane), so the corridor also covers the end stub.
- `island` is the rim in runtime coordinates, `{center: [x, z], half: [hx, hz],
  power, wobble}`: a superellipse around `center` (`Dresser.runtime_island`). The
  true wobbled outline scales its radius by at most `1 ± wobble`, measured over 720
  angles and rounded up to 1/1000. The runtime does not read `island`; the asset
  test and the placement rules do.

### Track kit core (P3)

`track.py`, `pieces.py`, `pads.py`, `layout.py`, `rails.py`, `build_track.py`, `floor_paint.py` and
`basic_parts.py` are the base of the P3 courses. `generate.py` does not call them yet; `courses_v1.py`
keeps building the legacy courses until the switch-over. The first five are plain Python (no `bpy`).

- Coordinates are Blender X/Y/Z, `heading` in radians (0 = +X, positive turns left). A `Section` has
  `ROWS + 1` = 13 points from left to right, `SPACING` = 1.72 / 12 apart. A piece returns only its new
  sections; `layout` puts the previous section object in front, so seam points are the same tuple objects
  and are never recomputed. A pad uses the point objects of its entry section on its back row; its exit
  section is made from its own raster points. Floor meshes merge vertices through a dict keyed by the exact
  tuple. `track_selftest.check_seams` guards this: one tuple object per distinct point value.
- Pad frame: origin = centre of the entry section, x forward, y left. `cup = (x, y)`, `contour` bumps and
  `pools` polygons use it. Width is whole columns of `SPACING` with an even number beside the lane. Raster
  cells whose centre is closer than `CUP_RADIUS + 0.13` to the cup, or whose rectangle reaches within
  `CUP_RADIUS`, have no floor.
- Rails: a side is walled between two sections when the later one has the flag; the wall follows `wall_up`
  (default the section normal). Riser edges of `steps` have no wall. `rails.rail_runs(track)` chains strip
  sides and pad perimeters into runs `[(point, outward, up)]` (mitred at bends); the rail trims will use them.
- `layout.PIECES` and `layout.PADS` are the registries; later modules extend them. Parameter errors are
  `ValueError`s that `layout.build` prefixes with the course number and piece index.
- Collision nodes (contract V1): `floor-<look>` (`floor-<look>-<surface>` if one look has both surfaces) with
  the extra `aurago_collider` = `grass` or `sand`; `rail-wood|stone|metal` with `aurago_collider` and
  `aurago_hidden`. Collision objects keep the identity transform (world coordinates).
  `export.glb_collision(path)` welds the vertices of each surface by Float32 bit pattern in order of first
  use, drops collapsed triangles and returns the digest: surfaces in the order grass, sand, wood, stone,
  metal; per surface the name, `0x00`, Uint32 LE vertex and index counts, Float32 LE vertices, Uint32 LE
  indices. It must agree bit for bit with `colliderDigest` in `ui/js/desktop/apps/minigolf-collision.js`.
- `export.canonical_export_names` keeps `floor-*`, `obstacle-*`, `mechanic-*` and the three rail names
  unnumbered; `is_functional` keeps them single nodes.

### Placement (`placement.py`)

- Deterministic: `random.Random(1000 + number)` and a 0.25 m candidate grid.
- Order: fixed placements (special parts), then the hero (largest free circle,
  scale at least 0.42, shifted towards the back), medium props (biggest first;
  the same kind at least 2.2 m apart), then scatter (beach critters in the shore
  band).
- Corridor: the footprint circle (`radius` × scale) keeps `CORRIDOR` (1.2 m; the
  asset test needs 1.0 m) from the `path` polyline, except `span` props.
- Rim: everything stays within the rim (`EDGE` 0.3 m) plus a share of its radius:
  `RIM` shares per name (fern, bush, tuft and similar), `TALL_RIM` 0.6 for tall
  props, `HERO_RIM` 0.85 for a hero. Those shares let fronds and rigging overhang,
  since the enclosing circle is mostly thin parts.
- Solid share: `SOLID` and `CANOPY` describe how much of a prop's radius blocks
  other props. `HERO_SOLID` does the same for heroes.
- Nothing stands in a trench. Tall props that would hide the lane from the game
  camera (`VIEW`) and everything tall in front of the hero are skipped.
- A fixed special that reaches over the rim moves towards the island centre in 5 cm
  steps (`settle`). The dish is placed by the larger circle of its head.
- Density: at most `LIMIT` (160) placements per course.
- `placement.check` repeats the asset test's rules for every placement, fixed ones
  too (corridor, and `EDGE` plus rim share inside the rim). It fails the build.

### Look

- Khronos PBR Neutral view transform (look None, exposure 0, gamma 1), matching
  three.js `NeutralToneMapping`.
- Camera: perspective, 43 degrees vertical FOV (sensor fit vertical), 35–40 degrees
  elevation, 1600×900 probe frames.
- Dark scenes need a direct fill light (the moon probe in `scene_moon.py` adds a
  wide, dim `kit-fill` sun); world light alone leaves speckle inside concave shells
  in shadow.
- The probes are Cycles renders with soft shadows and mild GI. The runtime
  approximates them with baked AO, one shadow-casting sun and environment lighting;
  treat the probes as the colour and shape target, not a pixel target.

## Work Guidance

- Keep modules small and split before they pass about 400 lines. Exceptions are the
  legacy `courses_v1.py` (about 730 lines; its geometry is unchanged, so only the
  hooks change) and `shapes.py` (about 490). Do not grow them; add new code to a
  neighbouring module.
- New props go into the world's `props_<world>.py` and `PROPS`; shared builders go
  into `shapes.py`.
- Give repeated parts unique names (`windmill-sail-0` to `-3`). Rename a replacement
  mesh only after the old one is removed (`shapes._apply_modifiers`,
  `dress_parts._putter`).
- Use `densify` (`dens=`) only where AO resolution matters (large flat faces). Thin
  parts use `dens=0` or a large value to save triangles.
- Iterate with `--prop world:name --view az,el,k` for one-prop close-ups and
  `--quick` (800×450) for look renders. Review a course with `--save-blend` and a
  scratch Cycles render.

## Verification

- Kits: `blender --background --factory-startup --python assets/minigolf/generate.py -- --only kits`
  (about 15 s) prints one `kit <look>: ...` line per world. Run it twice; the kit
  `sha256` values must match.
- Courses: `-- --only courses --course N` (about 15 s) rebuilds one course in a fresh
  process for iteration. A full run (`-- --only kits,courses`, about two minutes on 16
  cores) rebuilds all three kits and 18 courses. Then run
  `node scripts/test-minigolf-assets.mjs`, `node scripts/test-minigolf-courses.mjs`
  and `node scripts/test-minigolf-physics.mjs`. A second full run must give
  identical GLB and manifest hashes.
- Track kit: `blender --background --factory-startup --python assets/minigolf/kit/track_selftest.py -- --out <dir>`
  builds the probe course, exports `selftest.glb`, checks seams and the open cup, writes `selftest.json` and
  prints `selftest ok`. Run it twice: the GLB hash and the digest must match, and the digest must equal
  `colliderDigest` of the same GLB.
- Previews: `-- --only previews` (about a minute). They are never part of a full
  run, because Cycles output is not bit-reproducible.
- Probes: `blender --background --factory-startup --python assets/minigolf/kit/probe.py -- --out <scratch> --world all`
  renders the three probes (about 2.5 minutes each on 16 cores at 128 samples) and
  prints per-prop and scene triangle counts, bake and render times and the
  `scene_hash`. Options: `--world pirate|temple|moon|all`, `--samples N`, `--quick`,
  `--prop world:name`, `--view az,el,k`, `--webp-dir`, `--no-webp`, `--save-blend`,
  `--no-render`. `--no-render` rebuilds and bakes only; run it twice and compare
  `scene_hash`.

## Child DOX Index

No child AGENTS.md files.
