# Voxel games in Game Maker Studio

Choose **Voxel** to express the project preference. New drafts start as plain HTML;
the agent can install the optional Voxel template with `use_template` or write a
custom game. The controls and limits below describe the bundled Three.js Voxel
template, whose survival mode is the default. Its mode stays fixed during play.
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

The optional `use_template` operation with `template: "voxel"` installs the local
runtime and starter. `game_maker_project/reference` with `framework: "voxel"`
returns its API on demand. Publication records the installed template's dimension
and variant. Old plan schemas remain readable but do not control new jobs.

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

The same four tools remain available throughout the job. Source writes only save;
build, preview reload and `finish` compile the current sources without rewriting
them. No gameplay counters, target scenarios or repair budgets gate publication.
The agent may use its independent browser for logs, input and screenshots.
Published saves never enter that temporary browser or model context.

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
If publication made the running revision's grant stale, **Load latest** refreshes
the trusted frame and grant; same-revision save conflicts reload in place.
Studio flushes progress before a revision restore. External revision changes
also require resolving any unsaved progress before replacing the preview.

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
- `TestGameMakerVoxel*` in server and UI checks policy, source/channel/revision bindings and visible save conflicts.
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
