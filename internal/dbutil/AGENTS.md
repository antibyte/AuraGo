# SQLite utilities

## Purpose

Own shared SQLite opening, migration primitives and consistent private backups.

## Ownership

Callers own database paths, schema versions, migration ordering and recovery.
`BackupSQLite` owns copy preparation and publication; `fileutil` owns platform
permissions and replacement retries.

## Local Contracts

- Before changing an existing schema, finish the required backup or abort.
- Stage beside the destination in an owner-only directory. Protect the empty
  file before SQLite writes; VACUUM main includes WAL. Sync before publication.
- Never remove or overwrite an old backup. On collisions, insert a random suffix
  before the extension and return the actual path. Clean staging on every exit.
- Keep existing private snapshot/archive workflows and their authorization.

## Work Guidance

Reuse this helper for migrations; preserve each owner's filename prefix and
restore instructions. Do not change permissions of existing parent directories.

## Verification

`go test ./internal/dbutil` plus affected store migration tests. Windows CI
checks owner ACLs; Unix checks 0600. Cover WAL, collisions, cancellation, sync
and publication failures without running a migration after backup failure.

## Child DOX Index

None.
