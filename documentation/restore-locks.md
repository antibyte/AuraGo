# Restore startup and directory locks

System backups use fixed logical names for SQLite files outside the standard
archive layout. Import looks up these files in the active local configuration,
retains the corresponding locations in the staged config and records the
approved targets. If a target changes between staging and restart, create a new
restore from the backup so AuraGo can validate and lock the new destination.

AuraGo acquires its installation lock and then `aurago-data.lock` in each
configured SQLite directory before applying a pending system restore. It also
locks the restore state directory, actual mapped restore destinations and any
destinations retained in a sealed rollback journal. Canonical directory aliases
share one lock. This prevents separate installations from restoring over the same
open database.

Configuration and restore destinations are resolved without writing files. If a
required lock is occupied, startup stops before changing the pending restore or
database/WAL/SHM contents. Stop the other installation and restart; the pending
restore remains available. Do not delete a lock file while its process is running.

After restoring `config.yaml`, AuraGo locks any additional configured database
directories before opening databases. If that fails, startup stops with the
restored configuration already in place; rollback copies remain available. All
locks acquired so far stay held until process exit. The
`--restore-flows-purge-backup` command uses the same directory-lock implementation.

Normal restore validation and rollback continue to apply. A malformed restored
configuration rolls back without creating a diagnostic `.corrupted.*` copy during
preflight. Ordinary configuration loading retains its existing diagnostic behavior.
