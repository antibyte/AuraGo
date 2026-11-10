# System World beauty pass

Date: 2026-11-09 · Status: approved design (user, 2026-11-09). Phases 0 and 1 are detailed here. Phases 2–6 each get their own spec and plan.

This document: `plans/2026-11-09-system-world-beauty-design.md`. Phase 0 and 1 plan: `plans/2026-11-09-system-world-beauty-phase-0-1.md`.

## Goal

System World is the 3D city inside AuraGo's virtual desktop. The goal is to make it as beautiful as possible: much more detail, better models, a stronger ambience in environment and sound, buildings that can be entered, residents whose feet stay on the ground, and windows that show a life behind them. Existing models are reworked in Blender and new models are created where needed. The asset limits may be raised.

The art direction stays the same: a cinematic data metropolis in graphite, titanium and bronze, with warm windows and cyan accents. The style is kept and the detail is richer.

## Scope of files

| Area | Location |
| --- | --- |
| Desktop app | `ui/js/desktop/apps/sysworld*.js` |
| Bundle | `ui/js/vendor/system-world/city.esm.js`, built by `scripts/build-system-world.js` |
| Blender kits (current) | `ui/3d/system-world/v1`, `ui/3d/system-world/v2` |
| Authoring sources | `assets/system-world/` |

## Findings

These were verified in code on 2026-11-09.

1. **Ground height is a step function.** `surfaces` in `sysworld-layout.js` is `{ground: 0, road: .12, pavement: .5, quay: .08, room: .16, gallery: 4.16}`. `streetHeight` returns road within 3.3 m of a street centre line and pavement from 3.3 m to 6.0 m. Plots are at 0. The pavement is therefore a 0.5 m high strip with a vertical 0.38 m kerb on the road side and a 0.5 m drop on the plot side.
2. **The resident route graph ignores the kerb.** `createWayfinder` in `sysworld-places.js` puts its nodes exactly on the outer pavement edge (centre line ± 6 m). It links any two nodes closer than 48 m with a straight line. Routes therefore cut across roads and plots anywhere.
3. **Feet are not adapted.** Walker height is assigned per frame from the floor function, without smoothing. There is no foot adaptation.
4. **The companion and the camera do not follow the ground.** The Spot companion never queries the ground and uses a fixed quay height of 0.08. The visitor camera snaps the same way.
5. **Street models are stretched.** The v1 street models are scaled along their length to fill each street segment.
6. **Buildings are solid.** The seven district buildings are solid GLB blocks without interiors. Their windows are emissive quads. The kit glass is opaque by design. Only the three waterfront pavilions and the sky deck can be entered.
7. **Sound is synthesised only.** All sound is made with Web Audio. There are no interior acoustics, no continuous city or traffic bed, and no limiter on the master bus.
8. **The model directory is nearly full.** `ui/3d/system-world` uses about 11.8 MiB of its 12 MiB budget.
9. **Provenance breaks on CRLF.** Asset provenance hashes are computed over raw bytes. A CRLF checkout of `assets/system-world/*.py` breaks `scripts/test-system-world-expansion.mjs`.

## Principles

- **Art direction stays.** The city keeps its cinematic data-metropolis look. Detail grows, the palette does not change.
- **New kit.** All new and reworked models live in a new kit, `v3`. v1 and v2 files that v3 supersedes are removed from the runtime payload in the phase that replaces them.
- **One source of truth for heights.** Model geometry and walking logic read the same height values. No placement that stands on a road or pavement uses a hard-coded height.
- **Every phase ends working.** Each phase ends in a working, tested, committed state. Each phase has before and after pictures from the survey harness.
- **Work split.** All Blender authoring is done by the main session. Code, tests and docs are delegated to the agent roster: plan, reader, maker, test, docs, security and debug agents.
- **Branch and release.** Work happens on branch `feat/sysworld-beauty` in the worktree `.worktrees/sysworld-beauty`. Nothing is pushed or deployed without asking the user first. Other sessions commit to `main`, so the branch is rebased before each merge.

## Roadmap

| Phase | Scope | Spec |
| --- | --- | --- |
| 0 Foundation | v3 pipeline (meshopt compression, baked vertex ambient occlusion), raised budgets, line-ending-safe provenance, survey harness with baseline pictures | this document |
| 1 Ground and gait | New street, crossing and plot models; realistic kerbs with dropped kerbs and zebra crossings; one height profile; pedestrian routes on pavements and crossings; damped walker height, foot adaptation, contact shadows; companion and visitor follow the ground | this document |
| 2 Windows with life | Real glass with reflections; interior-mapped rooms behind upper-floor and skyline windows (furniture, screens, moving robot silhouettes, lighting by time of day); the seven district buildings and both skyline towers reworked with far more facade detail | `plans/2026-11-09-system-world-beauty-phase-2.md` |
| 3 Enterable buildings | Six district buildings get a real ground floor to walk into (themed lobby, glass front, door, furniture, working robots visible from outside). The small operations beacon stays exterior-only. Interiors load on approach | `plans/2026-11-10-system-world-beauty-phase-3.md` |
| 4 City life (implemented) | Trees and beds with wind, bird flocks, pigeons, boats, street furniture, kiosks, neon signs, steam, fireflies at night | `plans/2026-11-10-system-world-beauty-phase-4.md` |
| 5 Soundscape (implemented) | Master limiter, sound zones (harbour, park, street canyon, each interior), per-space reverb, damping behind glass, continuous traffic and tram bed, footsteps per surface | `plans/2026-11-10-system-world-beauty-phase-5.md` |
| 6 Finish | Night lighting, per-tier polish, performance, final acceptance | own spec and plan |

Recommendation for phase 5: add real CC0 recordings for birds, water, wind and city hum next to the synthesis. This changes the current rule that nothing is sampled. It needs the user's go-ahead before any download. The list of recordings is decided in phase 5's spec.

## Budgets

| Budget | Old | New |
| --- | --- | --- |
| Runtime model directory `ui/3d/system-world` | 12 MiB | 40 MiB |
| First display (bytes loaded before the city is shown) | 12 MiB | 20 MiB |
| Whole app | 48 MiB | 80 MiB |

The v1 kit budget (8 MiB) and the surface texture budget (1.5 MiB) do not change.

The budget measures are the existing ones. "First display" is bytes loaded before the city is first shown, reported as `loadedBytes` in the browser test. "Whole app" is the existing measure in `scripts/test-system-world-expansion.mjs`. Only the limits change.

Actual use is reported at the end of every phase.

Phase 2 actual use: the model directory `ui/3d/system-world` is 11,101,682 bytes (about 10.6 MiB) of its 40 MiB limit. First display and whole-app figures are measured in the final regression of phase 2.

Phase 3 actual use: the model directory `ui/3d/system-world` is 12,200,238 bytes (about 11.6 MiB) of its 40 MiB limit. First display and whole-app figures are measured in the final regression of phase 3.

Known places that pin the old numbers:

| File | Role |
| --- | --- |
| `assets/system-world/check_assets.py` | asset budget check |
| `assets/system-world/build_expansion.py` | expansion build and provenance |
| `scripts/test-system-world-expansion.mjs` | expansion contract test |
| `scripts/test-system-world-companion.mjs` | companion contract test |
| `scripts/build-system-world-spot.mjs` | Spot build script |
| `ui/desktop_sysworld_expansion_browser_test.go` | browser test |
| `assets/system-world/README.md` | authoring readme |
| `assets/system-world/AGENTS.md` | agent notes for the authoring folder |

## Phase 0 — Foundation

Phase 0 builds the pipeline, the budgets and the survey harness that the later phases rely on. The first shipped v3 assets are the phase 1 street models.

### Kit v3

- The runtime payload lives in `ui/3d/system-world/v3/`.
- Files: GLBs named `<id>.lod<0-2>.glb`, `manifest.json`, `LICENSE.txt`.
- Each design has three LODs, all authored in Blender by the generators with per-LOD detail parameters. gltfpack never simplifies. LOD selection keeps the existing tier logic of the renderer.
- Units: metres. Up axis: Y. Origins: base-centred.
- No image textures. No external URIs.
- License: MIT.

### Authoring

| Item | Path or command |
| --- | --- |
| Shared builder (materials, mesh helpers, LOD handling, ambient-occlusion bake, export with stable node names) | `assets/system-world/kit3.py` |
| One module per asset family (first: street assets) | `assets/system-world/street_assets3.py` |
| Entry point | `assets/system-world/build_world3.py` |
| Run command | `& 'D:/Blender 5.2/blender.exe' --background --factory-startup --python assets/system-world/build_world3.py` |
| Raw output (git-ignored) | `disposable/system-world/v3-raw/` (uncompressed GLBs and a raw manifest) |
| Source scene | `assets/system-world/production/aurago-world-3.blend` |

### Packing

`scripts/build-system-world-v3.mjs` does the following:

1. Runs gltfpack on every raw GLB with meshopt compression and no simplification. The flags are `-cc -kn -km -ke -kv`. These keep node names, materials, extras and all vertex attributes.
2. Validates each result with the Khronos validator.
3. Writes the payload and the final manifest.

`--check` verifies the committed payload against its manifest. When the raw directory exists, it also checks that repacking reproduces the payload byte for byte.

### Manifest schema 3

| Field | Value or meaning |
| --- | --- |
| `schema_version` | `3` |
| `version` | starts at `3.0.0` |
| `generator` | the v3 pipeline |
| `license` | MIT |
| `units` | metres |
| `up_axis` | Y |
| `compression` | `EXT_meshopt_compression` |
| `sources` | sha256 per v3 authoring file, over LF-normalised bytes: `kit3.py`, `build_world3.py` and each asset family module (`street_assets3.py` first) |
| `assets[]` | each entry has `id`, `name`, `category`, `lods[]`, `components`, `navigation`, `animations` |
| `lods[]` | each entry has `level`, `file`, `bytes`, `sha256`, `triangles`, `bounds` |
| `total_model_bytes` | sum of the payload |

### Baked ambient occlusion

- Every v3 mesh carries `COLOR_0`.
- The values come from deterministic ray-cast ambient occlusion.
- The values are greyscale and linear.
- The renderer multiplies the albedo by them.
- v1 and v2 models are unaffected.

### Renderer

- The kit loader accepts v3 files.
- It registers the meshopt decoder once. The companion already ships this decoder in the bundle.
- v3 materials enable vertex colours.
- v3 materials keep the projected surface maps from `sysworld-surfaces.js`.

### Provenance

- Source hashes are computed over LF-normalised bytes. This is done in `build_expansion.py`, in the v3 pipeline and in the tests.
- `.gitattributes` pins `assets/system-world/*.py` to `eol=lf`.

### Survey harness

- `scripts/system-world-survey.mjs` renders the real city in headless Chrome.
- It reads a scene list from `scripts/system-world-survey.scenes.json`. The schema is documented at the top of `scripts/system-world-survey.mjs`. A scene can set a free camera, time of day, weather, or a follow-a-resident sequence. Environment is day, evening, night or local. Weather is clear, rain or fog.
- Output goes to `reports/system-world-survey/<label>/`.
- The harness builds a debug bundle in memory. It never changes shipped files.
- A baseline set is captured before phase 1 changes anything visible.

## Phase 1 — Ground and gait

### Height values

| Surface | Old | New | Note |
| --- | --- | --- | --- |
| `surfaces.pavement` | 0.5 | 0 | flush with the plots |
| `surfaces.road` | 0.12 | -0.14 | kerb rise 0.14 m |
| `ground`, `quay`, `room`, `gallery` | unchanged | unchanged | everything on plots keeps its height |

Tram stops keep the existing v2 `station` model, which now stands on the flush pavement (platform top 0.30 m). The step from platform to tram floor grows from 0.25 m to 0.49 m. This is accepted until trams and stops are reworked in phase 4.

### Street cross-section

Offset is the distance from the street centre line. The section is symmetric.

| Offset (m) | Surface | Height (m) |
| --- | --- | --- |
| 0 – 3.00 | carriageway, centre dashes | -0.14 |
| 3.00 – 3.30 | gutter strip (stone setts, drains) | -0.14 |
| 3.30 – 3.44 | splayed kerb, 45 degrees | -0.14 rising to 0 |
| 3.44 – 3.58 | kerb stone top | 0 |
| 3.58 – 6.00 | pavement | 0 |

At the island edge every street ends in a `street-end` cap. See Models and Street assembly below.

### Dropped kerbs and crossings

- **Dropped kerb.** At every pedestrian crossing the kerb ramp is stretched from a 0.14 m run to a 1.2 m run. It runs from offset 3.30 m to 4.50 m. Inside the dropped kerb the ramp surface replaces the kerb face and the kerb top.
- **Opening.** The opening is 2.4 m wide, that is |x| ≤ 1.2 m from the crossing axis.
- **Flares.** The 0.8 m flares lie outside the opening on both sides, 1.2 to 2.0 m from the crossing axis. The run changes linearly across them. The opening plus the flares fill exactly one 4 m module.
- **Tactile paving.** A tactile paving strip, from offset 4.50 m to 4.90 m, marks the top of the ramp. Pavement lies beyond it. In the flares the kerb top strip tapers to zero width.
- **Zebra crossing.** Each street arm has a pedestrian crossing (zebra) 8 m from the intersection centre. This is the `street-zebra` module placed directly next to the intersection.
- **Corners.** Intersection corners have a rounded kerb with a 1.2 m radius.

### Height profile

```
height = road + rise * clamp(d / run, 0, 1)
```

- `d` is the distance from the kerb line at offset 3.30 m.
- `run` is 0.14 m, or up to 1.2 m inside a dropped kerb.
- The profile is continuous everywhere on streets.

### Models (kit v3, authored in Blender)

| Model | Length or size | Content |
| --- | --- | --- |
| `street-plain` | 4 m | standard street module |
| `street-drain` | 4 m | gutter drain grates and a manhole |
| `street-zebra` | 4 m | zebra crossing, dropped kerbs, tactile paving |
| `street-fill` | 1 m | fill module |
| `street-crossing` | 12 × 12 m | intersection with rounded corner pavements |
| `street-end` | 0.5 m deep, 12 m wide | island edge cap: kerb block across the carriageway (top at 0, inner face splayed over 0.14 m) with an outer wall across the full street width down to the island body |

- Streets are assembled by repeating fixed-length modules. A single tile is no longer stretched.
- The single island ground box becomes an island body. Its top is at -0.2 m. It has a dark road-like top face that only shows through sub-pixel seams between street modules.
- 25 plot slabs sit between the street bands, with their top at exactly 0. One quay base slab (x -85..85, z 74..80) has its top at exactly 0.
- The `street-end` outer wall reaches down to -0.62 m.
- Only the v1 `street-tile` and v1 `street-crossing` leave the runtime payload in phase 1. No v2 file leaves in phases 0 and 1.

### Street assembly

The assembly rule applies to every street arm:

1. A `street-zebra` module sits directly next to every intersection on each arm.
2. A `street-end` cap sits at each island edge.
3. The rest is filled with 4 m modules: every third one is `street-drain`, the others are `street-plain`.
4. The remainder of 0 to 3 m is filled with 1 m `street-fill` modules.

Streets along world Z (the `xs` lines) end at the quay edge (z = 74) with a `street-end` cap. `streets` gains `quayZ: 74`. The quay strip (v2 quay tiles at 0.08 m, lanterns) stays continuous across them.

### Code

- **New file `sysworld-ground.js`.** It is pure and shared by the renderer and the tests. It contains:
  - the kerb constants;
  - the list of street modules and zebra crossings, derived from `streets`;
  - the continuous street height profile;
  - the pedestrian graph.
- `groundHeight` in `sysworld-exploration.js` builds on `sysworld-ground.js`.
- Every placement that stands on a road or pavement derives its height from `surfaces`. No hard-coded numbers remain.

### Pedestrian routes

- Graph nodes sit on the pavement centre line, at offset 4.75 m, on both sides of every street.
- Edges run along pavements, around corners and across zebra crossings only.
- Plots and pavements are freely walkable.
- A route never crosses a carriageway outside a zebra crossing.

### Footing

- **Damped height.** Walker height follows the ground through a damped spring. It no longer snaps.
- **Foot adaptation.** A two-bone foot adaptation keeps each foot of the three resident robot types on the ground profile.
  - Stance feet are planted.
  - The pelvis lowers so the lower foot still reaches the ground.
- **Pitch.** The body pitches slightly on ramps.
- **Blending.** Foot adaptation is blended by stance weight. It runs at medium and above. The quality tiers are the existing ones (low, medium, high, ultra) defined in `sysworld-scene.js`.

### Contact shadows, companion and camera

- **Contact shadows.** A soft blob shadow sits under each resident, under the companion, and under the ground projection of each hover robot.
- **Companion.** The Spot companion follows the ground height and slope. Its patrol loop is moved east of the x = 18 street so that it stays on one level and never crosses a carriageway. The companion takes height and pitch from the ground.
- **Visitor camera.** The visitor camera follows the ground with the same damping as the walker.

### Materials and renderer

- New v3 material names for the street kit: `city.kerb`, `city.setts`, `city.iron`, `city.marking`, `city.tactile`.
- `sysworld-surfaces.js` gains surface kinds for these names. Existing names (`city.road`, `city.stone`, ...) keep their values.
- Shared kit materials are keyed by material name plus vertex-colour presence. v1 and v2 meshes without `COLOR_0` never share a vertex-coloured material.

### Baseline finding

The first survey run (2026-11-09) showed the following:

- pavements read as 0.5 m concrete blocks;
- residents walk on the carriageway beside the pavements;
- the street-lamp light cones render as solid bright cones in daylight.

The light cones were fixed in phase 2: `lampLevel` in `sysworld-atmosphere.js` turns them off by day and fades them in towards dusk.

## Testing

Ground checks are node tests. The existing browser run is extended where needed. Phase 1 adds no new browser test.

### New tests

`scripts/test-system-world-ground.mjs` checks:

| Check | Requirement |
| --- | --- |
| Profile continuity | no step larger than 2 cm per 5 cm of travel outside stairs, on all streets |
| Model match | exported v3 street models match the analytic profile within the limits below, when ray-cast from above |
| Pedestrian edges | every pedestrian graph edge stays off the carriageway, unless it is a zebra edge |
| Module placement | street modules fill every street segment without gaps or overlaps |

Model match limits are 15 mm (LOD 0), 20 mm (LOD 1) and 30 mm (LOD 2). Measured in Blender on 2026-11-09, the street models deviate from the height profile by at most 12 mm (LOD 0), 15 mm (LOD 1) and 24 mm (LOD 2). Two choices make this possible: the flare columns of the dropped kerb are spaced at a constant kerb-run ratio, and the gutter drain pit is only 12 mm deep.

Footing unit tests check:

- spring response;
- foot planting on a ramp and on a kerb;
- no foot below the ground.

### Existing suites

These suites stay green, with updated numbers:

| Command | Purpose |
| --- | --- |
| `scripts/test-system-world*.mjs` | system world contract tests |
| `python assets/system-world/check_assets.py` | asset budget and provenance check |
| `node scripts/build-system-world.js --check` | bundle check |
| `AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_SYSTEM_WORLD_MATRIX=1 AURAGO_SYSTEM_WORLD_EXPANSION=1 go test ./ui -run '^TestDesktopAuroraBrowser$'` | browser run |

### Visual acceptance

Survey pictures are taken before and after the phase. They cover:

- a kerb close-up;
- a crossing;
- the feet of residents over a sequence of steps;
- day;
- night;
- rain.

## Out of scope for phases 0 and 1

- Facade rework.
- Glass.
- Interiors.
- Vegetation.
- Sound.
- Changes to the district data model, the HUD or the APIs.

## Risks

| Risk | Handling |
| --- | --- |
| Height changes ripple into placements, tram, stations and tests | an audit of every dependency precedes the change |
| gltfpack quantisation moves vertices by fractions of a millimetre | the contract test tolerance covers it |
| Foot adaptation on the baked walk clips can look stiff | blended by stance weight; runs at medium and above |
| Other sessions commit to `main` | the branch is rebased before each merge |
| GitNexus impact results are unreliable for the worktree: the index does not cover it. Impact analysis on the main-checkout index (181 commits behind) rates `streetHeight` and `groundHeight` CRITICAL because of name-collision fan-out into unrelated apps | their direct callers are 1 and 7, all inside System World, and are covered by the height-dependency audit. `createWayfinder` rates LOW. Change detection before merge runs against main |

## Next steps

1. Phase 0 and 1 plan `plans/2026-11-09-system-world-beauty-phase-0-1.md`, written by the plan agent from `plans/2026-11-09-system-world-beauty-design.md`.
2. Phase 0 implementation: Blender pipeline by the main session; code, tests and docs by the roster agents.
3. Baseline survey pictures, captured before phase 1 changes anything visible.
4. Phase 1 plan, written after the dependency audit of the height change.
5. Specs and plans for phases 2–6, each written when its turn comes.
