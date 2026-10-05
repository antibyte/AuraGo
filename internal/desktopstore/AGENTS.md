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

## Verification

- Desktop Store jobs inherit the Desktop revocation context. Stop remains an
  explicit cleanup action, still subject to Docker permissions. Interrupted
  pending/running operations record a bounded cleanup failure using
  `InterruptOperation`; already terminal operations cannot be overwritten.

- Run `go test ./internal/desktopstore` and the named cross-component checks in the contracts above when those paths change.

## Child DOX Index

None.
