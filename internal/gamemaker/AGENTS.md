# Game Maker

## Purpose

Own offline game planning, source editing, asset import, browser validation,
revision publication and standalone export for Phaser and Three.js games.

## Ownership

- `guided_design.go`, `skills.go`, `skills/`: compact agent contracts and examples.
- `templates/`: new-project source bases; preserve authored existing source on edits.
- `runtime/game-flow.js`: player feedback and result/stage UI, driven by engine clocks.
- `runtime/player-ui.js`: initial Start/help, pause menu and device-specific input UI.
- `runtime/aurago-game-1.js`: Phaser input/assets and read-only test binding.
- `voxel*.go`, `runtime/voxel-*.js`, `runtime/aurago-voxel-1.js` and
  `templates/voxel-common.ts`: validated Voxel definitions, finite world/physics,
  transactions, evidence and isolated saves. `runtime/preview-voxel-tests.js`
  is the server-only normal-input driver; never export it.
- `assets/game-maker-presentation/` at repository root: editable effects/audio source;
  its build script produces the two bundled effects runtimes here.
- `runtime.go`: project-local runtime installation, also used by exports.
- `runtime_context.go`, `source_search.go`, `source_batch.go`: installed helper
  descriptors and bounded source discovery/edits for the isolated agent.
- `three_observation.go`, `three_pickup_observation.go`: build-only compatibility
  observations for verified legacy helpers; never rewrite installed source.
- `starter_reference.go`: plan-bound exact helper verification and compact
  source-generation references. Server prompt profiles and usage events remain
  bound by this contract across the agent and LLM transports.
- `events.go` accepts the private `prompt_usage` measurement event; the Studio's
  `diagnostic` event carries its compact summary with explicit unknown values.
- `source_diagnostics.go`, `validation_reuse.go`: private source mapping and
  exact-build validation reuse; explicit tool validation always runs afresh.

## Local Contracts

- Voxel definitions, finite worlds, transactions, evidence and isolated saves:
  `voxel/AGENTS.md`.
- Validation plan disclosure, repairable checks, tool validation evidence and
  the browser evidence boundary: `validation/AGENTS.md`.
- Revision blobs, publication fingerprints, filesystem receipts and ZIP export:
  `export/AGENTS.md`.

### Shared game lifecycle

- Terminal job states become observable only after working-copy cleanup, with the
  writer released under the same lock. Immediate retries and revision restores
  must not race a job that has already reported completion.
- Complete experiences include consequences, recovery or results, a suitable world
  and meaningful progression. Single-screen boards and peaceful sandboxes remain
  valid; do not require combat, lives, timers or linear levels for every idea.
- Phase guidance stays within 2500 characters and uses existing design fields.
  Keep its prefix deterministic; put detailed examples in curated skills.
- Prepared prompts are phase- and engine-pure and structured: ground rules, a
  numbered phase workflow, then `## ` sections of short bullets (no line over 600
  characters). Planning never carries edit mechanics or source-writing contracts
  (sprite contract, model API, presentation wiring); 2D never carries Three.js
  integration and 3D never Phaser. Choosing presentation IDs belongs to
  `PresentationPlanningGuide`; wiring belongs to `PresentationGuide2D/3D`.
  Voxel planning uses `VoxelPlanningGuide` (with a validated goal example) plus
  `PresentationPlanningGuide`, editing `VoxelRuntimeGuide`.
  3D planning inserts `## Guided 3D base` immediately after the planning workflow
  and before phase guidance. A new game whose loop fits uses fps, exploration,
  transport, flight or space. An endless runner, obstacle course or delivery route
  uses exploration or transport when that loop fits. A grid game such as snake stays
  on base minimal. Base three stays for a custom or procedural engine,
  and an existing base stays. Free three still has startup checks only and gameplay
  stays unverified. The same choice is the `design.base` description on the 3D
  planning schema only. 2D, voxel, building and repair omit that section. Building
  and repair stay byte-identical. Phase guidance length is unchanged.
- Planning a new game (phase planning, base revision 0) has only a scaffold. The
  planning workflow says so, and every file read/search answer carries a
  next_action to submit set_design instead of exploring. In bound runs a
  `game_maker_file` call with a line range and no content is a read.
- Feedback never changes gameplay counters, health or outcomes. Real contacts and
  game rules own those changes. Missing observations remain unverified.
- Non-scene Three.js helpers count real item/cargo/flight-goal collections in
  `pickup_events` and clear that counter on reset. Keep combat hits and cosmetic
  feedback separate; scene-builder pickup counters retain their own ownership.
  Build compatibility instruments real collection branches/reset for the two
  exact legacy helpers from `0f5d4b125` and `6040144e3`, verified against the whole
  template and accepted-plan asset bindings (only CRLF/LF is normalized).
  Runtime context distinguishes this instrumentation from unsupported/custom
  constant-zero helpers needing source repair. Keep source, gameplay rules,
  authored helpers and validation/publication gates intact; retries rebuild the
  preserved draft without extra model calls or a source migration.
- New common templates opt into baseline visual/audio feedback. Explicit imported
  sound bindings take precedence; one mixer, gesture gate and 32-voice budget apply
  to fallback cues too. Mute/volume survive stage changes within the same game root.
- Game flow owns no loop or timers. Pause/inactive state freezes it. Scene shutdown
  or renderer disposal removes feedback, UI and audio resources. Ending play still
  renders the result UI; authored stages rebuild through the engine lifecycle.
- New starters show objective/instructions once before an explicit Start (button
  or Enter); freeze simulation, timers, damage and gameplay input until then.
  Restarts/stage changes within the same game root retain that start state.
  Running HUDs contain compact live status only. Help/restart live in pause/result
  menus. Desktop never receives movement/action button rows, including narrow
  windows. Touch uses a left movement stick and relevant right actions; boards
  use direct cell input, platformers horizontal movement/jump, FPS separate
  drag-look/fire/reload, flight/space altitude controls. Release held pointers on
  cancel, lost capture, pause, blur, inactivity and disposal; retain simultaneous
  movement/action. Runtime labels cover all 16 locales. The finite preview driver
  starts through public Enter input; it never bypasses this lifecycle gate.
  `playerUIOptions` (Phaser) / `config.playerUI` (Three.js) configure the installed
  helper's objective, instructions, movement and primary action. Preserve custom
  and published games; new helper versions are not an automatic source migration.
- Phaser life-based authored rules use `damagePlayer()` and checkpoints; Three.js
  authored rules use `api.damagePlayer(amount)`. Scene-builder health/contact rules
  remain authoritative for scene-driven games. Do not double-apply damage.
- Level contents are authored source or scene data. Helpers select actual levels;
  never claim progression by cloning an empty map or incrementing a label.
- Restart selects the initially active level, not the first serialized entry;
  scene serialization sorts IDs. Preserve this in both 2D and 3D helpers.
- Held FPS/space fire follows the same action hook and scene mechanic routing as
  a single press, with the firing cooldown. Board pointer actions retain the
  start, pause, preview activity and end gates before changing selection or state.
- Scene projectiles resolve swept contacts in travel order and stop on their first
  hit unless piercing. Remove ended dynamic records and release their IDs; static
  objects remain available for reset. Runtime projectile IDs stay unique across
  expiration. Patrol steps cannot overshoot their next waypoint.
- `design.stages` (0–6, `stages.go`) are commitments, not prose. With at least two
  stages, `required_stages` observes `stage_count`: the 3D helper (three-4+) counts
  distinct layouts (objects, goal, bounds, mode, level_id scene nodes); the 2D
  driver counts configured levels and the server additionally requires source that
  reads `levelIndex` or scene nodes carrying `level_id`. Renamed or cloned layouts
  never count. Waves inside one arena are features, not stages.
- Controls (`controls.go`): `required_controls` presses RIGHT (2D) or D (guided 3D,
  helper three-4+) and fails only when the player moves toward screen-left along
  the screen-right ground axis reported at the start (`view_right_x/y`), so
  rotation, vertical-only and pointer controls stay valid. Guided 3D movement is
  camera-relative: FPS follows aim; follow cameras snap to the dominant world axis
  so angled views keep axis-aligned travel. The driver steers with the helper's
  declared `controls` basis and keeps the fixed legacy mapping for older helpers.
  Server checks are inserted before plan scenarios and only while fewer than 16
  checks exist; older helpers never receive a check they cannot observe.
- Starters demonstrate a complete escalating game: platformer, topdown, shooter,
  blocks and board ship three stages (layouts chosen by `levelIndex`), two enemy
  behaviors and a power-up or smarter opponent; role-based guided 3D starters get
  `config.levels` whose first stage keeps the objective along the starting heading
  and whose later stages scatter it with guards, cover and clustered scenery.
  `minimal` stays a blank slate. Keep each 2D starter's first stage gentle: the
  first item/goal/enemy near the spawn and hazards away from it, so required checks
  and idle waits observe normal play. Driver regressions patch exact geometry in
  frozen copies under `testdata/driver-fixtures/`, never the live starters.
- `craft.go` guides are part of the prepared profiles. Curated `SKILL.md` text is
  registered but not injected into Studio runs: design direction that must reach
  the model belongs in these guides or phase guidance, never only in a skill.
  Keep target-repair guidance in the shared editing profile so building and
  repair stay byte-identical. Peaceful requests may omit opposition and finales;
  Voxel planning context uses schema 5 and its own definition/check contract.
- Phaser HUD roots use screen-fixed coordinates and explicit `auragoHUD` data.
  World depth may equal world Y; crossing depth 1000 must never hide actors.
  Asset fitting preserves aspect ratio without resizing authored colliders.
  Keep collision footprints separate from artwork and transparent padding.
- Preview startup judges canvas layout only in an active, sized viewport after
  continuous layout settling. Hidden Studio windows/tabs are not game errors.
  Genuine invisible canvases still fail with measured layout diagnostics.
- Movement-only target scenarios measure positions, not primary-action counts.
  Explain mismatched legacy checks without marking them passed or fabricating
  counters. A displacement check alone never proves steering smoothness.
- `shooter_scenarios.go` adapts only non-scene 2D shooter `hits increased`
  scenarios consisting of Space/wait/observe to live `enemy` aiming at execution.
  Preserve ID, metric, comparison, initial waits, total duration and accepted
  plan/source. Target input lasts at most 4000 ms; remaining time stays a wait.
  Explicit targets, navigation/pointer input, other metrics and scene compositions
  keep their declared steps. Reports disclose adaptation and executed steps.
  Target effects and real hits remain mandatory; no extra calls, checks or retries.
- Visual review stays advisory and uses the selected multimodal route directly.
  Request JSON mode only for supported providers; tolerate normal JSON wrappers
  and allow at most one format correction within the original review deadline.
  A passing build after three failed versions has used the shared repair budget;
  retain visual findings without starting another repair in that case.

### Game Maker Studio Contract
- An explicit job provider or model pins planning, editing, source generation
  and repair to a dedicated client after resolving both values. A model-only
  selection uses the configured primary endpoint, never the global failover
  client that replaces the request model. Disable global fallback in that job's
  config snapshot; leave system defaults untouched. Verify the actual HTTP
  model/endpoint with `TestGameMakerSelectedModelReachesProvider`.
- Planning, building, repair and source generation pin sampling per phase
  (`game_maker.planning_temperature` default 0.6, `game_maker.building_temperature`
  default 0.2, both clamped 0.05–1.5); the tool-enabled loop and the tool-free
  minimal loops use the same values. Prepared profiles are `game-maker/v4/...`;
  small-model jobs use `game-maker/v4/<planning|building>/<2d|3d>/compact`.
  Voxel keeps the published v2 text (a small voxel job only takes the v3 name). Any
  prompt text change bumps the version; never edit a published profile in place.
- New Studio jobs resolve the selected provider/model limits before starting. A known
  model below 16384 usable context tokens is refused with `model_context_too_small`
  unless `StartJobRequest.AllowSmallModel` is set; unknown models keep the
  conservative defaults and are never refused. `GET /api/game-maker/model-limits`
  reports the resolved limits passively, without a provider probe, plus
  `small_model`, `small_context_window` (65536) and `small_output_tokens`
  (16384). `small_model` is true only for a known route below either threshold.
  Unknown models are not small and are never blocked. The Studio offers a compact
  checkbox for an unknown route; checking it sends `small_model: true`. `ActiveJobInfo` includes
  `started_at` once the job has started.
- Planning, editing, source-only generation and image review have versioned
  prepared prompt profiles. Editing is shared by building and repair. Select
  2D/3D guidance once; keep the three planning/four editing schemas and their
  order fixed. Job identity, phase, intent, plan, revisions and failures are
  untrusted context after the prefix. Tool discovery uses the existing bounded
  file/asset operations; no dynamic schema selection or cache warm-up.
- `prompt_usage` events contain per-attempt provider/model, nullable measured
  input/output/cache-read/cache-write tokens, duration, profile/schema/prefix
  fingerprints, context generation/reason and local prompt reuse. Include the
  final request's `requested_max_tokens` on success and failure, separately from
  measured provider usage; a requested ceiling is not a usage measurement. They supplement
  legacy `token_usage`; never sum the two streams. Evaluation reports keep
  unknown/partially priced calls explicit and compare costs with calls, repairs
  and outcome. Do not add prompt/source/reasoning to these events. StepFun's
  top-level cached_tokens and OpenAI's nested cache count share normalization;
  Anthropic reads/writes count once in cumulative stream input.
- Building/repair schemas omit server-bound job IDs and the duplicate full scene
  under scene_patch.replace. Planning offers read/search only; replace_many
  accepts up to eight non-overlapping edits to existing files. Repeated paths
  use the same original hash and unique original blocks; replacements never
  match text inserted by another edit. Verify the combined file imports/sizes
  and project limit before writing, roll back IO failures and build once.
  Conflicts leave all files unchanged. Managed paths and policy gates still apply.
- Runtime context reads the installed common.ts descriptor and current file
  hashes/hook locations. Only an exact accepted-plan template match (CRLF/LF may
  normalize) permits the descriptor/API reference, using the same verification
  as source generation. Changed, unknown or missing descriptors mean
  legacy/custom, even when a version comment survives; provide the actual hook
  locations for bounded reads. Verified versions attach `api_reference` from
  `runtime_reference.go`; tests bind every documented member
  to the template declaring that version and the 2D key list to `createInputs`.
  A new helper version needs its own reference entry; re-exporting templates
  (voxel-1) bind members to the runtime named by `Source`. In Studio-bound runs,
  reads and searches of `vendor/*.js` return `vendor_not_readable` with a hint to
  that reference instead of minified internals; unbound callers keep the content
  plus the hint. New three-2 helpers use the same ray
  blockers for shots and observations; enemy attacks respect cover, cooldown and
  the engine clock. Scene-builder damage remains separately owned.
- Browser reports forward at most five finite dist/game.js frame locations.
  Private in-memory source maps resolve only current source with a matching
  digest, returning bounded excerpts. Private maps and their embedded source
  snapshots never enter project files or exports. Preserve channel/token/build
  binding and all existing error gates.
- Orchestrator validation may reuse a successful complete result only for the
  same job, scope, scenarios, source/assets/runtime/plan fingerprint and current
  unexpired browser evidence. Targeted, changed, failed or stale results never
  qualify; explicit validation tools request a fresh run.
  Reject ineligible in-memory candidates before hashing the project; recheck
  eligibility under the existing locks and all evidence after the fingerprint.
- An explicit Studio retry of a failed published 2D edit may validate its
  restored draft before another model call when `src/main.ts` differs from the
  immutable base revision. Missing or unchanged source follows normal building.
  Fresh full validation, bounded repair and publication gates still apply;
  a copied validation report never establishes success.
- Compact design correction errors expose up to eight independent field issues,
  examples and remaining attempts. Draft omission/null/array rules and the
  charged correction budget remain binding; deterministic asset normalization (plural IDs, duplicate roles, unknown asset metadata) runs before validation and is reported as warnings.
- Building and repair context carries `current_sources`: the complete `src/main.ts`
  (and `src/voxel.json`) with sha256 when each fits one 24000-byte read window
  (12000 bytes for the compact small-model profile), so
  the first call can be the edit. Larger files instead appear in `omitted_sources`
  with reason, byte/line counts, hash and a readable first range when possible.
  File-size/path gates still apply. Prompts and budget guidance must not claim
  omitted content or an unverified runtime reference was supplied.
- Early core-loop checks use startup scope while accepted features/stages are
  unfinished. Full validation follows their complete implementation (or the
  disclosed startup-only scope); publication and shared repair limits stay intact.
- Ordinary phase requests carry current intent/plan/runtime once, up to four
  complete recent tool rounds and at most eight assistant messages. Private
  checkpoints retain earlier messages when request fitting drops them;
  tool-free retries retain their source snapshot.
  Repair packets preserve failed input steps, passed checks, source diagnostics
  and the remaining shared budget. Tool-result telemetry records only allowlisted
  operation/status, phase and duration, never arguments, output or reasoning.
  Earlier user requests may use a lossless text dictionary plus ordered sequence
  only when smaller than the original array. Preserve chronology, multiplicity
  and every distinct request; repeated instructions can be additive or restore
  an earlier preference. Never deduplicate against history omitted by fitting.
- Fresh non-scene platformer/topdown/shooter/blocks/board and guided 3D jobs use
  source-only generation immediately after planning when every installed template
  file exactly matches the accepted plan (CRLF/LF may normalize). No repair is
  consumed before the first validation. Scene/custom mechanics, blank/voxel bases,
  changed source, published edits and resumed drafts retain tools. Resume identity
  comes from the server-owned JobRun, not the database Job projection. Include
  the actual validation plan, design obligations and engine presentation guidance;
  retain unchanged-starter, compiler, browser and publication gates.
- Tool-free starter generation supplies the current job, fixed `src/main.ts`
  target and source digest. Its request view retains available reasoning and the
  current snapshot, omitting earlier tool calls, results and rejected source;
  unchanged helpers use a compact API only after exact comparison with the
  accepted plan's generated template (a version marker alone is insufficient).
  Custom/old helpers keep their complete references with the existing 96000-byte
  per-file limit (`SourceGenerationMaxBytes`). Known-helper scene/mechanics summaries explicitly report
  omissions and revisions. Keep the complete main.ts (`ReadJobSource`, same byte
  bound, never the 240-line interactive window) and the accepted plan.
  Retries preserve the sent prefix/reference packet and append correction plus
  newly available reasoning without additional model calls or repair attempts;
  the private archive remains complete. A completed single `game_maker_file`
  write envelope (arg-key XML, function/parameter XML, or name/arguments or flat
  action JSON) may supply source data without tool dispatch. Omitted write/job/
  digest metadata inherits the server binding; explicit metadata must match.
  Reject extra/duplicate fields, other targets, mixed prose and incomplete
  output. The checked write still rejects concurrent edits. Format correction
  shares one retry with stream/deadline/output-limit recovery; normal validation
  owns success. Source generation reserves 16384 output tokens of source for 2D
  or 32768 for 3D plus the separate `gameMakerReasoningTokens` allowance,
  clamped to every eligible route's limit and reserved before context fitting.
  If fitting fails, drop the reasoning allowance first, then retry at 16384,
  before any model call. A length-limited answer is discarded; its correction
  requests a concise complete implementation using the same helper packet and
  output ceiling. History carries earlier reasoning under server-written
  assistant notes; a response repeating one is rejected like a format error and
  never saved.
- Game Maker streams use the configured per-call LLM timeout through a request-local retry override; the global chat retry timeout is unchanged. That timeout is the base of a throughput deadline (`llm.NewThroughputDeadline`, `RunConfig.StreamThroughput`, also in tool-free source generation): once at least 5 s of output is measured above 3 tokens/s, the deadline moves to the time the remaining max_tokens reserve needs at that rate (+10%), up to 4× the base and never past the job deadline. Streams without output, below the rate floor or idle beyond the chunk timeout still end as before; other agents keep the fixed timeout (CYD Creator reuses `gameMakerStreamThroughput`). Require a complete stream marker before executing generated calls, retain interrupted reasoning privately, and distinguish timeout/truncation from empty completion. An empty new-game build can reach the existing bounded unchanged-starter recovery only with an accepted plan; all compiler/browser/publication gates still apply. Model progress events carry only allowlisted status codes, never reasoning or provider text.
- Providers count reasoning against max_tokens, so Game Maker keeps reasoning out of the answer budget: every phase requests its answer reserve (planning 4096, building/repair 16384 for 2D, 3D and voxel) plus the separate 16384-token reasoning allowance, clamped to the route's output limit, only when the locally resolved route has at least 65536 context and over 8192 output tokens; otherwise retain the general agent reserve. Normal route caps, context fitting and the per-call LLM timeout still apply (about 33K tokens fit 600 s at ~70 tokens/s). The 3D runtime guide requires planned gameplay roles in `config.objects`: bare model instances are not observable item/enemy/goal targets, and `api.event` is feedback rather than a gameplay counter. A custom `config.action` returning false cancels the primary action; successful interactions return true, while absent custom targets leave the built-in action available and real contacts still own pickups.
- Game Maker projects use `Games/<slug>` as their only persistent identity and live below the configured Virtual Desktop workspace. Never persist or return absolute host paths.
- Game Maker continuations keep the original project request, recent user changes and a private provider-native conversation (including available reasoning and complete tool/result groups) in `gm_agent_context`, isolated by project and revision. Checkpoint at tool/phase boundaries and cancellation; rebuild current system/tool scope, never replay historical calls. Route budgets still bound restored history; only runs that opt in with `PreserveReasoning` (`RunConfig`/`MinimalLoopOptions`: Game Maker, Detective, CYD Creator) retain completed reasoning. Provider changes keep reasoning as historical data rather than replaying provider-specific fields. Do not expose this state as Studio chat, assets, exports or general memory. Keep one failed/interrupted working copy per project until continuation succeeds or the project is deleted. Resume copies it into a new job and retains installed source; publication still requires all existing checks. Release the global writer only after working-copy cleanup. `StartJobRequest.resume` resumes directly without copying a prompt into the UI editor. An empty resume prompt is replaced with the continuation sentence before it is stored.
- StepFun chat requests use provider-compatible wire messages: omit stored `reasoning_content` and empty reasoning-only assistant entries, include `content: null` with assistant tool calls, and omit tool-result `name` and stream-only tool-call `index`. Keep the private continuation unchanged; the LLM transport also applies this shape to StepFun routed through OpenRouter.
- StepFun response `reasoning` maps to private `reasoning_content` in JSON and
  SSE, including chunks without usage. Existing nonempty canonical fields win;
  never expose reasoning as answer/source or double-count aliases. Stream errors
  report text/reasoning byte counts only. Thinking and call deadlines stay enabled.
- Jobs run in isolated staging directories with one global writer. A validated internal `.aurago/game-plan.json` is required before agent code/media/import mutations. Planning has an initial submission and at most three charged corrections: an attempt that reduces the independent issue count is free, every job caps at five submissions, and three `dry_run` previews validate a design without accepting it or consuming the budget. The selected model plans and builds; no user confirmation or stronger-model fallback is required.
- `ListRevisions` compares each revision's stored file hashes with its parent and fills `changed_paths` (at most 40 relative paths), `changed_count` and `truncated`. Revision 1 compares with an empty parent. The list contains no file contents and no host paths.
- `WorkspaceDrift` is a read-only hash compare of the published revision and the editable workspace. `GET /api/game-maker/projects/{id}/workspace-drift` uses the same read permission as other project reads. Skip export dot paths (`.aurago/` included). Cap each path list at 40 and return full counts plus `truncated`. Revision 0 or equal trees return empty counts. A missing workspace lists the published files as removed. Do not write blobs. Relative-path errors only; `RedactHostPaths` stays the last boundary. Symlinks and the publication size limits still reject the walk.
- `adopt_workspace` publishes Code Studio edits without a model. Reject it before the writer lock when there is no current revision, the drift is empty, or the request also resumes, supplies a prompt, asset selections, pack IDs, model IDs, presentation, image or music generation, or preview diagnostics. The job still copies the workspace. Accept the stored plan with `acceptPublishedPlan` (force `CurrentRevision` 0 for the preserve check) and do not rewrite `.aurago/game-plan.json`. Validate once at full scope, or startup for free-code 3D without scene data. Publish through `publishValidated` with source `adopt` only when the checks pass. No repair and no image review. Failure keeps the last revision.
- A normal edit that copied a drifted workspace (`ResumeFrom` empty, base revision above 0, kind not adopt) names that drift on the planning `JobRun` only. `compactGameMakerContext` puts `workspace_drift` in the untrusted context after the stable profile. Building omits it. A drift error omits the field and does not fail the job. The profile fingerprint stays unchanged.
- After plan acceptance, emit `plan_summary` once: template, objective truncated to 240 runes, perspective, up to 8 sorted control keys, and up to 12 assets `{role, id}`. No scene, mechanics, or model prose. `GET /api/game-maker/projects/{id}` also returns the latest `plan_summary` and `latest_job`.
- `gm_jobs` stores `error_code`, `gameplay_status`, and nullable `checks_passed`, `checks_failed`, and `repair_passes`. `ensureJobOutcomeColumns` backs up the database only when a column is missing. Fresh databases already include the columns. Terminal `job_status` carries the code and counts. Codes are `planning`, `planning_protocol` (the model never reached `set_design`; `planBudget.calls` is zero, so the correction budget was not spent), `validation`, `preview`, `timeout`, `cancelled`, `model`, and `internal`. Unavailable checks are not failures. A nil repair count means the run never entered repair; adopt stores 0.
- `gm_jobs.open_checks` stores the bounded JSON list behind the failure card: at most six still-open checks (failed or unavailable, never passed or skipped) with `id`, `status`, `expected`, `observed`, at most eight executed `steps`, `repairable`, and the result's `ValidationNextAction` as `next_action`. Text fields are truncated, unreadable values degrade to "no open checks", and the list never carries source or provider text. `LatestJob`, `GetJob` and the terminal `job_status` expose it as `open_checks`; a ready job clears it. The Studio card renders it above the technical details; it never becomes `passed`.
- `gm_jobs.small_model` and `gm_jobs.building_mode` record the small-model decision
  (additive columns; the backup still runs only when a column is missing). A job is
  small when the resolved route has fewer than 65536 context or 16384 output tokens
  or `StartJobRequest.SmallModel` opts in; unknown models are never small and an
  explicit `false` cannot disable a resolved-small route. `building_mode` is `adapt`
  only for a new game (`kind: create`, `BaseRevision 0`) on a small model whose
  template is not `minimal`, `three` or `voxel`; every other job stores `rewrite` and
  adopt keeps it empty. Both values reach `GetJob`/`LatestJob`, the terminal
  `job_status` and `plan_summary`.
- In `adapt` mode the building/repair context carries `edit_targets`: at most six
  `{path, sha256, start_line, end_line, snippet, purpose}` anchors derived
  deterministically from the installed `src/main.ts` (snippet at most 400 bytes).
  A full-content `write` of `src/main.ts` is refused with `mode_requires_replace`
  and that anchor list; `replace`, `replace_many`, other files under `src/`, the
  `minimal` base and normal jobs stay writable, and the refusal consumes no repair
  pass and does not end the round. Tool-free starter generation never runs in
  `adapt` mode, so the server cannot rewrite the starter itself. Guided 3D starters
  emit mode, objective, pacing, one object or stage line and `startGame(` as separate
  lines under 400 bytes, so the write gate closes when those anchors exist. A line
  that appears twice is not an anchor. An empty resume replays the selection stored
  on the previous job only when the restored copy has no accepted plan; an accepted
  plan keeps its bindings. `gm_jobs.plan_accepted` and `gm_jobs.requested_selection`
  record that. The failure card uses the pending-selection sentence for
  `planning` and `planning_protocol` only.
- The compact small-model building context caps `current_sources` at 12000 bytes and
  replaces `api_reference` with a bounded cheat sheet (`CompactRuntimeReference`, at
  most 2000 bytes) built from the same runtime reference. Ordinary runs keep the
  24000-byte window and the complete reference.
- The result card reads that job's `prompt_usage` events only. It reports the call count, the sum of measured input and output tokens when every call reported both, and `estimated_cost_usd` when every call has a finite price. Cache reads and writes stay out of the token sum. Legacy `token_usage` is ignored. A missing measurement stays off the field so the UI can call it unknown. Zero calls omit the line, including adopt. Fill it on the terminal `job_status` and in `LatestJob`. It is not a `gm_jobs` column and `GetJob` does not scan it.
- Game Maker planning/building/repair use a server-owned per-run tool budget of `max(40, ceil(circuit_breaker.max_tool_calls * 1.25))`, rounded upward and logged at job start. It overrides personality/tool adjustments for that run only; the shared system setting, other agents, time/token limits and plan/repair attempt limits remain unchanged. Tool-free implementation/image review stays tool-free.
- Provider schemas may encode `set_plan.plan` as a JSON object string. Native and XML fallback parsing must preserve the complete structured plan, including the legacy XML task-prompt alias. Keep field-specific validation errors across planning rounds and in the final failure; successful correction must not carry obsolete plan errors into building.
- Plan submission rejects unknown JSON fields within the same correction budget and reports independent asset errors together. Each asset role uses singular pack/asset/assembly IDs; variants need distinct roles. Legacy plan perspective errors target root `plan.perspective`, never a writable asset `view`. Compact-design errors instead target `design.assets`, keep the requested base and return bounded compatible catalog alternatives; corrections retain the complete asset array. Asset search ranks content above pack names and excludes pack-name matches when already scoped to that pack.
- Planning ends on server-owned acceptance or exhausted corrections through `RunConfig.RunComplete`, without another LLM request. Remaining declared native calls receive one skipped result each and are never dispatched; cancellation still fails the round. The orchestrator alone advances an accepted plan to building.
  Explicit user-selected artwork/model/presentation IDs and sound bindings are
  checked at the shared compact/legacy plan acceptance boundary, within the same
  correction budget. Never report acceptance and revoke it later for selections.
- Exhausted repair budgets (the per-job repair limit), unavailable browser feedback, and the first validation of each repair round also end the agent round through server-owned completion. Building can continue after core checks while budget remains. Exhaustion retains the last concrete failed check instead of replacing it with the budget error. Earlier rounds cannot complete a new round.
- A rejected tool call in the tool-free limit response during Game Maker building/repair hands the saved source back to orchestrator validation. Never execute the extra call, retain its prose, increase budgets or mark the game successful without the existing checks. Planning failures, cancellation and other provider/agent errors still fail normally.
- New 2D jobs install one of six embedded templates; guided 3D offers fps, exploration, transport, flight and space. New 3D planning prefers one of those bases when the request fits that loop. It does not rewrite an explicit `three` design. Edits retain existing code. Both guided paths require compilation, build-bound browser startup and full gameplay checks. Free-code `three` supports startup only and explicitly leaves gameplay unverified. A ready `job_status` includes `gameplay_status`; the Studio shows that disclosure on the activity line and on the result card, before Play now, only when the value is `unverified` on a non-voxel 3D project. The card button fills a change request to move the game onto a guided base and does not start a job. Failed/cancelled jobs preserve the last playable revision; the shared repair limit defaults to three passes and a job may request 1–6 (`StartJobRequest.MaxRepairPasses`); tool and orchestrator validation share it.
- Phaser 4.2.1 and Three.js 0.186.1 are embedded, pinned, offline runtimes. Generated games may not load CDNs, external APIs, remote assets, or AuraGo endpoints.
- Phaser phase guidance must distinguish dynamic and static Arcade bodies from their game objects. Moving paddles remain dynamic and immovable; body `setVelocity`/`setPosition` runtime errors receive bounded repair hints without weakening validation or increasing the repair budget.
- Phaser `scene.restart()` reuses the instance. Guidance and source references
  require custom cooldowns, spawn clocks and per-run flags to reset in `setup()`;
  class initializers alone cannot reset values against a restarted `elapsed`.
- Dynamic gameplay checks use bounded `target` steps (move, aim, reach, interact, catch, avoid, select), driven only by normal keys/pointer input and read-only engine geometry. Keep roles/IDs independent of artwork. Never mutate actors, damage, randomness or counters to pass. Target reports require matching steps and physical effects; counter-only changes, missing targets, blocked routes and unsupported controls stay unavailable. Only observed contact/response contradictions fail. `player_distance` is derived from engine positions. Targeted input lives only in the injected preview driver; read-only 3D observations may ship with the common helper. Existing scenario/driver deadlines, lifecycle cleanup, export exclusion and publication gates remain binding.
- Board selection observes both `scene.marks[index]` and the actual cell object's
  `opened` state, including cells with a zero mark. Skip occupied/open cells and
  still require an observed target change plus the scenario's metric change.
  Verify `TestTargetControlBrowser/occupied_cell` and `/opened_cell*`, including
  the counter-only rejection.
- 3D reach/interact steps navigate active targets using live world geometry even
  outside the camera frustum; low FPS pickups leave the viewport before contact.
  Aim/select still require visible targets. Preserve collision/height checks,
  input-only control and actual physical-effect evidence.
- 2D aim at a role may retarget formation members using the observed projectile
  firing lane. Exact IDs still select only that object; keep the original time
  budget and require a metric change plus a physical effect on the pursued target.
  Keep the alignment lane stable while moving; observe recent projectiles and
  turn travel-facing weapons back onto that lane through normal movement input.
  Never assume the initial projectile direction still describes subsequent shots.
- Asset detail examples must include executable preload/setup methods and preserve the template lifecycle. Asset creation rejects unloaded textures or missing frames; test binding requires a live controlled object assigned by setup. Missing Phaser textures cannot pass asset validation.
- New templates import and preload exact planned asset roles in common.ts. Every 2D template uses those roles through body(...,role), with uniformly fitted art and separate collision proxies; changes retain this wiring. The build guard rejects pack metadata as a texture key at the shared texture-manager boundary. Gameplay scenarios must allow actual travel time; hit counters represent collisions, including hits on durable targets.
- Scenario schemas enumerate `GameMetrics()` and `GameKeys()`, the same lists `validateScenario` enforces; never advertise a metric or key it rejects. Voxel scenarios accept only key/wait/observe steps (the voxel driver runs nothing else) in both the planning schema and `checkPlan`.
- Additional plan scenarios are optional (0–8). Schema 4 starter scenarios replace input, primary-action or rules checks only with driven `increased`, `decreased` or `changed` assertions for the matching metric; `equals` and `at_least` assertions remain additive because they may already hold before input. Scene/blank compositions retain their separate scenario selection. Lifecycle, resource and unrelated checks remain server-owned. Check results include the executed finite steps, so repairs distinguish launch/actions from collisions/hits and ESC end from natural defeat. New GameScene templates reject update overrides at startup with hook-specific guidance; preserve common.ts lifecycle and sprite following.
- Script writes preflight literal built-in metadata imports with esbuild against complete project PNG/JSON pairs, resolving from the source file. Invalid imports leave source/preview and repair counts unchanged and return existing import paths; ordinary module writes retain their build-validation workflow. Arcade collider/overlap registration rejects wrapper records whose body is a GameObject; spawned collision objects belong in persistent groups. Repairs preserve passing sprite/input behavior and the accepted plan.
- Source writes return advisory `lint` findings (at most five) for `src/main.ts`:
  missing `setup()`, missing `this.player`, declared stages without
  `configureLevels`/`levels:`, external URLs and unused planned roles. Lint never
  blocks a write, never feeds `passed` and never changes build or validation.
- Game Maker tool results reach the model as plain JSON (`<`, `>`, `&` literal) inside readable `<external_data>` source isolation, so reads can be copied exactly into `replace`. They remain untrusted data; boundary-forging or Guardian-critical content keeps the fully escaped form (see root Security & Safety).
- The isolated Game Maker agent receives phase-specific schemas for `game_maker_project`, `game_maker_file`, `game_maker_asset`, and `game_maker_validate`. Planning exposes only reads, asset discovery and `set_design`; build/repair omit plan mutation. Embedded guidance is already active; redundant skill activation is not advertised. `invoke_tool`, generic filesystem/shell/Python/network tools, and uncurated Agent Skills must remain unavailable.
- Studio dispatch uses a server-owned job context, never a job inferred from session names or the globally active job. Omitted job IDs use this binding; mismatches fail. In bound runs only, a file content payload without an operation means write. Unbound calls retain explicit job/operation requirements and all policy/path/size gates remain enforced. Plan examples distinguish position metrics from primary-action counters.
- `set_design` accepts a compact closed object (base, objective, features, optional assets/settings/preserve/scene/mechanics/presentation/scenarios). The server fills the canonical plan and exact catalog metadata. Only misplaced root `outcomes`, `lives`, `blocks` and `events` normalize into `mechanics`; conflicting simultaneous values are rejected and all normal value/size checks still apply. A root `dimension` equal to the project's is dropped as redundant; a different value is rejected naming the fixed dimension. Schemas and examples advertise the canonical nested form. Corrections retain omitted/null fields and replace supplied arrays; a flat mechanic correction retains omitted sibling helpers. The charged correction budget is shared with legacy `set_plan`. Guided 3D resolves default roles and validates FPS rigs/bindings; user-selected models cannot disappear. Goal/speed/duration settings apply only to guided 3D. On a non-guided base, a correction that omits settings or supplies settings:null removes incompatible carried settings from the failed draft, including after a base change; other fields and guided settings retain their omission/null semantics. Explicit unsupported settings still reject with a design.settings correction that preserves the requested base. Published guided settings must not be inherited into a non-guided edit.
- File reads return bounded line ranges and a full-file SHA-256; native numeric and XML numeric-text `start_line`/`end_line` use the same finite nonnegative integer checks and range limits. Unique exact `replace` requires that digest. Writes return `written` and bounded `build.ok`/diagnostics separately. Preserve path/policy/import limits and reject stale edits before mutation. Imported 2D roles bind automatically in every template, with catalog-supported animation and separate collision proxies.
- The browser test driver waits for asynchronous GLB initialization within its existing deadline. Guided 3D observations come from live input/state, including FPS aiming/reload; no model-provided code is evaluated. Pointer lock does not grant same-origin access. `TestGuidedBrowser` is opt-in with `GAMEMAKER_GUIDED_BROWSER=1`; `TestGameMakerLiveEvaluation` runs explicitly selected providers on their own host, synthetic projects only, without exporting credentials.
- `SetPlanJSON` accepts a plan object or one JSON-string wrapper at the shared validation boundary. Malformed inner JSON returns its syntax position, not a string-to-struct error. All transports retain strict field/rule checks, size limits and the shared charged correction budget; never guess missing plan content.
- System-managed Game Maker Agent Skills must match the complete embedded package. Startup self-heals their `SKILL.md` and registers the exact single-file binary package through the Skill Manager before exposing Game Maker; Guardian/SkillSpector latency on unrelated packages must not delay it. Optional scanner warnings can be replaced by verified binary provenance. Explicitly blocked packages, extra files/directories, symlinks, missing content and hash mismatches still block readiness. Runtime package hashes remain checked before use; bundle registration is internal and never accepts model/user-provided trust claims.
- Preview iframes omit `allow-same-origin`, use short-lived project/job-bound tokens, restrictive CSP including document `sandbox allow-scripts allow-pointer-lock` (so a top-level tab cannot ride the admin session), external-connect blocking, and a source/channel-validated bridge for bounded diagnostics and finite test inputs.
- Preview tests accept only finite key/pointer/wait/observe steps, never model JavaScript. The authenticated parent forwards bounded numeric observations and at most two bounded PNGs. The server compares evidence; missing observations never pass. Runs are bound to a build and preview token, last at most 60 seconds, and reset gameplay afterwards. Optional image review uses only a confirmed image-capable selected model, is tool-free and cannot override technical checks.
- Phase-specific verified guidance uses prepared prompt profiles; human intent, model plans, project files and diagnostics stay separate untrusted data. Planning text is never streamed/persisted as chat; final player prose is held until publication. `.aurago/validation-report.json` binds results to the compiled bundle hash and is revisioned but excluded from ZIP export.
- Image and music generation are optional project capabilities. Generator failure, disabled configuration, or exhausted budget must return a visible procedural fallback without claiming unsupported 3D model generation.

### Game Maker sprite library contract

- The same catalog includes `kind: model3d` pack `aurago-low-poly@1.0.0` with
  220 original MIT Blender models. `assets/game-maker-low-poly/AGENTS.md` owns
  editable sources, generators and acceptance fixtures. Runtime GLBs, previews,
  metadata and licenses together must stay under 100 MiB uncompressed.
- Model selection uses `model_asset_ids` (1–64) and plan schema 2 with metres,
  metric scale and 3D colliders. Import only explicit IDs after plan acceptance,
  including declared shared animation dependencies; never overwrite edited copies.
  Sprite plan v1 and all twenty-two fixed-grid sprite packs remain compatible.
- Three.js stays at 0.186.1. Rebuild the local GLTFLoader/SkeletonUtils/OrbitControls
  helper using `node scripts/build-game-maker-3d.js`. One game-owned clock advances
  independent animated instances; static instances share geometry. Studio owns
  one disposable viewer, never one render loop per catalog card.
- Every 3D build/repair prompt includes the compact public model API and actual
  per-import examples, including reconstructed imports from existing projects.
  Asset detail examples precede bulky metadata so bounded tool outputs retain
  them. Validation rejects unchanged 2D/3D scaffolds and plan-installed templates;
  automatic imports and diagnostic injection are not implementations. Empty
  provider completions fail the job unless a server-owned phase boundary ended
  the round. Successful imports or starter gameplay checks alone are not a game.

- `internal/gamemaker/asset_packs/` owns twenty-two locally packaged 10×10 RGBA sheets (64px
  cells), versioned JSON and a compact catalog. Original images and reviewed
  crops remain in `production/` but are excluded from the binary. Rebuild with
  `python scripts/pack_game_sprites.py`; verify with `--check` (Pillow 12.2).
- Sprite operations stay inside `game_maker_asset`; omitted operation still
  generates media. Selected `asset_pack_ids` import before the agent runs.
  Imports publish PNG/JSON together, enforce edit policy and existing limits,
  preserve differing copies, and record provenance. Revisions/exports use project
  copies. Catalog reads are authenticated and disabled with Studio. No new tool
  isolation exception or migration is permitted. Verify with `go test
  ./internal/gamemaker ./internal/server -run 'TestSpritePack|TestGameMakerAssetPack'`
  plus existing Game Maker UI checks.
- Modular buildings and large vehicles expose `assemblies` with pixel bounds,
  an origin and ordered numeric-frame parts. Slice one shared canvas to preserve
  seams; animated parts start together. Pack selection limits follow the embedded
  catalog. Keep assembly previews and import examples aligned with this metadata.
- Pack version 2 explicitly records entity/action groups and transform permissions
  in the production manifest. Radians are explicit; fixed assets cannot inherit
  category-wide mirroring. `search_assets` returns six (max twelve) targeted hits;
  `describe_asset` returns related actions/directions and missing actions.
  `vendor/aurago-game-1.js` handles exact frames, animation holds and complete
  assembly transforms. Physics proxies stay separate from visual containers.
- Pack import/detail results include a concrete helper example; selected-pack
  context stays compact. Built 2D games guard the loader against treating built-in
  sheets as single images/atlases or using the wrong grid. Keep that guard at
  the common loader boundary, including config arrays and variable URLs.
  Browser startup must observe three seconds after visible-canvas readiness
  to catch common delayed spawn errors. Verify with
  `node scripts/test-game-maker-sprites.mjs` and Game Maker Go tests.

## Work Guidance

Keep existing `common.ts` working on revisions. Advertise new APIs to older games
only after reading their source and deliberately incorporating needed helpers.
Do not patch a published game merely because a new starter changed.

- Version both the Three.js module and core filenames. Installing a new runtime must not overwrite the shared `three.core.min.js` used by older published games. Preview/asset fallbacks retain the r185 pair; export selects the pair already referenced by the published revision, including older Voxel exports.

## Desktop invocation ownership

- Server policy combines Game Maker and Desktop readonly. Policy revocation cancels
  active job contexts under policyMu -> mu; publication keeps its policy lock and
  validation gates. Agent phases also carry the revocable Desktop publication
  context. Cancel remains available after readonly; saved failed drafts survive.

## Verification

- `TestCustomScenarioCoverage*` and `TestGameScenarios*` cover additive baseline
  assertions and driven transition coverage. `TestValidationRequestErrors*`
  covers request correction without preview, budget or round changes;
  `TestFreeCodeThreePlanning*` checks the disclosed gameplay boundary.
  `TestGameMakerPlanningNamesGuided3DBasesBeforeFreeCode` and
  `TestGameMaker3DPlanningBasePrefersGuidedTemplates` check the early 3D base choice.
- `TestVoxel*` covers deterministic worlds, chunk collision/mesh updates,
  transactions, malformed saves, CAS/revision isolation, restart and copied DB
  migration. `GAMEMAKER_VOXEL_BROWSER=1` adds actual gameplay, touch, respawn,
  sandbox evidence, export/subdirectory IndexedDB and renderer cleanup.
- `TestGameMakerVoxel*` in server/UI covers stable profiles, HTTP policy and
  parent bridge isolation. UI browser tests use `AURAGO_RUN_BROWSER_SMOKE=1`.
  `VOXEL_SCREENSHOTS` may target ignored reports. Hardware FPS acceptance remains
  separate from headless software rendering and touch emulation; see
  `documentation/game-maker-voxel.md` for the performance scene and limits.

- `TestPreparedProfile*`, `TestUsageObserverContextGenerations`,
  `TestProviderUsageObservation`, `TestUsageUnknownZeroAndCumulative`,
  `TestGameMakerPromptProfiles` and `TestGameMakerPromptUsage*`
  cover stable prefixes, context limits, native rounds and nullable usage.
- `TestStarterReferenceVerifiedAndBounded` requires at least 50% reference-token
  reduction for unchanged 2D/3D starters and full legacy/custom compatibility.
  Local equality/reduction proves no provider cache hit or monetary saving.

- `TestMissingTargetStartsBoundedRepair`, `TestMissingTargetRepairSharesBudget`,
  `TestMissingObservationStillEndsWithoutRepair` and
  `TestRepairableCheckClassification` cover repairable unavailable checks.
  `TestValidationPlanMatchesExecutedChecks`, `TestBaseChecksNameTargetsPerBase`
  and `TestValidationNextActionNeverClaimsUnobservedSuccess` cover disclosure;
  `TestAgentFileErrorsStayProjectRelative`, `TestRedactHostPaths` and
  `TestProjectSearchCoversOnlyEditableSources` cover paths and search. Agent
  `TestGameMakerToolsDiscloseChecksAndNextAction` and server
  `TestGameMakerPlanningContextNamesBaseChecks` and
  `TestGameMakerBudgetIsStableAndPhaseSpecific` cover the tool and context side.
- `go test ./internal/gamemaker` and focused agent/server tests.
  `TestWorkspaceDrift*`, `TestAdopt*`, `TestPlanSummaryPayload`,
  `TestJobOutcomeColumns`, `TestPromptUsageCard*`, and server `TestWorkspaceDrift*` cover drift,
  adopt, plan summaries, stored job outcomes, and the result-card usage line.
- `GAMEMAKER_EXPERIENCE_BROWSER=1`: normal-input contact, feedback, checkpoint,
  pause, stage/result/restart and narrow-screen controls in exported 2D/3D fixtures.
  `TestPlayerUIBrowser` covers pre-start freeze, compact desktop/narrow-window HUD,
  menu pause, simultaneous touch input/cancellation, direct board input, FPS
  drag-look without firing, localized labels and restart cleanup in exports.
- `TestRuntimeInputRegressionBrowser` uses the same experience flag for locked
  mouse shots, held-action veto/default behavior, board input gates and active
  3D-level restarts. `TestWorldIsometricExportBrowser` covers the 2D restart.
  Builder/Voxel Node checks cover patrols, projectile order/retention and full
  creative inventory, including save dirtiness and unchanged Survival behavior.
- `GAMEMAKER_TARGET_BROWSER=1`: positive and negative read-only target evidence.
  `TestFPSPickupBrowser` covers normal input, target driving and pickup reset;
  `TestTargetControlBrowser/fps_ground_` covers low/behind, blocked, elevated and
  counter-only targets. `TestRuntimeContextIdentifiesInstalledPickupCounter`
  checks repair guidance against the actual installed helper.
  `TestFPSPickupBrowser` also runs both historical helper fixtures;
  `TestUpgradeThreePickupObservation` and `TestLegacyThreePickupBuildAndResume`
  cover exact recognition, authored-source preservation, reset instrumentation,
  build/context agreement and failed-draft continuation.
- `GAMEMAKER_SHOOTER_BROWSER=1`: `TestShooterBrowserDelayedSemiAutomatic`
  reproduces blind firing versus aimed hits on a moving offset enemy, plus broken
  collision, missing-target and counter-only failures, and a staggered formation
  whose first visible member is off the firing lane, including travel-facing
  weapons and their broken-collision variant. `TestShooterScenarioInput*`
  covers accepted-plan preservation, budgets, disclosure and custom-input isolation.
- `GAMEMAKER_GUIDED_BROWSER=1`: starter engine/lifecycle browser checks.
- `GAMEMAKER_NIGHTLY_BROWSER=1` runs only `TestNightlyExportBrowser`: one unmodified platformer export and one exploration export using `defaultModelRoles`, with no LLM. `.github/workflows/game-maker-nightly-browser.yml` schedules that test. The other `GAMEMAKER_*_BROWSER` suites stay opt-in and are not part of the job. Without the variable the test skips, so `go test ./...` does not need Chrome.
- `GAMEMAKER_PREVIEW_BROWSER=1`: hidden/resumed previews, delayed layout, invalid
  canvases and loading-HUD recovery in Chrome.
- `GAMEMAKER_OPTIMIZATION_BROWSER=1`: `TestFPSCoverBrowser` checks actual enemy
  damage and player shots with/without cover using normal browser input.
- `TestSourceBatchPreflightSearchAndRuntime`, `TestPrivateBuildSourceDiagnostics`,
  `TestValidationReuseRequiresExactCurrentEvidence`, and
  `TestDesignReportsIndependentCorrectionsWithinBudget` cover agent assistance.
- `TestSourceBatchSameFileEditsUseOriginalSnapshot` covers simultaneous edits,
  overlap/stale-hash rejection and whole-batch preflight. Runtime
  `TestRuntimeContextRejectsRetainedDescriptorOnChangedHelper` and server
  `TestGameMakerSourceContextDisclosesOmittedContent` cover source-aware guidance.
- Server `TestGameStarterProviderSourceFormats`,
  `TestGameStarterWrappedSourceRequiresExactSingleWrite`,
  `TestGameMakerUnchangedStarterGetsBoundedCodeRecovery` and
  `TestGameMakerSourceStreamRecovery` cover source recovery and its write boundary.
- `TestMinimalLoopEnforcesOutputBudget`,
  `TestMinimalLoopOutputReserveCannotBypassContextLimit`,
  `TestMinimalLoopStepFunStreamBudgetAndPrivateReasoning` and
  `TestStepFunReasoning*` cover real request ceilings and private SSE/JSON reasoning.
- `node scripts/build-game-maker-presentation.js --check` and
  `node scripts/test-game-maker-presentation-evidence.mjs`.
- Package changed runtimes with the matching resource flags and verify `--check-assets`.

## Child DOX Index

- `asset_packs/AGENTS.md`: original local content, metadata and reproducible packing.
- `export/AGENTS.md`: revision blobs, publication fingerprints, receipts and ZIP export.
- `validation/AGENTS.md`: validation plan disclosure, repairable checks and evidence.
- `voxel/AGENTS.md`: Voxel definitions, finite worlds, transactions and saves.
