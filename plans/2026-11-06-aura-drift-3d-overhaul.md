# Aura Drift 3D Overhaul: Implementation Plan

Date: 2026-11-06. Branch: `claude/aura-drift-3d`. Worktree: `C:\Users\Andi\Documents\repo\AuraGo\.claude\worktrees\aura-drift-3d`.

The user's request (German): make Aura Drift far more beautiful and better. Massively improve gameplay, sound and graphics, fix the swapped steering, create assets with Blender or directly, give it a fantastic look, lots of fun and variety, many new weapons, tricks and effects, some humour.

The user has approved true 3D with three.js plus Blender assets, and sound from CC0 recordings plus own synthesis.

Implementers see only this document. It is self-contained. Do not reopen design decisions. Tune numbers only where a task says so.

---

## 0. Working rules for every implementer

### Paths and commits
- Work only in the worktree above. Use absolute paths in tools. `node_modules` in the worktree is a junction to the main checkout.
- Commit with a normal `git commit` on the branch after every task, with this trailer: `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`. Do not push. Do not merge.
- Scratch scripts and screenshots go outside the repo (session scratchpad, or `D:\tmp\agdrift-shots`).
- Never kill processes by image name. Stop only PIDs you started.

### GitNexus
Before editing an existing function, run:

`node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "<symbol>" --direction upstream --repo .`

Before each commit, run:

`node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo .`

`AD.x = function` assignments in classic scripts often come back as `risk: UNKNOWN`. Treat that as unresolved and confirm with a text search. Known callers from reading the code:

| Symbol | Known callers |
| --- | --- |
| `AD.createRace`, `AD.stepRace` | `aura-drift.js`, `scripts/test-aura-drift-physics.mjs`, `scripts/test-aura-drift-balance.mjs` |
| `AD.stepVehicle`, `AD.createVehicle`, `AD.resolveWall`, `AD.resolveCarPair` | `aura-drift-race.js`, physics test |
| `AD.createRenderer`, `AD.createCamera`, `AD.createHud`, `AD.createAudio`, `AD.createUI` | `aura-drift.js` only |
| `AD.WEAPONS` | race, HUD, shell |
| `AD.CARS` | physics, cars, shell through `v.car` |

Warn in your report on any HIGH or CRITICAL result.

### Hard constraints
- Each first-party desktop JS file must stay under 1100 lines (`TestVirtualDesktopFirstPartyJSFilesStayBelowLineBudget`). This plan targets 1000 or fewer.
- No external resources (`TestShippedWebAssetsLoadNothingExternal`). Only same-origin URLs.
- CSS classes use the `agdrift-` prefix, never `ad-`, `ads-`, `advert-` or `cc-`.
- File names, URLs and class suffixes must avoid ad-filter words: `banner`, `sponsor`, `billboard`, `promo`, `advert`, `/ads/`. Those words may appear only as node names inside GLBs. Use `agdrift-comms`, not `agdrift-radio`.
- Write and Edit tools decode backslash-u sequences. Go test sources must use `\x5c` for a literal backslash. Locale JSON must contain literal UTF-8 characters.
- Simulation modules never touch DOM, `THREE`, `Math.random`, `Date` or `performance`. Gameplay randomness is `race.rng` or `features.rng`.
- All visible text goes through locale keys (section 2.11). Placeholders are `{name}` style and are replaced by `AD.fmt(text, values)`. Never pass args to `t`.
- Keep reduced motion, focus-loss pause, touch controls, readonly desktop, all-window disposal, the keyboard-only journey, 56 px touch targets and the stale-loader panel (`missingModules`, `showLoadError`, `createNoopPost`).

### Three.js lessons from this repo
- `THREE` r186 only resolves `#include` at line start.
- Integer-hash GLSL noise took 14.5 s to compile on D3D. Use texture-lookup noise only.
- `ShaderPass` deep-clones uniforms, so write to `pass.uniforms`, not to your original object.
- Neutral tone mapping desaturates above about 0.8.

### Blender lessons
- Blender 5.2.2 CLI: `D:\Blender 5.2\blender.exe`.
- Pass absolute output paths (relative ones land in `C:\disposable`).
- Vertex colour exports as float, about 33 B per triangle.
- Kit materials cull back faces.
- Blender numbers a name that is already taken, so clear the scene before each file.

### Browser tests
Run from PowerShell with the sandbox disabled, because Chrome started from repo-built binaries fails with a crashpad error. `GOTMPDIR` stays at its short backslash path (`D:\tmp\go-build`).

Example:

`$env:AURAGO_RUN_BROWSER_SMOKE='1'; $env:AURAGO_BROWSER_MUTE_AUDIO='1'; go test ./ui -run '^TestDesktopAuraDrift' -count=1 -timeout 20m`

Headless Chrome here has hardware WebGL but may report `prefers-reduced-motion`. Do not make tests depend on intro animation.

### Lean testing
- Write the tests this plan names and nothing more.
- No mutation, property or stress suites.
- One combined review per batch (reader-agent), reporting Critical and Important issues only, with no re-review rounds.
- At most one full browser smoke, at the end (batch 7).
- Visual checks use the survey command (section 1.7) and the Read tool on PNGs.

---

## 0.1 Existing defects this overhaul fixes

Each was found by reading the code, and each has a named owner task.

| # | Defect | Fix owner |
| --- | --- | --- |
| D1 | `aura-drift-conditions.js` `applyTarget()` overwrites `targetIntensity` with the current intensity. Intensity stays 0 forever, so no rain or snow is ever drawn and the depth grip penalty never applies. | T1.1 |
| D2 | `autoQuality` adds `dt` to `frameAccum` a second time. The average is doubled, so `auto` (the default) always falls to `low` within seconds. | T3.1 (governor) |
| D3 | `v.car.grip` is never read by `stepVehicle`, so car grip is a dead stat. | T1.1 |
| D4 | `updateGhost` passes the live player into `environmentAt`, so the ghost receives slipstream from the player and desyncs. | T1.1 |
| D5 | `car.tape.shift()` after 36000 steps silently corrupts ghosts, and raw JSON tapes eat localStorage. | T1.1 |
| D6 | Ctrl as nitro plus W as throttle means Ctrl+W closes the browser tab. Docs claim Alt, which is not implemented. Touch has no boost button. | T3.1 (keys), T5.2 (touch) |
| D7 | `hit` events carry no target, near-miss always says `car:0`, and weapon and surface names are German literals in tables. | T1.1, T1.4 |
| D8 | `resolveWall` limits the car to `halfWidth - 1.5`, so the `runoff` surface in `surfaceAt` is unreachable. Keep this behaviour. Loose-surface quirks act on `track.zones`. The 3D barrier must sit at the corridor edge so the wall is visible. | T3.3 |

---

## 1. Architecture

### 1.1 Layers

```
window.AuraDriftThree (vendor IIFE, optional)
        |
Sim (pure, node-testable): core physics track style conditions tricks features
                           weapons projectiles modes ai race
        | race state + race.events (read only for everything below)
Pure presentation: camera banter look
Assets: assets (the only module allowed to call fetch)
3D world: terrain road sky props actors fx post render3d   (optional, needs WebGL2)
2D fallback: cars scenery render                           (no WebGL2, or context lost)
HUD (2D canvas): hud hud-fx
Audio: audio-bank engine audio music
Menus (DOM): ui ui-cards
Shell: aura-drift.js
```

### 1.2 Render surfaces (DOM order inside `.agdrift-root`)
- `canvas.agdrift-gl`: WebGL2, three.js.
- `canvas.agdrift-world`: 2D fallback. `hidden` while 3D is active.
- `canvas.agdrift-hud`: 2D overlay, `pointer-events:none`.
- `div.agdrift-touch`, then `div.agdrift-overlay`.

The old `.agdrift-post` canvas is removed. A canvas can hold only one context type, so GL and 2D need separate canvases. The 2D renderer is created lazily, on fallback only.

### 1.3 Frame loop (`aura-drift.js`, one RAF per window)

```
loop(now):
  dt = min(0.05, realDelta)
  if !isActive() || document.hidden: return
  if view === 'menu':    renderer.showcase(dt, reduced); clear HUD; return
  if view === 'loading': renderer.showcase(dt, reduced); ui.progress(assets.progress()); maybe begin race; return
  simDt = timeWarp.scale(dt)
  if !paused:
     readInput()                     // running AND countdown
     scheduler.advance(simDt, s => AD.stepRace(race, s))
     for evt of race.events: renderer.event(evt, race); hud.event(evt, race); audio.event(evt, race);
                             banter.onEvent(evt, race); camera.onEvent(evt); timeWarp.onEvent(evt)
     race.events.length = 0
     renderer.emit(race, simDt)      // continuous smoke, spray, skid marks, flames
     audio.update(race, true, pose)
     msg = banter.update(dt); if (msg) hud.radio(msg)
     if race.state === 'finished': finishRace()
  pose = camera.update(race, dt, { reduced, paused })
  renderer.render(race, pose, paused ? 0 : simDt, reduced)
  hud.render(race, dt, settings, pose)
  tier = governor.sample(dt); if (tier) applyQuality(tier)
```

While paused, GL renders at 10 fps (skip frames) to save GPU.

**Hit-stop and slow-mo.** `timeWarp` only scales how many fixed steps run per wall second. Tapes and sim are per-step, so determinism is untouched. Reduced motion forces scale 1. HUD timers use sim `race.time`.

### 1.4 Final module list and load order

The loader entry in `ui/js/desktop/core/module-loader.js` is one path per line, exactly in this order. The stale-loader test removes single lines.

| # | Path | Lines (target) | Responsibility |
| --- | --- | --- | --- |
| 0 | `/js/vendor/aura-drift/three-runtime.js` | generated | IIFE, sets `window.AuraDriftThree` |
| 1 | `/js/desktop/apps/aura-drift-core.js` | 340 | constants, rng, `emit`, `fmt`, surfaces, CARS, DRIVERS, QUALITY, settings, bests v2, tape codec, scheduler, governor |
| 2 | `...-physics.js` | 340 | bicycle model extended (stats, burst, wobble, frozen, overdrive), wall in air, mass-aware pair |
| 3 | `...-track.js` | 360 | tracks, curvature, straight windows, crate rows |
| 4 | `...-style.js` | 240 | drift chains, near miss, `addStylePoints` |
| 5 | `...-conditions.js` | 125 | weather (D1 fixed) |
| 6 | `...-tricks.js` | 450 | air state, tricks, landing, drift turbo, perfect start, slingshot, reverse |
| 7 | `...-features.js` | 650 | ramps, pads, crates, destructibles, animals, set pieces |
| 8 | `...-weapons.js` | 620 | weapon table, roulette, firing, AI predicates |
| 9 | `...-projectiles.js` | 620 | projectiles, hazards, `applyHit`, car effect timers |
| 10 | `...-modes.js` | 420 | rule sets, elimination, hot potato, drift attack, results |
| 11 | `...-ai.js` | 460 | moved from race plus weapon, trick, crate and hazard logic |
| 12 | `...-race.js` | 430 | state machine, `stepActor`, progress, ghost |
| 13 | `...-banter.js` | 320 | taunts, result titles (non-sim rng) |
| 14 | `...-camera.js` | 380 | pure chase pose, projection, 2D project, time warp |
| 15 | `...-look.js` | 300 | per-biome look, terrain and scatter tables |
| 16 | `...-assets.js` | 340 | versioned URLs, manifest, GLB templates, stand-ins, audio bytes |
| 17 | `...-cars.js` | 450 | 2D fallback car sprites (adds 4 simple shapes) |
| 18 | `...-scenery.js` | 650 | 2D fallback scenery (unchanged apart from culling) |
| 19 | `...-render.js` | 900 | 2D fallback renderer rotated with the car, simple new entities |
| 20 | `...-terrain.js` | 560 | distance field, chunked heightfield, noise textures |
| 21 | `...-road.js` | 560 | ribbon, surfaces, kerbs, shoulder, barrier, start line |
| 22 | `...-sky.js` | 520 | dome, clouds, stars, aurora, water, salt, weather particles |
| 23 | `...-props.js` | 500 | deterministic placement, instanced streaming, signs |
| 24 | `...-actors.js` | 800 | cars, ghost, items, hazards, projectiles, features, animals |
| 25 | `...-fx.js` | 850 | particle pools, skid ribbon, beams, rings, event to FX mapping |
| 26 | `...-post.js` | 480 | EffectComposer pipeline (rewritten) |
| 27 | `...-render3d.js` | 650 | orchestrator, garage showcase, context loss, disposal |
| 28 | `...-hud.js` | 720 | frame, speed, nitro, turbo, weapon slot, tower, countdown |
| 29 | `...-hud-fx.js` | 580 | call-outs, comms with portraits, minimap, splits |
| 30 | `...-audio-bank.js` | 300 | sample loading, voices, synth fallback recipes |
| 31 | `...-engine.js` | 450 | engine loops, rivals, tyres, fallback oscillators |
| 32 | `...-audio.js` | 650 | buses, event mapping, ambience, API |
| 33 | `...-music.js` | 600 | layered pack playback plus existing live-synth fallback |
| 34 | `...-ui.js` | 720 | menu shell, tabs, pause, results, touch |
| 35 | `...-ui-cards.js` | 450 | mode, car, track, livery cards, map thumbnails |
| 36 | `/js/desktop/apps/aura-drift.js` | 800 | shell, input, lifecycle, debug API |

Other files:
- `ui/css/desktop-app-aura-drift.css` (rewritten).
- `assets/aura-drift/runtime-entry.mjs` (not shipped).
- `scripts/build-aura-drift-vendor.mjs`.
- `scripts/aura-drift-sim-loader.mjs`.
- `scripts/aura-drift-dev-server.mjs`.
- `ui/testdata/aura-drift-harness.html`.
- `ui/desktop_aura_drift_survey_browser_test.go`.

**Required factories** in shell `REQUIRED` (failing these shows the error panel):
- core `loadSettings`, `createScheduler`, `createQualityGovernor`
- physics `createVehicle`
- track `buildTrack`
- tricks `updateVehicleTricks`
- features `createFeatures`
- weapons `fireWeapon`
- projectiles `updateProjectiles`
- modes `updateMode`
- ai `aiInput`
- race `createRace`
- camera `createChaseCamera`
- render `createRenderer`
- hud `createHud`
- audio `createAudio`
- ui `createUI`

**Optional modules.** If any is missing the game still runs: vendor, `render3d` and its 3D modules (fall back to 2D with `post.degraded:true`), `assets` (stand-ins or synth only), `banter` (no-op), `audio-bank`, `engine`, `music` (silent or fallback).

### 1.5 Public API on `window.AuraDriftCore` (`AD`)

**core**
- `AD.STEP`, `AD.clamp`, `AD.lerp`, `AD.smooth`, `AD.wrapAngle`, `AD.distance`, `AD.rng`, `AD.hashString`, `AD.hashValues`, `AD.formatTime`, `AD.createScheduler`, `AD.createInput`, `AD.copyInput`, `AD.packInput`, `AD.unpackInput`: unchanged.
- `AD.emit(race, type, data)`: pushes `{type, t: race.time, ...data}`. No-op when `race.muteEvents` is true.
- `AD.fmt(text, values) -> string`.
- `AD.SURFACES`: remove German `label`; the HUD uses keys.
- `AD.CARS` (array, section 2.7), `AD.carById(id)`, `AD.carIndex(id)`.
- `AD.DRIVERS`, `AD.driverField(playerCarId, count, rng) -> driver[]`.
- `AD.QUALITY[tier]`: old 2D fields kept, plus `gl` (section 3.6).
- `AD.createQualityGovernor(startTier) -> { tier, sample(dt) -> newTier|null, lock(tier), reset() }`.
- `AD.defaultSettings()` adds `car:'vektor'`, `livery:0`, `camera:'chase'`.
- `AD.loadSettings`, `AD.saveSettings`: unchanged; settings key stays `.v1`, new fields default.
- `AD.BEST_KEY = 'aurago.desktop.aura-drift.best.v2'`. `AD.loadBests` removes the old v1 key when not readonly (no migration, old tapes cannot replay).
- A best entry is `{ time, tape: string, carId, score? }`.
- `AD.encodeTape(int[]) -> string`, `AD.decodeTape(string) -> int[]`: run-length pairs in base 36, comma separated.

**physics**
- `AD.createVehicle(x, y, heading, carIndex)`, `AD.resetVehicle`, `AD.vehicleState(v) -> number[]` (adds `z, vz, roll, pitch, burst`), `AD.stepVehicle(v, input, dt, env)`, `AD.resolveWall(v, sample, dt)`, `AD.resolveCarPair(a, b, radius)`, `AD.vehicleSpeedKmh(v)`.

**track**
- `AD.TRACKS`, `AD.BIOMES`, `AD.trackById`, `AD.buildTrack(manifest, seed)`, `AD.sampleTrack`, `AD.pointAt`, `AD.surfaceAt`, `AD.dailyTrack`, `AD.validateTrack`, `AD.chunkKey`, `AD.chunkProps`.
- New: `track.curvature: Float32Array` per sample, `AD.straightWindows(track, minLength) -> [{from, to}]`, `AD.trackName(track, t) -> string`.
- `track.pads` becomes 6 rows of 3 crates (18 entries `{s, offset, row}`).

**style**
- Existing, plus `AD.addStylePoints(car, points, useMultiplier)`.

**tricks**
- `AD.TRICKS`, `AD.createTrickState()`
- `AD.launch(race, actor, vz, source)`
- `AD.stepAir(actor, input, dt, race)`
- `AD.landingQuality(v) -> 'perfect'|'good'|'wobble'|'crash'`
- `AD.updateStart(actor, input, dt)`, `AD.applyStart(race, actor)`
- `AD.updateVehicleTricks(race, actor, input, dt)`
- `AD.aiAirInput(race, car, input)`

**features**
- `AD.createFeatures(track, seed, rules) -> fs`
- `AD.updateActorFeatures(race, actor, fs, dt)`
- `AD.updateFeatures(race, fs, dt)`
- `AD.featureLaneBias(race, car, sample) -> metres`
- `AD.validateFeatures(track, fs) -> {ok, reasons}`

**weapons**
- `AD.WEAPONS`, `AD.WEAPON_IDS`
- `AD.rollWeapon(rng, rankFraction, pool) -> id`
- `AD.giveWeapon(race, car, id)`
- `AD.updateWeaponInput(race, car, input, dt)`
- `AD.fireWeapon(race, car)`
- `AD.aiWantsFire(race, car) -> bool`

**projectiles**
- `AD.updateProjectiles(race, dt)`, `AD.updateHazards(race, dt)`
- `AD.applyHit(race, target, spec) -> bool`
- `AD.applyHazardEnv(race, actor, env)`
- `AD.updateCarEffects(race, car, dt)`

**modes**
- `AD.MODES`, `AD.modeRules(id)`
- `AD.initMode(race)`, `AD.updateMode(race, dt)`
- `AD.modeFinished(race) -> bool`
- `AD.buildResult(race) -> result`
- `AD.cupPoints(position) -> number`
- `AD.modeAiOverride(race, car, input)`

**ai**
- `AD.aiInput(race, car) -> input`, `AD.respawnCar(race, car)`.

**race**
- `AD.createRace(options)`. Options: `{ track, mode, seed, laps, opponents, weapons, ghostTape, ghostCar, showGhost, readonly, playerCar, livery }`.
- `AD.stepRace(race, dt)`, `AD.playerPosition(race)`.

**banter**
- `AD.createBanter({ seed, t, race }) -> { onEvent(evt, race), update(dt) -> {driverId, text}|null, dispose() }`
- `AD.pickResultTitle(result, car) -> titleId`.

**camera** (no `THREE`)
- `AD.CAMERA_MODES = ['chase','high','drone','bumper']`
- `AD.createChaseCamera({mode}) -> { mode, pose, update(race, dt, opts) -> pose, cycle() -> mode, setMode(m), script('intro'|'orbit'|'none'), onEvent(evt), addShake(a), heading }`
- `AD.chaseHeading(state, v, dt, params) -> radians`
- `AD.computeChasePose(state, v, params) -> pose`
- `AD.projectToScreen(pose, x, y, z, aspect) -> {x, y, depth, visible}`: NDC, +x right, +y up.
- `AD.project2D(cam, x, y, w, h) -> [sx, sy]`
- `AD.screenRotation(heading) -> theta`
- `AD.createTimeWarp({reduced}) -> { scale(dt) -> dt, trigger(kind), onEvent(evt), value }`

**look**
- `AD.LOOK[biomeId]`, `AD.TERRAIN[biomeId]`, `AD.SCATTER[biomeId]`, `AD.ANIMAL_FOR_BIOME`.

**assets**
- `AD.assetURL(path) -> versioned URL`
- `AD.assets.acquire()`, `AD.assets.release()` (refcount across windows)
- `AD.assets.loadCore() -> Promise`, `AD.assets.loadBiome(biomeId) -> Promise`
- `AD.assets.car(id) -> template`, `AD.assets.item(name)`, `AD.assets.prop(biomeId, name)`, `AD.assets.animal(kind)`
- `AD.assets.progress() -> 0..1`, `AD.assets.stats() -> {loaded, failed, standIns}`
- `AD.assets.fetchBytes(path) -> Promise<ArrayBuffer>`
- `AD.assets.fetchJson(path)`

**3D modules.** Each takes `T = window.AuraDriftThree`.
- `AD.buildDistanceField(track, cell) -> {at(x, y)}`, `AD.terrainHeight(biomeId, dist, x, y, sample)`
- `AD.createTerrain(T, track, look, q) -> {group, setWetness(w), dispose()}`
- `AD.roadRibbon(track, grow, vPeriod) -> {positions, uvs, indices}`
- `AD.createRoad(T, track, look, q) -> {group, setWetness(w), dispose()}`
- `AD.createSky(T, scene, look, q, track) -> {update(dt, pose, conditions), setDisco(on), dispose()}`
- `AD.scatterProps(track, biomeId, propsMeta) -> placements[]`, `AD.furnitureProps(track) -> placements[]`
- `AD.createProps(T, scene, track, biomeId, q, t) -> {update(pose), rebuild(), dispose(), stats()}`
- `AD.createActors(T, scene, q) -> {setRace(race), update(race, dt, pose), event(evt, race), dispose(), stats(), carPoints(slot) -> {car, nose, tail}}`
- `AD.createFx(T, scene, q) -> {event(evt, race), emit(race, dt), update(dt, pose), setQuality(q), dispose(), stats()}`
- `AD.createPost(T, renderer, scene, camera, q) -> {available, enabled, render(dt, race, pose, reduced), setQuality(q), setBiome(id), resize(w, h, dpr), impulse(kind, amount), setHoles(list), stats(), dispose()}`
- `AD.createRenderer3D(canvas, settings) -> renderer|null`

**Renderer interface** (both `createRenderer3D` and the 2D `createRenderer` implement it):
`{ kind:'three'|'canvas2d', post, resize(w,h,dpr), setQuality(tier), setTrack(track, race), showcase(dt, reduced), setShowcase({carId, livery, biomeId}), render(race, pose, dt, reduced), event(evt, race), emit(race, dt), stats(), dispose() }`.

The 2D renderer's `render` receives a `pose` and reads `pose.heading`, `pose.simX`, `pose.simY`, `pose.scale2d`. `post` is `createNoopPost()` for 2D.

**hud**
- `AD.createHud(canvas, labels, t) -> { resize, event(evt, race), render(race, dt, settings, pose), radio(msg), showMessage, popup, clear(), dispose() }`
- `AD.drawPortrait(c, driverId, x, y, size)`, `AD.drawMinimap(c, race, x, y, size, heading)`

**audio**
- `AD.createAudio(settings) -> { unlock, available, ui, setSettings, setBiome, setCar(id), horn(), update(race, active, pose), event(evt, race), suspend, resume, stats(), dispose }`
- `AD.createSampleBank(ctx, bus)`, `AD.createEngineAudio(ctx, bus, bank)`, `AD.createMusicEngine(ctx, noiseBuffer, destination, bank)`

**ui**
- `AD.createUI(root, labels, callbacks, settings, t) -> { touch, showMenu(bests), showLoading(), progress(p), showPause(), showResults(result, info), hide(), destroy() }`
- `callbacks.onStart(config)` where `config = {mode, trackId, laps, carId, livery}`.

**Diagnostics** on `window.AuraDriftApp`: `render, dispose, stats()`, plus:

```
debug = {
  start(config) -> Promise,   // bypasses the menu
  snapshot() -> {pose, aspect, kind, car:[x,y,z], aim:[x,y,z], psi},
  setCamera(mode),
  warp(fraction),             // sets car to arc fraction
  give(weaponId),
  fire(),
  freeze(bool)                // stops stepping, rendering continues
}
```

`stats()[i]`:

```
{ state:'menu'|'loading'|'countdown'|'running'|'finished', time, speedKmh, paused, mode, trackId, carId, quality, camera, field,
  renderer:{kind, calls, triangles, geometries, textures, particles, props, standIns, wetness, chunks, projection:{carX,noseX,tailX}|null},
  post:{available, enabled, degraded, quality, biome},
  audio:{samples, failed, fallback} }
```

### 1.6 Runtime delivery

`assets/aura-drift/runtime-entry.mjs` imports `* as THREE` from `three`, and from `three/addons`: `GLTFLoader`, `EffectComposer`, `RenderPass`, `UnrealBloomPass`, `ShaderPass`, `OutputPass`, `SMAAPass`, `FXAAPass`, `GTAOPass`, and `mergeGeometries` from `BufferGeometryUtils`. It re-exports them as named exports.

`scripts/build-aura-drift-vendor.mjs` follows `scripts/build-desktop-theme-vendor.mjs` with `format:'iife'`, `globalName:'AuraDriftThree'`, `target:'es2022'`, `minify:true`, `legalComments:'none'`, a banner comment, and supports `--check`. Output: `ui/js/vendor/aura-drift/three-runtime.js`, `LICENSE.txt`, `manifest.json` (`{three, bytes, sha256}`). It aborts if the three version is not `0.186.1` or the bundle exceeds 1.2 MiB.

No dynamic import. `TestAuraDriftShipsNoExternalResources` keeps forbidding `import(`, `XMLHttpRequest` and `http(s)://` in every module. It forbids `fetch(` in every module except `aura-drift-assets.js`, where every `fetch(` call must be literally `fetch(AD.assetURL(`.

`AD.assetURL(path)` is `window.AuraLazyAssets?.versionedURL(path) || path + '?v=' + encodeURIComponent(window.BUILD_VERSION || 'dev')`, the same pattern as `minigolf.js`. GLBs are fetched as bytes in `assets.js`, then `new T.GLTFLoader().parseAsync(bytes, '')`. Audio uses `{cache:'force-cache'}` like AuraGolf. `ui/sw.js` needs no change.

### 1.7 Dev harness and survey

`ui/testdata/aura-drift-harness.html` (never shipped; `testdata` is outside `assets/web-assets.json`):
- Fetches `/lang/desktop/de.json` into `window.I18N`.
- Loads `lazy-assets.js` and `module-loader.js`.
- Calls `AuraDesktopModules.loadAppAssets('aura-drift')`.
- Calls `AuraDriftApp.render(host, 'dev', {t, isActive:()=>true, readonly:false})`.
- Reads query params `track, car, mode, quality, camera, autostart, seed`.

`scripts/aura-drift-dev-server.mjs` serves `ui/` plus `/harness` on port 8791 with `no-store`, for real-screen play.

`ui/desktop_aura_drift_survey_browser_test.go`: `TestAuraDriftSurveyBrowser`, skipped unless `AURAGO_AURA_DRIFT_SURVEY=1`. It uses a Go mux over `http.FS(Content)` plus `/harness`, viewport 1280x720, runs scenes through `AuraDriftApp.debug`, and writes PNGs to `AURAGO_BROWSER_ARTIFACT_DIR`. `AURAGO_AURA_DRIFT_SCENES` is a comma-separated filter.

Scene names:
- `biome-<id>` (8, chase camera)
- `camera-<mode>` (4)
- `car-<id>` (8, showcase)
- `weapon-<id>` (18)
- `trick-air`
- `menu`
- `results`
- `hud`

It logs `stats()` per scene and fails only on `fixtureErrors`-style uncaught errors.

Command:

`$env:AURAGO_AURA_DRIFT_SURVEY='1'; $env:AURAGO_BROWSER_ARTIFACT_DIR='D:\tmp\agdrift-shots'; $env:AURAGO_AURA_DRIFT_SCENES='biome-alpine'; go test ./ui -run '^TestAuraDriftSurveyBrowser$' -count=1 -timeout 15m`

---

## 2. Simulation data contract

### 2.1 Step order (fixed; changing it breaks tapes)

```
stepRace(race, dt):
  countdown branch:
     for each car not out: input = player ? playerInput : AD.aiInput(race, car)
        AD.updateStart(car, input, dt)
        if player: car.tape.push(pack(input))
     ghost: input from tape, ghost.step++, AD.updateStart(ghost, input, dt)
     countdown bookkeeping, emit 'count'; at <= 0: state = 'running', for all actors AD.applyStart, emit 'go'
     return
  finished: return
  race.time += dt
  for each car in slot order, skipping car.out:
     stepActor(race, car, input, dt, race.features)
  pair collisions (i < j, skip out, cloak, or |z diff| > 1.6)
  AD.updateProjectiles; AD.updateHazards
  AD.updateFeatures(race, race.features, dt)
  updateGhost (stepActor with ghost feature copy, muteEvents around it, slipstream forced 0)
  race.conditions.update(dt)
  AD.updateStyle(race, dt)
  standings (out cars last, ordered by elimination, latest out first)
  AD.updateMode(race, dt)
  if AD.modeFinished(race) && player slot isPlayer && state running: state = 'finished'; result = AD.buildResult(race)

stepActor(race, actor, input, dt, fs):
  tape push (player only; cap 72000; beyond that result.tapeComplete=false and the tape is not saved)
  AD.updateWeaponInput(race, actor, input, dt) (not for ghost)
  env = environmentAt(race, actor)  // grip*conditions, roll, sample, slipstream, AD.applyHazardEnv
  effective = copy(input) with disabled, frozen, steerWobble, heat applied
  if v.airborne: AD.stepAir(actor, effective, dt, race) else AD.stepVehicle(v, effective, dt, env)
  AD.resolveWall(v, env.sample, dt)
  AD.updateVehicleTricks(race, actor, input, dt)
  AD.updateActorFeatures(race, actor, fs, dt)
  updateProgress(race, actor)
  AD.updateCarEffects(race, actor, dt)
```

The `tape` now includes countdown steps. The ghost replays them. Input packing is unchanged (no new bits). Camera and horn are presentation only and are never recorded.

### 2.2 Vehicle fields (`createVehicle` adds)

| Field | Init | Meaning |
| --- | --- | --- |
| `z`, `vz` | 0 | height (m), vertical speed (m/s) |
| `airborne` | false | in flight |
| `airTime` | 0 | seconds of the current flight |
| `roll`, `pitch` | 0 | radians. Roll positive = right side down. Pitch positive = nose up. |
| `rollRate`, `pitchRate`, `spinRate` | 0 | rad/s in air |
| `rollAccum`, `pitchAccum`, `spinAccum` | 0 | signed sum over the current flight |
| `wx`, `wy` | 0 | world velocity in air (`vx`, `vy` are kept in sync for HUD and audio) |
| `onRamp` | -1 | ramp feature index while rolling up it |
| `landWobble` | 0 | seconds, grip x0.55 |
| `burst`, `burstPower` | 0, 1 | seconds of forced drive boost, and its multiplier |
| `driftCharge`, `driftLevel` | 0 | seconds charged, level 0..3 |
| `draftTime` | 0 | seconds in a slipstream |
| `frozen` | 0 | seconds, steer x0.2, throttle 0 |
| `steerWobble`, `wobblePhase` | 0 | seconds of disco wobble, step-driven phase |
| `heat` | 0 | 0..1, at 1 sets `toast` |
| `toast` | 0 | seconds, power x0.6 |
| `cloak` | 0 | seconds, no collisions, immune, cannot fire |
| `overdrive` | 0 | seconds, drive x1.9, `vx` cap 105 |
| `towedBy`, `towing` | -1 | harpoon slots |
| `out` | false | eliminated |
| `hasBomb` | false | hot potato |
| `spinKick` | 0 | presentation hint, last hit spin direction |

Physics uses `const c = v.car`:
- `M*c.mass`, `IZ*c.mass`, `CF*c.mass`, `CR*c.mass`, `BRAKE_FORCE*c.mass`, handbrake `5200*c.mass`.
- So mass changes acceleration, collision weight and shove, not handling feel.
- `drag = DRAG*c.dragK*(1-0.32*draft)*vx*|vx|`, steer limit x `c.steerK`.
- `muFront = grip*c.grip`, `muRear = grip*c.grip*(handbrake?0.42:1)`; `landWobble` multiplies grip by 0.55 (D3).
- `drive *= v.burstPower` while `burst>0`, with `drive = max(drive, 3200*c.mass)`.
- `overdrive>0`: `drive*=1.9`, nitro not spent.

`resolveWall` when airborne reflects `wx`, `wy` instead of the body-frame velocity, keeping its signature.

`resolveCarPair(a,b,radius)` uses masses: `impulse_a = -rel*1.24*m_b/(m_a+m_b)`. With equal masses this is the existing 0.62. The race passes `radius = (a.car.radius + b.car.radius)/2`.

### 2.3 Race fields added

- `race.features`: `fs`. `race.pickups` is removed; crates are in `fs.crates`.
- `race.rules`: `AD.modeRules(mode)`.
- `race.drivers`, `race.mode`, `race.clock` (seconds left or null), `race.elimination` (`{next, count}`), `race.bomb` (`{holder, fuse, round, delay, immune, immuneFor}`).
- `race.nextOut` (slot for HUD), `race.muteEvents` (false).

Car fields:
- `driver`, `livery`, `tricks` (trick state), `tally`.
- `weaponLock` (seconds of roulette), `fuel` (flamethrower seconds), `holdFire`.
- `eliminatedAt`, `finished`.

`tally`:

```
{ wallHits, brakeSeconds, airTime, drifts, driftSeconds, reverseSeconds, tricks, crashLandings, perfectLandings,
  animalHits, smashed, weaponsFired, hitsDealt, hitsTaken, sausageHits, discoFired, carHits, respawns }
```

`race.result` adds:

```
{ mode, score, medal, title, tricks:{counts, best}, tally, tapeComplete, eliminatedAt, field, carId }
```

`time`, `bestLap`, `position`, `tape`, `style` and `sectors` stay.

### 2.4 Feature objects (`fs`)

All are deterministic from `AD.hashString(track.id+'/features/'+seed)`. `fs.rng` is separate from `race.rng`, so a ghost copy behaves identically. A ghost gets its own `AD.createFeatures` instance. `fs.buckets` holds 64 arrays indexed by `floor(s/length*64)`. Each actor only tests its bucket and the two neighbours.

| `kind` | Fields | Placement | Behaviour |
| --- | --- | --- | --- |
| `ramp` | `id, s, offset, x, y, psi, width, length:9, height:1.6` | `clamp(1+floor(length/1400),2,5)` per lap, on `AD.straightWindows(track,330)`, 150 m before the window end, `offset=±0.24*track.width`, `width=0.42*track.width` | Inside the rect: `v.onRamp`, `z=height*along/length`, `pitch=atan(height/length)`. At exit: `AD.launch(race, actor, clamp(0.2*max(vx,0),6,14),'ramp')`, or `vz=2` under 12 m/s. |
| `pad` | `id, s, offset, x, y, psi, width:6, length:10, power:1.35, duration:0.9` | `floor(length/700)` per lap on straights and corner exits | Per-actor cooldown 1.5 s, sets `burst`, `burstPower`, `boost_pad` event. Softeis quirk: duration x1.4. |
| `crate` | `id, s, offset, x, y, taken, row` | From `track.pads` | Radius 4.2, 8 s respawn. Taken even when a weapon is held (existing rule). `z<3.5`. `giveWeapon` only if `car.weapon` is null. |
| `cone`, `hay`, `barrel`, `tyres` | `id, kind, x, y, r, state:'idle'|'flying', vx, vy, vz, z, spin, t` | Rows on corner-exit edges: 6 clusters per km | Speed factor on hit: cone 0.995, hay 0.96, barrel 0.97, tyres 0.94; Bertha and Mähkönig pay nothing. Flies off with a deterministic impulse, respawns after 20 s, `smash` event. Radius r: cone 0.4, hay 1.1, barrel 0.7, tyres 1.0. |
| `animal` | `id, kind, crossing, herd, index, x, y, psi, state:'idle'|'walk'|'flying'|'trot', z, vz, vx, vy, t` | `clamp(floor(length/1800),1,3)` crossings on straights at least 120 m from ramps, each with 3 to 5 animals, period 14 to 22 s from `fs.rng` | Position is a pure function of `race.time`: walk across `road width + 8 m` in 6 to 8 s, wait, walk back. Hit (`z<2.2`): launched `vz=7`, flies, lands fine, trots away, returns next cycle. Speed x0.92, no damage, `animal_hit`, +150 style points. Three or more of one herd within 4 s: `strike` +1000. Passing above while `z>=2.2`: `cow_vault` trick. Never removed. No gore. |
| `crossing` | `id, s, x, y, kind, signS` | Per animal group | Telegraph: sign at `s-120`, marshal post, rumble bars rendered at `s-60`, HUD icon when 200 m ahead and the herd is on the road within 3 s. |
| `setpiece` | `id, kind:'ufo', x, y, period:45` | Salt only: 300 m from track on the inner side | Emits `ufo_abduct` at `race.time mod period == 20`. Presentation only. |

`AD.ANIMAL_FOR_BIOME`: alpine cow, desert llama, harbour crab, salt cow, forest sheep, industry sheep, glacier penguin, metro llama.

`rules` toggles (section 2.8): `crates`, `animals`, `destructibles`, `ramps`, `pads`.

`AD.validateFeatures`: no feature within 40 m of s/f, ramps have 150 m runout with `curvature<0.01`, crates inside corridor.

### 2.5 Hazards and projectiles

`race.hazards[]`, all with `{type, x, y, r, ttl, owner}`:

| `type` | Extra | Effect |
| --- | --- | --- |
| `oil` | | grip min 0.20 inside `r=7` |
| `gnome_mine` | `arm:0.45` | Blast on contact `r+1.4`: `applyHit` damage 16, speed x0.68, launch `vz=6` |
| `ice` | | grip min 0.30 inside `r=8` |
| `black_hole` | `age`, `pull:22` | Pull inside `r=26`, accel `pull*(1-d/r)` toward centre |
| `disco` | `z:6` | Inside `r=22`: rivals get `steerWobble=0.3` refreshed |
| `cow_shadow` | `land` (seconds), `tx, ty` | Telegraph only; removed when the cow lands |

`race.projectiles[]`, all with `{type, x, y, z, psi, speed, ttl, owner}`:

| `type` | Extra | Behaviour |
| --- | --- | --- |
| `rocket` | `target` | speed 62, ttl 6, turn 2.4 rad/s |
| `bullet` | | speed 105, ttl 0.65 (rendered as a bratwurst) |
| `duck` | `target`, `wob` | speed 34, ttl 7, turn 1.8 rad/s, ground hugging hop |
| `grenade` | `vz`, `tx`, `ty` | Arc over 0.7 s, lands as `ice` plus burst |
| `hole_shot` | `vz`, `tx`, `ty` | 40 m/s arc, lands as `black_hole` |
| `disco_shot` | `tx`, `ty` | Flies 20 m ahead, becomes `disco` |
| `harpoon` | `target` | speed 120, ttl 0.6, attaches |
| `cow` | `vz`, `tx`, `ty`, `flight:1.4` | Arc, lands with shock |
| `glove` | `target` | Presentation only: spring 0.12 s |

`beams[]` (`race.beams`, presentation-friendly, sim-owned): `{type:'rail', x, y, x2, y2, ttl:0.25}`, `{type:'tether', a, b}`, `{type:'flame', car, ttl}`.

### 2.6 Event catalogue

Base fields on all events: `type, t`. `car` is a slot index. `x`,`y` are sim metres. Reaction column key: F = fx, A = audio group, H = HUD, B = banter, C = camera or time warp.

| Type | Fields | Reaction |
| --- | --- | --- |
| `count` | `value` | H big light; A `ui.count` |
| `go` | | H GO; A `ui.go`, crowd cheer |
| `perfect_start` | `car` | F blue flames; H call-out; A `sting.perfect` |
| `wheelspin` | `car` | F smoke; H; A `tyre.squeal` |
| `lap` | `car, lap, time` | H; A `ui.lap` |
| `final_lap` | `car` | H; A `stinger.final_lap`; C slow-mo none |
| `sector` | `index, time, best` | H split |
| `finish` | `car, time` | F confetti; A fanfare by position; C orbit, slow-mo |
| `position` | `car, from, to` | H pulse; B passed or pass |
| `respawn` | `x, y, car` | F sparkle; B marshal |
| `wall_hit` | `car, x, y, speed, heavy` | F sparks; A `impact.metal`; C shake |
| `car_hit` | `a, b, x, y, impulse, heavy` | F sparks; A `impact.heavy` or `impact.light`; C shake if player |
| `drift_start` | `car, multiplier` | F smoke up; A skid |
| `drift_level` | `car, level` | F wheel sparks colour; H bar; A `turbo.tick` |
| `turbo_burst` | `car, level, x, y` | F flame burst; A `turbo.<level>`; C FOV punch |
| `style_bank` | `car, points, multiplier` | H call-out; A `ui.bank` |
| `near_miss` | `x, y, car, bonus, other` | H; A whoosh; C small shake |
| `slingshot` | `car, x, y` | F wind streaks; H; A `sting.slingshot` |
| `takeoff` | `car, x, y, vz, speed, source` | F dust; A `ramp.launch`; C camera lift |
| `trick` | `car, id, points, chain, x, y` | H kinetic text; A `sting.trick` |
| `landing` | `car, x, y, quality, airTime, points, nitro` | F dust and ring; H; A `land.*`; C hit-stop on crash, slow-mo on perfect; B wreck if crash |
| `reverse_hero` | `car` | H |
| `j_turn` | `car, x, y` | H; A squeal |
| `boost_pad` | `car, x, y` | F flash; A `pad.boost`; C FOV punch |
| `pickup` | `x, y, car, id, lock` | F crate burst; H roulette (player); A `crate.break`, `roulette.tick` |
| `smash` | `car, x, y, kind, feature` | F by kind (hay poof, cone bonk); A `prop.<kind>` |
| `animal_cross` | `kind, x, y, crossing` | H warning icon; A animal call |
| `animal_hit` | `car, x, y, kind, feature, points` | F feathers or stars; H "Kuh-Bowling"; A `animal.<kind>`, `sting.moo` |
| `strike` | `car, count, x, y` | H "STRIKE"; A `sting.strike`, crowd |
| `ufo_abduct` | `x, y, feature` | F beam; A `ufo.beam` |
| `weapon` | `id, x, y, car` | A `weapon.<id>`; H ammo; B |
| `weapon_end` | `id, car` | stop loops |
| `hit` | `x, y, car, by, heavy, weapon` | F explosion; A `explosion.*`; C shake and aberration; B got_you or hit |
| `shield` | `x, y, car` | F bubble; A `weapon.shield_on` |
| `shield_block` | `x, y, car, by` | F ring; A `weapon.shield_hit` |
| `rocket_lock` | `car, target` | H warning arrow if target is player; A `weapon.rocket_lock` |
| `emp_charge` | `car, x, y` | F charge ring; A |
| `emp_pulse` | `x, y, car, r` | F ring; A; C aberration |
| `emp_hit` | `x, y, car` | F sparks |
| `rail_charge` | `car, x, y, psi` | F laser line; A rising; H warning |
| `rail_fire` | `car, x, y, x2, y2, hits` | F beam; A; C shake and hit-stop |
| `flame_start`, `flame_end` | `car` | F cone; A loop |
| `flame_hit` | `car, by` | F embers |
| `toast` | `car` | F soot smoke; H comic |
| `frost_throw` | `car, x, y, tx, ty` | F arc marker |
| `frost_burst` | `x, y, r, frozen` | F ice shards; A |
| `horn` | `car, x, y, r, cleared` | F shock ring; A `weapon.horn_blast` |
| `hole_open` | `x, y, r, owner` | F lens; A loop; post `setHoles` |
| `hole_close` | `x, y` | F implosion |
| `disco_on` | `x, y, r, owner` | F; sky `setDisco(true)`; A `stinger.disco` |
| `disco_off` | `x, y` | sky false |
| `glove` | `car, target, x, y, hit` | F boing; A `weapon.glove` |
| `harpoon_fire` | `car, target` | F rope; A |
| `harpoon_attach` | `car, target` | A clank; H warning |
| `harpoon_break` | `car, target, reason` | F snap |
| `cow_launch` | `car, target, tx, ty, flight` | F arc; A moo; H shadow warning |
| `cow_land` | `x, y, r, hits` | F shock and dust; A; C shake |
| `duck_launch` | `car, count` | A quacks |
| `duck_hit` | `car, x, y` | F feathers; A |
| `cloak_on`, `cloak_off` | `car` | Material; A |
| `overdrive_on`, `overdrive_off` | `car` | F flames; A |
| `elimination_warn` | `car, in` | H next out pulse |
| `eliminated` | `car, x, y, position` | F comic explosion; H; A; C if player |
| `bomb_given` | `car, by, fuse` | H; A |
| `bomb_pass` | `from, to, x, y` | F spark; H; A |
| `bomb_tick` | `car, fuse` | A `bomb.tick` (rate by fuse) |
| `bomb_blast` | `car, x, y, r` | F huge; A; C |
| `clock` | `seconds` | H last 10 seconds |
| `sweeper` | | H "Besenwagen", race ends in 45 s |
| `weather` | `kind, intensity` | H label; sky |

### 2.7 Car table (`AD.CARS`, order fixed; `id` is the stable key)

| id | name | shape | power | mass | grip | dragK | steerK | airControl | looseRoll | radius | visualScale | quirk | Approx. top speed |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `vektor` | Vektor | `wedge` | 1.00 | 1.00 | 1.00 | 1.00 | 1.00 | 1.00 | 1.00 | 2.05 | 1.25 | Slipstream effect x1.25 | 71 m/s |
| `kobalt` | Kobalt | `gt` | 0.96 | 1.02 | 1.06 | 0.98 | 1.02 | 1.00 | 1.00 | 2.05 | 1.25 | Wet surface grip x1.06 | 70 |
| `lumen` | Lumen | `muscle` | 1.08 | 1.12 | 0.95 | 1.02 | 0.96 | 0.95 | 1.00 | 2.05 | 1.25 | Drift turbo duration x1.2 | 73 |
| `tundra` | Tundra | `rally` | 0.92 | 0.92 | 1.04 | 1.05 | 1.05 | 1.05 | 0.75 | 2.05 | 1.25 | Loose surface grip x1.18 | 68 |
| `softeis` | Softeis | `van` | 1.10 | 1.35 | 0.92 | 1.18 | 0.90 | 0.85 | 1.00 | 2.20 | 1.2 | Boost pad duration x1.4; horn is the jingle | 69 |
| `maehkoenig` | Mähkönig | `mower` | 0.72 | 0.50 | 1.12 | 1.30 | 1.15 | 1.15 | 0.45 | 1.80 | 1.3 | Destructibles cost no speed | 59 |
| `couch` | Couch-Potato | `sofa` | 0.90 | 0.72 | 0.88 | 1.10 | 1.12 | 1.35 | 1.00 | 1.90 | 1.25 | Trick points x1.25; drift nitro x1.3 | 67 |
| `bertha` | Big Bertha | `truck` | 1.25 | 1.60 | 0.98 | 1.20 | 0.86 | 0.90 | 0.60 | 2.40 | 1.15 | Mine and shock knockback x0.6; destructibles and small animals cost no speed | 72 |

Extra fields:
- `colour`: hex. Vektor `#ff5a4e`, Kobalt `#4f8cff`, Lumen `#f6c445`, Tundra `#4ade80`, Softeis `#ffb3d1`, Mähkönig `#7ac74f`, Couch `#b5764a`, Bertha `#8a5cf6`.
- `accent`: keep the existing four, add `#fff7e6`, `#ffe066`, `#f3e3c3`, `#2a2a2a`.
- `liveries`: array of 4 hex.
- `engine`: same as `id`.

`AD.DRIVERS` (proper names, not translated). Each has `{id, name, carId, skill, aggression, drift, portrait:{helmet, visor, hair, moustache:boolean}}`:

| id | name | carId | skill | aggression | drift |
| --- | --- | --- | --- | --- | --- |
| `viktor` | Viktor Vollgas | `vektor` | 0.97 | 1.05 | 0.5 |
| `rosi` | Rosi Rückspiegel | `kobalt` | 0.99 | 0.90 | 0.3 |
| `bruno` | Bruno Bleifuß | `lumen` | 0.92 | 1.12 | 0.6 |
| `heidi` | Heidi Handbremse | `tundra` | 0.94 | 1.00 | 1.0 |
| `ilse` | Ilse Eistüte | `softeis` | 0.90 | 0.95 | 0.4 |
| `gustav` | Gustav Gartenzwerg | `maehkoenig` | 0.92 | 0.88 | 0.4 |
| `kurt` | Kurt Kanapee | `couch` | 0.88 | 0.82 | 0.7 |
| `dolores` | Dolores Dampfwalze | `bertha` | 0.91 | 1.15 | 0.2 |

`AD.driverField(playerCarId, count, rng)` picks `count` drivers without the player's car, seeded. Grid order: the player takes grid slot `min(4, count)` (0-indexed), so most races start with overtakes. The slot index stays 0 for the player.

### 2.8 Mode table (`AD.MODES`)

| id | opponents | weapons | crates | ramps, pads, destructibles | animals | laps | End condition | Result ranking |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `quick` | 5 | setting | yes | yes | yes | config | Player finishes. Or `sweeper`: after the first finisher, 45 s later the race ends. | Finish order |
| `time_trial` | 0 | off | no | yes | no | config | Player finishes | Time, ghost saved |
| `cup` | 5 | setting | yes | yes | yes | track | As quick, across `AD.TRACKS`; points 25, 18, 15, 12, 10, 8 | Points |
| `daily` | 5 | setting | yes | yes | yes | 3 | As quick | Finish order |
| `drift_attack` | 0 | off | no | yes (no destructible penalty) | no | 99 | `race.clock` 90 s reaches 0 | `style.banked + pending`; medals bronze 10000, silver 22000, gold 40000 (retuned in T1.6 from the scripted drifter, see the balance script header) |
| `elimination` | 5 | setting | yes | yes | yes | 99 | First out at 45 s, then every 30 s; last in standings out (`elimination_warn` 5 s before); ends when one remains or the player is out | Reverse order out |
| `hot_potato` | 5 | pool `air_horn, shield, overdrive` | yes | yes | no | 99 | Bomb after 5 s to a `race.rng` pick. Fuse `max(10,16-round)`. Passes on contact (dist < sum of radii + 0.4) with giver immune 1.5 s. Holder drive x1.12. Blast: holder out, shock `r=16`. 3 s until the next bomb to the car nearest the blast. Ends when one remains or the player is out. | Reverse order out |

Bests keys: `bests[track.id]` for time trial, `bests['drift:'+track.id]` for drift attack (`{score, carId}`).

Drift-attack thresholds stay fixed unless the balance reference bot (section 5) scores outside 30 to 60 percent of gold.

AI hot potato: `AD.modeAiOverride` makes the holder chase the nearest rival (lane target toward it) and non-holders within 40 m bias away from the holder.

### 2.9 Weapon table (`AD.WEAPONS`)

Fields:

```
{ id, ammo, cooldown, range, colour, hold:boolean, nameKey:'aura_drift.weapon.<id>.name', hintKey, weights:{front, mid, back}, item:'<glb node>' }
```

Fire rules:
- `car.weaponLock = 1.1` after pickup (roulette; firing blocked).
- `race.weapons=false`: crates hidden and `giveWeapon` no-op.
- `applyHit` honours shield and cloak everywhere and credits `tally`.
- Telegraph events fire before the effect.
- Knockback impulses divide by `mass`.

| id | ammo | cooldown s | range m | Effect | Telegraph | Counter | AI fires when | Weights F/M/B |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `oil` | 1 | 0.9 | behind 3.2 | Hazard `oil r7 ttl6` | Visible slick | Steer around | Rival within 40 m behind | 18/6/0 |
| `gnome_mine` | 1 | 1.1 | behind 3.2 | Hazard r3.4, arm 0.45, ttl 12; dmg 16, speed x0.68, launch vz 6 | Gnome with blinking hat, radius ring | Avoid, shield, horn | Rival within 35 m behind | 18/8/0 |
| `emp` | 1 | 1.4 | 30 | After 0.25 s charge, rivals in range `disabled=1.5` | Expanding ring, `emp_charge` | Leave radius, shield, cloak | 2 or more rivals within 26 m, or 1 within 14 | 2/6/8 |
| `rocket` | 1 | 1.6 | homing | Speed 62, ttl 6, turn 2.4; dmg 26, speed x0.72, launch vz 7 | `rocket_lock`, HUD arrow | Sharp turn, shield, horn, cloak | Rival ahead within 90 m in 60 deg cone | 4/12/10 |
| `mg` | 6 | 0.16, hold | 68 | 5 bratwurst per shot, spread ±0.11 rad; dmg 7, speed x0.94 | Muzzle pop | Side-step, shield | Rival within 50 m in 12 deg cone | 8/10/6 |
| `shield` | 1 | 1.2 | self | 4 s, repair 22 | Bubble | Railgun breaks it | Threat in `rocket_lock`, or damage above 50 | 16/6/4 |
| `ducks` | 1 | 1.4 | homing | 5 ducks, speed 34, ttl 7, turn 1.8; speed x0.93, spin ±1.1, dmg 3 | Quacks, visible swarm | Outrun above 34 m/s on straights, shield, horn | Rival ahead within 80 m | 6/10/8 |
| `railgun` | 1 | 2.0 | 220 | 0.35 s charge then instant beam width 1.8, pierces all, dmg 22, speed x0.78, breaks shields without damage | Laser line 0.35 s | Sidestep, cloak | Rival within 150 m and 5 deg of the nose, straight ahead | 0/3/6 |
| `flamethrower` | 1 | 0.5, hold | 18 | 2.5 s fuel; cone half-angle 14 deg; 14 dmg/s, `heat += 1.2/s`, at 1: `toast 0.6 s` | Pilot flame 0.2 s | Leave cone, shield, frost melts ice | Rival in cone | 2/8/6 |
| `frost` | 1 | 1.2 | 30 | Arc lands as `ice r8 ttl8`, `frozen 0.8 s` to cars within 5 m | Landing marker | Avoid, shield, flamethrower clears ice | Rival ahead within 45 m | 8/6/2 |
| `air_horn` | 1 | 1.0 | 18 | Shove 14 m/s divided by mass, clears projectiles and hazards in radius | Comic honk, shock ring | Keep distance, mass | Projectile or rival within 12 m | 10/6/2 |
| `black_hole` | 1 | 1.8 | 45 ahead | 3.5 s, pull r26, accel up to 22, also pulls owner | Growing lens | Nitro out, horn clears, shield halves pull | 2 or more rivals ahead within 80 m | 0/2/7 |
| `disco` | 1 | 1.4 | 20 ahead | 4 s, r22, rivals `steerWobble` (steer + 0.45 sin), lights change | Mirror ball, coloured beams | Exit radius, shield | Rival within 50 m | 8/6/4 |
| `glove` | 1 | 1.0 | 7, 40 deg | 0.12 s wind-up; launch vz 9, speed x0.75, dmg 6 | Spring | Distance, shield blocks | Rival within 7 m in cone | 0/5/6 |
| `harpoon` | 1 | 1.6 | 60, 30 deg | 2.5 s tow: owner +18 m/s² up to target speed +8, target power x0.6; ends under 5 m or at 80 m | Rope flight 0.25 s | Shield, cloak, walls break line when sampled offset differs over 1.2 x width | Next rival within 60 m | 0/2/8 |
| `cow` | 1 | 2.0 | predicted | Arcs 1.4 s to target position + 1.4 s lead; shock r9, speed x0.6, launch vz 6, dmg 12; cow trots off as animal | Growing shadow, `cow_launch` | Steer out, shield | Next rival within 180 m | 0/2/7 |
| `cloak` | 1 | 1.2 | self | 3 s: no collisions, immune, cannot fire | Translucent | Wait it out | Threat, or in last third | 0/2/6 |
| `overdrive` | 1 | 1.2 | self | 3 s drive x1.9, `vx` cap 105, wall damage x1.5 | Flames, glow | None; walls punish | Straight ahead with `curvature<0.01` | 0/0/10 |

`rollWeapon`:
- `rankFraction = (rank-1)/(field-1)`.
- Front if under 0.34, back if 0.67 or more, else mid.
- `pool` restricts ids and re-normalises.
- All weights sum to 100 per band.

`updateWeaponInput`:
- Handles the fire latch; `hold` weapons auto-repeat while `input.fire` is held.
- Cooldown, lock, fuel.
- Blocked while `cloak>0`.

### 2.10 Trick table (`AD.TRICKS`)

Constants: gravity `16`; `ROLL_RATE 7.0`, `PITCH_RATE 6.0`, `SPIN_RATE 9.0` (rad/s) x `airControl`; rate smoothing 12/s.

Air input:
- `steer` = roll, unless handbrake is held.
- Handbrake + steer = flat spin.
- `brake - throttle` = pitch. Up is front flip, which is negative pitch.
- With no input for an axis, roll and pitch return to the nearest multiple of 2π at 3.5 rad/s (auto-level).

Landing tolerance from `wrapAngle(roll)`, `wrapAngle(pitch)`, and yaw error between `psi` and travel direction:

| Quality | Rule | Effect |
| --- | --- | --- |
| `perfect` | each ≤ 0.30 and yaw ≤ 0.7 | points x1.25, nitro +0.2, burst 0.5 s x1.25 |
| `good` | ≤ 0.75 and yaw ≤ 1.4 | points x1, nitro +0.05 |
| `wobble` | ≤ 1.4 | points x0.5, `landWobble` 0.9 |
| `crash` | otherwise | points 0 + 50 consolation, `vx` x0.7, `landWobble` 1.4, damage +6, `pancake` call-out |

| id | Detection | Points | Nitro |
| --- | --- | --- | --- |
| `air_time` | Landed with `airTime ≥ 0.6` | 50 + 100 per second | 0.04 |
| `barrel_roll` | `abs(rollAccum) ≥ 2π − 0.35` | 400 | 0.12 |
| `double_barrel` | `abs(rollAccum) ≥ 4π − 0.35` | 900 | 0.25 |
| `front_flip` | `pitchAccum ≤ −(2π − 0.35)` | 500 | 0.15 |
| `back_flip` | `pitchAccum ≥ 2π − 0.35` | 500 | 0.15 |
| `flat_spin` | `abs(spinAccum) ≥ 2π − 0.5` | 350 | 0.10 |
| `spin_720` | `abs(spinAccum) ≥ 4π − 0.5` | 900 | 0.22 |
| `corkscrew` | Full roll and full flip in one flight | 1200 | 0.30 |
| `cow_vault` | Above an animal with `z ≥ 2.2` | 600 | 0.10 |
| `perfect_start` | Throttle held continuously 0.05 to 0.30 s at GO: burst 1.2 s x1.3. Over 0.8 s: wheelspin, 0.6 s power x0.6. | 300 | 0 |
| `drift_blue` | Drift ended with charge ≥ 1.0 s: burst 0.5 s x1.25 | 100 | 0 |
| `drift_orange` | Charge ≥ 2.2 s: burst 0.9 s x1.35 | 250 | 0 |
| `drift_pink` | Charge ≥ 3.6 s: burst 1.4 s x1.5 | 500 | 0 |
| `slingshot` | `draftTime ≥ 1.2` then slipstream under 0.05 at speed above 25: burst 0.8 s x1.25 | 150 | 0 |
| `reverse_hero` | `vx ≤ −8` for 2 s, then +100 per second | 200 | 0 |
| `j_turn` | From `vx ≤ −8` to `vx ≥ 8` within 1.5 s | 400 | 0.1 |

Details:
- A `trick` event is emitted when a rotation completes, and points are added to the flight chain.
- Chain points are banked on landing, multiplied by landing quality and by the Couch quirk, through `AD.addStylePoints(car, pts, true)`.
- A rotation trick does not double-pay: `double_barrel` replaces `barrel_roll`; `spin_720` replaces `flat_spin`.
- Drift charge uses `car.style.drifting`, with a 0.18 s end grace. A wall hit or takeoff zeroes the charge.

AI tricks (`AD.aiAirInput`):
- At takeoff, `fs.rng` picks a plan: 40% none, 40% roll if `2*vz/16 ≥ 0.95`, 20% spin.
- Every plan releases input so auto-level recovers before landing.

### 2.11 Locale key scheme

All keys sit in the flat `ui/lang/desktop/<lang>.json` files, next to the existing `aura_drift.*` block. The existing 45 keys stay, except that `controls_hint` is rewritten. Add 292 new keys:

| Group | Pattern | Count |
| --- | --- | --- |
| Modes | `aura_drift.mode.{drift_attack,elimination,hot_potato}.name`, `aura_drift.mode.<7 ids>.desc` | 10 |
| Cars | `aura_drift.car.<id>.tag`, `.quirk`; `aura_drift.stat.{power,grip,mass,top,air}` | 21 |
| Drivers | `aura_drift.driver.<id>.bio` | 8 |
| Places | `aura_drift.biome.<id>`, `aura_drift.track.<id>.blurb` | 16 |
| Weapons | `aura_drift.weapon.<id>.name`, `.hint` | 36 |
| Tricks | `aura_drift.trick.<16 ids>`, `aura_drift.landing.{perfect,good,wobble,pancake}` | 20 |
| Result titles | `aura_drift.title.<16 ids>` | 16 |
| Medals | `aura_drift.medal.{bronze,silver,gold,none}` | 4 |
| HUD | `aura_drift.hud.{final_lap,lap_of,eliminated,next_out,bomb,have_bomb,pass_it,clock,score,perfect_start,wheelspin,slingshot,drift1,drift2,drift3,near_miss,boost,strike,cow_bowling,respawn,shield,frozen,toast,disco,cloak,overdrive,wrong_way,sweeper}` | 28 |
| Settings | `aura_drift.settings.{camera,camera_chase,camera_high,camera_drone,camera_bumper,car,livery,tab_mode,tab_car,tab_track,tab_settings,tab_controls,loading_assets,fallback_note,credits,reset_bests}` | 16 |
| Controls | `aura_drift.controls.{steer,throttle,brake_reverse,handbrake,nitro,fire,camera,horn,pause,restart,air_roll,air_flip,air_spin,drift_turbo}` | 14 |
| Weather, surfaces | `aura_drift.weather.<9 kinds>`, `aura_drift.surface.<9 types>` | 18 |
| Road signs | `aura_drift.sign.<16 ids>` (for example `cow_priority`, `brake_is_cowardly`, `sofa_lane`, `gnomes_watching`) | 16 |
| Banter | `aura_drift.banter.<driver>.{pass,passed,hit,got_you,wreck,win}`, `aura_drift.marshal.<1..12>` | 60 |
| Misc | `aura_drift.daily_route`, `aura_drift.result.{score,tricks,air_time,animals,smashed,cup_total,eliminated_at,survived}` | 9 |
| **Total new** | | **292** |

German and English are written by the T5.4 implementer, with real comic tone, not literal. Examples:
- `trick.barrel_roll` DE "Fassrolle" / EN "Barrel Roll"
- `landing.pancake` DE "Pfannkuchen!" / EN "Pancake!"
- `title.brake_coward` DE "Bremsen ist für Feiglinge" / EN "Brakes Are For Cowards"
- `hud.cow_bowling` DE "Kuh-Bowling!" / EN "Cow Bowling!"
- `weapon.mg.name` DE "Wurst-MG" / EN "Sausage Gun"

The other 14 languages are properly translated, including jokes, and are not machine-literal. Dynamic keys are not seen by `TestDesktopUsedI18nKeysExistInAllLanguages`, so the coverage test below is authoritative.

---

## 3. GLB asset contract

### 3.1 Files under `ui/3d/aura-drift/v1/`

| File | Content | Tri budget | Byte target |
| --- | --- | --- | --- |
| `car-vektor.glb` ... `car-bertha.glb` (`car-<id>.glb`, 8) | One car, including reviewed vertex-colour reconstruction | ≤ 30000 each | ≤ 1.25 MiB each |
| `items.glb` | `item-crate`, `item-gnome_mine`, `item-rubber_duck`, `item-rocket`, `item-bratwurst`, `item-boxing_glove`, `item-disco_ball`, `item-harpoon`, `item-air_horn`, `item-frost_grenade` | ≤ 12000 total | ≤ 450 KB |
| `track.glb` | `track-ramp`, `track-boost_pad`, `track-cone`, `track-hay_bale`, `track-barrel`, `track-tyre_stack`, `track-barrier_segment`, `track-gantry` (children `light_1`..`light_5`), `track-rumble_bars` | ≤ 12000 | ≤ 450 KB |
| `animals.glb` | `animal-cow`, `animal-sheep`, `animal-penguin`, `animal-crab`, `animal-llama` | ≤ 1800 each | ≤ 320 KB |
| `shared.glb` | `prop-grandstand` (with crowd vertex colours), `prop-fan_camp`, `prop-hoarding` (child `face`), `prop-marshal_post`, `prop-gnome_statue`, `prop-hotdog_stand`, `prop-camper_van`, `prop-inflatable_arch`, `prop-road_sign` (child `face`), `prop-balloon_bunch`, `prop-photo_tower` | ≤ 36000 | ≤ 1.75 MiB |
| `scenery-alpine.glb` | `pine_a`, `pine_b`, `rock_a`, `rock_b`, `chalet`, `cablecar_pylon`, `flower_meadow`, `hay_barn`, `trail_marker` | ≤ 30000 each file | ≤ 1.1 MiB each |
| `scenery-desert.glb` | `cactus_a`, `cactus_b`, `mesa_rock`, `water_tower`, `windmill_pump` (child `rotor`), `rock_arch`, `palm_oasis`, `sunshade` | | |
| `scenery-harbour.glb` | `container_stack`, `crane`, `lighthouse`, `fishing_boat`, `neon_sign`, `tugboat`, `buoy_cluster`, `signal_mast` | | |
| `scenery-salt.glb` | `salt_crystal`, `ufo`, `rover`, `flag`, `radar_dome`, `geyser_pool` | | |
| `scenery-forest.glb` | `oak`, `birch`, `mushroom_cluster`, `log_pile`, `hunting_hide`, `fern_glade`, `watermill` | | |
| `scenery-industry.glb` | `silo`, `smokestack`, `pipes`, `forklift`, `pylon`, `cooling_tower`, `chem_tank` | | |
| `scenery-glacier.glb` | `ice_block`, `ice_arch`, `snow_pine`, `igloo`, `research_station`, `ice_spire`, `snow_cat` | | |
| `scenery-metro.glb` | `skyscraper_a`, `skyscraper_b`, `skyscraper_c`, `billboard` (child `face`), `street_lamp`, `food_truck`, `taxi`, `metro_entrance`, `arcade_kiosk`, `holo_arrow` | | |

Scenery nodes are named `prop-<name>`. The polish round added six props from `kit/polish_props.py`:
`balloon_bunch` and `photo_tower` (shared, placed as furniture), `trail_marker` (alpine), `sunshade`
(desert), `signal_mast` (harbour) and `holo_arrow` (metro). Per-prop tri budget: tag `scatter` ≤ 450,
default ≤ 1200, `hero` ≤ 4500, `crowd` ≤ 14000. Total pack ≤ 18 MiB (18874368 bytes). Also
`manifest.json` and `LICENSE.txt` (MIT). The exact node lists live in
`scripts/test-aura-drift-assets.mjs`; the shipped pack is 20 GLBs / 9 041 340 bytes (8.62 MiB).

### 3.2 Frames and scale
- glTF frame: Y up, metres.
- Cars and items: nose on +X, right side +Z (left is −Z), origin at ground centre, wheels resting on y=0.
- Props: base at y=0, footprint centred, front toward +X.
- Runtime applies `rotation.order='YZX'` on the car group: `rotation.y=-psi`, `rotation.z=pitch`, `rotation.x=roll`.
- Wheels use order `'YXZ'`: `rotation.y=-steer` (front only), `rotation.z=-spin`.
- Typical car true size: Vektor 4.4 x 1.9 x 1.15; Softeis 5.0 x 2.2 x 2.6 (cone to 3.8); Mähkönig 2.4 x 1.3 x 1.5; Couch 3.0 x 1.6 x 1.8 (lamp to 2.6); Bertha 5.2 x 3.0 x 3.0.
- Runtime applies `visualScale` to the whole group (toy proportions).

### 3.3 Car nodes and materials

Required top-level nodes:
- `body`, `wheel_fl`, `wheel_fr`, `wheel_rl`, `wheel_rr`. Wheel origin sits at the wheel centre, axle along Z.
- Empties: `anchor_exhaust_l`, `anchor_exhaust_r`, `anchor_head_l`, `anchor_head_r`, `anchor_tail_l`, `anchor_tail_r`, `anchor_weapon` (roof or front mount).

Optional animated children under `body`, driven at runtime if present:
- Softeis: `cone` (wobble).
- Mähkönig: `blade` (spin), `gnome` (lean).
- Couch: `lamp` (swing), `cushion`.
- Bertha: none.

Materials in the `body` mesh (exact names):
- `paint` (white base; runtime sets `.color` to the livery)
- `paint_accent` (white base; accent colour)
- `trim`, `glass`, `chrome`
- `head_light` (emissive `#fff2cc`, strength 3)
- `brake_light` (emissive `#ff2a1a`, strength 2)

At most 7 materials per car. Wheels: one material `tyre`, with rim and hub tone from vertex colour.

Art: chunky low poly with flat-shaded facets (`face.smooth=False`) on bodies, rounded tyres, a generous bevel on main edges, no recreation of any real or game car. Softeis has a giant cone on the roof. Mähkönig has a gnome driver. Couch is a sofa on wheels with a floor lamp. Bertha is a monster truck.

### 3.4 Animals

`animal-<kind>` is an empty with children `body`, `head`, `tail`, `leg_fl`, `leg_fr`, `leg_rl`, `leg_rr`, with pivots at joints. Exception: the crab's six legs merge into `leg_fl`, `leg_fr`, `leg_rl`, `leg_rr`. No clips. The runtime swings legs with sine and bobs the head. `cow` doubles as the catapult projectile with limbs rigid.

### 3.5 Manifest and validator

`manifest.json`:

```
{ "version":1, "blender":"5.2.2", "generator":"assets/aura-drift/generate.py", "totalBytes":N,
  "files":[{"name":"car-vektor.glb","bytes":n,"sha256":"...","triangles":n,"kind":"car"}, ...],
  "cars":{"vektor":{"file":"car-vektor.glb","length":4.4,"width":1.9,"height":1.15,"wheelRadius":0.36,"wheelbase":2.7}, ...},
  "items":{"crate":{"file":"items.glb","node":"item-crate","radius":1.0}, ...},
  "track":{"ramp":{"file":"track.glb","node":"track-ramp","radius":6}, ...},
  "animals":{"cow":{"file":"animals.glb","node":"animal-cow","length":2.2,"height":1.5}, ...},
  "props":{"shared":{"grandstand":{"file":"shared.glb","node":"prop-grandstand","radius":14,"height":12,"tags":["hero"],"triangles":n}}, "alpine":{...}, ...} }
```

Tags: `hero`, `tall`, `scatter`, `far`.

`scripts/test-aura-drift-assets.mjs` (no dependencies; adapted from `scripts/test-desktop-theme-orbit-assets.mjs`) checks:
- GLB magic, header length, manifest bytes, hash, triangles.
- Budgets per file and pack, and required nodes and materials per kind.
- No `images`, `textures`, `samplers`, `animations` or `skins`.
- Emissive factor non-zero on `head_light`, `brake_light`.
- Exact file set.

### 3.6 Runtime loading and fallback

`AD.assets.loadCore()` fetches the manifest, all car GLBs, `items`, `track`, `animals` and `shared` with concurrency 4. `AD.assets.loadBiome(id)` fetches one scenery kit.

Every failure is caught and counted. Every getter always returns a template:
- `car(id)` returns a stand-in group built from primitives with the same node names (`body` with `paint`, `trim`, `glass` materials, four cylinders as wheels, anchors). The same animation code works.
- Items, animals and track pieces return boxes, spheres and cones (`AD.STAND_INS` table in `assets.js`, with colour and size per name).
- Props return a cone or box from manifest `radius` and `height`, or the table default if the manifest is missing.
- A stand-in is never an error. It only increases `stats().standIns`.

**Load timing.**
- Shell view goes to `'loading'`: wait for `Promise.race([all(core, biome), timeout 4 s])`, then create the race and start the countdown.
- Late biome kits trigger `props.rebuild()`.
- The sim clock does not run while loading.
- Templates are CPU objects shared across windows with a refcount. Per-track objects are owned and disposed by each renderer.

**`AD.QUALITY[tier].gl`:**

| Key | low | medium | high | ultra |
| --- | --- | --- | --- | --- |
| `dpr` | 1 | 1.25 | 1.5 | 2 |
| `shadows` (map size) | 0 | 1024 | 2048 | 2048 |
| `shadowBox` (m) | 0 | 90 | 140 | 180 |
| `post` | false | true | true | true |
| `bloom` | off | half res | full | full |
| `aa` | none | fxaa | smaa | smaa |
| `msaa` | 0 | 0 | 0 | 4 |
| `gtao` | false | false | false | true |
| `particles` | 300 | 900 | 2000 | 3500 |
| `skid` (quads) | 800 | 2000 | 3000 | 4000 |
| `props` (max instances) | 300 | 700 | 1400 | 2200 |
| `propRadius` (m) | 220 | 320 | 480 | 700 |
| `terrainCell` (m) | 16 | 8 | 8 | 8 |
| `lights` (pool) | 0 | 2 | 4 | 6 |
| `clearcoat` | false | false | true | true |
| `envSize` | 64 | 128 | 256 | 256 |
| `weather` | 0.4 | 0.7 | 1 | 1.3 |
| budget `calls` | 120 | 180 | 260 | 350 |
| budget `triangles` | 250k | 500k | 900k | 1.5M |

Low tier also gets blob shadows only. Changing the tier rebuilds lights and shadows at the next `setQuality`, never mid-frame.

**Governor** (replaces D2):
- Start tier `medium`.
- 2 s windows of rAF deltas.
- Down one tier after 2 consecutive windows with average above 22 ms.
- Up one tier after 3 consecutive windows with average under 17.6 ms and 95th percentile under 20 ms.
- 12 s cooldown after any change.
- A tier that failed is blocked for 60 s, doubling each repeat.

Software GL (renderer string matches `/swiftshader|llvmpipe|software/i`) on `auto` uses the 2D renderer. On an explicit tier it uses 3D.

---

## 4. Implementation batches

Dependency order:

```
B0 -> (B1, B2 parallel) -> B3 -> B4 -> B5 -> B6 -> B7
T6.1 (audio sourcing list) may run right after B0 to give the user time.
```

Within a batch, parallel tasks own disjoint files as listed. Run at most 3 or 4 agents at once. Each batch ends with one reader-agent review. All code is written by maker-agent, tests run by test-agent, docs by docs-agent.

### Batch 0: Foundation (serial)

**T0.1 Runtime, loader, stubs.** Owns: `assets/aura-drift/runtime-entry.mjs`, `scripts/build-aura-drift-vendor.mjs`, `ui/js/vendor/aura-drift/*`, `ui/js/desktop/core/module-loader.js`, `package.json`, `.gitignore`, `ui/desktop_aura_drift_test.go`, all new `ui/js/desktop/apps/aura-drift-*.js` stub files.

Steps:
1. Write entry and build script (section 1.6). Build.
2. Replace the loader `scripts` list with section 1.4, one line each. Stubs for modules 6 to 11, 13 to 16, 20 to 25, 27, 29 to 31 and 35 are `/* Aura Drift - <role>. */\n(function () {\n    'use strict';\n})();\n`. Keep `aura-drift-post.js` as is for now.
3. Add to `package.json`:
   - `"build:aura-drift": "node scripts/build-aura-drift-vendor.mjs"`
   - `"test:aura-drift": "node scripts/test-aura-drift-physics.mjs && node scripts/test-aura-drift-tracks.mjs && node scripts/test-aura-drift-balance.mjs"`
   Later tasks do not touch `package.json`; T7.1 adds the final list.
4. Add `/assets/aura-drift/build/` to `.gitignore`.
5. Update `TestAuraDriftAppRegistration` and `TestAuraDriftShipsNoExternalResources` module lists to the full list. Add an assertion that `'/js/vendor/aura-drift/three-runtime.js'` precedes `aura-drift-core.js` in the entry. Implement the `aura-drift-assets.js` `fetch(` exception (section 1.6).

Acceptance:
- `node scripts/build-aura-drift-vendor.mjs --check`
- `node scripts/build-ui-bundles.js --check` (if stale, run without `--check` and commit)
- `go test ./ui -run '^TestAuraDrift' -count=1`
- `node scripts/test-aura-drift-physics.mjs`

**T0.2 Harness and survey.** Owns: `ui/testdata/aura-drift-harness.html`, `scripts/aura-drift-dev-server.mjs`, `ui/desktop_aura_drift_survey_browser_test.go`.

Implement section 1.7. Until `debug` exists, a scene just opens the menu; the survey test guards with `if (window.AuraDriftApp.debug)`.

Acceptance: the survey command with `AURAGO_AURA_DRIFT_SCENES=menu` writes a PNG; read it.

### Batch 1: Simulation and node tests

**T1.1 Sim core (serial, first).** Owns: `core`, `physics`, `track`, `style`, `conditions`, `race`, `ai`, `scripts/aura-drift-sim-loader.mjs`, `scripts/test-aura-drift-physics.mjs`, `scripts/test-aura-drift-tracks.mjs`, `scripts/test-aura-drift-balance.mjs`. Writes first working no-op exports into `tricks`, `features`, `weapons`, `projectiles`, `modes` with the exact section 1.5 signatures.

Steps:
1. Core:
   - `emit`, `fmt`, CARS table (section 2.7), DRIVERS, `driverField`, QUALITY `gl` block, governor, tape codec, bests v2.
   - Move `AD.WEAPONS` out of core. For this task, put the full table into `weapons.js` so the HUD keeps working; `name` field removed, HUD temporarily reads `def.id`. T5.1 translates.
2. Physics: section 2.2 (mass, grip, burst, wobble, frozen, overdrive, `resolveWall` in air, mass pair). Fixes D3. The `steer` wobble and `frozen` are applied to `effective` input in `stepActor`.
3. Conditions: delete `applyTarget`'s overwrite (D1):
   ```
   state.kind = next.kind; state.targetIntensity = next.intensity;
   ```
4. Track: `curvature`, `straightWindows`, crate rows, `trackName`; daily name uses a key at presentation.
5. Style: `addStylePoints`; `near_miss` emits one event per participating player with `other`.
6. Race: extract AI into `ai.js` unchanged in behaviour, plus hooks:
   - `AD.featureLaneBias` adds to `lane`.
   - `AD.aiWantsFire` replaces the timer branch.
   - `AD.modeAiOverride`.
   - `AD.aiAirInput` when airborne.
   - AI slipstream-safe `car.ai` values come from the driver.
7. Race: rewrite `stepRace` exactly as section 2.1, `createRace` per section 1.5 (clamp opponents 0..5, grid order, drivers, `tally`). Ghost uses a stub actor with `{v, style:null, tricks, tally:null, ghost:true}`, `race.muteEvents=true` during its step, slipstream forced 0 (D4), own `fs` copy. Tape cap 72000 (D5).

Tests:
- Keep all existing physics checks. Load through `scripts/aura-drift-sim-loader.mjs` (`export function loadSim() -> AD`, loading modules 1 to 12 in order into a vm context).
- Balance test temporarily `opponents:3`, bounds unchanged.
- Add one physics check: `AD.encodeTape`/`decodeTape` round-trip on 5000 random ints from `AD.rng(1)`.
- Add one physics check: a time-trial run with a scripted input tape, then a second race replaying `result.tape` as ghost; ghost `x,y` at the same step equal the player's within `1e-6` (D4 regression).

Acceptance: all three scripts pass.

**T1.2 Tricks (parallel).** Owns `tricks`, `scripts/test-aura-drift-tricks.mjs`.

Implement section 2.10 including perfect start, drift turbo, slingshot, reverse, `j_turn`, `launch`, `stepAir` with `wx,wy` integration and landing conversion:

```
v.vx = wx*cos(psi)+wy*sin(psi); v.vy = -wx*sin(psi)+wy*cos(psi)
```

Test checks:
- Launch at 50 m/s stays in the air about `2*vz/16` s.
- Steer +1 for a full flight gives `barrel_roll` with `rollAccum>0`.
- Up gives negative pitch.
- All four landing qualities reachable.
- Each drift level produces its burst.
- Perfect start window.
- Determinism hash equal across two runs.

**T1.3 Features (parallel).** Owns `features`, `scripts/test-aura-drift-features.mjs`.

Implement section 2.4.

Test checks:
- For all 8 tracks and 20 daily seeds, `validateFeatures` ok.
- Same seed gives the same feature list.
- A scripted car crossing a ramp becomes airborne.
- Pad sets `burst`.
- Animal position is pure in time.
- A hit animal lands and is back in its cycle.
- Destructible respawns after 20 s.
- `rules` toggles remove features.

**T1.4 Weapons (parallel).** Owns `weapons`, `projectiles`, `scripts/test-aura-drift-weapons.mjs`.

Implement sections 2.5 and 2.9 including AI predicates. `hit` always has `car` and `by` (D7).

Test checks:
- `rollWeapon` over 100000 draws matches weights within 1.5 points per band, and `overdrive` never appears in front.
- Every weapon fired in a 2-car scripted race emits its telegraph and effect events and changes state.
- Shield blocks everything except the railgun break rule.
- Cloak immune.
- Cow never removes anything, only trots.
- Replay hash equal.

**T1.5 Modes (parallel).** Owns `modes`, `scripts/test-aura-drift-modes.mjs`.

Implement section 2.8, `buildResult`, and tally-derived `medal`. `title` is set by shell via banter.

Test checks:
- Elimination ends within 200 s with 5 out.
- Hot potato ends within 150 s, bomb always has one holder or a pending delay.
- Drift attack ends at 90 s exactly (±`AD.STEP`) with no opponents.
- Cup points.
- Sweeper ends 45 s after the first finisher when player slot is last.

**T1.6 Balance (serial, last).** Owns `scripts/test-aura-drift-balance.mjs`, and `core` CARS numbers and `ai` tuning only. Implements section 5.3.

Acceptance: `node scripts/test-aura-drift-balance.mjs`, plus all sim scripts above.

### Batch 2: Blender kit (parallel with batch 1)

**T2.0 Kit core (serial, first).** Owns:
- `assets/aura-drift/generate.py`, `assets/aura-drift/kit/__init__.py`, `core.py`, `shapes.py`, `materials.py`, `export.py`, `manifest.py`
- `assets/aura-drift/README.md`, `assets/aura-drift/AGENTS.md`
- `scripts/test-aura-drift-assets.mjs`
- `kit/cars_common.py`, `kit/car_vektor.py`

Rules:
- CLI: `D:\Blender 5.2\blender.exe --background --factory-startup --python <abs>/assets/aura-drift/generate.py -- --only cars,items,track,animals,shared,scenery-alpine,...|all --out <abs> [--preview <abs scratch dir>]`. Default `--out` is the absolute `ui/3d/aura-drift/v1`.
- `--only manifest` scans existing GLBs and rewrites `manifest.json` and `LICENSE.txt`, so parallel authors never edit the manifest. Metadata per piece lives in `kit/meta_<group>.py`.
- Author in the glTF frame and convert with `to_blender = (x, -z, y)`, like `assets/desktop-theme-orbit/generate.py`. Each builder accumulates verts and faces per material. Clear the whole scene and orphan data before each file.
- Colour: `Col` float attribute (domain POINT, separate verts per shape) read by a Color Attribute node into Base Color, so the exporter writes `COLOR_0`.
- Deterministic: `random.Random(fixed seed)`, `mathutils.noise` with fixed offsets, sorted iteration, no time.
- Export call as in `assets/desktop-theme-orbit/generate.py` `export_glb` (`export_format GLB`, `use_selection`, `export_apply`, `export_yup`, `export_materials EXPORT`, `export_image_format NONE`, `export_normals`, `export_texcoords True`, `export_tangents False`, `export_vertex_color ACTIVE`, extras, cameras, lights, animations, skins and morph off, filtered by supported options).
- UVs exist only on `face` meshes (0 to 1).
- Modules stay under about 400 lines.
- Preview option: EEVEE 1024x768 warm key light view to a scratch directory, never into the repo.

Acceptance:
- `car-vektor.glb` builds and `--only manifest` works.
- Two consecutive runs give equal SHA-256.
- Validator passes in partial mode (`AURAGO_AURA_DRIFT_ASSETS_PARTIAL=1` skips the exact-set check).
- Read the preview PNG. The car must read as chunky toy, glossy, nose on +X, wheels on the ground.

**T2.1 Cars (after T2.0).** Owns `kit/car_kobalt.py`, `car_lumen.py`, `car_tundra.py`, `car_softeis.py`, `car_maehkoenig.py`, `car_couch.py`, `car_bertha.py`, `meta_cars.py`. All 8 previews read at a glance.

**T2.2 Items, track, animals, shared (parallel).** Owns `kit/items.py`, `track_parts.py`, `animals.py`, `shared_props.py`, `meta_items.py`, `meta_track.py`, `meta_animals.py`, `meta_shared.py`.

**T2.3 Scenery A (parallel).** Owns `kit/scenery_alpine.py`, `scenery_desert.py`, `scenery_harbour.py`, `scenery_salt.py`, `meta_scenery_a.py`.

**T2.4 Scenery B (parallel).** Owns `kit/scenery_forest.py`, `scenery_industry.py`, `scenery_glacier.py`, `scenery_metro.py`, `meta_scenery_b.py`.

Batch acceptance (run once, at the end, by the controller):
- `generate.py -- --only all`
- `generate.py -- --only manifest`
- `node scripts/test-aura-drift-assets.mjs`
- A second full run gives identical hashes.
- Total at most 18 MiB. If over, reduce bevel segments on scatter props first.

### Batch 3: 3D renderer core

**T3.1 Camera, steering fix, 2D fallback (serial, first).** Owns `camera`, `render` (2D), `cars` (2D), `scenery` (cull only), `aura-drift.js`, `scripts/test-aura-drift-camera.mjs`.

Camera (pure): the heading follow is:

```
speed = hypot(worldVx, worldVy)
vdir  = speed > 4 ? atan2(worldVy, worldVx) : v.psi
blend = params.velBlend * clamp((speed - 6) / 24, 0, 1)   // reversing (vx < -0.5): blend = 0
if |wrap(vdir - psi)| > 2.1: blend = 1                    // spin: follow travel, no nausea
wanted = psi + wrap(vdir - psi) * blend
h += wrap(wanted - h) * (1 - exp(-params.follow * dt))
```

| mode | dist | height | ahead | targetY | fov | velBlend | follow |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `chase` | 9.5 | 3.7 | 7 | 1.0 | 60 | 0.45 | 5.5 |
| `high` | 15 | 8 | 12 | 0.5 | 56 | 0.35 | 4.5 |
| `drone` | 7 | 46 | 8 | 0 | 48 | 0.20 | 3.2 |
| `bumper` | -1.0 | 1.05 | 30 | 0.9 | 72 | 0 | 30 |

Pose:

```
fx=cos h, fy=sin h, d=dist+0.025*speed
eye    = [v.x-fx*d, v.z*0.6+height, v.y-fy*d]
target = [v.x+fx*ahead, v.z*0.85+targetY, v.y+fy*ahead]
fov   += 10*clamp((speed-20)/70,0,1) + 8*punch   // punch eases 7/s
shake: eye += right*n1*s*0.35 + up*n2*s*0.25, roll = n3*s*0.02   // s decays 2.4/s; n from a non-sim rng
pose = {eye, target, fov, roll, heading:h, simX:v.x, simY:v.y, scale2d}
```

Reduced motion: no shake, no FOV terms, `follow` x1.6, no scripts.

Projection:

```
f = norm(target-eye); right = norm([-f.z, 0, f.x]); up = cross(right, f)
apply roll to right and up
depth = dot(p-eye, f); t = tan(fov/2)
x = dot(p-eye, right)/(depth*t*aspect); y = dot(p-eye, up)/(depth*t)
```

Scripts:
- `intro` (3 s spline from high front to chase during countdown; skipped when reduced).
- `orbit` (finish; radius 12, height 4, 0.35 rad/s, static 3/4 view if reduced).

Time warp kinds: `hitstop` 0.06 s at 0.1, `slowmo` 0.35 s at 0.4, `finish` 1.0 s at 0.35.

2D fallback:
- `AD.screenRotation(h) = -PI/2 - h`.
- `AD.project2D`: `dx=(x-cam.x)*scale, dy=(y-cam.y)*scale; [w/2+dx*cos θ-dy*sin θ, h/2+dx*sin θ+dy*cos θ]`.
- `render.js`: wrap world drawing in `c.translate(w/2,h/2); c.rotate(θ); c.translate(-w/2,-h/2)`, restore before weather and vignette, and replace every rectangular screen cull with `onScreen(sx,sy,m) = hypot(sx-w/2, sy-h/2) < diag/2+m`.
- Chunk range uses `diag/(2*scale)` for both axes.
- `drawCar`, `drawFurniture` and `drawProp` culls take the same test (pass `onScreen`).
- Add simple shapes for new shapes in `cars.js` (`van`, `mower`, `sofa`, `truck`), and in `render.js` simple circles and glyphs for new hazards, projectiles, ramps, pads, crates and animals. Lift airborne cars by a shadow offset and scale.
- `render.js` implements the renderer interface (section 1.5), mapping events to its existing `burst`, `smoke`, `addDecal`.

Shell:
- Move `emitFx` into `render.js` as `emit(race, dt)`.
- Use `AD.createChaseCamera`, `createTimeWarp`, governor (D2), `readInput` in countdown too.
- Keys: `KeyC` cycles camera and saves it, `KeyH` horn, `KeyE` nitro in addition to Ctrl (D6).
- Debug API and full `stats()`.
- Shell still uses the 2D renderer and old post in this task.

Tests (`scripts/test-aura-drift-camera.mjs`, section 5.2).

Acceptance:
- `node scripts/test-aura-drift-camera.mjs`
- Survey `camera-chase` with `?quality=low`. Read the PNG: car nose points up the screen in 2D.

**T3.2 3D orchestrator (serial, second).** Owns `render3d`, `look`, `assets`, `actors` (cars, ghost, crates only), `aura-drift.js` (renderer selection), `ui/css/desktop-app-aura-drift.css` (only `.agdrift-gl` and removal of `.agdrift-post`).

`createRenderer3D`:
- Returns null if `window.AuraDriftThree`, `AD.LOOK` or WebGL2 is missing, or creation throws, or software GL on `auto`.
- `WebGLRenderer({canvas, antialias:false, alpha:false, powerPreference:'high-performance', stencil:false})`, `outputColorSpace=SRGB`, `toneMapping=NeutralToneMapping`, exposure from `LOOK`, shadow map `PCFSoftShadowMap`.
- Lights: `DirectionalLight` sun (per `LOOK`), shadow box follows the player snapped to texels; `HemisphereLight`; env from `PMREMGenerator.fromScene` of the sky at `gl.envSize`.
- Fixed pools: `gl.lights` PointLights at intensity 0 (count never changes within a tier).
- Scene graph: `terrain`, `road`, `sky`, `props`, `actors`, `fx`. Modules are called with `typeof AD.x === 'function'` guards, so T3.3, T3.4 and batch 4 need no edits here.
- `setTrack`: dispose all per-track objects, build new.
- `render`: update, sync camera from pose (`camera.fov`, `camera.up` with roll), `post.render` or `renderer.render`.
- Stats from `renderer.info` with `autoReset=false`, reset per frame. Compute `projection` with `Vector3.project(camera)` using `actors.carPoints(0)`.

Garage showcase (`setShowcase`, `showcase`): a separate small scene with a turntable disc, selected car and livery, warm key spot, rim light, biome-tinted backdrop gradient, a gnome statue, cone and tyre stack. Camera orbits at 0.25 rad/s (static under reduced motion).

Context loss:
- `webglcontextlost`: `preventDefault`, shell swaps to a lazily created 2D renderer, race continues.
- `webglcontextrestored`: rebuild env map and composer targets, `setTrack(current)`, swap back. On dispose: `renderer.dispose(); renderer.forceContextLoss()`.

Actors:
- Clone template (shared geometry), convert `paint` to `MeshPhysicalMaterial` with `clearcoat:1, clearcoatRoughness:0.08, roughness:0.28, metalness:0.35` when `gl.clearcoat`, else `MeshStandardMaterial` with `roughness:0.3`.
- Tint livery; drive wheels, steering, brake lamps (`emissiveIntensity` 1 braking, 0.25 otherwise), headlights (night biomes 1.6, day 0.6).
- Visual spring suspension: pitch from accel, roll from lateral g, max 0.06 rad.
- Blob shadow plane under each car always, height-faded in air. Ghost: transparent `opacity:0.35`, `depthWrite:false`.

Look table `AD.LOOK[biome]` (sun elevation in degrees; colours are hex):

| biome | time | sunEl | sun | skyTop | skyHorizon | fog | fogFar | exposure | bloom | grade tint |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| alpine | golden morning | 24 | `#ffd9a0` | `#5c8fd6` | `#ffd7ad` | `#d9c8b4` | 900 | 1.05 | 0.35 | warm |
| desert | late afternoon | 18 | `#ffb26b` | `#4a7bc4` | `#ffb07a` | `#e8b98a` | 1100 | 1.1 | 0.35 | orange |
| harbour | blue hour neon | 6 | `#8fb4ff` | `#141c44` | `#d36a7a` | `#2a3050` | 650 | 1.0 | 0.8 | teal-magenta |
| salt | low white sun | 14 | `#fff0d8` | `#6aa6e8` | `#ffd0c8` | `#f0e4dc` | 1400 | 1.15 | 0.45 | pastel |
| forest | dawn mist | 20 | `#ffd28a` | `#7aa890` | `#f4d9a8` | `#b8c8a4` | 420 | 1.0 | 0.3 | green-gold |
| industry | sulphur dusk | 8 | `#ff9a55` | `#2c2a3c` | `#d9703c` | `#55413a` | 600 | 1.0 | 0.7 | sodium |
| glacier | polar night, aurora | 10 | `#a8d0ff` | `#0a1630` | `#3a6a8a` | `#203248` | 750 | 1.05 | 0.7 | ice |
| metro | midnight neon | 4 | `#7f8cff` | `#0c0a22` | `#5a2a7a` | `#1a1436` | 520 | 1.0 | 0.95 | magenta |

Acceptance:
- Survey `biome-alpine`, `car-vektor`, `car-softeis` (cars on flat ground plane even without road or terrain).
- Stand-ins appear if GLBs are not built yet.
- `AuraDriftApp.stats()[0].renderer.kind === 'three'`.

**T3.3 Road and terrain (parallel with T3.4).** Owns `road`, `terrain`, `scripts/test-aura-drift-world.mjs`.

Road:
- Group by `AD.surfaceAt(track, s, ±1)` per sample and side, so look equals physics. One mesh per surface type, with lateral columns left, centre, right.
- Canvas-generated tile textures (512, once per biome; albedo plus roughness): asphalt with aggregate noise, patches, edge line, centre dash (period divides `length`, as `worldDash` does), gravel, sand, mud, snow, ice, salt, wet.
- World-anchored UVs: `u` across 0..1, `v = s/18` with total rounded to an integer so the seam closes.
- Kerbs at `halfWidth-1.2..-0.2`, red and white period 11 m, staggered sides, slightly raised (y=0.03).
- Shoulder ribbon `halfWidth..halfWidth+10`, y=-0.02, colour blending to terrain.
- Barrier meshes both sides at `halfWidth+0.05` following the corridor (D8), height 1.1 m. Style per biome: armco alpine, industry; tyre wall forest; concrete metro, harbour; snow bank glacier; low painted rope salt; rock wall desert.
- Checkered start line, gantry via actors or props, tyre-wear darkening. `setWetness(w)` lerps roughness from 0.85 to 0.18 and adds env intensity.
- `AD.conditions` wetness is computed in render3d: `rain` intensity plus biome wet base.

Terrain:
- `buildDistanceField(track, 8)`: stamp samples within 160 m into a Float32Array (distances beyond are clamped to 160, and `terrainHeight` adds a large-scale far term).
- Chunks 256 m; bounds = track bounds + 600 m.
- Chunks within 300 m of the track use `gl.terrainCell`, others 16 m at low detail; skirts hide cracks.
- Height: below `hw+10` it is -0.4; smoothstep up to full at `hw+60`, using `AD.TERRAIN` rules:

| biome | rule |
| --- | --- |
| alpine | hills amp 90 wavelength 900 + ridged peaks far up to 220 |
| desert | dunes amp 18 wavelength 240, terraced mesas beyond d 180 (quantise 30 m) |
| harbour | flat -0.4 on the inner side, drop to -3 under the water on the outer side |
| salt | flat -0.3, mountain ring beyond d 900 amp 120 |
| forest | rolling amp 28 wavelength 500 |
| industry | flat with embankments amp 6, slag heaps amp 30 beyond d 200 |
| glacier | ridged ice cliffs amp 70 |
| metro | flat -0.2 |

Vertex colours by height, slope and noise from `LOOK.terrain` palette; tiling 512 canvas noise detail texture; `flatShading:true` for hills so facets match the diorama look. Noise comes from a precomputed JS value-noise table, seeded.

Pure tests in `scripts/test-aura-drift-world.mjs`:
- Ribbon closes (first and last ring within 0.01 m, `v` integer).
- Distance field agrees with `AD.sampleTrack` within 8 m inside 100 m.
- `terrainHeight` is at most -0.2 inside the corridor for all biomes.

Acceptance: survey all `biome-*`. Road reads, no gaps, no z-fight, surface zones visible.

**T3.4 Sky, water, weather, props (parallel with T3.3).** Owns `sky`, `props`; adds its checks to `scripts/test-aura-drift-props.mjs`.

Sky:
- Dome radius 3000, follows camera, gradient plus sun disc plus low-frequency clouds from a 512 noise texture (texture lookups only), stars, aurora on glacier (2 scrolling noise bands), neon haze on metro and harbour.
- `setDisco(on)`: rotating colour tint on hemi light and additive beams.
- Harbour water plane y=-1.6 with ShaderMaterial (2 scrolling normal lookups, sky gradient reflection, sun glitter, emissive-tinted streaks). Salt flat: low-roughness ground, sky mirrored in the shader. No `Reflector`.

Weather (from `race.conditions.state`):
- Instanced streak quads around the camera. Rain up to 1500 x `gl.weather`, snow 1000, fog via `scene.fog` density, sand and sparks 400.
- Off or thinned under reduced motion, to a static fog veil.

Props:
- `AD.scatterProps`: deterministic from `AD.hashString(track.id+'/props/'+track.seed)`, per 100 m of track and side using `AD.SCATTER[biome]`. Candidate rejected if `|offset| < hw + 8`, or if tall (`height>3`) and inside `hw+18` on the inner side of a curve with `curvature>0.02`, which keeps sightlines.

`AD.SCATTER` (name, band metres from road edge, per 100 m of track and side):

| biome | entries |
| --- | --- |
| alpine | `pine_a` 10-160 x3, `pine_b` 14-180 x2, `rock_a` 8-120 x1.2, `rock_b` 20-200 x0.8, `chalet` 40-180 x0.08, `cablecar_pylon` 80-260 x0.05 |
| desert | `cactus_a` 8-140 x1.6, `cactus_b` 12-160 x1, `mesa_rock` 60-400 x0.2, `water_tower` 40-200 x0.05, `windmill_pump` 30-200 x0.08 |
| harbour | `container_stack` 12-140 x0.8, `crane` 60-240 x0.08, `lighthouse` 200-300 x0.02, `fishing_boat` 40-200 x0.15, `neon_sign` 8-60 x0.2 |
| salt | `salt_crystal` 8-200 x1.2, `rover` 30-200 x0.03, `flag` 15-100 x0.1, `ufo` set piece only |
| forest | `oak` 8-120 x3, `birch` 10-150 x2.5, `mushroom_cluster` 6-40 x1, `log_pile` 10-70 x0.2, `hunting_hide` 20-80 x0.06 |
| industry | `silo` 20-200 x0.2, `smokestack` 60-300 x0.1, `pipes` 10-80 x0.4, `forklift` 12-60 x0.1, `pylon` 40-260 x0.1 |
| glacier | `ice_block` 8-120 x1, `ice_arch` 30-160 x0.1, `snow_pine` 10-150 x1.8, `igloo` 20-120 x0.06, `research_station` 80-260 x0.03 |
| metro | `skyscraper_a` 16-90 x1.2, `skyscraper_b` 16-90 x1, `skyscraper_c` 30-160 x1, `billboard` 8-30 x0.2, `street_lamp` 6-8 x1, `food_truck` 8-30 x0.1, `taxi` 8-30 x0.2 |

`AD.furnitureProps`:
- Grandstands both sides of the start straight.
- Gantry at s=0 (lights driven from `race.countdown`).
- Hoarding every 200 m on straights, marshal post per crossing and 4 per lap, gnome statue every 500 m, 2 hot-dog stands, 4 camper vans, inflatable arch at sector lines.
- Silly signs from a `t`-driven 1024x512 canvas atlas (16 cells), max 16 sign meshes with cloned materials, placed 120 m before crossings and ramps.
- Hoardings and billboards take a canvas texture of fictional brands in `LOOK` colours.

Streaming:
- One `InstancedMesh` per prop primitive with capacity at the tier max.
- `update(pose)` refills every 0.25 s or 12 m moved, from a 128 m spatial hash, nearest first within `gl.propRadius` (tag `far` to 1.8x, `scatter` to 0.5x), capped at `gl.props`. `castShadow` only on `high`+ and for the nearest 60 m.
- Sway or spin for tagged nodes is presentation.

Pure test `scripts/test-aura-drift-props.mjs`:
- Same seed gives same placements.
- All outside corridor.
- Instance cap honoured by the filter function.

Acceptance: survey `biome-*`. Each biome distinct, no prop in the corridor; read PNGs against the look checklist below. Log `calls` and `triangles` within the tier budget at `medium`.

**Look checklist** (read PNGs):
- Horizon and sky visible.
- Warm directional light with long shadows on daylight biomes.
- Car glossy and readable against the road at 10 percent of the frame width.
- Props frame the road, none blocks it.
- Saturated, per-biome palette, unmistakably different from a generic grey racer.

### Batch 4: FX, post, weapon visuals (parallel)

**T4.1 FX.** Owns `fx`.

Two pools:
- Alpha and additive `InstancedBufferGeometry` billboards with per-instance `aPos`, `aSize`, `aColor`, `aAlpha`, `aRot`, `aFrame`.
- Atlas 512 canvas 4x4: puff, spark, flame, feather, confetti, ring, shard, drop, star, bolt, hex, leaf, plus 4 free.
- CPU update, capacity `gl.particles`, oldest recycled, randomness from `Math.random` only here.

Systems:
- Tyre smoke, dust and gravel by surface, snow, water spray, sparks (velocity-stretched), flames, explosions (flash light from the pool, shock ring mesh, debris), confetti, feathers.
- Tyre-lit drift turbo sparks in blue `#4fc3ff`, orange `#ff9a3c`, pink `#ff5ec4` at the rear wheels.
- Nitro flames plus pool light at `anchor_exhaust_*`.
- Rocket trails, tether rope and rail beam (additive quads, fading, 0.25 s), EMP and horn rings, flame cone (additive soft), UFO beam.

Skid ribbon:
- Ring buffer of `gl.skid` quads, y=0.02 with polygon offset, colour by surface.

`event(evt, race)` implements the Reaction column of section 2.6. `emit(race, dt)` does continuous emission for all cars within 200 m of the camera. No gore: hits on animals and props only produce stars, feathers and dust.

Acceptance: survey `weapon-*`, `trick-air`, read 6 representative PNGs.

**T4.2 Post.** Owns `post` (full rewrite; deletes `createPostProcessor`). Also removes the old WebGL2 pass.

Pipeline, in order, for tier `post:true`:
1. `RenderPass` (composer HalfFloat target, `samples = gl.msaa`)
2. `GTAOPass` (ultra, half resolution)
3. `UnrealBloomPass` (threshold 0.85, strength from `LOOK.bloom`, half or full)
4. `ShaderPass` final (grade lift, gamma, gain per biome, vignette 0.25, radial blur 6 taps only when `nitro>0`, chromatic aberration from `impulse('impact')` decaying 4/s, up to 2 black-hole lens warps at projected screen positions, low-health desaturation)
5. `OutputPass`
6. `SMAAPass` or `FXAAPass`

Reduced motion zeroes radial blur, aberration and lens wobble. Low tier: `available:true, enabled:false`, direct render. `stats()` keeps `{available, enabled, degraded:false, quality, biome}`.

Acceptance: survey `biome-metro`, `biome-alpine`, `weapon-black_hole` at `high` and at `low`.

**T4.3 Weapon and feature visuals.** Owns `actors` (extends T3.2).

- All `race.hazards`, `race.projectiles`, `race.beams`, `fs` features and animals rendered from section 3 GLBs by pooled instances.
- Bratwurst spin, ducks bob and hop, gnome blink (emissive pulse), disco ball spin with colour spots on the road, cow arc with leg flail, animals walk with leg swing, flying states with spin, ramps and pads, destructibles, rumble bars, gantry lights.
- Cloak (transparent shimmer on the car), overdrive glow, bomb on the holder's roof with a blinking fuse, shield bubble, frozen ice shell, toast soot tint, elimination comic poof.
- Crate spins with a pulse. Bob amplitude zero under reduced motion.

Acceptance: survey `weapon-*`. Each weapon is identifiable in its PNG.

### Batch 5: HUD, menu, humour, i18n

**T5.1 HUD (parallel with T5.2, T5.3).** Owns `hud`, `hud-fx`.

Layout at 1280x720, scaled by `min(w/1280, h/720)` with a compact variant under 660 px:
- Bottom left: speed (big number, km/h), rpm arc, gear, damage, nitro bar, drift-turbo bar with blue, orange, pink segments.
- Bottom right: weapon slot, 1.1 s roulette animation ticking through item glyphs (driven by `car.weaponLock`), ammo pips, cooldown ring, name from `weapon.<id>.name`.
- Top left: lap or clock, race time, sector splits (colour plus text, no colour-only state).
- Left: position tower up to 6 rows with colour chip, driver initials, gap, out strike-through, bomb icon.
- Top right: minimap via `AD.drawMinimap` (circular clip, rotated with `pose.heading` so up is forward, Path2D cached per track, rivals, ghost, crossing icons, hazards).
- Centre: countdown with 5 big lights; kinetic call-outs (scale overshoot 1.4 to 1, slight rotation, stacked to 3, trick chain total, reduced motion: fade only); warnings (rocket arrow, rail line, shadow, next out, bomb).
- Comms panel top centre with `AD.drawPortrait` (canvas-drawn helmet, visor, hair, moustache from `driver.portrait`), 4 s lifetime, name and text.

All text via `t`. `hud.event` handles the HUD reactions of section 2.6. Paused state: HUD cleared, as today.

Acceptance: survey `hud` plus `biome-glacier`.

**T5.2 Menu and shell flow (parallel).** Owns `ui`, `ui-cards`, `aura-drift.js`, `ui/css/desktop-app-aura-drift.css` (full rewrite). The live 3D garage shows through.

Structure:
- `.agdrift-overlay` has no blur and uses gradient scrims.
- `.agdrift-panel.agdrift-menu`: full-window grid: title block; tab row (`button.agdrift-tab`) Mode, Car, Track, Settings, Controls; content area; footer with `.agdrift-primary` start button and the `.agdrift-disclosure` catch-up line, always visible.

Cards:
- `button.agdrift-mode-card[data-mode]` (7), name and description.
- `button.agdrift-car-card[data-car]` (8), stat bars (text plus bar), tag, quirk, livery swatches. Selecting calls `callbacks.onShowcase({carId, livery, biomeId})`.
- `button.agdrift-track-card[data-track]` (8), canvas map thumbnail from `AD.trackPoints`, biome colour, blurb, best time. Keep `select.agdrift-laps` for laps. Disabled tracks in cup and daily.
- Settings: quality select, volume range, mute, weapons, reduced motion, ghost, camera select, reset bests (disabled when readonly).
- Controls table with every key including C, H, E and air tricks.

Other:
- `.agdrift-sr-only` hint retained, `aria-pressed` on selected cards, `:focus-visible`, arrow keys move within a card group.
- Loading view: `.agdrift-progress` with `settings.loading_assets`.
- Touch buttons: 8 at 56 px: left, right, gas, brake, fire, handbrake, boost (D6), camera. Pad container `aria-hidden` retained.
- Pause unchanged in spirit. Results: title from `AD.pickResultTitle`, medal, trick summary, tally lines, cup total, buttons.

Shell:
- `view` state machine `menu -> loading -> race -> results`.
- Passes `config.carId` and `livery`.
- `audio.setCar`, `labelsFor` kept for old keys, plus `t`.
- Bests with codec (save only if `tapeComplete`, D5).
- Cup points through `AD.cupPoints`; `cupScores` length 6.
- Ghost gets `ghostCar` from the best.
- `TestAuraDriftAvoidsAdBlockerClassPrefix` markers `agdrift-panel agdrift-menu`, `agdrift-primary`, `agdrift-touch-btn` stay in `aura-drift-ui.js`.

Acceptance:
- Survey `menu`, `results`.
- Keyboard: tab through to Start in a manual harness run via `debug`-free page. Verified by the T7.1 browser test.

**T5.3 Banter and titles (parallel).** Owns `banter`.

- Separate rng `AD.rng(seed ^ 0x51ed)`.
- Triggers: `position` (pass, passed), `hit` (hit, got_you), `landing` crash or `wall_hit` heavy (wreck), `finish` first (win), `respawn` and crossing warnings (marshal).
- Rate limit: 1 per 7 s, same driver not twice in a row, max 1 line per trigger type per 20 s.
- Never affects sim.

`AD.pickResultTitle`, first match wins:

| Order | id | Rule |
| --- | --- | --- |
| 1 | `pacifist` | `weaponsFired==0`, weapons on, position 1 |
| 2 | `brake_coward` | `brakeSeconds < 2%` of time |
| 3 | `cow_bowler` | `animalHits ≥ 5` |
| 4 | `air_force` | `airTime ≥ 20` |
| 5 | `pancake_chef` | `crashLandings ≥ 3` |
| 6 | `wall_lover` | `wallHits ≥ 25` |
| 7 | `drift_king` | `driftSeconds ≥ 35%` of time |
| 8 | `reverse_hero` | `reverseSeconds ≥ 20` |
| 9 | `sausage_baron` | `sausageHits ≥ 12` |
| 10 | `disco_fever` | `discoFired ≥ 2` |
| 11 | `gardener` | Mähkönig and `smashed ≥ 15` |
| 12 | `ice_man` | Softeis, position ≤ 3 |
| 13 | `sofa_king` | Couch, position ≤ 2 |
| 14 | `big_bertha` | Bertha and `carHits ≥ 15` |
| 15 | `last_but_loud` | Last and `hitsDealt ≥ 3` |
| 16 | `solid_driver` | Default |

Test: a few checks appended to `scripts/test-aura-drift-modes.mjs` by T5.3 are not allowed (other owner). So add `scripts/test-aura-drift-banter.mjs` with rate limit and title priority.

**T5.4 Locale DE and EN (serial, after T5.1 to T5.3).** Owns `ui/lang/desktop/de.json`, `ui/lang/desktop/en.json`, `ui/desktop_aura_drift_test.go` (locale test).

Add all 292 keys, inserted after the last `aura_drift.` key in each file, UTF-8 literal. Update `controls_hint` (mention E for nitro, C camera) and remove nothing else. Update `TestAuraDriftLocaleKeysCoverAllLanguages` (section 5.1).

**T5.5 to T5.8 Translations (parallel; 3 or 4 languages each; each owns only its files).**
- T5.5: `cs, da, nl, no`
- T5.6: `sv, pl, fr, es`
- T5.7: `it, pt, el`
- T5.8: `hi, ja, zh`

Rules:
- Copy the key set from `en.json`, with idiomatic comic tone from both DE and EN.
- Keep `{...}` placeholders.
- Keep car and driver proper names.
- JSON stays valid, literal UTF-8, no mojibake.

Batch acceptance:
- `go test ./ui -run '(TestAuraDrift|TestDesktopUsedI18nKeysExistInAllLanguages)' -count=1 -v`, count PASS lines.

### Batch 6: Audio

**T6.1 Sourcing list (no downloads, then STOP).** Returns to the controller a table: group, proposed work, author, page URL, download URL, license proof line, file size. The controller shows it to the user and waits for approval.

Candidates to check first: Kenney Impact Sounds (already pinned in `assets/minigolf/audio/sources.json`), other Kenney CC0 packs, OpenGameArt CC0 entries. Reject anything not explicitly CC0.

Coverage needed:
- `impact.light`, `impact.heavy`, `impact.metal`
- `scrape.metal`, `tyre.squeal` (loop)
- `explosion.small`, `explosion.big`
- `crate.break`, `prop.hay`, `prop.cone`, `prop.barrel`, `prop.tyres`
- `crowd.cheer`, `crowd.aww`
- `animal.cow`, `animal.sheep`, `animal.penguin`, `animal.llama`, `animal.crab`
- `amb.wind`, `amb.sea`, `amb.forest`, `amb.machinery`, `amb.city`, `amb.snow`, `amb.desert`

Anything without an approved source falls to own synthesis. The game never blocks on this.

**T6.2 Build pipeline (after approval).** Owns `assets/aura-drift/audio/sources.json`, `assets/aura-drift/audio/README.md`, `scripts/build-aura-drift-audio.mjs`, `scripts/aura-drift-synth/*.mjs`, `scripts/test-aura-drift-audio.mjs`, `ui/3d/aura-drift/audio/**`.

Modelled on `scripts/build-minigolf-audio.mjs` (same `--check`, `--write --sources DIR`, hash-pinned sources, archive safety, bit-exact Vorbis) with a new step: `--write` also renders synth recipes to float WAV with seeded JS DSP and encodes them.

Output paths match `^(sfx|engine|music)/[a-z0-9-]+\.ogg$`. Manifest:

```
{ version:1, sfx:{group:[paths]}, engine:{carId:{idle,low,mid,high}}, music:{biome:{bed,drive,peak}, menu, results}, files:[...], sources:[...] }
```

Own synthesis renders:
- Engine loops: 8 cars x 4 layers (`idle` 1000 rpm, `low` 2600, `mid` 4600, `high` 6600), 2 s mono seamless. Character per car: wedge turbo, GT six, muscle V8, rally boxer, van diesel, mower two-stroke, sofa electric whine with squeaks, truck big diesel.
- Weapons: `weapon.oil`, `mine_drop`, `emp_charge`, `emp_pulse`, `rocket_fire`, `rocket_lock`, `mg`, `shield_on`, `shield_hit`, `duck_launch`, `duck_quack`, `rail_charge`, `rail_fire`, `flame_loop`, `frost_throw`, `frost_burst`, `horn_blast`, `hole_loop`, `disco_on`, `glove`, `harpoon_fire`, `harpoon_clank`, `cow_launch`, `cow_land`, `cloak_on`, `overdrive_on`.
- Other: `sting.perfect`, `sting.pancake`, `sting.strike`, `sting.slingshot`, `sting.moo`, `sting.trick`, `turbo.1/2/3`, `turbo.tick`, `pad.boost`, `ramp.launch`, `land.soft`, `land.hard`, `roulette.tick`, `roulette.done`, `bomb.tick`, `bomb.blast`, `jingle.softeis`, `horn.<carId>`, `ufo.beam`, `ui.count`, `ui.go`, `ui.lap`, `ui.bank`, `whoosh`, plus any coverage group without an approved source.

Music (own arrangements, because dynamic layers need aligned stems):
- 8 biomes x 3 layers (`bed`, `drive`, `peak`), 16 bars at the tempi in `aura-drift-music.js` (alpine 100, desert 118, harbour 92, salt 106, forest 96, industry 128, glacier 84, metro 122), stereo, exactly aligned and seamless.
- Plus `menu`, `results`, `stinger.final_lap`, `stinger.disco`, `fanfare.win`, `fanfare.podium`, `fanfare.lose` (sad comic trombone).
- Reuse scales and patterns of `aura-drift-music.js`, richer: sidechain-style pumping, filtered pads, real drum synthesis.

Budgets enforced by `--check`: `sfx` ≤ 4 MiB, `engine` ≤ 1.5 MiB, `music` ≤ 10 MiB, manifest ≤ 128 KiB.

Normalisation: sfx peak -1 dBFS, engine peak -3, music -16 LUFS. Use the earshot `measure` tool if available to sanity-check loudness.

Acceptance:
- `node scripts/build-aura-drift-audio.mjs --check`
- `node scripts/test-aura-drift-audio.mjs` (manifest groups match `AD.AUDIO_GROUPS` by regex scan of `aura-drift-audio.js`, engine set complete, music layer durations equal within 1 ms)
- A rebuild gives identical hashes.

**T6.3 Audio runtime (after T6.2).** Owns `audio-bank`, `engine`, `audio`.

Bank:
- `AD.assets.fetchJson('/3d/aura-drift/audio/manifest.json')`, `fetchBytes` per path, `decodeAudioData`, lazily by priority (engine for the selected car and rivals, then sfx, then ambience). Failures counted.
- `play(group, {gain, rate, pan, bus, loop})` with a 32-voice cap, random member.
- Missing group: run the `AD.SYNTH_FALLBACK[group]` recipe (existing `tone` and `noiseHit` patterns).

Engine:
- Four looped `AudioBufferSourceNode`s for the player with equal-power crossfade across rpm, `playbackRate = clamp(rpm/layerRpm, 0.7, 1.3)`, throttle shaping lowpass, limiter bounce kept.
- Rivals: `mid` layer of each rival's car, 5 voices, existing distance gain, low-pass, Doppler and stereo pan.
- Existing oscillator stack is the fallback when samples are missing.
- Tyres: `tyre.squeal` loop driven by slip, surface layers retained, `scrape.metal` by impact, turbo whistle, gear click, backfire, blow-off retained.

Audio:
- Buses (engine, sfx, music, ambience) into compressor, existing reverb per biome retained.
- `event(evt, race)` implements section 2.6 for the player and for events within 120 m (pan by `pose` right vector, gain by distance).
- `horn()` plays `horn.<carId>` (Softeis jingle).
- Ducking: music -6 dB for 0.4 s on explosions and big stingers.
- Volume, mute, focus, suspend, disposal retained (stop all sources, close context).

Acceptance: `scripts/test-aura-drift-audio.mjs` adds a fake AudioContext check that `event` for every catalogue type does not throw, with and without samples.

**T6.4 Music runtime (parallel with T6.3).** Owns `music`.

- Keep `AD.createMusicEngine` live synth as fallback, moved into an inner `createLiveScore`.
- Pack playback: three looped sources started at the same `ctx.currentTime` per biome; gains by intensity (`bed` always, `drive` above 0.3, `peak` above 0.7 or nitro or final lap or last-place chase), smoothed 0.8 s.
- `setBiome` crossfades 1.5 s.
- `event`: `disco_on` plays `stinger.disco` and lowers the `peak` filter for 4 s, `final_lap`, `finish` fanfare by position. Ambience beds from samples, falling back to live beds.
- Disposal stops all sources.

### Batch 7: Tests, docs, landing

**T7.1 Tests.** Owns `ui/desktop_aura_drift_test.go`, `ui/desktop_aura_drift_browser_test.go`, `package.json`.

Apply section 5. Final `"test:aura-drift"` chains:
- physics, tracks, balance, tricks, features, weapons, modes, camera, world, props, banter, assets, audio.

`"build:aura-drift"` becomes `node scripts/build-aura-drift-audio.mjs --check && node scripts/build-aura-drift-vendor.mjs`.

**T7.2 Docs (docs-agent).** Rewrite for the 3D game:
- `documentation/aura-drift.md` (technical guide, verification section).
- `documentation/manuals/de/33-aura-drift.md`, `documentation/manuals/en/33-aura-drift.md` (modes, cars, weapons table, tricks, camera, humour).
- Contract block in `ui/js/desktop/apps/AGENTS.md` (the entry starting at line 3495).
- `ui/AGENTS.md` Aura Drift bullet (module count, 3D, no longer "top-down").
- `assets/aura-drift/README.md`, `assets/aura-drift/AGENTS.md`.
- Root `AGENTS.md` child index row beside AuraGolf (line 190) and its long Aura Drift line (221).
- `internal/desktop/AGENTS.md` line 13 and the catalog description in `internal/desktop/types.go` if they say "top-down" (run impact first; the app name key does not change).
- `documentation/dependencies.md` (runtime and CC0 pack, similar to lines 75 to 85).
- `documentation/manuals/de/04-webui.md`, `documentation/manuals/en/04-webui.md`, and manual READMEs where they describe it.
- Mark `plans/2026-11-04-aura-drift-racing-plan.md` as superseded by this plan in one line.

**T7.3 Final gate (test-agent), one run:**

```
npm run build:aura-drift
npm run test:aura-drift
node scripts/build-ui-bundles.js --check
go test ./ui -run '(TestAuraDrift|TestVirtualDesktopFirstPartyJSFilesStayBelowLineBudget|TestShippedWebAssetsLoadNothingExternal|TestDesktopBuiltinAppNamesExistInAllLanguages|TestDesktopUsedI18nKeysExistInAllLanguages)' -count=1 -v
go test ./internal/desktop -run TestBuiltinAppsCarryStartMenuCategories -count=1
go test ./internal/webassets ./cmd/assetpack -count=1
$env:AURAGO_RUN_BROWSER_SMOKE='1'; $env:AURAGO_BROWSER_MUTE_AUDIO='1'; go test ./ui -run '^TestDesktopAuraDrift' -count=1 -timeout 20m
```

Landing: the controller runs a whole-branch review once, then `detect-changes --scope compare --base-ref main`. Do not push. When the worktree is finally removed, first remove the `node_modules` junction with `cmd /c rmdir`.

---

## 5. Test changes

### 5.1 `ui/desktop_aura_drift_test.go`
- `TestAuraDriftAppRegistration` and `TestAuraDriftShipsNoExternalResources`: new 36-module list; vendor ordering assertion; `fetch(` exception (T0.1).
- Shell markers `missingModules`, `showLoadError`, `createNoopPost` stay.
- CSS markers stay.
- Add check: `ui/3d/aura-drift/v1/manifest.json` and `ui/3d/aura-drift/audio/manifest.json` exist (T7.1).
- `TestAuraDriftLocaleKeysCoverAllLanguages`:
  - Replace the required list with: every `aura_drift.*` key in `en` (at least 330 keys, plus `desktop.app_aura_drift`) is non-empty in all 16 languages, has no U+FFFD, and keeps each `{name}`-style placeholder.
  - For non-English languages, at least 60 percent of `aura_drift.banter.*` values differ from English.
- `TestAuraDriftAvoidsAdBlockerClassPrefix`: add `aura-drift-ui-cards.js` to paths.

### 5.2 Steering regression tests

**Node:** `scripts/test-aura-drift-camera.mjs`. For every `AD.TRACKS` entry plus one daily seed, and each mode in `AD.CAMERA_MODES`:
1. Create a race with no opponents, state running.
2. Throttle 1 for 1 s.
3. Settle the camera with 240 updates at 1/60 s.
4. Take `pose0`, `aim0 = car + heading*30`.
5. Assert `projectToScreen(pose0, aim0).y > 0` or nose-up: `project(car + heading*5).y > project(car).y` for all modes where car depth is positive.
6. Hold `steer:+1` for 60 sim steps; `aim1 = car + heading*30`.
7. Assert `project(pose0, aim1).x > project(pose0, aim0).x + 0.01`.
8. Repeat from a fresh race with `steer:-1` and assert the mirror.

Same procedure with `AD.project2D` for the 2D fallback using the same heading state:
- Nose maps to screen up, and hold-right moves the aim point to larger `sx`.
- Explicit: `screenRotation` for `h=PI/2` (the heading every track starts with) maps world +x (car's left) to smaller `sx`.

Also unit checks: heading follow stays finite and lags during a drift; reduced motion gives zero shake.

**Browser:** `TestDesktopAuraDriftSteeringBrowser` in `ui/desktop_aura_drift_browser_test.go`:
1. `await AuraDriftApp.debug.start({mode:'time_trial',trackId:'alpenpass',laps:1})`, wait for `running` (poll up to 20 s).
2. Key down `ArrowUp`, 1200 ms.
3. `s0 = debug.snapshot()`, key down `ArrowRight`, 500 ms, `s1 = debug.snapshot()`, keys up.
4. Assert `AD.projectToScreen(s0.pose, ...s1.aim, s0.aspect).x > AD.projectToScreen(s0.pose, ...s0.aim, s0.aspect).x`.
5. If `s0.kind==='three'`, also assert `stats().renderer.projection.noseX > stats().renderer.projection.tailX` on the real three camera while still turning, and that `AD.projectToScreen` matches `Vector3.project` within 0.02 NDC. This catches a mirrored axis.
6. Restart, repeat with `ArrowLeft` for the mirror.
7. Sub-test in the stale loader path (below) runs the same flow in 2D using `AD.project2D`.

### 5.3 `scripts/test-aura-drift-balance.mjs`

Uses `AD.createRace({track, mode:'quick', laps:1, opponents:5, weapons:true, seed, showGhost:false})` on all 8 tracks with seeds 90210, 777, 31337. The old documentation records AI laps at 89 to 117 s. Bounds:

| Check | Bound |
| --- | --- |
| All 6 cars finish | 6/6 within `MAX_SECONDS=300` |
| Fastest lap per track | 60 s to 135 s |
| Slowest finisher per track | ≤ 200 s |
| Track variety | `slowest fastest - fastest fastest > 8 s` |
| Wall contacts per race | < 1350 |
| Wall rate | < 9 per car-race second total |
| Catch-up | ≤ 1.06001 (disclosed 6 percent unchanged) |
| Respawns per race | ≤ 12 |
| Vehicle state finite | always |
| Style sum per race | > 0 on every track, < 180000 |
| Weather | deterministic for a seed, grip ≤ 1.0001, and after D1 at least one track has a mid-race intensity above 0.2 |
| Weapons | Over all runs every one of the 18 ids fired at least once; none above 22 percent of fires |
| Reproducible | Same seed twice gives equal fastest, walls, weapon counts, and final hash of `vehicleState` for all cars |

Car parity:
- For each car `c`, 6-car race with slot 0 forced to `c` and all others `vektor`, `weapons:false`, 1 lap, seeds 90210 and 777.
- Ratio of slot 0 finish time to the all-Vektor baseline for that track and seed is in `[0.88, 1.15]` per track, and the 8-track mean is in `[0.93, 1.08]`.
- T1.6 tunes `power`, `dragK` and AI corner limit to satisfy this and records the final numbers in the script header.

Drift reference bot:
- `drift_attack` with AI input forced aggressive (`drift=1`) on 3 tracks, 90 s.
- Score is between 15000 and 30000 (30 to 60 percent of gold). Otherwise adjust the medals table, not the bot, and update section numbers in docs.

### 5.4 `scripts/test-aura-drift-physics.mjs`
Keeps all checks; loads through the sim loader. `raceHash` includes `z`, `vz` through the extended `vehicleState`. Adds tape codec and ghost equality (T1.1).

### 5.5 `scripts/test-aura-drift-tracks.mjs`
Keeps all checks. Module list adds what `buildTrack` needs through the loader. Adds: 18 crates, `straightWindows` non-empty for all tracks and daily seeds.

### 5.6 `ui/desktop_aura_drift_browser_test.go`
- `auraDriftFixtureWithout`: when `script` starts with `/` match the whole path in the line, so `/js/vendor/aura-drift/three-runtime.js` can be removed too. Otherwise behaves as today.
- Add optional `failGLB` parameter via a new helper `auraDriftFixtureFailing(t, path)` that 404s one URL.
- Timing: replace every fixed 3600 ms start wait with a helper `auraDriftWaitState(page, 'running', 20*time.Second)`, because the loading step is now variable.
- `TestDesktopAuraDriftBrowser`:
  - Menu and disclosure checks stay (touch `aria-hidden` stays).
  - World canvas check moves to `.agdrift-gl`, or `.agdrift-world` when `stats.renderer.kind==='canvas2d'`.
  - Render-surface checks: remove `.agdrift-post`; assert active canvas width equals `round(rect.width*min(tierDpr, devicePixelRatio))` and `.agdrift-hud` width equals `round(rect.width*min(2,dpr))`. Viewport is DPR 1, so they match.
- `TestDesktopAuraDriftContentFiltersBrowser`: state check allows `loading` or `countdown`.
- Replace `TestDesktopAuraDriftWetReflectionsBrowser` with `TestDesktopAuraDriftNightHarbourBrowser`:
  - Pick `.agdrift-track-card[data-track="neon-hafen"]`, `select.agdrift-laps` value is `2`.
  - Settings quality `ultra`.
  - In `three` kind: `post.enabled` true, `post.biome==='harbour'`.
  - Screenshot artefact stays.
- `TestDesktopAuraDriftQualityMatrixBrowser`:
  - Remove `cacheCap` and the chunk assertion.
  - `post` expectation: `low` false; `medium`, `high`, `ultra` true (only asserted when `renderer.kind==='three'`).
  - New per-case check in `three` kind: `renderer.calls <= budget*1.5`, `renderer.triangles <= budget*1.5`, using section 3.6 budgets.
  - Touch: `buttons.length===8` and `width >= 56`.
  - Keep keyboard focus and theme coverage.
  - Render-surface check as above.
  - Quality in stats reflects the setting.
- `TestDesktopAuraDriftStaleLoaderBrowser`:
  - `optional_webgl2_module_missing` removes `aura-drift-render3d.js` and asserts `"kind":"canvas2d"`, `"degraded":true`, no error panel, game playable, and the 2D steering assert.
  - Add `runtime_missing` removing `/js/vendor/aura-drift/three-runtime.js`, same asserts minus steering.
  - `required_module_missing` unchanged.
- New `TestDesktopAuraDriftGLBFallbackBrowser`: 404 `/3d/aura-drift/v1/car-vektor.glb`, race runs, `renderer.standIns >= 1`, `fixtureErrors` is empty.
- New `TestDesktopAuraDriftDisposeBrowser` (short): open, start, close, open, start, close; `renderer.geometries` on the second run is at most first + 8; no errors.
- `TestDesktopAuraDriftAdBlockerBrowser`: start wait via helper; otherwise unchanged.
- `TestDesktopAuraDriftSteeringBrowser` as in section 5.2.

`AURAGO_AURA_DRIFT_MATRIX_LIMIT` stays supported. Tests use `-run '^TestDesktopAuraDrift'`.

---

## 6. Risks and mitigations

| Risk | Mitigation |
| --- | --- |
| iGPU performance | Tier budgets (section 3.6) logged by the survey and asserted softly in the matrix. Governor with vsync-aware thresholds. Prop streaming caps instance count. No `Reflector`, no dynamic light count change, pool lights only. Shadows snap and cover only near objects. Low tier has direct render with no post. Terrain static LOD and frustum-culled chunks. Remove `backdrop-filter` from menus. |
| Shader compile hitches | Texture-lookup noise only. Precompile with `renderer.compile(scene, camera)` during the `loading` view. |
| Determinism when adding sim state | Fixed step order (section 2.1). All new state in the sim hash via `vehicleState`. Banter, FX and audio have their own rng. `features.rng` separate from `race.rng`. Ghost equality test. Tapes are v2, old ones dropped. `timeWarp` scales step pacing only. |
| AI consuming `race.rng` in a different order after changes | Only the same-build replay guarantee matters. Bests key is versioned; bump `AD.BEST_KEY` again if sim semantics change after release. |
| File size | 18 MiB models, 15.5 MiB audio, 1.2 MiB runtime, all outside the executable through the web-asset pack (`.glb`, `.ogg` already included). Validators enforce budgets. Scatter props are first to lose bevels. |
| Three.js disposal between tracks and windows | Per-track objects owned by each module's `dispose()`. Shared GLB templates are refcounted by `AD.assets` and disposed only when the last window closes. `TestDesktopAuraDriftDisposeBrowser` checks geometry count. Renderer `forceContextLoss` on close, so contexts do not leak. |
| Context loss | Immediate swap to the 2D renderer, race continues. Restore rebuilds env, composer and track objects. Same code path as the no-WebGL2 fallback, covered by the stale loader test. |
| Line budget | Targets in section 1.4 at or under 900 for everything risky. Any file passing 1000 lines must split before commit. `TestVirtualDesktopFirstPartyJSFilesStayBelowLineBudget` runs in every batch's final gate. |
| Parallel agents editing the same file | Stubs and loader land once (T0.1). Hooks are `typeof`-guarded so no one needs to re-edit `render3d`. Shell owned by exactly one task per batch. Manifest generated by a scan step, never hand-merged. Language files split per task. |
| Scale of translation | 292 keys x 14 languages. Split across four tasks. The coverage test catches missing keys, damaged text and untranslated banter. |
| CC0 sourcing fails or is rejected | Any group can be rendered by own synthesis. Game has live fallbacks for every sound. T6.1 needs user approval before any download. |
| Software GL in CI or headless | `auto` picks 2D. Browser tests branch on `renderer.kind`. Explicit tiers still use 3D. |
| Mirrored axis or steering regression | Pure projection tests for all tracks and all camera modes, plus the real three camera check against pure math, plus the 2D path. |
| Humour or weapons reading as gore or cruelty | Contract: animals and cows only fly, land fine and trot off; no blood, no removal; cow catapult shock hits cars, the cow trots away. |
| Ad blockers | Prefix rule plus word rule (section 0). Existing `TestDesktopAuraDriftAdBlockerBrowser` stays; all GLB file names avoid filter words. |
| Blender export drift | Deterministic rules, a double-run hash comparison, manifest pinning `blender` version; regenerate after any kit edit and commit together. |
| Concurrent sessions | Worktree isolates the branch. Never run bare `git stash`. Commit after every task. |

---

## 7. Definition of done

- Hold right moves right on all tracks in all camera modes, in 3D and 2D, proven by tests.
- 8 cars, 18 weapons, 16 tricks, 7 modes, 8 biomes, 5 animals, 6-car fields, all playable by keyboard and touch.
- Default look is true 3D with Blender assets at golden-hour diorama style, with distinct biomes, working rain and snow, and big effects.
- Sound is sample-based with engine loops per car, layered biome music and comic stingers. Everything has fallbacks.
- All 16 languages complete.
- Section 4 T7.3 gate is green.
- User plays on a real screen next. Performance tuning beyond budgets and listening passes are left to that feedback.
