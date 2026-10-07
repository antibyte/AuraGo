# Desktop services

## Purpose

Desktop file operations, rooted mutations and local presentation assets.

## Ownership

The Service owns authorization, read-only state, mutation locks and cache invalidation; HTTP adapters delegate these operations.

## Local Contracts

- Archive create/extract use rooted operations and the same read-only/Notes gates as ordinary writes. Preflight every ZIP name, mode, size/count, namespace conflict and existing target before writing; reject escaping symlinks, special files, UNC/drive and traversal names.
- Read actual decoded bytes within budgets. Check ZIP close, file sync and close, and publish each file atomically with Windows replacement retries. Validation failures preserve destinations; unrelated I/O failure does not imply a multi-file rollback.
- Conditional writes, copies and moves check destination versions under `desktopMutationMu` and publish through rooted atomic operations. HTTP edits require observed strong ETags; creation uses `If-None-Match: *`. Existing destination symlinks are conflicts, including dangling links; moving the link entry itself to a new name remains supported. Copies stage the whole tree with count, depth and byte budgets; cancellation preserves the previous destination.
- Pet lookups validate ids against `petIDPattern` and resolve manifest paths (`spritesheetPath`) only as relative, slash-separated paths without `..`, `:` or `\`, through an `os.Root` at `Pets` with every component Lstat-checked (real directories, regular final file, no links). Bundled-pet repair reinstalls only missing files (created with `O_EXCL`) and replaces a manifest only when it cannot be parsed, so customised bundled pets survive; it never writes through a link, and a pet refused for another reason (link, special file, missing custom spritesheet) is logged and skipped. Lookup, listing and repair errors name the pet id or "pets directory", never a host path (the OS error is logged server-side); user `InstallPet`/`DeletePet` errors still wrap OS errors.
- `TrashPaths` preflights the whole selection before moving any entry. Notes use `Trash/Notes/<uuid>/<original Notes subpath>` and restore that subpath. Reject root/ancestor and overlapping selections. Generic HTTP moves cannot bypass Notes trash transitions; older ordinary Trash entries keep their existing interpretation.
- SFTP paths are relative to the remote home: `normalizeSFTPRemotePath` rejects traversal, `~`, sensitive POSIX roots, Windows drive prefixes (`C:`) and UNC hosts (`//server`, `\\server`); a bare `/` or `\\` is the home.
- On Windows `openFileNoFollow` refuses a symlink at `Lstat`, pins that entry's file ID before opening and refuses a handle that is not the same file (`TestOpenFileNoFollowRefusesEntrySwappedAfterLstat`).

## Work Guidance

- Desktop authority consists of scopes, readonly and runtime/tool grants.
  `control_level` is retired; old YAML remains readable and normal config saves
  remove that unused key. Never derive permissions from a UI confirmation mode.
- Live readonly changes use `SetReadOnly` and preserve the service/database and
  read clients. HTTP owner contexts carry a revocable final-publication gate.
- SSH/VNC cancellation begins before TCP and handshake; close both transports
  on cancellation and never install a late SSH client. Verify
  `TestDesktopSSHDialCancelsStalledHandshake` and the proxy/RFB suites.

Keep temporary files private and clean them on failure.

## Verification

`go test ./internal/desktop` and server Desktop archive/read-only tests.

## Child DOX Index

- `pets_assets/AGENTS.md` owns OpenPets sprites, licensing and asset validation.
