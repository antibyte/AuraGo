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

The selected model plans, builds, validates, and polishes autonomously. Planning
is internal and requires no additional user confirmation. The server validates
the structured plan before allowing code/media/import mutations. The initial
plan has at most two corrections. Verified phase guidance is supplied directly;
the model does not have to discover/activate four skills first. Progress events,
diagnostics, and the preview remain in the same window. Model prose is held until
publication, so an agent's early success claim cannot precede the actual checks.
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
The sprite helper creates complete assemblies. Read each object's direction:
most side-view vehicles face right, the ambulance faces left. Top-down robots
provide four directions; rotatable vehicles record their exact forward angle.

Search/filter packs, inspect sprites, play animations, and switch between
checkerboard, white and dark backgrounds. **Use for next job** prepares the next
creation/change request without starting it. The form shows the selection;
starting a job consumes it. Selections belong to the current Studio window.
An empty selection lets the agent choose.

Selected packs import before the agent starts. Additional packs are available
through `game_maker_asset` operations `list_packs`, `describe_pack`, `import_pack`,
`search_assets` and `describe_asset`, always with the active `job_id`.
Search accepts query, optional pack_id/view (side/top/board), and limit (default
six, maximum twelve); complete assemblies rank before fragments. Asset IDs,
names and tags outrank pack-name matches; a search within a selected pack matches
its contents, not the pack name. Describe accepts
pack_id and exactly one asset_id/assembly_id. It returns entity variants, available
directions/actions, explicitly missing actions and a use example. Imports require
an accepted plan; search and description are available during planning.
Omitting `operation` retains custom generation. Built-in imports need Studio
edit permission but no image provider or media-generation permission.
The import response and selected-pack context include a `phaser_example` with
the actual paths, spritesheet loader and an asset selected by ID. The PNG needs
`load.spritesheet` with 64×64 cells; `load.image` displays the whole sheet and
the metadata is not Phaser atlas JSON. Built games reject this incorrect loader
usage with a diagnostic so it reaches the agent's repair loop.

Each import publishes PNG/JSON together at `assets/builtin/<pack-id>/<version>/`.
Script writes check literal built-in metadata imports against complete project
copies before replacing source. Invalid pack IDs, versions or relative paths
return `asset_import_invalid` with usable imports; source, preview and repair
counts remain unchanged. Imports resolve from the source file's directory:
`src/main.ts` uses `../assets/...`, while runtime image URLs use `assets/...`.
Search/describe alone does not import a pack. Existing project versions remain valid.
Identical pairs are reused; incomplete/modified copies are not overwritten.
File, asset, file-count and project limits apply. Metadata includes stable IDs,
numeric frames, origins, directions, ordered animations, FPS, repeat/yoyo,
source hashes and the MIT notice. Project copies survive revisions and export.
The embedded Phaser skill includes a loading example using bundled JSON imports;
generated games use local files, never the catalog API. Production is documented
in `internal/gamemaker/asset_packs/production/README.md`.

Pack version 2 adds explicit entity/action groups and transformation rules without
changing artwork. Old project copies stay intact. The versioned embedded module
`vendor/aurago-game-1.js` loads exact spritesheets, registers ordered animations
idempotently (including hold frames), creates assets/assemblies, selects facing,
and changes actions without restarting the same animation every frame. Transform
angles are radians: zero right, pi/2 down. Four-view figures choose a directional
animation and retain their last idle direction; flips and rotation require explicit
metadata permission. Buildings, terrain, cards and signs have no blanket flip rule.
Assemblies transform in one container; physics uses a separate proxy body because
child Arcade bodies do not follow parent transforms correctly. See the complete
scene in the embedded Phaser skill and the six editable `templates/*.ts` examples.

## Internal plan and templates

`game_maker_project inspect` returns next_action and a complete plan_example.
The provider-compatible tool schema advertises `plan` as a JSON object string;
native calls also accept the object directly. Both formats and XML fallback calls
preserve the complete plan before validation. Field-specific errors survive a new
planning round and appear in the final failure if corrections are exhausted.
Unknown JSON fields are rejected before they can be discarded, within the same
correction budget. Each visual role uses one `pack_id` and one `asset_id` or
`assembly_id`; variants use separate uniquely named roles. Independent asset
errors are reported together. Catalog `view` is read-only: perspective corrections
must change the root `plan.perspective`. For example, the blocks-and-balls pack
uses `top`, including its Breakout paddles, balls and blocks.
Acceptance or exhausted corrections ends the agent round immediately through a
server-owned completion check. The model does not need to produce a final sentence
or a done marker to trigger the next phase. Remaining calls in the same native
batch receive skipped results without execution; tool limits remain unchanged.
`get_plan` reads `.aurago/game-plan.json`; `set_plan` validates and writes it during
planning. Plans include goal/loop/scope, template, perspective, resolution/camera,
controls/states/rules, exact asset/version/animation/assembly references, visual
size/origin/collider, scenarios, assumptions/fallback and preserved edit behavior.
Unknown references, incompatible perspectives/actions and unbounded scenarios
produce a field-specific correction. Templates are installed only for new 2D
projects: shooter, platformer, topdown, blocks, board, minimal. Default logical
resolution is 960×540 with FIT, pixelArt and uniform sprite scaling. Existing
projects keep their code and get an updated internal plan instead.

The helper's `bindGameTest(scene,state,player)` connects the current live scene,
object and numeric state to the finite preview driver. The editable template
common lifecycle resets state, physics and inputs. Keep standard Arrow/Space/R
controls for minimum checks; ESC ends/forfeits a run. State counters reflect actual
actions, hits, points, spawns, turns, timer ticks and terminal state. Do not invent
actions or fabricate test counters for absent behavior.
Planning examples distinguish position changes (`player_x`, `player_y`) from
primary actions (`actions`). Breakout uses the `blocks` template; `minimal` is
reserved for games without a matching starting template.
New templates embed exact library-role imports and preloading in `common.ts`.
The blocks template uses `body(..., role)` for its player, ball and block variants;
the helper fits opaque sprite bounds uniformly and follows a separate collision
proxy. Extend this wiring instead of rewriting the library loader. Custom preload
overrides must call `super.preload()`. The startup guard rejects pack JSON passed
directly to Phaser as a texture key and explains the correct `createAsset` call.
Additional collision tests need enough time for travel from launch to target;
hit counters count actual contacts, including contacts with durable blocks.
`setup()` must assign `this.player` to the controlled object (for Breakout,
`this.player = this.paddle`) before returning. A missing or foreign player is
rejected immediately with a concrete binding diagnostic. Asset detail examples
include complete preload/setup methods: importing `preloadPack` alone does not
load a texture. Sprite and assembly creation reject unloaded textures or missing
frames before Phaser can substitute placeholder art.

## Builds, revisions, and export

Each job works in its own staging copy with a server-bound job context for
Studio tool dispatch. An omitted `job_id`
uses that binding; an explicitly different job is rejected. A file call with
complete `content` and no operation means `write` only in an isolated Studio run.
Outside Studio, callers must still supply job ID and operation. Rejected writes
leave previous source unchanged and must be corrected before validation.
TypeScript and ES modules are compiled
with the Pure-Go esbuild API, so Game Maker itself needs neither Docker nor a
Node runtime. Successful validation atomically replaces the published project
and records a revision whose file data is deduplicated in a SHA-256 blob store.

Keep the project's Studio preview open during validation. With omitted scope or
`scope: startup`, `game_maker_validate` waits up to 12 seconds for a canvas
inside the visible viewport and at least three seconds of startup without
runtime/resource errors. Engine console errors are included. Game-authored
readiness alone does not prove a visible canvas. Those errors
are returned to the agent and the existing repair loop (at most three passes).
Missing browser feedback blocks publication instead of claiming playability.
This compatibility mode is a startup check, not a full gameplay check.
`scope: gameplay` or `full` additionally runs immutable template scenarios plus
1–8 plan scenarios. The complete run is limited to 60 seconds. Commands are bounded
key/pointer/wait/observe operations, never JavaScript expressions. Server-side
comparisons check input, primary action, rules, timers, a six-second late-event
interval, terminal state, sprite integrity and two consecutive restarts. Missing
measurements are unavailable, not passed. A new build invalidates prior evidence
and replaces preview diagnostics. The driver resets the game after testing.

All 2D publication uses full validation; 3D publication currently uses startup
and explicitly reports gameplay unverified. Technical failures share at most
three repair passes across tool calls and orchestration, with no nested allowance.
Repairs receive the accepted plan and exact check/expected/observed mismatch.
Exhausted repair budgets or unavailable browser feedback end the agent round
immediately; each repair round ends after its first validation. Building may
continue after core checks while budget remains, to finish the planned features.
The server then continues the bounded workflow,
without another model request in the completed round. On budget exhaustion the
last concrete validation failure remains the job error, including its checks;
it is not replaced by the generic repair-limit message.
At most two optional screenshots go to the same selected model only when its
catalog metadata confirms image input. A tool-free, budgeted request supplies
advisory observations; no provider switch occurs. Missing capture/capability or
review failure is skipped. Visual review cannot override a failed technical check.
Publication rechecks cancellation, edit permissions, build identity and late
runtime errors under the publication lock, including errors during image review.
The internal `.aurago/validation-report.json` records checks, separate gameplay/
visual status and the compiled bundle SHA-256. Plans and reports travel with
revisions and are excluded from export. Later preview errors accompany the next
change request as untrusted diagnostics.

For Phaser errors such as `body.setVelocity is not a function` or
`body.setPosition is not a function`, validation includes targeted repair guidance:
moving players/paddles need dynamic Arcade bodies, while position changes use the
game object or the body's `reset` method. The initial attempt and three repair
passes remain the limit; suppressing errors does not count as a repair. Expired
or finished validation previews stop submitting reports, so a late token rejection
does not appear as another game error. Published-game errors remain visible.

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
most eighteen IDs, duplicates removed). No database migration is needed.

The authenticated `preview-report` route additionally accepts at most sixteen
numeric observations and two PNG data URLs (700,000 characters each). The report
is bound to its build and the first ready preview token. Other JSON routes retain
their 256 KiB limit; preview reports are limited to 1,500,000 bytes.

## Acceptance commands

Run `go test ./internal/gamemaker`, focused Game Maker agent/server/UI tests,
`node scripts/test-game-maker-sprites.mjs`, and
`node scripts/build-ui-bundles.js --check`. Reproduce metadata with
`python scripts/pack_game_sprites.py --check`.
For real Phaser acceptance set `GAMEMAKER_BROWSER_TEST=1` and run
`go test ./internal/gamemaker -run TestGameMakerBrowserFixtures -v -timeout 15m`.
Open its loopback URL and run the series: six templates, documented sprite and
multiball scenes, all packs, and deliberate input/restart/late-event/asset/physics
failures. This opt-in test never calls an LLM.
Every case is loaded from the actual ZIP export with the production preview CSP,
which blocks external network resources. Set `GAMEMAKER_BROWSER_EXPORTS_ONLY=1`
to run only the positive exports. The diagnostic/test bridge is injected by
the fixture server; exported files themselves contain neither that bridge nor
internal plans/reports. The series also runs the exact documented multiball
example and rejects collider arguments containing `{body,art}` wrapper records
instead of physics GameObjects. Groups retain collision coverage for later spawns;
fixing that wiring must preserve passing sprite/input behavior and pack imports.

`game-maker-comparison.json` defines eight fixed weak-model cases and the recorded
fields. Run both commits with the same configured provider/model and settings,
then record human playability observations independently of model success prose.
This provider-dependent comparison is separate from the deterministic browser
suite; do not claim a model quality improvement without those measurements.
