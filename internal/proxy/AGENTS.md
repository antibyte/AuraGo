# Security proxy

## Purpose

The managed Caddy container `aurago-security-proxy` in front of AuraGo:
Caddyfile generation, image selection, container placement and lifecycle.

## Ownership

`internal/proxy` owns the Caddyfile, the proxy images and the container.
`internal/server/proxy_handlers.go` exposes `/api/proxy/*`, `config`
owns the `security_proxy` keys, and `ui/cfg/security_proxy.js` the Config
section. Operator guidance lives in the Security Proxy section of
`documentation/manual/{en,de}/08-integrations.md`.

## Local Contracts

- Config: `replaceConfigSnapshot` publishes every snapshot to the manager
  through `UpdateConfig`. Start and Reload read one snapshot under the
  lifecycle lock; never keep the startup config elsewhere.
- Basic Auth: Vault `proxy_basic_auth_user` / `proxy_basic_auth_pass` load
  into vault-only config fields and are blocked from Python export. The
  Caddyfile carries only a bcrypt hash and is rewritten in place with mode
  0600 (`writeCaddyfile`). Never replace it by rename: the native single-file
  bind mount follows the inode. Missing or unusable credentials fail before
  any Docker mutation; never emit an account-less `basic_auth` block.
- Rate limiting: the official image has no `rate_limit`. With rate limiting
  on, the proxy uses `rateLimitImageName`, built once from the pinned
  `caddy:<version>-builder` plus the pinned `caddy-ratelimit` pseudo-version;
  without it, `caddy:latest` stays tagged `aurago-proxy:latest`. Bump the
  Caddy and module pins together, re-check the generated `rate_limit` and
  `basic_auth` syntax against the built image, and update the host-side
  `docker build` recipe (tag and Dockerfile) in the Security Proxy section of
  both manuals; no test pins that recipe. An existing image with the tag is
  reused, so a host-built image works behind a socket proxy with `BUILD=0`.
  A failed build returns `ErrRateLimitImageUnavailable` and does not stop or
  remove a running container (see Lifecycle for the Caddyfile).
- Placement: a native install binds host paths and reaches AuraGo through
  the host gateway. When AuraGo runs in Docker, the manager inspects its own
  container (mountinfo ID, then hostname), maps the proxy directory onto
  that container's volume (`VolumeOptions.Subpath`, Engine API 1.45+) or bind
  source, joins the first non-internal user-defined network (prefer
  `*_default`, never the Docker control network) and proxies to the container
  name; otherwise it uses `host.docker.internal` and the published port. An
  engine that does not know the container (stray `/.dockerenv`) gets the
  native placement.
- Lifecycle: Start removes the old container only after the image is ready
  and reports `ErrCaddyExited` when Caddy stops within the settle time.
  Known gap: `startLocked` still writes the new Caddyfile before
  `ensureImage`, so after a failed build the running container keeps its
  loaded config but the file on disk is already the new one; the next
  container or daemon restart loads it, and the official image then fails on
  `rate_limit`. Reload runs an attached `caddy reload` and
  checks its exit code; a rejected config restores the previous Caddyfile
  (`ErrConfigRejected`). An image that no longer fits the config makes Reload
  recreate the container through `startLocked`. Ports, binds/mounts, network
  and `docker_host` change only through Start, which removes and recreates
  the container.
- Errors: the exported sentinel errors map to `backend.proxy_*` messages
  through `proxyErrorKey`; keep all 16 backend locales in step.

## Verification

- `go test ./internal/proxy`, `go test ./internal/server -run 'Proxy|ReplaceConfigSnapshot'`
  and `go test ./ui -run TestSecurityProxy`.
- Docker acceptance: native start and reload; a Compose-like stack (named
  data volume, user-defined network, socket proxy) with `BUILD=0` (clear
  rate-limit error) and `BUILD=1` (image build); Basic Auth 401/200 and HTTP
  429 after the burst. Local fixtures do not prove Let's Encrypt issuance.

## Child DOX Index

None.
