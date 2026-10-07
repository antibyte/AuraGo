# Tresor storage

## Purpose

This package owns the dedicated `data/tresor.db` SQLite store for opaque, browser-encrypted Desktop Tresor records and key envelopes.

## Ownership

- `store.go` owns schema creation, immutable record IDs, conditional revisions and persistence of opaque bytes.
- `internal/server/desktop_tresor_handlers.go` owns request limits, browser-admin authentication, Origin, HTTPS and API responses.
- `ui/js/desktop/apps/tresor-crypto.js` owns password derivation, key wrapping and content encryption. The server must never decrypt records or receive passwords.

## Local Contracts

- Store only random IDs, revisions, ciphertext and password/recovery key envelopes. Entry names, types, MIME types and content stay encrypted as metadata or body.
- Keep the single-header setup atomic and reject stale record/header changes. Preserve existing data on schema changes; back up before migration.
- Browser notes autosave encrypted records. Desktop close awaits writes and reports failures; explicit/idle/pagehide locks clear decrypted state immediately regardless of pending or failed writes. Only completed encrypted drafts survive locking. Late operations cannot change a newer unlocked session. Note selection changes only after successful loading/decryption; draft titles survive redraws. Reload the header before unlock/password change and clear keys/plaintext after an unlock failure.
- `Open` creates or tightens `tresor.db` to 0600 before SQLite opens it, so the database is never left under the umask (even when initialization fails) and its `-wal`/`-shm` sidecars inherit 0600; existing sidecars are tightened too (Unix tests in `store_unix_test.go`).
- Keep `config.TresorDBFilename` in `SQLiteDatabasePaths` and `SQLiteProtectedPaths`, including sidecars.
- Never log or export record bytes, keys or plaintext through agent tools.

## Work Guidance

Keep the database independently restorable and avoid coupling it to the ordinary Desktop workspace or the server-decryptable credential Vault. Update `documentation/tresor.md` when the storage or security boundary changes.

## Verification

Run `go test ./internal/tresor ./internal/config ./internal/server -run 'TestStoreRevisionsAndRestore|TestSQLiteDatabasePathsIncludesConfiguredAndDataDirDatabases|TestTresor|TestHandleBackupCreateIncludesRuntimeFilesAndConsistentSQLiteSnapshots'`.

## Child DOX Index

None.
