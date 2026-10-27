# Memory

## Purpose

Memory retrieval, hygiene, indexing, and maintenance.

## Ownership

`internal/memory` owns this domain. The contracts below also bind related server, UI, config, asset, and test work through the root routing table.

## Local Contracts

### Personality evidence

- Only ingress-owned `HumanInput` supplies current human evidence. Missing or
  empty proof never falls back to history, media, snapshots or model JSON.
- `ApplyPersonalityObservation` atomically commits numeric state, profile
  contributions, provenance, narration and receipts. Expire original events after
  90 days before mutation; never evict valid receipts to satisfy a row cap.
- Normal model/tool/history/reflection readers must never read
  `personality_legacy_review`. Administrative adoption is explicit manual curation;
  old emotion/inner-voice text cannot be reactivated.
- Chat privacy jobs capture personality candidates before transcript purge,
  recheck ownership/protection/fingerprints and fence late writes. Preserve manual
  corrections, shared evidence and numeric development. Keep additive counts and
  unknown legacy state accurate. Explicit profile deletion fences old analyses.

### Session ingress turns

- `BeginSessionTurn` registers an ingress run before waiting on the store/session
  gate. Hold that shared gate from history construction through durable
  publication and delivery across Web, Desktop, AgoDesk, realtime speech,
  Telegram, Discord, Rocket.Chat and agent loopback. Stop cancels active and
  queued registrations without fencing future turns or deleting data.
- Keep acquisition at ingress; `ExecuteAgentLoop` must not acquire the same gate
  again. Combined release is idempotent. Independent stores and session IDs have
  independent gates. Verify `TestBeginSessionTurnRegistersQueuedRunBeforeWaitingAndStopCancelsIt`
  and `TestBeginSessionTurnKeepsHistoryAssemblyAfterPriorPublication`.
- Pending question replies complete the already running turn before acquiring
  its gate. Telegram registers cancellation before media processing, then uses
  `LockSessionTurn` after routing any question reply.


### Conflict and graph integrity

- Automatic conflict confirmation preserves unrelated archival and replacement state. Graph cleanup aborts the complete SQL transaction after any delete failure, then updates semantic indexes only for committed deletes and reports index errors.
- Node merges retain both claim histories and prefer accepted active edges over retracted collisions. Canonical repair preserves the analysis envelope and provenance independently of graph identity. Batch large identifier lists; propagate identity-query failures before destructive writes.
- OptimizeGraph keeps its explicit low-confidence/degree/protection policy; no implicit age threshold. Pending-write dedupe retains exhausted state and retry scheduling; enqueue does not reactivate work.
- Verify `TestGraphCleanupRollsBackEarlierDeletesOnSQLFailure`, `TestGraphMergeKeepsAcceptedSourceAndBothClaimHistories`, `TestCanonicalRepairPreservesAnalysisEnvelope`, conflict and pending-write tests. Existing production data needs separately reviewed offline repair.

### Graph Search Patterns

- Every knowledge-graph `LIKE` parameter is escaped with `dbutil.EscapeLike` or the equivalent package-local `escapeLike`, and its predicate carries `ESCAPE '\'`, so `%`, `_` and `\` in a query match literally. Lexical search ORs independently escaped terms and ranks relevance; context type exclusions precede FTS/LIKE/important-node limits. Semantic search keeps the full query and backfills past excluded nodes/edges before applying its result limit. Verify `TestKGSearchForContextLikeFallbackTreatsWildcardsLiterally`, `TestKGSearchLikeTreatsBackslashLiterally`, `TestKGContextExclusionsPrecedeLexicalLimit` and `TestKGSemanticExpandsPastExcludedCandidatesAndUsesFullQuery`.

### Model remember provenance

- New model `remember` writes to Core Memory, Notes and Journal commit actor `agent`,
  origin `model_remember` and a content digest in the same transaction as the record.
  A hash identifies the private source session. Reusing an existing fact preserves its
  provenance; edits invalidate old labels. Legacy provenance stays unknown.
- Retrieval shows concise trust labels and excludes raw source session IDs from
  provenance presentation. Known analysis envelopes retain the complete fact while
  hiding the private source session; arbitrary factual paragraphs are preserved.
  Verify `TestModelRememberProvenanceAtomicAndDoesNotRelabelExistingFact`.
- `ValidateCoreMemoryFact` refuses a standing order that reaches for a tool, a secret
  or an outbound channel even without a listed marker: `isCoreMemoryInstructionLike`
  also asks `security.DirectiveShape`. That covers `remember` (`AddModelCoreFact`),
  `manage_memory` add and update, correction previews, learned-rule text, and the
  prompt-side filter that hides such an existing entry. Ordinary standing preferences
  ("Always answer in German") stay allowed. A change to `DirectiveShape` moves this
  gate. Verify `TestCoreMemoryFactPolicyRejectsStandingDirectives`,
  `TestCoreMemoryFactPolicyKeepsOrdinaryStandingPreferences` and
  `TestAddModelCoreFactRefusesStandingDirective`.

### Memory System
- **Schema lifecycle**: Read `user_version` before schema writes and reject newer
  versions. Version 12 is the one-time legacy bridge; later steps commit their
  changes and version atomically. Back up existing stores before migration,
  fail and close on required initialization errors, and keep bounded crash
  recovery separate from data migrations. Never repair a missing required table
  in a database already claiming the current version by silently recreating it.
  Verify `TestMemorySchema*`. Contract test references, including documented
  wildcard prefixes, are checked by `TestMemoryContractTestReferencesResolve`.
- **Short-Term**: SQLite sliding-window conversation context
- **Archive identity and cleanup**: Preserve a durable digest of complete normalized
  content and domain before archiving; only recognized analysis provenance is
  excluded. Automatic writers check the digest even after vector removal.
  Serialize budget enforcement, protection and restoration, recheck the candidate
  snapshot in the write transaction, and commit the actual previous status and
  cleanup receipt before deleting a vector. Unproven conditional deletion retains
  its receipt. Explicit restoration clears the identity only with available source
  content and a successful transaction. Backfill only available historical content.
  Context APIs bound lock/publication waits; compatibility wrappers use 30 seconds.
  Verify `TestArchive*`; operational limits are in
  `documentation/memory-persistence.md`.
- **Resumable maintenance**: Consolidation checkpoints bind each archive message,
  source digest, part number and extractor/model version. Persist validated parts,
  extract at most one part per message in a round, and publish complete messages
  through bounded idempotent cursor steps. Cancellation preserves progress without
  consuming retries. Delete checkpoints with their archive source and reject late
  writes from deleted sessions. Keep terminal consolidation failures visible in
  the ledger, briefing and operational issue until explicitly recovered. Pending
  counts exclude exhausted writes; remove terminal payloads after 30 days while
  retaining a private dedupe identity. Verify `TestConsolidationCheckpoint*`,
  `TestTerminalConsolidationFailure*` and `TestPendingMemoryCleanup*`.
- **Privacy evidence**: An existing deletion job has an immutable scope; both
  conflicting directions return HTTP 409 with `deletion_scope_conflict`. Scrub
  current and historical learning sources and nested correction snapshots before
  completion. Serialize corrections with deletion, retain committed effects and
  anti-replay guards, and terminally cancel unfinished correction work. Unknown
  external cleanup states require durable reconciliation. Store checks and SQL
  fences reject late source evidence. Shutdown cancellation never consumes a
  business retry. Verify `TestPrivacy*` and
  `TestChatSessionDeleteReturnsTypedScopeConflict`.
- **Corpus publication and diagnostics**: Page canonical sources including pending
  state; read only IDs and metadata for filtered deletion or replacement. Batch
  pending acknowledgements in a transaction after index publication succeeds.
  Candidate groups share archive, file-generation and canonical-snapshot checks;
  lookup failures remain errors. Diagnostics distinguish canonical row counts,
  dimensions, database/WAL bytes and the theoretical float32 payload estimate from
  actual process memory. The durable vector format remains JSON. See
  `documentation/memory-persistence.md`; verify `TestCorpusStatus*`.
- **Long-Term**: `directories.vectordb_dir/documents.sqlite` is the canonical document corpus: collection names, raw IDs, complete content, metadata, vectors, dimensions and provider/model fingerprints. Chromem is an in-memory derived index; its legacy 32-bit disk filenames must never define identity. Commit source documents before publishing index changes and retain durable pending work after failures/cancellation. Open and count sources even when embeddings are disabled.
- **Corpus Migration and Reset**: Under the application lock, back up legacy files before migration, verify copied/decoded source digests, complete counts/content and raw IDs transactionally, retain originals and refuse corrupt/orphan/unverified data. A reset or model change schedules reindex only: preserve source documents, archive/curation, extraction sources, file pointers and KG SQL facts. Unknown/incompatible fingerprints or dimensions never enter a live index. Pending multimodal records expose blocked status until the existing authorized file indexer can re-embed their source. Reproduced unchanged chunks preserve raw IDs and provenance; actual content/chunk drift uses the file-generation protocol. The retained legacy `kg_embeddings` snapshot is never restored into the live KG index; KG SQL is authoritative. Verify `TestDocumentCorpus*`, including SQLite FULL, cancellation/restart, collision and available/missing multimodal source cases.
- **Automatic Memory Snapshots**: Automatic snapshots and the memory portion of manual exports share a per-store `MemorySnapshotCoordinator`. Prepare embeddings before a publication lease; pass lease contexts explicitly through nested writes. Hold the lease across corpus, metadata and provenance commits; durable source-free completion markers or existing replacement journals prevent certification after an uncertain partial commit. Lock order is outer session/operation locks, coordinator, then index/SQLite locks. Snapshots reserve all three SQLite writers, pin all three read transactions, release reservations and copy those pinned snapshots with the online backup API. Integrity checks, hashing, ZIP and retention run outside the freeze. Only checksum-verified completed `auto-memory-v2` snapshots with the coordination proof participate in retention: keep 30 days and at least three complete snapshots. Legacy v1, manual, quarantine, migration, unknown and incomplete artifacts stay outside this policy. Restrict snapshot directories/files to their owner. Verify `TestSnapshotWaitsForCorpusMetadataAndProvenancePublication`, `TestBlockedEmbeddingDoesNotHoldSnapshotLease`, `TestPartialAutomaticPublicationPreventsSnapshotCertification`, `TestPinnedReadSnapshotSurvivesConcurrentCommitAcrossBackupSteps`, `TestAutomaticSnapshot*` and `TestManualBackupUsesCoordinatedMemorySnapshotAndRejectsPartialPublication`.
- **Knowledge Graph**: Entity-relationship store for structured facts
- **File Sync Provenance**: Only nodes and edges carrying `source=file_sync` and the matching `source_file` belong to file-sync cleanup. Preserve other provenance even when it names the same file; remove claims for identities that are truly deleted. Review legacy markings on a backed-up copy before any repair and restore already deleted facts only from verified backups.
- **Core Memory**: Permanent facts always included in context
- **Native Chunking**: File, documentation, and tool-guide indexing use the Go `internal/chunking` package. `indexing.chunking` defaults to recursive chunking with 3,500 chars, 200 overlap, and 200 chunks per file; chunking parameters are part of index fingerprints so config changes trigger clean reindexing. Oversize splitting prepares runes once and advances positions without rescanning/reallocating the remaining suffix; preserve boundary/overlap behavior and the 1/8 MiB `BenchmarkRecursiveOversize` cases.
- **Collection Write Receipts**: Collection stores return all allocated IDs together with a write error, including a failed single or multimodal write. Indexer retries stop when a receipt is present so the caller can reconcile the partial attempt before another write. Verify `TestCollectionStorePreservesFailedWriteReceipts` and `TestIndexerRetryPreservesPartialWriteReceipts`.
- **File Replacement**: Changed native file and Markdown source generations use fresh IDs and a versioned, source-free receipt in `memory_maintenance_meta` before vector writes. Publish only complete generations after comparing the full SQLite state and document pointer. Known staged generations require a current active pointer for retrieval; legacy documents remain compatible. Re-read ambiguous commits, retain externally referenced or changed vectors, and retry journaled cleanup before later indexing, including after restart. Legacy vector implementations without ownership evidence retain their receipts. Verify `TestIndexedFileReplacement*`, `TestFileIndexPublish*`, `TestFileIndexRecovery*`, and `TestAuditFailedFileReplacementKeepsPreviousIndex`.
- **On-Demand Context**: Auto-RAG and KG prompt injection keep only essential context in the prompt and expose additional `[memory:<id>]` / `[kg:<id>]` teasers for `recall_memory` and `explore_kg`. Keep complete fact/node entries and reference IDs inside byte budgets; oversized facts become intact recall teasers and dropped entries receive an omission marker. Recency ranking uses the shared timestamp parser and caps future timestamps at current freshness. Verify `TestPromptCompactionKeepsWholeFactsAndReferenceIDs`, `TestFitEntriesKeepsCompleteUnicodeReferencesWithinBudget` and `TestMemoryRecencyUsesSharedTimestampFormats`.
- **Memory Hygiene**: Dashboard and nightly maintenance can safely consolidate exact auto-generated journal error duplicates, archive stale low-priority notes with per-run limits and repeated-failure tracking, repair tracked canonical VectorDB names, and raise review issues for KG/Core Memory health. Automatically extracted claims that a tool, function, API, or endpoint is broken are transient and must not enter long-term memory; explicit user-managed memory remains available. Notes marked `protected` or `keep_forever` are excluded from auto-archive. Review-only/high-risk memory findings must not be auto-deleted.
- **Skill Quality Maintenance**: Nightly maintenance reviews Python skills and `SKILL.md` packages only when persisted provenance is exactly `agent`. Unknown disk discoveries remain `legacy_unknown`; user/system/curated skills are immutable to this phase. Improvement requires classifier confidence at least 0.95 plus complete staging validation and a clean security result. Deletion requires confidence at least 0.98 plus deterministic objective evidence, permanently removes files/registry/versions, and retains only a source-free maintenance tombstone. Missing usage alone never justifies deletion; read-only, ambiguity, cancellation, credential signals, scan warnings, fixed references, or failed daemon stops always prevent mutation.
- **FTS Migration State**: External-content FTS5 indexes for notes, journal entries, episodic memories, and activity turns use version `1` markers in `memory_schema_meta` (`fts.notes`, `fts.journal_entries`, `fts.episodic_memories`, `fts.activity_turns`). Missing or outdated markers require an FTS5 `rebuild`; write the marker only after a successful rebuild.
- **Maintenance Safety**: Follow `documentation/maintenance.md`. Vector writes distinguish created, reused and unknown ownership; rollback never deletes reused/unknown IDs. Compression and canonical repair share force-new replacement with complete metadata comparison and transactional archive/copy. Preserve replacements after commit or uncertain commit. Canonical scans are capped at 100 rows with a persistent keyset cursor in `memory_maintenance_meta`.
- **Fact Storage and Curation**: Similarity nominates a duplicate candidate; reuse requires identical complete factual content and domain after trimming outer whitespace and normalizing line endings. Only proven analysis envelopes exclude their session provenance from identity; kind and category remain significant. New analysis bodies use `source:memory_analysis`; the additive `memory_extraction_sources` table retains session observations independently of curation, with a consistent backup before migration of populated disk stores. Chunked candidates are never whole-document duplicates. All automatic writers insert metadata without overwriting existing rows and preserve reused rows. Rollback requires a created vector, a self-inserted unchanged metadata snapshot, unchanged sources/content and no subsequent references or curation; unavailable evidence retains the document. Automatic storage and rollback serialize within the store so an in-flight reuse cannot lose its vector before recording observations. Replacement stages also retain externally tracked IDs and copy extraction sources transactionally. Either archived status or a nonempty `archived_at` excludes a memory from automatic curation and retrieval; explicit curation clears archive fields when reactivating. Verify `TestAnalysis*`, `TestAutomaticMemory*` and `TestMemoryAnalysisCreatedIDPreservesInterveningCuration`.
- **Retrieval Safety**: Exclude archived status or nonempty archive timestamps before similarity candidates consume top-K or nominate deduplication; read current canonical/source metadata and file generation pointers. Read current metadata for the bounded candidate IDs, never a cached corpus prefix. A missing legacy row is allowed only after a successful database lookup; unavailable stores, missing IDs and lookup failures exclude the affected memory enrichment and remain errors for explicit queries. Conflict callers explicitly declare raw facts, stored documents or search results. Remove only the declared format's known envelopes; preserve every raw paragraph and unknown bracket text. Immediate, queue and nightly checks share normalization and the caller's cancellation context. Verify `TestConflictFormatsPreserveMultilineFact` and `TestConflictSignalsUnderstandStoredFactEnvelopes`.
- **Maintenance Lifecycle**: Skill cancellation and mutation state must preserve recoverable files and safe daemon restoration; source drift cannot inherit security approval. Each phase has one stable issue fingerprint and resolves only after a persisted success. The server-owned controller follows immutable config snapshots, local calendar/DST scheduling, a durable started-day claim, and shutdown before database close. Summaries cover at most three missing journal dates from the last seven completed days, insert-only; current-day summaries are excluded.
- **Consolidation Progress**: Run STM consolidation after baseline cleanup and before summaries/reflection/KG work, with a two-minute phase budget and the existing protected tail reserve and message cap. Continue claiming bounded batches while time and the message cap remain; do not reserve a fresh request's full timeout before each claim. Phase cancellation releases claims without consuming retries. An explicit valid empty facts array completes extraction without new facts; missing/null arrays and malformed facts remain errors in both direct and helper responses. Direct summary and consolidation completions honor the selected helper/main provider limits and reserve reasoning output. Verify `TestConsolidationUsesRemainingPhaseBudget`, `TestConsolidationRequiresExplicitValidFacts`, `TestMaintenanceConsolidationMakesProgressBeforeSlowPhases`, and `TestMaintenanceCompletionsReserveReasoningWithinProviderLimits`.
- **Archive Origin**: Archive `messages.is_internal` with each turn and exclude internal turns from consolidation at claim, count, and finalization. A populated legacy archive receives a consistent backup before the additive nullable origin column; NULL means unknown, never assume it was user-visible. Exclude only recognizable legacy cron triggers, native-call errors, raw assistant tool calls, and empty assistant placeholders; retain other legacy conversation turns. Preserve explicitly user-visible quoted content even when it resembles an internal marker. Verify `TestArchivedMessagesPreserveInternalOrigin`, `TestLegacyInternalArchiveRowsAreExcludedWithoutSuppressingUserContent`, and `TestNewSQLiteMemoryNormalizesInvalidConsolidationStatuses`.
- **Typed Conflict Review**: Automatic contradictions require the same typed single-choice property: recognized language/theme/editor preferences, explicit primary location/platform or a named endpoint default port. Generic usage, spoken languages, locations and host listening ports can coexist and never establish exclusivity. Administrative `action=dismiss` on the existing conflict-resolution API retains both facts and unrelated open conflicts; unchanged reviewed pair values remain dismissed, changed values can reopen. Verify `TestConflictSignalsOnlyCompareTypedSingletonFacts`, `TestDismissMemoryConflictKeepsFactsAndReviewedEvidence`, `TestHandleMemoryConflictDismissKeepsBothFacts` and `TestMemoryConflictDismissBrowser`.
- **Conflict Scan Progress**: Retained archived metadata does not consume the 250-active-document budget. Read keyset pages of 500 with at most 50,000 metadata rows per run; persist `memory_conflict_scan.cursor` in `memory_maintenance_meta` and reset only at the actual end. Recheck current archive state before reading vectors. Individual failures remain errors, are recorded and retried on the next pass; cancellation never advances past an incomplete check. Log the failing operation under `memory_conflict_scan`; the public ledger retains sanitized codes. Verify `TestMemoryConflictScan*`, `TestMaintenanceConflictScanSkipsArchivedBeforeLimit`, `TestMaintenanceBaselineSkipsMemoryArchivedDuringCuration`, `TestMemoryConflictScanHonorsCurrentArchiveStateAndErrors`, and `TestMaintenanceBaselineLogsConflictFailure`.
- **Archive Retention**: Apply the configured archive lifetime only to consolidated terminal states `done` and `excluded`. Retain pending, processing and failed work. Verify `TestExcludedArchiveRowsExpireAndIncompleteRowsSurvive` and `TestCleanOldArchivedMessages`.
- **Core Graph Sync**: Existing `AddNode`/`UpdateNode` remain best effort for semantic indexing. Core sync uses context-bound `AddNodeIndexed`/`UpdateNodeIndexed` with the same SQL/protection rules; commit a pending marker before indexing and complete only after successful indexing and an unchanged-node comparison. Background reindexing also rejects obsolete snapshots and checks complete node state before marking completion, including changes within one timestamp. Refresh content and labels (at most 50 Unicode characters), preserving protection. Index errors or cancellation retain pending work and prevent stale cleanup. A disabled index and unrelated pending nodes remain valid; disabled strict writes retain the marker for a later index enable. Retain protected and foreign stale nodes. Verify `TestCoreMemorySync*`, `TestKGReindex*`, `TestKGStrictWriteWithoutIndex*` and `TestSyncCoreMemoryToKnowledgeGraphRemovesDeletedFacts`.
- **Operator Repair**: `scripts/repair_memory_metadata.py` defaults to a read-only preview in ignored `reports/`. `--apply` requires its saved plan, a consistent backup and a successful rehearsal on a copy; full metadata and decision/conflict evidence are rechecked before transactional writes and audit events. Version-2 repair events prove before/after state, evidence and remaining work; a restored confirmation still exposes missing provenance/confidence for later backup-based repair. Prior own repairs require a verified status/evidence chain; later human decisions or conflicts block continuation. Restore source/confidence only from uniquely matching pre-damage backups; unresolved evidence stays review-required. `cmd/memory-repair/AGENTS.md` owns offline exact analysis merging under the actual application lock, full backups/rehearsal, transactional reference transfer, archived tombstones and a durable conditional vector-deletion journal. Never reconstruct lost facts or run automatically on production data. See `documentation/maintenance.md`; verify the Go operator tests and `python -B -m unittest discover -s scripts -p test_repair_memory_metadata.py`.
- **Chat History File**: `HistoryManager` treats only a missing file as empty. A read error leaves the file untouched and disables saving for the process (`LoadError`, `PersistenceBlocked`); an unparseable or zero-length file is moved to `<file>.corrupt-<UTC time>-<suffix>` with `fileutil.Rename` before a fresh history starts. Saves use `config.WriteFileAtomic` (synced temp file). Startup reports the load error as the `chat_history_load` warning. Verify `TestHistoryManagerQuarantines*` and `TestHistoryManagerReadErrorNeverOverwritesFile`.
- **Vector Write Lock**: Provider embedding calls never run under `cv.mu`. Memory, collection and cheatsheet stores embed with `embedDocumentsOutsideLock` (serialized by `storeEmbedMu`, which searches never take) while holding only their concept stripe, then take the write lock for the chromem mutation; dedupe probes compute the query embedding before `RLock`. A failed embedding writes nothing and still returns every allocated ID; a cheatsheet keeps its previous version. Verify `TestVectorStoresDoNotHoldWriteLockWhileEmbedding` and `TestStoreCheatsheetKeepsPreviousVersionWhenEmbeddingFails`.
- **Removed-File Cleanup**: `IndexDirectory` deletes a removed file's tracking row only after every vector delete succeeded (`deleteRemovedMarkdownVectors`, by tracked doc ID or by source name); a failed delete keeps the row, logs it and returns a `retain tracking` error so the next run retries. AgoDesk knowledge rollback (`rollbackIndexedDocument`) likewise removes `file_index` tracking and metadata only once all embeddings are gone, and keeps them when the vector store is missing. Verify `TestIndexDirectoryRetainsTrackingWhenStaleVectorDeleteFails` and `TestAgodeskKnowledgeRollbackKeepsTrackingUntilVectorsAreDeleted`.
- **Core Memory Normalization**: Core-memory identity is case- and whitespace-insensitive (`normalizeCoreMemoryFactForDedupe`). The startup check runs until the `core_memory.normalized_fact` marker in `memory_schema_meta` holds the current version; before deleting any duplicate it writes a `VACUUM INTO` backup via `backupMainDatabase` (`<db>.core-memory-normalized-v<N>-*.bak`), logs kept and removed IDs without content, and updates only changed rows. Bump the marker version whenever the normalization changes. Verify `TestCoreMemoryDedupeBacksUpOnceAndWritesMarker` and `TestCoreMemoryNormalizationWithoutDuplicatesCreatesNoBackup`.
- **Schema Migration Errors**: `migrateFileIndexToCollectionAware` returns schema-inspection and count errors so `applySQLiteMemoryMigrations` does not bump `user_version` over an unknown state. KG `initTables` returns column-add errors and `fts_schema_version` read/write errors; the `updated_at` backfill and performance indexes stay warnings, and a failed `idx_kg_conflicts_pair_open` is logged at error level for review. Verify `TestMigrateFileIndexReportsSchemaInspectionError` and `TestKGInitReportsFTSMarkerWriteFailure`.
- **Plan Dependencies**: Persist a non-null JSON array of task IDs (`[]` means no dependencies); reject null, other JSON shapes, malformed/empty values, missing IDs and cycles. Promotion visibly blocks invalid legacy tasks and their plan with `planTaskDependencyBlocker` (same shape as `SetPlanTaskBlocker`, resumable with `ClearPlanTaskBlocker`); splitting refuses unreadable dependency data. Validate new plans before publishing any rows. Verify `TestCreatePlanRejectsInvalidDependencyGraphs`, `TestLegacyInvalidDependencyGraphsBecomeVisibleBlockers`, `TestLegacyEmptyDependencyArrayRemainsRunnable`, `TestPromoteNextPlanTaskBlocksUnreadableDependencies` and `TestSplitPlanTaskRejectsUnreadableDependencies`.
- **Multimodal Embedding Origin**: `NewMultimodalEmbedder` uses `httporigin.NewClient`, so the key and file content never follow a redirect off the configured origin; `shouldRetryEmbeddingError` treats `httporigin.ErrCrossOriginRedirect` as final and never retries it. Verify `TestMultimodalEmbedderDoesNotFollowCrossOriginRedirect` and `TestEmbeddingRetryTreatsRejectedRedirectAsFinal`.

### Learned rules

- `UpsertLearnedRule` truncates pattern and rule to 500 runes, then rejects the insert when the rule text matches the core-memory instruction markers, or when the rule text contains its own pattern as a contiguous substring of at least 48 runes. The error is returned; callers log it and skip. Empty rules stay a no-op. Short operator preferences and the local shell-timeout template remain insertable. Journal, retrieval and prompt isolation stay. Verify `TestUpsertLearnedRuleRejectsInstructionTextAndCopiedSource`.

### Bounded Memory Writes
- Small memory writes embed the complete document once and reuse that vector for identity checks and storage across owned, domain and batch paths. Identity probes do not apply retrieval age decay; reuse still requires complete normalized content/domain equality, with only the proven analysis-provenance exception. Consolidation passes its context through automatic storage to provider embeddings, including chunks, and releases interrupted claims without spending retries. Log extraction and storage timings without source content. Verify `TestMemoryWritesEmbedEachFactOnce`, `TestMemoryDedupDoesNotAgeIdenticalFacts`, `TestAutomaticMemoryWriteStopsEmbeddingAtDeadline`, and `TestConsolidationStorageDeadlineReleasesClaims` alongside existing ownership/curation tests.

### Automatic Curation
- Use `ApplyAutomaticMemoryCurationAction` with its internal complete snapshot. The shared write transaction rechecks protection, lifetime/archive state and the latest non-preview curation event ID; actor strings never choose the permission path. The legacy `ApplyMemoryCurationAction` is for explicit administrative actions. Count only applied writes, excluding skipped stale/missing candidates. Verify `TestAutomaticCuration*` across maintenance, orchestrator and Dashboard callers.

### Identifier Recall
- Native and local-agent ID recall share current metadata checks. Missing legacy metadata is allowed only after a successful lookup; archived entries are omitted and unavailable metadata is an explicit error. Preserve bounded, same-session conversation recall and scrubbing. Verify `TestRecallMemoryHonorsCurrentArchiveAndMetadataFailures` and `TestConversationRecall*`.

### Partial Search Results
- Collection search returns healthy results together with named errors and cancellation. Wrappers preserve both; automatic consumers use only currently metadata-checked results and explicit queries expose incomplete search errors. Empty healthy collections remain successful. Verify `TestMultiCollectionSearchPreservesResultsAndErrors`, `TestAuditMultiCollectionSearchReportsFailedCollection` and `TestMemorySearchConsumersKeepCheckedPartialResults`. Knowledge-graph keyword search follows the same rule: `SearchResultWithOptions` returns healthy FTS, LIKE and edge results together with named errors, `SearchWithOptions` answers a total failure with a JSON `error` and a partial one with an `errors` field, and the explicit HTTP consumers (`/api/knowledge-graph/search`, `/api/people/lookup`) answer 503 when nothing could be searched. Verify `TestKGSearchReportsFTSFailureAndKeepsLikeResults`, `TestKGSearchReportsDatabaseFailureInsteadOfEmptyResult` and `TestHandleKnowledgeGraphSearchReportsDatabaseFailure`.

### Deferred Extraction Writes
- Persist the original `MemoryMetaUpdate` in version-1 nullable queue JSON. NULL retains the legacy fallback; malformed or unsupported payloads consume a failed attempt without a vector write. Dedupe preserves the first complete payload and all retry scheduling, enriching NULL once. Back up populated disk stores consistently before the additive column migration. Automatic retries continue to preserve existing curation. Verify `TestPendingMemoryMetadata*` and `TestAuditQueuedAnalysisPreservesExtractionConfidence`.

### Realtime Extraction
- Direct extraction, helper fallback and local-agent turns share validated completion state and provider-aware JSON output budgets, including reasoning reserves and explicit provider caps. Request JSON mode only for confirmed capabilities. Require complete non-null result arrays and valid items before persistence; empty arrays are a successful no-op. Allow one smaller, human-record-only retry for unusable responses; cancellation or budget failure prevents further requests and writes. Source text is isolated as external data. Verify `TestAuditMemoryAnalysisRejectsTruncatedCompletion` and `TestRealtimeMemoryAnalysis*`.

### Contact Graph Sync
- `SyncContact` transactionally reconciles the five contact-owned fields and relationship edges after current protection checks. Preserve foreign properties; new edges carry `contact_sync_id` and `contacts_sync` claim provenance. Clear or replace only proven own relationships without foreign claims or protected nodes/edges; retain ambiguous legacy edges and report review-required through maintenance. Generic node merge semantics stay unchanged. Deleted contacts do not authorize graph deletion. Verify `TestContactSync*`, `TestSyncContactsToKnowledgeGraphPrunesStaleBelongsToEdges` and `TestAuditContactsSyncClearsRemovedRelationshipAndFields`.

## Verification

- Run `go test ./internal/memory` and the named cross-component checks in the contracts above when those paths change.

### Privacy deletion and note protection

- `CancelSessionRuns` stops active and queued work without marking a session
  deleted or blocking later human turns. `LockSessionTurn` serializes complete
  ingress-to-publication turns per store/session, honors cancellation while
  waiting and releases unused gate entries. Register cancellation before
  acquiring this turn gate. These runtime gates are separate from durable
  privacy deletion fences and never replace them. Verify
  `TestSessionStopCancelsRunsWithoutDeletionFence` and
  `TestSessionTurnLockSerializesAndHonorsCancellation`.

- Explicit chat deletion physically removes transcripts, archives and its plans/tasks/events. Rotation remains archival. Source-free deletion fences reject delayed writers across restart; fresh human input alone advances fixed-channel generations. New `default` generations remain discoverable through chat history; internal channels never gain visibility from generation advancement.
- Contact deletion retains derived knowledge by default, including native `address_book` deletion. Only the administrative `delete_memories=true` option may remove exclusively proven contact provenance. Protected, shared, legacy and ambiguous graph records remain and are reported; `contact_sync_id` never authorizes deleting arbitrary merged properties. A source-only contact fence prevents later sync from restoring a deleted contact.
- Chat transcript deletion clears session-bound KG evidence text and identifying source references while retaining derived claims when optional memory deletion is off. Partial deletion responses report actual outcomes rather than implying rollback across stores.
- Protected notes require an explicit administrative unlock before modification or deletion. Automatic curation and retention recheck protection/keep-forever flags atomically with their mutation; previews alone never authorize writes.
- Verify `TestContactDeleteKeepsKnowledgeUnlessExplicitlySelected`, `TestNativeContactDeleteRetainsDerivedKnowledge`, `TestChatDeleteScrubsRawGraphTranscriptWithoutRemovingKnowledge`, `TestFreshUserIngressAdvancesDeletedFixedSessionAcrossRestart`, `TestPrivacy*`, `TestNoteProtectionRecheckedDuringCurationAndRetention` and `TestMemoryPrivacyControlsBrowser`.

### Durable privacy deletion jobs

- Persist the exact content-free target manifest and expected semantic fingerprints before scrubbing transcript or source evidence. If candidate discovery or job persistence fails, leave the transcript intact and report the preparation failure.
- Apply only journaled target IDs. Recheck the saved fingerprint, exclusive provenance, sharing, protection and curation state in the SQLite write transaction immediately before external vector or graph mutation; changed or uncertain targets become `retained_review`. Never expand a prepared job by re-reading post-scrub provenance.
- Keep per-target progress and header counts transactionally consistent. Automatic chat-memory retries use persisted backoff after restart and a manual retry path; the worker runs independently of `maintenance.enabled`. Clear target IDs, session hashes and contact operation tokens when jobs become terminal; retain only content-free status/counts for 30 days.
- Contact operations serialize by contact ID and use operation-token-owned fences. On a confirmed failed contact DELETE, compensate only that operation's fence; if the outcome is uncertain, expose `reconcile_pending` and reconcile the current contact row without deleting it automatically. Optional memory deletion is authorized only after the contact DELETE is confirmed. Do not retry contact deletion automatically.
- Public job status contains IDs/status/counts only, never a contact name, transcript, source text, target manifest or session identity. Verify `TestPrivacy*`, `TestChatSessionDeleteStatusUsesRequestedJobWhenAnotherJobFails` and contact fence reconciliation tests.

## Child DOX Index

None.
# Structured insights

- `learnings.go` adds opt-in, scoped insights in the existing SQLite database.
  Existing databases get a consistent backup before the additive transaction;
  no legacy memory or historical conversations are imported.
- Source identity and scope are runtime-owned. Repeated preferences need two
  independent high-confidence human sessions. Protected/archived/superseded
  entries cannot be automatically resurrected. Revisions retain provenance.
- Corrections use the existing preview/apply/status flow with `store: learning`.
  Bind lookup and status to the caller's scope, including resource restrictions.
- Fingerprint changes trigger revalidation, not automatic invalidation. See
  `documentation/learnings.md` for context budgets, review and write checks.

- Personality mood and combined mood/emotion requests declare finite output caps. Runtime callers apply provider-aware JSON limits and budget accounting before parsing; truncated responses cannot update state. EmotionSynthesizer owns shared state, cooldown, validation and persistence only; LLM generation belongs to the budgeted V2 agent analysis.

- Durable credential detection always rejects registered values and concrete credential/key material, while ordinary JSON/Vault/S3 key names, token booleans and explicit template/environment references remain data. Only internally indexed guide chunks proven byte-identical to compiled sources may retain reviewed credential demonstrations; registered secrets and modified/user-supplied guides receive no exception.
