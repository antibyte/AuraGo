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
strength rolls 28 m on flat felt, on the moon as well). The preview applies
bumper kicks and boost pads and ends at a teleporter or a cannon. Arrow keys adjust aim and strength while
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
that progress was reset. The courses of the 18-course P3 set have new ids, so a
round saved with the old course ids silently no longer resumes, and `best.v2` drops
the round bests of the old set (hole bests of ids that no longer exist go too).

## Worlds and courses

Each world has its own diorama look: an island with visible strata at its edge,
a sky, fog, light and bloom of its own, world props, and a water or space
underside. The island is as big as its layout, so a long course is a wide
diorama; the hero prop (ship, stone head, habitat) stands at full size. The
manifest (version 2) uses the world id as the look: `pirate`, `temple`, `moon`.
The game accepts only manifest version 2; there are no aliases for the old world
ids of the earlier builds (`coast`, `garden`, `night`).

| World | Id | Holes | Gravity | Rail |
|---|---|---|---|---|
| Pirate Bay | `pirate` | 1–6 | 9.81 m/s² | wood |
| Jungle Temple | `temple` | 7–12 | 9.81 m/s² | stone |
| Moon Station | `moon` | 13–18 | 4.0 m/s² | metal |

Every world starts gently and ends with a finale. The route length is the main
route from the tee to the hole; most holes also have a risky shortcut (a plank
without rails, a jump over water or a canyon, a tunnel behind a door, a
teleporter, a cannon).

| Hole | Id | Name | Par | Route | What is on it |
|---|---|---|---|---|---|
| 1 | `pirate-01` | Anchor Cove | 2 | 12 m | straight, curve, green; the ship at anchor beside the green |
| 2 | `pirate-02` | Barrel Bumpers | 3 | 17 m | sand straight and bank, wooden plaza with three barrel bumpers |
| 3 | `pirate-03` | Plank Walk | 3 | 22 m | fork: sand bunker or a railless plank bridge over the lagoon; lighthouse |
| 4 | `pirate-04` | Cannon Cove | 4 | 27 m | cannon deck, ramp, boardwalk on stilts around the lagoon; the cannon fires over the water |
| 5 | `pirate-05` | Smuggler's Cave | 4 | 27 m | hatch teleporter to a barrel, skull-rock tunnel, swinging anchor |
| 6 | `pirate-06` | Kraken's Whirlpool | 5 | 33 m | fork over a channel (bridge or jump), two bumpers, drawbridge gate, funnel |
| 7 | `temple-07` | Temple Steps | 3 | 16 m | three terraces, bend, green; the stone head above the steps |
| 8 | `temple-08` | Rolling Stone | 3 | 19 m | fork: long way or a valley crossed by a rolling boulder |
| 9 | `temple-09` | Stone Head | 4 | 24 m | fork: around the head or through its mouth, behind a stone door |
| 10 | `temple-10` | Swinging Logs | 4 | 26 m | two bridges over a canal: railed with two log pendulums, or railless; waterfall |
| 11 | `temple-11` | Sacred Well | 4 | 27 m | steps, pond plaza, idol portals (teleporters), hairpin, funnel well |
| 12 | `temple-12` | Golden Throne | 5 | 34 m | ramp, fork: two stone doors in alternating rhythm and a boulder, or a jump over the ravine |
| 13 | `moon-13` | Touchdown | 2 | 14 m | straight, hill, green: first lesson in low gravity |
| 14 | `moon-14` | Boost Lane | 3 | 22 m | two boost plates, banked curve, regolith, a plaza with a void pit, two neon bumpers |
| 15 | `moon-15` | Crater Pipe | 3 | 24 m | fork: S-shaped half-pipe or a jump over the canyon |
| 16 | `moon-16` | Beam Station | 4 | 25 m | two teleporters, bridge over a rift, rotating radar arm |
| 17 | `moon-17` | Zero-G Leap | 4 | 33 m | fork: detour behind a bulkhead door or a long jump over the canyon; two bumpers |
| 18 | `moon-18` | Orbit Loop | 5 | 35 m | boost plate, vertical loop, fork (blast door detour or half-pipe), funnel |

- **Pirate Bay:** sand around a turquoise lagoon, a ship as the hero prop,
  palms, barrels, a cannon, a treasure chest, a pier and a lighthouse. A warm
  white sun from above, a light aqua haze and a turquoise-to-light-blue sky with
  soft drifting clouds.
- **Jungle Temple:** a carved stone head as the hero prop, totems, torches, lianas,
  ferns, a waterfall, a sun wheel and a stone gate. A low, large orange sun,
  a peach-to-petrol evening sky with a haze band and a jungle silhouette.
- **Moon Station:** the island floats in space above a starfield, with the Earth
  in the sky. A habitat as the hero prop, a dish antenna that turns, a rover, solar
  panels, landing pads, neon posts and craters. There is no open water: a ball that
  falls over an open edge or into a pit leaves the course and counts as out of bounds.
- **Island:** the island surface runs 6 cm under the lane, also over hills and
  steep bends. The gaps under the jumps are a trench of water on Pirate Bay and
  Jungle Temple and a chasm into space on Moon Station. Ponds are their own
  water surfaces over a hollow, not a trench down to the sea.
- **Track pieces:** straight, curve, banked curve, ramp with a jump, hill and
  dip, steps, funnel, tunnel, bridge, fork with two routes, plaza, and on the moon
  a loop and a half-pipe. Elevated pieces stand on supports, and a thin metal shell
  closes the outer side of the loop (the floor has no back face, which the game hides).
- **Rails:** each world has a trim along its rails: wooden posts (Pirate Bay),
  stone blocks with joints (Jungle Temple), and metal with a neon strip (Moon
  Station). The collision rails are part of the course GLB and hidden at runtime.

### Mechanics and moving obstacles

- **Bumper:** a solid barrel (Pirate Bay), stone mushroom (Jungle Temple) or neon
  pylon (Moon Station) that kicks the ball away at a minimum speed.
- **Boost plate (moon):** a glowing plate that lifts the ball to a given speed in
  its direction once per entry. A loop needs one shortly before it.
- **Teleporter:** a ball entering the ring or hatch reappears at the exit (a
  barrel, an idol arch, a transporter pad with a spinning ring) and keeps a
  minimum speed. They are mostly shortcuts.
- **Cannon (Pirate Bay):** captures a ball that rolls into it, holds it for a
  moment and fires it along an arc onto a target spot.
- **Moving obstacles:** swinging anchor and logs, drawbridge, stone and
  bulkhead doors, a rolling boulder and the radar arm. They are periodic with one
  shared cycle per course, follow the physics time (also under reduced motion) and
  are drawn from exactly the shape that collides.
- **Gravity:** Moon Station uses 4 m/s². A full strength putt still rolls 28 m
  on flat felt, but the ball stays in the air much longer. A gap that needs full
  strength on Earth is cleared with a gentle putt on the moon (the shortcuts of
  holes 15, 17 and 18, a canyon jump, a long jump and the half-pipe, want strengths
  of about 0.3 to 0.5), and a harder one flies over the landing. The aim preview shows the flight path. Inside
  a loop or on a steep half-pipe wall the ball glides without rolling friction.
- **Sparks and sound:** boost, teleporter and cannon have their own sounds and
  spark bursts; a bumper kick plays the ordinary impact.
- **Lighting:** an environment map is built once per world from its sky, with one
  shadow-casting sun limited to the island and a faint rim light on Moon Station.
  The renderer uses Neutral tone mapping; exposure is set per world.
- **Water:** on Pirate Bay and Jungle Temple, one open water disc with depth
  colouring, two wave fields, glints on high quality, and fog at its rim. A
  shore strip (`water-shore`) has an animated foam edge.
- **Motion in the scene:** on medium and high, palms, ferns and grass sway, sails
  and flags ripple, torch and lamp flames flicker, and the dish turns. On every
  tier, the flag on the pole waves and the teleporter rings turn, unless reduced motion is on.
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

The overview pose adapts to the size of the course. A small island keeps the fixed
view of the tee-to-cup midpoint; a larger one is framed by its playable area (every
route of the lane plus 2.5 m), from 21.6 m up to 60 m away. The shadow camera, the
water disc and the fog follow the island size, and the fog thins with the camera
distance so the lane stays as clear as on a small course.

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
- sway, flicker, the dish, the teleporter rings and the flags,
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

Flags and teleporter rings move on every tier unless reduced motion is on, and haze
and the Jungle Temple silhouette are drawn on every tier.

## Sound and accessibility

Audio starts only after a pointer or keyboard gesture. No audio context is
created and nothing is fetched before that gesture. Music starts at 20% and
effects at 65%. Zero and mute survive reopening; the volumes stay in the
existing `aurago.desktop.minigolf.settings.v1` key.

Four looping CC0 music tracks (`menu`, `pirate`, `temple` and `moon`) play from
decoded buffers and loop without a gap. A world change crossfades over 1.8 s,
and the cup, cannon and win sounds briefly duck the music. Each world plays its
own track and ambience.

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

Each course GLB holds the visible floors (one `floor-<kind>` node per floor kind, for
example `floor-felt`, `floor-sand` or `floor-metal`, with the extra `aurago_collider`
`grass` or `sand`), the cup floor (`floor-cup`), the collision rails (`rail-wood`,
`rail-stone` or `rail-metal`, hidden at runtime), the rail trim (`rail-trim`), the
shore strip (`water-shore`), the flag, the ball and putter templates, the moving
obstacles (`obstacle-<id>`) and the spinning teleporter rings (`mechanic-<id>-ring`,
extra `aurago_spin`). Everything else (the island with visible strata, the tee ring,
the cup rim, the hole number, the static mechanic parts, the dressing) is merged into
one `scenery-<material>` node per material. A course has at most 32 mesh nodes.

Placed props are not in the course GLB. They come from the world's kit: the
manifest lists each course's placements as `[name, x, y, z, yaw, scale]`, and the
game draws each primitive of a prop with one `InstancedMesh`.

The collision is read from the course GLB itself: the game collects the
nodes with `aurago_collider`, welds the vertices of each surface and feeds them to
Rapier, so physics and picture come from the same Blender geometry and nothing is
listed twice. The manifest only carries a SHA-256 over the welded meshes
(`collision.digest`), which the generator (Python) and the game (`minigolf-collision.js`)
compute identically and the asset test compares. A GLB without a readable grass floor
fails to load. The meshes include real cup openings and stay independent of visual
effects. The isolated renderer uses Three.js 0.186.1 and Rapier 0.21.0 with 120 Hz
fixed physics updates, continuous collision detection and bounded catch-up work.
The ball rolls with its own model instead of Rapier contact friction: a
constant plus a speed-proportional deceleration per floor surface, rolling
spin from launch, rails that bounce (wood 0.78, stone 0.82), trimesh
internal-edge fixing and a 5 mm contact skin so seams do not brake the ball.
The aim preview re-simulates the shot in a cloned physics world with the same
rolling code. Each course sets its own gravity (`gravity`, 9.81 or 4.0 m/s²); launch speed and
rolling resistance are unchanged, and metal rails bounce with restitution 0.80.

Menu previews are 640×360 Cycles renders, at most 80 KB each (this build 23,932 to
33,640 bytes), rendered from the showcase holes `pirate-04`, `temple-09` and `moon-18`
and framed on the playable area. They are not bit-reproducible, so they are built
separately from the course and kit build (`--only previews`).

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
| Runtime package (everything under `ui/3d/minigolf/`) | ≤ 32 MiB | 23,318,031 bytes |
| Course GLB | ≤ 1.25 MiB and ≤ 36,000 triangles | largest 1,203,448 bytes (moon-18), most triangles 33,877 (temple-12); 494,664 triangles in all |
| Mesh nodes per course GLB | ≤ 32 | enforced |
| Manifest (`manifest.json`) | ≤ 256 KB | 132,124 bytes |
| Kit GLB per world | ≤ 3 MiB and ≤ 60,000 triangles | pirate 1,771,100 bytes (46,745 triangles), temple 1,768,908 bytes (57,226), moon 1,489,072 bytes (41,584) |
| Single prop | ≤ 15,000 triangles (hero ≤ 20,000) | the temple head, the hero, has 16,444 |
| Primitives per prop | 3 (hero 4) | enforced by the generator |
| Materials per world kit | 8 | enforced by the generator |
| Placed props per course | ≤ 220 | enforced (largest 198, pirate-06) |
| Menu preview | ≤ 80 KB, 640×360 WebP | enforced |
| Game bundle (`ui/js/vendor/minigolf/game.esm.js`) | ≤ 6 MiB | enforced by `scripts/build-minigolf.mjs` |
| Frame rate per quality tier | not worse than P2, measured on the same machine | parity at the frame cap, see the benchmark below |

Sizes of this build: the 18 course GLBs total 15,206,816 bytes, the manifest is
132,124 bytes (the collision is no longer listed there), the three kits total
5,029,080 bytes and the course triangles total 494,664.

### Benchmark

Measured with `AURAGO_MINIGOLF_BENCHMARK=1` at 1920×1080, over 240 frame intervals
per quality tier, on the last course loaded by the browser test. P2 and P3 were
measured on the same machine in the same session (Intel UHD Graphics through
ANGLE/Direct3D11, headless Chrome in a remote desktop session), three alternating
runs each; the table shows the means. P2 measures `night-18` from the P2 build, P3
measures `moon-18` (the Moon Station finale with a loop, a fork and a funnel).

| Tier | P2 FPS | P3 FPS | P2 / P3 median and p95 frame | P2 / P3 triangles | P2 / P3 draw calls |
|---|---|---|---|---|---|
| high | 52.65 | 52.72 | 19.00 / 19.70 ms, 18.97 / 19.73 ms | 214,546 / 422,952 | 99 / 115 |
| medium | 52.58 | 52.45 | 19.03 / 19.63 ms, 19.07 / 19.73 ms | 214,546 / 422,952 | 99 / 115 |
| low | 50.42 | 49.19 | 18.97 / 19.73 ms, 18.97 / 19.70 ms | 68,323 / 114,126 | 47 / 55 |
| empty page | 52.83 | 53.09 | 18.83 / 19.50 ms, 18.73 / 19.37 ms | | |

- Every tier and the empty page sit on the frame-pacing cap of this session
  (about 52–53 FPS), so the result is parity within noise, not headroom. Median and
  p95 frame times match P2 on every tier; the low-tier mean differs by one frame
  per second, inside the spread of the P2 runs (49.4–52.4 FPS).
- P3 draws about twice the triangles and 16–17 % more draw calls than P2 because its
  dioramas are larger. Headroom on a faster GPU without a frame cap is not measured.

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

`npm run test:minigolf` runs the model, look, UI, physics, collision, asset, course
and audio checks, all without environment variables. The look test
(`scripts/test-minigolf-look.mjs`) checks the look table, that the world id is the
look (and that an old world id is not mapped), the quality tiers, the placement
matrix and the intro flight. The physics test covers gravity, bumpers, boost
plates, teleporters, the cannon, every obstacle kind, the loop prototype and the
seam cases; the collision test reads a small test GLB (hierarchy, transforms,
welding, digest).

The asset test (`scripts/test-minigolf-assets.mjs`) checks manifest version 2 with
18 courses (6 per world, ids `pirate-01` to `moon-18`), the budgets above, the
collision digest of the GLB against the manifest, visible floors and hidden rails,
an open cup, a tee above the floor, `path` and `corridors`, placements against the
island rim and every corridor, a hero at scale 0.9 or more, every kit prop placed in
at least one course, mechanics and obstacles inside `bounds`, a whole-number `cycle`,
unnumbered node names, the kits, the previews and the glTF validator. The course
test (`scripts/test-minigolf-courses.mjs`) plays every course against Rapier with the
collision read from the GLB: an accepted tee shot over grass, obstacles that move with
physics time and a recorded route (at most par + 1 shots) into the real cup.

Both tests read `ui/3d/minigolf` unless the environment says otherwise:

| Variable | Effect |
|---|---|
| `AURAGO_MINIGOLF_DIR=<dir>` | check another asset folder (a scratch build under `assets/minigolf/build/`) |
| `AURAGO_MINIGOLF_PARTIAL=1` | accept a subset of the 18 courses (a course without a recorded route is skipped) |
| `AURAGO_MINIGOLF_SEARCH=1` | course test: search every route anew and print it as a `ROUTES` line (slow) |
| `AURAGO_MINIGOLF_COURSE=<id>` | course test: only this course (id, number or `<world>-<NN>`) |

`node scripts/minigolf-route-search.mjs --dir <dir> --course <id>` searches and
verifies a route for one course while it is tuned. Keep the full precision of the
angles in `ROUTES`: rounding an angle to six decimals can already miss the cup.

Run `TestDesktopMinigolfBrowser` with `AURAGO_RUN_BROWSER_SMOKE=1` for actual
WebGL integration, controls, lifecycle and responsive review:

```
$env:AURAGO_RUN_BROWSER_SMOKE='1'; go test -count=1 -v ./ui -run TestDesktopMinigolfBrowser
```

The smoke loads one course per world through the normal menus (the finales
`pirate-06`, `temple-12` and `moon-18`; `AURAGO_MINIGOLF_ALL_COURSES=1` loads all 18, which
takes a good deal longer) and checks each course's look (equal to its world), props and
environment, that the ball rests on the tee floor (the collision from the GLB is in
place), and that the intro flight starts and then ends after `skipIntro`. The headless Chrome used here
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
