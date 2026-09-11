# AuraGo game presentation 1.0.0

The existing Game Maker catalog contains `aurago-effects` (44 effects and eight
atmospheres) and `aurago-sounds` (40 sounds). Runtime data is local. Original
code/synthesis is MIT; the five retained recording sources are CC0. The source
lock and each sound's manifest contain origin, author, license, mastering and
SHA-256. Production inputs never enter exported games.

## Agent workflow

Use `search_assets` with `asset_kind: "effect"` or `"audio"`, then
`describe_asset`. Include semantic choices in `set_design`:

```json
{
  "base": "fps",
  "objective": "Clear the forest patrol route",
  "features": ["WASD movement", "aim and shoot", "reload", "restart"],
  "presentation": {
    "environment": "forest-rain",
    "effects": ["blood-spray", "blood-pool", "muzzle-flash"],
    "sounds": [
      {"event": "step", "sound": "step-grass"},
      {"event": "shot", "sound": "rifle"},
      {"event": "hit", "sound": "impact-flesh"}
    ],
    "quality": "auto"
  }
}
```

Model/sprite roles are selected through the existing `assets` field. The server
resolves schema 3, versions, exact paths and dependencies after plan acceptance.
`src/presentation.json` is generated data. Guided templates already own all
controller hooks. Existing schemas 1/2 remain valid; omitted presentation
preserves the prior selection during edits. A manual `import_pack` requires
exact `asset_ids` and uses the same atomic, non-overwriting import transaction.

## Custom game hooks

```js
import config from './presentation.json';
import {createPresentation, createThreeAdapter} from '../vendor/aurago-effects-3d-1.js';
const fx = createPresentation({config, root,
  adapter: createThreeAdapter({scene, camera, renderer, sun, ambient})});
const removeGround = fx.registerSurface(ground, {kind: 'ground'});
const removeRoof = fx.registerSurface(roof, {kind: 'roof'});
// In the existing game loop, dt is seconds:
fx.audio.listener(camera.position.toArray(), camera.getWorldDirection(direction).toArray());
fx.update(dt);
fx.render();
// Only in the actual ray/contact callback, with a world-space surface normal:
fx.event('hit', hit.point.toArray(), worldNormal.toArray(), 'metal');
// On lifecycle transitions:
fx.setPaused(true);
fx.reset();
// On disposal:
removeGround(); removeRoof(); fx.dispose();
```

For 2D, import `createPhaserAdapter` from `aurago-effects-2d-1.js`, pass
`{scene: this, view: 'side'}` or `view: 'top'`, and call `update(dt)` from the
existing scene update. Phaser renders normally. HUD objects use depth >=1000;
3D HUD stays in DOM. Canvas mode reports its shader fallbacks.

`set(id, params)` configures an imported continuous effect; point emitters such
as fire or engine trails require `position`. `emit(id, {position, normal})`
triggers a burst. `applyObject(object, 'hologram'|'dissolve'|'hit-flash')` returns
a release callback. `setEnvironment(id)` replaces active atmosphere components.
Positions use world pixels in 2D and metres in 3D. `describe_asset` supplies
supported default parameters. Ground/roof meshes must have current transforms.
In 2D, water width/depth and its default vertical offset are viewport percentages;
an explicit position anchors the water in world pixels. In 3D they are metres.

Audio starts after a real interaction. `audio.play(id, {position})` supports
spatial sources; `audio.stop(id)` stops a continuous source. `setRoom` accepts
`outside`, `small-room`, `hall`. `setMuted` and `setVolume` control the shared
mixer. Generated music can use `audio.context` after unlock and connect through
`audio.connectMusic(node)`; disconnect it during game teardown.

Day/night defaults to 480 game seconds; `set('day-night', {fixed:true,hour:22})`
fixes the hour. Pause freezes simulation. Low/medium/high/auto bound particles,
decals, render resolution, reflections and postprocessing. Auto changes only
after sustained load. No controller creates a second frame loop. Blood can be
disabled with `setBlood(false)`; Reduced Motion limits flashes/distortion.

## Reproduce and verify

Production requires Python with NumPy, ffmpeg and the repository's pinned Node
dependencies. First acquisition checks CC0 pages and locks original hashes.
Normal rebuilds reuse the reviewed originals:

```text
python -X utf8 assets/game-maker-presentation/acquire.py
python -X utf8 assets/game-maker-presentation/build.py --check
node scripts/build-game-maker-presentation.js --check
go test ./internal/gamemaker
```

Remove `--check` to regenerate. The JS build pins Three.js 0.185.1 and exposes
Water's render-target disposal with a checked build-time patch. Tests validate
all WAV formats, peaks, hashes and total uncompressed runtime against 64 MiB.

Set `GAMEMAKER_PRESENTATION_BROWSER=1` for actual Chrome scenes and Studio;
`GAMEMAKER_PRESENTATION_GALLERY=1` additionally captures every effect in both
engines. `GAMEMAKER_GUIDED_BROWSER=1 GAMEMAKER_PRESENTATION=1` exercises all
guided templates. Screenshots and metrics go to ignored `reports/`.
`GAMEMAKER_EVAL_PRESENTATION=1` extends the opt-in live provider comparison.
Listening quality and hardware frame-rate acceptance require actual observation;
decode, compile and startup checks do not substitute for them.
