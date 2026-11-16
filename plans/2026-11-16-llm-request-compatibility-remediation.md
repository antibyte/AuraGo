# LLM request compatibility remediation

Status: implemented and locally verified, with unrelated package failures recorded
below. Live provider acceptance and Linux race testing remain pending.

## Goal and scope

Make the main chat, minimal loop, helper calls and fallback routes send requests
that their selected provider/model can accept. Restore complete tool calling and
honest error handling for Copilot Responses models. Fix the confirmed reload,
configuration-error, provider-translation, URL and helper-limit defects.

This plan covers the seven confirmed repair items from the review:

| ID | Priority | Problem | Work package |
| --- | --- | --- | --- |
| F1 | High | Main/minimal chat sends incompatible reasoning-model parameters; route changes retain the original parameter assumptions | W1, W2 |
| F2 | High | Copilot Responses translation loses tools and can report errors as successful text | W2 |
| F3 | Medium | Changing only Anthropic thinking settings does not rebuild the main client | W3 |
| F4 | Medium | Local invalid-provider configuration errors can retry and count toward failover | W3 |
| F5 | Medium | The GLM provider receives MiniMax-specific request transformations | W4 |
| F6 | Medium | A complete Anthropic Messages URL produces a duplicated endpoint path | W4 |
| F7 | Medium | A helper model override inherits limits belonging to the provider's default model | W3 |

Related bounded cleanup: correct the OpenCode endpoint hint and temperature
documentation. Do not add general sampling controls as part of these repairs.

## Implementation decisions

- Keep `llm.ChatClient`, `llm.CompletionStream` and the raw SDK client factories
  compatible. Extend the existing client adapter and route projection rather
  than introducing a second client framework or changing every consumer's API.
- Prepare each physical request after its effective route has been selected and
  before invoking the SDK. An HTTP transport rewrite cannot repair a request
  rejected earlier by the SDK reasoning validator.
- Use the installed `go-openai v1.43.0` Responses support: `CreateResponse`,
  `CreateResponseStream`, `NewResponseFunctionTool` and typed stream events. Add
  only the mapping needed by AuraGo's existing completion interfaces. Do not add
  another SDK or build another SSE parser.
- Share the Responses mapping between native OpenAI and Copilot. Copilot keeps
  its token exchange and mandatory headers. Its non-streaming callers may still
  require an upstream stream, collected through the same Responses mapping.
- Select protocols from the actual provider/endpoint and model. Keep unknown
  compatible endpoints working; do not assume all OpenAI-shaped endpoints offer
  Responses. Unsupported known combinations fail locally with a typed error.
- Canonical model names may be used for capability lookup; preserve qualified
  wire IDs such as `openai/gpt-5` where the selected provider requires them.
- Preserve the existing context/output allowance, including reasoning tokens.
  Main/minimal budgeting already applies the smallest eligible route limit.
  Do not replace it or silently enlarge explicit application output ceilings.
- Preserve tool definitions, forced tool choices and structured-output intent.
  Never regain compatibility by silently dropping tools or disabling reasoning.
- Keep provider credentials, gateway identity, TLS/redirect policy, cancellation,
  retry ownership, usage accounting and configuration generations route-bound.

## Evidence and change risk

The current SDK already exposes native Responses requests and stream events.
`internal/llm/interface.go` is the existing adaptation boundary, but
`FailoverManager` and its probe also call stored `*openai.Client` instances
directly. Updating `WrapOpenAIClient` alone would leave those paths unfixed.

`usageObservationTransport` currently recognizes `/chat/completions` only.
Responses support must preserve measured usage and partial-failure accounting.
The current `ReasoningText` recognizes only the Anthropic envelope; any added
opaque Responses continuation envelope must also be excluded from display.

GitNexus reported **CRITICAL** impact for `WrapOpenAIClient`: 29 direct callers,
79 affected symbols within two hops, across nine modules. This is a lower bound:
the index was 843 commits behind HEAD and its receiver summary was incomplete.
Graft and direct call-site searches were also used. Refresh the affected-call
inventory before implementation; neither graph result is a completeness proof.
Do not rebuild a large index as a prerequisite to writing the fixes.

Before editing production functions, reread all applicable AGENTS guides and
run the required impact analysis. Warn on high/critical risk; confirm unknown or
missing graph callers against current source. Preserve unrelated local changes.

## W1: Prepare requests for the selected physical route

Dependencies: none. Deliver and test before W2.

Primary files:

- `internal/llm/interface.go`, `client.go`, `failover.go`,
  `completion_stream.go`, `task_route_client.go`, `helper_resolver.go`.
- A small request-preparation file in `internal/llm`, if needed to avoid
  duplicating normalization between streaming and non-streaming calls.
- Existing request producers in `internal/agent`, `internal/budget`,
  `internal/memory`, and generation callers discovered by the call-site sweep.

Tasks:

- [x] Give the existing adapter an immutable effective-route snapshot using the
  existing provider projection. Retain SDK factories and `WithClientFactory`
  compatibility for managed runtimes, metadata probes and test injection.
- [x] Route primary, fallback, task-router and generation-probe calls through
  the same preparation path. Reprepare the original logical request for each
  attempted route; do not carry a previous provider's rewritten request forward.
- [x] For known reasoning models/providers, use the supported output-limit
  field and omit unsupported sampling. Reject conflicting or negative explicit
  output limits rather than hiding the error while rewriting them.
- [x] Select a Responses operation before Chat SDK validation where required;
  reject unsupported provider/protocol combinations explicitly.
- [x] Check route compatibility against effective thinking policy and forced
  tool choice. An excluded always-on Claude fallback must not be selected as a
  compatible route. Preserve real local-runtime outage failover.
- [x] Keep request/config snapshots immutable. Replace scattered SDK-validator
  workarounds only after tests establish equivalent limits and error handling.

Call-site sweep must cover both adapter construction and raw generation calls:
agent main/minimal/finalization, helpers, personality/memory, KG extraction,
Guardian, mission preparation, flows, desktop chat/looper, Game Maker,
Storymaker, Detective, CYD, mail assist, SIP/Speech Lab, Vision/content summaries,
Store relay and failover probes. Change bindings only where necessary; retain
each feature's permissions and unrelated behavior.

Acceptance tests:

- [x] Exercise the real SDK with an injected RoundTripper, including main and
  minimal agent flows. Tests must reach captured wire JSON, not stop at a fake
  ChatClient or invoke only the transport beneath the SDK.
- [x] Table cases: ordinary chat; bare o1/o3/o4 and GPT-5 IDs; qualified router
  IDs; known GPT-6 IDs; unknown custom models; streaming/non-streaming; tool
  choice `auto`, `none`, `required` and a named function.
- [x] Switch ordinary chat to a reasoning fallback and back, with no parameter
  contamination. Test an incompatible Claude fallback and concurrent task routes.
- [x] Retain `TestNewRequestBudgetReasoningAndSmallestFailoverLimits` and
  cancellation/no-replay coverage. Output allowance must remain unchanged.

## W2: Complete Responses support and migrate Copilot

Dependencies: W1. This closes both high-priority findings.

Primary files:

- `internal/llm/copilot_transport.go`, `copilot_responses.go`, `interface.go`;
  shared completion/stream mapping files in `internal/llm` as needed.
- `internal/llm/usage_observation.go`, `completion_stream.go`, and the existing
  prompt-cache transport in `client.go`.
- Reasoning serialization/display helpers and affected agent/history consumers.

Tasks:

- [x] Map developer/system/user/assistant content and supported image parts to
  valid Responses input items. Preserve order, function call IDs, function-call
  outputs, schemas, tool choice, parallel-call intent and structured output.
  Use `NewResponseFunctionTool`; reject unsupported content and multiple-choice
  requests (`n > 1`) explicitly rather than silently reducing their meaning.
- [x] Map the validated completion allowance to `max_output_tokens` and effort
  to native `reasoning.effort` when supplied. Omit unsupported sampling. Retain
  provider/model defaults when no explicit effort was selected.
- [x] Set `store: false` for the stateless local-history path. Preserve native
  reasoning continuation items, including encrypted content, unchanged and in
  order through a versioned opaque envelope in the existing reasoning field.
  Use SDK raw event/output data where typed fields omit opaque native fields.
  Do not display the envelope, leak it through `ReasoningText`, or send an
  incompatible provider's envelope as native continuation data.
- [x] Convert native output and events to existing completion responses/streams:
  text, refusals, interleaved/multiple function calls, incremental arguments and
  terminal usage. Preserve IDs and tool order across a complete tool round trip.
- [x] Emit successful finish markers only after confirmed native completion.
  Map token exhaustion/refusal to the existing non-success finish reasons.
  Native errors, failed/incomplete responses, malformed required events and EOF
  before a terminal event must never become assistant success text.
- [x] Keep cancellation and early close neutral; record provider failures once.
  Never replay a stream already returned to the caller. Retain partial usage.
- [x] Extend usage observation to native Responses input/output/cache counters
  and terminal stream usage. Preserve provider attribution, unknown-versus-zero
  measurements, cost estimation and exactly-once booking on failures/probes.
  Carry existing prompt-cache/privacy policy to the new endpoint.
- [x] Move Copilot Responses-only selection and prefix handling before Chat SDK
  validation. Preserve auth refresh and the correct streaming Accept header.
  Aggregate required upstream streams for non-stream callers through the shared
  mapper, then delete the obsolete lossy Chat-to-Responses/SSE translation.

Acceptance tests:

- [x] GPT-6 tool use reaches the configured Responses endpoint through the real
  SDK in main and minimal loops, with no forbidden Chat fields.
- [x] Copilot `gpt-5*-codex`, `codex-*` and discovered `copilot/` names work for
  streaming and buffered callers, including a function result continuation.
- [x] Two tool calls with interleaved argument deltas retain the correct IDs.
- [x] Error before output, error after partial text/tool output, failed response,
  output limit, refusal, missing completion, invalid event, disconnect and caller
  cancellation produce the expected result and physical attempt count.
- [x] Opaque reasoning survives persistence/reload and continuation, never appears
  in visible prose, and leaves existing signed Anthropic continuations intact.
- [x] Cache/input/output/reasoning usage and partial-error costs are booked once.
  Store relay clients retain their existing external Chat Completions interface.

## W3: Repair reload, local errors and helper limits

Dependencies: independent of W2; verify again after central client changes.
Keep independent reload changes separate; commit coupled protocol consumers with
their shared adapter and contract tests.

- [x] **F3:** Compare the complete `LLM.AnthropicThinking` value in
  `internal/server/config_handlers_main.go:llmHotReloadChanged`. Test enablement,
  budget, allowlist contents and unchanged settings. Add a save-to-next-request
  test proving the main transport changes without a restart or another LLM edit.
- [x] **F4:** Return the existing typed non-retryable configuration error from
  `invalidProviderTransport` in `internal/llm/ai_gateway_timeout.go`. Confirm
  classification through URL/network error wrappers. Missing keys, invalid
  bindings and invalid Cloudflare model configuration must neither retry nor
  increment failover health. Keep unknown-error/transient-network, 429, HTTP
  4xx and real managed-local outage behavior unchanged.
- [x] **F7:** In `internal/agent/helper_llm_manager.go`, project provider limit
  overrides only when provider/endpoint/model identity matches the effective
  helper route, following `candidateModelRoutes` in `request_budget.go`.
  Resolve actual-model limits otherwise. Test matching defaults, explicit helper
  model override, the global context cap and helper cache invalidation.
  Apply the same identity rule to the personality helper's completion budget.

Test files: existing server config handler tests, LLM error/failover tests and
helper manager/request-budget tests. Include finite next-request assertions;
avoid tests that only search for field names in source.

## W4: Repair provider-specific transformations and endpoint handling

Dependencies: W1; recheck the final configuration with W2 enabled.

- [x] **F5:** Remove `glm` from the MiniMax-only transport selection in
  `internal/llm/client.go`. Preserve native GLM system messages and
  `reasoning_content`. Test a thinking/tool continuation and ordinary text.
  Keep the separate `zai` route and MiniMax M3/legacy behavior intact; do not
  silently migrate saved provider types or endpoints.
  Preserve all GLM reasoning tool turns in both agent loops, not only the newest.
- [x] **F6:** Normalize pasted Anthropic Messages endpoints in a provider-aware
  path before SDK endpoint composition. Cover native `/v1`, full `/v1/messages`,
  trailing slashes, compatible proxy prefixes, gateway paths and custom URLs.
  Do not strip `/messages` from unrelated providers or alter the endpoint origin.
  Compare the same normalized route during validation, probes and generation.
- [x] Correct the OpenCode diagnostic hint in `client.go` to match usable saved
  base URLs. `ui/cfg/providers.js` already supplies `/zen/go/v1`; it is not
  missing the suffix. Current upstream documentation uses protocol-specific
  `/inference/go/...` endpoints, so do not automatically migrate existing custom
  URLs or append another `/v1`. Verify a new canonical default separately before
  changing it; retain a working existing configuration.
- [x] Update affected provider help in all 16
  `ui/lang/config/providers/*.json` locales if visible wording changes. Use the
  existing Config save/probe flow and preserve route/auth diagnostics.
  No UI wording changed; locale and UI source files therefore remain unchanged.

Acceptance: injected-transport tests capture the exact path/body for each
provider. Add focused configuration/help checks, run JavaScript/JSON validation
for changed UI files, and run the existing applicable Config browser checks
when the UI changes. No new provider-management controls are required.

## W5: Documentation, verification and closeout

- [x] Update `internal/llm/AGENTS.md` for route-specific preparation, Responses
  completion/continuation and usage ownership. Update affected agent/server/UI
  contracts and their indexes only where their durable contracts change.
- [x] Update `documentation/anthropic.md` and
  `documentation/llm-router.md` for effective reload/route compatibility.
  Record the protocol/model support matrix in existing LLM operator guidance.
- [x] Correct `config_types.go` and affected help/template documentation to
  describe the existing temperature contract: zero selects AuraGo's default
  0.7; the chat can modulate/clamp sampling, and reasoning models can omit it.
  Do not change zero's persisted meaning or promise deterministic output.
- [x] Reread changed paths against the DOX chain, inspect encoding/LF and run
  `git diff --check`. Use meaningful regression tests with each repair, then
  run the affected package suites once after integration.

Baseline commands, adjusted to the final changed-file inventory:

```powershell
./scripts/go-test-with-cache.ps1 ./internal/llm ./internal/providerutil ./internal/budget ./internal/agent ./internal/memory ./internal/server -p=1 -count=1 -timeout=20m
./scripts/go-test-with-cache.ps1 ./internal/tools ./internal/security ./internal/services ./internal/kgextraction -run 'Vision|Guardian|Helper|LLM|Completion|MissionPreparation' -p=1 -count=1 -timeout=15m
node --check ui/cfg/providers.js
npm run check:ui
```

Use a writable repository-local `GOTMPDIR`/`TEMP`/`TMP` if the host temporary
directory is blocked. Run network-free fixture tests with injected transports
first. Real loopback HTTP/TLS tests must run in an environment that permits local
connections; an access-denied socket failure is an infrastructure failure, not a
passing provider test. Run relevant LLM/agent/server race tests on Linux with the
existing serialized CI policy. Check remaining discovered consumers with their
own focused tests when their binding or interpretation changes.

- [ ] Run a small live acceptance set on an authorized test setup: one ordinary
  model; an available o/GPT-5 model; GPT-6 with two tool turns; Copilot Codex in
  both stream modes; Anthropic thinking before/after a settings-only save; GLM
  tool continuation; one forced fallback between different model families.
  Use synthetic prompts and a tool fixture. Record route, result and usage
  metadata in ignored `reports/`, never keys or conversation bodies.
- [x] Verify configuration failures make zero provider attempts, accepted streams
  are not replayed and the configured output ceilings still hold.
- [x] Commit each coherent repair with its tests and owner-document updates.
  Inspect only intended staged files, perform the required graph change analysis,
  check the staged diff and scan for secrets. Separate baseline failures from
  new failures. Deployment, release and pushing are separate work.

Definition of done: F1-F7 pass their regression checks, the two high-priority
paths work end to end, measured usage/continuations survive the protocol change,
and live acceptance is either recorded or explicitly left pending. Local unit
tests alone do not establish current provider availability/compatibility.

## Local verification result

- Final full suites: `internal/llm`, `internal/budget` and `internal/config` pass.
  `internal/memory` also passes. Focused agent/server repair tests exercise real
  SDK wire requests, main/minimal tool rounds and a thinking-only Config save.
- Focused `internal/security`, `internal/services` and `internal/kgextraction`
  consumer checks pass. The broader tools selection fails its unrelated HTTP
  helper cancellation test, reproduced at unchanged baseline `b796c52c3`.
- The full agent suite retains five unrelated failure groups: Homepage aliases,
  MiniMax prepared-tool catalog expectations, dispatch coverage, search-term
  loading and search-term coverage. All five reproduce in a clean baseline
  worktree. Helper/personality fixture identities were completed and their
  affected tests pass separately after the fixes.
- The full server suite fails `TestStorePDFSeparatesGrantsAndPublishesNewFiles`
  at the pre-existing captured file-publication grant. Its owning source/test
  are unchanged and the path makes no LLM request; this classification is
  source-based rather than a separate clean-worktree reproduction.
- A 15-ms LLM timeout fixture flaked under concurrent checkout/test load; it
  passes in isolation and in the final full LLM suite. No timeout was weakened.
- Graph change analysis is partial because GitNexus is stale; unmapped new files
  are covered by direct source/call-site review and protocol/consumer tests.
  Graft refreshed 28 changed files during the final source check.
- DOX owners updated: `internal/llm`, `internal/agent`, `internal/server`.
  Root/global ownership and UI behavior did not change, so their guides/indexes
  need no update. No UI translations, new sampling controls or dependencies added.
- Detailed local evidence: ignored `reports/llm-remediation-20261116/`.
  No live provider acceptance, Linux race run, push, release or deployment.

## Explicitly deferred

- A provider/model-specific effort selector can follow after the compatibility
  fixes. It would need config resolution, model-valid choices, route projection,
  provider-specific wire mapping, hot reload and all 16 UI translations. Do not
  introduce a universal effort enum or silently force reasoning off now.
- Top-P, penalties, stop sequences, seed, verbosity and service-tier controls.
- Changing Agnes's always-enabled thinking policy without a product requirement.
- Removing legacy Game Maker temperature fields or changing Storymaker phase
  temperatures. Unused Game Maker fields should be deprecated/removed in a
  focused compatibility cleanup rather than wired into obsolete phases.
- Reworking Vision sampling. Vision already uses `budget.CompleteChat` with a
  finite allowance; preserve that path while repairing shared behavior.
- Training catalog/tool-schema changes, dependency upgrades and release work.

## Sources and document ownership

- Existing source contracts: `AGENTS.md`, `internal/llm/AGENTS.md`,
  `internal/agent/AGENTS.md`, `internal/server/AGENTS.md`, `ui/AGENTS.md`.
  Reread feature-specific indexed owners before editing their consumers.
- [OpenAI model/protocol guidance](https://developers.openai.com/api/docs/guides/latest-model?model=gpt-6-astra).
- [OpenAI Responses migration](https://developers.openai.com/api/docs/guides/migrate-to-responses).
- [Z.ai thinking continuation](https://docs.z.ai/guides/capabilities/thinking-mode).
- [OpenCode current endpoints](https://opencode.ai/docs/console/go/#endpoints).

This plan is owned by the root guide's existing `plans/` scope. Creating it does
not change runtime contracts or establish a new DOX boundary; existing AGENTS
files remain unchanged for the planning-only change.
