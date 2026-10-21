# AuraGolf

AuraGolf is the built-in Games app for eighteen low-poly 3D minigolf holes.
It runs locally in the browser after loading the matching AuraGo web resources.
WebGL2 and hardware acceleration are required. Blender is an authoring tool,
not an installation or server dependency.

## Playing

The start screen lists a round card for all eighteen holes, one card per world
with its preview (when the build provides one) and best score, and a Practice
card. Practice opens a hole grid that shows each hole's best score. A saved round
appears as a resume card. All worlds are available immediately. The start flow
hides the putting controls at the bottom, so the cards get the whole window.

One to four players share the same device and take turns after every shot. Names
take up to 24 characters; a duplicate name gets a number. Each player picks one of
eight colours, unique within the round, and that colour marks the player's ball,
HUD chip, scorecard row and podium step. Other players' translucent ball markers
show their names and never block the active ball. Pull back from the ball and
release to putt. Strength follows the on-screen drag distance (independent of
camera zoom); releasing next to the ball, a right click, a second finger or
Escape cancels the drag. A dashed preview path shows the roll up to the first
rail and a short stretch of the rebound, a color-graded ring marks the
strength, and the strength readout adds the estimated flat roll distance (full
strength rolls 28 m on flat felt). Arrow keys adjust aim and strength while
held (Shift gives finer steps); Space putts. A gentle aim assist snaps shots
within four degrees toward the cup, and after a penalty the rest position gets a marker ring. Right-drag rotates the view, the wheel
zooms, and **To the ball** restores focus. Optional settings move the camera
behind the ball on each turn and follow the rolling ball; reduced motion keeps
both functional without smooth transitions. Touch users can switch between
Camera and Aim; in Aim a one-finger drag from anywhere putts, and two fingers
zoom/pan.

The HUD shows a plaque with the world, hole, par and hole name, a chip per player
with the score and the active turn, and turn and hole banners. While you aim, a
power meter sits at the ball with the strength in percent and the estimated roll,
and it stays for a moment after each change. Toasts appear above the panels (the
main menu clears the ones left over from play), and a hint line covers the first shots of a session. The lane map, drawn from the
course collision geometry and water volumes, shows the bounds, water, tee, cup and
every ball; its toggle is saved. Water and out-of-bounds cost one penalty stroke
and return the ball to its previous shot position. Twelve unsuccessful putts
finish the hole with 13.

The scorecard appears when everyone finishes a hole. Its rows animate in when a
hole ends, Eagle/Birdie/Par/Bogey results are coloured, and Next hole continues
the round. After the last hole a podium names the winner; equal totals share a
place (1, 1, 3). A single player gets a result card instead. Play again restarts
the same mode with the same players and colours. A practice result also offers
Another hole, which returns to the hole grid.

New rounds start with an interactive introduction until it is finished or
skipped: a coach bubble walks through aiming, setting the power, releasing and
watching the first putt, without blocking the stage. Skip ends it for good, and
Help offers Restart introduction during a round.

Help opens from the question-mark button in the top bar, which stays at every
width, and with the ? key (ignored while you type). Escape cancels a drag first,
then steps back out of the current menu step, and otherwise closes the open panel
or pauses. Help and Settings are unavailable while the game loads or after a load
error.

Progress is saved after completed shots in namespaced browser localStorage.
Reopening resumes the last settled position, without counting an interrupted
shot. Records and saves belong to that browser/profile, not the server. If
browser storage is blocked, the game remains playable and reports that progress
cannot be saved. Practice updates per-hole bests only; round bests are kept for
the full course and for each world.

The browser keeps `aurago.desktop.minigolf.round.v2` (the saved round, including
each player's colour), `aurago.desktop.minigolf.best.v2` (round bests per mode,
hole bests per course, and the course set they were recorded for) and
`aurago.desktop.minigolf.settings.v1` (volumes, quality, motion, follow, aim
camera, magnet, `tutorial` from 0 to 2 where 2 hides the introduction, and
`showMap`). Unknown settings fields are kept. Old `round.v1` and `best.v1` data is
removed once without migration; if no v2 progress exists, a one-time notice says
that progress was reset.

## Worlds and look

Each world has its own diorama look: a rounded island with visible strata at its
edge, a sky, fog, light and bloom of its own, world props, and a water or space
underside. The manifest keeps the old world ids until P3 (`coast`, `garden`,
`night`); each course also carries its `look`.

| World | Manifest id | Holes | Look |
|---|---|---|---|
| Pirate Bay | `coast` | 7–12 | `pirate` |
| Jungle Temple | `garden` | 1–6 | `temple` |
| Moon Station | `night` | 13–18 | `moon` |

- **Pirate Bay:** sand around a turquoise lagoon, a ship as the hero prop,
  palms, barrels, a cannon, a treasure chest, a pier and a lighthouse. A warm
  white sun from above, a light aqua haze and a turquoise-to-light-blue sky with
  soft drifting clouds.
- **Jungle Temple:** a carved stone head as the hero prop, totems, torches, lianas,
  ferns, a waterfall, a sun wheel and a stone gate. A low, large orange sun,
  a peach-to-petrol evening sky with a haze band and a jungle silhouette.
- **Moon Station:** the island floats in space above a starfield, with the Earth
  in the sky. A habitat as the hero prop, a dish antenna that turns, a rover, solar
  panels, landing pads, neon posts and craters. There is no open water. The physics
  water volume is unchanged and acts as a fall into space.
- **Island:** the island surface runs 6 cm under the lane, also over hills and
  steep bends. The gaps under the jumps are a trench of water on Pirate Bay and a
  chasm into space on Moon Station.
- **Rails:** each world has a trim along its rails: wooden posts (Pirate Bay),
  stone blocks with joints (Jungle Temple), and metal with a neon strip (Moon
  Station). The collision rails stay in the course data and are hidden at runtime.
- **Lighting:** an environment map is built once per world from its sky, with one
  shadow-casting sun limited to the island and a faint rim light on Moon Station.
  The renderer uses Neutral tone mapping; exposure is set per world.
- **Water:** on Pirate Bay and Jungle Temple, one open water disc with depth
  colouring, two wave fields, glints on high quality, and fog at its rim. A
  shore strip (`water-shore`) has an animated foam edge.
- **Motion in the scene:** on medium and high, palms, ferns and grass sway, sails
  and flags ripple, torch and lamp flames flicker, and the dish turns. On every
  tier, the flag on the pole and the sun wheel turn, unless reduced motion is on.
  Small scatter props (critters, pebbles) are hidden on low.

## Intro flight

Loading a course flies the camera along its lane, from the tee to the cup, 2.2 m
behind and 3.2 m above the path while it looks 2.5 m ahead. It then settles into
the normal overview. The flight takes 0.35 s per metre of lane, between 2.5 and
4.5 s. Reduced motion skips it, and the overview appears at once.

Any of these ends the flight at once, in the overview pose:

- a pointer press on the stage (the press still counts for aiming),
- the mouse wheel,
- any key press (the key still counts for the game),
- a camera drag,
- a stroke or `focusBall()`.

A turn framing queued during the flight runs after it.

## Effects

- **Particles:** sand dust and a sand plume from a rolling ball on sand, water
  splashes with a ring, wood splinters on Pirate Bay, stone dust on Jungle
  Temple, sparks and dust on Moon Station, and ambient glints, fireflies or
  floating dust per world.
- **Ball trail:** a band in the player's colour behind a ball that moves faster
  than 0.6 m/s. It is on medium and high only.
- **Cup celebration:** confetti in the world colours, a gold ring that spreads over
  the green in 0.8 s, a glitter column and a flag that waves hard for about 2 s.
- **Win fireworks:** three rockets over the island at the end of the round, with
  confetti from three points.

## Reduced motion and graphics quality

Reduced motion follows the OS/Desktop preference or the game's checkbox. It
disables:

- the intro flight and camera turns (framing jumps to the end pose),
- decorative particles, confetti and fireworks,
- the gold ring's growth (it fades in and out at a fixed size),
- sway, flicker, the dish, the sun wheel and the flags,
- the animation of the water and the sky.

The ball trail stays, because it only follows the ball. Obstacles keep moving,
since that is gameplay.

Graphics quality can be set to low, medium or high; automatic starts at medium.
Automatic drops to low after a two-second sample below 38 FPS (not sooner than
6 s after the last change) and returns to medium after ten seconds at 55 FPS or
more, and at least 12 seconds after the last change.

| | low | medium | high |
|---|---|---|---|
| Pixel ratio | 1 | up to 1.5 | up to 2 |
| Shadows | none (blob shadow under the ball) | 1024 px, PCF | 2048 px, PCF |
| Bloom | off | on, half resolution | on, half resolution |
| Environment map | on | on | on |
| Sky | gradient, sun, Earth; no clouds or twinkle | + clouds | + star twinkle |
| Water | static gradient and shore foam | animated | + glints |
| Scatter props | off | on | on |
| Sway, flicker, dish | off | on | on |
| Particles (max per pool; confetti: half) | 60 | 160 | 320 |
| Ball trail | off | on | on |

The sun wheel and flags move on every tier unless reduced motion is on, and haze
and the Jungle Temple silhouette are drawn on every tier.

## Sound and accessibility

Audio starts only after a pointer or keyboard gesture. No audio context is
created and nothing is fetched before that gesture. Music starts at 20% and
effects at 65%. Zero and mute survive reopening; the volumes stay in the
existing `aurago.desktop.minigolf.settings.v1` key.

Four looping CC0 music tracks (`menu`, `pirate`, `temple` and `moon`) play from
decoded buffers and loop without a gap. A world change crossfades over 1.8 s,
and the cup, cannon and win sounds briefly duck the music. Until the new worlds
arrive, garden, coast and night use the temple, pirate and moon tracks and
ambience.

Effects are recorded CC0 samples with variants, a random pitch (±6%) and a
random volume (±15%); the same variant never plays twice in a row. Effects are
panned from the ball's screen position. Rolling loops follow ball speed and
surface: a soft loop on grass and sand, a hard loop on wood, stone and metal.
Impacts are chosen by material and strength. The cup plays a rim sound, two to
four rattles and a soft landing. Splash, cannon, boost, teleporter and strike
are sampled too, and each world has its own win jingle. At most 20 voices play
at once.

Interface clicks and world ambience are synthesized locally: waves and gulls on
the pirate world, insects and birds on temple, a hum with beeps on moon.
Effects and ambience send into a reverb built from a locally generated impulse
response for each world, and a limiter sits on the master bus.

Minimizing or hiding the browser tab pauses simulation and audio. Pause also
freezes the current shot and stops sound effects, rolling and ambience. The
audio context is suspended, so music resumes where it stopped. Closing releases
rendering, physics and audio resources.

Nothing is loaded from outside: the sound pack ships with the web resources, and
there is no CDN, SoundFont or external stream. The in-app credits list every
track and source under CC0; sources, authors, licenses and hashes are in
[`assets/minigolf/audio/README.md`](../assets/minigolf/audio/README.md).

Reduced motion also sets the `data-reduced` flag on the app root, which disables
CSS transitions. Gameplay obstacles keep moving.

## Assets and builds

Blender 5.2.2 LTS generates the original scenes, props and collision data from
`assets/minigolf/`. The runtime files are under `ui/3d/minigolf/`:

- `course-01.glb` to `course-18.glb`, one self-contained GLB per hole,
- `props-pirate.glb`, `props-temple.glb` and `props-moon.glb`, one prop kit per world,
- `previews/<look>.webp`, one menu preview per world,
- `manifest.json`, the compact course, kit and world index,
- the audio pack under `audio/`.

Authoring scenes are not included in the web resource archive. All models use
metres, Y up, and a 0.16 m sphere collider.

Each course GLB holds the lane (`surface-grass`, `surface-sand`), the collision
rails (`rail-left`, `rail-right`, hidden at runtime), the rail trims
(`rail-trim-*`), the island with visible strata, the shore strip (`water-shore`),
the layout specials, the cup, the flag and the hole number, the ball and putter
templates, and the obstacles. Everything else is merged into one `scenery-<material>`
node per material. A course has at most 24 mesh nodes.

Placed props are not in the course GLB. They come from the world's kit: the
manifest lists each course's placements as `[name, x, y, z, yaw, scale]`, and the
game draws each primitive of a prop with one `InstancedMesh`.

Course collision meshes include real cup openings and stay independent of visual
effects. The isolated renderer uses Three.js 0.186.1 and Rapier 0.21.0 with 120 Hz
fixed physics updates, continuous collision detection and bounded catch-up work.
The ball rolls with its own model instead of Rapier contact friction: a
constant plus a speed-proportional deceleration per floor surface, rolling
spin from launch, rails that bounce (wood 0.78, stone 0.82), trimesh
internal-edge fixing and a 5 mm contact skin so seams do not brake the ball.
The aim preview re-simulates the shot in a cloned physics world with the same
rolling code. P2 changes no physics or rules.

Menu previews are 640×360 Cycles renders, at most 80 KB each, rendered from holes
2 (temple), 7 (pirate) and 18 (moon). They are not bit-reproducible, so they are
built separately from the course and kit build.

### Generator

The generator is `assets/minigolf/generate.py`, a thin CLI over the `kit/` package
(see [`assets/minigolf/README.md`](../assets/minigolf/README.md) for the module map).
Run it from the repository root with Blender 5.2.2:

```powershell
& 'D:\Blender 5.2\blender.exe' --background --factory-startup --python assets/minigolf/generate.py -- --only kits,courses
& 'D:\Blender 5.2\blender.exe' --background --factory-startup --python assets/minigolf/generate.py -- --only kits
& 'D:\Blender 5.2\blender.exe' --background --factory-startup --python assets/minigolf/generate.py -- --only courses --course 7
& 'D:\Blender 5.2\blender.exe' --background --factory-startup --python assets/minigolf/generate.py -- --only courses --course 7 --save-blend
& 'D:\Blender 5.2\blender.exe' --background --factory-startup --python assets/minigolf/generate.py -- --only previews
```

- `--only kits,courses` (the default), `kits`, `courses`, `previews` or `all`.
- `--course N` (1–18) rebuilds one course and patches its manifest entry; a full
  build must exist first.
- `--save-blend` writes `assets/minigolf/build/aura-golf.blend` (git-ignored). It is
  allowed only with `--course N`, because a full run deletes each course after its
  export.

A full build takes about two minutes on 16 cores (measured 80–110 s); all three kits take about 15 seconds.
On Windows a transient file lock can abort one export. Run the command again.

Generator output is deterministic. Randomness comes only from seeded
`random.Random` and Blender's noise, the bakes run on the CPU with a fixed seed and
sample count, and the colours are rounded to 8 bits. Each course is deleted after
its export, so no node or mesh name carries a Blender counter (`scenery-matte.001`),
and `--course N` in a fresh process writes the same file as the full run. The
acceptance check runs the build twice and compares the GLB and manifest hashes.

### Budgets

The asset test (`scripts/test-minigolf-assets.mjs`) is the authoritative check.
The generator fails earlier for primitive and material limits.

| Item | Limit | Last full build |
|---|---|---|
| Runtime package (everything under `ui/3d/minigolf/`) | ≤ 32 MiB | 21,320,234 bytes |
| Course GLB | ≤ 768 KiB and ≤ 24,000 triangles | largest 754,904 bytes, 21,149 triangles; 336,816 triangles in all |
| Mesh nodes per course GLB | ≤ 24 | enforced |
| Kit GLB per world | ≤ 3 MiB and ≤ 60,000 triangles | pirate 1,771,100 bytes (46,745 triangles), temple 1,768,908 bytes (57,226), moon 1,489,072 bytes (41,584) |
| Single prop | ≤ 15,000 triangles (hero ≤ 20,000) | the temple head, the hero, has 16,444 |
| Primitives per prop | 3 (hero 4) | enforced by the generator |
| Materials per world kit | 8 | enforced by the generator |
| Placements per course | ≤ 160 | enforced |
| Menu preview | ≤ 80 KB, 640×360 WebP | enforced |
| Game bundle (`ui/js/vendor/minigolf/game.esm.js`) | ≤ 6 MiB | enforced by `scripts/build-minigolf.mjs` |
| Frame rate per quality tier | at least the baseline measured on the same machine | see the benchmark below |

Course manifest sizes: 18 course GLBs total 10,065,848 bytes; the manifest is
3,312,219 bytes; the three kits total 5,029,080 bytes; the course triangles total
336,816.

### Benchmark

Measured with `AURAGO_MINIGOLF_BENCHMARK=1` on an Intel UHD GPU through ANGLE,
at 1920×1080, over 240 frame intervals per quality tier:

- The measured rate sits at a frame-pacing cap of about 52 FPS on this machine,
  so the FPS figure shows parity, not headroom.
- The p95 frame time is within 1 ms of the like-for-like baseline on every tier.
- Draw calls on low fell from 149 to 47 for the whole frame. Low renders one
  pass, so the two counts compare directly.

These numbers are specific to this machine and browser. The benchmark logs them
with `go test -v`; a rerun on other hardware is a separate acceptance step.

### Verification

```
npm run build:minigolf
node scripts/build-minigolf-audio.mjs --check
node scripts/build-minigolf.mjs --check
npm run test:minigolf
npm run test:synth-studio
node scripts/build-ui-bundles.js --check
```

`npm run test:minigolf` runs the model, look, UI, physics, asset, course and
audio checks. The look test (`scripts/test-minigolf-look.mjs`) checks the look
table, the legacy world mapping, the quality tiers, the placement matrix and the
intro flight. The asset test checks the budgets above, the P1 collider and
physics fingerprints (`scripts/fixtures/minigolf-p1-colliders.json`), numbered
names, the look and path of each course, placements against the island rim and
the lane corridor, the kits, and the previews.

Run `TestDesktopMinigolfBrowser` with `AURAGO_RUN_BROWSER_SMOKE=1` for actual
WebGL integration, controls, lifecycle and responsive review:

```
$env:AURAGO_RUN_BROWSER_SMOKE='1'; go test -count=1 -v ./ui -run TestDesktopMinigolfBrowser
```

The smoke checks each world's look, props and environment, and that the intro
flight starts and then ends after `skipIntro`. The headless Chrome used here
reports `prefers-reduced-motion: reduce`, so the smoke emulates `no-preference`
for the first course to see the flight, then resets the emulation. Later loads
under reduced motion must start no flight.

Add `AURAGO_MINIGOLF_BENCHMARK=1` for the 1920×1080 measurements above, and
`AURAGO_BROWSER_ARTIFACT_DIR` for screenshots and the active-play audio statistics.
The AuraGolf browser fixture mutes speaker output while exercising Web Audio.
`AURAGO_BROWSER_MUTE_AUDIO=1` also mutes the Synth Studio browser regressions
(`TestDesktopSynthStudioBrowser`, `TestDesktopSynthStudioAudioBrowser`). The
underrun check has no tolerance; if only that check fails under heavy machine
load, rerun once before suspecting a regression.
Optional browser artifacts belong in ignored `reports/minigolf/`. Render measurements are specific
to the tested browser/GPU; software rendering is not hardware acceptance.

The course tests replay a verified route into every real cup. The optional
`AURAGO_MINIGOLF_SEARCH=1` mode searches for routes after geometry changes;
normal regression runs use the recorded shots instead.

The app is registered through the usual Desktop manifest, lazy module loader,
app routing and disposal map. It has no new HTTP endpoints or server state.
Release and installation use the normal version-bound web asset packaging.
