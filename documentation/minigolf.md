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
release to putt; a dashed guide line, a color-graded power ring at the ball
and the strength slider show the shot. Arrow keys adjust aim and strength
while held (Shift gives finer steps); Space putts and Escape pauses. A gentle
aim assist snaps shots within four degrees toward the cup, and after a
penalty the rest position gets a marker ring. Right-drag rotates the view,
the wheel zooms, and **To the ball** restores focus. Optional settings move
the camera behind the ball on each turn and follow the rolling ball; reduced
motion keeps both functional without smooth transitions. Touch users can
switch between Camera and Aim; in Aim a one-finger drag from anywhere putts,
and two fingers zoom/pan.

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

Audio starts only after a pointer or keyboard gesture. Music starts at 20% and
effects at 65%; zero and mute survive reopening. The three CC0 MIDI originals
are converted reproducibly into the existing Synth Studio project format and
played through local synthesized GM voices. Source URLs, authors, licenses and
hashes are recorded in `assets/minigolf/music/` and the runtime music manifest.
There is no MIDI device permission request, SoundFont download or external stream.
AuraGolf queues 750 ms of music ahead to bridge short rendering stalls. Synth
Studio retains its existing 120 ms scheduling reserve for editor playback.

All effects stay local and synthesized: material-specific impacts (drum
voices for stone and sand), a sustained rolling voice whose pitch follows ball
speed, stereo panning from the ball's screen position, layered cup drop,
splash and win fanfare (with a brief music duck), and per-world ambient
events during play — garden birds, coast gulls and surf, night crickets —
that follow the effects volume and stop while paused or a panel is open.

Minimizing or hiding the browser tab pauses simulation and audio. Pause also
freezes the current shot. Closing releases rendering, physics and audio
resources. Reduced motion follows the OS/Desktop preference or the game's
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
Course generation is deterministic; on Windows a transient file lock can
abort one export — rerun it until it reports `Wrote 18 courses`.

```
npm run build:minigolf
node scripts/build-minigolf-music.mjs --check
node scripts/build-minigolf.mjs --check
npm run test:minigolf
node scripts/build-ui-bundles.js --check
```

Run `TestDesktopMinigolfBrowser` with `AURAGO_RUN_BROWSER_SMOKE=1` for actual
WebGL integration, controls, lifecycle and responsive review. Add
`AURAGO_MINIGOLF_BENCHMARK=1` for 240-frame-interval 1080p measurements at each
graphics quality, and `AURAGO_BROWSER_ARTIFACT_DIR` for screenshots and short WAV previews.
The AuraGolf browser fixture mutes speaker output while exercising Web Audio.
Set `AURAGO_BROWSER_MUTE_AUDIO=1` to mute the accompanying Synth Studio browser
regressions as well; offline WAV previews still contain the rendered audio.
Optional browser artifacts belong in ignored `reports/minigolf/`. Render measurements are specific
to the tested browser/GPU; software rendering is not hardware acceptance.

The course tests replay a verified route into every real cup. The optional
`AURAGO_MINIGOLF_SEARCH=1` mode searches for routes after geometry changes;
normal regression runs use the recorded shots instead.

The app is registered through the usual Desktop manifest, lazy module loader,
app routing and disposal map. It has no new HTTP endpoints or server state.
Release and installation use the normal version-bound web asset packaging.
