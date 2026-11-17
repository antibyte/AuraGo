# SQLite utilities

## Purpose

Own shared SQLite opening, migration primitives and consistent private backups.

## Ownership

Callers own database paths, schema versions, migration ordering and recovery.
`BackupSQLite` owns copy preparation and publication; `fileutil` owns platform
permissions and the cancellation/publication gate.

## Local Contracts

- Before changing an existing schema, finish the required backup or abort.
- Stage beside the destination in an owner-only directory. Protect the empty
  file before SQLite writes; VACUUM main includes WAL. Sync before publication.
- Publish the synced file with an atomic, create-only hard link under the context
  publication gate. Require filesystem hard-link support; fail closed otherwise.
- Never remove or overwrite an old backup, including one created concurrently.
  On collisions, insert a random suffix before the extension and return the
  actual path. Clean staging on every exit.
- `BackupSQLiteForMigration` shares the preparation above but publishes
  `<stem>-<SHA-256><extension>`. Reuse only an identical regular private file
  with checked open/path identity and a fresh checksum. Changed data gets a
  different name; damaged or non-private content-address collisions abort.
  Reuse remains cancellation/publication-gated. Mutable rehearsals must use
  the fresh-copy `BackupSQLite` API. No retention or automatic deletion.
- Keep existing private snapshot/archive workflows and their authorization.

## Work Guidance

Reuse this helper for migrations; preserve each owner's filename prefix and
restore instructions. Do not change permissions of existing parent directories.

## Verification

`go test ./internal/dbutil` plus affected store migration tests. Windows CI
checks owner ACLs; Unix checks 0600. Cover WAL, collisions, cancellation, sync
and publication failures without running a migration after backup failure.
`TestBackupSQLiteForMigration*` covers repeat/restart reuse, WAL changes,
corruption, publication races and revocation. Run on Unix and Windows ACLs.

## Child DOX Index

None.
