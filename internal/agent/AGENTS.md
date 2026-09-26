# Agent runtime and tool dispatch

## Purpose

Runtime prompt, tool-discovery, dispatch, and context rules.

## Ownership

`internal/agent` owns this domain. The contracts below also bind related server, UI, config, asset, and test work through the root routing table.

## Local Contracts

- Interactive streamed agent turns send a transient localized `progress` event
  after five seconds or when starting a fourth tool step, whichever comes first,
  then at most every 30 seconds until completion. Respect `agent.workflow_feedback`;
  missions, co-agents, maintenance and autonomous runs stay silent. Progress
  never enters the assistant answer, tool result or persistent history, and
  cancellation/completion stops it. Count queued and batched tool calls too.

### Tool System

- Server-published configs bind `AuthorizationSnapshots` before publication.
  Scoped/delegated copies retain that runtime-only resolver. Dispatch intersects
  Enabled/Allow/Sudo grants and ReadOnly restrictions immediately before execution;
  new grants never widen an existing run. Compound allow/block-list and integration
  membership changes require a new request. Hooks with tightened captured gates
  require a fresh runner. Provider/model and native-schema snapshots stay fixed.
  Keep authorization field naming and coverage in `live_tool_authorization.go`
  synchronized when adding policy settings; Security filesystem access is a
  positive read-operation allowlist, including aliases and editor tools.
- Discovery belongs to an owned run ID, released on completion/cancellation;
  active runs cannot expire through orphan-cache pruning. Refresh the catalog
  against the actual scoped and budget-fitted request, including the lightweight
  Looper. Tool-free callers stay tool-free. `invoke_tool` resolves before policy,
  task rules, hooks and effect tracking and executes the real handler only once.
- Catalog IDs preserve `skill__`, `tool__`, `package__` and deterministic MCP
  namespaces. Bare aliases work only when unambiguous. Search/category/family
  pages are bounded summaries; `get_tool_info` returns one complete schema and
  `get_manual` reads revision-bound pages. Never byte-slice structured results.
  Detail schemas use `agent.tool_output_limit` and the remaining request capacity
  of every eligible route; report output and context limits separately.
  Missing local dependencies are `needs_setup`; account/connection state is
  distinct from configuration and never inferred from an enabled switch.
- Execution status is captured before scrubbing/compression and kept separately
  from display text. Only confirmed success supports learning, issue resolution
  and context invalidation. Preserve external-data boundaries through compression.
  Integration formatters capture trusted local envelope status before escaping;
  external payload text cannot establish status. Text-mode prefixes and trailing
  security guidance survive formatting so history grouping remains atomic.
  MCP transport failures never automatically replay an already-sent tool call.
  Discovery follows all pages atomically; notifications invalidate cached tools,
  and partial server failures must not erase healthy catalog results.
- Workflow guides are atomic optional prompt ledger sections in every tier.
  Native schemas do not replace workflow guidance. Share manual family bindings,
  validated disk/embedded sources and content digests with the search index.
  Optimizer variants are chosen before fitting; only delivered, fitted guide
  exposures with matched source, canonical action/target and operation evidence
  support promotion. One request/guide/action/operation contributes at most one
  observation; legacy traces without action identity retain provenance but do
  not establish exposure. The optimizer worker follows live enable/disable state.
- Configuration migrations share `NormalizeToolDisclosureConfig` across load,
  save, API and config-merger. Canonical values win; explicit false/empty lists
  stay authoritative. Removed controls are rejected in stale browser patches.
  Runtime metadata from GET `/api/config` must never enter editable drafts.
  Agent concurrency and Python skill-manager lifecycle changes require restart.
Tools are defined in `internal/tools/`:
- Each tool has a JSON schema definition
- Tools are registered in the tool registry
- Native OpenAI function calling format
- Dynamic tool creation supported (agent writes Python tools)
- `invoke_tool` may route any enabled native tool through its real handler, including an active catalog entry when the current model/runtime cannot emit the direct structured call. It must continue to reject disabled tools and self-invocation.
- Resolve `invoke_tool` before task rules, role/scope policy, hooks and effect tracking; preserve the transport call ID. Hook handlers must not bypass role policy. Agent Skill scripts obey `AllowedAgentSkills` just like activation. Dispatch, Guardian and scrubbing run once for the effective action.
- `ToolDispatchResult.Status` is captured before output sanitization and compression. Denied, setup-required, cancelled, deferred and unclassified results are not confirmed successes. Success learning, context mutation and issue resolution require a confirmed success; legacy string handlers must expose a recognizable result envelope before participating.
- The `call_method` returned by `discover_tools` is binding. Use `invoke_tool` immediately when requested; `activate_tools` must reject any tool for which discovery did not explicitly return `activate_tools`.
- Generated Virtual Desktop apps use the advertised `virtual_desktop_app_install` tool with one complete manifest-and-files payload; `virtual_desktop_apps(operation=install_app)` remains dispatch-compatible but is not advertised. Existing workspace files are never implicit install inputs.

### Prompt and Runtime Drift Contract

- Optional `RunConfig.PreparedPrompt` (also in `MinimalLoopOptions`) owns a
  complete immutable system prompt and ordered schemas. Its revision covers
  instruction bytes and schemas. Nil keeps the normal dynamic builder. Never
  add runtime memory, time, guides or mutable addenda to a prepared profile;
  append observations to the conversation. Required schemas and instructions
  fail closed when they cannot fit; current route budgets, prompt security,
  hard tool scope and live dispatch authorization still apply on every send.
- Prepared runs use bounded request-view compaction without helper LLM calls;
  private checkpoints retain full history and required reasoning. The optional
  `PromptUsageObserver` stores metadata/hashes only, distinguishes local prompt
  reuse from measured provider cache reads, and records route/profile/prefix or
  history changes as new context generations. Missing usage is unknown. Capture
  individual HTTP attempts, merge cumulative streaming counters by maximum and
  include Anthropic cache reads/writes in total input. Cached input still consumes
  the full context window. Only exact, known metered API prices justify estimates.

- Persistent history compression must make bounded progress through oversized
  conversations at complete tool-round boundaries. Preserve the current human
  request, pinned records and the two newest native tool rounds.
- Workspace asset fingerprints cover runtime sources, patches and dependencies,
  with normalized LF endings, excluding Go test files. Legacy compatibility
  must be bound to a verified exact runtime fingerprint, never a blanket bypass.
- PromptSec structure provenance must come from guard metadata, never a textual
  comparison with the current dynamic system prompt. A full structure envelope
  cannot replace a chat user message. Chunked security scans provide diagnostics
  only; a partial scan window must never replace the complete human request.
- Prompt logs must include provider/model, build and VCS identifiers, prompt revision, sorted active tools, tool-catalog hash, and recovery counters.
- `/api/system/info` exposes the running build identifier and VCS metadata. Deployment acceptance requires its `build_id` to match the reviewed commit; a `-dirty` identifier is not a clean release artifact.
- Every LLM request must fit every eligible primary/failover route after reserving output and protocol safety tokens. Resolve limits in this order: provider override, model registry, cached provider probe, configured global cap for an unknown primary model, then conservative 32768/4096 defaults; `agent.context_window` is always an upper cap.
- Account for the StepFun wire projection before fitting each route: unsent
  reasoning and empty reasoning-only assistant messages consume no request
  tokens. Keep private messages intact and still account for required reasoning
  on other eligible routes. Working-set, minimum-request and final accounting
  use the same projection. Verify `TestRequestBudgetStepFunProjectionPreservesOtherRoutes`.
- Carried chat history is additionally limited to `min(route history capacity, clamp(70% of model context, 65536, 131072))`. The current genuine user request is exempt from that history cap, the newest two tool rounds stay atomic and complete, older rounds compact first, and any automatic conversation recall is untrusted, capped at four same-session matches from Activity FTS plus active and archived messages and 4096 tokens.
- Model registry files are generated artifacts. Refresh `models.dev` and `@oh-my-pi/pi-catalog` only through their `--write` generators, verify them with `--check`, retain upstream version/hash provenance, and surface `metadata_source` in limit diagnostics. Recompute route warnings after provider/model changes; an intentional global cap is informational and not an unknown-model fallback.
- Helper and KG JSON completions request structured output only for supported routes, reserve reasoning output within effective provider/context caps, strip fenced/thinking wrappers defensively, reject empty/invalid/truncated (`finish_reason=length`) output before caching, and retry once with a smaller unit of work. Helper singleton identity includes provider limit overrides and the global context cap. File KG replacement is all-or-nothing across extraction segments.
- Prompt Markdown without frontmatter remains a compatible plain source. A source that starts frontmatter but cannot parse is rejected in root modules, fallback identity/rules, personalities, tool guides, and delegated templates; a malformed disk override falls back to its valid embedded source. Prompt caches bind the selected source to a content digest so corrections are detected even when timestamp and size are unchanged.
- `RunConfig.UserIntent` is the immutable human intent for one tool chain. Tool outputs remain in model history but must not retarget RAG, KG, dynamic guides, coding mode, task rules, or specialist selection. Recompute only explicitly invalidated turn-snapshot categories after successful mutations.
- `RunConfig.TrustedPromptAddenda` is internal trusted execution context only. A2A and co-agent contracts are fitted as atomic required ledger sections into the generated prompt so nested Markdown headings cannot make part of an addendum optional; delegated requests have one leading system message, while user and external content remain in user messages or isolated context blocks.
- The Writer specialist's `additional_prompt` defaults to compact multilingual Humanizer guidance. Keep `internal/config/config.go` and `config_template.yaml` synchronized; explicit empty/custom values remain authoritative. Preserve source claims, quotations, technical literals, requested voice and format; apply style cleanup silently. Attribution and license live in `THIRD_PARTY_NOTICES.md`.
- Go-built `PERSONA STATE` directives may guide tone and working style only. LLM-generated emotion descriptions and inner-voice values stay untrusted advisory data isolated with `<external_data>` and may affect tone only. Neither path may change user intent, safety, or tool policy. Valence, arousal, and mood are owned by the Go affect integrator; helper/synthesizer output may narrate and move those numbers by at most a small clamped delta. Autonomous runs may emit affect events but must not inject persona side-effects into chat. Lived character notes are a trusted, clamped, user-reversible ledger distinct from Core Memory and user profile; helper reflection may propose at most two notes per day and must not restore a user-deleted note. Channel style may shorten or relax tone only; high thoroughness may add one verify step, never a free tool-call multiplier; destructive confirmation requires low confidence and an ambiguous target. Co-agents receive at most one temperament line and never inner voice or character notes. Dashboard and Config expose the affect timeline, last events, and lived notes; Config warns when V2, emotion synthesis, or notes need the Helper LLM. The lightweight Looper omits memory/RAG/personality but still uses route-aware fitting, atomic history trimming, final validation, and prompt logging on every LLM round.
- Built-in persona bodies must fit the 1000-rune core-profile budget without truncation. Keep their voice recognizable in brief and technical replies; mood and channel modulate that voice rather than replacing it. Decode omitted metadata from `memory.DefaultPersonalityMeta()`; explicit zero volatility, empathy and loneliness modifiers remain zero. Missions and co-agents exclude the full persona and dynamic persona sections; delegated temperament remains separately bounded. Persona selection publishes a new config snapshot only after a successful save. Custom profile saves validate frontmatter before replacing files atomically and create the profile directory on first use; plain Markdown stays supported.
- The selected compact persona and Go-built `PERSONA STATE` are required prompt sections; keep profile Markdown atomic through `TURN CONTEXT`. Optional emotion narration and character notes may be shed first. Apply a human message's affect once per run, and prepare its local mood before the first reply without another blocking model call. Synthesizers share continuity, cooldown and in-flight reservations through the owning SQLite memory store and restore the latest persisted state after restart. Persist bounded semantic affect, mood and emotion history atomically before publishing their shared snapshot; subsequent overlays must retain the accepted numerical contribution. Creative/analytical modes and positive feedback must remain visible in trusted tone guidance. Standalone synthesis uses route-aware JSON output budgets and rejects truncated completions.
- Personality dynamics share the owning SQLite store and activate with the existing engine. `ApplyPersonalityObservation` owns atomic affect/dynamics/trait/history updates and durable observation receipts; semantic enrichment must retain its originating snapshot and cannot replay a primary event. Technical events affect load only, never familiarity or friction. Preserve the four-hour affect, twelve-hour load and twenty-four-hour friction half-lives, bounded family habituation, explicit zero modifiers, and two-event emotional hysteresis. Reads never advance state. Persona changes and resets invalidate pending analyses; resets preserve traits, familiarity and character notes. Back up existing on-disk personality data before the additive dynamics migration.
- Promptsec structure guards are request-local and must never mutate the shared Guardian with a per-request system prompt. Before every send, preserve the newest tool-call reasoning block when any eligible primary or failover route requires continuation reasoning.
- Native multi-tool assistant messages are persisted once. Every declared tool-call ID receives exactly one contiguous tool result before recovery or circuit-breaker system guidance is appended; sanitization remains defensive, not normal control flow.
- An exact-duplicate tool circuit breaker terminates that tool chain. Persist the blocked result, emit `not_executed_due_to_circuit_breaker` for every remaining declared native call without dispatching it, remove tools, and request exactly one tool-free final response.
- Internal chat control headers are trusted only from loopback with the process token. A mission ID additionally requires `X-Internal-FollowUp: true`; invalid internal headers fail with `invalid_internal_chat_headers` before normal authentication handling.
- Persisted and manual history compression share the coordinated per-session path and apply one stable-ID update to the summary, in-memory history, and SQLite archive. Summaries are capped at 8192 tokens; failed summaries must leave raw data intact and fall back to bounded request-only recaps. Hard truncation must preserve valid UTF-8 and the final token limit.
- `HistoryManager.CurrentSummary` belongs exclusively to chat context compression. Nightly maintenance must not use an LLM reflection loop to overwrite it; maintenance state is stored in the structured maintenance ledger and typed morning notification instead.

## Verification

- Run `go test ./internal/agent` and the named cross-component checks in the contracts above when those paths change.

## Child DOX Index

None.
