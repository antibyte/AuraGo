# Desktop Store

## Purpose

Store app configuration, runtime, assets, and publication.

## Ownership

`internal/desktopstore` owns this domain. The contracts below also bind related server, UI, config, asset, and test work through the root routing table.

## Local Contracts

### CommandCode Image

- `commandcode_assets/Dockerfile` pins `command-code@1.74.1` on Node 22.
  The release workflow and embedded fallback build share this build context;
  update the exact package version here to invalidate Docker's install cache.
  Verify `command-code --version` as the unprivileged image user after updates.
- The Store pulls `ghcr.io/antibyte/aurago-commandcode:latest`. Source updates
  require image publication and a Store update before installed apps change.
- The entrypoint adds one absolute import of image-owned
  `/usr/local/share/aurago/commandcode-preview.md` to user memory
  `~/.commandcode/AGENTS.md`, including existing home volumes. Preserve all
  personal/project instructions and never duplicate the import on restart.
  Keep the guide in both published and embedded fallback build contexts; it
  explains `/workspace`, preview ports/gateway and shell lifecycle. Verify
  `TestCommandCodePreviewMemoryPreservesInstructions` with Bash available.
- CommandCode preview HTTP and WebSocket requests strip reserved AuraGo cookies
  and internal credential headers. Preserve guest Authorization, CSRF and login
  cookies; verify `TestCommandCodePreviewPreservesGuestAuthWithoutAuraGoCredentials`.

### Read-only Docker Monitoring

- Dozzle and the optional local Beszel agent must use a managed Tecnativa socket
  proxy with the exact read-only environment profile `POST=0`,
  `CONTAINERS=1`, `EVENTS=1`, `INFO=1`, `PING=1`, `SECRETS=0`, `VERSION=1`,
  and `AUTH=0`. Do not grant `EXEC`, image/build, network, or volume access.
- Dozzle and its proxy share only `aurago-store-dozzle-net`; Dozzle connects via
  `DOZZLE_REMOTE_HOST=tcp://aurago-store-dozzle-socket-proxy:2375`. The proxy
  has no published host port. The Beszel proxy exposes only container port 2375
  on a dynamically allocated `127.0.0.1` host binding. Its optional agent keeps
  host networking and metrics, uses the loopback proxy through `DOCKER_HOST`,
  and never mounts the Docker socket. Keep hub/agent data and Vault secrets in
  their existing paths.
- Never add a direct Docker socket bind to either monitoring app or the Beszel
  agent. Existing installations report the computed `update_required` flag
  while their stored container configuration uses the old direct socket path.
  Startup and status reads do not recreate containers; migrate only when the
  operator invokes the existing Store Update action, which replaces the
  app/companion configuration through the normal update and rollback path.
- Verify catalog config, loopback port allocation against all app and companion
  ports, and legacy migration behavior with `go test ./internal/desktopstore`.
  A local Docker proxy smoke test should cover successful logs, events and stats
  reads plus denial of Engine mutation methods.

### God's Eye View Store Contract

- `internal/desktopstore/gods_eye.go` owns app-specific setup for catalog ID
  `gods-eye-view`. The admin GET/PUT `/api/desktop/store/apps/gods-eye-view/config`
  uses existing authentication, CSRF, Desktop/Docker write gates and the exclusive
  Store operation slot. GET returns configured flags, exact allowed HTTP(S)
  origins and pending status only; PUT accepts known keys, explicit removals and
  at most eight origins. Blank/omitted key fields preserve existing values.
- Every catalog entry declares `Category` from `desktop.DesktopAppCategories()`
  (office, media, creative, ai, dev, system, comms, games; never `installed`),
  and an entry that fronts a builtin (`DesktopAppID`) uses that builtin's
  category. `desktopAppManifest` copies it into the Desktop manifest, so the
  start menu sorts Store apps with the builtins; `ScheduleBrandingReconcile`
  back-fills installed apps after a restart. Verify
  `TestStoreCatalogEntriesCarryStartMenuCategories`.
- Every Store catalog icon must pass the real Desktop icon allowlist before
  app/shortcut registration. `gods-eye-view` is a dedicated allowed icon using
  the packaged logo. Verify catalog icons and installation with the real
  Desktop service/Vault, not only the Store's mock adapters.
- Desired and previous active provider settings live only in encrypted Vault
  `desktop_store_gods-eye-view_config`. SQLite and operation JSON contain no
  provider values, only the active revision. Resolve credentials when creating
  a container, never inherit AuraGo provider credentials, and preserve the old
  runtime configuration on failed replacement. Retain settings on uninstall
  unless `delete_data` explicitly removes both cache volume and Vault settings.
- `internal/desktopstore/gods_eye_assets/` builds pinned upstream commit
  `759652207fd1279ece97f0f19af566feb9a82146` with Node 24 and `npm ci`. The full
  Vite service on 4173 retains live proxies, runs as non-root and mounts only
  the named `.gev-cache` volume. Installations pull the published
  `ghcr.io/antibyte/aurago-gods-eye-view:gev-7596522-1` image; never build locally
  during installation. Catalog inclusion alone does not start the service.
- The image adapter removes upstream POWER-UP settings and rejects `/api/setup`
  writes. `frame-ancestors` allows only configured AuraGo origins and defaults
  to none. The app-specific Tailscale proxy preserves that policy. Local, LAN
  and Tailscale access keep the existing Store paths; LAN access is optional
  and grants access to configured provider quotas. Google/Cesium keys remain
  browser-visible by upstream design; other keys stay server-side.
- The release workflow builds amd64/arm64, runs the adapter/provider mock test
  within each image build, generates provenance/SBOM and signs the image digest.
  Manual `docker-publish.yml` dispatch accepts `image=gods-eye-view` to publish
  only this Store image with the workflow's package-write/OIDC credentials.
  Keep cosign-installer on a verified release tag; the bare `v4` ref is absent.
  Verify with Store/handler/Tailscale `TestGodsEye*`, the UI browser contract,
  and anonymous image pulls for both architectures before claiming publication.

### Catalog Host Binds

- Host binds declared in `DefaultCatalog()` (`HostBinds` of an entry or a
  companion, such as the read-only Docker socket of the socket-proxy
  companions) are the only binds the Store passes as trusted to
  `tools.DockerCreateRequestContextWithTrustedBinds`, matched by exact host
  path, container path and read-only flag against the code catalog at create
  time. Persisted record binds are trusted only while they still equal the
  current catalog; managed workspace binds and every other bind keep the
  generic Docker bind policy. A new catalog `HostBinds` entry is trusted
  automatically, so review it like a policy exemption. Verify
  `TestToolsDockerAdapterTrustsOnlyCatalogHostBinds`.
- The trusted catalog binds are exactly the read-only Docker sockets of the
  Arcane, Dozzle and Beszel `socket-proxy` companions; without the exemption
  their creation fails the generic bind check. Legacy Dozzle and Beszel-agent
  records with a direct socket bind are not trusted: the Store Update rebuilds
  them from the catalog. A failed Update restores the parked legacy containers
  without a create, so the bind check is not involved; only the
  remove-and-recreate fallback (engine without rename) still fails it for such
  records.

### Store Container Hardening

- Hardening beyond Docker's defaults plus `no-new-privileges` (`CapDrop`,
  `CapAdd`, `ReadonlyRootfs`, `Tmpfs`, `PidsLimit`) is opt-in per catalog image:
  `CatalogEntry.Hardening` for an app, `CompanionTemplate.Hardening` for a
  companion. There is no global default; many Store images start as root and
  switch users (`PUID`/`PGID`, s6-overlay) or run many threads, so a blanket
  setting breaks them. Both fields are `json:"-"`, so the catalog API is
  unchanged.
- An opt-in needs live evidence first: run the image with exactly that
  hardening on a real Docker host and record the date, host and image digest in
  the catalog comment and the commit body, then add the `app` or
  `app/companion` key to `verifiedCatalogHardening` in `hardening_test.go`.
  `TestCatalogHardeningOptInsAreVerifiedOnly` fails for any unverified opt-in.
- Hardening is resolved by app/companion ID from the current catalog every time
  a container is created (`runtimeContainerSpec`, `companionRuntimeSpec`);
  installed records do not store it and are not migrated. An opt-in therefore
  applies on the next create (install, update, a fallback rollback recreate,
  or companion recreation), not retroactively to running containers.
- A rollback restores the parked previous containers, which keep the
  hardening they were created with
  (`TestRollbackRestoresParkedContainersWithoutReapplyingHardening`). Only the
  fallback for engines without rename, and containers that were already
  missing, recreate the previous record's image with the current catalog
  hardening. Keep image swaps and hardening changes in separate catalog
  changes for that path
  (`TestCatalogHardeningAppliesOnRollbackWithCurrentCatalog`).
- Most catalog images use floating tags such as `:latest`, so a probe verifies
  the digest it ran against, not the tag. Re-verify an opted-in image when its
  upstream changes and update the recorded digest.
- Only `arcane/socket-proxy` is opted in. The Dozzle and Beszel `socket-proxy`
  companions use the same Tecnativa image with the read-only monitoring
  profile and their own network setup; they keep `no-new-privileges` and
  Docker's default capabilities until each is probed with `CapDrop: ALL` in
  its own configuration.
- Verify `TestCatalogHardening*`, `TestDockerCreatePayload*Hardening*` and
  `TestInstallArcaneAppliesOnlyVerifiedHardening`.

### Store Update Rollback

- `update()` replaces companions first and the app last. Each previous
  container is stopped and renamed to `<name>.prev` (`parkedContainerName`)
  before its replacement is created. A stopped container holds no host port
  and no network DNS name and keeps its ID, volumes, binds and networks.
- Parked containers are removed only after the replacements run (readiness)
  and the record is saved. A failed update removes the replacements first,
  renames the parked containers back and starts them when the app was
  running; the previous record is kept. The previous companions keep their
  env this way (`CompanionApp.Env` is not persisted, so a record-based
  recreate would lose it; `TestFailedUpdateRestoresPreviousCompanionsWithTheirEnv`).
- A rename conflict means a stale `.prev` from an earlier update; it is
  removed (the container under the original name wins). A missing container
  whose `.prev` exists (interrupted update) is adopted. Engines that cannot
  rename fall back to the old remove-and-recreate path with a warning.
- Companions the update does not recreate (the Beszel agent without its
  Vault secrets) are left alone by the update and its rollback.
- Startup never touches parked containers. The next Update adopts or replaces
  them, and Uninstall removes them.
- Verify `TestParkContainerHandlesEveryEngineAnswer`, `TestLegacy*UpdateFailure*`,
  `TestUpdate*Parked*`, `TestUpdateOfAStoppedAppRestoresItStopped`,
  `TestFailedUpdateRestoresPreviousCompanionsWithTheirEnv`,
  `TestUninstallRemovesParkedLeftovers`; the four remove-and-recreate tests set
  `renameErr` and pin the fallback.

## Verification

- Desktop Store jobs inherit the Desktop revocation context. Stop remains an
  explicit cleanup action, still subject to Docker permissions. Interrupted
  pending/running operations record a bounded cleanup failure using
  `InterruptOperation`; already terminal operations cannot be overwritten.

- Run `go test ./internal/desktopstore` and the named cross-component checks in the contracts above when those paths change.

## Child DOX Index

None.
