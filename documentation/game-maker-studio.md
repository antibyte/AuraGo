# Game Maker Studio

Game Maker Studio is AuraGo's built-in Virtual Desktop workspace for creating
self-contained 2D and 3D browser games with an isolated agent. Version 1 targets
offline, single-player games. Multiplayer, custom backends, external APIs, and
deployment are intentionally outside its scope.

## Enable the feature

Game Maker Studio is disabled and read-only by default. An administrator must
explicitly enable the needed capabilities:

```yaml
game_maker:
  enabled: true
  readonly: false
  allow_create: true
  allow_edit: true
  allow_delete: false
  allow_media_generation: true
  workspace_path: ./agent_workspace/workdir
  max_projects: 25
  max_files_per_project: 250
  max_file_size_kb: 2048
  max_asset_size_mb: 32
  max_project_size_mb: 100
  job_timeout_seconds: 1800
```

The ledger defaults to `./data/game_maker.db`. Project files are addressed only
as `Games/<slug>` below `workspace_path`; APIs never return the resolved host
path. Deletion is separately gated because it removes the project directory,
while shared AuraGo Media Registry files remain protected.
`max_file_size_kb` limits source and configuration files; generated images and
music use the separate `max_asset_size_mb` limit. The total project limit still
applies to both.

## Creating and refining a game

Open **Game Maker Studio** from the Virtual Desktop and select **New game**.
Choose 2D or 3D, describe the game, select a configured provider/model, and
optionally enable image and music generation. The global AuraGo provider and
model are preselected.

- 2D projects use the embedded Phaser 4.2.1 runtime.
- 3D projects use the embedded Three.js 0.185.1 runtime.
- Images can become sprites, backgrounds, textures, or UI art.
- Music can become a local background track.
- Sound effects fall back to procedural Web Audio.
- AuraGo does not claim to generate 3D models.

The agent plans, builds, validates, and polishes autonomously. Progress,
responses, diagnostics, and the playable preview remain in the same window.
After a validated revision is ready, enter a change request to create the next
revision. Stop cancels the staging job without changing the last playable
version.

## Studio workflow

- The creation dialog offers localized idea chips and a short description
  guide next to name, dimension, provider, and media options.
- While a job runs, a banner shows the current phase, the elapsed time, and
  the repair pass when the builder loops back from validation. The phase
  stepper below it mirrors the same state.
- Finished jobs end with a result card in the conversation: a playable
  revision offers **Play now**, a failure shows the error and a **Try again**
  button that restores the last prompt for editing.
- The preview toolbar reloads the game, toggles fullscreen, or opens the
  current revision in a new browser tab with a fresh preview token. While a
  newer build is running, the preview carries an "updating" badge.
- Only one job runs at a time. The library marks the busy project with a
  spinner, and the change form of other projects stays disabled with a hint
  until the active job finishes (the capabilities API exposes the active
  job's project ID, status, and phase).
- On narrow windows the secondary project actions move into an overflow
  menu; on phone-sized screens the library sidebar collapses into a project
  dropdown inside the agent pane.
- The skills dialog summarizes status in plain language and folds source,
  commit, and license details into a collapsible section.

## Offline sprite library

Open **Assets** to browse eighteen original pixel-art packs. Each contains 100 cells
of 64×64 pixels in a 640×640 RGBA PNG, with English asset descriptions and JSON
animations. Categories cover space shooters, animated effects, platformers,
top-down adventures, blocks/balls, cards/board games, side-view and top-down
humans, monsters/animals, buildings/structures, vehicles/planes, nature and
animated robots/drones. Animation frames count toward the 1,800 cells; some
sequences deliberately hold a source pose.

Buildings and large vehicles contain 48 assembly recipes across four packs.
The **Assembly** selector previews complete objects, including synchronized
vehicle animations. JSON `assemblies` describe dimensions, origin and ordered
parts with exact frame indices and pixel offsets. Parts can reach their cell
edges; assemble them in one container without individually resizing them.
The import example includes this container pattern. Side-view vehicles face
right; overhead vehicles face up. Top-down robots provide four directions.

Search/filter packs, inspect sprites, play animations, and switch between
checkerboard, white and dark backgrounds. **Use for next job** prepares the next
creation/change request without starting it. The form shows the selection;
starting a job consumes it. Selections belong to the current Studio window.
An empty selection lets the agent choose.

Selected packs import before the agent starts. Additional packs are available
through `game_maker_asset` operations `list_packs`, `describe_pack`, and
`import_pack`, with the active `job_id`; the latter two also need `pack_id`.
Omitting `operation` retains custom generation. Built-in imports need Studio
edit permission but no image provider or media-generation permission.
The import response and selected-pack context include a `phaser_example` with
the actual paths, spritesheet loader and an asset selected by ID. The PNG needs
`load.spritesheet` with 64×64 cells; `load.image` displays the whole sheet and
the metadata is not Phaser atlas JSON. Built games reject this incorrect loader
usage with a diagnostic so it reaches the agent's repair loop.

Each import publishes PNG/JSON together at `assets/builtin/<pack-id>/<version>/`.
Identical pairs are reused; incomplete/modified copies are not overwritten.
File, asset, file-count and project limits apply. Metadata includes stable IDs,
numeric frames, origins, directions, ordered animations, FPS, repeat/yoyo,
source hashes and the MIT notice. Project copies survive revisions and export.
The embedded Phaser skill includes a loading example using bundled JSON imports;
generated games use local files, never the catalog API. Production is documented
in `internal/gamemaker/asset_packs/production/README.md`.

## Builds, revisions, and export

Each job works in its own staging copy. TypeScript and ES modules are compiled
with the Pure-Go esbuild API, so Game Maker itself needs neither Docker nor a
Node runtime. Successful validation atomically replaces the published project
and records a revision whose file data is deduplicated in a SHA-256 blob store.

Keep the project's Studio preview open during validation. After compiling,
`game_maker_validate` waits up to 12 seconds for the browser to report a canvas
inside the visible viewport and at least three seconds of startup without
runtime/resource errors. Engine console errors are included. Game-authored
readiness alone does not prove a visible canvas. Those errors
are returned to the agent and the existing repair loop (at most three passes).
Missing browser feedback blocks publication instead of claiming playability.
This is a startup smoke check; controls and later gameplay still need testing.
Errors observed later in the current preview accompany the next change request.

Restoring an older revision creates a new revision and keeps the complete
history. ZIP export contains:

- `game.json`, `src/`, and other source files;
- the compiled `dist/` output;
- local runtime files under `vendor/`;
- project assets;
- `THIRD_PARTY_NOTICES.md`.

Staging files, preview tokens, ledger data, revision metadata, and AuraGo
secrets are excluded.

## Security model

Game Maker jobs receive only six callable tools: the four project-specific
Game Maker tools plus Agent Skill listing and activation. Generic filesystem,
shell, Python, network, Desktop, Homepage, and `invoke_tool` access is excluded.
The binding Agent Skill scope contains exactly the five embedded
`aurago-game-*` packages. These skills are system-managed: startup restores any
locally changed or missing `SKILL.md` to the embedded version (self-healing),
after which the package is rescanned. Packages that scan with a warning or an
error, or whose post-install hash no longer matches the verified registry
entry, still block new jobs.

Preview documents use a short-lived token bound to one project and optionally
one active staging job. The iframe permits scripts but deliberately has no
same-origin privilege. A restrictive content security policy blocks external
connections, objects, forms, and AuraGo API access. Runtime diagnostics travel
through a bounded `postMessage` channel validated by iframe window and random
channel ID.

## Curated skill sources

- Three.js material is adapted from
  `majidmanzarpour/threejs-game-skills` at commit
  `7221c1f4a6d2ae189a4d85d058d24f3228499d46` (MIT).
- Phaser material is adapted from `phaserjs/phaser` at commit
  `41be1e462bc600064e498cba370bfa8c5c055a22` (MIT).
- TinySwords commit `f59f1dca8bf461227c9b5d856764e1e90d8b8e90`
  declares no license. AuraGo therefore uses only clean-room general concepts
  such as scene-first structure, deterministic checks, and canvas diagnostics;
  it copies no TinySwords text, scripts, or assets.

## API

Authenticated Virtual Desktop clients use `/api/game-maker/capabilities`,
`/projects`, project jobs/events/revisions/restore/preview-token/export, and
`/jobs/{id}/cancel`. SSE event IDs are monotonic and support reconnecting with
`Last-Event-ID`.

Authenticated sprite endpoints are `GET /api/game-maker/asset-packs`,
`GET /api/game-maker/asset-packs/{id}/sheet.json`, and the corresponding
`sheet.png`. Only known IDs and these filenames are served. Disabled Studio
access is rejected. Start-job bodies optionally accept `asset_pack_ids` (at
most ten IDs, duplicates removed). No database migration is needed.
