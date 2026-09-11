---
name: aurago-game-assets
description: Select local sprite packs or original 3D models and integrate game art, animation, music, and procedural audio.
license: MIT
compatibility: AuraGo offline sprite library; optional image and music integrations
metadata:
  managed_by: aurago
  source: AuraGo synthesis
  phaser_commit: 41be1e462bc600064e498cba370bfa8c5c055a22
  threejs_commit: 7221c1f4a6d2ae189a4d85d058d24f3228499d46
allowed-tools: game_maker_project, game_maker_file, game_maker_asset, game_maker_validate
---

# Game Assets

Ask for only assets that materially improve the current game.

## Original 3D models

The same library contains `kind: model3d` pack `aurago-low-poly` version `1.0.0`:
220 original MIT models, metres, +Y up, +Z forward. Use `view: 3d` when searching.
The initial `user_selected_model_ids` are the user's intended selection. Describe
each required model and record its exact ID/version, rig and actions in plan v2.
Selection is not an import: after plan acceptance the server imports the selected
dependency closure. Additional `import_pack` calls require `asset_ids: ["exact-id"]`
(1–64 models). Never import all models, guess file paths, sockets, bones or clips.
Use the returned `manifests` and `three_example`. Project copies are immutable on
reimport and independent of future catalog versions. No Blender service is needed.
The build/repair prompt includes the complete public runtime API and import
examples. Use those directly; do not read the minified vendor bundle or re-import
unchanged models just to retrieve an example. Read needed source/metadata once,
then write the game. An unchanged starter demo is rejected during validation.

Import `../vendor/aurago-three-assets-1.js`. Its `loadAsset(meta,id,baseURL,{signal})`
loads verified local GLBs and the shared humanoid clip library. `createInstance`
gives each figure an independent skeleton/mixer while sharing mesh data. Add
`instance.root` to your scene. Start only declared actions with `playAction`;
call `updateInstance(instance,dt,camera)` from the existing game loop. Clip `speed`
is metres/second for matching world movement; clips are in place.
`attachToSocket(instance,socketID,object)` returns an idempotent detach function.
Align the held object's documented grip first; attaching does not invent alignment.
`setPart(instance,node,radiansOrMetres)` drives declared wheels, rotors and slides.
`createInstances(asset,matrices,lod)` is for repeated static models only.
Dispose instances before `releaseAsset(asset)`; abort pending loads on teardown.
Do not add an animation loop per model. Pause simulation with the existing game.

Use `bounds`, `collider` and `connections` for collisions and modular placement.
Architecture follows a 2 m grid / 3 m storey; roads are 8 m wide. Centered aircraft,
planets and FPS view rigs differ from ground-based origins. No physics, ragdoll,
speech-face animation or combat AI is included. Implement game rules separately.

Compact starting selections (describe first; choose only what the game needs):

| Game | Models | Essential integration |
|---|---|---|
| Transport | `road-pickup`, `props-crate-wood`, `architecture-warehouse` | wheel pivots, cargo socket, depot collision |
| Flight | `aircraft-prop-plane`, `props-checkpoint`, `landscape-island` | +Z flight, propeller pivot, ordered gates |
| Space | `space-scout`, `space-asteroid-split`, `space-planet-earth` | centered origins, muzzle socket, hit spheres |
| Exploration | `humans-explorer-a`, `animals-wolf`, `vegetation-pine` | independent walk/idle clips, static instancing |
| FPS | `fps-arms-modern`, `fps-rifle`, `humans-trooper-b` | synchronized clips, shot/magazine events, own hit rules |

For the authored FPS view, place weapon and arms as siblings: weapon translation
`[0.12,0.01,0.145]`, arms at zero. Start matching actions together and advance both
with the same dt. Rifle support is the default; pistol/energy-pistol arms use the
`pistol_` action prefix. Recoil is already baked in both rigs: do not apply it twice
by parenting the animated weapon under the moving hand. Use a shared view parent
to position the whole rig relative to your camera. Metadata events are timing
markers; the game owns ammo, projectiles, damage and audio.

## Existing 2D sprite packs

- Prefer matching built-in sprite packs. Initial context contains a compact
  catalog and paths of user-selected packs already imported before your turn.
  With no selection, choose suitable packs. Use `game_maker_asset` with `job_id`
  and `operation: search_assets`, `query`, optional `pack_id` and `view` (side/top/board).
  Search returns six candidates by default, at most twelve, complete assemblies
  before fragments. Then `describe_asset` with pack_id and asset_id OR assembly_id
  gives exact variants, actions, transform rules, missing actions and usage code.
  `list_packs`/`describe_pack` remain available. `import_pack` copies PNG and JSON together and returns exact local
  paths. It needs edit permission, not a media generator.
- Pack IDs and versions come only from tool results or the accepted plan.
  From `src/main.ts`, import `../assets/builtin/<id>/<version>/sheet.json`;
  a runtime PNG URL remains `assets/builtin/<id>/<version>/sheet.png`.
  For a nested source file, resolve the metadata import from that file's folder.
  `asset_import_invalid` rejects a write before replacing the existing file and
  lists usable imports. Correct that import, then resubmit the same focused edit.
  Search/describe does not import a pack. Never invent a pack or change versions
  to repair collisions; retain the existing sprite wiring and accepted plan.
- Follow the import response's `phaser_example`, including for user-selected
  packs supplied in the initial context. Load the PNG with `load.spritesheet`
  and 64×64 frames. `load.image` loads the entire 640×640 sheet as one texture;
  passing a frame later does not slice it. `sheet.json` is AuraGo metadata,
  not a Phaser texture-atlas file. Import that JSON in TypeScript, then select
  numeric frames by asset ID. Do not use `load.atlas` for these packs.
- Read imported `sheet.json` with `game_maker_file`. Schema version 1 uses numeric
  frames 0–99 in ten rows and ten columns of 64px cells. Asset IDs, descriptions,
  direction, origin, ordered frames and timing are authoritative. Never invent
  indices or treat consecutive assets as an animation. Namespace texture and
  animation keys with pack ID AND version. Pack version 2 adds explicit `entity`,
  `action` and `transform` fields. Only transform.flip_x permits mirroring;
  there is no category-wide flip rule. Top-down characters have separate directional sequences. Creature
  resting animations reuse movement frames. Repeated poses are deliberate holds.
- Keep both files at `assets/builtin/<pack-id>/<version>/`. Identical imports are
  repeatable; modified/incomplete copies are never overwritten. Preserve the
  provenance and license in JSON. Revisions and ZIP exports use project copies,
  never the Studio catalog API or a CDN.
- Buildings and large vehicles include `assemblies`: exact width/height, origin
  and ordered parts with `asset_id`, numeric `frame`, pixel `x`/`y`, and optional
  `animation_id`. `assembly_part` assets are fragments, not complete objects.
  Put parts in one Phaser Container, use `.setOrigin(0, 0)` on each part and
  subtract the assembly origin from all positions as shown in `phaser_example`.
  Never fit each fragment separately. Move/scale/flip the entire container.
  Register part animations first and start them together so tracks/rotors stay
  synchronized. Assembly parts intentionally reach cell edges to avoid seams.
  Read the exact assembly direction: some side vehicles face left. Vehicles with
  transform.mode rotate use atan2(dy,dx) minus transform.forward_radians; all
  angle values are radians. Buildings/terrain/cards/signs are fixed unless
  explicitly permitted. Top-down robots contain up/right/down/left movement and idle.
- Use `operation: generate` (or omit operation) for missing custom content.
  Generation alone needs the configured media capability.
- Plan the full asset list before the first request and batch what belongs
  together. Prefer one sprite sheet or one texture atlas over many single
  images; stay within roughly four images and one music track per job.
- Images may serve as sprites, sprite sheets, textures, backgrounds, decals, or
  UI art. Request transparent backgrounds when useful and keep dimensions
  modest.
- Music should be instrumental, loop-friendly, and aligned with the intended
  pace. Keep playback opt-in after a user gesture and expose mute or volume.
- Sound effects use procedural Web Audio unless a configured generator is
  explicitly available. Create the AudioContext after player interaction.
- A disabled generator, provider error, or budget limit is a normal fallback.
  Use shapes, gradients, particles, noise, or synthesized tones and keep the
  game playable.
- Reference only the project-local path returned by `game_maker_asset`, and
  confirm the exact path before wiring it into loaders or textures.
- Never delete or mutate the global AuraGo media registry asset. Project
  deletion removes only the project copy and ledger provenance.

For guided templates, supply semantic roles in `set_design`: player, enemy,
projectile, item, goal, obstacle; board uses cell and marker roles. Numbered
variants (enemy_1, enemy_2) are reused through the shared sprite helper. Guided
3D defaults cover the five common genres; override roles with exact model IDs.
All chosen manifests and loader paths are generated; do not read minified vendors.

## Exact discovery sequence

```json
{"job_id":"CURRENT_JOB_ID","operation":"search_assets","query":"robot","view":"top","limit":6}
```
```json
{"job_id":"CURRENT_JOB_ID","operation":"describe_asset","pack_id":"robots-drones-animated-top-down","asset_id":"service_robot_move_down"}
```
Use actual job_id from trusted job context. Plan the returned version/IDs first;
imports are permitted only after plan acceptance. This robot has no attack
animation: plan a projectile and optional effect, not an invented robot_attack.
`preloadPack`, `registerAnimations`, `createAsset`, `createAssembly`, `setFacing`,
and `playAction` from `../vendor/aurago-game-1.js` implement the metadata contract.
The Phaser skill provides a complete scene. For assemblies use createAssembly
with the returned assembly ID; move/scale/rotate only its container. Use a
separate collision proxy; no physics bodies on children. Visual transforms never
automatically transform a rectangular Arcade collider. Existing v1 project files
remain unchanged; import v2 alongside them when new helpers need transform data.

## Effects and sound

Use `search_assets` with `asset_kind: "effect"` or `"audio"`, then
`describe_asset` with the returned pack/ID. `aurago-effects@1.0.0` and
`aurago-sounds@1.0.0` are local. Put semantic choices in `design.presentation`:
`{"environment":"forest-rain","effects":["muzzle-flash","blood-spray"],"sounds":[{"event":"shot","sound":"rifle"},{"event":"step","sound":"step-grass"}],"quality":"auto"}`.
The server generates schema 3, imports selected dependencies and writes
`src/presentation.json`. Do not put effects/audio in visual actor roles.
Direct imports require exact `asset_ids`; atmosphere imports include sounds.
Never read minified vendors or invent shader parameters. Use returned defaults.
Guided templates handle effects, real hits, pause and restart. Custom games use
the public controller example in the compact job context.
