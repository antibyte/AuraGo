# System World Beautification — Phase 5 Implementation Plan (soundscape)

> **For agentic workers.** One code-writing agent per task, in the order given under "Order and parallelism". A test-running agent runs the commands listed in each task. Lean workflow: one combined review per batch, Critical and Important findings only.
>
> - **Agents do not change git state.** No commits, stashes, resets or branch switches. The coordinator commits each task with the message given at the end of it (last line `Co-Authored-By: Claude <noreply@anthropic.com>`).
> - **GitNexus is the coordinator's job.** Before handing a task out, the coordinator runs `impact` (upstream) on the functions it edits, and `detect_changes` before each commit. Expected HIGH or UNKNOWN results: `createCityAmbience` (called through the module namespace from the classic script `sysworld.js`, so UNKNOWN is expected), `createCityFoley`, `createCity`, `createExperience`, `createCompanion`, `NS.createHud`, `NS.createWorldControls`. This plan is the mitigation, and its tests cover the callers. The worktree has no GitNexus index (backlog 21), so run change detection against `main`.
> - **Scripts.** Throwaway scripts go into the session scratch directory via the Write tool, never into the repo. Bash heredocs mangle backslashes on this machine.
> - **Paths.** All paths are relative to the worktree `C:\Users\Andi\Documents\repo\AuraGo\.worktrees\sysworld-beauty`. Never Glob or Grep the main repository root.
> - **Anchors, not line numbers.** Every edit names a function and quotes the anchor text. If the text has drifted, make the same change to the equivalent statement and say so in your hand-back.
> - **No audio files.** No task except the optional Task 14 adds, downloads or decodes an audio file. Task 14 runs only after the user's explicit go-ahead.

**Goal**
- The city sounds like a place:
  - the harbour laps, and boats and gulls call from where they are;
  - streets hum and echo between the towers, trams rumble past and squeal in the curves;
  - the park rustles with the real wind;
  - every lobby has its own room tone, and glass damps the city without muting it;
  - steps sound like the surface under them;
  - nothing ever clips.
- Everything stays synthesised, opt-in, gesture-unlocked and bounded. It costs nothing while muted.
- The coordinator can judge the result without ears: an offline sound survey renders WAV files with loudness and true-peak numbers.

**Architecture**
- **Approach: extend, do not rewrite.** `sysworld-audio.js` keeps its graph, its event scheduler, its marker strings and every browser-test contract. New concerns become modules with one uniform layer interface (`update`, `setTier`, `setMood?`, `clear`, `stats`, `dispose`), created in `ensure()` and driven from a new per-frame `frame(f)`. Reasons:
  - Go markers pin `clearTimeout(eventTimer)`, `createConvolver`, `thunder(` and `setMood(`.
  - The browser test reads `sound.rms`, `sound.effects.weather`, `sound.foley.*` and `sound.voice.*`.
  - The effects and voice modules need no change.
- **New modules.**
  - `sysworld-zones.js` (pure): zone weights, rooms, door state, the surface under a foot, smoothing.
  - `sysworld-sound-space.js`: glass, reverb sends, the canyon and interior convolvers, the room bus.
  - `sysworld-sound-beds.js`: leaves, laps, steam, traffic, rain, patter.
  - `sysworld-sound-movers.js`: trams, hover whirs, boats, gull positions.
  - `sysworld-sound-rooms.js`: room tones and room events.
  - `sysworld-sound-frame.js`: the scene's per-frame input.
- **Ownership.**

  | Concern | Owner |
  | --- | --- |
  | Master chain (limiter, trim, ceiling, analyser), the three buses, score (sub drone, pad), air/water/murmur/lamp beds, event table and scheduler, stored levels, `frame()` dispatch, tier | `sysworld-audio.js` |
  | Zone weights, `ROOMS` registry, door state, `surfaceAt`, tracker smoothing | `sysworld-zones.js` (pure, no three) |
  | Glass low-pass and gain, outdoor/local/room sends, canyon + interior convolvers, room bus | `sysworld-sound-space.js` |
  | Leaves (wind), laps (sea wave), steam (vents), traffic wash, rain, glass patter | `sysworld-sound-beds.js` |
  | Tram hum/squeal/clacks, hover-robot whirs, launch and barge engines, gull call positions | `sysworld-sound-movers.js` |
  | Lobby and pavilion room tones, room events | `sysworld-sound-rooms.js` |
  | Footfalls by surface (`STEP_VOICES`), visitor steps, companion steps, robot actions | `sysworld-foley.js` |
  | One-shot world effects and four spatial hum beds | `sysworld-effects.js` (unchanged) |
  | Tower voice | `sysworld-voice.js` (unchanged; off in the survey) |
  | Scene input (clocks, wind, trams, companion, door openness) | `sysworld-sound-frame.js`, filled by `sysworld-scene.js` |
  | App ↔ engine wiring (`onSoundFrame`, `setTier`) | `sysworld.js` |
  | Door openness accessor | `sysworld-experience.js` `doorValue` |
  | Companion gait accessor | `sysworld-companion.js` `soundPose` |
  | Offline renders and loudness metering | `scripts/system-world-sound-survey*.mjs`, `scripts/system-world-sound-meter.mjs` |

- **Per frame** (the scene drives everything; no audio timer runs per frame):
  1. Fixed simulation steps. `onSound` covers one-shots, robot actions and the visitor step (which now goes to the foley); `onGait` covers residents (foley strides, plus glider positions for the movers).
  2. `options.onListener(...)` → `sound.setListener(...)`. Only while running, it calls voice, effects and foley `listener`, then `updateBeds()`, which runs `zoneWeights` into `raw`.
  3. After the particles: `soundFrame.fill(...)` → `options.onSoundFrame(frame)` → `sound.frame(frame)`, which runs `doorState` → `tracker.update` → `space.update` → `beds.update` → `movers.update` → `rooms.update` → `foley.companion`.
  4. `options.onMood` on change → `setMood` → pad, beds, rooms.

  `frame()` and `setListener()` return at once while muted, hidden, unfocused or in map mode.
- **Signal chain.**
  ```
  outdoor sources (air, water, laps, murmur, lamp hum, traffic, leaves, rain, steam, trams, hovers, boats, events, thunder)
     ├─dry──► outdoor ─► glassFilter (low-pass 18 kHz → 900 Hz) ─► glassGain (0 → −12 dB) ─► ambienceBus
     └─send─► sendOutdoor ─┬► openSplit (1 − canyon) ─► verb (existing 3.4 s convolver) ─► verbReturn .55 ─► outdoor
                           └► canyonSplit (canyon) ─► canyonConv (0.45 s taps; high/ultra) ─► canyonReturn .45 ─► outdoor
  score (sub drone, pad) ─► ambienceBus            pad ─► padSend ─► verb
  room tones ─dry─► room ─────────────────────────────┐
             └send─► roomSend ─► interiorConv (room IR; medium+) ─► interiorReturn .5 ─► room ─► roomPan ─► ambienceBus
  local (foley steps and actions) ─send─► sendLocal ─┬► interiorSplit (interior) ─► interiorConv
                                                     └► outdoorSplit (1 − interior) ─► sendOutdoor
  foley dry, effects one-shots ─► effectsBus          patter (on the glass) ─► ambienceBus
  ambienceBus, effectsBus, voiceBus ─► master (volume ≤ .35) ─► compressor ─► trim ─► ceiling (WaveShaper) ─► analyser ─► destination
  ```
  - Room bus gain: `interior + (1 − interior)·leak·.35`.
  - Low tier: no `canyonConv` (`canyonSplit` feeds `verb`) and no `interiorConv` (`interiorSplit` and `roomSend` feed `verb` through a .3 gain).

**Tech notes**
- **Native nodes only:** Gain, BiquadFilter, Oscillator, AudioBufferSource, StereoPanner, Convolver, DynamicsCompressor, WaveShaper, Analyser. No AudioWorklet and no PannerNode (deviation 4).
- **DynamicsCompressor makeup gain.** The Web Audio spec (and Chromium's `DynamicsCompressorKernel`) applies an automatic makeup gain of `(1/fullRangeGain)^0.6`, where `fullRangeGain` is the curve's output for a 0 dBFS input.
  - For threshold −10, knee 6 and ratio 12: knee constant k ≈ 9.2, knee end ≈ −7.56 dB, output at 0 dBFS ≈ −7.23 dB, makeup ≈ ×1.647 (+4.33 dB). Hence `LIMITER.trim = .607`.
  - Task 3 measures it, and the measurement wins.
- **Offline stepping.** At 48 kHz a 25 Hz step is 1920 frames, exactly 15 render quanta, so every `suspend(k/25)` lands on a quantum boundary.
- **Conventions (as today).** Distance runs on the ground from the acoustic origin; pan follows the camera (`bearingPan`, the formula of `reach()`).
- **Shares vs proximity.**
  - `zoneWeights` returns outdoor *shares* `harbour, canyon, green, plaza, height` (sum 1), `interior` (0/1), and `quay`, the raw harbour proximity before normalisation.
  - The water bed and laps use `quay`; reverb sends and event weights use shares.
- **Automation discipline.** An AudioParam is scheduled only when its quantized target changes, so a still scene queues no automation.

---

## What I verified, and what I could not

**Verified by reading** (worktree at `4a9cd8670`)
- **`sysworld-audio.js` `createCityAmbience(onState)`.**
  - It takes a single parameter, and `ensure()` creates `new (window.AudioContext||window.webkitAudioContext)()`.
  - Graph: `master` → `analyser` → destination; buses `ambientBus`, `effectsBus`, `voiceBus`; `verb` (`impulse(context,3.4,2.6)`) → `verbReturn` .55 → `ambientBus`; pad send → `verb`.
  - Beds `airGain` .32, `waterGain`, `murmurGain`, `lampGain`, all → `ambientBus`.
  - `MAX_EVENTS=12`. `schedule()` and `progress()` use global `setTimeout`. `Math.random` is used throughout (brown noise, white `noise`, events).
  - `updateBeds()` quantizes with `toFixed(3)` strings (an allocation per frame) and zeroes water and lamp by the `inside` flag.
  - `setListener` uses `Object.assign(listener,{…})` per frame and always calls `effects.listener`, which schedules four `setTargetAtTime` per call.
  - `sync()` calls `context.resume()`/`suspend()`. `dispose()` closes the context.
  - `stats().soundscape.beds.{water,murmur,lamp}`.
- **`sysworld-foley.js` `createCityFoley(context,destination,ambience)`.**
  - `MAX_VOICES=10`, `MAX_DISTANCE=48`, `RETRIGGER=.25`, `LAYER=4.5`.
  - `voice(kind,x,y,z,id)`. The send goes to the third argument (today `ambientBus`, never the convolver).
  - `gait()` uses strides .8/1.05/1.6 by distance.
- **`sysworld-effects.js`.** Specs `step`/`step_inside`. The air bed rain hiss is .07 in rain and ×.12 inside. Four fixed spatial beds.
- **`sysworld-voice.js`.** It fetches `/api/desktop/system-world/voice`; nothing else does network work.
- **`sysworld.js` (classic script).**
  - `inst.sound=module.createCityAmbience(...)` is created before `createCity`.
  - Callbacks `onListener`, `onSound`, `onGait`, `onThunder`, `onMood`, `onEnvironment` (→ `worldControls.environment` → `sound.setEnvironment(inside,weather)` on change).
  - `onQuality:(value,actual)=>…` receives the actual tier. Dispose disposes the sound before the city.
- **`sysworld-scene.js` `update()`.**
  - `options.onListener` is called before `const mood=weather.mood()`.
  - `const wind = atmosphere.wind(), seaTime = …` comes before `particles.update(dt, !reduced, wind);`.
  - `onMood` fires on change.
  - `applyTier()` calls `options.onQuality?.(quality, tier)`.
  - `traffic` is in scope. `companion` is assigned after `experience`.
- **`sysworld-atmosphere.js`.** `wind()` returns an object whose `strength` is forced to 0 on low tier or when not animated. `seaTime()`. `windAt(t,weather,out)` and `seaHeight(x,z,t)` are pure.
- **`sysworld-experience.js`.**
  - Trams are traffic bodies `'tram-0'`/`'tram-1'` with `enabled = visible`; one on low.
  - `traffic.body(id)` exists (`sysworld-traffic.js`).
  - `options.onEnvironment?.(!!room)` is called every frame. `room` comes from `roomAt` or a non-open `lobbyAt` hall below `SOLID_TOP`.
  - Pavilion doors are in the private `doors` Map (`value` 0..1). Lobby doors are exposed as `lobbies` with `value(key)`.
  - The visitor step `callSound(room?'step_inside':'step', camera.position)` fires every 1.8 m at eye height (2.4 m above the floor).
- **`sysworld-society.js` `update()`.** It calls `onGait(id,x,y,z,moving,carrying, role==='patrol'?'glide':'')` for every actor every frame. Patrols are the gliding street robots of `sysworld-life.js` (hovering at 1.6 m).
- **`sysworld-companion.js`.** `companionPose(time)` is pure. `LOOP` = {cx 30, cz 69.5, gaitHz 1.7}. `time` is internal. The header says it never joins the foley.
- **Registry.**
  - `LOBBIES` ids: agent, memory, integrations-west, integrations-east, missions, infra, graph (`open:true`).
  - `DOORS` keys are `lobby:<id>:<n>` at `z = leaf`.
  - `SOLID_TOP=8`. `interiors` (pavilions agent/memory/missions at z 73, 12×12, `doorZ = z−6`).
  - `roomAt` (exploration), `lobbyGround`/`podiumAt`, `streetProfile(x,z).kind` ∈ road/kerb/pavement (it allocates its return object), `groundHeight`.
- **City life inputs.** `SURFACES` (3 lawns, 3 paving). `steamVents()` (11 vents with `size`). `craftPoses`/`CRAFT` (launch index 0, barge index 1). `gullPose(i,t,barge,out)`, `FLOCK` (low 0).
- **Heights.** `data-tower-a` LOD 0 bounds max y 38.79 (v3 manifest); towers are y-scaled by `scale`.
- **Tests and docs.**
  - `scripts/test-system-world-foley.mjs` uses an inline fake without `createDynamicsCompressor`, and asserts `ambience.effect('step',2,0,0)` does **not** reach the foley.
  - The browser test asserts `sound.rms<.05`, `sound.effects.weather==='rain'`, `foley.peak>=.025` and voice gain, and reads stored keys only for `sound`. The stress test checks closed contexts.
  - `TestDesktopSysWorldAppMarkers` pins the audio strings above.
  - `ui/js/desktop/apps/AGENTS.md` says: "Do not add a `DynamicsCompressorNode`: its automatic makeup gain lifts the mix above the browser test's 0.05 RMS voice ceiling."
- **Tooling.**
  - `scripts/build-system-world.js` bundles from `sysworld-scene.js` (audio included). `/reports` is git-ignored.
  - `scripts/system-world-survey.mjs` (esbuild in-memory bundle + local server + headless Chrome) is the model for the sound survey.
  - Node tests import `three` and `sysworld-scene.js` without trouble.

**Could not verify** (check these first)
1. The exact makeup gain of Chrome's compressor for these settings (estimated +4.33 dB). Task 3 `--calibrate` measures it.
2. `OfflineAudioContext.suspend/resume` at 25 Hz in `--headless=new`: whether it works, how fast it renders, and whether `ended` events fire **during** offline rendering. If they do not, transient pools (events ≤ 12, foley ≤ 10, laps ≤ 3) fill and stop. Task 3 logs `stats.soundscape.transient` and event counts. Fallback: release transients by a scheduled time check in `frame()`.
3. Whether earshot reads 32-bit float WAV with a 16-byte `fmt` chunk. Fallback: Task 3 adds `--pcm24`.
4. Perceived balance. Every gain below is a first estimate; Task 12 tunes within the listed ranges.
5. Tree positions near the quay and streets (green share). The zone fixtures assert ranking and loose bounds, not exact values.
6. Real CPU cost in browsers. The plan reports node counts and the offline real-time factor per tier as proxies.
7. Where `u = 0` lies on the tram route (`streetCurve`). The `tram-corner` situation's start `u` is a guess that Task 3 adjusts.

## Deviations from the design

1. **The listener side resolves the surface** (decision 6). Emitters do not pass it.
   - `surfaceAt(x,y,z)` in `sysworld-zones.js` runs when a footfall actually plays, for residents, the visitor and the companion.
   - The `onGait` chain and `sysworld-society.js` stay untouched.
   - Reason: no per-frame cost for about 31 actors, one rule for every emitter, and no change to the chaotic resident simulation.
2. **`setEnvironment` keeps its boolean** (work package 2). The room id comes from `zoneWeights`/`interiorAt` on the listener position, using the same registry functions the experience uses (`roomAt`, `lobbyAt`, `SOLID_TOP`).
   - Pavilion and lobby ids collide (`agent`), so rooms are namespaced `lobby:<id>` / `pavilion:<id>`.
   - This avoids touching the controls/weather chain.
3. **Limiter = compressor + trim + ceiling** (decision 1). The DynamicsCompressor has the design's settings (−10/6/12/3 ms/250 ms). It is followed by:
   - a fixed trim that cancels the spec's makeup gain (the AGENTS rule; without it the browser test's 0.05 RMS ceiling breaks);
   - a WaveShaper ceiling, linear below 0.6 (−4.4 dBFS), that never exceeds 0.89 (−1 dBFS), oversampled 4×.

   A compressor without lookahead cannot guarantee the design's −1 dBTP on its own.
4. **No PannerNode.** StereoPanner plus hand-made distance laws, as today. It renders identically offline and costs less.
5. **"Hover robots" are the gliding patrol robots** (style `'glide'`), tracked from the existing gait calls. The service drones (30–70 m up) stay silent.
6. **The interior convolver is replaced** (a new node), not re-buffered. The swap happens only when its send is near silent (or after 1.5 s), which avoids the "buffer can be set once" rule of older engines.
7. **The canyon space is a ConvolverNode** with a generated tap IR (seven early echoes plus a faint tail), present on high/ultra only.
8. **Two clocks for events.** Room events, laps and clacks run on the audio clock inside `frame()`. City events keep their `setTimeout` scheduler, now through injected timers, so the marker and behaviour stay and the offline survey can drive them.
9. **Companion steps:** four ticks per gait cycle, the front and hind foot of each diagonal pair 25 ms apart.
10. **The garden court has no leaves of its own.** It counts as green, so the leaves bed covers it; the room tone adds the water trickle.
11. **The traffic bed is a tyre/rumble wash** of the city beyond (there are no cars), shaped by canyon, plaza, height, busy and rain.
12. **Stored levels** live under `aurago.desktop.sysworld.volume` and `aurago.desktop.sysworld.channels`. HUD and controls read them directly because they are built before the engine exists.
13. **The analyser now reads after the limiter.** It still measures what the speakers receive; with the trim, unity holds below the threshold.

## Requests to the coordinator

1. **Commit Task 1 before Task 3.** The baseline renders on that commit. Check Task 3's `calibration.json` before Task 10; if `suggestedTrim` falls outside .55–.68, stop and decide.
2. **Listening pass (Task 12).** Run earshot on the rendered files: the expectations table is in Task 12. Decide tuning within the ranges, and whether the target "a shut lobby is ≥ 3 LU under the street by day" is right.
3. **Recordings (Task 14).** Ask the user explicitly whether a few CC0 recordings may be downloaded. Without a yes, Task 14 is dropped and the phase is complete.
4. Run the opt-in `AURAGO_SYSTEM_WORLD_AUDIO=1` browser block once in Task 13. It logs the master RMS of the new mix.
5. The resident simulation must not change. No task edits `sysworld-society.js`, `sysworld-life.js`, `sysworld-navigation.js` or `sysworld-traffic.js`; reject any diff that does.

---

## File map

| File | Status | Responsibility |
| --- | --- | --- |
| `ui/js/desktop/apps/sysworld-audio.js` | edit (1, 10, 11) | Injection, limiter, stored levels, `frame`, `setTier`, zones/space/layers wiring, gated events, visitor steps |
| `ui/js/desktop/apps/sysworld-zones.js` | new (2) | Zones, rooms, doors, surfaces, tracker |
| `ui/js/desktop/apps/sysworld-sound-space.js` | new (4) | Glass, sends, canyon/interior convolvers, room bus |
| `ui/js/desktop/apps/sysworld-sound-beds.js` | new (5) | Leaves, laps, steam, traffic, rain, patter |
| `ui/js/desktop/apps/sysworld-sound-movers.js` | new (6) | Trams, hovers, boats, gulls |
| `ui/js/desktop/apps/sysworld-sound-rooms.js` | new (7) | Room tones |
| `ui/js/desktop/apps/sysworld-foley.js` | edit (8) | `STEP_VOICES`, `step`, `companion`, `setTier` |
| `ui/js/desktop/apps/sysworld-sound-frame.js` | new (9) | Scene input |
| `ui/js/desktop/apps/sysworld-scene.js`, `sysworld.js`, `sysworld-experience.js`, `sysworld-companion.js` | edit (9) | Frame, forwarding, `doorValue`, `soundPose` |
| `ui/js/desktop/apps/sysworld-hud.js`, `sysworld-controls.js` | edit (1) | Sliders start from stored levels |
| `scripts/system-world-audio-fake.mjs` | new (1) | Shared Web Audio fake |
| `scripts/test-system-world-soundscape.mjs` | new (1), append (10, 11) | Engine integration and budget |
| `scripts/test-system-world-{zones,sound-meter,sound-space,sound-beds,sound-movers,sound-rooms,sound-frame}.mjs` | new | Unit tests |
| `scripts/test-system-world-foley.mjs` | edit (1, 8, 10) | Shared fake, surfaces, companion, step pin |
| `scripts/system-world-sound-meter.mjs`, `system-world-sound-survey.mjs`, `system-world-sound-survey.page.mjs`, `system-world-sound-survey.situations.json` | new (3) | Offline survey |
| `ui/desktop_sysworld_test.go`, `ui/desktop_sysworld_city_browser_test.go` | edit (1, 13) | Stored-level reset, markers, soundscape check |
| `ui/js/desktop/apps/AGENTS.md` | edit (1, 13) | Compressor rule, sound docs |
| `assets/system-world/README.md`, `assets/system-world/AGENTS.md`, `plans/2026-11-09-system-world-beauty-design.md` | edit (13) | Docs, roadmap row |
| `ui/js/vendor/system-world/*` | regenerate (13) | Bundle |

## Invariants every task must keep

- **Residents untouched.** No edit to `sysworld-society.js`, `sysworld-life.js`, `sysworld-navigation.js` or `sysworld-traffic.js`. Sound only listens.
- **Go markers.** Every string in `TestDesktopSysWorldAppMarkers` stays, including `clearTimeout(eventTimer)`, `createConvolver`, `thunder(` and `setMood(` in `sysworld-audio.js`.
- **Survey patch anchors.** `sysworld-scene.js` keeps `  return {\n    districts, canvas, update, focus(id)`; `sysworld-society.js` keeps `    add,patrol,update,suspend,\n`.
- **Nothing external.**
  - No audio files.
  - No `fetch` in any new module (the only fetch stays the tower voice's).
  - No host names or URLs in shipped code.
- **Opt-in rules.**
  - The engine starts only from `unlock()`/`toggle()`.
  - `frame()` and `setListener()` do nothing while not `running()`.
  - Dispose releases every node, and closes the context only when the engine created it.
- **No per-frame allocation** in:
  - `setListener`/`updateBeds`;
  - `frame`, `zoneWeights`, `doorState`, `tracker.update`;
  - `space.update`;
  - `beds.update` (except a lap);
  - `movers.update` (except building or releasing a voice, and a clack);
  - `rooms.update` (except a tone switch or an event);
  - `soundFrame.fill`.
- **Automation only on change.** AudioParam targets are set only when their quantized value changes.
- **Determinism.** Every random draw in the audio-side modules goes through the injected `random`, so survey renders repeat exactly.
- **Analyser placement.** The analyser stays the last node before the destination (the browser test's `rms`).
- **Bundle staleness.** `node scripts/build-system-world.js --check` is red from Task 1 (the bundle contains the audio code) until Task 13.
- **One fake.** Tests use `scripts/system-world-audio-fake.mjs` and extend it there, never by copy.

## Order and parallelism

| Step | Task | Notes |
| --- | --- | --- |
| A | 1 ∥ 2 | Disjoint files. 1 = audio foundations, shared fake, foley-test switch, stored levels. 2 = zones module and test |
| B | 3 ∥ 4 ∥ 5 ∥ 6 ∥ 7 ∥ 8 ∥ 9 | All after 1; 5–8 also after 2. Disjoint: 3 survey scripts; 4 space; 5 beds; 6 movers; 7 rooms; 8 foley + foley test; 9 frame + scene + sysworld + experience + companion. 3 renders the baseline on Task 1's commit |
| C | 10 | After 2, 4, 8: wire space, zones, glass, gating, beds-by-zone and visitor steps into `sysworld-audio.js` |
| D | 11 | After 5, 6, 7, 10: wire beds, movers and rooms; tiers; budget test |
| E | 12 | After 3, 9, 11: survey run, checks, tuning, coordinator listening |
| F | 13 | Docs, markers, browser check, bundle, regression, budget report |
| G | 14 | **Optional, only after the user's go-ahead.** CC0 recordings |

---

## Task 1 — Audio foundations: injection, limiter, stored levels, shared fake

**Files:**
- `ui/js/desktop/apps/sysworld-audio.js`
- new `scripts/system-world-audio-fake.mjs`
- new `scripts/test-system-world-soundscape.mjs`
- `scripts/test-system-world-foley.mjs`
- `ui/js/desktop/apps/sysworld-hud.js`
- `ui/js/desktop/apps/sysworld-controls.js`
- `ui/desktop_sysworld_city_browser_test.go`
- `ui/js/desktop/apps/AGENTS.md`

**Steps**
- [ ] Write the fake and the soundscape test below, and switch the foley test to the fake. `node scripts/test-system-world-soundscape.mjs` → **FAIL** (`LIMITER` not exported).
- [ ] Edit `sysworld-audio.js` as listed. Both tests → PASS.

### `scripts/system-world-audio-fake.mjs` (complete)

```js
// Deterministic Web Audio double shared by the System World sound tests (phase 5). It models only what the city's
// sound modules use: parameters with their automation calls, node links, sources, filters, panners, convolvers,
// compressors, wave shapers, buffers and the timers the soundscape schedules with. Nothing renders audio.
export function createFakeAudio({sampleRate=8000}={}){
  const nodes=[],tasks=new Map();let id=0;
  const parameter=(value=0)=>({value,ramp:[],
    setValueAtTime(v,t){this.value=v;this.ramp.push(['set',v,t]);return this;},
    linearRampToValueAtTime(v,t){this.value=v;this.ramp.push(['linear',v,t]);return this;},
    exponentialRampToValueAtTime(v,t){this.value=v;this.ramp.push(['exp',v,t]);return this;},
    setTargetAtTime(v,t,c){this.value=v;this.ramp.push(['target',v,t,c]);return this;},
    cancelScheduledValues(){this.ramp.length=0;return this;}});
  function node(kind){
    const n={kind,links:[],type:'',buffer:null,curve:null,oversample:'none',normalize:true,loop:false,reduction:0,started:false,stopped:false,disconnected:false,onended:null,
      gain:parameter(1),frequency:parameter(350),Q:parameter(1),pan:parameter(0),delayTime:parameter(0),detune:parameter(0),playbackRate:parameter(1),
      threshold:parameter(-24),knee:parameter(30),ratio:parameter(12),attack:parameter(.003),release:parameter(.25),
      connect(other){this.links.push(other);this.disconnected=false;return other;},disconnect(){this.disconnected=true;this.links.length=0;},
      start(){this.started=true;},stop(){this.stopped=true;},getFloatTimeDomainData(target){target.fill(0);}};
    nodes.push(n);return n;
  }
  const context={currentTime:0,state:'running',sampleRate,destination:node('destination'),
    createGain:()=>node('gain'),createBiquadFilter:()=>node('filter'),createBufferSource:()=>node('source'),createOscillator:()=>node('oscillator'),
    createDelay:()=>node('delay'),createConvolver:()=>node('reverb'),createWaveShaper:()=>node('shaper'),createDynamicsCompressor:()=>node('compressor'),
    createStereoPanner:()=>node('pan'),createAnalyser:()=>node('analyser'),
    createBuffer:(channels,length,rate=sampleRate)=>({numberOfChannels:channels,length,sampleRate:rate,duration:length/rate,
      data:Array.from({length:channels},()=>new Float32Array(length)),getChannelData(c){return this.data[c];}}),
    decodeAudioData:async()=>({duration:1,numberOfChannels:1,sampleRate,getChannelData:()=>new Float32Array([0,.8,-.8,0])}),
    resume:()=>Promise.resolve(),suspend:()=>Promise.resolve(),close:()=>Promise.resolve()};
  const timers={
    setTimeout(fn,ms){tasks.set(++id,{fn,ms,due:context.currentTime+Math.max(0,Number(ms)||0)/1000});return id;},
    clearTimeout(handle){tasks.delete(handle);},
    // Runs every task due by `time` (context seconds) in due order, advancing currentTime; tasks added meanwhile run if due.
    run(time){context.currentTime=time;for(let guard=0;guard<256;guard++){let key=null,best=null;for(const [k,t]of tasks)if(t.due<=time&&(!best||t.due<best.due)){key=k;best=t;}if(!best)return;tasks.delete(key);best.fn();}},
  };
  // A source ends when the engine stops it. The real API reports that asynchronously.
  const flush=()=>{for(const n of nodes)if(n.onended&&!n.ended){n.ended=true;n.onended();}};
  const live=kind=>nodes.filter(n=>!n.disconnected&&(!kind||n.kind===kind));
  // Every node reachable downstream of `from` along current links (an AudioParam target is a leaf).
  function downstream(from){const seen=new Set(),stack=[from];while(stack.length){const n=stack.pop();if(!n||seen.has(n))continue;seen.add(n);for(const l of n.links||[])stack.push(l);}return seen;}
  return {context,nodes,node,tasks,timers,flush,live,downstream};
}
```

### `scripts/test-system-world-foley.mjs` (switch to the fake)
- Add at the top, after the two imports:
  ```js
  import {createFakeAudio} from './system-world-audio-fake.mjs';
  ```
- Replace everything from `// A deterministic Web Audio double. Only the parts the city sound engine uses are` through `const kinds=()=>nodes.filter(n=>n.kind==='pan').map(n=>n.pan.value);` with:
  ```js
  // The shared deterministic Web Audio double (scripts/system-world-audio-fake.mjs).
  const native={setTimeout,clearTimeout,window:globalThis.window,localStorage:globalThis.localStorage};
  const fake=createFakeAudio(),{context,nodes,node,flush}=fake;
  const kinds=()=>nodes.filter(n=>n.kind==='pan').map(n=>n.pan.value);
  ```
- Replace the two lines `  globalThis.setTimeout=(fn,ms)=>(tasks.set(++id,{fn,ms}),id);` and `  globalThis.clearTimeout=handle=>tasks.delete(handle);` with:
  ```js
    globalThis.setTimeout=fake.timers.setTimeout;
    globalThis.clearTimeout=fake.timers.clearTimeout;
  ```

### `scripts/test-system-world-soundscape.mjs` (complete for Task 1; Tasks 10 and 11 append)

```js
// Phase 5 soundscape: injection, the limiter chain, stored levels, gesture and owner rules (Task 1); zones, glass,
// gating, beds by zone and visitor steps (Task 10); layers, tiers and the voice and node budget (Task 11).
import assert from 'node:assert/strict';
import {createFakeAudio} from './system-world-audio-fake.mjs';
import {createCityAmbience,LIMITER,ceilingCurve,citySoundscapeEvents} from '../ui/js/desktop/apps/sysworld-audio.js';

const seeded=seed=>{let s=seed>>>0;return()=>(s=(Math.imul(s,1664525)+1013904223)>>>0)/4294967296;};
const memory=initial=>{const m=new Map(Object.entries(initial||{}));return {getItem:k=>m.has(k)?m.get(k):null,setItem:(k,v)=>m.set(k,String(v))};};
function city(extra={}){const fake=createFakeAudio(),storage=extra.storage||memory();
  const sound=createCityAmbience(()=>{},{context:fake.context,timers:fake.timers,random:seeded(5),voice:false,...extra,storage});return {fake,sound,storage};}
const frameOf=(o={})=>({time:0,seaTime:0,wind:{x:1,z:0,strength:.4},trams:[{x:0,z:0,heading:0,on:false},{x:0,z:0,heading:0,on:false}],
  companion:{x:0,z:0,phase:0,on:false},doorValue:()=>0,reduced:false,...o});
const ramps=fake=>fake.nodes.reduce((n,x)=>n+x.gain.ramp.length+x.frequency.ramp.length+x.pan.ramp.length+x.detune.ramp.length,0);

// --- Task 1: limiter, injection, stored levels ---------------------------------------------------------------------
{
  const curve=ceilingCurve();assert.ok(curve.every(v=>Math.abs(v)<=LIMITER.ceiling),'the ceiling never passes -1 dBFS');
  for(let i=0;i<curve.length;i++){const x=i*2/(curve.length-1)-1;if(Math.abs(x)<=LIMITER.linear)assert.ok(Math.abs(curve[i]-x)<1e-6,'linear below '+LIMITER.linear);if(i)assert.ok(curve[i]>=curve[i-1],'monotonic');}
  const {fake,sound}=city({storage:memory({'aurago.desktop.sysworld.sound':'true'})});
  assert.equal(sound.stats().state,'uninitialized','nothing before a gesture, even when switched on');
  assert.equal(fake.nodes.length,1,'no node before the gesture (the fake destination only)');
  sound.unlock();sound.setActive(true);
  const compressor=fake.nodes.find(n=>n.kind==='compressor');assert.ok(compressor,'a compressor guards the master');
  assert.deepEqual([compressor.threshold.value,compressor.knee.value,compressor.ratio.value,compressor.attack.value,compressor.release.value],
    [LIMITER.threshold,LIMITER.knee,LIMITER.ratio,LIMITER.attack,LIMITER.release]);
  const trim=compressor.links[0];assert.equal(trim.gain.value,LIMITER.trim,'the trim cancels the makeup gain');
  const shaper=trim.links[0];assert.equal(shaper.kind,'shaper');assert.equal(shaper.oversample,'4x');
  const analyser=fake.nodes.find(n=>n.kind==='analyser');assert.ok(shaper.links.includes(analyser)&&analyser.links.includes(fake.context.destination),'compressor -> trim -> ceiling -> analyser -> destination');
  const master=fake.nodes.find(n=>n.kind==='gain'&&n.links.includes(compressor));assert.ok(master&&master.links.length===1,'the master feeds only the limiter');
  assert.deepEqual(fake.nodes.filter(n=>n.links.includes(fake.context.destination)),[analyser],'nothing bypasses the limiter');
  assert.ok(sound.stats().limiter.trim===LIMITER.trim&&sound.stats().graph.nodes>20);
  sound.dispose();
}
// Stored levels: written on change, read at creation, clamped.
{
  const storage=memory(),{sound}=city({storage});sound.setVolume(.3);sound.setChannel('effects',.4);
  assert.equal(storage.getItem('aurago.desktop.sysworld.volume'),'0.3');
  assert.deepEqual(JSON.parse(storage.getItem('aurago.desktop.sysworld.channels')),{ambience:1,effects:.4,voice:1});
  const again=city({storage}).sound;assert.equal(again.stats().volume,.3);assert.equal(again.stats().channels.effects,.4);
  const bad=city({storage:memory({'aurago.desktop.sysworld.volume':'9','aurago.desktop.sysworld.channels':'{"voice":-2,"ambience":"x"}'})}).sound;
  assert.equal(bad.stats().volume,.35,'volume clamps to its maximum');assert.equal(bad.stats().channels.voice,0);assert.equal(bad.stats().channels.ambience,1);
  sound.dispose();again.dispose();bad.dispose();
}
// Injected context and timers: the owner closes the context; a muted city does no per-frame work.
{
  const {fake,sound}=city({enabled:true});let closed=false;fake.context.close=()=>{closed=true;return Promise.resolve();};
  sound.unlock();sound.setActive(true);assert.ok(fake.tasks.size>=2,'events and chords run on the injected timers');
  sound.setActive(false);assert.equal(sound.stats().soundscape.scheduled,false);
  const before=ramps(fake);
  for(let i=0;i<30;i++){sound.setListener(0,2.4,76,0,-1,0,76);sound.frame(frameOf());}
  assert.equal(ramps(fake),before,'a muted city schedules nothing per frame');
  sound.dispose();assert.equal(closed,false,'an injected context belongs to its owner');
}
// Determinism: the same seed builds the same noise.
{
  const a=city({enabled:true}),b=city({enabled:true});a.sound.unlock();b.sound.unlock();
  const buf=f=>f.nodes.find(n=>n.kind==='source'&&n.buffer)?.buffer.getChannelData(0);assert.deepEqual(buf(a.fake),buf(b.fake));
  a.sound.dispose();b.sound.dispose();
}
console.log('System World soundscape: limiter chain, stored levels, gesture and owner rules, deterministic injection verified');
```

### `sysworld-audio.js` edits
1. **Limiter exports.** After `const MAX_EVENTS=12;` insert:
   ```js
   // Master protection (phase 5): a DynamicsCompressor with the design's settings, a fixed trim that cancels the gain the
   // Web Audio spec adds by itself ((1/fullRangeGain)^0.6, about +4.3 dB here), and a wave-shaper ceiling that is linear
   // below `linear` and never passes `ceiling` (-1 dBFS). The sound survey measures the trim (--calibrate).
   export const LIMITER=Object.freeze({threshold:-10,knee:6,ratio:12,attack:.003,release:.25,trim:.607,ceiling:.89,linear:.6});
   export function ceilingCurve(n=2049){const c=new Float32Array(n),{ceiling,linear}=LIMITER,span=ceiling-linear;
     for(let i=0;i<n;i++){const x=i*2/(n-1)-1,a=Math.abs(x);c[i]=Math.sign(x)*(a<=linear?a:linear+span*Math.tanh((a-linear)/span));}return c;}
   export function createLimiter(context){
     const compressor=context.createDynamicsCompressor(),trim=context.createGain(),ceiling=context.createWaveShaper();
     compressor.threshold.value=LIMITER.threshold;compressor.knee.value=LIMITER.knee;compressor.ratio.value=LIMITER.ratio;
     compressor.attack.value=LIMITER.attack;compressor.release.value=LIMITER.release;
     trim.gain.value=LIMITER.trim;ceiling.curve=ceilingCurve();ceiling.oversample='4x';compressor.connect(trim).connect(ceiling);
     return {input:compressor,output:ceiling,compressor,nodes:[compressor,trim,ceiling]};
   }
   ```
2. **Signature and injection.** `export function createCityAmbience(onState=()=>{}) {` → `export function createCityAmbience(onState=()=>{},options={}) {`. Directly after it insert:
   ```js
     // Injection (phase 5): the sound survey renders this same graph through an OfflineAudioContext with audio-clock
     // timers and a seeded random. Defaults are the live browser: window.AudioContext, the global timers, Math.random,
     // localStorage. An injected context belongs to its owner (never closed here); `offline` skips resume/suspend.
     const timers=options.timers||globalThis,random=options.random||Math.random,offline=!!options.offline;
     const storage=options.storage===undefined?globalThis.localStorage:options.storage;
     const setTimeout=(fn,ms)=>timers.setTimeout(fn,ms),clearTimeout=handle=>timers.clearTimeout(handle);
     let tier=['low','medium','high','ultra'].includes(options.tier)?options.tier:'high',frames=0,compressor=null;
   ```
3. **Stored state.** Replace `try {enabled=localStorage.getItem('aurago.desktop.sysworld.sound')==='true';}catch(_){}` with:
   ```js
     try {enabled=options.enabled===true||storage?.getItem('aurago.desktop.sysworld.sound')==='true';}catch(_){}
     // Stored levels (phase 5): volume and the three channel sliders persist like the on/off switch.
     try {const raw=storage?.getItem('aurago.desktop.sysworld.volume'),v=Number(raw);if(raw!=null&&Number.isFinite(v))volume=Math.max(0,Math.min(.35,v));}catch(_){}
     try {const saved=JSON.parse(storage?.getItem('aurago.desktop.sysworld.channels')||'{}');for(const key of Object.keys(levels))if(Number.isFinite(saved?.[key]))levels[key]=Math.max(0,Math.min(1,saved[key]));}catch(_){}
     const save=(key,value)=>{try{storage?.setItem(key,value);}catch(_){}};
   ```
4. **`ensure()`.**
   - `context=new (window.AudioContext||window.webkitAudioContext)();` → `context=options.context||new (window.AudioContext||window.webkitAudioContext)();`
   - `analyser=node(context.createAnalyser());analyser.fftSize=512;master.connect(analyser);analyser.connect(context.destination);` →
     ```js
         const limiter=createLimiter(context);limiter.nodes.forEach(node);compressor=limiter.compressor;
         analyser=node(context.createAnalyser());analyser.fftSize=512;master.connect(limiter.input);limiter.output.connect(analyser);analyser.connect(context.destination);
     ```
   - `voice=createTowerVoice(context,voiceBus);` → `voice=options.voice===false?null:createTowerVoice(context,voiceBus);`
5. **Random.** Replace **every** `Math.random()` in `sysworld-audio.js` with `random()`: the brown and white noise loops in `ensure()`, every event voice, `positionFor`, `schedule`, `thunder`. Afterwards `Grep "Math.random" ui/js/desktop/apps/sysworld-audio.js` must find only `random=options.random||Math.random`.
6. **`updateBeds()`.** `if(!context||!waterGain||!murmurGain||!lampGain)return;` → `if(!context||!waterGain||!murmurGain||!lampGain||!running())return;`
7. **`sync()`.**
   - `void context.resume().catch(()=>{});` → `if(!offline)void context.resume().catch(()=>{});`
   - After `if(!eventTimer)schedule();if(!chordTimer)progress();` add `refresh();`
   - `fadeTimer=setTimeout(()=>{if(!disposed&&(!enabled||!active))void context.suspend().catch(()=>{});},180);` → prefix with `if(!offline)`.
   - Above `function sync(` insert:
     ```js
       // The listener-driven parts, refreshed on every unmute (setListener skips them while silent).
       function refresh(){voice?.setListener(listener.x,listener.y,listener.z,listener.fx,listener.fz);effects?.listener(listener.x,listener.y,listener.z,listener.fx,listener.fz);
         foley?.listener(listener.x,listener.y,listener.z,listener.fx,listener.fz,listener.origin.x,listener.origin.z);updateBeds();}
     ```
8. **Returned object.**
   - `setVolume(value) {if(Number.isFinite(value)){volume=Math.max(0,Math.min(.35,value));sync();}},` → `setVolume(value) {if(Number.isFinite(value)){volume=Math.max(0,Math.min(.35,value));save('aurago.desktop.sysworld.volume',String(volume));sync();}},`
   - Replace the `setListener(x,y,z,fx,fz,ox,oz) {…updateBeds();},` property (two lines) with:
     ```js
         setListener(x,y,z,fx,fz,ox,oz) {listener.x=x;listener.y=y;listener.z=z;listener.fx=fx;listener.fz=fz;listener.origin.x=ox??x;listener.origin.z=oz??z;
           // A silent city does no per-frame work; sync() refreshes everything on unmute.
           if(running())refresh();},
     ```
   - In `setChannel`, before `sync();}` insert `save('aurago.desktop.sysworld.channels',JSON.stringify(levels));`.
   - After the `setMood(value={}){…},` property add:
     ```js
         // Per-frame scene input (phase 5; sysworld-sound-frame.js). Returns at once while silent.
         frame(f){if(!running()||!f)return;frames++;},
         setTier(value){if(!['low','medium','high','ultra'].includes(value)||value===tier)return;tier=value;},
     ```
   - `stats()`: after `rms:Math.sqrt(rms/samples.length),` insert `limiter:compressor?{threshold:LIMITER.threshold,trim:LIMITER.trim,reduction:+(Number(compressor.reduction)||0).toFixed(2)}:null,graph:{nodes:nodes.length,sources:sources.length,tier,frames},`.
   - `dispose()`: `if(context)void context.close().catch(()=>{});` → `if(context&&!options.context&&context.close)void context.close().catch(()=>{});`

### Other edits
- **`sysworld-hud.js`.** In `volume.type='range';volume.min=0;volume.max=35;volume.value=18;volume.hidden=true;`, replace `volume.value=18;` with:
  ```js
  volume.value=(()=>{try{const raw=localStorage.getItem('aurago.desktop.sysworld.volume'),v=Number(raw);return raw!=null&&Number.isFinite(v)?String(Math.round(Math.max(0,Math.min(.35,v))*100)):'18';}catch(_){return '18';}})();
  ```
- **`sysworld-controls.js`.**
  - After `const sound=el('details','sw-world-section');sound.append(el('summary','',L('audio')));` insert:
    ```js
    const channel=key=>{try{const v=JSON.parse(localStorage.getItem('aurago.desktop.sysworld.channels')||'{}')[key];return Number.isFinite(v)?Math.max(0,Math.min(1,v)):1;}catch(_){return 1;}};
    ```
  - In the channel loop, replace `input.value=100;` with `input.value=String(Math.round(channel(key)*100));`.
- **`ui/desktop_sysworld_city_browser_test.go`.** After `        localStorage.setItem('aurago.desktop.sysworld.sound','false');` add the line `        localStorage.removeItem('aurago.desktop.sysworld.volume');localStorage.removeItem('aurago.desktop.sysworld.channels');`.
- **`ui/js/desktop/apps/AGENTS.md`** (`sysworld-audio.js` bullet). Replace the sentence beginning `Do not add a \`DynamicsCompressorNode\`:` (it ends `voice ceiling.`) with: "The master carries a `DynamicsCompressorNode` (`LIMITER`) only together with its fixed trim, which cancels the automatic makeup gain (about +4.3 dB, measured by `scripts/system-world-sound-survey.mjs --calibrate`), and a wave-shaper ceiling at −1 dBFS; without the trim the mix rises above the browser test's 0.05 RMS voice ceiling."

**Run**
- `node scripts/test-system-world-soundscape.mjs` → PASS
- `node scripts/test-system-world-foley.mjs` → PASS
- `node scripts/test-system-world-voice.mjs` → PASS
- `go test ./ui -run '^TestDesktopSysWorld' -count=1` → PASS (markers; the browser tests are skipped without their env)

**Acceptance:**
- The limiter is the only path to the destination.
- Levels persist.
- A muted city schedules nothing per frame.
- Live behaviour is unchanged otherwise.
- The bundle check is now red (expected).

**Commit:** `feat(system-world): master limiter with makeup trim and ceiling, stored sound levels, injectable audio context for offline renders`

---

## Task 2 — `sysworld-zones.js` and its test

**Files:** new `ui/js/desktop/apps/sysworld-zones.js`, new `scripts/test-system-world-zones.mjs`.

**Steps**
- [ ] Write the test. Run → **FAIL** (module missing).
- [ ] Write the module. Run → PASS.
- [ ] Report the printed weights at the fixtures and the field build time.
- [ ] If a fixture's ranking fails because of street trees, report the values. Do not loosen a ranking. The only levers are `ZONE.treeRadius` (8–12) and `ZONE.treeShare` (.5–.8).

### `ui/js/desktop/apps/sysworld-zones.js` (complete)

```js
// Sound zones (System World beauty pass, phase 5). Pure: where the listener stands (harbour, street canyon, green, open
// plaza, height above the roofs), the room it is in (lobbies of sysworld-lobbies.js, waterfront pavilions of
// sysworld-layout.js), the nearest door and its openness, and the surface under a foot. World metres, Y up. Every
// sound layer reads these values; none measures its own distances. Outdoor values are shares (harbour + canyon +
// green + plaza + height = 1); `quay` is the raw harbour proximity; `interior` is 0 or 1 (smoothed by the tracker).
// Green and canyon come from fields sampled every ZONE.cell metres, built once on first use, so zoneWeights,
// doorState and the tracker never allocate. scripts/test-system-world-zones.mjs fixes the weights at known points.
import {streets,interiors,stations,towers} from './sysworld-layout.js';
import {streetProfile} from './sysworld-ground.js';
import {LOBBIES,DOORS,SOLID_TOP,lobbyAt,lobbyGround,podiumAt} from './sysworld-lobbies.js';
import {SURFACES,furnishing} from './sysworld-furnishing.js';
import {roomAt} from './sysworld-exploration.js';

export const ZONE=Object.freeze({quayZ:79,quayHalf:80,harbourNear:6,harbourReach:36,canyonHeight:24,canyonNear:3,canyonReach:24,canyonPair:1.6,
  plotStreet:.6,lawnReach:14,gardenNear:3,gardenReach:12,courtReach:12,treeRadius:10,treeCount:6,treeShare:.7,heightFrom:14,heightTo:50,
  pavilionTop:12,cell:4,outdoorTau:.8,interiorTau:.3,leakReach:12});
export const TOWER_HEIGHT=38.79;
// The seven districts of sysworld-scene.js `districts` (x, z, height, radius; checked by the test) and the data towers.
export const BUILDINGS=Object.freeze([[0,-12,87,13],[-43,-53,24,18],[43,-10,34,14],[43,36,23,15],[-43,-10,37,14],[-43,36,28,17],[0,36,22,6],
  ...towers.map(t=>[t.x,t.z,TOWER_HEIGHT*t.scale,7.5])].map(([x,z,height,radius])=>Object.freeze({x,z,height,radius})));
export const GARDENS=Object.freeze([[-79,68],[-79,-67],[55,4],[8,40],[45,-23]]);
export const SURFACE_KINDS=Object.freeze(['road','pavement','kerb','stairs','quay','lobby','grating','garden']);
// Rooms: tone (sysworld-sound-rooms.js ROOM_TONES), reverb space (sysworld-sound-space.js ROOM_SPACES), open to the sky.
export const ROOMS=Object.freeze({
  'lobby:agent':{tone:'agent',space:'lobby',open:false},'lobby:memory':{tone:'archive',space:'hall',open:false},
  'lobby:integrations-west':{tone:'gate',space:'small',open:false},'lobby:integrations-east':{tone:'gate',space:'small',open:false},
  'lobby:missions':{tone:'departure',space:'hall',open:false},'lobby:infra':{tone:'machine',space:'machine',open:false},
  'lobby:graph':{tone:'court',space:'lobby',open:true},
  'pavilion:agent':{tone:'pavilion',space:'small',open:false},'pavilion:memory':{tone:'pavilion',space:'small',open:false},
  'pavilion:missions':{tone:'pavilion',space:'small',open:false},
});
const LOBBY_ROOM=new Map(LOBBIES.map(l=>[l.id,'lobby:'+l.id])),PAVILION_ROOM=new Map(interiors.map(r=>[r.id,'pavilion:'+r.id]));
const COURT=LOBBIES.find(l=>l.open).circle,LAWNS=SURFACES.filter(s=>s.kind==='lawn'),BRIDGE=[-82,28,-75,52];
const DOOR_LIST=Object.freeze([...DOORS.map(d=>({key:d.key,room:'lobby:'+d.lobby,x:d.x,z:d.z})),...interiors.map(r=>({key:'pavilion:'+r.id,room:'pavilion:'+r.id,x:r.x,z:r.doorZ}))]);
const ROOM_DOORS=new Map(Object.keys(ROOMS).map(room=>[room,DOOR_LIST.filter(d=>d.room===room)]));
const SHARES=['harbour','canyon','green','plaza','height'];
const clamp01=v=>v<0?0:v>1?1:v;
// Stereo position of a source at (dx, dz) from the camera facing (fx, fz): the formula of sysworld-audio.js reach().
export function bearingPan(dx,dz,fx,fz){const d=Math.hypot(dx,dz);return d<1e-6?0:Math.max(-1,Math.min(1,(dx*-fz+dz*fx)/Math.max(1,d)));}
export function interiorAt(x,y,z){
  if(y<SOLID_TOP){const l=lobbyAt(x,z);if(l)return LOBBY_ROOM.get(l.id);}
  if(y<ZONE.pavilionTop){const r=roomAt(x,z);if(r)return PAVILION_ROOM.get(r.id);}
  return null;
}
function harbourRaw(x,z){const dx=Math.max(0,Math.abs(x)-ZONE.quayHalf),dz=Math.max(0,ZONE.quayZ-z),v=clamp01(1-(Math.hypot(dx,dz)-ZONE.harbourNear)/ZONE.harbourReach);return v*Math.sqrt(v);}
function onLawn(x,z){for(let i=0;i<LAWNS.length;i++){const s=LAWNS[i];if(x>=s.x0&&x<=s.x1&&z>=s.z0&&z<=s.z1)return true;}return false;}
function streetShare(x,z){if(streetProfile(x,z)||lobbyGround(x,z)!==null)return 1;return z>streets.quayZ||onLawn(x,z)?0:ZONE.plotStreet;}
let treeCache=null;const trees=()=>treeCache||(treeCache=furnishing().filter(f=>f.asset.startsWith('tree-')));
// Exact values (the fields sample these): lawns, gardens, the garden court and street-tree density; tall buildings on
// both sides of a street.
export function greenAt(x,z){
  let g=0;
  for(const s of LAWNS){const dx=Math.max(s.x0-x,0,x-s.x1),dz=Math.max(s.z0-z,0,z-s.z1);g=Math.max(g,clamp01(1-Math.hypot(dx,dz)/ZONE.lawnReach));}
  for(const [gx,gz]of GARDENS)g=Math.max(g,clamp01(1-Math.max(0,Math.hypot(x-gx,z-gz)-ZONE.gardenNear)/ZONE.gardenReach));
  g=Math.max(g,clamp01(1-Math.max(0,Math.hypot(x-COURT.x,z-COURT.z)-COURT.radius)/ZONE.courtReach));
  let n=0;for(const t of trees())if(Math.hypot(t.x-x,t.z-z)<ZONE.treeRadius)n++;
  return Math.max(g,clamp01(n/ZONE.treeCount)*ZONE.treeShare);
}
export function canyonAt(x,z){
  let e1=0,e2=0;
  for(const b of BUILDINGS){const edge=Math.max(0,Math.hypot(x-b.x,z-b.z)-b.radius),e=clamp01(b.height/ZONE.canyonHeight)*clamp01(1-Math.max(0,edge-ZONE.canyonNear)/ZONE.canyonReach);
    if(e>e1){e2=e1;e1=e;}else if(e>e2)e2=e;}
  return clamp01((e1+e2)/ZONE.canyonPair)*streetShare(x,z);
}
const GX0=-120,GZ0=-130,GNX=61,GNZ=63;let fields=null;
function buildFields(){const green=new Float32Array(GNX*GNZ),canyon=new Float32Array(GNX*GNZ);
  for(let j=0;j<GNZ;j++)for(let i=0;i<GNX;i++){const x=GX0+i*ZONE.cell,z=GZ0+j*ZONE.cell;green[j*GNX+i]=greenAt(x,z);canyon[j*GNX+i]=canyonAt(x,z);}
  return fields={green,canyon};}
function sample(field,x,z){const fx=(x-GX0)/ZONE.cell,fz=(z-GZ0)/ZONE.cell;if(!(fx>=0&&fz>=0&&fx<GNX-1&&fz<GNZ-1))return 0;
  const i=Math.floor(fx),j=Math.floor(fz),u=fx-i,v=fz-j,k=j*GNX+i;return (field[k]*(1-u)+field[k+1]*u)*(1-v)+(field[k+GNX]*(1-u)+field[k+GNX+1]*u)*v;}
// Listener at (x, y, z) (camera), resting on (ox, oz) (the acoustic origin); writes into `out`.
export function zoneWeights(x,y,z,ox=x,oz=z,out={}){
  const f=fields||buildFields(),h=clamp01((y-ZONE.heightFrom)/(ZONE.heightTo-ZONE.heightFrom)),quay=harbourRaw(ox,oz);
  let harbour=quay,canyon=sample(f.canyon,ox,oz),green=sample(f.green,ox,oz),sum=harbour+canyon+green;
  if(sum>1){harbour/=sum;canyon/=sum;green/=sum;sum=1;}
  const ground=1-h;out.harbour=harbour*ground;out.canyon=canyon*ground;out.green=green*ground;out.plaza=(1-sum)*ground;out.height=h;out.quay=quay;
  const room=interiorAt(x,y,z),r=room?ROOMS[room]:null;out.room=room;out.interior=room?1:0;out.open=r?r.open:false;out.space=r?r.space:null;out.tone=r?r.tone:null;
  out.x=ox;out.z=oz;return out;
}
// Openness of the listener's room (largest of its doors) or, outside, of the nearest door within ZONE.leakReach.
// valueOf(key) -> 0..1 for 'lobby:<id>:<n>' and 'pavilion:<id>' keys.
export function doorState(x,y,z,valueOf,out={}){
  out.open=0;out.leakRoom=null;out.leakDistance=Infinity;out.leakX=0;out.leakZ=0;
  const room=interiorAt(x,y,z);out.room=room;if(typeof valueOf!=='function')return out;
  if(room){const doors=ROOM_DOORS.get(room);for(let i=0;i<doors.length;i++)out.open=Math.max(out.open,clamp01(Number(valueOf(doors[i].key))||0));return out;}
  if(y>=SOLID_TOP)return out;
  let best=null;for(let i=0;i<DOOR_LIST.length;i++){const d=DOOR_LIST[i],dist=Math.hypot(x-d.x,z-d.z);if(dist<ZONE.leakReach&&dist<out.leakDistance){out.leakDistance=dist;best=d;}}
  if(best){out.leakRoom=best.room;out.leakX=best.x;out.leakZ=best.z;out.open=clamp01(Number(valueOf(best.key))||0);}
  return out;
}
// The surface under a foot at floor height y (called once per footfall).
export function surfaceAt(x,y,z){
  if(y>40)return 'grating';
  if(y<SOLID_TOP){const l=lobbyAt(x,z);if(l)return l.id==='infra'?'grating':l.open?'garden':'lobby';}
  if(roomAt(x,z))return 'lobby';
  if(x>=BRIDGE[0]&&x<=BRIDGE[2]&&z>=BRIDGE[1]&&z<=BRIDGE[3])return 'grating';
  if(lobbyGround(x,z)!==null)return podiumAt(x,z)?'lobby':'stairs';
  for(const s of stations){const turned=!!s.angle&&Math.abs(s.angle)!==Math.PI;if(Math.abs(x-s.platformX)<(turned?2:4.5)&&Math.abs(z-s.platformZ)<(turned?4.5:2))return 'pavement';}
  const p=streetProfile(x,z);if(p)return p.kind==='road'?'road':p.kind==='kerb'?'kerb':'pavement';
  if(z>=streets.quayZ&&z<=80.5&&Math.abs(x)<=80)return 'quay';
  if(onLawn(x,z))return 'garden';
  for(const [gx,gz]of GARDENS)if(Math.hypot(x-gx,z-gz)<2.2)return 'garden';
  return 'pavement';
}
// Smoothed weights for the reverb sends, the glass and the event table. Shares settle with ZONE.outdoorTau, the room
// with ZONE.interiorTau; the room id stays until the interior weight has faded. Also: door openness, the leak of the
// nearest open door outside (tone, space, pan). `update` returns the same object every time.
export function createZoneTracker(){
  const w={harbour:0,canyon:0,green:0,plaza:1,height:0,quay:0,interior:0,room:null,space:null,tone:null,open:false,door:0,leak:0,leakRoom:null,leakTone:null,leakSpace:null,leakPan:0};
  let first=true;
  return {weights:w,
    update(raw,door,dt,listener){
      const ko=first?1:1-Math.exp(-Math.max(0,dt)/ZONE.outdoorTau),ki=first?1:1-Math.exp(-Math.max(0,dt)/ZONE.interiorTau);first=false;
      for(let i=0;i<SHARES.length;i++){const k=SHARES[i];w[k]+=(raw[k]-w[k])*ko;}
      w.quay+=(raw.quay-w.quay)*ko;w.interior+=(raw.interior-w.interior)*ki;
      if(raw.room){w.room=raw.room;w.space=raw.space;w.tone=raw.tone;w.open=raw.open;}else if(w.interior<.02){w.room=null;w.space=null;w.tone=null;w.open=false;}
      w.door=door?door.open:0;
      const leak=door&&door.leakRoom&&!raw.room?door.open*clamp01(1-door.leakDistance/ZONE.leakReach):0,r=leak>0?ROOMS[door.leakRoom]:null;
      w.leak=r?leak:0;w.leakRoom=r?door.leakRoom:null;w.leakTone=r?r.tone:null;w.leakSpace=r?r.space:null;
      w.leakPan=r&&listener?bearingPan(door.leakX-listener.x,door.leakZ-listener.z,listener.fx,listener.fz):0;
      return w;
    },
    reset(){first=true;},
  };
}
```

### `scripts/test-system-world-zones.mjs` (complete)

```js
// Phase 5 zones: the registry against the scene and kit, weights at known points, rooms, doors, surfaces, the tracker.
import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import * as Z from '../ui/js/desktop/apps/sysworld-zones.js';
import {districts} from '../ui/js/desktop/apps/sysworld-scene.js';
import {LOBBIES} from '../ui/js/desktop/apps/sysworld-lobbies.js';
import {interiors} from '../ui/js/desktop/apps/sysworld-layout.js';
import {streetProfile} from '../ui/js/desktop/apps/sysworld-ground.js';

districts.forEach((d,i)=>{const b=Z.BUILDINGS[i];assert.deepEqual([b.x,b.z,b.height,b.radius],[d.x,d.z,d.height,d.radius],'building '+d.id);});
const v3=JSON.parse(await fs.readFile('ui/3d/system-world/v3/manifest.json','utf8'));
assert.equal(Z.TOWER_HEIGHT,v3.assets.find(a=>a.id==='data-tower-a').lods[0].bounds.max[1],'TOWER_HEIGHT mirrors data-tower-a');
for(const l of LOBBIES)assert.ok(Z.ROOMS['lobby:'+l.id],'room for lobby '+l.id);
for(const r of interiors)assert.ok(Z.ROOMS['pavilion:'+r.id],'room for pavilion '+r.id);
assert.equal(Object.keys(Z.ROOMS).length,LOBBIES.length+interiors.length);assert.equal(Object.values(Z.ROOMS).filter(r=>r.open).length,1,'only the garden court is open');

const t0=performance.now(),w=(x,y,z,ox,oz)=>Z.zoneWeights(x,y,z,ox,oz,{}),first=w(0,2.4,0),build=performance.now()-t0;
const KEYS=['harbour','canyon','green','plaza','height'],top=o=>KEYS.reduce((a,k)=>o[k]>o[a]?k:a,'harbour'),sum=o=>KEYS.reduce((s,k)=>s+o[k],0),fmt=o=>KEYS.map(k=>k+' '+o[k].toFixed(2)).join(', ');
const quay=w(-20,2.48,77),street=w(-13.5,2.4,-12),park=w(-44,2.4,-88),plaza=w(79,2.4,-55),roof=w(0,74.6,-12),overview=w(120,110,183,0,-12);
assert.equal(top(quay),'harbour',fmt(quay));assert.ok(quay.harbour>=.6&&quay.quay===1&&quay.room===null,fmt(quay));
assert.equal(top(street),'canyon',fmt(street));assert.ok(street.canyon>=.5&&street.harbour<.05,fmt(street));
assert.equal(top(park),'green',fmt(park));assert.ok(park.green>=.8,fmt(park));
assert.equal(top(plaza),'plaza',fmt(plaza));assert.ok(plaza.plaza>=.5,fmt(plaza));
assert.ok(roof.height>=.95,'sky deck '+fmt(roof));assert.equal(overview.height,1);assert.equal(first.room,null);
const halls={agent:[0,-12],memory:[-43,-11],'integrations-west':[35,-10],'integrations-east':[51,-10],missions:[43,35],infra:[-43,-56],graph:[-43,36]};
for(const [id,[x,z]]of Object.entries(halls)){const o=w(x,3.28,z);assert.equal(o.room,'lobby:'+id,id);assert.equal(o.interior,1);assert.equal(o.open,id==='graph');assert.equal(o.space,Z.ROOMS['lobby:'+id].space);}
for(const r of interiors)assert.equal(w(r.x,2.56,r.z).room,'pavilion:'+r.id);
assert.equal(w(0,12,-12).room,null,'above the lobby band is outdoors');
let seed=7;const rnd=()=>(seed=(Math.imul(seed,1664525)+1013904223)>>>0)/4294967296;
for(let i=0;i<3000;i++){const o=w((rnd()-.5)*260,rnd()*120,(rnd()-.5)*260);assert.ok(Math.abs(sum(o)-1)<1e-9,'shares sum to 1: '+fmt(o));for(const k of KEYS)assert.ok(o[k]>=-1e-12&&o[k]<=1+1e-12);}
// Doors.
const value=key=>key==='lobby:agent:0'?.8:key==='pavilion:memory'?.5:0;
const inside=Z.doorState(0,3.28,-12,value,{});assert.deepEqual([inside.room,inside.open,inside.leakRoom],['lobby:agent',.8,null]);
const porch=Z.doorState(0,2.4,-1,value,{});assert.deepEqual([porch.room,porch.leakRoom,porch.open],[null,'lobby:agent',.8]);assert.ok(porch.leakDistance<5);
assert.equal(Z.doorState(0,2.4,30,value,{}).leakRoom,null);const pav=Z.doorState(-43,2.4,64,value,{});assert.deepEqual([pav.leakRoom,pav.open],['pavilion:memory',.5]);
// Surfaces.
const S=Z.surfaceAt,seen=new Set();const at=(x,y,z,want)=>{assert.equal(S(x,y,z),want,[x,y,z]+' should be '+want);seen.add(want);};
at(-18,-.14,-50,'road');at(-14.63,0,-50,'kerb');at(-13.5,0,-50,'pavement');at(0,.5,.3,'stairs');at(0,.88,-1,'lobby');at(0,.88,-12,'lobby');
at(-43,.88,-56,'grating');at(-43,.88,36,'garden');at(-44,0,-88,'garden');at(-20,.08,77,'quay');at(0,.16,73,'lobby');at(0,72.2,-16,'grating');at(-78.5,2,40,'grating');
assert.deepEqual([...seen].sort(),[...Z.SURFACE_KINDS].sort(),'every surface kind is reachable');
for(const z of[-50,-24])for(let x=-24.5;x<=-11.5;x+=.05){const p=streetProfile(x,z);if(!p)continue;assert.equal(S(x,0,z),p.kind==='road'?'road':p.kind==='kerb'?'kerb':'pavement','street profile at '+x.toFixed(2)+','+z);}
// Tracker.
const tr=Z.createZoneTracker(),listener={x:-20,z:77,fx:0,fz:-1};let raw=w(-20,2.48,77);
assert.equal(tr.update(raw,null,0,listener),tr.weights);assert.ok(Math.abs(tr.weights.harbour-raw.harbour)<1e-12,'the first update snaps');
const raw2=w(-13.5,2.4,-12);let prev=tr.weights.canyon;
for(let i=0;i<180;i++){tr.update(raw2,null,1/60,listener);assert.ok(tr.weights.canyon>=prev-1e-12);prev=tr.weights.canyon;assert.ok(Math.abs(sum(tr.weights)-1)<1e-9);}
assert.ok(Math.abs(tr.weights.canyon-raw2.canyon)<=.05*Math.abs(raw2.canyon-raw.canyon)+1e-9,'95 % after three seconds');
const hall=w(0,3.28,-12);for(let i=0;i<60;i++)tr.update(hall,Z.doorState(0,3.28,-12,()=>0,{}),1/60,listener);assert.ok(tr.weights.interior>.95&&tr.weights.room==='lobby:agent');
for(let i=0;i<30;i++)tr.update(raw2,null,1/60,listener);assert.equal(tr.weights.room,'lobby:agent','the room id stays while the interior fades');
for(let i=0;i<120;i++)tr.update(raw2,null,1/60,listener);assert.equal(tr.weights.room,null);
const out={x:3,z:-1,fx:0,fz:-1};tr.update(w(3,2.4,-1),Z.doorState(3,2.4,-1,()=>1,{}),1/60,out);
assert.ok(tr.weights.leak>.5&&tr.weights.leakTone==='agent'&&tr.weights.leakSpace==='lobby'&&tr.weights.leakPan<0,'an open door on the left leaks its room tone');
console.log(`System World zones: fields built in ${build.toFixed(0)} ms; quay ${fmt(quay)}; street ${fmt(street)}; park ${fmt(park)}; plaza ${fmt(plaza)}; rooms, doors, surfaces and tracker verified.`);
```

**Run:** `node scripts/test-system-world-zones.mjs` → PASS (report the line). `node --check ui/js/desktop/apps/sysworld-zones.js`.

**Acceptance:** fixtures and rankings hold; no runtime file imports the module yet.

**Commit:** `feat(system-world): pure sound zones (harbour, canyon, green, plaza, height, rooms), door state and surfaces with their test`

---

## Task 3 — Sound meter, offline sound survey, calibration and baseline

**Files:** new
- `scripts/system-world-sound-meter.mjs`
- `scripts/test-system-world-sound-meter.mjs`
- `scripts/system-world-sound-survey.mjs`
- `scripts/system-world-sound-survey.page.mjs`
- `scripts/system-world-sound-survey.situations.json`

**Steps**
- [ ] Write the meter test. Run → **FAIL**. Write the meter. Run → PASS.
- [ ] Write the survey, the page and the situations (below).
- [ ] **Calibrate:** `node scripts/system-world-sound-survey.mjs --label calibration --only quay-day --calibrate`. Then read `reports/system-world-sound/calibration/calibration.json`:
  - If `pass` is false and `suggestedTrim` lies within .55–.68, set `LIMITER.trim` in `sysworld-audio.js` to `suggestedTrim` and re-run until `pass` is true.
  - Outside that range: stop and report.
- [ ] **Tram timing:**
  - Run `--label probe --only tram-corner`.
  - Report `stats.soundscape.listener` and, once Task 11 exists, the tram distances. Before Task 11, log the scripted tram position from the page (temporary `log`, removed afterwards).
  - Adjust the situation's `"u"` so the closest approach falls between 8 s and 20 s.
- [ ] **Baseline** (on Task 1's commit): `node scripts/system-world-sound-survey.mjs --label baseline --timeout 1800`. Expect 15 WAVs and 0 page errors. Report the summary table (LUFS, dBTP, real-time factor).
- [ ] If renders take longer than about 3× real time, set `STEP` to 20 and say so. If `stats.soundscape.transient` sits at 12 for the whole of a situation, report it (could-not-verify 2).

### `scripts/system-world-sound-meter.mjs` (complete)

```js
// Loudness meter for the System World sound survey (phase 5): WAV read/write (PCM 16/24, 32-bit float), integrated
// loudness per ITU-R BS.1770-4 (K-weighting at 48 kHz, 400 ms blocks, 75 % overlap, -70 LUFS absolute and -10 LU
// relative gates), true peak by 4x oversampling (49-tap windowed sinc), sample peak and RMS. Pure Node, no dependencies.
const PRE={b:[1.53512485958697,-2.69169618940638,1.19839281085285],a:[1,-1.69065929318241,.73248077421585]};
const RLB={b:[1,-2,1],a:[1,-1.99004745483398,.99007225036621]};
export function readWav(bytes){
  const v=new DataView(bytes.buffer,bytes.byteOffset,bytes.byteLength),tag=o=>String.fromCharCode(v.getUint8(o),v.getUint8(o+1),v.getUint8(o+2),v.getUint8(o+3));
  if(tag(0)!=='RIFF'||tag(8)!=='WAVE')throw Error('not a WAV file');
  let o=12,format=0,channels=0,rate=0,bits=0,data=null;
  while(o+8<=v.byteLength){const id=tag(o),size=v.getUint32(o+4,true),body=o+8;
    if(id==='fmt '){format=v.getUint16(body,true);channels=v.getUint16(body+2,true);rate=v.getUint32(body+4,true);bits=v.getUint16(body+14,true);if(format===0xfffe)format=v.getUint16(body+24,true);}
    else if(id==='data')data={offset:body,size:Math.min(size,v.byteLength-body)};
    o=body+size+(size&1);}
  if(!data||!channels)throw Error('WAV without fmt or data');
  if(!(format===3&&bits===32)&&!(format===1&&(bits===16||bits===24)))throw Error('unsupported WAV format '+format+'/'+bits);
  const step=bits/8,frames=Math.floor(data.size/(step*channels)),out=Array.from({length:channels},()=>new Float32Array(frames));
  for(let i=0;i<frames;i++)for(let c=0;c<channels;c++){const p=data.offset+(i*channels+c)*step;
    out[c][i]=format===3?v.getFloat32(p,true):bits===16?v.getInt16(p,true)/32768:(v.getUint8(p)|v.getUint8(p+1)<<8|v.getInt8(p+2)<<16)/8388608;}
  return {sampleRate:rate,channels:out};
}
export function writeWav(channels,sampleRate){
  const ch=channels.length,len=channels[0].length,bytes=44+len*ch*4,v=new DataView(new ArrayBuffer(bytes)),w=(o,s)=>{for(let i=0;i<4;i++)v.setUint8(o+i,s.charCodeAt(i));};
  w(0,'RIFF');v.setUint32(4,bytes-8,true);w(8,'WAVE');w(12,'fmt ');v.setUint32(16,16,true);v.setUint16(20,3,true);v.setUint16(22,ch,true);v.setUint32(24,sampleRate,true);
  v.setUint32(28,sampleRate*ch*4,true);v.setUint16(32,ch*4,true);v.setUint16(34,32,true);w(36,'data');v.setUint32(40,len*ch*4,true);
  let o=44;for(let i=0;i<len;i++)for(let c=0;c<ch;c++){v.setFloat32(o,channels[c][i],true);o+=4;}
  return Buffer.from(v.buffer);
}
function biquad(x,{b,a}){const y=new Float64Array(x.length);let x1=0,x2=0,y1=0,y2=0;
  for(let i=0;i<x.length;i++){const s=x[i],o=b[0]*s+b[1]*x1+b[2]*x2-a[1]*y1-a[2]*y2;x2=x1;x1=s;y2=y1;y1=o;y[i]=o;}return y;}
export function integratedLoudness(channels,sampleRate){
  if(sampleRate!==48000)throw Error('meter: the K-weighting coefficients are for 48 kHz');
  const block=Math.round(.4*sampleRate),hop=Math.round(.1*sampleRate),len=channels[0].length;if(len<block)return -Infinity;
  const sums=channels.map(c=>{const k=biquad(biquad(c,PRE),RLB),s=new Float64Array(len+1);for(let i=0;i<len;i++)s[i+1]=s[i]+k[i]*k[i];return s;});
  const z=[];for(let s=0;s+block<=len;s+=hop){let v=0;for(const p of sums)v+=(p[s+block]-p[s])/block;z.push(v);}
  const L=v=>-.691+10*Math.log10(v),mean=a=>a.reduce((x,y)=>x+y,0)/a.length,abs=z.filter(v=>v>0&&L(v)>-70);if(!abs.length)return -Infinity;
  const rel=L(mean(abs))-10,gated=abs.filter(v=>L(v)>rel);return L(mean(gated));
}
const TAPS=49,PHASES=4,FIR=(()=>{const h=new Float64Array(TAPS),c=(TAPS-1)/2;
  for(let i=0;i<TAPS;i++){const x=(i-c)/PHASES,s=x===0?1:Math.sin(Math.PI*x)/(Math.PI*x);h[i]=s*(.5-.5*Math.cos(2*Math.PI*i/(TAPS-1)));}
  const out=[];for(let p=0;p<PHASES;p++){const taps=[];for(let i=p;i<TAPS;i+=PHASES)taps.push(h[i]);const sum=taps.reduce((a,b)=>a+b,0);out.push(Float64Array.from(taps,v=>v/sum));}return out;})();
export function truePeak(channels){let peak=0;
  for(const x of channels)for(const taps of FIR)for(let n=0;n<x.length;n++){let y=0;for(let j=0;j<taps.length&&j<=n;j++)y+=x[n-j]*taps[j];const a=Math.abs(y);if(a>peak)peak=a;}
  return peak;}
export function measure(channels,sampleRate,{skip=0}={}){
  const start=Math.min(channels[0].length,Math.round(skip*sampleRate)),cut=channels.map(c=>c.subarray(start));
  let sample=0,power=0;for(const c of cut)for(let i=0;i<c.length;i++){const a=Math.abs(c[i]);if(a>sample)sample=a;power+=c[i]*c[i];}
  const n=cut.reduce((s,c)=>s+c.length,0),dB=v=>v>0?20*Math.log10(v):-Infinity;
  return {lufs:integratedLoudness(cut,sampleRate),truePeakDb:dB(Math.max(sample,truePeak(cut))),samplePeakDb:dB(sample),rmsDb:dB(Math.sqrt(power/Math.max(1,n)))};
}
```

### `scripts/test-system-world-sound-meter.mjs` (complete)

```js
// Phase 5 sound meter: BS.1770 calibration, gating, true peak, the skip window and a WAV round trip.
import assert from 'node:assert/strict';
import {readWav,writeWav,measure,integratedLoudness} from './system-world-sound-meter.mjs';
const R=48000,sine=(f,a,s,phase=0)=>Float32Array.from({length:Math.round(R*s)},(_,i)=>a*Math.sin(2*Math.PI*f*i/R+phase));
const s=sine(997,.1,10);let m=measure([s,s],R);
assert.ok(Math.abs(m.lufs+20)<.1,'a 997 Hz sine at 0.1 in both channels reads -20 LUFS: '+m.lufs);assert.ok(Math.abs(m.samplePeakDb+20)<.01);assert.ok(Math.abs(m.rmsDb+23.01)<.02);
const q=sine(12000,.5,2,Math.PI/4);m=measure([q,q],R);
assert.ok(Math.abs(m.samplePeakDb+9.03)<.05,'sample peak '+m.samplePeakDb);assert.ok(Math.abs(m.truePeakDb+6.02)<.3,'true peak '+m.truePeakDb);
const g=new Float32Array(R*10);g.set(sine(997,.1,5));assert.ok(Math.abs(integratedLoudness([g,g],R)+20)<.2,'silence is gated out');
assert.equal(integratedLoudness([new Float32Array(R*3),new Float32Array(R*3)],R),-Infinity);
const ramp=new Float32Array(R*4);ramp.set(sine(997,.9,1));ramp.set(sine(997,.1,3),R);assert.ok(Math.abs(measure([ramp,ramp],R,{skip:1}).lufs+20)<.1,'skip drops the start');
const back=readWav(writeWav([s.subarray(0,1000),q.subarray(0,1000)],R));assert.equal(back.sampleRate,R);assert.deepEqual(back.channels[0],s.subarray(0,1000));assert.deepEqual(back.channels[1],q.subarray(0,1000));
console.log('System World sound meter: BS.1770 calibration, gating, true peak, skip window and WAV round trip verified');
```

### `scripts/system-world-sound-survey.mjs` (complete)

```js
#!/usr/bin/env node
/*
 * System World sound survey (phase 5). Renders the real soundscape (ui/js/desktop/apps/sysworld-audio.js with all its
 * layers) for named listener situations through an OfflineAudioContext in headless Chrome and writes
 * reports/system-world-sound/<label>/<situation>.wav (32-bit float, stereo, 48 kHz) and report.json with integrated
 * loudness (BS.1770, gated), true peak (4x oversampled), sample peak and RMS per file, the real-time factor of each
 * render, the engine's stats, and the plan's checks.
 *
 *   node scripts/system-world-sound-survey.mjs --label <name> [--only a,b] [--situations <file>] [--tier low|medium|high|ultra]
 *        [--baseline <label>] [--calibrate] [--strict] [--timeout 900] [--skip 2]
 *   --calibrate  renders a 1 kHz sine at -30 dBFS and at +6 dBFS through createLimiter() into calibration.json: the gain
 *                below the threshold must be 0 +- 0.25 dB, the peak above it under LIMITER.ceiling; suggestedTrim is
 *                the trim that would cancel the measured makeup gain.
 *   --tier       overrides every situation's tier (budget and real-time comparisons).
 *   --baseline   adds each situation's loudness change against reports/system-world-sound/<label>/report.json.
 *   --strict     exits 1 when a check fails: true peak above -1 dBTP; street-day and street-night-rain more than 2 LU
 *                apart; any situation more than 3 LU over the baseline; a shut lobby less than 3 LU under street-day.
 *   --skip       seconds dropped at the start of every file before measuring (the master fades in), default 2.
 *
 * The page (scripts/system-world-sound-survey.page.mjs) is bundled in memory with esbuild; nothing under ui/ changes.
 * Situation fields: name, seconds (30), seed, tier, volume (.18), listener {x,z,fx,fz[,y]} (y defaults to the ground
 * height + 2.4), origin [x,z] (acoustic origin; default the listener), walk [[x,z],...] with walkSpeed (m/s; visitor
 * steps every 1.8 m), mood {day,evening,busy,weather}, inside (the experience's indoor flag), door (openness returned
 * for every door key), seaTime and windOffset (clock starts), trams [{u,speed}] on the real route, walkers
 * [{id,from,to,speed,carrying,style}] (style "glide" = hover robot), companion (true: the quadruped trots its loop).
 */
import {build} from 'esbuild';
import fs from 'node:fs/promises';
import {existsSync} from 'node:fs';
import http from 'node:http';
import os from 'node:os';
import path from 'node:path';
import {spawn,spawnSync} from 'node:child_process';
import {fileURLToPath} from 'node:url';
import {readWav,measure} from './system-world-sound-meter.mjs';

const here=path.dirname(fileURLToPath(import.meta.url)),root=path.resolve(here,'..');
function parseArgs(argv){
  const o={label:null,only:null,situations:path.join(here,'system-world-sound-survey.situations.json'),tier:null,baseline:null,calibrate:false,strict:false,timeout:900,skip:2};
  for(let i=0;i<argv.length;i++){const a=argv[i],next=()=>{if(i+1>=argv.length)throw Error('Missing value for '+a);return argv[++i];};
    if(a==='--label')o.label=next();else if(a==='--only')o.only=next().split(',').map(s=>s.trim()).filter(Boolean);else if(a==='--situations')o.situations=next();
    else if(a==='--tier')o.tier=next();else if(a==='--baseline')o.baseline=next();else if(a==='--calibrate')o.calibrate=true;else if(a==='--strict')o.strict=true;
    else if(a==='--timeout')o.timeout=Number(next());else if(a==='--skip')o.skip=Number(next());else throw Error('Unknown argument '+a);}
  if(!o.label||!/^[a-z0-9-]+$/.test(o.label))throw Error('--label <name> ([a-z0-9-]) is required');
  if(o.tier&&!['low','medium','high','ultra'].includes(o.tier))throw Error('--tier must be low|medium|high|ultra');
  if(!(o.timeout>0)||!(o.skip>=0))throw Error('bad --timeout or --skip');return o;
}
function findChrome(){const found=[process.env.CHROME_PATH,'C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe','C:\\Program Files (x86)\\Google\\Chrome\\Application\\chrome.exe',
  'C:\\Program Files (x86)\\Microsoft\\Edge\\Application\\msedge.exe','C:\\Program Files\\Microsoft\\Edge\\Application\\msedge.exe'].find(p=>p&&existsSync(p));if(!found)throw Error('No Chrome/Edge found; set CHROME_PATH');return found;}
function readBody(req,limit=96*1024*1024){return new Promise((resolve,reject)=>{const chunks=[];let size=0;
  req.on('data',c=>{size+=c.length;if(size>limit){reject(Error('body too large'));req.destroy();}else chunks.push(c);});req.on('end',()=>resolve(Buffer.concat(chunks)));req.on('error',reject);});}
const db=v=>Number.isFinite(v)?+v.toFixed(2):null;
function checks(rows,baseline){
  const by=new Map(rows.map(r=>[r.name,r])),out={},worst=rows.reduce((m,r)=>Math.max(m,r.truePeakDb??-Infinity),-Infinity);
  out.truePeak={pass:worst<=-1,worst:db(worst),limit:-1};
  const day=by.get('street-day'),rain=by.get('street-night-rain'),shut=by.get('lobby-agent-shut');
  if(day&&rain){const d=Math.abs(day.lufs-rain.lufs);out.dayNight={pass:d<=2,delta:db(d),limit:2};}
  if(day&&shut){const d=day.lufs-shut.lufs;out.lobby={pass:d>=3,below:db(d),limit:3};}
  if(baseline){const over=[];for(const r of rows){const b=baseline.situations?.find(s=>s.name===r.name);if(b&&Number.isFinite(b.lufs)&&Number.isFinite(r.lufs)){r.deltaLu=db(r.lufs-b.lufs);if(r.lufs-b.lufs>3)over.push(r.name);}}out.baseline={pass:!over.length,over,limit:3};}
  return out;
}
async function main(){
  const o=parseArgs(process.argv.slice(2)),outDir=path.join(root,'reports','system-world-sound',o.label);
  let situations=JSON.parse(await fs.readFile(path.resolve(root,o.situations),'utf8'));if(!Array.isArray(situations))throw Error('Situations file must be a JSON array');
  if(o.only){const missing=o.only.filter(n=>!situations.some(s=>s.name===n));if(missing.length)throw Error('Unknown situation(s): '+missing.join(', '));situations=situations.filter(s=>o.only.includes(s.name));}
  if(o.tier)situations=situations.map(s=>({...s,tier:o.tier}));
  await fs.mkdir(outDir,{recursive:true});
  console.log('building sound page bundle ...');
  const result=await build({entryPoints:[path.join(here,'system-world-sound-survey.page.mjs')],bundle:true,format:'esm',target:'es2022',write:false,absWorkingDir:root,logLevel:'warning',outdir:path.join(root,'.sound-survey-unused')});
  const page=Buffer.from(result.outputFiles.find(f=>f.path.endsWith('.js')).contents),config=Buffer.from(JSON.stringify({situations,calibrate:o.calibrate}));
  const html='<!doctype html><meta charset="utf-8"><title>System World sound survey</title><script type="module" src="/__sound/page.js"></script>';
  const stats=new Map();let report=null,calibration=null,finish;const done=new Promise(r=>{finish=r;});
  const server=http.createServer(async(req,res)=>{try{
    const url=new URL(req.url,'http://127.0.0.1'),send=(c,t,b)=>{res.writeHead(c,{'Content-Type':t,'Cache-Control':'no-store'});res.end(b);};
    if(req.method==='POST'){const body=await readBody(req),name=(url.searchParams.get('name')||'').toLowerCase().replace(/[^a-z0-9-]/g,'-').slice(0,80);
      if(url.pathname==='/__sound/wav'&&name){await fs.writeFile(path.join(outDir,name+'.wav'),body);console.log('saved '+name+'.wav ('+Math.round(body.length/1024)+' KB)');return send(200,'text/plain','ok');}
      if(url.pathname==='/__sound/stats'&&name){stats.set(name,JSON.parse(body.toString('utf8')));return send(200,'text/plain','ok');}
      if(url.pathname==='/__sound/calibration'){calibration=JSON.parse(body.toString('utf8'));await fs.writeFile(path.join(outDir,'calibration.json'),JSON.stringify(calibration,null,2));return send(200,'text/plain','ok');}
      if(url.pathname==='/__sound/report'){report=JSON.parse(body.toString('utf8'));return send(200,'text/plain','ok');}
      if(url.pathname==='/__sound/log'){console.log('[page] '+body.toString('utf8'));return send(200,'text/plain','ok');}
      if(url.pathname==='/__sound/done'){send(200,'text/plain','ok');finish('done');return;}
      return send(404,'text/plain','not found');}
    if(url.pathname==='/__sound/index.html')return send(200,'text/html; charset=utf-8',html);
    if(url.pathname==='/__sound/page.js')return send(200,'text/javascript; charset=utf-8',page);
    if(url.pathname==='/__sound/config.json')return send(200,'application/json',config);
    return send(404,'text/plain','not found');
  }catch(e){console.error('server error: '+e.message);try{res.writeHead(500);res.end();}catch(_){}}});
  await new Promise(r=>server.listen(0,'127.0.0.1',r));
  const pageUrl='http://127.0.0.1:'+server.address().port+'/__sound/index.html',profile=await fs.mkdtemp(path.join(os.tmpdir(),'sw-sound-'));
  const chrome=spawn(findChrome(),['--headless=new','--user-data-dir='+profile,'--no-first-run','--no-default-browser-check','--disable-extensions','--autoplay-policy=no-user-gesture-required',pageUrl],{stdio:'ignore',windowsHide:true});
  let spawnError=null;chrome.on('error',e=>{spawnError=e;finish('spawn-error');});chrome.on('exit',code=>{if(!report){console.error('Chrome exited early with code '+code);finish('chrome-exit');}});
  const timer=setTimeout(()=>finish('timeout'),o.timeout*1000),reason=await done;clearTimeout(timer);
  if(chrome.pid&&chrome.exitCode===null){if(process.platform==='win32')spawnSync('taskkill',['/PID',String(chrome.pid),'/T','/F'],{stdio:'ignore'});else try{chrome.kill('SIGKILL');}catch(_){}}
  server.closeAllConnections?.();server.close();
  for(let i=0;i<10;i++){try{await fs.rm(profile,{recursive:true,force:true});break;}catch(_){await new Promise(r=>setTimeout(r,300));}}
  if(spawnError){console.error('Could not start Chrome: '+spawnError.message);process.exit(1);}
  // Measured in Node from the files on disk: the same files the coordinator analyses.
  const rows=[];
  for(const s of situations){const file=path.join(outDir,s.name+'.wav');if(!existsSync(file))continue;
    const wav=readWav(await fs.readFile(file)),m=measure(wav.channels,wav.sampleRate,{skip:o.skip}),st=stats.get(s.name)||{};
    rows.push({name:s.name,seconds:wav.channels[0].length/wav.sampleRate,tier:s.tier||'high',lufs:db(m.lufs),truePeakDb:db(m.truePeakDb),samplePeakDb:db(m.samplePeakDb),rmsDb:db(m.rmsDb),
      renderMs:st.renderMs,realtime:st.realtime,steps:st.steps,stats:st.sound||null});}
  let base=null;if(o.baseline){try{base=JSON.parse(await fs.readFile(path.join(root,'reports','system-world-sound',o.baseline,'report.json'),'utf8'));}catch(_){console.warn('baseline '+o.baseline+' not found');}}
  const out={label:o.label,generatedAt:new Date().toISOString(),reason,skip:o.skip,calibration,situations:rows,checks:checks(rows,base),errors:report?.errors||[]};
  await fs.writeFile(path.join(outDir,'report.json'),JSON.stringify(out,null,2));
  console.log('--- summary ---');
  for(const r of rows)console.log(r.name.padEnd(24)+String(r.lufs).padStart(8)+' LUFS  TP'+String(r.truePeakDb).padStart(7)+' dBTP'+(r.deltaLu!=null?'  delta '+r.deltaLu+' LU':'')+(r.realtime?'  x'+r.realtime+' real time':''));
  if(calibration)console.log('calibration: '+JSON.stringify(calibration));
  for(const [k,c]of Object.entries(out.checks))console.log('check '+k+': '+(c.pass?'pass':'FAIL')+' '+JSON.stringify(c));
  for(const e of out.errors)console.log('  error ['+e.where+'] '+String(e.message).split('\n')[0]);
  console.log('report: '+path.relative(root,path.join(outDir,'report.json')));
  const failed=Object.values(out.checks).some(c=>!c.pass);
  process.exit(reason!=='done'||out.errors.length||rows.length<situations.length||(o.strict&&failed)?1:0);
}
main().catch(e=>{console.error(e.stack||e.message||e);process.exit(1);});
```

### `scripts/system-world-sound-survey.page.mjs` (complete)

```js
// Sound survey page (System World beauty pass, phase 5), bundled by scripts/system-world-sound-survey.mjs. Each
// situation renders the real soundscape through an OfflineAudioContext. The city's inputs (listener, residents, trams,
// door, wind, clocks) are stepped STEP times per second with suspend()/resume(); the engine's timers run on the audio
// clock; randomness is seeded, so a render repeats exactly. The only requests go to this page's own server.
import * as THREE from 'three';
import {createCityAmbience,LIMITER,createLimiter} from '../ui/js/desktop/apps/sysworld-audio.js';
import {windAt} from '../ui/js/desktop/apps/sysworld-atmosphere.js';
import {groundHeight} from '../ui/js/desktop/apps/sysworld-exploration.js';
import {streetCurve} from '../ui/js/desktop/apps/sysworld-navigation.js';
import {tramWaypoints} from '../ui/js/desktop/apps/sysworld-layout.js';
const RATE=48000,STEP=25,EYE=2.4,STRIDE=1.8;
const post=(url,body,type)=>fetch(url,{method:'POST',headers:{'Content-Type':type||'text/plain'},body});
const errors=[],log=t=>post('/__sound/log',String(t)).catch(()=>{});
addEventListener('error',e=>errors.push({where:'window',message:String(e.message)}));
addEventListener('unhandledrejection',e=>errors.push({where:'promise',message:String(e.reason&&e.reason.stack||e.reason)}));
const seeded=seed=>{let s=(seed>>>0)||1;return()=>(s=(Math.imul(s,1664525)+1013904223)>>>0)/4294967296;};
function audioTimers(){const tasks=new Map();let id=0,now=0;
  return {setTimeout(fn,ms){tasks.set(++id,{fn,due:now+Math.max(0,Number(ms)||0)/1000});return id;},clearTimeout(h){tasks.delete(h);},
    run(t){now=t;for(let guard=0;guard<128;guard++){let key=0,best=null;for(const [k,v]of tasks)if(v.due<=t&&(!best||v.due<best.due)){key=k;best=v;}if(!best)return;tasks.delete(key);best.fn();}}};}
function wav(buffer){const ch=buffer.numberOfChannels,len=buffer.length,bytes=44+len*ch*4,v=new DataView(new ArrayBuffer(bytes)),w=(o,s)=>{for(let i=0;i<4;i++)v.setUint8(o+i,s.charCodeAt(i));};
  w(0,'RIFF');v.setUint32(4,bytes-8,true);w(8,'WAVE');w(12,'fmt ');v.setUint32(16,16,true);v.setUint16(20,3,true);v.setUint16(22,ch,true);v.setUint32(24,buffer.sampleRate,true);
  v.setUint32(28,buffer.sampleRate*ch*4,true);v.setUint16(32,ch*4,true);v.setUint16(34,32,true);w(36,'data');v.setUint32(40,len*ch*4,true);
  const data=Array.from({length:ch},(_,c)=>buffer.getChannelData(c));let o=44;for(let i=0;i<len;i++)for(let c=0;c<ch;c++){v.setFloat32(o,data[c][i],true);o+=4;}return v.buffer;}
function along(points,s,out){let left=s;for(let i=1;i<points.length;i++){const [ax,az]=points[i-1],[bx,bz]=points[i],len=Math.hypot(bx-ax,bz-az);
  if(left<=len||i===points.length-1){const t=len?Math.min(1,left/len):0;out.x=ax+(bx-ax)*t;out.z=az+(bz-az)*t;if(len){out.fx=(bx-ax)/len;out.fz=(bz-az)/len;}return out;}left-=len;}return out;}
const polyLength=p=>p.slice(1).reduce((n,q,i)=>n+Math.hypot(q[0]-p[i][0],q[1]-p[i][1]),0);
async function render(s){
  const seconds=s.seconds||30,ctx=new OfflineAudioContext({numberOfChannels:2,length:Math.round(RATE*seconds),sampleRate:RATE}),timers=audioTimers(),m=s.mood||{},weather=m.weather||'clear';
  const sound=createCityAmbience(()=>{},{context:ctx,offline:true,timers,random:seeded(s.seed||1),voice:false,storage:null,enabled:true,tier:s.tier||'high'});
  sound.setTier?.(s.tier||'high');sound.setVolume(s.volume??.18);sound.setMood({day:m.day??1,evening:m.evening??0,busy:!!m.busy,weather});sound.setEnvironment(!!s.inside,weather);
  sound.unlock();sound.setActive(true);
  const L=s.listener||{},walk=s.walk||null,walkLength=walk?polyLength(walk):0,pose={x:walk?walk[0][0]:L.x??0,z:walk?walk[0][1]:L.z??0,fx:L.fx??0,fz:L.fz??-1};
  const route=streetCurve(tramWaypoints),routeLength=route.getLength(),point=new THREE.Vector3(),tangent=new THREE.Vector3();
  const trams=(s.trams||[]).slice(0,2).map(t=>({u:t.u||0,speed:t.speed??11})),walkers=s.walkers||[];
  const frame={time:0,seaTime:0,wind:{x:1,z:0,strength:0},trams:[{x:0,z:0,heading:0,on:false},{x:0,z:0,heading:0,on:false}],companion:{x:0,z:0,phase:0,on:false},doorValue:()=>s.door||0,reduced:false};
  let travelled=0,lastX=pose.x,lastZ=pose.z,steps=0;
  function advance(t){
    timers.run(t);
    if(walk)along(walk,Math.min(walkLength,t*(s.walkSpeed||1.4)),pose);
    const y=L.y!=null&&!walk?L.y:groundHeight(pose.x,pose.z)+EYE,origin=s.origin||[pose.x,pose.z];
    travelled+=Math.hypot(pose.x-lastX,pose.z-lastZ);lastX=pose.x;lastZ=pose.z;
    if(walk&&travelled>=STRIDE){travelled-=STRIDE;sound.effect('step',pose.x,y,pose.z);steps++;}
    sound.setListener(pose.x,y,pose.z,pose.fx,pose.fz,origin[0],origin[1]);
    for(const w of walkers){const len=Math.hypot(w.to[0]-w.from[0],w.to[1]-w.from[1])||1,d=(t*(w.speed||1.2))%(2*len),k=(d<=len?d:2*len-d)/len,x=w.from[0]+(w.to[0]-w.from[0])*k,z=w.from[1]+(w.to[1]-w.from[1])*k;
      sound.gait(w.id,x,w.style==='glide'?1.6:groundHeight(x,z),z,true,!!w.carrying,w.style||'');}
    trams.forEach((tr,i)=>{const u=((tr.u+tr.speed*t/routeLength)%1+1)%1;route.getPointAt(u,point);route.getTangentAt(u,tangent);const f=frame.trams[i];f.x=point.x;f.z=point.z;f.heading=Math.atan2(tangent.x,tangent.z);f.on=true;});
    frame.time=t;frame.seaTime=(s.seaTime||0)+t;windAt(t+(s.windOffset||0),weather,frame.wind);
    if(s.companion){const p=frame.companion,a=t/16*Math.PI*2;p.on=true;p.phase=t*1.7*Math.PI*2;p.x=30+4.5*Math.cos(a);p.z=69.5+2.5*Math.sin(a);}
    sound.frame?.(frame);
  }
  advance(0);
  for(let k=1;k<seconds*STEP;k++){const t=k/STEP;ctx.suspend(t).then(()=>{try{advance(t);}catch(e){errors.push({where:s.name,message:String(e&&e.stack||e)});}return ctx.resume();});}
  const started=performance.now(),buffer=await ctx.startRendering(),renderMs=performance.now()-started,stats=sound.stats();sound.dispose();
  return {buffer,stats:{renderMs:Math.round(renderMs),realtime:+(seconds*1000/renderMs).toFixed(1),steps,sound:stats}};
}
async function calibrate(){
  const out={trim:LIMITER.trim,ceiling:LIMITER.ceiling};
  for(const [label,amp]of[['quiet',Math.pow(10,-30/20)],['loud',2]]){
    const ctx=new OfflineAudioContext({numberOfChannels:1,length:RATE*2,sampleRate:RATE}),osc=ctx.createOscillator(),g=ctx.createGain(),lim=createLimiter(ctx);
    osc.frequency.value=1000;g.gain.value=amp;osc.connect(g).connect(lim.input);lim.output.connect(ctx.destination);osc.start();
    const d=(await ctx.startRendering()).getChannelData(0);let p=0,peak=0;for(let i=RATE;i<d.length;i++){p+=d[i]*d[i];peak=Math.max(peak,Math.abs(d[i]));}
    out[label]={gainDb:+(20*Math.log10(Math.sqrt(p/(d.length-RATE))/(amp/Math.SQRT2))).toFixed(3),peak:+peak.toFixed(4)};
  }
  out.makeupDb=+(out.quiet.gainDb-20*Math.log10(LIMITER.trim)).toFixed(3);out.suggestedTrim=+Math.pow(10,-out.makeupDb/20).toFixed(3);
  out.pass=Math.abs(out.quiet.gainDb)<=.25&&out.loud.peak<=LIMITER.ceiling+1e-3;return out;
}
async function main(){
  const config=await (await fetch('/__sound/config.json')).json();
  if(config.calibrate){try{await post('/__sound/calibration',JSON.stringify(await calibrate()),'application/json');}catch(e){errors.push({where:'calibrate',message:String(e&&e.stack||e)});}}
  for(const s of config.situations){try{log('rendering '+s.name+' ('+(s.seconds||30)+' s, '+(s.tier||'high')+')');const r=await render(s);
      const res=await post('/__sound/wav?name='+encodeURIComponent(s.name),wav(r.buffer),'audio/wav');if(!res.ok)throw Error('upload failed '+res.status);
      await post('/__sound/stats?name='+encodeURIComponent(s.name),JSON.stringify(r.stats),'application/json');}
    catch(e){errors.push({where:s.name,message:String(e&&e.stack||e)});}}
}
main().catch(e=>errors.push({where:'main',message:String(e&&e.stack||e)})).finally(async()=>{
  try{await post('/__sound/report',JSON.stringify({errors}),'application/json');}catch(_){}await post('/__sound/done','done').catch(()=>{});});
```

### `scripts/system-world-sound-survey.situations.json` (complete; one object per line in the file)

```json
[
{"name":"quay-day","seconds":30,"seed":11,"listener":{"x":-20,"z":77,"fx":0,"fz":1},"mood":{"day":1,"evening":0,"busy":false,"weather":"clear"},"companion":true},
{"name":"quay-dusk","seconds":30,"seed":12,"listener":{"x":-35,"z":77,"fx":0,"fz":1},"mood":{"day":0.2,"evening":0.8,"busy":false,"weather":"clear"}},
{"name":"street-day","seconds":30,"seed":13,"listener":{"x":-13.5,"z":-12,"fx":0,"fz":-1},"mood":{"day":1,"evening":0,"busy":false,"weather":"clear"},"walkers":[{"id":"resident-0","from":[-13,-20],"to":[-13,0],"speed":1.2},{"id":"resident-1","from":[-14.5,-2],"to":[-14.5,-24],"speed":1.1,"carrying":true},{"id":"patrol-0","from":[-16,-30],"to":[-16,6],"speed":1.6,"style":"glide"}]},
{"name":"street-night-rain","seconds":30,"seed":13,"listener":{"x":-13.5,"z":-12,"fx":0,"fz":-1},"mood":{"day":0,"evening":0,"busy":false,"weather":"rain"},"walkers":[{"id":"resident-0","from":[-13,-20],"to":[-13,0],"speed":1.2},{"id":"resident-1","from":[-14.5,-2],"to":[-14.5,-24],"speed":1.1,"carrying":true},{"id":"patrol-0","from":[-16,-30],"to":[-16,6],"speed":1.6,"style":"glide"}]},
{"name":"street-busy-rain-120","seconds":120,"seed":14,"listener":{"x":-13.5,"z":-12,"fx":0,"fz":-1},"mood":{"day":0.3,"evening":0.6,"busy":true,"weather":"rain"},"trams":[{"u":0,"speed":11},{"u":0.5,"speed":11}],"walkers":[{"id":"resident-0","from":[-13,-20],"to":[-13,0],"speed":1.2},{"id":"patrol-0","from":[-16,-30],"to":[-16,6],"speed":1.6,"style":"glide"}]},
{"name":"park-wind","seconds":30,"seed":15,"listener":{"x":-44,"z":-88,"fx":0,"fz":-1},"mood":{"day":1,"evening":0,"busy":false,"weather":"clear"},"windOffset":0},
{"name":"lobby-agent-shut","seconds":30,"seed":16,"listener":{"x":0,"z":-12,"fx":0,"fz":1},"inside":true,"door":0,"mood":{"day":1,"evening":0,"busy":false,"weather":"clear"}},
{"name":"lobby-agent-open","seconds":30,"seed":16,"listener":{"x":0,"z":-12,"fx":0,"fz":1},"inside":true,"door":1,"mood":{"day":1,"evening":0,"busy":false,"weather":"clear"}},
{"name":"lobby-departure","seconds":30,"seed":17,"listener":{"x":43,"z":35,"fx":0,"fz":1},"inside":true,"door":0,"mood":{"day":1,"evening":0,"busy":true,"weather":"clear"}},
{"name":"machine-hall","seconds":30,"seed":18,"listener":{"x":-43,"z":-56,"fx":0,"fz":1},"inside":true,"door":0,"mood":{"day":1,"evening":0,"busy":false,"weather":"clear"}},
{"name":"garden-court","seconds":30,"seed":19,"listener":{"x":-43,"z":36,"fx":0,"fz":1},"inside":false,"door":0,"mood":{"day":1,"evening":0,"busy":false,"weather":"clear"}},
{"name":"plaza-charging","seconds":30,"seed":20,"listener":{"x":79,"z":-55,"fx":-1,"fz":0},"mood":{"day":1,"evening":0,"busy":true,"weather":"clear"},"walkers":[{"id":"resident-2","from":[76,-62],"to":[76,-46],"speed":1.2}]},
{"name":"tram-corner","seconds":30,"seed":21,"listener":{"x":-60,"z":-70,"fx":-1,"fz":0},"mood":{"day":1,"evening":0,"busy":false,"weather":"clear"},"trams":[{"u":0.79,"speed":11}]},
{"name":"overview-day","seconds":30,"seed":22,"listener":{"x":120,"y":110,"z":183,"fx":-0.55,"fz":-0.83},"origin":[0,-12],"mood":{"day":1,"evening":0,"busy":false,"weather":"clear"}},
{"name":"walk-surfaces","seconds":20,"seed":23,"walk":[[2,13],[2,6.5],[2,0.2],[2,-3],[2,-9]],"walkSpeed":1.4,"door":1,"mood":{"day":1,"evening":0,"busy":false,"weather":"clear"}}
]
```

**Run**
- `node scripts/test-system-world-sound-meter.mjs` → PASS
- the calibration command, then the baseline command (above)

**Acceptance:**
- `calibration.json` has `pass: true`.
- The baseline has 15 WAVs, a report with LUFS and dBTP per file, and 0 page errors.
- The hand-back holds the summary table and the real-time factors.

**Commit:** `test(system-world): offline sound survey (OfflineAudioContext renders, BS.1770 loudness, true peak, limiter calibration) and baseline situations`

---

## Task 4 — `sysworld-sound-space.js` (glass, sends, canyon and interior spaces)

**Files:** new `ui/js/desktop/apps/sysworld-sound-space.js`, new `scripts/test-system-world-sound-space.mjs`.

**Steps:** test first (→ FAIL), then the module (→ PASS).

### `ui/js/desktop/apps/sysworld-sound-space.js` (complete)

```js
// Acoustic spaces (System World beauty pass, phase 5): the glass between listener and city (a low-pass and a gain while
// a closed room is around the listener), the reverb sends of the three spaces and the room bus. The open-air
// convolver stays in sysworld-audio.js (`verb`, 3.4 s); this module adds the street canyon's early echoes and one
// interior convolver whose impulse follows the room (lobby, hall, machine, small). The interior convolver is replaced,
// never re-buffered, and only while its send is near silent. Low tier: no extra convolver (both sends feed `verb`);
// medium: interior only; high and ultra: both. Pure laws and impulses are exported for the test.
export const GLASS=Object.freeze({open:18000,shut:900,db:-12,door:.7});
export const RETURNS=Object.freeze({canyon:.45,interior:.5,low:.3,leak:.35});
export const ROOM_SPACES=Object.freeze({
  lobby:Object.freeze({seconds:1.6,decay:3.2,predelay:.012,tone:5200}),
  hall:Object.freeze({seconds:2.4,decay:2.8,predelay:.02,tone:4200}),
  machine:Object.freeze({seconds:1.1,decay:2.4,predelay:.006,tone:7000,ring:Object.freeze([310,620,1240])}),
  small:Object.freeze({seconds:.6,decay:3.6,predelay:.004,tone:3600}),
});
// Early echoes between facades: [seconds, gain, pan].
export const CANYON_TAPS=Object.freeze([[.019,.62,-.5],[.031,.5,.6],[.047,.42,-.25],[.064,.33,.4],[.088,.24,-.6],[.121,.16,.3],[.163,.1,-.2]]);
const TIERS=Object.freeze({low:{canyon:false,interior:false},medium:{canyon:false,interior:true},high:{canyon:true,interior:true},ultra:{canyon:true,interior:true}});
const clamp01=v=>v<0?0:v>1?1:v;
function lcg(seed){let s=seed>>>0;return()=>(s=(Math.imul(s,1664525)+1013904223)>>>0)/4294967296;}
export function glassLaw(interior,open,door,out={}){const amount=open?0:clamp01(interior)*(1-GLASS.door*clamp01(door));
  out.amount=amount;out.cutoff=GLASS.open*Math.pow(GLASS.shut/GLASS.open,amount);out.gain=Math.pow(10,GLASS.db*amount/20);return out;}
export function sendLaw(w,out={}){const canyon=clamp01(w.canyon);out.canyon=canyon;out.open=1-canyon;out.interior=w.space?clamp01(w.interior):0;out.outdoor=1-out.interior;return out;}
export function roomLaw(interior,leak){const i=clamp01(interior);return i+(1-i)*clamp01(leak)*RETURNS.leak;}
export function impulseCanyon(sampleRate){
  const n=Math.ceil(sampleRate*.45),L=new Float32Array(n),R=new Float32Array(n),rnd=lcg(4243),burst=Math.max(4,Math.round(sampleRate*.0005));
  for(const [t,g,p]of CANYON_TAPS){const i=Math.round(t*sampleRate),gl=g*Math.sqrt((1-p)/2),gr=g*Math.sqrt((1+p)/2);
    for(let k=0;k<burst&&i+k<n;k++){const v=(rnd()*2-1)*Math.exp(-4*k/burst);L[i+k]+=v*gl;R[i+k]+=v*gr;}}
  for(let i=Math.round(.03*sampleRate);i<n;i++){const d=.05*Math.pow(1-i/n,4);L[i]+=(rnd()*2-1)*d;R[i]+=(rnd()*2-1)*d;}
  return [L,R];
}
// A room: predelay, low-passed noise with a power-law decay (and a metallic ring for the machine hall), normalised to
// the same energy for every space so a swap never jumps in level.
export function impulseRoom(name,sampleRate){
  const s=ROOM_SPACES[name];if(!s)throw Error('Unknown room space '+name);
  const n=Math.ceil(sampleRate*s.seconds),pre=Math.round(sampleRate*s.predelay),a=Math.exp(-2*Math.PI*Math.min(s.tone,sampleRate*.45)/sampleRate),out=[new Float32Array(n),new Float32Array(n)];
  let seed=977;for(let i=0;i<name.length;i++)seed=Math.imul(seed,31)+name.charCodeAt(i);const rnd=lcg(seed);
  for(let c=0;c<2;c++){const d=out[c];let lp=0,power=0;
    for(let i=pre;i<n;i++){const t=(i-pre)/(n-pre);lp=lp*a+(rnd()*2-1)*(1-a);let v=lp*Math.pow(1-t,s.decay);
      if(s.ring)for(let k=0;k<s.ring.length;k++)v+=.08*Math.sin(2*Math.PI*s.ring[k]*i/sampleRate+c)*Math.exp(-6*t);
      d[i]=v;power+=v*v;}
    const scale=.5/Math.sqrt(power||1);for(let i=0;i<n;i++)d[i]*=scale;}
  return out;
}
function buffer(context,[l,r]){const b=context.createBuffer(2,l.length,context.sampleRate);b.getChannelData(0).set(l);b.getChannelData(1).set(r);return b;}
export function createSoundSpace(context,{ambience,verb,tier='high'}){
  const nodes=new Set(),own=n=>(nodes.add(n),n),gain=value=>{const g=own(context.createGain());g.gain.value=value;return g;};
  const outdoor=gain(1),glassFilter=own(context.createBiquadFilter()),glassGain=gain(1);
  glassFilter.type='lowpass';glassFilter.frequency.value=GLASS.open;glassFilter.Q.value=.5;outdoor.connect(glassFilter).connect(glassGain).connect(ambience);
  const sendOutdoor=gain(1),openSplit=gain(1),canyonSplit=gain(0);sendOutdoor.connect(openSplit).connect(verb);sendOutdoor.connect(canyonSplit);
  const sendLocal=gain(1),interiorSplit=gain(0),outdoorSplit=gain(1);sendLocal.connect(interiorSplit);sendLocal.connect(outdoorSplit).connect(sendOutdoor);
  const roomSend=gain(1),room=gain(0),roomPan=own(context.createStereoPanner());room.connect(roomPan).connect(ambience);
  let level=TIERS[tier]?tier:'high',canyon=null,interior=null,low=null,loaded=null,waiting=0,swaps=0;
  const g={amount:0,cutoff:GLASS.open,gain:1},s={},last=new Float64Array(8).fill(NaN);
  function release(parts){for(const n of parts){try{n.disconnect();}catch(_){}nodes.delete(n);}}
  function route(){
    canyonSplit.disconnect();interiorSplit.disconnect();roomSend.disconnect();
    if(canyon){release([canyon.conv,canyon.ret]);canyon=null;}
    if(low){release([low]);low=null;}
    const t=TIERS[level];
    if(t.canyon){const conv=own(context.createConvolver()),ret=gain(RETURNS.canyon);conv.normalize=false;conv.buffer=buffer(context,impulseCanyon(context.sampleRate));
      canyonSplit.connect(conv);conv.connect(ret).connect(outdoor);canyon={conv,ret};}
    else canyonSplit.connect(verb);
    if(t.interior){if(interior){interiorSplit.connect(interior.conv);roomSend.connect(interior.conv);}}
    else{if(interior){release([interior.conv,interior.ret]);interior=null;loaded=null;}low=gain(RETURNS.low);low.connect(verb);interiorSplit.connect(low);roomSend.connect(low);}
  }
  function load(name){
    if(interior){try{interiorSplit.disconnect(interior.conv);}catch(_){}try{roomSend.disconnect(interior.conv);}catch(_){}release([interior.conv,interior.ret]);}
    const conv=own(context.createConvolver()),ret=gain(RETURNS.interior);conv.normalize=false;conv.buffer=buffer(context,impulseRoom(name,context.sampleRate));
    conv.connect(ret).connect(room);interiorSplit.connect(conv);roomSend.connect(conv);interior={conv,ret};loaded=name;swaps++;
  }
  function set(slot,param,value,tau,eps){if(Math.abs(value-last[slot])<=eps)return;last[slot]=value;param.setTargetAtTime(value,context.currentTime,tau);}
  route();
  return {outdoor,sendOutdoor,sendLocal,room,roomSend,
    // w: createZoneTracker().weights; dt: seconds since the last frame.
    update(w,dt=0){
      glassLaw(w.interior,w.open,w.door,g);sendLaw(w,s);
      set(0,glassFilter.frequency,g.cutoff,.12,g.cutoff*.02);set(1,glassGain.gain,g.gain,.12,.004);
      set(2,openSplit.gain,s.open,.2,.01);set(3,canyonSplit.gain,s.canyon,.2,.01);set(4,interiorSplit.gain,s.interior,.15,.01);set(5,outdoorSplit.gain,s.outdoor,.15,.01);
      set(6,room.gain,roomLaw(w.interior,w.leak),.2,.004);set(7,roomPan.pan,(w.leakPan||0)*(1-clamp01(w.interior)),.2,.02);
      if(!TIERS[level].interior)return;
      const want=w.space||w.leakSpace;if(!want||want===loaded){waiting=0;return;}
      waiting+=dt;if(!loaded||(w.interior<.02&&w.leak<.02)||waiting>1.5){load(want);waiting=0;}
    },
    setTier(value){if(!TIERS[value]||value===level)return;level=value;route();},
    stats:()=>({tier:level,canyon:!!canyon,interior:loaded,swaps,glass:+g.amount.toFixed(3),cutoff:Math.round(g.cutoff),nodes:nodes.size}),
    dispose(){for(const n of nodes){try{n.disconnect();}catch(_){}}nodes.clear();canyon=interior=low=null;loaded=null;},
  };
}
```

### `scripts/test-system-world-sound-space.mjs` (complete)

```js
// Phase 5 spaces: glass, send and room laws; impulses; the graph per tier; swaps only at a silent send; no automation
// while nothing changes; dispose.
import assert from 'node:assert/strict';
import {createFakeAudio} from './system-world-audio-fake.mjs';
import * as S from '../ui/js/desktop/apps/sysworld-sound-space.js';
const open=S.glassLaw(0,false,0);assert.deepEqual([open.amount,open.cutoff,open.gain],[0,S.GLASS.open,1]);
const shut=S.glassLaw(1,false,0);assert.ok(Math.abs(shut.cutoff-900)<1e-6&&Math.abs(20*Math.log10(shut.gain)+12)<1e-9,'shut glass: 900 Hz at -12 dB');
const ajar=S.glassLaw(1,false,1);assert.ok(ajar.amount>.25&&ajar.amount<.35&&ajar.cutoff>5000,'an open door lets the city in');
assert.equal(S.glassLaw(1,true,0).amount,0,'the open court has no glass');
let prev=-1;for(let i=0;i<=20;i++){const a=S.glassLaw(i/20,false,0).amount;assert.ok(a>=prev);prev=a;}
const sl=S.sendLaw({canyon:.3,interior:.4,space:'hall'});assert.ok(Math.abs(sl.open+sl.canyon-1)<1e-12&&Math.abs(sl.interior+sl.outdoor-1)<1e-12&&sl.interior===.4);
assert.equal(S.sendLaw({canyon:0,interior:1,space:null}).interior,0,'no room space, no interior send');
assert.equal(S.roomLaw(1,0),1);assert.equal(S.roomLaw(0,0),0);assert.ok(Math.abs(S.roomLaw(0,1)-S.RETURNS.leak)<1e-12);
const energy=a=>a.reduce((s,v)=>s+v*v,0);
for(const name of Object.keys(S.ROOM_SPACES)){const [l,r]=S.impulseRoom(name,8000),[l2]=S.impulseRoom(name,8000),sp=S.ROOM_SPACES[name];
  assert.equal(l.length,Math.ceil(8000*sp.seconds));assert.deepEqual(l,l2,name+' deterministic');assert.ok(l.every(Number.isFinite));
  assert.ok(Math.abs(energy(l)-.25)<1e-4&&Math.abs(energy(r)-.25)<1e-4,name+' equal energy');assert.ok(l.slice(0,Math.round(8000*sp.predelay)).every(v=>v===0),name+' predelay');}
const [cl]=S.impulseCanyon(48000);for(const [t]of S.CANYON_TAPS){const i=Math.round(t*48000);assert.ok(cl.slice(i,i+24).reduce((m,v)=>Math.max(m,Math.abs(v)),0)>.01,'tap at '+t);}
const fake=createFakeAudio(),ambience=fake.node('ambience'),verb=fake.node('verb-open'),space=S.createSoundSpace(fake.context,{ambience,verb,tier:'high'});
const reaches=(from,to)=>fake.downstream(from).has(to),reverbs=()=>fake.live('reverb').length;
assert.ok(reaches(space.outdoor,ambience)&&reaches(space.sendOutdoor,verb),'outdoor reaches the ambience through the glass; its send reaches the open air');
assert.equal(reverbs(),1,'high: the canyon at start; the interior loads with the first room');
const w={harbour:0,canyon:0,green:0,plaza:1,height:0,quay:0,interior:0,room:null,space:null,tone:null,open:false,door:0,leak:0,leakRoom:null,leakTone:null,leakSpace:null,leakPan:0};
const ramps=()=>fake.nodes.reduce((n,x)=>n+x.gain.ramp.length+x.frequency.ramp.length+x.pan.ramp.length,0);
space.update(w,1/60);const r0=ramps();for(let i=0;i<100;i++)space.update(w,1/60);assert.equal(ramps(),r0,'unchanged weights schedule nothing');
Object.assign(w,{interior:1,room:'lobby:agent',space:'lobby',tone:'agent'});space.update(w,1/60);
assert.equal(space.stats().interior,'lobby');assert.equal(reverbs(),2);assert.ok(reaches(space.sendLocal,space.room)&&reaches(space.roomSend,space.room),'local and room sends reach the room bus');
Object.assign(w,{space:'machine',room:'lobby:infra'});space.update(w,1/60);assert.equal(space.stats().interior,'lobby','no swap at full send');
w.interior=.01;space.update(w,1/60);assert.equal(space.stats().interior,'machine');assert.equal(reverbs(),2,'the old interior convolver is released');
space.setTier('low');assert.equal(reverbs(),0,'low: no extra convolver');assert.ok(reaches(space.sendLocal,verb)&&reaches(space.roomSend,verb),'low: room and local sends use the open-air verb');
space.setTier('medium');space.update(w,1/60);assert.equal(reverbs(),1,'medium: interior only');assert.equal(space.stats().canyon,false);
space.dispose();assert.equal(space.stats().nodes,0);
console.log('System World sound spaces: glass, sends, room bus, impulses, tiers and swaps verified');
```

**Run:** `node scripts/test-system-world-sound-space.mjs` → PASS.

**Commit:** `feat(system-world): acoustic spaces (glass damping, open-air/canyon/interior sends, room impulses) with their test`

---

## Task 5 — `sysworld-sound-beds.js` (leaves, laps, steam, traffic, rain, patter)

**Files:** new `ui/js/desktop/apps/sysworld-sound-beds.js`, new `scripts/test-system-world-sound-beds.mjs`.

### `ui/js/desktop/apps/sysworld-sound-beds.js` (complete)

```js
// Zone-shaped beds (System World beauty pass, phase 5): wind in the leaves (the atmosphere's wind), lapping at the quay
// (the sea shader's own wave), steam hiss at the foundry vents and drains, a traffic wash in the street canyons, rain
// on stone, water and leaves, and rain patter on the glass indoors. One shared noise buffer plus a generated patter
// loop; the graph is fixed per tier and update() allocates only for a lap. Low tier: traffic, rain and laps. Levels
// are first estimates, tuned by the sound survey within the ranges of plans/2026-11-10-system-world-beauty-phase-5.md.
import {seaHeight} from './sysworld-atmosphere.js';
import {steamVents} from './sysworld-particles.js';
import {bearingPan} from './sysworld-zones.js';

export const BEDS=Object.freeze({leaves:.05,laps:.05,steam:.03,traffic:.012,rain:.02,patter:.02});
export const LAP=Object.freeze({rise:.15,gap:.9,max:3,low:380,high:700,shortest:.22,longest:.4,quayZ:80});
const RICH=Object.freeze({low:false,medium:true,high:true,ultra:true}),VENTS=steamVents();
const clamp01=v=>v<0?0:v>1?1:v;
export function leafLevel(green,strength){const s=clamp01(strength);return BEDS.leaves*clamp01(green)*(.12+.88*s*s);}
export function trafficLevel(busy,w,rain){return BEDS.traffic*(busy?1.6:1)*(.2+.8*w.canyon+.3*w.plaza)*(1-.6*w.height)*(1-.5*w.harbour)*(rain?1.25:1);}
export function rainLevel(rain,w){return rain?BEDS.rain*(.5+.5*w.harbour+.4*w.green+.3*w.canyon)*(1-.7*w.height):0;}
export function patterLevel(rain,w){return rain&&!w.open?BEDS.patter*clamp01(w.interior):0;}
// Each vent adds size/(1 + (d/6)^2); the loudest vent sets the direction.
export function steamLevel(x,y,z,out={}){let sum=0,best=0;out.x=x;out.z=z;
  for(let i=0;i<VENTS.length;i++){const v=VENTS[i],d=Math.hypot(v.x-x,v.y-y,v.z-z),l=v.size/(1+(d/6)*(d/6));sum+=l;if(l>best){best=l;out.x=v.x;out.z=v.z;}}
  out.level=BEDS.steam*Math.min(1,sum);return out;}
export function lapCrossing(previous,height){return previous<LAP.rise&&height>=LAP.rise;}
// Two seconds of sparse decaying clicks (about 40 per second): rain on glass once band-passed.
export function patterBuffer(context,random){
  const n=Math.floor(context.sampleRate*2),b=context.createBuffer(1,n,context.sampleRate),d=b.getChannelData(0),len=Math.max(4,Math.floor(context.sampleRate*.004));
  for(let t=-Math.log(1-random()*.999)/40;;t+=-Math.log(1-random()*.999)/40){const i=Math.floor(t*context.sampleRate);if(i>=n-len)break;
    const a=.3+.7*random();for(let k=0;k<len;k++)d[i+k]+=a*(random()*2-1)*Math.exp(-4*k/len);}
  return b;
}
export function createSoundBeds(context,{outdoor,send,local,noise,random=Math.random,tier='high'}){
  let level=RICH[tier]===undefined?'high':tier,busy=false,rain=false,lastLap=-Infinity,previousSea=0,played=0,parts=null;
  const laps=new Set(),st={level:0,x:0,z:0},last=new Float64Array(8);
  function build(){
    const nodes=[],own=n=>(nodes.push(n),n),amp=v=>{const g=own(context.createGain());g.gain.value=v;return g;};
    const filter=(type,f,q=.7)=>{const n=own(context.createBiquadFilter());n.type=type;n.frequency.value=f;n.Q.value=q;return n;};
    const loop=(rate,buffer=noise)=>{const s=own(context.createBufferSource());s.buffer=buffer;s.loop=true;s.playbackRate.value=rate;s.start();return s;};
    const lfo=(rate,depth,target)=>{const o=own(context.createOscillator()),d=amp(depth);o.frequency.value=rate;o.connect(d).connect(target);o.start();};
    const p={nodes};
    p.traffic=amp(0);loop(.81).connect(filter('lowpass',380)).connect(amp(.7)).connect(p.traffic);
    p.hiss=filter('bandpass',900,.8);loop(.77).connect(p.hiss).connect(amp(.3)).connect(p.traffic);
    p.traffic.connect(outdoor);p.traffic.connect(amp(.3)).connect(send);
    p.rain=amp(0);loop(1.13).connect(filter('highpass',1500)).connect(filter('lowpass',7000)).connect(p.rain).connect(outdoor);
    if(RICH[level]){
      p.leaves=amp(0);p.leafBand=filter('bandpass',3200,.7);const am=amp(1);
      loop(.93).connect(p.leafBand).connect(am).connect(p.leaves).connect(outdoor);lfo(.7,.25,am.gain);lfo(1.3,.2,am.gain);p.leaves.connect(amp(.25)).connect(send);
      p.steam=amp(0);p.steamPan=own(context.createStereoPanner());
      loop(1.07).connect(filter('highpass',2500)).connect(filter('bandpass',5000,.5)).connect(p.steam).connect(p.steamPan).connect(outdoor);
      p.patter=amp(0);loop(1,patterBuffer(context,random)).connect(filter('bandpass',2200,.9)).connect(p.patter).connect(local);
    }
    last.fill(NaN);return p;
  }
  function teardown(){if(!parts)return;for(const n of parts.nodes){try{n.stop?.();}catch(_){}try{n.disconnect();}catch(_){}}parts=null;}
  function set(slot,param,value,tau=.3,eps=2e-4){const prev=last[slot];if(Math.abs(value-prev)<=Math.max(eps,Math.abs(prev)*.03))return;last[slot]=value;param.setTargetAtTime(value,context.currentTime,tau);}
  function lap(at,quay){
    lastLap=at;played++;
    const src=context.createBufferSource(),band=context.createBiquadFilter(),env=context.createGain(),pan=context.createStereoPanner(),wet=context.createGain();
    const d=LAP.shortest+(LAP.longest-LAP.shortest)*random();
    src.buffer=noise;src.loop=true;band.type='bandpass';band.frequency.value=LAP.low+(LAP.high-LAP.low)*random();band.Q.value=1.2;pan.pan.value=random()-.5;wet.gain.value=.4;
    env.gain.setValueAtTime(0,at);env.gain.linearRampToValueAtTime(BEDS.laps*quay*Math.sqrt(quay),at+.03);env.gain.exponentialRampToValueAtTime(.0001,at+d);
    src.connect(band).connect(env).connect(pan).connect(outdoor);pan.connect(wet).connect(send);
    const entry={release(){if(!laps.delete(entry))return;for(const n of[src,band,env,pan,wet]){try{n.stop?.();}catch(_){}try{n.disconnect();}catch(_){}}}};
    laps.add(entry);src.onended=entry.release;src.start(at);src.stop(at+d+.05);
  }
  parts=build();
  return {
    // f: the sound frame; w: tracker weights; listener: {x,y,z,fx,fz,origin:{x,z}}; now: context seconds.
    update(f,w,listener,now){
      if(!parts)return;
      set(0,parts.traffic.gain,trafficLevel(busy,w,rain));set(1,parts.hiss.frequency,rain?1800:900,.8,1);set(2,parts.rain.gain,rainLevel(rain,w),.6);
      if(parts.leaves){const s=f&&f.wind?f.wind.strength:0;set(3,parts.leaves.gain,leafLevel(w.green,s),.25);set(4,parts.leafBand.frequency,2400+1600*clamp01(s),.4,20);
        steamLevel(listener.x,listener.y,listener.z,st);set(5,parts.steam.gain,st.level,.4);
        set(6,parts.steamPan.pan,bearingPan(st.x-listener.x,st.z-listener.z,listener.fx,listener.fz),.3,.02);set(7,parts.patter.gain,patterLevel(rain,w),.5);}
      const qx=Math.max(-80,Math.min(80,listener.origin.x)),h=seaHeight(qx,LAP.quayZ,f&&f.time||0),quay=w.quay||0;
      if(lapCrossing(previousSea,h)&&now-lastLap>LAP.gap&&quay>.05&&laps.size<LAP.max)lap(now,quay);
      previousSea=h;
    },
    setMood(m){busy=!!m?.busy;rain=m?.weather==='rain';},
    setTier(value){if(RICH[value]===undefined||value===level)return;const change=RICH[value]!==RICH[level];level=value;if(change){teardown();parts=build();}},
    clear(){for(const e of[...laps])e.release();},
    stats:()=>({tier:level,rich:!!RICH[level],laps:laps.size,played,nodes:parts?parts.nodes.length:0,
      levels:{traffic:+(last[0]||0).toFixed(4),rain:+(last[2]||0).toFixed(4),leaves:+(last[3]||0).toFixed(4),steam:+(last[5]||0).toFixed(4),patter:+(last[7]||0).toFixed(4)}}),
    dispose(){for(const e of[...laps])e.release();teardown();},
  };
}
```

### `scripts/test-system-world-sound-beds.mjs` (complete)

```js
// Phase 5 beds: level laws, steam by vent distance, laps from the sea wave, the graph per tier, cost and dispose.
import assert from 'node:assert/strict';
import {createFakeAudio} from './system-world-audio-fake.mjs';
import * as B from '../ui/js/desktop/apps/sysworld-sound-beds.js';
import {steamVents} from '../ui/js/desktop/apps/sysworld-particles.js';
const zone=o=>({harbour:0,canyon:0,green:0,plaza:1,height:0,interior:0,open:false,quay:0,...o});
assert.equal(B.leafLevel(0,1),0);assert.ok(Math.abs(B.leafLevel(1,0)-B.BEDS.leaves*.12)<1e-12);assert.ok(Math.abs(B.leafLevel(1,1)-B.BEDS.leaves)<1e-12);
let prev=-1;for(let s=0;s<=1.0001;s+=.05){const v=B.leafLevel(.7,s);assert.ok(v>=prev);prev=v;}
assert.ok(B.trafficLevel(false,zone({canyon:1,plaza:0}),false)>B.trafficLevel(false,zone({}),false),'canyons carry more traffic than an open plaza');
assert.ok(B.trafficLevel(false,zone({plaza:0,harbour:1}),false)<B.trafficLevel(false,zone({}),false));
assert.ok(Math.abs(B.trafficLevel(true,zone({}),false)/B.trafficLevel(false,zone({}),false)-1.6)<1e-12,'busy');
assert.equal(B.rainLevel(false,zone({})),0);assert.ok(B.rainLevel(true,zone({harbour:1,plaza:0}))>B.rainLevel(true,zone({})));
assert.equal(B.patterLevel(true,zone({interior:1,open:true})),0,'no glass in the open court');assert.equal(B.patterLevel(true,zone({interior:1})),B.BEDS.patter);assert.equal(B.patterLevel(false,zone({interior:1})),0);
const tower=steamVents()[0],near=B.steamLevel(tower.x,2.4,tower.z+3,{}),far=B.steamLevel(0,2.4,300,{});
assert.ok(near.level>.5*B.BEDS.steam&&near.x===tower.x&&near.z===tower.z,'beside a cooling tower '+JSON.stringify(near));assert.ok(far.level<.02*B.BEDS.steam,'far out at sea '+far.level);
assert.equal(B.lapCrossing(.1,.2),true);assert.equal(B.lapCrossing(.2,.3),false);assert.equal(B.lapCrossing(.2,.1),false);
const seeded=s=>()=>(s=(Math.imul(s,1664525)+1013904223)>>>0)/4294967296;
const L=(x,z)=>({x,y:2.4,z,fx:0,fz:-1,origin:{x,z}}),fake=createFakeAudio(),c=fake.context,[outdoor,send,local]=['outdoor','send','local'].map(k=>fake.node(k));
const beds=B.createSoundBeds(c,{outdoor,send,local,noise:c.createBuffer(1,c.sampleRate*2),random:seeded(3),tier:'high'});
const sources=()=>fake.live('source').length+fake.live('oscillator').length,high=sources();
assert.ok(fake.downstream(fake.nodes.find(n=>n.kind==='source')).has(outdoor));
beds.setMood({busy:false,weather:'rain'});const f={time:0,wind:{strength:.5}},w=zone({green:.5,plaza:.5});
beds.update(f,w,L(-44,-88),0);const ramps=()=>fake.nodes.reduce((n,x)=>n+x.gain.ramp.length+x.frequency.ramp.length+x.pan.ramp.length,0),r0=ramps();
for(let i=0;i<120;i++)beds.update(f,w,L(-44,-88),0);assert.equal(ramps(),r0,'unchanged input schedules nothing');
assert.ok(beds.stats().levels.leaves>0&&beds.stats().levels.rain>0&&beds.stats().levels.patter===0);
let maxAlive=0;for(let k=0;k<60*25;k++){const t=k/25;c.currentTime=t;beds.update({time:t,wind:{strength:.3}},zone({harbour:.8,plaza:.2,quay:1}),L(-20,77),t);maxAlive=Math.max(maxAlive,beds.stats().laps);if(k%5===0)fake.flush();}
const played=beds.stats().played;assert.ok(played>=5&&played<=60,'laps in a minute at the quay: '+played);assert.ok(maxAlive<=B.LAP.max);
for(let k=0;k<30*25;k++){const t=60+k/25;c.currentTime=t;beds.update({time:t,wind:{strength:.3}},zone({}),L(0,0),t);}assert.equal(beds.stats().played,played,'no laps inland');
beds.setTier('low');assert.ok(sources()<high&&beds.stats().rich===false,'low: traffic and rain only');beds.setTier('high');assert.equal(sources(),high,'high rebuilds');
fake.flush();beds.dispose();assert.equal(fake.live().filter(n=>!['destination','outdoor','send','local'].includes(n.kind)).length,0,'dispose releases everything');
console.log(`System World sound beds: laws, steam, ${played} laps per minute at the quay, tiers and cost verified`);
```

**Run:** `node scripts/test-system-world-sound-beds.mjs` → PASS.

**Commit:** `feat(system-world): zone-shaped sound beds (leaves in the wind, quay laps, steam, traffic, rain, glass patter) with their test`

---

## Task 6 — `sysworld-sound-movers.js` (trams, hover robots, boats, gulls)

**Files:** new `ui/js/desktop/apps/sysworld-sound-movers.js`, new `scripts/test-system-world-sound-movers.mjs`.

### `ui/js/desktop/apps/sysworld-sound-movers.js` (complete)

```js
// Moving sources (System World beauty pass, phase 5): the trams' rail hum with a squeal in curves and rail-joint clacks,
// a whir for the nearest hover robots (the gliding street patrols of sysworld-life.js, reported through the residents'
// gait calls), the harbour launch's engine and the barge's drone (craftPoses on the atmosphere clock), and positions for
// gull calls (gullPose). A voice is built when its source comes within reach and released MOVERS.release seconds
// after it leaves, so a quiet district costs nothing; update() allocates only at those edges and for a clack. Distance
// runs from the acoustic origin, the pan from the camera (the city's convention).
import {craftPoses} from './sysworld-harbour.js';
import {gullPose,FLOCK} from './sysworld-gulls.js';
import {bearingPan} from './sysworld-zones.js';

export const MOVERS=Object.freeze({tram:.06,squeal:.02,hover:.025,launch:.05,barge:.04,clack:.5,joint:12,axles:2.2,maxClacks:6,release:1.25,
  reach:Object.freeze({tram:90,hover:30,launch:120,barge:260,clack:60})});
export const VOICES=Object.freeze({low:Object.freeze({trams:1,hovers:2,barge:false}),medium:Object.freeze({trams:2,hovers:4,barge:true}),
  high:Object.freeze({trams:2,hovers:4,barge:true}),ultra:Object.freeze({trams:2,hovers:4,barge:true})});
const clamp01=v=>v<0?0:v>1?1:v;
export function distanceGain(d,ref,rolloff){return d<=ref?1:ref/(ref+rolloff*(d-ref));}
export function dopplerRatio(previous,distance,dt){if(!(dt>0)||!Number.isFinite(previous))return 1;return Math.max(.94,Math.min(1.06,343/(343+(distance-previous)/dt)));}
export function tramLaw(speed,lateral,distance,out={}){const g=distance<MOVERS.reach.tram?distanceGain(distance,6,1):0;
  out.hum=MOVERS.tram*Math.pow(clamp01(speed/11),.7)*g;out.squeal=MOVERS.squeal*clamp01((lateral-1)/3)*g;out.motor=60+32*clamp01(speed/11);return out;}
// Indices of the `count` nearest items with `on` within `reach` of (x, z), nearest first; -1 pads. No allocation.
export function nearestSlots(items,count,x,z,reach,out){
  for(let k=0;k<count;k++)out[k]=-1;
  for(let i=0;i<items.length;i++){const it=items[i];if(!it.on)continue;const d=Math.hypot(it.x-x,it.z-z);if(d>reach)continue;
    for(let k=0;k<count;k++){const j=out[k];if(j<0||d<Math.hypot(items[j].x-x,items[j].z-z)){for(let m=count-1;m>k;m--)out[m]=out[m-1];out[k]=i;break;}}}
  return out;
}
export function createSoundMovers(context,{outdoor,send,noise,tier='high'}){
  let level=VOICES[tier]?tier:'high',hoverClock=0,clacks=0,clacksPlayed=0;
  const trams=[0,1].map(()=>({voice:null,x:0,z:0,heading:0,speed:0,turn:0,distance:NaN,travelled:0,idle:0,fresh:true}));
  const gliders=[],gliderIds=new Map(),slots=Array.from({length:4},()=>({voice:null,index:-1,idle:0})),picks=new Int32Array(4);
  const launch={voice:null,idle:0},barge={voice:null,idle:0},poses=craftPoses(0),law={hum:0,squeal:0,motor:60},live=new Set();
  function kit(){const v={nodes:[],last:new Float64Array(6).fill(NaN)},own=n=>(v.nodes.push(n),n);
    v.amp=x=>{const g=own(context.createGain());g.gain.value=x;return g;};
    v.filter=(type,f,q)=>{const n=own(context.createBiquadFilter());n.type=type;n.frequency.value=f;n.Q.value=q;return n;};
    v.noise=rate=>{const s=own(context.createBufferSource());s.buffer=noise;s.loop=true;s.playbackRate.value=rate;return s;};
    v.osc=(type,f)=>{const o=own(context.createOscillator());o.type=type;o.frequency.value=f;return o;};
    v.out=v.amp(0);v.pan=own(context.createStereoPanner());v.out.connect(v.pan).connect(outdoor);return v;}
  function start(v){for(const n of v.nodes)if(n.start)n.start();live.add(v);return v;}
  function release(v){if(!v)return null;for(const n of v.nodes){try{n.stop?.();}catch(_){}try{n.disconnect();}catch(_){}}live.delete(v);return null;}
  function tramVoice(){const v=kit();v.squeal=v.amp(0);v.squeal.connect(v.pan);v.out.connect(v.amp(.35)).connect(send);
    v.noise(.6).connect(v.filter('lowpass',160,.7)).connect(v.amp(.6)).connect(v.out);
    v.motor=v.osc('sawtooth',60);v.motor.connect(v.filter('lowpass',400,.7)).connect(v.amp(.3)).connect(v.out);
    v.whine=v.osc('sine',90);v.whine.connect(v.amp(.15)).connect(v.out);
    v.noise(1).connect(v.filter('bandpass',2900,14)).connect(v.squeal);v.osc('sine',3150).connect(v.amp(.3)).connect(v.squeal);return start(v);}
  function hoverVoice(k){const v=kit();v.out.connect(v.amp(.2)).connect(send);
    v.osc('sine',180+7*k).connect(v.out);v.osc('sine',271+9*k).connect(v.amp(.4)).connect(v.out);v.noise(1+.05*k).connect(v.filter('bandpass',1200,3)).connect(v.amp(.5)).connect(v.out);return start(v);}
  function launchVoice(){const v=kit(),am=v.amp(.6);v.out.connect(v.amp(.3)).connect(send);
    v.osc('sawtooth',46).connect(v.filter('lowpass',260,.7)).connect(am).connect(v.out);v.osc('sine',7.5).connect(v.amp(.4)).connect(am.gain);return start(v);}
  function bargeVoice(){const v=kit(),lp=v.filter('lowpass',120,.7);lp.connect(v.out);v.osc('sine',33).connect(lp);v.osc('sine',66).connect(v.amp(.5)).connect(lp);return start(v);}
  function set(v,slot,param,value,tau,eps=2e-4){const prev=v.last[slot];if(Math.abs(value-prev)<=Math.max(eps,Math.abs(prev)*.004))return;v.last[slot]=value;param.setTargetAtTime(value,context.currentTime,tau);}
  function clack(at,gain,pan){if(clacks>=MOVERS.maxClacks)return;clacks++;clacksPlayed++;
    const src=context.createBufferSource(),band=context.createBiquadFilter(),env=context.createGain();src.buffer=noise;src.loop=true;band.type='bandpass';band.frequency.value=1800;band.Q.value=2;
    env.gain.setValueAtTime(0,at);env.gain.linearRampToValueAtTime(gain*MOVERS.clack,at+.002);env.gain.exponentialRampToValueAtTime(.0001,at+.012);
    src.connect(band).connect(env).connect(pan);src.onended=()=>{clacks--;for(const n of[src,band,env])try{n.disconnect();}catch(_){}};src.start(at);src.stop(at+.02);}
  function moveTram(t,src,listener,now,dt){
    const ox=listener.origin.x,oz=listener.origin.z;
    if(src&&src.on){
      const step=Math.hypot(src.x-t.x,src.z-t.z);
      if(t.fresh||step>20){t.x=src.x;t.z=src.z;t.heading=src.heading;t.speed=0;t.turn=0;t.fresh=false;t.distance=NaN;}
      else if(dt>0){const k=1-Math.exp(-dt/.25),dh=Math.atan2(Math.sin(src.heading-t.heading),Math.cos(src.heading-t.heading));
        t.speed+=(step/dt-t.speed)*k;t.turn+=(dh/dt-t.turn)*k;t.travelled+=step;t.x=src.x;t.z=src.z;t.heading=src.heading;}
      const d=Math.hypot(t.x-ox,t.z-oz);
      if(!t.voice&&d<MOVERS.reach.tram)t.voice=tramVoice();
      if(t.voice){const v=t.voice,r=dopplerRatio(t.distance,d,dt);tramLaw(t.speed,Math.abs(t.turn)*t.speed,d,law);
        set(v,0,v.out.gain,law.hum,.15);set(v,1,v.squeal.gain,law.squeal,.1);set(v,2,v.motor.frequency,law.motor*r,.2,.05);set(v,3,v.whine.frequency,law.motor*1.5*r,.2,.05);
        set(v,4,v.pan.pan,bearingPan(t.x-listener.x,t.z-listener.z,listener.fx,listener.fz),.1,.02);
        while(t.travelled>=MOVERS.joint){t.travelled-=MOVERS.joint;if(d<MOVERS.reach.clack&&law.hum>0){clack(now,law.hum,v.pan);clack(now+MOVERS.axles/Math.max(1,t.speed),law.hum,v.pan);}}
        t.idle=d>MOVERS.reach.tram*1.2?t.idle+dt:0;if(t.idle>MOVERS.release){t.voice=release(t.voice);t.idle=0;}}
      else t.travelled%=MOVERS.joint;
      t.distance=d;
    }else{t.fresh=true;if(t.voice){set(t.voice,0,t.voice.out.gain,0,.15);set(t.voice,1,t.voice.squeal.gain,0,.1);t.idle+=dt;if(t.idle>MOVERS.release){t.voice=release(t.voice);t.idle=0;}}}
  }
  function moveHovers(listener,dt){
    const n=VOICES[level].hovers,ox=listener.origin.x,oz=listener.origin.z;hoverClock+=dt;
    if(hoverClock>=.2||dt===0){hoverClock=0;nearestSlots(gliders,n,ox,oz,MOVERS.reach.hover,picks);
      for(let k=0;k<slots.length;k++){const s=slots[k],j=k<n?picks[k]:-1;if(j>=0){s.index=j;s.idle=0;if(!s.voice)s.voice=hoverVoice(k);}else s.index=-1;}}
    for(let k=0;k<slots.length;k++){const s=slots[k];if(!s.voice)continue;
      if(s.index<0){set(s.voice,0,s.voice.out.gain,0,.3);s.idle+=dt;if(s.idle>MOVERS.release){s.voice=release(s.voice);s.idle=0;}continue;}
      const g=gliders[s.index],d=Math.hypot(g.x-ox,g.z-oz);
      set(s.voice,0,s.voice.out.gain,MOVERS.hover*distanceGain(d,3,.6)*(g.moving?1:.6),.15);set(s.voice,1,s.voice.pan.pan,bearingPan(g.x-listener.x,g.z-listener.z,listener.fx,listener.fz),.1,.02);}
  }
  function boat(b,pose,make,gain,ref,rolloff,reach,listener,dt,allowed){
    const d=Math.hypot(pose.x-listener.origin.x,pose.z-listener.origin.z);
    if(allowed&&!b.voice&&d<reach)b.voice=make();if(!b.voice)return;
    const on=allowed&&d<reach*1.2;set(b.voice,0,b.voice.out.gain,on?gain*distanceGain(d,ref,rolloff):0,.4);
    set(b.voice,1,b.voice.pan.pan,bearingPan(pose.x-listener.x,pose.z-listener.z,listener.fx,listener.fz),.3,.02);
    b.idle=on?0:b.idle+dt;if(b.idle>MOVERS.release){b.voice=release(b.voice);b.idle=0;}
  }
  return {
    gait(id,x,y,z,moving,style){if(style!=='glide')return;let g=gliderIds.get(id);if(!g){if(gliders.length>=32)return;g={id,x,y,z,on:true,moving:false};gliderIds.set(id,g);gliders.push(g);}
      g.x=x;g.y=y;g.z=z;g.moving=!!moving;},
    update(f,listener,now,dt){
      for(let i=0;i<trams.length;i++)moveTram(trams[i],i<VOICES[level].trams&&f&&f.trams?f.trams[i]:null,listener,now,dt);
      moveHovers(listener,dt);craftPoses(f&&f.seaTime||0,poses);
      boat(launch,poses[0],launchVoice,MOVERS.launch,10,.15,MOVERS.reach.launch,listener,dt,true);
      boat(barge,poses[1],bargeVoice,MOVERS.barge,20,.08,MOVERS.reach.barge,listener,dt,VOICES[level].barge);
    },
    // A position for a gull call: one bird of the visible flock (none on low: the caller keeps its harbour anchor).
    gull(f,random,out){const n=FLOCK[level]||0;if(!n)return null;const t=f&&f.seaTime||0;craftPoses(t,poses);return gullPose(Math.floor(random()*n)%n,t,poses[1],out);},
    setTier(value){if(VOICES[value])level=value;},
    clear(){},
    stats:()=>({tier:level,trams:trams.map(t=>({voice:!!t.voice,speed:+t.speed.toFixed(2),hum:t.voice?+(t.voice.last[0]||0).toFixed(4):0,squeal:t.voice?+(t.voice.last[1]||0).toFixed(4):0,
        distance:Number.isFinite(t.distance)?+t.distance.toFixed(1):null})),
      hovers:slots.filter(s=>s.voice&&s.index>=0).map(s=>gliders[s.index].id),gliders:gliders.length,launch:!!launch.voice,barge:!!barge.voice,clacks,clacksPlayed,
      nodes:[...live].reduce((n,v)=>n+v.nodes.length,0)}),
    dispose(){for(const v of[...live])release(v);for(const t of trams)t.voice=null;for(const s of slots)s.voice=null;launch.voice=barge.voice=null;},
  };
}
```

### `scripts/test-system-world-sound-movers.mjs` (complete)

```js
// Phase 5 moving sources: laws, nearest slots, a tram pass (reach, Doppler, clacks, release), a curve's squeal, hover
// voices per tier, boats in reach, gull positions, no automation while nothing moves, dispose.
import assert from 'node:assert/strict';
import {createFakeAudio} from './system-world-audio-fake.mjs';
import * as M from '../ui/js/desktop/apps/sysworld-sound-movers.js';
import {HARBOUR_SKY} from '../ui/js/desktop/apps/sysworld-gulls.js';
assert.equal(M.distanceGain(3,6,1),1);let prev=2;for(let d=6;d<200;d+=7){const g=M.distanceGain(d,6,1);assert.ok(g<=prev&&g>0);prev=g;}
assert.ok(M.dopplerRatio(50,49,.1)>1&&M.dopplerRatio(49,50,.1)<1,'approaching sounds higher');assert.equal(M.dopplerRatio(0,1000,.1),.94);assert.equal(M.dopplerRatio(NaN,5,.1),1);
assert.equal(M.tramLaw(0,0,5,{}).hum,0);assert.equal(M.tramLaw(11,0,5,{}).hum,M.MOVERS.tram);assert.equal(M.tramLaw(11,4,5,{}).squeal,M.MOVERS.squeal);
assert.equal(M.tramLaw(11,0,95,{}).hum,0,'beyond reach');assert.equal(M.tramLaw(11,.5,5,{}).squeal,0,'a straight run does not squeal');
const items=[{x:10,z:0,on:true},{x:2,z:0,on:true},{x:5,z:0,on:false},{x:40,z:0,on:true},{x:3,z:0,on:true}],out=new Int32Array(3);
assert.deepEqual([...M.nearestSlots(items,3,0,0,30,out)],[1,4,0]);assert.deepEqual([...M.nearestSlots(items,3,0,0,4,out)],[1,4,-1]);
function rig(tier='high'){const fake=createFakeAudio(),c=fake.context,outdoor=fake.node('outdoor'),send=fake.node('send');return {fake,c,movers:M.createSoundMovers(c,{outdoor,send,noise:c.createBuffer(1,c.sampleRate*2),tier})};}
const L=(x,z,fx=0,fz=-1)=>({x,y:2.4,z,fx,fz,origin:{x,z}}),off={x:0,z:0,heading:0,on:false},frame=(trams=[],seaTime=0)=>({seaTime,trams:[trams[0]||off,trams[1]||off]});
{ // A tram passes along x at 11 m/s, 2 m from the listener.
  const {fake,c,movers}=rig(),tram={x:-150,z:2,heading:Math.PI/2,on:true};let built=null,approach=0,recede=0,maxClacks=0;
  const motor=()=>fake.nodes.find(n=>n.kind==='oscillator'&&n.type==='sawtooth'&&!n.disconnected);
  for(let k=0;k<30*60;k++){const t=k/60;c.currentTime=t;tram.x=-150+11*t;movers.update(frame([tram]),L(0,0),t,1/60);
    const s=movers.stats().trams[0];if(s.voice&&built===null)built=tram.x;if(Math.abs(tram.x+30)<.1)approach=motor().frequency.value;if(Math.abs(tram.x-30)<.1)recede=motor().frequency.value;
    maxClacks=Math.max(maxClacks,movers.stats().clacks);if(k%6===0)fake.flush();}
  assert.ok(built>-91&&built<-85,'the voice starts within 90 m: '+built);assert.ok(approach>recede&&approach>92&&recede<92,'Doppler '+approach+' / '+recede);
  assert.ok(movers.stats().clacksPlayed>4&&maxClacks<=M.MOVERS.maxClacks,'rail joints');assert.equal(movers.stats().trams[0].voice,false,'released after it left');
  assert.equal(movers.stats().trams[0].squeal,0);movers.dispose();
}
{ // A curve squeals.
  const {c,movers}=rig(),tram={x:0,z:0,heading:0,on:true};let squeal=0;
  for(let k=0;k<6*60;k++){const t=k/60,a=t*8/5.3;c.currentTime=t;tram.x=20+5.3*Math.cos(a);tram.z=5.3*Math.sin(a);tram.heading=Math.atan2(-Math.sin(a),Math.cos(a));
    movers.update(frame([tram]),L(10,0),t,1/60);squeal=Math.max(squeal,movers.stats().trams[0].squeal);}
  assert.ok(squeal>.5*M.MOVERS.squeal,'a 5.3 m curve at 8 m/s squeals: '+squeal);movers.dispose();
}
for(const [tier,n]of[['low',2],['medium',4],['high',4]]){ // The nearest hover robots win a voice.
  const {c,movers}=rig(tier);for(let i=0;i<12;i++)movers.gait('patrol-'+i,2+i*3,1.6,0,true,'glide');movers.gait('resident-0',1,0,0,true,'');
  for(let k=0;k<30;k++){c.currentTime=k/60;movers.update(frame(),L(0,0),k/60,1/60);}
  assert.deepEqual(movers.stats().hovers,['patrol-0','patrol-1','patrol-2','patrol-3'].slice(0,n),tier);assert.equal(movers.stats().gliders,12,'walking residents are not gliders');movers.dispose();
}
{ // Boats.
  const high=rig('high');high.movers.update(frame([],0),L(-20,77,0,1),0,1/60);assert.deepEqual([high.movers.stats().launch,high.movers.stats().barge],[true,true]);high.movers.dispose();
  const low=rig('low');low.movers.update(frame([],0),L(-20,77,0,1),0,1/60);assert.equal(low.movers.stats().barge,false,'low: no barge');
  const inland=rig('high');inland.movers.update(frame([],0),L(0,-60),0,1/60);assert.equal(inland.movers.stats().launch,false,'inland: no engines');
}
{ // Gulls call from the flock.
  const {movers}=rig('high');let s=1;const r=()=>(s=(Math.imul(s,1664525)+1013904223)>>>0)/4294967296,spot={};
  for(let i=0;i<50;i++){const p=movers.gull({seaTime:i*3},r,spot);assert.ok(p===spot&&p.y>=11.5&&Math.hypot(p.x-HARBOUR_SKY.x,p.z-HARBOUR_SKY.z)<300,'over the harbour or the barge');}
  assert.equal(rig('low').movers.gull({seaTime:0},r,spot),null);
}
{ // Nothing moves: nothing is scheduled; dispose releases everything.
  const {fake,c,movers}=rig(),tram={x:-30,z:2,heading:0,on:true};for(let k=0;k<120;k++){c.currentTime=k/60;movers.update(frame([tram]),L(0,0),k/60,1/60);}
  const ramps=()=>fake.nodes.reduce((n,x)=>n+x.gain.ramp.length+x.frequency.ramp.length+x.pan.ramp.length,0),r0=ramps();
  for(let k=120;k<240;k++){c.currentTime=k/60;movers.update(frame([tram]),L(0,0),k/60,1/60);}assert.equal(ramps(),r0,'a standing tram and listener schedule nothing');
  movers.dispose();assert.equal(fake.live().filter(n=>!['destination','outdoor','send'].includes(n.kind)).length,0);
}
console.log('System World sound movers: tram pass with Doppler, clacks and release, curve squeal, hover voices per tier, boats and gulls verified');
```

**Run:** `node scripts/test-system-world-sound-movers.mjs` → PASS.

**Commit:** `feat(system-world): moving sound sources (tram hum, squeal and clacks, hover whirs, launch and barge, gull positions) with their test`

---

## Task 7 — `sysworld-sound-rooms.js` (room tones)

**Files:** new `ui/js/desktop/apps/sysworld-sound-rooms.js`, new `scripts/test-system-world-sound-rooms.mjs`.

### `ui/js/desktop/apps/sysworld-sound-rooms.js` (complete)

```js
// Room tones (System World beauty pass, phase 5): one synthesised bed per kind of room plus sparse room events on the
// audio clock. Only the tone of the room the listener is in, or of the hall whose open door it stands at, is alive;
// a switch fades over FADE seconds and releases the old tone when it has ended. Low tier: the first bed of a tone, no
// events. Levels are first estimates, tuned by the sound survey (allowed range x0.5..x2 per tone).
export const FADE=.6,MAX_ROOM_EVENTS=6;
export const ROOM_TONES=Object.freeze({
  // Agent core: reactor hum that breathes (stronger while the agent is busy), soft interface tones.
  agent:{beds:[{wave:'sine',f:49,gain:.05,lfo:[.11,.3],busy:1.4},{wave:'sine',f:98,gain:.025,busy:1.4},{wave:'sine',f:147,gain:.012},{wave:'noise',filter:['lowpass',300,.5],gain:.04}],
    events:[{kind:'blip',every:[3,8],gain:.012,notes:[1318.5,1568,1760,2093],dur:.08,double:.3}]},
  // Archive: near silence, page-like ticks in pairs.
  archive:{beds:[{wave:'noise',filter:['lowpass',220,.5],gain:.02},{wave:'sine',f:60,gain:.004}],
    events:[{kind:'tick',every:[1.5,6],gain:.015,f:3500,q:1.2,dur:.015,count:[2,2],spacing:[.06,.12]}]},
  // Gate halls: mains hum, relay clicks, patch tones.
  gate:{beds:[{wave:'sine',f:60,gain:.012},{wave:'sine',f:120,gain:.008},{wave:'noise',filter:['bandpass',1000,.7],gain:.008}],
    events:[{kind:'tick',every:[1,4],gain:.02,f:4000,q:2,dur:.005,count:[2,4],spacing:[.04,.09]},{kind:'blip',every:[4,10],gain:.01,notes:[660,880,990],dur:.12,double:0}]},
  // Departure hall: belt rumble, hall murmur, split-flap boards, a chime before (wordless) announcements.
  departure:{beds:[{wave:'noise',filter:['lowpass',120,.7],gain:.05,lfo:[.8,.3]},{wave:'sine',f:31,gain:.02},{wave:'noise',filter:['bandpass',500,.4],gain:.015}],
    events:[{kind:'tick',every:[12,25],gain:.014,f:2200,q:1.5,dur:.006,count:[8,20],spacing:[.04,.04]},{kind:'chime',every:[40,70],gain:.02,notes:[1046.5,1318.5,1568],step:.35,murmur:2.2}]},
  // Machine hall: fans, blade pass, coolant flow, mains, an occasional clunk.
  machine:{beds:[{wave:'noise',filter:['bandpass',180,1],gain:.06},{wave:'sine',f:87,gain:.02},{wave:'sine',f:174,gain:.01},{wave:'noise',filter:['bandpass',900,.6],gain:.02,lfo:[.3,.5]},{wave:'sine',f:50,gain:.015}],
    events:[{kind:'clunk',every:[8,20],gain:.03,dur:.06}]},
  // Garden court (open to the sky; leaves come from the green zone): water trickle and drops.
  court:{beds:[{wave:'noise',filter:['bandpass',2400,2],gain:.012,lfo:[5.3,.4]},{wave:'noise',filter:['bandpass',1400,1.2],gain:.006,lfo:[7.1,.3]}],
    events:[{kind:'drop',every:[.7,2.5],gain:.01,from:[1200,1800],to:600,dur:.06}]},
  // Waterfront pavilions: quiet air handling.
  pavilion:{beds:[{wave:'noise',filter:['lowpass',400,.5],gain:.015},{wave:'sine',f:120,gain:.003}],events:[]},
});
export function createRoomTones(context,{room,send,noise,random=Math.random,tier='high'}){
  let level=tier,current=null,busy=false,fired=0,switches=0;const events=new Set(),fading=new Set(),span=([a,b])=>a+(b-a)*random();
  function build(name,now){
    const spec=ROOM_TONES[name],nodes=[],own=n=>(nodes.push(n),n),out=own(context.createGain()),wet=own(context.createGain());
    out.gain.setValueAtTime(0,now);out.gain.linearRampToValueAtTime(1,now+FADE);out.connect(room);wet.gain.value=.5;out.connect(wet).connect(send);
    const beds=[];
    for(const b of level==='low'?spec.beds.slice(0,1):spec.beds){
      let src;if(b.wave==='noise'){src=own(context.createBufferSource());src.buffer=noise;src.loop=true;src.playbackRate.value=.8+.4*random();}
      else{src=own(context.createOscillator());src.type=b.wave==='saw'?'sawtooth':'sine';src.frequency.value=b.f;}
      const g=own(context.createGain());g.gain.value=b.gain*(busy&&b.busy?b.busy:1);let head=src;
      if(b.filter){const f=own(context.createBiquadFilter());f.type=b.filter[0];f.frequency.value=b.filter[1];f.Q.value=b.filter[2];head.connect(f);head=f;}
      head.connect(g).connect(out);
      if(b.lfo){const o=own(context.createOscillator()),d=own(context.createGain());o.frequency.value=b.lfo[0];d.gain.value=b.gain*b.lfo[1];o.connect(d).connect(g.gain);o.start(now);}
      src.start(now);beds.push({g,spec:b});
    }
    const next=new Float64Array(level==='low'?0:spec.events.length);for(let i=0;i<next.length;i++)next[i]=now+span(spec.events[i].every);
    switches++;return {name,spec,out,nodes,beds,next};
  }
  function drop(tone,now){const g=tone.out.gain;g.cancelScheduledValues(now);g.setValueAtTime(g.value,now);g.linearRampToValueAtTime(0,now+FADE);fading.add(tone);
    let last=null;for(const n of tone.nodes)if(n.stop){try{n.stop(now+FADE+.05);}catch(_){}last=n;}
    const finish=()=>{if(!fading.delete(tone))return;for(const n of tone.nodes)try{n.disconnect();}catch(_){}};if(last)last.onended=finish;else finish();}
  function voice(list){const entry={release(){if(!events.delete(entry))return;for(const n of list){try{n.stop?.();}catch(_){}try{n.disconnect();}catch(_){}}}};events.add(entry);return entry;}
  function tone(f,at,dur,gain,out,list,to=0){const o=context.createOscillator(),g=context.createGain();o.type='sine';o.frequency.setValueAtTime(f,at);if(to)o.frequency.exponentialRampToValueAtTime(to,at+dur);
    g.gain.setValueAtTime(0,at);g.gain.linearRampToValueAtTime(gain,at+.005);g.gain.exponentialRampToValueAtTime(.0001,at+dur+.05);o.connect(g).connect(out);o.start(at);o.stop(at+dur+.08);list.push(o,g);return o;}
  function burst(f,q,at,dur,gain,out,list,type='bandpass'){const s=context.createBufferSource(),b=context.createBiquadFilter(),g=context.createGain();s.buffer=noise;s.loop=true;b.type=type;b.frequency.value=f;b.Q.value=q;
    g.gain.setValueAtTime(0,at);g.gain.linearRampToValueAtTime(gain,at+.001);g.gain.exponentialRampToValueAtTime(.0001,at+dur);s.connect(b).connect(g).connect(out);s.start(at);s.stop(at+dur+.02);list.push(s,b,g);return s;}
  function fire(e,at,out){
    const list=[];let end=null;fired++;
    if(e.kind==='blip'){end=tone(e.notes[Math.floor(random()*e.notes.length)],at,e.dur,e.gain,out,list);if(random()<e.double)end=tone(e.notes[Math.floor(random()*e.notes.length)],at+e.dur+.04,e.dur,e.gain*.8,out,list);}
    else if(e.kind==='tick'){const n=e.count[0]+Math.floor(random()*(e.count[1]-e.count[0]+1));let t=at;for(let i=0;i<n;i++){end=burst(e.f,e.q,t,e.dur,e.gain,out,list);t+=span(e.spacing);}}
    else if(e.kind==='chime'){e.notes.forEach((f,i)=>{end=tone(f,at+i*e.step,.9,e.gain,out,list);});
      if(e.murmur){const t=at+e.notes.length*e.step+.2,s=context.createBufferSource(),b=context.createBiquadFilter(),g=context.createGain(),am=context.createOscillator(),depth=context.createGain();
        s.buffer=noise;s.loop=true;b.type='bandpass';b.frequency.value=700;b.Q.value=4;am.frequency.value=4;depth.gain.value=e.gain*.2;
        g.gain.setValueAtTime(0,t);g.gain.linearRampToValueAtTime(e.gain*.4,t+.3);g.gain.setValueAtTime(e.gain*.4,t+e.murmur-.3);g.gain.linearRampToValueAtTime(0,t+e.murmur);
        am.connect(depth).connect(g.gain);s.connect(b).connect(g).connect(out);s.start(t);am.start(t);s.stop(t+e.murmur+.05);am.stop(t+e.murmur+.05);list.push(s,b,g,am,depth);end=s;}}
    else if(e.kind==='clunk')end=burst(300,.7,at,e.dur,e.gain,out,list,'lowpass');
    else if(e.kind==='drop')end=tone(e.from[0]+(e.from[1]-e.from[0])*random(),at,e.dur,e.gain,out,list,e.to);
    if(end)end.onended=voice(list).release;
  }
  return {
    // w: tracker weights (tone inside, leakTone at an open door); now: context seconds.
    update(w,now){
      const want=w.tone||w.leakTone||null;
      if((current?current.name:null)!==want){if(current){drop(current,now);current=null;}if(want&&ROOM_TONES[want])current=build(want,now);}
      if(!current)return;
      for(let i=0;i<current.next.length;i++)if(now>=current.next[i]){const e=current.spec.events[i];current.next[i]=now+span(e.every);if(events.size<MAX_ROOM_EVENTS)fire(e,now,current.out);}
    },
    setMood(m){const b=!!m?.busy;if(b===busy)return;busy=b;if(current)for(const bed of current.beds)if(bed.spec.busy)bed.g.gain.setTargetAtTime(bed.spec.gain*(busy?bed.spec.busy:1),context.currentTime,1);},
    setTier(value){if(value===level||!['low','medium','high','ultra'].includes(value))return;const change=(value==='low')!==(level==='low');level=value;
      if(change&&current){const name=current.name;drop(current,context.currentTime);current=build(name,context.currentTime);}},
    clear(){for(const e of[...events])e.release();},
    stats:()=>({tier:level,tone:current?current.name:null,events:events.size,fired,switches,fading:fading.size,nodes:(current?current.nodes.length:0)+[...fading].reduce((n,t)=>n+t.nodes.length,0)}),
    dispose(){for(const e of[...events])e.release();for(const t of[current,...fading])if(t)for(const n of t.nodes){try{n.stop?.();}catch(_){}try{n.disconnect();}catch(_){}}current=null;fading.clear();},
  };
}
```

### `scripts/test-system-world-sound-rooms.mjs` (complete)

```js
// Phase 5 room tones: the table covers every room of sysworld-zones.js, events on the audio clock, bounded voices,
// switching and fading, the leak at an open door, the low tier, dispose.
import assert from 'node:assert/strict';
import {createFakeAudio} from './system-world-audio-fake.mjs';
import * as R from '../ui/js/desktop/apps/sysworld-sound-rooms.js';
import {ROOMS} from '../ui/js/desktop/apps/sysworld-zones.js';
for(const r of Object.values(ROOMS))assert.ok(R.ROOM_TONES[r.tone],'tone '+r.tone);
for(const [name,t]of Object.entries(R.ROOM_TONES)){assert.ok(t.beds.length>=1,name);
  for(const b of t.beds)assert.ok(['sine','saw','noise'].includes(b.wave)&&b.gain>0&&b.gain<=.08,name+' bed');
  for(const e of t.events)assert.ok(['blip','tick','chime','clunk','drop'].includes(e.kind)&&e.every[0]>0&&e.every[1]>=e.every[0],name+' event '+e.kind);}
const seeded=s=>()=>(s=(Math.imul(s,1664525)+1013904223)>>>0)/4294967296;
function rig(tier='high'){const fake=createFakeAudio(),c=fake.context,room=fake.node('room'),send=fake.node('send');return {fake,c,tones:R.createRoomTones(c,{room,send,noise:c.createBuffer(1,c.sampleRate),random:seeded(9),tier})};}
const W=(tone=null,leakTone=null)=>({tone,leakTone});
{
  const {fake,c,tones}=rig();let maxEvents=0;
  for(let k=0;k<60*25;k++){const t=k/25;c.currentTime=t;tones.update(W('agent'),t);maxEvents=Math.max(maxEvents,tones.stats().events);if(k%3===0)fake.flush();}
  const s=tones.stats();assert.equal(s.tone,'agent');assert.ok(s.fired>=7&&s.fired<=21,'agent blips in a minute: '+s.fired);assert.ok(maxEvents<=R.MAX_ROOM_EVENTS);
  tones.update(W('machine'),60);assert.deepEqual([tones.stats().tone,tones.stats().fading],['machine',1]);fake.flush();assert.equal(tones.stats().fading,0,'the old tone is released after its fade');
  tones.update(W(null,'departure'),61);assert.equal(tones.stats().tone,'departure','an open door lets the hall be heard');
  tones.update(W(),62);fake.flush();assert.equal(tones.stats().tone,null);assert.equal(tones.stats().nodes,0);
  tones.dispose();assert.equal(fake.live().filter(n=>!['destination','room','send'].includes(n.kind)).length,0);
}
{
  const {fake,c,tones}=rig('low');tones.update(W('agent'),0);assert.equal(fake.live('oscillator').length+fake.live('source').length,2,'low: the first bed only (its source and its LFO)');
  for(let k=0;k<60*25;k++){c.currentTime=k/25;tones.update(W('agent'),k/25);}assert.equal(tones.stats().fired,0,'low: no room events');
}
console.log('System World room tones: table, events, voices, switching, leak, low tier and dispose verified');
```

**Run:** `node scripts/test-system-world-sound-rooms.mjs` → PASS.

**Commit:** `feat(system-world): synthesised room tones for the lobbies and pavilions with their test`

---

## Task 8 — Footfalls by surface and companion steps (`sysworld-foley.js`)

**Files:** `ui/js/desktop/apps/sysworld-foley.js`, `scripts/test-system-world-foley.mjs` (append).

**Steps**
- [ ] Append the test block. Run → **FAIL**.
- [ ] Edit the foley. Run → PASS: the whole file, including the old asserts.

### Test block
Append inside the `try`, directly before `console.log('System World city sound: …`:

```js
  // --- Phase 5: footfalls by surface, the companion, the low tier --------------------------------------------------
  {
    const {STEP_VOICES}=await import('../ui/js/desktop/apps/sysworld-foley.js');
    const {SURFACE_KINDS}=await import('../ui/js/desktop/apps/sysworld-zones.js');
    assert.deepEqual(Object.keys(STEP_VOICES).sort(),[...SURFACE_KINDS].sort(),'every surface has a footfall voice');
    for(const [k,v]of Object.entries(STEP_VOICES))assert.ok(v.gain>=.06&&v.gain<=.09&&v.layers.length>=2,'step voice '+k);
    const f=createCityFoley(context,node('out'),node('send'));f.setActive(true);
    const spots={road:[-18,-.14,-50],kerb:[-14.63,0,-50],pavement:[-13.5,0,-50],stairs:[0,.5,.3],quay:[-20,.08,77],lobby:[0,.88,-12],grating:[-43,.88,-56],garden:[-44,0,-88]};
    for(const [surface,[x,y,z]]of Object.entries(spots)){f.listener(x+2,y,z,0,-1,x+2,z);assert.equal(f.step(x,y,z,'v-'+surface),true,surface);assert.equal(f.stats().bySurface[surface],1,surface+' counted');flush();}
    // A carried stride lowers the voice by the CARRY pitch (0.85).
    context.currentTime+=1;f.listener(0,0,-50,0,-1,0,-50);const before=nodes.length;f.gait('c',0,0,-50,true,true);f.gait('c',1.2,0,-50,true,true);
    const tone=nodes.slice(before).find(n=>n.kind==='oscillator');assert.ok(tone&&Math.abs(tone.frequency.ramp[0][1]-104*.85)<1e-6,'carried stride on pavement at 88.4 Hz');flush();
    // The companion: two crossings of a diagonal pair per gait cycle, each a front and a hind foot.
    f.listener(30,0,72,0,-1,30,72);const pose={x:30,z:69.5,phase:0,on:true},played=f.stats().played,start=f.stats().companionSteps;
    for(let i=0;i<=40;i++){context.currentTime+=.03;pose.phase=i*Math.PI/10;f.companion(pose);flush();}
    assert.equal(f.stats().companionSteps-start,4,'four pair crossings over two gait cycles');assert.equal(f.stats().played-played,8,'front and hind foot each');
    pose.on=false;f.companion(pose);pose.on=true;pose.phase+=Math.PI*3;f.companion(pose);assert.equal(f.stats().companionSteps-start,4,'a hidden companion is silent and does not catch up');
    f.setTier('low');for(let i=0;i<20;i++)f.action('robot_work',3,0,-50,'L'+i);assert.equal(f.stats().voices,6,'low: six voices');f.setTier('high');flush();f.setActive(false);
  }
```

### `sysworld-foley.js` edits
1. **Import and the surface table.** At the top of the file add `import {surfaceAt} from './sysworld-zones.js';`. Directly before `export function createCityFoley` insert:
   ```js
   // Footfalls by surface (phase 5). The emitter's position decides the surface (sysworld-zones.js surfaceAt), so every
   // kind in SURFACE_KINDS needs a voice here. Gains stay >= .06: a footfall beside the listener must clear the bed.
   export const STEP_VOICES=Object.freeze({
     road:{gain:.07,layers:[{wave:'noise',f:900,q:.8,dur:.06,lvl:.5},{wave:'noise',f:2600,q:1.4,dur:.03,lvl:.25},{wave:'tone',f:90,f2:70,dur:.08,lvl:.25}]},
     pavement:{gain:.07,layers:[{wave:'noise',f:1900,q:1.7,dur:.05,lvl:.5},{wave:'noise',f:520,q:.9,dur:.1,lvl:.4},{wave:'tone',f:104,f2:74,dur:.12,lvl:.3}]},
     kerb:{gain:.072,layers:[{wave:'noise',f:2400,q:2,dur:.03,lvl:.6},{wave:'noise',f:700,q:1,dur:.07,lvl:.3},{wave:'tone',f:180,f2:120,dur:.06,lvl:.35}]},
     stairs:{gain:.075,layers:[{wave:'noise',f:1700,q:1.3,dur:.04,lvl:.5},{wave:'noise',f:600,q:.8,dur:.11,lvl:.4},{wave:'tone',f:140,f2:95,dur:.09,lvl:.35},{wave:'noise',f:3800,q:1,dur:.05,lvl:.15,delay:.06}]},
     quay:{gain:.075,layers:[{wave:'tone',f:170,f2:130,dur:.14,lvl:.45,type:'triangle'},{wave:'noise',f:700,q:2.5,dur:.12,lvl:.35},{wave:'noise',f:2000,q:1,dur:.03,lvl:.2}]},
     lobby:{gain:.068,layers:[{wave:'noise',f:3200,q:2.2,dur:.035,lvl:.55},{wave:'noise',f:1100,q:1.4,dur:.07,lvl:.3},{wave:'tone',f:220,f2:160,dur:.07,lvl:.2}]},
     grating:{gain:.07,layers:[{wave:'noise',f:3000,q:3,dur:.025,lvl:.45},{wave:'tone',f:820,dur:.18,lvl:.18},{wave:'tone',f:1370,dur:.14,lvl:.12},{wave:'tone',f:95,f2:70,dur:.08,lvl:.25}]},
     garden:{gain:.062,layers:[{wave:'noise',f:4200,q:.7,dur:.09,lvl:.4},{wave:'noise',f:1800,q:.6,dur:.07,lvl:.3},{wave:'noise',f:600,q:.8,dur:.06,lvl:.15}]},
   });
   ```
2. **Constants.** `const MAX_VOICES=10,MAX_DISTANCE=48,RETRIGGER=.25;` →
   ```js
     const MAX_DISTANCE=48,RETRIGGER=.25,CARRY={pitch:.85,length:1.2,scale:1.07},FRONT={scale:.35,pitch:1.6,length:.6},HIND={scale:.3,pitch:1.5,length:.6,delay:.025};
     let maxVoices=10,companionBeat=null,companionSteps=0;const bySurface=Object.fromEntries(Object.keys(STEP_VOICES).map(k=>[k,0]));
   ```
3. **`voice()`.**
   - `function voice(kind,x,y,z,id){` → `function voice(kind,x,y,z,id,surface,mod){`
   - `const spec=SOUNDS[name];` → `const spec=surface?STEP_VOICES[surface]:SOUNDS[name];`
   - `if(voices.size>=MAX_VOICES){` → `if(voices.size>=maxVoices){`
   - `const level=spec.gain*LAYER/(1+distance*.12)` → `const level=spec.gain*LAYER*(mod?.scale??1)/(1+distance*.12)`
   - `const at=now+(layer.delay||0),dur=Math.max(layer.dur,(layer.attack||.006)+.02);` → `const at=now+(layer.delay||0)+(mod?.delay||0),dur=Math.max(layer.dur*(mod?.length??1),(layer.attack||.006)+.02);`
   - `band.frequency.value=layer.f;` → `band.frequency.value=layer.f*(mod?.pitch??1);`
   - `source.frequency.setValueAtTime(layer.f,at);` → `source.frequency.setValueAtTime(layer.f*(mod?.pitch??1),at);`
   - `Math.max(20,layer.f2)` → `Math.max(20,layer.f2*(mod?.pitch??1))`
   - After `played++;` add `if(surface)bySurface[surface]++;`
4. **`gait()`.** `else voice(carrying?'step_carry':'step',x,y,z,id);` → `else voice('step',x,y,z,id,surfaceAt(x,y,z),carrying?CARRY:null);`
5. **`companion()`.** After `function clear(){…}` insert:
   ```js
     // The companion trots (phase 5): when a diagonal pair lands (phase passes a multiple of pi), its front and hind foot
     // sound 25 ms apart. A hidden or resumed companion resyncs silently.
     function companion(p){
       if(!active||!p||!p.on){companionBeat=null;return;}
       const beat=Math.floor(p.phase/Math.PI);
       if(companionBeat===null||beat<companionBeat||beat-companionBeat>2){companionBeat=beat;return;}
       if(beat===companionBeat)return;
       companionBeat=beat;companionSteps++;const surface=surfaceAt(p.x,0,p.z);
       voice('step',p.x,0,p.z,'companion-front',surface,FRONT);voice('step',p.x,0,p.z,'companion-hind',surface,HIND);
     }
   ```
6. **Returned object.**
   - Add `step(x,y,z,id,mod){return voice('step',x,y,z,id,surfaceAt(x,y,z),mod);},companion,setTier(value){maxVoices=value==='low'?6:10;},`.
   - In `stats`, add `companionSteps,bySurface:{...bySurface},` after `steps,`.

**Run:** `node scripts/test-system-world-foley.mjs` → PASS.

**Commit:** `feat(system-world): footfalls by surface for residents and visitor, light companion trot, six foley voices on low`

---

## Task 9 — Scene input: sound frame, forwarding, door and gait accessors

**Files:**
- new `ui/js/desktop/apps/sysworld-sound-frame.js`
- new `scripts/test-system-world-sound-frame.mjs`
- `ui/js/desktop/apps/sysworld-scene.js`
- `ui/js/desktop/apps/sysworld.js`
- `ui/js/desktop/apps/sysworld-experience.js`
- `ui/js/desktop/apps/sysworld-companion.js`

### `ui/js/desktop/apps/sysworld-sound-frame.js` (complete)

```js
// The scene's per-frame input for the soundscape (System World beauty pass, phase 5). One object, filled in place every
// frame: both clocks, the wind (the atmosphere's, or windAt on the app clock when the atmosphere zeroes it for the low
// tier or reduced motion, so the leaves keep sounding), the two trams from traffic, the companion's gait and a stable
// door-openness function. sysworld.js forwards it to sysworld-audio.js `frame()`. No allocation in fill().
import {windAt} from './sysworld-atmosphere.js';
const TRAMS=Object.freeze(['tram-0','tram-1']);
export function createSoundFrame({traffic=null,experience=null,companion=null}={}){
  const doorValue=key=>experience&&experience.doorValue?Number(experience.doorValue(key))||0:0,fallback={x:1,z:0,strength:0};
  const frame={time:0,seaTime:0,wind:{x:1,z:0,strength:0},trams:TRAMS.map(()=>({x:0,z:0,heading:0,on:false})),companion:{x:0,z:0,phase:0,on:false},doorValue,reduced:false};
  function fill(elapsed,seaTime,wind,weather,reduced){
    frame.time=Number(elapsed)||0;frame.seaTime=Number(seaTime)||0;frame.reduced=!!reduced;
    const w=wind&&wind.strength>0?wind:windAt(frame.time,weather,fallback);frame.wind.x=w.x;frame.wind.z=w.z;frame.wind.strength=w.strength;
    for(let i=0;i<TRAMS.length;i++){const b=traffic&&traffic.body?traffic.body(TRAMS[i]):null,t=frame.trams[i];t.on=!!b&&b.enabled!==false;if(b){t.x=b.x;t.z=b.z;t.heading=b.heading||0;}}
    if(companion&&companion.soundPose)companion.soundPose(frame.companion);else frame.companion.on=false;
    return frame;
  }
  return {frame,fill};
}
```

### `scripts/test-system-world-sound-frame.mjs` (complete)

```js
// Phase 5 sound frame: copies, the wind fallback, stable objects, door routing and missing parts.
import assert from 'node:assert/strict';
import {createSoundFrame} from '../ui/js/desktop/apps/sysworld-sound-frame.js';
import {windAt} from '../ui/js/desktop/apps/sysworld-atmosphere.js';
const bodies=new Map([['tram-0',{x:10,z:-77,heading:1.2,enabled:true}],['tram-1',{x:0,z:0,heading:0,enabled:false}]]);
const s=createSoundFrame({traffic:{body:id=>bodies.get(id)},experience:{doorValue:key=>key==='pavilion:agent'?.4:key==='lobby:agent:0'?.9:0},
  companion:{soundPose(out){out.on=true;out.x=30;out.z=69.5;out.phase=3;return out;}}}),f=s.frame;
assert.equal(s.fill(12,5,{x:.5,z:.8,strength:.6},'clear',false),f,'fill returns the reused frame');
assert.deepEqual([f.time,f.seaTime,f.wind.strength,f.trams[0].on,f.trams[0].x,f.trams[0].heading,f.trams[1].on,f.companion.phase],[12,5,.6,true,10,1.2,false,3]);
assert.equal(f.doorValue('pavilion:agent'),.4);assert.equal(f.doorValue('lobby:agent:0'),.9);
const trams=f.trams,wind=f.wind;s.fill(13,5,{x:1,z:0,strength:0},'rain',true);
assert.ok(Math.abs(f.wind.strength-windAt(13,'rain',{}).strength)<1e-12,'a zeroed atmosphere wind (low tier, reduced motion) keeps the wind sound');
assert.equal(f.trams,trams);assert.equal(f.wind,wind);assert.equal(f.reduced,true);
const bare=createSoundFrame({});bare.fill(1,1,null,'clear',false);
assert.deepEqual([bare.frame.trams[0].on,bare.frame.companion.on,bare.frame.doorValue('lobby:agent:0')],[false,false,0]);
console.log('System World sound frame: clocks, wind fallback, trams, companion, doors and stable objects verified');
```

### Edits
- **`sysworld-scene.js`.**
  1. After `import { createParticles } from './sysworld-particles.js';` add `import { createSoundFrame } from './sysworld-sound-frame.js';`.
  2. `let moodKey = '', nearHidden = null;` → `let moodKey = '', nearHidden = null, soundFrame = null;`
  3. In `update()`, after `particles.setMood(mood.day, mood.weather); particles.update(dt, !reduced, wind);` insert:
     ```js
         // Soundscape (phase 5): one reused frame after the listener (onListener above) and the wind; the audio reads the
         // clocks, trams, the companion's gait, door openness and wind from it. Nothing here allocates.
         if (soundFrame) options.onSoundFrame?.(soundFrame.fill(elapsed, seaTime, wind, mood.weather, reduced));
     ```
  4. After the statement `companion = createCompanion({…});` add `soundFrame = createSoundFrame({ traffic, experience, companion });`.
- **`sysworld.js`** (in `loadCity`).
  - Append `onSoundFrame:frame=>inst.sound?.frame(frame),` to the line `onThunder:delay=>inst.sound?.thunder(delay),onMood:mood=>inst.sound?.setMood(mood),`.
  - Replace `onQuality:(value,actual)=>{if(!inst.disposed)inst.hud.quality(value,actual);},` with `onQuality:(value,actual)=>{if(!inst.disposed){inst.hud.quality(value,actual);inst.sound?.setTier(actual);}},`.
- **`sysworld-experience.js`.**
  - Before `  return {update,interact,endRide,setTier,society,lobbies:lobbyDoors,` insert:
    ```js
      // Door openness for the soundscape (phase 5): 'pavilion:<id>' for the waterfront pavilions, lobby door keys otherwise.
      const PAVILION_KEYS=new Map(interiors.map(r=>['pavilion:'+r.id,r.id]));
      function doorValue(key){const id=PAVILION_KEYS.get(key);return id!==undefined?(doors.get(id)?.value||0):lobbyDoors.value(key);}
    ```
  - Change the return's first line to `  return {update,interact,endRide,setTier,society,lobbies:lobbyDoors,doorValue,`.
- **`sysworld-companion.js`.**
  - In the header, replace `// scenery, not an actor: it never joins traffic, telemetry or the robot foley.` with `// scenery, not an actor: it never joins traffic or telemetry; the soundscape reads its gait (soundPose).`
  - After `    ready, update,` add:
    ```js
        // Gait for the soundscape (phase 5): position and phase written into `out`, no allocation.
        soundPose(out) { out.on = !!model && group.visible && !disposed; out.x = group.position.x; out.z = group.position.z; out.phase = time * LOOP.gaitHz * Math.PI * 2; return out; },
    ```

**Run**
- `node scripts/test-system-world-sound-frame.mjs` → PASS
- `node scripts/test-system-world-companion.mjs` → PASS
- `node scripts/test-system-world-living.mjs` → PASS (no behaviour change; report its JSON line)
- `go test ./ui -run '^TestDesktopSysWorld' -count=1` → PASS

**Commit:** `feat(system-world): per-frame sound input from the scene (trams, companion gait, door openness, wind fallback) and tier forwarding`

---

## Task 10 — Engine wiring A: zones, space, glass, gated events, beds by zone, visitor steps

**Files:** `ui/js/desktop/apps/sysworld-audio.js`, `scripts/test-system-world-soundscape.mjs` (append), `scripts/test-system-world-foley.mjs` (one pin).

**Steps**
- [ ] Append the test block. Change the foley pin. Run both → **FAIL**.
- [ ] Edit the engine. Run → PASS.

### Foley test pin
In `scripts/test-system-world-foley.mjs`, replace `  ambience.effect('step',2,0,0);\n  assert.equal(ambience.stats().foley.played,1,'A world interaction never reaches the foley');` with:
```js
  ambience.effect('door',2,0,0);
  assert.equal(ambience.stats().foley.played,1,'A world interaction never reaches the foley');
```

### Test block (append to `scripts/test-system-world-soundscape.mjs`)

```js
// --- Task 10: zones, glass, gating, beds by zone, visitor steps ---------------------------------------------------
{
  const {fake,sound}=city({enabled:true});sound.unlock();sound.setActive(true);
  const run=(x,y,z,fx,fz,f,seconds=2)=>{for(let i=0;i<seconds*60;i++){fake.context.currentTime+=1/60;sound.setListener(x,y,z,fx,fz);sound.frame(f);}};
  run(-13.5,2.4,-12,0,-1,frameOf());let s=sound.stats().soundscape;
  assert.ok(s.zones.canyon>.5&&s.space.glass===0,'street: canyon, no glass '+JSON.stringify(s.zones));
  run(0,3.28,-12,0,1,frameOf());s=sound.stats().soundscape;
  assert.equal(s.zones.room,'lobby:agent');assert.ok(s.space.glass>.95&&s.space.cutoff<1000,'shut lobby '+JSON.stringify(s.space));
  const inside=citySoundscapeEvents({day:1,weather:'clear'},s.zones).map(e=>e.kind);
  assert.ok(inside.length&&inside.every(k=>['chime','horn','bell','siren'].includes(k)),'only glass-passing events indoors: '+inside);
  run(0,3.28,-12,0,1,frameOf({doorValue:()=>1}));s=sound.stats().soundscape;assert.ok(s.space.glass>.2&&s.space.glass<.4,'open doors let the city in '+s.space.glass);
  run(-43,3.28,36,0,1,frameOf());s=sound.stats().soundscape;assert.equal(s.zones.open,true);
  assert.ok(citySoundscapeEvents({day:1,weather:'clear'},s.zones).some(e=>e.kind==='birds'),'the open court keeps its birds');
  run(3,2.4,-1,0,-1,frameOf({doorValue:k=>k==='lobby:agent:0'?1:0}));s=sound.stats().soundscape;
  assert.ok(s.zones.leakRoom==='lobby:agent'&&s.zones.leak>.5&&s.zones.leakPan<0,'the open door on the left leaks');
  run(-20,2.48,77,0,1,frameOf());assert.ok(sound.stats().soundscape.beds.water>.04,'water by the quay proximity');
  const played=sound.stats().foley.played;sound.effect('step',2,2.4,0);
  assert.equal(sound.stats().foley.played,played+1,'the visitor step is a foley voice');assert.equal(sound.stats().foley.bySurface.stairs,1,'on the agent stairs');
  assert.deepEqual(citySoundscapeEvents({day:1,weather:'clear'}),citySoundscapeEvents({day:1,weather:'clear'},null),'without zones the table is unchanged');
  sound.dispose();
}
console.log('System World soundscape: zones, glass, indoor gate, open court, door leak, beds by zone and visitor steps verified');
```

### `sysworld-audio.js` edits
1. **Imports.** After the three existing imports add:
   ```js
   import {zoneWeights,createZoneTracker,doorState} from './sysworld-zones.js';
   import {createSoundSpace} from './sysworld-sound-space.js';
   ```
   After `const MAX_EVENTS=12;` add `const VISITOR_STEP=Object.freeze({scale:.45});`.
2. **Event table.** Replace the block from `// Weighted event table.` through the closing `}` of `citySoundscapeEvents` with:
   ```js
   // Weighted event table. Every entry carries its own mood condition, so the same city sounds different at noon, at
   // night, in rain and while the agent is busy. `zone` scales the weight by where the listener stands (outdoor shares
   // of sysworld-zones.js). Behind glass only `indoor` events stay (loud enough to pass it; the glass damps them), so no
   // bird sings in a machine hall; the open garden court keeps the whole table.
   const EVENTS=[
     {kind:'chime',weight:15,indoor:true},
     {kind:'swoosh',weight:13,zone:z=>.3+1.4*z.canyon+.4*z.plaza},
     {kind:'gust',weight:7,zone:z=>.4+z.height+.6*z.harbour+.4*z.green},
     {kind:'birds',weight:8,when:m=>m.day>.45&&m.weather==='clear',zone:z=>.15+2.2*z.green+.3*z.plaza},
     {kind:'gull',weight:6,when:m=>m.day>.2&&m.weather==='clear',zone:z=>.1+2.6*z.harbour+.6*z.height},
     {kind:'pigeon',weight:4,when:m=>m.day>.6&&!m.busy,zone:z=>.2+1.2*z.plaza+.8*z.canyon},
     {kind:'cricket',weight:6,when:m=>m.day<.25&&m.weather!=='rain',zone:z=>.1+2.4*z.green},
     {kind:'horn',weight:4,when:m=>m.day<.45&&m.weather!=='rain',indoor:true,zone:z=>.5+1.5*z.harbour},
     {kind:'bell',weight:3,when:m=>m.day<.7,indoor:true},
     {kind:'hammer',weight:4,when:m=>m.day>.3,zone:z=>.4+z.canyon+.5*z.plaza},
     {kind:'drip',weight:10,when:m=>m.weather==='rain',zone:z=>.3+z.canyon+.6*z.green},
     {kind:'windchime',weight:4,when:m=>m.weather==='clear'&&!m.busy,zone:z=>.2+1.6*z.harbour},
     {kind:'siren',weight:2,when:m=>m.evening>.3&&m.busy,indoor:true},
   ];
   export const INDOOR_GATE=.5;
   export function eventWeight(e,mood,zones){
     if(e.when&&!e.when(mood))return 0;
     if(zones){if(zones.interior>=INDOOR_GATE&&!zones.open&&!e.indoor)return 0;if(e.zone)return e.weight*e.zone(zones);}
     return e.weight;
   }
   // Exported for the acceptance check: the weighted event table, its mood filter and (with zones) the zone weights.
   export function citySoundscapeEvents(mood={},zones=null) {
     const state={busy:!!mood.busy,day:Number.isFinite(mood.day)?mood.day:0,evening:Number.isFinite(mood.evening)?mood.evening:0,
       weather:['clear','rain','fog'].includes(mood.weather)?mood.weather:'clear'};
     return EVENTS.map(e=>({kind:e.kind,weight:eventWeight(e,state,zones)})).filter(e=>e.weight>0);
   }
   ```
3. **State.** `let waterGain=null,murmurGain=null,lampGain=null,bedWater='0.000',bedMurmur='0.000',bedLamp='0.000';` → `let waterGain=null,murmurGain=null,lampGain=null,bedWater=0,bedMurmur=0,bedLamp=0;`. After `let eventTimer=0,chordTimer=0,chord=0,events=0;` add `let space=null,tracker=null,lastFrame=0,soundFrame=null,beds=null,movers=null,rooms=null;const raw={},door={};`.
4. **`ensure()`.**
   - Replace `    const verbReturn=node(context.createGain());verbReturn.gain.value=.55;verb.connect(verbReturn).connect(ambientBus);` with:
     ```js
         space=createSoundSpace(context,{ambience:ambientBus,verb,tier});tracker=createZoneTracker();
         zoneWeights(listener.x,listener.y,listener.z,listener.origin.x,listener.origin.z,raw);   // builds the zone fields during the gesture
         const verbReturn=node(context.createGain());verbReturn.gain.value=.55;verb.connect(verbReturn).connect(space.outdoor);
     ```
   - `foley=createCityFoley(context,effectsBus,ambientBus);` → `foley=createCityFoley(context,effectsBus,space.sendLocal);foley.setTier(tier);`
   - Route the outdoor beds into `space.outdoor`. In each of the following, replace `.connect(ambientBus)` with `.connect(space.outdoor)`:
     - `air.connect(airFilter).connect(airGain).connect(ambientBus);air.start();`
     - `wash.connect(washBand).connect(waterGain).connect(ambientBus);wash.start();`
     - `crowd.connect(crowdBand).connect(murmurGain).connect(ambientBus);crowd.start();`
     - `lampTrem.connect(lampFilter).connect(lampGain).connect(ambientBus);`

     The sub drone and the pad stay on `ambientBus`.
5. **`out()`.** Replace the function with:
   ```js
     // Outdoor events (phase 5) pass the outdoor bus, so glass damps them indoors, and send into the outdoor spaces.
     function out(level,pan,send){
       const panner=context.createStereoPanner(),reverb=context.createGain();panner.pan.value=pan;reverb.gain.value=send;
       level.connect(panner).connect(space.outdoor);panner.connect(reverb).connect(space.sendOutdoor);return [panner,reverb];
     }
   ```
6. **`pickEvent()` and `event()`.** Replace both with:
   ```js
     function pickEvent(){
       const z=tracker?tracker.weights:null;let total=0;
       for(let i=0;i<EVENTS.length;i++)total+=eventWeight(EVENTS[i],mood,z);
       if(total<=0)return null;
       let roll=random()*total;
       for(let i=0;i<EVENTS.length;i++){const w=eventWeight(EVENTS[i],mood,z);if(w<=0)continue;roll-=w;if(roll<=0)return EVENTS[i].kind;}
       return null;
     }
     function event(){
       if(transient.size>=MAX_EVENTS)return;
       const kind=pickEvent();if(kind)VOICES[kind]?.(positionFor(kind));
     }
   ```
7. **`updateBeds()`.** Replace from `  // The beds follow the listener:` through the end of `function updateBeds(){…}` with:
   ```js
     // The beds follow the zones (sysworld-zones.js): water and the lantern hum by the quay proximity, the murmur by the
     // plaza and canyon shares. Glass (sysworld-sound-space.js) damps them indoors, so nothing here is zeroed for the
     // inside flag. Levels are quantized to 1/1000 so a per-frame listener update queues no ramp while nothing changes.
     function updateBeds(){
       if(!context||!waterGain||!murmurGain||!lampGain||!running())return;
       zoneWeights(listener.x,listener.y,listener.z,listener.origin.x,listener.origin.z,raw);
       const water=Math.round(raw.quay*.05*(weather==='rain'?1.2:1)*1000);
       const crowd=Math.round((mood.busy?.03:.012)*(.35+mood.day*.65)*(.5+.5*Math.min(1,raw.plaza+raw.canyon))*1000);
       const lamp=Math.round(raw.quay*.012*(1-mood.day)*(weather==='rain'?.6:1)*1000);
       if(water===bedWater&&crowd===bedMurmur&&lamp===bedLamp)return;
       bedWater=water;bedMurmur=crowd;bedLamp=lamp;
       const t=context.currentTime;
       waterGain.gain.setTargetAtTime(water/1000,t,.5);murmurGain.gain.setTargetAtTime(crowd/1000,t,.8);lampGain.gain.setTargetAtTime(lamp/1000,t,.8);
     }
   ```
8. **Returned object.**
   - `effect(…)`: replace with
     ```js
         effect(kind,x,y,z,id){if(String(kind).startsWith('robot_')){foley?.action(kind,x,y,z,id);return;}
           // The visitor's own steps (phase 5): a surface voice from the foley at the floor under the eye (2.4 m).
           if(kind==='step'||kind==='step_inside'){foley?.step(x,y-2.4,z,'visitor',VISITOR_STEP);return;}
           effects?.play(kind,x,y,z);},
     ```
   - `frame(f){if(!running()||!f)return;frames++;},` → replace with
     ```js
         frame(f){
           if(!running()||!space||!f)return;
           const now=context.currentTime,dt=lastFrame?Math.min(.25,Math.max(0,now-lastFrame)):0;lastFrame=now;frames++;soundFrame=f;
           if(raw.harbour===undefined)updateBeds();
           doorState(listener.x,listener.y,listener.z,f.doorValue,door);tracker.update(raw,door,dt,listener);space.update(tracker.weights,dt);
         },
     ```
   - `setTier(…)`: replace with `setTier(value){if(!['low','medium','high','ultra'].includes(value)||value===tier)return;tier=value;space?.setTier(value);foley?.setTier(value);},`.
   - `stats()`:
     - in `beds:{water:Number(bedWater),murmur:Number(bedMurmur),lamp:Number(bedLamp)}` → `beds:{water:bedWater/1000,murmur:bedMurmur/1000,lamp:bedLamp/1000}`;
     - add `,zones:tracker?{...tracker.weights}:null,space:space?space.stats():null` inside `soundscape:{…}` after the `listener` entry;
     - `graph:{nodes:nodes.length,` → `graph:{nodes:nodes.length+(space?space.stats().nodes:0),`.
   - `dispose()`: insert `space?.dispose();` directly before `nodes.forEach(n=>n.disconnect());`.

**Run**
- `node scripts/test-system-world-soundscape.mjs` → PASS
- `node scripts/test-system-world-foley.mjs` → PASS
- `go test ./ui -run '^TestDesktopSysWorldAppMarkers$' -count=1` → PASS

**Commit:** `feat(system-world): sound zones, glass damping and the three reverb spaces in the engine; zone-weighted events, beds by zone, surface steps for the visitor`

---

## Task 11 — Engine wiring B: beds, movers, room tones, tiers and the budget

**Files:** `ui/js/desktop/apps/sysworld-audio.js`, `scripts/test-system-world-soundscape.mjs` (append).

### Test block (append)

```js
// --- Task 11: layers, tiers and the budget -------------------------------------------------------------------------
const budget={};
for(const tier of['low','medium','high']){
  const {fake,sound}=city({enabled:true,tier});sound.unlock();sound.setActive(true);
  const trams=[{x:-18,z:-40,heading:0,on:true},{x:-18,z:20,heading:Math.PI,on:true}];
  for(let i=0;i<180;i++){fake.context.currentTime+=1/60;for(const t of trams)t.z+=(t.heading?-1:1)*11/60;
    for(let k=0;k<12;k++)sound.gait('patrol-'+k,-13.5+(k%2?1:-1)*(2+k*3),1.6,-12,true,false,'glide');
    sound.setListener(-13.5,2.4,-12,0,-1);sound.frame(frameOf({trams}));fake.flush();}
  const st=sound.stats(),live=fake.live();
  budget[tier]={nodes:st.graph.nodes,live:live.length,sources:live.filter(n=>n.kind==='source'||n.kind==='oscillator').length,reverbs:live.filter(n=>n.kind==='reverb').length,
    hovers:st.soundscape.layers.movers.hovers.length,trams:st.soundscape.layers.movers.trams.filter(t=>t.voice).length,foley:st.foley.voices,events:st.soundscape.transient};
  assert.ok(budget[tier].foley<=(tier==='low'?6:10)&&budget[tier].events<=12);
  sound.dispose();assert.equal(fake.live().filter(n=>n.kind!=='destination').length,0,tier+': dispose releases every node');
}
assert.deepEqual([budget.low.hovers,budget.medium.hovers,budget.high.hovers],[2,4,4],'hover voices per tier');
assert.deepEqual([budget.low.trams,budget.high.trams],[1,2],'tram voices per tier');
assert.equal(budget.low.reverbs,1,'low: the open-air verb only');assert.ok(budget.medium.reverbs<=2&&budget.high.reverbs<=3);
assert.ok(budget.low.live<budget.high.live&&budget.high.live<=320&&budget.low.live<=220,'node budget '+JSON.stringify(budget));
console.table(budget);
console.log('System World soundscape: layers, tiers, voice caps and node budget verified');
```

### `sysworld-audio.js` edits
1. **Imports.**
   ```js
   import {createSoundBeds} from './sysworld-sound-beds.js';
   import {createSoundMovers} from './sysworld-sound-movers.js';
   import {createRoomTones} from './sysworld-sound-rooms.js';
   ```
   After `const VISITOR_STEP=…;` add `const gullSpot={x:0,y:0,z:0,heading:0,bank:0,flap:0};`.
2. **`ensure()`.** After `    lamp.start();harmonic.start();` insert:
   ```js
       // Phase 5 layers: zone-shaped beds, moving sources, room tones (outdoor ones behind the glass).
       beds=createSoundBeds(context,{outdoor:space.outdoor,send:space.sendOutdoor,local:ambientBus,noise,random,tier});beds.setMood(mood);
       movers=createSoundMovers(context,{outdoor:space.outdoor,send:space.sendOutdoor,noise,tier});
       rooms=createRoomTones(context,{room:space.room,send:space.roomSend,noise,random,tier});rooms.setMood(mood);
   ```
3. **`positionFor()`.** Replace `if(kind==='horn'||kind==='gull')return ANCHORS.harbour;` with `if(kind==='gull')return movers?.gull(soundFrame,random,gullSpot)||ANCHORS.harbour;if(kind==='horn')return ANCHORS.harbour;`.
4. **`quiet()`.** Before its closing `}` add `beds?.clear();movers?.clear();rooms?.clear();`.
5. **Returned object.**
   - `gait(…)`: `gait(id,x,y,z,moving,carrying,style){foley?.gait(id,x,y,z,moving,carrying,style);movers?.gait(id,x,y,z,moving,style);},`
   - In `frame()`, after `space.update(tracker.weights,dt);` append `beds.update(f,tracker.weights,listener,now);movers.update(f,listener,now,dt);rooms.update(tracker.weights,now);foley?.companion(f.companion);`.
   - `setMood(…)`: before the final `applyMood();updateBeds();` insert `beds?.setMood(mood);rooms?.setMood(mood);`.
   - `setTier(…)`: append `beds?.setTier(value);movers?.setTier(value);rooms?.setTier(value);` before the closing `},`.
   - `stats()`: `graph:{nodes:nodes.length+(space?space.stats().nodes:0),` → `graph:{nodes:nodes.length+(space?space.stats().nodes:0)+(beds?beds.stats().nodes:0)+(movers?movers.stats().nodes:0)+(rooms?rooms.stats().nodes:0),`. Add `,layers:{beds:beds?.stats()||null,movers:movers?.stats()||null,rooms:rooms?.stats()||null}` after `space:…` inside `soundscape`.
   - `dispose()`: before `space?.dispose();` insert `beds?.dispose();movers?.dispose();rooms?.dispose();`.

**Run**
- `node scripts/test-system-world-soundscape.mjs` → PASS (report the `console.table`)
- `node scripts/test-system-world-foley.mjs` → PASS
- All the new unit tests → PASS

**Commit:** `feat(system-world): beds, trams, hover robots, boats, gulls and room tones in the soundscape; reduced graph on low; node budget test`

---

## Task 12 — Listening pass: survey, checks, tuning

**Files:** only the tunable numbers below, then `scripts/system-world-sound-survey.situations.json` if a situation needs a fix.

**Steps**
- [ ] Run:
  ```
  node scripts/system-world-sound-survey.mjs --label phase-5 --baseline baseline --calibrate --strict --timeout 1800
  node scripts/system-world-sound-survey.mjs --label phase-5-low --tier low --only street-day,quay-day,lobby-agent-shut,park-wind
  node scripts/system-world-sound-survey.mjs --label phase-5-medium --tier medium --only street-day,quay-day,lobby-agent-shut,park-wind
  ```
- [ ] **Checks that must pass** (`--strict`):
  - every file ≤ −1.0 dBTP (above all `street-busy-rain-120`, the design's two-minute busy rain capture);
  - |`street-day` − `street-night-rain`| ≤ 2 LU (the design's day/night rule);
  - no situation more than +3 LU over the baseline (keeps the browser test's 0.05 RMS ceilings safe);
  - `lobby-agent-shut` ≥ 3 LU under `street-day`;
  - calibration `pass`.
- [ ] **Stats checks** in `report.json`:
  - `walk-surfaces` → `stats.foley.bySurface` has road, kerb or pavement, stairs and lobby > 0;
  - `tram-corner` → `layers.movers.trams[0].squeal` was > 0 at some point. Add a page log if needed; the survey's stats hold only the end state;
  - `quay-day` → `layers.beds.played` (laps) ≥ 3, `movers.launch` true;
  - `machine-hall` → `layers.rooms.tone === 'machine'`;
  - `lobby-agent-open` → `space.glass` < .4.
- [ ] **Tuning** (only these numbers, only in these ranges; record every change in the hand-back):

  | Constant | File | Start | Range |
  | --- | --- | --- | --- |
  | `LIMITER.trim` | audio | .607 | from calibration only, .55–.68 |
  | `RETURNS.canyon` / `.interior` / `.leak` | space | .45 / .5 / .35 | .3–.6 / .35–.7 / .2–.5 |
  | `GLASS.shut` / `GLASS.db` | space | 900 / −12 | 700–1400 / −15…−9 |
  | `BEDS.leaves` / `.laps` / `.steam` / `.traffic` / `.rain` / `.patter` | beds | .05 / .05 / .03 / .012 / .02 / .02 | .03–.08 / .03–.08 / .015–.05 / .006–.02 / .01–.04 / .01–.04 |
  | `LAP.rise` | beds | .15 | 0–.3 |
  | `MOVERS.tram` / `.squeal` / `.hover` / `.launch` / `.barge` | movers | .06 / .02 / .025 / .05 / .04 | .03–.09 / .01–.035 / .012–.04 / .03–.08 / .02–.06 |
  | room tone bed gains | rooms | table | ×0.5–×2 per tone, max .08 |
  | `STEP_VOICES[*].gain` | foley | table | .06–.09 |
  | `VISITOR_STEP.scale` | audio | .45 | .3–.7 |
  | foley send (`send.gain.value=inside?.14:.26`) | foley | .14/.26 | .1–.3 |
  | air bed `airGain` .32, murmur factors | audio | — | ±3 dB, only to meet the day/night rule |

- [ ] **Coordinator listening with earshot** (absolute paths under `C:\Users\Andi\Documents\repo\AuraGo\.worktrees\sysworld-beauty\reports\system-world-sound\phase-5\`):
  - `measure` on each file; LUFS and dBTP must agree with `report.json` within 0.5.
  - `describe` per file with these expectations:

    | Situation | Should rank (z ≥ 2.5) | Must not rank |
    | --- | --- | --- |
    | `quay-day` | water lapping, seagulls, harbour | traffic jam |
    | `street-day` | city street ambience, footsteps | birdsong as the top label |
    | `street-night-rain` | rain, city at night | birds |
    | `park-wind` | wind in trees, leaves rustling, birds | engine |
    | `lobby-agent-shut` | room tone, electrical hum | birds, rain |
    | `machine-hall` | machinery hum, fan | birds |
    | `lobby-departure` | airport or station hall, chime | — |
    | `tram-corner` | tram or train passing, metal squeal | — |
    | `garden-court` | water trickle, birds | — |

  - `ab` on baseline vs phase-5 for `quay-day`, `street-day` and `park-wind`.
  - `timeline` on `walk-surfaces` (step onsets about every 1.3 s, a different spectrum per surface) and `tram-corner` (approach, squeal, recede).
  - `realness` is not required (synthesised by design).
  - Tune within the ranges, re-render the touched situations, repeat until the checks pass and the descriptions match.

**Acceptance:** all strict checks pass; the coordinator's earshot notes and the final tuning table are in the hand-back.

**Commit:** `tune(system-world): soundscape levels from the listening pass (survey checks: true peak, day/night loudness, glass)`

---

## Task 13 — Docs, markers, browser check, bundle, regression, budget report

**Files:**
- `ui/desktop_sysworld_test.go`
- `ui/desktop_sysworld_city_browser_test.go`
- `ui/js/desktop/apps/AGENTS.md`
- `assets/system-world/README.md`
- `assets/system-world/AGENTS.md`
- `plans/2026-11-09-system-world-beauty-design.md`
- `ui/js/vendor/system-world/*` (generated)

**Steps**
- [ ] **Markers.** In `TestDesktopSysWorldAppMarkers` add:
  ```go
  "js/desktop/apps/sysworld-zones.js":       {"export function zoneWeights(", "export function surfaceAt(", "export function doorState(", "export const ROOMS", "export const SURFACE_KINDS", "export function createZoneTracker("},
  "js/desktop/apps/sysworld-sound-space.js": {"export function createSoundSpace(", "export function glassLaw(", "export const ROOM_SPACES", "createConvolver()"},
  "js/desktop/apps/sysworld-sound-beds.js":  {"export function createSoundBeds(", "export function leafLevel(", "seaHeight("},
  "js/desktop/apps/sysworld-sound-movers.js": {"export function createSoundMovers(", "export function tramLaw(", "craftPoses(", "gullPose("},
  "js/desktop/apps/sysworld-sound-rooms.js": {"export function createRoomTones(", "export const ROOM_TONES"},
  "js/desktop/apps/sysworld-sound-frame.js": {"export function createSoundFrame(", "windAt("},
  "js/desktop/apps/sysworld-foley.js":       {"export function createCityFoley", "export const STEP_VOICES", "surfaceAt("},
  ```
  Append:
  - to `sysworld-audio.js`: `"createDynamicsCompressor", "export const LIMITER", "createSoundSpace(", "zoneWeights(", "aurago.desktop.sysworld.volume", "createSoundBeds(", "createSoundMovers(", "createRoomTones("`;
  - to `sysworld-scene.js`: `"createSoundFrame(", "options.onSoundFrame?.("`;
  - to `sysworld.js`: `"onSoundFrame:", "inst.sound?.setTier("`;
  - to `sysworld-experience.js`: `"function doorValue("`.

  Run `gofmt -w ui/desktop_sysworld_test.go`.
- [ ] **City browser test.** After the eval that throws `'Audio lost the environment selected before unmute'` add:
  ```go
  	page.MustEval(`()=>{const s=SysWorldApp.inspect(cityId).sound;if(!s.limiter||s.limiter.trim<.5||typeof s.soundscape.zones?.harbour!=='number'||!s.soundscape.space||!s.soundscape.layers?.movers||!s.graph||s.graph.nodes>400)throw Error('Soundscape incomplete: '+JSON.stringify({limiter:s.limiter,zones:s.soundscape.zones,space:s.soundscape.space,graph:s.graph}));}`)
  ```
- [ ] **Docs** (concise, existing style).
  - **`ui/js/desktop/apps/AGENTS.md`, System World section.**
    - Rewrite the `sysworld-audio.js` bullet:
      - master chain;
      - injection (offline survey);
      - stored levels;
      - zone-weighted events with the indoor gate;
      - beds by zone;
      - the frame contract;
      - tiers (low: one reverb, no leaves/steam/patter, 1 tram, 2 hovers, no barge, first room bed only, 6 foley voices).
    - Add bullets for `sysworld-zones.js`, `-sound-space.js`, `-sound-beds.js`, `-sound-movers.js`, `-sound-rooms.js` and `-sound-frame.js`, using the Architecture ownership table.
    - Foley: surfaces and companion.
    - Companion: "it is heard (four light footfalls from `soundPose`), never in traffic or telemetry".
    - Add the new tests and `node scripts/system-world-sound-survey.mjs --label <name> [--baseline <label>] --strict` to the verification list.
  - **`assets/system-world/README.md`.** Extend `Sound is synthesized locally through the opt-in mixer, with no external audio files.` with one paragraph: zones, three spaces, glass, moving sources, room tones, surface steps, limiter, the survey command.
  - **`assets/system-world/AGENTS.md`.** Next to the foley test line add the new sound test commands.
  - **Design doc.** The phase 5 roadmap row points to `plans/2026-11-10-system-world-beauty-phase-5.md`.
- [ ] **Bundle:** `node scripts/build-system-world.js`, then `node scripts/build-system-world.js --check` → `verified`.
- [ ] **Final regression** (PowerShell, all must pass):
  ```powershell
  Get-ChildItem scripts/test-system-world*.mjs | ForEach-Object { node $_.FullName; if ($LASTEXITCODE) { throw $_.Name } }
  python assets/system-world/check_assets.py
  node scripts/build-system-world-v3.mjs --check
  node scripts/build-system-world.js --check
  go test ./ui -run '^TestDesktopSysWorld' -count=1
  $env:AURAGO_RUN_BROWSER_SMOKE='1'; $env:AURAGO_SYSTEM_WORLD_MATRIX='1'; $env:AURAGO_SYSTEM_WORLD_EXPANSION='1'; go test ./ui -run '^TestDesktopAuroraBrowser$' -count=1 -timeout 30m
  node scripts/system-world-sound-survey.mjs --label phase-5-final --baseline baseline --strict --timeout 1800
  ```
  Then, once (log only): `$env:AURAGO_SYSTEM_WORLD_AUDIO='1'` with the same browser command. Report the `overview-all` and `street-all` lines.
- [ ] **Budget report** in the hand-back:
  - the Task 11 `console.table` (nodes, live nodes, sources, reverbs, hover and tram voices per tier);
  - voice caps (foley 10/6, events 12, effects 16, laps 3, clacks 6, room events 6);
  - the real-time factor of `street-day` and `quay-day` at low, medium and high (Task 12 runs);
  - asset budgets unchanged: the `model directory` line of `test-system-world-v3.mjs` (< 40 MiB), first display, whole app. Audio bytes shipped: 0.

**Commit:** `chore(system-world): rebuild renderer bundle, pin soundscape markers and browser check, sound docs`

---

## Task 14 — Optional: CC0 recordings under the synthesis (only after the user's go-ahead)

**Precondition:** the user has explicitly agreed to download recordings, and the coordinator records that in the hand-back. Without it this task is not run, and the phase is complete without it.

**Files:**
- new `ui/3d/system-world/audio/*.ogg`
- new `assets/system-world/audio-sources.json`
- new `ui/js/desktop/apps/sysworld-sound-recordings.js`
- new `scripts/test-system-world-recordings.mjs`
- `sysworld-audio.js` (load after first display)
- `sysworld-sound-beds.js` (mix under the beds)
- licence notes in `licences/` and `assets/system-world/README.md`

**Steps (outline)**
1. **Pick sources (coordinator).** Six CC0-only loops: harbour water lapping, wind in leaves, songbird chorus, gulls, distant city hum, light rain on pavement. Download each into its own new empty folder (untrusted data). Verify the CC0 licence on the source page.
2. **Process** with ffmpeg: mono, 48 kHz, trimmed to 20–40 s, a 1 s crossfaded loop seam, loudness-normalised to −30 LUFS, Ogg Vorbis q3. Total ≤ 1.5 MiB.
3. **Provenance.** `audio-sources.json` lists per file: `source_url`, `author`, `licence:"CC0-1.0"`, `sha256`, `bytes`, `seconds`, `processing`, like `spot-source.json`.
4. **Loader.** `sysworld-sound-recordings.js` fetches through `resourceURL` only after `rebuild()` has resolved (not in first display), uses `decodeAudioData`, plays each recording as a looped `AudioBufferSource` under its synthesised bed at −8 dB (tune −12…−4), gated by the same zone level, and fails silent.
5. **Test.** Budget ≤ 1.5 MiB, hashes match, CC0 only, every file referenced, no external URLs in shipped code.
6. **Survey A/B.** Run `--label phase-5-recordings --baseline phase-5`; the strict checks still pass. In earshot, run `realness` on `quay-day` and `park-wind`.

**Commit:** `feat(system-world): CC0 ambience recordings under the synthesised beds (user-approved), with provenance and budget`

---

## Test and pin changes at a glance

| Pin | Old | New |
| --- | --- | --- |
| Foley test fake | inline, no compressor | shared `scripts/system-world-audio-fake.mjs` (Task 1) |
| Foley test "a world interaction never reaches the foley" | `effect('step')` | `effect('door')`; the visitor step is a foley surface voice (Task 10) |
| Browser test setup | resets the sound key | also clears stored volume and channels (Task 1) |
| Browser test, sound section | — | limiter, zones, space, movers, ≤ 400 nodes (Task 13) |
| AGENTS rule | "do not add a DynamicsCompressorNode" | compressor only with its trim and ceiling (Task 1) |
| `updateBeds` | quay distance rule, zeroed indoors, string quantization | zones (`quay`, plaza, canyon), glass damps, integer quantization |
| Event table | mood only | mood × zone weight, indoor gate; `citySoundscapeEvents(mood)` unchanged without zones |
| Visitor steps | `effects.play('step')` | `foley.step` with surface |
| `sysworld.js` | — | `onSoundFrame`, `setTier` on `onQuality` |
| Markers | 5 audio strings | + 7 files and new strings (Task 13) |
| New tests | — | soundscape, zones, sound-meter, sound-space, sound-beds, sound-movers, sound-rooms, sound-frame; foley sections |
| Assets | 0 audio bytes | 0 audio bytes (Task 14 only with approval: ≤ 1.5 MiB) |

## Risks and open questions

1. **Makeup trim wrong.** The mix would get louder or quieter. Task 3 calibrates, the browser test's RMS ceilings guard, and Task 12 keeps every situation within +3 LU of the baseline.
2. **More beds raise the browser test's RMS.** The baseline comparison and the opt-in audio run in Task 13 show it. The levers are the bed constants within their ranges.
3. **Offline rendering.**
   - If `ended` events do not fire during rendering, the event, foley and lap pools fill (could-not-verify 2). Fallback: a time-based release check in `frame()` (a follow-up task, not silently added).
   - If 25 Hz stepping is slow, use 20 Hz.
4. **CPU on weak machines.** Convolvers dominate. The low tier keeps the single existing reverb, voices are lazy (built only within reach), and the real-time factor per tier is reported.
5. **Automation churn** for moving sources (tram Doppler, hover pan). Quantized updates bound it; a dense tram pass schedules at most a few targets per frame per voice.
6. **Event gating uses the smoothed interior.** A bird can still sound in the first 0.3 s after entering. Accepted.
7. **Room tone switches.** Room tones fade in and out over 0.6 s; the interior IR swaps only at a near-silent send or after 1.5 s (a possible soft click). Listen in Task 12 (`walk-surfaces` enters the agent lobby).
8. **Resident simulation.** Untouched by design. Task 9 re-runs the living test only as evidence of no change.
9. **Float WAV compatibility** with earshot (could-not-verify 3). Fallback: 24-bit PCM output in Task 3.
10. **Street-tree density** may push the quay or street fixtures' shares (could-not-verify 5). The levers are `ZONE.treeRadius` and `treeShare`, never looser rankings.
11. **No HRTF/PannerNode.** Height is not localised (gulls overhead pan only). A possible phase 6 item if the listening pass asks for it.
12. **Recordings (Task 14)** stay out unless the user agrees. Synthesis remains the reacting layer either way.
