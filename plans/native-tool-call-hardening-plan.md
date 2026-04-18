# Native Tool Call Hardening Plan

## Scope

Validate [`reports/native_tool_call_analysis.md`](../reports/native_tool_call_analysis.md) against the current codebase and focus remediation on the issues that are still real and worth tackling now.

Primary code areas:

- [`internal/agent/native_tools.go`](../internal/agent/native_tools.go)
- [`internal/agent/native_tools_registry.go`](../internal/agent/native_tools_registry.go)
- [`internal/agent/agent_parse.go`](../internal/agent/agent_parse.go)
- [`internal/agent/agent_dispatch_exec.go`](../internal/agent/agent_dispatch_exec.go)
- [`internal/agent/tool_call_pipeline.go`](../internal/agent/tool_call_pipeline.go)
- [`internal/agent/agent_loop.go`](../internal/agent/agent_loop.go)

## Report validation result

The report is directionally useful, but it overstates a few testing and observability gaps.

### Confirmed current issues

1. `dispatchExec()` is still a large switch-based execution monolith in [`agent_dispatch_exec.go`](../internal/agent/agent_dispatch_exec.go).
2. `ParseToolCall()` is still a large multi-format parser monolith in [`agent_parse.go`](../internal/agent/agent_parse.go).
3. `ExecuteAgentLoop()` is still very large and mixes many responsibilities in [`agent_loop.go`](../internal/agent/agent_loop.go).
4. `NativeToolCallToToolCall()` still compiles fallback regexes inside the hot path in [`native_tools.go`](../internal/agent/native_tools.go).
5. `injectAdditionalPropertiesRec()` still has no cycle guard in [`native_tools.go`](../internal/agent/native_tools.go).
6. `ToolFeatureFlags.Key()` still uses `json.Marshal` for cache keys in [`native_tools_registry.go`](../internal/agent/native_tools_registry.go).
7. `_todo` is still injected into every tool schema and even appended to `required` in [`native_tools.go`](../internal/agent/native_tools.go).
8. Recovery still trims message history in [`tool_call_pipeline.go`](../internal/agent/tool_call_pipeline.go), so the behavior deserves tighter guardrails and clearer logging even though it is intentional.
9. Structured tool-execution error types are still missing from the main dispatch path.

### Report items that are partially outdated

1. Test coverage is better than the report states:
   - `ParseToolCall()` already has focused coverage in [`tool_call_pipeline_test.go`](../internal/agent/tool_call_pipeline_test.go) and [`agent_parse_guardian_test.go`](../internal/agent/agent_parse_guardian_test.go).
   - `dispatchExec()` already has targeted tests in [`agent_dispatch_exec_test.go`](../internal/agent/agent_dispatch_exec_test.go), [`agent_dispatch_exec_query_memory_test.go`](../internal/agent/agent_dispatch_exec_query_memory_test.go), and related files.
   - 422 / empty-response recovery already has dedicated tests in [`tool_call_pipeline_test.go`](../internal/agent/tool_call_pipeline_test.go).
2. Observability is not absent:
   - parse-source, recovery-event, policy-event, retrieval-event, and scoped tool-result telemetry already exist in [`telemetry.go`](../internal/agent/telemetry.go).
   - what is still missing is finer-grained per-tool latency/error telemetry, not telemetry in general.
3. Custom-tool collision handling is not uniformly silent:
   - schema generation still only warns and skips on collisions
   - but `save_tool` already rejects built-in collisions in tests and runtime paths

## Remediation goals

- Reduce hot-path overhead and correctness risk without destabilizing the current tool pipeline.
- Improve maintainability in the parser and dispatch layers through staged extraction, not one giant rewrite.
- Tighten recovery and schema behavior where the current implementation is too broad or too implicit.
- Expand tests around the real weak points rather than rebuilding coverage that already exists.

## Phase 1: Low-risk correctness and performance wins

### 1. Pre-compile native fallback regexes

Relevant code:

- [`internal/agent/native_tools.go`](../internal/agent/native_tools.go)

Current issue:

`NativeToolCallToToolCall()` still builds regexes inside the malformed-JSON fallback path via the local `extractField()` helper.

Implementation direction:

- Move the fallback patterns to package-level compiled regex templates or a small cached helper.
- Keep the same rescue behavior for truncated argument JSON.
- Add a focused test for malformed native-tool arguments so the fallback remains stable.

### 2. Add cycle protection to schema recursion

Relevant code:

- [`internal/agent/native_tools.go`](../internal/agent/native_tools.go)

Current issue:

`injectAdditionalPropertiesRec()` recursively walks nested schema maps without a visited set.

Implementation direction:

- Add a visited map keyed by map identity or pointer-equivalent wrapper state.
- Preserve the current behavior of not overwriting explicit `additionalProperties`.
- Add a regression test that uses a synthetic self-referential schema graph.

### 3. Replace JSON cache keys for `ToolFeatureFlags`

Relevant code:

- [`internal/agent/native_tools_registry.go`](../internal/agent/native_tools_registry.go)

Current issue:

`ToolFeatureFlags.Key()` still uses `json.Marshal(ff)` as the cache key.

Implementation direction:

- Replace it with a deterministic, cheap key builder over active flag names.
- Keep test coverage that verifies equal flags produce equal keys and different flags produce different keys.
- Avoid reflection-heavy complexity if a generated or hand-maintained flag-name list is simpler and safer.

### 4. Improve recovery-path logging and traceability

Relevant code:

- [`internal/agent/tool_call_pipeline.go`](../internal/agent/tool_call_pipeline.go)

Current issue:

Recovery already logs high-level events, but it still does not clearly surface which messages were trimmed or why a specific trim path was chosen.

Implementation direction:

- Add structured logging for recovery trims:
  - trigger category
  - retry count
  - before/after message counts
  - whether the last user intent was explicitly preserved
- Do not log raw sensitive content.

## Phase 2: Behavior tightening without major architecture risk

### 5. Revisit global `_todo` schema injection

Relevant code:

- [`internal/agent/native_tools.go`](../internal/agent/native_tools.go)
- [`internal/agent/agent.go`](../internal/agent/agent.go)

Current issue:

Every tool schema receives `_todo`, and the field is also forced into `required`. This is broad, token-costly, and surprising for tools that do not use task piggybacking.

Implementation direction:

- Audit which tools actually benefit from `_todo`.
- Move toward one of these models:
  - inject `_todo` only for selected tools
  - inject `_todo` globally but keep it optional
  - move task tracking out of tool schemas entirely into prompt/state handling
- Preserve backward compatibility for existing model behavior during rollout.

Tests to add:

- schema test proving `_todo` is present only where intended
- compatibility test for tool calls that omit `_todo`

### 6. Introduce structured tool-execution errors

Relevant code:

- [`internal/agent/agent_dispatch_exec.go`](../internal/agent/agent_dispatch_exec.go)
- tool-execution policy and recovery helpers around it

Current issue:

The execution path still largely communicates failure via formatted strings.

Implementation direction:

- Introduce a small internal `ToolExecError` type with:
  - code
  - message
  - retryability flag
  - optional details
- Start by converting shared infrastructure helpers and a small subset of high-churn tools.
- Avoid a full-system error rewrite in one pass.

Benefits:

- clearer recovery classification
- more reliable policy handling
- easier telemetry enrichment later

### 7. Tighten history trimming safeguards

Relevant code:

- [`internal/agent/tool_call_pipeline.go`](../internal/agent/tool_call_pipeline.go)

Current issue:

The trim behavior is intentional and already tested, but it still discards context aggressively under some failures.

Implementation direction:

- Keep the core recovery behavior.
- Add stronger invariants:
  - always preserve system prompt
  - always preserve the latest meaningful user intent
  - preserve the latest relevant tool result when the failure is not specifically tool-history corruption
- Split provider-422 trimming and empty-response trimming into more explicit policies.

Tests to add:

- preserve-last-user-intent regression
- preserve-latest-relevant-tool-result when recovery reason is empty response
- distinct tests for 422 history repair versus empty-response compaction

## Phase 3: Maintainability refactors in controlled slices

### 8. Extract parser helpers from `ParseToolCall()`

Relevant code:

- [`internal/agent/agent_parse.go`](../internal/agent/agent_parse.go)

Current issue:

`ParseToolCall()` is still too large and combines many formats inline.

Implementation direction:

- Do not jump straight to a full parser-interface chain.
- First extract stable helpers by format:
  - bracket format
  - `<action>` / XML variants
  - MiniMax variants
  - fenced/raw JSON variants
  - raw code detection
- Keep one coordinating entry point so behavior does not change unexpectedly.

Tests to add:

- add format-specific table tests per extracted helper
- keep end-to-end `ParseToolCall()` compatibility tests

### 9. Break up `dispatchExec()` by tool family

Relevant code:

- [`internal/agent/agent_dispatch_exec.go`](../internal/agent/agent_dispatch_exec.go)

Current issue:

The execution switch is still the main maintainability bottleneck.

Implementation direction:

- Refactor by families, not by 60 isolated files immediately.
- First carve out stable families into helper files/functions, for example:
  - memory/context tools
  - execution/python/shell tools
  - remote/network/platform tools
  - planning/content tools
- Keep a thin top-level dispatcher while reducing per-file size.

Tests to expand:

- family-specific unit tests for newly extracted helpers
- small dispatch-table smoke test that action names still resolve correctly

### 10. Decompose `ExecuteAgentLoop()` only after the lower layers stabilize

Relevant code:

- [`internal/agent/agent_loop.go`](../internal/agent/agent_loop.go)

Current issue:

The loop is large, but it also sits at the center of many other moving pieces.

Implementation direction:

- Delay deep loop refactoring until parser/dispatch cleanup is done.
- Extract around stable concerns first:
  - context preparation
  - RAG retrieval and context shaping
  - tool-turn handling
  - recovery / retry orchestration
- Preserve exact behavior with snapshot-style tests around the extracted seams where practical.

## Phase 4: Telemetry expansion

### 11. Extend existing telemetry instead of inventing a parallel system

Relevant code:

- [`internal/agent/telemetry.go`](../internal/agent/telemetry.go)
- tool execution paths in the loop and dispatch layers

Current issue:

Telemetry exists, but it is missing per-tool latency and richer execution-failure classification.

Implementation direction:

- Add scoped counters or summaries for:
  - per-tool execution success/failure
  - per-tool family latency buckets
  - malformed native-args recovery hits
  - parse-format fallbacks
- Reuse the existing scoped/global telemetry collector.

## Execution order

1. Pre-compile fallback regexes.
2. Add cycle protection to schema recursion.
3. Replace JSON cache keys for `ToolFeatureFlags`.
4. Improve recovery logging.
5. Rework `_todo` injection to be less global or less strict.
6. Introduce structured execution errors in shared paths.
7. Tighten history-trimming safeguards.
8. Extract parser helpers from `ParseToolCall()`.
9. Break up `dispatchExec()` by tool family.
10. Decompose `ExecuteAgentLoop()` last.
11. Extend the existing telemetry system with per-tool timing/error metrics.

## Notes

- Do not plan work based on the report’s “tests are mostly missing” claim; the repo already contains meaningful coverage for parsing, recovery, native conversion, and parts of dispatch.
- Do not replace the existing telemetry layer with a new one; build on the current scoped telemetry infrastructure.
- Treat the parser and dispatcher as refactoring targets, not emergency bug-fix targets. The first safe gains are hot-path cleanup, recursion safety, and clearer recovery behavior.
