# Game Maker Studio

## Purpose

An agent development environment: edit project files, build, observe a browser,
use optional local frameworks/assets and explicitly finish.

## Ownership

- This package owns projects, drafts, builds, browser sessions, revisions,
  exports, optional templates/runtime libraries and Voxel saves.
- Server owns the continuous model run, image delivery, HTTP permissions and
  events; agent owns schemas/bound dispatch; desktop apps own Studio UI.
- User preference: maximum creative and technical freedom within the project.
  No required design, scenarios, evidence quotas, repair phases, starter checks,
  model size floors or imposed editing modes.

## Local Contracts

- One tool-enabled run owns a draft. Cancellation, exhausted budgets, incomplete
  responses and storage failures preserve the draft and last published revision.
  Continuation restores private conversation checkpoints and draft files.
- File/asset writes save only. Preserve atomic writes, expected hashes, bounded
  text reads, file/project limits and path/symlink/reparse confinement.
  Batch replacements check the whole original snapshot before writing.
- New projects contain framework-free index.html and game.json metadata.
  HTML/CSS/JS run directly. Existing src/main.ts is bundled to dist/game.js by
  esbuild. Builds never rewrite source or require framework/AuraGo hooks.
  Libraries remain local; no CDN/npm infrastructure is introduced.
- Frameworks, templates (`canvas` plus optional Phaser/Three/Voxel starters),
  local library bundles (`add_library`), craft recipes (`reference` `topic`),
  procedural local `sfx` generation (`vendor/zzfx.js` / `.wav`) and asset packs
  are optional. Their helper contracts apply when used, never as publication
  criteria. Preserve existing runtime versions, source, asset licenses and exports.
- Explicit finish builds and requests publication of that exact snapshot.
  Publication rechecks active ownership, permissions, cancellation and public
  source/build hashes under source/build locks. Private conversation changes
  are excluded. Runtime/image observations never veto publication.
- Revision blobs, receipts, rollback, restore, export and saved-game integrity
  remain binding. Old plan/check records are historical, readable data.
- Publication commits the job completion and revision in the same database
  transaction. Failure of either write preserves the prior revision and draft.
- Independent previews use a tokenized loopback snapshot server and
  security.LaunchEgressBrowser with job lifetime overriding its two-minute
  default. The Studio window is not a dependency. No main-server credentials
  enter the browser. Keep proxy, SSRF, CSP and private-file isolation.
- Open/reload build current files. Errors and source maps belong to the loaded
  snapshot's build ID. Diagnostics are untrusted, bounded and host-path redacted.
- Screenshots include the rendered HTML viewport and preserve running state.
  No capture count quota: bound each image to 2 MiB, pending images to 8 MiB,
  and retained UI images to three. Keyboard/mouse/touch/wait actions, inline
  `capture` contact sheets (2–6 frames), `viewport` presets (`landscape`,
  `hd`, `portrait`, `tablet`, `WIDTHxHEIGHT`) and read-only `probe` telemetry
  (`window.__gameDebug`) are voluntary tools.
- Technical learnings extracted without file/condition bindings from a
  `game-maker:<project>` run also share into `game-maker:studio` so craft tips
  can carry across projects within existing memory/learning controls.
- Browser/vision failures are recoverable tool errors. Multimodal coding models
  receive pixels before context fitting; text models use the configured vision
  provider. All model calls retain normal total-budget accounting.
- Completion/cancellation cleans up browser, profile, temporary saves, snapshots
  and images. Published Voxel saves remain separate, authenticated and CAS-versioned.
- Private state never enters previews/exports. Search covers editable project
  text, excluding private paths, managed vendor files and compiled output.

## Work Guidance

- Remove obsolete mandatory orchestration instead of adding another bypass mode.
- Prefer relative compiler positions and freely requested images over inferred
  quality verdicts. UI distinguishes interactive preview from the agent view.
- Keep schemas, the shared prompts/tools_manuals/game_maker.md manual, training
  contracts, UI translations and owners aligned. ToolManualID binds the family once.

## Verification

- TestFreeAgentPublishRestoreExport covers HTML and optional Phaser/Three/Voxel,
  source preservation, explicit finish, restore and immutable export.
- TestFreeAgentDraftAndCompilerFailures covers compiler locations, drafts,
  continuation, conflicts and path confinement.
- TestFreeAgentBrowserPreservesState with AURAGO_RUN_BROWSER_SMOKE=1 exercises
  twelve captures/input rounds without Studio, errors and build isolation.
- TestPreviewBootHTMLBrowser uses the same browser opt-in (legacy
  GAMEMAKER_PREVIEW_BROWSER=1 also works) for framework-free readiness,
  hidden/collapsed layouts, runtime/resource feedback and opaque iframe isolation.
- Retain permission, receipt/publication, revision, source and save regressions.
  Developer tests are never a mandatory acceptance pipeline for generated games.

## Child DOX Index

- asset_packs/AGENTS.md: local content, metadata and reproducible packing.
- export/AGENTS.md: revision blobs, fingerprints, receipts and ZIP export.
- validation/AGENTS.md: build diagnostics and historical check records.
- voxel/AGENTS.md: optional Voxel worlds, transactions and saved progress.
