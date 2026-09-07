---
name: aurago-game-assets
description: Select offline sprite packs and integrate bounded game art, music, textures, and procedural audio.
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
