# Automatic maintenance contracts

## Memory ownership and replacement

`VectorDB` remains compatible with existing backends. `OwnershipAwareVectorDB`
reports created and reused document IDs; `StoreDocumentWithOwnership` reports
legacy IDs as unknown. A partial write must still report possibly created IDs.
Only created IDs can be rolled back. Reused metadata is immutable to the
consolidation/retry write, and unknown IDs receive insert-only tracking rows.

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
