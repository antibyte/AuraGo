# Desktop Store

## Purpose

Store app configuration, runtime, assets, and publication.

## Ownership

`internal/desktopstore` owns this domain. The contracts below also bind related server, UI, config, asset, and test work through the root routing table.

## Local Contracts

### God's Eye View Store Contract

- `internal/desktopstore/gods_eye.go` owns app-specific setup for catalog ID
  `gods-eye-view`. The admin GET/PUT `/api/desktop/store/apps/gods-eye-view/config`
  uses existing authentication, CSRF, Desktop/Docker write gates and the exclusive
  Store operation slot. GET returns configured flags, exact allowed HTTP(S)
  origins and pending status only; PUT accepts known keys, explicit removals and
  at most eight origins. Blank/omitted key fields preserve existing values.
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

- Run `go test ./internal/desktopstore` and the named cross-component checks in the contracts above when those paths change.

## Child DOX Index

None.
