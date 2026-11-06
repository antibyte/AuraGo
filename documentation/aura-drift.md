# Aura Drift

Aura Drift (`aura-drift`, category `games`) is a first-party, offline single-player top-down
racing game for the Virtual Desktop. It needs no provider, account, backend, network request
or external resource. See `plans/2026-11-04-aura-drift-racing-plan.md` for the product plan
and its remaining phases.

## Playing

Open **Aura Drift** from the Desktop start menu (Games). Choose a mode, a track and the
number of laps, then start. Selecting a track applies its recommended lap count (two laps,
about three minutes per race); 1–5 laps remain selectable. Four modes are available:
Quick race (three AI opponents), Time trial (solo, with a ghost replay of the recorded
best run), Cup (all eight tracks in sequence) and Daily route (a seeded, validated layout
generated from the UTC date). Weapons can be switched off in the settings; Time trial
never uses them. Trailing AI opponents receive a bounded catch-up of at most 6% target
speed, never extra grip; the menu discloses this in all 16 languages.

Every race runs a seeded **weather schedule**: conditions arrive and clear during the
race (rain on the harbour and in the city, snow on the glacier, fog, mist, haze, glare,
heat), and they are part of the deterministic simulation, so a replay always sees the
same weather at the same time. Weather changes the grip of every driver equally, is
drawn on the road and in the sky and drives the ambience mix of the score.

| Input | Action |
| --- | --- |
| Arrow keys / WASD | Steer, throttle, brake (left/right always rotate the car nose, also in reverse) |
| Space | Fire the equipped weapon |
| Shift | Handbrake (reduces rear grip for controlled oversteer) |
| Ctrl / Alt | Nitro boost (refilled by drifting, shown in the HUD) |
| Brake held at standstill | Reverse after a short beat |
| P / Escape | Pause and resume |
| R | Restart the current race |

Touch devices get on-screen steering, throttle, brake, fire and handbrake buttons. Focus
loss, window deactivation and tab hiding pause the race and release every held key. Audio
starts only after a user interaction and suspends on pause. A read-only Desktop still
allows racing but does not write settings or personal bests.

## What is implemented

- **Physics:** a deterministic bicycle model with front/rear slip angles, saturating tire
  forces, load transfer, drag, rolling resistance, brake bias, handbrake rear-grip loss,
  damage, a drift-refilled nitro resource and nine surface types. It runs at a fixed
  120 Hz with an accumulator and a five-substep catch-up clamp; rendering is independent
  and interpolated by the camera. Braking fades out at standstill (no slip-sign chatter),
  a held brake engages a bounded reverse, and the solver maps raw steering to an arcade
  nose response that is identical forward and reverse (no input-side flip, response fades
  to zero at a standstill so it never snaps through zero). Drafting a rival's wake trims
  drag and refills nitro. All gameplay
  randomness comes from a seeded PRNG; particles use a separate source.
- **Tracks:** eight authored radial-harmonic layouts with different widths, grip profiles,
  weather, lighting and checkpoint orders, plus a seeded daily generator. Closed Catmull-Rom
  splines are arc-length parameterized into roughly 4 m samples with a 64 m spatial grid.
  Ground and props are generated per 320 m chunk into offscreen canvases with an LRU cache
  capped by quality tier. `AD.validateTrack` checks length, width, spacing, corner radius
  and overlapping sections. Every layout is split into three **timing sectors** and its
  sharpest corners are derived into **drift zones**, which multiply the style score.
  The eight venues are: Alpenpass, Wüsten-Canyon, Neon-Hafen, Salzsee, Wald-Rallye,
  Industrie-Ring, Gletscherpass and Metropolis-Ring.
- **Style scoring:** sliding sideways above 9 m/s and 10 degrees of slip angle starts a
  drift chain. Chains are linked inside a 1.5 s window and grow a multiplier up to x8;
  the chain is banked when it ends, and only then counts. Loose surfaces pay more than
  tarmac and drift zones pay most. Near misses (a rival inside 2.7 m at closing speed)
  pay a bonus. The HUD shows the pending points, the live multiplier and the time left
  to link the next slide; the result screen shows the banked total and the best chain.
- **Graphics:** Canvas 2D world renderer with streamed ground chunks and a true offset
  road ribbon, so the surface edge always meets the kerbs exactly, even in hairpins.
  World-anchored markings (staggered red/white kerbs, haloed dashed centre line, worn
  racing-line bands, faint 55 m transverse joints, framed checkered start with gantry)
  never animate with frame time and scale with the camera zoom; skid-mark decals fade
  over the road surface. Cars are pre-rendered silhouettes (`aura-drift-cars.js`): four
  distinct shapes (prototype, GT, muscle, rally), body shading across the shell, glass
  with a sliding reflection, livery stripes, race numbers, wings and splitters, steered
  front wheels, spinning rears, brake lights with bloom, headlight cones at night,
  boost flames, shields, EMP rings and damage. Trackside furniture
  (`aura-drift-scenery.js`) dresses every venue: grandstands with tiers and a crowd,
  sponsor hoardings, tyre walls, safety fencing, marshal posts with flags, braking
  boards, pit garages and biome set pieces (cranes, containers, silos, pylons, radio
  masts, wind turbines, ice blocks, neon signs). The camera leans into the slide,
  punches the field of view on nitro and looks further ahead the faster the car goes.
  Particles cover tyre smoke for every car, water spray on wet asphalt, snow powder,
  boost flames, wall sparks and damage smoke. Weather draws rain, snow, fog, mist,
  haze, glare and heat shimmer. The HUD adds a live position tower with gaps, an
  rpm arc, gear, damage and nitro bars, the style meter, sector splits and a minimap
  with sector colours, rival dots and a start-line marker.
  The High and Ultra tiers add a WebGL2 post-processing pass (`aura-drift-post.js`) over
  the 2D frame: bright-pass bloom, radial speed streaks, short vertical wet-asphalt
  reflections (night harbour and industry, and any track while it rains), heat shimmer,
  rain on the lens, per-biome colour grading, vignette, impact
  chromatic aberration, damage heartbeat and film grain. Shader math is NaN-guarded and clamped; context
  loss, disposal and a Canvas 2D fallback are implemented. Four quality tiers scale
  particles, weather, lighting and chunk cache; `auto` selects a tier from measured frame
  time with a 2.5 s window and hysteresis. Reduced motion disables shake, weather, speed
  lines and shader animation.
- **Audio:** Web Audio synthesis only, no sample files. The engine stack (two detuned saws,
  a square body, an upper harmonic, a sub growl, intake and exhaust noise) is driven by
  rpm, load and the rev limiter; gear shifts click, throttle lifts backfire, the turbo
  spools and dumps through a blow-off valve, and a gear whine tracks the top end.
  Tyres have rolling noise, a resonant slip squeal, a surface layer per material
  (gravel rattle, sand rumble, mud squelch, snow crunch, wet spray, near silence on ice)
  and a wall-scrape layer. Rival engines are spatialised with distance gain, a distance
  low-pass, a **Doppler** pitch bend and a stereo pan, and everything feeds a generated
  convolution reverb sized to the venue. The score (`aura-drift-music.js`) plays a
  different arrangement per biome - drum pattern, bass rhythm, lead voice, tempo and scale -
  with an intensity that follows the speed, plus per-biome ambience beds (wind, water,
  machinery, crowd, birds) and one-shot gulls, clanks and bird calls. Menu, countdown,
  pickup, weapon, impact, sector, near miss, style bank and finish have their own cues.
  Volume, mute, focus and disposal are honoured.
- **Weapons:** oil slick, mine, EMP, homing rocket, MG salvo and shield, each with a
  telegraph, cooldown, ammo and a counter. AI uses the same rules. Hazards and projectiles
  live in the deterministic simulation.
- **Replays:** the player's input is recorded as a compact integer tape. Time trial re-simulates
  that tape through the identical solver to produce a ghost; the same tape is stored with a
  personal best in browser local storage.
- **AI and balance:** opponents use the identical solver with look-ahead pure pursuit,
  grip- and corridor-aware corner speeds, an overtaking line that moves around a rival
  ahead, bounded catch-up, nitro spent only on clean
  straights, handbrake slides through the tight corners where there is grip left, and a
  two-stage recovery (brief reverse, then a deterministic marshal
  respawn) after spins or weapon hits.
  `scripts/test-aura-drift-balance.mjs` runs seeded AI-only laps on all eight tracks and
  checks finishers, lap times (89–117 s), wall contacts, weapon usage, catch-up bounds,
  weather determinism and reproducibility; wall contacts stay in the low hundreds over a
  four-car race.
- **Accessibility:** keyboard-only start journey, visible `:focus-visible` outlines,
  screen-reader control hint with pointer-only touch controls hidden from the accessibility
  tree, 56 px touch targets, reduced motion, and no colour-only state in the HUD.
- **Desktop integration:** lazy module loading, one render loop per visible window, complete
  disposal of listeners, RAF, AudioContext, caches and observers, `isActive` gating,
  `app_aura_drift` plus `aura_drift.*` keys in all 16 Desktop locale bundles, Standard and
  Fruity chrome, and app-owned icon artwork.

## Troubleshooting: empty dark window

The original release used an `ad-` class prefix (`ad-overlay`, `ad-panel`,
`ad-menu`, …). Ad blockers such as AdGuard and EasyList hide elements matching
`[class^="ad-"]` with `display: none !important`, so the whole UI was hidden while
the game itself kept running — no JavaScript error and no failed request. All app
classes now use the `agdrift-` prefix, which contains no `ad` substring and cannot
match those filter rules. Never reintroduce `ad-`, `ads-`, `advert-` or `cc-`
prefixes; the same class of blockers hides `cc-` too.

`TestAuraDriftAvoidsAdBlockerClassPrefix` guards the source statically, and
`TestDesktopAuraDriftAdBlockerBrowser` injects a realistic `[class^="ad-"]`
blocker rule into the browser fixture and asserts the menu stays visible. If a
window is still empty after this fix, whitelist the Desktop origin in the ad
blocker and reload.

The app shell also defends against partial module sets: it verifies required
factories before boot, shows a translated error panel with the missing file names
and a reload button when one is absent, and replaces the optional WebGL2 post pass
with a no-op so the game stays fully playable on Canvas 2D.
`window.AuraDriftApp.stats()` reports `post.degraded` when that fallback is active;
`TestDesktopAuraDriftStaleLoaderBrowser` covers both cases.

## Boundaries

"Perfect physics" in the plan means deterministic, frame-rate independent, collision-safe,
tunable and believable — not an engineering-grade vehicle simulation. The world renderer
is Canvas 2D; WebGL2 is used for the optional High/Ultra post pass, not for scene geometry.
There is no multiplayer, backend, agent tool, config switch, external API or deployment.
Still open: engineering-grade tyre data, a recorded-instrument music layer on top of the
per-biome synthesised arrangements, offline audio clipping capture, a full WCAG 2.2 AA review,
 30-minute soak runs, acceptance on recorded reference hardware, a track editor and branch
 shortcuts. Cup aggregate scoring ships as 25-18-15-12 points shown with the cup total
 on each cup result screen.

## Verification

```text
node scripts/test-aura-drift-physics.mjs
node scripts/test-aura-drift-tracks.mjs
node scripts/test-aura-drift-balance.mjs
node scripts/build-ui-bundles.js --check
go test ./ui -run 'TestAuraDrift|TestDesktopBuiltinAppNamesExistInAllLanguages|TestShippedWebAssetsLoadNothingExternal' -count=1
go test ./internal/desktop -run TestBuiltinAppsCarryStartMenuCategories -count=1
AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_BROWSER_MUTE_AUDIO=1 go test ./ui -run TestDesktopAuraDrift -count=1
```

The browser suite has three parts. `TestDesktopAuraDriftBrowser` opens the app in the real
Desktop shell fixture, starts a race, waits out the countdown, drives with synthetic key
events and asserts that the race reaches `running` with a positive elapsed time and speed.
`TestDesktopAuraDriftQualityMatrixBrowser` covers all four quality tiers, Standard and
Fruity themes, 1440×900 and 900×600 windows and reduced motion; it checks WebGL2 gating,
chunk-cache caps, keyboard focus, 44 px touch targets and canvas sizing.
`TestDesktopAuraDriftWetReflectionsBrowser` selects the night harbour and verifies that
the wet pass is active and presenting. `AURAGO_BROWSER_ARTIFACT_DIR` writes screenshots to
an ignored `reports/` folder; `AURAGO_AURA_DRIFT_MATRIX_LIMIT` trims the matrix for quick
local runs. `window.AuraDriftApp.stats()` exposes read-only per-window state (race state,
time, speed, pause, quality, renderer caches and post-processing status).

The physics check compares state hashes for identical input tapes at 30 and 144 render FPS,
verifies solver bounds, low-grip understeer, wall contact at maximum speed, race replay
identity and input-tape round-tripping. The track check validates all eight authored layouts,
twenty generated daily routes, distinct shapes, spline round-trips, surface zones, drift
zones and sector splits. The
balance check runs seeded AI-only full laps and measures the figures described above.

Ownership: `ui/js/desktop/apps/AGENTS.md` (app modules and contracts), `ui/AGENTS.md`
(asset and locale rules), `internal/desktop/AGENTS.md` (catalog entry), and the plan above.
