# Game Maker Studio

Game Maker Studio provides files, builds and browser feedback to a free coding
agent. Describe a game or an edit; choose a configured provider/model and let the
agent work in one continuous run. There is no mandatory plan, template, gameplay
test, repair phase or Game Maker model-size floor.

## Enable and configure

Enable Game Maker in the Virtual Desktop configuration and grant create/edit
permissions. Read-only, deletion and media generation remain separate permissions.
The project ledger is game_maker.db; files stay below the configured workspace as
Games/<slug>. File, asset, project-size and total job budgets still apply.
Provider context/output limits and the configured global tool/cost budgets apply.
Old planning-temperature, small-model and repair-budget fields are compatibility
data; new jobs do not use them to select a workflow.

## Files and publication

New projects start with a small framework-free index.html. Plain HTML/CSS/JS and
local libraries work directly. If src/main.ts exists, esbuild produces dist/game.js.
The build does not edit source. Bundled Phaser, Three.js, Voxel, templates and asset
packs are optional; framework references are fetched only when needed. CDN and
npm infrastructure are not supported.

Writes save the draft. An explicit build, preview reload or finish processes the
current files. `add_library` copies bundled `phaser`, `three`, `voxel`,
`effects-2d`, `effects-3d` or `zzfx` helpers into `vendor/`. Identical existing
files are reused. A conflicting file stops the entire installation and the error
names the path. `use_template` installs optional starters including a
framework-free HTML5 `canvas` starter; `reference` also supplies optional craft
recipes (`juice`, `particles`, `procedural-audio`, `procgen`, `pathfinding`, `save`,
`tweening`, `ui-hud`); and `game_maker_asset` `generate` supports local
procedural sound effects (`kind: "sfx"`, `.wav` + ZzFX) without external media
permissions. The agent's finish operation ends the job; a successful current
build publishes one consistent source/build revision. Runtime errors and pictures
are observations, never publication gates. Cancelled, exhausted, incomplete or
failed runs publish nothing. Try again resumes the saved draft and conversation.
Workspace adoption builds and publishes saved edits without calling a model.
Restore and ZIP export retain source/build integrity checks and historical revisions.
Technical learnings, when enabled, stay with the game project that produced them.

## Preview and images

The agent has its own Chromium session, independent of the Studio window. Install
Chrome or Chromium on the backend host; no additional browser service is needed.
open/reload build and load files; logs returns exceptions, rejected promises,
console errors and failed resources with the build identity and source positions
when source maps exist. Browser failure is a tool error that can be retried.
The Studio repair button sends a fixed request with error details attached as diagnostics.

Screenshots show the rendered viewport, including HTML controls. They neither
reload nor reset the game. The agent can request as many as useful within the job's
overall budget; size bounds and cleanup limit memory use. Keyboard/mouse/touch/wait
actions, inline `capture` contact sheets (up to 6 frames per `input` batch),
optional `viewport` presets (`landscape`, `hd`, `portrait`, `tablet`, `WIDTHxHEIGHT`)
and read-only `probe` inspection (`window.__gameDebug` + HUD/canvas info) allow
free exploration without prescribed scenarios or success counters.
Image-capable coding models receive real image messages. Text-only models receive
descriptions from the configured Vision model. Missing Vision is a concrete tool
error, not a new judging or repair phase.

The Studio keeps an interactive preview for your own play and labels independent
browser captures **Agent view** (**Agent-Ansicht** in German). It shows activity,
compiler/browser errors, recent agent images, usage and the published revision.
The browser, inputs, temporary save state and image artifacts are removed at job
completion or cancellation. Published saves remain separate.

## Existing games and permissions

Existing Phaser, Three.js and Voxel code, revisions, exports and saves remain
available. Historical plans and check results stay readable but do not govern new
jobs. game_maker_validate is absent from the current tool catalog; its legacy
dispatch returns explicitly labelled build diagnostics only.

Paths, symlinks, atomic writes, stale-write hashes, browser isolation and secret
boundaries remain enforced. Game code runs inside the project boundary and cannot
access the host application APIs or saved secrets. There is no automatic gameplay
certification: published games may still contain bugs. The agent decides how much
preview feedback is useful before finishing.

## Verification

Focused regressions cover plain HTML and existing framework build/publish/restore/
export, failure preservation, stale writes and browser isolation. Set
AURAGO_RUN_BROWSER_SMOKE=1 for the independent Chromium and model-image transport
fixtures. These development tests are never a required workflow for generated games.
