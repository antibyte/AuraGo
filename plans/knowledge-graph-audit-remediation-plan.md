# Knowledge Graph Audit Review and Remediation Plan

Date: 2026-04-19
Source report reviewed: `reports/kg_audit.md`

## Assessment Summary

The report is useful, but only partially reliable in its current form.

- Confirmed: several operational and architectural findings are valid.
- Partially correct: some findings identify a real risk but overstate the current implementation gap.
- Incorrect or outdated: a few claims do not match the current codebase and should not drive prioritization.

Recommended confidence rating for the report: `medium`.

## Confirmed Findings

The following findings are directly supported by the current code:

1. Silent KG write failures exist in maintenance and activity capture paths.
   Evidence:
   - `internal/agent/maintenance.go:986`
   - `internal/agent/maintenance.go:1318`
   - `internal/agent/maintenance.go:1319`
   - `internal/agent/activity_capture.go:191`
   - `internal/agent/activity_capture.go:206`

2. Activity entity normalization can collapse distinct labels into the same ID.
   Evidence:
   - `internal/agent/activity_capture.go:211`

3. KG API shape is inconsistent.
   Evidence:
   - `internal/memory/graph_query.go:13` returns JSON string for `Search`
   - `internal/memory/graph_explore.go:9` returns JSON string for `Explore`
   - `internal/memory/graph_explore.go:69` returns JSON string for `SuggestRelations`
   - typed APIs exist alongside them such as `GetNode`, `GetNeighbors`, `GetSubgraph`

4. `DeleteEdge` can desynchronize semantic index and SQLite on SQL failure.
   Evidence:
   - `internal/memory/graph_edge.go:57`

5. `GetRecentChanges` duplicates property decoding logic instead of using the shared helper.
   Evidence:
   - `internal/memory/graph_node.go:348`

6. `Stats()` can return `-1` values that are surfaced by dashboard handlers.
   Evidence:
   - `internal/memory/graph_sqlite.go:275`
   - `internal/server/dashboard_handlers_memory.go:37`
   - `internal/server/dashboard_handlers_memory.go:212`

7. `SearchForContext` performs per-node follow-up queries and is a real N+1 hotspot.
   Evidence:
   - `internal/memory/graph_query.go:283`
   - `internal/memory/graph_query.go:305`

8. Semantic query cache eviction is nondeterministic and not true LRU behavior.
   Evidence:
   - `internal/memory/graph_semantic.go:556`

9. File KG sync is currently sequential.
   Evidence:
   - `internal/services/file_kg_sync.go:74`
   - `internal/services/file_kg_sync.go:120`

## Incorrect Or Outdated Findings

The following report statements do not match the current repository state:

1. Missing KG read-only protection in the dispatcher.
   Current code already blocks write operations when `tools.knowledge_graph.readonly` is enabled.
   Evidence:
   - `internal/agent/agent_dispatch_exec.go:392`
   - `internal/config/config_types.go:1079`

2. No tests for `FileKGSyncer`.
   This is false.
   Evidence:
   - `internal/services/file_kg_sync_test.go:13`

3. No tests for `kgextraction`.
   This is false.
   Evidence:
   - `internal/kgextraction/kg_extraction_test.go:82`

4. No tests for `GetSubgraph` beyond depth 2.
   This is overstated. There is coverage for BFS behavior and a depth-3 cycle case, although deeper result assertions could still be improved.
   Evidence:
   - `internal/memory/graph_sqlite_test.go:849`
   - `internal/memory/graph_sqlite_test.go:887`

5. Semantic index is maintained in a completely separate DB path by default.
   This is outdated. Current startup uses `EnableSemanticSearchShared`, which reuses the long-term memory chromem DB.
   Evidence:
   - `cmd/aurago/main.go:777`
   - `internal/memory/graph_semantic.go:70`

## Findings That Need Reframing

1. Semantic consistency risk is real, but the problem is not "missing on all mutation paths".
   - `BulkAddEntities` and `BulkMergeExtractedEntities` already update node and edge semantic indexes after commit.
   - `MergeNodes` still does not reconcile semantic index state and remains a valid gap.
   Evidence:
   - `internal/memory/graph_backup.go:65`
   - `internal/memory/graph_backup.go:153`
   - `internal/memory/graph_node.go:381`

2. Test coverage is mixed rather than broadly absent.
   - There is meaningful coverage for optimize, subgraph traversal, extraction, handlers, and file sync basics.
   - Real gaps still remain around merge behavior, failure handling, and load/performance scenarios.

## Recommended Remediation Plan

## Phase 1: Safety and correctness

Goal: remove silent corruption and misleading behavior first.

1. Replace ignored KG write errors in maintenance and activity capture with structured logging.
   Acceptance:
   - No `_ = kg.AddNode`, `_ = kg.AddEdge`, or `_ = kg.IncrementCoOccurrence` remain in KG sync paths.
   - Failures include context fields such as node IDs, relation, source path, or group key.

2. Fix `DeleteEdge` ordering so SQLite delete succeeds before semantic index removal, or make the sequence compensating.
   Acceptance:
   - Semantic index and SQL delete order is consistent.
   - A regression test covers SQL-delete failure behavior.

3. Stop leaking negative stats values to UI consumers.
   Acceptance:
   - Dashboard never displays negative KG counts.
   - Error state is logged or exposed separately.

4. Review `MergeNodes` semantic consistency.
   Acceptance:
   - Merging nodes updates or rebuilds semantic entries for affected nodes and edges.
   - Add dedicated merge tests.

## Phase 2: Data quality

Goal: improve graph quality so later performance work is worth doing.

1. Redesign `normalizeActivityEntityID`.
   Recommended direction:
   - preserve separators as normalized `_`
   - keep casing normalized to lower-case
   - optionally add collision-resistant suffixing only when needed
   Acceptance:
   - `"Docker Compose"` and `"Docker-Compose"` remain compatible
   - unrelated collapsed forms do not merge unexpectedly
   - add table-driven collision tests

2. Centralize node property decoding.
   Acceptance:
   - `GetRecentChanges` uses shared decoding/sanitizing helper
   - duplicated JSON parsing logic is reduced

3. Decide whether string-returning KG APIs remain legacy-only or should be replaced.
   Acceptance:
   - one clear API convention is documented
   - agent/server callers are aligned with that convention

## Phase 3: Performance

Goal: remove the most obvious query and throughput bottlenecks without risky redesign first.

1. Remove `SearchForContext` N+1 edge queries.
   Acceptance:
   - edges for matched node IDs are fetched in batch
   - add benchmark or focused test coverage around the query path

2. Parallelize file KG sync with a bounded worker pool.
   Acceptance:
   - concurrency is configurable or conservatively bounded
   - ordering-sensitive cleanup remains safe
   - add tests for partial failure aggregation

3. Replace nondeterministic semantic query cache eviction with deterministic LRU or TTL+ordered eviction.
   Acceptance:
   - no random map-iteration eviction
   - cache size bound remains enforced

4. Revisit expensive importance and relation suggestion queries after instrumentation.
   Acceptance:
   - measure before changing SQL shape
   - only optimize hotspots confirmed by profiling

## Phase 4: Coverage and observability

Goal: make future KG changes safer.

1. Add missing tests for:
   - `MergeNodes`
   - failure behavior around semantic index updates
   - maintenance sync logging/error handling
   - file sync parallel execution semantics

2. Add lightweight metrics/log counters for:
   - KG write failures
   - semantic reindex failures
   - dropped access-hit queue events
   - file sync throughput and failures

## Implementation Order

Recommended execution order:

1. Safety fixes in maintenance/activity capture and `DeleteEdge`
2. Dashboard stats hardening
3. `MergeNodes` consistency fix plus tests
4. Activity entity ID normalization redesign
5. `SearchForContext` batching
6. File KG sync worker pool
7. Cache eviction cleanup
8. API consistency refactor

## What Not To Prioritize Yet

Avoid spending the next sprint on these until the earlier fixes land:

- splitting KG packages into more files
- large interface abstraction work
- backup/restore naming cleanup only
- deep SQL rewrites without profiler evidence

## Final Recommendation

Treat `reports/kg_audit.md` as a good starting signal, not as a fully reliable source of truth.

Use it to drive:

- safety fixes
- semantic consistency fixes
- targeted performance work

Do not use it to drive:

- read-only permission redesign
- broad claims of missing test coverage
- assumptions based on the old semantic-index startup model
