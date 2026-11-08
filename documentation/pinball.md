# AuraPinball

AuraPinball is a built-in Games app with three original 3D tables: Cyber Neon
Metropolis, Deep Sea Leviathan and Lost Temple of the Sun. All tables are available
immediately. Play alone or with up to four alternating players on the same computer.
No provider key or internet service is required; the verified AuraGo resource set
contains the game, models, paintings and sound effects.

## Playing

- Left/right Shift operate the corresponding flippers. Hold Space to pull the
  spring launcher, then release it. Arrow Left/Right/Up nudge the cabinet.
- Escape pauses; C switches the play/overhead cameras. R resets the training ball.
- Standard gamepads use the shoulder buttons for flippers, right trigger for
  launch, left stick or D-pad for nudge, X/Square for training reset and Start for pause. Keyboard actions and
  gamepad buttons can be reassigned in Settings; conflicting bindings swap.
- A player receives three balls. The first ten seconds after launch protect a
  drained ball. Training has unlimited balls and never writes highscores.
- Drop all three targets to open the central pocket. Three qualified locks start
  three-ball multiball. Mission progress and locks are independent for each player.
- Finish each table's three missions to light its finale jackpots, which remain
  available in single-ball play. The Temple idol doors open in three qualification
  stages; the physical gate opens after the full target bank. On Leviathan,
  slow shots through the relic field activate the bounded magnetic hold.
- Repeated strong nudges produce two warnings and then tilt. Tilt disables coils
  and bonus; passive walls and ball contacts keep working.

The fixed play camera keeps the whole playable area visible. Overhead helps with
shot placement. Showcase pauses the game and allows orbiting the model. Minimizing,
switching away or disconnecting the controller pauses input; resume explicitly
when ready. Closing ends the current game and releases its resources. A long
frame also pauses before dropping simulation time.

Graphics quality changes resolution, shadow resolution, bloom and transparent
materials. It never changes the simulation step or table collisions. WebGL 2 is
required. If it is unavailable, enable browser hardware acceleration and retry.
Mechanics and ambience have separate volume controls, including true zero volume.
Audio starts only after an interaction that starts/resumes a game.
Returned shooter balls retry the physical launcher. Mechanical ball-search runs
per ball, so a held flipper or another ball cannot suppress it; an unsuccessful
bounded search feeds a replacement without consuming the player's turn.

## Local data

The browser stores validated settings in `aurago.desktop.pinball.settings.v1`
and ten results per table in `aurago.desktop.pinball.scores.v1`. This data belongs
to that browser profile. A storage failure leaves the current session playable.
Active physics worlds are not saved or synchronized.

## Production and verification

The retained ImageGen masters and prompts are in `assets/pinball/`; Blender's
Python builders create actual cabinet, ramps, wires, flippers, table toys and
collision surfaces. The same authoring scene produces both GLB and physics data.
GLB scene extras carry the exact collision-file digest. Blender working files are
reproducible outputs in the ignored authoring `build/` folder.

```powershell
& 'D:\Blender 5.2\blender.exe' --background --factory-startup --python-exit-code 1 --python assets/pinball/generate.py
python assets/pinball/prepare.py
npm run build:pinball
npm run build:ui
npm run test:pinball
node scripts/build-pinball.mjs --check
node scripts/build-pinball-assets.mjs --check
$env:AURAGO_RUN_BROWSER_SMOKE='1'
go test -count=1 ./ui -run DesktopPinball
```

Blender 5.2.2 LTS, Three.js 0.186.1 and Rapier 0.21.0 are pinned. Physics uses SI
units, 27 mm / 80 g spheres, a 6.5-degree gravity component and fixed 480 Hz ticks.
Flippers use limited-torque revolute motors and a physical spring drives launch.
The Leviathan return tube has a real collision roof. Sensors sweep the ball path
to catch fast crossings; rolling resistance applies a contact torque. Timestamped
keyboard edges preserve taps that begin and end between rendered frames.

Node checks cover qualified locks, missions, duplicate drains, multiball, hotseat,
tilt, training, persistence, 25 m/s wall contacts, motor timing, passive energy and
30/60/144 Hz replay parity. Browser checks cover real models, window lifecycle,
settings and the three table flows. For captures set `AURAGO_BROWSER_ARTIFACT_DIR`;
set `AURAGO_PINBALL_GPU=1` to allow the available hardware GPU.
Set `AURAGO_PINBALL_BENCHMARK=1` with `go test -v -timeout 25m` for three minutes
of 1920×1080 High-quality multiball per table. The optional fixture records FPS,
95th-percentile frame time, pauses, maximum ball count and GPU identity beside
the captures. An unrendered requestAnimationFrame baseline exposes display/session
refresh limits. It does not affect ordinary gameplay.

The rendering target is 1080p/60 on RTX-3060-/RX-6600-class hardware. A software
renderer or a different GPU does not qualify that target. Three-minute reference-GPU
multiball runs and human live/drop-catch assessment remain separate acceptance.

## Credits

Original table artwork: built-in ImageGen, with retained prompts and source hashes.
Original 3D models: AuraGo's Blender recipes. The shared mechanical samples reuse
the curated CC0 packs by Kenney (Impact Sounds and Music Jingles); exact source
members, archive pins and delivered hashes are in the table manifest. The game
runtime carries Three.js MIT and Rapier Apache-2.0 notices.
