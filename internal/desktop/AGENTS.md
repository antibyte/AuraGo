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
- Pet lookups validate ids against `petIDPattern` and resolve manifest paths (`spritesheetPath`) only as relative, slash-separated paths without `..`, `:` or `\`, through an `os.Root` at `Pets` with every component Lstat-checked (real directories, regular final file, no links). Bundled-pet repair reinstalls only missing files (created with `O_EXCL`) and replaces a manifest only when it cannot be parsed, so customised bundled pets survive; it never writes through a link, and a pet refused for another reason (link, special file, missing custom spritesheet) is logged and skipped. Lookup, listing and repair errors name the pet id or "pets directory", never a host path (the OS error is logged server-side); user `InstallPet`/`DeletePet` errors still wrap OS errors.
- When the server reuses a Desktop service, compare canonical configurations from `NormalizeConfig`; raw paths may be relative and omitted storage paths receive defaults in `NewService`, otherwise later requests can close a healthy live service.
- `TrashPaths` preflights the whole selection before moving any entry. Notes use `Trash/Notes/<uuid>/<original Notes subpath>` and restore that subpath. Reject root/ancestor and overlapping selections. Generic HTTP moves cannot bypass Notes trash transitions; older ordinary Trash entries keep their existing interpretation.
- SFTP paths are relative to the remote home: `normalizeSFTPRemotePath` rejects traversal, `~`, sensitive POSIX roots, Windows drive prefixes (`C:`) and UNC hosts (`//server`, `\\server`); a bare `/` or `\\` is the home.
- On Windows `openFileNoFollow` refuses a symlink at `Lstat`, pins that entry's file ID before opening and refuses a handle that is not the same file; `O_TRUNC` is applied only after that check, so a swap between the check and the open is refused before any truncation (`TestOpenFileNoFollowRefusesEntrySwappedAfterLstat`).
- OpenSCAD renders one export at a time. The probe container `aurago-openscad` mounts no job tree. Each export container is named `aurago-openscad-<jobID>-<export>`, bind-mounts only that job directory at `/work`, uses network `none`, drops all capabilities, sets `no-new-privileges`, a read-only rootfs, and a `/tmp` tmpfs. Cancel kills that container before `renderMu` is released. The OpenSCAD compiler default is the manifest-list pin `openscad/openscad@sha256:147e48525bec392bcf628d7a6d5ea4ccac71b16251952328f86e1061cbf47c37` (`defaultOpenSCADImage`). Config defaults and `config_template.yaml` use that same reference. A configured `virtual_desktop.openscad.image` overrides it. The image user stays the image default.
- OpenSCAD output and save reads use `openFileNoFollow` and reject the file when its size exceeds the configured cap, before hashing. The compiler log uses that same no-follow open, reads at most 6000 bytes, and still attaches to a successful export; a symlink or other non-regular log is a short read error and its target is not included. Job-file HTTP responses set `nosniff` and `Content-Security-Policy` with `script-src 'none'`, and they keep inline disposition unless `download=1`.
- OpenSCAD status and render JSON omit host paths. `max_concurrent_jobs` loads from YAML and the effective queue is 1. One render deadline covers every export in the request.
- `openscad_render` refuses when the virtual desktop is read-only, with the message `virtual desktop is read-only`, before it compiles.

## Work Guidance

- Code Studio's managed image includes GCC, libc development headers and make;
  the runtime probe requires GCC alongside Go, Python and Node. The shared
  sample map seeds `hello.go`, `hello.py` and `hello.c`. Existing workspaces with
  both original samples gain only a missing `hello.c`; host and container seeds
  use exclusive creation and preserve existing files and symlinks.
- Managed probes and workspace scripts use non-login shells to retain the image
  PATH, including `/usr/local/go/bin`. Never rebuild a healthy image because a
  login profile hid its tools. Verify `TestCodeStudioRuntimeRequiresCCompiler`.

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
- Host Quick Connect serial is an admin-only, disabled-by-default WebSocket
  bridge governed by Desktop readonly and the live serial grant. Open only an
  exact native-enumerator port name; keep frames binary, writes ordered, and
  device bytes out of logs. Its lease is shared with MeshCore and remains held
  until the serial port has closed.
- Retro-Net own entries live in the Desktop setting `retronet.entries`
  (validated by `retronet.ValidateEntriesDocument`, admin-only through the
  settings API). `SetRetroNetHostKey(ctx, dialed, fingerprint)` writes a
  first-contact SSH key into the own SSH entry that was dialed (the server
  calls it only for an administrator's confirmation) with compare-and-set
  under a package mutex, at most 3 attempts so concurrent settings saves are
  never overwritten, and refuses unknown, non-SSH or already keyed entries and
  entries whose protocol, host, port or user no longer equal the dialed ones. `SameHostWebSocketOrigin` is the exported
  strict origin check (empty Origin refused) for server-side WebSocket
  adapters. Verify `go test ./internal/desktop -run 'RetroNet|CompareAndSetSetting|SameHostWebSocketOrigin' -count=1`.

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
