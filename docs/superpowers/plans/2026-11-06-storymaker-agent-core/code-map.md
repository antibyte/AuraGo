# Code maps for the Storymaker agent core

Read-only analysis on 2026-11-06 at e14800937. Verify a line before relying on it.

---

<!-- Game Maker runner -->

# Game Maker runner: map for the Storymaker agent core

Line numbers are from reading the working tree on 2026-11-06. The repo has uncommitted edits from other sessions, so re-check before citing them.

## 0. Corrections to the brief

- `internal/server/game_maker_recovery_nudge.go` does not exist. The recovery logic is spread over the places listed in section 7.
- `game_maker_continuation.go` holds `gameConversation`, not the nudge logic.
- The spec citations were not re-checked. Only the code cited below was verified.

## 1. Queue, start, cancel, timeout, persistence

**Start (HTTP).** `handleGameMakerStartJob` (`internal/server/game_maker_handlers.go:275-326`) checks desktop write scope, resolves provider and model, and blocks models below the 16384-token floor (`:306-315`). It then records the small-model decision (`:318`) and calls `s.GameMaker.StartJob`. The response is 202 with the job JSON.

**Queue (`internal/gamemaker/service.go`).**
- `StartJob` (`:437-588`) validates policy, skill readiness, asset packs and project. It builds a `Job` with status and phase `queued` (`:529-546`).
- Only one job runs at a time. If `s.activeJobID != ""`, it returns `ErrBusy` (`:548-551`).
- It registers the cancel func in `s.jobCancels` (`:553-558`).
- It inserts the `gm_jobs` row (`:574-581`), appends the user message, emits `job_status` queued (`:585`), then runs `go s.executeJob(...)` (`:586`). The HTTP request returns before the job runs.

**Execution.** `executeJob` (`service.go:590-858`):
- Stages a working copy under `s.stagingDir/<jobID>`.
- Runs `RunGameMakerJob` with stage `planning` (up to 3 attempts, `:688-722`), then `building` (`:797`), then `validating`, `repair` (up to `repairLimit`, `:829`), `polishing` (visual review, `:847`), and `publishValidated`.
- A `defer` clears per-job maps and calls `releaseJob` (`:595-611`).

**Timeout and extension.** `jobContext` (`internal/gamemaker/job_timeout.go:15-31`) calls `newGameJobContext` (`:33-57`):
- The base deadline is `opts.JobTimeout`.
- A grace period of `min(budget/2, 15min)` is added to the hard deadline.
- At the base deadline a one-time extension runs `recentCompiledProgress`. It requires an accepted plan, a compiled preview, and a recent `file_changed` event. If it passes, the job gets one more grace window.

**Cancel.** `POST /api/game-maker/jobs/{id}/cancel` (`game_maker_handlers.go:328-348`) calls `CancelJob` (`service.go:877-886`), which invokes the stored cancel func. `terminateJob` (`:939`) then routes by `ctx.Err()`:
- `cancelledJob` (`:947`) with code `timeout` for `DeadlineExceeded` and `cancelled` otherwise.
- `failJob` (`:917`) for every other error.

Both write the `gm_jobs` columns, release the job, and emit `job_status`.

**Per-call LLM timeout.** `RunGameMakerJob` wraps the context with `llm.WithStreamAttemptTimeout(..., LLMTimeoutSeconds)` (`internal/server/game_maker_service.go:801`).

**Persistence.**
- Job state lives in `gm_jobs`. Each phase change goes through `updateJobPhase` (`service.go:899-915`).
- Events go to `gm_events` through `emit` (`internal/gamemaker/events.go:9-31`).
- Agent continuation goes to a separate store through `SaveAgentConversation` and `LoadAgentConversation` (`internal/gamemaker/continuation.go:24`, `:48`).

**Restart.** `storage.go:106-109` runs at schema init and sets every job in `queued/planning/building/validating/polishing` to `interrupted` with the error "Server restarted while the job was active". Nothing auto-resumes. The user can resume with `StartJob` (`Resume` flag or a continuation prompt). `resumableJob` (`continuation.go:102-129`) finds the latest job with status `failed`, `cancelled` or `interrupted` and the same base revision whose staging directory still exists. `executeJob` then copies that directory back (`service.go:619-625`).

## 2. SSE and event persistence

- Routes: `registerGameMakerRoutes` (`game_maker_handlers.go:19-28`). The events handler is called from `handleGameMakerProjectPath` (`:148`).
- `handleGameMakerEvents` (`:350-421`) sets `text/event-stream`. It replays from `Last-Event-ID` or `?after=`, in batches of 500 from `EventsAfter`. It then sends `stream_ready`, which is not stored and has no id. After that it waits on a subscriber channel (buffer 32, non-blocking sends in `emit`) and sends a 15 s heartbeat.
- DB replay is the source of truth. A dropped wakeup on a full channel is recovered by the next replay.
- `EmitAgentEvent` (`events.go:88-96`) accepts only a whitelist: `text_delta`, `skill_activation`, `file_changed`, `asset_changed`, `diagnostic`, `phase`, `token_usage`, `prompt_usage`, `tool_call`, `tool_result`, `model_progress`, `visual_progress`, `visual_result`. This whitelist is game-specific.

## 3. Prose withheld until the outcome

- `gameMakerBroker` (`game_maker_service.go:861-963`) stores the model's text deltas in `response` (`SendLLMStreamDelta`, `:911-920`). No SSE text event is emitted for them, even though `text_delta` is whitelisted.
- The broker emits only `model_progress` (statuses `waiting`, `receiving`, `retrying`, `recovering`, throttled per 10 s) and `tool_call`. The `tool_call` payload carries a tool name only for `game_maker_project`, `game_maker_file`, `game_maker_asset`, `game_maker_validate` and `activate_agent_skill`. Arguments and text are never retained (`:891-899`).
- After a completed building or repair stage, the answer is stored with `r.service.HoldAgentSummary` (`game_maker_service.go:842-848`, implementation in `game_plan.go:728`). The planning stage does not store one. I did not trace where the held summary is later released (unverified).

## 4. The LLM call (`RunGameMakerJob`, `game_maker_service.go:595-850`)

**Provider and model pinning.**
- The job's `ProviderID` is resolved with `cfg.FindProvider` and `llm.TaskProviderConfig` (`:610-618`). It must pass `config.ProviderUsageAllowsGeneralChat`.
- A model override is applied (`:620-622`).
- `applyGameMakerStreamIdle` runs (`:626`).
- If a provider or model is pinned, `cfg.FallbackLLM.Enabled = false` and a dedicated client is built with `llm.NewClientFromProviderWithConfig` (`:627-631`). Without this the global failover client overwrites the model.

**Routing inside the runner.**
- `visual` goes to `reviewGameImages` (`:632`).
- `building` with a plan and a ready starter goes to `implementGameStarter` (`:639-647`), which is the tool-free single-completion path.
- `repair` on an unchanged starter also goes there (`:653`).
- Everything else reaches the tool loop below.

**Tool loop setup.**
- `cfg.LLM.UseNativeFunctions = true` (`:656`). This is overridden by the tooling policy (section 5).
- Temperature comes from `gameMakerPhaseTemperature` (`:441-461`): planning 0.6, building 0.2, clamped to 0.05–1.5.
- The prompt profile is `gameMakerPromptProfile(stage, dimension, variant...)` (`game_maker_prompt_profile.go:103`), with the `compact` and `voxel` variants.
- The context packet is built from `compactGameMakerContext`, `RuntimeContext`, `ValidationPlan`, `gameMakerCurrentSourcesWithin` (`:520`, source windows 24000 bytes, or 12000 for compact), `PreviousJobRequests` and `gameConversation` (`:711`).
- Context fields include `budget`, `catalog` (planning only), `edit_targets` (adapt mode) and `omitted_sources`.

**RunConfig (`:721-759`).**
- `buildDesktopRunConfigForSession(s, &cfg, client, sessionID, "game_maker")` is the base (`desktop_handlers_chat.go:878`).
- `PreparedPrompt = profile` and `NativeToolSchemas = profile.Tools()` (`:736-738`).
- `AllowedTools` is the list of schema names (`:739-742`). A non-nil list turns on `ToolScopeRestricted` (`agent_loop.go:302`).
- `AllowedAgentSkills = gamemaker.CuratedSkillNames()` (`:751`).
- Other flags set here: `PreserveReasoning`, `StableSystemPrompt`, `RequireCompleteStream`, `RetryStreamIdle`, `StreamThroughput = &gameMakerStreamThroughput`, `ToolCallLimit`, `SuppressTurnSideEffects`, `RunComplete`, `IsMission`.
- `UsageObserver = gameUsageObserver(run)` (`game_maker_usage.go:16`).
- `Checkpoint = checkpoint` from `gameConversation`.
- Request: `Stream: true`. Planning sets `ToolChoice: "required"` (`:775`).
- Call: `agent.ExecuteAgentLoop(callCtx, req, runCfg, true, broker)` (`:802`).

**Recovery branches after the call (`:803-841`).**
- `IncompleteCompletionError` with reasoning-only length truncation (`:805-812`).
- `IsToolLimitFinalResponseInvalid` (`:813-818`).
- Empty response: one bounded tool-free retry for the unchanged starter (`:824-831`), or validate the source if it was saved (`:832-839`).

## 5. Native versus text tool mode

- `resolveModelCapabilities` (`internal/agent/tooling_policy.go:92-156`) sets `DisableNativeFunctionCalling = isGLMFamily` (`:154`). `isGLMFamily` (`:132-139`) matches `glm-`, `/glm-`, `zhipuai/` and `abab`, plus MiniMax names except M2.7, M3, M1 and `/text-`.
- `buildToolingPolicy` forces `useNativeFunctions = false` for those models (`:182-188`).
- `reconcileToolPromptModeWithSchemas` (`agent_loop_init.go:21-40`) turns native off and sets `IsTextModeModel` when there are zero schemas.
- `IsTextModeModel` is set at `tooling_policy.go:673`.
- Schema assembly: `allSchemas = filterSchemasByAllowedTools(...)` (`agent_loop_init.go:336-339`). Native schemas come from `runCfg.PreparedPrompt.Tools()`, or `NativeToolSchemas` when there is no profile.
- **Gap.** I did not locate the exact line where text mode withholds `req.Tools` from the request. The forensics claim that GLM and MiniMax "get no tool schemas" is therefore only partly verified. The flag path is verified, and the schema path is not.
- Text-call parsing for `minimax:tool_call` and GLM JSON forms is in `agent_parse.go:941-971`. Retry and marker handling is in `agent_loop_retry.go:95` and `:395`.
- The minimal loop rejects textual tool syntax. It sends one correction and does not execute it (`minimal_loop.go:326-349`). If that fails, it returns `ErrUnexpectedToolCallText`.

## 6. Tool loop, DispatchContext and scope

**Loop.** `ExecuteAgentLoop` (`agent_loop.go:314`) is the full loop used for building and repair. `ExecuteMinimalLoop` (`minimal_loop.go:83-465`) is the lean single-round loop used for the tool-free starter. Its signature is:

`ExecuteMinimalLoop(ctx, client llm.ChatClient, model, systemPrompt, userPrompt string, tools []openai.Tool, dispatchCtx *DispatchContext, history []openai.ChatCompletionMessage, logger *slog.Logger, opts *MinimalLoopOptions) (MinimalLoopResult, []openai.ChatCompletionMessage, error)`

**DispatchContext for the starter** (`game_maker_implementation_stream.go:80`): `Cfg`, `Guardian`, `BudgetTracker`, `SessionID`, `MessageSource: "game_maker"`, `ToolScopeRestricted: true`, `AllowedTools: map[string]struct{}{}` (empty, so no tools). `MaxToolRounds: 0` means no tools.

**DispatchContext fields** (`internal/agent/dispatch_context.go:60-107`): `ToolScopeRestricted bool` (`:103`) and `AllowedTools map[string]struct{}` (`:102`). `ExecutionHooks` and `Ctx` are also used by Game Maker code.

**Scope enforcement.**
- Schema filter: `filterSchemasByAllowedTools` (`tool_scope.go:27`), called from `minimal_loop.go:162` and `agent_loop_init.go:339`.
- Dispatch check: `dispatchToolAllowed` (`tool_scope.go:56`).
- Private Game Maker dispatch: `action_policy.go:321`. It applies when `ToolScopeRestricted` is on and the action starts with `game_maker_` or `cyd_creator_`.

**Refusals list the allowed tools.** `toolScopeDeniedFor` (`tool_scope.go:121-130`) and `scopeDeniedRefusal` (`:133-143`) add `allowed_tools` and `hint`:
- `scopeRecoveryFields` (`:83-98`).
- `allowedToolList` (`:103-116`), which sorts, caps the list at 12 entries, and appends `…`.
- The hint names the owning tool when an operation is called outside its scope.

**Operation names called as tools.** `canonicalizeGameMakerOperationCall` (`internal/agent/canonical_action.go:170`) reroutes calls when a job ID is bound to the context (`gamemaker.JobIDFromContext`). The owner map is `gameMakerOperationOwners` (`canonical_action.go:140-148`, `sync.OnceValue`). It is built from `gameMakerToolOperations()`. `canonicalNativeAlias` (`:12-27`) covers only equivalence aliases, not git aliases.

**Phase-scoped schemas.** `GameMakerPhaseToolSchemas(stage, dimension, variant...)` (`internal/agent/native_tools_game_maker.go:291`):
- Strips `job_id` from every schema, since Studio binds identity in the context.
- During planning it removes `game_maker_validate`, narrows `game_maker_file` to read and search, narrows `game_maker_asset` to search and describe, and narrows `game_maker_project` to the planning operations.
- The compact variant is applied through `compactGameDesignSchema` (`:277`).

## 7. Event stream, budgets, nudge, small models

**Budgets.**
- Tool call limit: `gameMakerToolCallLimit(cfg.CircuitBreaker.MaxToolCalls)` (`game_maker_service.go:586-593`). It adds a quarter with a floor of 40.
- Context budget: `gameMakerBudget` (`:407-424`) reports `tool_calls`, `seconds_remaining` (rounded to 30 s) and a stage guidance string.
- Plan budget (`internal/gamemaker/plan_guard.go:13-61`): `planChargedLimit = 3`, `planCallLimit = 5`, `planPreviewLimit = 3`. A failure is charged only when it does not reduce the independent issue count.
- Token charge category: `ExecuteAgentLoop` charges the category `"agent"` (`agent_loop.go:315`). The starter's `ExecuteMinimalLoop` uses `opts.BudgetCategory = "game_maker"` (`game_maker_implementation_stream.go:81`). The two paths therefore charge different categories. This is observed, not a deliberate design I could confirm.
- Usage events: `gameUsageObserver` emits `prompt_usage` and an info `diagnostic` (`game_maker_usage.go:16-36`). `SendTokenUpdate` emits `token_usage` (`game_maker_service.go:945-955`).

**Stream idle (forensics fix).**
- `gameMakerStreamIdleSeconds = 120` (`game_maker_service.go:475`).
- `applyGameMakerStreamIdle` (`:479-483`) raises `LLMStreamChunkTimeoutSeconds` to 120 and never lowers it.
- Called at `:626` for the job's config copy.
- Consumed by `minimalLoopStreamText` through `minimal_loop.go:272`.
- A test exists in `game_maker_stream_idle_test.go`.
- `gameMakerStreamThroughput` (`:468`) lets slow streams extend the deadline.

**Nudge and recovery.** The logic is spread across these places:
- The textual-tool correction message (`minimal_loop.go:341`).
- Reasoning-only length recovery (`game_maker_service.go:805-812`).
- Tool-limit recovery (`:813-818`).
- Empty-response recovery (`:824-839`).
- Starter retry, at most once, with a rewritten prompt for each failure class (`game_maker_implementation_stream.go:119-148`).
- The `recovering` and `retrying` progress statuses.
- Agent recovery classifiers in `internal/agent/recovery_classifier.go:413` and `recovery_session.go:193`. I did not read these in full.

**Small-model detection.** `game_maker_model_limits.go`:
- Thresholds (`:19-22`): small means context below 65536 or max output below 16384.
- Unknown models are never small (`:28-43`).
- The context floor is 16384 (`:14`, `:93-102`), with an explicit opt-in.
- The compact effects are the `compact` prompt variant, source window 12000 (`game_maker_service.go:511`), API reference 2000 (`:514`), and adapt building mode.
- Client-facing limits are at `handleGameMakerModelLimits` (`:104-138`).

## 8. Reuse, copy, do not copy

**Reuse as is (generic):**
- `agent.ExecuteAgentLoop`, `agent.ExecuteMinimalLoop`, `agent.RunConfig`, `agent.DispatchContext`, `agent.PreparedPromptProfile`, `agent.NewPreparedPromptProfile`, `agent.IncompleteCompletionError`, `agent.SanitizeToolMessages`, `security.Scrub`.
- `llm.NewClientFromProviderWithConfig`, `llm.TaskProviderConfig`, `config.ProviderUsageAllowsGeneralChat`, `llm.WithStreamAttemptTimeout`, `llm.NewThroughputDeadline`, `llm.ThroughputPolicy`.
- `buildDesktopRunConfigForSession` (`desktop_handlers_chat.go:878`), which takes a generic `messageSource` string.
- `newGameJobContext(budget, extend)` (`job_timeout.go:33`). It is generic: a timer, a cancel cause, and a single extension hook.
- The `ErrBusy` single-job guard and the `jobCancels` map pattern (`service.go:548-558`, `:877-886`).
- The restart-interrupted UPDATE (`storage.go:106-109`).
- The replay-then-subscribe SSE loop (`game_maker_handlers.go:350-421`). The event type whitelist must change.

**Copy the pattern (game-specific names):**
- The `RunGameMakerJob` sequence (`:595-850`), trimmed. This covers the config copy, provider and model pinning, disabling fallback when pinned, the native flag, the tool allow-list from schemas, the RunConfig flags and the ToolCallLimit.
- The answer-withheld pattern: buffer deltas, emit only progress and tool names, store the answer after completion.
- The empty-response guard and the reasoning-only checks.
- The `gameConversation` checkpoint, if the runner needs a resumable, scrubbed conversation archive (`game_maker_continuation.go:28-119`). It is tied to `gamemaker.Service` storage.
- The starter's bounded retry (`game_maker_implementation_stream.go:20-150`), if the runner makes a tool-free call.
- The scope-refusal fields (`tool_scope.go:83-143`). These are generic.

**Do not copy:**
- `gameMakerBroker` as is. Its event names and tool allow-list are game-specific.
- `EmitAgentEvent`'s whitelist, which names Game Maker event types.
- `executeJob`, staging, validation, publish, visual review, starter envelope handling and the plan budget. All are specific to a game build.
- `gameMakerPromptProfile`, `GameMakerPhaseToolSchemas` and `canonicalizeGameMakerOperationCall`. These are tied to Game Maker operation names and owners. A Storymaker runner needs its own owner map and a phase scope.
- The `gameMakerStreamThroughput` numbers. The 120 s idle and throughput constants are game-tuned.

**Tied to Game Maker types (not generic):** `gamemaker.JobRun`, `gamemaker.Runner` (`internal/gamemaker/types.go:232-253`), `gamemaker.Job`, `gamemaker.Service`, `configureGameKnowledge` (`project_knowledge.go:68`), `gameUsageObserver`.

## 9. Unverified

- Where text mode withholds `req.Tools` from the request (section 5).
- Where `HoldAgentSummary` is released after the stage (section 3).
- Behaviour of the agent recovery classifiers (`recovery_classifier.go`, `recovery_session.go`).
- The exact per-stage budget charged for tokens, beyond the category difference noted in section 7.

---

<!-- Tool wiring -->

## Tool-wiring map for storymaker_* job tools and the chat tool `storymaker`

Read-only findings from the working tree at `C:/Users/Andi/Documents/repo/AuraGo`. Line numbers are as read; other sessions have uncommitted edits there. Nothing was run, so the test claims are from reading the test code.

### 1. The import-cycle question

`internal/agent` never imports `internal/server`. The server calls the agent, so the flow is one-way. Three mechanisms cross the boundary:

1. **Published service in a lower package.** Used by retro_games.
   - `internal/tools/retro_games.go:41` `SetRetroGamesService(svc *retrogames.Service)`, `:48` `WithdrawRetroGamesService`, `:66` `RetroGamesAvailable() bool`, `:83` `ExecuteRetroGames(ctx, cfg, req RetroGamesRequest) VirtualDesktopExecution`.
   - The server publishes it at `internal/server/retro_games_tool.go:9-17` (`publishRetroGamesTool`, `withdrawRetroGamesTool`), called from `internal/server/server.go:2004-2005` and withdrawn at `:990` and `:1045`.
2. **Package default service plus an interface implemented by the server.** Used by Game Maker.
   - `internal/gamemaker/service.go:77` `SetDefaultService(*Service)`, `:83` `DefaultService() *Service`.
   - `internal/gamemaker/types.go:251` `type Runner interface { RunGameMakerJob(ctx context.Context, run JobRun) error }`.
   - The server implements it: `internal/server/game_maker_service.go:120` (`gameMakerAgentRunner`), `:595` (`RunGameMakerJob`). `initGameMaker` (`:47-87`) calls `service.SetRunner(...)` at `:77` and `gamemaker.SetDefaultService(service)` at `:79`. `server.go:1063` clears it on shutdown.
3. **Trusted callbacks on the run config.** Used by Detective and cyd_creator.
   - `internal/agent/execution_hooks.go:15` `ExecutionHooks` (func fields: `OnAcquire`, `BeforeIteration`, `BeforeRequest`, `BeforeTool`, `HandleTool`, `AfterTool`, `ObserveResult`, `AfterResponse`).
   - Set by the server, for example `internal/server/detective_service.go:251`.

**Job identity** comes from a context key set server-side, never from model arguments: `internal/gamemaker/job_context.go:9` `WithJobContext(ctx, jobID)`, `:13` `JobIDFromContext(ctx) string`. The server sets it at `game_maker_service.go:801`. The agent reads it at `agent_dispatch_game_maker.go:441` and `canonical_action.go:171`.

`internal/storymaker` does not import `internal/agent` or `internal/server`. Its non-test imports are `desktop`, `fileutil`, `security`, `videostudio`, `mediabins` and itself. So the agent can import it directly, as it imports gamemaker. No `DefaultService`-style hook exists there yet.

### 2. Exact Game Maker mechanism (how a tool reaches the service)

- Schemas: `internal/agent/native_tools_game_maker.go:11` `appendGameMakerToolSchemas(tools []openai.Tool, ff ToolFeatureFlags) []openai.Tool` is gated on `ff.GameMakerEnabled`. It defines `game_maker_project`, `_file`, `_asset`, `_validate`.
- Dispatch: `internal/agent/agent_dispatch_exec.go:228-230` routes the four names to `dispatchGameMaker(ctx, tc, dc)`, defined at `agent_dispatch_game_maker.go:430`. It calls `gamemaker.DefaultService()` at `:436` and returns "Game Maker service is unavailable" when nil.
- Job scope: the server builds the run at `game_maker_service.go:721-758`. It sets `runCfg.NativeToolSchemas = profile.Tools()` (`:738`), then `runCfg.AllowedTools` from those names (`:739-742`). `agent_loop.go:302` derives `ToolScopeRestricted: s.runCfg.AllowedTools != nil`.
- The Game Maker run does **not** set `ExecutionHooks`. A search for `ExecutionHooks` assignments in `game_maker*` files found none.
- Prepared profiles: `internal/agent/native_tools_game_maker.go:291` `GameMakerPhaseToolSchemas(stage, dimension, variant...)`, used by `internal/server/game_maker_prompt_profile.go:128` and `:152`.

### 3. Dispatch order (must be satisfied by a new tool)

`internal/agent/agent.go:1271` `dispatchInner(ctx, tc, dc)`:
- `:1282` `prepareToolCall` (this is where `canonicalizeLeafCall` runs, `canonical_action.go:303`)
- `:1286` `checkDispatchActionPolicy` (`action_policy.go:283`)
- `:1291` `dispatchToolAllowed` (scope, `tool_scope.go`/`action_policy.go`)
- `:1298` `roleToolRestriction`, `:1301` `checkLearningAction`, `:1306` `startDispatchedToolAction`
- `:1311` `toolRuntimeUnavailable`
- `:1315-1329` hooks (`BeforeTool`, `HandleTool`)
- `:1339` `dispatchExec` → `:1342` `dispatchComm` → `:1345` `dispatchServices` → `:1348` `dispatchInfra`

`dispatchPlatform` (`dispatch_platform.go:284`) is called from one of these. retro_games is at `:307-308`.

**Catalog gate.** `action_policy.go:315-322` computes `enabled` from `dispatchCatalogSchemas(dc)` (`discovery_run.go:50-71`). That function rebuilds the schemas from config flags via `buildToolFeatureFlags` and then filters by `AllowedTools`. `private` (`:321`) is true only if `dc.ExecutionHooks != nil` and the name has the prefix `game_maker_` or `cyd_creator_`. Otherwise `missingCatalog` (`:322`) is true and the native check at `:326-334` refuses the call unless the tool is runtime-available and configured. **Consequence:** a job-scoped `storymaker_*` tool must be enabled by its config flag, so it appears in `dispatchCatalogSchemas`. Adding its name to `private` alone is not enough if no hooks are set.

### 4. Full list of files and edit shapes

**Chat tool `storymaker`** (mirrors retro_games)

| Concern | File:line | Edit shape |
|---|---|---|
| Feature flag field | `internal/agent/native_tools.go:130-144` (ToolFeatureFlags) | add `StorymakerEnabled bool` |
| Schema | `internal/agent/native_tools_retro_games.go:8` `retroGamesSchema() openai.Tool` | new `storymakerSchema()` using `tool()`, `schema()`, `prop()`, `operationProperty()` |
| Schema wiring | `internal/agent/native_tools_integrations.go:1089-1091` | `if ff.StorymakerEnabled { tools = append(...) }`. The enclosing function was not read. |
| Full-flag set | `internal/agent/native_tools_registry.go:71-111` `allBuiltinToolFeatureFlags()` (retro at `:109`) | add `StorymakerEnabled: true` |
| Cache key | `internal/agent/native_tools_registry.go:124` `Key()` (retro at `:144`) | add `appendToolFeatureKeyPart("storymaker", ...)` |
| Config flags | `internal/agent/tooling_policy.go:331` `buildToolFlagsFromConfig` (retro at `:380`) | `StorymakerEnabled: cfg.Storymaker.Enabled && cfg.Storymaker.AgentAccess` (field does not exist yet) |
| Runtime availability | `tooling_policy.go:498-539` `resolveToolFeatureState` (retro at `:523`) | `toolFlags.StorymakerEnabled = ... && tools.StorymakerAvailable()` (publish-style, if a service is published) |
| Prompt flags | `internal/agent/tooling_policy.go:541` `buildPromptContextFlags` (retro at `:587`) | copy the flag |
| Prompt flag struct | `internal/prompts/builder.go:203` `ContextFlags.RetroGamesEnabled` | add field |
| Guide table | `internal/prompts/builder.go:2720` (`{"retro_games", "...", func(f *ContextFlags) bool {...}}`) | add entry |
| Prompt-cache tool list | `internal/agent/system_prompt_cache.go:483-485` | add `if flags.StorymakerEnabled { tools = append(tools, "storymaker") }` |
| Dispatch case | `internal/agent/dispatch_platform.go:307-308` | `case "storymaker": return dispatchStorymaker(ctx, tc, dc)`. File must be in the test list (see below). |
| Dispatch body | `internal/agent/retro_games_tool.go:13-29` | copy shape: decode, call service, publish `virtual_desktop_event` via `dc.Broker.SendJSON` |
| Arg decoder | `internal/agent/tool_args_retro_games.go:7` | copy shape |
| Intent trigger | `internal/agent/agent_loop_helpers.go:706-736` (`adaptiveAdditiveToolsForQuery`, `isRetroGamesIntent`) | new `isStorymakerIntent` with word-start matching, plus an append in `adaptiveAdditiveToolsForQuery` |
| Non-displacing | `internal/prompts/tool_manual_bindings.go:14` `NonDisplacingTools`, `:25-36` `nonDisplacingToolEnabled` | optional. retro_games and local_wikipedia are listed; `agent_loop_helpers.go:558` makes them `adaptiveIntentOnlyTools` |
| Categories | `internal/agent/tool_categories.go:230` (media list), `:61-64` (discover aliases in `discoverToolNameAliases`, `:46-65`) | add `{"storymaker", "..."}` to `media` or `system`; optional aliases |
| Family | `internal/agent/tool_family.go:31` (name list, `retro_games` in the media condition) | add the name if it should classify as media. Test: `retro_games_tool_test.go:173-180` requires `media` from both `classifyToolFamily` and `ToolCategoryForName`. |
| Search terms | `internal/agent/tool_search_terms/media_communication.yaml:74-76` | add `storymaker:` with `en:` and `de:` lists |
| Manual | `prompts/tools_manuals/retro_games.md` | add `prompts/tools_manuals/storymaker.md` (ID is the name, `ToolManualID` default at `tool_manual_bindings.go:80`) |
| Config struct | `internal/config/config_types.go:2192` already has `Storymaker StorymakerConfig` | add `AgentAccess` to `StorymakerConfig` (`internal/config/storymaker.go:12`) or a separate struct |
| Config default | `internal/config/config.go:567-569` (sets `Storymaker.Enabled = true`) | add `AgentAccess` default |
| Template | `config_template.yaml:2375` (storymaker block) | add the agent switch |
| Save path | `internal/config/config.go:3028` `Save`, Game Maker entries `:3275-3291` | retro_games and local_wikipedia are **absent** from the Save list. Add the Storymaker switch if a UI save must persist it. |
| Server | `internal/server/server.go:2004-2005` (retro init and publish) | copy only if a published service is used |

**Job-scoped `storymaker_*` tools** (mirrors game_maker)

| Concern | File:line | Edit shape |
|---|---|---|
| Schemas | new file, mirror `native_tools_game_maker.go:11` (`appendGameMakerToolSchemas`) | `appendStorymakerToolSchemas(tools, ff)`, gated on a new flag |
| Schema registration | `native_tools_registry.go:38-56` `builtinToolSchemas` (game_maker call at `:51`) | add a call |
| Flags | `native_tools.go:130`, `native_tools_registry.go:91` (game_maker in allBuiltin), `:223` (`Key()`), `tooling_policy.go:455` (`GameMakerEnabled: cfg.GameMaker.Enabled`) | add `StorymakerEnabled` at all four |
| Dispatch | `agent_dispatch_exec.go:228-230` (game_maker case in `dispatchExec`) | add a `case "storymaker_..."` group and a new function. Also add the file to `tool_registry_validation_test.go:16-29` if it is a new file. |
| Service | new `internal/storymaker` default service, mirroring `gamemaker/service.go:73-86` | `SetDefaultService` / `DefaultService` with a `sync.RWMutex` |
| Job context | mirror `gamemaker/job_context.go:9-16` | `WithJobContext` / `JobIDFromContext` (its own key type) |
| Runner | mirror `gamemaker/types.go:251` (`Runner`) and `server/game_maker_service.go:595` | only if a server-side job runner is needed |
| Canonicalization | `canonical_action.go:122-230` (operation-owner rerouting) | only for tools with an `operation` enum. Game Maker's rerouting is keyed to `game_maker_` names at `:125` and `:140`. |
| Metadata | `internal/agent/tool_registry_metadata.go:47-50` | add `builtinToolMetadata` entries (`Family`, `VisibilityClass: ToolVisibilityAdaptive`, `Dispatcher: "exec"`, `RequiresWrite` as needed). retro_games has none. The test that enforces these was not read (`tool_registry_metadata_test.go:7`). |
| Categories | `tool_categories.go:103-106` (game_maker entries in `system`) | add entries |
| Search terms | `tool_search_terms/system.yaml:32-43` (game_maker) | add entries |
| Scope hints | `tool_scope.go:108` `strings.HasPrefix(name, "game_maker_")` in `allowedToolList` | add the prefix or the Studio hint is not produced. The scope itself is unaffected. |
| Private route | `action_policy.go:321` (prefix list `game_maker_`, `cyd_creator_`) | add the prefix only if the run sets `ExecutionHooks`. Game Maker does not. |
| Manual | none for game_maker. `ToolManualAbsenceReason` at `tool_manual_bindings.go:85-86` exempts them. | add the manual, or add the exemption with the phase-specific reason |
| Server run | `game_maker_service.go:721-758` (run config) | copy the shape: `NativeToolSchemas`, `AllowedTools`, `ToolScopeRestricted` implied, `RunComplete`, `ToolCallLimit` |

### 5. Test and audit guards

These fail when a link is missing. Most iterate `allBuiltinToolFeatureFlags()`, so a tool that is not in that function is silently skipped by all of them.

- **Manual binding:** `internal/agent/progressive_disclosure_contract_test.go:137-147` `TestBuiltinManualBindingsAreValidOrExplicitlyExplained`. For each schema from `BuildNativeToolSchemas(..., allBuiltinToolFeatureFlags(), nil)`, unless `ToolManualAbsenceReason` is non-empty, `prompts.ReadToolGuideFull(manualPathFor(...))` must succeed. **Yes, a schema added without a manual fails this test**, but only if the flag is in `allBuiltinToolFeatureFlags()`. `manualPathFor` was not read.
- **Non-displacing manuals:** `internal/prompts/non_displacing_guides_test.go:25-34` reads every file in `tools_manuals`. Lines 83-84 `TestDisabledRetroGamesManualTakesNoGuideSlot` assert the disabled-guide behavior. Full assertions were not read.
- **Dispatch coverage:** `internal/agent/tool_registry_validation_test.go:15` `TestBuiltinToolSchemasHaveDispatchCoverage`. Scans the literal `case "name"` in 12 fixed files (`:16-29`: `agent_dispatch_exec.go`, `dispatch_platform.go`, `agent_dispatch_comm.go`, and others). A new file must be added to this list.
- **Search terms:** `tool_search_terms_test.go:17` `TestToolSearchTermsLoad` (every term name must be a known tool). `:82` `TestToolSearchTermsCoverEveryBuiltinTool` (3–12 English and 3–12 German phrases, at most 48 runes each, `:84-86`).
- **Categories:** `tool_categories_test.go:304` `TestAllBuiltinNativeToolsAreDiscoverableByCategory` (every builtin needs a `toolCategoryDef` entry). `:322` `TestToolCategoryNamesAreNativeSchemas` (no stale entries).
- **Duplicates and flags:** `native_tools_test.go:1662` `TestBuiltinToolSchemasDoNotContainDuplicateNames`. `:1677` `TestAllBuiltinToolFeatureFlagsIncludeYepAPITools` shows the pattern for asserting a new tool is in the full flag set.
- **Operation enums and risk:** `action_policy_risk_test.go:157` `builtinOperationEnums()` (reads `operation` enums from allBuiltin schemas). `:182` `TestExternalReachOperationsAreHighRisk`.
- **Schema size:** `retro_games_tool_test.go:44-52` `TestRetroGamesSchemaStaysCompact` (at most 220 tokens). A per-tool cap is the pattern.
- **Feature gate:** `retro_games_tool_test.go:95-106` (the flag is off without a published service and off when `AgentAccess` is false). The model for a published-service gate.
- **Intent and selection:** `retro_games_tool_test.go:111-133` (intent is additive) and `retro_games_selection_test.go:35-72` (enabling retro_games changes no other tool). These are retro-specific; a new tool needs its own copy.
- **Config:** `internal/config/retro_games_test.go:27` (defaults on, explicit off respected), `:43-78` (template block equals loaded defaults).
- **Dispatch-level private tests:** `tool_execution_policy_test.go:444` uses `game_maker_file` with `RunConfig{AllowedTools: ...}`. No test was found that asserts the `private` route at `action_policy.go:321`.
- **Token budgets:** `agent_loop_helpers_test.go:55` `TestStableLocalToolSchemasEnforceHardBudgetByPriority`, `adaptive_selection_test.go:112` (`MaxSchemaTokens: 6500`), `game_maker_prompt_cache_test.go:204` (Game Maker tool-call limit). No single fixed total tool count was found.
- **Discovery evaluation:** `internal/agent/testdata/discovery_tasks_v1.json:151,157` expects `retro_games`. Adding a tool may change those expectations. Not verified by running.

### 6. Reuse, copy, do not copy

**Reuse as is:**
- `ExecutionHooks` (`execution_hooks.go:15`) and the `dispatchInner` gate order (`agent.go:1271`).
- `DispatchContext` (`dispatch_context.go:30`) fields `Cfg`, `Broker`, `AllowedTools`, `ToolScopeRestricted`, `MessageSource`.
- `gamemaker.Runner` plus `SetRunner` for any server-driven job.
- The `tools.SetXService` publish and withdraw pattern (`tools/retro_games.go:41-63`) for the chat tool, if a service is published.

**Copy:**
- The retro_games shape for the chat tool: schema file, `dispatchRetroGames`, `decodeRetroGamesArgs`, the `virtual_desktop_event` broadcast (`retro_games_tool.go:19-27`).
- The gamemaker job-context and default-service pattern for a storymaker job.

**Do not copy:**
- The hardcoded name-prefix lists (`action_policy.go:321`, `tool_scope.go:108`). They are scattered and easy to miss. Prefer one list if a second prefix family is added.
- The Game Maker operation-owner rerouting (`canonical_action.go:122-230`). It only fits tools whose operations are disjoint enums.
- Global `NonDisplacingTools` membership without deciding the selection effect. It changes adaptive selection for every query (`agent_loop_helpers.go:558`).
- The retro_games config-Save omission. Do not treat a missing Save entry as fine for a switch that the UI must persist.

### 7. Not verified

- The enclosing function of `native_tools_integrations.go:1089`.
- The body of `manualPathFor` and `prompts.ReadToolGuideFull`.
- Full assertions of `non_displacing_guides_test.go` and `tool_registry_metadata_test.go`.
- Whether the config UI saves retro_games or local_wikipedia through a path other than `Save`.
- The spec's `User interface (762)` citation. The section heading is at 727; the 762 reference is inside it or in the Security section (764), and was not read.
- The `initGameMaker` call site in `server.go`. Only the function at `game_maker_service.go:47` was read.
- Actual test outcomes. Nothing was executed.

---

<!-- Storymaker Go side -->

# Storymaker Go side: agent-tool map

Read on the current checkout. Every citation was opened, but I ran no tests, and I did not read the UI JS beyond a grep. Spec headings check out: Agent 609, Job outcome 631, Studio tools 650, Chat tool 701, Config 762, Security 764, Error handling 781, Testing 785, Phases 810.

## Spec facts that do not exist in the code

- **No agent code.** `internal/agent` has no `storymaker` reference. There is no `storymaker.agent` config, no agent job manager, no agent queue (spec says 8), no `storymaker_*` tools, no `max_daily_jobs`.
- **No assets config.** No `assets.max_daily_generations`.
- **No revision counter.** Concurrency is the ETag only (`project.go:222`). `expected_rev` would map to `ETag`.
- **No `storymaker_check`.** No check endpoint or operation exists.
- **Project fields are dead.** `Project` has `TargetSeconds int json:"target_seconds,omitempty"`, `Audience string`, and `Template string` (`project.go:68-78`). Nothing writes them: `CreateWithVoice` takes only title, aspect, fps, and voice (`voices.go:164`), and the create handler decodes with `DisallowUnknownFields` (`storymaker_handlers.go:297-302, 326-328`). `touch()` rewrites project.json from the struct, so values written by other means would survive a save.

## Stores, project, ETag

- `func LoadStory(ctx, id string) ([]byte, string, error)` (`project.go:211`)
- `func SaveStory(ctx, id string, data []byte, ifMatch string) (string, error)` (`project.go:222`). `ifMatch` is `""` (unconditional) or the etag read earlier. A stale etag returns `ErrConflict`, which HTTP maps to 412 `precondition_failed`. `touch()` then updates project.json's title, aspect, fps and `updated_at`.
- `func ETag(data []byte) string` (`project.go:65`). It is a quoted SHA-256, enforced by the regex `^"[0-9a-f]{64}"$` (`storymaker_handlers.go:186`).
- **Create:** `func (p *ProjectStore) CreateWithVoice(ctx, title, aspect string, fps int, voice *Voice) (Project, Story, string, error)` (`voices.go:164`). Plain `Create(ctx, title, aspect, fps)` wraps it with nil voice (`project.go:143`). Rules: title 1–120 runes after trim; aspect `""` (becomes 16:9), `16:9`, `9:16` or `1:1`; fps 0 (becomes 30), 24, 25 or 30. Errors wrap `ErrInvalidArgument`. The story starts as one scene `s1` with defaults applied.
- **List:** `func (p *ProjectStore) List(ctx) ([]Project, error)` (`project.go:162`). Sorted by `UpdatedAt` desc. No limit parameter, so the caller slices to 20.
- **Get:** `Get(ctx, id) (Project, error)` (`project.go:189`).
- **Project record:** `ID, Title, Aspect, FPS, CreatedAt, UpdatedAt`, plus the three dead fields above.
- **HTTP:** `POST /api/desktop/storymaker/projects` with `{title, aspect, fps, language}`. A non-empty `language` runs `ResolveProjectVoice`; if no ready voice speaks it, the answer is 422 `narration_unavailable` with `languages`. Success is 201 with `Location` and `ETag`. `GET /projects/{id}` returns `{project, etag, code_scenes, story, issues}`, with `story` null and `story_text` set when the file is unreadable (`storymaker_handlers.go:359-402`). This is the only validate-without-save route.
- **PUT story:** requires If-Match (428 if missing, 400 if malformed). It saves even when issues include errors (`storymaker_handlers.go:428-489`).

## Catalog

- **Load:** `LoadCatalog(fsys, path) (*Catalog, error)` (`catalog.go:218`). Path is `img/storymaker/catalog.json` (`storymaker_service.go:26`).
- **Search:** `func (c *Catalog) Search(query, kind string, limit int) []Entry` (`catalog.go:678`). Default limit 6, cap 12. Score: exact id 10, exact name 8, exact tag 4, name substring 3, tag substring 2, id substring 2. A trailing "s" is stripped as a fallback. The unknown kind returns nothing. Kind accepts aliases such as `char`, `props`, `env`, `background`.
- **No HTTP search.** Nothing under `internal/server/storymaker*.go` calls `Search`. The only caller is `assetAlternatives` in `validate.go:770`. Whether the UI filters the catalog client-side was not checked beyond a grep.
- **Describe:** `Lookup(id) (Entry, bool)` (`catalog.go:612`). `Entry.Raw() json.RawMessage` (`catalog.go:145`) gives the full JSON.
- **Entry fields:** `ID, Kind, Name, NameKey, Tags, Thumb, UsageExample json.RawMessage, File, Params, Spots, Floor, Variants, Aspects, Takes, ImpactMS, Align, Hit, Pattern, Source`. `name_key` is for UI localisation.
- **Helpers:** `Names(kind) []string` (`catalog.go:626`), `HasName`, `Spots(env, aspect)` (`catalog.go:780`), `SpotNames(env)` (`catalog.go:806`), `Variants(env)`, `FocusX`.

### Starter catalog sizes (`ui/img/storymaker/catalog.json`, counted by grep)

| Kind | Count | IDs |
|---|---|---|
| character | 4 | char.office-woman-1, char.office-man-1, char.it-tech-1, char.senior-man-1 |
| prop | 23 | laptop, phone, monitor, server, router, cloud, database, lock, key, shield, mail, document, coffee-cup, desk, chair, plant, check, cross, warning, idea, question, arrow-right, gear |
| environment | 4 | env.plain-stage, env.office, env.home-office, env.abstract-tech |
| pose | 10 | stand, sit, typing, point-left, point-right, wave, think, present, arms-crossed, hands-on-hips |
| gesture | 9 | |
| expression | 10 | |
| mouth | 7 | |
| cycle | 2 | walk, idle |
| text_style | 6 | |
| transition | 4 | cut, fade, slide-left, wipe |
| sfx | 7 | |

- **Palettes:** 2 (sunny, calm), in the manifest, not in entries. **Fonts:** 4 faces, all named `default` (Geist).
- **Spots:** every environment has five base spots (far-left, left, center, right, far-right). Extras: plain-stage has `board`; office has `desk, door, board, window`; home-office has `desk, door, window`; abstract-tech has `board`. The 9:16 and 1:1 aspects override some spots.
- **Lookup order:** entries are not sorted by kind, but `Names` returns them in catalog order.

## Voices

- **Endpoint:** `GET /projects` is not it. `GET /api/desktop/storymaker/voices` calls `storymakerVoiceProviders` (`storymaker_audio_handlers.go:417`). It returns `{providers: []VoiceProvider}`; there is no ready filter.
- **Order:** the active provider first, then local rows (phonon, piper, sanotts, supertonic, speech_lab), then cloud ids sorted (elevenlabs, google only if active, minimax, mistral, openai, resemble, stepfun, xai).
- **Ready:** `Ready` comes from `tools.TTSReady` on a config copy, tool route (`storymakerVoiceRow`, `:452`). Piper and Supertonic lists are bounded to 5 s. The xAI list is cached for 5 min. Speech Lab uses `Require`.
- **Row fields:** `VoiceProvider{ID, Active, Ready, Voices []VoiceOption, FreeText, Languages, TestedLanguages, NeedsInstall, DefaultVoice, ChunkLimit, Note}`. `VoiceOption{ID, Name, Language, Gender, Installed *bool, Note}`.
- **Agent ready voices** are `Ready && (FreeText || len(Voices)>0)`, filtered by the agent. There is no `kind=voice` route.
- **Resolution:** `ResolveProjectVoice(providers, language, defaultVoiceFor) (Voice, []string, error)` (`voices.go:47`).

## Jobs

- **Render manager:** `render.Manager` (`render/jobs.go:230`). One job at a time. `MaxQueued = 16` (`DefaultMaxQueued`, `:67`). `ErrBusy` at `:88` and `:458`. Retention 200 finished jobs.
- **Narrate manager:** kinds `narrate` and `music`, `MaxQueued = 128`, `Workers = 1` (`storymaker_service.go:146-164`).
- **Agent queue:** none.
- **Enqueue:** `func (m *Manager) Enqueue(req EnqueueRequest) (job Job, replayed bool, err error)` (`jobs.go:426`). `EnqueueRequest{ProjectID, Kind, Coalesce, Key, Fingerprint, Payload}`. `Coalesce` returns the queued or running job of that project and kind (used by narrate and music only). `Key` is the idempotency key, and a reused key with a different fingerprint is `ErrIdempotencyConflict`.
- **Other methods:** `Get(id) (Job, bool)`, `List(projectID) []Job`, `Cancel(id) (Job, bool)`, `CancelProject`, `WaitProject`, `Subscribe`, `ChangedSince`.
- **Job struct:** `ID, ProjectID, Kind, Status, Progress, Phase, FramesDone/Total, PiecesDone/Total, Output, Warnings, Error *JobError, CreatedAt, UpdatedAt, StartedAt, FinishedAt`. Statuses: queued, running, succeeded, failed, cancelled, interrupted.
- **Lookup across queues:** `jobByID` (`storymaker_service.go:182`).

### Job error codes (`render/jobs.go:49-82`)

`invalid_story, browser_unavailable, ffmpeg_unavailable, render_failed, render_timeout, cancelled, storymaker_disabled, busy, code_error, interrupted, narration_failed, narration_unavailable, voice_required, budget_exceeded, narration_incomplete, music_incomplete, music_failed`.

## Start narrate

`POST /projects/{id}/narrate` (`storymaker_audio_handlers.go:109-145`). Gates come from `storymakerAuthorize`, which requires write scope.

1. **Config gates.** Storymaker enabled, desktop enabled, and not read-only. Read-only is refused by `requireDesktopOperation` (`desktop_operation.go:32`). The handler comment says 403 `desktop_readonly`; I did not read the full body.
2. **Story and audio.** `storymakerStoryAudio` decodes and applies defaults, then calls `AudioAnswer`. If nothing is missing, the answer is 200 `{job: null}`.
3. **Refusals in order** (`storymakerNarrateRefused`, `:188`):
   - `voice_required` (409): a missing block has no voice.
   - `narration_unavailable` (409): the provider is not offered, or not ready.
   - `voice_unknown` (409): the voice is not on the list. It carries `alternatives` (≤5, same language first).
   - `budget_exceeded` (409): cloud characters for missing pieces exceed `max_job_chars` or today's remaining allowance. It carries `limit` and `missing`.
4. **Enqueue.** Narrate queue, `Coalesce: true`. 202 for a new job, 200 for a replay, 429 `busy` when full.

Inside the job (`storymakerNarrateStep`, `storymaker_narrate.go:264`): disk floor `MinFreeDiskBytes`, per-job and daily character caps, and a daily count in `storymaker/narrate/usage.json` that fails closed (`:45-138`). Local providers are exempt (`LocalVoiceProvider`, `voices.go:148`).

## Start music

`POST /projects/{id}/music` with `{duration_ms, generate}` (`storymaker_audio_handlers.go:647-742`).

- No music, or volume 0: 200 `{job: null}`.
- Generated mode: `duration_ms` must be in `[1000, max_duration_seconds*1000]`, else 400 `invalid_argument`.
- Already stored: 200 `{job: null}`.
- Generated, not stored, `generate` false: 200 `{job: null}`, nothing is queued.
- Generated, `generate: true`: 409 `music_unavailable` if no provider is configured; 409 `budget_exceeded` if the budget is blocked.
- File or track source problems: 422 `invalid_music_file`, with `music_issue: source_missing` for a missing source.
- Otherwise queued on the narrate queue (`Coalesce`), with `GenerateKey` set only when `generate: true`.

Only `generate: true` sends a paid call. Inside the job the key must still match, or the job refuses. Budget category is `music_generation`. There is no separate daily cap for music.

## Start render

`POST /projects/{id}/render` (`storymaker_handlers.go:580-668`).

- Headers: `If-Match` required (428 or 400). `Idempotency-Key` optional, 1–128 printable characters. Body at most 16 KiB, parsed by `ParseRenderOptions` (`options.go:34`): `burn_in_captions`, `srt`, `vtt`, `stems`, `quality` (standard or high).
- Order: project exists, then `storymakerSettings` (FFmpeg and FFprobe resolved, read-only refused with `render_failed`), then FFmpeg check (409 `ffmpeg_unavailable`), then browser status (409 `browser_unavailable` if not ready and cannot download), then `Prepare`.
- `Prepare` (`render.go:164-247`):
  - stale etag: 412 `precondition_failed`, with the current etag in the header.
  - decode failure, or any error-severity issue, including code-scene issues: 422 `invalid_story` with issues.
  - missing narration blocks: 409 `narration_incomplete`, with `missing` and `chars`.
  - music not stored or source gone: 409 `music_incomplete`, with `music_key` or `music_issue`.
  - code scene with no bundle: 422 `code_error`.
- Enqueue: no coalescing, idempotency only. 429 `busy` at 16, 409 `idempotency_conflict`, otherwise 202 `{job}`.
- `Prepare` has no compile step, so it cannot give a compile report.

## Read-only, caps, config

- **Read-only:** the job starter, every provider call, and the in-job gates all check read-only (`storymakerSettings`, `storymakerNarrateGate`, `storymakerMusicPreflight`). Cancel uses `desktopStop` and works while read-only.
- **Daily caps:** only the narration caps exist. Cloud narration: `max_daily_chars`, `max_job_chars` (defaults 100000 and 20000). Music uses the shared budget tracker. Chat-tool and asset caps do not exist.
- **Config keys that exist** (`internal/config/storymaker.go:12-38`):
  - `storymaker.enabled`
  - `storymaker.render.timeout_seconds` (60–14400, default 3600)
  - `storymaker.render.max_duration_seconds` (10–3600, default 900)
  - `storymaker.render.browser_path`
  - `storymaker.render.transport` (`""`, png, webp, raw, screenshot)
  - `storymaker.narration.max_daily_chars` (1000–100000000, default 100000)
  - `storymaker.narration.max_job_chars` (100–10000000, default 20000)
- **Missing:** every `storymaker.agent.*` key and `storymaker.assets.*`.
- **Enabled default:** the comment says it defaults to true in the loader. I did not read the loader.
- **Status endpoint** (`storymaker_handlers.go:202-260`) returns `limits` including `max_queued_renders` (16), `max_queued_narrations` (128), and the caps. A chat tool can read these for next actions.

## Validate

`func Validate(s *Story, cat *Catalog, lim Limits) []Issue` (`validate.go:96`). A nil `lim` becomes `DefaultLimits()`. It does not fill defaults. It canonicalises prefixed names in place and reports each change as `canonicalised`.

**Issue** (`model.go:66-74`): `{severity, scene_id?, path (JSON Pointer), code, message, at_seconds? (never set by Go), fix?}`.
- **Severity:** error, warning, minor.
- **Alternatives:** there is no separate field. They are inside `fix` as "Use one of: a, b, c. Example: {...}" (`closestNames`, `issues.go:62`, up to 5).
- **Fix:** one or two sentences with an example JSON fragment.
- **Helpers:** `HasErrors`, `CapIssues(issues, n)` (errors kept first), `MaxModelIssues = 8`, `MaxHTTPIssues = 200`.

### Go validates without Chrome

- Schema, title, format (aspect, fps), style (palette and font against the catalog), voice (provider, language, voice-id pattern), music (mode, volume, desktop path rules, track id, mood list).
- Cast ids, duplicates, character asset prefix and catalog kind.
- Scene ids, kind, environment, variant (warning when the environment lacks it), narration length, speaker, transition, duration, camera framing, and explicit scene durations summed against `max_duration_seconds` (`validate.go:396-404`).
- Actor, prop and beat fields, including per-kind required fields and spot names against the catalog.
- Cue markers `{n}` must exist in the narration.
- Glyph coverage, if the catalog has `glyphs.json`.
- Code scenes: `CodeSceneIssues` uses the saved bundle status (`code_error`). The bundling itself (esbuild syntax check and a pre-check) runs at save time in `BundleCodeScene` (`codebundle.go:46`), not in `Validate`.

### Only the JS compile or the render page knows

- Motion-lint codes (`line_unread`, `dead_stretch`, `first_frame_empty`, `text_too_small`, `fade_between_worlds`, `entry_monotony`, `transition_variety`, `cue_late`). The constants exist in `model.go:48-60`, and the comment there says only the player's compile report emits them.
- `layout_unverified`, `narration_missing` and `deadline` are defined. Only `deadline` and `code_error` are referenced outside tests, both in the render code.
- `narration_seconds`, `cue_times`, `end_seconds`, `over_target`: no Go code.

## Timing and compile report

- **No Go timing estimate.** I found no words-per-second estimate. Grep for "estimate" in non-test `internal/storymaker` returned only comments and a mix-size helper.
- **Measured timing exists only after synthesis.** `AudioAnswer` (`narration.go:625`) has per-block pieces with `AtMS, InMS, OutMS`, and envelope durations. A block enters `Narration` only when every piece is stored (`:678-687`). Missing blocks have no timing, and the comment says JS estimates them.
- **Cue positions:** `parseMarkers` gives UTF-16 offsets of each marker (`narration.go:113`), with no time mapping.
- **Compile report without frames:** no. The only compile is in the render page (`render.go:528-540`), which posts `POST /report` to the per-job listener (`render/listener.go:457`). I grepped for `compileOnly`, `dryRun`, `reportOnly` and found nothing.
- **What `storymaker_check` could reuse:** the GET-project issue path (Validate plus `CodeSceneIssues`), and the narration answer for measured durations of fully narrated blocks.

## Story schema for a model

Keys in document order.

- **Story:** `schema` ("storymaker/1"), `title` (1–120), `format {aspect, fps}`, `style {palette, font, captions, rhythm?, auto_sfx?}`, `voice? {provider, voice, language}`, `music? {mode: none|generated|file, mood?, prompt?, file?, track?, volume 0–1}`, `cast [{id slug, name?, character char.*, voice?}]`, `scenes []`.
- **Scene:** `id` (slug, required), `kind` (storyboard|code), `environment` (env.*), `variant` (day|night), `narration` (≤1500 chars), `silent`, `speaker` (narrator or cast id), `actors [{cast, at, pose?, facing?, expression?, scale?}]`, `props [{id?, item, at, layer?, scale?}]`, `beats []`, `transition`, `duration` ("auto" or seconds >0), `camera {framing, target?}`.
- **Transitions:** cut, fade, slide-left, slide-right, slide-up, slide-down, wipe, zoom, iris, whip. Phase 1 plays only cut, fade, slide-left and wipe; the rest are accepted with `not_supported_yet`.
- **Beat kinds (16):** enter, exit, walk, move, pose, gesture, emote, look, say, show, hide, highlight, camera, chart, sfx, wait. `chart` is accepted but not played (`validate.go:443-446`).
- **Required per kind:**
  - enter: `who`, `from`, `to`.
  - exit: `who`, `to`.
  - walk, move: `who` or `target`, and `to`.
  - pose: `who`, `pose`.
  - gesture: `who`, `gesture`; `count` 1–5 (0 is unset).
  - emote: `who`, `expression`.
  - look: `who`, `target`.
  - say: `who`, `style` (speech|thought); `text` optional. Without text, `who` must be the scene speaker.
  - show: exactly one of `item`, `text`, `icon`, `chart`; `at`; `entry` (pop|fade|slide|none).
  - hide: `target`.
  - highlight: `target`, `style` (pulse|outline|arrow).
  - camera: `op` (zoom|pan|focus|shake); `target` and `amount` optional.
  - chart: `chart` (bar|line|pie|counter), `data {labels, values}` of equal length 1–24, `at`.
  - sfx: `sound` (sfx.*).
  - wait: `seconds` 0–600, greater than 0.
- **Common beat fields:** `with: "prev"`, `cue` (marker number), `delay` 0–600, `duration` 0–600, `ease` (linear, ease-in, ease-out, ease-in-out, back-out, bounce-out).
- **Positions:** a spot name of the environment, or `{x, y}` in 0–100.

### Server defaults (`ApplyDefaults`, `defaults.go:32`)

- Schema, format 16:9 and 30 fps, style sunny/default/captions true (only when `style` is wholly absent).
- Cast name = id.
- Scene environment from the previous scene, or `env.plain-stage`. Variant `day`. Speaker `narrator`. Transition: `cut` for the first scene, `wipe` on an environment change, else `fade`. Duration `auto`. Camera `wide`.
- Actors: position `center` for one actor, else slots left, right, far-left, far-right by cast order; pose `stand`; facing `auto`; expression `neutral`; scale 1.
- Props: layer `front`. Show beats get derived ids `<slug>-<n>`.
- Timing other than `duration` is not defaulted.

`Style.UnmarshalJSON` sets `captions: true` when the key is missing (`model.go:135`). `rhythm` is never defaulted.

### Beat examples from the validator (`validate.go:61-78`)

Example for `say`: `{"do":"say","who":"anna","text":"Das ist ein Paket.","style":"speech"}`. Each example is roughly 50–70 bytes.

### Minimal valid scene

`{"id":"s1","narration":"Hallo."}`, about 32 bytes. A whole minimal story is about 80 bytes: schema, title, and that one scene. I counted by hand; I did not run Validate on it.

## Reuse, copy, do not copy

- **Reuse as is:**
  - `storymaker`: `DecodeStory`, `ApplyDefaults`, `Validate`, `CapIssues`, `CanonicalJSON`, `Catalog.Search, Lookup, Names, Spots`, `ResolveProjectVoice`, `Blocks`, `PieceKey`.
  - `ProjectStore`: `CreateWithVoice, LoadStory, SaveStory, List, Get`.
  - `render.Manager.Enqueue`, `render.Runner.Prepare`.
  - `storymakerAuthorize`, the refusal-ordering helper, the status endpoint.
- **Copy the pattern:** the fail-closed daily counter (`storymakerNarrationUsage`, `storymaker_narrate.go:48`), and the narrate refusal order.
- **Do not copy:**
  - The HTTP error envelope for the chat tool. It is the desktop API shape, not an agent result.
  - The `storymakerSynth` and `storymakerMusicSeam` test seams.
  - `PUT` as a dry run: it always saves. Use `Validate` directly for a check without a save.
  - The idea that `Project` fields are used. They are never written.

## Unverified

- Client-side catalog filtering in `ui/js/storymaker`, beyond the grep.
- Whether `desktop_readonly` is returned exactly as the handler comment says.
- The loader default for `storymaker.enabled`.
- The actual output of `Validate` on the minimal story and the starter catalog. Nothing was run.
- `AudioBlock` field types in `envelope.go:188`. I used only what the calling code shows.

---

<!-- Storymaker app -->

# Storymaker desktop app: map for an agent panel

Read on the current working tree. Line numbers are from reading the files, not from the 2026-11-01 spec. The spec was not read.

## 1. Structure and state

- Sixteen classic scripts, all in `ui/js/desktop/apps/` (line counts): `storymaker-core.js` 346, `-model.js` 454, `-api.js` 219, `-assets.js` 128, `-player.js` 287, `-projects.js` 144, `-strip.js` 248, `-inspector.js` 282, `-code.js` 473, `-export.js` 539, `-audio.js` 607, `-voice.js` 478, `-music.js` 360, `-dialog.js` 109, `-handoff.js` 183, and the composition root `storymaker.js` 968. The root is the file closest to the 1100-line budget.
- Namespace: `window.StorymakerParts` (`S`). Core provides `S.el` (no `html` option; throws on it), `S.icon` (fixed `ICONS` table, core.js:62-87), `S.bag`, `S.createBus` (`on` returns unsubscribe, `emit` never throws), `S.clamp`, `S.format`, and `S.errors.{normalize, describe, describeJob, issueTitle}`.
- Window state lives on `inst`, created in `createInstance(host, windowId, ctx)` (storymaker.js:16-38). Key fields: `api`, `bus`, `bag`, `status`, `projectId`, `project`, `story`, `etag`, `issues`, `codeStatus`, `report`, `audioInfo`, `conflict`, `dirty`, `saveState`, `sideTab` (default `'scene'`), `parts` (mounted panels), `events`, `handoffWish`. `inst.domId(suffix)` makes window-scoped element ids.
- The bus carries these names: `status`, `story`, `report`, `issues`, `conflict`, `readonly`, `job`, `audio`, `catalog`, `selection`, `time`, `tab`, `thumb`. The `job` event is emitted at storymaker.js:219 from `inst.api.events(...)`.
- Editor layout (storymaker.js:271-310): toolbar `header.stm-bar`, banners, `.stm-body` as a grid of `208px | 1fr | 328px` (CSS line 106) holding strip, center (stage, code, issues) and `aside.stm-side` (`data-stm="side"`).
- Right rail: a tablist of side tabs built inline at storymaker.js:297-306 (scene, library, voice, then music appended separately). `setSideTab` (600-604) sets `inst.sideTab` and `dataset.sidetab`. `showPanel(name)` (607-610) switches to the storyboard tab and then the side tab. The export refusals use it.

## 2. How a panel is added (voice and music as the model)

- Factory shape: `createVoicePanel(inst)` (voice.js:229) and `createMusicPanel(inst)` (music.js:30) return `{el, paint, dispose}`. Voice also returns `marker`, which is appended to the stage (storymaker.js:279). Each builds its DOM with `el()`, subscribes with `inst.bus.on(...)` into an `off` array, and `dispose()` runs those unsubscribes. The editor calls `dispose` through `closeEditor` (storymaker.js:173-176). Mounting happens in `buildEditor` (277-306) and each panel is stored at `inst.parts.<name>`.
- Inputs the panels use from `inst`: `t`, `bus`, `api`, `story`, `selected`, `projectId`, `readonly`, `audioInfo`, `report`, `status`, `domId()`, `notify(text, kind)`, `editStory(fn, {immediate})`, `flushEdits()`, `refreshAudio()`. Story-level writes go through `inst.editStory` (storymaker.js:738-744), which clones, mutates and calls `commit`.
- `inst.bag` exists on the instance (storymaker.js:22) but the panel contract does not pass it. Export adds document listeners directly and removes them in `dispose` (export.js:477-530). That works but is not the pattern to copy.
- Voice's job filter, `take(next, adopt)` (voice.js:155-162), and music's `applyJob(next, mine)` (music.js:180-189) are the reusable idea: ignore jobs of other projects and other kinds, keep an active job, and keep a finished one only when this panel asked for it.

## 3. Load, save, ETags

- Start: `start` (storymaker.js:85) calls `api.status()`, then `showProjects` (100) and `api.listProjects()`.
- Open: `openProject(inst, id, fresh)` (193-228) calls `api.getProject(id)`, which returns `{project, story, etag, issues, code_scenes}`. A `story` that is null or not an object throws `story_unreadable`. Then `closeEditor`, `buildEditor`, `inst.events = inst.api.events(projectId, job => bus.emit('job', job))` (219), `reloadPlayer`. A sequence number (`inst.openSeq`) makes only the latest open win.
- Save: `commit(inst, draft, immediate)` (746) sets `dirty` and schedules `saveNow` after `SAVE_DELAY_MS` = 700 (line 11). `runSave` (763-794) sends `api.saveStory(id, JSON.stringify(story), inst.etag)` (api.js:118, `If-Match`). On success it takes the new etag and issues. If the server's story differs from the local text, the server version replaces the local one (779). Edits made while the save was in flight trigger another save (`saveAgain`).
- Errors: `handleSaveError` (807-816). 412 sets `inst.conflict = true`. 422 keeps the text and shows issues. `desktop_readonly` forces read-only. `runSave` returns early while `conflict` is set (764), so edits keep accumulating in `inst.story` with no save.
- Conflict UI: banner with `conflict-reload` and `conflict-overwrite` (storymaker.js:688-692). `reloadProject` (874-892) GETs the project, replaces `story` and `etag`, and drops unsaved local state. `overwrite` (895-907) GETs the current etag, then saves local state over it.
- Server side (`internal/server/storymaker_handlers.go`): `handleStorymakerStory` (428-489) requires `If-Match` (`requireStorymakerMatch` 189-200: 428 if absent, 400 if malformed, pattern `^"[0-9a-f]{64}"$`). It decodes strictly, applies defaults, validates, then `SaveStory`. On `ErrConflict` it sets the response `ETag` header to the current version (476). The ETag is the quoted SHA-256 (`internal/storymaker/project.go:63-65`, via `desktop.NoteVersion`).

## 4. Server change to story.json while open

There is no reload path except the conflict banner. No code in `ui/js/desktop/apps/storymaker*.js` or `internal/storymaker` watches or polls `story.json` (the grep for watch/poll/ModTime found only `ModTime` fields, a comment in `memstore.go`, and `setInterval` in unrelated modules). The app notices an external change only at the next save, which returns 412 and shows the conflict banner. Audio refreshes on project open, after each successful save, and on terminal job status (`storymaker-audio.js:196-198`). `refreshIssues` re-reads the project only after a code save (storymaker.js:858, 864). Not verified: whether a window focus or visibility handler re-reads the project. Grep found none, but I did not read every module in full.

Consequence for an agent that writes the story while the window is open: the open editor's next autosave gets 412, and the user must choose reload (local edits since the last save are lost) or overwrite.

## 5. Opening a project by launch parameter

There is none. `render(host, windowId, ctx)` (storymaker.js:934-954) reads only `ctx.t`, `ctx.readonly`, `ctx.setWindowBeforeClose`, `ctx.confirmDialog`, and `ctx.openApp` (export only). It never reads `ctx.projectId` or `ctx.launch`. The project list opens projects only by click (`storymaker-projects.js:99`, `inst.openProject(project.id)`).

- The render dispatch passes the whole context through: `menus-and-routing.js:1781-1791` spreads `context` into `ctx` and adds `esc, api, t, iconMarkup, notify: showDesktopNotification, readonly, openApp, confirmDialog, setWindowBeforeClose, isActive`. So a launch key would reach `ctx`. I did not trace `openApp(appId, context)` (`window-shell-runtime.js:1588`) to `renderAppContent` for storymaker.
- Patterns to copy: ECalc's sanitized launch (`AGENTS.md:60`, `sdk-events-bootstrap.js:455-462` builds `launch.launch` and calls `openApp(app_id, launch)`), and the Video Studio merge at `menus-and-routing.js:1649` (`pendingProjectId` becomes `projectId`).
- Single-instance behaviour is not verified. `render` disposes any instance under the same `windowId` (storymaker.js:935) but does not check whether a second Storymaker window may exist.

## 6. Job events: Game Maker and Storymaker

- Game Maker (`game-maker-studio.js:631-675`): connects `new EventSource(state.api.eventURL(projectId, lastEventID))` (`game-maker-studio-api.js:64`, `?after=`), with `source.onopen` and `onerror` handlers, and per-type `addEventListener`. It keeps a catch-up flag until `stream_ready`. It polls `refreshActiveJob` every 15 s (`:1875`) only while another project is busy. Its reconnect logic is hand-written.
- Storymaker: `api.events(projectId, onJob, onState)` (`storymaker-api.js:184-197`) wraps `reopeningSource` (30-64), which reopens a dropped source with a 1 s backoff doubling to 30 s. It listens for `job` events and passes `body.job` to `onJob` when it has a string `id`. Nothing else needs writing.
- Server (`storymaker_handlers.go:732-`): `event: job`, `data: {"job": ...}`. On connect it replays unfinished jobs and the latest five (`initialJobs`, 774). Heartbeat every 15 s (781). Both queues share the stream, so filter by `kind` (`render`, `narrate`, `music`) and by `project_id`.
- Export's safety net (`storymaker-export.js:327-339`): `setTimeout` 2500 ms (`POLL_MS`), only while a job is active, via `api.getJob`.
- Simplest pattern: subscribe to `bus.on('job')`, filter by kind and project, adopt only active jobs unless asked, and poll `getJob` with a 2.5 s timer only while a job is active. The voice panel has no poll; it relies on the stream and `refreshAudio`.

## 7. Desktop notifications

- Server sink, as in `internal/server/detective_notify.go:45-64`: `broadcastDesktopEvent(s, hub, desktop.Event{Type: "notification", Payload: {title, message, type, appId, context}, CreatedAt})`. It is silent unless `cfg.VirtualDesktop.Enabled`, and it is limited per key to one per minute (`detectiveNotifyLimiter`, 14-39).
- Client: `sdk-events-bootstrap.js:465-467` calls `showDesktopNotification(payload)` (494), which plays a sound and records the notification.
- Storymaker's `ctx.notify` is `showDesktopNotification` (`menus-and-routing.js:1788`). `storymaker.js` does not use it. Its in-window message is `inst.notify(text, kind)` (storymaker.js:42-50), a live region that clears after 5 s. Other modules were not grepped for `ctx.notify`.
- Storymaker posts no desktop notification today.

## 8. Browser tour

- File: `internal/server/storymaker_app_browser_test.go`. Gate `AURAGO_RUN_BROWSER_SMOKE=1`, `go test ./internal/server -run TestStorymakerAppRealServerBrowser -count=1 -timeout 15m` (lines 31-37). The test skips when no Chrome is installed (121-123).
- Setup: `smappPage` / `smappPageWith(prepare)` (74-148) serves `desktop.html`, mounts `handleStorymaker` at `storymakerAPIBase+"/"` (115) and `handleVideoStudio`, opens Chrome, and calls `auditTest.openApp('storymaker')` (146). It overrides `window.t` with the English words of `lang/storymaker` and `lang/storymaker_catalog` (39-56, 142).
- Helpers: `smappWait` (151, 90 s), `smappWaitFor(limit)` (156), `smappClick` (171), `smappText`, `smappNoErrors` (177), `smappStored` (217), `smappEventually` (265), `smappNarrateByRoute` (277), and `storymakerSynthOverride` (486) as the stub synth.
- Steps in `TestStorymakerAppRealServerBrowser` (479-737): start (493-499), create (501-519), play (521-526), narration (528-536), library (538-552), code (554-575), code scene (575-584), inspector (586-597), voice (601-634), a side-tab overflow check (636-639), gate (641-651), export (653-694), handoff (694-698), reopen (700-705), export-last (707-711), delete (714-726), close (734-737).
- Panel step: an agent side tab would be one `smappClick` on `[data-stm="side-tab"][data-tab=...]` plus an assertion, placed after the voice block (after line 634) and before the overflow check (636). The overflow check is generic and still applies with a fifth tab.

## 9. Budgets and guards (`ui/`)

- `TestStorymakerSourcesAreSafeAndSmall` (`desktop_storymaker_guards_test.go:44-87`). It globs `js/desktop/apps/storymaker*.js`, so a new file is picked up automatically, and `js/storymaker/*.js`. Forbidden: `innerHTML`, `outerHTML`, `insertAdjacentHTML`, `document.write`, `eval`, `new Function`, `alert`/`confirm`/`prompt`, `localStorage`/`sessionStorage`, `document.cookie`, and template strings in app modules. Budget: an app module must have fewer than 1100 newlines (line 76). It must start with `//` and contain `'use strict'`, and must mention `StorymakerParts` or `StorymakerApp`.
- `TestStorymakerAppLoadsNothingExternalAndAvoidsFilteredClasses` (91-127). No external `http(s)://` host. Each class in a `class: '...'` literal must start with `stm-` and must not begin with `cc-`, `cookie`, `consent`, `gdpr`, `adsbox`, `ad-banner`, `banner-ad`, `sponsor`. This regex catches only single-quoted literal classes.
- `TestStorymakerAppUsesOnlyKeysItHas` (130-187). Literal keys matched by `'(storymaker\.[A-Za-z0-9_.]+)'` (the regex at guards:11) must exist in `lang/storymaker/en.json`. At least 150 literals must match. Prefix families are checked against `ISSUE_CODES` and `JOB_ERROR_CODES` in core.js (lines 325 and 317), `PHASES` in export.js, and `source_*`. Fallbacks `storymaker.issue.unknown`, `job_error.unknown`, `export.source_unknown` are required.
- `TestStorymakerLoaderListsEveryScript` (204-230). Every `storymaker*.js` on disk must be in the loader's storymaker entry, and every listed name must exist.
- `TestDesktopStorymakerRegistration` (`desktop_storymaker_registration_test.go`). `storymakerModules` (line 11) is the load-order list, core first, composition root last, and the test checks order only for names it lists. It also checks the loader entry (CSS via `appStyles`), `APP_I18N_SECTIONS` (`storymaker: ['storymaker']`), the dispatch markers in `desktop-foundation.js`, `menus-and-routing.js`, `window-shell-runtime.js` and `main.bundle.js` (the bundle requires `npm run build:ui`), and `desktop.app_storymaker` in all 16 locales.
- `TestStorymakerLocalesCoverAllSixteenLanguages` (`desktop_storymaker_i18n_test.go:34-73`). Languages come from `storymakerLanguages` (`desktop_storymaker_name_test.go:9`): cs, da, de, el, en, es, fr, hi, it, ja, nl, no, pl, pt, sv, zh. Every key of `en.json` must be present, non-empty, carry the same placeholders (regex at line 13), and be free of U+FFFD. No extra keys are allowed. `storymaker_catalog/` holds only `storymaker.catalog.*`, and `storymaker/` holds all other `storymaker.*` keys.
- `TestStorymakerAppTranslationsAreTranslated` (`:78`) flags English text reused as a translation, with an allowlist (line 86). Not read in full.
- `TestStorymakerStylesheetUsesOnlyTokens` (`desktop_storymaker_css_test.go:31`) and `TestStorymakerScrubWaveIgnoresThePointer` (81). They check tokens, the container queries `stm / inline-size`, `@container stm (max-width: 940px)` and `(max-width: 640px)`, `@media (pointer: coarse)`, and a reduced-motion gate. I found no CSS line budget in these tests, and I did not count the CSS file's lines.
- Verify commands from `ui/js/desktop/apps/AGENTS.md:87`: `go test ./ui -run 'Storymaker|TestDesktopStorymaker|TestConfigStorymaker' -count=1`, and `AURAGO_RUN_BROWSER_SMOKE=1 go test ./internal/server -run '^TestStorymakerAppRealServerBrowser$' -count=1 -timeout 15m`.

## 10. Files a new app part touches

Assuming a new `storymaker-agent.js`:

1. `ui/js/desktop/apps/storymaker-agent.js` (new). Purpose comment, `'use strict'`, `S.createAgentPanel`, under 1100 lines.
2. `ui/js/desktop/core/module-loader.js:35`. Insert in the storymaker `scripts` array before `storymaker.js`. No bundle change, because storymaker scripts load lazily (`menus-and-routing.js:1781-1785`), not via `main.bundle.js`.
3. `ui/js/desktop/apps/storymaker.js`. `buildEditor` (storymaker.js:271-306) for creation, mounting, the side tab and the `inst.parts` entry; `setSideTab` and `showPanel` (600-610) if the name is new; `closeEditor` disposal (173-176) happens automatically.
4. `ui/desktop_storymaker_registration_test.go:11`. Add the new module name in the position it should load.
5. `lang/storymaker/en.json` plus the other 15 locale files. Keys must be literal strings in single quotes, or the guard does not see them. Keys built by concatenation need a family list.
6. `ui/css/desktop-app-storymaker.css`. New classes must start with `stm-`. Use tokens only.
7. `ui/js/desktop/apps/AGENTS.md:75` ("sixteen classic scripts" and the list) and the section at lines 73-95, if documentation is in scope.
8. `internal/server/storymaker_app_browser_test.go`. One step after line 634 in `TestStorymakerAppRealServerBrowser`.
9. Only if a shell file changes: `menus-and-routing.js`, `window-shell-runtime.js` or `desktop-foundation.js`, then rebuild with `npm run build:ui`.
10. `ui/desktop_storymaker_browser_test.go` (the stand-in variant of the same app) was not read; it may need a matching step.

## 11. Reuse, copy, do not copy

- Reuse as is: `S.el`, `S.icon` (existing icon names only, since the table is shared), `S.bag`, `S.createBus`, `S.errors.*`, `S.format`, `inst.editStory`, `inst.notify`, `inst.flushEdits`, `inst.domId`, the bus names above, and `api.events`/`reopeningSource`.
- Copy: the `createXPanel(inst) -> {el, paint, dispose}` factory with its `off` list (voice.js, music.js); the job filter `take`/`applyJob` (voice.js:155, music.js:180); the 2.5 s active-job poll (export.js:327-339) if a poll is needed.
- Do not copy:
  - Document-level listeners without the bag (export.js:477-484 pattern).
  - Hard-coded side-tab lists (storymaker.js:297-304): a panel should be registered from one list.
  - Writing `inst.parts` ad hoc.
  - Any path that sends the agent's story writes through `editStory`. The server is the writer, so the editor must handle 412 and reload.
  - `window.StorymakerApp` as the only entry point for a new feature.

## 12. Not verified

- Full reads: only `storymaker.js`, `-core.js`, `-voice.js`, `-music.js`, `-api.js`, `-export.js` and the bindAudio part of `-audio.js` (194-263). `-model.js`, `-assets.js`, `-player.js`, `-strip.js`, `-inspector.js`, `-code.js`, `-dialog.js`, `-handoff.js` and the rest of `-audio.js` were covered by grep only, and `-projects.js` only to line 30.
- The ECalc launch sanitizer (`sanitizeECalcLaunch`) and `desktop-app-storymaker.css` line count.
- Whether a focus/visibility handler re-reads a project.
- Whether Storymaker windows are single-instance.
- The full `openApp` path for a context payload.
- The full bodies of `TestStorymakerAppTranslationsAreTranslated` and `TestStorymakerStylesheetUsesOnlyTokens`.
- `ctx.notify` usage outside `storymaker.js`.
- The spec sections and their citations, and `internal/storymaker/AGENTS.md`.

---

<!-- Test harness -->

# Test harness map for the Storymaker agent core

I read the files at the paths cited below. I did not run any tests, and the repo was not modified. Durations are unverified.

## 1. Fake LLM client and how it reaches the Game Maker job runner

**Interface.** `llm.ChatClient` is at `internal/llm/interface.go:12-15`:
- `CreateChatCompletion(ctx, openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error)`
- `CreateChatCompletionStream(ctx, openai.ChatCompletionRequest) (llm.CompletionStream, error)`

`CompletionStream` (`interface.go:20-23`) has `Recv()` and `Close()`.

**Scripted fake used by the replay test.** `circuitBreakerSequenceClient` is at `internal/agent/circuit_breaker_protocol_test.go:307-334`. It is a test-only type with fields `responses []openai.ChatCompletionResponse`, `requests []openai.ChatCompletionRequest` and `onResponse func()`. Each call appends the request and returns `responses[len(requests)-1]`. It fills in `FinishReason` (`stop`, or `tool_calls` when tool calls are present). The stream method returns `nil, nil`. Other fakes are `mockChatClient` (`internal/agent/context_compression_test.go:18-42`: `response`, `finishReason`, `err`, `lastReq`, `calls`) and `mockChatClient` in `internal/kgextraction` and `internal/llm`.

**Injection, two routes:**
- **Agent loop (internal/agent).** `RunConfig.LLMClient llm.ChatClient` (`internal/agent/agent.go:1161`). The replay test sets `runCfg.LLMClient = client` inside the runner callback, then calls `ExecuteAgentLoop(ctx, req, runCfg, false, NoopBroker{})` (`agent_loop.go:314`). `NoopBroker` is at `agent.go:273`.
- **Server job runner (internal/server).** `Server.LLMClient` is a `llm.ChatClient`. `gameMakerAgentRunner{server, service}.RunGameMakerJob` (`internal/server/game_maker_service.go:595`) returns "Game Maker LLM is not configured" if `LLMClient` is nil. It switches to `llm.WrapOpenAIClient(...)` only when the job names a provider or model (`:627-631`).

**Minimal example, server route:**
```go
cc := openai.DefaultConfig("test-only"); cc.BaseURL = provider.URL   // httptest server that streams SSE
srv := &Server{Cfg: cfg, GameMaker: svc, LLMClient: llm.WrapOpenAIClient(openai.NewClientWithConfig(cc)), Logger: ...}
runner := &gameMakerAgentRunner{server: srv, service: svc}
svc.SetRunner(implementationTestRunner(func(ctx context.Context, run gamemaker.JobRun) error { ... runner.RunGameMakerJob(ctx, run) ... }))
```
This is from `internal/server/game_maker_direct_generation_test.go:70-76`.

**Two fake-provider styles in the server tests:**
- An `httptest` server behind a real `openai` client (`game_maker_direct_generation_test.go:39-75`, `game_maker_gateway_test.go:24-81`, `game_maker_implementation_test.go:155`).
- A direct `llm.ChatClient` on `Server.LLMClient`, as in `game_maker_gateway_test.go:66-68`, which uses `llm.NewFailoverManager`.

**Job runner interface.** `gamemaker.Runner` is `RunGameMakerJob(ctx, JobRun) error` (`internal/gamemaker/types.go:251-253`). `JobRun` (`types.go:232-247`) has `Stage` ("planning", "building", "repair", "visual"), `Job`, `Project`, `Plan`, `Result`, `Checks`, `Diagnostics`, `AssetPacks`, `WorkspaceDrift` and others.

**Service construction.** `gamemaker.NewService(gamemaker.Options{DBPath, WorkspacePath, Enabled, AllowCreate, AllowEdit})` (`internal/gamemaker/service.go:89`) opens SQLite and recovers the filesystem. `Close()` is at `:148` and `SetRunner(Runner)` at `:161`. `SetSkillStatus(nil, true)` is needed before jobs start. `SetDefaultService` and `DefaultService` are at `:77-87`. The agent-side `dispatchGameMaker(ctx, tc, nil)` (`internal/agent/agent_dispatch_game_maker.go:430`) uses the default service.

**Runner type used in agent tests.** `gameMakerPlanTestRunner func(ctx, gamemaker.JobRun) error`, with a `RunGameMakerJob` method (`internal/agent/native_tools_game_maker_test.go:116-120`).

## 2. Game Maker replay test

**Test.** `TestGameMakerTextModeReplay` is at `internal/agent/game_maker_text_mode_replay_test.go:159-197`. It runs in-process and uses no LLM.

**Helper.** `replayGameMakerPlanning(t, dimension, replies, tune...)` is at `:60-127`. It does the following:
1. Calls `newPromptPipelineTestRunConfig(t, sessionID, "game_maker")` (`internal/agent/loopback_test.go:383`). This builds an in-memory SQLite memory, an ephemeral history, a `mockChatClient` and a config with prompts from `../../prompts`. It returns `(RunConfig, *mockChatClient, cleanup)`.
2. Sets `UseNativeFunctions=false`, `Model="glm-4"`, `GameMaker.Enabled`, `CircuitBreaker.MaxToolCalls=12`, `SuppressTurnSideEffects=true` (`agent.go:1221`), and `IsMission=true`.
3. Sets `AllowedTools` to the names from `GameMakerPhaseToolSchemas("planning", dimension)` (`native_tools_game_maker.go:291`).
4. Creates a temp `gamemaker.Service`, sets it as default, creates a project and sets the runner.
5. Inside the runner: asserts stage is "planning", sets `runCfg.LLMClient`, sets `RunComplete`, and runs `ExecuteAgentLoop`. It collects `EventsAfter(ctx, projectID, 0, 500)` and the last request's messages.
6. Pads the scripted replies with three "Planning is done." replies.
7. Waits up to 20 s (`:123`).

**Fixture.** `internal/agent/testdata/game_maker_replay/text_mode_planning.json` has two recorded production jobs (MiniMax M3 in 2d, GLM-5.3-Flash in 3d). It holds `replies` keyed by name and a `sequence`. The labels are checked (`source == "production replay"`, `date == "2026-10"`).

**Assertions.** `requireTextModeWithoutDenial` checks that native tools were not sent, and that the replies contain none of "tool_scope_denied", "unknown_tool", "outside this run's scope" or "is not allowed in this agent run" (`:142-152`). `toolResults()` (`:132-140`) reads `tool_result` events as "tool/operation:status".

**Whether it runs with no real model.** Yes. The client is `circuitBreakerSequenceClient`, and the dispatch is real: the tool calls go through the agent loop and `dispatchGameMaker` into the real service.

**Duration.** Unverified. The code only sets a 20 s upper bound.

**Related test.** `internal/agent/game_maker_recovery_nudge_test.go` tests the Studio-specific nudge without an LLM (`:60-114`), and checks that the nudge example parses (`:118-130`).

## 3. Full agent loop with a scripted model and real dispatch

Yes, and the replay test above is one. The chain is `ExecuteAgentLoop` → scripted `ChatClient` → `ParseToolCall` (`agent_parse.go:848`) → `dispatchGameMaker` → the real `gamemaker.Service`. `SetPlan` and `set_design` are reached through it. The Game Maker analogue in the server is `implementationTestRunner` (`internal/server/game_maker_implementation_test.go:24-26`), which runs `gameMakerAgentRunner` with a fake provider.

Tool-scope enforcement is in `internal/agent/action_policy.go:321`. The private-execution prefix test is `game_maker_` or `cyd_creator_`; the spec's `:314` is stale. The scope flag is set in `agent_loop.go:302` (`ToolScopeRestricted: s.runCfg.AllowedTools != nil`). There is no `storymaker_` anywhere in `internal/agent` yet, so Storymaker will need these same three hooks.

## 4. Text tool mode testing

- The replay test sets `UseNativeFunctions=false` and asserts no native tool schemas were sent (`game_maker_text_mode_replay_test.go:144-145`).
- Text parsing is tested through `ParseToolCall` on the nudge example (`game_maker_recovery_nudge_test.go:118-130`) and in `native_tools_game_maker_test.go:60-70` (XML `<tool_call>` and `<invoke>` forms, with routing args).
- Scope refusals in text mode are tested in `game_maker_operation_alias_test.go:245-256` (expects `tool_scope_denied`) and `canonical_action_test.go:336-338`.

## 5. Known-red tests on main (October reports)

All three names still exist, so the list may still hold. I did not run them.
- **ExecManage**: `internal/agent/agent_dispatch_exec_test.go:20`, `:519`, `:558`, `:587`.
- **ExecutionOutputIsolation**: `internal/agent/tool_execution_policy_test.go:35` (`TestExecutionOutputIsolationSurvivesVaultAndCompression`).
- **ToolSearchTerms qr_code**: the name `qr_code` no longer appears in `internal/agent/tool_search_terms_test.go`. The test only checks format and coverage of every builtin tool (`:17-42`, `:82`). The QR case is probably covered generically, but I did not confirm whether `qr_code` is a builtin name.

Pass or fail status is unverified.

## 6. Storymaker test helpers (no agent yet)

**Server fixture.** `storymakerAPIFixture(t)` (`internal/server/storymaker_api_test.go:57-77`) does the following:
- Calls `testDesktopPermissionServer(t)` (`desktop_security_test.go:140`), which creates a vault and token manager.
- Creates a write token with scopes read and write.
- Calls `configureDesktopAppsHandlerTest` (`desktop_apps_handler_test.go:73-87`), which sets all data and workspace directories under `t.TempDir()`.
- Enables Storymaker, sets `storymakerUI = os.DirFS("../../ui")`, and sets `storymakerBrowsers` with a fake Chrome probe.
- Registers `closeStorymaker` cleanup.

**Seams on Server** (`internal/server/server.go:377-386`):
- `storymakerRunOverride`, `storymakerAudioRunOverride` (type `render.RunFunc`)
- `storymakerSynthOverride func(ctx, tools.TTSConfig, text) ([]byte, string, error)` for narration (used in `storymaker_narrate.go:210-213`)
- `storymakerFreeDiskOverride func(path) (int64, error)`
- `storymakerMusicOverride *storymakerMusicSeam` for music

**Narration env.** `newNarrateJobEnv` (`storymaker_narrate_test.go:81-92`) sets a free-disk override and creates a project. `env.run` (`:118-134`) enqueues on `rt.narrate` and polls for 20 s.

**Queue fixture.** `storymakerQueueFixture` (`storymaker_api_test.go:1100-1111`) installs run overrides, then `enqueueStorymakerJob` and `waitStorymakerManagerJob` (10 s poll) run jobs without FFmpeg or a browser.

**Render manager.** `render.RunFunc func(ctx, job Job, payload any, h *Handle) (Output, error)` (`internal/storymaker/render/jobs.go:193`). `NewManager(ManagerOptions{Dir, Run, MaxQueued, Retention, Logger, Kinds, Workers, NewID, Now})` (`:212-257`), `Enqueue(EnqueueRequest{ProjectID, Kind, Coalesce, Payload, ...})` (`:426`). Workers must be 0 or 1.

**Voice and TTS.** The `storymakerTTSConfig` and voice path are in `storymaker_narrate.go:174`. `storymakertest.SeedNarration(ctx, p, id, s, seconds)` (`internal/storymaker/storymakertest/seed.go:29`) writes a 24 kHz tone WAV and envelopes so a story is narratable with no provider.

**MemStore.** `internal/storymaker/memstore.go`: `NewMemStore()` is an in-memory implementation of `storymaker.Store` (`project.go:46`), built for tests of that package and its dependents. `NewProjectStore(st Store, root)` is at `project.go:107`. The server tests use the on-disk store, not MemStore. I did not confirm whether any server test uses MemStore.

**Test-only `storymaker` server runtime.** `storymakerRuntime()` (`storymaker_service.go:63`) needs `Cfg.Directories.DataDir`, which the fixture sets.

**Duration.** Narrate waits are capped at 20 s, and queue waits at 10 s. Full duration is unverified.

## 7. Real-model acceptance and aurago-test

**Repo docs.** No repo doc describes the aurago-test providers. `documentation/local_llm_aurago_qwen.md` covers the local Qwen runtime only. Nothing under `docs/` mentions aurago-test.

**User memory (6 to 33 days old, unverified against current state).**
- `reference_aurago_test_server.md` says the host is `ssh aurago-test` at `/home/aurago/aurago`, with a `bin/aurago_linux` binary and logs in `log/`. Game Maker validation there needs an open, logged-in Studio preview, so the user must start measurement runs in the UI.
- `reference_aurago_llm_request_forensics.md` (2026-10-03) says the main provider was `stepfun` (`step-5-preview`), with Qwen relay `obit`. The user switches providers in the UI, so `llm.provider` must be checked first.
- `reference_game_maker_job_forensics.md` says job outcomes live in `data/game_maker.db` on aurago-test, with tables `gm_jobs`, `gm_events` (`tool_result` payloads) and `gm_agent_context.messages`. Read them with `sqlite3 -readonly`. Scripts exist under `reports/game-maker-forensics-2026-10-26/`.

**Job outcome in code.** `internal/gamemaker/job_outcome.go`: `applyJobOutcome(payload, job)` (`:142`), `finishFailure` (`:177`), `finishReady` (`:187`), `LatestJob` (`:254`) and `LatestPlanSummary` (`:266`). The spec's `:158-181` is for model-prose release, which I did not check.

## 8. Minimal scripted acceptance test for "topic in, valid story out"

Built from pieces that exist:
1. Create an `httptest` provider or a `circuitBreakerSequenceClient` (`:307`) with scripted tool calls. Calls should be `storymaker_story {outline}`, then `storymaker_catalog`, `storymaker_story {put_scene}`, `storymaker_check`, then a plain-text end.
2. Build a `storymaker` store and runtime with `storymakerAPIFixture`, and fake narration with `storymakerSynthOverride` or `storymakertest.SeedNarration`.
3. Run the job through the render manager with `render.NewManager` and a `RunFunc`, and wait with the `waitStorymakerManagerJob` pattern.
4. Assert the story validates with `storymaker.DecodeStory(data, DefaultLimits())` and `ApplyDefaults` (as `narrateJobEnv.missing` does, `storymaker_narrate_test.go:136-153`), the job outcome is `ready` or `ready_unverified`, and no `tool_scope_denied` appears.
5. Skip browser and FFmpeg: the spec's check phase needs headless Chrome, so gate it with `AURAGO_RUN_BROWSER_SMOKE=1` (spec line 806).

## 9. Reuse, copy, do not copy

- **Reuse as is:** `circuitBreakerSequenceClient`; the `httptest` provider pattern; `storymakerAPIFixture`; `storymakerQueueFixture`; the `Storymaker` seams `storymakerSynthOverride` and `storymakerFreeDiskOverride`; `storymakertest.SeedNarration`; `render.Manager` with `RunFunc`.
- **Copy the shape:** the replay fixture format, with a recorded `sequence` and a `requireTextModeWithoutDenial`-style refusal scan. The three-reply padding in `replayGameMakerPlanning`. The `runner` pattern that forwards a stage check into `ExecuteAgentLoop`.
- **Do not copy:** the 20 s and 10 s wall-clock polls, which make tests flaky under load. The `glm-4` model pin and `"Planning is done."` padding, which are Game Maker-specific. `game_maker_model_selection_test`'s reliance on `mockChatClient` globals. Do not depend on the aurago-test memory notes, which are stale.

## Unverified

- Test durations and whether the listed known-red tests still fail.
- Whether `qr_code` is still a builtin tool and covered by the generic test.
- Whether any server test uses `storymaker.MemStore`.
- Spec citations `tool_scope.go:108`, `agent_dispatch_exec.go:226-234`, `dispatch_platform.go:312-313` (the last is now `:307`) and `game_maker_implementation_stream.go:80` (confirmed: `ToolScopeRestricted: true` at `:80`).
- Current aurago-test provider config (memory only).
