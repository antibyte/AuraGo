# Update artifact upkeep

## Purpose

`internal/upkeep` owns installation-scoped preview, guarded deletion, transaction
recovery and the `--update-maintenance` CLI used by `update.sh`.

## Ownership

Keep the current executable and resource set plus two verified rollback versions.
The updater owns transaction manifests; this package verifies them before collection.

## Local Contracts

- Do not execute archived binaries to identify their web-asset pins. Read Go build
  metadata when available. For `-trimpath` binaries, match linked 64-character
  IDs only against installed resource-set directory names; reject zero or
  multiple matches before any deletion.
- Preview is the default. Applying cleanup requires locks, verified resources,
  resolved transactions and running-version readiness. Keep unknown, damaged or
  unreadable artifacts rather than guessing their provenance.
- Keep cleanup results sanitized in `data/update_cleanup_status.json`; private
  backups may contain credentials.

## Work Guidance

Keep `update.sh`, the maintenance CLI and the release build flags aligned.

## Verification

Run `go test ./internal/upkeep` and `bash -n update.sh` after relevant changes.

## Child DOX Index

None.
