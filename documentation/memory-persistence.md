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
