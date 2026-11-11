# System World Beautification — Phase 6 Implementation Plan (finish)

> **For agentic workers.** Each task goes to one code-writing agent, in the order given under "Order and parallelism". A test-running agent runs the commands listed in each task. Lean workflow: one combined review per batch, Critical and Important findings only, no re-review rounds.
>
> - **Agents do not change git state.** No commits, stashes, resets or branch switches. The coordinator commits each task with the message at the end of it. The last line of every message is `Co-Authored-By: Claude <noreply@anthropic.com>`.
> - **GitNexus is the coordinator's job.** Before handing out a task, the coordinator runs `impact` (upstream) on the functions it edits. Before each commit it runs `detect_changes`.
>   - Expected HIGH or UNKNOWN results: `createCity`, `createAtmosphere`, `createWindows`, `adaptFeet`, `createSociety`, `createExperience`, `createSoundMovers`, `createCityFoley`, `createSurfaces`, `createLobbyRender`, `createWeather`.
>   - `createCityAmbience` is reached through a module namespace from the classic script `sysworld.js`, so UNKNOWN is expected there.
>   - This plan is the mitigation, and its tests cover the callers.
>   - The worktree has no index of its own (backlog 21), so change detection runs against `main`.
> - **The coordinator does all Blender work and all judging of pictures and sound.** Coordinator tasks C1 and C2 say what must change in which `assets/system-world/*.py` function, and what the runtime needs from the result.
>   - A kit rebuild takes about 15 minutes.
>   - It never overlaps with agents running tests, surveys or timing.
> - **Timing runs need a quiet machine.** No other agent may run tests, surveys, Blender or builds during a `--timing` run (Tasks 2, 10 and 17).
> - **Scripts.** Throwaway scripts go into the session scratch directory through the Write tool, never into the repo. Bash heredocs mangle backslashes on this machine. The Write and Edit tools decode backslash-u sequences, so write `\x5c` where Go source needs a literal backslash.
> - **`PAGE_JS` is a template literal.** Code added inside `PAGE_JS` in `scripts/system-world-survey.mjs` must not contain backticks, `${` or backslash escapes. Every code block in this plan that goes there obeys that rule.
> - **Paths.** All paths are relative to the worktree `C:\Users\Andi\Documents\repo\AuraGo\.worktrees\sysworld-beauty`. Never Glob or Grep the main repository root.
> - **Anchors, not line numbers.** Every edit names a function and quotes the anchor text. If the text has drifted, make the same change to the equivalent statement and say so in the hand-back.
> - **Lessons from the phase 4 and 5 execution notes:**
>   - Numbers measured in a run are recorded, never used as test floors.
>   - Instance matrices compare at float32 precision (1e-3).
>   - Fixture positions and camera numbers are checked in a picture before they become fixed.
>   - Both browser runs are needed: `AURAGO_SYSTEM_WORLD_EXPANSION=1` returns early and skips the checks that run only with `AURAGO_SYSTEM_WORLD_MATRIX=1`. Phase 5 ran only the combined run.
> - **No audio files** except in the optional Task 15. Task 14 re-cuts an already approved source only after the coordinator confirms that the source file is available (Request 6).

**Goal**
- Night pictures without white glare: lamps and lit rooms, not blown-out patches.
- Windows that keep their life by day and on the low tier.
- Entering a building looks into the hall, and feet stand on the treads.
- Furniture that costs little far away, under a measured frame-time budget per tier.
- Motion proven in sequences, not assumed from stills.
- A street that sounds like trams and footsteps.
- One acceptance record for the whole beauty pass: before/after pictures, sound survey, budgets and test results.

**Architecture of the changes**

| Concern | Owner | Tasks |
| --- | --- | --- |
| PNG codec, clipped share and blobs, masks, lit share, tone count, contact sheets, before/after pairs, budget init and check | `scripts/system-world-picture-checks.mjs` (new, pure, with a CLI) | 1 |
| Survey instruments: freeze and toggles, building rectangle, masks, view-only triangles, camera-inside check, visitor body follows the camera, feet gaps, light and emissive inventory, checks with `--strict`; sequence mode with contact sheets | `scripts/system-world-survey.mjs`, `.scenes.json` | 3, 10, 11 |
| Lamp cones fade near the camera; aviation lights from roof tops (`coneFade`, `roofTop`, `aviationPoints`) | `sysworld-atmosphere.js` | 5 |
| Wall tones, darker furniture, daylight falling off with depth, flat lit cells on the low tier, night caps by cause | `sysworld-windows.js` | 6 |
| Tread centres (`treadCentre`, `FOOT_HALF`) | `sysworld-lobbies.js` (pure registry) | 7 |
| Foot snapping on flights (`adaptFeet` option `snap`), `soleGaps` | `sysworld-footing.js` | 7 |
| Residents use the snap (presentation only) | `sysworld-society.js` `settle` | 7 |
| Night light levels by cause (environment, moon, pane lighting, lobby fixtures, bloom, point lights) | `sysworld-scene.js`, `sysworld-weather.js`, `sysworld-surfaces.js`, `sysworld-lobby-render.js`, `sysworld-hologram.js` | 8 (conditional) |
| View levelling (`easePitch`), lobby entry, `pitch`/`fov` in `stats()` | `sysworld-scene.js`; `lobby()` accessor in `sysworld-experience.js` | 9 |
| Furniture near and far instance sets | `sysworld-furniture-lod.js` (new), wired in `sysworld-scene.js` `rebuild`/`update` | 10 |
| Patrol `yield` label; visitor seated after rides | `sysworld-society.js` (label only), `sysworld-experience.js` `endRide` | 12 |
| Tram roar bands and flange ticks, wider reach; footfall body resonance | `sysworld-sound-movers.js`, `sysworld-foley.js`, `sysworld-audio.js` (passes `random`) | 13 |
| Darker leaves cut | `assets/system-world/build_audio.py` (`lowpass`, `--only`), `audio-sources.json` | 14 |

**Tech notes**
- three 0.186.1.
  - `InstancedMesh.computeBoundingSphere()` iterates `this.count` (`node_modules/three/src/objects/InstancedMesh.js`).
  - The frustum test computes the sphere lazily only when it is `null`.
- Survey PNGs are canvas captures: 8-bit RGBA, not interlaced, at the tier's pixel ratio (2400×1350 on high).
- The new codec supports colour types 6 and 2 at bit depth 8, which is all the harness writes. `package.json` has no PNG library.
- Angle convention, as in phases 4 and 5: world = (x·cos a + z·sin a, −x·sin a + z·cos a).
- Screen position of the horizon: y_ndc = tan(pitch)/tan(fov/2). "Horizon in the middle third" means |tan(pitch)| ≤ tan(fov/2)/3. At fov 43° that is |pitch| ≤ 7.45°.

---

## What I verified, and what I could not

**Verified by reading** (worktree at `92cfaf1f1`)
- **Design and backlog:** `plans/2026-11-10-system-world-beauty-phase-6-design.md` W1–W8; `plans/2026-11-10-system-world-beauty-phase-6-backlog.md` items 1–33.
- **Survey harness `scripts/system-world-survey.mjs`.**
  - Flags: `--out --scenes --only --tier --width --height --timeout --keep-open --timing --without-furniture`.
  - `EXPOSE` is the string `'globalThis.__sw = { THREE, camera, controls, scene, renderer, composer, experience, traffic, weather, atmosphere, surfaces, windows, sun, bloom, drones, ' + 'model, catalogV3: () => catalog3, lobbies: lobbyRender, ' + 'staticCity, nearHeight: NEAR_HEIGHT, harbour, gulls, particles, ' + 'cancelFlight: …'`, patched in before `  return {\n    districts, canvas, update, focus(id)`.
  - The society patch is `    add,patrol,update,suspend,\n` → `…,actors,`.
  - The `--without-furniture` patch replaces `...furnishing().map(`.
  - The existing `sequence`/`intervalMs` mode sleeps in real time while the RAF loop runs.
  - `eval` runs before the final camera is applied. `stats.triangles` comes from `renderer.info` after `info.reset()`; nothing flushes a pending shadow pass first.
  - The visitor body is moved only inside the lobby scenes' `eval`.
  - The exit code covers page errors only.
- **Scenes file.**
  - `p3-memory-night` (in `reports/system-world-survey/phase-3-first-look/report.json`) used the camera of today's `facade-memory-night`: (−37, 1.7, 17.5) → (−43, 8, −10). The archive front scene therefore exists.
  - The lobby scenes already skip the interior wait on low (`const low=__sw.lobbies.stats().level===null`), so backlog 15 is done.
  - `lobby-agent-step-1..4` relocate `resident-0` onto the agent flight at x 0, heading π.
- **Existing measurements.**
  - `phase-4-final`, high tier: `overview-day` 925,716 triangles / 1,362 calls; `street-agent` 1,943,093 / 1,900; `facade-agent-night` 880,606; `life-park-day` 658,696; `life-street-trees-day` 921,452.
  - `phase-4-timing-check`, high only: `overview-day` 12.6/16.8 ms, `street-agent` 13/15.7 ms (median/p95).
  - There is no phase 5 survey or timing run, and no frame time for medium or low.
  - Baseline folders under `reports/system-world-survey/` include `baseline`, `phase-2*`, `phase-3*`, `phase-4*` and `window-polish*`.
- **`sysworld-windows.js`.**
  - Walls take one of two colours (`step(0.55,winH1)`). Furniture is `winWall*0.3`.
  - `winLightAt` lets daylight fall to a third at the back (`1.0-0.67*z`).
  - Night lamp `mix(0.08,0.7,winDusk)`, ceiling panel up to 1.2, screens and visors 1.15.
  - The low tier (`WIN_LEVEL 0`) draws a flat cell.
  - The glass is `MeshStandardMaterial` with roughness .08 and no envMap, so it reflects `scene.environment`.
  - The windows test pins `stats()` keys and the division tokens allowed in the GLSL.
- **`sysworld-scene.js`.**
  - `scene.environment` is a `RoomEnvironment` PMREM; `environmentIntensity` is .48 and never changes, by day or by night.
  - Bloom: `UnrealBloomPass(…, .28, .68, 1.06)`. Per frame, strength is `.2+.15*(1-day)` and threshold `1.06+.26*smoothstep(day)`.
  - Lights: hemisphere, sun, rim, bounce and `reactorLight` (idle 12). `sysworld-weather.js` sets `sun.intensity=.8+d*2.6` (still .8 at night) and `rim.intensity=.7+…`.
  - Aviation points come from `box.setFromObject(distant.get(id)).max.y` at the placement origin. The placement's angle and any off-centre roof are ignored.
  - `rebuild()` batches instances per template node (`batches.set(n.uuid, …)`) and hides `NEAR` designs above `NEAR_HEIGHT` 40.
  - `focus()` in street mode aims at `d.height*.4`. `move()` never touches the pitch; `look()` clamps it to ±1.35.
  - `stats()` has `position` but no pitch.
- **`sysworld-atmosphere.js`.**
  - Lamp cones: an additive, double-sided `InstancedMesh` of `ConeGeometry(2.7, 6.5)` with strength .3, from the pavement to the luminaire.
  - Pools: radius 3.4 m.
  - Aviation: `Points` with `fog:false`, size 2–14 px.
  - Lamps and pools are hidden on low and by day.
- **`sysworld-surfaces.js`.** `WINDOWS = {city.ivory, city.warm}`. At night `setLighting` multiplies their emission by up to 1.3 on pane faces and by `.55+.45*night` (1.0 at night) on other faces. The kit AGENTS says vertex-coloured v3 ivory/warm parts are fixtures that do not switch per pane.
- **`kit3.py` `PALETTE` emission strengths:** warm 1.3, ivory .8, cyan 1.6, jade 1.3. Colour × strength puts cyan at 1.28 linear, above the night bloom threshold of 1.06.
- **`sysworld-lobby-render.js`.** `FIXTURES` = ivory, warm, cyan, jade inside interiors. `NIGHT.glow` .28.
- **Footing and stairs.**
  - `adaptFeet` takes the deeper of heel and toe penetration and only raises feet.
  - `SOLE` heel −.085 and toe .27 make the sole .355 m long, .41 m at the residents' scale of 1.15.
  - `TREAD` is .32, so a resident's sole is longer than a tread.
  - Flights run towards +z from each podium's `z1` (`lobbyGround`). Only `sysworld-society.js` `settle` calls `adaptFeet`, after traffic has committed, so it is presentation only.
  - The companion walks on one level and has no feet IK. The visitor is a camera with an invisible traffic body.
- **`sysworld-society.js`.**
  - Patrols get `state='yield'` in the tram branch; the hold branch (`yieldTram`) keeps them paused.
  - `if(a.trafficYield){…a.state=a.pause?'patrol':…}` resets the label.
  - `suspend()` clears `trafficYield` but leaves a patrol's state alone.
- **`sysworld-experience.js`.**
  - `endRide()` places the camera for `lift`, `tram`, `drone` and `scope` without `seat()`. Only `hover` seats.
  - The private `lobby` is set in `update()`. The returned object has `isRiding`, `interaction`, `stats().lobby`.
- **Visitor placement in the scene.** `focus` (street), `setMode`, `visit` and `enter` call `settleCamera()`, which calls `traffic.relocate` → `findFree`. `flyTo` uses `findFree`.
- **Go tests.**
  - `verifySystemWorldExpansion` checks `loadedBytes` against 20 MiB without logging it.
  - It takes `world2-lobby-agent.png` after `experience.lobby==='agent'`.
  - In `verifySystemWorldCity` the `AURAGO_SYSTEM_WORLD_EXPANSION` branch returns before the soundscape check (`'Soundscape incomplete: '`) and the low-tier checks (`'Low tier must keep the lobby stand-ins: '`). Those run only with `AURAGO_SYSTEM_WORLD_MATRIX=1` alone.
  - `TestDesktopSysWorldAppMarkers` pins, among others, `"furnishing().map("`, `"InstancedMesh"`, `"setAviation("`, `"export function adaptFeet"`, `"function seat()"`.
- **Sound.**
  - `MOVERS.reach` is tram 90 and hover 30. The tram has one `rollBand`; voice slots are `last` Float64Array(6).
  - `createSoundMovers` is called without `random`.
  - `STEP_VOICES` and the private `PAIRS` drive the footfalls.
  - `build_audio.py` rebuilds every recording; the leaves entry has `highpass` 110 and no `lowpass`.
- **No System World pictures** in `README.md` or `documentation/system-world-2.md`.

**Could not verify** (the executor checks these first)
1. **The causes of each glare patch.** Task 4 finds them. Candidates found by reading:
   - the constant environment reflection;
   - moonlight .8;
   - the pane lighting on ivory/warm;
   - emissive palette entries above threshold;
   - window lamp and panel radiance;
   - fog-free aviation points in front of towers dimmed by fog.
2. Whether the phase 4 triangle figures include a shadow pass. 1.94 M for `street-agent` against the backlog's about 900 k suggests they do. Task 3 adds `viewTriangles`; the W4 target is measured on that figure.
3. **Mask capture.** Whether the 2D render of the mask lands as exact colours: `MeshBasicMaterial`, `toneMapped:false`, a direct `renderer.render` to the canvas, then `toBlob` in the same task. Task 3 checks that pane masks have pixels.
4. **The restore path of `surfaces.setLighting`.** It is called from `sysworld-weather.js` (check the exact arguments with a grep for `surfaces.setLighting(`), and the toggle must restore it the same way.
5. **Sequence camera numbers.** Tram route position for `seq-tram`, the gust timing, the street line for `seq-walk-80` (`streets.zs` in `sysworld-layout.js`), the door sequence speeds. Task 11 adjusts them from pictures.
6. Whether a patrol's `yield` label stays on in any path other than `suspend()` (Task 12 diagnostic).
7. Whether `resident-0`'s `work` pose keeps both feet in stance (lift < 7 cm) in the step scenes. If it does not, the snap does not apply to that foot and the scene needs another pose (Task 7).
8. Run-to-run frame-time variance on this machine. Two baseline runs per tier; the larger median is the base.
9. Whether synthesis reaches classifier z ≥ 2.5 for "a tram passing" and "footsteps". The design accepts recording the outcome.
10. Whether the leaves source file still exists locally. Sources are not kept in the repo.
11. Whether any of the seven uncommitted `assets/system-world/*.py` edits shown by `git status` (`build_robot.py`, `build_textures.py`, `hover_assets.py`, `landmark_assets.py`, `living_assets.py`, `render_preview.py`, `robot_assets.py`) feed the kit v3 build. `AGENTS.md` lists the v3 families as `kit3.py`, `build_world3.py`, street/facade/district/furniture/interior/life; `robot_assets.py` and `hover_assets.py` belong to `build_expansion.py`.
12. Whether a furniture design's LOD 2 keeps `city.leaf` for both trees. Task 10's test reads the shipped GLBs.

## Deviations from the design

1. **No visitor robot body (W3).** The visitor is a first-person camera with an invisible traffic body, and the companion walks on one level. Tread snapping therefore applies to the residents. The visitor's eye already follows the treads through the eye spring.
2. **Archive front scene (W1).** It exists as `facade-memory-night`, the same camera as phase 3's `p3-memory-night`, so no new scene is added. The archive light bars are measured in `lobby-memory-night`.
3. **"window-polish scenes" (W2)** are read as `facade-agent-day`, `facade-memory-day`, `facade-missions-day` and `proof-window-day`, the scenes of that folder.
4. **Two more cause classes for W1**, found by reading:
   - E: the environment reflection is .48 by day and by night;
   - F: pane lighting in `sysworld-surfaces.js` scales ivory/warm emission at night.

   Moonlight (M), lobby fixtures (L) and bloom (G, last resort) are also listed. Every class has a conditional fix step.
5. **Triangle target (W4).** Measured as `viewTriangles` of `street-agent` on high (no shadow pass in that frame). The budget base is phase 5 code measured in Task 2: no phase 5 timing exists, so a reference figure cannot come from history.
6. **Furniture sets (W4).**
   - The shadow map is not refreshed for a re-sort, because pieces keep their place and footprint.
   - `banner-mast` stays in the static batch, since its cloth is a separate part instance.
   - The low tier keeps the single batch, since it is LOD 2 already.
   - Set visibility is left to the existing near gate; an empty set has `count` 0.
7. **Sequence mode (W5)** is a new `stepDt`/`sequenceFrames` mode with fixed simulated steps and a held animation loop. The old real-time `sequence`/`intervalMs` stays for the resident-feet scenes. Contact sheets and before/after pairs are composed by a Node PNG codec, so the coordinator can read them as images.
8. **Patrol label (W6):** diagnostic first. The label rule is "a patrol shows `yield` only while it is held for a tram". If the diagnostic shows all label time was genuine yielding, item 26 closes with that note and item 14 stays in the backlog.
9. **Tram flange ticks (W7)** need a deterministic random, so `createSoundMovers` gets the injected `random`.
10. **Leaves (W7).** `build_audio.py` gets `--only <ids>`, so re-cutting one loop does not re-encode the five others; encoder output can differ between ffmpeg versions.
11. **README/manual pictures (W8).** None show the System World, so nothing is refreshed and the record says so.

## Requests to the coordinator

1. **Commit Task 1, then run Task 2** before any app module changes, on a quiet machine.
2. **After Task 4,** choose the fix classes from its table:
   - classes B and E-w → Task 6 steps 6–7;
   - E, M, F, L, C, G → Task 8 steps;
   - A on an exterior part → C1.
3. **Blender (C1, C2).**
   - Share one kit rebuild if both are needed.
   - Decide first what to do with the seven uncommitted asset `.py` edits (Could-not-verify 11).
   - No agent runs tests or surveys during the rebuild.
4. **Visual and audio judgement.** You judge:
   - the night pictures (Tasks 5, 6, 8);
   - the day facades (Task 6);
   - the lobby screenshot (Task 9);
   - the walk sheet for popping (Tasks 10, 11);
   - every sequence sheet (Task 11);
   - the earshot results (Tasks 13, 14).

   Tuning ranges are listed with each constant.
5. **Living test JSON proof** (Tasks 7 and 12): compare the last stdout line with Task 2's `living.json`.
6. **Leaves source (Task 14).** Confirm that `0904.ogg` (sha256 `b223d4f7…`) is available locally, or that re-fetching this already approved file from its recorded URL is acceptable. If neither holds, ask the user.
7. **Optional recordings (Task 15).** Ask the user explicitly, with the concrete files (page, author, licence, size). Without a yes, Task 15 is dropped.
8. **Baseline folder for the before/after sheet (Task 17).** Choose it, for example `reports/system-world-survey/baseline` (2026-11-09) if that is the right date.
9. **Do not merge, push or deploy.** Task 17 ends with the rebase and a merge note.

---

## File map

| File | Status | Tasks |
| --- | --- | --- |
| `scripts/system-world-picture-checks.mjs` | new | 1 |
| `scripts/test-system-world-picture-checks.mjs` | new | 1 |
| `scripts/system-world-budget.json` | new (generated by Task 2) | 2 |
| `scripts/system-world-survey.mjs` | edit | 3, 10, 11 |
| `scripts/system-world-survey.scenes.json` | edit | 3, 11 |
| `ui/js/desktop/apps/sysworld-atmosphere.js` | edit | 5 |
| `scripts/test-system-world-lamps.mjs` | edit | 5 |
| `scripts/test-system-world-aviation.mjs` | new | 5 |
| `ui/js/desktop/apps/sysworld-windows.js`, `scripts/test-system-world-windows.mjs` | edit | 6 |
| `ui/js/desktop/apps/sysworld-footing.js`, `sysworld-lobbies.js`, `sysworld-society.js` (`settle` only) | edit | 7 |
| `scripts/test-system-world-footing.mjs`, `scripts/test-system-world-lobbies.mjs` | edit | 7 |
| `sysworld-scene.js`, `sysworld-weather.js`, `sysworld-surfaces.js`, `sysworld-lobby-render.js`, `sysworld-hologram.js` and their tests | edit (conditional) | 8 |
| `ui/js/desktop/apps/sysworld-scene.js` | edit | 5, 8, 9, 10 |
| `ui/js/desktop/apps/sysworld-experience.js` | edit | 9, 12 |
| `scripts/test-system-world-view.mjs` | new | 9 |
| `ui/desktop_sysworld_expansion_browser_test.go` | edit | 9 |
| `ui/js/desktop/apps/sysworld-furniture-lod.js`, `scripts/test-system-world-furniture-lod.mjs` | new | 10 |
| `ui/js/desktop/apps/sysworld-society.js` (patrol label) | edit | 12 |
| `scripts/test-system-world-patrol-label.mjs`, `scripts/test-system-world-visitor-placement.mjs` | new | 12 |
| `sysworld-sound-movers.js`, `sysworld-foley.js`, `sysworld-audio.js` (one argument) and the movers/foley tests | edit | 13 |
| `assets/system-world/build_audio.py`, `audio-sources.json`, `ui/3d/system-world/audio/{leaves.ogg,manifest.json}` | edit | 14 (coordinator rebuilds) |
| `assets/system-world/kit3.py` or a family module; `interior_assets3.py`; `ui/3d/system-world/v3/*` | coordinator | C1, C2 |
| `ui/desktop_sysworld_test.go`, `ui/desktop_sysworld_city_browser_test.go`, docs, `ui/js/vendor/system-world/*` | edit, regenerate | 16 |

## Invariants every task must keep

- **Survey patch anchors.** `sysworld-scene.js` keeps `  return {\n    districts, canvas, update, focus(id)` and `...furnishing().map(`. `sysworld-society.js` keeps `    add,patrol,update,suspend,\n`.
- **Go markers.** Every string in `TestDesktopSysWorldAppMarkers` stays.
- **Resident simulation untouched.**
  - No task changes what residents decide: paths, slots, traffic proposals, timings.
  - `sysworld-life.js`, `sysworld-navigation.js` and `sysworld-traffic.js` are not edited.
  - `sysworld-society.js` changes only in `settle` (Task 7) and in the patrol label (Task 12).
- **Limits never lowered.** Value pins (constants) may change with a reason given in the hand-back. Limits (budgets, maxima, minima) never loosen.
- **No per-frame allocation** in `coneFade` and the cone fade loop, `furnitureSets.sync`, `easePitch`/`levelView`, and the new `adaptFeet` path beyond what the existing function already allocates.
- **Bundle staleness.** `node scripts/build-system-world.js --check` is red from Task 5 until Task 16.
- **Material ownership** (phase 4): `city.window` belongs to `sysworld-windows.js`, `city.pane` to `sysworld-lobby-render.js`.
- **Nothing external:** no fetch, no URLs, no new downloads (Task 15 excepted).

## Order and parallelism

| Step | Tasks | Notes |
| --- | --- | --- |
| A | 1 | Pure script and test |
| B | 2 | After 1 is committed. Runs only, on a quiet machine. Writes `scripts/system-world-budget.json` |
| C | 3 | Survey instruments; the validation run gives the "before" metrics (app code is still phase 5) |
| D | 4 | Find step: runs only. Decides Tasks 6 (steps 6–7), 8 and C1 |
| E | 5 ∥ 6 ∥ 7, then C1 alone | Disjoint files: 5 atmosphere + scene aviation lines; 6 windows; 7 footing, lobbies, society `settle`. C1 only if needed, never during runs |
| F | 8 | Conditional; after 5 (scene), 6, C1 |
| G | 9 | After 8 or 5 (scene), experience |
| H | 10 | After 9 (scene) and 3 (survey) |
| I | 11 | After 10 (survey) |
| J | 12 ∥ 13 ∥ 14, plus C2 alone | 12 after 7 and 9 (society, experience). 13 and 14 touch only sound files and may start earlier if the coordinator wants. C2 never overlaps runs |
| K | 15 | Optional, only with the user's yes |
| L | 16 → 17 | Markers, docs, bundle, regression → acceptance record |

---

## Task 1 — Picture checks module (PNG codec, measurements, sheets, budget)

**Files:** new `scripts/system-world-picture-checks.mjs`, new `scripts/test-system-world-picture-checks.mjs`.

- [ ] **Step 1. Write the test first** (complete file `scripts/test-system-world-picture-checks.mjs`):

```js
// System World picture checks: PNG round trip, all five decode filters and colour type 2, clipped share and blob,
// masks, percentiles, lit share, tone count, downscale and compose, budget init and check. Synthetic images only.
import assert from 'node:assert/strict';
import zlib from 'node:zlib';
import {decodePNG,encodePNG,clipped,pixelRect,maskOf,maskLumas,percentile,litShare,toneCount,downscale,compose,budgetInit,budgetCheck,luma} from './system-world-picture-checks.mjs';

const image=(w,h,fill=[0,0,0,255])=>{const data=new Uint8Array(w*h*4);for(let i=0;i<w*h;i++)data.set(fill,i*4);return {width:w,height:h,data};};
const paint=(img,x0,y0,x1,y1,c)=>{for(let y=y0;y<y1;y++)for(let x=x0;x<x1;x++)img.data.set(c,(y*img.width+x)*4);};

// 1. Round trip through the encoder.
{ const img=image(7,5,[10,20,30,255]);paint(img,2,1,5,4,[250,251,252,255]);
  const back=decodePNG(encodePNG(img));assert.equal(back.width,7);assert.equal(back.height,5);assert.deepEqual([...back.data],[...img.data]); }
// 2. Every filter type (one per row) and colour type 2 (RGB), hand-built.
{ const w=4,h=5,rgb=[];for(let y=0;y<h;y++)for(let x=0;x<w;x++)rgb.push((x*37+y*11)&255,(x*5+y*90)&255,(x*71+y*3)&255);
  const stride=w*3,raw=Buffer.alloc((stride+1)*h);
  for(let y=0;y<h;y++){const f=y;raw[y*(stride+1)]=f;for(let i=0;i<stride;i++){
    const cur=rgb[y*stride+i],a=i>=3?rgb[y*stride+i-3]:0,b=y?rgb[(y-1)*stride+i]:0,c=i>=3&&y?rgb[(y-1)*stride+i-3]:0;
    let pred=0;if(f===1)pred=a;else if(f===2)pred=b;else if(f===3)pred=(a+b)>>1;else if(f===4){const p=a+b-c,pa=Math.abs(p-a),pb=Math.abs(p-b),pc=Math.abs(p-c);pred=pa<=pb&&pa<=pc?a:pb<=pc?b:c;}
    raw[y*(stride+1)+1+i]=(cur-pred)&255;}}
  const chunk=(kind,body)=>{const head=Buffer.alloc(8);head.writeUInt32BE(body.length,0);head.write(kind,4,'latin1');return Buffer.concat([head,body,Buffer.alloc(4)]);};
  const ihdr=Buffer.alloc(13);ihdr.writeUInt32BE(w,0);ihdr.writeUInt32BE(h,4);ihdr[8]=8;ihdr[9]=2;
  const png=Buffer.concat([Buffer.from([137,80,78,71,13,10,26,10]),chunk('IHDR',ihdr),chunk('IDAT',zlib.deflateSync(raw)),chunk('IEND',Buffer.alloc(0))]);
  const img=decodePNG(png);for(let i=0;i<w*h;i++)assert.deepEqual([...img.data.subarray(i*4,i*4+4)],[rgb[i*3],rgb[i*3+1],rgb[i*3+2],255],'pixel '+i); }
// 3. Clipped share and the largest blob inside a rectangle given in fractions of the frame.
{ const img=image(100,50);paint(img,10,10,14,14,[255,255,255,255]);paint(img,60,20,61,22,[255,255,255,255]);paint(img,70,30,80,31,[255,255,200,255]);
  const all=clipped(img);assert.equal(all.pixels,18);assert.equal(all.largest,16);assert.ok(Math.abs(all.share-18/5000)<1e-12);
  const left=clipped(img,[0,0,.5,1]);assert.equal(left.pixels,16);assert.equal(left.area,2500);
  assert.deepEqual(pixelRect(img,[.1,.2,.5,1.4]),[10,10,50,50]);assert.equal(clipped(img,[.5,.5,.5,.9]).share,0); }
// 4. Masks, lumas, percentiles, lit share.
{ const mask=image(10,10);paint(mask,0,0,5,10,[255,0,0,255]);paint(mask,5,0,10,10,[0,250,10,255]);
  const red=maskOf(mask,[255,0,0]),green=maskOf(mask,[0,255,0]);assert.equal(red.reduce((a,b)=>a+b,0),50);assert.equal(green.reduce((a,b)=>a+b,0),50);
  const img=image(10,10,[20,20,20,255]);paint(img,0,0,2,10,[200,200,200,255]);
  assert.ok(Math.abs(litShare(img,red)-.4)<1e-12,'20 of 50 pane pixels lit');
  const l=maskLumas(img,red);assert.equal(l.length,50);assert.ok(Math.abs(percentile(l,.99)-luma(200,200,200))<1e-9);assert.ok(Math.abs(percentile(l,.5)-20)<1e-9);
  assert.ok(Number.isNaN(percentile([],.5))); }
// 5. Tone count: three colour fields inside the mask are three tones, a grey field one, an empty mask no blocks.
{ const w=48,h=24,img=image(w,h),mask=new Uint8Array(w*h).fill(1);
  paint(img,0,0,16,24,[200,190,170,255]);paint(img,16,0,32,24,[70,110,140,255]);paint(img,32,0,48,24,[150,80,60,255]);
  assert.equal(toneCount(img,mask).tones,3);assert.equal(toneCount(image(w,h,[120,120,120,255]),mask).tones,1);
  assert.equal(toneCount(img,new Uint8Array(w*h)).blocks,0); }
// 6. Downscale (box filter) and compose (tiles on a dark ground with a gap).
{ const img=image(4,2);paint(img,0,0,2,2,[100,0,0,255]);paint(img,2,0,4,2,[0,200,0,255]);
  const half=downscale(img,2);assert.equal(half.width,2);assert.equal(half.height,1);assert.deepEqual([...half.data],[100,0,0,255,0,200,0,255]);
  const sheet=compose([half,half,half],2,1);assert.equal(sheet.width,7);assert.equal(sheet.height,5);
  assert.deepEqual([...sheet.data.subarray((1*sheet.width+1)*4,(1*sheet.width+1)*4+4)],[100,0,0,255],'first tile at the gap offset'); }
// 7. Budget: the base is the larger median of two runs; base +10 % passes, more fails; the view-triangle limit is strict.
{ const run=(tier,ms,tri)=>({info:{tier},shots:[{scene:'street-agent',frameMs:{median:ms,p95:ms*1.2},stats:{viewTriangles:tri,triangles:tri*2}}]});
  const budget=budgetInit([run('high',10,700000),run('high',11,700000),run('low',5,300000)]);
  assert.equal(budget.frameMs['street-agent'].high,11);assert.equal(budget.frameMs['street-agent'].low,5);assert.equal(budget.margin,.1);
  assert.deepEqual(budgetCheck(budget,[run('high',12.1,640000)]).failures,[]);
  assert.equal(budgetCheck(budget,[run('high',12.2,640000)]).failures.length,1);
  assert.equal(budgetCheck(budget,[run('high',11,650000)]).failures.length,1,'650 000 is not below the limit');
  assert.equal(budgetCheck(budget,[run('low',5,999999)]).failures.length,0,'the triangle limit applies to its own scene and tier'); }
console.log('System World picture checks: PNG codec (5 filters, RGB and RGBA), clipped share and blobs, masks, lit share, tones, sheets and budget passed.');
```

- [ ] **Step 2.** Run `node scripts/test-system-world-picture-checks.mjs` and confirm it fails because the module is missing.
- [ ] **Step 3. Write the module** (complete file `scripts/system-world-picture-checks.mjs`):

```js
#!/usr/bin/env node
// System World picture checks (beauty pass, phase 6). A minimal PNG codec (8-bit RGB or RGBA, not interlaced: what the
// survey's canvas captures are) and pure measurements on survey pictures: the clipped share and the largest clipped blob
// inside a rectangle, mask decoding, luma percentiles, the lit share and a coarse wall-tone count inside a mask; contact
// sheets and before/after pairs; the frame-time and triangle budget. Node only, no dependencies.
//   node scripts/system-world-picture-checks.mjs sheet --dir <dir> --name <sequence> [--cols 4] [--scale 2]
//   node scripts/system-world-picture-checks.mjs compare --before <dir> --after <dir> --out <dir> [--only a,b] [--scale 2]
//   node scripts/system-world-picture-checks.mjs lit-compare --a <dir> --b <dir> --scene <name> [--tol 0.10]
//   node scripts/system-world-picture-checks.mjs budget-init --reports <dir,dir> --out <file> [--margin 0.10] [--triangles 650000]
//   node scripts/system-world-picture-checks.mjs budget-check --budget <file> --reports <dir,dir>
import zlib from 'node:zlib';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const SIGNATURE = Buffer.from([137, 80, 78, 71, 13, 10, 26, 10]);
const CRC = new Uint32Array(256).map((_, n) => { let c = n; for (let k = 0; k < 8; k++) c = c & 1 ? 0xedb88320 ^ (c >>> 1) : c >>> 1; return c >>> 0; });
function crc32(buf) { let c = 0xffffffff; for (const b of buf) c = CRC[(c ^ b) & 255] ^ (c >>> 8); return (c ^ 0xffffffff) >>> 0; }

export function decodePNG(buf) {
  if (!Buffer.isBuffer(buf)) buf = Buffer.from(buf);
  if (!buf.subarray(0, 8).equals(SIGNATURE)) throw Error('not a PNG');
  let off = 8, width = 0, height = 0, depth = 0, type = 0, interlace = 0; const idat = [];
  while (off + 8 <= buf.length) {
    const len = buf.readUInt32BE(off), kind = buf.toString('latin1', off + 4, off + 8), data = buf.subarray(off + 8, off + 8 + len);
    if (kind === 'IHDR') { width = data.readUInt32BE(0); height = data.readUInt32BE(4); depth = data[8]; type = data[9]; interlace = data[12]; }
    else if (kind === 'IDAT') idat.push(data);
    else if (kind === 'IEND') break;
    off += 12 + len;
  }
  if (depth !== 8 || (type !== 6 && type !== 2) || interlace) throw Error(`unsupported PNG (depth ${depth}, colour type ${type}, interlace ${interlace})`);
  const channels = type === 6 ? 4 : 3, stride = width * channels, raw = zlib.inflateSync(Buffer.concat(idat));
  const data = new Uint8Array(width * height * 4); let prev = new Uint8Array(stride), line = new Uint8Array(stride);
  for (let y = 0; y < height; y++) {
    const filter = raw[y * (stride + 1)], src = raw.subarray(y * (stride + 1) + 1, (y + 1) * (stride + 1));
    for (let i = 0; i < stride; i++) {
      const a = i >= channels ? line[i - channels] : 0, b = prev[i], c = i >= channels ? prev[i - channels] : 0;
      let v = src[i];
      if (filter === 1) v += a; else if (filter === 2) v += b; else if (filter === 3) v += (a + b) >> 1;
      else if (filter === 4) { const p = a + b - c, pa = Math.abs(p - a), pb = Math.abs(p - b), pc = Math.abs(p - c); v += pa <= pb && pa <= pc ? a : pb <= pc ? b : c; }
      line[i] = v & 255;
    }
    for (let x = 0; x < width; x++) { const o = (y * width + x) * 4, s = x * channels; data[o] = line[s]; data[o + 1] = line[s + 1]; data[o + 2] = line[s + 2]; data[o + 3] = channels === 4 ? line[s + 3] : 255; }
    [prev, line] = [line, prev];
  }
  return { width, height, data };
}

export function encodePNG({ width, height, data }) {
  const stride = width * 4, raw = Buffer.alloc((stride + 1) * height);
  for (let y = 0; y < height; y++) Buffer.from(data.buffer, data.byteOffset + y * stride, stride).copy(raw, y * (stride + 1) + 1);
  const chunk = (kind, body) => { const head = Buffer.alloc(8); head.writeUInt32BE(body.length, 0); head.write(kind, 4, 'latin1');
    const crc = Buffer.alloc(4); crc.writeUInt32BE(crc32(Buffer.concat([head.subarray(4), body])), 0); return Buffer.concat([head, body, crc]); };
  const ihdr = Buffer.alloc(13); ihdr.writeUInt32BE(width, 0); ihdr.writeUInt32BE(height, 4); ihdr[8] = 8; ihdr[9] = 6;
  return Buffer.concat([SIGNATURE, chunk('IHDR', ihdr), chunk('IDAT', zlib.deflateSync(raw, { level: 6 })), chunk('IEND', Buffer.alloc(0))]);
}

export const luma = (r, g, b) => .2126 * r + .7152 * g + .0722 * b;
// [x0, y0, x1, y1] in fractions of the frame -> pixel bounds clamped to the image.
export function pixelRect(img, rect = [0, 0, 1, 1]) {
  const c = (v, n) => Math.max(0, Math.min(n, Math.round(v * n)));
  return [c(rect[0], img.width), c(rect[1], img.height), c(rect[2], img.width), c(rect[3], img.height)];
}
// Pixels at or above `threshold` in all three channels: their share of the rectangle and the largest 4-connected blob.
export function clipped(img, rect, threshold = 250) {
  const [x0, y0, x1, y1] = pixelRect(img, rect), w = x1 - x0, h = y1 - y0;
  if (w <= 0 || h <= 0) return { share: 0, pixels: 0, largest: 0, area: 0 };
  const hit = new Uint8Array(w * h); let pixels = 0;
  for (let y = 0; y < h; y++) for (let x = 0; x < w; x++) {
    const o = ((y0 + y) * img.width + x0 + x) * 4;
    if (img.data[o] >= threshold && img.data[o + 1] >= threshold && img.data[o + 2] >= threshold) { hit[y * w + x] = 1; pixels++; }
  }
  let largest = 0; const stack = [];
  for (let i = 0; i < hit.length; i++) {
    if (hit[i] !== 1) continue;
    let size = 0; hit[i] = 2; stack.push(i);
    while (stack.length) {
      const j = stack.pop(), x = j % w, y = (j - x) / w; size++;
      if (x > 0 && hit[j - 1] === 1) { hit[j - 1] = 2; stack.push(j - 1); }
      if (x < w - 1 && hit[j + 1] === 1) { hit[j + 1] = 2; stack.push(j + 1); }
      if (y > 0 && hit[j - w] === 1) { hit[j - w] = 2; stack.push(j - w); }
      if (y < h - 1 && hit[j + w] === 1) { hit[j + w] = 2; stack.push(j + w); }
    }
    if (size > largest) largest = size;
  }
  return { share: pixels / (w * h), pixels, largest, area: w * h };
}
export function maskOf(img, colour, tolerance = 60) {
  const out = new Uint8Array(img.width * img.height);
  for (let i = 0; i < out.length; i++) { const o = i * 4;
    out[i] = Math.abs(img.data[o] - colour[0]) <= tolerance && Math.abs(img.data[o + 1] - colour[1]) <= tolerance && Math.abs(img.data[o + 2] - colour[2]) <= tolerance ? 1 : 0; }
  return out;
}
export function maskLumas(img, mask) { const out = []; for (let i = 0; i < mask.length; i++) if (mask[i]) { const o = i * 4; out.push(luma(img.data[o], img.data[o + 1], img.data[o + 2])); } return out; }
export function percentile(values, p) { if (!values.length) return NaN; const s = Float64Array.from(values).sort(); return s[Math.min(s.length - 1, Math.max(0, Math.ceil(p * s.length) - 1))]; }
export function litShare(img, mask, lumaMin = 64) {
  let n = 0, lit = 0;
  for (let i = 0; i < mask.length; i++) if (mask[i]) { n++; const o = i * 4; if (luma(img.data[o], img.data[o + 1], img.data[o + 2]) >= lumaMin) lit++; }
  return n ? lit / n : NaN;
}
// Coarse wall tones: the mean colour of every block x block cell fully inside the mask, binned by luma (3 bins) and hue
// (12 bins, or grey below `greyChroma`); a tone is a bin that holds at least `minShare` of the blocks.
export function toneCount(img, mask, { block = 8, minShare = .03, greyChroma = 10 } = {}) {
  const bins = new Map(); let blocks = 0;
  for (let by = 0; by + block <= img.height; by += block) for (let bx = 0; bx + block <= img.width; bx += block) {
    let r = 0, g = 0, b = 0, full = true;
    for (let y = by; y < by + block && full; y++) for (let x = bx; x < bx + block; x++) {
      const i = y * img.width + x; if (!mask[i]) { full = false; break; }
      const o = i * 4; r += img.data[o]; g += img.data[o + 1]; b += img.data[o + 2];
    }
    if (!full) continue;
    const n = block * block; r /= n; g /= n; b /= n; blocks++;
    const l = luma(r, g, b), band = l < 40 ? 0 : l < 110 ? 1 : 2, chroma = Math.max(r, g, b) - Math.min(r, g, b);
    const hue = chroma < greyChroma ? 'grey' : Math.floor(((Math.atan2(Math.sqrt(3) * (g - b), 2 * r - g - b) / (2 * Math.PI) + 1) % 1) * 12);
    const key = band + ':' + hue; bins.set(key, (bins.get(key) || 0) + 1);
  }
  return { tones: [...bins.values()].filter(c => blocks && c / blocks >= minShare).length, blocks };
}

export function downscale(img, factor) {
  if (!(factor > 1)) return img;
  if (!Number.isInteger(factor)) throw Error('downscale needs an integer factor');
  const w = Math.floor(img.width / factor), h = Math.floor(img.height / factor), data = new Uint8Array(w * h * 4), n = factor * factor;
  for (let y = 0; y < h; y++) for (let x = 0; x < w; x++) for (let c = 0; c < 4; c++) {
    let s = 0; for (let dy = 0; dy < factor; dy++) for (let dx = 0; dx < factor; dx++) s += img.data[((y * factor + dy) * img.width + x * factor + dx) * 4 + c];
    data[(y * w + x) * 4 + c] = Math.round(s / n);
  }
  return { width: w, height: h, data };
}
export function compose(images, columns, gap = 8) {
  const w = Math.max(...images.map(i => i.width)), h = Math.max(...images.map(i => i.height)), rows = Math.ceil(images.length / columns);
  const width = columns * w + (columns + 1) * gap, height = rows * h + (rows + 1) * gap, data = new Uint8Array(width * height * 4);
  for (let i = 0; i < data.length; i += 4) { data[i] = 16; data[i + 1] = 16; data[i + 2] = 16; data[i + 3] = 255; }
  images.forEach((img, n) => {
    const ox = gap + (n % columns) * (w + gap), oy = gap + Math.floor(n / columns) * (h + gap);
    for (let y = 0; y < img.height; y++) data.set(img.data.subarray(y * img.width * 4, (y + 1) * img.width * 4), ((oy + y) * width + ox) * 4);
  });
  return { width, height, data };
}
const readPNG = file => decodePNG(fs.readFileSync(file));
// Contact sheet of a sequence: <name>-00.png, <name>-01.png ... -> <name>-sheet.png in the same folder.
export function writeSheet(dir, name, { columns = 4, scale = 2 } = {}) {
  const frames = fs.readdirSync(dir).filter(f => f.startsWith(name + '-') && /^[0-9]{2}\.png$/.test(f.slice(name.length + 1))).sort();
  if (!frames.length) return null;
  const file = path.join(dir, name + '-sheet.png');
  fs.writeFileSync(file, encodePNG(compose(frames.map(f => downscale(readPNG(path.join(dir, f)), scale)), Math.min(columns, frames.length))));
  return file;
}

// Budget file: {reference, margin, triangles: {scene, tier, max}, frameMs: {<scene>: {<tier>: base median}}}.
export function budgetInit(reports, { margin = .1, triangles = 650000 } = {}) {
  const frameMs = {};
  for (const r of reports) for (const s of r.shots || []) if (s.frameMs) {
    const t = r.info.tier; frameMs[s.scene] ??= {}; frameMs[s.scene][t] = Math.max(frameMs[s.scene][t] || 0, s.frameMs.median);
  }
  return { reference: reports[0]?.info?.renderer || null, margin, triangles: { scene: 'street-agent', tier: 'high', max: triangles }, frameMs };
}
export function budgetCheck(budget, reports) {
  const rows = [], failures = [];
  for (const r of reports) for (const s of r.shots || []) {
    const t = r.info.tier, base = budget.frameMs?.[s.scene]?.[t];
    if (s.frameMs && base) {
      const limit = +(base * (1 + budget.margin)).toFixed(2), ok = s.frameMs.median <= limit;
      rows.push({ scene: s.scene, tier: t, median: s.frameMs.median, base, limit, ok });
      if (!ok) failures.push(`${s.scene}@${t}: median ${s.frameMs.median} ms > ${limit} ms`);
    }
    const tri = budget.triangles;
    if (tri && s.scene === tri.scene && t === tri.tier && s.stats) {
      const value = s.stats.viewTriangles ?? s.stats.triangles, ok = value < tri.max;
      rows.push({ scene: s.scene, tier: t, triangles: value, limit: tri.max, ok });
      if (!ok) failures.push(`${s.scene}@${t}: ${value} triangles >= ${tri.max}`);
    }
  }
  return { rows, failures };
}

const readReport = dir => JSON.parse(fs.readFileSync(path.join(dir, 'report.json'), 'utf8'));
function parse(argv) {
  const o = {};
  for (let i = 0; i < argv.length; i++) {
    if (!argv[i].startsWith('--')) throw Error('bad argument ' + argv[i]);
    const key = argv[i].slice(2), next = argv[i + 1];
    o[key] = next === undefined || next.startsWith('--') ? true : (i++, next);
  }
  return o;
}
function main([command, ...rest]) {
  const o = parse(rest), list = v => String(v).split(',').map(s => s.trim()).filter(Boolean);
  if (command === 'sheet') { const file = writeSheet(path.resolve(o.dir), o.name, { columns: Number(o.cols || 4), scale: Number(o.scale || 2) }); console.log(file ? 'sheet ' + file : 'no frames for ' + o.name); return 0; }
  if (command === 'compare') {
    const before = path.resolve(o.before), after = path.resolve(o.after), out = path.resolve(o.out), scale = Number(o.scale || 2), only = o.only ? list(o.only) : null;
    fs.mkdirSync(out, { recursive: true });
    const shots = dir => new Map((fs.existsSync(path.join(dir, 'report.json')) ? readReport(dir).shots || [] : []).map(s => [s.name, s]));
    const sb = shots(before), sa = shots(after), f = v => v === undefined || v === null ? '' : v;
    const rows = ['| Picture | Triangles before | after | Calls before | after | Glare before | after |', '| --- | ---: | ---: | ---: | ---: | ---: | ---: |'];
    const names = fs.readdirSync(after).filter(n => n.endsWith('.png') && !/-mask\.png$|-sheet\.png$/.test(n) && fs.existsSync(path.join(before, n)))
      .map(n => n.slice(0, -4)).filter(n => !only || only.includes(n)).sort();
    for (const name of names) {
      fs.writeFileSync(path.join(out, name + '.png'), encodePNG(compose([downscale(readPNG(path.join(before, name + '.png')), scale), downscale(readPNG(path.join(after, name + '.png')), scale)], 2)));
      const b = sb.get(name), a = sa.get(name);
      rows.push(`| ${name} | ${f(b?.stats?.viewTriangles ?? b?.stats?.triangles)} | ${f(a?.stats?.viewTriangles ?? a?.stats?.triangles)} | ${f(b?.stats?.calls)} | ${f(a?.stats?.calls)} | ${f(b?.metrics?.glare?.share)} | ${f(a?.metrics?.glare?.share)} |`);
    }
    fs.writeFileSync(path.join(out, 'compare.md'), '# Before (left) and after (right)\n\n' + rows.join('\n') + '\n');
    console.log(`compared ${names.length} pictures into ${out}`); return 0;
  }
  if (command === 'lit-compare') {
    const pick = dir => (readReport(path.resolve(dir)).shots || []).find(s => s.scene === o.scene && s.metrics?.panes)?.metrics.panes.litShare;
    const a = pick(o.a), b = pick(o.b), tol = Number(o.tol || .1);
    if (!(Number.isFinite(a) && Number.isFinite(b))) { console.error('lit share missing: ' + JSON.stringify({ a, b })); return 1; }
    console.log(`lit share ${o.scene}: ${a} vs ${b} (difference ${Math.abs(a - b).toFixed(3)}, tolerance ${tol})`); return Math.abs(a - b) <= tol ? 0 : 1;
  }
  if (command === 'budget-init') {
    const budget = budgetInit(list(o.reports).map(d => readReport(path.resolve(d))), { margin: Number(o.margin || .1), triangles: Number(o.triangles || 650000) });
    fs.writeFileSync(path.resolve(o.out), JSON.stringify(budget, null, 2) + '\n'); console.log('budget written: ' + o.out); return 0;
  }
  if (command === 'budget-check') {
    const budget = JSON.parse(fs.readFileSync(path.resolve(o.budget), 'utf8')), result = budgetCheck(budget, list(o.reports).map(d => readReport(path.resolve(d))));
    console.table(result.rows); for (const f of result.failures) console.log('FAIL ' + f); return result.failures.length ? 1 : 0;
  }
  console.error('commands: sheet | compare | lit-compare | budget-init | budget-check'); return 1;
}
const self = fileURLToPath(import.meta.url);
if (process.argv[1] && path.resolve(process.argv[1]).toLowerCase() === self.toLowerCase()) {
  try { process.exit(main(process.argv.slice(2))); } catch (e) { console.error(e.message || e); process.exit(1); }
}
```

**Run:** `node scripts/test-system-world-picture-checks.mjs` → PASS. Then `node scripts/system-world-picture-checks.mjs sheet --dir reports/system-world-survey/phase-4-final --name resident-feet-0` writes a sheet if those frames exist (a smoke test on real PNGs; delete the output afterwards).

**Acceptance:** the test passes, and the smoke test decodes real survey PNGs without error.

**Commit:** `feat(system-world): picture checks (PNG codec, clipped share, masks, tones, sheets, budget) for the phase 6 survey`

---

## Task 2 — Baselines: frame-time budget, living JSON, sound (runs only, quiet machine)

**Files:** new `scripts/system-world-budget.json`, generated. The reports stay in the git-ignored `reports/`.

**Steps**
- [ ] **Timing, two runs per tier**, on the commit of Task 1 (app code is phase 5):
  ```powershell
  foreach ($t in 'low','medium','high') { foreach ($r in 1,2) { node scripts/system-world-survey.mjs --tier $t --timing --only overview-day,street-agent,life-park-day,facade-agent-night --out reports/system-world-survey/phase-6-t0-$t-$r --timeout 900; if ($LASTEXITCODE) { throw "$t $r" } } }
  ```
- [ ] **Budget file:**
  ```powershell
  node scripts/system-world-picture-checks.mjs budget-init --reports reports/system-world-survey/phase-6-t0-low-1,reports/system-world-survey/phase-6-t0-low-2,reports/system-world-survey/phase-6-t0-medium-1,reports/system-world-survey/phase-6-t0-medium-2,reports/system-world-survey/phase-6-t0-high-1,reports/system-world-survey/phase-6-t0-high-2 --out scripts/system-world-budget.json
  ```
- [ ] **Living JSON** (deterministic):
  ```powershell
  New-Item -ItemType Directory -Force reports/system-world-survey/phase-6-t0 | Out-Null
  node scripts/test-system-world-living.mjs | Select-Object -Last 1 | Set-Content -Encoding utf8 reports/system-world-survey/phase-6-t0/living.json
  ```
  Run it twice and confirm the two lines are identical. If they are not, stop and tell the coordinator: every later JSON proof depends on it.
- [ ] **Sound baseline:** `node scripts/system-world-sound-survey.mjs --label phase-6-t0 --baseline baseline --strict --timeout 1800`.

**Acceptance:**
- 6 timing reports, each with 4 `frameMs` entries and 0 page errors.
- The budget file lists 4 scenes × 3 tiers.
- The living JSON is stable.
- The sound survey's strict checks pass.

Report the medians table and the renderer string in the hand-back.

**Commit:** `chore(system-world): phase 6 frame-time budget measured on the phase 5 city (two runs per tier)`

---

## Task 3 — Survey instruments (W5 harness, W1 tools)

**Files:** `scripts/system-world-survey.mjs`, `scripts/system-world-survey.scenes.json`.

**Steps**
- [ ] **Header comment.** Document the new scene fields:
  - `freeze`, `toggles`;
  - `building`, `glare.rect`, `masks`, `feet`, `inventory`;
  - `checks` (`glareMax`, `paneOverCeramicMax`, `tonesMin`, `feetMax`), `allowInside`;
  - the `--strict` flag;
  - `viewTriangles` (no shadow pass in the captured frame);
  - camera checks (`camera.inside`, `camera.blocked`);
  - the visitor body follows the camera.
- [ ] **`parseArgs`.**
  - Add `strict: false` to the defaults object, next to `withoutFurniture: false`.
  - After `else if (a === '--without-furniture') o.withoutFurniture = true;` add `else if (a === '--strict') o.strict = true;`.
- [ ] **`buildBundle`.** After the line that does `text = replaceOnce(text, '  return {\n    districts, canvas, update, focus(id)', …)` (inside `if (isScene) {`) add:
  ```js
          text += "\nexport { SOLE as surveySole } from './sysworld-footing.js';\nexport { groundHeight as surveyGroundHeight } from './sysworld-exploration.js';\n";
  ```
  This is node code, outside `PAGE_JS`, so `\n` is fine here.
- [ ] **`PAGE_JS`: freeze flag.**
  - `let tickCount = 0, timingLoop = false;` → `let tickCount = 0, timingLoop = false, frozen = false;`
  - In `tick`, `if (!timingLoop) { city.update(dt, elapsed);` → `if (!timingLoop && !frozen) { city.update(dt, elapsed);`
- [ ] **`PAGE_JS`: the visitor body follows the camera.** In `applyCamera`, replace `    c.update();\n    return { position: pos, target: tgt };` with:
  ```js
    c.update();
    const body = sw.traffic.body('visitor');
    if (body) { body.x = sw.camera.position.x; body.y = sw.camera.position.y; body.z = sw.camera.position.z; }
    return { position: pos, target: tgt };
  ```
- [ ] **`PAGE_JS`: instruments.** Insert after the statement that defines `sub` (it ends with `furniture: s.furniture, nearHidden: nearApplied });`):
  ```js
  // ---- phase 6 instruments: building rectangle, camera check, inventory, toggles, masks, feet ----
  const T = sw.THREE;
  function districtObject(id) { let target = null; sw.scene.traverse(n => { if (!target && n.userData && n.userData.district === id) target = n; }); if (!target) throw Error('no district ' + id); return target; }
  function buildingCentre(id) { return new T.Box3().setFromObject(districtObject(id)).getCenter(new T.Vector3()); }
  // Screen rectangle [x0,y0,x1,y1] (fractions) of a district building; the whole frame when a corner lies behind the camera.
  function buildingRect(id) {
    const box = new T.Box3().setFromObject(districtObject(id)), v = new T.Vector3(), view = new T.Vector3();
    let x0 = 1, y0 = 1, x1 = 0, y1 = 0;
    sw.camera.updateMatrixWorld();
    for (let i = 0; i < 8; i++) {
      v.set(i & 1 ? box.max.x : box.min.x, i & 2 ? box.max.y : box.min.y, i & 4 ? box.max.z : box.min.z);
      if (view.copy(v).applyMatrix4(sw.camera.matrixWorldInverse).z > -sw.camera.near) return [0, 0, 1, 1];
      v.project(sw.camera);
      x0 = Math.min(x0, (v.x + 1) / 2); x1 = Math.max(x1, (v.x + 1) / 2); y0 = Math.min(y0, (1 - v.y) / 2); y1 = Math.max(y1, (1 - v.y) / 2);
    }
    return [Math.max(0, x0), Math.max(0, y0), Math.min(1, x1), Math.min(1, y1)].map(n => round(n, 4));
  }
  // The camera inside an instanced piece (upper 70 % of its box, horizontally 15 % inset: canopies and kiosks, not the
  // space under a tree) or overlapping a traffic solid at eye height.
  function cameraCheck() {
    const p = sw.camera.position, m = new T.Matrix4(), local = new T.Vector3(), inside = [];
    sw.staticCity.traverse(n => {
      if (!n.isInstancedMesh || !n.visible || !n.count) return;
      const g = n.geometry; if (!g.boundingBox) g.computeBoundingBox();
      const b = g.boundingBox, sx = (b.max.x - b.min.x) * .15, sz = (b.max.z - b.min.z) * .15, y0 = b.min.y + .3 * (b.max.y - b.min.y);
      for (let i = 0; i < n.count; i++) {
        n.getMatrixAt(i, m); local.copy(p).applyMatrix4(m.invert());
        if (local.x > b.min.x + sx && local.x < b.max.x - sx && local.y > y0 && local.y < b.max.y && local.z > b.min.z + sz && local.z < b.max.z - sz) { inside.push((n.material && n.material.name || 'mesh') + '#' + i); break; }
      }
    });
    const body = sw.traffic.body('visitor'), pose = { x: p.x, y: p.y, z: p.z, heading: 0 };
    return { inside, blocked: !!body && !sw.traffic.clear(body, pose, pose) };
  }
  const lights = () => { const out = []; sw.scene.traverse(n => { if (n.isLight) out.push(n); }); return out; };
  const materialsOf = () => { const set = new Set(); sw.scene.traverse(n => { if (n.isMesh) for (const m of Array.isArray(n.material) ? n.material : [n.material]) if (m) set.add(m); }); return [...set]; };
  // Lights (index = order of traversal, the index the light-off toggle takes) and emissive materials within 40 m.
  function inventory(centre) {
    const lightRows = lights().map((n, index) => ({ index, type: n.type, name: n.name, color: '#' + n.color.getHexString(), intensity: round(n.intensity, 3),
      distance: n.distance === undefined ? null : n.distance, position: n.getWorldPosition(new T.Vector3()).toArray().map(v => round(v, 1)) }));
    const emissive = new Map(), box = new T.Box3();
    sw.scene.traverse(n => {
      if (!n.isMesh || !n.visible) return;
      if (n.isInstancedMesh) { if (!n.boundingBox) n.computeBoundingBox(); box.copy(n.boundingBox).applyMatrix4(n.matrixWorld); } else box.setFromObject(n);
      if (box.isEmpty() || box.distanceToPoint(centre) > 40) return;
      for (const m of Array.isArray(n.material) ? n.material : [n.material]) {
        if (!m || !m.emissive) continue;
        const value = Math.max(m.emissive.r, m.emissive.g, m.emissive.b) * (m.emissiveIntensity === undefined ? 1 : m.emissiveIntensity);
        if (value <= 0) continue;
        const e = emissive.get(m.name) || { name: m.name, value: 0, meshes: 0 }; e.value = Math.max(e.value, round(value, 3)); e.meshes++; emissive.set(m.name, e);
      }
    });
    return { lights: lightRows, emissive: [...emissive.values()].sort((a, b) => b.value - a.value), bloomThreshold: round(sw.bloom.threshold, 3), environment: round(sw.scene.environmentIntensity, 3) };
  }
  // Toggles switch one suspect off while the city is frozen; each returns its undo.
  function applyToggle(spec) {
    const parts = spec.split(':'), kind = parts[0], arg = parts[1];
    if (kind === 'bloom-off') { const o = sw.bloom.enabled; sw.bloom.enabled = false; return () => { sw.bloom.enabled = o; }; }
    if (kind === 'env-off') { const o = sw.scene.environmentIntensity; sw.scene.environmentIntensity = 0; return () => { sw.scene.environmentIntensity = o; }; }
    if (kind === 'window-env-off') { const m = sw.windows.material, o = m.envMapIntensity; m.envMapIntensity = 0; return () => { m.envMapIntensity = o; }; }
    if (kind === 'windows-dark') { const v = sw.windows.uniforms.winState.value, o = v.clone(); v.set(0, o.y, 0, o.w); return () => { v.copy(o); }; }
    if (kind === 'surface-day') { sw.surfaces.setLighting(1, 0, 0); return () => { const m = sw.weather.mood(); sw.surfaces.setLighting(m.day, m.evening, m.weather === 'clear' ? 0 : 1, sw.scene.fog.color); }; }
    if (kind === 'lamps-off') { const hits = []; sw.scene.getObjectByName('city-atmosphere').children.forEach(n => { if (n.name === 'lamp-pools' || (n.isInstancedMesh && n.geometry.type === 'ConeGeometry')) { hits.push([n, n.visible]); n.visible = false; } }); return () => hits.forEach(h => { h[0].visible = h[1]; }); }
    if (kind === 'emissive-all-off' || kind === 'emissive-off') { const hits = materialsOf().filter(m => m.emissive && (kind === 'emissive-all-off' || m.name === arg)).map(m => [m, m.emissiveIntensity]); hits.forEach(h => { h[0].emissiveIntensity = 0; }); return () => hits.forEach(h => { h[0].emissiveIntensity = h[1]; }); }
    if (kind === 'light-off') { const l = lights()[Number(arg)]; if (!l) throw Error('no light ' + arg); const o = l.intensity; l.intensity = 0; return () => { l.intensity = o; }; }
    throw Error('unknown toggle ' + spec);
  }
  // Mask picture: named materials as flat colours, everything else black, no fog, sky, points or tone mapping.
  async function captureMask(name, masks) {
    const saved = [], basics = new Map(Object.keys(masks).map(k => [k, new T.MeshBasicMaterial({ color: new T.Color(masks[k]), toneMapped: false })]));
    const black = new T.MeshBasicMaterial({ color: 0x000000, toneMapped: false }), swap = m => basics.get(m && m.name) || black;
    sw.scene.traverse(n => {
      if (n.isMesh) { saved.push([n, 'material', n.material]); n.material = Array.isArray(n.material) ? n.material.map(swap) : swap(n.material); }
      else if ((n.isPoints || n.isLine || n.isSprite) && n.visible) { saved.push([n, 'visible', true]); n.visible = false; }
    });
    const fog = sw.scene.fog, background = sw.scene.background, tone = sw.renderer.toneMapping;
    sw.scene.fog = null; sw.scene.background = new T.Color(0); sw.renderer.toneMapping = T.NoToneMapping;
    let blob = null;
    try {
      sw.renderer.setRenderTarget(null); sw.renderer.render(sw.scene, sw.camera);
      blob = await new Promise(r => sw.renderer.domElement.toBlob(r, 'image/png'));
    } finally {
      for (const s of saved) s[0][s[1]] = s[2];
      sw.scene.fog = fog; sw.scene.background = background; sw.renderer.toneMapping = tone;
      basics.forEach(m => m.dispose()); black.dispose();
    }
    if (!blob) throw Error('mask capture returned no image');
    const res = await post('/__survey/shot?name=' + encodeURIComponent(name + '-mask'), blob, 'image/png');
    if (!res.ok) throw Error('mask upload failed ' + res.status);
  }
  // Height of each sole's lower contact above the ground under the sole centre (metres), for resident-N.
  function feetGaps(index) {
    const a = actors().find(r => r.id === 'resident-' + index);
    if (!a || !a.rig) return null;
    a.node.updateMatrixWorld(true);
    const v = new T.Vector3(), w = new T.Vector3(), sole = module.surveySole, ground = module.surveyGroundHeight;
    return a.rig.legs.map(l => { v.fromArray(sole[0]).applyMatrix4(l.foot.matrixWorld); w.fromArray(sole[1]).applyMatrix4(l.foot.matrixWorld);
      return round(Math.min(v.y, w.y) - ground((v.x + w.x) / 2, (v.z + w.z) / 2), 4); });
  }
  // One picture: flush a pending shadow pass first (so `triangles` counts the view only), then PNG, camera check, extras.
  async function captureFrame(scene, name, wanted) {
    wanted = applyCamera(scene) || wanted;
    syncNear();
    const flushed = sw.renderer.shadowMap.needsUpdate;
    if (flushed) sw.composer.render();
    sw.renderer.info.reset();
    const pending = city.capture();
    const stats = sub(city.stats());
    stats.viewTriangles = stats.triangles; stats.shadowFlushed = flushed;
    const actual = sw.camera.position.toArray().map(v => round(v));
    const blob = await pending;
    if (!blob) throw Error('capture returned no image');
    const res = await post('/__survey/shot?name=' + encodeURIComponent(name), blob, 'image/png');
    if (!res.ok) throw Error('shot upload failed ' + res.status);
    const drift = wanted ? Math.hypot(actual[0] - wanted.position[0], actual[1] - wanted.position[1], actual[2] - wanted.position[2]) : 0;
    const shot = { name, scene: scene.name, requestedPosition: wanted && wanted.position, actualPosition: actual, target: wanted && wanted.target,
      cameraMovedByCollision: drift > 0.05 ? round(drift) : 0, stats, camera: cameraCheck(), checks: scene.checks || null, allowInside: !!scene.allowInside };
    if (scene.building) shot.glareRect = buildingRect(scene.building);
    if (scene.glare && scene.glare.rect) shot.glareRect = scene.glare.rect;
    if (scene.feet !== undefined) shot.feet = feetGaps(scene.feet);
    if (scene.inventory) shot.inventory = inventory(scene.building ? buildingCentre(scene.building) : sw.controls.target);
    if (scene.masks) { await captureMask(name, scene.masks); shot.mask = true; }
    shots.push(shot);
    return wanted;
  }
  ```
- [ ] **`PAGE_JS`: the scene loop.** Replace the whole block from `  for (const scene of config.scenes) {` down to its closing `  }` (the line before `  const info = { tier: config.tier,`) with:
  ```js
  for (const scene of config.scenes) {
    const restore = [];
    try {
      city.setEnvironment({ time: scene.environment || 'day', weather: scene.weather || 'clear' });
      let wanted = applyCamera(scene);
      surveying = true; syncNear();
      await nextFrames(scene.frames === undefined ? 40 : scene.frames);
      await sleep(scene.waitMs === undefined ? 400 : scene.waitMs);
      if (scene.eval) await new AsyncFunction('city', '__sw', 'scene', scene.eval)(city, sw, scene);
      if (scene.freeze) { frozen = true; for (const t of scene.toggles || []) restore.push(applyToggle(t)); }
      const count = scene.sequence > 0 ? scene.sequence : 1;
      for (let i = 0; i < count; i++) {
        if (i > 0) await sleep(scene.intervalMs === undefined ? 350 : scene.intervalMs);
        wanted = await captureFrame(scene, count > 1 ? scene.name + '-' + String(i).padStart(2, '0') : scene.name, wanted);
      }
      if (config.timing) {
        applyCamera(scene); syncNear();
        await nextFrames(2); timingLoop = true; const gl = sw.renderer.getContext(), ms = [];
        for (let i = 0; i < 100; i++) { const t0 = performance.now(); elapsed += 1 / 60; city.update(1 / 60, elapsed); syncNear(); gl.finish(); if (i >= 10) ms.push(performance.now() - t0); }
        timingLoop = false; ms.sort((a, b) => a - b);
        shots[shots.length - 1].frameMs = { median: round(ms[Math.floor(ms.length / 2)], 2), p95: round(ms[Math.min(ms.length - 1, Math.ceil(ms.length * 0.95) - 1)], 2) };
        log('frameMs ' + scene.name + ' median ' + shots[shots.length - 1].frameMs.median + ' p95 ' + shots[shots.length - 1].frameMs.p95);
      }
    } catch (e) { addError('scene ' + scene.name, e && e.stack || e); }
    finally { for (const undo of restore.reverse()) { try { undo(); } catch (_) {} } frozen = false; }
  }
  ```
  The timing block is the old one, unchanged.
- [ ] **Node side: measurements and checks.** Add `import { decodePNG, clipped, maskOf, maskLumas, percentile, litShare, toneCount } from './system-world-picture-checks.mjs';` to the imports, and this function above `async function main()`:
  ```js
  // Measurements on the saved PNGs and the scene checks; returns the failed checks (they fail the run with --strict).
  async function measure(report, outDir) {
    const failures = [], load = async n => decodePNG(await fs.readFile(path.join(outDir, n + '.png')));
    for (const shot of report.shots || []) {
      const metrics = {}, result = {}, want = shot.checks || {};
      const check = (key, value, limit, ok) => { result[key] = { value, limit, ok }; if (!ok) failures.push(shot.name + ': ' + key + ' ' + JSON.stringify(value) + ' (limit ' + limit + ')'); };
      if (shot.glareRect) { const g = clipped(await load(shot.name), shot.glareRect); metrics.glare = { share: +g.share.toFixed(5), largest: g.largest, area: g.area }; }
      if (shot.mask) {
        const img = await load(shot.name), m = await load(shot.name + '-mask'), win = maskOf(m, [255, 0, 0]), cer = maskOf(m, [0, 255, 0]);
        const wl = maskLumas(img, win), cl = maskLumas(img, cer);
        metrics.panes = { pixels: wl.length, p99: +percentile(wl, .99).toFixed(1), litShare: +litShare(img, win).toFixed(3), tones: toneCount(img, win).tones };
        metrics.ceramic = { pixels: cl.length, p95: +percentile(cl, .95).toFixed(1) };
        metrics.paneOverCeramic = wl.length && cl.length ? +(metrics.panes.p99 / Math.max(1, metrics.ceramic.p95)).toFixed(3) : null;
      }
      if (want.glareMax !== undefined && metrics.glare) check('glare', metrics.glare.share, want.glareMax, metrics.glare.share <= want.glareMax);
      if (want.paneOverCeramicMax !== undefined && metrics.paneOverCeramic !== null && metrics.paneOverCeramic !== undefined) check('paneOverCeramic', metrics.paneOverCeramic, want.paneOverCeramicMax, metrics.paneOverCeramic <= want.paneOverCeramicMax);
      if (want.tonesMin !== undefined && metrics.panes) check('tones', metrics.panes.tones, want.tonesMin, metrics.panes.tones >= want.tonesMin);
      if (want.feetMax !== undefined) { const worst = shot.feet ? Math.max(...shot.feet.map(Math.abs)) : Infinity; check('feet', shot.feet, want.feetMax, worst <= want.feetMax); }
      if (!shot.allowInside && shot.camera && (shot.camera.blocked || shot.camera.inside.length)) check('camera', shot.camera, 'outside geometry', false);
      shot.metrics = metrics; shot.result = result;
    }
    return failures;
  }
  ```
  In `main()`, after `if (spawnError) { … }`, add:
  ```js
    let failures = [];
    if (report && report.shots && report.shots.length) {
      failures = await measure(report, outDir);
      await fs.writeFile(path.join(outDir, 'report.json'), JSON.stringify(report, null, 2));
    }
  ```
  After the `for (const e of report?.errors || [])` summary line, add:
  ```js
    for (const s of report?.shots || []) if (s.metrics && Object.keys(s.metrics).length) console.log('  ' + s.name + ' ' + JSON.stringify(s.metrics) + (s.feet ? ' feet ' + JSON.stringify(s.feet) : ''));
    for (const f of failures) console.log('  CHECK FAILED ' + f);
  ```
  Change the exit to `process.exit(reason === 'done' && report && !report.info?.failed && errorCount === 0 && !(opts.strict && failures.length) ? 0 : 1);`.
- [ ] **Scenes.** Add these fields to the existing one-line entries, keeping the style:
  - `facade-missions-night`: `"building":"missions","checks":{"glareMax":0.005}`
  - `facade-agent-night`: `"building":"agent","masks":{"city.window":"#ff0000","city.ceramic":"#00ff00"},"checks":{"glareMax":0.005}`
  - `facade-memory-night`: `"building":"memory","checks":{"glareMax":0.005}`
  - `life-street-trees-night`, `skyline-night`: `"glare":{"rect":[0,0,1,1]},"checks":{"glareMax":0.005}`
  - `facade-agent-day`, `facade-memory-day`, `facade-missions-day`, `proof-window-day`: `"masks":{"city.window":"#ff0000","city.ceramic":"#00ff00"},"checks":{"paneOverCeramicMax":1.0,"tonesMin":3}`
  - `lobby-agent-step-1` … `-4`: `"feet":0,"checks":{"feetMax":0.02}`
  - `crossing`, `kerb-close` (W6 zebra and kerb): `"glare":{"rect":[0,0.4,1,1]},"checks":{"glareMax":0.005}`
- [ ] **Validation run.** The app code is still phase 5, so these are the "before" metrics. Do not pass `--strict`:
  ```powershell
  node scripts/system-world-survey.mjs --tier high --out reports/system-world-survey/phase-6-before --timeout 1500 --only facade-missions-night,facade-agent-night,facade-memory-night,life-street-trees-night,skyline-night,facade-agent-day,facade-memory-day,facade-missions-day,proof-window-day,lobby-agent-step-1,lobby-agent-step-2,lobby-agent-step-3,lobby-agent-step-4,overview-day,street-agent,crossing,kerb-close,life-plaza-day,life-neon-night
  node scripts/system-world-survey.mjs --tier low --out reports/system-world-survey/phase-6-before-low --only facade-agent-night
  ```

**Acceptance:**
- `page errors: 0` in both runs.
- Every mask scene has `metrics.panes.pixels > 1000`, which proves the mask path.
- The step scenes report `feet` as two numbers.
- `overview-day` and `street-agent` report both `triangles` and `viewTriangles`. Name the `street-agent` figure in the hand-back; it is the W4 starting point.
- `cameraMovedByCollision: 0` where it was 0 before.
- Report every `camera` failure by name; the coordinator decides on `allowInside` or a camera move (backlog 25).

**Commit:** `feat(system-world): survey instruments (freeze and toggles, building rectangle, masks, view triangles, camera check, feet, checks)`

---

## Task 4 — Find the night glare (W1 find; runs only)

**Files:** none in the repo. Scratch files only.

**Steps**
- [ ] **Scenes generator.** Write `<scratch>/make-find-scenes.mjs`:
  ```js
  // Scratch helper: find-step scenes from the survey scenes file. node make-find-scenes.mjs <scenes.json> <out.json> <1|2> [lights.json]
  import fs from 'node:fs';
  const [, , scenesFile, out, pass, lightsFile] = process.argv;
  const all = JSON.parse(fs.readFileSync(scenesFile, 'utf8')), pick = n => { const s = all.find(x => x.name === n); if (!s) throw Error('missing ' + n); return { ...s }; };
  const TARGETS = [['facade-missions-night', { building: 'missions' }], ['facade-agent-night', { building: 'agent' }], ['facade-memory-night', { building: 'memory' }],
    ['lobby-memory-night', { building: 'memory' }], ['lobby-missions-night', { building: 'missions' }], ['lobby-agent-night', { building: 'agent' }],
    ['life-street-trees-night', { glare: { rect: [0, 0, 1, 1] } }], ['skyline-night', { glare: { rect: [0, 0, 1, 1] } }]];
  const VARIANTS = ['bloom-off', 'env-off', 'window-env-off', 'windows-dark', 'surface-day', 'emissive-all-off', 'lamps-off'];
  const lights = lightsFile ? JSON.parse(fs.readFileSync(lightsFile, 'utf8')) : {};
  const scenes = [];
  for (const [name, extra] of TARGETS) {
    const base = { ...pick(name), ...extra, freeze: true, checks: null, masks: undefined };
    if (pass === '1') { scenes.push({ ...base, name: name + '--base', inventory: true }); continue; }
    for (const v of VARIANTS) scenes.push({ ...base, name: name + '--' + v, toggles: [v] });
    for (const i of lights[name] || []) scenes.push({ ...base, name: name + '--light-' + i, toggles: ['light-off:' + i] });
  }
  fs.writeFileSync(out, JSON.stringify(scenes, null, 1));
  ```
- [ ] **Pass 1** (base pictures and inventory): run the generator with `1`, then
  `node scripts/system-world-survey.mjs --scenes <scratch>/find-1.json --out reports/system-world-survey/phase-6-find-1 --timeout 1200`.
- [ ] **Pick lights.** From each target's `inventory.lights`, choose every DirectionalLight and HemisphereLight, plus every PointLight within 80 m of the building centre. Write `<scratch>/lights.json` as `{"<target>":[indices]}`.
- [ ] **Pass 2** (variants): run the generator with `2 <scratch>/lights.json`, then the survey with `--out reports/system-world-survey/phase-6-find-2 --timeout 2400`.
- [ ] **Aviation probe.** Add one scratch scene `aviation-probe`: the `skyline-night` camera and this `eval`, which posts to the survey log:
  ```js
  const m=await import('/__survey/bundle.js'),T=__sw.THREE,v=new T.Vector3(),tops={},out=[];for(const id of ['data-tower-a','data-tower-b']){const g=await __sw.model(id,2,'v3');g.updateMatrixWorld(true);let top=-1e9;const pts=[];g.traverse(n=>{const p=n.isMesh&&n.geometry.getAttribute('position');if(!p)return;for(let i=0;i<p.count;i++){v.fromBufferAttribute(p,i).applyMatrix4(n.matrixWorld);pts.push(v.x,v.y,v.z);if(v.y>top)top=v.y;}});let x=0,z=0,k=0;for(let i=0;i<pts.length;i+=3)if(pts[i+1]>=top-.5){x+=pts[i];z+=pts[i+2];k++;}tops[id]={x:+(x/k).toFixed(2),z:+(z/k).toFixed(2),y:+top.toFixed(2)};}const d=__sw.scene.fog.density;for(const p of m.placements)if(tops[p.asset]&&(p.z<-100||p.scale[1]>1.1)){const t=tops[p.asset],c=Math.cos(p.angle||0),s=Math.sin(p.angle||0),lx=t.x*p.scale[0],lz=t.z*p.scale[2],r=Math.hypot(p.x-__sw.camera.position.x,p.z-__sw.camera.position.z);out.push({x:p.x,z:p.z,offset:+Math.hypot(lx*c+lz*s,-lx*s+lz*c).toFixed(2),fog:+Math.exp(-Math.pow(d*r,2)).toFixed(3)});}await fetch('/__survey/log',{method:'POST',body:'aviation '+JSON.stringify({tops,points:out})});
  ```
- [ ] **Cone probe.** Add one scratch scene `cone-probe`: the `life-street-trees-night` camera, with an `eval` that computes, for each `street-lamp` placement in `m.placements`, the camera's distance to that lamp's cone. Use the cone formula from Task 5 (apex at `y+6.5` above the luminaire at 1.55 m along the lamp's angle, radius 2.7 at the pavement). Log the three nearest distances.
- [ ] **Blob and low tier.**
  - Run `proof-window-close` at `--tier medium` with `"glare":{"rect":[0,0,1,1]}`.
  - Take the low-tier lit share from Task 3's `phase-6-before-low` and the high one from `phase-6-before`:
    `node scripts/system-world-picture-checks.mjs lit-compare --a reports/system-world-survey/phase-6-before --b reports/system-world-survey/phase-6-before-low --scene facade-agent-night`

**Report** (in the hand-back, and copied by the coordinator into the plan's execution notes):
1. **Glare table.** One row per target and variant: `glare.share`, `largest`, and Δ against the base. A variant **explains** a patch when the base share exceeds .005 and the variant cuts it by at least 50 %. Several variants may each explain part of it.
2. **Emissive inventory** per target, with values above `bloomThreshold` marked.
3. **Lights inventory** per target.
4. **Aviation:** the offsets and fog factors (the offset is the distance between the old origin-based point and the roof-top centre).
5. **Cone:** the nearest cone distances.
6. **Blob (item 5):** the largest clipped blob in `proof-window-close` on medium. If it is under 400 px, close item 5 with that figure.
7. **Low tier:** lit share low against high. If they are within 10 points, Task 6 step 5 is optional.
8. **Decision table**, filled in:

| Variant that explains the patch | Class | Fix |
| --- | --- | --- |
| `windows-dark` | B: window room light | Task 6 step 6 (`WIN_CAP`) |
| `window-env-off` | E-w: glass reflection | Task 6 step 7 (window env at night) |
| `env-off` without `window-env-off` | E: environment on every material | Task 8 step E |
| `emissive-off`/`-all-off`, exterior kit part | A | C1 (Blender) or Task 8 step F |
| `emissive-…`, interior fixture | A-in | Task 8 step L |
| `surface-day` | F: pane lighting on ivory/warm | Task 8 step F |
| `light-off:<sun>` | M: moonlight | Task 8 step M |
| `light-off:<point light>` | C: real light near a pale wall | Task 8 step C |
| `lamps-off` | cone | Task 5 (always) |
| only `bloom-off` | G | Task 8 step G (last resort, coordinator's call) |
| aviation offset ≥ 1 m | D | Task 5 (always) |
| aviation offset < 1 m and fog factor < .5 | D-fog | Task 5 step 5 |

**Acceptance:** every patch named in W1 has at least one explaining variant or an explicit "unexplained" note. The coordinator then picks the fix steps.

**Commit:** none (no repo change).

---

## Task 5 — Lamp cones near the camera; aviation lights on the roofs (W1)

**Files:** `ui/js/desktop/apps/sysworld-atmosphere.js`, `ui/js/desktop/apps/sysworld-scene.js` (only the `rebuild` aviation lines and the import), `scripts/test-system-world-lamps.mjs`, new `scripts/test-system-world-aviation.mjs`.

**Steps**
- [ ] **Step 1. Tests first.**
  - Extend `scripts/test-system-world-lamps.mjs`. Change the import to `import {createAtmosphere,coneFade} from …`. Before `atmosphere.dispose();` add:
    ```js
    // Phase 6: cones fade out near the camera (0 inside, 1 beyond 3 m of the cone volume), per instance.
    { const c={x:0,z:0,bottom:0,top:6.5,radius:2.7};
      assert.equal(coneFade(0,1,0,c),0,'inside the cone');assert.equal(coneFade(10,1,0,c),1,'far away');
      const mid=coneFade(2.7+1.5,0,0,c);assert.ok(mid>0&&mid<1,'fading band');
      let last=-1;for(let d=0;d<6;d+=.25){const f=coneFade(2.7+d,0,0,c);assert.ok(f>=last,'monotonic');last=f;}
      assert.equal(coneFade(0,9.6,0,c),1,'3.1 m above the apex'); }
    { const s=new THREE.Scene(),a=createAtmosphere(s,{lamps:[{x:0,z:0,y:0,angle:0}]}),cam=new THREE.PerspectiveCamera();a.setTier('medium');a.setLighting(0,0,up);
      const mesh=s.getObjectByName('city-atmosphere').children.find(n=>n.isInstancedMesh&&n.geometry.type==='ConeGeometry'),fade=mesh.geometry.getAttribute('fade');
      assert.ok(fade&&fade.isInstancedBufferAttribute,'per-instance fade attribute');
      cam.position.set(1.55,2,0);a.update(.016,1,cam,true);assert.equal(fade.array[0],0,'camera inside: cone faded out');
      cam.position.set(30,2,0);a.update(.016,1,cam,true);assert.ok(fade.array[0]>.99,'camera far: full cone');
      assert.ok(mesh.material.fragmentShader.includes('vFade'),'the fragment shader applies the fade');a.dispose(); }
    ```
  - Write `scripts/test-system-world-aviation.mjs` (complete):
    ```js
    // Aviation lights stand on the centre of each tower's roof: roofTop finds the highest vertices' centre, aviationPoints turns
    // and scales it with the placement; on the shipped LOD 2 towers every light lies over its roof.
    import assert from 'node:assert/strict';
    import fs from 'node:fs/promises';
    import * as THREE from 'three';
    import {GLTFLoader} from 'three/addons/loaders/GLTFLoader.js';
    import {MeshoptDecoder} from 'three/addons/libs/meshopt_decoder.module.js';
    import {roofTop,aviationPoints} from '../ui/js/desktop/apps/sysworld-atmosphere.js';
    import {placements} from '../ui/js/desktop/apps/sysworld-scene.js';

    { // a 10 m block with a 2 m mast off-centre: the top is the mast
      const g=new THREE.Group(),block=new THREE.Mesh(new THREE.BoxGeometry(8,10,8));block.position.y=5;
      const mast=new THREE.Mesh(new THREE.BoxGeometry(.4,2,.4));mast.position.set(3,11,-1);g.add(block,mast);
      const r=roofTop(g);assert.ok(Math.abs(r.y-12)<1e-6&&Math.abs(r.x-3)<1e-6&&Math.abs(r.z+1)<1e-6,JSON.stringify(r));
      const [p]=aviationPoints([{asset:'t',x:10,y:-3,z:-150,angle:Math.PI/2,scale:[2,1.5,2]}],new Map([['t',r]]));
      // local (3,-1) scaled to (6,-2), turned 90 degrees: x' = -2, z' = -6
      assert.ok(Math.abs(p.x-8)<1e-9&&Math.abs(p.z+156)<1e-9&&Math.abs(p.y-(-3+12*1.5+.6))<1e-9,JSON.stringify(p));
      assert.equal(aviationPoints([{asset:'t',x:0,y:0,z:0,angle:0,scale:[1,1,1]}],new Map([['t',r]])).length,0,'near, unscaled towers carry no light');
    }
    await MeshoptDecoder.ready;
    const loader=new GLTFLoader().setMeshoptDecoder(MeshoptDecoder),kit=JSON.parse(await fs.readFile('ui/3d/system-world/v3/manifest.json','utf8')),roofs=new Map(),bands=new Map();
    for(const id of ['data-tower-a','data-tower-b']){
      const lod=kit.assets.find(a=>a.id===id).lods.find(l=>l.level===2),data=await fs.readFile('ui/3d/system-world/v3/'+lod.file);
      const gltf=await loader.parseAsync(data.buffer.slice(data.byteOffset,data.byteOffset+data.byteLength),'');const r=roofTop(gltf.scene);roofs.set(id,r);
      const v=new THREE.Vector3(),box=new THREE.Box2();box.makeEmpty();gltf.scene.updateMatrixWorld(true);
      gltf.scene.traverse(n=>{const p=n.isMesh&&n.geometry.getAttribute('position');if(!p)return;for(let i=0;i<p.count;i++){v.fromBufferAttribute(p,i).applyMatrix4(n.matrixWorld);if(v.y>=r.y-.5)box.expandByPoint(new THREE.Vector2(v.x,v.z));}});
      bands.set(id,box);assert.ok(box.containsPoint(new THREE.Vector2(r.x,r.z)),id+': roof centre lies inside its top band');
    }
    const lit=placements.filter(p=>roofs.has(p.asset)&&(p.z<-100||p.scale[1]>1.1)),points=aviationPoints(placements,roofs);
    assert.equal(points.length,lit.length,'one light per tall or skyline tower');
    points.forEach((q,i)=>{const p=lit[i],c=Math.cos(p.angle||0),s=Math.sin(p.angle||0),dx=q.x-p.x,dz=q.z-p.z,lx=(dx*c-dz*s)/p.scale[0],lz=(dx*s+dz*c)/p.scale[2];
      assert.ok(bands.get(p.asset).clone().expandByScalar(.05).containsPoint(new THREE.Vector2(lx,lz)),'light '+i+' floats beside its roof');
      assert.ok(Math.abs(q.y-(p.y+roofs.get(p.asset).y*p.scale[1]+.6))<1e-9,'0.6 m above the roof');});
    console.log(`System World aviation: ${points.length} lights on roof tops (offsets from the origin ${[...roofs].map(([id,r])=>id+' '+Math.hypot(r.x,r.z).toFixed(2)+' m').join(', ')}).`);
    ```
    The inverse turn in the per-light check: for world = (lx·c + lz·s, −lx·s + lz·c), local = (dx·c − dz·s, dx·s + dz·c).
- [ ] **Step 2. Atmosphere** (`sysworld-atmosphere.js`). Above `export function createAtmosphere(` add:
  ```js
  // Phase 6: a lamp's cone fades out when the camera is inside it or within CONE_FADE.outer metres (it would veil the frame).
  export const CONE_FADE = Object.freeze({ inner: .5, outer: 3 });
  export function coneFade(x, y, z, cone, inner = CONE_FADE.inner, outer = CONE_FADE.outer) {
    const h = Math.min(cone.top, Math.max(cone.bottom, y)), r = cone.radius * (cone.top - h) / Math.max(1e-6, cone.top - cone.bottom);
    const radial = Math.max(0, Math.hypot(x - cone.x, z - cone.z) - r), vertical = y < cone.bottom ? cone.bottom - y : y > cone.top ? y - cone.top : 0;
    const t = Math.min(1, Math.max(0, (Math.hypot(radial, vertical) - inner) / (outer - inner)));
    return t * t * (3 - 2 * t);
  }
  // Phase 6: the top centre of a model in its own frame, the mean x/z of the vertices within `band` metres of its highest
  // point; and the aviation lights of the tall or skyline towers on it, turned and scaled with each placement.
  export function roofTop(object, band = .5) {
    object.updateMatrixWorld(true);
    const v = new THREE.Vector3(), pts = []; let top = -Infinity;
    object.traverse(n => { const p = n.isMesh && n.geometry.getAttribute('position'); if (!p) return;
      for (let i = 0; i < p.count; i++) { v.fromBufferAttribute(p, i).applyMatrix4(n.matrixWorld); pts.push(v.x, v.y, v.z); if (v.y > top) top = v.y; } });
    let x = 0, z = 0, n = 0;
    for (let i = 0; i < pts.length; i += 3) if (pts[i + 1] >= top - band) { x += pts[i]; z += pts[i + 2]; n++; }
    return n ? { x: x / n, y: top, z: z / n } : { x: 0, y: 0, z: 0 };
  }
  export function aviationPoints(placements, roofs, lift = .6) {
    return placements.filter(p => roofs.has(p.asset) && (p.z < -100 || p.scale[1] > 1.1)).map(p => {
      const r = roofs.get(p.asset), c = Math.cos(p.angle || 0), s = Math.sin(p.angle || 0), lx = r.x * p.scale[0], lz = r.z * p.scale[2];
      return { x: p.x + lx * c + lz * s, y: p.y + r.y * p.scale[1] + lift, z: p.z - lx * s + lz * c };
    });
  }
  ```
  - In `createAtmosphere`, in the lamp material's vertex string, replace `varying vec2 vUv;varying vec3 vNormal,vView;void main(){vUv=uv;vNormal=normalize(normalMatrix*normal);` with `attribute float fade;varying float vFade;varying vec2 vUv;varying vec3 vNormal,vView;void main(){vFade=fade;vUv=uv;vNormal=normalize(normalMatrix*normal);`.
  - In `LAMP`, replace `uniform vec3 color;uniform float strength;varying vec2 vUv;varying vec3 vNormal,vView;` with `uniform vec3 color;uniform float strength;varying float vFade;varying vec2 vUv;varying vec3 vNormal,vView;`, and `float a=pow(h,2.4)*pow(facing,1.5)*strength;` with `float a=pow(h,2.4)*pow(facing,1.5)*strength*vFade;`.
  - After `lampCones.count = lamps.length; lampCones.instanceMatrix.needsUpdate = true; group.add(lampCones);` add:
    ```js
    // Per-lamp cone volumes for the near-camera fade (same placement as the matrices above).
    const cones = lamps.map(lamp => ({ x: lamp.x + 1.55 * Math.cos(lamp.angle || 0), z: lamp.z - 1.55 * Math.sin(lamp.angle || 0), bottom: surfaces.pavement, top: (lamp.y || 0) + 6.5, radius: 2.7 }));
    const lampFade = new THREE.InstancedBufferAttribute(new Float32Array(Math.max(1, lamps.length)).fill(1), 1);
    lampGeometry.setAttribute('fade', lampFade);
    ```
  - In `update(…)`, after `lampMaterial.uniforms.strength.value = .3 * lampFlicker; poolMaterial.uniforms.strength.value = .6 * lampFlicker;` add:
    ```js
      if (lampCones.visible) { let dirty = false; const p = camera.position;
        for (let i = 0; i < cones.length; i++) { const f = coneFade(p.x, p.y, p.z, cones[i]); if (Math.abs(f - lampFade.array[i]) > .01 || (f === 0) !== (lampFade.array[i] === 0)) { lampFade.array[i] = f; dirty = true; } }
        if (dirty) lampFade.needsUpdate = true; }
    ```
- [ ] **Step 3. Scene** (`sysworld-scene.js`).
  - Import: `import { createAtmosphere } from './sysworld-atmosphere.js';` → `import { createAtmosphere, roofTop, aviationPoints } from './sysworld-atmosphere.js';`
  - In `rebuild()`, replace the three lines from `const box = new THREE.Box3(), roofs = new Map();` through `atmosphere.setAviation(placements.filter(…));` with:
    ```js
      // Aviation lights stand on the centre of each tower's roof, turned and scaled with the placement (phase 6).
      const roofs = new Map(['data-tower-a', 'data-tower-b'].map(id => [id, roofTop(distant.get(id))]));
      atmosphere.setAviation(aviationPoints(placements, roofs));
    ```
- [ ] **Step 4.** Run the tests (below).
- [ ] **Step 5. Only if Task 4 found class D-fog:**
  - In `AVIATION.vertex`, add `uniform float fogDensity;` and, after the `vA=` assignment, `vA*=exp(-0.5*fogDensity*fogDensity*dot(mv.xyz,mv.xyz));` (the exponent factor .5 may be tuned between .3 and 1).
  - Add `fogDensity: { value: 0 }` to the aviation uniforms.
  - In `update`, set `aviationMaterial.uniforms.fogDensity.value = scene.fog?.density || 0;`.
  - Add a lamps-test line asserting that the shader includes `fogDensity`.

**Tuning (coordinator, after pictures):** `CONE_FADE.inner` 0–1, `.outer` 2–4.

**Run:**
```
node scripts/test-system-world-lamps.mjs
node scripts/test-system-world-aviation.mjs
node scripts/test-system-world-living.mjs
node scripts/system-world-survey.mjs --tier high --out reports/system-world-survey/phase-6-t5 --only life-street-trees-night,skyline-night
```
The living run is a regression check only.

**Acceptance:**
- The tests pass.
- `life-street-trees-night` glare ≤ .005 if the cone was its cause (Task 4).
- In `skyline-night`, the coordinator sees no light beside a tower.

**Commit:** `fix(system-world): lamp cones fade near the camera; aviation lights stand on roof tops`

---

## Task 6 — Windows by day and on low; window night fixes (W2, W1 classes B and E-w)

**Files:** `ui/js/desktop/apps/sysworld-windows.js`, `scripts/test-system-world-windows.mjs`.

**Steps**
- [ ] **Step 1. Test first.**
  - Import `WINDOW_TONES` in `scripts/test-system-world-windows.mjs`.
  - Before `// 8. Routing` add:
    ```js
    // Phase 6: at least five wall tones, cumulative shares ending at 1, tones distinct and spread in brightness; generated once
    // into the shader; darker furniture; daylight falls to 12 % at the back of a room.
    assert.ok(WINDOW_TONES.length>=5,'five or more wall tones');
    WINDOW_TONES.forEach((t,i)=>{assert.ok(t.upTo>(i?WINDOW_TONES[i-1].upTo:0)&&t.upTo<=1,'ascending shares');assert.equal(t.color.length,3);});
    assert.equal(WINDOW_TONES.at(-1).upTo,1,'the last tone closes the range');
    for(let i=0;i<WINDOW_TONES.length;i++)for(let j=i+1;j<WINDOW_TONES.length;j++)assert.ok(Math.hypot(...WINDOW_TONES[i].color.map((c,k)=>c-WINDOW_TONES[j].color[k]))>=.1,'tones '+i+' and '+j+' too close');
    const lum=c=>.2126*c[0]+.7152*c[1]+.0722*c[2],l=WINDOW_TONES.map(t=>lum(t.color));assert.ok(Math.max(...l)-Math.min(...l)>=.25,'tones spread in brightness');
    assert.equal(WINDOW_SHADER.fragmentPars.split('vec3 winTone(float h)').length,2);assert.equal(WINDOW_SHADER.fragmentMain.split('winTone(winH3)').length,2);
    assert.ok(WINDOW_SHADER.fragmentPars.includes('fill*(0.12+0.88*(1.0-z)*(1.0-z))'),'daylight falls off with depth');
    assert.ok(WINDOW_SHADER.fragmentMain.includes('vec3(0.07,0.065,0.06)'),'darker furniture');
    ```
- [ ] **Step 2. Tones.** Above `export const WINDOW_SHADER = {` add:
  ```js
  // Room wall tones (phase 6): linear albedo and the cumulative share of rooms (the last entry ends at 1). Warm white,
  // cool grey-blue, sage, terracotta, teal and charcoal, so large facades read as rooms, not as rows of pale boxes.
  export const WINDOW_TONES = Object.freeze([
    Object.freeze({ upTo: .28, color: Object.freeze([.62, .56, .48]) }),
    Object.freeze({ upTo: .5, color: Object.freeze([.40, .48, .55]) }),
    Object.freeze({ upTo: .64, color: Object.freeze([.42, .50, .40]) }),
    Object.freeze({ upTo: .76, color: Object.freeze([.55, .38, .30]) }),
    Object.freeze({ upTo: .88, color: Object.freeze([.22, .36, .38]) }),
    Object.freeze({ upTo: 1, color: Object.freeze([.26, .25, .27]) }),
  ]);
  const glslFloat = v => (Number.isInteger(v) ? v.toFixed(1) : String(v));
  const TONE_GLSL = 'vec3 winTone(float h){return ' + WINDOW_TONES.slice(0, -1).map(t => 'h<' + glslFloat(t.upTo) + '?vec3(' + t.color.map(glslFloat).join(',') + '):').join('')
    + 'vec3(' + WINDOW_TONES.at(-1).color.map(glslFloat).join(',') + ');}';
  ```
- [ ] **Step 3. Shader edits** in `WINDOW_SHADER`.
  - `fragmentPars`:
    - Replace `vec3 winLightAt(vec3 lamp,vec3 fill,float z){return min(lamp*(1.0-0.3*z)+fill*(1.0-0.67*z),vec3(1.2));}` with `vec3 winLightAt(vec3 lamp,vec3 fill,float z){return min(lamp*(1.0-0.3*z)+fill*(0.12+0.88*(1.0-z)*(1.0-z)),vec3(1.2));}`
    - Append `\n${TONE_GLSL}` at the end of the template, just before its closing backtick.
    - Update the comment above `winLightAt` to say "daylight falls to 12 % at the back".
  - `fragmentMain`:
    - Replace `vec3 winWall=mix(vec3(0.62,0.56,0.48),vec3(0.40,0.48,0.55),step(0.55,winH1));` with `float winH3=winHash(winKey*1.73+5.9);\nvec3 winWall=winTone(winH3);`
    - Replace `vec3 winFill=vec3(0.94,1.0,1.06)*(0.012+0.26*winDay);` with `vec3 winFill=vec3(0.94,1.0,1.06)*(0.012+0.2*winDay);`
    - Replace `winRoomCol=mix(winRoomCol,winWall*0.3*winLightAt(winLampL,winFill,0.55),winFurn);` with `winRoomCol=mix(winRoomCol,vec3(0.07,0.065,0.06)*(0.8+0.4*fract(winH1*5.0))*winLightAt(winLampL,winFill,0.55),winFurn);`
- [ ] **Step 4.** Run the windows test.
- [ ] **Step 5. Low tier: a flat lit cell.** Required unless Task 4 found the low lit share within 10 points of high. Replace the low branch line `winRoomCol=winWall*winLightAt(winLampL,winFill,0.4)*0.8+winLamp*winOn*mix(0.1,0.2,winDusk)*smoothstep(0.35,1.0,winF.y);` with:
  ```glsl
  winRoomCol=winWall*winLightAt(winLampL,winFill,0.4)*0.6+winLamp*winOn*mix(0.12,0.62,winDusk)*(0.75+0.25*smoothstep(0.35,1.0,winF.y));
  ```
  Update the comment: "Low tier: a flat cell; lit rooms show a warm or cool tint per room seed, a little brighter under the ceiling".
- [ ] **Step 6. Only for class B** (Task 4: `windows-dark` explains a patch).
  - In `createWindows`, extend the `head` string: after `#define WIN_STOREY 3.05\n` add `#define WIN_CAP 1.1\n`.
  - Directly before `winRoomCol*=1.0-winFres;` insert `winRoomCol=min(winRoomCol,vec3(WIN_CAP));`.
  - Test: the compiled fragment contains `#define WIN_CAP` and `min(winRoomCol,vec3(WIN_CAP))`.
  - Tuning range for `WIN_CAP`: .85–1.2.
- [ ] **Step 7. Only for class E-w** (`window-env-off` explains).
  - Add `const WINDOW_ENV = { day: 1, night: .35 };` above `createWindows`.
  - In `setLighting`, after `uniforms.winState.value.set(…)`, add `material.envMapIntensity = WINDOW_ENV.night + (WINDOW_ENV.day - WINDOW_ENV.night) * clamp01(day);`.
  - Test after `windows.setLighting(0, 1, 0, …)`: `assert.ok(near(m.envMapIntensity,.35))`. After `setLighting(1,0)`: `near(m.envMapIntensity,1)`.
  - Tuning range for `night`: .2–.6.

**Tuning (coordinator, after pictures):**
- tone colours and shares (keep the test's distance and spread rules);
- day fill .2 (.14–.26); back share .12 (.08–.25);
- furniture albedo .07 (.05–.12);
- low-tier night lamp .62 (.45–.8).

**Run:**
```
node scripts/test-system-world-windows.mjs
node scripts/system-world-survey.mjs --tier high --out reports/system-world-survey/phase-6-t6 --only facade-agent-day,facade-memory-day,facade-missions-day,proof-window-day,facade-agent-night,proof-window-close
node scripts/system-world-survey.mjs --tier low --out reports/system-world-survey/phase-6-t6-low --only facade-agent-night
node scripts/system-world-picture-checks.mjs lit-compare --a reports/system-world-survey/phase-6-t6 --b reports/system-world-survey/phase-6-t6-low --scene facade-agent-night
```

**Acceptance:**
- The test passes.
- The day scenes meet `tones ≥ 3` and `paneOverCeramic ≤ 1.0` (their checks are in the report).
- `lit-compare` exits 0.
- The coordinator confirms the facades read as rooms.

**Commit:** `feat(system-world): window rooms by day (six wall tones, darker furniture, depth falloff) and lit cells on the low tier`

---

## Task 7 — Feet on the treads (W3)

**Files:** `ui/js/desktop/apps/sysworld-lobbies.js`, `ui/js/desktop/apps/sysworld-footing.js`, `ui/js/desktop/apps/sysworld-society.js` (`settle` and its import only), `scripts/test-system-world-footing.mjs`, `scripts/test-system-world-lobbies.mjs`.

**Steps**
- [ ] **Step 1. Tests first.**
  - `scripts/test-system-world-lobbies.mjs`: import `treadCentre, FOOT_HALF` and add:
    ```js
    { const p=PODIUMS.find(q=>q.id==='agent'),near=(a,b)=>Math.abs(a-b)<1e-9;
      assert.ok(near(treadCentre(0,p.z1+.1),p.z1+TREAD/2),'first tread centre');
      assert.ok(near(treadCentre(0,p.z1+1.5),p.z1+4.5*TREAD),'last tread centre');
      assert.ok(near(treadCentre(0,p.z1-.05),p.z1-FOOT_HALF),'landing lip: the sole stays behind the top edge');
      assert.ok(near(treadCentre(0,p.z1-.3),p.z1-.3),'deeper on the podium: unchanged');
      assert.ok(Number.isNaN(treadCentre(20,p.z1+.5)),'beside the flight');assert.ok(Number.isNaN(treadCentre(0,p.z1+FLIGHT+.1)),'below the flight');
      for(let s=.01;s<FLIGHT;s+=.07)assert.ok(near(lobbyGround(0,treadCentre(0,p.z1+s)),lobbyGround(0,p.z1+s)),'the centre lies on the same tread'); }
    ```
    The existing file imports the registry constants; add any missing names (`PODIUMS`, `TREAD`, `FLIGHT`, `lobbyGround`) to the import.
  - `scripts/test-system-world-footing.mjs`: import `soleGaps` and `{lobbyGround,treadCentre,PODIUMS}` from `../ui/js/desktop/apps/sysworld-lobbies.js`. Before the `// --- The shipped resident models ---` block add:
    ```js
    { // stairs: the agent flight; body origins every 8 cm from the podium lip down the flight, walking up and down
      const p=PODIUMS.find(q=>q.id==='agent'),ground=(x,z)=>lobbyGround(x,z)??0;let worst=0,before=0;
      for(let k=0;k<=24;k++)for(const heading of [0,Math.PI]){
        const z=p.z1-.2+k*.08,make=()=>{const root=rig();root.rotation.y=heading;root.position.set(0,ground(0,z),z);return root;};
        const old=make(),ro=findRig(old);adaptFeet(ro,old,ground);old.updateMatrixWorld(true);before=Math.max(before,...soleGaps(ro,ground).map(Math.abs));
        const root=make(),r=findRig(root);adaptFeet(r,root,ground,{snap:treadCentre});root.updateMatrixWorld(true);worst=Math.max(worst,...soleGaps(r,ground).map(Math.abs));
      }
      assert.ok(worst<=.02,'both soles within 2 cm of the tread under them: '+worst.toFixed(3));
      assert.ok(before>.05,'without the snap a sole hangs over a riser (the defect this fixes): '+before.toFixed(3));
    }
    { // the snap changes nothing off the flights: kerb case with and without it
      const a=rig(),b=rig(),ra=findRig(a),rb=findRig(b),ground=x=>x<0?.14:0;adaptFeet(ra,a,ground);adaptFeet(rb,b,ground,{snap:treadCentre});
      ra.legs.forEach((l,i)=>['leg','shin','foot'].forEach(k=>assert.ok(l[k].quaternion.angleTo(rb.legs[i][k].quaternion)<1e-9,'kerb pose changed by the snap')));
    }
    ```
    In the shipped-models loop, after the kerb asserts, add:
    ```js
      { const p=PODIUMS.find(q=>q.id==='agent'),g=(x,z)=>lobbyGround(x,z)??0,root2=new THREE.Group();root2.add(gltf.scene.clone(true));root2.scale.setScalar(1.15);
        root2.position.set(0,g(0,p.z1+.3),p.z1+.3);root2.rotation.y=Math.PI;const r2=findRig(root2);adaptFeet(r2,root2,g,{snap:treadCentre});root2.updateMatrixWorld(true);
        assert.ok(Math.max(...soleGaps(r2,g).map(Math.abs))<=.02,name+': soles on the treads'); }
    ```
- [ ] **Step 2. Registry** (`sysworld-lobbies.js`). After the `lobbyGround` function add:
  ```js
  // Feet on the flights (phase 6): the z of the tread centre under (x, z) on a stair flight; on the podium within one tread of
  // a flight's top edge, the furthest z that keeps a resident sole (FOOT_HALF, half of the 0.41 m sole at scale 1.15)
  // behind the edge; NaN elsewhere. Pure.
  export const FOOT_HALF=.21;
  export function treadCentre(x,z){
    for(const p of PODIUMS){const s=z-p.z1;if(s<=-TREAD||s>=FLIGHT)continue;
      for(const f of p.stairs)if(Math.abs(x-f.x)<=f.width/2)return s<=0?Math.min(z,p.z1-FOOT_HALF):p.z1+(Math.min(STEPS-1,Math.floor(s/TREAD))+.5)*TREAD;}
    return NaN;
  }
  ```
- [ ] **Step 3. Footing** (`sysworld-footing.js`). Replace the whole `adaptFeet` function, and its comment, with:
  ```js
  // Runs after the mixer has posed the clip and the body has its final position for this step.
  // Lowers the body until the lowest stance foot reaches the ground, then bends each knee so no
  // sole stays below the ground. Writes root.position.y and the leg/shin/foot quaternions only.
  // Stairs (phase 6): with `snap(x,z)` (the z of the tread centre under a point, NaN off the flights) a stance foot whose
  // heel and toe stand on different heights moves along z onto that centre (at most SNAP_REACH), is planted on the height
  // under its centre, and may then also be lowered.
  export const SNAP_REACH=.25;
  export function adaptFeet(rig,root,ground,{maxDrop=.25,maxRaise=.35,snap=null}={}){
    root.updateMatrixWorld(true);
    const feet=rig.legs.map(l=>{
      const lift=soleLift(l.foot,root);let shift=0,snapped=false,pen=0;   // soleLift leaves _a/_b at this foot's contacts
      if(snap&&Math.abs(ground(_a.x,_a.z)-ground(_b.x,_b.z))>.01){
        const cx=(_a.x+_b.x)/2,cz=(_a.z+_b.z)/2,c=snap(cx,cz),w=stanceWeight(lift);
        if(Number.isFinite(c)&&Math.abs(c-cz)<=SNAP_REACH&&w>0){shift=(c-cz)*w;snapped=true;pen=ground(cx,cz+shift)-Math.min(_a.y,_b.y);}
      }
      if(!snapped)pen=penetration(l.foot,ground);
      return {l,lift,pen,shift,snapped};
    });
    let drop=0;for(const f of feet)drop=Math.max(drop,stanceWeight(f.lift)*Math.max(0,-f.pen));
    drop=Math.min(drop,maxDrop);if(drop<1e-4)drop=0;
    if(drop){root.position.y-=drop;root.updateMatrixWorld(true);}
    const raised=[];
    for(const f of feet){
      const raise=f.snapped?Math.max(-maxDrop,Math.min(maxRaise,f.pen+drop)):Math.min(maxRaise,f.pen+drop);raised.push(Math.max(0,raise));
      if(f.snapped?Math.abs(raise)<=5e-4&&Math.abs(f.shift)<=5e-4:raise<=5e-4)continue;
      const {leg,shin,foot}=f.l;
      leg.getWorldPosition(_hip);shin.getWorldPosition(_knee);foot.getWorldPosition(_ankle);
      _target.copy(_ankle);_target.y+=raise;_target.z+=f.shift;_hinge.setFromMatrixColumn(leg.matrixWorld,0);
      const s=solveLeg(_hip,_knee,_ankle,_target,_hinge);if(!s)continue;
      foot.getWorldQuaternion(_f);
      rotateWorld(shin,s.knee);rotateWorld(leg,s.hip);leg.updateMatrixWorld(true);
      shin.getWorldQuaternion(_q);foot.quaternion.copy(_q.invert().multiply(_f));foot.updateMatrixWorld(true);
    }
    return {drop,raised};
  }
  // Measurement for tests and the survey: per leg, the lower sole contact's height above the ground under the sole centre
  // (matrices must be current).
  export function soleGaps(rig,ground,out=[]){
    for(let i=0;i<rig.legs.length;i++){contacts(rig.legs[i].foot);out[i]=Math.min(_a.y,_b.y)-ground((_a.x+_b.x)/2,(_a.z+_b.z)/2);}
    out.length=rig.legs.length;return out;
  }
  ```
  Keep the existing `contacts`, `soleLift` and `penetration` helpers. Without `snap` the behaviour is unchanged: the non-snapped path is the old code.
- [ ] **Step 4. Society** (`sysworld-society.js`).
  - Import `import {lobbyGround} from './sysworld-lobbies.js';` → `import {lobbyGround,treadCentre} from './sysworld-lobbies.js';`
  - Below the imports add `const STAIRS={snap:treadCentre};   // feet on the flights (phase 6); presentation only`.
  - In `settle`, change `if(feet&&a.rig)adaptFeet(a.rig,a.node,floor);` to `if(feet&&a.rig)adaptFeet(a.rig,a.node,floor,STAIRS);`.
- [ ] **Step 5. Living JSON proof.**
  - `node scripts/test-system-world-living.mjs | Select-Object -Last 1 | Set-Content -Encoding utf8 <scratch>/living-t7.json`
  - Compare it with `reports/system-world-survey/phase-6-t0/living.json`: `Compare-Object (Get-Content <scratch>/living-t7.json) (Get-Content reports/system-world-survey/phase-6-t0/living.json)` must print nothing.
  - If it differs, the change has leaked into decisions. Stop and report the diff; do not tune.

**Run:**
```
node scripts/test-system-world-lobbies.mjs
node scripts/test-system-world-footing.mjs
node scripts/test-system-world-lobby-walk.mjs
node scripts/test-system-world-living.mjs
node scripts/system-world-survey.mjs --tier high --out reports/system-world-survey/phase-6-t7 --only lobby-agent-step-1,lobby-agent-step-2,lobby-agent-step-3,lobby-agent-step-4 --strict
```

**Acceptance:**
- The tests pass and the living JSON is identical.
- The four step scenes pass `feetMax 0.02`. If a step scene fails because the pose puts a foot in swing (Could-not-verify 7), report the gaps. The coordinator decides on another stair spot; the limit stays.

**Commit:** `fix(system-world): residents' feet snap to tread centres on stair flights (soleGaps, treadCentre)`

---

## Task 8 — Night light fixes by cause (W1; conditional)

**Files** (only those whose step applies): `sysworld-scene.js`, `sysworld-weather.js`, `sysworld-surfaces.js`, `sysworld-lobby-render.js`, `sysworld-hologram.js`, and their tests (`test-system-world-surfaces.mjs`, `test-system-world-lobby-render.mjs`).

Each step runs only if Task 4's table names its class. Every constant changed here is a value pin, not a test limit. Update any test that pins the old value and give the reason.

- [ ] **Step E (environment at night).** In `sysworld-scene.js`:
  - Add the module constant `const NIGHT_ENV = .2;   // scene.environmentIntensity at night (day .48); range .1-.3`.
  - In `update()`, after the `bloom.strength = … renderer.toneMappingExposure = …;` line, add `scene.environmentIntensity = NIGHT_ENV + (.48 - NIGHT_ENV) * mood.day;`.
- [ ] **Step M (moonlight).** In `sysworld-weather.js`, `sun.intensity=.8+d*2.6;` → `sun.intensity=.45+d*2.95;` (day stays 3.4; night range .3–.8).
- [ ] **Step F (pane lighting on ivory/warm fixtures).**
  - In `sysworld-surfaces.js` `setLighting`, the fourth component `.55 + .45 * night` → `.55 + .2 * night` (range: night term .0–.45).
  - If the panes (second component) explain the patch, cap it: `Math.min(.9, .22 + night * .98 + Math.min(1, evening) * .1)` (cap range .7–1.3).
  - Add a surfaces test line asserting the night values.
- [ ] **Step L (interior fixtures at night).** In `sysworld-lobby-render.js`, `NIGHT={glow:.28,opacity:.26}` → `NIGHT={glow:.15,opacity:.26}` (range .1–.28). Update the lobby-render test pin if one exists.
- [ ] **Step A2 (after C1 adds a dimmed palette key).** Add its `city.<key>` name:
  - to `WINDOWS` in `sysworld-surfaces.js`, if it is an exterior light line like ivory/warm;
  - to `FIXTURES` in `sysworld-lobby-render.js`, if it is an interior fixture.

  Extend the surfaces coverage test so the new name is mapped.
- [ ] **Step C (a real light near a pale wall).** Reduce intensity or distance of the light Task 4 named:
  - hologram `new THREE.PointLight(0x62d8ff, 26, 46, 2)`: intensity range 10–26, distance 25–46;
  - `reactorLight` idle `12`: range 4–12.
- [ ] **Step G (bloom; last resort, only with the coordinator's agreement).** `bloom.threshold = 1.06 + .26 * …` → `bloom.threshold = 1.15 + .17 * …` (day stays 1.32; night range 1.06–1.25).

**Run** (after the applied steps):
```
node scripts/test-system-world-surfaces.mjs
node scripts/test-system-world-lobby-render.mjs
node scripts/test-system-world-windows.mjs
node scripts/system-world-survey.mjs --tier high --out reports/system-world-survey/phase-6-t8 --strict --only facade-missions-night,facade-agent-night,facade-memory-night,life-street-trees-night,skyline-night
node scripts/system-world-survey.mjs --tier high --out reports/system-world-survey/phase-6-t8-lobby --only lobby-memory-night,lobby-missions-night,lobby-agent-night --timeout 900
```
Run only the test files that exist.

**Acceptance:**
- The strict run passes: every night scene's glare ≤ .005.
- The coordinator judges the lobby night pictures for light bars, lintel and jambs.
- The tuned values and their ranges are listed in the hand-back.

**Commit:** `fix(system-world): night glare by cause (<classes applied>)`

### Coordinator task C1 — Emission in the kit (W1 class A, only if an exterior part explains a patch)

- **What changes.** Task 4's emissive inventory names the material, for example `city.cyan` at 1.28 > 1.06. Choose one of:
  1. **Palette.** Lower `PALETTE['<key>']`'s emission strength in `assets/system-world/kit3.py`:
     - cyan 1.6 → 1.2 (range 1.0–1.6);
     - warm 1.3 → 1.0 (.8–1.3);
     - ivory .8 → .65 (.5–.8).

     This affects every design that uses the key; check neon, signals and lobby fixtures in the survey.
  2. **Per part.** Add a dimmed palette entry (for example `'warm-dim': ((.65, .27, .06, 1), .1, .32, .7, 1)`). Assign it to the offending parts only, in the factory that authors them: the mission terminal's central lintel and the agent door jamb strips (in `district_assets3.py` or `facade_assets3.py`, by Task 4's node names). Then Task 8 step A2 maps the new name.
- **Runtime needs.**
  - Material names stay `city.<key>`, and node names are unchanged.
  - `lobby`/`door_<n>_<l|r>` nodes and every lobby contract number are unchanged.
- **Build** (alone; about 15 minutes; share the rebuild with C2 if both are due):
  ```
  & 'D:/Blender 5.2/blender.exe' --background --factory-startup --python assets/system-world/build_world3.py
  node scripts/build-system-world-v3.mjs
  node scripts/build-system-world-v3.mjs --check
  node scripts/test-system-world-v3.mjs
  node scripts/test-system-world-lobby-contract.mjs
  node scripts/test-system-world-surfaces.mjs
  node scripts/test-system-world-windows.mjs
  python assets/system-world/check_assets.py
  ```
- **Commit:** `fix(system-world): dimmer kit emission on <parts> (night glare, payload)`

---

## Task 9 — Level the view when walking and entering a lobby (W3)

**Files:** `ui/js/desktop/apps/sysworld-scene.js`, `ui/js/desktop/apps/sysworld-experience.js`, new `scripts/test-system-world-view.mjs`, `ui/desktop_sysworld_expansion_browser_test.go`.

**Steps**
- [ ] **Step 1. Test first** (complete file `scripts/test-system-world-view.mjs`):
  ```js
  // System World view levelling: the pitch decays with PITCH_TAU, frame-rate independent, unchanged without time; four seconds
  // of walking from the district focus pitch bring the horizon into the middle third of a 43 degree view.
  import assert from 'node:assert/strict';
  import {easePitch,PITCH_TAU,STEER_HOLD} from '../ui/js/desktop/apps/sysworld-scene.js';
  assert.ok(Math.abs(easePitch(.6,PITCH_TAU)-.6/Math.E)<1e-12,'one time constant');
  let a=.5;for(let i=0;i<60;i++)a=easePitch(a,1/60);assert.ok(Math.abs(a-easePitch(.5,1))<1e-12,'frame-rate independent');
  assert.equal(easePitch(.4,0),.4);assert.equal(easePitch(.4,-1),.4);assert.ok(Number.isNaN(easePitch(NaN,.1)));
  const start=Math.atan2(87*.4-2.4,20),limit=Math.atan(Math.tan(43*Math.PI/360)/3);
  assert.ok(Math.abs(easePitch(start,4))<limit,'horizon in the middle third after 4 s: '+easePitch(start,4).toFixed(3));
  assert.ok(PITCH_TAU>=1&&PITCH_TAU<=2.5&&STEER_HOLD>=.5&&STEER_HOLD<=1.5,'constants in their ranges');
  console.log(`System World view: pitch time constant ${PITCH_TAU} s, steering hold ${STEER_HOLD} s; ${start.toFixed(2)} rad levels to ${easePitch(start,4).toFixed(3)} in 4 s.`);
  ```
- [ ] **Step 2. Scene.**
  - After `export function kitFor(id, v3Catalog) { … }` add:
    ```js
    // Walking levels the view (phase 6): the pitch eases towards level with this time constant unless the user steered it
    // within STEER_HOLD seconds; entering a lobby levels it at once.
    export const PITCH_TAU = 1.5, STEER_HOLD = .8;
    export function easePitch(pitch, dt, tau = PITCH_TAU) { return dt > 0 && tau > 0 && Number.isFinite(pitch) ? pitch * Math.exp(-dt / tau) : pitch; }
    ```
  - After `const euler = new THREE.Euler(0, 0, 0, 'YXZ');` add:
    ```js
    let steeredAt = -Infinity, lastLobby = null;
    function levelView(dt) { euler.setFromQuaternion(camera.quaternion); euler.x = dt === Infinity ? 0 : easePitch(euler.x, dt); camera.quaternion.setFromEuler(euler); }
    ```
  - In `look(dx, dy)`, make the first statement `steeredAt = performance.now();`.
  - In `move(dt)`, after `if (allowed(camera.position.x, camera.position.z + dz)) camera.position.z += dz;` (inside the `if (forward || right) {` block) add `if (performance.now() - steeredAt > STEER_HOLD * 1000) levelView(reduced ? Infinity : dt);`.
  - In `update()`, after `if (lobbyRender.update(camera.position, landmarks)) renderer.shadowMap.needsUpdate = true;` add:
    ```js
    // Entering a lobby on foot levels the view at once: the hall, not its ceiling (phase 6).
    const hall = mode === 'street' && !experience.isRiding() ? experience.lobby() : null;
    if (hall && hall !== lastLobby) levelView(Infinity);
    lastLobby = hall;
    ```
  - In `stats()`, after `position:camera.position.toArray(),` add `pitch:+Math.asin(THREE.MathUtils.clamp(camera.getWorldDirection(new THREE.Vector3()).y,-1,1)).toFixed(3), fov:camera.fov,`.
- [ ] **Step 3. Experience.** `isRiding:()=>!!ride,interaction:()=>near,` → `isRiding:()=>!!ride,interaction:()=>near,lobby:()=>lobby,`.
- [ ] **Step 4. Go expansion test** (`verifySystemWorldExpansion`).
  - Replace
    ```go
    	if size := page.MustEval(`()=>SysWorldApp.inspect(cityId).loadedBytes`).Int(); size > 20*1024*1024 {
    		t.Fatal("initial load exceeds 20 MiB", size)
    	}
    ```
    with
    ```go
    	size := page.MustEval(`()=>SysWorldApp.inspect(cityId).loadedBytes`).Int()
    	t.Logf("loaded at first display: %d bytes", size)
    	if size > 20*1024*1024 {
    		t.Fatal("initial load exceeds 20 MiB", size)
    	}
    ```
  - Directly before `page.MustScreenshot(filepath.Join(dir, "world2-lobby-agent.png"))` insert:
    ```go
    	page.MustEval(`()=>{const s=SysWorldApp.inspect(cityId),k=Math.tan(Math.abs(s.pitch))/Math.tan(s.fov*Math.PI/360);if(!(k<=1/3))throw Error('The lobby view must look into the hall (horizon in the middle third): '+JSON.stringify({pitch:s.pitch,fov:s.fov}));}`)
    ```
  - Run `gofmt -w ui/desktop_sysworld_expansion_browser_test.go`.

**Tuning:** `PITCH_TAU` 1.0–2.5, `STEER_HOLD` .5–1.5 s.

**Run:**
```
node scripts/test-system-world-view.mjs
node scripts/test-system-world-living.mjs
go test ./ui -run '^TestDesktopSysWorld' -count=1
```
Then the expansion browser run:
```powershell
$env:AURAGO_RUN_BROWSER_SMOKE='1'; $env:AURAGO_SYSTEM_WORLD_MATRIX='1'; $env:AURAGO_SYSTEM_WORLD_EXPANSION='1'; go test ./ui -run '^TestDesktopAuroraBrowser$' -count=1 -timeout 30m
```
The browser run needs the bundle. Build it into a scratch copy only if the test reads `city.esm.js`; otherwise the coordinator runs this check in Task 16. Report which applies.

**Acceptance:**
- The view test passes; the living test passes.
- The horizon assertion holds, and the log shows "loaded at first display".
- The coordinator sees the hall in `reports/aurora/world2-lobby-agent.png`.

**Commit:** `feat(system-world): walking levels the view, entering a lobby looks into the hall; first-display bytes logged`

---

## Task 10 — Furniture near and far sets (W4)

**Files:** new `ui/js/desktop/apps/sysworld-furniture-lod.js`, new `scripts/test-system-world-furniture-lod.mjs`, `ui/js/desktop/apps/sysworld-scene.js`, `scripts/system-world-survey.mjs`.

**Steps**
- [ ] **Step 1. Test first** (complete file `scripts/test-system-world-furniture-lod.mjs`):
  ```js
  // System World furniture near and far sets: hysteresis, the re-sort gate (twice a second, 4 m), instance matrices of both
  // sets, counts and bounding spheres, NEAR flags, dispose; the shipped trees keep their leaf part at the far level.
  import assert from 'node:assert/strict';
  import fs from 'node:fs/promises';
  import * as THREE from 'three';
  import {GLTFLoader} from 'three/addons/loaders/GLTFLoader.js';
  import {MeshoptDecoder} from 'three/addons/libs/meshopt_decoder.module.js';
  import {FURNITURE_LOD,nearFlag,createFurnitureSets} from '../ui/js/desktop/apps/sysworld-furniture-lod.js';

  assert.equal(nearFlag(44.9,false),true);assert.equal(nearFlag(45,false),false);assert.equal(nearFlag(59.9,true),true);assert.equal(nearFlag(60.1,true),false);
  assert.deepEqual({...FURNITURE_LOD},{near:45,far:60,interval:.5,move:4,level:2});
  const template=parts=>{const g=new THREE.Group();parts.forEach(([y,name])=>{const m=new THREE.Mesh(new THREE.BoxGeometry(.5,.5,.5),new THREE.MeshStandardMaterial({name}));m.position.y=y;g.add(m);});return g;};
  const near=new Map([['bench',template([[0,'city.bronze'],[1,'city.leaf']])],['bin',template([[0,'city.iron']])]]);
  const far=new Map([['bench',template([[0,'city.bronze']])],['bin',template([[0,'city.iron']])]]);
  const pieces=[{asset:'bench',x:10,y:0,z:0,angle:Math.PI/2,scale:[1,1,1]},{asset:'bench',x:50,y:0,z:0,angle:0,scale:[1,1,1]},{asset:'bin',x:100,y:.2,z:0,angle:0,scale:[1,1,1]},{asset:'kiosk',x:0,y:0,z:0,angle:0,scale:[1,1,1]}];
  const sets=createFurnitureSets({pieces,near,far,isNear:id=>id==='bin'});
  const of=(asset,kind)=>sets.meshes.filter(m=>m.userData.furniture===kind&&m.userData.asset===asset);
  assert.equal(sets.meshes.length,5,'bench: 2 near parts + 1 far part; bin: 1 + 1; the kiosk has no templates');
  const at=(x,z)=>new THREE.Vector3(x,1.7,z);
  assert.equal(sets.sync(at(0,0),0,true),true,'a forced sort writes');
  assert.deepEqual(of('bench','near').map(m=>m.count),[1,1]);assert.equal(of('bench','far')[0].count,1);assert.equal(of('bin','far')[0].count,1);assert.equal(of('bin','near')[0].count,0);
  assert.ok(of('bin','near')[0].userData.near&&!of('bench','near')[0].userData.near,'NEAR flag per design');
  { const m=new THREE.Matrix4(),p=new THREE.Vector3(),q=new THREE.Quaternion(),s=new THREE.Vector3();of('bench','near')[1].getMatrixAt(0,m);m.decompose(p,q,s);
    assert.ok(p.distanceTo(new THREE.Vector3(10,1,0))<1e-3&&q.angleTo(new THREE.Quaternion().setFromAxisAngle(new THREE.Vector3(0,1,0),Math.PI/2))<1e-3,'near matrix (placement times part)');
    const f=new THREE.Matrix4();of('bench','far')[0].getMatrixAt(0,f);assert.ok(new THREE.Vector3().setFromMatrixPosition(f).distanceTo(new THREE.Vector3(50,0,0))<1e-3,'far matrix of the second bench');
    assert.ok(of('bench','far')[0].boundingSphere.center.distanceTo(new THREE.Vector3(50,0,0))<1,'bounding sphere follows the set'); }
  assert.equal(sets.sync(at(30,0),.3),false,'too soon');
  assert.equal(sets.sync(at(2,0),1),false,'moved less than 4 m');
  assert.equal(sets.sync(at(30,0),1),true,'the second bench is 20 m away now');assert.equal(of('bench','near')[0].count,2);assert.equal(of('bench','far')[0].count,0);
  assert.equal(sets.sync(at(-2,0),2),false,'52 m: the second bench stays near (hysteresis), nothing to rewrite');assert.equal(of('bench','near')[0].count,2);
  assert.equal(sets.sync(at(-12,0),3),true,'62 m: it leaves');assert.equal(of('bench','near')[0].count,1);
  assert.deepEqual(sets.stats(),{designs:2,pieces:3,near:1,far:2,sorts:3,meshes:5});
  { const parent=new THREE.Group(),meshes=[...sets.meshes];meshes.forEach(m=>parent.add(m));let disposed=0;meshes.forEach(m=>m.addEventListener('dispose',()=>disposed++));
    sets.dispose();assert.equal(disposed,5);assert.equal(parent.children.length,0);assert.ok(near.get('bench').children[0].geometry.getAttribute('position').count>0,'templates stay');sets.dispose(); }
  await MeshoptDecoder.ready;
  const loader=new GLTFLoader().setMeshoptDecoder(MeshoptDecoder),kit=JSON.parse(await fs.readFile('ui/3d/system-world/v3/manifest.json','utf8'));
  for(const id of ['tree-round','tree-column'])for(const level of [0,FURNITURE_LOD.level]){
    const lod=kit.assets.find(a=>a.id===id).lods.find(l=>l.level===level),data=await fs.readFile('ui/3d/system-world/v3/'+lod.file);
    const gltf=await loader.parseAsync(data.buffer.slice(data.byteOffset,data.byteOffset+data.byteLength),'');let leaf=0;
    gltf.scene.traverse(n=>{if(n.isMesh&&[].concat(n.material).some(m=>m.name==='city.leaf'))leaf++;});assert.ok(leaf>0,id+' LOD '+level+' has no city.leaf part (no sway)');}
  console.log('System World furniture sets: hysteresis 45/60 m, gate 0.5 s and 4 m, both sets written and bounded, trees keep their leaves at LOD 2.');
  ```
- [ ] **Step 2. Module** (complete file `ui/js/desktop/apps/sysworld-furniture-lod.js`):
  ```js
  // Furniture near and far instance sets (System World beauty pass, phase 6). Each furniture design keeps two instanced
  // sets: pieces within FURNITURE_LOD.near metres of the camera (they leave again beyond .far) draw at the tier's level of
  // detail, the rest at LOD .level. sync() re-sorts at most every .interval seconds and only after the camera moved .move
  // metres; it copies precomputed matrices into fixed-size instance buffers and allocates nothing. A re-sort does not
  // refresh the shadow map: a piece keeps its place and footprint whichever level draws it. Visibility belongs to the
  // scene's near gate (an empty set has count 0). Geometries and materials belong to the scene's model cache; dispose()
  // releases only the instanced meshes.
  import * as THREE from 'three';

  export const FURNITURE_LOD = Object.freeze({ near: 45, far: 60, interval: .5, move: 4, level: 2 });
  export function nearFlag(distance, wasNear, lod = FURNITURE_LOD) { return wasNear ? distance <= lod.far : distance < lod.near; }

  export function createFurnitureSets({ pieces, near, far, isNear = () => false, lod = FURNITURE_LOD }) {
    const groups = [], meshes = [], placement = new THREE.Matrix4(), part = new THREE.Matrix4(), position = new THREE.Vector3(),
      turn = new THREE.Quaternion(), scale = new THREE.Vector3(), up = new THREE.Vector3(0, 1, 0), byAsset = new Map();
    for (const p of pieces) if (near.has(p.asset) && far.has(p.asset)) { if (!byAsset.has(p.asset)) byAsset.set(p.asset, []); byAsset.get(p.asset).push(p); }
    function build(template, list, kind, asset) {
      template.updateMatrixWorld(true);
      const out = [];
      template.traverse(node => {
        if (!node.isMesh) return;
        const mesh = new THREE.InstancedMesh(node.geometry, node.material, list.length), source = new Float32Array(list.length * 16);
        list.forEach((p, i) => {
          placement.compose(position.set(p.x, p.y || 0, p.z), turn.setFromAxisAngle(up, p.angle || 0), scale.fromArray(p.scale || [1, 1, 1]));
          part.multiplyMatrices(placement, node.matrixWorld).toArray(source, i * 16);
        });
        mesh.count = 0; mesh.castShadow = true; mesh.receiveShadow = true;
        Object.assign(mesh.userData, { near: !!isNear(asset), furniture: kind, asset });
        meshes.push(mesh); out.push({ mesh, source });
      });
      return out;
    }
    for (const [asset, list] of byAsset) groups.push({ list, flags: new Uint8Array(list.length), near: build(near.get(asset), list, 'near', asset), far: build(far.get(asset), list, 'far', asset) });
    const total = groups.reduce((n, g) => n + g.list.length, 0);
    let lastTime = -Infinity, lastX = NaN, lastZ = NaN, sorts = 0, nearCount = 0;
    function write(entries, list, flags, want) {
      for (const { mesh, source } of entries) {
        const target = mesh.instanceMatrix.array; let k = 0;
        for (let i = 0; i < list.length; i++) if (flags[i] === want) { for (let j = 0; j < 16; j++) target[k * 16 + j] = source[i * 16 + j]; k++; }
        mesh.count = k; mesh.instanceMatrix.needsUpdate = true;
        if (k) mesh.computeBoundingSphere();
      }
    }
    function sync(camera, time, force = false) {
      if (!force && (time - lastTime < lod.interval || Math.hypot(camera.x - lastX, camera.z - lastZ) < lod.move)) return false;
      lastTime = time; lastX = camera.x; lastZ = camera.z;
      let changed = force; nearCount = 0;
      for (const g of groups) for (let i = 0; i < g.list.length; i++) {
        const p = g.list[i], flag = nearFlag(Math.hypot(p.x - camera.x, p.z - camera.z), g.flags[i] === 1, lod) ? 1 : 0;
        if (flag !== g.flags[i]) { g.flags[i] = flag; changed = true; }
        nearCount += flag;
      }
      if (!changed) return false;
      for (const g of groups) { write(g.near, g.list, g.flags, 1); write(g.far, g.list, g.flags, 0); }
      sorts++; return true;
    }
    return {
      meshes, sync,
      stats: () => ({ designs: groups.length, pieces: total, near: nearCount, far: total - nearCount, sorts, meshes: meshes.length }),
      dispose() { for (const m of meshes) { m.removeFromParent(); m.dispose(); } meshes.length = 0; groups.length = 0; },
    };
  }
  ```
- [ ] **Step 3. Scene.**
  - **Imports.** Add `import { createFurnitureSets, FURNITURE_LOD } from './sysworld-furniture-lod.js';`.
  - **Furniture design ids.** After `export const kitPlacements = [...]`, add `const FURNISHED = new Set(furnishing().map(f => f.asset).filter(id => id !== 'banner-mast'));   // phase 6: designs with near and far sets`.
  - **Declaration.** `let moodKey = '', nearHidden = null, soundFrame = null;` → `let moodKey = '', nearHidden = null, soundFrame = null, furnitureSets = null;`.
  - **`rebuild()`, expected bytes.** Append to the `expectedBytes = …` expression, after `+ kitIds.reduce((sum, id) => sum + pending(id, lod, 'v3'), 0)`:
    `+ (lod === FURNITURE_LOD.level ? 0 : kitIds.filter(id => FURNISHED.has(id)).reduce((sum, id) => sum + pending(id, FURNITURE_LOD.level, 'v3'), 0))`.
  - **`rebuild()`, far templates.** After `const kit = new Map(await Promise.all(kitIds.map(async id => [id, await model(id, lod, 'v3')])));` add:
    ```js
      const farKit = new Map(await Promise.all((lod === FURNITURE_LOD.level ? [] : kitIds.filter(id => FURNISHED.has(id))).map(async id => [id, await model(id, FURNITURE_LOD.level, 'v3')])));
    ```
  - **`rebuild()`, static batch.** `for (const p of kitPlacements) if (kit.has(p.asset)) add(p, kit.get(p.asset));` → `for (const p of kitPlacements) if (kit.has(p.asset) && !farKit.has(p.asset)) add(p, kit.get(p.asset));`.
  - **`rebuild()`, the sets.** After the loop `for (const { node, matrices, near } of batches.values()) { … }` add:
    ```js
      // Furniture (phase 6): near and far sets per design instead of one city-wide batch (sysworld-furniture-lod.js).
      furnitureSets = farKit.size ? createFurnitureSets({ pieces: kitPlacements.filter(p => farKit.has(p.asset)), near: kit, far: farKit, isNear: id => NEAR.has(id) }) : null;
      if (furnitureSets) { furnitureSets.meshes.forEach(m => staticCity.add(m)); furnitureSets.sync(camera.position, 0, true); }
    ```
    `clearGroup(staticCity)` at the start of the next rebuild disposes the old sets' meshes.
  - **`update()`.** After the near-gate line `if (far !== nearHidden) { … }` add `furnitureSets?.sync(camera.position, elapsed);`.
  - **`stats()`.** After `nearHidden,` add `furnitureLod:furnitureSets?furnitureSets.stats():null,`.
- [ ] **Step 4. Survey.**
  - In `EXPOSE`, `'staticCity, nearHeight: NEAR_HEIGHT, harbour, gulls, particles, '` → `'staticCity, nearHeight: NEAR_HEIGHT, harbour, gulls, particles, furniture: () => furnitureSets, '`. Add `furniture()` to the header comment's `__sw` list.
  - In `captureFrame`, before `syncNear();`, add `const sets = sw.furniture && sw.furniture(); if (sets && !scene.sequenceOf) sets.sync(sw.camera.position, Infinity, true);`.
- [ ] **Step 5. Measure.** Quiet machine:
  ```powershell
  node scripts/system-world-survey.mjs --tier high --out reports/system-world-survey/phase-6-t10 --only overview-day,street-agent,life-park-day,facade-agent-night,life-street-trees-day
  foreach ($t in 'low','medium','high') { node scripts/system-world-survey.mjs --tier $t --timing --only overview-day,street-agent,life-park-day,facade-agent-night --out reports/system-world-survey/phase-6-t10-timing-$t --timeout 900 }
  node scripts/system-world-picture-checks.mjs budget-check --budget scripts/system-world-budget.json --reports reports/system-world-survey/phase-6-t10-timing-low,reports/system-world-survey/phase-6-t10-timing-medium,reports/system-world-survey/phase-6-t10-timing-high
  ```

**Acceptance:**
- The test passes; `node scripts/test-system-world-street-models.mjs` and `node scripts/test-system-world-living.mjs` pass.
- `street-agent` `viewTriangles` (high) < 650,000, and the budget check passes. If a median exceeds its limit by 5 % or less, re-run that tier once and report both runs.
- If the triangle target is missed, report the figure and the share the furniture still draws. The coordinator decides; this task changes nothing else.
- Report a before/after table: triangles and calls from `phase-6-before` against `phase-6-t10`.

**Commit:** `perf(system-world): furniture near and far instance sets (LOD 2 beyond 45 m, re-sorted at most twice a second)`

---

## Task 11 — Sequence mode and the motion sequences (W5)

**Files:** `scripts/system-world-survey.mjs`, `scripts/system-world-survey.scenes.json`.

**Steps**
- [ ] **`PAGE_JS`.** Add `runSequence` after `captureFrame`:
  ```js
  // Sequence mode (phase 6): `sequenceFrames` pictures with `stepsPerFrame` fixed simulation steps of `stepDt` seconds between
  // them; the animation loop is held. Optional: `waitUntil` (expression; 0.1 s steps until true, at most `waitSteps`),
  // `aimAt` (traffic body id: the target is fixed on it after the wait), `path` (keyframes [x,y,z,tx,ty,tz], linear in
  // time; relative to `anchor` {asset,index} when given), `mode: "street"` (the eye spring owns y), `settleMs` per frame.
  async function runSequence(scene) {
    const n = scene.sequenceFrames, steps = scene.stepsPerFrame || 1, dt = scene.stepDt, total = Math.max(1, (n - 1) * steps), street = scene.mode === 'street';
    let path = scene.path || null;
    if (path && scene.anchor) { const p = module.kitPlacements.filter(q => q.asset === scene.anchor.asset)[scene.anchor.index || 0]; if (!p) throw Error('no anchor ' + scene.anchor.asset);
      path = path.map(k => [k[0] + p.x, k[1] + p.y, k[2] + p.z, k[3] + p.x, k[4] + p.y, k[5] + p.z]); }
    frozen = true;
    if (street) { city.setMode('street'); sw.cancelFlight(); }
    const place = k => {
      if (!path) return;
      const t = Math.min(1, k / total) * (path.length - 1), i = Math.min(path.length - 2, Math.floor(t)), f = t - i, a = path[i], b = path[i + 1], at = j => a[j] + (b[j] - a[j]) * f;
      sw.camera.position.set(at(0), street && k > 0 ? sw.camera.position.y : at(1), at(2));
      if (street) sw.camera.lookAt(at(3), at(4), at(5)); else { sw.controls.target.set(at(3), at(4), at(5)); sw.controls.update(); }
      const body = sw.traffic.body('visitor');
      if (body && (!street || k === 0)) { body.x = sw.camera.position.x; body.y = sw.camera.position.y; body.z = sw.camera.position.z; }
    };
    place(0);
    if (scene.waitUntil) {
      const until = new AsyncFunction('city', '__sw', 'return (' + scene.waitUntil + ');'), limit = scene.waitSteps || 1200; let k = 0;
      for (; k < limit && !(await until(city, sw)); k++) { elapsed += .1; city.update(.1, elapsed); syncNear(); }
      if (k >= limit) throw Error('waitUntil never held: ' + scene.waitUntil);
    }
    if (scene.aimAt) { const b = sw.traffic.body(scene.aimAt); if (!b) throw Error('no body ' + scene.aimAt); sw.controls.target.set(b.x, b.y + 1.5, b.z); sw.controls.update(); }
    for (let f = 0, k = 0; f < n; f++) {
      if (f > 0) for (let s = 0; s < steps; s++) { k++; place(k); elapsed += dt; city.update(dt, elapsed); syncNear(); }
      if (scene.settleMs) await sleep(scene.settleMs);
      await captureFrame(Object.assign({}, scene, { position: undefined, followResident: undefined, masks: undefined, sequenceOf: scene.name, checks: f === 0 ? scene.checks : null }), scene.name + '-' + String(f).padStart(2, '0'), null);
    }
    frozen = false;
    if (street) city.setMode('orbit');
  }
  ```
  - In `captureFrame`, after `shots.push(shot);`, insert before it: `if (scene.sequenceOf) shot.sequence = scene.sequenceOf;`.
  - In the scene loop, replace `const count = scene.sequence > 0 ? scene.sequence : 1;` and the `for` loop that follows with:
    ```js
      if (scene.stepDt) await runSequence(scene);
      else {
        const count = scene.sequence > 0 ? scene.sequence : 1;
        for (let i = 0; i < count; i++) {
          if (i > 0) await sleep(scene.intervalMs === undefined ? 350 : scene.intervalMs);
          wanted = await captureFrame(scene, count > 1 ? scene.name + '-' + String(i).padStart(2, '0') : scene.name, wanted);
        }
      }
    ```
- [ ] **Node side.** Import `writeSheet`. In `main()`, after `failures = await measure(…)`, add:
  ```js
      for (const name of new Set(report.shots.filter(s => s.sequence).map(s => s.sequence))) console.log('sheet ' + writeSheet(outDir, name));
  ```
- [ ] **Scenes.** Append these in the file's one-line style; `<wait-agent>` is defined below.
  ```json
  {"name":"seq-door-visitor","position":[0,3.3,0.6],"target":[0,3.3,-8],"fov":55,"environment":"day","weather":"clear","frames":40,"waitMs":400,"eval":"<wait-agent>","mode":"street","stepDt":0.0166667,"stepsPerFrame":15,"sequenceFrames":16,"path":[[0,0,0.6,0,3.2,-8],[0,0,-6.6,0,3.2,-14]]},
  {"name":"seq-door-resident","position":[3.5,2.6,-1.2],"target":[0,1.6,-6],"fov":50,"environment":"day","weather":"clear","frames":40,"waitMs":400,"eval":"<wait-agent>if(!low){const a=__sw.experience.society.actors.find(r=>r.id==='resident-0');if(!__sw.traffic.relocate(a.body,{x:0,y:.88,z:-2,heading:Math.PI},.5))throw Error('door spot blocked');a.slot=null;a.state='walk';a.path=[{x:0,y:.9,z:-8}];a.until=0;a.node.position.set(a.body.x,a.body.y,a.body.z);}","stepDt":0.0166667,"stepsPerFrame":15,"sequenceFrames":16,"path":[[3.5,2.6,-1.2,0,1.6,-6],[3.5,2.6,-1.2,0,1.6,-6]]},
  {"name":"seq-swap-45","position":[0,4,50],"target":[0,4,-12],"fov":43,"environment":"day","weather":"clear","frames":40,"waitMs":400,"stepDt":0.1,"stepsPerFrame":3,"sequenceFrames":12,"settleMs":600,"path":[[0,4,50,0,4,-12],[0,4,22,0,4,-12]]},
  {"name":"seq-gust","position":[-60,4,-32],"target":[-22,2.5,-32],"fov":43,"environment":"day","weather":"clear","frames":40,"waitMs":400,"waitUntil":"__sw.atmosphere.wind().strength>0.62","stepDt":0.0333333,"stepsPerFrame":6,"sequenceFrames":12,"path":[[-60,4,-32,-22,2.5,-32],[-60,4,-32,-22,2.5,-32]]},
  {"name":"seq-banners","position":[0,10,0],"target":[0,5,0],"fov":50,"environment":"day","weather":"clear","frames":40,"waitMs":400,"anchor":{"asset":"banner-mast","index":0},"waitUntil":"__sw.atmosphere.wind().strength>0.62","stepDt":0.0333333,"stepsPerFrame":6,"sequenceFrames":12,"path":[[7,4,7,0,5,0],[7,4,7,0,5,0]]},
  {"name":"seq-launch","position":[-22,7,85],"target":[-45,-2,100],"fov":43,"environment":"day","weather":"clear","frames":40,"waitMs":400,"stepDt":0.1,"stepsPerFrame":5,"sequenceFrames":16,"path":[[-22,7,85,-45,-2,100],[-22,7,85,-45,-2,100]]},
  {"name":"seq-gulls","position":[-34,5,82],"target":[-6,19,104],"fov":55,"environment":"day","weather":"clear","frames":40,"waitMs":400,"stepDt":0.0333333,"stepsPerFrame":6,"sequenceFrames":12,"path":[[-34,5,82,-6,19,104],[-34,5,82,-6,19,104]]},
  {"name":"seq-tram","position":[-54,3,-64],"target":[-62,1.5,-72],"fov":50,"environment":"day","weather":"clear","frames":40,"waitMs":400,"waitUntil":"Math.hypot(__sw.traffic.body('tram-0').x-__sw.camera.position.x,__sw.traffic.body('tram-0').z-__sw.camera.position.z)<40","aimAt":"tram-0","stepDt":0.1,"stepsPerFrame":3,"sequenceFrames":12},
  {"name":"seq-walk-80","position":[-60,1.9,-31],"target":[-40,1.9,-31],"fov":43,"environment":"day","weather":"clear","frames":40,"waitMs":400,"stepDt":0.1,"stepsPerFrame":29,"sequenceFrames":21,"path":[[-60,1.9,-31,-40,1.9,-31],[20,1.9,-31,40,1.9,-31]]}
  ```
  - `<wait-agent>` is the first part of the `lobby-agent-day` eval: from `const low=…` through the `throw Error('interior not shown: lobby-agent');` statement. In `seq-door-visitor` it ends there; in `seq-door-resident` it is followed by the resident lines shown.
  - Before committing, set `seq-walk-80`'s z to the street line nearest −31 from `streets.zs` in `sysworld-layout.js`, and the `seq-tram` camera to a corner beside the tram route. Record both adjustments.
- [ ] **Run:**
  ```
  node scripts/system-world-survey.mjs --tier high --out reports/system-world-survey/phase-6-sequences --only seq-door-visitor,seq-door-resident,seq-swap-45,seq-gust,seq-banners,seq-launch,seq-gulls,seq-tram,seq-walk-80 --timeout 2400
  ```
  Then read every `*-sheet.png`.

**Acceptance:**
- 9 sheets and `page errors: 0`.
- No sequence's first frame fails the `camera` check.
- The coordinator judges each sheet:
  - doors open for the visitor and for the resident;
  - the stand-in swaps to the interior;
  - leaves sway and banners swing in the gust;
  - the launch moves;
  - gulls flap;
  - the tram passes;
  - the walk shows no visible popping.

  If a camera misses its motion, adjust its numbers and report them.

**Commit:** `feat(system-world): survey sequence mode with contact sheets; door, swap, gust, banners, launch, gulls, tram and walk sequences`

---

## Task 12 — Patrol label and visitor placement (W6)

**Files:** `ui/js/desktop/apps/sysworld-society.js` (patrol label only), `ui/js/desktop/apps/sysworld-experience.js` (`endRide`), new `scripts/test-system-world-patrol-label.mjs`, new `scripts/test-system-world-visitor-placement.mjs`.

**Steps**
- [ ] **Step 1. Diagnostic** (scratch only, never committed).
  - In a scratch copy of `scripts/test-system-world-living.mjs`, log each patrol's `state`, `trafficYield`, `yieldTram`, `held` (`life.stats().navigation.held`) and distance moved every 0.5 s.
  - Classify `yield` label time as stepping aside (the tram branch), holding for the train (the `yieldTram` branch), or stale (neither applies, so the patrol is moving under its own drive).
  - Report seconds per class.
- [ ] **Step 2. Test first** (`scripts/test-system-world-patrol-label.mjs`):
  - Build a society as `scripts/test-system-world-traffic.mjs` does (`createSociety(scene,{traffic,camera,floor:()=>0,active:()=>true})`).
  - Register a patrol body `patrol-0` with `bodyShape({min:[-.3,0,-.3],max:[.3,1.6,.3]},1.15)` at (0, 0, 0), and add it with `society.patrol('patrol-0', body, new THREE.Group(), (held)=>{})`.
  - Register a tram body `tram-0` 3 m beside it, heading 0. Run `society.update(1/60,true)` a few times and assert that the patrol's state in `stats().residents` is `yield`.
  - Call `society.suspend()` and assert that its state is `patrol` (this fails before the fix).
  - Remove the tram (`traffic.remove('tram-0')`), run updates for 2 s, and assert `patrol`.
- [ ] **Step 3. Fix.**
  - In `suspend()`, `if(!a.pause){release(a);a.state='idle';a.until=clock+1;}` → `if(!a.pause){release(a);a.state='idle';a.until=clock+1;}else if(a.state==='yield')a.state='patrol';`.
  - In `update()`, `if(a.pause){if(a.state==='greet'&&clock>a.until){a.pause(false);a.state='patrol';}continue;}` → `if(a.pause){if(a.state==='yield'&&!a.trafficYield&&!a.yieldTram)a.state='patrol';if(a.state==='greet'&&clock>a.until){a.pause(false);a.state='patrol';}continue;}`.
  - If the diagnostic finds stale time in any other path, apply the same rule there (a patrol shows `yield` only while `a.trafficYield||a.yieldTram`) and describe it in the hand-back. If all label time was genuine, say so: item 26 closes with that note and item 14 stays in the backlog.
- [ ] **Step 4. Visitor placement test** (`scripts/test-system-world-visitor-placement.mjs`).
  - Copy the setup of `scripts/test-system-world-lobby-walk.mjs`: its lines 1–27 (globals, experience, `tick`, `put`). Import `dronePad` from `sysworld-exploration.js`.
  - Wait for `ready`, `put(dronePad.x,dronePad.z+3)`, run `tick()` until `experience.interaction()?.kind==='drone'` (at most 120 ticks), then `experience.interact()` and assert `experience.stats().ride==='drone'`.
  - Run 30 ticks. Register a blocker with the visitor's shape (`{circles:[{x:0,z:0,r:.38}],minY:-2.3,maxY:.4,reach:.38}`) at the ride's origin, `{x:dronePad.x,y:<the origin y>,z:dronePad.z+3,heading:0}`, priority 1.
  - Call `experience.endRide()`, then `tick()` 5 times.
  - Assert: `camera.position.y<5`; `traffic.clear(visitor,visitor,visitor)`; the camera within 1e-6 of the visitor body; the visitor within 3 m of the origin.
  - Run it before the fix: it should fail. Report what it shows.
- [ ] **Step 5. Fix in `endRide()`.** Insert directly before the function's last statement (`    ride=null;options.onRide?.(null);`, which follows the closing brace of `if(ride.kind==='scope'){…}`):
  ```js
      // A teleport out of a ride seats the visitor body on a free spot (never on a resident; backlog 27).
      if(ride.kind==='tram'||ride.kind==='drone'||ride.kind==='lift'||ride.kind==='scope')seat();
  ```
  Audit the remaining paths in `sysworld-scene.js` and `sysworld-experience.js` with a grep for `camera.position.set(` and `camera.position.copy(`. Paths already covered: `settleCamera` in `focus`, `setMode`, `visit`, `enter`; `findFree` in `flyTo`; `seat` after hover. Report any path not covered.
- [ ] **Step 6. Living JSON proof.** Compare the new last line with `phase-6-t0/living.json` with a scratch node script:
  ```js
  import fs from 'node:fs';const [a,b]=process.argv.slice(2).map(f=>JSON.parse(fs.readFileSync(f,'utf8').replace(/^\uFEFF/,'')));
  for(const j of[a,b])for(const k of Object.keys(j.yieldSeconds))if(k.startsWith('patrol-'))delete j.yieldSeconds[k];
  console.log(JSON.stringify(a)===JSON.stringify(b)?'identical apart from patrol yield seconds':'DIFFERENT');
  ```
  Write it with the Write tool; it contains a `\uFEFF` escape, so check that the file keeps the escape as written. If it prints `DIFFERENT`, stop and report.

**Run:**
```
node scripts/test-system-world-patrol-label.mjs
node scripts/test-system-world-visitor-placement.mjs
node scripts/test-system-world-living.mjs
node scripts/test-system-world-lobby-walk.mjs
node scripts/test-system-world-hover.mjs
node scripts/test-system-world-traffic.mjs
```

**Acceptance:** all pass, the JSON proof holds, and the audit list is in the hand-back.

**Commit:** `fix(system-world): patrol yield label clears when it moves on; rides seat the visitor on a free spot`

### Coordinator task C2 — Pot plants foliage; zebra and kerb check (W6)

- **`interior_assets3.py` `pot_plant(g, x, y, size)`.** Wrap the `blob(...)` calls in a soft part: `prev = g.part; g.part = 'foliage'` before the first `blob`, and `g.part = prev` after the last. The ceramic pot stays in its part.
- **Every lobby factory that places pot plants** (`lobby_agent`, `lobby_memory`, `lobby_integrations`, `lobby_missions`, `lobby_infra`, and `lobby_graph` if it does) adds `'soft': ['foliage']` to its returned dict, next to `'pivots'`.
- **Runtime needs.**
  - A new node `foliage` per interior.
  - Unchanged `lobby`, `door_<n>_<l|r>`, workpoints and colliders.
  - No new material.
  - `sysworld-lobby-render.js` looks up only `lobby` and the door regex, so no code change.
- **Zebra and kerb.** If `crossing` or `kerb-close` fails `glareMax` in the final run, lower `PALETTE['marking']` .5 → .42 (range .38–.5) or `kerb` .30 → .26 (.22–.30).
- **Build:** the C1 commands (one shared rebuild if both are due), plus `node scripts/test-system-world-lobby-render.mjs`.
- **Look:** `lobby-*-inside` pictures, with smooth plants.
- **Commit:** `feat(system-world): interior pot plants get a soft foliage part (payload)`

---

## Task 13 — Tram, footfalls and street presence by synthesis (W7 route b)

**Files:** `ui/js/desktop/apps/sysworld-sound-movers.js`, `ui/js/desktop/apps/sysworld-foley.js`, `ui/js/desktop/apps/sysworld-audio.js` (one argument), `scripts/test-system-world-sound-movers.mjs`, `scripts/test-system-world-foley.mjs`; `scripts/system-world-audio-fake.mjs` only if it must record nodes (the one-fake rule).

**Steps**
- [ ] **Step 1. Tests first.** Add to the movers test:
  - `tramLaw(11,0,100).hum>0` and `tramLaw(11,0,121).hum===0`.
  - A tram voice within reach builds three roll bandpass filters whose frequencies after one update follow `law.roll × [.55, 1.1, 2.05]` within 5 %. Read them through the shared fake; extend the fake there if it does not record nodes.
  - With a seeded `random` and a straight tram at 11 m/s and 20 m for 2 s of 1/60 updates, `stats().ticks` is between 3 and 15; in a curve (`turn` .3) it is higher.
  - A hover glider at 40 m gets a voice (reach 45).

  Add to the foley test:
  - Every surface except `garden` and `grating` has `STEP_VOICES[s].body` with `f` in 100–400 and `q ≥ 6`.
  - `PAIRS.pavement` has exactly one layer with `body:true` and no toe layer with `body`.
- [ ] **Step 2. Movers.**
  - `MOVERS`:
    - add `tick:.25,tickGap:.3,rollBands:Object.freeze([Object.freeze([.55,1.0,.6]),Object.freeze([1.1,1.4,.45]),Object.freeze([2.05,1.8,.3])]),tramFall:Object.freeze([8,.7]),hoverFall:Object.freeze([4,.45]),`;
    - change `reach` to `{tram:120,hover:45,launch:120,barge:260,clack:60}`.
  - `tramLaw`: `distanceGain(distance,6,1)` → `distanceGain(distance,MOVERS.tramFall[0],MOVERS.tramFall[1])`.
  - `createSoundMovers(context,{outdoor,send,noise,tier='high'})` → `…,tier='high',random=Math.random})`.
  - `trams` init: add `tick:0`. Add `let ticks=0;` beside `clacksPlayed`.
  - `kit()`: `last:new Float64Array(6).fill(NaN)` → `new Float64Array(9).fill(NaN)`.
  - `tramVoice()`: replace the line starting `v.rollBand=v.filter('bandpass',600,.55);` with
    ```js
    v.rolls=MOVERS.rollBands.map(([k,q,g],i)=>{const b=v.filter('bandpass',600*k,q);v.noise(.9+.07*i).connect(v.filter('highpass',220,.7)).connect(b).connect(v.amp(MOVERS.roll*g)).connect(v.out);return b;});
    ```
    The comment above it becomes "wheel-on-rail roar: three detuned noise bands that follow the speed".
  - `clack(at,gain,pan)` → `clack(at,gain,pan,f=1800,q=2,decay=.012)`:
    - the band uses `f`/`q`;
    - `exponentialRampToValueAtTime(.0001,at+decay)`;
    - `src.stop(at+decay+.008)`.
  - `moveTram`: replace `set(v,5,v.rollBand.frequency,law.roll,.2,1);` with `for(let i=0;i<v.rolls.length;i++)set(v,5+i,v.rolls[i].frequency,law.roll*MOVERS.rollBands[i][0]*(1+.012*(i-1)),.2,1);`. After the `while(t.travelled>=MOVERS.joint){…}` line add:
    ```js
    // Flange ticks: short bright noise bursts, more often at speed and in curves (deterministic through the injected random).
    if(d<MOVERS.reach.clack&&law.hum>0&&t.speed>1){t.tick-=dt*(t.speed/11)*(1+2*Math.min(1,Math.abs(t.turn)));if(t.tick<=0){ticks++;clack(now,law.hum*MOVERS.tick/MOVERS.clack,v.pan,4200,6,.006);t.tick=MOVERS.tickGap*(.4+1.2*random());}}
    ```
  - `moveHovers`: `distanceGain(d,3,.6)` → `distanceGain(d,MOVERS.hoverFall[0],MOVERS.hoverFall[1])`.
  - `stats`: add `ticks`.
- [ ] **Step 3. Audio.** `movers=createSoundMovers(context,{outdoor:space.outdoor,send:space.sendOutdoor,noise,tier});` → `…,noise,tier,random});`.
- [ ] **Step 4. Foley.**
  - Add `body:{f,q,lvl}` to `STEP_VOICES`:
    - road `{f:150,q:8,lvl:.3}`;
    - pavement `{f:200,q:9,lvl:.32}`;
    - kerb `{f:260,q:9,lvl:.3}`;
    - stairs `{f:170,q:10,lvl:.36}`;
    - quay `{f:130,q:8,lvl:.34}`;
    - lobby `{f:300,q:10,lvl:.28}`;
    - none for garden or grating.
  - `const PAIRS=` → `export const PAIRS=`. Inside its map, `return [name,[...heel,...toe]];` → `return [name,[...heel,...(spec.body?[{wave:'noise',f:spec.body.f,q:spec.body.q,dur:.07,lvl:spec.body.lvl,body:true}]:[]),...toe]];`. The toe list is built from `spec.layers`, so it never copies the body layer.
  - Update the header comment of `PAIR` (the body resonance) and the `STEP_VOICES` comment.
- [ ] **Step 5. Survey and listening.**
  ```
  node scripts/test-system-world-sound-movers.mjs
  node scripts/test-system-world-foley.mjs
  node scripts/test-system-world-soundscape.mjs
  node scripts/system-world-sound-survey.mjs --label phase-6-t13 --baseline baseline --strict --timeout 1800
  ```
  The coordinator runs earshot `find` on `reports/system-world-sound/phase-6-t13/tram-corner.wav` ("a tram passing") and `walk-surfaces.wav` ("footsteps"), and `ab` against `phase-6-t0`.

**Tuning:**
- tram reach 90–150 and fall rolloff .5–1;
- hover reach 30–60, reference 3–6 m, rolloff .3–.6;
- roll band gains ×.5–×1.5;
- tick gain .1–.4, mean gap .15–.6 s;
- body level .15–.45.

Keep the soundscape budget test's node limits; if a limit is hit, drop to two roll bands on low.

**Acceptance:**
- The tests and the four strict survey checks pass.
- The earshot z values are recorded. z ≥ 2.5 is the target; if it is not reached, the record says route (b) reached z = … .

**Commit:** `feat(system-world): tram roar bands and flange ticks, footfall body resonance, trams and hover robots heard further`

---

## Task 14 — Darker leaves loop (W7)

**Files:** `assets/system-world/build_audio.py`, `assets/system-world/audio-sources.json`. The coordinator then rebuilds `ui/3d/system-world/audio/leaves.ogg` and `manifest.json`.

**Steps (agent)**
- [ ] **`build_audio.py`.**
  - `def build(sources_dir):` → `def build(sources_dir, only=None):`.
  - At its start, after `entries = []`, add:
    ```python
    keep = {}
    if only:
        current = json.loads((OUT / 'manifest.json').read_text(encoding='utf-8'))
        keep = {e['id']: e for e in current['recordings']}
    ```
  - First thing inside `for rec in spec['recordings']:`:
    ```python
            if only and rec['id'] not in only:
                if rec['id'] not in keep:
                    sys.exit(f"{rec['id']}: not in the shipped manifest; rebuild it too")
                entries.append(keep[rec['id']])
                continue
    ```
  - Replace `highpass=f={rec['highpass']},asplit=3[a][b][c];` with `highpass=f={rec['highpass']}{lowpass},asplit=3[a][b][c];`, and define `lowpass = f",lowpass=f={rec['lowpass']}" if rec.get('lowpass') else ''` before `graph`.
  - Add the argument `parser.add_argument('--only', help='comma-separated recording ids to rebuild; the others keep their shipped files and manifest entries')`, and change the call to `build(options.sources, set(options.only.split(',')) if options.only else None)`.
  - Update the module docstring (low-pass, `--only`).
- [ ] **`audio-sources.json` leaves entry.** Add `"lowpass": 7000` and `"cut_note": "A darker cut: a low-pass near 7 kHz and a gusty stretch, so the loop reads as wind in leaves rather than rain."`. `start` is the coordinator's choice.
- [ ] **Run:** `python assets/system-world/build_audio.py --check` (still verifies the shipped files) and `node scripts/test-system-world-recordings.mjs`.

**Coordinator.**
- With the confirmed source (Request 6) in its own folder, run earshot `find` on `0904.ogg` for "wind gusting through tree leaves" and pick `start`.
- Then:
  ```
  python assets/system-world/build_audio.py --sources <dir> --only leaves
  python assets/system-world/build_audio.py --check
  node scripts/test-system-world-recordings.mjs
  node scripts/system-world-sound-survey.mjs --label phase-6-t14 --baseline baseline --strict --only park-wind,garden-court --timeout 900
  ```
- Run earshot `describe` on `park-wind.wav`: "rain" must rank below "wind in trees".
- Tuning: low-pass 5500–8500 Hz.

**Acceptance:** the budget holds (`check` total ≤ 1,572,864 bytes), the strict checks pass, and the earshot note is recorded.

**Commit:** `tune(system-world): darker leaves loop (low-pass, gusty stretch); build_audio --only`

---

## Task 15 — Optional: CC0 tram pass-by and footsteps recordings (W7 route a; only after the user's yes)

**Precondition:** the user approved the concrete files (page, author, CC0 licence, size). The coordinator records this in `audio-sources.json` `note`.

**Outline**
1. **Download.** The coordinator downloads each file into its own new empty folder and verifies CC0 on the page.
2. **Sources.** `audio-sources.json` gets two entries in the existing schema:
   - `tram`: a single pass-by, 6–10 s, not looped;
   - `steps`: 20–30 s of steps on a hard surface, from which the runtime takes 140–200 ms slices.

   `build_audio.py` gains a per-recording `"loop": false`: no seam crossfade, keep a 30 ms fade in and out.
3. **Runtime.** `sysworld-sound-recordings.js` loads both after the first frame like the others, and exposes `buffer(id)`.
   - `sysworld-sound-movers.js` `tramVoice` plays the tram buffer once per pass, through the voice's `out` (with distance law and pan), when a tram first comes within 60 m and is moving faster than 4 m/s, at −6 dB under the synthesis (range −12 to 0 dB).
   - `sysworld-foley.js` plays a random 160 ms slice of `steps` (seeded) as an extra layer under each road/pavement/kerb/stairs/lobby footfall, at .4 of the heel level.
   - If a buffer is missing, nothing changes.
4. **Budget and test.** The total stays ≤ 1.5 MiB; `test-system-world-recordings.mjs` covers the two ids. Strict survey; earshot `find` and `realness` on `tram-corner` and `walk-surfaces`.

**Commit:** `feat(system-world): CC0 tram pass-by and footsteps under the synthesis (user-approved), with provenance and budget`

---

## Task 16 — Markers, browser checks, docs, bundle, regression

**Files:** `ui/desktop_sysworld_test.go`, `ui/desktop_sysworld_city_browser_test.go`, `ui/js/desktop/apps/AGENTS.md`, `ui/3d/system-world/AGENTS.md`, `assets/system-world/AGENTS.md`, `assets/system-world/README.md`, `plans/2026-11-09-system-world-beauty-design.md`, `plans/2026-11-10-system-world-beauty-phase-6-backlog.md`, `ui/js/vendor/system-world/*` (generated).

**Steps**
- [ ] **Markers in `TestDesktopSysWorldAppMarkers`.** Use the strings below; if a task wrote different code, use its actual export lines.
  - New entry: `"js/desktop/apps/sysworld-furniture-lod.js": {"export function createFurnitureSets(", "export const FURNITURE_LOD", "export function nearFlag("},`.
  - Append:
    - scene: `"createFurnitureSets(", "export function easePitch(", "aviationPoints(", "experience.lobby()"`;
    - atmosphere: `"export function coneFade(", "export function roofTop(", "export function aviationPoints(", "attribute float fade;"`;
    - windows: `"export const WINDOW_TONES", "vec3 winTone(float h)"`;
    - footing: `"export function soleGaps(", "snap=null"`;
    - lobbies: `"export function treadCentre(", "export const FOOT_HALF"`;
    - society: `"snap:treadCentre"`;
    - experience: `"lobby:()=>lobby"`;
    - sound-movers: `"rollBands", "flange"`;
    - foley: `"export const PAIRS"`.

  Run `gofmt -w ui/desktop_sysworld_test.go`.
- [ ] **City browser test.**
  - After the eval that throws `'City life incomplete: '`, add:
    ```go
    	page.MustEval(`()=>{const s=SysWorldApp.inspect(cityId);if(!s.furnitureLod||s.furnitureLod.designs<10||s.furnitureLod.pieces<280||s.furnitureLod.near>s.furnitureLod.far)throw Error('Furniture near and far sets missing at the overview: '+JSON.stringify(s.furnitureLod));}`)
    ```
  - After the eval that throws `'Low tier must keep the lobby stand-ins: '` (a `AURAGO_SYSTEM_WORLD_MATRIX`-only path), add:
    ```go
    	page.MustEval(`()=>{const s=SysWorldApp.inspect(cityId);if(s.furnitureLod!==null)throw Error('Low tier keeps one furniture batch: '+JSON.stringify(s.furnitureLod));}`)
    ```
- [ ] **Docs** (concise, in the existing style).
  - **`ui/js/desktop/apps/AGENTS.md`, System World section.** Bullets for:
    - `sysworld-furniture-lod.js`;
    - view levelling (`easePitch`, lobby entry);
    - cone fade and roof-top aviation;
    - window tones and the low-tier cell;
    - tread snap (`treadCentre`, `soleGaps`, residents only);
    - visitor seating after rides;
    - tram bands, ticks and footfall body.

    Add the new tests and `node scripts/system-world-picture-checks.mjs …` to the verification list.
  - **`ui/3d/system-world/AGENTS.md`.** Add the ownership rows from this plan's Architecture.
  - **`assets/system-world/AGENTS.md`.**
    - The pot plants now have the soft `foliage` part: replace the "do not yet (backlog item 23)" sentence.
    - Add the new test commands and `build_audio.py --only`.
  - **`assets/system-world/README.md`.** One paragraph on phase 6: night light, windows, entering, furniture LOD and budget, sequences, sound finish, and the picture checks and budget commands.
  - **Design doc.** The phase 6 roadmap row points to `plans/2026-11-10-system-world-beauty-phase-6.md`.
  - **Backlog.** Mark items 1–6, 8–11, 15–19, 21–29, 31 with their outcome or "closed: <reason>". Items 12, 13, 14, 20, 30, 32 and 33 stay open (design "out of scope").
- [ ] **Bundle:** `node scripts/build-system-world.js`, then `node scripts/build-system-world.js --check` → `verified`.
- [ ] **Regression** (PowerShell; all must pass):
  ```powershell
  Get-ChildItem scripts/test-system-world*.mjs | ForEach-Object { node $_.FullName; if ($LASTEXITCODE) { throw $_.Name } }
  python assets/system-world/check_assets.py
  python assets/system-world/build_audio.py --check
  node scripts/build-system-world-v3.mjs --check
  node scripts/build-system-world.js --check
  go test ./ui -run '^TestDesktopSysWorld' -count=1
  $env:AURAGO_RUN_BROWSER_SMOKE='1'; $env:AURAGO_SYSTEM_WORLD_MATRIX='1'; Remove-Item Env:AURAGO_SYSTEM_WORLD_EXPANSION -ErrorAction SilentlyContinue; go test ./ui -run '^TestDesktopAuroraBrowser$' -count=1 -timeout 30m
  $env:AURAGO_SYSTEM_WORLD_EXPANSION='1'; go test ./ui -run '^TestDesktopAuroraBrowser$' -count=1 -timeout 30m
  ```
  The MATRIX-only run executes checks phase 5 never ran. Report any failure there as a finding with its message; the coordinator decides. Never loosen a check.

**Commit:** `chore(system-world): rebuild renderer bundle, pin phase 6 markers and browser checks, docs`

---

## Task 17 — Acceptance record and merge preparation (W8)

**Inputs from the coordinator:** `<baseline>` (for example `baseline`), and `<sound-baseline>` (the label the strict checks use, `baseline` as in phase 5).

**Steps (test agent; coordinator where marked)**
- [ ] **Picture survey on three tiers:**
  ```
  node scripts/system-world-survey.mjs --tier high --out reports/system-world-survey/phase-6-final-high --timeout 2400 --strict
  node scripts/system-world-survey.mjs --tier medium --out reports/system-world-survey/phase-6-final-medium --timeout 2400
  node scripts/system-world-survey.mjs --tier low --out reports/system-world-survey/phase-6-final-low --timeout 2400
  node scripts/system-world-survey.mjs --tier high --out reports/system-world-survey/phase-6-final-sequences --timeout 2400 --only seq-door-visitor,seq-door-resident,seq-swap-45,seq-gust,seq-banners,seq-launch,seq-gulls,seq-tram,seq-walk-80
  node scripts/system-world-picture-checks.mjs lit-compare --a reports/system-world-survey/phase-6-final-high --b reports/system-world-survey/phase-6-final-low --scene facade-agent-night
  node scripts/system-world-picture-checks.mjs compare --before reports/system-world-survey/<baseline> --after reports/system-world-survey/phase-6-final-high --out reports/system-world-survey/phase-6-compare
  ```
- [ ] **Frame times and budget** (quiet machine):
  ```powershell
  foreach ($t in 'low','medium','high') { node scripts/system-world-survey.mjs --tier $t --timing --only overview-day,street-agent,life-park-day,facade-agent-night --out reports/system-world-survey/phase-6-final-timing-$t --timeout 900 }
  node scripts/system-world-picture-checks.mjs budget-check --budget scripts/system-world-budget.json --reports reports/system-world-survey/phase-6-final-timing-low,reports/system-world-survey/phase-6-final-timing-medium,reports/system-world-survey/phase-6-final-timing-high
  ```
- [ ] **Sound survey:** `node scripts/system-world-sound-survey.mjs --label phase-6-final --baseline <sound-baseline> --strict --timeout 1800`.
- [ ] **Asset budgets:**
  - model directory: the `model directory N bytes` line of `node scripts/test-system-world-v3.mjs` (< 40 MiB);
  - first display: the "loaded at first display" log of the expansion run (< 20 MiB);
  - whole app: `runtimeBytes` of `node scripts/test-system-world-expansion.mjs` (< 80 MiB);
  - audio: `python assets/system-world/build_audio.py --check` total (≤ 1,572,864 bytes).
- [ ] **Regression:** the Task 16 block again, including both browser runs.
- [ ] **README and manual pictures:** none show the System World (Verified). Record "nothing to refresh"; if the coordinator finds one, copy the matching final survey PNG.
- [ ] **Coordinator: GitNexus change detection** against `main`:
  `node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope compare --base-ref main --repo .`
  A `partial` or `truncated` result is re-run. Report HIGH and CRITICAL items.
- [ ] **Coordinator: rebase.**
  - `git rebase main` on `feat/sysworld-beauty`, resolving conflicts.
  - Afterwards re-run all node tests, `go test ./ui -run '^TestDesktopSysWorld' -count=1` and `node scripts/build-system-world.js --check`.
  - Re-run the two browser runs if `main` brought changes under `ui/`.
- [ ] **Acceptance record** in the hand-back, and appended by the coordinator as "Execution notes" to this plan:
  - the W1 table: night scene, glare before and after (`phase-6-before` against `phase-6-final-high`), causes and fixes;
  - W2: tones, pane/ceramic ratio, lit share low against high;
  - W3: feet gaps of the 4 step scenes, the lobby horizon;
  - W4: triangles and calls before and after, the budget table;
  - W5: the 9 sheets with the coordinator's verdict;
  - W6: patrol and visitor findings, pot plants, zebra and kerb;
  - W7: strict checks, earshot z, the leaves verdict;
  - asset budgets; test results; the GitNexus summary.
- [ ] **Merge note** (a few lines for the user): what the branch contains (phases 0–6), the state after the rebase, open backlog items, and that nothing is pushed or deployed. Merging is the user's decision.

**Commit:** none, or `docs(system-world): phase 6 acceptance record (execution notes)` when the coordinator appends the record.

---

## Test and pin changes at a glance

| Pin or test | Old | New |
| --- | --- | --- |
| Survey `stats` | `triangles` (may include a shadow pass) | plus `viewTriangles`, `shadowFlushed`; `camera` check; metrics and `checks`; `--strict` |
| Survey scenes | — | `building`/`glare`/`masks`/`feet`/`checks` on 15 scenes; 9 `seq-*` scenes |
| `test-system-world-picture-checks.mjs` | — | new (codec, measures, budget) |
| `scripts/system-world-budget.json` | — | phase 5 medians per scene and tier (max of two runs), margin 10 %, `street-agent` view triangles < 650,000 |
| `test-system-world-lamps.mjs` | cones on and off by daylight | plus `coneFade` and the per-instance fade |
| `test-system-world-aviation.mjs` | — | roof-top lights, real LOD 2 towers |
| `test-system-world-windows.mjs` | two wall colours | `WINDOW_TONES` rules, depth falloff, furniture; optional `WIN_CAP`, window env |
| `test-system-world-footing.mjs` | kerb, ramp, random poses | plus stairs (≤ 2 cm with snap, > 5 cm without), snap has no effect off flights, shipped models on treads |
| `test-system-world-lobbies.mjs` | — | `treadCentre`, `FOOT_HALF` |
| `test-system-world-view.mjs` | — | `easePitch` |
| `test-system-world-furniture-lod.mjs` | — | sets, gate, hysteresis, tree leaves at LOD 2 |
| `test-system-world-patrol-label.mjs`, `-visitor-placement.mjs` | — | new |
| Movers and foley tests | reach 90/30; one roll band | reach 120/45; three bands, ticks; body resonance |
| Expansion browser test | 20 MiB check without a log | logs first-display bytes; lobby horizon in the middle third |
| City browser test | — | `furnitureLod` at the overview; `null` on low |
| Markers | — | the Task 16 list |
| Living test | unchanged | JSON identical after Task 7; identical apart from patrol yield seconds after Task 12 |

## Risks and open questions

1. **The find step may not isolate a single cause.** Several suspects may each cut part of a patch. Fixes then combine, and the strict glare check plus the coordinator's eye decide. Global levers (environment, moon, bloom) change the whole night mood; compare sheets guard against overcorrection.
2. **The resident sole (.41 m) is longer than a tread (.32 m).** Even centred, a sole overhangs about 4.5 cm at each end and clips into the riser above. This is accepted; deeper treads would change the phase 3 contract.
3. **Popping at 45 m.** LOD 0 → 2 may be visible on trees. The levers are hysteresis (fixed by the design), `seq-walk-80`, or a middle band at LOD 1 as a follow-up. They are not tuned here without the coordinator.
4. **Frame-time noise.** Medians may vary by ±10 % between runs. The base is the larger of two runs; one re-run is allowed for a failure within 5 %. A budget miss is reported, never hidden by a new baseline.
5. **The triangle target** may already be met by `viewTriangles`, if the shadow pass caused the inflation, or not be reachable through furniture alone. Report either outcome; the target stays.
6. **Mask accuracy.** MSAA edges blend mask colours; a tolerance of 60 and percentiles (p99, p95) limit their weight. Inventory and masks lengthen runs, so full runs use `--timeout 2400`.
7. **The patrol label may already be correct** apart from `suspend()`. Item 26 then closes with the diagnostic note, and item 14 (yield duration) stays in the backlog.
8. **Synthesis may not reach classifier z 2.5.** The design accepts recording it; route (a) needs the user.
9. **Kit rebuilds (C1, C2)** depend on the seven uncommitted `.py` edits in the worktree being understood first (Request 3).
10. **The MATRIX-only browser checks** ran last in phase 4. They may surface failures from phases 4–5 (sound check, low tier). They are findings; a check is never loosened.
11. **Rebase conflicts** with `main` (other sessions commit there). The coordinator resolves them, re-runs the cheap tests, and re-runs the browser runs if `ui/` changed.
12. **Fixed camera numbers in the sequences** (tram corner, walk street line) are first guesses. Task 11 adjusts them from pictures and records the change.

## Coordinator notes before execution (2026-11-10)

- **Backlog item 34** was added after this plan was written. In the phase 5 regression the main-path browser
  run (`AURAGO_SYSTEM_WORLD_MATRIX=1` alone) failed twice in the test agent's shell at the 10 s wait for the
  sound to start, and passed on the same commit in the coordinator's shell. Task 16 finds out before its
  browser runs whether that is time or a missing audio device, and lengthens the wait only if it is time.
- **Could-not-verify 11.** The seven `assets/system-world/*.py` files that `git status` shows as modified are
  line-ending noise in v1 and v2 sources (`git diff --ignore-cr-at-eol` is empty). They do not feed the kit v3
  build and are never staged.
- **Could-not-verify 10, Request 6.** The leaves source `0904.ogg` is still in the session scratch directory
  with the recorded sha256, so Task 14 needs no download.
- **Request 8.** The baseline for the before/after sheet is `reports/system-world-survey/baseline` (2026-11-09).

## Execution notes

### Tasks 1–3 (2026-11-10)
- **Task 2.** Frame-time medians are about 10 ms on every tier and scene on the reference machine (Arc B580), so the
  frame is bound by the update, not by triangles. `street-agent` draws 1.94 million triangles with the shadow pass
  and 882,938 without (`viewTriangles`, Task 3): the second figure is the W4 starting point.
- **Task 3.** The plan's `captureMask` awaited `toBlob` while its override materials were on; the page's own update
  then read uniforms of swapped materials and stopped the animation loop. The mask pass now freezes the update.

### Task 4 — what causes the night glare (2026-11-10)
Clean numbers need one page per target with a throw-away first picture: within one page the facade glare steps
down by itself after about the 11th picture (missions .0245 → .0157, memory .0400 → .0337, agent .0041 → .0025;
lights, emissive values and environment unchanged; cause not found, see backlog 35). The plan's single-page pass 2
was unusable for that reason.

| Patch (stable base share) | `env-off` | `bloom-off` | `windows-dark` | `emissive-all-off` | lights, lamps, pane lighting | Class |
| --- | --- | --- | --- | --- | --- | --- |
| `facade-memory-night` .0400 | −95 % | −81 % | −40 % | −16 % | ≤ 7 % | E, partly B |
| `facade-missions-night` .0245 | −95 % | −84 % | −51 % | −4 % | ≤ 9 % | E and B |
| `facade-agent-night` .0041 (passes) | −90 % | −81 % | −88 % | −3 % | ≤ 3 % | E and B |
| `lobby-memory-night` .0148 | −100 % | −63 % | 0 % | −67 % | ≤ 2 % (pane lighting −43 %) | E, A-in |
| `lobby-missions-night` .0064 | −100 % | −100 % | 0 % | −66 % | ≤ 5 % | E, A-in |
| `lobby-agent-night` .0027 (passes) | −100 % | −79 % | 0 % | −42 % | 0 % | below the limit |
| `life-street-trees-night`, `skyline-night` | base 0 | | | | | nothing to explain |

- **Cause.** The pale ceramic mirrors the bright panels of the environment map, which keeps its day intensity
  (.48) at night. Lit window rooms add to it. Neither the moonlight nor a point light nor the lamp cones matter.
- **Emissive inventory** (bloom threshold 1.06): `city.lantern-glow` 2.4 and `city.cyan` 1.68 lie above it,
  `city.warm` .845, `city.ivory` .64 and `city.jade` .624 below. Switching every emissive off changes the facades by
  4 to 16 %, so no kit emission change (C1) is needed.
- **Aviation.** All 56 points sit 1.79 to 5.27 m beside their tower's roof-top centre (class D); none needs the
  fog step.
- **Cone.** The camera of `life-street-trees-night` is 6.9 m from the nearest cone; cones cause no glare.
- **Blob (backlog 5).** `proof-window-close` on medium: no clipped pixel. Closed.
- **Low tier.** Lit share at night .78 against .775 on high: Task 6 step 5 is not needed.
- **Instruments.** `window-env-off` does nothing (the window material has no environment map of its own), so E
  and E-w cannot be told apart; `camera.blocked` depends on a moving body, not on the camera position.
- **Decision.** Task 5 (always), Task 8 step E starting at `NIGHT_ENV` .12 with the highest passing value kept,
  step L only if a lobby stays above .004, Task 6 steps 1–4 and 6 (`WIN_CAP` from 1.0). Steps M, F, C, G, A2, Task 6
  steps 5 and 7 and C1 are not applied.

### Tasks 5–9 and 12 — fixes by cause (2026-11-10/11)
- **Night light (Tasks 5, 8).** Only step E was needed: `NIGHT_ENV` .2 (the highest value that passes). Clipped
  share at night: archive front .040 → .003, mission terminal .0245 → .002, agent tower .004 → .0005, lobbies
  ≤ .002. Step L and every other Task 8 step stayed unused.
- **Windows (Task 6).** The plan's day changes (tones, furniture, depth falloff) changed the pictures hardly at
  all: by day the panes were pale because the glass mirrored the environment map (with the environment off,
  the lit share of the agent tower's panes fell from .61 to .08). The glass got its own environment intensity
  (`WINDOW_ENV` day .12, night .08; the plan's step 7 needed `material.envMap` to be set to act). Day fill .2,
  `WIN_CAP` 1.1. All four day scenes pass (pane over ceramic ≤ .93, tones ≥ 3).
- **Feet (Task 7).** Gaps on the four stair stills [0, 0]; the stills now start the resident's work clip, because
  a resident still in its walk clip had one foot in swing and the measured pose depended on the run order.
- **Patrol label (Task 12).** In the living run all `yield` label time was genuine (31 s stepping aside, 1.6 s
  holding); only a suspended held patrol kept a stale label. Backlog 14 stays open.

### Task 10 and the frame-time budget (2026-11-11)
- **The Task 2 budget was wrong.** That harness set the survey camera but not the visitor body; the app's
  traffic commit moved the camera back to the overview (y = 70) before the timed loop, so every scene was timed
  at the overview: a flat 10 ms on every tier. Later the same day Chrome switched to the integrated GPU (3.5 to 4
  times slower), which made the first Task 10 timing look like a regression. Measured on the same harness and
  GPU, the app before and after phase 6 agree within half a millisecond.
- **Harness.** It pins the high-performance GPU, fails a scene whose camera moves during the timed loop, and the
  budget check refuses reports from another adapter. The number is the uncapped cost of update and render
  submission at the scene's own pose, not a vsync frame time.
- **New budget** (phase 5 app, two runs per tier, worst median, +10 %; Arc B580): overview 7.4 / 8.5 / 9.1 ms,
  street 4.4 / 5.5 / 5.7, agent tower at night 4.6 / 5.9 / 5.9, park 2.6 / 2.9 / 3.1 on low / medium / high.
  Run-to-run spread is about half a millisecond, so rows near their limit need the allowed re-run.
- **Furniture sets.** Two sets for every design saved 100 to 300 thousand triangles but added 20 to 35 draw
  calls per view, which cost more than the triangles saved on this GPU (park on medium 2.9 → 3.4 ms). Only the
  designs worth it now have sets (`FURNITURE_LOD.worth` 8,000 triangles saved city-wide): both trees, bollards,
  flower beds, litter bins and zebra signs on high, the first five on medium. `street-agent` on high: 627,936
  view triangles (882,938 before; limit 650,000); every budget row passes.

### Tasks 13–15 — sound finish (2026-11-10)
- **Task 13 (synthesis).** First render: the tram read as a kettle and a vacuum cleaner (motor sawtooth and whine
  over a weak roar), and the footfall body layer was inaudible (a band-pass of Q 8–10 passes about 25 Hz of the
  noise). After two tuning rounds (roll 1.6, whine .09, motor .15, joint knocks, body levels 1.7–2.2, tram .7,
  visitor step scale 2.8) the strict survey passed, but the classifier still named neither sound: a tram passing
  z 0.96 (0.6 before), footsteps z 1.5 to 1.8 (unchanged). Route (b) did not reach 2.5.
- **Task 14 (leaves).** The plan's low-pass was dropped: every low-passed cut read as rain on a roof. The cut
  moved to the calm stretch from 100 s (tree wind 3.2, rain 2.24, birds 2.0; the old cut 2.84 / 2.83 / 2.07). In
  the park mix both cuts read as wind in leaves with birds (3.8) and not as rain (0.6).
- **Task 15 (recordings).** The user approved four concrete files on 2026-11-10; two are used: "Tram Passing #2"
  (BigSoundBank, Joseph Sardin; 13 s around the pass) and "light footsteps on concrete walking.wav" (Freesound,
  rsellick; twelve single footfalls on a 0.3 s grid). `build_audio.py` got the kinds `pass` and `slices`. With the
  recorded footfalls `walk-surfaces` reads as footsteps (z 3.4 on a step, 2.6 on a stretch). The tram pass first
  went through the voice's hum gain and was audible for three seconds (two distance envelopes on one sound,
  z 0.95 to 2.4); with its own gain following the square root of the distance law (`pass.level` .9, `tram` .28)
  the pass reads as a rail vehicle: z 3.3, tagger "Train" .50. Strict survey `phase-6-t15f` passes
  (`tram-corner` +2.24 LU against the baseline).

### Task 11 — sequences (2026-11-11)
Nine sequences plus `seq-patrol-pocket`, each with a contact sheet (`reports/system-world-survey/phase-6-seq`).
Judged by the coordinator: the walk of 80 m (16 frames, 5 m apart) shows no tree, bollard or bin changing
shape; the lobby door opens and the view goes level into the hall; the hall appears behind the glass on
approach; the launch crosses and turns; the banner swings; the tram passes in side view; one gull shows its
wing beats. `seq-gust` and `seq-door-resident` were judged from the maker's description and pixel
differences only (leaf sway changes 0.2 to 0.8 % of the tree's pixels per step). A change of level that
happens between two walk frames would not show.

### Task 16 and the standstill it found (2026-11-11)
- **Browser tests on a slow GPU.** With Chrome on the integrated GPU the expansion run failed in its own walking
  helper: a key press of 20 to 65 ms often fell between two frames and moved nothing. The helper now holds a key
  over whole frames. Backlog 34 (the sound start) did not reproduce.
- **Trams standing still for good.** The expansion run then failed at its tram wait. Cause: from about 115 s of
  city time a patrol robot stood in a pocket between the rails and street furniture; the tram's look-ahead
  waited for the patrol, the patrol could not step aside and its navigator found every lane blocked. Both trams
  stood until the end of a 260 s run. Deterministic, without a visitor, and the same before phase 6: it exists
  since the furniture became traffic solids (phase 4). The living test (122 s) ends just before it. Fix: a
  patrol boxed in for 3 s moves to the nearest free spot outside the rail corridor and its robot glides there
  (`corridorSpot`, `scripts/test-system-world-tram-pocket.mjs`). In 200 s no tram stands longer than 6 s; the
  rule fires five times; the living test's result line is unchanged.

### Task 17 — acceptance record (2026-11-11)
Runs: `reports/system-world-survey/phase-6-final-{high,medium,low}` (88 scenes; the low tier skips the two wind sequences;
strict on high and low, exit 0, no page error, no failed check), `phase-6-final-timing-*`, `phase-6-compare` (25 pairs against the
baseline of 2026-11-09), `reports/system-world-sound/phase-6-final`.

| Work package | Result |
| --- | --- |
| W1 night glare | Clipped share at night (limit .005), before → after on high: archive front .040 → .001, mission terminal .0245 → .0006, agent tower .0041 → .0001, `life-street-trees-night` 0, `skyline-night` 0 (the low tier's tree scene .0014). Cause: the environment map at day intensity at night; fix `NIGHT_ENV` .2. Aviation lights on roof tops, lamp cones fade near the camera. |
| W2 windows | Day scenes on high: tones 5 / 5 / 3 / 5, pane over ceramic .95 / .73 / .91 / .57 (limit 1.0; before up to 1.08, the mission terminal had 2 tones). Lit share at night .60 on high against .63 on low. |
| W3 entering | Feet gaps on the four stair stills [0, 0] on high and medium (limit 2 cm; before up to 14.7 cm; the low tier does not adapt feet). The expansion browser test asserts the horizon in the middle third before the lobby screenshot; `reports/aurora/world2-lobby-agent.png` looks into the hall. |
| W4 furniture cost | `street-agent` on high 628,072 view triangles and 563 calls (882,938 and 482 before; limit 650,000). Budget (phase 5 app, Arc B580, +10 %) against the final run in ms, low / medium / high: overview 7.3 / 8.6 / 8.7 (limits 8.14 / 9.35 / 10.01), street 4.4 / 5.7 / 5.8 (4.84 / 6.05 / 6.27), agent tower at night 4.5 / 5.7 / 5.9 (5.06 / 6.49 / 6.49), park 2.5 / 2.9 / 3.1 (2.86 / 3.19 / 3.41). One row needed the allowed re-run. |
| W5 motion | See Task 11 above. |
| W6 small fixes | Pot plants are broad-leaved plants with a soft `foliage` part; `crossing` and `kerb-close` show no clipped pixel by day (palette unchanged); patrol label and the visitor's seat after rides fixed; the tram standstill found and fixed. |
| W7 sound | Strict survey `phase-6-final`: true peak −16.4 dBTP (limit −1), day and night .95 LU apart (limit 2), shut lobby 5.91 LU below the street (minimum 3), largest rise over the baseline +2.90 LU (limit 3; `tram-corner` +2.24). Classifier: a tram passing z 3.3 (0.6 before), footsteps z 3.4 (0.6 to 1.8 before), the park as wind in leaves with birds 3.8 and not as rain 0.6. Six loops and two one-shot recordings load in all 15 situations. |

| Budget | Now | Limit |
| --- | --- | --- |
| Model directory | 14,405,534 bytes (13.7 MiB) | 40 MiB |
| First display | 4,910,208 bytes (4.7 MiB); 5,943,664 after a lobby visit | 20 MiB |
| Whole app (runtime) | 15,763,055 bytes (15.0 MiB) | 80 MiB |
| Audio | 1,537,920 bytes | 1,572,864 bytes |
| Renderer bundle | 1,025,598 bytes | – |

- **Tests.** 42 `scripts/test-system-world*.mjs`, `check_assets.py`, `build_audio.py --check`,
  `build-system-world-v3.mjs --check` (46 designs, 138 files), `build-system-world.js --check`,
  `go test ./ui -run '^TestDesktopSysWorld'`, the main-path browser run (92 s) and the expansion browser run
  (242 s; tram wait 10.4 s) pass.
- **README and manual pictures.** None show the System World; nothing to refresh.
- **GitNexus** change detection against `main` (index of `main`, partial, risk unknown): the branch's own
  changes lie in the System World modules, scripts, assets, the two licence files and the desktop `AGENTS.md`;
  every other difference it lists is one of `main`'s own commits that the branch does not have yet.
- **Not done or not proven.** Frame times on weak GPUs were not measured (the integrated GPU ran the same app
  at 20 to 44 ms where the B580 needs 4 to 9 ms; the furniture sets save triangles but no time on the B580). The
  glare numbers of a full run are on the low side (backlog 35). By day the archive hall's light bars still
  bloom and its cornice has a sun highlight. The patrol rule fires five times in 200 s; whether that looks
  restless was not judged in motion. The tram model is the plain v2 one. Open backlog: 12, 13, 20, 25, 30, 32,
  33, 35 to 39.
