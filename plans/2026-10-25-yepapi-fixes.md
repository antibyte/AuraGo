# YepAPI fix implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Repair the confirmed YepAPI diagnostics bug, make the paid-call accounting honest, enforce the documented rate limit, and remove the duplication that lets the seven native tools drift apart.

**Architecture:** The success envelope carries the diagnostics it was built for. The agent dispatch collapses into one `dispatchYepAPI` helper so the enable gate, budget gate, key resolution, dispatch and cost booking exist exactly once. A typed `YepAPICallError` exported from `internal/tools` tells the agent layer whether a failed call reached the provider, so uncertain outcomes are billed instead of silently lost. A per-key token bucket throttles outgoing requests before they are sent; a 429 never triggers a retry.

**Tech Stack:** Go (`internal/tools`, `internal/agent`, `internal/config`, `internal/server`, `internal/security`), operator manual `prompts/tools_manuals/yepapi.md`, `config_template.yaml`.

## Global Constraints

- **The GitNexus index cannot be read on this checkout.** Every `impact` call fails with `Database file version: 47, Current build storage version: 42` and returns `risk: UNKNOWN`, for the project-local `.gitnexus/run.cjs` and for the store checkout alike. `risk: UNKNOWN` is unresolved, never safe. The blast radius in this plan was therefore established from the graft graph (`graft/` ships with this repo) and confirmed by text search, and the per-symbol results are listed under each task. Before committing, either run `node .gitnexus/run.cjs analyze --index-only` to rebuild the index and re-run `impact`, or state in the commit message that the graph check could not be run.
- **Never send a billable YepAPI POST twice.** `internal/tools/AGENTS.md:29` states: "YepAPI POST operations are sent once because results may be billable or mutate provider jobs." `TestYepAPIPostDoesNotRepeatUncertainBillableRequests` (`internal/tools/integration_probe_regression_test.go:33`) asserts exactly one request for connection loss, 429 and 503. No task in this plan may add an automatic retry, a backoff loop or a resend. Rate limiting may only delay a request *before* it is sent; a 429 must return guidance, never a second attempt.
- **The working tree is dirty and overlaps this plan.** `internal/tools/yepapi_youtube.go`, `prompts/tools_manuals/yepapi.md` and `internal/agent/native_tools_content.go` already carry uncommitted edits, and `internal/tools/youtube_url.go` plus `internal/tools/youtube_url_test.go` are untracked. Do not revert, stash or overwrite them. Task 4 edits the same manual and the same package; keep the in-flight YouTube-URL refactor intact.
- **No tool schema changes.** The operation enums, the property lists and the per-operation semantics stay byte-identical, so `../agotrain/training/tool_tiers.json` and `operation_contracts.json` need no reconciliation. If a schema changes later, reconcile them per `documentation/training.md`.
- **All seven YepAPI tool outputs stay isolated.** `internal/security/tool_output_trust.go:102-111` classifies every `yepapi_*` tool as `toolOutputExternal`, and `TestGuardianSanitizeToolOutputIsolatesYepAPITools` (`internal/security/guardian_test.go:146`) enforces it. Do not add a second `<external_data>` wrapper, do not strip the existing one, and do not hand-write a `Tool Output:` prefix.
- **Do not change the recorded cost amounts.** The constants `0.08 / 0.01 / 0.02 / 0.01 / 0.01 / 0.01 / 0.015` stay at their current values. Task 3 moves them into one table; Task 4 changes *when* a cost is booked, never *how much*.
- **No new UI sentence in this plan.** Task 6 adds a config-only knob. The precedent is `co_agents.budget_quota_percent`, which has no Config UI field and no locale entry. If a string does become necessary, it goes into all 16 locales in `ui/lang/config/*.json` and is never copied from English.
- **The tool manual and `internal/tools/AGENTS.md` change in the same commit as the behaviour they describe.**
- Code comments stay English. Do not commit unless the user asks.

---

## File map

- `internal/tools/yepapi.go` — success/error envelope with diagnostics, key resolution, HTTP client, client cache, typed call error.
- `internal/tools/yepapi_helpers.go` — argument helpers; the `query` fallback.
- `internal/tools/yepapi_diagnostics.go` — request-shape logging; log level.
- `internal/tools/yepapi_instagram.go` — the only consumer of the payload diagnostics.
- `internal/tools/yepapi_rate_limit.go` (new) — per-key token bucket.
- `internal/tools/yepapi_test.go` — envelope, key resolution, rate limit and dispatch tests.
- `internal/tools/yepapi_rate_limit_test.go` (new) — token bucket unit tests.
- `internal/agent/dispatch_yepapi.go` (new) — the single dispatch helper: gate, budget, key, call, cost.
- `internal/agent/agent_dispatch_exec.go` — the seven near-identical `case` blocks collapse into calls to the helper.
- `internal/agent/dispatch_yepapi_test.go` (new) — helper behaviour and cost booking.
- `internal/config/config_types.go` — `YepAPIConfig.BudgetQuotaPercent`.
- `internal/config/config.go` — YepAPI defaults block, quota clamp.
- `config_template.yaml` — the new knob, written explicitly.
- `internal/server/server_routes_config.go` — admin gate on the probe route.
- `internal/security/tool_output_trust.go` — stale registry entries.
- `prompts/tools_manuals/yepapi.md` — pricing wording, rate-limit wording.
- `internal/tools/AGENTS.md` — the new YepAPI contract lines.

---

### Task 1: Restore the success-path diagnostics

**Finding:** `yepAPIFormatSuccessWithDiagnostics` (`internal/tools/yepapi.go:223`) uses its `diagnostics` map only in the error branch (`:229`). On success the envelope is `{"status":"success","data":…}` with `sent_endpoint`, `sent_operation` and `sent_payload_keys` missing. Reproduced with a throwaway probe: output was `{"data":{"username":"natgeo"},"status":"success"}`.

**Impact (graft, `depth: all`):** callers are `formatInstagramPayloadSuccess` (`internal/tools/yepapi_instagram.go:173`) and `yepAPIFormatSuccess` (`internal/tools/yepapi.go:219`). The latter reaches all seven dispatchers and 15 test functions, so the blast radius is the whole family. The fix is contained because a `nil` diagnostics map adds no keys and leaves the envelope byte-identical for the other six tools.

**Files:** `internal/tools/yepapi.go`, `internal/tools/yepapi_test.go`

- [x] **Step 1:** Add a failing test `TestYepAPIFormatSuccessKeepsDiagnostics`. Call `yepAPIFormatSuccessWithDiagnostics(json.RawMessage(`{"username":"natgeo"}`), map[string]interface{}{"sent_endpoint":"/v1/instagram/user","sent_operation":"user","sent_payload_keys":[]string{"username_or_url"}})`. Decode the result and assert `status == "success"`, `sent_endpoint == "/v1/instagram/user"`, `sent_operation == "user"` and that `sent_payload_keys` is a `[]interface{}` holding `"username_or_url"`.
- [x] **Step 2:** Add a second case in the same test: `yepAPIFormatSuccess(json.RawMessage(`{"a":1}`))` must stay exactly `{"status":"success","data":{"a":1}}`. This pins that the other six tools keep their current envelope.
- [x] **Step 3:** Run `go test ./internal/tools -count=1 -run TestYepAPIFormatSuccessKeepsDiagnostics` and confirm it fails with the missing keys.
- [x] **Step 4:** In `yepAPIFormatSuccessWithDiagnostics`, build the payload map first, then merge the diagnostics into it exactly as `yepAPIFormatErrorWithDiagnostics` (`:203-216`) does: skip empty keys and `nil` values. Keep the existing embedded-error promotion ahead of the merge.
- [x] **Step 5:** Run `go test ./internal/tools -count=1 -run 'YepAPI|DispatchYepAPI'` and confirm success. `TestDispatchYepAPIInstagramErrorIncludesSentPayloadKeys` (`:560`) must stay green — it asserts the error path and now shares the merge helper.
- [x] **Step 6:** Extend `TestDispatchYepAPIInstagramLogsPayloadShape` (`:523`) with a success case: the returned envelope must contain `sent_endpoint` and `sent_payload_keys`, and must not contain the raw value `natgeo`.

---

### Task 2: Key resolution and log hygiene

**Finding E:** `ResolveYepAPIKey` reads the raw `p.APIKey` (`internal/tools/yepapi.go:160` and `:176`). The rest of the codebase uses `p.RuntimeAPIKey()`, which returns `""` for the masked placeholder `••••••••` and resolves an OAuth token (`internal/server/provider_handlers.go:276` and `:523`, `internal/server/provider_diagnostics.go:132`, `internal/server/ai_gateway_handlers.go:84`). `provider_diagnostics.go:134` already treats a masked credential as an error state.

**Finding F:** `logYepAPIRequestPayload` and `logYepAPIMarshaledRequestBody` log at `Warn` (`internal/tools/yepapi_diagnostics.go:38` and `:51`) on every single request. Warn means "needs attention" here; these are traces. The payload-shape log also exists for Instagram only (`internal/tools/yepapi_instagram.go:161`), while the body log covers every tool — the diagnostics are half-implemented across the family.

**Impact (graft):** `ResolveYepAPIKey` resolves only to `TestResolveYepAPIKey` in the graph; the production callers are cross-package and were confirmed by text search: `internal/agent/agent_dispatch_exec.go` (seven call sites) and `internal/server/yepapi_handlers.go:35`.

**Files:** `internal/tools/yepapi.go`, `internal/tools/yepapi_diagnostics.go`, `internal/tools/yepapi_test.go`

- [x] **Step 1:** Add a failing test `TestResolveYepAPIKeyIgnoresMaskedPlaceholder`. Build a config with one provider of type `yepapi`, `APIKey: "••••••••"`, and a vault reader that returns `ErrSecretNotFound` for `provider_<id>_api_key`. Assert `ResolveYepAPIKey` returns an error that names the missing key, and that the returned string is not the placeholder.
- [x] **Step 2:** Add a second case to the same test: a provider with `AuthType: "oauth2"` and `RuntimeOAuthToken` set must resolve to the OAuth token, not to the static `APIKey`.
- [x] **Step 3:** Run `go test ./internal/tools -count=1 -run TestResolveYepAPIKeyIgnoresMaskedPlaceholder` and confirm it fails.
- [x] **Step 4:** In `ResolveYepAPIKey`, replace both `p.APIKey` reads with `p.RuntimeAPIKey()`. Treat an empty runtime key as "not found" and fall through to the next strategy. Keep the existing strategy order and the existing error texts.
- [x] **Step 5:** Lower both diagnostic logs from `Warn` to `Debug`. `TestDispatchYepAPIInstagramLogsPayloadShape` uses a `slog.LevelDebug` handler and therefore still captures them.
- [x] **Step 6:** Add a failing test `TestYepAPIDiagnosticsAreNotLoggedAtWarn`. Log a YepAPI request through a handler set to `slog.LevelWarn` and assert the buffer contains no `[YepAPI]` line; then repeat with `slog.LevelDebug` and assert the payload-shape line is present.
- [x] **Step 7:** Extend the payload-shape logging to all seven tools. **Implemented differently than first drafted:** routing ~50 `client.Post` call sites through a `postYepAPIPayload` helper would have touched every dispatcher file, including `internal/tools/yepapi_youtube.go`, which carries uncommitted work. Instead the tool and operation travel in the context: add `WithYepAPIRequestContext(ctx, tool, operation)` to `internal/tools/yepapi_diagnostics.go`, have each of the seven dispatchers set it once on entry, and let `postOnce` emit the payload diagnostic for every request. `postInstagramPayload` (`internal/tools/yepapi_instagram.go:160`) becomes redundant and is removed; the Instagram operation name survives in the output envelope via `sent_operation`. The logged payload keys stay values-free.
- [x] **Step 8:** Run `go test ./internal/tools ./internal/security -count=1 -run 'YepAPI|Guardian'` and confirm success.

---

### Task 3: Collapse the seven dispatch blocks into one helper

**Finding:** `internal/agent/agent_dispatch_exec.go:1827-2010` holds seven `case` blocks of roughly 22 lines each, of which about 17 are word-for-word identical: enable gate, budget gate, key resolution, client lookup, dispatch, error envelope, cost booking. The divergence has already started — only the Instagram block carries an operation check and the `toolCommandErrorMessage` special case (`:1958-1987`).

**Impact (graft):** the enclosing function is `dispatchExec` (`internal/agent/agent_dispatch_exec.go:208-2024`), with 41 in-graph callers, of which the production ones are `dispatchInner` (`internal/agent/agent.go:1230`) and `dispatchExecMustHandle` (`internal/agent/agent_dispatch_invoke.go:63`). The helper keeps the switch shape, so the routing behaviour is unchanged.

**New finding — latent nil-vault panic (fixed in this task):** the old code passed `dc.Vault` (a `*security.Vault`) straight into `tools.ResolveYepAPIKey(cfg, vault)`, whose parameter is the `config.SecretReader` interface. A typed-nil pointer is not a nil interface, so the `vault == nil` guard in `readYepAPISecret` (`internal/tools/yepapi.go:192`) never fired and `(*Vault).ReadSecret` dereferenced a nil receiver at `internal/security/vault.go:199`. Observed stack: `security.(*Vault).ReadSecret(0x0, …)` ← `tools.readYepAPISecret` ← `tools.ResolveYepAPIKey` ← `agent.dispatchYepAPI`. The helper now passes a true nil interface when no vault is configured, matching the pattern already used in `internal/server/yepapi_handlers.go:31-34`. `TestDispatchYepAPIGates/missing_api_key` guards the regression.

**Files:** `internal/agent/dispatch_yepapi.go` (new), `internal/agent/agent_dispatch_exec.go`, `internal/agent/dispatch_yepapi_test.go` (new)

- [x] **Step 1:** Add a characterisation test `TestDispatchYepAPIGates` in the new `internal/agent/dispatch_yepapi_test.go`. Cover, for `yepapi_seo`: the service disabled message, the global budget message, the missing-key message, and a successful call against an `httptest` server. Assert the exact message strings that exist today so the refactor cannot change them.
- [x] **Step 2:** Add `TestDispatchYepAPIRecordsCostOnSuccess`. Point the client at an `httptest` server that returns `{"ok":true,"data":{}}`, run the dispatch with a budget tracker, and assert the tracker recorded `0.08` under the category `yepapi`.
- [x] **Step 3:** Run `go test ./internal/agent -count=1 -run 'TestDispatchYepAPI'` and confirm the new tests pass against the *current* code. They are characterisation tests, not failing tests **Done.** Equivalence was proven mechanically instead: the seven disabled messages and the seven cost constants were extracted from `git show HEAD:internal/agent/agent_dispatch_exec.go` and matched against the helper's label table and `yepAPIServiceTable` — all identical.
- [x] **Step 4:** Create `internal/agent/dispatch_yepapi.go` with:

  ```go
  type yepAPIDispatchSpec struct {
      tool      string
      enabled   bool
      cost      float64
      dispatch  func(context.Context, *tools.YepAPIClient, string, map[string]interface{}) (string, error)
  }
  ```

  The helper performs, in this exact order: service-disabled message, `IsBlocked("yepapi")` message, `tools.ResolveYepAPIKey`, `tools.GetYepAPIClient`, dispatch, error envelope, cost booking. It returns the same `Tool Output: {"status":"error","message":…}` shape as today.
- [x] **Step 5:** Replace each of the seven `case` bodies with a call to the helper. The Instagram case keeps its own operation check and `toolCommandErrorMessage` handling *before* the helper runs, so its special case does not become general.
- [x] **Step 6:** Run `go test ./internal/agent -count=1 -run 'YepAPI|DispatchExec|DispatchCloud|ToolCatalog|DiscoverTools'` and confirm success. `TestDispatchExecRoutesEveryCYDCreatorTool` and the 41 other `dispatchExec` callers must stay green.
- [x] **Step 7:** Move the cost constants into one table next to the helper, keyed by tool, with the current values unchanged. Keep the numbers identical to `0.08 / 0.01 / 0.02 / 0.01 / 0.01 / 0.01 / 0.015`.

---

### Task 4: Book cost for calls that reached the provider

**Finding:** `RecordCostForCategory("yepapi", …)` runs only on the success path (`internal/agent/agent_dispatch_exec.go:1841-1847` and the six siblings). The error path returns before booking. The manual claims "Failed requests are never charged" (`prompts/tools_manuals/yepapi.md:227`), which does not hold for three cases that the code itself creates: the 60-second client timeout (`internal/tools/yepapi.go:20`) after the provider already accepted the request; a response cut off by the 2 MB `io.LimitReader` (`internal/tools/yepapi.go:109`) that then fails to parse (`:125-131`); and an `ok:false` envelope (`:133`). The tracker therefore under-counts real prepaid spend.

**Files:** `internal/tools/yepapi.go`, `internal/agent/dispatch_yepapi.go`, `internal/agent/dispatch_yepapi_test.go`, `internal/tools/yepapi_test.go`, `prompts/tools_manuals/yepapi.md`, `internal/tools/AGENTS.md`

- [x] **Step 1:** Add a failing test `TestYepAPICallErrorReportsWhetherTheRequestWasSent`. Assert that a marshal failure and a request-build failure report `RequestSent == false`, that a transport failure, a body-read failure and a non-2xx response report `RequestSent == true`, and that the transport failure reports `OutcomeUncertain == true`.
- [x] **Step 2:** Run `go test ./internal/tools -count=1 -run TestYepAPICallErrorReportsWhetherTheRequestWasSent` and confirm it fails.
- [x] **Step 3:** Add to `internal/tools/yepapi.go`:

  ```go
  // YepAPICallError reports a failed call and whether the request reached the provider.
  type YepAPICallError struct {
      Err              error
      RequestSent      bool
      OutcomeUncertain bool
  }
  func (e *YepAPICallError) Error() string { return e.Err.Error() }
  func (e *YepAPICallError) Unwrap() error { return e.Err }
  ```

  Wrap every error path in `postOnce` accordingly: marshal and build failures are not sent; `client.Do` errors, body-read errors, non-2xx responses and parse failures are sent; a transport error and a body-read error are uncertain.
- [x] **Step 4:** Add a failing test `TestDispatchYepAPIBooksCostForUncertainOutcome`. Serve a connection that is accepted and then dropped, so the request reached the provider but the outcome is unknown. Assert the tracker recorded the full cost for that call, and that a call rejected *before* sending recorded nothing.
- [x] **Step 5:** In the Task 3 helper, after the dispatch error, use `errors.As` to detect the exported `YepAPICallError` type from `internal/tools` and book the cost when `RequestSent` is true. Keep the success-path booking exactly as it is.
- [x] **Step 6:** Run `go test ./internal/agent ./internal/tools -count=1 -run 'YepAPI|DispatchYepAPI'` and confirm success. `TestYepAPIPostDoesNotRepeatUncertainBillableRequests` must stay green — booking a cost is not a resend.
- [x] **Step 7:** Correct the manual. Replace "Failed requests are never charged." with a sentence that separates the two cases: a request the provider rejected before doing work is not charged, while a request that was sent but whose outcome is unknown may still be charged. Keep the pricing table unchanged.
- [x] **Step 8:** Add a line to `internal/tools/AGENTS.md` next to the existing YepAPI sentence: YepAPI calls that reached the provider are booked against the budget even when the outcome is unknown, and no failed call is ever resent.
- **Note:** The agent-side test lives in the new file `internal/agent/dispatch_yepapi_cost_test.go` so the existing fixture file stays untouched. `postOnce` additionally checks `ctx.Err()` before `client.Do`: a call with an already-cancelled context never reaches the provider, so it must not be booked. A complete provider response (a non-2xx status or an error envelope) is a known outcome and stays `OutcomeUncertain = false`; only a transport failure and a body that breaks while being read are uncertain.

---

### Task 5: Enforce the documented rate limit

**Finding:** the manual documents "Rate limit: 60 requests/minute per API key" (`prompts/tools_manuals/yepapi.md:228`), but no `internal/tools/yepapi*.go` file mentions a rate, 429 or `TooManyRequests`. The agent can dispatch several YepAPI calls in one parallel tool batch, so the limit is reachable. A 429 currently surfaces as the generic `yepapi: HTTP 429 - …` with no guidance.

**Constraint:** no retry. The limiter may only delay a request before it is sent, and a 429 must return guidance to the caller.

**Files:** `internal/tools/yepapi_rate_limit.go` (new), `internal/tools/yepapi_rate_limit_test.go` (new), `internal/tools/yepapi.go`, `internal/tools/yepapi_test.go`, `prompts/tools_manuals/yepapi.md`

- [ ] **Step 1:** Add a failing test `TestYepAPIRateLimiterAllowsSixtyPerMinute`. Use an injected clock. Fire 61 requests against one key and assert the 61st is delayed and the first 60 are not.
- [ ] **Step 2:** Add a failing test `TestYepAPIRateLimiterSeparatesKeys`. Assert that a second key has its own budget.
- [ ] **Step 3:** Add a failing test `TestYepAPI429ReturnsGuidanceWithoutResending`. Serve 429 from the `httptest` server, assert the error names the rate limit and that the server saw exactly one request.
- [ ] **Step 4:** Run the three tests and confirm they fail.
- [ ] **Step 5:** Implement a token bucket in `internal/tools/yepapi_rate_limit.go`: 60 tokens per key per minute, refilled continuously, keyed by API key. The bucket lives next to the client cache. Expose an internal `wait(ctx)` that blocks for the refill interval and respects `ctx.Done()`.
- [ ] **Step 6:** Call `wait(ctx)` at the start of `postOnce`, before the request is built. A cancelled context returns a clear error and sends nothing.
- [ ] **Step 7:** Add a dedicated 429 branch in `postOnce` that returns a message naming the per-minute limit and asking the caller to continue with other work, instead of the generic HTTP error text. Do not retry, do not sleep and do not resend.
- [ ] **Step 8:** Run `go test ./internal/tools -count=1 -run 'YepAPI'` and confirm success, including `TestYepAPIPostDoesNotRepeatUncertainBillableRequests`, which must still see exactly one request for 429.
- [ ] **Step 9:** Adjust the manual's rate-limit line to state that the client paces itself to the documented limit and that a 429 is reported to the model rather than retried.

---

### Task 6: Give YepAPI its own budget quota

**Finding:** the dispatch uses only the global `budgetTracker.IsBlocked("yepapi")` (`internal/agent/agent_dispatch_exec.go:1831` and the six siblings). The tracker already offers a per-category share via `IsCategoryQuotaBlocked(category, quotaPercent)` (`internal/budget/tracker.go:367`), and the co-agent integration uses exactly that pattern (`internal/agent/dispatch_platform.go:365` with `cfg.CoAgents.BudgetQuotaPercent`). A user cannot cap YepAPI spend independently of the global daily budget.

**Files:** `internal/config/config_types.go`, `internal/config/config.go`, `internal/agent/dispatch_yepapi.go`, `config_template.yaml`, `internal/config/config_test.go`

- [ ] **Step 1:** Add a failing config test `TestYepAPIBudgetQuotaPercentClamp`. Load a config with `yepapi.budget_quota_percent: -5` and assert it becomes `0`; with `35` and assert it stays `35`. Follow the clamp pattern at `internal/config/config.go:2251-2253`.
- [ ] **Step 2:** Add `BudgetQuotaPercent int \`yaml:"budget_quota_percent"\`` to `YepAPIConfig` (`internal/config/config_types.go:2790`), documented as the share of the daily budget reserved for YepAPI, `0` disables the quota.
- [ ] **Step 3:** Clamp a negative value to `0` in the YepAPI defaults block (`internal/config/config.go:942-951`).
- [ ] **Step 4:** Write `budget_quota_percent: 0` explicitly under `yepapi:` in `config_template.yaml`, with a comment matching the `co_agents` wording. `TestRepositoryConfigTemplateYAMLIsParseable` and the template load test must stay green.
- [ ] **Step 5:** In the Task 3 helper, add the quota gate next to the global budget gate: `IsCategoryQuotaBlocked("yepapi", cfg.YepAPIBudgetQuotaPercent)` returns a distinct message naming the quota. Order matters: the global budget message stays first, matching the co-agent order at `internal/agent/dispatch_platform.go:362-367`.
- [ ] **Step 6:** Add a failing test `TestDispatchYepAPIBlocksOnCategoryQuota` that drives the tracker past the quota share and asserts the quota message, then a test that a quota of `0` disables the gate.
- [ ] **Step 7:** Run `go test ./internal/agent ./internal/config -count=1 -run 'YepAPI|ConfigTemplate|Budget'` and confirm success.

---

### Task 7: Tighten the argument fallback and clean up dead entries

**Finding G:** `stringArgWithFallback` (`internal/tools/yepapi_helpers.go:7`) falls back to `args["query"]` for *every* string parameter. The tool schemas are flat — one property set for all operations of a tool (`internal/agent/native_tools_edge.go:120-126`) — so the fallback absorbs a real ambiguity: `domain_overview` wants `domain`, `backlinks` wants `target`, `onpage` wants `url`. A `query` in those slots produces a wrong-domain lookup whose error surfaces far from the cause.

**Impact (graft, `depth: all`):** 12 production callers across all seven dispatchers plus 12 test symbols. This is the widest blast radius in the plan, so it runs last and only where the semantics are unambiguous.

**Finding H:** the client cache is keyed by the raw API key (`internal/tools/yepapi.go:57`), so the secret lives as a map key in memory and the map only ever grows as keys rotate. `security.RegisterSensitive(apiKey)` covers the field, not the cache key.

**Finding J:** `YepAPI.ProviderType` is assigned at `internal/config/config_migrate.go:686` and never read anywhere. **Finding K:** `internal/security/tool_output_trust.go:103-107` lists `yepapi_discover`, `yepapi_fetch` and `yepapi_search`, which are not native tools in this repo. **Finding L:** `/api/yepapi/test` (`internal/server/server_routes_config.go:293`) is registered without `requireAdmin`, unlike `/api/ansible/test` on the next line.

**Files:** `internal/tools/yepapi_helpers.go`, `internal/tools/yepapi.go`, `internal/config/config_migrate.go`, `internal/security/tool_output_trust.go`, `internal/server/server_routes_config.go`

- [ ] **Step 1:** Add a failing test `TestYepAPIStrictArgumentValidation`. For SEO `domain_overview`, pass `query: "example.com"` and assert the error names the expected `domain` parameter. Repeat for `backlinks` (expects `target`) and `onpage` (expects `url`). Assert that `keywords` still accepts `query` as its alias, because there the alias is semantically correct.
- [ ] **Step 2:** Replace the blanket fallback with an explicit per-operation alias table: a helper `stringArgForOperation(args, operation, key, aliases…)` that reads the canonical key first and only then the listed aliases. Every call site declares its own aliases; none of them falls back to `query` implicitly.
- [ ] **Step 3:** For the domain-shaped operations, validate the value: a domain must not contain a scheme, a path or whitespace; a URL must be an `http`/`https` URL. On failure return a message that names both the parameter and the expected shape. `yepapi_scrape` already validates its URL; reuse that helper.
- [ ] **Step 4:** Run `go test ./internal/tools -count=1 -run 'YepAPI'` and confirm success, including the 12 tests that reach `stringArgWithFallback`.
- [ ] **Step 5:** Hash the client cache key: `sha256(apiKey + "|" + baseURL)` as the `sync.Map` key, keeping the client fields unchanged. Add a test that two different keys produce two clients and that the same key reuses one.
- [ ] **Step 6:** Delete the `YepAPI.ProviderType` field and its assignment in `internal/config/config_migrate.go:686`. Confirm with a grep that no reader exists; if one appears in the meantime, keep the field and instead use it in `ResolveYepAPIKey` to select the first provider by type.
- [ ] **Step 7:** Remove `yepapi_discover`, `yepapi_fetch` and `yepapi_search` from `internal/security/tool_output_trust.go`. The remaining seven stay, and the `default` branch stays `toolOutputExternal`.
- [ ] **Step 8:** Wrap the probe route in `requireAdmin(s, …)`, matching `/api/ansible/test`. The endpoint keeps its current behaviour: it still probes the public model catalog, still reports `authentication_verified: false` and still costs nothing.
- [ ] **Step 9:** Run `go test ./internal/server ./internal/security ./internal/config ./internal/tools -count=1 -run 'YepAPI|Provider|Guardian|Config'` and confirm success.

---

### Task 8: Lock the three operation sources together

**Finding:** the operation set exists in three places — the JSON schema enums, the dispatcher switch statements and `prompts/tools_manuals/yepapi.md`. Today all three agree across 108 operations (SEO 8, SERP 13, Scrape 7, YouTube 32, TikTok 18, Instagram 19, Amazon 11). Nothing enforces that, so the next added operation can drift silently.

**Files:** `internal/tools/yepapi_test.go`

- [ ] **Step 1:** Add a test `TestYepAPIOperationsMatchSchemaDispatcherAndManual`. Parse the `operation` enums out of the native tool schemas, collect the operation names accepted by each `DispatchYepAPI*` function, and parse the bullet list of each tool section in `prompts/tools_manuals/yepapi.md`. Assert the three sets are equal per tool. Report the symmetric difference in the failure message so a mismatch names the missing and the extra operation.
- [ ] **Step 2:** Confirm the test passes against the current tree. If it fails, fix the drift in this plan rather than loosening the assertion.
- [ ] **Step 3:** Keep the test next to the existing `TestDispatchYepAPI*ParityEndpoints` tests so the parity idea lives in one place.

---

## Done when

- `go test ./internal/tools ./internal/agent ./internal/config ./internal/server ./internal/security ./ui -count=1 -run 'YepAPI|DispatchYepAPI|Provider|Guardian|Config|DispatchExec|ToolCatalog|DiscoverTools'` passes.
- A successful Instagram call carries `sent_endpoint`, `sent_operation` and `sent_payload_keys` in its envelope; the other six tools keep `{"status":"success","data":…}` byte-identical.
- A YepAPI call that reached the provider but ended in an unknown outcome is booked against the budget; a call that never left the client is not.
- No YepAPI POST is ever sent twice: `TestYepAPIPostDoesNotRepeatUncertainBillableRequests` still sees exactly one request for connection loss, 429 and 503.
- 61 YepAPI calls in one minute against one key are paced, and the 61st waits; a second key has its own budget.
- `yepapi.budget_quota_percent` loads, clamps a negative value to `0`, blocks the category once the share is spent and disables the gate at `0`.
- The seven native tool schemas, the seven dispatchers and `prompts/tools_manuals/yepapi.md` describe the same operation set, enforced by a test.
- The GitNexus index is rebuilt (`node .gitnexus/run.cjs analyze --index-only`) and `impact` returns a real risk for every touched symbol, or the commit message states that the graph check could not be run because of the storage-version mismatch.
