# Offline memory repair

## Purpose

Merge proven duplicate analysis vectors without losing curation or references.

## Ownership

This command owns offline previews, backups, rehearsals and resumable merges.
`internal/memory/AGENTS.md` owns shared fact identity and SQLite schema contracts.
`documentation/maintenance.md` owns operator instructions and acceptance boundaries.

## Local Contracts

- Reuse existing SQLite, Chromem, flock and fact-identity implementations. Never
  request embeddings, initialize stores, migrate schemas or access the network.
- Preview is the default. `--apply` requires the saved plan and actual application
  lock under `--install-dir`; both modes require a stopped agent.
- Keep plans, backups, manifests, rehearsal copies and results exclusively under
  ignored installation `reports/`. Reject symlinked artifacts and vector files.
- Select only complete analysis identities and domains. Missing or conflicting
  evidence, protected-ID conflicts and unknown references stay review-required.
  Preserve canonical provenance, confidence, archive and human curation.
- Back up consistent SQLite and the whole vector tree, then rehearse the complete
  plan and its repeat before mutation. Recheck full snapshots before every group.
- Transfer references and counters transactionally; retain archived old-ID
  tombstones until conditional on-disk vector deletion is confirmed. Persist
  progress in `memory_maintenance_meta`; resume only with the original plan.
  Changed evidence stops the group, repeated application never double-counts.

## Work Guidance

- Keep the operator command independent of the live agent and HTTP endpoints.
- Never use runtime data in tests; use temporary SQLite/Chromem fixtures with
  controlled embeddings. Preserve unknown cases rather than guessing references.

## Verification

- Run `go test ./cmd/memory-repair` and build the command.
- `TestMerge*` checks reference transfer, evidence, protection, archive,
  cancellation, changed snapshots and recovery around SQL/vector commit boundaries.
- `TestOperatorCLIBacksUpRehearsesAndUsesActualLock` checks preview, the actual lock,
  consistent backups, rehearsal and repeated application.
- Run the Python metadata repair tests when their proof format changes.

## Child DOX Index

None.
