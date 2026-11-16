# Pending system restores

## Purpose

Stage, validate and publish offline system restores with rollback and operator-visible results.

## Ownership

This package owns manifests, mapped destinations, rollback journals and staged Vault handoff. `cmd/aurago` owns process and data-directory locks; server handlers stage restores while the running process holds those locks.

## Local Contracts

- Production callers acquire `aurago-data.lock` for active SQLite directories, the restore state directory and every validated restore destination before `Apply`, including cleanup. Keep locks for the process lifetime.
- `AffectedDataDirectories` is read-only and includes sealed rollback-journal destinations after interrupted publication. Invalid input returns the restore state directory and an error; normal rejection may run only after that directory is locked.
- Resolve mapped destinations from the active configuration. After configuration publication, acquire additional configured SQLite directory locks before opening databases. Use `config.LoadReadOnly` for preflight/validation so invalid YAML cannot write diagnostic copies before locking.
- Lock contention preserves pending files and DB/WAL/SHM bytes. Restore failures keep the existing rollback/result semantics. Never delete the original recovery copies to work around a lock failure.

## Work Guidance

Keep staging/validation separate from publication. Operator recovery guidance: `documentation/restore-locks.md`.

## Verification

Run `go test -p 1 ./internal/restorepending` and `go test -p 1 ./cmd/aurago -run 'TestStartupRestore|TestDataDirectoryLocks|TestRestoreFlowsPurgeBackup'`. Test locks on Windows and Linux, including directory aliases and shared data across installations.

## Child DOX Index

None.
