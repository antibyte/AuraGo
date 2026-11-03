# Audit: Agent Core, LLM-Provider & Configuration

Area 2 of the AuraGo deep-audit series. Scope: `internal/agent/**`, `internal/llm/**`,
`internal/localllm/**`, `internal/config/**`, `internal/prompts/**` + `prompts/embed.go`,
`internal/budget/**`, `internal/providerutil/**`, `internal/a2a/**`, `internal/vaultprompt/**`,
`internal/promptsource/**`.

Audit date: 2026-11-02. Repository: `C:\Users\Andi\Documents\repo\AuraGo` (module `aurago`, Go 1.27.3).
**Read-only audit — no repository file was modified.**

---

## Scope & method

### Method

1. Read the mandatory root `AGENTS.md` and the area guide `internal/agent/AGENTS.md` before any analysis.
2. Orientation via the code-intelligence graph, then targeted source reads for every reported finding.
   **Tooling note:** the `graft` **MCP server timed out on every call** (`graft_check_freshness`,
   `graft_find_code`, `graft_repo_map` — all exceeded 120 s; `graft check` exceeded 180 s). The
   fallback `graft` CLI (`graft ask`, `graft grep`, `graft callers`, `graft skeleton`) was used instead
   and answered normally (one invocation reported *"a graph rebuild is already in flight — answering from
   the current graph"*; the graph later refreshed successfully). All graph/CLI findings below are therefore
   **CLI-derived, not MCP-derived**, and were independently re-verified against the raw source.
3. Per `AGENTS.md`, **no symbol is reported as dead on the strength of an empty caller list.** Where a
   caller set was ambiguous (`graft callers NewClient` matched several symbols), the claim was resolved with
   a literal text search (`llm.NewClient(`) before any assertion was made.
4. **Every finding below was read in the actual source.** No finding is asserted from a grep hit alone.
   Items I could not confirm are explicitly labelled *Needs-verification* with the exact check to run.
5. No builds, no test execution, no formatters, no codegen, no `git` mutations were performed.

### Coverage

| Package | Go files | non-test LOC | test LOC |
|---|---:|---:|---:|
| `internal/agent` | 744 | 91,990 | 80,316 |
| `internal/llm` | 100 | 22,806 | 9,204 |
| `internal/config` | 138 | 14,468 | 12,066 |
| `internal/localllm` | 23 | 5,774 | 3,449 |
| `internal/prompts` | 33 | 5,272 | 5,395 |
| `internal/budget` | 6 | 1,035 | 538 |
| `internal/a2a` | 15 | 1,101 | 664 |
| `internal/vaultprompt` | 2 | 605 | 571 |
| `internal/promptsource` | 2 | 38 | 16 |
| `internal/providerutil` | 1 | 15 | 0 |
| **Total (in-scope)** | **1,064** | **143,104** | **112,219** |

Plus `prompts/embed.go` (repo root, outside the package dirs).

Deep-read coverage: **38 source files** read verbatim in targeted spans (~1,900 lines) plus **14 test files**
inspected. The remaining ~1,026 files were covered by graph/LOC/pattern sweeps, not line-by-line reading —
see *Not covered / follow-ups*.

### Scope spillover (one line each, not audited further)

- `internal/server/**` — provider save/reload handlers are the primary consumers of `internal/config` and
  `internal/llm` APIs. Only the credential- and cache-relevant call sites were read.
- `internal/tools/**` — consumes decoded tool arguments produced by `internal/agent`.
- `internal/security/**` — `Guardian` used by `internal/a2a` and by the agent tool-call path.
- `internal/kgextraction/**`, `internal/memory/**`, `internal/services/**` — create their own LLM clients.

---

## Architecture map

```
internal/agent/agent_loop.go:313        ExecuteAgentLoop — the whole turn→provider→tool loop (L313-L2790, ~2,477 lines)
internal/agent/agent_loop.go:538        maxLoopIterations = 100, capped min(IterationLimit, 750)  ← loop bound
internal/agent/agent_loop.go:698        calculateEffectiveMaxCalls → circuit breaker (default 10)    ← tool-call bound
internal/agent/agent_loop.go:700        s.toolCallCount >= effectiveMaxCalls → ToolChoice="none" / Tools=nil
internal/agent/agent_loop.go:796        prepareRequestBudgetAndTools → request budget + schema shedding
internal/agent/request_budget.go:156   prepareRequestBudgetAndTools — resolves route limits, sheds optional schemas
internal/agent/request_budget.go:102   newRequestBudgetWithLimits — per-route token accounting
internal/agent/request_budget.go:187   candidateModelRoutes — builds llm.ModelRoute (APIKey set at L200)
internal/agent/request_budget.go:53    RequestTokenUsage — deliberately content-free accounting
internal/llm/model_limits.go:103       ResolveModelLimits — override → registry → probe → conservative fallback
internal/llm/model_limits.go:332       cachedProviderModelLimitProbe — single-flight + generation-guarded cache write
internal/llm/model_limits.go:91        InvalidateModelLimitCache — clears entries/inflight/scheduled, bumps generation
internal/llm/client.go:395             NewClientFromProviderWithConfig — per-call client factory
internal/llm/client.go:408             buildLLMHTTPClient — per-call Transport stack (6 wrappers)
internal/llm/client.go:500             defaultLLMHTTPTransport — http.DefaultTransport.Clone() per call
internal/llm/client.go:495             responseTimeoutTransport — per-attempt + 3 min stream timeout
internal/llm/anthropic_transport.go:429 anthropicTransport.RoundTrip — clones request, keeps ctx, rewrites path/headers
internal/llm/anthropic_transport.go:1193 translateAnthropicStream — io.Pipe bridge, closes both bodies
internal/llm/retry.go:66               maxRetryAttempts = 10; aborts on ctx errors and IsNonRetryable
internal/agent/agent_llm_call.go:238   recvCh (buffered 1) + recvCtx + panic-recovering receiver goroutine
internal/agent/agent_llm_call.go:432   teardown: cancelRecv → stm.Close → 1 s wait on recvDone
internal/agent/native_tools.go:545     NativeToolCallToToolCall — raw map → typed decode → malformed rescue
internal/agent/agent.go:997            ToolCall.UnmarshalJSON — coerce → typed decode → merge into Params
internal/agent/agent.go:735            toolCallStringFields — hand-maintained 14-name string allowlist
internal/agent/tool_args_builtin.go:76  toolArgString  (942 call sites in internal/agent)
internal/agent/tool_args_builtin.go:87  toolArgInt     (163 call sites, 20 files)  ← no string/json.Number case
internal/agent/tool_args_services.go:186 toolArgInt64  (11 call sites)  ← DOES handle string
internal/agent/tool_args_extended.go:1017 firstNonEmptyToolString (681 call sites)
internal/agent/agent_parse.go:601      numericStringToolFields — 7-field string→number repair list
internal/agent/agent_parse.go:1531     parseXMLParams — default branch stores raw STRING at L1650
internal/agent/tool_call_contract.go:63 prepareToolCall — malformed args ⇒ PreparationError=invalid_arguments (L67-70)
internal/config/config.go:377          Load — defaults BEFORE yaml.Unmarshal (L1054); corrupt file backed up (L1071-1075)
internal/config/config.go:1862         cfg.ResolveProviders() — after vault-ready defaults, before env overrides (L1869)
internal/config/config_migrate.go:241  ResolveProviders — migrate + normalise + resolve LLM/Fallback/Vision
internal/config/provider_credentials.go:104 applyProviderCredentialBindings — fail-closed invalid:// sentinels
internal/config/sip.go:159             ApplySIPDefaults — cfg.ReadOnly = true (L163), applied pre-unmarshal
```

---

## Findings

### F-1 [HIGH] `toolArgInt` silently discards numeric tool arguments that arrive as JSON strings

**Location** — `internal/agent/tool_args_builtin.go:87-105`

**Evidence**

```go
func toolArgInt(args map[string]interface{}, fallback int, keys ...string) int {
	for _, key := range keys {
		if raw, ok := args[key]; ok {
			switch value := raw.(type) {
			case int:
				return value
			case int32:
				return int(value)
			case int64:
				return int(value)
			case float64:
				return int(value)
			case float32:
				return int(value)
			}
		}
	}
	return fallback
}
```

There is no `case string:` and no `case json.Number:`. `toolArgString` at
`internal/agent/tool_args_builtin.go:76-85` has the mirror-image gap (it accepts only `string`).

`toolArgInt(` occurs **163 times across 20 files** in `internal/agent`
(`tool_args_execution.go` 35, `tool_args_infra.go` 26, `tool_args_extended.go` 21, `tool_args_comm.go` 17,
`tool_args_services.go` 12, `tool_args_huggingface.go` 6, `tool_args_builtin.go` 5, `tool_args_fritzbox.go` 5,
`tool_output_read_handler.go` 5, `dispatch_platform.go` 4, `bluetooth_tool.go` 2, `learning_checks.go` 2,
`tool_recovery_state.go` 8, `remote_tool.go` 8, and 5 more).

**Why it matters** — A tool argument the model wrote as `"5"` instead of `5` is not an error and not a
rejection: it is **silently replaced by the decoder's fallback value**. Concrete examples read in source:

- `internal/agent/tool_args_builtin.go:117` — `Limit: toolArgInt(args, 50, "limit")` → a model asking for
  `limit: "3"` gets 50.
- `internal/agent/bluetooth_tool.go:30` — `mediaID := int64(toolArgInt(tc.Params, 0, "media_id"))` → `0`.
- `internal/agent/tool_args_services.go:544` — `Limit: firstNonEmptyInt(tc.Limit, toolArgInt(tc.Params, 0, "limit"))`.

Two independent hand-rolled workarounds exist precisely because of this gap, which is direct evidence the
gap bites in production:

- `internal/agent/tool_args_builtin.go:227-239` (`paperlessLimitArg`) re-parses the string **before** calling `toolArgInt`.
- `internal/agent/bluetooth_tool.go:31-35` re-parses the string **after** `toolArgInt` returned 0.

The same package also contains a helper that *does* handle strings — `toolArgInt64`
(`internal/agent/tool_args_services.go:186-214`, `case string:` + `strconv.ParseInt`) — so the two
int helpers in the *same package* disagree about what an integer argument is. `toolArgInt64` has 11 call
sites; `toolArgInt` has 163.

**Fix** — Give `toolArgInt` the same `case string:` / `json.Number:` handling as `toolArgInt64` (ideally by
having `toolArgInt` delegate to `toolArgInt64` and narrow), then delete the `paperlessLimitArg` and
`decodeBluetoothToolArgs` workarounds. Note `json.Number` only appears if a decoder opts into
`json.Decoder.UseNumber()` — `internal/agent/discovery_schema.go:226` and
`internal/agent/coagent_schema.go:187` do; the tool-argument path does not.

**Confidence** — High. Mechanism read directly in source; call-site count measured with a pattern sweep;
both workarounds read directly.

---

### F-2 [HIGH] `ToolCall.UnmarshalJSON` contradicts its own documented raw-value contract

**Location** — `internal/agent/agent.go:997-1048`

**Evidence**

The doc comment states the intent:

```go
// Some LLMs (e.g. MiniMax) occasionally emit numeric values for fields that the
// schema declares as strings (most notably "id"). Coerce common scalar-string
// fields to strings before typed decoding so a single mistyped value does not
// reject the entire tool call. The flat Params map below preserves the raw
// values for any tool-specific decoder that cares about the original type.
data = coerceToolCallStringFields(data)
data = coerceToolCallBoolFields(data)
data = coerceToolCallStringMapFields(data)
```

But `raw` is decoded from the **coerced** buffer, and the merge overlays it last:

```go
var raw map[string]interface{}
if err := json.Unmarshal(data, &raw); err != nil {   // L1021 — `data` is post-coercion
	return nil
}
restoreOriginalArrayToolArgs(raw, originalData, "to", "cc")   // L1025 — only `to`/`cc` restored

merged := make(map[string]interface{})
if rawParams, ok := raw["params"].(map[string]interface{}); ok { ... }   // L1029-1033
for k, v := range tc.Params { merged[k] = v }                          // L1035-1037 (existing/raw first)
for k, v := range raw {                                                // L1039-1044
	if k == "params" { continue }
	merged[k] = v                                                     // <- coerced values WIN
}
tc.Params = merged
```

The same file's header comment at `internal/agent/agent.go:998-999` says the flat map exists
*"so tool-specific decoders can read from `tc.Params` only"* — so `Params` is the **only** channel a
decoder has. The raw value is therefore unreachable, not merely inconvenient.

**Why it matters** — For the 12 allowlisted string fields other than `to`/`cc` (`id`, `message_id`,
`task_id`, `artifact_id`, `file_id`, `device_id`, `nest_id`, `egg_id`, `skill_id`, `thread_id`,
`contact_id`, `event_id`), the coerced value permanently replaces the original:

- `internal/agent/agent.go:827-828` — a JSON number is re-serialised as a JSON **string**:
  `coerced, err = json.Marshal(string(v))`.
- `internal/agent/agent.go:817-826` — a JSON array is **comma-joined** into one string:
  `strings.Join(parts, ", ")`.

So `"event_id": 12345` becomes the string `"12345"`, and `"id": ["a","b"]` becomes `"a, b"` — a value
that no decoder can distinguish from a literal the model typed.

This is not hypothetical: `internal/agent/go2rtc_detection_tool.go:488-512` (`detectionEventID`)
explicitly defends against `json.Number`, `string` and `float64` variants of the same id, and
`internal/agent/agent.go:747` carries the comment *"go2rtc detection event ids arrive as JSON numbers;
the literal stays exact"* — the literal survives only because the coercion re-serialises the raw JSON
**text**; the raw **typed** value does not.

The native-tool path is affected too. `internal/agent/native_tools.go:594-604` builds a genuinely raw map
(`rawMap`) and assigns it to `tc.Params`, but the subsequent `json.Unmarshal([]byte(normalizedArgs), &tc)`
at `internal/agent/native_tools.go:609` re-enters `ToolCall.UnmarshalJSON`, whose merge at
`internal/agent/agent.go:1039-1044` overlays the coerced values on top of that raw map.

Ad-hoc per-key rescues already exist — `internal/agent/agent.go:798-804` (`_items_raw`),
`internal/agent/agent.go:1025` (`to`/`cc`), `internal/agent/native_tools.go:596` (`tags`) — which shows the
pattern is understood but implemented key-by-key instead of structurally.

**Fix** — Decode the *original* buffer into a second map and keep both: expose the coerced map for typed
decoding and the raw map to decoders (e.g. `Params` raw + `CoercedParams`), or merge **raw over coerced**
and let each decoder opt into the coerced form. Update the doc comment at L1006-1007 to match whichever
contract is chosen.

**Confidence** — High. The full function body, the coercion helpers and the native path were read; the
contradiction is between two statements in the same function.

---

### F-3 [MEDIUM] `toolCallStringFields` is a hand-maintained allowlist while its bool counterpart is derived reflectively

**Location** — `internal/agent/agent.go:735-750` (allowlist) vs. `internal/agent/agent.go:756-779`
(reflective bool set)

**Evidence**

```go
var toolCallStringFields = map[string]struct{}{
	"id": {}, "message_id": {}, "task_id": {}, "artifact_id": {}, "file_id": {},
	"device_id": {}, "nest_id": {}, "egg_id": {}, "skill_id": {}, "thread_id": {},
	"contact_id": {}, "event_id": {}, // go2rtc detection event ids arrive as JSON numbers; the literal stays exact
	"to": {}, "cc": {},
}
```

The bool counterpart, in the same file, is derived from the struct at runtime:

```go
func getToolCallBoolFields() map[string]struct{} {
	toolCallBoolFields.once.Do(func() {
		fields := make(map[string]struct{})
		typ := reflect.TypeOf(ToolCall{})
		for i := 0; i < typ.NumField(); i++ { ... }
```

**Why it matters** — The `ToolCall` struct spans `internal/agent/agent.go:356-728`. A new string field
added there is **not** protected unless someone remembers to edit a list 300 lines above the struct. When
forgotten, a model that emits the field as a number causes `json.Unmarshal` to fail on the whole struct
(`internal/agent/agent.go:1015-1017`) -> `NativeArgsMalformed` -> `PreparationError =
"invalid_arguments"` (`internal/agent/tool_call_contract.go:67-70`) -> the tool call is rejected outright.
The coercion exists precisely because providers routinely emit numeric ids, so this recurs.

**Fix** — Derive the string-field set reflectively the same way the bool set is derived (or derive it from
the tool JSON schemas the agent already builds), and add a test asserting that every `*_id`-style string
field on `ToolCall` is covered.

**Confidence** — High for the divergence and the failure mode. Whether a field has already been added
without coverage was not exhaustively checked (see *Not covered*).

---

### F-4 [MEDIUM] A fresh `http.Client` and a fresh `http.Transport` are constructed for every LLM request

**Location** — `internal/llm/client.go:395-406`, `:408-498`, `:500-517`

**Evidence**

```go
func NewClientFromProviderWithConfig(cfg *config.Config, providerType, baseURL, apiKey, accountID string, gatewayIDs ...string) *openai.Client {
	p := resolvedProvider{ ... }
	return openai.NewClientWithConfig(buildOpenAIClientConfig(cfg, p))
}
```

```go
func defaultLLMHTTPTransport(responseHeaderTimeout time.Duration) *http.Transport {
	base, _ := http.DefaultTransport.(*http.Transport)
	...
	transport := base.Clone()          // L505 — new connection pool per call
	...
	transport.ResponseHeaderTimeout = responseHeaderTimeout
	return transport
}
```

`buildLLMHTTPClient` then wraps that transport in six layers and returns a new client
(`internal/llm/client.go:436-497`): `miniMaxTransport` / `agnesThinkingTransport` /
`openAIPromptCacheTransport` / `anthropicTransport` / `copilotTransport` / `opencodeGoTransport` /
`manifestRoutingTransport` / `stepFunChatMessageTransport`, then `leadingSystemTransport`,
`usageObservationTransport`, `rateLimitAwareTransport`, `newLoggingTransport`, `responseTimeoutTransport`.

**Why it matters** — `http.Transport.Clone()` gives the new transport **its own connection pool**. Each
`http.Client` therefore cannot reuse connections kept alive by any other client. Consequences:

1. No keep-alive reuse across requests -> a fresh TCP + TLS handshake per LLM call.
2. Idle connections held by the abandoned transports are only reclaimed when the transport is garbage
   collected (Go's `http.Transport` has no finalizer that closes idle conns deterministically), so
   sockets and file descriptors accumulate under sustained load.

This is not confined to the main agent loop (which uses a long-lived `FailoverManager`). It applies to every
auxiliary path that calls the per-call factory, e.g. `internal/llm/helper_resolver.go:58`,
`internal/agent/activity_capture.go:286`, `internal/tools/content_summary.go:185`,
`internal/services/mission_preparation.go:248`, `internal/server/ai_gateway_handlers.go:177`,
`internal/server/agodesk_local_agent.go:561`, `internal/kgextraction/kg_extraction.go:277` and `:296`.

**Fix** — Cache the built `*http.Client` (or at least the base `*http.Transport`) keyed by
`(providerType, baseURL, config-revision)` and invalidate it on provider reconfiguration, the same trigger
that already exists for the model-limit cache (`internal/server/provider_handlers.go:426`).

**Confidence** — High for the construction pattern and its call sites (all read). The *magnitude* of the fd
growth under production load was **not measured** — no load test was run (see *Not covered*).

---

### F-5 [MEDIUM] `llm.ModelRoute` carries a raw API key in an exported, untagged, marshallable field

**Location** — `internal/llm/model_limits.go:27-38` and `:40-54`; transitively
`internal/agent/request_budget.go:38-40`

**Evidence**

```go
// ModelRoute describes one provider/model combination that may serve a request.
// APIKey is used only for a bounded metadata probe and must never be logged.
type ModelRoute struct {
	ProviderID              string
	ProviderType            string
	BaseURL                 string
	APIKey                  string      // <- no `json:"-"` tag
	Model                   string
	ContextWindowOverride   int
	MaxOutputTokensOverride int
	Primary                 bool
}

type ModelLimits struct {
	Route             ModelRoute      // <- embeds the route, and therefore the key
	ContextWindow     int
	...
}
```

**Why it matters** — `ModelLimits` and `ModelRoute` are exported types in a public package with **no JSON
tags at all**, so `json.Marshal(llm.ModelLimits{})` emits the API key in cleartext. `ModelLimits` is
embedded in `internal/agent/request_budget.go:39` (`RequestRouteBudget.Limits`) ->
`internal/agent/request_budget.go:44-50` (`RequestBudget.Routes`) — all exported.

**Current exposure: none found.** A repo-wide pattern sweep for `json.Marshal` over `ModelLimits` /
`ModelRoute` returns no hit. `internal/server/provider_diagnostics.go:251` deliberately builds a response
map field-by-field **without** the key; `internal/server/provider_handlers.go:19-49` (`providerJSON`) has an
explicit `api_key` field but is a separate type; `internal/agent/request_budget.go:53-69`
(`RequestTokenUsage`) is deliberately content-free and carries no key at all.

So this is a **latent trap**, not a live leak — but a single future `json.Marshal` of a budget or limits
struct would ship a secret to an HTTP response. The type comment says the key "must never be logged", yet
nothing in the type system prevents it.

**Fix** — Add `json:"-"` to `ModelRoute.APIKey` (and consider a `String()`/`LogValue()` on `ModelRoute` so
`%v`/`%+v` cannot print it either). Same for the `APIKey` fields on the exported config structs that are
already tagged `json:"-"` — those are fine, but the `llm` package types are not.

**Confidence** — High for the missing tag and for the absence of a current marshalling path (both
verified by sweep + read). The *risk* is prospective.

---

### F-6 [MEDIUM] The model-limit cache key omits the credential, so limits survive a credential rotation for up to an hour

**Location** — `internal/llm/model_limits.go:403-408` and `:16-25`

**Evidence**

```go
func providerModelLimitCacheKey(route ModelRoute) string {
	return strings.ToLower(strings.TrimSpace(route.ProviderID)) + "\x00" +
		strings.ToLower(strings.TrimSpace(route.ProviderType)) + "\x00" +
		strings.TrimRight(strings.TrimSpace(route.BaseURL), "/") + "\x00" +
		strings.ToLower(strings.TrimSpace(route.Model))
}
```

```go
modelLimitPositiveTTL = time.Hour
modelLimitNegativeTTL = 5 * time.Minute
```

The key is `(provider id, provider type, base URL, model)` — **no credential, no account/org**.

**Why it matters** — Context-window and max-output limits are a property of the *(provider, model,
account tier)*, not of the endpoint. A model exposed to a different organisation under the same
`provider id` + `base URL` + `model` can have different limits. Rotating the API key to a different
account therefore leaves the previous account's probe result valid for up to `modelLimitPositiveTTL`
(1 hour).

The common paths are covered — `InvalidateModelLimitCache()` is called on provider save
(`internal/server/provider_handlers.go:426`), on config save (`internal/server/config_handlers_main.go:1105`)
and on failover reconfigure (`internal/llm/failover.go:189`). Note `internal/server/provider_diagnostics.go:233`
passes `APIKey: p.RuntimeAPIKey()` into the route, so the credential is *available* to the key function but
deliberately not used.

**Needs-verification** — Whether a credential can change **without** passing through one of those three
handlers (e.g. an external vault refresh, or a provider whose credential is rotated by another process)
was not established. To verify: search for callers of `ApplyVaultSecrets` / `ApplyOAuthTokens` that do not
follow with `InvalidateModelLimitCache`, and check whether any vault-write path exists outside
`internal/server/provider_handlers.go`.

**Fix** — Either include a non-reversible credential fingerprint (e.g. SHA-256 of the key) in the cache key,
or invalidate the cache inside `ApplyOAuthTokens` / `ApplyVaultSecrets`.

**Confidence** — High for the mechanism and the TTL. Medium for the impact (depends on the unverified path).

---

### F-7 [MEDIUM] A single-flight waiter returns a pre-invalidation probe result and reports it as a cache hit

**Location** — `internal/llm/model_limits.go:344-352` (waiter) vs. `:91-98` (invalidation)

**Evidence**

```go
if call := providerModelLimitCache.inflight[key]; call != nil {
	providerModelLimitCache.Unlock()
	select {
	case <-ctx.Done():
		return providerModelLimitProbe{}, false
	case <-call.done:
		return call.value, true          // <- stale value, reported as a hit
	}
}
```

```go
func InvalidateModelLimitCache() {
	providerModelLimitCache.Lock()
	providerModelLimitCache.entries = make(map[string]modelLimitCacheEntry)
	providerModelLimitCache.inflight = make(map[string]*modelLimitProbeCall)   // <- replaces the map
	providerModelLimitCache.scheduled = make(map[string]uint64)
	providerModelLimitCache.generation++
	providerModelLimitCache.Unlock()
}
```

**Why it matters** — `InvalidateModelLimitCache` **replaces** the `inflight` map. A goroutine already blocked
on `call.done` still holds the old `call` pointer and receives its value — the probe result computed
*before* the invalidation — and returns `true`, which the caller records as `ProbeCacheHit: true`. The
value is also *not* stale in the sense of being written back: the write-back at
`internal/llm/model_limits.go:367` is correctly generation-guarded
(`call.generation == providerModelLimitCache.generation`), so the stale value never re-enters the cache.

So the defect is **read-side only** and bounded to the window in which a probe is in flight across an
invalidation. `internal/llm/model_limits_test.go:364`
(`TestModelLimitInvalidationRejectsStaleInflightResult`) covers the **write** side; the read-side waiter is
untested.

**Fix** — After `<-call.done`, re-read the generation under the lock and treat a generation mismatch as a
miss (`return providerModelLimitProbe{}, false`) instead of returning `call.value, true`.

**Confidence** — High. The waiter, the invalidation and the write-back guard were all read.

---

### F-8 [MEDIUM] `firstNonEmptyToolString` treats whitespace as present; `firstNonEmptyInt` treats `0` as absent

**Location** — `internal/agent/tool_args_extended.go:1017-1024` and `internal/agent/tool_args_services.go:549-556`

**Evidence**

```go
func firstNonEmptyToolString(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
```

```go
func firstNonEmptyInt(values ...int) int {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}
```

**Why it matters** — `firstNonEmptyToolString` has **681 call sites** in `internal/agent`. It does not trim,
so a whitespace-only value (`" "`, `"\n"`) wins over a later, meaningful fallback. Example read in source —
`internal/agent/bluetooth_tool.go:38`:

```go
Device: firstNonEmptyToolString(toolArgString(tc.Params, "device"), tc.DeviceAddr, tc.DeviceName),
```

A params `device` of `" "` produces a device named `" "`, which then wins over a valid `DeviceAddr`.

`firstNonEmptyInt` makes an **explicit `0` indistinguishable from "absent"**:
`internal/agent/tool_args_services.go:544` — `Limit: firstNonEmptyInt(tc.Limit, toolArgInt(tc.Params, 0, "limit"))`
— a caller that explicitly asks for zero rows gets the other value instead.

**Fix** — Trim before the emptiness test in `firstNonEmptyToolString` (compare against `strings.TrimSpace(value) == ""`
while returning the original or the trimmed value consistently). For `firstNonEmptyInt`, change the signature
to return `(int, bool)` or take pointers so "present and zero" is representable.

**Confidence** — High. Both helpers and the call sites quoted were read.

---

### F-9 [MEDIUM] `numericStringToolFields` repairs exactly 7 of the numeric tool arguments

**Location** — `internal/agent/agent_parse.go:601-609`, applied at `internal/agent/agent_parse.go:645-660`

**Evidence**

```go
var numericStringToolFields = map[string]bool{
	"wlan_index":   true,
	"tam_index":    true,
	"msg_index":    true,
	"phonebook_id": true,
	"limit":        true,
	"max_results":  true,
	"tail":         true,
}
```

```go
for field := range numericStringToolFields {
	fieldRaw, ok := raw[field]
	if !ok || len(fieldRaw) < 2 || fieldRaw[0] != '"' { continue }
	...
	if v, err := strconv.Atoi(strings.TrimSpace(asString)); err == nil {
		if encoded, err := json.Marshal(v); err == nil {
			raw[field] = json.RawMessage(encoded)
```

**Why it matters** — This is the *only* mechanism that rescues a string-typed number on the JSON path, and
it covers 7 hand-picked field names. Every other numeric argument the model emits as a JSON string —
`offset`, `timeout_seconds`, `count`, `page`, `duration`, `port`, ... — survives as a string and is then
discarded by `toolArgInt` (F-1). The list is not derived from the tool JSON schemas, so it drifts.

Only one field from this list has a test: `internal/agent/agent_parse_guardian_test.go:150`
(`TestParseToolCallCoercesNumericStringFields`) exercises `wlan_index` only.

**Fix** — Derive the numeric field set from the tool schemas the agent already assembles
(`openai.Tool` parameter schemas), or normalise every schema-typed integer field generically.

**Confidence** — High. The map, its single consumer and the test were all read.

---

### F-10 [MEDIUM] The XML/textual tool-call fallback stores every unrecognised parameter as a raw string

**Location** — `internal/agent/agent_parse.go:1642-1652`

**Evidence**

```go
default:
	// Store all unrecognized params in tc.Params so tool decode functions
	// (e.g. decodeFileEditorArgs) can retrieve them via toolArgString.
	if paramName != "" {
		if tc.Params == nil {
			tc.Params = make(map[string]interface{})
		}
		if _, alreadySet := tc.Params[paramName]; !alreadySet {
			tc.Params[paramName] = paramVal        // <- paramVal is a string
		}
	}
```

**Why it matters** — `ParseToolCall` (`internal/agent/agent_parse.go:829+`) is the path used when a provider
cannot emit native tool calls — a first-class production path, not an edge case. On that path **every**
numeric argument arrives as a Go `string`, which `toolArgInt` then discards (F-1).

The result is that the *same tool behaves differently depending on the provider transport*:

| transport | payload | decoded `limit` |
|---|---|---|
| native function call | `{"limit": 5}` | `5` |
| XML fallback | `<param name="limit">5</param>` | `"5"` -> discarded -> decoder default |

This is very hard to detect in testing because a provider change, not a code change, flips the behaviour.

**Fix** — Coerce XML parameters against the tool schema (the agent already has the schemas at this point),
or at minimum attempt an integer parse for numeric-looking values — the same shape as the existing
`paperlessLimitArg` workaround at `internal/agent/tool_args_builtin.go:227-239`.

**Confidence** — High. The default branch of `parseXMLParams` and `ParseToolCall`'s role were read.

---

### F-11 [MEDIUM] Environment-variable credential overrides run after vault binding, cover only 6 subsystems, and never update the provider list

**Location** — `internal/config/config.go:1862-1890`

**Evidence**

```go
// Resolve provider references -> populates all yaml:"-" fields.
cfg.ResolveProviders()                                   // L1862

// Environment overrides for API keys (applied AFTER provider resolution so
// they override any key from the providers list):
if val := os.Getenv("LLM_API_KEY"); val != "" {
	cfg.LLM.APIKey = val
} else if val := os.Getenv("OPENAI_API_KEY"); val != "" {
	cfg.LLM.APIKey = val
} else if val := os.Getenv("ANTHROPIC_API_KEY"); val != "" {
	cfg.LLM.APIKey = val
}
if val := os.Getenv("CO_AGENTS_LLM_API_KEY"); val != "" { cfg.CoAgents.LLM.APIKey = val }
if val := os.Getenv("EMBEDDINGS_API_KEY"); val != "" { cfg.Embeddings.APIKey = val }
if val := os.Getenv("VISION_API_KEY"); val != "" { cfg.Vision.APIKey = val }
if val := os.Getenv("WHISPER_API_KEY"); val != "" { cfg.Whisper.APIKey = val }
if val := os.Getenv("FALLBACK_LLM_API_KEY"); val != "" { cfg.FallbackLLM.APIKey = val }
```

**Why it matters** — Three separate problems:

1. **Precedence over the vault.** `cfg.ResolveProviders()` (L1862) runs before these overrides, and the
   vault binding (`cmd/aurago/main.go:760-764`: `ApplyVaultSecrets` -> `ResolveProviders` ->
   `ApplyOAuthTokens`) has already populated the resolved slots. The env var therefore wins over the
   vault — the opposite of the usual expectation, and undocumented at the vault layer.
2. **Partial coverage.** `cfg.LLM.HelperAPIKey` has **no** env override, although `cfg.LLM.FallbackLLM.APIKey`
   does. In a deployment that injects credentials purely through the environment, the primary LLM works
   and the helper LLM (memory analysis, summarisation, web-scraper summaries —
   `internal/config/config_migrate.go:518-519`, `:565-566`) has no credential.
3. **The provider list is not updated.** The overrides write only to the *resolved* slots, never to
   `cfg.Providers[i].APIKey`. The provider diagnostics view
   (`internal/server/provider_diagnostics.go:251`) reports the provider-entry key, so the operator sees one
   credential while the runtime uses another.

**Fix** — Apply env overrides to the provider entries *before* `ResolveProviders()` so that a single
resolution pass produces a consistent picture, and add a helper-provider override (or a generic
`AURAGO_<SUBSYSTEM>_API_KEY` scheme) so coverage is complete. At minimum, log a warning when an env
override shadows a vault-provided key.

**Confidence** — High for the ordering, the coverage gap and the divergence from the provider list — all
read directly. The *operational* impact depends on deployment style, which I could not observe.

---

### F-12 [MEDIUM] The argument-coercion helpers that 942 + 681 + 163 call sites depend on have no direct unit tests

**Location** — `internal/agent/tool_args_builtin.go:76-105`,
`internal/agent/tool_args_extended.go:1017-1024`, `internal/agent/tool_args_services.go:549-556`

**Evidence** — A pattern sweep over `internal/agent/*_test.go` for
`func Test.*(ToolArg|ArgString|ArgInt|FirstNonEmpty|NumericString|CoerceToolCall)` returns **no test whose
subject is any of these helpers**. `toolArgString` appears only as an incidental assertion helper
(`internal/agent/game_maker_operation_alias_test.go:129`,
`internal/agent/native_tools_game_maker_test.go:70`, `internal/agent/tool_call_pipeline_test.go:248,251`,
`internal/agent/tool_route_test.go:285`).

Seven `tool_args_*_test.go` files exist, and they do test the *decoders* — but they construct `Params`
maps **by hand with the correct Go types**, e.g. `internal/agent/tool_args_execution_test.go:37-53`:

```go
tc := ToolCall{
	Action: "save_tool",
	Params: map[string]interface{}{
		"name": "demo_tool",
		"description": "Demo",
		"code": "print('ok')",
	},
}
```

**Why it matters** — The decoders are tested against *pre-typed* inputs, so the entire JSON->map type
coercion layer — the layer where F-1, F-9 and F-10 actually live — is invisible to the test suite. The one
test that does exercise it, `internal/agent/agent_parse_guardian_test.go:150`, covers a single field of the
7-field `numericStringToolFields` map.

The practical consequence: a change that widens the string->number gap, or a new tool whose numeric
argument is emitted as a string, will pass CI and fail in production. This is the mechanism by which F-1
survived.

**Fix** — Add table-driven unit tests for `toolArgInt`, `toolArgString`, `firstNonEmptyToolString` and
`firstNonEmptyInt` covering the JSON-decoded value types (`float64`, `string`, `json.Number`, `bool`,
`nil`) — and at least one end-to-end test that decodes a tool call from **raw JSON text** (not a
hand-built `Params` map) for each numeric argument.

**Confidence** — High. The sweep was pattern-based over the whole `internal/agent` test corpus and the
quoted test was read.

---

### F-13 [LOW] `toolArgString` returns `""` for any non-string value

**Location** — `internal/agent/tool_args_builtin.go:76-85`

**Evidence**

```go
func toolArgString(args map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if raw, ok := args[key]; ok {
			if value, ok := raw.(string); ok && value != "" {
				return value
			}
		}
	}
	return ""
}
```

**Why it matters** — **942 call sites** in `internal/agent` depend on this. Any string-typed tool argument
that a model emits as a JSON number (`"port": 8080`, `"timeout": 30`) is silently reduced to the empty
string — no error, no fallback. The companion coercion `coerceToolCallStringFields`
(`internal/agent/agent.go:735-750`) only rescues **14 named fields**, so a numeric value for any other
string field is lost.

This is the mirror image of F-1 and has the same root cause: neither helper coerces across types.

**Fix** — Accept numbers (and `json.Number`) in `toolArgString` and format them, mirroring the
`toolArgInt` fix. Doing both in the same change keeps the two helpers symmetric.

**Confidence** — High. Helper and call-site count verified.

---

### F-14 [LOW] Two hand-rolled per-tool workarounds exist for the `toolArgInt` string gap

**Location** — `internal/agent/tool_args_builtin.go:227-239`, `internal/agent/bluetooth_tool.go:29-45`

**Evidence**

```go
func paperlessLimitArg(args map[string]interface{}) int {
	if raw, ok := args["limit"]; ok {
		if value, ok := raw.(string); ok {
			value = strings.TrimSpace(value)
			if value != "" {
				if number, err := strconv.Atoi(value); err == nil { return number }
			}
		}
	}
	return toolArgInt(args, 0, "limit")
}
```

```go
func decodeBluetoothToolArgs(tc ToolCall) bluetoothToolArgs {
	mediaID := int64(toolArgInt(tc.Params, 0, "media_id"))
	if mediaID == 0 {
		if raw := strings.TrimSpace(toolArgString(tc.Params, "media_id")); raw != "" {
			mediaID, _ = strconv.ParseInt(raw, 10, 64)
		}
	}
```

**Why it matters** — The two workarounds are semantically identical but written differently (one *before*
the call, one *after* it returns 0), and they fix **one field each**. They are the clearest available
evidence that F-1 is a recurring, felt problem rather than a theoretical one — and each new numeric
argument invites a third variant.

**Fix** — Fix `toolArgInt` (F-1) and delete both workarounds, asserting the new behaviour in a test.

**Confidence** — High. Both workarounds were read in full.

---

### F-15 [LOW] `migrateInlineProviders` creates a phantom `main` provider even when no legacy inline configuration exists

**Location** — `internal/config/config_migrate.go:1014-1058`

**Evidence**

```go
func (c *Config) migrateInlineProviders() {
	if len(c.Providers) > 0 {
		return // new-format config — no migration needed
	}
	...
	oldLLMType := c.LLM.Provider // save before overwriting
	if oldLLMType == "" {
		oldLLMType = inferType(c.LLM.LegacyURL, "")
	}
	c.LLM.Provider = addProvider("main", "Haupt-LLM", oldLLMType, c.LLM.LegacyURL, c.LLM.LegacyAPIKey, c.LLM.LegacyModel)
```

with

```go
inferType := func(baseURL, providerHint string) string {
	...
	default:
		return "openai"
	}
}
```

**Why it matters** — On a config with **no** `providers:` list and **no** legacy inline fields, migration
still creates a provider entry with ID `main`, type `openai` and an empty `BaseURL`/`APIKey`/`Model`, and
**overwrites `c.LLM.Provider` with `"main"`**. `ResolveProviders` then takes the `FindProvider` branch
(`internal/config/config_migrate.go:288-294`) and assigns `c.LLM.ProviderType = "openai"`,
`c.LLM.BaseURL = ""`, `c.LLM.APIKey = ""`, `c.LLM.Model = ""`.

Downstream, that phantom entry is surfaced to the operator as a real provider with an error:
`internal/server/provider_diagnostics.go:239` computes `"endpoint_missing"` when the base URL is empty, and
`:251` lists it in the providers array. So a fresh or unconfigured install shows a phantom `main` provider
of type `openai` flagged as broken.

Note the migration **is** idempotent for the intended case — the `len(c.Providers) > 0` guard at L1015-1017
makes a second call a no-op — and `ResolveProviders` is genuinely re-entrant. The issue is only the
phantom entry.

**Fix** — Skip creating the `main` entry when there is no legacy inline configuration at all (all of
`LegacyURL`/`LegacyAPIKey`/`LegacyModel` empty and `LLM.Provider` empty), or mark synthetic entries so the
diagnostics view can filter them.

**Confidence** — High for the code path. Medium for the operator-visible symptom (the diagnostics filter
behaviour in `internal/server` was read but not exercised).

---

### F-16 [LOW] Two overlapping provider-credential resolution mechanisms with an unenforced call order

**Location** — `internal/config/config_migrate.go:241-243` + `:763-766`;
`internal/config/provider_credentials.go:104`

**Evidence**

```go
func (c *Config) ResolveProviders() {
	defer c.applyProviderCredentialBindings(nil)   // L242 — tokens = nil
	c.migrateInlineProviders()
	...
```

```go
// ApplyVaultSecrets populates all vault-only secret fields from the vault.
// Must be called after Load() and before the config is used for the first time.
// After calling this, ResolveProviders() should be called again to propagate
// provider API keys into the resolved LLM/Vision/etc. slots.
func (c *Config) ApplyVaultSecrets(vault SecretReader) {
```

**Why it matters** — `ResolveProviders` ends with a **deferred** `applyProviderCredentialBindings(nil)`.
For an `auth_type: oauth2` provider, that call takes the branch at
`internal/config/provider_credentials.go:155-163`, where `tokens[provider.ID]` is `""` (the map is nil),
so it sets the resolved key to `""`, the resolved URL to the sentinel `invalid://provider-oauth` and the
kind to `invalid-provider`.

That is a deliberate **fail-closed** design — good — but it means `ResolveProviders()` *alone* leaves every
OAuth provider in the invalid state, and correctness depends entirely on the caller remembering to call
`ApplyOAuthTokens(vault)` afterwards. `ResolveProviders` is exported and its own doc comment says nothing
about this precondition, while `ApplyOAuthTokens`'s comment states it in the other direction.

I checked every non-test call site: all 9 currently pair them correctly —
`cmd/aurago/main.go:761`->`764`, `internal/server/auth_handlers.go:749`->`751`,
`internal/server/config_handlers_main.go:1006`->`1007`, `internal/server/go2rtc_config.go:165`->`166`,
`internal/server/provider_transaction.go:154`->`155`,
`internal/server/realtime_speech_handlers.go:281`->`282`, `internal/server/security_check.go:916`->`917`,
`internal/server/setup_handlers.go:345`->`347`, `internal/server/sip_handlers.go:588`->`589`.
**So there is no current defect.** The risk is purely structural: the next `ResolveProviders()` call site
added without the companion call silently breaks every OAuth provider.

One site diverges from the documented order: `internal/config/outgoing_webhooks.go:238-246` calls
`Load()` -> `ApplyVaultSecrets(vault)` -> `ApplyOAuthTokens(vault)` and **never** re-runs `ResolveProviders()`,
contrary to the `ApplyVaultSecrets` comment. It appears to work because `ApplyOAuthTokens` calls
`applyProviderCredentialBindings` itself (`internal/config/config_migrate.go:740`), which is the same
propagation mechanism — but the equivalence is by coincidence of implementation, not by contract.

**Needs-verification** — Whether `ApplyVaultSecrets` changes any resolved slot that
`applyProviderCredentialBindings` does **not** cover (for example the local-LLM branch at
`internal/config/config_migrate.go:281-287`, or the manifest/omniroute branches at `:262-273`) and would
therefore be lost in that one path. To verify: diff the slot sets written by `ResolveProviders` and by
`applyProviderCredentialBindings`, then re-run the `outgoing_webhooks` save path with a local-LLM or
manifest provider selected.

**Fix** — Fold the two mechanisms into one: make `ResolveProviders` accept an optional token map, or have
`ResolveProviders` detect that a vault-backed binding is required and fail loudly rather than silently
resetting. Document the required call order on `ResolveProviders` itself.

**Confidence** — High for the mechanism and for the current call-site pairing. The residual risk in the
`outgoing_webhooks` path is explicitly needs-verification.

---

## Metrics

| Metric | Value |
|---|---|
| Packages in scope | 10 (+ `prompts/embed.go`) |
| Go files in scope | 1,064 |
| Non-test LOC in scope | 143,104 |
| Test LOC in scope | 112,219 |
| Source files read verbatim | 38 (~1,900 lines in targeted spans) |
| Test files inspected | 14 |
| graft/graph calls | ~34 (3 graft-MCP attempts, **all timed out**; ~31 graft-CLI calls succeeded) |
| Targeted read/grep verification calls | ~41 |
| Builds / test runs / formatters / codegen | 0 |
| Repository files modified | 0 |
| **Findings** | **16** |
| — CRITICAL | **0** |
| — HIGH | **2** (F-1, F-2) |
| — MEDIUM | **10** (F-3 … F-12) |
| — LOW | **4** (F-13 … F-16) |

### Severity rationale

- **F-1 / F-2 — HIGH.** Both are on the critical tool-execution path and both produce *silently wrong
  results* rather than errors: a numeric argument is discarded (F-1) or an argument's type is silently
  changed (F-2). Neither raises an error, so neither the operator nor the model can detect the mismatch.
  They stop short of CRITICAL because the failure mode is a wrong *argument value* on the intended tool,
  not a tool call being reinterpreted as a *different tool*.
- **No CRITICAL.** No exploitable security hole, no data loss/corruption, and no path where a tool call is
  silently routed to a different tool was found in this area.

### Areas examined and cleared (no finding)

Recorded so the negative results are not re-litigated:

- **Anthropic transport** (`internal/llm/anthropic_transport.go`) — `RoundTrip` (`:429`) clones the request
  with `req.Clone(req.Context())` (`:458`), so the caller's context and cancellation survive the OpenAI->
  Anthropic rewrite; it reads and closes `req.Body` (`:436-437`); the error path reads and closes the body
  (`:1153-1154`); the stream path uses `io.Pipe` with `defer body.closeOriginal()` and
  `anthropicStreamBody.Close` closes both the pipe reader and the original body (`:1221-1223`).
- **Streaming receiver teardown** (`internal/agent/agent_llm_call.go:238-264`, `:432-438`) — `recvCh` is
  buffered (1), the receiver goroutine has `defer close(recvDone)`, `defer close(recvCh)` and a `recover()`
  that converts a panic into `ErrRuntimePanic`; teardown does `cancelRecv()` -> `stm.Close()` -> wait on
  `recvDone` with a 1-second bound and logs if the receiver does not stop.
- **Retry bounds** (`internal/llm/retry.go`) — `maxRetryAttempts = 10` (`:66`); the loop aborts on context
  errors (`:271-282`), on context-limit errors (`:283-285`) and on `IsNonRetryable` (`:287-299`); every log
  statement uses `safeAPIError`.
- **Model-limit cache write-side invalidation** (`internal/llm/model_limits.go:367`) — generation-guarded,
  covered by `internal/llm/model_limits_test.go:364`.
- **Secret redaction** — no `slog`/`logger` call in `internal/{llm,agent,config}` logs an API key,
  `Authorization` or `Bearer` value; `redactProviderURL`, `redactProviderError` and `safeAPIError` are used
  throughout. `RequestTokenUsage` (`internal/agent/request_budget.go:53-69`) is content-free by design and
  carries no key.
- **Malformed tool arguments** — `internal/agent/tool_call_contract.go:67-70` sets
  `PreparationError = invalid_arguments` for `NativeArgsMalformed`, blocking execution hooks before
  dispatch; covered by `internal/agent/tool_call_contract_test.go:15`.
- **Config fail-fast on malformed YAML** — `internal/config/config.go:1054-1076` returns an error and writes
  a `.corrupted.<ts>` backup before returning, so a corrupt config cannot half-apply.
- **`ApplySIPDefaults`** (`internal/config/sip.go:159-190`) — `cfg.ReadOnly = true` (`:163`) is applied
  **before** `yaml.Unmarshal` (`internal/config/config.go:413` -> `:1054`), so YAML can override it while an
  absent key keeps the secure default. This is the intended safe-default pattern, and it is also how the
  `Tools` defaults at `internal/config/config.go:421-440` behave (the comment at `:421-423` states the
  intent explicitly). Not reported as a defect.
- **A2A prompt-injection guard** (`internal/a2a/executor.go:244-257`) — blocks on `ThreatHigh` and above,
  and the A2A system prompt (`internal/a2a/executor.go:265-269`) enumerates the prohibited capabilities.
- **Agent-loop bounds** — `maxLoopIterations` (100, capped at 750) at `internal/agent/agent_loop.go:538-541`
  with the abort at `:553-557`; the tool-call circuit breaker at `:698-714` with a default limit of 10
  (`internal/agent/agent_parse.go:2000-2011`).

---

## Not covered / follow-ups

1. **The graft MCP server was unavailable.** Every `graft` MCP call timed out (120 s; `graft check` 180 s).
   All graph work used the `graft` CLI. The CLI's own output warned that *"a graph rebuild is already in
   flight — answering from the current graph"*, so some queries may have been answered against a
   **mid-rebuild graph**. Every graph-derived claim in this report was independently re-verified against
   raw source, but a rebuild-stable re-run is worth doing before acting on the metrics.
2. **`internal/tools` consumption of decoded arguments was not audited.** In particular, the downstream
   consumer of `webScraperArgs.Limit` (`internal/agent/tool_args_builtin.go:117`) was not traced, so whether a
   **negative** or unbounded `limit` is exploitable is **unconfirmed**. To verify: read the web-scraper
   implementation in `internal/tools` and check whether `Limit` is clamped before use. This is the main
   open question behind F-1's blast radius.
3. **A full-repo marshalling audit for `llm.ModelLimits` / `llm.ModelRoute` was not completed.** The sweep
   in F-5 covered `json.Marshal(...)` call sites in `internal/`; a repo-wide sweep including `ui/`,
   `cmd/` and any reflection-based serialisation (e.g. a generic API-response encoder) was not done. To
   verify: grep the whole repo for `ModelLimits` and `ModelRoute` and confirm no response encoder
   reflects over them.
4. **No tests were executed and no benchmark was run.** The connection-pool churn in F-4 is argued from the
   construction pattern, not measured. To verify: run the server under sustained agent load and count open
   file descriptors / TCP connections per minute.
5. **`internal/agent` was read in depth for the loop, budget, tool-call and argument-decoding paths only.**
   ~92,000 non-test LOC were not line-by-line read. Areas not opened include the co-agent / specialist
   implementations, the game-maker tool chain, the memory-reflection paths, the voice/SIP paths and the
   prompt-building helpers beyond `internal/prompts` (5,272 LOC, 33 files — not read in detail).
6. **The 3,449 LOC of `internal/localllm`, the 1,035 LOC of `internal/budget`, the 605 LOC of
   `internal/vaultprompt`, the 38 LOC of `internal/promptsource` and the 15 LOC of
   `internal/providerutil` were only enumerated, not read.** `internal/localllm` in particular is a
   first-class provider path (`internal/config/config_migrate.go:281-287` routes the primary LLM to it) and
   deserves its own pass.
7. **`internal/agent/AGENTS.md` was followed but its deeper area guidance files were not all read.**
8. **F-16's residual question** (whether `ApplyVaultSecrets` writes any resolved slot that
   `applyProviderCredentialBindings` does not cover) is stated with the exact check to run, and was not
   resolved.
9. **Concurrency beyond the model-limit cache was not modelled.** The agent loop's shared state
   (`s.toolCallCount`, `s.currentLogger`, `s.gameMakerDuplicateBlocks`) is mutated across goroutines in
   `ExecuteAgentLoop`; whether those are all confined to a single request goroutine was not traced.