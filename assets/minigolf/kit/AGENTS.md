# AuraGolf procedural kit

## Purpose

Reusable Blender 5.2 modules for the stylized AuraGolf diorama look: palettes,
soft shape builders, vertex-colour gradients, baked ambient occlusion, per-world
props, lane sections and the style-probe renders (P2 "Look & Welt-Requisiten").
It is the modular generator behind `../generate.py` (a slim CLI: `--only
kits,courses`, `--save-blend`): `courses_v1` and `export` hold the legacy
course code moved verbatim, `kits` exports one prop kit GLB per world. The
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
  2.5) so one prop never needs a primitive per brightness; `aurago_flicker` for
  flames (`get("emissive", 1.6, flicker=0.35)`); `aurago_sway` (metres) on
  `foliage`/`cloth`. Only `foliage`, `cloth` and `glass` are double sided. Pass
  `strength`, `flicker`, `sway` through `Prop.add`.
- Kits (`kits.py`): at most 3 primitives per prop and 8 materials per world
  (build errors otherwise), each prop a top-level node `prop-<look>-<name>` at
  the origin with `COLOR_0` on every primitive, built with `seed=1`. Kit
  budgets: < 3 MiB and < 60,000 triangles per world, hero <= 16,000 triangles.
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
  `export` (canonical node names, `export_glb`, bounds, collider read-back),
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
- `blender --background --factory-startup --python assets/minigolf/kit/probe.py
  -- --out <scratch> --world all` renders the three probes (about 2.5 minutes
  each on 16 cores at 128 samples) and prints per-prop and scene triangle counts,
  bake and render times and the `scene_hash`.
- `--no-render` rebuilds and bakes only; run it twice and compare `scene_hash`.

## Child DOX Index

No child AGENTS.md files.
