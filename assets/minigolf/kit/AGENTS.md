# AuraGolf procedural kit

## Purpose

Blender 5.2 modules behind AuraGolf's diorama look. `../generate.py` is a thin CLI
over this package (`--set v1|p3`, `--out DIR`, `--only kits,courses,previews|all`,
`--course N`, `--save-blend`). The package builds:

- per-world prop kits (`props-<look>.glb`, one per `pirate`, `temple`, `moon`),
- with `--set p3` the courses of `course_data_<world>.py` through the track kit and
  `courses.py` (`course-NN.glb`, manifest version 2, see "Course pipeline (P3)"),
- with `--set v1` (default until the switch-over, frozen) the 18 dressed courses from the
  legacy geometry in `courses_v1.py` and the manifest version 1,
- the menu previews (`--only previews`, v1 only).

`--set p3` needs `--out DIR` (for example `assets/minigolf/build/p3`, ignored by git) until the
switch-over makes `ui/3d/minigolf` its default. Kits are written to `--out` as well.

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
  trims `7000 + 2 * number + side`, support piles `8100 + number`, bridge parts and tunnel parts `number`
  (portals `number + tunnel index`).
- No time input and no unordered iteration.
- Bakes are CPU, fixed seed and sample count, no denoise, rounded to 8 bits.
- Each course is deleted after its export (`courses.discard_scene`, legacy `courses_v1.discard_scene`: objects,
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

### Dressing a legacy course (v1, frozen)

The `Dresser` in `dress.py`, `dress_island.Terrain`, `trim.py` and `placement.py` now belong to the
P3 pipeline and no longer fit `courses_v1.build_course`; `--set v1` is not expected to build until
`courses_v1.py`, `dress_specials.py` and `special_parts.py` are deleted at the switch-over. The
bullets of this section describe the P2 behaviour that the P3 pipeline generalises.

- `courses_v1.build_course(number, course, dresser)` builds the lane, collision
  rails, obstacles, tee, cup and flag with the legacy code. The `Dresser` in
  `dress.py` replaced only the decor around them.
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

`track.py`, `pieces.py`, `pieces_moon.py`, `pads.py`, `pads_funnel.py`, `layout.py`, `layout_fork.py`, `rails.py`,
`build_track.py`, `floor_paint.py`, `cup_hole.py`, `cup_parts.py`, `track_checks.py` and `basic_parts.py` are the base
of the P3 courses; `courses.py` (see "Course pipeline (P3)") drives them for `generate.py --set p3`. Plain Python (no
`bpy`): everything except `build_track.py`, `basic_parts.py`, `cup_parts.py` and `track_selftest.py`.

- Coordinates are Blender X/Y/Z, `heading` in radians (0 = +X, positive turns left). A `Section` has
  `ROWS + 1` = 13 points from left to right, `SPACING` = 1.72 / 12 apart. A piece returns only its new
  sections; `layout` puts the previous section object in front, so seam points are the same tuple objects
  and are never recomputed. A pad uses the point objects of its entry section on its back row; its exit
  section is made from its own raster points. Floor meshes merge vertices through a dict keyed by the exact
  tuple. `track_checks.check_seams` guards this: one tuple object per distinct point value.
- Pad frame: origin = centre of the entry section, x forward, y left. `cup = (x, y)`, `contour` bumps and
  `pools` polygons use it. Width is whole columns of `SPACING` with an even number beside the lane.
- Exact cup (`cup_hole`, `pads`, `pads_funnel`, `cup_parts`): the hole is a polygon of `CUP_SEGMENTS` (32) corners
  on the circle of radius `CUP_RADIUS` (0.32 m), `pad.cup_ring`. A green removes the raster cells that touch the
  circle of radius `CUP_RADIUS + SPACING` and fills the ring between their jagged boundary and the polygon with
  triangles (`pad.tris`, the cheapest counter-clockwise triangulation by dynamic programming over both rings;
  the boundary must be star-shaped around the cup, otherwise the build says to move the cup a few centimetres).
  A funnel's innermost ring is that polygon (z = bowl height at `CUP_RADIUS`, which is also the pad's `cup`
  height). `cup_parts.floor_cup` adds the collision cup: node `floor-cup` (`aurago_collider` `grass`, dark
  vertex colour), a tube from the polygon down `CUP_DEPTH` (0.30 m) with an inward-facing wall and a bottom
  disc; its rim corners are the polygon's tuple objects, so floor and cup weld in the runtime. The cup rule
  of the physics is unchanged (ball centre below rim - 0.1, within the radius, at most 2.4 m/s). The island
  hollow under the cup (`terrain.HOLLOW_*`) is wider than the top grid cell so no ground pokes into the tube.
- Rails: a side is walled between two sections when the later one has the flag; the wall follows `wall_up`
  (default the section normal). Riser edges of `steps` have no wall. `rails.rail_runs(track)` chains strip
  sides and pad perimeters into runs `[(point, outward, up, height)]` (mitred at bends); `height` is
  `WALL_HEIGHT` or the section's `wall_height`. The rail trims are built from these runs.
- `layout.PIECES` and `layout.PADS` are the registries; `pieces_moon` and `pads_funnel` add their entries when
  imported (the layout imports them). Parameter errors are `ValueError`s that `layout.build` prefixes with the
  course number and piece index (a fork branch adds `left branch,`/`right branch,` and its own piece index).
- Moon pieces (`pieces_moon`): `halfpipe` needs a flat, aligned `prev`. Its `ROWS + 1` points sit at equal
  arc-length steps on the U (quarter circle `radius`, flat `width`, `depth` <= `radius`; `depth == radius` ends the
  lips vertically) and blend `lerp(flat, U, smoothstep(s / blend))` over `blend` >= 1.5 m at both ends; the path is
  straight (`length`) or an arc (`radius_path`, `angle`), sections at most 0.30 m apart, lip walls with `wall_up` =
  world up and height `lip_wall` (default 0.45 m, 0.24-1.0; it blends from the plain rail height over the
  transitions through `Section.wall_height`). In the selftest probe (g = 4, powers 0.35 and 0.5, 34 diagonal
  shots each) 0.45 m cut the shots that end `out` from 6 to 3 and from 13 to 11; only one shot still left over
  the rim inside the pipe (power 0.5, 25 degrees), the others fall from the exit lane downstream. `loop` (R 1.2-1.5, `offset` >= 2.2 m to either side, `sections` default 72, minimum 48) sweeps the
  centreline formula of the plan; the first section is `prev`, the last is flat and offset; floor normal
  `n = T x across`, walls along `n`. Each loop section costs 40 collision triangles (24 floor + 16 rail).
- Funnel (`pads_funnel`): polar mesh around the cup, `z(r) = z_cup + depth * (r / R) ** 1.5`; outer ring = the entry
  section's point objects plus evenly spaced rim points, rings joined by a zipper over their angles, first inner
  ring 0.35 m inside the deepest entry point, innermost ring exactly at `CUP_RADIUS` (32 points), rim wall
  except over the entry arc. `R` 2-4 m, rim slope at most 22 degrees (`depth <= 0.27 R`), the whole entry section
  within `[R - 0.25, R]` of the centre (the default centre puts the corners 5 cm inside). The pad's `grid` is
  `{"polar": True, "rings", "centre", "radius", "samples"}` and `faces` are point triples. On the moon (g = 4) a
  ball rests on slopes below 11 degrees (rolling deceleration 0.8 m/s^2), so a funnel needs `depth >= 0.172 R^1.5`
  to keep resting balls within 0.6 m of the cup; `R` 2.4 with `depth` 0.64 collects shots of power 0.28-0.36.
- Fork (`layout_fork`, `pads.fork`, `pads.join`): `("fork", {length, spread, left, right, main, join_length,
  exit_offset, width, + style})`. `spread` is an even multiple of `SPACING`, at least 16 columns (2.29 m). The split
  pad has two exit ports; each branch is a piece list walked with the layout (no green or funnel inside); the join
  pad's back row is the left end section, raster points, the right end section. Branch ends must agree within 0.01
  degrees (heading) and 1 mm (height, forward coordinate) and be an even multiple of `SPACING` apart. `main` is
  `Track.path`; the other branch is an extra polyline in `Track.corridors` from the section before the fork to the
  join exit.
- Layout rules: a loop needs a `boost` mechanic anywhere in the course data or an 8 m flat straight run-up; clearance
  (0.5 m) is checked between route parts more than 4 m apart along the route, never within one loop, always between
  the two branches of a fork.
- Jump: `ramp` with `lip` (degrees), then `gap` (hazard `water`/`void`, `landing` section stored in
  `track.hazards`), then a flat piece; the pirate probe course covers it.
- Collision nodes (contract V1): `floor-<look>` (`floor-<look>-<surface>` if one look has both surfaces) with
  the extra `aurago_collider` = `grass` or `sand`; `rail-wood|stone|metal` with `aurago_collider` and
  `aurago_hidden`. Collision objects keep the identity transform (world coordinates).
  `export.glb_collision(path)` welds the vertices of each surface by Float32 bit pattern in order of first
  use, drops collapsed triangles and returns the digest: surfaces in the order grass, sand, wood, stone,
  metal; per surface the name, `0x00`, Uint32 LE vertex and index counts, Float32 LE vertices, Uint32 LE
  indices. It must agree bit for bit with `colliderDigest` in `ui/js/desktop/apps/minigolf-collision.js`.
  The collision rails are smooth shaded (they are hidden), the floors stay flat shaded; both keep their
  welded geometry, only the GLB gets smaller.
- `export.canonical_export_names` keeps `floor-*`, `obstacle-*`, `mechanic-*`, `rail-trim` and the three rail
  names unnumbered; `is_functional` keeps them single nodes.

### Course pipeline (P3)

`generate.py --set p3 --out DIR` calls `courses.build_course(number, data, world, out, keep_scene)` for every course
of `course_data_pirate|temple|moon.py` (plain literal lists `COURSES`; the shape is documented in `layout` and
`course_data_pirate`; `--course N` rebuilds and patches one entry of `DIR/manifest.json`, a missing manifest is
created, a partial course set is allowed until the switch-over). Course 1 of the pirate module is a placeholder
with probe content. The steps:

- `layout.build` makes the `Track`; `build_track.build` the floors, collision rails (+ `runs`) and `floor-cup`;
  `cup_parts.hole_marks` the tee ring, cup rim, flag, hole number and the ball/putter templates; then the
  `Dresser` (`dress.py`): `dress` (island, shore, trims), `place` (props), `restyle`, `bake`; then
  `export.join_scenery`, the export, `export.glb_collision` and `course_entry.entry`.
- Island (`footprint.py`): `island.Outline(power 3.0, wobble 0.035)` around the floor points of the layout, the hero
  circle (scale 1.0) and the fixed props plus a 2.5 m margin: centre = middle of the bounding box, half axes keep its
  aspect ratio and grow (1.5 % steps) until the margin samples around the convex hull lie inside the true wobbled
  outline; a half axis above 20 m fails the build. Top grid cells are 0.35 m (`dress_island.CELL`), coarser when the
  top would exceed `MAX_TOP_QUADS` (4400) quads.
- Terrain (`terrain.py`, `bed_field.py`): `top_height(x, y)` = base ground pulled to a bed 6 cm under the nearest
  grounded floor triangle (`section.grounded` and the section centre at most `GROUNDED_MAX` above the start
  height; 1 m cell hash of the triangles; smoothed from 1.1 m to 2.0 m); a second stretch of the track (route arc
  more than 3 m apart) within 2 m caps the ground at its bed; elevated pieces do not pull the ground up; cup hollow;
  pond hollow (water surface 0.25 m under the pool floor, bottom 0.45 m lower; `void` pools and `gap` hazards cut
  through the island on the moon, down below the water line elsewhere); a gap trench is a strip across the island
  that gives way beyond 2.4-3.2 m from the lane wherever another stretch of the track passes through it.
  `Terrain.wet` (trench or pond) keeps props out.
- Trims (`trim.py`): one node `rail-trim` from the rail runs (thinned within 4 mm, 4 degrees and 1 cm), per run
  the side (left of the route: cyan neon on the moon) decides the seed `7000 + 2 * number + side`. Flush with the
  rail inner face, at most 2 cm above and 3 cm outside it, follows `up` and the wall height; posts, clamps and
  the ground skirt only where the wall is upright.
- Placement (`placement.py`): `Field` takes `corridors` (every route of `Track.corridors`) and the plan points of
  all floor vertices; free props keep `CORRIDOR` (1.2 m) from every centre line and `FLOOR_GAP` (0.45 m) from the
  floor (wide pads), the asset test needs 1.0 m. Order: fixed parts (`fixed`), the hero of the course data
  (`{"at", "yaw"}`, scale 1.0, 0.95 or 0.9, else a `ValueError` that names the conflict), the fixed props
  (`(name, (x, y), yaw, scale)`), medium props (biggest first, the same kind at least 2.2 m apart; a kind that was
  placed by hand is not placed again), then scatter. `LIMIT` is 220. `placement.check` repeats the asset test's rules
  for every placement (corridors, rim share, hero at least 0.9) and fails the build.
- Manifest entry (`course_entry.py`, contract V2): `bounds` = island outline box + 1.5 m in x/z, y from
  `min(water - 0.4, lowest floor - 4)` (moon: second only) to the highest floor or cannon arc + 6; `water` = sea slab
  (pirate, temple), ponds and, outside the moon, gap volumes. `mechanics`, `obstacles` and `cycle` are passed through
  from the data. Budgets are enforced by `courses.check_budget`: 1.25 MiB, 36,000 triangles, 32 mesh nodes.
- Structures (supports, bridges, tunnels, ponds, trench walls) are described in "Structures (P3)", the mechanic and
  obstacle parts in "Mechanics and obstacles (P3)".

### Structures (P3)

`Dresser.build_structures(coll, built)` (after `dress`, before `place`) calls `structures.build`; its baked objects go to
`Dresser.extra`, the pond water to `Dresser.ponds` (colours only, hidden from the AO bake) and the `reserved` circles
to `place`. Modules: `support_plan` / `support_parts`, `bridge_parts`, `tunnel_plan` / `tunnel_parts` with
`portal_pirate` and `portal_temple`, `pond_parts`, `trench_parts`, `detail`, helpers in `part_utils`. `structure_selftest.py`
checks the rules below. A track without structures builds bit-identically.

- Supports (`support_plan`): a station about every 2 m of route (first and last 0.3 m from the ends of a strip) where
  the floor edge is more than 0.14 m above `terrain.top_height`: two posts under the lane edges, just inside the rail,
  and a cap beam 2 cm under the floor edge. Pirate wooden piles (6 sides, slightly tilted) with a cap plank, crossing
  braces from 0.6 m and a tie from 1.4 m; temple stone pillars (octagon, foot and capital blocks, moss at the foot) with a
  lintel; moon metal struts (panel mast, plate, orange band, leg from 0.7 m, brace from 0.9 m). Strips count as elevated
  where `bed_field` leaves them out of the ground (not `grounded`, or more than `GROUNDED_MAX` above the tee), and every
  `deck: bridge` strip is checked, so a bridge over a pond or trench gets piles standing in the water. A post is dropped
  where another stretch (floor triangles more than 1.2 m apart along the route, or a rail run) passes within 0.25 m in plan
  at a height between its bottom and 0.12 m under its top, so a support never stands in another lane; a bridge over a wide
  pad therefore carries itself by its stringers. A loop gets a frame: at about every 2 m of its arc above 0.7 m, a post
  1.30 m (else 1.55 m) beside the centre line on each side with an arm to the outer face of the rail (the ball runs on the
  inside of the ring, nothing stands under it). Elevated pads are not supported (pads always count as grounded).
- Bridges (`bridge_parts`): a piece with `deck: "bridge"` gets the floor look `deck` (pirate: planks, temple: bark) or
  `metal` (moon) unless it sets its own `look` (`layout`); railing posts 1.08 m from the centre line (outside the lane,
  rail and trim, which reach 1.03 m) along each side that has a rail wall, with ropes (pirate), a mossy stone curb
  (temple) or a neon tube (moon); stringers under the lane edges and joists every 0.55 m with their top 1.2 cm under the
  floor (a stone slab 0.2 m thick in the temple). The trim skirt (`trim`) fades back to the plain trim where the ground
  is more than 0.5 m below the lane edge, so the structure under a bridge stays visible.
- Tunnels (`tunnel_plan`, `tunnel_parts`): connected strips with `cover: "tunnel"` form one run. The inner opening is
  2.5 m wide and 1.45 m high over the floor (the rail trims reach 1.17 m; the rule is 1.9 x 1.0 m). The roof is an arch band
  swept along the lane (rings every 0.45 m, dark inside, rock with noise pirate, mossy stone temple, panels with orange
  bands moon, a thick collar at the exit and, on the moon, at the entrance). The entrance portal is a solid body of
  `PORTAL` depth around the lane: the pirate skull rock (2.7 m deep, mouth arch cut through it, teeth along the top of
  the arch above 1.3 m, round eye sockets, nose hollow, rocks at the foot) and, in the temple, a course-side copy of the kit
  head (own skull with the mouth cut through, 0.8 x depth, same colours, moss cap, eyes, lips and teeth above 1.3 m; the
  kit head has only a decorative recess and is not changed). The first `depth` metres of a tunnel must be straight and
  level and the run at least `depth + 0.3` m long; another stretch of the track must stay out of the roof volume
  (1.95 m above the floor); all three are `ValueError`s with the course and tunnel number. Roofs and portals have no
  collision: `layout` rejects `cover` on a `loop`, a `halfpipe` or a ramp with `lip`, and the lane has rails, so the ball
  cannot reach them. Props keep out through `reserved` circles along the tunnel and the island contains the portal
  (`footprint_points`).
- Ponds (`pond_parts`): every `water` pool of a plaza gets an object `pond-water` (kit material kind `water`, vertex
  colours from the shore tone to the deep tone), 0.25 m under the surrounding floor, a grid of 0.4 m quads over the pool
  polygon plus 0.45 m (the bank crosses the water level at about the polygon edge, so the border hides under it). They
  are joined into `scenery-water`. The manifest water volume runs from the pool bottom to 2 cm under the surface
  (`course_entry.water`). `void` pools have no water.
- Trenches (`terrain.warp`, `trench_parts`): a `gap` cuts a strip across the island. The top grid vertices within 0.6 m
  inside and 0.2 m outside a trench edge are snapped onto the edge (`Terrain.warp`, passed to `island.build_island`), so the
  island's wall is a straight, almost vertical strip; `trench-wall` objects (baked, matte) lay one row per stratum of the
  island in front of it (1.2 cm), so the cut shows the strata (moon: no water, dark floor). Where another stretch passes
  through a trench the ground stays (dam, grounded stretch) or the trench stays open (bridge); the walls stop where the
  trench is shallower than 0.3 m.
- Detail overrides (`detail.resolve`): the course data may carry `"detail": {...}` with `island_quads` (island top grid,
  default 4400 quads, 1200-8000), `loop_sections` (cross-sections of every loop without its own `sections`, 48-120) and
  `trim_facets` (facets of the rounded rail-trim top, default 4, 1-6). Heavy courses (loop, half-pipe, fork and funnel
  together) use them to stay inside 1.25 MiB and 36,000 triangles: on the moon probe `island_quads` 3000, `loop_sections` 56,
  `trim_facets` 2 saved about 18 % of the triangles and 13 % of the bytes.

### Mechanics and obstacles (P3)

`courses.build_course` calls `mechanic_data.resolve(track)` right after `layout.build`: it converts the authored
`track.mechanics` / `track.obstacles` into the manifest entries (runtime axes `(x, z, -y)`, the fields that
`validateMechanic` / `validateObstacle` of `minigolf-mechanics.js` read) and returns a `Resolved` object. After `dress`
`Resolved.check(bounds)` tests positions and cannon flights, `dresser.place(reserved=...)` keeps random props away, after
`place` `check_props` tests the flights against the placed props, and after `restyle` `mechanic_parts.build_all` builds the
parts: static parts are baked scenery (`Dresser.extra`), moving nodes go to `Dresser.pass_parts` (plain colours) and
`Dresser.movers` (hidden from the AO bake). `course_entry.entry` runs `mechanic_data.validate_runtime`, the Python mirror of
the two validators plus the cycle rule. Plain Python: `mechanic_floor`, `mechanic_data`, `obstacle_data`, `cannon_flight`;
Blender: `mech_util`, `mechanic_parts` (bumper, boost), `mechanic_portals` (teleporter), `mechanic_cannon`,
`obstacle_parts`, `obstacle_gates`.

- Data (Blender metres, degrees): every entry has `id` (`^[a-z][a-z0-9-]{0,31}$`, unique over mechanics and obstacles) and
  `kind`, placed by `at` `(x, y[, z])` or by `anchor` (a `("anchor", {"name": ...})` of the piece list) plus `offset`
  `(forward, left[, up])`; `heading` defaults to the route direction there (relative to the anchor heading with an anchor);
  `z` is the floor height from the track (the lowest floor where two stretches overlap: give `z`). Unknown keys fail.
  Mechanics: `bumper` (`radius`, `height`, `kick`, `surface`), `boost` (moon; `length`, `width`, `speed`), `teleport`
  (`radius`, `exit` as `(x, y[, z])` or a placement dict, `exit_heading`, `min_speed`), `cannon` (pirate; `target`, `flight`,
  `delay`, `barrel`, `breech`). Obstacles: `rotate` (pirate anchor weight, temple log; `amplitude`, `length`, `swing`
  `across|along`, `clearance`, `span`), `gate` (all worlds; `travel`, `width`, `height`, `sharp`), `slide` (`width`, `height`,
  `amplitude`), `boulder` (temple; `radius`, `amplitude`, `move`), `spin` (moon radar arm; `radius`); each with `speed` (rad/s)
  or `period` (s), `phase`, `surface`. `cycle * speed / (2 pi)` must be a whole number for every obstacle. The docstrings of
  `mechanic_data` and `obstacle_data` list the defaults.
- Collision shape = visible core. `obstacle_data` builds one body-frame part list (box, ball, cylinder; Blender axes, X along
  the body heading, origin = body origin = `obstacle-<id>` node origin) and derives the manifest `colliders` from it;
  `obstacle_parts` draws the node from the same list: the core is exactly the part extents, relief is flush or inside,
  slabs are 12 mm thinner than their box so relief never shares a plane. The node has the pose of `obstaclePose(spec, 0)`
  (`mech_util.pose_matrix`). A bumper's visible core is its collision cylinder (`mechanic_parts._bumper` fails the build
  when radius or height differ). Gates, gallows and door posts are scenery in world coordinates beside the rails.
- Teleporter art: pirate floor hatch (entry) and a lying barrel whose open end is at the exit; temple stone arch with idol
  head (teal glow entry, gold exit); moon transporter pad with a floating ring node `mechanic-<id>-ring` (`<id>-exit-ring`)
  carrying the node extra `aurago_spin` `{axis: [0, 1, 0], speed}`. The boost pad is a decal draped 5 mm over the floor.
- Cannon (`cannon_flight`): `velocity = (target - muzzle - g T^2 / 2) / T` in runtime coordinates, the muzzle sits `barrel`
  m from the breech point along the launch direction (fixed point iteration, elevation 18-72 degrees). Twenty samples of the
  flight must stay inside `bounds` and 0.5 m from floor, rails, obstacles, bumpers and props (the landing spot within 1 m is
  exempt from floor and rails). `bounds` takes the apex. In the Rapier probes (1/120 s steps) the first touchdown was within 1 mm of
  `target`, also uphill onto a floor 0.35 m higher.
- Budgets seen on scratch courses with everything of a world: bumper 0.7-0.9k triangles, hatch 0.9k, cannon 3.1k, pendulum with
  gallows 2.0k, gate with frame 1.6-1.8k, boulder 1.3k, radar arm 0.6k; moving nodes are not densified.

### Placement (`placement.py`)

- Deterministic: `random.Random(1000 + number)` and a 0.25 m candidate grid over the island.
- Rim: everything stays within the rim (`EDGE` 0.3 m) plus a share of its radius:
  `RIM` shares per name (fern, bush, tuft and similar), `TALL_RIM` 0.6 for tall
  props, `HERO_RIM` 0.85 for a hero. Those shares let fronds and rigging overhang,
  since the enclosing circle is mostly thin parts.
- Solid share: `SOLID` and `CANOPY` describe how much of a prop's radius blocks
  other props. `HERO_SOLID` does the same for heroes.
- Nothing stands in a trench or a pond. Tall props that would hide the lane from the game
  camera (`VIEW`) and everything tall in front of the hero are skipped.
- A fixed part that reaches over the rim moves towards the island centre in 5 cm
  steps (`settle`). The dish is placed by the larger circle of its head.
- Corridors, floor gap, hero order and `LIMIT`: see "Course pipeline (P3)".

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
- Legacy courses (`--set v1`, frozen): `-- --only courses --course N` (about 15 s) rebuilds one course in a fresh
  process for iteration. A full run (`-- --only kits,courses`, about two minutes on 16
  cores) rebuilds all three kits and 18 courses. Then run
  `node scripts/test-minigolf-assets.mjs`, `node scripts/test-minigolf-courses.mjs`
  and `node scripts/test-minigolf-physics.mjs`. A second full run must give
  identical GLB and manifest hashes.
- Track kit: `blender --background --factory-startup --python assets/minigolf/kit/track_selftest.py -- --out <dir>`
  runs the rule checks (`track_checks.check_rules`), builds the pirate and the moon probe course, exports
  `selftest.glb` and `selftest-moon.glb`, checks seams, geometry, the exact cup (hole polygon, `floor-cup` welded to
  the floor) and the half-pipe rails, writes `<name>.json` and prints
  `selftest ok`. Run it twice: the GLB hashes and digests must match, and each digest must equal `colliderDigest`
  of the same GLB. `track_checks` also runs without Blender.
- Structures: `blender --background --factory-startup --python assets/minigolf/kit/structure_selftest.py` checks the
  bridge look, the rejected tunnel covers, the support rules, the tunnel rules and the `detail` validation and prints
  `structure selftest ok`. Review a structure with a scratch course (bridge over a pond, ramp with supports, tunnel with
  each portal, moon chasm) and a Cycles render from the front of the portal and from the side of the bridge.
- P3 courses: `blender --background --factory-startup --python assets/minigolf/generate.py -- --set p3 --out assets/minigolf/build/p3 --only kits,courses`
  (course 1 of the data module; about 20 s with the kits, `--course N` about 15 s). Then
  `node scripts/minigolf-route-search.mjs --dir assets/minigolf/build/p3 --course pirate-01` must find a verified route.
  `--course N` run twice, and once inside a full run, must give identical GLB and manifest-entry hashes. Review
  a course with `--course N --save-blend` (writes `assets/minigolf/build/aura-golf.blend`) and a scratch Cycles render.
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
