# SQLite migration backups

Required migration copies are created before schema changes. Failure to copy,
protect, sync or publish the backup stops initialization; resolve the reported
storage problem and start again. No partially prepared copy counts as a backup.

`internal/dbutil.BackupSQLite` copies the main database including committed WAL
data into private staging beside the database, protects the file for its owner
(0600 on Unix; protected owner-only ACL on Windows), syncs it and publishes it
through the existing platform replacement helper. Parent permissions remain as
configured. Private staging is removed after either success or failure.

Each store retains its existing filename prefix. Examples include
`desktop_store.db.before-install-journal.bak`, `<db>.looper-v2.bak`, `<db>.v1.bak`
and `<db>.v<N>.bak`. An existing filename is preserved, including an empty file.
A collision produces `<stem>.<random-suffix><extension>` in the same directory;
find copies by their prefix and check their contents and modification time.
Existing private Memory snapshots, Flow rehearsal copies and `.ago` archives
retain their own layouts and recovery workflows.

To recover, stop AuraGo and keep a separate copy of the current database and its
WAL/SHM companions. Verify the chosen migration copy using SQLite
`PRAGMA integrity_check` and inspect its schema/version. Restore the consistent
copy with the matching AuraGo version; do not pair it with a newer database's
WAL/SHM files. Preserve both the current state and the migration copy until
recovery is verified. Never restore over a running database.
