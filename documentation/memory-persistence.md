# Memory persistence and maintenance

AuraGo keeps durable documents in `documents.sqlite`. JSON vectors remain the
on-disk format. Chromem holds a derived index in RAM; starting the index loads
the corpus in pages and makes it available only after the load succeeds.

The dashboard's memory health details and embedding status report the corpus
document count, stored vector dimensions, SQLite file size and WAL size.
The vector payload estimate is `4 × sum(dimensions)` bytes for stored vectors,
excluding the retained legacy KG snapshot. This estimates float32 payload if
those vectors are materialized. It does **not** measure process RSS or include
strings, metadata maps, index structures, copies, allocator overhead or temporary
embedding batches. JSON storage size and total RAM use can differ substantially.
RAM therefore grows with document count and dimensions, plus that overhead.
Changing the JSON representation or adding ANN indexing requires separate
measurements and is not part of this repair.

## Archives and restoration

Archiving saves a private identity derived from the complete normalized content
and domain. The existing analysis format excludes only its recognized session
provenance. Automatic extraction checks this identity even after the vector has
been removed. Archived documents remain excluded from retrieval and ordinary
deduplication.

Existing archives are backfilled from available canonical content. If historical
cleanup already removed that content, AuraGo cannot reconstruct its identity and
does not invent one. This historical gap cannot be repaired from metadata alone.

Budget cleanup commits the archive decision, previous status and cleanup receipt
before deleting vectors. Interrupted cleanup resumes in maintenance even when
the budget is subsequently disabled. Protection and retention decisions are
rechecked in the write transaction. The default budget remains disabled.

An explicit restore clears the archive identity only with successful
reactivation. If cleanup removed the canonical content, restore that source from
a verified backup first. Privacy deletion does not create an archive identity.

## Migrations, deletion and exhausted work

Startup rejects newer database schema versions before schema changes. Existing
stores receive a protected, consistent backup before migration. The historical
idempotent initializer runs once as a compatibility bridge; later versions
commit each step and its version together. Required initialization failures stop
startup. Recurring crash recovery is separate from data migrations.

A session deletion job has an immutable scope. Retrying with the same scope
reuses the job; changing whether derived memories are included returns HTTP 409
with `deletion_scope_conflict`. The chat dialog explains the conflict.
Deletion scrubs learning evidence in current and historical revisions and
cancels unfinished correction replay without undoing committed corrections.
Store checks and SQL fences reject late session evidence.

Consolidation checkpoints bind to each archived message, its source fingerprint,
part number and extractor/model version. Validated parts survive cancellation;
publication has its own durable cursor. Terminal consolidation failures remain
visible until resolved. Pending write counts include only open work.
Exhausted write payloads remain for 30 days after exhaustion, then are replaced
by private identity markers that prevent automatic re-enqueue.

All these changes are local software behavior. Deploying the binary and repairing
production data are separate operations.

## Safe publication and expiry

New automatic memories are prepared before they become searchable. A durable journal
binds new corpus IDs to content checksums; metadata, provenance, document identity and
expiry commit together before the prepared embeddings enter the search index. Failed
metadata or expiry writes are reported as failures, never as permanent saved facts.
Reusing an existing document preserves its manual curation. A fresh human statement
without an expiry removes the old deadline only after the complete transaction succeeds.

On restart and in the existing background retry worker, AuraGo removes provably owned
unfinished preparations or finishes confirmed publications using compatible stored
embeddings. The worker runs immediately and every five minutes, independently of nightly
maintenance, so a transient failure does not require another restart. Current permissions,
deletion fences, shutdown cancellation and a thirty-second recovery limit still apply.
Uncertain or incompatible publications remain hidden and prevent a complete snapshot;
their receipts are preserved for diagnosis. A matching new statement is deferred before
embedding instead of creating another copy; unrelated queued facts can proceed. Revoking
write permission during a retry preserves its queue entry and remaining attempts.
No automatic repair of unrelated or legacy documents is attempted.

Search and direct recall share current archive, expiry and publication checks. Expired
facts are unavailable immediately, before nightly archival, and failed recall does not
increase access statistics. Protected and keep-forever records retain their existing
exception. Extending or removing an expiry is serialized with archival, whose transaction
also checks that the observed deadline is still current and due.

## Summary provenance and human evidence

New persistent conversation summaries remain separately searchable, marked as unverified
model derivations with mixed human/assistant origin and the exact source session. They
are screened before storage, obey current Memory permissions and participate in chat
privacy deletion. Deleting a chat **with derived memories** removes eligible new copies;
deleting only the transcript preserves them. Normal history compression continues when
long-term Memory is disabled or read-only.

**Historical limit:** Previously stored orphan summaries without source provenance remain
untouched. Deleting their old chat cannot reliably locate those copies. This change does
not backfill, infer ownership or repair that historical data.

Nightly consolidation now requires a quote from the sanitized human message. Assistant
claims, quoted third-party content and durable tool instructions cannot independently
become facts. Unsupported archive candidates are finalized without repeated extraction.
Unfinished checkpoints from the previous extractor format are re-extracted; completed
historical facts are not changed retroactively.

## Backup failure and retention

Only verified, complete v2 snapshots are eligible for retention deletion. AuraGo keeps
snapshots from the last 30 days and at least three complete snapshots. Old v1, unknown,
incomplete and damaged snapshot directories, including extra user files, are preserved.
Only a failed backup attempt's own newly created directory can be removed by that attempt.
Retention inventories the directory once and revalidates each selected deletion candidate.

When persistence is active and the backup fails or its paths are incomplete, maintenance
records diagnostics and marks later phases skipped. It performs no archival, budget
cleanup, automatic curation, graph synchronization or consolidation catch-up in that run.
The next successful backup allows the normal phases to process outstanding work.
