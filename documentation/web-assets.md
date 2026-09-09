# External web resources

AuraGo's backend is a portable executable. The complete browser interface is a
separate, platform-independent resource set served locally by that executable.
Chat, Config, Desktop, animated personas, speech WASM/ONNX, NassCAD, desktop pets
and Game Maker runtime/art resources are in this set. Browser URLs remain on the
AuraGo origin; there is no new browser CDN dependency.

Only a small recovery/login page, prompts, model catalog, setup defaults and
small backend/protocol resources remain embedded. A newly copied executable
without configuration or resources opens recovery at `http://127.0.0.1:8088`.
It does not initialize integrations, create a vault, or start recording.
`--recovery-address` changes this initial listener. Existing configurations
continue through normal startup and authentication, including password and TOTP.

## Build and install

Run from the repository root after generating UI bundles:

```bash
node scripts/build-ui-bundles.js
go run ./cmd/assetpack -out deploy -stage assets/web
go build -trimpath -ldflags="-s -w $(cat deploy/web-assets.ldflags)" -o aurago ./cmd/aurago
./aurago --check-assets
```

PowerShell builds use `$assetFlags = Get-Content -Raw deploy/web-assets.ldflags`
and `go build -trimpath -ldflags "-s -w $assetFlags" -o aurago.exe ./cmd/aurago`.
`start.sh` and `start.bat` perform the packaging step. A plain, unpinned `go build`
intentionally provides recovery only. Never distribute that as a complete release.

If recovery shows no resource identifier and disables download, the executable
was built without these flags. Running `assetpack` alone cannot repair that
executable, even when the matching files already exist on disk. Rebuild with the
generated flags, run `--check-assets`, then restart the service. Recovery shows
the complete build commands for the server's operating system in this case.

`assets/web-assets.json` is the production inclusion manifest. The packer excludes
tests, maps, authoring sources, DOX and runtime state, keeps runtime license
notices, normalizes text line endings and archive ordering/timestamps, and emits:

- `aurago-web-assets-<assetSetID>.tar.gz` and `web-assets.json` release metadata;
- `web-assets.ldflags`, the exact build inputs; `web-assets.name`, for scripts;
- `<asset-root>/<assetSetID>/`, a verified immutable installation.

The set ID hashes its per-file SHA-256 manifest. The executable pins this ID,
archive SHA-256 and compressed byte length. Use `-release v...` on the packer
to pin its immutable GitHub release download URL. `make_release.bat` does this
with its selected tag; `make_deploy.sh` uses `AURAGO_RELEASE_TAG` when provided.
Local builds have no download URL. Never substitute a `latest` resource set.

The default root is `assets/web` alongside the executable, or alongside `bin/`
for the standard installer layout. Override with `--assets-dir` or
`AURAGO_ASSETS_DIR`. This path must stay outside the agent workspace. Docker
ships a separate resource layer under `/opt/aurago/web-assets`, outside writable
application volumes. It requires no first-start download.

For an offline installation, copy the executable and its matching archive:

```bash
./aurago --install-assets aurago-web-assets-<assetSetID>.tar.gz
./aurago --check-assets
```

Alternatively copy the complete `assets/web/<assetSetID>` directory. Installers
can use `--import-assets-dir <unpacked-root>` to verify and atomically import it.
These CLI operations run before config/vault initialization. `--assets-info`
prints the executable's exact pin. The recovery page offers the pinned download
when available, and a restart through the existing process lifecycle. A directly
launched executable without a supervisor must be started again manually.

## Integrity and lifecycle

Every startup verifies the manifest and all file hashes before registering the
full UI. Reads use `os.Root` and manifest-scoped paths. Installation bounds
compressed/expanded sizes and file counts and rejects traversal, links, duplicate
paths and executable archive modes. A process lock serializes installation;
staging directories are published only after complete verification. Failed
imports preserve existing sets. Damaged sets are retained with an `.invalid-...`
suffix. Windows publication tolerates brief antivirus sharing locks.

Installation requires an administrator and a same-origin POST even when ordinary
authentication is disabled. It never disables configured authentication. Templates
and translations are captured at startup, so installing resources requires a
restart. Missing resources do not block Desktop initialization; existing NassCAD
workspace apps and installed pets are preserved. Game Maker reports unavailable
resources explicitly; successful exports still contain their own runtime/art.

The UI's `BuildVersion` is the asset-set ID, stable across backend-only builds.
Requests with another version return HTTP 409 with a reload instruction rather
than silently returning new files. Unversioned subresources are network-only,
including in the service worker. Versioned resources retain cache and Range
support; HTML, authentication and APIs are not used as offline UI substitutes.
`/api/assets/status` reports the expected set and readiness; `/api/system/info`
adds `asset_set_id` and `assets_ready`. `/api/ready` retains its backend status
and adds `X-AuraGo-Asset-Set` and `X-AuraGo-Assets-Ready` headers. A pinned binary's
CLI health check additionally requires the running server's matching, ready set.

## Releases and rollback

Build binaries and the shared archive together and publish both with checksums.
The normal installer verifies the matching set before startup. The updater imports
and checks the set against the staged executable before replacing its binary.
Keep the previous binary and set: rollback selects the old immutable ID without
replacing files in the new set. Backend-only updates reuse identical sets locally.

For the transition release, `resources.dat` also carries the unpacked set under
`assets/web`. Old updater scripts already copy `assets/` before refreshing
themselves, so they can migrate to a thin executable. The new updater uses a
verified, atomic directory import. Do not remove this compatibility copy until
old-updater migration is no longer required. Retained sets are not automatically
deleted; remove obsolete versions deliberately after rollback is no longer needed.

`node scripts/check-web-assets.mjs <stripped-binary>...` enforces an initial
120 MB executable ceiling, an 8 MB first-party embed budget and a 1 MB bootstrap
limit. The packaging CI checks Windows and Linux. Production assets are never
committed as generated archives; source files and the inclusion manifest are.

Checks: `go test ./internal/webassets ./cmd/assetpack`, focused Desktop/Game Maker
and server asset tests, `node scripts/build-ui-bundles.js --check`, and existing
Live Speech checks. Frontend/consumer tests explicitly use source fixtures; there
is no production source-directory fallback.

The initial design deliberately uses one complete resource archive. It reduces
the executable, not the total disk payload. Optional feature packs can be added
later if download or installation size becomes the next constraint. Persona
animation behavior is unchanged: discrete mouth poses follow output rhythm,
without phoneme-accurate synchronization.

## Migration acceptance (2026-09-09)

The reviewed source snapshot builds stripped Windows amd64 at 106.56 MB and
Linux amd64 at 106.32 MB, with 4.24 MB of first-party embeds (12.75 KB recovery).
Its shared archive contains 2,790 files: 276.00 MB unpacked, 171.62 MB compressed.
The earlier 416.85 MB executable measurement included concurrent, unrelated
Galaxa artwork; those changes are excluded from this reviewed archive. All sizes
are decimal. Source-only files and generated archives are not binary payloads.

Native Windows/Linux archive installation and verification passed, including
Linux access as an unprivileged user with the Docker resource permissions.
Browser acceptance with external networking blocked covered recovery, authenticated
Chat/Config/Desktop, all twelve animated personas, rapid switching, minimizing,
reopening, light/dark narrow windows and PNG fallback after a forced WASM failure.
The existing weather widget attempted its usual external request; no CDN was used.
Focused Go, Live Speech Node, bundle and packaging/audit checks passed. The full
UI suite retains four pre-existing failures: Spark i18n fallback, TeeVee line
budget/contrast and the Desktop normalize-z-index source boundary.

Docker runtime acceptance requires a running daemon, unavailable on this machine.
Published-release download, a real installed-service upgrade/rollback and native
macOS/ARM execution remain release acceptance tasks. This migration did not change
audio providers or add a new real-provider TTS acceptance claim.
