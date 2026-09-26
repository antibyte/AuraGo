# Game Maker

## Purpose

Own offline game planning, source editing, asset import, browser validation,
revision publication and standalone export for Phaser and Three.js games.

## Ownership

- `guided_design.go`, `skills.go`, `skills/`: compact agent contracts and examples.
- `templates/`: new-project source bases; preserve authored existing source on edits.
- `runtime/game-flow.js`: player feedback and result/stage UI, driven by engine clocks.
- `runtime/aurago-game-1.js`: Phaser input/assets and read-only test binding.
- `assets/game-maker-presentation/` at repository root: editable effects/audio source;
  its build script produces the two bundled effects runtimes here.
- `runtime.go`: project-local runtime installation, also used by exports.
- `runtime_context.go`, `source_search.go`, `source_batch.go`: installed helper
  descriptors and bounded source discovery/edits for the isolated agent.
- `source_diagnostics.go`, `validation_reuse.go`: private source mapping and
  exact-build validation reuse; explicit tool validation always runs afresh.

## Local Contracts

- Terminal job states become observable only after working-copy cleanup, with the
  writer released under the same lock. Immediate retries and revision restores
  must not race a job that has already reported completion.
- Complete experiences include consequences, recovery or results, a suitable world
  and meaningful progression. Single-screen boards and peaceful sandboxes remain
  valid; do not require combat, lives, timers or linear levels for every idea.
- Phase guidance stays within 2500 characters and uses existing design fields.
  Keep its prefix deterministic; put detailed examples in curated skills.
- Feedback never changes gameplay counters, health or outcomes. Real contacts and
  game rules own those changes. Missing observations remain unverified.
- Non-scene Three.js helpers count real item/cargo/flight-goal collections in
  `pickup_events` and clear that counter on reset. Keep combat hits and cosmetic
  feedback separate; scene-builder pickup counters retain their own ownership.
  Runtime context identifies an installed legacy constant-zero pickup metric
  with its source line and repair guidance, without replacing authored helpers.
- New common templates opt into baseline visual/audio feedback. Explicit imported
  sound bindings take precedence; one mixer, gesture gate and 32-voice budget apply
  to fallback cues too. Mute/volume survive stage changes within the same game root.
- Game flow owns no loop or timers. Pause/inactive state freezes it. Scene shutdown
  or renderer disposal removes feedback, UI and audio resources. Ending play still
  renders the result UI; authored stages rebuild through the engine lifecycle.
- Phaser life-based authored rules use `damagePlayer()` and checkpoints; Three.js
  authored rules use `api.damagePlayer(amount)`. Scene-builder health/contact rules
  remain authoritative for scene-driven games. Do not double-apply damage.
- Level contents are authored source or scene data. Helpers select actual levels;
  never claim progression by cloning an empty map or incrementing a label.
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
- Visual review stays advisory and uses the selected multimodal route directly.
  Request JSON mode only for supported providers; tolerate normal JSON wrappers
  and allow at most one format correction within the original review deadline.

### Game Maker Studio Contract
- Building/repair schemas omit server-bound job IDs and the duplicate full scene
  under scene_patch.replace. Planning offers read/search only; replace_many
  accepts up to eight distinct existing paths, verifies every hash/match/import
  and project limit before writing, rolls back IO failures and builds once.
  Conflicts leave all files unchanged. Managed paths and policy gates still apply.
- Runtime context reads the installed common.ts descriptor and current file
  hashes/hook locations. Missing descriptors mean legacy/custom; never advertise
  newer bundled helpers to those projects. New three-2 helpers use the same ray
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
- Compact design correction errors expose up to eight independent field issues,
  examples and remaining attempts. Draft omission/null/array rules and the
  initial-plus-two-corrections budget remain binding.
- Ordinary phase requests carry current intent/plan/runtime once, up to four
  complete recent tool rounds and at most eight assistant messages. Private
  checkpoints retain earlier messages when request fitting drops them;
  tool-free retries retain their source snapshot.
  Repair packets preserve failed input steps, passed checks, source diagnostics
  and the remaining shared budget. Tool-result telemetry records only allowlisted
  operation/status, phase and duration, never arguments, output or reasoning.
- Tool-free starter generation supplies the current job, fixed `src/main.ts`
  target and source digest. Its request view retains available reasoning and the
  current snapshot, omitting earlier tool calls, results and rejected source;
  the private archive remains complete. A completed single `game_maker_file`
  write envelope (arg-key XML, function/parameter XML, or name/arguments or flat
  action JSON) may supply source data without tool dispatch. Omitted write/job/
  digest metadata inherits the server binding; explicit metadata must match.
  Reject extra/duplicate fields, other targets, mixed prose and incomplete
  output. The checked write still rejects concurrent edits. Format correction
  shares one retry with stream/deadline recovery; normal validation owns success.
- Game Maker streams use the configured per-call LLM timeout through a request-local retry override; the global chat retry timeout is unchanged. Require a complete stream marker before executing generated calls, retain interrupted reasoning privately, and distinguish timeout/truncation from empty completion. An empty new-game build can reach the existing bounded unchanged-starter recovery only with an accepted plan; all compiler/browser/publication gates still apply. Model progress events carry only allowlisted status codes, never reasoning or provider text.
- Game Maker projects use `Games/<slug>` as their only persistent identity and live below the configured Virtual Desktop workspace. Never persist or return absolute host paths.
- Game Maker continuations keep the original project request, recent user changes and a private provider-native conversation (including available reasoning and complete tool/result groups) in `gm_agent_context`, isolated by project and revision. Checkpoint at tool/phase boundaries and cancellation; rebuild current system/tool scope, never replay historical calls. Route budgets still bound restored history; only Game Maker opts into retaining completed reasoning. Provider changes keep reasoning as historical data rather than replaying provider-specific fields. Do not expose this state as Studio chat, assets, exports or general memory. Keep one failed/interrupted working copy per project until continuation succeeds or the project is deleted. Resume copies it into a new job and retains installed source; publication still requires all existing checks. Release the global writer only after working-copy cleanup. `StartJobRequest.resume` resumes directly without copying a prompt into the UI editor.
- StepFun chat requests use provider-compatible wire messages: omit stored `reasoning_content` and empty reasoning-only assistant entries, include `content: null` with assistant tool calls, and omit tool-result `name` and stream-only tool-call `index`. Keep the private continuation unchanged; the LLM transport also applies this shape to StepFun routed through OpenRouter.
- Jobs run in isolated staging directories with one global writer. A validated internal `.aurago/game-plan.json` is required before agent code/media/import mutations. Planning has an initial submission and at most two corrections. The selected model plans and builds; no user confirmation or stronger-model fallback is required.
- Game Maker planning/building/repair use a server-owned per-run tool budget of `max(40, ceil(circuit_breaker.max_tool_calls * 1.25))`, rounded upward and logged at job start. It overrides personality/tool adjustments for that run only; the shared system setting, other agents, time/token limits and plan/repair attempt limits remain unchanged. Tool-free implementation/image review stays tool-free.
- Provider schemas may encode `set_plan.plan` as a JSON object string. Native and XML fallback parsing must preserve the complete structured plan, including the legacy XML task-prompt alias. Keep field-specific validation errors across planning rounds and in the final failure; successful correction must not carry obsolete plan errors into building.
- Plan submission rejects unknown JSON fields within the same correction budget and reports independent asset errors together. Each asset role uses singular pack/asset/assembly IDs; variants need distinct roles. Legacy plan perspective errors target root `plan.perspective`, never a writable asset `view`. Compact-design errors instead target `design.assets`, keep the requested base and return bounded compatible catalog alternatives; corrections retain the complete asset array. Asset search ranks content above pack names and excludes pack-name matches when already scoped to that pack.
- Planning ends on server-owned acceptance or exhausted corrections through `RunConfig.RunComplete`, without another LLM request. Remaining declared native calls receive one skipped result each and are never dispatched; cancellation still fails the round. The orchestrator alone advances an accepted plan to building.
- Exhausted repair budgets, unavailable browser feedback, and the first validation of each repair round also end the agent round through server-owned completion. Building can continue after core checks while budget remains. Exhaustion retains the last concrete failed check instead of replacing it with the budget error. Earlier rounds cannot complete a new round.
- A rejected tool call in the tool-free limit response during Game Maker building/repair hands the saved source back to orchestrator validation. Never execute the extra call, retain its prose, increase budgets or mark the game successful without the existing checks. Planning failures, cancellation and other provider/agent errors still fail normally.
- New 2D jobs install one of six embedded templates; guided 3D offers fps, exploration, transport, flight and space. Edits retain existing code. Both guided paths require compilation, build-bound browser startup and full gameplay checks. Free-code `three` supports startup only and explicitly leaves gameplay unverified. Failed/cancelled jobs preserve the last playable revision; at most three repair passes are shared by tool and orchestrator validation.
- Phaser 4.2.1 and Three.js 0.185.1 are embedded, pinned, offline runtimes. Generated games may not load CDNs, external APIs, remote assets, or AuraGo endpoints.
- Phaser phase guidance must distinguish dynamic and static Arcade bodies from their game objects. Moving paddles remain dynamic and immovable; body `setVelocity`/`setPosition` runtime errors receive bounded repair hints without weakening validation or increasing the repair budget.
- Dynamic gameplay checks use bounded `target` steps (move, aim, reach, interact, catch, avoid, select), driven only by normal keys/pointer input and read-only engine geometry. Keep roles/IDs independent of artwork. Never mutate actors, damage, randomness or counters to pass. Target reports require matching steps and physical effects; counter-only changes, missing targets, blocked routes and unsupported controls stay unavailable. Only observed contact/response contradictions fail. `player_distance` is derived from engine positions. Targeted input lives only in the injected preview driver; read-only 3D observations may ship with the common helper. Existing scenario/driver deadlines, lifecycle cleanup, export exclusion and publication gates remain binding.
- 3D reach/interact steps navigate active targets using live world geometry even
  outside the camera frustum; low FPS pickups leave the viewport before contact.
  Aim/select still require visible targets. Preserve collision/height checks,
  input-only control and actual physical-effect evidence.
- Asset detail examples must include executable preload/setup methods and preserve the template lifecycle. Asset creation rejects unloaded textures or missing frames; test binding requires a live controlled object assigned by setup. Missing Phaser textures cannot pass asset validation.
- New templates import and preload exact planned asset roles in common.ts. Every 2D template uses those roles through body(...,role), with uniformly fitted art and separate collision proxies; changes retain this wiring. The build guard rejects pack metadata as a texture key at the shared texture-manager boundary. Gameplay scenarios must allow actual travel time; hit counters represent collisions, including hits on durable targets.
- Additional plan scenarios are optional (0–8); the server always retains its eight 2D minimums and eight/ten guided 3D minimums (maximum 16 total checks). Check results include the executed finite steps, so repairs distinguish launch/actions from collisions/hits and ESC end from natural defeat. New GameScene templates reject update overrides at startup with hook-specific guidance; preserve common.ts lifecycle and sprite following.
- Script writes preflight literal built-in metadata imports with esbuild against complete project PNG/JSON pairs, resolving from the source file. Invalid imports leave source/preview and repair counts unchanged and return existing import paths; ordinary module writes retain their build-validation workflow. Arcade collider/overlap registration rejects wrapper records whose body is a GameObject; spawned collision objects belong in persistent groups. Repairs preserve passing sprite/input behavior and the accepted plan.
- The isolated Game Maker agent receives phase-specific schemas for `game_maker_project`, `game_maker_file`, `game_maker_asset`, and `game_maker_validate`. Planning exposes only reads, asset discovery and `set_design`; build/repair omit plan mutation. Embedded guidance is already active; redundant skill activation is not advertised. `invoke_tool`, generic filesystem/shell/Python/network tools, and uncurated Agent Skills must remain unavailable.
- Studio dispatch uses a server-owned job context, never a job inferred from session names or the globally active job. Omitted job IDs use this binding; mismatches fail. In bound runs only, a file content payload without an operation means write. Unbound calls retain explicit job/operation requirements and all policy/path/size gates remain enforced. Plan examples distinguish position metrics from primary-action counters.
- `set_design` accepts a compact closed object (base, objective, features, optional assets/settings/preserve/scene/mechanics/presentation/scenarios). The server fills the canonical plan and exact catalog metadata. Only misplaced root `outcomes`, `lives`, `blocks` and `events` normalize into `mechanics`; conflicting simultaneous values are rejected and all normal value/size checks still apply. Schemas and examples advertise the canonical nested form. Corrections retain omitted/null fields and replace supplied arrays; a flat mechanic correction retains omitted sibling helpers. The initial-plus-two-corrections budget is shared with legacy `set_plan`. Guided 3D resolves default roles and validates FPS rigs/bindings; user-selected models cannot disappear. Goal/speed/duration settings apply only to guided 3D. On a non-guided base, a correction that omits settings or supplies settings:null removes incompatible carried settings from the failed draft, including after a base change; other fields and guided settings retain their omission/null semantics. Explicit unsupported settings still reject with a design.settings correction that preserves the requested base. Published guided settings must not be inherited into a non-guided edit.
- File reads return bounded line ranges and a full-file SHA-256; native numeric and XML numeric-text `start_line`/`end_line` use the same finite nonnegative integer checks and range limits. Unique exact `replace` requires that digest. Writes return `written` and bounded `build.ok`/diagnostics separately. Preserve path/policy/import limits and reject stale edits before mutation. Imported 2D roles bind automatically in every template, with catalog-supported animation and separate collision proxies.
- The browser test driver waits for asynchronous GLB initialization within its existing deadline. Guided 3D observations come from live input/state, including FPS aiming/reload; no model-provided code is evaluated. Pointer lock does not grant same-origin access. `TestGuidedBrowser` is opt-in with `GAMEMAKER_GUIDED_BROWSER=1`; `TestGameMakerLiveEvaluation` runs explicitly selected providers on their own host, synthetic projects only, without exporting credentials.
- `SetPlanJSON` accepts a plan object or one JSON-string wrapper at the shared validation boundary. Malformed inner JSON returns its syntax position, not a string-to-struct error. All transports retain strict field/rule checks, size limits and the shared initial-plus-two-corrections budget; never guess missing plan content.
- System-managed Game Maker Agent Skills must match the complete embedded package. Startup self-heals their `SKILL.md` and registers the exact single-file binary package through the Skill Manager before exposing Game Maker; Guardian/SkillSpector latency on unrelated packages must not delay it. Optional scanner warnings can be replaced by verified binary provenance. Explicitly blocked packages, extra files/directories, symlinks, missing content and hash mismatches still block readiness. Runtime package hashes remain checked before use; bundle registration is internal and never accepts model/user-provided trust claims.
- Preview iframes omit `allow-same-origin`, use short-lived project/job-bound tokens, restrictive CSP including document `sandbox allow-scripts allow-pointer-lock` (so a top-level tab cannot ride the admin session), external-connect blocking, and a source/channel-validated bridge for bounded diagnostics and finite test inputs.
- Preview tests accept only finite key/pointer/wait/observe steps, never model JavaScript. The authenticated parent forwards bounded numeric observations and at most two bounded PNGs. The server compares evidence; missing observations never pass. Runs are bound to a build and preview token, last at most 60 seconds, and reset gameplay afterwards. Optional image review uses only a confirmed image-capable selected model, is tool-free and cannot override technical checks.
- Phase-specific verified embedded skills use `TrustedPromptAddenda`; human intent, model plans, project files and diagnostics stay separate untrusted data. Planning text is never streamed/persisted as chat; final player prose is held until publication. `.aurago/validation-report.json` binds results to the compiled bundle hash and is revisioned but excluded from ZIP export.
- Image and music generation are optional project capabilities. Generator failure, disabled configuration, or exhausted budget must return a visible procedural fallback without claiming unsupported 3D model generation.
- Revision blobs are SHA-256 addressed and deduplicated. Restore creates a new revision; export excludes tokens, staging, revision metadata, and AuraGo state while including source, output, local runtimes, assets, and third-party notices.
- ZIP export reads one published revision from the blob store, verifies sizes/hashes and required entry files, and retains that revision's runtime files. Never export mutable workspace edits alongside old compiled output. Finish a temporary archive before committing HTTP download headers; export failures must not become successful partial ZIPs. Include standalone HTTP-server instructions; file:// is not a supported launch path. Check extracted 2D/3D games without the preview boot/driver, including subdirectory hosting, imported assets and audio.

### Game Maker tool validation contract

- Game Maker `BuildJob` compiles without waiting; `ValidateJob` additionally
  requires a build-bound browser startup check through the authenticated Studio
  parent. Runtime errors feed the bounded repair loop; missing feedback prevents
  publication. Diagnostics stay bounded, untrusted data and never become trusted
  prompt instructions. Only the server boot's visible-canvas report establishes
  readiness; engine console errors also fail validation. Procedural SVG assets
  return `.svg` paths, never SVG bytes mislabeled as PNG. A passed startup check
  does not certify all gameplay.

### Game Maker sprite library contract

- The same catalog includes `kind: model3d` pack `aurago-low-poly@1.0.0` with
  220 original MIT Blender models. `assets/game-maker-low-poly/AGENTS.md` owns
  editable sources, generators and acceptance fixtures. Runtime GLBs, previews,
  metadata and licenses together must stay under 100 MiB uncompressed.
- Model selection uses `model_asset_ids` (1–64) and plan schema 2 with metres,
  metric scale and 3D colliders. Import only explicit IDs after plan acceptance,
  including declared shared animation dependencies; never overwrite edited copies.
  Sprite plan v1 and all eighteen sprite packs remain compatible.
- Three.js stays at 0.185.1. Rebuild the local GLTFLoader/SkeletonUtils/OrbitControls
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

- `internal/gamemaker/asset_packs/` owns eighteen locally packaged 10×10 RGBA sheets (64px
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

## Verification

- `go test ./internal/gamemaker` and focused agent/server tests.
- `GAMEMAKER_EXPERIENCE_BROWSER=1`: normal-input contact, feedback, checkpoint,
  pause, stage/result/restart and narrow-screen controls in exported 2D/3D fixtures.
- `GAMEMAKER_TARGET_BROWSER=1`: positive and negative read-only target evidence.
  `TestFPSPickupBrowser` covers normal input, target driving and pickup reset;
  `TestTargetControlBrowser/fps_ground_` covers low/behind, blocked, elevated and
  counter-only targets. `TestRuntimeContextIdentifiesInstalledPickupCounter`
  checks repair guidance against the actual installed helper.
- `GAMEMAKER_GUIDED_BROWSER=1`: starter engine/lifecycle browser checks.
- `GAMEMAKER_PREVIEW_BROWSER=1`: hidden/resumed previews, delayed layout, invalid
  canvases and loading-HUD recovery in Chrome.
- `GAMEMAKER_OPTIMIZATION_BROWSER=1`: `TestFPSCoverBrowser` checks actual enemy
  damage and player shots with/without cover using normal browser input.
- `TestSourceBatchPreflightSearchAndRuntime`, `TestPrivateBuildSourceDiagnostics`,
  `TestValidationReuseRequiresExactCurrentEvidence`, and
  `TestDesignReportsIndependentCorrectionsWithinBudget` cover agent assistance.
- Server `TestGameStarterProviderSourceFormats`,
  `TestGameStarterWrappedSourceRequiresExactSingleWrite`,
  `TestGameMakerUnchangedStarterGetsBoundedCodeRecovery` and
  `TestGameMakerSourceStreamRecovery` cover source recovery and its write boundary.
- `node scripts/build-game-maker-presentation.js --check` and
  `node scripts/test-game-maker-presentation-evidence.mjs`.
- Package changed runtimes with the matching resource flags and verify `--check-assets`.

## Child DOX Index

- `asset_packs/AGENTS.md`: original local content, metadata and reproducible packing.
