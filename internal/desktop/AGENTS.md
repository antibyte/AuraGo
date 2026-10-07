# Desktop services

## Purpose

Desktop file operations, rooted mutations and local presentation assets.

## Ownership

The Service owns authorization, read-only state, mutation locks and cache invalidation; HTTP adapters delegate these operations.

## Local Contracts

- Archive create/extract use rooted operations and the same read-only/Notes gates as ordinary writes. Preflight every ZIP name, mode, size/count, namespace conflict and existing target before writing; reject escaping symlinks, special files, UNC/drive and traversal names.
- Archive listing uses the shared 10,000-entry limit before building response entries. ZIP extraction rejects normalized destination type and platform case collisions before any write.
- Symlink targets are relative to the opened real destination parent. Workspace HTTP serving permits in-root relative or absolute symlinks only after resolving and checking their final target beneath the workspace root.
- Read actual decoded bytes within budgets. Check ZIP close, file sync and close, and publish each file atomically with Windows replacement retries. Validation failures preserve destinations; unrelated I/O failure does not imply a multi-file rollback.
- Conditional writes, copies and moves check destination versions under `desktopMutationMu` and publish through rooted atomic operations. HTTP edits require observed strong ETags; creation uses `If-None-Match: *`. Existing destination symlinks are conflicts, including dangling links; moving the link entry itself to a new name remains supported. Copies stage the whole tree with count, depth and byte budgets; cancellation preserves the previous destination.
- When the server reuses a Desktop service, compare canonical configurations from `NormalizeConfig`; raw paths may be relative and omitted storage paths receive defaults in `NewService`, otherwise later requests can close a healthy live service.
- `TrashPaths` preflights the whole selection before moving any entry. Notes use `Trash/Notes/<uuid>/<original Notes subpath>` and restore that subpath. Reject root/ancestor and overlapping selections. Generic HTTP moves cannot bypass Notes trash transitions; older ordinary Trash entries keep their existing interpretation.

## Work Guidance

- Code Studio's managed image includes GCC, libc development headers and make;
  the runtime probe requires GCC alongside Go, Python and Node. The shared
  sample map seeds `hello.go`, `hello.py` and `hello.c`. Existing workspaces with
  both original samples gain only a missing `hello.c`; host and container seeds
  use exclusive creation and preserve existing files and symlinks.

- Desktop authority consists of scopes, readonly and runtime/tool grants.
  `control_level` is retired; old YAML remains readable and normal config saves
  remove that unused key. Never derive permissions from a UI confirmation mode.
- Live readonly changes use `SetReadOnly` and preserve the service/database and
  read clients. HTTP owner contexts carry a revocable final-publication gate.
- SSH/VNC cancellation begins before TCP and handshake; close both transports
  on cancellation and never install a late SSH client. Verify
  `TestDesktopSSHDialCancelsStalledHandshake` and the proxy/RFB suites.
- SFTP mutation JSON `device_id` must match the query ID authorized by the server guard before Vault access or dialing; multipart uploads keep the same query/body consistency check.
- Quick Connect sends that same URL-encoded query device ID for every SFTP write,
  including multipart uploads. Device binding does not add a remote home jail.

Keep temporary files private and clean them on failure.

- `WriteFileStreamConditional` stages bounded binary media outside the mutation
  lock, then rechecks readonly, path and preconditions before rooted publication.
  Its preconditions use `FileWriteState.Version`, not `Data`; nil is create-only.
  Notes and standalone widget HTML keep their content-validating byte writers.
  Verify `TestDesktopFileStream*` for large files, conflicts, cancellation and
  publication revocation. Video Studio uses this path for imports and exports.

## Verification

`go test ./internal/desktop` and server Desktop archive/read-only tests.

## Child DOX Index

- `pets_assets/AGENTS.md` owns OpenPets sprites, licensing and asset validation.
