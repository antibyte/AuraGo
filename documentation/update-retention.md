# Update artifact retention

Linux source and binary updates automatically retain the current version and
two distinct previous versions. A rollback version consists of its executable,
verified resource set and completed update backup. Shared resource sets are
stored once. Explicitly archived executables and release outputs can retain
additional resource sets; the maintenance preview explains these exceptions.

## Lifecycle and storage

The private `.aurago-update/transactions/txn-*` directories contain backups and
atomic version-1 JSON manifests. Manifests record the canonical installation,
transaction ID/time, binary checksums, resource IDs, backup completion and the
outcome (`pending`, `confirmed`, `rolled_back`, `uncertain`). They contain no
credentials. The actual backup can contain credentials and remains private.

The updater holds a kernel lock across backup, installation, readiness and
collection. Resource installation also shares its existing asset lock with
collection. A new version becomes confirmed only after asset verification,
core readiness and the existing conditional tsnet readiness check. Cleanup
failures do not roll back a healthy update.

The maintenance CLI identifies resource pins from Go build metadata when
available. Binaries built with `-trimpath` omit linker flags from that metadata;
for those, the CLI matches linked IDs against installed resource sets without
executing archived binaries. It stops before deletion if the match is missing
or ambiguous.

Unexpected termination, failed recovery or `--no-restart` leaves an unresolved
transaction. Further updates and collection stop until an administrator has
verified recovery. Temporary download/packaging directories are removed by the
exit handler; leftovers from a hard kill are collected after recovery. Old
rollback generations never get deleted just to make room for an update.
Directories selected for removal are first atomically renamed with an
installation-specific `.aurago-retired-...` suffix. If removal fails halfway,
the next cleanup can retry without relying on a partially deleted manifest.

Source updates package their resource archive into temporary staging, not
`deploy/`. Binary update backups exclude `assets/web`; they refer to the shared
resource sets instead of duplicating the complete version history.

All updater Go commands use `.aurago-update/go-cache`. After the build/update,
the cache is cleared if it exceeds 4 GiB. This is a post-build threshold, not a
hard quota during compilation. Existing user-wide Go caches, module caches and
Docker caches are not changed. A free-space preflight budgets the selected
backup data plus build/staging headroom before the service is stopped.

## Administrative preview and cleanup

Run from the installation directory, using the installed binary:

```sh
./bin/aurago_linux --update-maintenance --root "$PWD" --adopt-legacy
./bin/aurago_linux --update-maintenance --root "$PWD" --adopt-legacy --apply
```

Without `--apply`, the command only inventories and verifies files (apart from
creating lock files). JSON output lists paths, sizes, protection/deletion reasons,
completed deletions and errors. On Linux, freed-byte accounting uses allocated
file blocks, excludes multiply linked files and is conservative; filesystem
metadata and concurrent disk activity can make `df` differ. A nonzero exit code
means cleanup did not finish. Apply requires a ready running version matching
the installed resource pin. The command runs before service/vault initialization
and is an administrator CLI, not an agent tool or HTTP write endpoint.

`--adopt-legacy` includes old update archives and `/tmp/aurago-backup-*` only when
their provenance can be established. Legacy backups require matching ownership,
an installation-bound saved tsnet path, an identifiable binary and verified
resources. The two retained legacy backups are copied into private staging,
verified and atomically published before their old copies are removed. Unknown
backups are preserved; a timestamp alone is never evidence of ownership or
historical service health.

Unregistered resource sets have a 24-hour adoption grace period to protect
fresh manual builds. Explicit release archives carry a persistent
`aurago-web-assets-<id>.release.json` marker; existing release metadata and
retained release binaries also protect their pins. Unknown, damaged, linked or
unreadable objects are kept and reported. Routine cleanup does not target
`backups/`, models, audio, homepage projects, other runtime data or Docker volumes.

After resolving an interrupted update and starting the intended version:

```sh
./bin/aurago_linux --update-maintenance --root "$PWD" \
  --resolve txn-EXAMPLE --outcome confirmed --apply
# Or, after restoring the original binary/data and restarting:
./bin/aurago_linux --update-maintenance --root "$PWD" \
  --resolve txn-EXAMPLE --outcome rolled_back --apply
```

Resolution verifies the selected binary checksum, its resources and readiness;
saved tsnet state also requires tsnet readiness. It does not itself restore user
data. Preview and apply collection afterwards. Incomplete backups can only be
resolved as a verified rollback, never promoted to a confirmed new version.

Cleanup results are persisted as `data/update_cleanup_status.json` without raw
errors or credentials. This sanitized file is readable by the service account
even after an administrator runs the updater as root. The server
imports recurring failures into the existing operational-issue lifecycle;
polling one result does not increment the occurrence count. A subsequent
successful collection resolves the issue. Normal daily notice rules apply.

## Verification and rollout

Regression fixtures cover twelve successive updates, shared resource sets,
rollback/release pins, legacy adoption, readiness failures, pending/corrupt
state, resource integrity, symlinks, changed file identities, lock inheritance,
cache thresholds, shell exit cleanup and binary resource restoration. Linux
tests also run with the race detector. These fixtures do not constitute a live
installation or deletion test on the deployment host.

Install the updated code through the normal deployment process before using
the maintenance CLI on an existing server. The first healthy update performs
legacy adoption and collection; a manual preview provides an inventory first.
Deleting unknown manual backups remains a separate administrative decision.
