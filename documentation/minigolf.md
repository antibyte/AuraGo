# AuraGolf

AuraGolf is the built-in Games app for eighteen low-poly 3D minigolf holes.
It runs locally in the browser after loading the matching AuraGo web resources.
WebGL2 and hardware acceleration are required. Blender is an authoring tool,
not an installation or server dependency.

## Playing

Choose all eighteen holes, a six-hole world, or a single practice hole. All
worlds are available immediately. One to four players share the same device
and take turns after every shot. Other players' translucent ball markers show
their names and never block the active ball. Pull back from the ball and
release to putt. Strength follows the on-screen drag distance (independent of
camera zoom); releasing next to the ball, a right click, a second finger or
Escape cancels the drag. A dashed preview path shows the roll up to the first
rail and a short stretch of the rebound, a color-graded ring marks the
strength, and the strength readout adds the estimated flat roll distance (full
strength rolls 28 m on flat felt). Arrow keys adjust aim and strength while
held (Shift gives finer steps); Space putts and Escape pauses. A gentle aim
assist snaps shots within four degrees toward the cup, and after a penalty the
rest position gets a marker ring. Right-drag rotates the view, the wheel
zooms, and **To the ball** restores focus. Optional settings move the camera
behind the ball on each turn and follow the rolling ball; reduced motion keeps
both functional without smooth transitions. Touch users can switch between
Camera and Aim; in Aim a one-finger drag from anywhere putts, and two fingers
zoom/pan.

The stage shows a hole map with course bounds, water, tee, cup and every
ball. Water and out-of-bounds cost one penalty stroke and return the ball to
its previous shot position. Twelve unsuccessful putts finish the hole with 13.
The scorecard appears when everyone finishes a hole, colors
Eagle/Birdie/Par/Bogey results and picks the lowest total.

Progress is saved after completed shots in namespaced browser localStorage.
Reopening resumes the last settled position, without counting an interrupted
shot. Records and saves belong to that browser/profile, not the server. If
browser storage is blocked, the game remains playable and reports that progress
cannot be saved. Practice does not overwrite course best scores.

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

Reduced motion follows the OS/Desktop preference or the game's
checkbox, disabling decorative particles and camera movements while retaining
the movement of actual gameplay obstacles. Graphics quality can be set manually;
Automatic reduces quality when measured rendering is persistently slow.

## Assets and builds

Blender 5.2.2 LTS generates the original scenes, models and collision metadata
from `assets/minigolf/`. Runtime resources are GLBs and JSON under
`ui/3d/minigolf/`; authoring scenes are not included in the web resource archive.
All models use metres, Y up, and a 0.16 m sphere collider. Course collision
meshes include real cup openings and stay independent of visual effects. Each
course adds world-specific perimeter decor (fences and a bench, driftwood and
buoys, glowing shards), pebble clusters, a dark cup lining whose floor sits
above the island turf fan, and a seven-segment hole number on the flag pole.
The isolated renderer uses Three.js 0.186.1 and Rapier 0.21.0 with 120 Hz fixed
physics updates, continuous collision detection and bounded catch-up work.
The ball rolls with its own model instead of Rapier contact friction: a
constant plus a speed-proportional deceleration per floor surface, rolling
spin from launch, rails that bounce (wood 0.78, stone 0.82), trimesh
internal-edge fixing and a 5 mm contact skin so seams do not brake the ball.
The aim preview re-simulates the shot in a cloned physics world with the same
rolling code.
Course generation is deterministic; on Windows a transient file lock can
abort one export — rerun it until it reports `Wrote 18 courses`.

```
npm run build:minigolf
node scripts/build-minigolf-audio.mjs --check
node scripts/build-minigolf.mjs --check
npm run test:minigolf
npm run test:synth-studio
node scripts/build-ui-bundles.js --check
```

`npm run test:minigolf` runs the model, physics, asset, course and audio checks.
`npm run test:synth-studio` runs the Synth Studio checks that used to live in the
audio test; AuraGolf itself no longer depends on Synth Studio.

Rebuild the sound pack only when its recipe or sources change:
`node scripts/build-minigolf-audio.mjs --write --sources <download folder>`. It
needs ffmpeg 8 or newer with libvorbis, plus bsdtar (on Windows, `tar.exe`).
`--check` needs only Node.js. The download list, provenance and rebuild steps are
in [`assets/minigolf/audio/README.md`](../assets/minigolf/audio/README.md).

Run `TestDesktopMinigolfBrowser` with `AURAGO_RUN_BROWSER_SMOKE=1` for actual
WebGL integration, controls, lifecycle and responsive review. Add
`AURAGO_MINIGOLF_BENCHMARK=1` for 240-frame-interval 1080p measurements at each
graphics quality, and `AURAGO_BROWSER_ARTIFACT_DIR` for screenshots and the
active-play audio statistics.
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
