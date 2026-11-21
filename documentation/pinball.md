# AuraPinball

AuraPinball is a built-in Games app with four original tables: Cyber Neon
Metropolis, Deep Sea Leviathan, Lost Temple of the Sun and Clockwork Observatory. All tables are available
immediately. Play alone, or with up to four alternating players on the same computer.
No provider key or internet service is required. The game, its tables, lights,
sound and music ship inside AuraGo.

The physics is a custom deterministic 2.5D engine at a fixed 480 Hz tick. The ball has
spin as well as speed: it slides, then settles into rolling on the playfield, and friction at
walls, posts, rubbers and flippers changes its spin. The tables are built procedurally in
three.js, with painted playfields, backglasses, side art and printed plastics. The
observatory's orrery, telescope, gear train and dome are 3D toys loaded from one GLB file.
The lamps and the 192×48 dot-matrix display (DMD) are drawn in code, and all sound effects
and music are synthesized at runtime.

## Tables

### Cyber Neon Metropolis

- **FIREWALL and the VAULT lock.** Drop all three FIREWALL targets to light the lock.
  While the lock is lit, the right ramp feeds the VAULT. Each ball locked in the vault
  is one of three locks.
- **Gridlock multiball.** The third lock starts GRIDLOCK MULTIBALL: the FIREWALL bank
  drops, the vault releases its balls and the table tops up to three balls. The four
  ramps and orbits are jackpots. Four jackpots light the super jackpot at the Data Core
  for 20 seconds. The vault can add one extra ball during the multiball.
- **N-E-O-N.** Hit the four N-E-O-N standups to complete the word. It advances the bonus
  multiplier, lights mode start, and the second and third completions light the extra
  ball and the kickback.
- **UPLINK.** The three upper-flipper targets (U, P, L) pay 250,000 when all three are
  hit. Later completions pay 750,000. The first completion also lights the extra ball,
  and a running mode gets 10 seconds more.
- **Data Core modes.** Shoot the Data Core to start one of four modes: HACK THE GRID
  (six ramp hits), BLACKOUT (four hits on a moving target over five shots), NEON CHASE
  (five orbit hits, spinner worth more) and OVERCLOCK (thirty bumper hits). Play all
  four and the next Data Core shot starts the wizard.
- **System Meltdown.** The wizard gives four balls, a 20-second ball save and rising
  jackpots. Six jackpots light the super jackpot at the Data Core.
- **Super skill shot.** Shoot the Data Core within 6 seconds of launch.
- **Matrix Breach (secret combo).** Shoot the left ramp, right ramp and Data Core in
  sequence within 12 seconds to unlock 750,000 points and advance the bonus multiplier.

### Deep Sea Leviathan

- **Captive pearl and the virtual lock.** Strike the captive pearl three times to light
  the lock on the tentacle ramp (two strikes once the lock has been lit before). A pearl
  that rides to the top counts as one more strike. A tentacle shot while the lock is lit
  locks the ball virtually, and three locks start ABYSS MULTIBALL, which tops up to three
  balls.
- **Sonar targets and Deep Grab.** Hit the five sonar standups to light Deep Grab, for
  10 seconds. Hold a ball on the abyss magnet for 1.5 seconds and it is thrown toward the
  sunken ship. Scoop it in the ship within 2 seconds for 1,000,000. The sonar also relights
  the kickback.
- **Ship modes.** The sunken ship starts the modes: TSUNAMI, SONAR SWEEP, SUNKEN
  TREASURE (the magnet assists) and KRAKEN HUNT. After two completed modes, the ship
  lights an extra ball.
- **Leviathan Awakens.** After all four modes, the next ship shot starts the wizard
  with four balls. The super skill is the ship scoop within 6 seconds of launch.
- **Abyssal Vortex (secret combo).** Chain the current ramp, tentacle ramp and captive
  pearl within 12 seconds for 750,000 points and a bonus multiplier step.

### Lost Temple of the Sun

- **SUN bank and the idol doors.** Drop the four SUN targets. The bank opens the idol's
  doors and lights mode start. After a mode starts, the doors shut and the bank resets.
- **Pop-bumper lock.** Twenty pop-bumper hits light the lock in the right orbit. The
  lock is physical: the ball is held in the orbit kickhole. Three locks start GOLDEN
  MULTIBALL, which opens the idol doors and makes the idol the super jackpot shot.
- **Jewels.** Light both jewel standups for 100,000. The first completion advances the
  bonus multiplier, the second relights the kickback, and they alternate after that.
- **Temple modes.** The idol starts EXPEDITION (orbits), SERPENT'S PATH (ramps), the
  RIDDLE OF THE IDOL (hit the left ramp, right ramp, left orbit and right orbit in order)
  and ECLIPSE (25 bumper and sun-target hits, the lighting dims).
- **Temple of the Sun.** After all four modes, the wizard starts at the idol. The extra
  ball is lit at the left ramp after two modes, and the super skill is the left ramp
  within 8 seconds of launch.
- **Solar Secret (secret combo).** Shoot the left ramp, left orbit and idol scoop in
  order within 12 seconds for 750,000 points and a bonus multiplier step.

### Clockwork Observatory

A steampunk observatory at night: brass and mahogany instruments under a night-sky
blue dome. Six shots: left orbit, comet ramp, the orrery lane, the telescope, the
meridian ramp and the right orbit. The 3D toys are the orrery on the disc, the telescope
on its pier, the gear train on the left back corner and the dome on the right back corner.
The clock hands are on the clock face of the dome. The dome shutter opens during multiball
and the wizard.

- **Orrery disc.** A turntable set flush in the upper middle of the playfield. A ball that
  rolls onto it is carried round and curls off the rim. The disc turns faster in the wizard
  and multiball and slower in GRAND CONJUNCTION. Shooting the orrery lane reverses it.
- **Pendulum.** A pendulum swings across the mouth of the telescope channel and knocks
  balls on. Each strike advances the clock. It swings fast during TRANSIT OF VENUS and
  parks out of the way while the wizard runs or S-T-A-R has latched it.
- **Telescope.** The telescope kicks the ball up the brass tube along the barrel, then
  drops it onto the rim of the disc.
- **Gravity wells.** Two magnets sit either side of the telescope channel. A ball held by a
  well for 0.8 seconds is thrown at the mouth of the channel. A telescope shot within
  2 seconds of the throw pays 1,000,000 as GRAVITY SLINGSHOT.
- **S-T-A-R.** Hit the four standups S, T, A and R on the slanted bank to the right of the
  disc. Completing them scores 100,000 and lights the mode start. The clock moves on by
  one, both gravity wells grab balls for 10 seconds, and the pendulum parks for 12 seconds.
- **Clock and midnight.** Twelve lamps ring the disc and show the hour, and the clock face on the dome shows it too.
A brass bezel with twelve marks sits flush around the disc, inside the lamps. Each shot moves the
  hour on by one, and the telescope by two. A pendulum strike moves it by one, at most once
  every 0.6 seconds. The clock does not move during a multiball. At twelve, MIDNIGHT
  MULTIBALL starts and the clock returns to zero. If the mode select is open at midnight,
  the multiball starts when it closes.
- **Modes.** The telescope starts one of four modes from the mode select:
  - **METEOR SHOWER:** hit the two ramps five times within 40 seconds. Each spinner hit
    during the mode is worth 4,000.
  - **TRANSIT OF VENUS:** hit the left orbit, the orrery lane, the telescope and the right
    orbit in that order within 45 seconds. The pendulum swings fast.
  - **GRAND CONJUNCTION:** four hits on a lit shot within 45 seconds. The lit shot moves to
    another shot every five seconds. The disc turns slowly and the gravity wells stay on.
  - **SUPERNOVA:** 20 hits on the bumpers, standups and pendulum within 35 seconds. The
    lighting dims during the mode.
- **Mode extra ball.** After two completed modes the extra ball is lit. It is collected
  at the telescope shot.
- **GRAND ORRERY (wizard).** Complete all four modes and the next telescope shot starts the
  wizard: four balls, a 20-second ball save, and jackpots on the four multiball shots that
  start at 5,000,000 and rise by 1,000,000. Six jackpots light the super jackpot at the
  telescope.
- **MIDNIGHT MULTIBALL.** Three balls and a 10-second ball save. The comet ramp, meridian
  ramp, orrery lane and left orbit are jackpot shots, starting at 1,000,000 and rising by
  250,000. Four jackpots light the super jackpot at the telescope for 20 seconds. The right
  orbit can add a ball during the multiball.
- **Super skill shot.** Hit the telescope within 8 seconds of launch for 1,000,000.
- **Stargazer (secret combo).** Shoot the orrery lane, the telescope and the meridian ramp in
  sequence, each step within 12 seconds of the last, for 750,000 points and a bonus
  multiplier step.

The table has its own music, a clockwork night theme in D minor, and its own ambience with
crickets, wind and a soft clock tick.

## Cabinet and look

The cabinet is a wedge. The glass rests on the rails and tilts up toward the back wall: its
underside is 0.065 m above the playfield at the lockdown bar and 0.14 m at the back wall.
Every ramp, wire and tube fits under the glass, and every table is checked for that, for its
outline and for the crossings of its paths.

- **Wood, metal and chrome.** The inner side boards are wood with printed side art on each
  table. The apron is dark brushed metal with a pinstripe and chrome edges. The lane guides
  are metal with a rolled bead and mounting tabs, and the posts carry screw heads.
- **Habitrails.** Wire habitrails are polished stainless steel, with straps and standoffs. Plastic
  ramps are clear and have a stainless entry flap. Tubes are faint, with collars at their
  ends. Ramps that a ball enters from the playfield have a printed entrance sign.
- **Plastics.** Slings, lane guides, back-corner panels and the apron badge are printed
  acrylic pieces with real transparency, stood on chrome standoffs. The pop bumper caps carry
  a printed disc.
- **Reflections and shadows.** On high and medium quality the playfield reflects the balls
  the lights and the flipper tops faintly. Balls cast a tight contact shadow and a softer
  key-light shadow. Painted playfields have soft contact shadows under posts,
  bumpers and standups.
- **Room.** The table stands in a room with a parquet floor and a back wall. The wall shows
  a painted picture shared by all four tables.

## Controls

Keyboard defaults:

| Action | Key |
| --- | --- |
| Left / right flipper | Left Shift / Right Shift |
| Plunger (launch) | Hold Space, then release |
| Start | 1 |
| Nudge left / right / forward | Arrow Left / Right / Up |
| Pause | Escape |
| Camera (ball cam / play / overhead) | C |
| Reset training ball | R |

Standard gamepads: left and right bumpers are the flippers, the right trigger is the
plunger, the Back/Select button (8) starts a game, the Start button (9) pauses, Y (3)
switches the camera, X (2) resets the training ball, and the D-pad (left, right, up)
and the left stick nudge the cabinet.

Settings → Keyboard and Gamepad list every action. Select an action and press the new
key or button. Escape cancels a key capture. If the new binding is already used by
another action, the two actions swap. The Start key falls back to Digit2, KeyS or KeyT
if Digit1 is taken by another action.

In menus, Space and Enter keep their normal behaviour. The game only receives them while
it is playing. Both Shift keys can be held and released together: the input layer reconciles
the paired modifier state, so a flipper cannot stay latched when the browser drops one of the
two key-up events.

## Shared rules

- **Players and balls.** Each player gets three balls. Players alternate ball by ball,
  and the match at the end of the game shows a number. Training gives one player unlimited
  balls.
- **Ball save.** Each ball is protected for 8 seconds from the first switch it hits
  (plus a 1.5-second grace period). A quick drain is relaunched with BALL SAVED. Multiball
  saves last 10 seconds and the wizard 20.
- **Skill shot.** One top lane is lit at random when the ball is launched. Hitting it
  before any other switch awards 250,000. The super skill shot (table-specific, see above)
  awards 1,000,000.
- **Lanes.** Tap a flipper during play to rotate the lit top and lower lanes. Completing
  the three top lanes advances the bonus multiplier. Completing the four lower lanes
  lights the kickback.
- **Bonus multiplier.** It goes up to 5×, and it is counted with the other bonus items at
  the end of each ball. Hold both flippers to skip the count.
- **Combos.** Shots in quick succession within 3 seconds score a combo, worth 100,000 for
  each step after the first.
- **Surprises.** Surviving an unguarded outlane back onto the playfield awards a Lazarus
  Save (350,000). Ripping 16+ rapid spinner turns triggers Hyper Spin (300,000 plus 8
  seconds of overdrive bonus spins), 14 fast bumper hits award Bumper Mayhem (300,000),
  and three orbits within 9 seconds score a Warp Loop (500,000).
- **Extra ball and replay.** Table awards light an extra ball, which is used at the
  table's extra-ball shot. A player who reaches the replay score (40,000,000) gets one
  extra ball per game.
- **High scores.** Each table keeps its top ten. A qualifying player enters three
  initials: flippers choose a letter from A–Z, 0–9, space or the ← (delete) entry, and
  Start or Launch confirms. Entry times out after 45 seconds.
- **Tilt.** Hard nudges add up and give a warning. The third warning tilts the ball: the
  flippers and the coils go dead, the lights and music stop, and the bonus is lost. Passive
  walls and ball contacts keep working. The next ball restores everything.
- **Ball spin.** A ball that is struck or bounced can spin, not just move. A sliding ball
  settles into rolling and loses some speed on the way (on a flat floor it keeps 5/7 of it),
  and wall, post, rubber and flipper contacts can turn spin into sideways motion or back. A
  launched ball starts without spin. On screen the ball turns with its spin.
- **Training.** One player, unlimited balls, no replay awards, and no high-score entry.
  R resets the training ball.

## Settings

- **Graphics quality** (low, balanced, high) changes resolution, shadows, bloom, transparent
  materials and the playfield reflection (high and medium only). It never changes the
  simulation or the table collisions.
- **Camera:** ball cam (the default: the view behind the flippers, zoomed in on about half the
  table, smoothly following the ball), play (the fixed view that keeps the whole playfield
  visible), overhead (for shot placement) and showcase (pauses the game and lets you orbit the
  model). C cycles ball cam, play and overhead. Settings saved before the ball cam existed move
  from play to ball cam once.

  The ball cam follows the lowest ball in play (the one closest to the drain) with a short
  lookahead along its velocity, and stays inside the table so the flippers are fully visible when
  the ball is low (the lowest visible point is 0.03 m above the lockdown bar line, or the bar edge itself while the ball sits in the shooter lane so the plunger stays in view) and the top arc is the upper limit. With several balls far apart it zooms out
  smoothly until the two lowest balls fit (at most the whole table). Balls held in scoops, locks
  or by magnets and balls in the shooter lane keep a steady frame; without a ball in play the view
  rests on the flippers. Switching cameras blends over half a second. With **Reduce effects** (or
  the system reduced-motion setting) the ball cam does not chase the ball and shows the static play
  framing. The maths is the pure `render/follow.js`; the rig is `render/camera.js`.
- **Volumes:** mechanics (effects), music and ambience each have their own volume,
  including true zero.
- **Voice callouts** use on-device voices only, and only for the table's language. When no
  matching voice is installed, the option is disabled and the game stays silent.
- **Reduce effects** turns off camera shake, the glow and the reverb.

Audio starts only after an interaction that starts or resumes a game.

## Pauses and long frames

The game pauses when its window loses focus or is hidden, when a gamepad disconnects,
or when you open a menu. You resume explicitly. A long frame does not pause the game.
The simulation advances by at most 0.05 seconds per frame and at most 32 ticks, so a slow
frame slows the game down but never freezes it or shows a pause dialog.

## Local data

The browser stores validated settings in `aurago.desktop.pinball.settings.v1` and up to
ten results per table in `aurago.desktop.pinball.scores.v1`. This data belongs to that
browser profile. If storage fails, the current session stays playable and the game shows
a warning. Active physics worlds are not saved or synchronized.

## Development and verification

AuraPinball is split into a DOM-free simulation and a thin browser layer:

- `ui/js/desktop/apps/pinball/config.js`, `contracts.js` and `session.js` hold settings,
  the shared vocabulary and the core loop. They import under Node.
- `physics/` runs the 480 Hz engine: `world.js`, `collide.js`, `grid.js`, `flipper.js`,
  `spin.js` (ball spin and friction), `devices*.js` (bumpers, slings, drop targets, holes,
  magnets, captives, gates), `devices-motion.js` (the observatory's orrery disc and
  pendulum) and `paths.js` (ramps, wire habitrails and VUK tubes).
- `rules/` holds the framework (`core.js` lifecycle, `features.js` scoring and lanes,
  `modes.js` modes, locks and multiball, `lamps.js`, `highscore.js`).
- `tables/` contains the four tables. Each table has `layout.js` (geometry), `rules.js`
  (the rule pack), `lamps.js`, and `art.js` and `toys.js` for its look (`art-accents.js`
  adds the lit accents over a painted playfield). `tables/kit/` holds the shared shell,
  lower playfield, the cabinet dimensions (`cabinet-dims.js`) and the printed plastics
  sheet (`plastics.js`). `tables/validate.js` and `tables/validate-paths.js` check every
  layout, including that its ramps and wires fit under the glass.
- `lights/` (lamp shapes, mask, controller and shows), `dmd/` (core, scenes, font,
  raster and the DOM view), `audio/` (engine, SFX recipes, voice, music and the songs in
  `audio/songs/`) and `render/` (three.js scene, art kit, cameras, the cabinet, room and
  apron, ramps, hardware and plastics, the GLB toy loader in `models.js` and the playfield
  reflection) complete the runtime. `game.js` mounts it, `presenter.js` turns rule commands into
  shows, DMD scenes and sound, and `ui/` holds the views and HUD.

Dev tools:

```powershell
node scripts/pinball-dev-server.mjs
# http://127.0.0.1:8792/harness?table=neon&mode=2d
```

The harness takes these parameters: `table` (neon, leviathan, temple or observatory), `mode` (`2d`,
`engine` or `game`), `quality`, `camera`, `debug=1`, `lang`, `scene`, `seed` and
`autoplay=1`. `2d` draws the physics, the flippers, the devices and the event log for
debugging; `engine` renders the three.js table; `game` runs the real app, with `autoplay=1`
letting a bot play.

Survey screenshots (opt-in, visual review):

```powershell
$env:AURAGO_RUN_BROWSER_SMOKE='1'
$env:AURAGO_PINBALL_SURVEY='1'
$env:AURAGO_PINBALL_SCENES='neon:play:low,leviathan:top:high'
go test ./ui -run TestDesktopPinballSurvey -count=1
```

`AURAGO_PINBALL_SCENES` takes `table:scene:quality` items, where the scene is `2d`,
`follow:<seconds>[:narrow|:reduced]` (a real bot game in the ball cam; shots when the ball is low,
high or on a ramp, in the shooter lane and in multiball; `narrow` is a 720x1000 window),
`play`, `top`, `showcase`, `attract`, `action`, `closeup:x,y,h` or `dmd:<scene>`. PNGs go to
`AURAGO_BROWSER_ARTIFACT_DIR`, or to the git-ignored `reports/pinball-survey/`.

Audio render (optional): `TestPinballAudioRender` in
`ui/desktop_pinball_audio_render_browser_test.go` renders the synthesized sound and music
to WAV files. Set `AURAGO_RUN_BROWSER_SMOKE=1` and `AURAGO_PINBALL_AUDIO_RENDER=1`, and
optionally `AURAGO_PINBALL_AUDIO_DIR` and `AURAGO_PINBALL_AUDIO_FILTER` (a regex). The
files go to `reports/pinball-audio/` by default. The test fails on silence, NaN values
and clipping.

Verification:

```powershell
npm run test:pinball
node scripts/build-pinball.mjs --check
node scripts/build-pinball-assets.mjs --check
go test ./ui -run "TestVirtualDesktopFirstPartyJSFilesStayBelowLineBudget|TestFrontend_StaticI18nKeysExistInEnglishBundle|TestDesktopDynamicI18nPrefixKeysExistInAllLanguages|TestShippedWebAssetsLoadNothingExternal" -count=1
go test ./internal/desktop -run TestBuiltinPinballApp -count=1
$env:AURAGO_RUN_BROWSER_SMOKE='1'
go test ./ui -run TestDesktopPinballBrowser -count=1 -timeout 20m
```

`npm run test:pinball` runs the pinball suites in order: config, rules, input, physics,
devices, paths, tables, reachability, the four layouts, the four rule packs, lights, DMD,
audio, music, session and assets. The tables suite includes the path-fit checks (glass,
outline and crossings). The assets suite also checks that every `pinball.*` English key
exists in all 16 locales.

The browser test covers the lobby and attract loop, each table's canvas and DMD, training
play, the flipper response, a long frame that does not pause, focus loss, settings
persistence, and the WebGL and asset-error retry. Optional environment variables:
`AURAGO_PINBALL_BENCHMARK=1` (with `go test -v -timeout 25m`) runs a three-minute
1920×1080 High-quality multiball stress per table, and `AURAGO_PINBALL_GPU=1` lets it use
the available hardware GPU. `AURAGO_BROWSER_ARTIFACT_DIR` keeps the captures.

Software rendering (SwiftShader) is only for functional checks. The 1080p/60 target is
for RTX 3060 or RX 6600 class hardware, and human live and drop-catch assessment is a
separate acceptance step.

Asset builds are described in `assets/pinball/AGENTS.md` and `ui/3d/pinball/AGENTS.md`.

Art pipeline: `node scripts/pinball-art-guide.mjs playfield <id|all>` and
`node scripts/pinball-art-guide.mjs plastics` write the layout guides to the git-ignored
`reports/pinball-guide/`. The painted playfields, plastics sheets, side art, the observatory
backglass and the room wall are made with the Codex CLI image tool, using these guides as
reference images, and each prompt is recorded in `assets/pinball/prompts.json`.
`assets/pinball/prepare.py` converts the masters into the delivered WebP files, and
`node scripts/build-pinball-assets.mjs` writes `ui/3d/pinball/v1/manifest.json` (version 3).
The observatory toys are one GLB built by `assets/pinball/blender/observatory_toys.py` in
Blender's background mode. The script is the source; there is no `.blend` file to keep.

## Credits

Original table artwork: the concept and backglass masters of the first three tables were
made with the built-in ImageGen. The painted playfields, plastics sheets, side art, the
observatory backglass and the room wall were made with the Codex CLI image tool. The 20
retained masters, their prompts and source hashes are in `assets/pinball/`. The lobby table cards (`<id>-preview.webp`, 640x320) are not painted art but
renders of the real procedural tables from the survey harness. To re-make them run
`AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_PINBALL_SURVEY=1 AURAGO_PINBALL_SCENES="neon:preview:high,leviathan:preview:high,temple:preview:high" go test -run TestDesktopPinballSurvey -count=1 -timeout 20m ./ui` from the repo root, then encode the
`reports/pinball-survey/<id>-preview-high.png` shots with ffmpeg as described in
`ui/3d/pinball/AGENTS.md`.

Sound samples: 29 CC0 1.0 recordings from seven archives: the Kenney Impact Sounds and Music
Jingles packs, and five other CC0 packs listed in the manifest. Each delivered clip records
its source member, archive pin and SHA-256 hash in `ui/3d/pinball/v1/manifest.json`.

Graphics runtime: Three.js (MIT), including its GLTFLoader for the observatory toys. All other sound effects, music and table geometry are
synthesized or generated procedurally in code.
