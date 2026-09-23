# Memory

## Purpose

Memory retrieval, hygiene, indexing, and maintenance.

## Ownership

`internal/memory` owns this domain. The contracts below also bind related server, UI, config, asset, and test work through the root routing table.

## Local Contracts

### Memory System
- **Short-Term**: SQLite sliding-window conversation context
- **Long-Term**: Vector database with semantic search (chromem-go)
- **Knowledge Graph**: Entity-relationship store for structured facts
- **Core Memory**: Permanent facts always included in context
- **Native Chunking**: File, documentation, and tool-guide indexing use the Go `internal/chunking` package. `indexing.chunking` defaults to recursive chunking with 3,500 chars, 200 overlap, and 200 chunks per file; chunking parameters are part of index fingerprints so config changes trigger clean reindexing.
- **On-Demand Context**: Auto-RAG and KG prompt injection keep only essential context in the prompt and expose additional `[memory:<id>]` / `[kg:<id>]` teasers for `recall_memory` and `explore_kg`.
- **Memory Hygiene**: Dashboard and nightly maintenance can safely consolidate exact auto-generated journal error duplicates, archive stale low-priority notes with per-run limits and repeated-failure tracking, repair tracked canonical VectorDB names, and raise review issues for KG/Core Memory health. Automatically extracted claims that a tool, function, API, or endpoint is broken are transient and must not enter long-term memory; explicit user-managed memory remains available. Notes marked `protected` or `keep_forever` are excluded from auto-archive. Review-only/high-risk memory findings must not be auto-deleted.
- **Skill Quality Maintenance**: Nightly maintenance reviews Python skills and `SKILL.md` packages only when persisted provenance is exactly `agent`. Unknown disk discoveries remain `legacy_unknown`; user/system/curated skills are immutable to this phase. Improvement requires classifier confidence at least 0.95 plus complete staging validation and a clean security result. Deletion requires confidence at least 0.98 plus deterministic objective evidence, permanently removes files/registry/versions, and retains only a source-free maintenance tombstone. Missing usage alone never justifies deletion; read-only, ambiguity, cancellation, credential signals, scan warnings, fixed references, or failed daemon stops always prevent mutation.
- **FTS Migration State**: External-content FTS5 indexes for notes, journal entries, episodic memories, and activity turns use version `1` markers in `memory_schema_meta` (`fts.notes`, `fts.journal_entries`, `fts.episodic_memories`, `fts.activity_turns`). Missing or outdated markers require an FTS5 `rebuild`; write the marker only after a successful rebuild.
- **Maintenance Safety**: Follow `documentation/maintenance.md`. Vector writes distinguish created, reused and unknown ownership; rollback never deletes reused/unknown IDs. Compression and canonical repair share force-new replacement with complete metadata comparison and transactional archive/copy. Preserve replacements after commit or uncertain commit. Canonical scans are capped at 100 rows with a persistent keyset cursor in `memory_maintenance_meta`.
- **Maintenance Lifecycle**: Skill cancellation and mutation state must preserve recoverable files and safe daemon restoration; source drift cannot inherit security approval. Each phase has one stable issue fingerprint and resolves only after a persisted success. The server-owned controller follows immutable config snapshots, local calendar/DST scheduling, a durable started-day claim, and shutdown before database close. Summaries cover at most three missing journal dates from the last seven completed days, insert-only; current-day summaries are excluded.

## Verification

- Run `go test ./internal/memory` and the named cross-component checks in the contracts above when those paths change.

## Child DOX Index

None.
