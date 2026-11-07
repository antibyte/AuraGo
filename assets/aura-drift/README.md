# Aura Drift model kit

Blender 5.2 generator for the Aura Drift model pack (`ui/3d/aura-drift/v1`): eight cars, items,
track pieces, animals, shared props and eight scenery kits. Rebuilt offline from procedural sources
and retained reviewed vertex-colour geometry, MIT licensed, no runtime textures.
Contract: section 3 of `plans/2026-11-06-aura-drift-3d-overhaul.md`.
Art direction: an original stylised **toy diorama at golden hour**: chunky low poly, glossy paint,
bevelled silhouettes, flat-shaded facets, oversized wheels.

Softeis uses the reviewed NC AI reconstruction of an original Aura Drift ImageGen concept.
`kit/softeis_mesh.json.gz` retains the body/cone geometry, normalized vertex colours and pivots;
`kit/car_softeis.py` rebuilds it with the native six-spoke wheels, seven body material groups and
light/exhaust/weapon anchors. Its colours work with all four liveries. The roof cone and wheel
nodes are driven by the existing game physics; no baked animation or external API is needed.

The other seven vehicles and twenty selected environment objects use the same reviewed concept
workflow. `reviewed/sources.json` records each concept/reconstruction hash and request ID;
`reviewed/<group>.json.gz` retains texture-free mesh data. `kit/reviewed_assets.py` replaces listed
meshes after the procedural build, preserving the native scene hierarchy and animated audience.
Cars retain circular native wheels, livery materials and light anchors. The mower's driver/blade,
sofa's lamp/cushion and desert windmill rotor have independent joint origins. Window shells are
opaque because these reconstructions contain no interior. Colour boundaries are locally refined
to preserve the artwork without textures; the whole pack still has an 18 MiB limit.

Selected scenery replacements (large, frequently visible silhouettes):

| Kit | Reviewed objects |
| --- | --- |
| shared | grandstand, gnome statue, hotdog stand, camper van |
| alpine | chalet, cablecar pylon |
| desert | water tower, windmill pump |
| harbour | lighthouse, tugboat |
| salt | UFO, rover |
| forest | watermill, hunting hide |
| industry | cooling tower, chemical tank |
| glacier | research station, snowcat |
| metro | food truck, metro entrance |

The grandstand keeps its native interior seating so its animated spectators remain seated correctly.
Run `python assets/aura-drift/reviewed/check.py` before regenerating to verify all 27 retained inputs.

## Commands

```
"D:\Blender 5.2\blender.exe" --background --factory-startup --python <abs>/assets/aura-drift/generate.py -- \
    --only cars|items|track|animals|shared|scenery-alpine,...|all|manifest [--out <abs dir>] [--preview <abs scratch dir>]
node scripts/test-aura-drift-assets.mjs        # AURAGO_AURA_DRIFT_ASSETS_PARTIAL=1 skips the exact-file-set check
```

- Always pass absolute paths (relative ones land in `C:\disposable`).
- `--only` is a comma list. `cars` builds every `kit/car_<id>.py` that exists; a group whose module is not
  written yet is skipped with a message. `--only car-kobalt` builds one car.
- `--only manifest` builds nothing: it rescans the GLBs in `--out` and rewrites `manifest.json` and
  `LICENSE.txt`. Authors never edit the manifest; per-piece metadata (tags, radius overrides) lives in
  `kit/meta_<group>.py` (see the docstring of `kit/manifest.py`).
- `--preview <dir>` renders six EEVEE 1024x768 views per model (hero, chase, side, front, top, rear) with
  warm key sun, cool fill and a livery tint (`paint` red, `paint_accent` cream). **Preview directories
  stay outside the repo**, e.g. `D:\tmp\agdrift-shots\kit`. Then look at the PNGs with the Read tool.
- Build into a scratch `--out` while iterating (`D:\tmp\agdrift-build`), the default `--out` is the shipped
  pack. Output is deterministic: two runs give equal SHA-256.

## Modules (`kit/`)

| Module | Role |
| --- | --- |
| `core.py` | `Accum` (verts/faces/vertex colours per material slot), `Model` (objects of one GLB), `to_blender`, `hex_linear`, `rng_for`, `clear_scene`, `DEFAULT_OUT` |
| `shapes.py` | pure geometry: boxes, bevels, revolves, lofts, `Hull`, `arc_band`, `airfoil`, `spline`, ... |
| `materials.py` | `make_material(name, base, metallic, roughness, emission, strength, alpha)`; the car set |
| `export.py` | `export_glb`, `render_previews`, GLB facts |
| `manifest.py` | scans GLBs, writes `manifest.json` + `LICENSE.txt` |
| `cars_common.py` | `CarBuilder`, `wheel_accum`: everything a car needs |
| `reviewed_assets.py` | Offline mesh replacements, circular wheels and rigid-part pivots from `../reviewed/` |
| `car_vektor.py` | the reference car, read it first |
| `polish_props.py` | Six extra trackside props: `balloon_bunch` and `photo_tower` (shared, return `toy.Parts`), `trail_marker`, `sunshade`, `signal_mast` and `holo_arrow` (fill a scenery `Piece`) |
| `weather_props.py` | Six weather props: `windsock` and `rain_shelter` (shared, return `toy.Parts`), `snow_gauge` (alpine), `anemometer` (desert), `storm_lantern` (harbour) and `snow_fence` (glacier), each filling a scenery `Piece` |
| `generate.py` (parent dir) | CLI. Group modules expose `build() -> core.Model` (one model = one GLB, named like the file stem) |

## Frames

Author in the glTF frame: **Y up, metres, nose +X, right side +Z (left is -Z)**, origin at ground centre,
wheels resting on y=0. `core.to_blender` converts when a mesh is created, so the exporter returns exactly
what you wrote. Props: base at y=0, footprint centred, front toward +X.

## Shapes (`kit/shapes.py`)

A *shape* is `(verts, faces)`, a closed outward-facing solid. Place it with `Accum.add(shape, matrix, tint, faces)`
and `xf(pos, rot_degrees_xyz, scale)`.

| Helper | Use |
| --- | --- |
| `box(sx, sy, sz, bevel, segs)` | centred box, `segs=1` chamfer, `2` chunky round. Cost: about 12 triangles plain, 44 with `segs=1`. Skip bevels on small hidden parts |
| `taper_box(length, w0, h0, w1, h1, dy, bevel, segs)` | box along X with different end sections: splitters, skirts, wedges |
| `cylinder_x/_z/(r0, r1, length, seg, chamfer)`, `sphere`, `ellipsoid(rx, ry, rz)`, `torus` | rounds; use 8 to 12 segments, faceting is the style |
| `revolve / revolve_x / revolve_z(profile, seg, closed)` | (radius, axial) profile about Y / X / Z. A profile that starts and ends on the axis (radius 0) is a closed solid, `closed=True` makes a ring (tyre) |
| `extrude_z(polygon_xy, z0, z1, bevel)` | side-profile solids (end plates, fender blanks) |
| `arc_band(r_in, r_out, a0, a1, z0, z1, seg, bevel)` | ring sector around the origin, angle 90 is up: wheel arches |
| `airfoil(chord, thick, span, bevel)` | wing section along Z, tilt it with `xf(rot=(0, 0, -aoa))` |
| `strut(p, q, radius, sides)` | capped prism between two points (pillars, pylons, rails) |
| `loft(rings)` | skin equal-sized point rings |
| `annulus_z(r0, r1, z, seg)` | flat single-sided ring (decals, stripes) |
| `mirror_z(shape)`, `merge(shapes)`, `spline(keys)` | mirror to the left side, join, smooth monotone curve through `(x, value)` keys |

### Lofted hulls with material bands

`Hull(stations, m=6)` lofts rounded-rectangle cross sections (`ring`: `x, hw, yb, yt, power, top_flat, bot_flat`)
and keeps the face layout, so parts of the **same surface** can take other materials:

```python
hull = Hull([{"x": x, "hw": HW(x), "yb": BOT(x), "yt": TOP(x), "power": 3.4, "top_flat": 0.2} for x in XS], m=6)
hood  = hull.band([hull.TOP], *hull.span(0.55, 2.2))        # flat top panel between two x positions
under = hull.band([3, 4, hull.BOTTOM])                     # lower flank + underside
car.add("paint_accent", hull.shape, faces=hull.pick(hood))
car.add("trim",         hull.shape, faces=hull.pick(under))
car.add("paint",        hull.shape, faces=hull.rest(hood, under))
```

Ring positions with `m=6`: `TOP` (flat top panel), `0`/`1` shoulder, `2` flank panel, `3`/`4` lower flank, `BOTTOM`.
`hull.point(i, k)` returns a surface vertex (pillars: `strut(hull.point(i, 1), hull.point(i + 1, 1), 0.03)`).
Flat top panels of width `2 * top_flat` make clean racing stripes. Drive `hw/yb/yt` with `spline(...)`:
sample the same functions elsewhere (`TOP(x)`) to put details flush on the surface.

## Cars (`kit/cars_common.py`)

```python
car = CarBuilder("kobalt", wheel_radius=0.38, wheel_width=0.34, wheelbase=2.7, track=0.78, hub=hex_linear("#ffe066"))
car.add("paint", shape, xf(pos))                 # one shape, any body material
car.pair("trim", shape, (x, y, +z), rot=None)    # shape on the right (+z) side and mirrored on the left
car.lamp("head" | "brake", pos, (depth, height, width))   # bezel + emissive lens, both sides
car.exhaust(x, y, z)                             # chrome pipes + bore + anchor_exhaust_l/_r
car.wing(x, y, span, deck_y=...)                 # airfoil, end plates, pylons
car.mirror_housing((x, y, z)); car.driver((x, y, z))
car.anchor("anchor_weapon", (x, y, z)); car.anchor_pair("anchor_head", (x, y, z>0))
model = car.finish()                             # body + 4 wheels + 7 anchors, returns core.Model
```

Body materials (exact names, at most 7): `paint` and `paint_accent` (white base; the runtime tints them,
the preview uses red and cream), `trim` (dark plastic), `glass` (alpha 0.62), `chrome`, `head_light` and
`brake_light` (emissive). Wheels use only `tyre`; rim, tread, stripe and hub tones come from vertex colour.
`finish()` bakes a height-based ambient occlusion into `paint`, `paint_accent` and `trim` vertex colours
(`default_ao`, pass `ao=` to change it) and prints per-material triangle counts, use them to find fat parts.

Wheels (`wheel_accum`, about 1100 triangles): rounded tyre, chevron lugs, cream sidewall stripe, dished rim,
five spokes, hub cap in the car's accent colour, lug nuts. `finish()` builds the right wheel and its mirror
once and instances them for the rear axle (the GLB stores two meshes). Tweak with `car.wheel_kwargs =
{"spokes": 6, "tread_blocks": 15}`. Wheel nodes: origin at the wheel centre, axle along Z.

Budgets: at most 30000 triangles and 1.25 MiB per car. Reviewed cars retain extra geometry at painted
colour boundaries; reuse wheel meshes and keep small hidden details simple.

### Writing a new car

1. Copy `car_vektor.py`, set `CAR_ID`, build with `CarBuilder`. The module must expose `build()`.
2. Required nodes come from `finish()`; optional animated children (`cone`, `blade`, `gnome`, `lamp`, `cushion`)
   are separate objects parented to `body`: create them with `model.mesh(name, parts, parent=body_obj)` after
   `finish()` (and keep their pivots at the joint).
3. `python generate.py -- --only car-<id> --out <scratch> --preview <scratch shots>`, read the PNGs, iterate.
4. Per-car manifest data is measured from the GLB (length, width, height, wheelRadius, wheelbase); override in
   `kit/meta_cars.py` only when needed.

## Vertex colours and materials (why things look the way they do)

- `core.Model.mesh` always writes the float attribute `Col` (POINT domain, separate verts per shape) and
  marks it as the active render colour, so the exporter writes `COLOR_0` only.
- `materials.make_material` keeps the colour in the Principled socket's *default value* (the exporter reads
  `baseColorFactor` from there when images are off) **and** links `Col` through a Mix multiply node, so
  Blender previews exactly what three.js will show.
- Export options are pinned in `export.export_glb` (`export_all_vertex_colors` off, otherwise a second
  `COLOR_1` doubles the file size).
- Back faces are culled: closed solids with outward normals, which the helpers guarantee.

## Previews

`render_previews` lives in `kit/export.py`: EEVEE, gradient golden-hour world (reflections give the paint its
gloss), a warm key sun, cool fill, rim, a cream ground with soft shadows, livery tint. Add views with
`(name, azimuth, elevation, fill)`; azimuth 0 is in front of the nose, 90 the right side, 180 behind.

## Pack and status

- `ui/3d/aura-drift/v1/` holds 20 GLBs (eight cars, `items`, `track`, `animals`, `shared`, eight
  biome scenery kits): 9 179 600 bytes (8.75 MiB) in total, including the six polish and six
  weather props above.
  `manifest.json` records each file's size, hash and car measurements, and `LICENSE.txt` the licence.
  `shared.glb` may use up to 1.75 MiB; all other per-file budgets stay as the validator lists them.
- The sound pack is generated from `scripts/build-aura-drift-audio.mjs` into `ui/3d/aura-drift/audio/`;
  see [`audio/README.md`](audio/README.md). Engines, music and most groups are own synthesis; 17 sfx groups are hash-pinned CC0 recordings.
- Visual acceptance on a real screen is still pending. Use the survey and the dev harness
  (`node scripts/aura-drift-dev-server.mjs`, then `/harness`) described in `documentation/aura-drift.md`.
