# AuraPinball

AuraPinball is a built-in Games app with three original tables: Cyber Neon
Metropolis, Deep Sea Leviathan and Lost Temple of the Sun. All tables are available
immediately. Play alone, or with up to four alternating players on the same computer.
No provider key or internet service is required. The game, its tables, lights,
sound and music ship inside AuraGo.

The physics is a custom deterministic 2.5D engine at a fixed 480 Hz tick. The tables
are built procedurally in three.js, the lamps and the 192×48 dot-matrix display (DMD)
are drawn in code, and all sound effects and music are synthesized at runtime.

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
it is playing.

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
- **Extra ball and replay.** Table awards light an extra ball, which is used at the
  table's extra-ball shot. A player who reaches the replay score (40,000,000) gets one
  extra ball per game.
- **High scores.** Each table keeps its top ten. A qualifying player enters three
  initials: flippers choose a letter from A–Z, 0–9, space or the ← (delete) entry, and
  Start or Launch confirms. Entry times out after 45 seconds.
- **Tilt.** Hard nudges add up and give a warning. The third warning tilts the ball: the
  flippers and the coils go dead, the lights and music stop, and the bonus is lost. Passive
  walls and ball contacts keep working. The next ball restores everything.
- **Training.** One player, unlimited balls, no replay awards, and no high-score entry.
  R resets the training ball.

## Settings

- **Graphics quality** (low, balanced, high) changes resolution, shadows, bloom and
  transparent materials. It never changes the simulation or the table collisions.
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
  `devices*.js` (bumpers, slings, drop targets, holes, magnets, captives, gates) and
  `paths.js` (ramps, wire habitrails and VUK tubes).
- `rules/` holds the framework (`core.js` lifecycle, `features.js` scoring and lanes,
  `modes.js` modes, locks and multiball, `lamps.js`, `highscore.js`).
- `tables/` contains the three tables. Each table has `layout.js` (geometry), `rules.js`
  (the rule pack), `lamps.js`, and `art.js` and `toys.js` for its look. `tables/kit/` holds
  the shared shell and lower playfield.
- `lights/` (lamp shapes, mask, controller and shows), `dmd/` (core, scenes, font,
  raster and the DOM view), `audio/` (engine, SFX recipes, voice, music and the songs in
  `audio/songs/`) and `render/` (three.js scene, art kit, cameras and mechanical objects)
  complete the runtime. `game.js` mounts it, `presenter.js` turns rule commands into
  shows, DMD scenes and sound, and `ui/` holds the views and HUD.

Dev tools:

```powershell
node scripts/pinball-dev-server.mjs
# http://127.0.0.1:8792/harness?table=neon&mode=2d
```

The harness takes these parameters: `table` (neon, leviathan or temple), `mode` (`2d`,
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
devices, paths, tables, reachability, the three layouts, the three rule packs, lights, DMD,
audio, music, session and assets. The assets suite also checks that every `pinball.*`
English key exists in all 16 locales.

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

## Credits

Original table artwork: built-in ImageGen. The nine retained masters (concept,
backglass and playfield per table), their prompts and source hashes are in
`assets/pinball/`. The lobby table cards (`<id>-preview.webp`, 640x320) are not painted art but
renders of the real procedural tables from the survey harness. To re-make them run
`AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_PINBALL_SURVEY=1 AURAGO_PINBALL_SCENES="neon:preview:high,leviathan:preview:high,temple:preview:high" go test -run TestDesktopPinballSurvey -count=1 -timeout 20m ./ui` from the repo root, then encode the
`reports/pinball-survey/<id>-preview-high.png` shots with ffmpeg as described in
`ui/3d/pinball/AGENTS.md`.

Sound samples: 13 mechanical and jingle recordings from the Kenney Impact Sounds and
Music Jingles packs, both CC0 1.0. Each delivered clip records its source member, archive
pin and SHA-256 hash in `ui/3d/pinball/v1/manifest.json`.

Graphics runtime: Three.js (MIT). All other sound effects, music and table geometry are
synthesized or generated procedurally in code.
