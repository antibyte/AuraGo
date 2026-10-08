# File replacement and free space

## Purpose

Publish prepared files with bounded, context-aware platform replacement retries, and measure free disk space for download preflights.

## Ownership

`Rename`, `RenameContext` and `RenameRootContext` own replacement retries. `FreeDiskBytes` owns the platform free-space probe. Callers own path authorization,
temporary file creation, permissions, syncing, serialization, cleanup, publication
acknowledgements and free-space thresholds. This package does not grant filesystem access.

## Local Contracts

- Use `os.Rename` without removing the destination first. Preserve its errors.
- `WithPublicationGate` lets an owner serialize each final local rename against
  revocation. The callback is bounded, invokes the commit at most once, and must
  not enter another gated operation. It grants no path permission. Network work,
  preparation and Windows retry delays stay outside the gate. `WriteFileContext`
  prepares and syncs a temporary file, then uses this same replacement boundary.
- Retry only Windows permission/sharing violations, at most eight attempts with
  15–105 ms delays. Context cancellation stops waiting and prevents later attempts.
- Vault, Config, native editors, and both MissionV2 state files share this primitive.
- Token storage and Desktop archive publication share the same retry behavior; rooted callers supply an authorized root rename callback, never an unchecked absolute-path fallback.
  Keep their existing filesystem gates, locks, file modes, and error handling.
- `FreeDiskBytes(path)` reports the bytes available to the current user (`statfs`
  `Bavail*Bsize` outside Windows, the caller quota of `GetDiskFreeSpaceEx` on Windows)
  on the filesystem of the nearest existing directory at or above `path`; it never
  creates directories. Local LLM's swappable `availableDiskBytes` and Local
  Wikipedia's `localwiki.Deps.FreeDiskBytes` default to it; tests replace those hooks.

## Work Guidance

- Keep replacement independent of secret handling, configuration, and tool policy.

## Verification

- `go test ./internal/fileutil ./internal/config ./internal/security`
- On Windows, `TestVaultAtomicReplaceHandlesWindowsReaders` verifies transient
  readers, successful retries, cancellation, and temporary file cleanup.
- `go test ./internal/tools -run 'Hashline|FileEditor|WriteFileAtomic|NotesNativeFileGuards|MQTTMissionTriggers|MissionQueue'`
- Free space: `go test ./internal/fileutil -run 'FreeDiskBytes|NearestExistingDir'` and
  `GOOS=darwin GOARCH=arm64 go build ./internal/fileutil`, `GOOS=linux GOARCH=arm go build ./internal/fileutil`.

## Child DOX Index

None.
