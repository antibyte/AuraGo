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
  without it, `caddy:latest` stays tagged `aurago-proxy:latest`. When that
  tag is missing, `engine.pull` (`tools.PullImageForce`) fetches the current
  `caddy:latest`, never a stale local copy, and an error event in the pull
  stream fails Start. Bump the Caddy and module pins together, re-check the
  generated `rate_limit` and `basic_auth` syntax against the built image, and
  update the host-side `docker build` recipe (tag and Dockerfile) in the
  Security Proxy section of both manuals;
  `TestManualRateLimitImageRecipeMatchesPins` fails until they match. An
  existing image with the tag is reused, so a host-built image works behind a
  socket proxy with `BUILD=0`. A failed build returns
  `ErrRateLimitImageUnavailable`; when `docker.read_only` refused it
  (`tools.ErrDockerReadOnly`), the error also wraps
  `ErrRateLimitImageReadOnly`, whose message does not ask for image builds.
- Placement: a native install binds host paths and reaches AuraGo through
  the host gateway. When AuraGo runs in Docker, the manager inspects its own
  container (`dockerutil.OwnContainerID` from /proc, then
  `dockerutil.DefaultContainerHostname`; the server's container protection
  uses the same helpers, and a custom hostname is never looked up), maps the
  proxy directory onto
  that container's volume (`VolumeOptions.Subpath`, Engine API 1.45+) or bind
  source, joins the first non-internal user-defined network (prefer
  `*_default`, never the Docker control network) and proxies to the container
  name; otherwise it uses `host.docker.internal` and the published port. An
  engine that does not know the container (stray `/.dockerenv`) gets the
  native placement.
- Create: the container is created through `engine.createTrusted`
  (`tools.DockerCreateRequestContextWithTrustedBinds`) with exactly the
  placement's own `binds` trusted, so a native install under `/root`, `/mnt`,
  `/etc` or `/hostfs` passes the create bind policy. Never trust any other
  bind; the Docker placement has only `Mounts`, so it trusts nothing. Every
  other proxy request stays on `engine.request` (`tools.DockerRequest`).
- Hardening (K20): `securityProxyCreatePayload` keeps root, a writable root
  filesystem and no `User` (earlier root containers left root-owned 0600
  certificates), and sets `no-new-privileges:true`, `CapDrop: ALL` and
  `CapAdd: NET_BIND_SERVICE, DAC_OVERRIDE, CHOWN, FOWNER` in every placement.
  `NET_BIND_SERVICE` is needed even where unprivileged ports are allowed:
  `/usr/bin/caddy` carries the file capability `cap_net_bind_service=ep` in
  both images, and without it the exec fails. `DAC_OVERRIDE` reads the 0600
  Caddyfile and writes the service user's 0750 `caddy_data`/`caddy_config`.
  Verified live (Docker 29.8, `caddy:latest` v2.11.7 and the rate-limit
  image): serving, certificate reuse after the old profile, `caddy reload`
  and rate limiting. Re-run such a probe before changing the list or the
  image base. An existing container gets the profile only when Start (or
  Reload's image-change recreate) recreates it.
- Ownership (I5): the container carries
  `dockerutil.ManagedLabels(SecurityProxyOwner, "caddy", "proxy", "")`, and
  `containerName` is `dockerutil.SecurityProxyContainerName`, which
  `IsSecurityProxyContainerName` reserves. The server's
  `containerProtectedOwners` and the agent docker tool's owner list include
  the owner, so the admin container API asks for `confirm=protected` before
  a terminal, update or remove, and agent create/run of the name is refused.
  The manager itself addresses the container only by name through
  `engine.request`, never by labels: containers that older versions created
  without labels keep working, and none of these checks apply to its own
  stop, remove, create, start and exec calls.
- Lifecycle: `startLocked` generates the Caddyfile in memory first (credential
  errors come before any build or pull), then runs `ensureImage`, and only
  after that writes the Caddyfile and removes the old container. A failed
  build or pull therefore leaves the running container and the Caddyfile it
  loads untouched; Reload's recreate path goes through `startLocked` and
  inherits this. When the old container cannot be removed (`stopAndRemove`
  returns nil only once the engine no longer knows the container, e.g. a
  refused DELETE under `docker.read_only`) or the new one cannot be created,
  `startLocked` writes the previous Caddyfile back in place
  (`caddyfileRestorer`, shared with Reload) and fails, so the old container
  never restarts into the new file. Destroy still reports success and only
  logs a failed removal. Start reports `ErrCaddyExited` when Caddy stops within the
  settle time. Reload runs an attached `caddy reload` and
  checks its exit code; a rejected config restores the previous Caddyfile
  (`ErrConfigRejected`). An image that no longer fits the config makes Reload
  recreate the container through `startLocked`. Ports, binds/mounts, network
  and `docker_host` change only through Start, which removes and recreates
  the container.
- Lifecycle lock: Start, Reload, Stop and Destroy share `Manager.lifecycle`.
  A first rate-limit build holds it for up to 30 minutes
  (`rateLimitBuildTimeout`), a first `caddy:latest` pull for up to 15
  minutes (the `tools.PullImageForce` fallback). Stop, Destroy, Reload,
  another Start and the auto-start or auto-stop after a config save wait
  until it ends; their API requests stay open meanwhile. Status and Logs do
  not take the lock. This is accepted behaviour: do not release the lock
  around the build without a design for a Destroy or Start that runs during
  it.
- Errors: the exported sentinel errors map to `backend.proxy_*` messages
  through `proxyErrorKey`, first match wins: keep `ErrRateLimitImageReadOnly`
  before `ErrRateLimitImageUnavailable`. Keep all 16 backend locales in step.

## Verification

- `go test ./internal/proxy`, `go test ./internal/server -run 'Proxy|ReplaceConfigSnapshot'`
  and `go test ./ui -run TestSecurityProxy`.
- Docker acceptance: native start and reload; a Compose-like stack (named
  data volume, user-defined network, socket proxy) with `BUILD=0` (clear
  rate-limit error) and `BUILD=1` (image build); Basic Auth 401/200 and HTTP
  429 after the burst. Local fixtures do not prove Let's Encrypt issuance.

## Child DOX Index

None.
