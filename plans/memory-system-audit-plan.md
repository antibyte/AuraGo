# Memory System Audit Plan

## Scope

Audit the memory subsystem with priority on real defects, data-loss risks, and race conditions.

Primary code areas:

- [`internal/memory/short_term_init.go`](internal/memory/short_term_init.go)
- [`internal/memory/short_term.go`](internal/memory/short_term.go)
- [`internal/memory/long_term.go`](internal/memory/long_term.go)
- [`internal/memory/graph_sqlite.go`](internal/memory/graph_sqlite.go)
- [`internal/memory/history.go`](internal/memory/history.go)
- [`internal/agent/maintenance.go`](internal/agent/maintenance.go)

## Initial findings to validate in the report

### 1. High risk: pinned messages can still be deleted during STM archival

Relevant logic:

- [`memory.SQLiteMemory.DeleteOldMessages()`](internal/memory/short_term.go:199)
- pinned flag stored in [`internal/memory/short_term_init.go`](internal/memory/short_term_init.go)

Observation:

`DeleteOldMessages` deletes all rows older than the cutoff with `id < ?` and does not exclude `is_pinned = 1` rows. This creates a direct data-loss path for messages explicitly marked for retention.

Impact:

- pinned conversation anchors can disappear
- archival behavior violates semantic expectation of pinning
- consolidation may persist only part of the intended preserved context

Report action:

- confirm whether pinning is expected to protect against pruning
- propose SQL safeguard and regression tests for pinned STM messages

### 2. High risk: consolidation candidates are not claimed before processing

Relevant logic:

- [`memory.SQLiteMemory.GetConsolidationCandidates()`](internal/memory/short_term.go:264)
- [`consolidateSTMtoLTM()`](internal/agent/maintenance.go:762)
- [`memory.SQLiteMemory.MarkConsolidationSuccess()`](internal/memory/short_term.go:309)
- [`memory.SQLiteMemory.MarkConsolidationFailure()`](internal/memory/short_term.go:329)

Observation:

Candidates are selected with status `pending` or retryable `failed`, then processed, then marked later. There is no lease, claim, or `in_progress` transition before LLM/vector work starts.

Impact:

- parallel maintenance runs can process the same archived rows twice
- duplicated long-term memories can be produced
- retry counters and success markers can race each other

Report action:

- recommend atomic claim step in SQLite transaction
- recommend `in_progress` or lease timestamp state
- add concurrent consolidation regression test

### 3. Medium to high risk: KG access counter updates are silently dropped under pressure

Relevant logic:

- [`KnowledgeGraph.enqueueAccessHit()`](internal/memory/graph_sqlite.go:311)

Observation:

When the buffered queue is full, access updates are dropped intentionally with a debug log.

Impact:

- no direct corruption, but telemetry-driven ranking becomes lossy
- importance and recency heuristics can drift under load
- debugging production relevance issues becomes harder

Report action:

- classify as observability and model-quality degradation, not hard corruption
- evaluate bounded retry, larger queue, or batch fallback counters

### 4. Medium risk: vector dedup path serializes storage heavily and may bottleneck maintenance

Relevant logic:

- [`ChromemVectorDB.StoreDocumentWithDomain()`](internal/memory/long_term.go:303)
- [`ChromemVectorDB.StoreBatch()`](internal/memory/long_term.go:775)
- [`ChromemVectorDB.searchTopSimilarityScore()`](internal/memory/long_term.go:1146)

Observation:

Dedup check plus store are serialized with `storeDocMu`. In batch mode, each goroutine still enters the same serialized critical section after semaphore acquisition.

Impact:

- low concurrency for persistence-heavy consolidation runs
- expensive embedding-backed dedup can become a throughput choke point
- less a correctness bug, more a scalability hotspot

Report action:

- separate correctness lock from coarse global serialization where possible
- consider concept hash prefilter or collection-side uniqueness metadata

### 5. Medium risk: history persistence uses direct overwrite instead of atomic rename

Relevant logic:

- [`HistoryManager.save()`](internal/memory/history.go:184)

Observation:

History is persisted via `os.WriteFile` directly. If interrupted during write, the file can be truncated or partially written.

Impact:

- chat history file can be corrupted on crash or forced shutdown
- subsystem recovers by starting fresh, which hides silent data loss

Report action:

- recommend atomic temp-file plus rename strategy similar to config persistence
- add crash-safety note in report

## Planned report structure

1. Executive summary
2. Architecture slice of STM, LTM, KG, history, consolidation
3. Confirmed defect candidates
4. Data-loss and race-condition matrix
5. Optimization opportunities
6. Recommended remediation order
7. Suggested tests to add

## Execution checklist

- [x] Identify memory subsystem entry points and storage backends
- [x] Sample core implementations and related tests
- [x] Prioritize defect analysis around data loss and races
- [ ] Write the actual audit report in [`reports/`](reports/)
- [ ] If approved, switch to implementation mode for report creation and optional fixes
