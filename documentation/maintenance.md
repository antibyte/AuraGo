# Automatic maintenance contracts

## Memory ownership and replacement

`VectorDB` remains compatible with existing backends. `OwnershipAwareVectorDB`
reports created and reused document IDs; `StoreDocumentWithOwnership` reports
legacy IDs as unknown. A partial write must still report possibly created IDs.
Only proven created IDs can be rolled back. Every automatic writer inserts
metadata with `ON CONFLICT DO NOTHING`, including created IDs; existing curation
survives. Rollback compares self-inserted metadata, content, observations and
references while holding the SQLite writer lock. Automatic storage and rollback
also serialize within the memory store, closing the vector reuse/observation gap.
Unproven ownership or concurrent changes retain the document and return an error.

Compression and canonical-name repair use `ReplaceMemoryDocument`. The caller
captures content and the complete metadata before generating a replacement.
The backend force-creates new IDs, the helper rechecks the original snapshot,
and SQLite compares the expected row while copying metadata and archiving the
original in one transaction. Protection or other snapshot changes abort the
replacement. Old backends cannot perform destructive replacement.

Before metadata commit, rollback affects only new artifacts. After commit, the
replacement survives an old-vector deletion or reference-cleanup failure. An
uncertain commit preserves replacement artifacts for recovery. There is no
cross-store transaction; interruption may leave extra copies, never justify
deleting the replacement. Low-priority optimization archives through the
policy-aware metadata operation and retains the original vector.

Canonical repair scans at most 100 metadata rows using the existing keyset
query. Its cursor survives restart in `memory_maintenance_meta`; individual
document failures are reported and do not prevent scan progress. A completed
scan wraps to the beginning. This additive table also owns scheduler state;
`memory_schema_meta` remains reserved for existing FTS version markers.

Similarity only nominates a duplicate candidate. Reuse requires equal complete
content and domain after outer whitespace and line-ending normalization; chunked
or unreadable candidates remain separate documents. Complete analysis facts also
retain kind and category in their identity, while a proven session envelope does
not affect identity. New analysis bodies use `source:memory_analysis`;
`memory_extraction_sources` stores first/last document/source/session observations.
The additive migration backs up populated disk stores before activating this table.
Retrieval reads current metadata for the candidate IDs. Missing legacy metadata
is allowed after a healthy lookup; database failures skip and log the enrichment
and remain visible in explicit memory queries. Status `archived` or a nonempty
archive timestamp excludes automatic retrieval and curation.

The conflict scan reads pages of 500 metadata rows, capped at 50,000 rows and 250
active documents per run. Archived rows do not consume the active limit. Its
`memory_conflict_scan.cursor` survives restart in `memory_maintenance_meta` and
wraps only at the actual end. Individual failures are recorded and retried on the
next pass; cancellation leaves the incomplete document for the next run. Stored
facts and write checks explicitly declare raw, stored or search-result format.
Only that format's known wrappers are removed; every raw paragraph and unknown
bracket text survives. Queue and immediate checks use the same fact and cancellation
context. Archive retention removes consolidated terminal
`done` and `excluded` rows; pending, processing and failed work survives.

Core-memory synchronization uses context-bound `UpdateNodeIndexed` and
`AddNodeIndexed`; the existing public node writers remain best effort. Strict writes
commit a pending marker and finish only after successful semantic indexing and an
unchanged-node comparison. Background reindexing rejects obsolete snapshots and
compares complete node state before marking work complete, even within one timestamp.
Labels are limited to 50 Unicode characters. Protection survives updates; protected
and foreign stale nodes survive cleanup. Index errors and cancellation leave work
pending and prevent cleanup. Disabled indexing and unrelated pending nodes are allowed;
disabled strict writes retain their marker for indexing after a later enable.

Automatic curation uses `ApplyAutomaticMemoryCurationAction` with the complete
expected metadata and curation-event revision. It acquires the SQLite writer lock
before rechecking them. Protected, permanent, archived or changed rows are skipped;
only applied actions count as changes. Explicit administrator actions remain
available. Native and local-agent recall also read current metadata and exclude
archived documents; reactivation requires an explicit administrator decision.

Native file and Markdown indexing stage a fresh vector generation before publishing
its SQLite pointer. Failed replacements keep the previous index available. The
content-free journal in `memory_maintenance_meta` supports interrupted-write recovery;
only the published generation is visible. Cleanup requires proven ownership and
checks current references. Unknown legacy ownership or failed deletion retains work
for review or retry instead of discarding the active index.

Multi-collection searches return healthy results together with collection-specific
errors. Consumers may use those results after current metadata checks while recording
the failure. A failed search must not become a successful empty result or a cached
empty memory snapshot.

Deferred extraction writes retain their original source, category, confidence and
reliability in a versioned queue payload. Deduplication preserves the first complete
payload and retry backoff. Legacy rows without a payload retain their documented
fallback; invalid payloads fail without writing vectors. The additive queue migration
backs up populated disk stores to a private `.pending-memory-metadata-v1-*.bak` file
beside the database before adding the column. Keep those backups outside Git.

Realtime extraction uses the selected provider's context and output limits and only
requests JSON mode when supported. Truncated or invalid completions cannot persist
partial facts. One bounded retry carries the human source again without the rejected
assistant output. Complete, explicitly empty arrays are a successful no-op.

Contact graph sync clears removed email, phone, mobile, relationship and birthday
fields while preserving unrelated properties. Node changes and owned `belongs_to`
relationships commit together. Ownership requires contact-sync provenance; protected
or foreign relationships and unknown legacy edges remain and require review.
Deleting an entire contact does not authorize automatic deletion of its graph node.

## Targeted metadata repair

`scripts/repair_memory_metadata.py` uses Python 3.10+ and only the standard library.
The default mode opens the existing SQLite database read-only and saves a review
plan under ignored `reports/`. Optional pre-damage SQLite backups can prove source
and confidence values. The script never reconstructs lost fact text or modifies
the VectorDB. Refresh stale graph labels through the repaired core-memory sync.

```sh
python scripts/repair_memory_metadata.py --db data/short_term.db --plan reports/memory-repair-preview.json
# Optional evidence: add --baseline reports/pre-damage-short-term.sqlite (repeatable).
python scripts/repair_memory_metadata.py --db data/short_term.db --plan reports/memory-repair-preview.json --apply
python -B -m unittest discover -s scripts -p test_repair_memory_metadata.py
```

Review the plan before applying it; use a new preview filename for each run.
Apply separately after the reviewed build is installed, with AuraGo stopped.
`--apply` requires the saved plan and creates a consistent SQLite backup plus a
rehearsal copy under `reports/`. It tests the changes and their repeated application
on that copy before writing the original. Between preview and application, full
metadata snapshots, curation events, conflicts and matching backup evidence are
compared again inside a write transaction. Changed or already repaired rows are
skipped. Every applied repair has a version-2 transactional `metadata_repair` curation
event containing before/after values, changes, evidence and remaining work; a failed
audit write rolls back the repairs. Confirmation alone does not hide outstanding
source/confidence corrections: a later preview can continue with a matching backup.
Previous repair events require a verified causal suffix. A version-1 status-only
repair is usable only when its evidence hash and status chain still match; later
human decisions or changed conflict evidence require review.

Retained archive timestamps prove archival, unless a later reactivation decision
conflicts with that evidence. Confirmation requires the last effective recorded
confirmation, a matching review timestamp, and no open or later conflicts. Dry-run
events cannot prove a change; later protection does not erase a confirmation.
Source and confidence restoration requires identical curation history, review and
archive state, protection flags and document identity in an older backup. Conflicting
backups and unclear decisions remain review items. A status repair can leave source
and confidence for review when no matching backup exists.

Plans, backups and rehearsal files may contain private metadata. Keep them under
ignored `reports/` with operator access; never commit them. The utility does not
create or migrate tables. Successful local checks do not establish live repair:
verify the reviewed running build and a later persisted maintenance run separately.

## Offline analysis duplicate merge

`cmd/memory-repair` uses the existing SQLite, Chromem and application-lock dependencies.
It performs no embedding or network requests and does not migrate schemas. Complete
the backed-up source-table migration with the upgraded agent first, then stop the
agent for both preview and application. Set `--install-dir` to the actual directory
used for `aurago.lock`, including a custom `--install-dir` used when starting AuraGo.
An existing regular lock file is required; the command holds that same exclusive
application lock through backup, rehearsal and application.

```sh
go build -o bin/memory-repair ./cmd/memory-repair
bin/memory-repair --db /opt/aurago/data/short_term.db --vector-db /opt/aurago/data/vectordb --install-dir /opt/aurago --plan /opt/aurago/reports/analysis-merge-preview.json
bin/memory-repair --db /opt/aurago/data/short_term.db --vector-db /opt/aurago/data/vectordb --install-dir /opt/aurago --plan /opt/aurago/reports/analysis-merge-preview.json --apply
go test ./cmd/memory-repair
```

The default preview groups only complete, unchunked analysis documents with equal
fact identity and domain. Unknown references, unfinished metadata repair, conflicting
curation or colliding conflict histories remain review items. Archive evidence wins
against automatic unreviewed copies. Protected/permanent IDs survive; multiple such
IDs, or a protected ID that would supersede newer human curation, require review.
Otherwise choose the latest proved human curation, then the lexicographically smallest
ID. Automatic reviews cannot supersede human provenance or confidence.

Application requires the saved plan and creates a consistent SQLite backup, full
vector backup and checksummed manifest under installation `reports/`. It rehearses
every group and repeated application on copies first. Each group then rechecks the
complete vector documents, metadata and references before a SQL transaction transfers
usage counters, activity ranges, logs, curation, episodic references, session
observations, and unambiguous conflict/maintenance references. Original IDs and values
remain in the saved plan and backup. The canonical metadata retains its source,
confidence, protection and effective curation. Missing legacy observation dates are
recorded as observations made during repair; past timestamps are never invented.

After the reference transaction commits, redundant IDs remain archived tombstones.
A durable `memory_analysis_merge.*` journal in `memory_maintenance_meta` drives
conditional, individually verified vector deletions and final tombstone removal.
Deletion also holds the SQLite writer lock and compares unchanged content/evidence.
An interruption or intervening change leaves the group pending and visible. Resume
with the **same saved plan** after reviewing the failure report; committed counters
are never counted again. Retain that plan and its backups until completion. A fresh
preview reports pending journals and cannot replace the original recovery plan.
Completed groups are no-ops on repetition; any incomplete group yields a failure exit
status and a report. Review-only groups remain listed in the saved plan and
application reports.

All artifacts stay under ignored installation `reports/`; symlinked artifacts and
vector trees are rejected. Keep the agent stopped until pending groups have been
reviewed. Local fixture acceptance does not apply this repair to production data.

## Skill mutations

Only persisted agent-origin skills are eligible. Python source is hashed before
the review cooldown and again before execution. Source drift is review-required;
quality review cannot approve changed source for execution. The existing
scan/synchronization path remains responsible for security approval.

Deletion accepts a context and checks cancellation after classification, daemon
stop, lock acquisition, and immediately before the first rename. Once a file
transition begins, it is completed or rolled back consistently. Mutation errors
distinguish not committed, committed, and unknown state. Unknown state retains
recovery files and prohibits automatic daemon restart. A previously running
daemon is restored only when restoration is safe and current permissions still
allow it. Restart, rollback, and registry errors reach the phase ledger.

## Phases and operational issues

The public ledger schema stays compatible. Internally, phase outcomes distinguish
skipped work, processed/deferred counts, and sanitized error codes. Disablement
and not-due checks precede budget and backlog handling. Checked no-work phases
may complete; cancellation, errors, or remaining work are partial. Critical
initialization/persistence failures fail the run. Unfinished phases never imply
success. Skill quality, prompt loading, and the agent loop have distinct phases.

Each phase uses `maintenance|phase|<name>`. Only persisted successful phases
resolve their issue or matching legacy title/reference fingerprints; skipped
phases never resolve failures. Combined helper responses acknowledge summary
and KG persistence independently and retry only the missing part. An absent or
malformed KG field differs from a valid empty extraction.

The run ledger precedes the typed, idempotent `morning_briefing` notification.
The notification source ID stays bound to the run start timestamp. Background
maintenance does not send chat or Telegram notices directly.

STM consolidation runs immediately after baseline cleanup, before summaries,
weekly reflection, and knowledge-graph work. Its two-minute phase budget also
preserves the existing 90-second tail reserve; the configured message cap still
applies. Bounded batches continue while that budget remains, without requiring
another full request timeout before each claim. A phase deadline releases
unfinished claims without spending a retry. Explicit valid empty fact arrays
complete extraction without creating facts or retrying the same conversation;
missing/null arrays and invalid facts remain errors on both LLM paths.
Direct summaries and consolidation reserve reasoning output within the selected
helper/main provider's output and context limits.
For an old conversation backlog, `consolidation.catchup_minutes` can opt in to
another daily consolidation run after the normal maintenance ledger and morning
briefing are stored. The default is `0` (off); values above 60 are capped at
60 minutes. This work uses the same model and `max_batch_messages` cap, stops
when maintenance is disabled or the service shuts down, and releases unfinished
claims without consuming a retry. The next morning briefing reflects the
previous catch-up; its own counts describe only the normal maintenance run.
Archived turns keep their internal-origin flag. Internal turns never enter the
consolidation queue or its reported backlog. On existing stores, an additive
migration backs up populated archives before adding the nullable flag; legacy
rows without origin are excluded only when their content identifies a scheduled
trigger, native-call error, raw assistant tool call, or empty assistant
placeholder. Other legacy rows remain eligible for extraction.

The morning briefing reports processed archive messages and extracted facts
separately from the combined work count, and includes unfinished phases with
their deferred counts and sanitized codes. `phase_budget_exhausted` means that
the phase ran out of time; integration checks can pass while maintenance remains
partial. The operational issue count covers all active issues, including earlier
runs and other sources. A large backlog is drained across bounded runs and is
never cleared just to make the report look successful.

For `memory_baseline`, consult the maintenance log: conflict-scan failures record
`memory_conflict_scan` with the underlying metadata, document-read, search or
conflict-write error. The public ledger never includes raw backend error text.

## Scheduling and completed-day summaries

The server owns a `MaintenanceController` even when maintenance is disabled.
Central configuration publication updates the controller without overwriting
previous snapshots. Each active run has a private immutable start snapshot.
Updates replan future work; disablement cancels active work and clears next run.
Runs never overlap. Shutdown waits for maintenance before closing its stores.

Schedules follow local calendar days, including DST: a missing wall time runs
at the first valid instant after the gap, and an ambiguous wall time uses its
first occurrence. The started local day is persisted before automatic execution,
preventing a second run that day after restart or configuration change. Upgrade
initialization considers the latest existing run. The Dashboard reads the same
controller state; disabled `next_run` is empty.

Summary dates derive from the run's local start date. Only journal-containing
days in the last seven completed days qualify. Each run attempts at most three:
yesterday first, then the oldest missing dates. Maintenance inserts never
replace existing summaries, including concurrent writes. Catch-up dates only
generate summaries; current KG extraction executes once. The activity rollup
targets yesterday. Input bounds and the protected tail time reserve still apply;
unprocessed eligible days remain visible as deferred work.
