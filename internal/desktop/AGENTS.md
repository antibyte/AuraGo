# Desktop services

## Purpose

Desktop file operations, rooted mutations and local presentation assets.

## Ownership

The Service owns authorization, read-only state, mutation locks and cache invalidation; HTTP adapters delegate these operations.

## Local Contracts

<!-- pinball:start -->
- AuraPinball (`pinball`, category `games`) is a local built-in app with no capability
  gate. Keep `open_maximized=true` and app-owned `/img/desktop-icons/pinball.svg`;
  its icon key stays outside theme substitutions. Runtime ownership is
  `../../ui/js/desktop/apps/pinball/AGENTS.md`. Verify `TestBuiltinPinballApp`.
<!-- pinball:end -->

- Goldfish Adventure (`goldfish-adventure`, category `games`) is a built-in
  local point-and-click app without a capability/config gate. Its app-owned
  `logo_path` is `/img/goldfish-adventure/icon.png`; keep it out of themed icon
  substitution. Runtime and browser-save ownership belongs to
  `../../ui/js/desktop/apps/goldfish-adventure/AGENTS.md`. Verify
  `go test ./ui -run TestGoldfishAppRegistration` and the built-in category test.

- Aura Drift (`aura-drift`, category `games`) is a built-in offline single-player
  racing app with no capability gate and no config switch. Its app-owned icon is
  `logo_path` `/img/desktop-icons/aura-drift.svg`; the `Icon` value `aura-drift`
  is deliberately absent from `desktopPreferredIconNames` and every icon theme
  (as for ECalc and Synth Studio), so every theme shows the logo. App modules,
  locale keys and tests belong to `../../ui/js/desktop/apps/AGENTS.md`.
  Verify `TestAuraDriftAppRegistration` and `go test ./internal/desktop -run TestBuiltinAppsCarryStartMenuCategories`.

- The WebAmp launcher (`music-player`) declares app-owned `logo_path` `/img/desktop-icons/music-player.svg`; keep it visually distinct from the native `audio-player` and preserve their existing icon aliases.

- FindIt is a built-in optional physical inventory app with app-owned artwork at `/img/findit`. Its backend/storage/agent contract belongs to `../findit/AGENTS.md`; its large component atlas never changes the global Desktop sprite grid.

- ECalc (`ecalc`, category `dev`) is a built-in electronics formula app with no capability gate. Its icon is app-owned artwork, `logo_path` `/img/desktop-icons/ecalc.svg`, and its `Icon` value `ecalc` is deliberately absent from `desktopPreferredIconNames` and every icon theme (as for Synth Studio), so every theme shows the logo; adding a themed `ecalc` icon would replace it. Its catalog route is `GET /api/ecalc/catalog` (`../server/AGENTS.md`). Verify `TestBuiltinECalcApp*` and `go test ./ui -run TestDesktopECalc`.

- Storymaker (`storymaker`, category `creative`, `open_maximized`) is a built-in explainer-video app behind the `storymaker` capability (`Requires`, the `storymaker.enabled` switch; the capability case lives in `../server/bluetooth_live.go`). Its icon is app-owned artwork, `logo_path` `/img/desktop-icons/storymaker.svg`, and its `Icon` value `storymaker` is deliberately absent from `desktopPreferredIconNames` and every icon theme (as for ECalc). Its routes are `/api/desktop/storymaker/*` (`../server/AGENTS.md`), its projects live in `Documents/Storymaker/`. Verify `TestBuiltinStorymakerApp*` and `go test ./ui -run TestDesktopStorymaker`.

- `WriteStorymakerVideo` is the only desktop write into the read-only `Videos` mount. It takes a slug (`^[A-Za-z0-9-]{1,80}$`) and builds the path itself, `Videos/Storymaker/<slug>.mp4` with `-2`, `-3` on collision. It refuses read-only, checks read-only again under the lock, and needs free space of size plus 64 MiB. It creates `Storymaker` on the first copy and skips `RegisterMedia`. Verify `go test ./internal/desktop -run WriteStorymakerVideo`.

- The Storymaker handoff adds no Service method. Video Studio's `source_path` import opens files below `Documents/Storymaker/<id>/exports/` through `OpenPreviewFile` and copies them into its project. The library copy (`WriteStorymakerVideo`) exists for discoverability only: camera import accepts top-level `Videos/<file>` only, so the handoff never reads `Videos/Storymaker/`. Verify `go test ./internal/desktop -run WriteStorymakerVideo -count=1`.

- Desktop cloud transfers use `StatStoragePath` for fresh rooted metadata,
  `CreateStorageDirectory` for exclusive directory creation,
  `WriteFileStreamConditional` for bounded publication, and
  `RemoveStorageSource` for versioned file / empty-directory cleanup. Preserve
  Notes, managed widget and media-mount restrictions. See
  `documentation/desktop-cloud-storage.md`; verify `TestDesktopStorage*`.

- Archive create/extract use rooted operations and the same read-only/Notes gates as ordinary writes. Preflight every member name, mode, size/count, namespace conflict and existing target before writing; reject escaping symlinks, special files, UNC/drive and traversal names. ZIP stays the default create format. tar, tar.gz, tar.xz and tar.zst can also be created. tar.bz2 and single-file gz, bz2, xz and zst are read-only. 7z, rar, cab, iso, wim and the other host formats, including ext, squashfs, btrfs, xfs, erofs, cramfs and disk images, open only when a userspace tool is on PATH (`7zz`/`7z`, `debugfs`, `unsquashfs`, `btrfs`, `fsck.erofs`, `fsck.cramfs`, `virt-copy-out` or `bsdtar`). Images are never loop-mounted. Host tools write only into a private temp directory. The copy step does not follow directory links, and a hard link whose link count is higher than the names inside that directory is treated as a link. Filesystem images list symlinks and special files and skip them; classic archives reject the whole extract. Tar hard links to an earlier regular file are copied. Repeated directory entries are ignored. Encrypted 7z is refused.
- Archive listing uses the shared 10,000-entry limit and the 500 MiB expansion budget before publishing. Extraction rejects normalized destination type and platform case collisions before any write.
- Symlink targets are relative to the opened real destination parent. Workspace HTTP serving permits in-root relative or absolute symlinks only after resolving and checking their final target beneath the workspace root.
- Read actual decoded bytes within budgets. Check ZIP close, file sync and close, and publish each file atomically with Windows replacement retries. Validation failures preserve destinations; unrelated I/O failure does not imply a multi-file rollback.
- Conditional writes, copies and moves check destination versions under `desktopMutationMu` and publish through rooted atomic operations. HTTP edits require observed strong ETags; creation uses `If-None-Match: *`. Existing destination symlinks are conflicts, including dangling links; moving the link entry itself to a new name remains supported. Copies stage the whole tree with count, depth and byte budgets; cancellation preserves the previous destination.
- Pet lookups validate ids against `petIDPattern` and resolve manifest paths (`spritesheetPath`) only as relative, slash-separated paths without `..`, `:` or `\`, through an `os.Root` at `Pets` with every component Lstat-checked (real directories, regular final file, no links). Bundled-pet repair reinstalls only missing files (created with `O_EXCL`) and replaces a manifest only when it cannot be parsed, so customised bundled pets survive; it never writes through a link, and a pet refused for another reason (link, special file, missing custom spritesheet) is logged and skipped. Lookup, listing and repair errors name the pet id or "pets directory", never a host path (the OS error is logged server-side); user `InstallPet`/`DeletePet` errors still wrap OS errors.
- `aurago-punk-3d` is an additional bundled persona with subcategory `3d` and
  an ordinary OpenPets fallback. The browser maps only that id/subcategory pair
  to fixed version-bound 3D assets; this adds no manifest-supplied model URL or
  server-side execution. Preserve `aurago-punk` and the current active pet.
- When the server reuses a Desktop service, compare canonical configurations from `NormalizeConfig`; raw paths may be relative and omitted storage paths receive defaults in `NewService`, otherwise later requests can close a healthy live service.
- `TrashPaths` preflights the whole selection before moving any entry. Notes use `Trash/Notes/<uuid>/<original Notes subpath>` and restore that subpath. Reject root/ancestor and overlapping selections. Generic HTTP moves cannot bypass Notes trash transitions; older ordinary Trash entries keep their existing interpretation.
- SFTP paths are relative to the remote home: `normalizeSFTPRemotePath` rejects traversal, `~`, sensitive POSIX roots, Windows drive prefixes (`C:`) and UNC hosts (`//server`, `\\server`); a bare `/` or `\\` is the home.
- On Windows `openFileNoFollow` refuses a symlink at `Lstat`, pins that entry's file ID before opening and refuses a handle that is not the same file; `O_TRUNC` is applied only after that check, so a swap between the check and the open is refused before any truncation (`TestOpenFileNoFollowRefusesEntrySwappedAfterLstat`).
- OpenSCAD renders one export at a time. The probe container `aurago-openscad` mounts no job tree. Each export container is named `aurago-openscad-<jobID>-<export>`, bind-mounts only that job directory at `/work`, uses network `none`, drops all capabilities, sets `no-new-privileges`, a read-only rootfs, and a `/tmp` tmpfs. Cancel kills that container before `renderMu` is released. The OpenSCAD compiler default is the manifest-list pin `openscad/openscad@sha256:147e48525bec392bcf628d7a6d5ea4ccac71b16251952328f86e1061cbf47c37` (`defaultOpenSCADImage`). Config defaults and `config_template.yaml` use that same reference. A configured `virtual_desktop.openscad.image` overrides it. The image user stays the image default.
- OpenSCAD output and save reads use `openFileNoFollow` and reject the file when its size exceeds the configured cap, before hashing. The compiler log uses that same no-follow open, reads at most 6000 bytes, and still attaches to a successful export; a symlink or other non-regular log is a short read error and its target is not included. Job-file HTTP responses set `nosniff` and `Content-Security-Policy` with `script-src 'none'`, and they keep inline disposition unless `download=1`.
- OpenSCAD status and render JSON omit host paths. `max_concurrent_jobs` loads from YAML and the effective queue is 1. One render deadline covers every export in the request.
- `openscad_render` refuses when the virtual desktop is read-only, with the message `virtual desktop is read-only`, before it compiles.
- Desktop theme ids: `DesktopThemeIDs()` in `themes.go` is the picker order and the allowed `appearance.theme` values (`DesktopSettingDefinitions()` in `types.go`); `DefaultDesktopTheme` (`standard`) is the theme of a fresh desktop. `DESKTOP_THEMES` in `ui/js/desktop/core/theme-runtime.js` must list the same ids in the same order, and `ui/desktop_theme_registry_test.go` enforces it. `light` and `dark` name colour schemes and are never theme ids. Verify `go test ./ui -run DesktopTheme -count=1`.

- Icon settings retain Papirus by default and accept `tela`, `candy`, `fluent`, `buuf_nestort` alongside the existing sets. Tela/Fluent folder colors persist separately with default `standard`; `DesktopSettingDefinitions` is the backend allowlist. Manifest palette parity and all sixteen locale labels are checked by `TestDesktopAdditionalIcon*`. No database migration or settings API change is required.

## Work Guidance

- NASSCAD 4.7.0 is pinned to upstream `279da7c0f0ca28dfae19c11c8be231f326dfda4f`.
  Rebuild with `node scripts/vendor-nasscad.mjs --source=<release source>` and
  verify with `--check` and `scripts/test-nasscad-browser.mjs`. Preserve the
  CC BY-NC license, local Manifold/IFC/OCCT runtimes and authenticated sibling
  URLs. The seeder publishes the monolithic entry last and preserves existing
  user workspace apps; its lazy assets, guides and license files must all be
  included in `assets/web-assets.json`.
- The reproducible NASSCAD email adaptation is shared by `scripts/nasscad-email-adapter.mjs` and `ui/js/desktop/apps/nasscad-email-frame.js`; the provenance input hash includes both. The Desktop wrapper loads that version-bound frame adapter for existing workspace copies as well. Preserve their files and the exclusive-create seeder. Email capture restores native download/picker behavior after its task; cancelled exports must never fall through to a browser download.

- Code Studio's managed image includes GCC, libc development headers and make;
  the runtime probe requires GCC alongside Go, Python and Node. The shared
  sample map seeds `hello.go`, `hello.py` and `hello.c`. Existing workspaces with
  both original samples gain only a missing `hello.c`; host and container seeds
  use exclusive creation and preserve existing files and symlinks.
- Managed probes and workspace scripts use non-login shells to retain the image
  PATH, including `/usr/local/go/bin`. Never rebuild a healthy image because a
  login profile hid its tools. Verify `TestCodeStudioRuntimeRequiresCCompiler`.
- Code Studio acquisitions cache the container ID and perform one targeted
  inspect on ordinary access. Discovery, workspace repair and seeding run only
  during startup/adoption. Docker I/O stays outside the state mutex; the
  lifecycle lock serializes startup/stop while allowing concurrent acquisitions.
  Do not retry uncertain writes. `HoldTerminal` returns an idempotent lease:
  live sessions suspend idle stop, the last release restarts the idle interval,
  and timer generations reject callbacks superseded by activity. Verify
  `TestCodeContainerCachedParallelAccessDoesOneInspect` and
  `TestCodeContainerTerminalLeasesInvalidateIdleCallbacks`.
- `virtual_desktop.code_studio.allow_network` defaults false. Off creates the
  container with Docker `NetworkMode` `none`. On leaves `NetworkMode` empty so
  Docker uses its default bridge. The container still has no Docker socket and
  drops all capabilities, then adds only `CHOWN` and `FOWNER` for workspace
  ownership repair. `EnsureStarted` removes and recreates a container whose
  mode does not match; workspace content stays on the host mount. Status
  reports `network` from that setting until the container is running, then
  from the inspected mode (`none`, `bridge`, or the raw mode such as `host`).
  HTTP status JSON omits `workspace_host_path` and includes `read_only` from
  the desktop grant. Verify
  `TestCodeContainerEnsureStartedCreatesContainerWithNoPortsAndLimits`,
  `TestCodeContainerEnsureStartedReplacesIsolatedContainerWhenNetworkAllowed`
  and `TestCodeContainerEnsureStartedReplacesBridgedContainerWhenNetworkDisabled`.

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
