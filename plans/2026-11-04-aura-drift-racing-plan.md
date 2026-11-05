# Aura Drift — top-down racing for the Virtual Desktop

Status: **phases 0–2 and the craft slice of phase 3 implemented** (2026-11-04/05, quality
pass 2026-11-05). The game is a playable first-party Virtual Desktop app with WebGL2
post-processing, measured AI balance and a browser acceptance matrix. Remaining phase-3/4
items are listed below. Baseline: working tree inspected on 2026-11-04; reconfirm affected
sources before further work.

## Implementation status

| Plan area | Delivered | Still open |
| --- | --- | --- |
| Platform decision | First-party modular Desktop app (`ui/js/desktop/apps/aura-drift-*.js`), catalog entry, shell registration, 16 locales, icon, DE/EN manuals, operator guide | Game Maker Studio phase-0 feel lab was not needed as a separate prototype; the deterministic harness served that purpose |
| Physics | Bicycle model at fixed 120 Hz, seeded RNG, surfaces, wall/car collisions, replay-hash and 30/144 FPS equivalence tests; braking holds at standstill (no slip-sign chatter), held brake engages a bounded reverse, player steering flips while rolling backwards, drift-refilled nitro resource | Engineering-grade tyre data, measured handling sessions on reference hardware |
| Tracks | Six distinct validated spline layouts, 320 m chunk streaming with LRU caps, seeded daily generator with drivability validator, minimap | Track editor, branch shortcuts, moving gates |
| Graphics | Canvas 2D world with procedural chunks, **world-anchored road markings** (kerbs, dashed centre line, worn racing-line bands, checkered start/finish that never animate with frame time), decals over the road surface, procedural cars with steering front wheels, night headlight cones/lamp posts, particles, weather, vignette, speed streaks, four quality tiers plus automatic selection, reduced motion; **WebGL2 High/Ultra post pass** (bloom, speed streaks, wet reflections, grade, impact aberration, grain, NaN-guarded, context-loss fallback) | WebGL2 scene geometry (the plan's optional Three.js path), reference-hardware screenshot review |
| Audio | Fully synthesized layered engine/gear-shift/backfire/tyre-slip/surface/wind/SFX, adaptive kick/bass/arp/pad music per biome scale and tempo, separate buses, master compressor, mute/volume/focus/dispose | Offline capture with clipping analysis, fully authored per-biome themes |
| Weapons | Six counterable weapons, pickups, hazards, projectiles, AI use, weapon-free mode | Formal human balance sessions |
| AI / balance | Grip- and corridor-aware corner speeds, bounded disclosed 6% catch-up, nitro spent only on clean straights, two-stage recovery (brief reverse, deterministic marshal respawn); `scripts/test-aura-drift-balance.mjs` measures six seeded AI-only laps (89–117 s), finishers, wall contacts, weapon events and reproducibility | Handling sessions on reference hardware |
| Modes | Quick race, Time trial with input-tape ghost, Cup, Daily route, local bests, track-driven recommended lap count | Cup aggregate scoring, published daily leaderboard (out of scope offline) |
| Accessibility | Keyboard-only start, focus-visible outlines, screen-reader control hint, 56 px touch targets, reduced motion, no colour-only HUD state | Full WCAG 2.2 AA audit, NVDA/VoiceOver sessions |
| Verification | `scripts/test-aura-drift-{physics,tracks,balance}.mjs`, `ui/desktop_aura_drift_test.go`, three headless Chrome tests (drive/dispose, 5-case quality/theme/viewport/reduced-motion matrix, night-harbour wet pass), bundle check, i18n/catalog/asset tests, DOX pass | Reference-hardware acceptance, 30-minute audio/soak run, offline clipping capture, all-tier screenshot review |

Balance measurement changed two concrete design values: authored radii were reduced to
440–850 m (2.5–5 km laps, measured 89–117 s per lap) so the recommended two-lap race
lasts about three minutes, and every track now recommends two laps instead of three or
four. Chunk streaming remains in place. The forest stage was widened and its harmonics
softened after the balance harness found AI cars scraping the barrier for entire laps.
Implementation consolidated the proposed render/audio module split into
`aura-drift-render.js`, `aura-drift-post.js`, `aura-drift-hud.js` and
`aura-drift-audio.js`; the file table in section 9 remains the proposed layout.

Repository docs and guides were updated with implementation (`AGENTS.md`, `ui/AGENTS.md`,
`ui/js/desktop/apps/AGENTS.md`, `internal/desktop/AGENTS.md`); ship-time behavior is
documented in `documentation/aura-drift.md`. The rest of this document remains the
roadmap and acceptance contract for the open phases.

Requested outcome (user, German): a top-down car racing game for the Virtual Desktop with
exceptional graphics, large tracks, perfect physics, clean sound, a weapon system and
varied tracks. Working title **Aura Drift**, app id `aura-drift`. Repository docs stay in
English; the default application language is German and all UI strings need all 16 Desktop
locales.

## 1. Product direction

The product is the sensation of grip: a car rotating under braking, sliding past the apex,
catching traction again while the engine note climbs with the revs. Every system serves that
moment. A race is short enough to retry, the world is large enough to explore, and a lost race
is always understandable.

| Pillar | Measurable meaning | Anti-goal |
| --- | --- | --- |
| Grip | Deterministic, frame-rate independent, tunable vehicle dynamics with believable understeer, oversteer, weight transfer and drift | Arcade cars that rotate like textured rectangles |
| Weite | Kilometre-scale tracks streamed from compact data, never one giant bitmap | A small arena described as a "big track" |
| Glanz | Layered lighting, weather, decals, particles and post-processing at four quality tiers | Static sprites on a flat coloured road |
| Klang | Synthesis driven by physical state, clipped-free buses and a clean lifecycle | A looping engine WAV whose pitch never changes |
| Arsenal | Pickups with counterplay, shared by AI, optional per mode | Weapons that decide the race before the first corner |
| Vielfalt | Six genuinely different layouts plus a validated seeded generator | One layout with six colour palettes |

**"Perfect physics" is a direction, not a claim.** It is defined here as deterministic,
frame-rate independent, collision-safe, tunable and believable under drift, with published
replay tests. This is a game model, not a vehicle-engineering simulation.

## 2. Platform decision

Three paths exist in this repository. Prefer one shipped product and one throwaway lab.

| Path | What it gives | Hard limits | Role |
| --- | --- | --- | --- |
| Game Maker Studio 2D, `topdown` starter | Fast generated prototype, Phaser 4.2.1, built-in sprite packs, automatic build/validation | Offline single-player only; no multiplayer, custom backends, external APIs or deployment; 960x540 logical FIT; 100 MB project, 250 files, 2,048 KB per source, 32 MB per asset; plan/validation gates; Arcade physics is AABB-based with no tire model | Phase-0 feel lab, not the shipped engine |
| First-party Virtual Desktop app | Full control over simulation, streaming, renderer, audio and lifecycle; matches existing Galaxa Deluxe / System World contracts | Must obey Desktop shell, locale, asset and line-budget contracts | **Recommended product** |
| Three.js orthographic world | Real 3D lighting and geometry from above | Heavier assets, second render stack, no need for a 2.5D racer | Only if a later art direction demands it |

The Game Maker route cannot carry a custom tire solver and kilometre-scale chunk streaming
reliably through generated code and repair gates. Its `topdown.ts` starter does show that
`configureWorld(w, h)` handles worlds up to a few thousand logical pixels, which is enough
for a steering-feel prototype. The server's validation driver uses arrow keys and Space;
`required_stages` requires genuinely distinct layouts and `required_controls` fails when
RIGHT moves the player left. A prototype should respect both from the start.

Reuse existing first-party contracts instead of inventing a framework:

- **Galaxa Deluxe** (`ui/js/desktop/apps/AGENTS.md`): modular Canvas 2D game, fixed 60 Hz
  simulation via `ctx.stepFrame` independent of rendering, `ctx.scheduleGame` instead of
  wall-clock timers, seeded run RNG, 30 stages, procedural Web Audio with separate
  music/SFX buses, master compressor, 32-voice cap, cached noise, music scheduled with
  120 ms lookahead and bar-boundary transitions, `ctx.settings.particles` tiers, sprite
  atlas with `atlas.json` and provenance, `ctx.G.muted` / `ctx.G.vol`, RAF cancelled while
  inactive, focus loss releases held input, DPR 1/2 fit, modules soft-capped at 1,000
  lines under one `window.GalaxaCore` namespace with explicit load order.
- **System World** (`documentation/system-world-plan.md`): isolated Three.js 0.186.1 ESM
  bundle, quality tiers with frame-time-based automatic choice and hysteresis, one render
  loop per visible window, hidden windows stop rendering, pooled effects, no per-frame
  allocation, complete disposal, WebGL2 fallback that keeps the map usable, bloom shaders
  with NaN-clamped math, reduced-motion behaviour.
- **Desktop shell**: lazy app loading in `ui/js/desktop/core/module-loader.js`, ids and
  labels in `desktop-foundation.js`, routing in `menus-and-routing.js`, SDK in
  `aura-desktop-sdk.js`, 16 locale bundles in `ui/lang/desktop/*.json`, version-bound
  resources from `internal/webassets` / `cmd/assetpack` (see `documentation/web-assets.md`),
  and `TestShippedWebAssetsLoadNothingExternal`.

## 3. Experience specification

- Camera follows the car from above with a velocity-based look-ahead, speed-dependent zoom
  and damped motion. North-up remains the default for readability; camera shake, speed
  lines and decorative sweeps are disabled by reduced motion.
- Race loop: start grid, lap or point-to-point checkpoints, splits, finish, results with
  best lap and ghost unlock. Restart resets every cooldown, spawn timestamp, run flag and
  RNG generation (Phaser reuses scene instances; the same discipline applies here).
- Modes: **Schnellrennen**, **Cup** (six tracks), **Zeitfahren** with an input-tape ghost,
  **Tagesroute** (seeded), and a weapon-free **Friedlicher Modus** for pure racing.
  Personal bests stay local to the browser; no accounts, backend, network or multiplayer.
- Controls: arrows/WASD steering and throttle, Space primary weapon, Shift handbrake,
  Ctrl/Alt for nitro or secondary use, P pause, R restart, Esc exit. Touch gets a left
  movement stick and right-side action buttons. Focus loss pauses and releases held input.
  `Desktop isActive` gates all controls and rendering.

## 4. Physics contract

A deterministic single-track vehicle model in `aura-drift-physics.js`:

- State: world position, heading, yaw rate, body-frame longitudinal/lateral velocity,
  steering angle, wheel speeds, gear/RPM, throttle/brake/handbrake, surface grip, load on
  each axle, damage.
- Bicycle model with front/rear slip angles, a simplified saturating tire curve, load
  transfer under acceleration and braking, drag, rolling resistance, downforce, brake bias
  and wheelspin. Handbrake reduces rear grip so oversteer is a controllable technique, not
  an animation. Understeer and oversteer emerge from the same equations.
- Surfaces are data: asphalt, wet, gravel, ice, oil, mud, each with friction coefficient,
  rolling resistance, particle and audio profile. Grip changes are continuous at zone
  boundaries to avoid snapping.
- Integration at a fixed 120 Hz substep with an accumulator, clamped catch-up (maximum
  five substeps per frame), semi-implicit integration and render interpolation. No
  `Math.random` in the simulation; all gameplay randomness uses the seeded PRNG, with
  particle-only seeds kept outside the simulation signature.
- Collisions: signed distance to the track spline for barriers, swept circle for
  high-speed wall contact, oriented boxes plus impulse resolution for car-car contact,
  restitution and friction, scrape and spin response, penalty-based respawn. Tire-wall
  contact must never tunnel at maximum speed.
- AI uses the identical solver: look-ahead pure pursuit on the racing line, distinct
  aggression profiles, collision avoidance, weapon use, and bounded rubber-banding whose
  limits are disclosed in the UI. AI never receives extra grip or hidden top speed.

Acceptance: identical input tapes produce identical state hashes at 30/60/144 render FPS;
a 10,000-step replay is bit-identical across runs; momentum and energy sanity bounds; no
tunneling at `vmax`; identical pause/resume and restart behaviour; 30-minute soak with
stable step time and no state drift.

## 5. Track system

- Each track is compact data, not artwork: a manifest (shipped as in-module data in
  `aura-drift-track.js`) holds radial-harmonic control points, a width profile, an
  arc-length lookup table, checkpoints, surface zones, weather/lighting theme, weapon
  spawn table and a seed. Closed Catmull-Rom splines become arc-length-parameterized
  geometry.
- Generation emits 256 m chunks around the camera plus look-ahead into an LRU cache of
  offscreen canvases with an explicit per-tier cap. Chunk content is a deterministic
  function of `(track id, seed, chunk coordinates)`. The data model scales to very large
  worlds; shipped tracks span roughly 2.5–5 km per lap after balance tuning. The minimap
  is derived from the same spline.
- Six launch biomes, each with a different layout grammar, grip profile, signature hazard,
  lighting and music:

| Track | Identity | Grip / weather | Signature |
| --- | --- | --- | --- |
| Alpenpass | Serpentines, tunnels, elevation changes | Cool dry asphalt, gravel runoff, fog | Hairpins with a risky inside line |
| Wüsten-Canyon | Long straights, rock gates, big radius curves | Loose sand edges, heat shimmer | Slipstream duels and a canyon shortcut |
| Neon-Hafen | Night, containers, drawbridge, wet quays | Low-grip rain, reflective puddles | Headlight cones and tight dock chicanes |
| Salzsee | Vast open flats, minimal barriers | Very high grip, glare, crosswind | Full-throttle navigation, no walls to lean on |
| Wald-Rallye | Narrow forest stages, mud, dusk | Mud and wet leaves, low visibility | Rhythm changes and hidden crests |
| Industrie-Ring | Oval core plus factory branches | Dry asphalt, oil patches, sparks | Branching shortcuts and moving gates |

- Variety is enforced: distinct control points, width profiles, surface mixes, checkpoint
  order, prop sets, lighting and themes. Renamed clones are not stages (the same rule the
  Studio's `required_stages` check applies to prototypes).
- **Tagesroute** uses a bounded track grammar with an automated drivability validator:
  minimum radius, no self-intersection, reachable checkpoints, bounded straight length and
  a valid racing line. A seed is never offered before it passes.
- A developer-only track lab renders the top-down map, grip heatmap and racing line and runs
  that validator; it is not part of the shipped app.

## 6. Visual system

- Shipped: Canvas 2D world renderer with a separate Canvas 2D HUD, plus an optional
  WebGL2 fullscreen post pass on the High and Ultra tiers. Without WebGL2, or after a
  context loss, the world, HUD, menus and results remain fully usable on Canvas 2D alone.
  A WebGL2 renderer for the scene geometry itself remains an open roadmap option.
- Draw order: ground/albedo with procedural asphalt detail, decal buffer (skid marks,
  rubber, oil, puddles), road furniture, shadows, cars, particles, weather, post pass, HUD.
- Lighting: headlight cones, lamp pools, neon emissive surfaces, biome-specific day/night,
  wet reflections, tunnel darkness, soft drop shadows and selective bloom. Shader math
  clamps `pow` bases and zeroes NaN/Inf before bloom so a bad frame cannot black out.
- Cars: rotatable multi-angle sprite atlas (32 or 64 directions) plus suspension lean,
  brake lights, exhaust flames, damage and dirt accumulation. Art is local WebP sheets with
  `atlas.json`, `provenance.json`, SHA-256 and BuildVersion-bound URLs; no external URLs.
- Post: restrained bloom, vignette, speed-adaptive motion blur/streaks, impact chromatic
  aberration, per-biome grade and grain. All are reduced or disabled on Low and by reduced
  motion. Readability of the racing line wins over spectacle.
- Quality tiers Low/Medium/High/Ultra with automatic frame-time selection and hysteresis.
  Particle and trail caps scale with `ctx.settings.particles`. Pool FX, reuse canvas
  resources, allocate nothing per frame, handle context loss, run one RAF per visible
  window, stop rendering when hidden or minimized, and dispose completely.
- Chrome follows Standard and Fruity light/dark; the world keeps its own lighting. HUD
  stays legible at 960x540 logical in a small Desktop window and crisp at DPR 2; state is
  never conveyed by colour alone.

## 7. Audio contract

- Web Audio synthesis only, as in Galaxa; no sample files and no external resources.
- Buses: engine, tyres, impacts, weapons, ambience, music, UI, each into a master
  compressor/limiter with a -1 dBTP target. Voice cap 32 with priority stealing, cached
  noise buffers, pooled nodes.
- Engine sound is a function of state: layered oscillators and filtered noise mapped to
  RPM, load, gear, throttle and speed; gear-shift clicks, turbo whistle and backfire;
  squeal from slip angle; surface layers for gravel spray, water and mud; Doppler and
  stereo panning for opponents; tunnel reverb from a generated impulse response.
- Adaptive music is scheduled against the audio clock with 120 ms lookahead and
  bar-boundary transitions, intensity derived from race state, one theme per biome. No
  hard cuts except crash and finish stings.
- Lifecycle: audio starts only after player interaction, honours Desktop mute and volume
  (`ctx.G.muted`, `ctx.G.vol`) and sound settings, suspends on pause or focus loss, closes
  its AudioContext and cancels scheduled nodes on dispose. Volume zero persists.
- Acceptance: decoded offline capture has no clipping, mute and reduced-motion paths are
  silent and clean, pause/resume produces no clicks or doubled notes, and a 30-minute soak
  leaks no nodes.

## 8. Weapon system

Design rule: a weapon adds a decision, has a counter, works identically for AI, stays inside
the deterministic simulation, and never decides a race on its own.

| Weapon | Effect | Counter |
| --- | --- | --- |
| Ölspur | Temporary low-grip zone behind the car | Clean line, higher tier tyres, wait out the timer |
| Nagelband / Mine | One-hit slow and damage, telegraphed before arming | Look ahead, shield, dodge |
| EMP-Blitz | Short engine cut in a radius | Boost clears it; no boss-style permanent disable |
| Suchrakete | Homing projectile with lock-on telegraph | Shield or late handbrake dodge |
| MG-Salve | Short frontal cone with ammo and recoil | Stay out of the cone, use slipstream |
| Schild / Reparatur | Defensive window and damage repair | Timing; limited charges |

Pickups are track-side pads with one active slot, ammo/cooldown, respawn timer, HUD charge,
visible telegraph and an audio cue. Nitro is a resource, not a weapon. Effects end with the
race, friendly fire is off, and a bounded, disclosed catch-up helps a trailing player without
guaranteeing a win. **Friedlicher Modus** disables the whole system for Time Trial and for
players who only want to race.

## 9. App architecture and Desktop integration

One namespace (`window.AuraDriftCore`), per-window `ctx` via `Object.create`, explicit load
order under the `aura-drift` entry, and an app shell exposing
`window.AuraDriftApp = { render, dispose }`. Keep each module under the 1,000-line soft cap;
files above 1,100 lines need an entry in `knownOversizedContinuations`
(`ui/desktop_js_line_budget_test.go`).

| Area | Proposed file |
| --- | --- |
| Shell, state, seeded RNG, fixed-step scheduler, disposal | `ui/js/desktop/apps/aura-drift-core.js`, `aura-drift.js` |
| Vehicle, surfaces, collisions | `aura-drift-physics.js` |
| Opponents and racing line | `aura-drift-ai.js` |
| Spline, chunks, streaming, validator | `aura-drift-track.js` |
| Six manifests, cars, weapon defs | `aura-drift-content.js` |
| Rendering | `aura-drift-render-{world,fx,post,hud}.js` |
| Audio | `aura-drift-audio-{core,sfx,music}.js` |
| Menus, results, settings | `aura-drift-ui.js` |
| Styles and art | `ui/css/aura-drift.css`, `ui/img/aura-drift/` with `atlas.json` + `provenance.json` |
| Locales | `aura_drift.*` keys and `desktop.app_aura_drift` in all 16 `ui/lang/desktop/*.json` |
| Registration | `ui/js/desktop/core/module-loader.js`, `desktop-foundation.js`, `menus-and-routing.js` |
| Dev scripts | `scripts/build-aura-drift-assets.mjs`, `scripts/test-aura-drift-physics.mjs`, `scripts/test-aura-drift-tracks.mjs` |
| Operator guide | `documentation/aura-drift.md` |

Contracts: lazy load, one render loop per window, `isActive` gating, focus loss handling,
reduced motion, theme tokens, no native browser dialogs, no external requests,
version-bound assets, and no personal-best writes while the Desktop is read-only.

## 10. Delivery sequence and exit gates

Sizes are relative planning bands, not calendar commitments.

| Phase | Scope | Size | Exit gate |
| --- | --- | --- | --- |
| 0. Physics and feel lab | Deterministic bicycle model in a standalone harness, one test spline, replay hash test, Studio `topdown` prototype for steering feel | M | Identical replay at 30/60/144 FPS; a 60-second drive feels controllable under throttle, brake and handbrake |
| 1. Vertical slice | One track, one car, full simulation, three weapons, minimal renderer, synthesized engine/HUD, Quick Race | L | Complete race is playable offline; pause/restart/focus/dispose pass; no external requests |
| 2. World and content | Chunk streaming, six biomes, AI opponents, Cup, Time Trial ghost, seeded daily route with validator | XL | Six distinct validated layouts; memory and chunk caps hold during a full lap; ghost replays exactly |
| 3. Craft | WebGL2 tiers, lighting/weather/post, car atlas, adaptive music, accessibility, all 16 locales | L | Screenshot matrix across biomes, themes and tiers; audio capture clean; locale/key checks pass |
| 4. Acceptance and release | Performance, determinism, soak, asset verification, operator guide and DOX closeout | M | All gates below pass on a recorded reference machine |

**First-release boundary:** one complete track, Quick Race and Time Trial with ghost, and the
whole physics/audio/weapon loop finished. Cup, daily routes and Ultra-tier effects follow;
none of them should be advertised as shipped before they exist.

## 11. Quality gates and verification

| Dimension | Target and method |
| --- | --- |
| Frame time | 60 fps at 1080p on a recorded midrange reference laptop; p95 frame <=16.7 ms; no per-frame allocation; measure per tier and record hardware |
| Determinism | Input-tape replay hashes identical across render FPS, restart and pause/resume; 10,000-step drift test |
| Streaming | Chunk and MiB caps respected; a full lap never grows memory monotonically; cache eviction is deterministic |
| Audio | Decoded capture has no clipping; mute/reduced-motion/pause paths silent and clean; no node leak in a 30-minute soak |
| Visual | Screenshot comparison per biome, theme and tier; no clipped HUD, unreadable text, NaN bloom or accidental global CSS changes |
| Accessibility | Keyboard and single-pointer task journeys, 44x44 CSS px primary touch targets, no colour-only state, reduced-motion coverage |
| Locales | All 16 genuine translations, long-text and CJK/Hindi cases; `TestDesktopUsedI18nKeysExistInAllLanguages` and `TestFrontend_StaticI18nKeysExistInEnglishBundle` |
| Safety | `TestShippedWebAssetsLoadNothingExternal`; no network calls, no external URLs, no backend or account requirement |
| Assets | Version-bound sheets with provenance and hashes; respect the documented resource archive limits |

Proposed checks (add focused tests rather than duplicating established coverage):

```text
node scripts/build-ui-bundles.js --check
node scripts/build-aura-drift-assets.mjs --check
node scripts/test-aura-drift-physics.mjs
node scripts/test-aura-drift-tracks.mjs
go test ./ui -run 'AuraDrift|DesktopAppAssets|LineBudget|DesktopUsedI18nKeys' -count=1
go test ./internal/webassets ./cmd/assetpack
AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run 'AuraDrift' -count=1
```

Browser artifacts belong in an ignored `reports/` directory via
`AURAGO_BROWSER_ARTIFACT_DIR`. For the phase-0 Studio prototype, keep
`go test ./internal/gamemaker` and the bundle check green.

## 12. Risks and honest boundaries

- "Perfect physics" is a measurable engineering bar, not an absolute promise. If the
  bicycle model cannot be made stable at 120 Hz within the phase-0 gate, reduce scope
  before adding content.
- WebGL2 support differs across machines; the fallback must be implemented, not promised.
- Procedural streaming is the largest technical risk; caps, deterministic eviction and a
  track validator are prerequisites, not polish.
- Weapons can unbalance a race; bounded catch-up and the peaceful mode are part of the
  design, not an afterthought.
- The game is offline and single-player. Multiplayer, accounts, custom backends, external
  APIs and deployment remain out of scope, matching Game Maker Studio v1.
- This document adds only a plan. `ui/AGENTS.md` and `ui/js/desktop/apps/AGENTS.md` stay
  authoritative for shipped behavior and must change with implementation, not preemptively
  advertise proposed features.

## 13. Immediate next unit

The physics lab and the Game Maker feel sandbox are done (the deterministic harness served
as the lab; the Studio prototype was never needed). The 2026-11-05 quality pass fixed the
world-anchored road markings, the low-speed steering-sign chatter and the reverse steering
feel, added the drift-refilled nitro resource, start lights and layered adaptive music.

Next unit: fully authored per-biome music themes (replace the scale/tempo layers with
composed stems), then reference-hardware handling sessions and an offline audio clipping
capture per the quality gates in section 11.
