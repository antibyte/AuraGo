# AuraGolf procedural kit

## Purpose

Reusable Blender 5.2 modules for the stylized AuraGolf diorama look: palettes,
soft shape builders, vertex-colour gradients, baked ambient occlusion, per-world
props, lane sections and the style-probe renders (P2 "Look & Welt-Requisiten").
It is the modular generator behind `../generate.py` (a slim CLI: `--only
kits,courses`, `--course N`, `--save-blend`): `courses_v1` and `export` hold the
legacy course code moved verbatim, `kits` exports one prop kit GLB per world,
`dress` / `dress_island` / `trim` redress a legacy course as a diorama. The
probes in this folder do not touch `ui/`; `generate.py` writes `ui/3d/minigolf/`.

## Ownership

- MIT licensed like `../LICENSE`; every module starts with a docstring saying so.
- Blender 5.2.2 headless only (`blender --background --factory-startup --python
  assets/minigolf/kit/probe.py -- --out <dir> [--world pirate|temple|moon|all]
  [--samples N] [--quick] [--prop world:name] [--no-render]`). No third-party
  Python packages; the numpy bundled with Blender is used only for the optional
  `stage.clip_report` diagnostic.
- Probe PNGs go to the `--out` folder; final WebP files go to
  `documentation/auragolf/img/p2-stilprobe-<world>.webp` (never from `--quick`).

## Local Contracts

- Determinism: all randomness is `random.Random(seed)` or `mathutils.noise`
  (fixed permutation); no time input, no unordered iteration. `probe.py` prints a
  `scene_hash` (vertex positions plus baked 8-bit colours) that must be identical
  between runs. The Cycles beauty render is only visually, not bit, identical
  (threads and OIDN).
- Prop contract: a builder returns ONE joined mesh named `prop-<world>-<name>`,
  Z-up, metres, base at z=0, footprint centred, accepts `seed`, and is registered
  in the module's `PROPS` dict. The module's `PROP_META` holds each prop's
  placement `tags` (`hero`, `tall`, `scatter`, `flat`, `span`) and optional
  `clearance` / `spin`; `kits.build_kit` writes them as node extras
  `aurago_prop`. Helpers that are not kit props (the pirate foam ring) stay out
  of `PROPS`. The lane is `lane-<world>`, the ball
  `ball-<world>`. Props are placed by the scene modules; compose helpers keep the
  ground height function that built the terrain.
- Part pipeline (`prop.Prop.add`): `shapes.finish` (Bevel by angle, optional
  Subdivision Surface, edge densify, smooth-by-angle, Weighted Normal, all
  APPLIED) -> bake the TRS into the mesh -> `colors.paint` writes the
  `palette` gradient and `ao_w` weight -> one shared material. Colour comes from
  vertex colours; materials only differ in surface response.
- Bake (`bake.py`): Cycles AO into an `ao` POINT attribute (CPU, fixed seed and
  samples, no denoise), one-ring smoothing, then `Col = palette x lerp(1, AO, w)`
  as an 8-bit POINT `BYTE_COLOR`; scratch attributes are removed. Water, stars
  and the Earth are hidden from the bake as occluders; water and stars only get
  `Col`. The moon Earth is a Cycles-only procedural shader on a unit sphere
  (`materials.earth`, the runtime would bake a texture); sky bodies are counted
  as `backdrop_tris`, not as game geometry.
- Materials (`materials.py`): matte, satin, metal, glass, water, emissive, glow,
  foliage, cloth. Every material reads `Col` through a Color Attribute node.
  `emissive` feeds a dimmed copy to Base Color so glow stays saturated; `glow`
  keeps full lighting plus a soft emission (gems, eyes). The runtime must map
  both to emissive strength plus `COLOR_0`.
- Material contract (glTF material `extras`): `aurago_kind` on every material;
  `aurago_strength` on `emissive`/`glow`, snapped to `EMISSION_STEPS` (1.0, 1.6,
  2.5) so one prop never needs a primitive per brightness; `glow` (gems, eyes,
  water) is much softer and snaps to its own `GLOW_STEPS` (0.3, 0.7, 1.0), so pass
  glow strengths from that set; `aurago_flicker` for
  flames (`get("emissive", 1.6, flicker=0.35)`); `aurago_sway` (metres) on
  `foliage`/`cloth`. Only `foliage`, `cloth` and `glass` are double sided. Pass
  `strength`, `flicker`, `sway` through `Prop.add`.
- Kits (`kits.py`): at most 3 primitives per prop (4 for a `hero`, which is where
  the metal fittings next to wood, cloth and light are worth a fourth draw call)
  and 8 materials per world (build errors otherwise), each prop a top-level node
  `prop-<look>-<name>` at the origin with `COLOR_0` on every primitive, built with
  `seed=1`. A `hero` is moved so the centre of its smallest enclosing plan circle
  is the origin (`kits.centre_footprint`): the radius stays as small as possible
  and the placement can scale it up. Kit budgets: < 3 MiB and < 60,000 triangles
  per world, hero <= 20,000 triangles.
- Dressing (`dress.Dresser`, `courses_v1.build_course(number, course, dresser)`):
  lane, collision rails, obstacles, tee, cup and flag come from the unchanged
  `courses_v1` code; the dresser replaces `add_decor` / `add_special`. `surface-*`
  only get a POINT colour (never bevel, smoothing or `shade_smooth`), `rail-*`
  only `aurago_hidden` (no colour, saves COLOR_0 bytes); the P1 collider
  fingerprint and `bounds` (taken from a throw-away legacy build, so the physics
  out-of-bounds test never changes) stay identical. Island: `dress_island.Terrain`
  (`top_height(x, y)`: base ground, bed 6 cm under the lane within 1.1 m smoothing
  to 2.0 m, cup hollow, trench across the island for `gap` courses; a second
  stretch of lane close by never ends up below the island), three strata per
  world plus a submerged skirt (pirate, temple) or floating underside (moon), a
  `water-shore` strip with `uv.x` 0 to 1 (no shore on the moon or across a
  trench). Trims (`trim.py`): `rail-trim-left/right` share `courses_v1.rail_edge`
  with the rails, inner face flush, <= +2 cm up and <= 3 cm out, outer face reaches
  down to the ground (<= 0.5 m). Only `Dresser.bake` bakes (island, trims, lane);
  templates, obstacles, rails and shore are hidden from the occluders. The
  per-course GLB budget lives in the asset test; the island alone is about 11-12k
  triangles.
  `generate.py` dresses all 18 courses (`look`, `path`, `props` per course,
  `worlds` and `kits` at top level); `--course N` rebuilds one course and patches
  its manifest entry. Blender suffixes node and mesh names of later courses
  (`.001`), so a `--course N` GLB differs from the full-run GLB in those names:
  commit full-run output only. The manifest `path` runs to the lane end, up to 1 m
  behind the cup (the cup sits at 91 % of the lane), so the corridor also covers
  the end stub.
- Specials and placement: `dress_specials.build` turns each legacy layout special
  into fixed kit placements (gate, palms of the beach, harbour posts, neon posts,
  crystals, tower, lighthouse as hero, ...) and/or hand-built geometry
  (`special_parts`: pier beams and posts, jump ramps, bridge parapets, sun wheel with
  the four `windmill-sail` blades, moon pads, gate hub) in the place and size of the
  old part. `placement.place` then fills the island deterministically
  (`Random(1000 + number)`, 0.25 m grid): fixed placements, the hero (largest free
  circle, scale >= 0.42, towards the back), medium props (biggest first, same kind
  at least 2.2 m apart), scatter (beach critters in a shore band). Rules: the footprint
  circle (`radius` x scale) keeps `CORRIDOR` (1.2 m, the asset test needs 1.0) from the
  manifest `path` polyline except `span` props; tall props, foliage and the hero may
  overhang the rim by a share of their radius (`RIM` shares, their bounding circle is
  mostly thin parts); nothing stands in a gap trench; tall props that would hide the
  lane from the game camera and everything tall in front of the hero are skipped;
  `placement.check` repeats the asset test rules and fails the build. The dish is
  `dishbase` plus `dishhead` with one transform (the head is not based at z = 0).
  Contact shading: `Dresser.bake` links a copy of every placed prop (`occluders`,
  unbaked kit objects of the look, built once per process) into the course scene as
  occluder and removes them before the export.
- Functional parts (`dress_parts.restyle`): obstacles (bevel only, exact extents),
  ball (icosphere on exactly r = 0.16, plain white, the game tints it), putter (steel
  shaft 0 to 0.82 m, bevelled head), flag (`cloth`, double sided), hole digits
  (`emissive`), tee ring, cup rim and lining, a felt-coloured `cup-patch` that hides
  the dark ground in the stepped lane cut-out. `export.join_scenery` joins every
  other mesh per material into `scenery-<material>` nodes (`FUNCTIONAL_NAMES` /
  `FUNCTIONAL_PREFIXES` stay single nodes) and strips builder properties.
- Previews (`previews.py`, `generate.py --only previews`, about a minute): builds the
  dressed course (pirate 7, temple 2, moon 18) with the baked kit props as real
  objects, the water rings or the star/Earth backdrop and the probe lighting, and
  renders 640x360 Cycles (96 samples, OIDN, az -35, el 38, 43 degrees) to
  `ui/3d/minigolf/previews/<look>.webp` (quality 82, lowered until <= 80 KB). Cycles
  output is not bit-reproducible: previews are never part of a full run.
  `bake.commit_colours` gives passthrough objects (water, stars) their `Col`.
- Look: Khronos PBR Neutral view transform (look None, exposure 0, gamma 1) to
  match three.js `NeutralToneMapping`; keep emission strength near 1-2.5 because
  Neutral desaturates highlights above ~0.8. Camera: perspective, 43 degrees
  vertical FOV (sensor fit vertical), 35-40 degrees elevation, 1600x900.
- Dark scenes need a direct fill light (the moon uses a wide, dim `kit-fill` sun);
  world light alone leaves speckle inside concave shells in shadow.
- The probes are Cycles renders with soft shadows and mild GI. The runtime
  approximates them with baked AO, one shadow-casting sun and PMREM ambient;
  treat the probes as the colour/shape target, not as a pixel target.

## Work Guidance

- Module map: `courses_v1` (the 18 legacy courses; code moved verbatim from the
  old generator and bit-identical, guarded by the P1 collider fingerprint),
  `export` (canonical node names, `export_glb`, bounds, `join_scenery`, collider
  read-back), `dress` (per-course Dresser, lane colours, bake), `dress_island`
  (terrain, island, shore), `dress_specials` + `special_parts` (layout specials),
  `dress_parts` (functional parts), `placement` (prop placement), `occluders`
  (kit objects as bake occluders), `previews` (menu card images), `trim` (rail trims),
  `kits` (per-world prop kit GLB), `palette` (tones, gradient presets), `shapes`
  (builders, `finish`), `colors` (vertex-colour gradients), `bake`, `materials`,
  `prop` (composition),
  `island` (diorama chunk with strata, water rings), `lane` (felt, rails, cup,
  flag, ball), `compose` (placement, terrain helpers), `stage` (reset, camera,
  lights, sky, render), `props_<world>` plus `ship_pirate` / `head_temple` heroes,
  `scene_<world>` (probe compositions), `probe` (CLI).
- Keep modules small and split before they grow past a few hundred lines. New
  props go into `props_<world>.py` and `PROPS`; shared builders into `shapes.py`.
- Use `densify` (`dens=`) only where AO resolution matters (large flat faces);
  thin parts use `dens=0` or a large value to save triangles.
- Use `--prop world:name --view az,el,k` for one-prop close-ups and `--quick` for
  800x450 iteration renders.

## Verification

- `blender --background --factory-startup --python assets/minigolf/generate.py
  -- --only kits` (about 15 s) prints one `kit <look>: ... bytes ...` line per
  world; a second run must give identical GLB hashes. A full run must leave
  `git status ui/3d/minigolf` free of course GLB changes and
  `node scripts/test-minigolf-assets.mjs` green.
- Dressed courses: a full run (`generate.py` without arguments, about 2 minutes)
  rebuilds the three kits and all 18 courses; `node scripts/test-minigolf-assets.mjs`
  (budgets, P1 collider fingerprint, kits, placements), `test-minigolf-courses.mjs`
  and `test-minigolf-physics.mjs` must stay green, and a second full run must give
  identical GLB and manifest hashes. `-- --only courses --course N` (about 15 s)
  is for iteration. Reviewing a course: `--save-blend` and a scratch Cycles render
  of its scene with the baked kit props linked in as real objects.
- `blender --background --factory-startup --python assets/minigolf/kit/probe.py
  -- --out <scratch> --world all` renders the three probes (about 2.5 minutes
  each on 16 cores at 128 samples) and prints per-prop and scene triangle counts,
  bake and render times and the `scene_hash`.
- `--no-render` rebuilds and bakes only; run it twice and compare `scene_hash`.

## Child DOX Index

No child AGENTS.md files.
