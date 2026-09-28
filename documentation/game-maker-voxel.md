# Voxel games in Game Maker Studio

Choose **Voxel** when creating a project. Describe a survival or creative game;
survival is the default. The agent authors the world, recipes, opponents and rules
using the installed Three.js 0.185.1 runtime. The mode stays fixed during play.
The existing feature, creation, edit and deletion permissions still apply.

## Playing

The objective and instructions appear before **Start game**. During play, health,
selected item, optional goals and the nine-slot hotbar remain compact.

| Action | Desktop | Touch |
| --- | --- | --- |
| Move | WASD | Left stick |
| Look | Mouse / drag; arrows also work | Drag the scene |
| Jump | Space | Jump |
| Mine / attack | Left mouse; E alternative | Hold Mine / attack |
| Place | Right mouse; F alternative | Place |
| Choose quick slot | 1–9 or hotbar | Hotbar; scroll on narrow screens |
| Inventory / crafting | I | Inventory |
| Pause / help | Esc, P or pause button | Pause button |
| Respawn after death | R, Enter or Respawn | Respawn |

Inventory and pause freeze the single-player simulation. Crafting consumes all
ingredients and creates the output together, or changes nothing. The inventory
has 36 slots; tap a stored item to swap it with the selected quick slot. Creative
inventory also offers every declared material/tool without requiring resources.
The standard survival progression is wood, stone, then metal tools. Enemies
either pursue for melee attacks or guard and shoot blocked-by-terrain projectiles.

Death/respawn retains the modified world and inventory. Restart from pause returns
to the spawn without resetting the world. **Reset world** in inventory requires
confirmation and the existing deletion permission in Studio.

## Limits and world data

| Setting | Supported range |
| --- | --- |
| Dimensions (x, y, z) | Default 96×48×96; maximum 128×64×128; multiples of 16 |
| Scale / chunks | One metre per block; 16×16×16 blocks per chunk |
| Terrain | `flat`, `hills`, `island`, deterministic string seed |
| Palette / items / recipes | 1–64 of each; numeric block IDs 1–64, air 0 |
| Opponents | Melee or ranged; at most 24 total |
| Goals | Up to eight collect/craft/place/defeat goals; optional |
| Definition / save | 64 KiB JSON definition / 4 MiB saved state |

The generator uses the lowest numeric ID for each required terrain material:
grass, dirt, stone, wood, leaves, ore and sand. Additional palette entries can be
crafted and placed. Original small block textures are generated locally from the
palette, without downloads. Only exposed faces become chunk geometry. Changed
blocks invalidate neighboring geometry; work is spread across frames. Physics,
aiming, rendering and saves use the same grid. Placement rejects player/enemy
overlap, occluded targets and world boundaries.

Mobile reduces visible distance (40 rather than 88 metres) and pixel ratio; world
and gameplay limits remain identical. Version 1 has no multiplayer, infinite
streaming worlds, fluid simulation or electrical networks.

## Authoring and agent context

Projects retain `dimension: "3d"` and add immutable `variant: "voxel"`. Older
projects keep an empty variant. Plan schema 5 adds `voxel`; schemas 1–4 remain
supported for existing games. Native `set_design` uses `base: "voxel"` plus a
JSON-encoded `voxel` string. The example returned by `inspect` is the complete
valid starting definition. Source edits use the same validator.

`src/voxel.json` contains `version`, `mode`, `seed`, `size`, `terrain`, `blocks`,
`items`, `recipes`, `enemies` and optional `goals`. `src/main.ts` calls
`startVoxelGame(definition, hooks)` through the small installed `common.ts`.
The template also uses the existing presentation/audio configuration. Optional
hooks are `setup(api)`, `step(api, dtSeconds)`, `action(api, target)` and
`dispose(api)`. Returning false from `action` suppresses built-in mining/attack.

The API exposes `getBlock`, guarded `setBlock`, atomic `transaction(take,give)`,
`count`, `craft`, `damagePlayer`, read-only player/progress snapshots, and the
scene/camera/renderer for decoration. Hooks dispose their own additions when
reloading or closing. They must not introduce a competing input or animation loop.
Feedback does not grant resources or satisfy gameplay checks.

Planning, editing and source generation have stable Voxel prompt profiles.
Build/repair share the four editing tools; planning retains three. The agent
receives the compact contract and definitions, using existing reads/search/asset
operations for additional detail. Chunk arrays and player saves never enter
prompts. Known helpers require full content comparison before compact references
are used. Existing model ceilings, repair limits and usage accounting still apply;
local prefix equality does not establish provider cache savings.

## Saves and revision compatibility

Published Studio games save centrally in AuraGo. The opaque sandbox iframe asks
its trusted parent to load/save/reset the bound project's published revision.
Authentication and a separate parent-only play grant are required; asset preview
tokens do not grant save access. Drafts and automated tests use temporary state.
**Open in new window** uses the same trusted parent bridge.

Changed state is autosaved at most once every five seconds and flushed on pause,
inventory, and deliberate Studio window/project changes. Saving failures remain
visible. A failed close allows retry or explicit discard. A process crash or
abrupt browser termination can lose changes since the last successful save.
When another device has already written a newer version, the write is rejected:
use **Load latest** in inventory before continuing. No automatic overwrite occurs.

World compatibility includes generator/save versions, mode, seed, dimensions,
terrain, block/item identities and enemy identity/count. Compatible revisions
reuse the save; incompatible ones start a separate save and preserve older
progress. Renaming an item or changing its color does not discard progress.
Reset uses a versioned tombstone so a stale device cannot restore deleted data.
Deleting the whole project also removes its private saved states.

The additive SQLite migration creates `game_maker.db.before-voxel-*.bak` before
adding the variant column to an existing database. No published source or runtime
is migrated automatically. Preview/export reads that revision's immutable files.

ZIP exports include definition, source and runtime, excluding private saves and
server test drivers. Serve the extracted ZIP through a local HTTP server, including
from a subdirectory. IndexedDB stores export progress on that browser/origin/path;
it is independent from Studio progress. `file://` remains unsupported.

## Verification and performance acceptance

- `go test ./internal/gamemaker` includes world/transaction/save validation,
  immutable revision isolation and migration on a copied legacy database.
- Set `GAMEMAKER_VOXEL_BROWSER=1` for `TestVoxel*`: normal inputs exercise movement,
  jumping, block removal and inventory pickup, placement, crafting, enemy-driven
  death/respawn, touch cancellation, focus loss, pause and renderer disposal.
  Export checks include subdirectory hosting and IndexedDB reload.
- `TestVoxelValidationBrowser` runs the sandbox driver. The server requires actual
  block, inventory, player or enemy deltas; diagnostic counters alone cannot pass.
  Existing limits remain 16 observations and 60 seconds per validation.
- `TestGameMakerVoxel*` in server and UI checks stable profiles, policy, limits,
  source/channel/revision bindings, transient validation and visible conflicts.
  UI browser checks require `AURAGO_RUN_BROWSER_SMOKE=1`.
- Keep existing 2D/3D, provider, budget and security regression checks. Regenerate
  native-tool training artifacts and package/verify matching web resources.

The performance target is 60 FPS desktop and 30 FPS mobile on documented hardware.
Use the standard 96×48×96 world with 24 active survival enemies, record CPU/GPU,
OS/browser, viewport, quality and steady-state frame times, and repeat during
mining/rebuilding. `TestVoxelSurvivalAndCreativeBrowser` supplies a 24-enemy smoke
scene. Headless software-rendered FPS and touch emulation are functional evidence;
they do not certify desktop GPU or real-device mobile performance. Hardware
performance acceptance remains open until those measurements are recorded.

Implementation references: [Three.js voxel geometry](https://threejs.org/manual/pages/voxel-geometry.html)
and [MDN iframe sandbox](https://developer.mozilla.org/en-US/docs/Web/HTML/Reference/Elements/iframe#sandbox).
