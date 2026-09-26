# Optional task LLM router

Status: proposed implementation plan; no runtime implementation or activation.

## Product contract

Add an optional router that selects the provider/model for an incoming agent
task. It is disabled by default. The nine configurable areas are `general`,
`easy`, `normal`, `complex`, `coding`, `research`, `creativity`, `security`, and
`writing`. Each area uses the existing provider system, with an optional model
override. An unassigned area uses the model that would otherwise serve this
request, including its existing failover behavior.

The router selects a model for one agent run, including its tool rounds. It
does not change the globally configured model. A later human message can select
a different model. Existing tool grants, Guardian checks, specialist roles,
personality, conversation identity, and history ownership remain authoritative.

The implementation must work without a Helper LLM. An explicitly enabled and
configured helper may resolve uncertainty when it can change the selected
target, within strict call, token, and latency limits.

## Categories and overlap

Keep the requested nine assignments in the UI. Internally, classify two axes:
domain (`general`, `coding`, `research`, `creativity`, `security`, `writing`) and
complexity (`easy`, `normal`, `complex`, `unknown`). This avoids conflating
"complex coding" with a choice between unrelated categories. It does not add a
domain-by-complexity configuration matrix in version one.

| Area | Primary intent | Example |
| --- | --- | --- |
| General | Conversation, open discussion, no specific task type | "How are you?" |
| Easy tasks | One clear, bounded operation or simple factual answer | "Convert 25 minutes to seconds." |
| Normal tasks | Routine work with several clear steps | "Sort these appointments and flag overlaps." |
| Complex tasks | Cross-domain planning, interdependent decisions, substantial reasoning | "Plan a migration with dependencies, risks, and rollback." |
| Coding | Implement, debug, test, or refactor software | "Fix this Go function and add a regression test." |
| Research | Find, compare, and verify information or sources | "Compare these approaches using primary sources." |
| Creativity | Develop original ideas, stories, concepts, or alternatives | "Invent three concepts for a cooperative game." |
| Security | Assess vulnerabilities, threats, permissions, or hardening | "Audit this login flow for vulnerabilities." |
| Writing | Draft, revise, translate, or structure a text | "Rewrite this guide clearly in German." |

Classification follows the requested outcome, not isolated vocabulary. Writing
a report about an already supplied security review is usually Writing;
performing that review is Security. Implementing authentication is normally
Coding. A creative story is Creativity; correcting its grammar is Writing.
Quoted code, URLs, document titles, and prompt length alone are weak signals.

Selection rules:

1. An explicit model pin or dedicated workflow provider wins.
2. A confidently identified specialist domain selects that domain's area.
3. Without a specialist domain, an actionable task selects Easy, Normal, or
   Complex when its difficulty is sufficiently clear.
4. Confidently conversational/general requests select General.
5. Unresolved ambiguity selects the ordinary default route.

Resolve the category before looking at its assignment. If Coding is detected
but unassigned, use the ordinary default directly, even when Complex or General
has an assignment. General is an actual category, not an extra fallback layer.
An empty configuration must preserve existing routing exactly.

## Scope and switching lifecycle

- Version one covers ordinary interactive main-agent tasks across supported
  chat channels, including the desktop. Inventory every entry point; do not
  implement this only in the Web Chat handler.
- Dedicated provider selections in Speech Lab, SIP, Game Maker, Detective,
  prepared runs, A2A, missions, and specialists remain binding. These workflows
  are excluded from automatic routing in version one. Maintenance, helper,
  summarization, embeddings, vision utilities, TTS, and ASR are also excluded.
- Introduce trusted run routing metadata (`auto`, `pinned`, or `off`) and an
  immutable resolved decision. Classifier output cannot set the routing mode.
- Resolve once before model-dependent prompt/schema construction, image
  promotion, minimum-context preflight, and persistence of accepted input.
  Shared preparation is idempotent: preflight and execution reuse one decision
  and never call the helper twice. An early loop guard supports entry points
  without an HTTP preflight, before `initAgentLoopState` runs.
- Snapshot the ordinary route and routing configuration at task start. Publish
  config changes atomically for subsequent tasks. Current runs retain their
  provider/model snapshot while live authorization continues to tighten grants.
- Freeze the category throughout tool rounds, retries, and resumptions of the
  same task. Tool output cannot change it. Reconsider after a new human message.
- Short continuations such as "continue" may reuse the previous same-session
  decision when the task and configuration revision still match. A topic
  change or stronger new intent invalidates it. Never inherit another session's
  decision. On completion, release run resources; retain only bounded cache
  metadata. A restart safely returns to fresh classification.

## Local classification and sparse helper use

Use bounded deterministic rules first. Inputs are the immutable human intent
(`RunConfig.UserIntent`), trusted workflow metadata, attachment types, and
explicitly scoped previous human intent for a continuation. Reuse existing
intent signals only after testing their suitability. Do not read the entire
conversation, tool output, retrieved documents, or model reasoning to decide
which provider should receive the task.

Local features combine action/outcome phrases, code structure, requested
verification, dependencies, and ambiguity. Support German and English first;
unknown language or weak evidence must remain uncertainty rather than a
confident keyword guess. UI translations still cover all sixteen locales.
No extra network classification, embeddings call, model download, or training
pipeline is required for version one.

Invoke the existing helper only when all conditions hold:

- Router and helper fallback are enabled; `ResolveHelperLLM` reports an
  explicitly enabled, usable helper. Never fall back to the main LLM for
  classification.
- Local evidence is ambiguous, and plausible categories resolve to different
  eligible targets. If every plausible outcome uses the same default/model,
  skip the helper. No custom assignments means no classification work at all.
- No valid cache entry or identical in-flight classification is available.
- A helper slot, routing quota, and the existing global budget permit the call.

Recommended initial limits, to validate before release:

| Limit | Default proposal |
| --- | --- |
| Physical completion attempts per new task | 1; no retry |
| Total helper deadline | 1,500 ms, including acquisition and transport |
| Input | At most 768 tokens including instructions and excerpt |
| Total generated tokens | At most 256, including any reasoning |
| Per-session cooldown | 60 seconds |
| Installation quota | Token bucket: 20 calls/hour, burst of 2 |
| Decision cache | In memory, 256 entries, 10-minute TTL |

A busy helper is skipped instead of queued. The request is cancelled when its
deadline or parent task ends; late results cannot change an active run. These
limits bound local work and attempts; provider billing for a request already
received may still occur after cancellation.

Reuse the helper manager's provider resolution, normalization, concurrency,
and accounting, but add an operation-specific request policy. Its existing
generic retry loop must not turn one routing decision into several calls.
Keep existing retry behavior for other helper operations. If a reasoning model
requires a minimum output reserve beyond this operation's cap, skip it rather
than silently raising the cap. Count physical attempts and reported usage,
including failures where usage exists, in a separate routing operation/category.
Unknown price or usage stays unknown; do not report fabricated savings.

The helper returns a small validated object containing domain, complexity, and
an enum reason/uncertainty indicator. It receives a bounded, scrubbed intent
excerpt wrapped as external data, no tools, and no provider credentials. It
cannot return a model name, URL, config patch, permissions, or tool action.
Reject unknown enums, extra control fields, empty/invalid/truncated output,
and unresolved ambiguity. Failure immediately selects the ordinary default.
Validate the category schema before caching; generic JSON validity is not enough.

Cache keys include session, normalized intent digest, relevant attachment and
continuation context, rule version, config revision, and helper identity.
Store only decisions and bounded metadata; do not persist raw prompts. An exact
cache hit still revalidates current route readiness. Thresholds are heuristics,
not calibrated probabilities; tune them on development fixtures only.

## Provider selection, compatibility, and failure handling

Resolve a `RoutingDecision` into a task-local config view, matching client,
model, and eligible failover candidates. Preserve the config's runtime-only
authorization resolver and explicit caller tool scope.

The ordinary default means the caller's route without the router, not a copied
model ID saved in the router configuration. Its active failover state and
explicit settings are respected. Changing the user's normal model therefore
changes fallback behavior for the next task automatically.

Check credentials/readiness, chat capability, required tools, images, structured
output, provider reasoning rules, and context/output limits. Resolve metadata
for the exact provider/model pair, including model overrides. Managed local
models retain their qualification and attestation gates. Routing must never
download, start, or switch a managed model family as an implicit side effect.

Fallback order for an automatically routed task is the chosen target, then the
ordinary route and its configured eligible failover. Deduplicate identical
targets and bound the chain to those candidates. Incompatible or unavailable
targets are removed before sending; record a sanitized fallback reason. An
unassigned area enters the ordinary path immediately. A broken selected target
must not prevent a usable default from answering.

Use `RequestRouteProvider.CandidateRoutes` so prompt fitting covers every route
that may actually receive the request. If the selected target cannot hold the
required user input, instructions, schemas, and output reserve, abandon that
selection and rebuild/finalize for the ordinary plan before any send. Never
drop required content to force a cheaper target, or send to an undeclared
fallback. If no permitted route fits, return the existing explicit context or
capability error. Preserve complete tool rounds and native reasoning where
required; request-view compaction must not destroy persistent conversation data.

Routing fallback is only for eligible provider/transport failures. Cancellation,
authorization denial, and exhausted budgets are terminal. Keep existing bounded
retry semantics and a common deadline. Once answer content is streamed, a
provider switch must not replay the answer or tool effects. Preserve existing
complete-stream checks and recovery rules.

Reuse provider clients and existing transport/auth behavior. Do not reconfigure
the shared `FailoverManager` on every task or create a new probing goroutine per
message. A small lifecycle-owned route service should cache clients by full
route/config identity and provide immutable run views; stop retired resources
on reconfiguration/shutdown. Health state is isolated by provider/model identity.

## Proposed configuration

Keep a separate top-level section to avoid changing existing LLM defaults:

```yaml
llm_router:
  enabled: false
  helper_fallback: true
  helper_timeout_ms: 1500
  helper_max_calls_per_hour: 20
  areas:
    general:    { provider: "", model: "" }
    easy:       { provider: "", model: "" }
    normal:     { provider: "", model: "" }
    complex:    { provider: "", model: "" }
    coding:     { provider: "", model: "" }
    research:   { provider: "", model: "" }
    creativity: { provider: "", model: "" }
    security:   { provider: "", model: "" }
    writing:    { provider: "", model: "" }
```

- Empty provider/model: use the ordinary default. Provider plus empty model:
  use that provider entry's model. A model without a provider is invalid.
- Model overrides use the existing provider/model picker and capability
  resolution; custom model entry follows the existing provider UI rules.
- Add typed config, normalization, defaults, validation, template, save/load
  and config-merger coverage. Preserve explicit `false` and cleared assignments.
- Validate timeout within 250-5,000 ms and quota within 0-120 calls/hour; zero
  quota disables helper calls. Omitted fields use defaults. Activation with no
  assignments is valid and reports that the default model serves all requests.
- Invalid assignments are rejected on save with field-specific errors. On
  startup, stale assignments from removed providers are inactive with a visible
  diagnostic and ordinary fallback. Provider deletion must surface router
  references through the existing provider-reference checks.
- Reuse Vault/OAuth provider credentials. No keys, endpoints, or resolved secret
  fields belong in `llm_router`, API responses, logs, or routing caches.
- Version one needs no new database or history migration.

## Configuration and diagnostics UI

Add an optional "LLM Router" section beside the existing LLM/provider settings.
Use an activation toggle and nine rows with area, provider/model selection, and
effective target. Every row offers "Use current default" and displays its
current effective provider/model. Explain General versus fallback in place.

Show helper fallback as a separate toggle with the short explanation that it
only resolves uncertain assignments and may add cost/latency. If the configured
helper is disabled or unavailable, local routing remains usable; show that
state. Put timeout and hourly quota in advanced options. Do not hide an invalid
saved provider by silently rendering it as a valid empty selection.

Provide a bounded preview using the saved configuration. Its default action
performs local classification only, reports the chosen area/target/reason, and
states whether a helper would be useful. A separately labeled helper preview
uses the same quota and deadline and may consume tokens. Both preview paths
are admin-only, same-origin POST operations, execute no task or tool, do not
change global/session routing, and do not persist test text. Unsaved edits must
be saved before testing, matching existing config behavior.

Publish sanitized per-turn metadata: area, decision source (`rules`, `cache`,
`helper`, `default`, `pinned`), selected and actual provider/model, elapsed
classification time, and fallback reason. Correlate it with the originating
session/turn; never broadcast private session data across channels. Web Chat
and Desktop can show a compact informational badge such as
"Coding · Provider / Model". Display the actual serving model after failover.
Other channels retain ordinary reply text.

Dashboard counters show decisions by area/source, helper attempts/cache hits,
timeouts/fallbacks, and measured helper usage. Label memory-only counters as
"since startup". Do not include prompt excerpts or invent cost savings.
Translate all new Config, Chat/Desktop, and Dashboard strings into all sixteen
supported locales. Preserve drafts, accessible labels, pending/error feedback,
themes, and narrow layouts; rebuild canonical bundles and verify assets.

## Integration map

The following anchors were checked in the current source. Names of new files
and APIs below are proposed, not existing functionality.

| Area | Existing anchors and planned change |
| --- | --- |
| Configuration | `internal/config/config_types.go`, `config.go`, `config_migrate.go`, `config_template.yaml`; add a focused `llm_router.go` for validation/defaults |
| Pure classification | New `internal/llm/task_router.go` and `task_router_rules.go`; domain/complexity, effective assignment resolution, bounded cache and testable policy |
| Route clients | `internal/llm/interface.go`, `failover.go`, `client.go`; add a task route client implementing `ChatClient` and `RequestRouteProvider` |
| Agent preparation | `RunConfig` in `internal/agent/agent.go`, `ExecuteAgentLoop`, `initAgentLoopState`, `request_budget.go`; add `internal/agent/task_router.go` for shared idempotent preparation |
| Helper adapter | `internal/agent/helper_llm_manager.go` and `internal/llm/helper_resolver.go`; dedicated bounded classification operation, schema validation, no main-model fallback |
| Chat entry points | `internal/server/handlers.go`, desktop/AgoDesk preparation, Telegram/Discord and other main-agent adapters; route before provider-dependent preprocessing/preflight |
| Fixed workflows | Speech Lab/SIP, Game Maker/Detective, co-agent/specialist, mission/prepared/A2A entry points; explicitly mark routing as pinned/off and test it |
| Config/API | `internal/server/config_handlers_main.go`, `provider_handlers.go`, provider-reference diagnostics; preview and sanitized router status using existing auth/save mechanisms |
| UI | `ui/js/config/main.js`, proposed `ui/cfg/llm_router.js`, shared model picker, Chat/Desktop feedback, Dashboard, sixteen locale sets and canonical bundles |
| Accounting | Existing agent telemetry, request usage observation and budget tracker; add routing-operation usage and actual-route metadata |

Two integration constraints need explicit regression coverage:

1. `FailoverManager.CreateChatCompletion` and its streaming counterpart replace
   `req.Model` with their active model. Changing the request string alone cannot
   implement this feature; config view, selected client, and route metadata must
   agree before prompt preparation.
2. `handlers.go` currently promotes images and calls
   `ValidateMinimumRequestBudget` before `ExecuteAgentLoop`. Routing exclusively
   inside the loop would validate/preprocess against the wrong model. Move the
   shared preparation boundary earlier without consuming one-shot or history
   state before preflight succeeds.

## Implementation sequence and acceptance

1. **Configuration and pure policy.** Add types/defaults/validation, the nine
   assignments, two-axis rules, deterministic fixtures, and effective-target
   resolution. Verify disabled/empty configuration, clears, invalid references,
   category overlap, and exact ordinary fallback.
2. **Task-local integration.** Add the route service and immutable decision;
   wire shared preparation before preflight/init, enumerate main-chat callers,
   and mark fixed workflows. Verify two concurrent sessions use different
   models without global mutations, and tool rounds/config reloads preserve
   the captured route. Test provider failure, context, multimodal, native tool,
   reasoning, cancellation, streaming, and exactly-once effects.
3. **Sparse helper fallback.** Add the operation-specific policy, validated
   response, cache/deduplication, quota/cooldown, timeout and budget accounting.
   Verify one physical call maximum, zero calls for equivalent effective
   targets or disabled/unavailable helper, no retry after invalid JSON, no
   late result mutation, bounded reasoning reserve, and fallback on saturation.
4. **UI and observability.** Add assignment controls, preview, per-turn metadata,
   measured counters, all locale strings, canonical bundles and focused browser
   coverage. Verify enable/save/reload/clear, missing provider/helper, preview
   privacy and authorization, concurrent turn badges, and actual failover model.
5. **Quality gate and documentation.** Run focused Go/config/API tests and
   existing UI/build checks. Run concurrency/race checks in a supported test
   environment. Update owning DOX contracts and operator docs to match the
   implementation. Keep the default disabled and use the normal release path.

Before tuning, freeze a synthetic DE/EN evaluation set with at least 20 clear
examples per area plus 60 overlap, continuation, misleading-keyword, attachment,
and untrusted-text cases. Use separate development fixtures. Report category
precision/recall, automatic coverage, fallback rate, helper call share, actual
usage, and routing latency. Evaluate multiple assignment maps: all default,
partially assigned, and distinct models. A default response on an unclear task
is safe abstention, not correct classification.

Initial release targets: at least 95% precision on automatically classified
clear tasks, at least 80% local coverage of those tasks, helper calls on at most
10% of the balanced evaluation workload, and local classification p95 below
5 ms on documented hardware. Ambiguous and unsupported-language cases must
abstain or use the bounded helper. These are acceptance targets, not measured
results. All-default and router-disabled cases require zero router helper calls
and unchanged target selection. Sparse routing must also demonstrate no prompt,
permission, session, or tool-effect regressions.

Defer custom learned classifiers, automatic model purchasing/downloads, model
quality rankings, per-step rerouting, a domain-by-complexity assignment matrix,
and automatic routing of dedicated background/specialist workflows. Revisit
only after the simple router has measured quality and cost/latency evidence.
