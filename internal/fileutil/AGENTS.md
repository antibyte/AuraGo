# File replacement

## Purpose

Publish prepared files with bounded, context-aware platform replacement retries.

## Ownership

`Rename`, `RenameContext` and `RenameRootContext` own replacement retries. Callers own path authorization,
temporary file creation, permissions, syncing, serialization, cleanup, and publication
acknowledgements. This package does not grant filesystem access.

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

## Work Guidance

- Keep replacement independent of secret handling, configuration, and tool policy.

## Verification

- `go test ./internal/fileutil ./internal/config ./internal/security`
- On Windows, `TestVaultAtomicReplaceHandlesWindowsReaders` verifies transient
  readers, successful retries, cancellation, and temporary file cleanup.
- `go test ./internal/tools -run 'Hashline|FileEditor|WriteFileAtomic|NotesNativeFileGuards|MQTTMissionTriggers|MissionQueue'`

## Child DOX Index

None.
