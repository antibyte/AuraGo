# Game Maker: AuraGo Low Poly

The Assets dialog now combines the eighteen original sprite packs with 220
original low-poly 3D models. Choose the 3D category, search and select individual
models for the next game request. The inspector supports orbit/zoom, real animation
clips, pause and detail levels. Closing it releases the renderer and model data.

The pack is MIT licensed and versioned as `aurago-low-poly@1.0.0`. No Blender,
online asset service or CDN is required when playing an exported game.

## Agent workflow

Use the existing `game_maker_asset` tool:

1. `search_assets` with `view: "3d"` and a focused query.
2. `describe_asset` for each selected ID. Read dimensions, orientation, sockets,
   moving parts, collider and actual animation names.
3. Submit plan schema 2, `units: "metres"`, exact version/asset IDs, positive
   `scale` and a 3D collider. Up to 64 roles are supported. Sprite-only origin
   and display-height fields are unnecessary.
4. After acceptance, use the imports supplied by the server. Additional imports
   require `import_pack` with an explicit `asset_ids` list.
5. Use returned metadata paths and `three_example`. Never invent filenames or
   clip/bone/socket names. Check the actual rendered game before finishing.

```json
{"operation":"import_pack","pack_id":"aurago-low-poly",
 "asset_ids":["road-pickup","humans-pilot-a","architecture-garage"]}
```

Imports copy only selected GLBs and shared dependencies into the project.
Identical imports are reused; modified files are preserved and reported as
conflicts. Failed imports roll back newly published files. Existing file limits,
project boundaries and tool permissions still apply. Sprite imports stay compatible.

## Runtime helper

The pinned Three.js 0.185.1 helper uses official GLTFLoader, SkeletonUtils and
OrbitControls. It is copied into every 3D game's local `vendor/` directory.

```typescript
import * as A from '../vendor/aurago-three-assets-1.js';
import meta from '../assets/builtin/aurago-low-poly/1.0.0/assets/humans-pilot-a.json';

const abort = new AbortController();
const asset = await A.loadAsset(meta, 'humans-pilot-a',
  'assets/builtin/aurago-low-poly/1.0.0/', {signal: abort.signal});
const pilot = A.createInstance(asset);
scene.add(pilot.root);
A.playAction(pilot, 'walk');

// In the game's existing loop; movement uses the clip's metres/second metadata:
A.updateInstance(pilot, deltaSeconds, camera);

// On game teardown:
abort.abort();
A.disposeInstance(pilot);
A.releaseAsset(asset);
```

Every animated instance has independent skeletons and mixers. Geometry and
materials are shared. Use `createInstances(asset,matrices,lod)` only for static
models; use separate instances for animated or articulated objects.
`attachToSocket` returns a detach function and preserves LOD choice after detach.
`setPart` rotates/translates declared pivots. It does not simulate vehicle physics.

Metres, +Y up and +Z forward are consistent across the catalog. Ground origins,
centered aircraft/planets and FPS view origins are explicitly distinguished.
Architecture provides 2 m modules, 3 m storeys, openings and connector metadata;
roads use an 8 m width. Use compound colliders for walkable interiors rather than
one enclosing box that seals their doors.

Humans share 34 in-place actions. Animals have species-specific actions.
FPS `fps_binding` describes the common view frame and pistol action prefix.
Play matching arm/weapon clips together, with one clock; shot and magazine
events drive the game's own logic. Missing actions/rig mismatches are errors.
The pack does not include ragdolls, speech-face animation, physics or combat AI.

## Export and verification

Project-local model files and runtime are included in ZIP exports. Host the
extracted directory with an ordinary static HTTP server; browser `file://` loading
does not support these modules consistently. Games do not access Studio's catalog
or credentials. The Studio preview retains its opaque iframe sandbox.

Production and developer commands are in
`assets/game-maker-low-poly/README.md`. Acceptance exports/screenshots belong in
ignored `reports/low-poly/`. The normal agent's startup check remains distinct
from full gameplay verification; a successful GLB parse is not a visual review.
