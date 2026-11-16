# SQLite migration backups

Required migration copies are created before schema changes. Failure to copy,
protect, sync or publish the backup stops initialization; resolve the reported
storage problem and start again. No partially prepared copy counts as a backup.

`internal/dbutil.BackupSQLite` copies the main database including committed WAL
data into private staging beside the database, protects the file for its owner
(0600 on Unix; protected owner-only ACL on Windows), syncs it and publishes it
with an atomic hard link under the existing cancellation/publication gate.
The filesystem must support hard links; unsupported publication aborts the
migration. Parent permissions remain as configured. Private staging is removed
after either success or failure.

Each store retains its existing filename prefix. Examples include
`desktop_store.db.before-install-journal.bak`, `<db>.looper-v2.bak`, `<db>.v1.bak`
and `<db>.v<N>.bak`. An existing filename is preserved, including an empty file
or a backup created concurrently during publication.
A collision produces `<stem>.<random-suffix><extension>` in the same directory;
find copies by their prefix and check their contents and modification time.
Existing private Memory snapshots, Flow rehearsal copies and `.ago` archives
retain their own layouts and recovery workflows.

Flow's `pre-restore.db` and `restored.db` are private offline restore scratch,
not live stores in `.ago` archives. Recover them through the encrypted purge or
pre-restore backup and the Flow restore procedure, never as an independent live
database paired with another install's WAL/SHM files.

To recover, stop AuraGo and keep a separate copy of the current database and its
WAL/SHM companions. Verify the chosen migration copy using SQLite
`PRAGMA integrity_check` and inspect its schema/version. Restore the consistent
copy with the matching AuraGo version; do not pair it with a newer database's
WAL/SHM files. Preserve both the current state and the migration copy until
recovery is verified. Never restore over a running database.

## Legacy graph JSON

AuraGo imports `graph.json` atomically, with a fixed 64-MiB input limit. Invalid
identities, invalid edges, conflicting SQLite records or SQL failures roll back
the import. Startup logs a warning and continues using the existing SQLite graph;
the original JSON stays available for review and a later retry. Other database
initialization errors still stop startup.

After a successful commit the source is archived as `graph.json.migrated`, using
a unique suffix when that name already exists. Existing archives are preserved.
Archival uses create-only hard links; if the filesystem cannot publish the archive,
the committed graph remains usable and the source stays in place. A retry accepts
identical records and refuses conflicting ones; it never overwrites existing data.

## Legacy vector snapshots

Before migrating a legacy GOB corpus, AuraGo scans files individually, estimates
corpus growth and checks space for the snapshot, transaction and 256-MiB reserve.
Each decompressed GOB is limited to 64 MiB. Oversize or invalid sources remain
unchanged and the error names the failed check.

Verified private snapshots live next to the vector directory as
`<directory>.legacy-backup-<fingerprint-prefix>-<unique-suffix>`. Their
`aurago-legacy-manifest.json` records relative names, sizes and SHA-256 checksums.
Retries and restarts recheck and reuse a matching complete snapshot. A damaged
snapshot is retained; a new verified snapshot is created only when needed.
Changed sources receive a different fingerprint. Originals and older snapshots
are never removed automatically.

Import reads documents individually from the verified snapshot and commits only
after verifying all source and snapshot files again. Failure rolls back the whole
transaction. Only staging created by the current attempt is cleaned on failure;
interrupted or unrecognized staging directories remain for operator review.
