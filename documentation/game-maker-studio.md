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

## Builds, revisions, and export

Each job works in its own staging copy. TypeScript and ES modules are compiled
with the Pure-Go esbuild API, so Game Maker itself needs neither Docker nor a
Node runtime. Successful validation atomically replaces the published project
and records a revision whose file data is deduplicated in a SHA-256 blob store.

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
